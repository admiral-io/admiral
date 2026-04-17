package template

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"text/template"
)

// FuncMap returns the standard set of helper functions available in every
// Admiral template expression. The set is intentionally small and stable;
// add new entries when there is a demonstrated need, not speculatively.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		// Serialisation
		"toJson":   toJSON,
		"fromJson": fromJSON,

		// Defaults / guards
		"default":  dflt,
		"required": required,

		// URL decomposition
		"urlScheme": urlScheme,
		"urlHost":   urlHost,
		"urlPort":   urlPort,
		"urlPath":   urlPath,

		// String helpers
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"trimPrefix": strings.TrimPrefix,
		"trimSuffix": strings.TrimSuffix,
		"replace":    replace,
		"contains":   strings.Contains,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,

		// Collection helpers
		"join":  join,
		"split": strings.Split,
		"index": safeIndex,
	}
}

func toJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("toJson: %w", err)
	}
	return string(b), nil
}

func fromJSON(s string) (any, error) {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("fromJson: %w", err)
	}
	return v, nil
}

// dflt returns val if it is non-empty, otherwise returns fallback.
// "empty" means zero-value for the type (nil, "", 0, false, empty slice/map).
func dflt(fallback, val any) any {
	if isEmpty(val) {
		return fallback
	}
	return val
}

func required(msg string, val any) (any, error) {
	if isEmpty(val) {
		return nil, fmt.Errorf("required: %s", msg)
	}
	return val, nil
}

func urlScheme(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("urlScheme: %w", err)
	}
	return u.Scheme, nil
}

func urlHost(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("urlHost: %w", err)
	}
	return u.Hostname(), nil
}

func urlPort(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("urlPort: %w", err)
	}
	return u.Port(), nil
}

func urlPath(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("urlPath: %w", err)
	}
	return u.Path, nil
}

// replace wraps strings.ReplaceAll with argument order suitable for pipelines:
// {{ .var.x | replace "old" "new" }}
func replace(old, new, s string) string {
	return strings.ReplaceAll(s, old, new)
}

// join converts a slice to a delimited string. Handles []any (common from
// JSON unmarshalling) in addition to []string.
func join(sep string, v any) (string, error) {
	switch s := v.(type) {
	case []string:
		return strings.Join(s, sep), nil
	case []any:
		parts := make([]string, 0, len(s))
		for _, elem := range s {
			parts = append(parts, fmt.Sprint(elem))
		}
		return strings.Join(parts, sep), nil
	default:
		return "", fmt.Errorf("join: expected slice, got %T", v)
	}
}

// safeIndex performs deep key lookup on a map. Returns nil for missing keys
// rather than panicking, so templates can chain with `default`.
//
//	{{ index .component.vpc "subnet_ids" }}
func safeIndex(v any, keys ...string) any {
	cur := v
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = m[k]
		if !ok {
			return nil
		}
	}
	return cur
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case string:
		return t == ""
	case bool:
		return !t
	case int:
		return t == 0
	case int64:
		return t == 0
	case float64:
		return t == 0
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	default:
		return false
	}
}
