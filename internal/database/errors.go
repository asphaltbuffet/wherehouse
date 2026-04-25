package database

import (
	"errors"
)

// Common database errors.
var (
	// ErrDatabasePathRequired is returned when a database path is not provided.
	ErrDatabasePathRequired = errors.New("database path is required")

	// ErrEventNotFound is returned when an event is not found.
	ErrEventNotFound = errors.New("event not found")

	// ErrEntityNotFound is returned when an entity is not found.
	ErrEntityNotFound = errors.New("entity not found")
)
