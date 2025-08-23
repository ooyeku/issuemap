package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	lintAll       bool
	lintFix       bool
	lintSeverity  string
	lintRules     []string
	lintSkipRules []string
	lintOutput    string
)

// lintCmd represents the lint command
var lintCmd = &cobra.Command{
	Use:   "lint [issue-id]",
	Short: "Check issues for quality and completeness",
	Long: `Lint issues to check for missing fields, quality standards, and best practices.

Checks include:
- Required fields (title, description, type)
- Title quality (length, format, clarity)
- Description completeness
- Status consistency  
- Priority assignment
- Time tracking completeness
- Label and milestone usage
- Assignee validation

Examples:
  issuemap lint ISSUE-001                    # Lint specific issue
  issuemap lint --all                        # Lint all issues
  ismp lint --severity error                 # Only show errors
  issuemap lint --rules title,description    # Only check specific rules
  issuemap lint --fix ISSUE-001              # Show suggested fixes`,
	Args: func(cmd *cobra.Command, args []string) error {
		if !lintAll && len(args) == 0 {
			return fmt.Errorf("must specify an issue ID or use --all flag")
		}
		if lintAll && len(args) > 0 {
			return fmt.Errorf("cannot specify issue ID when using --all flag")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if lintAll {
			return runLintAll(cmd)
		}
		return runLintIssue(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(lintCmd)
	lintCmd.Flags().BoolVar(&lintAll, "all", false, "lint all issues")
	lintCmd.Flags().BoolVar(&lintFix, "fix", false, "show suggested fixes")
	lintCmd.Flags().StringVar(&lintSeverity, "severity", "all", "minimum severity to show (info, warning, error, all)")
	lintCmd.Flags().StringSliceVar(&lintRules, "rules", []string{}, "specific rules to check (comma-separated)")
	lintCmd.Flags().StringSliceVar(&lintSkipRules, "skip-rules", []string{}, "rules to skip (comma-separated)")
	lintCmd.Flags().StringVarP(&lintOutput, "output", "o", "text", "output format (text, json)")
}

type LintResult struct {
	IssueID    entities.IssueID `json:"issue_id"`
	Title      string           `json:"title"`
	Violations []LintViolation  `json:"violations"`
	Score      int              `json:"score"` // 0-100 quality score
	Grade      string           `json:"grade"` // A, B, C, D, F
}

type LintViolation struct {
	Rule        string `json:"rule"`
	Severity    string `json:"severity"` // info, warning, error
	Message     string `json:"message"`
	Field       string `json:"field"`
	Suggestion  string `json:"suggestion,omitempty"`
	AutoFixable bool   `json:"auto_fixable"`
}

type LintSummary struct {
	TotalIssues          int            `json:"total_issues"`
	IssuesChecked        int            `json:"issues_checked"`
	TotalViolations      int            `json:"total_violations"`
	ViolationsBySeverity map[string]int `json:"violations_by_severity"`
	ViolationsByRule     map[string]int `json:"violations_by_rule"`
	AverageScore         float64        `json:"average_score"`
	Results              []LintResult   `json:"results"`
}

func runLintAll(cmd *cobra.Command) error {
	ctx := context.Background()

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	basePath := filepath.Join(repoPath, ".issuemap")
	issueRepo := storage.NewFileIssueRepository(basePath)
	configRepo := storage.NewFileConfigRepository(basePath)
	gitClient, err := git.NewGitClient(repoPath)
	if err != nil {
		printError(fmt.Errorf("failed to initialize git client: %w", err))
		return err
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitClient)

	// Get all issues
	issueList, err := issueService.ListIssues(ctx, repositories.IssueFilter{})
	if err != nil {
		printError(fmt.Errorf("failed to list issues: %w", err))
		return err
	}

	if len(issueList.Issues) == 0 {
		printInfo("No issues found.")
		return nil
	}

	// Lint all issues
	var results []LintResult
	for _, issue := range issueList.Issues {
		result := lintIssue(&issue)
		if shouldShowResult(result) {
			results = append(results, result)
		}
	}

	// Create summary
	summary := createLintSummary(results, len(issueList.Issues))

	// Display results
	displayLintSummary(summary)

	return nil
}

func runLintIssue(cmd *cobra.Command, issueIDStr string) error {
	ctx := context.Background()
	issueID := normalizeIssueID(issueIDStr)

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	basePath := filepath.Join(repoPath, ".issuemap")
	issueRepo := storage.NewFileIssueRepository(basePath)
	configRepo := storage.NewFileConfigRepository(basePath)
	gitClient, err := git.NewGitClient(repoPath)
	if err != nil {
		printError(fmt.Errorf("failed to initialize git client: %w", err))
		return err
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitClient)

	// Get the issue
	issue, err := issueService.GetIssue(ctx, issueID)
	if err != nil {
		printError(fmt.Errorf("issue %s not found", issueID))
		return err
	}

	// Lint the issue
	result := lintIssue(issue)

	// Display result
	displayLintResult(result)

	return nil
}

func lintIssue(issue *entities.Issue) LintResult {
	result := LintResult{
		IssueID:    issue.ID,
		Title:      issue.Title,
		Violations: []LintViolation{},
	}

	// Define all lint rules
	rules := []func(*entities.Issue) []LintViolation{
		checkTitle,
		checkDescription,
		checkType,
		checkStatus,
		checkPriority,
		checkAssignee,
		checkLabels,
		checkMilestone,
		checkTimeTracking,
		checkBranch,
		checkComments,
		checkAge,
	}

	// Apply rules
	for _, rule := range rules {
		violations := rule(issue)
		for _, violation := range violations {
			if shouldIncludeRule(violation.Rule) && shouldIncludeSeverity(violation.Severity) {
				result.Violations = append(result.Violations, violation)
			}
		}
	}

	// Calculate score and grade
	result.Score = calculateQualityScore(issue, result.Violations)
	result.Grade = calculateGrade(result.Score)

	return result
}

func checkTitle(issue *entities.Issue) []LintViolation {
	var violations []LintViolation

	title := strings.TrimSpace(issue.Title)

	// Required field check
	if title == "" {
		violations = append(violations, LintViolation{
			Rule:        "title-required",
			Severity:    "error",
			Message:     "Title is required",
			Field:       "title",
			Suggestion:  "Add a descriptive title for this issue",
			AutoFixable: false,
		})
		return violations
	}

	// Length checks
	if len(title) < 10 {
		violations = append(violations, LintViolation{
			Rule:        "title-too-short",
			Severity:    "warning",
			Message:     "Title is too short (less than 10 characters)",
			Field:       "title",
			Suggestion:  "Expand the title to be more descriptive",
			AutoFixable: false,
		})
	}

	if len(title) > 100 {
		violations = append(violations, LintViolation{
			Rule:        "title-too-long",
			Severity:    "warning",
			Message:     "Title is too long (more than 100 characters)",
			Field:       "title",
			Suggestion:  "Shorten the title while keeping it descriptive",
			AutoFixable: false,
		})
	}

	// Format checks
	if title[0] != strings.ToUpper(title)[0] {
		violations = append(violations, LintViolation{
			Rule:        "title-capitalization",
			Severity:    "info",
			Message:     "Title should start with a capital letter",
			Field:       "title",
			Suggestion:  fmt.Sprintf("Change to: %s", strings.Title(title)),
			AutoFixable: true,
		})
	}

	// Check for common anti-patterns
	lowTitle := strings.ToLower(title)
	if strings.Contains(lowTitle, "fix") && strings.Contains(lowTitle, "bug") {
		violations = append(violations, LintViolation{
			Rule:        "title-redundant",
			Severity:    "info",
			Message:     "Title contains redundant words like 'fix' and 'bug'",
			Field:       "title",
			Suggestion:  "Consider removing redundant words or using the type field instead",
			AutoFixable: false,
		})
	}

	return violations
}

func checkDescription(issue *entities.Issue) []LintViolation {
	var violations []LintViolation

	description := strings.TrimSpace(issue.Description)

	// Required field check
	if description == "" {
		violations = append(violations, LintViolation{
			Rule:        "description-required",
			Severity:    "warning",
			Message:     "Description is missing",
			Field:       "description",
			Suggestion:  "Add a description explaining what this issue is about",
			AutoFixable: false,
		})
		return violations
	}

	// Length checks
	if len(description) < 20 {
		violations = append(violations, LintViolation{
			Rule:        "description-too-short",
			Severity:    "info",
			Message:     "Description is very short (less than 20 characters)",
			Field:       "description",
			Suggestion:  "Expand the description with more details",
			AutoFixable: false,
		})
	}

	// Quality checks
	if !containsMultipleWords(description, 5) {
		violations = append(violations, LintViolation{
			Rule:        "description-too-simple",
			Severity:    "info",
			Message:     "Description seems too simple (less than 5 words)",
			Field:       "description",
			Suggestion:  "Add more context and details to the description",
			AutoFixable: false,
		})
	}

	return violations
}

func checkType(issue *entities.Issue) []LintViolation {
	var violations []LintViolation

	if issue.Type == "" {
		violations = append(violations, LintViolation{
			Rule:        "type-required",
			Severity:    "error",
			Message:     "Issue type is required",
			Field:       "type",
			Suggestion:  "Set type to one of: bug, feature, task, epic",
			AutoFixable: false,
		})
	}

	return violations
}

func checkStatus(issue *entities.Issue) []LintViolation {
	var violations []LintViolation

	// Check for stale issues
	if issue.Status == entities.StatusInProgress {
		daysSinceUpdate := daysSince(issue.Timestamps.Updated)
		if daysSinceUpdate > 14 {
			violations = append(violations, LintViolation{
				Rule:        "status-stale-in-progress",
				Severity:    "warning",
				Message:     fmt.Sprintf("Issue has been in-progress for %d days", daysSinceUpdate),
				Field:       "status",
				Suggestion:  "Update the status or add recent activity",
				AutoFixable: false,
			})
		}
	}

	return violations
}

func checkPriority(issue *entities.Issue) []LintViolation {
	var violations []LintViolation

	if issue.Priority == "" {
		violations = append(violations, LintViolation{
			Rule:        "priority-missing",
			Severity:    "info",
			Message:     "Priority is not set",
			Field:       "priority",
			Suggestion:  "Set priority to one of: low, medium, high, critical",
			AutoFixable: false,
		})
	}

	return violations
}

func checkAssignee(issue *entities.Issue) []LintViolation {
	var violations []LintViolation

	if issue.Assignee == nil && issue.Status == entities.StatusInProgress {
		violations = append(violations, LintViolation{
			Rule:        "assignee-missing-in-progress",
			Severity:    "warning",
			Message:     "In-progress issue should have an assignee",
			Field:       "assignee",
			Suggestion:  "Assign this issue to someone or change status",
			AutoFixable: false,
		})
	}

	return violations
}

func checkLabels(issue *entities.Issue) []LintViolation {
	var violations []LintViolation

	if len(issue.Labels) == 0 {
		violations = append(violations, LintViolation{
			Rule:        "labels-missing",
			Severity:    "info",
			Message:     "Issue has no labels",
			Field:       "labels",
			Suggestion:  "Add relevant labels for categorization",
			AutoFixable: false,
		})
	}

	return violations
}

func checkMilestone(issue *entities.Issue) []LintViolation {
	var violations []LintViolation

	if issue.Milestone == nil && issue.Priority == entities.PriorityHigh {
		violations = append(violations, LintViolation{
			Rule:        "milestone-missing-high-priority",
			Severity:    "info",
			Message:     "High priority issue should have a milestone",
			Field:       "milestone",
			Suggestion:  "Assign this issue to a milestone",
			AutoFixable: false,
		})
	}

	return violations
}

func checkTimeTracking(issue *entities.Issue) []LintViolation {
	var violations []LintViolation

	hasEstimate := issue.Metadata.EstimatedHours != nil && *issue.Metadata.EstimatedHours > 0
	hasActual := issue.Metadata.ActualHours != nil && *issue.Metadata.ActualHours > 0

	if !hasEstimate && issue.Type != entities.IssueTypeEpic {
		violations = append(violations, LintViolation{
			Rule:        "estimate-missing",
			Severity:    "info",
			Message:     "Issue has no time estimate",
			Field:       "estimated_hours",
			Suggestion:  "Add a time estimate for better planning",
			AutoFixable: false,
		})
	}

	if hasActual && hasEstimate {
		actual := *issue.Metadata.ActualHours
		estimate := *issue.Metadata.EstimatedHours
		if actual > estimate*1.5 {
			violations = append(violations, LintViolation{
				Rule:        "time-overrun",
				Severity:    "warning",
				Message:     fmt.Sprintf("Actual time (%.1fh) significantly exceeds estimate (%.1fh)", actual, estimate),
				Field:       "actual_hours",
				Suggestion:  "Review time estimates for future similar work",
				AutoFixable: false,
			})
		}
	}

	return violations
}

func checkBranch(issue *entities.Issue) []LintViolation {
	var violations []LintViolation

	if issue.Branch == "" && issue.Status == entities.StatusInProgress {
		violations = append(violations, LintViolation{
			Rule:        "branch-missing-in-progress",
			Severity:    "info",
			Message:     "In-progress issue should have an associated branch",
			Field:       "branch",
			Suggestion:  "Create and link a branch for this issue",
			AutoFixable: false,
		})
	}

	return violations
}

func checkComments(issue *entities.Issue) []LintViolation {
	var violations []LintViolation

	daysSinceCreation := daysSince(issue.Timestamps.Created)
	if len(issue.Comments) == 0 && daysSinceCreation > 7 && issue.Status != entities.StatusClosed {
		violations = append(violations, LintViolation{
			Rule:        "comments-missing-old-issue",
			Severity:    "info",
			Message:     fmt.Sprintf("Issue is %d days old with no comments", daysSinceCreation),
			Field:       "comments",
			Suggestion:  "Add status updates or progress comments",
			AutoFixable: false,
		})
	}

	return violations
}

func checkAge(issue *entities.Issue) []LintViolation {
	var violations []LintViolation

	daysSinceCreation := daysSince(issue.Timestamps.Created)
	if daysSinceCreation > 90 && issue.Status == entities.StatusOpen {
		violations = append(violations, LintViolation{
			Rule:        "issue-very-old",
			Severity:    "warning",
			Message:     fmt.Sprintf("Issue has been open for %d days", daysSinceCreation),
			Field:       "status",
			Suggestion:  "Review if this issue is still relevant or close it",
			AutoFixable: false,
		})
	}

	return violations
}

// Helper functions

func shouldShowResult(result LintResult) bool {
	if lintSeverity == "all" {
		return len(result.Violations) > 0
	}

	for _, violation := range result.Violations {
		if shouldIncludeSeverity(violation.Severity) {
			return true
		}
	}
	return false
}

func shouldIncludeRule(rule string) bool {
	if len(lintRules) > 0 {
		return contains(lintRules, rule)
	}
	if len(lintSkipRules) > 0 {
		return !contains(lintSkipRules, rule)
	}
	return true
}

func shouldIncludeSeverity(severity string) bool {
	severityOrder := map[string]int{"info": 1, "warning": 2, "error": 3}
	if lintSeverity == "all" {
		return true
	}
	return severityOrder[severity] >= severityOrder[lintSeverity]
}

func calculateQualityScore(issue *entities.Issue, violations []LintViolation) int {
	score := 100

	for _, violation := range violations {
		switch violation.Severity {
		case "error":
			score -= 15
		case "warning":
			score -= 10
		case "info":
			score -= 5
		}
	}

	if score < 0 {
		score = 0
	}
	return score
}

func calculateGrade(score int) string {
	if score >= 90 {
		return "A"
	} else if score >= 80 {
		return "B"
	} else if score >= 70 {
		return "C"
	} else if score >= 60 {
		return "D"
	}
	return "F"
}

func createLintSummary(results []LintResult, totalIssues int) LintSummary {
	summary := LintSummary{
		TotalIssues:          totalIssues,
		IssuesChecked:        len(results),
		ViolationsBySeverity: make(map[string]int),
		ViolationsByRule:     make(map[string]int),
		Results:              results,
	}

	totalScore := 0
	for _, result := range results {
		totalScore += result.Score
		summary.TotalViolations += len(result.Violations)

		for _, violation := range result.Violations {
			summary.ViolationsBySeverity[violation.Severity]++
			summary.ViolationsByRule[violation.Rule]++
		}
	}

	if len(results) > 0 {
		summary.AverageScore = float64(totalScore) / float64(len(results))
	}

	return summary
}

func displayLintResult(result LintResult) {
	fmt.Printf("Issue: %s - %s\n", result.IssueID, result.Title)
	fmt.Printf("Score: %d/100 (Grade: %s)\n", result.Score, result.Grade)
	fmt.Println(strings.Repeat("─", 50))

	if len(result.Violations) == 0 {
		printSuccess("No violations found! ✨")
		return
	}

	// Group by severity
	violationsBySeverity := make(map[string][]LintViolation)
	for _, violation := range result.Violations {
		violationsBySeverity[violation.Severity] = append(violationsBySeverity[violation.Severity], violation)
	}

	// Display in order: error, warning, info
	severities := []string{"error", "warning", "info"}
	for _, severity := range severities {
		violations := violationsBySeverity[severity]
		if len(violations) == 0 {
			continue
		}

		fmt.Printf("\n%s (%d):\n", strings.ToUpper(severity), len(violations))
		for _, violation := range violations {
			icon := getSeverityIcon(violation.Severity)
			fmt.Printf("  %s [%s] %s\n", icon, violation.Rule, violation.Message)
			if lintFix && violation.Suggestion != "" {
				fmt.Printf("    💡 %s\n", violation.Suggestion)
			}
		}
	}
	fmt.Println()
}

func displayLintSummary(summary LintSummary) {
	fmt.Printf("Lint Summary\n")
	fmt.Println(strings.Repeat("═", 50))

	fmt.Printf("Issues checked: %d/%d\n", summary.IssuesChecked, summary.TotalIssues)
	fmt.Printf("Total violations: %d\n", summary.TotalViolations)
	fmt.Printf("Average score: %.1f/100\n\n", summary.AverageScore)

	// Show violations by severity
	if summary.TotalViolations > 0 {
		fmt.Printf("Violations by severity:\n")
		severities := []string{"error", "warning", "info"}
		for _, severity := range severities {
			count := summary.ViolationsBySeverity[severity]
			if count > 0 {
				icon := getSeverityIcon(severity)
				fmt.Printf("  %s %s: %d\n", icon, severity, count)
			}
		}
		fmt.Println()
	}

	// Show top violation rules
	if len(summary.ViolationsByRule) > 0 {
		fmt.Printf("Most common issues:\n")
		type ruleCount struct {
			rule  string
			count int
		}

		var rules []ruleCount
		for rule, count := range summary.ViolationsByRule {
			rules = append(rules, ruleCount{rule, count})
		}

		sort.Slice(rules, func(i, j int) bool {
			return rules[i].count > rules[j].count
		})

		for i, rule := range rules {
			if i >= 5 { // Show top 5
				break
			}
			fmt.Printf("  • %s: %d issues\n", rule.rule, rule.count)
		}
		fmt.Println()
	}

	// Show issues with violations
	if summary.IssuesChecked > 0 {
		fmt.Printf("Issues with violations:\n")
		for _, result := range summary.Results {
			if len(result.Violations) > 0 {
				fmt.Printf("  %s - Score: %d (%s) - %d violations\n",
					result.IssueID, result.Score, result.Grade, len(result.Violations))
			}
		}
	}
}

func getSeverityIcon(severity string) string {
	if noColor {
		switch severity {
		case "error":
			return "❌"
		case "warning":
			return "⚠️"
		case "info":
			return "ℹ️"
		}
	} else {
		switch severity {
		case "error":
			return "🔴"
		case "warning":
			return "🟡"
		case "info":
			return "🔵"
		}
	}
	return "•"
}

// Utility functions

func containsMultipleWords(text string, minWords int) bool {
	words := strings.Fields(text)
	return len(words) >= minWords
}

func daysSince(t time.Time) int {
	return int(time.Since(t).Hours() / 24)
}

// contains function is defined in import.go
