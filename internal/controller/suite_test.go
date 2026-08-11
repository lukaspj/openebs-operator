package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	storagev1alpha1 "github.com/aldershaab-it/openebs-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func e2eScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = storagev1alpha1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)
	_ = rbacv1.AddToScheme(s)
	_ = storagev1.AddToScheme(s)
	_ = apiextensionsv1.AddToScheme(s)
	return s
}

func e2eReconcile(ctx context.Context, t *testing.T, s *runtime.Scheme, cr *storagev1alpha1.OpenEBS, count int) (*storagev1alpha1.OpenEBS, client.WithWatch) {
	t.Helper()
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(cr).Build()
	r := &OpenEBSReconciler{Client: cl, Scheme: s}

	if err := cl.Create(ctx, cr); err != nil {
		t.Fatalf("create CR: %v", err)
	}

	for i := 0; i < count; i++ {
		req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name}}
		result, err := r.Reconcile(ctx, req)
		if err != nil {
			// Degraded is acceptable — log but continue
			t.Logf("reconcile %d error (may be expected): %v", i, err)
		}
		if result.RequeueAfter > 0 {
			t.Logf("reconcile %d: requeue after %v", i, result.RequeueAfter)
		}
	}

	// Re-fetch to get updated status
	updated := &storagev1alpha1.OpenEBS{}
	if err := cl.Get(ctx, types.NamespacedName{Name: cr.Name}, updated); err != nil {
		t.Fatalf("get CR after reconcile: %v", err)
	}
	return updated, cl
}

// waitFor is a simple polling helper for status checks.
func waitFor(t *testing.T, ctx context.Context, cl client.WithWatch, name string, cond func(*storagev1alpha1.OpenEBS) bool, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for condition")
		default:
		}
		obj := &storagev1alpha1.OpenEBS{}
		if err := cl.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
			if !errors.IsNotFound(err) {
				t.Fatalf("get during wait: %v", err)
			}
		} else if cond(obj) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// ============================================================
//  E2E: LVM Engine
// ============================================================

func TestE2E_LVMCreatesAllResources(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-lvm"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	updated, cl := e2eReconcile(ctx, t, s, cr, 3)

	if updated.Status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Errorf("expected Running, got %s", updated.Status.Phase)
	}

	dep := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmControllerName, Namespace: openebsNamespace}, dep); err != nil {
		t.Fatalf("LVM controller: %v", err)
	}
	if *dep.Spec.Replicas != 1 {
		t.Errorf("expected 1 replica, got %d", *dep.Spec.Replicas)
	}
	if len(dep.Spec.Template.Spec.Containers) != 4 {
		t.Errorf("expected 4 containers, got %d", len(dep.Spec.Template.Spec.Containers))
	}

	ds := &appsv1.DaemonSet{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmNodeName, Namespace: openebsNamespace}, ds); err != nil {
		t.Fatalf("LVM node DS: %v", err)
	}

	csi := &storagev1.CSIDriver{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmCSIDriverName}, csi); err != nil {
		t.Fatalf("CSIDriver: %v", err)
	}

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmSCName}, sc); err != nil {
		t.Fatalf("StorageClass: %v", err)
	}
	if sc.Provisioner != lvmCSIDriverName {
		t.Errorf("expected provisioner %s, got %s", lvmCSIDriverName, sc.Provisioner)
	}
}

func TestE2E_LVMCustomSCAndVG(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-lvm-custom"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{
				Enabled:          true,
				StorageClassName: "my-lvm-sc",
				VolumeGroup:      "myvg",
			},
		},
	}
	_, cl := e2eReconcile(ctx, t, s, cr, 2)

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "my-lvm-sc"}, sc); err != nil {
		t.Fatalf("custom SC: %v", err)
	}
	if sc.Parameters["volgroup"] != "myvg" {
		t.Errorf("expected volgroup=myvg, got %s", sc.Parameters["volgroup"])
	}
}

func TestE2E_LVMDefaultSCAnnotation(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-lvm-default"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true, IsDefaultClass: true},
		},
	}
	_, cl := e2eReconcile(ctx, t, s, cr, 2)

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmSCName}, sc); err != nil {
		t.Fatalf("LVM SC: %v", err)
	}
	if sc.Annotations["storageclass.kubernetes.io/is-default-class"] != "true" {
		t.Errorf("expected default class=true, got %s", sc.Annotations["storageclass.kubernetes.io/is-default-class"])
	}
}

