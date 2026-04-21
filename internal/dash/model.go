package dash

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thassiov/cmdvault/internal/command"
	"github.com/thassiov/cmdvault/internal/resolve"
)

// Model is the top-level bubbletea model for the dashboard.
type Model struct {
	width, height int
	cwd           string
	commands      []command.Descriptor

	picker Picker
	output Output

	active    *activeRun   // nil when idle
	prompting *PromptState // non-nil while resolving placeholders

	// Transient status line override (e.g., "can't run — already running").
	flash      string
	flashUntil time.Time

	// Spinner frame advanced on each tickMsg.
	spinnerIdx int

	// When the user pressed ^c while idle; a second ^c within ~2s quits.
	quitPendingAt time.Time

	// Help overlay toggle (F1).
	showHelp bool
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
		// While prompting, all keys (except ^c) go to the prompt widget.
		if m.prompting != nil {
			if msg.String() == "ctrl+c" {
				return m.handleCtrlC()
			}
			cmd := m.prompting.prompt.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c":
			return m.handleCtrlC()
		case "f1":
			m.showHelp = !m.showHelp
			return m, nil
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

	case promptConfirmedMsg:
		if m.prompting == nil {
			return m, nil
		}
		done := m.prompting.advance(msg.value)
		if !done {
			m.resizeChildren()
			return m, nil
		}
		// All placeholders resolved — fill args and launch.
		desc := m.prompting.desc
		resolved := resolve.FillPlaceholders(desc.Args, m.prompting.values)
		m.prompting = nil
		m.resizeChildren()
		return m, m.launchCommand(desc, resolved)

	case promptCanceledMsg:
		m.prompting = nil
		m.resizeChildren()
		m.setFlash("prompt canceled", 2*time.Second)
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

	// Minimum usable size. Below this we bail out with a prompt to resize.
	if m.height < 10 || m.width < 40 {
		return styleTopBar.Width(m.width).Render("cmdvault — terminal too small (min 40×10)")
	}

	if m.showHelp {
		return m.renderHelp()
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

	var paneBody string
	if m.prompting != nil {
		paneBody = m.prompting.prompt.View()
	} else {
		paneBody = m.picker.View()
	}
	picker := stylePickerPane.
		Width(m.width - 2).
		Height(pickerH - 1).
		Render(paneBody)

	text, fg := m.statusLineParts()
	status := styleStatusLine.
		Width(m.width).
		Foreground(fg).
		Render(text)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		topBar,
		strings.TrimRight(output, "\n"),
		picker,
		status,
	)
}

// statusLineParts returns the text and color for the status line, picked from
// the current mode (flash / prompting / running / idle).
func (m Model) statusLineParts() (string, lipgloss.TerminalColor) {
	if m.flash != "" && time.Now().Before(m.flashUntil) {
		return m.flash, colorWarn
	}
	if m.prompting != nil {
		text := fmt.Sprintf("prompt %d/%d · %s · ⏎ confirm · Esc cancel",
			m.prompting.current+1, len(m.prompting.placeholders), m.prompting.desc.Name)
		return text, colorAccent
	}
	if m.active != nil {
		spinner := spinnerFrames[m.spinnerIdx]
		elapsed := formatElapsed(time.Since(m.active.startedAt))
		hint := "^c stop"
		if !m.active.sigintAt.IsZero() && time.Since(m.active.sigintAt) < 2*time.Second {
			hint = "^c again to force kill"
		}
		text := fmt.Sprintf("%s  %s · %s · %d lines · %s",
			spinner, m.active.cmd.Descriptor.Name, elapsed, m.active.lineCount, hint)
		return text, colorAccent
	}
	follow := " (follow)"
	if !m.output.AtBottom() {
		follow = " (paused — ^g to resume)"
	}
	return fmt.Sprintf("⏎ run · ^u/^d scroll%s · ^k clear · F1 help · ^c quit", follow), colorMuted
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

// renderHelp draws the full-screen help overlay.
func (m Model) renderHelp() string {
	title := styleTopBar.Width(m.width).Render("cmdvault — help")
	footer := styleStatusLine.Width(m.width).Foreground(colorMuted).
		Render("F1 to close")

	sections := []struct {
		name  string
		keys  [][2]string
	}{
		{"Picker", [][2]string{
			{"type", "filter commands"},
			{"↑ ↓ / ^p ^n", "move cursor"},
			{"PgUp PgDn", "page"},
			{"Home End", "first / last"},
			{"Enter", "run / confirm"},
			{"Esc", "clear search"},
		}},
		{"Output", [][2]string{
			{"^u ^d", "half-page up / down"},
			{"^b ^f", "full-page up / down"},
			{"^g", "jump to bottom, resume follow"},
			{"^k", "clear all output"},
		}},
		{"Run", [][2]string{
			{"^c (running)", "SIGINT; again within 2s to SIGKILL"},
			{"^c (idle)", "quit (confirm if runs exist)"},
		}},
		{"Prompts (placeholders)", [][2]string{
			{"type", "filter / enter text"},
			{"/ ~ (file picker)", "switch root (leading char)"},
			{"↑ ↓", "move cursor in list"},
			{"Enter", "confirm value"},
			{"Esc", "cancel whole prompt chain"},
		}},
		{"Global", [][2]string{
			{"F1", "toggle this help"},
		}},
	}

	headerStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(colorOK)

	var body strings.Builder
	for _, sec := range sections {
		body.WriteString(headerStyle.Render(sec.name))
		body.WriteString("\n")
		for _, kv := range sec.keys {
			body.WriteString(fmt.Sprintf("  %-20s  %s\n", keyStyle.Render(kv[0]), kv[1]))
		}
		body.WriteString("\n")
	}

	// Pad/trim body to fill the space between title and footer.
	inner := m.height - 2
	lines := strings.Split(body.String(), "\n")
	if len(lines) > inner {
		lines = lines[:inner]
	}
	for len(lines) < inner {
		lines = append(lines, "")
	}
	content := strings.Join(lines, "\n")

	return lipgloss.JoinVertical(lipgloss.Left, title, content, footer)
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
	if m.prompting != nil && m.prompting.prompt != nil {
		m.prompting.prompt.SetSize(m.width-4, pickerH-2)
	}
}
