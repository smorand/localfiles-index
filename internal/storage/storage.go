package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bunslog"

	"localfiles-index/internal/storage/migrations"
)

// Store provides all database operations for the application.
type Store struct {
	db *bun.DB
}

// New creates a new Store connected to the given database URL.
func New(ctx context.Context, databaseURL string, logger *slog.Logger) (*Store, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(databaseURL)))

	db := bun.NewDB(sqldb, pgdialect.New())

	if logger.Handler().Enabled(ctx, slog.LevelDebug) {
		db.AddQueryHook(bunslog.NewQueryHook(
			bunslog.WithLogger(logger),
			bunslog.WithQueryLogLevel(slog.LevelDebug),
		))
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	return &Store{db: db}, nil
}

// Migrate runs all pending database migrations.
func (s *Store) Migrate(ctx context.Context) error {
	migrator := migrations.NewMigrator(s.db)
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("initializing migrator: %w", err)
	}

	group, err := migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	if group.IsZero() {
		slog.Info("no new migrations to run")
	} else {
		slog.Info("migrations applied", "group", group)
	}

	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// --- Category Operations ---

// CreateCategory creates a new category.
func (s *Store) CreateCategory(ctx context.Context, name, description string) (*Category, error) {
	cat := &Category{
		Name:        name,
		Description: description,
	}

	_, err := s.db.NewInsert().Model(cat).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating category: %w", err)
	}

	return cat, nil
}

// GetCategoryByName returns a category by name.
func (s *Store) GetCategoryByName(ctx context.Context, name string) (*Category, error) {
	cat := new(Category)
	err := s.db.NewSelect().Model(cat).Where("name = ?", name).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting category by name: %w", err)
	}
	return cat, nil
}

// ListCategories returns all categories.
func (s *Store) ListCategories(ctx context.Context) ([]*Category, error) {
	var cats []*Category
	err := s.db.NewSelect().Model(&cats).OrderExpr("name ASC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	return cats, nil
}

// UpdateCategory updates a category's description.
func (s *Store) UpdateCategory(ctx context.Context, name, description string) (*Category, error) {
	cat := new(Category)
	res, err := s.db.NewUpdate().
		Model(cat).
		Set("description = ?", description).
		Where("name = ?", name).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("updating category: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("category not found: %s", name)
	}

	return cat, nil
}

// DeleteCategory deletes a category. If documents reference it, newCategory must be provided
// to migrate them. If no documents reference the category, it is deleted directly.
func (s *Store) DeleteCategory(ctx context.Context, name string, newCategory string) error {
	cat, err := s.GetCategoryByName(ctx, name)
	if err != nil {
		return fmt.Errorf("category not found: %s", name)
	}

	// Check for documents referencing this category
	count, err := s.db.NewSelect().Model((*Document)(nil)).Where("category_id = ?", cat.ID).Count(ctx)
	if err != nil {
		return fmt.Errorf("checking category references: %w", err)
	}

	if count > 0 && newCategory == "" {
		return fmt.Errorf("category has %d documents referencing it, use --new-category to migrate them", count)
	}

	if count > 0 {
		// Migrate documents to the new category
		targetCat, err := s.GetCategoryByName(ctx, newCategory)
		if err != nil {
			return fmt.Errorf("target category not found: %s", newCategory)
		}

		_, err = s.db.NewUpdate().
			Model((*Document)(nil)).
			Set("category_id = ?", targetCat.ID).
			Where("category_id = ?", cat.ID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("migrating documents to category %s: %w", newCategory, err)
		}
	}

	_, err = s.db.NewDelete().Model((*Category)(nil)).Where("id = ?", cat.ID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting category: %w", err)
	}

	return nil
}

// CategoryDocumentCount returns the number of documents in a category.
func (s *Store) CategoryDocumentCount(ctx context.Context, categoryID uuid.UUID) (int, error) {
	count, err := s.db.NewSelect().Model((*Document)(nil)).Where("category_id = ?", categoryID).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting category documents: %w", err)
	}
	return count, nil
}

// --- Document Operations ---

// CreateDocument creates a new document record.
func (s *Store) CreateDocument(ctx context.Context, doc *Document) error {
	_, err := s.db.NewInsert().Model(doc).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating document: %w", err)
	}
	return nil
}

// GetDocumentByPath returns a document by file path.
func (s *Store) GetDocumentByPath(ctx context.Context, filePath string) (*Document, error) {
	doc := new(Document)
	err := s.db.NewSelect().
		Model(doc).
		Relation("Category").
		Where("d.file_path = ?", filePath).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting document by path: %w", err)
	}
	return doc, nil
}

