package entities

import (
	"fmt"
	"time"
)

// ChangeType represents the type of change made to an issue
type ChangeType string

const (
	ChangeTypeCreated      ChangeType = "created"
	ChangeTypeUpdated      ChangeType = "updated"
	ChangeTypeClosed       ChangeType = "closed"
	ChangeTypeReopened     ChangeType = "reopened"
	ChangeTypeAssigned     ChangeType = "assigned"
	ChangeTypeUnassigned   ChangeType = "unassigned"
	ChangeTypeLabeled      ChangeType = "labeled"
	ChangeTypeUnlabeled    ChangeType = "unlabeled"
	ChangeTypeCommented    ChangeType = "commented"
	ChangeTypeMilestoned   ChangeType = "milestoned"
	ChangeTypeUnmilestoned ChangeType = "unmilestoned"
	ChangeTypeLinked       ChangeType = "linked"
	ChangeTypeUnlinked     ChangeType = "unlinked"
)

// FieldChange represents a change to a specific field
type FieldChange struct {
	Field    string      `json:"field" yaml:"field"`
	OldValue interface{} `json:"old_value" yaml:"old_value"`
	NewValue interface{} `json:"new_value" yaml:"new_value"`
}

// HistoryEntry represents a single change event in an issue's history
type HistoryEntry struct {
	ID        string                 `json:"id" yaml:"id"`                                 // Unique identifier for this history entry
	IssueID   IssueID                `json:"issue_id" yaml:"issue_id"`                     // The issue this change belongs to
	Version   int                    `json:"version" yaml:"version"`                       // Version number of the issue after this change
	Type      ChangeType             `json:"type" yaml:"type"`                             // Type of change
	Author    string                 `json:"author" yaml:"author"`                         // Who made the change
	Timestamp time.Time              `json:"timestamp" yaml:"timestamp"`                   // When the change was made
	Message   string                 `json:"message" yaml:"message"`                       // Human-readable description of the change
	Changes   []FieldChange          `json:"changes" yaml:"changes"`                       // Detailed field changes
	Metadata  map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"` // Additional context
}

// IssueHistory represents the complete version history of an issue
type IssueHistory struct {
	IssueID        IssueID        `json:"issue_id" yaml:"issue_id"`
	CurrentVersion int            `json:"current_version" yaml:"current_version"`
	CreatedAt      time.Time      `json:"created_at" yaml:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at" yaml:"updated_at"`
	Entries        []HistoryEntry `json:"entries" yaml:"entries"`
}

// NewHistoryEntry creates a new history entry
func NewHistoryEntry(issueID IssueID, changeType ChangeType, author, message string) *HistoryEntry {
	return &HistoryEntry{
		ID:        generateHistoryID(),
		IssueID:   issueID,
		Type:      changeType,
		Author:    author,
		Timestamp: time.Now(),
		Message:   message,
		Changes:   []FieldChange{},
		Metadata:  make(map[string]interface{}),
	}
}

// AddFieldChange adds a field change to the history entry
func (h *HistoryEntry) AddFieldChange(field string, oldValue, newValue interface{}) {
	change := FieldChange{
		Field:    field,
		OldValue: oldValue,
		NewValue: newValue,
	}
	h.Changes = append(h.Changes, change)
}

// SetMetadata sets metadata for the history entry
func (h *HistoryEntry) SetMetadata(key string, value interface{}) {
	h.Metadata[key] = value
}

// NewIssueHistory creates a new issue history
func NewIssueHistory(issueID IssueID) *IssueHistory {
	now := time.Now()
	return &IssueHistory{
		IssueID:        issueID,
		CurrentVersion: 0,
		CreatedAt:      now,
		UpdatedAt:      now,
		Entries:        []HistoryEntry{},
	}
}

