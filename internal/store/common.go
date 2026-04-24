package store

import (
	"fmt"

	"gorm.io/gorm"
)

// WithActorRef returns a Gorm scope that LEFT JOINs the users table to resolve
// an actor reference column (e.g. "created_by", "triggered_by") into a display
// name and email. The joined columns are aliased as "<column>_name" and
// "<column>_email" and mapped to read-only fields on the model struct.
//
// For non-user actors (e.g. service tokens) the LEFT JOIN returns NULLs, which
// Gorm scans as zero-value strings — the ActorRef keeps its ID with empty
// DisplayName/Email, which is correct.
func WithActorRef(table, column string) func(*gorm.DB) *gorm.DB {
	alias := column + "_user"
	return func(db *gorm.DB) *gorm.DB {
		join := fmt.Sprintf(
			"LEFT JOIN users AS %s ON %s.id::text = %s.%s AND %s.deleted_at IS NULL",
			alias, alias, table, column, alias,
		)
		sel := fmt.Sprintf(
			"%s.*, %s.name AS %s_name, %s.email AS %s_email",
			table, alias, column, alias, column,
		)
		return db.Joins(join).Select(sel)
	}
}
