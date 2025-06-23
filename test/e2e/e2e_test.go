// Package e2e contains end-to-end tests for the cluster health monitor.
package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	clusterName    = "chm-e2e-test"
	namespace      = "cluster-health-monitor"
	deploymentName = "cluster-health-monitor"
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
})
