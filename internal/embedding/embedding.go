package embedding

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

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

// EmbedDocuments generates embeddings for multiple texts in a single batch API call.
func (e *Embedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	slog.Debug("generating batch embeddings", "count", len(texts), "task_type", "RETRIEVAL_DOCUMENT")

	var contents []*genai.Content
	for _, text := range texts {
		if len(text) > maxEmbeddingInputLen {
			text = text[:maxEmbeddingInputLen]
		}
		contents = append(contents, &genai.Content{Parts: []*genai.Part{genai.NewPartFromText(text)}})
	}

	dims := int32(e.dimensions)
	result, err := e.embedContent(ctx, contents, &genai.EmbedContentConfig{
		TaskType:             "RETRIEVAL_DOCUMENT",
		OutputDimensionality: &dims,
	})
	if err != nil {
		return nil, fmt.Errorf("batch embedding API error: %w", err)
	}

	if result == nil || result.Embeddings == nil || len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(result.Embeddings))
	}

	embeddings := make([][]float32, len(texts))
	for i, emb := range result.Embeddings {
		embeddings[i] = emb.Values
	}
	return embeddings, nil
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
	result, err := e.embedContent(ctx, []*genai.Content{
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

const maxRetries = 5

func (e *Embedder) embedContent(ctx context.Context, content []*genai.Content, config *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err := e.client.Models.EmbedContent(ctx, e.model, content, config)
		if err == nil {
			return result, nil
		}
		if !isRetryableError(err) || attempt == maxRetries {
			return nil, err
		}
		delay := time.Duration(1<<attempt) * time.Second
		slog.Warn("embedding API rate limited, retrying", "attempt", attempt+1, "delay", delay)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, fmt.Errorf("unreachable")
}

func isRetryableError(err error) bool {
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "RESOURCE_EXHAUSTED") || strings.Contains(s, "503")
}
