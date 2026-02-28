package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	DatabaseURL              string
	OpenRouterAPIKey         string
	InferenceModel           string
	GeminiAPIKey             string
	EmbeddingModel           string
	EmbeddingDimensions      int
	OAuthCredentialsPath     string
	GoogleCredentialsFile    string
	MCPPort                  int
	TitleConfidenceThreshold float64
	ChunkSize                int
	ChunkOverlap             int
	PDFExtractorPath         string
	LogLevel                 string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:              getEnv("DATABASE_URL", "postgresql://localfiles:localfiles@localhost:5432/localfiles?sslmode=disable"),
		OpenRouterAPIKey:         os.Getenv("OPENROUTER_API_KEY"),
		InferenceModel:           getEnv("INFERENCE_MODEL", "google/gemini-3-flash-preview"),
		GeminiAPIKey:             os.Getenv("GEMINI_API_KEY"),
		EmbeddingModel:           getEnv("EMBEDDING_MODEL", "gemini-embedding-001"),
		EmbeddingDimensions:      getEnvInt("EMBEDDING_DIMENSIONS", 768),
		OAuthCredentialsPath:     getEnv("OAUTH_CREDENTIALS_PATH", ""),
		GoogleCredentialsFile:    getEnv("GOOGLE_CREDENTIALS_FILE", ""),
		MCPPort:                  getEnvInt("MCP_PORT", 8080),
		TitleConfidenceThreshold: getEnvFloat("TITLE_CONFIDENCE_THRESHOLD", 0.9),
		ChunkSize:                getEnvInt("CHUNK_SIZE", 100),
		ChunkOverlap:             getEnvInt("CHUNK_OVERLAP", 5),
		PDFExtractorPath:         getEnv("PDF_EXTRACTOR_PATH", "pdf-extractor"),
		LogLevel:                 getEnv("LOG_LEVEL", "info"),
	}

	if cfg.OpenRouterAPIKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY environment variable is required")
	}
	if cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is required (used for embeddings)")
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}
