// Package prompt handles interactive user input for placeholder values,
// including plain text prompts, fzf source selection, and file pickers.
package prompt

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/thassiov/cmdvault/internal/command"
	"github.com/thassiov/cmdvault/internal/resolve"
	"github.com/thassiov/cmdvault/internal/shell"
)

// ForValue prompts the user to enter a value for the placeholder.
func ForValue(name string, config *command.PlaceholderConfig) string {
	reader := bufio.NewReader(os.Stdin)
	prompt := name
	if config != nil && config.Description != "" {
		prompt = fmt.Sprintf("%s (%s)", name, config.Description)
	}
	if config != nil && config.Default != "" {
		fmt.Fprintf(os.Stderr, "%s [%s]: ", prompt, config.Default)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
	}
	value, _ := reader.ReadString('\n')
	value = strings.TrimSpace(value)
	if value == "" && config != nil && config.Default != "" {
		return config.Default
	}
	return value
}

// SelectFromSource runs a source command and pipes output to fzf for selection.
func SelectFromSource(name, source string) (string, error) {
	cmd := exec.Command("sh", "-c", source+" | fzf --height=~100% --layout=reverse --border --cycle --prompt="+shell.Escape(name+"> "))
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// SelectFile launches an fzf file picker with path-aware root switching.
// When the query starts with "/" it searches from root; with "~" from $HOME;
// otherwise from the current directory.
func SelectFile(name string, config *command.PlaceholderConfig) (string, error) {
	prompt := name
	if config != nil && config.Description != "" {
		prompt = fmt.Sprintf("%s (%s)", name, config.Description)
	}

	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}

	finderScript := fmt.Sprintf(`
q={q}
if [ "${q#/}" != "$q" ]; then
  find / -maxdepth 6 -type f -not -path '*/.git/*' 2>/dev/null | head -50000
elif [ "${q#\~}" != "$q" ]; then
  find %s -maxdepth 6 -type f -not -path '*/.git/*' 2>/dev/null | head -50000
else
  find . -type f -not -path '*/.git/*' 2>/dev/null | head -50000
fi
`, shell.Escape(home))

	fzfArgs := []string{
		"--height=~100%",
		"--layout=reverse",
		"--border",
		"--cycle",
		"--scheme=path",
		"--prompt=" + prompt + "> ",
		"--bind", "start:reload:" + strings.TrimSpace(finderScript),
		"--bind", "change:reload:" + strings.TrimSpace(finderScript),
	}

	if config != nil && config.Default != "" {
		fzfArgs = append(fzfArgs, "--query="+config.Default)
	}

	cmd := exec.Command("fzf", fzfArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	selected := strings.TrimSpace(string(output))

	if strings.HasPrefix(selected, "~/") {
		selected = filepath.Join(home, selected[2:])
	}

	return selected, nil
}

// GetPlaceholderValue gets a value for a placeholder, using source or file picker if configured.
func GetPlaceholderValue(name string, config *command.PlaceholderConfig, resolved map[string]string) string {
	if config != nil && config.Default != "" {
		config.Default = resolve.ExpandDefaultTemplate(config.Default, resolved)
	}

	if config != nil && config.Type == "file" {
		value, err := SelectFile(name, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Selection canceled, enter manually.\n")
			return ForValue(name, config)
		}
		return value
	}

	if config != nil && config.Source != "" {
		value, err := SelectFromSource(name, config.Source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Selection canceled, enter manually.\n")
			return ForValue(name, config)
		}
		return value
	}

	return ForValue(name, config)
}
