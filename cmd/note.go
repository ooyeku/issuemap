package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

// noteCmd represents the note command
var noteCmd = &cobra.Command{
	Use:   "note <issue-id> <note-text>",
	Short: "Add a quick note to an issue",
	Long: `Add a one-line note to an issue without opening the editor.

This is useful for quickly capturing thoughts, progress updates, or observations
about an issue while working on it.

Examples:
  issuemap note ISSUE-001 Implemented basic authentication
  issuemap note ISSUE-002 Found the root cause of the bug
  issuemap note 123 "Meeting with team scheduled for tomorrow"
  ismp note ISSUE-001 I did xyz`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID := normalizeIssueID(args[0])
		noteText := strings.Join(args[1:], " ")
		return runNote(cmd, issueID, noteText)
	},
}

func init() {
	rootCmd.AddCommand(noteCmd)
}

func runNote(cmd *cobra.Command, issueID entities.IssueID, noteText string) error {
	ctx := context.Background()

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	basePath := filepath.Join(repoPath, ".issuemap")
	issueRepo := storage.NewFileIssueRepository(basePath)
	configRepo := storage.NewFileConfigRepository(basePath)
	gitClient, err := git.NewGitClient(repoPath)
	if err != nil {
		printError(fmt.Errorf("failed to initialize git client: %w", err))
		return err
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitClient)
	historyRepo := storage.NewFileHistoryRepository(basePath)
	historyService := services.NewHistoryService(historyRepo, gitClient)

	// Check if issue exists
	issue, err := issueService.GetIssue(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("issue %s not found", issueID))
		return err
	}

	// Get current user
	author := getCurrentUser(gitClient)

	// Add the note as a comment to the issue
	issue.AddComment(author, noteText)

	// Update the issue with the new comment
	updates := map[string]interface{}{
		"comments":           issue.Comments,
		"timestamps.updated": time.Now(),
	}

	_, err = issueService.UpdateIssue(ctx, issueID, updates)
	if err != nil {
		printError(fmt.Errorf("failed to add note to issue: %w", err))
		return err
	}

	// Record history entry for the note
	err = historyService.RecordIssueCommented(ctx, issueID, noteText, author)
	if err != nil {
		// Don't fail the operation if history fails, just warn
		printWarning(fmt.Sprintf("warning: failed to record history: %v", err))
	}

	// Display success message
	printSuccess(fmt.Sprintf("Note added to %s", issueID))
	fmt.Printf("\nIssue: %s - %s\n", issue.ID, issue.Title)
	fmt.Printf("Author: %s\n", author)
	fmt.Printf("Note: %s\n", noteText)
	fmt.Printf("Added at: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	return nil
}
