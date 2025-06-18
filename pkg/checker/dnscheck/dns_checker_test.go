package dnscheck

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Azure/cluster-health-monitor/pkg/config"
	"github.com/Azure/cluster-health-monitor/pkg/types"
)

type mockResolver struct {
	lookupHostFunc func(ctx context.Context, host string) ([]string, error)
}

func (m *mockResolver) lookupHost(ctx context.Context, host string) ([]string, error) {
	if m.lookupHostFunc != nil {
		return m.lookupHostFunc(ctx, host)
	}
	return nil, fmt.Errorf("not implemented")
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
			target, err := getCoreDNSServiceIP(context.Background(), tc.setupClientset())
			tc.validateTarget(g, target, err)
		})
	}
}

func TestGetCoreDNSPodIPs(t *testing.T) {
	for _, tc := range []struct {
		name            string
		setupClientset  func() *fake.Clientset
		validateTargets func(*WithT, []DNSTarget, error)
	}{
		{
			name: "Success with ready endpoints",
			setupClientset: func() *fake.Clientset {
				ready := true
				return fake.NewSimpleClientset(
					&discoveryv1.EndpointSlice{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns-12345",
							Namespace: "kube-system",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "kube-dns",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses:  []string{"10.244.0.2", "10.244.0.3"},
								Conditions: discoveryv1.EndpointConditions{Ready: &ready},
							},
						},
					},
				)
			},
			validateTargets: func(g *WithT, targets []DNSTarget, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(targets).To(ConsistOf(
					DNSTarget{IP: "10.244.0.2", Type: CoreDNSPod},
					DNSTarget{IP: "10.244.0.3", Type: CoreDNSPod},
				))
			},
		},
		{
			name: "Success with nil ready condition endpoints",
			setupClientset: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&discoveryv1.EndpointSlice{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns-12345",
							Namespace: "kube-system",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "kube-dns",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses:  []string{"10.244.0.2", "10.244.0.3"},
								Conditions: discoveryv1.EndpointConditions{Ready: nil},
							},
						},
					},
				)
			},
			validateTargets: func(g *WithT, targets []DNSTarget, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(targets).To(ConsistOf(
					DNSTarget{IP: "10.244.0.2", Type: CoreDNSPod},
					DNSTarget{IP: "10.244.0.3", Type: CoreDNSPod},
				))
			},
		},
		{
			name: "Error when endpoints are not ready",
			setupClientset: func() *fake.Clientset {
				ready := false
				return fake.NewSimpleClientset(
					&discoveryv1.EndpointSlice{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns-12345",
							Namespace: "kube-system",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "kube-dns",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses:  []string{"10.244.0.2", "10.244.0.3"},
								Conditions: discoveryv1.EndpointConditions{Ready: &ready},
							},
						},
					},
				)
			},
			validateTargets: func(g *WithT, targets []DNSTarget, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(targets).To(BeNil())
			},
		},
		{
			name: "Error when endpoints do not exist",
			setupClientset: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			validateTargets: func(g *WithT, targets []DNSTarget, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(targets).To(BeNil())
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			targets, err := getCoreDNSPodIPs(context.Background(), tc.setupClientset())
			tc.validateTargets(g, targets, err)
		})
	}
}

