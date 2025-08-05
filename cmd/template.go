package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	templateName        string
	templateType        string
	templateTitle       string
	templateDescription string
	templateLabels      []string
	templatePriority    string
	templateFields      []string
	templateInteractive bool
	templateFormat      string
	templateOverwrite   bool
)

// templateCmd represents the template command
var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage issue templates",
	Long: `Manage issue templates for creating standardized issues.

Templates help ensure consistency across issues by providing predefined
structures, fields, and default values.

Examples:
  issuemap template list                    # List all templates
  issuemap template show bug               # Show bug template details
  issuemap template create custom-bug     # Create custom template
  issuemap template validate bug          # Validate template`,
}

// templateListCmd lists all available templates
var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available templates",
	Long: `List all available issue templates including built-in and custom templates.

Examples:
  issuemap template list
  issuemap template list --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTemplateList(cmd, args)
	},
}

// templateShowCmd shows template details
var templateShowCmd = &cobra.Command{
	Use:   "show <template-name>",
	Short: "Show template details",
	Long: `Show detailed information about a specific template including
its structure, fields, and default values.

Examples:
  issuemap template show bug
  issuemap template show feature`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTemplateShow(cmd, args)
	},
}

// templateCreateCmd creates a new custom template
var templateCreateCmd = &cobra.Command{
	Use:   "create <template-name>",
	Short: "Create a new custom template",
	Long: `Create a new custom issue template with specified fields and defaults.

Examples:
  issuemap template create hotfix --type bug --priority critical
  issuemap template create user-story --type feature --interactive
  issuemap template create --interactive`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		if len(args) > 0 {
			name = args[0]
		}
		return runTemplateCreate(cmd, name)
	},
}

// templateValidateCmd validates a template
var templateValidateCmd = &cobra.Command{
	Use:   "validate <template-name>",
	Short: "Validate a template structure",
	Long: `Validate that a template has proper structure and required fields.

Examples:
  issuemap template validate bug
  issuemap template validate my-custom-template`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTemplateValidate(cmd, args)
	},
}

// templateDeleteCmd deletes a custom template
var templateDeleteCmd = &cobra.Command{
	Use:   "delete <template-name>",
	Short: "Delete a custom template",
	Long: `Delete a custom template. Built-in templates cannot be deleted.

Examples:
  issuemap template delete my-custom-template`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTemplateDelete(cmd, args)
	},
}

// templateExportCmd exports a template to a file
var templateExportCmd = &cobra.Command{
	Use:   "export <template-name> [output-file]",
	Short: "Export a template to a file",
	Long: `Export a template to a YAML file for sharing or backup.
If no output file is specified, exports to <template-name>.yaml

Examples:
  issuemap template export bug bug-template.yaml
  issuemap template export my-custom-template
  issuemap template export feature --format json`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTemplateExport(cmd, args)
	},
}

// templateImportCmd imports a template from a file
var templateImportCmd = &cobra.Command{
	Use:   "import <file-path> [template-name]",
	Short: "Import a template from a file",
	Long: `Import a template from a YAML or JSON file.
If no template name is specified, uses the name from the file.

