package services

import (
	"context"
	"fmt"
	"log"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// WorkflowAction represents different workflow actions
type WorkflowAction string

const (
	WorkflowActionAutoResolve     WorkflowAction = "auto_resolve"
	WorkflowActionNotifyBlocked   WorkflowAction = "notify_blocked"
	WorkflowActionValidateGraph   WorkflowAction = "validate_graph"
	WorkflowActionUpdateCritical  WorkflowAction = "update_critical_path"
)

// WorkflowRule represents a rule for automatic dependency management
type WorkflowRule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Trigger     string         `json:"trigger"`     // e.g., "issue_completed", "dependency_created"
	Action      WorkflowAction `json:"action"`
	Enabled     bool           `json:"enabled"`
	Conditions  []string       `json:"conditions,omitempty"`
}

// WorkflowService provides dependency workflow automation
type WorkflowService struct {
	dependencyService   *DependencyService
	issueService        *IssueService
	notificationService *NotificationService
	rules               []*WorkflowRule
}

// NewWorkflowService creates a new workflow service
func NewWorkflowService(
	dependencyService *DependencyService,
	issueService *IssueService,
	notificationService *NotificationService,
) *WorkflowService {
	service := &WorkflowService{
		dependencyService:   dependencyService,
		issueService:        issueService,
		notificationService: notificationService,
	}
	
	// Initialize default rules
	service.initializeDefaultRules()
	
	return service
}

// initializeDefaultRules sets up default workflow rules
func (s *WorkflowService) initializeDefaultRules() {
	s.rules = []*WorkflowRule{
		{
			ID:          "auto-resolve-on-completion",
			Name:        "Auto-resolve dependencies on issue completion",
			Description: "Automatically resolve dependencies when target issues are completed",
			Trigger:     "issue_completed",
			Action:      WorkflowActionAutoResolve,
			Enabled:     true,
		},
		{
			ID:          "notify-on-blocked",
			Name:        "Notify when issue becomes blocked",
			Description: "Send notifications when an issue becomes blocked by dependencies",
			Trigger:     "dependency_created",
			Action:      WorkflowActionNotifyBlocked,
			Enabled:     false, // Disabled by default to avoid spam
		},
		{
			ID:          "validate-on-change",
			Name:        "Validate graph on dependency changes",
			Description: "Validate dependency graph for circular dependencies when changes are made",
			Trigger:     "dependency_created",
			Action:      WorkflowActionValidateGraph,
			Enabled:     true,
		},
	}
}

// OnIssueCompleted handles workflow actions when an issue is completed
func (s *WorkflowService) OnIssueCompleted(ctx context.Context, issueID entities.IssueID, completedBy string) error {
	log.Printf("Running workflows for completed issue: %s", issueID)

	// Execute all enabled rules triggered by issue completion
	for _, rule := range s.rules {
		if !rule.Enabled || rule.Trigger != "issue_completed" {
			continue
		}

		if err := s.executeRule(ctx, rule, issueID, completedBy); err != nil {
			log.Printf("Failed to execute workflow rule %s: %v", rule.ID, err)
			// Continue with other rules even if one fails
		}
	}

	return nil
}

// OnDependencyCreated handles workflow actions when a dependency is created
func (s *WorkflowService) OnDependencyCreated(ctx context.Context, dependency *entities.Dependency, createdBy string) error {
	log.Printf("Running workflows for created dependency: %s", dependency.ID)

	// Execute all enabled rules triggered by dependency creation
	for _, rule := range s.rules {
		if !rule.Enabled || rule.Trigger != "dependency_created" {
			continue
		}

		if err := s.executeRuleForDependency(ctx, rule, dependency, createdBy); err != nil {
			log.Printf("Failed to execute workflow rule %s: %v", rule.ID, err)
			// Continue with other rules even if one fails
		}
	}

	return nil
}

// OnDependencyResolved handles workflow actions when a dependency is resolved
func (s *WorkflowService) OnDependencyResolved(ctx context.Context, dependency *entities.Dependency, resolvedBy string) error {
	log.Printf("Running workflows for resolved dependency: %s", dependency.ID)

	// Check if any issues became unblocked
	if s.notificationService != nil {
		// Check both source and target issues for blocking changes
		for _, issueID := range []entities.IssueID{dependency.SourceID, dependency.TargetID} {
			if err := s.notificationService.CheckAndNotifyBlockingChanges(ctx, issueID, resolvedBy); err != nil {
				log.Printf("Failed to check blocking changes for %s: %v", issueID, err)
			}
		}
	}

	return nil
}

// executeRule executes a workflow rule for a specific issue
func (s *WorkflowService) executeRule(ctx context.Context, rule *WorkflowRule, issueID entities.IssueID, actor string) error {
	switch rule.Action {
	case WorkflowActionAutoResolve:
		return s.autoResolveDependencies(ctx, issueID, actor)
		
	case WorkflowActionNotifyBlocked:
		return s.notifyIfBlocked(ctx, issueID, actor)
		
	case WorkflowActionValidateGraph:
		return s.validateDependencyGraph(ctx, actor)
		
	case WorkflowActionUpdateCritical:
		return s.updateCriticalPath(ctx, issueID, actor)
		
	default:
		return fmt.Errorf("unknown workflow action: %s", rule.Action)
	}
}

