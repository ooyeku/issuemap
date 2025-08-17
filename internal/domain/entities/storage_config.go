package entities

import (
	"fmt"
	"time"
)

// StorageConfig represents storage management configuration
type StorageConfig struct {
	// Maximum total size of .issuemap directory in bytes
	MaxProjectSize int64 `yaml:"max_project_size" json:"max_project_size"`

	// Maximum size for a single attachment in bytes
	MaxAttachmentSize int64 `yaml:"max_attachment_size" json:"max_attachment_size"`

	// Maximum total size of all attachments in bytes
	MaxTotalAttachments int64 `yaml:"max_total_attachments" json:"max_total_attachments"`

	// Maximum age for attachments in days (0 = no limit)
	MaxAttachmentAge int `yaml:"max_attachment_age" json:"max_attachment_age"`

	// Enable automatic cleanup
	EnableAutoCleanup bool `yaml:"enable_auto_cleanup" json:"enable_auto_cleanup"`

	// Warning threshold as percentage of max size (e.g., 80 = warn at 80%)
	WarningThreshold int `yaml:"warning_threshold" json:"warning_threshold"`

	// Critical threshold as percentage of max size (e.g., 95 = critical at 95%)
	CriticalThreshold int `yaml:"critical_threshold" json:"critical_threshold"`

	// Enable storage quota enforcement
	EnforceQuotas bool `yaml:"enforce_quotas" json:"enforce_quotas"`

	// Cleanup configuration
	CleanupConfig *CleanupConfig `yaml:"cleanup,omitempty" json:"cleanup,omitempty"`
}

// DefaultStorageConfig returns default storage configuration
func DefaultStorageConfig() *StorageConfig {
	return &StorageConfig{
		MaxProjectSize:      1024 * 1024 * 1024, // 1GB default
		MaxAttachmentSize:   10 * 1024 * 1024,   // 10MB default
		MaxTotalAttachments: 500 * 1024 * 1024,  // 500MB default
		MaxAttachmentAge:    0,                  // No age limit by default
		EnableAutoCleanup:   false,
		WarningThreshold:    80,
		CriticalThreshold:   95,
		EnforceQuotas:       false,
		CleanupConfig:       DefaultCleanupConfig(),
	}
}

// StorageStatus represents current storage usage
type StorageStatus struct {
	TotalSize          int64               `json:"total_size"`
	IssuesSize         int64               `json:"issues_size"`
	AttachmentsSize    int64               `json:"attachments_size"`
	HistorySize        int64               `json:"history_size"`
	TimeEntriesSize    int64               `json:"time_entries_size"`
	MetadataSize       int64               `json:"metadata_size"`
	ArchivesSize       int64               `json:"archives_size"`
	IssueCount         int                 `json:"issue_count"`
	AttachmentCount    int                 `json:"attachment_count"`
	ArchiveCount       int                 `json:"archive_count"`
	LargestFiles       []FileInfo          `json:"largest_files"`
	StorageByIssue     map[string]int64    `json:"storage_by_issue"`
	AvailableDiskSpace int64               `json:"available_disk_space"`
	UsagePercentage    float64             `json:"usage_percentage"`
	Status             StorageHealthStatus `json:"status"`
	Warnings           []string            `json:"warnings"`
	LastCalculated     time.Time           `json:"last_calculated"`

	// Deduplication information
	DeduplicationEnabled bool                `json:"deduplication_enabled,omitempty"`
	DeduplicationStats   *DeduplicationStats `json:"deduplication_stats,omitempty"`

	// Archive information
	ArchiveStats *ArchiveStats `json:"archive_stats,omitempty"`
}

// FileInfo represents information about a file
type FileInfo struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ModifiedTime time.Time `json:"modified_time"`
	Type         string    `json:"type"`
}

// StorageHealthStatus represents the health status of storage
type StorageHealthStatus string

const (
	StorageHealthOK       StorageHealthStatus = "ok"
	StorageHealthWarning  StorageHealthStatus = "warning"
	StorageHealthCritical StorageHealthStatus = "critical"
)

// CheckQuota checks if adding size would exceed quota
func (sc *StorageConfig) CheckQuota(currentSize, additionalSize int64) error {
	if !sc.EnforceQuotas {
		return nil
	}

	if sc.MaxProjectSize > 0 && currentSize+additionalSize > sc.MaxProjectSize {
		return fmt.Errorf("operation would exceed maximum project size of %s",
			FormatBytes(sc.MaxProjectSize))
	}

	return nil
}

// CheckAttachmentQuota checks if attachment meets size requirements
func (sc *StorageConfig) CheckAttachmentQuota(size int64, currentAttachmentsSize int64) error {
	if !sc.EnforceQuotas {
		return nil
	}

	if sc.MaxAttachmentSize > 0 && size > sc.MaxAttachmentSize {
		return fmt.Errorf("attachment size %s exceeds maximum allowed size of %s",
			FormatBytes(size), FormatBytes(sc.MaxAttachmentSize))
	}

	if sc.MaxTotalAttachments > 0 && currentAttachmentsSize+size > sc.MaxTotalAttachments {
		return fmt.Errorf("operation would exceed maximum total attachments size of %s",
			FormatBytes(sc.MaxTotalAttachments))
	}

	return nil
}

// GetHealthStatus determines health status based on usage
func (sc *StorageConfig) GetHealthStatus(currentSize int64) StorageHealthStatus {
	if sc.MaxProjectSize <= 0 {
		return StorageHealthOK
	}

	percentage := float64(currentSize) / float64(sc.MaxProjectSize) * 100

	if percentage >= float64(sc.CriticalThreshold) {
		return StorageHealthCritical
	}
	if percentage >= float64(sc.WarningThreshold) {
		return StorageHealthWarning
	}

	return StorageHealthOK
}

// FormatBytes formats bytes into human-readable string
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
