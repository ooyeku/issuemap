package storage

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// FileGlobalRepository implements the GlobalRepository interface using file storage
type FileGlobalRepository struct {
	globalPath string
}

// NewFileGlobalRepository creates a new file-based global repository
func NewFileGlobalRepository() *FileGlobalRepository {
	return &FileGlobalRepository{
		globalPath: entities.GetGlobalDir(),
	}
}

// InitializeGlobal initializes the global directory structure
func (r *FileGlobalRepository) InitializeGlobal(ctx context.Context) error {
	// Create main directory
	if err := os.MkdirAll(r.globalPath, 0755); err != nil {
		return errors.Wrap(err, "FileGlobalRepository.InitializeGlobal", "create_global_dir")
	}

	// Create subdirectories
	dirs := []string{
		entities.GetArchivePath(),
		entities.GetBackupPath(),
		filepath.Join(r.globalPath, "projects"),
		filepath.Join(r.globalPath, "temp"),
		filepath.Join(r.globalPath, "logs"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrap(err, "FileGlobalRepository.InitializeGlobal", "create_subdir")
		}
	}

	// Create default config if it doesn't exist
	configPath := entities.GetConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := entities.NewGlobalConfig()
		if err := r.SaveConfig(ctx, config); err != nil {
			return errors.Wrap(err, "FileGlobalRepository.InitializeGlobal", "save_config")
		}
	}

	// Create .gitignore for temp and logs
	gitignoreContent := "# Temporary files and logs\ntemp/\nlogs/\n*.log\n*.tmp\n"
	gitignorePath := filepath.Join(r.globalPath, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
			return errors.Wrap(err, "FileGlobalRepository.InitializeGlobal", "create_gitignore")
		}
	}

	return nil
}

// GetConfig retrieves the global configuration
func (r *FileGlobalRepository) GetConfig(ctx context.Context) (*entities.GlobalConfig, error) {
	configPath := entities.GetConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return entities.NewGlobalConfig(), nil
		}
		return nil, errors.Wrap(err, "FileGlobalRepository.GetConfig", "read_file")
	}

	var config entities.GlobalConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.GetConfig", "unmarshal")
	}

	return &config, nil
}

// SaveConfig saves the global configuration
func (r *FileGlobalRepository) SaveConfig(ctx context.Context, config *entities.GlobalConfig) error {
	config.UpdatedAt = time.Now()

	data, err := yaml.Marshal(config)
	if err != nil {
		return errors.Wrap(err, "FileGlobalRepository.SaveConfig", "marshal")
	}

	configPath := entities.GetConfigPath()
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return errors.Wrap(err, "FileGlobalRepository.SaveConfig", "write_file")
	}

	return nil
}

// RegisterProject registers a new project for global tracking
func (r *FileGlobalRepository) RegisterProject(ctx context.Context, path string, name string) (*entities.ProjectInfo, error) {
	config, err := r.GetConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.RegisterProject", "get_config")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.RegisterProject", "abs_path")
	}

	// Check if project already exists
	if project, exists := config.GetProject(absPath); exists {
		project.LastScan = time.Now()
		project.Status = entities.ProjectStatusActive
		if err := r.SaveConfig(ctx, config); err != nil {
			return nil, errors.Wrap(err, "FileGlobalRepository.RegisterProject", "save_existing")
		}
		return project, nil
	}

	// Add new project
	project := config.AddProject(absPath, name)

	// Try to detect git remote
	if gitRemote := r.detectGitRemote(absPath); gitRemote != "" {
		project.GitRemote = gitRemote
	}

	// Update issue count
	if stats := r.scanProjectStats(absPath); stats != nil {
		project.IssueCount = stats.IssueCount
	}

	if err := r.SaveConfig(ctx, config); err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.RegisterProject", "save_config")
	}

	return project, nil
}

// UnregisterProject removes a project from global tracking
func (r *FileGlobalRepository) UnregisterProject(ctx context.Context, path string) error {
	config, err := r.GetConfig(ctx)
	if err != nil {
		return errors.Wrap(err, "FileGlobalRepository.UnregisterProject", "get_config")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return errors.Wrap(err, "FileGlobalRepository.UnregisterProject", "abs_path")
	}

	if !config.RemoveProject(absPath) {
		return errors.Wrap(errors.ErrIssueNotFound, "FileGlobalRepository.UnregisterProject", "project_not_found")
	}

	if err := r.SaveConfig(ctx, config); err != nil {
		return errors.Wrap(err, "FileGlobalRepository.UnregisterProject", "save_config")
	}

	return nil
}

