package entities

import (
	"fmt"
	"strings"
	"time"
)

// IssueID represents a unique issue identifier
type IssueID string

// NewIssueID creates a new issue ID with the given number
func NewIssueID(number int) IssueID {
	return IssueID(fmt.Sprintf("ISSUE-%03d", number))
}

// String returns the string representation of the issue ID
func (id IssueID) String() string {
	return string(id)
}

// IssueType represents the type of an issue
type IssueType string

const (
	IssueTypeBug     IssueType = "bug"
	IssueTypeFeature IssueType = "feature"
	IssueTypeTask    IssueType = "task"
	IssueTypeEpic    IssueType = "epic"
)

// Status represents the current status of an issue
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in-progress"
	StatusReview     Status = "review"
	StatusDone       Status = "done"
	StatusClosed     Status = "closed"
)

// Priority represents the priority level of an issue
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Label represents a label that can be applied to issues
type Label struct {
	Name  string `yaml:"name" json:"name"`
	Color string `yaml:"color" json:"color"`
}

// User represents a user in the system
type User struct {
	Username string `yaml:"username" json:"username"`
	Email    string `yaml:"email" json:"email"`
}

// Milestone represents a project milestone
type Milestone struct {
	Name        string     `yaml:"name" json:"name"`
	Description string     `yaml:"description,omitempty" json:"description,omitempty"`
	DueDate     *time.Time `yaml:"due_date,omitempty" json:"due_date,omitempty"`
}

// CommitRef represents a reference to a git commit
type CommitRef struct {
	Hash    string    `yaml:"hash" json:"hash"`
	Message string    `yaml:"message" json:"message"`
	Author  string    `yaml:"author" json:"author"`
	Date    time.Time `yaml:"date" json:"date"`
}

// Comment represents a comment on an issue
type Comment struct {
	ID     int       `yaml:"id" json:"id"`
	Author string    `yaml:"author" json:"author"`
	Date   time.Time `yaml:"date" json:"date"`
	Text   string    `yaml:"text" json:"text"`
}

// IssueMetadata contains additional metadata for an issue
type IssueMetadata struct {
	EstimatedHours *float64          `yaml:"estimated_hours,omitempty" json:"estimated_hours,omitempty"`
	ActualHours    *float64          `yaml:"actual_hours,omitempty" json:"actual_hours,omitempty"`
	CustomFields   map[string]string `yaml:"custom_fields,omitempty" json:"custom_fields,omitempty"`
}

// Timestamps contains timestamp information for an issue
type Timestamps struct {
	Created time.Time  `yaml:"created" json:"created"`
	Updated time.Time  `yaml:"updated" json:"updated"`
	Closed  *time.Time `yaml:"closed,omitempty" json:"closed,omitempty"`
}

// Issue represents a single issue in the system
type Issue struct {
	ID          IssueID       `yaml:"id" json:"id"`
	Title       string        `yaml:"title" json:"title"`
	Description string        `yaml:"description" json:"description"`
	Type        IssueType     `yaml:"type" json:"type"`
	Status      Status        `yaml:"status" json:"status"`
	Priority    Priority      `yaml:"priority" json:"priority"`
	Labels      []Label       `yaml:"labels" json:"labels"`
	Assignee    *User         `yaml:"assignee,omitempty" json:"assignee,omitempty"`
	Milestone   *Milestone    `yaml:"milestone,omitempty" json:"milestone,omitempty"`
	Branch      string        `yaml:"branch,omitempty" json:"branch,omitempty"`
	Commits     []CommitRef   `yaml:"commits" json:"commits"`
	Comments    []Comment     `yaml:"comments" json:"comments"`
	Metadata    IssueMetadata `yaml:"metadata" json:"metadata"`
	Timestamps  Timestamps    `yaml:"timestamps" json:"timestamps"`
}

// NewIssue creates a new issue with default values
func NewIssue(id IssueID, title, description string, issueType IssueType) *Issue {
	now := time.Now()
	return &Issue{
		ID:          id,
		Title:       title,
		Description: description,
		Type:        issueType,
		Status:      StatusOpen,
		Priority:    PriorityMedium,
		Labels:      []Label{},
		Commits:     []CommitRef{},
		Comments:    []Comment{},
		Metadata:    IssueMetadata{CustomFields: make(map[string]string)},
		Timestamps: Timestamps{
			Created: now,
			Updated: now,
		},
	}
}

// UpdateStatus changes the status of the issue and updates the timestamp
func (i *Issue) UpdateStatus(status Status) {
	i.Status = status
	i.Timestamps.Updated = time.Now()

	if status == StatusClosed {
		now := time.Now()
		i.Timestamps.Closed = &now
	}
}

// AddComment adds a new comment to the issue
func (i *Issue) AddComment(author, text string) {
	commentID := len(i.Comments) + 1
	comment := Comment{
		ID:     commentID,
		Author: author,
		Date:   time.Now(),
		Text:   text,
	}
	i.Comments = append(i.Comments, comment)
	i.Timestamps.Updated = time.Now()
}

// AddLabel adds a label to the issue if it doesn't already exist
func (i *Issue) AddLabel(label Label) {
	for _, existingLabel := range i.Labels {
		if existingLabel.Name == label.Name {
			return // Label already exists
		}
	}
	i.Labels = append(i.Labels, label)
	i.Timestamps.Updated = time.Now()
}

// RemoveLabel removes a label from the issue
func (i *Issue) RemoveLabel(labelName string) {
	for idx, label := range i.Labels {
		if label.Name == labelName {
			i.Labels = append(i.Labels[:idx], i.Labels[idx+1:]...)
			i.Timestamps.Updated = time.Now()
			break
		}
	}
}

// SetAssignee sets the assignee for the issue
func (i *Issue) SetAssignee(user *User) {
	i.Assignee = user
	i.Timestamps.Updated = time.Now()
}

// SetMilestone sets the milestone for the issue
func (i *Issue) SetMilestone(milestone *Milestone) {
	i.Milestone = milestone
	i.Timestamps.Updated = time.Now()
}

// AddCommit adds a commit reference to the issue
func (i *Issue) AddCommit(commit CommitRef) {
	i.Commits = append(i.Commits, commit)
	i.Timestamps.Updated = time.Now()
}

// GetStatusDirectory returns the directory name for the issue based on its status
func (i *Issue) GetStatusDirectory() string {
	return strings.ReplaceAll(string(i.Status), "-", "_")
}

// Validate validates the issue data
func (i *Issue) Validate() error {
	if i.ID == "" {
		return fmt.Errorf("issue ID cannot be empty")
	}
	if strings.TrimSpace(i.Title) == "" {
		return fmt.Errorf("issue title cannot be empty")
	}
	if i.Type == "" {
		return fmt.Errorf("issue type cannot be empty")
	}
	if i.Status == "" {
		return fmt.Errorf("issue status cannot be empty")
	}
	if i.Priority == "" {
		return fmt.Errorf("issue priority cannot be empty")
	}
	return nil
}
