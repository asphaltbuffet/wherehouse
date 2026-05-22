package inventory

import "fmt"

//go:generate stringer -type=EntityStatus -linecomment

// EntityStatus represents the lifecycle/availability state of an inventory entity.
//
//nolint:recvcheck // Value() requires value receiver; Scan() requires pointer receiver.
type EntityStatus int

// EntityStatus values represent the canonical set of lifecycle states.
const (
	EntityStatusOk       EntityStatus = iota + 1 // ok
	EntityStatusBorrowed                         // borrowed
	EntityStatusMissing                          // missing
	EntityStatusLoaned                           // loaned
	EntityStatusRemoved                          // removed
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
