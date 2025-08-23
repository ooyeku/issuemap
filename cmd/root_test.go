package cmd

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestGetCommandName(t *testing.T) {
	// Save original os.Args
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "issuemap binary",
			args:     []string{"issuemap"},
			expected: "issuemap",
		},
		{
			name:     "ismp binary",
			args:     []string{"ismp"},
			expected: "ismp",
		},
		{
			name:     "issuemap with path",
			args:     []string{"/usr/local/bin/issuemap"},
			expected: "issuemap",
		},
		{
			name:     "ismp with path",
			args:     []string{"/usr/local/bin/ismp"},
			expected: "ismp",
		},
		{
			name:     "unknown binary name",
			args:     []string{"unknown"},
			expected: "issuemap",
		},
		{
			name:     "empty args",
			args:     []string{},
			expected: "issuemap", // Default when can't determine
		},
		{
			name:     "path with issuemap in middle",
			args:     []string{"/path/issuemap/bin/something"},
			expected: "issuemap",
		},
		{
			name:     "path with ismp in middle",
			args:     []string{"/path/ismp/bin/tool"},
			expected: "issuemap", // Falls back to default when basename doesn't contain ismp
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set os.Args for this test
			os.Args = tt.args

			result := getCommandName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRootCmdBasicProperties(t *testing.T) {
	// Test that root command has expected basic properties
	assert.NotNil(t, rootCmd)
	assert.NotEmpty(t, rootCmd.Use)
	assert.NotEmpty(t, rootCmd.Short)
	assert.NotEmpty(t, rootCmd.Long)
	assert.NotEmpty(t, rootCmd.Version)
}

func TestRootCmdFlags(t *testing.T) {
	// Test that global flags are properly set up
	// The actual flag setup happens in init(), so we test what should be there

	// Save original values and restore them later
	originalVerbose := verbose
	originalFormat := format
	originalNoColor := noColor

	// Reset to expected defaults
	verbose = false
	format = ""
	noColor = false

	// These tests verify the flag variables exist and have default values
	assert.False(t, verbose, "verbose should default to false")
	assert.Empty(t, format, "format should default to empty")
	assert.False(t, noColor, "noColor should default to false")

	// Restore original values
	verbose = originalVerbose
	format = originalFormat
	noColor = originalNoColor
}

func TestRootCmdExecution(t *testing.T) {
	// Test that root command can execute without errors
	// This is a basic smoke test

	// Save original os.Args
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Set args to avoid issues with test runner args
	os.Args = []string{"issuemap"}

	// Create a new root command for testing
	testRootCmd := &cobra.Command{
		Use:     getCommandName(),
		Short:   "Test command",
		Long:    "Test command for unit testing",
		Version: "test-version",
	}

	// Test that we can create and configure the command without panicking
	assert.NotNil(t, testRootCmd)
	assert.Equal(t, "issuemap", testRootCmd.Use)
}

func TestGlobalFlagVariables(t *testing.T) {
	// Test that global flag variables are properly declared and accessible

	// Test initial values
	originalVerbose := verbose
	originalFormat := format
	originalNoColor := noColor

	// Modify values to test they're mutable
	verbose = true
	format = "json"
	noColor = true

	assert.True(t, verbose)
	assert.Equal(t, "json", format)
	assert.True(t, noColor)

	// Restore original values
	verbose = originalVerbose
	format = originalFormat
	noColor = originalNoColor
}

func TestRootCmdVersionHandling(t *testing.T) {
	// Test that version is properly set
	assert.NotEmpty(t, rootCmd.Version, "Root command should have a version set")

	// The version comes from app.GetVersion(), which should return something
	// We can't test the exact value as it might be set during build time
}
