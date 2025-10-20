package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// Kubernetes resource names.
	// Note that these names must match with the applied manifests/overlays/test.
	namespace      = "kube-system"
	deploymentName = "cluster-health-monitor"

	remoteMetricsPort = 9800  // remoteMetricsPort is the fixed port used by the service in the container.
	baseLocalPort     = 10000 // baseLocalPort is the base local port for dynamic allocation.

	checkerResultMetricName    = "cluster_health_monitor_checker_result_total"
	coreDNSPodResultMetricName = "cluster_health_monitor_coredns_pod_result_total"
	metricsCheckerTypeLabel    = "checker_type"
	metricsCheckerNameLabel    = "checker_name"
	metricsStatusLabel         = "status"
	metricsErrorCodeLabel      = "error_code"
)

// safeSessionKill is shorthand to kill the provided gexec.Session if it is not nil.
func safeSessionKill(session *gexec.Session) {
	if session != nil {
		session.Kill()
	}
}

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

// getDynamicKubeClient creates a Kubernetes dynamic client using the provided kubeconfig path.
func getDynamicKubeClient(kubeConfigPath string) (dynamic.Interface, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build Kubernetes config: %w", err)
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes dynamic client: %w", err)
	}
	return dynamicClient, nil
}

func getClusterHealthMonitorDeployment(clientset *kubernetes.Clientset) (*appsv1.Deployment, error) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster health monitor deployment: %w", err)
	}
	return deployment, nil
}

func getClusterHealthMonitorPod(clientset *kubernetes.Clientset) (*corev1.Pod, error) {
	podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + deploymentName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster health monitor pod: %w", err)
	}
	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("no cluster health monitor pod found")
	}
	return &podList.Items[0], nil
}

// setupMetricsPortforwarding sets up port-forwarding to the cluster health monitor pod's metrics endpoint.
// It returns the session and the local port used for port-forwarding.
func setupMetricsPortforwarding(clientset *kubernetes.Clientset) (*gexec.Session, int) {
	By("Getting the cluster health monitor pod")
	pod, err := getClusterHealthMonitorPod(clientset)
	Expect(err).NotTo(HaveOccurred())

	By("Finding an available local port for metrics")
	localPort, err := getUnusedPort(baseLocalPort)
	Expect(err).NotTo(HaveOccurred(), "Failed to get unused port")
	GinkgoWriter.Printf("Using local port %d for metrics endpoint\n", localPort)

	By("Port-forwarding to the cluster health monitor pod")
	cmd := exec.Command("kubectl", "port-forward",
		fmt.Sprintf("pod/%s", pod.Name),
		fmt.Sprintf("%d:%d", localPort, remoteMetricsPort),
		"-n", namespace)
	cmd.Env = os.Environ()
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	GinkgoWriter.Printf("Port-forwarding to pod %s in namespace %s on port %d:%d\n", pod.Name, namespace, localPort, remoteMetricsPort)
	Eventually(session, "5s", "1s").Should(gbytes.Say("Forwarding from"), "Failed to establish port-forwarding")

	return session, localPort
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

// getUnusedPort generates a port number that is likely to be unique for parallel tests.
func getUnusedPort(basePort int) (int, error) {
	processID := GinkgoParallelProcess()
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

// getCoreDNSPodList lists all CoreDNS pods in the kube-system namespace.
func getCoreDNSPodList(clientset *kubernetes.Clientset) (*corev1.PodList, error) {
	podList, err := clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "k8s-app=kube-dns",
	})
	if err != nil {
		return nil, err
	}
	return podList, nil
}

// deleteCoreDNSPods deletes the CoreDNS pods.
func deleteCoreDNSPods(clientset *kubernetes.Clientset) error {
	podList, err := getCoreDNSPodList(clientset)
	if err != nil {
		return fmt.Errorf("failed to get CoreDNS pod list: %w", err)
	}

	for _, pod := range podList.Items {
		err := clientset.CoreV1().Pods("kube-system").Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete CoreDNS pod %s: %w", pod.Name, err)
		}
	}
	return nil
}

// getCoreDNSConfigMap gets the CoreDNS ConfigMap from the kube-system namespace.
func getCoreDNSConfigMap(clientset *kubernetes.Clientset) (*corev1.ConfigMap, error) {
	return clientset.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "coredns", metav1.GetOptions{})
}

