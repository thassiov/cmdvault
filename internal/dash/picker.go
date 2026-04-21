package dash

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/thassiov/cmdvault/internal/command"
)

// RunRequestedMsg is emitted when the user picks a command with Enter.
type RunRequestedMsg struct {
	Index int // index into the full commands slice
}

// Picker owns the search box and filtered result list.
type Picker struct {
	input    textinput.Model
	all      []command.Descriptor
	haystack []string // precomputed "category name description" per command
	filtered []int    // indices into all, in display order
	cursor   int      // cursor position within filtered
	offset   int      // scroll offset (first visible row in filtered)
	width    int
	height   int
}

// NewPicker builds a picker over the given commands.
func NewPicker(cmds []command.Descriptor) Picker {
	ti := textinput.New()
	ti.Prompt = "search › "
	ti.Placeholder = "type to filter"
	ti.Focus()
	ti.CharLimit = 128

	hay := make([]string, len(cmds))
	for i, c := range cmds {
		hay[i] = c.Category + " " + c.Name + " " + c.Description
	}

	p := Picker{
		input:    ti,
		all:      cmds,
		haystack: hay,
	}
	p.applyFilter()
	return p
}

// SetSize updates the picker's drawable size (total rows including the input line).
func (p *Picker) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.input.Width = w - len(p.input.Prompt) - 20 // leave room for counter on the right
	p.clampCursor()
}

// Selected returns the currently highlighted descriptor, if any.
func (p Picker) Selected() (command.Descriptor, int, bool) {
	if len(p.filtered) == 0 || p.cursor < 0 || p.cursor >= len(p.filtered) {
		return command.Descriptor{}, -1, false
	}
	idx := p.filtered[p.cursor]
	return p.all[idx], idx, true
}

func (p Picker) Init() tea.Cmd {
	return textinput.Blink
}

func (p Picker) Update(msg tea.Msg) (Picker, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "ctrl+p":
			p.moveCursor(-1)
			return p, nil
		case "down", "ctrl+n":
			p.moveCursor(1)
			return p, nil
		case "pgup":
			p.moveCursor(-p.visibleRows())
			return p, nil
		case "pgdown":
			p.moveCursor(p.visibleRows())
			return p, nil
		case "home":
			p.cursor = 0
			p.offset = 0
			return p, nil
		case "end":
			p.cursor = len(p.filtered) - 1
			p.ensureVisible()
			return p, nil
		case "enter":
			if _, idx, ok := p.Selected(); ok {
				return p, func() tea.Msg { return RunRequestedMsg{Index: idx} }
			}
			return p, nil
		}
	}

	// Any other key goes to the text input (incl. typing, backspace, arrows inside field).
	prev := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != prev {
		p.applyFilter()
	}
	return p, cmd
}

func (p Picker) View() string {
	if p.width == 0 {
		return ""
	}

	// Header row: search input + counter on the right.
	counter := fmt.Sprintf("[%d/%d]", len(p.filtered), len(p.all))
	counterStyled := lipgloss.NewStyle().Foreground(colorDim).Render(counter)
	header := p.input.View()
	pad := p.width - lipgloss.Width(header) - lipgloss.Width(counterStyled)
	if pad < 1 {
		pad = 1
	}
	headerLine := header + strings.Repeat(" ", pad) + counterStyled

	rowsN := p.visibleRows()
	if rowsN < 0 {
		rowsN = 0
	}

	var lines []string
	lines = append(lines, headerLine)

	if len(p.filtered) == 0 {
		lines = append(lines, stylePlaceholder.Render("no matches"))
		// pad to fill
		for len(lines) < p.height {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}

	for i := 0; i < rowsN; i++ {
		row := p.offset + i
		if row >= len(p.filtered) {
			lines = append(lines, "")
			continue
		}
		desc := p.all[p.filtered[row]]
		lines = append(lines, p.renderRow(desc, row == p.cursor))
	}

	return strings.Join(lines, "\n")
}

// visibleRows returns how many result rows fit below the header.
func (p Picker) visibleRows() int {
	return p.height - 1
}

func (p *Picker) moveCursor(delta int) {
	if len(p.filtered) == 0 {
		return
	}
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.filtered) {
		p.cursor = len(p.filtered) - 1
	}
	p.ensureVisible()
}

func (p *Picker) ensureVisible() {
	rows := p.visibleRows()
	if rows <= 0 {
		return
	}
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+rows {
		p.offset = p.cursor - rows + 1
	}
	if p.offset < 0 {
		p.offset = 0
	}
}

func (p *Picker) clampCursor() {
	if p.cursor >= len(p.filtered) {
		p.cursor = len(p.filtered) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	p.ensureVisible()
}

func (p *Picker) applyFilter() {
	q := strings.TrimSpace(p.input.Value())
	if q == "" {
		p.filtered = p.filtered[:0]
		for i := range p.all {
			p.filtered = append(p.filtered, i)
		}
	} else {
		matches := fuzzy.Find(q, p.haystack)
		p.filtered = p.filtered[:0]
		for _, m := range matches {
			p.filtered = append(p.filtered, m.Index)
		}
	}
	p.cursor = 0
	p.offset = 0
}

// renderRow formats one command row.
func (p Picker) renderRow(d command.Descriptor, selected bool) string {
	left := fmt.Sprintf("[%s] %s", d.Category, d.Name)
	right := d.Command
	if len(d.Args) > 0 {
		right = d.Command + " " + strings.Join(d.Args, " ")
	}

	// Split the width: left column up to ~half, right column gets the rest.
	avail := p.width - 2 // account for padding
	if avail < 20 {
		avail = 20
	}
	leftW := avail / 2
	rightW := avail - leftW - 2

	leftCol := truncate(left, leftW)
	rightCol := truncate(right, rightW)

	cursor := "  "
	leftStyled := leftCol
	rightStyled := lipgloss.NewStyle().Foreground(colorDim).Render(rightCol)

	if selected {
		cursor = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("▸ ")
		leftStyled = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render(leftCol)
	}

	return cursor + fmt.Sprintf("%-*s  %s", leftW, leftStyled, rightStyled)
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	if w <= 1 {
		return "…"
	}
	// naive truncation by rune count — good enough for ASCII/LGC; polish later
	runes := []rune(s)
	if len(runes) <= w-1 {
		return s
	}
	return string(runes[:w-1]) + "…"
}
