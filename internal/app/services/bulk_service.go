package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
)

// BulkOptions controls behavior for bulk operations
type BulkOptions struct {
	DryRun   bool
	Rollback bool
	Author   string
	Metadata map[string]string
	// Progress is an optional callback invoked for each processed item
	Progress func(completed, total int, id entities.IssueID, err error) `json:"-" yaml:"-"`
}

// BulkResult captures results of a bulk operation
type BulkResult struct {
	Operation   string
	Query       string
	StartedAt   time.Time
	CompletedAt time.Time
	Total       int
	Succeeded   int
	Failed      int
	SuccessIDs  []entities.IssueID
	FailedIDs   []entities.IssueID
	Errors      map[entities.IssueID]string
	DryRun      bool
}

// BulkService provides bulk operations on issues
type BulkService struct {
	issueService  *IssueService
	searchService *SearchService
	basePath      string // absolute path to .issuemap for audit logs
}

func NewBulkService(issueService *IssueService, searchService *SearchService, basePath string) *BulkService {
	return &BulkService{
		issueService:  issueService,
		searchService: searchService,
		basePath:      basePath,
	}
}

// SelectIssues returns issues matching the provided search query string
func (s *BulkService) SelectIssues(ctx context.Context, query string) ([]*entities.Issue, error) {
	parsed, err := s.searchService.ParseSearchQuery(query)
	if err != nil {
		return nil, errors.Wrap(err, "BulkService.SelectIssues", "parse_query")
	}
	result, err := s.searchService.ExecuteSearch(ctx, parsed)
	if err != nil {
		return nil, errors.Wrap(err, "BulkService.SelectIssues", "execute_search")
	}
	// Convert to pointers for consistency
	issues := make([]*entities.Issue, 0, len(result.Issues))
	for i := range result.Issues {
		issue := result.Issues[i]
		issues = append(issues, &issue)
	}
	return issues, nil
}

// BulkAssign assigns or unassigns multiple issues
func (s *BulkService) BulkAssign(ctx context.Context, issues []*entities.Issue, username string, opts BulkOptions) (*BulkResult, error) {
	op := "assign"
	if strings.TrimSpace(username) == "" {
		op = "unassign"
	}
	if err := s.validateBeforeApply(issues, map[string]string{"assignee": username}); err != nil {
		return nil, err
	}
	return s.applyBulk(ctx, issues, op, opts, func(issue *entities.Issue) error {
		if opts.DryRun {
			return nil
		}
		updates := map[string]interface{}{"assignee": username}
		_, err := s.issueService.UpdateIssue(ctx, issue.ID, updates)
		return err
	})
}

// BulkStatus updates the status of multiple issues
func (s *BulkService) BulkStatus(ctx context.Context, issues []*entities.Issue, status string, opts BulkOptions) (*BulkResult, error) {
	if err := s.validateBeforeApply(issues, map[string]string{"status": status}); err != nil {
		return nil, err
	}
	return s.applyBulk(ctx, issues, "status", opts, func(issue *entities.Issue) error {
		if opts.DryRun {
			return nil
		}
		updates := map[string]interface{}{"status": status}
		_, err := s.issueService.UpdateIssue(ctx, issue.ID, updates)
		return err
	})
}

// BulkLabels performs add/remove/replace on labels across issues
// If setLabels is non-empty, labels will be replaced by this set (override add/remove)
func (s *BulkService) BulkLabels(ctx context.Context, issues []*entities.Issue, addLabels, removeLabels, setLabels []string, opts BulkOptions) (*BulkResult, error) {
	// Normalize label lists
	normalize := func(in []string) []string {
		out := make([]string, 0, len(in))
		for _, l := range in {
			l = strings.TrimSpace(l)
			if l != "" {
				out = append(out, l)
			}
		}
		return out
	}
	addLabels = normalize(addLabels)
	removeLabels = normalize(removeLabels)
	setLabels = normalize(setLabels)

	if len(addLabels) == 0 && len(removeLabels) == 0 && len(setLabels) == 0 {
		return nil, errors.Wrap(fmt.Errorf("no label operations specified"), "BulkService.BulkLabels", "validate")
	}

	// No hard validation needed for label names aside from non-empty
	return s.applyBulk(ctx, issues, "labels", opts, func(issue *entities.Issue) error {
		if opts.DryRun {
			return nil
		}
		if len(setLabels) > 0 {
			updates := map[string]interface{}{"labels": setLabels}
			_, err := s.issueService.UpdateIssue(ctx, issue.ID, updates)
			return err
		}

		// Compute new label set from existing + add/remove
		existing := make(map[string]struct{})
		for _, lbl := range issue.Labels {
			existing[lbl.Name] = struct{}{}
		}
		for _, l := range addLabels {
			existing[l] = struct{}{}
		}
		for _, l := range removeLabels {
			delete(existing, l)
		}
		final := make([]string, 0, len(existing))
		for name := range existing {
			final = append(final, name)
		}
		sort.Strings(final)
		updates := map[string]interface{}{"labels": final}
		_, err := s.issueService.UpdateIssue(ctx, issue.ID, updates)
		return err
	})
}

