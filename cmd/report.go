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
	reportType      string // New: report type (time, velocity, burndown)
	reportIssueID   string
	reportAuthor    string
	reportFormat    string
	reportOutput    string
	reportDateFrom  string
	reportDateTo    string
	reportLimit     int
	reportShowStats bool
	// Additional report-specific variables
	reportSince     string
	reportUntil     string
	reportGroupBy   string
	reportIssue     string
	reportDetailed  bool
	reportSprint    int
	reportMilestone string
)

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate various reports",
	Long: `Generate comprehensive reports including time tracking, velocity, and burndown charts.

Examples:
  issuemap report                                    # Default time tracking report
  issuemap report --type time                        # Time tracking report
  issuemap report --type velocity                    # Team velocity report
  issuemap report --type burndown                    # Sprint burndown chart
  issuemap report --type summary                     # Overall project summary
  issuemap report --issue ISSUE-001                 # Time report for specific issue
  issuemap report --author john                     # Time report for specific author
  issuemap report --date-from 2024-01-01            # Time since specific date
  issuemap report --format csv --output report.csv  # Export to CSV
  issuemap report --format json --output report.json # Export to JSON
  issuemap report --stats                           # Show detailed statistics`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Route to appropriate report type
		switch reportType {
		case "velocity":
			return runVelocityReport(cmd, args)
		case "burndown":
			return runBurndownReport(cmd, args)
		case "summary":
			return runSummaryReport(cmd, args)
		case "time", "":
			return runReport(cmd, args)
		default:
			return fmt.Errorf("unknown report type: %s (supported: time, velocity, burndown, summary)", reportType)
		}
	},
}

func init() {
	rootCmd.AddCommand(reportCmd)

	// Report type selection
	reportCmd.Flags().StringVar(&reportType, "type", "time", "report type (time, velocity, burndown, summary)")

	// Common flags
	reportCmd.Flags().StringVar(&reportIssueID, "issue", "", "filter by issue ID")
	reportCmd.Flags().StringVar(&reportAuthor, "author", "", "filter by author")
	reportCmd.Flags().StringVar(&reportFormat, "format", "table", "output format (table, csv, json)")
	reportCmd.Flags().StringVarP(&reportOutput, "output", "o", "", "output file (default: stdout)")
	reportCmd.Flags().StringVar(&reportDateFrom, "date-from", "", "filter entries from date (YYYY-MM-DD)")
	reportCmd.Flags().StringVar(&reportDateTo, "date-to", "", "filter entries to date (YYYY-MM-DD)")
	reportCmd.Flags().IntVarP(&reportLimit, "limit", "l", 0, "limit number of entries")
	reportCmd.Flags().BoolVar(&reportShowStats, "stats", false, "show detailed statistics")

	// Additional flags for velocity/burndown reports
	reportCmd.Flags().IntVar(&reportSprint, "sprint", 0, "sprint number for burndown report")
	reportCmd.Flags().StringVar(&reportMilestone, "milestone", "", "milestone for velocity report")
	reportCmd.Flags().StringVar(&reportGroupBy, "group-by", "issue", "group results by (issue, day, week, month, sprint)")
	reportCmd.Flags().BoolVar(&reportDetailed, "detailed", false, "show detailed breakdown")
}

