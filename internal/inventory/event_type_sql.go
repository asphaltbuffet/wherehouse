package inventory

import (
	"database/sql/driver"
	"fmt"
)

// Value implements [driver.Valuer], serializing the event type as its string name.
func (e EventType) Value() (driver.Value, error) {
	return e.String(), nil
}

// Scan implements [sql.Scanner], parsing a string column back into an EventType.
func (e *EventType) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("EventType.Scan: expected string, got %T", src)
	}
	parsed, err := ParseEventType(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}
