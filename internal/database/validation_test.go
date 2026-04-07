package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateFromLocation tests the ValidateFromLocation function.
// This is critical for detecting projection corruption and concurrent modifications.
func TestValidateFromLocation(t *testing.T) {
	type args struct {
		itemID    string
		fromLocID string
	}

	tests := []struct {
		name                string
		args                args
		errAssertion        require.ErrorAssertionFunc
		expectedErrContains string
	}{
		{
			name:         "correct from_location matches proection",
			args:         args{TestItem10mmSocket, TestLocationToolbox},
			errAssertion: require.NoError,
		},
		{
			name:                "from_location mismatch with different location",
			args:                args{TestItem10mmSocket, TestLocationWorkbench},
			errAssertion:        require.Error,
			expectedErrContains: "from_location mismatch",
		},
		{
			name:                "non-existent item returns not found error",
			args:                args{"nonexistent-item-id", TestLocationToolbox},
			errAssertion:        require.Error,
			expectedErrContains: "item not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewTestDBWithSeed(t)
			ctx := t.Context()

			err := db.ValidateFromLocation(ctx, tt.args.itemID, tt.args.fromLocID)

			tt.errAssertion(t, err)
			if tt.expectedErrContains != "" {
				assert.ErrorContains(t, err, tt.expectedErrContains)
			}
		})
	}
}

// TestDetectLocationCycle tests the DetectLocationCycle function.
// A cycle occurs when a location would become its own ancestor.
func TestDetectLocationCycle(t *testing.T) {
	type args struct {
		locationID  string
		newParentID string
	}

	tests := []struct {
		name                string
		args                args
		setupFunc           func(t *testing.T) (locationID string, newParentID *string)
		errAssertion        require.ErrorAssertionFunc
		expectedErrContains string
	}{
		{
			name:         "nil parent is always safe (root location)",
			args:         args{TestLocationToolbox, ""},
			errAssertion: require.NoError,
		},
		{
			name:                "location cannot be its own parent",
			args:                args{TestLocationToolbox, TestLocationToolbox},
			errAssertion:        require.Error,
			expectedErrContains: "location cycle",
		},
		{
			name:                "child cannot become parent of its parent",
			args:                args{TestLocationWorkshop, TestLocationToolbox},
			errAssertion:        require.Error,
			expectedErrContains: "location cycle",
		},
		{
			name:         "safe reparent to unrelated location",
			args:         args{TestLocationToolbox, TestLocationStorage},
			errAssertion: require.NoError,
		},
		{
			name:         "safe reparent to root (nil parent)",
			args:         args{TestLocationToolbox, ""},
			errAssertion: require.NoError,
		},
		{
			name:                "deep hierarchy cycle detection",
			args:                args{TestLocationStorage, TestLocationBinA},
			errAssertion:        require.Error,
			expectedErrContains: "location cycle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewTestDBWithSeed(t)
			ctx := t.Context()

			var newParentID *string
			if tt.args.newParentID != "" {
				newParentID = &tt.args.newParentID
			}

			err := db.DetectLocationCycle(ctx, tt.args.locationID, newParentID)

			tt.errAssertion(t, err)
			if tt.expectedErrContains != "" {
				assert.ErrorContains(t, err, tt.expectedErrContains)
			}
		})
	}
}

// TestValidateUniqueLocationName tests the ValidateUniqueLocationName function.
// Location names must be globally unique.
func TestValidateUniqueLocationName(t *testing.T) {
	type args struct {
		locationName string
		locationID   string
	}

	tests := []struct {
		name         string
		args         args
		errAssertion require.ErrorAssertionFunc
		wantErrMsg   string
	}{
		{
			name:         "unique name passes validation",
			args:         args{"completely_unique_name_xyz", ""},
			errAssertion: require.NoError,
		},
		{
			name:         "duplicate canonical name fails",
			args:         args{"workshop", ""},
			errAssertion: require.Error,
			wantErrMsg:   "duplicate location",
		},
		{
			name:         "excluding location itself allows reuse in same ID",
			args:         args{"workshop", TestLocationWorkshop},
			errAssertion: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewTestDBWithSeed(t)
			ctx := t.Context()

			var exclude *string
			if tt.args.locationID != "" {
				exclude = &tt.args.locationID
			}

			err := db.ValidateUniqueLocationName(ctx, tt.args.locationName, exclude)

			tt.errAssertion(t, err)
			if tt.wantErrMsg != "" {
				assert.ErrorContains(t, err, tt.wantErrMsg)
			}
		})
	}
}

