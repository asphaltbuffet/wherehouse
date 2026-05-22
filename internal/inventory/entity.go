package inventory

import "time"

// Entity represents a single inventory node: a place, container, or leaf item.
type Entity struct {
	EntityID          string
	DisplayName       string
	CanonicalName     string
	EntityType        EntityType
	ParentID          *string
	FullPathDisplay   string
	FullPathCanonical string
	Depth             int
	Status            EntityStatus
	StatusContext     *string
	LastEventID       int64
	UpdatedAt         time.Time
}
