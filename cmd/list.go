package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	listStatus    string
	listType      string
	listPriority  string
	listAssignee  string
	listLabels    []string
	listMilestone string
	listBranch    string
	listLimit     int
	listAll       bool
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues",
	Long: `List issues with optional filtering.

Examples:
  issuemap list
  issuemap list --status open
  issuemap list --type bug --priority high
  issuemap list --assignee username
  issuemap list --labels bug,urgent`,
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listStatus, "status", "s", "", "filter by status (open, in-progress, review, done, closed)")
	listCmd.Flags().StringVarP(&listType, "type", "t", "", "filter by type (bug, feature, task, epic)")
	listCmd.Flags().StringVarP(&listPriority, "priority", "p", "", "filter by priority (low, medium, high, critical)")
	listCmd.Flags().StringVarP(&listAssignee, "assignee", "a", "", "filter by assignee")
	listCmd.Flags().StringSliceVarP(&listLabels, "labels", "l", []string{}, "filter by labels (comma separated)")
	listCmd.Flags().StringVarP(&listMilestone, "milestone", "m", "", "filter by milestone")
	listCmd.Flags().StringVarP(&listBranch, "branch", "b", "", "filter by branch")
	listCmd.Flags().IntVar(&listLimit, "limit", 20, "limit number of results")
	listCmd.Flags().BoolVar(&listAll, "all", false, "show all issues (no limit)")
}

func runList(cmd *cobra.Command, args []string) error {
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

	// Build filter
	filter := repositories.IssueFilter{}

	if listStatus != "" {
		status := entities.Status(listStatus)
		filter.Status = &status
	}
	if listType != "" {
		issueType := entities.IssueType(listType)
		filter.Type = &issueType
	}
	if listPriority != "" {
		priority := entities.Priority(listPriority)
		filter.Priority = &priority
	}
	if listAssignee != "" {
		filter.Assignee = &listAssignee
	}
	if listMilestone != "" {
		filter.Milestone = &listMilestone
	}
	if listBranch != "" {
		filter.Branch = &listBranch
	}
	if len(listLabels) > 0 {
		filter.Labels = listLabels
	}
	if !listAll {
		filter.Limit = &listLimit
	}

	// Get issues
	issueList, err := issueService.ListIssues(ctx, filter)
	if err != nil {
		printError(fmt.Errorf("failed to list issues: %w", err))
		return err
	}

	if len(issueList.Issues) == 0 {
		fmt.Println("No issues found.")
		return nil
	}

	// Display results
	if format == "table" {
		displayIssuesTable(issueList.Issues)
	} else {
		// TODO: Add JSON/YAML output formats
		fmt.Printf("Found %d issues (showing %d)\n", issueList.Total, issueList.Count)
		for _, issue := range issueList.Issues {
			fmt.Printf("%s: %s [%s] (%s)\n", issue.ID, issue.Title, issue.Status, issue.Type)
		}
	}

	if issueList.Total > issueList.Count {
		fmt.Printf("\nShowing %d of %d issues. Use --limit or --all to see more.\n", issueList.Count, issueList.Total)
	}

	return nil
}

func displayIssuesTable(issues []entities.Issue) {
	// Print header
	fmt.Printf("%-12s %-30s %-8s %-12s %-8s %-12s %-15s %-8s\n",
		"ID", "Title", "Type", "Status", "Priority", "Assignee", "Labels", "Updated")
	fmt.Printf("%-12s %-30s %-8s %-12s %-8s %-12s %-15s %-8s\n",
		"---", "-----", "----", "------", "--------", "--------", "------", "-------")

	for _, issue := range issues {
		assignee := ""
		if issue.Assignee != nil {
			assignee = issue.Assignee.Username
		}

		var labelNames []string
		for _, label := range issue.Labels {
			labelNames = append(labelNames, label.Name)
		}
		labels := strings.Join(labelNames, ", ")
		if len(labels) > 15 {
			labels = labels[:15] + "..."
		}

		title := issue.Title
		if len(title) > 30 {
			title = title[:30] + "..."
		}

		updated := issue.Timestamps.Updated.Format("Jan 02")

		fmt.Printf("%-12s %-30s %-8s %-12s %-8s %-12s %-15s %-8s\n",
			string(issue.ID),
			title,
			string(issue.Type),
			string(issue.Status),
			string(issue.Priority),
			assignee,
			labels,
			updated,
		)
	}
}
