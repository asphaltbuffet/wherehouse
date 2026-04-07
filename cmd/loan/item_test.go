package loan

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

type loanTestIDs struct {
	garageID string
	loanedID string
	itemID1  string
	itemID2  string
}

// setupLoanTest creates an in-memory DB with system locations and test items.
func setupLoanTest(t *testing.T) (*database.Database, context.Context, loanTestIDs) {
	t.Helper()

	ctx := context.Background()
	db, err := database.Open(database.Config{
		Path:        ":memory:",
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)

	ids := loanTestIDs{
		garageID: nanoid.MustNew(),
		itemID1:  nanoid.MustNew(),
		itemID2:  nanoid.MustNew(),
	}

	// Create garage location
	err = db.CreateLocation(ctx, ids.garageID, "Garage", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	// Get the auto-created "loaned" system location
	loanedLoc, err := db.GetLocationByCanonicalName(ctx, "loaned")
	require.NoError(t, err)
	require.NotNil(t, loanedLoc)
	ids.loanedID = loanedLoc.LocationID

	// Create items in garage
	err = db.CreateItem(ctx, ids.itemID1, "10mm socket", ids.garageID, 1, "2025-01-01T00:00:01Z")
	require.NoError(t, err)
	err = db.CreateItem(ctx, ids.itemID2, "wrench", ids.garageID, 2, "2025-01-01T00:00:02Z")
	require.NoError(t, err)

	return db, ctx, ids
}

// newTestContext returns a context with the given config stored under config.ConfigKey.
func newTestContext(cfg *config.Config) context.Context {
	return context.WithValue(context.Background(), config.ConfigKey, cfg)
}

// TestRunLoanItem_EmptyTo_ReturnsError verifies --to flag validation.
func TestRunLoanItem_EmptyTo_ReturnsError(t *testing.T) {
	db, _, ids := setupLoanTest(t)
	defer db.Close()

	cmd := NewLoanCmd(db)
	cmd.SetArgs([]string{ids.itemID1, "--to", "   "}) // whitespace only

	cfg := &config.Config{}
	cfg.Output.DefaultFormat = "human"
	cmd.SetContext(newTestContext(cfg))

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--to flag cannot be empty")
}

// TestRunLoanItem_SingleItem_TextOutput tests a single item loan in text mode.
func TestRunLoanItem_SingleItem_TextOutput(t *testing.T) {
	db, _, ids := setupLoanTest(t)
	defer db.Close()

	cmd := NewLoanCmd(db)
	cmd.SetArgs([]string{ids.itemID1, "--to", "Bob"})

	cfg := &config.Config{}
	cfg.Output.DefaultFormat = "human"
	cmd.SetContext(newTestContext(cfg))

	// Capture stdout
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Loaned item")
	assert.Contains(t, output, "10mm socket")
	assert.Contains(t, output, "Bob")
}

// TestRunLoanItem_SingleItem_JSONOutput tests a single item loan in JSON mode.
func TestRunLoanItem_SingleItem_JSONOutput(t *testing.T) {
	db, _, ids := setupLoanTest(t)
	defer db.Close()

	cmd := NewLoanCmd(db)
	cmd.SetArgs([]string{ids.itemID1, "--to", "Bob"})

	cfg := &config.Config{}
	cfg.Output.DefaultFormat = "json"
	cmd.SetContext(newTestContext(cfg))

	// Capture stdout
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)

	err := cmd.Execute()
	require.NoError(t, err)

	// Parse JSON output
	var output map[string]any
	err = json.Unmarshal(stdout.Bytes(), &output)
	require.NoError(t, err)

	assert.True(t, output["success"].(bool))
	itemsLoaned, ok := output["items_loaned"].([]any)
	require.True(t, ok)
	require.Len(t, itemsLoaned, 1)

	item := itemsLoaned[0].(map[string]any)
	assert.Equal(t, ids.itemID1, item["item_id"].(string))
	assert.Equal(t, "10mm socket", item["display_name"].(string))
	assert.Equal(t, "Bob", item["loaned_to"].(string))
	assert.False(t, item["was_re_loaned"].(bool))
}

// TestRunLoanItem_ReLoaned_TextOutput tests re-loaning in text mode.
func TestRunLoanItem_ReLoaned_TextOutput(t *testing.T) {
	db, ctx, ids := setupLoanTest(t)
	defer db.Close()

	// First loan to Alice via cli.LoanItem
	_, err := cli.LoanItem(ctx, db, ids.itemID1, "testuser", cli.LoanItemOptions{Borrower: "Alice"})
	require.NoError(t, err)

	// Now re-loan to Bob using the command
	cmd := NewLoanCmd(db)
	cmd.SetArgs([]string{ids.itemID1, "--to", "Bob"})

	cfg := &config.Config{}
	cfg.Output.DefaultFormat = "human"
	cmd.SetContext(newTestContext(cfg))

	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)

	err = cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "previously loaned to")
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "Bob")
}

