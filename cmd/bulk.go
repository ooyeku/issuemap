package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
)

var (
	bulkQuery        string
	bulkDryRun       bool
	bulkRollback     bool
	bulkAssignUser   string
	bulkStatusValue  string
	bulkAddLabels    []string
	bulkRemoveLabels []string
	bulkSetLabels    []string
	bulkExportCSV    string
)

// bulkCmd groups bulk operations
var bulkCmd = &cobra.Command{
	Use:   "bulk",
	Short: "Perform bulk operations on issues",
	Long:  "Run bulk updates across many issues selected by a search query.",
}

// bulkRunCmd executes a bulk operation
var bulkRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run bulk operations on query results",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBulk(cmd)
	},
}

// bulkExportCmd exports selected issues to CSV
var bulkExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export selected issues to CSV",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBulkExport(cmd)
	},
}

// bulkImportCmd imports CSV updates and applies them
var bulkImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import CSV updates and apply in bulk",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBulkImport(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(bulkCmd)

	// run
	bulkCmd.AddCommand(bulkRunCmd)
	bulkRunCmd.Flags().StringVarP(&bulkQuery, "query", "q", "", "search query to select issues (required)")
	err := bulkRunCmd.MarkFlagRequired("query")
	if err != nil {
		return
	}
	bulkRunCmd.Flags().BoolVar(&bulkDryRun, "dry-run", false, "simulate without writing changes")
	bulkRunCmd.Flags().BoolVar(&bulkRollback, "rollback", true, "rollback on first failure")
	bulkRunCmd.Flags().StringVar(&bulkAssignUser, "assign", "", "assign all to user (empty to unassign)")
	bulkRunCmd.Flags().StringVar(&bulkStatusValue, "status", "", "set status for all selected issues")
	bulkRunCmd.Flags().StringSliceVar(&bulkAddLabels, "add-label", nil, "label(s) to add (repeatable)")
	bulkRunCmd.Flags().StringSliceVar(&bulkRemoveLabels, "remove-label", nil, "label(s) to remove (repeatable)")
	bulkRunCmd.Flags().StringSliceVar(&bulkSetLabels, "set-labels", nil, "replace labels with given comma-separated list")

	// export
	bulkCmd.AddCommand(bulkExportCmd)
	bulkExportCmd.Flags().StringVarP(&bulkQuery, "query", "q", "", "search query to select issues (required)")
	qerr := bulkExportCmd.MarkFlagRequired("query")
	if qerr != nil {
		return
	}
	bulkExportCmd.Flags().StringVarP(&bulkExportCSV, "output", "o", "", "output CSV file (default stdout)")

	// import
	bulkCmd.AddCommand(bulkImportCmd)
	bulkImportCmd.Flags().BoolVar(&bulkDryRun, "dry-run", false, "simulate without writing changes")
	bulkImportCmd.Flags().BoolVar(&bulkRollback, "rollback", true, "rollback on first failure")
}

func buildBulkServices(ctx context.Context) (*services.BulkService, *services.SearchService, *services.IssueService, string, error) {
	repoPath, err := findGitRoot()
	if err != nil {
		printError(fmt.Errorf("not in a git repository: %w", err))
		return nil, nil, nil, "", err
	}
	issuemapPath := filepath.Join(repoPath, app.ConfigDirName)
	issueRepo := storage.NewFileIssueRepository(issuemapPath)
	configRepo := storage.NewFileConfigRepository(issuemapPath)

	var gitRepo *git.GitClient
	if gitClient, err := git.NewGitClient(repoPath); err == nil {
		gitRepo = gitClient
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitRepo)
	searchService := services.NewSearchService(issueRepo)
	bulkService := services.NewBulkService(issueService, searchService, issuemapPath)
	return bulkService, searchService, issueService, issuemapPath, nil
}

