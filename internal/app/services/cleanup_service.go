package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// CleanupService handles data cleanup and retention
type CleanupService struct {
	basePath       string
	configRepo     repositories.ConfigRepository
	issueRepo      repositories.IssueRepository
	attachmentRepo repositories.AttachmentRepository
	config         *entities.CleanupConfig
	mu             sync.RWMutex
	lastCleanup    time.Time
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(
	basePath string,
	configRepo repositories.ConfigRepository,
	issueRepo repositories.IssueRepository,
	attachmentRepo repositories.AttachmentRepository,
) *CleanupService {
	config := entities.DefaultCleanupConfig()

	// Try to load config from repository
	if configRepo != nil {
		if cfg, err := configRepo.Load(context.Background()); err == nil && cfg != nil {
			if cfg.StorageConfig != nil && cfg.StorageConfig.CleanupConfig != nil {
				config = cfg.StorageConfig.CleanupConfig
			}
		}
	}

	return &CleanupService{
		basePath:       basePath,
		configRepo:     configRepo,
		issueRepo:      issueRepo,
		attachmentRepo: attachmentRepo,
		config:         config,
	}
}

// RunCleanup performs a cleanup operation according to configuration
func (c *CleanupService) RunCleanup(ctx context.Context, dryRun bool) (*entities.CleanupResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	start := time.Now()
	result := &entities.CleanupResult{
		Timestamp:    start,
		DryRun:       dryRun || c.config.DryRunMode,
		ItemsCleaned: entities.CleanupStats{},
		Errors:       []string{},
	}

	// Check if cleanup is enabled
	if !c.config.Enabled {
		result.Errors = append(result.Errors, "cleanup is disabled in configuration")
		result.Duration = time.Since(start)
		return result, nil
	}

	var totalSpaceReclaimed int64

	// Clean up closed issues and their attachments
	if spaceReclaimed, err := c.cleanupClosedIssues(ctx, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to cleanup closed issues: %v", err))
	} else {
		totalSpaceReclaimed += spaceReclaimed
	}

	// Clean up orphaned attachments
	if spaceReclaimed, err := c.cleanupOrphanedAttachments(ctx, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to cleanup orphaned attachments: %v", err))
	} else {
		totalSpaceReclaimed += spaceReclaimed
	}

	// Clean up old history entries
	if spaceReclaimed, err := c.cleanupHistory(ctx, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to cleanup history: %v", err))
	} else {
		totalSpaceReclaimed += spaceReclaimed
	}

	// Clean up old time entries
	if spaceReclaimed, err := c.cleanupTimeEntries(ctx, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to cleanup time entries: %v", err))
	} else {
		totalSpaceReclaimed += spaceReclaimed
	}

	// Clean up empty directories
	if spaceReclaimed, err := c.cleanupEmptyDirectories(ctx, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to cleanup empty directories: %v", err))
	} else {
		totalSpaceReclaimed += spaceReclaimed
	}

	result.SpaceReclaimed = totalSpaceReclaimed
	result.ItemsCleaned.Total = result.ItemsCleaned.ClosedIssues +
		result.ItemsCleaned.Attachments +
		result.ItemsCleaned.OrphanedAttachments +
		result.ItemsCleaned.HistoryEntries +
		result.ItemsCleaned.TimeEntries +
		result.ItemsCleaned.EmptyDirectories

	result.Duration = time.Since(start)
	c.lastCleanup = start

	return result, nil
}

// cleanupClosedIssues removes old closed issues and their attachments
func (c *CleanupService) cleanupClosedIssues(ctx context.Context, result *entities.CleanupResult) (int64, error) {
	if c.config.RetentionDays.ClosedIssues <= 0 {
		return 0, nil
	}

	closedStatus := entities.StatusClosed
	filter := repositories.IssueFilter{
		Status: &closedStatus,
	}

	issueList, err := c.issueRepo.List(ctx, filter)
	if err != nil {
		return 0, errors.Wrap(err, "CleanupService.cleanupClosedIssues", "failed to list closed issues")
	}

	var spaceReclaimed int64
	cutoffDate := time.Now().Add(-time.Duration(c.config.RetentionDays.ClosedIssues) * 24 * time.Hour)

	// Sort issues by modification time to keep the most recent ones
	sort.Slice(issueList.Issues, func(i, j int) bool {
		return issueList.Issues[i].Timestamps.Updated.After(issueList.Issues[j].Timestamps.Updated)
	})

	keptCount := 0
	for _, issue := range issueList.Issues {
		// Always keep minimum number of closed issues
		if keptCount < c.config.MinimumKeep.ClosedIssues {
			keptCount++
			continue
		}

		// Check if issue is old enough to be cleaned up
		if issue.Timestamps.Updated.After(cutoffDate) {
			continue
		}

		if !result.DryRun {
			// Archive issue if enabled
			if c.config.ArchiveBeforeDelete {
				if err := c.archiveIssue(ctx, &issue); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("failed to archive issue %s: %v", issue.ID, err))
					continue
				}
				result.ItemsArchived++
			}

			// Delete issue attachments first
			for _, attachment := range issue.Attachments {
				if attachInfo, err := os.Stat(attachment.StoragePath); err == nil {
					spaceReclaimed += attachInfo.Size()
				}
				if err := c.attachmentRepo.DeleteFile(ctx, attachment.StoragePath); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("failed to delete attachment %s: %v", attachment.ID, err))
				} else {
					result.ItemsCleaned.Attachments++
				}
			}

			// Delete the issue
			if err := c.issueRepo.Delete(ctx, issue.ID); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to delete issue %s: %v", issue.ID, err))
				continue
			}
		}

		result.ItemsCleaned.ClosedIssues++
	}

	return spaceReclaimed, nil
}

