package controller

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	storagev1alpha1 "github.com/aldershaab-it/openebs-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func envMap(env []corev1.EnvVar) map[string]string {
	m := make(map[string]string, len(env))
	for _, e := range env {
		if e.Value != "" {
			m[e.Name] = e.Value
		} else if e.ValueFrom != nil {
			m[e.Name] = "<valueFrom>"
		}
	}
	return m
}

func TestLVMServiceAccount(t *testing.T) {
	sa := lvmServiceAccount()
	if sa.Name != "openebs-lvm-controller" {
		t.Errorf("expected name openebs-lvm-controller, got %s", sa.Name)
	}
	if sa.Namespace != openebsNamespace {
		t.Errorf("expected namespace %s, got %s", openebsNamespace, sa.Namespace)
	}
}

func TestLVMClusterRole(t *testing.T) {
	cr := lvmClusterRole()
	if cr.Name != "openebs-lvm-role" {
		t.Errorf("expected name openebs-lvm-role, got %s", cr.Name)
	}
	if len(cr.Rules) < 4 {
		t.Errorf("expected at least 4 rules, got %d", len(cr.Rules))
	}
}

func TestLVMClusterRoleBinding(t *testing.T) {
	crb := lvmClusterRoleBinding()
	if crb.Name != "openebs-lvm-binding" {
		t.Errorf("expected name openebs-lvm-binding, got %s", crb.Name)
	}
	if crb.RoleRef.Name != "openebs-lvm-role" {
		t.Errorf("expected role ref openebs-lvm-role, got %s", crb.RoleRef.Name)
	}
	if len(crb.Subjects) != 1 {
		t.Errorf("expected 1 subject, got %d", len(crb.Subjects))
	}
}

func TestLVMControllerDeployment(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	dep := lvmControllerDeployment(instance)

	if dep.Name != lvmControllerName {
		t.Errorf("expected name %s, got %s", lvmControllerName, dep.Name)
	}
	if dep.Namespace != openebsNamespace {
		t.Errorf("expected namespace %s, got %s", openebsNamespace, dep.Namespace)
	}
	if *dep.Spec.Replicas != 1 {
		t.Errorf("expected 1 replica, got %d", *dep.Spec.Replicas)
	}
	if dep.Spec.Template.Spec.ServiceAccountName != "openebs-lvm-controller" {
		t.Errorf("expected SA openebs-lvm-controller, got %s", dep.Spec.Template.Spec.ServiceAccountName)
	}
	if len(dep.Spec.Template.Spec.Containers) != 4 {
		t.Errorf("expected 4 containers, got %d", len(dep.Spec.Template.Spec.Containers))
	}

	names := []string{}
	for _, c := range dep.Spec.Template.Spec.Containers {
		names = append(names, c.Name)
	}
	expected := []string{"csi-provisioner", "csi-resizer", "csi-snapshotter", "lvm-plugin"}
	for _, e := range expected {
		found := false
		for _, n := range names {
			if n == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected container %s not found in %v", e, names)
		}
	}
}

func TestLVMNodeDaemonSet(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}
	ds := lvmNodeDaemonSet(instance)

	if ds.Name != lvmNodeName {
		t.Errorf("expected name %s, got %s", lvmNodeName, ds.Name)
	}
	if !ds.Spec.Template.Spec.HostNetwork {
		t.Error("expected hostNetwork to be true")
	}
	if len(ds.Spec.Template.Spec.Containers) != 2 {
		t.Errorf("expected 2 containers, got %d", len(ds.Spec.Template.Spec.Containers))
	}

	lvmPlugin := ds.Spec.Template.Spec.Containers[1]
	if lvmPlugin.SecurityContext.Privileged == nil || !*lvmPlugin.SecurityContext.Privileged {
		t.Error("expected LVM node plugin to be privileged")
	}
}

func TestLVMCSIDriver(t *testing.T) {
	driver := lvmCSIDriver()
	if driver.Name != lvmCSIDriverName {
		t.Errorf("expected name %s, got %s", lvmCSIDriverName, driver.Name)
	}
	if driver.Spec.AttachRequired == nil || !*driver.Spec.AttachRequired {
		t.Error("expected AttachRequired to be true")
	}
}

func TestLVMStorageClass(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		cfg := &storagev1alpha1.LVMConfig{Enabled: true}
		sc := lvmStorageClass("openebs-lvm", cfg)

		if sc.Name != "openebs-lvm" {
			t.Errorf("expected name openebs-lvm, got %s", sc.Name)
		}
		if sc.Provisioner != lvmCSIDriverName {
			t.Errorf("expected provisioner %s, got %s", lvmCSIDriverName, sc.Provisioner)
		}
		if sc.Parameters["volgroup"] != "lvmvg" {
			t.Errorf("expected default vg lvmvg, got %s", sc.Parameters["volgroup"])
		}
		if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
			t.Error("AllowVolumeExpansion should be true")
		}
	})

	t.Run("custom volume group", func(t *testing.T) {
		cfg := &storagev1alpha1.LVMConfig{Enabled: true, VolumeGroup: "myvg"}
		sc := lvmStorageClass("openebs-lvm", cfg)

		if sc.Parameters["volgroup"] != "myvg" {
			t.Errorf("expected vg myvg, got %s", sc.Parameters["volgroup"])
		}
	})

	t.Run("default class annotation", func(t *testing.T) {
		cfg := &storagev1alpha1.LVMConfig{Enabled: true, IsDefaultClass: true}
		sc := lvmStorageClass("openebs-lvm", cfg)

		if sc.Annotations["storageclass.kubernetes.io/is-default-class"] != "true" {
			t.Error("expected default class annotation to be true")
		}
	})
}

func TestHostpathDeployment(t *testing.T) {
	t.Run("default base path", func(t *testing.T) {
		instance := &storagev1alpha1.OpenEBS{
			Spec: storagev1alpha1.OpenEBSSpec{
				Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true},
			},
		}
		dep := hostpathDeployment(instance)

		if dep.Name != hostpathDeployName {
			t.Errorf("expected name %s, got %s", hostpathDeployName, dep.Name)
		}
		if len(dep.Spec.Template.Spec.Containers) != 1 {
			t.Errorf("expected 1 container, got %d", len(dep.Spec.Template.Spec.Containers))
		}

		hasBasePath := false
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if env.Name == "OPENEBS_IO_BASE_PATH" && env.Value == "/var/openebs/local" {
				hasBasePath = true
			}
		}
		if !hasBasePath {
			t.Error("expected OPENEBS_IO_BASE_PATH=/var/openebs/local")
		}
	})

	t.Run("custom base path", func(t *testing.T) {
		instance := &storagev1alpha1.OpenEBS{
			Spec: storagev1alpha1.OpenEBSSpec{
				Hostpath: &storagev1alpha1.HostpathConfig{
					Enabled:  true,
					BasePath: "/mnt/storage",
				},
			},
		}
		dep := hostpathDeployment(instance)

		hasBasePath := false
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if env.Name == "OPENEBS_IO_BASE_PATH" && env.Value == "/mnt/storage" {
				hasBasePath = true
			}
		}
		if !hasBasePath {
			t.Error("expected OPENEBS_IO_BASE_PATH=/mnt/storage")
		}
	})
}

