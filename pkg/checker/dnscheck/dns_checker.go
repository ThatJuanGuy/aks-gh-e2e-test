// Package dnscheck provides a checker for DNS.
package dnscheck

import (
	"context"
	"errors"
	"fmt"
	"net"

	"golang.org/x/sync/errgroup"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

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
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	coreDNSServiceTarget, err := getCoreDNSServiceIP(ctx, clientset)
	if err != nil {
		return fmt.Errorf("failed to get CoreDNS service IP: %w", err)
	}

	coreDNSPodTargets, err := getCoreDNSPodIPs(ctx, clientset)
	if err != nil {
		return fmt.Errorf("failed to get CoreDNS pod IPs: %w", err)
	}

	// TODO: Get LocalDNS IP.

	dnsTargets := make(map[DNSTarget]struct{})
	dnsTargets[coreDNSServiceTarget] = struct{}{}
	for _, target := range coreDNSPodTargets {
		dnsTargets[target] = struct{}{}
	}

	type dnsResult struct {
		target DNSTarget
		err    error
	}

	dnsResultChan := make(chan dnsResult, len(dnsTargets))
	g, gctx := errgroup.WithContext(ctx)
	for target := range dnsTargets {
		target := target
		g.Go(func() error {
			err := c.resolveDomain(gctx, target)
			dnsResultChan <- dnsResult{target: target, err: err}
			return nil
		})
	}
	g.Wait()
	close(dnsResultChan)

	var serviceError error
	var podErrors []error
	var hasHealthyPod bool
	for result := range dnsResultChan {
		if result.err == nil {
			if result.target.Type == CoreDNSPod {
				hasHealthyPod = true
			}
			continue
		}

		var errMsg string
		if errors.Is(result.err, context.DeadlineExceeded) {
			errMsg = fmt.Sprintf("CoreDNS %s %s timed out", result.target.Type, result.target.IP)
		} else {
			errMsg = fmt.Sprintf("CoreDNS %s %s unhealthy", result.target.Type, result.target.IP)
		}

		if result.target.Type == CoreDNSService {
			serviceError = fmt.Errorf("%s: %w", errMsg, result.err)
		} else {
			podErrors = append(podErrors, fmt.Errorf("%s: %w", errMsg, result.err))
	}
	}

	var allErrors []error
	allErrors = append(allErrors, serviceError)
	if !hasHealthyPod {
		allErrors = append(allErrors, podErrors...)
	}

	if len(allErrors) > 0 {
		return errors.Join(allErrors...)
	}

	return nil
}

// getCoreDNSServiceIP returns the ClusterIP of the CoreDNS service in the cluster as a DNSTarget.
func getCoreDNSServiceIP(ctx context.Context, clientset kubernetes.Interface) (DNSTarget, error) {
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

// getCoreDNSPodIPs returns the IPs of all CoreDNS pods in the cluster as DNSTargets.
func getCoreDNSPodIPs(ctx context.Context, clientset kubernetes.Interface) ([]DNSTarget, error) {
	endpointSliceList, err := clientset.DiscoveryV1().EndpointSlices(CoreDNSNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=" + CoreDNSServiceName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list CoreDNS endpoint slices: %w", err)
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
		return nil, fmt.Errorf("no ready CoreDNS pod endpoints found")
	}

	return dnsTargets, nil
}

// resolveDomain performs the DNS query against the specified DNSTarget.
func (c *DNSChecker) resolveDomain(ctx context.Context, dnsTarget DNSTarget) error {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, network, net.JoinHostPort(dnsTarget.IP, "53"))
		},
	}

	_, err := resolver.LookupHost(ctx, c.config.Domain)
	return err
}
