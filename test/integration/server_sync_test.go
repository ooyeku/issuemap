package integration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/server"
)

// IntegrationTestSuite provides a comprehensive test suite for CLI-server integration
type IntegrationTestSuite struct {
	suite.Suite
	testDir    string
	binaryPath string
	serverPort int
	server     *server.Server
	httpClient *http.Client
}

// APIResponse represents the standard API response format
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Count   int         `json:"count,omitempty"`
}

// IssueData represents issue data from API responses
type IssueData struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Status      string            `json:"status"`
	Priority    string            `json:"priority"`
	Labels      []string          `json:"labels"`
	Branch      string            `json:"branch"`
	Timestamps  map[string]string `json:"timestamps"`
}

// StatsData represents statistics from API responses
type StatsData struct {
	TotalIssues int            `json:"total_issues"`
	ByStatus    map[string]int `json:"by_status"`
	ByPriority  map[string]int `json:"by_priority"`
	ByType      map[string]int `json:"by_type"`
}

// SetupSuite initializes the test environment
func (suite *IntegrationTestSuite) SetupSuite() {
	// Build the binary first
	suite.buildBinary()

	// Create temporary test directory
	tempDir, err := ioutil.TempDir("", "issuemap_integration_test_")
	require.NoError(suite.T(), err)
	suite.testDir = tempDir

	// Setup HTTP client with timeout
	suite.httpClient = &http.Client{
		Timeout: 5 * time.Second,
	}

	// Initialize git repository
	suite.initGitRepo()

	// Initialize issuemap
	suite.runCLICommand("init", "--name", "Integration Test")
}

// TearDownSuite cleans up the test environment
func (suite *IntegrationTestSuite) TearDownSuite() {
	// Stop server if running
	if suite.server != nil {
		suite.server.Stop()
	}

	// Clean up test directory
	os.RemoveAll(suite.testDir)
}

// SetupTest prepares each individual test
func (suite *IntegrationTestSuite) SetupTest() {
	// Start fresh server for each test
	suite.startServer()

	// Wait for server to be ready
	suite.waitForServer()
}

// TearDownTest cleans up after each test
func (suite *IntegrationTestSuite) TearDownTest() {
	// Stop server
	if suite.server != nil {
		suite.server.Stop()
		suite.server = nil
	}

	// Clean up any test issues
	suite.cleanupIssues()
}

// TestBasicServerStartup tests that the server starts correctly
func (suite *IntegrationTestSuite) TestBasicServerStartup() {
	// Test health endpoint
	resp := suite.makeAPIRequest("GET", "/health", "")
	suite.assertAPISuccess(resp)

	// Test info endpoint
	resp = suite.makeAPIRequest("GET", "/info", "")
	suite.assertAPISuccess(resp)

	var apiResp APIResponse
	err := json.Unmarshal([]byte(resp), &apiResp)
	require.NoError(suite.T(), err)

	data := apiResp.Data.(map[string]interface{})
	assert.Equal(suite.T(), app.AppName, data["name"])
	assert.Equal(suite.T(), app.GetVersion(), data["version"])
}

// TestCLIIssueCreationSync tests that CLI-created issues appear in server memory
func (suite *IntegrationTestSuite) TestCLIIssueCreationSync() {
	// Initial state - should be empty
	initialCount := suite.getIssueCount()
	assert.Equal(suite.T(), 0, initialCount)

	// Create issue via CLI
	suite.runCLICommand("create", "Test Issue", "--type", "bug", "--priority", "high")

	// Wait for sync
	time.Sleep(200 * time.Millisecond)

	// Verify issue appears in server
	newCount := suite.getIssueCount()
	assert.Equal(suite.T(), 1, newCount)

	// Verify issue details
	issues := suite.getAllIssues()
	require.Len(suite.T(), issues, 1)

	issue := issues[0]
	assert.Equal(suite.T(), "Test Issue", issue.Title)
	assert.Equal(suite.T(), "bug", issue.Type)
	assert.Equal(suite.T(), "high", issue.Priority)
	assert.Equal(suite.T(), "open", issue.Status)
}

