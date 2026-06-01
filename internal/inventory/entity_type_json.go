package inventory

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON encodes the EntityType as its string name (e.g. "place").
// It returns an error for an out-of-range value, so a construction bug
// surfaces at the serialization boundary rather than emitting a bogus token.
func (e EntityType) MarshalJSON() ([]byte, error) {
	if _, ok := entityTypeByName[e.String()]; !ok {
		return nil, fmt.Errorf("invalid EntityType %d", int(e))
	}
	return json.Marshal(e.String())
}

// UnmarshalJSON decodes an EntityType from its string name, rejecting
// unknown names and non-string JSON values.
func (e *EntityType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("EntityType.UnmarshalJSON: %w", err)
	}
	parsed, err := ParseEntityType(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}
