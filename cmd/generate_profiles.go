package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/spf13/cobra"
	"gopkg.in/ini.v1"
)

// Configure ini formatting to avoid backticks and extra spaces
func configureIniFormatting() {
	ini.PrettyFormat = false
	ini.PrettyEqual = true
}

var (
	defaultRegion   string
	dryRun          bool
	ssoStartURLFlag string
)

var generateProfilesCmd = &cobra.Command{
	Use:   "generate-profiles",
	Short: "Generate AWS profiles for all SSO accounts and roles",
	Long: `Generate AWS profiles for all SSO accounts and roles accessible to your user.
This command will:
1. Create default SSO configuration if not present called "DEFAULT-SSO"
2. Query AWS SSO to get all accounts and roles available to you
3. Generate AWS CLI profiles for each account/role combination
4. Write them to ~/.aws/config

The profiles will be named in the format: <account-alias>-<role-name> or <account-id>-<role-name> if no alias is available.

Prerequisites:
- You must be logged in to AWS SSO (run 'aws sso login --profile DEFAULT-SSO' after first run)`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := generateProfiles(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error generating profiles: %v\n", err)
			os.Exit(1)
		}
	},
}

type AccountRole struct {
	AccountID    string
	AccountName  string
	RoleName     string
	EmailAddress string
}

type SSOCacheToken struct {
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Region      string    `json:"region"`
	StartURL    string    `json:"startUrl"`
}

