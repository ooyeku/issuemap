package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	searchQuery   string
	searchSave    string
	searchRun     string
	searchExplain bool
	searchFormat  string
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search issues with advanced query language",
	Long: `Search issues using an advanced query language that supports:

Field-based searches:
  type:bug              - Issues of type 'bug'
  status:open           - Open issues
  priority:high         - High priority issues
  assignee:username     - Issues assigned to user
  branch:feature-123    - Issues linked to branch
  labels:urgent,bug     - Issues with specific labels

Date-based searches:
  created:>2024-01-01   - Created after date
  updated:<7d           - Updated within last 7 days
  created:>=1w          - Created within last week

Boolean operators:
  bug AND priority:high - Issues that are bugs AND high priority
  type:bug OR type:task - Issues that are bugs OR tasks
  NOT status:closed     - Issues that are NOT closed

Text search:
  "login error"         - Exact phrase search
  fix login             - Search in title/description

Sorting and limits:
  sort:created:desc     - Sort by creation date (newest first)
  sort:priority:asc     - Sort by priority (lowest first)
  limit:10              - Limit results to 10 issues

Examples:
  issuemap search "login bug"
  issuemap search type:bug priority:high status:open
  issuemap search "fix login" AND created:>7d
  issuemap search assignee:john OR assignee:jane
  issuemap search NOT status:closed sort:updated:desc limit:5`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			searchQuery = args[0]
		}
		return runSearch(cmd, args)
	},
}

// searchSaveCmd saves a search query
var searchSaveCmd = &cobra.Command{
	Use:   "save <name> <query>",
	Short: "Save a search query for later use",
	Long: `Save a search query with a name for easy reuse.

Examples:
  issuemap search save "my-bugs" "type:bug assignee:me status:open"
  issuemap search save "urgent-items" "priority:critical OR priority:high"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSearchSave(cmd, args)
	},
}

// searchRunCmd runs a saved search query
var searchRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run a saved search query",
	Long: `Execute a previously saved search query.

Examples:
  issuemap search run "my-bugs"
  issuemap search run "urgent-items"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSearchRun(cmd, args)
	},
}

