package indexer

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"localfiles-index/internal/analyzer"
	"localfiles-index/internal/config"
	"localfiles-index/internal/embedding"
	"localfiles-index/internal/storage"
)

// Indexer orchestrates the file indexing pipeline.
type Indexer struct {
	store    *storage.Store
	analyzer *analyzer.Analyzer
	embedder *embedding.Embedder
	cfg      *config.Config
}

// New creates a new Indexer.
func New(store *storage.Store, analyzer *analyzer.Analyzer, embedder *embedding.Embedder, cfg *config.Config) *Indexer {
	return &Indexer{
		store:    store,
		analyzer: analyzer,
		embedder: embedder,
		cfg:      cfg,
	}
}

// IndexResult holds the result of indexing a file.
type IndexResult struct {
	DocumentID uuid.UUID
	Title      string
	ChunkCount int
	ImageCount int
	Tags       []string
}

// IndexFile indexes a single file.
func (idx *Indexer) IndexFile(ctx context.Context, filePath string, tagNames []string) (*IndexResult, error) {
	// Resolve absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	// Detect file type
	fileInfo, err := DetectFileType(absPath)
	if err != nil {
		return nil, err
	}

	slog.Info("indexing file", "path", absPath, "type", fileInfo.Type, "mime", fileInfo.MimeType)

	// Get file mtime
	stat, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	mtime := stat.ModTime()

	// Check if document already exists
	existingDoc, err := idx.store.GetDocumentByPath(ctx, absPath)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("checking existing document: %w", err)
		}
		existingDoc = nil // Not found — will create below
	}
	if existingDoc != nil {
		// Document exists — re-index: delete old chunks/images
		slog.Info("document already exists, re-indexing", "id", existingDoc.ID)
		if err := idx.store.DeleteDocumentImages(ctx, existingDoc.ID); err != nil {
			return nil, fmt.Errorf("deleting old images: %w", err)
		}
		if err := idx.store.DeleteDocumentChunks(ctx, existingDoc.ID); err != nil {
			return nil, fmt.Errorf("deleting old chunks: %w", err)
		}
	}

	// Process based on file type
	var chunks []*storage.Chunk
	var images []*storage.Image
	var title string
	var titleConfidence float64
	var docType string

	switch fileInfo.Type {
	case FileTypeImage:
		title, titleConfidence, chunks, images, err = idx.indexImage(ctx, absPath)
		docType = "image"
	case FileTypePDF:
		title, titleConfidence, chunks, images, err = idx.indexPDF(ctx, absPath)
		docType = "pdf"
	case FileTypeText:
		title, titleConfidence, chunks, err = idx.indexText(ctx, absPath)
		docType = "text"
	case FileTypeSpreadsheet:
		title, titleConfidence, chunks, err = idx.indexSpreadsheet(ctx, absPath)
		docType = "spreadsheet"
	case FileTypeDocument:
		title, titleConfidence, chunks, images, err = idx.indexDocument(ctx, absPath)
		docType = "other"
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileInfo.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("processing %s: %w", fileInfo.Type, err)
	}

	// Create or update document
	var docID uuid.UUID
	if existingDoc != nil {
		existingDoc.FileMtime = mtime
		existingDoc.Title = title
		existingDoc.TitleConfidence = titleConfidence
		existingDoc.DocumentType = docType
		existingDoc.MimeType = fileInfo.MimeType
		existingDoc.FileSize = fileInfo.Size
		existingDoc.IndexedAt = time.Now()
		existingDoc.UpdatedAt = time.Now()

		if err := idx.store.UpdateDocument(ctx, existingDoc); err != nil {
			return nil, fmt.Errorf("updating document: %w", err)
		}
		docID = existingDoc.ID
	} else {
		doc := &storage.Document{
			FilePath:        absPath,
			FileMtime:       mtime,
			Title:           title,
			TitleConfidence: titleConfidence,
			DocumentType:    docType,
			MimeType:        fileInfo.MimeType,
			FileSize:        fileInfo.Size,
			Metadata:        "{}",
		}

		if err := idx.store.CreateDocument(ctx, doc); err != nil {
			return nil, fmt.Errorf("creating document: %w", err)
		}
		docID = doc.ID
	}

	// Set document ID on all chunks and generate embeddings (batch)
	var texts []string
	for _, chunk := range chunks {
		chunk.DocumentID = docID
		texts = append(texts, chunk.Content)
	}

	embeddings, err := idx.embedder.EmbedDocuments(ctx, texts)
	if err != nil {
		slog.Warn("failed to generate batch embeddings", "error", err)
	} else {
		for i, emb := range embeddings {
			chunks[i].Embedding = emb
		}
	}

	// Bulk insert chunks
	if err := idx.store.CreateChunks(ctx, chunks); err != nil {
		return nil, fmt.Errorf("creating chunks: %w", err)
	}

	// Create images
	for _, img := range images {
		img.DocumentID = docID
		// Link to first chunk if available
		if len(chunks) > 0 {
			img.ChunkID = &chunks[0].ID
		}
		if err := idx.store.CreateImage(ctx, img); err != nil {
			return nil, fmt.Errorf("creating image: %w", err)
		}
	}

	// Auto-tagging: load all tags with non-empty rules and ask LLM
	allTags := make([]string, len(tagNames))
	copy(allTags, tagNames)

	autoTags, err := idx.autoTag(ctx, title, chunks)
	if err != nil {
		slog.Warn("auto-tagging failed, continuing with manual tags only", "error", err)
	} else {
		allTags = mergeUnique(allTags, autoTags)
	}

	// Set tags on document
	if len(allTags) > 0 {
		if err := idx.store.SetDocumentTags(ctx, docID, allTags); err != nil {
			return nil, fmt.Errorf("setting document tags: %w", err)
		}
	}

	slog.Info("file indexed successfully",
		"path", absPath,
		"document_id", docID,
		"title", title,
		"chunks", len(chunks),
		"images", len(images),
		"tags", allTags,
	)

	return &IndexResult{
		DocumentID: docID,
		Title:      title,
		ChunkCount: len(chunks),
		ImageCount: len(images),
		Tags:       allTags,
	}, nil
}

