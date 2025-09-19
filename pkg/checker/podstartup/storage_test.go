package podstartup

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/cluster-health-monitor/pkg/config"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestStorageResources(t *testing.T) {
	g := NewWithT(t)
	checker := &PodStartupChecker{
		config: &config.PodStartupConfig{
			EnabledCSIs:           []config.CSIType{config.CSITypeAzureDisk, config.CSITypeAzureFile, config.CSITypeAzureBlob},
			SyntheticPodNamespace: "default",
		},
	}
	pods := checker.generateSyntheticPod("timestampstr")

	g.Expect(pods).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes).To(HaveLen(3)) // Expect 3 volumes for AzureDisk, AzureFile, and AzureBlob

	g.Expect(pods.Spec.Volumes[0]).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes[0].PersistentVolumeClaim).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes[0].PersistentVolumeClaim.ClaimName).To(Equal(checker.azureDiskPVC("timestampstr").Name))

	g.Expect(pods.Spec.Volumes[1]).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes[1].PersistentVolumeClaim).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes[1].PersistentVolumeClaim.ClaimName).To(Equal(checker.azureFilePVC("timestampstr").Name))

	g.Expect(pods.Spec.Volumes[2]).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes[2].PersistentVolumeClaim).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes[2].PersistentVolumeClaim.ClaimName).To(Equal(checker.azureBlobPVC("timestampstr").Name))
}

func TestCreateCSIResources(t *testing.T) {
	testCases := []struct {
		name         string
		enabledCSIs  []config.CSIType
		k8sClient    *k8sfake.Clientset
		validateFunc func(g *WithT, err error, k8sClient *k8sfake.Clientset)
	}{
		{
			name:        "CSI tests disabled",
			enabledCSIs: []config.CSIType{},
			k8sClient:   k8sfake.NewClientset(),
			validateFunc: func(g *WithT, err error, k8sClient *k8sfake.Clientset) {
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
		{
			name:        "CSI tests enabled - successful creation",
			enabledCSIs: []config.CSIType{config.CSITypeAzureDisk, config.CSITypeAzureBlob, config.CSITypeAzureFile},
			k8sClient:   k8sfake.NewClientset(),
			validateFunc: func(g *WithT, err error, k8sClient *k8sfake.Clientset) {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(k8sClient.Actions()).To(HaveLen(4)) // Expect 4 create actions for 3 PVCs and 1 StorageClass
			},
		},
		{
			name:        "CSI tests enabled - error on creating StorageClass",
			enabledCSIs: []config.CSIType{config.CSITypeAzureDisk, config.CSITypeAzureBlob, config.CSITypeAzureFile},
			k8sClient: func() *k8sfake.Clientset {
				client := k8sfake.NewClientset()
				client.PrependReactor("create", "storageclasses", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("internal error")
				})
				return client
			}(),
			validateFunc: func(g *WithT, err error, k8sClient *k8sfake.Clientset) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("internal error"))
				g.Expect(k8sClient.Actions()).To(HaveLen(3)) // Expect 3 create actions for 2 PVCs and 1 StorageClass
			},
		},
		{
			name:        "CSI tests enabled - error on creating azure disk PVC",
			enabledCSIs: []config.CSIType{config.CSITypeAzureDisk},
			k8sClient: func() *k8sfake.Clientset {
				client := k8sfake.NewClientset()
				client.PrependReactor("create", "persistentvolumeclaims", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("internal error")
				})
				return client
			}(),
			validateFunc: func(g *WithT, err error, k8sClient *k8sfake.Clientset) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("internal error"))
				g.Expect(k8sClient.Actions()).To(HaveLen(1)) // Expect 1 create action for 1 PVC
			},
		},
		{
			name:        "CSI tests enabled - error on creating azure blob PVC",
			enabledCSIs: []config.CSIType{config.CSITypeAzureBlob},
			k8sClient: func() *k8sfake.Clientset {
				client := k8sfake.NewClientset()
				client.PrependReactor("create", "persistentvolumeclaims", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("internal error")
				})
				return client
			}(),
			validateFunc: func(g *WithT, err error, k8sClient *k8sfake.Clientset) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("internal error"))
				g.Expect(k8sClient.Actions()).To(HaveLen(1)) // Expect 1 create action for 1 PVC
			},
		},
		{
			name:        "CSI tests enabled - error on creating azure file PVC",
			enabledCSIs: []config.CSIType{config.CSITypeAzureFile},
			k8sClient: func() *k8sfake.Clientset {
				client := k8sfake.NewClientset()
				client.PrependReactor("create", "persistentvolumeclaims", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("internal error")
				})
				return client
			}(),
			validateFunc: func(g *WithT, err error, k8sClient *k8sfake.Clientset) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("internal error"))
				g.Expect(k8sClient.Actions()).To(HaveLen(2)) // Expect 2 create actions for 1 PVC and 1 StorageClass
			},
		},
	}
	for _, tc := range testCases {
		checker := &PodStartupChecker{
			config: &config.PodStartupConfig{
				EnabledCSIs: tc.enabledCSIs,
			},
			k8sClientset: tc.k8sClient,
		}
		err := checker.createCSIResources(context.Background(), "timestampstr")
		g := NewWithT(t)
		tc.validateFunc(g, err, tc.k8sClient)
	}
}

