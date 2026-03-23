package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/thassiov/cmdvault/internal/command"
	"github.com/thassiov/cmdvault/internal/history"
	"github.com/thassiov/cmdvault/internal/loader"
	"github.com/thassiov/cmdvault/internal/orchestrator"
	"github.com/thassiov/cmdvault/internal/picker"
	"github.com/thassiov/cmdvault/internal/prompt"
	"github.com/thassiov/cmdvault/internal/resolve"
	"github.com/thassiov/cmdvault/internal/shell"
)

// Version is set at build time via -ldflags "-X main.Version=...".
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

	commands := loadCommands(*filePath)

	if *listAliases {
		for _, cmd := range commands {
			fmt.Println(cmd.Alias)
		}
		os.Exit(0)
	}

	orch := orchestrator.New()
	orch.LoadFromDescriptors(commands)

	selected, cliArgs := selectCommand(orch, *simple)

	resolvedArgs := resolvePlaceholders(selected, cliArgs)

	if *printMode {
		printResolved(selected.Descriptor.Command, resolvedArgs)
		os.Exit(0)
	}

	selected.Descriptor.Args = resolvedArgs
	executeAndLog(selected, resolvedArgs)
}

// loadCommands loads command descriptors, creating the default directory if needed.
func loadCommands(filePath string) []command.Descriptor {
	l, err := loader.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if filePath == "" && !l.DefaultDirExists() {
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

	commands, err := l.Load(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading commands: %v\n", err)
		os.Exit(1)
	}

	if len(commands) == 0 {
		fmt.Println("no commands found")
		os.Exit(0)
	}

	return commands
}

// selectCommand picks a command via alias argument or interactive picker.
func selectCommand(orch *orchestrator.Orchestrator, simple bool) (*command.Command, []string) {
	if alias := flag.Arg(0); alias != "" {
		selected := orch.FindByAlias(alias)
		if selected == nil {
			fmt.Fprintf(os.Stderr, "error: unknown alias %q\n", alias)
			os.Exit(1)
		}
		return selected, flag.Args()[1:]
	}

	cmdList := orch.List()
	var selected *command.Command
	var err error

	if simple {
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

	return selected, nil
}

// resolvePlaceholders processes CLI args, fills placeholders, and returns the final arg list.
func resolvePlaceholders(selected *command.Command, cliArgs []string) []string {
	placeholderArgs, passthroughArgs := resolve.SplitOnDoubleDash(cliArgs)
	placeholders := resolve.ExtractPlaceholders(selected.Descriptor.Args)

	if len(placeholderArgs) > len(placeholders) {
		fmt.Fprintf(os.Stderr, "error: expected %d argument(s) but got %d\n", len(placeholders), len(placeholderArgs))
		if len(passthroughArgs) == 0 {
			fmt.Fprintf(os.Stderr, "hint: use -- to pass extra arguments to the command (e.g., cmdvault %s arg1 -- --extra-flag)\n", selected.Descriptor.Alias)
		}
		os.Exit(1)
	}

	values := make(map[string]string)
	for i, val := range placeholderArgs {
		values[placeholders[i]] = val
	}

	for _, name := range placeholders {
		if _, ok := values[name]; !ok {
			var config *command.PlaceholderConfig
			if selected.Descriptor.Placeholders != nil {
				if cfg, exists := selected.Descriptor.Placeholders[name]; exists {
					cfgCopy := cfg
					config = &cfgCopy
				}
			}
			values[name] = prompt.GetPlaceholderValue(name, config, values)
		}
	}

	resolvedArgs := resolve.FillPlaceholders(selected.Descriptor.Args, values)
	resolvedArgs = append(resolvedArgs, passthroughArgs...)
	return resolvedArgs
}

// printResolved outputs the resolved command string for --print mode.
func printResolved(cmd string, args []string) {
	if len(args) > 0 {
		fmt.Printf("%s %s", cmd, shell.Join(args))
	} else {
		fmt.Print(cmd)
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Println()
	}
}

// executeAndLog runs the command and logs execution to history.
func executeAndLog(selected *command.Command, resolvedArgs []string) {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	if isTTY {
		fmt.Printf("\nRunning: %s %v\n", selected.Descriptor.Command, resolvedArgs)
		fmt.Println(strings.Repeat("-", 40))
	}

	startTime := time.Now()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := selected.Start(ctx); err != nil {
		stop()
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

	logExecution(selected, startTime, duration)
}

func logExecution(cmd *command.Command, startTime time.Time, duration time.Duration) {
	hist, err := history.New()
	if err != nil {
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
