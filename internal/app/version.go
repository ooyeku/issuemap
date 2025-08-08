package app

import (
	"fmt"
	"runtime"
)

// Version information
const (
	// Version is the current version of IssueMap
	Version = "0.2.1"

	// VersionMajor is the major version number
	VersionMajor = 0

	// VersionMinor is the minor version number
	VersionMinor = 2

	// VersionPatch is the patch version number
	VersionPatch = 1

	// VersionPrerelease is the prerelease version (empty for stable)
	VersionPrerelease = ""

	// BuildDate is set during build time
	BuildDate = "dev"

	// GitCommit is set during build time
	GitCommit = "dev"
)

// Application metadata
const (
	// AppName is the name of the application
	AppName = "IssueMap"

	// AppDescription is a short description of the application
	AppDescription = "Git-native issue tracking for your projects"

	// AppLongDescription is a detailed description
	AppLongDescription = "IssueMap is a CLI-first issue tracker that stores issues as files in your git repository. It provides seamless integration with git workflows while keeping everything version-controlled."

	// AppAuthor is the author/organization
	AppAuthor = "IssueMap Contributors"

	// AppWebsite is the project website
	AppWebsite = "https://github.com/ooyeku/issuemap"

	// AppLicense is the license type
	AppLicense = "MIT"
)

// Directory and file constants
const (
	// ConfigDirName is the name of the configuration directory
	ConfigDirName = ".issuemap"

	// IssuesDirName is the subdirectory for issues
	IssuesDirName = "issues"

	// HistoryDirName is the subdirectory for history
	HistoryDirName = "history"

	// TemplatesDirName is the subdirectory for templates
	TemplatesDirName = "templates"

	// MetadataDirName is the subdirectory for metadata
	MetadataDirName = "metadata"

	// ConfigFileName is the main configuration file name
	ConfigFileName = "config.yaml"

	// IssueFileExtension is the file extension for issue files
	IssueFileExtension = ".yaml"

	// HistoryFileExtension is the file extension for history files
	HistoryFileExtension = ".yaml"

	// TemplateFileExtension is the file extension for template files
	TemplateFileExtension = ".yaml"
)

// Issue constants
const (
	// IssueIDPrefix is the prefix for issue IDs
	IssueIDPrefix = "ISSUE-"

	// IssueIDFormat is the format string for issue IDs
	IssueIDFormat = "ISSUE-%03d"

	// DefaultIssueType is the default type for new issues
	DefaultIssueType = "task"

	// DefaultIssuePriority is the default priority for new issues
	DefaultIssuePriority = "medium"

	// DefaultIssueStatus is the default status for new issues
	DefaultIssueStatus = "open"
)

// Git constants
const (
	// GitHooksDirName is the git hooks directory
	GitHooksDirName = ".git/hooks"

	// CommitMsgHook is the commit message hook filename
	CommitMsgHook = "commit-msg"

	// PreCommitHook is the pre-commit hook filename
	PreCommitHook = "pre-commit"

	// IssueReferencePattern is the regex pattern for issue references
	IssueReferencePattern = `(?i)\b(?:refs?|references?|fixes?|closes?|resolves?)\s+(ISSUE-\d+)\b`
)

// CLI constants
const (
	// DefaultListLimit is the default limit for list commands
	DefaultListLimit = 50

	// DefaultHistoryLimit is the default limit for history commands
	DefaultHistoryLimit = 50

	// MaxListLimit is the maximum limit for list commands
	MaxListLimit = 1000

	// TableFormat is the table output format
	TableFormat = "table"

	// JSONFormat is the JSON output format
	JSONFormat = "json"

	// YAMLFormat is the YAML output format
	YAMLFormat = "yaml"
)

// Status constants
const (
	// StatusOpen represents an open issue
	StatusOpen = "open"

	// StatusInProgress represents an issue in progress
	StatusInProgress = "in-progress"

	// StatusReview represents an issue under review
	StatusReview = "review"

	// StatusDone represents a completed issue
	StatusDone = "done"

	// StatusClosed represents a closed issue
	StatusClosed = "closed"
)

