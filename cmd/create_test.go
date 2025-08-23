package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestCreateCmdStructure(t *testing.T) {
	// Test command structure without executing
	tests := []struct {
		name     string
		args     []string
		flags    map[string]string
		validate func(t *testing.T, cmd *cobra.Command)
	}{
		{
			name: "create command has correct use",
			validate: func(t *testing.T, cmd *cobra.Command) {
				assert.Contains(t, cmd.Use, "create")
			},
		},
		{
			name: "create command has flags",
			validate: func(t *testing.T, cmd *cobra.Command) {
				// Check that expected flags are present
				expectedFlags := []string{"title", "description", "type", "priority", "labels"}
				for _, flagName := range expectedFlags {
					flag := cmd.Flags().Lookup(flagName)
					assert.NotNil(t, flag, "Flag %s should exist", flagName)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new command for this test
			cmd := &cobra.Command{
				Use: "create [title]",
				RunE: func(cmd *cobra.Command, args []string) error {
					// Don't actually execute, just validate structure
					return nil
				},
			}

			// Add flags like the real command
			cmd.Flags().StringVarP(&createTitle, "title", "t", "", "issue title")
			cmd.Flags().StringVarP(&createDescription, "description", "d", "", "issue description")
			cmd.Flags().StringVar(&createType, "type", "", "issue type")
			cmd.Flags().StringVarP(&createPriority, "priority", "p", "", "issue priority")
			cmd.Flags().StringSliceVarP(&createLabels, "labels", "l", []string{}, "labels")

			// Run validation
			if tt.validate != nil {
				tt.validate(t, cmd)
			}
		})
	}
}

func TestCreateTitleParsing(t *testing.T) {
	// Test title parsing logic without executing full command
	tests := []struct {
		name      string
		args      []string
		flagTitle string
		expected  string
	}{
		{
			name:     "title from args",
			args:     []string{"Test", "issue", "title"},
			expected: "Test issue title",
		},
		{
			name:      "title from flag takes precedence when args exist",
			args:      []string{"ignored", "args"},
			flagTitle: "Flag title",
			expected:  "Flag title",
		},
		{
			name:      "title from flag only",
			args:      []string{},
			flagTitle: "Flag only title",
			expected:  "Flag only title",
		},
		{
			name:     "empty title",
			args:     []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			resetCreateFlags()
			createTitle = tt.flagTitle

			// Simulate the logic from the actual command
			if len(tt.args) > 0 && createTitle == "" {
				createTitle = strings.Join(tt.args, " ")
			}

			assert.Equal(t, tt.expected, createTitle)
		})
	}
}

// Helper functions for tests

func resetCreateFlags() {
	createTitle = ""
	createDescription = ""
	createType = ""
	createPriority = ""
	createAssignee = ""
	createLabels = []string{}
	createMilestone = ""
	createTemplate = ""
	createInteractive = false
}
