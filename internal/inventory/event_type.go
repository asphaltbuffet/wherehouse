package inventory

import "fmt"

//go:generate stringer -type=EventType -linecomment

// EventType identifies the kind of domain event recorded in the event log.
//
//nolint:recvcheck // Value() requires value receiver; Scan() requires pointer receiver.
type EventType int

//nolint:revive // linecomment strings serve as the stringer output; no separate doc needed
const (
	EntityCreatedEvent         EventType = iota + 1 // entity.created
	EntityRenamedEvent                              // entity.renamed
	EntityReparentedEvent                           // entity.reparented
	EntityPathChangedEvent                          // entity.path_changed
	EntityStatusChangedEvent                        // entity.status_changed
	EntityRemovedEvent                              // entity.removed
	EntityTagAddedEvent                             // entity.tag_added
	EntityTagRemovedEvent                           // entity.tag_removed
	EntityLockedEvent                               // entity.locked
	EntityUnlockedEvent                             // entity.unlocked
	EntityDiscreteSetEvent                          // entity.discrete_set
	EntityDiscreteClearedEvent                      // entity.discrete_cleared
	EntityBorrowedEvent                             // entity.borrowed
)

var eventTypeByName = map[string]EventType{
	EntityCreatedEvent.String():         EntityCreatedEvent,
	EntityRenamedEvent.String():         EntityRenamedEvent,
	EntityReparentedEvent.String():      EntityReparentedEvent,
	EntityPathChangedEvent.String():     EntityPathChangedEvent,
	EntityStatusChangedEvent.String():   EntityStatusChangedEvent,
	EntityRemovedEvent.String():         EntityRemovedEvent,
	EntityTagAddedEvent.String():        EntityTagAddedEvent,
	EntityTagRemovedEvent.String():      EntityTagRemovedEvent,
	EntityLockedEvent.String():          EntityLockedEvent,
	EntityUnlockedEvent.String():        EntityUnlockedEvent,
	EntityDiscreteSetEvent.String():     EntityDiscreteSetEvent,
	EntityDiscreteClearedEvent.String(): EntityDiscreteClearedEvent,
	EntityBorrowedEvent.String():        EntityBorrowedEvent,
}

// ParseEventType converts a string to an EventType, returning an error for unknown values.
func ParseEventType(s string) (EventType, error) {
	if et, ok := eventTypeByName[s]; ok {
		return et, nil
	}
	return 0, fmt.Errorf("unknown event type %q", s)
}
