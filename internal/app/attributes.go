package app

import (
	"context"
	"encoding/json"
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

// attrChange is one validated attribute event, ready to dispatch.
type attrChange struct {
	eventType inventory.EventType
	payload   []byte
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
	var changes []attrChange
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

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	for _, c := range changes {
		if _, err = a.bus.DispatchInTx(ctx, tx, c.eventType, req.ActorID, c.payload, note); err != nil {
			return "", fmt.Errorf("set attributes %q: %w", entity.FullPathDisplay, err)
		}
	}

	return entity.EntityID, nil
}

func lockedChange(e *inventory.Entity, f EntityFacts, locked bool) (attrChange, error) {
	if !locked {
		if err := f.Require(ActionUnlock); err != nil {
			return attrChange{}, err
		}
		return newAttrChange(inventory.EntityUnlockedEvent, eventbus.EntityUnlockedPayload{EntityID: e.EntityID})
	}
	if err := f.Require(ActionLock); err != nil {
		return attrChange{}, err
	}
	return newAttrChange(inventory.EntityLockedEvent, eventbus.EntityLockedPayload{EntityID: e.EntityID})
}

func discreteChange(e *inventory.Entity, f EntityFacts, discrete bool) (attrChange, error) {
	if !discrete {
		if err := f.Require(ActionClearDiscrete); err != nil {
			return attrChange{}, err
		}
		return newAttrChange(
			inventory.EntityDiscreteClearedEvent,
			eventbus.EntityDiscreteClearedPayload{EntityID: e.EntityID},
		)
	}
	if err := f.Require(ActionSetDiscrete); err != nil {
		return attrChange{}, err
	}
	return newAttrChange(inventory.EntityDiscreteSetEvent, eventbus.EntityDiscreteSetPayload{EntityID: e.EntityID})
}

// newAttrChange marshals payload (one of the four {entity_id} attribute payloads).
func newAttrChange(t inventory.EventType, payload any) (attrChange, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return attrChange{}, fmt.Errorf("marshal %s payload: %w", t, err)
	}
	return attrChange{eventType: t, payload: raw}, nil
}
