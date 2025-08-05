package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// VelocityData represents velocity metrics for a time period
type VelocityData struct {
	Period           string        `json:"period"`
	StartDate        time.Time     `json:"start_date"`
	EndDate          time.Time     `json:"end_date"`
	CompletedIssues  int           `json:"completed_issues"`
	CompletedHours   float64       `json:"completed_hours"`
	EstimatedHours   float64       `json:"estimated_hours"`
	ActualHours      float64       `json:"actual_hours"`
	VelocityPoints   float64       `json:"velocity_points"`  // Issues completed per week
	VelocityHours    float64       `json:"velocity_hours"`   // Hours completed per week
	Accuracy         float64       `json:"accuracy"`         // Actual vs Estimated ratio
}

// BurndownData represents burndown chart data
type BurndownData struct {
	ProjectName      string               `json:"project_name"`
	StartDate        time.Time            `json:"start_date"`
	EndDate          time.Time            `json:"end_date"`
	TotalEstimated   float64              `json:"total_estimated"`
	TotalCompleted   float64              `json:"total_completed"`
	TotalRemaining   float64              `json:"total_remaining"`
	DailyData        []BurndownDayData    `json:"daily_data"`
	CompletionRate   float64              `json:"completion_rate"`
	ProjectedEndDate *time.Time           `json:"projected_end_date,omitempty"`
	IsOnTrack        bool                 `json:"is_on_track"`
}

// BurndownDayData represents a single day's burndown data
type BurndownDayData struct {
	Date           time.Time `json:"date"`
	CompletedHours float64   `json:"completed_hours"`
	RemainingHours float64   `json:"remaining_hours"`
	IdealRemaining float64   `json:"ideal_remaining"`
}

// MetricsService provides velocity and burndown calculations
type MetricsService struct {
	issueService        *IssueService
	timeTrackingService *TimeTrackingService
}

// NewMetricsService creates a new metrics service
func NewMetricsService(issueService *IssueService, timeTrackingService *TimeTrackingService) *MetricsService {
	return &MetricsService{
		issueService:        issueService,
		timeTrackingService: timeTrackingService,
	}
}