func TestHostpathStorageClass(t *testing.T) {
	cfg := &storagev1alpha1.HostpathConfig{Enabled: true, BasePath: "/data"}
	sc := hostpathStorageClass("openebs-hostpath", cfg)

	if sc.Provisioner != "openebs.io/local" {
		t.Errorf("expected provisioner openebs.io/local, got %s", sc.Provisioner)
	}
	if sc.Parameters["BasePath"] != "/data" {
		t.Errorf("expected BasePath /data, got %s", sc.Parameters["BasePath"])
	}
	if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
		t.Error("AllowVolumeExpansion should be true")
	}
	if sc.Annotations["openebs.io/cas-type"] != "local" {
		t.Errorf("expected annotation openebs.io/cas-type=local, got %s", sc.Annotations["openebs.io/cas-type"])
	}
	if casCfg := sc.Annotations["cas.openebs.io/config"]; !strings.Contains(casCfg, "hostpath") || !strings.Contains(casCfg, "/data") {
		t.Errorf("expected cas.openebs.io/config containing hostpath and /data, got %s", casCfg)
	}
}

func TestZFSDeployment(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			ZFS: &storagev1alpha1.ZFSConfig{Enabled: true},
		},
	}
	dep := zfsControllerDeployment(instance)

	if dep.Name != zfsControllerName {
		t.Errorf("expected name %s, got %s", zfsControllerName, dep.Name)
	}
	if len(dep.Spec.Template.Spec.Containers) != 3 {
		t.Errorf("expected 3 containers, got %d", len(dep.Spec.Template.Spec.Containers))
	}
	plugin := dep.Spec.Template.Spec.Containers[2]
	if plugin.Name != "zfs-plugin" {
		t.Fatalf("expected zfs-plugin container, got %s", plugin.Name)
	}
	expectedArgs := []string{"--endpoint=$(OPENEBS_CSI_ENDPOINT)", "--plugin=$(OPENEBS_CONTROLLER_DRIVER)"}
	if !slices.Equal(plugin.Args, expectedArgs) {
		t.Errorf("expected args %v, got %v", expectedArgs, plugin.Args)
	}
	env := envMap(plugin.Env)
	if env["OPENEBS_CONTROLLER_DRIVER"] != "controller" {
		t.Errorf("expected OPENEBS_CONTROLLER_DRIVER=controller, got %q", env["OPENEBS_CONTROLLER_DRIVER"])
	}
	if env["OPENEBS_CSI_ENDPOINT"] != "unix:///var/lib/csi/sockets/pluginproxy/csi.sock" {
		t.Errorf("expected controller CSI endpoint, got %q", env["OPENEBS_CSI_ENDPOINT"])
	}
	if _, ok := env["OPENEBS_NODE_ID"]; ok {
		t.Error("OPENEBS_NODE_ID must not be set on controller plugin")
	}
}

func TestZFSNodeDaemonSet(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			ZFS: &storagev1alpha1.ZFSConfig{Enabled: true},
		},
	}
	ds := zfsNodeDaemonSet(instance)

	if ds.Name != zfsNodeName {
		t.Errorf("expected name %s, got %s", zfsNodeName, ds.Name)
	}
	if len(ds.Spec.Template.Spec.Containers) != 2 {
		t.Errorf("expected 2 containers, got %d", len(ds.Spec.Template.Spec.Containers))
	}
	plugin := ds.Spec.Template.Spec.Containers[1]
	if plugin.Name != "zfs-node-plugin" {
		t.Fatalf("expected zfs-node-plugin container, got %s", plugin.Name)
	}
	expectedArgs := []string{"--nodename=$(OPENEBS_NODE_NAME)", "--endpoint=$(OPENEBS_CSI_ENDPOINT)", "--plugin=$(OPENEBS_NODE_DRIVER)"}
	if !slices.Equal(plugin.Args, expectedArgs) {
		t.Errorf("expected args %v, got %v", expectedArgs, plugin.Args)
	}
	env := envMap(plugin.Env)
	if env["OPENEBS_NODE_DRIVER"] != "agent" {
		t.Errorf("expected OPENEBS_NODE_DRIVER=agent, got %q", env["OPENEBS_NODE_DRIVER"])
	}
	if env["OPENEBS_CSI_ENDPOINT"] != "unix:///plugin/csi.sock" {
		t.Errorf("expected node CSI endpoint, got %q", env["OPENEBS_CSI_ENDPOINT"])
	}
	nodeName := env["OPENEBS_NODE_NAME"]
	if nodeName == "" {
		t.Error("OPENEBS_NODE_NAME must be set on node plugin")
	}
	if _, ok := env["OPENEBS_NODE_ID"]; ok {
		t.Error("OPENEBS_NODE_ID must not be set on node plugin")
	}
}

func TestRawfileDeployment(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Rawfile: &storagev1alpha1.RawfileConfig{Enabled: true},
		},
	}
	dep := rawfileDeployment(instance)

	if dep.Name != rawfileDeployName {
		t.Errorf("expected name %s, got %s", rawfileDeployName, dep.Name)
	}
}

func TestLabels(t *testing.T) {
	l := labels("test-component")
	expected := map[string]string{
		"app.kubernetes.io/name":       "openebs",
		"app.kubernetes.io/component":  "test-component",
		"app.kubernetes.io/managed-by": "openebs-operator",
	}
	for k, v := range expected {
		if l[k] != v {
			t.Errorf("label %s: expected %s, got %s", k, v, l[k])
		}
	}
}

