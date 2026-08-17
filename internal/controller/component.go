package controller

import (
	storagev1alpha1 "github.com/aldershaab-it/openebs-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	defaultLVMImage              = "openebs/lvm-driver:1.9.1"
	defaultHostpathImage         = "openebs/provisioner-localpv:4.5.0"
	defaultZFSImage              = "openebs/zfs-driver:2.10.1"
	defaultRawfileImage          = "openebs/rawfile-localpv:0.14.1"
	defaultHelperImage           = "openebs/linux-utils:4.5.0"
	defaultCSIProvisioner        = "registry.k8s.io/sig-storage/csi-provisioner:v4.0.1"
	defaultCSIResizer            = "registry.k8s.io/sig-storage/csi-resizer:v1.10.1"
	defaultCSISnapshotter        = "registry.k8s.io/sig-storage/csi-snapshotter:v7.0.2"
	defaultCSINodeRegistrar      = "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.10.1"
	defaultCSIAttacher           = "registry.k8s.io/sig-storage/csi-attacher:v4.8.1"
	defaultCSISnapshotController = "registry.k8s.io/sig-storage/snapshot-controller:v8.2.0"
	defaultMayastorTag           = "v2.11.1"
	defaultEtcdImage             = "openebs/etcd:3.6.4-debian-12-r0"
)

func resolveImage(override, defaultImage string) string {
	if override != "" {
		return override
	}
	return defaultImage
}

func boolPtr(b bool) *bool { return &b }

func int32Ptr(i int32) *int32 { return &i }

var intOrString1 = intstr.FromInt32(1)

// ---- LVM Resources ----

func lvmServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-lvm-controller", Namespace: openebsNamespace, Labels: labels("lvm-rbac")},
	}
}

func lvmClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-lvm-role"},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"namespaces"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumes", "services"}, Verbs: []string{"get", "list", "watch", "create", "delete", "update", "patch"}},
			{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumeclaims"}, Verbs: []string{"get", "list", "watch", "update"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumeclaims/status"}, Verbs: []string{"patch", "update"}},
			{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list", "watch", "update", "patch"}},
			{APIGroups: []string{""}, Resources: []string{"events"}, Verbs: []string{"get", "list", "watch", "create", "update", "patch"}},
			{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get", "list"}},
			{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"storageclasses", "volumeattachments", "csinodes", "volumeattributesclasses"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"csistoragecapacities"}, Verbs: []string{"*"}},
			{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, Verbs: []string{"get", "watch", "list", "delete", "update", "create"}},
			{APIGroups: []string{"local.openebs.io"}, Resources: []string{"*"}, Verbs: []string{"*"}},
			{APIGroups: []string{"snapshot.storage.k8s.io"}, Resources: []string{"volumesnapshotclasses"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"snapshot.storage.k8s.io"}, Resources: []string{"volumesnapshotcontents", "volumesnapshots"}, Verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
			{APIGroups: []string{"snapshot.storage.k8s.io"}, Resources: []string{"volumesnapshotcontents/status", "volumesnapshots/status"}, Verbs: []string{"update", "patch"}},
			{APIGroups: []string{"apiextensions.k8s.io"}, Resources: []string{"customresourcedefinitions"}, Verbs: []string{"create", "list", "watch", "delete"}},
		},
	}
}

func lvmClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-lvm-binding", Labels: labels("lvm-rbac")},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "openebs-lvm-role"},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "openebs-lvm-controller", Namespace: openebsNamespace}},
	}
}

func lvmControllerDeployment(instance *storagev1alpha1.OpenEBS) *appsv1.Deployment {
	csiProvisionerImg, csiResizerImg, csiSnapshotterImg, lvmPluginImg := defaultCSIProvisioner, defaultCSIResizer, defaultCSISnapshotter, defaultLVMImage
	if instance.Spec.Images != nil {
		csiProvisionerImg = resolveImage(instance.Spec.Images.CSIProvisioner, defaultCSIProvisioner)
		csiResizerImg = resolveImage(instance.Spec.Images.CSIResizer, defaultCSIResizer)
		csiSnapshotterImg = resolveImage(instance.Spec.Images.CSISnapshotter, defaultCSISnapshotter)
		lvmPluginImg = resolveImage(instance.Spec.Images.LVM, defaultLVMImage)
	}

	labels := labels("lvm-controller")
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lvmControllerName,
			Namespace: openebsNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName: "openebs-lvm-controller",
					Containers: []corev1.Container{
						{
							Name:  "csi-provisioner",
							Image: csiProvisionerImg,
							Args: []string{
								"--csi-address=$(ADDRESS)",
								"--v=2",
								"--feature-gates=Topology=true",
								"--timeout=150s",
								"--leader-election",
								"--leader-election-lease-duration=120s",
								"--leader-election-renew-deadline=80s",
								"--leader-election-retry-period=30s",
								"--extra-create-metadata=true",
								"--default-fstype=ext4",
							},
							Env: []corev1.EnvVar{{Name: "ADDRESS", Value: "/var/lib/csi/sockets/pluginproxy/csi.sock"}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: "/var/lib/csi/sockets/pluginproxy/"},
							},
						},
						{
							Name:  "csi-resizer",
							Image: csiResizerImg,
							Args: []string{
								"--csi-address=$(ADDRESS)",
								"--v=2",
								"--timeout=150s",
								"--leader-election",
								"--leader-election-lease-duration=120s",
								"--leader-election-renew-deadline=80s",
								"--leader-election-retry-period=30s",
							},
							Env: []corev1.EnvVar{{Name: "ADDRESS", Value: "/var/lib/csi/sockets/pluginproxy/csi.sock"}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: "/var/lib/csi/sockets/pluginproxy/"},
							},
						},
						{
							Name:  "csi-snapshotter",
							Image: csiSnapshotterImg,
							Args: []string{
								"--csi-address=$(ADDRESS)",
								"--v=2",
								"--timeout=150s",
								"--leader-election",
							},
							Env: []corev1.EnvVar{{Name: "ADDRESS", Value: "/var/lib/csi/sockets/pluginproxy/csi.sock"}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: "/var/lib/csi/sockets/pluginproxy/"},
							},
						},
						{
							Name:  "lvm-plugin",
							Image: lvmPluginImg,
							Args:  []string{"--plugin=controller", "--endpoint=$(CSI_ENDPOINT)", "--nodeid=$(OPENEBS_NODE_ID)"},
							Env: []corev1.EnvVar{
								{Name: "OPENEBS_NAMESPACE", Value: openebsNamespace},
								{Name: "CSI_ENDPOINT", Value: "unix:///var/lib/csi/sockets/pluginproxy/csi.sock"},
								{Name: "OPENEBS_NODE_ID", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
								{Name: "OPENEBS_CSI_ENDPOINT", Value: "unix:///var/lib/csi/sockets/pluginproxy/csi.sock"},
								{Name: "OPENEBS_MONITOR_PERIOD", Value: "60"},
								{Name: "OPENEBS_IO_INSTALLER_TYPE", Value: "openebs-operator"},
							},
							SecurityContext: &corev1.SecurityContext{Privileged: boolPtr(true)},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: "/var/lib/csi/sockets/pluginproxy/"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "socket-dir", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
					},
				},
			},
		},
	}
}

