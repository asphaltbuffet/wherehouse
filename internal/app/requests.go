package app

// CreateEntityRequest is the input for creating a new entity.
type CreateEntityRequest struct {
	DisplayName string
	Locked      bool
	Discrete    bool
	ParentID    string // empty means root-level
	ActorID     string
	Note        string
}

// RenameEntityRequest is the input for renaming an entity.
type RenameEntityRequest struct {
	EntityID string
	NewName  string
	ActorID  string
	Note     string
}

// ReparentEntityRequest is the input for moving an entity to a new parent.
type ReparentEntityRequest struct {
	EntityID    string
	NewParentID string // empty means make root-level
	ActorID     string
	Note        string
}

// RemoveEntityRequest is the input for removing an entity.
type RemoveEntityRequest struct {
	EntityID string
	ActorID  string
	Note     string
}

// ChangeStatusRequest is the input for the intent-specific status transitions
// (MarkLost, MarkFound, MarkLoaned, MarkReturned). The target status is implied
// by the method called; StatusContext carries the loan recipient for MarkLoaned.
type ChangeStatusRequest struct {
	EntityID      string
	StatusContext string
	ActorID       string
	Note          string
}

// SetAttributesRequest is the input for changing an entity's locked and/or
// discrete attributes. A nil field leaves that attribute unchanged.
type SetAttributesRequest struct {
	EntityID string
	Locked   *bool
	Discrete *bool
	ActorID  string
	Note     string
}

// BorrowEntityRequest is the input for borrowing an external entity into the inventory.
type BorrowEntityRequest struct {
	DisplayName   string
	ParentID      string // empty means root-level
	StatusContext string // lender name, stored in --from flag
	ActorID       string
	Note          string
}

// GetHistoryRequest is the input for retrieving an entity's event history.
type GetHistoryRequest struct {
	EntityID    string
	Limit       int
	OldestFirst bool
}

// FindEntitiesRequest is the input for searching entities by name.
type FindEntitiesRequest struct {
	Query string
	Limit int
}

// TagEntityRequest is the input for adding/removing tags on an entity.
type TagEntityRequest struct {
	EntityID string
	ActorID  string
	Add      []string
	Remove   []string
	Note     string
}

// ListTagsRequest is the input for listing tags on an entity.
type ListTagsRequest struct {
	EntityID string
}
