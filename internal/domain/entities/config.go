package entities

// Config represents the project configuration
type Config struct {
	Project       ProjectConfig     `yaml:"project" json:"project"`
	Workflow      WorkflowConfig    `yaml:"workflow" json:"workflow"`
	Templates     TemplatesConfig   `yaml:"templates" json:"templates"`
	Labels        []Label           `yaml:"labels" json:"labels"`
	Milestones    []Milestone       `yaml:"milestones" json:"milestones"`
	Git           GitConfig         `yaml:"git" json:"git"`
	UI            UIConfig          `yaml:"ui" json:"ui"`
	SavedSearches map[string]string `yaml:"saved_searches,omitempty" json:"saved_searches,omitempty"`
	StorageConfig *StorageConfig    `yaml:"storage,omitempty" json:"storage,omitempty"`
	ArchiveConfig *ArchiveConfig    `yaml:"archive,omitempty" json:"archive,omitempty"`
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
	AutoLink            bool         `yaml:"auto_link" json:"auto_link"`
	AutoCloseKeywords   []string     `yaml:"auto_close_keywords" json:"auto_close_keywords"`
	DefaultBranchPrefix string       `yaml:"default_branch_prefix" json:"default_branch_prefix"`
	BranchConfig        BranchConfig `yaml:"branch_config" json:"branch_config"`
}

// BranchConfig contains branch naming and management settings
type BranchConfig struct {
	Template         string            `yaml:"template" json:"template"`
	PrefixByType     map[string]string `yaml:"prefix_by_type" json:"prefix_by_type"`
	MaxTitleLength   int               `yaml:"max_title_length" json:"max_title_length"`
	AutoSwitch       bool              `yaml:"auto_switch" json:"auto_switch"`
	AutoMergeTargets []string          `yaml:"auto_merge_targets" json:"auto_merge_targets"`
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
	Name        string          `yaml:"name" json:"name"`
	Type        IssueType       `yaml:"type" json:"type"`
	Title       string          `yaml:"title" json:"title"`
	Description string          `yaml:"description" json:"description"`
	Labels      []string        `yaml:"labels" json:"labels"`
	Priority    Priority        `yaml:"priority" json:"priority"`
	Fields      []TemplateField `yaml:"fields,omitempty" json:"fields,omitempty"`
	Automation  TemplateAuto    `yaml:"automation,omitempty" json:"automation,omitempty"`
}

// TemplateField represents a custom field in a template
type TemplateField struct {
	Name        string            `yaml:"name" json:"name"`
	Type        TemplateFieldType `yaml:"type" json:"type"`
	Label       string            `yaml:"label" json:"label"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool              `yaml:"required,omitempty" json:"required,omitempty"`
	Default     string            `yaml:"default,omitempty" json:"default,omitempty"`
	Options     []string          `yaml:"options,omitempty" json:"options,omitempty"`
	Validation  *FieldValidation  `yaml:"validation,omitempty" json:"validation,omitempty"`
}

// TemplateFieldType defines the type of a template field
type TemplateFieldType string

const (
	FieldTypeText     TemplateFieldType = "text"
	FieldTypeTextarea TemplateFieldType = "textarea"
	FieldTypeSelect   TemplateFieldType = "select"
	FieldTypeMulti    TemplateFieldType = "multiselect"
	FieldTypeCheckbox TemplateFieldType = "checkbox"
	FieldTypeNumber   TemplateFieldType = "number"
	FieldTypeURL      TemplateFieldType = "url"
	FieldTypeEmail    TemplateFieldType = "email"
)

// FieldValidation defines validation rules for template fields
type FieldValidation struct {
	MinLength int    `yaml:"min_length,omitempty" json:"min_length,omitempty"`
	MaxLength int    `yaml:"max_length,omitempty" json:"max_length,omitempty"`
	Pattern   string `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	Message   string `yaml:"message,omitempty" json:"message,omitempty"`
}

// TemplateAuto defines automation rules for templates
type TemplateAuto struct {
	AssigneeRules     []AssignmentRule `yaml:"assignee_rules,omitempty" json:"assignee_rules,omitempty"`
	LabelRules        []LabelRule      `yaml:"label_rules,omitempty" json:"label_rules,omitempty"`
	WorkflowRules     []WorkflowRule   `yaml:"workflow_rules,omitempty" json:"workflow_rules,omitempty"`
	NotificationRules []NotifyRule     `yaml:"notification_rules,omitempty" json:"notification_rules,omitempty"`
}

// AssignmentRule defines automatic assignee assignment
type AssignmentRule struct {
	Condition string `yaml:"condition" json:"condition"`
	Assignee  string `yaml:"assignee" json:"assignee"`
}

// LabelRule defines automatic label assignment
type LabelRule struct {
	Condition string   `yaml:"condition" json:"condition"`
	Labels    []string `yaml:"labels" json:"labels"`
}

// WorkflowRule defines automatic status transitions
type WorkflowRule struct {
	Trigger   string `yaml:"trigger" json:"trigger"`
	NewStatus Status `yaml:"new_status" json:"new_status"`
	Condition string `yaml:"condition,omitempty" json:"condition,omitempty"`
}

