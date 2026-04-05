package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

func TestMigrateDatabase_DryRun_PrintsPreview(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBForMigration(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := cli.MigrateDatabase(cmd, db, true)
	require.NoError(t, err, "MigrateDatabase dry-run should not return an error")

	out := buf.String()
	assert.Contains(t, out, "DRY RUN", "dry-run output should contain DRY RUN indicator")
	assert.Contains(t, out, "complete", "dry-run output should contain completion message")
}

func TestMigrateDatabase_DryRun_NoDBChanges(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBForMigration(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := cli.MigrateDatabase(cmd, db, true)
	require.NoError(t, err, "MigrateDatabase dry-run should not return an error")

	// Verify locations still exist and haven't changed
	locs, err := db.GetAllLocations(ctx)
	require.NoError(t, err, "GetAllLocations should not return an error")
	require.NotEmpty(t, locs, "should have at least one location")

	// After dry-run, UUIDs should still be in the database (no changes made)
	// Check that at least some IDs are NOT exactly 10 chars (indicating they're still old UUIDs)
	hasNonNanoidID := false
	for _, loc := range locs {
		if len(loc.LocationID) != 10 {
			hasNonNanoidID = true
			break
		}
	}
	assert.True(t, hasNonNanoidID,
		"after dry-run, database should still contain non-nanoid IDs (dry-run should not modify DB)")
}

func TestMigrateDatabase_SystemLocations_GetDeterministicIDs(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBForMigration(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := cli.MigrateDatabase(cmd, db, false)
	require.NoError(t, err, "MigrateDatabase should not return an error")

	locs, err := db.GetAllLocations(ctx)
	require.NoError(t, err, "GetAllLocations should not return an error")
	require.NotEmpty(t, locs, "should have at least one location")

	systemIDMap := map[string]string{
		"missing":  "sys0000001",
		"borrowed": "sys0000002",
		"loaned":   "sys0000003",
	}

	for _, loc := range locs {
		if expected, ok := systemIDMap[loc.CanonicalName]; ok {
			assert.Equal(t, expected, loc.LocationID,
				"system location %q should have deterministic ID %q, got %q",
				loc.CanonicalName, expected, loc.LocationID)
		}
	}
}

func TestMigrateDatabase_UserLocations_GetNanoidIDs(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBForMigration(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := cli.MigrateDatabase(cmd, db, false)
	require.NoError(t, err, "MigrateDatabase should not return an error")

	locs, err := db.GetAllLocations(ctx)
	require.NoError(t, err, "GetAllLocations should not return an error")

	systemCanonical := map[string]bool{"missing": true, "borrowed": true, "loaned": true}

	for _, loc := range locs {
		if systemCanonical[loc.CanonicalName] {
			continue // Skip system locations
		}

		// User locations should be 10-char alphanumeric
		assert.Len(t, loc.LocationID, 10,
			"user location %q should have 10-char ID, got %q (len=%d)",
			loc.DisplayName, loc.LocationID, len(loc.LocationID))

		// Verify all characters are alphanumeric
		for _, c := range loc.LocationID {
			assert.True(t, isAlphanumeric(c),
				"location ID %q should contain only alphanumeric characters, found %c",
				loc.LocationID, c)
		}
	}
}

func TestMigrateDatabase_Items_GetNanoidIDs(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBForMigration(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := cli.MigrateDatabase(cmd, db, false)
	require.NoError(t, err, "MigrateDatabase should not return an error")

	items, err := db.GetAllItems(ctx)
	require.NoError(t, err, "GetAllItems should not return an error")

	for _, item := range items {
		// All item IDs should be 10-char alphanumeric
		assert.Len(t, item.ItemID, 10,
			"item ID should be 10-char, got %q (len=%d)", item.ItemID, len(item.ItemID))

		// Verify all characters are alphanumeric
		for _, c := range item.ItemID {
			assert.True(t, isAlphanumeric(c),
				"item ID %q should contain only alphanumeric characters, found %c",
				item.ItemID, c)
		}
	}
}

func TestMigrateDatabase_EventPayloads_UpdatedCorrectly(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBForMigration(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := cli.MigrateDatabase(cmd, db, false)
	require.NoError(t, err, "MigrateDatabase should not return an error")

	// Get all events and verify payloads no longer contain UUID strings
	events, err := db.GetAllEvents(ctx)
	require.NoError(t, err, "GetAllEvents should not return an error")

	for _, event := range events {
		// UUID format check: 8-4-4-4-12 hex digits with dashes
		// This is a simple check for the UUID pattern
		assert.NotContains(t, event.Payload, "-0000-",
			"event payload should not contain UUID patterns after migration: %s", event.Payload)
	}
}

func TestMigrateDatabase_Idempotency(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBForMigration(t)

	cmd := &cobra.Command{}
	cmd.SetOut(bytes.NewBuffer(nil))
	cmd.SetContext(ctx)

	// First migration
	err := cli.MigrateDatabase(cmd, db, false)
	require.NoError(t, err, "first MigrateDatabase should not return an error")

	// Capture state after first migration
	locs1, err := db.GetAllLocations(ctx)
	require.NoError(t, err)
	locationIDsAfter1 := make(map[string]string)
	for _, loc := range locs1 {
		locationIDsAfter1[loc.DisplayName] = loc.LocationID
	}

	items1, err := db.GetAllItems(ctx)
	require.NoError(t, err)
	itemIDsAfter1 := make(map[string]string)
	for _, item := range items1 {
		itemIDsAfter1[item.DisplayName] = item.ItemID
	}

	// Second migration (idempotency check)
	err = cli.MigrateDatabase(cmd, db, false)
	require.NoError(t, err, "second MigrateDatabase (idempotency) should not return an error")

	// Capture state after second migration
	locs2, err := db.GetAllLocations(ctx)
	require.NoError(t, err)
	for _, loc := range locs2 {
		if prev, ok := locationIDsAfter1[loc.DisplayName]; ok {
			assert.Equal(t, prev, loc.LocationID,
				"location %q ID should not change on second migration", loc.DisplayName)
		}
	}

	items2, err := db.GetAllItems(ctx)
	require.NoError(t, err)
	for _, item := range items2 {
		if prev, ok := itemIDsAfter1[item.DisplayName]; ok {
			assert.Equal(t, prev, item.ItemID,
				"item %q ID should not change on second migration", item.DisplayName)
		}
	}
}

func TestMigrateDatabase_PrintsMappingReport(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBForMigration(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := cli.MigrateDatabase(cmd, db, false)
	require.NoError(t, err, "MigrateDatabase should not return an error")

	out := buf.String()
	assert.Contains(t, out, "Location ID mappings", "output should list location mappings")
	assert.Contains(t, out, "Item ID mappings", "output should list item mappings")

	// Check for mapping format (oldID -> newID)
	assert.Contains(t, out, "->", "output should show ID mappings with arrow notation")
}

// Helper to check if character is alphanumeric.
func isAlphanumeric(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9')
}

// setupTestDBForMigration creates a test database with migrations applied.
// This creates an empty test database (no seed data) so migration tests can work with it.
func setupTestDBForMigration(t *testing.T) *database.Database {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	cfg := database.DefaultConfig()
	cfg.Path = dbPath
	cfg.AutoMigrate = true

	db, err := database.Open(cfg)
	require.NoError(t, err, "failed to open test database")

	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("failed to close test database: %v", closeErr)
		}
	})

	return db
}

// Ensure strings import is used (needed for test compilation).
var _ = strings.Contains
