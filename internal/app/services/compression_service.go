package services

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// CompressionService handles file compression for attachments
type CompressionService struct {
	basePath       string
	configRepo     repositories.ConfigRepository
	attachmentRepo repositories.AttachmentRepository
	config         *entities.CompressionConfig
	stats          *entities.CompressionStats
	jobQueue       chan *entities.BackgroundCompressionJob
	workers        int
	mu             sync.RWMutex
	running        bool
	wg             sync.WaitGroup
	cancel         context.CancelFunc
}

// NewCompressionService creates a new compression service
func NewCompressionService(
	basePath string,
	configRepo repositories.ConfigRepository,
	attachmentRepo repositories.AttachmentRepository,
) *CompressionService {
	config := entities.DefaultCompressionConfig()

	// Try to load config from repository
	if configRepo != nil {
		if cfg, err := configRepo.Load(context.Background()); err == nil && cfg != nil {
			if cfg.StorageConfig != nil && cfg.StorageConfig.CompressionConfig != nil {
				config = cfg.StorageConfig.CompressionConfig
			}
		}
	}

	service := &CompressionService{
		basePath:       basePath,
		configRepo:     configRepo,
		attachmentRepo: attachmentRepo,
		config:         config,
		stats:          entities.NewCompressionStats(),
		jobQueue:       make(chan *entities.BackgroundCompressionJob, 100),
		workers:        2, // Start with 2 background workers
	}

	// Load existing stats if available
	service.loadStats()

	return service
}

// GetConfig returns current compression configuration
func (s *CompressionService) GetConfig() *entities.CompressionConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig updates compression configuration
func (s *CompressionService) UpdateConfig(config *entities.CompressionConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config

	// TODO: Save to config repository
	return nil
}

// GetStats returns current compression statistics
func (s *CompressionService) GetStats() *entities.CompressionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// CompressFile compresses a file if it meets the compression criteria
func (s *CompressionService) CompressFile(sourcePath, destPath string) (*entities.CompressionResult, error) {
	start := time.Now()
	result := &entities.CompressionResult{
		Success: false,
	}

	// Get file info
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to stat source file: %v", err)
		return result, err
	}

	result.OriginalSize = sourceInfo.Size()
	result.FinalSize = sourceInfo.Size()

	filename := filepath.Base(sourcePath)

	// Check if file should be compressed
	if !s.config.ShouldCompress(filename, sourceInfo.Size()) {
		result.Success = true
		result.Reason = "file not suitable for compression"
		result.Duration = time.Since(start)
		return result, nil
	}

	// Calculate original checksum
	originalChecksum, err := s.calculateChecksum(sourcePath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to calculate original checksum: %v", err)
		return result, err
	}

	// Create compressed version
	compressedSize, compressedChecksum, err := s.createCompressedFile(sourcePath, destPath)
	if err != nil {
		result.Error = fmt.Sprintf("compression failed: %v", err)
		return result, err
	}

	// Calculate compression ratio
	compressionRatio := float64(compressedSize) / float64(sourceInfo.Size())

	// Check if compression is worthwhile
	if compressionRatio >= s.config.MinCompressionRatio {
		// Compression didn't save enough space, remove compressed file
		os.Remove(destPath)
		result.Success = true
		result.Reason = fmt.Sprintf("compression ratio %.2f%% below threshold", compressionRatio*100)
		result.CompressionRatio = compressionRatio
		result.Duration = time.Since(start)
		return result, nil
	}

	// Compression successful
	result.Success = true
	result.Compressed = true
	result.FinalSize = compressedSize
	result.CompressionRatio = compressionRatio
	result.Duration = time.Since(start)

	// Create compression metadata
	result.Metadata = &entities.CompressionMetadata{
		Compressed:         true,
		OriginalSize:       sourceInfo.Size(),
		CompressedSize:     compressedSize,
		CompressionRatio:   compressionRatio,
		Algorithm:          "gzip",
		Level:              s.config.Level,
		CompressedAt:       time.Now(),
		OriginalChecksum:   originalChecksum,
		CompressedChecksum: compressedChecksum,
	}

	return result, nil
}

