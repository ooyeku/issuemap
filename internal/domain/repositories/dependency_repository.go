package repositories

import (
	"context"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// DependencyFilter defines filtering options for dependencies
type DependencyFilter struct {
	SourceID   *entities.IssueID          `json:"source_id,omitempty"`
	TargetID   *entities.IssueID          `json:"target_id,omitempty"`
	Type       *entities.DependencyType   `json:"type,omitempty"`
	Status     *entities.DependencyStatus `json:"status,omitempty"`
	CreatedBy  *string                    `json:"created_by,omitempty"`
	DateFrom   *time.Time                 `json:"date_from,omitempty"`
	DateTo     *time.Time                 `json:"date_to,omitempty"`
	Limit      int                        `json:"limit,omitempty"`
	Offset     int                        `json:"offset,omitempty"`
}

// DependencyStats represents dependency statistics
type DependencyStats struct {
	TotalDependencies     int                                    `json:"total_dependencies"`
	ActiveDependencies    int                                    `json:"active_dependencies"`
	ResolvedDependencies  int                                    `json:"resolved_dependencies"`
	CircularDependencies  int                                    `json:"circular_dependencies"`
	DependenciesByType    map[entities.DependencyType]int        `json:"dependencies_by_type"`
	DependenciesByStatus  map[entities.DependencyStatus]int      `json:"dependencies_by_status"`
	IssuesWithDeps        int                                    `json:"issues_with_deps"`
	MostBlockedIssues     []entities.IssueID                     `json:"most_blocked_issues"`
	MostBlockingIssues    []entities.IssueID                     `json:"most_blocking_issues"`
	AverageDepPerIssue    float64                                `json:"average_dep_per_issue"`
	DependencyCreators    map[string]int                         `json:"dependency_creators"`
}

// DependencyRepository defines the interface for dependency persistence
type DependencyRepository interface {
	// Create saves a new dependency
	Create(ctx context.Context, dependency *entities.Dependency) error
	
	// GetByID retrieves a dependency by its ID
	GetByID(ctx context.Context, id string) (*entities.Dependency, error)
	
	// Update updates an existing dependency
	Update(ctx context.Context, dependency *entities.Dependency) error
	
	// Delete removes a dependency by its ID
	Delete(ctx context.Context, id string) error
	
	// List retrieves dependencies with optional filtering
	List(ctx context.Context, filter DependencyFilter) ([]*entities.Dependency, error)
	
	// GetByIssueID retrieves all dependencies for a specific issue (as source or target)
	GetByIssueID(ctx context.Context, issueID entities.IssueID) ([]*entities.Dependency, error)
	
	// GetBySourceID retrieves all dependencies where the issue is the source
	GetBySourceID(ctx context.Context, sourceID entities.IssueID) ([]*entities.Dependency, error)
	
	// GetByTargetID retrieves all dependencies where the issue is the target
	GetByTargetID(ctx context.Context, targetID entities.IssueID) ([]*entities.Dependency, error)
	
	// GetActiveDependencies retrieves all active dependencies
	GetActiveDependencies(ctx context.Context) ([]*entities.Dependency, error)
	
	// GetDependencyGraph builds and returns the complete dependency graph
	GetDependencyGraph(ctx context.Context) (*entities.DependencyGraph, error)
	
	// GetStats returns dependency statistics
	GetStats(ctx context.Context, filter DependencyFilter) (*DependencyStats, error)
	
	// BulkUpdate updates multiple dependencies
	BulkUpdate(ctx context.Context, dependencies []*entities.Dependency) error
	
	// DeleteByIssueID removes all dependencies for a specific issue
	DeleteByIssueID(ctx context.Context, issueID entities.IssueID) error
	
	// FindConflicts finds potentially conflicting dependencies
	FindConflicts(ctx context.Context) ([]*entities.Dependency, error)
}