package services

import (
	"context"
	"fmt"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// HistoryService provides high-level operations for issue history management
type HistoryService struct {
	historyRepo repositories.HistoryRepository
	gitRepo     repositories.GitRepository
}

// NewHistoryService creates a new history service
func NewHistoryService(
	historyRepo repositories.HistoryRepository,
	gitRepo repositories.GitRepository,
) *HistoryService {
	return &HistoryService{
		historyRepo: historyRepo,
		gitRepo:     gitRepo,
	}
}

// RecordIssueCreated records the creation of a new issue
func (s *HistoryService) RecordIssueCreated(ctx context.Context, issue *entities.Issue, author string) error {
	entry := entities.NewHistoryEntry(
		issue.ID,
		entities.ChangeTypeCreated,
		author,
		fmt.Sprintf("Issue created: %s", issue.Title),
	)

	// Add initial field values as "changes" for reference
	entry.AddFieldChange("title", nil, issue.Title)
	entry.AddFieldChange("description", nil, issue.Description)
	entry.AddFieldChange("type", nil, string(issue.Type))
	entry.AddFieldChange("status", nil, string(issue.Status))
	entry.AddFieldChange("priority", nil, string(issue.Priority))

	if issue.Assignee != nil {
		entry.AddFieldChange("assignee", nil, issue.Assignee.Username)
	}

	if len(issue.Labels) > 0 {
		var labelNames []string
		for _, label := range issue.Labels {
			labelNames = append(labelNames, label.Name)
		}
		entry.AddFieldChange("labels", nil, labelNames)
	}

	// Add git context if available
	if s.gitRepo != nil {
		if branch, err := s.gitRepo.GetCurrentBranch(ctx); err == nil {
			entry.SetMetadata("git_branch", branch)
		}
		if user, err := s.gitRepo.GetAuthorInfo(ctx); err == nil {
			entry.SetMetadata("git_author", user.Username)
			entry.SetMetadata("git_email", user.Email)
		}
	}

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordIssueUpdatedWithDetails records detailed changes to an existing issue with old and new values
func (s *HistoryService) RecordIssueUpdatedWithDetails(ctx context.Context, issueID entities.IssueID, oldIssue, newIssue *entities.Issue, author string) error {
	// Build a human-readable message based on what changed
	var changeParts []string

	entry := entities.NewHistoryEntry(
		issueID,
		entities.ChangeTypeUpdated,
		author,
		"Updated issue", // Will be updated based on changes
	)

	// Check for field changes and record old/new values
	if oldIssue.Title != newIssue.Title {
		entry.AddFieldChange("title", oldIssue.Title, newIssue.Title)
		changeParts = append(changeParts, "title")
	}

	if oldIssue.Description != newIssue.Description {
		entry.AddFieldChange("description", oldIssue.Description, newIssue.Description)
		changeParts = append(changeParts, "description")
	}

	if oldIssue.Type != newIssue.Type {
		entry.AddFieldChange("type", string(oldIssue.Type), string(newIssue.Type))
		changeParts = append(changeParts, "type")
	}

	if oldIssue.Status != newIssue.Status {
		entry.AddFieldChange("status", string(oldIssue.Status), string(newIssue.Status))
		changeParts = append(changeParts, "status")
	}

	if oldIssue.Priority != newIssue.Priority {
		entry.AddFieldChange("priority", string(oldIssue.Priority), string(newIssue.Priority))
		changeParts = append(changeParts, "priority")
	}

	// Check assignee changes
	oldAssignee := ""
	newAssignee := ""
	if oldIssue.Assignee != nil {
		oldAssignee = oldIssue.Assignee.Username
	}
	if newIssue.Assignee != nil {
		newAssignee = newIssue.Assignee.Username
	}
	if oldAssignee != newAssignee {
		entry.AddFieldChange("assignee", oldAssignee, newAssignee)
		changeParts = append(changeParts, "assignee")
	}

	// Check milestone changes
	oldMilestone := ""
	newMilestone := ""
	if oldIssue.Milestone != nil {
		oldMilestone = oldIssue.Milestone.Name
	}
	if newIssue.Milestone != nil {
		newMilestone = newIssue.Milestone.Name
	}
	if oldMilestone != newMilestone {
		entry.AddFieldChange("milestone", oldMilestone, newMilestone)
		changeParts = append(changeParts, "milestone")
	}

	// Check label changes
	oldLabels := make([]string, len(oldIssue.Labels))
	newLabels := make([]string, len(newIssue.Labels))
	for i, label := range oldIssue.Labels {
		oldLabels[i] = label.Name
	}
	for i, label := range newIssue.Labels {
		newLabels[i] = label.Name
	}

	// Compare label arrays
	if !stringSlicesEqual(oldLabels, newLabels) {
		entry.AddFieldChange("labels", oldLabels, newLabels)
		changeParts = append(changeParts, "labels")
	}

	if oldIssue.Branch != newIssue.Branch {
		entry.AddFieldChange("branch", oldIssue.Branch, newIssue.Branch)
		changeParts = append(changeParts, "branch")
	}

	// If no changes detected, don't record an entry
	if len(changeParts) == 0 {
		return nil
	}

	// Update the message with what actually changed
	entry.Message = fmt.Sprintf("Updated %s", joinWithComma(changeParts))

	// Add git context
	if s.gitRepo != nil {
		if branch, err := s.gitRepo.GetCurrentBranch(ctx); err == nil {
			entry.SetMetadata("git_branch", branch)
		}
	}

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordIssueFieldChanged records a specific field change with old and new values
func (s *HistoryService) RecordIssueFieldChanged(ctx context.Context, issueID entities.IssueID, field string, oldValue, newValue interface{}, author string) error {
	message := fmt.Sprintf("Changed %s from '%v' to '%v'", field, oldValue, newValue)

	entry := entities.NewHistoryEntry(
		issueID,
		entities.ChangeTypeUpdated,
		author,
		message,
	)

	entry.AddFieldChange(field, oldValue, newValue)

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordIssueAssigned records issue assignment
func (s *HistoryService) RecordIssueAssigned(ctx context.Context, issueID entities.IssueID, assignee string, author string) error {
	message := fmt.Sprintf("Assigned to %s", assignee)

	entry := entities.NewHistoryEntry(
		issueID,
		entities.ChangeTypeAssigned,
		author,
		message,
	)

	entry.AddFieldChange("assignee", nil, assignee)

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordIssueUnassigned records issue unassignment
func (s *HistoryService) RecordIssueUnassigned(ctx context.Context, issueID entities.IssueID, previousAssignee string, author string) error {
	message := "Unassigned"
	if previousAssignee != "" {
		message = fmt.Sprintf("Unassigned from %s", previousAssignee)
	}

	entry := entities.NewHistoryEntry(
		issueID,
		entities.ChangeTypeUnassigned,
		author,
		message,
	)

	entry.AddFieldChange("assignee", previousAssignee, nil)

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordIssueClosed records issue closure
func (s *HistoryService) RecordIssueClosed(ctx context.Context, issueID entities.IssueID, reason string, author string) error {
	message := "Closed"
	if reason != "" {
		message = fmt.Sprintf("Closed: %s", reason)
	}

	entry := entities.NewHistoryEntry(
		issueID,
		entities.ChangeTypeClosed,
		author,
		message,
	)

	entry.AddFieldChange("status", "open", "closed")
	if reason != "" {
		entry.SetMetadata("close_reason", reason)
	}

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordIssueReopened records issue reopening
func (s *HistoryService) RecordIssueReopened(ctx context.Context, issueID entities.IssueID, author string) error {
	entry := entities.NewHistoryEntry(
		issueID,
		entities.ChangeTypeReopened,
		author,
		"Reopened",
	)

	entry.AddFieldChange("status", "closed", "open")

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordIssueLabeled records label addition
func (s *HistoryService) RecordIssueLabeled(ctx context.Context, issueID entities.IssueID, labels []string, author string) error {
	message := fmt.Sprintf("Added labels: %s", joinWithComma(labels))

	entry := entities.NewHistoryEntry(
		issueID,
		entities.ChangeTypeLabeled,
		author,
		message,
	)

	entry.AddFieldChange("labels", nil, labels)

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordIssueCommented records a comment addition
func (s *HistoryService) RecordIssueCommented(ctx context.Context, issueID entities.IssueID, comment string, author string) error {
	entry := entities.NewHistoryEntry(
		issueID,
		entities.ChangeTypeCommented,
		author,
		"Added comment",
	)

	entry.SetMetadata("comment_text", comment)

	return s.historyRepo.AddEntry(ctx, entry)
}

// GetIssueHistory retrieves the complete history for an issue
func (s *HistoryService) GetIssueHistory(ctx context.Context, issueID entities.IssueID) (*entities.IssueHistory, error) {
	history, err := s.historyRepo.GetHistory(ctx, issueID)
	if err != nil {
		return nil, errors.Wrap(err, "HistoryService.GetIssueHistory", "get_history")
	}

	return history, nil
}

// GetAllHistory retrieves history for all issues with filtering
func (s *HistoryService) GetAllHistory(ctx context.Context, filter repositories.HistoryFilter) (*repositories.HistoryList, error) {
	historyList, err := s.historyRepo.ListEntries(ctx, filter)
	if err != nil {
		return nil, errors.Wrap(err, "HistoryService.GetAllHistory", "list_entries")
	}

	return historyList, nil
}

// GetHistoryStats returns statistics about issue history
func (s *HistoryService) GetHistoryStats(ctx context.Context, filter repositories.HistoryFilter) (*repositories.HistoryStats, error) {
	stats, err := s.historyRepo.GetHistoryStats(ctx, filter)
	if err != nil {
		return nil, errors.Wrap(err, "HistoryService.GetHistoryStats", "get_stats")
	}

	return stats, nil
}

// GetRecentActivity returns recent activity across all issues
func (s *HistoryService) GetRecentActivity(ctx context.Context, limit int) (*repositories.HistoryList, error) {
	filter := repositories.HistoryFilter{
		Limit: &limit,
	}

	return s.GetAllHistory(ctx, filter)
}

// GetActivityByAuthor returns activity for a specific author
func (s *HistoryService) GetActivityByAuthor(ctx context.Context, author string, limit int) (*repositories.HistoryList, error) {
	filter := repositories.HistoryFilter{
		Author: &author,
		Limit:  &limit,
	}

	return s.GetAllHistory(ctx, filter)
}

// GetActivitySince returns all activity since a specific time
func (s *HistoryService) GetActivitySince(ctx context.Context, since time.Time) (*repositories.HistoryList, error) {
	filter := repositories.HistoryFilter{
		Since: &since,
	}

	return s.GetAllHistory(ctx, filter)
}

// DeleteIssueHistory removes all history for a specific issue
func (s *HistoryService) DeleteIssueHistory(ctx context.Context, issueID entities.IssueID) error {
	err := s.historyRepo.DeleteHistory(ctx, issueID)
	if err != nil {
		return errors.Wrap(err, "HistoryService.DeleteIssueHistory", "delete_history")
	}

	return nil
}

// joinWithComma joins a slice of strings with commas
func joinWithComma(items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	if len(items) == 2 {
		return items[0] + " and " + items[1]
	}

	result := ""
	for i, item := range items {
		if i == len(items)-1 {
			result += "and " + item
		} else if i == 0 {
			result += item
		} else {
			result += ", " + item
		}
	}
	return result
}

// RecordTimerStarted records when a timer is started for an issue
func (s *HistoryService) RecordTimerStarted(ctx context.Context, issueID entities.IssueID, author, description string) error {
	message := "Started timer"
	if description != "" {
		message = fmt.Sprintf("Started timer: %s", description)
	}

	entry := entities.NewHistoryEntry(
		issueID,
		entities.ChangeTypeUpdated,
		author,
		message,
	)

	entry.SetMetadata("action", "timer_started")
	if description != "" {
		entry.SetMetadata("description", description)
	}

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordTimerStopped records when a timer is stopped for an issue
func (s *HistoryService) RecordTimerStopped(ctx context.Context, issueID entities.IssueID, author string, hours float64) error {
	message := fmt.Sprintf("Stopped timer (%.1f hours)", hours)

	entry := entities.NewHistoryEntry(
		issueID,
		entities.ChangeTypeUpdated,
		author,
		message,
	)

	entry.SetMetadata("action", "timer_stopped")
	entry.SetMetadata("hours_logged", fmt.Sprintf("%.2f", hours))

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordTimeLogged records when time is manually logged for an issue
func (s *HistoryService) RecordTimeLogged(ctx context.Context, issueID entities.IssueID, author string, hours float64, description string) error {
	message := fmt.Sprintf("Logged %.1f hours", hours)
	if description != "" {
		message = fmt.Sprintf("Logged %.1f hours: %s", hours, description)
	}

	entry := entities.NewHistoryEntry(
		issueID,
		entities.ChangeTypeUpdated,
		author,
		message,
	)

	entry.SetMetadata("action", "time_logged")
	entry.SetMetadata("hours_logged", fmt.Sprintf("%.2f", hours))
	if description != "" {
		entry.SetMetadata("description", description)
	}

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordDependencyCreated records when a dependency is created
func (s *HistoryService) RecordDependencyCreated(ctx context.Context, sourceID, targetID entities.IssueID, depType entities.DependencyType, author, message string) error {
	entry := entities.NewHistoryEntry(
		sourceID,
		entities.ChangeTypeUpdated,
		author,
		message,
	)

	entry.SetMetadata("action", "dependency_created")
	entry.SetMetadata("target_issue", string(targetID))
	entry.SetMetadata("dependency_type", string(depType))

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordDependencyRemoved records when a dependency is removed
func (s *HistoryService) RecordDependencyRemoved(ctx context.Context, sourceID, targetID entities.IssueID, depType entities.DependencyType, author, message string) error {
	entry := entities.NewHistoryEntry(
		sourceID,
		entities.ChangeTypeUpdated,
		author,
		message,
	)

	entry.SetMetadata("action", "dependency_removed")
	entry.SetMetadata("target_issue", string(targetID))
	entry.SetMetadata("dependency_type", string(depType))

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordDependencyResolved records when a dependency is resolved
func (s *HistoryService) RecordDependencyResolved(ctx context.Context, sourceID, targetID entities.IssueID, depType entities.DependencyType, author, message string) error {
	entry := entities.NewHistoryEntry(
		sourceID,
		entities.ChangeTypeUpdated,
		author,
		message,
	)

	entry.SetMetadata("action", "dependency_resolved")
	entry.SetMetadata("target_issue", string(targetID))
	entry.SetMetadata("dependency_type", string(depType))

	return s.historyRepo.AddEntry(ctx, entry)
}

// RecordDependencyReactivated records when a dependency is reactivated
func (s *HistoryService) RecordDependencyReactivated(ctx context.Context, sourceID, targetID entities.IssueID, depType entities.DependencyType, author, message string) error {
	entry := entities.NewHistoryEntry(
		sourceID,
		entities.ChangeTypeUpdated,
		author,
		message,
	)

	entry.SetMetadata("action", "dependency_reactivated")
	entry.SetMetadata("target_issue", string(targetID))
	entry.SetMetadata("dependency_type", string(depType))

	return s.historyRepo.AddEntry(ctx, entry)
}

// stringSlicesEqual compares two string slices for equality
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
