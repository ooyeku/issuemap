package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

// API Response structures
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Count   int         `json:"count,omitempty"`
}

// IssueDTO is a response-friendly representation of an issue
// Extended to support rich Details UI
type IssueDTO struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Status      string            `json:"status"`
	Priority    string            `json:"priority"`
	Labels      []string          `json:"labels"`
	Branch      string            `json:"branch"`
	Assignee    string            `json:"assignee,omitempty"`
	Milestone   *MilestoneDTO     `json:"milestone,omitempty"`
	Metadata    *MetadataDTO      `json:"metadata,omitempty"`
	Comments    []CommentDTO      `json:"comments,omitempty"`
	Commits     []CommitDTO       `json:"commits,omitempty"`
	Attachments []AttachmentDTO   `json:"attachments,omitempty"`
	Timestamps  map[string]string `json:"timestamps"`
}

type MilestoneDTO struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
}

type MetadataDTO struct {
	EstimatedHours float64           `json:"estimated_hours,omitempty"`
	ActualHours    float64           `json:"actual_hours,omitempty"`
	RemainingHours float64           `json:"remaining_hours,omitempty"`
	OverEstimate   bool              `json:"over_estimate,omitempty"`
	CustomFields   map[string]string `json:"custom_fields,omitempty"`
}

type CommentDTO struct {
	ID     int    `json:"id"`
	Author string `json:"author"`
	Date   string `json:"date"`
	Text   string `json:"text"`
}

type CommitDTO struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

