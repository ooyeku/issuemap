package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	burndownDateFrom string
	burndownDateTo   string
	burndownDays     int
)

// burndownCmd represents the burndown command
var burndownCmd = &cobra.Command{
	Use:   "burndown",
	Short: "Generate burndown chart data",
	Long: `Generate burndown chart data showing progress toward completion of active issues.

Examples:
  issuemap burndown                                    # Last 30 days
  issuemap burndown --days 14                         # Last 14 days
  issuemap burndown --from 2024-01-01 --to 2024-01-31  # Custom period`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Deprecation warning
		printWarning("DEPRECATED: 'burndown' command will be removed in a future version. Use 'report --type burndown' instead:")
		printWarning("  issuemap report --type burndown")
		fmt.Println()

		return runBurndown(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(burndownCmd)

	burndownCmd.Flags().StringVar(&burndownDateFrom, "from", "", "start date (YYYY-MM-DD)")
	burndownCmd.Flags().StringVar(&burndownDateTo, "to", "", "end date (YYYY-MM-DD)")
	burndownCmd.Flags().IntVarP(&burndownDays, "days", "d", 30, "number of days back from today")
}

func runBurndown(cmd *cobra.Command, args []string) error {
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

	metricsService := services.NewMetricsService(issueService, timeTrackingService)

	// Determine date range
	var startDate, endDate time.Time

	if burndownDateFrom != "" && burndownDateTo != "" {
		startDate, err = time.Parse("2006-01-02", burndownDateFrom)
		if err != nil {
			return fmt.Errorf("invalid start date format: %w", err)
		}

		endDate, err = time.Parse("2006-01-02", burndownDateTo)
		if err != nil {
			return fmt.Errorf("invalid end date format: %w", err)
		}
	} else {
		endDate = time.Now()
		startDate = endDate.AddDate(0, 0, -burndownDays)
	}

	// Add full day to end date
	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	// Calculate burndown data
	burndownData, err := metricsService.CalculateBurndown(ctx, startDate, endDate)
	if err != nil {
		printError(fmt.Errorf("failed to calculate burndown: %w", err))
		return err
	}

	displayBurndown(burndownData)
	return nil
}

func displayBurndown(data *services.BurndownData) {
	fmt.Printf("Burndown Chart\n")
	fmt.Printf("==============\n\n")

	fmt.Printf("Project: %s\n", data.ProjectName)
	fmt.Printf("Period: %s to %s\n",
		data.StartDate.Format("2006-01-02"),
		data.EndDate.Format("2006-01-02"))
	fmt.Printf("Total Estimated: %.1f hours\n", data.TotalEstimated)
	fmt.Printf("Total Completed: %.1f hours\n", data.TotalCompleted)
	fmt.Printf("Total Remaining: %.1f hours\n", data.TotalRemaining)
	fmt.Printf("Completion Rate: %.1f%%\n", data.CompletionRate*100)

	if data.ProjectedEndDate != nil {
		fmt.Printf("Projected End Date: %s\n", data.ProjectedEndDate.Format("2006-01-02"))
		if data.IsOnTrack {
			printSuccess("Project is on track")
		} else {
			printWarning("Project may be delayed")
		}
	}

	if len(data.DailyData) == 0 {
		fmt.Println("\nNo daily data available.")
		return
	}

	fmt.Printf("\nDaily Progress:\n")
	fmt.Printf("%-12s %-12s %-12s %-12s\n", "Date", "Completed", "Remaining", "Ideal Rem.")
	fmt.Printf("%-12s %-12s %-12s %-12s\n",
		"------------", "------------", "------------", "------------")

	// Show last 10 days or all if less than 10
	startIdx := 0
	if len(data.DailyData) > 10 {
		startIdx = len(data.DailyData) - 10
		fmt.Printf("... (showing last 10 days)\n")
	}

	for i := startIdx; i < len(data.DailyData); i++ {
		day := data.DailyData[i]
		fmt.Printf("%-12s %12.1f %12.1f %12.1f\n",
			day.Date.Format("2006-01-02"),
			day.CompletedHours,
			day.RemainingHours,
			day.IdealRemaining,
		)
	}

	// Show trend analysis
	if len(data.DailyData) >= 7 {
		fmt.Printf("\nTrend Analysis:\n")

		// Calculate recent daily average
		recentDays := data.DailyData[len(data.DailyData)-7:]
		var recentTotal float64
		for _, day := range recentDays {
			recentTotal += day.CompletedHours
		}
		recentAvg := recentTotal / 7.0

		fmt.Printf("• Recent daily average: %.1f hours\n", recentAvg)

		if data.TotalRemaining > 0 && recentAvg > 0 {
			daysToComplete := data.TotalRemaining / recentAvg
			fmt.Printf("• Days to completion at current rate: %.0f days\n", daysToComplete)
		}

		// Simple ASCII chart (very basic)
		fmt.Printf("\nSimple Chart (R=Remaining, I=Ideal):\n")
		maxHours := data.TotalEstimated
		chartWidth := 50

		// Show every few days to fit in chart
		step := len(data.DailyData) / chartWidth
		if step < 1 {
			step = 1
		}

		for i := 0; i < len(data.DailyData); i += step {
			day := data.DailyData[i]
			remainingBar := int((day.RemainingHours / maxHours) * float64(chartWidth))
			idealBar := int((day.IdealRemaining / maxHours) * float64(chartWidth))

			fmt.Printf("%s R:", day.Date.Format("01-02"))
			for j := 0; j < chartWidth; j++ {
				if j < remainingBar {
					fmt.Printf("█")
				} else {
					fmt.Printf("░")
				}
			}
			fmt.Printf("\n%s I:", "     ")
			for j := 0; j < chartWidth; j++ {
				if j < idealBar {
					fmt.Printf("▓")
				} else {
					fmt.Printf("░")
				}
			}
			fmt.Printf("\n")
		}

		fmt.Printf("0%%%s100%%\n", fmt.Sprintf("%*s", chartWidth-6, ""))
	}
}
