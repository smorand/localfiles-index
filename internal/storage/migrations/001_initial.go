package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

var Migrations = migrate.NewMigrations()

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Enable pgvector extension
		if _, err := db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
			return fmt.Errorf("creating pgvector extension: %w", err)
		}

		// Create categories table
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS categories (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name TEXT UNIQUE NOT NULL,
				description TEXT DEFAULT '',
				created_at TIMESTAMP NOT NULL DEFAULT NOW()
			)
		`); err != nil {
			return fmt.Errorf("creating categories table: %w", err)
		}

		// Create documents table
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS documents (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				file_path TEXT UNIQUE NOT NULL,
				file_mtime TIMESTAMP NOT NULL,
				title TEXT NOT NULL,
				title_confidence REAL,
				document_type TEXT NOT NULL,
				category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
				mime_type TEXT,
				file_size BIGINT,
				metadata JSONB DEFAULT '{}',
				indexed_at TIMESTAMP NOT NULL DEFAULT NOW(),
				created_at TIMESTAMP NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMP NOT NULL DEFAULT NOW()
			)
		`); err != nil {
			return fmt.Errorf("creating documents table: %w", err)
		}

		// Create chunks table with vector and tsvector columns
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS chunks (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
				chunk_index INTEGER NOT NULL,
				content TEXT NOT NULL,
				chunk_type TEXT NOT NULL,
				chunk_label TEXT,
				source_page INTEGER,
				embedding vector(768),
				search_vector tsvector,
				created_at TIMESTAMP NOT NULL DEFAULT NOW()
			)
		`); err != nil {
			return fmt.Errorf("creating chunks table: %w", err)
		}

		// Create images table
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS images (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
				chunk_id UUID REFERENCES chunks(id) ON DELETE SET NULL,
				image_path TEXT NOT NULL,
				description TEXT,
				image_type TEXT,
				caption TEXT,
				source_page INTEGER,
				created_at TIMESTAMP NOT NULL DEFAULT NOW()
			)
		`); err != nil {
			return fmt.Errorf("creating images table: %w", err)
		}

		// Create indexes
		if _, err := db.ExecContext(ctx, `
			CREATE INDEX IF NOT EXISTS idx_chunks_embedding ON chunks USING hnsw (embedding vector_cosine_ops);
			CREATE INDEX IF NOT EXISTS idx_chunks_search_vector ON chunks USING gin (search_vector);
			CREATE INDEX IF NOT EXISTS idx_documents_category ON documents (category_id);
			CREATE INDEX IF NOT EXISTS idx_documents_file_path ON documents (file_path);
			CREATE INDEX IF NOT EXISTS idx_chunks_document ON chunks (document_id);
		`); err != nil {
			return fmt.Errorf("creating indexes: %w", err)
		}

		// Create trigger for auto-generating search_vector
		if _, err := db.ExecContext(ctx, `
			CREATE OR REPLACE FUNCTION update_search_vector() RETURNS trigger AS $$
			BEGIN
				NEW.search_vector := to_tsvector('simple', NEW.content);
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql;

			DROP TRIGGER IF EXISTS trg_update_search_vector ON chunks;
			CREATE TRIGGER trg_update_search_vector
				BEFORE INSERT OR UPDATE ON chunks
				FOR EACH ROW
				EXECUTE FUNCTION update_search_vector();
		`); err != nil {
			return fmt.Errorf("creating search_vector trigger: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Down migration
		if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS images CASCADE`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS chunks CASCADE`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS documents CASCADE`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS categories CASCADE`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `DROP FUNCTION IF EXISTS update_search_vector() CASCADE`); err != nil {
			return err
		}
		return nil
	})
}
