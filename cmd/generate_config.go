package cmd

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/spf13/cobra"
	"gopkg.in/ini.v1"
)

var (
	ssoStartURL   string
	ssoRegion     string
	defaultRegion string
	profile       string
)

var generateConfigCmd = &cobra.Command{
	Use:   "generate-config",
	Short: "Generate ~/.aws/config with all visible SSO accounts (using existing AWS SSO login)",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		if ssoStartURL == "" || ssoRegion == "" || profile == "" {
			log.Fatal("You must provide --sso-start-url, --sso-region and --profile")
		}

		// Load AWS config using the provided SSO profile
		cfg, err := config.LoadDefaultConfig(ctx,
			config.WithRegion(ssoRegion),
			config.WithSharedConfigProfile(profile),
		)
		if err != nil {
			log.Fatalf("failed to load SDK config: %v", err)
		}

		// Create an SSO client
		ssoClient := sso.NewFromConfig(cfg)

		// Find the cached SSO access token
		token, err := getAccessTokenFromCache(ssoStartURL)
		if err != nil {
			log.Fatalf("Failed to get SSO token: %v", err)
		}

		// List accounts
		accountsResp, err := ssoClient.ListAccounts(ctx, &sso.ListAccountsInput{
			AccessToken: &token,
			MaxResults:  aws.Int32(100),
		})
		if err != nil {
			log.Fatalf("failed to list accounts: %v", err)
		}

		// Open ~/.aws/config
		cfgPath := filepath.Join(os.Getenv("HOME"), ".aws", "config")
		awsCfg, err := ini.LooseLoad(cfgPath)
		if err != nil {
			log.Fatalf("failed to read config: %v", err)
		}

		for _, acc := range accountsResp.AccountList {
			rolesResp, err := ssoClient.ListAccountRoles(ctx, &sso.ListAccountRolesInput{
				AccessToken: &token,
				AccountId:   acc.AccountId,
				MaxResults:  aws.Int32(100),
			})
			if err != nil {
				log.Printf("Failed to list roles for account %s: %v", *acc.AccountId, err)
				continue
			}

			for _, role := range rolesResp.RoleList {
				sectionName := fmt.Sprintf("profile %s-%s", cleanName(*acc.AccountName), *role.RoleName)
				sec := awsCfg.Section(sectionName)
				sec.Key("sso_start_url").SetValue(ssoStartURL)
				sec.Key("sso_region").SetValue(ssoRegion)
				sec.Key("sso_account_id").SetValue(*acc.AccountId)
				sec.Key("sso_role_name").SetValue(*role.RoleName)
				sec.Key("region").SetValue(defaultRegion)
				sec.Key("output").SetValue("json")
			}
		}

		err = awsCfg.SaveTo(cfgPath)
		if err != nil {
			log.Fatalf("Failed to save config: %v", err)
		}

		fmt.Printf("✅ AWS config updated at %s\n", cfgPath)
	},
}

func init() {
	rootCmd.AddCommand(generateConfigCmd)
	generateConfigCmd.Flags().StringVar(&ssoStartURL, "sso-start-url", "", "SSO start URL (required)")
	generateConfigCmd.Flags().StringVar(&ssoRegion, "sso-region", "", "SSO region (required)")
	generateConfigCmd.Flags().StringVar(&defaultRegion, "default-region", "eu-west-1", "Default AWS region for profiles")
	generateConfigCmd.Flags().StringVar(&profile, "profile", "", "AWS CLI SSO profile to use (required)")
}

func cleanName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "-")
}

func getAccessTokenFromCache(ssoStartURL string) (string, error) {
	cacheDir := filepath.Join(os.Getenv("HOME"), ".aws", "sso", "cache")
	files, err := ioutil.ReadDir(cacheDir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			fullPath := filepath.Join(cacheDir, file.Name())
			data, err := ioutil.ReadFile(fullPath)
			if err != nil {
				continue
			}

			var tokenFile map[string]interface{}
			if err := json.Unmarshal(data, &tokenFile); err != nil {
				continue
			}

			// Match the correct start URL
			if tokenFile["startUrl"] == ssoStartURL {
				accessToken, ok := tokenFile["accessToken"].(string)
				if !ok {
					continue
				}
				return accessToken, nil
			}
		}
	}

	return "", errors.New("no valid SSO token found, did you run aws sso login?")
}
