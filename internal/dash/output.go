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

// RunRecord is a single command execution rendered in the output pane.
// It may be in-flight (Finished=false, lines accumulating) or completed.
type RunRecord struct {
	Descriptor command.Descriptor
	Args       []string // resolved args
	Lines      []string // stdout/stderr lines in arrival order
	StartedAt  time.Time
	Duration   time.Duration
	ExitCode   int
	Finished   bool
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
	o.refresh()
}

// StartRun begins a new in-flight run. It renders the header immediately; the
// footer appears on FinishRun.
func (o *Output) StartRun(r RunRecord) {
	wasAtBottom := o.vp.AtBottom()
	o.runs = append(o.runs, r)
	o.refresh()
	if wasAtBottom {
		o.vp.GotoBottom()
	}
}

// AppendLine appends a line to the most recent run (which must be in-flight).
// If there's no in-flight run, the line is dropped.
func (o *Output) AppendLine(line string) {
	if len(o.runs) == 0 {
		return
	}
	last := &o.runs[len(o.runs)-1]
	if last.Finished {
		return
	}
	wasAtBottom := o.vp.AtBottom()
	last.Lines = append(last.Lines, line)
	o.refresh()
	if wasAtBottom {
		o.vp.GotoBottom()
	}
}

// FinishRun finalizes the most recent in-flight run with its exit code and
// duration. Writes the footer.
func (o *Output) FinishRun(exitCode int, duration time.Duration) {
	if len(o.runs) == 0 {
		return
	}
	last := &o.runs[len(o.runs)-1]
	if last.Finished {
		return
	}
	wasAtBottom := o.vp.AtBottom()
	last.ExitCode = exitCode
	last.Duration = duration
	last.Finished = true
	o.refresh()
	if wasAtBottom {
		o.vp.GotoBottom()
	}
}

// AppendRun adds a fully-finished run in one shot. Kept for cases where we
// don't need streaming (e.g., synthetic test runs).
func (o *Output) AppendRun(r RunRecord) {
	r.Finished = true
	o.StartRun(r)
}

// Clear empties the output pane.
func (o *Output) Clear() {
	o.runs = o.runs[:0]
	o.vp.SetContent("")
	o.vp.GotoTop()
}

func (o *Output) ScrollUp(n int)   { o.vp.ScrollUp(n) }
func (o *Output) ScrollDown(n int) { o.vp.ScrollDown(n) }
func (o *Output) GotoBottom()      { o.vp.GotoBottom() }
func (o Output) AtBottom() bool    { return o.vp.AtBottom() }

// Update delegates to the inner viewport.
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

// refresh re-renders the viewport content from the current runs slice.
func (o *Output) refresh() {
	var b strings.Builder
	for i, r := range o.runs {
		b.WriteString(renderRun(r))
		if i < len(o.runs)-1 {
			b.WriteString("\n")
		}
	}
	o.vp.SetContent(b.String())
}

// renderRun formats one run as:
//
//	$ <cmd> <args...>
//	<body>
//	[exit N · Xs]      (only if Finished)
func renderRun(r RunRecord) string {
	header := styleRunHeader.Render("$ " + commandLine(r))
	body := strings.Join(r.Lines, "\n")

	var footer string
	if r.Finished {
		footer = styleRunFooter.Render(fmt.Sprintf("[exit %d · %s]", r.ExitCode, shortDuration(r.Duration)))
	}

	parts := []string{header}
	if body != "" {
		parts = append(parts, body)
	}
	if footer != "" {
		parts = append(parts, footer)
	}
	return strings.Join(parts, "\n") + "\n"
}

func commandLine(r RunRecord) string {
	if len(r.Args) == 0 {
		return r.Descriptor.Command
	}
	return r.Descriptor.Command + " " + strings.Join(r.Args, " ")
}

// shortDuration formats a duration for the run footer.
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

var (
	styleRunHeader = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	styleRunFooter = lipgloss.NewStyle().Foreground(colorDim)
)
