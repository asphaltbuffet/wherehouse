package app

import (
	"context"
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
type ImportResult struct {
	Replayed int
	Failed   int
	Warnings int
	Errors   []error
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
			r.log.Warn("import: orphaned EntityPathChangedEvent (no preceding reparent)")
			r.result.Warnings++
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
		r.log.Warn("import: could not retrieve generated path-changed events", "error", err)
		r.result.Warnings++
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
		r.log.Warn("import: path-changed count mismatch after reparent",
			"expected", len(r.pathChangedBuf), "got", len(genPC))
		r.result.Warnings++
	} else {
		for i, buf := range r.pathChangedBuf {
			if string(genPC[i].Payload) != string(buf.Payload) {
				r.log.Warn("import: path-changed payload mismatch", "index", i)
				r.result.Warnings++
				break
			}
		}
	}
	r.pendingReparentID = -1
	r.pathChangedBuf = nil
}
