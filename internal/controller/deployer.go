package controller

import (
	"context"
	"fmt"

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Deployer handles applying Kubernetes resources for OpenEBS engines.
type Deployer struct {
	client.Client
	Scheme   *runtime.Scheme
	instance *storagev1alpha1.OpenEBS
}

// Engine labels and names.
const (
	openebsNamespace = "openebs"

	lvmControllerName = "openebs-lvm-controller"
	lvmNodeName       = "openebs-lvm-node"
	lvmCSIDriverName  = "local.csi.openebs.io"
	lvmSCName         = "openebs-lvm"

	hostpathDeployName = "openebs-localpv-provisioner"
	hostpathSCName     = "openebs-hostpath"

	zfsControllerName = "openebs-zfs-controller"
	zfsNodeName       = "openebs-zfs-node"
	zfsCSIDriverName  = "zfs.csi.openebs.io"
	zfsSCName         = "openebs-zfs"

	rawfileDeployName = "openebs-rawfile-provisioner"
	rawfileSCName     = "openebs-rawfile"

	mayastorNamespace  = "mayastor"
	mayastorLabelKey   = "openebs.io/engine"
	mayastorLabelValue = "mayastor"
)

func (d *Deployer) deployLVM(ctx context.Context) storagev1alpha1.EngineStatus {
	logger := log.FromContext(ctx)

	if err := d.ensureNamespace(ctx, openebsNamespace); err != nil {
		logger.Error(err, "failed to create openebs namespace")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineLVM, err)
	}

	if err := d.applyRBAC(ctx, lvmServiceAccount(), lvmClusterRole(), lvmClusterRoleBinding()); err != nil {
		logger.Error(err, "failed to apply LVM RBAC")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineLVM, err)
	}

	if err := d.applyDeployment(ctx, lvmControllerDeployment(d.instance)); err != nil {
		logger.Error(err, "failed to apply LVM controller")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineLVM, err)
	}

	if err := d.applyDaemonSet(ctx, lvmNodeDaemonSet(d.instance)); err != nil {
		logger.Error(err, "failed to apply LVM node daemonset")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineLVM, err)
	}

	if err := d.applyCSIDriver(ctx, lvmCSIDriver()); err != nil {
		logger.Error(err, "failed to apply LVM CSIDriver")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineLVM, err)
	}

	scName := lvmSCName
	if d.instance.Spec.LVM.StorageClassName != "" {
		scName = d.instance.Spec.LVM.StorageClassName
	}
	if err := d.applyStorageClass(ctx, lvmStorageClass(scName, d.instance.Spec.LVM)); err != nil {
		logger.Error(err, "failed to apply LVM StorageClass")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineLVM, err)
	}

	return storagev1alpha1.EngineStatus{
		Name:    storagev1alpha1.OpenEBSEngineLVM,
		Phase:   storagev1alpha1.OpenEBSPhaseRunning,
		Message: "LVM engine deployed",
	}
}

func (d *Deployer) deployHostpath(ctx context.Context) storagev1alpha1.EngineStatus {
	logger := log.FromContext(ctx)

	if err := d.ensureNamespace(ctx, openebsNamespace); err != nil {
		logger.Error(err, "failed to create openebs namespace")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineHostpath, err)
	}

	if err := d.applyRBAC(ctx, hostpathServiceAccount(), hostpathClusterRole(), hostpathClusterRoleBinding()); err != nil {
		logger.Error(err, "failed to apply hostpath RBAC")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineHostpath, err)
	}

	if err := d.applyDeployment(ctx, hostpathDeployment(d.instance)); err != nil {
		logger.Error(err, "failed to apply hostpath provisioner")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineHostpath, err)
	}

	scName := hostpathSCName
	if d.instance.Spec.Hostpath.StorageClassName != "" {
		scName = d.instance.Spec.Hostpath.StorageClassName
	}
	if err := d.applyStorageClass(ctx, hostpathStorageClass(scName, d.instance.Spec.Hostpath)); err != nil {
		logger.Error(err, "failed to apply hostpath StorageClass")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineHostpath, err)
	}

	return storagev1alpha1.EngineStatus{
		Name:    storagev1alpha1.OpenEBSEngineHostpath,
		Phase:   storagev1alpha1.OpenEBSPhaseRunning,
		Message: "Hostpath engine deployed",
	}
}

func (d *Deployer) deployZFS(ctx context.Context) storagev1alpha1.EngineStatus {
	logger := log.FromContext(ctx)

	if err := d.ensureNamespace(ctx, openebsNamespace); err != nil {
		logger.Error(err, "failed to create openebs namespace")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineZFS, err)
	}

	if err := d.applyRBAC(ctx, zfsServiceAccount(), zfsClusterRole(), zfsClusterRoleBinding()); err != nil {
		logger.Error(err, "failed to apply ZFS RBAC")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineZFS, err)
	}

	if err := d.applyDeployment(ctx, zfsControllerDeployment(d.instance)); err != nil {
		logger.Error(err, "failed to apply ZFS controller")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineZFS, err)
	}

	if err := d.applyDaemonSet(ctx, zfsNodeDaemonSet(d.instance)); err != nil {
		logger.Error(err, "failed to apply ZFS node daemonset")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineZFS, err)
	}

	if err := d.applyCSIDriver(ctx, zfsCSIDriver()); err != nil {
		logger.Error(err, "failed to apply ZFS CSIDriver")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineZFS, err)
	}

	scName := zfsSCName
	if d.instance.Spec.ZFS.StorageClassName != "" {
		scName = d.instance.Spec.ZFS.StorageClassName
	}
	if err := d.applyStorageClass(ctx, zfsStorageClass(scName, d.instance.Spec.ZFS)); err != nil {
		logger.Error(err, "failed to apply ZFS StorageClass")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineZFS, err)
	}

	return storagev1alpha1.EngineStatus{
		Name:    storagev1alpha1.OpenEBSEngineZFS,
		Phase:   storagev1alpha1.OpenEBSPhaseRunning,
		Message: "ZFS engine deployed",
	}
}

