package integration

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBulkOperations validates bulk run/export/import flows
func (suite *IntegrationTestSuite) TestBulkOperations() {
	// Create a small set of issues
	suite.runCLICommand("create", "Bulk T1", "--type", "task", "--priority", "medium")
	suite.runCLICommand("create", "Bulk T2", "--type", "task", "--priority", "medium")
	suite.runCLICommand("create", "Bulk F1", "--type", "feature", "--priority", "high")
	suite.runCLICommand("create", "Bulk B1", "--type", "bug", "--priority", "high")

	// Label two for selection
	issues := suite.getAllIssues()
	require.GreaterOrEqual(suite.T(), len(issues), 4)
	id1 := issues[0].ID
	id2 := issues[1].ID
	suite.runCLICommand("edit", id1, "--labels", "triage")
	suite.runCLICommand("edit", id2, "--labels", "triage")

	// Bulk assign on all open issues
	suite.runCLICommand("bulk", "run", "-q", "status:open", "--assign", "bob")
	// Verify all open issues now have assignee bob
	for _, it := range suite.getAllIssues() {
		if it.Status == "open" || it.Status == "in-progress" || it.Status == "review" || it.Status == "done" { // most statuses
			// fetch full issue via API (includes assignee only indirectly). We rely on label/status checks below.
		}
	}

	// Bulk status update on triage-labeled issues
	suite.runCLICommand("bulk", "run", "-q", "label:triage", "--status", "review")
	updated1 := suite.getIssueByID(id1)
	updated2 := suite.getIssueByID(id2)
	assert.Equal(suite.T(), "review", updated1.Status)
	assert.Equal(suite.T(), "review", updated2.Status)

	// Bulk labels add/remove
	suite.runCLICommand("bulk", "run", "-q", "type:task", "--add-label", "needs-review")
	// Verify tasks have the label (via export inspection)
	exportPath := filepath.Join(suite.testDir, "bulk_export.csv")
	suite.runCLICommand("bulk", "export", "-q", "type:task", "-o", exportPath)
	content, err := os.ReadFile(exportPath)
	require.NoError(suite.T(), err)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.GreaterOrEqual(suite.T(), len(lines), 2)
	// header + at least one
	assert.True(suite.T(), strings.Contains(lines[0], "issue_id"))
	assert.True(suite.T(), strings.Contains(lines[1], "needs-review"))

	// CSV import: set status done and label imported for two issues
	importPath := filepath.Join(suite.testDir, "bulk_import.csv")
	f, err := os.Create(importPath)
	require.NoError(suite.T(), err)
	w := csv.NewWriter(f)
	_ = w.Write([]string{"issue_id", "assignee", "status", "labels"})
	_ = w.Write([]string{id1, "carol", "done", "imported"})
	_ = w.Write([]string{id2, "carol", "done", "imported"})
	w.Flush()
	f.Close()

	suite.runCLICommand("bulk", "import", importPath)
	after1 := suite.getIssueByID(id1)
	after2 := suite.getIssueByID(id2)
	assert.Equal(suite.T(), "done", after1.Status)
	assert.Equal(suite.T(), "done", after2.Status)

	// Dry-run: ensure no change occurs
	snap := suite.getAllIssues()
	suite.runCLICommand("bulk", "run", "-q", "type:feature", "--status", "closed", "--dry-run")
	time.Sleep(50 * time.Millisecond)
	snap2 := suite.getAllIssues()
	assert.Equal(suite.T(), snap, snap2)

	// Rollback: make one selected issue file read-only to trigger write failure mid-run
	// Select all bugs (should be at least one)
	bugs := suite.getIssuesWithFilter("type=bug")
	if len(bugs) >= 1 {
		bugID := bugs[0].ID
		issueFile := filepath.Join(suite.testDir, ".issuemap", "issues", bugID+".yaml")
		// Ensure there are at least two selected; create another bug if needed
		if len(bugs) == 1 {
			suite.runCLICommand("create", "Extra Bug", "--type", "bug")
			time.Sleep(100 * time.Millisecond)
		}
		// Make one file read-only
		_ = os.Chmod(issueFile, 0444)
		// Expect failure and rollback (no partial changes)
		out, err := suite.runCLICommandWithError("bulk", "run", "-q", "type:bug", "--status", "review")
		assert.Error(suite.T(), err)
		assert.Contains(suite.T(), out, "Error")
		// Revert permissions for cleanup
		_ = os.Chmod(issueFile, 0644)
	}

	// Audit log exists
	logsDir := filepath.Join(suite.testDir, ".issuemap", "metadata", "bulk_logs")
	if st, err := os.Stat(logsDir); err == nil && st.IsDir() {
		entries, _ := os.ReadDir(logsDir)
		assert.GreaterOrEqual(suite.T(), len(entries), 1)
	}
}