// simulateCoreDNSHighLatency simulates high latency in DNS responses by modifying the CoreDNS ConfigMap.
// It adds an invalid plugin directive to the Corefile to crash new CoreDNS pods, simulating high latency for the DNS service.
// This is a workaround for testing purposes and should not be used in production.
// The existing CoreDNS pods must be deleted to apply the changes.
// It returns the original Corefile content so it can be restored later.
func simulateCoreDNSHighLatency(clientset *kubernetes.Clientset) (string, error) {
	configMap, err := getCoreDNSConfigMap(clientset)
	if err != nil {
		return "", fmt.Errorf("failed to get CoreDNS ConfigMap: %w", err)
	}

	originalCorefile := configMap.Data["Corefile"]
	modifiedCorefile := strings.Replace(originalCorefile, "cache", "invalidplugin\n    cache", 1)
	configMap.Data["Corefile"] = modifiedCorefile

	_, err = clientset.CoreV1().ConfigMaps("kube-system").Update(context.TODO(), configMap, metav1.UpdateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to update CoreDNS ConfigMap: %w", err)
	}
	return originalCorefile, nil
}

// restoreCoreDNSConfigMap restores the original CoreDNS ConfigMap.
func restoreCoreDNSConfigMap(clientset *kubernetes.Clientset, originalCorefile string) error {
	configMap, err := getCoreDNSConfigMap(clientset)
	if err != nil {
		return fmt.Errorf("failed to get CoreDNS ConfigMap: %w", err)
	}

	configMap.Data["Corefile"] = originalCorefile
	_, err = clientset.CoreV1().ConfigMaps("kube-system").Update(context.TODO(), configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to restore CoreDNS ConfigMap: %w", err)
	}
	return nil
}

// isMockLocalDNSAvailable checks if the mock LocalDNS server is available.
// It checks the resources deployed from the manifests/overlays/test/dnsmasq.yaml file.
func isMockLocalDNSAvailable(clientset *kubernetes.Clientset) bool {
	mockLocalDNS, err := clientset.AppsV1().DaemonSets("kube-system").Get(context.TODO(), "mock-localdns", metav1.GetOptions{})
	if err != nil {
		GinkgoWriter.Printf("Error getting mock-dns daemonset: %v\n", err)
		return false
	}

	return mockLocalDNS.Status.NumberAvailable == mockLocalDNS.Status.DesiredNumberScheduled
}

// applyYAMLFile applies a YAML file to the Kubernetes cluster using the provided dynamic client.
func applyYAMLFile(dynamicClient dynamic.Interface, yamlPath string) error {
	// Read the YAML file
	yamlData, err := os.ReadFile(yamlPath)
	if err != nil {
		return fmt.Errorf("failed to read YAML file %s: %w", yamlPath, err)
	}

	// Split YAML documents
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(yamlData)), 4096)

	for {
		var rawObj runtime.RawExtension
		if err := decoder.Decode(&rawObj); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode YAML: %w", err)
		}

		obj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(rawObj.Raw, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to decode object: %w", err)
		}

		unstructuredObj := obj.(*unstructured.Unstructured)

		// Get the resource mapping for this object
		gvr, err := getGVRForObject(gvk)
		if err != nil {
			return fmt.Errorf("failed to get GVR for object: %w", err)
		}

		// Apply the object
		namespace := unstructuredObj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}

		_, err = dynamicClient.Resource(gvr).Namespace(namespace).Create(
			context.TODO(), unstructuredObj, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create resource %s/%s: %w",
				unstructuredObj.GetKind(), unstructuredObj.GetName(), err)
		}
	}

	return nil
}

// deleteYAMLFile deletes resources from a YAML file from the Kubernetes cluster using the provided dynamic client.
func deleteYAMLFile(dynamicClient dynamic.Interface, yamlPath string) error {
	// Read the YAML file
	yamlData, err := os.ReadFile(yamlPath)
	if err != nil {
		return fmt.Errorf("failed to read YAML file %s: %w", yamlPath, err)
	}

	// Split YAML documents
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(yamlData)), 4096)

	for {
		var rawObj runtime.RawExtension
		if err := decoder.Decode(&rawObj); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode YAML: %w", err)
		}

		obj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(rawObj.Raw, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to decode object: %w", err)
		}

		unstructuredObj := obj.(*unstructured.Unstructured)

		// Get the resource mapping for this object
		gvr, err := getGVRForObject(gvk)
		if err != nil {
			return fmt.Errorf("failed to get GVR for object: %w", err)
		}

		// Delete the object
		namespace := unstructuredObj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}

		err = dynamicClient.Resource(gvr).Namespace(namespace).Delete(
			context.TODO(), unstructuredObj.GetName(), metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete resource %s/%s: %w",
				unstructuredObj.GetKind(), unstructuredObj.GetName(), err)
		}
	}

	return nil
}

