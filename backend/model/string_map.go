package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

// StringMap stores a map[string]string as JSON in the database.
type StringMap map[string]string

func (m StringMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	data, err := json.Marshal(map[string]string(m))
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (m *StringMap) Scan(value interface{}) error {
	if m == nil {
		return fmt.Errorf("StringMap: Scan on nil pointer")
	}

	if value == nil {
		*m = StringMap{}
		return nil
	}

	var raw string
	switch v := value.(type) {
	case []byte:
		raw = string(v)
	case string:
		raw = v
	default:
		return fmt.Errorf("StringMap: unsupported Scan type %T", value)
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		*m = StringMap{}
		return nil
	}

	var decoded map[string]string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return err
	}
	if decoded == nil {
		decoded = map[string]string{}
	}
	*m = StringMap(decoded)
	return nil
}
