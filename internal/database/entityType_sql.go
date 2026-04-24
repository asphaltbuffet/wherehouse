package database

import (
	"database/sql/driver"
	"fmt"
)

// Value implements [driver.Valuer], persisting EntityType as its string representation.
func (e EntityType) Value() (driver.Value, error) {
	return e.String(), nil
}

// Scan implements [sql.Scanner], reading a string from the database and converting
// back to the typed EntityType constant.
func (e *EntityType) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("EntityType.Scan: expected string, got %T", src)
	}
	parsed, err := ParseEntityType(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}
