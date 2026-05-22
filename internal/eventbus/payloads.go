package eventbus

type EntityCreatedPayload struct {
	EntityID    string  `json:"entity_id"`
	DisplayName string  `json:"display_name"`
	EntityType  string  `json:"entity_type"`
	ParentID    *string `json:"parent_id,omitempty"`
}

type EntityRenamedPayload struct {
	EntityID    string `json:"entity_id"`
	DisplayName string `json:"display_name"`
}

type EntityReparentedPayload struct {
	EntityID    string  `json:"entity_id"`
	NewParentID *string `json:"new_parent_id,omitempty"`
}

type EntityPathChangedPayload struct {
	EntityID          string `json:"entity_id"`
	FullPathDisplay   string `json:"full_path_display"`
	FullPathCanonical string `json:"full_path_canonical"`
	Depth             int    `json:"depth"`
}

type EntityStatusChangedPayload struct {
	EntityID      string  `json:"entity_id"`
	Status        string  `json:"status"`
	StatusContext *string `json:"status_context,omitempty"`
}

type EntityRemovedPayload struct {
	EntityID string `json:"entity_id"`
}
