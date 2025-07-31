package services

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
)

// AutomationService handles template automation and rules processing
type AutomationService struct {
	issueService *IssueService
}

// NewAutomationService creates a new automation service
func NewAutomationService(issueService *IssueService) *AutomationService {
	return &AutomationService{
		issueService: issueService,
	}
}

// ProcessTemplateAutomation applies automation rules from a template to an issue
func (s *AutomationService) ProcessTemplateAutomation(ctx context.Context, issue *entities.Issue, template *entities.Template, fieldValues map[string]interface{}) error {
	// Check if automation is empty
	if s.isAutomationEmpty(template.Automation) {
		return nil // No automation rules
	}

	// Process label rules
	if err := s.processLabelRules(ctx, issue, template.Automation.LabelRules, fieldValues); err != nil {
		return errors.Wrap(err, "AutomationService.ProcessTemplateAutomation", "label_rules")
	}

	// Process assignee rules
	if err := s.processAssigneeRules(ctx, issue, template.Automation.AssigneeRules, fieldValues); err != nil {
		return errors.Wrap(err, "AutomationService.ProcessTemplateAutomation", "assignee_rules")
	}

	// Process workflow rules
	if err := s.processWorkflowRules(ctx, issue, template.Automation.WorkflowRules, fieldValues); err != nil {
		return errors.Wrap(err, "AutomationService.ProcessTemplateAutomation", "workflow_rules")
	}

	// Process notification rules
	if err := s.processNotificationRules(ctx, issue, template.Automation.NotificationRules, fieldValues); err != nil {
		return errors.Wrap(err, "AutomationService.ProcessTemplateAutomation", "notification_rules")
	}

	return nil
}

// ValidateTemplateFields validates field values against template field definitions
func (s *AutomationService) ValidateTemplateFields(template *entities.Template, fieldValues map[string]interface{}) error {
	for _, field := range template.Fields {
		value, exists := fieldValues[field.Name]

		// Check required fields
		if field.Required && (!exists || value == "") {
			return fmt.Errorf("field '%s' (%s) is required", field.Name, field.Label)
		}

		if !exists || value == "" {
			continue // Skip validation for empty optional fields
		}

		// Validate field value
		if err := s.validateFieldValue(field, value); err != nil {
			return fmt.Errorf("field '%s' validation failed: %w", field.Name, err)
		}
	}

	return nil
}

