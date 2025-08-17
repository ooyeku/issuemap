package entities

import (
	"path/filepath"
	"strings"
	"time"
)

// CompressionConfig represents attachment compression configuration
type CompressionConfig struct {
	// Enable compression for attachments
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Compression level (1-9, 1=fast, 9=best compression)
	Level int `yaml:"level" json:"level"`

	// Minimum file size for compression (bytes)
	MinFileSize int64 `yaml:"min_file_size" json:"min_file_size"`

	// Maximum file size for compression (bytes)
	MaxFileSize int64 `yaml:"max_file_size" json:"max_file_size"`

	// Minimum compression ratio to keep compressed version (0.0-1.0)
	MinCompressionRatio float64 `yaml:"min_compression_ratio" json:"min_compression_ratio"`

	// File extensions to compress
	CompressibleExtensions []string `yaml:"compressible_extensions" json:"compressible_extensions"`

	// File extensions to skip compression
	SkipExtensions []string `yaml:"skip_extensions" json:"skip_extensions"`

	// Enable background compression for existing files
	BackgroundCompression bool `yaml:"background_compression" json:"background_compression"`

	// Batch size for background compression
	BackgroundBatchSize int `yaml:"background_batch_size" json:"background_batch_size"`
}

// DefaultCompressionConfig returns default compression settings
func DefaultCompressionConfig() *CompressionConfig {
	return &CompressionConfig{
		Enabled:             true,
		Level:               6,                // Good balance of compression/speed
		MinFileSize:         1024,             // 1KB minimum
		MaxFileSize:         50 * 1024 * 1024, // 50MB maximum
		MinCompressionRatio: 0.1,              // Keep if compressed to 90% or less of original
		CompressibleExtensions: []string{
			// Text files
			".txt", ".log", ".csv", ".json", ".xml", ".yaml", ".yml",
			// Code files
			".js", ".py", ".go", ".java", ".c", ".cpp", ".h", ".hpp",
			".rb", ".php", ".pl", ".sh", ".sql", ".html", ".css",
			// Documents
			".md", ".markdown", ".rst", ".tex",
			// Configuration files
			".ini", ".conf", ".config", ".properties",
		},
		SkipExtensions: []string{
			// Already compressed images
			".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp",
			// Archives
			".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar",
			// Media files
			".mp4", ".mp3", ".avi", ".mov", ".mkv", ".flv", ".wav", ".ogg",
			// Documents (mixed benefit)
			".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
			// Binaries
			".exe", ".dll", ".so", ".dylib",
		},
		BackgroundCompression: true,
		BackgroundBatchSize:   10,
	}
}

// ShouldCompress determines if a file should be compressed based on its properties
func (c *CompressionConfig) ShouldCompress(filename string, size int64) bool {
	if !c.Enabled {
		return false
	}

	// Check size limits
	if size < c.MinFileSize || size > c.MaxFileSize {
		return false
	}

	ext := strings.ToLower(filepath.Ext(filename))

	// Check skip list first
	for _, skipExt := range c.SkipExtensions {
		if ext == skipExt {
			return false
		}
	}

	// Check compressible list
	for _, compressExt := range c.CompressibleExtensions {
		if ext == compressExt {
			return true
		}
	}

	// Default: don't compress unknown types
	return false
}

// CompressionMetadata represents metadata about a compressed attachment
type CompressionMetadata struct {
	// Whether the file is compressed
	Compressed bool `json:"compressed"`

	// Original file size
	OriginalSize int64 `json:"original_size"`

	// Compressed file size
	CompressedSize int64 `json:"compressed_size"`

	// Compression ratio (0.0-1.0, lower is better compression)
	CompressionRatio float64 `json:"compression_ratio"`

	// Compression algorithm used
	Algorithm string `json:"algorithm"`

	// Compression level used
	Level int `json:"level"`

	// When compression was performed
	CompressedAt time.Time `json:"compressed_at"`

	// Checksum of original file
	OriginalChecksum string `json:"original_checksum"`

	// Checksum of compressed file
	CompressedChecksum string `json:"compressed_checksum"`
}

// CompressionResult represents the result of a compression operation
type CompressionResult struct {
	// Whether compression was successful
	Success bool `json:"success"`

	// Whether compression was actually performed
	Compressed bool `json:"compressed"`

	// Reason if not compressed
	Reason string `json:"reason,omitempty"`

	// Original file size
	OriginalSize int64 `json:"original_size"`

	// Final file size (compressed or original)
	FinalSize int64 `json:"final_size"`

	// Compression ratio achieved
	CompressionRatio float64 `json:"compression_ratio"`

	// Time taken for compression
	Duration time.Duration `json:"duration"`

	// Compression metadata
	Metadata *CompressionMetadata `json:"metadata,omitempty"`

	// Any errors encountered
	Error string `json:"error,omitempty"`
}

