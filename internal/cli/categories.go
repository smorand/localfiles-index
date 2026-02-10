package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var categoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "Manage document categories",
}

var categoriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all categories",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		cats, err := store.ListCategories(ctx)
		if err != nil {
			return err
		}

		if len(cats) == 0 {
			fmt.Println("No categories found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tDESCRIPTION")
		fmt.Fprintln(w, "----\t-----------")
		for _, cat := range cats {
			fmt.Fprintf(w, "%s\t%s\n", cat.Name, cat.Description)
		}
		return w.Flush()
	},
}

var categoriesAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a new category",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]
		description, _ := cmd.Flags().GetString("description")

		cat, err := store.CreateCategory(ctx, name, description)
		if err != nil {
			return err
		}

		fmt.Printf("Category created: %s (id: %s)\n", cat.Name, cat.ID)
		return nil
	},
}

var categoriesUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update a category",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]
		description, _ := cmd.Flags().GetString("description")

		cat, err := store.UpdateCategory(ctx, name, description)
		if err != nil {
			return err
		}

		fmt.Printf("Category updated: %s\n", cat.Name)
		return nil
	},
}

var categoriesRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Delete a category",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if err := store.DeleteCategory(ctx, name, force); err != nil {
			return err
		}

		fmt.Printf("Category deleted: %s\n", name)
		return nil
	},
}

func init() {
	categoriesAddCmd.Flags().String("description", "", "Category description")
	categoriesUpdateCmd.Flags().String("description", "", "Category description")
	categoriesRemoveCmd.Flags().Bool("force", false, "Delete even if documents reference this category")

	categoriesCmd.AddCommand(categoriesListCmd, categoriesAddCmd, categoriesUpdateCmd, categoriesRemoveCmd)
	rootCmd.AddCommand(categoriesCmd)
}
