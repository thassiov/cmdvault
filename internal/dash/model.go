package dash

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thassiov/cmdvault/internal/command"
)

// Model is the top-level bubbletea model for the dashboard.
type Model struct {
	width, height int
	cwd           string
	commands      []command.Descriptor

	picker Picker
	output Output
}

// NewModel constructs the initial dashboard model.
func NewModel(cmds []command.Descriptor, cwd string) Model {
	return Model{
		commands: cmds,
		cwd:      cwd,
		picker:   NewPicker(cmds),
		output:   NewOutput(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.picker.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeChildren()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			// Temporary: Esc quits. Finalized in M6.
			return m, tea.Quit
		case "ctrl+k":
			m.output.Clear()
			return m, nil
		case "ctrl+u":
			m.output.ScrollUp(m.outputHeight() / 2)
			return m, nil
		case "ctrl+d":
			m.output.ScrollDown(m.outputHeight() / 2)
			return m, nil
		case "ctrl+b":
			m.output.ScrollUp(m.outputHeight())
			return m, nil
		case "ctrl+f":
			m.output.ScrollDown(m.outputHeight())
			return m, nil
		case "ctrl+g":
			m.output.GotoBottom()
			return m, nil
		}
		// Everything else → picker (it owns typing + up/down/enter).
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd

	case RunRequestedMsg:
		// M2: synthesize a fake run so we can verify the output pane.
		// M3 will replace this with the real command lifecycle.
		d := m.commands[msg.Index]
		fake := RunRecord{
			Descriptor: d,
			Args:       d.Args,
			Body:       syntheticBody(d),
			StartedAt:  time.Now(),
			Duration:   142 * time.Millisecond,
			ExitCode:   0,
		}
		m.output.AppendRun(fake)
		return m, nil
	}

	// Unknown message: forward to picker.
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	outputH := m.outputHeight()
	pickerH := m.pickerHeight()

	topBar := styleTopBar.
		Width(m.width).
		Render("cmdvault — " + m.cwd)

	output := styleOutputPane.
		Width(m.width).
		Height(outputH).
		Render(m.output.View())

	picker := stylePickerPane.
		Width(m.width - 2).
		Height(pickerH - 1).
		Render(m.picker.View())

	status := styleStatusLine.
		Width(m.width).
		Render(m.statusText())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		topBar,
		strings.TrimRight(output, "\n"),
		picker,
		status,
	)
}

func (m Model) statusText() string {
	// Context-sensitive hints.
	follow := " (follow)"
	if !m.output.AtBottom() {
		follow = " (paused — ^g to resume)"
	}
	return fmt.Sprintf("M2 · ↑↓ pick · ⏎ run · ^u/^d scroll%s · ^k clear · ^c quit", follow)
}

// Layout helpers. Keep in sync with View.
func (m Model) pickerHeight() int {
	inner := m.height - 2
	h := inner * 30 / 100
	if h < 6 {
		h = 6
	}
	if h > inner-3 {
		h = inner - 3
	}
	return h
}

func (m Model) outputHeight() int {
	return (m.height - 2) - m.pickerHeight()
}

func (m *Model) resizeChildren() {
	pickerH := m.pickerHeight()
	outputH := m.outputHeight()
	// Picker: width minus border padding (1 each side) and padding (1 each side).
	m.picker.SetSize(m.width-4, pickerH-2)
	// Output: width minus horizontal padding (1 each side).
	m.output.SetSize(m.width-2, outputH)
}

// syntheticBody produces a plausible fake body so we can test the output pane
// without plumbing real command execution. Removed in M3.
func syntheticBody(d command.Descriptor) string {
	return strings.Join([]string{
		"[synthetic output for M2 testing]",
		"descriptor.name: " + d.Name,
		"descriptor.category: " + d.Category,
		"descriptor.description: " + d.Description,
		"(real command execution lands in M3)",
	}, "\n")
}
