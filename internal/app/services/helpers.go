package services

import "github.com/ooyeku/issuemap/internal/domain/repositories"

// RepositoriesIssueFilterNone returns an empty IssueFilter to list all issues.
func RepositoriesIssueFilterNone() repositories.IssueFilter { return repositories.IssueFilter{} }
