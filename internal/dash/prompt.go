package dash

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/thassiov/cmdvault/internal/command"
	"github.com/thassiov/cmdvault/internal/resolve"
)

// promptKind selects how a placeholder is resolved.
type promptKind int

const (
	kindText   promptKind = iota // plain text input, optional default
	kindSource                   // fuzzy-filter over a shell command's output
	kindFile                     // file picker with /, ~, . root switching
)

// promptConfirmedMsg carries the resolved value for the current placeholder.
type promptConfirmedMsg struct{ value string }

// promptCanceledMsg is emitted when the user Escs out of a prompt chain.
type promptCanceledMsg struct{}

// PromptState tracks a chain of placeholder resolutions for a single command.
type PromptState struct {
	desc         command.Descriptor
	placeholders []string          // names in appearance order
	values       map[string]string // resolved values so far
	current      int               // index into placeholders
	prompt       *Prompt           // active widget for placeholders[current]
}

// newPromptState begins a new prompt chain for the given descriptor.
func newPromptState(desc command.Descriptor, placeholders []string) *PromptState {
	return &PromptState{
		desc:         desc,
		placeholders: placeholders,
		values:       make(map[string]string),
		current:      0,
	}
}

// build constructs the Prompt widget for the current placeholder.
func (s *PromptState) build() {
	name := s.placeholders[s.current]
	var config *command.PlaceholderConfig
	if cfg, ok := s.desc.Placeholders[name]; ok {
		c := cfg
		config = &c
	}
	s.prompt = newPrompt(name, config, s.values)
}

// advance moves to the next placeholder. Returns true when all are resolved.
func (s *PromptState) advance(value string) bool {
	s.values[s.placeholders[s.current]] = value
	s.current++
	if s.current >= len(s.placeholders) {
		return true
	}
	s.build()
	return false
}

// Prompt is the widget for one placeholder. It supports three kinds:
// plain text input, fuzzy-filtered source, and a file picker.
type Prompt struct {
	name        string
	description string
	kind        promptKind

	// text
	input textinput.Model

	// source / file (list-based)
	filter    textinput.Model
	items     []string // full list (raw)
	haystack  []string // filtering view (often == items)
	filtered  []int
	cursor    int
	offset    int
	loadError string

	// file-specific
	currentRoot string // "cwd", "home", "root" — tracks which base is loaded

	width, height int
}

// newPrompt builds a widget for the named placeholder using its config.
// If config is nil, falls back to a text prompt with no default.
func newPrompt(name string, config *command.PlaceholderConfig, resolved map[string]string) *Prompt {
	p := &Prompt{name: name}

	if config != nil {
		p.description = config.Description
		// Expand {{other}} refs in the default.
		if config.Default != "" {
			config.Default = resolve.ExpandDefaultTemplate(config.Default, resolved)
		}
	}

	switch {
	case config != nil && config.Type == "file":
		p.kind = kindFile
		fi := textinput.New()
		fi.Prompt = "filter › "
		fi.Placeholder = "type to filter, / or ~ to switch root"
		fi.Focus()
		if config.Default != "" {
			fi.SetValue(config.Default)
		}
		p.filter = fi
		p.loadFileRoot(detectFileRoot(fi.Value()))

	case config != nil && config.Source != "":
		p.kind = kindSource
		fi := textinput.New()
		fi.Prompt = "filter › "
		fi.Placeholder = "type to filter"
		fi.Focus()
		p.filter = fi
		p.loadSource(config.Source)

	default:
		p.kind = kindText
		ti := textinput.New()
		ti.Prompt = p.promptLabel() + " › "
		if config != nil && config.Default != "" {
			ti.Placeholder = config.Default
		}
		ti.Focus()
		p.input = ti
	}

	return p
}

// promptLabel composes "name (description)" or just "name" for the input prompt.
func (p Prompt) promptLabel() string {
	if p.description != "" {
		return fmt.Sprintf("%s (%s)", p.name, p.description)
	}
	return p.name
}

// SetSize updates the prompt's drawable area.
func (p *Prompt) SetSize(w, h int) {
	p.width = w
	p.height = h
	switch p.kind {
	case kindText:
		p.input.Width = w - len(p.input.Prompt) - 2
	case kindSource, kindFile:
		p.filter.Width = w - len(p.filter.Prompt) - 20
		p.clampCursor()
	}
}

