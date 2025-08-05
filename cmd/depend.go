package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	dependType        string
	dependDescription string
	dependRemove      bool
	dependResolve     bool
	dependReactivate  bool
	dependList        bool
)

// dependCmd represents the depend command
var dependCmd = &cobra.Command{
	Use:   "depend <source-issue> <target-issue>",
	Short: "Manage dependencies between issues",
	Long: `Create and manage dependency relationships between issues.

Dependency Types:
  blocks   - Source issue blocks target issue (target cannot start until source is done)
  requires - Source issue requires target issue (source cannot finish until target is done)

Examples:
  issuemap depend ISSUE-001 ISSUE-002 --type blocks     # ISSUE-001 blocks ISSUE-002
  issuemap depend ISSUE-003 ISSUE-004 --type requires   # ISSUE-003 requires ISSUE-004
  issuemap depend ISSUE-001 ISSUE-002 --remove          # Remove dependency
  issuemap depend ISSUE-001 ISSUE-002 --resolve         # Mark dependency as resolved
  issuemap depend --list ISSUE-001                      # List all dependencies for issue`,
	Args: cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if dependList {
			if len(args) != 1 {
				return fmt.Errorf("--list requires exactly one issue ID")
			}
			return runDependList(cmd, entities.IssueID(args[0]))
		}

		if len(args) != 2 {
			return fmt.Errorf("source and target issue IDs are required")
		}

		sourceID := entities.IssueID(args[0])
		targetID := entities.IssueID(args[1])

		if dependRemove {
			return runDependRemove(cmd, sourceID, targetID)
		}

		if dependResolve {
			return runDependResolve(cmd, sourceID, targetID)
		}

		if dependReactivate {
			return runDependReactivate(cmd, sourceID, targetID)
		}

		return runDependCreate(cmd, sourceID, targetID)
	},
}

func init() {
	rootCmd.AddCommand(dependCmd)

	dependCmd.Flags().StringVarP(&dependType, "type", "t", "blocks", "dependency type (blocks, requires)")
	dependCmd.Flags().StringVarP(&dependDescription, "description", "d", "", "description of the dependency")
	dependCmd.Flags().BoolVar(&dependRemove, "remove", false, "remove dependency")
	dependCmd.Flags().BoolVar(&dependResolve, "resolve", false, "resolve dependency")
	dependCmd.Flags().BoolVar(&dependReactivate, "reactivate", false, "reactivate dependency")
	dependCmd.Flags().BoolVarP(&dependList, "list", "l", false, "list dependencies for issue")
}

func runDependCreate(cmd *cobra.Command, sourceID, targetID entities.IssueID) error {
	ctx := context.Background()

	// Validate dependency type
	var depType entities.DependencyType
	switch strings.ToLower(dependType) {
	case "blocks":
		depType = entities.DependencyTypeBlocks
	case "requires":
		depType = entities.DependencyTypeRequires
	default:
		return fmt.Errorf("invalid dependency type: %s (use 'blocks' or 'requires')", dependType)
	}

	// Initialize services
	dependencyService, err := initDependencyService()
	if err != nil {
		return err
	}

	// Get current user
	author := getCurrentUser(nil) // TODO: Pass git client if needed

	// Create dependency
	dependency, err := dependencyService.CreateDependency(ctx, sourceID, targetID, depType, dependDescription, author)
	if err != nil {
		printError(fmt.Errorf("failed to create dependency: %w", err))
		return err
	}

	// Display success message
	printSuccess(fmt.Sprintf("Created dependency: %s", dependency.String()))

	// Show dependency details
	fmt.Printf("\nDependency Details:\n")
	fmt.Printf("ID: %s\n", dependency.ID)
	fmt.Printf("Type: %s\n", dependency.Type)
	fmt.Printf("Status: %s\n", dependency.Status)
	if dependency.Description != "" {
		fmt.Printf("Description: %s\n", dependency.Description)
	}
	fmt.Printf("Created by: %s\n", dependency.CreatedBy)
	fmt.Printf("Created at: %s\n", dependency.CreatedAt.Format("2006-01-02 15:04:05"))

	return nil
}

