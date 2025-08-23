package cmd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	exportFormat  string
	exportOutput  string
	exportFilter  string
	exportInclude []string
	exportExclude []string
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export issues to various formats",
	Long: `Export issues to CSV, JSON, or YAML formats.

Supports filtering by status, priority, type, and other criteria.
Export includes all issue data including comments, attachments, and history.

Examples:
  issuemap export --format csv --output issues.csv
  issuemap export --format json --output issues.json --filter "status=open"
  issuemap export --format yaml --output issues.yaml
  ismp export --format csv --filter "priority=high,status=open"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runExport(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "csv", "export format (csv, json, yaml)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "output file (default: stdout)")
	exportCmd.Flags().StringVar(&exportFilter, "filter", "", "filter issues (e.g., status=open,priority=high)")
	exportCmd.Flags().StringSliceVar(&exportInclude, "include", []string{}, "fields to include (default: all)")
	exportCmd.Flags().StringSliceVar(&exportExclude, "exclude", []string{}, "fields to exclude")
}

type ExportableIssue struct {
	ID             string                 `json:"id" yaml:"id" csv:"id"`
	Title          string                 `json:"title" yaml:"title" csv:"title"`
	Description    string                 `json:"description" yaml:"description" csv:"description"`
	Type           string                 `json:"type" yaml:"type" csv:"type"`
	Status         string                 `json:"status" yaml:"status" csv:"status"`
	Priority       string                 `json:"priority" yaml:"priority" csv:"priority"`
	Assignee       string                 `json:"assignee" yaml:"assignee" csv:"assignee"`
	Branch         string                 `json:"branch" yaml:"branch" csv:"branch"`
	Labels         []string               `json:"labels" yaml:"labels" csv:"labels"`
	Milestone      string                 `json:"milestone" yaml:"milestone" csv:"milestone"`
	EstimatedHours float64                `json:"estimated_hours,omitempty" yaml:"estimated_hours,omitempty" csv:"estimated_hours"`
	ActualHours    float64                `json:"actual_hours,omitempty" yaml:"actual_hours,omitempty" csv:"actual_hours"`
	Created        time.Time              `json:"created" yaml:"created" csv:"created"`
	Updated        time.Time              `json:"updated" yaml:"updated" csv:"updated"`
	Closed         *time.Time             `json:"closed,omitempty" yaml:"closed,omitempty" csv:"closed"`
	Comments       []ExportableComment    `json:"comments,omitempty" yaml:"comments,omitempty" csv:"-"`
	Attachments    []ExportableAttachment `json:"attachments,omitempty" yaml:"attachments,omitempty" csv:"-"`
	CustomFields   map[string]string      `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty" csv:"-"`
}

type ExportableComment struct {
	ID     int       `json:"id" yaml:"id"`
	Author string    `json:"author" yaml:"author"`
	Date   time.Time `json:"date" yaml:"date"`
	Text   string    `json:"text" yaml:"text"`
}

