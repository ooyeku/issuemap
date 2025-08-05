package integration

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

// TestTemplateManagement tests comprehensive template functionality
func (suite *IntegrationTestSuite) TestTemplateManagement() {
	suite.T().Run("TemplateList", func(t *testing.T) {
		// Test listing built-in templates
		output := suite.runCLICommandWithOutput("template", "list")

		// Should contain built-in templates
		assert.Contains(t, output, "bug")
		assert.Contains(t, output, "feature")
		assert.Contains(t, output, "task")
		assert.Contains(t, output, "improvement")
	})

	suite.T().Run("TemplateShow", func(t *testing.T) {
		// Test showing a built-in template
		output := suite.runCLICommandWithOutput("template", "show", "bug")

		assert.Contains(t, output, "Name: bug")
		assert.Contains(t, output, "Type: bug")
		assert.Contains(t, output, "Priority:")
	})

	suite.T().Run("TemplateCreate", func(t *testing.T) {
		// Create a custom template
		suite.runCLICommand("template", "create", "hotfix",
			"--type", "bug",
			"--priority", "critical",
			"--title", "Hotfix: {summary}",
			"--description", "Critical production issue requiring immediate attention",
			"--labels", "hotfix,urgent")

		// Verify template was created
		output := suite.runCLICommandWithOutput("template", "show", "hotfix")
		assert.Contains(t, output, "Name: hotfix")
		assert.Contains(t, output, "Type: bug")
		assert.Contains(t, output, "Priority: critical")
		assert.Contains(t, output, "hotfix")
		assert.Contains(t, output, "urgent")
	})

	suite.T().Run("TemplateValidation", func(t *testing.T) {
		// Test validating a template
		suite.runCLICommand("template", "validate", "hotfix")

		// Test validation of non-existent template should fail
		_, err := suite.runCLICommandWithError("template", "validate", "non-existent-template")
		assert.Error(t, err, "Should fail for non-existent template")
	})

	suite.T().Run("TemplateExportYAML", func(t *testing.T) {
		exportFile := filepath.Join(suite.testDir, "exported-hotfix.yaml")

		// Export template to YAML
		suite.runCLICommand("template", "export", "hotfix", exportFile)

		// Verify file was created
		assert.FileExists(t, exportFile)

		// Verify content
		data, err := ioutil.ReadFile(exportFile)
		require.NoError(t, err)

		var exportData map[string]interface{}
		err = yaml.Unmarshal(data, &exportData)
		require.NoError(t, err)

		// Check structure
		assert.Contains(t, exportData, "template")
		assert.Contains(t, exportData, "metadata")

		template := exportData["template"].(map[string]interface{})
		assert.Equal(t, "hotfix", template["name"])
		assert.Equal(t, "bug", template["type"])
		assert.Equal(t, "critical", template["priority"])
	})

	suite.T().Run("TemplateExportJSON", func(t *testing.T) {
		exportFile := filepath.Join(suite.testDir, "exported-hotfix.json")

		// Export template to JSON
		suite.runCLICommand("template", "export", "hotfix", exportFile, "--format", "json")

		// Verify file was created
		assert.FileExists(t, exportFile)

		// Verify content
		data, err := ioutil.ReadFile(exportFile)
		require.NoError(t, err)

		var exportData map[string]interface{}
		err = json.Unmarshal(data, &exportData)
		require.NoError(t, err)

		// Check structure
		assert.Contains(t, exportData, "template")
		assert.Contains(t, exportData, "metadata")
	})

	suite.T().Run("TemplateImport", func(t *testing.T) {
		// Create a test template file
		testTemplate := entities.Template{
			Name:        "imported-template",
			Type:        entities.IssueTypeFeature,
			Title:       "Imported Feature: {title}",
			Description: "This template was imported from a file",
			Priority:    entities.PriorityMedium,
			Labels:      []string{"imported", "feature"},
		}

		exportData := map[string]interface{}{
			"template": testTemplate,
			"metadata": map[string]interface{}{
				"version": "1.0",
				"source":  "test",
			},
		}

		importFile := filepath.Join(suite.testDir, "import-template.yaml")
		data, err := yaml.Marshal(exportData)
		require.NoError(t, err)

		err = ioutil.WriteFile(importFile, data, 0644)
		require.NoError(t, err)

		// Import the template
		suite.runCLICommand("template", "import", importFile)

		// Verify template was imported
		output := suite.runCLICommandWithOutput("template", "show", "imported-template")
		assert.Contains(t, output, "Name: imported-template")
		assert.Contains(t, output, "Type: feature")
		assert.Contains(t, output, "imported")
	})

	suite.T().Run("TemplateImportWithOverwrite", func(t *testing.T) {
		// Create a template file with same name as existing
		testTemplate := entities.Template{
			Name:        "hotfix",
			Type:        entities.IssueTypeTask,
			Title:       "Overwritten Template",
			Description: "This template overwrites the existing one",
			Priority:    entities.PriorityLow,
			Labels:      []string{"overwritten"},
		}

		importFile := filepath.Join(suite.testDir, "overwrite-template.yaml")
		data, err := yaml.Marshal(map[string]interface{}{"template": testTemplate})
		require.NoError(t, err)

		err = ioutil.WriteFile(importFile, data, 0644)
		require.NoError(t, err)

		// Should fail without --overwrite flag
		_, err = suite.runCLICommandWithError("template", "import", importFile)
		assert.Error(t, err, "Should fail without overwrite flag")

		// Should succeed with --overwrite flag
		suite.runCLICommand("template", "import", importFile, "--overwrite")

		// Verify template was overwritten
		output := suite.runCLICommandWithOutput("template", "show", "hotfix")
		assert.Contains(t, output, "Type: task")
		assert.Contains(t, output, "overwritten")
	})

	suite.T().Run("TemplateDelete", func(t *testing.T) {
		// Delete the custom template
		suite.runCLICommand("template", "delete", "imported-template")

		// Verify template was deleted
		_, err := suite.runCLICommandWithError("template", "show", "imported-template")
		assert.Error(t, err, "Should fail after deletion")
	})

	suite.T().Run("TemplateUsageInIssueCreation", func(t *testing.T) {
		// Create issue using custom template
		suite.runCLICommand("create", "Critical production bug", "--template", "hotfix")

		// Verify issue was created with template values
		issues := suite.getAllIssues()
		require.Len(t, issues, 1)

		issue := issues[0]
		assert.Equal(t, "Critical production bug", issue.Title)
		assert.Equal(t, "task", issue.Type)    // From overwritten template
		assert.Equal(t, "low", issue.Priority) // From overwritten template
		assert.Contains(t, issue.Labels, "overwritten")
	})
}

