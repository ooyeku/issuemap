package storage

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// FileIssueRepository implements the IssueRepository interface using file storage
type FileIssueRepository struct {
	basePath string
}

// NewFileIssueRepository creates a new file-based issue repository
func NewFileIssueRepository(basePath string) *FileIssueRepository {
	return &FileIssueRepository{
		basePath: basePath,
	}
}

// GetBasePath returns the repository base path for issue storage
func (r *FileIssueRepository) GetBasePath() string { return r.basePath }

// Create creates a new issue file
func (r *FileIssueRepository) Create(ctx context.Context, issue *entities.Issue) error {
	if err := issue.Validate(); err != nil {
		return errors.Wrap(err, "FileIssueRepository.Create", "validation")
	}

	issuesDir := filepath.Join(r.basePath, "issues")
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		return errors.Wrap(err, "FileIssueRepository.Create", "mkdir")
	}

	filePath := filepath.Join(issuesDir, fmt.Sprintf("%s.yaml", issue.ID))
	if _, err := os.Stat(filePath); err == nil {
		return errors.Wrap(errors.ErrIssueAlreadyExists, "FileIssueRepository.Create", "exists")
	}

	data, err := yaml.Marshal(issue)
	if err != nil {
		return errors.Wrap(err, "FileIssueRepository.Create", "marshal")
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return errors.Wrap(err, "FileIssueRepository.Create", "write")
	}

	return nil
}

// GetByID retrieves an issue by its ID
func (r *FileIssueRepository) GetByID(ctx context.Context, id entities.IssueID) (*entities.Issue, error) {
	filePath := filepath.Join(r.basePath, "issues", fmt.Sprintf("%s.yaml", id))

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrap(errors.ErrIssueNotFound, "FileIssueRepository.GetByID", "not_found")
		}
		return nil, errors.Wrap(err, "FileIssueRepository.GetByID", "read")
	}

	var issue entities.Issue
	if err := yaml.Unmarshal(data, &issue); err != nil {
		return nil, errors.Wrap(err, "FileIssueRepository.GetByID", "unmarshal")
	}

	return &issue, nil
}

// List retrieves issues based on filter criteria
func (r *FileIssueRepository) List(ctx context.Context, filter repositories.IssueFilter) (*repositories.IssueList, error) {
	issuesDir := filepath.Join(r.basePath, "issues")

	files, err := ioutil.ReadDir(issuesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &repositories.IssueList{Issues: []entities.Issue{}, Total: 0, Count: 0}, nil
		}
		return nil, errors.Wrap(err, "FileIssueRepository.List", "read_dir")
	}

	var allIssues []entities.Issue
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		filePath := filepath.Join(issuesDir, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			continue // Skip files that can't be read
		}

		var issue entities.Issue
		if err := yaml.Unmarshal(data, &issue); err != nil {
			continue // Skip files that can't be parsed
		}

		if r.matchesFilter(&issue, filter) {
			allIssues = append(allIssues, issue)
		}
	}

	// Sort by creation date (newest first)
	sort.Slice(allIssues, func(i, j int) bool {
		return allIssues[i].Timestamps.Created.After(allIssues[j].Timestamps.Created)
	})

	// Apply pagination
	total := len(allIssues)
	start := 0
	end := total

	if filter.Offset != nil {
		start = *filter.Offset
		if start > total {
			start = total
		}
	}

	if filter.Limit != nil {
		end = start + *filter.Limit
		if end > total {
			end = total
		}
	}

	if start > end {
		start = end
	}

	pagedIssues := allIssues[start:end]

	return &repositories.IssueList{
		Issues: pagedIssues,
		Total:  total,
		Count:  len(pagedIssues),
	}, nil
}

