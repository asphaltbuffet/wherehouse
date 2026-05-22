package inventory

import (
	"database/sql/driver"
	"fmt"
)

// Value implements the [driver.Valuer] interface for SQL persistence.
func (e EntityStatus) Value() (driver.Value, error) {
	return e.String(), nil
}

// Scan implements the sql.Scanner interface for SQL retrieval.
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
