// Package apiserver provides a checker for the Kubernetes API server.
package apiserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/Azure/cluster-health-monitor/pkg/checker"
	"github.com/Azure/cluster-health-monitor/pkg/config"
	"github.com/Azure/cluster-health-monitor/pkg/types"
)

// APIServerChecker implements the Checker interface for API server checks.
type APIServerChecker struct {
	name       string
	config     *config.APIServerConfig
	timeout    time.Duration
	kubeClient kubernetes.Interface
}

func Register() {
	checker.RegisterChecker(config.CheckTypeAPIServer, buildAPIServerChecker)
}

// buildAPIServerChecker creates a new APIServerChecker instance.
func buildAPIServerChecker(config *config.CheckerConfig) (checker.Checker, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	client, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	chk := &APIServerChecker{
		name:       config.Name,
		config:     config.APIServerConfig,
		timeout:    config.Timeout,
		kubeClient: client,
	}
	klog.InfoS("Built APIServerChecker",
		"name", chk.name,
		"config", chk.config,
		"timeout", chk.timeout.String(),
	)
	return chk, nil
}

func (c APIServerChecker) Name() string {
	return c.name
}

func (c APIServerChecker) Type() config.CheckerType {
	return config.CheckTypeAPIServer
}

// Run executes the API server check.
// It creates an empty ConfigMap, gets it, and then deletes it.
// If all operations succeed, the check is considered healthy.
func (c APIServerChecker) Run(ctx context.Context) (*types.Result, error) {
	// Garbage collect any leftover ConfigMaps previously created by this checker.
	if err := c.garbageCollect(ctx); err != nil {
		// Logging instead of returning an error to avoid failing the checker run.
		klog.InfoS("Failed to garbage collect old ConfigMaps", "error", err.Error())
	}

	// Create ConfigMap.
	createCtx, createCancel := context.WithTimeout(ctx, c.config.ConfigMapMutateTimeout)
	defer createCancel()
	createdConfigMap, err := c.kubeClient.CoreV1().ConfigMaps(c.config.ConfigMapNamespace).Create(createCtx, c.generateConfigMap(), metav1.CreateOptions{})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return types.Unhealthy(errCodeConfigMapCreateTimeout, "timed out while creating ConfigMap"), nil
		}
		return types.Unhealthy(errCodeConfigMapCreateError, fmt.Sprintf("failed to create ConfigMap: %v", err)), nil
	}
	// Defer deletion of the created ConfigMap in case of failure later in the function.
	defer func() {
		err := c.kubeClient.CoreV1().ConfigMaps(c.config.ConfigMapNamespace).Delete(ctx, createdConfigMap.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			// Logging instead of returning an error here to avoid failing the checker run.
			klog.InfoS("Failed to delete ConfigMap", "name", createdConfigMap.Name, "error", err.Error())
		}
	}()

	// Get ConfigMap.
	getCtx, getCancel := context.WithTimeout(ctx, c.config.ConfigMapReadTimeout)
	defer getCancel()
	_, err = c.kubeClient.CoreV1().ConfigMaps(c.config.ConfigMapNamespace).Get(getCtx, createdConfigMap.Name, metav1.GetOptions{})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return types.Unhealthy(errCodeConfigMapGetTimeout, "timed out while getting ConfigMap"), nil
		}
		return types.Unhealthy(errCodeConfigMapGetError, fmt.Sprintf("failed to get ConfigMap: %v", err)), nil
	}

	// Delete ConfigMap.
	deleteCtx, deleteCancel := context.WithTimeout(ctx, c.config.ConfigMapMutateTimeout)
	defer deleteCancel()
	err = c.kubeClient.CoreV1().ConfigMaps(c.config.ConfigMapNamespace).Delete(deleteCtx, createdConfigMap.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return types.Unhealthy(errCodeConfigMapDeleteTimeout, "timed out while deleting ConfigMap"), nil
		}
		return types.Unhealthy(errCodeConfigMapDeleteError, fmt.Sprintf("failed to delete ConfigMap: %v", err)), nil
	}

	return types.Healthy(), nil
}

// configMapLabels returns the labels to be applied to ConfigMaps created specifically by this checker.
// The checker's name is a unique identifier for each checker.
func (c APIServerChecker) configMapLabels() map[string]string {
	return map[string]string{
		c.config.ConfigMapLabelKey: c.name,
	}
}

// configMapNamePrefix returns the prefix for ConfigMap names created specifically by this checker.
// This is used to ensure that ConfigMaps created by this checker can be easily identified and cleaned up.
func (c APIServerChecker) configMapNamePrefix() string {
	return strings.ToLower(fmt.Sprintf("%s-empty-configmap-", c.name))
}

// generateConfigMap creates a ConfigMap object for this checker.
func (c APIServerChecker) generateConfigMap() *corev1.ConfigMap {
	configMapName := fmt.Sprintf("%s%d", c.configMapNamePrefix(), time.Now().UnixNano())
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: c.config.ConfigMapNamespace,
			Labels:    c.configMapLabels(),
		},
	}
	return configMap
}

// garbageCollect attempts to delete any leftover ConfigMaps created by this checker
// in previous runs that may not have been properly deleted.
func (c APIServerChecker) garbageCollect(ctx context.Context) error {
	configMapList, err := c.kubeClient.CoreV1().ConfigMaps(c.config.ConfigMapNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set(c.configMapLabels())).String(),
	})
	if err != nil {
		return fmt.Errorf("failed to list ConfigMaps for garbage collection: %w", err)
	}

	var errs []error
	for _, cm := range configMapList.Items {
		if !strings.HasPrefix(cm.Name, c.configMapNamePrefix()) {
			// This ConfigMap is not created by this checker, skip it.
			continue
		}
		if time.Since(cm.CreationTimestamp.Time) > c.timeout {
			err := c.kubeClient.CoreV1().ConfigMaps(cm.Namespace).Delete(ctx, cm.Name, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("failed to delete old empty ConfigMap %s: %w", cm.Name, err))
			}
		}
	}
	return errors.Join(errs...)
}
