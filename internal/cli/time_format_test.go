package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatRelativeTime(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{"minutes", now.Add(-5 * time.Minute), "minutes ago"},
		{"hours", now.Add(-3 * time.Hour), "hours ago"},
		{"days", now.Add(-48 * time.Hour), "days ago"},
		{"weeks", now.Add(-14 * 24 * time.Hour), "weeks ago"},
		{"months", now.Add(-180 * 24 * time.Hour), "months ago"},
		{"years", now.Add(-5 * 365 * 24 * time.Hour), "years ago"},
		{"future", now.Add(5 * time.Minute), "from now"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := FormatRelativeTime(tt.time)
			assert.Contains(t, result, tt.expected)
		})
	}
}
