package controller

import (
	storagev1alpha1 "github.com/aldershaab-it/openebs-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultLVMImage         = "openebs/lvm-driver:1.9.1"
	defaultHostpathImage    = "openebs/provisioner-localpv:4.5.0"
	defaultZFSImage         = "openebs/zfs-driver:2.10.1"
	defaultRawfileImage     = "openebs/rawfile-localpv:0.14.1"
	defaultCSIProvisioner   = "registry.k8s.io/sig-storage/csi-provisioner:v4.0.1"
	defaultCSIResizer       = "registry.k8s.io/sig-storage/csi-resizer:v1.10.1"
	defaultCSISnapshotter   = "registry.k8s.io/sig-storage/csi-snapshotter:v7.0.2"
	defaultCSINodeRegistrar = "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.10.1"
)

func resolveImage(override, defaultImage string) string {
	if override != "" {
		return override
	}
	return defaultImage
}

func boolPtr(b bool) *bool { return &b }

func int32Ptr(i int32) *int32 { return &i }

// ---- LVM Resources ----

func lvmServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-lvm-controller", Namespace: openebsNamespace},
	}
}

func lvmClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-lvm-role"},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"nodes", "persistentvolumes"}, Verbs: []string{"get", "list", "watch", "patch", "update"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumeclaims"}, Verbs: []string{"get", "list", "watch", "update"}},
			{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"storageclasses", "volumeattachments"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{""}, Resources: []string{"events"}, Verbs: []string{"create", "patch", "update"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumeclaims/status"}, Verbs: []string{"patch", "update"}},
		},
	}
}

func lvmClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-lvm-binding"},
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
							},
							Env: []corev1.EnvVar{{Name: "ADDRESS", Value: "/var/lib/csi/sockets/pluginproxy/csi.sock"}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: "/var/lib/csi/sockets/pluginproxy/"},
							},
						},
						{
							Name:  "lvm-plugin",
							Image: lvmPluginImg,
							Args:  []string{"--plugin-type=controller", "--endpoint=$(CSI_ENDPOINT)", "--nodeid=$(OPENEBS_NODE_ID)"},
							Env: []corev1.EnvVar{
								{Name: "OPENEBS_NAMESPACE", Value: openebsNamespace},
								{Name: "CSI_ENDPOINT", Value: "unix:///var/lib/csi/sockets/pluginproxy/csi.sock"},
								{Name: "OPENEBS_NODE_ID", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
								{Name: "OPENEBS_CSI_ENDPOINT", Value: "unix:///var/lib/csi/sockets/pluginproxy/csi.sock"},
								{Name: "OPENEBS_MONITOR_PERIOD", Value: "60"},
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
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lvmNodeName,
			Namespace: openebsNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					Containers: []corev1.Container{
						{
							Name:  "csi-node-driver-registrar",
							Image: csiNodeRegistrarImg,
							Args: []string{
								"--v=2",
								"--csi-address=$(ADDRESS)",
								"--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)",
							},
							Env: []corev1.EnvVar{
								{Name: "ADDRESS", Value: "/plugin/csi.sock"},
								{Name: "DRIVER_REG_SOCK_PATH", Value: "/var/lib/kubelet/plugins/local.csi.openebs.io/csi.sock"},
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
							Args: []string{"--plugin-type=node", "--endpoint=$(CSI_ENDPOINT)", "--nodeid=$(OPENEBS_NODE_ID)"},
							Env: []corev1.EnvVar{
								{Name: "OPENEBS_NODE_ID", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
								{Name: "OPENEBS_NAMESPACE", Value: openebsNamespace},
								{Name: "CSI_ENDPOINT", Value: "unix:///plugin/csi.sock"},
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
						{Name: "plugin-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/local.csi.openebs.io/"}}},
						{Name: "registration-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins_registry"}}},
						{Name: "device-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/dev"}}},
						{Name: "pods-mount-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet"}}},
						{Name: "node-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/openebs"}}},
					},
				},
			},
		},
	}
}

