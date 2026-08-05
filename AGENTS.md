# AGENTS.md — openebs-operator

## Purpose

Kubernetes operator that deploys and manages OpenEBS storage engines via CRDs instead of Helm charts. Replaces Helm hooks with controller-runtime reconciliation: CRD pre-install → apply loop, post-install retry → requeue, pre-delete cleanup → finalizer.

## Build & Test

```sh
go mod tidy
go vet ./...
go test ./...
go build -o bin/manager ./cmd
```

## Key design decisions

- **Fake client for e2e** — no envtest needed. Tests call `Reconcile` directly against `fake.NewClientBuilder`. All 117 tests run offline.
- **Plain Go testing** — no ginkgo/gomega. Use `testing.T` with `t.Helper()`, `t.Fatal`, `t.Error`.
- **One CRD (`OpenEBS`)** — cluster-scoped, group `storage.aldershaab-it.dk/v1alpha1`. Multiple engines managed in a single resource.
- **Engine selection via nil check** — `Spec.LVM != nil && Spec.LVM.Enabled` gates deployment. Disabled config pointers mean engine skipped.
- **Finalizer: `storage.aldershaab-it.dk/finalizer`** — blocks CR deletion until cleanup runs (StorageClasses, CSIDrivers, Deployments, DaemonSets removed).
- **Mayastor stubbed** — reports `Installing` phase. Full support pending.

## Directory layout

```
api/v1alpha1/          CRD types + deepcopy
cmd/main.go            Entry point (controller-runtime manager)
internal/controller/   Reconciler, Deployer, resource constructors
config/crd/bases/      Generated CRD YAML
config/rbac/           ClusterRole for operator
config/manager/        Deployment manifest for operator
deploy/                All-in-one operator.yaml + example CR
```

## Engines

| Engine | Resources |
|--------|-----------|
| LVM | Deployment (4 containers: csi-provisioner, resizer, snapshotter, lvm-plugin) + DaemonSet + CSIDriver + StorageClass |
| Hostpath | Deployment (1 container: provisioner-localpv) + StorageClass |
| ZFS | Deployment (3 containers: csi-provisioner, resizer, zfs-plugin) + DaemonSet + CSIDriver + StorageClass |
| Rawfile | Deployment (1 container: rawfile-provisioner) + StorageClass |
| Mayastor | Pending — creates namespace, reports Installing |

## Image versions

Bump these in `component.go` when updating:

```go
lvmImage       = "openebs/lvm-driver:v2.12.2"
hostpathImage  = "openebs/provisioner-localpv:v4.1.0"
zfsImage       = "openebs/zfs-driver:v2.6.0"
rawfileImage   = "openebs/rawfile-localpv:v0.8.0"
csiProvisioner  = "registry.k8s.io/sig-storage/csi-provisioner:v4.0.1"
csiResizer      = "registry.k8s.io/sig-storage/csi-resizer:v1.10.1"
csiSnapshotter  = "registry.k8s.io/sig-storage/csi-snapshotter:v7.0.2"
csiNodeRegistrar = "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.10.1"
```

## Dependencies

- Go 1.24+
- controller-runtime v0.22.1
- k8s.io/api v0.34.0, apimachinery v0.34.1, client-go v0.34.0

No Helm, no Tiller. Operator replaces `openebs/openebs` umbrella chart.
