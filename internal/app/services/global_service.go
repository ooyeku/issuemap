package services

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

// GlobalService handles global issuemap operations
type GlobalService struct {
	globalRepo repositories.GlobalRepository
	issueRepo  repositories.IssueRepository
}

// NewGlobalService creates a new global service
func NewGlobalService() *GlobalService {
	globalRepo := storage.NewFileGlobalRepository()
	return &GlobalService{
		globalRepo: globalRepo,
	}
}

// SetIssueRepository sets the local issue repository (used for current project operations)
func (s *GlobalService) SetIssueRepository(repo repositories.IssueRepository) {
	s.issueRepo = repo
}

// InitializeGlobal initializes the global issuemap directory
func (s *GlobalService) InitializeGlobal(ctx context.Context) error {
	if err := s.globalRepo.InitializeGlobal(ctx); err != nil {
		return errors.Wrap(err, "GlobalService.InitializeGlobal", "init")
	}

	// Auto-discover projects if enabled
	config, err := s.globalRepo.GetConfig(ctx)
	if err == nil && config.GlobalSettings.AutoDiscovery {
		// Discover projects in common development directories
		commonPaths := s.getCommonDevPaths()
		if _, err := s.globalRepo.ScanForProjects(ctx, commonPaths); err != nil {
			// Log but don't fail on discovery errors
		}
	}

	return nil
}

// EnsureGlobalInitialized ensures the global directory is initialized
func (s *GlobalService) EnsureGlobalInitialized(ctx context.Context) error {
	globalDir := entities.GetGlobalDir()
	if _, err := os.Stat(globalDir); os.IsNotExist(err) {
		return s.InitializeGlobal(ctx)
	}
	return nil
}

// RegisterCurrentProject registers the current project directory
func (s *GlobalService) RegisterCurrentProject(ctx context.Context) (*entities.ProjectInfo, error) {
	if err := s.EnsureGlobalInitialized(ctx); err != nil {
		return nil, errors.Wrap(err, "GlobalService.RegisterCurrentProject", "ensure_init")
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "GlobalService.RegisterCurrentProject", "get_cwd")
	}

	// Check if current directory has .issuemap
	issuemapDir := filepath.Join(currentDir, ".issuemap")
	if _, err := os.Stat(issuemapDir); os.IsNotExist(err) {
		return nil, errors.Wrap(errors.New("GlobalService.RegisterCurrentProject", "not_issuemap_project", fmt.Errorf("current directory is not an issuemap project")), "GlobalService.RegisterCurrentProject", "not_issuemap_project")
	}

	projectName := filepath.Base(currentDir)
	return s.globalRepo.RegisterProject(ctx, currentDir, projectName)
}

// ListProjects lists all registered projects
func (s *GlobalService) ListProjects(ctx context.Context, status *entities.ProjectStatus) ([]*entities.ProjectInfo, error) {
	if err := s.EnsureGlobalInitialized(ctx); err != nil {
		return nil, errors.Wrap(err, "GlobalService.ListProjects", "ensure_init")
	}

	filter := repositories.ProjectFilter{}
	if status != nil {
		filter.Status = status
	}

	projects, err := s.globalRepo.ListProjects(ctx, filter)
	if err != nil {
		return nil, errors.Wrap(err, "GlobalService.ListProjects", "list")
	}

	// Update project stats if they're stale
	for _, project := range projects {
		if time.Since(project.LastScan).Hours() > 1 { // Update if more than 1 hour old
			if stats := s.scanProjectStats(project.Path); stats != nil {
				s.globalRepo.UpdateProjectStats(ctx, project.Path, stats)
			}
		}
	}

	return projects, nil
}