// GetDocumentByID returns a document by UUID.
func (s *Store) GetDocumentByID(ctx context.Context, id uuid.UUID) (*Document, error) {
	doc := new(Document)
	err := s.db.NewSelect().
		Model(doc).
		Relation("Category").
		Where("d.id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting document by id: %w", err)
	}
	return doc, nil
}

// GetDocumentWithChunks returns a document with all its chunks.
func (s *Store) GetDocumentWithChunks(ctx context.Context, id uuid.UUID) (*Document, error) {
	doc := new(Document)
	err := s.db.NewSelect().
		Model(doc).
		Relation("Category").
		Relation("Chunks", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.OrderExpr("ch.chunk_index ASC")
		}).
		Relation("Images").
		Where("d.id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting document with chunks: %w", err)
	}
	return doc, nil
}

// ListDocuments returns all documents.
func (s *Store) ListDocuments(ctx context.Context) ([]*Document, error) {
	var docs []*Document
	err := s.db.NewSelect().
		Model(&docs).
		Relation("Category").
		OrderExpr("d.created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing documents: %w", err)
	}
	return docs, nil
}

// UpdateDocument updates an existing document (replaces all data).
func (s *Store) UpdateDocument(ctx context.Context, doc *Document) error {
	_, err := s.db.NewUpdate().
		Model(doc).
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("updating document: %w", err)
	}
	return nil
}

// DeleteDocument deletes a document and all related data (cascading).
func (s *Store) DeleteDocument(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().
		Model((*Document)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting document: %w", err)
	}
	return nil
}

// DeleteDocumentChunks deletes all chunks for a document.
func (s *Store) DeleteDocumentChunks(ctx context.Context, docID uuid.UUID) error {
	_, err := s.db.NewDelete().
		Model((*Chunk)(nil)).
		Where("document_id = ?", docID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting document chunks: %w", err)
	}
	return nil
}

// DeleteDocumentImages deletes all images for a document.
func (s *Store) DeleteDocumentImages(ctx context.Context, docID uuid.UUID) error {
	_, err := s.db.NewDelete().
		Model((*Image)(nil)).
		Where("document_id = ?", docID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting document images: %w", err)
	}
	return nil
}

// --- Chunk Operations ---

// CreateChunk creates a new chunk record.
func (s *Store) CreateChunk(ctx context.Context, chunk *Chunk) error {
	_, err := s.db.NewInsert().Model(chunk).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating chunk: %w", err)
	}
	return nil
}

// CreateChunks bulk-inserts multiple chunks.
func (s *Store) CreateChunks(ctx context.Context, chunks []*Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	_, err := s.db.NewInsert().Model(&chunks).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating chunks: %w", err)
	}
	return nil
}

// GetChunksByDocumentID returns all chunks for a document.
func (s *Store) GetChunksByDocumentID(ctx context.Context, docID uuid.UUID) ([]*Chunk, error) {
	var chunks []*Chunk
	err := s.db.NewSelect().
		Model(&chunks).
		Where("document_id = ?", docID).
		OrderExpr("chunk_index ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting chunks by document: %w", err)
	}
	return chunks, nil
}

// --- Image Operations ---

// CreateImage creates a new image record.
func (s *Store) CreateImage(ctx context.Context, img *Image) error {
	_, err := s.db.NewInsert().Model(img).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating image: %w", err)
	}
	return nil
}

// --- Search Operations ---

// SemanticSearch performs vector similarity search.
func (s *Store) SemanticSearch(ctx context.Context, queryEmbedding []float32, limit int, categoryName string) ([]*SearchResult, error) {
	vec := pgvector.NewVector(queryEmbedding)

	q := s.db.NewSelect().
		TableExpr("chunks AS ch").
		Join("JOIN documents AS d ON d.id = ch.document_id").
		Join("LEFT JOIN categories AS c ON c.id = d.category_id").
		ColumnExpr("d.id AS document_id").
		ColumnExpr("d.title").
		ColumnExpr("d.file_path").
		ColumnExpr("d.document_type").
		ColumnExpr("ch.content AS excerpt").
		ColumnExpr("ch.chunk_type").
		ColumnExpr("ch.chunk_label").
		ColumnExpr("ch.source_page").
		ColumnExpr("c.name AS category_name").
		ColumnExpr("1 - (ch.embedding <=> ?) AS similarity", vec).
		Where("ch.embedding IS NOT NULL").
		OrderExpr("ch.embedding <=> ? ASC", vec).
		Limit(limit)

	if categoryName != "" {
		q = q.Where("c.name = ?", categoryName)
	}

	var results []*SearchResult
	err := q.Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("semantic search: %w", err)
	}

	return results, nil
}

