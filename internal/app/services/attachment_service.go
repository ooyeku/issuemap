package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// AttachmentService handles attachment operations
type AttachmentService struct {
	attachmentRepo repositories.AttachmentRepository
	issueRepo      repositories.IssueRepository
	storageService *StorageService
	security       *AttachmentSecurity
	dedupService   *DeduplicationService
	basePath       string
}

// NewAttachmentService creates a new attachment service
func NewAttachmentService(attachmentRepo repositories.AttachmentRepository, issueRepo repositories.IssueRepository, storageService *StorageService, basePath string) *AttachmentService {
	return &AttachmentService{
		attachmentRepo: attachmentRepo,
		issueRepo:      issueRepo,
		storageService: storageService,
		security:       NewAttachmentSecurity(),
		dedupService:   nil, // Will be set via SetDeduplicationService
		basePath:       basePath,
	}
}

// SetDeduplicationService sets the deduplication service
func (s *AttachmentService) SetDeduplicationService(dedupService *DeduplicationService) {
	s.dedupService = dedupService
}

// UploadAttachment uploads a new attachment for an issue
func (s *AttachmentService) UploadAttachment(ctx context.Context, issueID entities.IssueID, filename string, content io.Reader, size int64, uploadedBy string) (*entities.Attachment, error) {
	// Sanitize filename first
	filename = s.security.SanitizeFilename(filename)

	// Validate file security
	if err := s.security.ValidateFile(filename, size, content); err != nil {
		return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "security_validation")
	}

	// Check storage quotas if storage service is available
	if s.storageService != nil {
		if err := s.storageService.CheckAttachmentQuota(ctx, size); err != nil {
			return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "quota_exceeded")
		}
	}

	// Verify issue exists
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "get_issue")
	}

	// Detect content type
	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Validate MIME type
	if err := s.security.ValidateMimeType(contentType); err != nil {
		return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "mime_validation")
	}

	// Create attachment entity
	attachment := entities.NewAttachment(issueID, filename, contentType, size, uploadedBy)

	// Check if deduplication is enabled and should be used for this file
	var storagePath string
	var fileHash string

	if s.dedupService != nil && s.dedupService.ShouldDeduplicate(size, contentType) {
		// Read content into buffer for hash calculation
		contentBuffer := &bytes.Buffer{}
		teeReader := io.TeeReader(content, contentBuffer)

		// Calculate file hash
		hash, actualSize, err := s.dedupService.CalculateFileHash(teeReader)
		if err != nil {
			return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "calculate_hash")
		}

		// Verify size matches
		if actualSize != size {
			return nil, errors.Wrap(fmt.Errorf("file size doesn't match expected size"), "AttachmentService.UploadAttachment", "size_mismatch")
		}

		// Get or create file hash entry
		fileHashEntry, isNew, err := s.dedupService.GetOrCreateFileHash(hash, size, filename, contentType)
		if err != nil {
			return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "get_file_hash")
		}

		// If this is a new file, save it to deduplicated storage
		if isNew {
			// Ensure target directory exists
			targetDir := filepath.Dir(fileHashEntry.StoragePath)
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "create_dedup_dir")
			}

			// Save file to deduplicated location
			file, err := os.Create(fileHashEntry.StoragePath)
			if err != nil {
				return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "create_dedup_file")
			}
			defer file.Close()

			if _, err := io.Copy(file, contentBuffer); err != nil {
				os.Remove(fileHashEntry.StoragePath) // Clean up on failure
				return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "write_dedup_file")
			}

			if err := file.Sync(); err != nil {
				os.Remove(fileHashEntry.StoragePath) // Clean up on failure
				return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "sync_dedup_file")
			}
		}

		// Add reference to the deduplicated file
		if err := s.dedupService.AddReference(attachment.ID, issueID, hash, filename); err != nil {
			// Clean up if this was a new file
			if isNew {
				os.Remove(fileHashEntry.StoragePath)
			}
			return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "add_dedup_reference")
		}

		// Store relative path for compatibility with repository
		relPath, err := filepath.Rel(s.basePath, fileHashEntry.StoragePath)
		if err != nil {
			return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "get_relative_path")
		}
		storagePath = relPath
		fileHash = hash
	} else {
		// Use traditional storage
		path, err := s.attachmentRepo.SaveFile(ctx, issueID, filename, content)
		if err != nil {
			return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "save_file")
		}
		storagePath = path
	}

	attachment.StoragePath = storagePath

	// Save metadata
	if err := s.attachmentRepo.SaveMetadata(ctx, attachment); err != nil {
		// Try to clean up on failure
		if fileHash != "" && s.dedupService != nil {
			// Remove deduplication reference
			s.dedupService.RemoveReference(attachment.ID, fileHash)
		} else {
			// Remove traditional file
			s.attachmentRepo.DeleteFile(ctx, storagePath)
		}
		return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "save_metadata")
	}

	// Update issue with attachment
	issue.AddAttachment(*attachment)
	if err := s.issueRepo.Update(ctx, issue); err != nil {
		// Clean up on failure
		if fileHash != "" && s.dedupService != nil {
			// Remove deduplication reference
			s.dedupService.RemoveReference(attachment.ID, fileHash)
		} else {
			// Remove traditional file
			s.attachmentRepo.DeleteFile(ctx, storagePath)
		}
		s.attachmentRepo.DeleteMetadata(ctx, attachment.ID)
		return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "update_issue")
	}

	return attachment, nil
}

