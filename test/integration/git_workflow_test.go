package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGitWorkflowIntegration tests the complete Git workflow integration
func (suite *IntegrationTestSuite) TestGitWorkflowIntegration() {
	suite.T().Run("BranchCreationAndSwitching", func(t *testing.T) {
		// Create an issue first
		suite.runCLICommand("create", "Test Git Integration", "--type", "feature", "--priority", "medium")

		// Wait for issue to be created
		time.Sleep(200 * time.Millisecond)

		// Get the created issue ID
		issues := suite.getAllIssues()
		require.Len(t, issues, 1)
		issueID := issues[0].ID

		// Create a branch for the issue
		suite.runCLICommand("branch", issueID)

		// Verify branch was created
		currentBranch := suite.getCurrentGitBranch()
		assert.Contains(t, currentBranch, issueID)

		// Verify issue has branch reference
		updatedIssue := suite.getIssueByID(issueID)
		assert.Equal(t, currentBranch, updatedIssue.Branch)
	})

	suite.T().Run("CommitLinking", func(t *testing.T) {
		// Create test file and commit with issue reference
		testFile := filepath.Join(suite.testDir, "test-feature.txt")
		err := os.WriteFile(testFile, []byte("Test feature implementation"), 0644)
		require.NoError(t, err)

		// Stage and commit
		suite.runGitCommand("add", "test-feature.txt")

		// Get current issue
		issues := suite.getAllIssues()
		require.Len(t, issues, 1)
		issueID := issues[0].ID

		// Commit with issue reference
		commitMsg := fmt.Sprintf("Implement test feature for %s", issueID)
		suite.runGitCommand("commit", "-m", commitMsg)

		// Verify commit references issue
		output := suite.runGitCommandWithOutput("log", "--oneline", "-1")
		assert.Contains(t, output, issueID)
	})

	suite.T().Run("BranchSynchronization", func(t *testing.T) {
		// Test the sync command
		suite.runCLICommand("sync")

		// Test sync status
		output := suite.runCLICommandWithOutput("sync", "status")
		assert.Contains(t, output, "Branch:")
		assert.Contains(t, output, "Associated issue:")
	})

	suite.T().Run("ConflictDetectionAndResolution", func(t *testing.T) {
		// Close the issue to create a conflict (closed issue with open branch)
		issues := suite.getAllIssues()
		require.Len(t, issues, 1)
		issueID := issues[0].ID

		suite.runCLICommand("close", issueID, "--reason", "Testing conflict detection")

		// Run resolve to detect conflicts
		output := suite.runCLICommandWithOutput("resolve")
		assert.Contains(t, output, "conflict")
		assert.Contains(t, output, "closed but branch")

		// Test dry run
		output = suite.runCLICommandWithOutput("resolve", "--dry-run")
		assert.Contains(t, output, "Dry run")
	})

	suite.T().Run("MergeWorkflow", func(t *testing.T) {
		// Switch back to main branch for merge
		suite.runGitCommand("checkout", "main")

		// Get the feature branch name
		branches := suite.getGitBranches()
		var featureBranch string
		for _, branch := range branches {
			if branch != "main" && branch != "master" {
				featureBranch = branch
				break
			}
		}
		require.NotEmpty(t, featureBranch, "Should have a feature branch")

		// Merge the feature branch
		suite.runGitCommand("merge", featureBranch, "--no-ff")

		// Test the merge command functionality (pass issue ID instead of branch name)
		// Detect associated issue from current branch
		issues := suite.getAllIssues()
		require.NotEmpty(t, issues)
		issueID := issues[0].ID
		suite.runCLICommand("merge", issueID)
	})
}

