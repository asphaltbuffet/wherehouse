package app

import "github.com/asphaltbuffet/wherehouse/internal/inventory"

// This file holds the CLI JSON output types and the pure projector functions
// that build them from the rich app.*Result types. Per ADR 0014, each command's
// --json wire shape lives here (with JSON tags), not in the cmd/ layer. The
// enums serialize as their string names via their MarshalJSON methods.

// ListItem is the `list` command's JSON output shape: one row per entity.
type ListItem struct {
	EntityID string                 `json:"entity_id"`
	Path     string                 `json:"path"`
	Type     inventory.EntityType   `json:"type"`
	Status   inventory.EntityStatus `json:"status"`
	Tags     []string               `json:"tags"`
}

// ToListItems projects entity results into the `list` output shape.
func ToListItems(results []EntityResult) []ListItem {
	items := make([]ListItem, len(results))
	for i, e := range results {
		tags := e.Tags
		if tags == nil {
			tags = []string{}
		}
		items[i] = ListItem{
			EntityID: e.EntityID,
			Path:     e.FullPathDisplay,
			Type:     e.EntityType,
			Status:   e.Status,
			Tags:     tags,
		}
	}
	return items
}

// ScryItem is the `scry` command's JSON output shape. It currently coincides
// with ListItem, but is kept separate because scry is expected to gain a match
// Distance field (see issue #216) that list does not have.
type ScryItem struct {
	EntityID string                 `json:"entity_id"`
	Path     string                 `json:"path"`
	Type     inventory.EntityType   `json:"type"`
	Status   inventory.EntityStatus `json:"status"`
	Distance *int                   `json:"distance"`
}

// ToScryItems projects entity results into the `scry` output shape. Callers
// flatten []FindResult to []EntityResult first; the match Distance is dropped
// (see issue #216).
func ToScryItems(results []FindResult, searched bool) []ScryItem {
	items := make([]ScryItem, len(results))
	for i, r := range results {
		var dist *int
		if searched {
			d := r.Distance
			dist = &d
		}
		items[i] = ScryItem{
			EntityID: r.Entity.EntityID,
			Path:     r.Entity.FullPathDisplay,
			Type:     r.Entity.EntityType,
			Status:   r.Entity.Status,
			Distance: dist,
		}
	}
	return items
}

// HistoryItem is the `history` command's JSON output shape: one row per event.
// Payload and Note from HistoryResult are intentionally omitted.
type HistoryItem struct {
	EventID   int64               `json:"event_id"`
	EventType inventory.EventType `json:"event_type"`
	Timestamp string              `json:"timestamp"`
	ActorUser string              `json:"actor_user"`
}

// ToHistoryItems projects history results into the `history` output shape.
func ToHistoryItems(results []HistoryResult) []HistoryItem {
	items := make([]HistoryItem, len(results))
	for i, e := range results {
		items[i] = HistoryItem{
			EventID:   e.EventID,
			EventType: e.EventType,
			Timestamp: e.TimestampUTC,
			ActorUser: e.ActorUserID,
		}
	}
	return items
}

// AddOutput is the `add` command's JSON output shape for a newly created entity.
type AddOutput struct {
	EntityID string `json:"entity_id"`
	Path     string `json:"path"`
}

// ToAddOutput projects a created entity result into the `add` output shape.
func ToAddOutput(result EntityResult) AddOutput {
	return AddOutput{
		EntityID: result.EntityID,
		Path:     result.FullPathDisplay,
	}
}

// MoveOutput is the `move` command's JSON output shape. It reports the entity's
// current location only; the prior location is recorded by the move event and
// surfaced via the history command (ADR 0014).
type MoveOutput struct {
	EntityID    string `json:"entity_id"`
	DisplayName string `json:"display_name"`
	Path        string `json:"path"`
}

// ToMoveOutput projects a moved entity result into the `move` output shape.
func ToMoveOutput(result EntityResult) MoveOutput {
	return MoveOutput{
		EntityID:    result.EntityID,
		DisplayName: result.DisplayName,
		Path:        result.FullPathDisplay,
	}
}

// StatusOutput is the `status` command's JSON output shape. Because ChangeStatus
// returns no result, this is projected from the command's inputs rather than an
// EntityResult (see issue #214). StatusContext is omitted when there is no note.
type StatusOutput struct {
	Path          string                 `json:"path"`
	Status        inventory.EntityStatus `json:"status"`
	StatusContext *string                `json:"status_context,omitempty"`
}

// ToStatusOutput projects a status change into the `status` output shape. An
// empty note yields a nil StatusContext, which omitempty drops from the JSON.
func ToStatusOutput(path string, status inventory.EntityStatus, note string) StatusOutput {
	var sc *string
	if note != "" {
		sc = &note
	}
	return StatusOutput{
		Path:          path,
		Status:        status,
		StatusContext: sc,
	}
}

// TagOutput is the --json output shape for the tag command.
type TagOutput struct {
	Path string   `json:"path"`
	Tags []string `json:"tags"`
}

// ToTagOutput builds a TagOutput from a path and sorted tag slice.
func ToTagOutput(path string, tags []string) TagOutput {
	if tags == nil {
		tags = []string{}
	}
	return TagOutput{Path: path, Tags: tags}
}