// Update updates an existing issue
func (r *FileIssueRepository) Update(ctx context.Context, issue *entities.Issue) error {
	if err := issue.Validate(); err != nil {
		return errors.Wrap(err, "FileIssueRepository.Update", "validation")
	}

	filePath := filepath.Join(r.basePath, "issues", fmt.Sprintf("%s.yaml", issue.ID))

	// Check if issue exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return errors.Wrap(errors.ErrIssueNotFound, "FileIssueRepository.Update", "not_found")
	}

	// Update the timestamp
	issue.Timestamps.Updated = time.Now()

	data, err := yaml.Marshal(issue)
	if err != nil {
		return errors.Wrap(err, "FileIssueRepository.Update", "marshal")
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return errors.Wrap(err, "FileIssueRepository.Update", "write")
	}

	return nil
}

// Delete removes an issue
func (r *FileIssueRepository) Delete(ctx context.Context, id entities.IssueID) error {
	filePath := filepath.Join(r.basePath, "issues", fmt.Sprintf("%s.yaml", id))

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return errors.Wrap(errors.ErrIssueNotFound, "FileIssueRepository.Delete", "not_found")
		}
		return errors.Wrap(err, "FileIssueRepository.Delete", "remove")
	}

	return nil
}

// Search performs a text search across issues
func (r *FileIssueRepository) Search(ctx context.Context, query repositories.SearchQuery) (*repositories.SearchResult, error) {
	start := time.Now()

	// Get all issues first
	allIssues, err := r.List(ctx, query.Filter)
	if err != nil {
		return nil, errors.Wrap(err, "FileIssueRepository.Search", "list")
	}

	if query.Text == "" {
		return &repositories.SearchResult{
			Issues:   allIssues.Issues,
			Total:    allIssues.Total,
			Query:    query.Text,
			Duration: time.Since(start).String(),
		}, nil
	}

	// Perform text search
	var matchedIssues []entities.Issue
	searchText := strings.ToLower(query.Text)

	for _, issue := range allIssues.Issues {
		if r.matchesSearchText(&issue, searchText, query.Fields) {
			matchedIssues = append(matchedIssues, issue)
		}
	}

	return &repositories.SearchResult{
		Issues:   matchedIssues,
		Total:    len(matchedIssues),
		Query:    query.Text,
		Duration: time.Since(start).String(),
	}, nil
}

// GetNextID returns the next available issue ID
func (r *FileIssueRepository) GetNextID(ctx context.Context) (entities.IssueID, error) {
	issuesDir := filepath.Join(r.basePath, "issues")

	files, err := ioutil.ReadDir(issuesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return entities.NewIssueID(1), nil
		}
		return "", errors.Wrap(err, "FileIssueRepository.GetNextID", "read_dir")
	}

	maxID := 0
	re := regexp.MustCompile(`ISSUE-(\d+)\.yaml`)

	for _, file := range files {
		matches := re.FindStringSubmatch(file.Name())
		if len(matches) == 2 {
			if id, err := strconv.Atoi(matches[1]); err == nil && id > maxID {
				maxID = id
			}
		}
	}

	return entities.NewIssueID(maxID + 1), nil
}

// Exists checks if an issue with the given ID exists
func (r *FileIssueRepository) Exists(ctx context.Context, id entities.IssueID) (bool, error) {
	filePath := filepath.Join(r.basePath, "issues", fmt.Sprintf("%s.yaml", id))
	_, err := os.Stat(filePath)

	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, errors.Wrap(err, "FileIssueRepository.Exists", "stat")
}

// ListByStatus retrieves all issues with a specific status
func (r *FileIssueRepository) ListByStatus(ctx context.Context, status entities.Status) ([]*entities.Issue, error) {
	filter := repositories.IssueFilter{Status: &status}
	issueList, err := r.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	var issues []*entities.Issue
	for i := range issueList.Issues {
		issues = append(issues, &issueList.Issues[i])
	}

	return issues, nil
}

