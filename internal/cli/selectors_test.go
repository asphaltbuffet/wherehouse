package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

func TestResolveLocation(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	// Create test locations via events
	garageID := uuid.New().String()
	basementID := uuid.New().String()

	_, err := db.AppendEvent(ctx, "location.created", "test-user", map[string]any{
		"location_id":  garageID,
		"display_name": "Garage",
		"parent_id":    nil,
	}, "")
	require.NoError(t, err)

	_, err = db.AppendEvent(ctx, "location.created", "test-user", map[string]any{
		"location_id":  basementID,
		"display_name": "Base Ment",
		"parent_id":    nil,
	}, "")
	require.NoError(t, err)

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid UUID exists in database",
			input:   garageID,
			want:    garageID,
			wantErr: false,
		},
		{
			name:    "valid UUID does not exist - falls through to canonical",
			input:   uuid.New().String(),
			want:    "",
			wantErr: true,
		},
		{
			name:    "display name resolution",
			input:   "Garage",
			want:    garageID,
			wantErr: false,
		},
		{
			name:    "canonical name resolution",
			input:   "garage",
			want:    garageID,
			wantErr: false,
		},
		{
			name:    "canonical name with spaces",
			input:   "Base Ment",
			want:    basementID,
			wantErr: false,
		},
		{
			name:    "location not found",
			input:   "Nonexistent",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := ResolveLocation(ctx, db, tt.input)
			if tt.wantErr {
				require.Error(t, gotErr)
				assert.Empty(t, got)
			} else {
				require.NoError(t, gotErr)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestLooksLikeUUID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid UUID v4",
			input: "550e8400-e29b-41d4-a716-446655440000",
			want:  true,
		},
		{
			name:  "valid UUID v7",
			input: uuid.New().String(),
			want:  true,
		},
		{
			name:  "too short",
			input: "550e8400-e29b-41d4-a716",
			want:  false,
		},
		{
			name:  "too long",
			input: "550e8400-e29b-41d4-a716-446655440000-extra",
			want:  false,
		},
		{
			name:  "invalid format",
			input: "not-a-uuid-at-all-really-not-one",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "canonical name",
			input: "garage",
			want:  false,
		},
		{
			name:  "display name with spaces",
			input: "My Garage",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LooksLikeUUID(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseItemSelector(t *testing.T) {
	tests := []struct {
		name         string
		selector     string
		wantLocation string
		wantItem     string
		wantOK       bool
	}{
		{
			name:         "valid LOCATION:ITEM",
			selector:     "Garage:Wrench",
			wantLocation: "Garage",
			wantItem:     "Wrench",
			wantOK:       true,
		},
		{
			name:         "valid with spaces",
			selector:     "  Garage  :  Wrench  ",
			wantLocation: "Garage",
			wantItem:     "Wrench",
			wantOK:       true,
		},
		{
			name:         "no colon - not a selector",
			selector:     "Wrench",
			wantLocation: "",
			wantItem:     "",
			wantOK:       false,
		},
		{
			name:         "multiple colons - takes first two parts",
			selector:     "Garage:Workbench:Wrench",
			wantLocation: "Garage",
			wantItem:     "Workbench:Wrench",
			wantOK:       true,
		},
		{
			name:         "empty parts",
			selector:     ":",
			wantLocation: "",
			wantItem:     "",
			wantOK:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLocation, gotItem, gotOK := parseItemSelector(tt.selector)
			assert.Equal(t, tt.wantOK, gotOK)
			assert.Equal(t, tt.wantLocation, gotLocation)
			assert.Equal(t, tt.wantItem, gotItem)
		})
	}
}

func TestResolveItemSelector(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	// Create test locations
	garageID := uuid.New().String()
	basementID := uuid.New().String()

	_, err := db.AppendEvent(ctx, "location.created", "test-user", map[string]any{
		"location_id":  garageID,
		"display_name": "Garage",
		"parent_id":    nil,
	}, "")
	require.NoError(t, err)

	_, err = db.AppendEvent(ctx, "location.created", "test-user", map[string]any{
		"location_id":  basementID,
		"display_name": "Basement",
		"parent_id":    nil,
	}, "")
	require.NoError(t, err)

	// Create test items
	wrench1ID := uuid.New().String()
	wrench2ID := uuid.New().String()
	hammerID := uuid.New().String()

	// Two wrenches in different locations (to test ambiguity)
	_, err = db.AppendEvent(ctx, "item.created", "test-user", map[string]any{
		"item_id":      wrench1ID,
		"display_name": "Wrench",
		"location_id":  garageID,
	}, "")
	require.NoError(t, err)

	_, err = db.AppendEvent(ctx, "item.created", "test-user", map[string]any{
		"item_id":      wrench2ID,
		"display_name": "Wrench",
		"location_id":  basementID,
	}, "")
	require.NoError(t, err)

	// One hammer (unique)
	_, err = db.AppendEvent(ctx, "item.created", "test-user", map[string]any{
		"item_id":      hammerID,
		"display_name": "Hammer",
		"location_id":  garageID,
	}, "")
	require.NoError(t, err)

	tests := []struct {
		name        string
		selector    string
		commandName string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid UUID exists",
			selector:    wrench1ID,
			commandName: "wherehouse move",
			want:        wrench1ID,
			wantErr:     false,
		},
		{
			name:        "valid UUID does not exist",
			selector:    uuid.New().String(),
			commandName: "wherehouse move",
			want:        "",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "unique item by canonical name",
			selector:    "Hammer",
			commandName: "wherehouse move",
			want:        hammerID,
			wantErr:     false,
		},
		{
			name:        "ambiguous item by canonical name",
			selector:    "Wrench",
			commandName: "wherehouse move",
			want:        "",
			wantErr:     true,
			errContains: "multiple items",
		},
		{
			name:        "ambiguous error includes command name",
			selector:    "Wrench",
			commandName: "wherehouse history",
			want:        "",
			wantErr:     true,
			errContains: "wherehouse history --id",
		},
		{
			name:        "LOCATION:ITEM selector - valid",
			selector:    "Garage:Wrench",
			commandName: "wherehouse move",
			want:        wrench1ID,
			wantErr:     false,
		},
		{
			name:        "LOCATION:ITEM selector - canonical names",
			selector:    "garage:wrench",
			commandName: "wherehouse move",
			want:        wrench1ID,
			wantErr:     false,
		},
		{
			name:        "LOCATION:ITEM selector - location not found",
			selector:    "Kitchen:Wrench",
			commandName: "wherehouse move",
			want:        "",
			wantErr:     true,
			errContains: "location \"Kitchen\" not found",
		},
		{
			name:        "LOCATION:ITEM selector - item not in location",
			selector:    "Basement:Hammer",
			commandName: "wherehouse move",
			want:        "",
			wantErr:     true,
			errContains: "item \"Hammer\" not found in location \"Basement\"",
		},
		{
			name:        "item not found",
			selector:    "Nonexistent",
			commandName: "wherehouse move",
			want:        "",
			wantErr:     true,
			errContains: "item \"Nonexistent\" not found",
		},
		{
			name:        "empty selector",
			selector:    "",
			commandName: "wherehouse move",
			want:        "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := ResolveItemSelector(ctx, db, tt.selector, tt.commandName)
			if tt.wantErr {
				require.Error(t, gotErr)
				assert.Empty(t, got)
				if tt.errContains != "" {
					assert.Contains(t, gotErr.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, gotErr)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestResolveLocationItemSelector(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	// Create test location
	garageID := uuid.New().String()
	_, err := db.AppendEvent(ctx, "location.created", "test-user", map[string]any{
		"location_id":  garageID,
		"display_name": "Garage",
		"parent_id":    nil,
	}, "")
	require.NoError(t, err)

	// Create test items
	wrenchID := uuid.New().String()
	_, err = db.AppendEvent(ctx, "item.created", "test-user", map[string]any{
		"item_id":      wrenchID,
		"display_name": "Wrench",
		"location_id":  garageID,
	}, "")
	require.NoError(t, err)

	tests := []struct {
		name         string
		locationPart string
		itemPart     string
		commandName  string
		want         string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "valid location and item",
			locationPart: "Garage",
			itemPart:     "Wrench",
			commandName:  "wherehouse move",
			want:         wrenchID,
			wantErr:      false,
		},
		{
			name:         "canonical names",
			locationPart: "garage",
			itemPart:     "wrench",
			commandName:  "wherehouse move",
			want:         wrenchID,
			wantErr:      false,
		},
		{
			name:         "location not found",
			locationPart: "Kitchen",
			itemPart:     "Wrench",
			commandName:  "wherehouse move",
			want:         "",
			wantErr:      true,
			errContains:  "location \"Kitchen\" not found",
		},
		{
			name:         "item not in location",
			locationPart: "Garage",
			itemPart:     "Hammer",
			commandName:  "wherehouse move",
			want:         "",
			wantErr:      true,
			errContains:  "item \"Hammer\" not found in location \"Garage\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := resolveLocationItemSelector(ctx, db, tt.locationPart, tt.itemPart, tt.commandName)
			if tt.wantErr {
				require.Error(t, gotErr)
				assert.Empty(t, got)
				if tt.errContains != "" {
					assert.Contains(t, gotErr.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, gotErr)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestResolveItemByCanonicalName(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	// Create test location
	garageID := uuid.New().String()
	_, err := db.AppendEvent(ctx, "location.created", "test-user", map[string]any{
		"location_id":  garageID,
		"display_name": "Garage",
		"parent_id":    nil,
	}, "")
	require.NoError(t, err)

	// Create unique item
	hammerID := uuid.New().String()
	_, err = db.AppendEvent(ctx, "item.created", "test-user", map[string]any{
		"item_id":      hammerID,
		"display_name": "Hammer",
		"location_id":  garageID,
	}, "")
	require.NoError(t, err)

	// Create duplicate wrenches
	wrench1ID := uuid.New().String()
	_, err = db.AppendEvent(ctx, "item.created", "test-user", map[string]any{
		"item_id":      wrench1ID,
		"display_name": "Wrench",
		"location_id":  garageID,
	}, "")
	require.NoError(t, err)

	wrench2ID := uuid.New().String()
	_, err = db.AppendEvent(ctx, "item.created", "test-user", map[string]any{
		"item_id":      wrench2ID,
		"display_name": "Wrench",
		"location_id":  garageID,
	}, "")
	require.NoError(t, err)

	tests := []struct {
		name        string
		input       string
		commandName string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name:        "unique item found",
			input:       "Hammer",
			commandName: "wherehouse move",
			want:        hammerID,
			wantErr:     false,
		},
		{
			name:        "canonical name resolution",
			input:       "hammer",
			commandName: "wherehouse move",
			want:        hammerID,
			wantErr:     false,
		},
		{
			name:        "ambiguous item",
			input:       "Wrench",
			commandName: "wherehouse move",
			want:        "",
			wantErr:     true,
			errContains: "multiple items",
		},
		{
			name:        "item not found",
			input:       "Screwdriver",
			commandName: "wherehouse move",
			want:        "",
			wantErr:     true,
			errContains: "item \"Screwdriver\" not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := resolveItemByCanonicalName(ctx, db, tt.input, tt.commandName)
			if tt.wantErr {
				require.Error(t, gotErr)
				assert.Empty(t, got)
				if tt.errContains != "" {
					assert.Contains(t, gotErr.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, gotErr)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestBuildAmbiguousItemError(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	// Create test location
	garageID := uuid.New().String()
	_, err := db.AppendEvent(ctx, "location.created", "test-user", map[string]any{
		"location_id":  garageID,
		"display_name": "Garage",
		"parent_id":    nil,
	}, "")
	require.NoError(t, err)

	// Create duplicate items
	item1ID := uuid.New().String()
	item2ID := uuid.New().String()

	items := []*database.Item{
		{ItemID: item1ID, DisplayName: "Wrench", CanonicalName: "wrench", LocationID: garageID},
		{ItemID: item2ID, DisplayName: "Wrench", CanonicalName: "wrench", LocationID: garageID},
	}

	tests := []struct {
		name        string
		commandName string
		wantContain []string
	}{
		{
			name:        "move command",
			commandName: "wherehouse move",
			wantContain: []string{
				"multiple items named \"wrench\" found",
				item1ID,
				item2ID,
				"wherehouse move --id",
			},
		},
		{
			name:        "history command",
			commandName: "wherehouse history",
			wantContain: []string{
				"multiple items named \"wrench\" found",
				"wherehouse history --id",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := buildAmbiguousItemError(ctx, db, "wrench", items, tt.commandName)
			require.Error(t, gotErr)
			errMsg := gotErr.Error()
			for _, want := range tt.wantContain {
				assert.Contains(t, errMsg, want)
			}
		})
	}
}

// setupTestDB creates a test database with migrations applied.
func setupTestDB(t *testing.T) *database.Database {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := database.DefaultConfig()
	cfg.Path = dbPath
	cfg.AutoMigrate = true

	db, err := database.Open(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("failed to close test database: %v", closeErr)
		}
	})

	return db
}
