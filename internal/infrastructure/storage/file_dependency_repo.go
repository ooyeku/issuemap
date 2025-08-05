package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

const (
	dependenciesDir = "dependencies"
)

// FileDependencyRepository implements dependency persistence using files
type FileDependencyRepository struct {
	basePath string
}

// NewFileDependencyRepository creates a new file-based dependency repository
func NewFileDependencyRepository(basePath string) *FileDependencyRepository {
	return &FileDependencyRepository{
		basePath: basePath,
	}
}

// Create saves a new dependency
func (r *FileDependencyRepository) Create(ctx context.Context, dependency *entities.Dependency) error {
	if err := dependency.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	dir := filepath.Join(r.basePath, dependenciesDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	filename := fmt.Sprintf("%s.yaml", dependency.ID)
	path := filepath.Join(dir, filename)

	data, err := yaml.Marshal(dependency)
	if err != nil {
		return fmt.Errorf("failed to marshal dependency: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write dependency: %w", err)
	}

	return nil
}

// GetByID retrieves a dependency by its ID
func (r *FileDependencyRepository) GetByID(ctx context.Context, id string) (*entities.Dependency, error) {
	path := filepath.Join(r.basePath, dependenciesDir, fmt.Sprintf("%s.yaml", id))
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("dependency not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read dependency: %w", err)
	}

	var dependency entities.Dependency
	if err := yaml.Unmarshal(data, &dependency); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dependency: %w", err)
	}

	return &dependency, nil
}

// Update updates an existing dependency
func (r *FileDependencyRepository) Update(ctx context.Context, dependency *entities.Dependency) error {
	if err := dependency.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	path := filepath.Join(r.basePath, dependenciesDir, fmt.Sprintf("%s.yaml", dependency.ID))
	
	// Check if dependency exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("dependency not found: %s", dependency.ID)
	}

	dependency.UpdatedAt = time.Now()

	data, err := yaml.Marshal(dependency)
	if err != nil {
		return fmt.Errorf("failed to marshal dependency: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write dependency: %w", err)
	}

	return nil
}

// Delete removes a dependency by its ID
func (r *FileDependencyRepository) Delete(ctx context.Context, id string) error {
	path := filepath.Join(r.basePath, dependenciesDir, fmt.Sprintf("%s.yaml", id))
	
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("dependency not found: %s", id)
		}
		return fmt.Errorf("failed to delete dependency: %w", err)
	}

	return nil
}

