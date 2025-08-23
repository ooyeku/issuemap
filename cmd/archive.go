package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	archiveJSON        bool
	archiveDryRun      bool
	archiveClosed      bool
	archiveOlderThan   string
	archiveMinAge      int
	archiveIssueIDs    []string
	archiveList        bool
	archiveStats       bool
	archiveSearch      string
	archiveRestore     []string
	archiveVerify      string
	archiveEnable      bool
	archiveDisable     bool
	archiveCompress    int
	archiveFormat      string
	archiveIncludeAtt  bool
	archiveIncludeHist bool
)

// archivesCmd represents the archives command
var archivesCmd = &cobra.Command{
	Use:   "archives",
	Short: "Archive and restore old closed issues",
	Long: `Archive old closed issues to reduce active storage usage and restore them when needed.

This command provides comprehensive archive management including:
- Archive old closed issues with compression
- Search archived issues
- Restore issues from archives
- List and manage archives
- Configure automatic archiving

Examples:
  issuemap archive --closed --older-than 6m     # Archive closed issues older than 6 months
  issuemap archive --issue ISSUE-001 ISSUE-002 # Archive specific issues
  issuemap archive --restore ISSUE-001          # Restore issue from archive
  issuemap archive --list                       # List all archives
  issuemap archive --search "bug fix"           # Search archived issues
  issuemap archive --stats                      # Show archive statistics
  issuemap archive --verify archive_2024.tar.gz # Verify archive integrity`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runArchive(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(archivesCmd)

	// Action flags
	archivesCmd.Flags().BoolVar(&archiveClosed, "closed", false, "archive closed issues only")
	archivesCmd.Flags().StringVar(&archiveOlderThan, "older-than", "", "archive issues older than duration (e.g., 6m, 1y)")
	archivesCmd.Flags().IntVar(&archiveMinAge, "min-age", 0, "minimum age in days for archiving")
	archivesCmd.Flags().StringSliceVar(&archiveIssueIDs, "issue", []string{}, "specific issue IDs to archive")
	archivesCmd.Flags().BoolVar(&archiveList, "list", false, "list all archives")
	archivesCmd.Flags().BoolVar(&archiveStats, "stats", false, "show archive statistics")
	archivesCmd.Flags().StringVar(&archiveSearch, "search", "", "search archived issues")
	archivesCmd.Flags().StringSliceVar(&archiveRestore, "restore", []string{}, "restore issues from archive")
	archivesCmd.Flags().StringVar(&archiveVerify, "verify", "", "verify archive integrity")

	// Configuration flags
	archivesCmd.Flags().BoolVar(&archiveEnable, "enable", false, "enable automatic archiving")
	archivesCmd.Flags().BoolVar(&archiveDisable, "disable", false, "disable automatic archiving")
	archivesCmd.Flags().IntVar(&archiveCompress, "compression", 0, "compression level (0-9)")
	archivesCmd.Flags().StringVar(&archiveFormat, "format", "", "archive format (tar.gz, zip)")
	archivesCmd.Flags().BoolVar(&archiveIncludeAtt, "include-attachments", false, "include attachments in archives")
	archivesCmd.Flags().BoolVar(&archiveIncludeHist, "include-history", false, "include history in archives")

	// Common flags
	archivesCmd.Flags().BoolVar(&archiveDryRun, "dry-run", false, "preview changes without making them")
	archivesCmd.Flags().BoolVar(&archiveJSON, "json", false, "output in JSON format")
}

func runArchive(cmd *cobra.Command, args []string) error {
	// Initialize paths
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")

	// Initialize repositories and services
	configRepo := storage.NewFileConfigRepository(issuemapPath)
	issueRepo := storage.NewFileIssueRepository(issuemapPath)
	attachmentRepo := storage.NewFileAttachmentRepository(issuemapPath)

	// Create archive service
	archiveService := services.NewArchiveService(issuemapPath, issueRepo, configRepo, attachmentRepo)

	// Handle configuration changes
	if err := handleArchiveConfigChanges(archiveService, cmd); err != nil {
		printError(fmt.Errorf("failed to update configuration: %w", err))
		return err
	}

	ctx := context.Background()

	// Handle specific operations
	if archiveList {
		return listArchives(ctx, archiveService)
	}

	if archiveStats {
		return showArchiveStats(ctx, archiveService)
	}

	if archiveSearch != "" {
		return searchArchives(ctx, archiveService, archiveSearch)
	}

	if len(archiveRestore) > 0 {
		return restoreIssues(ctx, archiveService, archiveRestore)
	}

	if archiveVerify != "" {
		return verifyArchive(ctx, archiveService, archiveVerify)
	}

	// Default action: archive issues based on criteria
	return archiveIssues(ctx, archiveService)
}

func handleArchiveConfigChanges(archiveService *services.ArchiveService, cmd *cobra.Command) error {
	config := archiveService.GetConfig()
	changed := false

	if archiveEnable {
		config.Enabled = true
		changed = true
	}

	if archiveDisable {
		config.Enabled = false
		changed = true
	}

	if cmd.Flags().Changed("compression") {
		config.CompressionLevel = archiveCompress
		changed = true
	}

	if archiveFormat != "" {
		config.Format = archiveFormat
		changed = true
	}

	if cmd.Flags().Changed("include-attachments") {
		config.IncludeAttachments = archiveIncludeAtt
		changed = true
	}

	if cmd.Flags().Changed("include-history") {
		config.IncludeHistory = archiveIncludeHist
		changed = true
	}

	if changed {
		if err := archiveService.UpdateConfig(config); err != nil {
			return err
		}

		if !noColor {
			color.Green("✓ Archive configuration updated")
		} else {
			fmt.Println("✓ Archive configuration updated")
		}
	}

	return nil
}

func archiveIssues(ctx context.Context, archiveService *services.ArchiveService) error {
	// Build filter criteria
	filter := &entities.ArchiveFilter{}

	if archiveClosed {
		closedStatus := entities.StatusClosed
		filter.Status = &closedStatus
	}

	if archiveOlderThan != "" {
		duration, err := parseDuration(archiveOlderThan)
		if err != nil {
			return fmt.Errorf("invalid duration format: %v", err)
		}
		cutoff := time.Now().Add(-duration)
		filter.ClosedBefore = &cutoff
	}

	if archiveMinAge > 0 {
		filter.MinAgeDays = &archiveMinAge
	}

	if len(archiveIssueIDs) > 0 {
		for _, idStr := range archiveIssueIDs {
			filter.IssueIDs = append(filter.IssueIDs, entities.IssueID(idStr))
		}
	}

	// Perform archive operation
	if !noColor {
		if archiveDryRun {
			color.Yellow("Analyzing issues for archival...")
		} else {
			color.Yellow("Archiving issues...")
		}
	} else {
		if archiveDryRun {
			fmt.Println("Analyzing issues for archival...")
		} else {
			fmt.Println("Archiving issues...")
		}
	}

	result, err := archiveService.ArchiveIssues(ctx, filter, archiveDryRun)
	if err != nil {
		return fmt.Errorf("failed to archive issues: %w", err)
	}

	if archiveJSON {
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayArchiveResult(result)
	return nil
}

func listArchives(ctx context.Context, archiveService *services.ArchiveService) error {
	archives, err := archiveService.ListArchives()
	if err != nil {
		return fmt.Errorf("failed to list archives: %w", err)
	}

	if len(archives) == 0 {
		fmt.Println("No archives found")
		return nil
	}

	if archiveJSON {
		jsonData, err := json.MarshalIndent(archives, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	if !noColor {
		color.Cyan("Archives:")
		color.HiBlack("=========")
	} else {
		fmt.Println("Archives:")
		fmt.Println("=========")
	}

	for i, archive := range archives {
		fmt.Printf("%d. %s\n", i+1, archive)
	}

	return nil
}

func showArchiveStats(ctx context.Context, archiveService *services.ArchiveService) error {
	stats, err := archiveService.GetArchiveStats()
	if err != nil {
		return fmt.Errorf("failed to get archive stats: %w", err)
	}

	if archiveJSON {
		jsonData, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayArchiveStats(stats)
	return nil
}

func searchArchives(ctx context.Context, archiveService *services.ArchiveService, query string) error {
	results, err := archiveService.SearchArchives(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to search archives: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No archived issues found matching '%s'\n", query)
		return nil
	}

	if archiveJSON {
		jsonData, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displaySearchResults(results, query)
	return nil
}

func restoreIssues(ctx context.Context, archiveService *services.ArchiveService, issueIDs []string) error {
	var entityIDs []entities.IssueID
	for _, idStr := range issueIDs {
		entityIDs = append(entityIDs, entities.IssueID(idStr))
	}

	if !noColor {
		if archiveDryRun {
			color.Yellow("Analyzing issues for restoration...")
		} else {
			color.Yellow("Restoring issues from archive...")
		}
	} else {
		if archiveDryRun {
			fmt.Println("Analyzing issues for restoration...")
		} else {
			fmt.Println("Restoring issues from archive...")
		}
	}

	result, err := archiveService.RestoreIssues(ctx, entityIDs, archiveDryRun)
	if err != nil {
		return fmt.Errorf("failed to restore issues: %w", err)
	}

	if archiveJSON {
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayArchiveRestoreResult(result)
	return nil
}

func verifyArchive(ctx context.Context, archiveService *services.ArchiveService, archiveName string) error {
	if !noColor {
		color.Yellow("Verifying archive integrity...")
	} else {
		fmt.Println("Verifying archive integrity...")
	}

	err := archiveService.VerifyArchive(ctx, archiveName)
	if err != nil {
		if !noColor {
			color.Red("✗ Archive verification failed: %v", err)
		} else {
			fmt.Printf("✗ Archive verification failed: %v\n", err)
		}
		return err
	}

	if !noColor {
		color.Green("✓ Archive '%s' integrity verified successfully", archiveName)
	} else {
		fmt.Printf("✓ Archive '%s' integrity verified successfully\n", archiveName)
	}

	return nil
}

func parseDuration(s string) (time.Duration, error) {
	// Handle common patterns like "6m" (6 months), "1y" (1 year)
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration format")
	}

	unit := s[len(s)-1:]
	valueStr := s[:len(s)-1]
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("invalid number in duration: %s", valueStr)
	}

	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case "m":
		return time.Duration(value) * 30 * 24 * time.Hour, nil // Approximate month
	case "y":
		return time.Duration(value) * 365 * 24 * time.Hour, nil // Approximate year
	default:
		// Try standard Go duration parsing
		return time.ParseDuration(s)
	}
}

func displayArchiveResult(result *entities.ArchiveResult) {
	if !noColor {
		color.Cyan("Archive Result:")
		color.HiBlack("===============")
	} else {
		fmt.Println("Archive Result:")
		fmt.Println("===============")
	}

	fmt.Printf("  %s %s\n", colorLabel("Completed:"), result.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("  %s %v\n", colorLabel("Duration:"), result.Duration.Round(time.Millisecond))
	fmt.Printf("  %s %v\n", colorLabel("Dry Run:"), result.DryRun)
	fmt.Printf("  %s %d\n", colorLabel("Issues Archived:"), result.IssuesArchived)

	if result.ArchiveFile != "" {
		fmt.Printf("  %s %s\n", colorLabel("Archive File:"), colorValue(result.ArchiveFile))
	}

	fmt.Printf("  %s %s\n", colorLabel("Original Size:"), entities.FormatBytes(result.OriginalSize))
	fmt.Printf("  %s %s\n", colorLabel("Compressed Size:"), entities.FormatBytes(result.CompressedSize))
	fmt.Printf("  %s %.1f%%\n", colorLabel("Compression Ratio:"), result.CompressionRatio*100)

	if len(result.ArchivedIssues) > 0 && len(result.ArchivedIssues) <= 10 {
		fmt.Println()
		if !noColor {
			color.Yellow("Archived Issues:")
		} else {
			fmt.Println("Archived Issues:")
		}
		for _, issueID := range result.ArchivedIssues {
			fmt.Printf("  - %s\n", issueID)
		}
	} else if len(result.ArchivedIssues) > 10 {
		fmt.Printf("\n  %s %d issues (too many to list)\n", colorLabel("Archived:"), len(result.ArchivedIssues))
	}

	if len(result.Errors) > 0 {
		fmt.Println()
		if !noColor {
			color.Red("Errors:")
		} else {
			fmt.Println("Errors:")
		}
		for _, archiveError := range result.Errors {
			fmt.Printf("  - %s\n", archiveError)
		}
	} else if result.IssuesArchived > 0 {
		fmt.Println()
		if !noColor {
			if result.DryRun {
				color.Green("✓ Analysis completed successfully")
			} else {
				color.Green("✓ Archive operation completed successfully")
			}
		} else {
			if result.DryRun {
				fmt.Println("✓ Analysis completed successfully")
			} else {
				fmt.Println("✓ Archive operation completed successfully")
			}
		}
	}
}

func displayArchiveStats(stats *entities.ArchiveStats) {
	if !noColor {
		color.Cyan("Archive Statistics:")
		color.HiBlack("===================")
	} else {
		fmt.Println("Archive Statistics:")
		fmt.Println("===================")
	}

	fmt.Printf("  %s %d\n", colorLabel("Total Archives:"), stats.TotalArchives)
	fmt.Printf("  %s %d\n", colorLabel("Archived Issues:"), stats.TotalArchivedIssues)
	fmt.Printf("  %s %s\n", colorLabel("Compressed Size:"), entities.FormatBytes(stats.TotalCompressedSize))
	fmt.Printf("  %s %s\n", colorLabel("Original Size:"), entities.FormatBytes(stats.TotalOriginalSize))
	fmt.Printf("  %s %s\n", colorLabel("Space Saved:"), entities.FormatBytes(stats.SpaceSaved))
	fmt.Printf("  %s %.1f%%\n", colorLabel("Compression Ratio:"), stats.CompressionRatio*100)

	if stats.OldestIssue != nil {
		fmt.Printf("  %s %s\n", colorLabel("Oldest Issue:"), stats.OldestIssue.Format("2006-01-02"))
	}

	if stats.NewestIssue != nil {
		fmt.Printf("  %s %s\n", colorLabel("Newest Issue:"), stats.NewestIssue.Format("2006-01-02"))
	}

	if len(stats.ArchivesByPeriod) > 0 {
		fmt.Println()
		if !noColor {
			color.Yellow("Archives by Period:")
		} else {
			fmt.Println("Archives by Period:")
		}

		// Sort periods
		var periods []string
		for period := range stats.ArchivesByPeriod {
			periods = append(periods, period)
		}
		for _, period := range periods {
			fmt.Printf("  %s: %d issues\n", period, stats.ArchivesByPeriod[period])
		}
	}
}

func displaySearchResults(results []*entities.SearchResult, query string) {
	if !noColor {
		color.Cyan("Search Results for '%s':", query)
		color.HiBlack(strings.Repeat("=", len(query)+20))
	} else {
		fmt.Printf("Search Results for '%s':\n", query)
		fmt.Println(strings.Repeat("=", len(query)+20))
	}

	for i, result := range results {
		if i >= 20 { // Limit to top 20 results
			fmt.Printf("... and %d more results\n", len(results)-20)
			break
		}

		entry := result.Entry
		fmt.Printf("\n%d. %s %s\n", i+1, colorValue(string(entry.IssueID)), entry.Title)
		fmt.Printf("   %s %s | %s %s | %s %s\n",
			colorLabel("Type:"), entry.Type,
			colorLabel("Status:"), entry.Status,
			colorLabel("Archive:"), result.ArchiveFile)
		fmt.Printf("   %s %s | %s %.1f\n",
			colorLabel("Archived:"), entry.ArchivedAt.Format("2006-01-02"),
			colorLabel("Score:"), result.Score)

		if len(result.MatchedFields) > 0 {
			fmt.Printf("   %s %s\n", colorLabel("Matched:"), strings.Join(result.MatchedFields, ", "))
		}
	}
}

func displayArchiveRestoreResult(result *entities.ArchiveRestoreResult) {
	if !noColor {
		color.Cyan("Restore Result:")
		color.HiBlack("===============")
	} else {
		fmt.Println("Restore Result:")
		fmt.Println("===============")
	}

	fmt.Printf("  %s %s\n", colorLabel("Completed:"), result.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("  %s %v\n", colorLabel("Duration:"), result.Duration.Round(time.Millisecond))
	fmt.Printf("  %s %v\n", colorLabel("Dry Run:"), result.DryRun)
	fmt.Printf("  %s %d\n", colorLabel("Issues Restored:"), result.IssuesRestored)

	if len(result.RestoredIssues) > 0 {
		fmt.Println()
		if !noColor {
			color.Yellow("Restored Issues:")
		} else {
			fmt.Println("Restored Issues:")
		}
		for _, issueID := range result.RestoredIssues {
			fmt.Printf("  - %s\n", issueID)
		}
	}

	if len(result.Errors) > 0 {
		fmt.Println()
		if !noColor {
			color.Red("Errors:")
		} else {
			fmt.Println("Errors:")
		}
		for _, restoreError := range result.Errors {
			fmt.Printf("  - %s\n", restoreError)
		}
	} else if result.IssuesRestored > 0 {
		fmt.Println()
		if !noColor {
			if result.DryRun {
				color.Green("✓ Restore analysis completed successfully")
			} else {
				color.Green("✓ Restore operation completed successfully")
			}
		} else {
			if result.DryRun {
				fmt.Println("✓ Restore analysis completed successfully")
			} else {
				fmt.Println("✓ Restore operation completed successfully")
			}
		}
	}
}
