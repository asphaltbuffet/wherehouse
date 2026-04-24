package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/goccy/go-json"
)

// Event represents a single event from the event log.
type Event struct {
	EventID      int64
	EventType    EventType
	TimestampUTC string
	ActorUserID  string
	Payload      json.RawMessage
	Note         *string
	EntityID     *string
}

// AppendEvent creates a new event in the event log and immediately applies it to
// the projections within a single atomic transaction.
//
// This is the primary method for recording new domain events from command code.
// Use insertEvent directly only for replay/seed scenarios where you want to
// batch-insert events before processing them with ProcessEvent.
func (d *Database) AppendEvent(
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

	timestamp := time.Now().UTC().Format(time.RFC3339)

	var notePtr *string
	if note != "" {
		notePtr = &note
	}

	var eventID int64

	err = d.ExecInTransaction(ctx, func(tx *sql.Tx) error {
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

		result, txErr := tx.ExecContext(ctx, query,
			eventType,
			timestamp,
			actorUserID,
			string(payloadJSON),
			notePtr,
			entityID,
		)
		if txErr != nil {
			return fmt.Errorf("failed to insert event: %w", txErr)
		}

		var idErr error
		eventID, idErr = result.LastInsertId()
		if idErr != nil {
			return fmt.Errorf("failed to get event ID: %w", idErr)
		}

		// Build Event struct for projection processing
		event := &Event{
			EventID:      eventID,
			EventType:    eventType,
			TimestampUTC: timestamp,
			ActorUserID:  actorUserID,
			Payload:      json.RawMessage(payloadJSON),
			Note:         notePtr,
			EntityID:     entityID,
		}

		// Apply event to projections within the same transaction
		return d.processEventInTx(ctx, tx, event)
	})
	if err != nil {
		return 0, err
	}

	return eventID, nil
}

// GetEventByID retrieves a single event by its event_id.
func (d *Database) GetEventByID(ctx context.Context, eventID int64) (*Event, error) {
	const query = `
		SELECT
			event_id,
			event_type,
			timestamp_utc,
			actor_user_id,
			payload,
			note,
			entity_id
		FROM events
		WHERE event_id = ?
	`

	var event Event
	var payloadStr string

	err := d.db.QueryRowContext(ctx, query, eventID).Scan(
		&event.EventID,
		&event.EventType,
		&event.TimestampUTC,
		&event.ActorUserID,
		&payloadStr,
		&event.Note,
		&event.EntityID,
	)
	if err == sql.ErrNoRows {
		return nil, ErrEventNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	event.Payload = json.RawMessage(payloadStr)
	return &event, nil
}

// GetEventsByType retrieves all events of a specific type, ordered by event_id.
func (d *Database) GetEventsByType(ctx context.Context, eventType EventType) ([]*Event, error) {
	const query = `
		SELECT
			event_id,
			event_type,
			timestamp_utc,
			actor_user_id,
			payload,
			note,
			entity_id
		FROM events
		WHERE event_type = ?
		ORDER BY event_id ASC
	`

	rows, err := d.db.QueryContext(ctx, query, eventType)
	if err != nil {
		return nil, fmt.Errorf("failed to query events by type: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// GetEventsByEntity retrieves all events for a specific entity by entity_id.
func (d *Database) GetEventsByEntity(ctx context.Context, entityID string) ([]*Event, error) {
	const query = `
		SELECT
			event_id,
			event_type,
			timestamp_utc,
			actor_user_id,
			payload,
			note,
			entity_id
		FROM events
		WHERE entity_id = ?
		ORDER BY event_id ASC
	`

	rows, err := d.db.QueryContext(ctx, query, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to query events by entity: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// GetAllEvents retrieves all events ordered by event_id (for replay).
func (d *Database) GetAllEvents(ctx context.Context) ([]*Event, error) {
	const query = `
		SELECT
			event_id,
			event_type,
			timestamp_utc,
			actor_user_id,
			payload,
			note,
			entity_id
		FROM events
		ORDER BY event_id ASC
	`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all events: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// GetEventsAfter retrieves all events after a specific event_id (for incremental replay).
func (d *Database) GetEventsAfter(ctx context.Context, afterEventID int64) ([]*Event, error) {
	const query = `
		SELECT
			event_id,
			event_type,
			timestamp_utc,
			actor_user_id,
			payload,
			note,
			entity_id
		FROM events
		WHERE event_id > ?
		ORDER BY event_id ASC
	`

	rows, err := d.db.QueryContext(ctx, query, afterEventID)
	if err != nil {
		return nil, fmt.Errorf("failed to query events after ID: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// scanEvents is a helper function to scan multiple events from query rows.
func scanEvents(rows *sql.Rows) ([]*Event, error) {
	var events []*Event

	for rows.Next() {
		var event Event
		var payloadStr string

		err := rows.Scan(
			&event.EventID,
			&event.EventType,
			&event.TimestampUTC,
			&event.ActorUserID,
			&payloadStr,
			&event.Note,
			&event.EntityID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		event.Payload = json.RawMessage(payloadStr)
		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}

	return events, nil
}
