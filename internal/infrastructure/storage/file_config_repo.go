package storage

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
)

// FileConfigRepository implements the ConfigRepository interface using file storage
type FileConfigRepository struct {
	basePath string
}

// NewFileConfigRepository creates a new file-based config repository
func NewFileConfigRepository(basePath string) *FileConfigRepository {
	return &FileConfigRepository{
		basePath: basePath,
	}
}

// defaultAgentDoc contains concise guidance for AI agents to use IssueMap.
// It is written to .issuemap/agent.md during initialization if not present.
const defaultAgentDoc = "## IssueMap Agent Guide\n\n" +
	"Audience: AI coding agents automating typical dev workflows in a Git repo.\n\n" +
	"Key facts\n" +
	"- IssueMap is Git-native. Issues are YAML files under .issuemap/issues.\n" +
	"- Run all commands from the Git repo root. Ensure a clean worktree for merges.\n" +
	"- Use non-interactive flags. Avoid prompts. Do not use emojis.\n\n" +
	"Initialize (once per repo)\n\n" +
	"    git init -b main    # if repo not initialized\n" +
	"    issuemap init --name \"<Project Name>\"\n\n" +
	"Daily golden path\n" +
	"1) Create an issue\n\n" +
	"    issuemap create \"<Title>\" --type feature --priority medium --labels a,b\n" +
	"    issuemap list --status open\n\n" +
	"2) Branch for the issue (one branch per issue)\n\n" +
	"    issuemap branch ISSUE-XXX\n\n" +
	"3) Commit work and reference the issue ID\n\n" +
	"    git add -A\n" +
	"    git commit -m \"ISSUE-XXX: short summary\"\n\n" +
	"4) Optional time tracking\n\n" +
	"    issuemap start ISSUE-XXX\n" +
	"    # ...work...\n" +
	"    issuemap stop ISSUE-XXX\n\n" +
	"5) Keep in sync (derive status, links)\n\n" +
	"    issuemap sync --auto-update\n" +
	"    issuemap show ISSUE-XXX\n\n" +
	"6) Merge and close\n\n" +
	"    # From the feature branch\n" +
	"    issuemap merge\n" +
	"    # Or from main\n" +
	"    issuemap merge ISSUE-XXX\n\n" +
	"7) Housekeeping\n\n" +
	"    git branch -d <feature/ISSUE-XXX-short-title>\n" +
	"    issuemap list --status open\n\n" +
	"Common operations (non-interactive)\n" +
	"- Create with template:\n\n" +
	"    issuemap create \"Hotfix: CSRF token mismatch\" --template hotfix\n\n" +
	"- Edit fields:\n\n" +
	"    issuemap edit ISSUE-XXX --status in-progress --assignee alice --labels auth,backend\n\n" +
	"- Dependencies:\n\n" +
	"    issuemap depend ISSUE-B --on ISSUE-A\n" +
	"    issuemap deps ISSUE-B --graph\n\n" +
	"- Search (query DSL):\n\n" +
	"    issuemap search \"type:bug AND priority:high AND updated:<7d\"\n\n" +
	"- Bulk update (query-driven):\n\n" +
	"    issuemap bulk --query \"label:frontend AND status:open\" --set status=review\n\n" +
	"Conventions\n" +
	"- IDs: either ISSUE-003 or 003 (both accepted). Prefer the full form in commits.\n" +
	"- Commits: prefix messages with the ID: ISSUE-003: message.\n" +
	"- Branch names: created by the branch command and recorded on the issue.\n\n" +
	"Server (optional)\n" +
	"- A local server provides a REST API; not required for CLI flows.\n\n" +
	"    issuemap server start\n" +
	"    issuemap server status\n\n" +
	"Safety & etiquette for agents\n" +
	"- Always run from repo root; verify with: git rev-parse --show-toplevel\n" +
	"- Prefer explicit flags; avoid interactive prompts.\n" +
	"- After creation/edits, verify with: issuemap show ISSUE-XXX\n" +
	"- Before merge, ensure no unstaged changes; commit .issuemap updates if needed:\n\n" +
	"    git add .issuemap && git commit -m \"Update issues\"\n\n" +
	"Quick reference\n\n" +
	"    issuemap create \"Title\" --type feature --priority medium\n" +
	"    issuemap branch ISSUE-123\n" +
	"    git commit -m \"ISSUE-123: change\"\n" +
	"    issuemap sync --auto-update\n" +
	"    issuemap merge\n"

// Load loads the project configuration
func (r *FileConfigRepository) Load(ctx context.Context) (*entities.Config, error) {
	configPath := filepath.Join(r.basePath, "config.yaml")

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrap(errors.ErrConfigNotFound, "FileConfigRepository.Load", "not_found")
		}
		return nil, errors.Wrap(err, "FileConfigRepository.Load", "read")
	}

	var config entities.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrap(err, "FileConfigRepository.Load", "unmarshal")
	}

	return &config, nil
}

// Save saves the project configuration
func (r *FileConfigRepository) Save(ctx context.Context, config *entities.Config) error {
	configPath := filepath.Join(r.basePath, "config.yaml")

	// Ensure templates list includes improvement
	ensure := map[string]bool{}
	for _, n := range config.Templates.Available {
		ensure[n] = true
	}
	if !ensure["improvement"] {
		config.Templates.Available = append(config.Templates.Available, "improvement")
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return errors.Wrap(err, "FileConfigRepository.Save", "marshal")
	}

	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return errors.Wrap(err, "FileConfigRepository.Save", "write")
	}

	return nil
}

