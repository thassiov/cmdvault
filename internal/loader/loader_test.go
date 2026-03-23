package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thassiov/cmdvault/internal/command"
)

func TestDeriveCategory(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		baseDir  string
		expected string
	}{
		{
			name:     "flat file with base dir",
			path:     "/commands/docker.yaml",
			baseDir:  "/commands",
			expected: "docker",
		},
		{
			name:     "one level deep",
			path:     "/commands/grid/health.yaml",
			baseDir:  "/commands",
			expected: "grid/health",
		},
		{
			name:     "two levels deep",
			path:     "/commands/grid/opnsense/dnsbl.yaml",
			baseDir:  "/commands",
			expected: "grid/opnsense/dnsbl",
		},
		{
			name:     "empty base dir falls back to filename",
			path:     "/anywhere/grid/opnsense/dnsbl.yaml",
			baseDir:  "",
			expected: "dnsbl",
		},
		{
			name:     "yml extension",
			path:     "/commands/grid/tools.yml",
			baseDir:  "/commands",
			expected: "grid/tools",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveCategory(tt.path, tt.baseDir)
			if got != tt.expected {
				t.Errorf("deriveCategory(%q, %q) = %q, want %q", tt.path, tt.baseDir, got, tt.expected)
			}
		})
	}
}

