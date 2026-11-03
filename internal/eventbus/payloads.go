package eventbus

import "github.com/asphaltbuffet/wherehouse/internal/inventory"

// Payload is an event payload that names its own EventType and the entity it
// is about. It is sealed: only eventbus types implement it (ADR 0042).
type Payload interface {
	EventType() inventory.EventType
	// Subject is the entity the event is about; it becomes the event's entity_id.
	Subject() string
	isPayload()
}

// EntityCreatedPayload is the JSON payload for an EntityCreatedEvent.
type EntityCreatedPayload struct {
	EntityID    string  `json:"entity_id"`
	DisplayName string  `json:"display_name"`
	Locked      bool    `json:"locked"`
	Discrete    bool    `json:"discrete"`
	ParentID    *string `json:"parent_id,omitempty"`
	// LegacyEntityType is only populated when replaying old events that predate ADR 0018.
	// It is used to migrate place entities to locked=true during projection rebuild.
	LegacyEntityType string `json:"entity_type,omitempty"`
}

// EntityRenamedPayload is the JSON payload for an EntityRenamedEvent.
type EntityRenamedPayload struct {
	EntityID    string `json:"entity_id"`
	DisplayName string `json:"display_name"`
}

// EntityReparentedPayload is the JSON payload for an EntityReparentedEvent.
type EntityReparentedPayload struct {
	EntityID    string  `json:"entity_id"`
	NewParentID *string `json:"new_parent_id,omitempty"`
}

// EntityUsedPayload is the JSON payload for an EntityUsedEvent.
type EntityUsedPayload struct {
	EntityID    string  `json:"entity_id"`
	NewParentID *string `json:"new_parent_id,omitempty"`
}

// EntityReturnedHomePayload is the JSON payload for an EntityReturnedHomeEvent.
// ParentID is the home the entity returned to, stated explicitly so the event
// is self-describing (nil = root).
type EntityReturnedHomePayload struct {
	EntityID string  `json:"entity_id"`
	ParentID *string `json:"parent_id,omitempty"`
}

// EntityRehomedPayload is the JSON payload for an EntityRehomedEvent.
type EntityRehomedPayload struct {
	EntityID string  `json:"entity_id"`
	HomeID   *string `json:"home_id,omitempty"`
}

// EntityPathChangedPayload is the JSON payload for an EntityPathChangedEvent.
type EntityPathChangedPayload struct {
	EntityID          string `json:"entity_id"`
	FullPathDisplay   string `json:"full_path_display"`
	FullPathCanonical string `json:"full_path_canonical"`
	Depth             int    `json:"depth"`
}

// EntityStatusChangedPayload is the JSON payload for an EntityStatusChangedEvent.
type EntityStatusChangedPayload struct {
	EntityID      string                 `json:"entity_id"`
	Status        inventory.EntityStatus `json:"status"`
	StatusContext *string                `json:"status_context,omitempty"`
}

// EntityRemovedPayload is the JSON payload for an EntityRemovedEvent.
type EntityRemovedPayload struct {
	EntityID string `json:"entity_id"`
}

// EntityTagAddedPayload is the JSON payload for an EntityTagAddedEvent.
type EntityTagAddedPayload struct {
	EntityID string `json:"entity_id"`
	Tag      string `json:"tag"`
}

// EntityTagRemovedPayload is the JSON payload for an EntityTagRemovedEvent.
type EntityTagRemovedPayload struct {
	EntityID string `json:"entity_id"`
	Tag      string `json:"tag"`
}

// EntityLockedPayload is the JSON payload for an EntityLockedEvent.
type EntityLockedPayload struct {
	EntityID string `json:"entity_id"`
}

// EntityUnlockedPayload is the JSON payload for an EntityUnlockedEvent.
type EntityUnlockedPayload struct {
	EntityID string `json:"entity_id"`
}

// EntityDiscreteSetPayload is the JSON payload for an EntityDiscreteSetEvent.
type EntityDiscreteSetPayload struct {
	EntityID string `json:"entity_id"`
}

// EntityDiscreteClearedPayload is the JSON payload for an EntityDiscreteClearedEvent.
type EntityDiscreteClearedPayload struct {
	EntityID string `json:"entity_id"`
}