// CompressionStats represents statistics about compression operations
type CompressionStats struct {
	// Total files processed
	TotalFiles int `json:"total_files"`

	// Files successfully compressed
	CompressedFiles int `json:"compressed_files"`

	// Files skipped (not suitable for compression)
	SkippedFiles int `json:"skipped_files"`

	// Total original size of all files
	TotalOriginalSize int64 `json:"total_original_size"`

	// Total compressed size of all files
	TotalCompressedSize int64 `json:"total_compressed_size"`

	// Total space saved
	SpaceSaved int64 `json:"space_saved"`

	// Overall compression ratio
	OverallCompressionRatio float64 `json:"overall_compression_ratio"`

	// Average compression ratio for compressed files
	AverageCompressionRatio float64 `json:"average_compression_ratio"`

	// Compression by file type
	CompressionByType map[string]*TypeCompressionStats `json:"compression_by_type"`

	// Last update time
	LastUpdated time.Time `json:"last_updated"`
}

// TypeCompressionStats represents compression statistics for a specific file type
type TypeCompressionStats struct {
	// File extension
	Extension string `json:"extension"`

	// Number of files of this type
	FileCount int `json:"file_count"`

	// Number of compressed files of this type
	CompressedCount int `json:"compressed_count"`

	// Total original size for this type
	OriginalSize int64 `json:"original_size"`

	// Total compressed size for this type
	CompressedSize int64 `json:"compressed_size"`

	// Average compression ratio for this type
	AverageRatio float64 `json:"average_ratio"`

	// Best compression ratio achieved for this type
	BestRatio float64 `json:"best_ratio"`

	// Worst compression ratio for this type
	WorstRatio float64 `json:"worst_ratio"`
}

// BackgroundCompressionJob represents a background compression task
type BackgroundCompressionJob struct {
	// Job ID
	ID string `json:"id"`

	// Attachment ID to compress
	AttachmentID string `json:"attachment_id"`

	// File path
	FilePath string `json:"file_path"`

	// Original file size
	FileSize int64 `json:"file_size"`

	// Job status
	Status BackgroundJobStatus `json:"status"`

	// When job was created
	CreatedAt time.Time `json:"created_at"`

	// When job was started
	StartedAt *time.Time `json:"started_at,omitempty"`

	// When job was completed
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Compression result
	Result *CompressionResult `json:"result,omitempty"`

	// Error message if failed
	Error string `json:"error,omitempty"`

	// Number of retry attempts
	Attempts int `json:"attempts"`
}

// BackgroundJobStatus represents the status of a background compression job
type BackgroundJobStatus string

const (
	JobStatusPending   BackgroundJobStatus = "pending"
	JobStatusRunning   BackgroundJobStatus = "running"
	JobStatusCompleted BackgroundJobStatus = "completed"
	JobStatusFailed    BackgroundJobStatus = "failed"
	JobStatusSkipped   BackgroundJobStatus = "skipped"
)

// NewCompressionStats creates a new compression statistics object
func NewCompressionStats() *CompressionStats {
	return &CompressionStats{
		CompressionByType: make(map[string]*TypeCompressionStats),
		LastUpdated:       time.Now(),
	}
}

// UpdateStats updates compression statistics with a new result
func (s *CompressionStats) UpdateStats(filename string, result *CompressionResult) {
	s.TotalFiles++
	s.TotalOriginalSize += result.OriginalSize
	s.TotalCompressedSize += result.FinalSize
	s.LastUpdated = time.Now()

	if result.Compressed {
		s.CompressedFiles++
	} else {
		s.SkippedFiles++
	}

	// Update by type
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = "no_extension"
	}

	typeStats, exists := s.CompressionByType[ext]
	if !exists {
		typeStats = &TypeCompressionStats{
			Extension:  ext,
			BestRatio:  1.0,
			WorstRatio: 0.0,
		}
		s.CompressionByType[ext] = typeStats
	}

	typeStats.FileCount++
	typeStats.OriginalSize += result.OriginalSize
	typeStats.CompressedSize += result.FinalSize

	if result.Compressed {
		typeStats.CompressedCount++
		ratio := result.CompressionRatio

		// Update best/worst ratios
		if typeStats.CompressedCount == 1 {
			typeStats.BestRatio = ratio
			typeStats.WorstRatio = ratio
			typeStats.AverageRatio = ratio
		} else {
			if ratio < typeStats.BestRatio {
				typeStats.BestRatio = ratio
			}
			if ratio > typeStats.WorstRatio {
				typeStats.WorstRatio = ratio
			}

			// Update average ratio
			totalRatio := typeStats.AverageRatio * float64(typeStats.CompressedCount-1)
			typeStats.AverageRatio = (totalRatio + ratio) / float64(typeStats.CompressedCount)
		}
	}

	// Calculate overall statistics
	if s.TotalOriginalSize > 0 {
		s.OverallCompressionRatio = float64(s.TotalCompressedSize) / float64(s.TotalOriginalSize)
		s.SpaceSaved = s.TotalOriginalSize - s.TotalCompressedSize
	}

	// Calculate average compression ratio for compressed files only
	if s.CompressedFiles > 0 {
		var totalRatio float64
		for _, typeStats := range s.CompressionByType {
			if typeStats.CompressedCount > 0 {
				totalRatio += typeStats.AverageRatio * float64(typeStats.CompressedCount)
			}
		}
		s.AverageCompressionRatio = totalRatio / float64(s.CompressedFiles)
	}
}
