package repositories

import (
	"context"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// HistoryFilter represents filter criteria for listing history entries
type HistoryFilter struct {
	IssueID    *entities.IssueID    `json:"issue_id,omitempty"`
	Author     *string              `json:"author,omitempty"`
	ChangeType *entities.ChangeType `json:"change_type,omitempty"`
	Since      *time.Time           `json:"since,omitempty"`
	Until      *time.Time           `json:"until,omitempty"`
	Field      *string              `json:"field,omitempty"`
	Limit      *int                 `json:"limit,omitempty"`
	Offset     *int                 `json:"offset,omitempty"`
}

// HistoryList represents a list of history entries with metadata
type HistoryList struct {
	Entries []entities.HistoryEntry `json:"entries"`
	Total   int                     `json:"total"`
	Count   int                     `json:"count"`
}

// HistoryRepository defines the interface for history storage operations
type HistoryRepository interface {
	// CreateHistory creates a new issue history
	CreateHistory(ctx context.Context, history *entities.IssueHistory) error

	// GetHistory retrieves the complete history for an issue
	GetHistory(ctx context.Context, issueID entities.IssueID) (*entities.IssueHistory, error)

	// AddEntry adds a new history entry to an issue's history
	AddEntry(ctx context.Context, entry *entities.HistoryEntry) error

	// GetEntry retrieves a specific history entry by ID
	GetEntry(ctx context.Context, entryID string) (*entities.HistoryEntry, error)

	// ListEntries retrieves history entries based on filter criteria
	ListEntries(ctx context.Context, filter HistoryFilter) (*HistoryList, error)

	// GetAllHistory retrieves all issue histories (for global history view)
	GetAllHistory(ctx context.Context, filter HistoryFilter) ([]*entities.IssueHistory, error)

	// DeleteHistory removes all history for an issue
	DeleteHistory(ctx context.Context, issueID entities.IssueID) error

	// GetHistoryStats returns statistics across all histories
	GetHistoryStats(ctx context.Context, filter HistoryFilter) (*HistoryStats, error)

	// GetHistoryByDateRange returns all changes within a date range
	GetHistoryByDateRange(ctx context.Context, since, until time.Time) ([]*entities.HistoryEntry, error)
}

// HistoryStats contains statistics about issue histories
type HistoryStats struct {
	TotalIssuesWithHistory int                         `json:"total_issues_with_history"`
	TotalHistoryEntries    int                         `json:"total_history_entries"`
	EntriesByType          map[entities.ChangeType]int `json:"entries_by_type"`
	EntriesByAuthor        map[string]int              `json:"entries_by_author"`
	MostActiveIssue        *entities.IssueID           `json:"most_active_issue,omitempty"`
	MostActiveAuthor       string                      `json:"most_active_author"`
	ActivityByDay          map[string]int              `json:"activity_by_day"`
	AverageChangesPerIssue float64                     `json:"average_changes_per_issue"`
}
