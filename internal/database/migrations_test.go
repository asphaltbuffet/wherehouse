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

		// Verify all tables exist after migration 6 (entity consolidation)
		tables := []string{
			"events",
			"entities_current",
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

		// Verify old tables were removed by migration 6
		oldTables := []string{"items_current", "locations_current"}
		for _, table := range oldTables {
			var count int
			require.NoError(t, db.db.QueryRow(
				"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
			).Scan(&count))
			assert.Zero(t, count, "table %s should not exist after migration 6", table)
		}
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
		assert.EqualValues(t, 6, version, "should be at version 6 after all migrations")
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
	})

	t.Run("dirty state detection", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		ctx := t.Context()

		// Manually set dirty state at current version
		require.NoError(t, db.SetMigrationVersion(ctx, 6, true))

		// Verify dirty state is detected
		version, dirty, err := db.GetMigrationVersion()
		require.NoError(t, err)
		assert.EqualValues(t, 6, version)
		assert.True(t, dirty, "dirty flag should be set")
	})
}

func TestMigrationRollback(t *testing.T) {
	t.Run("down migration removes all tables", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		// Verify entities_current exists before rollback
		var tableCount int
		require.NoError(t, db.db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name IN (
				'events', 'entities_current', 'schema_metadata'
			)
		`).Scan(&tableCount))
		assert.Equal(t, 3, tableCount, "core tables should exist before rollback")

		// Run rollback six times (we have 6 migrations now)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 6 (entity consolidation)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 5 (remove project tables)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 4 (Removed location)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 3 (nanoid marker)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 2 (Loaned location)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 1 (initial schema)

		// Verify tables removed
		require.NoError(t, db.db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name IN (
				'events', 'entities_current', 'locations_current', 'items_current',
				'projects_current', 'schema_metadata'
			)
		`).Scan(&tableCount))
		assert.Zero(t, tableCount, "application tables should be removed after rollback")
	})

	t.Run("up after down restores schema", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		// Rollback six times (we have 6 migrations now)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 6 (entity consolidation)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 5 (remove project tables)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 4 (Removed location)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 3 (nanoid marker)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 2 (Loaned location)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 1 (initial schema)

		// Verify tables removed
		var tableCount int
		require.NoError(t, db.db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name IN ('events', 'entities_current', 'schema_metadata')
		`).Scan(&tableCount))
		assert.Zero(t, tableCount)

		// Re-run migrations
		require.NoError(t, db.RunMigrations())

		// Verify schema restored
		tables := []string{"events", "entities_current", "schema_metadata"}
		for _, table := range tables {
			var name string
			require.NoError(t, db.db.QueryRow(
				"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
				table,
			).Scan(&name),
				"table %s should exist", table)
		}
	})
}

func TestMigrationWithData(t *testing.T) {
	t.Run("schema constraints enforced", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		ctx := t.Context()

		// Test foreign key constraint (child entity requires valid parent)
		_, err := db.db.ExecContext(ctx, `
			INSERT INTO entities_current (
				entity_id, display_name, canonical_name, entity_type,
				parent_id, full_path_display, full_path_canonical,
				depth, status, last_event_id, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, 1, 'ok', 1, ?)
		`, "child-entity", "Child", "child", "leaf", "non-existent-parent",
			"Parent >> Child", "parent:child", "2026-02-21T10:00:00Z")
		require.Error(t, err, "should fail due to foreign key constraint")

		// Test CHECK constraint (entity_type must be valid)
		_, err = db.db.ExecContext(ctx, `
			INSERT INTO entities_current (
				entity_id, display_name, canonical_name, entity_type,
				parent_id, full_path_display, full_path_canonical,
				depth, status, last_event_id, updated_at
			) VALUES (?, ?, ?, ?, NULL, ?, ?, 0, 'ok', 1, ?)
		`, "bad-entity", "Bad", "bad", "invalid_type",
			"Bad", "bad", "2026-02-21T10:00:00Z")
		require.Error(t, err, "should fail due to CHECK constraint on entity_type")

		// Test CHECK constraint (status must be valid)
		_, err = db.db.ExecContext(ctx, `
			INSERT INTO entities_current (
				entity_id, display_name, canonical_name, entity_type,
				parent_id, full_path_display, full_path_canonical,
				depth, status, last_event_id, updated_at
			) VALUES (?, ?, ?, ?, NULL, ?, ?, 0, 'invalid_status', 1, ?)
		`, "bad-status-entity", "Bad Status", "bad_status", "leaf",
			"Bad Status", "bad_status", "2026-02-21T10:00:00Z")
		require.Error(t, err, "should fail due to CHECK constraint on status")
	})

	t.Run("indexes created", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()

		// Verify key indexes exist
		indexes := []string{
			"idx_events_type",
			"idx_events_timestamp",
			"idx_events_entity_id",
			"idx_entities_canonical_name",
			"idx_entities_parent_id",
			"idx_entities_status",
			"idx_entities_entity_type",
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

		// Create a test entity in transaction
		require.NoError(t, db.ExecInTransaction(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO entities_current (
					entity_id, display_name, canonical_name, entity_type,
					parent_id, full_path_display, full_path_canonical,
					depth, status, last_event_id, updated_at
				) VALUES (?, ?, ?, ?, NULL, ?, ?, 0, 'ok', 1, ?)
			`, "tx-test-entity", "TX Test", "tx_test", "leaf",
				"TX Test", "tx_test", "2026-02-21T10:00:00Z")
			return err
		}))

		// Verify data was committed
		var count int
		require.NoError(t, db.db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM entities_current WHERE entity_id = 'tx-test-entity'",
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
				INSERT INTO entities_current (
					entity_id, display_name, canonical_name, entity_type,
					parent_id, full_path_display, full_path_canonical,
					depth, status, last_event_id, updated_at
				) VALUES (?, ?, ?, ?, NULL, ?, ?, 0, 'ok', 1, ?)
			`, "rollback-test", "Rollback Test", "rollback_test", "leaf",
				"Rollback Test", "rollback_test", "2026-02-21T10:00:00Z")
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
			"SELECT COUNT(*) FROM entities_current WHERE entity_id = 'rollback-test'",
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
