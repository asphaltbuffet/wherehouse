package app

import "github.com/asphaltbuffet/wherehouse/internal/entitypath"

// EntityFilter is the AND of the entity filter's criteria shared by `list`
// and web Reports (CONTEXT.md "Report"); zero values match everything.
type EntityFilter struct {
	Status    string          // EntityStatus name; "" matches any status
	Under     entitypath.Path // current location at or under this path (inclusive)
	HomeUnder entitypath.Path // home at or under this path (inclusive)
	Displaced bool
}

// Matches reports whether e satisfies every criterion of f.
func (f EntityFilter) Matches(e EntityResult) bool {
	if f.Status != "" && e.Status.String() != f.Status {
		return false
	}
	if f.Displaced && !e.IsDisplaced() {
		return false
	}
	if !f.Under.IsEmpty() && !pathHasPrefix(e.FullPathDisplay, f.Under) {
		return false
	}
	if !f.HomeUnder.IsEmpty() && !pathHasPrefix(homePathOf(e), f.HomeUnder) {
		return false
	}
	return true
}

// FilterEntities returns the entities of all that match f, in their original order.
func FilterEntities(all []EntityResult, f EntityFilter) []EntityResult {
	var out []EntityResult
	for _, e := range all {
		if f.Matches(e) {
			out = append(out, e)
		}
	}
	return out
}

// homePathOf is the display path of e's home: its parent's path when at home.
// Root-homed entities yield "" and never match a prefix.
func homePathOf(e EntityResult) string {
	if e.IsDisplaced() {
		return e.HomePathDisplay
	}
	p, err := entitypath.Parse(e.FullPathDisplay)
	if err != nil {
		return ""
	}
	return p.Dir().String()
}

func pathHasPrefix(p string, prefix entitypath.Path) bool {
	ep, err := entitypath.Parse(p)
	if err != nil {
		return false
	}
	return ep.HasPrefix(prefix)
}
