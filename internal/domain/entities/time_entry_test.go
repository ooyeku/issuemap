package entities

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeEntryType(t *testing.T) {
	// Test that TimeEntryType constants are defined correctly
	assert.Equal(t, TimeEntryType("manual"), TimeEntryTypeManual)
	assert.Equal(t, TimeEntryType("timer"), TimeEntryTypeTimer)
	assert.Equal(t, TimeEntryType("commit"), TimeEntryTypeCommit)
}

func TestGenerateUniqueID(t *testing.T) {
	issueID := IssueID("TEST-001")

	// Generate multiple IDs
	id1 := generateUniqueID(issueID)
	id2 := generateUniqueID(issueID)
	id3 := generateUniqueID(issueID)

	// All IDs should be different
	assert.NotEqual(t, id1, id2)
	assert.NotEqual(t, id2, id3)
	assert.NotEqual(t, id1, id3)

	// All IDs should start with the issue ID
	assert.True(t, strings.HasPrefix(id1, string(issueID)+"-"))
	assert.True(t, strings.HasPrefix(id2, string(issueID)+"-"))
	assert.True(t, strings.HasPrefix(id3, string(issueID)+"-"))
}

func TestTimeEntryStructure(t *testing.T) {
	// Test TimeEntry structure
	now := time.Now()
	endTime := now.Add(time.Hour)

	entry := TimeEntry{
		ID:          "test-id",
		IssueID:     IssueID("TEST-001"),
		Type:        TimeEntryTypeManual,
		Duration:    time.Hour,
		Description: "Test work",
		Author:      "testuser",
		StartTime:   now,
		EndTime:     &endTime,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	assert.Equal(t, "test-id", entry.ID)
	assert.Equal(t, IssueID("TEST-001"), entry.IssueID)
	assert.Equal(t, TimeEntryTypeManual, entry.Type)
	assert.Equal(t, time.Hour, entry.Duration)
	assert.Equal(t, "Test work", entry.Description)
	assert.Equal(t, "testuser", entry.Author)
	assert.Equal(t, now, entry.StartTime)
	assert.NotNil(t, entry.EndTime)
	assert.Equal(t, endTime, *entry.EndTime)
	assert.Equal(t, now, entry.CreatedAt)
	assert.Equal(t, now, entry.UpdatedAt)
}

func TestActiveTimerStructure(t *testing.T) {
	// Test ActiveTimer structure
	now := time.Now()

	timer := ActiveTimer{
		IssueID:     IssueID("TEST-001"),
		Description: "Working on feature",
		Author:      "testuser",
		StartTime:   now,
		CreatedAt:   now,
	}

	assert.Equal(t, IssueID("TEST-001"), timer.IssueID)
	assert.Equal(t, "Working on feature", timer.Description)
	assert.Equal(t, "testuser", timer.Author)
	assert.Equal(t, now, timer.StartTime)
	assert.Equal(t, now, timer.CreatedAt)
}

func TestTimeEntryWithOptionalFields(t *testing.T) {
	// Test TimeEntry with optional fields (EndTime can be nil)
	now := time.Now()

	entry := TimeEntry{
		ID:        "test-id",
		IssueID:   IssueID("TEST-001"),
		Type:      TimeEntryTypeTimer,
		Duration:  time.Minute * 30,
		Author:    "testuser",
		StartTime: now,
		EndTime:   nil, // Optional field
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Nil(t, entry.EndTime)
	assert.Empty(t, entry.Description) // Optional field
}

func TestTimeEntryTypes(t *testing.T) {
	// Test all time entry types
	types := []TimeEntryType{
		TimeEntryTypeManual,
		TimeEntryTypeTimer,
		TimeEntryTypeCommit,
	}

	for _, entryType := range types {
		entry := TimeEntry{
			Type: entryType,
		}
		assert.Equal(t, entryType, entry.Type)
	}
}

func TestGenerateUniqueIDDifferentIssues(t *testing.T) {
	// Test that different issue IDs produce different ID patterns
	issueID1 := IssueID("TEST-001")
	issueID2 := IssueID("PROJ-042")

	id1 := generateUniqueID(issueID1)
	id2 := generateUniqueID(issueID2)

	assert.True(t, strings.HasPrefix(id1, "TEST-001-"))
	assert.True(t, strings.HasPrefix(id2, "PROJ-042-"))
	assert.NotEqual(t, id1, id2)
}
