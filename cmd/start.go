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

var (
	startDescription string
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start <issue-id>",
	Short: "Start time tracking for an issue",
	Long: `Start a timer for tracking time spent on an issue. Only one timer can be active at a time.

Examples:
  issuemap start ISSUE-001
  issuemap start ISSUE-002 --description "Working on authentication"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Accept both numeric (e.g., 001) and full (ISSUE-001) formats
		issueID := normalizeIssueID(args[0])
		return runStart(cmd, issueID)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVarP(&startDescription, "description", "d", "", "description of work being done")
}

func runStart(cmd *cobra.Command, issueID entities.IssueID) error {
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

	// Start the timer
	activeTimer, err := timeTrackingService.StartTimer(ctx, issueID, author, startDescription)
	if err != nil {
		printError(fmt.Errorf("failed to start timer: %w", err))
		return err
	}

	// Also set status to in-progress automatically
	if _, err := issueService.UpdateIssue(ctx, issueID, map[string]interface{}{
		"status": string(entities.StatusInProgress),
	}); err != nil {
		printWarning(fmt.Sprintf("warning: failed to set status to in-progress: %v", err))
	}

	// Get issue for display
	issue, err := issueService.GetIssue(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("failed to get issue: %w", err))
		return err
	}

	// Display success message
	printSuccess(fmt.Sprintf("Started timer for %s", issueID))
	fmt.Printf("\nIssue: %s - %s\n", issue.ID, issue.Title)
	fmt.Printf("Status: %s\n", issue.Status)
	fmt.Printf("Author: %s\n", author)
	if startDescription != "" {
		fmt.Printf("Description: %s\n", startDescription)
	}
	fmt.Printf("Started at: %s\n", activeTimer.StartTime.Format("2006-01-02 15:04:05"))

	if issue.GetEstimatedHours() > 0 {
		fmt.Printf("Estimated: %.1f hours\n", issue.GetEstimatedHours())
		if issue.GetActualHours() > 0 {
			fmt.Printf("Previous time: %.1f hours\n", issue.GetActualHours())
		}
	}

	printInfo("Use 'issuemap stop' to stop the timer")

	return nil
}

func getCurrentUser(gitRepo *git.GitClient) string {
	ctx := context.Background()
	if gitRepo != nil {
		if author, err := gitRepo.GetAuthorInfo(ctx); err == nil && author != nil {
			if author.Username != "" {
				return author.Username
			}
			if author.Email != "" {
				return author.Email
			}
		}
	}
	return "unknown"
}
