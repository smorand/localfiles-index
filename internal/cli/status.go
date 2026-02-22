package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display index statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		format, _ := cmd.Flags().GetString("format")

		stats, err := store.GetStats(ctx)
		if err != nil {
			return err
		}

		switch format {
		case "json":
			data, err := json.MarshalIndent(stats, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling JSON: %w", err)
			}
			fmt.Println(string(data))
		case "table":
			fmt.Printf("Total Documents: %d\n", stats.TotalDocuments)
			fmt.Printf("Total Chunks:    %d\n", stats.TotalChunks)

			if len(stats.ByType) > 0 {
				fmt.Println("\nBy Type:")
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for t, count := range stats.ByType {
					fmt.Fprintf(w, "  %s\t%d\n", t, count)
				}
				w.Flush()
			}

			if len(stats.ByTag) > 0 {
				fmt.Println("\nBy Tag:")
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for c, count := range stats.ByTag {
					fmt.Fprintf(w, "  %s\t%d\n", c, count)
				}
				w.Flush()
			}
		default:
			return fmt.Errorf("unsupported format: %s", format)
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().StringP("format", "f", "table", "Output format: table or json")
	rootCmd.AddCommand(statusCmd)
}
