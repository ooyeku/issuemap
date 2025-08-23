package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	logDescription string
	logHours       float64
)

// logCmd represents the log command
var logCmd = &cobra.Command{
	Use:   "log <issue-id> <hours>",
	Short: "Log time spent on an issue",
	Long: `Manually log time spent working on an issue.

Examples:
  issuemap log ISSUE-001 2.5
  issuemap log ISSUE-002 1.0 --description "Code review"
  issuemap log --hours 0.5 --description "Bug investigation" ISSUE-003`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Deprecation warning
		printWarning("DEPRECATED: 'log' command will be removed in a future version. Use 'time log' instead:")
		printWarning("  issuemap time log ISSUE-001 2.5")
		fmt.Println()

		if len(args) < 1 {
			return fmt.Errorf("issue ID is required")
		}

		issueID := entities.IssueID(args[0])

		// If hours provided as argument, use it; otherwise use flag
		if len(args) == 2 {
			hours, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return fmt.Errorf("invalid hours value: %w", err)
			}
			logHours = hours
		}

		if logHours <= 0 {
			return fmt.Errorf("hours must be greater than 0")
		}

		return runLog(cmd, issueID, logHours, logDescription)
	},
}

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().Float64Var(&logHours, "hours", 0, "hours to log")
	logCmd.Flags().StringVarP(&logDescription, "description", "d", "", "description of work done")
}

func runLog(cmd *cobra.Command, issueID entities.IssueID, hours float64, description string) error {
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

	// Log the time
	duration := time.Duration(hours * float64(time.Hour))
	_, err = timeTrackingService.LogTime(ctx, issueID, author, duration, description)
	if err != nil {
		printError(fmt.Errorf("failed to log time: %w", err))
		return err
	}

	// Get updated issue for display
	issue, err := issueService.GetIssue(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("failed to get issue: %w", err))
		return err
	}

	// Display success message
	printSuccess(fmt.Sprintf("Logged %.1f hours for %s", hours, issueID))

	// Display current status
	fmt.Printf("\nIssue: %s - %s\n", issue.ID, issue.Title)
	fmt.Printf("Author: %s\n", author)
	if description != "" {
		fmt.Printf("Description: %s\n", description)
	}
	fmt.Printf("Time logged: %.1f hours\n", hours)
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

	return nil
}
