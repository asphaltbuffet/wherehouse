package inventory

import "fmt"

//go:generate stringer -type=EntityType -linecomment

//nolint:recvcheck // Value() requires value receiver; Scan() requires pointer receiver.
type EntityType int

const (
	EntityTypePlace     EntityType = iota + 1 // place
	EntityTypeContainer                        // container
	EntityTypeLeaf                             // leaf
)

var entityTypeByName = map[string]EntityType{
	EntityTypePlace.String():     EntityTypePlace,
	EntityTypeContainer.String(): EntityTypeContainer,
	EntityTypeLeaf.String():      EntityTypeLeaf,
}

func ParseEntityType(s string) (EntityType, error) {
	if et, ok := entityTypeByName[s]; ok {
		return et, nil
	}
	return 0, fmt.Errorf("unknown entity type %q: must be place, container, or leaf", s)
}