func TestLoadDirRecursiveCategories(t *testing.T) {
	// Create a temp directory structure:
	//   root/
	//     flat.yaml          → category "flat"
	//     sub/
	//       nested.yaml      → category "sub/nested"
	//     sub/deep/
	//       deep.yaml        → category "sub/deep/deep"
	root := t.TempDir()

	// flat.yaml at root
	writeTestYAML(t, filepath.Join(root, "flat.yaml"), "flat command")

	// sub/nested.yaml
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeTestYAML(t, filepath.Join(root, "sub", "nested.yaml"), "nested command")

	// sub/deep/deep.yaml
	if err := os.MkdirAll(filepath.Join(root, "sub", "deep"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeTestYAML(t, filepath.Join(root, "sub", "deep", "deep.yaml"), "deep command")

	loader := NewWithPath(root)
	commands, err := loader.LoadDirRecursive(root)
	if err != nil {
		t.Fatalf("LoadDirRecursive: %v", err)
	}

	if len(commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(commands))
	}

	categories := map[string]bool{}
	for _, cmd := range commands {
		categories[cmd.Category] = true
	}

	expected := []string{"flat", "sub/nested", "sub/deep/deep"}
	for _, exp := range expected {
		if !categories[exp] {
			t.Errorf("expected category %q, got categories: %v", exp, categories)
		}
	}
}

func writeTestYAML(t *testing.T, path, name string) {
	t.Helper()
	content := []byte("commands:\n  - name: " + name + "\n    command: echo\n    args: [\"test\"]\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no change needed",
			input:    "a normal string",
			expected: "a normal string",
		},
		{
			name:     "newline collapsed",
			input:    "line one\nline two",
			expected: "line one line two",
		},
		{
			name:     "multiple newlines",
			input:    "one\ntwo\nthree\nfour",
			expected: "one two three four",
		},
		{
			name:     "carriage return and newline",
			input:    "line one\r\nline two",
			expected: "line one line two",
		},
		{
			name:     "tabs collapsed",
			input:    "col1\tcol2\tcol3",
			expected: "col1 col2 col3",
		},
		{
			name:     "multiple spaces collapsed",
			input:    "too   many    spaces",
			expected: "too many spaces",
		},
		{
			name:     "leading and trailing whitespace trimmed",
			input:    "  padded  ",
			expected: "padded",
		},
		{
			name:     "trailing newline (YAML folded/literal block)",
			input:    "description from yaml block\n",
			expected: "description from yaml block",
		},
		{
			name:     "mixed whitespace chaos",
			input:    "\n\tfoo\n  bar \t baz\n",
			expected: "foo bar baz",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "  \n\t\n  ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitize(tt.input)
			if got != tt.expected {
				t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLoadMissingName(t *testing.T) {
	t.Run("description promoted to name", func(t *testing.T) {
		root := t.TempDir()
		content := []byte(`commands:
  - command: echo
    args: ["hello"]
    description: "no name provided"
`)
		path := filepath.Join(root, "test.yaml")
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		loader := NewWithPath(root)
		commands, err := loader.LoadFile(path)
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}

		if len(commands) != 1 {
			t.Fatalf("expected 1 command, got %d", len(commands))
		}

		// Description should be promoted to name
		if commands[0].Name != "no name provided" {
			t.Errorf("expected name %q, got %q", "no name provided", commands[0].Name)
		}

		// Description should be auto-generated from command+args
		if commands[0].Description != "echo hello" {
			t.Errorf("expected description %q, got %q", "echo hello", commands[0].Description)
		}

		// Alias should be derived from the promoted name
		if commands[0].Alias != "no-name-provided" {
			t.Errorf("expected alias %q, got %q", "no-name-provided", commands[0].Alias)
		}
	})

	t.Run("no name no description falls back to filename#index", func(t *testing.T) {
		root := t.TempDir()
		content := []byte(`commands:
  - command: echo
    args: ["hello"]
  - command: ls
    args: ["-la"]
`)
		path := filepath.Join(root, "test.yaml")
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		loader := NewWithPath(root)
		commands, err := loader.LoadFile(path)
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}

		if len(commands) != 2 {
			t.Fatalf("expected 2 commands, got %d", len(commands))
		}

		// Both names should be filename#index
		if commands[0].Name != "test.yaml#0" {
			t.Errorf("expected name %q, got %q", "test.yaml#0", commands[0].Name)
		}
		if commands[1].Name != "test.yaml#1" {
			t.Errorf("expected name %q, got %q", "test.yaml#1", commands[1].Name)
		}

		// Descriptions should be auto-generated from command+args
		if commands[0].Description != "echo hello" {
			t.Errorf("expected description %q, got %q", "echo hello", commands[0].Description)
		}
		if commands[1].Description != "ls -la" {
			t.Errorf("expected description %q, got %q", "ls -la", commands[1].Description)
		}
	})
}

func TestLoadMissingDescription(t *testing.T) {
	root := t.TempDir()
	content := []byte(`commands:
  - name: "no desc"
    command: echo
    args: ["hello", "world"]

  - name: "no desc no args"
    command: ls
`)
	path := filepath.Join(root, "test.yaml")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loader := NewWithPath(root)
	commands, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	if len(commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(commands))
	}

	// Description should be generated from command + args
	if commands[0].Description != "echo hello world" {
		t.Errorf("expected description %q, got %q", "echo hello world", commands[0].Description)
	}

	// No args: description is just the command
	if commands[1].Description != "ls" {
		t.Errorf("expected description %q, got %q", "ls", commands[1].Description)
	}
}

func TestLoadMissingCommand(t *testing.T) {
	root := t.TempDir()
	content := []byte(`commands:
  - name: "valid"
    command: echo
    args: ["test"]

  - name: "no command"
    description: "this has no command field"

  - name: "also valid"
    command: ls
`)
	path := filepath.Join(root, "test.yaml")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loader := NewWithPath(root)
	commands, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	// Entry without command should be skipped
	if len(commands) != 2 {
		t.Fatalf("expected 2 commands (1 skipped), got %d", len(commands))
	}

	if commands[0].Name != "valid" {
		t.Errorf("expected first command %q, got %q", "valid", commands[0].Name)
	}
	if commands[1].Name != "also valid" {
		t.Errorf("expected second command %q, got %q", "also valid", commands[1].Name)
	}
}

func TestLoadMultilineDescription(t *testing.T) {
	root := t.TempDir()
	// YAML folded (>) and literal (|) block scalars
	content := []byte(`commands:
  - name: "folded"
    command: echo
    description: >
      This is a folded description
      that spans multiple lines
      in the YAML source.

  - name: "literal"
    command: echo
    description: |
      This is a literal block
      with actual newlines
      preserved in the string.

  - name: "plain wrap"
    command: echo
    description:
      This is a plain scalar
      that wraps across lines.
`)
	path := filepath.Join(root, "test.yaml")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loader := NewWithPath(root)
	commands, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	if len(commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(commands))
	}

	// All descriptions should be single-line with no newlines
	for _, cmd := range commands {
		for _, c := range cmd.Description {
			if c == '\n' || c == '\r' || c == '\t' {
				t.Errorf("command %q description contains whitespace char %q: %q",
					cmd.Name, string(c), cmd.Description)
			}
		}
	}

	// Folded block: newlines become spaces
	expected := "This is a folded description that spans multiple lines in the YAML source."
	if commands[0].Description != expected {
		t.Errorf("folded: got %q, want %q", commands[0].Description, expected)
	}

	// Literal block: newlines collapsed to spaces
	expected = "This is a literal block with actual newlines preserved in the string."
	if commands[1].Description != expected {
		t.Errorf("literal: got %q, want %q", commands[1].Description, expected)
	}

	// Plain wrapped: YAML already joins these
	expected = "This is a plain scalar that wraps across lines."
	if commands[2].Description != expected {
		t.Errorf("plain: got %q, want %q", commands[2].Description, expected)
	}
}