// getGVRForObject returns the GroupVersionResource for a given GroupVersionKind.
func getGVRForObject(gvk *schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	// Map common kinds to their resource names (plurals)
	kindToResource := map[string]string{
		"DaemonSet": "daemonsets",
		"Pod":       "pods",
		"Service":   "services",
		"ConfigMap": "configmaps",
		// Add more as needed
	}

	resource, ok := kindToResource[gvk.Kind]
	if !ok {
		return schema.GroupVersionResource{}, fmt.Errorf("unknown kind: %s", gvk.Kind)
	}

	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resource,
	}, nil
}

// enableMockLocalDNS applies the mock LocalDNS YAML file (equivalent to kubectl apply -f dnsmasq.yaml).
func enableMockLocalDNS(dynamicClient dynamic.Interface) error {
	gitRoot, err := getGitRoot()
	if err != nil {
		return fmt.Errorf("failed to get git root: %w", err)
	}
	yamlPath := filepath.Join(gitRoot, "manifests", "overlays", "test", "dnsmasq.yaml")
	err = applyYAMLFile(dynamicClient, yamlPath)
	if err != nil {
		return fmt.Errorf("failed to apply mock LocalDNS YAML: %w", err)
	}

	GinkgoWriter.Printf("Successfully enabled mock LocalDNS from %s\n", yamlPath)
	return nil
}

// disableMockLocalDNS deletes the mock LocalDNS resources (equivalent to kubectl delete -f dnsmasq.yaml).
func disableMockLocalDNS(dynamicClient dynamic.Interface) error {
	gitRoot, err := getGitRoot()
	if err != nil {
		return fmt.Errorf("failed to get git root: %w", err)
	}
	yamlPath := filepath.Join(gitRoot, "manifests", "overlays", "test", "dnsmasq.yaml")
	err = deleteYAMLFile(dynamicClient, yamlPath)
	if err != nil {
		return fmt.Errorf("failed to delete mock LocalDNS YAML: %w", err)
	}

	GinkgoWriter.Printf("Successfully disabled mock LocalDNS from %s\n", yamlPath)
	return nil
}

func verifyCoreDNSPodCheckerResultMetrics(localPort int, expectedChkNames []string, expectedType, expectedStatus, expectedErrorCode string) (bool, map[string]struct{}) {
	return verifyCheckerResultMetricsHelper(coreDNSPodResultMetricName, localPort, expectedChkNames, expectedType, expectedStatus, expectedErrorCode, []string{"pod_name"})
}

func verifyCheckerResultMetrics(localPort int, expectedChkNames []string, expectedType, expectedStatus, expectedErrorCode string) (bool, map[string]struct{}) {
	return verifyCheckerResultMetricsHelper(checkerResultMetricName, localPort, expectedChkNames, expectedType, expectedStatus, expectedErrorCode, nil)
}

// verifyCheckerResultMetricsHelper checks if all the checker result metrics match the expected type, status, and error code.
// It returns true if all checker names match the criteria, false otherwise. If expectedErrorCode is an empty string, any error code is accepted.
func verifyCheckerResultMetricsHelper(metricName string, localPort int, expectedChkNames []string, expectedType, expectedStatus, expectedErrorCode string, expectedLabels []string) (bool, map[string]struct{}) {
	metricsData, err := getMetrics(localPort)
	if err != nil {
		GinkgoWriter.Printf("Failed to get metrics: %v\n", err)
		return false, nil
	}

	metricFamily, found := metricsData[metricName]
	if !found {
		GinkgoWriter.Printf("%s not found\n", metricName)
		return false, nil
	}

	// Get checkers reporting the expected type, status, and error code.
	foundCheckers := make(map[string]struct{})
	for _, m := range metricFamily.Metric {
		labels := make(map[string]string)
		for _, label := range m.Label {
			labels[label.GetName()] = label.GetValue()
		}

		if labels[metricsCheckerTypeLabel] == expectedType &&
			labels[metricsStatusLabel] == expectedStatus &&
			(labels[metricsErrorCodeLabel] == expectedErrorCode || expectedErrorCode == "") &&
			containExpectedLabels(labels, expectedLabels) {
			foundCheckers[labels[metricsCheckerNameLabel]] = struct{}{}
		}
	}

	// Check count of expected checkers matching the criteria.
	if len(foundCheckers) != len(expectedChkNames) {
		return false, foundCheckers
	}

	// Verify all expected checkers are present.
	for _, checkerName := range expectedChkNames {
		if _, found := foundCheckers[checkerName]; !found {
			return false, foundCheckers
		}
	}

	return true, foundCheckers
}

func containExpectedLabels(labels map[string]string, expectedLabels []string) bool {
	for _, label := range expectedLabels {
		if _, found := labels[label]; !found {
			return false
		}
	}
	return true
}