Examples:
  issuemap template import bug-template.yaml
  issuemap template import shared-template.yaml my-template
  issuemap template import --overwrite existing-template.yaml`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTemplateImport(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(templateCmd)

	// Add subcommands
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateShowCmd)
	templateCmd.AddCommand(templateCreateCmd)
	templateCmd.AddCommand(templateValidateCmd)
	templateCmd.AddCommand(templateDeleteCmd)
	templateCmd.AddCommand(templateExportCmd)
	templateCmd.AddCommand(templateImportCmd)

	// Template create flags
	templateCreateCmd.Flags().StringVar(&templateType, "type", "task", "default issue type (bug, feature, task, epic)")
	templateCreateCmd.Flags().StringVar(&templateTitle, "title", "", "default title template")
	templateCreateCmd.Flags().StringVar(&templateDescription, "description", "", "default description template")
	templateCreateCmd.Flags().StringSliceVar(&templateLabels, "labels", []string{}, "default labels")
	templateCreateCmd.Flags().StringVar(&templatePriority, "priority", "medium", "default priority")
	templateCreateCmd.Flags().StringSliceVar(&templateFields, "fields", []string{}, "custom fields (key:type:description)")
	templateCreateCmd.Flags().BoolVarP(&templateInteractive, "interactive", "i", false, "interactive template creation")

	// Template export flags
	templateExportCmd.Flags().StringVarP(&templateFormat, "format", "f", "yaml", "export format (yaml, json)")

	// Template import flags
	templateImportCmd.Flags().BoolVar(&templateOverwrite, "overwrite", false, "overwrite existing template")

	// Global template flags
	templateCmd.PersistentFlags().StringVarP(&templateFormat, "format", "f", "table", "output format (table, json, yaml)")
}

func runTemplateList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize repositories
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	// Get all templates
	templates, err := configRepo.ListTemplates(ctx)
	if err != nil {
		printError(fmt.Errorf("failed to list templates: %w", err))
		return err
	}

	if templateFormat == "json" || templateFormat == "yaml" {
		return outputTemplatesStructured(templates, templateFormat)
	}

	// Display templates in table format
	printSectionHeader("Available Templates")
	fmt.Printf("%-15s %-10s %-10s %-30s\n", "NAME", "TYPE", "PRIORITY", "DESCRIPTION")
	fmt.Printf("%-15s %-10s %-10s %-30s\n", "────────────", "────────", "────────", "──────────────────────────────")

	for _, template := range templates {
		description := strings.Split(template.Description, "\n")[0] // First line only
		if len(description) > 30 {
			description = description[:27] + "..."
		}

		fmt.Printf("%-15s %-10s %-10s %-30s\n",
			colorValue(template.Name),
			colorType(template.Type),
			colorPriority(template.Priority),
			description)
	}

	fmt.Printf("\nUse 'issuemap template show <name>' for detailed information\n")
	return nil
}

func runTemplateShow(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	templateName := args[0]

	// Initialize repositories
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	// Get template
	template, err := configRepo.GetTemplate(ctx, templateName)
	if err != nil {
		printError(fmt.Errorf("failed to get template '%s': %w", templateName, err))
		return err
	}

	if templateFormat == "json" || templateFormat == "yaml" {
		return outputTemplateStructured(template, templateFormat)
	}

	// Display template details
	printSectionHeader(fmt.Sprintf("Template: %s", template.Name))

	fmt.Printf("%-12s %s\n", "Name:", colorValue(template.Name))
	fmt.Printf("%-12s %s\n", "Type:", colorType(template.Type))
	fmt.Printf("%-12s %s\n", "Priority:", colorPriority(template.Priority))

	if len(template.Labels) > 0 {
		fmt.Printf("%-12s %s\n", "Labels:", strings.Join(template.Labels, ", "))
	}

	if template.Title != "" {
		fmt.Printf("\n")
		printSectionHeader("Default Title")
		fmt.Printf("%s\n", template.Title)
	}

	if template.Description != "" {
		fmt.Printf("\n")
		printSectionHeader("Description Template")
		fmt.Printf("%s\n", template.Description)
	}

	return nil
}

func runTemplateCreate(cmd *cobra.Command, name string) error {
	ctx := context.Background()

	if templateInteractive || name == "" {
		return runInteractiveTemplateCreate(ctx)
	}

	// Initialize repositories
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	// Create template
	template := &entities.Template{
		Name:        name,
		Type:        entities.IssueType(templateType),
		Title:       templateTitle,
		Description: templateDescription,
		Labels:      templateLabels,
		Priority:    entities.Priority(templatePriority),
	}

	// Validate template
	if err := validateTemplate(template); err != nil {
		printError(fmt.Errorf("template validation failed: %w", err))
		return err
	}

	// Save template
	if err := configRepo.SaveTemplate(ctx, template); err != nil {
		printError(fmt.Errorf("failed to save template: %w", err))
		return err
	}

	printSuccess(fmt.Sprintf("Template '%s' created successfully", name))
	return nil
}

func runTemplateValidate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	templateName := args[0]

	// Initialize repositories
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	// Get template
	template, err := configRepo.GetTemplate(ctx, templateName)
	if err != nil {
		printError(fmt.Errorf("failed to get template '%s': %w", templateName, err))
		return err
	}

	// Validate template
	if err := validateTemplate(template); err != nil {
		printError(fmt.Errorf("template validation failed: %w", err))
		return err
	}

	printSuccess(fmt.Sprintf("Template '%s' is valid", templateName))
	return nil
}

func runTemplateDelete(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	templateName := args[0]

	// Check if it's a built-in template
	builtInTemplates := []string{"bug", "feature", "task", "epic"}
	for _, builtIn := range builtInTemplates {
		if templateName == builtIn {
			printError(fmt.Errorf("cannot delete built-in template '%s'", templateName))
			return fmt.Errorf("built-in template")
		}
	}

	// Initialize repositories
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	// Delete template
	if err := configRepo.DeleteTemplate(ctx, templateName); err != nil {
		printError(fmt.Errorf("failed to delete template '%s': %w", templateName, err))
		return err
	}

	printSuccess(fmt.Sprintf("Template '%s' deleted successfully", templateName))
	return nil
}

func runInteractiveTemplateCreate(ctx context.Context) error {
	printInfo("Creating a new template interactively...")

	// TODO: Implement interactive template creation
	printInfo("Interactive template creation not yet implemented")
	printInfo("Use flags to create templates: issuemap template create <name> --type <type> --title <title>")

	return nil
}

func validateTemplate(template *entities.Template) error {
	if template.Name == "" {
		return fmt.Errorf("template name is required")
	}

	validTypes := []entities.IssueType{
		entities.IssueTypeBug,
		entities.IssueTypeFeature,
		entities.IssueTypeTask,
		entities.IssueTypeEpic,
	}

	validType := false
	for _, vt := range validTypes {
		if template.Type == vt {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("invalid issue type: %s (must be one of: bug, feature, task, epic)", template.Type)
	}

	validPriorities := []entities.Priority{
		entities.PriorityLow,
		entities.PriorityMedium,
		entities.PriorityHigh,
		entities.PriorityCritical,
	}

	validPriority := false
	for _, vp := range validPriorities {
		if template.Priority == vp {
			validPriority = true
			break
		}
	}
	if !validPriority {
		return fmt.Errorf("invalid priority: %s (must be one of: low, medium, high, critical)", template.Priority)
	}

	return nil
}

func outputTemplatesStructured(templates []*entities.Template, format string) error {
	switch format {
	case "json":
		return outputJSON(templates)
	case "yaml":
		return outputYAML(templates)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func outputTemplateStructured(template *entities.Template, format string) error {
	switch format {
	case "json":
		return outputJSON(template)
	case "yaml":
		return outputYAML(template)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func outputJSON(data interface{}) error {
	// Simple JSON output - in a real implementation you'd use proper JSON marshaling
	fmt.Printf("JSON output not yet implemented\n")
	return nil
}

func outputYAML(data interface{}) error {
	output, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	fmt.Print(string(output))
	return nil
}

func runTemplateExport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	templateName := args[0]

	// Determine output file
	var outputFile string
	if len(args) > 1 {
		outputFile = args[1]
	} else {
		ext := "yaml"
		if templateFormat == "json" {
			ext = "json"
		}
		outputFile = fmt.Sprintf("%s.%s", templateName, ext)
	}

	// Initialize repositories
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	// Get the template
	template, err := configRepo.GetTemplate(ctx, templateName)
	if err != nil {
		printError(fmt.Errorf("failed to get template '%s': %w", templateName, err))
		return err
	}

	// Create export data with metadata
	exportData := map[string]interface{}{
		"template": template,
		"metadata": map[string]interface{}{
			"exported_at": time.Now().Format(time.RFC3339),
			"version":     "1.0",
			"source":      "issuemap",
		},
	}

	// Marshal to requested format
	var data []byte
	switch templateFormat {
	case "json":
		data, err = json.MarshalIndent(exportData, "", "  ")
	case "yaml":
		data, err = yaml.Marshal(exportData)
	default:
		return fmt.Errorf("unsupported export format: %s", templateFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	// Write to file
	err = ioutil.WriteFile(outputFile, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write template to file: %w", err)
	}

	printSuccess(fmt.Sprintf("Template '%s' exported to '%s'", templateName, outputFile))
	return nil
}

func runTemplateImport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	filePath := args[0]

	// Read the file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	// Parse the file
	var importData map[string]interface{}

	// Try YAML first, then JSON
	err = yaml.Unmarshal(data, &importData)
	if err != nil {
		err = json.Unmarshal(data, &importData)
		if err != nil {
			return fmt.Errorf("failed to parse template file (not valid YAML or JSON): %w", err)
		}
	}

	// Extract template data
	templateData, exists := importData["template"]
	if !exists {
		// Maybe it's a direct template file without metadata wrapper
		templateData = importData
	}

	// Convert to template struct
	templateBytes, err := yaml.Marshal(templateData)
	if err != nil {
		return fmt.Errorf("failed to process template data: %w", err)
	}

	var template entities.Template
	err = yaml.Unmarshal(templateBytes, &template)
	if err != nil {
		return fmt.Errorf("failed to parse template structure: %w", err)
	}

	// Determine template name
	if len(args) > 1 {
		template.Name = args[1]
	}

	if template.Name == "" {
		return fmt.Errorf("template name is required (either specify as argument or include in template file)")
	}

	// Validate template
	err = validateTemplate(&template)
	if err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	// Initialize repositories
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	// Check if template already exists
	if !templateOverwrite {
		existing, err := configRepo.GetTemplate(ctx, template.Name)
		if err == nil && existing != nil {
			return fmt.Errorf("template '%s' already exists. Use --overwrite to replace it", template.Name)
		}
	}

	// Save the template
	err = configRepo.SaveTemplate(ctx, &template)
	if err != nil {
		return fmt.Errorf("failed to save template: %w", err)
	}

	printSuccess(fmt.Sprintf("Template '%s' imported successfully", template.Name))

	// Show template details
	fmt.Println()
	printSectionHeader("Template Details:")
	fmt.Printf("  Name: %s\n", template.Name)
	fmt.Printf("  Type: %s\n", template.Type)
	fmt.Printf("  Priority: %s\n", template.Priority)
	if len(template.Labels) > 0 {
		fmt.Printf("  Labels: %s\n", strings.Join(template.Labels, ", "))
	}
	if len(template.Fields) > 0 {
		fmt.Printf("  Custom Fields: %d\n", len(template.Fields))
	}

	return nil
}