func TestDerivePhase(t *testing.T) {
	r := &OpenEBSReconciler{}

	tests := []struct {
		name    string
		engines []storagev1alpha1.EngineStatus
		want    storagev1alpha1.OpenEBSPhase
	}{
		{
			name:    "empty engines = Pending",
			engines: nil,
			want:    storagev1alpha1.OpenEBSPhasePending,
		},
		{
			name: "all Running = Running",
			engines: []storagev1alpha1.EngineStatus{
				{Name: "lvm", Phase: storagev1alpha1.OpenEBSPhaseRunning},
				{Name: "hostpath", Phase: storagev1alpha1.OpenEBSPhaseRunning},
			},
			want: storagev1alpha1.OpenEBSPhaseRunning,
		},
		{
			name: "one Installing = Installing",
			engines: []storagev1alpha1.EngineStatus{
				{Name: "lvm", Phase: storagev1alpha1.OpenEBSPhaseRunning},
				{Name: "hostpath", Phase: storagev1alpha1.OpenEBSPhaseInstalling},
			},
			want: storagev1alpha1.OpenEBSPhaseInstalling,
		},
		{
			name: "one Failed = Degraded",
			engines: []storagev1alpha1.EngineStatus{
				{Name: "lvm", Phase: storagev1alpha1.OpenEBSPhaseRunning},
				{Name: "hostpath", Phase: storagev1alpha1.OpenEBSPhaseFailed},
			},
			want: storagev1alpha1.OpenEBSPhaseDegraded,
		},
		{
			name: "one Degraded = Installing (not all Running)",
			engines: []storagev1alpha1.EngineStatus{
				{Name: "lvm", Phase: storagev1alpha1.OpenEBSPhaseRunning},
				{Name: "hostpath", Phase: storagev1alpha1.OpenEBSPhaseDegraded},
			},
			want: storagev1alpha1.OpenEBSPhaseInstalling,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.derivePhase(tt.engines)
			if got != tt.want {
				t.Errorf("derivePhase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildConditions(t *testing.T) {
	r := &OpenEBSReconciler{}

	conds := r.buildConditions(storagev1alpha1.OpenEBSPhaseRunning, nil)
	if len(conds) != 4 {
		t.Errorf("expected 4 conditions, got %d", len(conds))
	}

	types := map[string]metav1.ConditionStatus{}
	for _, c := range conds {
		types[c.Type] = c.Status
	}

	if types["Available"] != metav1.ConditionTrue {
		t.Errorf("expected Available=True for Running phase")
	}
	if types["Failed"] != metav1.ConditionFalse {
		t.Errorf("expected Failed=False for Running phase")
	}
}

func TestBoolToStr(t *testing.T) {
	if boolToStr(true) != "true" {
		t.Error("expected 'true'")
	}
	if boolToStr(false) != "false" {
		t.Error("expected 'false'")
	}
}

// Utility assertions for test output
func assertDeployment(t *testing.T, dep *appsv1.Deployment, name, ns, sa string, replicas int32, containers int) {
	t.Helper()
	if dep.Name != name {
		t.Errorf("name: want %s, got %s", name, dep.Name)
	}
	if dep.Namespace != ns {
		t.Errorf("namespace: want %s, got %s", ns, dep.Namespace)
	}
	if dep.Spec.Template.Spec.ServiceAccountName != sa {
		t.Errorf("SA: want %s, got %s", sa, dep.Spec.Template.Spec.ServiceAccountName)
	}
	if r := *dep.Spec.Replicas; r != replicas {
		t.Errorf("replicas: want %d, got %d", replicas, r)
	}
	if c := len(dep.Spec.Template.Spec.Containers); c != containers {
		t.Errorf("containers: want %d, got %d", containers, c)
	}
}

func assertDaemonSet(t *testing.T, ds *appsv1.DaemonSet, name, ns string, hostNetwork bool) {
	t.Helper()
	if ds.Name != name {
		t.Errorf("name: want %s, got %s", name, ds.Name)
	}
	if ds.Namespace != ns {
		t.Errorf("namespace: want %s, got %s", ns, ds.Namespace)
	}
	if ds.Spec.Template.Spec.HostNetwork != hostNetwork {
		t.Errorf("hostNetwork: want %v, got %v", hostNetwork, ds.Spec.Template.Spec.HostNetwork)
	}
}

func assertCSIDriver(t *testing.T, driver *storagev1.CSIDriver, name string, attachRequired bool) {
	t.Helper()
	if driver.Name != name {
		t.Errorf("name: want %s, got %s", name, driver.Name)
	}
	if r := *driver.Spec.AttachRequired; r != attachRequired {
		t.Errorf("attachRequired: want %v, got %v", attachRequired, r)
	}
}

func assertStorageClass(t *testing.T, sc *storagev1.StorageClass, name, provisioner string) {
	t.Helper()
	if sc.Name != name {
		t.Errorf("name: want %s, got %s", name, sc.Name)
	}
	if sc.Provisioner != provisioner {
		t.Errorf("provisioner: want %s, got %s", provisioner, sc.Provisioner)
	}
	if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
		t.Errorf("%s: AllowVolumeExpansion must be true", name)
	}
}

func assertClusterRole(t *testing.T, cr *rbacv1.ClusterRole, name string, ruleCount int) {
	t.Helper()
	if cr.Name != name {
		t.Errorf("name: want %s, got %s", name, cr.Name)
	}
	if len(cr.Rules) != ruleCount {
		t.Errorf("rules: want %d, got %d", ruleCount, len(cr.Rules))
	}
}

// Validate all engine resource constructors produce well-formed objects.
func TestAllResourceConstructors(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM:      &storagev1alpha1.LVMConfig{Enabled: true},
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true},
			ZFS:      &storagev1alpha1.ZFSConfig{Enabled: true},
			Rawfile:  &storagev1alpha1.RawfileConfig{Enabled: true},
		},
	}

	t.Run("LVM resources", func(t *testing.T) {
		assertDeployment(t, lvmControllerDeployment(instance), lvmControllerName, openebsNamespace, "openebs-lvm-controller", 1, 4)
		assertDaemonSet(t, lvmNodeDaemonSet(instance), lvmNodeName, openebsNamespace, true)
		assertCSIDriver(t, lvmCSIDriver(), lvmCSIDriverName, true)
		assertStorageClass(t, lvmStorageClass("openebs-lvm", instance.Spec.LVM), "openebs-lvm", lvmCSIDriverName)
	})

	t.Run("Hostpath resources", func(t *testing.T) {
		assertDeployment(t, hostpathDeployment(instance), hostpathDeployName, openebsNamespace, "openebs-localpv-provisioner", 1, 1)
		assertStorageClass(t, hostpathStorageClass("openebs-hostpath", instance.Spec.Hostpath), "openebs-hostpath", "openebs.io/local")
	})

	t.Run("ZFS resources", func(t *testing.T) {
		assertDeployment(t, zfsControllerDeployment(instance), zfsControllerName, openebsNamespace, "openebs-zfs-controller", 1, 3)
		assertDaemonSet(t, zfsNodeDaemonSet(instance), zfsNodeName, openebsNamespace, true)
		assertCSIDriver(t, zfsCSIDriver(), zfsCSIDriverName, true)
		assertStorageClass(t, zfsStorageClass("openebs-zfs", instance.Spec.ZFS), "openebs-zfs", zfsCSIDriverName)
	})

	t.Run("Rawfile resources", func(t *testing.T) {
		assertDeployment(t, rawfileDeployment(instance), rawfileDeployName, openebsNamespace, "openebs-rawfile-provisioner", 1, 1)
		assertStorageClass(t, rawfileStorageClass("openebs-rawfile", instance.Spec.Rawfile), "openebs-rawfile", "rawfile.csi.openebs.io")
	})
}

func TestEngineFailed(t *testing.T) {
	d := &Deployer{}
	status := d.engineFailed(storagev1alpha1.OpenEBSEngineLVM, fmt.Errorf("test error"))
	if status.Name != storagev1alpha1.OpenEBSEngineLVM {
		t.Errorf("expected name lvm, got %s", status.Name)
	}
	if status.Phase != storagev1alpha1.OpenEBSPhaseFailed {
		t.Errorf("expected phase Failed, got %s", status.Phase)
	}
}

func TestHostpathServiceAccount(t *testing.T) {
	sa := hostpathServiceAccount()
	if sa.Name != "openebs-localpv-provisioner" {
		t.Errorf("expected name openebs-localpv-provisioner, got %s", sa.Name)
	}
	if sa.Namespace != openebsNamespace {
		t.Errorf("expected namespace %s, got %s", openebsNamespace, sa.Namespace)
	}
}

func TestHostpathClusterRole(t *testing.T) {
	cr := hostpathClusterRole()
	if cr.Name != "openebs-localpv-provisioner" {
		t.Errorf("expected name openebs-localpv-provisioner, got %s", cr.Name)
	}
	if len(cr.Rules) < 3 {
		t.Errorf("expected at least 3 rules, got %d", len(cr.Rules))
	}
}

func TestHostpathClusterRoleBinding(t *testing.T) {
	crb := hostpathClusterRoleBinding()
	if crb.RoleRef.Name != "openebs-localpv-provisioner" {
		t.Errorf("expected role ref openebs-localpv-provisioner, got %s", crb.RoleRef.Name)
	}
	if len(crb.Subjects) != 1 {
		t.Errorf("expected 1 subject, got %d", len(crb.Subjects))
	}
}

func TestZFSServiceAccount(t *testing.T) {
	sa := zfsServiceAccount()
	if sa.Name != "openebs-zfs-controller" {
		t.Errorf("expected name openebs-zfs-controller, got %s", sa.Name)
	}
	if sa.Namespace != openebsNamespace {
		t.Errorf("expected namespace %s, got %s", openebsNamespace, sa.Namespace)
	}
}

func TestZFSClusterRole(t *testing.T) {
	cr := zfsClusterRole()
	if cr.Name != "openebs-zfs-role" {
		t.Errorf("expected name openebs-zfs-role, got %s", cr.Name)
	}
	if len(cr.Rules) < 3 {
		t.Errorf("expected at least 3 rules, got %d", len(cr.Rules))
	}
}

