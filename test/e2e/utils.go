package e2e

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"

	ginkgo "github.com/onsi/ginkgo/v2"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
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

// getMetrics fetches and parses metrics from the metrics endpoint.
func getMetrics(port int) (map[string]*dto.MetricFamily, error) {
	res, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
	if err != nil {
		return nil, fmt.Errorf("failed to access metrics endpoint: %w", err)
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			fmt.Printf("Failed to close response body: %v\n", err)
		}
	}()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrics endpoint returned status code %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read metrics response body: %w", err)
	}

	var parser expfmt.TextParser
	metrics, err := parser.TextToMetricFamilies(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse metrics: %w", err)
	}

	return metrics, nil
}

// getUniquePort generates a port number that is likely to be unique for parallel tests.
func getUniquePort(basePort int) (int, error) {
	processID := ginkgo.GinkgoParallelProcess()
	portRangeSize := 1000
	initialPort := basePort + (processID * portRangeSize)

	// Try ports in range initialPort to initialPort+portRangeSize
	for port := initialPort; port < initialPort+portRangeSize; port++ {
		addr := fmt.Sprintf("localhost:%d", port)
		conn, err := net.Listen("tcp", addr)
		if err != nil {
			// Port is not available, try the next one.
			continue
		}
		err = conn.Close()
		if err != nil {
			return 0, fmt.Errorf("failed to close listener: %w", err)
		}
		return port, nil
	}

	return 0, fmt.Errorf("no available ports found between %d and %d", initialPort, initialPort+portRangeSize)
}
