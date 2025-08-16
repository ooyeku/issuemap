package entities

import (
	"time"
)

// DeduplicationConfig represents deduplication settings
type DeduplicationConfig struct {
	// Enable deduplication for new uploads
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Hash algorithm to use (sha256, sha1, md5)
	HashAlgorithm string `yaml:"hash_algorithm" json:"hash_algorithm"`

	// Minimum file size for deduplication (bytes)
	MinFileSize int64 `yaml:"min_file_size" json:"min_file_size"`

	// Maximum file size for deduplication (bytes, 0 = no limit)
	MaxFileSize int64 `yaml:"max_file_size" json:"max_file_size"`

	// File types to exclude from deduplication
	ExcludedTypes []string `yaml:"excluded_types" json:"excluded_types"`

	// Enable automatic migration of existing duplicates
	AutoMigrate bool `yaml:"auto_migrate" json:"auto_migrate"`
}

// DefaultDeduplicationConfig returns default deduplication settings
func DefaultDeduplicationConfig() *DeduplicationConfig {
	return &DeduplicationConfig{
		Enabled:       true,
		HashAlgorithm: "sha256",
		MinFileSize:   1024,       // 1KB minimum
		MaxFileSize:   0,          // No maximum
		ExcludedTypes: []string{}, // No exclusions by default
		AutoMigrate:   true,
	}
}

// FileHash represents a file's content hash and metadata
type FileHash struct {
	// Content hash (hex string)
	Hash string `yaml:"hash" json:"hash"`

	// Hash algorithm used
	Algorithm string `yaml:"algorithm" json:"algorithm"`

	// File size in bytes
	Size int64 `yaml:"size" json:"size"`

	// Original filename (first uploaded)
	OriginalFilename string `yaml:"original_filename" json:"original_filename"`

	// Content type
	ContentType string `yaml:"content_type" json:"content_type"`

	// Storage path for the actual file
	StoragePath string `yaml:"storage_path" json:"storage_path"`

	// Reference count (number of attachments using this file)
	RefCount int `yaml:"ref_count" json:"ref_count"`

	// First created timestamp
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`

	// Last accessed timestamp
	LastAccessed time.Time `yaml:"last_accessed" json:"last_accessed"`
}

// FileReference represents a reference from an attachment to a deduplicated file
type FileReference struct {
	// Attachment ID that references this file
	AttachmentID string `yaml:"attachment_id" json:"attachment_id"`

	// Issue ID for the attachment
	IssueID IssueID `yaml:"issue_id" json:"issue_id"`

	// File hash being referenced
	FileHash string `yaml:"file_hash" json:"file_hash"`

	// Original filename for this attachment (may differ from stored file)
	Filename string `yaml:"filename" json:"filename"`

	// When this reference was created
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
}

// DeduplicationStats represents statistics about deduplication
type DeduplicationStats struct {
	// Total number of unique files
	UniqueFiles int `json:"unique_files"`

	// Total number of references (attachments)
	TotalReferences int `json:"total_references"`

	// Number of deduplicated files (references > 1)
	DeduplicatedFiles int `json:"deduplicated_files"`

	// Total size of unique files (actual storage used)
	UniqueSize int64 `json:"unique_size"`

	// Total size if all files were stored separately
	TotalSizeWithoutDedup int64 `json:"total_size_without_dedup"`

	// Space saved through deduplication
	SpaceSaved int64 `json:"space_saved"`

	// Deduplication ratio (0.0 to 1.0)
	DeduplicationRatio float64 `json:"deduplication_ratio"`

	// Most referenced files
	TopDuplicates []FileHashInfo `json:"top_duplicates"`
}

// FileHashInfo represents information about a file hash for reporting
type FileHashInfo struct {
	Hash             string    `json:"hash"`
	OriginalFilename string    `json:"original_filename"`
	Size             int64     `json:"size"`
	RefCount         int       `json:"ref_count"`
	SpaceSaved       int64     `json:"space_saved"`
	CreatedAt        time.Time `json:"created_at"`
}

// DeduplicationReport represents a full deduplication analysis
type DeduplicationReport struct {
	// Timestamp of analysis
	Timestamp time.Time `json:"timestamp"`

	// Overall statistics
	Stats DeduplicationStats `json:"stats"`

	// Configuration used
	Config *DeduplicationConfig `json:"config"`

	// Potential duplicates found (for migration)
	PotentialDuplicates []DuplicateGroup `json:"potential_duplicates,omitempty"`

	// Errors encountered during analysis
	Errors []string `json:"errors,omitempty"`

	// Duration of analysis
	Duration time.Duration `json:"duration"`
}

// DuplicateGroup represents a group of files with identical content
type DuplicateGroup struct {
	// Content hash
	Hash string `json:"hash"`

	// File size
	Size int64 `json:"size"`

	// Number of duplicate files
	Count int `json:"count"`

	// Space that could be saved
	SpaceSavings int64 `json:"space_savings"`

	// List of duplicate files
	Files []DuplicateFile `json:"files"`
}

// DuplicateFile represents a file that has duplicates
type DuplicateFile struct {
	// Attachment ID
	AttachmentID string `json:"attachment_id"`

	// Issue ID
	IssueID IssueID `json:"issue_id"`

	// Current storage path
	StoragePath string `json:"storage_path"`

	// Filename
	Filename string `json:"filename"`

	// Content type
	ContentType string `json:"content_type"`

	// Upload timestamp
	UploadedAt time.Time `json:"uploaded_at"`
}

// MigrationResult represents the result of a deduplication migration
type MigrationResult struct {
	// Timestamp of migration
	Timestamp time.Time `json:"timestamp"`

	// Was this a dry run?
	DryRun bool `json:"dry_run"`

	// Number of files migrated
	FilesMigrated int `json:"files_migrated"`

	// Number of duplicate files removed
	DuplicatesRemoved int `json:"duplicates_removed"`

	// Space reclaimed
	SpaceReclaimed int64 `json:"space_reclaimed"`

	// Errors encountered
	Errors []string `json:"errors"`

	// Duration of migration
	Duration time.Duration `json:"duration"`
}
