package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Rename categories table to tags
		if _, err := db.ExecContext(ctx, `ALTER TABLE categories RENAME TO tags`); err != nil {
			return fmt.Errorf("renaming categories to tags: %w", err)
		}

		// Add rule column for auto-tagging prompts
		if _, err := db.ExecContext(ctx, `ALTER TABLE tags ADD COLUMN rule TEXT DEFAULT ''`); err != nil {
			return fmt.Errorf("adding rule column: %w", err)
		}

		// Create document_tags junction table
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS document_tags (
				document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
				tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
				created_at TIMESTAMP NOT NULL DEFAULT NOW(),
				PRIMARY KEY (document_id, tag_id)
			)
		`); err != nil {
			return fmt.Errorf("creating document_tags table: %w", err)
		}

		// Create indexes on junction table
		if _, err := db.ExecContext(ctx, `
			CREATE INDEX IF NOT EXISTS idx_document_tags_document ON document_tags (document_id);
			CREATE INDEX IF NOT EXISTS idx_document_tags_tag ON document_tags (tag_id);
		`); err != nil {
			return fmt.Errorf("creating document_tags indexes: %w", err)
		}

		// Migrate existing category_id data into document_tags
		if _, err := db.ExecContext(ctx, `
			INSERT INTO document_tags (document_id, tag_id)
			SELECT id, category_id FROM documents WHERE category_id IS NOT NULL
			ON CONFLICT DO NOTHING
		`); err != nil {
			return fmt.Errorf("migrating category data to document_tags: %w", err)
		}

		// Drop the old category_id column and its index
		if _, err := db.ExecContext(ctx, `DROP INDEX IF EXISTS idx_documents_category`); err != nil {
			return fmt.Errorf("dropping category index: %w", err)
		}
		if _, err := db.ExecContext(ctx, `ALTER TABLE documents DROP COLUMN IF EXISTS category_id`); err != nil {
			return fmt.Errorf("dropping category_id column: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Down migration: reverse all steps

		// Re-add category_id column
		if _, err := db.ExecContext(ctx, `ALTER TABLE documents ADD COLUMN category_id UUID REFERENCES tags(id) ON DELETE SET NULL`); err != nil {
			return fmt.Errorf("re-adding category_id column: %w", err)
		}

		// Migrate data back (pick one tag per document)
		if _, err := db.ExecContext(ctx, `
			UPDATE documents d SET category_id = dt.tag_id
			FROM (SELECT DISTINCT ON (document_id) document_id, tag_id FROM document_tags) dt
			WHERE d.id = dt.document_id
		`); err != nil {
			return fmt.Errorf("migrating document_tags back to category_id: %w", err)
		}

		// Re-create index
		if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_documents_category ON documents (category_id)`); err != nil {
			return fmt.Errorf("re-creating category index: %w", err)
		}

		// Drop document_tags table
		if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS document_tags CASCADE`); err != nil {
			return fmt.Errorf("dropping document_tags table: %w", err)
		}

		// Drop rule column
		if _, err := db.ExecContext(ctx, `ALTER TABLE tags DROP COLUMN IF EXISTS rule`); err != nil {
			return fmt.Errorf("dropping rule column: %w", err)
		}

		// Rename tags back to categories
		if _, err := db.ExecContext(ctx, `ALTER TABLE tags RENAME TO categories`); err != nil {
			return fmt.Errorf("renaming tags to categories: %w", err)
		}

		return nil
	})
}
