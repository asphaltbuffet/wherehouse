package inventory

import "errors"

//nolint:revive // sentinel errors; names are self-documenting
var (
	ErrEntityNotFound = errors.New("entity not found")
	ErrEventNotFound  = errors.New("event not found")
)
