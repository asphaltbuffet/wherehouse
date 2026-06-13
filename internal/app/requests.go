package app

import "github.com/asphaltbuffet/wherehouse/internal/inventory"

// CreateEntityRequest is the input for creating a new entity.
type CreateEntityRequest struct {
	DisplayName string
	Locked      bool
	Discrete    bool
	// ParentPath is a colon-separated path, e.g. "Garage:Toolbox". Empty means root-level.
	ParentPath    string
	ActorID       string
	Note          string
	CreateParents bool
}

// RenameEntityRequest is the input for renaming an entity.
type RenameEntityRequest struct {
	EntityPath string
	NewName    string
	ActorID    string
	Note       string
}

// ReparentEntityRequest is the input for moving an entity to a new parent.
type ReparentEntityRequest struct {
	EntityPath    string
	NewParentPath string // empty means make root-level
	ActorID       string
	Note          string
}

// RemoveEntityRequest is the input for removing an entity.
type RemoveEntityRequest struct {
	EntityPath string
	ActorID    string
	Note       string
}

// ChangeStatusRequest is the input for changing an entity's status.
type ChangeStatusRequest struct {
	EntityPath    string
	Status        inventory.EntityStatus
	StatusContext string
	ActorID       string
	Note          string
}

// BorrowEntityRequest is the input for borrowing an external entity into the inventory.
type BorrowEntityRequest struct {
	DisplayName   string
	ParentPath    string
	StatusContext string // lender name, stored in --from flag
	ActorID       string
	Note          string
}

// GetHistoryRequest is the input for retrieving an entity's event history.
type GetHistoryRequest struct {
	EntityPath  string
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
	EntityPath string
	ActorID    string
	Add        []string
	Remove     []string
	Note       string
}

// ListTagsRequest is the input for listing tags on an entity.
type ListTagsRequest struct {
	EntityPath string
}