// TestCLIIssueUpdateSync tests that CLI updates sync to server memory
func (suite *IntegrationTestSuite) TestCLIIssueUpdateSync() {
	// Create initial issue
	suite.runCLICommand("create", "Original Title", "--type", "feature")
	time.Sleep(200 * time.Millisecond)

	// Get original issue
	issues := suite.getAllIssues()
	require.Len(suite.T(), issues, 1)
	originalIssue := issues[0]
	assert.Equal(suite.T(), "Original Title", originalIssue.Title)

	// Update via CLI
	suite.runCLICommand("edit", originalIssue.ID, "--title", "Updated Title", "--priority", "critical")
	time.Sleep(200 * time.Millisecond)

	// Verify update in server
	updatedIssue := suite.getIssueByID(originalIssue.ID)
	assert.Equal(suite.T(), "Updated Title", updatedIssue.Title)
	assert.Equal(suite.T(), "critical", updatedIssue.Priority)
	assert.Equal(suite.T(), "feature", updatedIssue.Type) // Should remain unchanged
}

// TestMultipleCLIOperationsSync tests concurrent CLI operations
func (suite *IntegrationTestSuite) TestMultipleCLIOperationsSync() {
	// Create multiple issues rapidly
	for i := 1; i <= 5; i++ {
		title := fmt.Sprintf("Issue %d", i)
		issueType := []string{"bug", "feature", "task"}[i%3]
		priority := []string{"low", "medium", "high"}[i%3]

		suite.runCLICommand("create", title, "--type", issueType, "--priority", priority)
	}

	// Wait for all syncs to complete
	time.Sleep(500 * time.Millisecond)

	// Verify all issues are in server
	assert.Equal(suite.T(), 5, suite.getIssueCount())

	// Verify statistics are correct
	stats := suite.getStatistics()
	assert.Equal(suite.T(), 5, stats.TotalIssues)
	assert.Equal(suite.T(), 5, stats.ByStatus["open"])

	// Verify type distribution
	expectedTypes := map[string]int{"bug": 2, "feature": 2, "task": 1}
	for issueType, expectedCount := range expectedTypes {
		assert.Equal(suite.T(), expectedCount, stats.ByType[issueType],
			"Expected %d issues of type %s", expectedCount, issueType)
	}
}

// TestCLIIssueLifecycleSync tests complete issue lifecycle via CLI
func (suite *IntegrationTestSuite) TestCLIIssueLifecycleSync() {
	// Create issue
	suite.runCLICommand("create", "Lifecycle Test", "--type", "bug", "--priority", "high")
	time.Sleep(200 * time.Millisecond)

	issues := suite.getAllIssues()
	require.Len(suite.T(), issues, 1)
	issueID := issues[0].ID

	// Assign issue
	suite.runCLICommand("assign", issueID, "testuser")
	time.Sleep(200 * time.Millisecond)

	// Close issue
	suite.runCLICommand("close", issueID, "--reason", "Fixed in testing")
	time.Sleep(200 * time.Millisecond)

	// Verify final state
	closedIssue := suite.getIssueByID(issueID)
	assert.Equal(suite.T(), "closed", closedIssue.Status)

	// Verify statistics reflect closure
	stats := suite.getStatistics()
	assert.Equal(suite.T(), 1, stats.ByStatus["closed"])
	assert.Equal(suite.T(), 0, stats.ByStatus["open"])

	// Reopen issue
	suite.runCLICommand("reopen", issueID)
	time.Sleep(200 * time.Millisecond)

	// Verify reopened state
	reopenedIssue := suite.getIssueByID(issueID)
	assert.Equal(suite.T(), "open", reopenedIssue.Status)
}

// TestAPIFilteringWithCLIData tests API filtering on CLI-created data
func (suite *IntegrationTestSuite) TestAPIFilteringWithCLIData() {
	// Create diverse set of issues via CLI
	testCases := []struct {
		title     string
		issueType string
		priority  string
		status    string
	}{
		{"Bug 1", "bug", "high", "open"},
		{"Bug 2", "bug", "low", "open"},
		{"Feature 1", "feature", "medium", "open"},
		{"Task 1", "task", "high", "open"},
		{"Task 2", "task", "low", "open"},
	}

	for _, tc := range testCases {
		suite.runCLICommand("create", tc.title, "--type", tc.issueType, "--priority", tc.priority)
	}

	time.Sleep(500 * time.Millisecond)

	// Test filtering by type
	bugIssues := suite.getIssuesWithFilter("type=bug")
	assert.Len(suite.T(), bugIssues, 2)

	featureIssues := suite.getIssuesWithFilter("type=feature")
	assert.Len(suite.T(), featureIssues, 1)

	taskIssues := suite.getIssuesWithFilter("type=task")
	assert.Len(suite.T(), taskIssues, 2)

	// Test filtering by priority
	highPriorityIssues := suite.getIssuesWithFilter("priority=high")
	assert.Len(suite.T(), highPriorityIssues, 2)

	lowPriorityIssues := suite.getIssuesWithFilter("priority=low")
	assert.Len(suite.T(), lowPriorityIssues, 2)

	// Test filtering by status
	openIssues := suite.getIssuesWithFilter("status=open")
	assert.Len(suite.T(), openIssues, 5)
}