func lvmNodeDaemonSet(instance *storagev1alpha1.OpenEBS) *appsv1.DaemonSet {
	csiNodeRegistrarImg, lvmPluginImg := defaultCSINodeRegistrar, defaultLVMImage
	if instance.Spec.Images != nil {
		csiNodeRegistrarImg = resolveImage(instance.Spec.Images.CSINodeRegistrar, defaultCSINodeRegistrar)
		lvmPluginImg = resolveImage(instance.Spec.Images.LVM, defaultLVMImage)
	}

	labels := labels("lvm-node")
	maxUnavailable := intstr.FromString("100%")
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lvmNodeName,
			Namespace: openebsNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName: "openebs-lvm-controller",
					HostNetwork:        false,
					Containers: []corev1.Container{
						{
							Name:  "csi-node-driver-registrar",
							Image: csiNodeRegistrarImg,
							Args: []string{
								"--v=5",
								"--csi-address=$(ADDRESS)",
								"--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)",
							},
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"/bin/sh", "-c", "rm -rf /registration/lvm-localpv /registration/lvm-localpv-reg.sock"},
									},
								},
							},
							Env: []corev1.EnvVar{
								{Name: "ADDRESS", Value: "/plugin/csi.sock"},
								{Name: "DRIVER_REG_SOCK_PATH", Value: "/var/lib/kubelet/plugins/lvm-localpv/csi.sock"},
								{Name: "KUBE_NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "plugin-dir", MountPath: "/plugin"},
								{Name: "registration-dir", MountPath: "/registration"},
							},
						},
						{
							Name:  "lvm-node-plugin",
							Image: lvmPluginImg,
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"SYS_ADMIN"},
								},
								AllowPrivilegeEscalation: boolPtr(true),
							},
							Args: []string{
								"--nodeid=$(OPENEBS_NODE_ID)",
								"--endpoint=$(OPENEBS_CSI_ENDPOINT)",
								"--plugin=$(OPENEBS_NODE_DRIVER)",
								"--kube-api-qps=0",
								"--kube-api-burst=0",
								"--kubelet-dir=/var/lib/kubelet/",
							},
							Env: []corev1.EnvVar{
								{Name: "OPENEBS_NODE_ID", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
								{Name: "OPENEBS_NAMESPACE", Value: openebsNamespace},
								{Name: "OPENEBS_CSI_ENDPOINT", Value: "unix:///plugin/csi.sock"},
								{Name: "OPENEBS_NODE_DRIVER", Value: "agent"},
								{Name: "OPENEBS_IO_INSTALLER_TYPE", Value: "openebs-operator"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "plugin-dir", MountPath: "/plugin"},
								{Name: "device-dir", MountPath: "/dev"},
								{Name: "pods-mount-dir", MountPath: "/var/lib/kubelet", MountPropagation: &hostpathMountPropagation},
								{Name: "node-dir", MountPath: "/var/lib/openebs"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "plugin-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/lvm-localpv/", Type: &hostpathDirOrCreate}}},
						{Name: "registration-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins_registry/", Type: &hostpathDirOrCreate}}},
						{Name: "device-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/dev", Type: &hostpathDir}}},
						{Name: "pods-mount-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet", Type: &hostpathDir}}},
						{Name: "node-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/openebs", Type: &hostpathDirOrCreate}}},
					},
				},
			},
		},
	}
}

var hostpathMountPropagation = corev1.MountPropagationBidirectional
var hostpathMountPropagationHostToContainer = corev1.MountPropagationHostToContainer
var hostpathDir = corev1.HostPathDirectory
var hostpathDirOrCreate = corev1.HostPathDirectoryOrCreate

func lvmCSIDriver() *storagev1.CSIDriver {
	attachRequired := true
	podInfoOnMount := true
	return &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: lvmCSIDriverName, Labels: labels("lvm-csidriver")},
		Spec: storagev1.CSIDriverSpec{
			AttachRequired: &attachRequired,
			PodInfoOnMount: &podInfoOnMount,
			FSGroupPolicy:  &fsGroupPolicyFile,
		},
	}
}

var fsGroupPolicyFile = storagev1.FileFSGroupPolicy

func lvmStorageClass(name string, cfg *storagev1alpha1.LVMConfig) *storagev1.StorageClass {
	vgName := "lvmvg"
	if cfg.VolumeGroup != "" {
		vgName = cfg.VolumeGroup
	}
	deletePolicy := corev1.PersistentVolumeReclaimDelete
	allowExpansion := true
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels("lvm-sc"),
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": boolToStr(cfg.IsDefaultClass),
			},
		},
		Provisioner:          lvmCSIDriverName,
		ReclaimPolicy:        &deletePolicy,
		VolumeBindingMode:    &volumeBindingWaitForFirstConsumer,
		AllowVolumeExpansion: &allowExpansion,
		Parameters: map[string]string{
			"storage":  "lvm",
			"volgroup": vgName,
		},
	}
	return sc
}

// ---- Hostpath Resources ----

func hostpathServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-localpv-provisioner", Namespace: openebsNamespace, Labels: labels("hostpath-rbac")},
	}
}

func hostpathClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-localpv-provisioner", Labels: labels("hostpath-rbac")},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{""}, Resources: []string{"namespaces", "pods", "events", "endpoints"}, Verbs: []string{"*"}},
			{APIGroups: []string{""}, Resources: []string{"resourcequotas", "limitranges"}, Verbs: []string{"list", "watch"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumes", "persistentvolumeclaims"}, Verbs: []string{"*"}},
			{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get", "create", "update"}},
			{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"storageclasses"}, Verbs: []string{"*"}},
			{APIGroups: []string{"apiextensions.k8s.io"}, Resources: []string{"customresourcedefinitions"}, Verbs: []string{"get", "list", "create", "update", "delete", "patch"}},
			{APIGroups: []string{"openebs.io"}, Resources: []string{"*"}, Verbs: []string{"*"}},
			{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, Verbs: []string{"get", "create", "update"}},
		},
	}
}

func hostpathClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-localpv-provisioner", Labels: labels("hostpath-rbac")},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "openebs-localpv-provisioner"},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "openebs-localpv-provisioner", Namespace: openebsNamespace}},
	}
}

func hostpathDeployment(instance *storagev1alpha1.OpenEBS) *appsv1.Deployment {
	hostpathImg := defaultHostpathImage
	if instance.Spec.Images != nil {
		hostpathImg = resolveImage(instance.Spec.Images.Hostpath, defaultHostpathImage)
	}

	lbls := labels("hostpath-provisioner")
	replicas := int32(1)
	basePath := "/var/openebs/local"
	if instance.Spec.Hostpath != nil && instance.Spec.Hostpath.BasePath != "" {
		basePath = instance.Spec.Hostpath.BasePath
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostpathDeployName,
			Namespace: openebsNamespace,
			Labels:    lbls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: lbls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: lbls},
				Spec: corev1.PodSpec{
					ServiceAccountName: "openebs-localpv-provisioner",
					Containers: []corev1.Container{
						{
							Name:  "provisioner",
							Image: hostpathImg,
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"sh", "-c", "test `pgrep -c \"^provisioner-loc.*\"` = 1"},
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       60,
							},
							Env: []corev1.EnvVar{
								{Name: "NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
								{Name: "OPENEBS_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
								{Name: "OPENEBS_SERVICE_ACCOUNT", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.serviceAccountName"}}},
								{Name: "OPENEBS_IO_ENABLE_ANALYTICS", Value: "false"},
								{Name: "OPENEBS_IO_BASE_PATH", Value: basePath},
								{Name: "OPENEBS_IO_WORKER_THREADS", Value: "1"},
								{Name: "OPENEBS_IO_HELPER_IMAGE", Value: defaultHelperImage},
								{Name: "OPENEBS_IO_HELPER_POD_HOST_NETWORK", Value: "false"},
								{Name: "OPENEBS_IO_INSTALLER_TYPE", Value: "openebs-operator-helperpod"},
								{Name: "OPENEBS_IO_HELPER_POD_TIMEOUT_SECS", Value: "120"},
								{Name: "LEADER_ELECTION_ENABLED", Value: "true"},
							},
						},
					},
				},
			},
		},
	}
}