// NotifyRule defines notification automation
type NotifyRule struct {
	Event      string   `yaml:"event" json:"event"`
	Recipients []string `yaml:"recipients" json:"recipients"`
	Message    string   `yaml:"message,omitempty" json:"message,omitempty"`
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
			Available: []string{"bug", "feature", "task", "epic", "improvement"},
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
			DefaultBranchPrefix: "feature",
			BranchConfig: BranchConfig{
				Template:       "{prefix}/{issue}-{title}",
				MaxTitleLength: 50,
				AutoSwitch:     true,
				PrefixByType: map[string]string{
					"bug":         "bugfix",
					"feature":     "feature",
					"task":        "feature",
					"improvement": "feature",
					"hotfix":      "hotfix",
					"epic":        "feature",
				},
				AutoMergeTargets: []string{"main", "master", "develop"},
			},
		},
		UI: UIConfig{
			Colors:      true,
			Interactive: true,
			Pager:       "auto",
			Format:      "table",
		},
		StorageConfig: DefaultStorageConfig(),
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
			Fields: []TemplateField{
				{
					Name:        "reproduction_steps",
					Type:        FieldTypeTextarea,
					Label:       "Steps to Reproduce",
					Description: "Detailed steps to reproduce the bug",
					Required:    true,
					Validation: &FieldValidation{
						MinLength: 10,
						Message:   "Please provide detailed reproduction steps",
					},
				},
				{
					Name:        "environment",
					Type:        FieldTypeSelect,
					Label:       "Environment",
					Description: "Environment where the bug occurs",
					Required:    true,
					Options:     []string{"development", "staging", "production"},
				},
				{
					Name:        "browser",
					Type:        FieldTypeSelect,
					Label:       "Browser",
					Description: "Browser where the bug occurs (if applicable)",
					Options:     []string{"Chrome", "Firefox", "Safari", "Edge", "Other", "N/A"},
				},
				{
					Name:        "severity",
					Type:        FieldTypeSelect,
					Label:       "Severity",
					Description: "How severe is this bug?",
					Required:    true,
					Options:     []string{"low", "medium", "high", "critical"},
					Default:     "medium",
				},
			},
			Automation: TemplateAuto{
				LabelRules: []LabelRule{
					{
						Condition: "severity == 'critical'",
						Labels:    []string{"urgent", "critical"},
					},
					{
						Condition: "environment == 'production'",
						Labels:    []string{"prod-issue"},
					},
				},
				AssigneeRules: []AssignmentRule{
					{
						Condition: "severity == 'critical'",
						Assignee:  "lead-developer",
					},
				},
				WorkflowRules: []WorkflowRule{
					{
						Trigger:   "on_create",
						NewStatus: StatusOpen,
						Condition: "severity != 'critical'",
					},
					{
						Trigger:   "on_create",
						NewStatus: StatusInProgress,
						Condition: "severity == 'critical'",
					},
				},
			},
		}
	case "feature":
		return &Template{
			Name:        "feature",
			Type:        IssueTypeFeature,
			Title:       "Feature Request",
			Description: "## Summary\nA clear and concise description of the feature you'd like to see.\n\n## Motivation\nWhy is this feature important? What problem does it solve?\n\n## Detailed Description\nProvide a detailed description of the feature.\n\n## Acceptance Criteria\n- [ ] Criterion 1\n- [ ] Criterion 2\n- [ ] Criterion 3\n\n## Additional Context\nAdd any other context or screenshots about the feature request here.",
			Labels:      []string{"enhancement"},
			Priority:    PriorityMedium,
			Fields: []TemplateField{
				{
					Name:        "user_story",
					Type:        FieldTypeTextarea,
					Label:       "User Story",
					Description: "As a [user type], I want [functionality] so that [benefit]",
					Required:    true,
					Default:     "As a [user type], I want [functionality] so that [benefit]",
				},
				{
					Name:        "acceptance_criteria",
					Type:        FieldTypeTextarea,
					Label:       "Acceptance Criteria",
					Description: "Define what \"done\" means for this feature",
					Required:    true,
				},
				{
					Name:        "complexity",
					Type:        FieldTypeSelect,
					Label:       "Complexity",
					Description: "Estimated complexity of implementation",
					Options:     []string{"simple", "medium", "complex"},
					Default:     "medium",
				},
				{
					Name:        "target_release",
					Type:        FieldTypeText,
					Label:       "Target Release",
					Description: "Target release version (optional)",
				},
			},
			Automation: TemplateAuto{
				LabelRules: []LabelRule{
					{
						Condition: "complexity == 'complex'",
						Labels:    []string{"complex", "needs-design"},
					},
					{
						Condition: "target_release != ''",
						Labels:    []string{"scheduled"},
					},
				},
				WorkflowRules: []WorkflowRule{
					{
						Trigger:   "on_create",
						NewStatus: StatusOpen,
					},
				},
			},
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
	case "improvement":
		return &Template{
			Name:        "improvement",
			Type:        IssueTypeFeature,
			Title:       "Improvement",
			Description: "## Description\nDescribe the improvement.\n\n## Current Behavior\nDescribe the current behavior.\n\n## Proposed Changes\nDescribe the proposed improvements.",
			Labels:      []string{"enhancement"},
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
