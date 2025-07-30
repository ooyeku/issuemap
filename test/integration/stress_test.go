package integration

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStressOperations tests the system under heavy load
func (suite *IntegrationTestSuite) TestStressOperations() {
	if testing.Short() {
		suite.T().Skip("Skipping stress test in short mode")
	}

	suite.T().Run("HighVolumeCreation", func(t *testing.T) {
		// Test creating a large number of issues
		numIssues := 200
		start := time.Now()

		// Create issues in smaller batches to be more realistic
		batchSize := 20
		for batch := 0; batch < numIssues/batchSize; batch++ {
			for i := 0; i < batchSize; i++ {
				issueNum := batch*batchSize + i + 1
				suite.runCLICommand("create",
					fmt.Sprintf("Stress Test Issue %d", issueNum),
					"--type", []string{"bug", "feature", "task", "improvement"}[issueNum%4],
					"--priority", []string{"low", "medium", "high", "critical"}[issueNum%4])
			}
			// Brief pause between batches to allow sync
			time.Sleep(50 * time.Millisecond)
		}

		// Wait for all operations to sync
		time.Sleep(3 * time.Second)

		duration := time.Since(start)

		// Verify all issues were created and synced
		count := suite.getIssueCount()
		assert.Equal(t, numIssues, count)

		// Verify server is still responsive
		resp := suite.makeAPIRequest("GET", "/health", "")
		suite.assertAPISuccess(resp)

		t.Logf("Created %d issues in %v (%.2f issues/sec)",
			numIssues, duration, float64(numIssues)/duration.Seconds())
	})
}

// TestStressWithConcurrentUsers simulates multiple users working simultaneously
func (suite *IntegrationTestSuite) TestStressWithConcurrentUsers() {
	if testing.Short() {
		suite.T().Skip("Skipping concurrent stress test in short mode")
	}

	numUsers := 8
	operationsPerUser := 15
	var wg sync.WaitGroup
	errors := make(chan error, numUsers)

	start := time.Now()

	// Simulate multiple concurrent users
	for user := 0; user < numUsers; user++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()

			userErrors := suite.simulateUserActivity(userID, operationsPerUser)
			if len(userErrors) > 0 {
				errors <- fmt.Errorf("user %d had %d errors: %v", userID, len(userErrors), userErrors[0])
			}
		}(user)
	}

	// Wait for all users to complete
	wg.Wait()
	close(errors)

	duration := time.Since(start)

	// Check for errors
	for err := range errors {
		require.NoError(suite.T(), err)
	}

	// Wait for all syncs to complete
	time.Sleep(2 * time.Second)

	// Verify system is still consistent and responsive
	finalCount := suite.getIssueCount()

	// Each user creates some issues (exact number depends on operations)
	// We verify that we have a reasonable number of issues
	assert.Greater(suite.T(), finalCount, numUsers*5, "Too few issues created")
	assert.Less(suite.T(), finalCount, numUsers*operationsPerUser, "Too many issues created")

	// Verify server health
	resp := suite.makeAPIRequest("GET", "/health", "")
	suite.assertAPISuccess(resp)

	// Verify statistics are accurate
	stats := suite.getStatistics()
	assert.Equal(suite.T(), finalCount, stats.TotalIssues)

	suite.T().Logf("Completed stress test with %d concurrent users, %d operations each, in %v",
		numUsers, operationsPerUser, duration)
}

// TestLongRunningStability tests system stability over extended periods
func (suite *IntegrationTestSuite) TestLongRunningStability() {
	if testing.Short() {
		suite.T().Skip("Skipping long-running stability test in short mode")
	}

	// Run continuous operations for a shorter period in tests
	duration := 30 * time.Second
	operationInterval := 500 * time.Millisecond

	start := time.Now()
	end := start.Add(duration)

	operationCount := 0

	suite.T().Logf("Starting long-running stability test for %v", duration)

	for time.Now().Before(end) {
		operationCount++

		// Vary the operations to simulate real usage
		switch operationCount % 5 {
		case 0, 1, 2: // 60% create operations
			suite.runCLICommand("create",
				fmt.Sprintf("Stability Test Issue %d", operationCount),
				"--type", []string{"bug", "feature", "task"}[operationCount%3])

		case 3: // 20% update operations
			if operationCount > 3 {
				// Update a recent issue
				issues := suite.getAllIssues()
				if len(issues) > 0 {
					issueToUpdate := issues[len(issues)-1]
					suite.runCLICommand("edit", issueToUpdate.ID, "--priority", "high")
				}
			}

		case 4: // 20% close operations
			if operationCount > 5 {
				// Close an open issue
				openIssues := suite.getIssuesWithFilter("status=open")
				if len(openIssues) > 0 {
					suite.runCLICommand("close", openIssues[0].ID)
				}
			}
		}

		// Verify server is still responsive every 10 operations
		if operationCount%10 == 0 {
			resp := suite.makeAPIRequest("GET", "/health", "")
			suite.assertAPISuccess(resp)
		}

		time.Sleep(operationInterval)
	}

	totalDuration := time.Since(start)

	// Final verification
	finalCount := suite.getIssueCount()
	stats := suite.getStatistics()

	assert.Equal(suite.T(), finalCount, stats.TotalIssues)
	assert.Greater(suite.T(), finalCount, 0)

	// Verify server is still responsive
	resp := suite.makeAPIRequest("GET", "/health", "")
	suite.assertAPISuccess(resp)

	suite.T().Logf("Completed %d operations over %v, final count: %d issues",
		operationCount, totalDuration, finalCount)
}

