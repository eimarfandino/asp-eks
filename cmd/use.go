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

// AzureConfig holds the Azure authentication configuration
type AzureConfig struct {
	ServerID string
	ClientID string
	TenantID string
}

// getAzureConfig returns the appropriate Azure configuration based on the dev flag
func getAzureConfig(dev bool) AzureConfig {
	if dev {
		return AzureConfig{
			ServerID: "6dae42f8-4368-4678-94ff-3960e28e3630",
			ClientID: "6dae42f8-4368-4678-94ff-3960e28e3630",
			TenantID: "f8cdef31-a31e-4b4a-93e4-5f571e91255a",
		}
	}

	// Default/production configuration
	return AzureConfig{
		ServerID: "92996fd8-8fc1-4676-ab8f-63a70ebf20dd",
		ClientID: "92996fd8-8fc1-4676-ab8f-63a70ebf20dd",
		TenantID: "fa4a04c1-369d-4205-9eb1-84f8de4b3248",
	}
}

var execCommand = exec.Command
var outputWriter io.Writer = os.Stdout

// Default cluster provider - can be easily swapped to ManualClusterProvider in the future
var clusterProvider ClusterProvider = &AWSClusterProvider{}

// credentialsValidator can be mocked in tests
var credentialsValidator = isCredentialsValid

// Flag to determine if dev configuration should be used
var devFlag bool

var useCmd = &cobra.Command{
	Use:   "use [profile]",
	Short: "Use a specific AWS profile and set kubeconfig for an EKS cluster",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		profile := args[0]
		ctx := context.Background()

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
			updateKubeconfig(profile, clusterList[0], devFlag)
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
		updateKubeconfig(profile, selected, devFlag)
	},
}

func init() {
	rootCmd.AddCommand(useCmd)
	useCmd.Flags().BoolVar(&devFlag, "dev", false, "Use development Azure configuration")
}

func updateKubeconfig(profile, cluster string, dev bool) {
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

	// Add Azure user configuration
	if err := addAzureUser(dev); err != nil {
		fmt.Fprintf(outputWriter, "Warning: Failed to add Azure user configuration: %v\n", err)
	}

	// Add Azure context for this cluster
	if err := addAzureContext(cluster); err != nil {
		fmt.Fprintf(outputWriter, "Warning: Failed to add Azure context: %v\n", err)
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

func addAzureUser(dev bool) error {
	configType := "production"
	if dev {
		configType = "development"
	}
	fmt.Fprintf(outputWriter, "Adding Azure user configuration (%s)...\n", configType)

	configPath := getDefaultKubeConfigPath()
	if configPath == "" {
		return fmt.Errorf("could not determine kubeconfig path")
	}

	loadingRules := clientcmd.ClientConfigLoadingRules{
		Precedence: []string{configPath},
	}
	config, err := loadingRules.Load()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Get Azure configuration based on dev flag
	azureConfig := getAzureConfig(dev)

	// Create azure-user with kubelogin configuration
	authInfo := config.AuthInfos["azure-user"]
	if authInfo == nil {
		authInfo = api.NewAuthInfo()
	}
	authInfo.LocationOfOrigin = configPath
	authInfo.Exec = &api.ExecConfig{
		APIVersion: "client.authentication.k8s.io/v1beta1",
		Command:    "kubelogin",
		Args: []string{
			"get-token",
			"--environment", "AzurePublicCloud",
			"--server-id", azureConfig.ServerID,
			"--client-id", azureConfig.ClientID,
			"--tenant-id", azureConfig.TenantID,
		},
	}

	config.AuthInfos["azure-user"] = authInfo

	// Write config
	configAccess := clientcmd.NewDefaultPathOptions()
	configAccess.GlobalFile = configPath
	if err := clientcmd.ModifyConfig(configAccess, *config, true); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return nil
}

func addAzureContext(cluster string) error {
	fmt.Fprintln(outputWriter, "Adding Azure context for cluster...")

	configPath := getDefaultKubeConfigPath()
	if configPath == "" {
		return fmt.Errorf("could not determine kubeconfig path")
	}

	loadingRules := clientcmd.ClientConfigLoadingRules{
		Precedence: []string{configPath},
	}
	config, err := loadingRules.Load()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Get the cluster ARN from the existing context
	var clusterArn string
	if existingContext, exists := config.Contexts[cluster]; exists {
		clusterArn = existingContext.Cluster
	} else {
		fmt.Fprintln(outputWriter, "Warning: Could not find existing context, using cluster name as fallback")
		clusterArn = cluster
	}

	if clusterArn == "" {
		fmt.Fprintln(outputWriter, "Warning: Empty cluster ARN, using cluster name as fallback")
		clusterArn = cluster
	}

	contextName := fmt.Sprintf("entraid-%s", cluster)

	// Create or update Azure context
	context := config.Contexts[contextName]
	if context == nil {
		context = api.NewContext()
	}
	context.LocationOfOrigin = configPath
	context.Cluster = clusterArn
	context.AuthInfo = "azure-user"

	config.Contexts[contextName] = context

	// Write config
	configAccess := clientcmd.NewDefaultPathOptions()
	configAccess.GlobalFile = configPath
	if err := clientcmd.ModifyConfig(configAccess, *config, true); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return nil
}

// ensureSSO attempts to validate credentials and provides helpful messaging if they're invalid
func ensureSSO(profile string) error {
	fmt.Fprintf(outputWriter, "Checking credentials for profile %s...\n", profile)

	ctx := context.Background()

	// Check if credentials work using AWS SDK
	if credentialsValidator(ctx, profile) {
		fmt.Fprintln(outputWriter, "Credentials are valid")
		return nil
	}

	fmt.Fprintf(outputWriter, "Credentials for profile '%s' are expired or invalid.\n", profile)
	fmt.Fprintf(outputWriter, "Please run: aws sso login --profile %s\n", profile)
	fmt.Fprintln(outputWriter, "Then try this command again.")

	return fmt.Errorf("credentials are invalid for profile %s", profile)
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

// Helper functions to extract region and account from cluster context - removed as not needed with improved approach above
