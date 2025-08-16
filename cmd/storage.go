package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	storageJSON    bool
	storageVerbose bool
	storageLargest int
	storageByIssue bool
	storageRefresh bool
)

// storageCmd represents the storage command
var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Storage management and monitoring",
	Long: `Monitor and manage storage usage for the issuemap project.
	
This command provides insights into disk usage, storage quotas, and helps
identify potential storage issues before they become problems.

Examples:
  issuemap storage                    # Show storage status
  issuemap storage --json             # JSON output
  issuemap storage --verbose          # Detailed breakdown
  issuemap storage --largest 10       # Show 10 largest files
  issuemap storage --by-issue         # Group by issue
  issuemap storage --refresh          # Force refresh cache`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStorage(cmd, args)
	},
}

var storageConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure storage settings",
	Long: `Configure storage quotas and limits.

Examples:
  issuemap storage config                           # Show current config
  issuemap storage config --max-size 2GB           # Set max project size
  issuemap storage config --enforce-quotas         # Enable quota enforcement
  issuemap storage config --warning-threshold 75   # Set warning at 75%`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStorageConfig(cmd, args)
	},
}

var (
	configMaxSize            string
	configMaxAttachment      string
	configMaxAttachments     string
	configEnforceQuotas      bool
	configDisableQuotas      bool
	configWarningThreshold   int
	configCriticalThreshold  int
	configAutoCleanup        bool
	configDisableAutoCleanup bool
)

func init() {
	rootCmd.AddCommand(storageCmd)
	storageCmd.AddCommand(storageConfigCmd)

	// Storage status flags
	storageCmd.Flags().BoolVar(&storageJSON, "json", false, "output in JSON format")
	storageCmd.Flags().BoolVar(&storageVerbose, "verbose", false, "verbose output with detailed breakdown")
	storageCmd.Flags().IntVar(&storageLargest, "largest", 5, "show N largest files")
	storageCmd.Flags().BoolVar(&storageByIssue, "by-issue", false, "group storage by issue")
	storageCmd.Flags().BoolVar(&storageRefresh, "refresh", false, "force refresh cache")

	// Storage config flags
	storageConfigCmd.Flags().StringVar(&configMaxSize, "max-size", "", "maximum project size (e.g., 1GB, 500MB)")
	storageConfigCmd.Flags().StringVar(&configMaxAttachment, "max-attachment", "", "maximum single attachment size")
	storageConfigCmd.Flags().StringVar(&configMaxAttachments, "max-attachments", "", "maximum total attachments size")
	storageConfigCmd.Flags().BoolVar(&configEnforceQuotas, "enforce-quotas", false, "enable quota enforcement")
	storageConfigCmd.Flags().BoolVar(&configDisableQuotas, "disable-quotas", false, "disable quota enforcement")
	storageConfigCmd.Flags().IntVar(&configWarningThreshold, "warning-threshold", 0, "warning threshold percentage (0-100)")
	storageConfigCmd.Flags().IntVar(&configCriticalThreshold, "critical-threshold", 0, "critical threshold percentage (0-100)")
	storageConfigCmd.Flags().BoolVar(&configAutoCleanup, "auto-cleanup", false, "enable automatic cleanup")
	storageConfigCmd.Flags().BoolVar(&configDisableAutoCleanup, "disable-auto-cleanup", false, "disable automatic cleanup")
}

