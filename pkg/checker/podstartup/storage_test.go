package podstartup

import (
	"testing"

	"github.com/Azure/cluster-health-monitor/pkg/config"
	. "github.com/onsi/gomega"
)

func TestStorageResources(t *testing.T) {
	g := NewWithT(t)
	checker := &PodStartupChecker{
		config: &config.PodStartupConfig{
			EnabledCSITests:       []config.CSIType{config.CSITypeAzureDisk, config.CSITypeAzureFile, config.CSITypeAzureBlob},
			SyntheticPodNamespace: "default",
		},
	}
	pods := checker.generateSyntheticPod("test")

	g.Expect(pods).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes).To(HaveLen(3)) // Expect 3 volumes for AzureDisk, AzureFile, and AzureBlob

	g.Expect(pods.Spec.Volumes[0]).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes[0].PersistentVolumeClaim).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes[0].PersistentVolumeClaim.ClaimName).To(Equal(checker.azureDiskPVC().Name))

	g.Expect(pods.Spec.Volumes[1]).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes[1].PersistentVolumeClaim).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes[1].PersistentVolumeClaim.ClaimName).To(Equal(checker.azureFilePVC().Name))

	g.Expect(pods.Spec.Volumes[2]).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes[2].PersistentVolumeClaim).ToNot(BeNil())
	g.Expect(pods.Spec.Volumes[2].PersistentVolumeClaim.ClaimName).To(Equal(checker.azureBlobPVC().Name))
}
