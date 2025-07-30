# IssueMap Design Document

## Overview

**IssueMap** is a git-native, CLI-first issue tracking system that stores issues as structured files within your repository. It provides seamless integration with git workflows while maintaining professional code quality and clean architecture principles.

## Core Principles

### 1. Clean Architecture
- **Separation of Concerns**: Clear boundaries between CLI, business logic, and storage
- **Dependency Inversion**: Core business logic independent of external frameworks
- **Interface-Driven Design**: Abstractions for all external dependencies
- **Testability**: 100% unit test coverage with dependency injection

### 2. Professional Code Quality
- **Go Best Practices**: Follows effective Go patterns and idioms
- **Error Handling**: Comprehensive error handling with wrapped errors
- **Documentation**: Godoc comments for all public APIs
- **Linting**: golangci-lint with strict rules
- **Performance**: Optimized for large repositories and many issues

### 3. Robust API Design
- **Type Safety**: Strong typing with validation
- **Immutability**: Immutable core data structures where possible
- **Consistency**: Uniform patterns across all operations
- **Extensibility**: Plugin-friendly architecture

## Architecture

### Layered Architecture

```
┌─────────────────────────────────────────┐
│              CLI Layer                  │
│  (cobra commands, flags, output)        │
├─────────────────────────────────────────┤
│            Application Layer            │
│     (use cases, orchestration)          │
├─────────────────────────────────────────┤
│              Domain Layer               │
│   (business logic, entities, rules)     │
├─────────────────────────────────────────┤
│           Infrastructure Layer          │
│  (storage, git integration, filesystem) │
└─────────────────────────────────────────┘
```

### Project Structure

```
issuemap/
├── cmd/
│   └── issuemap/
│       └── main.go                 # Application entry point
├── internal/
│   ├── app/                        # Application layer
│   │   ├── commands/               # Use case implementations
│   │   │   ├── create_issue.go
│   │   │   ├── list_issues.go
│   │   │   ├── update_issue.go
│   │   │   └── ...
│   │   └── services/               # Application services
│   │       ├── issue_service.go
│   │       ├── git_service.go
│   │       └── template_service.go
│   ├── domain/                     # Domain layer
│   │   ├── entities/               # Core entities
│   │   │   ├── issue.go
│   │   │   ├── milestone.go
│   │   │   ├── label.go
│   │   │   └── comment.go
│   │   ├── repositories/           # Repository interfaces
│   │   │   ├── issue_repository.go
│   │   │   ├── config_repository.go
│   │   │   └── git_repository.go
│   │   ├── services/               # Domain services
│   │   │   ├── issue_validator.go
│   │   │   ├── id_generator.go
│   │   │   └── workflow_engine.go
│   │   └── errors/                 # Domain errors
│   │       └── errors.go
│   ├── infrastructure/             # Infrastructure layer
│   │   ├── storage/                # Storage implementations
│   │   │   ├── file_issue_repo.go
│   │   │   ├── file_config_repo.go
│   │   │   └── storage_manager.go
│   │   ├── git/                    # Git integration
│   │   │   ├── git_client.go
│   │   │   ├── hook_manager.go
│   │   │   └── commit_parser.go
│   │   └── filesystem/             # File system operations
│   │       ├── file_manager.go
│   │       └── path_resolver.go
│   └── cli/                        # CLI layer
│       ├── commands/               # Cobra command definitions
│       │   ├── root.go
│       │   ├── create.go
│       │   ├── list.go
│       │   ├── show.go
│       │   ├── update.go
│       │   ├── git.go
│       │   └── config.go
│       ├── presenters/             # Output formatting
│       │   ├── table_presenter.go
│       │   ├── json_presenter.go
│       │   └── markdown_presenter.go
│       ├── input/                  # Input handling
│       │   ├── validators.go
│       │   ├── prompts.go
│       │   └── parsers.go
│       └── ui/                     # UI components
│           ├── colors.go
│           ├── spinners.go
│           └── progress.go
├── pkg/                            # Public API
│   └── issuemap/
│       ├── client.go               # Public client API
│       ├── types.go                # Public types
│       └── errors.go               # Public errors
├── configs/                        # Configuration templates
│   ├── default.yaml
│   └── templates/
│       ├── bug.yaml
│       ├── feature.yaml
│       └── task.yaml
├── docs/                          # Documentation
│   ├── api.md
│   ├── commands.md
│   └── architecture.md
└── scripts/                       # Build and utility scripts
    ├── build.sh
    ├── test.sh
    └── lint.sh
```