func TestLoadEmptyDescription(t *testing.T) {
	root := t.TempDir()
	content := []byte(`commands:
  - name: "empty string"
    command: echo
    args: ["hello"]
    description: ""

  - name: "missing field"
    command: echo
    args: ["world"]
`)
	path := filepath.Join(root, "test.yaml")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loader := NewWithPath(root)
	commands, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	if len(commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(commands))
	}

	// Both should get auto-generated description
	if commands[0].Description != "echo hello" {
		t.Errorf("empty string desc: got %q, want %q", commands[0].Description, "echo hello")
	}
	if commands[1].Description != "echo world" {
		t.Errorf("missing field desc: got %q, want %q", commands[1].Description, "echo world")
	}
}

func TestLoadPlaceholderConfig(t *testing.T) {
	t.Run("type and description fields", func(t *testing.T) {
		root := t.TempDir()
		content := []byte(`commands:
  - name: strip metadata
    command: ffmpeg
    args: ["-i", "{{input}}", "-map_metadata", "-1", "-c", "copy", "{{output}}"]
    placeholders:
      input:
        type: file
        description: source media file
      output:
        description: output file path
        default: "{{input}}"
`)
		path := filepath.Join(root, "test.yaml")
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		loader := NewWithPath(root)
		commands, err := loader.LoadFile(path)
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}

		if len(commands) != 1 {
			t.Fatalf("expected 1 command, got %d", len(commands))
		}

		ph := commands[0].Placeholders
		if ph == nil {
			t.Fatal("expected placeholders, got nil")
		}

		// Input placeholder: type=file, description set, no default
		input, ok := ph["input"]
		if !ok {
			t.Fatal("expected 'input' placeholder")
		}
		if input.Type != "file" {
			t.Errorf("input.Type = %q, want %q", input.Type, "file")
		}
		if input.Description != "source media file" {
			t.Errorf("input.Description = %q, want %q", input.Description, "source media file")
		}
		if input.Default != "" {
			t.Errorf("input.Default = %q, want empty", input.Default)
		}

		// Output placeholder: no type, description set, default references input
		output, ok := ph["output"]
		if !ok {
			t.Fatal("expected 'output' placeholder")
		}
		if output.Type != "" {
			t.Errorf("output.Type = %q, want empty", output.Type)
		}
		if output.Description != "output file path" {
			t.Errorf("output.Description = %q, want %q", output.Description, "output file path")
		}
		if output.Default != "{{input}}" {
			t.Errorf("output.Default = %q, want %q", output.Default, "{{input}}")
		}
	})

	t.Run("source field still works", func(t *testing.T) {
		root := t.TempDir()
		content := []byte(`commands:
  - name: attach session
    command: tmux
    args: ["attach", "-t", "{{session}}"]
    placeholders:
      session:
        source: "tmux list-sessions -F '#S' 2>/dev/null"
`)
		path := filepath.Join(root, "test.yaml")
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		loader := NewWithPath(root)
		commands, err := loader.LoadFile(path)
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}

		ph := commands[0].Placeholders
		session, ok := ph["session"]
		if !ok {
			t.Fatal("expected 'session' placeholder")
		}
		if session.Source != "tmux list-sessions -F '#S' 2>/dev/null" {
			t.Errorf("session.Source = %q, unexpected", session.Source)
		}
		if session.Type != "" {
			t.Errorf("session.Type = %q, want empty", session.Type)
		}
	})
}

