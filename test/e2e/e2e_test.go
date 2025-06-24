// Package e2e contains end-to-end tests for the cluster health monitor.
package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/prometheus/common/expfmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

const (
	namespace            = "kube-system"
	deploymentName       = "cluster-health-monitor"
	metricsPort          = 9800
	checkerConfigMapName = "cluster-health-monitor-config"
)

var (
	clientset          *kubernetes.Clientset
	skipClusterSetup   = os.Getenv("E2E_SKIP_CLUSTER_SETUP") == "true"
	skipClusterCleanup = os.Getenv("E2E_SKIP_CLUSTER_CLEANUP") == "true"
)

func beforeSuiteAllProcesses() []byte {
	By("Getting kubeconfig path from KUBECONFIG or defaulting to $(HOME)/.kube/config")
	kubeConfigPath := os.Getenv("KUBECONFIG")
	if kubeConfigPath == "" {
		kubeConfigPath = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	GinkgoWriter.Println("Using kubeconfig:", kubeConfigPath)

	if skipClusterSetup {
		By("Applying the cluster health monitor deployment")
		cmd := exec.Command("make", "kind-apply-manifests")
		output, err := run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply cluster health monitor manifests: %s", string(output))
		GinkgoWriter.Println(string(output))
	} else {
		By("Setting up a Kind cluster for E2E")
		cmd := exec.Command("make", "kind-test-local")
		output, err := run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to setup Kind cluster for E2E: %s", string(output))
		GinkgoWriter.Println(string(output))
	}

	// Initialize Kubernetes client.
	clientset, err := getKubeClient(kubeConfigPath)
	Expect(err).NotTo(HaveOccurred())

	By("Waiting for CoreDNS pods to be running")
	Eventually(func() bool {
		podList, err := clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{
			LabelSelector: "k8s-app=kube-dns",
		})
		if err != nil {
			return false
		}
		for _, pod := range podList.Items {
			if pod.Status.Phase != "Running" || pod.Status.PodIP == "" {
				return false
			}
		}
		return len(podList.Items) > 0
	}, "180s", "2s").Should(BeTrue(), "CoreDNS pods are not running")

	By("Waiting for cluster health monitor deployment to become ready")
	Eventually(func() bool {
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
	}, "90s", "2s").Should(BeTrue())

	By("Listing all pods in all namespaces")
	cmd := exec.Command("kubectl", "get", "po", "-A")
	output, err := run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to list pods: %s", string(output))
	GinkgoWriter.Println(string(output))

	return []byte(kubeConfigPath)
}

var _ = SynchronizedBeforeSuite(beforeSuiteAllProcesses, func(kubeConfigPath []byte) {
	var err error
	clientset, err = getKubeClient(string(kubeConfigPath))
	Expect(err).NotTo(HaveOccurred())
})

func afterSuiteAllProcesses() {
	if skipClusterCleanup {
		By("Deleting the test deployment")
		cmd := exec.Command("make", "kind-delete-deployment")
		output, err := run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete deployment: %s", string(output))
		GinkgoWriter.Println(string(output))
		return
	}

	By("Deleting the Kind cluster")
	cmd := exec.Command("make", "kind-delete-cluster")
	output, err := run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to delete Kind cluster: %s", string(output))
	GinkgoWriter.Println(string(output))
}

var _ = SynchronizedAfterSuite(func() {}, afterSuiteAllProcesses)

var _ = Describe("Cluster health monitor deployment", func() {
	It("should have the cluster health monitor pod running", func() {
		By("Checking if the cluster health monitor deployment is available")
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deployment.Status.AvailableReplicas).To(Equal(*deployment.Spec.Replicas))

		By("Checking if the cluster health monitor pod is running")
		podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=" + deploymentName,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(podList.Items).NotTo(BeEmpty())
		pod := podList.Items[0]
		Expect(pod.Status.Phase).To(BeEquivalentTo("Running"), "Pod %s should be in Running state", pod.Name)
		for _, containerStatus := range pod.Status.ContainerStatuses {
			Expect(containerStatus.Ready).To(BeTrue(), "Container %s in pod %s should be ready", containerStatus.Name, pod.Name)
		}
	})

	It("should expose metrics endpoint", func() {
		By("Port-forwarding to the cluster health monitor pod")
		podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=" + deploymentName,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(podList.Items).NotTo(BeEmpty())
		pod := podList.Items[0]
		cmd := exec.Command("kubectl", "port-forward",
			fmt.Sprintf("pod/%s", pod.Name),
			fmt.Sprintf("%d:%d", metricsPort, metricsPort),
			"-n", namespace)
		cmd.Env = os.Environ()
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer session.Kill()
		GinkgoWriter.Printf("Port-forwarding to pod %s in namespace %s on port %d\n", pod.Name, namespace, metricsPort)
		Eventually(session, "5s", "1s").Should(gbytes.Say("Forwarding from"), "Failed to establish port-forwarding")

		By("Waiting for metrics endpoint to contain data")
		var body []byte
		Eventually(func() bool {
			By("Checking if metrics endpoint is accessible")
			res, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", metricsPort))
			Expect(err).NotTo(HaveOccurred(), "Failed to access metrics endpoint")
			defer func() {
				if err := res.Body.Close(); err != nil {
					GinkgoWriter.Printf("Failed to close response body: %v\n", err)
				}
			}()
			Expect(res.StatusCode).To(Equal(http.StatusOK), "Metrics endpoint did not return 200 OK")

			By("Reading metrics response body")
			body, err = io.ReadAll(res.Body)
			Expect(err).NotTo(HaveOccurred(), "Failed to read metrics response body")
			return len(body) > 0
		}, "30s", "10s").Should(BeTrue(), "Metrics response body is empty")

		By("Parsing metrics response body")
		var parser expfmt.TextParser
		metrics, err := parser.TextToMetricFamilies(strings.NewReader(string(body)))
		Expect(err).NotTo(HaveOccurred(), "Failed to parse metrics")
		metric := metrics["cluster_health_monitor_checker_result_total"]
		Expect(metric).NotTo(BeNil(), "Expected cluster_health_monitor_checker_result_total metric not found", string(body))

		// TODO: Check if the metric has expected labels and values
	})
})
