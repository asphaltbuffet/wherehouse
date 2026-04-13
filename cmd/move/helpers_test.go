package move

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

func TestLooksLikeID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid 10-char alphanumeric",
			input: "aB3xK9mPqR",
			want:  true,
		},
		{
			name:  "valid all digits",
			input: "0000000000",
			want:  true,
		},
		{
			name:  "valid all uppercase",
			input: "AAAAAAAAAA",
			want:  true,
		},
		{
			name:  "valid test format",
			input: "tst0loc001",
			want:  true,
		},
		{
			name:  "invalid UUID format",
			input: "550e8400-e29b-41d4-a716-446655440000",
			want:  false,
		},
		{
			name:  "too short",
			input: "socket",
			want:  false,
		},
		{
			name:  "too long",
			input: "aB3xK9mPqRx",
			want:  false,
		},
		{
			name:  "contains hyphen",
			input: "not-a-uuid-format-string",
			want:  false,
		},
		{
			name:  "contains underscore",
			input: "aB3xK9mP_R",
			want:  false,
		},
		{
			name:  "no dashes",
			input: "550e8400e29b41d4a716446655440000",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cli.LooksLikeID(tt.input)
			assert.Equal(t, tt.want, got, "LooksLikeID() mismatch")
		})
	}
}

func TestResolveLocation(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t)
	defer db.Close()

	// Create test locations with valid UUID
	garageID := "550e8400-e29b-41d4-a716-446655440001"
	toolboxID := "550e8400-e29b-41d4-a716-446655440002"

	err := db.CreateLocation(ctx, garageID, "Garage", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)
	err = db.CreateLocation(ctx, toolboxID, "Tool Box", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	tests := []struct {
		name      string
		input     string
		wantID    string
		wantError bool
	}{
		{
			name:   "resolve by UUID",
			input:  garageID,
			wantID: garageID,
		},
		{
			name:   "resolve by canonical name",
			input:  "garage",
			wantID: garageID,
		},
		{
			name:   "resolve by display name",
			input:  "Garage",
			wantID: garageID,
		},
		{
			name:   "resolve with spaces in name",
			input:  "Tool Box",
			wantID: toolboxID,
		},
		{
			name:      "location not found",
			input:     "nonexistent",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotErr := cli.ResolveLocation(ctx, db, tt.input)
			if tt.wantError {
				assert.Error(t, gotErr)
			} else {
				require.NoError(t, gotErr)
				assert.Equal(t, tt.wantID, gotID)
			}
		})
	}
}

func TestResolveItemSelector(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t)
	defer db.Close()

	// Create test location
	garageID := "550e8400-e29b-41d4-a716-446655440001"
	err := db.CreateLocation(ctx, garageID, "Garage", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	// Create test item
	itemID := "550e8400-e29b-41d4-a716-446655440011"
	err = db.CreateItem(ctx, itemID, "10mm socket", garageID, 1, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	tests := []struct {
		name      string
		selector  string
		wantID    string
		wantError bool
	}{
		{
			name:     "resolve by UUID",
			selector: itemID,
			wantID:   itemID,
		},
		{
			name:     "resolve by LOCATION:ITEM",
			selector: "garage:10mm socket",
			wantID:   itemID,
		},
		{
			name:     "resolve by canonical name",
			selector: "10mm socket",
			wantID:   itemID,
		},
		{
			name:      "invalid UUID",
			selector:  "550e8400-e29b-41d4-a716-000000000000",
			wantError: true,
		},
		{
			name:      "item not found",
			selector:  "nonexistent",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotErr := cli.ResolveItemSelector(ctx, db, tt.selector, "wherehouse move")
			if tt.wantError {
				assert.Error(t, gotErr)
			} else {
				require.NoError(t, gotErr)
				assert.Equal(t, tt.wantID, gotID)
			}
		})
	}
}

// setupTestDatabase creates an in-memory database for testing.
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

func TestIsQuietMode(t *testing.T) {
	// Note: These functions read from persistent flags, which are tested via integration tests.
	// Unit testing flag parsing without full cobra setup is challenging.
	// This test validates the function logic with mock flag values.

	// The actual flag parsing is covered by integration tests and GetMoveCmd_Structure test.
	// cli.IsQuietMode is now tested in internal/cli/flags_test.go
	t.Skip("Flag helper functions are covered by integration tests")
}
