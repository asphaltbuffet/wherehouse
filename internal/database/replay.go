package database

import (
	"context"
	"database/sql"
	"fmt"
)

// RebuildProjections drops and rebuilds all projection tables from the event log.
// This is an atomic operation - either all projections are rebuilt successfully or none are.
// The rebuild happens within a single transaction to ensure consistency.
func (d *Database) RebuildProjections(ctx context.Context) error {
	return d.ExecInTransaction(ctx, func(tx *sql.Tx) error {
		// Clear all projection tables
		if _, err := tx.ExecContext(ctx, "DELETE FROM items_current"); err != nil {
			return fmt.Errorf("failed to clear items projection: %w", err)
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM projects_current"); err != nil {
			return fmt.Errorf("failed to clear projects projection: %w", err)
		}

		// Delete non-system locations
		if _, err := tx.ExecContext(ctx, "DELETE FROM locations_current WHERE is_system = 0"); err != nil {
			return fmt.Errorf("failed to clear locations projection: %w", err)
		}

		// Get all events in order
		events, err := d.GetAllEvents(ctx)
		if err != nil {
			return fmt.Errorf("failed to get events for replay: %w", err)
		}

		// Replay each event
		for _, event := range events {
			if err = d.processEventInTx(ctx, tx, event); err != nil {
				return fmt.Errorf("failed to process event %d (%s): %w", event.EventID, event.EventType, err)
			}
		}

		return nil
	})
}

// ReplayEventsFrom replays events starting from a specific event_id.
// This is used for incremental projection updates after a known checkpoint.
// The replay happens within a transaction to ensure consistency.
func (d *Database) ReplayEventsFrom(ctx context.Context, fromEventID int64) error {
	return d.ExecInTransaction(ctx, func(tx *sql.Tx) error {
		// Get events after the specified ID
		events, err := d.GetEventsAfter(ctx, fromEventID)
		if err != nil {
			return fmt.Errorf("failed to get events after %d: %w", fromEventID, err)
		}

		// Replay each event
		for _, event := range events {
			if err = d.processEventInTx(ctx, tx, event); err != nil {
				return fmt.Errorf("failed to process event %d (%s): %w", event.EventID, event.EventType, err)
			}
		}

		return nil
	})
}
