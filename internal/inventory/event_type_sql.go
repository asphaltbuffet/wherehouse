package inventory

import (
	"database/sql/driver"
	"fmt"
)

func (e EventType) Value() (driver.Value, error) {
	return e.String(), nil
}

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
