package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// FileAttachmentRepository implements the AttachmentRepository interface using file storage
type FileAttachmentRepository struct {
	basePath string
}

// NewFileAttachmentRepository creates a new file-based attachment repository
func NewFileAttachmentRepository(basePath string) *FileAttachmentRepository {
	return &FileAttachmentRepository{
		basePath: basePath,
	}
}

// SaveFile saves an attachment file to storage
func (r *FileAttachmentRepository) SaveFile(ctx context.Context, issueID entities.IssueID, filename string, content io.Reader) (string, error) {
	// Validate inputs
	if issueID == "" {
		return "", errors.Wrap(fmt.Errorf("invalid issue ID"), "FileAttachmentRepository.SaveFile", "validation")
	}
	if filename == "" {
		return "", errors.Wrap(fmt.Errorf("invalid filename"), "FileAttachmentRepository.SaveFile", "validation")
	}

	// Sanitize filename to prevent path traversal
	safeFilename := filepath.Base(filename)
	if safeFilename == "." || safeFilename == ".." {
		return "", errors.Wrap(fmt.Errorf("invalid filename"), "FileAttachmentRepository.SaveFile", "validation")
	}

	// Create attachments directory structure
	attachmentsDir := filepath.Join(r.basePath, "attachments", string(issueID))
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		return "", errors.Wrap(err, "FileAttachmentRepository.SaveFile", "mkdir")
	}

	// Generate unique filename to avoid collisions
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano())
	uniqueFilename := fmt.Sprintf("%s_%s", timestamp, safeFilename)
	storagePath := filepath.Join("attachments", string(issueID), uniqueFilename)

	// Validate storage path for security
	if strings.Contains(storagePath, "..") {
		return "", errors.Wrap(fmt.Errorf("invalid storage path"), "FileAttachmentRepository.SaveFile", "security")
	}

	fullPath := filepath.Join(r.basePath, storagePath)

	// Create the file
	file, err := os.Create(fullPath)
	if err != nil {
		return "", errors.Wrap(err, "FileAttachmentRepository.SaveFile", "create")
	}
	defer file.Close()

	// Copy content to file
	if _, err := io.Copy(file, content); err != nil {
		os.Remove(fullPath) // Clean up on error
		return "", errors.Wrap(err, "FileAttachmentRepository.SaveFile", "copy")
	}

	return storagePath, nil
}

// GetFile retrieves an attachment file from storage
func (r *FileAttachmentRepository) GetFile(ctx context.Context, storagePath string) (io.ReadCloser, error) {
	// Validate storage path for security
	if strings.Contains(storagePath, "..") ||
		!strings.HasPrefix(storagePath, "attachments/") {
		return nil, errors.Wrap(fmt.Errorf("invalid storage path"), "FileAttachmentRepository.GetFile", "security")
	}

	// Clean the path to prevent path traversal
	cleanPath := filepath.Clean(storagePath)
	if cleanPath != storagePath {
		return nil, errors.Wrap(fmt.Errorf("invalid storage path"), "FileAttachmentRepository.GetFile", "security")
	}

	fullPath := filepath.Join(r.basePath, storagePath)

	// Ensure the resolved path is still within our base directory
	absBasePath, err := filepath.Abs(r.basePath)
	if err != nil {
		return nil, errors.Wrap(err, "FileAttachmentRepository.GetFile", "abs_path")
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, errors.Wrap(err, "FileAttachmentRepository.GetFile", "abs_path")
	}

	if !strings.HasPrefix(absFullPath, absBasePath) {
		return nil, errors.Wrap(fmt.Errorf("path outside base directory"), "FileAttachmentRepository.GetFile", "security")
	}

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrap(errors.ErrAttachmentNotFound, "FileAttachmentRepository.GetFile", "not_found")
		}
		return nil, errors.Wrap(err, "FileAttachmentRepository.GetFile", "open")
	}

	return file, nil
}

// DeleteFile removes an attachment file from storage
func (r *FileAttachmentRepository) DeleteFile(ctx context.Context, storagePath string) error {
	fullPath := filepath.Join(r.basePath, storagePath)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return errors.Wrap(errors.ErrAttachmentNotFound, "FileAttachmentRepository.DeleteFile", "not_found")
		}
		return errors.Wrap(err, "FileAttachmentRepository.DeleteFile", "remove")
	}

	return nil
}

