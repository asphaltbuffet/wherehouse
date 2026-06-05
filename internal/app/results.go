package app

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// EntityResult is the output representation of an entity.
type EntityResult struct {
	EntityID        string
	DisplayName     string
	CanonicalName   string
	EntityType      inventory.EntityType
	FullPathDisplay string
	Status          inventory.EntityStatus
	StatusContext   string
	HasChildren     bool
	Tags            []string
}

// HistoryResult is the output representation of a single history event.
type HistoryResult struct {
	EventID      int64
	EventType    inventory.EventType
	TimestampUTC string
	ActorUserID  string
	Payload      []byte
	Note         string
}

// FindResult pairs an EntityResult with its fuzzy-match distance.
type FindResult struct {
	Entity   EntityResult
	Distance int
}

func (a *App) entityToResult(ctx context.Context, e *inventory.Entity) (EntityResult, error) {
	statusCtx := ""
	if e.StatusContext != nil {
		statusCtx = *e.StatusContext
	}
	tags, err := a.store.GetTagsByEntity(ctx, e.EntityID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get tags for %q: %w", e.EntityID, err)
	}
	return EntityResult{
		EntityID:        e.EntityID,
		DisplayName:     e.DisplayName,
		CanonicalName:   e.CanonicalName,
		EntityType:      e.EntityType,
		FullPathDisplay: e.FullPathDisplay,
		Status:          e.Status,
		StatusContext:   statusCtx,
		Tags:            tags,
	}, nil
}