// ExportIssuesCSV writes the provided issues to a CSV file (or stdout if outputPath is empty)
func (s *BulkService) ExportIssuesCSV(_ context.Context, issues []*entities.Issue, outputPath string) error {
	var out *os.File
	var err error
	if outputPath == "" {
		out = os.Stdout
	} else {
		out, err = os.Create(outputPath)
		if err != nil {
			return errors.Wrap(err, "BulkService.ExportIssuesCSV", "create")
		}
		defer out.Close()
	}

	w := csv.NewWriter(out)
	defer w.Flush()

	header := []string{"issue_id", "title", "status", "priority", "assignee", "labels", "milestone", "branch", "created", "updated"}
	if err := w.Write(header); err != nil {
		return errors.Wrap(err, "BulkService.ExportIssuesCSV", "write_header")
	}

	for _, is := range issues {
		assignee := ""
		if is.Assignee != nil {
			assignee = is.Assignee.Username
		}
		labels := make([]string, 0, len(is.Labels))
		for _, l := range is.Labels {
			labels = append(labels, l.Name)
		}
		milestone := ""
		if is.Milestone != nil {
			milestone = is.Milestone.Name
		}
		record := []string{
			string(is.ID),
			is.Title,
			string(is.Status),
			string(is.Priority),
			assignee,
			strings.Join(labels, ","),
			milestone,
			is.Branch,
			is.Timestamps.Created.Format("2006-01-02 15:04:05"),
			is.Timestamps.Updated.Format("2006-01-02 15:04:05"),
		}
		if err := w.Write(record); err != nil {
			return errors.Wrap(err, "BulkService.ExportIssuesCSV", "write_record")
		}
	}
	return nil
}

// ImportUpdatesCSV reads a CSV of updates and applies them transactionally
// Expected columns: issue_id, assignee, status, labels
func (s *BulkService) ImportUpdatesCSV(ctx context.Context, inputPath string, opts BulkOptions) (*BulkResult, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return nil, errors.Wrap(err, "BulkService.ImportUpdatesCSV", "open")
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, errors.Wrap(err, "BulkService.ImportUpdatesCSV", "read")
	}
	if len(rows) == 0 {
		return nil, errors.Wrap(fmt.Errorf("empty CSV"), "BulkService.ImportUpdatesCSV", "validate")
	}
	header := rows[0]
	colIdx := func(name string) int {
		for i, h := range header {
			if strings.EqualFold(h, name) {
				return i
			}
		}
		return -1
	}

	idxIssue := colIdx("issue_id")
	if idxIssue == -1 {
		return nil, errors.Wrap(fmt.Errorf("column 'issue_id' required"), "BulkService.ImportUpdatesCSV", "validate_columns")
	}
	idxAssignee := colIdx("assignee")
	idxStatus := colIdx("status")
	idxLabels := colIdx("labels")

	// Load all issues referenced
	var issues []*entities.Issue
	issueByID := map[entities.IssueID]*entities.Issue{}
	for _, row := range rows[1:] {
		if len(row) <= idxIssue {
			continue
		}
		id := entities.IssueID(strings.TrimSpace(row[idxIssue]))
		if id == "" {
			continue
		}
		if _, ok := issueByID[id]; ok {
			continue
		}
		issue, err := s.issueService.GetIssue(ctx, id)
		if err != nil {
			return nil, errors.Wrap(err, "BulkService.ImportUpdatesCSV", "get_issue")
		}
		issues = append(issues, issue)
		issueByID[id] = issue
	}

	// Validation pass
	for _, row := range rows[1:] {
		if len(row) <= idxIssue {
			continue
		}
		id := entities.IssueID(strings.TrimSpace(row[idxIssue]))
		if id == "" {
			continue
		}
		if idxStatus >= 0 && idxStatus < len(row) {
			status := strings.TrimSpace(row[idxStatus])
			if status != "" && !isValidStatus(status) {
				return nil, errors.Wrap(fmt.Errorf("invalid status '%s' for %s", status, id), "BulkService.ImportUpdatesCSV", "validate_status")
			}
		}
	}

	// Apply
	return s.applyBulk(ctx, issues, "import_csv", opts, func(issue *entities.Issue) error {
		if opts.DryRun {
			return nil
		}
		row := findRowByIssue(rows, string(issue.ID), idxIssue)
		if row == nil {
			return nil
		}
		updates := map[string]interface{}{}
		if idxAssignee >= 0 && idxAssignee < len(row) {
			updates["assignee"] = strings.TrimSpace(row[idxAssignee])
		}
		if idxStatus >= 0 && idxStatus < len(row) {
			val := strings.TrimSpace(row[idxStatus])
			if val != "" {
				updates["status"] = val
			}
		}
		if idxLabels >= 0 && idxLabels < len(row) {
			val := strings.TrimSpace(row[idxLabels])
			if val != "" {
				parts := strings.Split(val, ",")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}
				updates["labels"] = parts
			}
		}
		if len(updates) == 0 {
			return nil
		}
		_, err := s.issueService.UpdateIssue(ctx, issue.ID, updates)
		return err
	})
}

