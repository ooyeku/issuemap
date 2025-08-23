package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

var (
	restoreDryRun       bool
	restoreJSON         bool
	restoreBackupID     string
	restoreForceConfirm bool
	restoreList         bool
)

// restoreCmd represents the restore command
var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore data from cleanup backups",
	Long: `Restore data that was previously backed up during cleanup operations.

This command allows you to recover data from automatic backups created
during cleanup operations, providing a safety net for accidental deletions.

Examples:
  issuemap restore --list                    # List available backups
  issuemap restore --backup-id abc123        # Restore specific backup
  issuemap restore --backup-id abc123 --dry-run  # Preview restore
  issuemap restore --json                    # JSON output`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Deprecation warning
		printWarning("DEPRECATED: 'restore' command will be removed in a future version. Use 'storage restore' instead:")
		printWarning("  issuemap storage restore")
		fmt.Println()

		return runRestore(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	// Restore flags
	restoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "preview restore without actually restoring files")
	restoreCmd.Flags().BoolVar(&restoreJSON, "json", false, "output in JSON format")
	restoreCmd.Flags().StringVar(&restoreBackupID, "backup-id", "", "ID of backup to restore")
	restoreCmd.Flags().BoolVar(&restoreForceConfirm, "yes", false, "skip confirmation prompts")
	restoreCmd.Flags().BoolVar(&restoreList, "list", false, "list available backups")
}

