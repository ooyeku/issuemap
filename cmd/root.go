/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

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
	Use:   "issuemap",
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
