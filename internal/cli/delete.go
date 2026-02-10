package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <path|id>",
	Short: "Remove a document from the index",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		identifier := args[0]
		yes, _ := cmd.Flags().GetBool("yes")

		var docID uuid.UUID

		// Try as UUID
		id, parseErr := uuid.Parse(identifier)
		if parseErr == nil {
			_, err := store.GetDocumentByID(ctx, id)
			if err != nil {
				return fmt.Errorf("document not found: %s", identifier)
			}
			docID = id
		} else {
			// Try as file path
			d, err := store.GetDocumentByPath(ctx, identifier)
			if err != nil {
				return fmt.Errorf("document not found: %s", identifier)
			}
			docID = d.ID
		}

		if !yes {
			fmt.Printf("Delete document %s? [y/N]: ", docID)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := store.DeleteDocument(ctx, docID); err != nil {
			return err
		}

		fmt.Printf("Document deleted: %s\n", docID)
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(deleteCmd)
}
