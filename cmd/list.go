package cmd

import (
	"eks-aws-profile-switcher/awsutils"
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available AWS profiles",
	Run: func(cmd *cobra.Command, args []string) {
		profiles, err := awsutils.GetAwsProfiles()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Println("Available profiles:")
		for _, profile := range profiles {
			fmt.Println(profile)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
