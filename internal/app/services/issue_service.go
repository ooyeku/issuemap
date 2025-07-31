package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

// IssueService provides high-level operations for issue management
type IssueService struct {
	issueRepo      repositories.IssueRepository
	configRepo     repositories.ConfigRepository
	gitRepo        repositories.GitRepository
	historyRepo    repositories.HistoryRepository
	historyService *HistoryService
}

// NewIssueService creates a new issue service
func NewIssueService(
	issueRepo repositories.IssueRepository,
	configRepo repositories.ConfigRepository,
	gitRepo repositories.GitRepository,
) *IssueService {
	// Extract base path for history repository
	// Use the same base path as the issue repository
	var basePath string
	if _, ok := issueRepo.(*storage.FileIssueRepository); ok {
		// Access the basePath - we'll assume the same structure for now
		basePath = app.ConfigDirName // This should match the issueRepo base path
	} else {
		basePath = app.ConfigDirName
	}

	historyRepo := storage.NewFileHistoryRepository(basePath)
	historyService := NewHistoryService(historyRepo, gitRepo)

	return &IssueService{
		issueRepo:      issueRepo,
		configRepo:     configRepo,
		gitRepo:        gitRepo,
		historyRepo:    historyRepo,
		historyService: historyService,
	}
}

// CreateIssueRequest represents a request to create a new issue
type CreateIssueRequest struct {
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Type        entities.IssueType `json:"type"`
	Priority    entities.Priority  `json:"priority"`
	Labels      []string           `json:"labels"`
	Assignee    *string            `json:"assignee,omitempty"`
	Milestone   *string            `json:"milestone,omitempty"`
	Template    *string            `json:"template,omitempty"`
}

// CreateIssue creates a new issue
func (s *IssueService) CreateIssue(ctx context.Context, req CreateIssueRequest) (*entities.Issue, error) {
	// Generate next issue ID
	id, err := s.issueRepo.GetNextID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "IssueService.CreateIssue", "get_next_id")
	}

	// Apply template if specified
	if req.Template != nil {
		template, err := s.configRepo.GetTemplate(ctx, *req.Template)
		if err != nil {
			return nil, errors.Wrap(err, "IssueService.CreateIssue", "get_template")
		}

		// Apply template defaults if not overridden
		if req.Title == "" {
			req.Title = template.Title
		}
		if req.Description == "" {
			req.Description = template.Description
		}
		if req.Type == "" {
			req.Type = template.Type
		}
		if req.Priority == "" {
			req.Priority = template.Priority
		}
		if len(req.Labels) == 0 {
			req.Labels = template.Labels
		}
	}

	// Create the issue
	issue := entities.NewIssue(id, req.Title, req.Description, req.Type)
	issue.Priority = req.Priority

	// Load configuration for labels and assignees
	config, err := s.configRepo.Load(ctx)
	if err != nil {
		// Use default config if not found
		config = entities.NewDefaultConfig()
	}

	// Set labels
	for _, labelName := range req.Labels {
		var label entities.Label
		for _, configLabel := range config.Labels {
			if configLabel.Name == labelName {
				label = configLabel
				break
			}
		}
		if label.Name == "" {
			label = entities.Label{Name: labelName, Color: "#gray"}
		}
		issue.AddLabel(label)
	}

	// Set assignee
	if req.Assignee != nil {
		user := &entities.User{Username: *req.Assignee}
		issue.SetAssignee(user)
	}

	// Set milestone
	if req.Milestone != nil {
		for _, milestone := range config.Milestones {
			if milestone.Name == *req.Milestone {
				issue.SetMilestone(&milestone)
				break
			}
		}
	}

	// Try to get current branch and link issue
	if branch, err := s.gitRepo.GetCurrentBranch(ctx); err == nil {
		issue.Branch = branch
	}

	// Save the issue
	if err := s.issueRepo.Create(ctx, issue); err != nil {
		return nil, errors.Wrap(err, "IssueService.CreateIssue", "save")
	}

	// Record creation in history
	author := "system"
	if s.gitRepo != nil {
		if user, err := s.gitRepo.GetAuthorInfo(ctx); err == nil {
			author = user.Username
		}
	}

	if s.historyService != nil {
		if err := s.historyService.RecordIssueCreated(ctx, issue, author); err != nil {
			// Don't fail the creation if history fails, just log
			fmt.Printf("Warning: Failed to record issue creation in history: %v\n", err)
		}
	}

	return issue, nil
}

// GetIssue retrieves an issue by ID
func (s *IssueService) GetIssue(ctx context.Context, id entities.IssueID) (*entities.Issue, error) {
	issue, err := s.issueRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, "IssueService.GetIssue", "get_by_id")
	}

	// Update issue with latest commits if linked to git
	if issue.Branch != "" {
		commits, err := s.gitRepo.GetCommitsByIssue(ctx, id)
		if err == nil {
			// Update commits in issue
			var commitRefs []entities.CommitRef
			for _, commit := range commits {
				commitRef := entities.CommitRef{
					Hash:    commit.Hash,
					Message: commit.Message,
					Author:  commit.Author,
					Date:    commit.Date,
				}
				commitRefs = append(commitRefs, commitRef)
			}
			issue.Commits = commitRefs
		}
	}

	return issue, nil
}

