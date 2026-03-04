# Task 4: Replace Inline Timestamp Logic with cli.FormatRelativeTime

## Files Modified

### `cmd/history/output.go`

**Removed:**
- `const` block containing `hoursPerDay = 24` and `recentDaysThreshold = 7`
- `formatTimestamp(timestampUTC string) string` function (lines 174–191 original)
- `formatRelativeTime(d time.Duration) string` function (lines 193–217 original)

**Added:**
- Import: `github.com/asphaltbuffet/wherehouse/internal/cli`

**Changed:**
- In `formatEvent()`: replaced `timestamp := formatTimestamp(event.TimestampUTC)` with inline
  `time.Parse` + `cli.FormatRelativeTime(t)` call. On parse error, falls back to zero time
  (go-humanize renders that as a distant past string rather than crashing).

**Retained:**
- `"time"` import — still required for `time.Parse` and `time.RFC3339` and `time.Time{}`
- All other imports unchanged

## Build

`go build ./cmd/history/...` passes with no errors or warnings.
