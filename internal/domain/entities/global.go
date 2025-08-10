package entities

import (
	"os"
	"path/filepath"
	"time"
)

// GlobalConfig represents the global issuemap configuration
type GlobalConfig struct {
	Version         string                  `yaml:"version" json:"version"`
	CreatedAt       time.Time               `yaml:"created_at" json:"created_at"`
	UpdatedAt       time.Time               `yaml:"updated_at" json:"updated_at"`
	Projects        map[string]*ProjectInfo `yaml:"projects" json:"projects"`
	GlobalSettings  GlobalSettings          `yaml:"global_settings" json:"global_settings"`
	ArchiveSettings ArchiveSettings         `yaml:"archive_settings" json:"archive_settings"`
}

// ProjectInfo contains metadata about tracked projects
type ProjectInfo struct {
	Path          string            `yaml:"path" json:"path"`
	Name          string            `yaml:"name" json:"name"`
	Description   string            `yaml:"description,omitempty" json:"description,omitempty"`
	GitRemote     string            `yaml:"git_remote,omitempty" json:"git_remote,omitempty"`
	LastScan      time.Time         `yaml:"last_scan" json:"last_scan"`
	LastBackup    *time.Time        `yaml:"last_backup,omitempty" json:"last_backup,omitempty"`
	IssueCount    int               `yaml:"issue_count" json:"issue_count"`
	ArchivedCount int               `yaml:"archived_count" json:"archived_count"`
	Status        ProjectStatus     `yaml:"status" json:"status"`
	Tags          []string          `yaml:"tags,omitempty" json:"tags,omitempty"`
	CustomFields  map[string]string `yaml:"custom_fields,omitempty" json:"custom_fields,omitempty"`
}

// ProjectStatus represents the status of a tracked project
type ProjectStatus string

const (
	ProjectStatusActive   ProjectStatus = "active"
	ProjectStatusInactive ProjectStatus = "inactive"
	ProjectStatusArchived ProjectStatus = "archived"
	ProjectStatusDeleted  ProjectStatus = "deleted"
)

// GlobalSettings contains global issuemap settings
type GlobalSettings struct {
	DefaultFormat        string               `yaml:"default_format" json:"default_format"`
	AutoDiscovery        bool                 `yaml:"auto_discovery" json:"auto_discovery"`
	ScanIntervalHours    int                  `yaml:"scan_interval_hours" json:"scan_interval_hours"`
	MaxBackupRetention   int                  `yaml:"max_backup_retention" json:"max_backup_retention"`
	CompressionEnabled   bool                 `yaml:"compression_enabled" json:"compression_enabled"`
	EncryptionEnabled    bool                 `yaml:"encryption_enabled" json:"encryption_enabled"`
	NotificationSettings NotificationSettings `yaml:"notification_settings" json:"notification_settings"`
}

// NotificationSettings contains notification preferences
type NotificationSettings struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	OnBackup          bool     `yaml:"on_backup" json:"on_backup"`
	OnArchive         bool     `yaml:"on_archive" json:"on_archive"`
	OnProjectDetected bool     `yaml:"on_project_detected" json:"on_project_detected"`
	Methods           []string `yaml:"methods" json:"methods"` // email, desktop, webhook
}

// ArchiveSettings contains archival policies
type ArchiveSettings struct {
	AutoArchiveAfterDays int      `yaml:"auto_archive_after_days" json:"auto_archive_after_days"`
	ArchiveClosedIssues  bool     `yaml:"archive_closed_issues" json:"archive_closed_issues"`
	KeepLocalCopy        bool     `yaml:"keep_local_copy" json:"keep_local_copy"`
	CompressionLevel     int      `yaml:"compression_level" json:"compression_level"`
	ExcludeStatuses      []Status `yaml:"exclude_statuses" json:"exclude_statuses"`
}

