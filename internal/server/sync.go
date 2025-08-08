package server

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"os"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// SyncService handles synchronization between disk and memory
type SyncService struct {
	server     *Server
	watcher    *fsnotify.Watcher
	issuesPath string
	stopChan   chan bool
}

// NewSyncService creates a new sync service
func NewSyncService(server *Server, basePath string) (*SyncService, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	issuesPath := filepath.Join(basePath, app.IssuesDirName)

	return &SyncService{
		server:     server,
		watcher:    watcher,
		issuesPath: issuesPath,
		stopChan:   make(chan bool),
	}, nil
}

// Start begins watching for file system changes
func (s *SyncService) Start() error {
	// Add the issues directory to the watcher
	// Ensure directory exists
	if err := ensureDirExists(s.issuesPath); err != nil {
		return err
	}

	err := s.watcher.Add(s.issuesPath)
	if err != nil {
		return err
	}

	log.Printf("Started file system sync for: %s", s.issuesPath)

	// Start the event processing goroutine
	go s.processEvents()

	return nil
}

// ensureDirExists creates the directory if it doesn't exist
func ensureDirExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

// Stop stops the sync service
func (s *SyncService) Stop() {
	close(s.stopChan)
	s.watcher.Close()
	log.Println("Stopped file system sync service")
}

// processEvents handles file system events
func (s *SyncService) processEvents() {
	// Debounce timer to handle rapid file changes
	var debounceTimer *time.Timer
	debounceDuration := 50 * time.Millisecond

	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}

			// Only process YAML files in the issues directory
			if !strings.HasSuffix(event.Name, app.IssueFileExtension) {
				continue
			}

			// Reset the debounce timer
			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			// Debounced full reload to ensure no events are dropped under bursty writes
			debounceTimer = time.AfterFunc(debounceDuration, func() {
				// Reload from disk to reflect all recent changes
				if err := s.server.loadIssuesIntoMemory(); err != nil {
					log.Printf("Failed to reload issues from disk: %v", err)
				}
			})

		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)

		case <-s.stopChan:
			return
		}
	}
}

// handleFileEvent processes a single file system event
func (s *SyncService) handleFileEvent(event fsnotify.Event) {
	fileName := filepath.Base(event.Name)
	issueID := strings.TrimSuffix(fileName, app.IssueFileExtension)

	log.Printf("File event: %s on %s (issueID: %s)", event.Op.String(), fileName, issueID)

	switch {
	case event.Op&fsnotify.Create != 0:
		s.handleIssueCreated(issueID)
	case event.Op&fsnotify.Write != 0:
		s.handleIssueUpdated(issueID)
	case event.Op&fsnotify.Remove != 0:
		s.handleIssueDeleted(issueID)
	case event.Op&fsnotify.Rename != 0:
		// Handle rename as a delete (old file removed)
		s.handleIssueDeleted(issueID)
	}
}

// handleIssueCreated handles creation of a new issue file
func (s *SyncService) handleIssueCreated(issueID string) {
	ctx := context.Background()

	// Load the new issue from disk
	issue, err := s.server.issueService.GetIssue(ctx, entities.IssueID(issueID))
	if err != nil {
		log.Printf("Failed to load created issue %s: %v", issueID, err)
		return
	}

	// Add to memory storage
	s.server.memoryStorage.Add(issue)
	log.Printf("Synced: Added issue %s to memory", issueID)
}

// handleIssueUpdated handles modification of an existing issue file
func (s *SyncService) handleIssueUpdated(issueID string) {
	ctx := context.Background()

	// Load the updated issue from disk
	issue, err := s.server.issueService.GetIssue(ctx, entities.IssueID(issueID))
	if err != nil {
		log.Printf("Failed to load updated issue %s: %v", issueID, err)
		return
	}

	// Update in memory storage
	updated := s.server.memoryStorage.Update(issue)
	if updated {
		log.Printf("Synced: Updated issue %s in memory", issueID)
	} else {
		// Issue doesn't exist in memory, add it
		s.server.memoryStorage.Add(issue)
		log.Printf("Synced: Added new issue %s to memory", issueID)
	}
}

// handleIssueDeleted handles deletion of an issue file
func (s *SyncService) handleIssueDeleted(issueID string) {
	// Remove from memory storage
	removed := s.server.memoryStorage.Remove(entities.IssueID(issueID))
	if removed {
		log.Printf("Synced: Removed issue %s from memory", issueID)
	}
}

// ReloadFromDisk forces a complete reload from disk (for manual sync)
func (s *SyncService) ReloadFromDisk() error {
	log.Println("Performing full reload from disk...")
	return s.server.loadIssuesIntoMemory()
}
