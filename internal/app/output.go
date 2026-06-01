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
