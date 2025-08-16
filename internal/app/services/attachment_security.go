package services

import (
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"regexp"
	"strings"
)

// AttachmentSecurity provides security validations for attachments
type AttachmentSecurity struct {
	maxFileSize       int64
	allowedMimeTypes  map[string]bool
	blockedExtensions map[string]bool
	filenamePattern   *regexp.Regexp
}

// NewAttachmentSecurity creates a new attachment security validator
func NewAttachmentSecurity() *AttachmentSecurity {
	return &AttachmentSecurity{
		maxFileSize: 10 * 1024 * 1024, // 10MB
		allowedMimeTypes: map[string]bool{
			// Images
			"image/jpeg":    true,
			"image/png":     true,
			"image/gif":     true,
			"image/svg+xml": true,
			"image/webp":    true,
			"image/bmp":     true,
			// Documents
			"application/pdf":    true,
			"application/msword": true,
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
			"application/vnd.ms-excel": true,
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
			"application/vnd.ms-powerpoint":                                             true,
			"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
			// Text
			"text/plain":         true,
			"text/html":          false, // Blocked for security
			"text/css":           true,
			"text/csv":           true,
			"text/markdown":      true,
			"text/x-markdown":    true,
			"application/json":   true,
			"application/xml":    true,
			"text/xml":           true,
			"application/x-yaml": true,
			"text/yaml":          true,
			// Archives (disabled by default for security)
			"application/zip":   false,
			"application/x-rar": false,
			"application/x-tar": false,
			"application/gzip":  false,
			// Generic
			"application/octet-stream": true, // Allow but validate extension
		},
		blockedExtensions: map[string]bool{
			".exe":   true,
			".bat":   true,
			".cmd":   true,
			".sh":    true,
			".ps1":   true,
			".app":   true,
			".jar":   true,
			".com":   true,
			".scr":   true,
			".vbs":   true,
			".vbe":   true,
			".js":    true,
			".jse":   true,
			".ws":    true,
			".wsf":   true,
			".wsc":   true,
			".wsh":   true,
			".msc":   true,
			".dll":   true,
			".so":    true,
			".dylib": true,
		},
		// Only allow alphanumeric, dots, dashes, underscores, and spaces
		filenamePattern: regexp.MustCompile(`^[a-zA-Z0-9._\- ]+$`),
	}
}

// ValidateFile validates an attachment file
func (s *AttachmentSecurity) ValidateFile(filename string, size int64, reader io.Reader) error {
	// Validate size
	if err := s.ValidateSize(size); err != nil {
		return err
	}

	// Validate filename
	if err := s.ValidateFilename(filename); err != nil {
		return err
	}

	// Validate extension
	if err := s.ValidateExtension(filename); err != nil {
		return err
	}

	// Validate MIME type
	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if err := s.ValidateMimeType(contentType); err != nil {
		return err
	}

	// Additional content validation could be added here
	// For example, checking file magic numbers

	return nil
}

// ValidateSize checks if file size is within limits
func (s *AttachmentSecurity) ValidateSize(size int64) error {
	if size <= 0 {
		return fmt.Errorf("invalid file size")
	}
	if size > s.maxFileSize {
		return fmt.Errorf("file size exceeds maximum allowed size of %d MB", s.maxFileSize/(1024*1024))
	}
	return nil
}

// ValidateFilename checks for dangerous patterns in filename
func (s *AttachmentSecurity) ValidateFilename(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	// Check for path traversal attempts
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("invalid filename: path traversal detected")
	}

	// Check filename pattern
	if !s.filenamePattern.MatchString(filename) {
		return fmt.Errorf("filename contains invalid characters")
	}

	// Check filename length
	if len(filename) > 255 {
		return fmt.Errorf("filename too long")
	}

	return nil
}

// ValidateExtension checks if file extension is allowed
func (s *AttachmentSecurity) ValidateExtension(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))

	if ext == "" {
		return fmt.Errorf("file must have an extension")
	}

	if s.blockedExtensions[ext] {
		return fmt.Errorf("file type %s is not allowed for security reasons", ext)
	}

	return nil
}

// ValidateMimeType checks if MIME type is allowed
func (s *AttachmentSecurity) ValidateMimeType(mimeType string) error {
	if mimeType == "" {
		return nil // Allow empty MIME type, will be set to octet-stream
	}

	// Strip parameters from MIME type
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = mimeType[:idx]
	}
	mimeType = strings.TrimSpace(strings.ToLower(mimeType))

	// Check if explicitly allowed or blocked
	if allowed, exists := s.allowedMimeTypes[mimeType]; exists {
		if !allowed {
			return fmt.Errorf("MIME type %s is not allowed for security reasons", mimeType)
		}
		return nil
	}

	// Deny unknown MIME types by default
	return fmt.Errorf("MIME type %s is not in the allowed list", mimeType)
}

// SanitizeFilename removes dangerous characters from filename
func (s *AttachmentSecurity) SanitizeFilename(filename string) string {
	// Remove any path components
	filename = filepath.Base(filename)

	// Replace dangerous characters with underscores
	sanitized := regexp.MustCompile(`[^a-zA-Z0-9._\- ]`).ReplaceAllString(filename, "_")

	// Ensure it has a safe extension if missing
	if filepath.Ext(sanitized) == "" {
		sanitized = sanitized + ".txt"
	}

	// Limit length
	if len(sanitized) > 100 {
		ext := filepath.Ext(sanitized)
		base := sanitized[:len(sanitized)-len(ext)]
		if len(base) > 96 {
			base = base[:96]
		}
		sanitized = base + ext
	}

	return sanitized
}

// ValidateStoragePath ensures storage path is safe
func (s *AttachmentSecurity) ValidateStoragePath(path string) error {
	// Check for path traversal
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("invalid storage path: path traversal detected")
	}

	// Ensure path doesn't start with system directories
	if strings.HasPrefix(cleaned, "/etc") ||
		strings.HasPrefix(cleaned, "/sys") ||
		strings.HasPrefix(cleaned, "/proc") ||
		strings.HasPrefix(cleaned, "/dev") {
		return fmt.Errorf("invalid storage path: system directory access denied")
	}

	return nil
}
