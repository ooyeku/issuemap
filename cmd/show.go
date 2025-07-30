package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show <issue-id>",
	Short: "Show detailed information about an issue",
	Long: `Show detailed information about a specific issue including title, description,
comments, commits, and all metadata.

Examples:
  issuemap show ISSUE-001
  issuemap show ISSUE-001 --format json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runShow(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	issueID := entities.IssueID(args[0])

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
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

	// Display the issue
	displayIssueDetails(issue)
	return nil
}

func displayIssueDetails(issue *entities.Issue) {
	fmt.Printf("Issue %s\n", issue.ID)
	fmt.Printf("Title: %s\n", issue.Title)
	fmt.Printf("Type: %s\n", issue.Type)
	fmt.Printf("Status: %s\n", issue.Status)
	fmt.Printf("Priority: %s\n", issue.Priority)

	if issue.Assignee != nil {
		fmt.Printf("Assignee: %s (%s)\n", issue.Assignee.Username, issue.Assignee.Email)
	} else {
		fmt.Printf("Assignee: Unassigned\n")
	}

	if len(issue.Labels) > 0 {
		var labelNames []string
		for _, label := range issue.Labels {
			labelNames = append(labelNames, label.Name)
		}
		fmt.Printf("Labels: %s\n", strings.Join(labelNames, ", "))
	}

	if issue.Milestone != nil {
		fmt.Printf("Milestone: %s", issue.Milestone.Name)
		if issue.Milestone.DueDate != nil {
			fmt.Printf(" (due %s)", issue.Milestone.DueDate.Format("2006-01-02"))
		}
		fmt.Println()
	}

	if issue.Branch != "" {
		fmt.Printf("Branch: %s\n", issue.Branch)
	}

	fmt.Printf("Created: %s\n", issue.Timestamps.Created.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s\n", issue.Timestamps.Updated.Format("2006-01-02 15:04:05"))

	if issue.Timestamps.Closed != nil {
		fmt.Printf("Closed: %s\n", issue.Timestamps.Closed.Format("2006-01-02 15:04:05"))
	}

	if issue.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", issue.Description)
	}

	if len(issue.Commits) > 0 {
		fmt.Printf("\nCommits (%d):\n", len(issue.Commits))
		for _, commit := range issue.Commits {
			fmt.Printf("  %s - %s (%s)\n",
				commit.Hash[:8],
				commit.Message[:min(50, len(commit.Message))],
				commit.Author)
		}
	}

	if len(issue.Comments) > 0 {
		fmt.Printf("\nComments (%d):\n", len(issue.Comments))
		for _, comment := range issue.Comments {
			fmt.Printf("\n--- Comment #%d by %s on %s ---\n",
				comment.ID,
				comment.Author,
				comment.Date.Format("2006-01-02 15:04:05"))
			fmt.Printf("%s\n", comment.Text)
		}
	}

	if issue.Metadata.EstimatedHours != nil {
		fmt.Printf("\nEstimated Hours: %.1f\n", *issue.Metadata.EstimatedHours)
	}
	if issue.Metadata.ActualHours != nil {
		fmt.Printf("Actual Hours: %.1f\n", *issue.Metadata.ActualHours)
	}

	if len(issue.Metadata.CustomFields) > 0 {
		fmt.Printf("\nCustom Fields:\n")
		for key, value := range issue.Metadata.CustomFields {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