func runStorage(cmd *cobra.Command, args []string) error {
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

	storageService := services.NewStorageService(issuemapPath, configRepo, issueRepo, attachmentRepo)

	// Get storage status
	status, err := storageService.GetStorageStatus(ctx, storageRefresh)
	if err != nil {
		printError(fmt.Errorf("failed to get storage status: %w", err))
		return err
	}

	if storageJSON {
		breakdown, err := storageService.GetStorageBreakdown(ctx)
		if err != nil {
			printError(fmt.Errorf("failed to get storage breakdown: %w", err))
			return err
		}

		jsonData, err := json.MarshalIndent(breakdown, "", "  ")
		if err != nil {
			printError(fmt.Errorf("failed to marshal JSON: %w", err))
			return err
		}

		fmt.Println(string(jsonData))
		return nil
	}

	displayStorageStatus(status, storageService.GetConfig())

	if storageVerbose {
		displayDetailedBreakdown(status)
	}

	if storageByIssue && len(status.StorageByIssue) > 0 {
		displayStorageByIssue(status.StorageByIssue)
	}

	if storageLargest > 0 && len(status.LargestFiles) > 0 {
		displayLargestFiles(status.LargestFiles, storageLargest)
	}

	return nil
}

func runStorageConfig(cmd *cobra.Command, args []string) error {
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

	storageService := services.NewStorageService(issuemapPath, configRepo, issueRepo, attachmentRepo)
	config := storageService.GetConfig()

	// If no flags provided, show current config
	if !cmd.Flags().Changed("max-size") &&
		!cmd.Flags().Changed("max-attachment") &&
		!cmd.Flags().Changed("max-attachments") &&
		!cmd.Flags().Changed("enforce-quotas") &&
		!cmd.Flags().Changed("disable-quotas") &&
		!cmd.Flags().Changed("warning-threshold") &&
		!cmd.Flags().Changed("critical-threshold") &&
		!cmd.Flags().Changed("auto-cleanup") &&
		!cmd.Flags().Changed("disable-auto-cleanup") {
		displayStorageConfig(config)
		return nil
	}

	// Update configuration
	newConfig := *config // Copy current config

	if configMaxSize != "" {
		size, err := parseSize(configMaxSize)
		if err != nil {
			printError(fmt.Errorf("invalid max-size: %w", err))
			return err
		}
		newConfig.MaxProjectSize = size
	}

	if configMaxAttachment != "" {
		size, err := parseSize(configMaxAttachment)
		if err != nil {
			printError(fmt.Errorf("invalid max-attachment: %w", err))
			return err
		}
		newConfig.MaxAttachmentSize = size
	}

	if configMaxAttachments != "" {
		size, err := parseSize(configMaxAttachments)
		if err != nil {
			printError(fmt.Errorf("invalid max-attachments: %w", err))
			return err
		}
		newConfig.MaxTotalAttachments = size
	}

	if cmd.Flags().Changed("enforce-quotas") {
		newConfig.EnforceQuotas = configEnforceQuotas
	}

	if cmd.Flags().Changed("disable-quotas") {
		newConfig.EnforceQuotas = !configDisableQuotas
	}

	if cmd.Flags().Changed("warning-threshold") {
		if configWarningThreshold < 0 || configWarningThreshold > 100 {
			printError(fmt.Errorf("warning threshold must be between 0 and 100"))
			return fmt.Errorf("invalid threshold")
		}
		newConfig.WarningThreshold = configWarningThreshold
	}

	if cmd.Flags().Changed("critical-threshold") {
		if configCriticalThreshold < 0 || configCriticalThreshold > 100 {
			printError(fmt.Errorf("critical threshold must be between 0 and 100"))
			return fmt.Errorf("invalid threshold")
		}
		newConfig.CriticalThreshold = configCriticalThreshold
	}

	if cmd.Flags().Changed("auto-cleanup") {
		newConfig.EnableAutoCleanup = configAutoCleanup
	}

	if cmd.Flags().Changed("disable-auto-cleanup") {
		newConfig.EnableAutoCleanup = !configDisableAutoCleanup
	}

	// Save updated configuration
	if err := storageService.UpdateConfig(&newConfig); err != nil {
		printError(fmt.Errorf("failed to update storage config: %w", err))
		return err
	}

	if noColor {
		fmt.Println("Storage configuration updated successfully")
	} else {
		color.Green("Storage configuration updated successfully")
	}

	// Show updated config
	displayStorageConfig(&newConfig)

	return nil
}