// addLabelsToAllNodes applies the given labels to all nodes in the cluster.
func addLabelsToAllNodes(clientset kubernetes.Interface, labels map[string]string) {
	Eventually(func() error {
		nodeList, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list nodes: %w", err)
		}

		// Add labels to all nodes.
		for _, node := range nodeList.Items {
			if node.Labels == nil {
				node.Labels = make(map[string]string)
			}
			for key, value := range labels {
				node.Labels[key] = value
			}

			_, err := clientset.CoreV1().Nodes().Update(context.TODO(), &node, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update node %s: %w", node.Name, err)
			}
			GinkgoWriter.Printf("Node %s: Added labels %v\n", node.Name, labels)
		}

		return nil
	}, "30s", "2s").ShouldNot(HaveOccurred(), "Failed to add labels to nodes")
}

// getMetricsServerDeployment gets the metrics server deployment from the kube-system namespace.
func getMetricsServerDeployment(clientset *kubernetes.Clientset) (*appsv1.Deployment, error) {
	return clientset.AppsV1().Deployments("kube-system").Get(context.TODO(), "metrics-server", metav1.GetOptions{})
}

// updateMetricsServerDeploymentReplicas updates the replica count of the metrics server deployment.
func updateMetricsServerDeploymentReplicas(clientset *kubernetes.Clientset, replicas int32) error {
	deployment, err := getMetricsServerDeployment(clientset)
	if err != nil {
		return fmt.Errorf("failed to get metrics server deployment: %w", err)
	}

	deployment.Spec.Replicas = &replicas
	_, err = clientset.AppsV1().Deployments("kube-system").Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update metrics server deployment replicas: %w", err)
	}

	return nil
}

// replaceRolePermissions replaces a role's permissions with new ones, returning the original permissions for restoration.
func replaceRolePermissions(clientset kubernetes.Interface, namespace, roleName string, newRules []rbacv1.PolicyRule) ([]rbacv1.PolicyRule, error) {
	role, err := clientset.RbacV1().Roles(namespace).Get(context.TODO(), roleName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get role %s: %w", roleName, err)
	}

	// Backup original rules
	originalRules := make([]rbacv1.PolicyRule, len(role.Rules))
	copy(originalRules, role.Rules)

	// Replace with new rules
	role.Rules = newRules

	_, err = clientset.RbacV1().Roles(namespace).Update(context.TODO(), role, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update role %s: %w", roleName, err)
	}

	GinkgoWriter.Printf("Replaced permissions for role %s in namespace %s\n", roleName, namespace)
	return originalRules, nil
}

// restoreRolePermissions restores a role to its original permissions.
func restoreRolePermissions(clientset kubernetes.Interface, namespace, roleName string, originalRules []rbacv1.PolicyRule) error {
	role, err := clientset.RbacV1().Roles(namespace).Get(context.TODO(), roleName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get role %s: %w", roleName, err)
	}

	role.Rules = originalRules
	_, err = clientset.RbacV1().Roles(namespace).Update(context.TODO(), role, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to restore role %s: %w", roleName, err)
	}

	GinkgoWriter.Printf("Restored permissions for role %s in namespace %s\n", roleName, namespace)
	return nil
}

// checkLabelsExistOnAnyNode checks if the required labels exist on at least one node.
func checkLabelsExistOnAnyNode(clientset kubernetes.Interface, requiredLabels map[string]string) (bool, error) {
	nodeList, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list nodes: %w", err)
	}

	for _, node := range nodeList.Items {
		hasAllLabels := true
		for key, expectedValue := range requiredLabels {
			if actualValue, exists := node.Labels[key]; !exists || (expectedValue != "" && actualValue != expectedValue) {
				hasAllLabels = false
				break
			}
		}
		if hasAllLabels {
			GinkgoWriter.Printf("Found node %s with all required labels: %v\n", node.Name, requiredLabels)
			return true, nil
		}
	}

	GinkgoWriter.Printf("No node found with all required labels: %v\n", requiredLabels)
	return false, nil
}

// ensureLabelsExistOnAtLeastOneNode adds required labels to all nodes if they don't exist on any node.
func ensureLabelsExistOnAtLeastOneNode(clientset kubernetes.Interface, requiredLabels map[string]string) {
	Eventually(func() error {
		labelsExist, err := checkLabelsExistOnAnyNode(clientset, requiredLabels)
		if err != nil {
			return fmt.Errorf("failed to check if required labels exist: %w", err)
		}

		if !labelsExist {
			GinkgoWriter.Printf("Required labels not found on any node, adding to all nodes: %v\n", requiredLabels)
			addLabelsToAllNodes(clientset, requiredLabels)
		}

		return nil
	}, "30s", "2s").ShouldNot(HaveOccurred(), "Failed to ensure required labels exist on at least one node")
}