// autoTag loads tag rules and suggests tags using the LLM.
func (idx *Indexer) autoTag(ctx context.Context, title string, chunks []*storage.Chunk) ([]string, error) {
	tags, err := idx.store.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}

	var rules []analyzer.TagRule
	for _, t := range tags {
		if t.Rule != "" {
			rules = append(rules, analyzer.TagRule{Name: t.Name, Rule: t.Rule})
		}
	}

	if len(rules) == 0 {
		return nil, nil
	}

	// Build a description from first chunks
	var desc strings.Builder
	for _, ch := range chunks {
		if ch.ChunkType == "doc_summary" || ch.ChunkType == "doc_title" || ch.ChunkType == "image_segment" {
			desc.WriteString(ch.Content)
			desc.WriteString("\n")
		}
		if desc.Len() > 2000 {
			break
		}
	}

	suggested, err := idx.analyzer.SuggestTags(ctx, title, desc.String(), rules)
	if err != nil {
		return nil, err
	}

	return suggested, nil
}

// mergeUnique merges two string slices, returning unique lowercase values.
func mergeUnique(a, b []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range a {
		lower := strings.ToLower(s)
		if !seen[lower] {
			seen[lower] = true
			result = append(result, lower)
		}
	}
	for _, s := range b {
		lower := strings.ToLower(s)
		if !seen[lower] {
			seen[lower] = true
			result = append(result, lower)
		}
	}
	return result
}

func (idx *Indexer) indexImage(ctx context.Context, path string) (string, float64, []*storage.Chunk, []*storage.Image, error) {
	// Analyze image with Gemini
	result, err := idx.analyzer.AnalyzeImage(ctx, path)
	if err != nil {
		return "", 0, nil, nil, fmt.Errorf("analyzing image: %w", err)
	}

	var chunks []*storage.Chunk
	for i, seg := range result.Segments {
		chunks = append(chunks, &storage.Chunk{
			ChunkIndex: i,
			Content:    seg.Content,
			ChunkType:  "image_segment",
			ChunkLabel: seg.Label,
		})
	}

	images := []*storage.Image{
		{
			ImagePath:   path,
			Description: result.Description,
			ImageType:   result.ContentType,
			Caption:     result.Title,
		},
	}

	return result.Title, result.Confidence, chunks, images, nil
}

