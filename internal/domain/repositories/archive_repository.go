package repositories

import (
	"context"
	"io"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// ArchiveRepository defines the interface for archive storage operations
type ArchiveRepository interface {
	// CreateArchive creates a new compressed archive with the given issues
	CreateArchive(ctx context.Context, archiveName string, issues []*entities.Issue) (*entities.ArchiveResult, error)

	// GetArchiveIndex retrieves the current archive index
	GetArchiveIndex(ctx context.Context) (*entities.ArchiveIndex, error)

	// UpdateArchiveIndex updates the archive index
	UpdateArchiveIndex(ctx context.Context, index *entities.ArchiveIndex) error

	// ListArchives returns a list of all archive files
	ListArchives(ctx context.Context) ([]string, error)

	// GetArchiveInfo returns information about a specific archive
	GetArchiveInfo(ctx context.Context, archiveName string) (*entities.ArchiveEntry, error)

	// ExtractIssue extracts a specific issue from an archive
	ExtractIssue(ctx context.Context, issueID entities.IssueID) (*entities.Issue, error)

	// ExtractIssues extracts multiple issues from archives
	ExtractIssues(ctx context.Context, issueIDs []entities.IssueID) ([]*entities.Issue, error)

	// DeleteArchive removes an archive file and updates the index
	DeleteArchive(ctx context.Context, archiveName string) error

	// VerifyArchive checks the integrity of an archive
	VerifyArchive(ctx context.Context, archiveName string) error

	// GetArchiveContent returns a reader for the raw archive content
	GetArchiveContent(ctx context.Context, archiveName string) (io.ReadCloser, error)

	// SearchArchives searches for issues in archives by title/description
	SearchArchives(ctx context.Context, query string) ([]*entities.SearchResult, error)

	// GetArchiveStats returns statistics about all archives
	GetArchiveStats(ctx context.Context) (*entities.ArchiveStats, error)
}

// ArchiveSearchFilter represents criteria for filtering issues to archive
type ArchiveSearchFilter struct {
	// Only include closed issues
	ClosedOnly bool

	// Issues closed before this date
	ClosedBefore *time.Time

	// Issues created before this date
	CreatedBefore *time.Time

	// Minimum age in days
	MinAgeDays int

	// Specific issue IDs
	IssueIDs []entities.IssueID

	// Issue types to include
	Types []entities.IssueType

	// Exclude issues with these statuses
	ExcludeStatuses []entities.Status
}
