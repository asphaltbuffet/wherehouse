package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

func TestFilterEvents_NewestFirst_ReversesOrder(t *testing.T) {
	events := makeTestEvents(3)
	filtered, err := filterEvents(events, 0, "", true)

	require.NoError(t, err)
	require.Len(t, filtered, 3)

	// Verify reverse order (newest first)
	assert.Equal(t, int64(3), filtered[0].EventID)
	assert.Equal(t, int64(2), filtered[1].EventID)
	assert.Equal(t, int64(1), filtered[2].EventID)
}

func TestFilterEvents_OldestFirst_KeepsOrder(t *testing.T) {
	events := makeTestEvents(3)
	filtered, err := filterEvents(events, 0, "", false)

	require.NoError(t, err)
	require.Len(t, filtered, 3)

	// Verify original order (oldest first)
	assert.Equal(t, int64(1), filtered[0].EventID)
	assert.Equal(t, int64(2), filtered[1].EventID)
	assert.Equal(t, int64(3), filtered[2].EventID)
}

func TestFilterEvents_LimitTakesTopN(t *testing.T) {
	events := makeTestEvents(5)
	filtered, err := filterEvents(events, 3, "", true)

	require.NoError(t, err)
	assert.Len(t, filtered, 3)

	// With newest first, should take events 5, 4, 3
	assert.Equal(t, int64(5), filtered[0].EventID)
	assert.Equal(t, int64(4), filtered[1].EventID)
	assert.Equal(t, int64(3), filtered[2].EventID)
}

func TestFilterEvents_LimitExceedsCount_ReturnsAll(t *testing.T) {
	events := makeTestEvents(3)
	filtered, err := filterEvents(events, 10, "", true)

	require.NoError(t, err)
	assert.Len(t, filtered, 3)
}

func TestFilterEvents_LimitZero_NoLimit(t *testing.T) {
	events := makeTestEvents(5)
	filtered, err := filterEvents(events, 0, "", true)

	require.NoError(t, err)
	assert.Len(t, filtered, 5)
}

func TestFilterEvents_SinceAbsoluteDate_Filters(t *testing.T) {
	events := []*database.Event{
		makeEventWithTime(1, "2026-01-01T12:00:00Z"),
		makeEventWithTime(2, "2026-01-15T12:00:00Z"),
		makeEventWithTime(3, "2026-02-01T12:00:00Z"),
	}

	filtered, err := filterEvents(events, 0, "2026-01-10", true)

	require.NoError(t, err)
	assert.Len(t, filtered, 2)
	// Should contain events 2 and 3 (after Jan 10)
	assert.Equal(t, int64(3), filtered[0].EventID)
	assert.Equal(t, int64(2), filtered[1].EventID)
}

func TestFilterEvents_SinceRelativeYesterday_Filters(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	twoDaysAgo := now.AddDate(0, 0, -2)
	afterYesterday := yesterday.Add(1 * time.Hour) // Event after yesterday timestamp

	events := []*database.Event{
		makeEventWithTime(1, twoDaysAgo.Format(time.RFC3339)),
		makeEventWithTime(2, afterYesterday.Format(time.RFC3339)),
		makeEventWithTime(3, now.Format(time.RFC3339)),
	}

	filtered, err := filterEvents(events, 0, "yesterday", true)

	require.NoError(t, err)
	require.Len(t, filtered, 2)
	// Events 2 and 3 (after yesterday)
	assert.Equal(t, int64(3), filtered[0].EventID)
	assert.Equal(t, int64(2), filtered[1].EventID)
}

func TestFilterEvents_SinceRelativeNDaysAgo_Filters(t *testing.T) {
	now := time.Now()
	tenDaysAgo := now.AddDate(0, 0, -10)
	fiveDaysAgo := now.AddDate(0, 0, -5)
	oneDayAgo := now.AddDate(0, 0, -1)

	events := []*database.Event{
		makeEventWithTime(1, tenDaysAgo.Format(time.RFC3339)),
		makeEventWithTime(2, fiveDaysAgo.Format(time.RFC3339)),
		makeEventWithTime(3, oneDayAgo.Format(time.RFC3339)),
	}

	filtered, err := filterEvents(events, 0, "8 days ago", true)

	require.NoError(t, err)
	assert.Len(t, filtered, 2)
	// Events after 8 days ago: events 2 and 3 (5 and 1 days ago are after 8 days ago)
	assert.Equal(t, int64(3), filtered[0].EventID)
	assert.Equal(t, int64(2), filtered[1].EventID)
}

func TestFilterEvents_SinceRelativeWeeksAgo_Filters(t *testing.T) {
	now := time.Now()
	fourWeeksAgo := now.AddDate(0, 0, -28)
	twoWeeksAgo := now.AddDate(0, 0, -14)
	oneWeekAgo := now.AddDate(0, 0, -7)

	events := []*database.Event{
		makeEventWithTime(1, fourWeeksAgo.Format(time.RFC3339)),
		makeEventWithTime(2, twoWeeksAgo.Format(time.RFC3339)),
		makeEventWithTime(3, oneWeekAgo.Format(time.RFC3339)),
	}

	filtered, err := filterEvents(events, 0, "3 weeks ago", true)

	require.NoError(t, err)
	assert.Len(t, filtered, 2)
	// Events after 3 weeks ago: events 2 and 3 (2 and 1 weeks ago are after 3 weeks ago)
	assert.Equal(t, int64(3), filtered[0].EventID)
	assert.Equal(t, int64(2), filtered[1].EventID)
}

