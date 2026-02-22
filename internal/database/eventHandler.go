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
	case "location.created":
		return d.handleLocationCreated(ctx, tx, event)
	case "location.renamed":
		return d.handleLocationRenamed(ctx, tx, event)
	case "location.reparented":
		return d.handleLocationReparented(ctx, tx, event)
	case "location.deleted":
		return d.handleLocationDeleted(ctx, tx, event)

	// item event handling
	case "item.created":
		return d.handleItemCreated(ctx, tx, event)
	case "item.moved":
		return d.handleItemMoved(ctx, tx, event)
	case "item.missing":
		return d.handleItemMissing(ctx, tx, event)
	case "item.borrowed":
		return d.handleItemBorrowed(ctx, tx, event)
	case "item.found":
		return d.handleItemFound(ctx, tx, event)
	case "item.deleted":
		return d.handleItemDeleted(ctx, tx, event)

	// project event handling
	case "project.created":
		return d.handleProjectCreated(ctx, tx, event)
	case "project.completed":
		return d.handleProjectCompleted(ctx, tx, event)
	case "project.reopened":
		return d.handleProjectReopened(ctx, tx, event)
	case "project.deleted":
		return d.handleProjectDeleted(ctx, tx, event)

	default:
		return fmt.Errorf("unknown event type: %s", event.EventType)
	}
}
