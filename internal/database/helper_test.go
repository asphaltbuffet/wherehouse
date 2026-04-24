package database

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
)

// Test Constants - Fixed 10-character alphanumeric IDs for reproducible tests.
const (
	// TestActorUser is the test user ID for event attribution.
	TestActorUser = "test-user"

	// Test timing and depth constants.
	testWaitTimeoutSeconds   = 5
	testWaitPollMilliseconds = 10
	testExpectedDepthLevel2  = 2
)

// NewTestDB creates a new test database with migrations applied.
// It returns a Database instance connected to a temporary SQLite file.
// The database is automatically cleaned up when the test completes.
func NewTestDB(t *testing.T) *Database {
	t.Helper()

	tmpDir := t.TempDir()
	// Create temporary database file
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database with migrations
	cfg := DefaultConfig()
	cfg.Path = dbPath
	cfg.AutoMigrate = true

	db, err := Open(cfg)
	require.NoError(t, err, "failed to open test database")

	// Clean up on test completion
	t.Cleanup(func() {
		if err = db.Close(); err != nil {
			t.Logf("failed to close test database: %v", err)
		}
	})

	return db
}

// insertEvent creates a new event in the events table without updating projections.
// This is a low-level primitive for replay and seed scenarios where events are
// batch-inserted before being processed.
func (d *Database) insertEvent(
	ctx context.Context,
	eventType EventType,
	actorUserID string,
	payload any,
	note string,
) (int64, error) {
	// Marshal payload to JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Extract entity IDs from payload for indexing
	var payloadMap map[string]any
	if unmarshalErr := json.Unmarshal(payloadJSON, &payloadMap); unmarshalErr != nil {
		return 0, fmt.Errorf("failed to unmarshal payload for ID extraction: %w", unmarshalErr)
	}

	var entityID *string
	if id, ok := payloadMap["entity_id"].(string); ok && id != "" {
		entityID = &id
	}

	// Generate timestamp in RFC3339 format with Z
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Prepare note (NULL if empty)
	var notePtr *string
	if note != "" {
		notePtr = &note
	}

	// Insert event
	const query = `
		INSERT INTO events (
			event_type,
			timestamp_utc,
			actor_user_id,
			payload,
			note,
			entity_id
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := d.db.ExecContext(ctx, query,
		eventType,
		timestamp,
		actorUserID,
		string(payloadJSON),
		notePtr,
		entityID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert event: %w", err)
	}

	eventID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get event ID: %w", err)
	}

	return eventID, nil
}

// WaitForEventID is a test helper that waits for a specific event to be created.
// Useful for testing concurrent scenarios. Times out after 5 seconds.
func (d *Database) WaitForEventID(ctx context.Context, targetEventID int64) error {
	deadline := time.Now().Add(testWaitTimeoutSeconds * time.Second)
	for {
		var count int
		err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE event_id >= ?", targetEventID).Scan(&count)
		if err != nil {
			return err
		}

		if count > 0 {
			return nil
		}

		if time.Now().After(deadline) {
			return ErrEventNotFound
		}

		time.Sleep(testWaitPollMilliseconds * time.Millisecond)
	}
}
