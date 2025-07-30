package cmd

import (
	"fmt"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/spf13/cobra"
)

var (
	versionShort bool
	versionFull  bool
	versionJSON  bool
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long: `Display version information for IssueMap.

Examples:
  issuemap version          # Show version number
  issuemap version --full   # Show detailed version info
  issuemap version --json   # Show version info as JSON`,
	Run: func(cmd *cobra.Command, args []string) {
		showVersion()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	versionCmd.Flags().BoolVarP(&versionShort, "short", "s", false, "show only version number")
	versionCmd.Flags().BoolVar(&versionFull, "full", false, "show detailed version information")
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "output version information as JSON")
}

func showVersion() {
	if versionJSON {
		showVersionJSON()
	} else if versionFull {
		showVersionFull()
	} else if versionShort {
		showVersionShort()
	} else {
		showVersionDefault()
	}
}

func showVersionShort() {
	fmt.Println(app.GetVersion())
}

func showVersionDefault() {
	fmt.Printf("%s %s\n", app.AppName, app.GetVersion())
}

func showVersionFull() {
	fmt.Println(app.GetFullVersion())
}

func showVersionJSON() {
	versionInfo := app.GetVersionInfo()

	fmt.Println("{")
	first := true
	for key, value := range versionInfo {
		if !first {
			fmt.Println(",")
		}
		fmt.Printf("  \"%s\": \"%s\"", key, value)
		first = false
	}
	fmt.Println("\n}")
}
