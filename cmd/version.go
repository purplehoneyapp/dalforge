package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd prints the current version of dalcreator.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of dalcreator",
	Long:  `All software has versions. This is dalcreator's.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("dalcreator v1.0.0")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