var hostpathMountPropagation = corev1.MountPropagationBidirectional

func lvmCSIDriver() *storagev1.CSIDriver {
	attachRequired := true
	podInfoOnMount := true
	return &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: lvmCSIDriverName},
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
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": boolToStr(cfg.IsDefaultClass),
			},
		},
		Provisioner:       lvmCSIDriverName,
		ReclaimPolicy:     &deletePolicy,
		VolumeBindingMode: &volumeBindingWaitForFirstConsumer,
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
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-localpv-provisioner", Namespace: openebsNamespace},
	}
}

func hostpathClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-localpv-provisioner"},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumes"}, Verbs: []string{"get", "list", "watch", "create", "delete", "update", "patch"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumeclaims"}, Verbs: []string{"get", "list", "watch", "update"}},
			{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"storageclasses"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{""}, Resources: []string{"events"}, Verbs: []string{"create", "update", "patch"}},
		},
	}
}

func hostpathClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-localpv-provisioner"},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "openebs-localpv-provisioner"},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "openebs-localpv-provisioner", Namespace: openebsNamespace}},
	}
}

func hostpathDeployment(instance *storagev1alpha1.OpenEBS) *appsv1.Deployment {
	hostpathImg := defaultHostpathImage
	if instance.Spec.Images != nil {
		hostpathImg = resolveImage(instance.Spec.Images.Hostpath, defaultHostpathImage)
	}

	labels := labels("hostpath-provisioner")
	replicas := int32(1)
	basePath := "/var/openebs/local"
	if instance.Spec.Hostpath != nil && instance.Spec.Hostpath.BasePath != "" {
		basePath = instance.Spec.Hostpath.BasePath
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostpathDeployName,
			Namespace: openebsNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName: "openebs-localpv-provisioner",
					Containers: []corev1.Container{
						{
							Name:  "provisioner",
							Image: hostpathImg,
							Env: []corev1.EnvVar{
								{Name: "NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
								{Name: "OPENEBS_NAMESPACE", Value: openebsNamespace},
								{Name: "OPENEBS_IO_INSTANCE_ID", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.uid"}}},
								{Name: "OPENEBS_IO_ENABLE_ANALYTICS", Value: "false"},
								{Name: "OPENEBS_IO_BASE_PATH", Value: basePath},
								{Name: "OPENEBS_IO_HELPER_IMAGE", Value: hostpathImg},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{Command: []string{"pgrep", "provisioner"}},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       60,
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
	basePath := "/var/openebs/local"
	if cfg.BasePath != "" {
		basePath = cfg.BasePath
	}
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": boolToStr(cfg.IsDefaultClass),
			},
		},
		Provisioner:       "openebs.io/local",
		ReclaimPolicy:     &deletePolicy,
		VolumeBindingMode: &volumeBindingWaitForFirstConsumer,
		Parameters: map[string]string{
			"storage":  "hostpath",
			"basePath": basePath,
		},
	}
}

// ---- ZFS Resources ----

func zfsServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-zfs-controller", Namespace: openebsNamespace},
	}
}

func zfsClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-zfs-role"},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"nodes", "persistentvolumes"}, Verbs: []string{"get", "list", "watch", "patch", "update"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumeclaims"}, Verbs: []string{"get", "list", "watch", "update"}},
			{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"storageclasses", "volumeattachments"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{""}, Resources: []string{"events"}, Verbs: []string{"create", "patch", "update"}},
		},
	}
}

func zfsClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-zfs-binding"},
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
							Args:  []string{"--csi-address=$(ADDRESS)", "--v=2"},
							Env:   []corev1.EnvVar{{Name: "ADDRESS", Value: "/var/lib/csi/sockets/pluginproxy/csi.sock"}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: "/var/lib/csi/sockets/pluginproxy/"},
							},
						},
						{
							Name:  "csi-resizer",
							Image: csiResizerImg,
							Args:  []string{"--csi-address=$(ADDRESS)", "--v=2"},
							Env:   []corev1.EnvVar{{Name: "ADDRESS", Value: "/var/lib/csi/sockets/pluginproxy/csi.sock"}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "socket-dir", MountPath: "/var/lib/csi/sockets/pluginproxy/"},
							},
						},
						{
							Name:  "zfs-plugin",
							Image: zfsPluginImg,
							Args:  []string{"--plugin-type=controller", "--endpoint=$(CSI_ENDPOINT)", "--nodeid=$(OPENEBS_NODE_ID)"},
							Env: []corev1.EnvVar{
								{Name: "OPENEBS_NAMESPACE", Value: openebsNamespace},
								{Name: "CSI_ENDPOINT", Value: "unix:///var/lib/csi/sockets/pluginproxy/csi.sock"},
								{Name: "OPENEBS_NODE_ID", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
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
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zfsNodeName,
			Namespace: openebsNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					Containers: []corev1.Container{
						{
							Name:  "csi-node-driver-registrar",
							Image: csiNodeRegistrarImg,
							Args: []string{
								"--v=2",
								"--csi-address=$(ADDRESS)",
								"--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)",
							},
							Env: []corev1.EnvVar{
								{Name: "ADDRESS", Value: "/plugin/csi.sock"},
								{Name: "DRIVER_REG_SOCK_PATH", Value: "/var/lib/kubelet/plugins/zfs.csi.openebs.io/csi.sock"},
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
							Args: []string{"--plugin-type=node", "--endpoint=$(CSI_ENDPOINT)", "--nodeid=$(OPENEBS_NODE_ID)"},
							Env: []corev1.EnvVar{
								{Name: "OPENEBS_NODE_ID", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
								{Name: "OPENEBS_NAMESPACE", Value: openebsNamespace},
								{Name: "CSI_ENDPOINT", Value: "unix:///plugin/csi.sock"},
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
						{Name: "plugin-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/zfs.csi.openebs.io/"}}},
						{Name: "registration-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins_registry"}}},
						{Name: "device-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/dev"}}},
						{Name: "pods-mount-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet"}}},
						{Name: "node-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/openebs"}}},
					},
				},
			},
		},
	}
}

func zfsCSIDriver() *storagev1.CSIDriver {
	attachRequired := true
	podInfoOnMount := true
	return &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: zfsCSIDriverName},
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
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Provisioner:       zfsCSIDriverName,
		ReclaimPolicy:     &deletePolicy,
		VolumeBindingMode: &volumeBindingWaitForFirstConsumer,
		Parameters: map[string]string{
			"poolname": poolName,
			"fstype":   "zfs",
		},
	}
}

// ---- Rawfile Resources ----

func rawfileServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-rawfile-provisioner", Namespace: openebsNamespace},
	}
}

func rawfileClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-rawfile-provisioner"},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumes"}, Verbs: []string{"get", "list", "watch", "create", "delete", "update", "patch"}},
			{APIGroups: []string{""}, Resources: []string{"persistentvolumeclaims"}, Verbs: []string{"get", "list", "watch", "update"}},
			{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"storageclasses"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{""}, Resources: []string{"events"}, Verbs: []string{"create", "update", "patch"}},
			{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list", "watch"}},
		},
	}
}

func rawfileClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "openebs-rawfile-provisioner"},
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
								{Name: "OPENEBS_NAMESPACE", Value: openebsNamespace},
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
	basePath := "/var/openebs/rawfile"
	if cfg.BasePath != "" {
		basePath = cfg.BasePath
	}
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Provisioner:       "rawfile.csi.openebs.io",
		ReclaimPolicy:     &deletePolicy,
		VolumeBindingMode: &volumeBindingWaitForFirstConsumer,
		Parameters: map[string]string{
			"basePath": basePath,
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
