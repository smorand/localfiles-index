package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"localfiles-index/internal/mcp"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP HTTP Streamable server",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		credentialsPath, _ := cmd.Flags().GetString("credentials")

		// Expand ~ in path
		if strings.HasPrefix(credentialsPath, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("expanding home directory: %w", err)
			}
			credentialsPath = filepath.Join(home, credentialsPath[1:])
		}

		// Use config port if not explicitly set
		if !cmd.Flags().Changed("port") && cfg.MCPPort != 0 {
			port = cfg.MCPPort
		}

		// Use config credentials path if not explicitly set
		if !cmd.Flags().Changed("credentials") && cfg.OAuthCredentialsPath != "" {
			credentialsPath = cfg.OAuthCredentialsPath
		}

		// Load OAuth credentials
		creds, err := mcp.LoadCredentials(credentialsPath)
		if err != nil {
			return fmt.Errorf("loading OAuth credentials: %w", err)
		}

		// Create and start server
		server := mcp.NewServer(store, cfg, creds, port)

		fmt.Printf("MCP server starting on port %d\n", port)
		return server.Start()
	},
}

func init() {
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	serveCmd.Flags().String("credentials", "~/.credentials/scm-pwd-web.json", "Path to OAuth credentials")
	rootCmd.AddCommand(serveCmd)
}
