package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// ChangeStatus records a status-change event for the entity at the given path.
func (a *App) ChangeStatus(ctx context.Context, req ChangeStatusRequest) error {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return fmt.Errorf("resolve path %q: %w", req.EntityPath, err)
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
		return fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err = a.bus.Dispatch(ctx, inventory.EntityStatusChangedEvent, req.ActorID, raw, note); err != nil {
		return fmt.Errorf("change status: %w", err)
	}

	return nil
}