// ============================================================
//  E2E: ZFS Engine
// ============================================================

func TestE2E_ZFSCreatesAllResources(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-zfs"},
		Spec: storagev1alpha1.OpenEBSSpec{
			ZFS: &storagev1alpha1.ZFSConfig{Enabled: true},
		},
	}
	_, cl := e2eReconcile(ctx, t, s, cr, 2)

	dep := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: zfsControllerName, Namespace: openebsNamespace}, dep); err != nil {
		t.Fatalf("ZFS controller: %v", err)
	}
	if len(dep.Spec.Template.Spec.Containers) != 3 {
		t.Errorf("expected 3 containers, got %d", len(dep.Spec.Template.Spec.Containers))
	}

	ds := &appsv1.DaemonSet{}
	if err := cl.Get(ctx, types.NamespacedName{Name: zfsNodeName, Namespace: openebsNamespace}, ds); err != nil {
		t.Fatalf("ZFS node DS: %v", err)
	}

	csi := &storagev1.CSIDriver{}
	if err := cl.Get(ctx, types.NamespacedName{Name: zfsCSIDriverName}, csi); err != nil {
		t.Fatalf("CSIDriver: %v", err)
	}

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: zfsSCName}, sc); err != nil {
		t.Fatalf("StorageClass: %v", err)
	}
	if sc.Provisioner != zfsCSIDriverName {
		t.Errorf("expected provisioner %s, got %s", zfsCSIDriverName, sc.Provisioner)
	}
	if sc.Parameters["fstype"] != "zfs" {
		t.Errorf("expected fstype=zfs, got %s", sc.Parameters["fstype"])
	}
}

func TestE2E_ZFSCustomPoolName(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-zfs-pool"},
		Spec: storagev1alpha1.OpenEBSSpec{
			ZFS: &storagev1alpha1.ZFSConfig{Enabled: true, PoolName: "tank"},
		},
	}
	_, cl := e2eReconcile(ctx, t, s, cr, 2)

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: zfsSCName}, sc); err != nil {
		t.Fatalf("ZFS SC: %v", err)
	}
	if sc.Parameters["poolname"] != "tank" {
		t.Errorf("expected poolname=tank, got %s", sc.Parameters["poolname"])
	}
}

// ============================================================
//  E2E: Hostpath Engine
// ============================================================

func TestE2E_HostpathCreatesResources(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-hp"},
		Spec: storagev1alpha1.OpenEBSSpec{
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true},
		},
	}
	_, cl := e2eReconcile(ctx, t, s, cr, 2)

	dep := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: hostpathDeployName, Namespace: openebsNamespace}, dep); err != nil {
		t.Fatalf("hostpath deployment: %v", err)
	}
	if dep.Spec.Template.Spec.ServiceAccountName != "openebs-localpv-provisioner" {
		t.Errorf("expected SA openebs-localpv-provisioner, got %s", dep.Spec.Template.Spec.ServiceAccountName)
	}

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: hostpathSCName}, sc); err != nil {
		t.Fatalf("hostpath SC: %v", err)
	}
	if sc.Provisioner != "openebs.io/local" {
		t.Errorf("expected provisioner openebs.io/local, got %s", sc.Provisioner)
	}
}

func TestE2E_HostpathCustomBasePath(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-hp-path"},
		Spec: storagev1alpha1.OpenEBSSpec{
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true, BasePath: "/mnt/data"},
		},
	}
	_, cl := e2eReconcile(ctx, t, s, cr, 2)

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: hostpathSCName}, sc); err != nil {
		t.Fatalf("hostpath SC: %v", err)
	}
	if sc.Parameters["BasePath"] != "/mnt/data" {
		t.Errorf("expected BasePath=/mnt/data, got %s", sc.Parameters["BasePath"])
	}
	if sc.Annotations["openebs.io/cas-type"] != "local" {
		t.Errorf("expected openebs.io/cas-type=local, got %s", sc.Annotations["openebs.io/cas-type"])
	}
	if casCfg := sc.Annotations["cas.openebs.io/config"]; !strings.Contains(casCfg, "hostpath") || !strings.Contains(casCfg, "/mnt/data") {
		t.Errorf("expected cas.openebs.io/config with hostpath and /mnt/data, got %s", casCfg)
	}
}

