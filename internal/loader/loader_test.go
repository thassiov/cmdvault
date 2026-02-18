package loader

import (
	"os"
	"path/filepath"
	"testing"
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
	os.MkdirAll(filepath.Join(root, "sub"), 0755)
	writeTestYAML(t, filepath.Join(root, "sub", "nested.yaml"), "nested command")

	// sub/deep/deep.yaml
	os.MkdirAll(filepath.Join(root, "sub", "deep"), 0755)
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

func writeTestYAML(t *testing.T, path string, name string) {
	t.Helper()
	content := []byte("commands:\n  - name: " + name + "\n    command: echo\n    args: [\"test\"]\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
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
		os.WriteFile(path, content, 0644)

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
		os.WriteFile(path, content, 0644)

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
	os.WriteFile(path, content, 0644)

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
	os.WriteFile(path, content, 0644)

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
	os.WriteFile(path, content, 0644)

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
	os.WriteFile(path, content, 0644)

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
		os.WriteFile(path, content, 0644)

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
		os.WriteFile(path, content, 0644)

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
