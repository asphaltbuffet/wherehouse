package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
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
