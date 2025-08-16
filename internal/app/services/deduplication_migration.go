package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// DeduplicationMigration handles migration of existing files to deduplicated storage
type DeduplicationMigration struct {
	dedupService    *DeduplicationService
	attachmentRepo  repositories.AttachmentRepository
	basePath        string
	attachmentsPath string
}

// NewDeduplicationMigration creates a new migration service
func NewDeduplicationMigration(
	dedupService *DeduplicationService,
	attachmentRepo repositories.AttachmentRepository,
	basePath string,
) *DeduplicationMigration {
	return &DeduplicationMigration{
		dedupService:    dedupService,
		attachmentRepo:  attachmentRepo,
		basePath:        basePath,
		attachmentsPath: filepath.Join(basePath, "attachments"),
	}
}

// MigrateExistingFiles migrates existing files to deduplicated storage
func (m *DeduplicationMigration) MigrateExistingFiles(ctx context.Context, dryRun bool) (*entities.MigrationResult, error) {
	start := time.Now()
	result := &entities.MigrationResult{
		Timestamp: start,
		DryRun:    dryRun,
		Errors:    []string{},
	}

	// Find potential duplicates
	duplicateGroups, err := m.dedupService.FindPotentialDuplicates(m.attachmentsPath)
	if err != nil {
		return result, errors.Wrap(err, "DeduplicationMigration.MigrateExistingFiles", "failed to find duplicates")
	}

	// Process each duplicate group
	for _, group := range duplicateGroups {
		migrated, removed, spaceReclaimed, err := m.migrateDuplicateGroup(ctx, &group, dryRun)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to migrate group %s: %v", group.Hash, err))
			continue
		}

		result.FilesMigrated += migrated
		result.DuplicatesRemoved += removed
		result.SpaceReclaimed += spaceReclaimed
	}

	result.Duration = time.Since(start)
	return result, nil
}

// migrateDuplicateGroup migrates a group of duplicate files
func (m *DeduplicationMigration) migrateDuplicateGroup(ctx context.Context, group *entities.DuplicateGroup, dryRun bool) (int, int, int64, error) {
	if len(group.Files) <= 1 {
		return 0, 0, 0, nil // No duplicates to migrate
	}

	// Use the first file as the master copy
	masterFile := &group.Files[0]

	// Create or get file hash entry
	fileHash, isNew, err := m.dedupService.GetOrCreateFileHash(
		group.Hash,
		group.Size,
		masterFile.Filename,
		masterFile.ContentType,
	)
	if err != nil {
		return 0, 0, 0, err
	}

	var migrated, removed int
	var spaceReclaimed int64

	// If this is a new hash, move the master file to deduplicated storage
	if isNew && !dryRun {
		// Ensure target directory exists
		targetDir := filepath.Dir(fileHash.StoragePath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return 0, 0, 0, err
		}

		// Move master file to deduplicated location
		if err := m.moveFile(masterFile.StoragePath, fileHash.StoragePath); err != nil {
			return 0, 0, 0, err
		}
		migrated++
	}

	// Process all files (including master)
	for i, file := range group.Files {
		// Add reference for this attachment
		if !dryRun {
			err := m.dedupService.AddReference(
				file.AttachmentID,
				file.IssueID,
				group.Hash,
				file.Filename,
			)
			if err != nil {
				return migrated, removed, spaceReclaimed, err
			}
		}

		// Remove duplicate files (all except master if it was moved)
		if i > 0 || !isNew {
			if !dryRun {
				if err := os.Remove(file.StoragePath); err != nil && !os.IsNotExist(err) {
					return migrated, removed, spaceReclaimed, err
				}
			}
			removed++
			spaceReclaimed += group.Size
		}
	}

	return migrated, removed, spaceReclaimed, nil
}

// moveFile moves a file from source to destination
func (m *DeduplicationMigration) moveFile(src, dst string) error {
	// Try rename first (fastest if on same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// If rename fails, copy and delete
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Sync to ensure data is written
	if err := dstFile.Sync(); err != nil {
		return err
	}

	// Remove source file
	return os.Remove(src)
}

// GenerateMigrationReport generates a detailed migration report
func (m *DeduplicationMigration) GenerateMigrationReport(ctx context.Context) (*entities.DeduplicationReport, error) {
	start := time.Now()

	// Get current stats
	stats, err := m.dedupService.GetDeduplicationStats()
	if err != nil {
		return nil, err
	}

	// Find potential duplicates
	duplicateGroups, err := m.dedupService.FindPotentialDuplicates(m.attachmentsPath)
	if err != nil {
		return nil, err
	}

	report := &entities.DeduplicationReport{
		Timestamp:           start,
		Stats:               *stats,
		Config:              m.dedupService.GetConfig(),
		PotentialDuplicates: duplicateGroups,
		Duration:            time.Since(start),
	}

	return report, nil
}

// ValidateMigration validates the integrity of migrated files
func (m *DeduplicationMigration) ValidateMigration(ctx context.Context) ([]string, error) {
	var errors []string

	// Walk through deduplicated storage and verify files
	dedupPath := filepath.Join(m.basePath, "dedup")
	err := filepath.Walk(dedupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip index files
		if filepath.Dir(path) == filepath.Join(dedupPath, "index") {
			return nil
		}

		// Calculate hash and verify it matches the filename
		hash, _, err := m.dedupService.CalculateFileHashFromPath(path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to hash file %s: %v", path, err))
			return nil
		}

		// Extract expected hash from path
		relPath, _ := filepath.Rel(dedupPath, path)
		expectedHash := filepath.Dir(relPath) + filepath.Base(relPath)

		if hash != expectedHash {
			errors = append(errors, fmt.Sprintf("hash mismatch for file %s: expected %s, got %s", path, expectedHash, hash))
		}

		return nil
	})

	if err != nil {
		errors = append(errors, fmt.Sprintf("failed to walk dedup directory: %v", err))
	}

	return errors, nil
}
