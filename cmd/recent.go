package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var recentLimit int

// recentCmd represents the recent command
var recentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Show recently worked-on issues",
	Long: `Show the most recently worked-on issues with their recent activities.

This command displays the last 5 issues that have had recent activity,
along with a summary of recent actions taken on each issue.

Examples:
  issuemap recent
  ismp recent
  issuemap recent --limit 10`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRecent(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(recentCmd)
	recentCmd.Flags().IntVar(&recentLimit, "limit", 5, "number of recent issues to show")
}

type RecentIssue struct {
	Issue      entities.Issue
	LastAction time.Time
	Actions    []RecentAction
}

type RecentAction struct {
	Type      string
	Author    string
	Timestamp time.Time
	Message   string
}

func runRecent(cmd *cobra.Command, args []string) error {
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
	historyRepo := storage.NewFileHistoryRepository(basePath)
	gitClient, err := git.NewGitClient(repoPath)
	if err != nil {
		printError(fmt.Errorf("failed to initialize git client: %w", err))
		return err
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitClient)
	historyService := services.NewHistoryService(historyRepo, gitClient)

	// Get all issues
	issueFilter := repositories.IssueFilter{}
	issueList, err := issueService.ListIssues(ctx, issueFilter)
	if err != nil {
		printError(fmt.Errorf("failed to list issues: %w", err))
		return err
	}

	if len(issueList.Issues) == 0 {
		printInfo("No issues found.")
		return nil
	}

	// Get recent activity for each issue
	var recentIssues []RecentIssue
	for _, issue := range issueList.Issues {
		// Get history for this issue
		history, err := historyService.GetIssueHistory(ctx, issue.ID)
		if err != nil {
			// Skip issues without history
			continue
		}

		if len(history.Entries) == 0 {
			// Use the issue's updated timestamp if no history
			recentIssues = append(recentIssues, RecentIssue{
				Issue:      issue,
				LastAction: issue.Timestamps.Updated,
				Actions:    []RecentAction{},
			})
			continue
		}

		// Get the most recent entry
		latestEntry := &history.Entries[len(history.Entries)-1]

		// Get recent actions (last 3 entries)
		var actions []RecentAction
		start := len(history.Entries) - 3
		if start < 0 {
			start = 0
		}

		for i := start; i < len(history.Entries); i++ {
			entry := &history.Entries[i]
			action := RecentAction{
				Type:      string(entry.Type),
				Author:    entry.Author,
				Timestamp: entry.Timestamp,
				Message:   entry.Message,
			}
			actions = append(actions, action)
		}

		recentIssues = append(recentIssues, RecentIssue{
			Issue:      issue,
			LastAction: latestEntry.Timestamp,
			Actions:    actions,
		})
	}

	// Sort by last action time (most recent first)
	sort.Slice(recentIssues, func(i, j int) bool {
		return recentIssues[i].LastAction.After(recentIssues[j].LastAction)
	})

	// Limit to requested number
	if len(recentIssues) > recentLimit {
		recentIssues = recentIssues[:recentLimit]
	}

	// Display recent issues
	displayRecentIssues(recentIssues)

	return nil
}

func displayRecentIssues(recentIssues []RecentIssue) {
	if len(recentIssues) == 0 {
		printInfo("No recent activity found.")
		return
	}

	fmt.Printf("Recent Issues (last %d)\n", len(recentIssues))
	fmt.Println("═══════════════════════════════════════════════════════════════")

	for i, recent := range recentIssues {
		issue := recent.Issue

		// Issue header
		fmt.Printf("\n%d. %s - %s\n", i+1, issue.ID, issue.Title)

		// Status and priority
		statusColor := getIssueStatusColor(issue.Status)
		priorityColor := getIssuePriorityColor(issue.Priority)

		if noColor {
			fmt.Printf("   Status: %s | Priority: %s | Updated: %s\n",
				issue.Status, issue.Priority, formatRelativeTime(recent.LastAction))
		} else {
			fmt.Printf("   Status: %s | Priority: %s | Updated: %s\n",
				statusColor(string(issue.Status)),
				priorityColor(string(issue.Priority)),
				formatRelativeTime(recent.LastAction))
		}

		// Recent actions
		if len(recent.Actions) > 0 {
			fmt.Printf("   Recent activity:\n")
			for _, action := range recent.Actions {
				timeAgo := formatRelativeTime(action.Timestamp)
				if noColor {
					fmt.Printf("     • %s by %s (%s)\n", action.Message, action.Author, timeAgo)
				} else {
					fmt.Printf("     • %s by %s (%s)\n",
						action.Message,
						colorValue(action.Author),
						colorMuted(timeAgo))
				}
			}
		}
	}

	fmt.Println()
	printInfo(fmt.Sprintf("Use 'issuemap show <issue-id>' to see full details"))
}

func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else {
		return t.Format("Jan 2, 2006")
	}
}

func getIssueStatusColor(status entities.Status) func(string) string {
	if noColor {
		return func(s string) string { return s }
	}

	switch status {
	case entities.StatusOpen:
		return colorValue // Green-ish
	case entities.StatusInProgress:
		return func(s string) string { return "\033[33m" + s + "\033[0m" } // Yellow
	case entities.StatusReview:
		return func(s string) string { return "\033[36m" + s + "\033[0m" } // Cyan
	case entities.StatusDone:
		return func(s string) string { return "\033[32m" + s + "\033[0m" } // Green
	case entities.StatusClosed:
		return colorMuted // Gray
	default:
		return func(s string) string { return s }
	}
}

func getIssuePriorityColor(priority entities.Priority) func(string) string {
	if noColor {
		return func(s string) string { return s }
	}

	switch priority {
	case entities.PriorityCritical:
		return func(s string) string { return "\033[91m" + s + "\033[0m" } // Bright red
	case entities.PriorityHigh:
		return func(s string) string { return "\033[31m" + s + "\033[0m" } // Red
	case entities.PriorityMedium:
		return func(s string) string { return "\033[33m" + s + "\033[0m" } // Yellow
	case entities.PriorityLow:
		return colorMuted // Gray
	default:
		return func(s string) string { return s }
	}
}

// colorMuted returns muted/gray colored text
func colorMuted(s string) string {
	if noColor {
		return s
	}
	return "\033[90m" + s + "\033[0m" // Gray
}
