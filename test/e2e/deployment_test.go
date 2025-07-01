// Package e2e contains end-to-end tests for the cluster health monitor.
package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cluster health monitor deployment", func() {
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
