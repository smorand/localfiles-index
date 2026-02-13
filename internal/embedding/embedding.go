package embedding

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/genai"
)

// Embedder generates text embeddings using Gemini.
type Embedder struct {
	client     *genai.Client
	model      string
	dimensions int
}

// New creates a new Embedder.
func New(ctx context.Context, apiKey string, model string, dimensions int) (*Embedder, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Gemini client: %w", err)
	}

	return &Embedder{
		client:     client,
		model:      model,
		dimensions: dimensions,
	}, nil
}

// EmbedDocument generates an embedding for document content (RETRIEVAL_DOCUMENT task type).
func (e *Embedder) EmbedDocument(ctx context.Context, text string) ([]float32, error) {
	return e.embed(ctx, text, "RETRIEVAL_DOCUMENT")
}

// EmbedQuery generates an embedding for a search query (RETRIEVAL_QUERY task type).
func (e *Embedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return e.embed(ctx, text, "RETRIEVAL_QUERY")
}

// maxEmbeddingInputLen is the maximum character length of text sent for embedding.
const maxEmbeddingInputLen = 8000

func (e *Embedder) embed(ctx context.Context, text string, taskType string) ([]float32, error) {
	slog.Debug("generating embedding", "text_length", len(text), "task_type", taskType)

	// Truncate text if too long for embedding model
	if len(text) > maxEmbeddingInputLen {
		text = text[:maxEmbeddingInputLen]
	}

	dims := int32(e.dimensions)
	result, err := e.client.Models.EmbedContent(ctx, e.model, []*genai.Content{
		{Parts: []*genai.Part{genai.NewPartFromText(text)}},
	}, &genai.EmbedContentConfig{
		TaskType:             taskType,
		OutputDimensionality: &dims,
	})
	if err != nil {
		return nil, fmt.Errorf("embedding API error: %w", err)
	}

	if result == nil || result.Embeddings == nil || len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Embeddings[0].Values, nil
}
