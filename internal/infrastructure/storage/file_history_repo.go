package storage

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// FileHistoryRepository implements the HistoryRepository interface using file storage
type FileHistoryRepository struct {
	basePath string
}

// NewFileHistoryRepository creates a new file-based history repository
func NewFileHistoryRepository(basePath string) *FileHistoryRepository {
	return &FileHistoryRepository{
		basePath: basePath,
	}
}

// CreateHistory creates a new issue history
func (r *FileHistoryRepository) CreateHistory(ctx context.Context, history *entities.IssueHistory) error {
	historyDir := filepath.Join(r.basePath, "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return errors.Wrap(err, "FileHistoryRepository.CreateHistory", "mkdir")
	}

	filePath := filepath.Join(historyDir, fmt.Sprintf("%s.yaml", history.IssueID))

	data, err := yaml.Marshal(history)
	if err != nil {
		return errors.Wrap(err, "FileHistoryRepository.CreateHistory", "marshal")
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return errors.Wrap(err, "FileHistoryRepository.CreateHistory", "write")
	}

	return nil
}

// GetHistory retrieves the complete history for an issue
func (r *FileHistoryRepository) GetHistory(ctx context.Context, issueID entities.IssueID) (*entities.IssueHistory, error) {
	filePath := filepath.Join(r.basePath, "history", fmt.Sprintf("%s.yaml", issueID))

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty history if none exists
			return entities.NewIssueHistory(issueID), nil
		}
		return nil, errors.Wrap(err, "FileHistoryRepository.GetHistory", "read")
	}

	var history entities.IssueHistory
	if err := yaml.Unmarshal(data, &history); err != nil {
		return nil, errors.Wrap(err, "FileHistoryRepository.GetHistory", "unmarshal")
	}

	return &history, nil
}

// AddEntry adds a new history entry to an issue's history
func (r *FileHistoryRepository) AddEntry(ctx context.Context, entry *entities.HistoryEntry) error {
	// Get existing history
	history, err := r.GetHistory(ctx, entry.IssueID)
	if err != nil {
		return errors.Wrap(err, "FileHistoryRepository.AddEntry", "get_history")
	}

	// Add the new entry
	history.AddEntry(entry)

	// Save the updated history
	return r.CreateHistory(ctx, history)
}

// GetEntry retrieves a specific history entry by ID
func (r *FileHistoryRepository) GetEntry(ctx context.Context, entryID string) (*entities.HistoryEntry, error) {
	// Since we don't know which issue the entry belongs to, we need to search all history files
	historyDir := filepath.Join(r.basePath, "history")

	files, err := ioutil.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrap(errors.ErrNotFound, "FileHistoryRepository.GetEntry", "no_history")
		}
		return nil, errors.Wrap(err, "FileHistoryRepository.GetEntry", "read_dir")
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		filePath := filepath.Join(historyDir, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			continue
		}

		var history entities.IssueHistory
		if err := yaml.Unmarshal(data, &history); err != nil {
			continue
		}

		// Search for the entry in this history
		for _, entry := range history.Entries {
			if entry.ID == entryID {
				return &entry, nil
			}
		}
	}

	return nil, errors.Wrap(errors.ErrNotFound, "FileHistoryRepository.GetEntry", "entry_not_found")
}

// ListEntries retrieves history entries based on filter criteria
func (r *FileHistoryRepository) ListEntries(ctx context.Context, filter repositories.HistoryFilter) (*repositories.HistoryList, error) {
	var allEntries []entities.HistoryEntry

	if filter.IssueID != nil {
		// Get history for specific issue
		history, err := r.GetHistory(ctx, *filter.IssueID)
		if err != nil {
			return nil, errors.Wrap(err, "FileHistoryRepository.ListEntries", "get_history")
		}
		allEntries = history.Entries
	} else {
		// Get all histories
		histories, err := r.GetAllHistory(ctx, filter)
		if err != nil {
			return nil, errors.Wrap(err, "FileHistoryRepository.ListEntries", "get_all_history")
		}

		// Collect all entries
		for _, history := range histories {
			allEntries = append(allEntries, history.Entries...)
		}
	}

	// Apply filters
	filteredEntries := r.applyFilters(allEntries, filter)

	// Sort by timestamp (newest first)
	sort.Slice(filteredEntries, func(i, j int) bool {
		return filteredEntries[i].Timestamp.After(filteredEntries[j].Timestamp)
	})

	// Apply pagination
	total := len(filteredEntries)
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

	pagedEntries := filteredEntries[start:end]

	return &repositories.HistoryList{
		Entries: pagedEntries,
		Total:   total,
		Count:   len(pagedEntries),
	}, nil
}

