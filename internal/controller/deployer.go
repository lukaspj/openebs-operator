package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	storagev1alpha1 "github.com/aldershaab-it/openebs-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
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

	mayastorServiceAccountName   = "mayastor-service-account"
	mayastorClusterRoleName      = "mayastor-role"
	mayastorClusterRoleBindingName = "mayastor-binding"
	mayastorEtcdName             = "mayastor-etcd"
	mayastorEtcdServiceName      = "mayastor-etcd"
	mayastorAgentCoreName        = "mayastor-agent-core"
	mayastorAgentCoreServiceName = "mayastor-agent-core"
	mayastorAPIRestName          = "mayastor-api-rest"
	mayastorAPIRestServiceName   = "mayastor-api-rest"
	mayastorCSIControllerName    = "mayastor-csi-controller"
	mayastorIOEngineName         = "mayastor-io-engine"
	mayastorCSINodeName          = "mayastor-csi-node"
	mayastorDiskpoolName         = "mayastor-operator-diskpool"
	mayastorHANodeName           = "mayastor-agent-ha-node"
	mayastorCSIDriverName        = "csi.nvmf.openebs.io"
	mayastorSCName               = "mayastor"
	mayastorSnapshotClassName    = "mayastor-snapshot"
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

	if err := d.applyVolumeSnapshotCRDs(ctx); err != nil {
		logger.Error(err, "failed to apply VolumeSnapshot CRDs")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.applyRBAC(ctx, mayastorServiceAccount(), mayastorClusterRole(), mayastorClusterRoleBinding()); err != nil {
		logger.Error(err, "failed to apply Mayastor RBAC")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.apply(ctx, mayastorEtcdService()); err != nil {
		logger.Error(err, "failed to apply etcd Service")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.etcdHealthCheck(ctx); err != nil {
		logger.Error(err, "etcd health check failed, skipping StatefulSet update")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.apply(ctx, mayastorEtcdStatefulSet(d.instance)); err != nil {
		logger.Error(err, "failed to apply etcd StatefulSet")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.applyDeployment(ctx, mayastorAgentCoreDeployment(d.instance)); err != nil {
		logger.Error(err, "failed to apply agent-core")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.apply(ctx, mayastorAgentCoreService()); err != nil {
		logger.Error(err, "failed to apply agent-core Service")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.apply(ctx, mayastorAPIRestService()); err != nil {
		logger.Error(err, "failed to apply api-rest Service")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.applyDeployment(ctx, mayastorAPIRestDeployment(d.instance)); err != nil {
		logger.Error(err, "failed to apply api-rest")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.applyDeployment(ctx, mayastorCSIControllerDeployment(d.instance)); err != nil {
		logger.Error(err, "failed to apply csi-controller")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.applyDaemonSet(ctx, mayastorIOEngineDaemonSet(d.instance)); err != nil {
		logger.Error(err, "failed to apply io-engine DaemonSet")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.applyDaemonSet(ctx, mayastorCSINodeDaemonSet(d.instance)); err != nil {
		logger.Error(err, "failed to apply csi-node DaemonSet")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.applyDaemonSet(ctx, mayastorHANodeDaemonSet(d.instance)); err != nil {
		logger.Error(err, "failed to apply ha-node DaemonSet")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.applyDeployment(ctx, mayastorOperatorDiskpoolDeployment(d.instance)); err != nil {
		logger.Error(err, "failed to apply operator-diskpool")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	if err := d.applyCSIDriver(ctx, mayastorCSIDriver()); err != nil {
		logger.Error(err, "failed to apply Mayastor CSIDriver")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	scName := mayastorSCName
	if d.instance.Spec.Mayastor.StorageClassName != "" {
		scName = d.instance.Spec.Mayastor.StorageClassName
	}
	if err := d.applyStorageClass(ctx, mayastorStorageClass(scName, d.instance)); err != nil {
		logger.Error(err, "failed to apply Mayastor StorageClass")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	snapName := mayastorSnapshotClassName
	if d.instance.Spec.Mayastor.SnapshotClassName != "" {
		snapName = d.instance.Spec.Mayastor.SnapshotClassName
	}
	if err := d.applyUnstructured(ctx, mayastorVolumeSnapshotClass(snapName)); err != nil {
		logger.Error(err, "failed to apply VolumeSnapshotClass")
		return d.engineFailed(storagev1alpha1.OpenEBSEngineMayastor, err)
	}

	return storagev1alpha1.EngineStatus{
		Name:    storagev1alpha1.OpenEBSEngineMayastor,
		Phase:   storagev1alpha1.OpenEBSPhaseRunning,
		Message: "Mayastor engine deployed",
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
		mayastorEtcdStatefulSet(d.instance),
		mayastorAgentCoreDeployment(d.instance),
		mayastorAgentCoreService(),
		mayastorAPIRestDeployment(d.instance),
		mayastorCSIControllerDeployment(d.instance),
		mayastorIOEngineDaemonSet(d.instance),
		mayastorCSINodeDaemonSet(d.instance),
		mayastorHANodeDaemonSet(d.instance),
		mayastorOperatorDiskpoolDeployment(d.instance),
		&storagev1.CSIDriver{ObjectMeta: metav1.ObjectMeta{Name: mayastorCSIDriverName}},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: mayastorSCName}},
		&unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]interface{}{"name": "volumesnapshotclasses.snapshot.storage.k8s.io"},
		}},
		&unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]interface{}{"name": "volumesnapshots.snapshot.storage.k8s.io"},
		}},
		&unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]interface{}{"name": "volumesnapshotcontents.snapshot.storage.k8s.io"},
		}},
		&unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "snapshot.storage.k8s.io/v1",
			"kind":       "VolumeSnapshotClass",
			"metadata":   map[string]interface{}{"name": mayastorSnapshotClassName},
		}},
	} {
		if err := d.Delete(ctx, obj); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// --- Helpers ---

func (d *Deployer) etcdHealthCheck(ctx context.Context) error {
	sts := &appsv1.StatefulSet{}
	err := d.Get(ctx, types.NamespacedName{Name: mayastorEtcdName, Namespace: mayastorNamespace}, sts)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("etcd statefulset lookup: %w", err)
	}
	if sts.Status.ReadyReplicas == 0 {
		return nil
	}
	hc := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://mayastor-etcd.mayastor:2379/health", nil)
	if err != nil {
		return fmt.Errorf("etcd health request: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("etcd not healthy: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("etcd health check returned status %d", resp.StatusCode)
	}
	return nil
}

func (d *Deployer) ensureNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: name,
		Labels: map[string]string{
			"pod-security.kubernetes.io/enforce": "privileged",
		},
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, d.Client, ns, func() error {
		if ns.Labels == nil {
			ns.Labels = map[string]string{}
		}
		ns.Labels["pod-security.kubernetes.io/enforce"] = "privileged"
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
	err := d.apply(ctx, sc)
	if err == nil || !errors.IsInvalid(err) {
		return err
	}
	key := client.ObjectKeyFromObject(sc)
	existing := &storagev1.StorageClass{}
	if err := d.Get(ctx, key, existing); err != nil {
		return err
	}
	if err := d.Delete(ctx, existing); err != nil {
		return err
	}
	sc.SetResourceVersion("")
	return d.Create(ctx, sc)
}

// apply creates or updates a Kubernetes resource. For CRDs it uses
// a direct Apply since controllerutil.CreateOrUpdate cannot handle
// CRD schema size due to annotation limits.
func (d *Deployer) apply(ctx context.Context, obj client.Object) error {
	if d.instance != nil {
		obj.SetOwnerReferences(ownerRefs(d.instance))
	}

	key := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
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
	})
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

func (d *Deployer) cleanupOrphans(ctx context.Context) error {
	logger := log.FromContext(ctx)
	managedLabel := "app.kubernetes.io/managed-by"
	expected := expectedResources(d.instance)

	var deps appsv1.DeploymentList
	if err := d.List(ctx, &deps, client.InNamespace(openebsNamespace), client.MatchingLabels{managedLabel: "openebs-operator"}); err != nil {
		return err
	}
	for _, dep := range deps.Items {
		if !expected[dep.Name] {
			logger.Info("deleting orphan Deployment", "name", dep.Name)
			if err := d.Delete(ctx, &dep); err != nil {
				return err
			}
		}
	}

	var dss appsv1.DaemonSetList
	if err := d.List(ctx, &dss, client.InNamespace(openebsNamespace), client.MatchingLabels{managedLabel: "openebs-operator"}); err != nil {
		return err
	}
	for _, ds := range dss.Items {
		if !expected[ds.Name] {
			logger.Info("deleting orphan DaemonSet", "name", ds.Name)
			if err := d.Delete(ctx, &ds); err != nil {
				return err
			}
		}
	}

	var sas corev1.ServiceAccountList
	if err := d.List(ctx, &sas, client.InNamespace(openebsNamespace), client.MatchingLabels{managedLabel: "openebs-operator"}); err != nil {
		return err
	}
	for _, sa := range sas.Items {
		if !expected[sa.Name] {
			logger.Info("deleting orphan ServiceAccount", "name", sa.Name)
			if err := d.Delete(ctx, &sa); err != nil {
				return err
			}
		}
	}

	var mayastorDeps appsv1.DeploymentList
	if err := d.List(ctx, &mayastorDeps, client.InNamespace(mayastorNamespace), client.MatchingLabels{managedLabel: "openebs-operator"}); err != nil {
		return err
	}
	for _, dep := range mayastorDeps.Items {
		if !expected[dep.Name] {
			logger.Info("deleting orphan mayastor Deployment", "name", dep.Name)
			if err := d.Delete(ctx, &dep); err != nil {
				return err
			}
		}
	}

	var mayastorDss appsv1.DaemonSetList
	if err := d.List(ctx, &mayastorDss, client.InNamespace(mayastorNamespace), client.MatchingLabels{managedLabel: "openebs-operator"}); err != nil {
		return err
	}
	for _, ds := range mayastorDss.Items {
		if !expected[ds.Name] {
			logger.Info("deleting orphan mayastor DaemonSet", "name", ds.Name)
			if err := d.Delete(ctx, &ds); err != nil {
				return err
			}
		}
	}

	var mayastorSts appsv1.StatefulSetList
	if err := d.List(ctx, &mayastorSts, client.InNamespace(mayastorNamespace), client.MatchingLabels{managedLabel: "openebs-operator"}); err != nil {
		return err
	}
	for _, sts := range mayastorSts.Items {
		if !expected[sts.Name] {
			logger.Info("deleting orphan mayastor StatefulSet", "name", sts.Name)
			if err := d.Delete(ctx, &sts); err != nil {
				return err
			}
		}
	}

	var mayastorSas corev1.ServiceAccountList
	if err := d.List(ctx, &mayastorSas, client.InNamespace(mayastorNamespace), client.MatchingLabels{managedLabel: "openebs-operator"}); err != nil {
		return err
	}
	for _, sa := range mayastorSas.Items {
		if !expected[sa.Name] {
			logger.Info("deleting orphan mayastor ServiceAccount", "name", sa.Name)
			if err := d.Delete(ctx, &sa); err != nil {
				return err
			}
		}
	}

	var crs rbacv1.ClusterRoleList
	if err := d.List(ctx, &crs, client.MatchingLabels{managedLabel: "openebs-operator"}); err != nil {
		return err
	}
	for _, cr := range crs.Items {
		if !expected[cr.Name] {
			logger.Info("deleting orphan ClusterRole", "name", cr.Name)
			if err := d.Delete(ctx, &cr); err != nil {
				return err
			}
		}
	}

	var crbs rbacv1.ClusterRoleBindingList
	if err := d.List(ctx, &crbs, client.MatchingLabels{managedLabel: "openebs-operator"}); err != nil {
		return err
	}
	for _, crb := range crbs.Items {
		if !expected[crb.Name] {
			logger.Info("deleting orphan ClusterRoleBinding", "name", crb.Name)
			if err := d.Delete(ctx, &crb); err != nil {
				return err
			}
		}
	}

	var cds storagev1.CSIDriverList
	if err := d.List(ctx, &cds, client.MatchingLabels{managedLabel: "openebs-operator"}); err != nil {
		return err
	}
	for _, cd := range cds.Items {
		if !expected[cd.Name] {
			logger.Info("deleting orphan CSIDriver", "name", cd.Name)
			if err := d.Delete(ctx, &cd); err != nil {
				return err
			}
		}
	}

	var scs storagev1.StorageClassList
	if err := d.List(ctx, &scs, client.MatchingLabels{managedLabel: "openebs-operator"}); err != nil {
		return err
	}
	for _, sc := range scs.Items {
		if !expected[sc.Name] {
			logger.Info("deleting orphan StorageClass", "name", sc.Name)
			if err := d.Delete(ctx, &sc); err != nil {
				return err
			}
		}
	}

	return nil
}

func expectedResources(instance *storagev1alpha1.OpenEBS) map[string]bool {
	resources := map[string]bool{}

	if instance.Spec.LVM != nil && instance.Spec.LVM.Enabled {
		resources[lvmControllerName] = true
		resources[lvmNodeName] = true
		resources[lvmCSIDriverName] = true
		resources[lvmSCName] = true
		resources["openebs-lvm-controller"] = true
		resources["openebs-lvm-role"] = true
		resources["openebs-lvm-binding"] = true
		if instance.Spec.LVM.StorageClassName != "" {
			resources[instance.Spec.LVM.StorageClassName] = true
		}
	}
	if instance.Spec.Hostpath != nil && instance.Spec.Hostpath.Enabled {
		resources[hostpathDeployName] = true
		resources[hostpathSCName] = true
		resources["openebs-localpv-provisioner"] = true
		if instance.Spec.Hostpath.StorageClassName != "" {
			resources[instance.Spec.Hostpath.StorageClassName] = true
		}
	}
	if instance.Spec.ZFS != nil && instance.Spec.ZFS.Enabled {
		resources[zfsControllerName] = true
		resources[zfsNodeName] = true
		resources[zfsCSIDriverName] = true
		resources[zfsSCName] = true
		resources["openebs-zfs-controller"] = true
		resources["openebs-zfs-role"] = true
		resources["openebs-zfs-binding"] = true
		if instance.Spec.ZFS.StorageClassName != "" {
			resources[instance.Spec.ZFS.StorageClassName] = true
		}
	}
	if instance.Spec.Rawfile != nil && instance.Spec.Rawfile.Enabled {
		resources[rawfileDeployName] = true
		resources[rawfileSCName] = true
		resources["openebs-rawfile-provisioner"] = true
		resources["openebs-rawfile-role"] = true
		resources["openebs-rawfile-binding"] = true
	}

	if instance.Spec.Mayastor != nil && instance.Spec.Mayastor.Enabled {
		resources[mayastorEtcdName] = true
		resources[mayastorAgentCoreName] = true
		resources[mayastorAPIRestName] = true
		resources[mayastorCSIControllerName] = true
		resources[mayastorIOEngineName] = true
		resources[mayastorCSINodeName] = true
		resources[mayastorHANodeName] = true
		resources[mayastorDiskpoolName] = true
		resources[mayastorServiceAccountName] = true
		resources[mayastorClusterRoleName] = true
		resources[mayastorClusterRoleBindingName] = true
		resources[mayastorCSIDriverName] = true
		resources[mayastorSCName] = true
		resources[mayastorSnapshotClassName] = true
		resources["volumesnapshotclasses.snapshot.storage.k8s.io"] = true
		resources["volumesnapshots.snapshot.storage.k8s.io"] = true
		resources["volumesnapshotcontents.snapshot.storage.k8s.io"] = true
		if instance.Spec.Mayastor.StorageClassName != "" {
			resources[instance.Spec.Mayastor.StorageClassName] = true
		}
		if instance.Spec.Mayastor.SnapshotClassName != "" {
			resources[instance.Spec.Mayastor.SnapshotClassName] = true
		}
	}

	return resources
}
