package inventory

import "fmt"

//go:generate stringer -type=EventType -linecomment

// EventType identifies the kind of domain event recorded in the event log.
//
//nolint:recvcheck // Value() requires value receiver; Scan() requires pointer receiver.
type EventType int

const (
	// EntityCreatedEvent records that a new entity was added to the inventory.
	EntityCreatedEvent EventType = iota + 1 // entity.created
	// EntityRenamedEvent records that an entity's display name changed.
	EntityRenamedEvent // entity.renamed
	// EntityReparentedEvent records that an entity was moved to a new parent.
	EntityReparentedEvent // entity.reparented
	// EntityPathChangedEvent records a derived path update propagated from an ancestor reparent.
	EntityPathChangedEvent // entity.path_changed
	// EntityStatusChangedEvent records that an entity's status (and optional context) changed.
	EntityStatusChangedEvent // entity.status_changed
	// EntityRemovedEvent records that an entity was soft-deleted. The entity remains in the projection.
	EntityRemovedEvent // entity.removed
	// EntityTagAddedEvent records that a tag was applied to an entity.
	EntityTagAddedEvent // entity.tag_added
	// EntityTagRemovedEvent records that a tag was removed from an entity.
	EntityTagRemovedEvent // entity.tag_removed
	// EntityLockedEvent records that an entity's locked flag was set to true.
	EntityLockedEvent // entity.locked
	// EntityUnlockedEvent records that an entity's locked flag was set to false.
	EntityUnlockedEvent // entity.unlocked
	// EntityDiscreteSetEvent records that an entity's discrete flag was set to true.
	EntityDiscreteSetEvent // entity.discrete_set
	// EntityDiscreteClearedEvent records that an entity's discrete flag was set to false.
	EntityDiscreteClearedEvent // entity.discrete_cleared
	// EntityBorrowedEvent records that a new entity was created directly in borrowed status.
	// This is atomic — there is no preceding EntityCreatedEvent.
	EntityBorrowedEvent // entity.borrowed
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