// Update handles key events and emits promptConfirmedMsg / promptCanceledMsg.
func (p *Prompt) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyMsg); ok {
		if cmd, handled := p.handleControlKey(key); handled {
			return cmd
		}
	}
	return p.forwardToInput(msg)
}

// handleControlKey handles keys that aren't typed into the input field
// (Enter, Esc, arrow keys in list mode). Returns handled=false when the
// key should pass through to the input.
func (p *Prompt) handleControlKey(key tea.KeyMsg) (tea.Cmd, bool) {
	switch key.String() {
	case "esc":
		return func() tea.Msg { return promptCanceledMsg{} }, true
	case "enter":
		return p.confirm(), true
	case "up", "ctrl+p":
		if p.kind != kindText {
			p.moveCursor(-1)
			return nil, true
		}
	case "down", "ctrl+n":
		if p.kind != kindText {
			p.moveCursor(1)
			return nil, true
		}
	case "pgup":
		if p.kind != kindText {
			p.moveCursor(-p.visibleRows())
			return nil, true
		}
	case "pgdown":
		if p.kind != kindText {
			p.moveCursor(p.visibleRows())
			return nil, true
		}
	}
	return nil, false
}

// forwardToInput routes the message to the underlying text input (either
// the text prompt's input or the list's filter field), then refreshes the
// filter / root if the query changed.
func (p *Prompt) forwardToInput(msg tea.Msg) tea.Cmd {
	if p.kind == kindText {
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		return cmd
	}

	prev := p.filter.Value()
	var cmd tea.Cmd
	p.filter, cmd = p.filter.Update(msg)
	if p.filter.Value() == prev {
		return cmd
	}
	if p.kind == kindFile {
		if want := detectFileRoot(p.filter.Value()); want != p.currentRoot {
			p.loadFileRoot(want)
		}
	}
	p.applyFilter()
	return cmd
}

// confirm returns the confirmation message with the selected value.
func (p *Prompt) confirm() tea.Cmd {
	switch p.kind {
	case kindText:
		val := p.input.Value()
		if val == "" && p.input.Placeholder != "" {
			val = p.input.Placeholder
		}
		return func() tea.Msg { return promptConfirmedMsg{value: val} }
	case kindSource, kindFile:
		if len(p.filtered) == 0 || p.cursor < 0 || p.cursor >= len(p.filtered) {
			// Fall back to the filter text literally if no match selected.
			return func() tea.Msg { return promptConfirmedMsg{value: p.filter.Value()} }
		}
		sel := p.items[p.filtered[p.cursor]]
		if p.kind == kindFile {
			sel = expandFilePath(sel)
		}
		return func() tea.Msg { return promptConfirmedMsg{value: sel} }
	}
	return nil
}

// View renders the prompt for the current placeholder.
func (p Prompt) View() string {
	switch p.kind {
	case kindText:
		header := stylePromptHeader.Render(p.promptLabel())
		return header + "\n" + p.input.View()
	case kindSource, kindFile:
		return p.viewList()
	}
	return ""
}

func (p Prompt) viewList() string {
	header := stylePromptHeader.Render(p.promptLabel())
	if p.kind == kindFile {
		header += " " + lipgloss.NewStyle().Foreground(colorDim).Render("["+p.currentRoot+"]")
	}
	counter := lipgloss.NewStyle().Foreground(colorDim).
		Render(fmt.Sprintf("[%d/%d]", len(p.filtered), len(p.items)))

	filterLine := p.filter.View()
	pad := p.width - lipgloss.Width(filterLine) - lipgloss.Width(counter)
	if pad < 1 {
		pad = 1
	}
	filterLine = filterLine + strings.Repeat(" ", pad) + counter

	lines := []string{header, filterLine}

	if p.loadError != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("! "+p.loadError))
		return strings.Join(lines, "\n")
	}

	rows := p.visibleRows() - 1 // minus one for header offset already counted? no — visibleRows is for items area
	if rows < 0 {
		rows = 0
	}

	if len(p.filtered) == 0 {
		lines = append(lines, stylePlaceholder.Render("no matches"))
		return strings.Join(lines, "\n")
	}

	for i := 0; i < rows; i++ {
		row := p.offset + i
		if row >= len(p.filtered) {
			lines = append(lines, "")
			continue
		}
		val := p.items[p.filtered[row]]
		selected := row == p.cursor
		if selected {
			lines = append(lines, lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("▸ "+val))
		} else {
			lines = append(lines, "  "+val)
		}
	}
	return strings.Join(lines, "\n")
}

