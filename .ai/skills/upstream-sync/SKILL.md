---
name: upstream-sync
description: >
  Verify openebs-operator constructs match upstream OpenEBS Helm charts for every
  storage engine (LVM, Hostpath, ZFS, Rawfile, Mayastor).  Fetches chart templates
  from GitHub release branches, compares Deployments, DaemonSets, StatefulSets,
  Services, env vars, probes, ports, volumes, RBAC, CSIDrivers, StorageClasses
  against operator code in component.go + deployer.go. Flags mismatches, applies
  fixes, runs go test. Trigger: "align with chart", "sync with upstream",
  "check chart version", "update to match helm chart", "compare with upstream".
---

# upstream-sync — align operator with upstream Helm charts

Read image versions from `internal/controller/component.go` constants, resolve each
engine to its upstream repo + release branch, fetch the matching chart templates
and values, then compare against operator constructors.

---

## Workflow

### Step 1: Read image versions

```go
defaultLVMImage         = "openebs/lvm-driver:X.Y.Z"
defaultHostpathImage    = "openebs/provisioner-localpv:X.Y.Z"
defaultZFSImage         = "openebs/zfs-driver:X.Y.Z"
defaultRawfileImage     = "openebs/rawfile-localpv:X.Y.Z"
defaultHelperImage      = "openebs/linux-utils:X.Y.Z"
defaultMayastorTag      = "2.7.0"
defaultEtcdImage        = "openebs/etcd:3.6.4-debian-12-r0"
```

For Mayastor the tag is a bare version; images are constructed as
`openebs/mayastor-<component>:<mayastorTag>` via `mayastorImage()` helper.

### Step 2: Map engine to upstream repo

| Engine    | Image                           | Upstream Repo                             | Release Branch        |
|-----------|---------------------------------|-------------------------------------------|-----------------------|
| LVM       | `openebs/lvm-driver`            | `openebs/lvm-localpv`                     | `release/X.Y`         |
| Hostpath  | `openebs/provisioner-localpv`   | `openebs/dynamic-localpv-provisioner`     | `release/X.Y`         |
| ZFS       | `openebs/zfs-driver`            | `openebs/zfs-localpv`                     | `release/X.Y`         |
| Rawfile   | `openebs/rawfile-localpv`       | `openebs/rawfile-localpv`                 | `release/X.Y`         |
| Mayastor  | `openebs/mayastor-*`            | `openebs/mayastor-extensions`             | `develop`             |

### Step 3: Fetch upstream chart templates

For each engine fetch templates from the matching branch on GitHub raw.

**LVM** (`openebs/lvm-localpv`, branch `release/X.Y`):
```
deploy/helm/charts/templates/deployment.yaml   (CSI controller)
deploy/helm/charts/templates/daemonset.yaml    (CSI node)
deploy/helm/charts/templates/csidriver.yaml
deploy/helm/charts/templates/storageclass.yaml
deploy/helm/charts/templates/rbac.yaml
deploy/helm/charts/values.yaml
```

**Hostpath** (`openebs/dynamic-localpv-provisioner`, deployment mode only):
```
deploy/helm/charts/templates/deployment.yaml
deploy/helm/charts/values.yaml
```
Check that `helperPod.image.tag` in values.yaml matches `defaultHelperImage`.

**ZFS** (`openebs/zfs-localpv`):
```
deploy/helm/charts/templates/controller.yaml   (CSI controller)
deploy/helm/charts/templates/node.yaml          (CSI node)
deploy/helm/charts/templates/csidriver.yaml
deploy/helm/charts/templates/storageclass.yaml
deploy/helm/charts/templates/rbac.yaml
deploy/helm/charts/values.yaml
```

**Rawfile** (`openebs/rawfile-localpv`):
```
deploy/helm/charts/templates/deployment.yaml
deploy/helm/charts/values.yaml
```

**Mayastor** (`openebs/mayastor-extensions`, branch `develop`):