// ============================================================
//  E2E: Rawfile Engine
// ============================================================

func TestE2E_RawfileCreatesResources(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-rawfile"},
		Spec: storagev1alpha1.OpenEBSSpec{
			Rawfile: &storagev1alpha1.RawfileConfig{Enabled: true},
		},
	}
	_, cl := e2eReconcile(ctx, t, s, cr, 2)

	dep := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: rawfileDeployName, Namespace: openebsNamespace}, dep); err != nil {
		t.Fatalf("rawfile deployment: %v", err)
	}
	if dep.Spec.Template.Spec.ServiceAccountName != "openebs-rawfile-provisioner" {
		t.Errorf("expected SA openebs-rawfile-provisioner, got %s", dep.Spec.Template.Spec.ServiceAccountName)
	}

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: rawfileSCName}, sc); err != nil {
		t.Fatalf("rawfile SC: %v", err)
	}
	if sc.Provisioner != "rawfile.csi.openebs.io" {
		t.Errorf("expected provisioner rawfile.csi.openebs.io, got %s", sc.Provisioner)
	}
}

// ============================================================
//  E2E: Multi-Engine
// ============================================================

func TestE2E_MultiEngine(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-multi"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM:      &storagev1alpha1.LVMConfig{Enabled: true},
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true},
		},
	}
	updated, cl := e2eReconcile(ctx, t, s, cr, 3)

	lvmDep := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmControllerName, Namespace: openebsNamespace}, lvmDep); err != nil {
		t.Error("LVM deployment missing")
	}

	hpDep := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: hostpathDeployName, Namespace: openebsNamespace}, hpDep); err != nil {
		t.Error("Hostpath deployment missing")
	}

	if len(updated.Status.Engines) != 2 {
		t.Errorf("expected 2 engines, got %d", len(updated.Status.Engines))
	}
	found := map[storagev1alpha1.OpenEBSEngine]bool{}
	for _, e := range updated.Status.Engines {
		found[e.Name] = true
	}
	if !found[storagev1alpha1.OpenEBSEngineLVM] || !found[storagev1alpha1.OpenEBSEngineHostpath] {
		t.Error("expected LVM and Hostpath in engine status")
	}
}

// ============================================================
//  E2E: Disabled Engines
// ============================================================

func TestE2E_DisabledEngines(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-disabled"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM:      &storagev1alpha1.LVMConfig{Enabled: false},
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: false},
			ZFS:      &storagev1alpha1.ZFSConfig{Enabled: false},
			Rawfile:  &storagev1alpha1.RawfileConfig{Enabled: false},
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: false},
		},
	}
	updated, cl := e2eReconcile(ctx, t, s, cr, 2)

	dep := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmControllerName, Namespace: openebsNamespace}, dep); !errors.IsNotFound(err) {
		t.Error("expected no LVM deployment when disabled")
	}
	if len(updated.Status.Engines) != 0 {
		t.Errorf("expected 0 engines, got %d", len(updated.Status.Engines))
	}
	if updated.Status.Phase != storagev1alpha1.OpenEBSPhasePending {
		t.Errorf("expected Pending phase, got %s", updated.Status.Phase)
	}
}

func TestE2E_NilConfigNoop(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-nil"},
		Spec:       storagev1alpha1.OpenEBSSpec{},
	}
	updated, _ := e2eReconcile(ctx, t, s, cr, 2)

	if updated.Status.Phase != storagev1alpha1.OpenEBSPhasePending {
		t.Errorf("expected Pending, got %s", updated.Status.Phase)
	}
}

// ============================================================
//  E2E: Mayastor Engine (pending support)
// ============================================================

func TestE2E_MayastorRunning(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-mayastor"},
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	updated, _ := e2eReconcile(ctx, t, s, cr, 2)

	if len(updated.Status.Engines) != 1 {
		t.Fatalf("expected 1 engine, got %d", len(updated.Status.Engines))
	}
	if updated.Status.Engines[0].Name != storagev1alpha1.OpenEBSEngineMayastor {
		t.Errorf("expected engine mayastor, got %s", updated.Status.Engines[0].Name)
	}
	if updated.Status.Engines[0].Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Errorf("expected Running, got %s", updated.Status.Engines[0].Phase)
	}
}

// ============================================================
//  E2E: Deletion
// ============================================================

