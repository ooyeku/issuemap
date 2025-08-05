package services

import (
	"context"
	"fmt"
	"sort"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// DependencyService provides high-level operations for dependency management
type DependencyService struct {
	dependencyRepo repositories.DependencyRepository
	issueService   *IssueService
	historyService *HistoryService
}

// NewDependencyService creates a new dependency service
func NewDependencyService(
	dependencyRepo repositories.DependencyRepository,
	issueService *IssueService,
	historyService *HistoryService,
) *DependencyService {
	return &DependencyService{
		dependencyRepo: dependencyRepo,
		issueService:   issueService,
		historyService: historyService,
	}
}

// CreateDependency creates a new dependency relationship with validation
func (s *DependencyService) CreateDependency(ctx context.Context, sourceID, targetID entities.IssueID, depType entities.DependencyType, description, createdBy string) (*entities.Dependency, error) {
	// Validate that both issues exist
	if _, err := s.issueService.GetIssue(ctx, sourceID); err != nil {
		return nil, fmt.Errorf("source issue not found: %w", err)
	}
	if _, err := s.issueService.GetIssue(ctx, targetID); err != nil {
		return nil, fmt.Errorf("target issue not found: %w", err)
	}

	// Check for circular dependencies
	graph, err := s.dependencyRepo.GetDependencyGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency graph: %w", err)
	}

	// For blocks: source blocks target, so adding this would mean source -> target
	// For requires: source requires target, so adding this would mean target -> source in blocking terms
	var checkFrom, checkTo entities.IssueID
	if depType == entities.DependencyTypeBlocks {
		checkFrom, checkTo = sourceID, targetID
	} else {
		checkFrom, checkTo = targetID, sourceID
	}

	if graph.HasCircularDependency(checkFrom, checkTo) {
		return nil, fmt.Errorf("creating this dependency would create a circular dependency")
	}

	// Create the dependency
	dependency := entities.NewDependency(sourceID, targetID, depType, description, createdBy)

	if err := s.dependencyRepo.Create(ctx, dependency); err != nil {
		return nil, fmt.Errorf("failed to create dependency: %w", err)
	}

	// Record in history
	if s.historyService != nil {
		message := fmt.Sprintf("Added dependency: %s", dependency.String())
		s.historyService.RecordDependencyCreated(ctx, sourceID, targetID, depType, createdBy, message)
		
		// Also record on the target issue
		if sourceID != targetID {
			s.historyService.RecordDependencyCreated(ctx, targetID, sourceID, depType, createdBy, message)
		}
	}

	return dependency, nil
}

// RemoveDependency removes a dependency relationship
func (s *DependencyService) RemoveDependency(ctx context.Context, dependencyID, removedBy string) error {
	dependency, err := s.dependencyRepo.GetByID(ctx, dependencyID)
	if err != nil {
		return fmt.Errorf("dependency not found: %w", err)
	}

	if err := s.dependencyRepo.Delete(ctx, dependencyID); err != nil {
		return fmt.Errorf("failed to delete dependency: %w", err)
	}

	// Record in history
	if s.historyService != nil {
		message := fmt.Sprintf("Removed dependency: %s", dependency.String())
		s.historyService.RecordDependencyRemoved(ctx, dependency.SourceID, dependency.TargetID, dependency.Type, removedBy, message)
		
		// Also record on the target issue
		if dependency.SourceID != dependency.TargetID {
			s.historyService.RecordDependencyRemoved(ctx, dependency.TargetID, dependency.SourceID, dependency.Type, removedBy, message)
		}
	}

	return nil
}

// ResolveDependency marks a dependency as resolved
func (s *DependencyService) ResolveDependency(ctx context.Context, dependencyID, resolvedBy string) error {
	dependency, err := s.dependencyRepo.GetByID(ctx, dependencyID)
	if err != nil {
		return fmt.Errorf("dependency not found: %w", err)
	}

	if dependency.IsResolved() {
		return fmt.Errorf("dependency is already resolved")
	}

	dependency.Resolve(resolvedBy)

	if err := s.dependencyRepo.Update(ctx, dependency); err != nil {
		return fmt.Errorf("failed to update dependency: %w", err)
	}

	// Record in history
	if s.historyService != nil {
		message := fmt.Sprintf("Resolved dependency: %s", dependency.String())
		s.historyService.RecordDependencyResolved(ctx, dependency.SourceID, dependency.TargetID, dependency.Type, resolvedBy, message)
	}

	return nil
}

