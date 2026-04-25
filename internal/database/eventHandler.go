package database

import (
	"context"
	"database/sql"
	"fmt"
)

// ProcessEvent applies a single event to the projections.
// This is the primary entry point for event replay and is wrapped in a transaction.
func (d *Database) ProcessEvent(ctx context.Context, event *Event) error {
	return d.ExecInTransaction(ctx, func(tx *sql.Tx) error {
		return d.processEventInTx(ctx, tx, event)
	})
}

// processEventInTx applies a single event to projections within an existing transaction.
// This is the core event handler that routes to specific handlers based on event type.
func (d *Database) processEventInTx(ctx context.Context, tx *sql.Tx, event *Event) error {
	switch event.EventType {
	case EntityCreatedEvent:
		return d.handleEntityCreated(ctx, tx, event)
	case EntityRenamedEvent:
		return d.handleEntityRenamed(ctx, tx, event)
	case EntityReparentedEvent:
		return d.handleEntityReparented(ctx, tx, event)
	case EntityPathChangedEvent:
		return d.handleEntityPathChanged(ctx, tx, event)
	case EntityStatusChangedEvent:
		return d.handleEntityStatusChanged(ctx, tx, event)
	case EntityRemovedEvent:
		return d.handleEntityRemoved(ctx, tx, event)
	default:
		return fmt.Errorf("unknown event type: %s", event.EventType)
	}
}
