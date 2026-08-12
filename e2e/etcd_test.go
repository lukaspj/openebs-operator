//go:build e2e

package e2e

import (
	"context"
	"testing"

	storagev1alpha1 "github.com/aldershaab-it/openebs-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestEtcdVersionUpgrade(t *testing.T) {
	ctx := context.Background()

	stalePVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data-mayastor-etcd-0", Namespace: mayastorNamespace},
	}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(stalePVC), stalePVC); err == nil {
		if err := k8sClient.Delete(ctx, stalePVC); err != nil {
			t.Fatalf("delete stale etcd PVC: %v", err)
		}
		if err := wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(stalePVC), &corev1.PersistentVolumeClaim{}) != nil, nil
		}); err != nil {
			t.Fatalf("stale etcd PVC not deleted: %v", err)
		}
	}

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-etcd-upgrade"},
		Spec: storagev1alpha1.OpenEBSSpec{
		Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		Images: &storagev1alpha1.ImageConfig{
			Etcd: "openebs/etcd:3.5.6-debian-11-r10",
		},
		},
	}
	createCR(ctx, t, cr)
	waitForCRReady(ctx, t, "e2e-etcd-upgrade")
	waitForStatefulSet(ctx, t, "mayastor-etcd", mayastorNamespace, 1)

	sts := &appsv1.StatefulSet{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: "mayastor-etcd", Namespace: mayastorNamespace}, sts); err != nil {
		t.Fatalf("get etcd STS: %v", err)
	}
	if sts.Spec.Template.Spec.Containers[0].Image != "openebs/etcd:3.5.6-debian-11-r10" {
		t.Errorf("expected etcd image 3.5.6, got %s", sts.Spec.Template.Spec.Containers[0].Image)
	}

	updateCR(ctx, t, "e2e-etcd-upgrade", func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Images.Etcd = "openebs/etcd:3.6.4-debian-12-r0"
	})
	waitForCRReady(ctx, t, "e2e-etcd-upgrade")

	upgraded := &appsv1.StatefulSet{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: "mayastor-etcd", Namespace: mayastorNamespace}, upgraded); err != nil {
		t.Fatalf("get upgraded etcd: %v", err)
	}
	if upgraded.Spec.Template.Spec.Containers[0].Image != "openebs/etcd:3.6.4-debian-12-r0" {
		t.Errorf("expected etcd image 3.6.4 after upgrade, got %s", upgraded.Spec.Template.Spec.Containers[0].Image)
	}

	deleteCR(ctx, t, "e2e-etcd-upgrade")
}

func TestEtcdReplicaScaleUp(t *testing.T) {
	ctx := context.Background()
	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-etcd-scale"},
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true, EtcdReplicaCount: 1},
		},
	}
	createCR(ctx, t, cr)
	waitForCRReady(ctx, t, "e2e-etcd-scale")

	sts := &appsv1.StatefulSet{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: "mayastor-etcd", Namespace: mayastorNamespace}, sts); err != nil {
		t.Fatalf("get etcd: %v", err)
	}
	if *sts.Spec.Replicas != 1 {
		t.Errorf("expected 1 replica, got %d", *sts.Spec.Replicas)
	}

	updateCR(ctx, t, "e2e-etcd-scale", func(cr *storagev1alpha1.OpenEBS) {
		cr.Spec.Mayastor.EtcdReplicaCount = 2
	})
	waitForCRReady(ctx, t, "e2e-etcd-scale")

	waitForStatefulSetReplicas(ctx, t, "mayastor-etcd", mayastorNamespace, 2)

	deleteCR(ctx, t, "e2e-etcd-scale")
}

func TestEtcdStoragePVC(t *testing.T) {
	ctx := context.Background()
	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-etcd-pvc"},
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{
				Enabled:          true,
				EtcdStorageSize:  "5Gi",
			},
		},
	}
	createCR(ctx, t, cr)
	waitForCRReady(ctx, t, "e2e-etcd-pvc")

	sts := &appsv1.StatefulSet{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: "mayastor-etcd", Namespace: mayastorNamespace}, sts); err != nil {
		t.Fatalf("get etcd: %v", err)
	}
	if len(sts.Spec.VolumeClaimTemplates) == 0 {
		t.Fatal("expected PVC template on etcd StatefulSet")
	}
	pvcSize := sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
	expected := resource.MustParse("5Gi")
	if pvcSize.Cmp(expected) != 0 {
		t.Errorf("expected PVC size 5Gi, got %s", pvcSize.String())
	}

	deleteCR(ctx, t, "e2e-etcd-pvc")
}
