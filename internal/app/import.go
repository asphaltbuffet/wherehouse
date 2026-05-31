package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/logging"
)

// ImportOptions controls the behaviour of ImportEvents.
type ImportOptions struct {
	Replace  bool // clear all data before replay (caller must have confirmed)
	Continue bool // accumulate per-event errors instead of aborting
}

// ImportResult summarises the outcome of an import run.
//
// WarningCount is the number of non-fatal anomalies detected during replay
// (e.g. orphaned EntityPathChangedEvent records, mismatched path-changed
// payloads). Warnings holds one descriptive error per increment, in
// detection order, so callers can render diagnostics rather than only a
// count. len(Warnings) == WarningCount as an invariant.
type ImportResult struct {
	Replayed     int
	Failed       int
	WarningCount int
	Warnings     []error
	Errors       []error
}

type importBus interface {
	ReplayEvent(ctx context.Context, ev *inventory.Event) (int64, []eventbus.EntityPathChangedPayload, error)
}

type importStore interface {
	HasEvents(ctx context.Context) (bool, error)
	ClearAllData(ctx context.Context) error
}

// ImportEvents replays a slice of ExportResult records into the database.
func (a *App) ImportEvents(ctx context.Context, events []ExportResult, opts ImportOptions) (ImportResult, error) {
	return importEvents(ctx, a.bus, a.store, events, opts)
}

// HasEvents reports whether the database contains any events.
func (a *App) HasEvents(ctx context.Context) (bool, error) {
	return a.store.HasEvents(ctx)
}

// ClearAllData removes all events and entity projections from the database.
func (a *App) ClearAllData(ctx context.Context) error {
	return a.store.ClearAllData(ctx)
}

func importEvents(
	ctx context.Context,
	bus importBus,
	store importStore,
	events []ExportResult,
	opts ImportOptions,
) (ImportResult, error) {
	for i := 1; i < len(events); i++ {
		if events[i].EventID <= events[i-1].EventID {
			return ImportResult{}, fmt.Errorf(
				"import: event_id order is non-monotonic at index %d (id %d after %d); re-export from source",
				i,
				events[i].EventID,
				events[i-1].EventID,
			)
		}
	}

	// Pre-replay validation (Level 2): every record must have a known EventType
	// and a payload that parses as a JSON object. This runs before opts.Replace
	// so a malformed input file cannot leave the database empty after ClearAllData
	// followed by a mid-replay failure. Per-event-type schema validation is
	// tracked as deferred work (see follow-up issue referenced in CHANGELOG).
	for i, rec := range events {
		if _, err := inventory.ParseEventType(rec.EventType); err != nil {
			return ImportResult{}, fmt.Errorf(
				"import: validation failed at index %d (event_id %d): %w",
				i,
				rec.EventID,
				err,
			)
		}
		var payloadShape map[string]any
		if err := json.Unmarshal(rec.Payload, &payloadShape); err != nil {
			return ImportResult{}, fmt.Errorf(
				"import: validation failed at index %d (event_id %d): payload is not a JSON object: %w",
				i,
				rec.EventID,
				err,
			)
		}
	}

	if opts.Replace {
		if err := store.ClearAllData(ctx); err != nil {
			return ImportResult{}, fmt.Errorf("import: clear data: %w", err)
		}
	}

	r := &importRunner{bus: bus, store: store, log: logging.GetLogger(), opts: opts}
	for _, rec := range events {
		if err := r.processRecord(ctx, rec); err != nil {
			return r.result, err
		}
	}
	r.flushValidation()
	return r.result, nil
}

type importRunner struct {
	bus    importBus
	store  importStore
	log    logging.Logger
	opts   ImportOptions
	result ImportResult

	// pendingReparent is set when an EntityReparentedEvent has been replayed
	// but its buffered EntityPathChangedEvent records have not yet been
	// validated. Nil means no reparent is in flight.
	pendingReparent *pendingReparentState
	pathChangedBuf  []ExportResult
}

type pendingReparentState struct {
	eventID  int64
	entityID *string
	payloads []eventbus.EntityPathChangedPayload
}

func (r *importRunner) processRecord(ctx context.Context, rec ExportResult) error {
	et, err := inventory.ParseEventType(rec.EventType)
	if err != nil {
		return r.handleError(rec.EventID, err)
	}

	if et == inventory.EntityPathChangedEvent {
		if r.pendingReparent == nil {
			orphan := fmt.Errorf("orphaned EntityPathChangedEvent at event_id %d (no preceding reparent)", rec.EventID)
			r.log.Warn(orphan.Error())
			r.warn(orphan)
		} else {
			r.pathChangedBuf = append(r.pathChangedBuf, rec)
		}
		return nil
	}

	r.flushValidation()

	ev := &inventory.Event{
		EventType:    et,
		TimestampUTC: rec.TimestampUTC,
		ActorUserID:  rec.ActorUserID,
		Payload:      rec.Payload,
		Note:         rec.Note,
		EntityID:     rec.EntityID,
	}

	newID, payloads, replayErr := r.bus.ReplayEvent(ctx, ev)
	if replayErr != nil {
		return r.handleError(rec.EventID, replayErr)
	}
	r.result.Replayed++

	if et == inventory.EntityReparentedEvent {
		r.pendingReparent = &pendingReparentState{
			eventID:  newID,
			entityID: rec.EntityID,
			payloads: payloads,
		}
	}
	return nil
}

// warn records a non-fatal anomaly. It bumps the counter and appends a
// descriptive error so callers see both "how many" and "which ones.".
func (r *importRunner) warn(err error) {
	r.result.WarningCount++
	r.result.Warnings = append(r.result.Warnings, err)
}

func (r *importRunner) handleError(eventID int64, err error) error {
	wrapped := fmt.Errorf("event id %d: %w", eventID, err)
	if r.opts.Continue {
		r.result.Failed++
		r.result.Errors = append(r.result.Errors, wrapped)
		return nil
	}
	return wrapped
}

func (r *importRunner) flushValidation() {
	if r.pendingReparent == nil {
		return
	}
	pr := r.pendingReparent
	r.pendingReparent = nil

	computed := pr.payloads
	buf := r.pathChangedBuf
	r.pathChangedBuf = nil

	if len(computed) != len(buf) {
		mismatch := fmt.Errorf("path-changed count mismatch after reparent event_id %d (expected %d, got %d)",
			pr.eventID, len(buf), len(computed))
		r.log.Warn(mismatch.Error())
		r.warn(mismatch)
		return
	}

	for i, bufRec := range buf {
		var imported eventbus.EntityPathChangedPayload
		if err := json.Unmarshal(bufRec.Payload, &imported); err != nil {
			mismatch := fmt.Errorf("path-changed payload parse error after reparent event_id %d at index %d: %w",
				pr.eventID, i, err)
			r.log.Warn(mismatch.Error())
			r.warn(mismatch)
			break
		}
		c := computed[i]
		if imported.EntityID != c.EntityID ||
			imported.FullPathDisplay != c.FullPathDisplay ||
			imported.FullPathCanonical != c.FullPathCanonical ||
			imported.Depth != c.Depth {
			mismatch := fmt.Errorf("path-changed payload mismatch after reparent event_id %d at index %d",
				pr.eventID, i)
			r.log.Warn(mismatch.Error())
			r.warn(mismatch)
			break
		}
	}
}