// GetAllHistory retrieves all issue histories
func (r *FileHistoryRepository) GetAllHistory(ctx context.Context, filter repositories.HistoryFilter) ([]*entities.IssueHistory, error) {
	historyDir := filepath.Join(r.basePath, "history")

	files, err := ioutil.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*entities.IssueHistory{}, nil
		}
		return nil, errors.Wrap(err, "FileHistoryRepository.GetAllHistory", "read_dir")
	}

	var histories []*entities.IssueHistory

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		filePath := filepath.Join(historyDir, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			continue // Skip files that can't be read
		}

		var history entities.IssueHistory
		if err := yaml.Unmarshal(data, &history); err != nil {
			continue // Skip files that can't be parsed
		}

		histories = append(histories, &history)
	}

	return histories, nil
}

// DeleteHistory removes all history for an issue
func (r *FileHistoryRepository) DeleteHistory(ctx context.Context, issueID entities.IssueID) error {
	filePath := filepath.Join(r.basePath, "history", fmt.Sprintf("%s.yaml", issueID))

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted, no error
		}
		return errors.Wrap(err, "FileHistoryRepository.DeleteHistory", "remove")
	}

	return nil
}

// GetHistoryStats returns statistics across all histories
func (r *FileHistoryRepository) GetHistoryStats(ctx context.Context, filter repositories.HistoryFilter) (*repositories.HistoryStats, error) {
	histories, err := r.GetAllHistory(ctx, filter)
	if err != nil {
		return nil, errors.Wrap(err, "FileHistoryRepository.GetHistoryStats", "get_all_history")
	}

	stats := &repositories.HistoryStats{
		TotalIssuesWithHistory: len(histories),
		EntriesByType:          make(map[entities.ChangeType]int),
		EntriesByAuthor:        make(map[string]int),
		ActivityByDay:          make(map[string]int),
	}

	var totalEntries int
	var mostActiveIssueID entities.IssueID
	var maxEntriesPerIssue int

	for _, history := range histories {
		entryCount := len(history.Entries)
		totalEntries += entryCount

		// Track most active issue
		if entryCount > maxEntriesPerIssue {
			maxEntriesPerIssue = entryCount
			mostActiveIssueID = history.IssueID
		}

		// Process each entry
		for _, entry := range history.Entries {
			// Count by type
			stats.EntriesByType[entry.Type]++

			// Count by author
			stats.EntriesByAuthor[entry.Author]++

			// Count by day
			day := entry.Timestamp.Format("2006-01-02")
			stats.ActivityByDay[day]++
		}
	}

	stats.TotalHistoryEntries = totalEntries

	if len(histories) > 0 {
		stats.AverageChangesPerIssue = float64(totalEntries) / float64(len(histories))
	}

	if maxEntriesPerIssue > 0 {
		stats.MostActiveIssue = &mostActiveIssueID
	}

	// Find most active author
	maxAuthorChanges := 0
	for author, count := range stats.EntriesByAuthor {
		if count > maxAuthorChanges {
			maxAuthorChanges = count
			stats.MostActiveAuthor = author
		}
	}

	return stats, nil
}

// GetHistoryByDateRange returns all changes within a date range
func (r *FileHistoryRepository) GetHistoryByDateRange(ctx context.Context, since, until time.Time) ([]*entities.HistoryEntry, error) {
	filter := repositories.HistoryFilter{
		Since: &since,
		Until: &until,
	}

	entryList, err := r.ListEntries(ctx, filter)
	if err != nil {
		return nil, errors.Wrap(err, "FileHistoryRepository.GetHistoryByDateRange", "list_entries")
	}

	var entries []*entities.HistoryEntry
	for i := range entryList.Entries {
		entries = append(entries, &entryList.Entries[i])
	}

	return entries, nil
}

// applyFilters applies filter criteria to a list of history entries
func (r *FileHistoryRepository) applyFilters(entries []entities.HistoryEntry, filter repositories.HistoryFilter) []entities.HistoryEntry {
	var filtered []entities.HistoryEntry

	for _, entry := range entries {
		// Apply filters
		if filter.Author != nil && entry.Author != *filter.Author {
			continue
		}

		if filter.ChangeType != nil && entry.Type != *filter.ChangeType {
			continue
		}

		if filter.Since != nil && entry.Timestamp.Before(*filter.Since) {
			continue
		}

		if filter.Until != nil && entry.Timestamp.After(*filter.Until) {
			continue
		}

		if filter.Field != nil {
			// Check if any field change matches
			fieldMatches := false
			for _, change := range entry.Changes {
				if change.Field == *filter.Field {
					fieldMatches = true
					break
				}
			}
			if !fieldMatches {
				continue
			}
		}

		filtered = append(filtered, entry)
	}

	return filtered
}
