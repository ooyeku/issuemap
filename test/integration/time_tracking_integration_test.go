package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

func TestTimeTrackingIntegration(t *testing.T) {
	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "issuemap-time-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize repositories
	issueRepo := storage.NewFileIssueRepository(tempDir)
	configRepo := storage.NewFileConfigRepository(tempDir)
	timeEntryRepo := storage.NewFileTimeEntryRepository(tempDir)
	activeTimerRepo := storage.NewFileActiveTimerRepository(tempDir)
	historyRepo := storage.NewFileHistoryRepository(tempDir)

	// Initialize services
	issueService := services.NewIssueService(issueRepo, configRepo, nil)
	historyService := services.NewHistoryService(historyRepo, nil)
	timeTrackingService := services.NewTimeTrackingService(
		timeEntryRepo,
		activeTimerRepo,
		issueService,
		historyService,
	)

	ctx := context.Background()

	// Create a test issue
	issue, err := issueService.CreateIssue(ctx, services.CreateIssueRequest{
		Title:       "Test Issue for Time Tracking",
		Description: "Integration test issue",
		Type:        entities.IssueTypeTask,
		Priority:    entities.PriorityMedium,
	})
	require.NoError(t, err)

	// Set an estimate
	updates := map[string]interface{}{
		"estimated_hours": 4.0,
	}
	issue, err = issueService.UpdateIssue(ctx, issue.ID, updates)
	require.NoError(t, err)
	assert.Equal(t, 4.0, issue.GetEstimatedHours())

	author := "test.user"

	t.Run("Start and Stop Timer", func(t *testing.T) {
		// Start timer
		activeTimer, err := timeTrackingService.StartTimer(ctx, issue.ID, author, "Working on integration test")
		require.NoError(t, err)
		assert.Equal(t, issue.ID, activeTimer.IssueID)
		assert.Equal(t, author, activeTimer.Author)

		// Verify active timer exists
		foundTimer, err := timeTrackingService.GetActiveTimer(ctx, author)
		require.NoError(t, err)
		assert.Equal(t, activeTimer.IssueID, foundTimer.IssueID)

		// Try to start another timer (should fail)
		_, err = timeTrackingService.StartTimer(ctx, issue.ID, author, "Another timer")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "active timer already exists")

		// Wait a short time
		time.Sleep(10 * time.Millisecond)

		// Stop timer
		timeEntry, err := timeTrackingService.StopTimer(ctx, author)
		require.NoError(t, err)
		assert.Equal(t, issue.ID, timeEntry.IssueID)
		assert.Equal(t, entities.TimeEntryTypeTimer, timeEntry.Type)
		assert.True(t, timeEntry.Duration > 0)

		// Verify active timer is removed
		_, err = timeTrackingService.GetActiveTimer(ctx, author)
		assert.Error(t, err)

		// Verify issue actual hours updated
		updatedIssue, err := issueService.GetIssue(ctx, issue.ID)
		require.NoError(t, err)
		assert.True(t, updatedIssue.GetActualHours() > 0)
	})

	t.Run("Manual Time Logging", func(t *testing.T) {
		// Log some manual time
		duration := 90 * time.Minute // 1.5 hours
		description := "Manual work done offline"

		timeEntry, err := timeTrackingService.LogTime(ctx, issue.ID, author, duration, description)
		require.NoError(t, err)
		assert.Equal(t, entities.TimeEntryTypeManual, timeEntry.Type)
		assert.Equal(t, duration, timeEntry.Duration)
		assert.Equal(t, description, timeEntry.Description)

		// Verify issue actual hours updated (should be at least the manual entry)
		updatedIssue, err := issueService.GetIssue(ctx, issue.ID)
		require.NoError(t, err)
		assert.True(t, updatedIssue.GetActualHours() >= 1.5) // At least 1.5 hours from manual entry
	})

	t.Run("Time Entry Queries", func(t *testing.T) {
		// Get all time entries
		allEntries, err := timeTrackingService.GetTimeEntries(ctx, repositories.TimeEntryFilter{})
		require.NoError(t, err)
		assert.Len(t, allEntries, 2) // One timer + one manual

		// Get entries for specific issue
		issueEntries, err := timeTrackingService.GetTimeEntriesByIssue(ctx, issue.ID)
		require.NoError(t, err)
		assert.Len(t, issueEntries, 2)

		// Get entries for specific author
		filter := repositories.TimeEntryFilter{
			Author: &author,
		}
		authorEntries, err := timeTrackingService.GetTimeEntries(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, authorEntries, 2)

		// Get entries by type
		manualType := entities.TimeEntryTypeManual
		typeFilter := repositories.TimeEntryFilter{
			Type: &manualType,
		}
		manualEntries, err := timeTrackingService.GetTimeEntries(ctx, typeFilter)
		require.NoError(t, err)
		assert.Len(t, manualEntries, 1)
		assert.Equal(t, entities.TimeEntryTypeManual, manualEntries[0].Type)
	})

	t.Run("Time Statistics", func(t *testing.T) {
		stats, err := timeTrackingService.GetTimeStats(ctx, repositories.TimeEntryFilter{})
		require.NoError(t, err)

		assert.Equal(t, 2, stats.TotalEntries)
		assert.True(t, stats.TotalTime > 0)
		assert.Equal(t, 1, stats.UniqueAuthors)
		assert.Contains(t, stats.TimeByAuthor, author)
		assert.Contains(t, stats.TimeByIssue, issue.ID)
		assert.Len(t, stats.EntriesByType, 2) // Manual and Timer types
	})

	t.Run("Stop Timer Without Active Timer", func(t *testing.T) {
		// Try to stop timer when none is active
		_, err := timeTrackingService.StopTimer(ctx, author)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no active timer found")
	})

	t.Run("Log Time for Non-existent Issue", func(t *testing.T) {
		_, err := timeTrackingService.LogTime(ctx, "NONEXISTENT", author, time.Hour, "Test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "issue not found")
	})
}

