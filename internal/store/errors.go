package store

import "errors"

var (
	// ErrDatabasePathRequired is returned when Open is called with an empty path.
	ErrDatabasePathRequired = errors.New("database path is required")
	// ErrNotFound is returned when a requested record does not exist.
	ErrNotFound = errors.New("not found")
)
