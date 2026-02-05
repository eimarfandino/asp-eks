package cmd

import (
	"fmt"

	"github.com/eimarfandino/asp-eks/awsutils"
	"github.com/spf13/cobra"
)

var getProfiles = awsutils.GetAwsProfiles // 👈 override in test if needed

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available AWS profiles",
	Run: func(cmd *cobra.Command, args []string) {
		profiles, err := getProfiles()
		if err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "Error:", err)
			return
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Available profiles:")
		for _, profile := range profiles {
			fmt.Fprintln(cmd.OutOrStdout(), profile)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
