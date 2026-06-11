package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// ChangeStatus records a status-change event for the entity at the given path.
func (a *App) ChangeStatus(ctx context.Context, req ChangeStatusRequest) (EntityResult, error) {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return EntityResult{}, fmt.Errorf("resolve path %q: %w", req.EntityPath, err)
	}

	if entity.Locked && req.Status == inventory.EntityStatusMissing {
		return EntityResult{}, fmt.Errorf("cannot mark %q as missing: entity is locked", entity.FullPathDisplay)
	}

	var statusContext *string
	if req.StatusContext != "" {
		statusContext = &req.StatusContext
	}

	payload := eventbus.EntityStatusChangedPayload{
		EntityID:      entity.EntityID,
		Status:        req.Status.String(),
		StatusContext: statusContext,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return EntityResult{}, fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err = a.bus.Dispatch(ctx, inventory.EntityStatusChangedEvent, req.ActorID, raw, note); err != nil {
		return EntityResult{}, fmt.Errorf("change status: %w", err)
	}

	result, err := a.GetEntityByID(ctx, entity.EntityID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get updated entity: %w", err)
	}

	return result, nil
}

// MarkLost sets the status of all specified entities to missing in a single atomic transaction.
func (a *App) MarkLost(ctx context.Context, reqs []ChangeStatusRequest) ([]EntityResult, error) {
	return a.markStatusBatch(ctx, "MarkLost", reqs, a.markLostInTx)
}

func (a *App) markStatusBatch(
	ctx context.Context,
	caller string,
	reqs []ChangeStatusRequest,
	inTx func(context.Context, store.Tx, ChangeStatusRequest) (string, error),
) ([]EntityResult, error) {
	if len(reqs) == 0 {
		return nil, fmt.Errorf("%s: at least one request required", caller)
	}

	entityIDs := make([]string, len(reqs))

	if err := a.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		for i, req := range reqs {
			id, err := inTx(ctx, tx, req)
			if err != nil {
				return err
			}
			entityIDs[i] = id
		}
		return nil
	}); err != nil {
		return nil, err
	}

	results := make([]EntityResult, len(entityIDs))
	for i, id := range entityIDs {
		result, err := a.GetEntityByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get updated entity: %w", err)
		}
		results[i] = result
	}

	return results, nil
}

func (a *App) markLostInTx(ctx context.Context, tx store.Tx, req ChangeStatusRequest) (string, error) {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", req.EntityPath, err)
	}

	if entity.Locked {
		return "", fmt.Errorf("cannot mark %q as missing: entity is locked", entity.FullPathDisplay)
	}

	payload := eventbus.EntityStatusChangedPayload{
		EntityID: entity.EntityID,
		Status:   inventory.EntityStatusMissing.String(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err = a.bus.DispatchInTx(
		ctx, tx, inventory.EntityStatusChangedEvent, req.ActorID, raw, note,
	); err != nil {
		return "", fmt.Errorf("mark lost %q: %w", req.EntityPath, err)
	}

	return entity.EntityID, nil
}

// MarkFound sets the status of all specified entities to ok in a single atomic transaction.
func (a *App) MarkFound(ctx context.Context, reqs []ChangeStatusRequest) ([]EntityResult, error) {
	return a.markStatusBatch(ctx, "MarkFound", reqs, a.markFoundInTx)
}

// MarkLoaned sets the status of all specified entities to loaned in a single atomic transaction.
func (a *App) MarkLoaned(ctx context.Context, reqs []ChangeStatusRequest) ([]EntityResult, error) {
	return a.markStatusBatch(ctx, "MarkLoaned", reqs, a.markLoanInTx)
}

func (a *App) markLoanInTx(ctx context.Context, tx store.Tx, req ChangeStatusRequest) (string, error) {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", req.EntityPath, err)
	}

	if entity.Locked {
		return "", fmt.Errorf("cannot loan %q: entity is locked", entity.FullPathDisplay)
	}

	var statusContext *string
	if req.StatusContext != "" {
		statusContext = &req.StatusContext
	}

	payload := eventbus.EntityStatusChangedPayload{
		EntityID:      entity.EntityID,
		Status:        inventory.EntityStatusLoaned.String(),
		StatusContext: statusContext,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err = a.bus.DispatchInTx(
		ctx, tx, inventory.EntityStatusChangedEvent, req.ActorID, raw, note,
	); err != nil {
		return "", fmt.Errorf("mark loaned %q: %w", req.EntityPath, err)
	}

	return entity.EntityID, nil
}

func (a *App) markFoundInTx(ctx context.Context, tx store.Tx, req ChangeStatusRequest) (string, error) {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", req.EntityPath, err)
	}

	payload := eventbus.EntityStatusChangedPayload{
		EntityID: entity.EntityID,
		Status:   inventory.EntityStatusOk.String(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err = a.bus.DispatchInTx(
		ctx, tx, inventory.EntityStatusChangedEvent, req.ActorID, raw, note,
	); err != nil {
		return "", fmt.Errorf("mark found %q: %w", req.EntityPath, err)
	}

	return entity.EntityID, nil
}
