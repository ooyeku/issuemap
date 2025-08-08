package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// API Response structures
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Count   int         `json:"count,omitempty"`
}

// IssueDTO is a response-friendly representation of an issue
type IssueDTO struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Status      string            `json:"status"`
	Priority    string            `json:"priority"`
	Labels      []string          `json:"labels"`
	Branch      string            `json:"branch"`
	Timestamps  map[string]string `json:"timestamps"`
}

func issueToDTO(issue *entities.Issue) IssueDTO {
	// Extract label names
	var labelNames []string
	for _, l := range issue.Labels {
		labelNames = append(labelNames, l.Name)
	}
	// Timestamps as strings
	ts := map[string]string{
		"created": issue.Timestamps.Created.Format("2006-01-02T15:04:05Z07:00"),
		"updated": issue.Timestamps.Updated.Format("2006-01-02T15:04:05Z07:00"),
	}
	if issue.Timestamps.Closed != nil {
		ts["closed"] = issue.Timestamps.Closed.Format("2006-01-02T15:04:05Z07:00")
	}

	return IssueDTO{
		ID:          issue.ID.String(),
		Title:       issue.Title,
		Description: issue.Description,
		Type:        string(issue.Type),
		Status:      string(issue.Status),
		Priority:    string(issue.Priority),
		Labels:      labelNames,
		Branch:      issue.Branch,
		Timestamps:  ts,
	}
}

type IssueCreateRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Priority    string   `json:"priority"`
	Assignee    string   `json:"assignee"`
	Labels      []string `json:"labels"`
	Milestone   string   `json:"milestone"`
	Branch      string   `json:"branch"`
}

