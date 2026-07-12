package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// NullableFeatures is a custom type for nullable JSONB array of strings.
type NullableFeatures []string

// Scan implements sql.Scanner for reading from database.
func (nf *NullableFeatures) Scan(value interface{}) error {
	if value == nil {
		*nf = nil

		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal JSONB value: %v", value)
	}

	var features []string
	if err := json.Unmarshal(bytes, &features); err != nil {
		return err
	}

	*nf = features

	return nil
}

// Value implements driver.Valuer for writing to database.
func (nf NullableFeatures) Value() (driver.Value, error) {
	if nf == nil {
		return nil, nil
	}

	return json.Marshal(nf)
}

// Features is a custom type for JSONB array of strings.
type Features []string

// Scan implements sql.Scanner for reading from database.
func (f *Features) Scan(value interface{}) error {
	if value == nil {
		*f = []string{}

		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal JSONB value: %v", value)
	}

	return json.Unmarshal(bytes, f)
}

// Value implements driver.Valuer for writing to database.
func (f Features) Value() (driver.Value, error) {
	if len(f) == 0 {
		return json.Marshal([]string{})
	}

	return json.Marshal(f)
}