// TestBranchStatusIntegration tests branch status tracking
func (suite *IntegrationTestSuite) TestBranchStatusIntegration() {
	suite.T().Run("BranchStatusTracking", func(t *testing.T) {
		// Create a new issue and branch
		suite.runCLICommand("create", "Status Tracking Test", "--type", "task")
		// Allow fs sync to pick up the new file
		time.Sleep(300 * time.Millisecond)

		issues := suite.getAllIssues()
		require.Len(t, issues, 1)
		newIssue := issues[0]

		// Create branch for the issue
		suite.runCLICommand("branch", newIssue.ID)

		// Add some commits to the branch
		testFile := filepath.Join(suite.testDir, "status-test.txt")
		err := os.WriteFile(testFile, []byte("Initial content"), 0644)
		require.NoError(t, err)

		suite.runGitCommand("add", "status-test.txt")
		suite.runGitCommand("commit", "-m", fmt.Sprintf("Initial commit for %s", newIssue.ID))

		// Update file and commit again
		err = os.WriteFile(testFile, []byte("Updated content"), 0644)
		require.NoError(t, err)

		suite.runGitCommand("add", "status-test.txt")
		suite.runGitCommand("commit", "-m", fmt.Sprintf("Update for %s", newIssue.ID))

		// Test sync with auto-update
		suite.runCLICommand("sync", "--auto-update")
		// Give the server a moment to reload from disk after sync
		time.Sleep(300 * time.Millisecond)

		// Verify issue status was updated
		updatedIssue := suite.getIssueByID(newIssue.ID)
		// The auto-update should have changed status from open to in-progress
		// (this depends on the implementation of auto-update logic)
		assert.NotEqual(t, "open", updatedIssue.Status)
	})

	suite.T().Run("MultipleIssuesBranchConflict", func(t *testing.T) {
		// Create two issues
		suite.runCLICommand("create", "First Issue", "--type", "bug")
		suite.runCLICommand("create", "Second Issue", "--type", "bug")
		time.Sleep(300 * time.Millisecond)

		issues := suite.getAllIssues()
		require.GreaterOrEqual(t, len(issues), 2)

		// Find the new issues
		var firstIssue, secondIssue IssueData
		for _, issue := range issues {
			if issue.Title == "First Issue" {
				firstIssue = issue
			} else if issue.Title == "Second Issue" {
				secondIssue = issue
			}
		}

		// Create a branch and manually assign it to both issues
		branchName := "test-conflict-branch"
		suite.runGitCommand("checkout", "-b", branchName)

		// Manually update both issues to reference the same branch
		suite.runCLICommand("edit", firstIssue.ID, "--branch", branchName)
		suite.runCLICommand("edit", secondIssue.ID, "--branch", branchName)
		time.Sleep(300 * time.Millisecond)

		// Run conflict detection
		output := suite.runCLICommandWithOutput("resolve")
		assert.Contains(t, output, "Multiple issues")
		assert.Contains(t, output, branchName)
	})
}

// TestGitHooksIntegration tests Git hooks functionality
func (suite *IntegrationTestSuite) TestGitHooksIntegration() {
	suite.T().Skip("Git hooks testing requires more complex setup - placeholder for future implementation")
	// This would test:
	// - Hook installation and removal
	// - Automatic issue linking in commits
	// - Commit message validation
	// - Branch naming enforcement
}

// TestAdvancedGitIntegration tests advanced Git features
func (suite *IntegrationTestSuite) TestAdvancedGitIntegration() {
	suite.T().Run("BranchNamingConventions", func(t *testing.T) {
		// Test different branch naming patterns
		suite.runCLICommand("create", "Naming Test", "--type", "improvement")
		time.Sleep(100 * time.Millisecond)

		issues := suite.getAllIssues()
		var testIssue IssueData
		for _, issue := range issues {
			if issue.Title == "Naming Test" {
				testIssue = issue
				break
			}
		}
		require.NotEmpty(t, testIssue.ID)

		// Create branch with custom prefix
		suite.runCLICommand("branch", testIssue.ID, "--prefix", "improvement")

		currentBranch := suite.getCurrentGitBranch()
		assert.Contains(t, currentBranch, "improvement")
		assert.Contains(t, currentBranch, testIssue.ID)
	})

	suite.T().Run("IssueFromBranchDetection", func(t *testing.T) {
		// Create a branch with issue-like name
		branchName := "feature/TEST-999-sample-feature"
		suite.runGitCommand("checkout", "-b", branchName)

		// Run sync to detect issue from branch name
		output := suite.runCLICommandWithOutput("sync", "status")
		assert.Contains(t, output, "TEST-999")
	})
}

// Helper methods for Git operations

func (suite *IntegrationTestSuite) runGitCommand(args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = suite.testDir
	err := cmd.Run()
	require.NoError(suite.T(), err, "Git command failed: %s", strings.Join(args, " "))
}

func (suite *IntegrationTestSuite) runGitCommandWithOutput(args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = suite.testDir
	output, err := cmd.CombinedOutput()
	require.NoError(suite.T(), err, "Git command failed: %s\nOutput: %s", strings.Join(args, " "), string(output))
	return string(output)
}

func (suite *IntegrationTestSuite) getCurrentGitBranch() string {
	output := suite.runGitCommandWithOutput("branch", "--show-current")
	return strings.TrimSpace(output)
}

func (suite *IntegrationTestSuite) getGitBranches() []string {
	output := suite.runGitCommandWithOutput("branch")
	lines := strings.Split(output, "\n")
	var branches []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Remove the * prefix for current branch
		if strings.HasPrefix(line, "* ") {
			line = line[2:]
		}
		branches = append(branches, line)
	}

	return branches
}

func (suite *IntegrationTestSuite) createGitCommit(message string) {
	// Create a test file
	testFile := filepath.Join(suite.testDir, fmt.Sprintf("test-%d.txt", time.Now().Unix()))
	err := os.WriteFile(testFile, []byte("Test content"), 0644)
	require.NoError(suite.T(), err)

	// Stage and commit
	suite.runGitCommand("add", filepath.Base(testFile))
	suite.runGitCommand("commit", "-m", message)
}
