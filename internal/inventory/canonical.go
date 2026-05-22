package inventory

import (
	"strings"
	"unicode"
)

// CanonicalizeString converts a display string to its lowercase, underscore-separated canonical form.
func CanonicalizeString(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
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
	return strings.TrimRight(result.String(), "_")
}
