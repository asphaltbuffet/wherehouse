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

// rehomeTx makes a displaced entity's current parent its home (CONTEXT.md "Move"). checkRelocation has already accepted that parent.
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
func (a *App) UseEntity(ctx context.Context, req UseEntityRequest) (RelocationResult, error) {
	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	var from string
	if err := a.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		entity, facts, err := a.loadFactsTx(ctx, tx, req.EntityID)
		if err != nil {
			return err
		}
		from = idOf(entity.ParentID)
		if err = facts.Require(ActionUse); err != nil {
			return err
		}
		if err = checkRelocation(ctx, txHierarchy{a: a, tx: tx}, entity, req.NewParentID, verbUse); err != nil {
			return err
		}
		var newParentID *string
		if req.NewParentID != "" {
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
		return RelocationResult{}, err
	}
	return a.relocationResult(ctx, req.EntityID, from)
}

// returnHomeTx moves a displaced entity back to its home. The home is re-checked
// as a destination; the home invariants make that pass in practice, but a
// failed check rolls back the whole return.
func (a *App) returnHomeTx(
	ctx context.Context, tx store.Tx, entity *inventory.Entity, actorID string, note *string,
) error {
	if err := checkRelocation(ctx, txHierarchy{a: a, tx: tx}, entity, idOf(entity.HomeID), verbReturn); err != nil {
		return err
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
