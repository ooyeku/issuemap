package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	estimateHours float64
)

// estimateCmd represents the estimate command
var estimateCmd = &cobra.Command{
	Use:   "estimate <issue-id> <hours>",
	Short: "Set time estimate for an issue",
	Long: `Set the estimated time for completing an issue in hours.

Examples:
  issuemap estimate ISSUE-001 8.5
  issuemap estimate ISSUE-002 2.0
  issuemap estimate --hours 4.5 ISSUE-003`,
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
			estimateHours = hours
		}

		if estimateHours <= 0 {
			return fmt.Errorf("hours must be greater than 0")
		}

		return runEstimate(cmd, issueID, estimateHours)
	},
}

func init() {
	rootCmd.AddCommand(estimateCmd)
	estimateCmd.Flags().Float64Var(&estimateHours, "hours", 0, "estimated hours")
}

func runEstimate(cmd *cobra.Command, issueID entities.IssueID, hours float64) error {
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

	// Get the issue
	issue, err := issueService.GetIssue(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("failed to get issue: %w", err))
		return err
	}

	// Update the issue with estimate
	updates := map[string]interface{}{
		"estimated_hours": hours,
	}

	issue, err = issueService.UpdateIssue(ctx, issueID, updates)
	if err != nil {
		printError(fmt.Errorf("failed to update issue: %w", err))
		return err
	}

	// Display success message
	printSuccess(fmt.Sprintf("Set estimate for %s to %.1f hours", issueID, hours))

	// Display current status
	fmt.Printf("\nIssue: %s - %s\n", issue.ID, issue.Title)
	fmt.Printf("Estimated: %.1f hours\n", issue.GetEstimatedHours())
	
	if issue.GetActualHours() > 0 {
		fmt.Printf("Actual: %.1f hours\n", issue.GetActualHours())
		fmt.Printf("Remaining: %.1f hours\n", issue.GetRemainingHours())
		
		if issue.IsOverEstimate() {
			printWarning("⚠️  Actual time exceeds estimate")
		}
	}

	return nil
}