// FulltextSearch performs full-text keyword search.
func (s *Store) FulltextSearch(ctx context.Context, query string, limit int, categoryName string) ([]*SearchResult, error) {
	q := s.db.NewSelect().
		TableExpr("chunks AS ch").
		Join("JOIN documents AS d ON d.id = ch.document_id").
		Join("LEFT JOIN categories AS c ON c.id = d.category_id").
		ColumnExpr("d.id AS document_id").
		ColumnExpr("d.title").
		ColumnExpr("d.file_path").
		ColumnExpr("d.document_type").
		ColumnExpr("ch.content AS excerpt").
		ColumnExpr("ch.chunk_type").
		ColumnExpr("ch.chunk_label").
		ColumnExpr("ch.source_page").
		ColumnExpr("c.name AS category_name").
		ColumnExpr("ts_rank(ch.search_vector, plainto_tsquery('simple', ?)) AS similarity", query).
		Where("ch.search_vector @@ plainto_tsquery('simple', ?)", query).
		OrderExpr("ts_rank(ch.search_vector, plainto_tsquery('simple', ?)) DESC", query).
		Limit(limit)

	if categoryName != "" {
		q = q.Where("c.name = ?", categoryName)
	}

	var results []*SearchResult
	err := q.Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("fulltext search: %w", err)
	}

	return results, nil
}

// SearchResult represents a single search result.
type SearchResult struct {
	DocumentID   uuid.UUID `bun:"document_id" json:"document_id"`
	Title        string    `bun:"title" json:"title"`
	FilePath     string    `bun:"file_path" json:"file_path"`
	DocumentType string    `bun:"document_type" json:"document_type"`
	Excerpt      string    `bun:"excerpt" json:"excerpt"`
	ChunkType    string    `bun:"chunk_type" json:"chunk_type"`
	ChunkLabel   string    `bun:"chunk_label" json:"chunk_label"`
	SourcePage   *int      `bun:"source_page" json:"source_page"`
	CategoryName string    `bun:"category_name" json:"category_name"`
	Similarity   float64   `bun:"similarity" json:"similarity"`
}

// --- Statistics ---

// IndexStats holds index statistics.
type IndexStats struct {
	TotalDocuments int            `json:"total_documents"`
	TotalChunks    int            `json:"total_chunks"`
	ByType         map[string]int `json:"by_type"`
	ByCategory     map[string]int `json:"by_category"`
}

// GetStats returns index statistics.
func (s *Store) GetStats(ctx context.Context) (*IndexStats, error) {
	stats := &IndexStats{
		ByType:     make(map[string]int),
		ByCategory: make(map[string]int),
	}

	// Total documents
	var err error
	stats.TotalDocuments, err = s.db.NewSelect().Model((*Document)(nil)).Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("counting documents: %w", err)
	}

	// Total chunks
	stats.TotalChunks, err = s.db.NewSelect().Model((*Chunk)(nil)).Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("counting chunks: %w", err)
	}

	// By type
	var typeRows []struct {
		DocumentType string `bun:"document_type"`
		Count        int    `bun:"count"`
	}
	err = s.db.NewSelect().
		Model((*Document)(nil)).
		ColumnExpr("document_type").
		ColumnExpr("count(*) AS count").
		GroupExpr("document_type").
		Scan(ctx, &typeRows)
	if err != nil {
		return nil, fmt.Errorf("counting by type: %w", err)
	}
	for _, row := range typeRows {
		stats.ByType[row.DocumentType] = row.Count
	}

	// By category
	var catRows []struct {
		Name  string `bun:"name"`
		Count int    `bun:"count"`
	}
	err = s.db.NewSelect().
		TableExpr("documents AS d").
		Join("LEFT JOIN categories AS c ON c.id = d.category_id").
		ColumnExpr("COALESCE(c.name, 'uncategorized') AS name").
		ColumnExpr("count(*) AS count").
		GroupExpr("c.name").
		Scan(ctx, &catRows)
	if err != nil {
		return nil, fmt.Errorf("counting by category: %w", err)
	}
	for _, row := range catRows {
		stats.ByCategory[row.Name] = row.Count
	}

	return stats, nil
}

// DB returns the underlying bun.DB for advanced queries.
func (s *Store) DB() *bun.DB {
	return s.db
}
