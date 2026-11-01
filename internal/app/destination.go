package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// hierarchy reads entities by ID in any status, removed included: a live
// entity may sit under a removed one (#287). Every legal-destination rule
// walks upward, so nothing more is needed (ADR 0041).
type hierarchy interface {
	get(ctx context.Context, id string) (*inventory.Entity, error)
}

// txHierarchy reads through the write transaction, so a guard sees the tree
// the write will commit to (ADR 0035, #301).
type txHierarchy struct {
	a  *App
	tx store.Tx
}

func (h txHierarchy) get(ctx context.Context, id string) (*inventory.Entity, error) {
	e, err := h.a.store.GetEntityTx(ctx, h.tx, id)
	if err != nil {
		return nil, wrapEntityError(id, err)
	}
	return e, nil
}

// storeHierarchy reads committed state outside any transaction, for read-only
// answers that need only a few lookups. The store opens transactions with
// the write lock, so a form render must not take one just to read.
type storeHierarchy struct {
	a *App
}

func (h storeHierarchy) get(ctx context.Context, id string) (*inventory.Entity, error) {
	e, err := h.a.store.GetEntityAnyStatus(ctx, id)
	if err != nil {
		return nil, wrapEntityError(id, err)
	}
	return e, nil
}

// snapshotHierarchy reads one listing, for pickers that test many destinations.
type snapshotHierarchy map[string]*inventory.Entity

func newSnapshotHierarchy(all []*inventory.Entity) snapshotHierarchy {
	h := make(snapshotHierarchy, len(all))
	for _, e := range all {
		h[e.EntityID] = e
	}
	return h
}

func (h snapshotHierarchy) get(_ context.Context, id string) (*inventory.Entity, error) {
	e, ok := h[id]
	if !ok {
		return nil, fmt.Errorf("get entity %q: %w", id, ErrNotFound)
	}
	return e, nil
}

// DestinationRejection reports why a destination is not a legal destination
// (CONTEXT.md, ADR 0041). Kind is ErrNotFound, ErrDiscrete, ErrInsideItself,
// ErrHomeCycle, ErrDestinationIsHome or ErrNoChange. It carries no
// interface-specific prose: web maps Kind to its own copy.
type DestinationRejection struct { //nolint:errname // named by ADR 0041
	Verb    string // move, use, return, add or borrow
	Kind    error
	Path    string // the entity being placed: FullPathDisplay, or a new entity's name
	Dest    string // destination FullPathDisplay, or its ID when it does not exist
	Blocker string // ErrHomeCycle: the displaced entity whose home would end up inside it
	Rehome  bool   // the destination is a displaced entity's current parent
}

// Unwrap returns Kind and ErrInvalidDestination, so callers can match either.
func (r *DestinationRejection) Unwrap() []error {
	if r == nil {
		return nil
	}
	return []error{r.Kind, ErrInvalidDestination}
}

// Error renders the CLI sentence for r.
func (r *DestinationRejection) Error() string {
	if r == nil {
		return nilRejectionText
	}
	if r.Rehome {
		return fmt.Sprintf("cannot %s %q here: its current location cannot be a home", r.Verb, r.Path)
	}
	head := fmt.Sprintf("cannot %s %q", r.Verb, r.Path)
	switch {
	case errors.Is(r.Kind, ErrHomeCycle):
		return fmt.Sprintf("%s there: %q's home would end up inside it — return %q first: %s",
			head, r.Blocker, r.Blocker, ErrInvalidDestination)
	case errors.Is(r.Kind, ErrInsideItself):
		return fmt.Sprintf("%s into itself or its own contents: %s", head, ErrInvalidDestination)
	case errors.Is(r.Kind, ErrNoChange):
		return fmt.Sprintf("%s: already there: %s", head, ErrInvalidDestination)
	case errors.Is(r.Kind, ErrDestinationIsHome):
		prep := "to"
		if r.Verb == verbUse {
			prep = "at"
		}
		return fmt.Sprintf("%s %s its home — use 'return' instead: %s: %s",
			head, prep, ErrDestinationIsHome, ErrInvalidDestination)
	case errors.Is(r.Kind, ErrDiscrete):
		return fmt.Sprintf("%s: %q is discrete and cannot hold entities: %s", head, r.Dest, ErrInvalidDestination)
	case errors.Is(r.Kind, ErrNotFound):
		return fmt.Sprintf("%s: destination %q %s", head, r.Dest, ErrNotFound)
	}
	return fmt.Sprintf("%s: %s", head, ErrInvalidDestination)
}

// ineligibleKind is the Kind for a destination that fails canAddChild: a
// removed entity reads as not found, as it does everywhere else.
func ineligibleKind(e *inventory.Entity) error {
	if e.Status == inventory.EntityStatusRemoved {
		return ErrNotFound
	}
	return ErrDiscrete
}

// checkNewParent loads destID as the parent of a new entity named path (add,
// borrow) and rejects it unless it is an eligible parent.
func checkNewParent(ctx context.Context, h hierarchy, verb, path, destID string) (*inventory.Entity, error) {
	dest, err := h.get(ctx, destID)
	if errors.Is(err, ErrNotFound) {
		return nil, &DestinationRejection{Verb: verb, Kind: ErrNotFound, Path: path, Dest: destID}
	}
	if err != nil {
		return nil, err
	}
	if !canAddChild(dest.Status, dest.Discrete) {
		return nil, &DestinationRejection{
			Verb: verb, Kind: ineligibleKind(dest), Path: path, Dest: dest.FullPathDisplay,
		}
	}
	return dest, nil
}

