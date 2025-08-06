package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	branchPrefix     string
	branchAutoSwitch bool
	branchTemplate   string
)

// branchCmd represents the branch command
var branchCmd = &cobra.Command{
	Use:   "branch <issue-id>",
	Short: "Create and switch to a Git branch for an issue",
	Long: `Create a Git branch for working on a specific issue. The branch name
will be automatically generated based on the issue ID and title.

The branch naming convention follows: <prefix>/<issue-id>-<sanitized-title>

Examples:
  issuemap branch 001                    # Creates: feature/ISSUE-001-fix-login-bug
  issuemap branch ISSUE-002              # Creates: feature/ISSUE-002-add-dark-mode
  issuemap branch 003 --prefix bugfix   # Creates: bugfix/ISSUE-003-update-docs
  issuemap branch 004 --no-switch       # Creates branch but doesn't switch to it`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBranch(cmd, args)
	},
}

// mergeCmd represents the merge command for auto-closing issues
var mergeCmd = &cobra.Command{
	Use:   "merge [issue-id]",
	Short: "Merge branch and auto-close related issue",
	Long: `Merge the current branch and automatically close the associated issue.
If no issue ID is provided, it will attempt to detect the issue from the current branch name.

Examples:
  issuemap merge                 # Auto-detect issue from branch name
  issuemap merge 001             # Explicitly specify issue to close
  issuemap merge ISSUE-002       # Close specific issue after merge`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMerge(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(branchCmd)
	rootCmd.AddCommand(mergeCmd)

	// Branch command flags
	branchCmd.Flags().StringVarP(&branchPrefix, "prefix", "p", "feature", "branch prefix (feature, bugfix, hotfix, etc.)")
	branchCmd.Flags().BoolVar(&branchAutoSwitch, "no-switch", false, "create branch but don't switch to it")
	branchCmd.Flags().StringVarP(&branchTemplate, "template", "t", "", "branch naming template")
}

func runBranch(cmd *cobra.Command, args []string) error {
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

	gitClient, err := git.NewGitClient(repoPath)
	if err != nil {
		printError(fmt.Errorf("failed to initialize git client: %w", err))
		return err
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitClient)

	// Get the issue to extract title for branch name
	issue, err := issueService.GetIssue(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("failed to get issue: %w", err))
		return err
	}

	// Get configuration
	config, err := configRepo.Load(ctx)
	if err != nil {
		// Use defaults if config can't be loaded
		config = entities.NewDefaultConfig()
	}

	// Determine branch prefix from configuration if not specified
	if branchPrefix == "feature" { // Check if it's the default value
		if customPrefix, exists := config.Git.BranchConfig.PrefixByType[string(issue.Type)]; exists {
			branchPrefix = customPrefix
		} else {
			branchPrefix = config.Git.DefaultBranchPrefix
		}
	}

	// Use configured template if not specified
	if branchTemplate == "" {
		branchTemplate = config.Git.BranchConfig.Template
	}

	// Generate branch name
	branchName := generateBranchName(branchPrefix, issueID, issue.Title, branchTemplate, &config.Git.BranchConfig)

	// Check if branch already exists
	if branchExists, err := checkBranchExists(gitClient, branchName); err != nil {
		printError(fmt.Errorf("failed to check if branch exists: %w", err))
		return err
	} else if branchExists {
		printWarning(fmt.Sprintf("Branch '%s' already exists", branchName))

		// Ask if user wants to switch to existing branch
		if !branchAutoSwitch {
			printInfo(fmt.Sprintf("Switching to existing branch: %s", branchName))
			return switchToBranch(gitClient, branchName)
		}
		return nil
	}

	// Create the branch
	err = gitClient.CreateBranch(ctx, branchName)
	if err != nil {
		printError(fmt.Errorf("failed to create branch: %w", err))
		return err
	}

	// Update the issue with the branch information
	issue.Branch = branchName
	_, err = issueService.UpdateIssue(ctx, issueID, map[string]interface{}{
		"branch": branchName,
	})
	if err != nil {
		printWarning(fmt.Sprintf("Created branch but couldn't update issue: %v", err))
	}

	// Switch to the branch only if auto-switch is enabled and working directory is clean
	shouldSwitch := !branchAutoSwitch && config.Git.BranchConfig.AutoSwitch
	if shouldSwitch {
		err = gitClient.SwitchToBranch(ctx, branchName)
		if err != nil {
			printWarning(fmt.Sprintf("Created branch '%s' but couldn't switch to it: %v", branchName, err))
			printInfo("You can switch manually with: git checkout " + branchName)
		} else {
			printSuccess(fmt.Sprintf("Created and switched to branch: %s", branchName))
		}
	} else {
		printSuccess(fmt.Sprintf("Created branch: %s", branchName))
		printInfo(fmt.Sprintf("Switch to it with: git checkout %s", branchName))
	}

	printInfo(fmt.Sprintf("Working on issue: %s - %s", issueID, issue.Title))

	// Add helpful next steps
	fmt.Println()
	printSectionHeader("Next steps:")
	fmt.Printf("  • Make your changes and commit them\n")
	fmt.Printf("  • Commits will be automatically linked to %s\n", issueID)
	fmt.Printf("  • When ready, run: issuemap merge\n")

	return nil
}

