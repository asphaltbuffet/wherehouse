package app

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// DoctorIssue describes a single structural problem found during a doctor validation.
type DoctorIssue struct {
	Kind        DoctorKind
	EventID     *int64
	Description string
}

// ValidateEventLog reads all events and returns a DoctorIssue for each structural problem found.
// Returns an empty (non-nil) slice when the log is clean. Never mutates state.
func (a *App) ValidateEventLog(ctx context.Context) ([]DoctorIssue, error) {
	events, err := a.store.GetAllEventsRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("validate event log: %w", err)
	}
	return validateEventLog(events, a.bus.PayloadFactories()), nil
}

func validateEventLog(events []store.RawEvent, factories map[inventory.EventType]func() any) []DoctorIssue {
	createdIDs := make(map[string]bool)
	removedIDs := make(map[string]int64) // entity ID → first remove event_id

	issues := make([]DoctorIssue, 0)
	for _, ev := range events {
		et, parseErr := inventory.ParseEventType(ev.EventType)

		// Track created/removed before per-event validation so that malformed
		// or otherwise-flagged events are still counted for orphan detection.
		if parseErr == nil && ev.EntityID != nil && *ev.EntityID != "" {
			switch et { //nolint:exhaustive // only created/borrowed/removed affect orphan tracking; all other event types are intentionally ignored
			case inventory.EntityCreatedEvent, inventory.EntityBorrowedEvent:
				createdIDs[*ev.EntityID] = true
			case inventory.EntityRemovedEvent:
				if _, seen := removedIDs[*ev.EntityID]; !seen {
					removedIDs[*ev.EntityID] = ev.EventID
				}
			}
		}

		issues = append(issues, validateSingleEvent(ev, et, parseErr, factories)...)
	}

	for entityID, removeEventID := range removedIDs {
		if !createdIDs[entityID] {
			eid := removeEventID
			issues = append(issues, DoctorIssue{
				Kind:        DoctorKindEventLog,
				EventID:     &eid,
				Description: fmt.Sprintf("entity %s has entity.removed event but no entity.created event", entityID),
			})
		}
	}

	return issues
}

func validateSingleEvent(
	ev store.RawEvent,
	et inventory.EventType,
	parseErr error,
	factories map[inventory.EventType]func() any,
) []DoctorIssue {
	id := ev.EventID
	var issues []DoctorIssue

	if parseErr != nil {
		return append(issues, DoctorIssue{
			Kind:        DoctorKindEventLog,
			EventID:     &id,
			Description: fmt.Sprintf("unknown event type %q", ev.EventType),
		})
	}

	if proto, ok := factories[et]; ok {
		target := proto()
		if unmarshalErr := json.Unmarshal(ev.Payload, target); unmarshalErr != nil {
			return append(issues, DoctorIssue{
				Kind:        DoctorKindEventLog,
				EventID:     &id,
				Description: fmt.Sprintf("malformed payload for %s: %v", ev.EventType, unmarshalErr),
			})
		}
	}

	if ev.EntityID == nil || *ev.EntityID == "" {
		return append(issues, DoctorIssue{
			Kind:        DoctorKindEventLog,
			EventID:     &id,
			Description: fmt.Sprintf("event %d missing entity_id", id),
		})
	}

	if et == inventory.EntityCreatedEvent {
		var p eventbus.EntityCreatedPayload
		_ = json.Unmarshal(ev.Payload, &p) // already validated above
		if p.DisplayName == "" {
			return append(issues, DoctorIssue{
				Kind:        DoctorKindEventLog,
				EventID:     &id,
				Description: fmt.Sprintf("entity.created event %d missing display_name", id),
			})
		}
	}

	return issues
}

// CheckProjectionConsistency performs a logical diff between the event log and the
// entities_current projection without mutating any state.
func (a *App) CheckProjectionConsistency(ctx context.Context) ([]DoctorIssue, error) {
	events, err := a.store.GetAllEventsRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("check projection consistency: %w", err)
	}

	expectedRows, maxEventID := buildProjectionSets(events)

	projRows, err := a.store.ListAllEntities(ctx)
	if err != nil {
		return nil, fmt.Errorf("check projection consistency: %w", err)
	}

	issues := checkProjectionRows(projRows, expectedRows, maxEventID)
	issues = append(issues, checkHomeRows(projRows)...)
	return issues, nil
}

// RunDoctorChecks fetches the event log and projection rows once (concurrently)
// and runs both validation passes against the shared data.
func (a *App) RunDoctorChecks(ctx context.Context) ([]DoctorIssue, error) {
	var events []store.RawEvent
	var projRows []*inventory.Entity

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		events, err = a.store.GetAllEventsRaw(egCtx)
		return err
	})
	eg.Go(func() error {
		var err error
		projRows, err = a.store.ListAllEntities(egCtx)
		return err
	})
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("doctor checks: %w", err)
	}

	issues := validateEventLog(events, a.bus.PayloadFactories())
	expectedRows, maxEventID := buildProjectionSets(events)
	issues = append(issues, checkProjectionRows(projRows, expectedRows, maxEventID)...)
	issues = append(issues, checkHomeRows(projRows)...)
	return issues, nil
}

