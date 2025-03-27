package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "dalforge",
	Short: "A CLI tool for generating .go and .sql files from YAML definitions",
	Long: `dalforge is a command-line interface for scanning a directory
containing YAML files and generating corresponding .go and .sql files
in a specified output directory.`,
	// No default run action here—subcommands will handle the logic.
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Subcommands (generate, version) are added from their own files.
}
