package repositories

import (
	"context"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// GlobalRepository defines operations for global issuemap management
type GlobalRepository interface {
	// Config operations
	GetConfig(ctx context.Context) (*entities.GlobalConfig, error)
	SaveConfig(ctx context.Context, config *entities.GlobalConfig) error
	InitializeGlobal(ctx context.Context) error

	// Project management
	RegisterProject(ctx context.Context, path string, name string) (*entities.ProjectInfo, error)
	UnregisterProject(ctx context.Context, path string) error
	GetProject(ctx context.Context, path string) (*entities.ProjectInfo, error)
	ListProjects(ctx context.Context, filter ProjectFilter) ([]*entities.ProjectInfo, error)
	UpdateProjectStats(ctx context.Context, path string, stats *ProjectStats) error
	ScanForProjects(ctx context.Context, rootPaths []string) ([]*entities.ProjectInfo, error)

	// Archive operations
	ArchiveIssue(ctx context.Context, issue *entities.Issue, projectPath string, reason string) (*entities.ArchivedIssue, error)
	GetArchivedIssue(ctx context.Context, issueID entities.IssueID, projectPath string) (*entities.ArchivedIssue, error)
	ListArchivedIssues(ctx context.Context, filter ArchiveFilter) ([]*entities.ArchivedIssue, error)
	RestoreArchivedIssue(ctx context.Context, archivedIssue *entities.ArchivedIssue, targetPath string) error
	DeleteArchivedIssue(ctx context.Context, issueID entities.IssueID, projectPath string) error

	// Backup operations
	CreateBackup(ctx context.Context, projectPath string, metadata *entities.BackupMetadata) (*entities.ProjectBackup, error)
	ListBackups(ctx context.Context, filter BackupFilter) ([]*entities.ProjectBackup, error)
	GetBackup(ctx context.Context, backupID string) (*entities.ProjectBackup, error)
	RestoreBackup(ctx context.Context, backupID string, targetPath string) error
	DeleteBackup(ctx context.Context, backupID string) error

	// Global search and listing
	GlobalListIssues(ctx context.Context, filter GlobalIssueFilter) (*GlobalIssueList, error)
	GlobalSearchIssues(ctx context.Context, query GlobalSearchQuery) (*GlobalSearchResult, error)
	GetGlobalStats(ctx context.Context) (*GlobalStats, error)

	// Maintenance operations
	CleanupOrphanedArchives(ctx context.Context) error
	ValidateIntegrity(ctx context.Context) (*IntegrityReport, error)
	ExportData(ctx context.Context, format string, outputPath string) error
	ImportData(ctx context.Context, inputPath string) error
}

// ProjectFilter defines criteria for filtering projects
type ProjectFilter struct {
	Status       *entities.ProjectStatus
	Tags         []string
	Name         *string
	LastScanDays *int
	HasBackup    *bool
}

// ProjectStats contains project statistics
type ProjectStats struct {
	IssueCount    int
	ArchivedCount int
	LastScan      time.Time
}

// ArchiveFilter defines criteria for filtering archived issues
type ArchiveFilter struct {
	ProjectPath   *string
	ArchivedSince *time.Time
	ArchivedBy    *string
	IssueType     *entities.IssueType
	Status        *entities.Status
	Priority      *entities.Priority
	Limit         *int
	Offset        *int
}

// BackupFilter defines criteria for filtering backups
type BackupFilter struct {
	ProjectPath  *string
	CreatedSince *time.Time
	CreatedBy    *string
	MinSize      *int64
	MaxSize      *int64
	Tags         []string
	Limit        *int
	Offset       *int
}

// GlobalIssueFilter combines local and global filtering criteria
type GlobalIssueFilter struct {
	IssueFilter     // Embed local issue filter
	ProjectPaths    []string
	IncludeArchived bool
}

// GlobalSearchQuery extends search capabilities across projects
type GlobalSearchQuery struct {
	Text            string
	Fields          []string
	Filter          GlobalIssueFilter
	IncludeProjects bool
	IncludeBackups  bool
}

// GlobalIssueList represents issues from multiple projects
type GlobalIssueList struct {
	Issues       []GlobalIssue `json:"issues"`
	Total        int           `json:"total"`
	Count        int           `json:"count"`
	ProjectCount int           `json:"project_count"`
}

// GlobalIssue represents an issue with project context
type GlobalIssue struct {
	*entities.Issue
	ProjectPath string `json:"project_path"`
	ProjectName string `json:"project_name"`
	IsArchived  bool   `json:"is_archived"`
}

// GlobalSearchResult represents search results across projects
type GlobalSearchResult struct {
	Issues       []GlobalIssue             `json:"issues"`
	Projects     []*entities.ProjectInfo   `json:"projects,omitempty"`
	Backups      []*entities.ProjectBackup `json:"backups,omitempty"`
	Total        int                       `json:"total"`
	Query        string                    `json:"query"`
	Duration     string                    `json:"duration"`
	ProjectCount int                       `json:"project_count"`
}

// GlobalStats provides comprehensive statistics across all projects
type GlobalStats struct {
	TotalProjects       int                        `json:"total_projects"`
	ActiveProjects      int                        `json:"active_projects"`
	TotalIssues         int                        `json:"total_issues"`
	TotalArchivedIssues int                        `json:"total_archived_issues"`
	TotalBackups        int                        `json:"total_backups"`
	ProjectStats        map[string]*ProjectStats   `json:"project_stats"`
	IssuesByStatus      map[entities.Status]int    `json:"issues_by_status"`
	IssuesByType        map[entities.IssueType]int `json:"issues_by_type"`
	IssuesByPriority    map[entities.Priority]int  `json:"issues_by_priority"`
	RecentActivity      []GlobalIssue              `json:"recent_activity"`
	TopProjects         []ProjectStat              `json:"top_projects"`
	StorageUsage        *StorageUsage              `json:"storage_usage"`
}

// ProjectStat contains project-level statistics
type ProjectStat struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	IssueCount    int    `json:"issue_count"`
	ArchivedCount int    `json:"archived_count"`
	BackupCount   int    `json:"backup_count"`
}

// StorageUsage contains storage utilization information
type StorageUsage struct {
	TotalSize        int64   `json:"total_size"`
	ArchiveSize      int64   `json:"archive_size"`
	BackupSize       int64   `json:"backup_size"`
	ConfigSize       int64   `json:"config_size"`
	CompressionRatio float64 `json:"compression_ratio"`
}

// IntegrityReport contains validation results
type IntegrityReport struct {
	IsValid           bool               `json:"is_valid"`
	CheckedAt         time.Time          `json:"checked_at"`
	ProjectsChecked   int                `json:"projects_checked"`
	IssuesChecked     int                `json:"issues_checked"`
	ArchivesChecked   int                `json:"archives_checked"`
	BackupsChecked    int                `json:"backups_checked"`
	Errors            []IntegrityError   `json:"errors"`
	Warnings          []IntegrityWarning `json:"warnings"`
	OrphanedFiles     []string           `json:"orphaned_files"`
	CorruptedFiles    []string           `json:"corrupted_files"`
	MissingReferences []string           `json:"missing_references"`
}

// IntegrityError represents a critical integrity issue
type IntegrityError struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Path        string `json:"path,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// IntegrityWarning represents a non-critical integrity issue
type IntegrityWarning struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Path        string `json:"path,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}
