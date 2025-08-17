/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/spf13/cobra"
)

// Global flags
var (
	verbose bool
	format  string
	noColor bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   getCommandName(),
	Short: app.AppDescription,
	Long: app.AppLongDescription + `

Features:
- Git-native storage - issues are stored as YAML files
- Branch-aware issue tracking  
- Automatic commit linking
- Rich CLI interface with filtering and search
- Template-based issue creation
- No external dependencies`,
	Version: app.GetVersion(),
}

// getCommandName determines the command name based on how the binary was invoked
func getCommandName() string {
	if len(os.Args) > 0 {
		baseName := filepath.Base(os.Args[0])
		// Handle different executable names and aliases
		switch {
		case strings.Contains(baseName, "ismp"):
			return "ismp"
		case strings.Contains(baseName, "issuemap"):
			return "issuemap"
		default:
			// Default to issuemap if we can't determine
			return "issuemap"
		}
	}
	return "issuemap"
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&format, "format", "f", "table", "output format (table, json, yaml)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
}
