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
