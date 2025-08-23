package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var guideSection string

// guideCmd represents the guide command
var guideCmd = &cobra.Command{
	Use:   "guide [section]",
	Short: "Interactive guide to IssueMap commands and workflows",
	Long: `Comprehensive guide to IssueMap commands, workflows, and best practices.

Shows detailed usage patterns, examples, and recommended workflows for effective
issue management with IssueMap.

Sections available:
  basics     - Core commands for daily use
  workflow   - Recommended workflows and processes  
  time       - Time tracking and reporting
  data       - Import/export and data management
  quality    - Issue quality and maintenance
  advanced   - Advanced features and automation
  aliases    - Using the 'ismp' shorthand alias

Examples:
  issuemap guide              # Show full guide
  issuemap guide basics       # Show basic commands
  ismp guide workflow         # Show workflow recommendations`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return showFullGuide()
		}
		return showGuideSection(args[0])
	},
}

func init() {
	rootCmd.AddCommand(guideCmd)
}

func showFullGuide() error {
	fmt.Printf(`
╔══════════════════════════════════════════════════════════════════════════════╗
║                            IssueMap Complete Guide                           ║
╚══════════════════════════════════════════════════════════════════════════════╝

IssueMap is a Git-native issue tracking system that integrates seamlessly with
your development workflow. This guide covers all commands and workflows.

`)

	sections := []string{"basics", "workflow", "time", "data", "quality", "advanced", "aliases"}
	for _, section := range sections {
		showGuideSection(section)
		fmt.Println()
	}

	showFooter()
	return nil
}

func showGuideSection(section string) error {
	switch section {
	case "basics":
		showBasicsSection()
	case "workflow":
		showWorkflowSection()
	case "time":
		showTimeSection()
	case "data":
		showDataSection()
	case "quality":
		showQualitySection()
	case "advanced":
		showAdvancedSection()
	case "aliases":
		showAliasesSection()
	default:
		return fmt.Errorf("unknown section: %s. Available: basics, workflow, time, data, quality, advanced, aliases")
	}
	return nil
}

func showBasicsSection() {
	fmt.Printf(`
┌─ BASICS - Essential Commands ───────────────────────────────────────────────┐

Getting Started:
  issuemap init                          # Initialize in git repository
  issuemap create --title "Fix bug" --type bug --priority high
  issuemap list                          # View all issues
  issuemap show ISSUE-001                # View issue details

Daily Operations:
  issuemap edit ISSUE-001 --status in-progress --assignee username
  issuemap note ISSUE-001 "Made progress on authentication"
  issuemap close ISSUE-001               # Close completed issue
  issuemap assign ISSUE-001 username     # Assign to someone

Finding Issues:
  issuemap list --status open --priority high
  issuemap search "authentication bug"   # Full-text search
  issuemap recent                        # Recently worked issues

Organization:
  issuemap edit ISSUE-001 --labels bug,urgent --milestone v1.0
  issuemap template list                 # View available templates
  issuemap bulk assign --assignee user --filter "status=open"

└──────────────────────────────────────────────────────────────────────────────┘`)
}

func showWorkflowSection() {
	fmt.Printf(`
┌─ WORKFLOW - Recommended Processes ──────────────────────────────────────────┐

Issue Lifecycle:
  1. Create:    issuemap create --title "Feature X" --type feature
  2. Plan:      issuemap edit ISSUE-001 --priority high --milestone v2.0
  3. Start:     issuemap time start ISSUE-001  # Auto-sets to in-progress
  4. Branch:    issuemap branch ISSUE-001      # Creates feature branch
  5. Work:      issuemap note ISSUE-001 "Implemented core logic"
  6. Complete:  issuemap time stop && issuemap close ISSUE-001

Project Management:
  • Use milestones for versions/sprints
  • Apply labels for categorization (bug, enhancement, priority)  
  • Set estimates for planning: issuemap estimate ISSUE-001 4.5h
  • Track dependencies: issuemap depend ISSUE-001 --blocks ISSUE-002

Regular Maintenance:
  issuemap lint --all                    # Check issue quality
  issuemap cleanup --dry-run            # Preview cleanup operations
  issuemap report --type burndown --milestone v1.0  # Track progress

Branch Integration:
  issuemap sync                          # Sync branch status with issues
  issuemap merge ISSUE-001               # Auto-close when merging

└──────────────────────────────────────────────────────────────────────────────┘`)
}

