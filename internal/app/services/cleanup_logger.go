package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// CleanupLogger handles logging of cleanup operations
type CleanupLogger struct {
	basePath string
	logFile  string
}

// CleanupLogEntry represents a single cleanup log entry
type CleanupLogEntry struct {
	Timestamp      time.Time                    `json:"timestamp"`
	Operation      string                       `json:"operation"`
	Target         entities.CleanupTarget       `json:"target,omitempty"`
	Result         *entities.CleanupResult      `json:"result,omitempty"`
	AdminResult    *entities.AdminCleanupResult `json:"admin_result,omitempty"`
	RestoreResult  *entities.RestoreResult      `json:"restore_result,omitempty"`
	FilterCriteria map[string]interface{}       `json:"filter_criteria,omitempty"`
	User           string                       `json:"user,omitempty"`
	Success        bool                         `json:"success"`
	Error          string                       `json:"error,omitempty"`
}

// NewCleanupLogger creates a new cleanup logger
func NewCleanupLogger(basePath string) *CleanupLogger {
	logFile := filepath.Join(basePath, "logs", "cleanup.log")

	// Ensure log directory exists
	logDir := filepath.Dir(logFile)
	os.MkdirAll(logDir, 0755)

	return &CleanupLogger{
		basePath: basePath,
		logFile:  logFile,
	}
}

// LogCleanupOperation logs a regular cleanup operation
func (l *CleanupLogger) LogCleanupOperation(result *entities.CleanupResult, err error) error {
	entry := &CleanupLogEntry{
		Timestamp: time.Now(),
		Operation: "cleanup",
		Result:    result,
		Success:   err == nil,
		User:      getCurrentUser(),
	}

	if err != nil {
		entry.Error = err.Error()
	}

	return l.writeLogEntry(entry)
}

// LogAdminCleanupOperation logs an admin cleanup operation
func (l *CleanupLogger) LogAdminCleanupOperation(result *entities.AdminCleanupResult, err error) error {
	entry := &CleanupLogEntry{
		Timestamp:      time.Now(),
		Operation:      "admin_cleanup",
		Target:         result.Target,
		AdminResult:    result,
		FilterCriteria: result.FilterCriteria,
		Success:        err == nil,
		User:           getCurrentUser(),
	}

	if err != nil {
		entry.Error = err.Error()
	}

	return l.writeLogEntry(entry)
}

// LogRestoreOperation logs a restore operation
func (l *CleanupLogger) LogRestoreOperation(result *entities.RestoreResult, err error) error {
	entry := &CleanupLogEntry{
		Timestamp:     time.Now(),
		Operation:     "restore",
		RestoreResult: result,
		Success:       err == nil,
		User:          getCurrentUser(),
	}

	if err != nil {
		entry.Error = err.Error()
	}

	return l.writeLogEntry(entry)
}

// writeLogEntry writes a log entry to the log file
func (l *CleanupLogger) writeLogEntry(entry *CleanupLogEntry) error {
	// Open log file for appending
	file, err := os.OpenFile(l.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Marshal entry to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Write JSON line
	if _, err := file.Write(append(jsonData, '\n')); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	return nil
}

// GetLogEntries retrieves log entries with optional filtering
func (l *CleanupLogger) GetLogEntries(since *time.Time, operation string, limit int) ([]*CleanupLogEntry, error) {
	file, err := os.Open(l.logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []*CleanupLogEntry{}, nil
		}
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	var entries []*CleanupLogEntry
	decoder := json.NewDecoder(file)

	for decoder.More() {
		var entry CleanupLogEntry
		if err := decoder.Decode(&entry); err != nil {
			// Skip invalid entries
			continue
		}

		// Apply filters
		if since != nil && entry.Timestamp.Before(*since) {
			continue
		}

		if operation != "" && entry.Operation != operation {
			continue
		}

		entries = append(entries, &entry)

		// Apply limit
		if limit > 0 && len(entries) >= limit {
			break
		}
	}

	return entries, nil
}

// getCurrentUser gets the current user name
func getCurrentUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	return "unknown"
}
