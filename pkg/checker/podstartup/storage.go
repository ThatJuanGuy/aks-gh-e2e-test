package podstartup

import (
	"context"
	"fmt"

	"github.com/Azure/cluster-health-monitor/pkg/config"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	azureDiskPVCName = "clusterhealthmonitor-azuredisk-pvc"
	azureFilePVCName = "clusterhealthmonitor-azurefile-pvc"
	azureBlobPVCName = "clusterhealthmonitor-azureblob-pvc"
)

var (
	// Storage class names must be variables to get their pointers.
	azureDiskStorageClassName = "managed-csi"                       // builtin storage class for AKS
	azureFileStorageClassName = "clusterhealthmonitor-azurefile-sc" // custom storage class for Azure File CSI
	azureBlobStorageClassName = "azureblob-nfs-premium"             // builtin storage class for AKS
)

func (c *PodStartupChecker) azureDiskPVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      azureDiskPVCName,
			Namespace: c.config.SyntheticPodNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: &azureDiskStorageClassName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
	}
}

func (c *PodStartupChecker) azureFileStorageClass() *storagev1.StorageClass {
	// Recommended mount options from: https://learn.microsoft.com/en-us/azure/aks/azure-csi-files-storage-provision#smb-shares
	allowVolumeExpansion := true
	reclaimPolicy := corev1.PersistentVolumeReclaimDelete
	volumeBindingMode := storagev1.VolumeBindingImmediate
	return &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StorageClass",
			APIVersion: "storage.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: azureFileStorageClassName,
		},
		Provisioner:          "file.csi.azure.com",
		AllowVolumeExpansion: &allowVolumeExpansion,
		ReclaimPolicy:        &reclaimPolicy,
		VolumeBindingMode:    &volumeBindingMode,
		MountOptions: []string{
			"dir_mode=0777",
			"file_mode=0777",
			"mfsymlinks",
			"cache=strict",
			"nosharesock",
			"actimeo=30",
			"nobrl",
		},
		Parameters: map[string]string{
			"skuName": "Premium_LRS",
		},
	}
}

func (c *PodStartupChecker) azureFilePVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      azureFilePVCName,
			Namespace: c.config.SyntheticPodNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			StorageClassName: &azureFileStorageClassName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
	}
}

func (c *PodStartupChecker) azureBlobPVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      azureBlobPVCName,
			Namespace: c.config.SyntheticPodNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			StorageClassName: &azureBlobStorageClassName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
	}
}

func (c *PodStartupChecker) createCSITestResources(ctx context.Context) error {
	for _, csiType := range c.config.EnabledCSITests {
		switch csiType {
		case config.CSITypeAzureDisk:
			_, err := c.k8sClientset.CoreV1().PersistentVolumeClaims(c.config.SyntheticPodNamespace).Create(ctx, c.azureDiskPVC(), metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create Azure Disk PVC: %w", err)
			}
		case config.CSITypeAzureFile:
			_, err := c.k8sClientset.StorageV1().StorageClasses().Create(ctx, c.azureFileStorageClass(), metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create azurefile-csi storage class: %w", err)
			}
			_, err = c.k8sClientset.CoreV1().PersistentVolumeClaims(c.config.SyntheticPodNamespace).Create(ctx, c.azureFilePVC(), metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create Azure File PVC: %w", err)
			}
		case config.CSITypeAzureBlob:
			_, err := c.k8sClientset.CoreV1().PersistentVolumeClaims(c.config.SyntheticPodNamespace).Create(ctx, c.azureBlobPVC(), metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create Azure Blob PVC: %w", err)
			}
		default:
			return fmt.Errorf("failed to create resources for unsupported CSI type: %s", csiType)
		}
	}
	return nil
}

func (c *PodStartupChecker) deleteCSITestResources(ctx context.Context) error {
	err := c.k8sClientset.CoreV1().PersistentVolumeClaims(c.config.SyntheticPodNamespace).Delete(ctx, azureDiskPVCName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Azure Disk PVC: %w", err)
	}

	err = c.k8sClientset.StorageV1().StorageClasses().Delete(ctx, azureFileStorageClassName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete azurefile-csi storage class: %w", err)
	}
	err = c.k8sClientset.CoreV1().PersistentVolumeClaims(c.config.SyntheticPodNamespace).Delete(ctx, azureFilePVCName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Azure File PVC: %w", err)
	}

	err = c.k8sClientset.CoreV1().PersistentVolumeClaims(c.config.SyntheticPodNamespace).Delete(ctx, azureBlobPVCName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Azure Blob PVC: %w", err)
	}

	return nil
}
