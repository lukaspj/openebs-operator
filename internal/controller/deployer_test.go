package controller

import (
	"context"
	"testing"

	storagev1alpha1 "github.com/aldershaab-it/openebs-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeployerEnsureNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := &Deployer{Client: cl, Scheme: scheme}

	ctx := context.Background()
	if err := d.ensureNamespace(ctx, "openebs"); err != nil {
		t.Fatalf("ensureNamespace failed: %v", err)
	}

	ns := &corev1.Namespace{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "openebs"}, ns); err != nil {
		t.Fatalf("namespace not created: %v", err)
	}
}

func TestDeployerEnsureNamespaceIdempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := &Deployer{Client: cl, Scheme: scheme}

	ctx := context.Background()
	if err := d.ensureNamespace(ctx, "openebs"); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if err := d.ensureNamespace(ctx, "openebs"); err != nil {
		t.Fatalf("second call failed: %v", err)
	}
}

func TestDeployLVMWithFakeClient(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}

	ctx := context.Background()
	status := d.deployLVM(ctx)

	if status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Errorf("expected phase Running, got %s: %s", status.Phase, status.Message)
	}
	if status.Name != storagev1alpha1.OpenEBSEngineLVM {
		t.Errorf("expected engine LVM, got %s", status.Name)
	}
}

func TestDeployHostpathWithFakeClient(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true},
		},
	}
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}

	ctx := context.Background()
	status := d.deployHostpath(ctx)

	if status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Errorf("expected phase Running, got %s: %s", status.Phase, status.Message)
	}
}

func TestDeployZFSWithFakeClient(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			ZFS: &storagev1alpha1.ZFSConfig{Enabled: true},
		},
	}
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}

	ctx := context.Background()
	status := d.deployZFS(ctx)

	if status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Errorf("expected phase Running, got %s: %s", status.Phase, status.Message)
	}
}

func TestDeployRawfileWithFakeClient(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Rawfile: &storagev1alpha1.RawfileConfig{Enabled: true},
		},
	}
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}

	ctx := context.Background()
	status := d.deployRawfile(ctx)

	if status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Errorf("expected phase Running, got %s: %s", status.Phase, status.Message)
	}
}

func TestDeployMayastorNotImplemented(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}

	ctx := context.Background()
	status := d.deployMayastor(ctx)

	if status.Phase != storagev1alpha1.OpenEBSPhaseInstalling {
		t.Errorf("expected phase Installing (not implemented), got %s", status.Phase)
	}
}

func TestDeployerApplyCreatesNewResource(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := &Deployer{Client: cl, Scheme: scheme}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	if err := d.applyRBAC(ctx, sa, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "test-cr"}}, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "test-crb"}}); err != nil {
		t.Fatalf("applyRBAC failed: %v", err)
	}

	// Verify SA was created
	got := &corev1.ServiceAccount{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "test-sa", Namespace: "default"}, got); err != nil {
		t.Fatalf("SA not found: %v", err)
	}
	if got.Name != "test-sa" {
		t.Errorf("expected name test-sa, got %s", got.Name)
	}
}

func TestDeployerCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)

	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM:      &storagev1alpha1.LVMConfig{Enabled: true},
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true},
		},
	}

	ctx := context.Background()

	t.Run("cleanup removes nil resources without error", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).Build()
		d := &Deployer{Client: cl, Scheme: scheme, instance: instance}

		err := d.cleanup(ctx)
		if err != nil {
			t.Errorf("cleanup should not error on empty state: %v", err)
		}
	})

	t.Run("cleanup removes existing resources", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).Build()
		d := &Deployer{Client: cl, Scheme: scheme, instance: instance}

		dep := lvmControllerDeployment(instance)
		if err := cl.Create(ctx, dep); err != nil {
			t.Fatalf("failed to create deployment: %v", err)
		}

		err := d.cleanup(ctx)
		if err != nil {
			t.Errorf("cleanup failed: %v", err)
		}

		got := &appsv1.Deployment{}
		err = cl.Get(ctx, types.NamespacedName{Name: lvmControllerName, Namespace: openebsNamespace}, got)
		if err == nil {
			t.Error("expected deployment to be deleted")
		}
	})
}

func TestDeployerApplyIdempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := &Deployer{Client: cl, Scheme: scheme}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa-idem",
			Namespace: "default",
		},
	}
	ctx := context.Background()

	if err := d.apply(ctx, sa); err != nil {
		t.Fatalf("first apply failed: %v", err)
	}
	if err := d.apply(ctx, sa); err != nil {
		t.Fatalf("second apply failed: %v", err)
	}

	got := &corev1.ServiceAccount{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "test-sa-idem", Namespace: "default"}, got); err != nil {
		t.Fatalf("SA not found after second apply: %v", err)
	}
}

func TestDeployLVMWithCustomConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{
				Enabled:          true,
				StorageClassName: "custom-lvm",
				VolumeGroup:      "custom-vg",
				IsDefaultClass:   true,
			},
		},
	}
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}
	ctx := context.Background()

	status := d.deployLVM(ctx)
	if status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Fatalf("expected Running, got %s: %s", status.Phase, status.Message)
	}

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "custom-lvm"}, sc); err != nil {
		t.Fatalf("custom StorageClass not found: %v", err)
	}
	if sc.Parameters["volgroup"] != "custom-vg" {
		t.Errorf("expected volgroup custom-vg, got %s", sc.Parameters["volgroup"])
	}
	if sc.Annotations["storageclass.kubernetes.io/is-default-class"] != "true" {
		t.Error("expected default class annotation to be true")
	}
}

func TestDeployHostpathWithCustomBasePath(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Hostpath: &storagev1alpha1.HostpathConfig{
				Enabled:          true,
				BasePath:         "/mnt/custom-hostpath",
				StorageClassName: "custom-hp",
				IsDefaultClass:   true,
			},
		},
	}
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}
	ctx := context.Background()

	status := d.deployHostpath(ctx)
	if status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Fatalf("expected Running, got %s: %s", status.Phase, status.Message)
	}

	dep := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: hostpathDeployName, Namespace: openebsNamespace}, dep); err != nil {
		t.Fatalf("hostpath deployment not found: %v", err)
	}

	hasBasePath := false
	for _, envVar := range dep.Spec.Template.Spec.Containers[0].Env {
		if envVar.Name == "OPENEBS_IO_BASE_PATH" && envVar.Value == "/mnt/custom-hostpath" {
			hasBasePath = true
		}
	}
	if !hasBasePath {
		t.Error("expected OPENEBS_IO_BASE_PATH=/mnt/custom-hostpath")
	}

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "custom-hp"}, sc); err != nil {
		t.Fatalf("custom hostpath SC not found: %v", err)
	}
}

func TestDeployZFSWithCustomPoolName(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			ZFS: &storagev1alpha1.ZFSConfig{
				Enabled:  true,
				PoolName: "tank",
			},
		},
	}
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}
	ctx := context.Background()

	status := d.deployZFS(ctx)
	if status.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Fatalf("expected Running, got %s: %s", status.Phase, status.Message)
	}

	sc := &storagev1.StorageClass{}
	if err := cl.Get(ctx, types.NamespacedName{Name: zfsSCName}, sc); err != nil {
		t.Fatalf("ZFS StorageClass not found: %v", err)
	}
	if sc.Parameters["poolname"] != "tank" {
		t.Errorf("expected poolname tank, got %s", sc.Parameters["poolname"])
	}
}

func TestDeployerRemovesNilResources(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)

	instance := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}
	ctx := context.Background()

	err := d.cleanup(ctx)
	if err != nil {
		t.Errorf("cleanup should not error when no engines enabled: %v", err)
	}
}

func TestDeployerRBACResourcesCreated(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := &Deployer{Client: cl, Scheme: scheme}
	ctx := context.Background()

	sa := lvmServiceAccount()
	cr := lvmClusterRole()
	crb := lvmClusterRoleBinding()

	if err := d.applyRBAC(ctx, sa, cr, crb); err != nil {
		t.Fatalf("applyRBAC failed: %v", err)
	}

	gotSA := &corev1.ServiceAccount{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "openebs-lvm-controller", Namespace: openebsNamespace}, gotSA); err != nil {
		t.Errorf("ServiceAccount not created: %v", err)
	}

	gotCR := &rbacv1.ClusterRole{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "openebs-lvm-role"}, gotCR); err != nil {
		t.Errorf("ClusterRole not created: %v", err)
	}

	gotCRB := &rbacv1.ClusterRoleBinding{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "openebs-lvm-binding"}, gotCRB); err != nil {
		t.Errorf("ClusterRoleBinding not created: %v", err)
	}
}

func TestDeployLVMIdempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}
	ctx := context.Background()

	status1 := d.deployLVM(ctx)
	if status1.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Fatalf("first deployLVM failed: %s", status1.Message)
	}

	status2 := d.deployLVM(ctx)
	if status2.Phase != storagev1alpha1.OpenEBSPhaseRunning {
		t.Fatalf("second deployLVM failed: %s", status2.Message)
	}
}

func TestDeployerApplyDeploymentUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dep-update",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "test", Image: "test:v1"}},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := &Deployer{Client: cl, Scheme: scheme}
	ctx := context.Background()

	if err := d.apply(ctx, dep.DeepCopy()); err != nil {
		t.Fatalf("first apply deployment failed: %v", err)
	}

	// Update replicas and apply again
	replicas2 := int32(3)
	updatedDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dep-update",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas2,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "test", Image: "test:v1"}},
				},
			},
		},
	}

	if err := d.apply(ctx, updatedDep); err != nil {
		t.Fatalf("second apply deployment failed: %v", err)
	}

	got := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "test-dep-update", Namespace: "default"}, got); err != nil {
		t.Fatalf("deployment not found: %v", err)
	}
	if *got.Spec.Replicas != 3 {
		t.Errorf("expected 3 replicas after update, got %d", *got.Spec.Replicas)
	}
}

