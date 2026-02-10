package migrations

import (
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// NewMigrator creates a new migrator with all registered migrations.
func NewMigrator(db *bun.DB) *migrate.Migrator {
	return migrate.NewMigrator(db, Migrations)
}