// TestValidateUniqueItemName tests the ValidateUniqueItemName function.
// Item names are unique per location (not globally unique).
func TestValidateUniqueItemName(t *testing.T) {
	type args struct {
		locationID    string
		canonicalname string
		excludeID     string
	}

	tests := []struct {
		name         string
		args         args
		setupFunc    func(t *testing.T) (locationID, canonicalName string, excludeID *string)
		errAssertion require.ErrorAssertionFunc
		wantErrMsg   string
	}{
		{
			name:         "unique item name in location passes",
			args:         args{TestLocationToolbox, "unique_item_xyz", ""},
			errAssertion: require.NoError,
		},
		{
			name:         "duplicate item name in same location fails",
			args:         args{TestLocationToolbox, "10mm_socket", ""},
			errAssertion: require.Error,
			wantErrMsg:   "duplicate item",
		},
		{
			name:         "same item name in different location passes",
			args:         args{TestLocationWorkbench, "10mm_socket", ""},
			errAssertion: require.NoError,
		},
		{
			name:         "excluding item itself allows reuse in same location",
			args:         args{TestLocationToolbox, "10mm_socket", TestItem10mmSocket},
			errAssertion: require.NoError,
		},
		{
			name:         "different canonical names pass",
			args:         args{TestLocationToolbox, "wrench", ""},
			errAssertion: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewTestDBWithSeed(t)
			ctx := t.Context()

			var excludeID *string
			if tt.args.excludeID != "" {
				excludeID = &tt.args.excludeID
			}

			err := db.ValidateUniqueItemName(ctx, tt.args.locationID, tt.args.canonicalname, excludeID)

			tt.errAssertion(t, err)
			if tt.wantErrMsg != "" {
				assert.ErrorContains(t, err, tt.wantErrMsg)
			}
		})
	}
}

// TestValidateLocationExists tests the ValidateLocationExists function.
func TestValidateLocationExists(t *testing.T) {
	type args struct {
		locationID string
	}

	tests := []struct {
		name         string
		args         args
		setupFunc    func(t *testing.T) string
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name:         "existing location passes",
			args:         args{TestLocationWorkshop},
			errAssertion: require.NoError,
		},
		{
			name:         "non-existent location fails",
			args:         args{"nonexistent-location-xyz"},
			errAssertion: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewTestDBWithSeed(t)
			ctx := t.Context()

			err := db.ValidateLocationExists(ctx, tt.args.locationID)

			tt.errAssertion(t, err)
		})
	}
}

// TestValidateItemExists tests the ValidateItemExists function.
func TestValidateItemExists(t *testing.T) {
	type args struct {
		itemID string
	}

	tests := []struct {
		name         string
		args         args
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name:         "existing item passes",
			args:         args{TestItem10mmSocket},
			errAssertion: require.NoError,
		},
		{
			name:         "non-existent item fails",
			args:         args{"nonexistent-item-xyz"},
			errAssertion: require.Error,
		},
		{
			name:         "item in system location (missing) exists",
			args:         args{TestItemMissingWrench},
			errAssertion: require.NoError,
		},
		{
			name:         "item in system location (borrowed) exists",
			args:         args{TestItemBorrowedSaw},
			errAssertion: require.NoError,
		},
		{
			name:         "different items from different locations",
			args:         args{TestItemHammer},
			errAssertion: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewTestDBWithSeed(t)
			ctx := t.Context()

			err := db.ValidateItemExists(ctx, tt.args.itemID)

			tt.errAssertion(t, err)
		})
	}
}

// TestValidateLocationEmpty tests the ValidateLocationEmpty function.
// A location must be empty (no child locations and no items) before deletion.
func TestValidateLocationEmpty(t *testing.T) {
	emptyID := "empty-location"

	type args struct {
		locationID string
	}

	tests := []struct {
		name         string
		setupFunc    func(t *testing.T) string
		args         args
		errAssertion require.ErrorAssertionFunc
		wantErrMsg   string
	}{
		{
			name:         "empty location (no children, no items) passes",
			args:         args{emptyID},
			errAssertion: require.NoError,
		},
		{
			name:         "location with children fails",
			args:         args{TestLocationWorkshop},
			errAssertion: require.Error,
			wantErrMsg:   "child locations",
		},
		{
			name:         "location with items fails",
			args:         args{TestLocationToolbox},
			errAssertion: require.Error,
			wantErrMsg:   "items",
		},
		{
			name:         "location with only items fails",
			args:         args{TestLocationBinA},
			errAssertion: require.Error,
			wantErrMsg:   "items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewTestDBWithSeed(t)
			ctx := t.Context()

			// Create an empty location
			require.NoError(t, db.CreateLocation(ctx, emptyID, "Empty", nil, false, 1, "2026-02-21T10:00:00Z"))

			err := db.ValidateLocationEmpty(ctx, tt.args.locationID)

			tt.errAssertion(t, err)
			if tt.wantErrMsg != "" {
				assert.ErrorContains(t, err, tt.wantErrMsg)
			}
		})
	}
}