func hostpathStorageClass(name string, cfg *storagev1alpha1.HostpathConfig) *storagev1.StorageClass {
	deletePolicy := corev1.PersistentVolumeReclaimDelete
	allowExpansion := true
	basePath := "/var/openebs/local"
	if cfg.BasePath != "" {
		basePath = cfg.BasePath
	}
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels("hostpath-sc"),
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": boolToStr(cfg.IsDefaultClass),
				"openebs.io/cas-type":                         "local",
				"cas.openebs.io/config": "- name: StorageType\n" +
					"  value: hostpath\n" +
					"- name: BasePath\n" +
					"  value: " + basePath,
			},
		},
		Provisioner:          "openebs.io/local",
		ReclaimPolicy:        &deletePolicy,
		VolumeBindingMode:    &volumeBindingWaitForFirstConsumer,
		AllowVolumeExpansion: &allowExpansion,
		Parameters: map[string]string{
			"storage":  "hostpath",
			"BasePath": basePath,
		},
	}
}

// ---- ZFS Resources ----

func zfsServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-zfs-controller", Namespace: openebsNamespace, Labels: labels("zfs-rbac")},
	}
}

func zfsClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-zfs-role"},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"namespaces"}, Verbs: []string{"*"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumes", "services"}, Verbs: []string{"get", "list", "watch", "create", "delete", "update", "patch"}},
			{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumeclaims"}, Verbs: []string{"get", "list", "watch", "update"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumeclaims/status"}, Verbs: []string{"patch", "update"}},
			{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list", "watch", "update", "patch"}},
			{APIGroups: []string{""}, Resources: []string{"events"}, Verbs: []string{"get", "list", "watch", "create", "update", "patch"}},
			{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get", "list"}},
			{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"storageclasses", "volumeattachments", "csinodes", "volumeattributesclasses"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"csistoragecapacities"}, Verbs: []string{"*"}},
			{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, Verbs: []string{"get", "watch", "list", "delete", "update", "create"}},
			{APIGroups: []string{"zfs.openebs.io"}, Resources: []string{"*"}, Verbs: []string{"*"}},
			{APIGroups: []string{"snapshot.storage.k8s.io"}, Resources: []string{"volumesnapshotclasses"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"snapshot.storage.k8s.io"}, Resources: []string{"volumesnapshotcontents", "volumesnapshots"}, Verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
			{APIGroups: []string{"snapshot.storage.k8s.io"}, Resources: []string{"volumesnapshotcontents/status", "volumesnapshots/status"}, Verbs: []string{"update", "patch"}},
			{APIGroups: []string{"apiextensions.k8s.io"}, Resources: []string{"customresourcedefinitions"}, Verbs: []string{"create", "list", "watch", "delete"}},
		},
	}
}

func zfsClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-zfs-binding", Labels: labels("zfs-rbac")},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "openebs-zfs-role"},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "openebs-zfs-controller", Namespace: openebsNamespace}},
	}
}

func zfsControllerDeployment(instance *storagev1alpha1.OpenEBS) *appsv1.Deployment {
	csiProvisionerImg, csiResizerImg, zfsPluginImg := defaultCSIProvisioner, defaultCSIResizer, defaultZFSImage
	if instance.Spec.Images != nil {
		csiProvisionerImg = resolveImage(instance.Spec.Images.CSIProvisioner, defaultCSIProvisioner)
		csiResizerImg = resolveImage(instance.Spec.Images.CSIResizer, defaultCSIResizer)
		zfsPluginImg = resolveImage(instance.Spec.Images.ZFS, defaultZFSImage)
	}

	labels := labels("zfs-controller")
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zfsControllerName,
			Namespace: openebsNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName: "openebs-zfs-controller",
					Containers: []corev1.Container{
						{
							Name:  "csi-provisioner",
							Image: csiProvisionerImg,
							Args:  []string{"--csi-address=$(ADDRESS)", "--v=2", "--timeout=150s", "--leader-election", "--leader-election-lease-duration=120s", "--leader-election-renew-deadline=80s", "--leader-election-retry-period=30s", "--extra-create-metadata=true", "--default-fstype=ext4"},
							Env:   []corev1.EnvVar{{Name: "ADDRESS", Value: "/var/lib/csi/sockets/pluginproxy/csi.sock"}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: "/var/lib/csi/sockets/pluginproxy/"},
							},
						},
						{
							Name:  "csi-resizer",
							Image: csiResizerImg,
							Args:  []string{"--csi-address=$(ADDRESS)", "--v=2", "--timeout=150s", "--leader-election", "--leader-election-lease-duration=120s", "--leader-election-renew-deadline=80s", "--leader-election-retry-period=30s"},
							Env:   []corev1.EnvVar{{Name: "ADDRESS", Value: "/var/lib/csi/sockets/pluginproxy/csi.sock"}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: "/var/lib/csi/sockets/pluginproxy/"},
							},
						},
						{
							Name:  "zfs-plugin",
							Image: zfsPluginImg,
							Args:  []string{"--endpoint=$(OPENEBS_CSI_ENDPOINT)", "--plugin=$(OPENEBS_CONTROLLER_DRIVER)"},
							Env: []corev1.EnvVar{
								{Name: "OPENEBS_NAMESPACE", Value: openebsNamespace},
								{Name: "OPENEBS_CONTROLLER_DRIVER", Value: "controller"},
								{Name: "OPENEBS_CSI_ENDPOINT", Value: "unix:///var/lib/csi/sockets/pluginproxy/csi.sock"},
								{Name: "OPENEBS_IO_INSTALLER_TYPE", Value: "openebs-operator"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: "/var/lib/csi/sockets/pluginproxy/"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "socket-dir", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
					},
				},
			},
		},
	}
}

func zfsNodeDaemonSet(instance *storagev1alpha1.OpenEBS) *appsv1.DaemonSet {
	csiNodeRegistrarImg, zfsPluginImg := defaultCSINodeRegistrar, defaultZFSImage
	if instance.Spec.Images != nil {
		csiNodeRegistrarImg = resolveImage(instance.Spec.Images.CSINodeRegistrar, defaultCSINodeRegistrar)
		zfsPluginImg = resolveImage(instance.Spec.Images.ZFS, defaultZFSImage)
	}

	labels := labels("zfs-node")
	maxUnavailable := intstr.FromString("100%")
	binMode := int32(0555)
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zfsNodeName,
			Namespace: openebsNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName: "openebs-zfs-controller",
					HostNetwork:        true,
					Containers: []corev1.Container{
						{
							Name:  "csi-node-driver-registrar",
							Image: csiNodeRegistrarImg,
							Args: []string{
								"--v=5",
								"--csi-address=$(ADDRESS)",
								"--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)",
							},
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"/bin/sh", "-c", "rm -rf /registration/zfs-localpv /registration/zfs-localpv-reg.sock"},
									},
								},
							},
							Env: []corev1.EnvVar{
								{Name: "ADDRESS", Value: "/plugin/csi.sock"},
								{Name: "DRIVER_REG_SOCK_PATH", Value: "/var/lib/kubelet/plugins/zfs-localpv/csi.sock"},
								{Name: "KUBE_NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "plugin-dir", MountPath: "/plugin"},
								{Name: "registration-dir", MountPath: "/registration"},
							},
						},
						{
							Name:  "zfs-node-plugin",
							Image: zfsPluginImg,
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"SYS_ADMIN"},
								},
								AllowPrivilegeEscalation: boolPtr(true),
							},
							Args: []string{"--nodename=$(OPENEBS_NODE_NAME)", "--endpoint=$(OPENEBS_CSI_ENDPOINT)", "--plugin=$(OPENEBS_NODE_DRIVER)"},
							Env: []corev1.EnvVar{
								{Name: "OPENEBS_NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
								{Name: "OPENEBS_NODE_DRIVER", Value: "agent"},
								{Name: "OPENEBS_NAMESPACE", Value: openebsNamespace},
								{Name: "OPENEBS_CSI_ENDPOINT", Value: "unix:///plugin/csi.sock"},
								{Name: "ALLOWED_TOPOLOGIES", Value: "All"},
								{Name: "OPENEBS_IO_INSTALLER_TYPE", Value: "openebs-operator"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "plugin-dir", MountPath: "/plugin"},
								{Name: "device-dir", MountPath: "/dev"},
								{Name: "encr-keys", MountPath: "/home/keys"},
								{Name: "chroot-zfs", MountPath: "/sbin/zfs", SubPath: "zfs"},
								{Name: "host-root", MountPath: "/host", MountPropagation: &hostpathMountPropagationHostToContainer, ReadOnly: true},
								{Name: "pods-mount-dir", MountPath: "/var/lib/kubelet", MountPropagation: &hostpathMountPropagation},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "plugin-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/zfs-localpv/", Type: &hostpathDirOrCreate}}},
						{Name: "registration-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins_registry/", Type: &hostpathDirOrCreate}}},
						{Name: "device-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/dev", Type: &hostpathDir}}},
						{Name: "encr-keys", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/home/keys", Type: &hostpathDirOrCreate}}},
						{Name: "chroot-zfs", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "openebs-zfspv-bin"}, DefaultMode: &binMode}}},
						{Name: "host-root", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/", Type: &hostpathDir}}},
						{Name: "pods-mount-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet", Type: &hostpathDir}}},
					},
				},
			},
		},
	}
}

func zfsBinConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-zfspv-bin", Namespace: openebsNamespace, Labels: labels("zfs-bin")},
		Data: map[string]string{
			"zfs": `#!/bin/sh
if [ -x /host/sbin/zfs ]; then
  chroot /host /sbin/zfs "$@"
elif [ -x /host/usr/sbin/zfs ]; then
  chroot /host /usr/sbin/zfs "$@"
else
  chroot /host "zfs" "$@"
fi
`,
		},
	}
}

func zfsCSIDriver() *storagev1.CSIDriver {
	attachRequired := true
	podInfoOnMount := true
	return &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: zfsCSIDriverName, Labels: labels("zfs-csidriver")},
		Spec: storagev1.CSIDriverSpec{
			AttachRequired: &attachRequired,
			PodInfoOnMount: &podInfoOnMount,
			FSGroupPolicy:  &fsGroupPolicyFile,
		},
	}
}

func zfsStorageClass(name string, cfg *storagev1alpha1.ZFSConfig) *storagev1.StorageClass {
	poolName := "zfspool"
	if cfg.PoolName != "" {
		poolName = cfg.PoolName
	}
	deletePolicy := corev1.PersistentVolumeReclaimDelete
	allowExpansion := true
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels("zfs-sc"),
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": boolToStr(cfg.IsDefaultClass),
			},
		},
		Provisioner:          zfsCSIDriverName,
		ReclaimPolicy:        &deletePolicy,
		VolumeBindingMode:    &volumeBindingWaitForFirstConsumer,
		AllowVolumeExpansion: &allowExpansion,
		Parameters: map[string]string{
			"poolname": poolName,
			"fstype":   "zfs",
		},
	}
}

// ---- Rawfile Resources ----

func rawfileServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-rawfile-provisioner", Namespace: openebsNamespace, Labels: labels("rawfile-rbac")},
	}
}

func rawfileClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-rawfile-provisioner", Labels: labels("rawfile-rbac")},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{""}, Resources: []string{"namespaces", "pods", "events", "endpoints"}, Verbs: []string{"*"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumes", "persistentvolumeclaims"}, Verbs: []string{"*"}},
			{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get", "create", "update"}},
			{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"storageclasses"}, Verbs: []string{"*"}},
			{APIGroups: []string{"apiextensions.k8s.io"}, Resources: []string{"customresourcedefinitions"}, Verbs: []string{"get", "list", "create", "update", "delete", "patch"}},
			{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, Verbs: []string{"get", "create", "update"}},
		},
	}
}

func rawfileClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-rawfile-provisioner", Labels: labels("rawfile-rbac")},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "openebs-rawfile-provisioner"},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "openebs-rawfile-provisioner", Namespace: openebsNamespace}},
	}
}

func rawfileDeployment(instance *storagev1alpha1.OpenEBS) *appsv1.Deployment {
	rawfileImg := defaultRawfileImage
	if instance.Spec.Images != nil {
		rawfileImg = resolveImage(instance.Spec.Images.Rawfile, defaultRawfileImage)
	}

	labels := labels("rawfile-provisioner")
	replicas := int32(1)
	basePath := "/var/openebs/rawfile"
	if instance.Spec.Rawfile != nil && instance.Spec.Rawfile.BasePath != "" {
		basePath = instance.Spec.Rawfile.BasePath
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rawfileDeployName,
			Namespace: openebsNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName: "openebs-rawfile-provisioner",
					Containers: []corev1.Container{
						{
							Name:  "rawfile-provisioner",
							Image: rawfileImg,
							Env: []corev1.EnvVar{
								{Name: "NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
								{Name: "OPENEBS_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
								{Name: "OPENEBS_SERVICE_ACCOUNT", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.serviceAccountName"}}},
								{Name: "OPENEBS_IO_INSTANCE_ID", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.uid"}}},
								{Name: "OPENEBS_IO_ENABLE_ANALYTICS", Value: "false"},
								{Name: "OPENEBS_IO_BASE_PATH", Value: basePath},
							},
						},
					},
				},
			},
		},
	}
}

func rawfileStorageClass(name string, cfg *storagev1alpha1.RawfileConfig) *storagev1.StorageClass {
	deletePolicy := corev1.PersistentVolumeReclaimDelete
	allowExpansion := true
	basePath := "/var/openebs/rawfile"
	if cfg.BasePath != "" {
		basePath = cfg.BasePath
	}
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels("rawfile-sc"),
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": boolToStr(cfg.IsDefaultClass),
			},
		},
		Provisioner:          "rawfile.csi.openebs.io",
		ReclaimPolicy:        &deletePolicy,
		VolumeBindingMode:    &volumeBindingWaitForFirstConsumer,
		AllowVolumeExpansion: &allowExpansion,
		Parameters: map[string]string{
			"basePath": basePath,
		},
	}
}

// ---- Mayastor Resources ----

func mayastorImage(tag, component string) string {
	return "openebs/mayastor-" + component + ":" + tag
}

func mayastorServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: mayastorServiceAccountName, Namespace: mayastorNamespace, Labels: labels("mayastor-rbac")},
	}
}

func mayastorClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: mayastorClusterRoleName, Labels: labels("mayastor-rbac")},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"pods", "nodes", "services", "persistentvolumes", "persistentvolumeclaims", "events", "configmaps", "secrets", "endpoints"}, Verbs: []string{"*"}},
			{APIGroups: []string{""}, Resources: []string{"namespaces", "resourcequotas", "limitranges"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"apps"}, Resources: []string{"deployments", "daemonsets", "statefulsets"}, Verbs: []string{"*"}},
			{APIGroups: []string{"batch"}, Resources: []string{"jobs"}, Verbs: []string{"*"}},
			{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"storageclasses", "volumeattachments", "csinodes", "volumeattributesclasses"}, Verbs: []string{"*"}},
			{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, Verbs: []string{"*"}},
			{APIGroups: []string{"apiextensions.k8s.io"}, Resources: []string{"customresourcedefinitions"}, Verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
			{APIGroups: []string{"apiextensions.k8s.io"}, Resources: []string{"customresourcedefinitions/status"}, Verbs: []string{"get", "patch", "update"}},
			{APIGroups: []string{"openebs.io"}, Resources: []string{"*"}, Verbs: []string{"*"}},
			{APIGroups: []string{"snapshot.storage.k8s.io"}, Resources: []string{"volumesnapshotclasses", "volumesnapshotcontents", "volumesnapshots"}, Verbs: []string{"*"}},
			{APIGroups: []string{"snapshot.storage.k8s.io"}, Resources: []string{"volumesnapshotcontents/status", "volumesnapshots/status"}, Verbs: []string{"update", "patch"}},
			{APIGroups: []string{"mayastor.openebs.io"}, Resources: []string{"*"}, Verbs: []string{"*"}},
			{APIGroups: []string{"monitoring.coreos.com"}, Resources: []string{"servicemonitors"}, Verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
		},
	}
}

func mayastorClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: mayastorClusterRoleBindingName, Labels: labels("mayastor-rbac")},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: mayastorClusterRoleName},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: mayastorServiceAccountName, Namespace: mayastorNamespace}},
	}
}

func mayastorEtcdStatefulSet(instance *storagev1alpha1.OpenEBS) *appsv1.StatefulSet {
	etcdImg := defaultEtcdImage
	if instance.Spec.Images != nil {
		etcdImg = resolveImage(instance.Spec.Images.Etcd, defaultEtcdImage)
	}

	replicas := int32(1)
	storageSize := "10Gi"
	storageClassName := ""
	if instance.Spec.Mayastor != nil {
		if instance.Spec.Mayastor.EtcdReplicaCount > 0 {
			replicas = int32(instance.Spec.Mayastor.EtcdReplicaCount)
		}
		if instance.Spec.Mayastor.EtcdStorageSize != "" {
			storageSize = instance.Spec.Mayastor.EtcdStorageSize
		}
		storageClassName = instance.Spec.Mayastor.EtcdStorageClassName
	}

	lbls := labels("mayastor-etcd")
	pvcLabels := map[string]string{"app": "etcd"}
	etcdUID := int64(1001)
	zero := int64(0)
	podAnnotations := map[string]string{}
	if instance.Spec.Mayastor != nil && instance.Spec.Mayastor.EtcdVeleroBackup {
		podAnnotations = map[string]string{
			"backup.velero.io/backup-volumes":     "data",
			"pre.hook.backup.velero.io/container": "etcd",
			"pre.hook.backup.velero.io/command":   `["/bin/sh","-c","etcdctl snapshot save /bitnami/etcd/velero-etcd-snapshot.db && sync"]`,
			"pre.hook.backup.velero.io/timeout":   "5m",
			"pre.hook.backup.velero.io/on-error":  "Fail",
		}
	}
	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(storageSize),
			},
		},
	}
	if storageClassName != "" {
		pvcSpec.StorageClassName = &storageClassName
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mayastorEtcdName,
			Namespace: mayastorNamespace,
			Labels:    lbls,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: mayastorEtcdServiceName,
			Replicas:    &replicas,
			Selector:    &metav1.LabelSelector{MatchLabels: lbls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: lbls, Annotations: podAnnotations},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: &etcdUID,
					},
					InitContainers: []corev1.Container{{
						Name:  "volume-permissions",
						Image: "openebs/alpine-bash:4.5.0",
						Command: []string{
							"sh", "-c", "chown -R 1001:1001 /bitnami/etcd",
						},
						SecurityContext: &corev1.SecurityContext{
							RunAsUser: &zero,
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "data", MountPath: "/bitnami/etcd"},
						},
					}},
					Containers: []corev1.Container{{
						Name:    "etcd",
						Image:   etcdImg,
						Command: []string{"etcd"},
						SecurityContext: &corev1.SecurityContext{
							RunAsUser: &etcdUID,
						},
						Args: []string{
							"--listen-client-urls=http://0.0.0.0:2379",
							"--advertise-client-urls=http://$(POD_NAME).mayastor-etcd:2379",
						},
						Env: []corev1.EnvVar{
							{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
							{Name: "ETCD_AUTO_COMPACTION_MODE", Value: "revision"},
							{Name: "ETCD_AUTO_COMPACTION_RETENTION", Value: "100"},
							{Name: "ETCD_DATA_DIR", Value: "/bitnami/etcd/data"},
							{Name: "ETCD_QUOTA_BACKEND_BYTES", Value: "8589934592"},
						},
						Ports: []corev1.ContainerPort{
							{Name: "client", ContainerPort: 2379},
							{Name: "peer", ContainerPort: 2380},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "data", MountPath: "/bitnami/etcd"},
						},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "data",
						Labels: pvcLabels,
					},
					Spec: pvcSpec,
				},
			},
		},
	}
}

func mayastorEtcdService() *corev1.Service {
	lbls := labels("mayastor-etcd")
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mayastorEtcdServiceName,
			Namespace: mayastorNamespace,
			Labels:    lbls,
		},
		Spec: corev1.ServiceSpec{
			Selector: lbls,
			Ports: []corev1.ServicePort{
				{Name: "client", Port: 2379},
			},
		},
	}
}

func mayastorEtcdVeleroSchedule(instance *storagev1alpha1.OpenEBS) *unstructured.Unstructured {
	ns := "velero"
	cron := ""
	if instance.Spec.Mayastor != nil {
		if instance.Spec.Mayastor.EtcdVeleroNamespace != "" {
			ns = instance.Spec.Mayastor.EtcdVeleroNamespace
		}
		cron = instance.Spec.Mayastor.EtcdVeleroSchedule
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "velero.io/v1",
			"kind":       "Schedule",
			"metadata": map[string]interface{}{
				"name":      mayastorEtcdName,
				"namespace": ns,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":       "openebs",
					"app.kubernetes.io/component":  "mayastor-etcd-velero",
					"app.kubernetes.io/managed-by": "openebs-operator",
				},
			},
			"spec": map[string]interface{}{
				"schedule": cron,
				"template": map[string]interface{}{
					"includedNamespaces": []interface{}{mayastorNamespace},
					"includedResources":  []interface{}{"statefulsets"},
					"labelSelector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app.kubernetes.io/component": "mayastor-etcd",
						},
					},
					"snapshotVolumes": false,
				},
			},
		},
	}
}

func mayastorAgentCoreDeployment(instance *storagev1alpha1.OpenEBS) *appsv1.Deployment {
	tag := defaultMayastorTag
	if instance.Spec.Images != nil && instance.Spec.Images.Mayastor != "" {
		tag = instance.Spec.Images.Mayastor
	}
	coreImg := mayastorImage(tag, "agent-core")
	haImg := mayastorImage(tag, "agent-ha-cluster")

	lbls := labels("mayastor-agent-core")
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mayastorAgentCoreName,
			Namespace: mayastorNamespace,
			Labels:    lbls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: lbls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: lbls},
				Spec: corev1.PodSpec{
					ServiceAccountName: mayastorServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "agent-core",
							Image: coreImg,
							Args: []string{
								"--store=http://mayastor-etcd:2379",
								"--request-timeout=5s",
								"--cache-period=30s",
								"--grpc-server-addr=[::]:50051",
								"--pool-commitment=250%",
								"--snapshot-commitment=40%",
								"--volume-commitment-initial=40%",
								"--volume-commitment=40%",
								"--fmt-style=pretty",
								"--ansi-colors=true",
								"--create-volume-limit=10",
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 50051},
							},
							Env: []corev1.EnvVar{
								{Name: "RUST_LOG", Value: "info"},
								{Name: "MY_POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
								{Name: "MY_POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("32Mi"),
								},
							},
						},
						{
							Name:  "agent-ha-cluster",
							Image: haImg,
							Args: []string{
								"-g=[::]:50052",
								"--store=http://mayastor-etcd:2379",
								"--core-grpc=https://" + mayastorAgentCoreName + ":50051",
								"--ansi-colors=true",
								"--fmt-style=pretty",
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 50052},
							},
							Env: []corev1.EnvVar{
								{Name: "RUST_LOG", Value: "info"},
								{Name: "MY_POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
								{Name: "MY_POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
						},
					},
				},
			},
		},
	}
}