func TestDeployerAllRBACForAllEngines(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name   string
		saFn   func() *corev1.ServiceAccount
		saName string
	}{
		{"LVM", lvmServiceAccount, "openebs-lvm-controller"},
		{"Hostpath", hostpathServiceAccount, "openebs-localpv-provisioner"},
		{"ZFS", zfsServiceAccount, "openebs-zfs-controller"},
		{"Rawfile", rawfileServiceAccount, "openebs-rawfile-provisioner"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(scheme).Build()
			d := &Deployer{Client: cl, Scheme: scheme}
			ctx := context.Background()

			sa := tt.saFn()
			if sa.Name != tt.saName {
				t.Errorf("expected SA name %s, got %s", tt.saName, sa.Name)
			}
			if sa.Namespace != openebsNamespace {
				t.Errorf("expected SA namespace %s, got %s", openebsNamespace, sa.Namespace)
			}

			if err := d.apply(ctx, sa); err != nil {
				t.Errorf("failed to apply SA: %v", err)
			}
		})
	}
}

func TestOwnerReferencesAreSet(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)

	instance := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs", UID: "test-uid"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}
	ctx := context.Background()

	dep := lvmControllerDeployment(instance)
	if err := d.applyDeployment(ctx, dep); err != nil {
		t.Fatalf("applyDeployment failed: %v", err)
	}

	var got appsv1.Deployment
	if err := cl.Get(ctx, types.NamespacedName{Name: dep.Name, Namespace: dep.Namespace}, &got); err != nil {
		t.Fatalf("Deployment not found: %v", err)
	}
	if len(got.OwnerReferences) != 1 {
		t.Fatalf("expected 1 owner ref, got %d", len(got.OwnerReferences))
	}
	ref := got.OwnerReferences[0]
	if ref.Kind != "OpenEBS" {
		t.Errorf("expected owner kind OpenEBS, got %s", ref.Kind)
	}
	if ref.Name != "openebs" {
		t.Errorf("expected owner name openebs, got %s", ref.Name)
	}
	if !*ref.Controller {
		t.Error("expected controller owner ref")
	}
	if !*ref.BlockOwnerDeletion {
		t.Error("expected blockOwnerDeletion")
	}
}

func TestCleanupOrphansDeletesUnexpectedResources(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)

	instance := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs"},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}

	managedLabel := map[string]string{"app.kubernetes.io/managed-by": "openebs-operator"}

	// Create an expected Deployment (LVM controller)
	expectedDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: lvmControllerName, Namespace: openebsNamespace, Labels: managedLabel},
	}
	// Create an orphan Deployment (something from a disabled engine)
	orphanDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "orphan-deployment", Namespace: openebsNamespace, Labels: managedLabel},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(expectedDep, orphanDep).
		Build()
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}
	ctx := context.Background()

	if err := d.cleanupOrphans(ctx); err != nil {
		t.Fatalf("cleanupOrphans failed: %v", err)
	}

	// Expected deployment should still exist
	var gotExpected appsv1.Deployment
	if err := cl.Get(ctx, types.NamespacedName{Name: lvmControllerName, Namespace: openebsNamespace}, &gotExpected); err != nil {
		t.Errorf("expected Deployment not found: %v", err)
	}

	// Orphan deployment should be gone
	var gotOrphan appsv1.Deployment
	if err := cl.Get(ctx, types.NamespacedName{Name: "orphan-deployment", Namespace: openebsNamespace}, &gotOrphan); err == nil {
		t.Error("orphan Deployment was not deleted")
	}
}

func TestCleanupOrphansPreservesUnmanagedResources(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)

	instance := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs"},
		Spec:       storagev1alpha1.OpenEBSSpec{},
	}

	// This resource has NO managed-by label — should NOT be deleted
	unmanagedDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "other-deployment", Namespace: openebsNamespace},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(unmanagedDep).
		Build()
	d := &Deployer{Client: cl, Scheme: scheme, instance: instance}
	ctx := context.Background()

	if err := d.cleanupOrphans(ctx); err != nil {
		t.Fatalf("cleanupOrphans failed: %v", err)
	}

	// Unmanaged deployment should still exist
	var got appsv1.Deployment
	if err := cl.Get(ctx, types.NamespacedName{Name: "other-deployment", Namespace: openebsNamespace}, &got); err != nil {
		t.Errorf("unmanaged Deployment was deleted: %v", err)
	}
}

func TestExpectedResourcesEmptyWhenNoEngines(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{},
	}
	resources := expectedResources(instance)
	if len(resources) != 0 {
		t.Errorf("expected 0 resources when no engines enabled, got %d", len(resources))
	}
}

func TestExpectedResourcesIncludesEnabledEngines(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM:      &storagev1alpha1.LVMConfig{Enabled: true},
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true},
		},
	}
	resources := expectedResources(instance)
	if !resources[lvmControllerName] {
		t.Error("expected LVM controller in resources")
	}
	if !resources[hostpathDeployName] {
		t.Error("expected hostpath deployment in resources")
	}
	if resources[zfsControllerName] {
		t.Error("ZFS controller should NOT be in resources when disabled")
	}
}
