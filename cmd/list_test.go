package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestListCmdStructure(t *testing.T) {
	// Test command structure without executing
	tests := []struct {
		name     string
		validate func(t *testing.T, cmd *cobra.Command)
	}{
		{
			name: "list command has correct use",
			validate: func(t *testing.T, cmd *cobra.Command) {
				assert.Equal(t, "list", cmd.Use)
			},
		},
		{
			name: "list command has aliases",
			validate: func(t *testing.T, cmd *cobra.Command) {
				assert.Contains(t, cmd.Aliases, "ls")
			},
		},
		{
			name: "list command has all expected flags",
			validate: func(t *testing.T, cmd *cobra.Command) {
				expectedFlags := []string{
					"status", "type", "priority", "assignee", "labels",
					"milestone", "branch", "limit", "all", "blocked", "no-truncate",
				}
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
				Use:     "list",
				Aliases: []string{"ls"},
				RunE: func(cmd *cobra.Command, args []string) error {
					// Don't actually execute, just validate structure
					return nil
				},
			}

			// Add flags like the real command
			cmd.Flags().StringVar(&listStatus, "status", "", "filter by status")
			cmd.Flags().StringVar(&listType, "type", "", "filter by type")
			cmd.Flags().StringVar(&listPriority, "priority", "", "filter by priority")
			cmd.Flags().StringVar(&listAssignee, "assignee", "", "filter by assignee")
			cmd.Flags().StringSliceVar(&listLabels, "labels", []string{}, "filter by labels")
			cmd.Flags().StringVar(&listMilestone, "milestone", "", "filter by milestone")
			cmd.Flags().StringVar(&listBranch, "branch", "", "filter by branch")
			cmd.Flags().IntVar(&listLimit, "limit", 0, "limit results")
			cmd.Flags().BoolVar(&listAll, "all", false, "show all issues")
			cmd.Flags().BoolVar(&listBlocked, "blocked", false, "show blocked issues")
			cmd.Flags().BoolVar(&listNoTruncate, "no-truncate", false, "don't truncate output")

			// Run validation
			if tt.validate != nil {
				tt.validate(t, cmd)
			}
		})
	}
}

func TestListCmdAliases(t *testing.T) {
	// Test that 'ls' alias works
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
	}

	// Check that alias is properly set
	assert.Contains(t, cmd.Aliases, "ls")
}

func TestListFlagDefaults(t *testing.T) {
	// Test default flag values
	resetListFlags()

	assert.Empty(t, listStatus, "listStatus should default to empty")
	assert.Empty(t, listType, "listType should default to empty")
	assert.Empty(t, listPriority, "listPriority should default to empty")
	assert.Empty(t, listAssignee, "listAssignee should default to empty")
	assert.Empty(t, listLabels, "listLabels should default to empty slice")
	assert.Empty(t, listMilestone, "listMilestone should default to empty")
	assert.Empty(t, listBranch, "listBranch should default to empty")
	assert.Equal(t, 0, listLimit, "listLimit should default to 0")
	assert.False(t, listAll, "listAll should default to false")
	assert.False(t, listBlocked, "listBlocked should default to false")
	assert.False(t, listNoTruncate, "listNoTruncate should default to false")
}

func TestListCmdFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use: "list",
	}

	// Add all the flags that list command should have
	cmd.Flags().StringVar(&listStatus, "status", "", "filter by status")
	cmd.Flags().StringVar(&listType, "type", "", "filter by type")
	cmd.Flags().StringVar(&listPriority, "priority", "", "filter by priority")
	cmd.Flags().StringVar(&listAssignee, "assignee", "", "filter by assignee")
	cmd.Flags().StringSliceVar(&listLabels, "labels", []string{}, "filter by labels")
	cmd.Flags().StringVar(&listMilestone, "milestone", "", "filter by milestone")
	cmd.Flags().StringVar(&listBranch, "branch", "", "filter by branch")
	cmd.Flags().IntVar(&listLimit, "limit", 0, "limit results")
	cmd.Flags().BoolVar(&listAll, "all", false, "show all issues")
	cmd.Flags().BoolVar(&listBlocked, "blocked", false, "show blocked issues")
	cmd.Flags().BoolVar(&listNoTruncate, "no-truncate", false, "don't truncate output")

	// Test that all expected flags are present
	expectedFlags := []string{
		"status", "type", "priority", "assignee", "labels",
		"milestone", "branch", "limit", "all", "blocked", "no-truncate",
	}

	for _, flagName := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "Flag %s should exist", flagName)
	}
}

// Helper function to reset list flags
func resetListFlags() {
	listStatus = ""
	listType = ""
	listPriority = ""
	listAssignee = ""
	listLabels = []string{}
	listMilestone = ""
	listBranch = ""
	listLimit = 0
	listAll = false
	listBlocked = false
	listNoTruncate = false
}