// TestServerMemoryConsistency tests that server memory stays consistent
func (suite *IntegrationTestSuite) TestServerMemoryConsistency() {
	// Create initial set of issues
	for i := 1; i <= 3; i++ {
		suite.runCLICommand("create", fmt.Sprintf("Issue %d", i), "--type", "task")
	}
	time.Sleep(300 * time.Millisecond)

	// Get initial state
	initialIssues := suite.getAllIssues()
	initialStats := suite.getStatistics()

	assert.Len(suite.T(), initialIssues, 3)
	assert.Equal(suite.T(), 3, initialStats.TotalIssues)

	// Perform various operations
	suite.runCLICommand("edit", initialIssues[0].ID, "--priority", "high")
	suite.runCLICommand("close", initialIssues[1].ID)
	suite.runCLICommand("create", "New Issue", "--type", "bug")

	time.Sleep(400 * time.Millisecond)

	// Verify consistency
	finalIssues := suite.getAllIssues()
	finalStats := suite.getStatistics()

	assert.Len(suite.T(), finalIssues, 4) // 3 original + 1 new
	assert.Equal(suite.T(), 4, finalStats.TotalIssues)
	assert.Equal(suite.T(), 3, finalStats.ByStatus["open"])   // 2 original + 1 new
	assert.Equal(suite.T(), 1, finalStats.ByStatus["closed"]) // 1 closed

	// Verify specific issue states
	updatedIssue := suite.getIssueByID(initialIssues[0].ID)
	assert.Equal(suite.T(), "high", updatedIssue.Priority)

	closedIssue := suite.getIssueByID(initialIssues[1].ID)
	assert.Equal(suite.T(), "closed", closedIssue.Status)
}

// TestErrorHandlingAndRecovery tests error scenarios
func (suite *IntegrationTestSuite) TestErrorHandlingAndRecovery() {
	// Create a valid issue first
	suite.runCLICommand("create", "Valid Issue", "--type", "bug")
	time.Sleep(200 * time.Millisecond)

	initialCount := suite.getIssueCount()
	assert.Equal(suite.T(), 1, initialCount)

	// Try invalid operations (these should not crash the server)
	suite.runCLICommandIgnoreError("edit", "INVALID-ID", "--title", "Should Fail")
	suite.runCLICommandIgnoreError("close", "NONEXISTENT-001")

	time.Sleep(200 * time.Millisecond)

	// Server should still be responsive
	resp := suite.makeAPIRequest("GET", "/health", "")
	suite.assertAPISuccess(resp)

	// Valid issue should still exist
	finalCount := suite.getIssueCount()
	assert.Equal(suite.T(), 1, finalCount)
}

// Helper methods

func (suite *IntegrationTestSuite) buildBinary() {
	// Build the issuemap binary
	cmd := exec.Command("go", "build", "-o", "issuemap", ".")
	cmd.Dir = filepath.Join("..", "..")
	err := cmd.Run()
	require.NoError(suite.T(), err, "Failed to build issuemap binary")

	suite.binaryPath = filepath.Join("..", "..", "issuemap")
}

func (suite *IntegrationTestSuite) initGitRepo() {
	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = suite.testDir
	err := cmd.Run()
	require.NoError(suite.T(), err)

	// Configure git
	exec.Command("git", "config", "user.name", "Test User").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()

	// Create initial commit
	readmeFile := filepath.Join(suite.testDir, "README.md")
	err = ioutil.WriteFile(readmeFile, []byte("# Integration Test"), 0644)
	require.NoError(suite.T(), err)

	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = suite.testDir
	err = cmd.Run()
	require.NoError(suite.T(), err)

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = suite.testDir
	err = cmd.Run()
	require.NoError(suite.T(), err)
}

func (suite *IntegrationTestSuite) runCLICommand(args ...string) {
	cmd := exec.Command(suite.binaryPath, args...)
	cmd.Dir = suite.testDir
	output, err := cmd.CombinedOutput()
	require.NoError(suite.T(), err, "CLI command failed: %s\nOutput: %s", strings.Join(args, " "), string(output))
}

