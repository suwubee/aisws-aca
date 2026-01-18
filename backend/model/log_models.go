package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONB 类型，支持 PostgreSQL JSONB 和 SQLite TEXT
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("invalid type for JSONB")
	}

	return json.Unmarshal(bytes, j)
}

// JSONBArray 类型，用于 JSON 数组
type JSONBArray []interface{}

func (j JSONBArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONBArray) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("invalid type for JSONBArray")
	}

	return json.Unmarshal(bytes, j)
}
