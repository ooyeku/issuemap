package services

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// SearchService provides advanced search capabilities
type SearchService struct {
	issueRepo repositories.IssueRepository
}

// NewSearchService creates a new search service
func NewSearchService(issueRepo repositories.IssueRepository) *SearchService {
	return &SearchService{
		issueRepo: issueRepo,
	}
}

// SearchQuery represents a parsed search query
type SearchQuery struct {
	Text         string                 `json:"text,omitempty"`
	Filters      map[string]interface{} `json:"filters,omitempty"`
	DateFilters  map[string]DateFilter  `json:"date_filters,omitempty"`
	BoolOperator string                 `json:"bool_operator,omitempty"` // AND, OR
	Negated      []string               `json:"negated,omitempty"`       // Fields to negate
	SortBy       string                 `json:"sort_by,omitempty"`
	SortOrder    string                 `json:"sort_order,omitempty"` // asc, desc
	Limit        int                    `json:"limit,omitempty"`
}

// DateFilter represents date-based filtering
type DateFilter struct {
	Operator string    `json:"operator"` // >, <, >=, <=, =
	Value    time.Time `json:"value"`
	Relative string    `json:"relative,omitempty"` // 7d, 1w, 1m, etc.
}

// ParseSearchQuery parses a natural language search query into structured filters
func (s *SearchService) ParseSearchQuery(query string) (*SearchQuery, error) {
	sq := &SearchQuery{
		Filters:      make(map[string]interface{}),
		DateFilters:  make(map[string]DateFilter),
		BoolOperator: "AND", // default
		Limit:        50,    // default
	}

	// Split query into tokens while preserving quoted strings
	tokens := tokenizeQuery(query)

	var textTokens []string
	i := 0

	for i < len(tokens) {
		token := tokens[i]

		// Check for field:value patterns
		if strings.Contains(token, ":") && !strings.HasPrefix(token, "\"") {
			parts := strings.SplitN(token, ":", 2)
			if len(parts) == 2 {
				field := strings.ToLower(parts[0])
				value := parts[1]

				// Remove quotes if present
				if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
					value = strings.Trim(value, "\"")
				}

				err := s.parseFieldFilter(sq, field, value)
				if err != nil {
					return nil, fmt.Errorf("invalid filter %s:%s - %w", field, value, err)
				}
				i++
				continue
			}
		}

		// Check for boolean operators
		upperToken := strings.ToUpper(token)
		if upperToken == "AND" || upperToken == "OR" {
			sq.BoolOperator = upperToken
			i++
			continue
		}

		// Check for NOT operator
		if upperToken == "NOT" {
			if i+1 < len(tokens) {
				nextToken := tokens[i+1]
				if strings.Contains(nextToken, ":") {
					parts := strings.SplitN(nextToken, ":", 2)
					if len(parts) == 2 {
						sq.Negated = append(sq.Negated, strings.ToLower(parts[0]))
						i += 2
						continue
					}
				}
			}
		}

		// Check for sort directives
		if strings.HasPrefix(strings.ToLower(token), "sort:") {
			sortValue := strings.TrimPrefix(strings.ToLower(token), "sort:")
			parts := strings.SplitN(sortValue, ":", 2)
			sq.SortBy = parts[0]
			if len(parts) == 2 && (parts[1] == "desc" || parts[1] == "asc") {
				sq.SortOrder = parts[1]
			} else {
				sq.SortOrder = "desc" // default
			}
			i++
			continue
		}

		// Check for limit directive
		if strings.HasPrefix(strings.ToLower(token), "limit:") {
			limitStr := strings.TrimPrefix(strings.ToLower(token), "limit:")
			if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
				sq.Limit = limit
			}
			i++
			continue
		}

		// Otherwise, it's a text search token
		textTokens = append(textTokens, token)
		i++
	}

	// Join remaining tokens as text search
	if len(textTokens) > 0 {
		sq.Text = strings.Join(textTokens, " ")
		// Remove quotes from text search
		sq.Text = strings.Trim(sq.Text, "\"")
	}

	return sq, nil
}