type AttachmentDTO struct {
	ID            string `json:"id"`
	Filename      string `json:"filename"`
	ContentType   string `json:"content_type"`
	Size          int64  `json:"size"`
	SizeFormatted string `json:"size_formatted"`
	Type          string `json:"type"`
	UploadedBy    string `json:"uploaded_by"`
	UploadedAt    string `json:"uploaded_at"`
	Description   string `json:"description,omitempty"`
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

	// Assignee
	assignee := ""
	if issue.Assignee != nil {
		if issue.Assignee.Username != "" {
			assignee = issue.Assignee.Username
		} else if issue.Assignee.Email != "" {
			assignee = issue.Assignee.Email
		}
	}

	// Milestone
	var milestoneDTO *MilestoneDTO
	if issue.Milestone != nil {
		ms := &MilestoneDTO{Name: issue.Milestone.Name, Description: issue.Milestone.Description}
		if issue.Milestone.DueDate != nil {
			ms.DueDate = issue.Milestone.DueDate.Format("2006-01-02")
		}
		milestoneDTO = ms
	}

	// Metadata
	var metaDTO *MetadataDTO
	{
		est := issue.GetEstimatedHours()
		act := issue.GetActualHours()
		rem := issue.GetRemainingHours()
		meta := &MetadataDTO{
			EstimatedHours: est,
			ActualHours:    act,
			RemainingHours: rem,
			OverEstimate:   issue.IsOverEstimate(),
			CustomFields:   nil,
		}
		if len(issue.Metadata.CustomFields) > 0 {
			meta.CustomFields = issue.Metadata.CustomFields
		}
		// Only attach if any values are non-zero or custom fields exist
		if meta.EstimatedHours != 0 || meta.ActualHours != 0 || meta.RemainingHours != 0 || meta.OverEstimate || meta.CustomFields != nil {
			metaDTO = meta
		}
	}

	// Comments
	comments := make([]CommentDTO, 0, len(issue.Comments))
	for _, c := range issue.Comments {
		comments = append(comments, CommentDTO{
			ID:     c.ID,
			Author: c.Author,
			Date:   c.Date.Format("2006-01-02T15:04:05Z07:00"),
			Text:   c.Text,
		})
	}

	// Commits
	commits := make([]CommitDTO, 0, len(issue.Commits))
	for _, cm := range issue.Commits {
		commits = append(commits, CommitDTO{
			Hash:    cm.Hash,
			Message: cm.Message,
			Author:  cm.Author,
			Date:    cm.Date.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	// Attachments
	attachments := make([]AttachmentDTO, 0, len(issue.Attachments))
	for _, att := range issue.Attachments {
		attachments = append(attachments, AttachmentDTO{
			ID:            att.ID,
			Filename:      att.Filename,
			ContentType:   att.ContentType,
			Size:          att.Size,
			SizeFormatted: att.GetSizeFormatted(),
			Type:          string(att.Type),
			UploadedBy:    att.UploadedBy,
			UploadedAt:    att.UploadedAt.Format("2006-01-02T15:04:05Z07:00"),
			Description:   att.Description,
		})
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
		Assignee:    assignee,
		Milestone:   milestoneDTO,
		Metadata:    metaDTO,
		Comments:    comments,
		Commits:     commits,
		Attachments: attachments,
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
	// Try to load project name from config; fail gracefully
	projectName := ""
	if repo := storage.NewFileConfigRepository(s.basePath); repo != nil {
		if cfg, err := repo.Load(context.Background()); err == nil && cfg != nil {
			projectName = cfg.Project.Name
		}
	}

	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"name":         app.AppName,
			"version":      app.GetVersion(),
			"description":  app.AppDescription,
			"port":         s.port,
			"api_base":     app.APIBasePath,
			"issues_count": s.memoryStorage.Size(),
			"project_name": projectName,
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

	// Use service to enrich (e.g., commits from git)
	ctx := context.Background()
	issue, err := s.issueService.GetIssue(ctx, issueID)
	if err != nil || issue == nil {
		s.errorResponse(w, app.ErrIssueNotFound, http.StatusNotFound)
		return
	}

	// Ensure memory storage stays in sync
	s.memoryStorage.Update(issue)

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
	_ = json.NewDecoder(r.Body).Decode(&req) // Optional body

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
	err := s.issueService.AddComment(ctx, issueID, req.Author, req.Text)
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

// getCommitDiffHandler returns a commit diff with files and patches
func (s *Server) getCommitDiffHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hash := vars["hash"]
	if hash == "" {
		s.errorResponse(w, "commit hash required", http.StatusBadRequest)
		return
	}
	ctx := context.Background()
	diff, err := s.issueService.GetCommitDiff(ctx, hash)
	if err != nil {
		s.errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Marshal a DTO formatting the date as RFC3339
	resp := map[string]interface{}{
		"hash":    diff.Hash,
		"message": diff.Message,
		"author":  diff.Author,
		"email":   diff.Email,
		"date":    diff.Date.Format("2006-01-02T15:04:05Z07:00"),
		"files":   diff.Files,
	}
	s.jsonResponse(w, APIResponse{Success: true, Data: resp}, http.StatusOK)
}

// listAttachmentsHandler returns all attachments for an issue
func (s *Server) listAttachmentsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	issueID := entities.IssueID(vars["id"])

	ctx := context.Background()
	attachments, err := s.attachmentService.ListIssueAttachments(ctx, issueID)
	if err != nil {
		s.errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to DTOs
	dtos := make([]AttachmentDTO, 0, len(attachments))
	for _, att := range attachments {
		dtos = append(dtos, AttachmentDTO{
			ID:            att.ID,
			Filename:      att.Filename,
			ContentType:   att.ContentType,
			Size:          att.Size,
			SizeFormatted: att.GetSizeFormatted(),
			Type:          string(att.Type),
			UploadedBy:    att.UploadedBy,
			UploadedAt:    att.UploadedAt.Format("2006-01-02T15:04:05Z07:00"),
			Description:   att.Description,
		})
	}

	response := APIResponse{
		Success: true,
		Data:    dtos,
		Count:   len(dtos),
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// uploadAttachmentHandler handles file upload for an issue
func (s *Server) uploadAttachmentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	issueID := entities.IssueID(vars["id"])

	// Validate issue ID format
	if issueID == "" {
		s.errorResponse(w, "Invalid issue ID", http.StatusBadRequest)
		return
	}

	// Check content length before parsing
	if r.ContentLength > 10<<20 { // 10MB
		s.errorResponse(w, "File too large. Maximum size is 10MB", http.StatusRequestEntityTooLarge)
		return
	}

	// Parse multipart form (10MB max)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		log.Printf("Failed to parse multipart form: %v", err)
		s.errorResponse(w, "Failed to parse upload form", http.StatusBadRequest)
		return
	}

	// Get file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("Failed to get file from form: %v", err)
		s.errorResponse(w, "No file provided in upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file size
	if header.Size <= 0 {
		s.errorResponse(w, "Invalid file size", http.StatusBadRequest)
		return
	}

	// Get optional description and validate
	description := strings.TrimSpace(r.FormValue("description"))
	if len(description) > 500 {
		s.errorResponse(w, "Description too long (max 500 characters)", http.StatusBadRequest)
		return
	}

	uploadedBy := strings.TrimSpace(r.FormValue("uploaded_by"))
	if uploadedBy == "" {
		uploadedBy = "anonymous"
	}
	if len(uploadedBy) > 100 {
		s.errorResponse(w, "Uploaded by field too long (max 100 characters)", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	attachment, err := s.attachmentService.UploadAttachment(ctx, issueID, header.Filename, file, header.Size, uploadedBy)
	if err != nil {
		log.Printf("Failed to upload attachment: %v", err)
		// Check for specific error types to provide better user feedback
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError

		if strings.Contains(errMsg, "security_validation") ||
			strings.Contains(errMsg, "mime_validation") ||
			strings.Contains(errMsg, "not allowed") ||
			strings.Contains(errMsg, "invalid") {
			statusCode = http.StatusBadRequest
		} else if strings.Contains(errMsg, "not found") {
			statusCode = http.StatusNotFound
		}

		s.errorResponse(w, errMsg, statusCode)
		return
	}

	// Update description if provided
	if description != "" {
		attachment.Description = description
		// Save the updated metadata
		ctx := context.Background()
		if err := s.attachmentService.UpdateDescription(ctx, attachment.ID, description); err != nil {
			log.Printf("Failed to update attachment description: %v", err)
			// Continue anyway, don't fail the whole upload
		}
	}

	// Convert to DTO
	dto := AttachmentDTO{
		ID:            attachment.ID,
		Filename:      attachment.Filename,
		ContentType:   attachment.ContentType,
		Size:          attachment.Size,
		SizeFormatted: attachment.GetSizeFormatted(),
		Type:          string(attachment.Type),
		UploadedBy:    attachment.UploadedBy,
		UploadedAt:    attachment.UploadedAt.Format("2006-01-02T15:04:05Z07:00"),
		Description:   attachment.Description,
	}

	// Update memory storage with the modified issue
	issue, _ := s.issueService.GetIssue(ctx, issueID)
	if issue != nil {
		s.memoryStorage.Update(issue)
	}

	response := APIResponse{
		Success: true,
		Data:    dto,
	}
	s.jsonResponse(w, response, http.StatusCreated)
}

// getAttachmentHandler returns attachment metadata
func (s *Server) getAttachmentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attachmentID := vars["id"]

	ctx := context.Background()
	attachment, err := s.attachmentService.GetAttachment(ctx, attachmentID)
	if err != nil {
		s.errorResponse(w, "Attachment not found", http.StatusNotFound)
		return
	}

	dto := AttachmentDTO{
		ID:            attachment.ID,
		Filename:      attachment.Filename,
		ContentType:   attachment.ContentType,
		Size:          attachment.Size,
		SizeFormatted: attachment.GetSizeFormatted(),
		Type:          string(attachment.Type),
		UploadedBy:    attachment.UploadedBy,
		UploadedAt:    attachment.UploadedAt.Format("2006-01-02T15:04:05Z07:00"),
		Description:   attachment.Description,
	}

	response := APIResponse{
		Success: true,
		Data:    dto,
	}
	s.jsonResponse(w, response, http.StatusOK)
}

// downloadAttachmentHandler streams the attachment content
func (s *Server) downloadAttachmentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attachmentID := vars["id"]

	// Validate attachment ID
	if attachmentID == "" || len(attachmentID) > 200 {
		s.errorResponse(w, "Invalid attachment ID", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	content, attachment, err := s.attachmentService.GetAttachmentContent(ctx, attachmentID)
	if err != nil {
		log.Printf("Failed to get attachment content: %v", err)
		s.errorResponse(w, "Attachment not found", http.StatusNotFound)
		return
	}
	defer func() {
		if err := content.Close(); err != nil {
			log.Printf("Error closing attachment content: %v", err)
		}
	}()

	// Sanitize filename for header
	safeFilename := strings.ReplaceAll(attachment.Filename, "\"", "")
	safeFilename = strings.ReplaceAll(safeFilename, "\n", "")
	safeFilename = strings.ReplaceAll(safeFilename, "\r", "")

	// Set headers for file download
	w.Header().Set("Content-Type", attachment.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", safeFilename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", attachment.Size))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Stream the file content
	if _, err := io.Copy(w, content); err != nil {
		log.Printf("Error streaming attachment: %v", err)
	}
}

// deleteAttachmentHandler deletes an attachment
func (s *Server) deleteAttachmentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attachmentID := vars["id"]

	ctx := context.Background()

	// Get attachment to find issue ID
	attachment, err := s.attachmentService.GetAttachment(ctx, attachmentID)
	if err != nil {
		s.errorResponse(w, "Attachment not found", http.StatusNotFound)
		return
	}

	issueID := attachment.IssueID

	// Delete the attachment
	if err := s.attachmentService.DeleteAttachment(ctx, attachmentID); err != nil {
		s.errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update memory storage
	issue, _ := s.issueService.GetIssue(ctx, issueID)
	if issue != nil {
		s.memoryStorage.Update(issue)
	}

	response := APIResponse{
		Success: true,
		Data:    map[string]string{"message": "Attachment deleted successfully"},
	}
	s.jsonResponse(w, response, http.StatusOK)
}
