package services

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// AdminCleanupService provides admin-level cleanup operations
type AdminCleanupService struct {
	cleanupService *CleanupService
	basePath       string
	configRepo     repositories.ConfigRepository
	issueRepo      repositories.IssueRepository
	attachmentRepo repositories.AttachmentRepository
	logger         *CleanupLogger
}

// NewAdminCleanupService creates a new admin cleanup service
func NewAdminCleanupService(
	cleanupService *CleanupService,
	basePath string,
	configRepo repositories.ConfigRepository,
	issueRepo repositories.IssueRepository,
	attachmentRepo repositories.AttachmentRepository,
) *AdminCleanupService {
	return &AdminCleanupService{
		cleanupService: cleanupService,
		basePath:       basePath,
		configRepo:     configRepo,
		issueRepo:      issueRepo,
		attachmentRepo: attachmentRepo,
		logger:         NewCleanupLogger(basePath),
	}
}

// RunAdminCleanup performs targeted cleanup based on admin options
func (s *AdminCleanupService) RunAdminCleanup(ctx context.Context, options *entities.AdminCleanupOptions) (*entities.AdminCleanupResult, error) {
	start := time.Now()

	// Determine cleanup target
	target := s.determineCleanupTarget(options)

	// Create filter criteria map for reporting
	filterCriteria := s.buildFilterCriteria(options)

	// Create backup if needed
	var backupLocation string
	var backupCreated bool
	if !options.NoBackup && !options.DryRun {
		location, err := s.createBackup(ctx, target, options)
		if err != nil {
			return nil, errors.Wrap(err, "AdminCleanupService.RunAdminCleanup", "failed to create backup")
		}
		backupLocation = location
		backupCreated = true
	}

	// Get confirmation if needed
	confirmationUsed := false
	if !options.ForceConfirm && !options.DryRun {
		confirmed, err := s.getConfirmation(target, filterCriteria)
		if err != nil {
			return nil, errors.Wrap(err, "AdminCleanupService.RunAdminCleanup", "confirmation failed")
		}
		if !confirmed {
			return &entities.AdminCleanupResult{
				CleanupResult: &entities.CleanupResult{
					Timestamp: start,
					DryRun:    false,
					Errors:    []string{"Operation cancelled by user"},
					Duration:  time.Since(start),
				},
				Target:           target,
				BackupCreated:    false,
				ConfirmationUsed: true,
				FilterCriteria:   filterCriteria,
			}, nil
		}
		confirmationUsed = true
	}

	// Perform the cleanup based on target
	var result *entities.CleanupResult
	var err error

	switch target {
	case entities.CleanupTargetClosedIssues:
		result, err = s.cleanupClosedIssuesWithOptions(ctx, options)
	case entities.CleanupTargetOrphanedAttachments:
		result, err = s.cleanupOrphanedAttachmentsWithOptions(ctx, options)
	case entities.CleanupTargetTimeEntries:
		result, err = s.cleanupTimeEntriesWithOptions(ctx, options)
	case entities.CleanupTargetHistory:
		result, err = s.cleanupHistoryWithOptions(ctx, options)
	case entities.CleanupTargetEmptyDirectories:
		result, err = s.cleanupEmptyDirectoriesWithOptions(ctx, options)
	default:
		// Default to regular cleanup with applied filters
		result, err = s.cleanupWithGlobalFilter(ctx, options)
	}

	if err != nil {
		return nil, errors.Wrap(err, "AdminCleanupService.RunAdminCleanup", "cleanup operation failed")
	}

	// Build admin result
	adminResult := &entities.AdminCleanupResult{
		CleanupResult:    result,
		Target:           target,
		BackupCreated:    backupCreated,
		BackupLocation:   backupLocation,
		ConfirmationUsed: confirmationUsed,
		FilterCriteria:   filterCriteria,
	}

	// Log the operation
	if logErr := s.logger.LogAdminCleanupOperation(adminResult, err); logErr != nil {
		// Don't fail the operation due to logging errors, just log them
		fmt.Printf("Warning: failed to log cleanup operation: %v\n", logErr)
	}

	return adminResult, nil
}

// determineCleanupTarget determines what should be cleaned based on options
func (s *AdminCleanupService) determineCleanupTarget(options *entities.AdminCleanupOptions) entities.CleanupTarget {
	if options.ClosedOnly {
		return entities.CleanupTargetClosedIssues
	}
	if options.OrphanedOnly {
		return entities.CleanupTargetOrphanedAttachments
	}
	if options.TimeEntriesOnly {
		return entities.CleanupTargetTimeEntries
	}
	return entities.CleanupTargetAll
}

// buildFilterCriteria builds a map of filter criteria for reporting
func (s *AdminCleanupService) buildFilterCriteria(options *entities.AdminCleanupOptions) map[string]interface{} {
	criteria := make(map[string]interface{})

	if options.OlderThan != nil {
		criteria["older_than"] = options.OlderThan.String()
	}
	if options.ClosedOnly {
		criteria["closed_only"] = true
		if options.ClosedDays != nil {
			criteria["closed_days"] = *options.ClosedDays
		}
	}
	if options.OrphanedOnly {
		criteria["orphaned_only"] = true
	}
	if options.TimeEntriesOnly {
		criteria["time_entries_only"] = true
		if options.TimeEntriesBefore != nil {
			criteria["time_entries_before"] = options.TimeEntriesBefore.Format("2006-01-02")
		}
	}
	criteria["dry_run"] = options.DryRun
	criteria["no_backup"] = options.NoBackup

	return criteria
}