func TestDeleteCSIResources(t *testing.T) {
	testCases := []struct {
		name         string
		k8sClient    *k8sfake.Clientset
		enabledCSIs  []config.CSIType
		validateFunc func(g *WithT, err error, k8sClient *k8sfake.Clientset)
	}{
		{
			name:        "all resources successfully deleted",
			k8sClient:   k8sfake.NewClientset(),
			enabledCSIs: []config.CSIType{config.CSITypeAzureDisk, config.CSITypeAzureFile, config.CSITypeAzureBlob},
			validateFunc: func(g *WithT, err error, k8sClient *k8sfake.Clientset) {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(k8sClient.Actions()).To(HaveLen(4)) // Expect 4 delete actions for 3 PVCs and 1 StorageClass
			},
		},
		{
			name:        "resources successfully deleted with some resources not found",
			enabledCSIs: []config.CSIType{config.CSITypeAzureFile},
			k8sClient: func() *k8sfake.Clientset {
				client := k8sfake.NewClientset()
				client.PrependReactor("delete", "storageclasses", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "storage.k8s.io", Resource: "storageclasses"}, "azurefile-csi")
				})
				return client
			}(),
			validateFunc: func(g *WithT, err error, k8sClient *k8sfake.Clientset) {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(k8sClient.Actions()).To(HaveLen(2)) // Expect 1 StorageClass deletion and 1 PVC deletion
			},
		},
		{
			name:        "deletion error",
			enabledCSIs: []config.CSIType{config.CSITypeAzureFile},
			k8sClient: func() *k8sfake.Clientset {
				client := k8sfake.NewClientset()
				client.PrependReactor("delete", "storageclasses", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("unexpected error occurred while deleting storage class")
				})
				return client
			}(),
			validateFunc: func(g *WithT, err error, k8sClient *k8sfake.Clientset) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("unexpected error occurred while deleting storage class"))
				g.Expect(k8sClient.Actions()).To(HaveLen(1)) // Expect 1 delete action for 1 StorageClass
			},
		},
	}
	for _, tc := range testCases {
		checker := &PodStartupChecker{
			config: &config.PodStartupConfig{
				SyntheticPodNamespace: "test-namespace",
				EnabledCSIs:           tc.enabledCSIs,
			},
			k8sClientset: tc.k8sClient,
		}
		err := checker.deleteCSIResources(context.Background(), "timestampstr")
		g := NewWithT(t)
		tc.validateFunc(g, err, tc.k8sClient)
	}
}
