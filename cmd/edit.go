package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	editTitle       string
	editDescription string
	editType        string
	editStatus      string
	editPriority    string
	editAssignee    string
	editBranch      string
	editLabels      []string
	editMilestone   string
)

// editCmd represents the edit command
var editCmd = &cobra.Command{
	Use:   "edit <issue-id>",
	Short: "Edit an existing issue",
	Long: `Edit properties of an existing issue such as title, description, type, status, 
priority, assignee, labels, milestone, and branch.

Examples:
  issuemap edit ISSUE-001 --type bug --priority high
  issuemap edit 001 --type bug --priority high
  issuemap edit ISSUE-001 --status in-progress --assignee john
  issuemap edit 001 --title "New title" --description "Updated description"
  issuemap edit ISSUE-001 --labels bug,urgent --milestone v1.0.0`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEdit(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(editCmd)

	editCmd.Flags().StringVarP(&editTitle, "title", "t", "", "update issue title")
	editCmd.Flags().StringVarP(&editDescription, "description", "d", "", "update issue description")
	editCmd.Flags().StringVar(&editType, "type", "", "update issue type (bug, feature, task, epic)")
	editCmd.Flags().StringVarP(&editStatus, "status", "s", "", "update issue status (open, in-progress, review, done, closed)")
	editCmd.Flags().StringVarP(&editPriority, "priority", "p", "", "update issue priority (low, medium, high, critical)")
	editCmd.Flags().StringVarP(&editAssignee, "assignee", "a", "", "update assignee (use 'none' to unassign)")
	editCmd.Flags().StringVarP(&editBranch, "branch", "b", "", "update associated branch")
	editCmd.Flags().StringSliceVarP(&editLabels, "labels", "l", []string{}, "update labels (comma separated, replaces existing)")
	editCmd.Flags().StringVarP(&editMilestone, "milestone", "m", "", "update milestone (use 'none' to remove)")
}

func runEdit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	issueID := normalizeIssueID(args[0])

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

	// Build updates map
	updates := make(map[string]interface{})

	if editTitle != "" {
		updates["title"] = editTitle
	}
	if editDescription != "" {
		updates["description"] = editDescription
	}
	if editType != "" {
		updates["type"] = editType
	}
	if editStatus != "" {
		updates["status"] = editStatus
	}
	if editPriority != "" {
		updates["priority"] = editPriority
	}
	if editAssignee != "" {
		if editAssignee == "none" {
			updates["assignee"] = ""
		} else {
			updates["assignee"] = editAssignee
		}
	}
	if editBranch != "" {
		updates["branch"] = editBranch
	}
	if editMilestone != "" {
		if editMilestone == "none" {
			updates["milestone"] = ""
		} else {
			updates["milestone"] = editMilestone
		}
	}

	// Handle labels separately since they need special processing
	if len(editLabels) > 0 {
		updates["labels"] = editLabels
	}

	if len(updates) == 0 {
		printError(fmt.Errorf("no changes specified. Use --help to see available options"))
		return fmt.Errorf("no changes specified")
	}

	// Update the issue
	issue, err := issueService.UpdateIssue(ctx, issueID, updates)
	if err != nil {
		printError(fmt.Errorf("failed to update issue: %w", err))
		return err
	}

	// Show success message and updated issue
	printSuccess(fmt.Sprintf("Issue %s updated successfully", issue.ID))

	// Show what was changed
	fmt.Printf("\nUpdated fields:\n")
	for field, value := range updates {
		switch field {
		case "assignee":
			if value == "" {
				fmt.Printf("  %s: Unassigned\n", field)
			} else {
				fmt.Printf("  %s: %v\n", field, value)
			}
		case "milestone":
			if value == "" {
				fmt.Printf("  %s: None\n", field)
			} else {
				fmt.Printf("  %s: %v\n", field, value)
			}
		case "labels":
			if labels, ok := value.([]string); ok && len(labels) > 0 {
				fmt.Printf("  %s: %s\n", field, strings.Join(labels, ", "))
			} else {
				fmt.Printf("  %s: None\n", field)
			}
		default:
			fmt.Printf("  %s: %v\n", field, value)
		}
	}

	return nil
}