func runBulk(cmd *cobra.Command) error {
	ctx := context.Background()
	bulkService, _, _, _, err := buildBulkServices(ctx)
	if err != nil {
		return err
	}

	printSectionHeader("Bulk Run")
	fmt.Printf("Query: %s\n\n", bulkQuery)

	issues, err := bulkService.SelectIssues(ctx, bulkQuery)
	if err != nil {
		printError(fmt.Errorf("failed to select issues: %w", err))
		return err
	}
	if len(issues) == 0 {
		printWarning("No issues matched the query")
		return nil
	}
	fmt.Printf("Selected issues: %d\n", len(issues))

	opts := services.BulkOptions{DryRun: bulkDryRun, Rollback: bulkRollback, Author: getCurrentUserName()}
	opts.Progress = func(completed, total int, id entities.IssueID, err error) {
		prefix := fmt.Sprintf("[%d/%d] %s ", completed, total, id)
		if err != nil {
			printWarning(prefix + "error: " + err.Error())
		} else {
			printSuccess(prefix + "ok")
		}
	}

	var res *services.BulkResult
	// Priority: set-labels > add/remove labels > status > assign
	switch {
	case len(bulkSetLabels) > 0 || len(bulkAddLabels) > 0 || len(bulkRemoveLabels) > 0:
		res, err = bulkService.BulkLabels(ctx, issues, bulkAddLabels, bulkRemoveLabels, bulkSetLabels, opts)
	case strings.TrimSpace(bulkStatusValue) != "":
		res, err = bulkService.BulkStatus(ctx, issues, bulkStatusValue, opts)
	default:
		// default to assignment (can be empty to unassign)
		res, err = bulkService.BulkAssign(ctx, issues, bulkAssignUser, opts)
	}
	if err != nil {
		printError(fmt.Errorf("bulk operation failed: %w", err))
	}
	printBulkSummary(res)
	return err
}

func runBulkExport(cmd *cobra.Command) error {
	ctx := context.Background()
	bulkService, _, _, _, err := buildBulkServices(ctx)
	if err != nil {
		return err
	}

	printSectionHeader("Bulk Export (CSV)")
	fmt.Printf("Query: %s\n\n", bulkQuery)
	issues, err := bulkService.SelectIssues(ctx, bulkQuery)
	if err != nil {
		printError(fmt.Errorf("failed to select issues: %w", err))
		return err
	}
	if len(issues) == 0 {
		printWarning("No issues matched the query")
		return nil
	}
	if err := bulkService.ExportIssuesCSV(ctx, issues, bulkExportCSV); err != nil {
		printError(fmt.Errorf("failed to export CSV: %w", err))
		return err
	}
	if bulkExportCSV != "" {
		printSuccess(fmt.Sprintf("Exported %d issues to %s", len(issues), bulkExportCSV))
	}
	return nil
}

func runBulkImport(cmd *cobra.Command, filePath string) error {
	ctx := context.Background()
	bulkService, _, _, _, err := buildBulkServices(ctx)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("input file not found: %s", filePath)
	}
	printSectionHeader("Bulk Import (CSV)")
	fmt.Printf("File: %s\n\n", filePath)

	opts := services.BulkOptions{DryRun: bulkDryRun, Rollback: bulkRollback, Author: getCurrentUserName()}
	res, err := bulkService.ImportUpdatesCSV(ctx, filePath, opts)
	if err != nil {
		printError(fmt.Errorf("bulk import failed: %w", err))
	}
	printBulkSummary(res)
	return err
}

func printBulkSummary(res *services.BulkResult) {
	if res == nil {
		fmt.Println("No result")
		return
	}
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Operation: %s\n", res.Operation)
	fmt.Printf("  Total: %d\n", res.Total)
	fmt.Printf("  Succeeded: %d\n", res.Succeeded)
	fmt.Printf("  Failed: %d\n", res.Failed)
	if len(res.FailedIDs) > 0 {
		fmt.Printf("  Failed IDs: %s\n", joinIDs(res.FailedIDs))
	}
	if res.DryRun {
		printInfo("Dry-run: no changes were written")
	}
}

func getCurrentUserName() string {
	// best-effort from git; fall back to env
	repoPath, err := findGitRoot()
	if err == nil {
		if gitClient, err := git.NewGitClient(repoPath); err == nil {
			if user, err := gitClient.GetAuthorInfo(context.Background()); err == nil {
				if user.Username != "" {
					return user.Username
				}
			}
		}
	}
	if v := os.Getenv("USER"); v != "" {
		return v
	}
	return "system"
}

func joinIDs(ids []entities.IssueID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = string(id)
	}
	return strings.Join(parts, ", ")
}
