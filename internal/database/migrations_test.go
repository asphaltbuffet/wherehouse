package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrations(t *testing.T) {
	t.Run("fresh database creates schema", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		// Verify all tables exist
		tables := []string{
			"events",
			"locations_current",
			"items_current",
			"projects_current",
			"schema_metadata",
			"schema_migrations",
		}
		for _, table := range tables {
			var name string
			require.NoError(
				t,
				db.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name),
				"table %s should exist",
				table,
			)
			assert.Equal(t, table, name)
		}

		// Verify system locations were seeded
		var count int
		require.NoError(t, db.db.QueryRow("SELECT COUNT(*) FROM locations_current WHERE is_system = 1").Scan(&count))
		assert.Equal(t, 3, count, "should have 3 system locations (Missing, Borrowed, and Loaned)")

		// Verify system locations have correct canonical names
		var missing, borrowed, loaned int
		require.NoError(
			t,
			db.db.QueryRow("SELECT COUNT(*) FROM locations_current WHERE canonical_name = 'missing' AND is_system = 1").
				Scan(&missing),
		)
		assert.Equal(t, 1, missing, "should have Missing location")

		require.NoError(
			t,
			db.db.QueryRow("SELECT COUNT(*) FROM locations_current WHERE canonical_name = 'borrowed' AND is_system = 1").
				Scan(&borrowed),
		)
		assert.Equal(t, 1, borrowed, "should have Borrowed location")

		require.NoError(
			t,
			db.db.QueryRow("SELECT COUNT(*) FROM locations_current WHERE canonical_name = 'loaned' AND is_system = 1").
				Scan(&loaned),
		)
		assert.Equal(t, 1, loaned, "should have Loaned location")
	})

	t.Run("version tracking", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		// Verify schema_migrations table was created
		var exists int
		require.NoError(
			t,
			db.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").
				Scan(&exists),
		)
		assert.Equal(t, 1, exists)

		// Verify migration version
		version, dirty, err := db.GetMigrationVersion()
		require.NoError(t, err)
		assert.EqualValues(t, 2, version, "should be at version 2 after all migrations")
		assert.False(t, dirty, "migration should not be dirty")
	})

	t.Run("idempotent migrations", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		// Get initial version
		version1, dirty1, err := db.GetMigrationVersion()
		require.NoError(t, err)
		assert.False(t, dirty1)

		// Run migrations again (should be no-op)
		require.NoError(t, db.RunMigrations())

		// Verify version unchanged
		version2, dirty2, err := db.GetMigrationVersion()
		require.NoError(t, err)
		assert.Equal(t, version1, version2, "version should be unchanged")
		assert.False(t, dirty2)

		// Verify system locations weren't duplicated
		var count int
		require.NoError(t, db.db.QueryRow("SELECT COUNT(*) FROM locations_current WHERE is_system = 1").Scan(&count))
		assert.Equal(t, 3, count, "should still have only 3 system locations")
	})

	t.Run("dirty state detection", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		ctx := t.Context()

		// Manually set dirty state
		require.NoError(t, db.SetMigrationVersion(ctx, 2, true))

		// Verify dirty state is detected
		version, dirty, err := db.GetMigrationVersion()
		require.NoError(t, err)
		assert.EqualValues(t, 2, version)
		assert.True(t, dirty, "dirty flag should be set")
	})
}