func generateProfiles() error {
	ctx := context.Background()

	// Load AWS config to get SSO configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Get SSO start URL and region from existing config, unless overridden by flag
	var ssoStartURL, ssoRegion, ssoSessionName string
	if ssoStartURLFlag != "" {
		// Sanitize the SSO start URL: remove trailing # or /
		ssoStartURL = strings.TrimRight(ssoStartURLFlag, "#/\\")
		ssoRegion, ssoSessionName = "", ""
		homeDir, homeErr := os.UserHomeDir()
		if homeErr == nil {
			configPath := filepath.Join(homeDir, ".aws", "config")
			if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
				// Create minimal config file using the provided SSO start URL
				cfg := ini.Empty()
				cfg.BlockMode = false
				ssoSessionSection, _ := cfg.NewSection("sso-session DEFAULT-SSO")
				ssoSessionSection.NewKey("sso_start_url", ssoStartURL)
				ssoSessionSection.NewKey("sso_region", defaultRegion)
				ssoSessionSection.NewKey("sso_registration_scopes", "sso:account:access")
				baseProfileSection, _ := cfg.NewSection("profile DEFAULT-SSO")
				baseProfileSection.NewKey("sso_start_url", ssoStartURL)
				baseProfileSection.NewKey("sso_region", defaultRegion)
				baseProfileSection.NewKey("sso_role_name", "itfrun-operator")
				baseProfileSection.NewKey("region", defaultRegion)
				baseProfileSection.NewKey("output", "json")
				// Ensure .aws directory exists
				awsDir := filepath.Join(homeDir, ".aws")
				os.MkdirAll(awsDir, 0755)
				// Write config file
				writeConfigWithoutEscaping(cfg, configPath)
				ssoRegion = defaultRegion
				ssoSessionName = "DEFAULT-SSO"
			} else {
				iniCfg, iniErr := ini.Load(configPath)
				if iniErr == nil {
					for _, section := range iniCfg.Sections() {
						if section.HasKey("sso_start_url") && section.Key("sso_start_url").String() == ssoStartURL {
							if section.HasKey("sso_region") {
								ssoRegion = section.Key("sso_region").String()
							}
							if strings.HasPrefix(section.Name(), "sso-session ") {
								ssoSessionName = strings.TrimPrefix(section.Name(), "sso-session ")
							}
							break
						}
					}
				}
			}
		}
		if ssoRegion == "" {
			ssoRegion = defaultRegion
		}
	} else {
		// Check if ~/.aws/config exists
		homeDir, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return fmt.Errorf("failed to get home directory: %w", homeErr)
		}
		configPath := filepath.Join(homeDir, ".aws", "config")
		if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
			return fmt.Errorf("No AWS config file found and --sso-start-url not provided. Please provide --sso-start-url to continue.")
		}
		var getInfoErr error
		ssoStartURL, ssoRegion, ssoSessionName, getInfoErr = getSSOReuiredInfo()
		if getInfoErr != nil {
			return fmt.Errorf("failed to get SSO configuration from config file: %w", getInfoErr)
		}
	}

	fmt.Printf("Using SSO start URL: %s\n", ssoStartURL)
	fmt.Printf("Using SSO region: %s\n", ssoRegion)
	if ssoSessionName != "" {
		fmt.Printf("Using SSO session: %s\n", ssoSessionName)
	}

	// Get access token
	accessToken, err := getSSOAccessToken(ctx, ssoStartURL, ssoRegion)
	if err != nil {
		return fmt.Errorf("failed to get SSO access token: %s\n\nTo continue, please login to AWS SSO:\n  aws sso login --profile DEFAULT-SSO\n\nThen run this command again.", err.Error())
	}

	// Create SSO client
	ssoClient := sso.NewFromConfig(cfg, func(o *sso.Options) {
		o.Region = ssoRegion
	})

	// Get all accounts and roles
	accountRoles, err := listAccountRoles(ctx, ssoClient, accessToken)
	if err != nil {
		return fmt.Errorf("failed to list account roles: %w", err)
	}

	if len(accountRoles) == 0 {
		fmt.Println("No accounts or roles found")
		return nil
	}

	fmt.Printf("Found %d account/role combinations\n", len(accountRoles))

	// Generate profiles
	profiles := generateProfilesFromAccountRoles(accountRoles, ssoStartURL, ssoRegion, ssoSessionName)

	if dryRun {
		fmt.Println("\nDry run mode - showing profiles that would be generated:")
		for profileName, profileConfig := range profiles {
			fmt.Printf("\n[profile %s]\n", profileName)
			for key, value := range profileConfig {
				fmt.Printf("%s = %s\n", key, value)
			}
		}
		fmt.Printf("\nTotal profiles that would be generated: %d\n", len(profiles))
		return nil
	}

	// Write to ~/.aws/config
	if err := writeProfilesToConfig(profiles); err != nil {
		return fmt.Errorf("failed to write profiles to config: %w", err)
	}

	fmt.Printf("Successfully generated %d profiles in ~/.aws/config\n", len(profiles))
	return nil
}

func getSSOReuiredInfo() (startURL, region, ssoSessionName string, err error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".aws", "config")
	cfg, err := ini.Load(configPath)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to load AWS config file: %w", err)
	}

	// Look for SSO session configuration first (newer format)
	for _, section := range cfg.Sections() {
		if strings.HasPrefix(section.Name(), "sso-session ") {
			sessionName := strings.TrimPrefix(section.Name(), "sso-session ")
			if section.HasKey("sso_start_url") && section.HasKey("sso_region") {
				startURL = section.Key("sso_start_url").String()
				region = section.Key("sso_region").String()
				return startURL, region, sessionName, nil
			}
		}
	}

	// Fallback to old format if no sso-session found
	var fallbackStartURL, fallbackRegion string

	for _, section := range cfg.Sections() {
		if section.HasKey("sso_start_url") && section.HasKey("sso_region") {
			currentStartURL := section.Key("sso_start_url").String()
			currentRegion := section.Key("sso_region").String()

			// Prefer profiles without sso_account_id (these are typically the base SSO profiles for login)
			if !section.HasKey("sso_account_id") {
				return currentStartURL, currentRegion, "", nil
			}

			// Keep as fallback if we don't find a base SSO profile
			if fallbackStartURL == "" {
				fallbackStartURL = currentStartURL
				fallbackRegion = currentRegion
			}
		}
	}

	if fallbackStartURL != "" && fallbackRegion != "" {
		return fallbackStartURL, fallbackRegion, "", nil
	}

	return "", "", "", fmt.Errorf("SSO configuration not found in ~/.aws/config. Please ensure you have at least one SSO profile or sso-session configured")
}