// CalculateVelocity calculates velocity metrics for a given time period
func (s *MetricsService) CalculateVelocity(ctx context.Context, startDate, endDate time.Time) (*VelocityData, error) {
	// Get all issues
	filter := repositories.IssueFilter{
		UpdatedSince: &startDate,
	}
	issueList, err := s.issueService.ListIssues(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Filter completed issues in the time period
	var completedIssues []*entities.Issue
	var totalEstimatedHours, totalActualHours float64

	for _, issue := range issueList.Issues {
		// Check if issue was completed in the time period
		if issue.Status == entities.StatusDone || issue.Status == entities.StatusClosed {
			if issue.Timestamps.Closed != nil &&
				issue.Timestamps.Closed.After(startDate) &&
				issue.Timestamps.Closed.Before(endDate) {
				completedIssues = append(completedIssues, &issue)
				totalEstimatedHours += issue.GetEstimatedHours()
				totalActualHours += issue.GetActualHours()
			}
		}
	}

	// Calculate velocity (per week)
	duration := endDate.Sub(startDate)
	weeks := duration.Hours() / (24 * 7)
	if weeks == 0 {
		weeks = 1
	}

	velocityPoints := float64(len(completedIssues)) / weeks
	velocityHours := totalActualHours / weeks

	// Calculate accuracy (actual vs estimated)
	accuracy := 1.0
	if totalEstimatedHours > 0 {
		accuracy = totalActualHours / totalEstimatedHours
	}

	return &VelocityData{
		Period:          formatPeriod(startDate, endDate),
		StartDate:       startDate,
		EndDate:         endDate,
		CompletedIssues: len(completedIssues),
		CompletedHours:  totalActualHours,
		EstimatedHours:  totalEstimatedHours,
		ActualHours:     totalActualHours,
		VelocityPoints:  velocityPoints,
		VelocityHours:   velocityHours,
		Accuracy:        accuracy,
	}, nil
}

// CalculateBurndown calculates burndown chart data for active issues
func (s *MetricsService) CalculateBurndown(ctx context.Context, startDate, endDate time.Time) (*BurndownData, error) {
	// Get all active/in-progress issues
	filter := repositories.IssueFilter{}
	issueList, err := s.issueService.ListIssues(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Filter issues that were active during the period
	var activeIssues []*entities.Issue
	var totalEstimated float64

	for _, issue := range issueList.Issues {
		// Include issues that were created before end date and not completed before start date
		if issue.Timestamps.Created.Before(endDate) {
			if issue.Timestamps.Closed == nil || issue.Timestamps.Closed.After(startDate) {
				activeIssues = append(activeIssues, &issue)
				totalEstimated += issue.GetEstimatedHours()
			}
		}
	}

	if len(activeIssues) == 0 {
		return &BurndownData{
			ProjectName:    "No Active Issues",
			StartDate:      startDate,
			EndDate:        endDate,
			TotalEstimated: 0,
			DailyData:      []BurndownDayData{},
		}, nil
	}

	// Get time entries for the period
	timeFilter := repositories.TimeEntryFilter{
		DateFrom: &startDate,
		DateTo:   &endDate,
	}
	timeEntries, err := s.timeTrackingService.GetTimeEntries(ctx, timeFilter)
	if err != nil {
		return nil, err
	}

	// Calculate daily burndown data
	dailyData := s.calculateDailyBurndown(activeIssues, timeEntries, startDate, endDate, totalEstimated)

	// Calculate current totals
	var totalCompleted, totalRemaining float64
	for _, issue := range activeIssues {
		totalCompleted += issue.GetActualHours()
		totalRemaining += issue.GetRemainingHours()
	}

	// Calculate completion rate and projection
	completionRate := 0.0
	if totalEstimated > 0 {
		completionRate = totalCompleted / totalEstimated
	}

	projectedEndDate := s.calculateProjectedEndDate(dailyData, totalRemaining, endDate)
	isOnTrack := projectedEndDate == nil || projectedEndDate.Before(endDate) || projectedEndDate.Equal(endDate)

	return &BurndownData{
		ProjectName:      "Project Burndown",
		StartDate:        startDate,
		EndDate:          endDate,
		TotalEstimated:   totalEstimated,
		TotalCompleted:   totalCompleted,
		TotalRemaining:   totalRemaining,
		DailyData:        dailyData,
		CompletionRate:   completionRate,
		ProjectedEndDate: projectedEndDate,
		IsOnTrack:        isOnTrack,
	}, nil
}

// calculateDailyBurndown calculates day-by-day burndown data
func (s *MetricsService) calculateDailyBurndown(issues []*entities.Issue, timeEntries []*entities.TimeEntry, startDate, endDate time.Time, totalEstimated float64) []BurndownDayData {
	// Group time entries by date
	dailyHours := make(map[string]float64)
	for _, entry := range timeEntries {
		dateKey := entry.StartTime.Format("2006-01-02")
		dailyHours[dateKey] += entry.GetDurationHours()
	}

	var dailyData []BurndownDayData
	currentRemaining := totalEstimated
	duration := endDate.Sub(startDate)
	totalDays := int(duration.Hours() / 24)

	// Generate data for each day
	for day := 0; day <= totalDays; day++ {
		currentDate := startDate.AddDate(0, 0, day)
		dateKey := currentDate.Format("2006-01-02")

		// Hours completed on this day
		hoursCompleted := dailyHours[dateKey]
		currentRemaining -= hoursCompleted

		// Ideal remaining (linear burndown)
		idealRemaining := totalEstimated * (1.0 - float64(day)/float64(totalDays))

		dailyData = append(dailyData, BurndownDayData{
			Date:           currentDate,
			CompletedHours: hoursCompleted,
			RemainingHours: math.Max(0, currentRemaining),
			IdealRemaining: math.Max(0, idealRemaining),
		})
	}

	return dailyData
}

// calculateProjectedEndDate estimates when the project will be completed based on current velocity
func (s *MetricsService) calculateProjectedEndDate(dailyData []BurndownDayData, remainingHours float64, originalEndDate time.Time) *time.Time {
	if remainingHours <= 0 || len(dailyData) < 7 {
		return nil
	}

	// Calculate average daily velocity from the last 7 days
	recentDays := dailyData
	if len(dailyData) > 7 {
		recentDays = dailyData[len(dailyData)-7:]
	}

	var totalCompleted float64
	for _, day := range recentDays {
		totalCompleted += day.CompletedHours
	}

	if totalCompleted <= 0 {
		return nil
	}

	averageDailyVelocity := totalCompleted / float64(len(recentDays))
	daysToComplete := int(math.Ceil(remainingHours / averageDailyVelocity))

	projectedDate := time.Now().AddDate(0, 0, daysToComplete)
	return &projectedDate
}

// GetHistoricalVelocity calculates velocity for multiple periods
func (s *MetricsService) GetHistoricalVelocity(ctx context.Context, periods int, periodType string) ([]*VelocityData, error) {
	var velocities []*VelocityData
	now := time.Now()

	for i := 0; i < periods; i++ {
		var startDate, endDate time.Time

		switch periodType {
		case "week":
			endDate = now.AddDate(0, 0, -i*7)
			startDate = endDate.AddDate(0, 0, -7)
		case "month":
			endDate = now.AddDate(0, -i, 0)
			startDate = endDate.AddDate(0, -1, 0)
		default:
			return nil, fmt.Errorf("unsupported period type: %s", periodType)
		}

		velocity, err := s.CalculateVelocity(ctx, startDate, endDate)
		if err != nil {
			continue // Skip periods with errors
		}

		velocities = append(velocities, velocity)
	}

	// Sort by start date (oldest first)
	sort.Slice(velocities, func(i, j int) bool {
		return velocities[i].StartDate.Before(velocities[j].StartDate)
	})

	return velocities, nil
}

// formatPeriod formats a time period for display
func formatPeriod(start, end time.Time) string {
	return start.Format("2006-01-02") + " to " + end.Format("2006-01-02")
}