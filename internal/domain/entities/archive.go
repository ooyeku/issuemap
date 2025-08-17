package entities

import (
	"time"
)

// ArchiveConfig represents archive management configuration
type ArchiveConfig struct {
	// Enable automatic archiving
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Threshold for archiving closed issues (in days)
	ClosedIssueDays int `yaml:"closed_issue_days" json:"closed_issue_days"`

	// Compression level (0-9, 0=no compression, 9=max compression)
	CompressionLevel int `yaml:"compression_level" json:"compression_level"`

	// Maximum archive size before creating new archive (bytes)
	MaxArchiveSize int64 `yaml:"max_archive_size" json:"max_archive_size"`

	// Archive format: "tar.gz" or "zip"
	Format string `yaml:"format" json:"format"`

	// Verify archive integrity after creation
	VerifyIntegrity bool `yaml:"verify_integrity" json:"verify_integrity"`

	// Include attachments in archives
	IncludeAttachments bool `yaml:"include_attachments" json:"include_attachments"`

	// Include history in archives
	IncludeHistory bool `yaml:"include_history" json:"include_history"`
}

// DefaultArchiveConfig returns default archive settings
func DefaultArchiveConfig() *ArchiveConfig {
	return &ArchiveConfig{
		Enabled:            false,             // Disabled by default
		ClosedIssueDays:    180,               // 6 months
		CompressionLevel:   6,                 // Good balance of compression/speed
		MaxArchiveSize:     100 * 1024 * 1024, // 100MB
		Format:             "tar.gz",
		VerifyIntegrity:    true,
		IncludeAttachments: true,
		IncludeHistory:     true,
	}
}

// ArchiveEntry represents metadata about an archived issue
type ArchiveEntry struct {
	// Issue ID
	IssueID IssueID `yaml:"issue_id" json:"issue_id"`

	// Issue title for quick reference
	Title string `yaml:"title" json:"title"`

	// Issue type and status when archived
	Type   IssueType `yaml:"type" json:"type"`
	Status Status    `yaml:"status" json:"status"`

	// Archive filename containing this issue
	ArchiveFile string `yaml:"archive_file" json:"archive_file"`

	// Original file paths archived
	Files []string `yaml:"files" json:"files"`

	// Size of archived data (compressed)
	CompressedSize int64 `yaml:"compressed_size" json:"compressed_size"`

	// Size of original data (uncompressed)
	OriginalSize int64 `yaml:"original_size" json:"original_size"`

	// When the issue was archived
	ArchivedAt time.Time `yaml:"archived_at" json:"archived_at"`

	// Who archived the issue
	ArchivedBy string `yaml:"archived_by" json:"archived_by"`

	// Issue timestamps for reference
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
	ClosedAt  time.Time `yaml:"closed_at" json:"closed_at"`

	// Checksum for integrity verification
	Checksum string `yaml:"checksum" json:"checksum"`
}

// ArchiveIndex represents the master index of all archives
type ArchiveIndex struct {
	// Index version for compatibility
	Version int `yaml:"version" json:"version"`

	// When the index was last updated
	LastUpdated time.Time `yaml:"last_updated" json:"last_updated"`

	// Total number of archived issues
	TotalIssues int `yaml:"total_issues" json:"total_issues"`

	// Total compressed size of all archives
	TotalCompressedSize int64 `yaml:"total_compressed_size" json:"total_compressed_size"`

	// Total original size before compression
	TotalOriginalSize int64 `yaml:"total_original_size" json:"total_original_size"`

	// Map of issue ID to archive entry
	Issues map[IssueID]*ArchiveEntry `yaml:"issues" json:"issues"`

	// Map of archive file to list of issue IDs
	Archives map[string][]IssueID `yaml:"archives" json:"archives"`
}

// NewArchiveIndex creates a new empty archive index
func NewArchiveIndex() *ArchiveIndex {
	return &ArchiveIndex{
		Version:             1,
		LastUpdated:         time.Now(),
		TotalIssues:         0,
		TotalCompressedSize: 0,
		TotalOriginalSize:   0,
		Issues:              make(map[IssueID]*ArchiveEntry),
		Archives:            make(map[string][]IssueID),
	}
}

// ArchiveResult represents the result of an archive operation
type ArchiveResult struct {
	// Operation timestamp
	Timestamp time.Time `json:"timestamp"`

	// Was this a dry run?
	DryRun bool `json:"dry_run"`

	// Archive file created
	ArchiveFile string `json:"archive_file,omitempty"`

	// Number of issues archived
	IssuesArchived int `json:"issues_archived"`

	// Size before compression
	OriginalSize int64 `json:"original_size"`

	// Size after compression
	CompressedSize int64 `json:"compressed_size"`

	// Compression ratio (0.0 to 1.0)
	CompressionRatio float64 `json:"compression_ratio"`

	// List of archived issue IDs
	ArchivedIssues []IssueID `json:"archived_issues"`

	// Errors encountered
	Errors []string `json:"errors"`

	// Duration of operation
	Duration time.Duration `json:"duration"`
}

// ArchiveRestoreResult represents the result of a restore operation
type ArchiveRestoreResult struct {
	// Operation timestamp
	Timestamp time.Time `json:"timestamp"`

	// Was this a dry run?
	DryRun bool `json:"dry_run"`

	// Number of issues restored
	IssuesRestored int `json:"issues_restored"`

	// List of restored issue IDs
	RestoredIssues []IssueID `json:"restored_issues"`

	// Errors encountered
	Errors []string `json:"errors"`

	// Duration of operation
	Duration time.Duration `json:"duration"`
}

// ArchiveStats represents statistics about archives
type ArchiveStats struct {
	// Total number of archives
	TotalArchives int `json:"total_archives"`

	// Total number of archived issues
	TotalArchivedIssues int `json:"total_archived_issues"`

	// Total compressed size
	TotalCompressedSize int64 `json:"total_compressed_size"`

	// Total original size
	TotalOriginalSize int64 `json:"total_original_size"`

	// Overall compression ratio
	CompressionRatio float64 `json:"compression_ratio"`

	// Space saved through archiving
	SpaceSaved int64 `json:"space_saved"`

	// Oldest archived issue
	OldestIssue *time.Time `json:"oldest_issue,omitempty"`

	// Most recent archived issue
	NewestIssue *time.Time `json:"newest_issue,omitempty"`

	// Archives by month/year
	ArchivesByPeriod map[string]int `json:"archives_by_period"`
}

// ArchiveFilter represents filter criteria for archive operations
type ArchiveFilter struct {
	// Filter by issue status
	Status *Status `json:"status,omitempty"`

	// Filter by issue type
	Type *IssueType `json:"type,omitempty"`

	// Issues closed before this date
	ClosedBefore *time.Time `json:"closed_before,omitempty"`

	// Issues created before this date
	CreatedBefore *time.Time `json:"created_before,omitempty"`

	// Minimum age in days
	MinAgeDays *int `json:"min_age_days,omitempty"`

	// Specific issue IDs to include
	IssueIDs []IssueID `json:"issue_ids,omitempty"`

	// Exclude specific issue IDs
	ExcludeIDs []IssueID `json:"exclude_ids,omitempty"`
}

// SearchResult represents search results in archives
type SearchResult struct {
	// Archive entry found
	Entry *ArchiveEntry `json:"entry"`

	// Archive file containing the issue
	ArchiveFile string `json:"archive_file"`

	// Match score (0.0 to 1.0)
	Score float64 `json:"score"`

	// Matched fields
	MatchedFields []string `json:"matched_fields"`
}
