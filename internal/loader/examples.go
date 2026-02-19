package loader

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed examples/*.yaml
var exampleFiles embed.FS

// CopyExamples copies the embedded example files to commands/examples/
func (l *Loader) CopyExamples() error {
	examplesDir := filepath.Join(l.commandsDir, "examples")

	if err := os.MkdirAll(examplesDir, 0o755); err != nil {
		return fmt.Errorf("create examples dir: %w", err)
	}

	entries, err := exampleFiles.ReadDir("examples")
	if err != nil {
		return fmt.Errorf("read embedded examples: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		srcPath := "examples/" + entry.Name()
		dstPath := filepath.Join(examplesDir, entry.Name())

		// Don't overwrite existing files
		if _, err := os.Stat(dstPath); err == nil {
			continue
		}

		data, err := exampleFiles.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", srcPath, err)
		}

		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dstPath, err)
		}
	}

	return nil
}

// EnsureDefaultDirsWithExamples creates the directories and copies example files
func (l *Loader) EnsureDefaultDirsWithExamples() error {
	if err := l.EnsureDefaultDirs(); err != nil {
		return err
	}

	// Check if directory is empty
	entries, err := os.ReadDir(l.commandsDir)
	if err != nil {
		return err
	}

	// Only copy examples if directory is empty
	hasYAML := false
	for _, e := range entries {
		if isYAMLFile(e.Name()) {
			hasYAML = true
			break
		}
	}

	if !hasYAML {
		if err := l.CopyExamples(); err != nil {
			// Non-fatal - just warn
			fmt.Fprintf(os.Stderr, "warning: couldn't copy examples: %v\n", err)
		}
	}

	return nil
}

// ListExamples returns the names of embedded example files
func ListExamples() ([]string, error) {
	entries, err := fs.ReadDir(exampleFiles, "examples")
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