// --- New tests for untested public functions ---

func TestLoad(t *testing.T) {
	t.Run("empty path loads from default commands dir", func(t *testing.T) {
		root := t.TempDir()
		writeTestYAML(t, filepath.Join(root, "cmd.yaml"), "default dir cmd")

		loader := NewWithPath(root)
		commands, err := loader.Load("")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}

		if len(commands) != 1 {
			t.Fatalf("expected 1 command, got %d", len(commands))
		}
		if commands[0].Name != "default dir cmd" {
			t.Errorf("expected name %q, got %q", "default dir cmd", commands[0].Name)
		}
	})

	t.Run("file path loads single file", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "single.yaml")
		writeTestYAML(t, path, "single file cmd")

		loader := NewWithPath(root)
		commands, err := loader.Load(path)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}

		if len(commands) != 1 {
			t.Fatalf("expected 1 command, got %d", len(commands))
		}
		if commands[0].Name != "single file cmd" {
			t.Errorf("expected name %q, got %q", "single file cmd", commands[0].Name)
		}
	})

	t.Run("directory path loads recursively", func(t *testing.T) {
		root := t.TempDir()
		writeTestYAML(t, filepath.Join(root, "top.yaml"), "top cmd")
		subdir := filepath.Join(root, "sub")
		if err := os.MkdirAll(subdir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		writeTestYAML(t, filepath.Join(subdir, "nested.yaml"), "nested cmd")

		loader := NewWithPath(root)
		commands, err := loader.Load(root)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}

		if len(commands) != 2 {
			t.Fatalf("expected 2 commands, got %d", len(commands))
		}
	})

	t.Run("nonexistent path returns error", func(t *testing.T) {
		root := t.TempDir()
		loader := NewWithPath(root)
		_, err := loader.Load(filepath.Join(root, "does-not-exist.yaml"))
		if err == nil {
			t.Fatal("expected error for nonexistent path, got nil")
		}
	})
}

