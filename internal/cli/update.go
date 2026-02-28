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
	"localfiles-index/internal/gdrive"
	"localfiles-index/internal/indexer"
)

var updateCmd = &cobra.Command{
	Use:   "update [path]",
	Short: "Check and re-index modified documents",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		force, _ := cmd.Flags().GetBool("force")

		anlz := analyzer.New(cfg.OpenRouterAPIKey, cfg.InferenceModel)

		emb, err := embedding.New(ctx, cfg.GeminiAPIKey, cfg.EmbeddingModel, cfg.EmbeddingDimensions)
		if err != nil {
			return fmt.Errorf("creating embedder: %w", err)
		}

		idx := indexer.New(store, anlz, emb, cfg)

		if len(args) == 1 {
			path := args[0]

			// Handle Google Drive paths
			if gdrive.IsGDrivePath(path) {
				return updateSingleFile(ctx, idx, path, force)
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			info, err := os.Stat(absPath)
			if err != nil {
				// Path doesn't exist on disk — try as indexed file path
				return updateSingleFile(ctx, idx, absPath, force)
			}

			if info.IsDir() {
				return updateAllDocuments(ctx, idx, force, absPath)
			}
			return updateSingleFile(ctx, idx, absPath, force)
		}

		// Update all documents
		return updateAllDocuments(ctx, idx, force, "")
	},
}

func updateSingleFile(ctx context.Context, idx *indexer.Indexer, path string, force bool) error {
	doc, err := store.GetDocumentByPath(ctx, path)
	if err != nil {
		return fmt.Errorf("document not found in index: %s", path)
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

	// Handle Google Drive paths
	if gdrive.IsGDrivePath(path) {
		gdriveClient, err := initGDriveClient(ctx)
		if err != nil {
			return err
		}

		fileID := gdrive.ExtractFileID(path)
		info, err := gdriveClient.GetFileInfo(ctx, fileID)
		if err != nil {
			return fmt.Errorf("getting Drive file info: %w", err)
		}

		if !force && !info.ModifiedTime.After(doc.FileMtime) {
			fmt.Println("0 updated, 1 unchanged, 0 missing")
			return nil
		}

		_, err = idx.IndexGDriveFile(ctx, gdriveClient, fileID, tagNames)
		if err != nil {
			return fmt.Errorf("re-indexing GDrive file: %w", err)
		}

		fmt.Println("1 updated, 0 unchanged, 0 missing")
		return nil
	}

	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file not found on disk: %s", path)
	}

	if !force && !stat.ModTime().After(doc.FileMtime) {
		fmt.Println("0 updated, 1 unchanged, 0 missing")
		return nil
	}

	_, err = idx.IndexFile(ctx, path, tagNames)
	if err != nil {
		return fmt.Errorf("re-indexing: %w", err)
	}

	fmt.Println("1 updated, 0 unchanged, 0 missing")
	return nil
}

func updateAllDocuments(ctx context.Context, idx *indexer.Indexer, force bool, dirPrefix string) error {
	docs, err := store.ListDocuments(ctx)
	if err != nil {
		return fmt.Errorf("listing documents: %w", err)
	}

	var updated, unchanged, missing int
	var gdriveClient *gdrive.Client // lazily initialized

	for _, doc := range docs {
		// GDrive docs are skipped for directory-scoped updates (no "/" prefix match)
		if dirPrefix != "" && !strings.HasPrefix(doc.FilePath, dirPrefix+"/") {
			continue
		}

		// Preserve existing tags
		var tagNames []string
		for _, t := range doc.Tags {
			tagNames = append(tagNames, t.Name)
		}

		if gdrive.IsGDrivePath(doc.FilePath) {
			// Lazily init GDrive client
			if gdriveClient == nil {
				if cfg.GoogleCredentialsFile == "" {
					slog.Warn("skipping GDrive document (no credentials configured)", "path", doc.FilePath)
					missing++
					continue
				}
				gdriveClient, err = initGDriveClient(ctx)
				if err != nil {
					slog.Warn("failed to init GDrive client, skipping GDrive docs", "error", err)
					missing++
					continue
				}
			}

			fileID := gdrive.ExtractFileID(doc.FilePath)
			info, err := gdriveClient.GetFileInfo(ctx, fileID)
			if err != nil {
				slog.Warn("failed to get GDrive file info", "path", doc.FilePath, "error", err)
				missing++
				continue
			}

			if !force && !info.ModifiedTime.After(doc.FileMtime) {
				unchanged++
				continue
			}

			_, err = idx.IndexGDriveFile(ctx, gdriveClient, fileID, tagNames)
			if err != nil {
				slog.Error("failed to re-index GDrive file", "path", doc.FilePath, "error", err)
				continue
			}

			updated++
			continue
		}

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
