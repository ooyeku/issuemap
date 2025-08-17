package services

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// ArchiveService handles archiving and restoration of old issues
type ArchiveService struct {
	basePath       string
	archivePath    string
	issueRepo      repositories.IssueRepository
	configRepo     repositories.ConfigRepository
	attachmentRepo repositories.AttachmentRepository
	config         *entities.ArchiveConfig
	index          *entities.ArchiveIndex
	mu             sync.RWMutex
}

// NewArchiveService creates a new archive service
func NewArchiveService(
	basePath string,
	issueRepo repositories.IssueRepository,
	configRepo repositories.ConfigRepository,
	attachmentRepo repositories.AttachmentRepository,
) *ArchiveService {
	archivePath := filepath.Join(basePath, "archives")

	// Ensure archive directory exists
	os.MkdirAll(archivePath, 0755)

	service := &ArchiveService{
		basePath:       basePath,
		archivePath:    archivePath,
		issueRepo:      issueRepo,
		configRepo:     configRepo,
		attachmentRepo: attachmentRepo,
		config:         entities.DefaultArchiveConfig(),
	}

	// Try to load config from repository
	if configRepo != nil {
		if cfg, err := configRepo.Load(context.Background()); err == nil && cfg != nil {
			if cfg.ArchiveConfig != nil {
				service.config = cfg.ArchiveConfig
			}
		}
	}

	// Load existing index
	service.loadIndex()

	return service
}

// GetConfig returns current archive configuration
func (s *ArchiveService) GetConfig() *entities.ArchiveConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig updates archive configuration
func (s *ArchiveService) UpdateConfig(config *entities.ArchiveConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config

	// TODO: Save to config repository
	return nil
}

// ArchiveIssues archives issues based on filter criteria
func (s *ArchiveService) ArchiveIssues(ctx context.Context, filter *entities.ArchiveFilter, dryRun bool) (*entities.ArchiveResult, error) {
	start := time.Now()
	result := &entities.ArchiveResult{
		Timestamp: start,
		DryRun:    dryRun,
		Errors:    []string{},
	}

	// Find issues matching filter criteria
	issues, err := s.findIssuesForArchival(ctx, filter)
	if err != nil {
		return result, errors.Wrap(err, "ArchiveService.ArchiveIssues", "find_issues")
	}

	if len(issues) == 0 {
		result.Duration = time.Since(start)
		return result, nil
	}

	// Calculate original size
	originalSize, err := s.calculateIssuesSize(issues)
	if err != nil {
		return result, errors.Wrap(err, "ArchiveService.ArchiveIssues", "calculate_size")
	}

	result.OriginalSize = originalSize
	result.IssuesArchived = len(issues)

	// Collect issue IDs
	for _, issue := range issues {
		result.ArchivedIssues = append(result.ArchivedIssues, issue.ID)
	}

	if dryRun {
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create archive
	archiveFile := s.generateArchiveFilename()
	compressedSize, checksum, err := s.createArchive(ctx, archiveFile, issues)
	if err != nil {
		return result, errors.Wrap(err, "ArchiveService.ArchiveIssues", "create_archive")
	}

	result.ArchiveFile = archiveFile
	result.CompressedSize = compressedSize
	result.CompressionRatio = 1.0 - (float64(compressedSize) / float64(originalSize))

	// Update index
	if err := s.updateIndexWithArchivedIssues(issues, archiveFile, compressedSize, originalSize, checksum); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to update index: %v", err))
	}

	// Remove archived issues from active storage
	for _, issue := range issues {
		if err := s.removeIssueFromActiveStorage(ctx, issue); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to remove issue %s: %v", issue.ID, err))
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// RestoreIssue restores a single issue from archives
func (s *ArchiveService) RestoreIssue(ctx context.Context, issueID entities.IssueID, dryRun bool) (*entities.ArchiveRestoreResult, error) {
	return s.RestoreIssues(ctx, []entities.IssueID{issueID}, dryRun)
}

// RestoreIssues restores multiple issues from archives
func (s *ArchiveService) RestoreIssues(ctx context.Context, issueIDs []entities.IssueID, dryRun bool) (*entities.ArchiveRestoreResult, error) {
	start := time.Now()
	result := &entities.ArchiveRestoreResult{
		Timestamp: start,
		DryRun:    dryRun,
		Errors:    []string{},
	}

	s.mu.RLock()
	index := s.index
	s.mu.RUnlock()

	if index == nil {
		return result, errors.New("ArchiveService.RestoreIssues", "no_index", fmt.Errorf("archive index not loaded"))
	}

	// Group issues by archive file
	archiveGroups := make(map[string][]entities.IssueID)
	for _, issueID := range issueIDs {
		if entry, exists := index.Issues[issueID]; exists {
			archiveGroups[entry.ArchiveFile] = append(archiveGroups[entry.ArchiveFile], issueID)
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("issue %s not found in archives", issueID))
		}
	}

	if dryRun {
		result.IssuesRestored = len(issueIDs) - len(result.Errors)
		result.Duration = time.Since(start)
		return result, nil
	}

	// Restore issues from each archive
	for archiveFile, groupIssueIDs := range archiveGroups {
		issues, err := s.extractIssuesFromArchive(ctx, archiveFile, groupIssueIDs)
		if err != nil {
			for _, id := range groupIssueIDs {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to extract issue %s: %v", id, err))
			}
			continue
		}

		// Restore each issue to active storage
		for _, issue := range issues {
			if err := s.restoreIssueToActiveStorage(ctx, issue); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to restore issue %s: %v", issue.ID, err))
				continue
			}

			result.RestoredIssues = append(result.RestoredIssues, issue.ID)
			result.IssuesRestored++

			// Remove from index
			delete(index.Issues, issue.ID)
		}
	}

	// Update index
	if err := s.saveIndex(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to update index: %v", err))
	}

	result.Duration = time.Since(start)
	return result, nil
}

