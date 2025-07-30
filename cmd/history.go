package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	historyIssueID  string
	historyAuthor   string
	historyType     string
	historyField    string
	historySince    string
	historyUntil    string
	historyLimit    int
	historyStats    bool
	historyRecent   bool
	historyDetailed bool
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show version control history of issues",
	Long: `Display the complete version control history of issues, showing all changes,
who made them, and when they occurred. This provides powerful audit trails and 
change tracking capabilities.

Examples:
  issuemap history                           # Show recent history for all issues
  issuemap history --issue ISSUE-001        # Show history for specific issue
  issuemap history --author alice           # Show all changes by alice
  issuemap history --type updated           # Show only update events
  issuemap history --field priority         # Show only priority changes
  issuemap history --since "2024-01-01"     # Show changes since date
  issuemap history --limit 20               # Limit to 20 entries
  issuemap history --stats                  # Show statistics summary
  issuemap history --recent --limit 10      # Show 10 most recent changes
  issuemap history --detailed               # Show detailed field changes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHistory(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(historyCmd)

	historyCmd.Flags().StringVar(&historyIssueID, "issue", "", "show history for specific issue ID")
	historyCmd.Flags().StringVar(&historyAuthor, "author", "", "filter by author")
	historyCmd.Flags().StringVar(&historyType, "type", "", "filter by change type (created, updated, closed, etc.)")
	historyCmd.Flags().StringVar(&historyField, "field", "", "filter by field name (title, priority, status, etc.)")
	historyCmd.Flags().StringVar(&historySince, "since", "", "show changes since date (YYYY-MM-DD)")
	historyCmd.Flags().StringVar(&historyUntil, "until", "", "show changes until date (YYYY-MM-DD)")
	historyCmd.Flags().IntVar(&historyLimit, "limit", app.DefaultHistoryLimit, "limit number of entries")
	historyCmd.Flags().BoolVar(&historyStats, "stats", false, "show statistics summary")
	historyCmd.Flags().BoolVar(&historyRecent, "recent", false, "show only recent activity")
	historyCmd.Flags().BoolVarP(&historyDetailed, "detailed", "d", false, "show detailed field changes")
}

func runHistory(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, app.ConfigDirName)
	historyRepo := storage.NewFileHistoryRepository(issuemapPath)

	var gitRepo *git.GitClient
	if gitClient, err := git.NewGitClient(repoPath); err == nil {
		gitRepo = gitClient
	}

	historyService := services.NewHistoryService(historyRepo, gitRepo)

	// Handle stats request
	if historyStats {
		return showHistoryStats(ctx, historyService)
	}

	// Build filter
	filter, err := buildHistoryFilter()
	if err != nil {
		printError(fmt.Errorf("invalid filter parameters: %w", err))
		return err
	}

	// Get history
	var historyList *repositories.HistoryList

	if historyIssueID != "" {
		// Get history for specific issue
		issueHistory, err := historyService.GetIssueHistory(ctx, entities.IssueID(historyIssueID))
		if err != nil {
			printError(fmt.Errorf("failed to get issue history: %w", err))
			return err
		}

		// Convert to HistoryList format
		historyList = &repositories.HistoryList{
			Entries: issueHistory.Entries,
			Total:   len(issueHistory.Entries),
			Count:   len(issueHistory.Entries),
		}
	} else {
		// Get all history
		historyList, err = historyService.GetAllHistory(ctx, filter)
		if err != nil {
			printError(fmt.Errorf("failed to get history: %w", err))
			return err
		}
	}

	// Display results
	if len(historyList.Entries) == 0 {
		fmt.Println("No history entries found matching the criteria.")
		return nil
	}

	displayHistory(historyList, historyDetailed)

	// Show summary
	fmt.Printf("\nShowing %d of %d total entries\n", historyList.Count, historyList.Total)

	return nil
}

func buildHistoryFilter() (repositories.HistoryFilter, error) {
	filter := repositories.HistoryFilter{
		Limit: &historyLimit,
	}

	// Parse issue ID
	if historyIssueID != "" {
		issueID := entities.IssueID(historyIssueID)
		filter.IssueID = &issueID
	}

	// Parse author
	if historyAuthor != "" {
		filter.Author = &historyAuthor
	}

	// Parse change type
	if historyType != "" {
		changeType := entities.ChangeType(historyType)
		filter.ChangeType = &changeType
	}

	// Parse field
	if historyField != "" {
		filter.Field = &historyField
	}

	// Parse since date
	if historySince != "" {
		since, err := time.Parse("2006-01-02", historySince)
		if err != nil {
			return filter, fmt.Errorf("invalid since date format (use YYYY-MM-DD): %w", err)
		}
		filter.Since = &since
	}

	// Parse until date
	if historyUntil != "" {
		until, err := time.Parse("2006-01-02", historyUntil)
		if err != nil {
			return filter, fmt.Errorf("invalid until date format (use YYYY-MM-DD): %w", err)
		}
		filter.Until = &until
	}

	// Handle recent flag
	if historyRecent {
		since := time.Now().AddDate(0, 0, -7) // Last 7 days
		filter.Since = &since
		if historyLimit == 50 { // Default limit
			newLimit := 10
			filter.Limit = &newLimit
		}
	}

	return filter, nil
}

func displayHistory(historyList *repositories.HistoryList, detailed bool) {
	fmt.Printf("\n=== Issue History ===\n\n")

	for _, entry := range historyList.Entries {
		displayHistoryEntry(entry, detailed)
		fmt.Println()
	}
}

func displayHistoryEntry(entry entities.HistoryEntry, detailed bool) {
	// Header with timestamp and author
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
	fmt.Printf("%s | %s | v%d\n", timestamp, entry.Author, entry.Version)

	// Issue ID and change type
	changeTypeIcon := getChangeTypeIcon(entry.Type)
	fmt.Printf("%s %s %s: %s\n", changeTypeIcon, entry.IssueID, entry.Type, entry.Message)

	// Show detailed field changes if requested
	if detailed && len(entry.Changes) > 0 {
		fmt.Printf("   Field changes:\n")
		for _, change := range entry.Changes {
			oldVal := formatValue(change.OldValue)
			newVal := formatValue(change.NewValue)

			if change.OldValue == nil {
				fmt.Printf("   - %s: %s\n", change.Field, newVal)
			} else if change.NewValue == nil {
				fmt.Printf("   - %s: %s -> (removed)\n", change.Field, oldVal)
			} else {
				fmt.Printf("   - %s: %s -> %s\n", change.Field, oldVal, newVal)
			}
		}
	}

	// Show metadata if available
	if detailed && len(entry.Metadata) > 0 {
		fmt.Printf("   Metadata:\n")
		for key, value := range entry.Metadata {
			fmt.Printf("   - %s: %v\n", key, value)
		}
	}
}

func getChangeTypeIcon(changeType entities.ChangeType) string {
	switch changeType {
	case entities.ChangeTypeCreated:
		return "[+]"
	case entities.ChangeTypeUpdated:
		return "[*]"
	case entities.ChangeTypeClosed:
		return "[X]"
	case entities.ChangeTypeReopened:
		return "[O]"
	case entities.ChangeTypeAssigned:
		return "[A]"
	case entities.ChangeTypeUnassigned:
		return "[-A]"
	case entities.ChangeTypeLabeled:
		return "[L]"
	case entities.ChangeTypeUnlabeled:
		return "[-L]"
	case entities.ChangeTypeCommented:
		return "[C]"
	case entities.ChangeTypeMilestoned:
		return "[M]"
	case entities.ChangeTypeUnmilestoned:
		return "[-M]"
	case entities.ChangeTypeLinked:
		return "[LK]"
	case entities.ChangeTypeUnlinked:
		return "[-LK]"
	default:
		return "[E]"
	}
}

func formatValue(value interface{}) string {
	if value == nil {
		return "(none)"
	}

	switch v := value.(type) {
	case []string:
		return "[" + strings.Join(v, ", ") + "]"
	case string:
		if v == "" {
			return "(empty)"
		}
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func showHistoryStats(ctx context.Context, historyService *services.HistoryService) error {
	filter := repositories.HistoryFilter{}

	stats, err := historyService.GetHistoryStats(ctx, filter)
	if err != nil {
		printError(fmt.Errorf("failed to get history statistics: %w", err))
		return err
	}

	fmt.Printf("\n=== Issue History Statistics ===\n\n")

	fmt.Printf("Overall Statistics:\n")
	fmt.Printf("   - Total issues with history: %d\n", stats.TotalIssuesWithHistory)
	fmt.Printf("   - Total history entries: %d\n", stats.TotalHistoryEntries)
	fmt.Printf("   - Average changes per issue: %.2f\n", stats.AverageChangesPerIssue)

	if stats.MostActiveIssue != nil {
		fmt.Printf("   - Most active issue: %s\n", *stats.MostActiveIssue)
	}

	if stats.MostActiveAuthor != "" {
		fmt.Printf("   - Most active author: %s\n", stats.MostActiveAuthor)
	}

	fmt.Printf("\nChanges by Type:\n")
	for changeType, count := range stats.EntriesByType {
		icon := getChangeTypeIcon(changeType)
		fmt.Printf("   %s %-12s: %d\n", icon, changeType, count)
	}

	fmt.Printf("\nChanges by Author:\n")
	for author, count := range stats.EntriesByAuthor {
		fmt.Printf("   - %-15s: %d changes\n", author, count)
	}

	if len(stats.ActivityByDay) > 0 {
		fmt.Printf("\nRecent Activity by Day:\n")

		// Show last 7 days of activity
		now := time.Now()
		for i := 6; i >= 0; i-- {
			day := now.AddDate(0, 0, -i)
			dayStr := day.Format("2006-01-02")
			count := stats.ActivityByDay[dayStr]
			if count > 0 {
				fmt.Printf("   - %s: %d changes\n", dayStr, count)
			}
		}
	}

	return nil
}
