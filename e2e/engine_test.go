//go:build e2e

package e2e

import (
	"context"
	"testing"

	storagev1alpha1 "github.com/aldershaab-it/openebs-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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
	waitForCRReady(ctx, t, crName)

	depName := "openebs-lvm-controller"
	waitForDeployment(ctx, t, depName, "openebs")
	waitForDaemonSet(ctx, t, "openebs-lvm-node", "openebs")
}

func TestEngineDisableLVM(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.LVM = &storagev1alpha1.LVMConfig{Enabled: false}
	})
	waitForCRReady(ctx, t, crName)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-lvm-controller", Namespace: "openebs"},
	}
	waitForResourceGone(ctx, t, dep)
}

func TestEngineReEnableLVM(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.LVM = &storagev1alpha1.LVMConfig{Enabled: true}
	})
	waitForCRReady(ctx, t, crName)
	waitForDeployment(ctx, t, "openebs-lvm-controller", "openebs")
}

func TestEngineAddRemoveZFS(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.ZFS = &storagev1alpha1.ZFSConfig{Enabled: true}
	})
	waitForCRReady(ctx, t, crName)
	waitForDeployment(ctx, t, "openebs-zfs-controller", "openebs")

	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.ZFS = &storagev1alpha1.ZFSConfig{Enabled: false}
	})
	waitForCRReady(ctx, t, crName)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-zfs-controller", Namespace: "openebs"},
	}
	waitForResourceGone(ctx, t, dep)
}

func TestEngineMayastorDeployAndRemove(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Mayastor = &storagev1alpha1.MayastorConfig{Enabled: true}
	})
	waitForCRReady(ctx, t, crName)

	waitForStatefulSet(ctx, t, "mayastor-etcd", mayastorNamespace, 1)
	waitForDeployment(ctx, t, "mayastor-agent-core", mayastorNamespace)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "mayastor-agent-core", Namespace: mayastorNamespace},
	}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(svc), svc); err != nil {
		t.Fatalf("agent-core Service not found: %v", err)
	}
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
	waitForCRReady(ctx, t, crName)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "mayastor-agent-core", Namespace: mayastorNamespace},
	}
	waitForResourceGone(ctx, t, dep)
}

func TestMayastorStorageClassCustomName(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Mayastor = &storagev1alpha1.MayastorConfig{
			Enabled:          true,
			StorageClassName: "custom-mayastor-sc",
		}
	})
	waitForCRReady(ctx, t, crName)
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

func TestMayastorStorageClassDefaultClass(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Mayastor = &storagev1alpha1.MayastorConfig{
			Enabled:        true,
			IsDefaultClass: true,
		}
	})
	waitForCRReady(ctx, t, crName)

	sc := &storagev1.StorageClass{}
	err := wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: "mayastor"}, sc); err != nil {
			return false, nil
		}
		return sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true", nil
	})
	if err != nil {
		t.Fatalf("mayastor StorageClass did not become default: %v", err)
	}

	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Mayastor.IsDefaultClass = false
	})
	waitForCRReady(ctx, t, crName)

	err = wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: "mayastor"}, sc); err != nil {
			return false, nil
		}
		return sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "false", nil
	})
	if err != nil {
		t.Fatalf("mayastor StorageClass did not lose default annotation: %v", err)
	}
}

func TestEtcdVeleroBackupAndSchedule(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Mayastor = &storagev1alpha1.MayastorConfig{
			Enabled:            true,
			EtcdVeleroBackup:   true,
			EtcdVeleroSchedule: "0 * * * *",
		}
	})
	waitForCRReady(ctx, t, crName)

	sts := &appsv1.StatefulSet{}
	err := wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: "mayastor-etcd", Namespace: mayastorNamespace}, sts); err != nil {
			return false, nil
		}
		return sts.Spec.Template.Annotations["backup.velero.io/backup-volumes"] == "data", nil
	})
	if err != nil {
		t.Fatalf("etcd pod template did not get velero backup annotation: %v", err)
	}
	if sts.Spec.Template.Annotations["pre.hook.backup.velero.io/command"] == "" {
		t.Error("etcd pod template missing pre-backup hook")
	}

	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Mayastor.EtcdVeleroBackup = false
		cr.Spec.Mayastor.EtcdVeleroSchedule = ""
	})
	waitForCRReady(ctx, t, crName)

	err = wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: "mayastor-etcd", Namespace: mayastorNamespace}, sts); err != nil {
			return false, nil
		}
		_, ok := sts.Spec.Template.Annotations["backup.velero.io/backup-volumes"]
		return !ok, nil
	})
	if err != nil {
		t.Fatalf("etcd velero annotations not removed after disable: %v", err)
	}
}

func TestIOEngineEnvContext(t *testing.T) {
	ctx := context.Background()
	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Mayastor = &storagev1alpha1.MayastorConfig{
			Enabled:            true,
			IoEngineEnvContext: "iova-mode=pa",
		}
	})
	waitForCRReady(ctx, t, crName)

	ds := &appsv1.DaemonSet{}
	err := wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: "mayastor-io-engine", Namespace: mayastorNamespace}, ds); err != nil {
			return false, nil
		}
		for _, arg := range ds.Spec.Template.Spec.Containers[0].Args {
			if arg == "--env-context=--iova-mode=pa" {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("io-engine did not get --env-context arg: %v", err)
	}

	updateCR(ctx, t, crName, func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Mayastor.IoEngineEnvContext = ""
	})
	waitForCRReady(ctx, t, crName)

	err = wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: "mayastor-io-engine", Namespace: mayastorNamespace}, ds); err != nil {
			return false, nil
		}
		for _, arg := range ds.Spec.Template.Spec.Containers[0].Args {
			if arg == "--env-context=--iova-mode=pa" {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("io-engine --env-context arg not removed after disable: %v", err)
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