func findRowByIssue(rows [][]string, issueID string, idxIssue int) []string {
	for _, row := range rows[1:] {
		if idxIssue < len(row) && strings.TrimSpace(row[idxIssue]) == issueID {
			return row
		}
	}
	return nil
}

// applyBulk applies a per-issue function with optional rollback and audit logging
func (s *BulkService) applyBulk(ctx context.Context, issues []*entities.Issue, operation string, opts BulkOptions, apply func(*entities.Issue) error) (*BulkResult, error) {
	result := &BulkResult{
		Operation:  operation,
		StartedAt:  time.Now(),
		Total:      len(issues),
		Errors:     map[entities.IssueID]string{},
		DryRun:     opts.DryRun,
		SuccessIDs: []entities.IssueID{},
		FailedIDs:  []entities.IssueID{},
	}

	// Snapshot originals for rollback
	originals := map[entities.IssueID]*entities.Issue{}
	for _, is := range issues {
		originals[is.ID] = cloneIssue(is)
	}

	// Apply sequentially (file backend); can be parallelized later
	for _, is := range issues {
		if err := apply(is); err != nil {
			result.Failed++
			result.FailedIDs = append(result.FailedIDs, is.ID)
			result.Errors[is.ID] = err.Error()
			if opts.Progress != nil {
				opts.Progress(result.Succeeded+result.Failed, result.Total, is.ID, err)
			}
			if opts.Rollback && !opts.DryRun {
				// rollback previously succeeded
				for _, okID := range result.SuccessIDs {
					orig := originals[okID]
					_ = s.issueService.issueRepo.Update(ctx, orig)
				}
			}
			result.CompletedAt = time.Now()
			_ = s.writeAuditLog(result, opts)
			return result, err
		}
		result.Succeeded++
		result.SuccessIDs = append(result.SuccessIDs, is.ID)
		if opts.Progress != nil {
			opts.Progress(result.Succeeded+result.Failed, result.Total, is.ID, nil)
		}
	}

	result.CompletedAt = time.Now()
	_ = s.writeAuditLog(result, opts)
	return result, nil
}

// validateBeforeApply performs a preflight validation to fail-fast before any change
func (s *BulkService) validateBeforeApply(issues []*entities.Issue, props map[string]string) error {
	if len(issues) == 0 {
		return errors.Wrap(fmt.Errorf("no matching issues for query"), "BulkService.validateBeforeApply", "empty_selection")
	}
	if v, ok := props["status"]; ok && v != "" && !isValidStatus(v) {
		return errors.Wrap(fmt.Errorf("invalid status: %s", v), "BulkService.validateBeforeApply", "status")
	}
	// assignee can be empty string for unassign; labels validated elsewhere
	return nil
}

// writeAuditLog persists a YAML log of the bulk operation
func (s *BulkService) writeAuditLog(res *BulkResult, opts BulkOptions) error {
	// Ensure directory
	dir := filepath.Join(s.basePath, app.MetadataDirName, "bulk_logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	ts := res.StartedAt.Format("20060102_150405")
	fname := fmt.Sprintf("%s_%s.yaml", ts, res.Operation)
	path := filepath.Join(dir, fname)
	payload := map[string]interface{}{
		"operation":    res.Operation,
		"started_at":   res.StartedAt,
		"completed_at": res.CompletedAt,
		"total":        res.Total,
		"succeeded":    res.Succeeded,
		"failed":       res.Failed,
		"success_ids":  res.SuccessIDs,
		"failed_ids":   res.FailedIDs,
		"errors":       res.Errors,
		"dry_run":      res.DryRun,
		"options":      opts,
	}
	data, err := yaml.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// cloneIssue makes a deep copy of the issue for rollback
func cloneIssue(src *entities.Issue) *entities.Issue {
	cp := *src
	if src.Assignee != nil {
		u := *src.Assignee
		cp.Assignee = &u
	}
	if src.Milestone != nil {
		m := *src.Milestone
		cp.Milestone = &m
	}
	if len(src.Labels) > 0 {
		cp.Labels = make([]entities.Label, len(src.Labels))
		copy(cp.Labels, src.Labels)
	}
	if len(src.Commits) > 0 {
		cp.Commits = make([]entities.CommitRef, len(src.Commits))
		copy(cp.Commits, src.Commits)
	}
	if len(src.Comments) > 0 {
		cp.Comments = make([]entities.Comment, len(src.Comments))
		copy(cp.Comments, src.Comments)
	}
	if src.Metadata.CustomFields != nil {
		m := make(map[string]string, len(src.Metadata.CustomFields))
		for k, v := range src.Metadata.CustomFields {
			m[k] = v
		}
		cp.Metadata.CustomFields = m
	}
	return &cp
}

// status validation is provided elsewhere in the services package