func runDependRemove(cmd *cobra.Command, sourceID, targetID entities.IssueID) error {
	ctx := context.Background()

	dependencyService, err := initDependencyService()
	if err != nil {
		return err
	}

	// Find the dependency to remove
	dependencies, err := dependencyService.GetIssueDependencies(ctx, sourceID)
	if err != nil {
		printError(fmt.Errorf("failed to get dependencies: %w", err))
		return err
	}

	var targetDep *entities.Dependency
	for _, dep := range dependencies {
		if (dep.SourceID == sourceID && dep.TargetID == targetID) ||
			(dep.SourceID == targetID && dep.TargetID == sourceID) {
			targetDep = dep
			break
		}
	}

	if targetDep == nil {
		printError(fmt.Errorf("no dependency found between %s and %s", sourceID, targetID))
		return fmt.Errorf("dependency not found")
	}

	author := getCurrentUser(nil)
	if err := dependencyService.RemoveDependency(ctx, targetDep.ID, author); err != nil {
		printError(fmt.Errorf("failed to remove dependency: %w", err))
		return err
	}

	printSuccess(fmt.Sprintf("Removed dependency: %s", targetDep.String()))
	return nil
}

func runDependResolve(cmd *cobra.Command, sourceID, targetID entities.IssueID) error {
	ctx := context.Background()

	dependencyService, err := initDependencyService()
	if err != nil {
		return err
	}

	// Find the dependency to resolve
	dependencies, err := dependencyService.GetIssueDependencies(ctx, sourceID)
	if err != nil {
		printError(fmt.Errorf("failed to get dependencies: %w", err))
		return err
	}

	var targetDep *entities.Dependency
	for _, dep := range dependencies {
		if (dep.SourceID == sourceID && dep.TargetID == targetID) ||
			(dep.SourceID == targetID && dep.TargetID == sourceID) {
			targetDep = dep
			break
		}
	}

	if targetDep == nil {
		printError(fmt.Errorf("no dependency found between %s and %s", sourceID, targetID))
		return fmt.Errorf("dependency not found")
	}

	author := getCurrentUser(nil)
	if err := dependencyService.ResolveDependency(ctx, targetDep.ID, author); err != nil {
		printError(fmt.Errorf("failed to resolve dependency: %w", err))
		return err
	}

	printSuccess(fmt.Sprintf("Resolved dependency: %s", targetDep.String()))
	return nil
}

func runDependReactivate(cmd *cobra.Command, sourceID, targetID entities.IssueID) error {
	ctx := context.Background()

	dependencyService, err := initDependencyService()
	if err != nil {
		return err
	}

	// Find the dependency to reactivate
	dependencies, err := dependencyService.GetIssueDependencies(ctx, sourceID)
	if err != nil {
		printError(fmt.Errorf("failed to get dependencies: %w", err))
		return err
	}

	var targetDep *entities.Dependency
	for _, dep := range dependencies {
		if (dep.SourceID == sourceID && dep.TargetID == targetID) ||
			(dep.SourceID == targetID && dep.TargetID == sourceID) {
			targetDep = dep
			break
		}
	}

	if targetDep == nil {
		printError(fmt.Errorf("no dependency found between %s and %s", sourceID, targetID))
		return fmt.Errorf("dependency not found")
	}

	author := getCurrentUser(nil)
	if err := dependencyService.ReactivateDependency(ctx, targetDep.ID, author); err != nil {
		printError(fmt.Errorf("failed to reactivate dependency: %w", err))
		return err
	}

	printSuccess(fmt.Sprintf("Reactivated dependency: %s", targetDep.String()))
	return nil
}

