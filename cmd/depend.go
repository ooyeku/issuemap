package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
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
	// New flags from deps command
	dependGraph    bool
	dependBlocked  bool
	dependValidate bool
	dependStats    bool
	dependImpact   string
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
		// Handle visualization and analysis flags from deps
		if dependGraph {
			return runDepsGraph(cmd)
		}
		if dependBlocked {
			return runDepsBlocked(cmd)
		}
		if dependValidate {
			return runDepsValidate(cmd)
		}
		if dependStats {
			return runDepsStats(cmd)
		}
		if dependImpact != "" {
			return runDepsImpact(cmd, entities.IssueID(dependImpact))
		}

		// Original depend command logic
		if dependList {
			if len(args) != 1 {
				return fmt.Errorf("--list requires exactly one issue ID")
			}
			return runDependList(cmd, entities.IssueID(args[0]))
		}

		// If no args and no special flags, show overview
		if len(args) == 0 {
			return runDepsOverview(cmd)
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

	// Add flags from deps command
	dependCmd.Flags().BoolVarP(&dependGraph, "graph", "g", false, "show dependency graph visualization")
	dependCmd.Flags().BoolVarP(&dependBlocked, "blocked", "b", false, "list blocked issues")
	dependCmd.Flags().BoolVar(&dependValidate, "validate", false, "validate dependency graph")
	dependCmd.Flags().BoolVar(&dependStats, "stats", false, "show dependency statistics")
	dependCmd.Flags().StringVar(&dependImpact, "impact", "", "analyze impact of changes to issue")
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
			printWarning(fmt.Sprintf("Issue is BLOCKED by %d issues", len(blockingInfo.BlockedBy)))
		} else {
			printSuccess("Issue is not blocked")
		}

		if blockingInfo.BlockingCount > 0 {
			fmt.Printf("Blocking %d issues\n", blockingInfo.BlockingCount)
		}

		if blockingInfo.CriticalPath {
			printWarning("Issue is on the critical path")
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

// Functions from deps command integrated into depend

func runDepsGraph(cmd *cobra.Command) error {
	ctx := context.Background()

	dependencyService, err := initDependencyService()
	if err != nil {
		return err
	}

	graph, err := dependencyService.GetDependencyGraph(ctx)
	if err != nil {
		printError(fmt.Errorf("failed to get dependency graph: %w", err))
		return err
	}

	if len(graph.Dependencies) == 0 {
		fmt.Println("No dependencies found.")
		return nil
	}

	fmt.Printf("Dependency Graph\n")
	fmt.Printf("================\n\n")

	// Build a simplified representation of the graph
	nodes := make(map[entities.IssueID]bool)
	for _, dep := range graph.Dependencies {
		if dep.IsActive() {
			nodes[dep.SourceID] = true
			nodes[dep.TargetID] = true
		}
	}

	// Convert to sorted slice for consistent output
	var sortedNodes []entities.IssueID
	for node := range nodes {
		sortedNodes = append(sortedNodes, node)
	}
	sort.Slice(sortedNodes, func(i, j int) bool {
		return string(sortedNodes[i]) < string(sortedNodes[j])
	})

	fmt.Printf("Issues involved in dependencies: %d\n", len(sortedNodes))
	fmt.Printf("Active dependencies: %d\n\n", countActiveDependencies(graph))

	// Show each issue and its relationships
	for _, issueID := range sortedNodes {
		blocking := graph.GetBlockedIssues(issueID)
		blockedBy := graph.GetBlockingIssues(issueID)

		fmt.Printf("%s\n", issueID)

		if len(blockedBy) > 0 {
			fmt.Printf("  ↑ Blocked by: %s\n", strings.Join(issueIDsToStrings(blockedBy), ", "))
		}

		if len(blocking) > 0 {
			fmt.Printf("  ↓ Blocking: %s\n", strings.Join(issueIDsToStrings(blocking), ", "))
		}

		if len(blockedBy) == 0 && len(blocking) == 0 {
			fmt.Printf("  (no active blocking relationships)\n")
		}

		fmt.Printf("\n")
	}

	// Show simple ASCII art representation for small graphs
	if len(sortedNodes) <= 10 {
		fmt.Printf("Simple Graph Visualization:\n")
		fmt.Printf("---------------------------\n")
		showSimpleGraph(graph, sortedNodes)
	}

	return nil
}

func runDepsBlocked(cmd *cobra.Command) error {
	ctx := context.Background()

	dependencyService, err := initDependencyService()
	if err != nil {
		return err
	}

	blockedIssues, err := dependencyService.GetBlockedIssues(ctx)
	if err != nil {
		printError(fmt.Errorf("failed to get blocked issues: %w", err))
		return err
	}

	if len(blockedIssues) == 0 {
		printSuccess("No issues are currently blocked")
		return nil
	}

	fmt.Printf("Blocked Issues (%d)\n", len(blockedIssues))
	fmt.Printf("==================\n\n")

	for _, issueID := range blockedIssues {
		blockingInfo, err := dependencyService.GetBlockingInfo(ctx, issueID)
		if err != nil {
			fmt.Printf("%s - Error getting blocking info: %v\n", issueID, err)
			continue
		}

		fmt.Printf("%s\n", issueID)
		if len(blockingInfo.BlockedBy) > 0 {
			fmt.Printf("  Blocked by: %s\n", strings.Join(issueIDsToStrings(blockingInfo.BlockedBy), ", "))
		}

		if blockingInfo.CriticalPath {
			fmt.Printf("  Critical Path\n")
		}

		if blockingInfo.BlockingCount > 0 {
			fmt.Printf("  Also blocking %d other issues\n", blockingInfo.BlockingCount)
		}

		fmt.Printf("\n")
	}

	return nil
}

func runDepsValidate(cmd *cobra.Command) error {
	ctx := context.Background()

	dependencyService, err := initDependencyService()
	if err != nil {
		return err
	}

	result, err := dependencyService.ValidateDependencyGraph(ctx)
	if err != nil {
		printError(fmt.Errorf("failed to validate dependency graph: %w", err))
		return err
	}

	fmt.Printf("Dependency Graph Validation\n")
	fmt.Printf("===========================\n\n")

	if result.IsValid {
		printSuccess("Dependency graph is valid")
	} else {
		printError(fmt.Errorf("Dependency graph has issues"))
	}

	if len(result.CircularPaths) > 0 {
		fmt.Printf("\nCircular Dependencies Found (%d):\n", len(result.CircularPaths))
		fmt.Printf("----------------------------------\n")
		for i, cycle := range result.CircularPaths {
			fmt.Printf("%d. %s → %s\n", i+1,
				strings.Join(issueIDsToStrings(cycle), " → "),
				cycle[0]) // Show the cycle back to start
		}
	}

	if len(result.ConflictingDeps) > 0 {
		fmt.Printf("\nConflicting Dependencies (%d):\n", len(result.ConflictingDeps))
		fmt.Printf("-------------------------------\n")
		for i, dep := range result.ConflictingDeps {
			fmt.Printf("%d. %s\n", i+1, dep.String())
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("\nWarnings:\n")
		fmt.Printf("---------\n")
		for _, warning := range result.Warnings {
			printWarning(warning)
		}
	}

	return nil
}

func runDepsStats(cmd *cobra.Command) error {
	ctx := context.Background()

	dependencyService, err := initDependencyService()
	if err != nil {
		return err
	}

	stats, err := dependencyService.GetDependencyStats(ctx, repositories.DependencyFilter{})
	if err != nil {
		printError(fmt.Errorf("failed to get dependency statistics: %w", err))
		return err
	}

	fmt.Printf("Dependency Statistics\n")
	fmt.Printf("=====================\n\n")

	fmt.Printf("Overview:\n")
	fmt.Printf("  Total Dependencies: %d\n", stats.TotalDependencies)
	fmt.Printf("  Active Dependencies: %d\n", stats.ActiveDependencies)
	fmt.Printf("  Resolved Dependencies: %d\n", stats.ResolvedDependencies)
	fmt.Printf("  Issues with Dependencies: %d\n", stats.IssuesWithDeps)
	fmt.Printf("  Average Dependencies per Issue: %.1f\n", stats.AverageDepPerIssue)

	if stats.CircularDependencies > 0 {
		printWarning(fmt.Sprintf("  Circular Dependencies: %d", stats.CircularDependencies))
	}

	fmt.Printf("\nBy Type:\n")
	for depType, count := range stats.DependenciesByType {
		fmt.Printf("  %s: %d\n", capitalizeFirst(string(depType)), count)
	}

	fmt.Printf("\nBy Status:\n")
	for status, count := range stats.DependenciesByStatus {
		fmt.Printf("  %s: %d\n", capitalizeFirst(string(status)), count)
	}

	if len(stats.MostBlockedIssues) > 0 {
		fmt.Printf("\nMost Blocked Issues:\n")
		for i, issueID := range stats.MostBlockedIssues {
			if i >= 5 { // Limit to top 5
				break
			}
			fmt.Printf("  %d. %s\n", i+1, issueID)
		}
	}

	if len(stats.MostBlockingIssues) > 0 {
		fmt.Printf("\nMost Blocking Issues:\n")
		for i, issueID := range stats.MostBlockingIssues {
			if i >= 5 { // Limit to top 5
				break
			}
			fmt.Printf("  %d. %s\n", i+1, issueID)
		}
	}

	if len(stats.DependencyCreators) > 0 {
		fmt.Printf("\nDependency Creators:\n")
		for creator, count := range stats.DependencyCreators {
			fmt.Printf("  %s: %d\n", creator, count)
		}
	}

	return nil
}

func runDepsImpact(cmd *cobra.Command, issueID entities.IssueID) error {
	ctx := context.Background()

	dependencyService, err := initDependencyService()
	if err != nil {
		return err
	}

	analysis, err := dependencyService.AnalyzeDependencyImpact(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("failed to analyze dependency impact: %w", err))
		return err
	}

	fmt.Printf("Dependency Impact Analysis for %s\n", issueID)
	fmt.Printf("===================================\n\n")

	fmt.Printf("Risk Level: %s\n", strings.ToUpper(analysis.RiskLevel))
	fmt.Printf("Affected Issues: %d\n", len(analysis.AffectedIssues))

	if len(analysis.AffectedIssues) > 0 {
		fmt.Printf("\nAffected Issues:\n")
		for _, affected := range analysis.AffectedIssues {
			fmt.Printf("  • %s", affected)
			if chain, exists := analysis.BlockingChain[affected]; exists && len(chain) > 1 {
				fmt.Printf(" (via: %s)", strings.Join(issueIDsToStrings(chain[1:len(chain)-1]), " → "))
			}
			fmt.Printf("\n")
		}
	}

	if len(analysis.CriticalPath) > 0 {
		fmt.Printf("\nCritical Path:\n")
		fmt.Printf("  %s\n", strings.Join(issueIDsToStrings(analysis.CriticalPath), " → "))
	}

	if analysis.DelayEstimate != nil {
		fmt.Printf("\nEstimated Delay Impact: %v\n", *analysis.DelayEstimate)
	}

	if len(analysis.Recommendations) > 0 {
		fmt.Printf("\nRecommendations:\n")
		for _, rec := range analysis.Recommendations {
			fmt.Printf("  • %s\n", rec)
		}
	}

	return nil
}

func runDepsOverview(cmd *cobra.Command) error {
	ctx := context.Background()

	dependencyService, err := initDependencyService()
	if err != nil {
		return err
	}

	stats, err := dependencyService.GetDependencyStats(ctx, repositories.DependencyFilter{})
	if err != nil {
		printError(fmt.Errorf("failed to get dependency statistics: %w", err))
		return err
	}

	blockedIssues, err := dependencyService.GetBlockedIssues(ctx)
	if err != nil {
		printError(fmt.Errorf("failed to get blocked issues: %w", err))
		return err
	}

	fmt.Printf("Dependencies Overview\n")
	fmt.Printf("====================\n\n")

	fmt.Printf("Statistics:\n")
	fmt.Printf("  Total Dependencies: %d\n", stats.TotalDependencies)
	fmt.Printf("  Active Dependencies: %d\n", stats.ActiveDependencies)
	fmt.Printf("  Issues with Dependencies: %d\n", stats.IssuesWithDeps)

	fmt.Printf("\nBlocking Status:\n")
	if len(blockedIssues) > 0 {
		printWarning(fmt.Sprintf("  %d issues are currently blocked", len(blockedIssues)))
		if len(blockedIssues) <= 5 {
			for _, issue := range blockedIssues {
				fmt.Printf("    • %s\n", issue)
			}
		} else {
			for i := 0; i < 3; i++ {
				fmt.Printf("    • %s\n", blockedIssues[i])
			}
			fmt.Printf("    ... and %d more\n", len(blockedIssues)-3)
		}
	} else {
		printSuccess("  No issues are currently blocked")
	}

	if stats.CircularDependencies > 0 {
		fmt.Printf("\nIssues:\n")
		printWarning(fmt.Sprintf("  %d circular dependencies detected", stats.CircularDependencies))
	}

	fmt.Printf("\nAvailable Commands:\n")
	fmt.Printf("  issuemap depend --graph      Show dependency visualization\n")
	fmt.Printf("  issuemap depend --blocked    List all blocked issues\n")
	fmt.Printf("  issuemap depend --validate   Check for circular dependencies\n")
	fmt.Printf("  issuemap depend --stats      Show detailed statistics\n")
	fmt.Printf("  issuemap depend --list <issue>  Show dependencies for specific issue\n")

	return nil
}

// Helper functions

func countActiveDependencies(graph *entities.DependencyGraph) int {
	count := 0
	for _, dep := range graph.Dependencies {
		if dep.IsActive() {
			count++
		}
	}
	return count
}

func issueIDsToStrings(issues []entities.IssueID) []string {
	var result []string
	for _, issue := range issues {
		result = append(result, string(issue))
	}
	return result
}

func showSimpleGraph(graph *entities.DependencyGraph, nodes []entities.IssueID) {
	fmt.Printf("\n")

	// Simple ASCII visualization for small graphs
	for _, node := range nodes {
		blocking := graph.GetBlockedIssues(node)

		if len(blocking) > 0 {
			fmt.Printf("%s\n", node)
			for i, blocked := range blocking {
				if i == len(blocking)-1 {
					fmt.Printf("  └─ %s\n", blocked)
				} else {
					fmt.Printf("  ├─ %s\n", blocked)
				}
			}
		} else {
			// Check if it's a leaf node (not blocking anything)
			blockedBy := graph.GetBlockingIssues(node)
			if len(blockedBy) > 0 {
				fmt.Printf("%s (leaf)\n", node)
			}
		}
	}
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
