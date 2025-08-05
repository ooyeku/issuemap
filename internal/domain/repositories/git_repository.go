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

// BranchStatus represents the status of a branch relative to origin
type BranchStatus struct {
	Name          string `json:"name"`
	Exists        bool   `json:"exists"`
	IsTracked     bool   `json:"is_tracked"`
	AheadBy       int    `json:"ahead_by"`
	BehindBy      int    `json:"behind_by"`
	HasUnpushed   bool   `json:"has_unpushed"`
	HasUnpulled   bool   `json:"has_unpulled"`
	LastCommit    string `json:"last_commit"`
	LastCommitMsg string `json:"last_commit_msg"`
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

	// SwitchToBranch switches to an existing branch
	SwitchToBranch(ctx context.Context, name string) error

	// GetBranches returns a list of all branches
	GetBranches(ctx context.Context) ([]string, error)

	// BranchExists checks if a branch exists
	BranchExists(ctx context.Context, name string) (bool, error)

	// GetBranchStatus returns the status of a branch relative to origin
	GetBranchStatus(ctx context.Context, branch string) (*BranchStatus, error)

	// PushBranch pushes a branch to the remote repository
	PushBranch(ctx context.Context, branch string) error

	// PullBranch pulls changes from the remote repository
	PullBranch(ctx context.Context, branch string) error
}