func runDependList(cmd *cobra.Command, issueID entities.IssueID) error {
	ctx := context.Background()

	dependencyService, err := initDependencyService()
	if err != nil {
		return err
	}

	// Get all dependencies for the issue
	dependencies, err := dependencyService.GetIssueDependencies(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("failed to get dependencies: %w", err))
		return err
	}

	if len(dependencies) == 0 {
		fmt.Printf("No dependencies found for %s\n", issueID)
		return nil
	}

	// Get blocking info
	blockingInfo, err := dependencyService.GetBlockingInfo(ctx, issueID)
	if err != nil {
		printWarning(fmt.Sprintf("Could not get blocking info: %v", err))
	}

	fmt.Printf("Dependencies for %s\n", issueID)
	fmt.Printf("===================\n\n")

	if blockingInfo != nil {
		if blockingInfo.IsBlocked {
			printWarning(fmt.Sprintf("⚠️  Issue is BLOCKED by %d issues", len(blockingInfo.BlockedBy)))
		} else {
			printSuccess("✓ Issue is not blocked")
		}

		if blockingInfo.BlockingCount > 0 {
			fmt.Printf("🔒 Blocking %d issues\n", blockingInfo.BlockingCount)
		}

		if blockingInfo.CriticalPath {
			printWarning("⚡ Issue is on the critical path")
		}
		fmt.Printf("\n")
	}

	// Group dependencies by type and status
	activeDeps := make(map[entities.DependencyType][]*entities.Dependency)
	resolvedDeps := make(map[entities.DependencyType][]*entities.Dependency)

	for _, dep := range dependencies {
		if dep.IsActive() {
			activeDeps[dep.Type] = append(activeDeps[dep.Type], dep)
		} else {
			resolvedDeps[dep.Type] = append(resolvedDeps[dep.Type], dep)
		}
	}

	// Display active dependencies
	if len(activeDeps) > 0 {
		fmt.Printf("Active Dependencies:\n")
		fmt.Printf("-------------------\n")
		
		for depType, deps := range activeDeps {
			fmt.Printf("\n%s:\n", strings.Title(string(depType)))
			for _, dep := range deps {
				var other entities.IssueID
				var direction string
				
				if dep.SourceID == issueID {
					other = dep.TargetID
					if dep.Type == entities.DependencyTypeBlocks {
						direction = "blocks"
					} else {
						direction = "requires"
					}
				} else {
					other = dep.SourceID
					if dep.Type == entities.DependencyTypeBlocks {
						direction = "blocked by"
					} else {
						direction = "required by"
					}
				}
				
				fmt.Printf("  • %s %s %s", issueID, direction, other)
				if dep.Description != "" {
					fmt.Printf(" - %s", dep.Description)
				}
				fmt.Printf("\n")
			}
		}
	}

	// Display resolved dependencies
	if len(resolvedDeps) > 0 {
		fmt.Printf("\nResolved Dependencies:\n")
		fmt.Printf("---------------------\n")
		
		for depType, deps := range resolvedDeps {
			fmt.Printf("\n%s:\n", strings.Title(string(depType)))
			for _, dep := range deps {
				var other entities.IssueID
				if dep.SourceID == issueID {
					other = dep.TargetID
				} else {
					other = dep.SourceID
				}
				
				fmt.Printf("  • %s ↔ %s", issueID, other)
				if dep.Description != "" {
					fmt.Printf(" - %s", dep.Description)
				}
				if dep.ResolvedAt != nil {
					fmt.Printf(" (resolved %s)", dep.ResolvedAt.Format("2006-01-02"))
				}
				fmt.Printf("\n")
			}
		}
	}

	return nil
}

func initDependencyService() (*services.DependencyService, error) {
	// Initialize repositories
	repoPath, err := findGitRoot()
	if err != nil {
		return nil, fmt.Errorf("not in a git repository: %w", err)
	}

	issuemapPath := filepath.Join(repoPath, app.ConfigDirName)
	issueRepo := storage.NewFileIssueRepository(issuemapPath)
	configRepo := storage.NewFileConfigRepository(issuemapPath)
	dependencyRepo := storage.NewFileDependencyRepository(issuemapPath)
	historyRepo := storage.NewFileHistoryRepository(issuemapPath)

	var gitRepo *git.GitClient
	if gitClient, err := git.NewGitClient(repoPath); err == nil {
		gitRepo = gitClient
	}

	// Initialize services
	issueService := services.NewIssueService(issueRepo, configRepo, gitRepo)
	historyService := services.NewHistoryService(historyRepo, gitRepo)
	dependencyService := services.NewDependencyService(dependencyRepo, issueService, historyService)

	return dependencyService, nil
}