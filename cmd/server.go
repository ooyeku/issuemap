package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/server"
)

var (
	serverPort       int
	serverBackground bool
	serverLogLevel   string
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage the IssueMap HTTP server",
	Long: `Start, stop, and manage the IssueMap HTTP server for REST API access.

The server provides a REST API for managing issues programmatically and supports
high-performance in-memory operations with automatic persistence.

Examples:
  issuemap server start           # Start server on default port
  issuemap server stop            # Stop running server
  issuemap server status          # Check server status`,
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the IssueMap HTTP server",
	Long:  `Start the IssueMap HTTP server to provide REST API access to issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServerStart(cmd, args)
	},
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running IssueMap server",
	Long:  `Stop the currently running IssueMap HTTP server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		printInfo("Server stop functionality not yet implemented")
		return nil
	},
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show server status",
	Long:  `Display the current status of the IssueMap HTTP server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		printInfo("Server status functionality not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// Add subcommands
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverStatusCmd)

	// Start command flags
	serverStartCmd.Flags().IntVarP(&serverPort, "port", "p", app.DefaultServerPort, "port to run server on")
	serverStartCmd.Flags().BoolVarP(&serverBackground, "background", "b", false, "run server in background")
	serverStartCmd.Flags().StringVar(&serverLogLevel, "log-level", "info", "log level (debug, info, warn, error)")
}

func runServerStart(cmd *cobra.Command, args []string) error {
	// Find git repository root
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf(app.ErrNotGitRepo))
		return err
	}

	// Check if issuemap is initialized
	issuemapPath := filepath.Join(repoPath, app.ConfigDirName)
	if _, err := os.Stat(issuemapPath); os.IsNotExist(err) {
		printError(fmt.Errorf(app.ErrNotInitialized))
		return err
	}

	// Create server instance
	srv, err := server.NewServer(issuemapPath)
	if err != nil {
		printError(fmt.Errorf("Failed to create server: %v", err))
		return err
	}

	printInfo(fmt.Sprintf("Starting IssueMap server on port %d...", srv.GetPort()))
	printInfo(fmt.Sprintf("API will be available at: http://localhost:%d%s", srv.GetPort(), app.APIBasePath))

	// Start server
	if err := srv.Start(); err != nil {
		printError(fmt.Errorf("Server failed to start: %v", err))
		return err
	}

	return nil
}
