package services

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SchedulerService handles scheduled tasks like cleanup
type SchedulerService struct {
	cleanupService *CleanupService
	storageService *StorageService
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	running        bool
	mu             sync.RWMutex
}

// NewSchedulerService creates a new scheduler service
func NewSchedulerService(cleanupService *CleanupService, storageService *StorageService) *SchedulerService {
	return &SchedulerService{
		cleanupService: cleanupService,
		storageService: storageService,
	}
}

// Start begins the scheduler
func (s *SchedulerService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.running = true

	s.wg.Add(1)
	go s.schedulerLoop()

	log.Println("Scheduler service started")
	return nil
}

// Stop gracefully stops the scheduler
func (s *SchedulerService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.cancel()
	s.running = false

	// Wait for scheduler to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Scheduler service stopped")
	case <-time.After(5 * time.Second):
		log.Println("Scheduler service stop timeout")
	}
}

// IsRunning returns whether the scheduler is currently running
func (s *SchedulerService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// schedulerLoop is the main loop that checks for scheduled tasks
func (s *SchedulerService) schedulerLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkScheduledTasks()
		}
	}
}

// checkScheduledTasks checks if any scheduled tasks need to run
func (s *SchedulerService) checkScheduledTasks() {
	config := s.cleanupService.GetConfig()

	// Check if automatic cleanup is enabled
	if !config.Enabled {
		return
	}

	// Check if it's time to run cleanup based on schedule
	if s.shouldRunCleanup(config.Schedule) {
		log.Println("Running scheduled cleanup...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		result, err := s.cleanupService.RunCleanup(ctx, false)
		if err != nil {
			log.Printf("Scheduled cleanup failed: %v", err)
		} else {
			log.Printf("Scheduled cleanup completed: %d items cleaned, %s reclaimed",
				result.ItemsCleaned.Total, formatBytes(result.SpaceReclaimed))
		}
	}

	// Check size-based triggers
	s.checkSizeTriggers()
}

// shouldRunCleanup checks if cleanup should run based on schedule
func (s *SchedulerService) shouldRunCleanup(schedule string) bool {
	if schedule == "" {
		return false
	}

	now := time.Now()
	lastCleanup := s.cleanupService.GetLastCleanup()

	// Simple schedule parsing (cron-like)
	// Format: "0 2 * * *" = daily at 2 AM
	// Format: "0 */6 * * *" = every 6 hours
	// Format: "*/30 * * * *" = every 30 minutes

	parts := strings.Fields(schedule)
	if len(parts) != 5 {
		log.Printf("Invalid schedule format: %s", schedule)
		return false
	}

	minute, hour, day, month, weekday := parts[0], parts[1], parts[2], parts[3], parts[4]

	// Check if we've already run cleanup recently enough
	if s.hasRunRecently(schedule, lastCleanup, now) {
		return false
	}

	// Simple matching for common patterns
	return s.matchesSchedule(now, minute, hour, day, month, weekday)
}

// hasRunRecently checks if cleanup has run recently enough for the given schedule
func (s *SchedulerService) hasRunRecently(schedule string, lastRun, now time.Time) bool {
	if lastRun.IsZero() {
		return false
	}

	// Determine minimum interval based on schedule
	var minInterval time.Duration

	if strings.Contains(schedule, "*/") {
		// Handle interval patterns like */30 * * * * (every 30 minutes)
		if strings.HasPrefix(schedule, "*/") {
			if minutes, err := strconv.Atoi(strings.TrimPrefix(strings.Fields(schedule)[0], "*/")); err == nil {
				minInterval = time.Duration(minutes) * time.Minute
			}
		} else if strings.Contains(schedule, "*/") {
			// Handle hourly intervals like 0 */6 * * * (every 6 hours)
			fields := strings.Fields(schedule)
			if len(fields) > 1 && strings.HasPrefix(fields[1], "*/") {
				if hours, err := strconv.Atoi(strings.TrimPrefix(fields[1], "*/")); err == nil {
					minInterval = time.Duration(hours) * time.Hour
				}
			}
		}
	} else {
		// Default to daily for fixed schedules
		minInterval = 23 * time.Hour // Allow some flexibility
	}

	if minInterval == 0 {
		minInterval = 23 * time.Hour // Default to daily
	}

	return now.Sub(lastRun) < minInterval
}

// matchesSchedule performs simple schedule matching
func (s *SchedulerService) matchesSchedule(now time.Time, minute, hour, day, month, weekday string) bool {
	// For now, implement basic daily scheduling
	// A full cron implementation would be more complex

	if hour == "*" {
		return true // Run every hour (simplified)
	}

	if targetHour, err := strconv.Atoi(hour); err == nil {
		if now.Hour() == targetHour {
			if minute == "*" {
				return true
			}
			if targetMinute, err := strconv.Atoi(minute); err == nil {
				return now.Minute() == targetMinute
			}
		}
	}

	return false
}

// checkSizeTriggers checks if cleanup should be triggered by size limits
func (s *SchedulerService) checkSizeTriggers() {
	// Get current storage status
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	storageStatus, err := s.storageService.GetStorageStatus(ctx, false)
	if err != nil {
		log.Printf("Failed to get storage status for size triggers: %v", err)
		return
	}

	// Check if cleanup should be triggered
	if s.cleanupService.ShouldRunCleanup(storageStatus) {
		log.Println("Running cleanup due to size triggers...")

		result, err := s.cleanupService.RunCleanup(ctx, false)
		if err != nil {
			log.Printf("Size-triggered cleanup failed: %v", err)
		} else {
			log.Printf("Size-triggered cleanup completed: %d items cleaned, %s reclaimed",
				result.ItemsCleaned.Total, formatBytes(result.SpaceReclaimed))
		}
	}
}

// formatBytes is a simple byte formatting function
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return strconv.FormatFloat(float64(bytes)/float64(GB), 'f', 2, 64) + " GB"
	case bytes >= MB:
		return strconv.FormatFloat(float64(bytes)/float64(MB), 'f', 2, 64) + " MB"
	case bytes >= KB:
		return strconv.FormatFloat(float64(bytes)/float64(KB), 'f', 2, 64) + " KB"
	default:
		return strconv.FormatInt(bytes, 10) + " bytes"
	}
}
