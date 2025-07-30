package errors

import (
	"fmt"
)

// Domain errors
var (
	ErrIssueNotFound      = fmt.Errorf("issue not found")
	ErrIssueAlreadyExists = fmt.Errorf("issue already exists")
	ErrInvalidIssueID     = fmt.Errorf("invalid issue ID")
	ErrInvalidStatus      = fmt.Errorf("invalid status transition")
	ErrGitNotInitialized  = fmt.Errorf("git repository not initialized")
	ErrNotInGitRepo       = fmt.Errorf("not in a git repository")
	ErrConfigNotFound     = fmt.Errorf("configuration not found")
	ErrConfigInvalid      = fmt.Errorf("configuration is invalid")
	ErrTemplateNotFound   = fmt.Errorf("template not found")
	ErrPermissionDenied   = fmt.Errorf("permission denied")
	ErrDirectoryNotFound  = fmt.Errorf("directory not found")
	ErrFileNotFound       = fmt.Errorf("file not found")
	ErrInvalidInput       = fmt.Errorf("invalid input")
	ErrOperationFailed    = fmt.Errorf("operation failed")
	ErrNotInitialized     = fmt.Errorf("issuemap not initialized in this repository")
	ErrNotFound           = fmt.Errorf("not found")
)

// Error represents a wrapped error with additional context
type Error struct {
	Op   string // Operation that failed
	Kind string // Error kind/category
	Err  error  // Underlying error
}

// Error returns the string representation of the error
func (e *Error) Error() string {
	if e.Op == "" {
		return fmt.Sprintf("%s: %v", e.Kind, e.Err)
	}
	return fmt.Sprintf("%s: %s: %v", e.Op, e.Kind, e.Err)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Err
}

// New creates a new Error with the given operation, kind, and underlying error
func New(op, kind string, err error) *Error {
	return &Error{
		Op:   op,
		Kind: kind,
		Err:  err,
	}
}

// Wrap wraps an existing error with operation and kind information
func Wrap(err error, op, kind string) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		Op:   op,
		Kind: kind,
		Err:  err,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

// Error returns the string representation of the validation error
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
