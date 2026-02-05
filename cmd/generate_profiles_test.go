package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestGenerateProfilesCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "help command",
			args: []string{"generate-profiles", "--help"},
			want: "Generate AWS profiles for all SSO accounts and roles",
		},
		{
			name: "dry-run flag available",
			args: []string{"generate-profiles", "--help"},
			want: "--dry-run",
		},
		{
			name: "region flag available",
			args: []string{"generate-profiles", "--help"},
			want: "--region",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			rootCmd.SetOut(&output)
			rootCmd.SetArgs(tt.args)

			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			got := output.String()
			if !strings.Contains(got, tt.want) {
				t.Errorf("Expected output to contain '%s', got: %s", tt.want, got)
			}
		})
	}
}

func TestGenerateProfilesFromAccountRoles(t *testing.T) {
	accountRoles := []AccountRole{
		{
			AccountID:   "123456789012",
			AccountName: "Test Account",
			RoleName:    "AdminRole",
		},
		{
			AccountID:   "987654321098",
			AccountName: "Prod Account",
			RoleName:    "ReadOnlyRole",
		},
	}

	ssoStartURL := "https://test.awsapps.com/start"
	ssoRegion := "us-east-1"

	// Set dry run to avoid output during tests
	originalDryRun := dryRun
	dryRun = true
	defer func() { dryRun = originalDryRun }()

	profiles := generateProfilesFromAccountRoles(accountRoles, ssoStartURL, ssoRegion, "")

	expectedProfiles := []string{
		"test-account-adminrole",
		"prod-account-readonlyrole",
	}

	if len(profiles) != len(expectedProfiles) {
		t.Errorf("Expected %d profiles, got %d", len(expectedProfiles), len(profiles))
	}

	for _, expectedProfile := range expectedProfiles {
		if _, exists := profiles[expectedProfile]; !exists {
			t.Errorf("Expected profile '%s' to exist in generated profiles", expectedProfile)
		}
	}

	// Test profile configuration
	testProfile := profiles["test-account-adminrole"]
	if testProfile["sso_start_url"] != ssoStartURL {
		t.Errorf("Expected sso_start_url to be '%s', got '%s'", ssoStartURL, testProfile["sso_start_url"])
	}
	if testProfile["sso_region"] != ssoRegion {
		t.Errorf("Expected sso_region to be '%s', got '%s'", ssoRegion, testProfile["sso_region"])
	}
	if testProfile["sso_account_id"] != "123456789012" {
		t.Errorf("Expected sso_account_id to be '123456789012', got '%s'", testProfile["sso_account_id"])
	}
	if testProfile["sso_role_name"] != "AdminRole" {
		t.Errorf("Expected sso_role_name to be 'AdminRole', got '%s'", testProfile["sso_role_name"])
	}
}
