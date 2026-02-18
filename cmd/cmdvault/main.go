package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
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
		fmt.Scanln(&answer)

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
					config = &cfg
				}
			}
			values[name] = getPlaceholderValue(name, config)
		}
	}

	// Fill placeholders and append passthrough args
	finalArgs := fillPlaceholders(selected.Descriptor.Args, values)
	finalArgs = append(finalArgs, passthroughArgs...)

	// Update the command's args with the processed ones
	selected.Descriptor.Args = finalArgs

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

	hist.Log(entry)
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
func promptForValue(name string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stderr, "%s: ", name)
	value, _ := reader.ReadString('\n')
	return strings.TrimSpace(value)
}

// selectFromSource runs a source command and pipes output to fzf for selection
func selectFromSource(name, source string) (string, error) {
	// Run source command and pipe to fzf
	cmd := exec.Command("sh", "-c", source+" | fzf --height=40% --layout=reverse --border --prompt='"+name+"> '")
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		// User cancelled or error
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// getPlaceholderValue gets a value for a placeholder, using source if available
func getPlaceholderValue(name string, config *command.PlaceholderConfig) string {
	if config != nil && config.Source != "" {
		value, err := selectFromSource(name, config.Source)
		if err != nil {
			// Fallback to manual prompt if fzf fails
			fmt.Fprintf(os.Stderr, "Selection cancelled, enter manually.\n")
			return promptForValue(name)
		}
		return value
	}
	return promptForValue(name)
}
