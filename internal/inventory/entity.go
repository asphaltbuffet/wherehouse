package inventory

import "time"

// Entity represents a single inventory node. Entities have no type classification;
// they are distinguished only by position in the hierarchy and the mutable
// Locked and Discrete flags.
type Entity struct {
	EntityID          string
	DisplayName       string
	CanonicalName     string
	Locked            bool
	Discrete          bool
	ParentID          *string
	FullPathDisplay   string
	FullPathCanonical string
	Depth             int
	Status            EntityStatus
	StatusContext     *string
	LastEventID       int64
	UpdatedAt         time.Time
}