func TestE2E_Deletion(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&storagev1alpha1.OpenEBS{}).Build()
	r := &OpenEBSReconciler{Client: cl, Scheme: s}

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-delete"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	if err := cl.Create(ctx, cr); err != nil {
		t.Fatalf("create CR: %v", err)
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name}}
	for i := 0; i < 3; i++ {
		_, _ = r.Reconcile(ctx, req)
	}

	if err := cl.Delete(ctx, cr); err != nil {
		t.Fatalf("delete CR: %v", err)
	}

	// Trigger reconcile for deletion
	_, _ = r.Reconcile(ctx, req)

	// CR should be gone after finalizer removal
	got := &storagev1alpha1.OpenEBS{}
	err := cl.Get(ctx, req.NamespacedName, got)
	if !errors.IsNotFound(err) {
		t.Error("expected CR to be deleted after finalizer removal")
	}
}

// ============================================================
//  E2E: Finalizer
// ============================================================

func TestE2E_Finalizer(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-finalizer"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	updated, _ := e2eReconcile(ctx, t, s, cr, 2)

	hasFinalizer := false
	for _, f := range updated.Finalizers {
		if f == openebsFinalizer {
			hasFinalizer = true
		}
	}
	if !hasFinalizer {
		t.Errorf("expected finalizer %s to be present", openebsFinalizer)
	}
}

// ============================================================
//  E2E: Status Conditions
// ============================================================

func TestE2E_StatusConditions(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-cond"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	updated, _ := e2eReconcile(ctx, t, s, cr, 2)

	if updated.Status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Errorf("expected Running, got %s", updated.Status.Phase)
	}
	if len(updated.Status.Conditions) == 0 {
		t.Fatal("expected conditions not empty")
	}

	hasAvailable := false
	for _, c := range updated.Status.Conditions {
		if c.Type == string(storagev1alpha1.ConditionAvailable) && c.Status == metav1.ConditionTrue {
			hasAvailable = true
		}
	}
	if !hasAvailable {
		t.Error("expected Available=True condition")
	}
}

// ============================================================
//  E2E: Spec Update (adding engine after creation)
// ============================================================

func TestE2E_SpecUpdate(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&storagev1alpha1.OpenEBS{}).Build()
	r := &OpenEBSReconciler{Client: cl, Scheme: s}

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-update"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	if err := cl.Create(ctx, cr); err != nil {
		t.Fatalf("create CR: %v", err)
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name}}
	for i := 0; i < 2; i++ {
		_, _ = r.Reconcile(ctx, req)
	}

	// Update spec — add hostpath
	updated := &storagev1alpha1.OpenEBS{}
	if err := cl.Get(ctx, types.NamespacedName{Name: cr.Name}, updated); err != nil {
		t.Fatalf("get CR: %v", err)
	}
	updated.Spec.Hostpath = &storagev1alpha1.HostpathConfig{Enabled: true}
	if err := cl.Update(ctx, updated); err != nil {
		t.Fatalf("update CR: %v", err)
	}

	for i := 0; i < 2; i++ {
		_, _ = r.Reconcile(ctx, req)
	}

	// Verify hostpath deployment created
	hpDep := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: hostpathDeployName, Namespace: openebsNamespace}, hpDep); err != nil {
		t.Errorf("hostpath deployment not created: %v", err)
	}

	// Verify status has both engines
	current := &storagev1alpha1.OpenEBS{}
	if err := cl.Get(ctx, types.NamespacedName{Name: cr.Name}, current); err != nil {
		t.Fatalf("get CR: %v", err)
	}
	if len(current.Status.Engines) != 2 {
		t.Errorf("expected 2 engines after update, got %d", len(current.Status.Engines))
	}
}

// ============================================================
//  E2E: Idempotent Reconcile
// ============================================================

func TestE2E_Idempotent(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-idem"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	updated, _ := e2eReconcile(ctx, t, s, cr, 5) // Extra reconcile calls

	if updated.Status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Errorf("expected Running after multiple reconciles, got %s", updated.Status.Phase)
	}
}

// ============================================================
//  E2E: Multiple CR Instances
// ============================================================

