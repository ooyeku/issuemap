package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	listStatus    string
	listType      string
	listPriority  string
	listAssignee  string
	listLabels    []string
	listMilestone string
	listBranch    string
	listLimit     int
	listAll       bool
	listBlocked   bool
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues",
	Long: `List issues with optional filtering.

Examples:
  issuemap list
  issuemap list --status open
  issuemap list --type bug --priority high
  issuemap list --assignee username
  issuemap list --labels bug,urgent`,
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listStatus, "status", "s", "", "filter by status (open, in-progress, review, done, closed)")
	listCmd.Flags().StringVarP(&listType, "type", "t", "", "filter by type (bug, feature, task, epic)")
	listCmd.Flags().StringVarP(&listPriority, "priority", "p", "", "filter by priority (low, medium, high, critical)")
	listCmd.Flags().StringVarP(&listAssignee, "assignee", "a", "", "filter by assignee")
	listCmd.Flags().StringSliceVarP(&listLabels, "labels", "l", []string{}, "filter by labels (comma separated)")
	listCmd.Flags().StringVarP(&listMilestone, "milestone", "m", "", "filter by milestone")
	listCmd.Flags().StringVarP(&listBranch, "branch", "b", "", "filter by branch")
	listCmd.Flags().IntVar(&listLimit, "limit", 20, "limit number of results")
	listCmd.Flags().BoolVar(&listAll, "all", false, "show all issues (no limit)")
	listCmd.Flags().BoolVar(&listBlocked, "blocked", false, "show only blocked issues")
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

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

	// Initialize dependency service if needed for blocked filtering
	var dependencyService *services.DependencyService
	if listBlocked {
		dependencyRepo := storage.NewFileDependencyRepository(issuemapPath)
		historyRepo := storage.NewFileHistoryRepository(issuemapPath)
		historyService := services.NewHistoryService(historyRepo, gitRepo)
		dependencyService = services.NewDependencyService(dependencyRepo, issueService, historyService)
	}

	// Build filter
	filter := repositories.IssueFilter{}

	if listStatus != "" {
		status := entities.Status(listStatus)
		filter.Status = &status
	}
	if listType != "" {
		issueType := entities.IssueType(listType)
		filter.Type = &issueType
	}
	if listPriority != "" {
		priority := entities.Priority(listPriority)
		filter.Priority = &priority
	}
	if listAssignee != "" {
		filter.Assignee = &listAssignee
	}
	if listMilestone != "" {
		filter.Milestone = &listMilestone
	}
	if listBranch != "" {
		filter.Branch = &listBranch
	}
	if len(listLabels) > 0 {
		filter.Labels = listLabels
	}
	if !listAll {
		filter.Limit = &listLimit
	}

	// Get issues
	issueList, err := issueService.ListIssues(ctx, filter)
	if err != nil {
		printError(fmt.Errorf("failed to list issues: %w", err))
		return err
	}

	// Filter for blocked issues if requested
	if listBlocked && dependencyService != nil {
		blockedIssueIDs, err := dependencyService.GetBlockedIssues(ctx)
		if err != nil {
			printError(fmt.Errorf("failed to get blocked issues: %w", err))
			return err
		}

		// Create a map for faster lookup
		blockedMap := make(map[entities.IssueID]bool)
		for _, id := range blockedIssueIDs {
			blockedMap[id] = true
		}

		// Filter the issues to only include blocked ones
		var blockedIssues []entities.Issue
		for _, issue := range issueList.Issues {
			if blockedMap[issue.ID] {
				blockedIssues = append(blockedIssues, issue)
			}
		}

		// Update the issue list
		issueList.Issues = blockedIssues
		issueList.Count = len(blockedIssues)
	}

	if len(issueList.Issues) == 0 {
		if noColor {
			fmt.Println("No issues found.")
		} else {
			color.HiBlack("No issues found.")
		}
		return nil
	}

	// Display results
	if format == "table" {
		displayIssuesTable(issueList.Issues)
	} else {
		fmt.Printf("Found %d issues (showing %d)\n", issueList.Total, issueList.Count)
		for _, issue := range issueList.Issues {
			fmt.Printf("%s: %s [%s] (%s)\n", issue.ID, issue.Title, issue.Status, issue.Type)
		}
	}

	if issueList.Total > issueList.Count {
		if noColor {
			fmt.Printf("\nShowing %d of %d issues. Use --limit or --all to see more.\n", issueList.Count, issueList.Total)
		} else {
			fmt.Printf("\n")
			color.HiBlack("Showing %d of %d issues. Use --limit or --all to see more.", issueList.Count, issueList.Total)
		}
	}

	return nil
}

