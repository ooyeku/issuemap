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

	return nil
}
