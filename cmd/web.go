package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
)

// webCmd starts (or ensures) the API server and opens the embedded web UI in the browser.
var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Open the IssueMap Web UI",
	Long:  "Starts the IssueMap HTTP server if necessary and opens the embedded Web UI in your default browser.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWeb(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(webCmd)
}

func runWeb(cmd *cobra.Command, args []string) error {
	// 1) Locate repo and .issuemap
	repoPath, err := findGitRoot()
	if err != nil {
		printError(errors.New(app.ErrNotGitRepo))
		return err
	}
	issuemapPath := filepath.Join(repoPath, app.ConfigDirName)
	if _, err := os.Stat(issuemapPath); os.IsNotExist(err) {
		printError(errors.New(app.ErrNotInitialized))
		return err
	}

	pidPath := filepath.Join(issuemapPath, app.ServerPIDFile)
	logPath := filepath.Join(issuemapPath, app.ServerLogFile)

	// 2) Determine if server is running and healthy
	running := fileExists(pidPath)
	port := findPortInLog(logPath)
	healthy := false
	if running && port > 0 {
		healthy = pingHealth(port)
	}

	startedByWeb := false

	// 3) Start server in background if not healthy
	if !healthy {
		// Spawn `issuemap server start` in background
		self, _ := os.Executable()
		bg := exec.Command(self, "server", "start")
		bg.Stdout = nil
		bg.Stderr = nil
		bg.Stdin = nil
		if err := bg.Start(); err != nil {
			printError(fmt.Errorf("failed to start server: %v", err))
			return err
		}
		startedByWeb = true
		// Wait until health is OK or timeout
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			// Port might not be known yet; try to refresh from log and ping
			if port == 0 {
				port = findPortInLog(logPath)
			}
			if port > 0 && pingHealth(port) {
				healthy = true
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	if !healthy {
		printError(fmt.Errorf("server did not become healthy"))
		return fmt.Errorf("server not healthy")
	}

	// Fallback: if still no port, try default
	if port == 0 {
		port = app.DefaultServerPort
	}

	// 4) Open browser to the root UI
	url := fmt.Sprintf("http://localhost:%d/", port)
	if err := openBrowser(url); err != nil {
		// If browser open fails, at least print the URL and basic ping result
		printWarning(fmt.Sprintf("Open your browser to: %s", url))
		// Try a quick HEAD to verify
		_ = quickPing(url)
	} else {
		printSuccess(fmt.Sprintf("Opened IssueMap Web UI at %s", url))
	}

	// 5) Keep CLI alive and handle Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	if startedByWeb {
		printInfo("Press Ctrl+C to stop the IssueMap server...")
	} else {
		printInfo("Press Ctrl+C to exit (server will continue running). To stop it, run: 'issuemap server stop'")
	}
	<-sigCh

	if startedByWeb {
		printInfo("Stopping IssueMap server...")
		self, _ := os.Executable()
		stop := exec.Command(self, "server", "stop")
		_ = stop.Run()
		printSuccess("Server stopped. Exiting web command.")
	} else {
		printInfo("Exiting web command. Server left running.")
	}
	return nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		// Linux/BSD
		if err := exec.Command("xdg-open", url).Start(); err == nil {
			return nil
		}
		// Try sensible-browser as fallback
		return exec.Command("sensible-browser", url).Start()
	}
}

func quickPing(url string) bool {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Head(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}
