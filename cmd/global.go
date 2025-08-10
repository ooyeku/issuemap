package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	globalService *services.GlobalService

	// Global command flags
	globalIncludeArchived bool
	globalProjectPath     string
	globalFormat          string
	globalTags            []string
	globalReason          string
)

// globalCmd represents the global command
var globalCmd = &cobra.Command{
	Use:   "global",
	Short: "Global issuemap operations across all projects",
	Long: `Global operations allow you to manage issues, projects, and backups 
across all your issuemap projects from a centralized location.

The global system maintains a directory at ~/.issuemap_global that tracks:
- All your issuemap projects
- Archived issues from all projects  
- Complete project backups with rich metadata
- Global configuration and settings

Examples:
  issuemap global list                    # List issues from all projects
  issuemap global projects               # List all tracked projects
  issuemap archive ISSUE-001             # Archive an issue to global storage
  issuemap backup                        # Backup current project
  issuemap archive list                  # List archived issues`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize global service
		globalService = services.NewGlobalService()
	},
}

// globalListCmd lists issues from all projects
var globalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues from all projects",
	Long: `List issues from all tracked issuemap projects.

This command aggregates issues from all active projects and displays them
with project context. Archived issues can be included with the --archived flag.

Examples:
  issuemap global list                   # List all active issues
  issuemap global list --archived        # Include archived issues  
  issuemap global list --format json     # Output as JSON`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		issues, err := globalService.GlobalListIssues(ctx, globalIncludeArchived)
		if err != nil {
			return fmt.Errorf("failed to list global issues: %w", err)
		}

		if len(issues) == 0 {
			fmt.Println("No issues found across tracked projects")
			if !globalIncludeArchived {
				fmt.Println("Use --archived to include archived issues")
			}
			return nil
		}

		return displayGlobalIssues(issues, globalFormat)
	},
}

// globalProjectsCmd lists all tracked projects
var globalProjectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all tracked projects",
	Long: `List all projects currently tracked by the global issuemap system.

Shows project paths, issue counts, last scan times, and backup status.
Projects are automatically discovered and registered when you run issuemap 
commands within them.

Examples:
  issuemap global projects              # List all projects
  issuemap global projects --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		projects, err := globalService.ListProjects(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		if len(projects) == 0 {
			fmt.Println("No projects are currently tracked")
			fmt.Println("Run 'issuemap init' in a project directory to start tracking it")
			return nil
		}

		return displayProjects(projects, globalFormat)
	},
}

// archiveCmd archives an issue to global storage
var archiveCmd = &cobra.Command{
	Use:   "archive <issue-id>",
	Short: "Archive an issue to global storage",
	Long: `Archive an issue from the current project to global storage.

Archived issues are moved from the local project to the global ~/.issuemap_global
directory, saving local space while preserving the complete issue history.
The issue is removed from the local project after successful archival.

Examples:
  issuemap archive ISSUE-001                    # Archive with default reason
  issuemap archive ISSUE-001 --reason "completed"  # Archive with custom reason`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Initialize global service if not already initialized
		if globalService == nil {
			globalService = services.NewGlobalService()
		}

		// Set up local issue repository
		issuemapPath := filepath.Join(".", app.ConfigDirName)
		issueRepo := storage.NewFileIssueRepository(issuemapPath)
		globalService.SetIssueRepository(issueRepo)

		issueID := entities.IssueID(args[0])
		reason := globalReason
		if reason == "" {
			reason = "Archived for space savings"
		}

		archivedIssue, err := globalService.ArchiveIssue(ctx, issueID, reason)
		if err != nil {
			return fmt.Errorf("failed to archive issue %s: %w", issueID, err)
		}

		fmt.Printf("Successfully archived issue %s\n", color.CyanString(string(issueID)))
		fmt.Printf("Project: %s\n", color.GreenString(archivedIssue.ProjectName))
		fmt.Printf("Archived at: %s\n", archivedIssue.ArchivedAt.Format(time.RFC3339))
		if archivedIssue.ArchiveReason != "" {
			fmt.Printf("Reason: %s\n", archivedIssue.ArchiveReason)
		}

		return nil
	},
}