// validateFieldValue validates a single field value against its definition
func (s *AutomationService) validateFieldValue(field entities.TemplateField, value interface{}) error {
	strValue := fmt.Sprintf("%v", value)

	// Type-specific validation
	switch field.Type {
	case entities.FieldTypeSelect:
		if len(field.Options) > 0 {
			valid := false
			for _, option := range field.Options {
				if strValue == option {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("value '%s' is not a valid option (must be one of: %s)", strValue, strings.Join(field.Options, ", "))
			}
		}
	case entities.FieldTypeNumber:
		if _, err := strconv.ParseFloat(strValue, 64); err != nil {
			return fmt.Errorf("value '%s' is not a valid number", strValue)
		}
	case entities.FieldTypeURL:
		urlPattern := `^https?://[^\s]+$`
		if matched, _ := regexp.MatchString(urlPattern, strValue); !matched {
			return fmt.Errorf("value '%s' is not a valid URL", strValue)
		}
	case entities.FieldTypeEmail:
		emailPattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
		if matched, _ := regexp.MatchString(emailPattern, strValue); !matched {
			return fmt.Errorf("value '%s' is not a valid email address", strValue)
		}
	}

	// Custom validation rules
	if field.Validation != nil {
		if field.Validation.MinLength > 0 && len(strValue) < field.Validation.MinLength {
			message := field.Validation.Message
			if message == "" {
				message = fmt.Sprintf("must be at least %d characters", field.Validation.MinLength)
			}
			return fmt.Errorf("%s", message)
		}

		if field.Validation.MaxLength > 0 && len(strValue) > field.Validation.MaxLength {
			message := field.Validation.Message
			if message == "" {
				message = fmt.Sprintf("must be no more than %d characters", field.Validation.MaxLength)
			}
			return fmt.Errorf("%s", message)
		}

		if field.Validation.Pattern != "" {
			if matched, err := regexp.MatchString(field.Validation.Pattern, strValue); err != nil {
				return fmt.Errorf("pattern validation error: %w", err)
			} else if !matched {
				message := field.Validation.Message
				if message == "" {
					message = "does not match required pattern"
				}
				return fmt.Errorf("%s", message)
			}
		}
	}

	return nil
}

// processLabelRules applies label automation rules
func (s *AutomationService) processLabelRules(ctx context.Context, issue *entities.Issue, rules []entities.LabelRule, fieldValues map[string]interface{}) error {
	for _, rule := range rules {
		if s.evaluateCondition(rule.Condition, fieldValues, issue) {
			for _, labelName := range rule.Labels {
				// Add label if not already present
				hasLabel := false
				for _, existingLabel := range issue.Labels {
					if existingLabel.Name == labelName {
						hasLabel = true
						break
					}
				}
				if !hasLabel {
					label := entities.Label{Name: labelName, Color: "#gray"} // Default color
					issue.AddLabel(label)
				}
			}
		}
	}
	return nil
}

// processAssigneeRules applies assignee automation rules
func (s *AutomationService) processAssigneeRules(ctx context.Context, issue *entities.Issue, rules []entities.AssignmentRule, fieldValues map[string]interface{}) error {
	for _, rule := range rules {
		if s.evaluateCondition(rule.Condition, fieldValues, issue) {
			user := &entities.User{Username: rule.Assignee}
			issue.SetAssignee(user)
			break // Only apply first matching rule
		}
	}
	return nil
}

// processWorkflowRules applies workflow automation rules
func (s *AutomationService) processWorkflowRules(ctx context.Context, issue *entities.Issue, rules []entities.WorkflowRule, fieldValues map[string]interface{}) error {
	for _, rule := range rules {
		triggerMatches := rule.Trigger == "on_create" // For now, only support on_create
		conditionMatches := rule.Condition == "" || s.evaluateCondition(rule.Condition, fieldValues, issue)

		if triggerMatches && conditionMatches {
			issue.UpdateStatus(rule.NewStatus)
		}
	}
	return nil
}

// processNotificationRules applies notification automation rules
func (s *AutomationService) processNotificationRules(ctx context.Context, issue *entities.Issue, rules []entities.NotifyRule, fieldValues map[string]interface{}) error {
	// For now, just log notifications (in a real implementation, you'd send actual notifications)
	for _, rule := range rules {
		if rule.Event == "on_create" {
			fmt.Printf("Notification: Issue %s created, notifying: %s\n", issue.ID, strings.Join(rule.Recipients, ", "))
			if rule.Message != "" {
				fmt.Printf("Message: %s\n", rule.Message)
			}
		}
	}
	return nil
}

// evaluateCondition evaluates a simple condition string against field values and issue data
func (s *AutomationService) evaluateCondition(condition string, fieldValues map[string]interface{}, issue *entities.Issue) bool {
	if condition == "" {
		return true
	}

	// Simple condition evaluation (field == value, field != value)
	// In a production system, you'd want a proper expression evaluator

	// Handle equality conditions
	if strings.Contains(condition, "==") {
		parts := strings.Split(condition, "==")
		if len(parts) != 2 {
			return false
		}

		field := strings.TrimSpace(parts[0])
		expectedValue := strings.Trim(strings.TrimSpace(parts[1]), "'\"")

		actualValue := s.getConditionValue(field, fieldValues, issue)
		return actualValue == expectedValue
	}

	// Handle inequality conditions
	if strings.Contains(condition, "!=") {
		parts := strings.Split(condition, "!=")
		if len(parts) != 2 {
			return false
		}

		field := strings.TrimSpace(parts[0])
		expectedValue := strings.Trim(strings.TrimSpace(parts[1]), "'\"")

		actualValue := s.getConditionValue(field, fieldValues, issue)
		return actualValue != expectedValue
	}

	return false
}

// getConditionValue gets the value for a field name from field values or issue properties
func (s *AutomationService) getConditionValue(field string, fieldValues map[string]interface{}, issue *entities.Issue) string {
	// Check field values first
	if value, exists := fieldValues[field]; exists {
		return fmt.Sprintf("%v", value)
	}

	// Check issue properties
	switch field {
	case "type":
		return string(issue.Type)
	case "priority":
		return string(issue.Priority)
	case "status":
		return string(issue.Status)
	case "title":
		return issue.Title
	default:
		return ""
	}
}

// isAutomationEmpty checks if automation config is empty
func (s *AutomationService) isAutomationEmpty(automation entities.TemplateAuto) bool {
	return len(automation.AssigneeRules) == 0 &&
		len(automation.LabelRules) == 0 &&
		len(automation.WorkflowRules) == 0 &&
		len(automation.NotificationRules) == 0
}
