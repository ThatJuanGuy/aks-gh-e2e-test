package podstartup

import (
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *PodStartupChecker) azureDiskPVC() *corev1.PersistentVolumeClaim {
	storageClassName := "managed-csi"
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "azuredisk-pvc",
			Namespace: c.config.SyntheticPodNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: &storageClassName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
	}
}
func (c *PodStartupChecker) azureFileStorageClass() *storagev1.StorageClass {
	allowVolumeExpansion := true
	return &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StorageClass",
			APIVersion: "storage.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "azurefile-sc",
		},
		Provisioner:          "file.csi.azure.com",
		AllowVolumeExpansion: &allowVolumeExpansion,
		MountOptions: []string{
			"dir_mode=0777",
			"file_mode=0777",
			"uid=0",
			"gid=0",
			"mfsymlinks",
			"cache=strict",
			"actimeo=30",
			"nobrl", // disable sending byte range lock requests to the server and for applications which have challenges with posix locks
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
			Name:      "azurefile-pvc",
			Namespace: c.config.SyntheticPodNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			StorageClassName: &c.azureFileStorageClass().Name,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
	}
}

func (c *PodStartupChecker) azureBlobPVC() *corev1.PersistentVolumeClaim {
	storageClassname := "azureblob-nfs-premium"
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "azureblob-pvc",
			Namespace: c.config.SyntheticPodNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			StorageClassName: &storageClassname,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
	}
}