func TestZFSClusterRoleBinding(t *testing.T) {
	crb := zfsClusterRoleBinding()
	if crb.RoleRef.Name != "openebs-zfs-role" {
		t.Errorf("expected role ref openebs-zfs-role, got %s", crb.RoleRef.Name)
	}
}

func TestRawfileServiceAccount(t *testing.T) {
	sa := rawfileServiceAccount()
	if sa.Name != "openebs-rawfile-provisioner" {
		t.Errorf("expected name openebs-rawfile-provisioner, got %s", sa.Name)
	}
}

func TestRawfileClusterRole(t *testing.T) {
	cr := rawfileClusterRole()
	if cr.Name != "openebs-rawfile-provisioner" {
		t.Errorf("expected name openebs-rawfile-provisioner, got %s", cr.Name)
	}
	if len(cr.Rules) < 4 {
		t.Errorf("expected at least 4 rules, got %d", len(cr.Rules))
	}
}

func TestZFSStorageClass(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		cfg := &storagev1alpha1.ZFSConfig{Enabled: true}
		sc := zfsStorageClass("openebs-zfs", cfg)

		if sc.Provisioner != zfsCSIDriverName {
			t.Errorf("expected provisioner %s, got %s", zfsCSIDriverName, sc.Provisioner)
		}
		if sc.Parameters["poolname"] != "zfspool" {
			t.Errorf("expected default pool zfspool, got %s", sc.Parameters["poolname"])
		}
		if sc.Parameters["fstype"] != "zfs" {
			t.Errorf("expected fstype zfs, got %s", sc.Parameters["fstype"])
		}
		if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
			t.Error("AllowVolumeExpansion should be true")
		}
	})

	t.Run("custom pool name", func(t *testing.T) {
		cfg := &storagev1alpha1.ZFSConfig{Enabled: true, PoolName: "tank"}
		sc := zfsStorageClass("openebs-zfs", cfg)

		if sc.Parameters["poolname"] != "tank" {
			t.Errorf("expected poolname tank, got %s", sc.Parameters["poolname"])
		}
	})
}

func TestRawfileStorageClass(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		cfg := &storagev1alpha1.RawfileConfig{Enabled: true}
		sc := rawfileStorageClass("openebs-rawfile", cfg)

		if sc.Provisioner != "rawfile.csi.openebs.io" {
			t.Errorf("expected provisioner rawfile.csi.openebs.io, got %s", sc.Provisioner)
		}
		if sc.Parameters["basePath"] != "/var/openebs/rawfile" {
			t.Errorf("expected default basePath /var/openebs/rawfile, got %s", sc.Parameters["basePath"])
		}
		if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
			t.Error("AllowVolumeExpansion should be true")
		}
	})

	t.Run("custom base path", func(t *testing.T) {
		cfg := &storagev1alpha1.RawfileConfig{Enabled: true, BasePath: "/mnt/rawfile"}
		sc := rawfileStorageClass("openebs-rawfile", cfg)

		if sc.Parameters["basePath"] != "/mnt/rawfile" {
			t.Errorf("expected basePath /mnt/rawfile, got %s", sc.Parameters["basePath"])
		}
	})
}

func TestBuildConditionsAllPhases(t *testing.T) {
	r := &OpenEBSReconciler{}

	tests := []struct {
		phase             storagev1alpha1.OpenEBSPhase
		expectAvailable   metav1.ConditionStatus
		expectProgressing metav1.ConditionStatus
		expectDegraded    metav1.ConditionStatus
		expectFailed      metav1.ConditionStatus
	}{
		{
			phase:             storagev1alpha1.OpenEBSPhasePending,
			expectAvailable:   metav1.ConditionFalse,
			expectProgressing: metav1.ConditionTrue,
			expectDegraded:    metav1.ConditionFalse,
			expectFailed:      metav1.ConditionFalse,
		},
		{
			phase:             storagev1alpha1.OpenEBSPhaseInstalling,
			expectAvailable:   metav1.ConditionFalse,
			expectProgressing: metav1.ConditionTrue,
			expectDegraded:    metav1.ConditionFalse,
			expectFailed:      metav1.ConditionFalse,
		},
		{
			phase:             storagev1alpha1.OpenEBSPhaseRunning,
			expectAvailable:   metav1.ConditionTrue,
			expectProgressing: metav1.ConditionFalse,
			expectDegraded:    metav1.ConditionFalse,
			expectFailed:      metav1.ConditionFalse,
		},
		{
			phase:             storagev1alpha1.OpenEBSPhaseDegraded,
			expectAvailable:   metav1.ConditionTrue,
			expectProgressing: metav1.ConditionFalse,
			expectDegraded:    metav1.ConditionTrue,
			expectFailed:      metav1.ConditionFalse,
		},
		{
			phase:             storagev1alpha1.OpenEBSPhaseFailed,
			expectAvailable:   metav1.ConditionFalse,
			expectProgressing: metav1.ConditionFalse,
			expectDegraded:    metav1.ConditionFalse,
			expectFailed:      metav1.ConditionTrue,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			conds := r.buildConditions(tt.phase, nil)
			for _, c := range conds {
				switch c.Type {
				case string(storagev1alpha1.ConditionAvailable):
					if c.Status != tt.expectAvailable {
						t.Errorf("Available: want %v, got %v", tt.expectAvailable, c.Status)
					}
				case string(storagev1alpha1.ConditionProgressing):
					if c.Status != tt.expectProgressing {
						t.Errorf("Progressing: want %v, got %v", tt.expectProgressing, c.Status)
					}
				case string(storagev1alpha1.ConditionDegraded):
					if c.Status != tt.expectDegraded {
						t.Errorf("Degraded: want %v, got %v", tt.expectDegraded, c.Status)
					}
				case string(storagev1alpha1.ConditionFailed):
					if c.Status != tt.expectFailed {
						t.Errorf("Failed: want %v, got %v", tt.expectFailed, c.Status)
					}
				}
			}
		})
	}
}

func TestOwnerRefs(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-openebs",
		},
	}
	refs := ownerRefs(instance)
	if len(refs) != 1 {
		t.Fatalf("expected 1 owner ref, got %d", len(refs))
	}
	if !*refs[0].Controller {
		t.Error("expected owner ref to be controller")
	}
}

func TestLVMStorageClassDefaultAnnotationFalse(t *testing.T) {
	cfg := &storagev1alpha1.LVMConfig{Enabled: true, IsDefaultClass: false}
	sc := lvmStorageClass("openebs-lvm", cfg)

	if sc.Annotations["storageclass.kubernetes.io/is-default-class"] != "false" {
		t.Error("expected default class annotation to be false")
	}
}

