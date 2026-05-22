package inventory

import "errors"

// Sentinel errors returned by inventory operations.
var (
	ErrEntityNotFound = errors.New("entity not found")
	ErrEventNotFound  = errors.New("event not found")
)
