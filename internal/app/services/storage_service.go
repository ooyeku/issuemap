package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// StorageService handles storage monitoring and management
type StorageService struct {
	basePath       string
	configRepo     repositories.ConfigRepository
	issueRepo      repositories.IssueRepository
	attachmentRepo repositories.AttachmentRepository
	config         *entities.StorageConfig
	mu             sync.RWMutex
	cache          *storageCache
	dedupService   *DeduplicationService
}

type storageCache struct {
	status    *entities.StorageStatus
	timestamp time.Time
	mu        sync.RWMutex
}

// NewStorageService creates a new storage service
func NewStorageService(
	basePath string,
	configRepo repositories.ConfigRepository,
	issueRepo repositories.IssueRepository,
	attachmentRepo repositories.AttachmentRepository,
) *StorageService {
	config := entities.DefaultStorageConfig()

	// Try to load config from repository
	if configRepo != nil {
		if cfg, err := configRepo.Load(context.Background()); err == nil && cfg != nil {
			if cfg.StorageConfig != nil {
				config = cfg.StorageConfig
			}
		}
	}

	return &StorageService{
		basePath:       basePath,
		configRepo:     configRepo,
		issueRepo:      issueRepo,
		attachmentRepo: attachmentRepo,
		config:         config,
		cache:          &storageCache{},
		dedupService:   nil, // Will be set via SetDeduplicationService
	}
}

// SetDeduplicationService sets the deduplication service
func (s *StorageService) SetDeduplicationService(dedupService *DeduplicationService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dedupService = dedupService
}

// GetStorageStatus returns current storage status
func (s *StorageService) GetStorageStatus(ctx context.Context, forceRefresh bool) (*entities.StorageStatus, error) {
	// Check cache first
	if !forceRefresh {
		s.cache.mu.RLock()
		if s.cache.status != nil && time.Since(s.cache.timestamp) < 5*time.Minute {
			status := s.cache.status
			s.cache.mu.RUnlock()
			return status, nil
		}
		s.cache.mu.RUnlock()
	}

	// Calculate fresh status
	status, err := s.calculateStorageStatus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "StorageService.GetStorageStatus", "calculation failed")
	}

	// Update cache
	s.cache.mu.Lock()
	s.cache.status = status
	s.cache.timestamp = time.Now()
	s.cache.mu.Unlock()

	return status, nil
}

// calculateStorageStatus calculates current storage usage
func (s *StorageService) calculateStorageStatus(ctx context.Context) (*entities.StorageStatus, error) {
	status := &entities.StorageStatus{
		StorageByIssue: make(map[string]int64),
		LastCalculated: time.Now(),
		Warnings:       []string{},
	}

	// Calculate directory sizes
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Issues directory
	wg.Add(1)
	go func() {
		defer wg.Done()
		size, count := s.calculateDirSize(filepath.Join(s.basePath, "issues"))
		mu.Lock()
		status.IssuesSize = size
		status.IssueCount = count
		mu.Unlock()
	}()

	// Attachments directory
	wg.Add(1)
	go func() {
		defer wg.Done()
		size, count := s.calculateDirSize(filepath.Join(s.basePath, "attachments"))
		mu.Lock()
		status.AttachmentsSize = size
		status.AttachmentCount = count
		mu.Unlock()
	}()

	// History directory
	wg.Add(1)
	go func() {
		defer wg.Done()
		size, _ := s.calculateDirSize(filepath.Join(s.basePath, "history"))
		mu.Lock()
		status.HistorySize = size
		mu.Unlock()
	}()

	// Time entries directory
	wg.Add(1)
	go func() {
		defer wg.Done()
		size, _ := s.calculateDirSize(filepath.Join(s.basePath, "time_entries"))
		mu.Lock()
		status.TimeEntriesSize = size
		mu.Unlock()
	}()

	// Metadata directory
	wg.Add(1)
	go func() {
		defer wg.Done()
		size, _ := s.calculateDirSize(filepath.Join(s.basePath, "metadata"))
		mu.Lock()
		status.MetadataSize = size
		mu.Unlock()
	}()

	wg.Wait()

	// Add deduplication directory to total if it exists
	dedupSize, _ := s.calculateDirSize(filepath.Join(s.basePath, "dedup"))

	// Calculate total size
	status.TotalSize = status.IssuesSize + status.AttachmentsSize +
		status.HistorySize + status.TimeEntriesSize + status.MetadataSize + dedupSize

	// Add deduplication information if available
	s.mu.RLock()
	if s.dedupService != nil {
		status.DeduplicationEnabled = s.dedupService.GetConfig().Enabled
		if dedupStats, err := s.dedupService.GetDeduplicationStats(); err == nil {
			status.DeduplicationStats = dedupStats
		}
	}
	s.mu.RUnlock()

	// Get largest files
	status.LargestFiles = s.findLargestFiles(10)

	// Calculate storage by issue for attachments
	s.calculateStorageByIssue(status)

	// Get available disk space
	status.AvailableDiskSpace = s.getAvailableDiskSpace()

	// Calculate usage percentage
	if s.config.MaxProjectSize > 0 {
		status.UsagePercentage = float64(status.TotalSize) / float64(s.config.MaxProjectSize) * 100
	}

	// Determine health status
	status.Status = s.config.GetHealthStatus(status.TotalSize)

	// Generate warnings
	s.generateWarnings(status)

	return status, nil
}

