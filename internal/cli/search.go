package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"localfiles-index/internal/embedding"
	"localfiles-index/internal/searcher"
	"localfiles-index/internal/storage"
)

const (
	// maxTitleDisplayLen is the maximum display width for title columns.
	maxTitleDisplayLen = 40
	// maxPathDisplayLen is the maximum display width for path columns.
	maxPathDisplayLen = 50
	// maxExcerptDisplayLen is the maximum display width for excerpt text.
	maxExcerptDisplayLen = 200
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search indexed documents",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		query := args[0]

		mode, _ := cmd.Flags().GetString("mode")
		category, _ := cmd.Flags().GetString("category")
		limit, _ := cmd.Flags().GetInt("limit")
		format, _ := cmd.Flags().GetString("format")

		// Validate category exists if specified
		if category != "" {
			_, err := store.GetCategoryByName(ctx, category)
			if err != nil {
				return fmt.Errorf("category not found: %s", category)
			}
		}

		emb, err := embedding.New(ctx, cfg.GeminiAPIKey, cfg.EmbeddingModel, cfg.EmbeddingDimensions)
		if err != nil {
			return fmt.Errorf("creating embedder: %w", err)
		}

		srch := searcher.New(store, emb)
		results, err := srch.Search(ctx, query, mode, category, limit)
		if err != nil {
			return err
		}

		return formatResults(results, format)
	},
}

func formatResults(results []*storage.SearchResult, format string) error {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	switch format {
	case "table":
		return formatTable(results)
	case "json":
		return formatJSON(results)
	case "detail":
		return formatDetail(results)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func formatTable(results []*storage.SearchResult) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TITLE\tPATH\tTYPE\tCATEGORY\tSCORE\tPAGE")
	fmt.Fprintln(w, "-----\t----\t----\t--------\t-----\t----")

	for _, r := range results {
		page := ""
		if r.SourcePage != nil {
			page = fmt.Sprintf("%d", *r.SourcePage)
		}
		cat := r.CategoryName
		if cat == "" {
			cat = "-"
		}
		title := truncate(r.Title, maxTitleDisplayLen)
		path := truncate(r.FilePath, maxPathDisplayLen)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.4f\t%s\n", title, path, r.DocumentType, cat, r.Similarity, page)
	}
	return w.Flush()
}

func formatJSON(results []*storage.SearchResult) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func formatDetail(results []*storage.SearchResult) error {
	for i, r := range results {
		if i > 0 {
			fmt.Println(strings.Repeat("─", 60))
		}
		fmt.Printf("Result %d:\n", i+1)
		fmt.Printf("  Title:    %s\n", r.Title)
		fmt.Printf("  Path:     %s\n", r.FilePath)
		fmt.Printf("  Type:     %s\n", r.DocumentType)
		fmt.Printf("  Category: %s\n", r.CategoryName)
		fmt.Printf("  Score:    %.4f\n", r.Similarity)
		fmt.Printf("  Segment:  %s", r.ChunkType)
		if r.ChunkLabel != "" {
			fmt.Printf(" (%s)", r.ChunkLabel)
		}
		fmt.Println()
		if r.SourcePage != nil {
			fmt.Printf("  Page:     %d\n", *r.SourcePage)
		}
		fmt.Printf("  Excerpt:  %s\n", truncate(r.Excerpt, maxExcerptDisplayLen))
	}
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	searchCmd.Flags().StringP("mode", "m", "semantic", "Search mode: semantic or fulltext")
	searchCmd.Flags().StringP("category", "c", "", "Filter by category")
	searchCmd.Flags().IntP("limit", "l", 10, "Max results")
	searchCmd.Flags().StringP("format", "f", "table", "Output format: table, json, detail")
	rootCmd.AddCommand(searchCmd)
}
