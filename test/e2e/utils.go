package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// run executes the provided command in the Git root directory and returns its output.
// It uses the current environment variables.
func run(cmd *exec.Cmd) ([]byte, error) {
	dir, _ := getGitRoot()
	cmd.Dir = dir
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}

// getGitRoot retrieves the root directory of the Git repository.
func getGitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get Git root directory: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// getKubeClient creates a Kubernetes clientset using the provided kubeconfig path.
func getKubeClient(kubeConfigPath string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build Kubernetes config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}
	return clientset, nil
}
