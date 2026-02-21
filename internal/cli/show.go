package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// maxChunkDisplayLen is the maximum display width for chunk content preview.
const maxChunkDisplayLen = 200

var showCmd = &cobra.Command{
	Use:   "show <path|id>",
	Short: "Display full details of an indexed document",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		identifier := args[0]
		showChunks, _ := cmd.Flags().GetBool("chunks")

		docID, err := resolveDocumentID(ctx, identifier)
		if err != nil {
			return err
		}

		d, err := store.GetDocumentWithChunks(ctx, docID)
		if err != nil {
			return fmt.Errorf("document not found: %s", identifier)
		}

		fmt.Printf("Document: %s\n", d.Title)
		fmt.Printf("  ID:       %s\n", d.ID)
		fmt.Printf("  Path:     %s\n", d.FilePath)
		fmt.Printf("  Type:     %s\n", d.DocumentType)
		fmt.Printf("  MIME:     %s\n", d.MimeType)
		fmt.Printf("  Size:     %d bytes\n", d.FileSize)
		if d.Category != nil {
			fmt.Printf("  Category: %s\n", d.Category.Name)
		}
		fmt.Printf("  Title Confidence: %.2f\n", d.TitleConfidence)
		fmt.Printf("  Indexed:  %s\n", d.IndexedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Chunks:   %d\n", len(d.Chunks))
		fmt.Printf("  Images:   %d\n", len(d.Images))

		if showChunks && len(d.Chunks) > 0 {
			fmt.Println("\nChunks:")
			for _, ch := range d.Chunks {
				fmt.Printf("  [%d] type=%s", ch.ChunkIndex, ch.ChunkType)
				if ch.ChunkLabel != "" {
					fmt.Printf(" label=%s", ch.ChunkLabel)
				}
				if ch.SourcePage != nil {
					fmt.Printf(" page=%d", *ch.SourcePage)
				}
				fmt.Println()
				fmt.Printf("      %s\n", truncate(ch.Content, maxChunkDisplayLen))
			}
		}

		return nil
	},
}

func init() {
	showCmd.Flags().Bool("chunks", false, "Include chunk content in output")
	rootCmd.AddCommand(showCmd)
}
