package entities

import (
	"time"
)

// CleanupConfig represents cleanup and retention configuration
type CleanupConfig struct {
	// Enable automatic cleanup
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Schedule for automatic cleanup (cron expression or interval)
	Schedule string `yaml:"schedule" json:"schedule"`

	// Retention policies by data type (in days)
	RetentionDays CleanupRetention `yaml:"retention_days" json:"retention_days"`

	// Size-based cleanup triggers
	SizeTriggers CleanupSizeTriggers `yaml:"size_triggers" json:"size_triggers"`

	// Archive deleted items before removal
	ArchiveBeforeDelete bool `yaml:"archive_before_delete" json:"archive_before_delete"`

	// Archive location
	ArchivePath string `yaml:"archive_path" json:"archive_path"`

	// Dry run mode - simulate cleanup without actually deleting
	DryRunMode bool `yaml:"dry_run_mode" json:"dry_run_mode"`

	// Keep minimum number of items regardless of age
	MinimumKeep CleanupMinimumKeep `yaml:"minimum_keep" json:"minimum_keep"`
}

// CleanupRetention defines retention periods for different data types
type CleanupRetention struct {
	// Closed issues older than this will be cleaned up (0 = never)
	ClosedIssues int `yaml:"closed_issues" json:"closed_issues"`

	// Attachments for closed issues older than this will be cleaned up
	ClosedIssueAttachments int `yaml:"closed_issue_attachments" json:"closed_issue_attachments"`

	// History entries older than this will be cleaned up
	History int `yaml:"history" json:"history"`

	// Time entries older than this will be cleaned up
	TimeEntries int `yaml:"time_entries" json:"time_entries"`

	// Orphaned attachments older than this will be cleaned up
	OrphanedAttachments int `yaml:"orphaned_attachments" json:"orphaned_attachments"`

	// Empty directories will be cleaned up immediately
	EmptyDirectories bool `yaml:"empty_directories" json:"empty_directories"`
}

// CleanupSizeTriggers defines size-based cleanup triggers
type CleanupSizeTriggers struct {
	// Trigger cleanup when total size exceeds this (bytes)
	MaxTotalSize int64 `yaml:"max_total_size" json:"max_total_size"`

	// Trigger cleanup when attachments exceed this size
	MaxAttachmentsSize int64 `yaml:"max_attachments_size" json:"max_attachments_size"`

	// Trigger cleanup when history exceeds this size
	MaxHistorySize int64 `yaml:"max_history_size" json:"max_history_size"`

	// Percentage of quota that triggers cleanup (e.g., 90 = cleanup at 90% full)
	TriggerPercentage int `yaml:"trigger_percentage" json:"trigger_percentage"`
}

// CleanupMinimumKeep defines minimum items to keep
type CleanupMinimumKeep struct {
	// Minimum number of closed issues to keep
	ClosedIssues int `yaml:"closed_issues" json:"closed_issues"`

	// Minimum number of history entries per issue
	HistoryPerIssue int `yaml:"history_per_issue" json:"history_per_issue"`

	// Minimum number of time entries per issue
	TimeEntriesPerIssue int `yaml:"time_entries_per_issue" json:"time_entries_per_issue"`
}

// CleanupResult represents the result of a cleanup operation
type CleanupResult struct {
	// Timestamp of cleanup
	Timestamp time.Time `json:"timestamp"`

	// Was this a dry run?
	DryRun bool `json:"dry_run"`

	// Items cleaned by category
	ItemsCleaned CleanupStats `json:"items_cleaned"`

	// Space reclaimed in bytes
	SpaceReclaimed int64 `json:"space_reclaimed"`

	// Errors encountered during cleanup
	Errors []string `json:"errors"`

	// Duration of cleanup operation
	Duration time.Duration `json:"duration"`

	// Items archived (if archiving enabled)
	ItemsArchived int `json:"items_archived"`
}

// CleanupStats tracks cleanup statistics by category
type CleanupStats struct {
	ClosedIssues        int `json:"closed_issues"`
	Attachments         int `json:"attachments"`
	OrphanedAttachments int `json:"orphaned_attachments"`
	HistoryEntries      int `json:"history_entries"`
	TimeEntries         int `json:"time_entries"`
	EmptyDirectories    int `json:"empty_directories"`
	Total               int `json:"total"`
}

// DefaultCleanupConfig returns default cleanup configuration
func DefaultCleanupConfig() *CleanupConfig {
	return &CleanupConfig{
		Enabled:  false,
		Schedule: "0 2 * * *", // Daily at 2 AM
		RetentionDays: CleanupRetention{
			ClosedIssues:           90,  // Keep closed issues for 90 days
			ClosedIssueAttachments: 30,  // Keep attachments for closed issues for 30 days
			History:                365, // Keep history for 1 year
			TimeEntries:            365, // Keep time entries for 1 year
			OrphanedAttachments:    7,   // Clean orphaned attachments after 7 days
			EmptyDirectories:       true,
		},
		SizeTriggers: CleanupSizeTriggers{
			MaxTotalSize:       0, // No size limit by default
			MaxAttachmentsSize: 0,
			MaxHistorySize:     0,
			TriggerPercentage:  90, // Trigger at 90% of quota
		},
		ArchiveBeforeDelete: false,
		ArchivePath:         ".issuemap_archive",
		DryRunMode:          false,
		MinimumKeep: CleanupMinimumKeep{
			ClosedIssues:        10, // Always keep at least 10 closed issues
			HistoryPerIssue:     5,  // Keep at least 5 history entries per issue
			TimeEntriesPerIssue: 5,  // Keep at least 5 time entries per issue
		},
	}
}

// ShouldCleanup checks if an item should be cleaned up based on age
func (c *CleanupConfig) ShouldCleanup(itemAge time.Duration, retentionDays int) bool {
	if retentionDays <= 0 {
		return false // Never cleanup if retention is 0 or negative
	}

	retentionDuration := time.Duration(retentionDays) * 24 * time.Hour
	return itemAge > retentionDuration
}

// NeedsCleanup checks if cleanup should be triggered based on size
func (c *CleanupConfig) NeedsCleanup(currentSize int64, maxSize int64) bool {
	if maxSize <= 0 {
		return false
	}

	if c.SizeTriggers.TriggerPercentage > 0 {
		threshold := float64(maxSize) * float64(c.SizeTriggers.TriggerPercentage) / 100
		return float64(currentSize) >= threshold
	}

	return currentSize >= maxSize
}