// DecompressFile decompresses a gzip-compressed file
func (s *CompressionService) DecompressFile(sourcePath, destPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open compressed file: %w", err)
	}
	defer sourceFile.Close()

	gzReader, err := gzip.NewReader(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, gzReader)
	if err != nil {
		os.Remove(destPath) // Clean up on failure
		return fmt.Errorf("failed to decompress file: %w", err)
	}

	return nil
}

// CompressAttachment compresses an attachment if suitable
func (s *CompressionService) CompressAttachment(ctx context.Context, attachment *entities.Attachment) (*entities.CompressionResult, error) {
	if !s.config.Enabled {
		return &entities.CompressionResult{
			Success:      true,
			Reason:       "compression disabled",
			OriginalSize: attachment.Size,
			FinalSize:    attachment.Size,
		}, nil
	}

	sourcePath := filepath.Join(s.basePath, attachment.StoragePath)
	compressedPath := sourcePath + ".gz"

	result, err := s.CompressFile(sourcePath, compressedPath)
	if err != nil {
		return result, err
	}

	// Update statistics
	s.updateStats(attachment.Filename, result)

	if result.Compressed {
		// Update attachment metadata
		attachment.Compression = result.Metadata
		attachment.Size = result.FinalSize

		// Replace original file with compressed version
		if err := os.Rename(compressedPath, sourcePath); err != nil {
			os.Remove(compressedPath) // Clean up
			return result, fmt.Errorf("failed to replace original with compressed file: %w", err)
		}

		// Update attachment in repository
		if err := s.attachmentRepo.SaveMetadata(ctx, attachment); err != nil {
			// Try to restore original by decompressing
			if decompErr := s.DecompressFile(sourcePath, sourcePath+".orig"); decompErr == nil {
				os.Rename(sourcePath+".orig", sourcePath)
			}
			return result, fmt.Errorf("failed to update attachment metadata: %w", err)
		}
	}

	return result, nil
}

// DecompressAttachment decompresses an attachment for retrieval
func (s *CompressionService) DecompressAttachment(attachment *entities.Attachment, destPath string) error {
	if attachment.Compression == nil || !attachment.Compression.Compressed {
		// File is not compressed, just copy it
		sourcePath := filepath.Join(s.basePath, attachment.StoragePath)
		return s.copyFile(sourcePath, destPath)
	}

	// File is compressed, decompress it
	sourcePath := filepath.Join(s.basePath, attachment.StoragePath)
	return s.DecompressFile(sourcePath, destPath)
}

// Start begins background compression workers
func (s *CompressionService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true

	// Start background workers
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.backgroundWorker(ctx)
	}

	return nil
}

// Stop gracefully stops background compression workers
func (s *CompressionService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.cancel()
	close(s.jobQueue)
	s.running = false

	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Workers finished
	case <-time.After(30 * time.Second):
		// Timeout waiting for workers
	}
}

// QueueBackgroundCompression queues an attachment for background compression
func (s *CompressionService) QueueBackgroundCompression(attachment *entities.Attachment) error {
	if !s.config.BackgroundCompression {
		return nil
	}

	job := &entities.BackgroundCompressionJob{
		ID:           fmt.Sprintf("comp_%d", time.Now().UnixNano()),
		AttachmentID: attachment.ID,
		FilePath:     filepath.Join(s.basePath, attachment.StoragePath),
		FileSize:     attachment.Size,
		Status:       entities.JobStatusPending,
		CreatedAt:    time.Now(),
	}

	select {
	case s.jobQueue <- job:
		return nil
	default:
		return fmt.Errorf("compression job queue is full")
	}
}

// backgroundWorker processes background compression jobs
func (s *CompressionService) backgroundWorker(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-s.jobQueue:
			if !ok {
				return
			}
			s.processBackgroundJob(ctx, job)
		}
	}
}

