package inventory

import "fmt"

//go:generate stringer -type=EntityStatus -linecomment

// EntityStatus represents the lifecycle/availability state of an inventory entity.
//
//nolint:recvcheck // Value() requires value receiver; Scan() requires pointer receiver.
type EntityStatus int

const (
	// EntityStatusOk means the entity is at its known location and available.
	EntityStatusOk EntityStatus = iota + 1 // ok
	// EntityStatusBorrowed means an external item brought into the inventory temporarily.
	// The only valid transition out of this status is return, which sets the entity to removed.
	EntityStatusBorrowed // borrowed
	// EntityStatusMissing means the entity's location is unknown.
	EntityStatusMissing // missing
	// EntityStatusLoaned means the entity has been given out to someone else.
	// The entity still exists in the inventory; no new entity is created.
	EntityStatusLoaned // loaned
	// EntityStatusRemoved means the entity has been soft-deleted from the inventory.
	// Removed entities remain in the projection but are excluded from normal queries.
	EntityStatusRemoved // removed
)

var entityStatusByName = map[string]EntityStatus{
	EntityStatusOk.String():       EntityStatusOk,
	EntityStatusBorrowed.String(): EntityStatusBorrowed,
	EntityStatusMissing.String():  EntityStatusMissing,
	EntityStatusLoaned.String():   EntityStatusLoaned,
	EntityStatusRemoved.String():  EntityStatusRemoved,
}

// ParseEntityStatus converts a string to an EntityStatus, returning an error for unknown values.
func ParseEntityStatus(s string) (EntityStatus, error) {
	if es, ok := entityStatusByName[s]; ok {
		return es, nil
	}
	return 0, fmt.Errorf("unknown entity status %q", s)
}