// ReactivateDependency reactivates a resolved dependency
func (s *DependencyService) ReactivateDependency(ctx context.Context, dependencyID, reactivatedBy string) error {
	dependency, err := s.dependencyRepo.GetByID(ctx, dependencyID)
	if err != nil {
		return fmt.Errorf("dependency not found: %w", err)
	}

	if dependency.IsActive() {
		return fmt.Errorf("dependency is already active")
	}

	dependency.Reactivate()

	if err := s.dependencyRepo.Update(ctx, dependency); err != nil {
		return fmt.Errorf("failed to update dependency: %w", err)
	}

	// Record in history
	if s.historyService != nil {
		message := fmt.Sprintf("Reactivated dependency: %s", dependency.String())
		s.historyService.RecordDependencyReactivated(ctx, dependency.SourceID, dependency.TargetID, dependency.Type, reactivatedBy, message)
	}

	return nil
}

// GetDependencies retrieves dependencies with filtering
func (s *DependencyService) GetDependencies(ctx context.Context, filter repositories.DependencyFilter) ([]*entities.Dependency, error) {
	return s.dependencyRepo.List(ctx, filter)
}

// GetIssueDependencies retrieves all dependencies for a specific issue
func (s *DependencyService) GetIssueDependencies(ctx context.Context, issueID entities.IssueID) ([]*entities.Dependency, error) {
	return s.dependencyRepo.GetByIssueID(ctx, issueID)
}

// GetBlockingInfo gets comprehensive blocking information for an issue
func (s *DependencyService) GetBlockingInfo(ctx context.Context, issueID entities.IssueID) (*entities.BlockingInfo, error) {
	graph, err := s.dependencyRepo.GetDependencyGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency graph: %w", err)
	}

	blockingIssues := graph.GetBlockingIssues(issueID)
	blockedIssues := graph.GetBlockedIssues(issueID)
	isBlocked := len(blockingIssues) > 0

	// Get unresolved dependencies
	var unresolvedDeps []*entities.Dependency
	issueDeps := graph.GetDependenciesFromSource(issueID)
	issueDeps = append(issueDeps, graph.GetDependenciesFromTarget(issueID)...)
	
	for _, dep := range issueDeps {
		if dep.IsActive() {
			unresolvedDeps = append(unresolvedDeps, dep)
		}
	}

	// Determine if on critical path (simplified - issues with most blocking dependencies)
	criticalPath := len(blockedIssues) > 2 // Simple heuristic

	return &entities.BlockingInfo{
		IssueID:        issueID,
		IsBlocked:      isBlocked,
		BlockedBy:      blockingIssues,
		Blocking:       blockedIssues,
		UnresolvedDeps: unresolvedDeps,
		BlockingCount:  len(blockedIssues),
		CriticalPath:   criticalPath,
	}, nil
}

// GetDependencyGraph returns the complete dependency graph
func (s *DependencyService) GetDependencyGraph(ctx context.Context) (*entities.DependencyGraph, error) {
	return s.dependencyRepo.GetDependencyGraph(ctx)
}

// ValidateDependencyGraph validates the entire dependency graph for issues
func (s *DependencyService) ValidateDependencyGraph(ctx context.Context) (*entities.DependencyValidationResult, error) {
	graph, err := s.dependencyRepo.GetDependencyGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency graph: %w", err)
	}

	result := &entities.DependencyValidationResult{
		IsValid: true,
	}

	// Find circular dependencies
	cycles := graph.FindCircularDependencies()
	if len(cycles) > 0 {
		result.IsValid = false
		result.CircularPaths = cycles
	}

	// Find conflicting dependencies
	conflicts, err := s.dependencyRepo.FindConflicts(ctx)
	if err == nil && len(conflicts) > 0 {
		result.ConflictingDeps = conflicts
		if result.IsValid {
			result.IsValid = false
		}
	}

	// Generate warnings
	var warnings []string
	if len(cycles) > 0 {
		warnings = append(warnings, fmt.Sprintf("Found %d circular dependency paths", len(cycles)))
	}
	if len(conflicts) > 0 {
		warnings = append(warnings, fmt.Sprintf("Found %d conflicting dependencies", len(conflicts)))
	}

	result.Warnings = warnings

	return result, nil
}