// processBackgroundJob processes a single background compression job
func (s *CompressionService) processBackgroundJob(ctx context.Context, job *entities.BackgroundCompressionJob) {
	job.Status = entities.JobStatusRunning
	startTime := time.Now()
	job.StartedAt = &startTime

	// Get attachment
	attachment, err := s.attachmentRepo.GetMetadata(ctx, job.AttachmentID)
	if err != nil {
		job.Status = entities.JobStatusFailed
		job.Error = fmt.Sprintf("failed to get attachment: %v", err)
		job.CompletedAt = &[]time.Time{time.Now()}[0]
		return
	}

	// Skip if already compressed
	if attachment.Compression != nil && attachment.Compression.Compressed {
		job.Status = entities.JobStatusSkipped
		job.Error = "attachment already compressed"
		job.CompletedAt = &[]time.Time{time.Now()}[0]
		return
	}

	// Compress attachment
	result, err := s.CompressAttachment(ctx, attachment)
	if err != nil {
		job.Status = entities.JobStatusFailed
		job.Error = fmt.Sprintf("compression failed: %v", err)
		job.Attempts++
	} else {
		job.Status = entities.JobStatusCompleted
		job.Result = result
	}

	job.CompletedAt = &[]time.Time{time.Now()}[0]
}

// createCompressedFile creates a gzip-compressed version of a file
func (s *CompressionService) createCompressedFile(sourcePath, destPath string) (int64, string, error) {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return 0, "", err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return 0, "", err
	}
	defer destFile.Close()

	// Create gzip writer with specified compression level
	gzWriter, err := gzip.NewWriterLevel(destFile, s.config.Level)
	if err != nil {
		os.Remove(destPath)
		return 0, "", err
	}
	defer gzWriter.Close()

	// Create hasher for checksum
	hasher := sha256.New()
	multiWriter := io.MultiWriter(gzWriter, hasher)

	// Copy and compress
	_, err = io.Copy(multiWriter, sourceFile)
	if err != nil {
		os.Remove(destPath)
		return 0, "", err
	}

	// Close gzip writer to flush
	gzWriter.Close()

	// Get file size
	info, err := destFile.Stat()
	if err != nil {
		os.Remove(destPath)
		return 0, "", err
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	return info.Size(), checksum, nil
}

// calculateChecksum calculates SHA256 checksum of a file
func (s *CompressionService) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// copyFile copies a file from source to destination
func (s *CompressionService) copyFile(sourcePath, destPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// updateStats updates compression statistics
func (s *CompressionService) updateStats(filename string, result *entities.CompressionResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats.UpdateStats(filename, result)
	s.saveStats()
}

// loadStats loads compression statistics from disk
func (s *CompressionService) loadStats() error {
	statsPath := filepath.Join(s.basePath, "compression_stats.json")

	data, err := os.ReadFile(statsPath)
	if os.IsNotExist(err) {
		s.stats = entities.NewCompressionStats()
		return nil
	}
	if err != nil {
		return err
	}

	var stats entities.CompressionStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return err
	}

	s.stats = &stats
	return nil
}

// saveStats saves compression statistics to disk
func (s *CompressionService) saveStats() error {
	if s.stats == nil {
		return nil
	}

	statsPath := filepath.Join(s.basePath, "compression_stats.json")

	data, err := json.MarshalIndent(s.stats, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(statsPath, data, 0644)
}

// RunBatchCompression compresses a batch of existing attachments
func (s *CompressionService) RunBatchCompression(ctx context.Context, batchSize int) (*entities.CompressionResult, error) {
	start := time.Now()
	result := &entities.CompressionResult{
		Success: true,
	}

	// Get uncompressed attachments
	// This would need to be implemented in the attachment repository
	// For now, we'll return a placeholder
	result.Duration = time.Since(start)
	return result, nil
}
