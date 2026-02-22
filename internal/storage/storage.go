package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

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

	// Register junction model for m2m relationships
	db.RegisterModel((*DocumentTag)(nil))

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

// --- Tag Operations ---

// CreateTag creates a new tag.
func (s *Store) CreateTag(ctx context.Context, name, description string) (*Tag, error) {
	tag := &Tag{
		Name:        strings.ToLower(name),
		Description: description,
	}

	_, err := s.db.NewInsert().Model(tag).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating tag: %w", err)
	}

	return tag, nil
}

// GetTagByName returns a tag by name.
func (s *Store) GetTagByName(ctx context.Context, name string) (*Tag, error) {
	tag := new(Tag)
	err := s.db.NewSelect().Model(tag).Where("name = ?", strings.ToLower(name)).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting tag by name: %w", err)
	}
	return tag, nil
}

// GetTagsByNames returns tags matching the given names.
func (s *Store) GetTagsByNames(ctx context.Context, names []string) ([]*Tag, error) {
	lower := make([]string, len(names))
	for i, n := range names {
		lower[i] = strings.ToLower(n)
	}

	var tags []*Tag
	err := s.db.NewSelect().Model(&tags).Where("name IN (?)", bun.In(lower)).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting tags by names: %w", err)
	}
	return tags, nil
}

