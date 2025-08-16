package services

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// AttachmentService handles attachment operations
type AttachmentService struct {
	attachmentRepo repositories.AttachmentRepository
	issueRepo      repositories.IssueRepository
	security       *AttachmentSecurity
}

// NewAttachmentService creates a new attachment service
func NewAttachmentService(attachmentRepo repositories.AttachmentRepository, issueRepo repositories.IssueRepository) *AttachmentService {
	return &AttachmentService{
		attachmentRepo: attachmentRepo,
		issueRepo:      issueRepo,
		security:       NewAttachmentSecurity(),
	}
}

// UploadAttachment uploads a new attachment for an issue
func (s *AttachmentService) UploadAttachment(ctx context.Context, issueID entities.IssueID, filename string, content io.Reader, size int64, uploadedBy string) (*entities.Attachment, error) {
	// Sanitize filename first
	filename = s.security.SanitizeFilename(filename)

	// Validate file security
	if err := s.security.ValidateFile(filename, size, content); err != nil {
		return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "security_validation")
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

	// Save file to storage
	storagePath, err := s.attachmentRepo.SaveFile(ctx, issueID, filename, content)
	if err != nil {
		return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "save_file")
	}
	attachment.StoragePath = storagePath

	// Save metadata
	if err := s.attachmentRepo.SaveMetadata(ctx, attachment); err != nil {
		// Try to clean up the file if metadata save fails
		_ = s.attachmentRepo.DeleteFile(ctx, storagePath)
		return nil, errors.Wrap(err, "AttachmentService.UploadAttachment", "save_metadata")
	}

	// Update issue with attachment
	issue.AddAttachment(*attachment)
	if err := s.issueRepo.Update(ctx, issue); err != nil {
		// Clean up on failure
		_ = s.attachmentRepo.DeleteFile(ctx, storagePath)
		_ = s.attachmentRepo.DeleteMetadata(ctx, attachment.ID)
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

	// Delete file
	if err := s.attachmentRepo.DeleteFile(ctx, attachment.StoragePath); err != nil {
		// Continue even if file deletion fails
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
