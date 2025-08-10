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
	Use:   "delete <issue-id> [more-issue-ids]",
	Short: "Permanently delete one or more issues",
	Long: `Permanently delete one or more issues and all associated data including history.
This action cannot be undone. Use with caution.

Examples:
  issuemap delete ISSUE-001
  issuemap delete 001 002 003
  issuemap delete ISSUE-001 ISSUE-002 --force      # Skip confirmation
  issuemap delete 001 002 --yes                    # Skip confirmation`,
	Args: cobra.MinimumNArgs(1),
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
	// Normalize all provided IDs
	ids := make([]entities.IssueID, 0, len(args))
	split := func(s string) []string {
		// Split on commas and any whitespace
		f := func(r rune) bool { return r == ',' || r == '\n' || r == '\t' || r == ' ' || r == '\r' }
		return strings.FieldsFunc(s, f)
	}
	for _, raw := range args {
		for _, part := range split(raw) {
			p := strings.TrimSpace(part)
			if p == "" {
				continue
			}
			ids = append(ids, normalizeIssueID(p))
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("no issue ids provided")
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

	// For single delete, keep existing detailed summary; for multiple, show compact list
	if len(ids) == 1 {
		issueID := ids[0]
		issue, err := issueService.GetIssue(ctx, issueID)
		if err != nil {
			printError(fmt.Errorf("failed to find issue %s: %w", issueID, err))
			return err
		}
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
		if err := issueService.DeleteIssue(ctx, issueID); err != nil {
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

	// Multi-delete path
	valid := make([]entities.IssueID, 0, len(ids))
	for _, id := range ids {
		if _, err := issueService.GetIssue(ctx, id); err == nil {
			valid = append(valid, id)
		} else {
			printError(fmt.Errorf("skipping %s: %v", id, err))
		}
	}
	if len(valid) == 0 {
		return fmt.Errorf("no valid issues to delete")
	}
	if noColor {
		fmt.Printf("Issues to be deleted (%d): %s\n", len(valid), strings.Join(formatIDs(valid), ", "))
	} else {
		fmt.Printf("%s %d: %s\n", colorHeader("Issues to be deleted"), len(valid), strings.Join(formatIDs(valid), ", "))
	}
	if !deleteForce && !deleteYes {
		if !askForConfirmationMultiple(valid) {
			if noColor {
				fmt.Println("Deletion cancelled.")
			} else {
				colorValue("Deletion cancelled.")
			}
			return nil
		}
	}
	var firstErr error
	for _, id := range valid {
		if err := issueService.DeleteIssue(ctx, id); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			printError(fmt.Errorf("failed to delete %s: %v", id, err))
			continue
		}
		printSuccess(fmt.Sprintf("Deleted %s", id))
	}
	if noColor {
		fmt.Println("This action cannot be undone.")
	} else {
		color.HiRed("This action cannot be undone.")
	}
	return firstErr
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

// askForConfirmationMultiple prompts once for multiple deletions
func askForConfirmationMultiple(ids []entities.IssueID) bool {
	if noColor {
		fmt.Printf("\nWARNING: This will permanently delete %d issues and all their history.\n", len(ids))
		fmt.Print("This action cannot be undone. Are you sure? (yes/no): ")
	} else {
		color.HiRed("\nWARNING: This will permanently delete %d issues and all their history.", len(ids))
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

func formatIDs(ids []entities.IssueID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, string(id))
	}
	return out
}
