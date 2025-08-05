package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

const (
	timeEntriesDir = "time_entries"
	activeTimerFile = "active_timer.yaml"
)

// FileTimeEntryRepository implements time entry persistence using files
type FileTimeEntryRepository struct {
	basePath string
}

// FileActiveTimerRepository implements active timer persistence using files
type FileActiveTimerRepository struct {
	basePath string
}

// NewFileTimeEntryRepository creates a new file-based time entry repository
func NewFileTimeEntryRepository(basePath string) *FileTimeEntryRepository {
	return &FileTimeEntryRepository{
		basePath: basePath,
	}
}

// NewFileActiveTimerRepository creates a new file-based active timer repository
func NewFileActiveTimerRepository(basePath string) *FileActiveTimerRepository {
	return &FileActiveTimerRepository{
		basePath: basePath,
	}
}

// Create saves a new time entry
func (r *FileTimeEntryRepository) Create(ctx context.Context, entry *entities.TimeEntry) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	dir := filepath.Join(r.basePath, timeEntriesDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	filename := fmt.Sprintf("%s.yaml", entry.ID)
	path := filepath.Join(dir, filename)

	data, err := yaml.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal time entry: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write time entry: %w", err)
	}

	return nil
}

// GetByID retrieves a time entry by its ID
func (r *FileTimeEntryRepository) GetByID(ctx context.Context, id string) (*entities.TimeEntry, error) {
	path := filepath.Join(r.basePath, timeEntriesDir, fmt.Sprintf("%s.yaml", id))
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("time entry not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read time entry: %w", err)
	}

	var entry entities.TimeEntry
	if err := yaml.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal time entry: %w", err)
	}

	return &entry, nil
}

// Update updates an existing time entry
func (r *FileTimeEntryRepository) Update(ctx context.Context, entry *entities.TimeEntry) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	path := filepath.Join(r.basePath, timeEntriesDir, fmt.Sprintf("%s.yaml", entry.ID))
	
	// Check if entry exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("time entry not found: %s", entry.ID)
	}

	entry.UpdatedAt = time.Now()

	data, err := yaml.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal time entry: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write time entry: %w", err)
	}

	return nil
}

// Delete removes a time entry by its ID
func (r *FileTimeEntryRepository) Delete(ctx context.Context, id string) error {
	path := filepath.Join(r.basePath, timeEntriesDir, fmt.Sprintf("%s.yaml", id))
	
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("time entry not found: %s", id)
		}
		return fmt.Errorf("failed to delete time entry: %w", err)
	}

	return nil
}

