package app

import (
	"context"
	"fmt"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// EntityResult is the output representation of an entity.
type EntityResult struct {
	EntityID        string
	DisplayName     string
	CanonicalName   string
	Locked          bool
	Discrete        bool
	FullPathDisplay string
	Status          inventory.EntityStatus
	StatusContext   string
	HasChildren     bool
	Tags            []string
	UpdatedAt       time.Time
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

// InfoResult is the output of GetInfo — database name, path, and entity counts by status.
type InfoResult struct {
	Name         string
	DatabasePath string
	EntityCounts map[string]int
}

// FindResult pairs an EntityResult with its fuzzy-match distance.
type FindResult struct {
	Entity   EntityResult
	Distance int
}

func entityToResult(e *inventory.Entity, tags []string) EntityResult {
	statusCtx := ""
	if e.StatusContext != nil {
		statusCtx = *e.StatusContext
	}
	return EntityResult{
		EntityID:        e.EntityID,
		DisplayName:     e.DisplayName,
		CanonicalName:   e.CanonicalName,
		Locked:          e.Locked,
		Discrete:        e.Discrete,
		FullPathDisplay: e.FullPathDisplay,
		Status:          e.Status,
		StatusContext:   statusCtx,
		Tags:            tags,
		UpdatedAt:       e.UpdatedAt,
	}
}

// entityWithTags fetches tags for a single entity and returns the full EntityResult.
// Use this for point-lookup operations. For list operations use store.GetTagsByEntities
// with entityToResult directly to avoid N+1 queries (see ADR 0015).
func (a *App) entityWithTags(ctx context.Context, e *inventory.Entity) (EntityResult, error) {
	tags, err := a.store.GetTagsByEntity(ctx, e.EntityID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get tags for %q: %w", e.EntityID, err)
	}
	return entityToResult(e, tags), nil
}
