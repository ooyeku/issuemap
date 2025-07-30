package repositories

import (
	"context"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// Commit represents a git commit
type Commit struct {
	Hash      string    `json:"hash"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Date      time.Time `json:"date"`
	IssueRefs []string  `json:"issue_refs"` // Extracted issue references
}

// GitRepository defines the interface for git integration operations
type GitRepository interface {
	// IsGitRepository checks if the current directory is a git repository
	IsGitRepository(ctx context.Context) (bool, error)

	// GetCurrentBranch returns the current git branch
	GetCurrentBranch(ctx context.Context) (string, error)

	// GetCommitsSince returns commits since a specific date
	GetCommitsSince(ctx context.Context, since time.Time) ([]Commit, error)

	// CreateBranch creates a new branch with the given name
	CreateBranch(ctx context.Context, name string) error

	// GetCommitMessage returns the commit message for a given hash
	GetCommitMessage(ctx context.Context, hash string) (string, error)

	// GetLatestCommit returns the latest commit on the current branch
	GetLatestCommit(ctx context.Context) (*Commit, error)

	// GetCommitsByIssue returns commits that reference a specific issue
	GetCommitsByIssue(ctx context.Context, issueID entities.IssueID) ([]Commit, error)

	// ParseIssueReferences extracts issue references from a commit message
	ParseIssueReferences(message string) []string

	// GetAuthorInfo returns the current git user information
	GetAuthorInfo(ctx context.Context) (*entities.User, error)

	// InstallHooks installs git hooks for issue tracking
	InstallHooks(ctx context.Context) error

	// UninstallHooks removes git hooks
	UninstallHooks(ctx context.Context) error

	// GetRepositoryRoot returns the root directory of the git repository
	GetRepositoryRoot(ctx context.Context) (string, error)
}
