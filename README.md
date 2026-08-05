# openebs-operator

Kubernetes operator that deploys and manages [OpenEBS](https://openebs.io) storage engines via CRDs instead of Helm charts. Uses controller-runtime reconciliation — CRD pre-install, post-install retry with requeue, pre-delete cleanup via finalizer.

## Engines

| Engine   | Status  | Resources                                                                                                           |
|----------|---------|---------------------------------------------------------------------------------------------------------------------|
| LVM      | Stable  | Deployment (CSI provisioner, resizer, snapshotter, lvm-plugin) + DaemonSet + CSIDriver + StorageClass               |
| Hostpath | Stable  | Deployment (provisioner-localpv) + StorageClass                                                                     |
| ZFS      | Stable  | Deployment (CSI provisioner, resizer, zfs-plugin) + DaemonSet + CSIDriver + StorageClass                            |
| Rawfile  | Stable  | Deployment (rawfile-provisioner) + StorageClass                                                                     |
| Mayastor | Stubbed | Creates namespace only; reports `Installing` phase. Full support pending.                                           |

## Quick start

```sh
# Deploy the operator
kubectl apply -f deploy/operator.yaml

# Deploy an example OpenEBS CR (LVM + hostpath)
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
```

Each engine supports `storageClassName`, `isDefaultClass`, and `nodeSelector`. LVM adds `volumeGroup`; Hostpath and Rawfile add `basePath`; ZFS adds `poolName`.

## Build

```sh
go mod tidy
go vet ./...
go test ./...
go build -o bin/manager ./cmd
```

Pre-built image: `ghcr.io/aldershaab-it/openebs-operator:latest`

## License

MIT
