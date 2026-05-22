package inventory

import "fmt"

//go:generate stringer -type=EntityStatus -linecomment

//nolint:recvcheck
type EntityStatus int

const (
	EntityStatusOk       EntityStatus = iota + 1 // ok
	EntityStatusBorrowed                          // borrowed
	EntityStatusMissing                           // missing
	EntityStatusLoaned                            // loaned
	EntityStatusRemoved                           // removed
)

var entityStatusByName = map[string]EntityStatus{
	EntityStatusOk.String():       EntityStatusOk,
	EntityStatusBorrowed.String(): EntityStatusBorrowed,
	EntityStatusMissing.String():  EntityStatusMissing,
	EntityStatusLoaned.String():   EntityStatusLoaned,
	EntityStatusRemoved.String():  EntityStatusRemoved,
}

func ParseEntityStatus(s string) (EntityStatus, error) {
	if es, ok := entityStatusByName[s]; ok {
		return es, nil
	}
	return 0, fmt.Errorf("unknown entity status %q", s)
}
