package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP HTTP Streamable server",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		credentials, _ := cmd.Flags().GetString("credentials")
		_ = credentials

		fmt.Printf("Starting MCP server on port %d...\n", port)
		// MCP server implementation will be added in Lot 7
		return fmt.Errorf("MCP server not yet implemented")
	},
}

func init() {
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	serveCmd.Flags().String("credentials", "~/.credentials/scm-pwd-web.json", "Path to OAuth credentials")
	rootCmd.AddCommand(serveCmd)
}
