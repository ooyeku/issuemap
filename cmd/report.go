package cmd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	reportIssueID   string
	reportAuthor    string
	reportFormat    string
	reportOutput    string
	reportDateFrom  string
	reportDateTo    string
	reportLimit     int
	reportShowStats bool
)

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate time tracking reports",
	Long: `Generate comprehensive time tracking reports with various filtering and output options.

Examples:
  issuemap report                                    # Summary of all time entries
  issuemap report --issue ISSUE-001                 # Time report for specific issue
  issuemap report --author john                     # Time report for specific author
  issuemap report --date-from 2024-01-01            # Time since specific date
  issuemap report --format csv --output report.csv  # Export to CSV
  issuemap report --format json --output report.json # Export to JSON
  issuemap report --stats                           # Show detailed statistics`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd)
	},
}

func init() {
	rootCmd.AddCommand(reportCmd)
	
	reportCmd.Flags().StringVar(&reportIssueID, "issue", "", "filter by issue ID")
	reportCmd.Flags().StringVar(&reportAuthor, "author", "", "filter by author")
	reportCmd.Flags().StringVar(&reportFormat, "format", "table", "output format (table, csv, json)")
	reportCmd.Flags().StringVarP(&reportOutput, "output", "o", "", "output file (default: stdout)")
	reportCmd.Flags().StringVar(&reportDateFrom, "date-from", "", "filter entries from date (YYYY-MM-DD)")
	reportCmd.Flags().StringVar(&reportDateTo, "date-to", "", "filter entries to date (YYYY-MM-DD)")
	reportCmd.Flags().IntVarP(&reportLimit, "limit", "l", 0, "limit number of entries")
	reportCmd.Flags().BoolVar(&reportShowStats, "stats", false, "show detailed statistics")
}

func runReport(cmd *cobra.Command) error {
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
	timeEntryRepo := storage.NewFileTimeEntryRepository(issuemapPath)
	activeTimerRepo := storage.NewFileActiveTimerRepository(issuemapPath)

	var gitRepo *git.GitClient
	if gitClient, err := git.NewGitClient(repoPath); err == nil {
		gitRepo = gitClient
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitRepo)
	historyRepo := storage.NewFileHistoryRepository(issuemapPath)
	historyService := services.NewHistoryService(historyRepo, gitRepo)
	
	timeTrackingService := services.NewTimeTrackingService(
		timeEntryRepo,
		activeTimerRepo,
		issueService,
		historyService,
	)

	// Build filter from command line options
	filter, err := buildTimeEntryFilter()
	if err != nil {
		printError(fmt.Errorf("invalid filter options: %w", err))
		return err
	}

	// Get time entries
	entries, err := timeTrackingService.GetTimeEntries(ctx, filter)
	if err != nil {
		printError(fmt.Errorf("failed to get time entries: %w", err))
		return err
	}

	// Get statistics if requested
	var stats *repositories.TimeEntryStats
	if reportShowStats {
		stats, err = timeTrackingService.GetTimeStats(ctx, filter)
		if err != nil {
			printError(fmt.Errorf("failed to get time statistics: %w", err))
			return err
		}
	}

	// Generate report based on format
	switch strings.ToLower(reportFormat) {
	case "table":
		return generateTableReport(entries, stats)
	case "csv":
		return generateCSVReport(entries, reportOutput)
	case "json":
		return generateJSONReport(entries, stats, reportOutput)
	default:
		return fmt.Errorf("unsupported format: %s (supported: table, csv, json)", reportFormat)
	}
}

func buildTimeEntryFilter() (repositories.TimeEntryFilter, error) {
	filter := repositories.TimeEntryFilter{}

	if reportIssueID != "" {
		issueID := entities.IssueID(reportIssueID)
		filter.IssueID = &issueID
	}

	if reportAuthor != "" {
		filter.Author = &reportAuthor
	}

	if reportDateFrom != "" {
		dateFrom, err := time.Parse("2006-01-02", reportDateFrom)
		if err != nil {
			return filter, fmt.Errorf("invalid date-from format: %w", err)
		}
		filter.DateFrom = &dateFrom
	}

	if reportDateTo != "" {
		dateTo, err := time.Parse("2006-01-02", reportDateTo)
		if err != nil {
			return filter, fmt.Errorf("invalid date-to format: %w", err)
		}
		// Set to end of day
		dateTo = dateTo.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		filter.DateTo = &dateTo
	}

	if reportLimit > 0 {
		filter.Limit = reportLimit
	}

	return filter, nil
}

