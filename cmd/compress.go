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
	compressEnable     bool
	compressDisable    bool
	compressLevel      int
	compressMinSize    string
	compressMaxSize    string
	compressMinRatio   float64
	compressBackground bool
	compressBatchSize  int
	compressStats      bool
	compressJSON       bool
	compressRunBatch   bool
)

// compressCmd represents the compress command
var compressCmd = &cobra.Command{
	Use:   "compress",
	Short: "Manage attachment compression",
	Long: `Manage attachment compression settings and operations.

This command allows you to:
- Configure compression settings
- View compression statistics  
- Run batch compression on existing files
- Enable/disable background compression

Examples:
  issuemap compress --stats                    # Show compression statistics
  issuemap compress --enable --level 6        # Enable compression with level 6
  issuemap compress --run-batch --batch-size 10  # Compress existing files
  issuemap compress --disable                 # Disable compression`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCompress(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(compressCmd)

	// Configuration flags
	compressCmd.Flags().BoolVar(&compressEnable, "enable", false, "enable compression")
	compressCmd.Flags().BoolVar(&compressDisable, "disable", false, "disable compression")
	compressCmd.Flags().IntVar(&compressLevel, "level", 0, "compression level (1-9)")
	compressCmd.Flags().StringVar(&compressMinSize, "min-size", "", "minimum file size for compression (e.g., 1KB)")
	compressCmd.Flags().StringVar(&compressMaxSize, "max-size", "", "maximum file size for compression (e.g., 50MB)")
	compressCmd.Flags().Float64Var(&compressMinRatio, "min-ratio", 0.0, "minimum compression ratio to keep compressed (0.0-1.0)")
	compressCmd.Flags().BoolVar(&compressBackground, "background", false, "enable background compression")
	compressCmd.Flags().IntVar(&compressBatchSize, "batch-size", 0, "batch size for background compression")

	// Operation flags
	compressCmd.Flags().BoolVar(&compressStats, "stats", false, "show compression statistics")
	compressCmd.Flags().BoolVar(&compressRunBatch, "run-batch", false, "run batch compression on existing files")

	// Output flags
	compressCmd.Flags().BoolVar(&compressJSON, "json", false, "output in JSON format")
}

func runCompress(cmd *cobra.Command, args []string) error {
	// Initialize paths
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")

	// Initialize repositories and services
	configRepo := storage.NewFileConfigRepository(issuemapPath)
	attachmentRepo := storage.NewFileAttachmentRepository(issuemapPath)

	// Create compression service
	compressionService := services.NewCompressionService(issuemapPath, configRepo, attachmentRepo)

	// Handle configuration changes
	if err := handleCompressionConfigChanges(compressionService, cmd); err != nil {
		printError(fmt.Errorf("failed to update compression configuration: %w", err))
		return err
	}

	ctx := context.Background()

	// Handle specific operations
	if compressStats {
		return showCompressionStats(compressionService)
	}

	if compressRunBatch {
		return runBatchCompression(ctx, compressionService)
	}

	// If no specific operation, show current configuration
	return showCompressionConfig(compressionService)
}

func handleCompressionConfigChanges(compressionService *services.CompressionService, cmd *cobra.Command) error {
	config := compressionService.GetConfig()
	changed := false

	if compressEnable {
		config.Enabled = true
		changed = true
	}

	if compressDisable {
		config.Enabled = false
		changed = true
	}

	if cmd.Flags().Changed("level") {
		if compressLevel < 1 || compressLevel > 9 {
			return fmt.Errorf("compression level must be between 1 and 9")
		}
		config.Level = compressLevel
		changed = true
	}

	if compressMinSize != "" {
		size, err := parseCompressSize(compressMinSize)
		if err != nil {
			return fmt.Errorf("invalid min-size format: %v", err)
		}
		config.MinFileSize = size
		changed = true
	}

	if compressMaxSize != "" {
		size, err := parseCompressSize(compressMaxSize)
		if err != nil {
			return fmt.Errorf("invalid max-size format: %v", err)
		}
		config.MaxFileSize = size
		changed = true
	}

	if cmd.Flags().Changed("min-ratio") {
		if compressMinRatio < 0.0 || compressMinRatio > 1.0 {
			return fmt.Errorf("compression ratio must be between 0.0 and 1.0")
		}
		config.MinCompressionRatio = compressMinRatio
		changed = true
	}

	if cmd.Flags().Changed("background") {
		config.BackgroundCompression = compressBackground
		changed = true
	}

	if cmd.Flags().Changed("batch-size") {
		if compressBatchSize < 1 {
			return fmt.Errorf("batch size must be at least 1")
		}
		config.BackgroundBatchSize = compressBatchSize
		changed = true
	}

	if changed {
		if err := compressionService.UpdateConfig(config); err != nil {
			return err
		}

		if !noColor {
			color.Green("✓ Compression configuration updated")
		} else {
			fmt.Println("✓ Compression configuration updated")
		}
	}

	return nil
}

func showCompressionStats(compressionService *services.CompressionService) error {
	stats := compressionService.GetStats()

	if compressJSON {
		jsonData, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayCompressionStats(stats)
	return nil
}

func showCompressionConfig(compressionService *services.CompressionService) error {
	config := compressionService.GetConfig()

	if compressJSON {
		jsonData, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayCompressionConfig(config)
	return nil
}

func runBatchCompression(ctx context.Context, compressionService *services.CompressionService) error {
	batchSize := compressBatchSize
	if batchSize == 0 {
		batchSize = 10 // Default batch size
	}

	if !noColor {
		color.Yellow("🗜️ Running batch compression...")
	} else {
		fmt.Println("🗜️ Running batch compression...")
	}

	result, err := compressionService.RunBatchCompression(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("failed to run batch compression: %w", err)
	}

	if compressJSON {
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	displayCompressionResult(result)
	return nil
}

func displayCompressionStats(stats *entities.CompressionStats) {
	if !noColor {
		color.Cyan("Compression Statistics:")
		color.HiBlack("=====================")
	} else {
		fmt.Println("Compression Statistics:")
		fmt.Println("=====================")
	}

	fmt.Printf("  %s %d\n", colorLabel("Total Files:"), stats.TotalFiles)
	fmt.Printf("  %s %d\n", colorLabel("Compressed Files:"), stats.CompressedFiles)
	fmt.Printf("  %s %d\n", colorLabel("Skipped Files:"), stats.SkippedFiles)

	if stats.TotalOriginalSize > 0 {
		fmt.Printf("  %s %s\n", colorLabel("Original Size:"), entities.FormatBytes(stats.TotalOriginalSize))
		fmt.Printf("  %s %s\n", colorLabel("Compressed Size:"), entities.FormatBytes(stats.TotalCompressedSize))
		fmt.Printf("  %s %s\n", colorLabel("Space Saved:"), entities.FormatBytes(stats.SpaceSaved))
		fmt.Printf("  %s %.1f%%\n", colorLabel("Overall Compression:"), stats.OverallCompressionRatio*100)

		if stats.CompressedFiles > 0 {
			fmt.Printf("  %s %.1f%%\n", colorLabel("Avg Compression:"), stats.AverageCompressionRatio*100)
		}
	}

	if len(stats.CompressionByType) > 0 {
		fmt.Println()
		if !noColor {
			color.Yellow("Compression by File Type:")
		} else {
			fmt.Println("Compression by File Type:")
		}

		for ext, typeStats := range stats.CompressionByType {
			if typeStats.CompressedCount > 0 {
				fmt.Printf("  %s: %d/%d files, %.1f%% avg compression\n",
					ext, typeStats.CompressedCount, typeStats.FileCount, typeStats.AverageRatio*100)
			}
		}
	}
}

func displayCompressionConfig(config *entities.CompressionConfig) {
	if !noColor {
		color.Cyan("Compression Configuration:")
		color.HiBlack("=========================")
	} else {
		fmt.Println("Compression Configuration:")
		fmt.Println("=========================")
	}

	fmt.Printf("  %s %v\n", colorLabel("Enabled:"), config.Enabled)
	fmt.Printf("  %s %d\n", colorLabel("Level:"), config.Level)
	fmt.Printf("  %s %s\n", colorLabel("Min File Size:"), entities.FormatBytes(config.MinFileSize))
	fmt.Printf("  %s %s\n", colorLabel("Max File Size:"), entities.FormatBytes(config.MaxFileSize))
	fmt.Printf("  %s %.1f%%\n", colorLabel("Min Compression Ratio:"), config.MinCompressionRatio*100)
	fmt.Printf("  %s %v\n", colorLabel("Background Compression:"), config.BackgroundCompression)
	fmt.Printf("  %s %d\n", colorLabel("Background Batch Size:"), config.BackgroundBatchSize)

	if len(config.CompressibleExtensions) > 0 {
		fmt.Printf("  %s %d extensions\n", colorLabel("Compressible Types:"), len(config.CompressibleExtensions))
	}

	if len(config.SkipExtensions) > 0 {
		fmt.Printf("  %s %d extensions\n", colorLabel("Skip Types:"), len(config.SkipExtensions))
	}
}

func displayCompressionResult(result *entities.CompressionResult) {
	if !noColor {
		color.Cyan("Batch Compression Result:")
		color.HiBlack("========================")
	} else {
		fmt.Println("Batch Compression Result:")
		fmt.Println("========================")
	}

	fmt.Printf("  %s %v\n", colorLabel("Success:"), result.Success)
	fmt.Printf("  %s %v\n", colorLabel("Duration:"), result.Duration.Round(time.Millisecond))

	if result.Error != "" {
		fmt.Printf("  %s %s\n", colorLabel("Error:"), result.Error)
	}
}

func parseCompressSize(sizeStr string) (int64, error) {
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