## Core Domain Model

### Issue Entity

```go
type Issue struct {
    ID          IssueID           `yaml:"id" json:"id"`
    Title       string            `yaml:"title" json:"title"`
    Description string            `yaml:"description" json:"description"`
    Type        IssueType         `yaml:"type" json:"type"`
    Status      Status            `yaml:"status" json:"status"`
    Priority    Priority          `yaml:"priority" json:"priority"`
    Labels      []Label           `yaml:"labels" json:"labels"`
    Assignee    *User             `yaml:"assignee,omitempty" json:"assignee,omitempty"`
    Milestone   *Milestone        `yaml:"milestone,omitempty" json:"milestone,omitempty"`
    Branch      string            `yaml:"branch,omitempty" json:"branch,omitempty"`
    Commits     []CommitRef       `yaml:"commits" json:"commits"`
    Comments    []Comment         `yaml:"comments" json:"comments"`
    Metadata    IssueMetadata     `yaml:"metadata" json:"metadata"`
    Timestamps  Timestamps        `yaml:"timestamps" json:"timestamps"`
}

type IssueMetadata struct {
    EstimatedHours *float64          `yaml:"estimated_hours,omitempty" json:"estimated_hours,omitempty"`
    ActualHours    *float64          `yaml:"actual_hours,omitempty" json:"actual_hours,omitempty"`
    CustomFields   map[string]string `yaml:"custom_fields,omitempty" json:"custom_fields,omitempty"`
}

type Timestamps struct {
    Created time.Time  `yaml:"created" json:"created"`
    Updated time.Time  `yaml:"updated" json:"updated"`
    Closed  *time.Time `yaml:"closed,omitempty" json:"closed,omitempty"`
}
```

### Repository Interfaces

```go
type IssueRepository interface {
    Create(ctx context.Context, issue *Issue) error
    GetByID(ctx context.Context, id IssueID) (*Issue, error)
    List(ctx context.Context, filter IssueFilter) ([]*Issue, error)
    Update(ctx context.Context, issue *Issue) error
    Delete(ctx context.Context, id IssueID) error
    Search(ctx context.Context, query SearchQuery) ([]*Issue, error)
}

type GitRepository interface {
    GetCurrentBranch(ctx context.Context) (string, error)
    GetCommitsSince(ctx context.Context, since time.Time) ([]Commit, error)
    CreateBranch(ctx context.Context, name string) error
    GetCommitMessage(ctx context.Context, hash string) (string, error)
    InstallHooks(ctx context.Context) error
}

type ConfigRepository interface {
    Load(ctx context.Context) (*Config, error)
    Save(ctx context.Context, config *Config) error
    GetTemplate(ctx context.Context, name string) (*Template, error)
}
```

## Storage Format

### Directory Structure
```
.issuemap/
├── config.yaml                    # Project configuration
├── issues/
│   ├── ISSUE-001.yaml             # Issues stored by ID
│   ├── ISSUE-002.yaml
│   └── ...
├── metadata/
│   ├── labels.yaml                # Available labels
│   ├── milestones.yaml            # Project milestones
│   └── users.yaml                 # Project users
├── templates/                     # Issue templates
│   ├── bug.yaml
│   ├── feature.yaml
│   └── custom.yaml
└── hooks/                         # Git hooks (optional)
    ├── commit-msg
    └── post-merge
```

