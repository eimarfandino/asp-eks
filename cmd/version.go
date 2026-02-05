package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set during build time via ldflags
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of asp-eks",
	Long:  "Print the version of asp-eks. The version is set during build time by the CI/CD pipeline.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
