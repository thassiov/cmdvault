package dash

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thassiov/cmdvault/internal/command"
)

// RunRecord is one completed command execution as rendered in the output pane.
// M3 will extend this to support in-progress runs.
type RunRecord struct {
	Descriptor command.Descriptor
	Args       []string // resolved args
	Body       string   // joined stdout/stderr
	StartedAt  time.Time
	Duration   time.Duration
	ExitCode   int
}

// Output is the scrollable run buffer.
type Output struct {
	vp   viewport.Model
	runs []RunRecord
}

// NewOutput constructs an empty Output pane.
func NewOutput() Output {
	vp := viewport.New(0, 0)
	vp.SetContent("")
	return Output{vp: vp}
}

// SetSize updates viewport dimensions.
func (o *Output) SetSize(w, h int) {
	o.vp.Width = w
	o.vp.Height = h
	o.vp.SetContent(o.renderAll())
}

// AppendRun adds a run to the buffer. If the user was at the bottom before
// the append, follow-tail scrolls to the new bottom; otherwise the viewport
// stays put so the user's scroll position is preserved.
func (o *Output) AppendRun(r RunRecord) {
	wasAtBottom := o.vp.AtBottom()
	o.runs = append(o.runs, r)
	o.vp.SetContent(o.renderAll())
	if wasAtBottom {
		o.vp.GotoBottom()
	}
}

// Clear empties the output pane.
func (o *Output) Clear() {
	o.runs = o.runs[:0]
	o.vp.SetContent("")
	o.vp.GotoTop()
}

// ScrollUp scrolls the viewport up by n lines. Pauses follow-tail implicitly
// because it moves YOffset away from the bottom.
func (o *Output) ScrollUp(n int) {
	o.vp.ScrollUp(n)
}

// ScrollDown scrolls the viewport down by n lines.
func (o *Output) ScrollDown(n int) {
	o.vp.ScrollDown(n)
}

// GotoBottom jumps to the newest run and resumes follow-tail.
func (o *Output) GotoBottom() {
	o.vp.GotoBottom()
}

// AtBottom reports whether the viewport is scrolled to the end.
func (o Output) AtBottom() bool {
	return o.vp.AtBottom()
}

// Update delegates scroll messages to the inner viewport.
// The model decides which key events to forward.
func (o Output) Update(msg tea.Msg) (Output, tea.Cmd) {
	var cmd tea.Cmd
	o.vp, cmd = o.vp.Update(msg)
	return o, cmd
}

// View renders the output pane.
func (o Output) View() string {
	if o.vp.Height == 0 || o.vp.Width == 0 {
		return ""
	}
	if len(o.runs) == 0 {
		return stylePlaceholder.Render("(output pane — run a command)")
	}
	return o.vp.View()
}

// renderAll produces the full viewport content from all runs.
func (o Output) renderAll() string {
	var b strings.Builder
	for i, r := range o.runs {
		b.WriteString(renderRun(r))
		if i < len(o.runs)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// renderRun formats a single run as:
//
//	$ <cmd> <args...>
//	<body>
//	[exit N · Xs]
func renderRun(r RunRecord) string {
	header := styleRunHeader.Render("$ " + commandLine(r))
	footer := styleRunFooter.Render(fmt.Sprintf("[exit %d · %s]", r.ExitCode, shortDuration(r.Duration)))

	body := strings.TrimRight(r.Body, "\n")
	if body == "" {
		return header + "\n" + footer + "\n"
	}
	return header + "\n" + body + "\n" + footer + "\n"
}

func commandLine(r RunRecord) string {
	if len(r.Args) == 0 {
		return r.Descriptor.Command
	}
	return r.Descriptor.Command + " " + strings.Join(r.Args, " ")
}

// shortDuration formats a duration for the run footer.
//
//	0.08s, 2.4s, 1m12s, 1h02m
func shortDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < 10*time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// run header + footer styles live here rather than in style.go to keep the
// Output package self-contained.
var (
	styleRunHeader = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	styleRunFooter = lipgloss.NewStyle().Foreground(colorDim)
)
