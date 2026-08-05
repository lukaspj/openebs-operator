package controller

import (
	"context"
	"fmt"
	"time"

	storagev1alpha1 "github.com/aldershaab-it/openebs-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	openebsFinalizer  = "storage.aldershaab-it.dk/finalizer"
	requeueAfter      = 30 * time.Second
	requeueAfterError = 10 * time.Second
)

// OpenEBSReconciler reconciles OpenEBS resources.
type OpenEBSReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=storage.aldershaab-it.dk,resources=openebs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.aldershaab-it.dk,resources=openebs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=storage.aldershaab-it.dk,resources=openebs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers;storageclasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch

func (r *OpenEBSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &storagev1alpha1.OpenEBS{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch OpenEBS")
		return ctrl.Result{}, err
	}

	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance)
	}

	if !controllerutil.ContainsFinalizer(instance, openebsFinalizer) {
		controllerutil.AddFinalizer(instance, openebsFinalizer)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	return r.reconcileNormal(ctx, instance)
}

func (r *OpenEBSReconciler) reconcileNormal(ctx context.Context, instance *storagev1alpha1.OpenEBS) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	changed := false
	engines, err := r.deployEngines(ctx, instance)
	if err != nil {
		instance.Status.Phase = storagev1alpha1.OpenEBSPhaseFailed
		changed = true
		logger.Error(err, "engine deployment failed")
	} else {
		instance.Status.Engines = engines
		instance.Status.Phase = r.derivePhase(engines)
		changed = true
	}

	instance.Status.Conditions = r.buildConditions(instance.Status.Phase, instance.Status.Engines)

	if changed {
		latest := &storagev1alpha1.OpenEBS{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(instance), latest); err != nil {
			logger.Error(err, "failed to re-fetch OpenEBS before status update")
			return ctrl.Result{}, err
		}
		latest.Status.Phase = instance.Status.Phase
		latest.Status.Engines = instance.Status.Engines
		latest.Status.Conditions = instance.Status.Conditions
		if err := r.Status().Update(ctx, latest); err != nil {
			logger.Error(err, "failed to update status")
			return ctrl.Result{}, err
		}
	}

	if instance.Status.Phase == storagev1alpha1.OpenEBSPhaseFailed {
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *OpenEBSReconciler) deployEngines(ctx context.Context, instance *storagev1alpha1.OpenEBS) ([]storagev1alpha1.EngineStatus, error) {
	d := &Deployer{Client: r.Client, Scheme: r.Scheme, instance: instance}

	var engines []storagev1alpha1.EngineStatus
	var errs []error

	if instance.Spec.LVM != nil && instance.Spec.LVM.Enabled {
		status := d.deployLVM(ctx)
		engines = append(engines, status)
		if status.Phase == storagev1alpha1.OpenEBSPhaseFailed {
			errs = append(errs, fmt.Errorf("lvm: %s", status.Message))
		}
	}

	if instance.Spec.ZFS != nil && instance.Spec.ZFS.Enabled {
		status := d.deployZFS(ctx)
		engines = append(engines, status)
		if status.Phase == storagev1alpha1.OpenEBSPhaseFailed {
			errs = append(errs, fmt.Errorf("zfs: %s", status.Message))
		}
	}

	if instance.Spec.Hostpath != nil && instance.Spec.Hostpath.Enabled {
		status := d.deployHostpath(ctx)
		engines = append(engines, status)
		if status.Phase == storagev1alpha1.OpenEBSPhaseFailed {
			errs = append(errs, fmt.Errorf("hostpath: %s", status.Message))
		}
	}

	if instance.Spec.Rawfile != nil && instance.Spec.Rawfile.Enabled {
		status := d.deployRawfile(ctx)
		engines = append(engines, status)
		if status.Phase == storagev1alpha1.OpenEBSPhaseFailed {
			errs = append(errs, fmt.Errorf("rawfile: %s", status.Message))
		}
	}

	if instance.Spec.Mayastor != nil && instance.Spec.Mayastor.Enabled {
		status := d.deployMayastor(ctx)
		engines = append(engines, status)
		if status.Phase == storagev1alpha1.OpenEBSPhaseFailed {
			errs = append(errs, fmt.Errorf("mayastor: %s", status.Message))
		}
	}

	if len(errs) > 0 {
		return engines, fmt.Errorf("engine deployment errors: %v", errs)
	}

	return engines, nil
}

func (r *OpenEBSReconciler) derivePhase(engines []storagev1alpha1.EngineStatus) storagev1alpha1.OpenEBSPhase {
	allRunning := true
	anyFailed := false

	for _, e := range engines {
		switch e.Phase {
		case storagev1alpha1.OpenEBSPhaseFailed:
			anyFailed = true
			allRunning = false
		case storagev1alpha1.OpenEBSPhaseDegraded:
			allRunning = false
		case storagev1alpha1.OpenEBSPhaseInstalling, storagev1alpha1.OpenEBSPhasePending:
			allRunning = false
		}
	}

	if len(engines) == 0 {
		return storagev1alpha1.OpenEBSPhasePending
	}
	if anyFailed {
		return storagev1alpha1.OpenEBSPhaseDegraded
	}
	if allRunning {
		return storagev1alpha1.OpenEBSPhaseRunning
	}
	return storagev1alpha1.OpenEBSPhaseInstalling
}

func (r *OpenEBSReconciler) buildConditions(phase storagev1alpha1.OpenEBSPhase, engines []storagev1alpha1.EngineStatus) []metav1.Condition {
	now := metav1.Now()

	available := metav1.ConditionFalse
	progressing := metav1.ConditionFalse
	degraded := metav1.ConditionFalse
	failed := metav1.ConditionFalse

	switch phase {
	case storagev1alpha1.OpenEBSPhaseRunning:
		available = metav1.ConditionTrue
	case storagev1alpha1.OpenEBSPhaseInstalling, storagev1alpha1.OpenEBSPhasePending:
		progressing = metav1.ConditionTrue
	case storagev1alpha1.OpenEBSPhaseDegraded:
		degraded = metav1.ConditionTrue
		available = metav1.ConditionTrue
	case storagev1alpha1.OpenEBSPhaseFailed:
		failed = metav1.ConditionTrue
	}

	return []metav1.Condition{
		{
			Type:               string(storagev1alpha1.ConditionAvailable),
			Status:             available,
			LastTransitionTime: now,
			Reason:             string(phase),
			Message:            fmt.Sprintf("OpenEBS is %s", phase),
		},
		{
			Type:               string(storagev1alpha1.ConditionProgressing),
			Status:             progressing,
			LastTransitionTime: now,
			Reason:             string(phase),
			Message:            fmt.Sprintf("OpenEBS is %s", phase),
		},
		{
			Type:               string(storagev1alpha1.ConditionDegraded),
			Status:             degraded,
			LastTransitionTime: now,
			Reason:             string(phase),
			Message:            "",
		},
		{
			Type:               string(storagev1alpha1.ConditionFailed),
			Status:             failed,
			LastTransitionTime: now,
			Reason:             string(phase),
			Message:            "",
		},
	}
}

func (r *OpenEBSReconciler) reconcileDelete(ctx context.Context, instance *storagev1alpha1.OpenEBS) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	d := &Deployer{Client: r.Client, Scheme: r.Scheme, instance: instance}

	if err := d.cleanup(ctx); err != nil {
		logger.Error(err, "cleanup failed")
		return ctrl.Result{RequeueAfter: requeueAfterError}, err
	}

	controllerutil.RemoveFinalizer(instance, openebsFinalizer)
	if err := r.Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenEBSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.OpenEBS{}).
		Complete(r)
}
