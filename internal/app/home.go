package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// pathsByID maps entity ID to FullPathDisplay. List-style callers already hold
// every live entity, and homes are always live (CONTEXT.md "Home"), so homes
// resolve with no extra query (ADR 0015).
func pathsByID(entities []*inventory.Entity) map[string]string {
	m := make(map[string]string, len(entities))
	for _, e := range entities {
		m[e.EntityID] = e.FullPathDisplay
	}
	return m
}

// fillHomePath sets HomePathDisplay on a displaced result with a non-root home.
func fillHomePath(r *EntityResult, paths map[string]string) {
	if r.IsDisplaced() && r.HomeID != "" {
		r.HomePathDisplay = paths[r.HomeID]
	}
}

// fillHomePaths applies fillHomePath to every result.
func fillHomePaths(results []EntityResult, paths map[string]string) {
	for i := range results {
		fillHomePath(&results[i], paths)
	}
}

// withHomePath fills HomePathDisplay for a single result (point lookups).
func (a *App) withHomePath(ctx context.Context, r EntityResult) (EntityResult, error) {
	if !r.IsDisplaced() || r.HomeID == "" {
		return r, nil
	}
	home, err := a.store.GetEntityAnyStatus(ctx, r.HomeID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get home of %q: %w", r.EntityID, err)
	}
	r.HomePathDisplay = home.FullPathDisplay
	return r, nil
}

// checkHomeCycleTx rejects placing moving under destID ("" = root) when some
// displaced ancestor-or-self of the destination has its home inside moving's
// subtree: the move would put that entity's home inside the entity itself
// (CONTEXT.md "Home"). It covers indirect cases, e.g. moving Garage (which
// holds Shelf, the drill's home) into the drill.
func (a *App) checkHomeCycleTx(ctx context.Context, tx store.Tx, moving *inventory.Entity, destID string) error {
	seen := make(map[string]bool)
	for id := destID; id != ""; {
		if seen[id] {
			return fmt.Errorf("ancestor chain of %q loops at %q", destID, id)
		}
		seen[id] = true
		e, err := a.store.GetEntityTx(ctx, tx, id)
		if err != nil {
			return wrapEntityError(id, err)
		}
		if e.IsDisplaced() && e.HomeID != nil {
			inside, ierr := a.isSelfOrDescendant(ctx, tx, moving.EntityID, *e.HomeID)
			if ierr != nil {
				return ierr
			}
			if inside {
				return fmt.Errorf("cannot move %q there: %q's home would end up inside it — return %q first: %w",
					moving.FullPathDisplay, e.FullPathDisplay, e.FullPathDisplay, ErrInvalidDestination)
			}
		}
		id = idOf(e.ParentID)
	}
	return nil
}

// rehomeTx makes a displaced entity's current parent its home (CONTEXT.md
// "Move": moving a displaced entity to where it already is).
func (a *App) rehomeTx(ctx context.Context, tx store.Tx, entity *inventory.Entity, actorID string, note *string) error {
	raw, err := json.Marshal(eventbus.EntityRehomedPayload{EntityID: entity.EntityID, HomeID: entity.ParentID})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	if _, err = a.bus.DispatchInTx(ctx, tx, inventory.EntityRehomedEvent, actorID, raw, note); err != nil {
		return fmt.Errorf("rehome entity: %w", err)
	}
	return nil
}

// UseEntity temporarily moves an entity: the parent changes and the home stays,
// leaving it displaced (CONTEXT.md "Use"). Preconditions are move's plus
// "destination is not home". All guards read through the write transaction (ADR 0035).
func (a *App) UseEntity(ctx context.Context, req UseEntityRequest) (EntityResult, error) {
	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if err := a.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		entity, err := a.store.GetLiveEntityTx(ctx, tx, req.EntityID)
		if err != nil {
			return wrapEntityError(req.EntityID, err)
		}
		if err = checkMovable(entity); err != nil {
			return err
		}
		if req.NewParentID == idOf(entity.HomeID) {
			return fmt.Errorf("cannot use %q at its home — use 'return' instead: %w: %w",
				entity.FullPathDisplay, ErrDestinationIsHome, ErrInvalidDestination)
		}
		if err = checkNotAlreadyThere(entity, req.NewParentID); err != nil {
			return err
		}

		var newParentID *string
		if req.NewParentID != "" {
			if err = a.checkMoveDestinationTx(ctx, tx, entity, req.NewParentID); err != nil {
				return err
			}
			newParentID = &req.NewParentID
		}

		raw, err := json.Marshal(eventbus.EntityUsedPayload{EntityID: entity.EntityID, NewParentID: newParentID})
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		if _, err = a.bus.DispatchInTx(ctx, tx, inventory.EntityUsedEvent, req.ActorID, raw, note); err != nil {
			return fmt.Errorf("use entity: %w", err)
		}
		return nil
	}); err != nil {
		return EntityResult{}, err
	}

	// AnyStatus: a concurrent remove may commit between our commit and this read.
	return a.GetEntityByIDAnyStatus(ctx, req.EntityID)
}

// returnHomeTx moves a displaced entity back to its home. The home is re-checked
// as a destination; the home invariants make that pass in practice, but a
// failed check rolls back the whole return.
func (a *App) returnHomeTx(
	ctx context.Context, tx store.Tx, entity *inventory.Entity, actorID string, note *string,
) error {
	if entity.HomeID != nil {
		if err := a.checkMoveDestinationTx(ctx, tx, entity, *entity.HomeID); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(eventbus.EntityReturnedHomePayload{EntityID: entity.EntityID, ParentID: entity.HomeID})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	if _, err = a.bus.DispatchInTx(ctx, tx, inventory.EntityReturnedHomeEvent, actorID, raw, note); err != nil {
		return fmt.Errorf("return %q home: %w", entity.FullPathDisplay, err)
	}
	return nil
}

// checkNotHomeTx rejects action on e while any non-removed displaced entity
// belongs in it (CONTEXT.md "Home"). action is the verb for the message
// ("remove", "mark discrete").
func (a *App) checkNotHomeTx(ctx context.Context, tx store.Tx, e *inventory.Entity, action string) error {
	n, err := a.store.CountDisplacedWithHomeTx(ctx, tx, e.EntityID)
	if err != nil {
		return fmt.Errorf("%s %q: %w", action, e.FullPathDisplay, err)
	}
	if n > 0 {
		return fmt.Errorf("cannot %s %q: it is home to %d displaced entities — return them first: %w",
			action, e.FullPathDisplay, n, ErrIsHome)
	}
	return nil
}
