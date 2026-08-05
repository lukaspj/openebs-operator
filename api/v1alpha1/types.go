// Package v1alpha1 contains API Schema definitions for the storage v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=storage.aldershaab-it.dk
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenEBSEngine represents a storage engine type.
// +kubebuilder:validation:Enum=hostpath;lvm;zfs;rawfile;mayastor
type OpenEBSEngine string

const (
	OpenEBSEngineHostpath OpenEBSEngine = "hostpath"
	OpenEBSEngineLVM      OpenEBSEngine = "lvm"
	OpenEBSEngineZFS      OpenEBSEngine = "zfs"
	OpenEBSEngineRawfile  OpenEBSEngine = "rawfile"
	OpenEBSEngineMayastor OpenEBSEngine = "mayastor"
)

// OpenEBSConditionType defines condition types for OpenEBS resources.
type OpenEBSConditionType string

const (
	ConditionAvailable   OpenEBSConditionType = "Available"
	ConditionProgressing OpenEBSConditionType = "Progressing"
	ConditionDegraded    OpenEBSConditionType = "Degraded"
	ConditionFailed      OpenEBSConditionType = "Failed"
)

// OpenEBSPhase represents the overall phase of the OpenEBS installation.
// +kubebuilder:validation:Enum=Pending;Installing;Running;Degraded;Failed
type OpenEBSPhase string

const (
	OpenEBSPhasePending    OpenEBSPhase = "Pending"
	OpenEBSPhaseInstalling OpenEBSPhase = "Installing"
	OpenEBSPhaseRunning    OpenEBSPhase = "Running"
	OpenEBSPhaseDegraded   OpenEBSPhase = "Degraded"
	OpenEBSPhaseFailed     OpenEBSPhase = "Failed"
)

// OpenEBSSpec defines the desired state of OpenEBS.
type OpenEBSSpec struct {
	// Hostpath configures the hostpath local PV provisioner.
	// +optional
	Hostpath *HostpathConfig `json:"hostpath,omitempty"`

	// LVM configures the LVM local PV CSI driver.
	// +optional
	LVM *LVMConfig `json:"lvm,omitempty"`

	// ZFS configures the ZFS local PV CSI driver.
	// +optional
	ZFS *ZFSConfig `json:"zfs,omitempty"`

	// Rawfile configures the rawfile local PV provisioner.
	// +optional
	Rawfile *RawfileConfig `json:"rawfile,omitempty"`

	// Mayastor configures the Mayastor replicated storage engine.
	// +optional
	Mayastor *MayastorConfig `json:"mayastor,omitempty"`

	// ImagePullSecrets is an optional list of references to secrets
	// to use for pulling OpenEBS images.
	// +optional
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	// NodeSelector restricts OpenEBS components to nodes matching the labels.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// HostpathConfig configures the hostpath local PV provisioner.
type HostpathConfig struct {
	// Enabled determines whether the hostpath provisioner is deployed.
	Enabled bool `json:"enabled"`

	// BasePath is the directory on the host used for storing volumes.
	// +kubebuilder:default="/var/openebs/local"
	// +optional
	BasePath string `json:"basePath,omitempty"`

	// IsDefaultClass makes the hostpath StorageClass the default.
	// +optional
	IsDefaultClass bool `json:"isDefaultClass,omitempty"`

	// StorageClassName overrides the name of the StorageClass.
	// +kubebuilder:default="openebs-hostpath"
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`
}

// LVMConfig configures the LVM local PV CSI driver.
type LVMConfig struct {
	// Enabled determines whether the LVM CSI driver is deployed.
	Enabled bool `json:"enabled"`

	// StorageClassName overrides the name of the StorageClass.
	// +kubebuilder:default="openebs-lvm"
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// IsDefaultClass makes the LVM StorageClass the default.
	// +optional
	IsDefaultClass bool `json:"isDefaultClass,omitempty"`

	// VolumeGroup is the default LVM volume group to use.
	// +kubebuilder:default="lvmvg"
	// +optional
	VolumeGroup string `json:"volumeGroup,omitempty"`
}

// ZFSConfig configures the ZFS local PV CSI driver.
type ZFSConfig struct {
	// Enabled determines whether the ZFS CSI driver is deployed.
	Enabled bool `json:"enabled"`

	// StorageClassName overrides the name of the StorageClass.
	// +kubebuilder:default="openebs-zfs"
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// PoolName is the default ZFS pool to use.
	// +kubebuilder:default="zfspool"
	// +optional
	PoolName string `json:"poolName,omitempty"`
}

// RawfileConfig configures the rawfile local PV provisioner.
type RawfileConfig struct {
	// Enabled determines whether the rawfile provisioner is deployed.
	Enabled bool `json:"enabled"`

	// BasePath is the directory on the host used for storing volume files.
	// +kubebuilder:default="/var/openebs/rawfile"
	// +optional
	BasePath string `json:"basePath,omitempty"`
}

// MayastorConfig configures the Mayastor replicated storage engine.
type MayastorConfig struct {
	// Enabled determines whether Mayastor is deployed.
	Enabled bool `json:"enabled"`

	// EtcdReplicaCount sets the number of etcd replicas.
	// +kubebuilder:default=3
	// +optional
	EtcdReplicaCount int `json:"etcdReplicaCount,omitempty"`

	// IOEngineResources defines resource requests/limits for the io-engine.
	// +optional
	IOEngineResources *ResourceSpec `json:"ioEngineResources,omitempty"`

	// CoreAgentResources defines resource requests/limits for the core agent.
	// +optional
	CoreAgentResources *ResourceSpec `json:"coreAgentResources,omitempty"`
}

// ResourceSpec defines CPU and memory resources.
type ResourceSpec struct {
	// +optional
	CPU string `json:"cpu,omitempty"`
	// +optional
	Memory string `json:"memory,omitempty"`
	// +optional
	Hugepages2Mi string `json:"hugepages2Mi,omitempty"`
}

// EngineStatus reports the status of a deployed storage engine.
type EngineStatus struct {
	// Name is the engine identifier.
	Name OpenEBSEngine `json:"name"`

	// Phase reports the current state of the engine.
	Phase OpenEBSPhase `json:"phase,omitempty"`

	// Version is the deployed version of the engine.
	// +optional
	Version string `json:"version,omitempty"`

	// Message provides additional information about the current state.
	// +optional
	Message string `json:"message,omitempty"`
}

// OpenEBSStatus defines the observed state of OpenEBS.
type OpenEBSStatus struct {
	// Phase is the overall phase of the installation.
	// +optional
	Phase OpenEBSPhase `json:"phase,omitempty"`

	// Conditions represent the current state of the OpenEBS deployment.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Engines lists the status of each engine.
	// +optional
	Engines []EngineStatus `json:"engines,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// OpenEBS is the Schema for the OpenEBS API.
type OpenEBS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenEBSSpec   `json:"spec,omitempty"`
	Status OpenEBSStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenEBSList contains a list of OpenEBS.
type OpenEBSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenEBS `json:"items"`
}