func runRestore(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize paths
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	backupsPath := filepath.Join(issuemapPath, "backups")

	if restoreList {
		return listBackups(backupsPath)
	}

	if restoreBackupID == "" {
		printError(fmt.Errorf("backup ID is required. Use --list to see available backups"))
		return fmt.Errorf("backup ID required")
	}

	// Find backup by ID
	backup, err := findBackup(backupsPath, restoreBackupID)
	if err != nil {
		printError(fmt.Errorf("backup not found: %w", err))
		return err
	}

	// Get confirmation if needed
	if !restoreForceConfirm && !restoreDryRun {
		confirmed, err := getRestoreConfirmation(backup)
		if err != nil {
			printError(fmt.Errorf("confirmation failed: %w", err))
			return err
		}
		if !confirmed {
			fmt.Println("Restore operation cancelled")
			return nil
		}
	}

	// Perform restore
	result, err := performRestore(ctx, backup, restoreDryRun)
	if err != nil {
		printError(fmt.Errorf("restore failed: %w", err))
		return err
	}

	// Output results
	if restoreJSON {
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			printError(fmt.Errorf("failed to marshal JSON: %w", err))
			return err
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayRestoreResult(result)
	return nil
}

func listBackups(backupsPath string) error {
	backups, err := scanBackups(backupsPath)
	if err != nil {
		printError(fmt.Errorf("failed to scan backups: %w", err))
		return err
	}

	if len(backups) == 0 {
		fmt.Println("No backups found")
		return nil
	}

	if restoreJSON {
		jsonData, err := json.MarshalIndent(backups, "", "  ")
		if err != nil {
			printError(fmt.Errorf("failed to marshal JSON: %w", err))
			return err
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayBackupsList(backups)
	return nil
}

func scanBackups(backupsPath string) ([]*entities.BackupInfo, error) {
	if _, err := os.Stat(backupsPath); os.IsNotExist(err) {
		return []*entities.BackupInfo{}, nil
	}

	entries, err := os.ReadDir(backupsPath)
	if err != nil {
		return nil, err
	}

	var backups []*entities.BackupInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		backupPath := filepath.Join(backupsPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Parse backup directory name for info
		// Format: cleanup_<target>_<timestamp>
		parts := strings.Split(entry.Name(), "_")
		if len(parts) < 3 {
			continue
		}

		target := strings.Join(parts[1:len(parts)-1], "_")
		timestamp := parts[len(parts)-1]

		// Parse timestamp
		backupTime, err := time.Parse("20060102_150405", timestamp)
		if err != nil {
			backupTime = info.ModTime()
		}

		// Calculate backup size
		size, itemsCount := calculateBackupSize(backupPath)

		backup := &entities.BackupInfo{
			ID:          entry.Name(),
			Timestamp:   backupTime,
			Description: fmt.Sprintf("Cleanup backup: %s", target),
			Location:    backupPath,
			Size:        size,
			ItemsCount:  itemsCount,
			CanRestore:  true,
		}

		backups = append(backups, backup)
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

func calculateBackupSize(path string) (int64, int) {
	var totalSize int64
	var itemsCount int

	filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		totalSize += info.Size()
		itemsCount++
		return nil
	})

	return totalSize, itemsCount
}

func findBackup(backupsPath, backupID string) (*entities.BackupInfo, error) {
	backups, err := scanBackups(backupsPath)
	if err != nil {
		return nil, err
	}

	for _, backup := range backups {
		if backup.ID == backupID || strings.HasPrefix(backup.ID, backupID) {
			return backup, nil
		}
	}

	return nil, fmt.Errorf("backup with ID '%s' not found", backupID)
}

func getRestoreConfirmation(backup *entities.BackupInfo) (bool, error) {
	fmt.Printf("⚠️  You are about to restore from backup:\n")
	fmt.Printf("ID: %s\n", backup.ID)
	fmt.Printf("Created: %s\n", backup.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Description: %s\n", backup.Description)
	fmt.Printf("Size: %s (%d items)\n", entities.FormatBytes(backup.Size), backup.ItemsCount)
	fmt.Print("\nThis will overwrite existing data. Continue? (y/N): ")

	var response string
	fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

func performRestore(ctx context.Context, backup *entities.BackupInfo, dryRun bool) (*entities.RestoreResult, error) {
	start := time.Now()
	result := &entities.RestoreResult{
		Timestamp:     start,
		DryRun:        dryRun,
		BackupID:      backup.ID,
		ItemsRestored: 0,
		Errors:        []string{},
	}

	// TODO: Implement actual restore logic
	// For now, just simulate the restore
	if !dryRun {
		result.Errors = append(result.Errors, "Restore functionality not yet implemented")
	}

	result.Duration = time.Since(start)
	return result, nil
}

func displayBackupsList(backups []*entities.BackupInfo) {
	if noColor {
		fmt.Println("Available Backups:")
		fmt.Println("==================")
	} else {
		color.Cyan("Available Backups:")
		color.HiBlack("==================")
	}

	if len(backups) == 0 {
		fmt.Println("No backups found")
		return
	}

	for _, backup := range backups {
		if noColor {
			fmt.Printf("ID: %s\n", backup.ID)
			fmt.Printf("Created: %s\n", backup.Timestamp.Format("2006-01-02 15:04:05"))
			fmt.Printf("Description: %s\n", backup.Description)
			fmt.Printf("Size: %s (%d items)\n", entities.FormatBytes(backup.Size), backup.ItemsCount)
			fmt.Printf("Location: %s\n", backup.Location)
		} else {
			fmt.Printf("%s %s\n", colorLabel("ID:"), colorValue(backup.ID))
			fmt.Printf("%s %s\n", colorLabel("Created:"), colorValue(backup.Timestamp.Format("2006-01-02 15:04:05")))
			fmt.Printf("%s %s\n", colorLabel("Description:"), colorValue(backup.Description))
			fmt.Printf("%s %s (%d items)\n", colorLabel("Size:"), colorValue(entities.FormatBytes(backup.Size)), backup.ItemsCount)
			fmt.Printf("%s %s\n", colorLabel("Location:"), colorValue(backup.Location))
		}
		fmt.Println()
	}
}

func displayRestoreResult(result *entities.RestoreResult) {
	// Header
	if result.DryRun {
		if noColor {
			fmt.Printf("Restore Result (DRY RUN) - %s\n", result.Timestamp.Format("2006-01-02 15:04:05"))
		} else {
			color.Cyan("Restore Result (DRY RUN) - %s", result.Timestamp.Format("2006-01-02 15:04:05"))
		}
	} else {
		if noColor {
			fmt.Printf("Restore Result - %s\n", result.Timestamp.Format("2006-01-02 15:04:05"))
		} else {
			color.Cyan("Restore Result - %s", result.Timestamp.Format("2006-01-02 15:04:05"))
		}
	}

	if noColor {
		fmt.Println("════════════════════════════════════════")
	} else {
		color.HiBlack("════════════════════════════════════════")
	}

	// Summary
	if noColor {
		fmt.Printf("Backup ID: %s\n", result.BackupID)
		fmt.Printf("Duration: %v\n", result.Duration.Round(time.Millisecond))
		fmt.Printf("Items Restored: %d\n", result.ItemsRestored)
	} else {
		fmt.Printf("%s %s\n", colorLabel("Backup ID:"), colorValue(result.BackupID))
		fmt.Printf("%s %v\n", colorLabel("Duration:"), colorValue(result.Duration.Round(time.Millisecond).String()))
		fmt.Printf("%s %d\n", colorLabel("Items Restored:"), result.ItemsRestored)
	}

	// Errors
	if len(result.Errors) > 0 {
		fmt.Println()
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
	}

	// Status message
	if len(result.Errors) == 0 {
		if result.DryRun {
			if noColor {
				fmt.Println("\nRestore dry run completed successfully.")
			} else {
				color.Green("\nRestore dry run completed successfully.")
			}
		} else {
			if noColor {
				fmt.Println("\nRestore completed successfully.")
			} else {
				color.Green("\nRestore completed successfully.")
			}
		}
	}
}