// List retrieves dependencies with optional filtering
func (r *FileDependencyRepository) List(ctx context.Context, filter repositories.DependencyFilter) ([]*entities.Dependency, error) {
	dir := filepath.Join(r.basePath, dependenciesDir)
	
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*entities.Dependency{}, nil
		}
		return nil, fmt.Errorf("failed to read dependencies directory: %w", err)
	}

	var dependencies []*entities.Dependency
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip files we can't read
		}

		var dependency entities.Dependency
		if err := yaml.Unmarshal(data, &dependency); err != nil {
			continue // Skip files we can't parse
		}

		// Apply filters
		if !r.matchesFilter(&dependency, filter) {
			continue
		}

		dependencies = append(dependencies, &dependency)
	}

	// Sort by creation time (newest first)
	sort.Slice(dependencies, func(i, j int) bool {
		return dependencies[i].CreatedAt.After(dependencies[j].CreatedAt)
	})

	// Apply limit and offset
	if filter.Offset > 0 && filter.Offset < len(dependencies) {
		dependencies = dependencies[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(dependencies) {
		dependencies = dependencies[:filter.Limit]
	}

	return dependencies, nil
}

// GetByIssueID retrieves all dependencies for a specific issue (as source or target)
func (r *FileDependencyRepository) GetByIssueID(ctx context.Context, issueID entities.IssueID) ([]*entities.Dependency, error) {
	allDeps, err := r.List(ctx, repositories.DependencyFilter{})
	if err != nil {
		return nil, err
	}

	var result []*entities.Dependency
	for _, dep := range allDeps {
		if dep.SourceID == issueID || dep.TargetID == issueID {
			result = append(result, dep)
		}
	}

	return result, nil
}

// GetBySourceID retrieves all dependencies where the issue is the source
func (r *FileDependencyRepository) GetBySourceID(ctx context.Context, sourceID entities.IssueID) ([]*entities.Dependency, error) {
	filter := repositories.DependencyFilter{
		SourceID: &sourceID,
	}
	return r.List(ctx, filter)
}

// GetByTargetID retrieves all dependencies where the issue is the target
func (r *FileDependencyRepository) GetByTargetID(ctx context.Context, targetID entities.IssueID) ([]*entities.Dependency, error) {
	filter := repositories.DependencyFilter{
		TargetID: &targetID,
	}
	return r.List(ctx, filter)
}

// GetActiveDependencies retrieves all active dependencies
func (r *FileDependencyRepository) GetActiveDependencies(ctx context.Context) ([]*entities.Dependency, error) {
	activeStatus := entities.DependencyStatusActive
	filter := repositories.DependencyFilter{
		Status: &activeStatus,
	}
	return r.List(ctx, filter)
}

// GetDependencyGraph builds and returns the complete dependency graph
func (r *FileDependencyRepository) GetDependencyGraph(ctx context.Context) (*entities.DependencyGraph, error) {
	dependencies, err := r.List(ctx, repositories.DependencyFilter{})
	if err != nil {
		return nil, err
	}

	graph := entities.NewDependencyGraph()
	for _, dep := range dependencies {
		graph.AddDependency(dep)
	}

	return graph, nil
}

// GetStats returns dependency statistics
func (r *FileDependencyRepository) GetStats(ctx context.Context, filter repositories.DependencyFilter) (*repositories.DependencyStats, error) {
	dependencies, err := r.List(ctx, repositories.DependencyFilter{}) // Get all first, then filter
	if err != nil {
		return nil, err
	}

	// Filter dependencies
	var filteredDeps []*entities.Dependency
	for _, dep := range dependencies {
		if r.matchesFilter(dep, filter) {
			filteredDeps = append(filteredDeps, dep)
		}
	}

	if len(filteredDeps) == 0 {
		return &repositories.DependencyStats{
			DependenciesByType:   make(map[entities.DependencyType]int),
			DependenciesByStatus: make(map[entities.DependencyStatus]int),
			DependencyCreators:   make(map[string]int),
		}, nil
	}

	stats := &repositories.DependencyStats{
		TotalDependencies:    len(filteredDeps),
		DependenciesByType:   make(map[entities.DependencyType]int),
		DependenciesByStatus: make(map[entities.DependencyStatus]int),
		DependencyCreators:   make(map[string]int),
	}

	issueDepCount := make(map[entities.IssueID]int)
	issueBlockingCount := make(map[entities.IssueID]int)
	issueBlockedCount := make(map[entities.IssueID]int)

	for _, dep := range filteredDeps {
		// Count by type
		stats.DependenciesByType[dep.Type]++

		// Count by status
		stats.DependenciesByStatus[dep.Status]++
		if dep.Status == entities.DependencyStatusActive {
			stats.ActiveDependencies++
		} else if dep.Status == entities.DependencyStatusResolved {
			stats.ResolvedDependencies++
		}

		// Count by creator
		stats.DependencyCreators[dep.CreatedBy]++

		// Track issue involvement
		issueDepCount[dep.SourceID]++
		issueDepCount[dep.TargetID]++

		// Track blocking relationships
		if dep.IsActive() {
			if dep.Type == entities.DependencyTypeBlocks {
				issueBlockingCount[dep.SourceID]++
				issueBlockedCount[dep.TargetID]++
			} else { // DependencyTypeRequires
				issueBlockingCount[dep.TargetID]++
				issueBlockedCount[dep.SourceID]++
			}
		}
	}

	stats.IssuesWithDeps = len(issueDepCount)

	// Calculate average dependencies per issue
	if stats.IssuesWithDeps > 0 {
		stats.AverageDepPerIssue = float64(stats.TotalDependencies) / float64(stats.IssuesWithDeps)
	}

	// Find most blocked and blocking issues
	stats.MostBlockedIssues = r.getTopIssues(issueBlockedCount, 5)
	stats.MostBlockingIssues = r.getTopIssues(issueBlockingCount, 5)

	// Check for circular dependencies
	graph, err := r.GetDependencyGraph(ctx)
	if err == nil {
		cycles := graph.FindCircularDependencies()
		stats.CircularDependencies = len(cycles)
	}

	return stats, nil
}

// BulkUpdate updates multiple dependencies
func (r *FileDependencyRepository) BulkUpdate(ctx context.Context, dependencies []*entities.Dependency) error {
	for _, dep := range dependencies {
		if err := r.Update(ctx, dep); err != nil {
			return fmt.Errorf("failed to update dependency %s: %w", dep.ID, err)
		}
	}
	return nil
}

// DeleteByIssueID removes all dependencies for a specific issue
func (r *FileDependencyRepository) DeleteByIssueID(ctx context.Context, issueID entities.IssueID) error {
	dependencies, err := r.GetByIssueID(ctx, issueID)
	if err != nil {
		return err
	}

	for _, dep := range dependencies {
		if err := r.Delete(ctx, dep.ID); err != nil {
			return fmt.Errorf("failed to delete dependency %s: %w", dep.ID, err)
		}
	}

	return nil
}

// FindConflicts finds potentially conflicting dependencies
func (r *FileDependencyRepository) FindConflicts(ctx context.Context) ([]*entities.Dependency, error) {
	// For now, we'll just return dependencies that are part of circular dependencies
	graph, err := r.GetDependencyGraph(ctx)
	if err != nil {
		return nil, err
	}

	cycles := graph.FindCircularDependencies()
	if len(cycles) == 0 {
		return []*entities.Dependency{}, nil
	}

	// Collect all dependencies involved in cycles
	conflictingDepIDs := make(map[string]bool)
	for _, cycle := range cycles {
		for i := 0; i < len(cycle); i++ {
			source := cycle[i]
			target := cycle[(i+1)%len(cycle)]
			
			// Find dependencies between these issues
			sourceDeps := graph.GetDependenciesFromSource(source)
			for _, dep := range sourceDeps {
				if dep.TargetID == target && dep.IsActive() {
					conflictingDepIDs[dep.ID] = true
				}
			}
		}
	}

	var conflicts []*entities.Dependency
	for depID := range conflictingDepIDs {
		if dep, exists := graph.Dependencies[depID]; exists {
			conflicts = append(conflicts, dep)
		}
	}

	return conflicts, nil
}

// matchesFilter checks if a dependency matches the given filter
func (r *FileDependencyRepository) matchesFilter(dep *entities.Dependency, filter repositories.DependencyFilter) bool {
	if filter.SourceID != nil && dep.SourceID != *filter.SourceID {
		return false
	}
	if filter.TargetID != nil && dep.TargetID != *filter.TargetID {
		return false
	}
	if filter.Type != nil && dep.Type != *filter.Type {
		return false
	}
	if filter.Status != nil && dep.Status != *filter.Status {
		return false
	}
	if filter.CreatedBy != nil && dep.CreatedBy != *filter.CreatedBy {
		return false
	}
	if filter.DateFrom != nil && dep.CreatedAt.Before(*filter.DateFrom) {
		return false
	}
	if filter.DateTo != nil && dep.CreatedAt.After(*filter.DateTo) {
		return false
	}
	return true
}

// getTopIssues returns the top N issues by count, sorted descending
func (r *FileDependencyRepository) getTopIssues(counts map[entities.IssueID]int, limit int) []entities.IssueID {
	type issueCount struct {
		issue entities.IssueID
		count int
	}

	var items []issueCount
	for issue, count := range counts {
		items = append(items, issueCount{issue, count})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].count > items[j].count
	})

	var result []entities.IssueID
	for i := 0; i < len(items) && i < limit; i++ {
		result = append(result, items[i].issue)
	}

	return result
}