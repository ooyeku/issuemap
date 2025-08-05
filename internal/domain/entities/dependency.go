package entities

import (
	"fmt"
	"time"
)

// DependencyType represents the type of dependency relationship
type DependencyType string

const (
	DependencyTypeBlocks   DependencyType = "blocks"   // Source blocks Target (Source must be completed before Target can be started)
	DependencyTypeRequires DependencyType = "requires" // Source requires Target (Target must be completed before Source can be completed)
)

// DependencyStatus represents the current status of a dependency
type DependencyStatus string

const (
	DependencyStatusActive   DependencyStatus = "active"   // Dependency is active and blocking
	DependencyStatusResolved DependencyStatus = "resolved" // Dependency has been resolved
	DependencyStatusIgnored  DependencyStatus = "ignored"  // Dependency has been manually ignored
)

// Dependency represents a dependency relationship between two issues
type Dependency struct {
	ID          string           `yaml:"id" json:"id"`
	SourceID    IssueID          `yaml:"source_id" json:"source_id"`       // The issue that has the dependency
	TargetID    IssueID          `yaml:"target_id" json:"target_id"`       // The issue being depended upon
	Type        DependencyType   `yaml:"type" json:"type"`                 // Type of dependency (blocks/requires)
	Status      DependencyStatus `yaml:"status" json:"status"`             // Current status
	Description string           `yaml:"description,omitempty" json:"description,omitempty"` // Optional description
	CreatedBy   string           `yaml:"created_by" json:"created_by"`     // Who created this dependency
	CreatedAt   time.Time        `yaml:"created_at" json:"created_at"`     // When it was created
	UpdatedAt   time.Time        `yaml:"updated_at" json:"updated_at"`     // When it was last updated
	ResolvedAt  *time.Time       `yaml:"resolved_at,omitempty" json:"resolved_at,omitempty"` // When it was resolved
	ResolvedBy  string           `yaml:"resolved_by,omitempty" json:"resolved_by,omitempty"` // Who resolved it
}

// DependencyGraph represents the dependency relationships for issues
type DependencyGraph struct {
	Dependencies map[string]*Dependency `yaml:"dependencies" json:"dependencies"` // All dependencies keyed by ID
	SourceIndex  map[IssueID][]string   `yaml:"-" json:"-"`                       // Index: source issue -> dependency IDs
	TargetIndex  map[IssueID][]string   `yaml:"-" json:"-"`                       // Index: target issue -> dependency IDs
}

// DependencyValidationResult represents the result of dependency validation
type DependencyValidationResult struct {
	IsValid        bool              `json:"is_valid"`
	CircularPaths  [][]IssueID       `json:"circular_paths,omitempty"`
	ConflictingDeps []*Dependency    `json:"conflicting_deps,omitempty"`
	Warnings       []string          `json:"warnings,omitempty"`
}

// BlockingInfo represents information about what is blocking an issue
type BlockingInfo struct {
	IssueID           IssueID       `json:"issue_id"`
	IsBlocked         bool          `json:"is_blocked"`
	BlockedBy         []IssueID     `json:"blocked_by,omitempty"`         // Issues that must be completed first
	Blocking          []IssueID     `json:"blocking,omitempty"`           // Issues that are waiting for this one
	UnresolvedDeps    []*Dependency `json:"unresolved_deps,omitempty"`    // Active dependencies
	BlockingCount     int           `json:"blocking_count"`               // Number of issues this blocks
	CriticalPath      bool          `json:"critical_path"`                // Whether this issue is on the critical path
}

// DependencyImpactAnalysis represents analysis of dependency changes
type DependencyImpactAnalysis struct {
	AffectedIssues    []IssueID            `json:"affected_issues"`
	CriticalPath      []IssueID            `json:"critical_path"`
	DelayEstimate     *time.Duration       `json:"delay_estimate,omitempty"`
	RiskLevel         string               `json:"risk_level"` // low, medium, high, critical
	Recommendations   []string             `json:"recommendations"`
	BlockingChain     map[IssueID][]IssueID `json:"blocking_chain"` // Issue -> issues it transitively blocks
}

// NewDependency creates a new dependency relationship
func NewDependency(sourceID, targetID IssueID, depType DependencyType, description, createdBy string) *Dependency {
	now := time.Now()
	return &Dependency{
		ID:          fmt.Sprintf("%s-%s-%s", sourceID, depType, targetID),
		SourceID:    sourceID,
		TargetID:    targetID,
		Type:        depType,
		Status:      DependencyStatusActive,
		Description: description,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewDependencyGraph creates a new dependency graph with initialized indices
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		Dependencies: make(map[string]*Dependency),
		SourceIndex:  make(map[IssueID][]string),
		TargetIndex:  make(map[IssueID][]string),
	}
}

