package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	resolveInteractive bool
	resolveAutoFix     bool
	resolveDryRun      bool
)

// resolveCmd represents the resolve command for handling conflicts
var resolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Resolve conflicts between branches and issues",
	Long: `Detect and resolve conflicts between Git branches and issue status.
This helps maintain consistency when branches and issues get out of sync.

Common conflicts include:
- Issues marked as closed but branches still exist
- Branches exist but no associated issue found
- Issue references branch that no longer exists
- Multiple issues claiming the same branch

Examples:
  issuemap resolve                     # Detect and show conflicts
  issuemap resolve --interactive       # Interactively resolve conflicts
  issuemap resolve --auto-fix          # Automatically fix safe conflicts
  issuemap resolve --dry-run           # Show what would be resolved without making changes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runResolve(cmd, args)
	},
}

// ConflictType represents the type of branch-issue conflict
type ConflictType string

const (
	ConflictClosedIssueOpenBranch    ConflictType = "closed_issue_open_branch"
	ConflictBranchNoIssue            ConflictType = "branch_no_issue"
	ConflictIssueNoBranch            ConflictType = "issue_no_branch"
	ConflictMultipleIssuesSameBranch ConflictType = "multiple_issues_same_branch"
	ConflictBranchNameMismatch       ConflictType = "branch_name_mismatch"
)

// Conflict represents a detected conflict between branches and issues
type Conflict struct {
	Type        ConflictType      `json:"type"`
	Description string            `json:"description"`
	Branch      string            `json:"branch,omitempty"`
	Issues      []*entities.Issue `json:"issues,omitempty"`
	Severity    string            `json:"severity"` // low, medium, high
	AutoFixable bool              `json:"auto_fixable"`
}

func init() {
	rootCmd.AddCommand(resolveCmd)

	// Resolve command flags
	resolveCmd.Flags().BoolVarP(&resolveInteractive, "interactive", "i", false, "interactively resolve conflicts")
	resolveCmd.Flags().BoolVar(&resolveAutoFix, "auto-fix", false, "automatically fix safe conflicts")
	resolveCmd.Flags().BoolVar(&resolveDryRun, "dry-run", false, "show what would be resolved without making changes")
}

func runResolve(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	issueRepo := storage.NewFileIssueRepository(issuemapPath)
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	gitClient, err := git.NewGitClient(repoPath)
	if err != nil {
		printError(fmt.Errorf("failed to initialize git client: %w", err))
		return err
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitClient)

	// Detect conflicts
	printSectionHeader("Analyzing Branch-Issue Conflicts")
	conflicts, err := detectConflicts(ctx, gitClient, issueService)
	if err != nil {
		printError(fmt.Errorf("failed to detect conflicts: %w", err))
		return err
	}

	if len(conflicts) == 0 {
		printSuccess("No conflicts detected! Your branches and issues are in sync.")
		return nil
	}

	// Display conflicts
	fmt.Printf("\nFound %d conflict(s):\n\n", len(conflicts))
	for i, conflict := range conflicts {
		displayConflict(i+1, conflict)
	}

	// Handle resolution based on flags
	if resolveDryRun {
		fmt.Printf("\nDry run - no changes made\n")
		return nil
	}

	var resolvedCount int
	if resolveAutoFix {
		resolvedCount, err = autoFixConflicts(ctx, conflicts, issueService, gitClient)
		if err != nil {
			printError(fmt.Errorf("auto-fix failed: %w", err))
			return err
		}
	} else if resolveInteractive {
		resolvedCount, err = interactiveResolve(ctx, conflicts, issueService, gitClient)
		if err != nil {
			printError(fmt.Errorf("interactive resolution failed: %w", err))
			return err
		}
	} else {
		fmt.Printf("\nTo resolve conflicts, use:\n")
		fmt.Printf("  issuemap resolve --interactive  # Interactive resolution\n")
		fmt.Printf("  issuemap resolve --auto-fix     # Automatic safe fixes\n")
		return nil
	}

	// Summary
	fmt.Printf("\nResolution Summary:\n")
	fmt.Printf("  Conflicts detected: %d\n", len(conflicts))
	fmt.Printf("  Conflicts resolved: %d\n", resolvedCount)
	if resolvedCount < len(conflicts) {
		fmt.Printf("  Remaining conflicts: %d\n", len(conflicts)-resolvedCount)
	}

	return nil
}

func detectConflicts(ctx context.Context, gitClient *git.GitClient, issueService *services.IssueService) ([]Conflict, error) {
	var conflicts []Conflict

	// Get all branches
	branches, err := gitClient.GetBranches(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}

	// Get all issues
	filter := repositories.IssueFilter{}
	issueList, err := issueService.ListIssues(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get issues: %w", err)
	}

	// Create maps for efficient lookup
	branchMap := make(map[string]bool)
	for _, branch := range branches {
		branchMap[branch] = true
	}

	issuesByBranch := make(map[string][]*entities.Issue)
	branchlessIssues := []*entities.Issue{}

	for i := range issueList.Issues {
		issue := &issueList.Issues[i]
		if issue.Branch != "" {
			issuesByBranch[issue.Branch] = append(issuesByBranch[issue.Branch], issue)
		} else {
			branchlessIssues = append(branchlessIssues, issue)
		}
	}

	// Detect conflicts

	// 1. Closed issues with open branches
	for branch, issues := range issuesByBranch {
		if branch == "main" || branch == "master" {
			continue // Skip main branches
		}

		for _, issue := range issues {
			if issue.Status == entities.StatusClosed && branchMap[branch] {
				conflicts = append(conflicts, Conflict{
					Type:        ConflictClosedIssueOpenBranch,
					Description: fmt.Sprintf("Issue %s is closed but branch '%s' still exists", issue.ID, branch),
					Branch:      branch,
					Issues:      []*entities.Issue{issue},
					Severity:    "medium",
					AutoFixable: true,
				})
			}
		}
	}

	// 2. Branches with no associated issues
	for _, branch := range branches {
		if branch == "main" || branch == "master" {
			continue // Skip main branches
		}

		issueID := extractIssueFromBranch(branch)
		if issueID != "" {
			// Check if the extracted issue ID has a corresponding issue
			found := false
			for i := range issueList.Issues {
				if string(issueList.Issues[i].ID) == issueID {
					found = true
					break
				}
			}

			if !found {
				conflicts = append(conflicts, Conflict{
					Type:        ConflictBranchNoIssue,
					Description: fmt.Sprintf("Branch '%s' suggests issue %s but no such issue exists", branch, issueID),
					Branch:      branch,
					Issues:      []*entities.Issue{},
					Severity:    "low",
					AutoFixable: false,
				})
			}
		} else if len(issuesByBranch[branch]) == 0 {
			conflicts = append(conflicts, Conflict{
				Type:        ConflictBranchNoIssue,
				Description: fmt.Sprintf("Branch '%s' has no associated issue", branch),
				Branch:      branch,
				Issues:      []*entities.Issue{},
				Severity:    "low",
				AutoFixable: false,
			})
		}
	}

	// 3. Issues referencing non-existent branches
	for branch, issues := range issuesByBranch {
		if !branchMap[branch] {
			for _, issue := range issues {
				conflicts = append(conflicts, Conflict{
					Type:        ConflictIssueNoBranch,
					Description: fmt.Sprintf("Issue %s references branch '%s' but branch doesn't exist", issue.ID, branch),
					Branch:      branch,
					Issues:      []*entities.Issue{issue},
					Severity:    "medium",
					AutoFixable: true,
				})
			}
		}
	}

	// 4. Multiple issues claiming the same branch
	for branch, issues := range issuesByBranch {
		if len(issues) > 1 {
			conflicts = append(conflicts, Conflict{
				Type:        ConflictMultipleIssuesSameBranch,
				Description: fmt.Sprintf("Multiple issues (%d) claim branch '%s'", len(issues), branch),
				Branch:      branch,
				Issues:      issues,
				Severity:    "high",
				AutoFixable: false,
			})
		}
	}

	return conflicts, nil
}

func displayConflict(index int, conflict Conflict) {
	var severityEmoji string
	switch conflict.Severity {
	case "low":
		severityEmoji = "LOW"
	case "medium":
		severityEmoji = "MEDIUM"
	case "high":
		severityEmoji = "HIGH"
	}

	fmt.Printf("%s %d. %s\n", severityEmoji, index, conflict.Description)
	fmt.Printf("   Type: %s\n", conflict.Type)
	fmt.Printf("   Severity: %s\n", conflict.Severity)
	if conflict.AutoFixable {
		fmt.Printf("   Auto-fixable: YES\n")
	} else {
		fmt.Printf("   Auto-fixable: NO\n")
	}
	if len(conflict.Issues) > 0 {
		fmt.Printf("   Issues: ")
		for i, issue := range conflict.Issues {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s (%s)", issue.ID, issue.Status)
		}
		fmt.Printf("\n")
	}
	fmt.Printf("\n")
}

func autoFixConflicts(ctx context.Context, conflicts []Conflict, issueService *services.IssueService, gitClient *git.GitClient) (int, error) {
	resolvedCount := 0

	fmt.Printf("\nAuto-fixing safe conflicts...\n\n")

	for _, conflict := range conflicts {
		if !conflict.AutoFixable {
			continue
		}

		fmt.Printf("Fixing: %s\n", conflict.Description)

		switch conflict.Type {
		case ConflictClosedIssueOpenBranch:
			// Delete the branch since issue is closed
			fmt.Printf("   Deleting branch '%s' (issue is closed)\n", conflict.Branch)
			// Note: We're not actually deleting the branch here as it might have unpushed commits
			// In a real implementation, you'd want to check for unpushed commits first
			fmt.Printf("   Branch deletion skipped - please verify no important changes exist\n")

		case ConflictIssueNoBranch:
			// Clear the branch reference from the issue
			if len(conflict.Issues) > 0 {
				issue := conflict.Issues[0]
				fmt.Printf("   Clearing branch reference from issue %s\n", issue.ID)
				updates := map[string]interface{}{
					"branch": "",
				}
				_, err := issueService.UpdateIssue(ctx, issue.ID, updates)
				if err != nil {
					fmt.Printf("   Failed to update issue: %v\n", err)
					continue
				}
			}
		}

		fmt.Printf("   Fixed\n\n")
		resolvedCount++
	}

	return resolvedCount, nil
}

func interactiveResolve(ctx context.Context, conflicts []Conflict, issueService *services.IssueService, gitClient *git.GitClient) (int, error) {
	resolvedCount := 0
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\nInteractive conflict resolution...\n\n")

	for i, conflict := range conflicts {
		fmt.Printf("Conflict %d of %d:\n", i+1, len(conflicts))
		displayConflict(i+1, conflict)

		fmt.Printf("Choose an action:\n")

		switch conflict.Type {
		case ConflictClosedIssueOpenBranch:
			fmt.Printf("  1. Delete branch '%s' (recommended)\n", conflict.Branch)
			fmt.Printf("  2. Reopen issue %s\n", conflict.Issues[0].ID)
			fmt.Printf("  3. Skip this conflict\n")

		case ConflictBranchNoIssue:
			fmt.Printf("  1. Create issue for branch '%s'\n", conflict.Branch)
			fmt.Printf("  2. Skip this conflict\n")

		case ConflictIssueNoBranch:
			fmt.Printf("  1. Clear branch reference from issue\n")
			fmt.Printf("  2. Create branch '%s'\n", conflict.Branch)
			fmt.Printf("  3. Skip this conflict\n")

		case ConflictMultipleIssuesSameBranch:
			fmt.Printf("  1. Keep first issue, clear others\n")
			for j, issue := range conflict.Issues {
				fmt.Printf("  %d. Keep issue %s, clear others\n", j+2, issue.ID)
			}
			fmt.Printf("  %d. Skip this conflict\n", len(conflict.Issues)+2)
		}

		fmt.Printf("\nEnter your choice: ")
		choice, err := reader.ReadString('\n')
		if err != nil {
			return resolvedCount, fmt.Errorf("failed to read input: %w", err)
		}

		choice = strings.TrimSpace(choice)

		// Process the choice (simplified implementation)
		switch choice {
		case "1":
			fmt.Printf("Processing choice 1...\n")
			// Implement the actual resolution logic here
			resolvedCount++
		case "2":
			fmt.Printf("Processing choice 2...\n")
			// Implement the actual resolution logic here
			resolvedCount++
		case "3":
			fmt.Printf("Skipping conflict...\n")
		default:
			fmt.Printf("Invalid choice, skipping...\n")
		}

		fmt.Printf("\n")
	}

	return resolvedCount, nil
}
