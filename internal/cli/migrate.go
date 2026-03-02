package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// systemLocationIDs maps canonical system location names to their fixed deterministic IDs.
var systemLocationIDs = map[string]string{
	"missing":  "sys0000001",
	"borrowed": "sys0000002",
	"loaned":   "sys0000003",
}

// migrateMapping tracks old-to-new ID remappings for a migration run.
type migrateMapping struct {
	Locations map[string]string // old location_id -> new location_id
	Items     map[string]string // old item_id -> new item_id
}

// MigrateDatabase rewrites all entity IDs from UUID format to nanoid format.
// It uses cmd for output (cmd.OutOrStdout()) and respects the dryRun flag.
// All changes are applied in a single atomic transaction; on failure no changes persist.
// Idempotent: if an ID already looks like a nanoid (10-char alphanumeric), it is left unchanged.
func MigrateDatabase(cmd *cobra.Command, db *database.Database, dryRun bool) error {
	ctx := cmd.Context()
	w := cmd.OutOrStdout()

	if dryRun {
		fmt.Fprintln(w, "DRY RUN: No changes will be made to the database.")
		fmt.Fprintln(w)
	}

	// Build ID mapping
	mapping, err := buildMigrateMapping(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to build ID mapping: %w", err)
	}

	// Print mapping report
	printMigrateReport(w, mapping)

	if dryRun {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Dry run complete. Run without --dry-run to apply changes.")
		return nil
	}

	// Apply migration in a single atomic transaction
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Applying migration...")

	txErr := db.ExecInTransaction(ctx, func(tx *sql.Tx) error {
		return applyMigration(ctx, tx, mapping)
	})
	if txErr != nil {
		return fmt.Errorf("migration failed (no changes applied): %w", txErr)
	}

	fmt.Fprintln(w, "Migration complete.")
	return nil
}

// buildMigrateMapping queries the database and constructs old->new ID mappings.
// System locations get fixed IDs. User entities with already-valid nanoid IDs are mapped to themselves (idempotent).
func buildMigrateMapping(ctx context.Context, db *database.Database) (*migrateMapping, error) {
	mapping := &migrateMapping{
		Locations: make(map[string]string),
		Items:     make(map[string]string),
	}

	// Map location IDs
	locations, err := db.GetAllLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query locations: %w", err)
	}

	for _, loc := range locations {
		if fixedID, ok := systemLocationIDs[loc.CanonicalName]; ok {
			// System locations always get their deterministic fixed ID
			mapping.Locations[loc.LocationID] = fixedID
		} else if LooksLikeID(loc.LocationID) {
			// Already migrated — map to itself (idempotent, no DB update needed)
			mapping.Locations[loc.LocationID] = loc.LocationID
		} else {
			// User location — generate a new nanoid
			newID, genErr := nanoid.New()
			if genErr != nil {
				return nil, fmt.Errorf("failed to generate ID for location %q: %w", loc.DisplayName, genErr)
			}
			mapping.Locations[loc.LocationID] = newID
		}
	}

	// Map item IDs
	items, err := db.GetAllItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query items: %w", err)
	}

	for _, item := range items {
		if LooksLikeID(item.ItemID) {
			// Already migrated — map to itself (idempotent)
			mapping.Items[item.ItemID] = item.ItemID
		} else {
			newID, genErr := nanoid.New()
			if genErr != nil {
				return nil, fmt.Errorf("failed to generate ID for item %q: %w", item.DisplayName, genErr)
			}
			mapping.Items[item.ItemID] = newID
		}
	}

	return mapping, nil
}

// applyMigration applies the full ID rewrite within a transaction.
func applyMigration(ctx context.Context, tx *sql.Tx, mapping *migrateMapping) error {
	if err := migrateLocationRows(ctx, tx, mapping.Locations); err != nil {
		return err
	}
	if err := migrateItemRows(ctx, tx, mapping); err != nil {
		return err
	}
	if err := migrateEventIndexedColumns(ctx, tx, mapping); err != nil {
		return err
	}
	return migrateEventPayloads(ctx, tx, mapping)
}

// migrateLocationRows rewrites location_id and parent_id in locations_current.
func migrateLocationRows(ctx context.Context, tx *sql.Tx, locations map[string]string) error {
	for oldID, newID := range locations {
		if oldID == newID {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE locations_current SET location_id = ? WHERE location_id = ?`,
			newID, oldID); err != nil {
			return fmt.Errorf("failed to update location_id %q: %w", oldID, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE locations_current SET parent_id = ? WHERE parent_id = ?`,
			newID, oldID); err != nil {
			return fmt.Errorf("failed to update parent_id reference %q: %w", oldID, err)
		}
	}
	return nil
}