// UpdateIssue updates an existing issue
func (s *IssueService) UpdateIssue(ctx context.Context, id entities.IssueID, updates map[string]interface{}) (*entities.Issue, error) {
	// Get the existing issue
	issue, err := s.issueRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, "IssueService.UpdateIssue", "get_existing")
	}

	// Create a copy of the original issue for history tracking
	var originalIssue *entities.Issue
	if s.historyService != nil {
		originalIssue = &entities.Issue{
			ID:          issue.ID,
			Title:       issue.Title,
			Description: issue.Description,
			Type:        issue.Type,
			Status:      issue.Status,
			Priority:    issue.Priority,
			Labels:      make([]entities.Label, len(issue.Labels)),
			Assignee:    issue.Assignee,
			Milestone:   issue.Milestone,
			Branch:      issue.Branch,
			Commits:     issue.Commits,
			Comments:    issue.Comments,
			Metadata:    issue.Metadata,
			Timestamps:  issue.Timestamps,
		}
		copy(originalIssue.Labels, issue.Labels)
		if issue.Assignee != nil {
			originalIssue.Assignee = &entities.User{
				Username: issue.Assignee.Username,
				Email:    issue.Assignee.Email,
			}
		}
		if issue.Milestone != nil {
			originalIssue.Milestone = &entities.Milestone{
				Name:        issue.Milestone.Name,
				Description: issue.Milestone.Description,
				DueDate:     issue.Milestone.DueDate,
			}
		}
	}

	// Load configuration for labels and milestones
	config, err := s.configRepo.Load(ctx)
	if err != nil {
		// Use default config if not found
		config = entities.NewDefaultConfig()
	}

	// Apply updates
	for field, value := range updates {
		switch field {
		case "title":
			if title, ok := value.(string); ok {
				issue.Title = title
			}
		case "description":
			if description, ok := value.(string); ok {
				issue.Description = description
			}
		case "type":
			if issueType, ok := value.(string); ok {
				issue.Type = entities.IssueType(issueType)
			}
		case "status":
			if status, ok := value.(string); ok {
				issue.UpdateStatus(entities.Status(status))
			}
		case "priority":
			if priority, ok := value.(string); ok {
				issue.Priority = entities.Priority(priority)
			}
		case "assignee":
			if assignee, ok := value.(string); ok {
				if assignee == "" {
					issue.SetAssignee(nil)
				} else {
					user := &entities.User{Username: assignee}
					issue.SetAssignee(user)
				}
			}
		case "branch":
			if branch, ok := value.(string); ok {
				issue.Branch = branch
			}
		case "labels":
			if labelNames, ok := value.([]string); ok {
				// Clear existing labels
				issue.Labels = []entities.Label{}

				// Add new labels
				for _, labelName := range labelNames {
					var label entities.Label
					// Check if label exists in config
					for _, configLabel := range config.Labels {
						if configLabel.Name == labelName {
							label = configLabel
							break
						}
					}
					// If not found in config, create with default color
					if label.Name == "" {
						label = entities.Label{Name: labelName, Color: "#gray"}
					}
					issue.AddLabel(label)
				}
			}
		case "milestone":
			if milestoneName, ok := value.(string); ok {
				if milestoneName == "" {
					issue.SetMilestone(nil)
				} else {
					// Find milestone in config
					for _, milestone := range config.Milestones {
						if milestone.Name == milestoneName {
							issue.SetMilestone(&milestone)
							break
						}
					}
				}
			}
		}
	}

	// Update timestamps
	issue.Timestamps.Updated = time.Now()

	// Save the updated issue
	if err := s.issueRepo.Update(ctx, issue); err != nil {
		return nil, errors.Wrap(err, "IssueService.UpdateIssue", "save")
	}

	// Record update in history with detailed field changes
	author := "system"
	if s.gitRepo != nil {
		if user, err := s.gitRepo.GetAuthorInfo(ctx); err == nil {
			author = user.Username
		}
	}

	if s.historyService != nil && originalIssue != nil {
		if err := s.historyService.RecordIssueUpdatedWithDetails(ctx, issue.ID, originalIssue, issue, author); err != nil {
			// Don't fail the update if history fails, just log
			fmt.Printf("Warning: Failed to record issue update in history: %v\n", err)
		}
	}

	return issue, nil
}

// ListIssues retrieves issues based on filter criteria
func (s *IssueService) ListIssues(ctx context.Context, filter repositories.IssueFilter) (*repositories.IssueList, error) {
	issueList, err := s.issueRepo.List(ctx, filter)
	if err != nil {
		return nil, errors.Wrap(err, "IssueService.ListIssues", "list")
	}

	return issueList, nil
}