// Priority constants
const (
	// PriorityLow represents low priority
	PriorityLow = "low"

	// PriorityMedium represents medium priority
	PriorityMedium = "medium"

	// PriorityHigh represents high priority
	PriorityHigh = "high"

	// PriorityCritical represents critical priority
	PriorityCritical = "critical"
)

// Issue type constants
const (
	// TypeBug represents a bug issue
	TypeBug = "bug"

	// TypeFeature represents a feature request
	TypeFeature = "feature"

	// TypeTask represents a task
	TypeTask = "task"

	// TypeEpic represents an epic
	TypeEpic = "epic"

	// TypeImprovement represents an improvement
	TypeImprovement = "improvement"

	// TypeDocumentation represents documentation work
	TypeDocumentation = "documentation"
)

// Error messages
const (
	// ErrNotInitialized is shown when issuemap is not initialized
	ErrNotInitialized = "issuemap not initialized in this repository. Run 'issuemap init' first."

	// ErrNotGitRepo is shown when not in a git repository
	ErrNotGitRepo = "not in a git repository"

	// ErrIssueNotFound is shown when an issue is not found
	ErrIssueNotFound = "issue not found"

	// ErrInvalidIssueID is shown for invalid issue IDs
	ErrInvalidIssueID = "invalid issue ID format"
)

// Success messages
const (
	// MsgIssueCreated is shown when an issue is created
	MsgIssueCreated = "Issue %s created successfully"

	// MsgIssueUpdated is shown when an issue is updated
	MsgIssueUpdated = "Issue %s updated successfully"

	// MsgIssueClosed is shown when an issue is closed
	MsgIssueClosed = "Issue %s closed successfully"

	// MsgIssueReopened is shown when an issue is reopened
	MsgIssueReopened = "Issue %s reopened successfully"

	// MsgIssueAssigned is shown when an issue is assigned
	MsgIssueAssigned = "Issue %s assigned to %s successfully"

	// MsgProjectInitialized is shown when a project is initialized
	MsgProjectInitialized = "IssueMap initialized successfully in %s"
)

// Server constants
const (
	// DefaultServerPort is the default port for the IssueMap server
	DefaultServerPort = 4042

	// ServerPortRange defines the range for automatic port selection
	ServerPortRangeStart = 4042
	ServerPortRangeEnd   = 4052

	// ServerPIDFile is the filename for storing server PID
	ServerPIDFile = "server.pid"

	// ServerLogFile is the filename for server logs
	ServerLogFile = "server.log"

	// ServerShutdownTimeout is the timeout for graceful shutdown
	ServerShutdownTimeout = 30 // seconds

	// APIBasePath is the base path for API endpoints
	APIBasePath = "/api/v1"
)

// GetVersion returns the full version string
func GetVersion() string {
	version := Version
	if VersionPrerelease != "" {
		version += "-" + VersionPrerelease
	}
	return version
}

// GetFullVersion returns detailed version information
func GetFullVersion() string {
	return fmt.Sprintf("%s %s\nBuilt with %s %s on %s\nCommit: %s\nBuild Date: %s",
		AppName,
		GetVersion(),
		runtime.Compiler,
		runtime.Version(),
		runtime.GOOS+"/"+runtime.GOARCH,
		GitCommit,
		BuildDate,
	)
}

// GetVersionInfo returns structured version information
func GetVersionInfo() map[string]string {
	return map[string]string{
		"version":     GetVersion(),
		"major":       fmt.Sprintf("%d", VersionMajor),
		"minor":       fmt.Sprintf("%d", VersionMinor),
		"patch":       fmt.Sprintf("%d", VersionPatch),
		"prerelease":  VersionPrerelease,
		"build_date":  BuildDate,
		"git_commit":  GitCommit,
		"go_version":  runtime.Version(),
		"go_compiler": runtime.Compiler,
		"platform":    runtime.GOOS + "/" + runtime.GOARCH,
	}
}
