package app

import (
	"context"
	"encoding/json"
	"fmt"

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
	ReplayEvent(ctx context.Context, ev *inventory.Event) (int64, error)
}

type importStore interface {
	HasEvents(ctx context.Context) (bool, error)
	ClearAllData(ctx context.Context) error
	GetEventsAfter(ctx context.Context, afterID int64) ([]*inventory.Event, error)
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

	r := &importRunner{bus: bus, store: store, log: logging.GetLogger(), opts: opts, pendingReparentID: -1}
	for _, rec := range events {
		if err := r.processRecord(ctx, rec); err != nil {
			return r.result, err
		}
	}
	r.flushValidation(ctx)
	return r.result, nil
}

type importRunner struct {
	bus               importBus
	store             importStore
	log               logging.Logger
	opts              ImportOptions
	result            ImportResult
	pendingReparentID int64
	pathChangedBuf    []ExportResult
}

func (r *importRunner) processRecord(ctx context.Context, rec ExportResult) error {
	et, err := inventory.ParseEventType(rec.EventType)
	if err != nil {
		return r.handleError(rec.EventID, err)
	}

	if et == inventory.EntityPathChangedEvent {
		if r.pendingReparentID < 0 {
			orphan := fmt.Errorf("orphaned EntityPathChangedEvent at event_id %d (no preceding reparent)", rec.EventID)
			r.log.Warn(orphan.Error())
			r.warn(orphan)
		} else {
			r.pathChangedBuf = append(r.pathChangedBuf, rec)
		}
		return nil
	}

	r.flushValidation(ctx)

	ev := &inventory.Event{
		EventType:    et,
		TimestampUTC: rec.TimestampUTC,
		ActorUserID:  rec.ActorUserID,
		Payload:      rec.Payload,
		Note:         rec.Note,
		EntityID:     rec.EntityID,
	}

	newID, replayErr := r.bus.ReplayEvent(ctx, ev)
	if replayErr != nil {
		return r.handleError(rec.EventID, replayErr)
	}
	r.result.Replayed++

	if et == inventory.EntityReparentedEvent {
		r.pendingReparentID = newID
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

func (r *importRunner) flushValidation(ctx context.Context) {
	if r.pendingReparentID < 0 {
		return
	}
	generated, err := r.store.GetEventsAfter(ctx, r.pendingReparentID)
	if err != nil {
		wrapped := fmt.Errorf(
			"could not retrieve generated path-changed events after reparent event_id %d: %w",
			r.pendingReparentID,
			err,
		)
		r.log.Warn(wrapped.Error())
		r.warn(wrapped)
		r.pendingReparentID = -1
		r.pathChangedBuf = nil
		return
	}
	var genPC []*inventory.Event
	for _, ev := range generated {
		if ev.EventType == inventory.EntityPathChangedEvent {
			genPC = append(genPC, ev)
		} else {
			break
		}
	}
	if len(genPC) != len(r.pathChangedBuf) {
		mismatch := fmt.Errorf("path-changed count mismatch after reparent event_id %d (expected %d, got %d)",
			r.pendingReparentID, len(r.pathChangedBuf), len(genPC))
		r.log.Warn(mismatch.Error())
		r.warn(mismatch)
	} else {
		for i, buf := range r.pathChangedBuf {
			if string(genPC[i].Payload) != string(buf.Payload) {
				mismatch := fmt.Errorf("path-changed payload mismatch after reparent event_id %d at index %d",
					r.pendingReparentID, i)
				r.log.Warn(mismatch.Error())
				r.warn(mismatch)
				break
			}
		}
	}
	r.pendingReparentID = -1
	r.pathChangedBuf = nil
}
