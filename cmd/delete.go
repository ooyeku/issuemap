package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	deleteForce bool
	deleteYes   bool
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete <issue-id>",
	Short: "Permanently delete an issue",
	Long: `Permanently delete an issue and all associated data including history.
This action cannot be undone. Use with caution.

Examples:
  issuemap delete ISSUE-001
  issuemap delete 001
  issuemap delete ISSUE-001 --force      # Skip confirmation
  issuemap delete 001 --yes              # Skip confirmation`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDelete(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVar(&deleteForce, "force", false, "skip confirmation prompt")
	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "skip confirmation prompt")
}

func runDelete(cmd *cobra.Command, args []string) error {
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

	// Check if issue exists first
	issue, err := issueService.GetIssue(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("failed to find issue %s: %w", issueID, err))
		return err
	}

	// Show issue details before deletion
	if noColor {
		fmt.Printf("Issue to be deleted: %s\n", issueID)
		fmt.Printf("Title: %s\n", issue.Title)
		fmt.Printf("Status: %s\n", issue.Status)
		fmt.Printf("Type: %s\n", issue.Type)
	} else {
		fmt.Printf("%s %s\n", colorHeader("Issue to be deleted:"), colorIssueID(issueID))
		fmt.Printf("%s %s\n", colorLabel("Title:"), colorValue(issue.Title))
		fmt.Printf("%s %s\n", colorLabel("Status:"), colorStatus(issue.Status))
		fmt.Printf("%s %s\n", colorLabel("Type:"), colorType(issue.Type))
	}

	// Skip confirmation if force or yes flag is set
	if !deleteForce && !deleteYes {
		if !askForConfirmation(issueID) {
			if noColor {
				fmt.Println("Deletion cancelled.")
			} else {
				colorValue("Deletion cancelled.")
			}
			return nil
		}
	}

	// Delete the issue
	err = issueService.DeleteIssue(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("failed to delete issue %s: %w", issueID, err))
		return err
	}

	printSuccess(fmt.Sprintf("Issue %s has been permanently deleted", issueID))

	if noColor {
		fmt.Println("This action cannot be undone.")
	} else {
		color.HiRed("This action cannot be undone.")
	}

	return nil
}

// askForConfirmation prompts the user for confirmation before deleting
func askForConfirmation(issueID entities.IssueID) bool {
	if noColor {
		fmt.Printf("\nWARNING: This will permanently delete issue %s and all its history.\n", issueID)
		fmt.Print("This action cannot be undone. Are you sure? (yes/no): ")
	} else {
		color.HiRed("\nWARNING: This will permanently delete issue %s and all its history.", issueID)
		color.White("This action cannot be undone. Are you sure? (yes/no): ")
	}

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "yes" || response == "y"
}
