package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// GitClient implements the GitRepository interface
type GitClient struct {
	repoPath string
	repo     *git.Repository
}

// NewGitClient creates a new git client
func NewGitClient(repoPath string) (*GitClient, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, errors.Wrap(err, "NewGitClient", "open_repo")
	}

	return &GitClient{
		repoPath: repoPath,
		repo:     repo,
	}, nil
}

// IsGitRepository checks if the current directory is a git repository
func (g *GitClient) IsGitRepository(ctx context.Context) (bool, error) {
	_, err := git.PlainOpen(g.repoPath)
	return err == nil, nil
}

// GetCurrentBranch returns the current git branch
func (g *GitClient) GetCurrentBranch(ctx context.Context) (string, error) {
	head, err := g.repo.Head()
	if err != nil {
		return "", errors.Wrap(err, "GitClient.GetCurrentBranch", "get_head")
	}

	if head.Name().IsBranch() {
		return head.Name().Short(), nil
	}

	return "HEAD", nil
}

// GetCommitsSince returns commits since a specific date
func (g *GitClient) GetCommitsSince(ctx context.Context, since time.Time) ([]repositories.Commit, error) {
	commits, err := g.repo.Log(&git.LogOptions{
		Since: &since,
	})
	if err != nil {
		return nil, errors.Wrap(err, "GitClient.GetCommitsSince", "log")
	}

	var result []repositories.Commit
	err = commits.ForEach(func(c *object.Commit) error {
		commit := repositories.Commit{
			Hash:      c.Hash.String(),
			Message:   c.Message,
			Author:    c.Author.Name,
			Email:     c.Author.Email,
			Date:      c.Author.When,
			IssueRefs: g.ParseIssueReferences(c.Message),
		}
		result = append(result, commit)
		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "GitClient.GetCommitsSince", "iterate")
	}

	return result, nil
}

// CreateBranch creates a new branch with the given name
func (g *GitClient) CreateBranch(ctx context.Context, name string) error {
	// Get current HEAD
	head, err := g.repo.Head()
	if err != nil {
		return errors.Wrap(err, "GitClient.CreateBranch", "get_head")
	}

	branchRef := plumbing.NewBranchReferenceName(name)

	err = g.repo.Storer.SetReference(plumbing.NewHashReference(branchRef, head.Hash()))
	if err != nil {
		return errors.Wrap(err, "GitClient.CreateBranch", "set_reference")
	}

	return nil
}

// SwitchToBranch switches to an existing branch
func (g *GitClient) SwitchToBranch(ctx context.Context, name string) error {
	workTree, err := g.repo.Worktree()
	if err != nil {
		return errors.Wrap(err, "GitClient.SwitchToBranch", "worktree")
	}

	branchRef := plumbing.NewBranchReferenceName(name)

	// Check out the branch
	err = workTree.Checkout(&git.CheckoutOptions{
		Branch: branchRef,
	})
	if err != nil {
		return errors.Wrap(err, "GitClient.SwitchToBranch", "checkout")
	}

	return nil
}

// GetCommitMessage returns the commit message for a given hash
func (g *GitClient) GetCommitMessage(ctx context.Context, hash string) (string, error) {
	h := plumbing.NewHash(hash)
	commit, err := g.repo.CommitObject(h)
	if err != nil {
		return "", errors.Wrap(err, "GitClient.GetCommitMessage", "get_commit")
	}

	return commit.Message, nil
}

// GetLatestCommit returns the latest commit on the current branch
func (g *GitClient) GetLatestCommit(ctx context.Context) (*repositories.Commit, error) {
	head, err := g.repo.Head()
	if err != nil {
		return nil, errors.Wrap(err, "GitClient.GetLatestCommit", "get_head")
	}

	commit, err := g.repo.CommitObject(head.Hash())
	if err != nil {
		return nil, errors.Wrap(err, "GitClient.GetLatestCommit", "get_commit")
	}

	return &repositories.Commit{
		Hash:      commit.Hash.String(),
		Message:   commit.Message,
		Author:    commit.Author.Name,
		Email:     commit.Author.Email,
		Date:      commit.Author.When,
		IssueRefs: g.ParseIssueReferences(commit.Message),
	}, nil
}