// GetAttachment retrieves an attachment by ID
func (s *AttachmentService) GetAttachment(ctx context.Context, attachmentID string) (*entities.Attachment, error) {
	attachment, err := s.attachmentRepo.GetMetadata(ctx, attachmentID)
	if err != nil {
		return nil, errors.Wrap(err, "AttachmentService.GetAttachment", "get_metadata")
	}
	return attachment, nil
}

// GetAttachmentContent retrieves the content of an attachment
func (s *AttachmentService) GetAttachmentContent(ctx context.Context, attachmentID string) (io.ReadCloser, *entities.Attachment, error) {
	// Get metadata first
	attachment, err := s.attachmentRepo.GetMetadata(ctx, attachmentID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "AttachmentService.GetAttachmentContent", "get_metadata")
	}

	// Get file content
	content, err := s.attachmentRepo.GetFile(ctx, attachment.StoragePath)
	if err != nil {
		return nil, nil, errors.Wrap(err, "AttachmentService.GetAttachmentContent", "get_file")
	}

	return content, attachment, nil
}

// ListIssueAttachments lists all attachments for an issue
func (s *AttachmentService) ListIssueAttachments(ctx context.Context, issueID entities.IssueID) ([]*entities.Attachment, error) {
	attachments, err := s.attachmentRepo.ListByIssue(ctx, issueID)
	if err != nil {
		return nil, errors.Wrap(err, "AttachmentService.ListIssueAttachments", "list")
	}
	return attachments, nil
}

// DeleteAttachment deletes an attachment
func (s *AttachmentService) DeleteAttachment(ctx context.Context, attachmentID string) error {
	// Get attachment metadata
	attachment, err := s.attachmentRepo.GetMetadata(ctx, attachmentID)
	if err != nil {
		return errors.Wrap(err, "AttachmentService.DeleteAttachment", "get_metadata")
	}

	// Get issue
	issue, err := s.issueRepo.GetByID(ctx, attachment.IssueID)
	if err != nil {
		return errors.Wrap(err, "AttachmentService.DeleteAttachment", "get_issue")
	}

	// Remove attachment from issue
	if !issue.RemoveAttachment(attachmentID) {
		return errors.Wrap(fmt.Errorf("attachment not found in issue"), "AttachmentService.DeleteAttachment", "remove_from_issue")
	}

	// Update issue
	if err := s.issueRepo.Update(ctx, issue); err != nil {
		return errors.Wrap(err, "AttachmentService.DeleteAttachment", "update_issue")
	}

	// Check if this file is in deduplicated storage
	var isDeduplicatedFile bool
	var fileHash string

	if s.dedupService != nil {
		// Check if storage path is in dedup directory (relative path)
		if strings.HasPrefix(attachment.StoragePath, "dedup/") {
			isDeduplicatedFile = true

			// Extract hash from storage path
			// Remove "dedup/" prefix
			pathWithoutPrefix := strings.TrimPrefix(attachment.StoragePath, "dedup/")
			// Combine directory + filename to get hash (remove the "/" separator)
			dir := filepath.Dir(pathWithoutPrefix)
			filename := filepath.Base(pathWithoutPrefix)
			fileHash = dir + filename
		}
	}

	// Handle file deletion based on storage type
	if isDeduplicatedFile && s.dedupService != nil {
		// Remove deduplication reference (this will handle file deletion if ref count reaches 0)
		if err := s.dedupService.RemoveReference(attachmentID, fileHash); err != nil {
			// Log error but continue - metadata cleanup is more important
			fmt.Printf("Warning: failed to remove deduplication reference: %v\n", err)
		}
	} else {
		// Delete traditional file
		if err := s.attachmentRepo.DeleteFile(ctx, attachment.StoragePath); err != nil {
			// Continue even if file deletion fails
		}
	}

	// Delete metadata
	if err := s.attachmentRepo.DeleteMetadata(ctx, attachmentID); err != nil {
		return errors.Wrap(err, "AttachmentService.DeleteAttachment", "delete_metadata")
	}

	return nil
}

// UpdateDescription updates an attachment's description
func (s *AttachmentService) UpdateDescription(ctx context.Context, attachmentID, description string) error {
	// Get attachment metadata
	attachment, err := s.attachmentRepo.GetMetadata(ctx, attachmentID)
	if err != nil {
		return errors.Wrap(err, "AttachmentService.UpdateDescription", "get_metadata")
	}

	// Update description
	attachment.Description = description

	// Save updated metadata
	if err := s.attachmentRepo.SaveMetadata(ctx, attachment); err != nil {
		return errors.Wrap(err, "AttachmentService.UpdateDescription", "save_metadata")
	}

	return nil
}

// GetStorageStats returns attachment storage statistics
func (s *AttachmentService) GetStorageStats(ctx context.Context) (*repositories.AttachmentStats, error) {
	stats, err := s.attachmentRepo.GetStorageStats(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "AttachmentService.GetStorageStats", "get_stats")
	}
	return stats, nil
}
