package podstartup

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/scheme"
	karpenter "sigs.k8s.io/karpenter/pkg/apis/v1"
)

func (c *PodStartupChecker) createKarpenterNodePool(ctx context.Context, nodePool *karpenter.NodePool) error {
	unstructuredNodePool := &unstructured.Unstructured{}
	scheme.Scheme.AddKnownTypes(NodePoolGVR.GroupVersion(), nodePool)
	if err := scheme.Scheme.Convert(nodePool, unstructuredNodePool, nil); err != nil {
		return err
	}

	// Create the NodePool resource.
	if _, err := c.dynamicClient.Resource(NodePoolGVR).Namespace(c.config.SyntheticPodNamespace).Create(ctx, unstructuredNodePool, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (c *PodStartupChecker) deleteKarpenterNodePool(ctx context.Context, nodePoolName string) error {
	// Delete the NodePool resource.
	err := c.dynamicClient.Resource(NodePoolGVR).Namespace(c.config.SyntheticPodNamespace).Delete(ctx, nodePoolName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (c *PodStartupChecker) deleteAllKarpenterNodePools(ctx context.Context) error {
	var errs []error

	// List all NodePools in the synthetic pod namespace.
	nodePools, err := c.dynamicClient.Resource(NodePoolGVR).Namespace(c.config.SyntheticPodNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: c.config.SyntheticPodLabelKey,
	})
	if err != nil {
		return err
	}

	// Iterate over the NodePools and delete each one.
	for _, nodePool := range nodePools.Items {
		nodePoolName, nameFound, err := unstructured.NestedString(nodePool.Object, "metadata", "name")
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse Karpenter Node Pool name: %w", err))
			continue
		}

		if !nameFound {
			errs = append(errs, fmt.Errorf("got Karpenter NodePool with no name field in metadata"))
			continue
		}

		if err := c.deleteKarpenterNodePool(ctx, nodePoolName); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete old Karpenter Node Pool %s: %w", nodePoolName, err))
		}
	}

	return errors.Join(errs...)
}

func (c *PodStartupChecker) karpenterNodePool(nodePoolName, timestampStr string) *karpenter.NodePool {
	return &karpenter.NodePool{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NodePool",
			APIVersion: "karpenter.sh/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: nodePoolName,
			Labels: map[string]string{
				c.config.SyntheticPodLabelKey: timestampStr,
			},
		},
		Spec: karpenter.NodePoolSpec{
			Template: karpenter.NodeClaimTemplate{
				Spec: karpenter.NodeClaimTemplateSpec{
					NodeClassRef: &karpenter.NodeClassReference{
						Group: "karpenter.azure.com",
						Kind:  "AKSNodeClass",
						Name:  "default",
					},
					Requirements: []karpenter.NodeSelectorRequirementWithMinValues{
						{
							NodeSelectorRequirement: corev1.NodeSelectorRequirement{
								Key:      c.config.SyntheticPodLabelKey,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{timestampStr},
							},
						},
					},
				},
			},
		},
	}
}