Templates are under `chart/templates/mayastor/`:
```
chart/templates/mayastor/agents/core/agent-core-deployment.yaml
chart/templates/mayastor/apis/rest/   (explore for api-rest resources)
chart/templates/mayastor/csi/csi-controller-deployment.yaml
chart/templates/mayastor/csi/csi-node-daemonset.yaml
chart/templates/mayastor/io-engine/   (explore)
chart/templates/mayastor/diskpool/    (explore)
chart/templates/mayastor/agents/ha/   (explore)
chart/templates/mayastor/etcd/        (explore — may use bitnami subchart)
chart/templates/mayastor/csidriver.yaml
chart/templates/mayastor/storageclass.yaml
chart/values.yaml
chart/Chart.yaml
```

Chart uses OpenEBS's own etcd image (`openebs/etcd`), not bitnami.
CSI sidecar versions are in `values.yaml` under `csi.image.*Tag`.

### Step 4: Compare resources

For each constructor in `component.go`, compare against chart default values
(no `Values.` conditionals, only static defaults).

**Checklist per resource:**

1.  **Container image** — matches chart default (`registry/repo:tag`)
2.  **Container count** — same number, same order as chart
3.  **Container names** — match chart exactly
4.  **Args** — every arg present with correct values
5.  **Env vars** — all chart env vars present (skip `OPENEBS_IO_INSTALLER_TYPE`, analytics)
6.  **Ports** — names match chart, protocol matches
7.  **Probes** — liveness/readiness type, path, port, timing
8.  **Volumes / VolumeMounts** — all chart volumes, mount paths match
9.  **Security context** — if chart set, operator matches
10. **Service account** — matches chart
11. **Strategy** — chart default (e.g., `Recreate`)
12. **Replicas** — chart default
13. **Tolerations / NodeSelector / Affinity** — chart defaults
14. **hostNetwork** — if chart set, operator matches

**Do NOT enforce:**
- `OPENEBS_IO_INSTALLER_TYPE` — intentional operator vs helm difference
- Analytics / telemetry env vars — operator disables by omitting
- Labels — operator uses own label scheme
- Namespace — operator uses `openebs` (and `mayastor` for mayastor)
- RBAC — chart RBAC is broader; operator RBAC is per-engine

### Step 5: Apply fixes

`edit` in `component.go` for each mismatch. Run `go test ./...` after all fixes.

### Step 6: Verify with tests

After all edits, run:
```sh
go vet ./...
go test ./...
```
All tests must pass.

---

## Engine-specific notes

### LVM & ZFS (CSI engines)

Controller Deployment (CSI sidecars + driver plugin) and node DaemonSet
(node registrar + driver plugin).  Both must be compared.

CSI sidecar images are from chart values, not engine image.  Check versions
in the chart's `values.yaml` under `csi.*`.

### Hostpath (deployment mode)

Chart has deployment + node-deployment modes.  Operator only does deployment
mode (`.Values.localpv.nodeDeployment.enabled: false`).  Ignore DaemonSet.

`defaultHelperImage` must match `helperPod.image.repository:tag` from
hostpath chart values.yaml.

### Rawfile

Single Deployment + StorageClass.  Simplest engine.

### Mayastor

Summary of resources operator deploys (18 total):

| # | Resource | Kind | Container(s) |
|---|----------|------|-------------|
| 1 | mayastor-etcd | StatefulSet | `etcd` |
| 2 | mayastor-etcd | Service | — |
| 3 | mayastor-agent-core | Deployment | `agent-core` + `agent-ha-cluster` (sidecar) |
| 4 | mayastor-api-rest | Deployment | `api-rest` |
| 5 | mayastor-api-rest | Service | — |
| 6 | mayastor-csi-controller | Deployment | 6 containers: `mayastor-csi-controller` + 5 CSI sidecars |
| 7 | mayastor-io-engine | DaemonSet | `io-engine` (privileged, hostNetwork) |
| 8 | mayastor-csi-node | DaemonSet | `csi-node` (privileged) + `csi-driver-registrar` |
| 9 | mayastor-operator-diskpool | Deployment | `operator-diskpool` |
| 10 | mayastor-agent-ha-node | DaemonSet | `agent-ha-node` (hostNetwork) |
| 11 | mayastor-service-account | ServiceAccount | — |
| 12 | mayastor-role | ClusterRole | — |
| 13 | mayastor-binding | ClusterRoleBinding | — |
| 14 | csi.nvmf.openebs.io | CSIDriver | — |
| 15 | mayastor | StorageClass | — |
| 16 | volumesnapshotclasses.snapshot.storage.k8s.io | CRD | — |
| 17 | volumesnapshots.snapshot.storage.k8s.io | CRD | — |
| 18 | volumesnapshotcontents.snapshot.storage.k8s.io | CRD | — |
| 19 | mayastor-snapshot | VolumeSnapshotClass | — |