// AddEntry adds a new history entry and increments the version
func (h *IssueHistory) AddEntry(entry *HistoryEntry) {
	h.CurrentVersion++
	entry.Version = h.CurrentVersion
	h.Entries = append(h.Entries, *entry)
	h.UpdatedAt = time.Now()
}

// GetLatestEntry returns the most recent history entry
func (h *IssueHistory) GetLatestEntry() *HistoryEntry {
	if len(h.Entries) == 0 {
		return nil
	}
	return &h.Entries[len(h.Entries)-1]
}

// GetEntryByVersion returns the history entry for a specific version
func (h *IssueHistory) GetEntryByVersion(version int) *HistoryEntry {
	for _, entry := range h.Entries {
		if entry.Version == version {
			return &entry
		}
	}
	return nil
}

// GetEntriesByType returns all history entries of a specific type
func (h *IssueHistory) GetEntriesByType(changeType ChangeType) []HistoryEntry {
	var entries []HistoryEntry
	for _, entry := range h.Entries {
		if entry.Type == changeType {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetEntriesByAuthor returns all history entries by a specific author
func (h *IssueHistory) GetEntriesByAuthor(author string) []HistoryEntry {
	var entries []HistoryEntry
	for _, entry := range h.Entries {
		if entry.Author == author {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetEntriesSince returns all history entries since a specific time
func (h *IssueHistory) GetEntriesSince(since time.Time) []HistoryEntry {
	var entries []HistoryEntry
	for _, entry := range h.Entries {
		if entry.Timestamp.After(since) {
			entries = append(entries, entry)
		}
	}
	return entries
}

// generateHistoryID generates a unique ID for a history entry
func generateHistoryID() string {
	// Simple time-based ID - in production, could use UUID
	return fmt.Sprintf("hist_%d", time.Now().UnixNano())
}

// HistoryStats provides statistics about the issue history
type HistoryStats struct {
	TotalChanges         int                `json:"total_changes"`
	ChangesByType        map[ChangeType]int `json:"changes_by_type"`
	ChangesByAuthor      map[string]int     `json:"changes_by_author"`
	FirstChange          *time.Time         `json:"first_change,omitempty"`
	LastChange           *time.Time         `json:"last_change,omitempty"`
	MostActiveAuthor     string             `json:"most_active_author"`
	AverageChangesPerDay float64            `json:"average_changes_per_day"`
}

// GetStats returns statistics about the issue history
func (h *IssueHistory) GetStats() *HistoryStats {
	stats := &HistoryStats{
		TotalChanges:    len(h.Entries),
		ChangesByType:   make(map[ChangeType]int),
		ChangesByAuthor: make(map[string]int),
	}

	if len(h.Entries) == 0 {
		return stats
	}

	// Calculate statistics
	var firstTime, lastTime time.Time
	authorCounts := make(map[string]int)

	for i, entry := range h.Entries {
		// Track by type
		stats.ChangesByType[entry.Type]++

		// Track by author
		authorCounts[entry.Author]++

		// Track time range
		if i == 0 {
			firstTime = entry.Timestamp
			lastTime = entry.Timestamp
		} else {
			if entry.Timestamp.Before(firstTime) {
				firstTime = entry.Timestamp
			}
			if entry.Timestamp.After(lastTime) {
				lastTime = entry.Timestamp
			}
		}
	}

	stats.FirstChange = &firstTime
	stats.LastChange = &lastTime
	stats.ChangesByAuthor = authorCounts

	// Find most active author
	maxChanges := 0
	for author, count := range authorCounts {
		if count > maxChanges {
			maxChanges = count
			stats.MostActiveAuthor = author
		}
	}

	// Calculate average changes per day
	if !firstTime.IsZero() && !lastTime.IsZero() {
		duration := lastTime.Sub(firstTime)
		if duration.Hours() > 0 {
			days := duration.Hours() / 24
			if days < 1 {
				days = 1 // Minimum 1 day
			}
			stats.AverageChangesPerDay = float64(len(h.Entries)) / days
		}
	}

	return stats
}
