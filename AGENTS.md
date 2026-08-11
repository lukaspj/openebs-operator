# AGENTS.md — openebs-operator

## Purpose

Kubernetes operator that deploys and manages OpenEBS storage engines via CRDs instead of Helm charts. Replaces Helm hooks with controller-runtime reconciliation: CRD pre-install → apply loop, post-install retry → requeue, pre-delete cleanup → finalizer.

## Build & Test

```sh
go vet ./...
go test ./...
go build -o bin/manager ./cmd
```

## Key design decisions

- **Fake client for e2e** — no envtest needed. Tests call `Reconcile` directly against `fake.NewClientBuilder`. All 159 tests run offline.
- **Plain Go testing** — no ginkgo/gomega. Use `testing.T` with `t.Helper()`, `t.Fatal`, `t.Error`.
- **One CRD (`OpenEBS`)** — cluster-scoped, group `storage.aldershaab-it.dk/v1alpha1`. Multiple engines managed in a single resource.
- **Engine selection via nil check** — `Spec.LVM != nil && Spec.LVM.Enabled` gates deployment. Disabled config pointers mean engine skipped.
- **Finalizer: `storage.aldershaab-it.dk/finalizer`** — blocks CR deletion until cleanup runs (StorageClasses, CSIDrivers, Deployments, DaemonSets removed).

## Helm hook coverage

Operator replaces three Helm hook patterns:

| Helm hook          | Operator equivalent                          |
|---------------------|-----------------------------------------------|
| `pre-install` CRDs  | `applyVolumeSnapshotCRDs()` called first in `deployMayastor`, before pods |
| `post-install` retry| Controller-runtime requeue (30s), `Reconcile` re-runs on failure |
| `pre-delete` cleanup| `finalizer` blocks CR deletion until `cleanup()` removes all engine resources |

Chart init containers (etcd-probe, grpc-probe) are unnecessary — operator deploys resources sequentially and pod restarts suffice.

## Directory layout

```
api/v1alpha1/              CRD types + deepcopy
cmd/main.go                Entry point (controller-runtime manager)
internal/controller/       Reconciler, Deployer, resource constructors
internal/controller/crds/  Embedded CRD YAML (VolumeSnapshot CRDs)
config/crd/bases/          Generated CRD YAML
config/rbac/               ClusterRole for operator
config/manager/            Deployment manifest for operator
deploy/                    All-in-one operator.yaml + example CR
.ai/skills/                OpenCode project skills
```

## Engines

| Engine | Resources |
|--------|-----------|
| LVM | Deployment (4 containers: csi-provisioner, resizer, snapshotter, lvm-plugin) + DaemonSet + CSIDriver + StorageClass |
| Hostpath | Deployment (1 container: provisioner-localpv) + StorageClass |
| ZFS | Deployment (3 containers: csi-provisioner, resizer, zfs-plugin) + DaemonSet + CSIDriver + StorageClass |
| Rawfile | Deployment (1 container: rawfile-provisioner) + StorageClass |
| Mayastor | 19 resources: etcd StatefulSet+Service, agent-core Deployment (2 containers), api-rest Deployment+Service, csi-controller Deployment (6 containers), io-engine DaemonSet, csi-node DaemonSet (2 containers), ha-node DaemonSet, operator-diskpool Deployment, ServiceAccount, ClusterRole+Binding, CSIDriver, StorageClass, 3 VolumeSnapshot CRDs, VolumeSnapshotClass |

## Image versions

Defaults in `component.go`. Override via CR `spec.images.*` (empty = fall back to default).

```go
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
defaultMayastorTag           = "2.7.0"
defaultEtcdImage             = "openebs/etcd:3.6.4-debian-12-r0"
```

## Upstream chart alignment

When engine image defaults change, verify constructs match the upstream Helm chart. Full workflow at `.ai/skills/upstream-sync/SKILL.md`. Quick reference:

| Engine    | Upstream repo                         | Branch          |
|-----------|---------------------------------------|-----------------|
| LVM       | openebs/lvm-localpv                   | release/X.Y     |
| Hostpath  | openebs/dynamic-localpv-provisioner   | release/X.Y     |
| ZFS       | openebs/zfs-localpv                   | release/X.Y     |
| Rawfile   | openebs/rawfile-localpv               | release/X.Y     |
| Mayastor  | openebs/mayastor-extensions           | develop         |

Checklist per resource: container names, images, args, env vars, ports, probes, volumes, security context, hostNetwork, nodeSelector.

Always run `go test ./...` after alignment. Verify Helm hooks are covered — see "Helm hook coverage" above for the three patterns each engine must satisfy.

## Dependencies

- Go 1.24+
- controller-runtime v0.22.1
- k8s.io/api v0.34.0, apimachinery v0.34.1, client-go v0.34.0

No Helm, no Tiller. Operator replaces `openebs/openebs` umbrella chart.

## Gotchas

- **Unstructured DeepCopy**: `unstructured.Unstructured` panics on `map[string]string` inside `Object`. Always use `map[string]interface{}` when constructing unstructured objects. See `mayastorVolumeSnapshotClass`.
- **apply vs applyUnstructured**: Use `d.applyUnstructured` for resources not in the controller-runtime scheme (VolumeSnapshotClass, CRDs). The normal `d.apply` calls `DeepCopyObject()` which fails on unstructured.
- **CRD delete**: Operator RBAC needs `delete` on `customresourcedefinitions` if cleanup removes CRDs. Currently granted.
- **Kubelet plugin dir ≠ CSI driver name**: For Mayastor, the kubelet plugin directory is `io.openebs.mayastor` but the CSIDriver name is `csi.nvmf.openebs.io`. The registrar registration path and the CSIDriver object are different resources with different naming.
