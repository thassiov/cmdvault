package shell

import "strings"

// Escape wraps a string in single quotes for safe shell embedding.
func Escape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// Join combines args into a shell-safe string, quoting args that need it.
func Join(args []string) string {
	var parts []string
	for _, arg := range args {
		if arg == "" {
			parts = append(parts, "''")
		} else if NeedsQuoting(arg) {
			escaped := strings.ReplaceAll(arg, "'", "'\\''")
			parts = append(parts, "'"+escaped+"'")
		} else {
			parts = append(parts, arg)
		}
	}
	return strings.Join(parts, " ")
}

// NeedsQuoting returns true if a string contains characters that need shell quoting.
func NeedsQuoting(s string) bool {
	for _, c := range s {
		switch c {
		case ' ', '\t', '\n', '"', '\'', '\\', '`', '$', '!', '&', '|', ';', '(', ')', '<', '>', '*', '?', '[', ']', '#', '~', '{', '}', '^':
			return true
		}
	}
	return false
}
