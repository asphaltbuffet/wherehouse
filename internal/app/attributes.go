package app

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// SetAttributes changes the locked and/or discrete attributes of all specified
// entities in a single atomic transaction (ADR 0033). Each change is validated
// against the entity's state before the call; setting the value an entity
// already holds fails with ErrNoChange rather than writing a no-op event
// (ADR 0024). Preconditions are defined in CONTEXT.md (locked, discrete).
func (a *App) SetAttributes(ctx context.Context, reqs []SetAttributesRequest) ([]EntityResult, error) {
	ids := make([]string, len(reqs))
	for i, req := range reqs {
		ids[i] = req.EntityID
	}
	if err := a.rejectDuplicateIDs(ctx, ids); err != nil {
		return nil, err
	}
	return execBatch(ctx, a, "SetAttributes", reqs, a.setAttributesInTx)
}

func (a *App) setAttributesInTx(ctx context.Context, tx store.Tx, req SetAttributesRequest) (string, error) {
	if req.Locked == nil && req.Discrete == nil {
		return "", fmt.Errorf("set attributes %q: %w", req.EntityID, ErrNoAttributes)
	}

	entity, facts, err := a.loadFactsTx(ctx, tx, req.EntityID)
	if err != nil {
		return "", err
	}

	// Validate every change before dispatching any, so each check sees the
	// pre-call state. Locked is emitted before discrete.
	var changes []eventbus.Payload
	if req.Locked != nil {
		c, cerr := lockedChange(entity, facts, *req.Locked)
		if cerr != nil {
			return "", cerr
		}
		changes = append(changes, c)
	}
	if req.Discrete != nil {
		c, cerr := discreteChange(entity, facts, *req.Discrete)
		if cerr != nil {
			return "", cerr
		}
		changes = append(changes, c)
	}

	for _, c := range changes {
		if _, err = a.bus.Dispatch(ctx, tx, req.ActorID, c, req.Note); err != nil {
			return "", fmt.Errorf("set attributes %q: %w", entity.FullPathDisplay, err)
		}
	}

	return entity.EntityID, nil
}

// lockedChange returns the validated lock or unlock event for e.
func lockedChange(e *inventory.Entity, f EntityFacts, locked bool) (eventbus.Payload, error) {
	if !locked {
		if err := f.Require(ActionUnlock); err != nil {
			return nil, err
		}
		return eventbus.EntityUnlockedPayload{EntityID: e.EntityID}, nil
	}
	if err := f.Require(ActionLock); err != nil {
		return nil, err
	}
	return eventbus.EntityLockedPayload{EntityID: e.EntityID}, nil
}

// discreteChange returns the validated discrete set or clear event for e.
func discreteChange(e *inventory.Entity, f EntityFacts, discrete bool) (eventbus.Payload, error) {
	if !discrete {
		if err := f.Require(ActionClearDiscrete); err != nil {
			return nil, err
		}
		return eventbus.EntityDiscreteClearedPayload{EntityID: e.EntityID}, nil
	}
	if err := f.Require(ActionSetDiscrete); err != nil {
		return nil, err
	}
	return eventbus.EntityDiscreteSetPayload{EntityID: e.EntityID}, nil
}
