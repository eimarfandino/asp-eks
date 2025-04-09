package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use [profile]",
	Short: "Use a specific AWS profile and set kubeconfig for an EKS cluster",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		profile := args[0]

		// Login via SSO
		if err := runAwsCommand(profile, "sso", "login"); err != nil {
			fmt.Println("SSO login failed:", err)
			return
		}

		// Get region
		regionRaw, err := getAwsCommandOutput(profile, "configure", "get", "region")
		if err != nil || strings.TrimSpace(regionRaw) == "" {
			fmt.Printf("No region configured for profile %s\n", profile)
			return
		}
		region := strings.TrimSpace(regionRaw)

		// Get EKS clusters
		clustersRaw, err := getAwsCommandOutput(profile,
			"eks", "list-clusters",
			"--region", region,
			"--query", "clusters[]",
			"--output", "text",
		)
		if err != nil || strings.TrimSpace(clustersRaw) == "" {
			fmt.Println("No EKS clusters found in this account")
			return
		}

		clusterList := strings.Fields(clustersRaw)

		if len(clusterList) == 1 {
			fmt.Println("Only one cluster found:", clusterList[0])
			updateKubeconfig(profile, region, clusterList[0])
			return
		}

		fmt.Println("Available clusters in region", region)
		for i, cluster := range clusterList {
			fmt.Printf("[%d] %s\n", i+1, cluster)
		}

		fmt.Print("Select cluster by number: ")
		var input string
		fmt.Scanln(&input)
		choice, err := strconv.Atoi(strings.TrimSpace(input))
		if err != nil || choice < 1 || choice > len(clusterList) {
			fmt.Println("Invalid selection")
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
	cmd := exec.Command("aws", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getAwsCommandOutput(profile string, args ...string) (string, error) {
	args = append(args, "--profile", profile)
	out, err := exec.Command("aws", args...).CombinedOutput()
	return string(out), err
}

func updateKubeconfig(profile, region, cluster string) {
	fmt.Println("Updating kubeconfig for cluster:", cluster)
	cmd := exec.Command("aws", "eks", "update-kubeconfig",
		"--region", region,
		"--name", cluster,
		"--alias", cluster,
		"--profile", profile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
