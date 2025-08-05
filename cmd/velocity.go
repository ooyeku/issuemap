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
	velocityPeriods    int
	velocityPeriodType string
	velocityDateFrom   string
	velocityDateTo     string
)

// velocityCmd represents the velocity command
var velocityCmd = &cobra.Command{
	Use:   "velocity",
	Short: "Calculate team velocity metrics",
	Long: `Calculate and display team velocity metrics including completed issues, 
hours per period, and estimation accuracy.

Examples:
  issuemap velocity                              # Current week velocity
  issuemap velocity --periods 4 --type week     # Last 4 weeks
  issuemap velocity --periods 3 --type month    # Last 3 months
  issuemap velocity --from 2024-01-01 --to 2024-01-31  # Custom period`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runVelocity(cmd)
	},
}

func init() {
	rootCmd.AddCommand(velocityCmd)

	velocityCmd.Flags().IntVarP(&velocityPeriods, "periods", "p", 1, "number of periods to analyze")
	velocityCmd.Flags().StringVarP(&velocityPeriodType, "type", "t", "week", "period type (week, month)")
	velocityCmd.Flags().StringVar(&velocityDateFrom, "from", "", "custom start date (YYYY-MM-DD)")
	velocityCmd.Flags().StringVar(&velocityDateTo, "to", "", "custom end date (YYYY-MM-DD)")
}

func runVelocity(cmd *cobra.Command) error {
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

	// Handle custom date range or historical periods
	if velocityDateFrom != "" && velocityDateTo != "" {
		return runCustomPeriodVelocity(ctx, metricsService)
	}

	return runHistoricalVelocity(ctx, metricsService)
}

func runCustomPeriodVelocity(ctx context.Context, metricsService *services.MetricsService) error {
	startDate, err := time.Parse("2006-01-02", velocityDateFrom)
	if err != nil {
		return fmt.Errorf("invalid start date format: %w", err)
	}

	endDate, err := time.Parse("2006-01-02", velocityDateTo)
	if err != nil {
		return fmt.Errorf("invalid end date format: %w", err)
	}

	// Add full day to end date
	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	velocity, err := metricsService.CalculateVelocity(ctx, startDate, endDate)
	if err != nil {
		printError(fmt.Errorf("failed to calculate velocity: %w", err))
		return err
	}

	displayVelocity([]*services.VelocityData{velocity})
	return nil
}

func runHistoricalVelocity(ctx context.Context, metricsService *services.MetricsService) error {
	velocities, err := metricsService.GetHistoricalVelocity(ctx, velocityPeriods, velocityPeriodType)
	if err != nil {
		printError(fmt.Errorf("failed to calculate historical velocity: %w", err))
		return err
	}

	if len(velocities) == 0 {
		fmt.Println("No velocity data available for the specified periods.")
		return nil
	}

	displayVelocity(velocities)
	return nil
}

func displayVelocity(velocities []*services.VelocityData) {
	fmt.Printf("Team Velocity Report\n")
	fmt.Printf("===================\n\n")

	if len(velocities) == 1 {
		// Single period detailed view
		v := velocities[0]
		fmt.Printf("Period: %s\n", v.Period)
		fmt.Printf("Completed Issues: %d\n", v.CompletedIssues)
		fmt.Printf("Estimated Hours: %.1f\n", v.EstimatedHours)
		fmt.Printf("Actual Hours: %.1f\n", v.ActualHours)
		fmt.Printf("Velocity (Issues/Week): %.1f\n", v.VelocityPoints)
		fmt.Printf("Velocity (Hours/Week): %.1f\n", v.VelocityHours)
		fmt.Printf("Estimation Accuracy: %.1f%% ", v.Accuracy*100)
		
		if v.Accuracy > 1.1 {
			fmt.Printf("(Over-estimated)\n")
		} else if v.Accuracy < 0.9 {
			fmt.Printf("(Under-estimated)\n")
		} else {
			fmt.Printf("(Good)\n")
		}
		
		return
	}

	// Multi-period table view
	fmt.Printf("%-20s %-8s %-12s %-12s %-12s %-12s\n", 
		"Period", "Issues", "Est Hours", "Act Hours", "Vel/Week", "Accuracy")
	fmt.Printf("%-20s %-8s %-12s %-12s %-12s %-12s\n", 
		"--------------------", "--------", "------------", "------------", "------------", "------------")

	var totalIssues int
	var totalEstHours, totalActHours, totalVelPoints, totalVelHours float64

	for _, v := range velocities {
		accuracy := fmt.Sprintf("%.0f%%", v.Accuracy*100)
		fmt.Printf("%-20s %8d %12.1f %12.1f %12.1f %12s\n",
			v.Period,
			v.CompletedIssues,
			v.EstimatedHours,
			v.ActualHours,
			v.VelocityPoints,
			accuracy,
		)

		totalIssues += v.CompletedIssues
		totalEstHours += v.EstimatedHours
		totalActHours += v.ActualHours
		totalVelPoints += v.VelocityPoints
		totalVelHours += v.VelocityHours
	}

	// Calculate averages
	periods := float64(len(velocities))
	avgVelPoints := totalVelPoints / periods
	avgVelHours := totalVelHours / periods
	avgAccuracy := 1.0
	if totalEstHours > 0 {
		avgAccuracy = totalActHours / totalEstHours
	}

	fmt.Printf("%-20s %-8s %-12s %-12s %-12s %-12s\n", 
		"--------------------", "--------", "------------", "------------", "------------", "------------")
	fmt.Printf("%-20s %8d %12.1f %12.1f %12.1f %12s\n",
		"AVERAGE",
		int(float64(totalIssues)/periods),
		totalEstHours/periods,
		totalActHours/periods,
		avgVelPoints,
		fmt.Sprintf("%.0f%%", avgAccuracy*100),
	)

	// Summary insights
	fmt.Printf("\nSummary Insights:\n")
	fmt.Printf("• Average velocity: %.1f issues/week, %.1f hours/week\n", avgVelPoints, avgVelHours)
	
	if avgAccuracy > 1.1 {
		fmt.Printf("• Team tends to over-estimate (takes %.0f%% more time than estimated)\n", (avgAccuracy-1)*100)
	} else if avgAccuracy < 0.9 {
		fmt.Printf("• Team tends to under-estimate (takes %.0f%% less time than estimated)\n", (1-avgAccuracy)*100)
	} else {
		fmt.Printf("• Team has good estimation accuracy\n")
	}

	// Trend analysis for multi-period data
	if len(velocities) >= 3 {
		firstHalf := velocities[:len(velocities)/2]
		secondHalf := velocities[len(velocities)/2:]
		
		var firstHalfAvg, secondHalfAvg float64
		for _, v := range firstHalf {
			firstHalfAvg += v.VelocityPoints
		}
		firstHalfAvg /= float64(len(firstHalf))
		
		for _, v := range secondHalf {
			secondHalfAvg += v.VelocityPoints
		}
		secondHalfAvg /= float64(len(secondHalf))
		
		trend := secondHalfAvg - firstHalfAvg
		if trend > 0.5 {
			fmt.Printf("• Velocity is trending upward (+%.1f issues/week)\n", trend)
		} else if trend < -0.5 {
			fmt.Printf("• Velocity is trending downward (%.1f issues/week)\n", trend)
		} else {
			fmt.Printf("• Velocity is stable\n")
		}
	}
}