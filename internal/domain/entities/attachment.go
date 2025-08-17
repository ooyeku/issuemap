package entities

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// AttachmentType represents the type of attachment
type AttachmentType string

const (
	AttachmentTypeImage    AttachmentType = "image"
	AttachmentTypeDocument AttachmentType = "document"
	AttachmentTypeText     AttachmentType = "text"
	AttachmentTypeOther    AttachmentType = "other"
)

// Attachment represents a file attached to an issue
type Attachment struct {
	ID          string         `yaml:"id" json:"id"`
	IssueID     IssueID        `yaml:"issue_id" json:"issue_id"`
	Filename    string         `yaml:"filename" json:"filename"`
	ContentType string         `yaml:"content_type" json:"content_type"`
	Size        int64          `yaml:"size" json:"size"`
	Type        AttachmentType `yaml:"type" json:"type"`
	StoragePath string         `yaml:"storage_path" json:"storage_path"`
	UploadedBy  string         `yaml:"uploaded_by" json:"uploaded_by"`
	UploadedAt  time.Time      `yaml:"uploaded_at" json:"uploaded_at"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`

	// Compression metadata
	Compression *CompressionMetadata `yaml:"compression,omitempty" json:"compression,omitempty"`
}

// NewAttachment creates a new attachment
func NewAttachment(issueID IssueID, filename, contentType string, size int64, uploadedBy string) *Attachment {
	id := fmt.Sprintf("%s-att-%d", issueID, time.Now().UnixNano())

	return &Attachment{
		ID:          id,
		IssueID:     issueID,
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
		Type:        determineAttachmentType(filename, contentType),
		UploadedBy:  uploadedBy,
		UploadedAt:  time.Now(),
	}
}

// determineAttachmentType determines the type of attachment based on filename and content type
func determineAttachmentType(filename, contentType string) AttachmentType {
	ext := strings.ToLower(filepath.Ext(filename))
	contentTypeLower := strings.ToLower(contentType)

	// Check by content type first
	if strings.HasPrefix(contentTypeLower, "image/") {
		return AttachmentTypeImage
	}
	if strings.HasPrefix(contentTypeLower, "text/") {
		return AttachmentTypeText
	}

	// Check by extension
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp":
		return AttachmentTypeImage
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx":
		return AttachmentTypeDocument
	case ".txt", ".md", ".log", ".csv", ".json", ".xml", ".yaml", ".yml":
		return AttachmentTypeText
	default:
		return AttachmentTypeOther
	}
}

// GetSizeFormatted returns a human-readable size format
func (a *Attachment) GetSizeFormatted() string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case a.Size >= GB:
		return fmt.Sprintf("%.2f GB", float64(a.Size)/float64(GB))
	case a.Size >= MB:
		return fmt.Sprintf("%.2f MB", float64(a.Size)/float64(MB))
	case a.Size >= KB:
		return fmt.Sprintf("%.2f KB", float64(a.Size)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", a.Size)
	}
}

// Validate validates the attachment data
func (a *Attachment) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("attachment ID cannot be empty")
	}
	if a.IssueID == "" {
		return fmt.Errorf("issue ID cannot be empty")
	}
	if strings.TrimSpace(a.Filename) == "" {
		return fmt.Errorf("filename cannot be empty")
	}
	if a.Size < 0 {
		return fmt.Errorf("file size cannot be negative")
	}
	// Max file size: 10MB
	maxSize := int64(10 * 1024 * 1024)
	if a.Size > maxSize {
		return fmt.Errorf("file size exceeds maximum allowed size of 10MB")
	}
	return nil
}
