package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var closeReason string

// closeCmd represents the close command
var closeCmd = &cobra.Command{
	Use:   "close <issue-id>",
	Short: "Close an issue",
	Long: `Close an issue with an optional reason.

Examples:
  issuemap close ISSUE-001
  issuemap close ISSUE-001 --reason "Fixed in commit abc123"
  issuemap close ISSUE-001 --reason "Won't fix - duplicate of ISSUE-002"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runClose(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(closeCmd)
	closeCmd.Flags().StringVarP(&closeReason, "reason", "r", "", "reason for closing the issue")
}

func runClose(cmd *cobra.Command, args []string) error {
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

	// Close the issue
	err = issueService.CloseIssue(ctx, issueID, closeReason)
	if err != nil {
		printError(fmt.Errorf("failed to close issue: %w", err))
		return err
	}

	if closeReason != "" {
		printSuccess(fmt.Sprintf("Issue %s closed successfully with reason: %s", issueID, closeReason))
	} else {
		printSuccess(fmt.Sprintf("Issue %s closed successfully", issueID))
	}

	return nil
}

// reopenCmd represents the reopen command
var reopenCmd = &cobra.Command{
	Use:   "reopen <issue-id>",
	Short: "Reopen a closed issue",
	Long: `Reopen a previously closed issue.

Examples:
  issuemap reopen ISSUE-001`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReopen(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(reopenCmd)
}

func runReopen(cmd *cobra.Command, args []string) error {
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

	// Reopen the issue
	err = issueService.ReopenIssue(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("failed to reopen issue: %w", err))
		return err
	}

	printSuccess(fmt.Sprintf("Issue %s reopened successfully", issueID))
	return nil
}
