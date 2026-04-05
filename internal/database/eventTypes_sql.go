package database

import (
	"database/sql/driver"
	"fmt"
)

// Value implements [driver.Valuer], persisting EventType as its string representation.
func (e EventType) Value() (driver.Value, error) {
	return e.String(), nil
}

// Scan implements [sql.Scanner], reading a string from the database and converting
// back to the typed EventType constant.
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