// archiveListCmd lists archived issues
var archiveListCmd = &cobra.Command{
	Use:   "list",
	Short: "List archived issues",
	Long: `List issues that have been archived to global storage.

By default, shows archived issues for the current project. Use --project 
to specify a different project path, or omit to see archived issues from 
all projects.

Examples:
  issuemap archive list                       # List archived issues for current project
  issuemap archive list --project /path/to/project  # List for specific project  
  issuemap archive list --format json        # Output as JSON`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Initialize global service if not already initialized
		if globalService == nil {
			globalService = services.NewGlobalService()
		}

		var projectPath *string
		if globalProjectPath != "" {
			projectPath = &globalProjectPath
		}

		archivedIssues, err := globalService.ListArchivedIssues(ctx, projectPath)
		if err != nil {
			return fmt.Errorf("failed to list archived issues: %w", err)
		}

		if len(archivedIssues) == 0 {
			if projectPath != nil {
				fmt.Printf("No archived issues found for project: %s\n", *projectPath)
			} else {
				fmt.Println("No archived issues found for current project")
			}
			return nil
		}

		return displayArchivedIssues(archivedIssues, globalFormat)
	},
}

// archiveCmd represents the archive command group
var archiveRootCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive and manage archived issues",
	Long: `Archive issues to global storage and manage archived issues.

Archiving moves issues from local project storage to the global ~/.issuemap_global
directory, helping save space while preserving complete issue history.`,
}

// backupCmd creates a backup of the current project
var backupCmd = &cobra.Command{
	Use:   "backup [project-path]",
	Short: "Create a complete backup of a project",
	Long: `Create a complete backup of an issuemap project with rich metadata.

Backups are stored in ~/.issuemap_global/backups and include:
- All issues and their complete history
- Project configuration and settings  
- Git information (branch, commit, remote)
- System metadata (OS, architecture, user)
- Comprehensive statistics and summaries

If no project path is specified, backs up the current directory.

Examples:
  issuemap backup                              # Backup current project
  issuemap backup /path/to/project            # Backup specific project
  issuemap backup --tags release,v1.0        # Backup with tags`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Initialize global service if not already initialized
		if globalService == nil {
			globalService = services.NewGlobalService()
		}

		var projectPath string
		if len(args) > 0 {
			projectPath = args[0]
		}

		fmt.Println("Creating project backup...")

		backup, err := globalService.BackupProject(ctx, projectPath, globalTags)
		if err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}

		fmt.Printf("✓ Backup created successfully\n")
		fmt.Printf("Backup ID: %s\n", color.CyanString(backup.ID))
		fmt.Printf("Project: %s\n", color.GreenString(backup.ProjectName))
		fmt.Printf("Size: %s\n", formatFileSize(backup.Size))
		fmt.Printf("Issues: %d\n", backup.IssueCount)
		fmt.Printf("Created: %s\n", backup.CreatedAt.Format(time.RFC3339))

		if len(backup.Tags) > 0 {
			fmt.Printf("Tags: %s\n", color.YellowString(strings.Join(backup.Tags, ", ")))
		}

		return nil
	},
}

// Helper functions

func displayGlobalIssues(issues []repositories.GlobalIssue, format string) error {
	if format == "json" {
		return outputJSON(issues)
	}

	// Table format
	fmt.Printf("Found %d issues across %d projects\n\n",
		len(issues), countUniqueProjects(issues))

	for _, issue := range issues {
		status := colorStatus(issue.Status)
		priority := colorPriority(issue.Priority)

		archiveIndicator := ""
		if issue.IsArchived {
			archiveIndicator = color.YellowString(" [ARCHIVED]")
		}

		fmt.Printf("%s %s %s %s%s\n",
			color.CyanString(string(issue.ID)),
			status,
			priority,
			issue.Title,
			archiveIndicator)

		fmt.Printf("    Project: %s (%s)\n",
			color.GreenString(issue.ProjectName),
			globalService.FormatGlobalPath(issue.ProjectPath))

		if issue.Assignee != nil {
			fmt.Printf("    Assignee: %s\n", issue.Assignee.Username)
		}

		fmt.Printf("    Updated: %s\n\n", issue.Timestamps.Updated.Format("2006-01-02 15:04"))
	}

	return nil
}

