package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempHistoryPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "history.jsonl")
}

func sampleEntry(name, command string, exitCode int) Entry {
	return Entry{
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		User:      "testuser",
		Name:      name,
		Command:   command,
		Args:      []string{"-v"},
		ExitCode:  exitCode,
		Duration:  2 * time.Second,
		WorkDir:   "/tmp",
	}
}

func TestNewWithPath(t *testing.T) {
	path := "/tmp/test-history.jsonl"
	h := NewWithPath(path)

	if h.Path() != path {
		t.Errorf("Path() = %q, want %q", h.Path(), path)
	}
}

func TestLogAndList(t *testing.T) {
	h := NewWithPath(tempHistoryPath(t))

	entry := sampleEntry("docker-ps", "docker", 0)
	if err := h.Log(entry); err != nil {
		t.Fatalf("Log() error: %v", err)
	}

	entries, err := h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("List() returned %d entries, want 1", len(entries))
	}

	if entries[0].Name != "docker-ps" {
		t.Errorf("entry.Name = %q, want %q", entries[0].Name, "docker-ps")
	}
	if entries[0].Command != "docker" {
		t.Errorf("entry.Command = %q, want %q", entries[0].Command, "docker")
	}
	if entries[0].ExitCode != 0 {
		t.Errorf("entry.ExitCode = %d, want 0", entries[0].ExitCode)
	}
	if entries[0].User != "testuser" {
		t.Errorf("entry.User = %q, want %q", entries[0].User, "testuser")
	}
}

func TestLogMultiple(t *testing.T) {
	h := NewWithPath(tempHistoryPath(t))

	for i := 0; i < 5; i++ {
		if err := h.Log(sampleEntry("cmd", "echo", 0)); err != nil {
			t.Fatalf("Log() error on entry %d: %v", i, err)
		}
	}

	entries, err := h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("List() returned %d entries, want 5", len(entries))
	}
}

func TestListEmptyFile(t *testing.T) {
	h := NewWithPath(tempHistoryPath(t))

	// File doesn't exist yet — should return nil, nil
	entries, err := h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if entries != nil {
		t.Errorf("List() = %v, want nil for non-existent file", entries)
	}
}

