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
	"k8s.io/client-go/tools/clientcmd"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

const (
	kubeConfigPathEnvVarName  = "KUBECONFIG"
	kindClusterNameEnvVarName = "KIND_CLUSTER_NAME"

	clusterName          = "chm-e2e-test"
	namespace            = "cluster-health-monitor"
	deploymentName       = "cluster-health-monitor"
	metricsPort          = 9800
	checkerConfigMapName = "cluster-health-monitor-config"
)

var (
	clientset *kubernetes.Clientset
	testDir   string
)

func beforeSuiteAllProcesses() {
	// Create a temporary directory for test artifacts.
	testDir, err := os.MkdirTemp("", "cluster-health-monitor-e2e-")
	Expect(err).NotTo(HaveOccurred())

	// Ensure environment variables are set.
	kubeConfigPath := filepath.Join(testDir, "kubeconfig")
	os.Setenv(kubeConfigPathEnvVarName, kubeConfigPath)
	Expect(os.Getenv(kubeConfigPathEnvVarName)).To(Equal(kubeConfigPath), "Environment variable KUBECONFIG is not set")
	os.Setenv(kindClusterNameEnvVarName, clusterName)
	Expect(os.Getenv(kindClusterNameEnvVarName)).To(Equal(clusterName), "Environment variable KIND_CLUSTER_NAME is not set")

	By("Setting up a Kind cluster for E2E")
	cmd := exec.Command("make", "kind-test-local")
	output, err := run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to setup Kind cluster for E2E: %s", string(output))
	GinkgoWriter.Println(string(output))

	// Initialize Kubernetes client.
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	Expect(err).NotTo(HaveOccurred())
	clientset, err = kubernetes.NewForConfig(config)
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
	cmd = exec.Command("kubectl", "get", "po", "-A")
	output, err = run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to list pods: %s", string(output))
	GinkgoWriter.Println(string(output))
}

var _ = SynchronizedBeforeSuite(beforeSuiteAllProcesses, func() {})

func afterSuiteAllProcesses() {
	By("Deleting the Kind cluster")
	cmd := exec.Command("make", "kind-delete-cluster")
	output, err := run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to delete Kind cluster: %s", string(output))
	GinkgoWriter.Println(string(output))

	err = os.RemoveAll(testDir)
	Expect(err).NotTo(HaveOccurred(), "Failed to remove test directory: %s", testDir)
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
		Eventually(session, "5s", "1s").Should(gbytes.Say("Forwarding from"), "Failed to establish port-forwarding")

		By("Waiting for metrics endpoint to contain data")
		var body []byte
		Eventually(func() bool {
			By("Checking if metrics endpoint is accessible")
			res, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", metricsPort))
			Expect(err).NotTo(HaveOccurred(), "Failed to access metrics endpoint")
			defer res.Body.Close()
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
	})
})