// GetStats returns repository statistics
func (r *FileIssueRepository) GetStats(ctx context.Context) (*repositories.RepositoryStats, error) {
	allIssues, err := r.List(ctx, repositories.IssueFilter{})
	if err != nil {
		return nil, errors.Wrap(err, "FileIssueRepository.GetStats", "list")
	}

	stats := &repositories.RepositoryStats{
		TotalIssues:      allIssues.Total,
		IssuesByStatus:   make(map[entities.Status]int),
		IssuesByType:     make(map[entities.IssueType]int),
		IssuesByPriority: make(map[entities.Priority]int),
		IssuesByAssignee: make(map[string]int),
		RecentActivity:   []entities.Issue{},
	}

	if len(allIssues.Issues) == 0 {
		return stats, nil
	}

	// Calculate statistics
	for _, issue := range allIssues.Issues {
		stats.IssuesByStatus[issue.Status]++
		stats.IssuesByType[issue.Type]++
		stats.IssuesByPriority[issue.Priority]++

		if issue.Assignee != nil {
			stats.IssuesByAssignee[issue.Assignee.Username]++
		} else {
			stats.IssuesByAssignee["unassigned"]++
		}
	}

	// Set oldest and newest issues
	if len(allIssues.Issues) > 0 {
		sorted := make([]entities.Issue, len(allIssues.Issues))
		copy(sorted, allIssues.Issues)

		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Timestamps.Created.Before(sorted[j].Timestamps.Created)
		})

		stats.OldestIssue = &sorted[0]
		stats.NewestIssue = &sorted[len(sorted)-1]

		// Recent activity (last 10 updated issues)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Timestamps.Updated.After(sorted[j].Timestamps.Updated)
		})

		limit := 10
		if len(sorted) < limit {
			limit = len(sorted)
		}
		stats.RecentActivity = sorted[:limit]
	}

	return stats, nil
}

// matchesFilter checks if an issue matches the given filter criteria
func (r *FileIssueRepository) matchesFilter(issue *entities.Issue, filter repositories.IssueFilter) bool {
	if filter.Status != nil && issue.Status != *filter.Status {
		return false
	}

	if filter.Type != nil && issue.Type != *filter.Type {
		return false
	}

	if filter.Priority != nil && issue.Priority != *filter.Priority {
		return false
	}

	if filter.Assignee != nil {
		if issue.Assignee == nil {
			return *filter.Assignee == "unassigned" || *filter.Assignee == ""
		}
		if issue.Assignee.Username != *filter.Assignee {
			return false
		}
	}

	if filter.Branch != nil && issue.Branch != *filter.Branch {
		return false
	}

	if filter.Milestone != nil {
		if issue.Milestone == nil {
			return *filter.Milestone == ""
		}
		if issue.Milestone.Name != *filter.Milestone {
			return false
		}
	}

	if len(filter.Labels) > 0 {
		issueLabels := make(map[string]bool)
		for _, label := range issue.Labels {
			issueLabels[label.Name] = true
		}

		for _, requiredLabel := range filter.Labels {
			if !issueLabels[requiredLabel] {
				return false
			}
		}
	}

	if filter.CreatedSince != nil && issue.Timestamps.Created.Before(*filter.CreatedSince) {
		return false
	}

	if filter.UpdatedSince != nil && issue.Timestamps.Updated.Before(*filter.UpdatedSince) {
		return false
	}

	return true
}

// matchesSearchText checks if an issue matches the search text
func (r *FileIssueRepository) matchesSearchText(issue *entities.Issue, searchText string, fields []string) bool {
	if searchText == "" {
		return true
	}

	// Default to searching all fields if none specified
	if len(fields) == 0 {
		fields = []string{"title", "description", "comments"}
	}

	for _, field := range fields {
		switch field {
		case "title":
			if strings.Contains(strings.ToLower(issue.Title), searchText) {
				return true
			}
		case "description":
			if strings.Contains(strings.ToLower(issue.Description), searchText) {
				return true
			}
		case "comments":
			for _, comment := range issue.Comments {
				if strings.Contains(strings.ToLower(comment.Text), searchText) {
					return true
				}
			}
		}
	}

	return false
}
