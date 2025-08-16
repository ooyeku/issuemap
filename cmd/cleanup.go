package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	cleanupDryRun      bool
	cleanupJSON        bool
	cleanupVerbose     bool
	cleanupConfigShow  bool
	cleanupConfigReset bool
)

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Data cleanup and retention management",
	Long: `Manage data cleanup and retention policies for the issuemap project.

This command provides tools to clean up old data, manage retention policies,
and prevent storage bloat over time.

Examples:
  issuemap cleanup                    # Run cleanup with current settings
  issuemap cleanup --dry-run          # Preview what would be cleaned
  issuemap cleanup --json             # JSON output
  issuemap cleanup config             # Show cleanup configuration
  issuemap cleanup config --reset     # Reset to default configuration`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCleanup(cmd, args)
	},
}

var cleanupConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure cleanup settings",
	Long: `Configure cleanup and retention settings.

Examples:
  issuemap cleanup config                    # Show current configuration
  issuemap cleanup config --reset            # Reset to defaults
  issuemap cleanup config --enable           # Enable automatic cleanup
  issuemap cleanup config --disable          # Disable automatic cleanup`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCleanupConfig(cmd, args)
	},
}

var (
	configEnable      bool
	configDisable     bool
	configSchedule    string
	configClosedDays  int
	configHistoryDays int
	configAttachDays  int
	configOrphanDays  int
)

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.AddCommand(cleanupConfigCmd)

	// Cleanup flags
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "preview cleanup without actually deleting files")
	cleanupCmd.Flags().BoolVar(&cleanupJSON, "json", false, "output in JSON format")
	cleanupCmd.Flags().BoolVar(&cleanupVerbose, "verbose", false, "verbose output with detailed information")

	// Cleanup config flags
	cleanupConfigCmd.Flags().BoolVar(&cleanupConfigShow, "show", false, "show current configuration")
	cleanupConfigCmd.Flags().BoolVar(&cleanupConfigReset, "reset", false, "reset configuration to defaults")
	cleanupConfigCmd.Flags().BoolVar(&configEnable, "enable", false, "enable automatic cleanup")
	cleanupConfigCmd.Flags().BoolVar(&configDisable, "disable", false, "disable automatic cleanup")
	cleanupConfigCmd.Flags().StringVar(&configSchedule, "schedule", "", "cleanup schedule (cron expression)")
	cleanupConfigCmd.Flags().IntVar(&configClosedDays, "closed-issues-days", 0, "retention days for closed issues")
	cleanupConfigCmd.Flags().IntVar(&configHistoryDays, "history-days", 0, "retention days for history")
	cleanupConfigCmd.Flags().IntVar(&configAttachDays, "attachment-days", 0, "retention days for closed issue attachments")
	cleanupConfigCmd.Flags().IntVar(&configOrphanDays, "orphan-days", 0, "retention days for orphaned attachments")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	configRepo := storage.NewFileConfigRepository(issuemapPath)
	issueRepo := storage.NewFileIssueRepository(issuemapPath)
	attachmentRepo := storage.NewFileAttachmentRepository(issuemapPath)

	cleanupService := services.NewCleanupService(issuemapPath, configRepo, issueRepo, attachmentRepo)

	// Run cleanup
	result, err := cleanupService.RunCleanup(ctx, cleanupDryRun)
	if err != nil {
		printError(fmt.Errorf("cleanup failed: %w", err))
		return err
	}

	// Output results
	if cleanupJSON {
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			printError(fmt.Errorf("failed to marshal JSON: %w", err))
			return err
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayCleanupResult(result, cleanupVerbose)
	return nil
}

func runCleanupConfig(cmd *cobra.Command, args []string) error {
	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	configRepo := storage.NewFileConfigRepository(issuemapPath)
	issueRepo := storage.NewFileIssueRepository(issuemapPath)
	attachmentRepo := storage.NewFileAttachmentRepository(issuemapPath)

	cleanupService := services.NewCleanupService(issuemapPath, configRepo, issueRepo, attachmentRepo)
	config := cleanupService.GetConfig()

	// If no flags provided, show current config
	if !cmd.Flags().Changed("enable") &&
		!cmd.Flags().Changed("disable") &&
		!cmd.Flags().Changed("reset") &&
		!cmd.Flags().Changed("schedule") &&
		!cmd.Flags().Changed("closed-issues-days") &&
		!cmd.Flags().Changed("history-days") &&
		!cmd.Flags().Changed("attachment-days") &&
		!cmd.Flags().Changed("orphan-days") {
		displayCleanupConfig(config)
		return nil
	}

	// Handle reset
	if cleanupConfigReset {
		config = entities.DefaultCleanupConfig()
		if err := cleanupService.UpdateConfig(config); err != nil {
			printError(fmt.Errorf("failed to reset cleanup config: %w", err))
			return err
		}

		if noColor {
			fmt.Println("Cleanup configuration reset to defaults")
		} else {
			color.Green("Cleanup configuration reset to defaults")
		}
		displayCleanupConfig(config)
		return nil
	}

	// Update configuration
	newConfig := *config // Copy current config

	if cmd.Flags().Changed("enable") {
		newConfig.Enabled = configEnable
	}

	if cmd.Flags().Changed("disable") {
		newConfig.Enabled = !configDisable
	}

	if cmd.Flags().Changed("schedule") {
		newConfig.Schedule = configSchedule
	}

	if cmd.Flags().Changed("closed-issues-days") {
		newConfig.RetentionDays.ClosedIssues = configClosedDays
	}

	if cmd.Flags().Changed("history-days") {
		newConfig.RetentionDays.History = configHistoryDays
	}

	if cmd.Flags().Changed("attachment-days") {
		newConfig.RetentionDays.ClosedIssueAttachments = configAttachDays
	}

	if cmd.Flags().Changed("orphan-days") {
		newConfig.RetentionDays.OrphanedAttachments = configOrphanDays
	}

	// Save updated configuration
	if err := cleanupService.UpdateConfig(&newConfig); err != nil {
		printError(fmt.Errorf("failed to update cleanup config: %w", err))
		return err
	}

	if noColor {
		fmt.Println("Cleanup configuration updated successfully")
	} else {
		color.Green("Cleanup configuration updated successfully")
	}

	// Show updated config
	displayCleanupConfig(&newConfig)

	return nil
}

