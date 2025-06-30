// Package e2e contains end-to-end tests for the cluster health monitor.
package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var (
	clientset          *kubernetes.Clientset
	skipClusterSetup   = os.Getenv("E2E_SKIP_CLUSTER_SETUP") == "true"
	skipClusterCleanup = os.Getenv("E2E_SKIP_CLUSTER_CLEANUP") == "true"

	// Expected checkers.
	// Note that these checkers must match with the configmap in manifests/overlays/test.
	dnsCheckerNames = []string{"test-internal-dns-checker", "test-external-dns-checker"}
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
		podList, err := getCoreDNSPodList(clientset)
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
		deployment, err := getClusterHealthMonitorDeployment(clientset)
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
			deployment, err := getClusterHealthMonitorDeployment(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment.Status.AvailableReplicas).To(Equal(*deployment.Spec.Replicas))

			By("Checking if the cluster health monitor pod is running")
			pod, err := getClusterHealthMonitorPod(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Status.Phase).To(BeEquivalentTo("Running"), "Pod %s should be in Running state", pod.Name)
			for _, containerStatus := range pod.Status.ContainerStatuses {
				Expect(containerStatus.Ready).To(BeTrue(), "Container %s in pod %s should be ready", containerStatus.Name, pod.Name)
			}
		})
	})
})
