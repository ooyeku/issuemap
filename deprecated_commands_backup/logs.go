package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
)

var (
	logsJSON      bool
	logsOperation string
	logsSince     string
	logsLimit     int
)

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View cleanup operation logs",
	Long: `View logs of cleanup and restore operations.

This command displays a history of all cleanup operations, including
regular cleanups, admin targeted cleanups, and restore operations.

Examples:
  issuemap logs                              # Show recent cleanup logs
  issuemap logs --limit 50                   # Show last 50 log entries
  issuemap logs --since 2024-01-01          # Show logs since date
  issuemap logs --operation admin_cleanup   # Show only admin cleanup logs
  issuemap logs --json                      # JSON output`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLogs(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)

	// Logs flags
	logsCmd.Flags().BoolVar(&logsJSON, "json", false, "output in JSON format")
	logsCmd.Flags().StringVar(&logsOperation, "operation", "", "filter by operation type (cleanup, admin_cleanup, restore)")
	logsCmd.Flags().StringVar(&logsSince, "since", "", "show logs since date (YYYY-MM-DD)")
	logsCmd.Flags().IntVar(&logsLimit, "limit", 20, "maximum number of log entries to show")
}

func runLogs(cmd *cobra.Command, args []string) error {
	// Initialize paths
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	logger := services.NewCleanupLogger(issuemapPath)

	// Parse since date if provided
	var sinceTime *time.Time
	if logsSince != "" {
		parsed, err := time.Parse("2006-01-02", logsSince)
		if err != nil {
			printError(fmt.Errorf("invalid since date format (use YYYY-MM-DD): %w", err))
			return err
		}
		sinceTime = &parsed
	}

	// Get log entries
	entries, err := logger.GetLogEntries(sinceTime, logsOperation, logsLimit)
	if err != nil {
		printError(fmt.Errorf("failed to get log entries: %w", err))
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No log entries found")
		return nil
	}

	// Output results
	if logsJSON {
		jsonData, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			printError(fmt.Errorf("failed to marshal JSON: %w", err))
			return err
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayLogEntries(entries)
	return nil
}

func displayLogEntries(entries []*services.CleanupLogEntry) {
	if noColor {
		fmt.Printf("Cleanup Operation Logs (%d entries):\n", len(entries))
		fmt.Println("=====================================")
	} else {
		color.Cyan("Cleanup Operation Logs (%d entries):", len(entries))
		color.HiBlack("=====================================")
	}

	for i, entry := range entries {
		if i > 0 {
			fmt.Println()
		}

		// Entry header
		statusIcon := "✓"
		if !entry.Success {
			statusIcon = "✗"
		}

		if noColor {
			fmt.Printf("[%s] %s %s - %s\n",
				entry.Timestamp.Format("2006-01-02 15:04:05"),
				statusIcon,
				entry.Operation,
				entry.User)
		} else {
			timestamp := color.HiBlackString(entry.Timestamp.Format("2006-01-02 15:04:05"))
			operation := colorValue(entry.Operation)
			user := colorValue(entry.User)

			if entry.Success {
				statusColor := color.GreenString(statusIcon)
				fmt.Printf("[%s] %s %s - %s\n", timestamp, statusColor, operation, user)
			} else {
				statusColor := color.RedString(statusIcon)
				fmt.Printf("[%s] %s %s - %s\n", timestamp, statusColor, operation, user)
			}
		}

		// Operation details
		switch entry.Operation {
		case "cleanup":
			if entry.Result != nil {
				displayLogCleanupResult(entry.Result)
			}
		case "admin_cleanup":
			if entry.AdminResult != nil {
				displayLogAdminCleanupResult(entry.AdminResult)
			}
		case "restore":
			if entry.RestoreResult != nil {
				displayLogRestoreResult(entry.RestoreResult)
			}
		}

		// Error information
		if !entry.Success && entry.Error != "" {
			if noColor {
				fmt.Printf("Error: %s\n", entry.Error)
			} else {
				color.Red("Error: %s", entry.Error)
			}
		}
	}
}

func displayLogCleanupResult(result *entities.CleanupResult) {
	if noColor {
		fmt.Printf("  Duration: %v\n", result.Duration.Round(time.Millisecond))
		fmt.Printf("  Items Cleaned: %d\n", result.ItemsCleaned.Total)
		fmt.Printf("  Space Reclaimed: %s\n", entities.FormatBytes(result.SpaceReclaimed))
	} else {
		fmt.Printf("  %s %v\n", colorLabel("Duration:"), result.Duration.Round(time.Millisecond))
		fmt.Printf("  %s %d\n", colorLabel("Items Cleaned:"), result.ItemsCleaned.Total)
		fmt.Printf("  %s %s\n", colorLabel("Space Reclaimed:"), entities.FormatBytes(result.SpaceReclaimed))
	}
}

func displayLogAdminCleanupResult(result *entities.AdminCleanupResult) {
	if noColor {
		fmt.Printf("  Target: %s\n", result.Target)
		fmt.Printf("  Duration: %v\n", result.Duration.Round(time.Millisecond))
		fmt.Printf("  Items Cleaned: %d\n", result.ItemsCleaned.Total)
		fmt.Printf("  Space Reclaimed: %s\n", entities.FormatBytes(result.SpaceReclaimed))
		fmt.Printf("  Backup Created: %v\n", result.BackupCreated)
	} else {
		fmt.Printf("  %s %s\n", colorLabel("Target:"), colorValue(string(result.Target)))
		fmt.Printf("  %s %v\n", colorLabel("Duration:"), result.Duration.Round(time.Millisecond))
		fmt.Printf("  %s %d\n", colorLabel("Items Cleaned:"), result.ItemsCleaned.Total)
		fmt.Printf("  %s %s\n", colorLabel("Space Reclaimed:"), entities.FormatBytes(result.SpaceReclaimed))
		fmt.Printf("  %s %v\n", colorLabel("Backup Created:"), result.BackupCreated)
	}

	// Show filter criteria if present
	if len(result.FilterCriteria) > 0 {
		if noColor {
			fmt.Printf("  Filters: ")
		} else {
			fmt.Printf("  %s ", colorLabel("Filters:"))
		}

		var filters []string
		for key, value := range result.FilterCriteria {
			filters = append(filters, fmt.Sprintf("%s=%v", key, value))
		}

		for i, filter := range filters {
			if i > 0 {
				fmt.Print(", ")
			}
			if noColor {
				fmt.Print(filter)
			} else {
				fmt.Print(colorValue(filter))
			}
		}
		fmt.Println()
	}
}

func displayLogRestoreResult(result *entities.RestoreResult) {
	if noColor {
		fmt.Printf("  Backup ID: %s\n", result.BackupID)
		fmt.Printf("  Duration: %v\n", result.Duration.Round(time.Millisecond))
		fmt.Printf("  Items Restored: %d\n", result.ItemsRestored)
	} else {
		fmt.Printf("  %s %s\n", colorLabel("Backup ID:"), colorValue(result.BackupID))
		fmt.Printf("  %s %v\n", colorLabel("Duration:"), result.Duration.Round(time.Millisecond))
		fmt.Printf("  %s %d\n", colorLabel("Items Restored:"), result.ItemsRestored)
	}
}