// GetTemplate retrieves a template by name
func (r *FileConfigRepository) GetTemplate(ctx context.Context, name string) (*entities.Template, error) {
	// First try to load from custom templates
	templatePath := filepath.Join(r.basePath, "templates", fmt.Sprintf("%s.yaml", name))

	if data, err := ioutil.ReadFile(templatePath); err == nil {
		var template entities.Template
		if err := yaml.Unmarshal(data, &template); err == nil {
			return &template, nil
		}
	}

	// Fall back to built-in templates
	config, err := r.Load(ctx)
	if err != nil {
		// If config doesn't exist, use default config for built-in templates
		config = entities.NewDefaultConfig()
	}

	template := config.GetTemplate(name)
	if template.Name == "default" && name != "default" {
		return nil, errors.Wrap(errors.ErrTemplateNotFound, "FileConfigRepository.GetTemplate", "not_found")
	}

	return template, nil
}

// SaveTemplate saves a custom template
func (r *FileConfigRepository) SaveTemplate(ctx context.Context, template *entities.Template) error {
	templatesDir := filepath.Join(r.basePath, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return errors.Wrap(err, "FileConfigRepository.SaveTemplate", "mkdir")
	}

	templatePath := filepath.Join(templatesDir, fmt.Sprintf("%s.yaml", template.Name))

	data, err := yaml.Marshal(template)
	if err != nil {
		return errors.Wrap(err, "FileConfigRepository.SaveTemplate", "marshal")
	}

	if err := ioutil.WriteFile(templatePath, data, 0644); err != nil {
		return errors.Wrap(err, "FileConfigRepository.SaveTemplate", "write")
	}

	return nil
}

// ListTemplates returns all available templates
func (r *FileConfigRepository) ListTemplates(ctx context.Context) ([]*entities.Template, error) {
	var templates []*entities.Template

	// Add built-in templates
	config, err := r.Load(ctx)
	if err != nil {
		config = entities.NewDefaultConfig()
	}

	for _, templateName := range config.Templates.Available {
		template := config.GetTemplate(templateName)
		templates = append(templates, template)
	}

	// Ensure new built-ins are included even if older configs are present
	ensureNames := []string{"improvement"}
	existing := make(map[string]bool)
	for _, t := range templates {
		existing[t.Name] = true
	}
	for _, name := range ensureNames {
		if !existing[name] {
			templates = append(templates, config.GetTemplate(name))
		}
	}

	// Add custom templates
	templatesDir := filepath.Join(r.basePath, "templates")
	if files, err := ioutil.ReadDir(templatesDir); err == nil {
		for _, file := range files {
			if !file.IsDir() && filepath.Ext(file.Name()) == ".yaml" {
				templatePath := filepath.Join(templatesDir, file.Name())
				if data, err := ioutil.ReadFile(templatePath); err == nil {
					var template entities.Template
					if err := yaml.Unmarshal(data, &template); err == nil {
						templates = append(templates, &template)
					}
				}
			}
		}
	}

	return templates, nil
}

// DeleteTemplate removes a custom template
func (r *FileConfigRepository) DeleteTemplate(ctx context.Context, name string) error {
	templatePath := filepath.Join(r.basePath, "templates", fmt.Sprintf("%s.yaml", name))

	if err := os.Remove(templatePath); err != nil {
		if os.IsNotExist(err) {
			return errors.Wrap(errors.ErrTemplateNotFound, "FileConfigRepository.DeleteTemplate", "not_found")
		}
		return errors.Wrap(err, "FileConfigRepository.DeleteTemplate", "remove")
	}

	return nil
}

// Exists checks if the configuration exists
func (r *FileConfigRepository) Exists(ctx context.Context) (bool, error) {
	configPath := filepath.Join(r.basePath, "config.yaml")
	_, err := os.Stat(configPath)

	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, errors.Wrap(err, "FileConfigRepository.Exists", "stat")
}

// Initialize creates the initial configuration structure
func (r *FileConfigRepository) Initialize(ctx context.Context, config *entities.Config) error {
	// Create base directory
	if err := os.MkdirAll(r.basePath, 0755); err != nil {
		return errors.Wrap(err, "FileConfigRepository.Initialize", "mkdir_base")
	}

	// Create subdirectories
	dirs := []string{"issues", "templates", "metadata"}
	for _, dir := range dirs {
		dirPath := filepath.Join(r.basePath, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return errors.Wrap(err, "FileConfigRepository.Initialize", "mkdir_sub")
		}
	}

	// Save the configuration
	if err := r.Save(ctx, config); err != nil {
		return errors.Wrap(err, "FileConfigRepository.Initialize", "save_config")
	}

	// Write default agent guide if not present
	agentDocPath := filepath.Join(r.basePath, "agent.md")
	if _, err := os.Stat(agentDocPath); os.IsNotExist(err) {
		if writeErr := ioutil.WriteFile(agentDocPath, []byte(defaultAgentDoc), 0644); writeErr != nil {
			// Non-fatal: keep initialization successful even if guide fails to write
		}
	}

	return nil
}
