package database

import "fmt"

//go:generate stringer -type=EventType -linecomment

// EventType is an enumeration of event types.
//
//nolint:recvcheck // Value() requires a value receiver (driver.Valuer); Scan() requires a pointer receiver (sql.Scanner). Mixed receivers are required by the interfaces.
type EventType int

const (
	// ItemCreatedEvent records a new item being added to the inventory at a location.
	ItemCreatedEvent EventType = iota + 1 // item.created
	// ItemMovedEvent records an item being relocated from one storage location to another.
	ItemMovedEvent // item.moved
	// ItemMissingEvent records that an item could not be found at its expected location.
	ItemMissingEvent // item.missing
	// ItemBorrowedEvent records an item being taken by someone with the intent to return it.
	ItemBorrowedEvent // item.borrowed
	// ItemLoanedEvent records an item being lent out to someone else.
	ItemLoanedEvent // item.loaned
	// ItemFoundEvent records a previously missing item being located again.
	ItemFoundEvent // item.found
	// ItemRemovedEvent records an item being moved to the Removed system location.
	ItemRemovedEvent // item.removed

	// LocationCreatedEvent records a new storage location being added.
	LocationCreatedEvent // location.created
	// LocationRenamedEvent records a location's display name being changed.
	LocationRenamedEvent // location.renamed
	// LocationMovedEvent records a location being reparented under a different location.
	LocationMovedEvent // location.reparented
	// LocationRemovedEvent records a location being moved to the Removed system location.
	LocationRemovedEvent // location.removed

	// ProjectCreatedEvent records a new project being opened to group related items.
	ProjectCreatedEvent // project.created
	// ProjectCompletedEvent records a project being marked as finished.
	ProjectCompletedEvent // project.completed
	// ProjectReopenedEvent records a completed project being re-activated.
	ProjectReopenedEvent // project.reopened
)

// eventTypeByName maps string representations back to EventType constants.
// Initialized using .String() on each constant so the stringer linecomments
// remain the single source of truth — no separate string literals to maintain.
var eventTypeByName = map[string]EventType{
	ItemCreatedEvent.String():      ItemCreatedEvent,
	ItemMovedEvent.String():        ItemMovedEvent,
	ItemMissingEvent.String():      ItemMissingEvent,
	ItemBorrowedEvent.String():     ItemBorrowedEvent,
	ItemLoanedEvent.String():       ItemLoanedEvent,
	ItemFoundEvent.String():        ItemFoundEvent,
	ItemRemovedEvent.String():      ItemRemovedEvent,
	LocationCreatedEvent.String():  LocationCreatedEvent,
	LocationRenamedEvent.String():  LocationRenamedEvent,
	LocationMovedEvent.String():    LocationMovedEvent,
	LocationRemovedEvent.String():  LocationRemovedEvent,
	ProjectCreatedEvent.String():   ProjectCreatedEvent,
	ProjectCompletedEvent.String(): ProjectCompletedEvent,
	ProjectReopenedEvent.String():  ProjectReopenedEvent,
}

// ParseEventType converts a string representation to an EventType constant.
// Returns an error for unrecognized strings to fail loudly on mismatch.
func ParseEventType(s string) (EventType, error) {
	if et, ok := eventTypeByName[s]; ok {
		return et, nil
	}
	return 0, fmt.Errorf("unknown event type %q", s)
}
