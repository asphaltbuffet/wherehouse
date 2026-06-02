package inventory

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON encodes the EventType as its string name (e.g. "entity.created").
// It returns an error for an out-of-range value, so a construction bug
// surfaces at the serialization boundary rather than emitting a bogus token.
func (e EventType) MarshalJSON() ([]byte, error) {
	if _, ok := eventTypeByName[e.String()]; !ok {
		return nil, fmt.Errorf("invalid EventType %d", int(e))
	}
	return json.Marshal(e.String())
}

// UnmarshalJSON decodes an EventType from its string name, rejecting
// unknown names and non-string JSON values.
func (e *EventType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("EventType.UnmarshalJSON: %w", err)
	}
	parsed, err := ParseEventType(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}