func (d *Deployer) deployRawfile(ctx context.Context) storagev1alpha1.EngineStatus {
	logger := log.FromContext(ctx)

	if err := d.ensureNamespace(ctx, openebsNamespace); err != nil {
		logger.Error(err, "failed to create openebs namespace")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineRawfile, err)
	}

	if err := d.applyRBAC(ctx, rawfileServiceAccount(), rawfileClusterRole(), rawfileClusterRoleBinding()); err != nil {
		logger.Error(err, "failed to apply rawfile RBAC")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineRawfile, err)
	}

	if err := d.applyDeployment(ctx, rawfileDeployment(d.instance)); err != nil {
		logger.Error(err, "failed to apply rawfile provisioner")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineRawfile, err)
	}

	scName := rawfileSCName
	if err := d.applyStorageClass(ctx, rawfileStorageClass(scName, d.instance.Spec.Rawfile)); err != nil {
		logger.Error(err, "failed to apply rawfile StorageClass")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineRawfile, err)
	}

	return storagev1alpha1.EngineStatus{
		Name:    storagev1alpha1.OpenEBSEngineRawfile,
		Phase:   storagev1alpha1.OpenEBSPhaseRunning,
		Message: "Rawfile engine deployed",
	}
}

func (d *Deployer) deployMayastor(ctx context.Context) storagev1alpha1.EngineStatus {
	logger := log.FromContext(ctx)

	if err := d.ensureNamespace(ctx, mayastorNamespace); err != nil {
		logger.Error(err, "failed to create mayastor namespace")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	// Mayastor deployment is complex. For now, report as installing
	// to indicate that Mayastor support is pending full implementation.
	logger.Info("Mayastor engine requested but full operator support is pending")
	return storagev1alpha1.EngineStatus{
		Name:    storagev1alpha1.OpenEBSEngineMayastor,
		Phase:   storagev1alpha1.OpenEBSPhaseInstalling,
		Message: "Mayastor operator support is pending; deploy via helm chart manually for now",
	}
}

func (d *Deployer) cleanup(ctx context.Context) error {
	for _, obj := range []client.Object{
		lvmControllerDeployment(d.instance),
		lvmNodeDaemonSet(d.instance),
		hostpathDeployment(d.instance),
		zfsControllerDeployment(d.instance),
		zfsNodeDaemonSet(d.instance),
		rawfileDeployment(d.instance),
		&storagev1.CSIDriver{ObjectMeta: metav1.ObjectMeta{Name: lvmCSIDriverName}},
		&storagev1.CSIDriver{ObjectMeta: metav1.ObjectMeta{Name: zfsCSIDriverName}},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: lvmSCName}},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: hostpathSCName}},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: zfsSCName}},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: rawfileSCName}},
	} {
		if err := d.Delete(ctx, obj); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// --- Helpers ---

func (d *Deployer) ensureNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if _, err := controllerutil.CreateOrUpdate(ctx, d.Client, ns, func() error {
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (d *Deployer) applyRBAC(ctx context.Context, sa *corev1.ServiceAccount, cr *rbacv1.ClusterRole, crb *rbacv1.ClusterRoleBinding) error {
	if err := d.apply(ctx, sa); err != nil {
		return fmt.Errorf("serviceaccount %s: %w", sa.Name, err)
	}
	if err := d.apply(ctx, cr); err != nil {
		return fmt.Errorf("clusterrole %s: %w", cr.Name, err)
	}
	if err := d.apply(ctx, crb); err != nil {
		return fmt.Errorf("clusterrolebinding %s: %w", crb.Name, err)
	}
	return nil
}

func (d *Deployer) applyDeployment(ctx context.Context, dep *appsv1.Deployment) error {
	return d.apply(ctx, dep)
}

func (d *Deployer) applyDaemonSet(ctx context.Context, ds *appsv1.DaemonSet) error {
	return d.apply(ctx, ds)
}

func (d *Deployer) applyCSIDriver(ctx context.Context, driver *storagev1.CSIDriver) error {
	return d.apply(ctx, driver)
}

func (d *Deployer) applyStorageClass(ctx context.Context, sc *storagev1.StorageClass) error {
	return d.apply(ctx, sc)
}

// apply creates or updates a Kubernetes resource. For CRDs it uses
// a direct Apply since controllerutil.CreateOrUpdate cannot handle
// CRD schema size due to annotation limits.
func (d *Deployer) apply(ctx context.Context, obj client.Object) error {
	key := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}

	existing := obj.DeepCopyObject().(client.Object)
	err := d.Get(ctx, key, existing)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return d.Create(ctx, obj)
	}

	obj.SetResourceVersion(existing.GetResourceVersion())
	if _, ok := obj.(*apiextensionsv1.CustomResourceDefinition); ok {
		existing.SetAnnotations(nil)
	}
	return d.Update(ctx, obj)
}

func (d *Deployer) engineFailed(engine storagev1alpha1.OpenEBSEngine, err error) storagev1alpha1.EngineStatus {
	return storagev1alpha1.EngineStatus{
		Name:    engine,
		Phase:   storagev1alpha1.OpenEBSPhaseFailed,
		Message: err.Error(),
	}
}

func labels(component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "openebs",
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/managed-by": "openebs-operator",
	}
}

func ownerRefs(instance *storagev1alpha1.OpenEBS) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		*metav1.NewControllerRef(instance, storagev1alpha1.GroupVersion.WithKind("OpenEBS")),
	}
}
