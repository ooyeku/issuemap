package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

// assignCmd represents the assign command
var assignCmd = &cobra.Command{
	Use:   "assign <issue-id> [username]",
	Short: "Assign or unassign an issue",
	Long: `Assign an issue to a user or unassign it.

Examples:
  issuemap assign ISSUE-001 john        # Assign to john
  issuemap assign 001 john              # Assign to john (short format)
  issuemap assign ISSUE-001             # Unassign (interactive prompt)
  issuemap assign 001 --unassign        # Unassign the issue`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAssign(cmd, args)
	},
}

var unassignFlag bool

func init() {
	rootCmd.AddCommand(assignCmd)
	assignCmd.Flags().BoolVar(&unassignFlag, "unassign", false, "unassign the issue")
}

func runAssign(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	issueID := normalizeIssueID(args[0])

	var username string
	if len(args) > 1 {
		username = args[1]
	}

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

	// Determine action
	updates := make(map[string]interface{})

	if unassignFlag || username == "" && len(args) == 1 {
		// Unassign
		updates["assignee"] = ""
		_, err := issueService.UpdateIssue(ctx, issueID, updates)
		if err != nil {
			printError(fmt.Errorf("failed to unassign issue: %w", err))
			return err
		}
		printSuccess(fmt.Sprintf("Issue %s unassigned successfully", issueID))
	} else {
		// Assign
		updates["assignee"] = username
		_, err := issueService.UpdateIssue(ctx, issueID, updates)
		if err != nil {
			printError(fmt.Errorf("failed to assign issue: %w", err))
			return err
		}
		printSuccess(fmt.Sprintf("Issue %s assigned to %s successfully", issueID, username))
	}

	return nil
}