func TestFilterEvents_SinceRelativeMonthsAgo_Filters(t *testing.T) {
	now := time.Now()
	threeMonthsAgo := now.AddDate(0, -3, 0)
	oneMonthAgo := now.AddDate(0, -1, 0)

	events := []*database.Event{
		makeEventWithTime(1, threeMonthsAgo.Format(time.RFC3339)),
		makeEventWithTime(2, oneMonthAgo.Format(time.RFC3339)),
		makeEventWithTime(3, now.Format(time.RFC3339)),
	}

	filtered, err := filterEvents(events, 0, "2 months ago", true)

	require.NoError(t, err)
	assert.Len(t, filtered, 2)
	// Events after 2 months ago: events 2 and 3 (1 month and now are after 2 months ago)
	assert.Equal(t, int64(3), filtered[0].EventID)
	assert.Equal(t, int64(2), filtered[1].EventID)
}

func TestFilterEvents_SinceRelativeYearsAgo_Filters(t *testing.T) {
	now := time.Now()
	twoYearsAgo := now.AddDate(-2, 0, 0)
	oneYearAgo := now.AddDate(-1, 0, 0)

	events := []*database.Event{
		makeEventWithTime(1, twoYearsAgo.Format(time.RFC3339)),
		makeEventWithTime(2, oneYearAgo.Format(time.RFC3339)),
		makeEventWithTime(3, now.Format(time.RFC3339)),
	}

	filtered, err := filterEvents(events, 0, "18 months ago", true)

	require.NoError(t, err)
	assert.Len(t, filtered, 2)
	// Events after 18 months ago: events 2 and 3 (1 year and now are after 18 months ago)
	assert.Equal(t, int64(3), filtered[0].EventID)
	assert.Equal(t, int64(2), filtered[1].EventID)
}

func TestFilterEvents_InvalidDate_ReturnsError(t *testing.T) {
	events := makeTestEvents(1)
	_, err := filterEvents(events, 0, "not a valid date!!!", true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to parse date")
}

func TestFilterEvents_EmptyEventList_ReturnsEmpty(t *testing.T) {
	var events []*database.Event
	filtered, err := filterEvents(events, 0, "", true)

	require.NoError(t, err)
	assert.Empty(t, filtered)
}

func TestFilterEvents_DefensiveCopy_DoesNotMutateCaller(t *testing.T) {
	events := makeTestEvents(3)
	original := make([]*database.Event, len(events))
	copy(original, events)

	_, err := filterEvents(events, 0, "", true)

	require.NoError(t, err)
	// Verify original slice not mutated
	assert.Equal(t, original[0].EventID, events[0].EventID)
	assert.Equal(t, original[1].EventID, events[1].EventID)
	assert.Equal(t, original[2].EventID, events[2].EventID)
}

func TestFilterEvents_SinceAndLimit_CombineCorrectly(t *testing.T) {
	now := time.Now()
	fiveDaysAgo := now.AddDate(0, 0, -5)
	twoDaysAgo := now.AddDate(0, 0, -2)

	events := []*database.Event{
		makeEventWithTime(1, fiveDaysAgo.Format(time.RFC3339)),
		makeEventWithTime(2, twoDaysAgo.Format(time.RFC3339)),
		makeEventWithTime(3, twoDaysAgo.Format(time.RFC3339)),
		makeEventWithTime(4, now.Format(time.RFC3339)),
	}

	filtered, err := filterEvents(events, 2, "3 days ago", true)

	require.NoError(t, err)
	assert.Len(t, filtered, 2)
	// After "3 days ago" filter: events 2, 3, 4 (3 events)
	// With limit 2 and newest first: events 4, 3
	assert.Equal(t, int64(4), filtered[0].EventID)
	assert.Equal(t, int64(3), filtered[1].EventID)
}

func TestFilterEvents_SinceExclusiveFilter_DoesNotIncludeSinceTime(t *testing.T) {
	baseTime := time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC)
	afterTime := baseTime.Add(1 * time.Second)
	beforeTime := baseTime.Add(-1 * time.Second)

	events := []*database.Event{
		makeEventWithTime(1, beforeTime.Format(time.RFC3339)),
		makeEventWithTime(2, afterTime.Format(time.RFC3339)),
	}

	filtered, err := filterEvents(events, 0, "2026-02-24T12:00:00Z", true)

	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	// Only event 2 (after the exact time)
	assert.Equal(t, int64(2), filtered[0].EventID)
}

