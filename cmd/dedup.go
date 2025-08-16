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
	dedupMigrate     bool
	dedupStats       bool
	dedupReport      bool
	dedupValidate    bool
	dedupDryRun      bool
	dedupJSON        bool
	dedupEnable      bool
	dedupDisable     bool
	dedupHashAlgo    string
	dedupMinSize     int64
	dedupMaxSize     int64
	dedupAutoMigrate bool
)

// dedupCmd represents the dedup command
var dedupCmd = &cobra.Command{
	Use:   "dedup",
	Short: "Manage file deduplication",
	Long: `Manage file deduplication for attachments.

This command provides tools to configure, migrate, and monitor file deduplication
to save storage space by eliminating duplicate files.

Examples:
  issuemap dedup --stats                    # Show deduplication statistics
  issuemap dedup --migrate --dry-run        # Preview migration of existing duplicates
  issuemap dedup --migrate                  # Migrate existing duplicate files
  issuemap dedup --report                   # Generate detailed deduplication report
  issuemap dedup --validate                 # Validate deduplicated files integrity
  issuemap dedup --enable                   # Enable deduplication for new uploads
  issuemap dedup --disable                  # Disable deduplication`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDedup(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(dedupCmd)

	// Action flags
	dedupCmd.Flags().BoolVar(&dedupMigrate, "migrate", false, "migrate existing duplicate files")
	dedupCmd.Flags().BoolVar(&dedupStats, "stats", false, "show deduplication statistics")
	dedupCmd.Flags().BoolVar(&dedupReport, "report", false, "generate detailed deduplication report")
	dedupCmd.Flags().BoolVar(&dedupValidate, "validate", false, "validate deduplicated files integrity")
	dedupCmd.Flags().BoolVar(&dedupEnable, "enable", false, "enable deduplication")
	dedupCmd.Flags().BoolVar(&dedupDisable, "disable", false, "disable deduplication")

	// Configuration flags
	dedupCmd.Flags().StringVar(&dedupHashAlgo, "hash-algorithm", "", "hash algorithm (sha256, sha1, md5)")
	dedupCmd.Flags().Int64Var(&dedupMinSize, "min-size", 0, "minimum file size for deduplication (bytes)")
	dedupCmd.Flags().Int64Var(&dedupMaxSize, "max-size", 0, "maximum file size for deduplication (bytes, 0=no limit)")
	dedupCmd.Flags().BoolVar(&dedupAutoMigrate, "auto-migrate", false, "enable automatic migration of existing duplicates")

	// Common flags
	dedupCmd.Flags().BoolVar(&dedupDryRun, "dry-run", false, "preview changes without making them")
	dedupCmd.Flags().BoolVar(&dedupJSON, "json", false, "output in JSON format")
}

func runDedup(cmd *cobra.Command, args []string) error {
	// Initialize paths
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")

	// Initialize repositories
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	// Initialize deduplication service
	dedupService := services.NewDeduplicationService(issuemapPath, configRepo)

	// Handle configuration changes
	if err := handleConfigChanges(dedupService, cmd); err != nil {
		printError(fmt.Errorf("failed to update configuration: %w", err))
		return err
	}

	// Handle main operations
	ctx := context.Background()

	if dedupStats {
		return showDeduplicationStats(ctx, dedupService)
	}

	if dedupReport {
		return generateDeduplicationReport(ctx, issuemapPath, dedupService)
	}

	if dedupMigrate {
		return migrateExistingFiles(ctx, issuemapPath, dedupService)
	}

	if dedupValidate {
		return validateDeduplication(ctx, issuemapPath, dedupService)
	}

	// If no specific action, show current configuration and stats
	return showDeduplicationStatus(ctx, dedupService)
}

func handleConfigChanges(dedupService *services.DeduplicationService, cmd *cobra.Command) error {
	config := dedupService.GetConfig()
	changed := false

	if dedupEnable {
		config.Enabled = true
		changed = true
	}

	if dedupDisable {
		config.Enabled = false
		changed = true
	}

	if dedupHashAlgo != "" {
		config.HashAlgorithm = dedupHashAlgo
		changed = true
	}

	if cmd.Flags().Changed("min-size") {
		config.MinFileSize = dedupMinSize
		changed = true
	}

	if cmd.Flags().Changed("max-size") {
		config.MaxFileSize = dedupMaxSize
		changed = true
	}

	if cmd.Flags().Changed("auto-migrate") {
		config.AutoMigrate = dedupAutoMigrate
		changed = true
	}

	if changed {
		if err := dedupService.UpdateConfig(config); err != nil {
			return err
		}

		if !noColor {
			color.Green("✓ Deduplication configuration updated")
		} else {
			fmt.Println("✓ Deduplication configuration updated")
		}
	}

	return nil
}

func showDeduplicationStats(ctx context.Context, dedupService *services.DeduplicationService) error {
	stats, err := dedupService.GetDeduplicationStats()
	if err != nil {
		return fmt.Errorf("failed to get deduplication stats: %w", err)
	}

	if dedupJSON {
		jsonData, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayDeduplicationStats(stats)
	return nil
}

func generateDeduplicationReport(ctx context.Context, basePath string, dedupService *services.DeduplicationService) error {
	// Initialize attachment repository for migration service
	attachmentRepo := storage.NewFileAttachmentRepository(basePath)
	migration := services.NewDeduplicationMigration(dedupService, attachmentRepo, basePath)

	report, err := migration.GenerateMigrationReport(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate deduplication report: %w", err)
	}

	if dedupJSON {
		jsonData, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayDeduplicationReport(report)
	return nil
}

func migrateExistingFiles(ctx context.Context, basePath string, dedupService *services.DeduplicationService) error {
	// Initialize attachment repository for migration service
	attachmentRepo := storage.NewFileAttachmentRepository(basePath)
	migration := services.NewDeduplicationMigration(dedupService, attachmentRepo, basePath)

	if !noColor {
		if dedupDryRun {
			color.Yellow("🔍 Analyzing existing files for deduplication opportunities...")
		} else {
			color.Yellow("🔄 Migrating existing duplicate files...")
		}
	} else {
		if dedupDryRun {
			fmt.Println("🔍 Analyzing existing files for deduplication opportunities...")
		} else {
			fmt.Println("🔄 Migrating existing duplicate files...")
		}
	}

	result, err := migration.MigrateExistingFiles(ctx, dedupDryRun)
	if err != nil {
		return fmt.Errorf("failed to migrate existing files: %w", err)
	}

	if dedupJSON {
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayMigrationResult(result)
	return nil
}

func validateDeduplication(ctx context.Context, basePath string, dedupService *services.DeduplicationService) error {
	// Initialize attachment repository for migration service
	attachmentRepo := storage.NewFileAttachmentRepository(basePath)
	migration := services.NewDeduplicationMigration(dedupService, attachmentRepo, basePath)

	if !noColor {
		color.Yellow("🔍 Validating deduplicated files integrity...")
	} else {
		fmt.Println("🔍 Validating deduplicated files integrity...")
	}

	errors, err := migration.ValidateMigration(ctx)
	if err != nil {
		return fmt.Errorf("failed to validate deduplication: %w", err)
	}

	if len(errors) == 0 {
		if !noColor {
			color.Green("✓ All deduplicated files are valid")
		} else {
			fmt.Println("✓ All deduplicated files are valid")
		}
	} else {
		if !noColor {
			color.Red("✗ Found %d validation errors:", len(errors))
		} else {
			fmt.Printf("✗ Found %d validation errors:\n", len(errors))
		}

		for _, validationError := range errors {
			fmt.Printf("  - %s\n", validationError)
		}
	}

	return nil
}

func showDeduplicationStatus(ctx context.Context, dedupService *services.DeduplicationService) error {
	config := dedupService.GetConfig()

	if !noColor {
		color.Cyan("Deduplication Configuration:")
		color.HiBlack("============================")
	} else {
		fmt.Println("Deduplication Configuration:")
		fmt.Println("============================")
	}

	fmt.Printf("  %s %v\n", colorLabel("Enabled:"), config.Enabled)
	fmt.Printf("  %s %s\n", colorLabel("Hash Algorithm:"), colorValue(config.HashAlgorithm))
	fmt.Printf("  %s %s\n", colorLabel("Min File Size:"), entities.FormatBytes(config.MinFileSize))
	if config.MaxFileSize > 0 {
		fmt.Printf("  %s %s\n", colorLabel("Max File Size:"), entities.FormatBytes(config.MaxFileSize))
	} else {
		fmt.Printf("  %s %s\n", colorLabel("Max File Size:"), colorValue("no limit"))
	}
	fmt.Printf("  %s %v\n", colorLabel("Auto Migration:"), config.AutoMigrate)

	// Show current stats
	fmt.Println()
	return showDeduplicationStats(ctx, dedupService)
}

func displayDeduplicationStats(stats *entities.DeduplicationStats) {
	if !noColor {
		color.Cyan("Deduplication Statistics:")
		color.HiBlack("=========================")
	} else {
		fmt.Println("Deduplication Statistics:")
		fmt.Println("=========================")
	}

	fmt.Printf("  %s %d\n", colorLabel("Unique Files:"), stats.UniqueFiles)
	fmt.Printf("  %s %d\n", colorLabel("Total References:"), stats.TotalReferences)
	fmt.Printf("  %s %d\n", colorLabel("Deduplicated Files:"), stats.DeduplicatedFiles)
	fmt.Printf("  %s %s\n", colorLabel("Unique Size:"), entities.FormatBytes(stats.UniqueSize))
	fmt.Printf("  %s %s\n", colorLabel("Total Size (no dedup):"), entities.FormatBytes(stats.TotalSizeWithoutDedup))
	fmt.Printf("  %s %s\n", colorLabel("Space Saved:"), entities.FormatBytes(stats.SpaceSaved))
	fmt.Printf("  %s %.1f%%\n", colorLabel("Deduplication Ratio:"), stats.DeduplicationRatio*100)

	if len(stats.TopDuplicates) > 0 {
		fmt.Println()
		if !noColor {
			color.Cyan("Top Duplicated Files:")
			color.HiBlack("=====================")
		} else {
			fmt.Println("Top Duplicated Files:")
			fmt.Println("=====================")
		}

		for i, duplicate := range stats.TopDuplicates {
			if i >= 5 { // Show top 5
				break
			}
			fmt.Printf("  %d. %s (%d refs, %s saved)\n",
				i+1,
				duplicate.OriginalFilename,
				duplicate.RefCount,
				entities.FormatBytes(duplicate.SpaceSaved))
		}
	}
}

func displayDeduplicationReport(report *entities.DeduplicationReport) {
	if !noColor {
		color.Cyan("Deduplication Report:")
		color.HiBlack("=====================")
	} else {
		fmt.Println("Deduplication Report:")
		fmt.Println("=====================")
	}

	fmt.Printf("  %s %s\n", colorLabel("Generated:"), report.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("  %s %v\n", colorLabel("Duration:"), report.Duration.Round(time.Millisecond))

	// Show configuration
	fmt.Println()
	if !noColor {
		color.Cyan("Configuration:")
	} else {
		fmt.Println("Configuration:")
	}
	fmt.Printf("  %s %v\n", colorLabel("Enabled:"), report.Config.Enabled)
	fmt.Printf("  %s %s\n", colorLabel("Hash Algorithm:"), colorValue(report.Config.HashAlgorithm))

	// Show statistics
	fmt.Println()
	displayDeduplicationStats(&report.Stats)

	// Show potential duplicates
	if len(report.PotentialDuplicates) > 0 {
		fmt.Println()
		if !noColor {
			color.Cyan("Potential Duplicates for Migration:")
			color.HiBlack("===================================")
		} else {
			fmt.Println("Potential Duplicates for Migration:")
			fmt.Println("===================================")
		}

		totalSavings := int64(0)
		for _, group := range report.PotentialDuplicates {
			totalSavings += group.SpaceSavings
		}

		fmt.Printf("  %s %d groups\n", colorLabel("Duplicate Groups:"), len(report.PotentialDuplicates))
		fmt.Printf("  %s %s\n", colorLabel("Potential Savings:"), entities.FormatBytes(totalSavings))

		// Show top 3 groups
		for i, group := range report.PotentialDuplicates {
			if i >= 3 {
				break
			}
			fmt.Printf("  %d. %d files (%s each, %s savings)\n",
				i+1,
				group.Count,
				entities.FormatBytes(group.Size),
				entities.FormatBytes(group.SpaceSavings))
		}

		if len(report.PotentialDuplicates) > 3 {
			fmt.Printf("  ... and %d more groups\n", len(report.PotentialDuplicates)-3)
		}
	}

	// Show errors if any
	if len(report.Errors) > 0 {
		fmt.Println()
		if !noColor {
			color.Red("Errors:")
		} else {
			fmt.Println("Errors:")
		}
		for _, reportError := range report.Errors {
			fmt.Printf("  - %s\n", reportError)
		}
	}
}

func displayMigrationResult(result *entities.MigrationResult) {
	if !noColor {
		color.Cyan("Migration Result:")
		color.HiBlack("=================")
	} else {
		fmt.Println("Migration Result:")
		fmt.Println("=================")
	}

	fmt.Printf("  %s %s\n", colorLabel("Completed:"), result.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("  %s %v\n", colorLabel("Duration:"), result.Duration.Round(time.Millisecond))
	fmt.Printf("  %s %v\n", colorLabel("Dry Run:"), result.DryRun)
	fmt.Printf("  %s %d\n", colorLabel("Files Migrated:"), result.FilesMigrated)
	fmt.Printf("  %s %d\n", colorLabel("Duplicates Removed:"), result.DuplicatesRemoved)
	fmt.Printf("  %s %s\n", colorLabel("Space Reclaimed:"), entities.FormatBytes(result.SpaceReclaimed))

	if len(result.Errors) > 0 {
		fmt.Println()
		if !noColor {
			color.Red("Errors:")
		} else {
			fmt.Println("Errors:")
		}
		for _, migrationError := range result.Errors {
			fmt.Printf("  - %s\n", migrationError)
		}
	} else {
		fmt.Println()
		if !noColor {
			if result.DryRun {
				color.Green("✓ Analysis completed successfully")
			} else {
				color.Green("✓ Migration completed successfully")
			}
		} else {
			if result.DryRun {
				fmt.Println("✓ Analysis completed successfully")
			} else {
				fmt.Println("✓ Migration completed successfully")
			}
		}
	}
}