// GetProject retrieves a specific project
func (r *FileGlobalRepository) GetProject(ctx context.Context, path string) (*entities.ProjectInfo, error) {
	config, err := r.GetConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.GetProject", "get_config")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.GetProject", "abs_path")
	}

	project, exists := config.GetProject(absPath)
	if !exists {
		return nil, errors.Wrap(errors.ErrIssueNotFound, "FileGlobalRepository.GetProject", "not_found")
	}

	return project, nil
}

// ListProjects retrieves all projects matching the filter
func (r *FileGlobalRepository) ListProjects(ctx context.Context, filter repositories.ProjectFilter) ([]*entities.ProjectInfo, error) {
	config, err := r.GetConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.ListProjects", "get_config")
	}

	var projects []*entities.ProjectInfo
	for _, project := range config.Projects {
		if r.matchesProjectFilter(project, filter) {
			projects = append(projects, project)
		}
	}

	// Sort by last scan time (most recent first)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastScan.After(projects[j].LastScan)
	})

	return projects, nil
}

// UpdateProjectStats updates statistics for a project
func (r *FileGlobalRepository) UpdateProjectStats(ctx context.Context, path string, stats *repositories.ProjectStats) error {
	config, err := r.GetConfig(ctx)
	if err != nil {
		return errors.Wrap(err, "FileGlobalRepository.UpdateProjectStats", "get_config")
	}

	updated := config.UpdateProject(path, func(project *entities.ProjectInfo) {
		project.IssueCount = stats.IssueCount
		project.ArchivedCount = stats.ArchivedCount
		project.LastScan = stats.LastScan
	})

	if !updated {
		return errors.Wrap(errors.ErrIssueNotFound, "FileGlobalRepository.UpdateProjectStats", "project_not_found")
	}

	if err := r.SaveConfig(ctx, config); err != nil {
		return errors.Wrap(err, "FileGlobalRepository.UpdateProjectStats", "save_config")
	}

	return nil
}

// ScanForProjects scans specified paths for issuemap projects
func (r *FileGlobalRepository) ScanForProjects(ctx context.Context, rootPaths []string) ([]*entities.ProjectInfo, error) {
	var foundProjects []*entities.ProjectInfo

	for _, rootPath := range rootPaths {
		projects, err := r.scanDirectory(ctx, rootPath)
		if err != nil {
			continue // Skip directories that can't be scanned
		}
		foundProjects = append(foundProjects, projects...)
	}

	return foundProjects, nil
}

// ArchiveIssue archives an issue to global storage
func (r *FileGlobalRepository) ArchiveIssue(ctx context.Context, issue *entities.Issue, projectPath string, reason string) (*entities.ArchivedIssue, error) {
	config, err := r.GetConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.ArchiveIssue", "get_config")
	}

	project, exists := config.GetProject(projectPath)
	if !exists {
		return nil, errors.Wrap(errors.ErrIssueNotFound, "FileGlobalRepository.ArchiveIssue", "project_not_found")
	}

	// Create archived issue
	archivedIssue := entities.NewArchivedIssue(issue, projectPath, project.Name)
	archivedIssue.ArchiveReason = reason

	// Create archive directory structure
	archivePath := filepath.Join(entities.GetArchivePath(), project.Name)
	if err := os.MkdirAll(archivePath, 0755); err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.ArchiveIssue", "create_archive_dir")
	}

	// Save archived issue
	archiveFile := filepath.Join(archivePath, fmt.Sprintf("%s.yaml", issue.ID))
	data, err := yaml.Marshal(archivedIssue)
	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.ArchiveIssue", "marshal")
	}

	if err := os.WriteFile(archiveFile, data, 0644); err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.ArchiveIssue", "write_archive")
	}

	// Update project stats
	config.UpdateProject(projectPath, func(p *entities.ProjectInfo) {
		p.ArchivedCount++
		if p.IssueCount > 0 {
			p.IssueCount--
		}
	})

	if err := r.SaveConfig(ctx, config); err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.ArchiveIssue", "save_config")
	}

	return archivedIssue, nil
}

