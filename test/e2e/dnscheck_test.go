// Package e2e contains end-to-end tests for the cluster health monitor.
package e2e

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const (
	checkerTypeDNS             = "dns"
	dnsPodsNotReadyErrorCode   = "pods_not_ready"
	dnsDelayDuration           = 5 * time.Second
	dnsServiceTimeoutErrorCode = "service_timeout"
)

var (
	// Expected DNS checkers.
	// Note that these checkers must match with the configmap in manifests/overlays/test.
	dnsCheckerNames = []string{"test-internal-dns-checker", "test-external-dns-checker"}
)

var _ = Describe("DNS checker metrics", func() {
	var (
		session   *gexec.Session
		localPort int
	)

	BeforeEach(func() {
		session, localPort = setupMetricsPortforwarding(clientset)
	})

	AfterEach(func() {
		if session != nil {
			session.Kill()
		}
	})

	It("should report healthy status for all DNS checkers", func() {
		By("Waiting for DNS checker metrics to report healthy status")
		Eventually(func() bool {
			metricsData, err := getMetrics(localPort)
			if err != nil {
				GinkgoWriter.Printf("Failed to get metrics: %v\n", err)
				return false
			}

			matched, foundCheckers := verifyCheckerResultMetrics(metricsData, dnsCheckerNames, checkerTypeDNS, metricsHealthyStatus, metricsHealthyErrorCode)
			if !matched {
				GinkgoWriter.Printf("Expected DNS checkers to be healthy: %v, found: %v\n", dnsCheckerNames, foundCheckers)
				return false
			}
			GinkgoWriter.Printf("Found healthy DNS checker metric for %v\n", foundCheckers)
			return true
		}, "30s", "5s").Should(BeTrue(), "DNS checker metrics did not report healthy status within the timeout period")
	})

	It("should report unhealthy status for all DNS checkers when CoreDNS pods are not ready", func() {
		By("Getting the CoreDNS deployment")
		deployment, err := getCoreDNSDeployment(clientset)
		Expect(err).NotTo(HaveOccurred(), "Failed to get CoreDNS deployment")
		originalReplicas := *deployment.Spec.Replicas

		By("Scaling down CoreDNS deployment to 0 replicas to simulate unhealthy state")
		err = updateCoreDNSDeploymentReplicas(clientset, 0)
		Expect(err).NotTo(HaveOccurred(), "Failed to scale down CoreDNS deployment")

		DeferCleanup(func() {
			By("Restoring CoreDNS deployment to original replica count")
			err := updateCoreDNSDeploymentReplicas(clientset, originalReplicas)
			Expect(err).NotTo(HaveOccurred(), "Failed to restore CoreDNS deployment")

			By("Waiting for CoreDNS pods to be ready again")
			Eventually(func() bool {
				deployment, err := getCoreDNSDeployment(clientset)
				if err != nil {
					return false
				}
				return deployment.Status.ReadyReplicas == originalReplicas
			}, "60s", "5s").Should(BeTrue(), "CoreDNS pods did not return to ready state")
		})

		By("Waiting for all CoreDNS pods to terminate")
		Eventually(func() bool {
			deployment, err := getCoreDNSDeployment(clientset)
			if err != nil {
				return false
			}
			return deployment.Status.AvailableReplicas == 0
		}, "30s", "2s").Should(BeTrue(), "Not all CoreDNS pods terminated")

		By("Waiting for DNS checker metrics to report unhealthy status with pods not ready")
		Eventually(func() bool {
			metricsData, err := getMetrics(localPort)
			if err != nil {
				GinkgoWriter.Printf("Failed to get metrics: %v\n", err)
				return false
			}

			matched, foundCheckers := verifyCheckerResultMetrics(metricsData, dnsCheckerNames, checkerTypeDNS, metricsUnhealthyStatus, dnsPodsNotReadyErrorCode)
			if !matched {
				GinkgoWriter.Printf("Expected DNS checkers to be unhealthy and pods not ready: %v, found: %v\n", dnsCheckerNames, foundCheckers)
				return false
			}
			GinkgoWriter.Printf("Found unhealthy and pods not ready DNS checker metric for %v\n", foundCheckers)
			return true
		}, "30s", "5s").Should(BeTrue(), "DNS checker metrics did not report unhealthy status and pods not ready within the timeout period")
	})

	It("should report unhealthy status for all DNS checkers when DNS service has high latency", func() {
		By("Adding global delay to all DNS queries")
		originalCorefile, err := addGlobalDNSDelay(clientset, dnsDelayDuration)
		Expect(err).NotTo(HaveOccurred(), "Failed to add global DNS delay")

		By("Restarting CoreDNS pods to apply the delay")
		err = restartCoreDNSPods(clientset)
		Expect(err).NotTo(HaveOccurred(), "Failed to restart CoreDNS pods with original configuration")

		DeferCleanup(func() {
			By("Restoring the original CoreDNS ConfigMap")
			err := restoreCoreDNSConfigMap(clientset, originalCorefile)
			Expect(err).NotTo(HaveOccurred(), "Failed to restore CoreDNS ConfigMap")

			By("Restarting CoreDNS pods to apply the original configuration")
			err = restartCoreDNSPods(clientset)
			Expect(err).NotTo(HaveOccurred(), "Failed to restart CoreDNS pods with original configuration")
		})

		By("Waiting for DNS checker metrics to report unhealthy status with service timeout")
		Eventually(func() bool {
			metricsData, err := getMetrics(localPort)
			if err != nil {
				GinkgoWriter.Printf("Failed to get metrics: %v\n", err)
				return false
			}

			matched, foundCheckers := verifyCheckerResultMetrics(metricsData, dnsCheckerNames, checkerTypeDNS, metricsUnhealthyStatus, dnsServiceTimeoutErrorCode)
			if !matched {
				GinkgoWriter.Printf("Expected DNS checkers to be unhealthy and service timeout: %v, found: %v\n", dnsCheckerNames, foundCheckers)
				return false
			}
			GinkgoWriter.Printf("Found unhealthy and service timeout DNS checker metric for %v\n", foundCheckers)
			return true
		}, "60s", "5s").Should(BeTrue(), "DNS checker metrics did not report unhealthy status and service timeout within the timeout period")
	})
})