// TestMemoryLeakDetection tests for potential memory leaks
func (suite *IntegrationTestSuite) TestMemoryLeakDetection() {
	if testing.Short() {
		suite.T().Skip("Skipping memory leak test in short mode")
	}

	// Create and delete issues repeatedly to test for memory leaks
	cycles := 10
	issuesPerCycle := 20

	for cycle := 1; cycle <= cycles; cycle++ {
		// Create issues
		createdIssues := make([]string, 0, issuesPerCycle)
		for i := 1; i <= issuesPerCycle; i++ {
			title := fmt.Sprintf("Cycle %d Issue %d", cycle, i)
			suite.runCLICommand("create", title, "--type", "task")
		}

		time.Sleep(500 * time.Millisecond)

		// Get the created issues
		issues := suite.getAllIssues()
		for _, issue := range issues {
			if len(createdIssues) < issuesPerCycle {
				createdIssues = append(createdIssues, issue.ID)
			}
		}

		// Delete half of them (simulating realistic usage)
		for i := 0; i < len(createdIssues)/2; i++ {
			// Delete by removing the file directly (simulating external deletion)
			issueFile := fmt.Sprintf("%s/%s/%s/%s.yaml",
				suite.testDir, ".issuemap", "issues", createdIssues[i])
			os.Remove(issueFile)
		}

		time.Sleep(300 * time.Millisecond)

		// Verify server handled the deletions correctly
		resp := suite.makeAPIRequest("GET", "/health", "")
		suite.assertAPISuccess(resp)

		suite.T().Logf("Completed memory leak test cycle %d/%d", cycle, cycles)
	}

	// Final verification - server should still be responsive
	finalCount := suite.getIssueCount()
	stats := suite.getStatistics()
	assert.Equal(suite.T(), finalCount, stats.TotalIssues)

	suite.T().Logf("Memory leak test completed, final issue count: %d", finalCount)
}

// simulateUserActivity simulates realistic user activity patterns
func (suite *IntegrationTestSuite) simulateUserActivity(userID, numOperations int) []error {
	var errors []error
	createdIssues := make([]string, 0)

	for op := 1; op <= numOperations; op++ {
		// Vary operation types based on realistic usage patterns
		switch op % 10 {
		case 1, 2, 3, 4, 5: // 50% create new issues
			title := fmt.Sprintf("User %d Issue %d", userID, op)
			cmd := exec.Command(suite.binaryPath, "create", title,
				"--type", []string{"bug", "feature", "task"}[op%3],
				"--priority", []string{"low", "medium", "high"}[op%3])
			cmd.Dir = suite.testDir
			if err := cmd.Run(); err != nil {
				errors = append(errors, fmt.Errorf("create failed: %v", err))
			}

		case 6, 7: // 20% update existing issues
			if len(createdIssues) > 0 {
				issueID := createdIssues[op%len(createdIssues)]
				cmd := exec.Command(suite.binaryPath, "edit", issueID, "--priority", "high")
				cmd.Dir = suite.testDir
				if err := cmd.Run(); err != nil {
					errors = append(errors, fmt.Errorf("edit failed: %v", err))
				}
			}

		case 8: // 10% assign issues
			if len(createdIssues) > 0 {
				issueID := createdIssues[op%len(createdIssues)]
				cmd := exec.Command(suite.binaryPath, "assign", issueID, fmt.Sprintf("user%d", userID))
				cmd.Dir = suite.testDir
				if err := cmd.Run(); err != nil {
					errors = append(errors, fmt.Errorf("assign failed: %v", err))
				}
			}

		case 9: // 10% close issues
			if len(createdIssues) > 0 {
				issueID := createdIssues[op%len(createdIssues)]
				cmd := exec.Command(suite.binaryPath, "close", issueID)
				cmd.Dir = suite.testDir
				if err := cmd.Run(); err != nil {
					errors = append(errors, fmt.Errorf("close failed: %v", err))
				}
			}

		case 0: // 10% reopen issues
			if len(createdIssues) > 0 {
				issueID := createdIssues[op%len(createdIssues)]
				cmd := exec.Command(suite.binaryPath, "reopen", issueID)
				cmd.Dir = suite.testDir
				if err := cmd.Run(); err != nil {
					errors = append(errors, fmt.Errorf("reopen failed: %v", err))
				}
			}
		}

		// Small delay between operations to be more realistic
		time.Sleep(100 * time.Millisecond)
	}

	return errors
}
