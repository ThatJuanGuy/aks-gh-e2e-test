package podstartup

import (
	"context"
	"errors"
	"fmt"
	"os"
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

type PodStartupChecker struct {
	name         string
	k8sClientset kubernetes.Interface
}

// The maximum number of synthetic pods created by the checker that can exist at any one time. If the limit has been reached, the checker
// will not create any more synthetic pods until some of the existing ones are deleted. Instead, it will fail the run with an error.
// Reaching this limit effectively disables the checker.
const MaxSyntheticPods = 10

func Register() {
	checker.RegisterChecker(config.CheckTypePodStartup, BuildPodStartupChecker)
}

// BuildPodStartupChecker creates a new PodStartupChecker instance.
func BuildPodStartupChecker(config *config.CheckerConfig) (checker.Checker, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	k8sClientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	return &PodStartupChecker{
		name:         config.Name,
		k8sClientset: k8sClientset,
	}, nil
}

func (c *PodStartupChecker) Name() string {
	return c.name
}

func (c *PodStartupChecker) Run(ctx context.Context) (*types.Result, error) {
	// Pods and service accounts have to share the same namespace. Read the namespace the checker is running in and use it to create
	// synthetic pods.
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return nil, fmt.Errorf("failed to read namespace from service account: %w", err)
	}
	namespace := strings.TrimSpace(string(data))

	// podLabels are shared by all synthetic pods created by this checker.
	podLabels := map[string]string{
		"cluster-health-monitor/checker-name": c.name,
		"app":                                 "cluster-health-monitor-podstartup-synthetic",
	}

	// Garbage collect any synthetic pods previously created by this checker.
	pods, err := c.k8sClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set(podLabels)).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in namespace %s: %w", namespace, err)
	}
	if err := c.garbageCollect(ctx, pods.Items); err != nil {
		// Logging instead of returning an error here to avoid failing the checker run.
		klog.Warningf("failed to garbage collect old synthetic pods: %s", err.Error())
	}

	// Do not run the checker if the maximum number of synthetic pods has been reached.
	if len(pods.Items) >= MaxSyntheticPods {
		return nil, fmt.Errorf("maximum number of synthetic pods reached in namespace %s, current: %d, max allowed: %d, delete some pods before running the checker again",
			namespace, len(pods.Items), MaxSyntheticPods)
	}

	// Create a synthetic pod to measure the startup time.
	synthPod, err := c.k8sClientset.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-synthetic-%d", c.name, time.Now().UnixNano()),
			Labels: podLabels,
		},
		Spec: corev1.PodSpec{
			// TODO? Maybe expose this in the config so this is not Azure-specific.
			NodeSelector: map[string]string{
				"kubernetes.azure.com/mode": "system",
			},
			Containers: []corev1.Container{
				{
					// TODO? Maybe use a different image
					Name:  "pause",
					Image: "k8s.gcr.io/pause:3.2",
				},
			},
			// TODO: Add pod cpu/memory requests and/or limits.
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create synthetic pod in namespace %s: %w", namespace, err)
	}

	// Ensure the synthetic pod is deleted when the function returns.
	defer func() {
		err := c.k8sClientset.CoreV1().Pods(namespace).Delete(ctx, synthPod.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			// Logging instead of returning an error here to avoid failing the checker run.
			klog.Warningf("failed to delete synthetic pod %s in namespace %s: %s", synthPod.Name, namespace, err.Error())
		}
	}()

	// TODO measure the pod startup time and determine if result is healthy or not.

	return nil, fmt.Errorf("pod startup checker is not fully implemented yet")
}

func (c *PodStartupChecker) garbageCollect(ctx context.Context, pods []corev1.Pod) error {
	var errs []error
	for _, pod := range pods {
		// TODO? Maybe take into account pod age and only try delete pods older than timeout in config
		if err := c.k8sClientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete synthetic pod %s: %s", pod.Name, err.Error()))
		}
	}
	return errors.Join(errs...)
}
