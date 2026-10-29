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

	entity, err := a.store.GetLiveEntityTx(ctx, tx, req.EntityID)
	if err != nil {
		return "", wrapEntityError(req.EntityID, err)
	}

	// Validate every change before dispatching any, so each check sees the
	// pre-call state. Locked is emitted before discrete.
	var changes []attrChange
	if req.Locked != nil {
		c, cerr := lockedChange(entity, *req.Locked)
		if cerr != nil {
			return "", cerr
		}
		changes = append(changes, c)
	}
	if req.Discrete != nil {
		c, cerr := a.discreteChange(ctx, tx, entity, *req.Discrete)
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

func lockedChange(e *inventory.Entity, locked bool) (attrChange, error) {
	switch {
	case locked && e.Locked:
		return attrChange{}, fmt.Errorf("cannot lock %q: already locked (%w)", e.FullPathDisplay, ErrNoChange)
	case !locked && !e.Locked:
		return attrChange{}, fmt.Errorf("cannot unlock %q: not locked (%w)", e.FullPathDisplay, ErrNoChange)
	case !locked:
		return newAttrChange(inventory.EntityUnlockedEvent, eventbus.EntityUnlockedPayload{EntityID: e.EntityID})
	case e.Status != inventory.EntityStatusOk && e.Status != inventory.EntityStatusBorrowed:
		return attrChange{}, fmt.Errorf(
			"cannot lock %q: entity is %s (only ok or borrowed entities can be locked)",
			e.FullPathDisplay, e.Status,
		)
	case e.IsDisplaced():
		return attrChange{}, rejection([]error{ErrDisplaced},
			"cannot lock %q: entity is displaced — return it first", e.FullPathDisplay)
	default:
		return newAttrChange(inventory.EntityLockedEvent, eventbus.EntityLockedPayload{EntityID: e.EntityID})
	}
}

func (a *App) discreteChange(
	ctx context.Context, tx store.Tx, e *inventory.Entity, discrete bool,
) (attrChange, error) {
	switch {
	case discrete && e.Discrete:
		return attrChange{}, fmt.Errorf(
			"cannot mark %q discrete: already discrete (%w)", e.FullPathDisplay, ErrNoChange,
		)
	case !discrete && !e.Discrete:
		return attrChange{}, fmt.Errorf(
			"cannot clear discrete on %q: not discrete (%w)", e.FullPathDisplay, ErrNoChange,
		)
	case !discrete:
		return newAttrChange(
			inventory.EntityDiscreteClearedEvent,
			eventbus.EntityDiscreteClearedPayload{EntityID: e.EntityID},
		)
	}

	n, err := a.store.CountLiveChildrenTx(ctx, tx, e.EntityID)
	if err != nil {
		return attrChange{}, fmt.Errorf("mark %q discrete: %w", e.FullPathDisplay, err)
	}
	if n > 0 {
		return attrChange{}, fmt.Errorf("cannot mark %q discrete (%d inside; move or remove them first): %w",
			e.FullPathDisplay, n, ErrHasChildren)
	}
	if err = a.checkNotHomeTx(ctx, tx, e, "mark discrete"); err != nil {
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
