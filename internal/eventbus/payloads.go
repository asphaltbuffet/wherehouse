package eventbus

// EntityCreatedPayload is the JSON payload for an EntityCreatedEvent.
type EntityCreatedPayload struct {
	EntityID    string  `json:"entity_id"`
	DisplayName string  `json:"display_name"`
	EntityType  string  `json:"entity_type"`
	ParentID    *string `json:"parent_id,omitempty"`
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
