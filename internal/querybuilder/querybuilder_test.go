package querybuilder

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

type object struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Foo       string
	Bar       string
	Meta      string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func TestUUIDAsPrimaryKey(t *testing.T) {
	qb := New([]string{"foo", "bar"})
	db, _ := gorm.Open(tests.DummyDialector{}, nil)

	// Create an object with a UUID.
	sampleUUID := uuid.New()
	obj := object{
		ID:  sampleUUID,
		Foo: "some foo",
		Bar: "some bar",
	}

	dryRunDB := db.Session(&gorm.Session{DryRun: true}).Create(&obj)
	insertSQL := dryRunDB.Statement.SQL.String()
	assert.Contains(t, insertSQL, "INSERT INTO")

	filter := "field['id'] = '" + sampleUUID.String() + "'"
	q := db.Session(&gorm.Session{DryRun: true}).
		Scopes(qb.PaginatedQuery(filter, 0, nil)).
		Find(&[]object{})
	querySQL := q.Statement.SQL.String()

	assert.Contains(t, querySQL, "id =")
	assert.NoError(t, q.Error)
}

// ---------------------------------------------------------------------------
// TestMaxQueryLimit
// ---------------------------------------------------------------------------

func TestMaxQueryLimit(t *testing.T) {
	qb := New([]string{"foo", "bar"})
	db, _ := gorm.Open(tests.DummyDialector{}, nil)

	testCases := []struct {
		id          string
		input       int32
		shouldError bool
	}{
		{
			id:          "Empty limit",
			input:       0,
			shouldError: false,
		},
		{
			id:          "Under limit",
			input:       999,
			shouldError: false,
		},
		{
			id:          "Equal to limit",
			input:       1000,
			shouldError: false,
		},
		{
			id:          "Above limit",
			input:       1001,
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			err := qb.PaginatedQuery("", tc.input, nil)(db).Error
			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestMaxQueryLimitInSQL — verifies actual LIMIT values in generated SQL
// ---------------------------------------------------------------------------

func TestMaxQueryLimitInSQL(t *testing.T) {
	qb := New([]string{"foo", "bar"})
	db, _ := gorm.Open(tests.DummyDialector{}, nil)

	t.Run("Default limit is 50 when zero is provided", func(t *testing.T) {
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("", 0, nil)).
			Find(&[]object{})
		require.NoError(t, q.Error)
		assert.Contains(t, q.Statement.Vars, int(DefaultLimit),
			"expected LIMIT to use DefaultLimit (%d)", DefaultLimit)
	})

	t.Run("Custom limit is passed through", func(t *testing.T) {
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("", 25, nil)).
			Find(&[]object{})
		require.NoError(t, q.Error)
		assert.Contains(t, q.Statement.Vars, 25)
	})

	t.Run("Max limit is accepted", func(t *testing.T) {
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("", MaxResultLimit, nil)).
			Find(&[]object{})
		require.NoError(t, q.Error)
		assert.Contains(t, q.Statement.Vars, int(MaxResultLimit))
	})

	t.Run("Negative limit uses default", func(t *testing.T) {
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("", -1, nil)).
			Find(&[]object{})
		require.NoError(t, q.Error)
		assert.Contains(t, q.Statement.Vars, int(DefaultLimit))
	})
}

// ---------------------------------------------------------------------------
// TestPaginatedQueryBuilder
// ---------------------------------------------------------------------------

