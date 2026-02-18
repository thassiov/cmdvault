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
