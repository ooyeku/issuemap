package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	timeStartDescription string
	timeLogDescription   string
	timeLogHours         float64
)

// timeCmd represents the time command with subcommands
var timeCmd = &cobra.Command{
	Use:   "time",
	Short: "Time tracking management",
	Long: `Manage time tracking for issues with start, stop, and log subcommands.

Examples:
  issuemap time start ISSUE-001
  issuemap time stop
  issuemap time log ISSUE-001 2.5`,
}

// timeStartCmd represents the time start command
var timeStartCmd = &cobra.Command{
	Use:   "start <issue-id>",
	Short: "Start time tracking for an issue",
	Long: `Start a timer for tracking time spent on an issue. Only one timer can be active at a time.

Examples:
  issuemap time start ISSUE-001
  issuemap time start ISSUE-002 --description "Working on authentication"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID := normalizeIssueID(args[0])
		return runTimeStart(cmd, issueID)
	},
}

// timeStopCmd represents the time stop command
var timeStopCmd = &cobra.Command{
	Use:   "stop [issue-id]",
	Short: "Stop time tracking",
	Long: `Stop the currently active timer and log the time spent.

Examples:
  issuemap time stop
  issuemap time stop ISSUE-001
  issuemap time stop --force ISSUE-001         # Force stop any timer for the issue
  issuemap time stop --close ISSUE-001         # Stop timer and close the issue`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var maybeIssueID *entities.IssueID
		if len(args) == 1 {
			id := normalizeIssueID(args[0])
			maybeIssueID = &id
		}
		return runTimeStop(cmd, maybeIssueID)
	},
}

// timeLogCmd represents the time log command
var timeLogCmd = &cobra.Command{
	Use:   "log <issue-id> <hours>",
	Short: "Log time spent on an issue",
	Long: `Manually log time spent working on an issue.

Examples:
  issuemap time log ISSUE-001 2.5
  issuemap time log ISSUE-002 1.0 --description "Code review"
  issuemap time log --hours 0.5 --description "Bug investigation" ISSUE-003`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
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
			timeLogHours = hours
		}

		if timeLogHours <= 0 {
			return fmt.Errorf("hours must be greater than 0")
		}

		return runTimeLog(cmd, issueID)
	},
}

// timeReportCmd represents the time report command
var timeReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate time tracking reports",
	Long: `Generate time tracking reports for issues.

Examples:
  issuemap time report
  issuemap time report --since 2024-01-01
  issuemap time report --issue ISSUE-001`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Reuse the existing report command logic with time type
		reportType = "time"
		return runReport(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(timeCmd)

	// Add subcommands
	timeCmd.AddCommand(timeStartCmd)
	timeCmd.AddCommand(timeStopCmd)
	timeCmd.AddCommand(timeLogCmd)
	timeCmd.AddCommand(timeReportCmd)

	// Time start flags
	timeStartCmd.Flags().StringVarP(&timeStartDescription, "description", "d", "", "description of work being done")

	// Time stop flags
	timeStopCmd.Flags().Bool("force", false, "Force stop timer regardless of who started it")
	timeStopCmd.Flags().Bool("close", false, "Automatically close the issue after stopping timer")

	// Time log flags
	timeLogCmd.Flags().Float64Var(&timeLogHours, "hours", 0, "hours spent on the issue")
	timeLogCmd.Flags().StringVarP(&timeLogDescription, "description", "d", "", "description of work done")

	// Time report flags (reuse report flags)
	timeReportCmd.Flags().StringVar(&reportSince, "since", "", "report time since date (YYYY-MM-DD)")
	timeReportCmd.Flags().StringVar(&reportUntil, "until", "", "report time until date (YYYY-MM-DD)")
	timeReportCmd.Flags().StringVar(&reportGroupBy, "group-by", "issue", "group results by (issue, day, week, month)")
	timeReportCmd.Flags().StringVar(&reportIssue, "issue", "", "report time for specific issue")
	timeReportCmd.Flags().BoolVar(&reportDetailed, "detailed", false, "show detailed time entries")
}