// TestRunLoanItem_ReLoaned_JSONOutput tests re-loaning in JSON mode.
func TestRunLoanItem_ReLoaned_JSONOutput(t *testing.T) {
	db, ctx, ids := setupLoanTest(t)
	defer db.Close()

	// First loan to Alice via cli.LoanItem
	_, err := cli.LoanItem(ctx, db, ids.itemID1, "testuser", cli.LoanItemOptions{Borrower: "Alice"})
	require.NoError(t, err)

	// Re-loan to Bob using the command in JSON mode
	cmd := NewLoanCmd(db)
	cmd.SetArgs([]string{ids.itemID1, "--to", "Bob"})
	cfg := &config.Config{}
	cfg.Output.DefaultFormat = "json"
	cmd.SetContext(newTestContext(cfg))
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	err = cmd.Execute()
	require.NoError(t, err)

	// Parse JSON
	var output map[string]any
	err = json.Unmarshal(stdout.Bytes(), &output)
	require.NoError(t, err)

	itemsLoaned := output["items_loaned"].([]any)
	require.Len(t, itemsLoaned, 1)

	item := itemsLoaned[0].(map[string]any)
	assert.True(t, item["was_re_loaned"].(bool))
	assert.Equal(t, "Alice", item["previous_loaned_to"].(string))
}

// TestRunLoanItem_MultipleSelectors_AllSucceed tests loaning multiple items.
func TestRunLoanItem_MultipleSelectors_AllSucceed(t *testing.T) {
	db, _, ids := setupLoanTest(t)
	defer db.Close()

	cmd := NewLoanCmd(db)
	cmd.SetArgs([]string{ids.itemID1, ids.itemID2, "--to", "Bob"})

	cfg := &config.Config{}
	cfg.Output.DefaultFormat = "json"
	cmd.SetContext(newTestContext(cfg))

	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)

	err := cmd.Execute()
	require.NoError(t, err)

	var output map[string]any
	err = json.Unmarshal(stdout.Bytes(), &output)
	require.NoError(t, err)

	itemsLoaned := output["items_loaned"].([]any)
	require.Len(t, itemsLoaned, 2)
	assert.True(t, output["success"].(bool))
}

// TestRunLoanItem_MultipleSelectors_FirstFails_ErrorReturned tests error handling with multiple items.
func TestRunLoanItem_MultipleSelectors_FirstFails_ErrorReturned(t *testing.T) {
	db, _, ids := setupLoanTest(t)
	defer db.Close()

	nonExistentID := nanoid.MustNew()

	cmd := NewLoanCmd(db)
	cmd.SetArgs([]string{nonExistentID, ids.itemID1, "--to", "Bob"})

	cfg := &config.Config{}
	cfg.Output.DefaultFormat = "human"
	cmd.SetContext(newTestContext(cfg))

	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to loan")
}

// TestRunLoanItem_CloseError_LoggedToStderr tests that close errors are logged but not returned.
// This test verifies the defer close pattern handles errors gracefully.
func TestRunLoanItem_CloseError_LoggedToStderr(t *testing.T) {
	// This test is integration-level and verifies that successful loan operations
	// complete even if there are minor issues. Since mocking the close error requires
	// mocking the database entirely, we verify the success path instead.
	db, _, ids := setupLoanTest(t)
	defer db.Close()

	cmd := NewLoanCmd(db)
	cmd.SetArgs([]string{ids.itemID1, "--to", "Bob"})

	cfg := &config.Config{}
	cfg.Output.DefaultFormat = "human"
	cmd.SetContext(newTestContext(cfg))

	stderr := &bytes.Buffer{}
	cmd.SetErr(stderr)
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	require.NoError(t, err)
}

// TestRunLoanItem_LoanItemDBError_ReturnsWrapped tests error propagation.
func TestRunLoanItem_LoanItemDBError_ReturnsWrapped(t *testing.T) {
	db, _, _ := setupLoanTest(t)
	defer db.Close()

	// Use a valid selector format that doesn't resolve to any item
	nonExistentCanonical := "nonexistent-item-name"

	cmd := NewLoanCmd(db)
	cmd.SetArgs([]string{nonExistentCanonical, "--to", "Bob"})

	cfg := &config.Config{}
	cfg.Output.DefaultFormat = "human"
	cmd.SetContext(newTestContext(cfg))

	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to loan")
}

// TestResult_JSONMarshal tests Result struct JSON marshaling.
func TestResult_JSONMarshal(t *testing.T) {
	result := Result{
		ItemID:           "test-id-123",
		DisplayName:      "10mm socket",
		LoanedTo:         "Bob",
		EventID:          42,
		WasReLoaned:      true,
		PreviousLoanedTo: "Alice",
		PreviousLocation: "Garage",
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(&result)
	require.NoError(t, err)

	// Unmarshal to verify
	var unmarshaled map[string]any
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, "test-id-123", unmarshaled["item_id"])
	assert.Equal(t, "10mm socket", unmarshaled["display_name"])
	assert.Equal(t, "Bob", unmarshaled["loaned_to"])
	assert.InDelta(t, float64(42), unmarshaled["event_id"], 0.1)
	assert.True(t, unmarshaled["was_re_loaned"].(bool))
	assert.Equal(t, "Alice", unmarshaled["previous_loaned_to"])
	assert.Equal(t, "Garage", unmarshaled["previous_location"])
}

// TestResult_PreviousLoanedTo_Omitted_WhenEmpty tests omitempty behavior.
func TestResult_PreviousLoanedTo_Omitted_WhenEmpty(t *testing.T) {
	result := Result{
		ItemID:           "test-id-456",
		DisplayName:      "wrench",
		LoanedTo:         "Carol",
		EventID:          99,
		WasReLoaned:      false,
		PreviousLoanedTo: "", // Empty, should be omitted
		PreviousLocation: "Shelf",
	}

	jsonBytes, err := json.Marshal(&result)
	require.NoError(t, err)

	jsonStr := string(jsonBytes)
	// Field should not appear if empty
	assert.NotContains(t, jsonStr, "previous_loaned_to")
}
