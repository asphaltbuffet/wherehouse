package database

import (
	"strings"
	"unicode"
)

// CanonicalizeString converts a display name to canonical form.
// Rules: lowercase, trim whitespace, collapse runs to '_', normalize separators to '_'.
func CanonicalizeString(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Trim leading and trailing whitespace
	s = strings.TrimSpace(s)

	// Replace runs of whitespace and separators with single underscore
	var result strings.Builder
	var prevWasUnderscore bool

	for _, r := range s {
		if unicode.IsSpace(r) || r == '-' || r == '_' {
			if !prevWasUnderscore {
				result.WriteRune('_')
				prevWasUnderscore = true
			}
		} else {
			result.WriteRune(r)
			prevWasUnderscore = false
		}
	}

	// Trim trailing underscore
	canonical := result.String()
	canonical = strings.TrimRight(canonical, "_")

	return canonical
}
