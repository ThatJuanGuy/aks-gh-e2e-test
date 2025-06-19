package podstartup

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestPodStartupChecker_GarbageCollect(t *testing.T) {
	checkerName := "checker"
	checkerNamespace := "checker-ns"
	checkerTimeout := 5 * time.Second
	checkerPodLabels := map[string]string{
		"cluster-health-monitor/checker-name": checkerName,
		"app":                                 "cluster-health-monitor-podstartup-synthetic",
	}

	tests := []struct {
		name        string
		setupPods   []*corev1.Pod
		validateRes func(g *WithT, pods *corev1.PodList)
	}{
		{
			name: "only removes pods older than timeout",
			setupPods: []*corev1.Pod{
				makePodWithLabels("old-pod", checkerNamespace, checkerPodLabels, time.Now().Add(-2*time.Hour)),
				makePodWithLabels("new-pod", checkerNamespace, checkerPodLabels, time.Now()),
			},
			validateRes: func(g *WithT, pods *corev1.PodList) {
				g.Expect(pods.Items).To(HaveLen(1))
				g.Expect(pods.Items[0].Name).To(Equal("new-pod"))
			},
		},
		{
			name: "no pods to delete",
			setupPods: []*corev1.Pod{
				makePodWithLabels("new-pod-1", checkerNamespace, checkerPodLabels, time.Now()),                      // pod too new
				makePodWithLabels("new-pod-2", checkerNamespace, map[string]string{}, time.Now().Add(-2*time.Hour)), // old pod wrong labels
			},
			validateRes: func(g *WithT, pods *corev1.PodList) {
				g.Expect(pods.Items).To(HaveLen(2))
				actualNames := make([]string, len(pods.Items))
				for i, pod := range pods.Items {
					actualNames[i] = pod.Name
				}
				g.Expect(actualNames).To(ConsistOf([]string{"new-pod-1", "new-pod-2"}))
			},
		},
		{
			name: "only removes pod with checker labels",
			setupPods: []*corev1.Pod{
				makePodWithLabels("checker-pod", checkerNamespace, checkerPodLabels, time.Now().Add(-2*time.Hour)),
				makePodWithLabels("non-checker-pod", checkerNamespace, map[string]string{}, time.Now().Add(-2*time.Hour)),
			},
			validateRes: func(g *WithT, pods *corev1.PodList) {
				g.Expect(pods.Items).To(HaveLen(1))
				g.Expect(pods.Items[0].Name).To(Equal("non-checker-pod"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			client := k8sfake.NewClientset()
			for _, pod := range tt.setupPods {
				_, err := client.CoreV1().Pods(checkerNamespace).Create(context.Background(), pod, metav1.CreateOptions{})
				g.Expect(err).ToNot(HaveOccurred())
			}

			checker := &PodStartupChecker{
				name:         checkerName,
				timeout:      checkerTimeout,
				k8sClientset: client,
				namespace:    checkerNamespace,
				podLabels: map[string]string{
					"cluster-health-monitor/checker-name": checkerName,
					"app":                                 "cluster-health-monitor-podstartup-synthetic",
				},
			}

			// Run garbage collect
			err := checker.garbageCollect(context.Background())
			g.Expect(err).ToNot(HaveOccurred())

			// Get pods and validate result
			pods, err := client.CoreV1().Pods(checkerNamespace).List(context.Background(), metav1.ListOptions{})
			g.Expect(err).ToNot(HaveOccurred())

			tt.validateRes(g, pods)
		})
	}
}

// --- helpers ---
func makePodWithLabels(name string, namespace string, labels map[string]string, creationTime time.Time) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			Labels:            labels,
			CreationTimestamp: metav1.NewTime(creationTime),
		},
	}
}
