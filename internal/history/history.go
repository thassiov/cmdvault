package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	DefaultConfigDir = ".config/cmdvault"
	HistoryFile      = "history.jsonl"
)

// Entry represents a single command execution
type Entry struct {
	Timestamp time.Time     `json:"timestamp"`
	User      string        `json:"user"`
	Name      string        `json:"name"`
	Command   string        `json:"command"`
	Args      []string      `json:"args"`
	ExitCode  int           `json:"exit_code"`
	Duration  time.Duration `json:"duration_ns"`
	WorkDir   string        `json:"workdir,omitempty"`
}

// History manages command execution history
type History struct {
	path string
}

// New creates a history manager with default path (~/.config/cmdvault/history.jsonl)
func New() (*History, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	dir := filepath.Join(home, DefaultConfigDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	return &History{
		path: filepath.Join(dir, HistoryFile),
	}, nil
}

// NewWithPath creates a history manager with a custom path
func NewWithPath(path string) *History {
	return &History{path: path}
}

// Log appends an entry to the history file
func (h *History) Log(entry Entry) error {
	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}

	return nil
}

// List returns all history entries
func (h *History) List() ([]Entry, error) {
	f, err := os.Open(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// Recent returns the last n entries (most recent first)
func (h *History) Recent(n int) ([]Entry, error) {
	entries, err := h.List()
	if err != nil {
		return nil, err
	}

	// Reverse to get most recent first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	if n > len(entries) {
		n = len(entries)
	}

	return entries[:n], nil
}

// CommandCount represents a command and its execution count
type CommandCount struct {
	Name    string
	Command string
	Count   int
}

// MostUsed returns the n most frequently executed commands
func (h *History) MostUsed(n int) ([]CommandCount, error) {
	entries, err := h.List()
	if err != nil {
		return nil, err
	}

	// Count by command name
	counts := make(map[string]*CommandCount)
	for _, e := range entries {
		key := e.Name
		if cc, ok := counts[key]; ok {
			cc.Count++
		} else {
			counts[key] = &CommandCount{
				Name:    e.Name,
				Command: e.Command,
				Count:   1,
			}
		}
	}

	// Convert to slice and sort
	result := make([]CommandCount, 0, len(counts))
	for _, cc := range counts {
		result = append(result, *cc)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	if n > len(result) {
		n = len(result)
	}

	return result[:n], nil
}

// Clear removes all history
func (h *History) Clear() error {
	return os.Remove(h.path)
}

// Path returns the history file path
func (h *History) Path() string {
	return h.path
}
