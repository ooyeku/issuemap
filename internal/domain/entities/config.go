package entities

// Config represents the project configuration
type Config struct {
	Project    ProjectConfig   `yaml:"project" json:"project"`
	Workflow   WorkflowConfig  `yaml:"workflow" json:"workflow"`
	Templates  TemplatesConfig `yaml:"templates" json:"templates"`
	Labels     []Label         `yaml:"labels" json:"labels"`
	Milestones []Milestone     `yaml:"milestones" json:"milestones"`
	Git        GitConfig       `yaml:"git" json:"git"`
	UI         UIConfig        `yaml:"ui" json:"ui"`
}

// ProjectConfig contains project-specific settings
type ProjectConfig struct {
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// WorkflowConfig defines the workflow statuses and transitions
type WorkflowConfig struct {
	Statuses      []Status `yaml:"statuses" json:"statuses"`
	DefaultStatus Status   `yaml:"default_status" json:"default_status"`
}

// TemplatesConfig defines available issue templates
type TemplatesConfig struct {
	Default   string   `yaml:"default" json:"default"`
	Available []string `yaml:"available" json:"available"`
}

// GitConfig contains git integration settings
type GitConfig struct {
	AutoLink            bool     `yaml:"auto_link" json:"auto_link"`
	AutoCloseKeywords   []string `yaml:"auto_close_keywords" json:"auto_close_keywords"`
	DefaultBranchPrefix string   `yaml:"default_branch_prefix" json:"default_branch_prefix"`
}

// UIConfig contains user interface settings
type UIConfig struct {
	Colors      bool   `yaml:"colors" json:"colors"`
	Interactive bool   `yaml:"interactive" json:"interactive"`
	Pager       string `yaml:"pager" json:"pager"`
	Format      string `yaml:"format" json:"format"`
}

// Template represents an issue template
type Template struct {
	Name        string    `yaml:"name" json:"name"`
	Type        IssueType `yaml:"type" json:"type"`
	Title       string    `yaml:"title" json:"title"`
	Description string    `yaml:"description" json:"description"`
	Labels      []string  `yaml:"labels" json:"labels"`
	Priority    Priority  `yaml:"priority" json:"priority"`
}

// NewDefaultConfig creates a new configuration with default values
func NewDefaultConfig() *Config {
	return &Config{
		Project: ProjectConfig{
			Name:    "My Project",
			Version: "1.0.0",
		},
		Workflow: WorkflowConfig{
			Statuses:      []Status{StatusOpen, StatusInProgress, StatusReview, StatusDone, StatusClosed},
			DefaultStatus: StatusOpen,
		},
		Templates: TemplatesConfig{
			Default:   "task",
			Available: []string{"bug", "feature", "task", "epic"},
		},
		Labels: []Label{
			{Name: "bug", Color: "#d73a4a"},
			{Name: "enhancement", Color: "#a2eeef"},
			{Name: "documentation", Color: "#0075ca"},
			{Name: "good first issue", Color: "#7057ff"},
		},
		Milestones: []Milestone{},
		Git: GitConfig{
			AutoLink:            true,
			AutoCloseKeywords:   []string{"closes", "fixes", "resolves"},
			DefaultBranchPrefix: "feature/",
		},
		UI: UIConfig{
			Colors:      true,
			Interactive: true,
			Pager:       "auto",
			Format:      "table",
		},
	}
}

// GetTemplate returns a template by name
func (c *Config) GetTemplate(name string) *Template {
	switch name {
	case "bug":
		return &Template{
			Name:        "bug",
			Type:        IssueTypeBug,
			Title:       "Bug Report",
			Description: "## Description\nA clear and concise description of what the bug is.\n\n## Steps to Reproduce\n1. Go to '...'\n2. Click on '....'\n3. Scroll down to '....'\n4. See error\n\n## Expected Behavior\nA clear and concise description of what you expected to happen.\n\n## Actual Behavior\nA clear and concise description of what actually happened.\n\n## Additional Context\nAdd any other context about the problem here.",
			Labels:      []string{"bug"},
			Priority:    PriorityMedium,
		}
	case "feature":
		return &Template{
			Name:        "feature",
			Type:        IssueTypeFeature,
			Title:       "Feature Request",
			Description: "## Summary\nA clear and concise description of the feature you'd like to see.\n\n## Motivation\nWhy is this feature important? What problem does it solve?\n\n## Detailed Description\nProvide a detailed description of the feature.\n\n## Acceptance Criteria\n- [ ] Criterion 1\n- [ ] Criterion 2\n- [ ] Criterion 3\n\n## Additional Context\nAdd any other context or screenshots about the feature request here.",
			Labels:      []string{"enhancement"},
			Priority:    PriorityMedium,
		}
	case "task":
		return &Template{
			Name:        "task",
			Type:        IssueTypeTask,
			Title:       "Task",
			Description: "## Description\nA clear description of the task to be completed.\n\n## Acceptance Criteria\n- [ ] Criterion 1\n- [ ] Criterion 2\n- [ ] Criterion 3\n\n## Notes\nAny additional notes or context.",
			Labels:      []string{},
			Priority:    PriorityMedium,
		}
	case "epic":
		return &Template{
			Name:        "epic",
			Type:        IssueTypeEpic,
			Title:       "Epic",
			Description: "## Overview\nA high-level description of this epic.\n\n## Goals\n- Goal 1\n- Goal 2\n- Goal 3\n\n## User Stories\n- [ ] As a user, I want...\n- [ ] As a user, I want...\n- [ ] As a user, I want...\n\n## Definition of Done\n- [ ] All user stories completed\n- [ ] Documentation updated\n- [ ] Tests written and passing",
			Labels:      []string{"epic"},
			Priority:    PriorityHigh,
		}
	default:
		return &Template{
			Name:        "default",
			Type:        IssueTypeTask,
			Title:       "",
			Description: "",
			Labels:      []string{},
			Priority:    PriorityMedium,
		}
	}
}