// cleanupOrphanedAttachments removes attachments that don't belong to any issue
func (c *CleanupService) cleanupOrphanedAttachments(ctx context.Context, result *entities.CleanupResult) (int64, error) {
	if c.config.RetentionDays.OrphanedAttachments <= 0 {
		return 0, nil
	}

	attachmentsPath := filepath.Join(c.basePath, "attachments")
	if _, err := os.Stat(attachmentsPath); os.IsNotExist(err) {
		return 0, nil
	}

	var spaceReclaimed int64
	cutoffDate := time.Now().Add(-time.Duration(c.config.RetentionDays.OrphanedAttachments) * 24 * time.Hour)

	err := filepath.Walk(attachmentsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check if file is old enough
		if info.ModTime().After(cutoffDate) {
			return nil
		}

		// Extract issue ID from path
		relPath, _ := filepath.Rel(attachmentsPath, path)
		pathParts := strings.Split(relPath, string(filepath.Separator))
		if len(pathParts) < 2 {
			return nil
		}

		issueID := pathParts[0]

		// Check if issue exists
		if _, err := c.issueRepo.GetByID(ctx, entities.IssueID(issueID)); err != nil {
			// Issue doesn't exist, this is an orphaned attachment
			if !result.DryRun {
				if err := os.Remove(path); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("failed to remove orphaned attachment %s: %v", path, err))
					return nil
				}
			}

			spaceReclaimed += info.Size()
			result.ItemsCleaned.OrphanedAttachments++
		}

		return nil
	})

	if err != nil {
		return spaceReclaimed, errors.Wrap(err, "CleanupService.cleanupOrphanedAttachments", "failed to walk attachments directory")
	}

	return spaceReclaimed, nil
}

// cleanupHistory removes old history entries
func (c *CleanupService) cleanupHistory(ctx context.Context, result *entities.CleanupResult) (int64, error) {
	if c.config.RetentionDays.History <= 0 {
		return 0, nil
	}

	historyPath := filepath.Join(c.basePath, "history")
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		return 0, nil
	}

	var spaceReclaimed int64
	cutoffDate := time.Now().Add(-time.Duration(c.config.RetentionDays.History) * 24 * time.Hour)

	err := filepath.Walk(historyPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check if file is old enough
		if info.ModTime().After(cutoffDate) {
			return nil
		}

		// TODO: Implement per-issue minimum history keeping logic
		// For now, just clean based on age

		if !result.DryRun {
			if err := os.Remove(path); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to remove history file %s: %v", path, err))
				return nil
			}
		}

		spaceReclaimed += info.Size()
		result.ItemsCleaned.HistoryEntries++
		return nil
	})

	if err != nil {
		return spaceReclaimed, errors.Wrap(err, "CleanupService.cleanupHistory", "failed to walk history directory")
	}

	return spaceReclaimed, nil
}

