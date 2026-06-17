package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// ChangeStatus records a status-change event for the entity at the given path.
func (a *App) ChangeStatus(ctx context.Context, req ChangeStatusRequest) (EntityResult, error) {
	entity, err := a.store.GetEntity(ctx, req.EntityID)
	if err != nil {
		return EntityResult{}, wrapEntityError(req.EntityID, err)
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
	return execBatch(ctx, a, caller, reqs, inTx)
}

// execBatch runs inTx for every request inside a single transaction, then fetches the
// resulting entities by ID. The post-commit fetch uses AnyStatus because a transition
// may move an entity to removed (e.g. returning a borrowed entity). On any per-request
// error the whole transaction rolls back and no entities are changed.
func execBatch[T any](
	ctx context.Context,
	a *App,
	caller string,
	reqs []T,
	inTx func(context.Context, store.Tx, T) (string, error),
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
		result, err := a.GetEntityByIDAnyStatus(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get updated entity: %w", err)
		}
		results[i] = result
	}

	return results, nil
}

func (a *App) markLostInTx(ctx context.Context, tx store.Tx, req ChangeStatusRequest) (string, error) {
	entity, err := a.store.GetEntity(ctx, req.EntityID)
	if err != nil {
		return "", wrapEntityError(req.EntityID, err)
	}

	if entity.Locked {
		return "", fmt.Errorf("cannot mark %q as missing: entity is locked", entity.FullPathDisplay)
	}
	if entity.Status != inventory.EntityStatusOk {
		return "", fmt.Errorf(
			"cannot mark %q as missing: entity is %s (only ok entities can be marked missing)",
			entity.FullPathDisplay, entity.Status,
		)
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
		return "", fmt.Errorf("mark lost %q: %w", req.EntityID, err)
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

// MarkReturned sets the status of all specified entities to ok in a single atomic transaction.
func (a *App) MarkReturned(ctx context.Context, reqs []ChangeStatusRequest) ([]EntityResult, error) {
	return a.markStatusBatch(ctx, "MarkReturned", reqs, a.markReturnInTx)
}

func (a *App) markReturnInTx(ctx context.Context, tx store.Tx, req ChangeStatusRequest) (string, error) {
	entity, err := a.store.GetEntity(ctx, req.EntityID)
	if err != nil {
		return "", wrapEntityError(req.EntityID, err)
	}

	if entity.Status != inventory.EntityStatusLoaned && entity.Status != inventory.EntityStatusBorrowed {
		return "", fmt.Errorf(
			"cannot return %q: entity is %s (only loaned or borrowed entities can be returned)",
			entity.FullPathDisplay, entity.Status,
		)
	}

	// Borrowed entities are removed from inventory when returned, not reset to ok.
	targetStatus := inventory.EntityStatusOk
	if entity.Status == inventory.EntityStatusBorrowed {
		targetStatus = inventory.EntityStatusRemoved
	}

	payload := eventbus.EntityStatusChangedPayload{
		EntityID: entity.EntityID,
		Status:   targetStatus.String(),
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
		return "", fmt.Errorf("mark returned %q: %w", req.EntityID, err)
	}

	return entity.EntityID, nil
}

func (a *App) markLoanInTx(ctx context.Context, tx store.Tx, req ChangeStatusRequest) (string, error) {
	entity, err := a.store.GetEntity(ctx, req.EntityID)
	if err != nil {
		return "", wrapEntityError(req.EntityID, err)
	}

	if entity.Locked {
		return "", fmt.Errorf("cannot loan %q: entity is locked", entity.FullPathDisplay)
	}
	if entity.Status != inventory.EntityStatusOk && entity.Status != inventory.EntityStatusMissing {
		return "", fmt.Errorf(
			"cannot loan %q: entity is %s (only ok or missing entities can be loaned)",
			entity.FullPathDisplay, entity.Status,
		)
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
		return "", fmt.Errorf("mark loaned %q: %w", req.EntityID, err)
	}

	return entity.EntityID, nil
}

// BorrowEntities creates new entities in borrowed status in a single atomic transaction.
func (a *App) BorrowEntities(ctx context.Context, reqs []BorrowEntityRequest) ([]EntityResult, error) {
	return execBatch(ctx, a, "BorrowEntities", reqs, a.borrowEntityInTx)
}

func (a *App) borrowEntityInTx(ctx context.Context, tx store.Tx, req BorrowEntityRequest) (string, error) {
	var parentID *string
	if req.ParentPath != "" {
		parent, err := a.resolveEntityPathTx(ctx, tx, req.ParentPath)
		if err != nil {
			return "", fmt.Errorf("resolve parent path %q: %w", req.ParentPath, err)
		}
		if parent.Discrete {
			return "", fmt.Errorf("cannot add child to %q: entity is discrete", parent.FullPathDisplay)
		}
		parentID = &parent.EntityID
	}

	entityID, err := nanoid.New()
	if err != nil {
		return "", fmt.Errorf("generate entity ID: %w", err)
	}

	var statusContext *string
	if req.StatusContext != "" {
		statusContext = &req.StatusContext
	}

	payload := eventbus.EntityBorrowedPayload{
		EntityID:      entityID,
		DisplayName:   req.DisplayName,
		ParentID:      parentID,
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

	if _, err = a.bus.DispatchInTx(ctx, tx, inventory.EntityBorrowedEvent, req.ActorID, raw, note); err != nil {
		return "", fmt.Errorf("borrow entity %q: %w", req.DisplayName, err)
	}

	return entityID, nil
}

func (a *App) markFoundInTx(ctx context.Context, tx store.Tx, req ChangeStatusRequest) (string, error) {
	entity, err := a.store.GetEntity(ctx, req.EntityID)
	if err != nil {
		return "", wrapEntityError(req.EntityID, err)
	}

	if entity.Status != inventory.EntityStatusMissing {
		return "", fmt.Errorf(
			"cannot mark %q as found: entity is %s (only missing entities can be found; "+
				"use 'return' for loaned or borrowed)",
			entity.FullPathDisplay, entity.Status,
		)
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
		return "", fmt.Errorf("mark found %q: %w", req.EntityID, err)
	}

	return entity.EntityID, nil
}
