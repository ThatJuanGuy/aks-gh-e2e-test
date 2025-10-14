// Package e2e contains end-to-end tests for the cluster health monitor.
package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	// Expected DNS checkers.
	// Note that these checkers must match with the configmap in manifests/overlays/test.
	coreDNSPerPodCheckerNames = []string{"TestInternalCoreDNSPerPod", "TestExternalCoreDNSPerPod"}
)

var _ = Describe("DNS per pod checker metrics", Ordered, ContinueOnFailure, func() {
	var (
		session   *gexec.Session
		localPort int
	)

	BeforeAll(func() {
		session, localPort = setupMetricsPortforwarding(clientset)
	})

	AfterAll(func() {
		safeSessionKill(session)
	})

	It("should report healthy status for CoreDNSPerPod checkers", func() {
		By("Waiting for CoreDNSPerPod checker metrics to report healthy status")
		Eventually(func() bool {
			matched, foundCheckers := verifyCoreDNSPodCheckerResultMetrics(localPort, coreDNSPerPodCheckerNames, checkerTypeDNS, metricsHealthyStatus, metricsHealthyErrorCode)
			if !matched {
				GinkgoWriter.Printf("Expected CoreDNSPerPod checkers to be healthy: %v, found: %v\n", coreDNSPerPodCheckerNames, foundCheckers)
				return false
			}
			GinkgoWriter.Printf("Found healthy CoreDNSPerPod checker metric for %v\n", foundCheckers)
			return true
		}, "60s", "5s").Should(BeTrue(), "CoreDNSPerPod checker metrics did not report healthy status within the timeout period")
	})
})
