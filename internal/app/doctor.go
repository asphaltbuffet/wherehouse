package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
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