type IssueUpdateRequest struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Type        *string  `json:"type,omitempty"`
	Priority    *string  `json:"priority,omitempty"`
	Status      *string  `json:"status,omitempty"`
	Assignee    *string  `json:"assignee,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Milestone   *string  `json:"milestone,omitempty"`
	Branch      *string  `json:"branch,omitempty"`
}

type AssignRequest struct {
	Assignee string `json:"assignee"`
}

type CommentRequest struct {
	Text   string `json:"text"`
	Author string `json:"author"`
}

type CloseRequest struct {
	Reason string `json:"reason,omitempty"`
}

// Health check handler
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"status":       "healthy",
			"issues_count": s.memoryStorage.Size(),
			"last_updated": s.memoryStorage.LastModified(),
			"uptime":       "running",
		},
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// Server info handler
func (s *Server) infoHandler(w http.ResponseWriter, r *http.Request) {
	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"name":         app.AppName,
			"version":      app.GetVersion(),
			"description":  app.AppDescription,
			"port":         s.port,
			"api_base":     app.APIBasePath,
			"issues_count": s.memoryStorage.Size(),
		},
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// List issues handler
func (s *Server) listIssuesHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for filtering
	query := r.URL.Query()
	status := query.Get("status")
	priority := query.Get("priority")
	assignee := query.Get("assignee")
	issueType := query.Get("type")
	limit := query.Get("limit")

	var issues []*entities.Issue

	// Apply filters
	if status != "" {
		issues = s.memoryStorage.GetByStatus(status)
	} else if priority != "" {
		issues = s.memoryStorage.GetByPriority(priority)
	} else if assignee != "" {
		issues = s.memoryStorage.GetByAssignee(assignee)
	} else if issueType != "" {
		issues = s.memoryStorage.GetByType(issueType)
	} else {
		issues = s.memoryStorage.GetAll()
	}

	// Apply limit
	if limit != "" {
		if limitNum, err := strconv.Atoi(limit); err == nil && limitNum > 0 && limitNum < len(issues) {
			issues = issues[:limitNum]
		}
	}

	// Convert to DTOs
	dto := make([]IssueDTO, 0, len(issues))
	for _, iss := range issues {
		dto = append(dto, issueToDTO(iss))
	}

	response := APIResponse{
		Success: true,
		Data:    dto,
		Count:   len(dto),
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// Get single issue handler
func (s *Server) getIssueHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	issueID := entities.IssueID(vars["id"])

	issue, exists := s.memoryStorage.Get(issueID)
	if !exists {
		s.errorResponse(w, app.ErrIssueNotFound, http.StatusNotFound)
		return
	}

	response := APIResponse{
		Success: true,
		Data:    issueToDTO(issue),
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// Create issue handler (simplified for now)
func (s *Server) createIssueHandler(w http.ResponseWriter, r *http.Request) {
	var req IssueCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Title == "" {
		s.errorResponse(w, "Title is required", http.StatusBadRequest)
		return
	}

	// For now, return a mock response until we fix the service interface
	response := APIResponse{
		Success: true,
		Data:    map[string]string{"message": "Issue creation via API not yet implemented"},
	}
	s.jsonResponse(w, response, http.StatusNotImplemented)
}

// Update issue handler
func (s *Server) updateIssueHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	issueID := entities.IssueID(vars["id"])

	// Check if issue exists
	_, exists := s.memoryStorage.Get(issueID)
	if !exists {
		s.errorResponse(w, app.ErrIssueNotFound, http.StatusNotFound)
		return
	}

	var req IssueUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Build updates map
	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Assignee != nil {
		updates["assignee"] = *req.Assignee
	}
	if req.Labels != nil {
		updates["labels"] = req.Labels
	}
	if req.Milestone != nil {
		updates["milestone"] = *req.Milestone
	}
	if req.Branch != nil {
		updates["branch"] = *req.Branch
	}

	// Update through service
	ctx := context.Background()
	issue, err := s.issueService.UpdateIssue(ctx, issueID, updates)
	if err != nil {
		s.errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update memory storage
	s.memoryStorage.Update(issue)

	response := APIResponse{
		Success: true,
		Data:    issue,
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// Delete issue handler (simplified - just remove from memory for now)
func (s *Server) deleteIssueHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	issueID := entities.IssueID(vars["id"])

	// Check if issue exists
	_, exists := s.memoryStorage.Get(issueID)
	if !exists {
		s.errorResponse(w, app.ErrIssueNotFound, http.StatusNotFound)
		return
	}

	// Remove from memory storage (no persistent deletion yet)
	s.memoryStorage.Remove(issueID)

	response := APIResponse{
		Success: true,
		Data:    map[string]string{"message": "Issue removed from memory (not persistent)"},
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// Close issue handler
func (s *Server) closeIssueHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	issueID := entities.IssueID(vars["id"])

	var req CloseRequest
	json.NewDecoder(r.Body).Decode(&req) // Optional body

	ctx := context.Background()
	err := s.issueService.CloseIssue(ctx, issueID, req.Reason)
	if err != nil {
		s.errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get updated issue
	issue, err := s.issueService.GetIssue(ctx, issueID)
	if err != nil {
		s.errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update memory storage
	s.memoryStorage.Update(issue)

	response := APIResponse{
		Success: true,
		Data:    issue,
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// Reopen issue handler
func (s *Server) reopenIssueHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	issueID := entities.IssueID(vars["id"])

	ctx := context.Background()
	err := s.issueService.ReopenIssue(ctx, issueID)
	if err != nil {
		s.errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get updated issue
	issue, err := s.issueService.GetIssue(ctx, issueID)
	if err != nil {
		s.errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update memory storage
	s.memoryStorage.Update(issue)

	response := APIResponse{
		Success: true,
		Data:    issue,
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// Assign issue handler
func (s *Server) assignIssueHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	issueID := entities.IssueID(vars["id"])

	var req AssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	issue, err := s.issueService.UpdateIssue(ctx, issueID, map[string]interface{}{
		"assignee": req.Assignee,
	})
	if err != nil {
		s.errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update memory storage
	s.memoryStorage.Update(issue)

	response := APIResponse{
		Success: true,
		Data:    issue,
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// Add comment handler
func (s *Server) addCommentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	issueID := entities.IssueID(vars["id"])

	var req CommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.Text == "" {
		s.errorResponse(w, "Comment text is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	err := s.issueService.AddComment(ctx, issueID, req.Text, req.Author)
	if err != nil {
		s.errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get updated issue
	issue, err := s.issueService.GetIssue(ctx, issueID)
	if err != nil {
		s.errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update memory storage
	s.memoryStorage.Update(issue)

	response := APIResponse{
		Success: true,
		Data:    issue,
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// List history handler
func (s *Server) listHistoryHandler(w http.ResponseWriter, r *http.Request) {
	// This would integrate with the history service
	// For now, return a placeholder
	response := APIResponse{
		Success: true,
		Data:    []string{}, // Placeholder
		Count:   0,
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// Get issue history handler
func (s *Server) getIssueHistoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	issueID := entities.IssueID(vars["id"])

	// Check if issue exists
	_, exists := s.memoryStorage.Get(issueID)
	if !exists {
		s.errorResponse(w, app.ErrIssueNotFound, http.StatusNotFound)
		return
	}

	// This would integrate with the history service
	// For now, return a placeholder
	response := APIResponse{
		Success: true,
		Data:    map[string]interface{}{"issue_id": issueID, "history": []string{}},
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// Get statistics handler
func (s *Server) getStatsHandler(w http.ResponseWriter, r *http.Request) {
	allIssues := s.memoryStorage.GetAll()

	stats := map[string]interface{}{
		"total_issues": len(allIssues),
		"by_status": map[string]int{
			app.StatusOpen:       len(s.memoryStorage.GetByStatus(app.StatusOpen)),
			app.StatusClosed:     len(s.memoryStorage.GetByStatus(app.StatusClosed)),
			app.StatusInProgress: len(s.memoryStorage.GetByStatus(app.StatusInProgress)),
		},
		"by_priority": map[string]int{
			app.PriorityLow:      len(s.memoryStorage.GetByPriority(app.PriorityLow)),
			app.PriorityMedium:   len(s.memoryStorage.GetByPriority(app.PriorityMedium)),
			app.PriorityHigh:     len(s.memoryStorage.GetByPriority(app.PriorityHigh)),
			app.PriorityCritical: len(s.memoryStorage.GetByPriority(app.PriorityCritical)),
		},
		"by_type": map[string]int{
			app.TypeBug:           len(s.memoryStorage.GetByType(app.TypeBug)),
			app.TypeFeature:       len(s.memoryStorage.GetByType(app.TypeFeature)),
			app.TypeTask:          len(s.memoryStorage.GetByType(app.TypeTask)),
			app.TypeImprovement:   len(s.memoryStorage.GetByType(app.TypeImprovement)),
			app.TypeDocumentation: len(s.memoryStorage.GetByType(app.TypeDocumentation)),
		},
	}

	response := APIResponse{
		Success: true,
		Data:    stats,
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// Get summary handler
func (s *Server) getSummaryHandler(w http.ResponseWriter, r *http.Request) {
	allIssues := s.memoryStorage.GetAll()
	openIssues := s.memoryStorage.GetByStatus(app.StatusOpen)
	closedIssues := s.memoryStorage.GetByStatus(app.StatusClosed)

	summary := map[string]interface{}{
		"total_issues":  len(allIssues),
		"open_issues":   len(openIssues),
		"closed_issues": len(closedIssues),
		"completion_rate": func() float64 {
			if len(allIssues) == 0 {
				return 0
			}
			return float64(len(closedIssues)) / float64(len(allIssues)) * 100
		}(),
		"last_updated": s.memoryStorage.LastModified(),
	}

	response := APIResponse{
		Success: true,
		Data:    summary,
	}
	s.jsonResponse(w, response, http.StatusOK)
}