func TestMigrationRollback(t *testing.T) {
	t.Run("down migration removes all tables", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		// Verify tables exist before rollback
		var tableCount int
		require.NoError(t, db.db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name IN (
				'events', 'locations_current', 'items_current',
				'projects_current', 'schema_metadata'
			)
		`).Scan(&tableCount))
		assert.Equal(t, 5, tableCount, "all tables should exist before rollback")

		// Run rollback twice (we have 2 migrations now)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 2 (Loaned location)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 1 (initial schema)

		// Verify tables removed
		require.NoError(t, db.db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name IN (
				'events', 'locations_current', 'items_current',
				'projects_current', 'schema_metadata'
			)
		`).Scan(&tableCount))
		assert.Zero(t, tableCount, "application tables should be removed after rollback")
	})

	t.Run("up after down restores schema", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		// Rollback twice (we have 2 migrations now)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 2 (Loaned location)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 1 (initial schema)

		// Verify tables removed
		var tableCount int
		require.NoError(t, db.db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name IN ('events', 'locations_current', 'items_current', 'projects_current', 'schema_metadata')
		`).Scan(&tableCount))
		assert.Zero(t, tableCount)

		// Re-run migrations
		require.NoError(t, db.RunMigrations())

		// Re-seed system locations
		require.NoError(t, db.seedSystemLocations(t.Context()))

		// Verify schema restored
		tables := []string{"events", "locations_current", "items_current", "projects_current", "schema_metadata"}
		for _, table := range tables {
			var name string
			require.NoError(t, db.db.QueryRow(
				"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
				table,
			).Scan(&name),
				"table %s should exist", table)
		}

		// Verify system locations re-seeded
		var count int
		require.NoError(t, db.db.QueryRow("SELECT COUNT(*) FROM locations_current WHERE is_system = 1").Scan(&count))
		assert.Equal(t, 3, count, "system locations should be re-seeded")
	})
}

func TestMigrationWithData(t *testing.T) {
	t.Run("schema constraints enforced", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		ctx := t.Context()

		// Test foreign key constraint (item requires valid location)
		_, err := db.db.ExecContext(ctx, `
			INSERT INTO items_current (
				item_id, display_name, canonical_name, location_id,
				in_temporary_use, last_event_id, updated_at
			) VALUES (?, ?, ?, ?, 0, 1, ?)
		`, "test-item", "Test Item", "test_item", "non-existent-location", "2026-02-21T10:00:00Z")
		require.Error(t, err, "should fail due to foreign key constraint")

		// Test CHECK constraint (status must be 'active' or 'completed')
		_, err = db.db.ExecContext(ctx, `
			INSERT INTO projects_current (project_id, status, updated_at)
			VALUES (?, ?, ?)
		`, "test-project", "invalid-status", "2026-02-21T10:00:00Z")
		require.Error(t, err, "should fail due to CHECK constraint")

		// Test CHECK constraint (in_temporary_use must be 0 or 1)
		// First create a valid location
		_, err = db.db.ExecContext(ctx, `
			INSERT INTO locations_current (
				location_id, display_name, canonical_name,
				parent_id, full_path_display, full_path_canonical,
				depth, is_system, updated_at
			) VALUES (?, ?, ?, NULL, ?, ?, 0, 0, ?)
		`, "test-loc", "Test Location", "test_location", "Test Location", "test_location", "2026-02-21T10:00:00Z")
		require.NoError(t, err)

		// Now test invalid in_temporary_use value
		_, err = db.db.ExecContext(ctx, `
			INSERT INTO items_current (
				item_id, display_name, canonical_name, location_id,
				in_temporary_use, last_event_id, updated_at
			) VALUES (?, ?, ?, ?, 2, 1, ?)
		`, "test-item", "Test Item", "test_item", "test-loc", "2026-02-21T10:00:00Z")
		require.Error(t, err, "should fail due to CHECK constraint on in_temporary_use")
	})

	t.Run("indexes created", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		// Verify key indexes exist
		indexes := []string{
			"idx_events_type",
			"idx_events_timestamp",
			"idx_items_canonical_name",
			"idx_items_location_id",
			"idx_locations_canonical_parent",
			"idx_projects_status",
		}

		for _, index := range indexes {
			var name string
			err := db.db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", index).Scan(&name)
			require.NoError(t, err, "index %s should exist", index)
			assert.Equal(t, index, name)
		}
	})
}

func TestDatabaseConfig(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		cfg := DefaultConfig()
		assert.Equal(t, 30000, cfg.BusyTimeout)
		assert.True(t, cfg.AutoMigrate)
	})

	t.Run("requires database path", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Path = ""
		_, err := Open(cfg)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrDatabasePathRequired)
	})

	t.Run("pragmas configured", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		// Verify WAL mode
		var journalMode string
		require.NoError(t, db.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode))
		assert.Equal(t, "wal", journalMode)

		// Verify foreign keys enabled
		var foreignKeys int
		require.NoError(t, db.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys))
		assert.Equal(t, 1, foreignKeys, "foreign keys should be enabled")
	})
}

func TestTransactionHelpers(t *testing.T) {
	t.Run("ExecInTransaction commits on success", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		ctx := t.Context()

		// Create a test location in transaction
		require.NoError(t, db.ExecInTransaction(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO locations_current (
					location_id, display_name, canonical_name,
					parent_id, full_path_display, full_path_canonical,
					depth, is_system, updated_at
				) VALUES (?, ?, ?, NULL, ?, ?, 0, 0, ?)
			`, "tx-test-loc", "TX Test", "tx_test", "TX Test", "tx_test", "2026-02-21T10:00:00Z")
			return err
		}))

		// Verify data was committed
		var count int
		require.NoError(t, db.db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM locations_current WHERE location_id = 'tx-test-loc'",
		).Scan(&count))
		assert.Equal(t, 1, count, "transaction should have committed")
	})

	t.Run("ExecInTransaction rolls back on error", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		ctx := t.Context()

		// Transaction that fails
		require.Error(t, db.ExecInTransaction(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO locations_current (
					location_id, display_name, canonical_name,
					parent_id, full_path_display, full_path_canonical,
					depth, is_system, updated_at
				) VALUES (?, ?, ?, NULL, ?, ?, 0, 0, ?)
			`, "rollback-test", "Rollback Test", "rollback_test", "Rollback Test", "rollback_test", "2026-02-21T10:00:00Z")
			if err != nil {
				return err
			}

			// Force an error
			return assert.AnError
		}))

		// Verify data was rolled back
		var count int
		require.NoError(t, db.db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM locations_current WHERE location_id = 'rollback-test'",
		).Scan(&count))
		assert.Zero(t, count, "transaction should have rolled back")
	})
}

func TestSchemaMetadata(t *testing.T) {
	t.Run("get and set metadata", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		ctx := t.Context()

		// Set metadata
		require.NoError(t, db.SetMetadata(ctx, "test_key", "test_value"))

		// Get metadata
		value, err := db.GetMetadata(ctx, "test_key")
		require.NoError(t, err)
		assert.Equal(t, "test_value", value)
	})

	t.Run("metadata not found", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		ctx := t.Context()

		// Try to get non-existent key
		_, err := db.GetMetadata(ctx, "non_existent_key")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("initial metadata exists", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		ctx := t.Context()

		// Verify created_at exists
		createdAt, err := db.GetMetadata(ctx, "created_at")
		require.NoError(t, err)
		assert.NotEmpty(t, createdAt)

		// Verify app_version exists
		appVersion, err := db.GetMetadata(ctx, "app_version")
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", appVersion)
	})
}

// openTestDB creates a new test database with migrations applied.
func openTestDB(t *testing.T) *Database {
	t.Helper()

	// Create temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database with migrations
	cfg := DefaultConfig()
	cfg.Path = dbPath
	cfg.AutoMigrate = true

	db, err := Open(cfg)
	require.NoError(t, err, "failed to open test database")

	// Clean up on test completion
	t.Cleanup(func() {
		db.Close()
	})

	return db
}
