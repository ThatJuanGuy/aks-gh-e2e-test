package e2e

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
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
	var lastErr error

	// Retry up to 3 times with a short delay between attempts
	for attempts := 0; attempts < 3; attempts++ {
		if attempts > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		res, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
		if err != nil {
			lastErr = fmt.Errorf("failed to access metrics endpoint (attempt %d): %w", attempts+1, err)
			continue
		}

		defer func() {
			if err := res.Body.Close(); err != nil {
				fmt.Printf("Failed to close response body: %v\n", err)
			}
		}()

		if res.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("metrics endpoint returned status code %d (attempt %d)", res.StatusCode, attempts+1)
			continue
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read metrics response body (attempt %d): %w", attempts+1, err)
			continue
		}

		var parser expfmt.TextParser
		metrics, err := parser.TextToMetricFamilies(strings.NewReader(string(body)))
		if err != nil {
			lastErr = fmt.Errorf("failed to parse metrics (attempt %d): %w", attempts+1, err)
			continue
		}

		return metrics, nil
	}

	return nil, fmt.Errorf("failed to get metrics after multiple attempts: %w", lastErr)
}

// getUniquePort generates a port number that is likely to be unique for parallel tests.
func getUniquePort(basePort int) (int, error) {
	processID := GinkgoParallelProcess()
	initialPort := basePort + (processID * 100)

	// Try ports in range initialPort to initialPort+1000.
	for port := initialPort; port < initialPort+1000; port++ {
		addr := fmt.Sprintf("localhost:%d", port)
		conn, err := net.Listen("tcp", addr)
		if err != nil {
			// Port is not available, try the next one.
			continue
		}
		conn.Close()
		return port, nil
	}

	return 0, fmt.Errorf("no available ports found between %d and %d", initialPort, initialPort+1000)
}
