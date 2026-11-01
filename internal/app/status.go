package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

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
	ids := make([]string, len(reqs))
	for i, req := range reqs {
		ids[i] = req.EntityID
	}
	if err := a.rejectDuplicateIDs(ctx, ids); err != nil {
		return nil, err
	}
	return execBatch(ctx, a, caller, reqs, inTx)
}

// rejectDuplicateIDs fails a batch that names the same entity twice. Without it
// the second request trips its own status guard ("entity is ok", or not found
// for a removed borrow), which hides the real mistake from the user.
func (a *App) rejectDuplicateIDs(ctx context.Context, ids []string) error {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			continue
		}
		name := id
		if e, err := a.store.GetEntityAnyStatus(ctx, id); err == nil {
			name = e.FullPathDisplay
		}
		return fmt.Errorf("%q: %w", name, ErrDuplicateEntity)
	}
	return nil
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
	entity, facts, err := a.loadFactsTx(ctx, tx, req.EntityID)
	if err != nil {
		return "", err
	}
	if err = facts.Require(ActionLost); err != nil {
		return "", err
	}

	return a.setStatusInTx(ctx, tx, entity, inventory.EntityStatusMissing, req)
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
	entity, facts, err := a.loadFactsTx(ctx, tx, req.EntityID)
	if err != nil {
		return "", err
	}
	if err = facts.Require(ActionReturn); err != nil {
		return "", err
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	switch entity.Status { //nolint:exhaustive // Require admitted only borrowed, loaned, or displaced ok
	case inventory.EntityStatusBorrowed:
		// Borrowed entities leave the inventory without a trip home (ADR 0039).
		if err = a.dispatchStatusTx(ctx, tx, entity, inventory.EntityStatusRemoved, req.ActorID, note); err != nil {
			return "", err
		}
		return entity.EntityID, nil
	case inventory.EntityStatusLoaned:
		if err = a.dispatchStatusTx(ctx, tx, entity, inventory.EntityStatusOk, req.ActorID, note); err != nil {
			return "", err
		}
		note = nil // the note rides on the first event of the return only
	}

	if entity.IsDisplaced() {
		if err = a.returnHomeTx(ctx, tx, entity, req.ActorID, note); err != nil {
			return "", err
		}
	}
	return entity.EntityID, nil
}

// dispatchStatusTx writes an EntityStatusChangedEvent with no status context.
func (a *App) dispatchStatusTx(
	ctx context.Context, tx store.Tx, entity *inventory.Entity,
	status inventory.EntityStatus, actorID string, note *string,
) error {
	raw, err := json.Marshal(eventbus.EntityStatusChangedPayload{EntityID: entity.EntityID, Status: status.String()})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	if _, err = a.bus.DispatchInTx(ctx, tx, inventory.EntityStatusChangedEvent, actorID, raw, note); err != nil {
		return fmt.Errorf("mark returned %q: %w", entity.EntityID, err)
	}
	return nil
}

func (a *App) markLoanInTx(ctx context.Context, tx store.Tx, req ChangeStatusRequest) (string, error) {
	recipient := strings.TrimSpace(req.StatusContext)
	if recipient == "" {
		return "", fmt.Errorf("loan %q: %w", req.EntityID, ErrRecipientRequired)
	}

	entity, facts, err := a.loadFactsTx(ctx, tx, req.EntityID)
	if err != nil {
		return "", err
	}
	if err = facts.Require(ActionLoan); err != nil {
		return "", err
	}

	payload := eventbus.EntityStatusChangedPayload{
		EntityID:      entity.EntityID,
		Status:        inventory.EntityStatusLoaned.String(),
		StatusContext: &recipient,
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
	if err := entitypath.ValidateSegment(req.DisplayName); err != nil {
		return "", fmt.Errorf("invalid name %q: %w", req.DisplayName, err)
	}
	lender := strings.TrimSpace(req.StatusContext)
	if lender == "" {
		return "", fmt.Errorf("borrow %q: %w", req.DisplayName, ErrLenderRequired)
	}

	var parentID *string
	if req.ParentID != "" {
		parent, err := a.getParentTx(ctx, tx, req.ParentID)
		if err != nil {
			return "", err
		}
		parentID = &parent.EntityID
	}

	entityID, err := nanoid.New()
	if err != nil {
		return "", fmt.Errorf("generate entity ID: %w", err)
	}

	payload := eventbus.EntityBorrowedPayload{
		EntityID:      entityID,
		DisplayName:   req.DisplayName,
		ParentID:      parentID,
		StatusContext: &lender,
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
	entity, facts, err := a.loadFactsTx(ctx, tx, req.EntityID)
	if err != nil {
		return "", err
	}
	if err = facts.Require(ActionFound); err != nil {
		return "", err
	}

	return a.setStatusInTx(ctx, tx, entity, inventory.EntityStatusOk, req)
}

// setStatusInTx dispatches a plain status change for an already-admitted entity.
func (a *App) setStatusInTx(
	ctx context.Context, tx store.Tx, entity *inventory.Entity,
	status inventory.EntityStatus, req ChangeStatusRequest,
) (string, error) {
	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	if err := a.dispatchStatusTx(ctx, tx, entity, status, req.ActorID, note); err != nil {
		return "", err
	}
	return entity.EntityID, nil
}