func TestMayastorConfigDeepCopy(t *testing.T) {
	t.Run("nil resources", func(t *testing.T) {
		cfg := &storagev1alpha1.MayastorConfig{
			Enabled:          true,
			EtcdReplicaCount: 3,
		}
		out := cfg.DeepCopy()
		if out.EtcdReplicaCount != 3 {
			t.Errorf("expected EtcdReplicaCount 3, got %d", out.EtcdReplicaCount)
		}
		if out.IOEngineResources != nil {
			t.Error("expected nil IOEngineResources")
		}
	})

	t.Run("with resources", func(t *testing.T) {
		cfg := &storagev1alpha1.MayastorConfig{
			Enabled:          true,
			EtcdReplicaCount: 5,
			IOEngineResources: &storagev1alpha1.ResourceSpec{
				CPU:    "500m",
				Memory: "1Gi",
			},
			CoreAgentResources: &storagev1alpha1.ResourceSpec{
				CPU:    "250m",
				Memory: "128Mi",
			},
		}
		out := cfg.DeepCopy()
		if out.EtcdReplicaCount != 5 {
			t.Errorf("expected EtcdReplicaCount 5, got %d", out.EtcdReplicaCount)
		}
		if out.IOEngineResources.CPU != "500m" {
			t.Errorf("expected IO CPU 500m, got %s", out.IOEngineResources.CPU)
		}
		if out.CoreAgentResources.Memory != "128Mi" {
			t.Errorf("expected agent memory 128Mi, got %s", out.CoreAgentResources.Memory)
		}

		// Verify deep copy is independent
		out.IOEngineResources.CPU = "999m"
		if cfg.IOEngineResources.CPU != "500m" {
			t.Error("original was mutated - deep copy not independent")
		}
	})
}

func TestMayastorServiceAccount(t *testing.T) {
	sa := mayastorServiceAccount()
	if sa.Name != mayastorServiceAccountName {
		t.Errorf("expected name %s, got %s", mayastorServiceAccountName, sa.Name)
	}
	if sa.Namespace != mayastorNamespace {
		t.Errorf("expected namespace %s, got %s", mayastorNamespace, sa.Namespace)
	}
}

func TestMayastorClusterRole(t *testing.T) {
	cr := mayastorClusterRole()
	if cr.Name != mayastorClusterRoleName {
		t.Errorf("expected name %s, got %s", mayastorClusterRoleName, cr.Name)
	}
	if len(cr.Rules) < 3 {
		t.Errorf("expected at least 3 rules, got %d", len(cr.Rules))
	}
}

func TestMayastorClusterRoleBinding(t *testing.T) {
	crb := mayastorClusterRoleBinding()
	if crb.Name != mayastorClusterRoleBindingName {
		t.Errorf("expected name %s, got %s", mayastorClusterRoleBindingName, crb.Name)
	}
	if crb.RoleRef.Name != mayastorClusterRoleName {
		t.Errorf("expected role ref %s, got %s", mayastorClusterRoleName, crb.RoleRef.Name)
	}
	if len(crb.Subjects) != 1 {
		t.Errorf("expected 1 subject, got %d", len(crb.Subjects))
	}
}

func TestMayastorEtcdStatefulSet(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	sts := mayastorEtcdStatefulSet(instance)
	if sts.Name != mayastorEtcdName {
		t.Errorf("expected name %s, got %s", mayastorEtcdName, sts.Name)
	}
	if sts.Namespace != mayastorNamespace {
		t.Errorf("expected namespace %s, got %s", mayastorNamespace, sts.Namespace)
	}
	if *sts.Spec.Replicas != 1 {
		t.Errorf("expected 1 replica, got %d", *sts.Spec.Replicas)
	}
	if len(sts.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(sts.Spec.Template.Spec.Containers))
	}
	if sts.Spec.Template.Spec.Containers[0].Name != "etcd" {
		t.Errorf("expected container name etcd, got %s", sts.Spec.Template.Spec.Containers[0].Name)
	}
	if len(sts.Spec.VolumeClaimTemplates) != 1 {
		t.Errorf("expected 1 volumeClaimTemplate, got %d", len(sts.Spec.VolumeClaimTemplates))
	}
	if sts.Spec.VolumeClaimTemplates[0].Name != "data" {
		t.Errorf("expected PVC name data, got %s", sts.Spec.VolumeClaimTemplates[0].Name)
	}
	if len(sts.Spec.Template.Spec.Containers[0].VolumeMounts) != 1 {
		t.Errorf("expected 1 volume mount, got %d", len(sts.Spec.Template.Spec.Containers[0].VolumeMounts))
	}
	if sts.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name != "data" {
		t.Errorf("expected mount name data, got %s", sts.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)
	}
	env := findEnv(sts.Spec.Template.Spec.Containers[0].Env, "ETCD_DATA_DIR")
	if env == nil || env.Value != "/bitnami/etcd/data" {
		t.Error("expected ETCD_DATA_DIR=/bitnami/etcd/data")
	}
}

func TestMayastorEtcdStatefulSetCustomReplicas(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true, EtcdReplicaCount: 3},
		},
	}
	sts := mayastorEtcdStatefulSet(instance)
	if *sts.Spec.Replicas != 3 {
		t.Errorf("expected 3 replicas, got %d", *sts.Spec.Replicas)
	}
}

func TestMayastorEtcdStatefulSetCustomStorage(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{
				Enabled:               true,
				EtcdStorageSize:       "20Gi",
				EtcdStorageClassName:  "fast-ssd",
			},
		},
	}
	sts := mayastorEtcdStatefulSet(instance)
	if len(sts.Spec.VolumeClaimTemplates) != 1 {
		t.Fatalf("expected 1 PVC template, got %d", len(sts.Spec.VolumeClaimTemplates))
	}
	storage := sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage()
	if storage.String() != "20Gi" {
		t.Errorf("expected 20Gi storage, got %s", storage.String())
	}
	sc := sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName
	if sc == nil || *sc != "fast-ssd" {
		t.Errorf("expected StorageClassName fast-ssd, got %v", sc)
	}
}

func TestMayastorEtcdService(t *testing.T) {
	svc := mayastorEtcdService()
	if svc.Name != mayastorEtcdServiceName {
		t.Errorf("expected name %s, got %s", mayastorEtcdServiceName, svc.Name)
	}
	if svc.Namespace != mayastorNamespace {
		t.Errorf("expected namespace %s, got %s", mayastorNamespace, svc.Namespace)
	}
	if len(svc.Spec.Ports) != 1 {
		t.Errorf("expected 1 port, got %d", len(svc.Spec.Ports))
	}
}

func TestMayastorAgentCoreDeployment(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	dep := mayastorAgentCoreDeployment(instance)
	if dep.Name != mayastorAgentCoreName {
		t.Errorf("expected name %s, got %s", mayastorAgentCoreName, dep.Name)
	}
	if dep.Namespace != mayastorNamespace {
		t.Errorf("expected namespace %s, got %s", mayastorNamespace, dep.Namespace)
	}
	if dep.Spec.Template.Spec.ServiceAccountName != mayastorServiceAccountName {
		t.Errorf("expected SA %s, got %s", mayastorServiceAccountName, dep.Spec.Template.Spec.ServiceAccountName)
	}
}

