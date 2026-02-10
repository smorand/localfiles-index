package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"localfiles-index/internal/config"
	"localfiles-index/internal/storage"
)

var (
	verbose bool
	cfg     *config.Config
	store   *storage.Store
)

var rootCmd = &cobra.Command{
	Use:   "localfiles-index",
	Short: "Personal file indexing and semantic search system",
	Long:  "Index local files (images, PDFs, text, spreadsheets) and search them using natural language or keywords.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Setup logging
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		}
		logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
		slog.SetDefault(logger)

		// Skip config/db for help commands
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}

		// Load config
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Connect to database
		ctx := context.Background()
		store, err = storage.New(ctx, cfg.DatabaseURL, logger)
		if err != nil {
			return fmt.Errorf("connecting to database: %w", err)
		}

		// Run migrations
		if err := store.Migrate(ctx); err != nil {
			return fmt.Errorf("running migrations: %w", err)
		}

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if store != nil {
			store.Close()
		}
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug-level logging")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
