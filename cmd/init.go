package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	initProjectName string
	initTemplate    string
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize issuemap in the current git repository",
	Long: `Initialize issuemap in the current git repository by creating the .issuemap directory
and configuration files. This command should be run from the root of a git repository.

The init command will:
- Create .issuemap/ directory structure
- Generate default configuration
- Set up issue templates
- Optionally install git hooks`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&initProjectName, "name", "n", "", "project name (defaults to directory name)")
	initCmd.Flags().StringVarP(&initTemplate, "template", "t", "default", "configuration template to use")
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Check if we're in a git repository
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	// Create configuration
	config := entities.NewDefaultConfig()

	if initProjectName != "" {
		config.Project.Name = initProjectName
	} else {
		// Use directory name as project name
		dir := filepath.Base(repoPath)
		config.Project.Name = dir
	}

	// Initialize the configuration repository
	configRepo := storage.NewFileConfigRepository(filepath.Join(repoPath, app.ConfigDirName))

	// Check if already initialized
	if exists, _ := configRepo.Exists(ctx); exists {
		printWarning("IssueMap is already initialized in this repository")
		return nil
	}

	// Initialize the structure
	if err := configRepo.Initialize(ctx, config); err != nil {
		printError(fmt.Errorf("failed to initialize issuemap: %w", err))
		return err
	}

	// Try to install git hooks (optional)
	if gitClient, err := git.NewGitClient(repoPath); err == nil {
		if err := gitClient.InstallHooks(ctx); err != nil {
			printWarning("Failed to install git hooks (continuing anyway)")
		} else {
			printSuccess("Git hooks installed successfully")
		}
	}

	printSuccess(fmt.Sprintf(app.MsgProjectInitialized, repoPath))
	printInfo("You can now create issues with: issuemap create")

	// Register project globally
	if err := registerProjectGlobally(ctx, repoPath, config.Project.Name); err != nil {
		printWarning("Failed to register project globally (project will still work locally)")
	} else {
		printInfo("Project registered in global issuemap system")
	}

	return nil
}

func printInfo(msg string) {
	if noColor {
		fmt.Println(msg)
	} else {
		color.Cyan(msg)
	}
}

func printError(err error) {
	if noColor {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	} else {
		color.Red("Error: %v", err)
	}
}

func printSuccess(msg string) {
	if noColor {
		fmt.Println(msg)
	} else {
		color.Green(msg)
	}
}

func printWarning(msg string) {
	if noColor {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
	} else {
		color.Yellow("Warning: %s", msg)
	}
}

// findGitRoot finds the root directory of the git repository
func findGitRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("not in a git repository")
}

// normalizeIssueID normalizes an issue ID to the project-specific format
// Accepts formats like "001", "PROJECT-001", or existing IDs and returns project-specific format
func normalizeIssueID(input string) entities.IssueID {
	input = strings.TrimSpace(input)

	// Get current project name for ID normalization
	ctx := context.Background()
	projectName := getCurrentProjectName(ctx)

	// If it already has a project prefix, return as-is
	if regexp.MustCompile(`^[A-Z]+[A-Z0-9]*-\d{3}$`).MatchString(strings.ToUpper(input)) {
		return entities.IssueID(strings.ToUpper(input))
	}

	// If it's just a number (like "001"), add the project prefix
	if regexp.MustCompile(`^\d+$`).MatchString(input) {
		// Parse as number and format with leading zeros
		if num, err := strconv.Atoi(input); err == nil {
			return entities.IssueID(fmt.Sprintf("%s-%03d", projectName, num))
		}
	}

	// If it's already a 3-digit format (like "001"), add the project prefix
	if regexp.MustCompile(`^\d{3}$`).MatchString(input) {
		return entities.IssueID(fmt.Sprintf("%s-%s", projectName, input))
	}

	// For any other format, return as-is (let validation handle it)
	return entities.IssueID(input)
}

// Color and formatting utilities for enhanced output
func colorStatus(status entities.Status) string {
	if noColor {
		return string(status)
	}

	switch status {
	case entities.StatusOpen:
		return color.CyanString(string(status))
	case entities.StatusInProgress:
		return color.YellowString(string(status))
	case entities.StatusReview:
		return color.MagentaString(string(status))
	case entities.StatusDone:
		return color.BlueString(string(status))
	case entities.StatusClosed:
		return color.RedString(string(status))
	default:
		return string(status)
	}
}

func colorPriority(priority entities.Priority) string {
	if noColor {
		return string(priority)
	}

	switch priority {
	case entities.PriorityCritical:
		return color.HiRedString(string(priority))
	case entities.PriorityHigh:
		return color.RedString(string(priority))
	case entities.PriorityMedium:
		return color.YellowString(string(priority))
	case entities.PriorityLow:
		return color.GreenString(string(priority))
	default:
		return string(priority)
	}
}

func colorType(issueType entities.IssueType) string {
	if noColor {
		return string(issueType)
	}

	switch issueType {
	case entities.IssueTypeBug:
		return color.RedString(string(issueType))
	case entities.IssueTypeFeature:
		return color.GreenString(string(issueType))
	case entities.IssueTypeTask:
		return color.BlueString(string(issueType))
	case entities.IssueTypeEpic:
		return color.MagentaString(string(issueType))
	default:
		return string(issueType)
	}
}

func colorIssueID(id entities.IssueID) string {
	if noColor {
		return string(id)
	}
	return color.HiWhiteString(string(id))
}

func colorHeader(text string) string {
	if noColor {
		return text
	}
	return color.HiCyanString(text)
}

func colorLabel(text string) string {
	if noColor {
		return text
	}
	return color.HiBlackString(text)
}

func colorValue(text string) string {
	if noColor {
		return text
	}
	return color.WhiteString(text)
}

func printSeparator() {
	if noColor {
		fmt.Println("────────────────────────────────────────────────────────────────")
	} else {
		color.HiBlack("────────────────────────────────────────────────────────────────")
	}
}

func printSectionHeader(title string) {
	if noColor {
		fmt.Printf("\n▶ %s\n", title)
	} else {
		fmt.Printf("\n")
		color.HiCyan("▶ %s", title)
	}
}

func formatFieldValue(label, value string) {
	if noColor {
		fmt.Printf("%s: %s\n", label, value)
	} else {
		fmt.Printf("%s: %s\n", colorLabel(label), colorValue(value))
	}
}

// getCurrentProjectName gets the current project name from config or fallback to directory name
func getCurrentProjectName(ctx context.Context) string {
	// Try to load config first
	repoPath, err := findGitRoot()
	if err != nil {
		// Fallback to current directory name if not in git repo
		if wd, err := os.Getwd(); err == nil {
			return strings.ToUpper(filepath.Base(wd))
		}
		return "UNKNOWN"
	}

	configRepo := storage.NewFileConfigRepository(filepath.Join(repoPath, app.ConfigDirName))
	if config, err := configRepo.Load(ctx); err == nil && config.Project.Name != "" {
		return strings.ToUpper(config.Project.Name)
	}

	// Fallback to directory name
	return strings.ToUpper(filepath.Base(repoPath))
}

// registerProjectGlobally registers a project with the global issuemap system
func registerProjectGlobally(ctx context.Context, projectPath, projectName string) error {
	globalService := services.NewGlobalService()
	_, err := globalService.RegisterCurrentProject(ctx)
	return err
}
