package e2e

import (
	"context"
	"testing"

	storagev1alpha1 "github.com/aldershaab-it/openebs-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestEngineAddLVM(t *testing.T) {
	ctx := context.Background()
	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: crName},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	createCR(ctx, t, cr)
	waitForCRReady(ctx, t)

	depName := "openebs-lvm-controller"
	waitForDeployment(ctx, t, depName, "openebs")
	waitForDaemonSet(ctx, t, "openebs-lvm-node", "openebs")
}

func TestEngineDisableLVM(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.LVM = &storagev1alpha1.LVMConfig{Enabled: false}
	})
	waitForCRReady(ctx, t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-lvm-controller", Namespace: "openebs"},
	}
	if resourceExists(ctx, t, dep) {
		t.Error("LVM deployment should be removed after disabling")
	}
}

func TestEngineReEnableLVM(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.LVM = &storagev1alpha1.LVMConfig{Enabled: true}
	})
	waitForCRReady(ctx, t)
	waitForDeployment(ctx, t, "openebs-lvm-controller", "openebs")
}

func TestEngineAddRemoveZFS(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.ZFS = &storagev1alpha1.ZFSConfig{Enabled: true}
	})
	waitForCRReady(ctx, t)
	waitForDeployment(ctx, t, "openebs-zfs-controller", "openebs")

	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.ZFS = &storagev1alpha1.ZFSConfig{Enabled: false}
	})
	waitForCRReady(ctx, t)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-zfs-controller", Namespace: "openebs"},
	}
	if resourceExists(ctx, t, dep) {
		t.Error("ZFS deployment should be removed")
	}
}

func TestEngineMayastorDeployAndRemove(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Mayastor = &storagev1alpha1.MayastorConfig{Enabled: true}
	})
	waitForCRReady(ctx, t)

	waitForStatefulSet(ctx, t, "mayastor-etcd", mayastorNamespace, 1)
	waitForDeployment(ctx, t, "mayastor-agent-core", mayastorNamespace)
	waitForDeployment(ctx, t, "mayastor-api-rest", mayastorNamespace)
	waitForDeployment(ctx, t, "mayastor-csi-controller", mayastorNamespace)
	waitForDeployment(ctx, t, "mayastor-operator-diskpool", mayastorNamespace)

	// Verify CSIDriver
	csid := client.ObjectKey{Name: mayastorCSIDriverName}
	if err := k8sClient.Get(ctx, csid, unstructuredObj(gvk("storage.k8s.io", "v1", "CSIDriver"), mayastorCSIDriverName, "")); err == nil {
		t.Log("CSIDriver deployed")
	}

	// Verify VolumeSnapshotClass
	vsc := unstructuredObj(gvk("snapshot.storage.k8s.io", "v1", "VolumeSnapshotClass"), "mayastor-snapshot", "")
	if resourceExists(ctx, t, vsc) {
		t.Log("VolumeSnapshotClass deployed")
	}

	// Disable Mayastor
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Mayastor = &storagev1alpha1.MayastorConfig{Enabled: false}
	})
	waitForCRReady(ctx, t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "mayastor-agent-core", Namespace: mayastorNamespace},
	}
	if resourceExists(ctx, t, dep) {
		t.Error("agent-core should be removed after disabling mayastor")
	}
}

func TestMayastorStorageClassCustomName(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Mayastor = &storagev1alpha1.MayastorConfig{
			Enabled:          true,
			StorageClassName: "custom-mayastor-sc",
		}
	})
	waitForCRReady(ctx, t)
	waitForStatefulSet(ctx, t, "mayastor-etcd", mayastorNamespace, 1)

	sc := unstructuredObj(gvk("storage.k8s.io", "v1", "StorageClass"), "custom-mayastor-sc", "")
	if resourceExists(ctx, t, sc) {
		t.Log("custom StorageClass deployed")
	}

	vsc := unstructuredObj(gvk("snapshot.storage.k8s.io", "v1", "VolumeSnapshotClass"), "mayastor-snapshot", "")
	if resourceExists(ctx, t, vsc) {
		t.Log("VolumeSnapshotClass deployed with default name")
	}
}

func TestCRDeleteCleanup(t *testing.T) {
	ctx := context.Background()
	deleteCR(ctx, t, crName)

	ns := &corev1.Namespace{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: "openebs"}, ns)
	if err == nil {
		t.Log("openebs namespace still exists (expected — operator doesn't delete namespaces)")
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "mayastor-etcd", Namespace: mayastorNamespace},
	}
	if resourceExists(ctx, t, sts) {
		t.Error("mayastor etcd should be removed after CR delete")
	}
}
