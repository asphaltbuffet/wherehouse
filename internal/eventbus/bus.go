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

// pathChangeFn applies a path-propagating event to its own entity and returns
// that entity with the descendants whose paths follow it, as they were before
// the change. The bus decides whether to write EntityPathChangedEvents for them.
type pathChangeFn func(context.Context, store.Tx, *inventory.Event) (*inventory.Entity, []*inventory.Entity, error)

// eventRegistration sets exactly one of applyFn or pathChangeFn.
type eventRegistration struct {
	decode       func(json.RawMessage) (Payload, error)
	applyFn      eventHandlerFn
	pathChangeFn pathChangeFn // set for events where PropagatesPaths() is true
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
	register(b, b.handleEntityCreated)
	registerPathChange(b, b.renameEntityTx)
	registerRelocation[EntityReparentedPayload](b)
	register(b, b.handleEntityPathChanged)
	register(b, b.handleEntityStatusChanged)
	register(b, b.handleEntityRemoved)
	register(b, b.handleEntityTagAdded)
	register(b, b.handleEntityTagRemoved)
	register(b, b.handleEntityLocked)
	register(b, b.handleEntityUnlocked)
	register(b, b.handleEntityDiscreteSet)
	register(b, b.handleEntityDiscreteCleared)
	register(b, b.handleEntityBorrowed)
	registerRelocation[EntityUsedPayload](b)
	registerRelocation[EntityReturnedHomePayload](b)
	register(b, b.handleEntityRehomed)
}

// DecodePayload decodes raw as the payload registered for et. Doctor validates
// stored events with it, against the same types the handlers read.
func (b *Bus) DecodePayload(et inventory.EventType, raw json.RawMessage) (Payload, error) {
	reg, ok := b.registry[et]
	if !ok {
		return nil, fmt.Errorf("unknown event type: %s", et)
	}
	return reg.decode(raw)
}

// Dispatch writes p as one event and applies it to projections inside tx,
// which the caller owns (ADR 0035). The event type and entity_id come from p
// (ADR 0042); an empty note is stored as NULL.
func (b *Bus) Dispatch(
	ctx context.Context,
	tx store.Tx,
	actorUserID string,
	p Payload,
	note string,
) (int64, error) {
	et := p.EventType()
	if et.IsDerived() {
		return 0, fmt.Errorf("dispatch %s: %w", et, ErrDerivedEvent)
	}
	subject := p.Subject()
	if subject == "" {
		return 0, fmt.Errorf("dispatch %s: %w", et, ErrNoSubject)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return 0, fmt.Errorf("dispatch %s: marshal: %w", et, err)
	}
	var notePtr *string
	if note != "" {
		notePtr = &note
	}
	ev := &inventory.Event{
		EventType:    et,
		TimestampUTC: time.Now().UTC().Format(time.RFC3339),
		ActorUserID:  actorUserID,
		Payload:      raw,
		Note:         notePtr,
		EntityID:     &subject,
	}
	return b.writeEventInTx(ctx, tx, ev, b.applyEventTx)
}

// ReplayEvent inserts a fully-populated event using its original TimestampUTC
// and EntityID (no payload reparse). A derived event is skipped without
// error and returns 0. Path-propagating events regenerate their descendants'
// EntityPathChangedEvents in the destination log, exactly as live dispatch
// does, so a later TruncateAndReplay finds them (ADR 0009). The written
// payloads are returned so import can validate the source stream's copies
// against them (ADR 0005). It rejects an event whose entity_id disagrees with
// its payload (ErrSubjectMismatch).
func (b *Bus) ReplayEvent(ctx context.Context, ev *inventory.Event) (int64, []EntityPathChangedPayload, error) {
	if ev.EventType.IsDerived() {
		return 0, nil, nil
	}
	if err := b.checkSubject(ev); err != nil {
		return 0, nil, err
	}
	var payloads []EntityPathChangedPayload
	id, err := b.writeEvent(ctx, ev, func(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
		var applyErr error
		payloads, applyErr = b.applyAndPropagateTx(ctx, tx, ev)
		return applyErr
	})
	return id, payloads, err
}

// checkSubject rejects an event whose entity_id column disagrees with its
// payload. Only import calls it; rebuild tolerates a mismatch (ADR 0042).
func (b *Bus) checkSubject(ev *inventory.Event) error {
	p, err := b.DecodePayload(ev.EventType, ev.Payload)
	if err != nil {
		return err
	}
	var column string
	if ev.EntityID != nil {
		column = *ev.EntityID
	}
	if ev.EntityID == nil || column != p.Subject() {
		return fmt.Errorf("%s: entity_id %q, payload entity_id %q: %w",
			ev.EventType, column, p.Subject(), ErrSubjectMismatch)
	}
	return nil
}

// writeEvent is the transaction-owning write path for ReplayEvent; Dispatch
// calls writeEventInTx inside the caller's transaction. It inserts ev into the events table inside a transaction,
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
	_, err := b.applyAndPropagateTx(ctx, tx, ev)
	return err
}

// applyAndPropagateTx applies ev and, for a path-propagating event, writes an
// EntityPathChangedEvent per descendant and returns their payloads. Used by
// live dispatch and by ReplayEvent (import).
func (b *Bus) applyAndPropagateTx(
	ctx context.Context, tx store.Tx, ev *inventory.Event,
) ([]EntityPathChangedPayload, error) {
	reg, ok := b.registry[ev.EventType]
	if !ok {
		return nil, fmt.Errorf("unknown event type: %s", ev.EventType)
	}
	if reg.pathChangeFn == nil {
		return nil, reg.applyFn(ctx, tx, ev)
	}
	entity, descendants, err := reg.pathChangeFn(ctx, tx, ev)
	if err != nil {
		return nil, err
	}
	payloads, err := b.propagatePathChangesTx(ctx, tx, ev, entity, descendants)
	if err != nil {
		return nil, fmt.Errorf("%s: propagate: %w", ev.EventType, err)
	}
	return payloads, nil
}

// applyEventProjectionOnlyTx is like applyEventTx but never writes new events.
// Path-propagating events update only their own entity; the descendants'
// EntityPathChangedEvents already in the log are applied normally via
// handleEntityPathChanged. Used by TruncateAndReplay so a rebuild never grows
// the event log.
func (b *Bus) applyEventProjectionOnlyTx(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	reg, ok := b.registry[ev.EventType]
	if !ok {
		return fmt.Errorf("unknown event type: %s", ev.EventType)
	}
	if reg.pathChangeFn == nil {
		return reg.applyFn(ctx, tx, ev)
	}
	_, _, err := reg.pathChangeFn(ctx, tx, ev)
	return err
}

// TruncateAndReplay rebuilds entities_current from the full event log.
// Events are loaded inside the write transaction, which BEGIN IMMEDIATE opens
// holding the write lock, so no concurrent event can commit between the load
// and the replay. Derived events are applied directly via applyEventProjectionOnlyTx
// (no new events are written). Returns the count of non-derived
// events applied; on any error the transaction rolls back and the projection is
// left intact.
func (b *Bus) TruncateAndReplay(ctx context.Context) (int, error) {
	var count int
	err := b.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		events, loadErr := b.store.GetAllEventsTx(ctx, tx)
		if loadErr != nil {
			return fmt.Errorf("TruncateAndReplay: load events: %w", loadErr)
		}
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
			if !ev.EventType.IsDerived() {
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