// EntityBorrowedPayload is the JSON payload for an EntityBorrowedEvent.
type EntityBorrowedPayload struct {
	EntityID      string  `json:"entity_id"`
	DisplayName   string  `json:"display_name"`
	ParentID      *string `json:"parent_id,omitempty"`
	StatusContext *string `json:"status_context,omitempty"`
}

// EventType implements [Payload].
func (EntityCreatedPayload) EventType() inventory.EventType {
	return inventory.EntityCreatedEvent
}

// Subject implements [Payload].
func (p EntityCreatedPayload) Subject() string { return p.EntityID }

func (EntityCreatedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityRenamedPayload) EventType() inventory.EventType {
	return inventory.EntityRenamedEvent
}

// Subject implements [Payload].
func (p EntityRenamedPayload) Subject() string { return p.EntityID }

func (EntityRenamedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityReparentedPayload) EventType() inventory.EventType {
	return inventory.EntityReparentedEvent
}

// Subject implements [Payload].
func (p EntityReparentedPayload) Subject() string { return p.EntityID }

func (EntityReparentedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityUsedPayload) EventType() inventory.EventType {
	return inventory.EntityUsedEvent
}

// Subject implements [Payload].
func (p EntityUsedPayload) Subject() string { return p.EntityID }

func (EntityUsedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityReturnedHomePayload) EventType() inventory.EventType {
	return inventory.EntityReturnedHomeEvent
}

// Subject implements [Payload].
func (p EntityReturnedHomePayload) Subject() string { return p.EntityID }

func (EntityReturnedHomePayload) isPayload() {}

// EventType implements [Payload].
func (EntityRehomedPayload) EventType() inventory.EventType {
	return inventory.EntityRehomedEvent
}

// Subject implements [Payload].
func (p EntityRehomedPayload) Subject() string { return p.EntityID }

func (EntityRehomedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityPathChangedPayload) EventType() inventory.EventType {
	return inventory.EntityPathChangedEvent
}

// Subject implements [Payload].
func (p EntityPathChangedPayload) Subject() string { return p.EntityID }

func (EntityPathChangedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityStatusChangedPayload) EventType() inventory.EventType {
	return inventory.EntityStatusChangedEvent
}

// Subject implements [Payload].
func (p EntityStatusChangedPayload) Subject() string { return p.EntityID }

func (EntityStatusChangedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityRemovedPayload) EventType() inventory.EventType {
	return inventory.EntityRemovedEvent
}

// Subject implements [Payload].
func (p EntityRemovedPayload) Subject() string { return p.EntityID }

func (EntityRemovedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityTagAddedPayload) EventType() inventory.EventType {
	return inventory.EntityTagAddedEvent
}

// Subject implements [Payload].
func (p EntityTagAddedPayload) Subject() string { return p.EntityID }

func (EntityTagAddedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityTagRemovedPayload) EventType() inventory.EventType {
	return inventory.EntityTagRemovedEvent
}

// Subject implements [Payload].
func (p EntityTagRemovedPayload) Subject() string { return p.EntityID }

func (EntityTagRemovedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityLockedPayload) EventType() inventory.EventType {
	return inventory.EntityLockedEvent
}

// Subject implements [Payload].
func (p EntityLockedPayload) Subject() string { return p.EntityID }

func (EntityLockedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityUnlockedPayload) EventType() inventory.EventType {
	return inventory.EntityUnlockedEvent
}

// Subject implements [Payload].
func (p EntityUnlockedPayload) Subject() string { return p.EntityID }

func (EntityUnlockedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityDiscreteSetPayload) EventType() inventory.EventType {
	return inventory.EntityDiscreteSetEvent
}

// Subject implements [Payload].
func (p EntityDiscreteSetPayload) Subject() string { return p.EntityID }

func (EntityDiscreteSetPayload) isPayload() {}

// EventType implements [Payload].
func (EntityDiscreteClearedPayload) EventType() inventory.EventType {
	return inventory.EntityDiscreteClearedEvent
}

// Subject implements [Payload].
func (p EntityDiscreteClearedPayload) Subject() string { return p.EntityID }

func (EntityDiscreteClearedPayload) isPayload() {}

// EventType implements [Payload].
func (EntityBorrowedPayload) EventType() inventory.EventType {
	return inventory.EntityBorrowedEvent
}

// Subject implements [Payload].
func (p EntityBorrowedPayload) Subject() string { return p.EntityID }

func (EntityBorrowedPayload) isPayload() {}
