package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewIssueID(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		number      int
		expected    string
	}{
		{
			name:        "simple project name",
			projectName: "myproject",
			number:      1,
			expected:    "MYPROJEC-001", // Truncated to 8 chars
		},
		{
			name:        "project with spaces",
			projectName: "My Project",
			number:      42,
			expected:    "MY_PROJE-042",
		},
		{
			name:        "empty project name",
			projectName: "",
			number:      5,
			expected:    "ISSUE-005",
		},
		{
			name:        "project with special characters",
			projectName: "test-project!@#",
			number:      123,
			expected:    "TEST_PRO-123",
		},
		{
			name:        "numeric project name",
			projectName: "123project",
			number:      7,
			expected:    "PROJ_123-007",
		},
		{
			name:        "very long project name",
			projectName: "verylongprojectname",
			number:      99,
			expected:    "VERYLONG-099",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewIssueID(tt.projectName, tt.number)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestNewLegacyIssueID(t *testing.T) {
	tests := []struct {
		name     string
		number   int
		expected string
	}{
		{
			name:     "single digit",
			number:   1,
			expected: "ISSUE-001",
		},
		{
			name:     "double digit",
			number:   42,
			expected: "ISSUE-042",
		},
		{
			name:     "triple digit",
			number:   123,
			expected: "ISSUE-123",
		},
		{
			name:     "four digit",
			number:   1234,
			expected: "ISSUE-1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewLegacyIssueID(tt.number)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestSanitizeProjectName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty name",
			input:    "",
			expected: "ISSUE",
		},
		{
			name:     "simple name",
			input:    "project",
			expected: "PROJECT",
		},
		{
			name:     "name with spaces",
			input:    "my project",
			expected: "MY_PROJE", // truncated to 8 chars
		},
		{
			name:     "name with special chars",
			input:    "test-project@2024",
			expected: "TEST_PRO", // truncated to 8 chars
		},
		{
			name:     "numeric start",
			input:    "123project",
			expected: "PROJ_123",
		},
		{
			name:     "long name",
			input:    "verylongprojectname",
			expected: "VERYLONG",
		},
		{
			name:     "only special chars",
			input:    "@#$%",
			expected: "PROJ", // Trailing underscores removed
		},
		{
			name:     "trailing underscores",
			input:    "test_",
			expected: "TEST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeProjectName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIssueIDString(t *testing.T) {
	// Test that IssueID can be converted to string
	id := IssueID("TEST-001")
	assert.Equal(t, "TEST-001", string(id))

	// Test that it can be used as a string
	var str string = string(id)
	assert.Equal(t, "TEST-001", str)
}

func TestIssueIDComparison(t *testing.T) {
	// Test that IssueID values can be compared
	id1 := IssueID("TEST-001")
	id2 := IssueID("TEST-001")
	id3 := IssueID("TEST-002")

	assert.Equal(t, id1, id2)
	assert.NotEqual(t, id1, id3)
}