func TestListMalformedLines(t *testing.T) {
	path := tempHistoryPath(t)

	// Write a file with some valid and some malformed lines
	content := `{"timestamp":"2025-01-15T10:30:00Z","user":"test","name":"good","command":"echo","args":["-v"],"exit_code":0,"duration_ns":2000000000}
not-valid-json
{"timestamp":"2025-01-15T10:31:00Z","user":"test","name":"also-good","command":"ls","args":[],"exit_code":0,"duration_ns":1000000000}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	h := NewWithPath(path)
	entries, err := h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// Should skip the malformed line, return 2 valid entries
	if len(entries) != 2 {
		t.Fatalf("List() returned %d entries, want 2 (skipping malformed)", len(entries))
	}
	if entries[0].Name != "good" {
		t.Errorf("entries[0].Name = %q, want %q", entries[0].Name, "good")
	}
	if entries[1].Name != "also-good" {
		t.Errorf("entries[1].Name = %q, want %q", entries[1].Name, "also-good")
	}
}

func TestRecent(t *testing.T) {
	h := NewWithPath(tempHistoryPath(t))

	names := []string{"first", "second", "third", "fourth", "fifth"}
	for _, name := range names {
		if err := h.Log(sampleEntry(name, "echo", 0)); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}

	t.Run("get last 3", func(t *testing.T) {
		recent, err := h.Recent(3)
		if err != nil {
			t.Fatalf("Recent(3) error: %v", err)
		}
		if len(recent) != 3 {
			t.Fatalf("Recent(3) returned %d entries, want 3", len(recent))
		}
		// Most recent first
		if recent[0].Name != "fifth" {
			t.Errorf("recent[0].Name = %q, want %q", recent[0].Name, "fifth")
		}
		if recent[1].Name != "fourth" {
			t.Errorf("recent[1].Name = %q, want %q", recent[1].Name, "fourth")
		}
		if recent[2].Name != "third" {
			t.Errorf("recent[2].Name = %q, want %q", recent[2].Name, "third")
		}
	})

	t.Run("request more than available", func(t *testing.T) {
		recent, err := h.Recent(100)
		if err != nil {
			t.Fatalf("Recent(100) error: %v", err)
		}
		if len(recent) != 5 {
			t.Errorf("Recent(100) returned %d entries, want 5", len(recent))
		}
	})

	t.Run("request zero", func(t *testing.T) {
		recent, err := h.Recent(0)
		if err != nil {
			t.Fatalf("Recent(0) error: %v", err)
		}
		if len(recent) != 0 {
			t.Errorf("Recent(0) returned %d entries, want 0", len(recent))
		}
	})
}

func TestMostUsed(t *testing.T) {
	h := NewWithPath(tempHistoryPath(t))

	// Log commands with different frequencies
	for i := 0; i < 5; i++ {
		h.Log(sampleEntry("docker-ps", "docker", 0))
	}
	for i := 0; i < 3; i++ {
		h.Log(sampleEntry("git-status", "git", 0))
	}
	h.Log(sampleEntry("ls-files", "ls", 0))

	t.Run("top 2", func(t *testing.T) {
		top, err := h.MostUsed(2)
		if err != nil {
			t.Fatalf("MostUsed(2) error: %v", err)
		}
		if len(top) != 2 {
			t.Fatalf("MostUsed(2) returned %d, want 2", len(top))
		}
		if top[0].Name != "docker-ps" || top[0].Count != 5 {
			t.Errorf("top[0] = {%q, %d}, want {%q, 5}", top[0].Name, top[0].Count, "docker-ps")
		}
		if top[1].Name != "git-status" || top[1].Count != 3 {
			t.Errorf("top[1] = {%q, %d}, want {%q, 3}", top[1].Name, top[1].Count, "git-status")
		}
	})

	t.Run("request all", func(t *testing.T) {
		top, err := h.MostUsed(10)
		if err != nil {
			t.Fatalf("MostUsed(10) error: %v", err)
		}
		if len(top) != 3 {
			t.Errorf("MostUsed(10) returned %d, want 3 (only 3 unique commands)", len(top))
		}
	})

	t.Run("empty history", func(t *testing.T) {
		empty := NewWithPath(tempHistoryPath(t))
		top, err := empty.MostUsed(5)
		if err != nil {
			t.Fatalf("MostUsed(5) error: %v", err)
		}
		if len(top) != 0 {
			t.Errorf("MostUsed on empty = %d entries, want 0", len(top))
		}
	})
}

func TestClear(t *testing.T) {
	path := tempHistoryPath(t)
	h := NewWithPath(path)

	// Log something first
	h.Log(sampleEntry("test", "echo", 0))

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("history file should exist after Log()")
	}

	// Clear
	if err := h.Clear(); err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	// File should be gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("history file should not exist after Clear()")
	}

	// List after clear should return nil (file doesn't exist)
	entries, err := h.List()
	if err != nil {
		t.Fatalf("List() after Clear() error: %v", err)
	}
	if entries != nil {
		t.Errorf("List() after Clear() = %v, want nil", entries)
	}
}

func TestClearNonExistent(t *testing.T) {
	h := NewWithPath(tempHistoryPath(t))

	// Clear on non-existent file should return error
	err := h.Clear()
	if err == nil {
		t.Error("Clear() on non-existent file should return error")
	}
}

func TestEntryDurationSerialization(t *testing.T) {
	h := NewWithPath(tempHistoryPath(t))

	entry := sampleEntry("test", "echo", 0)
	entry.Duration = 1500 * time.Millisecond

	if err := h.Log(entry); err != nil {
		t.Fatalf("Log() error: %v", err)
	}

	entries, err := h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if entries[0].Duration != 1500*time.Millisecond {
		t.Errorf("Duration = %v, want %v", entries[0].Duration, 1500*time.Millisecond)
	}
}

func TestEntryWorkDir(t *testing.T) {
	h := NewWithPath(tempHistoryPath(t))

	entry := sampleEntry("test", "echo", 0)
	entry.WorkDir = "/home/user/project"

	if err := h.Log(entry); err != nil {
		t.Fatalf("Log() error: %v", err)
	}

	entries, err := h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if entries[0].WorkDir != "/home/user/project" {
		t.Errorf("WorkDir = %q, want %q", entries[0].WorkDir, "/home/user/project")
	}
}
