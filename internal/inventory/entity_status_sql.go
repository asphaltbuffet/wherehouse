package inventory

import (
	"database/sql/driver"
	"fmt"
)

func (e EntityStatus) Value() (driver.Value, error) {
	return e.String(), nil
}

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
