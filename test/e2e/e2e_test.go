// Package e2e contains end-to-end tests for the cluster health monitor.
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

const (
	// Kubernetes resource names.
	// Note that these names must match with the applied manifests/overlays/test.
	namespace            = "kube-system"
	deploymentName       = "cluster-health-monitor"
	checkerConfigMapName = "cluster-health-monitor-config"

	remoteMetricsPort       = 9800  // remoteMetricsPort is the fixed port used by the service in the container.
	baseLocalPort           = 10000 // baseLocalPort is the base local port for dynamic allocation.
	checkerResultMetricName = "cluster_health_monitor_checker_result_total"
	metricsCheckerTypeLabel = "checker_type"
	metricsCheckerNameLabel = "checker_name"
	metricsStatusLabel      = "status"
	metricsErrorCodeLabel   = "error_code"

	metricsHealthyStatus    = "healthy"
	metricsHealthyErrorCode = metricsHealthyStatus

	checkerTypeDNS = "dns"
)

var (
	clientset          *kubernetes.Clientset
	skipClusterSetup   = os.Getenv("E2E_SKIP_CLUSTER_SETUP") == "true"
	skipClusterCleanup = os.Getenv("E2E_SKIP_CLUSTER_CLEANUP") == "true"

	// Expected checkers.
	// Note that these checkers must match with the configmap in manifests/overlays/test.
	dnsCheckerNames = map[string]struct{}{"test-internal-dns-checker": {}, "test-external-dns-checker": {}}
	// TODO: Add pod startup checker.
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

var _ = Describe("Cluster health monitor", func() {
	Context("deployment", func() {
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
	})

	Context("metrics endpoint", func() {
		var (
			session   *gexec.Session
			localPort int
		)

		BeforeEach(func() {
			By("Getting the cluster health monitor pod")
			podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
				LabelSelector: "app=" + deploymentName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).NotTo(BeEmpty())
			pod := podList.Items[0]

			By("Finding an available local port for metrics")
			localPort, err = getUnusedPort(baseLocalPort)
			Expect(err).NotTo(HaveOccurred(), "Failed to get unused port")
			GinkgoWriter.Printf("Using local port %d for metrics endpoint\n", localPort)

			By("Port-forwarding to the cluster health monitor pod")
			cmd := exec.Command("kubectl", "port-forward",
				fmt.Sprintf("pod/%s", pod.Name),
				fmt.Sprintf("%d:%d", localPort, remoteMetricsPort),
				"-n", namespace)
			cmd.Env = os.Environ()
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Port-forwarding to pod %s in namespace %s on port %d:%d\n", pod.Name, namespace, localPort, remoteMetricsPort)
			Eventually(session, "5s", "1s").Should(gbytes.Say("Forwarding from"), "Failed to establish port-forwarding")
		})

		AfterEach(func() {
			if session != nil {
				session.Kill()
			}
		})

		It("should provide metrics data", func() {
			By("Waiting for metrics endpoint to contain data")
			Eventually(func() bool {
				metricsData, err := getMetrics(localPort)
				if err != nil {
					GinkgoWriter.Printf("Error getting metrics: %v\n", err)
					return false
				}
				return len(metricsData) > 0
			}, "30s", "10s").Should(BeTrue(), "No metrics data found within timeout period")

			By("Verifying metrics endpoint contains expected metrics")
			metricsData, err := getMetrics(localPort)
			Expect(err).NotTo(HaveOccurred(), "Failed to get metrics")
			Expect(metricsData).To(HaveKey(checkerResultMetricName), "Expected %s metric not found", checkerResultMetricName)
		})

		Context("DNS checker", func() {
			It("should include all configured DNS checkers", func() {
				By("Waiting for all DNS checker metrics to appear")
				Eventually(func() bool {
					metricsData, err := getMetrics(localPort)
					if err != nil {
						GinkgoWriter.Printf("Failed to get metrics: %v\n", err)
						return false
					}
					metricFamily, found := metricsData[checkerResultMetricName]
					if !found {
						return false
					}

					// Check if all DNS checkers are present in the metrics.
					foundDNSCheckers := make(map[string]bool)
					for _, m := range metricFamily.Metric {
						labels := make(map[string]string)
						for _, label := range m.Label {
							labels[label.GetName()] = label.GetValue()
						}

						if labels[metricsCheckerTypeLabel] == checkerTypeDNS {
							checkerName := labels[metricsCheckerNameLabel]
							if _, ok := dnsCheckerNames[checkerName]; ok {
								foundDNSCheckers[checkerName] = true
								GinkgoWriter.Printf("Found DNS checker metric for %s\n", checkerName)
							}
						}
					}

					if len(foundDNSCheckers) != len(dnsCheckerNames) {
						GinkgoWriter.Printf("Expected %d DNS checkers, found %d: %v\n", len(dnsCheckerNames), len(foundDNSCheckers), foundDNSCheckers)
						return false
					}

					return true
				}, "30s", "5s").Should(BeTrue(), "DNS checker metrics were not found within the timeout period")
			})

			It("should report healthy status for all DNS checkers", func() {
				By("Waiting for DNS checker metrics to report healthy status")
				Eventually(func() bool {
					metricsData, err := getMetrics(localPort)
					if err != nil {
						GinkgoWriter.Printf("Failed to get metrics: %v\n", err)
						return false
					}
					metricFamily, found := metricsData[checkerResultMetricName]
					if !found {
						return false
					}

					// Check if all DNS checkers report healthy status.
					healthyDNSCheckers := make(map[string]bool)
					for _, m := range metricFamily.Metric {
						labels := make(map[string]string)
						for _, label := range m.Label {
							labels[label.GetName()] = label.GetValue()
						}

						if labels[metricsCheckerTypeLabel] == checkerTypeDNS &&
							labels[metricsStatusLabel] == metricsHealthyStatus &&
							labels[metricsErrorCodeLabel] == metricsHealthyErrorCode {
							GinkgoWriter.Printf("Found healthy DNS checker metric for %s\n", labels[metricsCheckerNameLabel])
							healthyDNSCheckers[labels[metricsCheckerNameLabel]] = true
						}
					}

					if len(healthyDNSCheckers) != len(dnsCheckerNames) {
						GinkgoWriter.Printf("Expected %d DNS checkers to be healthy, found %d: %v\n", len(dnsCheckerNames), len(healthyDNSCheckers), healthyDNSCheckers)
						return false
					}

					return true
				}, "30s", "5s").Should(BeTrue(), "DNS checker metrics did not report healthy status within the timeout period")
			})

			// TODO: Add case for DNS checker failure scenarios.
		})

		// TODO: Add pod startup checker tests.
	})
})
