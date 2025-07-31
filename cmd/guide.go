package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// guideCmd represents the guide command
var guideCmd = &cobra.Command{
	Use:   "guide",
	Short: "Show a comprehensive guide to using issuemap",
	Long: `Display an elegant, step-by-step guide demonstrating how to use issuemap
in a typical development workflow. This guide covers everything from initialization
to advanced issue management and Git integration.`,
	Run: func(cmd *cobra.Command, args []string) {
		showGuide()
	},
}

func init() {
	rootCmd.AddCommand(guideCmd)
}

func showGuide() {
	printSection("IssueMap Development Workflow Guide")
	printDivider()

	fmt.Println("Welcome to IssueMap! This guide will walk you through a complete")
	fmt.Println("development workflow using issuemap's powerful issue tracking capabilities.")
	fmt.Println()

	// Step 1: Project Setup
	printStep("1", "Project Setup & Initialization")
	fmt.Println("Start by initializing issuemap in your Git repository:")
	fmt.Println()
	printCommand("cd your-project")
	printCommand("issuemap init --name \"My Awesome Project\"")
	fmt.Println()
	printNote("This creates the .issuemap/ directory structure and installs Git hooks")
	fmt.Println()

	// Step 2: Creating Your First Issue
	printStep("2", "Creating Your First Issue")
	fmt.Println("Create issues to track bugs, features, and tasks:")
	fmt.Println()
	printCommand("issuemap create \"Fix login authentication bug\" --type bug --priority high")
	printCommand("issuemap create \"Add dark mode support\" --type feature --priority medium")
	printCommand("issuemap create \"Update documentation\" --type task --priority low")
	fmt.Println()
	printNote("Issues are automatically assigned sequential IDs (ISSUE-001, ISSUE-002, etc.)")
	fmt.Println()

	// Step 3: Issue Management
	printStep("3", "Managing Issues")
	fmt.Println("View and manage your issues:")
	fmt.Println()
	printCommand("issuemap list                    # View all issues")
	printCommand("issuemap list --status open     # Filter by status")
	printCommand("issuemap show ISSUE-001         # View detailed issue info")
	fmt.Println()

	// Step 4: Development Workflow
	printStep("4", "Development Workflow")
	fmt.Println("Assign issues and track progress:")
	fmt.Println()
	printCommand("issuemap assign ISSUE-001 alice               # Assign to team member")
	printCommand("issuemap edit ISSUE-001 --status in-progress  # Update status")
	printCommand("issuemap edit ISSUE-001 --labels urgent,auth  # Add labels")
	fmt.Println()
	printNote("All changes are automatically tracked in the issue history")
	fmt.Println()

	// Step 5: Git Integration
	printStep("5", "Git Integration")
	fmt.Println("Link issues to branches and commits:")
	fmt.Println()
	printCommand("git checkout -b fix/auth-bug")
	printCommand("git commit -m \"Fix auth validation - refs ISSUE-001\"")
	printCommand("git commit -m \"Add tests for auth fix - fixes ISSUE-001\"")
	fmt.Println()
	printNote("References like 'refs ISSUE-001' and 'fixes ISSUE-001' are automatically tracked")
	fmt.Println()

	// Step 6: Closing Issues
	printStep("6", "Closing Issues")
	fmt.Println("Close issues when work is complete:")
	fmt.Println()
	printCommand("issuemap close ISSUE-001 --reason \"Fixed in commit abc123\"")
	fmt.Println()
	printNote("You can reopen issues if needed with: issuemap reopen ISSUE-001")
	fmt.Println()

	// Step 7: Advanced Features
	printStep("7", "Advanced Features")
	fmt.Println("Leverage powerful filtering and search:")
	fmt.Println()
	printCommand("issuemap list --assignee alice --priority high")
	printCommand("issuemap list --labels urgent")
	printCommand("issuemap list --status closed --limit 10")
	fmt.Println()

	// Step 8: History & Analytics
	printStep("8", "History & Analytics")
	fmt.Println("Track changes and analyze project activity:")
	fmt.Println()
	printCommand("issuemap history                        # View all changes")
	printCommand("issuemap history --issue ISSUE-001      # Issue-specific history")
	printCommand("issuemap history --author alice         # Changes by author")
	printCommand("issuemap history --stats                # Project statistics")
	printCommand("issuemap history --detailed             # Detailed field changes")
	fmt.Println()

	// Step 9: Team Collaboration
	printStep("9", "Team Collaboration")
	fmt.Println("Coordinate with your team:")
	fmt.Println()
	printCommand("issuemap list --assignee alice")
	printCommand("issuemap assign ISSUE-002 bob")
	printCommand("issuemap edit ISSUE-002 --milestone \"v1.0.0\"")
	fmt.Println()

	// Step 10: Project Overview
	printStep("10", "Project Overview")
	fmt.Println("Get insights into your project:")
	fmt.Println()
	printCommand("issuemap list --status open            # Open work")
	printCommand("issuemap list --status closed          # Completed work")
	printCommand("issuemap history --recent --limit 20   # Recent activity")
	printCommand("issuemap history --stats               # Overall statistics")
	fmt.Println()

	// Step 11: Issue Templates & Automation
	printStep("11", "Issue Templates & Automation")
	fmt.Println("Use templates for consistent issue creation:")
	fmt.Println()
	printCommand("issuemap template list                  # See available templates")
	printCommand("issuemap template show bug              # View template details")
	printCommand("issuemap create --template bug \"Login fails on Chrome\"")
	printCommand("issuemap create --template feature \"Add dark mode support\"")
	fmt.Println()
	printNote("Templates provide structured fields and automatic labeling")
	fmt.Println()

	// Step 12: Advanced Template Management
	printStep("12", "Custom Template Creation")
	fmt.Println("Create and manage custom templates:")
	fmt.Println()
	printCommand("issuemap template create hotfix --type bug --priority critical")
	printCommand("issuemap template create user-story --type feature")
	printCommand("issuemap template validate my-template")
	printCommand("issuemap template delete old-template")
	fmt.Println()
	printNote("Custom templates help standardize issue creation across your team")
	fmt.Println()

	// Step 13: Smart Branch Integration
	printStep("13", "Smart Branch Integration")
	fmt.Println("Streamline Git workflow with automatic branch creation:")
	fmt.Println()
	printCommand("issuemap branch 001                     # Create feature/ISSUE-001-...")
	printCommand("issuemap branch ISSUE-002               # Uses issue type for prefix")
	printCommand("issuemap branch 003 --prefix hotfix    # Custom prefix override")
	printCommand("issuemap branch 004 --no-switch        # Create but don't switch")
	fmt.Println()
	printNote("Branch names are auto-generated from issue ID and title")
	fmt.Println()

	// Step 14: Branch-Issue Workflow
	printStep("14", "Complete Branch-Issue Workflow")
	fmt.Println("Full development workflow integration:")
	fmt.Println()
	printCommand("# 1. Create issue")
	printCommand("issuemap create \"Fix database timeout\" --template bug --priority high")
	fmt.Println()
	printCommand("# 2. Create branch for the issue")
	printCommand("issuemap branch 005  # Creates: bugfix/ISSUE-005-fix-database-timeout")
	fmt.Println()
	printCommand("# 3. Make your changes and commit")
	printCommand("git add . && git commit -m \"Fix connection pooling\"")
	printNote("Commits are automatically linked to ISSUE-005 via Git hooks")
	fmt.Println()
	printCommand("# 4. Complete and auto-close issue")
	printCommand("issuemap merge      # Auto-detects issue from branch name")
	fmt.Println()
	printNote("The issue is automatically closed when you merge")
	fmt.Println()

	// Step 15: Template Field Examples
	printStep("15", "Working with Template Fields")
	fmt.Println("Templates support rich field definitions:")
	fmt.Println()
	printCommand("# Bug template includes:")
	printCommand("#  - Environment (development/staging/production)")
	printCommand("#  - Browser (Chrome/Firefox/Safari/Edge)")
	printCommand("#  - Severity (low/medium/high/critical)")
	printCommand("#  - Reproduction steps (required, min 10 chars)")
	fmt.Println()
	printCommand("# Feature template includes:")
	printCommand("#  - User story (required)")
	printCommand("#  - Acceptance criteria (required)")
	printCommand("#  - Complexity (simple/medium/complex)")
	printCommand("#  - Target release (optional)")
	fmt.Println()
	printNote("Templates validate input and apply automation rules")
	fmt.Println()

	// Step 16: Automation Examples
	printStep("16", "Template Automation in Action")
	fmt.Println("See how automation enhances your workflow:")
	fmt.Println()
	printCommand("# Critical bugs are automatically:")
	printCommand("#  - Assigned to lead developer")
	printCommand("#  - Labeled as 'urgent' and 'critical'")
	printCommand("#  - Status set to 'in-progress'")
	fmt.Println()
	printCommand("# Complex features automatically get:")
	printCommand("#  - 'complex' and 'needs-design' labels")
	printCommand("#  - Default status of 'open'")
	fmt.Println()
	printCommand("# Production issues get 'prod-issue' label")
	fmt.Println()
	printNote("Automation rules reduce manual work and ensure consistency")
	fmt.Println()

	// Pro Tips
	printSection("Pro Tips & Best Practices")
	printDivider()

	printTip("Use descriptive issue titles and add detailed descriptions")
	printTip("Tag issues with relevant labels for easy filtering")
	printTip("Reference issues in commit messages for automatic linking")
	printTip("Use milestones to group related issues for releases")
	printTip("Regularly review project history to track progress")
	printTip("Assign issues to team members for clear ownership")
	printTip("Close issues with descriptive reasons for future reference")
	printTip("Use templates consistently to maintain issue quality")
	printTip("Create custom templates for recurring issue types")
	printTip("Leverage automation rules to reduce manual work")
	printTip("Use 'issuemap branch' for clean Git workflow integration")
	printTip("Let branch names auto-generate for consistency")
	printTip("Use 'issuemap merge' to automatically close completed issues")
	fmt.Println()

	// Command Reference
	printSection("Quick Command Reference")
	printDivider()

	fmt.Printf("%-25s %s\n", "issuemap init", "Initialize project")
	fmt.Printf("%-25s %s\n", "issuemap create", "Create new issue")
	fmt.Printf("%-25s %s\n", "issuemap list", "List issues")
	fmt.Printf("%-25s %s\n", "issuemap show", "Show issue details")
	fmt.Printf("%-25s %s\n", "issuemap edit", "Edit issue properties")
	fmt.Printf("%-25s %s\n", "issuemap assign", "Assign/unassign issues")
	fmt.Printf("%-25s %s\n", "issuemap close", "Close issues")
	fmt.Printf("%-25s %s\n", "issuemap reopen", "Reopen closed issues")
	fmt.Printf("%-25s %s\n", "issuemap template", "Manage issue templates")
	fmt.Printf("%-25s %s\n", "issuemap branch", "Create Git branch for issue")
	fmt.Printf("%-25s %s\n", "issuemap merge", "Merge branch and close issue")
	fmt.Printf("%-25s %s\n", "issuemap history", "View change history")
	fmt.Printf("%-25s %s\n", "issuemap guide", "Show this guide")
	fmt.Printf("%-25s %s\n", "issuemap --help", "Get help for any command")
	fmt.Println()

	// Footer
	printSection("Ready to Start?")
	printDivider()
	fmt.Println("You're now ready to use issuemap effectively! Start with 'issuemap init'")
	fmt.Println("in your project directory and begin tracking issues like a pro.")
	fmt.Println()
	fmt.Println("For detailed help on any command, use: issuemap [command] --help")
	fmt.Println()
}

func printSection(title string) {
	fmt.Printf("\n=== %s ===\n", strings.ToUpper(title))
}

func printDivider() {
	fmt.Println(strings.Repeat("=", 60))
}

func printStep(number, title string) {
	fmt.Printf("\n[STEP %s] %s\n", number, title)
	fmt.Println(strings.Repeat("-", len(title)+10))
}

func printCommand(cmd string) {
	fmt.Printf("  $ %s\n", cmd)
}

func printNote(note string) {
	fmt.Printf("  Note: %s\n", note)
}

func printTip(tip string) {
	fmt.Printf("  * %s\n", tip)
}
