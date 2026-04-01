package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type Labels map[string]string

func (l Labels) Value() (driver.Value, error) {
	if l == nil {
		return "{}", nil
	}

	b, err := json.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal labels: %w", err)
	}

	return string(b), nil
}

func (l *Labels) Scan(value any) error {
	if value == nil {
		*l = Labels{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("cannot scan %T into Labels", value)
	}

	return json.Unmarshal(bytes, l)
}