// GetCommitsByIssue returns commits that reference a specific issue
func (g *GitClient) GetCommitsByIssue(ctx context.Context, issueID entities.IssueID) ([]repositories.Commit, error) {
	commits, err := g.repo.Log(&git.LogOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "GitClient.GetCommitsByIssue", "log")
	}

	var result []repositories.Commit
	targetIssue := issueID.String()

	err = commits.ForEach(func(c *object.Commit) error {
		issueRefs := g.ParseIssueReferences(c.Message)
		for _, ref := range issueRefs {
			if ref == targetIssue {
				commit := repositories.Commit{
					Hash:      c.Hash.String(),
					Message:   c.Message,
					Author:    c.Author.Name,
					Email:     c.Author.Email,
					Date:      c.Author.When,
					IssueRefs: issueRefs,
				}
				result = append(result, commit)
				break
			}
		}
		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "GitClient.GetCommitsByIssue", "iterate")
	}

	return result, nil
}

// ParseIssueReferences extracts issue references from a commit message
func (g *GitClient) ParseIssueReferences(message string) []string {
	// Patterns to match:
	// - ISSUE-123
	// - #123 (if using numeric IDs)
	// - Closes ISSUE-123
	// - Fixes #123
	patterns := []string{
		`ISSUE-\d+`,
		`#\d+`,
		`(?i)(?:closes?|fixes?|resolves?)\s+(ISSUE-\d+)`,
		`(?i)(?:closes?|fixes?|resolves?)\s+(#\d+)`,
	}

	var references []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(message, -1)

		for _, match := range matches {
			// Clean up the match (remove "Closes", "Fixes", etc.)
			cleanMatch := regexp.MustCompile(`(?i)(?:closes?|fixes?|resolves?)\s+`).ReplaceAllString(match, "")
			cleanMatch = strings.TrimSpace(cleanMatch)

			if !seen[cleanMatch] {
				references = append(references, cleanMatch)
				seen[cleanMatch] = true
			}
		}
	}

	return references
}

// GetAuthorInfo returns the current git user information
func (g *GitClient) GetAuthorInfo(ctx context.Context) (*entities.User, error) {
	// Try to get user info from git config
	name, err := g.getGitConfig("user.name")
	if err != nil {
		return nil, errors.Wrap(err, "GitClient.GetAuthorInfo", "get_name")
	}

	email, err := g.getGitConfig("user.email")
	if err != nil {
		return nil, errors.Wrap(err, "GitClient.GetAuthorInfo", "get_email")
	}

	return &entities.User{
		Username: name,
		Email:    email,
	}, nil
}

// InstallHooks installs git hooks for issue tracking
func (g *GitClient) InstallHooks(ctx context.Context) error {
	hooksDir := filepath.Join(g.repoPath, ".git", "hooks")

	// Create commit-msg hook for automatic issue linking
	commitMsgHook := `#!/bin/sh
# IssueMap commit-msg hook
# Automatically links commits to issues and updates issue status

commit_msg_file=$1
commit_msg=$(cat "$commit_msg_file")

# Extract current branch name
branch=$(git rev-parse --abbrev-ref HEAD)

# Check if branch follows pattern like "feature/ISSUE-123-description" or "bugfix/ISSUE-123-description"
if echo "$branch" | grep -q "ISSUE-[0-9]\+"; then
    issue_id=$(echo "$branch" | sed -n 's/.*\(ISSUE-[0-9]\+\).*/\1/p')
    
    # Check if commit message already contains issue reference
    if ! echo "$commit_msg" | grep -q "$issue_id"; then
        # Append issue reference to commit message
        echo "$commit_msg" > "$commit_msg_file"
        echo "" >> "$commit_msg_file"
        echo "Refs: $issue_id" >> "$commit_msg_file"
    fi
    
    # Update issue status if this is the first commit on the branch
    commit_count=$(git rev-list --count HEAD ^origin/main 2>/dev/null || git rev-list --count HEAD ^origin/master 2>/dev/null || echo "1")
    if [ "$commit_count" = "1" ]; then
        # This is the first commit, optionally update issue to "in-progress"
        # We could call issuemap here but keeping it simple for now
        echo "# First commit for $issue_id - consider updating issue status to 'in-progress'" >> "$commit_msg_file"
    fi
fi

# Check for auto-close keywords
if echo "$commit_msg" | grep -qiE "(closes?|fixes?|resolves?) #?ISSUE-[0-9]+"; then
    # Extract issue IDs that should be closed
    close_issues=$(echo "$commit_msg" | grep -oiE "(closes?|fixes?|resolves?) #?ISSUE-[0-9]+" | grep -oE "ISSUE-[0-9]+")
    for issue in $close_issues; do
        echo "# This commit will close $issue" >> "$commit_msg_file"
        # In a full implementation, we could call: issuemap close $issue --reason "Fixed in commit $(git rev-parse --short HEAD)"
    done
fi
`

	commitMsgPath := filepath.Join(hooksDir, "commit-msg")
	if err := os.WriteFile(commitMsgPath, []byte(commitMsgHook), 0755); err != nil {
		return errors.Wrap(err, "GitClient.InstallHooks", "write_commit_msg")
	}

	return nil
}

