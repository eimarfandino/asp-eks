package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestListCommand(t *testing.T) {
	// ✅ Inject a fake version of GetAwsProfiles
	getProfiles = func() ([]string, error) {
		return []string{"test-profile-1", "test-profile-2"}, nil
	}
	defer func() {
		getProfiles = nil
	}()

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetArgs([]string{"list"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	got := output.String()
	if !strings.Contains(got, "Available profiles:") {
		t.Errorf("Expected output to contain 'Available profiles:', got: %s", got)
	}
	if !strings.Contains(got, "test-profile-1") || !strings.Contains(got, "test-profile-2") {
		t.Errorf("Expected mocked profiles in output, got: %s", got)
	}
}