func TestMayastorAgentCoreService(t *testing.T) {
	svc := mayastorAgentCoreService()
	if svc.Name != mayastorAgentCoreServiceName {
		t.Errorf("expected name %s, got %s", mayastorAgentCoreServiceName, svc.Name)
	}
	if svc.Namespace != mayastorNamespace {
		t.Errorf("expected namespace %s, got %s", mayastorNamespace, svc.Namespace)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != 50051 {
		t.Errorf("expected port 50051, got %v", svc.Spec.Ports)
	}
	if svc.Spec.Selector["app.kubernetes.io/component"] != "mayastor-agent-core" {
		t.Errorf("expected selector component mayastor-agent-core, got %v", svc.Spec.Selector)
	}
}

func TestMayastorAPIRestDeployment(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	dep := mayastorAPIRestDeployment(instance)
	if dep.Name != mayastorAPIRestName {
		t.Errorf("expected name %s, got %s", mayastorAPIRestName, dep.Name)
	}
}

func TestMayastorAPIRestService(t *testing.T) {
	svc := mayastorAPIRestService()
	if svc.Name != mayastorAPIRestServiceName {
		t.Errorf("expected name %s, got %s", mayastorAPIRestServiceName, svc.Name)
	}
	if svc.Namespace != mayastorNamespace {
		t.Errorf("expected namespace %s, got %s", mayastorNamespace, svc.Namespace)
	}
}

func TestMayastorCSIControllerDeployment(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	dep := mayastorCSIControllerDeployment(instance)
	if dep.Name != mayastorCSIControllerName {
		t.Errorf("expected name %s, got %s", mayastorCSIControllerName, dep.Name)
	}
	if dep.Namespace != mayastorNamespace {
		t.Errorf("expected namespace %s, got %s", mayastorNamespace, dep.Namespace)
	}
	if len(dep.Spec.Template.Spec.Containers) != 6 {
		t.Errorf("expected 6 containers, got %d", len(dep.Spec.Template.Spec.Containers))
	}
}

func TestMayastorIOEngineDaemonSet(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	ds := mayastorIOEngineDaemonSet(instance)
	if ds.Name != mayastorIOEngineName {
		t.Errorf("expected name %s, got %s", mayastorIOEngineName, ds.Name)
	}
	if ds.Namespace != mayastorNamespace {
		t.Errorf("expected namespace %s, got %s", mayastorNamespace, ds.Namespace)
	}
	if ds.Spec.Template.Spec.Containers[0].SecurityContext.Privileged == nil || !*ds.Spec.Template.Spec.Containers[0].SecurityContext.Privileged {
		t.Error("expected privileged security context")
	}
	if _, ok := ds.Spec.Template.Spec.NodeSelector["openebs.io/engine"]; !ok {
		t.Error("expected nodeSelector openebs.io/engine=mayastor")
	}
}

func TestMayastorCSINodeDaemonSet(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	ds := mayastorCSINodeDaemonSet(instance)
	if ds.Name != mayastorCSINodeName {
		t.Errorf("expected name %s, got %s", mayastorCSINodeName, ds.Name)
	}
	if len(ds.Spec.Template.Spec.Containers) != 2 {
		t.Errorf("expected 2 containers, got %d", len(ds.Spec.Template.Spec.Containers))
	}
}

func TestMayastorOperatorDiskpoolDeployment(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	dep := mayastorOperatorDiskpoolDeployment(instance)
	if dep.Name != mayastorDiskpoolName {
		t.Errorf("expected name %s, got %s", mayastorDiskpoolName, dep.Name)
	}
}

func TestMayastorHANodeDaemonSet(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	ds := mayastorHANodeDaemonSet(instance)
	if ds.Name != mayastorHANodeName {
		t.Errorf("expected name %s, got %s", mayastorHANodeName, ds.Name)
	}
	if ds.Namespace != mayastorNamespace {
		t.Errorf("expected namespace %s, got %s", mayastorNamespace, ds.Namespace)
	}
}

func TestMayastorCSIDriver(t *testing.T) {
	driver := mayastorCSIDriver()
	if driver.Name != mayastorCSIDriverName {
		t.Errorf("expected name %s, got %s", mayastorCSIDriverName, driver.Name)
	}
	if driver.Spec.AttachRequired == nil || !*driver.Spec.AttachRequired {
		t.Error("expected AttachRequired=true")
	}
}

func TestMayastorStorageClass(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
		},
	}
	sc := mayastorStorageClass(mayastorSCName, instance)
	if sc.Name != mayastorSCName {
		t.Errorf("expected name %s, got %s", mayastorSCName, sc.Name)
	}
	if sc.Provisioner != mayastorCSIDriverName {
		t.Errorf("expected provisioner %s, got %s", mayastorCSIDriverName, sc.Provisioner)
	}
	if sc.Parameters["repl"] != "1" {
		t.Errorf("expected repl=1, got %s", sc.Parameters["repl"])
	}
}

func TestMayastorStorageClassCustomName(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true, StorageClassName: "my-mayastor"},
		},
	}
	sc := mayastorStorageClass("my-mayastor", instance)
	if sc.Name != "my-mayastor" {
		t.Errorf("expected name my-mayastor, got %s", sc.Name)
	}
}

func TestMayastorImageOverride(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
			Images: &storagev1alpha1.ImageConfig{
				Mayastor: "2.8.0",
			},
		},
	}
	dep := mayastorAgentCoreDeployment(instance)
	expected := "openebs/mayastor-agent-core:2.8.0"
	if dep.Spec.Template.Spec.Containers[0].Image != expected {
		t.Errorf("expected image %s, got %s", expected, dep.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestMayastorEtcdImageOverride(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			Mayastor: &storagev1alpha1.MayastorConfig{Enabled: true},
			Images: &storagev1alpha1.ImageConfig{
				Etcd: "custom/etcd:v1",
			},
		},
	}
	sts := mayastorEtcdStatefulSet(instance)
	if sts.Spec.Template.Spec.Containers[0].Image != "custom/etcd:v1" {
		t.Errorf("expected custom/etcd:v1, got %s", sts.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestOpenEBSSpecDeepCopy(t *testing.T) {
	spec := &storagev1alpha1.OpenEBSSpec{
		LVM: &storagev1alpha1.LVMConfig{
			Enabled:          true,
			StorageClassName: "lvm-sc",
			VolumeGroup:      "vg1",
			IsDefaultClass:   true,
		},
		Hostpath: &storagev1alpha1.HostpathConfig{
			Enabled:          true,
			BasePath:         "/data",
			StorageClassName: "hp-sc",
			IsDefaultClass:   false,
		},
		ImagePullSecrets: []string{"regcred"},
		NodeSelector:     map[string]string{"disktype": "ssd"},
	}

	out := spec.DeepCopy()

	if out.LVM.StorageClassName != "lvm-sc" {
		t.Errorf("LVM StorageClassName: want lvm-sc, got %s", out.LVM.StorageClassName)
	}
	if len(out.ImagePullSecrets) != 1 || out.ImagePullSecrets[0] != "regcred" {
		t.Errorf("ImagePullSecrets not copied correctly")
	}
	if out.NodeSelector["disktype"] != "ssd" {
		t.Errorf("NodeSelector not copied correctly")
	}

	// Verify deep copy independence
	out.LVM.VolumeGroup = "modified"
	if spec.LVM.VolumeGroup != "vg1" {
		t.Error("original LVM was mutated")
	}
	out.ImagePullSecrets[0] = "modified"
	if spec.ImagePullSecrets[0] != "regcred" {
		t.Error("original ImagePullSecrets was mutated")
	}
	out.NodeSelector["disktype"] = "modified"
	if spec.NodeSelector["disktype"] != "ssd" {
		t.Error("original NodeSelector was mutated")
	}
}

func TestOpenEBSDeepCopy(t *testing.T) {
	obj := &storagev1alpha1.OpenEBS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "openebs",
		},
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM: &storagev1alpha1.LVMConfig{Enabled: true},
		},
	}

	clone := obj.DeepCopy()
	if clone.Name != "test" {
		t.Errorf("name not copied: %s", clone.Name)
	}

	runtimeObj := obj.DeepCopyObject()
	if runtimeObj.(*storagev1alpha1.OpenEBS).Name != "test" {
		t.Error("DeepCopyObject returned wrong type")
	}

	nilObj := (*storagev1alpha1.OpenEBS)(nil).DeepCopy()
	if nilObj != nil {
		t.Error("DeepCopy of nil should return nil")
	}

	nilCopy := (*storagev1alpha1.OpenEBS)(nil).DeepCopyObject()
	if nilCopy != nil {
		t.Error("DeepCopyObject of nil should return nil")
	}
}

