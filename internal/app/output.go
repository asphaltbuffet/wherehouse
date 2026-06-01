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
}

// ToListItems projects entity results into the `list` output shape.
func ToListItems(results []EntityResult) []ListItem {
	items := make([]ListItem, len(results))
	for i, e := range results {
		items[i] = ListItem{
			EntityID: e.EntityID,
			Path:     e.FullPathDisplay,
			Type:     e.EntityType,
			Status:   e.Status,
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
}

// ToScryItems projects entity results into the `scry` output shape. Callers
// flatten []FindResult to []EntityResult first; the match Distance is dropped
// (see issue #216).
func ToScryItems(results []EntityResult) []ScryItem {
	items := make([]ScryItem, len(results))
	for i, e := range results {
		items[i] = ScryItem{
			EntityID: e.EntityID,
			Path:     e.FullPathDisplay,
			Type:     e.EntityType,
			Status:   e.Status,
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