// calculateDirSize calculates size of a directory
func (s *StorageService) calculateDirSize(path string) (int64, int) {
	var size int64
	var count int

	filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			size += info.Size()
			count++
		}
		return nil
	})

	return size, count
}

// findLargestFiles finds the largest files in the project
func (s *StorageService) findLargestFiles(limit int) []entities.FileInfo {
	var files []entities.FileInfo

	filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip git directory
		if strings.Contains(path, ".git") {
			return nil
		}

		relPath, _ := filepath.Rel(s.basePath, path)
		fileType := "other"

		if strings.HasPrefix(relPath, "attachments/") {
			fileType = "attachment"
		} else if strings.HasPrefix(relPath, "issues/") {
			fileType = "issue"
		} else if strings.HasPrefix(relPath, "history/") {
			fileType = "history"
		}

		files = append(files, entities.FileInfo{
			Path:         relPath,
			Size:         info.Size(),
			ModifiedTime: info.ModTime(),
			Type:         fileType,
		})

		return nil
	})

	// Sort by size descending
	sort.Slice(files, func(i, j int) bool {
		return files[i].Size > files[j].Size
	})

	// Return top N files
	if len(files) > limit {
		files = files[:limit]
	}

	return files
}

// calculateStorageByIssue calculates storage used by each issue
func (s *StorageService) calculateStorageByIssue(status *entities.StorageStatus) {
	attachmentsPath := filepath.Join(s.basePath, "attachments")

	entries, err := os.ReadDir(attachmentsPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			issueID := entry.Name()
			issuePath := filepath.Join(attachmentsPath, issueID)
			size, _ := s.calculateDirSize(issuePath)
			if size > 0 {
				status.StorageByIssue[issueID] = size
			}
		}
	}
}

// getAvailableDiskSpace gets available disk space
func (s *StorageService) getAvailableDiskSpace() int64 {
	var stat syscall.Statfs_t

	if err := syscall.Statfs(s.basePath, &stat); err != nil {
		return 0
	}

	// Available space = block size * available blocks
	return int64(stat.Bavail) * int64(stat.Bsize)
}

// generateWarnings generates storage warnings
func (s *StorageService) generateWarnings(status *entities.StorageStatus) {
	config := s.config

	// Check total size warnings
	if config.MaxProjectSize > 0 {
		if status.Status == entities.StorageHealthCritical {
			status.Warnings = append(status.Warnings,
				fmt.Sprintf("CRITICAL: Storage usage at %.1f%% of maximum", status.UsagePercentage))
		} else if status.Status == entities.StorageHealthWarning {
			status.Warnings = append(status.Warnings,
				fmt.Sprintf("WARNING: Storage usage at %.1f%% of maximum", status.UsagePercentage))
		}
	}

	// Check available disk space
	if status.AvailableDiskSpace < 100*1024*1024 { // Less than 100MB
		status.Warnings = append(status.Warnings,
			fmt.Sprintf("CRITICAL: Only %s of disk space remaining",
				entities.FormatBytes(status.AvailableDiskSpace)))
	} else if status.AvailableDiskSpace < 500*1024*1024 { // Less than 500MB
		status.Warnings = append(status.Warnings,
			fmt.Sprintf("WARNING: Only %s of disk space remaining",
				entities.FormatBytes(status.AvailableDiskSpace)))
	}

	// Check attachment size
	if config.MaxTotalAttachments > 0 && status.AttachmentsSize > config.MaxTotalAttachments {
		status.Warnings = append(status.Warnings,
			fmt.Sprintf("Attachments size %s exceeds limit of %s",
				entities.FormatBytes(status.AttachmentsSize),
				entities.FormatBytes(config.MaxTotalAttachments)))
	}

	// Check for large individual files
	for _, file := range status.LargestFiles {
		if file.Size > 50*1024*1024 { // Files larger than 50MB
			status.Warnings = append(status.Warnings,
				fmt.Sprintf("Large file detected: %s (%s)",
					file.Path, entities.FormatBytes(file.Size)))
		}
	}
}

