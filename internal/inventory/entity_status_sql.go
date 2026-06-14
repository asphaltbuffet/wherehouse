package inventory

import (
	"database/sql/driver"
	"fmt"
)

// Value implements [driver.Valuer], serializing the status as its string name.
func (e EntityStatus) Value() (driver.Value, error) {
	return e.String(), nil
}

// Scan implements [sql.Scanner], parsing a string column back into an EntityStatus.
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