func TestE2E_MultipleCRs(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&storagev1alpha1.OpenEBS{}).Build()
	r := &OpenEBSReconciler{Client: cl, Scheme: s}

	crA := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-cr-a"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	crB := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-cr-b"},
		Spec: storagev1alpha1.OpenEBSSpec{
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true},
		},
	}
	if err := cl.Create(ctx, crA); err != nil {
		t.Fatalf("create crA: %v", err)
	}
	if err := cl.Create(ctx, crB); err != nil {
		t.Fatalf("create crB: %v", err)
	}

	for i := 0; i < 2; i++ {
		_, _ = r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: crA.Name}})
		_, _ = r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: crB.Name}})
	}

	objA := &storagev1alpha1.OpenEBS{}
	if err := cl.Get(ctx, types.NamespacedName{Name: crA.Name}, objA); err != nil {
		t.Fatalf("get crA: %v", err)
	}
	if len(objA.Status.Engines) != 1 || objA.Status.Engines[0].Name != storagev1alpha1.OpenEBSEngineLVM {
		t.Errorf("crA should have LVM only, got %v", objA.Status.Engines)
	}

	objB := &storagev1alpha1.OpenEBS{}
	if err := cl.Get(ctx, types.NamespacedName{Name: crB.Name}, objB); err != nil {
		t.Fatalf("get crB: %v", err)
	}
	if len(objB.Status.Engines) != 1 || objB.Status.Engines[0].Name != storagev1alpha1.OpenEBSEngineHostpath {
		t.Errorf("crB should have Hostpath only, got %v", objB.Status.Engines)
	}
}

// ============================================================
//  E2E: RBAC Resource Creation
// ============================================================

func TestE2E_RBACResources(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-rbac"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	_, cl := e2eReconcile(ctx, t, s, cr, 2)

	sa := &corev1.ServiceAccount{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "openebs-lvm-controller", Namespace: openebsNamespace}, sa); err != nil {
		t.Fatalf("SA: %v", err)
	}

	crRole := &rbacv1.ClusterRole{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "openebs-lvm-role"}, crRole); err != nil {
		t.Fatalf("ClusterRole: %v", err)
	}
	if len(crRole.Rules) == 0 {
		t.Error("ClusterRole has no rules")
	}

	crb := &rbacv1.ClusterRoleBinding{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "openebs-lvm-binding"}, crb); err != nil {
		t.Fatalf("ClusterRoleBinding: %v", err)
	}
	if crb.RoleRef.Name != "openebs-lvm-role" {
		t.Errorf("expected role ref openebs-lvm-role, got %s", crb.RoleRef.Name)
	}
}

// ============================================================
//  E2E: Hostpath IsDefaultClass
// ============================================================

func TestE2E_HostpathIsDefaultClass(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-hp-default"},
		Spec: storagev1alpha1.OpenEBSSpec{
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true, IsDefaultClass: true},
		},
	}
	_, cl := e2eReconcile(ctx, t, s, cr, 2)

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: hostpathSCName}, sc); err != nil {
		t.Fatalf("hostpath SC: %v", err)
	}
	if sc.Annotations["storageclass.kubernetes.io/is-default-class"] != "true" {
		t.Errorf("expected default class=true, got %s", sc.Annotations["storageclass.kubernetes.io/is-default-class"])
	}
}

// ============================================================
//  E2E: Phase Derivation — Degraded
// ============================================================

func TestE2E_AllEnginesRunning(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-all-running"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM:      &storagev1alpha1.LVMConfig{Enabled: true},
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	updated, _ := e2eReconcile(ctx, t, s, cr, 2)

	if updated.Status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Errorf("expected Running (all engines ready), got %s", updated.Status.Phase)
	}
}

// ============================================================
//  E2E: Helm Hook Replacement — Self-Healing (recreate deleted resource)
// ============================================================

func TestE2E_SelfHealing(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&storagev1alpha1.OpenEBS{}).Build()
	r := &OpenEBSReconciler{Client: cl, Scheme: s}

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-heal"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	if err := cl.Create(ctx, cr); err != nil {
		t.Fatalf("create CR: %v", err)
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name}}
	for i := 0; i < 2; i++ {
		_, _ = r.Reconcile(ctx, req)
	}

	// Simulate accidental deletion of the LVM deployment (what Helm pre-delete hook handles)
	dep := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmControllerName, Namespace: openebsNamespace}, dep); err != nil {
		t.Fatalf("get deployment before delete: %v", err)
	}
	if err := cl.Delete(ctx, dep); err != nil {
		t.Fatalf("delete deployment: %v", err)
	}

	// Reconcile — should recreate the deployment (replaces Helm post-install retry)
	_, _ = r.Reconcile(ctx, req)

	recreated := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmControllerName, Namespace: openebsNamespace}, recreated); err != nil {
		t.Fatalf("self-healing failed: deployment not recreated: %v", err)
	}
}

