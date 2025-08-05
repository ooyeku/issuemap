package repositories

import (
	"context"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// TimeEntryFilter defines filtering options for time entries
type TimeEntryFilter struct {
	IssueID     *entities.IssueID       `json:"issue_id,omitempty"`
	Author      *string                 `json:"author,omitempty"`
	Type        *entities.TimeEntryType `json:"type,omitempty"`
	DateFrom    *time.Time              `json:"date_from,omitempty"`
	DateTo      *time.Time              `json:"date_to,omitempty"`
	MinDuration *time.Duration          `json:"min_duration,omitempty"`
	MaxDuration *time.Duration          `json:"max_duration,omitempty"`
	Limit       int                     `json:"limit,omitempty"`
	Offset      int                     `json:"offset,omitempty"`
}

// TimeEntryStats represents time tracking statistics
type TimeEntryStats struct {
	TotalEntries    int           `json:"total_entries"`
	TotalTime       time.Duration `json:"total_time"`
	AverageTime     time.Duration `json:"average_time"`
	UniqueAuthors   int           `json:"unique_authors"`
	EntriesByType   map[entities.TimeEntryType]int `json:"entries_by_type"`
	TimeByAuthor    map[string]time.Duration `json:"time_by_author"`
	TimeByIssue     map[entities.IssueID]time.Duration `json:"time_by_issue"`
	EntriesByDay    map[string]int `json:"entries_by_day"`
	TimeByDay       map[string]time.Duration `json:"time_by_day"`
}

// ActiveTimerRepository defines the interface for managing active timers
type ActiveTimerRepository interface {
	// Create saves a new active timer
	Create(ctx context.Context, timer *entities.ActiveTimer) error
	
	// GetByAuthor retrieves the active timer for a specific author
	GetByAuthor(ctx context.Context, author string) (*entities.ActiveTimer, error)
	
	// GetByIssueAndAuthor retrieves the active timer for a specific issue and author
	GetByIssueAndAuthor(ctx context.Context, issueID entities.IssueID, author string) (*entities.ActiveTimer, error)
	
	// Delete removes an active timer
	Delete(ctx context.Context, issueID entities.IssueID, author string) error
	
	// List retrieves all active timers
	List(ctx context.Context) ([]*entities.ActiveTimer, error)
}

// TimeEntryRepository defines the interface for time entry persistence
type TimeEntryRepository interface {
	// Create saves a new time entry
	Create(ctx context.Context, entry *entities.TimeEntry) error
	
	// GetByID retrieves a time entry by its ID
	GetByID(ctx context.Context, id string) (*entities.TimeEntry, error)
	
	// Update updates an existing time entry
	Update(ctx context.Context, entry *entities.TimeEntry) error
	
	// Delete removes a time entry by its ID
	Delete(ctx context.Context, id string) error
	
	// List retrieves time entries with optional filtering
	List(ctx context.Context, filter TimeEntryFilter) ([]*entities.TimeEntry, error)
	
	// GetByIssueID retrieves all time entries for a specific issue
	GetByIssueID(ctx context.Context, issueID entities.IssueID) ([]*entities.TimeEntry, error)
	
	// GetByAuthor retrieves all time entries for a specific author
	GetByAuthor(ctx context.Context, author string, filter TimeEntryFilter) ([]*entities.TimeEntry, error)
	
	// GetStats returns time tracking statistics
	GetStats(ctx context.Context, filter TimeEntryFilter) (*TimeEntryStats, error)
	
	// GetTotalTimeByIssue returns the total time logged for each issue
	GetTotalTimeByIssue(ctx context.Context, filter TimeEntryFilter) (map[entities.IssueID]time.Duration, error)
}