// CheckStorageQuota checks if an operation would exceed storage quota
func (s *StorageService) CheckStorageQuota(ctx context.Context, additionalSize int64) error {
	status, err := s.GetStorageStatus(ctx, false)
	if err != nil {
		return errors.Wrap(err, "StorageService.CheckStorageQuota", "failed to get status")
	}

	return s.config.CheckQuota(status.TotalSize, additionalSize)
}

// CheckAttachmentQuota checks if an attachment would exceed quota
func (s *StorageService) CheckAttachmentQuota(ctx context.Context, size int64) error {
	status, err := s.GetStorageStatus(ctx, false)
	if err != nil {
		return errors.Wrap(err, "StorageService.CheckAttachmentQuota", "failed to get status")
	}

	return s.config.CheckAttachmentQuota(size, status.AttachmentsSize)
}

// UpdateConfig updates the storage configuration
func (s *StorageService) UpdateConfig(config *entities.StorageConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config

	// Save to config repository
	if s.configRepo != nil {
		cfg, err := s.configRepo.Load(context.Background())
		if err != nil {
			cfg = &entities.Config{}
		}
		cfg.StorageConfig = config

		if err := s.configRepo.Save(context.Background(), cfg); err != nil {
			return errors.Wrap(err, "StorageService.UpdateConfig", "failed to save config")
		}
	}

	// Clear cache to force recalculation
	s.cache.mu.Lock()
	s.cache.status = nil
	s.cache.mu.Unlock()

	return nil
}

// GetConfig returns current storage configuration
func (s *StorageService) GetConfig() *entities.StorageConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// GetStorageBreakdown returns a detailed breakdown of storage usage
func (s *StorageService) GetStorageBreakdown(ctx context.Context) (map[string]interface{}, error) {
	status, err := s.GetStorageStatus(ctx, false)
	if err != nil {
		return nil, err
	}

	breakdown := map[string]interface{}{
		"total_size":       entities.FormatBytes(status.TotalSize),
		"total_size_bytes": status.TotalSize,
		"categories": map[string]interface{}{
			"issues": map[string]interface{}{
				"size":       entities.FormatBytes(status.IssuesSize),
				"size_bytes": status.IssuesSize,
				"count":      status.IssueCount,
				"percentage": s.calculatePercentage(status.IssuesSize, status.TotalSize),
			},
			"attachments": map[string]interface{}{
				"size":       entities.FormatBytes(status.AttachmentsSize),
				"size_bytes": status.AttachmentsSize,
				"count":      status.AttachmentCount,
				"percentage": s.calculatePercentage(status.AttachmentsSize, status.TotalSize),
			},
			"history": map[string]interface{}{
				"size":       entities.FormatBytes(status.HistorySize),
				"size_bytes": status.HistorySize,
				"percentage": s.calculatePercentage(status.HistorySize, status.TotalSize),
			},
			"time_entries": map[string]interface{}{
				"size":       entities.FormatBytes(status.TimeEntriesSize),
				"size_bytes": status.TimeEntriesSize,
				"percentage": s.calculatePercentage(status.TimeEntriesSize, status.TotalSize),
			},
			"metadata": map[string]interface{}{
				"size":       entities.FormatBytes(status.MetadataSize),
				"size_bytes": status.MetadataSize,
				"percentage": s.calculatePercentage(status.MetadataSize, status.TotalSize),
			},
		},
		"disk_space": map[string]interface{}{
			"available":       entities.FormatBytes(status.AvailableDiskSpace),
			"available_bytes": status.AvailableDiskSpace,
		},
		"quotas": map[string]interface{}{
			"project_limit":    entities.FormatBytes(s.config.MaxProjectSize),
			"attachment_limit": entities.FormatBytes(s.config.MaxAttachmentSize),
			"usage_percentage": fmt.Sprintf("%.1f%%", status.UsagePercentage),
			"enforce_quotas":   s.config.EnforceQuotas,
		},
		"health": map[string]interface{}{
			"status":   status.Status,
			"warnings": status.Warnings,
		},
	}

	return breakdown, nil
}

// calculatePercentage calculates percentage safely
func (s *StorageService) calculatePercentage(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}
