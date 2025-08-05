package services

import (
	"context"
	"fmt"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// TimeTrackingService provides high-level operations for time tracking
type TimeTrackingService struct {
	timeEntryRepo   repositories.TimeEntryRepository
	activeTimerRepo repositories.ActiveTimerRepository
	issueService    *IssueService
	historyService  *HistoryService
}

// NewTimeTrackingService creates a new time tracking service
func NewTimeTrackingService(
	timeEntryRepo repositories.TimeEntryRepository,
	activeTimerRepo repositories.ActiveTimerRepository,
	issueService *IssueService,
	historyService *HistoryService,
) *TimeTrackingService {
	return &TimeTrackingService{
		timeEntryRepo:   timeEntryRepo,
		activeTimerRepo: activeTimerRepo,
		issueService:    issueService,
		historyService:  historyService,
	}
}

// StartTimer starts a timer for the given issue and author
func (s *TimeTrackingService) StartTimer(ctx context.Context, issueID entities.IssueID, author, description string) (*entities.ActiveTimer, error) {
	// Check if issue exists
	_, err := s.issueService.GetIssue(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("issue not found: %w", err)
	}

	// Check if user already has an active timer
	existingTimer, _ := s.activeTimerRepo.GetByAuthor(ctx, author)
	if existingTimer != nil {
		return nil, fmt.Errorf("active timer already exists for author %s on issue %s", author, existingTimer.IssueID)
	}

	// Create new active timer
	timer := entities.NewActiveTimer(issueID, description, author)
	if err := s.activeTimerRepo.Create(ctx, timer); err != nil {
		return nil, fmt.Errorf("failed to create active timer: %w", err)
	}

	// Record in history
	if s.historyService != nil {
		s.historyService.RecordTimerStarted(ctx, issueID, author, description)
	}

	return timer, nil
}

// StopTimer stops the active timer for the given author and creates a time entry
func (s *TimeTrackingService) StopTimer(ctx context.Context, author string) (*entities.TimeEntry, error) {
	// Get active timer
	activeTimer, err := s.activeTimerRepo.GetByAuthor(ctx, author)
	if err != nil {
		return nil, fmt.Errorf("no active timer found for author: %s", author)
	}

	// Create time entry from timer
	endTime := time.Now()
	timeEntry := activeTimer.ToTimeEntry(endTime)

	// Save time entry
	if err := s.timeEntryRepo.Create(ctx, timeEntry); err != nil {
		return nil, fmt.Errorf("failed to create time entry: %w", err)
	}

	// Update issue actual hours
	issue, err := s.issueService.GetIssue(ctx, activeTimer.IssueID)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	newActualHours := issue.GetActualHours() + timeEntry.GetDurationHours()
	updates := map[string]interface{}{
		"actual_hours": newActualHours,
	}

	_, err = s.issueService.UpdateIssue(ctx, activeTimer.IssueID, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update issue actual hours: %w", err)
	}

	// Remove active timer
	if err := s.activeTimerRepo.Delete(ctx, activeTimer.IssueID, author); err != nil {
		return nil, fmt.Errorf("failed to remove active timer: %w", err)
	}

	// Record in history
	if s.historyService != nil {
		s.historyService.RecordTimerStopped(ctx, activeTimer.IssueID, author, timeEntry.GetDurationHours())
	}

	return timeEntry, nil
}

// LogTime manually logs time for an issue
func (s *TimeTrackingService) LogTime(ctx context.Context, issueID entities.IssueID, author string, duration time.Duration, description string) (*entities.TimeEntry, error) {
	// Check if issue exists
	issue, err := s.issueService.GetIssue(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("issue not found: %w", err)
	}

	// Create time entry
	timeEntry := entities.NewTimeEntry(issueID, entities.TimeEntryTypeManual, duration, description, author)
	if err := s.timeEntryRepo.Create(ctx, timeEntry); err != nil {
		return nil, fmt.Errorf("failed to create time entry: %w", err)
	}

	// Update issue actual hours
	newActualHours := issue.GetActualHours() + timeEntry.GetDurationHours()
	updates := map[string]interface{}{
		"actual_hours": newActualHours,
	}

	_, err = s.issueService.UpdateIssue(ctx, issueID, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update issue actual hours: %w", err)
	}

	// Record in history
	if s.historyService != nil {
		s.historyService.RecordTimeLogged(ctx, issueID, author, timeEntry.GetDurationHours(), description)
	}

	return timeEntry, nil
}

// GetActiveTimer returns the active timer for an author
func (s *TimeTrackingService) GetActiveTimer(ctx context.Context, author string) (*entities.ActiveTimer, error) {
	return s.activeTimerRepo.GetByAuthor(ctx, author)
}

// GetTimeEntries returns time entries with filtering
func (s *TimeTrackingService) GetTimeEntries(ctx context.Context, filter repositories.TimeEntryFilter) ([]*entities.TimeEntry, error) {
	return s.timeEntryRepo.List(ctx, filter)
}

// GetTimeEntriesByIssue returns all time entries for a specific issue
func (s *TimeTrackingService) GetTimeEntriesByIssue(ctx context.Context, issueID entities.IssueID) ([]*entities.TimeEntry, error) {
	return s.timeEntryRepo.GetByIssueID(ctx, issueID)
}

// GetTimeStats returns time tracking statistics
func (s *TimeTrackingService) GetTimeStats(ctx context.Context, filter repositories.TimeEntryFilter) (*repositories.TimeEntryStats, error) {
	return s.timeEntryRepo.GetStats(ctx, filter)
}

// GetActiveTimers returns all active timers
func (s *TimeTrackingService) GetActiveTimers(ctx context.Context) ([]*entities.ActiveTimer, error) {
	return s.activeTimerRepo.List(ctx)
}