func displayIssuesTable(issues []entities.Issue) {
	// Column widths - optimized for better terminal fit
	const (
		idWidth       = 10
		titleWidth    = 24
		typeWidth     = 7
		statusWidth   = 10
		priorityWidth = 8
		assigneeWidth = 10
		labelWidth    = 12
		updatedWidth  = 6
	)

	// Print enhanced header
	if noColor {
		fmt.Printf("%-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s\n",
			idWidth, "ID", titleWidth, "Title", typeWidth, "Type",
			statusWidth, "Status", priorityWidth, "Priority", assigneeWidth, "Assignee",
			labelWidth, "Labels", updatedWidth, "Updated")
		fmt.Printf("%s %s %s %s %s %s %s %s\n",
			strings.Repeat("-", idWidth), strings.Repeat("-", titleWidth), strings.Repeat("-", typeWidth),
			strings.Repeat("-", statusWidth), strings.Repeat("-", priorityWidth), strings.Repeat("-", assigneeWidth),
			strings.Repeat("-", labelWidth), strings.Repeat("-", updatedWidth))
	} else {
		// For colored headers, use simple padding without complex calculations
		fmt.Printf("%s %s %s %s %s %s %s %s\n",
			padColoredString(colorHeader("ID"), idWidth),
			padColoredString(colorHeader("Title"), titleWidth),
			padColoredString(colorHeader("Type"), typeWidth),
			padColoredString(colorHeader("Status"), statusWidth),
			padColoredString(colorHeader("Priority"), priorityWidth),
			padColoredString(colorHeader("Assignee"), assigneeWidth),
			padColoredString(colorHeader("Labels"), labelWidth),
			padColoredString(colorHeader("Updated"), updatedWidth))
		color.HiBlack("%s+%s+%s+%s+%s+%s+%s+%s",
			strings.Repeat("-", idWidth), strings.Repeat("-", titleWidth), strings.Repeat("-", typeWidth),
			strings.Repeat("-", statusWidth), strings.Repeat("-", priorityWidth), strings.Repeat("-", assigneeWidth),
			strings.Repeat("-", labelWidth), strings.Repeat("-", updatedWidth))
	}

	for _, issue := range issues {
		// Format and truncate each field to fit column width
		idStr := string(issue.ID)
		// Add attachment indicator if issue has attachments
		if issue.HasAttachments() {
			idStr = idStr + " [+]"
		}
		id := truncateString(idStr, idWidth)
		title := truncateString(issue.Title, titleWidth)
		issueType := truncateString(string(issue.Type), typeWidth)
		status := truncateString(string(issue.Status), statusWidth)
		priority := truncateString(string(issue.Priority), priorityWidth)

		assignee := "-"
		if issue.Assignee != nil {
			assignee = truncateString(issue.Assignee.Username, assigneeWidth)
		}

		// Format labels
		var labelDisplay string
		if len(issue.Labels) > 0 {
			var labelNames []string
			for _, label := range issue.Labels {
				labelNames = append(labelNames, label.Name)
			}
			labelDisplay = truncateString(strings.Join(labelNames, ","), labelWidth)
		} else {
			labelDisplay = "-"
		}

		updated := issue.Timestamps.Updated.Format("Jan 02")

		if noColor {
			fmt.Printf("%-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s\n",
				idWidth, id, titleWidth, title, typeWidth, issueType,
				statusWidth, status, priorityWidth, priority, assigneeWidth, assignee,
				labelWidth, labelDisplay, updatedWidth, updated)
		} else {
			// For colored output, create each field with exact width and join them
			fields := []string{
				padColoredString(colorIssueID(entities.IssueID(id)), idWidth),
				padColoredString(colorValue(title), titleWidth),
				padColoredString(colorType(entities.IssueType(issueType)), typeWidth),
				padColoredString(colorStatus(entities.Status(status)), statusWidth),
				padColoredString(colorPriority(entities.Priority(priority)), priorityWidth),
				padColoredString(colorValue(assignee), assigneeWidth),
				padColoredString(formatLabelsColored(issue.Labels, labelWidth), labelWidth),
				color.HiBlackString(updated), // Last column doesn't need padding
			}
			fmt.Printf("%s\n", strings.Join(fields, " "))
		}
	}

	if !noColor {
		color.HiBlack("%s+%s+%s+%s+%s+%s+%s+%s",
			strings.Repeat("-", idWidth), strings.Repeat("-", titleWidth), strings.Repeat("-", typeWidth),
			strings.Repeat("-", statusWidth), strings.Repeat("-", priorityWidth), strings.Repeat("-", assigneeWidth),
			strings.Repeat("-", labelWidth), strings.Repeat("-", updatedWidth))
	}
}

