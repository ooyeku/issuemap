package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// runCleanupHandler handles POST /api/cleanup
func (s *Server) runCleanupHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Parse query parameters
	dryRun := r.URL.Query().Get("dry_run") == "true"

	// Run cleanup
	result, err := s.cleanupService.RunCleanup(ctx, dryRun)
	if err != nil {
		s.errorResponse(w, "Failed to run cleanup: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, result, http.StatusOK)
}

// getCleanupConfigHandler handles GET /api/cleanup/config
func (s *Server) getCleanupConfigHandler(w http.ResponseWriter, r *http.Request) {
	config := s.cleanupService.GetConfig()
	s.jsonResponse(w, config, http.StatusOK)
}

// updateCleanupConfigHandler handles PUT /api/cleanup/config
func (s *Server) updateCleanupConfigHandler(w http.ResponseWriter, r *http.Request) {
	var config entities.CleanupConfig

	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		s.errorResponse(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.cleanupService.UpdateConfig(&config); err != nil {
		s.errorResponse(w, "Failed to update cleanup config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Cleanup configuration updated successfully",
		"config":  &config,
	}

	s.jsonResponse(w, response, http.StatusOK)
}

// getCleanupStatusHandler handles GET /api/cleanup/status
func (s *Server) getCleanupStatusHandler(w http.ResponseWriter, r *http.Request) {
	config := s.cleanupService.GetConfig()
	lastCleanup := s.cleanupService.GetLastCleanup()

	// Get storage status to check if cleanup is needed
	storageStatus, err := s.storageService.GetStorageStatus(context.Background(), false)
	if err != nil {
		s.errorResponse(w, "Failed to get storage status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	status := map[string]interface{}{
		"enabled":            config.Enabled,
		"schedule":           config.Schedule,
		"dry_run_mode":       config.DryRunMode,
		"last_cleanup":       lastCleanup,
		"needs_cleanup":      s.cleanupService.ShouldRunCleanup(storageStatus),
		"retention_policies": config.RetentionDays,
		"minimum_keep":       config.MinimumKeep,
		"archive_enabled":    config.ArchiveBeforeDelete,
		"archive_path":       config.ArchivePath,
	}

	s.jsonResponse(w, status, http.StatusOK)
}
