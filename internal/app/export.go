package app

import (
	"context"
	"encoding/json"
	"fmt"
)

// ExportResult is the app-layer representation of a single event for export.
type ExportResult struct {
	EventID      int64           `json:"event_id"`
	EventType    string          `json:"event_type"`
	TimestampUTC string          `json:"timestamp_utc"`
	ActorUserID  string          `json:"actor_user_id"`
	EntityID     *string         `json:"entity_id,omitempty"`
	Payload      json.RawMessage `json:"payload"`
	Note         *string         `json:"note,omitempty"`
}

// GetAllEvents returns all events ordered by event_id ASC.
func (a *App) GetAllEvents(ctx context.Context) ([]ExportResult, error) {
	events, err := a.store.GetAllEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all events: %w", err)
	}

	results := make([]ExportResult, len(events))
	for i, ev := range events {
		results[i] = ExportResult{
			EventID:      ev.EventID,
			EventType:    ev.EventType.String(),
			TimestampUTC: ev.TimestampUTC,
			ActorUserID:  ev.ActorUserID,
			EntityID:     ev.EntityID,
			Payload:      ev.Payload,
			Note:         ev.Note,
		}
	}

	return results, nil
}
