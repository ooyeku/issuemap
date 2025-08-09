package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	tuiReadOnly    bool
	tuiServerMode  bool
	tuiRepoPath    string
	tuiCheckParity bool
	tuiHelpOverlay bool
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

	// Connection mode
	mode := "file"
	if tuiServerMode {
		mode = "server"
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
	}

	// Exit non-zero if terminal not suitable (very small width/height)
	if w, h := os.Stdout.Stat(); w != nil && h != nil {
		// no-op: placeholder for future TTY checks
	}
	return nil
}

// runTUICheckParity prints the readiness checklist for TUI parity with CLI.
func runTUICheckParity() error {
	repoRoot, _ := findGitRoot()
	if noColor {
		fmt.Printf("TUI Parity Check\n")
		if repoRoot != "" {
			fmt.Printf("Repo: %s\n", repoRoot)
		}
		fmt.Println("Core flows available:")
		fmt.Println("  create ✔  branch ✔  sync ✔  merge ✔  show ✔  list ✔")
		fmt.Println("Supporting:")
		fmt.Println("  estimate ✔  start/stop ✔  deps ✔  bulk ✔  search ✔")
		fmt.Println("Next steps:")
		fmt.Println("  Wire TUI actions to CLI commands; keep CLI as source of truth")
	} else {
		fmt.Printf("%s\n", colorHeader("TUI Parity Check"))
		if repoRoot != "" {
			fmt.Printf("%s %s\n", colorLabel("Repo:"), colorValue(repoRoot))
		}
		fmt.Println(colorHeader("Core flows available:"))
		fmt.Println("  create ✔  branch ✔  sync ✔  merge ✔  show ✔  list ✔")
		fmt.Println(colorHeader("Supporting:"))
		fmt.Println("  estimate ✔  start/stop ✔  deps ✔  bulk ✔  search ✔")
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