func displayStorageStatus(status *entities.StorageStatus, config *entities.StorageConfig) {
	// Header
	if noColor {
		fmt.Printf("Storage Status - %s\n", status.LastCalculated.Format("2006-01-02 15:04:05"))
		fmt.Println(strings.Repeat("=", 50))
	} else {
		color.Cyan("Storage Status - %s", status.LastCalculated.Format("2006-01-02 15:04:05"))
		color.HiBlack(strings.Repeat("=", 50))
	}

	// Total usage
	if noColor {
		fmt.Printf("Total Size: %s\n", entities.FormatBytes(status.TotalSize))
	} else {
		fmt.Printf("%s %s\n", colorLabel("Total Size:"), colorValue(entities.FormatBytes(status.TotalSize)))
	}

	// Usage percentage if quota is set
	if config.MaxProjectSize > 0 {
		percentage := status.UsagePercentage
		usageColor := getUsageColor(status.Status)

		if noColor {
			fmt.Printf("Usage: %.1f%% of %s\n", percentage, entities.FormatBytes(config.MaxProjectSize))
		} else {
			fmt.Printf("%s %s of %s\n",
				colorLabel("Usage:"),
				usageColor("%.1f%%", percentage),
				colorValue(entities.FormatBytes(config.MaxProjectSize)))
		}
	}

	// Health status
	statusText := strings.Title(string(status.Status))
	statusColor := getStatusColor(status.Status)

	if noColor {
		fmt.Printf("Health: %s\n", statusText)
	} else {
		fmt.Printf("%s %s\n", colorLabel("Health:"), statusColor(statusText))
	}

	// Available disk space
	if noColor {
		fmt.Printf("Available Disk: %s\n", entities.FormatBytes(status.AvailableDiskSpace))
	} else {
		fmt.Printf("%s %s\n", colorLabel("Available Disk:"), colorValue(entities.FormatBytes(status.AvailableDiskSpace)))
	}

	fmt.Println()

	// Category breakdown
	if noColor {
		fmt.Println("Breakdown by Category:")
		fmt.Printf("  Issues:       %s (%d files)\n", entities.FormatBytes(status.IssuesSize), status.IssueCount)
		fmt.Printf("  Attachments:  %s (%d files)\n", entities.FormatBytes(status.AttachmentsSize), status.AttachmentCount)
		fmt.Printf("  History:      %s\n", entities.FormatBytes(status.HistorySize))
		fmt.Printf("  Time Entries: %s\n", entities.FormatBytes(status.TimeEntriesSize))
		fmt.Printf("  Metadata:     %s\n", entities.FormatBytes(status.MetadataSize))
	} else {
		color.Yellow("Breakdown by Category:")
		fmt.Printf("  %s %s (%d files)\n", colorLabel("Issues:"), colorValue(entities.FormatBytes(status.IssuesSize)), status.IssueCount)
		fmt.Printf("  %s %s (%d files)\n", colorLabel("Attachments:"), colorValue(entities.FormatBytes(status.AttachmentsSize)), status.AttachmentCount)
		fmt.Printf("  %s %s\n", colorLabel("History:"), colorValue(entities.FormatBytes(status.HistorySize)))
		fmt.Printf("  %s %s\n", colorLabel("Time Entries:"), colorValue(entities.FormatBytes(status.TimeEntriesSize)))
		fmt.Printf("  %s %s\n", colorLabel("Metadata:"), colorValue(entities.FormatBytes(status.MetadataSize)))
	}

	// Warnings
	if len(status.Warnings) > 0 {
		fmt.Println()
		if noColor {
			fmt.Println("Warnings:")
		} else {
			color.Yellow("Warnings:")
		}

		for _, warning := range status.Warnings {
			if strings.HasPrefix(warning, "CRITICAL:") {
				if noColor {
					fmt.Printf("  %s\n", warning)
				} else {
					color.Red("  %s", warning)
				}
			} else if strings.HasPrefix(warning, "WARNING:") {
				if noColor {
					fmt.Printf("  %s\n", warning)
				} else {
					color.Yellow("  %s", warning)
				}
			} else {
				if noColor {
					fmt.Printf("  %s\n", warning)
				} else {
					color.HiBlack("  %s", warning)
				}
			}
		}
	}
}

