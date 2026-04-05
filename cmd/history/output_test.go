package history

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

func TestFormatJSON_SingleEvent_ProducesValidJSON(t *testing.T) {
	event := &database.Event{
		EventID:      1,
		EventType:    "item.created",
		TimestampUTC: "2026-02-24T12:00:00Z",
		ActorUserID:  "alice",
		Payload:      []byte(`{"location_id":"loc1"}`),
	}

	var buf bytes.Buffer
	err := formatJSON(&buf, []*database.Event{event})

	require.NoError(t, err)

	var output JSONHistoryOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Equal(t, 1, output.Count)
	assert.Len(t, output.Events, 1)
	assert.Equal(t, int64(1), output.Events[0].EventID)
}

func TestFormatJSON_MultipleEvents_CreatesArray(t *testing.T) {
	events := []*database.Event{
		{
			EventID:      1,
			EventType:    "item.created",
			TimestampUTC: "2026-02-24T12:00:00Z",
			ActorUserID:  "alice",
			Payload:      []byte(`{}`),
		},
		{
			EventID:      2,
			EventType:    "item.moved",
			TimestampUTC: "2026-02-24T13:00:00Z",
			ActorUserID:  "bob",
			Payload:      []byte(`{}`),
		},
		{
			EventID:      3,
			EventType:    "item.deleted",
			TimestampUTC: "2026-02-24T14:00:00Z",
			ActorUserID:  "alice",
			Payload:      []byte(`{}`),
		},
	}

	var buf bytes.Buffer
	err := formatJSON(&buf, events)

	require.NoError(t, err)

	var output JSONHistoryOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Equal(t, 3, output.Count)
	assert.Len(t, output.Events, 3)
	assert.Equal(t, int64(1), output.Events[0].EventID)
	assert.Equal(t, int64(2), output.Events[1].EventID)
	assert.Equal(t, int64(3), output.Events[2].EventID)
}

func TestFormatJSON_CountField_MatchesEventCount(t *testing.T) {
	events := []*database.Event{
		{
			EventID:      1,
			EventType:    "item.created",
			TimestampUTC: "2026-02-24T12:00:00Z",
			ActorUserID:  "alice",
			Payload:      []byte(`{}`),
		},
		{
			EventID:      2,
			EventType:    "item.moved",
			TimestampUTC: "2026-02-24T13:00:00Z",
			ActorUserID:  "bob",
			Payload:      []byte(`{}`),
		},
	}

	var buf bytes.Buffer
	err := formatJSON(&buf, events)

	require.NoError(t, err)

	var output JSONHistoryOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Equal(t, len(events), output.Count)
	assert.Len(t, output.Events, len(events))
}

func TestFormatJSON_PayloadPreserved_RawJSON(t *testing.T) {
	payload := []byte(`{"location_id":"loc1","move_type":"permanent"}`)
	event := &database.Event{
		EventID:      1,
		EventType:    "item.moved",
		TimestampUTC: "2026-02-24T12:00:00Z",
		ActorUserID:  "alice",
		Payload:      payload,
	}

	var buf bytes.Buffer
	err := formatJSON(&buf, []*database.Event{event})

	require.NoError(t, err)

	var output JSONHistoryOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	// Payload will be re-serialized with indentation, so just check content
	payloadStr := string(output.Events[0].Payload)
	assert.Contains(t, payloadStr, "location_id")
	assert.Contains(t, payloadStr, "loc1")
	assert.Contains(t, payloadStr, "move_type")
	assert.Contains(t, payloadStr, "permanent")
}

func TestFormatJSON_NoteOmitted_WhenNil(t *testing.T) {
	event := &database.Event{
		EventID:      1,
		EventType:    "item.created",
		TimestampUTC: "2026-02-24T12:00:00Z",
		ActorUserID:  "alice",
		Payload:      []byte(`{}`),
		Note:         nil,
	}

	var buf bytes.Buffer
	err := formatJSON(&buf, []*database.Event{event})

	require.NoError(t, err)

	// Check JSON doesn't include "note" field
	assert.NotContains(t, buf.String(), `"note"`)
}

func TestFormatJSON_NoteIncluded_WhenPresent(t *testing.T) {
	note := "temporary move"
	event := &database.Event{
		EventID:      1,
		EventType:    "item.moved",
		TimestampUTC: "2026-02-24T12:00:00Z",
		ActorUserID:  "alice",
		Payload:      []byte(`{}`),
		Note:         &note,
	}

	var buf bytes.Buffer
	err := formatJSON(&buf, []*database.Event{event})

	require.NoError(t, err)

	var output JSONHistoryOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	require.NotNil(t, output.Events[0].Note)
	assert.Equal(t, "temporary move", *output.Events[0].Note)
}

func TestConvertToJSONEvent_PreservesAllFields(t *testing.T) {
	note := "test note"
	event := &database.Event{
		EventID:      123,
		EventType:    "item.moved",
		TimestampUTC: "2026-02-24T12:00:00Z",
		ActorUserID:  "alice",
		Payload:      []byte(`{"test": "data"}`),
		Note:         &note,
	}

	jsonEvent := convertToJSONEvent(event)

	assert.Equal(t, int64(123), jsonEvent.EventID)
	assert.Equal(t, "item.moved", jsonEvent.EventType)
	assert.Equal(t, "2026-02-24T12:00:00Z", jsonEvent.TimestampUTC)
	assert.Equal(t, "alice", jsonEvent.ActorUserID)
	require.NotNil(t, jsonEvent.Note)
	assert.Equal(t, "test note", *jsonEvent.Note)
}