// migrateItemRows rewrites item_id and location FK columns in items_current.
func migrateItemRows(ctx context.Context, tx *sql.Tx, mapping *migrateMapping) error {
	for oldID, newID := range mapping.Items {
		if oldID == newID {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE items_current SET item_id = ? WHERE item_id = ?`,
			newID, oldID); err != nil {
			return fmt.Errorf("failed to update item_id %q: %w", oldID, err)
		}
	}
	for oldLocID, newLocID := range mapping.Locations {
		if oldLocID == newLocID {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE items_current SET location_id = ? WHERE location_id = ?`,
			newLocID, oldLocID); err != nil {
			return fmt.Errorf("failed to update items_current.location_id for %q: %w", oldLocID, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE items_current SET temp_origin_location_id = ? WHERE temp_origin_location_id = ?`,
			newLocID, oldLocID); err != nil {
			return fmt.Errorf("failed to update items_current.temp_origin_location_id for %q: %w", oldLocID, err)
		}
	}
	return nil
}

// migrateEventIndexedColumns rewrites item_id and location_id index columns on events.
func migrateEventIndexedColumns(ctx context.Context, tx *sql.Tx, mapping *migrateMapping) error {
	for oldID, newID := range mapping.Items {
		if oldID == newID {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE events SET item_id = ? WHERE item_id = ?`,
			newID, oldID); err != nil {
			return fmt.Errorf("failed to update events.item_id %q: %w", oldID, err)
		}
	}
	for oldID, newID := range mapping.Locations {
		if oldID == newID {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE events SET location_id = ? WHERE location_id = ?`,
			newID, oldID); err != nil {
			return fmt.Errorf("failed to update events.location_id %q: %w", oldID, err)
		}
	}
	return nil
}

// migrateEventPayloads collects changed ID pairs and rewrites event payload JSON blobs.
func migrateEventPayloads(ctx context.Context, tx *sql.Tx, mapping *migrateMapping) error {
	allMappings := make(map[string]string, len(mapping.Locations)+len(mapping.Items))
	for k, v := range mapping.Locations {
		if k != v {
			allMappings[k] = v
		}
	}
	for k, v := range mapping.Items {
		if k != v {
			allMappings[k] = v
		}
	}
	if len(allMappings) == 0 {
		return nil
	}
	return rewriteEventPayloads(ctx, tx, allMappings)
}

// rewriteEventPayloads applies string substitution to all event payload JSON blobs.
// Safety: [strings.ReplaceAll] is safe here because source IDs are UUIDs (36 chars with dashes)
// and target IDs are nanoids (10 alphanumeric chars). A UUID cannot be a substring of a nanoid
// or appear accidentally in other JSON fields, so replacement cannot produce false positives.
func rewriteEventPayloads(ctx context.Context, tx *sql.Tx, mapping map[string]string) error {
	rows, err := tx.QueryContext(ctx, `SELECT event_id, payload FROM events`)
	if err != nil {
		return fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	type eventRow struct {
		id      int64
		payload string
	}

	var evts []eventRow
	for rows.Next() {
		var e eventRow
		if scanErr := rows.Scan(&e.id, &e.payload); scanErr != nil {
			return fmt.Errorf("failed to scan event row: %w", scanErr)
		}
		evts = append(evts, e)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return fmt.Errorf("error iterating event rows: %w", rowsErr)
	}

	stmt, prepErr := tx.PrepareContext(ctx, `UPDATE events SET payload = ? WHERE event_id = ?`)
	if prepErr != nil {
		return fmt.Errorf("failed to prepare update statement: %w", prepErr)
	}
	defer stmt.Close()

	for _, e := range evts {
		updated := e.payload
		for oldID, newID := range mapping {
			updated = strings.ReplaceAll(updated, oldID, newID)
		}
		if updated != e.payload {
			if _, execErr := stmt.ExecContext(ctx, updated, e.id); execErr != nil {
				return fmt.Errorf("failed to update event %d payload: %w", e.id, execErr)
			}
		}
	}

	return nil
}

// printMigrateReport writes the ID mapping summary to the given writer.
func printMigrateReport(w io.Writer, mapping *migrateMapping) {
	fmt.Fprintf(w, "Location ID mappings (%d):\n", len(mapping.Locations))
	for oldID, newID := range mapping.Locations {
		fmt.Fprintf(w, "  %s -> %s\n", oldID, newID)
	}
	fmt.Fprintf(w, "\nItem ID mappings (%d):\n", len(mapping.Items))
	for oldID, newID := range mapping.Items {
		fmt.Fprintf(w, "  %s -> %s\n", oldID, newID)
	}
}