// createBackup creates a backup of items that will be deleted
func (s *AdminCleanupService) createBackup(ctx context.Context, target entities.CleanupTarget, options *entities.AdminCleanupOptions) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	backupDir := filepath.Join(s.basePath, "backups", fmt.Sprintf("cleanup_%s_%s", target, timestamp))

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", errors.Wrap(err, "AdminCleanupService.createBackup", "failed to create backup directory")
	}

	// TODO: Implement specific backup logic based on target
	// For now, just create the directory structure

	return backupDir, nil
}

// getConfirmation prompts user for confirmation
func (s *AdminCleanupService) getConfirmation(target entities.CleanupTarget, criteria map[string]interface{}) (bool, error) {
	fmt.Printf("⚠️  You are about to perform a %s cleanup operation.\n", target)
	fmt.Println("Filter criteria:")
	for key, value := range criteria {
		fmt.Printf("  %s: %v\n", key, value)
	}
	fmt.Print("\nThis operation cannot be undone. Continue? (y/N): ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// parseDuration parses duration strings like "30d", "6m", "1y"
func (s *AdminCleanupService) parseDuration(duration string) (time.Duration, error) {
	if duration == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Handle common patterns
	re := regexp.MustCompile(`^(\d+)([dmy])$`)
	matches := re.FindStringSubmatch(strings.ToLower(duration))

	if len(matches) != 3 {
		// Try standard Go duration parsing
		return time.ParseDuration(duration)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid number in duration: %s", matches[1])
	}

	unit := matches[2]
	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "m":
		return time.Duration(value) * 30 * 24 * time.Hour, nil // Approximate month
	case "y":
		return time.Duration(value) * 365 * 24 * time.Hour, nil // Approximate year
	default:
		return 0, fmt.Errorf("unknown duration unit: %s", unit)
	}
}

// cleanupClosedIssuesWithOptions performs targeted closed issue cleanup
func (s *AdminCleanupService) cleanupClosedIssuesWithOptions(ctx context.Context, options *entities.AdminCleanupOptions) (*entities.CleanupResult, error) {
	result := &entities.CleanupResult{
		Timestamp:    time.Now(),
		DryRun:       options.DryRun,
		ItemsCleaned: entities.CleanupStats{},
		Errors:       []string{},
	}

	// Determine cutoff date
	var cutoffDate time.Time
	if options.ClosedDays != nil {
		cutoffDate = time.Now().Add(-time.Duration(*options.ClosedDays) * 24 * time.Hour)
	} else if options.OlderThan != nil {
		cutoffDate = time.Now().Add(-*options.OlderThan)
	} else {
		// Use default config
		config := s.cleanupService.GetConfig()
		cutoffDate = time.Now().Add(-time.Duration(config.RetentionDays.ClosedIssues) * 24 * time.Hour)
	}

	// Get closed issues
	closedStatus := entities.StatusClosed
	filter := repositories.IssueFilter{
		Status: &closedStatus,
	}

	issueList, err := s.issueRepo.List(ctx, filter)
	if err != nil {
		return result, errors.Wrap(err, "AdminCleanupService.cleanupClosedIssuesWithOptions", "failed to list closed issues")
	}

	var spaceReclaimed int64

	for _, issue := range issueList.Issues {
		// Check if issue meets criteria
		if issue.Timestamps.Updated.After(cutoffDate) {
			continue
		}

		if !options.DryRun {
			// Delete issue attachments
			for _, attachment := range issue.Attachments {
				if attachInfo, err := os.Stat(attachment.StoragePath); err == nil {
					spaceReclaimed += attachInfo.Size()
				}
				if err := s.attachmentRepo.DeleteFile(ctx, attachment.StoragePath); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("failed to delete attachment %s: %v", attachment.ID, err))
				} else {
					result.ItemsCleaned.Attachments++
				}
			}

			// Delete the issue
			if err := s.issueRepo.Delete(ctx, issue.ID); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to delete issue %s: %v", issue.ID, err))
				continue
			}
		}

		result.ItemsCleaned.ClosedIssues++
	}

	result.SpaceReclaimed = spaceReclaimed
	result.ItemsCleaned.Total = result.ItemsCleaned.ClosedIssues + result.ItemsCleaned.Attachments
	result.Duration = time.Since(result.Timestamp)

	return result, nil
}

