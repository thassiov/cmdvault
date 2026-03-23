package shell

import "testing"

func TestEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "simple string", input: "hello", expected: "'hello'"},
		{name: "string with spaces", input: "hello world", expected: "'hello world'"},
		{name: "string with single quote", input: "it's", expected: "'it'\\''s'"},
		{name: "empty string", input: "", expected: "''"},
		{name: "string with dollar sign", input: "$HOME", expected: "'$HOME'"},
		{name: "string with backticks", input: "`whoami`", expected: "'`whoami`'"},
		{name: "multiple single quotes", input: "it's a 'test'", expected: "'it'\\''s a '\\''test'\\'''"},
		{name: "path", input: "/home/user/my docs", expected: "'/home/user/my docs'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Escape(tt.input)
			if got != tt.expected {
				t.Errorf("Escape(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{name: "simple args", args: []string{"-v", "quiet"}, expected: "-v quiet"},
		{name: "args with spaces", args: []string{"/path/to/my file.mp4"}, expected: "'/path/to/my file.mp4'"},
		{name: "empty arg", args: []string{"echo", ""}, expected: "echo ''"},
		{name: "single quotes in arg", args: []string{"it's"}, expected: "'it'\\''s'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Join(tt.args)
			if got != tt.expected {
				t.Errorf("Join(%v) = %q, want %q", tt.args, got, tt.expected)
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
			got := NeedsQuoting(tt.input)
			if got != tt.expected {
				t.Errorf("NeedsQuoting(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
