package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	syncBranch     string
	syncPush       bool
	syncPull       bool
	syncAll        bool
	syncAutoUpdate bool
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize branch and issue status",
	Long: `Synchronize branch status with issue tracking and optionally 
push/pull changes from remote repositories.

This command helps maintain consistency between Git branches and issue status,
automatically updating issue status based on branch activity.

Examples:
  issuemap sync                          # Sync current branch
  issuemap sync --branch feature-001    # Sync specific branch
  issuemap sync --all                   # Sync all branches
  issuemap sync --push                  # Sync and push changes
  issuemap sync --pull                  # Sync and pull changes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd, args)
	},
}

// syncStatusCmd shows sync status for branches
var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show branch synchronization status",
	Long: `Display the synchronization status of Git branches and their
associated issues, including upstream tracking, unpushed commits,
and issue status alignment.

Examples:
  issuemap sync status                   # Show current branch status
  issuemap sync status --all             # Show all branches
  issuemap sync status --branch main     # Show specific branch`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSyncStatus(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncStatusCmd)

	// Sync command flags
	syncCmd.Flags().StringVarP(&syncBranch, "branch", "b", "", "specific branch to sync (defaults to current)")
	syncCmd.Flags().BoolVar(&syncPush, "push", false, "push changes to remote after sync")
	syncCmd.Flags().BoolVar(&syncPull, "pull", false, "pull changes from remote before sync")
	syncCmd.Flags().BoolVar(&syncAll, "all", false, "sync all branches")
	syncCmd.Flags().BoolVar(&syncAutoUpdate, "auto-update", false, "automatically update issue status based on branch activity")

	// Sync status command flags
	syncStatusCmd.Flags().StringVarP(&syncBranch, "branch", "b", "", "specific branch to check")
	syncStatusCmd.Flags().BoolVar(&syncAll, "all", false, "show status for all branches")
}

func runSync(cmd *cobra.Command, args []string) error {
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

	// Determine which branches to sync
	var branchesToSync []string

	if syncAll {
		branches, err := gitClient.GetBranches(ctx)
		if err != nil {
			printError(fmt.Errorf("failed to get branches: %w", err))
			return err
		}
		branchesToSync = branches
	} else if syncBranch != "" {
		branchesToSync = []string{syncBranch}
	} else {
		// Use current branch
		currentBranch, err := gitClient.GetCurrentBranch(ctx)
		if err != nil {
			printError(fmt.Errorf("failed to get current branch: %w", err))
			return err
		}
		branchesToSync = []string{currentBranch}
	}

	printSectionHeader("Branch Synchronization")

	syncedCount := 0
	errorCount := 0

	for _, branch := range branchesToSync {
		fmt.Printf("\n📊 Syncing branch: %s\n", branch)

		err := syncBranchWithIssues(ctx, branch, gitClient, issueService)
		if err != nil {
			printWarning(fmt.Sprintf("Failed to sync branch %s: %v", branch, err))
			errorCount++
			continue
		}

		syncedCount++
		printSuccess(fmt.Sprintf("Branch %s synced successfully", branch))
	}

	// Summary
	fmt.Printf("\n📈 Sync Summary:\n")
	fmt.Printf("  Branches synced: %d\n", syncedCount)
	if errorCount > 0 {
		fmt.Printf("  Errors: %d\n", errorCount)
	}

	return nil
}

func runSyncStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	issueRepo := storage.NewFileIssueRepository(issuemapPath)

	gitClient, err := git.NewGitClient(repoPath)
	if err != nil {
		printError(fmt.Errorf("failed to initialize git client: %w", err))
		return err
	}

	// Determine which branches to check
	var branchesToCheck []string

	if syncAll {
		branches, err := gitClient.GetBranches(ctx)
		if err != nil {
			printError(fmt.Errorf("failed to get branches: %w", err))
			return err
		}
		branchesToCheck = branches
	} else if syncBranch != "" {
		branchesToCheck = []string{syncBranch}
	} else {
		// Use current branch
		currentBranch, err := gitClient.GetCurrentBranch(ctx)
		if err != nil {
			printError(fmt.Errorf("failed to get current branch: %w", err))
			return err
		}
		branchesToCheck = []string{currentBranch}
	}

	printSectionHeader("Branch Synchronization Status")

	for _, branch := range branchesToCheck {
		fmt.Printf("\n🌟 Branch: %s\n", branch)

		// Get branch status
		status, err := gitClient.GetBranchStatus(ctx, branch)
		if err != nil {
			printWarning(fmt.Sprintf("Failed to get status for branch %s: %v", branch, err))
			continue
		}

		if !status.Exists {
			fmt.Printf("  ❌ Branch does not exist\n")
			continue
		}

		// Branch tracking info
		if status.IsTracked {
			fmt.Printf("  📡 Tracked: Yes\n")
			if status.HasUnpushed {
				fmt.Printf("  ⬆️  Ahead by: %d commits\n", status.AheadBy)
			}
			if status.HasUnpulled {
				fmt.Printf("  ⬇️  Behind by: %d commits\n", status.BehindBy)
			}
			if !status.HasUnpushed && !status.HasUnpulled {
				fmt.Printf("  ✅ Up to date with origin\n")
			}
		} else {
			fmt.Printf("  📡 Tracked: No (local branch only)\n")
		}

		// Last commit info
		if status.LastCommit != "" {
			fmt.Printf("  📝 Last commit: %s - %s\n", status.LastCommit, status.LastCommitMsg)
		}

		// Associated issue info
		issueID := extractIssueFromBranch(branch)
		if issueID != "" {
			issue, err := issueRepo.GetByID(ctx, entities.IssueID(issueID))
			if err == nil {
				fmt.Printf("  🎫 Associated issue: %s - %s (%s)\n",
					issue.ID, issue.Title, issue.Status)

				// Check for sync issues
				if branch != "main" && branch != "master" && issue.Status == entities.StatusClosed {
					fmt.Printf("  ⚠️  Warning: Issue is closed but branch still exists\n")
				}
			} else {
				fmt.Printf("  🎫 Associated issue: %s (not found)\n", issueID)
			}
		} else {
			fmt.Printf("  🎫 Associated issue: None detected\n")
		}
	}

	return nil
}

func syncBranchWithIssues(ctx context.Context, branch string, gitClient *git.GitClient, issueService *services.IssueService) error {
	// Get branch status
	status, err := gitClient.GetBranchStatus(ctx, branch)
	if err != nil {
		return fmt.Errorf("failed to get branch status: %w", err)
	}

	if !status.Exists {
		return fmt.Errorf("branch %s does not exist", branch)
	}

	// Pull changes if requested
	if syncPull && status.IsTracked {
		fmt.Printf("  ⬇️  Pulling changes from origin...\n")
		err := gitClient.PullBranch(ctx, branch)
		if err != nil {
			printWarning(fmt.Sprintf("Failed to pull changes: %v", err))
		} else {
			fmt.Printf("  ✅ Pull completed\n")
		}
	}

	// Check for associated issue
	issueID := extractIssueFromBranch(branch)
	if issueID != "" {
		issue, err := issueService.GetIssue(ctx, entities.IssueID(issueID))
		if err == nil {
			fmt.Printf("  🎫 Found associated issue: %s - %s\n", issue.ID, issue.Title)

			// Auto-update issue status if requested
			if syncAutoUpdate {
				err := autoUpdateIssueStatus(ctx, issue, status, issueService)
				if err != nil {
					printWarning(fmt.Sprintf("Failed to auto-update issue status: %v", err))
				}
			}

			// Update issue branch reference
			if issue.Branch != branch {
				fmt.Printf("  📝 Updating issue branch reference to %s\n", branch)
				updates := map[string]interface{}{
					"branch": branch,
				}
				_, err := issueService.UpdateIssue(ctx, issue.ID, updates)
				if err != nil {
					printWarning(fmt.Sprintf("Failed to update issue branch: %v", err))
				}
			}
		}
	}

	// Push changes if requested
	if syncPush && status.IsTracked {
		fmt.Printf("  ⬆️  Pushing changes to origin...\n")
		err := gitClient.PushBranch(ctx, branch)
		if err != nil {
			printWarning(fmt.Sprintf("Failed to push changes: %v", err))
		} else {
			fmt.Printf("  ✅ Push completed\n")
		}
	}

	return nil
}

func autoUpdateIssueStatus(ctx context.Context, issue *entities.Issue, branchStatus *repositories.BranchStatus, issueService *services.IssueService) error {
	// Logic for auto-updating issue status based on branch activity

	// If branch has recent commits and issue is still open, mark as in progress
	if issue.Status == entities.StatusOpen && branchStatus.LastCommit != "" {
		fmt.Printf("  🔄 Auto-updating issue status to in-progress (has commits)\n")
		updates := map[string]interface{}{
			"status": string(entities.StatusInProgress),
		}
		_, err := issueService.UpdateIssue(ctx, issue.ID, updates)
		return err
	}

	// If branch is merged and issue is still open/in-progress, consider closing
	// This would require more sophisticated merge detection
	// For now, we'll keep it simple and not auto-close

	return nil
}