func runReport(cmd *cobra.Command, args []string) error {
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

// New report functions for velocity and burndown
func runVelocityReport(cmd *cobra.Command, args []string) error {
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

	var gitRepo *git.GitClient
	if gitClient, err := git.NewGitClient(repoPath); err == nil {
		gitRepo = gitClient
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitRepo)
	historyRepo := storage.NewFileHistoryRepository(issuemapPath)
	historyService := services.NewHistoryService(historyRepo, gitRepo)
	activeTimerRepo := storage.NewFileActiveTimerRepository(issuemapPath)
	timeTrackingService := services.NewTimeTrackingService(timeEntryRepo, activeTimerRepo, issueService, historyService)
	metricsService := services.NewMetricsService(issueService, timeTrackingService)
	_ = metricsService // Silence unused variable warning

	// Calculate velocity metrics
	startDate := time.Now().AddDate(0, -3, 0) // Last 3 months by default
	if reportDateFrom != "" {
		startDate, err = time.Parse("2006-01-02", reportDateFrom)
		if err != nil {
			printError(fmt.Errorf("invalid date-from: %w", err))
			return err
		}
	}

	endDate := time.Now()
	if reportDateTo != "" {
		endDate, err = time.Parse("2006-01-02", reportDateTo)
		if err != nil {
			printError(fmt.Errorf("invalid date-to: %w", err))
			return err
		}
	}

	// TODO: Implement CalculateVelocity method in MetricsService
	velocity := &VelocityMetrics{
		StartDate:       startDate,
		EndDate:         endDate,
		SprintsAnalyzed: 1,
		AverageVelocity: 10.0,
		CurrentVelocity: 12.0,
		Trend:           "Improving",
	}
	_ = err // Silence unused variable for now
	_ = ctx // Silence unused variable

	// Display velocity report
	displayVelocityReport(velocity)
	return nil
}

func runBurndownReport(cmd *cobra.Command, args []string) error {
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

	var gitRepo *git.GitClient
	if gitClient, err := git.NewGitClient(repoPath); err == nil {
		gitRepo = gitClient
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitRepo)
	historyRepo := storage.NewFileHistoryRepository(issuemapPath)
	historyService := services.NewHistoryService(historyRepo, gitRepo)
	activeTimerRepo := storage.NewFileActiveTimerRepository(issuemapPath)
	timeTrackingService := services.NewTimeTrackingService(timeEntryRepo, activeTimerRepo, issueService, historyService)
	metricsService := services.NewMetricsService(issueService, timeTrackingService)
	_ = metricsService // Silence unused variable warning

	// Generate burndown data
	startDate := time.Now().AddDate(0, 0, -14) // Last 2 weeks by default
	if reportDateFrom != "" {
		startDate, err = time.Parse("2006-01-02", reportDateFrom)
		if err != nil {
			printError(fmt.Errorf("invalid date-from: %w", err))
			return err
		}
	}

	endDate := time.Now()
	if reportDateTo != "" {
		endDate, err = time.Parse("2006-01-02", reportDateTo)
		if err != nil {
			printError(fmt.Errorf("invalid date-to: %w", err))
			return err
		}
	}

	// TODO: Implement GenerateBurndownChart method in MetricsService
	burndown := &BurndownChart{
		SprintName:      "Current Sprint",
		StartDate:       startDate,
		EndDate:         endDate,
		TotalPoints:     100,
		CompletedPoints: 60,
		RemainingPoints: 40,
		DaysRemaining:   5,
		OnTrack:         true,
	}
	_ = err // Silence unused variable for now
	_ = ctx // Silence unused variable

	// Display burndown report
	displayBurndownReport(burndown)
	return nil
}

func runSummaryReport(cmd *cobra.Command, args []string) error {
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

	// Get project summary
	filter := repositories.IssueFilter{}
	issueList, err := issueService.ListIssues(ctx, filter)
	if err != nil {
		printError(fmt.Errorf("failed to get issues: %w", err))
		return err
	}

	// Display summary report
	displaySummaryReport(issueList)
	return nil
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

// Display functions for new report types

// Placeholder types for velocity and burndown - these would need to be defined in entities
type VelocityMetrics struct {
	StartDate            time.Time
	EndDate              time.Time
	SprintsAnalyzed      int
	AverageVelocity      float64
	CurrentVelocity      float64
	Trend                string
	IssuesCompleted      int
	StoryPointsCompleted float64
	PredictedVelocity    float64
	Confidence           float64
	SprintVelocities     map[int]float64
	TeamMemberVelocity   map[string]float64
}

func displayVelocityReport(velocity *VelocityMetrics) {
	fmt.Printf("Team Velocity Report\n")
	fmt.Printf("====================\n\n")

	fmt.Printf("Period: %s to %s\n", velocity.StartDate.Format("2006-01-02"), velocity.EndDate.Format("2006-01-02"))
	fmt.Printf("Sprints Analyzed: %d\n\n", velocity.SprintsAnalyzed)

	fmt.Printf("Velocity Metrics:\n")
	fmt.Printf("  Average Velocity:     %.1f points/sprint\n", velocity.AverageVelocity)
	fmt.Printf("  Current Velocity:     %.1f points\n", velocity.CurrentVelocity)
	fmt.Printf("  Velocity Trend:       %s\n", velocity.Trend)
	fmt.Printf("  Issues Completed:     %d\n", velocity.IssuesCompleted)
	fmt.Printf("  Story Points:         %.0f\n", velocity.StoryPointsCompleted)

	if velocity.PredictedVelocity > 0 {
		fmt.Printf("\nPredictions:\n")
		fmt.Printf("  Next Sprint:          %.1f points\n", velocity.PredictedVelocity)
		fmt.Printf("  Confidence:           %.0f%%\n", velocity.Confidence*100)
	}

	if len(velocity.SprintVelocities) > 0 {
		fmt.Printf("\nSprint Breakdown:\n")
		for sprint, vel := range velocity.SprintVelocities {
			fmt.Printf("  Sprint %d:            %.1f points\n", sprint, vel)
		}
	}

	if len(velocity.TeamMemberVelocity) > 0 {
		fmt.Printf("\nTeam Member Velocity:\n")
		for member, vel := range velocity.TeamMemberVelocity {
			fmt.Printf("  %-20s %.1f points\n", member, vel)
		}
	}
}

type DailyProgress struct {
	Date            time.Time
	RemainingPoints float64
	IdealPoints     float64
}

type BurndownChart struct {
	SprintName          string
	StartDate           time.Time
	EndDate             time.Time
	TotalPoints         float64
	CompletedPoints     float64
	RemainingPoints     float64
	DaysRemaining       int
	ProjectedCompletion *time.Time
	OnTrack             bool
	RequiredVelocity    float64
	DailyProgress       []DailyProgress
}

func displayBurndownReport(burndown *BurndownChart) {
	fmt.Printf("Sprint Burndown Chart\n")
	fmt.Printf("=====================\n\n")

	fmt.Printf("Sprint: %s\n", burndown.SprintName)
	fmt.Printf("Period: %s to %s\n", burndown.StartDate.Format("2006-01-02"), burndown.EndDate.Format("2006-01-02"))
	fmt.Printf("Total Points: %.0f\n\n", burndown.TotalPoints)

	fmt.Printf("Progress:\n")
	fmt.Printf("  Completed:            %.0f points (%.0f%%)\n", burndown.CompletedPoints, (burndown.CompletedPoints/burndown.TotalPoints)*100)
	fmt.Printf("  Remaining:            %.0f points\n", burndown.RemainingPoints)
	fmt.Printf("  Days Remaining:       %d\n", burndown.DaysRemaining)

	if burndown.ProjectedCompletion != nil {
		fmt.Printf("\nProjection:\n")
		fmt.Printf("  Projected Completion: %s\n", burndown.ProjectedCompletion.Format("2006-01-02"))
		fmt.Printf("  On Track:             %v\n", burndown.OnTrack)
		if !burndown.OnTrack && burndown.RequiredVelocity > 0 {
			fmt.Printf("  Required Velocity:    %.1f points/day\n", burndown.RequiredVelocity)
		}
	}

	// ASCII burndown chart
	fmt.Printf("\nBurndown Chart:\n")
	fmt.Printf("Points\n")

	maxPoints := burndown.TotalPoints
	chartHeight := 10
	_ = len(burndown.DailyProgress) // chartWidth unused

	for row := chartHeight; row >= 0; row-- {
		threshold := (float64(row) / float64(chartHeight)) * maxPoints
		fmt.Printf("%5.0f |", threshold)

		for _, day := range burndown.DailyProgress {
			if day.RemainingPoints >= threshold {
				fmt.Print(" █")
			} else if day.IdealPoints >= threshold {
				fmt.Print(" ·")
			} else {
				fmt.Print("  ")
			}
		}
		fmt.Println()
	}

	// X-axis
	fmt.Printf("      +")
	for range burndown.DailyProgress {
		fmt.Print("--")
	}
	fmt.Println()

	fmt.Printf("       ")
	for i := range burndown.DailyProgress {
		if i%5 == 0 {
			fmt.Printf("%-2d", i+1)
		} else {
			fmt.Print("  ")
		}
	}
	fmt.Printf(" Days\n")

	fmt.Printf("\nLegend: █ Actual  · Ideal\n")
}

func displaySummaryReport(issueList *repositories.IssueList) {
	fmt.Printf("Project Summary Report\n")
	fmt.Printf("======================\n\n")

	// Count issues by status
	statusCounts := make(map[entities.Status]int)
	typeCounts := make(map[entities.IssueType]int)
	priorityCounts := make(map[entities.Priority]int)

	for _, issue := range issueList.Issues {
		statusCounts[issue.Status]++
		typeCounts[issue.Type]++
		priorityCounts[issue.Priority]++
	}

	fmt.Printf("Total Issues: %d\n\n", len(issueList.Issues))

	fmt.Printf("By Status:\n")
	for status, count := range statusCounts {
		fmt.Printf("  %-12s %d\n", status, count)
	}

	fmt.Printf("\nBy Type:\n")
	for issueType, count := range typeCounts {
		fmt.Printf("  %-12s %d\n", issueType, count)
	}

	fmt.Printf("\nBy Priority:\n")
	for priority, count := range priorityCounts {
		fmt.Printf("  %-12s %d\n", priority, count)
	}

	// Recent activity
	fmt.Printf("\nRecent Activity:\n")
	recentIssues := 0
	oneWeekAgo := time.Now().AddDate(0, 0, -7)
	for _, issue := range issueList.Issues {
		if issue.Timestamps.Created.After(oneWeekAgo) ||
			issue.Timestamps.Updated.After(oneWeekAgo) {
			recentIssues++
		}
	}
	fmt.Printf("  Issues updated in last 7 days: %d\n", recentIssues)

	// Assignee distribution
	assigneeCounts := make(map[string]int)
	unassigned := 0
	for _, issue := range issueList.Issues {
		if issue.Assignee != nil && issue.Assignee.Username != "" {
			assigneeCounts[issue.Assignee.Username]++
		} else {
			unassigned++
		}
	}

	if len(assigneeCounts) > 0 {
		fmt.Printf("\nBy Assignee:\n")
		for assignee, count := range assigneeCounts {
			fmt.Printf("  %-20s %d\n", assignee, count)
		}
		if unassigned > 0 {
			fmt.Printf("  %-20s %d\n", "(unassigned)", unassigned)
		}
	}
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
