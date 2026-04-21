package dash

import (
	"fmt"
	"strings"

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

	// M1 placeholder: last picked command shown in status line until M2 replaces it.
	lastPicked string
}

// NewModel constructs the initial dashboard model.
func NewModel(cmds []command.Descriptor, cwd string) Model {
	return Model{
		commands: cmds,
		cwd:      cwd,
		picker:   NewPicker(cmds),
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
		m.resizePicker()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			// For now: unfocus quits (simple). Finalized in M6.
			return m, tea.Quit
		}
		// Fall through to picker.
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd

	case RunRequestedMsg:
		d := m.commands[msg.Index]
		m.lastPicked = fmt.Sprintf("would run: %s %s", d.Command, strings.Join(d.Args, " "))
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

	inner := m.height - 2
	pickerH := inner * 30 / 100
	if pickerH < 6 {
		pickerH = 6
	}
	if pickerH > inner-3 {
		pickerH = inner - 3
	}
	outputH := inner - pickerH

	topBar := styleTopBar.
		Width(m.width).
		Render("cmdvault — " + m.cwd)

	output := styleOutputPane.
		Width(m.width).
		Height(outputH).
		Render(stylePlaceholder.Render("(output pane — empty)"))

	picker := stylePickerPane.
		Width(m.width - 2).
		Height(pickerH - 1).
		Render(m.picker.View())

	statusText := "M1 picker · ↑↓ move · ⏎ run · ^c quit"
	if m.lastPicked != "" {
		statusText = m.lastPicked
	}
	status := styleStatusLine.
		Width(m.width).
		Render(statusText)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		topBar,
		strings.TrimRight(output, "\n"),
		picker,
		status,
	)
}

// resizePicker tells the picker its drawable area.
// Must be kept in sync with View's layout math.
func (m *Model) resizePicker() {
	inner := m.height - 2
	pickerH := inner * 30 / 100
	if pickerH < 6 {
		pickerH = 6
	}
	if pickerH > inner-3 {
		pickerH = inner - 3
	}
	// Account for pane padding (1 col each side) and border top (1 row).
	m.picker.SetSize(m.width-4, pickerH-2)
}
