package dash

import (
	"context"
	"os"
	"os/user"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thassiov/cmdvault/internal/command"
	"github.com/thassiov/cmdvault/internal/history"
	"github.com/thassiov/cmdvault/internal/resolve"
)

// activeRun tracks a single in-flight command execution.
type activeRun struct {
	cmd       *command.Command
	startedAt time.Time
}

// Bubbletea messages for the run lifecycle.
type (
	outputLineMsg struct {
		line string
	}
	runFinishedMsg struct {
		exitCode int
		duration time.Duration
	}
	runFailedMsg struct {
		err error
	}
	// runRejectedMsg is emitted when we can't run the selected command
	// (e.g., it has placeholders, which M5 will handle).
	runRejectedMsg struct {
		reason string
	}
)

// startRun launches the command at the given index. Returns the next tea.Cmd
// that begins draining the command's output channel, or a rejection.
func (m *Model) startRun(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.commands) {
		return nil
	}
	d := m.commands[idx]

	// M5 will handle placeholder prompts. For now, reject commands that
	// have them so we don't run with unresolved {{name}} literals.
	if placeholders := resolve.ExtractPlaceholders(d.Args); len(placeholders) > 0 {
		return func() tea.Msg {
			return runRejectedMsg{reason: "command has placeholders (M5 will handle prompting)"}
		}
	}

	cmd := command.New(d)
	if err := cmd.Start(context.Background()); err != nil {
		return func() tea.Msg { return runFailedMsg{err: err} }
	}

	m.active = &activeRun{cmd: cmd, startedAt: time.Now()}

	m.output.StartRun(RunRecord{
		Descriptor: d,
		Args:       d.Args,
		StartedAt:  m.active.startedAt,
	})

	return waitForOutput(cmd)
}

// waitForOutput reads one message from the command's output channel and
// returns it as a tea.Msg. On channel close, returns runFinishedMsg.
//
// The Update loop re-invokes this via another waitForOutput call for each
// received line, draining the channel one message at a time. If output is
// very bursty this could fall behind — acceptable for M3; revisit if it lags.
func waitForOutput(cmd *command.Command) tea.Cmd {
	return func() tea.Msg {
		out, ok := <-cmd.Output
		if !ok {
			// Channel closed — process has exited. ExitCode is set before
			// close() in command.wait, so it's safe to read here.
			code := -1
			if cmd.ExitCode != nil {
				code = *cmd.ExitCode
			}
			var duration time.Duration
			if cmd.StartedAt != nil && cmd.FinishedAt != nil {
				duration = cmd.FinishedAt.Sub(*cmd.StartedAt)
			}
			return runFinishedMsg{exitCode: code, duration: duration}
		}
		return outputLineMsg{line: out.Content}
	}
}

// logRunToHistory records a completed run in the JSONL history file.
// Errors are swallowed to avoid disrupting the TUI.
func logRunToHistory(desc command.Descriptor, args []string, startedAt time.Time, duration time.Duration, exitCode int) {
	hist, err := history.New()
	if err != nil {
		return
	}

	username := "unknown"
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	workdir, _ := os.Getwd()

	_ = hist.Log(history.Entry{
		Timestamp: startedAt,
		User:      username,
		Name:      desc.Name,
		Command:   desc.Command,
		Args:      args,
		ExitCode:  exitCode,
		Duration:  duration,
		WorkDir:   workdir,
	})
}
