package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Mock ClusterProvider for testing
type mockClusterProvider struct {
	shouldFailGetRegion      bool
	shouldFailListClusters   bool
	shouldFailGetClusterInfo bool
	clusters                 []string
	region                   string
}

func (m *mockClusterProvider) GetRegion(ctx context.Context, profile string) (string, error) {
	if m.shouldFailGetRegion {
		return "", fmt.Errorf("failed to get region")
	}
	return m.region, nil
}

func (m *mockClusterProvider) ListClusters(ctx context.Context, profile string) ([]string, error) {
	if m.shouldFailListClusters {
		return nil, fmt.Errorf("failed to list clusters")
	}
	return m.clusters, nil
}

func (m *mockClusterProvider) GetClusterInfo(ctx context.Context, profile, clusterName string) (*ClusterInfo, error) {
	if m.shouldFailGetClusterInfo {
		return nil, fmt.Errorf("failed to get cluster info")
	}

	// Create a mock cluster response
	endpoint := "https://test-cluster.sk1.eu-west-1.eks.amazonaws.com"
	arn := "arn:aws:eks:eu-west-1:123456789012:cluster/" + clusterName
	ca := []byte("fake-certificate-data") // Mock CA data as bytes

	return &ClusterInfo{
		Name:            clusterName,
		Endpoint:        endpoint,
		CertificateData: ca,
		Region:          m.region,
		Arn:             arn,
		AuthCommand:     "aws",
		AuthArgs: []string{
			"eks",
			"get-token",
			"--cluster-name", clusterName,
			"--region", m.region,
		},
		AuthEnv: map[string]string{
			"AWS_PROFILE": profile,
		},
	}, nil
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	cmdStr := strings.Join(os.Args, " ")

	switch {
	case strings.Contains(cmdStr, "eks update-kubeconfig"):
		fmt.Fprint(os.Stdout, "kubeconfig updated")
	case strings.Contains(cmdStr, "kubectl"):
		fmt.Fprint(os.Stdout, "kubectl command executed")
	}

	os.Exit(0)
}

func TestUseCommand_MockAWS(t *testing.T) {
	// Mock ClusterProvider
	originalProvider := clusterProvider
	clusterProvider = &mockClusterProvider{
		region:   "eu-west-1",
		clusters: []string{"cluster-one", "cluster-two"},
	}
	defer func() { clusterProvider = originalProvider }()

	// Mock credentials validator to return true (valid credentials)
	originalCredentialsValidator := credentialsValidator
	credentialsValidator = func(ctx context.Context, profile string) bool {
		return true
	}
	defer func() { credentialsValidator = originalCredentialsValidator }()

	// Mock exec commands for kubectl operations
	execCommand = fakeExecCommand
	defer func() { execCommand = exec.Command }()

	// Simulate stdin input for selecting cluster 1
	r, w, _ := os.Pipe()
	originalStdin := os.Stdin
	os.Stdin = r
	w.Write([]byte("1\n"))
	w.Close()
	defer func() { os.Stdin = originalStdin }()

	var output bytes.Buffer
	outputWriter = &output
	defer func() { outputWriter = os.Stdout }()

	rootCmd.SetOut(&output)
	rootCmd.SetArgs([]string{"use", "mock-profile"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	outStr := output.String()

	if !strings.Contains(outStr, "Available clusters in region eu-west-1") {
		t.Errorf("Expected region info, got: %s", outStr)
	}
	if !strings.Contains(outStr, "Updating kubeconfig for cluster: cluster-one") {
		t.Errorf("Expected kubeconfig update message, got: %s", outStr)
	}
	if !strings.Contains(outStr, "Successfully updated kubeconfig for cluster: cluster-one") {
		t.Errorf("Expected kubeconfig update confirmation, got: %s", outStr)
	}
}
