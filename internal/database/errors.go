package database

import (
	"errors"
	"fmt"
)

// Common database errors.
var (
	// ErrDatabasePathRequired is returned when a database path is not provided.
	ErrDatabasePathRequired = errors.New("database path is required")

	// ErrEventNotFound is returned when an event is not found.
	ErrEventNotFound = errors.New("event not found")

	// ErrLocationNotFound is returned when a location is not found.
	ErrLocationNotFound = errors.New("location not found")

	// ErrItemNotFound is returned when an item is not found.
	ErrItemNotFound = errors.New("item not found")

	// ErrProjectNotFound is returned when a project is not found.
	ErrProjectNotFound = errors.New("project not found")
)

// InvalidFromLocationError is returned when the from_location_id doesn't match the current location.
type InvalidFromLocationError struct {
	ItemID           string
	ExpectedLocation string
	ActualLocation   string
}

func (e *InvalidFromLocationError) Error() string {
	return fmt.Sprintf("from_location mismatch: item %s is in location %s, not %s",
		e.ItemID, e.ActualLocation, e.ExpectedLocation)
}

// LocationCycleError is returned when a location parent change would create a cycle.
type LocationCycleError struct {
	LocationID string
	ParentID   string
	Cycle      []string // The cycle path for debugging
}

func (e *LocationCycleError) Error() string {
	if len(e.Cycle) > 0 {
		return fmt.Sprintf("location cycle detected: cannot set parent of %s to %s (cycle: %v)",
			e.LocationID, e.ParentID, e.Cycle)
	}

	return fmt.Sprintf("location cycle detected: cannot set parent of %s to %s (would create circular reference)",
		e.LocationID, e.ParentID)
}

// DuplicateLocationError is returned when a location with the same canonical name and parent already exists.
type DuplicateLocationError struct {
	CanonicalName string
	ParentID      *string
	ExistingID    string
}

func (e *DuplicateLocationError) Error() string {
	parentStr := "root"
	if e.ParentID != nil {
		parentStr = *e.ParentID
	}

	return fmt.Sprintf("duplicate location: a location named %q already exists in parent %s (existing ID: %s)",
		e.CanonicalName, parentStr, e.ExistingID)
}

// DuplicateItemError is returned when an item with the same canonical name already exists in a location.
type DuplicateItemError struct {
	CanonicalName string
	LocationID    string
	ExistingID    string
}

func (e *DuplicateItemError) Error() string {
	return fmt.Sprintf("duplicate item: an item named %q already exists in location %s (existing ID: %s)",
		e.CanonicalName, e.LocationID, e.ExistingID)
}

// ProjectNotActiveError is returned when attempting to use an inactive project.
type ProjectNotActiveError struct {
	ProjectID      string
	Status         string // Deprecated: use CurrentStatus
	CurrentStatus  string
	RequiredStatus *string // Optional: the required status if specific
}

func (e *ProjectNotActiveError) Error() string {
	status := e.CurrentStatus
	if status == "" {
		status = e.Status // Fallback for backward compatibility
	}
	if e.RequiredStatus != nil {
		return fmt.Sprintf("project not active: project %s has status %q (expected %q)",
			e.ProjectID, status, *e.RequiredStatus)
	}
	return fmt.Sprintf("project not active: project %s has status %q (expected 'active')",
		e.ProjectID, status)
}

// AmbiguousLocationError is returned when multiple locations match a canonical name.
type AmbiguousLocationError struct {
	CanonicalName string
	MatchingIDs   []string
}

func (e *AmbiguousLocationError) Error() string {
	return fmt.Sprintf("ambiguous location: multiple locations match canonical name %q (IDs: %v)",
		e.CanonicalName, e.MatchingIDs)
}