// TruncateAndReplay rebuilds all projection tables by replaying the event log and returns the number of replayed events.
func (a *App) TruncateAndReplay(ctx context.Context) (int, error) {
	return a.bus.TruncateAndReplay(ctx)
}
func buildProjectionSets(events []store.RawEvent) (map[string]bool, map[string]int64) {
	created := make(map[string]bool)
	removed := make(map[string]bool)
	maxEventID := make(map[string]int64)

	for _, ev := range events {
		if ev.EntityID == nil {
			continue
		}
		id := *ev.EntityID
		if ev.EventID > maxEventID[id] {
			maxEventID[id] = ev.EventID
		}
		et, parseErr := inventory.ParseEventType(ev.EventType)
		if parseErr != nil {
			continue
		}
		switch et { //nolint:exhaustive // only created/borrowed/removed affect the expected-rows set; all other event types are intentionally ignored
		case inventory.EntityCreatedEvent, inventory.EntityBorrowedEvent:
			created[id] = true
		case inventory.EntityRemovedEvent:
			removed[id] = true
		}
	}

	// A soft remove keeps the projection row (status removed), so every created
	// entity is expected to have a row; the value records whether it was removed.
	expectedRows := make(map[string]bool, len(created))
	for id := range created {
		expectedRows[id] = removed[id]
	}
	return expectedRows, maxEventID
}

func checkProjectionRows(
	projRows []*inventory.Entity,
	expectedRows map[string]bool,
	maxEventID map[string]int64,
) []DoctorIssue {
	issues := make([]DoctorIssue, 0)
	inProjection := make(map[string]bool, len(projRows))

	for _, row := range projRows {
		inProjection[row.EntityID] = true

		wantRemoved, expected := expectedRows[row.EntityID]
		if !expected {
			issues = append(issues, DoctorIssue{
				Kind:        DoctorKindProjection,
				Description: fmt.Sprintf("phantom projection row for entity %s", row.EntityID),
			})
			continue
		}

		if wantRemoved && row.Status != inventory.EntityStatusRemoved {
			issues = append(issues, DoctorIssue{
				Kind: DoctorKindProjection,
				Description: fmt.Sprintf(
					"projection for entity %s has status %s, want removed",
					row.EntityID,
					row.Status,
				),
			})
		}

		if wantID, ok := maxEventID[row.EntityID]; ok && row.LastEventID != wantID {
			issues = append(issues, DoctorIssue{
				Kind: DoctorKindProjection,
				Description: fmt.Sprintf(
					"stale projection for entity %s: last_event_id=%d, want %d",
					row.EntityID,
					row.LastEventID,
					wantID,
				),
			})
		}
	}

	for id := range expectedRows {
		if !inProjection[id] {
			issues = append(issues, DoctorIssue{
				Kind:        DoctorKindProjection,
				Description: fmt.Sprintf("missing projection row for entity %s", id),
			})
		}
	}

	return issues
}

// checkHomeRows reports violations of the home invariant (CONTEXT.md "Hard
// Invariants"): every non-removed entity's home is non-removed, non-discrete,
// and neither the entity itself nor one of its descendants.
func checkHomeRows(rows []*inventory.Entity) []DoctorIssue {
	byID := make(map[string]*inventory.Entity, len(rows))
	for _, e := range rows {
		byID[e.EntityID] = e
	}
	var issues []DoctorIssue
	report := func(e *inventory.Entity, why string) {
		issues = append(issues, DoctorIssue{
			Kind:        DoctorKindProjection,
			Description: fmt.Sprintf("entity %s: home %s %s", e.EntityID, *e.HomeID, why),
		})
	}
	for _, e := range rows {
		if e.Status == inventory.EntityStatusRemoved || e.HomeID == nil {
			continue
		}
		home, ok := byID[*e.HomeID]
		switch {
		case !ok || home.Status == inventory.EntityStatusRemoved:
			report(e, "is removed or missing")
		case home.Discrete:
			report(e, "is discrete")
		case homeInsideEntity(byID, e.EntityID, home.EntityID):
			report(e, "is the entity itself or one of its descendants")
		}
	}
	return issues
}

// homeInsideEntity walks homeID's ancestors (bounded by len(byID) to survive a
// damaged, looping tree) looking for entityID.
func homeInsideEntity(byID map[string]*inventory.Entity, entityID, homeID string) bool {
	id := homeID
	for range len(byID) + 1 {
		if id == entityID {
			return true
		}
		e, ok := byID[id]
		if !ok || e.ParentID == nil {
			return false
		}
		id = *e.ParentID
	}
	return false
}
