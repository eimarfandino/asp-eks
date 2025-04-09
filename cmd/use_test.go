package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

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
	case strings.Contains(cmdStr, "sso login"):
		fmt.Fprint(os.Stdout, "SSO login simulated")
	case strings.Contains(cmdStr, "configure get region"):
		fmt.Fprint(os.Stdout, "eu-west-1")
	case strings.Contains(cmdStr, "eks list-clusters"):
		fmt.Fprint(os.Stdout, "cluster-one\tcluster-two")
	case strings.Contains(cmdStr, "update-kubeconfig"):
		fmt.Fprint(os.Stdout, "kubeconfig updated")
	}

	os.Exit(0)
}

func TestUseCommand_MockAWS(t *testing.T) {
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

	if !strings.Contains(outStr, "SSO login simulated") {
		t.Errorf("Expected SSO login output, got: %s", outStr)
	}
	if !strings.Contains(outStr, "Available clusters in region eu-west-1") {
		t.Errorf("Expected region info, got: %s", outStr)
	}
	if !strings.Contains(outStr, "Updating kubeconfig for cluster: cluster-one") {
		t.Errorf("Expected kubeconfig update message, got: %s", outStr)
	}
	if !strings.Contains(outStr, "kubeconfig updated") {
		t.Errorf("Expected confirmation of update, got: %s", outStr)
	}
}