// parseFieldFilter parses field-specific filters
func (s *SearchService) parseFieldFilter(sq *SearchQuery, field, value string) error {
	switch field {
	case "type":
		if !isValidIssueType(value) {
			return fmt.Errorf("invalid issue type: %s", value)
		}
		sq.Filters["type"] = entities.IssueType(value)

	case "status":
		if !isValidStatus(value) {
			return fmt.Errorf("invalid status: %s", value)
		}
		sq.Filters["status"] = entities.Status(value)

	case "priority":
		if !isValidPriority(value) {
			return fmt.Errorf("invalid priority: %s", value)
		}
		sq.Filters["priority"] = entities.Priority(value)

	case "assignee":
		sq.Filters["assignee"] = value

	case "branch":
		sq.Filters["branch"] = value

	case "milestone":
		sq.Filters["milestone"] = value

	case "labels", "label":
		// Handle comma-separated labels
		labels := strings.Split(value, ",")
		for i, label := range labels {
			labels[i] = strings.TrimSpace(label)
		}
		sq.Filters["labels"] = labels

	case "created", "updated":
		dateFilter, err := parseDateFilter(value)
		if err != nil {
			return fmt.Errorf("invalid date filter: %w", err)
		}
		sq.DateFilters[field] = dateFilter

	default:
		return fmt.Errorf("unknown filter field: %s", field)
	}

	return nil
}

// ExecuteSearch executes a parsed search query
func (s *SearchService) ExecuteSearch(ctx context.Context, searchQuery *SearchQuery) (*repositories.SearchResult, error) {
	// Convert SearchQuery to repositories.SearchQuery
	repoQuery := repositories.SearchQuery{
		Text: searchQuery.Text,
	}

	// Build filter from parsed query
	filter := repositories.IssueFilter{}

	if status, ok := searchQuery.Filters["status"].(entities.Status); ok {
		filter.Status = &status
	}

	if issueType, ok := searchQuery.Filters["type"].(entities.IssueType); ok {
		filter.Type = &issueType
	}

	if priority, ok := searchQuery.Filters["priority"].(entities.Priority); ok {
		filter.Priority = &priority
	}

	if assignee, ok := searchQuery.Filters["assignee"].(string); ok {
		filter.Assignee = &assignee
	}

	if branch, ok := searchQuery.Filters["branch"].(string); ok {
		filter.Branch = &branch
	}

	if milestone, ok := searchQuery.Filters["milestone"].(string); ok {
		filter.Milestone = &milestone
	}

	if labels, ok := searchQuery.Filters["labels"].([]string); ok {
		filter.Labels = labels
	}

	// Handle date filters
	if createdFilter, ok := searchQuery.DateFilters["created"]; ok {
		if createdFilter.Operator == ">" || createdFilter.Operator == ">=" {
			filter.CreatedSince = &createdFilter.Value
		}
	}

	if updatedFilter, ok := searchQuery.DateFilters["updated"]; ok {
		if updatedFilter.Operator == ">" || updatedFilter.Operator == ">=" {
			filter.UpdatedSince = &updatedFilter.Value
		}
	}

	// Set limit
	if searchQuery.Limit > 0 {
		filter.Limit = &searchQuery.Limit
	}

	repoQuery.Filter = filter

	// Execute search
	result, err := s.issueRepo.Search(ctx, repoQuery)
	if err != nil {
		return nil, errors.Wrap(err, "SearchService.ExecuteSearch", "repo_search")
	}

	// Apply post-processing (sorting, negation, etc.)
	result = s.postProcessResults(result, searchQuery)

	return result, nil
}

// postProcessResults applies additional processing to search results
func (s *SearchService) postProcessResults(result *repositories.SearchResult, searchQuery *SearchQuery) *repositories.SearchResult {
	// Apply negation filters
	if len(searchQuery.Negated) > 0 {
		filtered := make([]entities.Issue, 0, len(result.Issues))
		for _, issue := range result.Issues {
			include := true
			for _, negatedField := range searchQuery.Negated {
				if s.matchesNegatedFilter(issue, negatedField, searchQuery) {
					include = false
					break
				}
			}
			if include {
				filtered = append(filtered, issue)
			}
		}
		result.Issues = filtered
		result.Total = len(filtered)
	}

	// Apply sorting
	if searchQuery.SortBy != "" {
		s.sortResults(result.Issues, searchQuery.SortBy, searchQuery.SortOrder)
	}

	return result
}

// matchesNegatedFilter checks if an issue matches a negated filter
func (s *SearchService) matchesNegatedFilter(issue entities.Issue, field string, searchQuery *SearchQuery) bool {
	switch field {
	case "type":
		if filterType, ok := searchQuery.Filters["type"].(entities.IssueType); ok {
			return issue.Type == filterType
		}
	case "status":
		if filterStatus, ok := searchQuery.Filters["status"].(entities.Status); ok {
			return issue.Status == filterStatus
		}
	case "priority":
		if filterPriority, ok := searchQuery.Filters["priority"].(entities.Priority); ok {
			return issue.Priority == filterPriority
		}
	case "assignee":
		if filterAssignee, ok := searchQuery.Filters["assignee"].(string); ok {
			return issue.Assignee != nil && issue.Assignee.Username == filterAssignee
		}
	}
	return false
}