// SearchArchives searches for issues in archives
func (s *ArchiveService) SearchArchives(ctx context.Context, query string) ([]*entities.SearchResult, error) {
	s.mu.RLock()
	index := s.index
	s.mu.RUnlock()

	if index == nil {
		return nil, errors.New("ArchiveService.SearchArchives", "no_index", fmt.Errorf("archive index not loaded"))
	}

	var results []*entities.SearchResult
	query = strings.ToLower(query)

	for _, entry := range index.Issues {
		score := 0.0
		var matchedFields []string

		// Search in title
		if strings.Contains(strings.ToLower(entry.Title), query) {
			score += 1.0
			matchedFields = append(matchedFields, "title")
		}

		// Search in issue ID
		if strings.Contains(strings.ToLower(string(entry.IssueID)), query) {
			score += 0.8
			matchedFields = append(matchedFields, "id")
		}

		// Search in type/status
		if strings.Contains(strings.ToLower(string(entry.Type)), query) ||
			strings.Contains(strings.ToLower(string(entry.Status)), query) {
			score += 0.5
			matchedFields = append(matchedFields, "metadata")
		}

		if score > 0 {
			results = append(results, &entities.SearchResult{
				Entry:         entry,
				ArchiveFile:   entry.ArchiveFile,
				Score:         score,
				MatchedFields: matchedFields,
			})
		}
	}

	// Sort by score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

// VerifyArchive verifies the integrity of an archive
func (s *ArchiveService) VerifyArchive(ctx context.Context, archiveFile string) error {
	s.mu.RLock()
	index := s.index
	s.mu.RUnlock()

	if index == nil {
		return errors.New("ArchiveService.VerifyArchive", "no_index", fmt.Errorf("archive index not loaded"))
	}

	// Get expected issues in this archive
	expectedIssues, exists := index.Archives[archiveFile]
	if !exists {
		return errors.New("ArchiveService.VerifyArchive", "not_found", fmt.Errorf("archive not found in index"))
	}

	archivePath := filepath.Join(s.archivePath, archiveFile)

	// Check if archive file exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return errors.New("ArchiveService.VerifyArchive", "file_missing", fmt.Errorf("archive file not found"))
	}

	// Open and read archive
	file, err := os.Open(archivePath)
	if err != nil {
		return errors.Wrap(err, "ArchiveService.VerifyArchive", "open_archive")
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return errors.Wrap(err, "ArchiveService.VerifyArchive", "decompress")
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	// Track found issues
	foundIssues := make(map[entities.IssueID]bool)

	// Verify archive contents
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "ArchiveService.VerifyArchive", "read_entry")
		}

		// Check issue files
		if strings.HasPrefix(header.Name, "issues/") && strings.HasSuffix(header.Name, ".json") {
			issueID := strings.TrimSuffix(strings.TrimPrefix(header.Name, "issues/"), ".json")

			// Verify issue data
			data := make([]byte, header.Size)
			if _, err := io.ReadFull(tr, data); err != nil {
				return errors.Wrap(err, "ArchiveService.VerifyArchive", "read_issue_data")
			}

			var issue entities.Issue
			if err := json.Unmarshal(data, &issue); err != nil {
				return errors.Wrap(err, "ArchiveService.VerifyArchive", "parse_issue_data")
			}

			foundIssues[entities.IssueID(issueID)] = true
		}
	}

	// Verify all expected issues were found
	for _, expectedID := range expectedIssues {
		if !foundIssues[expectedID] {
			return errors.New("ArchiveService.VerifyArchive", "missing_issue",
				fmt.Errorf("issue %s not found in archive", expectedID))
		}
	}

	return nil
}

