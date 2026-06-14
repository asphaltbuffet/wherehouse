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
	Locked   bool                   `json:"locked"`
	Discrete bool                   `json:"discrete"`
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
			Locked:   e.Locked,
			Discrete: e.Discrete,
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
	Locked   bool                   `json:"locked"`
	Discrete bool                   `json:"discrete"`
	Status   inventory.EntityStatus `json:"status"`
	Distance *int                   `json:"distance,omitempty"`
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
			Locked:   r.Entity.Locked,
			Discrete: r.Entity.Discrete,
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

// ToAddOutputs projects created entity results into the `add` output shape.
func ToAddOutputs(results []EntityResult) []AddOutput {
	out := make([]AddOutput, len(results))
	for i, r := range results {
		out[i] = AddOutput{
			EntityID: r.EntityID,
			Path:     r.FullPathDisplay,
		}
	}
	return out
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

// StatusOutput is the `status` command's JSON output shape. Projected from the
// EntityResult returned by ChangeStatus. StatusContext is omitted when there is no note.
type StatusOutput struct {
	Path          string                 `json:"path"`
	Status        inventory.EntityStatus `json:"status"`
	StatusContext *string                `json:"status_context,omitempty"`
}

// ToStatusOutput projects a status change into the `status` output shape. An
// empty note yields a nil StatusContext, which omitempty drops from the JSON.
func ToStatusOutput(result EntityResult) StatusOutput {
	var sc *string
	if result.StatusContext != "" {
		sc = &result.StatusContext
	}
	return StatusOutput{
		Path:          result.FullPathDisplay,
		Status:        result.Status,
		StatusContext: sc,
	}
}

// ToStatusOutputs converts a slice of EntityResult to a slice of StatusOutput for JSON serialization.
func ToStatusOutputs(results []EntityResult) []StatusOutput {
	out := make([]StatusOutput, len(results))
	for i, r := range results {
		out[i] = ToStatusOutput(r)
	}
	return out
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

// BulkAddOutput is the JSON shape for a bulk-add operation.
type BulkAddOutput struct {
	Created  []AddOutput `json:"created"`
	Skipped  []BulkSkip  `json:"skipped"`
	Warnings []string    `json:"warnings"`
}

// ToBulkAddOutput converts a BulkAddResult to its JSON output shape.
func ToBulkAddOutput(r BulkAddResult) BulkAddOutput {
	created := make([]AddOutput, len(r.Created))
	for i, e := range r.Created {
		created[i] = AddOutput{EntityID: e.EntityID, Path: e.FullPathDisplay}
	}

	skipped := r.Skipped
	if skipped == nil {
		skipped = []BulkSkip{}
	}

	warnings := r.Warnings
	if warnings == nil {
		warnings = []string{}
	}

	return BulkAddOutput{
		Created:  created,
		Skipped:  skipped,
		Warnings: warnings,
	}
}

// InfoOutput is the --json output shape for the info command.
type InfoOutput struct {
	Name     string         `json:"name"`
	Database string         `json:"database"`
	Entities map[string]int `json:"entities"`
}

// ToInfoOutput projects an InfoResult into the info command's JSON output shape.
func ToInfoOutput(r InfoResult) InfoOutput {
	return InfoOutput{
		Name:     r.Name,
		Database: r.DatabasePath,
		Entities: r.EntityCounts,
	}
}
