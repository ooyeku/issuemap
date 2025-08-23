package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	importDryRun     bool
	importOverwrite  bool
	importValidation bool
	importPrefix     string
)

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import issues from YAML file",
	Long: `Import issues from a YAML file with strong validation.

The YAML format should contain a list of issues with the following structure:
  - title: "Issue Title"
    description: "Issue description"
    type: "task"  # bug, feature, task, epic
    status: "open"  # open, in-progress, review, done, closed
    priority: "medium"  # low, medium, high, critical
    assignee: "username"  # optional
    branch: "feature-branch"  # optional
    labels:  # optional
      - "bug"
      - "urgent" 
    milestone: "v1.0"  # optional
    estimated_hours: 5.0  # optional
    custom_fields:  # optional
      component: "frontend"
      complexity: "high"

Examples:
  issuemap import issues.yaml
  issuemap import --dry-run issues.yaml
  issuemap import --prefix PROJ issues.yaml
  ismp import --overwrite issues.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runImport(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "validate and show what would be imported without creating issues")
	importCmd.Flags().BoolVar(&importOverwrite, "overwrite", false, "overwrite existing issues with same ID")
	importCmd.Flags().BoolVar(&importValidation, "validate", true, "enable strict validation (default: true)")
	importCmd.Flags().StringVar(&importPrefix, "prefix", "", "prefix to add to issue IDs (e.g., PROJ)")
}

type ImportableIssue struct {
	ID             string              `yaml:"id,omitempty"`
	Title          string              `yaml:"title"`
	Description    string              `yaml:"description"`
	Type           string              `yaml:"type"`
	Status         string              `yaml:"status,omitempty"`
	Priority       string              `yaml:"priority,omitempty"`
	Assignee       string              `yaml:"assignee,omitempty"`
	Branch         string              `yaml:"branch,omitempty"`
	Labels         []string            `yaml:"labels,omitempty"`
	Milestone      string              `yaml:"milestone,omitempty"`
	EstimatedHours float64             `yaml:"estimated_hours,omitempty"`
	CustomFields   map[string]string   `yaml:"custom_fields,omitempty"`
	Comments       []ImportableComment `yaml:"comments,omitempty"`
}

type ImportableComment struct {
	Author string `yaml:"author"`
	Text   string `yaml:"text"`
}

type ImportResult struct {
	Created int
	Updated int
	Skipped int
	Errors  []ImportError
}

type ImportError struct {
	Issue string
	Error string
}

func runImport(cmd *cobra.Command, filename string) error {
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

	// Read and parse YAML file
	importableIssues, err := readImportFile(filename)
	if err != nil {
		printError(fmt.Errorf("failed to read import file: %w", err))
		return err
	}

	if len(importableIssues) == 0 {
		printInfo("No issues found in import file.")
		return nil
	}

	// Validate issues
	if importValidation {
		validationErrors := validateImportableIssues(importableIssues)
		if len(validationErrors) > 0 {
			printError(fmt.Errorf("validation failed"))
			for _, err := range validationErrors {
				fmt.Printf("  • %s: %s\n", err.Issue, err.Error)
			}
			return fmt.Errorf("validation failed with %d errors", len(validationErrors))
		}
	}

	if importDryRun {
		fmt.Printf("Dry run: Would import %d issues\n", len(importableIssues))
		for i, issue := range importableIssues {
			fmt.Printf("  %d. %s - %s (%s)\n", i+1, generateIssueID(issue, i+1), issue.Title, issue.Type)
		}
		return nil
	}

	// Import issues
	result := importIssues(ctx, issueService, importableIssues)

	// Display results
	displayImportResult(result)

	if len(result.Errors) > 0 {
		return fmt.Errorf("import completed with %d errors", len(result.Errors))
	}

	return nil
}

func readImportFile(filename string) ([]ImportableIssue, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var issues []ImportableIssue
	err = yaml.Unmarshal(content, &issues)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return issues, nil
}

func validateImportableIssues(issues []ImportableIssue) []ImportError {
	var errors []ImportError

	for i, issue := range issues {
		issueRef := fmt.Sprintf("Issue %d", i+1)
		if issue.Title != "" {
			issueRef = fmt.Sprintf("Issue %d (%s)", i+1, issue.Title)
		}

		// Required fields
		if strings.TrimSpace(issue.Title) == "" {
			errors = append(errors, ImportError{issueRef, "title is required"})
		}

		if strings.TrimSpace(issue.Type) == "" {
			errors = append(errors, ImportError{issueRef, "type is required"})
		}

		// Validate enum values
		if issue.Type != "" {
			validTypes := []string{"bug", "feature", "task", "epic"}
			if !contains(validTypes, issue.Type) {
				errors = append(errors, ImportError{issueRef, fmt.Sprintf("invalid type: %s (must be one of: %s)", issue.Type, strings.Join(validTypes, ", "))})
			}
		}

		if issue.Status != "" {
			validStatuses := []string{"open", "in-progress", "review", "done", "closed"}
			if !contains(validStatuses, issue.Status) {
				errors = append(errors, ImportError{issueRef, fmt.Sprintf("invalid status: %s (must be one of: %s)", issue.Status, strings.Join(validStatuses, ", "))})
			}
		}

		if issue.Priority != "" {
			validPriorities := []string{"low", "medium", "high", "critical"}
			if !contains(validPriorities, issue.Priority) {
				errors = append(errors, ImportError{issueRef, fmt.Sprintf("invalid priority: %s (must be one of: %s)", issue.Priority, strings.Join(validPriorities, ", "))})
			}
		}

		// Validate estimated hours
		if issue.EstimatedHours < 0 {
			errors = append(errors, ImportError{issueRef, "estimated_hours cannot be negative"})
		}

		// Validate title length
		if len(issue.Title) > 200 {
			errors = append(errors, ImportError{issueRef, "title too long (maximum 200 characters)"})
		}
	}

	return errors
}

func importIssues(ctx context.Context, issueService *services.IssueService, importableIssues []ImportableIssue) ImportResult {
	var result ImportResult

	for i, importableIssue := range importableIssues {
		issueID := entities.IssueID(generateIssueID(importableIssue, i+1))

		// Check if issue already exists
		existingIssue, err := issueService.GetIssue(ctx, issueID)
		if err == nil && existingIssue != nil && !importOverwrite {
			result.Skipped++
			result.Errors = append(result.Errors, ImportError{
				Issue: string(issueID),
				Error: "issue already exists (use --overwrite to replace)",
			})
			continue
		}

		// Convert to entities.Issue
		issue, err := convertToIssue(importableIssue, issueID)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{
				Issue: string(issueID),
				Error: fmt.Sprintf("conversion failed: %v", err),
			})
			continue
		}

		// Create or update issue
		if existingIssue != nil && importOverwrite {
			// Update existing issue
			updates := buildUpdateMap(issue)
			_, err = issueService.UpdateIssue(ctx, issueID, updates)
			if err != nil {
				result.Errors = append(result.Errors, ImportError{
					Issue: string(issueID),
					Error: fmt.Sprintf("update failed: %v", err),
				})
				continue
			}
			result.Updated++
		} else {
			// Create new issue using CreateIssueRequest
			req := convertToCreateRequest(importableIssue)
			_, err = issueService.CreateIssue(ctx, req)
			if err != nil {
				result.Errors = append(result.Errors, ImportError{
					Issue: string(issueID),
					Error: fmt.Sprintf("creation failed: %v", err),
				})
				continue
			}
			result.Created++
		}
	}

	return result
}

func generateIssueID(issue ImportableIssue, index int) string {
	if issue.ID != "" {
		if importPrefix != "" {
			return fmt.Sprintf("%s-%s", importPrefix, issue.ID)
		}
		return issue.ID
	}

	// Generate ID from title or index
	baseID := "IMPORTED"
	if importPrefix != "" {
		baseID = importPrefix
	}

	return fmt.Sprintf("%s-%03d", baseID, index)
}

func convertToIssue(importable ImportableIssue, issueID entities.IssueID) (*entities.Issue, error) {
	// Set defaults
	issueType := entities.IssueTypeTask
	if importable.Type != "" {
		issueType = entities.IssueType(importable.Type)
	}

	status := entities.StatusOpen
	if importable.Status != "" {
		status = entities.Status(importable.Status)
	}

	priority := entities.PriorityMedium
	if importable.Priority != "" {
		priority = entities.Priority(importable.Priority)
	}

	// Create new issue
	issue := entities.NewIssue(issueID, importable.Title, importable.Description, issueType)
	issue.Status = status
	issue.Priority = priority

	// Set optional fields
	if importable.Assignee != "" {
		issue.Assignee = &entities.User{Username: importable.Assignee}
	}

	if importable.Branch != "" {
		issue.Branch = importable.Branch
	}

	if importable.Milestone != "" {
		issue.Milestone = &entities.Milestone{Name: importable.Milestone}
	}

	// Set labels
	for _, labelName := range importable.Labels {
		label := entities.Label{Name: labelName, Color: "#gray"}
		issue.Labels = append(issue.Labels, label)
	}

	// Set estimated hours
	if importable.EstimatedHours > 0 {
		issue.Metadata.EstimatedHours = &importable.EstimatedHours
	}

	// Set custom fields
	if len(importable.CustomFields) > 0 {
		if issue.Metadata.CustomFields == nil {
			issue.Metadata.CustomFields = make(map[string]string)
		}
		for k, v := range importable.CustomFields {
			issue.Metadata.CustomFields[k] = v
		}
	}

	// Add comments
	for _, importableComment := range importable.Comments {
		issue.AddComment(importableComment.Author, importableComment.Text)
	}

	return issue, nil
}

func buildUpdateMap(issue *entities.Issue) map[string]interface{} {
	updates := map[string]interface{}{
		"title":       issue.Title,
		"description": issue.Description,
		"type":        string(issue.Type),
		"status":      string(issue.Status),
		"priority":    string(issue.Priority),
		"branch":      issue.Branch,
		"comments":    issue.Comments,
	}

	if issue.Assignee != nil {
		updates["assignee"] = issue.Assignee.Username
	}

	if issue.Milestone != nil {
		updates["milestone"] = issue.Milestone.Name
	}

	// Labels
	var labelNames []string
	for _, label := range issue.Labels {
		labelNames = append(labelNames, label.Name)
	}
	updates["labels"] = labelNames

	// Estimated hours
	if issue.Metadata.EstimatedHours != nil {
		updates["estimated_hours"] = *issue.Metadata.EstimatedHours
	}

	return updates
}

func displayImportResult(result ImportResult) {
	fmt.Printf("Import completed:\n")
	fmt.Printf("  Created: %d issues\n", result.Created)
	fmt.Printf("  Updated: %d issues\n", result.Updated)
	fmt.Printf("  Skipped: %d issues\n", result.Skipped)
	fmt.Printf("  Errors:  %d issues\n", len(result.Errors))

	if len(result.Errors) > 0 {
		fmt.Printf("\nErrors:\n")
		for _, err := range result.Errors {
			fmt.Printf("  • %s: %s\n", err.Issue, err.Error)
		}
	}

	if result.Created > 0 || result.Updated > 0 {
		printSuccess(fmt.Sprintf("Successfully imported %d issues", result.Created+result.Updated))
	}
}

func convertToCreateRequest(importable ImportableIssue) services.CreateIssueRequest {
	// Set defaults
	issueType := entities.IssueTypeTask
	if importable.Type != "" {
		issueType = entities.IssueType(importable.Type)
	}

	priority := entities.PriorityMedium
	if importable.Priority != "" {
		priority = entities.Priority(importable.Priority)
	}

	req := services.CreateIssueRequest{
		Title:       importable.Title,
		Description: importable.Description,
		Type:        issueType,
		Priority:    priority,
		Labels:      importable.Labels,
	}

	// Set optional fields
	if importable.Assignee != "" {
		req.Assignee = &importable.Assignee
	}

	if importable.Milestone != "" {
		req.Milestone = &importable.Milestone
	}

	// Add field values
	req.FieldValues = make(map[string]interface{})

	if importable.EstimatedHours > 0 {
		req.FieldValues["estimated_hours"] = importable.EstimatedHours
	}

	if importable.Branch != "" {
		req.FieldValues["branch"] = importable.Branch
	}

	if importable.Status != "" && importable.Status != "open" {
		req.FieldValues["status"] = importable.Status
	}

	// Add custom fields
	for k, v := range importable.CustomFields {
		req.FieldValues[k] = v
	}

	return req
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