// GetArchivedIssue retrieves a specific archived issue
func (r *FileGlobalRepository) GetArchivedIssue(ctx context.Context, issueID entities.IssueID, projectPath string) (*entities.ArchivedIssue, error) {
	config, err := r.GetConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.GetArchivedIssue", "get_config")
	}

	project, exists := config.GetProject(projectPath)
	if !exists {
		return nil, errors.Wrap(errors.ErrIssueNotFound, "FileGlobalRepository.GetArchivedIssue", "project_not_found")
	}

	archiveFile := filepath.Join(entities.GetArchivePath(), project.Name, fmt.Sprintf("%s.yaml", issueID))

	data, err := os.ReadFile(archiveFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrap(errors.ErrIssueNotFound, "FileGlobalRepository.GetArchivedIssue", "not_found")
		}
		return nil, errors.Wrap(err, "FileGlobalRepository.GetArchivedIssue", "read_file")
	}

	var archivedIssue entities.ArchivedIssue
	if err := yaml.Unmarshal(data, &archivedIssue); err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.GetArchivedIssue", "unmarshal")
	}

	return &archivedIssue, nil
}

// ListArchivedIssues retrieves archived issues matching the filter
func (r *FileGlobalRepository) ListArchivedIssues(ctx context.Context, filter repositories.ArchiveFilter) ([]*entities.ArchivedIssue, error) {
	archiveDir := entities.GetArchivePath()

	var archivedIssues []*entities.ArchivedIssue

	err := filepath.Walk(archiveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files that can't be read
		}

		var archivedIssue entities.ArchivedIssue
		if err := yaml.Unmarshal(data, &archivedIssue); err != nil {
			return nil // Skip files that can't be parsed
		}

		if r.matchesArchiveFilter(&archivedIssue, filter) {
			archivedIssues = append(archivedIssues, &archivedIssue)
		}

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.ListArchivedIssues", "walk")
	}

	// Sort by archive date (most recent first)
	sort.Slice(archivedIssues, func(i, j int) bool {
		return archivedIssues[i].ArchivedAt.After(archivedIssues[j].ArchivedAt)
	})

	// Apply pagination
	start := 0
	end := len(archivedIssues)

	if filter.Offset != nil {
		start = *filter.Offset
		if start > end {
			start = end
		}
	}

	if filter.Limit != nil {
		end = start + *filter.Limit
		if end > len(archivedIssues) {
			end = len(archivedIssues)
		}
	}

	if start > end {
		start = end
	}

	return archivedIssues[start:end], nil
}

// RestoreArchivedIssue restores an archived issue back to a project
func (r *FileGlobalRepository) RestoreArchivedIssue(ctx context.Context, archivedIssue *entities.ArchivedIssue, targetPath string) error {
	// This would need to integrate with the local issue repository
	// For now, we'll implement the basic structure
	targetIssuesDir := filepath.Join(targetPath, ".issuemap", "issues")
	if err := os.MkdirAll(targetIssuesDir, 0755); err != nil {
		return errors.Wrap(err, "FileGlobalRepository.RestoreArchivedIssue", "create_target_dir")
	}

	// Write the issue back to the local project
	targetFile := filepath.Join(targetIssuesDir, fmt.Sprintf("%s.yaml", archivedIssue.Issue.ID))
	data, err := yaml.Marshal(archivedIssue.Issue)
	if err != nil {
		return errors.Wrap(err, "FileGlobalRepository.RestoreArchivedIssue", "marshal")
	}

	if err := os.WriteFile(targetFile, data, 0644); err != nil {
		return errors.Wrap(err, "FileGlobalRepository.RestoreArchivedIssue", "write_target")
	}

	// Remove from archive
	return r.DeleteArchivedIssue(ctx, archivedIssue.Issue.ID, archivedIssue.ProjectPath)
}