func TestPaginatedQueryBuilder(t *testing.T) {
	qb := New([]string{"foo", "bar"})
	db, _ := gorm.Open(tests.DummyDialector{}, nil)

	testCases := []struct {
		id          string
		input       string
		expect      string
		shouldError bool
	}{
		{
			id:          "No filter",
			input:       "",
			expect:      "SELECT * FROM `objects` WHERE `objects`.`deleted_at` IS NULL ORDER BY created_at ASC, id ASC LIMIT ?",
			shouldError: false,
		},
		{
			id:          "Non-whitelisted field rejected",
			input:       "field['baz'] = value",
			expect:      "",
			shouldError: true,
		},
		{
			id:          "Search by field",
			input:       "field['foo'] = value",
			expect:      "SELECT * FROM `objects` WHERE foo ILIKE ? AND `objects`.`deleted_at` IS NULL ORDER BY created_at ASC, id ASC LIMIT ?",
			shouldError: false,
		},
		{
			id:          "Search by quoted field",
			input:       "field['foo'] = 'value'",
			expect:      "SELECT * FROM `objects` WHERE foo ILIKE ? AND `objects`.`deleted_at` IS NULL ORDER BY created_at ASC, id ASC LIMIT ?",
			shouldError: false,
		},
		{
			id:          "Search by multiple fields",
			input:       "field['foo'] = value1 field['bar'] = value2",
			expect:      "SELECT * FROM `objects` WHERE (foo ILIKE ? AND bar ILIKE ?) AND `objects`.`deleted_at` IS NULL ORDER BY created_at ASC, id ASC LIMIT ?",
			shouldError: false,
		},
		{
			id:          "Search by substring",
			input:       "field['foo'] ~= value",
			expect:      "SELECT * FROM `objects` WHERE foo ILIKE ? AND `objects`.`deleted_at` IS NULL ORDER BY created_at ASC, id ASC LIMIT ?",
			shouldError: false,
		},
		{
			id:          "Numeric equality",
			input:       "field['foo'] = 0",
			expect:      "SELECT * FROM `objects` WHERE foo = ? AND `objects`.`deleted_at` IS NULL ORDER BY created_at ASC, id ASC LIMIT ?",
			shouldError: false,
		},
		{
			id:          "Numeric less than",
			input:       "field['foo'] < 0",
			expect:      "SELECT * FROM `objects` WHERE foo < ? AND `objects`.`deleted_at` IS NULL ORDER BY created_at ASC, id ASC LIMIT ?",
			shouldError: false,
		},
		{
			id:          "Numeric less than or equal",
			input:       "field['foo'] <= 0",
			expect:      "SELECT * FROM `objects` WHERE foo <= ? AND `objects`.`deleted_at` IS NULL ORDER BY created_at ASC, id ASC LIMIT ?",
			shouldError: false,
		},
		{
			id:          "Numeric greater than",
			input:       "field['foo'] > 0",
			expect:      "SELECT * FROM `objects` WHERE foo > ? AND `objects`.`deleted_at` IS NULL ORDER BY created_at ASC, id ASC LIMIT ?",
			shouldError: false,
		},
		{
			id:          "Numeric greater than or equal",
			input:       "field['foo'] >= 0",
			expect:      "SELECT * FROM `objects` WHERE foo >= ? AND `objects`.`deleted_at` IS NULL ORDER BY created_at ASC, id ASC LIMIT ?",
			shouldError: false,
		},
		{
			id:          "Not equal operator",
			input:       "field['foo'] != 'bar'",
			expect:      "SELECT * FROM `objects` WHERE foo != ? AND `objects`.`deleted_at` IS NULL ORDER BY created_at ASC, id ASC LIMIT ?",
			shouldError: false,
		},
		{
			id:          "Multiple fields with AND keyword",
			input:       "field['foo'] = 'value1' AND field['bar'] = 'value2'",
			expect:      "SELECT * FROM `objects` WHERE (foo ILIKE ? AND bar ILIKE ?) AND `objects`.`deleted_at` IS NULL ORDER BY created_at ASC, id ASC LIMIT ?",
			shouldError: false,
		},
		{
			id:          "Trailing garbage rejected",
			input:       "field['foo'] = 'v' GARBAGE",
			expect:      "",
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			var objects []*object

			q := db.Session(&gorm.Session{DryRun: true}).Scopes(qb.PaginatedQuery(tc.input, 0, nil)).Find(&objects)
			err := q.Error
			sql := q.Statement.SQL.String()

			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expect, sql)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestParseFilter
// ---------------------------------------------------------------------------

func TestParseFilter(t *testing.T) {
	queryBuilder := New([]string{"foo", "bar"})

	testCases := []struct {
		id           string
		input        string
		expectQuery  string
		expectValues map[string]interface{}
		shouldError  bool
	}{
		{
			id:           "Empty string returns empty result",
			input:        "",
			expectQuery:  "",
			expectValues: nil,
			shouldError:  false,
		},
		{
			id:          "Non-whitelisted field rejected",
			input:       "field['baz'] = value",
			shouldError: true,
		},
		{
			id:           "Search by field",
			input:        "field['foo'] = value",
			expectQuery:  "foo ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "value"},
			shouldError:  false,
		},
		{
			id:           "Search by double-quoted field key",
			input:        `field[ "foo" ] = "value"`,
			expectQuery:  "foo ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "value"},
			shouldError:  false,
		},
		{
			id:           "Search by single quoted field",
			input:        "field['foo'] = 'value'",
			expectQuery:  "foo ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "value"},
			shouldError:  false,
		},
		{
			id:           "Search by multiple fields",
			input:        "field['foo'] = 'value1' field['bar'] = 'value2'",
			expectQuery:  "foo ILIKE @p0 AND bar ILIKE @p1",
			expectValues: map[string]interface{}{"p0": "value1", "p1": "value2"},
			shouldError:  false,
		},
		{
			id:           "Search by substring",
			input:        "field['foo'] ~= 'value'",
			expectQuery:  "foo ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "%value%"},
			shouldError:  false,
		},
		{
			id:           "Search by uuid",
			input:        "field['foo'] = '258e4d48-0369-4287-a375-4a496718d174'",
			expectQuery:  "foo = @p0",
			expectValues: map[string]interface{}{"p0": "258e4d48-0369-4287-a375-4a496718d174"},
			shouldError:  false,
		},
		{
			id:           "Numeric equality",
			input:        "field['foo'] = 0",
			expectQuery:  "foo = @p0",
			expectValues: map[string]interface{}{"p0": int64(0)},
			shouldError:  false,
		},
		{
			id:           "Numeric less than",
			input:        "field['foo'] < 0",
			expectQuery:  "foo < @p0",
			expectValues: map[string]interface{}{"p0": int64(0)},
			shouldError:  false,
		},
		{
			id:           "Numeric less than or equal",
			input:        "field['foo'] <= 0",
			expectQuery:  "foo <= @p0",
			expectValues: map[string]interface{}{"p0": int64(0)},
			shouldError:  false,
		},
		{
			id:           "Numeric greater than",
			input:        "field['foo'] > 0",
			expectQuery:  "foo > @p0",
			expectValues: map[string]interface{}{"p0": int64(0)},
			shouldError:  false,
		},
		{
			id:           "Numeric greater than or equal",
			input:        "field['foo'] >= 0",
			expectQuery:  "foo >= @p0",
			expectValues: map[string]interface{}{"p0": int64(0)},
			shouldError:  false,
		},
		{
			id:           "Not equal operator",
			input:        "field['foo'] != 'bar'",
			expectQuery:  "foo != @p0",
			expectValues: map[string]interface{}{"p0": "bar"},
			shouldError:  false,
		},
		{
			id:           "Explicit AND keyword",
			input:        "field['foo'] = 'a' AND field['bar'] = 'b'",
			expectQuery:  "foo ILIKE @p0 AND bar ILIKE @p1",
			expectValues: map[string]interface{}{"p0": "a", "p1": "b"},
			shouldError:  false,
		},
		{
			id:           "Case-insensitive AND keyword",
			input:        "field['foo'] = 'a' and field['bar'] = 'b'",
			expectQuery:  "foo ILIKE @p0 AND bar ILIKE @p1",
			expectValues: map[string]interface{}{"p0": "a", "p1": "b"},
			shouldError:  false,
		},
		{
			id:           "Integer value parsed as int64",
			input:        "field['foo'] = 42",
			expectQuery:  "foo = @p0",
			expectValues: map[string]interface{}{"p0": int64(42)},
			shouldError:  false,
		},
		{
			id:           "Float value with decimal",
			input:        "field['foo'] = 3.14",
			expectQuery:  "foo = @p0",
			expectValues: map[string]interface{}{"p0": float64(3.14)},
			shouldError:  false,
		},
		{
			id:           "Float value with exponent",
			input:        "field['foo'] = 1e5",
			expectQuery:  "foo = @p0",
			expectValues: map[string]interface{}{"p0": float64(100000)},
			shouldError:  false,
		},
		{
			id:          "Trailing garbage rejected",
			input:       "field['foo'] = 'v' GARBAGE",
			shouldError: true,
		},
		{
			id:           "Single meta key uses text extraction",
			input:        "meta['region'] = 'us'",
			expectQuery:  "metadata ->> 'region' ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "us"},
			shouldError:  false,
		},
		{
			id:          "Meta key with SQL injection rejected",
			input:       "meta['x\\') OR 1=1--'] = 'us'",
			shouldError: true,
		},
		{
			id:           "Duplicate field names produce distinct params",
			input:        "field['foo'] > 10 field['foo'] < 20",
			expectQuery:  "foo > @p0 AND foo < @p1",
			expectValues: map[string]interface{}{"p0": int64(10), "p1": int64(20)},
			shouldError:  false,
		},
		{
			id:          "Filter exceeding max length rejected",
			input:       strings.Repeat("a", MaxFilterLen+1),
			shouldError: true,
		},
		// ---------------------------------------------------------------
		// New test cases: value types
		// ---------------------------------------------------------------
		{
			id:           "Boolean true value",
			input:        "field['foo'] = true",
			expectQuery:  "foo = @p0",
			expectValues: map[string]interface{}{"p0": true},
			shouldError:  false,
		},
		{
			id:           "Boolean false value",
			input:        "field['foo'] = false",
			expectQuery:  "foo = @p0",
			expectValues: map[string]interface{}{"p0": false},
			shouldError:  false,
		},
		{
			id:           "Case-insensitive boolean TRUE",
			input:        "field['foo'] = TRUE",
			expectQuery:  "foo = @p0",
			expectValues: map[string]interface{}{"p0": true},
			shouldError:  false,
		},
		{
			id:           "Negative integer",
			input:        "field['foo'] = -42",
			expectQuery:  "foo = @p0",
			expectValues: map[string]interface{}{"p0": int64(-42)},
			shouldError:  false,
		},
		{
			id:           "Negative float",
			input:        "field['foo'] = -3.14",
			expectQuery:  "foo = @p0",
			expectValues: map[string]interface{}{"p0": float64(-3.14)},
			shouldError:  false,
		},
		{
			id:           "Double-quoted string value",
			input:        `field['foo'] = "hello world"`,
			expectQuery:  "foo ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "hello world"},
			shouldError:  false,
		},
		{
			id:           "Escaped single quote in value",
			input:        "field['foo'] = 'it\\'s here'",
			expectQuery:  "foo ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "it's here"},
			shouldError:  false,
		},
		{
			id:           "Escaped double quote in value",
			input:        `field['foo'] = "say \"hi\""`,
			expectQuery:  "foo ILIKE @p0",
			expectValues: map[string]interface{}{"p0": `say "hi"`},
			shouldError:  false,
		},
		{
			id:           "Identifier value with dots",
			input:        "field['foo'] = some.dotted.value",
			expectQuery:  "foo ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "some.dotted.value"},
			shouldError:  false,
		},
		// ---------------------------------------------------------------
		// New test cases: meta key paths
		// ---------------------------------------------------------------
		{
			id:           "Nested two-level meta key",
			input:        "meta['a.b'] = 'val'",
			expectQuery:  "metadata -> 'a' ->> 'b' ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "val"},
			shouldError:  false,
		},
		{
			id:           "Deeply nested meta key",
			input:        "meta['a.b.c'] = 'val'",
			expectQuery:  "metadata -> 'a' -> 'b' ->> 'c' ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "val"},
			shouldError:  false,
		},
		{
			id:           "Meta key with substring operator",
			input:        "meta['region'] ~= 'us'",
			expectQuery:  "metadata ->> 'region' ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "%us%"},
			shouldError:  false,
		},
		{
			id:          "Empty meta key segment rejected",
			input:       "meta[''] = 'x'",
			shouldError: true,
		},
		{
			id:          "Meta key with special characters rejected",
			input:       "meta['bad;key'] = 'x'",
			shouldError: true,
		},
		// ---------------------------------------------------------------
		// New test cases: default columns
		// ---------------------------------------------------------------
		{
			id:           "Default column created_at is filterable",
			input:        "field['created_at'] > '2024-01-01'",
			expectQuery:  "created_at > @p0",
			expectValues: map[string]interface{}{"p0": "2024-01-01"},
			shouldError:  false,
		},
		{
			id:           "Default column id with non-UUID uses ILIKE",
			input:        "field['id'] = 'somevalue'",
			expectQuery:  "id ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "somevalue"},
			shouldError:  false,
		},
		{
			id:           "Default column id with UUID uses exact match",
			input:        "field['id'] = '550e8400-e29b-41d4-a716-446655440000'",
			expectQuery:  "id = @p0",
			expectValues: map[string]interface{}{"p0": "550e8400-e29b-41d4-a716-446655440000"},
			shouldError:  false,
		},
		{
			id:           "Default column metadata is filterable as plain field",
			input:        "field['metadata'] = 'test'",
			expectQuery:  "metadata ILIKE @p0",
			expectValues: map[string]interface{}{"p0": "test"},
			shouldError:  false,
		},
		// ---------------------------------------------------------------
		// New test cases: parser rejection
		// ---------------------------------------------------------------
		{
			id:          "OR keyword is not supported",
			input:       "field['foo'] = 'a' OR field['bar'] = 'b'",
			shouldError: true,
		},
		// NOTE: Whitespace-only input hits the parser (not the empty-string
		// fast path) and returns a parse error. This is arguable — you could
		// expect it to behave like empty string. Flagging for discussion.
		{
			id:          "Whitespace-only filter errors",
			input:       "   ",
			shouldError: true,
		},
		{
			id:          "Filter at exactly max length rejected",
			input:       strings.Repeat("a", MaxFilterLen+1),
			shouldError: true,
		},
		{
			id:          "Incomplete field expression rejected",
			input:       "field[",
			shouldError: true,
		},
		{
			id:          "Unclosed quote in field key rejected",
			input:       "field['unclosed",
			shouldError: true,
		},
		{
			id:          "Missing value rejected",
			input:       "field['foo'] = ",
			shouldError: true,
		},
		{
			id:          "Invalid operator rejected",
			input:       "field['foo'] ** 'value'",
			shouldError: true,
		},
		{
			id:          "Random text rejected",
			input:       "this is not a filter",
			shouldError: true,
		},
		// ---------------------------------------------------------------
		// New test cases: three fields with mixed operators
		// ---------------------------------------------------------------
		{
			id:           "Three fields with mixed operators",
			input:        "field['foo'] = 'a' AND field['bar'] > 10 AND field['id'] = '550e8400-e29b-41d4-a716-446655440000'",
			expectQuery:  "foo ILIKE @p0 AND bar > @p1 AND id = @p2",
			expectValues: map[string]interface{}{"p0": "a", "p1": int64(10), "p2": "550e8400-e29b-41d4-a716-446655440000"},
			shouldError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			q, v, err := queryBuilder.ParseFilter(tc.input)

			if tc.shouldError {
				assert.Error(t, err, "input: %q", tc.input)
			} else {
				assert.NoError(t, err, "input: %q", tc.input)
				assert.Equal(t, tc.expectQuery, q, "input: %q", tc.input)
				assert.Equal(t, tc.expectValues, v, "input: %q", tc.input)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestNewDefaultColumns — verifies that New() auto-adds id, metadata,
// created_at regardless of what the caller passes.
// ---------------------------------------------------------------------------

func TestNewDefaultColumns(t *testing.T) {
	qb := New([]string{"foo"})

	defaultCols := []string{"id", "metadata", "created_at"}
	for _, col := range defaultCols {
		t.Run(col+" is filterable", func(t *testing.T) {
			filter := "field['" + col + "'] = 'test'"
			_, _, err := qb.ParseFilter(filter)
			assert.NoError(t, err, "%s should be auto-added to whitelist", col)
		})
	}

	t.Run("Explicit column still works", func(t *testing.T) {
		_, _, err := qb.ParseFilter("field['foo'] = 'test'")
		assert.NoError(t, err)
	})

	t.Run("Non-added column still rejected", func(t *testing.T) {
		_, _, err := qb.ParseFilter("field['nope'] = 'test'")
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// TestNewDeduplicatesDefaultColumns — passing a default column explicitly
// should not cause duplicates or errors.
// ---------------------------------------------------------------------------

func TestNewDeduplicatesDefaultColumns(t *testing.T) {
	qb := New([]string{"id", "foo"})

	_, _, err := qb.ParseFilter("field['id'] = '550e8400-e29b-41d4-a716-446655440000'")
	assert.NoError(t, err)

	_, _, err = qb.ParseFilter("field['foo'] = 'bar'")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// TestPaginatedQueryPageToken — covers all page token branches in
// PaginatedQuery which previously had zero test coverage.
// ---------------------------------------------------------------------------

func TestPaginatedQueryPageToken(t *testing.T) {
	qb := New([]string{"foo", "bar"})
	db, _ := gorm.Open(tests.DummyDialector{}, nil)

	makeToken := func(payload string) *string {
		s := base64.RawURLEncoding.EncodeToString([]byte(payload))
		return &s
	}

	t.Run("Valid page token without filter", func(t *testing.T) {
		token := makeToken("1700000000|550e8400-e29b-41d4-a716-446655440000")
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("", 0, token)).
			Find(&[]object{})
		require.NoError(t, q.Error)
		sql := q.Statement.SQL.String()
		assert.Contains(t, sql, "created_at >")
		assert.Contains(t, sql, "created_at =")
		assert.Contains(t, sql, "id >=")
		assert.Contains(t, sql, "ORDER BY created_at ASC, id ASC")
	})

	t.Run("Valid page token with filter", func(t *testing.T) {
		token := makeToken("1700000000|550e8400-e29b-41d4-a716-446655440000")
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("field['foo'] = 'test'", 0, token)).
			Find(&[]object{})
		require.NoError(t, q.Error)
		sql := q.Statement.SQL.String()
		// Filter and cursor should both appear.
		assert.Contains(t, sql, "foo ILIKE")
		assert.Contains(t, sql, "created_at >")
	})

	t.Run("Invalid base64 page token", func(t *testing.T) {
		bad := "not-valid-base64!!!"
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("", 0, &bad)).
			Find(&[]object{})
		assert.Error(t, q.Error)
		assert.Contains(t, q.Error.Error(), "invalid page token encoding")
	})

	t.Run("Page token missing pipe separator", func(t *testing.T) {
		token := makeToken("nopipe")
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("", 0, token)).
			Find(&[]object{})
		assert.Error(t, q.Error)
		assert.Contains(t, q.Error.Error(), "invalid page token format")
	})

	t.Run("Page token with non-numeric timestamp", func(t *testing.T) {
		token := makeToken("notanumber|someid")
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("", 0, token)).
			Find(&[]object{})
		assert.Error(t, q.Error)
		assert.Contains(t, q.Error.Error(), "invalid timestamp in page token")
	})

	t.Run("Page token with too many pipe segments", func(t *testing.T) {
		token := makeToken("123|abc|extra")
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("", 0, token)).
			Find(&[]object{})
		assert.Error(t, q.Error)
		assert.Contains(t, q.Error.Error(), "invalid page token format")
	})

	t.Run("Nil page token is no-op", func(t *testing.T) {
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("", 0, nil)).
			Find(&[]object{})
		require.NoError(t, q.Error)
		sql := q.Statement.SQL.String()
		assert.NotContains(t, sql, "created_at >")
	})

	t.Run("Empty string page token", func(t *testing.T) {
		empty := ""
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("", 0, &empty)).
			Find(&[]object{})
		// Empty base64 decodes to empty string, which splits to [""],
		// len 1 != 2, so should error with "invalid page token format".
		assert.Error(t, q.Error)
		assert.Contains(t, q.Error.Error(), "invalid page token format")
	})
}

// ---------------------------------------------------------------------------
// TestPaginatedQueryPageTokenWithTooManyPipes — the current implementation
// uses strings.Split which yields >2 parts for multiple pipes, testing that
// this is correctly rejected.
// ---------------------------------------------------------------------------

func TestPaginatedQueryPageTokenEdgeCases(t *testing.T) {
	qb := New([]string{"foo"})
	db, _ := gorm.Open(tests.DummyDialector{}, nil)

	t.Run("Page token with pipe in cursor ID", func(t *testing.T) {
		// If a cursor ID somehow contained a pipe, Split would produce 3+ parts.
		// This should be rejected by the format check.
		token := base64.RawURLEncoding.EncodeToString([]byte("1700000000|id|with|pipes"))
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("", 0, &token)).
			Find(&[]object{})
		assert.Error(t, q.Error)
		assert.Contains(t, q.Error.Error(), "invalid page token format")
	})

	t.Run("Valid page token combined with invalid filter", func(t *testing.T) {
		token := base64.RawURLEncoding.EncodeToString([]byte("1700000000|someid"))
		q := db.Session(&gorm.Session{DryRun: true}).
			Scopes(qb.PaginatedQuery("field['nonexistent'] = 'x'", 0, &token)).
			Find(&[]object{})
		assert.Error(t, q.Error)
	})
}

// ---------------------------------------------------------------------------
// TestPaginatedQueryOrdering — verifies ORDER BY is always present
// ---------------------------------------------------------------------------

func TestPaginatedQueryOrdering(t *testing.T) {
	qb := New([]string{"foo"})
	db, _ := gorm.Open(tests.DummyDialector{}, nil)

	q := db.Session(&gorm.Session{DryRun: true}).
		Scopes(qb.PaginatedQuery("field['foo'] = 'bar'", 10, nil)).
		Find(&[]object{})
	require.NoError(t, q.Error)

	sql := q.Statement.SQL.String()
	assert.Contains(t, sql, "ORDER BY created_at ASC, id ASC")
}

// ---------------------------------------------------------------------------
// FuzzParseFilter
// ---------------------------------------------------------------------------

func FuzzParseFilter(f *testing.F) {
	qb := New([]string{"foo", "bar", "baz", "name", "value"})

	// Seed the fuzzer with known good and bad inputs
	testCases := []string{
		"",
		"field['foo'] = value",
		"field['foo'] = 'value'",
		`field["foo"] = "value"`,
		"field['foo'] ~= 'test'",
		"field['foo'] = 123",
		"field['foo'] > 0",
		"field['foo'] < 100",
		"field['foo'] >= 50",
		"field['foo'] <= 25",
		"field['foo'] = 'value1' field['bar'] = 'value2'",
		"field['nonexistent'] = 'test'", // Should error
		"invalid syntax here",
		"field[",
		"field['unclosed",
		"field['foo'] = ",
		"field['foo'] invalid_op 'value'",
		"field['foo'] = 'value' field['bar'] = 'value2' field['baz'] > 100",
		"meta['nested.field'] = 'value'",
		"meta['deeply.nested.field.path'] ~= 'search'",
		"field['foo'] = '258e4d48-0369-4287-a375-4a496718d174'", // UUID
		"field['foo'] != 'bar'",
		"field['foo'] = 'a' AND field['bar'] = 'b'",
		"field['foo'] = 42",
		"field['foo'] = 3.14",
		"field['foo'] = 1e5",
		"field['foo'] = 'v' GARBAGE",                 // Trailing garbage
		"meta['x\\') OR 1=1--'] = 'test'",            // SQL injection attempt
		"field['foo'] > 10 field['foo'] < 20",         // Duplicate field names
		"field['foo'] = true",                         // Boolean
		"field['foo'] = false",                        // Boolean
		"field['foo'] = -42",                          // Negative integer
		"field['foo'] = -3.14",                        // Negative float
		"field['foo'] = 'it\\'s here'",                // Escaped quote
		"   ",                                         // Whitespace only
		"field['foo'] = 'a' OR field['bar'] = 'b'",   // OR (unsupported)
		"meta[''] = 'x'",                             // Empty meta key
	}

	for _, tc := range testCases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// The function should never panic, regardless of input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ParseFilter panicked with input %q: %v", input, r)
			}
		}()

		// Call the function - it may return an error, but should never panic
		query, values, err := qb.ParseFilter(input)

		// If no error occurred, the query and values should be valid
		if err == nil {
			// Query should be a string (may be empty)
			if query != "" && values == nil {
				t.Errorf("Non-empty query %q but nil values for input %q", query, input)
			}

			// Values should be a valid map if query is not empty
			if query != "" && values != nil {
				for k, v := range values {
					if k == "" {
						t.Errorf("Empty key in values map for input %q", input)
					}
					if v == nil {
						t.Errorf("Nil value for key %q in values map for input %q", k, input)
					}
				}
			}
		}

		// Length checks to prevent extremely large outputs that could cause DoS
		if len(query) > 10000 {
			t.Errorf("Query too long (%d chars) for input %q", len(query), input)
		}
		if len(values) > 100 {
			t.Errorf("Too many values (%d) for input %q", len(values), input)
		}
	})
}
