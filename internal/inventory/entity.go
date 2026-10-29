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
	HomeID            *string // designated parent; nil = root (see CONTEXT.md "Home")
	FullPathDisplay   string
	FullPathCanonical string
	Depth             int
	Status            EntityStatus
	StatusContext     *string
	LastEventID       int64
	UpdatedAt         time.Time
}

// IsDisplaced reports whether a non-removed entity sits somewhere other than
// its home (see CONTEXT.md "Displaced"). It is derived, never stored.
func (e *Entity) IsDisplaced() bool {
	if e.Status == EntityStatusRemoved {
		return false
	}
	return derefOrEmpty(e.ParentID) != derefOrEmpty(e.HomeID)
}

func derefOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
