package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var execCommand = exec.Command
var outputWriter io.Writer = os.Stdout

var useCmd = &cobra.Command{
	Use:   "use [profile]",
	Short: "Use a specific AWS profile and set kubeconfig for an EKS cluster",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		profile := args[0]

		if err := runAwsCommand(profile, "sso", "login"); err != nil {
			fmt.Fprintln(outputWriter, "SSO login failed:", err)
			return
		}

		regionRaw, err := getAwsCommandOutput(profile, "configure", "get", "region")
		if err != nil || strings.TrimSpace(regionRaw) == "" {
			fmt.Fprintf(outputWriter, "No region configured for profile %s\n", profile)
			return
		}
		region := strings.TrimSpace(regionRaw)

		clustersRaw, err := getAwsCommandOutput(profile,
			"eks", "list-clusters",
			"--region", region,
			"--query", "clusters[]",
			"--output", "text",
		)
		if err != nil || strings.TrimSpace(clustersRaw) == "" {
			fmt.Fprintln(outputWriter, "No EKS clusters found in this account")
			return
		}

		clusterList := strings.Fields(clustersRaw)

		if len(clusterList) == 1 {
			fmt.Fprintln(outputWriter, "Only one cluster found:", clusterList[0])
			updateKubeconfig(profile, region, clusterList[0])
			return
		}

		fmt.Fprintln(outputWriter, "Available clusters in region", region)
		for i, cluster := range clusterList {
			fmt.Fprintf(outputWriter, "[%d] %s\n", i+1, cluster)
		}

		fmt.Fprint(outputWriter, "Select cluster by number: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintln(outputWriter, "Error reading input:", err)
			return
		}
		input = strings.TrimSpace(input)
		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(clusterList) {
			fmt.Fprintln(outputWriter, "Invalid selection")
			return
		}

		selected := clusterList[choice-1]
		updateKubeconfig(profile, region, selected)
	},
}

func init() {
	rootCmd.AddCommand(useCmd)
}

func runAwsCommand(profile string, args ...string) error {
	args = append(args, "--profile", profile)
	cmd := execCommand("aws", args...)
	cmd.Stdout = outputWriter
	cmd.Stderr = outputWriter
	return cmd.Run()
}

func getAwsCommandOutput(profile string, args ...string) (string, error) {
	args = append(args, "--profile", profile)
	cmd := execCommand("aws", args...)
	out, err := cmd.Output()
	return string(out), err
}

func updateKubeconfig(profile, region, cluster string) {
	fmt.Fprintln(outputWriter, "Updating kubeconfig for cluster:", cluster)
	cmd := execCommand("aws", "eks", "update-kubeconfig",
		"--region", region,
		"--name", cluster,
		"--alias", cluster,
		"--profile", profile)
	cmd.Stdout = outputWriter
	cmd.Stderr = outputWriter
	_ = cmd.Run()
}
