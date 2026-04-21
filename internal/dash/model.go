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

	// Spinner frame advanced on each tickMsg.
	spinnerIdx int

	// When the user pressed ^c while idle; a second ^c within ~2s quits.
	quitPendingAt time.Time
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
			return m.handleCtrlC()
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
			m.active.lineCount++
			return m, waitForOutput(m.active.cmd)
		}
		return m, nil

	case tickMsg:
		if m.active == nil {
			return m, nil
		}
		m.spinnerIdx = (m.spinnerIdx + 1) % len(spinnerFrames)
		return m, tickEvery(100 * time.Millisecond)

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
		spinner := spinnerFrames[m.spinnerIdx]
		elapsed := formatElapsed(time.Since(m.active.startedAt))
		hint := "^c stop"
		if !m.active.sigintAt.IsZero() && time.Since(m.active.sigintAt) < 2*time.Second {
			hint = "^c again to force kill"
		}
		return fmt.Sprintf("%s  %s · %s · %d lines · %s",
			spinner, m.active.cmd.Descriptor.Name, elapsed, m.active.lineCount, hint)
	}
	follow := " (follow)"
	if !m.output.AtBottom() {
		follow = " (paused — ^g to resume)"
	}
	return fmt.Sprintf("⏎ run · ^u/^d scroll%s · ^k clear · ^c quit", follow)
}

func (m *Model) setFlash(msg string, d time.Duration) {
	m.flash = msg
	m.flashUntil = time.Now().Add(d)
}

// handleCtrlC implements the three-way ^c behavior:
//   - run in flight: first ^c → SIGINT, second within 2s → SIGKILL
//   - idle, no runs yet: quit
//   - idle, runs exist: first ^c arms quit, second within 2s actually quits
func (m Model) handleCtrlC() (tea.Model, tea.Cmd) {
	if m.active != nil {
		if m.active.sigintAt.IsZero() || time.Since(m.active.sigintAt) > 2*time.Second {
			if err := m.active.cmd.Stop(); err != nil {
				m.setFlash("stop failed: "+err.Error(), 2*time.Second)
				return m, nil
			}
			m.active.sigintAt = time.Now()
			m.setFlash("SIGINT sent · ^c again within 2s to force kill", 2*time.Second)
			return m, nil
		}
		if err := m.active.cmd.Kill(); err != nil {
			m.setFlash("kill failed: "+err.Error(), 2*time.Second)
			return m, nil
		}
		m.setFlash("SIGKILL sent", 2*time.Second)
		return m, nil
	}

	// Idle.
	if len(m.output.runs) == 0 {
		return m, tea.Quit
	}
	if !m.quitPendingAt.IsZero() && time.Since(m.quitPendingAt) < 2*time.Second {
		return m, tea.Quit
	}
	m.quitPendingAt = time.Now()
	m.setFlash("^c again within 2s to quit", 2*time.Second)
	return m, nil
}

// formatElapsed renders elapsed time as MM:SS (or HH:MM:SS past an hour).
func formatElapsed(d time.Duration) string {
	d = d.Truncate(time.Second)
	h := int(d / time.Hour)
	m := int(d/time.Minute) % 60
	s := int(d/time.Second) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
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