func createDefaultSSOConfiguration() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".aws", "config")

	// Ensure the .aws directory exists
	awsDir := filepath.Join(homeDir, ".aws")
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		return fmt.Errorf("failed to create .aws directory: %w", err)
	}

	// Load existing config or create new one
	var cfg *ini.File
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg = ini.Empty()
	} else {
		cfg, err = ini.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load existing config: %w", err)
		}
	}

	// Configure formatting to avoid backticks
	cfg.BlockMode = false

	// Check if sso-session already exists
	ssoSessionExists := false
	for _, section := range cfg.Sections() {
		if section.Name() == "sso-session DEFAULT-SSO" {
			ssoSessionExists = true
			break
		}
	}

	// Use the sanitized flag value if provided, else error
	ssoStartURL := strings.TrimRight(ssoStartURLFlag, "#/\\")
	if ssoStartURL == "" {
		return fmt.Errorf("No SSO start URL provided. Please use --sso-start-url flag.")
	}

	// Create sso-session if it doesn't exist
	if !ssoSessionExists {
		ssoSessionSection, err := cfg.NewSection("sso-session DEFAULT-SSO")
		if err != nil {
			return fmt.Errorf("failed to create sso-session section: %w", err)
		}
		ssoSessionSection.NewKey("sso_start_url", ssoStartURL)
		ssoSessionSection.NewKey("sso_region", defaultRegion)
		ssoSessionSection.NewKey("sso_registration_scopes", "sso:account:access")
		fmt.Println("Created [sso-session DEFAULT-SSO] configuration")
	}

	// Check if base DEFAULT-SSO profile exists
	baseProfileExists := false
	for _, section := range cfg.Sections() {
		if section.Name() == "profile DEFAULT-SSO" {
			baseProfileExists = true
			break
		}
	}

	// Create base DEFAULT-SSO profile if it doesn't exist
	if !baseProfileExists {
		baseProfileSection, err := cfg.NewSection("profile DEFAULT-SSO")
		if err != nil {
			return fmt.Errorf("failed to create base profile section: %w", err)
		}
		baseProfileSection.NewKey("sso_start_url", ssoStartURL)
		baseProfileSection.NewKey("sso_region", defaultRegion)
		baseProfileSection.NewKey("sso_role_name", "itfrun-operator")
		baseProfileSection.NewKey("region", defaultRegion)
		baseProfileSection.NewKey("output", "json")
		fmt.Println("Created [profile DEFAULT-SSO] base profile")
	}

	// Save the configuration
	return writeConfigWithoutEscaping(cfg, configPath)
}

// appendToConfig appends text to a config file
func appendToConfig(configPath, content string) error {
	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}

func getAvailableSSOProfiles() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	configPath := filepath.Join(homeDir, ".aws", "config")
	cfg, err := ini.Load(configPath)
	if err != nil {
		return nil
	}

	var completeSSOProfiles []string
	var baseSSOProfiles []string

	for _, section := range cfg.Sections() {
		if section.HasKey("sso_start_url") && section.HasKey("sso_region") {
			name := section.Name()
			profileName := ""

			if name == "DEFAULT" {
				profileName = "default"
			} else if strings.HasPrefix(name, "profile ") {
				profileName = strings.TrimPrefix(name, "profile ")
			}

			if profileName != "" {
				// Separate complete profiles (with account_id) from base profiles
				if section.HasKey("sso_account_id") {
					completeSSOProfiles = append(completeSSOProfiles, profileName)
				} else {
					baseSSOProfiles = append(baseSSOProfiles, profileName)
				}
			}
		}
	}

	// Return complete profiles first (these work for login), then base profiles
	result := append(completeSSOProfiles, baseSSOProfiles...)
	return result
}