// GetDependencyStats returns dependency statistics
func (s *DependencyService) GetDependencyStats(ctx context.Context, filter repositories.DependencyFilter) (*repositories.DependencyStats, error) {
	return s.dependencyRepo.GetStats(ctx, filter)
}

// AnalyzeDependencyImpact analyzes the impact of changes to an issue's dependencies
func (s *DependencyService) AnalyzeDependencyImpact(ctx context.Context, issueID entities.IssueID) (*entities.DependencyImpactAnalysis, error) {
	graph, err := s.dependencyRepo.GetDependencyGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency graph: %w", err)
	}

	analysis := &entities.DependencyImpactAnalysis{
		BlockingChain: make(map[entities.IssueID][]entities.IssueID),
	}

	// Find all issues that would be affected by changes to this issue
	affectedIssues := s.findTransitivelyBlockedIssues(graph, issueID, make(map[entities.IssueID]bool))
	analysis.AffectedIssues = affectedIssues

	// Build blocking chains
	for _, affected := range affectedIssues {
		chain := s.getBlockingChain(graph, issueID, affected)
		if len(chain) > 0 {
			analysis.BlockingChain[affected] = chain
		}
	}

	// Calculate risk level based on number of affected issues
	affectedCount := len(affectedIssues)
	switch {
	case affectedCount == 0:
		analysis.RiskLevel = "low"
	case affectedCount <= 2:
		analysis.RiskLevel = "medium" 
	case affectedCount <= 5:
		analysis.RiskLevel = "high"
	default:
		analysis.RiskLevel = "critical"
	}

	// Generate recommendations
	analysis.Recommendations = s.generateRecommendations(graph, issueID, affectedCount)

	// Find critical path (longest dependency chain)
	analysis.CriticalPath = s.findCriticalPath(graph, issueID)

	return analysis, nil
}

// GetBlockedIssues returns all issues that are currently blocked
func (s *DependencyService) GetBlockedIssues(ctx context.Context) ([]entities.IssueID, error) {
	graph, err := s.dependencyRepo.GetDependencyGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency graph: %w", err)
	}

	var blockedIssues []entities.IssueID
	checkedIssues := make(map[entities.IssueID]bool)

	// Check all issues involved in dependencies
	for _, dep := range graph.Dependencies {
		if !dep.IsActive() {
			continue
		}
		
		// Check both source and target
		for _, issueID := range []entities.IssueID{dep.SourceID, dep.TargetID} {
			if checkedIssues[issueID] {
				continue
			}
			checkedIssues[issueID] = true
			
			if graph.IsBlocked(issueID) {
				blockedIssues = append(blockedIssues, issueID)
			}
		}
	}

	// Sort for consistent output
	sort.Slice(blockedIssues, func(i, j int) bool {
		return string(blockedIssues[i]) < string(blockedIssues[j])
	})

	return blockedIssues, nil
}

// AutoResolveDependencies automatically resolves dependencies when target issues are completed
func (s *DependencyService) AutoResolveDependencies(ctx context.Context, completedIssueID entities.IssueID, resolvedBy string) error {
	// Get all dependencies where this issue is the target
	targetDeps, err := s.dependencyRepo.GetByTargetID(ctx, completedIssueID)
	if err != nil {
		return fmt.Errorf("failed to get target dependencies: %w", err)
	}

	var toResolve []*entities.Dependency
	for _, dep := range targetDeps {
		if dep.IsActive() {
			// For blocks: if target is completed, dependency is resolved
			// For requires: if target is completed, dependency is resolved
			toResolve = append(toResolve, dep)
		}
	}

	// Also check source dependencies of type "requires"
	sourceDeps, err := s.dependencyRepo.GetBySourceID(ctx, completedIssueID)
	if err != nil {
		return fmt.Errorf("failed to get source dependencies: %w", err)
	}

	for _, dep := range sourceDeps {
		if dep.IsActive() && dep.Type == entities.DependencyTypeRequires {
			// If the requiring issue is completed, the requirement is satisfied
			toResolve = append(toResolve, dep)
		}
	}

	// Resolve all applicable dependencies
	for _, dep := range toResolve {
		dep.Resolve(resolvedBy)
		if err := s.dependencyRepo.Update(ctx, dep); err != nil {
			return fmt.Errorf("failed to auto-resolve dependency %s: %w", dep.ID, err)
		}

		// Record in history
		if s.historyService != nil {
			message := fmt.Sprintf("Auto-resolved dependency due to issue completion: %s", dep.String())
			s.historyService.RecordDependencyResolved(ctx, dep.SourceID, dep.TargetID, dep.Type, resolvedBy, message)
		}
	}

	return nil
}

