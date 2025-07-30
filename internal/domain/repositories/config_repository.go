package repositories

import (
	"context"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// ConfigRepository defines the interface for configuration storage operations
type ConfigRepository interface {
	// Load loads the project configuration
	Load(ctx context.Context) (*entities.Config, error)

	// Save saves the project configuration
	Save(ctx context.Context, config *entities.Config) error

	// GetTemplate retrieves a template by name
	GetTemplate(ctx context.Context, name string) (*entities.Template, error)

	// SaveTemplate saves a custom template
	SaveTemplate(ctx context.Context, template *entities.Template) error

	// ListTemplates returns all available templates
	ListTemplates(ctx context.Context) ([]*entities.Template, error)

	// DeleteTemplate removes a custom template
	DeleteTemplate(ctx context.Context, name string) error

	// Exists checks if the configuration exists
	Exists(ctx context.Context) (bool, error)

	// Initialize creates the initial configuration structure
	Initialize(ctx context.Context, config *entities.Config) error
}
