package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	tuiReadOnly    bool
	tuiServerMode  bool
	tuiRepoPath    string
	tuiCheckParity bool
	tuiHelpOverlay bool
	tuiView        string
	tuiPalette     bool
)

// tuiCmd provides a professional, keyboard-first terminal UI entry point.
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Terminal UI (preview)",
	Long:  "Terminal UI for IssueMap (preview). Optimized for keyboard use and aligned with CLI conventions.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if tuiCheckParity {
			return runTUICheckParity()
		}
		if tuiHelpOverlay {
			return runTUIHelpOverlay()
		}
		return runTUIOverlay()
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
	tuiCmd.Flags().BoolVar(&tuiReadOnly, "read-only", false, "run in read-only mode")
	tuiCmd.Flags().BoolVar(&tuiServerMode, "server", false, "prefer server mode if available")
	tuiCmd.Flags().StringVar(&tuiRepoPath, "repo", "", "repo path (defaults to git root)")
	tuiCmd.Flags().BoolVar(&tuiCheckParity, "check-parity", false, "check CLI parity readiness for TUI")
	tuiCmd.Flags().BoolVar(&tuiHelpOverlay, "help-overlay", false, "print keyboard help overlay and exit")
	tuiCmd.Flags().StringVar(&tuiView, "view", "list", "view to render (list, detail, board, search, graph, activity, settings)")
	tuiCmd.Flags().BoolVar(&tuiPalette, "palette", false, "print command palette and exit")
}

// runTUIOverlay shows a concise help overlay for keyboard-first usage.
func runTUIOverlay() error {
	// Determine repo root if not provided
	var repo string
	if tuiRepoPath != "" {
		repo = tuiRepoPath
	} else {
		root, err := findGitRoot()
		if err != nil {
			repo = "(not a git repo)"
		} else {
			repo = root
		}
	}

	// Connection mode (detect when possible)
	mode := "file"
	port := 0
	if root, err := findGitRoot(); err == nil {
		issuemapPath := filepath.Join(root, ".issuemap")
		pidPath := filepath.Join(issuemapPath, "server.pid")
		logPath := filepath.Join(issuemapPath, "server.log")
		if fileExists(pidPath) {
			if p := findPortInLog(logPath); p > 0 && pingHealth(p) {
				port = p
				mode = "server"
			}
		}
	}
	// Force server mode if explicitly requested
	if tuiServerMode {
		mode = "server"
	}

	// Optional: palette output
	if tuiPalette {
		return runTUIPalette()
	}
	if noColor {
		fmt.Printf("IssueMap TUI (preview)\n")
		fmt.Printf("Repo: %s\nMode: %s\nRead-only: %v\n\n", repo, mode, tuiReadOnly)
		fmt.Println("Keybindings (planned)")
		fmt.Println("  j/k or arrows  - navigate")
		fmt.Println("  enter          - open details")
		fmt.Println("  space          - multi-select")
		fmt.Println("  /              - focus query bar")
		fmt.Println("  ctrl+p         - command palette")
		fmt.Println("  ?              - help overlay")
		fmt.Println()
		fmt.Println("Views (planned)")
		fmt.Println("  list, detail, board, search, graph, activity, settings")
		if port > 0 {
			fmt.Printf("\nConnected to server on port %d\n", port)
		}
	} else {
		fmt.Printf("%s\n", colorHeader("IssueMap TUI (preview)"))
		fmt.Printf("%s %s\n", colorLabel("Repo:"), colorValue(repo))
		fmt.Printf("%s %s\n", colorLabel("Mode:"), colorValue(mode))
		fmt.Printf("%s %v\n\n", colorLabel("Read-only:"), tuiReadOnly)
		fmt.Println(colorHeader("Keybindings (planned)"))
		fmt.Println("  j/k or arrows  - navigate")
		fmt.Println("  enter          - open details")
		fmt.Println("  space          - multi-select")
		fmt.Println("  /              - focus query bar")
		fmt.Println("  ctrl+p         - command palette")
		fmt.Println("  ?              - help overlay")
		fmt.Println()
		fmt.Println(colorHeader("Views (planned)"))
		fmt.Println("  list, detail, board, search, graph, activity, settings")
		if port > 0 {
			fmt.Printf("%s %s %d\n", colorLabel("Connected:"), colorValue("port"), port)
		}
	}

	// Exit non-zero if terminal not suitable (very small width/height)
	if w, h := os.Stdout.Stat(); w != nil && h != nil {
		// no-op: placeholder for future TTY checks
	}

	// Persist basic UI state
	_ = saveTUIState(repo, mode, tuiView, tuiReadOnly)

	// Render selected view (stubbed)
	if err := renderView(tuiView); err != nil {
		return err
	}
	return nil
}