// UninstallHooks removes git hooks
func (g *GitClient) UninstallHooks(ctx context.Context) error {
	hooksDir := filepath.Join(g.repoPath, ".git", "hooks")

	hooks := []string{"commit-msg", "post-merge"}
	for _, hook := range hooks {
		hookPath := filepath.Join(hooksDir, hook)
		if err := os.Remove(hookPath); err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err, "GitClient.UninstallHooks", "remove_hook")
		}
	}

	return nil
}

// GetRepositoryRoot returns the root directory of the git repository
func (g *GitClient) GetRepositoryRoot(ctx context.Context) (string, error) {
	return g.repoPath, nil
}

// getGitConfig retrieves a git configuration value
func (g *GitClient) getGitConfig(key string) (string, error) {
	cmd := exec.Command("git", "config", "--get", key)
	cmd.Dir = g.repoPath

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// GetBranches returns a list of all branches
func (g *GitClient) GetBranches(ctx context.Context) ([]string, error) {
	refs, err := g.repo.Branches()
	if err != nil {
		return nil, errors.Wrap(err, "GitClient.GetBranches", "list_branches")
	}

	var branches []string
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		branchName := ref.Name().Short()
		branches = append(branches, branchName)
		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "GitClient.GetBranches", "iterate_branches")
	}

	return branches, nil
}

// BranchExists checks if a branch exists
func (g *GitClient) BranchExists(ctx context.Context, name string) (bool, error) {
	branchRef := plumbing.NewBranchReferenceName(name)
	_, err := g.repo.Reference(branchRef, false)
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return false, nil
		}
		return false, errors.Wrap(err, "GitClient.BranchExists", "check_reference")
	}
	return true, nil
}

// GetBranchStatus returns the status of a branch relative to origin
func (g *GitClient) GetBranchStatus(ctx context.Context, branch string) (*repositories.BranchStatus, error) {
	status := &repositories.BranchStatus{
		Name:   branch,
		Exists: false,
	}

	// Check if branch exists
	exists, err := g.BranchExists(ctx, branch)
	if err != nil {
		return nil, err
	}

	if !exists {
		return status, nil
	}

	status.Exists = true

	// Get branch reference
	branchRef := plumbing.NewBranchReferenceName(branch)
	ref, err := g.repo.Reference(branchRef, false)
	if err != nil {
		return status, nil
	}

	// Get last commit info
	commit, err := g.repo.CommitObject(ref.Hash())
	if err == nil {
		status.LastCommit = commit.Hash.String()[:8]
		status.LastCommitMsg = strings.Split(commit.Message, "\n")[0]
	}

	// Check if branch is tracked (has upstream)
	cmd := exec.Command("git", "branch", "-vv")
	cmd.Dir = g.repoPath
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, branch) && strings.Contains(line, "[") {
				status.IsTracked = true
				// Parse ahead/behind info from git branch -vv output
				if strings.Contains(line, "ahead") {
					status.HasUnpushed = true
					// Extract ahead count with regex
					re := regexp.MustCompile(`ahead (\d+)`)
					if matches := re.FindStringSubmatch(line); len(matches) > 1 {
						if count, err := strconv.Atoi(matches[1]); err == nil {
							status.AheadBy = count
						}
					}
				}
				if strings.Contains(line, "behind") {
					status.HasUnpulled = true
					// Extract behind count with regex
					re := regexp.MustCompile(`behind (\d+)`)
					if matches := re.FindStringSubmatch(line); len(matches) > 1 {
						if count, err := strconv.Atoi(matches[1]); err == nil {
							status.BehindBy = count
						}
					}
				}
				break
			}
		}
	}

	return status, nil
}

// PushBranch pushes a branch to the remote repository
func (g *GitClient) PushBranch(ctx context.Context, branch string) error {
	cmd := exec.Command("git", "push", "origin", branch)
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "GitClient.PushBranch", "push_failed: "+string(output))
	}

	return nil
}

// PullBranch pulls changes from the remote repository
func (g *GitClient) PullBranch(ctx context.Context, branch string) error {
	// First, fetch from origin
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = g.repoPath
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "GitClient.PullBranch", "fetch_failed")
	}

	// Then merge origin/branch into current branch
	cmd = exec.Command("git", "merge", "origin/"+branch)
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "GitClient.PullBranch", "merge_failed: "+string(output))
	}

	return nil
}
