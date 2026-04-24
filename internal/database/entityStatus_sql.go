package database

import (
	"database/sql/driver"
	"fmt"
)

// Value implements [driver.Valuer], persisting EntityStatus as its string representation.
func (e EntityStatus) Value() (driver.Value, error) {
	return e.String(), nil
}

// Scan implements [sql.Scanner], reading a string from the database and converting
// back to the typed EntityStatus constant.
func (e *EntityStatus) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("EntityStatus.Scan: expected string, got %T", src)
	}
	parsed, err := ParseEntityStatus(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}