func showTimeSection() {
	fmt.Printf(`
┌─ TIME - Time Tracking & Reporting ──────────────────────────────────────────┐

Time Tracking:
  issuemap time start ISSUE-001 --description "Working on auth"
  issuemap time stop                     # Stop current timer
  issuemap time log ISSUE-001 2.5 --description "Code review"
  issuemap time report --issue ISSUE-001 # View time for issue

Time Reports:
  issuemap report --type time --since 2024-01-01    # Time report since date
  issuemap report --type time --group-by week       # Group by time period
  issuemap report --type velocity --milestone v1.0  # Velocity tracking
  issuemap report --type burndown --milestone v1.0  # Burndown chart data

Quick Tracking:
  ismp time start 001            # Start timer (shorthand)
  ismp time stop                 # Stop timer
  ismp time log 001 1.5          # Quick time log

Planning & Estimation:
  issuemap estimate ISSUE-001 8.0           # Set estimate
  issuemap report --type summary --detailed # See estimate vs actual
  issuemap bulk --filter "type=bug" --set-estimate 4

└──────────────────────────────────────────────────────────────────────────────┘`)
}

func showDataSection() {
	fmt.Printf(`
┌─ DATA - Import/Export & Management ─────────────────────────────────────────┐

Export Data:
  issuemap export --format csv --output issues.csv
  issuemap export --format json --filter "status=open"
  issuemap export --format yaml --filter "milestone=v1.0"

Import Issues:
  issuemap import issues.yaml              # Import from YAML
  issuemap import --dry-run issues.yaml    # Preview import
  issuemap import --prefix PROJ issues.yaml # Add prefix to IDs
  issuemap import --overwrite issues.yaml  # Replace existing

Storage Management:
  issuemap storage                         # View storage usage
  issuemap cleanup --older-than 90d       # Clean old data
  issuemap storage --compress              # Enable compression
  issuemap archives --older-than 180d     # Archive old issues

Archive Operations:
  issuemap archives list                   # View archived issues
  issuemap archives restore ISSUE-001     # Restore from archive
  issuemap archives cleanup --older-than 1y

Bulk Operations:
  issuemap bulk --filter "milestone=v1.0,status=done" --set-status closed
  issuemap bulk --filter "status=closed" --add-labels archived
  issuemap bulk --filter "created_before=2023-01-01" --delete --confirm

└──────────────────────────────────────────────────────────────────────────────┘`)
}

func showQualitySection() {
	fmt.Printf(`
┌─ QUALITY - Issue Quality & Maintenance ─────────────────────────────────────┐

Issue Linting:
  issuemap lint ISSUE-001                # Lint specific issue
  issuemap lint --all                    # Lint all issues  
  issuemap lint --severity warning       # Show warnings and errors only
  issuemap lint --fix ISSUE-001          # Show fix suggestions

Quality Rules:
  • Title: Clear, descriptive, proper length (10-100 chars)
  • Description: Meaningful content with context
  • Assignment: In-progress issues should have assignees  
  • Time: Estimates for planning, track overruns
  • Organization: Labels, milestones for categorization
  • Workflow: Address stale issues, update status regularly

Quality Reports:
  issuemap lint --all --severity error   # Critical quality issues
  issuemap report --type summary         # Overall project health

Maintenance:
  issuemap cleanup --dry-run            # Preview cleanup operations
  issuemap cleanup --older-than 90d     # Clean old closed issues  
  issuemap logs cleanup                  # View cleanup history

Health Monitoring:
  issuemap storage                       # Monitor disk usage
  issuemap depend --validate            # Check dependency integrity
  issuemap template --validate          # Validate templates

└──────────────────────────────────────────────────────────────────────────────┘`)
}

