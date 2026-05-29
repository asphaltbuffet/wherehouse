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
// The timestamp is set to [time.Now].UTC() and entity_id is parsed from the payload.
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

	ev := &inventory.Event{
		EventType:    eventType,
		TimestampUTC: time.Now().UTC().Format(time.RFC3339),
		ActorUserID:  actorUserID,
		Payload:      payload,
		Note:         note,
		EntityID:     entityID,
	}
	return b.writeEvent(ctx, ev, b.applyEventTx)
}

// ReplayEvent inserts a fully-populated event using its original TimestampUTC
// and EntityID (no payload reparse). EntityPathChangedEvent is a no-op: it is
// skipped without error and returns 0, because EntityReparentedEvent's handler
// regenerates path-changed events itself.
// ReplayEvent inserts a fully-populated event using its original TimestampUTC
// and EntityID (no payload reparse). Uses applyEventProjectionOnlyTx so that
// EntityReparentedEvent does not regenerate EntityPathChangedEvents — those
// already exist in the import stream and are applied directly, keeping the
// event log from growing on every import.
// ReplayEvent inserts a fully-populated event using its original TimestampUTC
// and EntityID (no payload reparse). EntityPathChangedEvent is skipped without
// error and returns 0 — the import path omits these from replay and validates
// them separately (see ADR-0005). See also: the event-log-growth issue filed
// against this behaviour.
func (b *Bus) ReplayEvent(ctx context.Context, ev *inventory.Event) (int64, error) {
	if ev.EventType == inventory.EntityPathChangedEvent {
		return 0, nil
	}
	return b.writeEvent(ctx, ev, b.applyEventTx)
}

// writeEvent is the single canonical write path shared by Dispatch and
// ReplayEvent. It inserts ev into the events table inside a transaction,
// then runs applyEventTx in that same transaction so projection updates are
// atomic with the event row.
//
// Callers populate ev's EventType, TimestampUTC, ActorUserID, Payload, Note,
// and EntityID. EventID is assigned by SQLite and returned to the caller.
//
// If a future change needs to differ between Dispatch and ReplayEvent (e.g.
// one path adds a validation step the other shouldn't), express it as a
// parameter to writeEvent rather than re-splitting into two methods.
func (b *Bus) writeEvent(
	ctx context.Context,
	ev *inventory.Event,
	applyFn func(context.Context, store.Tx, *inventory.Event) error,
) (int64, error) {
	var eventID int64
	err := b.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		const q = `
            INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
            VALUES (?, ?, ?, ?, ?, ?)`
		result, err := tx.ExecContext(ctx, q,
			ev.EventType, ev.TimestampUTC, ev.ActorUserID,
			string(ev.Payload), ev.Note, ev.EntityID)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get event ID: %w", err)
		}
		eventID = id

		inserted := *ev
		inserted.EventID = eventID
		return applyFn(ctx, tx, &inserted)
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

// applyEventProjectionOnlyTx is like applyEventTx but never writes new events.
// EntityReparentedEvent uses handleEntityReparentedProjectionOnlyTx (skips
// propagatePathChangesTx); EntityPathChangedEvent is applied normally via
// handleEntityPathChanged. Used by ReplayEvent (import) and TruncateAndReplay
// (projection rebuild) so neither path grows the event log.
func (b *Bus) applyEventProjectionOnlyTx(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	switch ev.EventType {
	case inventory.EntityCreatedEvent:
		return b.handleEntityCreated(ctx, tx, ev)
	case inventory.EntityRenamedEvent:
		return b.handleEntityRenamed(ctx, tx, ev)
	case inventory.EntityReparentedEvent:
		return b.handleEntityReparentedProjectionOnlyTx(ctx, tx, ev)
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

// TruncateAndReplay rebuilds entities_current from the full event log.
// Events are loaded before the write transaction opens (read-then-write) to
// minimise the window during which a concurrent write could be missed.
// EntityPathChangedEvent rows are applied directly via applyEventProjectionOnlyTx
// (no new events are written). Returns the count of non-EntityPathChangedEvent
// events applied; on any error the transaction rolls back and the projection is
// left intact.
func (b *Bus) TruncateAndReplay(ctx context.Context) (int, error) {
	events, err := b.store.GetAllEvents(ctx)
	if err != nil {
		return 0, fmt.Errorf("TruncateAndReplay: load events: %w", err)
	}

	var count int
	err = b.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		if truncErr := b.store.TruncateEntitiesTx(ctx, tx); truncErr != nil {
			return fmt.Errorf("truncate: %w", truncErr)
		}
		for _, ev := range events {
			if applyErr := b.applyEventProjectionOnlyTx(ctx, tx, ev); applyErr != nil {
				return fmt.Errorf("replay event_id %d: %w", ev.EventID, applyErr)
			}
			if ev.EventType != inventory.EntityPathChangedEvent {
				count++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}