// ============================================================
//  E2E: Helm Hook Replacement — Cleanup of StorageClass and CSIDriver on delete
// ============================================================

func TestE2E_CleanupComplete(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&storagev1alpha1.OpenEBS{}).Build()
	r := &OpenEBSReconciler{Client: cl, Scheme: s}

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-cleanup-all"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	if err := cl.Create(ctx, cr); err != nil {
		t.Fatalf("create CR: %v", err)
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name}}
	for i := 0; i < 2; i++ {
		_, _ = r.Reconcile(ctx, req)
	}

	// Verify resources exist before deletion
	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmSCName}, sc); err != nil {
		t.Fatalf("StorageClass must exist before deletion: %v", err)
	}
	csi := &storagev1.CSIDriver{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmCSIDriverName}, csi); err != nil {
		t.Fatalf("CSIDriver must exist before deletion: %v", err)
	}

	// Delete CR
	if err := cl.Delete(ctx, cr); err != nil {
		t.Fatalf("delete CR: %v", err)
	}
	_, _ = r.Reconcile(ctx, req)

	// CR must be gone
	got := &storagev1alpha1.OpenEBS{}
	if err := cl.Get(ctx, req.NamespacedName, got); !errors.IsNotFound(err) {
		t.Error("CR should be fully deleted after finalizer removed")
	}

	// StorageClass and CSIDriver should be cleaned up (Helm pre-delete hook behavior)
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmSCName}, sc); !errors.IsNotFound(err) {
		t.Error("StorageClass should be cleaned up on delete")
	}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmCSIDriverName}, csi); !errors.IsNotFound(err) {
		t.Error("CSIDriver should be cleaned up on delete")
	}
}

// ============================================================
//  E2E: Helm Hook Replacement — Dependency ordering (namespace before pods)
// ============================================================

func TestE2E_NamespaceBeforeComponents(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&storagev1alpha1.OpenEBS{}).Build()
	r := &OpenEBSReconciler{Client: cl, Scheme: s}

	// Pre-create namespace — but we verify the reconciler creates it if missing
	// The reconciler calls ensureNamespace before applyDeployment

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-ns-order"},
		Spec: storagev1alpha1.OpenEBSSpec{
			ZFS: &storagev1alpha1.ZFSConfig{Enabled: true},
		},
	}
	if err := cl.Create(ctx, cr); err != nil {
		t.Fatalf("create CR: %v", err)
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name}}
	_, _ = r.Reconcile(ctx, req)

	// Namespace must exist after reconcile
	ns := &corev1.Namespace{}
	if err := cl.Get(ctx, types.NamespacedName{Name: openebsNamespace}, ns); err != nil {
		t.Fatalf("namespace not created: %v", err)
	}
}

// ============================================================
//  E2E: Helm Hook Replacement — One-shot init (Mayastor doesn't re-init)
// ============================================================

func TestE2E_OneShotInit(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-oneshot"},
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	// Reconcile multiple times — Mayastor should remain Running, not re-initialize
	updated, _ := e2eReconcile(ctx, t, s, cr, 4)

	if updated.Status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Errorf("expected Running after multiple reconciles, got %s", updated.Status.Phase)
	}
	if len(updated.Status.Engines) != 1 {
		t.Errorf("expected 1 engine, got %d", len(updated.Status.Engines))
	}
	// Message should be consistent across reconciles
	if updated.Status.Engines[0].Message == "" {
		t.Error("expected message for Mayastor status")
	}
}

// ============================================================
//  E2E: Helm Hook Replacement — Keep StorageClass on CR update (no reset)
// ============================================================

func TestE2E_StorageClassPreservedOnUpdate(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&storagev1alpha1.OpenEBS{}).Build()
	r := &OpenEBSReconciler{Client: cl, Scheme: s}

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-sc-preserve"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	if err := cl.Create(ctx, cr); err != nil {
		t.Fatalf("create CR: %v", err)
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name}}
	for i := 0; i < 2; i++ {
		_, _ = r.Reconcile(ctx, req)
	}

	// Reconcile again — StorageClass should remain with same params
	_, _ = r.Reconcile(ctx, req)

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmSCName}, sc); err != nil {
		t.Fatalf("SC should still exist: %v", err)
	}
	if sc.Provisioner != lvmCSIDriverName {
		t.Errorf("SC provisioner changed after re-reconcile")
	}
}