func runTimeStart(cmd *cobra.Command, issueID entities.IssueID) error {
	ctx := context.Background()

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	basePath := filepath.Join(repoPath, ".issuemap")
	issueRepo := storage.NewFileIssueRepository(basePath)
	timeRepo := storage.NewFileTimeEntryRepository(basePath)
	configRepo := storage.NewFileConfigRepository(basePath)
	gitClient, err := git.NewGitClient(repoPath)
	if err != nil {
		printError(fmt.Errorf("failed to initialize git client: %w", err))
		return err
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitClient)
	activeTimerRepo := storage.NewFileActiveTimerRepository(basePath)
	historyRepo := storage.NewFileHistoryRepository(basePath)
	historyService := services.NewHistoryService(historyRepo, gitClient)
	timeService := services.NewTimeTrackingService(timeRepo, activeTimerRepo, issueService, historyService)

	// Check if issue exists
	issue, err := issueService.GetIssue(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("issue %s not found", issueID))
		return err
	}

	// Start timer
	author := getCurrentUser(gitClient)
	_, err = timeService.StartTimer(ctx, issueID, author, timeStartDescription)
	if err != nil {
		printError(err)
		return err
	}

	// Automatically set status to in-progress if it's currently open
	if issue.Status == entities.StatusOpen {
		updates := map[string]interface{}{
			"status": string(entities.StatusInProgress),
		}
		_, err = issueService.UpdateIssue(ctx, issueID, updates)
		if err != nil {
			printWarning(fmt.Sprintf("timer started but failed to update status to in-progress: %v", err))
		} else {
			printInfo("Status automatically changed to in-progress")
		}
	}

	printSuccess(fmt.Sprintf("Timer started for issue %s", issueID))
	if timeStartDescription != "" {
		fmt.Printf("Description: %s\n", timeStartDescription)
	}
	return nil
}

func runTimeStop(cmd *cobra.Command, maybeIssueID *entities.IssueID) error {
	ctx := context.Background()

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	basePath := filepath.Join(repoPath, ".issuemap")
	issueRepo := storage.NewFileIssueRepository(basePath)
	timeRepo := storage.NewFileTimeEntryRepository(basePath)
	configRepo := storage.NewFileConfigRepository(basePath)
	gitClient, err := git.NewGitClient(repoPath)
	if err != nil {
		printError(fmt.Errorf("failed to initialize git client: %w", err))
		return err
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitClient)
	activeTimerRepo := storage.NewFileActiveTimerRepository(basePath)
	historyRepo := storage.NewFileHistoryRepository(basePath)
	historyService := services.NewHistoryService(historyRepo, gitClient)
	timeService := services.NewTimeTrackingService(timeRepo, activeTimerRepo, issueService, historyService)

	force, _ := cmd.Flags().GetBool("force")
	closeIssue, _ := cmd.Flags().GetBool("close")

	// Stop timer
	var entry *entities.TimeEntry
	author := getCurrentUser(gitClient)

	if maybeIssueID != nil && force {
		entry, err = timeService.ForceStopTimer(ctx, *maybeIssueID, author)
	} else {
		entry, err = timeService.StopTimer(ctx, author)
	}

	if err != nil {
		printError(err)
		return err
	}

	if entry != nil {
		duration := entry.EndTime.Sub(entry.StartTime)
		hours := duration.Hours()
		printSuccess(fmt.Sprintf("Timer stopped for issue %s", entry.IssueID))
		fmt.Printf("Time logged: %.2f hours\n", hours)
		if entry.Description != "" {
			fmt.Printf("Description: %s\n", entry.Description)
		}

		// Close issue if requested
		if closeIssue && maybeIssueID != nil {
			now := time.Now()
			updates := map[string]interface{}{
				"status":             entities.StatusClosed,
				"timestamps.closed":  &now,
				"timestamps.updated": now,
			}
			_, err = issueService.UpdateIssue(ctx, *maybeIssueID, updates)
			if err == nil {
				printSuccess(fmt.Sprintf("Issue %s closed", *maybeIssueID))
			}
		}
	}

	return nil
}

func runTimeLog(cmd *cobra.Command, issueID entities.IssueID) error {
	ctx := context.Background()

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	basePath := filepath.Join(repoPath, ".issuemap")
	issueRepo := storage.NewFileIssueRepository(basePath)
	timeRepo := storage.NewFileTimeEntryRepository(basePath)
	configRepo := storage.NewFileConfigRepository(basePath)
	gitClient, err := git.NewGitClient(repoPath)
	if err != nil {
		printError(fmt.Errorf("failed to initialize git client: %w", err))
		return err
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitClient)
	activeTimerRepo := storage.NewFileActiveTimerRepository(basePath)
	historyRepo := storage.NewFileHistoryRepository(basePath)
	historyService := services.NewHistoryService(historyRepo, gitClient)
	timeService := services.NewTimeTrackingService(timeRepo, activeTimerRepo, issueService, historyService)

	// Check if issue exists
	_, err = issueService.GetIssue(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("issue %s not found", issueID))
		return err
	}

	// Log time
	duration := time.Duration(timeLogHours * float64(time.Hour))
	author := getCurrentUser(gitClient)
	entry, err := timeService.LogTime(ctx, issueID, author, duration, timeLogDescription)
	if err != nil {
		printError(err)
		return err
	}

	printSuccess(fmt.Sprintf("Logged %.2f hours for issue %s", timeLogHours, issueID))
	if entry.Description != "" {
		fmt.Printf("Description: %s\n", entry.Description)
	}
	return nil
}