// GlobalListIssues lists issues across all projects
func (s *GlobalService) GlobalListIssues(ctx context.Context, includeArchived bool) ([]repositories.GlobalIssue, error) {
	if err := s.EnsureGlobalInitialized(ctx); err != nil {
		return nil, errors.Wrap(err, "GlobalService.GlobalListIssues", "ensure_init")
	}

	projects, err := s.ListProjects(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "GlobalService.GlobalListIssues", "list_projects")
	}

	var allIssues []repositories.GlobalIssue

	for _, project := range projects {
		if project.Status != entities.ProjectStatusActive {
			continue
		}

		// Load issues from project
		issueRepo := storage.NewFileIssueRepository(filepath.Join(project.Path, ".issuemap"))
		issueList, err := issueRepo.List(ctx, repositories.IssueFilter{})
		if err != nil {
			continue // Skip projects with errors
		}

		for _, issue := range issueList.Issues {
			globalIssue := repositories.GlobalIssue{
				Issue:       &issue,
				ProjectPath: project.Path,
				ProjectName: project.Name,
				IsArchived:  false,
			}
			allIssues = append(allIssues, globalIssue)
		}
	}

	// Include archived issues if requested
	if includeArchived {
		archivedIssues, err := s.globalRepo.ListArchivedIssues(ctx, repositories.ArchiveFilter{})
		if err == nil {
			for _, archived := range archivedIssues {
				globalIssue := repositories.GlobalIssue{
					Issue:       archived.Issue,
					ProjectPath: archived.ProjectPath,
					ProjectName: archived.ProjectName,
					IsArchived:  true,
				}
				allIssues = append(allIssues, globalIssue)
			}
		}
	}

	return allIssues, nil
}

// ArchiveIssue archives an issue from the current project
func (s *GlobalService) ArchiveIssue(ctx context.Context, issueID entities.IssueID, reason string) (*entities.ArchivedIssue, error) {
	if err := s.EnsureGlobalInitialized(ctx); err != nil {
		return nil, errors.Wrap(err, "GlobalService.ArchiveIssue", "ensure_init")
	}

	if s.issueRepo == nil {
		return nil, errors.Wrap(errors.New("GlobalService.ArchiveIssue", "no_local_repo", fmt.Errorf("no local issue repository configured")), "GlobalService.ArchiveIssue", "no_local_repo")
	}

	// Get the issue from local repository
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return nil, errors.Wrap(err, "GlobalService.ArchiveIssue", "get_issue")
	}

	// Get current project path
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "GlobalService.ArchiveIssue", "get_cwd")
	}

	// Archive the issue
	archivedIssue, err := s.globalRepo.ArchiveIssue(ctx, issue, currentDir, reason)
	if err != nil {
		return nil, errors.Wrap(err, "GlobalService.ArchiveIssue", "archive")
	}

	// Delete from local repository
	if err := s.issueRepo.Delete(ctx, issueID); err != nil {
		// If local deletion fails, we should rollback the archive
		// For now, we'll just log the error
		return archivedIssue, errors.Wrap(err, "GlobalService.ArchiveIssue", "delete_local")
	}

	return archivedIssue, nil
}

// ListArchivedIssues lists archived issues for the current project
func (s *GlobalService) ListArchivedIssues(ctx context.Context, projectPath *string) ([]*entities.ArchivedIssue, error) {
	if err := s.EnsureGlobalInitialized(ctx); err != nil {
		return nil, errors.Wrap(err, "GlobalService.ListArchivedIssues", "ensure_init")
	}

	filter := repositories.ArchiveFilter{}

	if projectPath != nil {
		filter.ProjectPath = projectPath
	} else {
		// Default to current directory
		currentDir, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "GlobalService.ListArchivedIssues", "get_cwd")
		}
		filter.ProjectPath = &currentDir
	}

	return s.globalRepo.ListArchivedIssues(ctx, filter)
}

// BackupProject creates a complete backup of a project
func (s *GlobalService) BackupProject(ctx context.Context, projectPath string, tags []string) (*entities.ProjectBackup, error) {
	if err := s.EnsureGlobalInitialized(ctx); err != nil {
		return nil, errors.Wrap(err, "GlobalService.BackupProject", "ensure_init")
	}

	// If no project path specified, use current directory
	if projectPath == "" {
		var err error
		projectPath, err = os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "GlobalService.BackupProject", "get_cwd")
		}
	}

	// Ensure project is registered
	_, err := s.globalRepo.GetProject(ctx, projectPath)
	if err != nil {
		// Auto-register if not found
		projectName := filepath.Base(projectPath)
		_, err = s.globalRepo.RegisterProject(ctx, projectPath, projectName)
		if err != nil {
			return nil, errors.Wrap(err, "GlobalService.BackupProject", "register_project")
		}
	}

	// Gather metadata
	metadata := s.gatherBackupMetadata(ctx, projectPath)

	// Create backup
	backup, err := s.globalRepo.CreateBackup(ctx, projectPath, metadata)
	if err != nil {
		return nil, errors.Wrap(err, "GlobalService.BackupProject", "create_backup")
	}

	// Add tags if provided
	backup.Tags = tags

	return backup, nil
}