func TestFilterEvents_AllEventsSince_ReturnsAll(t *testing.T) {
	now := time.Now()
	oneDayAgo := now.AddDate(0, 0, -1)
	fiveDaysAgo := now.AddDate(0, 0, -5)

	events := []*database.Event{
		makeEventWithTime(1, fiveDaysAgo.Format(time.RFC3339)),
		makeEventWithTime(2, oneDayAgo.Format(time.RFC3339)),
		makeEventWithTime(3, now.Format(time.RFC3339)),
	}

	// Filter since before all events
	filtered, err := filterEvents(events, 0, "10 days ago", true)

	require.NoError(t, err)
	assert.Len(t, filtered, 3)
}

func TestFilterEvents_AllEventsSinceTwoDaysAgo_ReturnsAll(t *testing.T) {
	now := time.Now()
	threeDaysAgo := now.AddDate(0, 0, -3)
	twoDaysAgo := now.AddDate(0, 0, -2)

	events := []*database.Event{
		makeEventWithTime(1, threeDaysAgo.Format(time.RFC3339)),
		makeEventWithTime(2, twoDaysAgo.Format(time.RFC3339)),
	}

	// Filter since 5 days ago - should return all events
	filtered, err := filterEvents(events, 0, "5 days ago", true)

	require.NoError(t, err)
	assert.Len(t, filtered, 2)
}

func TestFilterEvents_FutureDate_ReturnsEmpty(t *testing.T) {
	now := time.Now()
	oneDayAgo := now.AddDate(0, 0, -1)

	events := []*database.Event{
		makeEventWithTime(1, oneDayAgo.Format(time.RFC3339)),
		makeEventWithTime(2, now.Format(time.RFC3339)),
	}

	tomorrow := now.AddDate(0, 0, 1)
	filtered, err := filterEvents(events, 0, tomorrow.Format(time.RFC3339), true)

	require.NoError(t, err)
	assert.Empty(t, filtered)
}

func TestParseRelativeTime_Yesterday_ParsesCorrectly(t *testing.T) {
	result, ok := parseRelativeTime("yesterday")

	assert.True(t, ok)
	now := time.Now()
	expected := now.AddDate(0, 0, -1)

	// Allow 1-second tolerance for test execution time
	assert.WithinDuration(t, expected, result, 2*time.Second)
}

func TestParseRelativeTime_NDaysAgo_ParsesCorrectly(t *testing.T) {
	result, ok := parseRelativeTime("5 days ago")

	assert.True(t, ok)
	now := time.Now()
	expected := now.AddDate(0, 0, -5)

	// Allow 1-second tolerance
	assert.WithinDuration(t, expected, result, 2*time.Second)
}

func TestParseRelativeTime_WeeksAgo_ParsesCorrectly(t *testing.T) {
	result, ok := parseRelativeTime("2 weeks ago")

	assert.True(t, ok)
	now := time.Now()
	expected := now.AddDate(0, 0, -14)

	// Allow 1-second tolerance
	assert.WithinDuration(t, expected, result, 2*time.Second)
}

func TestParseRelativeTime_MonthsAgo_ParsesCorrectly(t *testing.T) {
	result, ok := parseRelativeTime("3 months ago")

	assert.True(t, ok)
	now := time.Now()
	expected := now.AddDate(0, -3, 0)

	// Allow 1-second tolerance
	assert.WithinDuration(t, expected, result, 2*time.Second)
}

func TestParseRelativeTime_YearsAgo_ParsesCorrectly(t *testing.T) {
	result, ok := parseRelativeTime("1 year ago")

	assert.True(t, ok)
	now := time.Now()
	expected := now.AddDate(-1, 0, 0)

	// Allow 1-second tolerance
	assert.WithinDuration(t, expected, result, 2*time.Second)
}

func TestParseRelativeTime_InvalidFormat_ReturnsFalse(t *testing.T) {
	_, ok := parseRelativeTime("not a relative time")
	assert.False(t, ok)
}

func TestParseRelativeTime_CaseInsensitive_Works(t *testing.T) {
	result, ok := parseRelativeTime("YESTERDAY")

	assert.True(t, ok)
	now := time.Now()
	expected := now.AddDate(0, 0, -1)

	assert.WithinDuration(t, expected, result, 2*time.Second)
}

func TestReverseSlice_EmptySlice_NoOp(t *testing.T) {
	events := []*database.Event{}
	reverseSlice(events)
	assert.Empty(t, events)
}

func TestReverseSlice_SingleElement_NoChange(t *testing.T) {
	events := makeTestEvents(1)
	reverseSlice(events)

	assert.Len(t, events, 1)
	assert.Equal(t, int64(1), events[0].EventID)
}

// Helper functions

func makeTestEvents(count int) []*database.Event {
	events := make([]*database.Event, count)
	for i := range count {
		events[i] = &database.Event{
			EventID:      int64(i + 1),
			EventType:    "item.moved",
			TimestampUTC: time.Now().Add(time.Duration(i) * time.Hour).Format(time.RFC3339),
			ActorUserID:  "user1",
			Payload:      []byte("{}"),
		}
	}
	return events
}

func makeEventWithTime(eventID int64, timestamp string) *database.Event {
	return &database.Event{
		EventID:      eventID,
		EventType:    "item.moved",
		TimestampUTC: timestamp,
		ActorUserID:  "user1",
		Payload:      []byte("{}"),
	}
}