// cleanupOrphanedAttachmentsWithOptions performs targeted orphaned attachment cleanup
func (s *AdminCleanupService) cleanupOrphanedAttachmentsWithOptions(ctx context.Context, options *entities.AdminCleanupOptions) (*entities.CleanupResult, error) {
	result := &entities.CleanupResult{
		Timestamp:    time.Now(),
		DryRun:       options.DryRun,
		ItemsCleaned: entities.CleanupStats{},
		Errors:       []string{},
	}

	// Determine cutoff date
	var cutoffDate time.Time
	if options.OlderThan != nil {
		cutoffDate = time.Now().Add(-*options.OlderThan)
	} else {
		// Use default config
		config := s.cleanupService.GetConfig()
		cutoffDate = time.Now().Add(-time.Duration(config.RetentionDays.OrphanedAttachments) * 24 * time.Hour)
	}

	attachmentsPath := filepath.Join(s.basePath, "attachments")
	if _, err := os.Stat(attachmentsPath); os.IsNotExist(err) {
		result.Duration = time.Since(result.Timestamp)
		return result, nil
	}

	var spaceReclaimed int64

	err := filepath.Walk(attachmentsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check if file meets age criteria
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
		if _, err := s.issueRepo.GetByID(ctx, entities.IssueID(issueID)); err != nil {
			// Issue doesn't exist, this is an orphaned attachment
			if !options.DryRun {
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
		result.Errors = append(result.Errors, fmt.Sprintf("failed to walk attachments directory: %v", err))
	}

	result.SpaceReclaimed = spaceReclaimed
	result.ItemsCleaned.Total = result.ItemsCleaned.OrphanedAttachments
	result.Duration = time.Since(result.Timestamp)

	return result, nil
}

// cleanupTimeEntriesWithOptions performs targeted time entry cleanup
func (s *AdminCleanupService) cleanupTimeEntriesWithOptions(ctx context.Context, options *entities.AdminCleanupOptions) (*entities.CleanupResult, error) {
	result := &entities.CleanupResult{
		Timestamp:    time.Now(),
		DryRun:       options.DryRun,
		ItemsCleaned: entities.CleanupStats{},
		Errors:       []string{},
	}

	// Determine cutoff date
	var cutoffDate time.Time
	if options.TimeEntriesBefore != nil {
		cutoffDate = *options.TimeEntriesBefore
	} else if options.OlderThan != nil {
		cutoffDate = time.Now().Add(-*options.OlderThan)
	} else {
		// Use default config
		config := s.cleanupService.GetConfig()
		cutoffDate = time.Now().Add(-time.Duration(config.RetentionDays.TimeEntries) * 24 * time.Hour)
	}

	timeEntriesPath := filepath.Join(s.basePath, "time_entries")
	if _, err := os.Stat(timeEntriesPath); os.IsNotExist(err) {
		result.Duration = time.Since(result.Timestamp)
		return result, nil
	}

	var spaceReclaimed int64

	err := filepath.Walk(timeEntriesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check if file meets criteria
		if info.ModTime().After(cutoffDate) {
			return nil
		}

		if !options.DryRun {
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
		result.Errors = append(result.Errors, fmt.Sprintf("failed to walk time entries directory: %v", err))
	}

	result.SpaceReclaimed = spaceReclaimed
	result.ItemsCleaned.Total = result.ItemsCleaned.TimeEntries
	result.Duration = time.Since(result.Timestamp)

	return result, nil
}

// cleanupHistoryWithOptions performs targeted history cleanup
func (s *AdminCleanupService) cleanupHistoryWithOptions(ctx context.Context, options *entities.AdminCleanupOptions) (*entities.CleanupResult, error) {
	result := &entities.CleanupResult{
		Timestamp:    time.Now(),
		DryRun:       options.DryRun,
		ItemsCleaned: entities.CleanupStats{},
		Errors:       []string{},
	}

	// Similar implementation to time entries
	// TODO: Implement history-specific cleanup logic

	result.Duration = time.Since(result.Timestamp)
	return result, nil
}

// cleanupEmptyDirectoriesWithOptions performs targeted empty directory cleanup
func (s *AdminCleanupService) cleanupEmptyDirectoriesWithOptions(ctx context.Context, options *entities.AdminCleanupOptions) (*entities.CleanupResult, error) {
	result := &entities.CleanupResult{
		Timestamp:    time.Now(),
		DryRun:       options.DryRun,
		ItemsCleaned: entities.CleanupStats{},
		Errors:       []string{},
	}

	dirs := []string{"attachments", "history", "time_entries", "issues"}

	for _, dir := range dirs {
		dirPath := filepath.Join(s.basePath, dir)
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
				if !options.DryRun {
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

	result.ItemsCleaned.Total = result.ItemsCleaned.EmptyDirectories
	result.Duration = time.Since(result.Timestamp)

	return result, nil
}

// cleanupWithGlobalFilter performs cleanup with global age filter
func (s *AdminCleanupService) cleanupWithGlobalFilter(ctx context.Context, options *entities.AdminCleanupOptions) (*entities.CleanupResult, error) {
	// Apply global older-than filter to regular cleanup
	if options.OlderThan != nil {
		// TODO: Modify cleanup service to accept age override
		// For now, delegate to regular cleanup
		return s.cleanupService.RunCleanup(ctx, options.DryRun)
	}

	return s.cleanupService.RunCleanup(ctx, options.DryRun)
}
