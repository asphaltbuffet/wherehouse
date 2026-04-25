package database

import "fmt"

//go:generate stringer -type=EventType -linecomment

// EventType identifies the kind of domain event stored in the events table.
//
//nolint:recvcheck // Value() requires a value receiver (driver.Valuer); Scan() requires a pointer receiver (sql.Scanner). Mixed receivers are required by the interfaces.
type EventType int

// Domain event types for the entity model.
const (
	EntityCreatedEvent       EventType = iota + 1 // entity.created
	EntityRenamedEvent                            // entity.renamed
	EntityReparentedEvent                         // entity.reparented
	EntityPathChangedEvent                        // entity.path_changed
	EntityStatusChangedEvent                      // entity.status_changed
	EntityRemovedEvent                            // entity.removed
)

// eventTypeByName maps string representations back to EventType constants.
// Initialized using .String() on each constant so the stringer linecomments
// remain the single source of truth — no separate string literals to maintain.
var eventTypeByName = map[string]EventType{
	EntityCreatedEvent.String():       EntityCreatedEvent,
	EntityRenamedEvent.String():       EntityRenamedEvent,
	EntityReparentedEvent.String():    EntityReparentedEvent,
	EntityPathChangedEvent.String():   EntityPathChangedEvent,
	EntityStatusChangedEvent.String(): EntityStatusChangedEvent,
	EntityRemovedEvent.String():       EntityRemovedEvent,
}

// ParseEventType converts a string like "entity.created" to its EventType constant.
func ParseEventType(s string) (EventType, error) {
	if et, ok := eventTypeByName[s]; ok {
		return et, nil
	}
	return 0, fmt.Errorf("unknown event type %q", s)
}
