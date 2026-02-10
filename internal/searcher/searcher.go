package searcher

import (
	"context"
	"fmt"

	"localfiles-index/internal/embedding"
	"localfiles-index/internal/storage"
)

// Searcher performs search operations against the index.
type Searcher struct {
	store    *storage.Store
	embedder *embedding.Embedder
}

// New creates a new Searcher.
func New(store *storage.Store, embedder *embedding.Embedder) *Searcher {
	return &Searcher{
		store:    store,
		embedder: embedder,
	}
}

// Search performs a search based on mode.
func (s *Searcher) Search(ctx context.Context, query string, mode string, category string, limit int) ([]*storage.SearchResult, error) {
	switch mode {
	case "semantic", "":
		return s.semanticSearch(ctx, query, category, limit)
	case "fulltext":
		return s.fulltextSearch(ctx, query, category, limit)
	default:
		return nil, fmt.Errorf("unsupported search mode: %s", mode)
	}
}

func (s *Searcher) semanticSearch(ctx context.Context, query string, category string, limit int) ([]*storage.SearchResult, error) {
	queryEmbedding, err := s.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("generating query embedding: %w", err)
	}

	results, err := s.store.SemanticSearch(ctx, queryEmbedding, limit, category)
	if err != nil {
		return nil, fmt.Errorf("semantic search: %w", err)
	}

	return results, nil
}

func (s *Searcher) fulltextSearch(ctx context.Context, query string, category string, limit int) ([]*storage.SearchResult, error) {
	results, err := s.store.FulltextSearch(ctx, query, limit, category)
	if err != nil {
		return nil, fmt.Errorf("fulltext search: %w", err)
	}

	return results, nil
}