func (idx *Indexer) indexPDF(ctx context.Context, path string) (string, float64, []*storage.Chunk, []*storage.Image, error) {
	// Extract text and images using pdf-extractor
	extracted, err := idx.extractPDF(ctx, path)
	if err != nil {
		return "", 0, nil, nil, fmt.Errorf("extracting PDF: %w", err)
	}

	if extracted.Text == "" && len(extracted.Images) == 0 {
		return "", 0, nil, nil, fmt.Errorf("no content to index: PDF is empty")
	}

	var chunks []*storage.Chunk
	var images []*storage.Image
	chunkIdx := 0

	// Generate title and summary from full text
	titleResult, err := idx.analyzer.GenerateTitle(ctx, extracted.Text)
	if err != nil {
		return "", 0, nil, nil, fmt.Errorf("generating title: %w", err)
	}

	summaryResult, err := idx.analyzer.GenerateSummary(ctx, extracted.Text)
	if err != nil {
		return "", 0, nil, nil, fmt.Errorf("generating summary: %w", err)
	}

	// Doc title chunk
	chunks = append(chunks, &storage.Chunk{
		ChunkIndex: chunkIdx,
		Content:    titleResult.Title,
		ChunkType:  "doc_title",
	})
	chunkIdx++

	// Doc summary chunk
	chunks = append(chunks, &storage.Chunk{
		ChunkIndex: chunkIdx,
		Content:    summaryResult,
		ChunkType:  "doc_summary",
	})
	chunkIdx++

	// Split text into page-aware chunks
	pageTexts := splitByPages(extracted.Text)
	for pageNum, pageText := range pageTexts {
		textChunks := ChunkText(pageText, idx.cfg.ChunkSize, idx.cfg.ChunkOverlap)
		for _, tc := range textChunks {
			page := pageNum + 1
			chunks = append(chunks, &storage.Chunk{
				ChunkIndex: chunkIdx,
				Content:    tc.Content,
				ChunkType:  "text",
				SourcePage: &page,
			})
			chunkIdx++
		}
	}

	// Process extracted images
	for _, ei := range extracted.Images {
		imgResult, err := idx.analyzer.AnalyzeImage(ctx, ei.Path)
		if err != nil {
			slog.Warn("failed to analyze extracted image", "path", ei.Path, "error", err)
			continue
		}

		for _, seg := range imgResult.Segments {
			page := ei.PageNumber
			chunks = append(chunks, &storage.Chunk{
				ChunkIndex: chunkIdx,
				Content:    seg.Content,
				ChunkType:  "image_segment",
				ChunkLabel: seg.Label,
				SourcePage: &page,
			})
			chunkIdx++
		}

		images = append(images, &storage.Image{
			ImagePath:   ei.Path,
			Description: imgResult.Description,
			ImageType:   imgResult.ContentType,
			Caption:     imgResult.Title,
			SourcePage:  &ei.PageNumber,
		})
	}

	return titleResult.Title, titleResult.Confidence, chunks, images, nil
}

func (idx *Indexer) indexText(ctx context.Context, path string) (string, float64, []*storage.Chunk, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", 0, nil, fmt.Errorf("reading text file: %w", err)
	}

	text := string(content)
	if strings.TrimSpace(text) == "" {
		return "", 0, nil, fmt.Errorf("file is empty: %s", path)
	}

	var chunks []*storage.Chunk
	chunkIdx := 0

	// Generate title and summary
	titleResult, err := idx.analyzer.GenerateTitle(ctx, text)
	if err != nil {
		return "", 0, nil, fmt.Errorf("generating title: %w", err)
	}

	summaryResult, err := idx.analyzer.GenerateSummary(ctx, text)
	if err != nil {
		return "", 0, nil, fmt.Errorf("generating summary: %w", err)
	}

	// Doc title chunk
	chunks = append(chunks, &storage.Chunk{
		ChunkIndex: chunkIdx,
		Content:    titleResult.Title,
		ChunkType:  "doc_title",
	})
	chunkIdx++

	// Doc summary chunk
	chunks = append(chunks, &storage.Chunk{
		ChunkIndex: chunkIdx,
		Content:    summaryResult,
		ChunkType:  "doc_summary",
	})
	chunkIdx++

	// Text chunks
	textChunks := ChunkText(text, idx.cfg.ChunkSize, idx.cfg.ChunkOverlap)
	for _, tc := range textChunks {
		chunks = append(chunks, &storage.Chunk{
			ChunkIndex: chunkIdx,
			Content:    tc.Content,
			ChunkType:  "text",
		})
		chunkIdx++
	}

	return titleResult.Title, titleResult.Confidence, chunks, nil
}

