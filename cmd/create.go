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
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	createTitle       string
	createDescription string
	createType        string
	createPriority    string
	createAssignee    string
	createLabels      []string
	createMilestone   string
	createTemplate    string
	createInteractive bool
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create [title]",
	Short: "Create a new issue",
	Long: `Create a new issue in the current project. You can provide the title as an argument
or use flags to specify details.

Examples:
  issuemap create "Fix login bug"
  issuemap create --type bug --priority high "User authentication fails"
  issuemap create --template bug`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 && createTitle == "" {
			createTitle = strings.Join(args, " ")
		}
		return runCreate(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVarP(&createTitle, "title", "t", "", "issue title")
	createCmd.Flags().StringVarP(&createDescription, "description", "d", "", "issue description")
	createCmd.Flags().StringVar(&createType, "type", "", "issue type (bug, feature, task, epic)")
	createCmd.Flags().StringVarP(&createPriority, "priority", "p", "", "issue priority (low, medium, high, critical)")
	createCmd.Flags().StringVarP(&createAssignee, "assignee", "a", "", "assignee username")
	createCmd.Flags().StringSliceVarP(&createLabels, "labels", "l", []string{}, "labels (comma separated)")
	createCmd.Flags().StringVarP(&createMilestone, "milestone", "m", "", "milestone name")
	createCmd.Flags().StringVar(&createTemplate, "template", "", "template to use (bug, feature, task, epic)")
	createCmd.Flags().BoolVarP(&createInteractive, "interactive", "i", false, "interactive mode")
}

func runCreate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate required fields
	if createTitle == "" {
		printError(fmt.Errorf("issue title is required"))
		fmt.Println("Usage: issuemap create \"Issue title\" [flags]")
		return fmt.Errorf("title required")
	}

	// Set defaults only when no template is provided (template should provide defaults)
	if createTemplate == "" {
		if createType == "" {
			createType = "task"
		}
		if createPriority == "" {
			createPriority = "medium"
		}
	}

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

	// Build create request
	req := services.CreateIssueRequest{
		Title:       createTitle,
		Description: createDescription,
		Type:        entities.IssueType(createType),
		Priority:    entities.Priority(createPriority),
		Labels:      createLabels,
	}

	if createAssignee != "" {
		req.Assignee = &createAssignee
	}
	if createMilestone != "" {
		req.Milestone = &createMilestone
	}
	if createTemplate != "" {
		req.Template = &createTemplate
	}

	// Create the issue
	issue, err := issueService.CreateIssue(ctx, req)
	if err != nil {
		printError(fmt.Errorf("failed to create issue: %w", err))
		return err
	}

	// Display success message
	printSuccess(fmt.Sprintf(app.MsgIssueCreated, issue.ID))

	// Display issue details
	fmt.Printf("\nTitle: %s\n", issue.Title)
	fmt.Printf("Type: %s\n", issue.Type)
	fmt.Printf("Status: %s\n", issue.Status)
	fmt.Printf("Priority: %s\n", issue.Priority)
	if issue.Assignee != nil {
		fmt.Printf("Assignee: %s\n", issue.Assignee.Username)
	}
	if len(issue.Labels) > 0 {
		var labelNames []string
		for _, label := range issue.Labels {
			labelNames = append(labelNames, label.Name)
		}
		fmt.Printf("Labels: %s\n", strings.Join(labelNames, ", "))
	}
	if issue.Branch != "" {
		fmt.Printf("Branch: %s\n", issue.Branch)
	}
	fmt.Printf("Created: %s\n", issue.Timestamps.Created.Format("2006-01-02 15:04:05"))

	return nil
}