func mayastorAgentCoreService() *corev1.Service {
	lbls := labels("mayastor-agent-core")
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mayastorAgentCoreServiceName,
			Namespace: mayastorNamespace,
			Labels:    lbls,
		},
		Spec: corev1.ServiceSpec{
			Selector: lbls,
			Ports: []corev1.ServicePort{
				{Name: "grpc", Port: 50051},
				{Name: "ha-cluster", Port: 50052},
			},
		},
	}
}

func mayastorAPIRestDeployment(instance *storagev1alpha1.OpenEBS) *appsv1.Deployment {
	tag := defaultMayastorTag
	if instance.Spec.Images != nil && instance.Spec.Images.Mayastor != "" {
		tag = instance.Spec.Images.Mayastor
	}
	img := mayastorImage(tag, "api-rest")

	lbls := labels("mayastor-api-rest")
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mayastorAPIRestName,
			Namespace: mayastorNamespace,
			Labels:    lbls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: lbls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: lbls},
				Spec: corev1.PodSpec{
					ServiceAccountName: mayastorServiceAccountName,
					Containers: []corev1.Container{{
						Name:  "api-rest",
						Image: img,
						Args: []string{
							"--dummy-certificates",
							"--http=[::]:8081",
							"--no-auth",
							"--request-timeout=5s",
							"--core-grpc=https://" + mayastorAgentCoreName + ":50051",
							"--ansi-colors=true",
							"--fmt-style=pretty",
							"--core-health-freq=20s",
						},
						Ports: []corev1.ContainerPort{
							{Name: "http", ContainerPort: 8080},
							{Name: "rest", ContainerPort: 8081},
						},
						Env: []corev1.EnvVar{
							{Name: "RUST_LOG", Value: "info"},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("32Mi"),
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/ready",
									Port: intstr.FromInt32(8081),
								},
							},
							FailureThreshold:    3,
							InitialDelaySeconds: 1,
							PeriodSeconds:       20,
							TimeoutSeconds:      5,
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/live",
									Port: intstr.FromInt32(8081),
								},
							},
							FailureThreshold:    3,
							InitialDelaySeconds: 1,
							PeriodSeconds:       30,
							TimeoutSeconds:      5,
						},
					}},
				},
			},
		},
	}
}

func mayastorAPIRestService() *corev1.Service {
	lbls := labels("mayastor-api-rest")
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mayastorAPIRestServiceName,
			Namespace: mayastorNamespace,
			Labels:    lbls,
		},
		Spec: corev1.ServiceSpec{
			Selector: lbls,
			Ports: []corev1.ServicePort{
				{Name: "https", Port: 8080},
				{Name: "http", Port: 8081},
			},
		},
	}
}

func mayastorCSIControllerDeployment(instance *storagev1alpha1.OpenEBS) *appsv1.Deployment {
	csiProvisionerImg, csiAttacherImg, csiSnapshotterImg, csiSnapshotControllerImg, csiResizerImg := defaultCSIProvisioner, defaultCSIAttacher, defaultCSISnapshotter, defaultCSISnapshotController, defaultCSIResizer
	mayastorTag := defaultMayastorTag
	if instance.Spec.Images != nil {
		csiProvisionerImg = resolveImage(instance.Spec.Images.CSIProvisioner, defaultCSIProvisioner)
		csiAttacherImg = resolveImage(instance.Spec.Images.CSIAttacher, defaultCSIAttacher)
		csiSnapshotterImg = resolveImage(instance.Spec.Images.CSISnapshotter, defaultCSISnapshotter)
		csiSnapshotControllerImg = resolveImage(instance.Spec.Images.CSISnapshotController, defaultCSISnapshotController)
		csiResizerImg = resolveImage(instance.Spec.Images.CSIResizer, defaultCSIResizer)
		if instance.Spec.Images.Mayastor != "" {
			mayastorTag = instance.Spec.Images.Mayastor
		}
	}
	csiControllerImg := mayastorImage(mayastorTag, "csi-controller")

	lbls := labels("mayastor-csi-controller")
	replicas := int32(1)
	socketDir := "/var/lib/csi/sockets/pluginproxy"
	socketPath := socketDir + "/csi.sock"
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mayastorCSIControllerName,
			Namespace: mayastorNamespace,
			Labels:    lbls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: lbls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: lbls},
				Spec: corev1.PodSpec{
					HostNetwork:        true,
					DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
					ServiceAccountName: mayastorServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "csi-provisioner",
							Image: csiProvisionerImg,
							Args: []string{
								"--csi-address=$(ADDRESS)",
								"--v=2",
								"--feature-gates=Topology=true",
								"--strict-topology=false",
								"--default-fstype=ext4",
								"--extra-create-metadata",
								"--timeout=36s",
								"--worker-threads=10",
								"--prevent-volume-mode-conversion",
							},
							Env: []corev1.EnvVar{{Name: "ADDRESS", Value: socketPath}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: socketDir},
							},
						},
						{
							Name:  "csi-attacher",
							Image: csiAttacherImg,
							Args: []string{
								"--csi-address=$(ADDRESS)",
								"--v=2",
								"--timeout=36s",
							},
							Env: []corev1.EnvVar{{Name: "ADDRESS", Value: socketPath}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: socketDir},
							},
						},
						{
							Name:  "csi-snapshotter",
							Image: csiSnapshotterImg,
							Args: []string{
								"--csi-address=$(ADDRESS)",
								"--v=2",
							},
							Env: []corev1.EnvVar{{Name: "ADDRESS", Value: socketPath}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: socketDir},
							},
						},
						{
							Name:  "csi-snapshot-controller",
							Image: csiSnapshotControllerImg,
							Args: []string{
								"--v=2",
								"--leader-election=false",
								"--prevent-volume-mode-conversion",
							},
						},
						{
							Name:  "csi-resizer",
							Image: csiResizerImg,
							Args: []string{
								"--csi-address=$(ADDRESS)",
								"--v=2",
								"--handle-volume-inuse-error=false",
							},
							Env: []corev1.EnvVar{{Name: "ADDRESS", Value: socketPath}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: socketDir},
							},
						},
						{
							Name:  "mayastor-csi-controller",
							Image: csiControllerImg,
							Args: []string{
								"--csi-socket=" + socketPath,
								"--rest-endpoint=http://mayastor-api-rest:8081",
								"--node-selector=openebs.io/csi-node=mayastor",
								"--create-volume-limit=10",
								"--enable-orphan-vol-gc=false",
								"--ansi-colors=true",
								"--fmt-style=pretty",
							},
							Env: []corev1.EnvVar{
								{Name: "RUST_LOG", Value: "info"},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("32m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("16m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: socketDir},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "socket-dir", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
					},
				},
			},
		},
	}
}

