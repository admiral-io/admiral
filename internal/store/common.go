package store

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// JoinSpec describes one LEFT JOIN enrichment applied to a list/get query.
// Use ActorJoin or NameJoin to construct one rather than building by hand.
type JoinSpec struct {
	// FkColumn is the column on the queried table whose value matches the
	// parent's id (e.g. "created_by", "application_id").
	FkColumn string
	// ParentTable is the table the FK points at (e.g. "users", "applications").
	ParentTable string
	// Columns are the parent columns to surface on the result row, each with
	// the alias to read them back into the model.
	Columns []JoinedColumn
}

// JoinedColumn names a parent column to select and the alias it should be
// returned as. The alias maps directly to a `gorm:"->;column:<alias>"` field
// on the model.
type JoinedColumn struct {
	Source string // column on the parent table (e.g. "name", "email")
	As     string // alias on the result row (e.g. "created_by_name")
}

// ActorJoin builds a JoinSpec that resolves an actor reference column into
// display name and email columns aliased as "<column>_name" / "<column>_email".
// For non-user actors the LEFT JOIN returns NULLs, which Gorm scans as the
// zero string — the ActorRef keeps its ID with empty DisplayName/Email.
func ActorJoin(column string) JoinSpec {
	return JoinSpec{
		FkColumn:    column,
		ParentTable: "users",
		Columns: []JoinedColumn{
			{Source: "name", As: column + "_name"},
			{Source: "email", As: column + "_email"},
		},
	}
}

// NameJoin builds a JoinSpec that resolves a foreign-key column into a single
// "<fkColumn>_name" alias by selecting `name` from the parent table. Use this
// to denormalize a parent's display name onto a list/get response without
// requiring the CLI to make follow-up calls.
func NameJoin(fkColumn, parentTable string) JoinSpec {
	return JoinSpec{
		FkColumn:    fkColumn,
		ParentTable: parentTable,
		Columns:     []JoinedColumn{{Source: "name", As: fkColumn + "_name"}},
	}
}

// WithEnrichment returns a Gorm scope that LEFT JOINs each parent table
// described by `joins` and emits a single SELECT with the queried table's
// columns plus the aliased parent columns. Each join uses a distinct alias
// derived from its FkColumn ("<fkColumn>_join") so multiple specs compose
// safely.
//
// The join predicate casts both sides to text so the same scope works whether
// the FK column on the queried table is UUID-typed (e.g. application_id) or
// TEXT-typed (e.g. created_by, where the canonical id format is a UUID string
// but the column is text to also accept synthetic actor IDs).
func WithEnrichment(table string, joins ...JoinSpec) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		sel := []string{table + ".*"}
		d := db
		for _, j := range joins {
			alias := j.FkColumn + "_join"
			d = d.Joins(fmt.Sprintf(
				"LEFT JOIN %s AS %s ON %s.id::text = %s.%s::text AND %s.deleted_at IS NULL",
				j.ParentTable, alias, alias, table, j.FkColumn, alias,
			))
			for _, c := range j.Columns {
				sel = append(sel, fmt.Sprintf("%s.%s AS %s", alias, c.Source, c.As))
			}
		}
		return d.Select(strings.Join(sel, ", "))
	}
}

// WithActorRef is shorthand for WithEnrichment(table, ActorJoin(column)).
// Kept for backward compatibility with stores that need only actor enrichment;
// new sites that combine actor + FK denorms should call WithEnrichment with
// the join specs directly so all columns land in a single SELECT.
func WithActorRef(table, column string) func(*gorm.DB) *gorm.DB {
	return WithEnrichment(table, ActorJoin(column))
}
