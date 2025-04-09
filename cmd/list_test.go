package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestListCommand(t *testing.T) {
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
}