// TestTemplateValidationRules tests template validation edge cases
func (suite *IntegrationTestSuite) TestTemplateValidationRules() {
	suite.T().Run("InvalidTemplateType", func(t *testing.T) {
		// Try to create template with invalid type
		_, err := suite.runCLICommandWithError("template", "create", "invalid-type-template",
			"--type", "invalid-type")
		assert.Error(t, err, "Should fail with invalid type")
	})

	suite.T().Run("InvalidTemplatePriority", func(t *testing.T) {
		// Try to create template with invalid priority
		_, err := suite.runCLICommandWithError("template", "create", "invalid-priority-template",
			"--priority", "invalid-priority")
		assert.Error(t, err, "Should fail with invalid priority")
	})

	suite.T().Run("EmptyTemplateName", func(t *testing.T) {
		// Try to create template without name
		_, err := suite.runCLICommandWithError("template", "create", "")
		assert.Error(t, err, "Should fail with empty name")
	})
}

// TestTemplateFieldCustomization tests custom field functionality
func (suite *IntegrationTestSuite) TestTemplateFieldCustomization() {
	suite.T().Run("TemplateWithCustomFields", func(t *testing.T) {
		// Create template with custom fields
		suite.runCLICommand("template", "create", "detailed-bug",
			"--type", "bug",
			"--fields", "reproduction_steps:text:Steps to reproduce the issue",
			"--fields", "expected_behavior:text:What should happen",
			"--fields", "actual_behavior:text:What actually happens",
			"--fields", "browser:select:Browser used",
			"--fields", "severity:number:Severity rating")

		// Verify template has custom fields
		output := suite.runCLICommandWithOutput("template", "show", "detailed-bug")
		assert.Contains(t, output, "Custom Fields")
		assert.Contains(t, output, "reproduction_steps")
		assert.Contains(t, output, "expected_behavior")
		assert.Contains(t, output, "browser")
	})

	suite.T().Run("ExportImportCustomFields", func(t *testing.T) {
		exportFile := filepath.Join(suite.testDir, "detailed-bug.yaml")

		// Export template with custom fields
		suite.runCLICommand("template", "export", "detailed-bug", exportFile)

		// Delete original
		suite.runCLICommand("template", "delete", "detailed-bug")

		// Import it back
		suite.runCLICommand("template", "import", exportFile)

		// Verify custom fields are preserved
		output := suite.runCLICommandWithOutput("template", "show", "detailed-bug")
		assert.Contains(t, output, "reproduction_steps")
		assert.Contains(t, output, "expected_behavior")
	})
}

// TestTemplateInteractiveMode tests interactive template creation
func (suite *IntegrationTestSuite) TestTemplateInteractiveMode() {
	// Note: Interactive mode testing would require input simulation
	// This is a placeholder for when we implement proper interactive testing
	suite.T().Skip("Interactive mode testing requires input simulation setup")
}

// Helper function to run CLI command and capture output
func (suite *IntegrationTestSuite) runCLICommandWithOutput(args ...string) string {
	cmd := suite.buildCommand(args...)
	output, err := cmd.CombinedOutput()
	require.NoError(suite.T(), err, "CLI command failed: %s\nOutput: %s", strings.Join(args, " "), string(output))
	return string(output)
}

// Helper function to run CLI command expecting an error
func (suite *IntegrationTestSuite) runCLICommandWithError(args ...string) (string, error) {
	cmd := suite.buildCommand(args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// Helper function to build command
func (suite *IntegrationTestSuite) buildCommand(args ...string) *exec.Cmd {
	cmd := exec.Command(suite.binaryPath, args...)
	cmd.Dir = suite.testDir
	return cmd
}
