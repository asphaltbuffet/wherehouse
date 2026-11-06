package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// AppendEvent inserts ev into the events table and returns its
// event_id. It is the only writer to events: the bus calls it for dispatched,
// replayed and derived events. Fields are stored as given (the caller sets
// TimestampUTC); ev.EventID is ignored. It does not apply projections — that
// is eventbus's responsibility.
func (t Tx) AppendEvent(ctx context.Context, ev *inventory.Event) (int64, error) {
	const query = `
		INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
		VALUES (?, ?, ?, ?, ?, ?)`
	result, err := t.q.ExecContext(ctx, query,
		ev.EventType, ev.TimestampUTC, ev.ActorUserID, string(ev.Payload), ev.Note, ev.EntityID)
	if err != nil {
		return 0, fmt.Errorf("insert event: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get event ID: %w", err)
	}
	return id, nil
}

// GetEventByID retrieves a single event by its ID.
func (r Reader) GetEventByID(ctx context.Context, eventID int64) (*inventory.Event, error) {
	const query = `
		SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, entity_id
		FROM events WHERE event_id = ?`
	row := r.q.QueryRowContext(ctx, query, eventID)
	ev, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get event %d: %w", eventID, err)
	}
	return ev, nil
}

// GetEventsByEntity retrieves all events for a given entity ID, ordered by event_id ASC.
func (r Reader) GetEventsByEntity(ctx context.Context, entityID string) ([]*inventory.Event, error) {
	const query = `
		SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, entity_id
		FROM events WHERE entity_id = ? ORDER BY event_id ASC`
	rows, err := r.q.QueryContext(ctx, query, entityID)
	if err != nil {
		return nil, fmt.Errorf("query events for entity %s: %w", entityID, err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

const allEventsQuery = `
	SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, entity_id
	FROM events ORDER BY event_id ASC`

// GetAllEvents retrieves all events ordered by event_id ASC. Called on a Tx,
// a caller holding the write lock sees exactly the log it is about to act on.
func (r Reader) GetAllEvents(ctx context.Context) ([]*inventory.Event, error) {
	rows, err := r.q.QueryContext(ctx, allEventsQuery)
	if err != nil {
		return nil, fmt.Errorf("query all events: %w", err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

// HasEvents reports whether the events table contains at least one row.
func (r Reader) HasEvents(ctx context.Context) (bool, error) {
	var exists bool
	const query = `SELECT EXISTS(SELECT 1 FROM events LIMIT 1)`
	if err := r.q.QueryRowContext(ctx, query).Scan(&exists); err != nil {
		return false, fmt.Errorf("has events: %w", err)
	}
	return exists, nil
}

// ClearAllData deletes all rows from the projection tables (entity_tags,
// entities_current) and then events, leaving schema_metadata intact.
//
// INVARIANT: every projection table (see CONTEXT.md "Projection") is cleared
// here. A new projection table must be added to this method.
func (t Tx) ClearAllData(ctx context.Context) error {
	// entity_tags references entities_current, so it is cleared first.
	if _, err := t.q.ExecContext(ctx, `DELETE FROM entity_tags`); err != nil {
		return fmt.Errorf("clear tags: %w", err)
	}
	if _, err := t.q.ExecContext(ctx, `DELETE FROM entities_current`); err != nil {
		return fmt.Errorf("clear entities: %w", err)
	}
	if _, err := t.q.ExecContext(ctx, `DELETE FROM events`); err != nil {
		return fmt.Errorf("clear events: %w", err)
	}
	return nil
}

// RawEvent is an unvalidated event row — EventType is a plain string so callers
// can detect unknown types without triggering a scan error.
type RawEvent struct {
	EventID   int64
	EventType string
	Payload   json.RawMessage
	EntityID  *string
}

// GetAllEventsRaw returns all events ordered by event_id ASC with EventType as a raw string.
func (r Reader) GetAllEventsRaw(ctx context.Context) ([]RawEvent, error) {
	const query = `
		SELECT event_id, event_type, payload, entity_id
		FROM events ORDER BY event_id ASC`
	rows, err := r.q.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query all events raw: %w", err)
	}
	defer rows.Close()

	var events []RawEvent
	for rows.Next() {
		var ev RawEvent
		var payloadStr string
		if scanErr := rows.Scan(&ev.EventID, &ev.EventType, &payloadStr, &ev.EntityID); scanErr != nil {
			return nil, fmt.Errorf("scan raw event: %w", scanErr)
		}
		ev.Payload = json.RawMessage(payloadStr)
		events = append(events, ev)
	}
	return events, rows.Err()
}

func scanEvent(row *sql.Row) (*inventory.Event, error) {
	var ev inventory.Event
	var payloadStr string
	err := row.Scan(
		&ev.EventID,
		&ev.EventType,
		&ev.TimestampUTC,
		&ev.ActorUserID,
		&payloadStr,
		&ev.Note,
		&ev.EntityID,
	)
	if err != nil {
		return nil, err
	}
	ev.Payload = json.RawMessage(payloadStr)
	return &ev, nil
}

func scanEvents(rows *sql.Rows) ([]*inventory.Event, error) {
	var events []*inventory.Event
	for rows.Next() {
		var ev inventory.Event
		var payloadStr string
		if err := rows.Scan(
			&ev.EventID,
			&ev.EventType,
			&ev.TimestampUTC,
			&ev.ActorUserID,
			&payloadStr,
			&ev.Note,
			&ev.EntityID,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		ev.Payload = json.RawMessage(payloadStr)
		events = append(events, &ev)
	}
	return events, rows.Err()
}
