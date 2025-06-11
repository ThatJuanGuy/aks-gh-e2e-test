package dnscheck

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Azure/cluster-health-monitor/pkg/config"
)

func TestBuildDNSChecker(t *testing.T) {
	for _, tc := range []struct {
		name        string
		checkerName string
		dnsConfig   *config.DNSConfig
		validateRes func(g *WithT, checker *DNSChecker, err error)
	}{
		{
			name:        "Valid config",
			checkerName: "test-dns-checker",
			dnsConfig: &config.DNSConfig{
				Domain: "example.com",
			},
			validateRes: func(g *WithT, checker *DNSChecker, err error) {
				g.Expect(checker).To(Equal(
					&DNSChecker{
						name: "test-dns-checker",
						config: &config.DNSConfig{
							Domain: "example.com",
						},
					}))
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			name:        "Empty Checker Name",
			checkerName: "",
			dnsConfig: &config.DNSConfig{
				Domain: "example.com",
			},
			validateRes: func(g *WithT, checker *DNSChecker, err error) {
				g.Expect(checker).To(BeNil())
				g.Expect(err).To(HaveOccurred())
			},
		},
		{
			name:        "Missing DNSConfig",
			checkerName: "test-dns-checker",
			dnsConfig:   nil,
			validateRes: func(g *WithT, checker *DNSChecker, err error) {
				g.Expect(checker).To(BeNil())
				g.Expect(err).To(HaveOccurred())
			},
		},
		{
			name:        "Empty Domain",
			checkerName: "test-dns-checker",
			dnsConfig: &config.DNSConfig{
				Domain: "",
			},
			validateRes: func(g *WithT, checker *DNSChecker, err error) {
				g.Expect(checker).To(BeNil())
				g.Expect(err).To(HaveOccurred())
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			checker, err := BuildDNSChecker(tc.checkerName, tc.dnsConfig)
			tc.validateRes(g, checker, err)
		})
	}
}

func TestGetCoreDNSServiceIP(t *testing.T) {
	for _, tc := range []struct {
		name           string
		setupClientset func() *fake.Clientset
		validateTarget func(*WithT, DNSTarget, error)
	}{
		{
			name: "Success",
			setupClientset: func() *fake.Clientset {
				return fake.NewSimpleClientset(&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-dns",
						Namespace: "kube-system",
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "10.96.0.10",
					},
				})
			},
			validateTarget: func(g *WithT, target DNSTarget, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(target).To(Equal(DNSTarget{IP: "10.96.0.10", Type: CoreDNSService}))
			},
		},
		{
			name: "Error when service does not exist",
			setupClientset: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			validateTarget: func(g *WithT, target DNSTarget, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(target).To(Equal(DNSTarget{}))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			target, err := GetCoreDNSServiceIP(context.Background(), tc.setupClientset())
			tc.validateTarget(g, target, err)
		})
	}
}
