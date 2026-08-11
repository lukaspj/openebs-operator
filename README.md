# openebs-operator

Kubernetes operator that deploys and manages [OpenEBS](https://openebs.io) storage engines via CRDs instead of Helm charts. Uses controller-runtime reconciliation — CRD pre-install, post-install retry with requeue, pre-delete cleanup via finalizer.

## Engines

| Engine   | Status | Resources                                                                                                     |
|----------|--------|---------------------------------------------------------------------------------------------------------------|
| LVM      | Stable | Deployment (4 containers) + DaemonSet + CSIDriver + StorageClass                                              |
| Hostpath | Stable | Deployment (1 container) + StorageClass                                                                       |
| ZFS      | Stable | Deployment (3 containers) + DaemonSet + CSIDriver + StorageClass                                              |
| Rawfile  | Stable | Deployment (1 container) + StorageClass                                                                       |
| Mayastor | Stable | StatefulSet (etcd) + 5 Deployments + 4 DaemonSets + ServiceAccount + ClusterRole + CSIDriver + StorageClass + VolumeSnapshotClass + VolumeSnapshot CRDs |

## Quick start

```sh
# Deploy the operator
kubectl apply -f deploy/operator.yaml

# Deploy an example OpenEBS CR
kubectl apply -f deploy/cr-openebs.yaml
```

## Configuration

Edit the `OpenEBS` CR to enable engines:

```yaml
apiVersion: storage.aldershaab-it.dk/v1alpha1
kind: OpenEBS
metadata:
  name: openebs
spec:
  lvm:
    enabled: true
    volumeGroup: lvmvg
  hostpath:
    enabled: true
    basePath: /var/openebs/local
    isDefaultClass: true
  zfs:
    enabled: false
    poolName: zfspool
  rawfile:
    enabled: false
    basePath: /var/openebs/rawfile
  mayastor:
    enabled: false
    etcdReplicaCount: 1
    etcdStorageSize: 10Gi
    etcdStorageClassName: ""
    storageClassName: mayastor
    snapshotClassName: mayastor-snapshot
  images:
    mayastor: "2.7.0"
    etcd: "openebs/etcd:3.6.4-debian-12-r0"
```

### Engine-specific fields

| Engine   | Fields |
|----------|--------|
| LVM      | `volumeGroup`, `storageClassName`, `isDefaultClass` |
| Hostpath | `basePath`, `storageClassName`, `isDefaultClass`, `nodeSelector` |
| ZFS      | `poolName`, `storageClassName`, `isDefaultClass` |
| Rawfile  | `basePath`, `storageClassName`, `isDefaultClass` |
| Mayastor | `etcdReplicaCount`, `etcdStorageSize`, `etcdStorageClassName`, `storageClassName`, `snapshotClassName`, `ioEngineResources`, `coreAgentResources` |

### Image overrides

All engine images can be overridden via `spec.images`:

| Field | Default |
|-------|---------|
| `lvm` | `openebs/lvm-driver:1.9.1` |
| `hostpath` | `openebs/provisioner-localpv:4.5.0` |
| `zfs` | `openebs/zfs-driver:2.10.1` |
| `rawfile` | `openebs/rawfile-localpv:0.14.1` |
| `mayastor` | `2.7.0` (expands to `openebs/mayastor-<component>:2.7.0`) |
| `etcd` | `openebs/etcd:3.6.4-debian-12-r0` |
| `csiProvisioner` | `registry.k8s.io/sig-storage/csi-provisioner:v4.0.1` |
| `csiResizer` | `registry.k8s.io/sig-storage/csi-resizer:v1.10.1` |
| `csiSnapshotter` | `registry.k8s.io/sig-storage/csi-snapshotter:v7.0.2` |
| `csiAttacher` | `registry.k8s.io/sig-storage/csi-attacher:v4.8.1` |
| `csiNodeRegistrar` | `registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.10.1` |
| `csiSnapshotController` | `registry.k8s.io/sig-storage/snapshot-controller:v8.2.0` |

## Build

```sh
go vet ./...
go test ./...
go build -o bin/manager ./cmd
```

Pre-built image: `ghcr.io/lukaspj/openebs-operator:latest`

## E2e tests

Real-cluster end-to-end tests live in `e2e/`. They deploy the operator to a kind cluster and verify engine add/update/remove, etcd upgrades, and CR deletion cleanup.

```sh
go test -v -tags=e2e -count=1 ./e2e/...
```

Requires `kubeconfig` pointing to a cluster with the operator deployed. CI runs these on push/PR via `.github/workflows/e2e.yaml`.

## License

MIT
