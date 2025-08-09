package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"context"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	tuiReadOnly    bool
	tuiServerMode  bool
	tuiRepoPath    string
	tuiCheckParity bool
	tuiHelpOverlay bool
	tuiView        string
	tuiPalette     bool
	tuiStatus      string
	tuiAssignee    string
	tuiLabels      []string
	tuiLimit       int
	// Detail view options
	tuiDetailChecklist bool
	tuiDetailDeps      bool
	tuiDetailHistory   bool
	tuiDetailHistLimit int
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
	// List view filters (basic)
	tuiCmd.Flags().StringVar(&tuiStatus, "status", "", "filter by status for list view")
	tuiCmd.Flags().StringVar(&tuiAssignee, "assignee", "", "filter by assignee for list view")
	tuiCmd.Flags().StringSliceVar(&tuiLabels, "labels", []string{}, "filter by labels for list view")
	tuiCmd.Flags().IntVar(&tuiLimit, "limit", app.DefaultListLimit, "limit results in list view")
	// Detail view toggles
	tuiCmd.Flags().BoolVar(&tuiDetailChecklist, "checklist", true, "show checklist parsed from description in detail view")
	tuiCmd.Flags().BoolVar(&tuiDetailDeps, "deps", true, "show dependency info in detail view")
	tuiCmd.Flags().BoolVar(&tuiDetailHistory, "history", true, "show recent history in detail view")
	tuiCmd.Flags().IntVar(&tuiDetailHistLimit, "history-limit", 5, "limit history entries in detail view")
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

	// Render selected view
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
		return renderListView()
	case "detail":
		return renderDetailView()
	case "activity":
		return renderActivityView()
	case "board":
		fmt.Println("\n[View] board - statuses as columns (planned)")
	case "search":
		fmt.Println("\n[View] search - use --query (planned)")
	case "graph":
		fmt.Println("\n[View] graph - dependency graph (planned)")
	case "settings":
		fmt.Println("\n[View] settings - theme, keys, columns (planned)")
	default:
		// no-op
	}
	return nil
}

// renderListView lists issues using the same services as the CLI list command and prints the table.
func renderListView() error {
	ctx := context.Background()
	repoRoot, err := findGitRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %v", err)
	}
	issuemapPath := filepath.Join(repoRoot, app.ConfigDirName)
	issueRepo := storage.NewFileIssueRepository(issuemapPath)
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	var gitRepo *git.GitClient
	if gitClient, err := git.NewGitClient(repoRoot); err == nil {
		gitRepo = gitClient
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitRepo)

	filter := repositories.IssueFilter{}
	if tuiStatus != "" {
		status := entities.Status(tuiStatus)
		filter.Status = &status
	}
	if tuiAssignee != "" {
		filter.Assignee = &tuiAssignee
	}
	if len(tuiLabels) > 0 {
		filter.Labels = tuiLabels
	}
	if tuiLimit > 0 {
		if tuiLimit > app.MaxListLimit {
			v := app.MaxListLimit
			filter.Limit = &v
		} else {
			filter.Limit = &tuiLimit
		}
	}

	list, err := issueService.ListIssues(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list issues: %w", err)
	}
	if len(list.Issues) == 0 {
		fmt.Println("No issues found for current filters.")
		return nil
	}
	displayIssuesTable(list.Issues)
	if list.Total > list.Count {
		fmt.Printf("\nShowing %d of %d issues. Use --limit to see more.\n", list.Count, list.Total)
	}
	return nil
}

