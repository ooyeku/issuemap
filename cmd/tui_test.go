package cmd

import (
	"testing"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
)

func TestDefaultWidthFor(t *testing.T) {
	cases := map[string]int{
		"ID": 10, "Title": 30, "Type": 7, "Status": 10,
		"Priority": 8, "Assignee": 12, "Labels": 14, "Updated": 16, "Branch": 16,
		"Other": 12,
	}
	for col, want := range cases {
		if got := defaultWidthFor(col); got != want {
			t.Fatalf("defaultWidthFor(%s) = %d, want %d", col, got, want)
		}
	}
}

func TestValueForColumn(t *testing.T) {
	now := time.Date(2025, 8, 8, 12, 34, 0, 0, time.UTC)
	issue := entities.Issue{
		ID:         "ISSUE-123",
		Title:      "Add login",
		Type:       entities.IssueTypeFeature,
		Status:     entities.StatusOpen,
		Priority:   entities.PriorityHigh,
		Labels:     []entities.Label{{Name: "tui"}, {Name: "goal"}},
		Assignee:   &entities.User{Username: "alice"},
		Branch:     "feature/ISSUE-123-add-login",
		Timestamps: entities.Timestamps{Updated: now},
	}
	if v := valueForColumn(issue, "ID"); v != "ISSUE-123" {
		t.Fatal(v)
	}
	if v := valueForColumn(issue, "Title"); v != "Add login" {
		t.Fatal(v)
	}
	if v := valueForColumn(issue, "Type"); v != "feature" {
		t.Fatal(v)
	}
	if v := valueForColumn(issue, "Status"); v != "open" {
		t.Fatal(v)
	}
	if v := valueForColumn(issue, "Priority"); v != "high" {
		t.Fatal(v)
	}
	if v := valueForColumn(issue, "Assignee"); v != "alice" {
		t.Fatal(v)
	}
	if v := valueForColumn(issue, "Labels"); v != "tui,goal" {
		t.Fatal(v)
	}
	if v := valueForColumn(issue, "Updated"); v != "2025-08-08 12:34" {
		t.Fatal(v)
	}
	if v := valueForColumn(issue, "Branch"); v != "feature/ISSUE-123-add-login" {
		t.Fatal(v)
	}
}
