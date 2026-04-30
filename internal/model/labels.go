package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Label syntax mirrors Kubernetes label rules
// (https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/):
//
//   - Key has an optional `prefix/` followed by a name.
//   - Prefix is a DNS subdomain: lowercase alphanumeric segments separated
//     by dots, max 253 characters.
//   - Name is alphanumeric (any case) plus `-`, `_`, `.`; starts and ends
//     with alphanumeric; max 63 characters.
//   - Value is empty, or follows the same rules as name (max 63 chars).
const (
	labelMaxNameLen   = 63
	labelMaxPrefixLen = 253
	labelMaxValueLen  = 63
)

var (
	labelNameRegex   = regexp.MustCompile(`^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`)
	labelPrefixRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
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

func (l Labels) Validate() error {
	for k, v := range l {
		if err := validateLabelKey(k); err != nil {
			return fmt.Errorf("invalid label key %q: %w", k, err)
		}
		if err := validateLabelValue(v); err != nil {
			return fmt.Errorf("invalid label value for key %q: %w", k, err)
		}
	}
	return nil
}

func validateLabelKey(k string) error {
	if k == "" {
		return fmt.Errorf("must not be empty")
	}
	var prefix, name string
	if i := strings.Index(k, "/"); i >= 0 {
		prefix = k[:i]
		name = k[i+1:]
	} else {
		name = k
	}
	if prefix != "" {
		if len(prefix) > labelMaxPrefixLen {
			return fmt.Errorf("prefix exceeds %d characters", labelMaxPrefixLen)
		}
		if !labelPrefixRegex.MatchString(prefix) {
			return fmt.Errorf("prefix must be a DNS subdomain (lowercase alphanumeric, dots, hyphens)")
		}
	}
	if name == "" {
		return fmt.Errorf("name part is required")
	}
	if len(name) > labelMaxNameLen {
		return fmt.Errorf("name exceeds %d characters", labelMaxNameLen)
	}
	if !labelNameRegex.MatchString(name) {
		return fmt.Errorf("name must be alphanumeric (with -_.), starting and ending with alphanumeric")
	}
	return nil
}

func validateLabelValue(v string) error {
	if v == "" {
		return nil
	}
	if len(v) > labelMaxValueLen {
		return fmt.Errorf("exceeds %d characters", labelMaxValueLen)
	}
	if !labelNameRegex.MatchString(v) {
		return fmt.Errorf("must be alphanumeric (with -_.), starting and ending with alphanumeric")
	}
	return nil
}