func (idx *Indexer) indexSpreadsheet(ctx context.Context, path string) (string, float64, []*storage.Chunk, error) {
	ext := strings.ToLower(filepath.Ext(path))

	var content string
	var err error

	switch ext {
	case ".csv":
		content, err = readCSV(path)
	case ".xlsx":
		content, err = readXLSX(path)
	default:
		return "", 0, nil, fmt.Errorf("unsupported spreadsheet format: %s", ext)
	}

	if err != nil {
		return "", 0, nil, fmt.Errorf("reading spreadsheet: %w", err)
	}

	if strings.TrimSpace(content) == "" {
		return "", 0, nil, fmt.Errorf("spreadsheet is empty: %s", path)
	}

	// Analyze spreadsheet content
	descResult, err := idx.analyzer.DescribeSpreadsheet(ctx, content)
	if err != nil {
		return "", 0, nil, fmt.Errorf("analyzing spreadsheet: %w", err)
	}

	var chunks []*storage.Chunk
	chunkIdx := 0

	// Doc title chunk
	chunks = append(chunks, &storage.Chunk{
		ChunkIndex: chunkIdx,
		Content:    descResult.Title,
		ChunkType:  "doc_title",
	})
	chunkIdx++

	// Doc summary chunk
	chunks = append(chunks, &storage.Chunk{
		ChunkIndex: chunkIdx,
		Content:    descResult.Summary,
		ChunkType:  "doc_summary",
	})
	chunkIdx++

	// Full description chunk
	chunks = append(chunks, &storage.Chunk{
		ChunkIndex: chunkIdx,
		Content:    descResult.Description,
		ChunkType:  "text",
	})

	return descResult.Title, descResult.Confidence, chunks, nil
}

func (idx *Indexer) indexDocument(ctx context.Context, path string) (string, float64, []*storage.Chunk, []*storage.Image, error) {
	// Convert to PDF using LibreOffice
	tmpDir, err := os.MkdirTemp("", "localfiles-convert-*")
	if err != nil {
		return "", 0, nil, nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.CommandContext(ctx, "soffice", "--headless", "--convert-to", "pdf", "--outdir", tmpDir, path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", 0, nil, nil, fmt.Errorf("converting document to PDF: %w\noutput: %s", err, string(output))
	}

	// Find the converted PDF
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + ".pdf"
	pdfPath := filepath.Join(tmpDir, base)

	if _, err := os.Stat(pdfPath); err != nil {
		return "", 0, nil, nil, fmt.Errorf("converted PDF not found: %s", pdfPath)
	}

	// Process as PDF
	return idx.indexPDF(ctx, pdfPath)
}

// readCSV reads a CSV file and returns its content as a string.
func readCSV(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening CSV: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return "", fmt.Errorf("reading CSV: %w", err)
	}

	var sb strings.Builder
	for _, record := range records {
		sb.WriteString(strings.Join(record, " | "))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// readXLSX reads an XLSX file and returns its content as a string.
func readXLSX(path string) (string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return "", fmt.Errorf("opening XLSX: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	for _, sheet := range f.GetSheetList() {
		sb.WriteString(fmt.Sprintf("Sheet: %s\n", sheet))
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		for _, row := range rows {
			sb.WriteString(strings.Join(row, " | "))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// ExtractedPDF holds the result of PDF text/image extraction.
type ExtractedPDF struct {
	Text   string
	Images []ExtractedImage
}

// ExtractedImage holds information about an extracted image.
type ExtractedImage struct {
	Path       string
	PageNumber int
}

// extractPDF runs pdf-extractor and parses the output.
func (idx *Indexer) extractPDF(ctx context.Context, path string) (*ExtractedPDF, error) {
	cmd := exec.CommandContext(ctx, idx.cfg.PDFExtractorPath, path)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running pdf-extractor: %w", err)
	}

	return parsePDFExtractorOutput(string(output), path)
}

// splitByPages splits text by "--- page N ---" markers.
func splitByPages(text string) []string {
	lines := strings.Split(text, "\n")
	var pages []string
	var currentPage strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--- page ") && strings.HasSuffix(trimmed, " ---") {
			if currentPage.Len() > 0 {
				pages = append(pages, currentPage.String())
				currentPage.Reset()
			}
			continue
		}
		if trimmed == "---" {
			continue
		}
		currentPage.WriteString(line)
		currentPage.WriteString("\n")
	}

	if currentPage.Len() > 0 {
		pages = append(pages, currentPage.String())
	}

	// If no page markers found, return the whole text as one page
	if len(pages) == 0 && strings.TrimSpace(text) != "" {
		pages = append(pages, text)
	}

	return pages
}
