package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var execCommand = exec.Command
var outputWriter io.Writer = os.Stdout

// Default cluster provider - can be easily swapped to ManualClusterProvider in the future
var clusterProvider ClusterProvider = &AWSClusterProvider{}

// credentialsValidator can be mocked in tests
var credentialsValidator = isCredentialsValid

var exportFlag bool

var useCmd = &cobra.Command{
	Use:   "use [profile]",
	Short: "Use a specific AWS profile and set kubeconfig for an EKS cluster",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		profile := args[0]
		ctx := context.Background()

		// If export flag is set, redirect informational output to stderr
		originalWriter := outputWriter
		if exportFlag {
			outputWriter = os.Stderr
		}
		defer func() { outputWriter = originalWriter }()

		// Try to login via SSO first
		if err := ensureSSO(profile); err != nil {
			fmt.Fprintf(outputWriter, "Failed to ensure SSO login: %v\n", err)
			return
		}

		// Check if region is configured
		region, err := clusterProvider.GetRegion(ctx, profile)
		if err != nil {
			fmt.Fprintf(outputWriter, "Failed to get region for profile %s: %v\n", profile, err)
			return
		}

		if region == "" {
			fmt.Fprintf(outputWriter, "No region configured for profile %s\n", profile)
			return
		}

		// List clusters
		clusterList, err := clusterProvider.ListClusters(ctx, profile)
		if err != nil {
			fmt.Fprintln(outputWriter, "Failed to list clusters:", err)
			return
		}

		if len(clusterList) == 0 {
			fmt.Fprintln(outputWriter, "No clusters found in this account")
			return
		}

		if len(clusterList) == 1 {
			fmt.Fprintln(outputWriter, "Only one cluster found:", clusterList[0])
			updateKubeconfig(profile, clusterList[0], exportFlag)
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
		updateKubeconfig(profile, selected, exportFlag)
	},
}

func init() {
	rootCmd.AddCommand(useCmd)
	useCmd.Flags().BoolVar(&exportFlag, "export", false, "Output shell commands for eval (export AWS_PROFILE)")
}

func updateKubeconfig(profile, cluster string, export bool) {
	fmt.Fprintln(outputWriter, "Updating kubeconfig for cluster:", cluster)

	ctx := context.Background()

	// Get cluster information
	clusterInfo, err := clusterProvider.GetClusterInfo(ctx, profile, cluster)
	if err != nil {
		fmt.Fprintf(outputWriter, "Failed to get cluster info: %v\n", err)
		return
	}

	err = createOrUpdateKubeContext(profile, clusterInfo)
	if err != nil {
		fmt.Fprintf(outputWriter, "Failed to update kubeconfig: %v\n", err)
		return
	}

	fmt.Fprintf(outputWriter, "Successfully updated kubeconfig for cluster: %s\n", cluster)
	fmt.Fprintf(outputWriter, "Current context set to: %s\n", cluster)

	// If export flag is set, output shell commands
	if export {
		fmt.Fprintf(os.Stdout, "export AWS_PROFILE=%s\n", profile)
	}
}

func getDefaultKubeConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".kube", "config")
}

func createOrUpdateKubeContext(profile string, clusterInfo *ClusterInfo) error {
	configPath := getDefaultKubeConfigPath()
	if configPath == "" {
		return fmt.Errorf("could not determine kubeconfig path")
	}

	// Ensure .kube directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create .kube directory: %w", err)
	}

	loadingRules := clientcmd.ClientConfigLoadingRules{
		Precedence: []string{configPath},
	}
	config, err := loadingRules.Load()
	if err != nil {
		// If config doesn't exist, create a new one
		config = api.NewConfig()
	}

	// Configure cluster
	clusterConfig := config.Clusters[clusterInfo.Arn]
	if clusterConfig == nil {
		clusterConfig = api.NewCluster()
	}
	clusterConfig.Server = clusterInfo.Endpoint
	clusterConfig.CertificateAuthorityData = clusterInfo.CertificateData
	clusterConfig.LocationOfOrigin = configPath

	// Configure auth info
	authInfo := config.AuthInfos[clusterInfo.Arn]
	if authInfo == nil {
		authInfo = api.NewAuthInfo()
	}
	authInfo.LocationOfOrigin = configPath
	authInfo.Exec = &api.ExecConfig{
		APIVersion: "client.authentication.k8s.io/v1beta1",
		Command:    clusterInfo.AuthCommand,
		Args:       clusterInfo.AuthArgs,
	}

	// Add environment variables if present
	if len(clusterInfo.AuthEnv) > 0 {
		var envVars []api.ExecEnvVar
		for key, value := range clusterInfo.AuthEnv {
			envVars = append(envVars, api.ExecEnvVar{
				Name:  key,
				Value: value,
			})
		}
		authInfo.Exec.Env = envVars
	}

	// Configure context
	context := config.Contexts[clusterInfo.Name]
	if context == nil {
		context = api.NewContext()
	}
	context.LocationOfOrigin = configPath
	context.Cluster = clusterInfo.Arn
	context.AuthInfo = clusterInfo.Arn

	// Update config
	config.Clusters[clusterInfo.Arn] = clusterConfig
	config.AuthInfos[clusterInfo.Arn] = authInfo
	config.Contexts[clusterInfo.Name] = context
	config.CurrentContext = clusterInfo.Name

	// Write config
	configAccess := clientcmd.NewDefaultPathOptions()
	configAccess.GlobalFile = configPath
	if err := clientcmd.ModifyConfig(configAccess, *config, true); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return nil
}

// ensureSSO attempts to validate credentials and automatically runs sso login if they're invalid
func ensureSSO(profile string) error {
	fmt.Fprintf(outputWriter, "Checking credentials for profile %s...\n", profile)

	ctx := context.Background()

	if credentialsValidator(ctx, profile) {
		fmt.Fprintln(outputWriter, "Credentials are valid")
		return nil
	}

	fmt.Fprintf(outputWriter, "Credentials for profile '%s' are expired or invalid. Attempting SSO login...\n", profile)

	cmd := execCommand("aws", "sso", "login", "--profile", profile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = outputWriter
	cmd.Stderr = outputWriter
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SSO login failed for profile %s: %w", profile, err)
	}

	// Validate again after login
	if !credentialsValidator(ctx, profile) {
		return fmt.Errorf("credentials still invalid after SSO login for profile %s", profile)
	}

	fmt.Fprintln(outputWriter, "SSO login successful")
	return nil
}

// isCredentialsValid checks if the current credentials are valid using AWS SDK
func isCredentialsValid(ctx context.Context, profile string) bool {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		return false
	}

	stsClient := sts.NewFromConfig(cfg)
	_, err = stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	return err == nil
}
