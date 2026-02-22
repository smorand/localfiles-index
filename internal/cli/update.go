package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"localfiles-index/internal/analyzer"
	"localfiles-index/internal/embedding"
	"localfiles-index/internal/indexer"
)

var updateCmd = &cobra.Command{
	Use:   "update [path]",
	Short: "Check and re-index modified documents",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		force, _ := cmd.Flags().GetBool("force")

		anlz, err := analyzer.New(ctx, cfg.GeminiAPIKey, cfg.GeminiModel)
		if err != nil {
			return fmt.Errorf("creating analyzer: %w", err)
		}

		emb, err := embedding.New(ctx, cfg.GeminiAPIKey, cfg.EmbeddingModel, cfg.EmbeddingDimensions)
		if err != nil {
			return fmt.Errorf("creating embedder: %w", err)
		}

		idx := indexer.New(store, anlz, emb, cfg)

		if len(args) == 1 {
			// Update specific file
			return updateSingleFile(ctx, idx, args[0], force)
		}

		// Update all documents
		return updateAllDocuments(ctx, idx, force)
	},
}

func updateSingleFile(ctx context.Context, idx *indexer.Indexer, path string, force bool) error {
	doc, err := store.GetDocumentByPath(ctx, path)
	if err != nil {
		return fmt.Errorf("document not found in index: %s", path)
	}

	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file not found on disk: %s", path)
	}

	if !force && !stat.ModTime().After(doc.FileMtime) {
		fmt.Println("0 updated, 1 unchanged, 0 missing")
		return nil
	}

	// Preserve existing tags
	tags, err := store.GetDocumentTags(ctx, doc.ID)
	if err != nil {
		return fmt.Errorf("getting document tags: %w", err)
	}
	var tagNames []string
	for _, t := range tags {
		tagNames = append(tagNames, t.Name)
	}

	_, err = idx.IndexFile(ctx, path, tagNames)
	if err != nil {
		return fmt.Errorf("re-indexing: %w", err)
	}

	fmt.Println("1 updated, 0 unchanged, 0 missing")
	return nil
}

func updateAllDocuments(ctx context.Context, idx *indexer.Indexer, force bool) error {
	docs, err := store.ListDocuments(ctx)
	if err != nil {
		return fmt.Errorf("listing documents: %w", err)
	}

	var updated, unchanged, missing int

	for _, doc := range docs {
		stat, err := os.Stat(doc.FilePath)
		if err != nil {
			slog.Warn("file missing from disk", "path", doc.FilePath)
			missing++
			continue
		}

		if !force && !stat.ModTime().After(doc.FileMtime) {
			unchanged++
			continue
		}

		// Preserve existing tags
		var tagNames []string
		for _, t := range doc.Tags {
			tagNames = append(tagNames, t.Name)
		}

		_, err = idx.IndexFile(ctx, doc.FilePath, tagNames)
		if err != nil {
			slog.Error("failed to re-index", "path", doc.FilePath, "error", err)
			continue
		}

		updated++
	}

	fmt.Printf("%d updated, %d unchanged, %d missing\n", updated, unchanged, missing)
	return nil
}

func init() {
	updateCmd.Flags().BoolP("force", "f", false, "Force re-indexing regardless of mtime")
	rootCmd.AddCommand(updateCmd)
}
