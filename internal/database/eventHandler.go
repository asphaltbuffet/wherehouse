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
	// location event handling
	case LocationCreatedEvent:
		return d.handleLocationCreated(ctx, tx, event)
	case LocationRenamedEvent:
		return d.handleLocationRenamed(ctx, tx, event)
	case LocationMovedEvent:
		return d.handleLocationReparented(ctx, tx, event)
	case LocationDeletedEvent:
		return d.handleLocationDeleted(ctx, tx, event)

	// item event handling
	case ItemCreatedEvent:
		return d.handleItemCreated(ctx, tx, event)
	case ItemMovedEvent:
		return d.handleItemMoved(ctx, tx, event)
	case ItemMissingEvent:
		return d.handleItemMissing(ctx, tx, event)
	case ItemBorrowedEvent:
		return d.handleItemBorrowed(ctx, tx, event)
	case ItemLoanedEvent:
		return d.handleItemLoaned(ctx, tx, event)
	case ItemFoundEvent:
		return d.handleItemFound(ctx, tx, event)
	case ItemDeletedEvent:
		return d.handleItemDeleted(ctx, tx, event)

	// project event handling
	case ProjectCreatedEvent:
		return d.handleProjectCreated(ctx, tx, event)
	case ProjectCompletedEvent:
		return d.handleProjectCompleted(ctx, tx, event)
	case ProjectReopenedEvent:
		return d.handleProjectReopened(ctx, tx, event)
	case ProjectDeletedEvent:
		return d.handleProjectDeleted(ctx, tx, event)

	default:
		return fmt.Errorf("unknown event type: %s", event.EventType)
	}
}