// TestValidateSystemLocation tests the ValidateSystemLocation function.
// System locations (Missing, Borrowed) cannot be modified or deleted.
func TestValidateSystemLocation(t *testing.T) {
	type args struct {
		locationID string
	}

	tests := []struct {
		name         string
		args         args
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name:         "non-system location (Workshop) passes",
			args:         args{TestLocationWorkshop},
			errAssertion: require.NoError,
		},
		{
			name:         "non-system location (Toolbox) passes",
			args:         args{TestLocationToolbox},
			errAssertion: require.NoError,
		},
		{
			name:         "non-existent location returns not found",
			args:         args{"nonexistent-location-xyz"},
			errAssertion: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewTestDBWithSeed(t)
			ctx := t.Context()

			err := db.ValidateSystemLocation(ctx, tt.args.locationID)

			tt.errAssertion(t, err)
		})
	}
}

// TestValidateNoColonInName tests the ValidateNoColonInName function.
// Colons are reserved for the selector syntax (LOCATION:ITEM).
func TestValidateNoColonInName(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		errAssertion assert.ErrorAssertionFunc
	}{
		{
			name:         "name without colon passes",
			input:        "My Item",
			errAssertion: assert.NoError,
		},
		{
			name:         "simple name passes",
			input:        "Socket",
			errAssertion: assert.NoError,
		},
		{
			name:         "name with spaces passes",
			input:        "10mm Socket Wrench",
			errAssertion: assert.NoError,
		},
		{
			name:         "name with special chars (except colon) passes",
			input:        "Screwdriver-Set (Magnetic)",
			errAssertion: assert.NoError,
		},
		{
			name:         "name with single colon fails",
			input:        "Location:Item",
			errAssertion: assert.Error,
		},
		{
			name:         "name with colon at start fails",
			input:        ":InvalidName",
			errAssertion: assert.Error,
		},
		{
			name:         "name with colon at end fails",
			input:        "InvalidName:",
			errAssertion: assert.Error,
		},
		{
			name:         "name with multiple colons fails",
			input:        "Location:Sub:Item",
			errAssertion: assert.Error,
		},
		{
			name:         "name with only colon fails",
			input:        ":",
			errAssertion: assert.Error,
		},
		{
			name:         "empty name passes (other validation would catch it)",
			input:        "",
			errAssertion: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNoColonInName(tt.input)

			tt.errAssertion(t, err)
		})
	}
}

// TestValidateFromParent tests the ValidateFromParent function.
// This is critical for location.reparented events to detect projection corruption.
func TestValidateFromParent(t *testing.T) {
	type args struct {
		locationID   string
		fromParentID string
	}

	tests := []struct {
		name         string
		args         args
		errAssertion require.ErrorAssertionFunc
		wantErrMsg   string
	}{
		{
			name:         "root location with nil parent matches nil expectation",
			args:         args{TestLocationWorkshop, ""},
			errAssertion: require.NoError,
			wantErrMsg:   "",
		},
		{
			name:         "child location with matching parent ID passes",
			args:         args{TestLocationToolbox, TestLocationWorkshop},
			errAssertion: require.NoError,
			wantErrMsg:   "",
		},
		{
			name:         "root location with non-nil expectation fails",
			args:         args{TestLocationWorkshop, TestLocationStorage},
			errAssertion: require.Error,
			wantErrMsg:   "parent mismatch",
		},
		{
			name:         "child location with nil expectation fails",
			args:         args{TestLocationToolbox, ""},
			errAssertion: require.Error,
			wantErrMsg:   "parent mismatch",
		},
		{
			name:         "child location with mismatched parent ID fails",
			args:         args{TestLocationToolbox, TestLocationStorage},
			errAssertion: require.Error,
			wantErrMsg:   "parent mismatch",
		},
		{
			name:         "deep hierarchy child location matches parent",
			args:         args{TestLocationBinA, TestLocationShelves},
			errAssertion: require.NoError,
			wantErrMsg:   "",
		},
		{
			name:         "non-existent location returns not found",
			args:         args{"nonexistent-location-xyz", ""},
			errAssertion: require.Error,
			wantErrMsg:   "failed to get location",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewTestDBWithSeed(t)
			ctx := t.Context()

			var fromParentID *string
			if tt.args.fromParentID != "" {
				fromParentID = &tt.args.fromParentID
			}

			err := db.ValidateFromParent(ctx, tt.args.locationID, fromParentID)

			tt.errAssertion(t, err)
			if tt.wantErrMsg != "" {
				assert.ErrorContains(t, err, tt.wantErrMsg)
			}
		})
	}
}
