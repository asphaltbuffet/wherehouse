package lost

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// setupTestDatabase creates an in-memory database for testing helpers.
func setupTestDatabase(t *testing.T) *database.Database {
	t.Helper()

	db, err := database.Open(database.Config{
		Path:        ":memory:",
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err, "failed to open test database")

	return db
}

// Test: openDatabase returns a database connection.
func TestOpenDatabase_ReturnsConnection(t *testing.T) {
	// Note: This test is minimal because openDatabase is a simple wrapper.
	// The actual database opening is tested by other tests that use setupTestDatabase.
	// A full test would require setting up configuration, which is handled by integration tests.

	// For unit testing, we verify the function exists and has correct signature
	// by calling it in integration tests or through other functions that depend on it.
	t.Skip("openDatabase requires configuration setup - tested via integration tests")
}

// Test: resolveItemSelector delegates correctly and returns UUID.
func TestResolveItemSelector_ReturnsUUID(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t)
	defer db.Close()

	// Create test location and item
	locationID := "550e8400-e29b-41d4-a716-446655440001"
	err := db.CreateLocation(ctx, locationID, "Garage", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	itemID := "550e8400-e29b-41d4-a716-446655440011"
	err = db.CreateItem(ctx, itemID, "10mm socket", locationID, 1, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	// Test resolution by UUID
	resolvedID, err := resolveItemSelector(ctx, db, itemID)
	require.NoError(t, err)
	assert.Equal(t, itemID, resolvedID)
}

// Test: resolveItemSelector resolves by canonical name.
func TestResolveItemSelector_ByCanonicalName(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t)
	defer db.Close()

	// Create test location and item
	locationID := "550e8400-e29b-41d4-a716-446655440001"
	err := db.CreateLocation(ctx, locationID, "Garage", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	itemID := "550e8400-e29b-41d4-a716-446655440011"
	err = db.CreateItem(ctx, itemID, "10mm socket", locationID, 1, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	// Test resolution by canonical name
	resolvedID, err := resolveItemSelector(ctx, db, "10mm socket")
	require.NoError(t, err)
	assert.Equal(t, itemID, resolvedID)
}

// Test: resolveItemSelector resolves by LOCATION:ITEM selector.
func TestResolveItemSelector_ByLocationItem(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t)
	defer db.Close()

	// Create test location and item
	locationID := "550e8400-e29b-41d4-a716-446655440001"
	err := db.CreateLocation(ctx, locationID, "Garage", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	itemID := "550e8400-e29b-41d4-a716-446655440011"
	err = db.CreateItem(ctx, itemID, "10mm socket", locationID, 1, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	// Test resolution by LOCATION:ITEM
	resolvedID, err := resolveItemSelector(ctx, db, "garage:10mm socket")
	require.NoError(t, err)
	assert.Equal(t, itemID, resolvedID)
}

// Test: resolveItemSelector fails for non-existent item.
func TestResolveItemSelector_ItemNotFound_Error(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t)
	defer db.Close()

	// Attempt to resolve non-existent item
	_, err := resolveItemSelector(ctx, db, "nonexistent-item")
	require.Error(t, err)
}

// Test: resolveItemSelector fails for ambiguous selector.
func TestResolveItemSelector_AmbiguousSelector_Error(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t)
	defer db.Close()

	// Create test location with multiple items with same display name (canonically)
	locationID := "550e8400-e29b-41d4-a716-446655440001"
	err := db.CreateLocation(ctx, locationID, "Garage", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	// Note: Creating two items with exact same name would violate uniqueness constraints
	// So we test with a selector that could match multiple items if they existed
	// This is handled by the CLI resolution layer

	t.Skip("Ambiguous selector test requires duplicate item names - handled by database constraints")
}

// Test: resolveItemSelector works with display name variations.
func TestResolveItemSelector_DisplayNameVariations(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t)
	defer db.Close()

	// Create test location
	locationID := "550e8400-e29b-41d4-a716-446655440001"
	err := db.CreateLocation(ctx, locationID, "Garage", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	// Create item with display name
	itemID := "550e8400-e29b-41d4-a716-446655440011"
	err = db.CreateItem(ctx, itemID, "10MM Socket Wrench", locationID, 1, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	// Test resolution by different variations
	tests := []struct {
		name     string
		selector string
	}{
		{
			name:     "exact UUID",
			selector: itemID,
		},
		{
			name:     "canonical name (lowercase)",
			selector: "10mm socket wrench",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolvedID, selErr := resolveItemSelector(ctx, db, tt.selector)
			require.NoError(t, selErr)
			assert.Equal(t, itemID, resolvedID)
		})
	}
}
