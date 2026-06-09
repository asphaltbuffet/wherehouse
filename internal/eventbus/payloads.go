package eventbus

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

// EntityPathChangedPayload is the JSON payload for an EntityPathChangedEvent.
type EntityPathChangedPayload struct {
	EntityID          string `json:"entity_id"`
	FullPathDisplay   string `json:"full_path_display"`
	FullPathCanonical string `json:"full_path_canonical"`
	Depth             int    `json:"depth"`
}

// EntityStatusChangedPayload is the JSON payload for an EntityStatusChangedEvent.
type EntityStatusChangedPayload struct {
	EntityID      string  `json:"entity_id"`
	Status        string  `json:"status"`
	StatusContext *string `json:"status_context,omitempty"`
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
