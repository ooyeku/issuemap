package entities

import (
	"crypto/rand"
	"fmt"
	"time"
)

// TimeEntryType represents the type of time entry
type TimeEntryType string

const (
	TimeEntryTypeManual TimeEntryType = "manual"
	TimeEntryTypeTimer  TimeEntryType = "timer"
	TimeEntryTypeCommit TimeEntryType = "commit"
)

// generateUniqueID creates a unique ID for time entries
func generateUniqueID(issueID IssueID) string {
	now := time.Now()
	// Use random bytes to ensure uniqueness even within the same nanosecond
	randBytes := make([]byte, 4)
	_, _ = rand.Read(randBytes)
	return fmt.Sprintf("%s-%d-%d-%x", issueID, now.Unix(), now.Nanosecond(), randBytes)
}

// TimeEntry represents a single time tracking entry for an issue
type TimeEntry struct {
	ID          string        `yaml:"id" json:"id"`
	IssueID     IssueID       `yaml:"issue_id" json:"issue_id"`
	Type        TimeEntryType `yaml:"type" json:"type"`
	Duration    time.Duration `yaml:"duration" json:"duration"`
	Description string        `yaml:"description,omitempty" json:"description,omitempty"`
	Author      string        `yaml:"author" json:"author"`
	StartTime   time.Time     `yaml:"start_time" json:"start_time"`
	EndTime     *time.Time    `yaml:"end_time,omitempty" json:"end_time,omitempty"`
	CreatedAt   time.Time     `yaml:"created_at" json:"created_at"`
	UpdatedAt   time.Time     `yaml:"updated_at" json:"updated_at"`
}

// ActiveTimer represents an active timer session
type ActiveTimer struct {
	IssueID     IssueID   `yaml:"issue_id" json:"issue_id"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	Author      string    `yaml:"author" json:"author"`
	StartTime   time.Time `yaml:"start_time" json:"start_time"`
	CreatedAt   time.Time `yaml:"created_at" json:"created_at"`
}

// NewTimeEntry creates a new time entry
func NewTimeEntry(issueID IssueID, entryType TimeEntryType, duration time.Duration, description, author string) *TimeEntry {
	now := time.Now()
	return &TimeEntry{
		ID:          generateUniqueID(issueID),
		IssueID:     issueID,
		Type:        entryType,
		Duration:    duration,
		Description: description,
		Author:      author,
		StartTime:   now.Add(-duration),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewTimerEntry creates a new timer-based time entry
func NewTimerEntry(issueID IssueID, description, author string, startTime, endTime time.Time) *TimeEntry {
	duration := endTime.Sub(startTime)
	return &TimeEntry{
		ID:          generateUniqueID(issueID),
		IssueID:     issueID,
		Type:        TimeEntryTypeTimer,
		Duration:    duration,
		Description: description,
		Author:      author,
		StartTime:   startTime,
		EndTime:     &endTime,
		CreatedAt:   endTime,
		UpdatedAt:   endTime,
	}
}

// NewActiveTimer creates a new active timer
func NewActiveTimer(issueID IssueID, description, author string) *ActiveTimer {
	now := time.Now()
	return &ActiveTimer{
		IssueID:     issueID,
		Description: description,
		Author:      author,
		StartTime:   now,
		CreatedAt:   now,
	}
}

// GetDuration returns the duration of the time entry
func (te *TimeEntry) GetDuration() time.Duration {
	return te.Duration
}

// GetDurationHours returns the duration in hours
func (te *TimeEntry) GetDurationHours() float64 {
	return te.Duration.Hours()
}

// IsRunning returns true if this is an active timer (no end time)
func (te *TimeEntry) IsRunning() bool {
	return te.EndTime == nil && te.Type == TimeEntryTypeTimer
}

// Stop stops an active timer entry
func (te *TimeEntry) Stop() error {
	if te.Type != TimeEntryTypeTimer {
		return fmt.Errorf("cannot stop non-timer entry")
	}
	if te.EndTime != nil {
		return fmt.Errorf("timer is already stopped")
	}

	now := time.Now()
	te.EndTime = &now
	te.Duration = now.Sub(te.StartTime)
	te.UpdatedAt = now
	return nil
}

// GetElapsedTime returns the elapsed time for an active timer
func (at *ActiveTimer) GetElapsedTime() time.Duration {
	return time.Since(at.StartTime)
}

// ToTimeEntry converts an active timer to a time entry when stopped
func (at *ActiveTimer) ToTimeEntry(endTime time.Time) *TimeEntry {
	duration := endTime.Sub(at.StartTime)
	return &TimeEntry{
		ID:          generateUniqueID(at.IssueID),
		IssueID:     at.IssueID,
		Type:        TimeEntryTypeTimer,
		Duration:    duration,
		Description: at.Description,
		Author:      at.Author,
		StartTime:   at.StartTime,
		EndTime:     &endTime,
		CreatedAt:   endTime,
		UpdatedAt:   endTime,
	}
}

// Validate validates the time entry data
func (te *TimeEntry) Validate() error {
	if te.ID == "" {
		return fmt.Errorf("time entry ID cannot be empty")
	}
	if te.IssueID == "" {
		return fmt.Errorf("issue ID cannot be empty")
	}
	if te.Duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	if te.Author == "" {
		return fmt.Errorf("author cannot be empty")
	}
	if te.Type == "" {
		return fmt.Errorf("time entry type cannot be empty")
	}
	return nil
}

// Validate validates the active timer data
func (at *ActiveTimer) Validate() error {
	if at.IssueID == "" {
		return fmt.Errorf("issue ID cannot be empty")
	}
	if at.Author == "" {
		return fmt.Errorf("author cannot be empty")
	}
	if at.StartTime.IsZero() {
		return fmt.Errorf("start time cannot be zero")
	}
	return nil
}
