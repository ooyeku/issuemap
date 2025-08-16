package services

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// AttachmentMaintenance provides cleanup and maintenance operations for attachments
type AttachmentMaintenance struct {
	attachmentRepo repositories.AttachmentRepository
	basePath       string
}

// NewAttachmentMaintenance creates a new attachment maintenance service
func NewAttachmentMaintenance(attachmentRepo repositories.AttachmentRepository, basePath string) *AttachmentMaintenance {
	return &AttachmentMaintenance{
		attachmentRepo: attachmentRepo,
		basePath:       basePath,
	}
}

// CleanupOrphanedFiles removes files that don't have corresponding metadata
func (m *AttachmentMaintenance) CleanupOrphanedFiles(ctx context.Context, dryRun bool) (int, error) {
	attachmentsDir := filepath.Join(m.basePath, "attachments")
	metadataDir := filepath.Join(attachmentsDir, ".metadata")

	// Get all metadata files to build a map of valid attachments
	validAttachments := make(map[string]bool)

	if metadataFiles, err := ioutil.ReadDir(metadataDir); err == nil {
		for _, file := range metadataFiles {
			if strings.HasSuffix(file.Name(), ".yaml") {
				attachmentID := strings.TrimSuffix(file.Name(), ".yaml")

				// Get attachment metadata to find storage path
				if attachment, err := m.attachmentRepo.GetMetadata(ctx, attachmentID); err == nil {
					validAttachments[attachment.StoragePath] = true
				}
			}
		}
	}

	orphanedCount := 0

	// Walk through all attachment files
	err := filepath.Walk(attachmentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and metadata directory
		if info.IsDir() || strings.Contains(path, ".metadata") {
			return nil
		}

		// Get relative path from base
		relPath, err := filepath.Rel(m.basePath, path)
		if err != nil {
			log.Printf("Error getting relative path for %s: %v", path, err)
			return nil
		}

		// Check if this file has valid metadata
		if !validAttachments[relPath] {
			log.Printf("Found orphaned file: %s", relPath)
			orphanedCount++

			if !dryRun {
				if err := os.Remove(path); err != nil {
					log.Printf("Error removing orphaned file %s: %v", path, err)
				} else {
					log.Printf("Removed orphaned file: %s", relPath)
				}
			}
		}

		return nil
	})

	return orphanedCount, err
}

// CleanupOrphanedMetadata removes metadata that doesn't have corresponding files
func (m *AttachmentMaintenance) CleanupOrphanedMetadata(ctx context.Context, dryRun bool) (int, error) {
	metadataDir := filepath.Join(m.basePath, "attachments", ".metadata")

	metadataFiles, err := ioutil.ReadDir(metadataDir)
	if err != nil {
		return 0, err
	}

	orphanedCount := 0

	for _, file := range metadataFiles {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		attachmentID := strings.TrimSuffix(file.Name(), ".yaml")

		// Get attachment metadata
		attachment, err := m.attachmentRepo.GetMetadata(ctx, attachmentID)
		if err != nil {
			log.Printf("Error reading metadata for %s: %v", attachmentID, err)
			continue
		}

		// Check if corresponding file exists
		fullPath := filepath.Join(m.basePath, attachment.StoragePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			log.Printf("Found orphaned metadata: %s (file: %s)", attachmentID, attachment.StoragePath)
			orphanedCount++

			if !dryRun {
				if err := m.attachmentRepo.DeleteMetadata(ctx, attachmentID); err != nil {
					log.Printf("Error removing orphaned metadata %s: %v", attachmentID, err)
				} else {
					log.Printf("Removed orphaned metadata: %s", attachmentID)
				}
			}
		}
	}

	return orphanedCount, nil
}

// CleanupOldTempFiles removes temporary files older than specified duration
func (m *AttachmentMaintenance) CleanupOldTempFiles(ctx context.Context, maxAge time.Duration, dryRun bool) (int, error) {
	tempDir := filepath.Join(m.basePath, "temp")

	// Check if temp directory exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		return 0, nil
	}

	cutoff := time.Now().Add(-maxAge)
	cleanedCount := 0

	err := filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.ModTime().Before(cutoff) {
			log.Printf("Found old temp file: %s (age: %v)", path, time.Since(info.ModTime()))
			cleanedCount++

			if !dryRun {
				if err := os.Remove(path); err != nil {
					log.Printf("Error removing old temp file %s: %v", path, err)
				} else {
					log.Printf("Removed old temp file: %s", path)
				}
			}
		}

		return nil
	})

	return cleanedCount, err
}

// ValidateAttachmentIntegrity checks for corruption in attachment files
func (m *AttachmentMaintenance) ValidateAttachmentIntegrity(ctx context.Context) (map[string]error, error) {
	metadataDir := filepath.Join(m.basePath, "attachments", ".metadata")

	metadataFiles, err := ioutil.ReadDir(metadataDir)
	if err != nil {
		return nil, err
	}

	corruptedFiles := make(map[string]error)

	for _, file := range metadataFiles {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		attachmentID := strings.TrimSuffix(file.Name(), ".yaml")

		// Get attachment metadata
		attachment, err := m.attachmentRepo.GetMetadata(ctx, attachmentID)
		if err != nil {
			corruptedFiles[attachmentID] = err
			continue
		}

		// Check if file exists and has correct size
		fullPath := filepath.Join(m.basePath, attachment.StoragePath)
		info, err := os.Stat(fullPath)
		if err != nil {
			corruptedFiles[attachmentID] = err
			continue
		}

		if info.Size() != attachment.Size {
			corruptedFiles[attachmentID] =
				fmt.Errorf("file size mismatch: expected %d, got %d", attachment.Size, info.Size())
		}
	}

	return corruptedFiles, nil
}

// GetMaintenanceStats returns statistics about attachment storage
func (m *AttachmentMaintenance) GetMaintenanceStats(ctx context.Context) (*MaintenanceStats, error) {
	stats := &MaintenanceStats{
		TotalFiles:       0,
		TotalMetadata:    0,
		OrphanedFiles:    0,
		OrphanedMetadata: 0,
		TotalSize:        0,
	}

	attachmentsDir := filepath.Join(m.basePath, "attachments")
	metadataDir := filepath.Join(attachmentsDir, ".metadata")

	// Count metadata files
	if metadataFiles, err := ioutil.ReadDir(metadataDir); err == nil {
		for _, file := range metadataFiles {
			if strings.HasSuffix(file.Name(), ".yaml") {
				stats.TotalMetadata++
			}
		}
	}

	// Count and size attachment files
	err := filepath.Walk(attachmentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && !strings.Contains(path, ".metadata") {
			stats.TotalFiles++
			stats.TotalSize += info.Size()
		}

		return nil
	})

	// Count orphaned files (simplified check)
	orphanedFiles, _ := m.CleanupOrphanedFiles(ctx, true) // Dry run
	stats.OrphanedFiles = orphanedFiles

	orphanedMetadata, _ := m.CleanupOrphanedMetadata(ctx, true) // Dry run
	stats.OrphanedMetadata = orphanedMetadata

	return stats, err
}

// MaintenanceStats contains statistics about attachment maintenance
type MaintenanceStats struct {
	TotalFiles       int
	TotalMetadata    int
	OrphanedFiles    int
	OrphanedMetadata int
	TotalSize        int64
}
