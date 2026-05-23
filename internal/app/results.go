package app

import "github.com/asphaltbuffet/wherehouse/internal/inventory"

// EntityResult is the output representation of an entity.
type EntityResult struct {
	EntityID        string
	DisplayName     string
	CanonicalName   string
	EntityType      inventory.EntityType
	FullPathDisplay string
	Status          inventory.EntityStatus
	StatusContext   string
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

func entityToResult(e *inventory.Entity) EntityResult {
	ctx := ""
	if e.StatusContext != nil {
		ctx = *e.StatusContext
	}
	return EntityResult{
		EntityID:        e.EntityID,
		DisplayName:     e.DisplayName,
		CanonicalName:   e.CanonicalName,
		EntityType:      e.EntityType,
		FullPathDisplay: e.FullPathDisplay,
		Status:          e.Status,
		StatusContext:   ctx,
	}
}
