package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"localfiles-index/internal/analyzer"
	"localfiles-index/internal/embedding"
	"localfiles-index/internal/indexer"
)

var indexCmd = &cobra.Command{
	Use:   "index <path>",
	Short: "Index a file or directory into the search index",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		path := args[0]

		tagsStr, _ := cmd.Flags().GetString("tags")
		tags := splitTags(tagsStr)

		// Create analyzer and embedder
		anlz, err := analyzer.New(ctx, cfg.GeminiAPIKey, cfg.GeminiModel)
		if err != nil {
			return fmt.Errorf("creating analyzer: %w", err)
		}

		emb, err := embedding.New(ctx, cfg.GeminiAPIKey, cfg.EmbeddingModel, cfg.EmbeddingDimensions)
		if err != nil {
			return fmt.Errorf("creating embedder: %w", err)
		}

		idx := indexer.New(store, anlz, emb, cfg)

		// Check if path is a directory
		stat, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("file not found: %s", path)
		}

		if stat.IsDir() {
			return indexDirectory(ctx, idx, path, tags)
		}

		result, err := idx.IndexFile(ctx, path, tags)
		if err != nil {
			return err
		}

		fmt.Printf("Indexed: %s\n", result.Title)
		fmt.Printf("  Document ID: %s\n", result.DocumentID)
		fmt.Printf("  Chunks: %d\n", result.ChunkCount)
		if result.ImageCount > 0 {
			fmt.Printf("  Images: %d\n", result.ImageCount)
		}
		if len(result.Tags) > 0 {
			fmt.Printf("  Tags: %s\n", strings.Join(result.Tags, ", "))
		}

		return nil
	},
}

func indexDirectory(ctx context.Context, idx *indexer.Indexer, dir string, tags []string) error {
	var indexed, skipped, errors int

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		if !indexer.IsSupported(path) {
			slog.Warn("skipping unsupported file", "path", path)
			skipped++
			return nil
		}

		result, err := idx.IndexFile(ctx, path, tags)
		if err != nil {
			slog.Error("failed to index file", "path", path, "error", err)
			errors++
			return nil
		}

		fmt.Printf("Indexed: %s (%d chunks)\n", result.Title, result.ChunkCount)
		indexed++
		return nil
	})

	if err != nil {
		return fmt.Errorf("walking directory: %w", err)
	}

	fmt.Printf("\nSummary: %d indexed, %d skipped, %d errors\n", indexed, skipped, errors)
	return nil
}

// splitTags splits a comma-separated tag string into a slice of trimmed, non-empty names.
func splitTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var tags []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			tags = append(tags, p)
		}
	}
	return tags
}

func init() {
	indexCmd.Flags().StringP("tags", "t", "", "Comma-separated tag names to assign")
	rootCmd.AddCommand(indexCmd)
}