// ShouldAutoArchive checks if automatic archiving should run
func (s *ArchiveService) ShouldAutoArchive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config.Enabled
}

// RunAutoArchive performs automatic archiving based on configuration
func (s *ArchiveService) RunAutoArchive(ctx context.Context) (*entities.ArchiveResult, error) {
	if !s.ShouldAutoArchive() {
		return nil, nil
	}

	// Create filter for auto-archiving
	filter := &entities.ArchiveFilter{
		Status:     &[]entities.Status{entities.StatusClosed}[0],
		MinAgeDays: &s.config.ClosedIssueDays,
	}

	return s.ArchiveIssues(ctx, filter, false)
}

// GetArchiveStats returns statistics about archives
func (s *ArchiveService) GetArchiveStats() (*entities.ArchiveStats, error) {
	s.mu.RLock()
	index := s.index
	s.mu.RUnlock()

	if index == nil {
		return nil, errors.New("ArchiveService.GetArchiveStats", "no_index", fmt.Errorf("archive index not loaded"))
	}

	stats := &entities.ArchiveStats{
		TotalArchives:       len(index.Archives),
		TotalArchivedIssues: index.TotalIssues,
		TotalCompressedSize: index.TotalCompressedSize,
		TotalOriginalSize:   index.TotalOriginalSize,
		ArchivesByPeriod:    make(map[string]int),
	}

	if stats.TotalOriginalSize > 0 {
		stats.CompressionRatio = 1.0 - (float64(stats.TotalCompressedSize) / float64(stats.TotalOriginalSize))
		stats.SpaceSaved = stats.TotalOriginalSize - stats.TotalCompressedSize
	}

	// Find oldest and newest issues
	for _, entry := range index.Issues {
		if stats.OldestIssue == nil || entry.CreatedAt.Before(*stats.OldestIssue) {
			stats.OldestIssue = &entry.CreatedAt
		}
		if stats.NewestIssue == nil || entry.ArchivedAt.After(*stats.NewestIssue) {
			stats.NewestIssue = &entry.ArchivedAt
		}

		// Group by month
		period := entry.ArchivedAt.Format("2006-01")
		stats.ArchivesByPeriod[period]++
	}

	return stats, nil
}

// ListArchives returns a list of all archive files
func (s *ArchiveService) ListArchives() ([]string, error) {
	s.mu.RLock()
	index := s.index
	s.mu.RUnlock()

	if index == nil {
		return nil, errors.New("ArchiveService.ListArchives", "no_index", fmt.Errorf("archive index not loaded"))
	}

	var archives []string
	for archiveFile := range index.Archives {
		archives = append(archives, archiveFile)
	}

	sort.Strings(archives)
	return archives, nil
}