func ioEngineArgs(instance *storagev1alpha1.OpenEBS) []string {
	args := []string{
		"--grpc-ip=$(MY_POD_IP)",
		"--grpc-port=10124",
		"-N$(MY_NODE_NAME)",
		"-Rhttps://" + mayastorAgentCoreName + ":50051",
		"-y/var/local/mayastor/io-engine/config.yaml",
		"-l1,2",
		"-p=mayastor-etcd:2379",
		"--ptpl-dir=/var/local/mayastor/io-engine/ptpl/",
		"--api-versions=v1",
	}
	if instance.Spec.Mayastor != nil && instance.Spec.Mayastor.IoEngineEnvContext != "" {
		args = append(args, "--env-context=--"+instance.Spec.Mayastor.IoEngineEnvContext)
	}
	args = append(args,
		"--tgt-crdt=30",
		"--ps-retries=300",
		"--pool-io-error-threshold=64",
		"--pool-io-stall-deadline=110s110s",
		"--pool-io-stall-transition-threshold=3",
		"--pool-io-stall-transition-window=3h",
	)
	return args
}

func mayastorIOEngineDaemonSet(instance *storagev1alpha1.OpenEBS) *appsv1.DaemonSet {
	tag := defaultMayastorTag
	if instance.Spec.Images != nil && instance.Spec.Images.Mayastor != "" {
		tag = instance.Spec.Images.Mayastor
	}
	ioEngineImg := mayastorImage(tag, "io-engine")

	lbls := labels("mayastor-io-engine")
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mayastorIOEngineName,
			Namespace: mayastorNamespace,
			Labels:    lbls,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector:        &metav1.LabelSelector{MatchLabels: lbls},
			MinReadySeconds: 10,
			UpdateStrategy:  appsv1.DaemonSetUpdateStrategy{Type: appsv1.OnDeleteDaemonSetStrategyType},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: lbls},
				Spec: corev1.PodSpec{
					ServiceAccountName: mayastorServiceAccountName,
					NodeSelector: map[string]string{
						"openebs.io/engine":  "mayastor",
						"kubernetes.io/arch": "amd64",
					},
					HostNetwork: true,
					DNSPolicy:   corev1.DNSClusterFirstWithHostNet,
					Containers: []corev1.Container{{
						Name:            "io-engine",
						Image:           ioEngineImg,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"io-engine"},
						SecurityContext: &corev1.SecurityContext{
							Privileged: boolPtr(true),
						},
						Args: ioEngineArgs(instance),
						Env: []corev1.EnvVar{
							{Name: "RUST_LOG", Value: "info"},
							{Name: "NVMF_TCP_NUM_SHARED_BUF", Value: "2047"},
							{Name: "NVMF_TCP_BUF_CACHE_SIZE", Value: "64"},
							{Name: "NVMF_TCP_MAX_QPAIRS_PER_CTRL", Value: "32"},
							{Name: "NVMF_TCP_MAX_QUEUE_DEPTH", Value: "32"},
							{Name: "NVMF_TCP_IN_CAPSULE_DATA_SIZE", Value: "4096"},
							{Name: "NVMF_TCP_MAX_IO_SIZE", Value: "131072"},
							{Name: "NVMF_TCP_IO_UNIT_SIZE", Value: "131072"},
							{Name: "NVME_TIMEOUT", Value: "110s"},
							{Name: "NVME_TIMEOUT_ADMIN", Value: "30s"},
							{Name: "NVME_KATO", Value: "10s"},
							{Name: "NVME_MAX_NAMESPACES", Value: "4096"},
							{Name: "MY_NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
							{Name: "MY_POD_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}},
							{Name: "NEXUS_NVMF_ANA_ENABLE", Value: "1"},
							{Name: "NEXUS_NVMF_RESV_ENABLE", Value: "1"},
							{Name: "POOL_HANDLE_RESCAN_PERIOD", Value: "5m"},
						},
						Ports: []corev1.ContainerPort{
							{Name: "io-engine", ContainerPort: 10124, Protocol: corev1.ProtocolTCP},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:                   resource.MustParse("2"),
								corev1.ResourceMemory:                resource.MustParse("1Gi"),
								corev1.ResourceName("hugepages-2Mi"): resource.MustParse("2Gi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:                   resource.MustParse("2"),
								corev1.ResourceMemory:                resource.MustParse("1Gi"),
								corev1.ResourceName("hugepages-2Mi"): resource.MustParse("2Gi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "device", MountPath: "/dev"},
							{Name: "udev", MountPath: "/run/udev"},
							{Name: "dshm", MountPath: "/dev/shm"},
							{Name: "configlocation", MountPath: "/var/local/mayastor/io-engine/"},
							{Name: "hugepages-2mi", MountPath: "/dev/hugepages-2mi"},
						},
					}, {
						Name:  "metrics-exporter-io-engine",
						Image: mayastorImage(tag, "metrics-exporter-io-engine"),
						Args: []string{
							"--metrics-endpoint=[::]:9502",
							"--grpc-port=10124",
							"--fmt-style=pretty",
							"--ansi-colors=true",
							"--rest-endpoint=http://" + mayastorAPIRestName + ":8081",
						},
						Env: []corev1.EnvVar{
							{Name: "MY_NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
							{Name: "MY_POD_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}},
						},
						Ports: []corev1.ContainerPort{
							{Name: "metrics", ContainerPort: 9502, Protocol: corev1.ProtocolTCP},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "device", MountPath: "/dev"},
							{Name: "udev", MountPath: "/run/udev"},
							{Name: "dshm", MountPath: "/dev/shm"},
							{Name: "hugepages-2mi", MountPath: "/dev/hugepages-2mi"},
							{Name: "configlocation", MountPath: "/var/local/mayastor/io-engine/"},
						},
					}},
					Volumes: []corev1.Volume{
						{Name: "device", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/dev", Type: &hostpathDir}}},
						{Name: "udev", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/run/udev", Type: &hostpathDir}}},
						{Name: "dshm", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory, SizeLimit: resource.NewQuantity(1<<30, resource.BinarySI)}}},
						{Name: "hugepages-2mi", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumHugePages}}},
						{Name: "configlocation", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/local/mayastor/io-engine/", Type: &hostpathDirOrCreate}}},
					},
				},
			},
		},
	}
}

func mayastorMetricsExporterService() *corev1.Service {
	lbls := labels("mayastor-io-engine")
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mayastorMetricsExporterSvcName,
			Namespace: mayastorNamespace,
			Labels:    lbls,
		},
		Spec: corev1.ServiceSpec{
			Selector: lbls,
			Ports: []corev1.ServicePort{
				{Name: "metrics", Port: 9502, TargetPort: intstr.FromInt32(9502)},
			},
		},
	}
}

