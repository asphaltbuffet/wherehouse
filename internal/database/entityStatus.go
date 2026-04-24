package database

import "fmt"

//go:generate stringer -type=EntityStatus -linecomment

// EntityStatus represents the current lifecycle state of an entity.
//
//nolint:recvcheck // Value() requires a value receiver (driver.Valuer); Scan() requires a pointer receiver (sql.Scanner). Mixed receivers are required by the interfaces.
type EntityStatus int

const (
	// EntityStatusOk is the default state — the entity is accounted for at its location.
	EntityStatusOk EntityStatus = iota + 1 // ok
	// EntityStatusBorrowed means the entity has been taken temporarily by someone.
	EntityStatusBorrowed // borrowed
	// EntityStatusMissing means the entity cannot be found at its expected location.
	EntityStatusMissing // missing
	// EntityStatusLoaned means the entity has been lent out to someone else.
	EntityStatusLoaned // loaned
	// EntityStatusRemoved means the entity has been removed from the inventory.
	EntityStatusRemoved // removed
)

var entityStatusByName = map[string]EntityStatus{
	EntityStatusOk.String():       EntityStatusOk,
	EntityStatusBorrowed.String(): EntityStatusBorrowed,
	EntityStatusMissing.String():  EntityStatusMissing,
	EntityStatusLoaned.String():   EntityStatusLoaned,
	EntityStatusRemoved.String():  EntityStatusRemoved,
}

// ParseEntityStatus converts a string like "borrowed" to its EntityStatus constant.
func ParseEntityStatus(s string) (EntityStatus, error) {
	if es, ok := entityStatusByName[s]; ok {
		return es, nil
	}
	return 0, fmt.Errorf("unknown entity status %q: must be ok, borrowed, missing, loaned, or removed", s)
}