// visibleRows returns rows available for result items (excludes header + filter).
func (p Prompt) visibleRows() int {
	return p.height - 2
}

func (p *Prompt) applyFilter() {
	q := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(p.filter.Value(), "/"), "~"))
	if q == "" {
		p.filtered = p.filtered[:0]
		for i := range p.items {
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

func (p *Prompt) moveCursor(delta int) {
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
}

func (p *Prompt) clampCursor() {
	if p.cursor >= len(p.filtered) {
		p.cursor = len(p.filtered) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

// loadSource runs the source shell command and populates items.
func (p *Prompt) loadSource(source string) {
	out, err := exec.Command("sh", "-c", source).Output()
	if err != nil {
		p.loadError = "source command failed: " + err.Error()
		return
	}
	for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if l == "" {
			continue
		}
		p.items = append(p.items, l)
	}
	p.haystack = p.items
	p.applyFilter()
}

// loadFileRoot walks files under the given root and populates items.
// The root identifier is one of "cwd", "home", "root".
func (p *Prompt) loadFileRoot(root string) {
	p.currentRoot = root
	p.items = p.items[:0]
	p.loadError = ""

	var base string
	switch root {
	case "root":
		base = "/"
	case "home":
		h, err := os.UserHomeDir()
		if err != nil {
			p.loadError = "no HOME set"
			return
		}
		base = h
	default:
		cwd, err := os.Getwd()
		if err != nil {
			p.loadError = "getcwd failed: " + err.Error()
			return
		}
		base = cwd
	}

	files, err := walkFiles(base, 6, 50000)
	if err != nil {
		p.loadError = "walk failed: " + err.Error()
		return
	}
	p.items = files
	// haystack strips the home prefix / leading slash for nicer matching
	p.haystack = make([]string, len(files))
	copy(p.haystack, files)
	p.applyFilter()
}

// walkFiles returns a flat list of file paths under root, bounded by maxDepth
// relative to root and a total count cap. Returns paths with ~/ shorthand
// when under $HOME.
func walkFiles(root string, maxDepth, limit int) ([]string, error) {
	home, _ := os.UserHomeDir()
	var out []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries rather than abort
		}
		if len(out) >= limit {
			return filepath.SkipAll
		}
		if d.IsDir() {
			return dirWalkAction(root, path, d, maxDepth)
		}
		out = append(out, displayPath(path, home))
		return nil
	})
	return out, err
}

// dirWalkAction decides whether to descend into, skip, or ignore a directory
// during walkFiles.
func dirWalkAction(root, path string, d os.DirEntry, maxDepth int) error {
	if d.Name() == ".git" || d.Name() == "node_modules" {
		return filepath.SkipDir
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return nil
	}
	if strings.Count(rel, string(filepath.Separator))+1 > maxDepth {
		return filepath.SkipDir
	}
	return nil
}

// displayPath formats a path with ~/ shorthand when it lives under $HOME.
func displayPath(path, home string) string {
	if home != "" && strings.HasPrefix(path, home+"/") {
		return "~/" + path[len(home)+1:]
	}
	return path
}

// detectFileRoot inspects the leading chars of a filter query and decides
// which root the file listing should draw from.
func detectFileRoot(q string) string {
	if strings.HasPrefix(q, "/") {
		return "root"
	}
	if strings.HasPrefix(q, "~") {
		return "home"
	}
	return "cwd"
}

// expandFilePath resolves ~/... to an absolute home-relative path.
func expandFilePath(p string) string {
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

var stylePromptHeader = lipgloss.NewStyle().
	Foreground(colorAccent).
	Bold(true)
