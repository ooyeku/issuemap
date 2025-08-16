package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
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
  issuemap show 001
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
	// Header with issue ID and title
	if noColor {
		fmt.Printf("Issue %s\n", issue.ID)
		fmt.Printf("Title: %s\n", issue.Title)
	} else {
		fmt.Printf("%s %s\n", colorHeader("Issue"), colorIssueID(issue.ID))
		fmt.Printf("%s %s\n", colorLabel("Title:"), colorValue(issue.Title))
	}

	printSeparator()

	// Core properties
	printSectionHeader("Properties")
	if noColor {
		fmt.Printf("Type: %s\n", issue.Type)
		fmt.Printf("Status: %s\n", issue.Status)
		fmt.Printf("Priority: %s\n", issue.Priority)
	} else {
		fmt.Printf("%s %s\n", colorLabel("Type:"), colorType(issue.Type))
		fmt.Printf("%s %s\n", colorLabel("Status:"), colorStatus(issue.Status))
		fmt.Printf("%s %s\n", colorLabel("Priority:"), colorPriority(issue.Priority))
	}

	// Assignment info
	if issue.Assignee != nil {
		assigneeText := fmt.Sprintf("%s (%s)", issue.Assignee.Username, issue.Assignee.Email)
		formatFieldValue("Assignee", assigneeText)
	} else {
		formatFieldValue("Assignee", "Unassigned")
	}

	// Labels with colors
	if len(issue.Labels) > 0 {
		var labelNames []string
		for _, label := range issue.Labels {
			if noColor {
				labelNames = append(labelNames, label.Name)
			} else {
				labelNames = append(labelNames, color.HiYellowString(label.Name))
			}
		}
		if noColor {
			fmt.Printf("Labels: %s\n", strings.Join(labelNames, ", "))
		} else {
			fmt.Printf("%s %s\n", colorLabel("Labels:"), strings.Join(labelNames, " "))
		}
	}

	// Milestone
	if issue.Milestone != nil {
		milestoneText := issue.Milestone.Name
		if issue.Milestone.DueDate != nil {
			milestoneText += fmt.Sprintf(" (due %s)", issue.Milestone.DueDate.Format("2006-01-02"))
		}
		formatFieldValue("Milestone", milestoneText)
	}

	// Branch
	if issue.Branch != "" {
		formatFieldValue("Branch", issue.Branch)
	}

	// Timestamps
	printSectionHeader("Timeline")
	formatFieldValue("Created", issue.Timestamps.Created.Format("2006-01-02 15:04:05"))
	formatFieldValue("Updated", issue.Timestamps.Updated.Format("2006-01-02 15:04:05"))

	if issue.Timestamps.Closed != nil {
		formatFieldValue("Closed", issue.Timestamps.Closed.Format("2006-01-02 15:04:05"))
	}

	// Description
	if issue.Description != "" {
		printSectionHeader("Description")
		if noColor {
			fmt.Printf("%s\n", issue.Description)
		} else {
			color.White("%s", issue.Description)
		}
	}

	// Commits
	if len(issue.Commits) > 0 {
		printSectionHeader(fmt.Sprintf("Commits (%d)", len(issue.Commits)))
		for _, commit := range issue.Commits {
			commitMsg := commit.Message
			if len(commitMsg) > 50 {
				commitMsg = commitMsg[:50] + "..."
			}

			if noColor {
				fmt.Printf("  %s - %s (%s)\n",
					commit.Hash[:8], commitMsg, commit.Author)
			} else {
				fmt.Printf("  %s - %s %s\n",
					color.YellowString(commit.Hash[:8]),
					colorValue(commitMsg),
					color.HiBlackString("(%s)", commit.Author))
			}
		}
	}

	// Attachments
	if len(issue.Attachments) > 0 {
		printSectionHeader(fmt.Sprintf("Attachments (%d)", len(issue.Attachments)))
		for _, attachment := range issue.Attachments {
			icon := "[ATT]"
			switch attachment.Type {
			case entities.AttachmentTypeImage:
				icon = "[IMG]"
			case entities.AttachmentTypeDocument:
				icon = "[DOC]"
			case entities.AttachmentTypeText:
				icon = "[TXT]"
			}

			if noColor {
				fmt.Printf("  %s %s - %s (%s, uploaded by %s)\n",
					icon, attachment.Filename, attachment.GetSizeFormatted(),
					attachment.Type, attachment.UploadedBy)
				if attachment.Description != "" {
					fmt.Printf("     Description: %s\n", attachment.Description)
				}
			} else {
				fmt.Printf("  %s %s - %s %s\n",
					icon, colorValue(attachment.Filename),
					color.HiBlackString(attachment.GetSizeFormatted()),
					color.HiBlackString("(%s, uploaded by %s)", attachment.Type, attachment.UploadedBy))
				if attachment.Description != "" {
					fmt.Printf("     %s %s\n", colorLabel("Description:"), attachment.Description)
				}
			}
		}
	}

	// Comments
	if len(issue.Comments) > 0 {
		printSectionHeader(fmt.Sprintf("Comments (%d)", len(issue.Comments)))
		for i, comment := range issue.Comments {
			if i > 0 {
				fmt.Println()
			}

			if noColor {
				fmt.Printf("── Comment #%d by %s on %s ──\n",
					comment.ID, comment.Author, comment.Date.Format("2006-01-02 15:04:05"))
				fmt.Printf("%s\n", comment.Text)
			} else {
				color.HiBlack("── Comment #%d by %s on %s ──",
					comment.ID, comment.Author, comment.Date.Format("2006-01-02 15:04:05"))
				color.White("%s", comment.Text)
			}
		}
	}

	// Metadata
	if issue.Metadata.EstimatedHours != nil {
		printSectionHeader("Metadata")
		formatFieldValue("Estimated Hours", fmt.Sprintf("%.1f", *issue.Metadata.EstimatedHours))
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
