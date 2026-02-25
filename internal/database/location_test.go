package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLocationByCanonicalName_Success(t *testing.T) {
	db := NewTestDBWithSeed(t)
	ctx := context.Background()

	tests := []struct {
		name              string
		canonicalName     string
		expectedID        string
		expectedDisplay   string
		expectedIsSystem  bool
		expectedDepth     int
		expectedPathDisp  string
		expectedPathCanon string
	}{
		{
			name:              "root location - workshop",
			canonicalName:     "workshop",
			expectedID:        TestLocationWorkshop,
			expectedDisplay:   "Workshop",
			expectedIsSystem:  false,
			expectedDepth:     0,
			expectedPathDisp:  "Workshop",
			expectedPathCanon: "workshop",
		},
		{
			name:              "root location - storage",
			canonicalName:     "storage",
			expectedID:        TestLocationStorage,
			expectedDisplay:   "Storage",
			expectedIsSystem:  false,
			expectedDepth:     0,
			expectedPathDisp:  "Storage",
			expectedPathCanon: "storage",
		},
		{
			name:              "child location - toolbox",
			canonicalName:     "toolbox",
			expectedID:        TestLocationToolbox,
			expectedDisplay:   "Toolbox",
			expectedIsSystem:  false,
			expectedDepth:     1,
			expectedPathDisp:  "Workshop >> Toolbox",
			expectedPathCanon: "workshop:toolbox",
		},
		{
			name:              "nested location - bin_a",
			canonicalName:     "bin_a",
			expectedID:        TestLocationBinA,
			expectedDisplay:   "Bin A",
			expectedIsSystem:  false,
			expectedDepth:     2,
			expectedPathDisp:  "Storage >> Shelves >> Bin A",
			expectedPathCanon: "storage:shelves:bin_a",
		},
		{
			name:              "system location - missing",
			canonicalName:     "missing",
			expectedDisplay:   "Missing",
			expectedIsSystem:  true,
			expectedDepth:     0,
			expectedPathDisp:  "Missing",
			expectedPathCanon: "missing",
		},
		{
			name:              "system location - borrowed",
			canonicalName:     "borrowed",
			expectedDisplay:   "Borrowed",
			expectedIsSystem:  true,
			expectedDepth:     0,
			expectedPathDisp:  "Borrowed",
			expectedPathCanon: "borrowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := db.GetLocationByCanonicalName(ctx, tt.canonicalName)
			require.NoError(t, err)
			require.NotNil(t, loc)

			assert.Equal(t, tt.canonicalName, loc.CanonicalName)
			assert.Equal(t, tt.expectedDisplay, loc.DisplayName)
			assert.Equal(t, tt.expectedIsSystem, loc.IsSystem)
			assert.Equal(t, tt.expectedDepth, loc.Depth)
			assert.Equal(t, tt.expectedPathDisp, loc.FullPathDisplay)
			assert.Equal(t, tt.expectedPathCanon, loc.FullPathCanonical)

			if tt.expectedID != "" {
				assert.Equal(t, tt.expectedID, loc.LocationID)
			}
		})
	}
}

func TestGetLocationByCanonicalName_NotFound(t *testing.T) {
	db := NewTestDBWithSeed(t)
	ctx := context.Background()

	tests := []struct {
		name          string
		canonicalName string
	}{
		{
			name:          "nonexistent location",
			canonicalName: "nonexistent",
		},
		{
			name:          "empty string",
			canonicalName: "",
		},
		{
			name:          "almost matching name",
			canonicalName: "workshops",
		},
		{
			name:          "wrong case sensitivity should not match",
			canonicalName: "Workshop", // canonical should be lowercase
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := db.GetLocationByCanonicalName(ctx, tt.canonicalName)
			require.ErrorIs(t, err, ErrLocationNotFound)
			assert.Nil(t, loc)
		})
	}
}

func TestGetLocationByCanonicalName_Ambiguous(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	// Create two locations with the same canonical name but different parents
	// This violates the global uniqueness constraint from business rules,
	// but tests that the function correctly detects this scenario

	// Create parent locations
	parentA := "parent-a-uuid"
	parentB := "parent-b-uuid"

	err := db.CreateLocation(ctx, parentA, "Parent A", nil, false, 0, "2026-02-24T00:00:00Z")
	require.NoError(t, err)

	err = db.CreateLocation(ctx, parentB, "Parent B", nil, false, 0, "2026-02-24T00:00:00Z")
	require.NoError(t, err)

	// Create two child locations with same canonical name (duplicate)
	// Note: This will fail due to UNIQUE constraint in current schema,
	// but if the schema is changed to allow this, the function should detect it
	childA := "child-a-uuid"
	childB := "child-b-uuid"

	err = db.CreateLocation(ctx, childA, "Tools", &parentA, false, 0, "2026-02-24T00:00:01Z")
	require.NoError(t, err)

	// Try to create second location with same canonical name in different parent
	err = db.CreateLocation(ctx, childB, "Tools", &parentB, false, 0, "2026-02-24T00:00:02Z")

	// Current schema enforces uniqueness per parent, so this should succeed
	require.NoError(t, err)

	// Now query by canonical name - should get 2 matches
	loc, err := db.GetLocationByCanonicalName(ctx, "tools")

	// Check for ambiguous error
	var ambigErr *AmbiguousLocationError
	if assert.ErrorAs(t, err, &ambigErr) {
		assert.Equal(t, "tools", ambigErr.CanonicalName)
		assert.Len(t, ambigErr.MatchingIDs, 2)
		assert.Contains(t, ambigErr.MatchingIDs, childA)
		assert.Contains(t, ambigErr.MatchingIDs, childB)
	}
	assert.Nil(t, loc)
}