// truncateString truncates a string to fit within the specified width
func truncateString(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

// padColoredString pads a colored string to the specified width
// This accounts for ANSI escape codes that don't contribute to visual width
func padColoredString(coloredStr string, width int) string {
	// Calculate visual width by removing ANSI escape codes
	stripped := stripAnsiCodes(coloredStr)
	visualWidth := len(stripped)

	if visualWidth >= width {
		// If content is too long, truncate the visual part and reapply colors
		if visualWidth > width {
			truncated := truncateString(stripped, width)
			// For truncated content, we can't preserve colors perfectly,
			// so just return the truncated version
			return truncated
		}
		return coloredStr
	}

	// Add padding spaces to reach exact desired width
	padding := strings.Repeat(" ", width-visualWidth)
	return coloredStr + padding
}

// stripAnsiCodes removes ANSI escape codes from a string to calculate visual width
func stripAnsiCodes(s string) string {
	// Simple regex to remove ANSI escape sequences
	// This handles standard color codes like \033[31m, \033[0m etc.
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)
	return ansiRegex.ReplaceAllString(s, "")
}

// formatLabelsColored formats labels with colors for the given width
func formatLabelsColored(labels []entities.Label, width int) string {
	if len(labels) == 0 {
		return "-"
	}

	if noColor {
		var labelNames []string
		for _, label := range labels {
			labelNames = append(labelNames, label.Name)
		}
		result := strings.Join(labelNames, ",")
		return truncateString(result, width)
	}

	// For colored output, try to fit as many labels as possible
	var coloredLabels []string
	totalWidth := 0

	for i, label := range labels {
		colored := color.HiYellowString(label.Name)
		labelWidth := len(label.Name) // Visual width without ANSI codes

		if i > 0 {
			totalWidth += 1 // for comma
		}

		if totalWidth+labelWidth > width {
			if len(coloredLabels) == 0 {
				// At least show first label truncated
				return color.HiYellowString(truncateString(label.Name, width))
			}
			// Add "+N" indicator for remaining labels
			remaining := len(labels) - i
			indicator := color.HiBlackString("+%d", remaining)
			indicatorWidth := len(fmt.Sprintf("+%d", remaining))

			if totalWidth+1+indicatorWidth <= width {
				return strings.Join(coloredLabels, ",") + "," + indicator
			}
			// Not enough space for indicator, just return what we have
			return strings.Join(coloredLabels, ",")
		}

		coloredLabels = append(coloredLabels, colored)
		totalWidth += labelWidth
	}

	return strings.Join(coloredLabels, ",")
}
