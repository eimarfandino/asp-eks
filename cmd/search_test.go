package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestSearchCommand(t *testing.T) {
	getProfiles = func() ([]string, error) {
		return []string{
			"cluster1-test",
			"test-cluster2",
			"cluster3-prod",
			"test-cluster4-dev",
		}, nil
	}
	defer func() { getProfiles = nil }()

	tests := []struct {
		name     string
		query    string
		contains []string
		excludes []string
	}{
		{
			name:     "matches substring in middle",
			query:    "cluster1",
			contains: []string{"cluster1-test"},
			excludes: []string{"cluster3-prod", "test-cluster4-dev"},
		},
		{
			name:     "case insensitive match",
			query:    "CLUSTER1",
			contains: []string{"cluster1-test"},
			excludes: []string{"cluster3-prod", "test-cluster4-dev"},
		},
		{
			name:     "no matches",
			query:    "staging",
			contains: []string{"No profiles found matching"},
			excludes: []string{"cluster1-test", "cluster3-prod"},
		},
		{
			name:     "matches multiple",
			query:    "test",
			contains: []string{"cluster1-test", "test-cluster2", "test-cluster4-dev"},
			excludes: []string{"cluster3-prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			outputWriter = &buf
			defer func() { outputWriter = nil }()

			rootCmd.SetArgs([]string{"search", tt.query})
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := buf.String()
			for _, s := range tt.contains {
				if !strings.Contains(got, s) {
					t.Errorf("expected output to contain %q, got:\n%s", s, got)
				}
			}
			for _, s := range tt.excludes {
				if strings.Contains(got, s) {
					t.Errorf("expected output NOT to contain %q, got:\n%s", s, got)
				}
			}
		})
	}
}