func runMerge(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize Git client
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	gitClient, err := git.NewGitClient(repoPath)
	if err != nil {
		printError(fmt.Errorf("failed to initialize git client: %w", err))
		return err
	}

	// Get current branch
	currentBranch, err := gitClient.GetCurrentBranch(ctx)
	if err != nil {
		printError(fmt.Errorf("failed to get current branch: %w", err))
		return err
	}

	// Determine issue ID
	var issueID entities.IssueID
	if len(args) > 0 {
		// Issue ID provided explicitly
		issueID = normalizeIssueID(args[0])
	} else {
		// Try to extract issue ID from branch name
		extractedID := extractIssueFromBranch(currentBranch)
		if extractedID == "" {
			printError(fmt.Errorf("could not detect issue ID from branch name '%s'. Please provide issue ID explicitly", currentBranch))
			return fmt.Errorf("no issue ID detected")
		}
		issueID = entities.IssueID(extractedID)
	}

	// Initialize issue services
	issuemapPath := filepath.Join(repoPath, ".issuemap")
	issueRepo := storage.NewFileIssueRepository(issuemapPath)
	configRepo := storage.NewFileConfigRepository(issuemapPath)
	issueService := services.NewIssueService(issueRepo, configRepo, gitClient)

	// Verify issue exists
	issue, err := issueService.GetIssue(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("failed to get issue %s: %w", issueID, err))
		return err
	}

	// Check if we're on main/master branch
	if currentBranch == "main" || currentBranch == "master" {
		printWarning("You're on the main branch. Consider switching to a feature branch first.")
		return fmt.Errorf("cannot merge from main branch")
	}

	// Get the main branch name
	mainBranch, err := gitClient.GetMainBranch(ctx)
	if err != nil {
		printError(fmt.Errorf("failed to determine main branch: %w", err))
		return err
	}

	printInfo(fmt.Sprintf("Merging branch '%s' into '%s' and closing issue %s", currentBranch, mainBranch, issueID))

	// Perform the actual Git merge
	err = gitClient.MergeBranch(ctx, currentBranch, mainBranch)
	if err != nil {
		printError(fmt.Errorf("failed to merge branch '%s' into '%s': %w", currentBranch, mainBranch, err))
		printWarning("Merge failed. You may need to resolve conflicts manually.")
		return err
	}

	printSuccess(fmt.Sprintf("Successfully merged branch '%s' into '%s'", currentBranch, mainBranch))

	// Close the issue after successful merge
	err = issueService.CloseIssue(ctx, issueID, fmt.Sprintf("Merged branch '%s' into %s", currentBranch, mainBranch))
	if err != nil {
		printWarning(fmt.Sprintf("Merge completed, but failed to close issue: %v", err))
	} else {
		printSuccess(fmt.Sprintf("Issue %s closed successfully", issueID))
	}

	printInfo(fmt.Sprintf("Issue: %s - %s", issueID, issue.Title))

	// Add helpful next steps
	fmt.Println()
	printSectionHeader("Merge completed:")
	fmt.Printf("  • Branch '%s' merged into '%s'\n", currentBranch, mainBranch)
	fmt.Printf("  • Issue %s has been closed\n", issueID)
	fmt.Printf("  • You are now on the '%s' branch\n", mainBranch)
	fmt.Printf("  • Delete the feature branch if no longer needed: git branch -d %s\n", currentBranch)

	return nil
}

