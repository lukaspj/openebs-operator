package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	storagev1alpha1 "github.com/aldershaab-it/openebs-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

var k8sClient client.Client

func TestMain(m *testing.M) {
	cfg, err := crconfig.GetConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to get kubeconfig: %v", err))
	}

	s := runtime.NewScheme()
	utilruntime.Must(scheme.AddToScheme(s))
	utilruntime.Must(storagev1alpha1.AddToScheme(s))
	utilruntime.Must(appsv1.AddToScheme(s))
	utilruntime.Must(storagev1.AddToScheme(s))
	utilruntime.Must(apiextensionsv1.AddToScheme(s))

	k8sClient, err = client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		panic(fmt.Sprintf("failed to create client: %v", err))
	}

	m.Run()
}

const (
	mayastorNamespace         = "mayastor"
	openebsOperatorNamespace  = "openebs-operator-system"
	mayastorCSIDriverName     = "csi.nvmf.openebs.io"
	defaultTimeout            = 90 * time.Second
	defaultInterval           = 2 * time.Second
	crName                    = "e2e-test"
)

func waitForCRReady(ctx context.Context, t *testing.T) {
	t.Helper()
	var cr storagev1alpha1.OpenEBS
	err := wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: crName}, &cr); err != nil {
			return false, nil
		}
		for _, e := range cr.Status.Engines {
			if e.Phase != "Running" {
				return false, nil
			}
		}
		return len(cr.Status.Engines) > 0, nil
	})
	if err != nil {
		t.Fatalf("CR did not reach Running: %v. Status: %+v", err, cr.Status)
	}
}

func waitForDeployment(ctx context.Context, t *testing.T, name, namespace string) {
	t.Helper()
	dep := &appsv1.Deployment{}
	err := wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, dep); err != nil {
			return false, nil
		}
		return dep.Status.ReadyReplicas >= 1, nil
	})
	if err != nil {
		t.Fatalf("deployment %s/%s not ready: %v", namespace, name, err)
	}
}

func waitForDaemonSet(ctx context.Context, t *testing.T, name, namespace string) {
	t.Helper()
	ds := &appsv1.DaemonSet{}
	err := wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, ds); err != nil {
			return false, nil
		}
		return ds.Status.NumberReady >= 1, nil
	})
	if err != nil {
		t.Fatalf("daemonset %s/%s not ready: %v", namespace, name, err)
	}
}

func waitForStatefulSet(ctx context.Context, t *testing.T, name, namespace string, replicas int32) {
	t.Helper()
	sts := &appsv1.StatefulSet{}
	err := wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, sts); err != nil {
			return false, nil
		}
		return sts.Status.ReadyReplicas >= replicas, nil
	})
	if err != nil {
		t.Fatalf("statefulset %s/%s not ready: %v", namespace, name, err)
	}
}

func resourceExists(ctx context.Context, t *testing.T, obj client.Object) bool {
	t.Helper()
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	return err == nil
}

func createCR(ctx context.Context, t *testing.T, cr *storagev1alpha1.OpenEBS) {
	t.Helper()
	existing := &storagev1alpha1.OpenEBS{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: cr.Name}, existing); err == nil {
		k8sClient.Delete(ctx, existing)
		wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
			return k8sClient.Get(ctx, client.ObjectKey{Name: cr.Name}, &storagev1alpha1.OpenEBS{}) != nil, nil
		})
	}
	if err := k8sClient.Create(ctx, cr); err != nil {
		t.Fatalf("create CR: %v", err)
	}
}

func updateCR(ctx context.Context, t *testing.T, name string, mutate func(*storagev1alpha1.OpenEBS)) {
	t.Helper()
	cr := &storagev1alpha1.OpenEBS{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name}, cr); err != nil {
		t.Fatalf("get CR: %v", err)
	}
	mutate(cr)
	if err := k8sClient.Update(ctx, cr); err != nil {
		t.Fatalf("update CR: %v", err)
	}
}

func deleteCR(ctx context.Context, t *testing.T, name string) {
	t.Helper()
	cr := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	if err := k8sClient.Delete(ctx, cr); err != nil {
		t.Fatalf("delete CR: %v", err)
	}
	err := wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name}, &storagev1alpha1.OpenEBS{}); err != nil {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("CR not deleted: %v", err)
	}
}

func gvk(g, v, k string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: g, Version: v, Kind: k}
}

func unstructuredObj(gvk schema.GroupVersionKind, name, namespace string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": gvk.GroupVersion().String(),
			"kind":       gvk.Kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
	return u
}

func waitForResource(ctx context.Context, t *testing.T, obj client.Object) {
	t.Helper()
	err := wait.PollUntilContextTimeout(ctx, defaultInterval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		return err == nil, nil
	})
	if err != nil {
		t.Fatalf("resource %s/%s not found: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err)
	}
}

func mustParseQuantity(s string) resource.Quantity {
	q, _ := resource.ParseQuantity(s)
	return q
}