func displayCleanupResult(result *entities.CleanupResult, verbose bool) {
	// Header
	if result.DryRun {
		if noColor {
			fmt.Println("Cleanup Result (DRY RUN) - " + result.Timestamp.Format("2006-01-02 15:04:05"))
		} else {
			color.Cyan("Cleanup Result (DRY RUN) - %s", result.Timestamp.Format("2006-01-02 15:04:05"))
		}
	} else {
		if noColor {
			fmt.Println("Cleanup Result - " + result.Timestamp.Format("2006-01-02 15:04:05"))
		} else {
			color.Cyan("Cleanup Result - %s", result.Timestamp.Format("2006-01-02 15:04:05"))
		}
	}

	if noColor {
		fmt.Println("═══════════════════════════════════════════════════")
	} else {
		color.HiBlack("═══════════════════════════════════════════════════")
	}

	// Summary
	if noColor {
		fmt.Printf("Duration: %v\n", result.Duration.Round(time.Millisecond))
		fmt.Printf("Items Cleaned: %d\n", result.ItemsCleaned.Total)
		fmt.Printf("Space Reclaimed: %s\n", entities.FormatBytes(result.SpaceReclaimed))
	} else {
		fmt.Printf("%s %v\n", colorLabel("Duration:"), colorValue(result.Duration.Round(time.Millisecond).String()))
		fmt.Printf("%s %d\n", colorLabel("Items Cleaned:"), result.ItemsCleaned.Total)
		fmt.Printf("%s %s\n", colorLabel("Space Reclaimed:"), colorValue(entities.FormatBytes(result.SpaceReclaimed)))
	}

	if result.ItemsArchived > 0 {
		if noColor {
			fmt.Printf("Items Archived: %d\n", result.ItemsArchived)
		} else {
			fmt.Printf("%s %d\n", colorLabel("Items Archived:"), result.ItemsArchived)
		}
	}

	fmt.Println()

	// Breakdown
	if result.ItemsCleaned.Total > 0 || verbose {
		if noColor {
			fmt.Println("Breakdown by Category:")
		} else {
			color.Yellow("Breakdown by Category:")
		}

		categories := []struct {
			name  string
			count int
		}{
			{"Closed Issues", result.ItemsCleaned.ClosedIssues},
			{"Attachments", result.ItemsCleaned.Attachments},
			{"Orphaned Attachments", result.ItemsCleaned.OrphanedAttachments},
			{"History Entries", result.ItemsCleaned.HistoryEntries},
			{"Time Entries", result.ItemsCleaned.TimeEntries},
			{"Empty Directories", result.ItemsCleaned.EmptyDirectories},
		}

		for _, cat := range categories {
			if cat.count > 0 || verbose {
				if noColor {
					fmt.Printf("  %-18s: %d\n", cat.name, cat.count)
				} else {
					fmt.Printf("  %s %d\n",
						colorLabel(fmt.Sprintf("%-18s:", cat.name)),
						cat.count)
				}
			}
		}
		fmt.Println()
	}

	// Errors
	if len(result.Errors) > 0 {
		if noColor {
			fmt.Println("Errors:")
		} else {
			color.Red("Errors:")
		}

		for _, err := range result.Errors {
			if noColor {
				fmt.Printf("  %s\n", err)
			} else {
				color.HiRed("  %s", err)
			}
		}
		fmt.Println()
	}

	// Status message
	if result.ItemsCleaned.Total == 0 && len(result.Errors) == 0 {
		if noColor {
			fmt.Println("No items needed cleanup based on current retention policies.")
		} else {
			color.Green("No items needed cleanup based on current retention policies.")
		}
	} else if len(result.Errors) == 0 {
		if result.DryRun {
			if noColor {
				fmt.Println("Dry run completed successfully. Use without --dry-run to perform actual cleanup.")
			} else {
				color.Green("Dry run completed successfully. Use without --dry-run to perform actual cleanup.")
			}
		} else {
			if noColor {
				fmt.Println("Cleanup completed successfully.")
			} else {
				color.Green("Cleanup completed successfully.")
			}
		}
	}
}

