package inventory

import "fmt"

//go:generate stringer -type=EventType -linecomment

//nolint:recvcheck
type EventType int

const (
	EntityCreatedEvent       EventType = iota + 1 // entity.created
	EntityRenamedEvent                            // entity.renamed
	EntityReparentedEvent                         // entity.reparented
	EntityPathChangedEvent                        // entity.path_changed
	EntityStatusChangedEvent                      // entity.status_changed
	EntityRemovedEvent                            // entity.removed
)

var eventTypeByName = map[string]EventType{
	EntityCreatedEvent.String():       EntityCreatedEvent,
	EntityRenamedEvent.String():       EntityRenamedEvent,
	EntityReparentedEvent.String():    EntityReparentedEvent,
	EntityPathChangedEvent.String():   EntityPathChangedEvent,
	EntityStatusChangedEvent.String(): EntityStatusChangedEvent,
	EntityRemovedEvent.String():       EntityRemovedEvent,
}

func ParseEventType(s string) (EventType, error) {
	if et, ok := eventTypeByName[s]; ok {
		return et, nil
	}
	return 0, fmt.Errorf("unknown event type %q", s)
}
