package inventory

import "errors"

var (
	// ErrEntityNotFound is returned when a query finds no matching entity.
	ErrEntityNotFound = errors.New("entity not found")
	// ErrEventNotFound is returned when a query finds no matching event.
	ErrEventNotFound = errors.New("event not found")
)