func TestGetLocationByCanonicalName_SystemLocationFlags(t *testing.T) {
	// Test that is_system flag queries work correctly
	db := NewTestDBWithSeed(t)
	ctx := context.Background()

	tests := []struct {
		name           string
		canonicalName  string
		expectedSystem bool
	}{
		{
			name:           "system location - missing",
			canonicalName:  "missing",
			expectedSystem: true,
		},
		{
			name:           "system location - borrowed",
			canonicalName:  "borrowed",
			expectedSystem: true,
		},
		{
			name:           "non-system location - workshop",
			canonicalName:  "workshop",
			expectedSystem: false,
		},
		{
			name:           "non-system location - toolbox",
			canonicalName:  "toolbox",
			expectedSystem: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := db.GetLocationByCanonicalName(ctx, tt.canonicalName)
			require.NoError(t, err)
			require.NotNil(t, loc)

			assert.Equal(t, tt.expectedSystem, loc.IsSystem,
				"is_system flag should be %v for %s", tt.expectedSystem, tt.canonicalName)
		})
	}
}

func TestGetLocationByCanonicalName_AllFields(t *testing.T) {
	// Test that all location fields are correctly populated
	db := NewTestDBWithSeed(t)
	ctx := context.Background()

	loc, err := db.GetLocationByCanonicalName(ctx, "toolbox")
	require.NoError(t, err)
	require.NotNil(t, loc)

	// Verify all fields are populated
	assert.NotEmpty(t, loc.LocationID, "location_id should be populated")
	assert.NotEmpty(t, loc.DisplayName, "display_name should be populated")
	assert.NotEmpty(t, loc.CanonicalName, "canonical_name should be populated")
	assert.NotNil(t, loc.ParentID, "parent_id should be populated for child location")
	assert.NotEmpty(t, loc.FullPathDisplay, "full_path_display should be populated")
	assert.NotEmpty(t, loc.FullPathCanonical, "full_path_canonical should be populated")
	assert.GreaterOrEqual(t, loc.Depth, 0, "depth should be non-negative")
	assert.NotEmpty(t, loc.UpdatedAt, "updated_at should be populated")

	// Verify specific values
	assert.Equal(t, "Toolbox", loc.DisplayName)
	assert.Equal(t, "toolbox", loc.CanonicalName)
	assert.Equal(t, TestLocationWorkshop, *loc.ParentID)
	assert.Equal(t, "Workshop >> Toolbox", loc.FullPathDisplay)
	assert.Equal(t, "workshop:toolbox", loc.FullPathCanonical)
	assert.Equal(t, 1, loc.Depth)
	assert.False(t, loc.IsSystem)
}

func TestGetLocationByCanonicalName_RootLocation(t *testing.T) {
	// Test root location has nil parent_id
	db := NewTestDBWithSeed(t)
	ctx := context.Background()

	loc, err := db.GetLocationByCanonicalName(ctx, "workshop")
	require.NoError(t, err)
	require.NotNil(t, loc)

	assert.Nil(t, loc.ParentID, "root location should have nil parent_id")
	assert.Equal(t, 0, loc.Depth, "root location should have depth 0")
	assert.Equal(t, "Workshop", loc.FullPathDisplay, "root location path_display should be just the name")
	assert.Equal(t, "workshop", loc.FullPathCanonical, "root location path_canonical should be just the canonical name")
}

func TestGetLocationByCanonicalName_CanonicalizationMatching(t *testing.T) {
	// Test that canonicalization rules are applied correctly
	db := NewTestDB(t)
	ctx := context.Background()

	// Create location with complex name
	locID := "test-loc-uuid"
	err := db.CreateLocation(ctx, locID, "Test Location With Spaces", nil, false, 0, "2026-02-24T00:00:00Z")
	require.NoError(t, err)

	// Query using canonical form
	loc, err := db.GetLocationByCanonicalName(ctx, "test_location_with_spaces")
	require.NoError(t, err)
	require.NotNil(t, loc)

	assert.Equal(t, locID, loc.LocationID)
	assert.Equal(t, "Test Location With Spaces", loc.DisplayName)
	assert.Equal(t, "test_location_with_spaces", loc.CanonicalName)
}

func TestGetLocationByCanonicalName_EmptyDatabase(t *testing.T) {
	// Test querying empty database (no seed data, but migrations applied)
	db := NewTestDB(t)
	ctx := context.Background()

	loc, err := db.GetLocationByCanonicalName(ctx, "anything")

	// Should get not found error, not a database error
	require.ErrorIs(t, err, ErrLocationNotFound)
	assert.Nil(t, loc)
}
