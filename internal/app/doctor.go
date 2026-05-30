package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

var payloadPrototypes = map[inventory.EventType]func() any{
	inventory.EntityCreatedEvent:       func() any { return &eventbus.EntityCreatedPayload{} },
	inventory.EntityRenamedEvent:       func() any { return &eventbus.EntityRenamedPayload{} },
	inventory.EntityReparentedEvent:    func() any { return &eventbus.EntityReparentedPayload{} },
	inventory.EntityPathChangedEvent:   func() any { return &eventbus.EntityPathChangedPayload{} },
	inventory.EntityStatusChangedEvent: func() any { return &eventbus.EntityStatusChangedPayload{} },
	inventory.EntityRemovedEvent:       func() any { return &eventbus.EntityRemovedPayload{} },
}

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

	issues := make([]DoctorIssue, 0)
	for _, ev := range events {
		id := ev.EventID
		et, parseErr := inventory.ParseEventType(ev.EventType)
		if parseErr != nil {
			issues = append(issues, DoctorIssue{
				Kind:        DoctorKindEventLog,
				EventID:     &id,
				Description: fmt.Sprintf("unknown event type %q", ev.EventType),
			})
			continue
		}

		if proto, ok := payloadPrototypes[et]; ok {
			target := proto()
			if unmarshalErr := json.Unmarshal(ev.Payload, target); unmarshalErr != nil {
				issues = append(issues, DoctorIssue{
					Kind:        DoctorKindEventLog,
					EventID:     &id,
					Description: fmt.Sprintf("malformed payload for %s: %v", ev.EventType, unmarshalErr),
				})
				continue
			}
		}

		if ev.EntityID == nil || *ev.EntityID == "" {
			issues = append(issues, DoctorIssue{
				Kind:        DoctorKindEventLog,
				EventID:     &id,
				Description: fmt.Sprintf("event %d missing entity_id", id),
			})
			continue
		}

		if et == inventory.EntityCreatedEvent {
			var p eventbus.EntityCreatedPayload
			_ = json.Unmarshal(ev.Payload, &p) // already validated above
			if p.DisplayName == "" {
				issues = append(issues, DoctorIssue{
					Kind:        DoctorKindEventLog,
					EventID:     &id,
					Description: fmt.Sprintf("entity.created event %d missing display_name", id),
				})
				continue
			}
			if _, typeErr := inventory.ParseEntityType(p.EntityType); typeErr != nil {
				issues = append(issues, DoctorIssue{
					Kind:        DoctorKindEventLog,
					EventID:     &id,
					Description: fmt.Sprintf("entity.created event %d invalid entity_type %q", id, p.EntityType),
				})
				continue
			}
		}
	}
	return issues, nil
}

// CheckProjectionConsistency performs a logical diff between the event log and the
// entities_current projection without mutating any state.
func (a *App) CheckProjectionConsistency(ctx context.Context) ([]DoctorIssue, error) {
	events, err := a.store.GetAllEventsRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("check projection consistency: %w", err)
	}

	expectedPresent, maxEventID := buildProjectionSets(events)

	projRows, err := a.store.ListAllEntities(ctx)
	if err != nil {
		return nil, fmt.Errorf("check projection consistency: %w", err)
	}

	return checkProjectionRows(projRows, expectedPresent, maxEventID), nil
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
		switch et { //nolint:exhaustive // only created/removed affect the expected-present set
		case inventory.EntityCreatedEvent:
			created[id] = true
		case inventory.EntityRemovedEvent:
			removed[id] = true
		}
	}

	expectedPresent := make(map[string]bool, len(created))
	for id := range created {
		if !removed[id] {
			expectedPresent[id] = true
		}
	}
	return expectedPresent, maxEventID
}

func checkProjectionRows(
	projRows []*inventory.Entity,
	expectedPresent map[string]bool,
	maxEventID map[string]int64,
) []DoctorIssue {
	issues := make([]DoctorIssue, 0)
	inProjection := make(map[string]bool, len(projRows))

	for _, row := range projRows {
		inProjection[row.EntityID] = true

		if !expectedPresent[row.EntityID] {
			issues = append(issues, DoctorIssue{
				Kind:        DoctorKindProjection,
				Description: fmt.Sprintf("phantom projection row for entity %s", row.EntityID),
			})
			continue
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

	for id := range expectedPresent {
		if !inProjection[id] {
			issues = append(issues, DoctorIssue{
				Kind:        DoctorKindProjection,
				Description: fmt.Sprintf("missing projection row for entity %s", id),
			})
		}
	}

	return issues
}