func (suite *IntegrationTestSuite) runCLICommandIgnoreError(args ...string) {
	cmd := exec.Command(suite.binaryPath, args...)
	cmd.Dir = suite.testDir
	cmd.Run() // Ignore error for negative testing
}

func (suite *IntegrationTestSuite) startServer() {
	issuemapPath := filepath.Join(suite.testDir, app.ConfigDirName)

	srv, err := server.NewServer(issuemapPath)
	require.NoError(suite.T(), err)

	suite.server = srv
	suite.serverPort = srv.GetPort()

	// Start server in goroutine
	go func() {
		srv.Start()
	}()
}

func (suite *IntegrationTestSuite) waitForServer() {
	// Wait up to 10 seconds for server to be ready
	for i := 0; i < 50; i++ {
		resp, err := suite.httpClient.Get(fmt.Sprintf("http://localhost:%d%s/health", suite.serverPort, app.APIBasePath))
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	suite.T().Fatal("Server failed to start within timeout")
}

func (suite *IntegrationTestSuite) makeAPIRequest(method, endpoint, body string) string {
	url := fmt.Sprintf("http://localhost:%d%s%s", suite.serverPort, app.APIBasePath, endpoint)

	var req *http.Request
	var err error

	if body != "" {
		req, err = http.NewRequest(method, url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	require.NoError(suite.T(), err)

	resp, err := suite.httpClient.Do(req)
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(suite.T(), err)

	return string(responseBody)
}

func (suite *IntegrationTestSuite) assertAPISuccess(response string) {
	var apiResp APIResponse
	err := json.Unmarshal([]byte(response), &apiResp)
	require.NoError(suite.T(), err, "Failed to parse API response: %s", response)
	assert.True(suite.T(), apiResp.Success, "API response not successful: %s", response)
}

func (suite *IntegrationTestSuite) getIssueCount() int {
	resp := suite.makeAPIRequest("GET", "/issues", "")
	suite.assertAPISuccess(resp)

	var apiResp APIResponse
	json.Unmarshal([]byte(resp), &apiResp)
	return apiResp.Count
}

func (suite *IntegrationTestSuite) getAllIssues() []IssueData {
	resp := suite.makeAPIRequest("GET", "/issues", "")
	suite.assertAPISuccess(resp)

	var apiResp APIResponse
	json.Unmarshal([]byte(resp), &apiResp)

	var issues []IssueData
	data, _ := json.Marshal(apiResp.Data)
	json.Unmarshal(data, &issues)

	return issues
}

func (suite *IntegrationTestSuite) getIssueByID(issueID string) IssueData {
	resp := suite.makeAPIRequest("GET", "/issues/"+issueID, "")
	suite.assertAPISuccess(resp)

	var apiResp APIResponse
	json.Unmarshal([]byte(resp), &apiResp)

	var issue IssueData
	data, _ := json.Marshal(apiResp.Data)
	json.Unmarshal(data, &issue)

	return issue
}

func (suite *IntegrationTestSuite) getIssuesWithFilter(filter string) []IssueData {
	resp := suite.makeAPIRequest("GET", "/issues?"+filter, "")
	suite.assertAPISuccess(resp)

	var apiResp APIResponse
	json.Unmarshal([]byte(resp), &apiResp)

	var issues []IssueData
	data, _ := json.Marshal(apiResp.Data)
	json.Unmarshal(data, &issues)

	return issues
}

func (suite *IntegrationTestSuite) getStatistics() StatsData {
	resp := suite.makeAPIRequest("GET", "/stats", "")
	suite.assertAPISuccess(resp)

	var apiResp APIResponse
	json.Unmarshal([]byte(resp), &apiResp)

	var stats StatsData
	data, _ := json.Marshal(apiResp.Data)
	json.Unmarshal(data, &stats)

	return stats
}

func (suite *IntegrationTestSuite) cleanupIssues() {
	// Remove all issue files
	issuesDir := filepath.Join(suite.testDir, app.ConfigDirName, app.IssuesDirName)
	if _, err := os.Stat(issuesDir); err == nil {
		files, _ := ioutil.ReadDir(issuesDir)
		for _, file := range files {
			if strings.HasSuffix(file.Name(), app.IssueFileExtension) {
				os.Remove(filepath.Join(issuesDir, file.Name()))
			}
		}
	}
}

// TestIntegrationSuite runs the integration test suite
func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(IntegrationTestSuite))
}