// sortResults sorts issues by the specified field and order
func (s *SearchService) sortResults(issues []entities.Issue, sortBy, sortOrder string) {
	// Implementation would go here - for now just a placeholder
	// In a real implementation, you'd sort based on the field and order
}

// Helper functions

func tokenizeQuery(query string) []string {
	// Regex to split on spaces but preserve quoted strings
	re := regexp.MustCompile(`[^\s"']+|"([^"]*)"|'([^']*)'`)
	matches := re.FindAllString(query, -1)

	var tokens []string
	for _, match := range matches {
		// Remove outer quotes but keep the content
		if (strings.HasPrefix(match, "\"") && strings.HasSuffix(match, "\"")) ||
			(strings.HasPrefix(match, "'") && strings.HasSuffix(match, "'")) {
			tokens = append(tokens, match)
		} else {
			tokens = append(tokens, match)
		}
	}

	return tokens
}

func parseDateFilter(value string) (DateFilter, error) {
	// Handle relative dates like "7d", "1w", "1m"
	if matched, _ := regexp.MatchString(`^\d+[dwmy]$`, value); matched {
		return parseRelativeDate(value)
	}

	// Handle operator-based dates like ">2024-01-01", "<7d"
	if len(value) > 1 {
		var operator string
		var dateStr string

		if strings.HasPrefix(value, ">=") {
			operator = ">="
			dateStr = value[2:]
		} else if strings.HasPrefix(value, "<=") {
			operator = "<="
			dateStr = value[2:]
		} else if strings.HasPrefix(value, ">") {
			operator = ">"
			dateStr = value[1:]
		} else if strings.HasPrefix(value, "<") {
			operator = "<"
			dateStr = value[1:]
		} else {
			operator = "="
			dateStr = value
		}

		// Try to parse as relative date first
		if matched, _ := regexp.MatchString(`^\d+[dwmy]$`, dateStr); matched {
			filter, err := parseRelativeDate(dateStr)
			if err != nil {
				return DateFilter{}, err
			}
			filter.Operator = operator
			return filter, nil
		}

		// Try to parse as absolute date
		date, err := parseAbsoluteDate(dateStr)
		if err != nil {
			return DateFilter{}, err
		}

		return DateFilter{
			Operator: operator,
			Value:    date,
		}, nil
	}

	// Default to absolute date parsing
	date, err := parseAbsoluteDate(value)
	if err != nil {
		return DateFilter{}, err
	}

	return DateFilter{
		Operator: "=",
		Value:    date,
	}, nil
}

func parseRelativeDate(relative string) (DateFilter, error) {
	re := regexp.MustCompile(`^(\d+)([dwmy])$`)
	matches := re.FindStringSubmatch(relative)
	if len(matches) != 3 {
		return DateFilter{}, fmt.Errorf("invalid relative date format: %s", relative)
	}

	amount, err := strconv.Atoi(matches[1])
	if err != nil {
		return DateFilter{}, fmt.Errorf("invalid number in relative date: %s", matches[1])
	}

	unit := matches[2]
	now := time.Now()
	var date time.Time

	switch unit {
	case "d":
		date = now.AddDate(0, 0, -amount)
	case "w":
		date = now.AddDate(0, 0, -amount*7)
	case "m":
		date = now.AddDate(0, -amount, 0)
	case "y":
		date = now.AddDate(-amount, 0, 0)
	default:
		return DateFilter{}, fmt.Errorf("unsupported time unit: %s", unit)
	}

	return DateFilter{
		Operator: ">",
		Value:    date,
		Relative: relative,
	}, nil
}

func parseAbsoluteDate(dateStr string) (time.Time, error) {
	// Try different date formats
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"01/02/2006",
		"01-02-2006",
		"Jan 02, 2006",
		"January 02, 2006",
	}

	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized date format: %s", dateStr)
}

func isValidIssueType(value string) bool {
	validTypes := []string{"bug", "feature", "task", "epic", "improvement"}
	for _, t := range validTypes {
		if strings.ToLower(value) == t {
			return true
		}
	}
	return false
}

func isValidStatus(value string) bool {
	validStatuses := []string{"open", "in-progress", "review", "done", "closed"}
	for _, s := range validStatuses {
		if strings.ToLower(value) == s {
			return true
		}
	}
	return false
}

func isValidPriority(value string) bool {
	validPriorities := []string{"low", "medium", "high", "critical"}
	for _, p := range validPriorities {
		if strings.ToLower(value) == p {
			return true
		}
	}
	return false
}
