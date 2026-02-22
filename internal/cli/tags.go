package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var tagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage document tags",
}

var tagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tags, err := store.ListTags(ctx)
		if err != nil {
			return err
		}

		if len(tags) == 0 {
			fmt.Println("No tags found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tDESCRIPTION\tRULE\tDOCUMENTS")
		fmt.Fprintln(w, "----\t-----------\t----\t---------")
		for _, tag := range tags {
			count, _ := store.TagDocumentCount(ctx, tag.ID)
			rule := tag.Rule
			if len(rule) > 40 {
				rule = rule[:37] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", tag.Name, tag.Description, rule, count)
		}
		return w.Flush()
	},
}

var tagsAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a new tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]
		description, _ := cmd.Flags().GetString("description")
		rule, _ := cmd.Flags().GetString("rule")

		tag, err := store.CreateTag(ctx, name, description)
		if err != nil {
			return err
		}

		if rule != "" {
			tag, err = store.UpdateTagRule(ctx, tag.Name, rule)
			if err != nil {
				return fmt.Errorf("setting tag rule: %w", err)
			}
		}

		fmt.Printf("Tag created: %s (id: %s)\n", tag.Name, tag.ID)
		return nil
	},
}

var tagsUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update a tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			if _, err := store.UpdateTag(ctx, name, description); err != nil {
				return err
			}
		}

		if cmd.Flags().Changed("rule") {
			rule, _ := cmd.Flags().GetString("rule")
			if _, err := store.UpdateTagRule(ctx, name, rule); err != nil {
				return err
			}
		}

		fmt.Printf("Tag updated: %s\n", name)
		return nil
	},
}

var tagsRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Delete a tag (unlinks from all documents)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		if err := store.DeleteTag(ctx, name); err != nil {
			return err
		}

		fmt.Printf("Tag deleted: %s\n", name)
		return nil
	},
}

var tagsMergeCmd = &cobra.Command{
	Use:   "merge <source> <target>",
	Short: "Merge source tag into target tag (moves all documents, deletes source)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		source := args[0]
		target := args[1]

		if err := store.MergeTag(ctx, source, target); err != nil {
			return err
		}

		fmt.Printf("Tag merged: %s → %s\n", source, target)
		return nil
	},
}

func init() {
	tagsAddCmd.Flags().String("description", "", "Tag description")
	tagsAddCmd.Flags().String("rule", "", "Auto-tagging rule (LLM prompt)")
	tagsUpdateCmd.Flags().String("description", "", "Tag description")
	tagsUpdateCmd.Flags().String("rule", "", "Auto-tagging rule (LLM prompt)")

	tagsCmd.AddCommand(tagsListCmd, tagsAddCmd, tagsUpdateCmd, tagsRemoveCmd, tagsMergeCmd)
	rootCmd.AddCommand(tagsCmd)
}