// generateBranchName creates a branch name based on issue ID and title
func generateBranchName(prefix string, issueID entities.IssueID, title string, template string, config *entities.BranchConfig) string {
	if template != "" {
		// Use custom template
		return customBranchTemplate(template, prefix, string(issueID), title, config)
	}

	// Sanitize title for branch name
	sanitizedTitle := sanitizeForBranch(title)

	// Use configured max title length
	maxLength := config.MaxTitleLength
	if maxLength <= 0 {
		maxLength = 50 // Default fallback
	}

	// Limit title length to keep branch names reasonable
	if len(sanitizedTitle) > maxLength {
		sanitizedTitle = sanitizedTitle[:maxLength]
	}

	// Remove trailing hyphens
	sanitizedTitle = strings.TrimRight(sanitizedTitle, "-")

	// Remove trailing slash from prefix to avoid double slashes
	prefix = strings.TrimSuffix(prefix, "/")
	return fmt.Sprintf("%s/%s-%s", prefix, issueID, sanitizedTitle)
}

// sanitizeForBranch converts a title to a branch-safe string
func sanitizeForBranch(title string) string {
	// Convert to lowercase
	result := strings.ToLower(title)

	// Replace spaces and special characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	result = reg.ReplaceAllString(result, "-")

	// Remove leading/trailing hyphens
	result = strings.Trim(result, "-")

	return result
}

// extractIssueFromBranch extracts issue ID from branch name
func extractIssueFromBranch(branchName string) string {
	// Look for ISSUE-XXX pattern in branch name
	re := regexp.MustCompile(`ISSUE-\d+`)
	matches := re.FindStringSubmatch(branchName)
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// customBranchTemplate applies a custom template
func customBranchTemplate(template, prefix, issueID, title string, config *entities.BranchConfig) string {
	// Sanitize title according to config
	sanitizedTitle := sanitizeForBranch(title)

	// Apply max length
	maxLength := config.MaxTitleLength
	if maxLength <= 0 {
		maxLength = 50
	}
	if len(sanitizedTitle) > maxLength {
		sanitizedTitle = sanitizedTitle[:maxLength]
	}
	sanitizedTitle = strings.TrimRight(sanitizedTitle, "-")

	// Apply template replacements
	result := template
	// Remove trailing slash from prefix to avoid double slashes in templates like "{prefix}/{issue}-{title}"
	cleanPrefix := strings.TrimSuffix(prefix, "/")
	result = strings.ReplaceAll(result, "{prefix}", cleanPrefix)
	result = strings.ReplaceAll(result, "{issue}", issueID)
	result = strings.ReplaceAll(result, "{title}", sanitizedTitle)

	return result
}

// checkBranchExists checks if a branch already exists
func checkBranchExists(gitClient *git.GitClient, branchName string) (bool, error) {
	return gitClient.BranchExists(context.Background(), branchName)
}

// switchToBranch switches to an existing branch
func switchToBranch(gitClient *git.GitClient, branchName string) error {
	err := gitClient.SwitchToBranch(context.Background(), branchName)
	if err != nil {
		return err
	}
	printInfo(fmt.Sprintf("Switched to branch: %s", branchName))
	return nil
}