func generateTableReport(entries []*entities.TimeEntry, stats *repositories.TimeEntryStats) error {
	if len(entries) == 0 {
		fmt.Println("No time entries found matching the criteria.")
		return nil
	}

	// Print summary header
	var totalDuration time.Duration
	for _, entry := range entries {
		totalDuration += entry.GetDuration()
	}

	fmt.Printf("Time Tracking Report\n")
	fmt.Printf("===================\n\n")
	fmt.Printf("Total Entries: %d\n", len(entries))
	fmt.Printf("Total Time: %.1f hours\n", totalDuration.Hours())
	if len(entries) > 0 {
		avgDuration := totalDuration / time.Duration(len(entries))
		fmt.Printf("Average Time: %.1f hours\n", avgDuration.Hours())
	}
	fmt.Printf("\n")

	// Print detailed entries
	fmt.Printf("%-12s %-20s %-10s %-8s %-19s %s\n", "Issue", "Author", "Type", "Hours", "Date", "Description")
	fmt.Printf("%-12s %-20s %-10s %-8s %-19s %s\n", 
		strings.Repeat("-", 12), 
		strings.Repeat("-", 20), 
		strings.Repeat("-", 10), 
		strings.Repeat("-", 8), 
		strings.Repeat("-", 19), 
		strings.Repeat("-", 20))

	for _, entry := range entries {
		description := entry.Description
		if len(description) > 20 {
			description = description[:17] + "..."
		}
		fmt.Printf("%-12s %-20s %-10s %8.1f %-19s %s\n",
			entry.IssueID,
			truncateText(entry.Author, 20),
			entry.Type,
			entry.GetDurationHours(),
			entry.StartTime.Format("2006-01-02 15:04"),
			description,
		)
	}

	// Print statistics if requested
	if stats != nil {
		fmt.Printf("\nDetailed Statistics\n")
		fmt.Printf("==================\n\n")
		
		fmt.Printf("Time by Author:\n")
		// Sort authors by time
		type authorTime struct {
			author string
			hours  float64
		}
		var authorTimes []authorTime
		for author, duration := range stats.TimeByAuthor {
			authorTimes = append(authorTimes, authorTime{author, duration.Hours()})
		}
		sort.Slice(authorTimes, func(i, j int) bool {
			return authorTimes[i].hours > authorTimes[j].hours
		})
		for _, at := range authorTimes {
			fmt.Printf("  %-20s %.1f hours\n", at.author, at.hours)
		}

		fmt.Printf("\nTime by Issue:\n")
		// Sort issues by time
		type issueTime struct {
			issue string
			hours float64
		}
		var issueTimes []issueTime
		for issue, duration := range stats.TimeByIssue {
			issueTimes = append(issueTimes, issueTime{string(issue), duration.Hours()})
		}
		sort.Slice(issueTimes, func(i, j int) bool {
			return issueTimes[i].hours > issueTimes[j].hours
		})
		for _, it := range issueTimes {
			fmt.Printf("  %-12s %.1f hours\n", it.issue, it.hours)
		}

		fmt.Printf("\nEntries by Type:\n")
		for entryType, count := range stats.EntriesByType {
			fmt.Printf("  %-10s %d entries\n", entryType, count)
		}
	}

	return nil
}

func generateCSVReport(entries []*entities.TimeEntry, outputFile string) error {
	var output *os.File
	var err error

	if outputFile != "" {
		output, err = os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer output.Close()
	} else {
		output = os.Stdout
	}

	writer := csv.NewWriter(output)
	defer writer.Flush()

	// Write header
	header := []string{"Issue ID", "Author", "Type", "Hours", "Description", "Start Time", "End Time", "Created At"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write entries
	for _, entry := range entries {
		endTime := ""
		if entry.EndTime != nil {
			endTime = entry.EndTime.Format("2006-01-02 15:04:05")
		}

		record := []string{
			string(entry.IssueID),
			entry.Author,
			string(entry.Type),
			fmt.Sprintf("%.2f", entry.GetDurationHours()),
			entry.Description,
			entry.StartTime.Format("2006-01-02 15:04:05"),
			endTime,
			entry.CreatedAt.Format("2006-01-02 15:04:05"),
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	if outputFile != "" {
		printSuccess(fmt.Sprintf("CSV report saved to %s", outputFile))
	}

	return nil
}

func generateJSONReport(entries []*entities.TimeEntry, stats *repositories.TimeEntryStats, outputFile string) error {
	report := map[string]interface{}{
		"entries": entries,
		"summary": map[string]interface{}{
			"total_entries": len(entries),
			"total_hours":   calculateTotalHours(entries),
		},
	}

	if stats != nil {
		report["statistics"] = stats
	}

	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, reportJSON, 0644); err != nil {
			return fmt.Errorf("failed to write JSON file: %w", err)
		}
		printSuccess(fmt.Sprintf("JSON report saved to %s", outputFile))
	} else {
		fmt.Println(string(reportJSON))
	}

	return nil
}

func calculateTotalHours(entries []*entities.TimeEntry) float64 {
	var total time.Duration
	for _, entry := range entries {
		total += entry.GetDuration()
	}
	return total.Hours()
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}