// SaveMetadata saves attachment metadata
func (r *FileAttachmentRepository) SaveMetadata(ctx context.Context, attachment *entities.Attachment) error {
	if err := attachment.Validate(); err != nil {
		return errors.Wrap(err, "FileAttachmentRepository.SaveMetadata", "validation")
	}

	// Create metadata directory
	metadataDir := filepath.Join(r.basePath, "attachments", ".metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return errors.Wrap(err, "FileAttachmentRepository.SaveMetadata", "mkdir")
	}

	// Save metadata file
	metadataPath := filepath.Join(metadataDir, fmt.Sprintf("%s.yaml", attachment.ID))
	data, err := yaml.Marshal(attachment)
	if err != nil {
		return errors.Wrap(err, "FileAttachmentRepository.SaveMetadata", "marshal")
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return errors.Wrap(err, "FileAttachmentRepository.SaveMetadata", "write")
	}

	return nil
}

// GetMetadata retrieves attachment metadata
func (r *FileAttachmentRepository) GetMetadata(ctx context.Context, attachmentID string) (*entities.Attachment, error) {
	metadataPath := filepath.Join(r.basePath, "attachments", ".metadata", fmt.Sprintf("%s.yaml", attachmentID))

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrap(errors.ErrAttachmentNotFound, "FileAttachmentRepository.GetMetadata", "not_found")
		}
		return nil, errors.Wrap(err, "FileAttachmentRepository.GetMetadata", "read")
	}

	var attachment entities.Attachment
	if err := yaml.Unmarshal(data, &attachment); err != nil {
		return nil, errors.Wrap(err, "FileAttachmentRepository.GetMetadata", "unmarshal")
	}

	return &attachment, nil
}

// ListByIssue retrieves all attachments for an issue
func (r *FileAttachmentRepository) ListByIssue(ctx context.Context, issueID entities.IssueID) ([]*entities.Attachment, error) {
	metadataDir := filepath.Join(r.basePath, "attachments", ".metadata")

	// Create directory if it doesn't exist
	if _, err := os.Stat(metadataDir); os.IsNotExist(err) {
		return []*entities.Attachment{}, nil
	}

	files, err := os.ReadDir(metadataDir)
	if err != nil {
		return nil, errors.Wrap(err, "FileAttachmentRepository.ListByIssue", "read_dir")
	}

	var attachments []*entities.Attachment
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		metadataPath := filepath.Join(metadataDir, file.Name())
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			continue // Skip files that can't be read
		}

		var attachment entities.Attachment
		if err := yaml.Unmarshal(data, &attachment); err != nil {
			continue // Skip files that can't be parsed
		}

		if attachment.IssueID == issueID {
			attachments = append(attachments, &attachment)
		}
	}

	return attachments, nil
}

// DeleteMetadata removes attachment metadata
func (r *FileAttachmentRepository) DeleteMetadata(ctx context.Context, attachmentID string) error {
	metadataPath := filepath.Join(r.basePath, "attachments", ".metadata", fmt.Sprintf("%s.yaml", attachmentID))

	if err := os.Remove(metadataPath); err != nil {
		if os.IsNotExist(err) {
			return errors.Wrap(errors.ErrAttachmentNotFound, "FileAttachmentRepository.DeleteMetadata", "not_found")
		}
		return errors.Wrap(err, "FileAttachmentRepository.DeleteMetadata", "remove")
	}

	return nil
}

// GetStorageStats returns storage statistics
func (r *FileAttachmentRepository) GetStorageStats(ctx context.Context) (*repositories.AttachmentStats, error) {
	metadataDir := filepath.Join(r.basePath, "attachments", ".metadata")

	stats := &repositories.AttachmentStats{
		AttachmentTypes: make(map[entities.AttachmentType]int),
	}

	// Check if metadata directory exists
	if _, err := os.Stat(metadataDir); os.IsNotExist(err) {
		return stats, nil
	}

	files, err := os.ReadDir(metadataDir)
	if err != nil {
		return nil, errors.Wrap(err, "FileAttachmentRepository.GetStorageStats", "read_dir")
	}

	var totalSize int64
	var largestSize int64

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		metadataPath := filepath.Join(metadataDir, file.Name())
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			continue
		}

		var attachment entities.Attachment
		if err := yaml.Unmarshal(data, &attachment); err != nil {
			continue
		}

		stats.TotalCount++
		stats.AttachmentTypes[attachment.Type]++
		totalSize += attachment.Size

		if attachment.Size > largestSize {
			largestSize = attachment.Size
			stats.LargestFile = &attachment
		}
	}

	stats.TotalSize = totalSize
	if stats.TotalCount > 0 {
		stats.AverageSize = totalSize / int64(stats.TotalCount)
	}

	return stats, nil
}