// Helper methods

func (s *DependencyService) findTransitivelyBlockedIssues(graph *entities.DependencyGraph, issueID entities.IssueID, visited map[entities.IssueID]bool) []entities.IssueID {
	if visited[issueID] {
		return []entities.IssueID{}
	}
	visited[issueID] = true

	var result []entities.IssueID
	blocked := graph.GetBlockedIssues(issueID)

	for _, blockedIssue := range blocked {
		result = append(result, blockedIssue)
		// Recursively find transitively blocked issues
		transitiveBlocked := s.findTransitivelyBlockedIssues(graph, blockedIssue, visited)
		result = append(result, transitiveBlocked...)
	}

	return result
}

func (s *DependencyService) getBlockingChain(graph *entities.DependencyGraph, from, to entities.IssueID) []entities.IssueID {
	// Simple path finding - could be improved with proper graph algorithms
	visited := make(map[entities.IssueID]bool)
	path := []entities.IssueID{}
	
	if s.dfsPath(graph, from, to, visited, &path) {
		return path
	}
	return []entities.IssueID{}
}

func (s *DependencyService) dfsPath(graph *entities.DependencyGraph, current, target entities.IssueID, visited map[entities.IssueID]bool, path *[]entities.IssueID) bool {
	if current == target {
		*path = append(*path, current)
		return true
	}
	
	if visited[current] {
		return false
	}
	visited[current] = true
	
	*path = append(*path, current)
	
	blocked := graph.GetBlockedIssues(current)
	for _, next := range blocked {
		if s.dfsPath(graph, next, target, visited, path) {
			return true
		}
	}
	
	// Backtrack
	*path = (*path)[:len(*path)-1]
	return false
}

func (s *DependencyService) generateRecommendations(graph *entities.DependencyGraph, issueID entities.IssueID, affectedCount int) []string {
	var recommendations []string
	
	if affectedCount > 5 {
		recommendations = append(recommendations, "Consider breaking down this issue into smaller, independent tasks")
		recommendations = append(recommendations, "Review if all dependencies are truly necessary")
	}
	
	if affectedCount > 0 {
		recommendations = append(recommendations, "Prioritize this issue to unblock dependent work")
		recommendations = append(recommendations, "Communicate delays early to affected stakeholders")
	}
	
	blocking := graph.GetBlockingIssues(issueID)
	if len(blocking) > 3 {
		recommendations = append(recommendations, "This issue has many blockers - consider parallel work where possible")
	}
	
	return recommendations
}

func (s *DependencyService) findCriticalPath(graph *entities.DependencyGraph, issueID entities.IssueID) []entities.IssueID {
	// Find the longest path from this issue through its dependencies
	visited := make(map[entities.IssueID]bool)
	var longestPath []entities.IssueID
	
	s.dfsLongestPath(graph, issueID, visited, []entities.IssueID{}, &longestPath)
	
	return longestPath
}

func (s *DependencyService) dfsLongestPath(graph *entities.DependencyGraph, current entities.IssueID, visited map[entities.IssueID]bool, currentPath []entities.IssueID, longestPath *[]entities.IssueID) {
	if visited[current] {
		return
	}
	
	visited[current] = true
	currentPath = append(currentPath, current)
	
	blocked := graph.GetBlockedIssues(current)
	if len(blocked) == 0 {
		// Leaf node - check if this path is longer
		if len(currentPath) > len(*longestPath) {
			*longestPath = make([]entities.IssueID, len(currentPath))
			copy(*longestPath, currentPath)
		}
	} else {
		for _, next := range blocked {
			s.dfsLongestPath(graph, next, visited, currentPath, longestPath)
		}
	}
	
	visited[current] = false // Allow revisiting in other paths
}