package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

type eventHandlerFn func(context.Context, store.Tx, *inventory.Event) error

type eventRegistration struct {
	payloadFactory func() any
	applyFn        eventHandlerFn
	replayFn       eventHandlerFn // nil falls back to applyFn
}

// Bus is the single entry point for persisting events and updating projections.
type Bus struct {
	store    *store.Store
	registry map[inventory.EventType]eventRegistration
}

// New creates a new Bus backed by the given Store.
func New(s *store.Store) *Bus {
	b := &Bus{
		store:    s,
		registry: make(map[inventory.EventType]eventRegistration),
	}
	b.registerAll()
	return b
}

func (b *Bus) registerAll() {
	b.registry[inventory.EntityCreatedEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityCreatedPayload{} },
		applyFn:        b.handleEntityCreated,
	}
	b.registry[inventory.EntityRenamedEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityRenamedPayload{} },
		applyFn:        b.handleEntityRenamed,
	}
	b.registry[inventory.EntityReparentedEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityReparentedPayload{} },
		applyFn:        b.handleEntityReparented,
		replayFn:       b.handleEntityReparentedProjectionOnlyTx,
	}
	b.registry[inventory.EntityPathChangedEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityPathChangedPayload{} },
		applyFn:        b.handleEntityPathChanged,
	}
	b.registry[inventory.EntityStatusChangedEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityStatusChangedPayload{} },
		applyFn:        b.handleEntityStatusChanged,
	}
	b.registry[inventory.EntityRemovedEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityRemovedPayload{} },
		applyFn:        b.handleEntityRemoved,
	}
	b.registry[inventory.EntityTagAddedEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityTagAddedPayload{} },
		applyFn:        b.handleEntityTagAdded,
	}
	b.registry[inventory.EntityTagRemovedEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityTagRemovedPayload{} },
		applyFn:        b.handleEntityTagRemoved,
	}
	b.registry[inventory.EntityLockedEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityLockedPayload{} },
		applyFn:        b.handleEntityLocked,
	}
	b.registry[inventory.EntityUnlockedEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityUnlockedPayload{} },
		applyFn:        b.handleEntityUnlocked,
	}
	b.registry[inventory.EntityDiscreteSetEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityDiscreteSetPayload{} },
		applyFn:        b.handleEntityDiscreteSet,
	}
	b.registry[inventory.EntityDiscreteClearedEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityDiscreteClearedPayload{} },
		applyFn:        b.handleEntityDiscreteCleared,
	}
	b.registry[inventory.EntityBorrowedEvent] = eventRegistration{
		payloadFactory: func() any { return &EntityBorrowedPayload{} },
		applyFn:        b.handleEntityBorrowed,
	}
}

// PayloadFactories returns the live registry map of payload factory functions,
// keyed by EventType. Used by doctor validation to unmarshal payloads without
// maintaining a duplicate map.
func (b *Bus) PayloadFactories() map[inventory.EventType]func() any {
	factories := make(map[inventory.EventType]func() any, len(b.registry))
	for et, reg := range b.registry {
		factories[et] = reg.payloadFactory
	}
	return factories
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
	return b.writeEvent(ctx, b.buildEvent(eventType, actorUserID, payload, note), b.applyEventTx)
}

func (b *Bus) buildEvent(
	eventType inventory.EventType,
	actorUserID string,
	payload json.RawMessage,
	note *string,
) *inventory.Event {
	var entityID *string
	var m map[string]any
	if json.Unmarshal(payload, &m) == nil {
		if id, ok := m["entity_id"].(string); ok && id != "" {
			entityID = &id
		}
	}
	return &inventory.Event{
		EventType:    eventType,
		TimestampUTC: time.Now().UTC().Format(time.RFC3339),
		ActorUserID:  actorUserID,
		Payload:      payload,
		Note:         note,
		EntityID:     entityID,
	}
}

// DispatchInTx writes and applies a single event within a caller-owned transaction.
func (b *Bus) DispatchInTx(
	ctx context.Context,
	tx store.Tx,
	eventType inventory.EventType,
	actorUserID string,
	payload json.RawMessage,
	note *string,
) (int64, error) {
	ev := b.buildEvent(eventType, actorUserID, payload, note)
	return b.writeEventInTx(ctx, tx, ev, b.applyEventTx)
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
func (b *Bus) ReplayEvent(ctx context.Context, ev *inventory.Event) (int64, []EntityPathChangedPayload, error) {
	if ev.EventType == inventory.EntityPathChangedEvent {
		return 0, nil, nil
	}
	var payloads []EntityPathChangedPayload
	id, err := b.writeEvent(ctx, ev, func(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
		if ev.EventType == inventory.EntityReparentedEvent {
			var applyErr error
			payloads, applyErr = b.handleEntityReparentedComputePayloadsTx(ctx, tx, ev)
			return applyErr
		}
		return b.applyEventProjectionOnlyTx(ctx, tx, ev)
	})
	return id, payloads, err
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
		var err error
		eventID, err = b.writeEventInTx(ctx, tx, ev, applyFn)
		return err
	})
	if err != nil {
		return 0, err
	}
	return eventID, nil
}

func (b *Bus) writeEventInTx(
	ctx context.Context,
	tx store.Tx,
	ev *inventory.Event,
	applyFn func(context.Context, store.Tx, *inventory.Event) error,
) (int64, error) {
	const q = `
            INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
            VALUES (?, ?, ?, ?, ?, ?)`
	result, err := tx.ExecContext(ctx, q,
		ev.EventType, ev.TimestampUTC, ev.ActorUserID,
		string(ev.Payload), ev.Note, ev.EntityID)
	if err != nil {
		return 0, fmt.Errorf("insert event: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get event ID: %w", err)
	}
	inserted := *ev
	inserted.EventID = id
	return id, applyFn(ctx, tx, &inserted)
}

func (b *Bus) applyEventTx(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	reg, ok := b.registry[ev.EventType]
	if !ok {
		return fmt.Errorf("unknown event type: %s", ev.EventType)
	}
	return reg.applyFn(ctx, tx, ev)
}

// applyEventProjectionOnlyTx is like applyEventTx but never writes new events.
// EntityReparentedEvent uses handleEntityReparentedProjectionOnlyTx (skips
// propagatePathChangesTx); EntityPathChangedEvent is applied normally via
// handleEntityPathChanged. Used by ReplayEvent (import) and TruncateAndReplay
// (projection rebuild) so neither path grows the event log.
func (b *Bus) applyEventProjectionOnlyTx(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	reg, ok := b.registry[ev.EventType]
	if !ok {
		return fmt.Errorf("unknown event type: %s", ev.EventType)
	}
	fn := reg.applyFn
	if reg.replayFn != nil {
		fn = reg.replayFn
	}
	return fn(ctx, tx, ev)
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
		if truncErr := b.store.TruncateTagsTx(ctx, tx); truncErr != nil {
			return fmt.Errorf("truncate tags: %w", truncErr)
		}
		if truncErr := b.store.TruncateEntitiesTx(ctx, tx); truncErr != nil {
			return fmt.Errorf("truncate entities: %w", truncErr)
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