// ScanForProjects discovers projects in specified directories
func (s *GlobalService) ScanForProjects(ctx context.Context, paths []string) ([]*entities.ProjectInfo, error) {
	if err := s.EnsureGlobalInitialized(ctx); err != nil {
		return nil, errors.Wrap(err, "GlobalService.ScanForProjects", "ensure_init")
	}

	if len(paths) == 0 {
		paths = s.getCommonDevPaths()
	}

	return s.globalRepo.ScanForProjects(ctx, paths)
}

// GetGlobalConfig retrieves the global configuration
func (s *GlobalService) GetGlobalConfig(ctx context.Context) (*entities.GlobalConfig, error) {
	if err := s.EnsureGlobalInitialized(ctx); err != nil {
		return nil, errors.Wrap(err, "GlobalService.GetGlobalConfig", "ensure_init")
	}

	return s.globalRepo.GetConfig(ctx)
}

// UpdateGlobalConfig updates the global configuration
func (s *GlobalService) UpdateGlobalConfig(ctx context.Context, config *entities.GlobalConfig) error {
	return s.globalRepo.SaveConfig(ctx, config)
}

// Helper methods

func (s *GlobalService) getCommonDevPaths() []string {
	var paths []string

	// Get user home directory
	usr, err := user.Current()
	if err != nil {
		return paths
	}

	homeDir := usr.HomeDir

	// Common development directories
	commonDirs := []string{
		filepath.Join(homeDir, "Code"),
		filepath.Join(homeDir, "Development"),
		filepath.Join(homeDir, "Projects"),
		filepath.Join(homeDir, "Workspace"),
		filepath.Join(homeDir, "src"),
		filepath.Join(homeDir, "go", "src"),
		filepath.Join(homeDir, "Documents"),
	}

	// Add directories that exist
	for _, dir := range commonDirs {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			paths = append(paths, dir)
		}
	}

	return paths
}

func (s *GlobalService) scanProjectStats(projectPath string) *repositories.ProjectStats {
	issuesDir := filepath.Join(projectPath, ".issuemap", "issues")
	files, err := os.ReadDir(issuesDir)
	if err != nil {
		return nil
	}

	issueCount := 0
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".yaml") {
			issueCount++
		}
	}

	return &repositories.ProjectStats{
		IssueCount: issueCount,
		LastScan:   time.Now(),
	}
}

func (s *GlobalService) gatherBackupMetadata(ctx context.Context, projectPath string) *entities.BackupMetadata {
	metadata := &entities.BackupMetadata{
		OperatingSystem: runtime.GOOS,
		Architecture:    runtime.GOARCH,
	}

	// Get current user
	if usr, err := user.Current(); err == nil {
		metadata.CreatedByUser = usr.Username
	}

	// Get hostname
	if hostname, err := os.Hostname(); err == nil {
		metadata.CreatedByHost = hostname
	}

	// Try to load project config
	if configRepo := storage.NewFileConfigRepository(filepath.Join(projectPath, ".issuemap")); configRepo != nil {
		if config, err := configRepo.Load(ctx); err == nil {
			metadata.OriginalConfig = config
		}
	}

	// Gather issue summary
	if issueRepo := storage.NewFileIssueRepository(filepath.Join(projectPath, ".issuemap")); issueRepo != nil {
		if stats, err := issueRepo.GetStats(ctx); err == nil {
			metadata.IssuesSummary = entities.BackupIssueSummary{
				TotalIssues: stats.TotalIssues,
				ByStatus:    stats.IssuesByStatus,
				ByType:      stats.IssuesByType,
				ByPriority:  stats.IssuesByPriority,
			}

			// Set date range if we have issues
			if stats.OldestIssue != nil && stats.NewestIssue != nil {
				metadata.IssuesSummary.DateRange = &entities.DateRange{
					Earliest: stats.OldestIssue.Timestamps.Created,
					Latest:   stats.NewestIssue.Timestamps.Created,
				}
			}

			// Convert assignee stats
			for username, count := range stats.IssuesByAssignee {
				if username != "unassigned" {
					metadata.IssuesSummary.TopAssignees = append(
						metadata.IssuesSummary.TopAssignees,
						entities.AssigneeStat{Username: username, Count: count},
					)
				}
			}
		}
	}

	return metadata
}

// FormatGlobalPath formats a path for global display (relative to home if possible)
func (s *GlobalService) FormatGlobalPath(path string) string {
	if usr, err := user.Current(); err == nil {
		if rel, err := filepath.Rel(usr.HomeDir, path); err == nil && !strings.HasPrefix(rel, "..") {
			return "~/" + rel
		}
	}
	return path
}
