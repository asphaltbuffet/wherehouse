package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// AppendRawEvent inserts a pre-constructed event into the events table.
// Does NOT apply projections — that is eventbus's responsibility.
func (s *Store) AppendRawEvent(
	ctx context.Context,
	eventType inventory.EventType,
	actorUserID string,
	payload json.RawMessage,
	note *string,
	entityID *string,
) (int64, error) {
	timestamp := time.Now().UTC().Format(time.RFC3339)

	var eventID int64
	err := s.ExecInTransaction(ctx, func(tx Tx) error {
		const query = `
			INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
			VALUES (?, ?, ?, ?, ?, ?)`
		result, err := tx.ExecContext(ctx, query, eventType, timestamp, actorUserID, string(payload), note, entityID)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get event ID: %w", err)
		}
		eventID = id
		return nil
	})
	if err != nil {
		return 0, err
	}
	return eventID, nil
}

// GetEventByID retrieves a single event by its ID.
func (s *Store) GetEventByID(ctx context.Context, eventID int64) (*inventory.Event, error) {
	const query = `
		SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, entity_id
		FROM events WHERE event_id = ?`
	row := s.db.QueryRowContext(ctx, query, eventID)
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
func (s *Store) GetEventsByEntity(ctx context.Context, entityID string) ([]*inventory.Event, error) {
	const query = `
		SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, entity_id
		FROM events WHERE entity_id = ? ORDER BY event_id ASC`
	rows, err := s.db.QueryContext(ctx, query, entityID)
	if err != nil {
		return nil, fmt.Errorf("query events for entity %s: %w", entityID, err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

// GetAllEvents retrieves all events ordered by event_id ASC.
func (s *Store) GetAllEvents(ctx context.Context) ([]*inventory.Event, error) {
	const query = `
		SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, entity_id
		FROM events ORDER BY event_id ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query all events: %w", err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

// GetEventsAfter retrieves events with event_id > afterID, ordered by event_id ASC.
func (s *Store) GetEventsAfter(ctx context.Context, afterID int64) ([]*inventory.Event, error) {
	const query = `
		SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, entity_id
		FROM events WHERE event_id > ? ORDER BY event_id ASC`
	rows, err := s.db.QueryContext(ctx, query, afterID)
	if err != nil {
		return nil, fmt.Errorf("query events after %d: %w", afterID, err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

// HasEvents reports whether the events table contains at least one row.
func (s *Store) HasEvents(ctx context.Context) (bool, error) {
	var exists bool
	const query = `SELECT EXISTS(SELECT 1 FROM events LIMIT 1)`
	if err := s.db.QueryRowContext(ctx, query).Scan(&exists); err != nil {
		return false, fmt.Errorf("has events: %w", err)
	}
	return exists, nil
}

// ClearAllData deletes all rows from entities_current then events, leaving schema_metadata intact.
func (s *Store) ClearAllData(ctx context.Context) error {
	return s.ExecInTransaction(ctx, func(tx Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM entities_current`); err != nil {
			return fmt.Errorf("clear entities: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM events`); err != nil {
			return fmt.Errorf("clear events: %w", err)
		}
		return nil
	})
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
