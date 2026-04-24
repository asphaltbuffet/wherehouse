package database

import "fmt"

//go:generate stringer -type=EntityType -linecomment

// EntityType classifies the behavioral role of an entity.
//
//nolint:recvcheck // Value() requires a value receiver (driver.Valuer); Scan() requires a pointer receiver (sql.Scanner). Mixed receivers are required by the interfaces.
type EntityType int

const (
	// EntityTypePlace is immovable and may only be nested inside other place entities.
	EntityTypePlace EntityType = iota + 1 // place
	// EntityTypeContainer is movable and may contain any entity type.
	EntityTypeContainer // container
	// EntityTypeLeaf is movable and may not contain other entities (enforcement deferred).
	EntityTypeLeaf // leaf
)

var entityTypeByName = map[string]EntityType{
	EntityTypePlace.String():     EntityTypePlace,
	EntityTypeContainer.String(): EntityTypeContainer,
	EntityTypeLeaf.String():      EntityTypeLeaf,
}

// ParseEntityType converts a string like "place" to its EntityType constant.
func ParseEntityType(s string) (EntityType, error) {
	if et, ok := entityTypeByName[s]; ok {
		return et, nil
	}
	return 0, fmt.Errorf("unknown entity type %q: must be place, container, or leaf", s)
}
