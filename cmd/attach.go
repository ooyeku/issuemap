package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	attachDescription string
	attachContentType string
	attachForce       bool
)

// attachCmd represents the attach command
var attachCmd = &cobra.Command{
	Use:   "attach <issue-id> <file1> [file2] [file3]...",
	Short: "Attach files to an issue",
	Long: `Attach one or more files to an existing issue.

This command allows you to upload files and associate them with an issue.
The files will be stored securely and will be available through the web UI
and API endpoints.

Examples:
  issuemap attach ISSUEMAP-100 screenshot.png       # Attach single file
  issuemap attach ISSUEMAP-100 *.log                # Attach multiple files
  issuemap attach ISSUEMAP-100 doc.pdf --description "Requirements document"
  issuemap attach ISSUEMAP-100 data.json --content-type "application/json"`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID := args[0]
		filePaths := args[1:]
		return runAttach(cmd, issueID, filePaths)
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)

	// Flags
	attachCmd.Flags().StringVarP(&attachDescription, "description", "d", "", "description for the attachment(s)")
	attachCmd.Flags().StringVar(&attachContentType, "content-type", "", "override content type (MIME type) for the attachment(s)")
	attachCmd.Flags().BoolVar(&attachForce, "force", false, "force upload even if file validation warnings occur")
}

func runAttach(cmd *cobra.Command, issueIDStr string, filePaths []string) error {
	ctx := context.Background()

	// Find git repository root
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	// Initialize repositories and services
	issuemapPath := filepath.Join(repoPath, app.ConfigDirName)
	issueRepo := storage.NewFileIssueRepository(issuemapPath)
	configRepo := storage.NewFileConfigRepository(issuemapPath)
	attachmentRepo := storage.NewFileAttachmentRepository(issuemapPath)

	// Initialize git client for user detection
	gitRepo, err := git.NewGitClient(repoPath)
	if err != nil {
		printError(fmt.Errorf("failed to initialize git client: %w", err))
		return err
	}

	// Get current user from git config
	currentUser := getAttachCurrentUser(gitRepo)

	// Parse issue ID
	issueID := entities.IssueID(issueIDStr)

	// Verify issue exists
	issue, err := issueRepo.GetByID(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("issue %s not found: %w", issueIDStr, err))
		return err
	}

	// Create services
	storageService := services.NewStorageService(issuemapPath, configRepo, issueRepo, attachmentRepo)
	attachmentService := services.NewAttachmentService(attachmentRepo, issueRepo, storageService, issuemapPath)

	// Set up optional services
	if dedupService := services.NewDeduplicationService(issuemapPath, configRepo); dedupService != nil {
		attachmentService.SetDeduplicationService(dedupService)
	}
	if compressionService := services.NewCompressionService(issuemapPath, configRepo, attachmentRepo); compressionService != nil {
		attachmentService.SetCompressionService(compressionService)
	}

	// Expand file paths (handle globs)
	expandedPaths, err := expandFilePaths(filePaths)
	if err != nil {
		printError(fmt.Errorf("failed to expand file paths: %w", err))
		return err
	}

	if len(expandedPaths) == 0 {
		printError(fmt.Errorf("no files found matching the specified patterns"))
		return fmt.Errorf("no files found")
	}

	// Upload each file
	var uploaded []*entities.Attachment
	var failed []string

	for _, filePath := range expandedPaths {
		if !noColor {
			color.Yellow("📎 Uploading %s...", filePath)
		} else {
			fmt.Printf("📎 Uploading %s...\n", filePath)
		}

		attachment, err := uploadSingleFile(ctx, attachmentService, issueID, filePath, currentUser)
		if err != nil {
			if !noColor {
				color.Red("✗ Failed to upload %s: %v", filePath, err)
			} else {
				fmt.Printf("✗ Failed to upload %s: %v\n", filePath, err)
			}
			failed = append(failed, filePath)

			if !attachForce {
				// If not forcing, stop on first error
				break
			}
			continue
		}

		uploaded = append(uploaded, attachment)
		if !noColor {
			color.Green("✓ Successfully uploaded %s (%s)", attachment.Filename, attachment.GetSizeFormatted())
		} else {
			fmt.Printf("✓ Successfully uploaded %s (%s)\n", attachment.Filename, attachment.GetSizeFormatted())
		}
	}

	// Summary
	fmt.Println()
	if len(uploaded) > 0 {
		if !noColor {
			color.Cyan("Attachment Summary:")
			color.HiBlack("==================")
		} else {
			fmt.Println("Attachment Summary:")
			fmt.Println("==================")
		}

		fmt.Printf("Issue: %s (%s)\n", issue.ID, issue.Title)
		fmt.Printf("Successfully uploaded: %d file(s)\n", len(uploaded))

		if len(failed) > 0 {
			fmt.Printf("Failed uploads: %d file(s)\n", len(failed))
		}

		fmt.Println("\nAttached files:")
		for _, attachment := range uploaded {
			fmt.Printf("  • %s (%s) - %s\n",
				attachment.Filename,
				attachment.GetSizeFormatted(),
				attachment.Type)
			if attachment.Description != "" {
				fmt.Printf("    Description: %s\n", attachment.Description)
			}
		}
	} else {
		if !noColor {
			color.Red("No files were successfully uploaded.")
		} else {
			fmt.Println("No files were successfully uploaded.")
		}
		return fmt.Errorf("upload failed")
	}

	return nil
}

func uploadSingleFile(ctx context.Context, attachmentService *services.AttachmentService, issueID entities.IssueID, filePath, uploadedBy string) (*entities.Attachment, error) {
	// Check if file exists and get info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file not accessible: %w", err)
	}

	if fileInfo.IsDir() {
		return nil, fmt.Errorf("cannot attach directory: %s", filePath)
	}

	// Open file for reading
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get filename
	filename := filepath.Base(filePath)

	// Upload attachment
	attachment, err := attachmentService.UploadAttachment(
		ctx,
		issueID,
		filename,
		file,
		fileInfo.Size(),
		uploadedBy,
	)
	if err != nil {
		return nil, err
	}

	// Set description if provided
	if attachDescription != "" {
		if err := attachmentService.UpdateDescription(ctx, attachment.ID, attachDescription); err != nil {
			// Don't fail the upload if description update fails
			fmt.Printf("Warning: failed to set description: %v\n", err)
		}
	}

	return attachment, nil
}

func expandFilePaths(patterns []string) ([]string, error) {
	var expandedPaths []string

	for _, pattern := range patterns {
		// Use filepath.Glob to expand wildcards
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}

		if len(matches) == 0 {
			// If no glob matches, check if it's a literal file path
			if _, err := os.Stat(pattern); err == nil {
				matches = []string{pattern}
			} else {
				return nil, fmt.Errorf("no files found matching pattern: %s", pattern)
			}
		}

		expandedPaths = append(expandedPaths, matches...)
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var unique []string
	for _, path := range expandedPaths {
		if !seen[path] {
			seen[path] = true
			unique = append(unique, path)
		}
	}

	return unique, nil
}

func getAttachCurrentUser(gitRepo *git.GitClient) string {
	ctx := context.Background()
	if gitRepo != nil {
		if author, err := gitRepo.GetAuthorInfo(ctx); err == nil && author != nil {
			if author.Username != "" {
				return author.Username
			}
			if author.Email != "" {
				return author.Email
			}
		}
	}

	// Fallback to environment variables
	if user := os.Getenv("GIT_AUTHOR_NAME"); user != "" {
		return user
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}

	// Final fallback
	return "unknown"
}
