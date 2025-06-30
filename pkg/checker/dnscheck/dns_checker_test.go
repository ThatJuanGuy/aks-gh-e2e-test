package dnscheck

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/cluster-health-monitor/pkg/config"
	"github.com/Azure/cluster-health-monitor/pkg/types"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

type fakeResolver struct {
	lookupHostFunc func(ctx context.Context, ip, domain string) ([]string, error)
}

func (f *fakeResolver) lookupHost(ctx context.Context, ip, domain string) ([]string, error) {
	return f.lookupHostFunc(ctx, ip, domain)
}

func TestDNSChecker_Healthy(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	mockFs := afero.NewMemMapFs()
	resolveConfContent := `
nameserver 169.254.10.10
`
	err := afero.WriteFile(mockFs, resolvConfPath, []byte(resolveConfContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	client := k8sfake.NewClientset(
		makeCoreDNSService("10.0.0.10"),
		makeCoreDNSEndpointSlice([]string{"10.0.0.11", "10.0.0.12"}),
	)
	chk := &DNSChecker{
		name:       "dns-test",
		config:     &config.DNSConfig{Domain: "example.com"},
		kubeClient: client,
		resolver: &fakeResolver{
			lookupHostFunc: func(ctx context.Context, ip, domain string) ([]string, error) {
				return []string{"1.2.3.4"}, nil
			},
		},
		fs: mockFs,
	}
	res, err := chk.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Status).To(Equal(types.StatusHealthy))
}

func TestDNSChecker_ServiceNotReady(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	client := k8sfake.NewClientset() // no service
	chk := &DNSChecker{
		name:       "dns-test",
		config:     &config.DNSConfig{Domain: "example.com"},
		kubeClient: client,
		resolver: &fakeResolver{
			lookupHostFunc: func(ctx context.Context, ip, domain string) ([]string, error) {
				return []string{"1.2.3.4"}, nil
			},
		},
		fs: afero.NewMemMapFs(),
	}
	res, err := chk.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Status).To(Equal(types.StatusUnhealthy))
	g.Expect(res.Detail.Code).To(Equal(errCodeServiceNotReady))
}

func TestDNSChecker_PodsNotReady(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	client := k8sfake.NewClientset(
		makeCoreDNSService("10.0.0.10"),
	)
	chk := &DNSChecker{
		name:       "dns-test",
		config:     &config.DNSConfig{Domain: "example.com"},
		kubeClient: client,
		resolver: &fakeResolver{
			lookupHostFunc: func(ctx context.Context, ip, domain string) ([]string, error) {
				return []string{"1.2.3.4"}, nil
			},
		},
		fs: afero.NewMemMapFs(),
	}
	res, err := chk.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Status).To(Equal(types.StatusUnhealthy))
	g.Expect(res.Detail.Code).To(Equal(errCodePodsNotReady))
}

func TestDNSChecker_Timeout(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	client := k8sfake.NewClientset(
		makeCoreDNSService("10.0.0.10"),
		makeCoreDNSEndpointSlice([]string{"10.0.0.11"}),
	)
	chk := &DNSChecker{
		name:       "dns-test",
		config:     &config.DNSConfig{Domain: "example.com"},
		kubeClient: client,
		resolver: &fakeResolver{
			lookupHostFunc: func(ctx context.Context, ip, domain string) ([]string, error) {
				return nil, context.DeadlineExceeded
			},
		},
		fs: afero.NewMemMapFs(),
	}
	res, err := chk.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Status).To(Equal(types.StatusUnhealthy))
	g.Expect(res.Detail.Code).To(Equal(errCodeServiceTimeout))
}

func TestDNSChecker_LocalDNSError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	mockFs := afero.NewMemMapFs()
	resolveConfContent := `
nameserver 169.254.10.10
`
	err := afero.WriteFile(mockFs, resolvConfPath, []byte(resolveConfContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	client := k8sfake.NewClientset(
		makeCoreDNSService("10.0.0.10"),
		makeCoreDNSEndpointSlice([]string{"10.0.0.11", "10.0.0.12"}),
	)
	chk := &DNSChecker{
		name:       "dns-test",
		config:     &config.DNSConfig{Domain: "example.com"},
		kubeClient: client,
		resolver: &fakeResolver{
			lookupHostFunc: func(ctx context.Context, ip, domain string) ([]string, error) {
				if ip == "169.254.10.10" {
					return nil, fmt.Errorf("connection refused")
				}
				return []string{"1.2.3.4"}, nil
			},
		},
		fs: mockFs,
	}
	res, err := chk.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Status).To(Equal(types.StatusUnhealthy))
	g.Expect(res.Detail.Code).To(Equal(errCodeLocalDNSError))
}

func TestDNSChecker_LocalDNSTimeout(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	mockFs := afero.NewMemMapFs()
	resolveConfContent := `
nameserver 169.254.10.11
`
	err := afero.WriteFile(mockFs, resolvConfPath, []byte(resolveConfContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	client := k8sfake.NewClientset(
		makeCoreDNSService("10.0.0.10"),
		makeCoreDNSEndpointSlice([]string{"10.0.0.11", "10.0.0.12"}),
	)
	chk := &DNSChecker{
		name:       "dns-test",
		config:     &config.DNSConfig{Domain: "example.com"},
		kubeClient: client,
		resolver: &fakeResolver{
			lookupHostFunc: func(ctx context.Context, ip, domain string) ([]string, error) {
				if ip == "169.254.10.11" {
					return nil, context.DeadlineExceeded
				}
				return []string{"1.2.3.4"}, nil
			},
		},
		fs: mockFs,
	}
	res, err := chk.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Status).To(Equal(types.StatusUnhealthy))
	g.Expect(res.Detail.Code).To(Equal(errCodeLocalDNSTimeout))
}

func TestDNSChecker_NoLocalDNS(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	mockFs := afero.NewMemMapFs()
	resolveConfContent := `
nameserver 8.8.8.8
`
	err := afero.WriteFile(mockFs, resolvConfPath, []byte(resolveConfContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	client := k8sfake.NewClientset(
		makeCoreDNSService("10.0.0.10"),
		makeCoreDNSEndpointSlice([]string{"10.0.0.11", "10.0.0.12"}),
	)

	chk := &DNSChecker{
		name:       "dns-test",
		config:     &config.DNSConfig{Domain: "example.com"},
		kubeClient: client,
		resolver: &fakeResolver{
			lookupHostFunc: func(ctx context.Context, ip, domain string) ([]string, error) {
				// Check if 8.8.8.8 is being queried.
				if ip == "8.8.8.8" {
					return nil, fmt.Errorf("connection refused")
				}
				return []string{"1.2.3.4"}, nil
			},
		},
		fs: mockFs,
	}

	res, err := chk.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Status).To(Equal(types.StatusHealthy))
}

// --- helpers ---

func makeCoreDNSService(ip string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: coreDNSNamespace,
			Name:      coreDNSServiceName,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: ip,
		},
	}
}

func makeCoreDNSEndpointSlice(ips []string) *discoveryv1.EndpointSlice {
	endpoints := []discoveryv1.Endpoint{}
	for _, ip := range ips {
		ready := true
		endpoints = append(endpoints, discoveryv1.Endpoint{
			Addresses: []string{ip},
			Conditions: discoveryv1.EndpointConditions{
				Ready: &ready,
			},
		})
	}
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: coreDNSNamespace,
			Labels: map[string]string{
				discoveryv1.LabelServiceName: coreDNSServiceName,
			},
		},
		Endpoints: endpoints,
	}
}
