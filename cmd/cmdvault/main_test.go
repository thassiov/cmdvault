package main

import (
	"testing"
)

func TestExpandDefaultTemplate(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		resolved map[string]string
		expected string
	}{
		{
			name:     "simple reference",
			tmpl:     "{{input}}",
			resolved: map[string]string{"input": "/path/to/file.mp4"},
			expected: "/path/to/file.mp4",
		},
		{
			name:     "no reference",
			tmpl:     "output.mp4",
			resolved: map[string]string{"input": "/path/to/file.mp4"},
			expected: "output.mp4",
		},
		{
			name:     "unresolved reference kept",
			tmpl:     "{{unknown}}",
			resolved: map[string]string{"input": "/path/to/file.mp4"},
			expected: "{{unknown}}",
		},
		{
			name:     "mixed text and reference",
			tmpl:     "/out/{{input}}.bak",
			resolved: map[string]string{"input": "file.mp4"},
			expected: "/out/file.mp4.bak",
		},
		{
			name:     "multiple references",
			tmpl:     "{{dir}}/{{name}}",
			resolved: map[string]string{"dir": "/tmp", "name": "out.mp4"},
			expected: "/tmp/out.mp4",
		},
		{
			name:     "empty resolved map",
			tmpl:     "{{input}}",
			resolved: map[string]string{},
			expected: "{{input}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandDefaultTemplate(tt.tmpl, tt.resolved)
			if got != tt.expected {
				t.Errorf("expandDefaultTemplate(%q, %v) = %q, want %q", tt.tmpl, tt.resolved, got, tt.expected)
			}
		})
	}
}

func TestExtractPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "single placeholder",
			args:     []string{"-i", "{{input}}"},
			expected: []string{"input"},
		},
		{
			name:     "multiple unique",
			args:     []string{"{{input}}", "{{output}}"},
			expected: []string{"input", "output"},
		},
		{
			name:     "duplicates",
			args:     []string{"{{input}}", "-o", "{{input}}"},
			expected: []string{"input"},
		},
		{
			name:     "no placeholders",
			args:     []string{"-v", "quiet"},
			expected: nil,
		},
		{
			name:     "mixed",
			args:     []string{"-i", "{{input}}", "-ss", "{{start}}", "-to", "{{end}}", "{{output}}"},
			expected: []string{"input", "start", "end", "output"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPlaceholders(tt.args)
			if len(got) != len(tt.expected) {
				t.Fatalf("extractPlaceholders(%v) = %v (len %d), want %v (len %d)",
					tt.args, got, len(got), tt.expected, len(tt.expected))
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("extractPlaceholders(%v)[%d] = %q, want %q", tt.args, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestShelljoin(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "simple args",
			args:     []string{"-v", "quiet"},
			expected: "-v quiet",
		},
		{
			name:     "args with spaces",
			args:     []string{"/path/to/my file.mp4"},
			expected: "'/path/to/my file.mp4'",
		},
		{
			name:     "empty arg",
			args:     []string{"echo", ""},
			expected: "echo ''",
		},
		{
			name:     "single quotes in arg",
			args:     []string{"it's"},
			expected: "'it'\\''s'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shelljoin(tt.args)
			if got != tt.expected {
				t.Errorf("shelljoin(%v) = %q, want %q", tt.args, got, tt.expected)
			}
		})
	}
}

func TestSplitOnDoubleDash(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantBefore []string
		wantAfter  []string
	}{
		{
			name:       "no double dash",
			args:       []string{"foo", "bar"},
			wantBefore: []string{"foo", "bar"},
			wantAfter:  nil,
		},
		{
			name:       "double dash in middle",
			args:       []string{"foo", "--", "bar", "baz"},
			wantBefore: []string{"foo"},
			wantAfter:  []string{"bar", "baz"},
		},
		{
			name:       "double dash at start",
			args:       []string{"--", "bar", "baz"},
			wantBefore: []string{},
			wantAfter:  []string{"bar", "baz"},
		},
		{
			name:       "double dash at end",
			args:       []string{"foo", "--"},
			wantBefore: []string{"foo"},
			wantAfter:  []string{},
		},
		{
			name:       "empty args",
			args:       []string{},
			wantBefore: []string{},
			wantAfter:  nil,
		},
		{
			name:       "nil args",
			args:       nil,
			wantBefore: nil,
			wantAfter:  nil,
		},
		{
			name:       "only double dash",
			args:       []string{"--"},
			wantBefore: []string{},
			wantAfter:  []string{},
		},
		{
			name:       "multiple double dashes splits on first",
			args:       []string{"a", "--", "b", "--", "c"},
			wantBefore: []string{"a"},
			wantAfter:  []string{"b", "--", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, after := splitOnDoubleDash(tt.args)
			if !slicesEqual(before, tt.wantBefore) {
				t.Errorf("splitOnDoubleDash(%v) before = %v, want %v", tt.args, before, tt.wantBefore)
			}
			if !slicesEqual(after, tt.wantAfter) {
				t.Errorf("splitOnDoubleDash(%v) after = %v, want %v", tt.args, after, tt.wantAfter)
			}
		})
	}
}

func TestFillPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		values   map[string]string
		expected []string
	}{
		{
			name:     "single replacement",
			args:     []string{"-i", "{{input}}"},
			values:   map[string]string{"input": "file.mp4"},
			expected: []string{"-i", "file.mp4"},
		},
		{
			name:     "multiple replacements",
			args:     []string{"{{input}}", "-o", "{{output}}"},
			values:   map[string]string{"input": "in.mp4", "output": "out.mp4"},
			expected: []string{"in.mp4", "-o", "out.mp4"},
		},
		{
			name:     "placeholder in middle of string",
			args:     []string{"prefix-{{name}}-suffix"},
			values:   map[string]string{"name": "test"},
			expected: []string{"prefix-test-suffix"},
		},
		{
			name:     "unresolved placeholder kept",
			args:     []string{"{{missing}}"},
			values:   map[string]string{"other": "val"},
			expected: []string{"{{missing}}"},
		},
		{
			name:     "no placeholders",
			args:     []string{"-v", "--quiet"},
			values:   map[string]string{"input": "file"},
			expected: []string{"-v", "--quiet"},
		},
		{
			name:     "empty values map",
			args:     []string{"{{input}}"},
			values:   map[string]string{},
			expected: []string{"{{input}}"},
		},
		{
			name:     "empty args",
			args:     []string{},
			values:   map[string]string{"input": "file"},
			expected: []string{},
		},
		{
			name:     "two placeholders in one arg",
			args:     []string{"{{host}}:{{port}}"},
			values:   map[string]string{"host": "localhost", "port": "5432"},
			expected: []string{"localhost:5432"},
		},
		{
			name:     "same placeholder twice",
			args:     []string{"{{file}}", "--backup", "{{file}}.bak"},
			values:   map[string]string{"file": "data.db"},
			expected: []string{"data.db", "--backup", "data.db.bak"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fillPlaceholders(tt.args, tt.values)
			if len(got) != len(tt.expected) {
				t.Fatalf("fillPlaceholders(%v, %v) = %v (len %d), want %v (len %d)",
					tt.args, tt.values, got, len(got), tt.expected, len(tt.expected))
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("fillPlaceholders(%v, %v)[%d] = %q, want %q", tt.args, tt.values, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestShellescape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: "'hello'",
		},
		{
			name:     "string with spaces",
			input:    "hello world",
			expected: "'hello world'",
		},
		{
			name:     "string with single quote",
			input:    "it's",
			expected: "'it'\\''s'",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "''",
		},
		{
			name:     "string with dollar sign",
			input:    "$HOME",
			expected: "'$HOME'",
		},
		{
			name:     "string with backticks",
			input:    "`whoami`",
			expected: "'`whoami`'",
		},
		{
			name:     "multiple single quotes",
			input:    "it's a 'test'",
			expected: "'it'\\''s a '\\''test'\\'''",
		},
		{
			name:     "path",
			input:    "/home/user/my docs",
			expected: "'/home/user/my docs'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellescape(tt.input)
			if got != tt.expected {
				t.Errorf("shellescape(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNeedsQuoting(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{name: "simple word", input: "hello", expected: false},
		{name: "flag", input: "-v", expected: false},
		{name: "long flag", input: "--verbose", expected: false},
		{name: "path", input: "/usr/bin/ls", expected: false},
		{name: "dotted", input: "file.txt", expected: false},
		{name: "space", input: "hello world", expected: true},
		{name: "tab", input: "hello\tworld", expected: true},
		{name: "newline", input: "hello\nworld", expected: true},
		{name: "double quote", input: `say "hi"`, expected: true},
		{name: "single quote", input: "it's", expected: true},
		{name: "backslash", input: `path\to`, expected: true},
		{name: "dollar", input: "$HOME", expected: true},
		{name: "backtick", input: "`cmd`", expected: true},
		{name: "ampersand", input: "a&b", expected: true},
		{name: "pipe", input: "a|b", expected: true},
		{name: "semicolon", input: "a;b", expected: true},
		{name: "paren", input: "(sub)", expected: true},
		{name: "angle bracket", input: "a>b", expected: true},
		{name: "asterisk", input: "*.txt", expected: true},
		{name: "question mark", input: "file?.txt", expected: true},
		{name: "hash", input: "#comment", expected: true},
		{name: "tilde", input: "~user", expected: true},
		{name: "curly brace", input: "{a,b}", expected: true},
		{name: "exclamation", input: "!important", expected: true},
		{name: "empty", input: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsQuoting(tt.input)
			if got != tt.expected {
				t.Errorf("needsQuoting(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// slicesEqual compares two string slices, treating nil and empty as equal
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