type ExportableAttachment struct {
	ID          string `json:"id" yaml:"id"`
	Filename    string `json:"filename" yaml:"filename"`
	Size        int64  `json:"size" yaml:"size"`
	Type        string `json:"type" yaml:"type"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	UploadedBy  string `json:"uploaded_by" yaml:"uploaded_by"`
	UploadedAt  string `json:"uploaded_at" yaml:"uploaded_at"`
}

func runExport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate format
	if !isValidExportFormat(exportFormat) {
		return fmt.Errorf("invalid format: %s. Supported formats: csv, json, yaml", exportFormat)
	}

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

	// Build filter from string
	filter, err := parseExportFilter(exportFilter)
	if err != nil {
		printError(fmt.Errorf("invalid filter: %w", err))
		return err
	}

	// Get issues
	issueList, err := issueService.ListIssues(ctx, filter)
	if err != nil {
		printError(fmt.Errorf("failed to list issues: %w", err))
		return err
	}

	if len(issueList.Issues) == 0 {
		printInfo("No issues found matching the criteria.")
		return nil
	}

	// Convert to exportable format
	exportableIssues := convertToExportable(issueList.Issues)

	// Apply field filters
	exportableIssues = applyFieldFilters(exportableIssues, exportInclude, exportExclude)

	// Export to specified format
	var output []byte
	switch exportFormat {
	case "csv":
		output, err = exportToCSV(exportableIssues)
	case "json":
		output, err = exportToJSON(exportableIssues)
	case "yaml":
		output, err = exportToYAML(exportableIssues)
	default:
		return fmt.Errorf("unsupported format: %s", exportFormat)
	}

	if err != nil {
		printError(fmt.Errorf("failed to export: %w", err))
		return err
	}

	// Write to file or stdout
	if exportOutput != "" {
		err = os.WriteFile(exportOutput, output, 0644)
		if err != nil {
			printError(fmt.Errorf("failed to write to file: %w", err))
			return err
		}
		printSuccess(fmt.Sprintf("Exported %d issues to %s", len(exportableIssues), exportOutput))
	} else {
		fmt.Print(string(output))
	}

	return nil
}

func isValidExportFormat(format string) bool {
	validFormats := []string{"csv", "json", "yaml"}
	for _, v := range validFormats {
		if format == v {
			return true
		}
	}
	return false
}

func parseExportFilter(filterStr string) (repositories.IssueFilter, error) {
	filter := repositories.IssueFilter{}

	if filterStr == "" {
		return filter, nil
	}

	parts := strings.Split(filterStr, ",")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		switch key {
		case "status":
			status := entities.Status(value)
			filter.Status = &status
		case "priority":
			priority := entities.Priority(value)
			filter.Priority = &priority
		case "type":
			issueType := entities.IssueType(value)
			filter.Type = &issueType
		case "assignee":
			filter.Assignee = &value
		}
	}

	return filter, nil
}

func convertToExportable(issues []entities.Issue) []ExportableIssue {
	var exportable []ExportableIssue

	for _, issue := range issues {
		exp := ExportableIssue{
			ID:          string(issue.ID),
			Title:       issue.Title,
			Description: issue.Description,
			Type:        string(issue.Type),
			Status:      string(issue.Status),
			Priority:    string(issue.Priority),
			Branch:      issue.Branch,
			Created:     issue.Timestamps.Created,
			Updated:     issue.Timestamps.Updated,
			Closed:      issue.Timestamps.Closed,
		}

		// Assignee
		if issue.Assignee != nil {
			exp.Assignee = issue.Assignee.Username
		}

		// Milestone
		if issue.Milestone != nil {
			exp.Milestone = issue.Milestone.Name
		}

		// Labels
		for _, label := range issue.Labels {
			exp.Labels = append(exp.Labels, label.Name)
		}

		// Time estimates
		if issue.Metadata.EstimatedHours != nil {
			exp.EstimatedHours = *issue.Metadata.EstimatedHours
		}
		if issue.Metadata.ActualHours != nil {
			exp.ActualHours = *issue.Metadata.ActualHours
		}

		// Comments
		for _, comment := range issue.Comments {
			exp.Comments = append(exp.Comments, ExportableComment{
				ID:     comment.ID,
				Author: comment.Author,
				Date:   comment.Date,
				Text:   comment.Text,
			})
		}

		// Attachments
		for _, att := range issue.Attachments {
			exp.Attachments = append(exp.Attachments, ExportableAttachment{
				ID:          att.ID,
				Filename:    att.Filename,
				Size:        att.Size,
				Type:        string(att.Type),
				Description: att.Description,
				UploadedBy:  att.UploadedBy,
				UploadedAt:  att.UploadedAt.Format(time.RFC3339),
			})
		}

		// Custom fields
		if issue.Metadata.CustomFields != nil && len(issue.Metadata.CustomFields) > 0 {
			exp.CustomFields = issue.Metadata.CustomFields
		}

		exportable = append(exportable, exp)
	}

	return exportable
}

func applyFieldFilters(issues []ExportableIssue, include, exclude []string) []ExportableIssue {
	// For now, return all fields - field filtering can be implemented later if needed
	// This is a placeholder for future enhancement
	return issues
}

func exportToCSV(issues []ExportableIssue) ([]byte, error) {
	if len(issues) == 0 {
		return []byte{}, nil
	}

	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write headers
	headers := []string{
		"id", "title", "description", "type", "status", "priority",
		"assignee", "branch", "labels", "milestone", "estimated_hours",
		"actual_hours", "created", "updated", "closed",
	}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	// Write data
	for _, issue := range issues {
		record := []string{
			issue.ID,
			issue.Title,
			issue.Description,
			issue.Type,
			issue.Status,
			issue.Priority,
			issue.Assignee,
			issue.Branch,
			strings.Join(issue.Labels, ";"),
			issue.Milestone,
			fmt.Sprintf("%.2f", issue.EstimatedHours),
			fmt.Sprintf("%.2f", issue.ActualHours),
			issue.Created.Format(time.RFC3339),
			issue.Updated.Format(time.RFC3339),
		}

		if issue.Closed != nil {
			record = append(record, issue.Closed.Format(time.RFC3339))
		} else {
			record = append(record, "")
		}

		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return []byte(buf.String()), writer.Error()
}

func exportToJSON(issues []ExportableIssue) ([]byte, error) {
	return json.MarshalIndent(issues, "", "  ")
}

func exportToYAML(issues []ExportableIssue) ([]byte, error) {
	return yaml.Marshal(issues)
}
