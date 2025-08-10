package repositories

import (
	"context"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// IssueFilter represents filter criteria for listing issues
type IssueFilter struct {
	Status       *entities.Status    `json:"status,omitempty"`
	Type         *entities.IssueType `json:"type,omitempty"`
	Priority     *entities.Priority  `json:"priority,omitempty"`
	Assignee     *string             `json:"assignee,omitempty"`
	Labels       []string            `json:"labels,omitempty"`
	Milestone    *string             `json:"milestone,omitempty"`
	Branch       *string             `json:"branch,omitempty"`
	CreatedSince *time.Time          `json:"created_since,omitempty"`
	UpdatedSince *time.Time          `json:"updated_since,omitempty"`
	Limit        *int                `json:"limit,omitempty"`
	Offset       *int                `json:"offset,omitempty"`
}

// SearchQuery represents a search query for issues
type SearchQuery struct {
	Text   string      `json:"text"`
	Filter IssueFilter `json:"filter"`
	Fields []string    `json:"fields,omitempty"` // Fields to search in: title, description, comments
}

// IssueList represents a list of issues with metadata
type IssueList struct {
	Issues []entities.Issue `json:"issues"`
	Total  int              `json:"total"`
	Count  int              `json:"count"`
}

// SearchResult represents search results
type SearchResult struct {
	Issues   []entities.Issue `json:"issues"`
	Total    int              `json:"total"`
	Query    string           `json:"query"`
	Duration string           `json:"duration"`
}

// IssueRepository defines the interface for issue storage operations
type IssueRepository interface {
	// Create creates a new issue
	Create(ctx context.Context, issue *entities.Issue) error

	// GetByID retrieves an issue by its ID
	GetByID(ctx context.Context, id entities.IssueID) (*entities.Issue, error)

	// List retrieves issues based on filter criteria
	List(ctx context.Context, filter IssueFilter) (*IssueList, error)

	// Update updates an existing issue
	Update(ctx context.Context, issue *entities.Issue) error

	// Delete removes an issue
	Delete(ctx context.Context, id entities.IssueID) error

	// Search performs a text search across issues
	Search(ctx context.Context, query SearchQuery) (*SearchResult, error)

	// GetNextID returns the next available issue ID
	GetNextID(ctx context.Context, projectName string) (entities.IssueID, error)

	// Exists checks if an issue with the given ID exists
	Exists(ctx context.Context, id entities.IssueID) (bool, error)

	// ListByStatus retrieves all issues with a specific status
	ListByStatus(ctx context.Context, status entities.Status) ([]*entities.Issue, error)

	// GetStats returns repository statistics
	GetStats(ctx context.Context) (*RepositoryStats, error)
}

// RepositoryStats contains statistics about the issue repository
type RepositoryStats struct {
	TotalIssues      int                        `json:"total_issues"`
	IssuesByStatus   map[entities.Status]int    `json:"issues_by_status"`
	IssuesByType     map[entities.IssueType]int `json:"issues_by_type"`
	IssuesByPriority map[entities.Priority]int  `json:"issues_by_priority"`
	IssuesByAssignee map[string]int             `json:"issues_by_assignee"`
	RecentActivity   []entities.Issue           `json:"recent_activity"`
	OldestIssue      *entities.Issue            `json:"oldest_issue,omitempty"`
	NewestIssue      *entities.Issue            `json:"newest_issue,omitempty"`
}
