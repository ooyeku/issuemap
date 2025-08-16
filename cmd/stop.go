package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop [issue-id]",
	Short: "Stop time tracking",
	Long: `Stop the currently active timer and log the time spent.

Examples:
  issuemap stop
  issuemap stop ISSUE-001
  issuemap stop --force ISSUE-001         # Force stop any timer for the issue
  issuemap stop --close ISSUE-001         # Stop timer and close the issue
  issuemap stop --force --close ISSUE-001 # Force stop and close the issue`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var maybeIssueID *entities.IssueID
		if len(args) == 1 {
			id := normalizeIssueID(args[0])
			maybeIssueID = &id
		}
		return runStop(cmd, maybeIssueID)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().Bool("force", false, "Force stop timer regardless of who started it")
	stopCmd.Flags().Bool("close", false, "Automatically close the issue after stopping timer")
}

func runStop(cmd *cobra.Command, maybeIssueID *entities.IssueID) error {
	ctx := context.Background()

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, app.ConfigDirName)
	issueRepo := storage.NewFileIssueRepository(issuemapPath)
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	var gitRepo *git.GitClient
	if gitClient, err := git.NewGitClient(repoPath); err == nil {
		gitRepo = gitClient
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitRepo)

	// Initialize time tracking repositories and service
	timeEntryRepo := storage.NewFileTimeEntryRepository(issuemapPath)
	activeTimerRepo := storage.NewFileActiveTimerRepository(issuemapPath)

	// Create history service for time tracking
	historyRepo := storage.NewFileHistoryRepository(issuemapPath)
	historyService := services.NewHistoryService(historyRepo, gitRepo)

	timeTrackingService := services.NewTimeTrackingService(
		timeEntryRepo,
		activeTimerRepo,
		issueService,
		historyService,
	)

	// Get current user
	author := getCurrentUser(gitRepo)

	// Check if force flag is set
	force, _ := cmd.Flags().GetBool("force")
	closeIssue, _ := cmd.Flags().GetBool("close")

	var timeEntry *entities.TimeEntry

	if force {
		// Force mode: requires issue ID and stops any timer for that issue
		if maybeIssueID == nil {
			printError(fmt.Errorf("issue ID is required when using --force flag"))
			return fmt.Errorf("issue ID required for force stop")
		}

		timeEntry, err = timeTrackingService.ForceStopTimer(ctx, *maybeIssueID, author)
		if err != nil {
			printError(fmt.Errorf("failed to force stop timer: %w", err))
			return err
		}
	} else {
		// Normal mode: stop the user's own active timer
		timeEntry, err = timeTrackingService.StopTimer(ctx, author)
		if err != nil {
			printError(fmt.Errorf("failed to stop timer: %w", err))
			return err
		}

		if maybeIssueID != nil && timeEntry.IssueID != *maybeIssueID {
			printError(fmt.Errorf("active timer is for %s, not %s",
				timeEntry.IssueID, *maybeIssueID))
			return fmt.Errorf("timer issue mismatch")
		}
	}

	// Get issue for display
	issue, err := issueService.GetIssue(ctx, timeEntry.IssueID)
	if err != nil {
		printError(fmt.Errorf("failed to get issue: %w", err))
		return err
	}

	// Display success message
	printSuccess(fmt.Sprintf("Stopped timer for %s", timeEntry.IssueID))

	fmt.Printf("\nIssue: %s - %s\n", issue.ID, issue.Title)
	fmt.Printf("Author: %s\n", author)
	if timeEntry.Description != "" {
		fmt.Printf("Description: %s\n", timeEntry.Description)
	}
	fmt.Printf("Duration: %.1f hours\n", timeEntry.GetDurationHours())
	fmt.Printf("Started: %s\n", timeEntry.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("Stopped: %s\n", timeEntry.EndTime.Format("2006-01-02 15:04:05"))

	fmt.Printf("Total actual: %.1f hours\n", issue.GetActualHours())

	if issue.GetEstimatedHours() > 0 {
		fmt.Printf("Estimated: %.1f hours\n", issue.GetEstimatedHours())
		fmt.Printf("Remaining: %.1f hours\n", issue.GetRemainingHours())

		progress := (issue.GetActualHours() / issue.GetEstimatedHours()) * 100
		fmt.Printf("Progress: %.1f%%\n", progress)

		if issue.IsOverEstimate() {
			printWarning("Actual time exceeds estimate")
		}
	}

	// Close issue if --close flag is set
	if closeIssue {
		fmt.Printf("\nClosing issue %s...\n", timeEntry.IssueID)
		err = issueService.CloseIssue(ctx, timeEntry.IssueID, "Closed after stopping timer")
		if err != nil {
			printError(fmt.Errorf("failed to close issue: %w", err))
			return err
		}
		printSuccess(fmt.Sprintf("Issue %s closed successfully", timeEntry.IssueID))
	}

	return nil
}
