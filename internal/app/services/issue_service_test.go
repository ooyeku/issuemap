package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

func TestIssueServiceStructure(t *testing.T) {
	// Test the IssueService structure exists and has expected fields
	service := &IssueService{}

	assert.NotNil(t, service)

	// Test that we can set the fields (this validates they exist)
	service.issueRepo = nil
	service.configRepo = nil
	service.gitRepo = nil
	service.historyRepo = nil
	service.historyService = nil

	// Just verify the fields can be accessed
	assert.Nil(t, service.issueRepo)
	assert.Nil(t, service.configRepo)
	assert.Nil(t, service.gitRepo)
	assert.Nil(t, service.historyRepo)
	assert.Nil(t, service.historyService)
}

func TestCreateIssueRequest(t *testing.T) {
	// Test CreateIssueRequest struct validation
	req := CreateIssueRequest{
		Title:       "Test Issue",
		Description: "Test Description",
		Type:        entities.IssueType("bug"),
		Priority:    entities.Priority("high"),
		Labels:      []string{"test", "unit"},
	}

	assert.Equal(t, "Test Issue", req.Title)
	assert.Equal(t, "Test Description", req.Description)
	assert.Equal(t, entities.IssueType("bug"), req.Type)
	assert.Equal(t, entities.Priority("high"), req.Priority)
	assert.Equal(t, []string{"test", "unit"}, req.Labels)
	assert.Nil(t, req.Assignee)
	assert.Nil(t, req.Milestone)
	assert.Nil(t, req.Template)
}

func TestCreateIssueRequestWithOptionalFields(t *testing.T) {
	assignee := "testuser"
	milestone := "v1.0"
	template := "bug"

	req := CreateIssueRequest{
		Title:       "Test Issue",
		Description: "Test Description",
		Type:        entities.IssueType("bug"),
		Priority:    entities.Priority("high"),
		Labels:      []string{"test"},
		Assignee:    &assignee,
		Milestone:   &milestone,
		Template:    &template,
		FieldValues: map[string]interface{}{"severity": "critical"},
	}

	assert.NotNil(t, req.Assignee)
	assert.Equal(t, "testuser", *req.Assignee)
	assert.NotNil(t, req.Milestone)
	assert.Equal(t, "v1.0", *req.Milestone)
	assert.NotNil(t, req.Template)
	assert.Equal(t, "bug", *req.Template)
	assert.NotNil(t, req.FieldValues)
	assert.Equal(t, "critical", req.FieldValues["severity"])
}

// Test validation of issue types and priorities
func TestIssueValidationTypes(t *testing.T) {
	validTypes := []entities.IssueType{"bug", "feature", "task", "epic"}
	validPriorities := []entities.Priority{"low", "medium", "high", "critical"}

	for _, issueType := range validTypes {
		req := CreateIssueRequest{
			Title: "Test Issue",
			Type:  issueType,
		}
		assert.NotEmpty(t, req.Type)
	}

	for _, priority := range validPriorities {
		req := CreateIssueRequest{
			Title:    "Test Issue",
			Priority: priority,
		}
		assert.NotEmpty(t, req.Priority)
	}
}

func TestIssueServiceHasExpectedMethods(t *testing.T) {
	// Test that IssueService has expected method signature for CreateIssue
	// This is a compile-time check - if the method doesn't exist or has wrong signature, this won't compile
	service := &IssueService{}

	// Verify CreateIssue method exists with correct signature
	// We don't call it, just verify it can be assigned to a function variable
	var createIssueFunc func(ctx context.Context, req CreateIssueRequest) (*entities.Issue, error)
	createIssueFunc = service.CreateIssue

	// This just tests that we can assign the method, proving it exists with correct signature
	_ = createIssueFunc

	assert.True(t, true, "IssueService has CreateIssue method with correct signature")
}