### Issue File Format (YAML)
```yaml
id: ISSUE-001
title: "Implement user authentication system"
description: |
  Add JWT-based authentication with the following features:
  - User registration and login endpoints
  - Token validation middleware
  - Password hashing with bcrypt
  - Rate limiting for auth endpoints
type: feature
status: in-progress
priority: high
labels:
  - name: authentication
    color: "#ff0000"
  - name: security
    color: "#ffa500"
assignee:
  username: ooyeku
  email: ooyeku@example.com
milestone:
  name: v1.0.0
  due_date: 2024-03-01T00:00:00Z
branch: feature/auth-system
commits:
  - hash: a1b2c3d4
    message: "Initial auth scaffold"
    author: ooyeku
    date: 2024-01-15T10:30:00Z
comments:
  - id: 1
    author: ooyeku
    date: 2024-01-15T11:00:00Z
    text: "Started implementation with JWT library"
metadata:
  estimated_hours: 16
  actual_hours: 8
  custom_fields:
    epic: "user-management"
    component: "backend"
timestamps:
  created: 2024-01-15T10:00:00Z
  updated: 2024-01-16T14:30:00Z
```

## CLI Commands

### Core Commands

```bash
# Initialize project
issuemap init [--template <template>]

# Issue management
issuemap create [--type <type>] [--template <template>] [--interactive]
issuemap list [--status <status>] [--assignee <user>] [--label <label>] [--format <format>]
issuemap show <issue-id> [--format <format>]
issuemap edit <issue-id> [--field <field>=<value>]
issuemap close <issue-id> [--reason <reason>]
issuemap reopen <issue-id>

# Search and filtering
issuemap search <query> [--type <type>] [--status <status>]
issuemap filter --assignee <user> --priority high --created-since 1w

# Git integration
issuemap branch <issue-id> [--name <branch-name>]
issuemap link <issue-id> [--branch <branch>]
issuemap commit <issue-id> --message "<message>"

# Project management
issuemap milestone create <name> [--due <date>]
issuemap milestone list [--status <status>]
issuemap label create <name> --color <color>
issuemap stats [--since <date>] [--format <format>]

# Configuration
issuemap config set <key> <value>
issuemap config get <key>
issuemap template create <name> --type <type>
```

## API Design

### Public Client API

```go
package issuemap

// Client provides the main API for issuemap operations
type Client struct {
    config *Config
    repos  *Repositories
}

// NewClient creates a new issuemap client
func NewClient(repoPath string, options ...Option) (*Client, error)

// Issue operations
func (c *Client) CreateIssue(ctx context.Context, req CreateIssueRequest) (*Issue, error)
func (c *Client) GetIssue(ctx context.Context, id IssueID) (*Issue, error)
func (c *Client) ListIssues(ctx context.Context, filter IssueFilter) (*IssueList, error)
func (c *Client) UpdateIssue(ctx context.Context, id IssueID, updates IssueUpdates) (*Issue, error)
func (c *Client) DeleteIssue(ctx context.Context, id IssueID) error
func (c *Client) SearchIssues(ctx context.Context, query SearchQuery) (*SearchResult, error)

// Git integration
func (c *Client) LinkIssueToBranch(ctx context.Context, issueID IssueID, branch string) error
func (c *Client) CreateBranchForIssue(ctx context.Context, issueID IssueID, name string) error
func (c *Client) GetIssuesForCommit(ctx context.Context, commitHash string) ([]*Issue, error)

// Project management
func (c *Client) CreateMilestone(ctx context.Context, milestone *Milestone) error
func (c *Client) ListMilestones(ctx context.Context) ([]*Milestone, error)
func (c *Client) CreateLabel(ctx context.Context, label *Label) error
func (c *Client) GetProjectStats(ctx context.Context, opts StatsOptions) (*ProjectStats, error)
```

## Error Handling

### Error Types

```go
package errors

// Domain errors
var (
    ErrIssueNotFound      = errors.New("issue not found")
    ErrIssueAlreadyExists = errors.New("issue already exists")
    ErrInvalidIssueID     = errors.New("invalid issue ID")
    ErrInvalidStatus      = errors.New("invalid status transition")
    ErrGitNotInitialized  = errors.New("git repository not initialized")
    ErrNotInGitRepo       = errors.New("not in a git repository")
)

// Error wrapping for context
type Error struct {
    Op   string // Operation that failed
    Kind string // Error kind
    Err  error  // Underlying error
}

func (e *Error) Error() string {
    return fmt.Sprintf("%s: %s: %v", e.Op, e.Kind, e.Err)
}
```

## Testing Strategy

