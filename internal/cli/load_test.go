package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// newTestLoadDB creates a test database for loadCSV tests.
func newTestLoadDB(t *testing.T) *database.Database {
	t.Helper()
	db, err := database.Open(database.Config{
		Path:        filepath.Join(t.TempDir(), "test.db"),
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// writeTestCSV writes a CSV file to a temp directory and returns its absolute path.
func writeTestCSV(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.csv")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	abs, err := filepath.Abs(p)
	require.NoError(t, err)
	return abs
}

// Hard error tests using exported LoadCSV (these don't need a DB).

func TestLoadCSV_WrongExtension(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	wrongFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(wrongFile, []byte("type,name,home\n"), 0o600))

	result, err := LoadCSV(context.Background(), wrongFile)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "invalid extension")
}

func TestLoadCSV_FileNotFound(t *testing.T) {
	t.Parallel()
	result, err := LoadCSV(context.Background(), "/nonexistent/path/to/file.csv")
	require.Error(t, err)
	assert.Nil(t, result)
	// Error message varies by OS; just check that we got an error.
}

// Inner function tests using loadCSV directly with a real test DB.

func TestLoadCSV_EmptyFile(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	abs := writeTestCSV(t, "type,name,home\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "test.csv", result.Path)
	assert.Equal(t, 0, result.ItemCount)
	assert.Equal(t, 0, result.LocationCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_ValidLocationRow(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	abs := writeTestCSV(t, "type,name,home\nL,Garage,\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.LocationCount)
	assert.Equal(t, 0, result.ItemCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_ValidItemRow(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Location must be defined first in the CSV so the item can reference it.
	abs := writeTestCSV(t, "type,name,home\nL,Garage,\nI,Hammer,Garage\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.LocationCount)
	assert.Equal(t, 1, result.ItemCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_ItemBeforeLocation(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Item row before its home location is created.
	abs := writeTestCSV(t, "type,name,home\nI,Hammer,Garage\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 0, result.LocationCount)
	assert.Equal(t, 0, result.ItemCount)
	require.Len(t, result.InvalidEntries, 1)

	inv := result.InvalidEntries[0]
	assert.Equal(t, 2, inv.Line)
	assert.Equal(t, "Hammer", inv.Entry)
	assert.NotEmpty(t, inv.Error)
}

func TestLoadCSV_MissingName(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	abs := writeTestCSV(t, "type,name,home\nL,,\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 0, result.LocationCount)
	require.Len(t, result.InvalidEntries, 1)

	inv := result.InvalidEntries[0]
	assert.Equal(t, 2, inv.Line)
	assert.Contains(t, inv.Error, "missing name")
}

func TestLoadCSV_UnknownType(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	abs := writeTestCSV(t, "type,name,home\nX,Widget,\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 0, result.LocationCount)
	assert.Equal(t, 0, result.ItemCount)
	require.Len(t, result.InvalidEntries, 1)

	inv := result.InvalidEntries[0]
	assert.Equal(t, 2, inv.Line)
	assert.Equal(t, "Widget", inv.Entry)
	assert.Contains(t, inv.Error, "unknown type")
}

func TestLoadCSV_ColonInName(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	abs := writeTestCSV(t, "type,name,home\nL,Ga:rage,\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 0, result.LocationCount)
	require.Len(t, result.InvalidEntries, 1)

	inv := result.InvalidEntries[0]
	assert.Equal(t, 2, inv.Line)
	assert.Equal(t, "Ga:rage", inv.Entry)
	assert.Contains(t, inv.Error, ":")
}

func TestLoadCSV_BlankAndCommentRowsSkipped(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Blank row and comment row should be skipped without counting toward line numbers.
	// However, the line counter in the code does NOT skip them in the displayed line number,
	// so we need to account for that behavior.
	csv := `type,name,home

# This is a comment
L,Garage,
`
	abs := writeTestCSV(t, csv)

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	// The blank row on line 2 is skipped (lineNum-- happens),
	// the comment row on line 3 is skipped (lineNum-- happens),
	// the location on line 4 is processed successfully.
	assert.Equal(t, 1, result.LocationCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_NestedLocation(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Parent location, then child location with home set to parent name.
	abs := writeTestCSV(t, "type,name,home\nL,Garage,\nL,Shelf,Garage\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 2, result.LocationCount)
	assert.Equal(t, 0, result.ItemCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_ItemMissingHome(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Item row with empty home field.
	abs := writeTestCSV(t, "type,name,home\nI,Hammer,\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 0, result.ItemCount)
	require.Len(t, result.InvalidEntries, 1)

	inv := result.InvalidEntries[0]
	assert.Equal(t, 2, inv.Line)
	assert.Equal(t, "Hammer", inv.Entry)
	assert.Contains(t, inv.Error, "missing home")
}

func TestLoadCSV_MultipleLocationsAndItems(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Multiple locations and items with various nesting.
	csv := `type,name,home
L,House,
L,Garage,House
L,Workshop,House
I,Hammer,Workshop
I,Wrench,Workshop
I,Flashlight,Garage
`
	abs := writeTestCSV(t, csv)

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 3, result.LocationCount)
	assert.Equal(t, 3, result.ItemCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_CaseSensitiveType(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Type is case-insensitive (uppercase conversion happens).
	abs := writeTestCSV(t, "type,name,home\nl,Garage,\ni,Hammer,Garage\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.LocationCount)
	assert.Equal(t, 1, result.ItemCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_FlexibleColumnOrder(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Columns in different order.
	abs := writeTestCSV(t, "name,home,type\nGarage,,L\nHammer,Garage,I\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.LocationCount)
	assert.Equal(t, 1, result.ItemCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_DuplicateLocationName(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Try to add the same location twice.
	abs := writeTestCSV(t, "type,name,home\nL,Garage,\nL,Garage,\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	// First location succeeds, second fails due to duplicate.
	assert.Equal(t, 1, result.LocationCount)
	require.Len(t, result.InvalidEntries, 1)

	inv := result.InvalidEntries[0]
	assert.Equal(t, 3, inv.Line)
	assert.Equal(t, "Garage", inv.Entry)
	assert.NotEmpty(t, inv.Error)
}

func TestLoadCSV_WhitespaceHandling(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Whitespace around field values should be trimmed.
	abs := writeTestCSV(t, "type,name,home\n L , Garage , \n I , Hammer , Garage \n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.LocationCount)
	assert.Equal(t, 1, result.ItemCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_HeaderColumnCaseInsensitive(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Header column names should be case-insensitive.
	abs := writeTestCSV(t, "TYPE,NAME,HOME\nL,Garage,\nI,Hammer,Garage\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.LocationCount)
	assert.Equal(t, 1, result.ItemCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_MissingOptionalHomeForLocation(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Home is optional for locations (can be completely missing from CSV).
	abs := writeTestCSV(t, "type,name\nL,Garage\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.LocationCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_MalformedCSVRow(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// An unterminated quoted field causes encoding/csv to return a parse error.
	// The row should be recorded as an InvalidEntry and parsing should continue.
	abs := writeTestCSV(t, "type,name,home\nL,\"unterminated")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 0, result.LocationCount)
	require.Len(t, result.InvalidEntries, 1)
	assert.Contains(t, result.InvalidEntries[0].Error, "malformed CSV row")
}

func TestLoadCSV_MultipleInvalidRows(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Multiple invalid rows should all be recorded.
	csv := `type,name,home
X,BadType,
L,,
I,NoHome,
L,Ga:rage,
L,ValidLocation,
`
	abs := writeTestCSV(t, csv)

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.LocationCount)
	assert.Equal(t, 0, result.ItemCount)
	require.Len(t, result.InvalidEntries, 4)

	// Verify line numbers are correct.
	expectedLines := []int{2, 3, 4, 5}
	for i, inv := range result.InvalidEntries {
		assert.Equal(t, expectedLines[i], inv.Line, "invalid entry %d should be on line %d", i, expectedLines[i])
	}
}

func TestLoadCSV_EmptyCSVHeaderOnly(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// CSV with only a header and no data rows.
	abs := writeTestCSV(t, "type,name,home")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 0, result.LocationCount)
	assert.Equal(t, 0, result.ItemCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_PathPreservedInResult(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	abs := writeTestCSV(t, "type,name,home\nL,Garage,\n")
	displayPath := "my_data/import.csv"

	result, err := loadCSV(context.Background(), db, abs, displayPath)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, displayPath, result.Path)
}

func TestLoadCSV_ExtraColumnsIgnored(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Extra columns beyond type, name, home should be ignored.
	abs := writeTestCSV(t, "type,name,home,extra,more\nL,Garage,,data,stuff\nI,Hammer,Garage,unused,columns\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.LocationCount)
	assert.Equal(t, 1, result.ItemCount)
	assert.Empty(t, result.InvalidEntries)
}

func TestLoadCSV_ContextPropagation(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	abs := writeTestCSV(t, "type,name,home\nL,Garage,\n")

	// Use a context that could be cancelled.
	ctx := t.Context()

	result, err := loadCSV(ctx, db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.LocationCount)
}

func TestLoadCSV_AllLocationNamesInvalid(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// All locations have invalid names (e.g., colons).
	csv := `type,name,home
L,Ga:rage,
L,Work:shop,
I,Hammer,Garage
`
	abs := writeTestCSV(t, csv)

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 0, result.LocationCount)
	assert.Equal(t, 0, result.ItemCount)
	require.Len(t, result.InvalidEntries, 3)
}

func TestLoadCSV_ItemNameWithinValidCharacters(t *testing.T) {
	t.Parallel()
	db := newTestLoadDB(t)
	// Names with hyphens, underscores, numbers should be valid.
	abs := writeTestCSV(t, "type,name,home\nL,Garage-1,\nI,Hammer_10mm,Garage-1\nI,Tool-123,Garage-1\n")

	result, err := loadCSV(context.Background(), db, abs, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.LocationCount)
	assert.Equal(t, 2, result.ItemCount)
	assert.Empty(t, result.InvalidEntries)
}
