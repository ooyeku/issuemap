package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
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
