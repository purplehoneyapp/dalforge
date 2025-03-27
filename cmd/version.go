package cmd

import (
	"dalforge/version"
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd prints the current version of dalforge.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of dalforge",
	Long:  `All software has versions. This is dalforge's.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("dalforge %v\n", version.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