func TestRun(t *testing.T) {
	for _, tc := range []struct {
		name           string
		setupClientset func() *fake.Clientset
		setupResolver  func() func(dnsTarget DNSTarget) resolver
		validateResult func(*WithT, *types.Result, error)
	}{
		{
			name: "All DNS servers are healthy",
			setupClientset: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns",
							Namespace: "kube-system",
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "10.96.0.10",
						},
					},
					&discoveryv1.EndpointSlice{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns-12345",
							Namespace: "kube-system",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "kube-dns",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses:  []string{"10.244.0.2", "10.244.0.3"},
								Conditions: discoveryv1.EndpointConditions{Ready: nil},
							},
						},
					},
				)
			},
			setupResolver: func() func(dnsTarget DNSTarget) resolver {
				return func(dnsTarget DNSTarget) resolver {
					return &mockResolver{
						lookupHostFunc: func(ctx context.Context, host string) ([]string, error) {
							return []string{"1.2.3.4"}, nil
						},
					}
				}
			},
			validateResult: func(g *WithT, result *types.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).NotTo(BeNil())
				g.Expect(result.Status).To(Equal(types.StatusHealthy))
			},
		},
		{
			name: "CoreDNS service is unhealthy",
			setupClientset: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns",
							Namespace: "kube-system",
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "10.96.0.10",
						},
					},
					&discoveryv1.EndpointSlice{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns-12345",
							Namespace: "kube-system",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "kube-dns",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses:  []string{"10.244.0.2", "10.244.0.3"},
								Conditions: discoveryv1.EndpointConditions{Ready: nil},
							},
						},
					},
				)
			},
			setupResolver: func() func(dnsTarget DNSTarget) resolver {
				return func(dnsTarget DNSTarget) resolver {
					return &mockResolver{
						lookupHostFunc: func(ctx context.Context, host string) ([]string, error) {
							if dnsTarget.Type == CoreDNSService {
								return nil, fmt.Errorf("connection refused")
							}
							return []string{"1.2.3.4"}, nil
						},
					}
				}
			},
			validateResult: func(g *WithT, result *types.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).NotTo(BeNil())
				g.Expect(result.Status).To(Equal(types.StatusUnhealthy))
				g.Expect(result.Detail.Code).To(Equal("DNS_SERVICE_UNHEALTHY"))
				g.Expect(result.Detail.Message).To(ContainSubstring("DNS service 10.96.0.10 unhealthy"))
			},
		},
		{
			name: "CoreDNS service is healthy but CoreDNS pod is unhealthy",
			setupClientset: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns",
							Namespace: "kube-system",
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "10.96.0.10",
						},
					},
					&discoveryv1.EndpointSlice{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns-12345",
							Namespace: "kube-system",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "kube-dns",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses:  []string{"10.244.0.2", "10.244.0.3"},
								Conditions: discoveryv1.EndpointConditions{Ready: nil},
							},
						},
					},
				)
			},
			setupResolver: func() func(dnsTarget DNSTarget) resolver {
				return func(dnsTarget DNSTarget) resolver {
					return &mockResolver{
						lookupHostFunc: func(ctx context.Context, host string) ([]string, error) {
							if dnsTarget.Type == CoreDNSPod && dnsTarget.IP == "10.244.0.3" {
								return nil, fmt.Errorf("connection refused")
							}
							return []string{"1.2.3.4"}, nil
						},
					}
				}
			},
			validateResult: func(g *WithT, result *types.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).NotTo(BeNil())
				g.Expect(result.Status).To(Equal(types.StatusUnhealthy))
				g.Expect(result.Detail.Code).To(Equal("DNS_POD_UNHEALTHY"))
				g.Expect(result.Detail.Message).To(ContainSubstring("DNS pod 10.244.0.3 unhealthy"))
			},
		},
		{
			name: "Error when CoreDNS service doesn't exist",
			setupClientset: func() *fake.Clientset {
				// No CoreDNS service, only endpointslice
				return fake.NewSimpleClientset(
					&discoveryv1.EndpointSlice{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns-12345",
							Namespace: "kube-system",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "kube-dns",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses:  []string{"10.244.0.2", "10.244.0.3"},
								Conditions: discoveryv1.EndpointConditions{Ready: nil},
							},
						},
					},
				)
			},
			setupResolver: func() func(dnsTarget DNSTarget) resolver {
				return func(dnsTarget DNSTarget) resolver {
					return &mockResolver{
						lookupHostFunc: func(ctx context.Context, host string) ([]string, error) {
							return []string{"1.2.3.4"}, nil
						},
					}
				}
			},
			validateResult: func(g *WithT, result *types.Result, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(result).To(BeNil())
				g.Expect(err.Error()).To(ContainSubstring("failed to get CoreDNS service"))
			},
		},
		{
			name: "Error when CoreDNS service has no ClusterIP",
			setupClientset: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns",
							Namespace: "kube-system",
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "",
						},
					},
					&discoveryv1.EndpointSlice{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns-12345",
							Namespace: "kube-system",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "kube-dns",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses:  []string{"10.244.0.2", "10.244.0.3"},
								Conditions: discoveryv1.EndpointConditions{Ready: nil},
							},
						},
					},
				)
			},
			setupResolver: func() func(dnsTarget DNSTarget) resolver {
				return func(dnsTarget DNSTarget) resolver {
					return &mockResolver{
						lookupHostFunc: func(ctx context.Context, host string) ([]string, error) {
							return []string{"1.2.3.4"}, nil
						},
					}
				}
			},
			validateResult: func(g *WithT, result *types.Result, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(result).To(BeNil())
				g.Expect(err.Error()).To(ContainSubstring("CoreDNS service has no ClusterIP"))
			},
		},
		{
			name: "Error when CoreDNS endpoint slices don't exist",
			setupClientset: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns",
							Namespace: "kube-system",
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "10.96.0.10",
						},
					},
				)
			},
			setupResolver: func() func(dnsTarget DNSTarget) resolver {
				return func(dnsTarget DNSTarget) resolver {
					return &mockResolver{
						lookupHostFunc: func(ctx context.Context, host string) ([]string, error) {
							return []string{"1.2.3.4"}, nil
						},
					}
				}
			},
			validateResult: func(g *WithT, result *types.Result, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(result).To(BeNil())
				g.Expect(err.Error()).To(ContainSubstring("failed to get CoreDNS pod IPs"))
			},
		},
		{
			name: "Error when CoreDNS endpoints are not ready",
			setupClientset: func() *fake.Clientset {
				ready := false
				return fake.NewSimpleClientset(
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns",
							Namespace: "kube-system",
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "10.96.0.10",
						},
					},
					&discoveryv1.EndpointSlice{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-dns-12345",
							Namespace: "kube-system",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "kube-dns",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses:  []string{"10.244.0.2", "10.244.0.3"},
								Conditions: discoveryv1.EndpointConditions{Ready: &ready},
							},
						},
					},
				)
			},
			setupResolver: func() func(dnsTarget DNSTarget) resolver {
				return func(dnsTarget DNSTarget) resolver {
					return &mockResolver{
						lookupHostFunc: func(ctx context.Context, host string) ([]string, error) {
							return []string{"1.2.3.4"}, nil
						},
					}
				}
			},
			validateResult: func(g *WithT, result *types.Result, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(result).To(BeNil())
				g.Expect(err.Error()).To(ContainSubstring("failed to get CoreDNS pod IPs"))
				g.Expect(err.Error()).To(ContainSubstring("no ready CoreDNS pod endpoints found"))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			dnsChecker := &DNSChecker{
				name: "test-dns-checker",
				config: &config.DNSConfig{
					Domain: "example.com",
				},
				k8sClientset: tc.setupClientset(),
				dnsResolver:  tc.setupResolver(),
			}

			result, err := dnsChecker.Run(context.Background())
			tc.validateResult(g, result, err)
		})
	}
}