func showAdvancedSection() {
	fmt.Printf(`
┌─ ADVANCED - Power User Features ────────────────────────────────────────────┐

Web Interface:
  issuemap server start --port 4042     # Start web server
  issuemap web                          # Open web UI in browser
  issuemap server status                # Check server status

Dependencies:
  issuemap depend ISSUE-001 --blocks ISSUE-002
  issuemap depend ISSUE-001 --depends-on ISSUE-003  
  issuemap depend --graph               # Visualize dependencies
  issuemap depend --blocked             # Find blocked issues

Advanced Search:
  issuemap search 'type:bug AND priority:high'
  issuemap search 'assignee:username AND created:>2024-01-01'
  issuemap search 'has:estimate AND estimate:>8h'

Templates:
  issuemap template create bug-template # Create issue template
  issuemap template list                # List templates  
  issuemap create --template bug-report # Use template

Automation:
  issuemap bulk --filter "milestone=v1.0,status=done" --set-status closed
  issuemap sync                         # Auto-sync branch/issue status
  issuemap cleanup --schedule daily     # Schedule maintenance

Attachments:
  issuemap attach ISSUE-001 file.pdf --description "Design spec"
  issuemap attach ISSUE-001 --list      # List attachments
  issuemap show ISSUE-001                # View with attachments

└──────────────────────────────────────────────────────────────────────────────┘`)
}

func showAliasesSection() {
	fmt.Printf(`
┌─ ALIASES - Using 'ismp' Shorthand ──────────────────────────────────────────┐

The 'ismp' alias provides shorthand access to all IssueMap functionality:

Quick Commands:
  ismp create "Fix auth bug" --type bug  # Quick issue creation
  ismp list                              # List issues  
  ismp show 001                          # Show issue (auto-normalizes ID)
  ismp note 001 "Fixed the bug"          # Quick note

Time Tracking:
  ismp time start 001                    # Start timer
  ismp time stop                         # Stop timer
  ismp time log 001 2.5                  # Log time

Reports:
  ismp recent                            # Recently worked issues
  ismp report --type time --since last-week     # Time reports
  ismp lint --all                        # Quality check

Search & Filter:
  ismp list --status open --priority high
  ismp search "authentication"           # Quick search
  ismp export --format csv               # Quick export

Pro Tips:
  • All 'issuemap' commands work with 'ismp'
  • Issue IDs auto-normalize: '001' becomes 'ISSUE-001' 
  • Use tab completion for faster workflows
  • Combine with shell aliases for custom shortcuts

Examples:
  alias bugs="ismp list --type bug --status open"
  alias mystuff="ismp list --assignee \$(git config user.name)"
  alias start="ismp time start"

└──────────────────────────────────────────────────────────────────────────────┘`)
}

func showFooter() {
	fmt.Printf(`
┌─ GETTING HELP ───────────────────────────────────────────────────────────────┐

Documentation:
  issuemap [command] --help              # Command-specific help
  issuemap guide [section]               # This guide by section
  issuemap version                       # Version information

Resources:
  • GitHub: https://github.com/ooyeku/issuemap
  • Docs: Use --help flags for detailed documentation
  • Web UI: Start server and browse to http://localhost:4042

Quick Start Reminder:
  1. issuemap init                       # Initialize project
  2. issuemap create --title "My Task"   # Create first issue  
  3. issuemap time start ISSUE-001       # Start working
  4. issuemap note ISSUE-001 "Progress"  # Add notes
  5. issuemap close ISSUE-001            # Complete issue

└──────────────────────────────────────────────────────────────────────────────┘

`)
}
