// Package dnscheck provides a checker for DNS.
package dnscheck

import (
	"context"
	"fmt"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/Azure/cluster-health-monitor/pkg/config"
)

// DNSChecker implements the Checker interface for DNS checks.
type DNSChecker struct {
	name   string
	config *config.DNSConfig
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

// BuildDNSChecker creates a new DNSChecker instance.
func BuildDNSChecker(name string, config *config.DNSConfig) (*DNSChecker, error) {
	if name == "" {
		return nil, fmt.Errorf("checker name cannot be empty")
	}
	if err := config.ValidateDNSConfig(); err != nil {
		return nil, err
	}

	return &DNSChecker{
		name:   name,
		config: config,
	}, nil
}

func (c DNSChecker) Name() string {
	return c.name
}

func (c DNSChecker) Run(ctx context.Context) error {
	// TODO: Get the CoreDNS service IP and pod IPs.

	// TODO: Get LocalDNS IP.

	// TODO: Implement the DNS checking logic here
	return fmt.Errorf("DNSChecker not implemented yet")
}

// GetCoreDNSServiceIP returns the ClusterIP of the CoreDNS service in the cluster as a DNSTarget.
func GetCoreDNSServiceIP(ctx context.Context, clientset kubernetes.Interface) (DNSTarget, error) {
	if clientset == nil {
		return DNSTarget{}, fmt.Errorf("clientset cannot be nil")
	}

	service, err := clientset.CoreV1().Services(CoreDNSNamespace).Get(ctx, CoreDNSServiceName, metav1.GetOptions{})
	if err != nil {
		return DNSTarget{}, fmt.Errorf("failed to get CoreDNS service: %w", err)
	}

	if service.Spec.ClusterIP == "" {
		return DNSTarget{}, fmt.Errorf("CoreDNS service has no ClusterIP")
	}

	return DNSTarget{
		IP:   service.Spec.ClusterIP,
		Type: CoreDNSService,
	}, nil
}

// GetCoreDNSPodIPs returns the IPs of all CoreDNS pods in the cluster as DNSTargets.
func GetCoreDNSPodIPs(ctx context.Context, clientset kubernetes.Interface) ([]DNSTarget, error) {
	if clientset == nil {
		return nil, fmt.Errorf("clientset cannot be nil")
	}

	endpointSliceList, err := clientset.DiscoveryV1().EndpointSlices(CoreDNSNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=" + CoreDNSServiceName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list CoreDNS endpoint slices: %w", err)
	}

	var dnsTargets []DNSTarget
	for _, endpointSlice := range endpointSliceList.Items {
		for _, endpoint := range endpointSlice.Endpoints {
			for _, address := range endpoint.Addresses {
				dnsTargets = append(dnsTargets, DNSTarget{
					IP:   address,
					Type: CoreDNSPod,
				})
			}
		}
	}

	if len(dnsTargets) == 0 {
		return nil, fmt.Errorf("no CoreDNS pod endpoints found")
	}

	return dnsTargets, nil
}