func TestLoadDir(t *testing.T) {
	t.Run("loads yaml files recursively from commands dir", func(t *testing.T) {
		root := t.TempDir()
		writeTestYAML(t, filepath.Join(root, "a.yaml"), "cmd a")

		subdir := filepath.Join(root, "nested")
		if err := os.MkdirAll(subdir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		writeTestYAML(t, filepath.Join(subdir, "b.yaml"), "cmd b")

		loader := NewWithPath(root)
		commands, err := loader.LoadDir()
		if err != nil {
			t.Fatalf("LoadDir: %v", err)
		}

		if len(commands) != 2 {
			t.Fatalf("expected 2 commands, got %d", len(commands))
		}
	})

	t.Run("nonexistent commands dir returns error", func(t *testing.T) {
		loader := NewWithPath(filepath.Join(t.TempDir(), "nope"))
		_, err := loader.LoadDir()
		if err == nil {
			t.Fatal("expected error for nonexistent dir, got nil")
		}
	})
}

func TestLoadDirFrom(t *testing.T) {
	t.Run("loads only top-level yaml files", func(t *testing.T) {
		root := t.TempDir()
		writeTestYAML(t, filepath.Join(root, "top.yaml"), "top level")

		subdir := filepath.Join(root, "sub")
		if err := os.MkdirAll(subdir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		writeTestYAML(t, filepath.Join(subdir, "nested.yaml"), "nested level")

		loader := NewWithPath(root)
		commands, err := loader.LoadDirFrom(root)
		if err != nil {
			t.Fatalf("LoadDirFrom: %v", err)
		}

		// Should only get the top-level file, not the nested one
		if len(commands) != 1 {
			t.Fatalf("expected 1 command (non-recursive), got %d", len(commands))
		}
		if commands[0].Name != "top level" {
			t.Errorf("expected name %q, got %q", "top level", commands[0].Name)
		}
	})

	t.Run("skips non-yaml files", func(t *testing.T) {
		root := t.TempDir()
		writeTestYAML(t, filepath.Join(root, "valid.yaml"), "valid")
		if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("not yaml"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		loader := NewWithPath(root)
		commands, err := loader.LoadDirFrom(root)
		if err != nil {
			t.Fatalf("LoadDirFrom: %v", err)
		}

		if len(commands) != 1 {
			t.Fatalf("expected 1 command, got %d", len(commands))
		}
	})

	t.Run("nonexistent dir returns error", func(t *testing.T) {
		loader := NewWithPath(t.TempDir())
		_, err := loader.LoadDirFrom(filepath.Join(t.TempDir(), "missing"))
		if err == nil {
			t.Fatal("expected error for nonexistent dir, got nil")
		}
	})

	t.Run("empty dir returns no commands", func(t *testing.T) {
		root := t.TempDir()
		loader := NewWithPath(root)
		commands, err := loader.LoadDirFrom(root)
		if err != nil {
			t.Fatalf("LoadDirFrom: %v", err)
		}
		if len(commands) != 0 {
			t.Fatalf("expected 0 commands, got %d", len(commands))
		}
	})
}

func TestEnsureDefaultDirs(t *testing.T) {
	t.Run("creates commands directory", func(t *testing.T) {
		root := t.TempDir()
		cmdDir := filepath.Join(root, "config", "commands")
		loader := NewWithPath(cmdDir)

		if err := loader.EnsureDefaultDirs(); err != nil {
			t.Fatalf("EnsureDefaultDirs: %v", err)
		}

		info, err := os.Stat(cmdDir)
		if err != nil {
			t.Fatalf("commands dir does not exist after EnsureDefaultDirs: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected commands path to be a directory")
		}
	})

	t.Run("idempotent when dir already exists", func(t *testing.T) {
		root := t.TempDir()
		cmdDir := filepath.Join(root, "existing")
		if err := os.MkdirAll(cmdDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		loader := NewWithPath(cmdDir)
		if err := loader.EnsureDefaultDirs(); err != nil {
			t.Fatalf("EnsureDefaultDirs on existing dir: %v", err)
		}
	})
}

func TestDefaultDirExists(t *testing.T) {
	t.Run("returns true when dir exists", func(t *testing.T) {
		root := t.TempDir()
		loader := NewWithPath(root)
		if !loader.DefaultDirExists() {
			t.Error("expected true for existing directory")
		}
	})

	t.Run("returns false when dir does not exist", func(t *testing.T) {
		loader := NewWithPath(filepath.Join(t.TempDir(), "nonexistent"))
		if loader.DefaultDirExists() {
			t.Error("expected false for nonexistent directory")
		}
	})
}

func TestGetCommandsDir(t *testing.T) {
	path := "/some/custom/path"
	loader := NewWithPath(path)
	if got := loader.GetCommandsDir(); got != path {
		t.Errorf("GetCommandsDir() = %q, want %q", got, path)
	}
}

func TestCopyExamples(t *testing.T) {
	t.Run("copies embedded example files to examples subdir", func(t *testing.T) {
		root := t.TempDir()
		loader := NewWithPath(root)

		if err := loader.CopyExamples(); err != nil {
			t.Fatalf("CopyExamples: %v", err)
		}

		examplesDir := filepath.Join(root, "examples")
		entries, err := os.ReadDir(examplesDir)
		if err != nil {
			t.Fatalf("ReadDir examples: %v", err)
		}

		if len(entries) == 0 {
			t.Fatal("expected example files to be copied, got none")
		}

		// Verify known example files exist
		expectedFiles := map[string]bool{
			"docker.yaml": false,
			"gh.yaml":     false,
			"system.yaml": false,
		}
		for _, e := range entries {
			expectedFiles[e.Name()] = true
		}
		for name, found := range expectedFiles {
			if !found {
				t.Errorf("expected example file %q not found", name)
			}
		}

		// Verify files have content (not empty)
		for _, e := range entries {
			info, _ := e.Info()
			if info.Size() == 0 {
				t.Errorf("example file %q is empty", e.Name())
			}
		}
	})

	t.Run("does not overwrite existing files", func(t *testing.T) {
		root := t.TempDir()
		loader := NewWithPath(root)

		examplesDir := filepath.Join(root, "examples")
		if err := os.MkdirAll(examplesDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		// Write a custom file with the same name as an example
		customContent := []byte("custom content that should be preserved")
		customPath := filepath.Join(examplesDir, "docker.yaml")
		if err := os.WriteFile(customPath, customContent, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		if err := loader.CopyExamples(); err != nil {
			t.Fatalf("CopyExamples: %v", err)
		}

		// Verify the custom file was NOT overwritten
		data, err := os.ReadFile(customPath)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(data) != string(customContent) {
			t.Error("CopyExamples overwrote an existing file")
		}
	})
}

func TestEnsureDefaultDirsWithExamples(t *testing.T) {
	t.Run("creates dir and copies examples when empty", func(t *testing.T) {
		root := t.TempDir()
		cmdDir := filepath.Join(root, "commands")
		loader := NewWithPath(cmdDir)

		if err := loader.EnsureDefaultDirsWithExamples(); err != nil {
			t.Fatalf("EnsureDefaultDirsWithExamples: %v", err)
		}

		// Commands dir should exist
		if _, err := os.Stat(cmdDir); err != nil {
			t.Fatalf("commands dir not created: %v", err)
		}

		// Examples should be copied
		examplesDir := filepath.Join(cmdDir, "examples")
		entries, err := os.ReadDir(examplesDir)
		if err != nil {
			t.Fatalf("ReadDir examples: %v", err)
		}
		if len(entries) == 0 {
			t.Error("expected example files to be present")
		}
	})

	t.Run("skips examples when yaml files already exist", func(t *testing.T) {
		root := t.TempDir()
		cmdDir := filepath.Join(root, "commands")
		if err := os.MkdirAll(cmdDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		// Create an existing yaml file
		writeTestYAML(t, filepath.Join(cmdDir, "existing.yaml"), "existing")

		loader := NewWithPath(cmdDir)
		if err := loader.EnsureDefaultDirsWithExamples(); err != nil {
			t.Fatalf("EnsureDefaultDirsWithExamples: %v", err)
		}

		// Examples dir should NOT be created since yaml files already exist
		examplesDir := filepath.Join(cmdDir, "examples")
		if _, err := os.Stat(examplesDir); err == nil {
			t.Error("expected examples dir to not be created when yaml files exist")
		}
	})
}

func TestListExamples(t *testing.T) {
	names, err := ListExamples()
	if err != nil {
		t.Fatalf("ListExamples: %v", err)
	}

	if len(names) == 0 {
		t.Fatal("expected at least one example file name")
	}

	// Verify known examples are present
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}

	for _, expected := range []string{"docker.yaml", "gh.yaml", "system.yaml"} {
		if !found[expected] {
			t.Errorf("expected %q in ListExamples output, got: %v", expected, names)
		}
	}
}

func TestNormalizeDescriptor(t *testing.T) {
	t.Run("name present keeps name and generates alias", func(t *testing.T) {
		cmd := command.Descriptor{
			Name:    "Run Tests",
			Command: "go",
			Args:    []string{"test", "./..."},
		}
		normalizeDescriptor(&cmd, "testing", "test.yaml", 0)

		if cmd.Name != "Run Tests" {
			t.Errorf("name = %q, want %q", cmd.Name, "Run Tests")
		}
		if cmd.Alias != "run-tests" {
			t.Errorf("alias = %q, want %q", cmd.Alias, "run-tests")
		}
		if cmd.Description != "go test ./..." {
			t.Errorf("description = %q, want %q", cmd.Description, "go test ./...")
		}
		if cmd.Category != "testing" {
			t.Errorf("category = %q, want %q", cmd.Category, "testing")
		}
	})

	t.Run("no name promotes description to name", func(t *testing.T) {
		cmd := command.Descriptor{
			Command:     "docker",
			Args:        []string{"ps"},
			Description: "List Containers",
		}
		normalizeDescriptor(&cmd, "docker", "docker.yaml", 0)

		if cmd.Name != "List Containers" {
			t.Errorf("name = %q, want %q", cmd.Name, "List Containers")
		}
		// Description was promoted, so it should be regenerated
		if cmd.Description != "docker ps" {
			t.Errorf("description = %q, want %q", cmd.Description, "docker ps")
		}
		if cmd.Alias != "list-containers" {
			t.Errorf("alias = %q, want %q", cmd.Alias, "list-containers")
		}
	})

	t.Run("no name no description uses filename#index", func(t *testing.T) {
		cmd := command.Descriptor{
			Command: "ls",
		}
		normalizeDescriptor(&cmd, "misc", "tools.yaml", 3)

		if cmd.Name != "tools.yaml#3" {
			t.Errorf("name = %q, want %q", cmd.Name, "tools.yaml#3")
		}
		if cmd.Description != "ls" {
			t.Errorf("description = %q, want %q", cmd.Description, "ls")
		}
	})

	t.Run("explicit alias is preserved", func(t *testing.T) {
		cmd := command.Descriptor{
			Name:    "Some Command",
			Command: "echo",
			Alias:   "custom-alias",
		}
		normalizeDescriptor(&cmd, "cat", "test.yaml", 0)

		if cmd.Alias != "custom-alias" {
			t.Errorf("alias = %q, want %q", cmd.Alias, "custom-alias")
		}
	})

	t.Run("description with args is command plus args", func(t *testing.T) {
		cmd := command.Descriptor{
			Name:    "Build",
			Command: "make",
			Args:    []string{"-j4", "all"},
		}
		normalizeDescriptor(&cmd, "build", "build.yaml", 0)

		if cmd.Description != "make -j4 all" {
			t.Errorf("description = %q, want %q", cmd.Description, "make -j4 all")
		}
	})

	t.Run("description without args is just command", func(t *testing.T) {
		cmd := command.Descriptor{
			Name:    "Status",
			Command: "git status",
		}
		normalizeDescriptor(&cmd, "git", "git.yaml", 0)

		if cmd.Description != "git status" {
			t.Errorf("description = %q, want %q", cmd.Description, "git status")
		}
	})

	t.Run("name with whitespace is sanitized", func(t *testing.T) {
		cmd := command.Descriptor{
			Name:    "multi\nline\tname  here",
			Command: "echo",
		}
		normalizeDescriptor(&cmd, "test", "test.yaml", 0)

		if cmd.Name != "multi line name here" {
			t.Errorf("name = %q, want %q", cmd.Name, "multi line name here")
		}
		if cmd.Alias != "multi-line-name-here" {
			t.Errorf("alias = %q, want %q", cmd.Alias, "multi-line-name-here")
		}
	})

	t.Run("existing description is kept and sanitized", func(t *testing.T) {
		cmd := command.Descriptor{
			Name:        "Test",
			Command:     "echo",
			Description: "a\nmultiline\tdescription",
		}
		normalizeDescriptor(&cmd, "test", "test.yaml", 0)

		if cmd.Description != "a multiline description" {
			t.Errorf("description = %q, want %q", cmd.Description, "a multiline description")
		}
	})
}