// List retrieves time entries with optional filtering
func (r *FileTimeEntryRepository) List(ctx context.Context, filter repositories.TimeEntryFilter) ([]*entities.TimeEntry, error) {
	dir := filepath.Join(r.basePath, timeEntriesDir)
	
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*entities.TimeEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read time entries directory: %w", err)
	}

	var timeEntries []*entities.TimeEntry
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip files we can't read
		}

		var timeEntry entities.TimeEntry
		if err := yaml.Unmarshal(data, &timeEntry); err != nil {
			continue // Skip files we can't parse
		}

		// Apply filters
		if !r.matchesFilter(&timeEntry, filter) {
			continue
		}

		timeEntries = append(timeEntries, &timeEntry)
	}

	// Sort by start time (newest first)
	sort.Slice(timeEntries, func(i, j int) bool {
		return timeEntries[i].StartTime.After(timeEntries[j].StartTime)
	})

	// Apply limit and offset
	if filter.Offset > 0 && filter.Offset < len(timeEntries) {
		timeEntries = timeEntries[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(timeEntries) {
		timeEntries = timeEntries[:filter.Limit]
	}

	return timeEntries, nil
}

// GetByIssueID retrieves all time entries for a specific issue
func (r *FileTimeEntryRepository) GetByIssueID(ctx context.Context, issueID entities.IssueID) ([]*entities.TimeEntry, error) {
	filter := repositories.TimeEntryFilter{
		IssueID: &issueID,
	}
	return r.List(ctx, filter)
}

// GetByAuthor retrieves all time entries for a specific author
func (r *FileTimeEntryRepository) GetByAuthor(ctx context.Context, author string, filter repositories.TimeEntryFilter) ([]*entities.TimeEntry, error) {
	filter.Author = &author
	return r.List(ctx, filter)
}

// GetStats returns time tracking statistics
func (r *FileTimeEntryRepository) GetStats(ctx context.Context, filter repositories.TimeEntryFilter) (*repositories.TimeEntryStats, error) {
	entries, err := r.List(ctx, repositories.TimeEntryFilter{}) // Get all entries first
	if err != nil {
		return nil, err
	}

	// Filter entries
	var filteredEntries []*entities.TimeEntry
	for _, entry := range entries {
		if r.matchesFilter(entry, filter) {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	if len(filteredEntries) == 0 {
		return &repositories.TimeEntryStats{}, nil
	}

	stats := &repositories.TimeEntryStats{
		TotalEntries:  len(filteredEntries),
		EntriesByType: make(map[entities.TimeEntryType]int),
		TimeByAuthor:  make(map[string]time.Duration),
		TimeByIssue:   make(map[entities.IssueID]time.Duration),
		EntriesByDay:  make(map[string]int),
		TimeByDay:     make(map[string]time.Duration),
	}

	authorSet := make(map[string]bool)
	var totalDuration time.Duration

	for _, entry := range filteredEntries {
		duration := entry.GetDuration()
		totalDuration += duration

		// Track unique authors
		authorSet[entry.Author] = true

		// Count by type
		stats.EntriesByType[entry.Type]++

		// Time by author
		stats.TimeByAuthor[entry.Author] += duration

		// Time by issue
		stats.TimeByIssue[entry.IssueID] += duration

		// Entries and time by day
		day := entry.StartTime.Format("2006-01-02")
		stats.EntriesByDay[day]++
		stats.TimeByDay[day] += duration
	}

	stats.TotalTime = totalDuration
	stats.UniqueAuthors = len(authorSet)
	if stats.TotalEntries > 0 {
		stats.AverageTime = totalDuration / time.Duration(stats.TotalEntries)
	}

	return stats, nil
}

// GetTotalTimeByIssue returns the total time logged for each issue
func (r *FileTimeEntryRepository) GetTotalTimeByIssue(ctx context.Context, filter repositories.TimeEntryFilter) (map[entities.IssueID]time.Duration, error) {
	entries, err := r.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	timeByIssue := make(map[entities.IssueID]time.Duration)
	for _, entry := range entries {
		timeByIssue[entry.IssueID] += entry.GetDuration()
	}

	return timeByIssue, nil
}

// matchesFilter checks if a time entry matches the given filter
func (r *FileTimeEntryRepository) matchesFilter(entry *entities.TimeEntry, filter repositories.TimeEntryFilter) bool {
	if filter.IssueID != nil && entry.IssueID != *filter.IssueID {
		return false
	}
	if filter.Author != nil && entry.Author != *filter.Author {
		return false
	}
	if filter.Type != nil && entry.Type != *filter.Type {
		return false
	}
	if filter.DateFrom != nil && entry.StartTime.Before(*filter.DateFrom) {
		return false
	}
	if filter.DateTo != nil && entry.StartTime.After(*filter.DateTo) {
		return false
	}
	if filter.MinDuration != nil && entry.Duration < *filter.MinDuration {
		return false
	}
	if filter.MaxDuration != nil && entry.Duration > *filter.MaxDuration {
		return false
	}
	return true
}

// Active Timer Repository Implementation

// Create saves a new active timer
func (r *FileActiveTimerRepository) Create(ctx context.Context, timer *entities.ActiveTimer) error {
	if err := timer.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Only one active timer per author
	existing, _ := r.GetByAuthor(ctx, timer.Author)
	if existing != nil {
		return fmt.Errorf("active timer already exists for author: %s", timer.Author)
	}

	if err := os.MkdirAll(r.basePath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	filename := fmt.Sprintf("timer_%s.yaml", timer.Author)
	path := filepath.Join(r.basePath, filename)

	data, err := yaml.Marshal(timer)
	if err != nil {
		return fmt.Errorf("failed to marshal active timer: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write active timer: %w", err)
	}

	return nil
}

// GetByAuthor retrieves the active timer for a specific author
func (r *FileActiveTimerRepository) GetByAuthor(ctx context.Context, author string) (*entities.ActiveTimer, error) {
	filename := fmt.Sprintf("timer_%s.yaml", author)
	path := filepath.Join(r.basePath, filename)
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no active timer for author: %s", author)
		}
		return nil, fmt.Errorf("failed to read active timer: %w", err)
	}

	var timer entities.ActiveTimer
	if err := yaml.Unmarshal(data, &timer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal active timer: %w", err)
	}

	return &timer, nil
}

// GetByIssueAndAuthor retrieves the active timer for a specific issue and author
func (r *FileActiveTimerRepository) GetByIssueAndAuthor(ctx context.Context, issueID entities.IssueID, author string) (*entities.ActiveTimer, error) {
	timer, err := r.GetByAuthor(ctx, author)
	if err != nil {
		return nil, err
	}

	if timer.IssueID != issueID {
		return nil, fmt.Errorf("no active timer for issue %s and author %s", issueID, author)
	}

	return timer, nil
}

// Delete removes an active timer
func (r *FileActiveTimerRepository) Delete(ctx context.Context, issueID entities.IssueID, author string) error {
	filename := fmt.Sprintf("timer_%s.yaml", author)
	path := filepath.Join(r.basePath, filename)
	
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no active timer for author: %s", author)
		}
		return fmt.Errorf("failed to delete active timer: %w", err)
	}

	return nil
}

// List retrieves all active timers
func (r *FileActiveTimerRepository) List(ctx context.Context) ([]*entities.ActiveTimer, error) {
	entries, err := os.ReadDir(r.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*entities.ActiveTimer{}, nil
		}
		return nil, fmt.Errorf("failed to read active timers directory: %w", err)
	}

	var timers []*entities.ActiveTimer
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "timer_") || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(r.basePath, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip files we can't read
		}

		var timer entities.ActiveTimer
		if err := yaml.Unmarshal(data, &timer); err != nil {
			continue // Skip files we can't parse
		}

		timers = append(timers, &timer)
	}

	// Sort by start time (newest first)
	sort.Slice(timers, func(i, j int) bool {
		return timers[i].StartTime.After(timers[j].StartTime)
	})

	return timers, nil
}