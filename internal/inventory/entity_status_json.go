package inventory

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON encodes the EntityStatus as its string name (e.g. "ok").
// It returns an error for an out-of-range value, so a construction bug
// surfaces at the serialization boundary rather than emitting a bogus token.
func (e EntityStatus) MarshalJSON() ([]byte, error) {
	if _, ok := entityStatusByName[e.String()]; !ok {
		return nil, fmt.Errorf("invalid EntityStatus %d", int(e))
	}
	return json.Marshal(e.String())
}

// UnmarshalJSON decodes an EntityStatus from its string name, rejecting
// unknown names and non-string JSON values.
func (e *EntityStatus) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("EntityStatus.UnmarshalJSON: %w", err)
	}
	parsed, err := ParseEntityStatus(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}