// cleanupTimeEntries removes old time entries
func (c *CleanupService) cleanupTimeEntries(ctx context.Context, result *entities.CleanupResult) (int64, error) {
	if c.config.RetentionDays.TimeEntries <= 0 {
		return 0, nil
	}

	timeEntriesPath := filepath.Join(c.basePath, "time_entries")
	if _, err := os.Stat(timeEntriesPath); os.IsNotExist(err) {
		return 0, nil
	}

	var spaceReclaimed int64
	cutoffDate := time.Now().Add(-time.Duration(c.config.RetentionDays.TimeEntries) * 24 * time.Hour)

	err := filepath.Walk(timeEntriesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check if file is old enough
		if info.ModTime().After(cutoffDate) {
			return nil
		}

		if !result.DryRun {
			if err := os.Remove(path); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to remove time entry file %s: %v", path, err))
				return nil
			}
		}

		spaceReclaimed += info.Size()
		result.ItemsCleaned.TimeEntries++
		return nil
	})

	if err != nil {
		return spaceReclaimed, errors.Wrap(err, "CleanupService.cleanupTimeEntries", "failed to walk time entries directory")
	}

	return spaceReclaimed, nil
}

// cleanupEmptyDirectories removes empty directories
func (c *CleanupService) cleanupEmptyDirectories(ctx context.Context, result *entities.CleanupResult) (int64, error) {
	if !c.config.RetentionDays.EmptyDirectories {
		return 0, nil
	}

	dirs := []string{"attachments", "history", "time_entries", "issues"}
	var spaceReclaimed int64

	for _, dir := range dirs {
		dirPath := filepath.Join(c.basePath, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || !info.IsDir() || path == dirPath {
				return nil
			}

			// Check if directory is empty
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil
			}

			if len(entries) == 0 {
				if !result.DryRun {
					if err := os.Remove(path); err != nil {
						result.Errors = append(result.Errors, fmt.Sprintf("failed to remove empty directory %s: %v", path, err))
						return nil
					}
				}
				result.ItemsCleaned.EmptyDirectories++
			}

			return nil
		})

		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to walk directory %s: %v", dirPath, err))
		}
	}

	return spaceReclaimed, nil
}

// archiveIssue archives an issue before deletion (placeholder implementation)
func (c *CleanupService) archiveIssue(ctx context.Context, issue *entities.Issue) error {
	// TODO: Implement archiving logic
	// For now, just return nil
	return nil
}

// GetConfig returns current cleanup configuration
func (c *CleanupService) GetConfig() *entities.CleanupConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// UpdateConfig updates the cleanup configuration
func (c *CleanupService) UpdateConfig(config *entities.CleanupConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.config = config

	// Save to config repository
	if c.configRepo != nil {
		cfg, err := c.configRepo.Load(context.Background())
		if err != nil {
			cfg = &entities.Config{}
		}

		if cfg.StorageConfig == nil {
			cfg.StorageConfig = entities.DefaultStorageConfig()
		}
		cfg.StorageConfig.CleanupConfig = config

		if err := c.configRepo.Save(context.Background(), cfg); err != nil {
			return errors.Wrap(err, "CleanupService.UpdateConfig", "failed to save config")
		}
	}

	return nil
}

// GetLastCleanup returns the timestamp of the last cleanup
func (c *CleanupService) GetLastCleanup() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastCleanup
}

// ShouldRunCleanup checks if cleanup should be triggered
func (c *CleanupService) ShouldRunCleanup(storageStatus *entities.StorageStatus) bool {
	if !c.config.Enabled {
		return false
	}

	// Check size-based triggers
	if c.config.SizeTriggers.MaxTotalSize > 0 &&
		c.config.NeedsCleanup(storageStatus.TotalSize, c.config.SizeTriggers.MaxTotalSize) {
		return true
	}

	if c.config.SizeTriggers.MaxAttachmentsSize > 0 &&
		c.config.NeedsCleanup(storageStatus.AttachmentsSize, c.config.SizeTriggers.MaxAttachmentsSize) {
		return true
	}

	if c.config.SizeTriggers.MaxHistorySize > 0 &&
		c.config.NeedsCleanup(storageStatus.HistorySize, c.config.SizeTriggers.MaxHistorySize) {
		return true
	}

	return false
}