// ArchivedIssue represents an issue that has been archived
type ArchivedIssue struct {
	*Issue                   // Embed the original issue
	ProjectPath    string    `yaml:"project_path" json:"project_path"`
	ProjectName    string    `yaml:"project_name" json:"project_name"`
	ArchivedAt     time.Time `yaml:"archived_at" json:"archived_at"`
	ArchivedBy     string    `yaml:"archived_by,omitempty" json:"archived_by,omitempty"`
	ArchiveReason  string    `yaml:"archive_reason,omitempty" json:"archive_reason,omitempty"`
	OriginalPath   string    `yaml:"original_path" json:"original_path"`
	BackupChecksum string    `yaml:"backup_checksum,omitempty" json:"backup_checksum,omitempty"`
}

// ProjectBackup represents a complete backup of a project
type ProjectBackup struct {
	ID               string         `yaml:"id" json:"id"`
	ProjectPath      string         `yaml:"project_path" json:"project_path"`
	ProjectName      string         `yaml:"project_name" json:"project_name"`
	BackupPath       string         `yaml:"backup_path" json:"backup_path"`
	CreatedAt        time.Time      `yaml:"created_at" json:"created_at"`
	Size             int64          `yaml:"size" json:"size"`
	IssueCount       int            `yaml:"issue_count" json:"issue_count"`
	Checksum         string         `yaml:"checksum" json:"checksum"`
	GitCommit        string         `yaml:"git_commit,omitempty" json:"git_commit,omitempty"`
	GitBranch        string         `yaml:"git_branch,omitempty" json:"git_branch,omitempty"`
	GitRemote        string         `yaml:"git_remote,omitempty" json:"git_remote,omitempty"`
	Metadata         BackupMetadata `yaml:"metadata" json:"metadata"`
	CompressionRatio float64        `yaml:"compression_ratio,omitempty" json:"compression_ratio,omitempty"`
	Tags             []string       `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// BackupMetadata contains rich metadata about the backup
type BackupMetadata struct {
	IssuemapVersion string             `yaml:"issuemap_version" json:"issuemap_version"`
	GoVersion       string             `yaml:"go_version,omitempty" json:"go_version,omitempty"`
	OperatingSystem string             `yaml:"operating_system" json:"operating_system"`
	Architecture    string             `yaml:"architecture" json:"architecture"`
	CreatedByUser   string             `yaml:"created_by_user,omitempty" json:"created_by_user,omitempty"`
	CreatedByHost   string             `yaml:"created_by_host,omitempty" json:"created_by_host,omitempty"`
	OriginalConfig  *Config            `yaml:"original_config,omitempty" json:"original_config,omitempty"`
	IssuesSummary   BackupIssueSummary `yaml:"issues_summary" json:"issues_summary"`
	CustomFields    map[string]string  `yaml:"custom_fields,omitempty" json:"custom_fields,omitempty"`
}

// BackupIssueSummary contains statistics about backed up issues
type BackupIssueSummary struct {
	TotalIssues    int               `yaml:"total_issues" json:"total_issues"`
	ByStatus       map[Status]int    `yaml:"by_status" json:"by_status"`
	ByType         map[IssueType]int `yaml:"by_type" json:"by_type"`
	ByPriority     map[Priority]int  `yaml:"by_priority" json:"by_priority"`
	DateRange      *DateRange        `yaml:"date_range,omitempty" json:"date_range,omitempty"`
	TopAssignees   []AssigneeStat    `yaml:"top_assignees,omitempty" json:"top_assignees,omitempty"`
	MostUsedLabels []LabelStat       `yaml:"most_used_labels,omitempty" json:"most_used_labels,omitempty"`
}

// DateRange represents a date range for issues
type DateRange struct {
	Earliest time.Time `yaml:"earliest" json:"earliest"`
	Latest   time.Time `yaml:"latest" json:"latest"`
}

// AssigneeStat contains assignee statistics
type AssigneeStat struct {
	Username string `yaml:"username" json:"username"`
	Count    int    `yaml:"count" json:"count"`
}

// LabelStat contains label usage statistics
type LabelStat struct {
	Name  string `yaml:"name" json:"name"`
	Count int    `yaml:"count" json:"count"`
}

// NewGlobalConfig creates a new global configuration with default values
func NewGlobalConfig() *GlobalConfig {
	now := time.Now()
	return &GlobalConfig{
		Version:   "1.0.0",
		CreatedAt: now,
		UpdatedAt: now,
		Projects:  make(map[string]*ProjectInfo),
		GlobalSettings: GlobalSettings{
			DefaultFormat:      "table",
			AutoDiscovery:      true,
			ScanIntervalHours:  24,
			MaxBackupRetention: 30,
			CompressionEnabled: true,
			EncryptionEnabled:  false,
			NotificationSettings: NotificationSettings{
				Enabled:           false,
				OnBackup:          false,
				OnArchive:         false,
				OnProjectDetected: false,
				Methods:           []string{},
			},
		},
		ArchiveSettings: ArchiveSettings{
			AutoArchiveAfterDays: 90,
			ArchiveClosedIssues:  false,
			KeepLocalCopy:        true,
			CompressionLevel:     6,
			ExcludeStatuses:      []Status{StatusOpen, StatusInProgress},
		},
	}
}

// AddProject adds or updates a project in the global config
func (gc *GlobalConfig) AddProject(path, name string) *ProjectInfo {
	absPath, _ := filepath.Abs(path)

	project := &ProjectInfo{
		Path:         absPath,
		Name:         name,
		LastScan:     time.Now(),
		Status:       ProjectStatusActive,
		Tags:         []string{},
		CustomFields: make(map[string]string),
	}

	gc.Projects[absPath] = project
	gc.UpdatedAt = time.Now()

	return project
}

// GetProject retrieves a project by path
func (gc *GlobalConfig) GetProject(path string) (*ProjectInfo, bool) {
	absPath, _ := filepath.Abs(path)
	project, exists := gc.Projects[absPath]
	return project, exists
}

// RemoveProject removes a project from tracking
func (gc *GlobalConfig) RemoveProject(path string) bool {
	absPath, _ := filepath.Abs(path)
	if _, exists := gc.Projects[absPath]; exists {
		delete(gc.Projects, absPath)
		gc.UpdatedAt = time.Now()
		return true
	}
	return false
}

// GetActiveProjects returns all active projects
func (gc *GlobalConfig) GetActiveProjects() []*ProjectInfo {
	var active []*ProjectInfo
	for _, project := range gc.Projects {
		if project.Status == ProjectStatusActive {
			active = append(active, project)
		}
	}
	return active
}

// UpdateProject updates project information
func (gc *GlobalConfig) UpdateProject(path string, updateFn func(*ProjectInfo)) bool {
	absPath, _ := filepath.Abs(path)
	if project, exists := gc.Projects[absPath]; exists {
		updateFn(project)
		gc.UpdatedAt = time.Now()
		return true
	}
	return false
}

// NewArchivedIssue creates an archived issue from a regular issue
func NewArchivedIssue(issue *Issue, projectPath, projectName string) *ArchivedIssue {
	return &ArchivedIssue{
		Issue:        issue,
		ProjectPath:  projectPath,
		ProjectName:  projectName,
		ArchivedAt:   time.Now(),
		OriginalPath: filepath.Join(projectPath, ".issuemap", "issues", string(issue.ID)+".yaml"),
	}
}

// GetGlobalDir returns the global issuemap directory path
func GetGlobalDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if we can't determine home
		return ".issuemap_global"
	}
	return filepath.Join(homeDir, ".issuemap_global")
}

// GetArchivePath returns the path where archived issues are stored
func GetArchivePath() string {
	return filepath.Join(GetGlobalDir(), "archive")
}

// GetBackupPath returns the path where project backups are stored
func GetBackupPath() string {
	return filepath.Join(GetGlobalDir(), "backups")
}

// GetConfigPath returns the path to the global config file
func GetConfigPath() string {
	return filepath.Join(GetGlobalDir(), "config.yaml")
}
