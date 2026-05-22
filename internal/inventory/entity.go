package inventory

import "time"

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
