package repositories

import (
	"context"
	"io"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// AttachmentRepository defines the interface for attachment storage operations
type AttachmentRepository interface {
	// SaveFile saves an attachment file to storage
	SaveFile(ctx context.Context, issueID entities.IssueID, filename string, content io.Reader) (string, error)

	// GetFile retrieves an attachment file from storage
	GetFile(ctx context.Context, storagePath string) (io.ReadCloser, error)

	// DeleteFile removes an attachment file from storage
	DeleteFile(ctx context.Context, storagePath string) error

	// SaveMetadata saves attachment metadata
	SaveMetadata(ctx context.Context, attachment *entities.Attachment) error

	// GetMetadata retrieves attachment metadata
	GetMetadata(ctx context.Context, attachmentID string) (*entities.Attachment, error)

	// ListByIssue retrieves all attachments for an issue
	ListByIssue(ctx context.Context, issueID entities.IssueID) ([]*entities.Attachment, error)

	// DeleteMetadata removes attachment metadata
	DeleteMetadata(ctx context.Context, attachmentID string) error

	// GetStorageStats returns storage statistics
	GetStorageStats(ctx context.Context) (*AttachmentStats, error)
}

// AttachmentStats contains storage statistics for attachments
type AttachmentStats struct {
	TotalCount      int
	TotalSize       int64
	AverageSize     int64
	LargestFile     *entities.Attachment
	AttachmentTypes map[entities.AttachmentType]int
}
