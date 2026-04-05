package cli

import (
	"time"

	"github.com/dustin/go-humanize"
)

// FormatRelativeTime formats a timestamp as a human-readable relative time string.
func FormatRelativeTime(t time.Time) string {
	return humanize.Time(t)
}
