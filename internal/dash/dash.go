package dash

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thassiov/cmdvault/internal/command"
)

// Run starts the dashboard TUI with the given commands.
// It blocks until the user quits.
func Run(cmds []command.Descriptor) error {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "?"
	}

	p := tea.NewProgram(
		NewModel(cmds, cwd),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("dash: %w", err)
	}
	return nil
}