// SearchIssues performs a search across issues
func (s *IssueService) SearchIssues(ctx context.Context, query repositories.SearchQuery) (*repositories.SearchResult, error) {
	result, err := s.issueRepo.Search(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "IssueService.SearchIssues", "search")
	}

	return result, nil
}

// AddComment adds a comment to an issue
func (s *IssueService) AddComment(ctx context.Context, issueID entities.IssueID, author, text string) error {
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return errors.Wrap(err, "IssueService.AddComment", "get_issue")
	}

	issue.AddComment(author, text)

	if err := s.issueRepo.Update(ctx, issue); err != nil {
		return errors.Wrap(err, "IssueService.AddComment", "save")
	}

	return nil
}

// CloseIssue closes an issue
func (s *IssueService) CloseIssue(ctx context.Context, issueID entities.IssueID, reason string) error {
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return errors.Wrap(err, "IssueService.CloseIssue", "get_issue")
	}

	issue.UpdateStatus(entities.StatusClosed)

	if reason != "" {
		issue.AddComment("system", fmt.Sprintf("Issue closed: %s", reason))
	}

	if err := s.issueRepo.Update(ctx, issue); err != nil {
		return errors.Wrap(err, "IssueService.CloseIssue", "save")
	}

	return nil
}

// ReopenIssue reopens a closed issue
func (s *IssueService) ReopenIssue(ctx context.Context, issueID entities.IssueID) error {
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return errors.Wrap(err, "IssueService.ReopenIssue", "get_issue")
	}

	issue.UpdateStatus(entities.StatusOpen)
	issue.AddComment("system", "Issue reopened")

	if err := s.issueRepo.Update(ctx, issue); err != nil {
		return errors.Wrap(err, "IssueService.ReopenIssue", "save")
	}

	return nil
}

// DeleteIssue completely removes an issue and its history
func (s *IssueService) DeleteIssue(ctx context.Context, issueID entities.IssueID) error {
	// First check if the issue exists
	_, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return errors.Wrap(err, "IssueService.DeleteIssue", "get_issue")
	}

	// Delete the issue from the repository
	if err := s.issueRepo.Delete(ctx, issueID); err != nil {
		return errors.Wrap(err, "IssueService.DeleteIssue", "delete_issue")
	}

	// Delete the history if history service is available
	if s.historyService != nil {
		if err := s.historyService.DeleteIssueHistory(ctx, issueID); err != nil {
			// Log warning but don't fail the deletion if history cleanup fails
			fmt.Printf("Warning: Failed to delete issue history: %v\n", err)
		}
	}

	return nil
}

// CreateBranchForIssue creates a git branch for an issue
func (s *IssueService) CreateBranchForIssue(ctx context.Context, issueID entities.IssueID, branchName string) error {
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return errors.Wrap(err, "IssueService.CreateBranchForIssue", "get_issue")
	}

	// Generate branch name if not provided
	if branchName == "" {
		config, _ := s.configRepo.Load(ctx)
		if config == nil {
			config = entities.NewDefaultConfig()
		}

		prefix := config.Git.DefaultBranchPrefix
		if prefix == "" {
			prefix = "feature/"
		}

		// Sanitize issue title for branch name
		sanitizedTitle := sanitizeBranchName(issue.Title)
		branchName = fmt.Sprintf("%s%s-%s", prefix, issueID, sanitizedTitle)
	}

	// Create the branch
	if err := s.gitRepo.CreateBranch(ctx, branchName); err != nil {
		return errors.Wrap(err, "IssueService.CreateBranchForIssue", "create_branch")
	}

	// Update issue with branch information
	issue.Branch = branchName
	if err := s.issueRepo.Update(ctx, issue); err != nil {
		return errors.Wrap(err, "IssueService.CreateBranchForIssue", "update_issue")
	}

	return nil
}

// LinkIssueToBranch links an issue to a git branch
func (s *IssueService) LinkIssueToBranch(ctx context.Context, issueID entities.IssueID, branch string) error {
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return errors.Wrap(err, "IssueService.LinkIssueToBranch", "get_issue")
	}

	issue.Branch = branch
	if err := s.issueRepo.Update(ctx, issue); err != nil {
		return errors.Wrap(err, "IssueService.LinkIssueToBranch", "update_issue")
	}

	return nil
}

// GetProjectStats returns project statistics
func (s *IssueService) GetProjectStats(ctx context.Context) (*repositories.RepositoryStats, error) {
	stats, err := s.issueRepo.GetStats(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "IssueService.GetProjectStats", "get_stats")
	}

	return stats, nil
}

// sanitizeBranchName converts a title to a git-safe branch name
func sanitizeBranchName(title string) string {
	// Convert to lowercase and replace spaces with hyphens
	title = strings.ToLower(title)
	title = strings.ReplaceAll(title, " ", "-")

	// Remove special characters
	reg := regexp.MustCompile(`[^a-z0-9\-]`)
	title = reg.ReplaceAllString(title, "")

	// Limit length
	if len(title) > 50 {
		title = title[:50]
	}

	// Remove trailing hyphens
	title = strings.TrimRight(title, "-")

	return title
}
