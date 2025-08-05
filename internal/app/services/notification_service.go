package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// NotificationType represents different types of notifications
type NotificationType string

const (
	NotificationTypeDependencyBlocked     NotificationType = "dependency_blocked"
	NotificationTypeDependencyUnblocked   NotificationType = "dependency_unblocked"
	NotificationTypeDependencyResolved    NotificationType = "dependency_resolved"
	NotificationTypeCircularDependency    NotificationType = "circular_dependency"
	NotificationTypeCriticalPathChanged   NotificationType = "critical_path_changed"
)

// Notification represents a system notification
type Notification struct {
	ID        string           `json:"id"`
	Type      NotificationType `json:"type"`
	Title     string           `json:"title"`
	Message   string           `json:"message"`
	IssueID   entities.IssueID `json:"issue_id,omitempty"`
	Recipient string           `json:"recipient"`
	CreatedAt time.Time        `json:"created_at"`
	Read      bool             `json:"read"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// NotificationService provides notification functionality
type NotificationService struct {
	dependencyService *DependencyService
}

// NewNotificationService creates a new notification service
func NewNotificationService(dependencyService *DependencyService) *NotificationService {
	return &NotificationService{
		dependencyService: dependencyService,
	}
}

// NotifyDependencyBlocked sends notification when an issue becomes blocked
func (s *NotificationService) NotifyDependencyBlocked(ctx context.Context, issueID entities.IssueID, blockedBy []entities.IssueID, recipient string) error {
	notification := &Notification{
		ID:        fmt.Sprintf("blocked-%s-%d", issueID, time.Now().Unix()),
		Type:      NotificationTypeDependencyBlocked,
		Title:     fmt.Sprintf("Issue %s is now blocked", issueID),
		Message:   fmt.Sprintf("Issue %s is blocked by: %v", issueID, blockedBy),
		IssueID:   issueID,
		Recipient: recipient,
		CreatedAt: time.Now(),
		Data: map[string]interface{}{
			"blocked_by": blockedBy,
		},
	}

	return s.sendNotification(notification)
}

// NotifyDependencyUnblocked sends notification when an issue becomes unblocked
func (s *NotificationService) NotifyDependencyUnblocked(ctx context.Context, issueID entities.IssueID, recipient string) error {
	notification := &Notification{
		ID:        fmt.Sprintf("unblocked-%s-%d", issueID, time.Now().Unix()),
		Type:      NotificationTypeDependencyUnblocked,
		Title:     fmt.Sprintf("Issue %s is now unblocked", issueID),
		Message:   fmt.Sprintf("Issue %s is no longer blocked and can be worked on", issueID),
		IssueID:   issueID,
		Recipient: recipient,
		CreatedAt: time.Now(),
	}

	return s.sendNotification(notification)
}

// NotifyDependencyResolved sends notification when a dependency is resolved
func (s *NotificationService) NotifyDependencyResolved(ctx context.Context, dependency *entities.Dependency, recipient string) error {
	notification := &Notification{
		ID:        fmt.Sprintf("resolved-%s-%d", dependency.ID, time.Now().Unix()),
		Type:      NotificationTypeDependencyResolved,
		Title:     "Dependency resolved",
		Message:   fmt.Sprintf("Dependency resolved: %s", dependency.String()),
		IssueID:   dependency.SourceID,
		Recipient: recipient,
		CreatedAt: time.Now(),
		Data: map[string]interface{}{
			"dependency_id": dependency.ID,
			"target_issue":  dependency.TargetID,
		},
	}

	return s.sendNotification(notification)
}

// NotifyCircularDependency sends notification about circular dependencies
func (s *NotificationService) NotifyCircularDependency(ctx context.Context, cycle []entities.IssueID, recipient string) error {
	notification := &Notification{
		ID:        fmt.Sprintf("circular-%d", time.Now().Unix()),
		Type:      NotificationTypeCircularDependency,
		Title:     "Circular dependency detected",
		Message:   fmt.Sprintf("Circular dependency detected in issues: %v", cycle),
		Recipient: recipient,
		CreatedAt: time.Now(),
		Data: map[string]interface{}{
			"cycle": cycle,
		},
	}

	return s.sendNotification(notification)
}

// NotifyCriticalPathChanged sends notification when critical path changes
func (s *NotificationService) NotifyCriticalPathChanged(ctx context.Context, issueID entities.IssueID, newPath []entities.IssueID, recipient string) error {
	notification := &Notification{
		ID:        fmt.Sprintf("critical-path-%s-%d", issueID, time.Now().Unix()),
		Type:      NotificationTypeCriticalPathChanged,
		Title:     fmt.Sprintf("Critical path changed for %s", issueID),
		Message:   fmt.Sprintf("New critical path: %v", newPath),
		IssueID:   issueID,
		Recipient: recipient,
		CreatedAt: time.Now(),
		Data: map[string]interface{}{
			"critical_path": newPath,
		},
	}

	return s.sendNotification(notification)
}

// CheckAndNotifyBlockingChanges checks for blocking changes and sends notifications
func (s *NotificationService) CheckAndNotifyBlockingChanges(ctx context.Context, issueID entities.IssueID, recipient string) error {
	blockingInfo, err := s.dependencyService.GetBlockingInfo(ctx, issueID)
	if err != nil {
		return fmt.Errorf("failed to get blocking info: %w", err)
	}

	// Check if issue became blocked
	if blockingInfo.IsBlocked && len(blockingInfo.BlockedBy) > 0 {
		return s.NotifyDependencyBlocked(ctx, issueID, blockingInfo.BlockedBy, recipient)
	}

	// Check if issue became unblocked
	if !blockingInfo.IsBlocked {
		// This would need state tracking to know if it was previously blocked
		// For now, we'll skip this notification
	}

	return nil
}

// ValidateAndNotify validates dependencies and sends notifications about issues
func (s *NotificationService) ValidateAndNotify(ctx context.Context, recipient string) error {
	result, err := s.dependencyService.ValidateDependencyGraph(ctx)
	if err != nil {
		return fmt.Errorf("failed to validate dependency graph: %w", err)
	}

	// Notify about circular dependencies
	for _, cycle := range result.CircularPaths {
		if err := s.NotifyCircularDependency(ctx, cycle, recipient); err != nil {
			log.Printf("Failed to send circular dependency notification: %v", err)
		}
	}

	return nil
}

// sendNotification sends a notification (implementation depends on desired notification method)
func (s *NotificationService) sendNotification(notification *Notification) error {
	// For now, just log the notification
	// In a real implementation, this could send emails, Slack messages, etc.
	log.Printf("NOTIFICATION [%s]: %s - %s", notification.Type, notification.Title, notification.Message)
	
	// Could also write to a file, database, or send via webhook
	return nil
}

// GetNotifications retrieves notifications for a recipient (if we had storage)
func (s *NotificationService) GetNotifications(ctx context.Context, recipient string, limit int) ([]*Notification, error) {
	// This would typically query a notification storage system
	// For now, return empty slice
	return []*Notification{}, nil
}

// MarkNotificationRead marks a notification as read (if we had storage)
func (s *NotificationService) MarkNotificationRead(ctx context.Context, notificationID string) error {
	// This would typically update notification storage
	log.Printf("Marked notification %s as read", notificationID)
	return nil
}