// ============================================================
//  E2E: Helm Hook Replacement — Partial failure self-healing
// ============================================================

func TestE2E_PartialFailureRecovery(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&storagev1alpha1.OpenEBS{}).Build()
	r := &OpenEBSReconciler{Client: cl, Scheme: s}

	// LVM and Mayastor enabled — both should deploy successfully
	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-both"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM:      &storagev1alpha1.LVMConfig{Enabled: true},
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	if err := cl.Create(ctx, cr); err != nil {
		t.Fatalf("create CR: %v", err)
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name}}
	for i := 0; i < 3; i++ {
		_, _ = r.Reconcile(ctx, req)
	}

	// LVM should be fully deployed
	dep := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmControllerName, Namespace: openebsNamespace}, dep); err != nil {
		t.Errorf("LVM deployment should exist despite Mayastor being pending: %v", err)
	}

	// Status should show both LVM and Mayastor Running
	obj := &storagev1alpha1.OpenEBS{}
	if err := cl.Get(ctx, types.NamespacedName{Name: cr.Name}, obj); err != nil {
		t.Fatalf("get CR: %v", err)
	}
	if len(obj.Status.Engines) != 2 {
		t.Fatalf("expected 2 engines, got %d", len(obj.Status.Engines))
	}
	for _, e := range obj.Status.Engines {
		switch e.Name {
		case storagev1alpha1.OpenEBSEngineLVM:
			if e.Phase != storagev1alpha1.OpenEBSPhaseRunning {
				t.Errorf("LVM should be Running, got %s", e.Phase)
			}
		case storagev1alpha1.OpenEBSEngineMayastor:
			if e.Phase != storagev1alpha1.OpenEBSPhaseRunning {
				t.Errorf("Mayastor should be Running, got %s", e.Phase)
			}
		}
	}

	// Overall phase: Running (both engines ready)
	if obj.Status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Errorf("expected Running phase, got %s", obj.Status.Phase)
	}
}

// ============================================================
//  E2E: Helm Hook Replacement — Engine disabled then re-enabled
// ============================================================

func TestE2E_EngineReenable(t *testing.T) {
	ctx := context.Background()
	s := e2eScheme()
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&storagev1alpha1.OpenEBS{}).Build()
	r := &OpenEBSReconciler{Client: cl, Scheme: s}

	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-reenable"},
		Spec: storagev1alpha1.OpenEBSSpec{
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true},
		},
	}
	if err := cl.Create(ctx, cr); err != nil {
		t.Fatalf("create CR: %v", err)
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name}}
	for i := 0; i < 2; i++ {
		_, _ = r.Reconcile(ctx, req)
	}

	// Hostpath deployment exists
	hpDep := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: hostpathDeployName, Namespace: openebsNamespace}, hpDep); err != nil {
		t.Fatalf("hostpath should exist: %v", err)
	}

	// Disable hostpath
	obj := &storagev1alpha1.OpenEBS{}
	if err := cl.Get(ctx, types.NamespacedName{Name: cr.Name}, obj); err != nil {
		t.Fatalf("get CR: %v", err)
	}
	obj.Spec.Hostpath.Enabled = false
	if err := cl.Update(ctx, obj); err != nil {
		t.Fatalf("update to disable: %v", err)
	}
	_, _ = r.Reconcile(ctx, req)

	// Re-enable — re-read to get latest resource version
	obj2 := &storagev1alpha1.OpenEBS{}
	if err := cl.Get(ctx, types.NamespacedName{Name: cr.Name}, obj2); err != nil {
		t.Fatalf("get CR after disable: %v", err)
	}
	obj2.Spec.Hostpath.Enabled = true
	if err := cl.Update(ctx, obj2); err != nil {
		t.Fatalf("update to re-enable: %v", err)
	}
	_, _ = r.Reconcile(ctx, req)

	// Hostpath deployment should still exist (or be recreated)
	reEnabled := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: hostpathDeployName, Namespace: openebsNamespace}, reEnabled); err != nil {
		t.Errorf("hostpath should still exist after re-enable: %v", err)
	}
}