### Unit Tests
- **Domain Layer**: 100% coverage of business logic
- **Application Layer**: Use case testing with mocked dependencies
- **Infrastructure Layer**: Integration tests with test fixtures
- **CLI Layer**: Command testing with captured output

### Integration Tests
- **End-to-End**: Full CLI workflow testing
- **Git Integration**: Real git repository testing
- **Storage**: File system operations testing

### Test Structure
```
internal/
├── app/
│   └── commands/
│       ├── create_issue_test.go
│       └── ...
├── domain/
│   └── entities/
│       ├── issue_test.go
│       └── ...
└── infrastructure/
    └── storage/
        ├── file_issue_repo_test.go
        └── ...
test/
├── fixtures/                      # Test data
├── testutils/                     # Test utilities
└── integration/                   # Integration tests
```

## Performance Considerations

### Optimization Strategies
- **Lazy Loading**: Load issues on-demand for large repositories
- **Indexing**: In-memory indexing for fast searches
- **Caching**: Cache frequently accessed data
- **Parallel Processing**: Concurrent operations where safe
- **Minimal I/O**: Batch file operations and optimize reads

### Scalability Targets
- **Repository Size**: Support repositories with 10,000+ issues
- **Search Performance**: Sub-second search across all issues
- **Memory Usage**: Constant memory usage regardless of repository size
- **CLI Responsiveness**: All commands complete within 100ms for typical operations

## Security Considerations

### Data Protection
- **Input Validation**: Strict validation of all user inputs
- **Path Traversal**: Prevent directory traversal attacks
- **File Permissions**: Secure file permissions for issue data
- **Git Hooks**: Safe git hook installation and execution

### Authentication & Authorization
- **Git Integration**: Respect git's security model
- **File Access**: Use OS-level file permissions
- **No Secrets**: Never store sensitive data in issue files

## Configuration

### Global Configuration
```yaml
# ~/.config/issuemap/config.yaml
defaults:
  issue_type: task
  priority: medium
  format: table
  editor: $EDITOR
  
integrations:
  git:
    auto_link: true
    auto_close_keywords: ["closes", "fixes", "resolves"]
    
ui:
  colors: true
  interactive: true
  pager: auto
```

### Project Configuration
```yaml
# .issuemap/config.yaml
project:
  name: "My Project"
  version: "1.0.0"
  
workflow:
  statuses: [open, in-progress, review, done, closed]
  default_status: open
  
templates:
  default: task
  available: [bug, feature, task, epic]
  
labels:
  - name: bug
    color: "#d73a4a"
  - name: enhancement
    color: "#a2eeef"
  
milestones:
  - name: v1.0.0
    due_date: 2024-03-01
    description: "Initial release"
```

## Dependencies

```go
// Core dependencies
github.com/spf13/cobra           // CLI framework (already added)
github.com/spf13/viper          // Configuration management
gopkg.in/yaml.v3                // YAML processing
github.com/go-git/go-git/v5     // Git operations

// Utilities
github.com/google/uuid          // ID generation
github.com/pkg/errors           // Error wrapping
github.com/stretchr/testify     // Testing framework

// CLI enhancement
github.com/fatih/color          // Colored output
github.com/olekukonko/tablewriter // Table formatting
github.com/AlecAivazis/survey/v2   // Interactive prompts
github.com/briandowns/spinner   // Loading indicators

// Optional (for advanced features)
github.com/blevesearch/bleve/v2 // Full-text search (if needed)
github.com/fsnotify/fsnotify    // File watching (for live updates)
```

## Development Workflow

### Build and Development
```bash
# Setup
make setup          # Install dependencies and tools
make build          # Build binary
make test           # Run all tests
make lint           # Run linters
make docs           # Generate documentation

# Development
make dev            # Build and install development version
make test-watch     # Continuous testing
make integration    # Run integration tests
```

### Release Process
1. Version tagging with semantic versioning
2. Automated builds for multiple platforms
3. GitHub releases with binaries
4. Homebrew formula updates
5. Documentation updates

This design provides a solid foundation for a professional, maintainable, and extensible issue tracking system that integrates seamlessly with git workflows while maintaining clean architecture principles.