func displayDetailedBreakdown(status *entities.StorageStatus) {
	fmt.Println()
	if noColor {
		fmt.Println("Detailed Breakdown:")
	} else {
		color.Yellow("Detailed Breakdown:")
	}

	total := status.TotalSize
	categories := []struct {
		name string
		size int64
	}{
		{"Issues", status.IssuesSize},
		{"Attachments", status.AttachmentsSize},
		{"History", status.HistorySize},
		{"Time Entries", status.TimeEntriesSize},
		{"Metadata", status.MetadataSize},
	}

	for _, cat := range categories {
		if cat.size > 0 {
			percentage := float64(cat.size) / float64(total) * 100
			if noColor {
				fmt.Printf("  %-12s: %s (%.1f%%)\n", cat.name, entities.FormatBytes(cat.size), percentage)
			} else {
				fmt.Printf("  %s %s %s\n",
					colorLabel(fmt.Sprintf("%-12s:", cat.name)),
					colorValue(entities.FormatBytes(cat.size)),
					color.HiBlackString("(%.1f%%)", percentage))
			}
		}
	}
}

func displayStorageByIssue(storageByIssue map[string]int64) {
	fmt.Println()
	if noColor {
		fmt.Println("Storage by Issue:")
	} else {
		color.Yellow("Storage by Issue:")
	}

	// Convert to slice for sorting
	type issueStorage struct {
		issue string
		size  int64
	}

	var issues []issueStorage
	for issue, size := range storageByIssue {
		issues = append(issues, issueStorage{issue, size})
	}

	// Sort by size descending
	for i := 0; i < len(issues); i++ {
		for j := i + 1; j < len(issues); j++ {
			if issues[i].size < issues[j].size {
				issues[i], issues[j] = issues[j], issues[i]
			}
		}
	}

	// Display top 10
	limit := len(issues)
	if limit > 10 {
		limit = 10
	}

	for i := 0; i < limit; i++ {
		issue := issues[i]
		if noColor {
			fmt.Printf("  %s: %s\n", issue.issue, entities.FormatBytes(issue.size))
		} else {
			fmt.Printf("  %s %s\n",
				colorValue(issue.issue),
				colorValue(entities.FormatBytes(issue.size)))
		}
	}

	if len(issues) > 10 {
		if noColor {
			fmt.Printf("  ... and %d more\n", len(issues)-10)
		} else {
			color.HiBlack("  ... and %d more", len(issues)-10)
		}
	}
}

func displayLargestFiles(files []entities.FileInfo, limit int) {
	fmt.Println()
	if noColor {
		fmt.Printf("Largest Files (top %d):\n", limit)
	} else {
		color.Yellow("Largest Files (top %d):", limit)
	}

	displayLimit := len(files)
	if displayLimit > limit {
		displayLimit = limit
	}

	for i := 0; i < displayLimit; i++ {
		file := files[i]
		if noColor {
			fmt.Printf("  %s: %s (%s)\n", file.Path, entities.FormatBytes(file.Size), file.Type)
		} else {
			fmt.Printf("  %s %s %s\n",
				colorValue(file.Path),
				colorValue(entities.FormatBytes(file.Size)),
				color.HiBlackString("(%s)", file.Type))
		}
	}
}

