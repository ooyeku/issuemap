package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

func TestDependency_NewDependency(t *testing.T) {
	sourceID := entities.IssueID("ISSUE-001")
	targetID := entities.IssueID("ISSUE-002")
	depType := entities.DependencyTypeBlocks
	description := "Blocks UI work"
	createdBy := "john.doe"

	dep := entities.NewDependency(sourceID, targetID, depType, description, createdBy)

	assert.Equal(t, sourceID, dep.SourceID)
	assert.Equal(t, targetID, dep.TargetID)
	assert.Equal(t, depType, dep.Type)
	assert.Equal(t, description, dep.Description)
	assert.Equal(t, createdBy, dep.CreatedBy)
	assert.Equal(t, entities.DependencyStatusActive, dep.Status)
	assert.NotEmpty(t, dep.ID)
	assert.False(t, dep.CreatedAt.IsZero())
	assert.False(t, dep.UpdatedAt.IsZero())
}

func TestDependency_Validate(t *testing.T) {
	testCases := []struct {
		name        string
		dependency  *entities.Dependency
		expectError bool
	}{
		{
			name: "valid dependency",
			dependency: &entities.Dependency{
				ID:        "test-id",
				SourceID:  "ISSUE-001",
				TargetID:  "ISSUE-002",
				Type:      entities.DependencyTypeBlocks,
				Status:    entities.DependencyStatusActive,
				CreatedBy: "author",
			},
			expectError: false,
		},
		{
			name: "empty ID",
			dependency: &entities.Dependency{
				ID:        "",
				SourceID:  "ISSUE-001",
				TargetID:  "ISSUE-002",
				Type:      entities.DependencyTypeBlocks,
				Status:    entities.DependencyStatusActive,
				CreatedBy: "author",
			},
			expectError: true,
		},
		{
			name: "empty source ID",
			dependency: &entities.Dependency{
				ID:        "test-id",
				SourceID:  "",
				TargetID:  "ISSUE-002",
				Type:      entities.DependencyTypeBlocks,
				Status:    entities.DependencyStatusActive,
				CreatedBy: "author",
			},
			expectError: true,
		},
		{
			name: "empty target ID",
			dependency: &entities.Dependency{
				ID:        "test-id",
				SourceID:  "ISSUE-001",
				TargetID:  "",
				Type:      entities.DependencyTypeBlocks,
				Status:    entities.DependencyStatusActive,
				CreatedBy: "author",
			},
			expectError: true,
		},
		{
			name: "self dependency",
			dependency: &entities.Dependency{
				ID:        "test-id",
				SourceID:  "ISSUE-001",
				TargetID:  "ISSUE-001",
				Type:      entities.DependencyTypeBlocks,
				Status:    entities.DependencyStatusActive,
				CreatedBy: "author",
			},
			expectError: true,
		},
		{
			name: "invalid type",
			dependency: &entities.Dependency{
				ID:        "test-id",
				SourceID:  "ISSUE-001",
				TargetID:  "ISSUE-002",
				Type:      entities.DependencyType("invalid"),
				Status:    entities.DependencyStatusActive,
				CreatedBy: "author",
			},
			expectError: true,
		},
		{
			name: "empty created by",
			dependency: &entities.Dependency{
				ID:        "test-id",
				SourceID:  "ISSUE-001",
				TargetID:  "ISSUE-002",
				Type:      entities.DependencyTypeBlocks,
				Status:    entities.DependencyStatusActive,
				CreatedBy: "",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.dependency.Validate()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDependency_StatusOperations(t *testing.T) {
	dep := entities.NewDependency("ISSUE-001", "ISSUE-002", entities.DependencyTypeBlocks, "Test", "author")

	// Test initial state
	assert.True(t, dep.IsActive())
	assert.False(t, dep.IsResolved())

	// Test resolve
	dep.Resolve("resolver")
	assert.False(t, dep.IsActive())
	assert.True(t, dep.IsResolved())
	assert.Equal(t, entities.DependencyStatusResolved, dep.Status)
	assert.NotNil(t, dep.ResolvedAt)
	assert.Equal(t, "resolver", dep.ResolvedBy)

	// Test reactivate
	dep.Reactivate()
	assert.True(t, dep.IsActive())
	assert.False(t, dep.IsResolved())
	assert.Equal(t, entities.DependencyStatusActive, dep.Status)
	assert.Nil(t, dep.ResolvedAt)
	assert.Empty(t, dep.ResolvedBy)
}

func TestDependency_GetOppositeType(t *testing.T) {
	blocksDep := &entities.Dependency{Type: entities.DependencyTypeBlocks}
	assert.Equal(t, entities.DependencyTypeRequires, blocksDep.GetOppositeType())

	requiresDep := &entities.Dependency{Type: entities.DependencyTypeRequires}
	assert.Equal(t, entities.DependencyTypeBlocks, requiresDep.GetOppositeType())
}

func TestDependency_String(t *testing.T) {
	blocksDep := &entities.Dependency{
		SourceID: "ISSUE-001",
		TargetID: "ISSUE-002",
		Type:     entities.DependencyTypeBlocks,
	}
	assert.Equal(t, "ISSUE-001 blocks ISSUE-002", blocksDep.String())

	requiresDep := &entities.Dependency{
		SourceID: "ISSUE-003",
		TargetID: "ISSUE-004",
		Type:     entities.DependencyTypeRequires,
	}
	assert.Equal(t, "ISSUE-003 requires ISSUE-004", requiresDep.String())
}

func TestDependencyGraph_NewDependencyGraph(t *testing.T) {
	graph := entities.NewDependencyGraph()

	assert.NotNil(t, graph.Dependencies)
	assert.NotNil(t, graph.SourceIndex)
	assert.NotNil(t, graph.TargetIndex)
	assert.Empty(t, graph.Dependencies)
	assert.Empty(t, graph.SourceIndex)
	assert.Empty(t, graph.TargetIndex)
}

func TestDependencyGraph_AddDependency(t *testing.T) {
	graph := entities.NewDependencyGraph()
	dep := entities.NewDependency("ISSUE-001", "ISSUE-002", entities.DependencyTypeBlocks, "Test", "author")

	graph.AddDependency(dep)

	assert.Contains(t, graph.Dependencies, dep.ID)
	assert.Contains(t, graph.SourceIndex["ISSUE-001"], dep.ID)
	assert.Contains(t, graph.TargetIndex["ISSUE-002"], dep.ID)
}

func TestDependencyGraph_RemoveDependency(t *testing.T) {
	graph := entities.NewDependencyGraph()
	dep := entities.NewDependency("ISSUE-001", "ISSUE-002", entities.DependencyTypeBlocks, "Test", "author")

	graph.AddDependency(dep)
	assert.Contains(t, graph.Dependencies, dep.ID)

	graph.RemoveDependency(dep.ID)
	assert.NotContains(t, graph.Dependencies, dep.ID)
	assert.Empty(t, graph.SourceIndex["ISSUE-001"])
	assert.Empty(t, graph.TargetIndex["ISSUE-002"])
}

func TestDependencyGraph_GetDependencies(t *testing.T) {
	graph := entities.NewDependencyGraph()

	dep1 := entities.NewDependency("ISSUE-001", "ISSUE-002", entities.DependencyTypeBlocks, "Test 1", "author")
	dep2 := entities.NewDependency("ISSUE-001", "ISSUE-003", entities.DependencyTypeRequires, "Test 2", "author")
	dep3 := entities.NewDependency("ISSUE-004", "ISSUE-001", entities.DependencyTypeBlocks, "Test 3", "author")

	graph.AddDependency(dep1)
	graph.AddDependency(dep2)
	graph.AddDependency(dep3)

	// Test GetDependenciesFromSource
	sourceDeps := graph.GetDependenciesFromSource("ISSUE-001")
	assert.Len(t, sourceDeps, 2)
	depIDs := []string{sourceDeps[0].ID, sourceDeps[1].ID}
	assert.Contains(t, depIDs, dep1.ID)
	assert.Contains(t, depIDs, dep2.ID)

	// Test GetDependenciesFromTarget
	targetDeps := graph.GetDependenciesFromTarget("ISSUE-001")
	assert.Len(t, targetDeps, 1)
	assert.Equal(t, dep3.ID, targetDeps[0].ID)
}

func TestDependencyGraph_BlockingRelationships(t *testing.T) {
	graph := entities.NewDependencyGraph()

	// ISSUE-001 blocks ISSUE-002
	dep1 := entities.NewDependency("ISSUE-001", "ISSUE-002", entities.DependencyTypeBlocks, "Test", "author")
	// ISSUE-003 requires ISSUE-001 (so ISSUE-001 blocks ISSUE-003)
	dep2 := entities.NewDependency("ISSUE-003", "ISSUE-001", entities.DependencyTypeRequires, "Test", "author")

	graph.AddDependency(dep1)
	graph.AddDependency(dep2)

	// ISSUE-001 should block both ISSUE-002 and ISSUE-003
	blocked := graph.GetBlockedIssues("ISSUE-001")
	assert.Len(t, blocked, 2)
	assert.Contains(t, blocked, entities.IssueID("ISSUE-002"))
	assert.Contains(t, blocked, entities.IssueID("ISSUE-003"))

	// ISSUE-002 should be blocked by ISSUE-001
	blocking002 := graph.GetBlockingIssues("ISSUE-002")
	assert.Len(t, blocking002, 1)
	assert.Contains(t, blocking002, entities.IssueID("ISSUE-001"))

	// ISSUE-003 should be blocked by ISSUE-001
	blocking003 := graph.GetBlockingIssues("ISSUE-003")
	assert.Len(t, blocking003, 1)
	assert.Contains(t, blocking003, entities.IssueID("ISSUE-001"))

	// Test IsBlocked
	assert.False(t, graph.IsBlocked("ISSUE-001"))
	assert.True(t, graph.IsBlocked("ISSUE-002"))
	assert.True(t, graph.IsBlocked("ISSUE-003"))
}

func TestDependencyGraph_CircularDependencyDetection(t *testing.T) {
	graph := entities.NewDependencyGraph()

	// Create a simple cycle: A blocks B, B blocks C, C blocks A
	dep1 := entities.NewDependency("ISSUE-A", "ISSUE-B", entities.DependencyTypeBlocks, "Test", "author")
	dep2 := entities.NewDependency("ISSUE-B", "ISSUE-C", entities.DependencyTypeBlocks, "Test", "author")
	dep3 := entities.NewDependency("ISSUE-C", "ISSUE-A", entities.DependencyTypeBlocks, "Test", "author")

	graph.AddDependency(dep1)
	graph.AddDependency(dep2)

	// Should detect potential circular dependency (A->B->C exists, so C->A would create cycle)
	assert.True(t, graph.HasCircularDependency("ISSUE-C", "ISSUE-A"))

	graph.AddDependency(dep3)

	// Now should detect circular dependency
	assert.True(t, graph.HasCircularDependency("ISSUE-A", "ISSUE-B"))
	assert.True(t, graph.HasCircularDependency("ISSUE-B", "ISSUE-C"))
	assert.True(t, graph.HasCircularDependency("ISSUE-C", "ISSUE-A"))

	// Find circular dependencies
	cycles := graph.FindCircularDependencies()
	assert.Len(t, cycles, 1)
	assert.Len(t, cycles[0], 3)
}

func TestDependencyGraph_ComplexScenario(t *testing.T) {
	graph := entities.NewDependencyGraph()

	// Create a more complex dependency graph:
	// FRONTEND requires BACKEND
	// BACKEND requires DATABASE
	// UI blocks FRONTEND
	// DEPLOY requires UI

	dep1 := entities.NewDependency("FRONTEND", "BACKEND", entities.DependencyTypeRequires, "Frontend needs backend", "dev")
	dep2 := entities.NewDependency("BACKEND", "DATABASE", entities.DependencyTypeRequires, "Backend needs DB", "dev")
	dep3 := entities.NewDependency("UI", "FRONTEND", entities.DependencyTypeBlocks, "UI blocks frontend", "designer")
	dep4 := entities.NewDependency("DEPLOY", "UI", entities.DependencyTypeRequires, "Deploy needs UI", "devops")

	graph.AddDependency(dep1)
	graph.AddDependency(dep2)
	graph.AddDependency(dep3)
	graph.AddDependency(dep4)

	// DATABASE should directly block BACKEND (BACKEND requires DATABASE)
	databaseBlocked := graph.GetBlockedIssues("DATABASE")
	assert.Contains(t, databaseBlocked, entities.IssueID("BACKEND"))
	// DATABASE does not directly block FRONTEND - only through BACKEND

	// FRONTEND should be blocked by BACKEND (FRONTEND requires BACKEND) and UI (UI blocks FRONTEND)
	frontendBlocking := graph.GetBlockingIssues("FRONTEND")
	assert.Contains(t, frontendBlocking, entities.IssueID("BACKEND"))
	assert.Contains(t, frontendBlocking, entities.IssueID("UI"))

	// UI should block FRONTEND and DEPLOY
	uiBlocked := graph.GetBlockedIssues("UI")
	assert.Contains(t, uiBlocked, entities.IssueID("FRONTEND"))
	assert.Contains(t, uiBlocked, entities.IssueID("DEPLOY"))

	// Should not have circular dependencies
	cycles := graph.FindCircularDependencies()
	assert.Empty(t, cycles)
}

func TestBlockingInfo(t *testing.T) {
	blockingInfo := &entities.BlockingInfo{
		IssueID:       "ISSUE-001",
		IsBlocked:     true,
		BlockedBy:     []entities.IssueID{"ISSUE-002", "ISSUE-003"},
		Blocking:      []entities.IssueID{"ISSUE-004"},
		BlockingCount: 1,
		CriticalPath:  false,
	}

	assert.Equal(t, entities.IssueID("ISSUE-001"), blockingInfo.IssueID)
	assert.True(t, blockingInfo.IsBlocked)
	assert.Len(t, blockingInfo.BlockedBy, 2)
	assert.Len(t, blockingInfo.Blocking, 1)
	assert.Equal(t, 1, blockingInfo.BlockingCount)
	assert.False(t, blockingInfo.CriticalPath)
}

func TestDependencyImpactAnalysis(t *testing.T) {
	analysis := &entities.DependencyImpactAnalysis{
		AffectedIssues:  []entities.IssueID{"ISSUE-001", "ISSUE-002"},
		CriticalPath:    []entities.IssueID{"ISSUE-001", "ISSUE-002", "ISSUE-003"},
		RiskLevel:       "high",
		Recommendations: []string{"Consider parallel work", "Communicate delays"},
		BlockingChain:   make(map[entities.IssueID][]entities.IssueID),
	}

	analysis.BlockingChain["ISSUE-002"] = []entities.IssueID{"ISSUE-001", "ISSUE-002"}

	assert.Len(t, analysis.AffectedIssues, 2)
	assert.Len(t, analysis.CriticalPath, 3)
	assert.Equal(t, "high", analysis.RiskLevel)
	assert.Len(t, analysis.Recommendations, 2)
	assert.Contains(t, analysis.BlockingChain, entities.IssueID("ISSUE-002"))
}

func TestDependencyValidationResult(t *testing.T) {
	result := &entities.DependencyValidationResult{
		IsValid:       false,
		CircularPaths: [][]entities.IssueID{{"ISSUE-001", "ISSUE-002", "ISSUE-001"}},
		Warnings:      []string{"Found 1 circular dependency"},
	}

	assert.False(t, result.IsValid)
	assert.Len(t, result.CircularPaths, 1)
	assert.Len(t, result.Warnings, 1)
}