func TestOpenEBSListDeepCopy(t *testing.T) {
	list := &storagev1alpha1.OpenEBSList{
		Items: []storagev1alpha1.OpenEBS{
			{ObjectMeta: metav1.ObjectMeta{Name: "a"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "b"}},
		},
	}
	clone := list.DeepCopy()
	if len(clone.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(clone.Items))
	}
	clone.Items[0].Name = "modified"
	if list.Items[0].Name != "a" {
		t.Error("original list item was mutated")
	}

	nilList := (*storagev1alpha1.OpenEBSList)(nil).DeepCopy()
	if nilList != nil {
		t.Error("DeepCopy of nil list should return nil")
	}
}

func TestEngineStatusValues(t *testing.T) {
	tests := []struct {
		engine storagev1alpha1.OpenEBSEngine
		str    string
	}{
		{storagev1alpha1.OpenEBSEngineHostpath, "hostpath"},
		{storagev1alpha1.OpenEBSEngineLVM, "lvm"},
		{storagev1alpha1.OpenEBSEngineZFS, "zfs"},
		{storagev1alpha1.OpenEBSEngineRawfile, "rawfile"},
		{storagev1alpha1.OpenEBSEngineMayastor, "mayastor"},
	}
	for _, tt := range tests {
		if string(tt.engine) != tt.str {
			t.Errorf("engine %s string value is %s", tt.str, string(tt.engine))
		}
	}
}

func TestPhaseValues(t *testing.T) {
	tests := []struct {
		phase storagev1alpha1.OpenEBSPhase
		str   string
	}{
		{storagev1alpha1.OpenEBSPhasePending, "Pending"},
		{storagev1alpha1.OpenEBSPhaseInstalling, "Installing"},
		{storagev1alpha1.OpenEBSPhaseRunning, "Running"},
		{storagev1alpha1.OpenEBSPhaseDegraded, "Degraded"},
		{storagev1alpha1.OpenEBSPhaseFailed, "Failed"},
	}
	for _, tt := range tests {
		if string(tt.phase) != tt.str {
			t.Errorf("phase %s string value is %s", tt.str, string(tt.phase))
		}
	}
}

func TestHelperFunctions(t *testing.T) {
	if *boolPtr(true) != true {
		t.Error("boolPtr(true) should return true")
	}
	if *boolPtr(false) != false {
		t.Error("boolPtr(false) should return false")
	}
	if *int32Ptr(42) != 42 {
		t.Error("int32Ptr(42) should return 42")
	}
}

func TestVolumeBindingMode(t *testing.T) {
	// Ensure the volume binding mode is WaitForFirstConsumer for all storage classes
	sc1 := lvmStorageClass("test", &storagev1alpha1.LVMConfig{Enabled: true})
	if sc1.VolumeBindingMode == nil || *sc1.VolumeBindingMode != storagev1.VolumeBindingWaitForFirstConsumer {
		t.Error("LVM SC should use WaitForFirstConsumer binding mode")
	}

	sc2 := hostpathStorageClass("test", &storagev1alpha1.HostpathConfig{Enabled: true})
	if sc2.VolumeBindingMode == nil || *sc2.VolumeBindingMode != storagev1.VolumeBindingWaitForFirstConsumer {
		t.Error("Hostpath SC should use WaitForFirstConsumer binding mode")
	}

	sc3 := zfsStorageClass("test", &storagev1alpha1.ZFSConfig{Enabled: true})
	if sc3.VolumeBindingMode == nil || *sc3.VolumeBindingMode != storagev1.VolumeBindingWaitForFirstConsumer {
		t.Error("ZFS SC should use WaitForFirstConsumer binding mode")
	}

	sc4 := rawfileStorageClass("test", &storagev1alpha1.RawfileConfig{Enabled: true})
	if sc4.VolumeBindingMode == nil || *sc4.VolumeBindingMode != storagev1.VolumeBindingWaitForFirstConsumer {
		t.Error("Rawfile SC should use WaitForFirstConsumer binding mode")
	}
}

func TestReclaimPolicy(t *testing.T) {
	sc := lvmStorageClass("test", &storagev1alpha1.LVMConfig{Enabled: true})
	if sc.ReclaimPolicy == nil || *sc.ReclaimPolicy != corev1.PersistentVolumeReclaimDelete {
		t.Error("StorageClass should use Delete reclaim policy")
	}
}

func TestResolveImage(t *testing.T) {
	if got := resolveImage("", "default:v1"); got != "default:v1" {
		t.Errorf("empty override, expected default:v1, got %s", got)
	}
	if got := resolveImage("custom:v2", "default:v1"); got != "custom:v2" {
		t.Errorf("non-empty override, expected custom:v2, got %s", got)
	}
}

func TestDefaultImages(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM:      &storagev1alpha1.LVMConfig{Enabled: true},
			ZFS:      &storagev1alpha1.ZFSConfig{Enabled: true},
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true},
			Rawfile:  &storagev1alpha1.RawfileConfig{Enabled: true},
		},
	}

	t.Run("LVM controller uses defaults", func(t *testing.T) {
		dep := lvmControllerDeployment(instance)
		images := map[string]string{}
		for _, c := range dep.Spec.Template.Spec.Containers {
			images[c.Name] = c.Image
		}
		if images["csi-provisioner"] != defaultCSIProvisioner {
			t.Errorf("csi-provisioner: expected %s, got %s", defaultCSIProvisioner, images["csi-provisioner"])
		}
		if images["csi-resizer"] != defaultCSIResizer {
			t.Errorf("csi-resizer: expected %s, got %s", defaultCSIResizer, images["csi-resizer"])
		}
		if images["csi-snapshotter"] != defaultCSISnapshotter {
			t.Errorf("csi-snapshotter: expected %s, got %s", defaultCSISnapshotter, images["csi-snapshotter"])
		}
		if images["lvm-plugin"] != defaultLVMImage {
			t.Errorf("lvm-plugin: expected %s, got %s", defaultLVMImage, images["lvm-plugin"])
		}
	})

	t.Run("LVM node uses defaults", func(t *testing.T) {
		ds := lvmNodeDaemonSet(instance)
		images := map[string]string{}
		for _, c := range ds.Spec.Template.Spec.Containers {
			images[c.Name] = c.Image
		}
		if images["csi-node-driver-registrar"] != defaultCSINodeRegistrar {
			t.Errorf("csi-node-driver-registrar: expected %s, got %s", defaultCSINodeRegistrar, images["csi-node-driver-registrar"])
		}
		if images["lvm-node-plugin"] != defaultLVMImage {
			t.Errorf("lvm-node-plugin: expected %s, got %s", defaultLVMImage, images["lvm-node-plugin"])
		}
	})

	t.Run("hostpath uses defaults", func(t *testing.T) {
		dep := hostpathDeployment(instance)
		c := dep.Spec.Template.Spec.Containers[0]
		if c.Image != defaultHostpathImage {
			t.Errorf("expected %s, got %s", defaultHostpathImage, c.Image)
		}
		env := findEnv(c.Env, "OPENEBS_IO_HELPER_IMAGE")
		if env == nil || env.Value != defaultHelperImage {
			t.Errorf("OPENEBS_IO_HELPER_IMAGE: expected %s", defaultHelperImage)
		}
	})

	t.Run("ZFS controller uses defaults", func(t *testing.T) {
		dep := zfsControllerDeployment(instance)
		images := map[string]string{}
		for _, c := range dep.Spec.Template.Spec.Containers {
			images[c.Name] = c.Image
		}
		if images["csi-provisioner"] != defaultCSIProvisioner {
			t.Errorf("csi-provisioner: expected %s, got %s", defaultCSIProvisioner, images["csi-provisioner"])
		}
		if images["csi-resizer"] != defaultCSIResizer {
			t.Errorf("csi-resizer: expected %s, got %s", defaultCSIResizer, images["csi-resizer"])
		}
		if images["zfs-plugin"] != defaultZFSImage {
			t.Errorf("zfs-plugin: expected %s, got %s", defaultZFSImage, images["zfs-plugin"])
		}
	})

	t.Run("ZFS node uses defaults", func(t *testing.T) {
		ds := zfsNodeDaemonSet(instance)
		images := map[string]string{}
		for _, c := range ds.Spec.Template.Spec.Containers {
			images[c.Name] = c.Image
		}
		if images["csi-node-driver-registrar"] != defaultCSINodeRegistrar {
			t.Errorf("csi-node-driver-registrar: expected %s, got %s", defaultCSINodeRegistrar, images["csi-node-driver-registrar"])
		}
		if images["zfs-node-plugin"] != defaultZFSImage {
			t.Errorf("zfs-node-plugin: expected %s, got %s", defaultZFSImage, images["zfs-node-plugin"])
		}
	})

	t.Run("rawfile uses defaults", func(t *testing.T) {
		dep := rawfileDeployment(instance)
		c := dep.Spec.Template.Spec.Containers[0]
		if c.Image != defaultRawfileImage {
			t.Errorf("expected %s, got %s", defaultRawfileImage, c.Image)
		}
	})
}

