// Package dnscheck provides a checker for DNS.
package dnscheck

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/afero"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/Azure/cluster-health-monitor/pkg/checker"
	"github.com/Azure/cluster-health-monitor/pkg/config"
	"github.com/Azure/cluster-health-monitor/pkg/types"
)

func Register() {
	checker.RegisterChecker(config.CheckTypeDNS, BuildDNSChecker)
}

const (
	coreDNSNamespace   = "kube-system"
	coreDNSServiceName = "kube-dns"
	resolvConfPath     = "/etc/resolv.conf"
)

var localDNSIPSet = map[string]struct{}{
	"169.254.10.10": {},
	"169.254.10.11": {},
}

// DNSChecker implements the Checker interface for DNS checks.
type DNSChecker struct {
	name       string
	config     *config.DNSConfig
	kubeClient kubernetes.Interface
	resolver   resolver
	fs         afero.Fs
}

// BuildDNSChecker creates a new DNSChecker instance.
// If the DNSType is LocalDNS, it checks if LocalDNS IPs are available before creating the checker.
func BuildDNSChecker(config *config.CheckerConfig) (checker.Checker, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	client, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// If this is a LocalDNS checker, check if LocalDNS IPs are available.
	if config.DNSConfig.CheckLocalDNS {
		tempChecker := &DNSChecker{
			fs: afero.NewOsFs(),
		}
		localDNSIPs, err := tempChecker.getLocalDNSIPs()
		if err != nil {
			klog.ErrorS(err, "Failed to get LocalDNS IPs")
			return nil, fmt.Errorf("failed to create LocalDNS checker: %w", err)
		}

		if len(localDNSIPs) == 0 {
			klog.InfoS("Found no LocalDNS IPs", "name", config.Name)
			return nil, checker.ErrSkipChecker
		}
	}

	// Create the checker
	chk := &DNSChecker{
		name:       config.Name,
		config:     config.DNSConfig,
		kubeClient: client,
		resolver:   &defaultResolver{},
		fs:         afero.NewOsFs(),
	}
	klog.InfoS("Built DNSChecker",
		"name", chk.name,
		"config", chk.config,
	)
	return chk, nil
}

func (c DNSChecker) Name() string {
	return c.name
}

func (c DNSChecker) Type() config.CheckerType {
	return config.CheckTypeDNS
}

// Run executes the DNS check.
// It will check either CoreDNS or LocalDNS for the configured domain.
func (c DNSChecker) Run(ctx context.Context) (*types.Result, error) {
	if c.config.CheckLocalDNS {
		return c.checkLocalDNS(ctx)
	} else {
		return c.checkCoreDNS(ctx)
	}
}

// checkCoreDNS queries CoreDNS service and pods.
// If all queries succeed, the check is considered healthy.
func (c DNSChecker) checkCoreDNS(ctx context.Context) (*types.Result, error) {
	// Check CoreDNS service.
	svcIP, err := getCoreDNSSvcIP(ctx, c.kubeClient)
	if errors.Is(err, errServiceNotReady) {
		return types.Unhealthy(errCodeServiceNotReady, "CoreDNS service is not ready"), nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := c.resolver.lookupHost(ctx, svcIP, c.config.Domain); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return types.Unhealthy(errCodeServiceTimeout, "CoreDNS service query timed out"), nil
		}
		return types.Unhealthy(errCodeServiceError, fmt.Sprintf("CoreDNS service query error: %s", err)), nil
	}

	// Check CoreDNS pods.
	podIPs, err := getCoreDNSPodIPs(ctx, c.kubeClient)
	if errors.Is(err, errPodsNotReady) {
		return types.Unhealthy(errCodePodsNotReady, "CoreDNS Pods are not ready"), nil
	}
	if err != nil {
		return nil, err
	}

	for _, ip := range podIPs {
		if _, err := c.resolver.lookupHost(ctx, ip, c.config.Domain); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return types.Unhealthy(errCodePodTimeout, "CoreDNS pod query timed out"), nil
			}
			return types.Unhealthy(errCodePodError, fmt.Sprintf("CoreDNS pod query error: %s", err)), nil
		}
	}

	return types.Healthy(), nil
}

// checkLocalDNS queries the LocalDNS servers.
// If all queries succeed, the check is considered healthy.
func (c DNSChecker) checkLocalDNS(ctx context.Context) (*types.Result, error) {
	localDNSIPs, err := c.getLocalDNSIPs()
	if err != nil {
		return nil, fmt.Errorf("failed to get LocalDNS IPs: %w", err)
	}

	if len(localDNSIPs) == 0 {
		return types.Unhealthy(errCodeLocalDNSNoIPs, "No LocalDNS IPs found"), nil
	}

	klog.V(3).InfoS("Found LocalDNS IPs", "ips", localDNSIPs)
	for _, ip := range localDNSIPs {
		if _, err := c.resolver.lookupHost(ctx, ip, c.config.Domain); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return types.Unhealthy(errCodeLocalDNSTimeout, "LocalDNS query timed out"), nil
			}
			return types.Unhealthy(errCodeLocalDNSError, fmt.Sprintf("LocalDNS query error: %s", err)), nil
		}
	}

	return types.Healthy(), nil
}

// getCoreDNSSvcIP returns the ClusterIP of the CoreDNS service in the cluster as a DNSTarget.
func getCoreDNSSvcIP(ctx context.Context, kubeClient kubernetes.Interface) (string, error) {
	svc, err := kubeClient.CoreV1().Services(coreDNSNamespace).Get(ctx, coreDNSServiceName, metav1.GetOptions{})

	if err != nil && apierrors.IsNotFound(err) {
		return "", errServiceNotReady
	}
	if err != nil {
		return "", fmt.Errorf("failed to get CoreDNS service: %w", err)
	}

	if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
		return "", errServiceNotReady
	}

	return svc.Spec.ClusterIP, nil
}

// getCoreDNSPodIPs returns the IPs of all CoreDNS pods in the cluster as DNSTargets.
func getCoreDNSPodIPs(ctx context.Context, kubeClient kubernetes.Interface) ([]string, error) {
	endpointSliceList, err := kubeClient.DiscoveryV1().EndpointSlices(coreDNSNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=" + coreDNSServiceName,
	})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, errPodsNotReady
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get CoreDNS pod IPs: %w", err)
	}

	var podIPs []string
	for _, endpointSlice := range endpointSliceList.Items {
		for _, ep := range endpointSlice.Endpoints {
			// According to Kubernetes docs: "A nil value should be interpreted as 'true'".
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}

			podIPs = append(podIPs, ep.Addresses...)
		}
	}

	if len(podIPs) == 0 {
		return nil, errPodsNotReady
	}

	return podIPs, nil
}

// getLocalDNSIPs reads /etc/resolv.conf and returns any nameserver entries that match LocalDNS IPs.
func (c DNSChecker) getLocalDNSIPs() ([]string, error) {
	file, err := c.fs.Open(resolvConfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", resolvConfPath, err)
	}

	defer func() {
		if err := file.Close(); err != nil {
			klog.ErrorS(err, "Failed to close file", "path", resolvConfPath)
		}
	}()

	var localDNSIPs []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ip := fields[1]
				if _, isLocalDNS := localDNSIPSet[ip]; isLocalDNS {
					localDNSIPs = append(localDNSIPs, ip)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading %s: %w", resolvConfPath, err)
	}

	return localDNSIPs, nil
}
