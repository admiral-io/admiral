package querybuilder

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// validMetaKey matches alphanumeric identifiers with underscores (no SQL-special characters).
var validMetaKey = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validOperators is the set of comparison operators allowed in filter expressions.
var validOperators = map[string]bool{
	"~=": true,
	"!=": true,
	"<=": true,
	">=": true,
	"=":  true,
	"<":  true,
	">":  true,
}

const (
	DefaultLimit   int32 = 50
	MaxResultLimit int32 = 1000
	MaxFilterLen         = 4096
)

type QueryBuilder interface {
	ParseFilter(string) (string, map[string]any, error)
	PaginatedQuery(string, int32, *string) func(*gorm.DB) *gorm.DB
}

type queryBuilder struct {
	columns []string // Whitelisted columns for filtering
}

func New(columns []string) QueryBuilder {
	defaultCols := []string{"id", "metadata", "created_at"}
	colMap := map[string]bool{}
	for _, c := range columns {
		colMap[c] = true
	}

	for _, d := range defaultCols {
		if !colMap[d] {
			colMap[d] = true
		}
	}

	merged := make([]string, 0, len(colMap))
	for c := range colMap {
		merged = append(merged, c)
	}

	return &queryBuilder{
		columns: merged,
	}
}

func (qb *queryBuilder) ParseFilter(filter string) (string, map[string]any, error) {
	filter = strings.TrimSpace(filter)
	if len(filter) == 0 {
		return "", nil, nil
	}

	if len(filter) > MaxFilterLen {
		return "", nil, fmt.Errorf("filter exceeds maximum length of %d bytes", MaxFilterLen)
	}

	res, err := ParseReader("", strings.NewReader(filter))
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse filter: %w", err)
	}

	q, ok := res.(*Query)
	if !ok {
		return "", nil, errors.New("unable to parse query into expected structure")
	}

	var aq strings.Builder
	av := make(map[string]any)

	for i, af := range q.AndFields {
		var colExpr string // the SQL column/expression (may contain JSON operators)
		k := af.Key

		switch k.Kind {
		case "field":
			if !slices.Contains(qb.columns, k.Name) {
				return "", nil, fmt.Errorf("query field '%s' is not whitelisted", k.Name)
			}
			colExpr = k.Name
		case "meta":
			parts := strings.Split(k.Name, ".")
			for _, p := range parts {
				if !validMetaKey.MatchString(p) {
					return "", nil, fmt.Errorf("invalid metadata key: %q", p)
				}
			}
			// Safety: single quotes in path segments are safe because validMetaKey
			// only allows [a-zA-Z_][a-zA-Z0-9_]* — no quotes or SQL-special chars.
			var sb strings.Builder
			for j, p := range parts {
				if j == 0 {
					sb.WriteString("metadata")
				}
				if j < len(parts)-1 {
					sb.WriteString(" -> ")
				} else {
					sb.WriteString(" ->> ")
				}
				fmt.Fprintf(&sb, "'%s'", p)
			}
			colExpr = sb.String()
		default:
			return "", nil, fmt.Errorf("query field '%s' has an unknown kind", k.Name)
		}

		if !validOperators[af.Operation] {
			return "", nil, fmt.Errorf("unsupported operator: %q", af.Operation)
		}

		// Use indexed parameter names to avoid collisions when the same
		// field appears more than once in a filter expression.
		paramName := fmt.Sprintf("p%d", i)

		if i > 0 {
			aq.WriteString(" AND ")
		}

		switch af.Operation {
		case "~=":
			fmt.Fprintf(&aq, "%s ILIKE @%s", colExpr, paramName)
			av[paramName] = fmt.Sprintf("%%%s%%", af.Value)
		case "!=":
			fmt.Fprintf(&aq, "%s != @%s", colExpr, paramName)
			av[paramName] = af.Value
		case "=":
			fmt.Fprintf(&aq, "%s = @%s", colExpr, paramName)
			av[paramName] = af.Value
		default:
			fmt.Fprintf(&aq, "%s %s @%s", colExpr, af.Operation, paramName)
			av[paramName] = af.Value
		}
	}

	return aq.String(), av, nil
}

func (qb *queryBuilder) PaginatedQuery(filter string, limit int32, pageToken *string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		wq, wv, err := qb.ParseFilter(filter)
		if err != nil {
			_ = db.AddError(err)
			return db
		}

		if pageToken != nil {
			decoded, err := base64.RawURLEncoding.DecodeString(*pageToken)
			if err != nil {
				_ = db.AddError(fmt.Errorf("invalid page token encoding: %w", err))
				return db
			}

			parts := strings.SplitN(string(decoded), "|", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				_ = db.AddError(fmt.Errorf("invalid page token format"))
				return db
			}

			ts, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				_ = db.AddError(fmt.Errorf("invalid timestamp in page token: %w", err))
				return db
			}

			cursorTime := time.Unix(ts, 0)
			cursorID := parts[1]
			cursorCondition := `((created_at > @cursorTime) OR (created_at = @cursorTime AND id > @cursorID))`

			if len(wq) > 0 {
				wq += " AND " + cursorCondition
			} else {
				wq = cursorCondition
			}
			if wv == nil {
				wv = make(map[string]any)
			}
			wv["cursorTime"] = cursorTime
			wv["cursorID"] = cursorID
		}

		if len(wq) > 0 {
			db = db.Where(wq, wv)
		}

		// Validate and set the limit.
		queryLimit := DefaultLimit
		if limit > MaxResultLimit {
			_ = db.AddError(fmt.Errorf("maximum query limit is %d", MaxResultLimit))
			return db
		} else if limit > 0 {
			queryLimit = limit
		}

		db = db.Order("created_at ASC, id ASC")
		return db.Limit(int(queryLimit))
	}
}

func EffectiveLimit(pageSize int32) int32 {
	if pageSize <= 0 {
		return DefaultLimit
	}
	if pageSize > MaxResultLimit {
		return MaxResultLimit
	}
	return pageSize
}