func displayStorageConfig(config *entities.StorageConfig) {
	fmt.Println()
	if noColor {
		fmt.Println("Storage Configuration:")
		fmt.Println(strings.Repeat("-", 30))
	} else {
		color.Cyan("Storage Configuration:")
		color.HiBlack(strings.Repeat("-", 30))
	}

	// Quota settings
	if noColor {
		fmt.Printf("Max Project Size:     %s\n", formatConfigSize(config.MaxProjectSize))
		fmt.Printf("Max Attachment Size:  %s\n", formatConfigSize(config.MaxAttachmentSize))
		fmt.Printf("Max Total Attachments: %s\n", formatConfigSize(config.MaxTotalAttachments))
		fmt.Printf("Enforce Quotas:       %v\n", config.EnforceQuotas)
	} else {
		fmt.Printf("%s %s\n", colorLabel("Max Project Size:"), colorValue(formatConfigSize(config.MaxProjectSize)))
		fmt.Printf("%s %s\n", colorLabel("Max Attachment Size:"), colorValue(formatConfigSize(config.MaxAttachmentSize)))
		fmt.Printf("%s %s\n", colorLabel("Max Total Attachments:"), colorValue(formatConfigSize(config.MaxTotalAttachments)))
		fmt.Printf("%s %s\n", colorLabel("Enforce Quotas:"), colorValue(fmt.Sprintf("%v", config.EnforceQuotas)))
	}

	// Threshold settings
	if noColor {
		fmt.Printf("Warning Threshold:    %d%%\n", config.WarningThreshold)
		fmt.Printf("Critical Threshold:   %d%%\n", config.CriticalThreshold)
		fmt.Printf("Auto Cleanup:         %v\n", config.EnableAutoCleanup)
	} else {
		fmt.Printf("%s %s\n", colorLabel("Warning Threshold:"), colorValue(fmt.Sprintf("%d%%", config.WarningThreshold)))
		fmt.Printf("%s %s\n", colorLabel("Critical Threshold:"), colorValue(fmt.Sprintf("%d%%", config.CriticalThreshold)))
		fmt.Printf("%s %s\n", colorLabel("Auto Cleanup:"), colorValue(fmt.Sprintf("%v", config.EnableAutoCleanup)))
	}
}

func formatConfigSize(size int64) string {
	if size <= 0 {
		return "unlimited"
	}
	return entities.FormatBytes(size)
}

func parseSize(sizeStr string) (int64, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	// Extract number and unit
	var numStr, unit string
	for i, r := range sizeStr {
		if r >= '0' && r <= '9' || r == '.' {
			continue
		}
		numStr = sizeStr[:i]
		unit = sizeStr[i:]
		break
	}

	if numStr == "" {
		numStr = sizeStr
	}

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", numStr)
	}

	var multiplier int64 = 1
	switch unit {
	case "", "B", "BYTES":
		multiplier = 1
	case "K", "KB", "KILOBYTES":
		multiplier = 1024
	case "M", "MB", "MEGABYTES":
		multiplier = 1024 * 1024
	case "G", "GB", "GIGABYTES":
		multiplier = 1024 * 1024 * 1024
	case "T", "TB", "TERABYTES":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}

	return int64(num * float64(multiplier)), nil
}

func getUsageColor(status entities.StorageHealthStatus) func(format string, a ...interface{}) string {
	if noColor {
		return func(format string, a ...interface{}) string {
			return fmt.Sprintf(format, a...)
		}
	}

	switch status {
	case entities.StorageHealthCritical:
		return color.RedString
	case entities.StorageHealthWarning:
		return color.YellowString
	default:
		return color.GreenString
	}
}

func getStatusColor(status entities.StorageHealthStatus) func(format string, a ...interface{}) string {
	if noColor {
		return func(format string, a ...interface{}) string {
			return fmt.Sprintf(format, a...)
		}
	}

	switch status {
	case entities.StorageHealthCritical:
		return color.RedString
	case entities.StorageHealthWarning:
		return color.YellowString
	default:
		return color.GreenString
	}
}