func displayProjects(projects []*entities.ProjectInfo, format string) error {
	if format == "json" {
		return outputJSON(projects)
	}

	// Table format
	fmt.Printf("Found %d tracked projects\n\n", len(projects))

	for _, project := range projects {
		status := colorProjectStatus(string(project.Status))

		fmt.Printf("%s %s\n",
			color.CyanString(project.Name),
			status)

		fmt.Printf("    Path: %s\n", globalService.FormatGlobalPath(project.Path))
		fmt.Printf("    Issues: %d active", project.IssueCount)

		if project.ArchivedCount > 0 {
			fmt.Printf(", %d archived", project.ArchivedCount)
		}

		fmt.Printf("\n    Last scan: %s\n", project.LastScan.Format("2006-01-02 15:04"))

		if project.LastBackup != nil {
			fmt.Printf("    Last backup: %s\n", project.LastBackup.Format("2006-01-02 15:04"))
		} else {
			fmt.Printf("    Last backup: %s\n", color.RedString("never"))
		}

		if project.GitRemote != "" {
			fmt.Printf("    Git remote: %s\n", project.GitRemote)
		}

		if len(project.Tags) > 0 {
			fmt.Printf("    Tags: %s\n", color.YellowString(strings.Join(project.Tags, ", ")))
		}

		fmt.Println()
	}

	return nil
}

func displayArchivedIssues(issues []*entities.ArchivedIssue, format string) error {
	if format == "json" {
		return outputJSON(issues)
	}

	// Table format
	fmt.Printf("Found %d archived issues\n\n", len(issues))

	for _, archived := range issues {
		issue := archived.Issue
		status := colorStatus(issue.Status)
		priority := colorPriority(issue.Priority)

		fmt.Printf("%s %s %s %s\n",
			color.CyanString(string(issue.ID)),
			status,
			priority,
			issue.Title)

		fmt.Printf("    Project: %s\n", color.GreenString(archived.ProjectName))
		fmt.Printf("    Archived: %s\n", archived.ArchivedAt.Format("2006-01-02 15:04"))

		if archived.ArchiveReason != "" {
			fmt.Printf("    Reason: %s\n", archived.ArchiveReason)
		}

		if archived.ArchivedBy != "" {
			fmt.Printf("    Archived by: %s\n", archived.ArchivedBy)
		}

		fmt.Println()
	}

	return nil
}

func countUniqueProjects(issues []repositories.GlobalIssue) int {
	projects := make(map[string]bool)
	for _, issue := range issues {
		projects[issue.ProjectPath] = true
	}
	return len(projects)
}

func colorProjectStatus(status string) string {
	switch status {
	case "active":
		return color.GreenString(status)
	case "inactive":
		return color.YellowString(status)
	case "archived":
		return color.BlueString(status)
	case "deleted":
		return color.RedString(status)
	default:
		return status
	}
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func init() {
	rootCmd.AddCommand(globalCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(backupCmd)

	// Global command and subcommands
	globalCmd.AddCommand(globalListCmd)
	globalCmd.AddCommand(globalProjectsCmd)

	// Archive command and subcommands
	archiveRootCmd.AddCommand(archiveListCmd)
	globalCmd.AddCommand(archiveRootCmd)

	// Global flags
	globalCmd.PersistentFlags().StringVarP(&globalFormat, "format", "f", "table", "output format (table, json)")

	// Global list flags
	globalListCmd.Flags().BoolVar(&globalIncludeArchived, "archived", false, "include archived issues")

	// Archive flags
	archiveCmd.Flags().StringVar(&globalReason, "reason", "", "reason for archiving")
	archiveListCmd.Flags().StringVar(&globalProjectPath, "project", "", "project path (default: current directory)")

	// Backup flags
	backupCmd.Flags().StringSliceVar(&globalTags, "tags", []string{}, "tags to apply to backup")
}
