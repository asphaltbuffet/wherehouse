package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/goccy/go-json"
)

// LoanedInfo represents information about a loaned item.
type LoanedInfo struct {
	LoanedTo    string
	LoanedAt    time.Time
	EventID     int64
	ActorUserID string
}

// GetItemLoanedInfo retrieves the loaned_to value and timestamp from the latest item.loaned event.
// This is used by the find command to display loan information for items in the Loaned location.
// Returns ErrEventNotFound if no item.loaned event exists for this item.
func (d *Database) GetItemLoanedInfo(ctx context.Context, itemID string) (*LoanedInfo, error) {
	const query = `
		SELECT
			event_id,
			timestamp_utc,
			actor_user_id,
			payload
		FROM events
		WHERE item_id = ? AND event_type = 'item.loaned'
		ORDER BY event_id DESC
		LIMIT 1
	`

	var eventID int64
	var timestampStr string
	var actorUserID string
	var payloadStr string

	err := d.db.QueryRowContext(ctx, query, itemID).Scan(&eventID, &timestampStr, &actorUserID, &payloadStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEventNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query item.loaned event: %w", err)
	}

	// Parse payload to extract loaned_to
	var payload struct {
		LoanedTo string `json:"loaned_to"`
	}
	if unmarshalErr := json.Unmarshal([]byte(payloadStr), &payload); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal loaned event payload: %w", unmarshalErr)
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return &LoanedInfo{
		LoanedTo:    payload.LoanedTo,
		LoanedAt:    timestamp,
		EventID:     eventID,
		ActorUserID: actorUserID,
	}, nil
}
