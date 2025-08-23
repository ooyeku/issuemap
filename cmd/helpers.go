package cmd

import (
	"context"

	"github.com/ooyeku/issuemap/internal/infrastructure/git"
)

// getCurrentUser returns the current user from git config or falls back to unknown
func getCurrentUser(gitRepo *git.GitClient) string {
	ctx := context.Background()
	if gitRepo != nil {
		if author, err := gitRepo.GetAuthorInfo(ctx); err == nil && author != nil {
			if author.Username != "" {
				return author.Username
			}
			if author.Email != "" {
				return author.Email
			}
		}
	}
	return "unknown"
}
