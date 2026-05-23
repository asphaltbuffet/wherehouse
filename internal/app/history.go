package app

import (
	"context"
	"errors"
	"fmt"
)

// GetHistory retrieves the event history for an entity identified by path or ID.
// Results default to newest-first; set OldestFirst to reverse.
// A positive Limit caps the number of returned events.
func (a *App) GetHistory(ctx context.Context, req GetHistoryRequest) ([]HistoryResult, error) {
	var entityID string

	switch {
	case req.EntityID != "":
		entityID = req.EntityID
	case req.EntityPath != "":
		entity, err := a.resolveEntityPath(ctx, req.EntityPath)
		if err != nil {
			return nil, fmt.Errorf("resolve path %q: %w", req.EntityPath, err)
		}
		entityID = entity.EntityID
	default:
		return nil, errors.New("GetHistory: either EntityPath or EntityID must be set")
	}

	events, err := a.store.GetEventsByEntity(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("get history for %s: %w", entityID, err)
	}

	results := make([]HistoryResult, 0, len(events))
	for _, ev := range events {
		note := ""
		if ev.Note != nil {
			note = *ev.Note
		}
		results = append(results, HistoryResult{
			EventID:      ev.EventID,
			EventType:    ev.EventType,
			TimestampUTC: ev.TimestampUTC,
			ActorUserID:  ev.ActorUserID,
			Payload:      []byte(ev.Payload),
			Note:         note,
		})
	}

	if req.OldestFirst {
		if req.Limit > 0 && len(results) > req.Limit {
			results = results[:req.Limit]
		}
		return results, nil
	}

	// Default: newest first — reverse in-place.
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}

	if req.Limit > 0 && len(results) > req.Limit {
		results = results[:req.Limit]
	}

	return results, nil
}