// searchListCmd lists saved searches
var searchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved search queries",
	Long: `Display all saved search queries with their names and query strings.

Examples:
  issuemap search list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSearchList(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.AddCommand(searchSaveCmd)
	searchCmd.AddCommand(searchRunCmd)
	searchCmd.AddCommand(searchListCmd)

	// Search flags
	searchCmd.Flags().StringVar(&searchSave, "save", "", "save this search with a name")
	searchCmd.Flags().BoolVar(&searchExplain, "explain", false, "explain how the query will be executed")
	searchCmd.Flags().StringVarP(&searchFormat, "format", "f", "table", "output format (table, json, yaml)")
}

func runSearch(cmd *cobra.Command, args []string) error {
	if searchQuery == "" && searchRun == "" {
		return fmt.Errorf("search query is required")
	}

	ctx := context.Background()

	// Initialize services
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	issueRepo := storage.NewFileIssueRepository(issuemapPath)
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	searchService := services.NewSearchService(issueRepo)

	// Parse the search query
	parsedQuery, err := searchService.ParseSearchQuery(searchQuery)
	if err != nil {
		printError(fmt.Errorf("failed to parse search query: %w", err))
		return err
	}

	// Explain query if requested
	if searchExplain {
		explainSearchQuery(parsedQuery)
		return nil
	}

	// Execute search
	printSectionHeader("Search Results")
	fmt.Printf("Query: %s\n\n", searchQuery)

	result, err := searchService.ExecuteSearch(ctx, parsedQuery)
	if err != nil {
		printError(fmt.Errorf("search failed: %w", err))
		return err
	}

	// Display results
	if len(result.Issues) == 0 {
		fmt.Printf("No issues found matching your query.\n")
		fmt.Printf("Try:\n")
		fmt.Printf("  • Broadening your search terms\n")
		fmt.Printf("  • Removing some filters\n")
		fmt.Printf("  • Using 'OR' instead of 'AND'\n")
		fmt.Printf("  • Checking spelling of field values\n")
		return nil
	}

	// Format and display results
	switch searchFormat {
	case "table":
		displaySearchResultsTable(result)
	case "json":
		return outputJSON(result)
	case "yaml":
		return outputYAML(result)
	default:
		return fmt.Errorf("unsupported format: %s", searchFormat)
	}

	// Save search if requested
	if searchSave != "" {
		err := saveSearchQuery(ctx, configRepo, searchSave, searchQuery)
		if err != nil {
			printWarning(fmt.Sprintf("Failed to save search: %v", err))
		} else {
			fmt.Printf("\nSearch saved as '%s'\n", searchSave)
		}
	}

	return nil
}

func runSearchSave(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	name := args[0]
	query := args[1]

	// Initialize config repository
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	err = saveSearchQuery(ctx, configRepo, name, query)
	if err != nil {
		printError(fmt.Errorf("failed to save search: %w", err))
		return err
	}

	printSuccess(fmt.Sprintf("Search query saved as '%s'", name))
	fmt.Printf("Query: %s\n", query)
	fmt.Printf("\nRun with: issuemap search run \"%s\"\n", name)

	return nil
}

func runSearchRun(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	name := args[0]

	// Initialize config repository
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	// Load saved search
	query, err := loadSearchQuery(ctx, configRepo, name)
	if err != nil {
		printError(fmt.Errorf("failed to load saved search '%s': %w", name, err))
		return err
	}

	// Execute the saved search
	searchQuery = query
	fmt.Printf("Running saved search '%s': %s\n\n", name, query)
	return runSearch(cmd, []string{query})
}

func runSearchList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize config repository
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return err
	}

	issuemapPath := filepath.Join(repoPath, ".issuemap")
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	// Load saved searches
	searches, err := listSearchQueries(ctx, configRepo)
	if err != nil {
		printError(fmt.Errorf("failed to list saved searches: %w", err))
		return err
	}

	if len(searches) == 0 {
		fmt.Printf("No saved searches found.\n")
		fmt.Printf("Save a search with: issuemap search \"your query\" --save \"name\"\n")
		return nil
	}

	printSectionHeader("Saved Searches")
	fmt.Printf("Found %d saved search(es):\n\n", len(searches))

	for name, query := range searches {
		fmt.Printf("%s\n", name)
		fmt.Printf("   Query: %s\n", query)
		fmt.Printf("   Run: issuemap search run \"%s\"\n\n", name)
	}

	return nil
}

func explainSearchQuery(query *services.SearchQuery) {
	printSectionHeader("Search Query Explanation")

	fmt.Printf("Text Search: ")
	if query.Text != "" {
		fmt.Printf("\"%s\"\n", query.Text)
	} else {
		fmt.Printf("(none)\n")
	}

	fmt.Printf("Filters:\n")
	if len(query.Filters) == 0 {
		fmt.Printf("   (none)\n")
	} else {
		for field, value := range query.Filters {
			fmt.Printf("   %s = %v\n", field, value)
		}
	}

	fmt.Printf("Date Filters:\n")
	if len(query.DateFilters) == 0 {
		fmt.Printf("   (none)\n")
	} else {
		for field, filter := range query.DateFilters {
			if filter.Relative != "" {
				fmt.Printf("   %s %s %s (relative)\n", field, filter.Operator, filter.Relative)
			} else {
				fmt.Printf("   %s %s %s\n", field, filter.Operator, filter.Value.Format("2006-01-02"))
			}
		}
	}

	fmt.Printf("Boolean Operator: %s\n", query.BoolOperator)

	if len(query.Negated) > 0 {
		fmt.Printf("Negated Fields: %s\n", strings.Join(query.Negated, ", "))
	}

	if query.SortBy != "" {
		fmt.Printf("Sort: %s (%s)\n", query.SortBy, query.SortOrder)
	}

	fmt.Printf("Limit: %d\n", query.Limit)
}

func displaySearchResultsTable(result *repositories.SearchResult) {
	fmt.Printf("Found %d issue(s) (in %s):\n\n", result.Total, result.Duration)

	if len(result.Issues) == 0 {
		return
	}

	// Use the same table display as the list command
	displayIssuesTable(result.Issues)
}

// Helper functions for saved searches

func saveSearchQuery(ctx context.Context, configRepo *storage.FileConfigRepository, name, query string) error {
	// Load current config
	config, err := configRepo.Load(ctx)
	if err != nil {
		// Create new config if it doesn't exist
		config = entities.NewDefaultConfig()
	}

	// Initialize searches map if needed
	if config.SavedSearches == nil {
		config.SavedSearches = make(map[string]string)
	}

	// Save the search
	config.SavedSearches[name] = query

	// Save config back
	return configRepo.Save(ctx, config)
}

func loadSearchQuery(ctx context.Context, configRepo *storage.FileConfigRepository, name string) (string, error) {
	config, err := configRepo.Load(ctx)
	if err != nil {
		return "", err
	}

	if config.SavedSearches == nil {
		return "", fmt.Errorf("no saved searches found")
	}

	query, exists := config.SavedSearches[name]
	if !exists {
		return "", fmt.Errorf("saved search '%s' not found", name)
	}

	return query, nil
}

func listSearchQueries(ctx context.Context, configRepo *storage.FileConfigRepository) (map[string]string, error) {
	config, err := configRepo.Load(ctx)
	if err != nil {
		return nil, err
	}

	if config.SavedSearches == nil {
		return make(map[string]string), nil
	}

	return config.SavedSearches, nil
}
