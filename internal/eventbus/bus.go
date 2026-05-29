package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// Bus is the single entry point for persisting events and updating projections.
type Bus struct {
	store *store.Store
}

// New creates a new Bus backed by the given Store.
func New(s *store.Store) *Bus {
	return &Bus{store: s}
}

// Dispatch persists an event and applies it to projections in a single transaction.
func (b *Bus) Dispatch(
	ctx context.Context,
	eventType inventory.EventType,
	actorUserID string,
	payload json.RawMessage,
	note *string,
) (int64, error) {
	var entityID *string
	var m map[string]any
	if json.Unmarshal(payload, &m) == nil {
		if id, ok := m["entity_id"].(string); ok && id != "" {
			entityID = &id
		}
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)

	var eventID int64
	err := b.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		const q = `
            INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
            VALUES (?, ?, ?, ?, ?, ?)`
		result, err := tx.ExecContext(ctx, q, eventType, timestamp, actorUserID, string(payload), note, entityID)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get event ID: %w", err)
		}
		eventID = id

		ev := &inventory.Event{
			EventID:      eventID,
			EventType:    eventType,
			TimestampUTC: timestamp,
			ActorUserID:  actorUserID,
			Payload:      payload,
			Note:         note,
			EntityID:     entityID,
		}

		return b.applyEventTx(ctx, tx, ev)
	})
	if err != nil {
		return 0, err
	}
	return eventID, nil
}

// ReplayEvent inserts a fully-populated event using its original TimestampUTC.
// EntityPathChangedEvent is a no-op: it is skipped without error and returns 0.
// All other event types go through applyEventTx as normal.
func (b *Bus) ReplayEvent(ctx context.Context, ev *inventory.Event) (int64, error) {
	if ev.EventType == inventory.EntityPathChangedEvent {
		return 0, nil
	}

	var eventID int64
	err := b.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		const q = `
            INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
            VALUES (?, ?, ?, ?, ?, ?)`
		result, err := tx.ExecContext(
			ctx,
			q,
			ev.EventType,
			ev.TimestampUTC,
			ev.ActorUserID,
			string(ev.Payload),
			ev.Note,
			ev.EntityID,
		)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get event ID: %w", err)
		}
		eventID = id

		inserted := &inventory.Event{
			EventID:      eventID,
			EventType:    ev.EventType,
			TimestampUTC: ev.TimestampUTC,
			ActorUserID:  ev.ActorUserID,
			Payload:      ev.Payload,
			Note:         ev.Note,
			EntityID:     ev.EntityID,
		}
		return b.applyEventTx(ctx, tx, inserted)
	})
	if err != nil {
		return 0, err
	}
	return eventID, nil
}

func (b *Bus) applyEventTx(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	switch ev.EventType {
	case inventory.EntityCreatedEvent:
		return b.handleEntityCreated(ctx, tx, ev)
	case inventory.EntityRenamedEvent:
		return b.handleEntityRenamed(ctx, tx, ev)
	case inventory.EntityReparentedEvent:
		return b.handleEntityReparented(ctx, tx, ev)
	case inventory.EntityPathChangedEvent:
		return b.handleEntityPathChanged(ctx, tx, ev)
	case inventory.EntityStatusChangedEvent:
		return b.handleEntityStatusChanged(ctx, tx, ev)
	case inventory.EntityRemovedEvent:
		return b.handleEntityRemoved(ctx, tx, ev)
	default:
		return fmt.Errorf("unknown event type: %s", ev.EventType)
	}
}