func mayastorCSINodeDaemonSet(instance *storagev1alpha1.OpenEBS) *appsv1.DaemonSet {
	csiRegistrarImg := defaultCSINodeRegistrar
	mayastorTag := defaultMayastorTag
	if instance.Spec.Images != nil {
		csiRegistrarImg = resolveImage(instance.Spec.Images.CSINodeRegistrar, defaultCSINodeRegistrar)
		if instance.Spec.Images.Mayastor != "" {
			mayastorTag = instance.Spec.Images.Mayastor
		}
	}
	csiNodeImg := mayastorImage(mayastorTag, "csi-node")

	lbls := labels("mayastor-csi-node")
	kubeletPluginDir := "/var/lib/kubelet/plugins/io.openebs.mayastor"
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mayastorCSINodeName,
			Namespace: mayastorNamespace,
			Labels:    lbls,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector:        &metav1.LabelSelector{MatchLabels: lbls},
			MinReadySeconds: 10,
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type:          appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{MaxUnavailable: &intOrString1},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: lbls},
				Spec: corev1.PodSpec{
					ServiceAccountName: mayastorServiceAccountName,
					HostNetwork:        true,
					DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
					NodeSelector:       map[string]string{"openebs.io/csi-node": "mayastor"},
					Containers: []corev1.Container{
						{
							Name:  "csi-driver-registrar",
							Image: csiRegistrarImg,
							Args: []string{
								"--csi-address=/csi/csi.sock",
								"--kubelet-registration-path=" + kubeletPluginDir + "/csi.sock",
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "plugin-dir", MountPath: "/csi"},
								{Name: "registration-dir", MountPath: "/registration"},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							Ports: []corev1.ContainerPort{
								{Name: "mayastor-node", ContainerPort: 10199, Protocol: corev1.ProtocolTCP},
							},
						},
						{
							Name:  "csi-node",
							Image: csiNodeImg,
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
							},
							Args: []string{
								"--csi-socket=/csi/csi.sock",
								"--node-name=$(MY_NODE_NAME)",
								"--rest-endpoint=http://mayastor-api-rest:8081",
								"--enable-rest",
								"--enable-registration",
								"--grpc-ip=$(MY_POD_IP)",
								"--grpc-port=10199",
								"--nvme-io-timeout=110s10s",
								"--nvme-core-io-timeout=110s10s",
								"--nvme-ctrl-loss-tmo=1980",
								"--nvme-nr-io-queues=2",
								"--nvme-connect-fallback=true",
								"--kubelet-path=/var/lib/kubelet",
								"--node-selector=openebs.io/csi-node=mayastor",
								"--fmt-style=pretty",
								"--ansi-colors=true",
							},
							Env: []corev1.EnvVar{
								{Name: "RUST_LOG", Value: "info"},
								{Name: "MY_NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
								{Name: "MY_POD_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}},
								{Name: "RUST_BACKTRACE", Value: "1"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "device", MountPath: "/dev"},
								{Name: "sys", MountPath: "/sys"},
								{Name: "run-udev", MountPath: "/run/udev"},
								{Name: "plugin-dir", MountPath: "/csi"},
								{Name: "kubelet-dir", MountPath: "/var/lib/kubelet", MountPropagation: &hostpathMountPropagation},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "device", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/dev"}}},
						{Name: "sys", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/sys"}}},
						{Name: "run-udev", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/run/udev"}}},
						{Name: "registration-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins_registry"}}},
						{Name: "plugin-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: kubeletPluginDir, Type: &hostpathDirOrCreate}}},
						{Name: "kubelet-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet"}}},
					},
				},
			},
		},
	}
}

func mayastorOperatorDiskpoolDeployment(instance *storagev1alpha1.OpenEBS) *appsv1.Deployment {
	tag := defaultMayastorTag
	if instance.Spec.Images != nil && instance.Spec.Images.Mayastor != "" {
		tag = instance.Spec.Images.Mayastor
	}
	img := mayastorImage(tag, "operator-diskpool")

	lbls := labels("mayastor-operator-diskpool")
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mayastorDiskpoolName,
			Namespace: mayastorNamespace,
			Labels:    lbls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: lbls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: lbls},
				Spec: corev1.PodSpec{
					ServiceAccountName: mayastorServiceAccountName,
					Containers: []corev1.Container{{
						Name:  "operator-diskpool",
						Image: img,
						Args: []string{
							"-e http://" + mayastorAPIRestName + ":8081",
							"-n" + mayastorNamespace,
							"--request-timeout=5s",
							"--interval=30s",
							"--ansi-colors=true",
							"--fmt-style=pretty",
						},
						Env: []corev1.EnvVar{
							{Name: "RUST_LOG", Value: "info"},
							{Name: "MY_POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("32Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("16Mi"),
							},
						},
					}},
				},
			},
		},
	}
}

func mayastorHANodeDaemonSet(instance *storagev1alpha1.OpenEBS) *appsv1.DaemonSet {
	tag := defaultMayastorTag
	if instance.Spec.Images != nil && instance.Spec.Images.Mayastor != "" {
		tag = instance.Spec.Images.Mayastor
	}
	img := mayastorImage(tag, "agent-ha-node")

	lbls := labels("mayastor-ha-node")
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mayastorHANodeName,
			Namespace: mayastorNamespace,
			Labels:    lbls,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector:        &metav1.LabelSelector{MatchLabels: lbls},
			MinReadySeconds: 10,
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type:          appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{MaxUnavailable: &intOrString1},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: lbls},
				Spec: corev1.PodSpec{
					ServiceAccountName: mayastorServiceAccountName,
					HostNetwork:        true,
					DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
					Containers: []corev1.Container{{
						Name:  "agent-ha-node",
						Image: img,
						SecurityContext: &corev1.SecurityContext{
							Privileged: boolPtr(true),
						},
						Args: []string{
							"--node-name=$(MY_NODE_NAME)",
							"--csi-socket=/csi/csi.sock",
							"--grpc-ip=$(MY_POD_IP)",
							"--grpc-port=50053",
							"--cluster-agent=https://" + mayastorAgentCoreName + ":50052",
							"--ansi-colors=true",
							"--fmt-style=pretty",
						},
						Env: []corev1.EnvVar{
							{Name: "RUST_LOG", Value: "info"},
							{Name: "MY_NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
							{Name: "MY_POD_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}},
							{Name: "RUST_BACKTRACE", Value: "1"},
						},
						Ports: []corev1.ContainerPort{
							{Name: "ha-node", ContainerPort: 50053, Protocol: corev1.ProtocolTCP},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "device", MountPath: "/dev"},
							{Name: "sys", MountPath: "/sys"},
							{Name: "run-udev", MountPath: "/run/udev"},
							{Name: "plugin-dir", MountPath: "/csi"},
						},
					}},
					Volumes: []corev1.Volume{
						{Name: "device", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/dev"}}},
						{Name: "sys", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/sys"}}},
						{Name: "run-udev", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/run/udev"}}},
						{Name: "plugin-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/io.openebs.mayastor", Type: &hostpathDirOrCreate}}},
					},
				},
			},
		},
	}
}

func mayastorCSIDriver() *storagev1.CSIDriver {
	attachRequired := true
	podInfoOnMount := true
	return &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: mayastorCSIDriverName, Labels: labels("mayastor-csidriver")},
		Spec: storagev1.CSIDriverSpec{
			AttachRequired: &attachRequired,
			PodInfoOnMount: &podInfoOnMount,
			FSGroupPolicy:  &fsGroupPolicyFile,
		},
	}
}

func mayastorStorageClass(name string, instance *storagev1alpha1.OpenEBS) *storagev1.StorageClass {
	repl := "1"
	protocol := "nvmf"
	deletePolicy := corev1.PersistentVolumeReclaimDelete
	allowExpansion := true
	isDefault := false
	if instance.Spec.Mayastor != nil {
		isDefault = instance.Spec.Mayastor.IsDefaultClass
	}
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels("mayastor-sc"),
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": boolToStr(isDefault),
			},
		},
		Provisioner:          mayastorCSIDriverName,
		ReclaimPolicy:        &deletePolicy,
		VolumeBindingMode:    &volumeBindingWaitForFirstConsumer,
		AllowVolumeExpansion: &allowExpansion,
		Parameters: map[string]string{
			"repl":     repl,
			"protocol": protocol,
		},
	}
}

func mayastorVolumeSnapshotClass(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "snapshot.storage.k8s.io/v1",
			"kind":       "VolumeSnapshotClass",
			"metadata": map[string]interface{}{
				"name": name,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":       "mayastor-snapshot",
					"app.kubernetes.io/managed-by": "openebs-operator",
				},
			},
			"driver":         mayastorCSIDriverName,
			"deletionPolicy": "Delete",
		},
	}
}

var volumeBindingWaitForFirstConsumer = storagev1.VolumeBindingWaitForFirstConsumer

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
