// Package history implements the wherehouse history command for displaying item event timelines.
package history

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// filterEvents applies limit and since filters to event list.
// Returns events in reverse chronological order (newest first) by default.
// Set newestFirst=false for chronological (oldest first).
func filterEvents(
	events []*database.Event,
	limit int,
	sinceStr string,
	newestFirst bool,
) ([]*database.Event, error) {
	// Make defensive copy to avoid mutating caller's slice
	filtered := make([]*database.Event, len(events))
	copy(filtered, events)

	// Apply since filter (time-based)
	if sinceStr != "" {
		sinceTime, err := parseSinceTime(sinceStr)
		if err != nil {
			return nil, fmt.Errorf("invalid --since value: %w", err)
		}
		filtered = filterSince(filtered, sinceTime)
	}

	// Apply ordering
	if newestFirst {
		reverseSlice(filtered)
	}

	// Apply limit filter (after ordering)
	if limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

// parseSinceTime parses a date string or relative time expression.
// Supports:
//   - Absolute dates: "2026-01-15", "Jan 15 2026"
//   - Relative: "2 weeks ago", "3 days ago", "yesterday"
//
// Tries manual relative time parsing first, then falls back to araddon/dateparse.
func parseSinceTime(sinceStr string) (time.Time, error) {
	// Try manual relative time parsing first
	if t, ok := parseRelativeTime(sinceStr); ok {
		return t, nil
	}

	// Fallback to dateparse for absolute dates
	t, err := dateparse.ParseAny(sinceStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("unable to parse date %q: %w", sinceStr, err)
	}
	return t, nil
}

// parseRelativeTime handles relative time expressions manually.
// Returns (time, true) if parsed successfully, (zero, false) otherwise.
func parseRelativeTime(s string) (time.Time, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	now := time.Now()

	// Handle "yesterday"
	if s == "yesterday" {
		return now.AddDate(0, 0, -1), true
	}

	// Handle "N days/weeks/months/years ago"
	re := regexp.MustCompile(`^(\d+)\s+(day|week|month|year)s?\s+ago$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return time.Time{}, false
	}

	amount, err := strconv.Atoi(matches[1])
	if err != nil {
		return time.Time{}, false
	}

	const daysPerWeek = 7

	unit := matches[2]
	switch unit {
	case "day":
		return now.AddDate(0, 0, -amount), true
	case "week":
		return now.AddDate(0, 0, -amount*daysPerWeek), true
	case "month":
		return now.AddDate(0, -amount, 0), true
	case "year":
		return now.AddDate(-amount, 0, 0), true
	default:
		return time.Time{}, false
	}
}

// filterSince returns events after the specified time (exclusive).
// Events with unparseable timestamps are included with a warning to stderr.
func filterSince(events []*database.Event, since time.Time) []*database.Event {
	var filtered []*database.Event
	for _, event := range events {
		eventTime, err := time.Parse(time.RFC3339, event.TimestampUTC)
		if err != nil {
			// Include events with invalid timestamps (cannot filter by time)
			// Emit warning to stderr
			fmt.Fprintf(os.Stderr, "warning: event %d has unparseable timestamp %q, including in output\n",
				event.EventID, event.TimestampUTC)
			filtered = append(filtered, event)
			continue
		}
		if eventTime.After(since) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// reverseSlice reverses a slice of events in-place.
func reverseSlice(events []*database.Event) {
	for i := range len(events) / 2 {
		j := len(events) - 1 - i

		events[i], events[j] = events[j], events[i]
	}
}