// runTUICheckParity prints the readiness checklist for TUI parity with CLI.
func runTUICheckParity() error {
	repoRoot, _ := findGitRoot()
	// Define core and supporting commands we expect in CLI
	core := []string{"create", "branch", "sync", "show", "list", "merge"}
	support := []string{"estimate", "start", "stop", "depend", "deps", "bulk", "search", "close"}

	// Build availability maps
	available := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		available[c.Name()] = true
		for _, a := range c.Aliases {
			available[a] = true
		}
	}

	// Format lines
	formatLine := func(names []string) string {
		parts := make([]string, 0, len(names))
		for _, n := range names {
			status := "[MISSING]"
			if available[n] {
				status = "[OK]"
			}
			parts = append(parts, fmt.Sprintf("%s %s", n, status))
		}
		sort.Strings(parts)
		return "  " + strings.Join(parts, "  ")
	}
	if noColor {
		fmt.Printf("TUI Parity Check\n")
		if repoRoot != "" {
			fmt.Printf("Repo: %s\n", repoRoot)
		}
		fmt.Println("Core flows:")
		fmt.Println(formatLine(core))
		fmt.Println("Supporting:")
		fmt.Println(formatLine(support))
		fmt.Println("Next steps:")
		fmt.Println("  Wire TUI actions to CLI commands; keep CLI as source of truth")
	} else {
		fmt.Printf("%s\n", colorHeader("TUI Parity Check"))
		if repoRoot != "" {
			fmt.Printf("%s %s\n", colorLabel("Repo:"), colorValue(repoRoot))
		}
		fmt.Println(colorHeader("Core flows:"))
		fmt.Println(formatLine(core))
		fmt.Println(colorHeader("Supporting:"))
		fmt.Println(formatLine(support))
		fmt.Println(colorHeader("Next steps:"))
		fmt.Println("  Wire TUI actions to CLI commands; keep CLI as source of truth")
	}
	_ = filepath.Join // silence import until used later
	return nil
}

// runTUIHelpOverlay prints just the keyboard overlay in a compact, script-friendly form.
func runTUIHelpOverlay() error {
	if noColor {
		fmt.Println("Keys:\n  j/k, arrows; enter; space; /; ctrl+p; ?")
		fmt.Println("Views:\n  list, detail, board, search, graph, activity, settings")
		return nil
	}
	fmt.Println(colorHeader("Keys:"))
	fmt.Println("  j/k, arrows; enter; space; /; ctrl+p; ?")
	fmt.Println(colorHeader("Views:"))
	fmt.Println("  list, detail, board, search, graph, activity, settings")
	return nil
}

// runTUIPalette prints a simple command palette of common actions.
func runTUIPalette() error {
	lines := []string{
		"create <title> --type <t> --labels a,b",
		"list --status open",
		"show ISSUE-123",
		"branch ISSUE-123",
		"start ISSUE-123 | stop ISSUE-123",
		"edit ISSUE-123 --status review --assignee me",
		"bulk --query \"status:open AND label:frontend\" --set status=review",
		"deps ISSUE-123 --graph",
		"search \"type:bug AND updated:<7d\"",
	}
	for _, l := range lines {
		fmt.Println("  ", l)
	}
	return nil
}

// View rendering (stubs) and state persistence
type tuiState struct {
	Repo     string `json:"repo"`
	Mode     string `json:"mode"`
	View     string `json:"view"`
	ReadOnly bool   `json:"read_only"`
}

func saveTUIState(repo, mode, view string, readOnly bool) error {
	root, err := findGitRoot()
	if err != nil {
		return nil
	}
	dir := filepath.Join(root, ".issuemap")
	path := filepath.Join(dir, "tui_state.json")
	st := tuiState{Repo: repo, Mode: mode, View: view, ReadOnly: readOnly}
	data, err := json.MarshalIndent(&st, "", "  ")
	if err != nil {
		return nil
	}
	return os.WriteFile(path, data, 0644)
}

func renderView(view string) error {
	switch strings.ToLower(view) {
	case "list":
		// Minimal list banner (list details via `issuemap list`)
		fmt.Println("\n[View] list - use `issuemap list` for full results")
	case "detail":
		fmt.Println("\n[View] detail - open with: issuemap show ISSUE-XXX")
	case "board":
		fmt.Println("\n[View] board - statuses as columns (planned)")
	case "search":
		fmt.Println("\n[View] search - use --query (planned)")
	case "graph":
		fmt.Println("\n[View] graph - dependency graph (planned)")
	case "activity":
		fmt.Println("\n[View] activity - recent changes (planned)")
	case "settings":
		fmt.Println("\n[View] settings - theme, keys, columns (planned)")
	default:
		// no-op
	}
	return nil
}