// DeleteArchivedIssue removes an archived issue
func (r *FileGlobalRepository) DeleteArchivedIssue(ctx context.Context, issueID entities.IssueID, projectPath string) error {
	config, err := r.GetConfig(ctx)
	if err != nil {
		return errors.Wrap(err, "FileGlobalRepository.DeleteArchivedIssue", "get_config")
	}

	project, exists := config.GetProject(projectPath)
	if !exists {
		return errors.Wrap(errors.ErrIssueNotFound, "FileGlobalRepository.DeleteArchivedIssue", "project_not_found")
	}

	archiveFile := filepath.Join(entities.GetArchivePath(), project.Name, fmt.Sprintf("%s.yaml", issueID))

	if err := os.Remove(archiveFile); err != nil {
		if os.IsNotExist(err) {
			return errors.Wrap(errors.ErrIssueNotFound, "FileGlobalRepository.DeleteArchivedIssue", "not_found")
		}
		return errors.Wrap(err, "FileGlobalRepository.DeleteArchivedIssue", "remove_file")
	}

	return nil
}

// CreateBackup creates a complete backup of a project
func (r *FileGlobalRepository) CreateBackup(ctx context.Context, projectPath string, metadata *entities.BackupMetadata) (*entities.ProjectBackup, error) {
	config, err := r.GetConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.CreateBackup", "get_config")
	}

	project, exists := config.GetProject(projectPath)
	if !exists {
		return nil, errors.Wrap(errors.ErrIssueNotFound, "FileGlobalRepository.CreateBackup", "project_not_found")
	}

	// Generate backup ID
	backupID := fmt.Sprintf("%s-%d", project.Name, time.Now().Unix())

	// Create backup directory
	backupDir := filepath.Join(entities.GetBackupPath(), backupID)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.CreateBackup", "create_backup_dir")
	}

	// Create backup archive
	backupFile := filepath.Join(backupDir, "backup.tar.gz")
	if err := r.createBackupArchive(projectPath, backupFile); err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.CreateBackup", "create_archive")
	}

	// Get file size and checksum
	stat, err := os.Stat(backupFile)
	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.CreateBackup", "stat_backup")
	}

	checksum, err := r.calculateChecksum(backupFile)
	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.CreateBackup", "calculate_checksum")
	}

	// Enhance metadata with system information
	if metadata == nil {
		metadata = &entities.BackupMetadata{}
	}
	metadata.IssuemapVersion = app.GetVersion()
	metadata.OperatingSystem = runtime.GOOS
	metadata.Architecture = runtime.GOARCH

	// Create backup record
	backup := &entities.ProjectBackup{
		ID:          backupID,
		ProjectPath: projectPath,
		ProjectName: project.Name,
		BackupPath:  backupFile,
		CreatedAt:   time.Now(),
		Size:        stat.Size(),
		Checksum:    checksum,
		Metadata:    *metadata,
	}

	// Count issues
	if stats := r.scanProjectStats(projectPath); stats != nil {
		backup.IssueCount = stats.IssueCount
	}

	// Save backup metadata
	metadataFile := filepath.Join(backupDir, "metadata.yaml")
	data, err := yaml.Marshal(backup)
	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.CreateBackup", "marshal_metadata")
	}

	if err := os.WriteFile(metadataFile, data, 0644); err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.CreateBackup", "write_metadata")
	}

	// Update project's last backup time
	config.UpdateProject(projectPath, func(p *entities.ProjectInfo) {
		now := time.Now()
		p.LastBackup = &now
	})

	if err := r.SaveConfig(ctx, config); err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.CreateBackup", "save_config")
	}

	return backup, nil
}

// Helper functions

func (r *FileGlobalRepository) detectGitRemote(projectPath string) string {
	gitDir := filepath.Join(projectPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return ""
	}

	configFile := filepath.Join(gitDir, "config")
	data, err := os.ReadFile(configFile)
	if err != nil {
		return ""
	}

	// Simple parsing to extract remote URL
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "url =") {
			parts := strings.Split(line, "=")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}

func (r *FileGlobalRepository) scanProjectStats(projectPath string) *repositories.ProjectStats {
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

func (r *FileGlobalRepository) scanDirectory(ctx context.Context, rootPath string) ([]*entities.ProjectInfo, error) {
	var projects []*entities.ProjectInfo

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip directories with errors
		}

		if !info.IsDir() {
			return nil
		}

		// Check if this directory contains an .issuemap folder
		issuemapDir := filepath.Join(path, ".issuemap")
		if stat, err := os.Stat(issuemapDir); err == nil && stat.IsDir() {
			// Found a project, register it
			projectName := filepath.Base(path)
			if project, err := r.RegisterProject(ctx, path, projectName); err == nil {
				projects = append(projects, project)
			}
		}

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "FileGlobalRepository.scanDirectory", "walk")
	}

	return projects, nil
}

