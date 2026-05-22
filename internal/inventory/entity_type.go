package inventory

import "fmt"

//go:generate stringer -type=EntityType -linecomment

// EntityType classifies an inventory entity as a place, container, or leaf item.
//
//nolint:recvcheck // Value() requires value receiver; Scan() requires pointer receiver.
type EntityType int

//nolint:revive // linecomment strings serve as the stringer output; no separate doc needed
const (
	EntityTypePlace     EntityType = iota + 1 // place
	EntityTypeContainer                       // container
	EntityTypeLeaf                            // leaf
)

var entityTypeByName = map[string]EntityType{
	EntityTypePlace.String():     EntityTypePlace,
	EntityTypeContainer.String(): EntityTypeContainer,
	EntityTypeLeaf.String():      EntityTypeLeaf,
}

// ParseEntityType converts a string to an EntityType, returning an error for unknown values.
func ParseEntityType(s string) (EntityType, error) {
	if et, ok := entityTypeByName[s]; ok {
		return et, nil
	}
	return 0, fmt.Errorf("unknown entity type: %q", s)
}