func getSSOAccessToken(ctx context.Context, startURL, region string) (string, error) {
	// Load the cached SSO token
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Check for cached tokens in ~/.aws/sso/cache/
	cacheDir := filepath.Join(homeDir, ".aws", "sso", "cache")

	// List all cache files
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", fmt.Errorf("failed to read SSO cache directory. Please run 'aws sso login' first: %w", err)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			cachePath := filepath.Join(cacheDir, entry.Name())
			token, err := readTokenFromCache(cachePath, startURL)
			if err == nil && token != "" {
				return token, nil
			}
		}
	}

	return "", fmt.Errorf("no valid SSO token found. Please run 'aws sso login' first")
}

func readTokenFromCache(cachePath, startURL string) (string, error) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return "", err
	}

	var token SSOCacheToken
	if err := json.Unmarshal(data, &token); err != nil {
		return "", fmt.Errorf("failed to parse cache file: %w", err)
	}

	// Check if this cache file is for the correct start URL
	if token.StartURL != startURL {
		return "", fmt.Errorf("cache file doesn't match start URL")
	}

	// Check if token is expired
	if time.Now().After(token.ExpiresAt) {
		return "", fmt.Errorf("token is expired")
	}

	return token.AccessToken, nil
}

func listAccountRoles(ctx context.Context, ssoClient *sso.Client, accessToken string) ([]AccountRole, error) {
	var accountRoles []AccountRole

	// List accounts
	listAccountsInput := &sso.ListAccountsInput{
		AccessToken: aws.String(accessToken),
	}

	accountsPaginator := sso.NewListAccountsPaginator(ssoClient, listAccountsInput)

	for accountsPaginator.HasMorePages() {
		accountsOutput, err := accountsPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list accounts: %w", err)
		}

		// For each account, list roles
		for _, account := range accountsOutput.AccountList {
			listRolesInput := &sso.ListAccountRolesInput{
				AccessToken: aws.String(accessToken),
				AccountId:   account.AccountId,
			}

			rolesPaginator := sso.NewListAccountRolesPaginator(ssoClient, listRolesInput)

			for rolesPaginator.HasMorePages() {
				rolesOutput, err := rolesPaginator.NextPage(ctx)
				if err != nil {
					fmt.Printf("Warning: failed to list roles for account %s: %v\n", *account.AccountId, err)
					continue
				}

				for _, role := range rolesOutput.RoleList {
					accountRoles = append(accountRoles, AccountRole{
						AccountID:    *account.AccountId,
						AccountName:  *account.AccountName,
						RoleName:     *role.RoleName,
						EmailAddress: *account.EmailAddress,
					})
				}
			}
		}
	}

	// Sort by account name, then role name
	sort.Slice(accountRoles, func(i, j int) bool {
		if accountRoles[i].AccountName == accountRoles[j].AccountName {
			return accountRoles[i].RoleName < accountRoles[j].RoleName
		}
		return accountRoles[i].AccountName < accountRoles[j].AccountName
	})

	return accountRoles, nil
}

