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

	active *activeRun // nil when idle

	// Transient status line override (e.g., "can't run — already running").
	flash      string
	flashUntil time.Time
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
		// Everything else → picker.
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd

	case RunRequestedMsg:
		if m.active != nil {
			m.setFlash("command already running — ^c to stop first", 2*time.Second)
			return m, nil
		}
		cmd := m.startRun(msg.Index)
		return m, cmd

	case outputLineMsg:
		m.output.AppendLine(msg.line)
		if m.active != nil {
			return m, waitForOutput(m.active.cmd)
		}
		return m, nil

	case runFinishedMsg:
		m.output.FinishRun(msg.exitCode, msg.duration)
		if m.active != nil {
			logRunToHistory(
				m.active.cmd.Descriptor,
				m.active.cmd.Descriptor.Args,
				m.active.startedAt,
				msg.duration,
				msg.exitCode,
			)
			m.active = nil
		}
		return m, nil

	case runFailedMsg:
		// Start failure: render a one-shot run record with the error.
		if m.active == nil {
			m.output.AppendRun(RunRecord{
				Descriptor: command.Descriptor{Command: "(error)"},
				Lines:      []string{msg.err.Error()},
				ExitCode:   -1,
			})
		} else {
			m.output.AppendLine("error: " + msg.err.Error())
			m.output.FinishRun(-1, time.Since(m.active.startedAt))
			m.active = nil
		}
		return m, nil

	case runRejectedMsg:
		m.setFlash(msg.reason, 3*time.Second)
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
	if m.flash != "" && time.Now().Before(m.flashUntil) {
		return m.flash
	}
	if m.active != nil {
		elapsed := time.Since(m.active.startedAt).Truncate(time.Second)
		return fmt.Sprintf("● running %s · %s · ^c stop (M4)", m.active.cmd.Descriptor.Name, elapsed)
	}
	follow := " (follow)"
	if !m.output.AtBottom() {
		follow = " (paused — ^g to resume)"
	}
	return fmt.Sprintf("M3 · ⏎ run · ^u/^d scroll%s · ^k clear · ^c quit", follow)
}

func (m *Model) setFlash(msg string, d time.Duration) {
	m.flash = msg
	m.flashUntil = time.Now().Add(d)
}

// Layout helpers — keep in sync with View.
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
	m.picker.SetSize(m.width-4, pickerH-2)
	m.output.SetSize(m.width-2, outputH)
}