func TestFormatTimestamp_RecentEvent_ShowsRelative(t *testing.T) {
	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)

	result := formatTimestamp(twoHoursAgo.Format(time.RFC3339))

	assert.Contains(t, result, "hour")
	assert.NotContains(t, result, "-")
}

func TestFormatTimestamp_OldEvent_ShowsAbsolute(t *testing.T) {
	oldTime := time.Now().AddDate(0, 0, -10)

	result := formatTimestamp(oldTime.Format(time.RFC3339))

	// Should be in YYYY-MM-DD HH:MM format
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}`, result)
}

func TestFormatTimestamp_JustNow_ShowsMinimalTime(t *testing.T) {
	now := time.Now()

	result := formatTimestamp(now.Format(time.RFC3339))

	assert.Equal(t, "just now", result)
}

func TestFormatTimestamp_MinutesAgo_ShowsCountdown(t *testing.T) {
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)

	result := formatTimestamp(fiveMinutesAgo.Format(time.RFC3339))

	assert.Contains(t, result, "minutes ago")
}

func TestFormatTimestamp_HoursAgo_ShowsCountdown(t *testing.T) {
	threeHoursAgo := time.Now().Add(-3 * time.Hour)

	result := formatTimestamp(threeHoursAgo.Format(time.RFC3339))

	assert.Contains(t, result, "hours ago")
}

func TestFormatTimestamp_DaysAgo_ShowsCountdown(t *testing.T) {
	threeDaysAgo := time.Now().AddDate(0, 0, -3)

	result := formatTimestamp(threeDaysAgo.Format(time.RFC3339))

	assert.Contains(t, result, "days ago")
}

func TestFormatTimestamp_SingularOne_NoPluralS(t *testing.T) {
	oneMinuteAgo := time.Now().Add(-1 * time.Minute)

	result := formatTimestamp(oneMinuteAgo.Format(time.RFC3339))

	assert.Equal(t, "1 minute ago", result)
}

func TestFormatTimestamp_SingularOneHour_NoPluralS(t *testing.T) {
	oneHourAgo := time.Now().Add(-1 * time.Hour)

	result := formatTimestamp(oneHourAgo.Format(time.RFC3339))

	assert.Equal(t, "1 hour ago", result)
}

func TestFormatTimestamp_SingularOneDay_NoPluralS(t *testing.T) {
	oneDayAgo := time.Now().AddDate(0, 0, -1)

	result := formatTimestamp(oneDayAgo.Format(time.RFC3339))

	assert.Equal(t, "1 day ago", result)
}

func TestFormatTimestamp_InvalidFormat_FallsBackToRaw(t *testing.T) {
	result := formatTimestamp("not a valid timestamp")

	assert.Equal(t, "not a valid timestamp", result)
}

func TestFormatRelativeTime_UnderMinute_ShowsJustNow(t *testing.T) {
	result := formatRelativeTime(30 * time.Second)
	assert.Equal(t, "just now", result)
}

func TestFormatRelativeTime_Minutes_Singular(t *testing.T) {
	result := formatRelativeTime(1 * time.Minute)
	assert.Equal(t, "1 minute ago", result)
}

func TestFormatRelativeTime_Minutes_Plural(t *testing.T) {
	result := formatRelativeTime(5 * time.Minute)
	assert.Equal(t, "5 minutes ago", result)
}

func TestFormatRelativeTime_Hours_Singular(t *testing.T) {
	result := formatRelativeTime(1 * time.Hour)
	assert.Equal(t, "1 hour ago", result)
}

func TestFormatRelativeTime_Hours_Plural(t *testing.T) {
	result := formatRelativeTime(3 * time.Hour)
	assert.Equal(t, "3 hours ago", result)
}

func TestFormatRelativeTime_Days_Singular(t *testing.T) {
	result := formatRelativeTime(24 * time.Hour)
	assert.Equal(t, "1 day ago", result)
}

func TestFormatRelativeTime_Days_Plural(t *testing.T) {
	result := formatRelativeTime(7 * 24 * time.Hour)
	assert.Equal(t, "7 days ago", result)
}

func TestFormatRelativeTime_IsReadable(t *testing.T) {
	durations := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"just now", 30 * time.Second, "just now"},
		{"minutes", 5 * time.Minute, "5 minutes ago"},
		{"hours", 2 * time.Hour, "2 hours ago"},
		{"days", 3 * 24 * time.Hour, "3 days ago"},
	}

	for _, tt := range durations {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRelativeTime(tt.duration)
			assert.NotEmpty(t, result)
			assert.NotEmpty(t, result)
		})
	}
}

func TestConvertToJSONEvent_HandlesNilNote(t *testing.T) {
	event := &database.Event{
		EventID:      1,
		EventType:    "item.created",
		TimestampUTC: "2026-02-24T12:00:00Z",
		ActorUserID:  "alice",
		Payload:      []byte(`{}`),
		Note:         nil,
	}

	jsonEvent := convertToJSONEvent(event)

	assert.Nil(t, jsonEvent.Note)
}
