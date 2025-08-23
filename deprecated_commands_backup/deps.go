package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

var (
	depsGraph    bool
	depsBlocked  bool
	depsValidate bool
	depsStats    bool
	depsImpact   string
)

// depsCmd represents the deps command
var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Visualize and analyze issue dependencies",
	Long: `Visualize dependency relationships and analyze blocking issues.

Examples:
  issuemap deps --graph                    # Show dependency graph
  issuemap deps --blocked                  # List all blocked issues
  issuemap deps --validate                 # Validate dependency graph for issues
  issuemap deps --stats                    # Show dependency statistics
  issuemap deps --impact ISSUE-001        # Analyze impact of changes to issue`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Deprecation warning
		printWarning("DEPRECATED: 'deps' command will be removed in a future version. Use 'depend' with flags instead:")
		printWarning("  issuemap depend --graph      (instead of deps --graph)")
		printWarning("  issuemap depend --stats      (instead of deps --stats)")
		printWarning("  issuemap depend --blocked    (instead of deps --blocked)")
		printWarning("  issuemap depend --validate   (instead of deps --validate)")
		printWarning("  issuemap depend --impact     (instead of deps --impact)")
		fmt.Println()
		if depsGraph {
			return runDepsGraph(cmd)
		}
		if depsBlocked {
			return runDepsBlocked(cmd)
		}
		if depsValidate {
			return runDepsValidate(cmd)
		}
		if depsStats {
			return runDepsStats(cmd)
		}
		if depsImpact != "" {
			return runDepsImpact(cmd, entities.IssueID(depsImpact))
		}

		// Default: show basic dependency overview
		return runDepsOverview(cmd)
	},
}

func init() {
	rootCmd.AddCommand(depsCmd)

	depsCmd.Flags().BoolVarP(&depsGraph, "graph", "g", false, "show dependency graph visualization")
	depsCmd.Flags().BoolVarP(&depsBlocked, "blocked", "b", false, "list blocked issues")
	depsCmd.Flags().BoolVar(&depsValidate, "validate", false, "validate dependency graph")
	depsCmd.Flags().BoolVar(&depsStats, "stats", false, "show dependency statistics")
	depsCmd.Flags().StringVar(&depsImpact, "impact", "", "analyze impact of changes to issue")
}

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
	fmt.Printf("  issuemap deps --graph      Show dependency visualization\n")
	fmt.Printf("  issuemap deps --blocked    List all blocked issues\n")
	fmt.Printf("  issuemap deps --validate   Check for circular dependencies\n")
	fmt.Printf("  issuemap deps --stats      Show detailed statistics\n")
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

// Helper functions

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