func TestCustomImages(t *testing.T) {
	custom := &storagev1alpha1.ImageConfig{
		LVM:              "myreg/lvm:v1.0.0",
		Hostpath:         "myreg/hostpath:v1.0.0",
		ZFS:              "myreg/zfs:v1.0.0",
		Rawfile:          "myreg/rawfile:v1.0.0",
		CSIProvisioner:   "myreg/csi-provisioner:v1.0.0",
		CSIResizer:       "myreg/csi-resizer:v1.0.0",
		CSISnapshotter:   "myreg/csi-snapshotter:v1.0.0",
		CSINodeRegistrar: "myreg/csi-node-registrar:v1.0.0",
	}

	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM:      &storagev1alpha1.LVMConfig{Enabled: true},
			ZFS:      &storagev1alpha1.ZFSConfig{Enabled: true},
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true},
			Rawfile:  &storagev1alpha1.RawfileConfig{Enabled: true},
			Images:   custom,
		},
	}

	t.Run("LVM controller uses custom images", func(t *testing.T) {
		dep := lvmControllerDeployment(instance)
		images := map[string]string{}
		for _, c := range dep.Spec.Template.Spec.Containers {
			images[c.Name] = c.Image
		}
		if images["csi-provisioner"] != custom.CSIProvisioner {
			t.Errorf("csi-provisioner: expected %s, got %s", custom.CSIProvisioner, images["csi-provisioner"])
		}
		if images["csi-resizer"] != custom.CSIResizer {
			t.Errorf("csi-resizer: expected %s, got %s", custom.CSIResizer, images["csi-resizer"])
		}
		if images["csi-snapshotter"] != custom.CSISnapshotter {
			t.Errorf("csi-snapshotter: expected %s, got %s", custom.CSISnapshotter, images["csi-snapshotter"])
		}
		if images["lvm-plugin"] != custom.LVM {
			t.Errorf("lvm-plugin: expected %s, got %s", custom.LVM, images["lvm-plugin"])
		}
	})

	t.Run("ZFS controller uses custom images", func(t *testing.T) {
		dep := zfsControllerDeployment(instance)
		images := map[string]string{}
		for _, c := range dep.Spec.Template.Spec.Containers {
			images[c.Name] = c.Image
		}
		if images["csi-provisioner"] != custom.CSIProvisioner {
			t.Errorf("csi-provisioner: expected %s, got %s", custom.CSIProvisioner, images["csi-provisioner"])
		}
		if images["csi-resizer"] != custom.CSIResizer {
			t.Errorf("csi-resizer: expected %s, got %s", custom.CSIResizer, images["csi-resizer"])
		}
		if images["zfs-plugin"] != custom.ZFS {
			t.Errorf("zfs-plugin: expected %s, got %s", custom.ZFS, images["zfs-plugin"])
		}
	})

	t.Run("hostpath uses custom image", func(t *testing.T) {
		dep := hostpathDeployment(instance)
		c := dep.Spec.Template.Spec.Containers[0]
		if c.Image != custom.Hostpath {
			t.Errorf("expected %s, got %s", custom.Hostpath, c.Image)
		}
	})

	t.Run("rawfile uses custom image", func(t *testing.T) {
		dep := rawfileDeployment(instance)
		c := dep.Spec.Template.Spec.Containers[0]
		if c.Image != custom.Rawfile {
			t.Errorf("expected %s, got %s", custom.Rawfile, c.Image)
		}
	})
}

func TestPartialImageOverride(t *testing.T) {
	instance := &storagev1alpha1.OpenEBS{
		Spec: storagev1alpha1.OpenEBSSpec{
			LVM:      &storagev1alpha1.LVMConfig{Enabled: true},
			Hostpath: &storagev1alpha1.HostpathConfig{Enabled: true},
			Images: &storagev1alpha1.ImageConfig{
				LVM:      "myreg/lvm:v9.9.9",
				Hostpath: "myreg/hostpath:v9.9.9",
			},
		},
	}

	t.Run("LVM CSI sidecars fall back to defaults", func(t *testing.T) {
		dep := lvmControllerDeployment(instance)
		images := map[string]string{}
		for _, c := range dep.Spec.Template.Spec.Containers {
			images[c.Name] = c.Image
		}
		if images["csi-provisioner"] != defaultCSIProvisioner {
			t.Errorf("csi-provisioner should be default, got %s", images["csi-provisioner"])
		}
		if images["lvm-plugin"] != "myreg/lvm:v9.9.9" {
			t.Errorf("lvm-plugin should be overridden, got %s", images["lvm-plugin"])
		}
	})

	t.Run("hostpath overridden, defaults for unspecified", func(t *testing.T) {
		dep := hostpathDeployment(instance)
		c := dep.Spec.Template.Spec.Containers[0]
		if c.Image != "myreg/hostpath:v9.9.9" {
			t.Errorf("expected overridden image, got %s", c.Image)
		}
	})
}

func findEnv(env []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range env {
		if env[i].Name == name {
			return &env[i]
		}
	}
	return nil
}

func TestMayastorVolumeSnapshotClass(t *testing.T) {
	vsc := mayastorVolumeSnapshotClass(mayastorSnapshotClassName)
	if vsc.GetName() != mayastorSnapshotClassName {
		t.Errorf("expected name %s, got %s", mayastorSnapshotClassName, vsc.GetName())
	}
	if vsc.GetKind() != "VolumeSnapshotClass" {
		t.Errorf("expected kind VolumeSnapshotClass, got %s", vsc.GetKind())
	}
	driver, found, _ := unstructured.NestedString(vsc.Object, "driver")
	if !found || driver != mayastorCSIDriverName {
		t.Errorf("expected driver %s, got %v/%s", mayastorCSIDriverName, found, driver)
	}
	policy, found, _ := unstructured.NestedString(vsc.Object, "deletionPolicy")
	if !found || policy != "Delete" {
		t.Errorf("expected deletionPolicy Delete, got %v/%s", found, policy)
	}
}

func TestMayastorVolumeSnapshotClassCustomName(t *testing.T) {
	customName := "my-custom-snapshot-class"
	vsc := mayastorVolumeSnapshotClass(customName)
	if vsc.GetName() != customName {
		t.Errorf("expected name %s, got %s", customName, vsc.GetName())
	}
}
