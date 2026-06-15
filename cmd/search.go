package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for AWS profiles by name",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.ToLower(args[0])

		profiles, err := getProfiles()
		if err != nil {
			fmt.Fprintln(outputWriter, "Failed to list profiles:", err)
			return
		}

		var matches []string
		for _, p := range profiles {
			if strings.Contains(strings.ToLower(p), query) {
				matches = append(matches, p)
			}
		}

		if len(matches) == 0 {
			fmt.Fprintf(outputWriter, "No profiles found matching %q\n", args[0])
			return
		}

		fmt.Fprintf(outputWriter, "Profiles matching %q:\n", args[0])
		for _, p := range matches {
			fmt.Fprintln(outputWriter, p)
		}
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