// findIssuesForArchival finds issues matching filter criteria
func (s *ArchiveService) findIssuesForArchival(ctx context.Context, filter *entities.ArchiveFilter) ([]*entities.Issue, error) {
	// Build issue filter
	issueFilter := repositories.IssueFilter{}

	if filter.Status != nil {
		issueFilter.Status = filter.Status
	}
	if filter.Type != nil {
		issueFilter.Type = filter.Type
	}

	// Get all issues (we'll filter by date locally for more complex criteria)
	issueList, err := s.issueRepo.List(ctx, issueFilter)
	if err != nil {
		return nil, err
	}

	var filteredIssues []*entities.Issue
	now := time.Now()

	for _, issue := range issueList.Issues {
		// Skip if specific IDs provided and this isn't one of them
		if len(filter.IssueIDs) > 0 {
			found := false
			for _, id := range filter.IssueIDs {
				if issue.ID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Skip if excluded
		if len(filter.ExcludeIDs) > 0 {
			excluded := false
			for _, id := range filter.ExcludeIDs {
				if issue.ID == id {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
		}

		// Check age criteria
		if filter.MinAgeDays != nil {
			minDate := now.AddDate(0, 0, -*filter.MinAgeDays)
			if issue.Timestamps.Updated.After(minDate) {
				continue
			}
		}

		if filter.ClosedBefore != nil {
			if issue.Timestamps.Closed == nil || issue.Timestamps.Closed.After(*filter.ClosedBefore) {
				continue
			}
		}

		if filter.CreatedBefore != nil {
			if issue.Timestamps.Created.After(*filter.CreatedBefore) {
				continue
			}
		}

		// Make a copy to avoid modifying the original
		issueCopy := issue
		filteredIssues = append(filteredIssues, &issueCopy)
	}

	return filteredIssues, nil
}

// calculateIssuesSize calculates the total size of issues and their attachments
func (s *ArchiveService) calculateIssuesSize(issues []*entities.Issue) (int64, error) {
	var totalSize int64

	for _, issue := range issues {
		// Size of issue YAML file
		issueData, err := json.Marshal(issue)
		if err == nil {
			totalSize += int64(len(issueData))
		}

		// Size of attachments
		for _, attachment := range issue.Attachments {
			if s.config.IncludeAttachments {
				if info, err := os.Stat(filepath.Join(s.basePath, attachment.StoragePath)); err == nil {
					totalSize += info.Size()
				}
			}
		}
	}

	return totalSize, nil
}

// generateArchiveFilename creates a filename for the new archive
func (s *ArchiveService) generateArchiveFilename() string {
	timestamp := time.Now().Format("2006-01-02_150405")
	return fmt.Sprintf("archive_%s.tar.gz", timestamp)
}

// createArchive creates a compressed tar.gz archive with the given issues
func (s *ArchiveService) createArchive(ctx context.Context, filename string, issues []*entities.Issue) (int64, string, error) {
	archivePath := filepath.Join(s.archivePath, filename)

	// Create the archive file
	file, err := os.Create(archivePath)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()

	// Create gzip writer
	gzw := gzip.NewWriter(file)
	defer gzw.Close()

	// Create tar writer
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// Hash for integrity verification
	hasher := sha256.New()

	// Add each issue to the archive
	for _, issue := range issues {
		if err := s.addIssueToArchive(tw, hasher, issue); err != nil {
			os.Remove(archivePath) // Clean up on failure
			return 0, "", err
		}
	}

	// Get file size
	if info, err := file.Stat(); err == nil {
		checksum := hex.EncodeToString(hasher.Sum(nil))
		return info.Size(), checksum, nil
	}

	return 0, "", fmt.Errorf("failed to get archive file info")
}

// addIssueToArchive adds a single issue and its files to the tar archive
func (s *ArchiveService) addIssueToArchive(tw *tar.Writer, hasher io.Writer, issue *entities.Issue) error {
	// Add issue YAML file
	issueData, err := json.MarshalIndent(issue, "", "  ")
	if err != nil {
		return err
	}

	issueHeader := &tar.Header{
		Name: fmt.Sprintf("issues/%s.json", issue.ID),
		Mode: 0644,
		Size: int64(len(issueData)),
	}

	if err := tw.WriteHeader(issueHeader); err != nil {
		return err
	}

	if _, err := tw.Write(issueData); err != nil {
		return err
	}

	// Update hash
	hasher.Write(issueData)

	// Add attachments if configured
	if s.config.IncludeAttachments {
		for _, attachment := range issue.Attachments {
			if err := s.addAttachmentToArchive(tw, hasher, attachment); err != nil {
				return err
			}
		}
	}

	return nil
}

// addAttachmentToArchive adds an attachment file to the archive
func (s *ArchiveService) addAttachmentToArchive(tw *tar.Writer, hasher io.Writer, attachment entities.Attachment) error {
	attachmentPath := filepath.Join(s.basePath, attachment.StoragePath)

	file, err := os.Open(attachmentPath)
	if err != nil {
		return err // Skip missing attachments
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name: fmt.Sprintf("attachments/%s/%s", attachment.IssueID, attachment.Filename),
		Mode: 0644,
		Size: info.Size(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	// Copy file content and update hash
	teeReader := io.TeeReader(file, hasher)
	if _, err := io.Copy(tw, teeReader); err != nil {
		return err
	}

	return nil
}

// updateIndexWithArchivedIssues updates the archive index with newly archived issues
func (s *ArchiveService) updateIndexWithArchivedIssues(issues []*entities.Issue, archiveFile string, compressedSize, originalSize int64, checksum string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.index == nil {
		s.index = entities.NewArchiveIndex()
	}

	var archiveIssueIDs []entities.IssueID
	archivedBy := "system" // TODO: Get from context

	for _, issue := range issues {
		entry := &entities.ArchiveEntry{
			IssueID:        issue.ID,
			Title:          issue.Title,
			Type:           issue.Type,
			Status:         issue.Status,
			ArchiveFile:    archiveFile,
			CompressedSize: compressedSize / int64(len(issues)), // Approximate per issue
			OriginalSize:   originalSize / int64(len(issues)),   // Approximate per issue
			ArchivedAt:     time.Now(),
			ArchivedBy:     archivedBy,
			CreatedAt:      issue.Timestamps.Created,
			ClosedAt:       issue.Timestamps.Closed.UTC(),
			Checksum:       checksum,
		}

		// Collect file paths
		entry.Files = append(entry.Files, fmt.Sprintf("issues/%s.json", issue.ID))
		if s.config.IncludeAttachments {
			for _, attachment := range issue.Attachments {
				entry.Files = append(entry.Files, fmt.Sprintf("attachments/%s/%s", attachment.IssueID, attachment.Filename))
			}
		}

		s.index.Issues[issue.ID] = entry
		archiveIssueIDs = append(archiveIssueIDs, issue.ID)
	}

	// Update archive mapping
	s.index.Archives[archiveFile] = archiveIssueIDs

	// Update totals
	s.index.TotalIssues += len(issues)
	s.index.TotalCompressedSize += compressedSize
	s.index.TotalOriginalSize += originalSize
	s.index.LastUpdated = time.Now()

	return s.saveIndex()
}

// removeIssueFromActiveStorage removes an issue from active storage
func (s *ArchiveService) removeIssueFromActiveStorage(ctx context.Context, issue *entities.Issue) error {
	// Delete attachments
	for _, attachment := range issue.Attachments {
		attachmentPath := filepath.Join(s.basePath, attachment.StoragePath)
		os.Remove(attachmentPath) // Ignore errors for missing files
	}

	// Delete the issue
	return s.issueRepo.Delete(ctx, issue.ID)
}

// extractIssuesFromArchive extracts specific issues from an archive
func (s *ArchiveService) extractIssuesFromArchive(ctx context.Context, archiveFile string, issueIDs []entities.IssueID) ([]*entities.Issue, error) {
	archivePath := filepath.Join(s.archivePath, archiveFile)

	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	var issues []*entities.Issue
	issueIDSet := make(map[entities.IssueID]bool)
	for _, id := range issueIDs {
		issueIDSet[id] = true
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Check if this is an issue file we want
		if strings.HasPrefix(header.Name, "issues/") && strings.HasSuffix(header.Name, ".json") {
			issueID := strings.TrimSuffix(strings.TrimPrefix(header.Name, "issues/"), ".json")

			if issueIDSet[entities.IssueID(issueID)] {
				// Read issue data
				data := make([]byte, header.Size)
				if _, err := io.ReadFull(tr, data); err != nil {
					return nil, err
				}

				var issue entities.Issue
				if err := json.Unmarshal(data, &issue); err != nil {
					return nil, err
				}

				issues = append(issues, &issue)
			}
		}
	}

	return issues, nil
}

// restoreIssueToActiveStorage restores an issue to active storage
func (s *ArchiveService) restoreIssueToActiveStorage(ctx context.Context, issue *entities.Issue) error {
	return s.issueRepo.Create(ctx, issue)
}

// loadIndex loads the archive index from disk
func (s *ArchiveService) loadIndex() error {
	indexPath := filepath.Join(s.archivePath, "index.json")

	data, err := os.ReadFile(indexPath)
	if os.IsNotExist(err) {
		s.index = entities.NewArchiveIndex()
		return nil
	}
	if err != nil {
		return err
	}

	var index entities.ArchiveIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return err
	}

	s.index = &index
	return nil
}

// saveIndex saves the archive index to disk
func (s *ArchiveService) saveIndex() error {
	if s.index == nil {
		return nil
	}

	indexPath := filepath.Join(s.archivePath, "index.json")

	data, err := json.MarshalIndent(s.index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(indexPath, data, 0644)
}
