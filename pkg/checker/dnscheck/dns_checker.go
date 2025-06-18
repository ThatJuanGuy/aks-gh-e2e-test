// Package dnscheck provides a checker for DNS.
package dnscheck

import (
	"context"
	"fmt"
	"net"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/Azure/cluster-health-monitor/pkg/checker"
	"github.com/Azure/cluster-health-monitor/pkg/config"
	"github.com/Azure/cluster-health-monitor/pkg/types"
)

func Register() {
	checker.RegisterChecker(config.CheckTypeDNS, BuildDNSChecker)
}

const (
	CoreDNSNamespace   = "kube-system"
	CoreDNSServiceName = "kube-dns"
)

// DNSTargetType defines the type of DNS target.
type DNSTargetType string

const (
	CoreDNSService DNSTargetType = "service"
	CoreDNSPod     DNSTargetType = "pod"
)

// DNSTarget represents a DNS target with its IP and type.
type DNSTarget struct {
	IP   string
	Type DNSTargetType
}

// DNSChecker implements the Checker interface for DNS checks.
type DNSChecker struct {
	name         string
	config       *config.DNSConfig
	k8sClientset kubernetes.Interface
	dnsResolver  func(dnsTarget DNSTarget) resolver
}

// resolver is an interface for DNS resolution.
type resolver interface {
	lookupHost(ctx context.Context, host string) ([]string, error)
}

// defaultResolver implements the resolver interface using net.Resolver.
type defaultResolver struct {
	target DNSTarget
}

func (r *defaultResolver) lookupHost(ctx context.Context, host string) ([]string, error) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, network, net.JoinHostPort(r.target.IP, "53"))
		},
	}
	return resolver.LookupHost(ctx, host)
}

// newDefaultResolver creates a new defaultResolver.
func newDefaultResolver(target DNSTarget) resolver {
	return &defaultResolver{target: target}
}

// BuildDNSChecker creates a new DNSChecker instance.
func BuildDNSChecker(config *config.CheckerConfig) (checker.Checker, error) {
	if config.Name == "" {
		return nil, fmt.Errorf("checker name cannot be empty")
	}
	if err := config.DNSConfig.ValidateDNSConfig(); err != nil {
		return nil, err
	}

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &DNSChecker{
		name:         config.Name,
		config:       config.DNSConfig,
		k8sClientset: clientset,
		dnsResolver:  newDefaultResolver,
	}, nil
}

func (c DNSChecker) Name() string {
	return c.name
}

// Run executes the DNS check.
// It queries the CoreDNS service and pods for the configured domain.
// If LocalDNS is configured, it should also query that.
// If all queries succeed, the check is considered healthy.
func (c DNSChecker) Run(ctx context.Context) (*types.Result, error) {
	coreDNSServiceTarget, err := getCoreDNSServiceIP(ctx, c.k8sClientset)
	if err != nil {
		return types.Unhealthy("dns_service_unhealthy", err.Error()), nil
	}

	coreDNSPodTargets, err := getCoreDNSPodIPs(ctx, c.k8sClientset)
	if err != nil {
		return types.Unhealthy("dns_pod_unhealthy", err.Error()), nil
	}

	// TODO: Get LocalDNS IP.

	// Resolve the domain against DNS targets.
	dnsTargets := append([]DNSTarget{coreDNSServiceTarget}, coreDNSPodTargets...)
	for _, target := range dnsTargets {
		// TODO: Have a short timeout for each DNS query.
		// The timeout is configurable from the DNSConfig.
		// Have different error code for timeout: dns_<target.type>_timeout.
		if err := c.resolveDomain(ctx, target); err != nil {
			errorCode := fmt.Sprintf("dns_%s_unhealthy", target.Type)
			return types.Unhealthy(errorCode, fmt.Sprintf("DNS %s %s unhealthy: %v", target.Type, target.IP, err)), nil
		}
	}

	return types.Healthy(), nil
}

// getCoreDNSServiceIP returns the ClusterIP of the CoreDNS service in the cluster as a DNSTarget.
func getCoreDNSServiceIP(ctx context.Context, clientset kubernetes.Interface) (DNSTarget, error) {
	service, err := clientset.CoreV1().Services(CoreDNSNamespace).Get(ctx, CoreDNSServiceName, metav1.GetOptions{})
	if err != nil {
		return DNSTarget{}, fmt.Errorf("failed to get CoreDNS service IP: %w", err)
	}

	if service.Spec.ClusterIP == "" {
		return DNSTarget{}, fmt.Errorf("failed to get CoreDNS service IP: no ClusterIP")
	}

	return DNSTarget{
		IP:   service.Spec.ClusterIP,
		Type: CoreDNSService,
	}, nil
}

// getCoreDNSPodIPs returns the IPs of all CoreDNS pods in the cluster as DNSTargets.
func getCoreDNSPodIPs(ctx context.Context, clientset kubernetes.Interface) ([]DNSTarget, error) {
	endpointSliceList, err := clientset.DiscoveryV1().EndpointSlices(CoreDNSNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=" + CoreDNSServiceName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get CoreDNS pod IPs: %w", err)
	}

	var dnsTargets []DNSTarget
	for _, endpointSlice := range endpointSliceList.Items {
		for _, endpoint := range endpointSlice.Endpoints {
			// According to Kubernetes docs: "A nil value should be interpreted as 'true'".
			if endpoint.Conditions.Ready != nil && !*endpoint.Conditions.Ready {
				continue
			}

			for _, address := range endpoint.Addresses {
				dnsTargets = append(dnsTargets, DNSTarget{
					IP:   address,
					Type: CoreDNSPod,
				})
			}
		}
	}

	if len(dnsTargets) == 0 {
		return nil, fmt.Errorf("failed to get CoreDNS pod IPs: no ready endpoints")
	}

	return dnsTargets, nil
}

// resolveDomain performs the DNS query against the specified DNSTarget.
func (c *DNSChecker) resolveDomain(ctx context.Context, dnsTarget DNSTarget) error {
	resolver := c.dnsResolver(dnsTarget)
	_, err := resolver.lookupHost(ctx, c.config.Domain)
	return err
}