// renderDetailView shows details for a single issue. Use env ISSUE_ID or print hint.
func renderDetailView() error {
	id := os.Getenv("ISSUE_ID")
	if strings.TrimSpace(id) == "" {
		fmt.Println("Provide ISSUE_ID env, e.g.: ISSUE_ID=ISSUE-001 issuemap tui --view detail")
		return nil
	}
	ctx := context.Background()
	repoRoot, err := findGitRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %v", err)
	}
	issuemapPath := filepath.Join(repoRoot, app.ConfigDirName)
	issueRepo := storage.NewFileIssueRepository(issuemapPath)
	configRepo := storage.NewFileConfigRepository(issuemapPath)
	var gitRepo *git.GitClient
	if gitClient, err := git.NewGitClient(repoRoot); err == nil {
		gitRepo = gitClient
	}
	issueService := services.NewIssueService(issueRepo, configRepo, gitRepo)
	issue, err := issueService.GetIssue(ctx, entities.IssueID(id))
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}
	// Reuse show-like formatting
	fmt.Printf("\nID: %s\nTitle: %s\nType: %s\nStatus: %s\nPriority: %s\n",
		issue.ID, issue.Title, issue.Type, issue.Status, issue.Priority)
	if issue.Assignee != nil {
		fmt.Printf("Assignee: %s\n", issue.Assignee.Username)
	}
	if len(issue.Labels) > 0 {
		names := make([]string, 0, len(issue.Labels))
		for _, l := range issue.Labels {
			names = append(names, l.Name)
		}
		fmt.Printf("Labels: %s\n", strings.Join(names, ", "))
	}
	if issue.Branch != "" {
		fmt.Printf("Branch: %s\n", issue.Branch)
	}
	if len(issue.Commits) > 0 {
		fmt.Printf("Commits: %d (latest: %s)\n", len(issue.Commits), issue.Commits[len(issue.Commits)-1].Message)
	}
	fmt.Printf("Updated: %s\n", issue.Timestamps.Updated.Format("2006-01-02 15:04:05"))
	// Checklist (parse from description lines starting with - [ ] / - [x])
	if tuiDetailChecklist && strings.TrimSpace(issue.Description) != "" {
		lines := strings.Split(issue.Description, "\n")
		has := false
		for _, ln := range lines {
			t := strings.TrimSpace(ln)
			if strings.HasPrefix(t, "- [ ]") || strings.HasPrefix(t, "- [x]") || strings.HasPrefix(t, "- [X]") {
				if !has {
					fmt.Println("Checklist:")
					has = true
				}
				fmt.Printf("  %s\n", t)
			}
		}
	}
	// Dependencies
	if tuiDetailDeps {
		if err := renderDepsForIssue(ctx, repoRoot, issue.ID); err == nil {
			// printed inline
		}
	}
	// Recent history
	if tuiDetailHistory {
		if err := renderIssueHistory(ctx, repoRoot, issue.ID, tuiDetailHistLimit); err == nil {
			// printed inline
		}
	}
	return nil
}

func renderDepsForIssue(ctx context.Context, repoRoot string, id entities.IssueID) error {
	base := filepath.Join(repoRoot, app.ConfigDirName)
	depRepo := storage.NewFileDependencyRepository(base)
	issueRepo := storage.NewFileIssueRepository(base)
	cfgRepo := storage.NewFileConfigRepository(base)
	var gitRepo *git.GitClient
	if g, err := git.NewGitClient(repoRoot); err == nil {
		gitRepo = g
	}
	issSvc := services.NewIssueService(issueRepo, cfgRepo, gitRepo)
	histSvc := services.NewHistoryService(storage.NewFileHistoryRepository(base), gitRepo)
	depSvc := services.NewDependencyService(depRepo, issSvc, histSvc)
	info, err := depSvc.GetBlockingInfo(ctx, id)
	if err != nil {
		return err
	}
	if info.IsBlocked || len(info.Blocking) > 0 {
		fmt.Println("Dependencies:")
		if info.IsBlocked {
			fmt.Printf("  Blocked by: %v\n", info.BlockedBy)
		}
		if len(info.Blocking) > 0 {
			fmt.Printf("  Blocking: %v\n", info.Blocking)
		}
	}
	return nil
}

func renderIssueHistory(ctx context.Context, repoRoot string, id entities.IssueID, limit int) error {
	base := filepath.Join(repoRoot, app.ConfigDirName)
	histRepo := storage.NewFileHistoryRepository(base)
	var gitRepo *git.GitClient
	if g, err := git.NewGitClient(repoRoot); err == nil {
		gitRepo = g
	}
	histSvc := services.NewHistoryService(histRepo, gitRepo)
	list, err := histSvc.GetIssueHistory(ctx, id)
	if err != nil {
		return err
	}
	if len(list.Entries) == 0 {
		return nil
	}
	fmt.Println("History:")
	end := len(list.Entries)
	start := end - limit
	if start < 0 {
		start = 0
	}
	for _, e := range list.Entries[start:end] {
		ts := e.Timestamp.Format("2006-01-02 15:04:05")
		fmt.Printf("  %s %s %s\n", ts, e.Type, e.Message)
	}
	return nil
}

// renderActivityView shows recent history entries using HistoryService
func renderActivityView() error {
	ctx := context.Background()
	repoRoot, err := findGitRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %v", err)
	}
	issuemapPath := filepath.Join(repoRoot, app.ConfigDirName)
	historyRepo := storage.NewFileHistoryRepository(issuemapPath)
	var gitRepo *git.GitClient
	if gitClient, err := git.NewGitClient(repoRoot); err == nil {
		gitRepo = gitClient
	}
	historyService := services.NewHistoryService(historyRepo, gitRepo)
	// Recent 10 entries
	limit := 10
	since := time.Now().AddDate(0, 0, -7)
	list, err := historyService.GetAllHistory(ctx, repositories.HistoryFilter{Since: &since, Limit: &limit})
	if err != nil {
		return fmt.Errorf("failed to load activity: %w", err)
	}
	if len(list.Entries) == 0 {
		fmt.Println("No recent activity")
		return nil
	}
	for _, e := range list.Entries {
		ts := e.Timestamp.Format("2006-01-02 15:04:05")
		fmt.Printf("%s %s %s %s\n", ts, e.IssueID, e.Type, e.Message)
	}
	return nil
}