func displayCleanupConfig(config *entities.CleanupConfig) {
	if noColor {
		fmt.Println("Cleanup Configuration:")
		fmt.Println("─────────────────────")
	} else {
		color.Cyan("Cleanup Configuration:")
		color.HiBlack("─────────────────────")
	}

	// Basic settings
	if noColor {
		fmt.Printf("Enabled:           %v\n", config.Enabled)
		fmt.Printf("Schedule:          %s\n", config.Schedule)
		fmt.Printf("Dry Run Mode:      %v\n", config.DryRunMode)
		fmt.Printf("Archive Before Delete: %v\n", config.ArchiveBeforeDelete)
	} else {
		fmt.Printf("%s %s\n", colorLabel("Enabled:"), colorValue(fmt.Sprintf("%v", config.Enabled)))
		fmt.Printf("%s %s\n", colorLabel("Schedule:"), colorValue(config.Schedule))
		fmt.Printf("%s %s\n", colorLabel("Dry Run Mode:"), colorValue(fmt.Sprintf("%v", config.DryRunMode)))
		fmt.Printf("%s %s\n", colorLabel("Archive Before Delete:"), colorValue(fmt.Sprintf("%v", config.ArchiveBeforeDelete)))
	}

	if config.ArchiveBeforeDelete && config.ArchivePath != "" {
		if noColor {
			fmt.Printf("Archive Path:      %s\n", config.ArchivePath)
		} else {
			fmt.Printf("%s %s\n", colorLabel("Archive Path:"), colorValue(config.ArchivePath))
		}
	}

	fmt.Println()

	// Retention policies
	if noColor {
		fmt.Println("Retention Policies (days):")
		fmt.Printf("  Closed Issues:           %s\n", formatRetentionDays(config.RetentionDays.ClosedIssues))
		fmt.Printf("  Closed Issue Attachments: %s\n", formatRetentionDays(config.RetentionDays.ClosedIssueAttachments))
		fmt.Printf("  History:                 %s\n", formatRetentionDays(config.RetentionDays.History))
		fmt.Printf("  Time Entries:            %s\n", formatRetentionDays(config.RetentionDays.TimeEntries))
		fmt.Printf("  Orphaned Attachments:    %s\n", formatRetentionDays(config.RetentionDays.OrphanedAttachments))
		fmt.Printf("  Empty Directories:       %v\n", config.RetentionDays.EmptyDirectories)
	} else {
		color.Yellow("Retention Policies (days):")
		fmt.Printf("  %s %s\n", colorLabel("Closed Issues:"), colorValue(formatRetentionDays(config.RetentionDays.ClosedIssues)))
		fmt.Printf("  %s %s\n", colorLabel("Closed Issue Attachments:"), colorValue(formatRetentionDays(config.RetentionDays.ClosedIssueAttachments)))
		fmt.Printf("  %s %s\n", colorLabel("History:"), colorValue(formatRetentionDays(config.RetentionDays.History)))
		fmt.Printf("  %s %s\n", colorLabel("Time Entries:"), colorValue(formatRetentionDays(config.RetentionDays.TimeEntries)))
		fmt.Printf("  %s %s\n", colorLabel("Orphaned Attachments:"), colorValue(formatRetentionDays(config.RetentionDays.OrphanedAttachments)))
		fmt.Printf("  %s %s\n", colorLabel("Empty Directories:"), colorValue(fmt.Sprintf("%v", config.RetentionDays.EmptyDirectories)))
	}

	fmt.Println()

	// Minimum keep policies
	if noColor {
		fmt.Println("Minimum Keep Policies:")
		fmt.Printf("  Closed Issues:       %d\n", config.MinimumKeep.ClosedIssues)
		fmt.Printf("  History per Issue:   %d\n", config.MinimumKeep.HistoryPerIssue)
		fmt.Printf("  Time Entries per Issue: %d\n", config.MinimumKeep.TimeEntriesPerIssue)
	} else {
		color.Yellow("Minimum Keep Policies:")
		fmt.Printf("  %s %d\n", colorLabel("Closed Issues:"), config.MinimumKeep.ClosedIssues)
		fmt.Printf("  %s %d\n", colorLabel("History per Issue:"), config.MinimumKeep.HistoryPerIssue)
		fmt.Printf("  %s %d\n", colorLabel("Time Entries per Issue:"), config.MinimumKeep.TimeEntriesPerIssue)
	}
}

func formatRetentionDays(days int) string {
	if days <= 0 {
		return "never"
	}
	return fmt.Sprintf("%d", days)
}
