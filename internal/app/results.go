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
	ParentID        string // empty for root entities
	HomeID          string // empty when the home is the root
	HomePathDisplay string // FullPathDisplay of the home; filled only when displaced and the home is not root
	FullPathDisplay string
	Status          inventory.EntityStatus
	StatusContext   string
	HasChildren     bool
	Tags            []string
	UpdatedAt       time.Time
}

// RelocationResult is the outcome of a move or use: the entity as read after
// the write, and the parent it left as read inside the writing transaction, so
// a concurrent move cannot make FromParentID stale.
type RelocationResult struct {
	Entity       EntityResult
	FromParentID string // empty when it left the root
}

// IsEligibleParent reports whether the entity may receive a new child:
// any entity that is neither removed nor discrete (see CONTEXT.md "Eligible parent").
func (r EntityResult) IsEligibleParent() bool {
	return r.Status != inventory.EntityStatusRemoved && !r.Discrete
}

// IsDisplaced reports whether a non-removed entity is somewhere other than its
// home (CONTEXT.md "Displaced").
func (r EntityResult) IsDisplaced() bool {
	return r.Status != inventory.EntityStatusRemoved && r.ParentID != r.HomeID
}

// rootHomeLabel is how a root home is shown, in text and JSON alike.
const rootHomeLabel = "(root)"

// HomeLabel is the home's display path, or "(root)" for a root home.
func (r EntityResult) HomeLabel() string {
	if r.HomeID == "" {
		return rootHomeLabel
	}
	return r.HomePathDisplay
}

// CanMove reports whether the entity may be moved: not locked, and ok or
// borrowed (see CONTEXT.md "Move", ADR 0034).
func (r EntityResult) CanMove() bool {
	return !r.Locked &&
		(r.Status == inventory.EntityStatusOk || r.Status == inventory.EntityStatusBorrowed)
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
	parentID := ""
	if e.ParentID != nil {
		parentID = *e.ParentID
	}
	homeID := ""
	if e.HomeID != nil {
		homeID = *e.HomeID
	}
	return EntityResult{
		EntityID:        e.EntityID,
		DisplayName:     e.DisplayName,
		CanonicalName:   e.CanonicalName,
		Locked:          e.Locked,
		Discrete:        e.Discrete,
		ParentID:        parentID,
		HomeID:          homeID,
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
	return a.withHomePath(ctx, entityToResult(e, tags))
}