// ListTags returns all tags.
func (s *Store) ListTags(ctx context.Context) ([]*Tag, error) {
	var tags []*Tag
	err := s.db.NewSelect().Model(&tags).OrderExpr("name ASC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	return tags, nil
}

// UpdateTag updates a tag's description.
func (s *Store) UpdateTag(ctx context.Context, name, description string) (*Tag, error) {
	tag := new(Tag)
	res, err := s.db.NewUpdate().
		Model(tag).
		Set("description = ?", description).
		Where("name = ?", strings.ToLower(name)).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("updating tag: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("tag not found: %s", name)
	}

	return tag, nil
}

// UpdateTagRule updates a tag's auto-tagging rule.
func (s *Store) UpdateTagRule(ctx context.Context, name, rule string) (*Tag, error) {
	tag := new(Tag)
	res, err := s.db.NewUpdate().
		Model(tag).
		Set("rule = ?", rule).
		Where("name = ?", strings.ToLower(name)).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("updating tag rule: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("tag not found: %s", name)
	}

	return tag, nil
}

// DeleteTag deletes a tag and unlinks all documents from it.
func (s *Store) DeleteTag(ctx context.Context, name string) error {
	tag, err := s.GetTagByName(ctx, name)
	if err != nil {
		return fmt.Errorf("tag not found: %s", name)
	}

	// Delete tag — CASCADE will remove document_tags entries
	_, err = s.db.NewDelete().Model((*Tag)(nil)).Where("id = ?", tag.ID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting tag: %w", err)
	}

	return nil
}

// MergeTag moves all documents from source tag to target tag, then deletes source.
func (s *Store) MergeTag(ctx context.Context, sourceName, targetName string) error {
	source, err := s.GetTagByName(ctx, sourceName)
	if err != nil {
		return fmt.Errorf("source tag not found: %s", sourceName)
	}

	target, err := s.GetTagByName(ctx, targetName)
	if err != nil {
		return fmt.Errorf("target tag not found: %s", targetName)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Move document_tags from source to target (skip duplicates)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO document_tags (document_id, tag_id, created_at)
		SELECT document_id, ?, NOW()
		FROM document_tags
		WHERE tag_id = ?
		ON CONFLICT DO NOTHING
	`, target.ID, source.ID)
	if err != nil {
		return fmt.Errorf("merging document_tags: %w", err)
	}

	// Delete source tag (CASCADE removes its document_tags entries)
	_, err = tx.NewDelete().Model((*Tag)(nil)).Where("id = ?", source.ID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting source tag: %w", err)
	}

	return tx.Commit()
}

// TagDocumentCount returns the number of documents with a given tag.
func (s *Store) TagDocumentCount(ctx context.Context, tagID uuid.UUID) (int, error) {
	count, err := s.db.NewSelect().
		Model((*DocumentTag)(nil)).
		Where("tag_id = ?", tagID).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting tag documents: %w", err)
	}
	return count, nil
}

// --- Document-Tag Operations ---

// SetDocumentTags replaces all tags on a document (auto-creates missing tags).
func (s *Store) SetDocumentTags(ctx context.Context, docID uuid.UUID, tagNames []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Remove existing tags
	_, err = tx.NewDelete().Model((*DocumentTag)(nil)).Where("document_id = ?", docID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("removing existing tags: %w", err)
	}

	if len(tagNames) == 0 {
		return tx.Commit()
	}

	// Ensure all tags exist (auto-create missing ones)
	for _, name := range tagNames {
		lowerName := strings.ToLower(name)
		_, err := tx.NewInsert().
			Model(&Tag{Name: lowerName}).
			On("CONFLICT (name) DO NOTHING").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("ensuring tag exists: %w", err)
		}
	}

	// Get tag IDs
	lower := make([]string, len(tagNames))
	for i, n := range tagNames {
		lower[i] = strings.ToLower(n)
	}

	var tags []*Tag
	err = tx.NewSelect().Model(&tags).Where("name IN (?)", bun.In(lower)).Scan(ctx)
	if err != nil {
		return fmt.Errorf("fetching tag IDs: %w", err)
	}

	// Insert document_tags
	dts := make([]*DocumentTag, 0, len(tags))
	for _, tag := range tags {
		dts = append(dts, &DocumentTag{
			DocumentID: docID,
			TagID:      tag.ID,
		})
	}

	if len(dts) > 0 {
		_, err = tx.NewInsert().Model(&dts).Exec(ctx)
		if err != nil {
			return fmt.Errorf("inserting document_tags: %w", err)
		}
	}

	return tx.Commit()
}

// GetDocumentTags returns all tags for a document.
func (s *Store) GetDocumentTags(ctx context.Context, docID uuid.UUID) ([]*Tag, error) {
	var tags []*Tag
	err := s.db.NewSelect().
		Model(&tags).
		Join("JOIN document_tags AS dt ON dt.tag_id = t.id").
		Where("dt.document_id = ?", docID).
		OrderExpr("t.name ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting document tags: %w", err)
	}
	return tags, nil
}

// --- Document Operations ---

// CreateDocument creates a new document record.
func (s *Store) CreateDocument(ctx context.Context, doc *Document) error {
	_, err := s.db.NewInsert().Model(doc).Returning("*").Exec(ctx)
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
		Relation("Tags").
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
		Relation("Tags").
		Where("d.id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting document by id: %w", err)
	}
	return doc, nil
}

// GetDocumentByIDPrefix returns a document matching a UUID prefix (minimum 8 chars).
// Returns an error if the prefix matches zero or more than one document.
func (s *Store) GetDocumentByIDPrefix(ctx context.Context, prefix string) (*Document, error) {
	var docs []*Document
	err := s.db.NewSelect().
		Model(&docs).
		Relation("Tags").
		Where("d.id::text LIKE ?", prefix+"%").
		Limit(2).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("searching document by id prefix: %w", err)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("no document found with id prefix: %s", prefix)
	}
	if len(docs) > 1 {
		return nil, fmt.Errorf("ambiguous id prefix %s: matches multiple documents", prefix)
	}
	return docs[0], nil
}

// GetDocumentWithChunks returns a document with all its chunks.
func (s *Store) GetDocumentWithChunks(ctx context.Context, id uuid.UUID) (*Document, error) {
	doc := new(Document)
	err := s.db.NewSelect().
		Model(doc).
		Relation("Tags").
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
		Relation("Tags").
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
func (s *Store) SemanticSearch(ctx context.Context, queryEmbedding []float32, limit int, tagNames []string) ([]*SearchResult, error) {
	vec := pgvector.NewVector(queryEmbedding)

	q := s.db.NewSelect().
		TableExpr("chunks AS ch").
		Join("JOIN documents AS d ON d.id = ch.document_id").
		ColumnExpr("d.id AS document_id").
		ColumnExpr("d.title").
		ColumnExpr("d.file_path").
		ColumnExpr("d.document_type").
		ColumnExpr("ch.content AS excerpt").
		ColumnExpr("ch.chunk_type").
		ColumnExpr("ch.chunk_label").
		ColumnExpr("ch.source_page").
		ColumnExpr("(SELECT string_agg(t.name, ', ' ORDER BY t.name) FROM document_tags dt JOIN tags t ON t.id = dt.tag_id WHERE dt.document_id = d.id) AS tag_names").
		ColumnExpr("1 - (ch.embedding <=> ?) AS similarity", vec).
		Where("ch.embedding IS NOT NULL").
		OrderExpr("ch.embedding <=> ? ASC", vec).
		Limit(limit)

	for _, tn := range tagNames {
		q = q.Where("EXISTS (SELECT 1 FROM document_tags dt JOIN tags t ON t.id = dt.tag_id WHERE dt.document_id = d.id AND t.name = ?)", strings.ToLower(tn))
	}

	var results []*SearchResult
	err := q.Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("semantic search: %w", err)
	}

	return results, nil
}

// FulltextSearch performs full-text keyword search.
func (s *Store) FulltextSearch(ctx context.Context, query string, limit int, tagNames []string) ([]*SearchResult, error) {
	q := s.db.NewSelect().
		TableExpr("chunks AS ch").
		Join("JOIN documents AS d ON d.id = ch.document_id").
		ColumnExpr("d.id AS document_id").
		ColumnExpr("d.title").
		ColumnExpr("d.file_path").
		ColumnExpr("d.document_type").
		ColumnExpr("ch.content AS excerpt").
		ColumnExpr("ch.chunk_type").
		ColumnExpr("ch.chunk_label").
		ColumnExpr("ch.source_page").
		ColumnExpr("(SELECT string_agg(t.name, ', ' ORDER BY t.name) FROM document_tags dt JOIN tags t ON t.id = dt.tag_id WHERE dt.document_id = d.id) AS tag_names").
		ColumnExpr("ts_rank(ch.search_vector, plainto_tsquery('simple', ?)) AS similarity", query).
		Where("ch.search_vector @@ plainto_tsquery('simple', ?)", query).
		OrderExpr("ts_rank(ch.search_vector, plainto_tsquery('simple', ?)) DESC", query).
		Limit(limit)

	for _, tn := range tagNames {
		q = q.Where("EXISTS (SELECT 1 FROM document_tags dt JOIN tags t ON t.id = dt.tag_id WHERE dt.document_id = d.id AND t.name = ?)", strings.ToLower(tn))
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
	TagNames     string    `bun:"tag_names" json:"tag_names"`
	Similarity   float64   `bun:"similarity" json:"similarity"`
}

// --- Statistics ---

// IndexStats holds index statistics.
type IndexStats struct {
	TotalDocuments int            `json:"total_documents"`
	TotalChunks    int            `json:"total_chunks"`
	ByType         map[string]int `json:"by_type"`
	ByTag          map[string]int `json:"by_tag"`
}

// GetStats returns index statistics.
func (s *Store) GetStats(ctx context.Context) (*IndexStats, error) {
	stats := &IndexStats{
		ByType: make(map[string]int),
		ByTag:  make(map[string]int),
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

	// By tag
	var tagRows []struct {
		Name  string `bun:"name"`
		Count int    `bun:"count"`
	}
	err = s.db.NewSelect().
		TableExpr("document_tags AS dt").
		Join("JOIN tags AS t ON t.id = dt.tag_id").
		ColumnExpr("t.name AS name").
		ColumnExpr("COUNT(DISTINCT dt.document_id) AS count").
		GroupExpr("t.name").
		Scan(ctx, &tagRows)
	if err != nil {
		return nil, fmt.Errorf("counting by tag: %w", err)
	}
	for _, row := range tagRows {
		stats.ByTag[row.Name] = row.Count
	}

	return stats, nil
}

// DB returns the underlying bun.DB for advanced queries.
func (s *Store) DB() *bun.DB {
	return s.db
}