func generateProfilesFromAccountRoles(accountRoles []AccountRole, ssoStartURL, ssoRegion, ssoSessionName string) map[string]map[string]string {
	profiles := make(map[string]map[string]string)

	// Use the provided default region or fall back to eu-central-1
	region := defaultRegion
	if region == "" {
		region = "eu-central-1"
	}

	for _, ar := range accountRoles {
		// Generate profile name: use account name if available, otherwise account ID
		accountIdentifier := ar.AccountName
		if accountIdentifier == "" {
			accountIdentifier = ar.AccountID
		}

		// Clean up account identifier for use in profile name
		accountIdentifier = strings.ReplaceAll(accountIdentifier, " ", "-")
		accountIdentifier = strings.ReplaceAll(accountIdentifier, ".", "-")
		accountIdentifier = strings.ToLower(accountIdentifier)

		// Simplify role name
		roleName := strings.ToLower(ar.RoleName)
		// If role contains "itfrun-operator", just use "operator"
		if strings.Contains(roleName, "itfrun-operator") {
			roleName = "operator"
		} else if strings.HasPrefix(roleName, "itfrun-") {
			roleName = strings.TrimPrefix(roleName, "itfrun-")
		}

		// Generate cleaner profile name: <account>-<role>
		profileName := fmt.Sprintf("%s-%s", accountIdentifier, roleName)

		// Use sso_session format if available, otherwise fall back to old format
		profileConfig := map[string]string{
			"sso_account_id": ar.AccountID,
			"sso_role_name":  ar.RoleName,
			"region":         region,
			"output":         "json",
		}

		if ssoSessionName != "" {
			// Use new sso-session format
			profileConfig["sso_session"] = ssoSessionName
		} else {
			// Use old format
			profileConfig["sso_start_url"] = ssoStartURL
			profileConfig["sso_region"] = ssoRegion
		}

		profiles[profileName] = profileConfig

		if !dryRun {
			fmt.Printf("Generated profile: %s (Account: %s, Role: %s)\n", profileName, ar.AccountName, ar.RoleName)
		}
	}

	return profiles
}

func writeProfilesToConfig(profiles map[string]map[string]string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".aws", "config")

	// Load existing config
	cfg, err := ini.Load(configPath)
	if err != nil {
		// If file doesn't exist, create a new one
		cfg = ini.Empty()
	}

	// Configure formatting to avoid backticks
	cfg.BlockMode = false

	// Add or update profiles
	for profileName, profileConfig := range profiles {
		sectionName := fmt.Sprintf("profile %s", profileName)

		// Remove existing section if it exists
		cfg.DeleteSection(sectionName)

		// Create new section
		section, err := cfg.NewSection(sectionName)
		if err != nil {
			return fmt.Errorf("failed to create section %s: %w", sectionName, err)
		}

		// Add keys
		for key, value := range profileConfig {
			section.NewKey(key, value)
		}
	}

	// Custom save to avoid ini library escaping URLs with #
	return writeConfigWithoutEscaping(cfg, configPath)
}

// writeConfigWithoutEscaping manually writes the config file to avoid URL escaping issues
func writeConfigWithoutEscaping(cfg *ini.File, configPath string) error {
	// Create a temporary file to write the new content
	tempFile, err := os.CreateTemp(filepath.Dir(configPath), ".aws-config-temp-")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	// Write sections manually
	for _, section := range cfg.Sections() {
		if section.Name() != "DEFAULT" {
			if _, err := fmt.Fprintf(tempFile, "[%s]\n", section.Name()); err != nil {
				return fmt.Errorf("failed to write section header: %w", err)
			}
		}

		keys := section.Keys()
		// Sort keys for consistent output
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].Name() < keys[j].Name()
		})

		for _, key := range keys {
			value := key.String()
			// Write key-value pair without escaping
			if _, err := fmt.Fprintf(tempFile, "%s = %s\n", key.Name(), value); err != nil {
				return fmt.Errorf("failed to write key-value pair: %w", err)
			}
		}

		// Add blank line after each section
		if _, err := fmt.Fprintf(tempFile, "\n"); err != nil {
			return fmt.Errorf("failed to write blank line: %w", err)
		}
	}

	tempFile.Close()

	// Replace the original file with the temp file
	return os.Rename(tempFile.Name(), configPath)
}

func init() {
	// Configure ini formatting to avoid backticks and extra spaces
	configureIniFormatting()

	rootCmd.AddCommand(generateProfilesCmd)

	generateProfilesCmd.Flags().StringVarP(&defaultRegion, "region", "r", "eu-central-1", "Default AWS region for generated profiles")
	generateProfilesCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what profiles would be generated without writing to config file")
	generateProfilesCmd.Flags().StringVar(&ssoStartURLFlag, "sso-start-url", "", "Override the SSO start URL for generated profiles (optional)")
}