**Critical chart-operator differences to verify:**

- `--store` arg for agent-core/agent-ha-cluster uses `http://` prefix:
  `--store=http://mayastor-etcd:2379`
- CSI socket path: `/var/lib/csi/sockets/pluginproxy/csi.sock`
- Kubelet plugin directory: `io.openebs.mayastor` (NOT `csi.nvmf.openebs.io`)
  The CSIDriver name (`csi.nvmf.openebs.io`) is distinct from the kubelet
  plugin dir (`io.openebs.mayastor`).
- CSI controller: `hostNetwork: true`
- CSI node: volumes `/dev`, `/sys`, `/run/udev`, `/csi` (plugin-dir),
  `/var/lib/kubelet` (kubelet-dir, bidirectional mount propagation).
  Also `registration-dir` for registrar.
- csi-node container uses `--enable-rest`, `--enable-registration`,
  `--grpc-ip=$(MY_POD_IP)`, `--node-name=$(MY_NODE_NAME)`
- csi-node env: `RUST_LOG`, `MY_NODE_NAME`, `MY_POD_IP`, `RUST_BACKTRACE`
- agent-core env: `MY_POD_NAME`, `MY_POD_NAMESPACE` (from fieldRef)
- io-engine: privileged, hostNetwork, nodeSelector `openebs.io/engine=mayastor`,
  volumes: hugepage-2mi, hugepage-1gi, host-tmp, device (/dev)
- agent-ha-agent-ha-node: hostNetwork, args include `--store=http://mayastor-etcd:2379`
- Operator skips: TLS (no cert-manager), jaeger, loki, alloy, nats, callhome,
  metrics-exporter.  Use `http://` instead of `https://` for REST endpoint args.
- REST endpoint: `http://mayastor-api-rest:8081` (no TLS)
- CSI sidecar versions come from chart `values.yaml` `csi.image.*Tag` fields.
  Mayastor uses a `snapshot-controller` container which no other engine uses.
  Its image tag is `snapshotControllerTag` (v8.2.0 in chart).
- VolumeSnapshot CRDs installed via embedded YAML from external-snapshotter v8.2.0:
  `volumesnapshotclasses.snapshot.storage.k8s.io`,
  `volumesnapshots.snapshot.storage.k8s.io`,
  `volumesnapshotcontents.snapshot.storage.k8s.io`.
  CRDs applied as unstructured before RBAC. Managed via `volume_snapshot.go`.
- VolumeSnapshotClass `mayastor-snapshot` uses driver `csi.nvmf.openebs.io`,
  deletionPolicy `Delete`. Customizable via `spec.mayastor.snapshotClassName`.

**Template fetch checklist for Mayastor:**
- `chart/templates/mayastor/agents/core/agent-core-deployment.yaml`
- `chart/templates/mayastor/apis/rest/` — find api-rest deployment + service
- `chart/templates/mayastor/csi/csi-controller-deployment.yaml`
- `chart/templates/mayastor/csi/csi-node-daemonset.yaml`
- `chart/templates/mayastor/io-engine/` — find DaemonSet template
- `chart/templates/mayastor/agents/ha/` — find node + cluster templates
- `chart/templates/mayastor/diskpool/` — find diskpool deployment
- `chart/templates/mayastor/etcd/` — find StatefulSet + Service
- `chart/values.yaml` — default values and image tags

---

## CI/CD note

Run this check whenever engine image defaults are bumped in `component.go`.
