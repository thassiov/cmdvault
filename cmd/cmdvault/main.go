package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/thassiov/cmdvault/internal/command"
	"github.com/thassiov/cmdvault/internal/history"
	"github.com/thassiov/cmdvault/internal/loader"
	"github.com/thassiov/cmdvault/internal/orchestrator"
	"github.com/thassiov/cmdvault/internal/picker"
)

// Version is set at build time via -ldflags "-X main.Version=..."
var Version = "dev"

func main() {
	filePath := flag.String("f", "", "path to command file or directory")
	simple := flag.Bool("simple", false, "use simple numbered list instead of fuzzy finder")
	listAliases := flag.Bool("list-aliases", false, "list all command aliases (for shell completion)")
	printMode := flag.Bool("print", false, "print the resolved command instead of executing it")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("cmdvault %s\n", Version)
		os.Exit(0)
	}

	l, err := loader.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// If using default directory and it doesn't exist, offer to create it
	if *filePath == "" && !l.DefaultDirExists() {
		fmt.Printf("Commands directory not found: %s\n", l.GetCommandsDir())
		fmt.Print("Create it? (y/n): ")

		var answer string
		_, _ = fmt.Scanln(&answer)

		if answer != "y" && answer != "Y" {
			os.Exit(0)
		}

		if err := l.EnsureDefaultDirsWithExamples(); err != nil {
			fmt.Fprintf(os.Stderr, "error creating directory: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created %s\n", l.GetCommandsDir())
		fmt.Println("Added example command files. Run again to get started.")
		os.Exit(0)
	}

	commands, err := l.Load(*filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading commands: %v\n", err)
		os.Exit(1)
	}

	if len(commands) == 0 {
		fmt.Println("no commands found")
		os.Exit(0)
	}

	// Handle --list-aliases for shell completion
	if *listAliases {
		for _, cmd := range commands {
			fmt.Println(cmd.Alias)
		}
		os.Exit(0)
	}

	orch := orchestrator.New()
	orch.LoadFromDescriptors(commands)

	var selected *command.Command
	var cliArgs []string // args after alias (for placeholders and passthrough)

	// Check if an alias was provided as positional argument
	if alias := flag.Arg(0); alias != "" {
		selected = orch.FindByAlias(alias)
		if selected == nil {
			fmt.Fprintf(os.Stderr, "error: unknown alias %q\n", alias)
			os.Exit(1)
		}
		// Collect remaining args after alias
		cliArgs = flag.Args()[1:]
	} else {
		// No alias provided, use picker
		cmdList := orch.List()

		if *simple {
			selected, err = picker.PickSimple(cmdList)
		} else {
			selected, err = picker.Pick(cmdList)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		if selected == nil {
			os.Exit(0)
		}
	}

	// Process placeholders and passthrough args
	placeholderArgs, passthroughArgs := splitOnDoubleDash(cliArgs)
	placeholders := extractPlaceholders(selected.Descriptor.Args)

	// Check for too many positional args
	if len(placeholderArgs) > len(placeholders) {
		fmt.Fprintf(os.Stderr, "error: expected %d argument(s) but got %d\n", len(placeholders), len(placeholderArgs))
		if len(passthroughArgs) == 0 {
			fmt.Fprintf(os.Stderr, "hint: use -- to pass extra arguments to the command (e.g., cmdvault %s arg1 -- --extra-flag)\n", selected.Descriptor.Alias)
		}
		os.Exit(1)
	}

	// Build values map from positional args
	values := make(map[string]string)
	for i, val := range placeholderArgs {
		values[placeholders[i]] = val
	}

	// Prompt for missing placeholders (with source selection if configured)
	for _, name := range placeholders {
		if _, ok := values[name]; !ok {
			var config *command.PlaceholderConfig
			if selected.Descriptor.Placeholders != nil {
				if cfg, exists := selected.Descriptor.Placeholders[name]; exists {
					cfgCopy := cfg // copy so we can mutate Default without affecting original
					config = &cfgCopy
				}
			}
			values[name] = getPlaceholderValue(name, config, values)
		}
	}

	// Fill placeholders and append passthrough args
	finalArgs := fillPlaceholders(selected.Descriptor.Args, values)
	finalArgs = append(finalArgs, passthroughArgs...)

	// Update the command's args with the processed ones
	selected.Descriptor.Args = finalArgs

	// Print mode: output the resolved command string and exit.
	// When stdout is a TTY (direct invocation), add a trailing newline for clean display.
	// When captured (e.g. $(cmdvault --print)), omit it so shell widgets work cleanly.
	if *printMode {
		if len(selected.Descriptor.Args) > 0 {
			fmt.Printf("%s %s", selected.Descriptor.Command, shelljoin(selected.Descriptor.Args))
		} else {
			fmt.Print(selected.Descriptor.Command)
		}
		if term.IsTerminal(int(os.Stdout.Fd())) {
			fmt.Println()
		}
		os.Exit(0)
	}

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	if isTTY {
		fmt.Printf("\nRunning: %s %v\n", selected.Descriptor.Command, selected.Descriptor.Args)
		fmt.Println(strings.Repeat("-", 40))
	}

	startTime := time.Now()

	if err := selected.Start(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error starting command: %v\n", err)
		os.Exit(1)
	}

	for out := range selected.Output {
		fmt.Println(out.Content)
	}

	duration := time.Since(startTime)

	if isTTY {
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("Exit code: %d\n", *selected.ExitCode)
	}

	// Log execution to history
	logExecution(selected, startTime, duration)
}

func logExecution(cmd *command.Command, startTime time.Time, duration time.Duration) {
	hist, err := history.New()
	if err != nil {
		// Silent fail - don't disrupt user experience for logging
		return
	}

	username := "unknown"
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	workdir, _ := os.Getwd()

	entry := history.Entry{
		Timestamp: startTime,
		User:      username,
		Name:      cmd.Descriptor.Name,
		Command:   cmd.Descriptor.Command,
		Args:      cmd.Descriptor.Args,
		ExitCode:  *cmd.ExitCode,
		Duration:  duration,
		WorkDir:   workdir,
	}

	_ = hist.Log(entry)
}

var placeholderRegex = regexp.MustCompile(`\{\{(\w+)\}\}`)

// extractPlaceholders finds all {{name}} placeholders in args, returns unique names in order
func extractPlaceholders(args []string) []string {
	seen := make(map[string]bool)
	var placeholders []string

	for _, arg := range args {
		matches := placeholderRegex.FindAllStringSubmatch(arg, -1)
		for _, match := range matches {
			name := match[1]
			if !seen[name] {
				seen[name] = true
				placeholders = append(placeholders, name)
			}
		}
	}

	return placeholders
}

// splitOnDoubleDash splits args into before and after "--"
func splitOnDoubleDash(args []string) (before, after []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

// fillPlaceholders replaces {{name}} with values from the map
func fillPlaceholders(args []string, values map[string]string) []string {
	result := make([]string, len(args))
	for i, arg := range args {
		result[i] = placeholderRegex.ReplaceAllStringFunc(arg, func(match string) string {
			name := placeholderRegex.FindStringSubmatch(match)[1]
			if val, ok := values[name]; ok {
				return val
			}
			return match
		})
	}
	return result
}

// promptForValue prompts the user to enter a value for the placeholder
func promptForValue(name string, config *command.PlaceholderConfig) string {
	reader := bufio.NewReader(os.Stdin)
	prompt := name
	if config != nil && config.Description != "" {
		prompt = fmt.Sprintf("%s (%s)", name, config.Description)
	}
	if config != nil && config.Default != "" {
		fmt.Fprintf(os.Stderr, "%s [%s]: ", prompt, config.Default)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
	}
	value, _ := reader.ReadString('\n')
	value = strings.TrimSpace(value)
	if value == "" && config != nil && config.Default != "" {
		return config.Default
	}
	return value
}

// selectFromSource runs a source command and pipes output to fzf for selection
func selectFromSource(name, source string) (string, error) {
	// Run source command and pipe to fzf
	cmd := exec.Command("sh", "-c", source+" | fzf --height=~100% --layout=reverse --border --cycle --prompt='"+name+"> '")
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		// User canceled or error
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// selectFile launches an fzf file picker with path-aware root switching.
// When the query starts with "/" it searches from root; with "~" from $HOME;
// otherwise from the current directory.
func selectFile(name string, config *command.PlaceholderConfig) (string, error) {
	prompt := name
	if config != nil && config.Description != "" {
		prompt = fmt.Sprintf("%s (%s)", name, config.Description)
	}

	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}

	// Shell script that picks the find root based on the fzf query prefix.
	// {q} is replaced by fzf with the current query string.
	// For / and ~, we limit depth to keep it fast; cwd gets unlimited depth.
	finderScript := fmt.Sprintf(`
q={q}
if [ "${q#/}" != "$q" ]; then
  find / -maxdepth 6 -type f -not -path '*/.git/*' 2>/dev/null | head -50000
elif [ "${q#\~}" != "$q" ]; then
  find %s -maxdepth 6 -type f -not -path '*/.git/*' 2>/dev/null | head -50000
else
  find . -type f -not -path '*/.git/*' 2>/dev/null | head -50000
fi
`, shellescape(home))

	// Build the fzf command.
	// --print-query is NOT used; we just want the selected line.
	// --bind start:reload runs the finder immediately.
	// --bind change:reload re-runs when the user types (to switch roots).
	fzfArgs := []string{
		"--height=~100%",
		"--layout=reverse",
		"--border",
		"--cycle",
		"--scheme=path",
		"--prompt=" + prompt + "> ",
		"--bind", "start:reload:" + strings.TrimSpace(finderScript),
		"--bind", "change:reload:" + strings.TrimSpace(finderScript),
	}

	// Pre-fill with default value if set
	if config != nil && config.Default != "" {
		fzfArgs = append(fzfArgs, "--query="+config.Default)
	}

	cmd := exec.Command("fzf", fzfArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	selected := strings.TrimSpace(string(output))

	// Expand ~ prefix if present in the selection (shouldn't happen with find, but safety)
	if strings.HasPrefix(selected, "~/") {
		selected = filepath.Join(home, selected[2:])
	}

	return selected, nil
}

// shellescape wraps a string in single quotes for safe shell embedding
func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// shelljoin combines args into a shell-safe string, quoting args that need it
func shelljoin(args []string) string {
	var parts []string
	for _, arg := range args {
		if arg == "" {
			parts = append(parts, "''")
		} else if needsQuoting(arg) {
			// Use single quotes, escaping any single quotes in the value
			escaped := strings.ReplaceAll(arg, "'", "'\\''")
			parts = append(parts, "'"+escaped+"'")
		} else {
			parts = append(parts, arg)
		}
	}
	return strings.Join(parts, " ")
}

// needsQuoting returns true if a string contains characters that need shell quoting
func needsQuoting(s string) bool {
	for _, c := range s {
		switch c {
		case ' ', '\t', '\n', '"', '\'', '\\', '`', '$', '!', '&', '|', ';', '(', ')', '<', '>', '*', '?', '[', ']', '#', '~', '{', '}', '^':
			return true
		}
	}
	return false
}

// getPlaceholderValue gets a value for a placeholder, using source or file picker if configured
func getPlaceholderValue(name string, config *command.PlaceholderConfig, resolved map[string]string) string {
	// Apply default template: {{other_placeholder}} references in default are replaced
	// with already-resolved values (e.g., output defaulting to input's path)
	if config != nil && config.Default != "" {
		config.Default = expandDefaultTemplate(config.Default, resolved)
	}

	// type: file → file picker
	if config != nil && config.Type == "file" {
		value, err := selectFile(name, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Selection canceled, enter manually.\n")
			return promptForValue(name, config)
		}
		return value
	}

	// source → fzf selection from command output
	if config != nil && config.Source != "" {
		value, err := selectFromSource(name, config.Source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Selection canceled, enter manually.\n")
			return promptForValue(name, config)
		}
		return value
	}

	// default → plain text prompt
	return promptForValue(name, config)
}

// expandDefaultTemplate replaces {{name}} references in a default value
// with already-resolved placeholder values. This allows e.g.:
//
//	output:
//	  default: "{{input}}"
func expandDefaultTemplate(tmpl string, resolved map[string]string) string {
	return placeholderRegex.ReplaceAllStringFunc(tmpl, func(match string) string {
		ref := placeholderRegex.FindStringSubmatch(match)[1]
		if val, ok := resolved[ref]; ok {
			return val
		}
		return match
	})
}
