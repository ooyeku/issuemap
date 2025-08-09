package cmd

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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
		return runServerStop(cmd, args)
	},
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show server status",
	Long:  `Display the current status of the IssueMap HTTP server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServerStatus(cmd, args)
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

// runServerStatus reports whether the server is running and on which port.
func runServerStatus(cmd *cobra.Command, args []string) error {
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf(app.ErrNotGitRepo))
		return err
	}
	issuemapPath := filepath.Join(repoPath, app.ConfigDirName)
	pidPath := filepath.Join(issuemapPath, app.ServerPIDFile)
	logPath := filepath.Join(issuemapPath, app.ServerLogFile)

	running := fileExists(pidPath)
	port := findPortInLog(logPath)
	healthy := false
	if running && port > 0 {
		healthy = pingHealth(port)
	}
	if noColor {
		fmt.Printf("Running: %v\n", running)
		if port > 0 {
			fmt.Printf("Port: %d\n", port)
		}
		fmt.Printf("Healthy: %v\n", healthy)
	} else {
		fmt.Printf("%s %v\n", colorLabel("Running:"), running)
		if port > 0 {
			fmt.Printf("%s %d\n", colorLabel("Port:"), port)
		}
		fmt.Printf("%s %v\n", colorLabel("Healthy:"), healthy)
	}
	return nil
}

// runServerStop stops the server by killing the PID from the pid file.
func runServerStop(cmd *cobra.Command, args []string) error {
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf(app.ErrNotGitRepo))
		return err
	}
	issuemapPath := filepath.Join(repoPath, app.ConfigDirName)
	pidPath := filepath.Join(issuemapPath, app.ServerPIDFile)
	if !fileExists(pidPath) {
		printWarning("Server is not running")
		return nil
	}
	// Use pkill-like behavior: read PID and send SIGTERM
	pidBytes, err := os.ReadFile(pidPath)
	if err != nil {
		printError(fmt.Errorf("failed to read PID file: %v", err))
		return err
	}
	pid := strings.TrimSpace(string(pidBytes))
	// Try to terminate
	cmdKill := exec.Command("kill", pid)
	if err := cmdKill.Run(); err != nil {
		printWarning(fmt.Sprintf("Failed to send SIGTERM to %s: %v", pid, err))
	}
	// Wait briefly and confirm
	time.Sleep(500 * time.Millisecond)
	if fileExists(pidPath) {
		_ = os.Remove(pidPath)
	}
	printSuccess("Server stop requested")
	return nil
}

// Helpers
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func findPortInLog(logPath string) int {
	f, err := os.Open(logPath)
	if err != nil {
		return 0
	}
	defer f.Close()
	// Scan lines, keep last match
	re := regexp.MustCompile(`starting on port (\d+)`)
	var port int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		m := re.FindStringSubmatch(line)
		if len(m) == 2 {
			fmt.Sscanf(m[1], "%d", &port)
		}
	}
	return port
}

func pingHealth(port int) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d%s/health", port, app.APIBasePath))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}
