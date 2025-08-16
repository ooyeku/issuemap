package entities

import (
	"time"
)

// AdminCleanupOptions represents options for admin cleanup operations
type AdminCleanupOptions struct {
	// General options
	DryRun       bool `json:"dry_run"`
	ForceConfirm bool `json:"force_confirm"`
	NoBackup     bool `json:"no_backup"`

	// Target filtering
	OlderThan         *time.Duration `json:"older_than,omitempty"`
	ClosedOnly        bool           `json:"closed_only"`
	ClosedDays        *int           `json:"closed_days,omitempty"`
	OrphanedOnly      bool           `json:"orphaned_only"`
	TimeEntriesOnly   bool           `json:"time_entries_only"`
	TimeEntriesBefore *time.Time     `json:"time_entries_before,omitempty"`
}

// CleanupTarget represents what type of cleanup to perform
type CleanupTarget string

const (
	CleanupTargetAll                 CleanupTarget = "all"
	CleanupTargetClosedIssues        CleanupTarget = "closed_issues"
	CleanupTargetOrphanedAttachments CleanupTarget = "orphaned_attachments"
	CleanupTargetTimeEntries         CleanupTarget = "time_entries"
	CleanupTargetHistory             CleanupTarget = "history"
	CleanupTargetEmptyDirectories    CleanupTarget = "empty_directories"
)

// AdminCleanupResult extends CleanupResult with admin-specific information
type AdminCleanupResult struct {
	*CleanupResult

	// Admin-specific fields
	Target           CleanupTarget          `json:"target"`
	BackupCreated    bool                   `json:"backup_created"`
	BackupLocation   string                 `json:"backup_location,omitempty"`
	ConfirmationUsed bool                   `json:"confirmation_used"`
	FilterCriteria   map[string]interface{} `json:"filter_criteria"`
}

// BackupInfo represents information about a cleanup backup
type BackupInfo struct {
	ID          string     `json:"id"`
	Timestamp   time.Time  `json:"timestamp"`
	Description string     `json:"description"`
	Location    string     `json:"location"`
	Size        int64      `json:"size"`
	ItemsCount  int        `json:"items_count"`
	CanRestore  bool       `json:"can_restore"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// RestoreOptions represents options for restoring from backup
type RestoreOptions struct {
	BackupID     string `json:"backup_id"`
	DryRun       bool   `json:"dry_run"`
	ForceConfirm bool   `json:"force_confirm"`
}

// RestoreResult represents the result of a restore operation
type RestoreResult struct {
	Timestamp     time.Time     `json:"timestamp"`
	DryRun        bool          `json:"dry_run"`
	BackupID      string        `json:"backup_id"`
	ItemsRestored int           `json:"items_restored"`
	Errors        []string      `json:"errors"`
	Duration      time.Duration `json:"duration"`
}