// checkRelocation rejects destID ("" = root) unless it is a legal destination
// for relocating moving by verb (verbMove, verbUse or verbReturn). It is the
// only place the destination rules for move, use and return are encoded
// (CONTEXT.md "Move", "Use", "Legal destination"; ADR 0039, ADR 0041).
func checkRelocation(ctx context.Context, h hierarchy, moving *inventory.Entity, destID, verb string) error {
	reject := func(kind error, dest string) error {
		return &DestinationRejection{Verb: verb, Kind: kind, Path: moving.FullPathDisplay, Dest: dest}
	}
	parentID, homeID := idOf(moving.ParentID), idOf(moving.HomeID)
	if verb != verbReturn {
		switch {
		case verb == verbMove && isRehome(moving, destID):
			return checkRehome(ctx, h, moving, parentID)
		case destID == homeID && (verb == verbUse || moving.IsDisplaced()):
			return reject(ErrDestinationIsHome, "")
		case destID == parentID:
			return reject(ErrNoChange, "")
		}
	}
	if destID == "" {
		return nil
	}
	dest, err := h.get(ctx, destID)
	if errors.Is(err, ErrNotFound) {
		return reject(ErrNotFound, destID)
	}
	if err != nil {
		return err
	}
	if !canAddChild(dest.Status, dest.Discrete) {
		return reject(ineligibleKind(dest), dest.FullPathDisplay)
	}
	inside, err := isSelfOrDescendant(ctx, h, moving.EntityID, destID)
	if err != nil {
		return err
	}
	if inside {
		return reject(ErrInsideItself, dest.FullPathDisplay)
	}
	return checkHomeCycle(ctx, h, moving, dest, verb)
}

// isRehome reports whether moving destID re-homes a displaced entity in place:
// the destination is the parent it already has (CONTEXT.md "Move").
func isRehome(moving *inventory.Entity, destID string) bool {
	return moving.IsDisplaced() && destID == idOf(moving.ParentID)
}

// checkRehome accepts a displaced entity's current parent as its new home
// (CONTEXT.md "Move") when that parent is an eligible parent or the root. The
// entity does not move, so the structural and home-cycle rules do not apply.
func checkRehome(ctx context.Context, h hierarchy, moving *inventory.Entity, parentID string) error {
	if parentID == "" {
		return nil
	}
	parent, err := h.get(ctx, parentID)
	kind, dest := ErrNotFound, parentID // a missing parent is named by ID
	switch {
	case err != nil && !errors.Is(err, ErrNotFound):
		return err
	case err == nil && canAddChild(parent.Status, parent.Discrete):
		return nil
	case err == nil:
		kind, dest = ineligibleKind(parent), parent.FullPathDisplay
	}
	return &DestinationRejection{Verb: verbMove, Kind: kind, Path: moving.FullPathDisplay, Dest: dest, Rehome: true}
}

// isSelfOrDescendant reports whether candidateID is entityID or lies beneath it.
// It walks the candidate's ancestors by ID — names are not unique, so path
// prefixes cannot be trusted. A projection already damaged by the
// pre-ADR-0034 cycle bug is reported as an error instead of looping forever.
func isSelfOrDescendant(ctx context.Context, h hierarchy, entityID, candidateID string) (bool, error) {
	seen := make(map[string]bool)
	for id := candidateID; id != ""; {
		if id == entityID {
			return true, nil
		}
		if seen[id] {
			return false, fmt.Errorf("ancestor chain of %q loops at %q", candidateID, id)
		}
		seen[id] = true
		e, err := h.get(ctx, id)
		if err != nil {
			return false, err
		}
		id = idOf(e.ParentID)
	}
	return false, nil
}

// checkHomeCycle rejects placing moving under dest when some displaced
// ancestor-or-self of dest has its home inside moving's subtree: the move
// would put that entity's home inside the entity itself (CONTEXT.md "Home").
// It covers indirect cases, e.g. moving Garage (which holds Shelf, the
// drill's home) into the drill.
func checkHomeCycle(ctx context.Context, h hierarchy, moving, dest *inventory.Entity, verb string) error {
	seen := make(map[string]bool)
	for e := dest; e != nil; {
		if seen[e.EntityID] {
			return fmt.Errorf("ancestor chain of %q loops at %q", dest.EntityID, e.EntityID)
		}
		seen[e.EntityID] = true
		if e.IsDisplaced() && e.HomeID != nil {
			inside, err := isSelfOrDescendant(ctx, h, moving.EntityID, *e.HomeID)
			if err != nil {
				return err
			}
			if inside {
				return &DestinationRejection{
					Verb: verb, Kind: ErrHomeCycle, Path: moving.FullPathDisplay,
					Dest: dest.FullPathDisplay, Blocker: e.FullPathDisplay,
				}
			}
		}
		if e.ParentID == nil {
			return nil
		}
		next, err := h.get(ctx, *e.ParentID)
		if err != nil {
			return err
		}
		e = next
	}
	return nil
}
