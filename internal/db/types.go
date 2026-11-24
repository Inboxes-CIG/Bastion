package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringArray is a []string that maps to a PostgreSQL jsonb column.
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (a *StringArray) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("StringArray: cannot scan %T", src)
	}
	if len(b) == 0 {
		*a = nil
		return nil
	}
	return json.Unmarshal(b, a)
}
