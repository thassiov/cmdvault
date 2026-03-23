// Package resolve handles placeholder extraction, filling, and template expansion
// for command argument strings using the {{name}} syntax.
package resolve

import "regexp"

// PlaceholderRegex matches {{name}} placeholders in argument strings.
var PlaceholderRegex = regexp.MustCompile(`\{\{(\w+)\}\}`)

// ExtractPlaceholders finds all {{name}} placeholders in args, returns unique names in order.
func ExtractPlaceholders(args []string) []string {
	seen := make(map[string]bool)
	var placeholders []string

	for _, arg := range args {
		matches := PlaceholderRegex.FindAllStringSubmatch(arg, -1)
		for _, match := range matches {
			name := match[1]
			if !seen[name] {
				seen[name] = true
				placeholders = append(placeholders, name)
			}
		}
	}

	return placeholders
}

// SplitOnDoubleDash splits args into before and after "--".
func SplitOnDoubleDash(args []string) (before, after []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

// FillPlaceholders replaces {{name}} with values from the map.
func FillPlaceholders(args []string, values map[string]string) []string {
	result := make([]string, len(args))
	for i, arg := range args {
		result[i] = PlaceholderRegex.ReplaceAllStringFunc(arg, func(match string) string {
			name := PlaceholderRegex.FindStringSubmatch(match)[1]
			if val, ok := values[name]; ok {
				return val
			}
			return match
		})
	}
	return result
}

// ExpandDefaultTemplate replaces {{name}} references in a default value
// with already-resolved placeholder values.
func ExpandDefaultTemplate(tmpl string, resolved map[string]string) string {
	return PlaceholderRegex.ReplaceAllStringFunc(tmpl, func(match string) string {
		ref := PlaceholderRegex.FindStringSubmatch(match)[1]
		if val, ok := resolved[ref]; ok {
			return val
		}
		return match
	})
}