// Validate validates the dependency data
func (d *Dependency) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("dependency ID cannot be empty")
	}
	if d.SourceID == "" {
		return fmt.Errorf("source issue ID cannot be empty")
	}
	if d.TargetID == "" {
		return fmt.Errorf("target issue ID cannot be empty")
	}
	if d.SourceID == d.TargetID {
		return fmt.Errorf("an issue cannot depend on itself")
	}
	if d.Type == "" {
		return fmt.Errorf("dependency type cannot be empty")
	}
	if d.Type != DependencyTypeBlocks && d.Type != DependencyTypeRequires {
		return fmt.Errorf("invalid dependency type: %s", d.Type)
	}
	if d.Status == "" {
		return fmt.Errorf("dependency status cannot be empty")
	}
	if d.CreatedBy == "" {
		return fmt.Errorf("created by cannot be empty")
	}
	return nil
}

// Resolve marks the dependency as resolved
func (d *Dependency) Resolve(resolvedBy string) {
	d.Status = DependencyStatusResolved
	now := time.Now()
	d.ResolvedAt = &now
	d.ResolvedBy = resolvedBy
	d.UpdatedAt = now
}

// Ignore marks the dependency as ignored
func (d *Dependency) Ignore(ignoredBy string) {
	d.Status = DependencyStatusIgnored
	d.ResolvedBy = ignoredBy
	d.UpdatedAt = time.Now()
}

// Reactivate marks the dependency as active again
func (d *Dependency) Reactivate() {
	d.Status = DependencyStatusActive
	d.ResolvedAt = nil
	d.ResolvedBy = ""
	d.UpdatedAt = time.Now()
}

// IsActive returns true if the dependency is currently active
func (d *Dependency) IsActive() bool {
	return d.Status == DependencyStatusActive
}

// IsResolved returns true if the dependency has been resolved
func (d *Dependency) IsResolved() bool {
	return d.Status == DependencyStatusResolved
}

// GetOppositeType returns the opposite dependency type
func (d *Dependency) GetOppositeType() DependencyType {
	if d.Type == DependencyTypeBlocks {
		return DependencyTypeRequires
	}
	return DependencyTypeBlocks
}

// String returns a human-readable description of the dependency
func (d *Dependency) String() string {
	switch d.Type {
	case DependencyTypeBlocks:
		return fmt.Sprintf("%s blocks %s", d.SourceID, d.TargetID)
	case DependencyTypeRequires:
		return fmt.Sprintf("%s requires %s", d.SourceID, d.TargetID)
	default:
		return fmt.Sprintf("%s -> %s (%s)", d.SourceID, d.TargetID, d.Type)
	}
}

// AddDependency adds a dependency to the graph and updates indices
func (g *DependencyGraph) AddDependency(dep *Dependency) {
	g.Dependencies[dep.ID] = dep
	
	// Update source index
	if deps, exists := g.SourceIndex[dep.SourceID]; exists {
		g.SourceIndex[dep.SourceID] = append(deps, dep.ID)
	} else {
		g.SourceIndex[dep.SourceID] = []string{dep.ID}
	}
	
	// Update target index
	if deps, exists := g.TargetIndex[dep.TargetID]; exists {
		g.TargetIndex[dep.TargetID] = append(deps, dep.ID)
	} else {
		g.TargetIndex[dep.TargetID] = []string{dep.ID}
	}
}

// RemoveDependency removes a dependency from the graph and updates indices
func (g *DependencyGraph) RemoveDependency(depID string) {
	dep, exists := g.Dependencies[depID]
	if !exists {
		return
	}
	
	// Remove from dependencies
	delete(g.Dependencies, depID)
	
	// Update source index
	if deps, exists := g.SourceIndex[dep.SourceID]; exists {
		for i, id := range deps {
			if id == depID {
				g.SourceIndex[dep.SourceID] = append(deps[:i], deps[i+1:]...)
				break
			}
		}
		if len(g.SourceIndex[dep.SourceID]) == 0 {
			delete(g.SourceIndex, dep.SourceID)
		}
	}
	
	// Update target index
	if deps, exists := g.TargetIndex[dep.TargetID]; exists {
		for i, id := range deps {
			if id == depID {
				g.TargetIndex[dep.TargetID] = append(deps[:i], deps[i+1:]...)
				break
			}
		}
		if len(g.TargetIndex[dep.TargetID]) == 0 {
			delete(g.TargetIndex, dep.TargetID)
		}
	}
}

// GetDependenciesFromSource returns all dependencies where the given issue is the source
func (g *DependencyGraph) GetDependenciesFromSource(issueID IssueID) []*Dependency {
	var result []*Dependency
	if depIDs, exists := g.SourceIndex[issueID]; exists {
		for _, depID := range depIDs {
			if dep, exists := g.Dependencies[depID]; exists {
				result = append(result, dep)
			}
		}
	}
	return result
}

// GetDependenciesFromTarget returns all dependencies where the given issue is the target
func (g *DependencyGraph) GetDependenciesFromTarget(issueID IssueID) []*Dependency {
	var result []*Dependency
	if depIDs, exists := g.TargetIndex[issueID]; exists {
		for _, depID := range depIDs {
			if dep, exists := g.Dependencies[depID]; exists {
				result = append(result, dep)
			}
		}
	}
	return result
}