func TestTimeTrackingPersistence(t *testing.T) {
	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "issuemap-persistence-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize repositories
	timeEntryRepo := storage.NewFileTimeEntryRepository(tempDir)
	activeTimerRepo := storage.NewFileActiveTimerRepository(tempDir)

	ctx := context.Background()
	author := "test.user"
	issueID := entities.IssueID("ISSUE-001")

	t.Run("Time Entry Persistence", func(t *testing.T) {
		// Create and save time entry
		entry := entities.NewTimeEntry(issueID, entities.TimeEntryTypeManual, 2*time.Hour, "Test work", author)
		err := timeEntryRepo.Create(ctx, entry)
		require.NoError(t, err)

		// Retrieve time entry
		retrieved, err := timeEntryRepo.GetByID(ctx, entry.ID)
		require.NoError(t, err)
		assert.Equal(t, entry.ID, retrieved.ID)
		assert.Equal(t, entry.IssueID, retrieved.IssueID)
		assert.Equal(t, entry.Duration, retrieved.Duration)

		// Update time entry
		retrieved.Description = "Updated description"
		err = timeEntryRepo.Update(ctx, retrieved)
		require.NoError(t, err)

		// Verify update
		updated, err := timeEntryRepo.GetByID(ctx, entry.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", updated.Description)

		// Delete time entry
		err = timeEntryRepo.Delete(ctx, entry.ID)
		require.NoError(t, err)

		// Verify deletion
		_, err = timeEntryRepo.GetByID(ctx, entry.ID)
		assert.Error(t, err)
	})

	t.Run("Active Timer Persistence", func(t *testing.T) {
		// Create and save active timer
		timer := entities.NewActiveTimer(issueID, "Test timer", author)
		err := activeTimerRepo.Create(ctx, timer)
		require.NoError(t, err)

		// Retrieve active timer
		retrieved, err := activeTimerRepo.GetByAuthor(ctx, author)
		require.NoError(t, err)
		assert.Equal(t, timer.IssueID, retrieved.IssueID)
		assert.Equal(t, timer.Author, retrieved.Author)

		// Try to create another timer for same author (should fail)
		timer2 := entities.NewActiveTimer("ISSUE-002", "Another timer", author)
		err = activeTimerRepo.Create(ctx, timer2)
		assert.Error(t, err)

		// List all active timers
		timers, err := activeTimerRepo.List(ctx)
		require.NoError(t, err)
		assert.Len(t, timers, 1)

		// Delete active timer
		err = activeTimerRepo.Delete(ctx, issueID, author)
		require.NoError(t, err)

		// Verify deletion
		_, err = activeTimerRepo.GetByAuthor(ctx, author)
		assert.Error(t, err)
	})

	t.Run("Time Entry Filtering", func(t *testing.T) {
		// Clear any existing entries first to ensure clean state
		allEntries, _ := timeEntryRepo.List(ctx, repositories.TimeEntryFilter{})
		for _, entry := range allEntries {
			timeEntryRepo.Delete(ctx, entry.ID)
		}

		// Create multiple time entries with distinct data
		entries := []*entities.TimeEntry{
			entities.NewTimeEntry("ISSUE-001", entities.TimeEntryTypeManual, time.Hour, "Work 1", "user1"),
			entities.NewTimeEntry("ISSUE-001", entities.TimeEntryTypeTimer, 2*time.Hour, "Work 2", "user1"),
			entities.NewTimeEntry("ISSUE-002", entities.TimeEntryTypeManual, 30*time.Minute, "Work 3", "user2"),
		}

		// Save all entries and verify they were created
		for i, entry := range entries {
			err := timeEntryRepo.Create(ctx, entry)
			require.NoError(t, err, "Failed to create entry %d", i)

			// Verify the entry was saved correctly
			retrieved, err := timeEntryRepo.GetByID(ctx, entry.ID)
			require.NoError(t, err, "Failed to retrieve entry %d", i)
			assert.Equal(t, entry.IssueID, retrieved.IssueID, "IssueID mismatch for entry %d", i)
			assert.Equal(t, entry.Author, retrieved.Author, "Author mismatch for entry %d", i)
			assert.Equal(t, entry.Type, retrieved.Type, "Type mismatch for entry %d", i)
		}

		// Verify all entries exist
		allResults, err := timeEntryRepo.List(ctx, repositories.TimeEntryFilter{})
		require.NoError(t, err)
		assert.Len(t, allResults, 3, "Should have 3 total entries")

		// Test filtering by issue (should return 2 entries for ISSUE-001)
		issueID1 := entities.IssueID("ISSUE-001")
		filter := repositories.TimeEntryFilter{IssueID: &issueID1}
		results, err := timeEntryRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, results, 2, "Should have 2 entries for ISSUE-001")
		for _, result := range results {
			assert.Equal(t, "ISSUE-001", string(result.IssueID))
		}

		// Test filtering by author (should return 2 entries for user1)
		user1 := "user1"
		filter = repositories.TimeEntryFilter{Author: &user1}
		results, err = timeEntryRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, results, 2, "Should have 2 entries for user1")
		for _, result := range results {
			assert.Equal(t, "user1", result.Author)
		}

		// Test filtering by type (should return 2 manual entries)
		manualType := entities.TimeEntryTypeManual
		filter = repositories.TimeEntryFilter{Type: &manualType}
		results, err = timeEntryRepo.List(ctx, filter)
		require.NoError(t, err)

		assert.Len(t, results, 2, "Should have 2 manual entries")
		for _, result := range results {
			assert.Equal(t, entities.TimeEntryTypeManual, result.Type)
		}

		// Test limit functionality
		filter = repositories.TimeEntryFilter{Limit: 2}
		results, err = timeEntryRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(results), 2, "Should have at most 2 entries with limit")
	})
}

func TestFileTimeEntryRepositoryPaths(t *testing.T) {
	// Test that files are created in expected locations
	tempDir, err := os.MkdirTemp("", "issuemap-paths-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	timeEntryRepo := storage.NewFileTimeEntryRepository(tempDir)
	activeTimerRepo := storage.NewFileActiveTimerRepository(tempDir)

	ctx := context.Background()

	// Create time entry
	entry := entities.NewTimeEntry("ISSUE-001", entities.TimeEntryTypeManual, time.Hour, "Test", "author")
	err = timeEntryRepo.Create(ctx, entry)
	require.NoError(t, err)

	// Check that time entries directory exists
	timeEntriesDir := filepath.Join(tempDir, "time_entries")
	assert.DirExists(t, timeEntriesDir)

	// Check that entry file exists
	entryFile := filepath.Join(timeEntriesDir, entry.ID+".yaml")
	assert.FileExists(t, entryFile)

	// Create active timer
	timer := entities.NewActiveTimer("ISSUE-001", "Test", "author")
	err = activeTimerRepo.Create(ctx, timer)
	require.NoError(t, err)

	// Check that timer file exists
	timerFile := filepath.Join(tempDir, "timer_author.yaml")
	assert.FileExists(t, timerFile)
}