// executeRuleForDependency executes a workflow rule for a specific dependency
func (s *WorkflowService) executeRuleForDependency(ctx context.Context, rule *WorkflowRule, dependency *entities.Dependency, actor string) error {
	switch rule.Action {
	case WorkflowActionNotifyBlocked:
		// Check if any issues became blocked due to this dependency
		for _, issueID := range []entities.IssueID{dependency.SourceID, dependency.TargetID} {
			if err := s.notifyIfBlocked(ctx, issueID, actor); err != nil {
				log.Printf("Failed to check if %s is blocked: %v", issueID, err)
			}
		}
		return nil
		
	case WorkflowActionValidateGraph:
		return s.validateDependencyGraph(ctx, actor)
		
	default:
		return fmt.Errorf("workflow action %s not supported for dependency events", rule.Action)
	}
}

// autoResolveDependencies automatically resolves dependencies when target issues are completed
func (s *WorkflowService) autoResolveDependencies(ctx context.Context, completedIssueID entities.IssueID, resolvedBy string) error {
	if s.dependencyService == nil {
		return fmt.Errorf("dependency service not available")
	}

	return s.dependencyService.AutoResolveDependencies(ctx, completedIssueID, resolvedBy)
}

// notifyIfBlocked checks if an issue is blocked and sends notifications
func (s *WorkflowService) notifyIfBlocked(ctx context.Context, issueID entities.IssueID, recipient string) error {
	if s.notificationService == nil {
		return nil // No notification service available
	}

	return s.notificationService.CheckAndNotifyBlockingChanges(ctx, issueID, recipient)
}

// validateDependencyGraph validates the dependency graph and notifies about issues
func (s *WorkflowService) validateDependencyGraph(ctx context.Context, recipient string) error {
	if s.notificationService == nil {
		return nil // No notification service available
	}

	return s.notificationService.ValidateAndNotify(ctx, recipient)
}

// updateCriticalPath updates critical path information for an issue
func (s *WorkflowService) updateCriticalPath(ctx context.Context, issueID entities.IssueID, actor string) error {
	if s.dependencyService == nil || s.notificationService == nil {
		return nil
	}

	analysis, err := s.dependencyService.AnalyzeDependencyImpact(ctx, issueID)
	if err != nil {
		return fmt.Errorf("failed to analyze dependency impact: %w", err)
	}

	if len(analysis.CriticalPath) > 0 {
		return s.notificationService.NotifyCriticalPathChanged(ctx, issueID, analysis.CriticalPath, actor)
	}

	return nil
}

// GetWorkflowRules returns all workflow rules
func (s *WorkflowService) GetWorkflowRules() []*WorkflowRule {
	return s.rules
}

// EnableRule enables a workflow rule
func (s *WorkflowService) EnableRule(ruleID string) error {
	for _, rule := range s.rules {
		if rule.ID == ruleID {
			rule.Enabled = true
			log.Printf("Enabled workflow rule: %s", rule.Name)
			return nil
		}
	}
	return fmt.Errorf("workflow rule not found: %s", ruleID)
}

// DisableRule disables a workflow rule
func (s *WorkflowService) DisableRule(ruleID string) error {
	for _, rule := range s.rules {
		if rule.ID == ruleID {
			rule.Enabled = false
			log.Printf("Disabled workflow rule: %s", rule.Name)
			return nil
		}
	}
	return fmt.Errorf("workflow rule not found: %s", ruleID)
}

// AddCustomRule adds a custom workflow rule
func (s *WorkflowService) AddCustomRule(rule *WorkflowRule) error {
	// Validate rule
	if rule.ID == "" || rule.Name == "" || rule.Trigger == "" {
		return fmt.Errorf("rule must have ID, name, and trigger")
	}

	// Check for duplicate ID
	for _, existing := range s.rules {
		if existing.ID == rule.ID {
			return fmt.Errorf("rule with ID %s already exists", rule.ID)
		}
	}

	s.rules = append(s.rules, rule)
	log.Printf("Added custom workflow rule: %s", rule.Name)
	return nil
}

// RemoveRule removes a workflow rule
func (s *WorkflowService) RemoveRule(ruleID string) error {
	for i, rule := range s.rules {
		if rule.ID == ruleID {
			s.rules = append(s.rules[:i], s.rules[i+1:]...)
			log.Printf("Removed workflow rule: %s", rule.Name)
			return nil
		}
	}
	return fmt.Errorf("workflow rule not found: %s", ruleID)
}

// ProcessIssueStatusChange processes workflow rules when an issue status changes
func (s *WorkflowService) ProcessIssueStatusChange(ctx context.Context, issueID entities.IssueID, oldStatus, newStatus entities.Status, changedBy string) error {
	// If issue was completed, run completion workflows
	if newStatus == entities.StatusDone || newStatus == entities.StatusClosed {
		return s.OnIssueCompleted(ctx, issueID, changedBy)
	}

	return nil
}