// GetBlockingIssues returns all issues that are blocking the given issue
func (g *DependencyGraph) GetBlockingIssues(issueID IssueID) []IssueID {
	var blocking []IssueID
	
	// Get all dependencies where this issue is the source
	deps := g.GetDependenciesFromSource(issueID)
	for _, dep := range deps {
		if dep.IsActive() {
			if dep.Type == DependencyTypeRequires {
				// This issue requires the target, so target blocks this
				blocking = append(blocking, dep.TargetID)
			}
		}
	}
	
	// Get all dependencies where this issue is the target
	targetDeps := g.GetDependenciesFromTarget(issueID)
	for _, dep := range targetDeps {
		if dep.IsActive() {
			if dep.Type == DependencyTypeBlocks {
				// Source blocks this issue
				blocking = append(blocking, dep.SourceID)
			}
		}
	}
	
	return blocking
}

// GetBlockedIssues returns all issues that are blocked by the given issue
func (g *DependencyGraph) GetBlockedIssues(issueID IssueID) []IssueID {
	var blocked []IssueID
	
	// Get all dependencies where this issue is the source
	deps := g.GetDependenciesFromSource(issueID)
	for _, dep := range deps {
		if dep.IsActive() {
			if dep.Type == DependencyTypeBlocks {
				// This issue blocks the target
				blocked = append(blocked, dep.TargetID)
			}
		}
	}
	
	// Get all dependencies where this issue is the target
	targetDeps := g.GetDependenciesFromTarget(issueID)
	for _, dep := range targetDeps {
		if dep.IsActive() {
			if dep.Type == DependencyTypeRequires {
				// Source requires this, so this blocks source
				blocked = append(blocked, dep.SourceID)
			}
		}
	}
	
	return blocked
}

// IsBlocked returns true if the issue is blocked by any other issues
func (g *DependencyGraph) IsBlocked(issueID IssueID) bool {
	blocking := g.GetBlockingIssues(issueID)
	return len(blocking) > 0
}

// HasCircularDependency checks if adding a dependency would create a circular dependency
func (g *DependencyGraph) HasCircularDependency(from, to IssueID) bool {
	// Use DFS to check if there's already a path from 'to' to 'from'
	visited := make(map[IssueID]bool)
	return g.dfsHasPath(to, from, visited)
}

// dfsHasPath performs depth-first search to find if there's a path from start to target
func (g *DependencyGraph) dfsHasPath(start, target IssueID, visited map[IssueID]bool) bool {
	if start == target {
		return true
	}
	
	if visited[start] {
		return false
	}
	
	visited[start] = true
	
	// Check all issues that start depends on or blocks
	deps := g.GetDependenciesFromSource(start)
	for _, dep := range deps {
		if !dep.IsActive() {
			continue
		}
		
		var nextIssue IssueID
		if dep.Type == DependencyTypeBlocks {
			nextIssue = dep.TargetID
		} else { // DependencyTypeRequires
			nextIssue = dep.TargetID
		}
		
		if g.dfsHasPath(nextIssue, target, visited) {
			return true
		}
	}
	
	return false
}

// FindCircularDependencies finds all circular dependency paths in the graph
func (g *DependencyGraph) FindCircularDependencies() [][]IssueID {
	var cycles [][]IssueID
	visited := make(map[IssueID]bool)
	recStack := make(map[IssueID]bool)
	
	// Get all unique issue IDs
	allIssues := make(map[IssueID]bool)
	for _, dep := range g.Dependencies {
		if dep.IsActive() {
			allIssues[dep.SourceID] = true
			allIssues[dep.TargetID] = true
		}
	}
	
	// Check each issue for cycles
	for issueID := range allIssues {
		if !visited[issueID] {
			path := []IssueID{}
			g.dfsFindCycles(issueID, visited, recStack, path, &cycles)
		}
	}
	
	return cycles
}

// dfsFindCycles performs DFS to find cycles
func (g *DependencyGraph) dfsFindCycles(issueID IssueID, visited, recStack map[IssueID]bool, path []IssueID, cycles *[][]IssueID) {
	visited[issueID] = true
	recStack[issueID] = true
	path = append(path, issueID)
	
	deps := g.GetDependenciesFromSource(issueID)
	for _, dep := range deps {
		if !dep.IsActive() {
			continue
		}
		
		var nextIssue IssueID
		if dep.Type == DependencyTypeBlocks {
			nextIssue = dep.TargetID
		} else {
			nextIssue = dep.TargetID
		}
		
		if !visited[nextIssue] {
			g.dfsFindCycles(nextIssue, visited, recStack, path, cycles)
		} else if recStack[nextIssue] {
			// Found a cycle, extract it from the path
			cycleStart := -1
			for i, id := range path {
				if id == nextIssue {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := make([]IssueID, len(path)-cycleStart)
				copy(cycle, path[cycleStart:])
				*cycles = append(*cycles, cycle)
			}
		}
	}
	
	recStack[issueID] = false
}