package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thassiov/cmdvault/internal/command"
	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigDir  = ".config/cmdvault"
	DefaultCommandDir = "commands"
)

// CommandFile represents the structure of a YAML command file.
type CommandFile struct {
	// Optional metadata for the file
	Name        string               `yaml:"name,omitempty"`
	Description string               `yaml:"description,omitempty"`
	Commands    []command.Descriptor `yaml:"commands"`
}

// Loader handles loading command files.
type Loader struct {
	commandsDir string
}

// New creates a loader with default paths (~/.config/cmdvault/commands).
func New() (*Loader, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	return &Loader{
		commandsDir: filepath.Join(home, DefaultConfigDir, DefaultCommandDir),
	}, nil
}

// NewWithPath creates a loader with a custom commands directory.
func NewWithPath(commandsDir string) *Loader {
	return &Loader{
		commandsDir: commandsDir,
	}
}

// LoadFile loads commands from a specific YAML file.
func (l *Loader) LoadFile(path string) ([]command.Descriptor, error) {
	return l.loadFileWithBase(path, "")
}

// loadFileWithBase loads commands from a YAML file, deriving the category from
// the file's path relative to baseDir. When baseDir is empty, the category is
// derived from the filename only (backward-compatible behavior).
//
// Examples (baseDir = "/home/user/.config/cmdvault/commands"):
//
//	commands/docker.yaml              → category "docker"
//	commands/grid/health.yaml         → category "grid/health"
//	commands/grid/opnsense/dnsbl.yaml → category "grid/opnsense/dnsbl"
func (l *Loader) loadFileWithBase(path, baseDir string) ([]command.Descriptor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	var cf CommandFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse yaml %s: %w", path, err)
	}

	category := deriveCategory(path, baseDir)

	// Validate and tag each command
	valid := make([]command.Descriptor, 0, len(cf.Commands))
	for i := range cf.Commands {
		cmd := &cf.Commands[i]

		// Skip entries without a command binary
		if cmd.Command == "" {
			fmt.Fprintf(os.Stderr, "warning: %s entry %d has no command, skipping\n", path, i)
			continue
		}

		// Generate name from filename if missing
		if cmd.Name == "" {
			if cmd.Description != "" {
				// Promote user-written description to name;
				// clear description so it gets auto-generated from command+args
				cmd.Name = cmd.Description
				cmd.Description = ""
			} else {
				cmd.Name = fmt.Sprintf("%s#%d", filepath.Base(path), i)
			}
		}

		// Sanitize name and description: collapse newlines/tabs to spaces
		cmd.Name = sanitize(cmd.Name)
		cmd.Description = sanitize(cmd.Description)

		// Generate description from the command itself if missing
		if cmd.Description == "" {
			if len(cmd.Args) > 0 {
				cmd.Description = cmd.Command + " " + strings.Join(cmd.Args, " ")
			} else {
				cmd.Description = cmd.Command
			}
		}

		cmd.Category = category
		if cmd.Alias == "" {
			cmd.Alias = generateAlias(cmd.Name)
		}

		valid = append(valid, *cmd)
	}

	return valid, nil
}

// deriveCategory computes the category from a file path relative to a base directory.
// If baseDir is empty or the relative path cannot be computed, it falls back to the
// filename without extension. Directory separators are normalized to forward slashes.
//
// Examples:
//
//	deriveCategory("/commands/docker.yaml", "/commands")             → "docker"
//	deriveCategory("/commands/grid/health.yaml", "/commands")        → "grid/health"
//	deriveCategory("/commands/grid/opnsense/dnsbl.yaml", "/commands") → "grid/opnsense/dnsbl"
//	deriveCategory("/anywhere/docker.yaml", "")                       → "docker"
func deriveCategory(path, baseDir string) string {
	if baseDir != "" {
		rel, err := filepath.Rel(baseDir, path)
		if err == nil {
			// Strip extension and normalize separators to forward slash
			rel = strings.TrimSuffix(rel, filepath.Ext(rel))
			return filepath.ToSlash(rel)
		}
	}
	// Fallback: filename only
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// LoadDir loads all YAML files from the commands directory recursively.
func (l *Loader) LoadDir() ([]command.Descriptor, error) {
	return l.LoadDirRecursive(l.commandsDir)
}

// LoadDirFrom loads YAML files from first level of a directory (for custom paths).
func (l *Loader) LoadDirFrom(dir string) ([]command.Descriptor, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("commands directory does not exist: %s", dir)
		}
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	var allCommands []command.Descriptor

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !isYAMLFile(entry.Name()) {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		commands, err := l.loadFileWithBase(path, dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load %s: %v\n", path, err)
			continue
		}

		allCommands = append(allCommands, commands...)
	}

	return allCommands, nil
}

// LoadDirRecursive loads all YAML files from a directory recursively (for default dir).
func (l *Loader) LoadDirRecursive(dir string) ([]command.Descriptor, error) {
	// Resolve symlinks so filepath.WalkDir works with symlinked directories
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("commands directory does not exist: %s", dir)
		}
		return nil, fmt.Errorf("resolve path %s: %w", dir, err)
	}
	dir = resolved

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("commands directory does not exist: %s", dir)
	}

	var allCommands []command.Descriptor

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if !isYAMLFile(d.Name()) {
			return nil
		}

		commands, err := l.loadFileWithBase(path, dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load %s: %v\n", path, err)
			return nil
		}

		allCommands = append(allCommands, commands...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk dir %s: %w", dir, err)
	}

	return allCommands, nil
}

// Load is the main entry point - loads from file if provided, otherwise from default dir.
func (l *Loader) Load(filePath string) ([]command.Descriptor, error) {
	if filePath != "" {
		// Check if it's a file or directory
		info, err := os.Stat(filePath)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", filePath, err)
		}

		if info.IsDir() {
			return l.LoadDirRecursive(filePath)
		}
		return l.LoadFile(filePath)
	}

	return l.LoadDir()
}

// EnsureDefaultDirs creates the default config directories if they don't exist.
func (l *Loader) EnsureDefaultDirs() error {
	if err := os.MkdirAll(l.commandsDir, 0o755); err != nil {
		return fmt.Errorf("create commands dir: %w", err)
	}
	return nil
}

// DefaultDirExists checks if the default commands directory exists.
func (l *Loader) DefaultDirExists() bool {
	_, err := os.Stat(l.commandsDir)
	return err == nil
}

// GetCommandsDir returns the commands directory path.
func (l *Loader) GetCommandsDir() string {
	return l.commandsDir
}

// sanitize collapses newlines, tabs, and multiple spaces into single spaces.
func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	// Collapse multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

func isYAMLFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

// generateAlias creates an alias from a name by lowercasing and joining with dashes.
// "List Containers" → "list-containers".
func generateAlias(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-"))
}
