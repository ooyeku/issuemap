package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

func TestTimeEntry_NewTimeEntry(t *testing.T) {
	issueID := entities.IssueID("ISSUE-001")
	duration := 2 * time.Hour
	description := "Working on authentication"
	author := "john.doe"

	entry := entities.NewTimeEntry(issueID, entities.TimeEntryTypeManual, duration, description, author)

	assert.Equal(t, issueID, entry.IssueID)
	assert.Equal(t, entities.TimeEntryTypeManual, entry.Type)
	assert.Equal(t, duration, entry.Duration)
	assert.Equal(t, description, entry.Description)
	assert.Equal(t, author, entry.Author)
	assert.NotEmpty(t, entry.ID)
	assert.False(t, entry.CreatedAt.IsZero())
	assert.False(t, entry.UpdatedAt.IsZero())
}

func TestTimeEntry_GetDurationHours(t *testing.T) {
	entry := entities.NewTimeEntry(
		"ISSUE-001",
		entities.TimeEntryTypeManual,
		90*time.Minute, // 1.5 hours
		"Test work",
		"author",
	)

	assert.Equal(t, 1.5, entry.GetDurationHours())
}

func TestTimeEntry_Validate(t *testing.T) {
	testCases := []struct {
		name        string
		entry       *entities.TimeEntry
		expectError bool
	}{
		{
			name: "valid entry",
			entry: &entities.TimeEntry{
				ID:       "test-id",
				IssueID:  "ISSUE-001",
				Type:     entities.TimeEntryTypeManual,
				Duration: time.Hour,
				Author:   "author",
			},
			expectError: false,
		},
		{
			name: "empty ID",
			entry: &entities.TimeEntry{
				ID:       "",
				IssueID:  "ISSUE-001",
				Type:     entities.TimeEntryTypeManual,
				Duration: time.Hour,
				Author:   "author",
			},
			expectError: true,
		},
		{
			name: "empty issue ID",
			entry: &entities.TimeEntry{
				ID:       "test-id",
				IssueID:  "",
				Type:     entities.TimeEntryTypeManual,
				Duration: time.Hour,
				Author:   "author",
			},
			expectError: true,
		},
		{
			name: "zero duration",
			entry: &entities.TimeEntry{
				ID:       "test-id",
				IssueID:  "ISSUE-001",
				Type:     entities.TimeEntryTypeManual,
				Duration: 0,
				Author:   "author",
			},
			expectError: true,
		},
		{
			name: "negative duration",
			entry: &entities.TimeEntry{
				ID:       "test-id",
				IssueID:  "ISSUE-001",
				Type:     entities.TimeEntryTypeManual,
				Duration: -time.Hour,
				Author:   "author",
			},
			expectError: true,
		},
		{
			name: "empty author",
			entry: &entities.TimeEntry{
				ID:       "test-id",
				IssueID:  "ISSUE-001",
				Type:     entities.TimeEntryTypeManual,
				Duration: time.Hour,
				Author:   "",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestActiveTimer_NewActiveTimer(t *testing.T) {
	issueID := entities.IssueID("ISSUE-001")
	description := "Working on bug fix"
	author := "jane.smith"

	timer := entities.NewActiveTimer(issueID, description, author)

	assert.Equal(t, issueID, timer.IssueID)
	assert.Equal(t, description, timer.Description)
	assert.Equal(t, author, timer.Author)
	assert.False(t, timer.StartTime.IsZero())
	assert.False(t, timer.CreatedAt.IsZero())
}

func TestActiveTimer_GetElapsedTime(t *testing.T) {
	timer := entities.NewActiveTimer("ISSUE-001", "Test", "author")
	
	// Sleep for a short time to get measurable elapsed time
	time.Sleep(10 * time.Millisecond)
	
	elapsed := timer.GetElapsedTime()
	assert.True(t, elapsed > 0)
	assert.True(t, elapsed < time.Second) // Should be very short
}

func TestActiveTimer_ToTimeEntry(t *testing.T) {
	timer := entities.NewActiveTimer("ISSUE-001", "Test work", "author")
	endTime := timer.StartTime.Add(2 * time.Hour)

	entry := timer.ToTimeEntry(endTime)

	assert.Equal(t, timer.IssueID, entry.IssueID)
	assert.Equal(t, timer.Description, entry.Description)
	assert.Equal(t, timer.Author, entry.Author)
	assert.Equal(t, entities.TimeEntryTypeTimer, entry.Type)
	assert.Equal(t, timer.StartTime, entry.StartTime)
	assert.Equal(t, endTime, *entry.EndTime)
	assert.Equal(t, 2*time.Hour, entry.Duration)
	assert.NotEmpty(t, entry.ID)
}

func TestActiveTimer_Validate(t *testing.T) {
	testCases := []struct {
		name        string
		timer       *entities.ActiveTimer
		expectError bool
	}{
		{
			name: "valid timer",
			timer: &entities.ActiveTimer{
				IssueID:   "ISSUE-001",
				Author:    "author",
				StartTime: time.Now(),
			},
			expectError: false,
		},
		{
			name: "empty issue ID",
			timer: &entities.ActiveTimer{
				IssueID:   "",
				Author:    "author",
				StartTime: time.Now(),
			},
			expectError: true,
		},
		{
			name: "empty author",
			timer: &entities.ActiveTimer{
				IssueID:   "ISSUE-001",
				Author:    "",
				StartTime: time.Now(),
			},
			expectError: true,
		},
		{
			name: "zero start time",
			timer: &entities.ActiveTimer{
				IssueID:   "ISSUE-001",
				Author:    "author",
				StartTime: time.Time{},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.timer.Validate()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIssue_TimeTrackingMethods(t *testing.T) {
	issue := entities.NewIssue("ISSUE-001", "Test Issue", "Description", entities.IssueTypeTask)

	// Test initial state
	assert.Equal(t, 0.0, issue.GetEstimatedHours())
	assert.Equal(t, 0.0, issue.GetActualHours())
	assert.Equal(t, 0.0, issue.GetRemainingHours())
	assert.False(t, issue.IsOverEstimate())

	// Test setting estimate
	issue.SetEstimate(8.0)
	assert.Equal(t, 8.0, issue.GetEstimatedHours())
	assert.Equal(t, 8.0, issue.GetRemainingHours())
	assert.False(t, issue.IsOverEstimate())

	// Test adding time
	issue.AddTimeEntry(2.5)
	assert.Equal(t, 2.5, issue.GetActualHours())
	assert.Equal(t, 5.5, issue.GetRemainingHours())
	assert.False(t, issue.IsOverEstimate())

	// Test adding more time
	issue.AddTimeEntry(1.5)
	assert.Equal(t, 4.0, issue.GetActualHours())
	assert.Equal(t, 4.0, issue.GetRemainingHours())
	assert.False(t, issue.IsOverEstimate())

	// Test over-estimate
	issue.AddTimeEntry(5.0)
	assert.Equal(t, 9.0, issue.GetActualHours())
	assert.Equal(t, 0.0, issue.GetRemainingHours()) // Should not go negative
	assert.True(t, issue.IsOverEstimate())
}

func TestIssue_TimeTrackingWithoutEstimate(t *testing.T) {
	issue := entities.NewIssue("ISSUE-001", "Test Issue", "Description", entities.IssueTypeTask)

	// Add time without setting estimate
	issue.AddTimeEntry(3.0)
	assert.Equal(t, 0.0, issue.GetEstimatedHours())
	assert.Equal(t, 3.0, issue.GetActualHours())
	assert.Equal(t, 0.0, issue.GetRemainingHours())
	assert.False(t, issue.IsOverEstimate()) // No estimate, so not over
}

func TestTimeEntry_Stop(t *testing.T) {
	entry := &entities.TimeEntry{
		ID:        "test-id",
		IssueID:   "ISSUE-001",
		Type:      entities.TimeEntryTypeTimer,
		StartTime: time.Now().Add(-time.Hour),
		Author:    "author",
	}

	err := entry.Stop()
	require.NoError(t, err)
	
	assert.NotNil(t, entry.EndTime)
	assert.True(t, entry.Duration > 0)
	assert.True(t, entry.Duration >= 50*time.Minute) // Should be close to 1 hour
	assert.False(t, entry.UpdatedAt.IsZero())
}

func TestTimeEntry_StopErrors(t *testing.T) {
	// Test stopping non-timer entry
	entry := &entities.TimeEntry{
		Type: entities.TimeEntryTypeManual,
	}
	err := entry.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot stop non-timer entry")

	// Test stopping already stopped timer
	entry = &entities.TimeEntry{
		Type:    entities.TimeEntryTypeTimer,
		EndTime: &time.Time{},
	}
	err = entry.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timer is already stopped")
}

func TestTimeEntry_IsRunning(t *testing.T) {
	// Timer without end time
	entry := &entities.TimeEntry{
		Type:    entities.TimeEntryTypeTimer,
		EndTime: nil,
	}
	assert.True(t, entry.IsRunning())

	// Timer with end time
	now := time.Now()
	entry.EndTime = &now
	assert.False(t, entry.IsRunning())

	// Manual entry
	entry = &entities.TimeEntry{
		Type:    entities.TimeEntryTypeManual,
		EndTime: nil,
	}
	assert.False(t, entry.IsRunning())
}