func (r *FileGlobalRepository) matchesProjectFilter(project *entities.ProjectInfo, filter repositories.ProjectFilter) bool {
	if filter.Status != nil && project.Status != *filter.Status {
		return false
	}

	if filter.Name != nil && !strings.Contains(strings.ToLower(project.Name), strings.ToLower(*filter.Name)) {
		return false
	}

	if filter.LastScanDays != nil {
		cutoff := time.Now().AddDate(0, 0, -*filter.LastScanDays)
		if project.LastScan.Before(cutoff) {
			return false
		}
	}

	if filter.HasBackup != nil && *filter.HasBackup && project.LastBackup == nil {
		return false
	}

	if len(filter.Tags) > 0 {
		projectTags := make(map[string]bool)
		for _, tag := range project.Tags {
			projectTags[tag] = true
		}

		for _, requiredTag := range filter.Tags {
			if !projectTags[requiredTag] {
				return false
			}
		}
	}

	return true
}

func (r *FileGlobalRepository) matchesArchiveFilter(archived *entities.ArchivedIssue, filter repositories.ArchiveFilter) bool {
	if filter.ProjectPath != nil && archived.ProjectPath != *filter.ProjectPath {
		return false
	}

	if filter.ArchivedSince != nil && archived.ArchivedAt.Before(*filter.ArchivedSince) {
		return false
	}

	if filter.ArchivedBy != nil && archived.ArchivedBy != *filter.ArchivedBy {
		return false
	}

	if filter.IssueType != nil && archived.Issue.Type != *filter.IssueType {
		return false
	}

	if filter.Status != nil && archived.Issue.Status != *filter.Status {
		return false
	}

	if filter.Priority != nil && archived.Issue.Priority != *filter.Priority {
		return false
	}

	return true
}

func (r *FileGlobalRepository) createBackupArchive(projectPath string, outputPath string) error {
	sourceDir := filepath.Join(projectPath, ".issuemap")

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Set name relative to source directory
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// Write file content if it's a regular file
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			if _, err := tarWriter.Write(data); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *FileGlobalRepository) calculateChecksum(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	hash := md5.Sum(data)
	return fmt.Sprintf("%x", hash), nil
}

// Placeholder implementations for remaining interface methods
// These would need full implementation based on specific requirements

func (r *FileGlobalRepository) ListBackups(ctx context.Context, filter repositories.BackupFilter) ([]*entities.ProjectBackup, error) {
	// Implementation would scan backup directory and filter results
	return nil, nil
}

func (r *FileGlobalRepository) GetBackup(ctx context.Context, backupID string) (*entities.ProjectBackup, error) {
	// Implementation would load backup metadata by ID
	return nil, nil
}

func (r *FileGlobalRepository) RestoreBackup(ctx context.Context, backupID string, targetPath string) error {
	// Implementation would extract backup archive to target path
	return nil
}

func (r *FileGlobalRepository) DeleteBackup(ctx context.Context, backupID string) error {
	// Implementation would remove backup directory
	return nil
}

func (r *FileGlobalRepository) GlobalListIssues(ctx context.Context, filter repositories.GlobalIssueFilter) (*repositories.GlobalIssueList, error) {
	// Implementation would aggregate issues from all projects
	return nil, nil
}

func (r *FileGlobalRepository) GlobalSearchIssues(ctx context.Context, query repositories.GlobalSearchQuery) (*repositories.GlobalSearchResult, error) {
	// Implementation would search across all projects
	return nil, nil
}

func (r *FileGlobalRepository) GetGlobalStats(ctx context.Context) (*repositories.GlobalStats, error) {
	// Implementation would calculate comprehensive statistics
	return nil, nil
}

func (r *FileGlobalRepository) CleanupOrphanedArchives(ctx context.Context) error {
	// Implementation would remove archives for deleted projects
	return nil
}

func (r *FileGlobalRepository) ValidateIntegrity(ctx context.Context) (*repositories.IntegrityReport, error) {
	// Implementation would validate all data integrity
	return nil, nil
}

func (r *FileGlobalRepository) ExportData(ctx context.Context, format string, outputPath string) error {
	// Implementation would export all data to specified format
	return nil
}

func (r *FileGlobalRepository) ImportData(ctx context.Context, inputPath string) error {
	// Implementation would import data from file
	return nil
}
