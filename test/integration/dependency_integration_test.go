package integration

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

func TestDependencyServiceIntegration(t *testing.T) {
	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "issuemap-dependency-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize repositories
	issueRepo := storage.NewFileIssueRepository(tempDir)
	configRepo := storage.NewFileConfigRepository(tempDir)
	dependencyRepo := storage.NewFileDependencyRepository(tempDir)
	historyRepo := storage.NewFileHistoryRepository(tempDir)

	// Initialize services
	issueService := services.NewIssueService(issueRepo, configRepo, nil)
	historyService := services.NewHistoryService(historyRepo, nil)
	dependencyService := services.NewDependencyService(dependencyRepo, issueService, historyService)

	ctx := context.Background()

	// Create test issues
	issue1, err := issueService.CreateIssue(ctx, services.CreateIssueRequest{
		Title:       "Backend API",
		Description: "Create backend API",
		Type:        entities.IssueTypeFeature,
		Priority:    entities.PriorityHigh,
	})
	require.NoError(t, err)

	issue2, err := issueService.CreateIssue(ctx, services.CreateIssueRequest{
		Title:       "Frontend UI",
		Description: "Create frontend UI",
		Type:        entities.IssueTypeFeature,
		Priority:    entities.PriorityMedium,
	})
	require.NoError(t, err)

	issue3, err := issueService.CreateIssue(ctx, services.CreateIssueRequest{
		Title:       "Database Schema",
		Description: "Create database schema",
		Type:        entities.IssueTypeTask,
		Priority:    entities.PriorityHigh,
	})
	require.NoError(t, err)

	author := "test.user"

	t.Run("Create and Manage Dependencies", func(t *testing.T) {
		// Create dependency: Backend requires Database
		dep1, err := dependencyService.CreateDependency(
			ctx,
			issue1.ID,
			issue3.ID,
			entities.DependencyTypeRequires,
			"Backend needs database schema",
			author,
		)
		require.NoError(t, err)
		assert.Equal(t, issue1.ID, dep1.SourceID)
		assert.Equal(t, issue3.ID, dep1.TargetID)
		assert.Equal(t, entities.DependencyTypeRequires, dep1.Type)
		assert.True(t, dep1.IsActive())

		// Create dependency: Database blocks Frontend
		_, err = dependencyService.CreateDependency(
			ctx,
			issue3.ID,
			issue2.ID,
			entities.DependencyTypeBlocks,
			"Database must be ready before frontend",
			author,
		)
		require.NoError(t, err)

		// Get dependencies for issue1
		deps1, err := dependencyService.GetIssueDependencies(ctx, issue1.ID)
		require.NoError(t, err)
		assert.Len(t, deps1, 1)
		assert.Equal(t, dep1.ID, deps1[0].ID)

		// Get dependencies for issue3 (should have 2 - as source and target)
		deps3, err := dependencyService.GetIssueDependencies(ctx, issue3.ID)
		require.NoError(t, err)
		assert.Len(t, deps3, 2)
	})

	t.Run("Blocking Analysis", func(t *testing.T) {
		// Get blocking info for frontend (should be blocked by database)
		blockingInfo, err := dependencyService.GetBlockingInfo(ctx, issue2.ID)
		require.NoError(t, err)
		assert.True(t, blockingInfo.IsBlocked)
		assert.Contains(t, blockingInfo.BlockedBy, issue3.ID)

		// Get blocking info for database (should not be blocked, but blocking others)
		dbBlockingInfo, err := dependencyService.GetBlockingInfo(ctx, issue3.ID)
		require.NoError(t, err)
		assert.False(t, dbBlockingInfo.IsBlocked)
		assert.Greater(t, dbBlockingInfo.BlockingCount, 0)

		// Get all blocked issues
		blockedIssues, err := dependencyService.GetBlockedIssues(ctx)
		require.NoError(t, err)
		assert.Contains(t, blockedIssues, issue1.ID) // Backend is blocked by database
		assert.Contains(t, blockedIssues, issue2.ID) // Frontend is blocked by database
	})

	t.Run("Dependency Graph Operations", func(t *testing.T) {
		graph, err := dependencyService.GetDependencyGraph(ctx)
		require.NoError(t, err)
		assert.Len(t, graph.Dependencies, 2)

		// Test blocking relationships in graph
		blocked := graph.GetBlockedIssues(issue3.ID)
		assert.Contains(t, blocked, issue1.ID) // Database blocks backend (via requires)
		assert.Contains(t, blocked, issue2.ID) // Database blocks frontend (via blocks)

		blocking := graph.GetBlockingIssues(issue1.ID)
		assert.Contains(t, blocking, issue3.ID) // Backend blocked by database

		blocking2 := graph.GetBlockingIssues(issue2.ID)
		assert.Contains(t, blocking2, issue3.ID) // Frontend blocked by database
	})

	t.Run("Circular Dependency Prevention", func(t *testing.T) {
		// Try to create a circular dependency: Frontend blocks Database
		_, err := dependencyService.CreateDependency(
			ctx,
			issue2.ID,
			issue3.ID,
			entities.DependencyTypeBlocks,
			"This would create a cycle",
			author,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circular dependency")
	})

	t.Run("Dependency Validation", func(t *testing.T) {
		result, err := dependencyService.ValidateDependencyGraph(ctx)
		require.NoError(t, err)
		assert.True(t, result.IsValid)
		assert.Empty(t, result.CircularPaths)
	})

	t.Run("Dependency Resolution", func(t *testing.T) {
		// Get a dependency to resolve
		deps, err := dependencyService.GetDependencies(ctx, repositories.DependencyFilter{})
		require.NoError(t, err)
		require.NotEmpty(t, deps)

		dep := deps[0]

		// Resolve the dependency
		err = dependencyService.ResolveDependency(ctx, dep.ID, author)
		require.NoError(t, err)

		// Verify it's resolved
		resolvedDep, err := dependencyRepo.GetByID(ctx, dep.ID)
		require.NoError(t, err)
		assert.True(t, resolvedDep.IsResolved())
		assert.Equal(t, author, resolvedDep.ResolvedBy)

		// Reactivate the dependency
		err = dependencyService.ReactivateDependency(ctx, dep.ID, author)
		require.NoError(t, err)

		// Verify it's active again
		reactivatedDep, err := dependencyRepo.GetByID(ctx, dep.ID)
		require.NoError(t, err)
		assert.True(t, reactivatedDep.IsActive())
	})

	t.Run("Auto-resolve Dependencies", func(t *testing.T) {
		// Auto-resolve dependencies when issue3 (database) is completed
		err = dependencyService.AutoResolveDependencies(ctx, issue3.ID, author)
		require.NoError(t, err)

		// Check that dependencies involving issue3 as target are resolved
		deps, err := dependencyService.GetDependencies(ctx, repositories.DependencyFilter{})
		require.NoError(t, err)

		for _, dep := range deps {
			if dep.TargetID == issue3.ID {
				assert.True(t, dep.IsResolved(), "Dependency %s should be resolved", dep.ID)
			}
		}
	})

	t.Run("Impact Analysis", func(t *testing.T) {
		// Recreate active dependencies for impact analysis
		_, err := dependencyService.CreateDependency(
			ctx,
			issue1.ID,
			issue3.ID,
			entities.DependencyTypeRequires,
			"Backend needs database",
			author,
		)
		require.NoError(t, err)

		analysis, err := dependencyService.AnalyzeDependencyImpact(ctx, issue3.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, analysis.AffectedIssues)
		assert.NotEmpty(t, analysis.RiskLevel)
		assert.NotEmpty(t, analysis.Recommendations)
	})

	t.Run("Dependency Statistics", func(t *testing.T) {
		stats, err := dependencyService.GetDependencyStats(ctx, repositories.DependencyFilter{})
		require.NoError(t, err)
		assert.Greater(t, stats.TotalDependencies, 0)
		assert.NotEmpty(t, stats.DependenciesByType)
		assert.NotEmpty(t, stats.DependenciesByStatus)
	})

	t.Run("Remove Dependency", func(t *testing.T) {
		// Create a dependency to remove
		dep, err := dependencyService.CreateDependency(
			ctx,
			issue1.ID,
			issue2.ID,
			entities.DependencyTypeBlocks,
			"Temporary dependency",
			author,
		)
		require.NoError(t, err)

		// Remove it
		err = dependencyService.RemoveDependency(ctx, dep.ID, author)
		require.NoError(t, err)

		// Verify it's gone
		_, err = dependencyRepo.GetByID(ctx, dep.ID)
		assert.Error(t, err)
	})
}

func TestDependencyRepositoryPersistence(t *testing.T) {
	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "issuemap-dependency-persistence-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dependencyRepo := storage.NewFileDependencyRepository(tempDir)
	ctx := context.Background()

	t.Run("CRUD Operations", func(t *testing.T) {
		// Create dependency
		dep := entities.NewDependency(
			"ISSUE-001",
			"ISSUE-002",
			entities.DependencyTypeBlocks,
			"Test dependency",
			"test.user",
		)

		err := dependencyRepo.Create(ctx, dep)
		require.NoError(t, err)

		// Read dependency
		retrieved, err := dependencyRepo.GetByID(ctx, dep.ID)
		require.NoError(t, err)
		assert.Equal(t, dep.ID, retrieved.ID)
		assert.Equal(t, dep.SourceID, retrieved.SourceID)
		assert.Equal(t, dep.TargetID, retrieved.TargetID)
		assert.Equal(t, dep.Type, retrieved.Type)

		// Update dependency
		retrieved.Description = "Updated description"
		err = dependencyRepo.Update(ctx, retrieved)
		require.NoError(t, err)

		// Verify update
		updated, err := dependencyRepo.GetByID(ctx, dep.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", updated.Description)

		// Delete dependency
		err = dependencyRepo.Delete(ctx, dep.ID)
		require.NoError(t, err)

		// Verify deletion
		_, err = dependencyRepo.GetByID(ctx, dep.ID)
		assert.Error(t, err)
	})

	t.Run("Filtering and Querying", func(t *testing.T) {
		// Create multiple dependencies
		deps := []*entities.Dependency{
			entities.NewDependency("ISSUE-001", "ISSUE-002", entities.DependencyTypeBlocks, "Dep 1", "user1"),
			entities.NewDependency("ISSUE-001", "ISSUE-003", entities.DependencyTypeRequires, "Dep 2", "user1"),
			entities.NewDependency("ISSUE-004", "ISSUE-002", entities.DependencyTypeBlocks, "Dep 3", "user2"),
		}

		for _, dep := range deps {
			err := dependencyRepo.Create(ctx, dep)
			require.NoError(t, err)
		}

		// Test filtering by source
		sourceID := entities.IssueID("ISSUE-001")
		sourceFilter := repositories.DependencyFilter{
			SourceID: &sourceID,
		}
		sourceDeps, err := dependencyRepo.List(ctx, sourceFilter)
		require.NoError(t, err)
		assert.Len(t, sourceDeps, 2)

		// Test filtering by target
		targetID := entities.IssueID("ISSUE-002")
		targetFilter := repositories.DependencyFilter{
			TargetID: &targetID,
		}
		targetDeps, err := dependencyRepo.List(ctx, targetFilter)
		require.NoError(t, err)
		assert.Len(t, targetDeps, 2)

		// Test filtering by type
		blocksType := entities.DependencyTypeBlocks
		typeFilter := repositories.DependencyFilter{
			Type: &blocksType,
		}
		typeDeps, err := dependencyRepo.List(ctx, typeFilter)
		require.NoError(t, err)
		assert.Len(t, typeDeps, 2)

		// Test filtering by creator
		user1 := "user1"
		creatorFilter := repositories.DependencyFilter{
			CreatedBy: &user1,
		}
		creatorDeps, err := dependencyRepo.List(ctx, creatorFilter)
		require.NoError(t, err)
		assert.Len(t, creatorDeps, 2)

		// Test GetBySourceID
		sourceIDDeps, err := dependencyRepo.GetBySourceID(ctx, "ISSUE-001")
		require.NoError(t, err)
		assert.Len(t, sourceIDDeps, 2)

		// Test GetByTargetID
		targetIDDeps, err := dependencyRepo.GetByTargetID(ctx, "ISSUE-002")
		require.NoError(t, err)
		assert.Len(t, targetIDDeps, 2)

		// Test GetByIssueID (both source and target)
		issueDeps, err := dependencyRepo.GetByIssueID(ctx, "ISSUE-002")
		require.NoError(t, err)
		assert.Len(t, issueDeps, 2)
	})

	t.Run("Dependency Graph Building", func(t *testing.T) {
		graph, err := dependencyRepo.GetDependencyGraph(ctx)
		require.NoError(t, err)
		assert.Len(t, graph.Dependencies, 3)
		assert.NotEmpty(t, graph.SourceIndex)
		assert.NotEmpty(t, graph.TargetIndex)
	})

	t.Run("Statistics", func(t *testing.T) {
		stats, err := dependencyRepo.GetStats(ctx, repositories.DependencyFilter{})
		require.NoError(t, err)
		assert.Equal(t, 3, stats.TotalDependencies)
		assert.Equal(t, 3, stats.ActiveDependencies)
		assert.Equal(t, 0, stats.ResolvedDependencies)
		assert.NotEmpty(t, stats.DependenciesByType)
		assert.NotEmpty(t, stats.DependenciesByStatus)
	})

	t.Run("Bulk Operations", func(t *testing.T) {
		// Get all dependencies
		deps, err := dependencyRepo.List(ctx, repositories.DependencyFilter{})
		require.NoError(t, err)
		require.NotEmpty(t, deps)

		// Resolve all dependencies
		for _, dep := range deps {
			dep.Resolve("bulk.resolver")
		}

		// Bulk update
		err = dependencyRepo.BulkUpdate(ctx, deps)
		require.NoError(t, err)

		// Verify all are resolved
		activeDeps, err := dependencyRepo.GetActiveDependencies(ctx)
		require.NoError(t, err)
		assert.Empty(t, activeDeps)
	})

	t.Run("Delete by Issue ID", func(t *testing.T) {
		// Create a new dependency
		dep := entities.NewDependency("ISSUE-999", "ISSUE-888", entities.DependencyTypeBlocks, "Test", "user")
		err := dependencyRepo.Create(ctx, dep)
		require.NoError(t, err)

		// Delete all dependencies for ISSUE-999
		err = dependencyRepo.DeleteByIssueID(ctx, "ISSUE-999")
		require.NoError(t, err)

		// Verify deletion
		deps, err := dependencyRepo.GetByIssueID(ctx, "ISSUE-999")
		require.NoError(t, err)
		assert.Empty(t, deps)
	})
}
