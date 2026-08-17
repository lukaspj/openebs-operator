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
defaultMayastorTag      = "v2.11.1"
defaultEtcdImage        = "openebs/etcd:3.6.4-debian-12-r0"
```

For Mayastor the tag is a bare version; images are constructed as
`openebs/mayastor-<component>:<mayastorTag>` via `mayastorImage()` helper.

### Step 2: Map engine to upstream repo

| Engine    | Image                           | Upstream Repo                         | Release Branch        |
|-----------|---------------------------------|---------------------------------------|-----------------------|
| LVM       | `openebs/lvm-driver`            | `openebs/lvm-localpv`                 | `release/X.Y`         |
| Hostpath  | `openebs/provisioner-localpv`   | `openebs/dynamic-localpv-provisioner` | `release/X.Y`         |
| ZFS       | `openebs/zfs-driver`            | `openebs/zfs-localpv`                 | `release/X.Y`         |
| Rawfile   | `openebs/rawfile-localpv`       | `openebs/rawfile-localpv`             | `develop` (only branch that carries the chart) |
| Mayastor  | `openebs/mayastor-*`            | `openebs/mayastor-extensions`         | `release/X.Y` (match `defaultMayastorTag`) |

### Step 3: Fetch upstream chart templates

Fetch every template in the chart's `templates/` dir for the engine — NOT just
the deployment/daemonset. Missing configmaps (e.g. `openebs-zfspv-bin`) and
hidden pod-spec details are the most common drift source.

All URLs on `raw.githubusercontent.com/openebs/<repo>/refs/heads/<branch>`:

**LVM** (`openebs/lvm-localpv`, e.g. `release/1.9`, path `deploy/helm/charts/`):
```
values.yaml
templates/lvm-controller.yaml   (CSI controller Deployment)
templates/lvm-node.yaml         (CSI node DaemonSet)
```

**Hostpath** (`openebs/dynamic-localpv-provisioner`, e.g. `release/4.5`, path `deploy/helm/charts/`):
```
values.yaml
templates/deployment.yaml
```
Check that `helperPod.image.tag` in values.yaml matches `defaultHelperImage`.
Deployment mode only — ignore the node-deployment DaemonSet.

**ZFS** (`openebs/zfs-localpv`, e.g. `release/2.10`, path `deploy/helm/charts/`):
```
values.yaml
templates/zfs-controller.yaml   (CSI controller Deployment)
templates/zfs-node.yaml         (CSI node DaemonSet)
templates/configmap.yaml        (openebs-zfspv-bin zfs wrapper script)
```

**Rawfile** (`openebs/rawfile-localpv`, branch `develop`, path `deploy/helm/rawfile-localpv/` — note extra subdir):
```
values.yaml
templates/deployment.yaml
```

**Mayastor** (`openebs/mayastor-extensions`, e.g. `release/2.11`, path `chart/` in repo root; etcd is a subchart — skip):
```
values.yaml
templates/io-engine-daemonset.yaml
templates/agent-core-deployment.yaml      (agent-core + agent-ha-cluster)
templates/ha-node-daemonset.yaml
templates/api-rest-deployment.yaml
templates/operator-diskpool-deployment.yaml
templates/csi-controller-deployment.yaml
templates/csi-node-daemonset.yaml
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
8.  **Volumes / VolumeMounts** — all chart volumes, mount paths, hostPath types,
    mountPropagation match
9.  **Lifecycle hooks** — preStop/postStart on any container (registrar containers
    carry `rm -rf /registration/<driver> /registration/<driver>-reg.sock` preStop)
10. **Security context** — if chart set, operator matches
11. **Service account** — matches chart
12. **Strategy / updateStrategy** — Deployment strategy AND DaemonSet updateStrategy
    (LVM/ZFS use RollingUpdate maxUnavailable 100%)
13. **Replicas** — chart default
14. **Tolerations / NodeSelector / Affinity** — chart defaults
15. **hostNetwork** — per-engine values default, do NOT assume:
    LVM node = **false** (values), ZFS node = **true** (hardcoded), Mayastor io-engine/ha-node = true
16. **Extra templates** — configmaps (e.g. `openebs-zfspv-bin`), any
    non-workload template the chart ships. Constructor must exist in
    component.go AND be wired into deployer.go before its consumer + in cleanup()
17. **CSI sidecar versions** — from chart `values.yaml` `csi.*` / `*Tag` fields,
    not from the engine image

**Do NOT enforce:**
- `OPENEBS_IO_INSTALLER_TYPE` — intentional operator vs helm difference
- Analytics / telemetry env vars — operator disables by omitting
- Labels — operator uses own label scheme
- Namespace — operator uses `openebs` (and `mayastor` for mayastor)
- RBAC — chart RBAC is broader; operator RBAC is per-engine
- PriorityClasses — operator ships none
- Chart init containers (etcd-probe/grpc-probe) — operator deploys sequentially,
  pod restarts suffice

### Step 5: Apply fixes

`edit` in `component.go` for each mismatch. If a new standalone resource
(configmap etc.) is needed, add constructor + wire into `deployer.go`
(deploy before consumer, add to cleanup list). Update `component_test.go`
expectations for changed args/env/volumes. Run `go test ./...` after all fixes.

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

Registrar registration socket names are engine-specific:
`plugins/lvm-localpv/csi.sock` vs `plugins/zfs-localpv/csi.sock` — the preStop
`rm -rf` path must match the same driver dir.

ZFS node plugin expects extra volumes/mounts the LVM one does not have:
`encr-keys` (/home/keys), `chroot-zfs` (configMap openebs-zfspv-bin, subPath zfs,
mode 0555), `host-root` (/host, readOnly, HostToContainer). Missing chroot-zfs
silently breaks zfs plugin commands on nodes.

LVM node plugin args include rate-limit flags `--kube-api-qps=0 --kube-api-burst=0`
and `--kubelet-dir=/var/lib/kubelet/`. ZFS node env includes
`ALLOWED_TOPOLOGIES` (chart default `All`).

CSI sidecar images are from chart values, not engine image.  Check versions
in the chart's `values.yaml` under `csi.*`. Chart release branches sometimes
carry newer sidecars than `component.go` defaults (e.g. lvm chart registrar
v2.13.0 vs our v2.10.1) — flag as drift, fix only when bumping defaults.

### Hostpath (deployment mode)

Chart has deployment + node-deployment modes.  Operator only does deployment
mode (`.Values.localpv.nodeDeployment.enabled: false`).  Ignore DaemonSet.

`defaultHelperImage` must match `helperPod.image.repository:tag` from
hostpath chart values.yaml.

### Rawfile

Single Deployment + StorageClass.  Simplest engine. Chart lives ONLY on
`develop` (release branches have no chart). Path has extra subdir:
`deploy/helm/rawfile-localpv/`.

### Mayastor

Summary of resources operator deploys (19 total):

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
| 14 | io.openebs.csi-mayastor | CSIDriver | — |
| 15 | mayastor | StorageClass | — |
| 16 | volumesnapshotclasses.snapshot.storage.k8s.io | CRD | — |
| 17 | volumesnapshots.snapshot.storage.k8s.io | CRD | — |
| 18 | volumesnapshotcontents.snapshot.storage.k8s.io | CRD | — |
| 19 | mayastor-snapshot | VolumeSnapshotClass | — |

**Chart-operator facts to verify (2.11 CLI):**

- io-engine args (no more `--endpoint/--node/--namespace`): `--grpc-ip=$(MY_POD_IP)`,
  `--grpc-port=10124`, `-N$(MY_NODE_NAME)`, `-Rhttps://mayastor-agent-core:50051`,
  `-y/var/local/mayastor/io-engine/config.yaml`, `-l1,2` (from cpuCount via
  `coreListUniq`/`untilStep` helpers), `-p=mayastor-etcd:2379` (BARE, no scheme),
  `--ptpl-dir=/var/local/mayastor/io-engine/ptpl/`, `--api-versions=v1`,
  `--tgt-crdt=30`, `--ps-retries=300`, `--pool-io-error-threshold=64`,
  `--pool-io-stall-deadline=110s110s` (compound duration = 2× ioTimeout, intentional),
  `--pool-io-stall-transition-threshold=3`, `--pool-io-stall-transition-window=3h`
- Compound-duration quirks are NOT template bugs: `--nvme-io-timeout=110s10s`
  = 110s+10s (humantime), ships on both release/2.11 and develop — replicate literally.
- agent-core `--store=mayastor-etcd:2379` (bare) but agent-ha-cluster
  `--store=http://mayastor-etcd:2379` (http scheme) — scheme mismatch is upstream behavior.
- agent-ha-cluster `--core-grpc=https://mayastor-agent-core:50051` (https).
- api-rest: ports 8080+8081, `--http=[::]:8081`,
  `--core-grpc=https://mayastor-agent-core:50051`.
- operator-diskpool: `-e http://mayastor-api-rest:8081 -nmayastor --request-timeout=5s
  --interval=30s --ansi-colors=true --fmt-style=pretty` (2.11 CLI — old
  `--namespace=` arg was removed and crashes).
- csi-node container: `--grpc-port=10199`, nvme args, `--kubelet-path=/var/lib/kubelet`,
  `--enable-rest`, `--enable-registration`, `--fmt-style=pretty`, `--ansi-colors=true`.
- Kubelet plugin directory: `io.openebs.mayastor` (NOT `io.openebs.csi-mayastor`)
  The CSIDriver name (`io.openebs.csi-mayastor`) is distinct from the kubelet
  plugin dir (`io.openebs.mayastor`).
- CSI socket path: `/var/lib/csi/sockets/pluginproxy/csi.sock`
- agent-core env: `MY_POD_NAME`, `MY_POD_NAMESPACE` (from fieldRef)
- io-engine: privileged, hostNetwork, nodeSelector `openebs.io/engine=mayastor`,
  volumes: device (/dev), udev (/run/udev), dshm (emptyDir Memory 1Gi),
  hugepages-2mi, configlocation (/var/local/mayastor/io-engine/ DirOrCreate)
- Operator skips: TLS (no cert-manager), jaeger, loki, alloy, nats, callhome,
  metrics-exporter (follow-up — needs its own image field).
- VolumeSnapshot CRDs installed via embedded YAML from external-snapshotter v8.2.0,
  managed via `volume_snapshot.go`, applied as unstructured.
- VolumeSnapshotClass `mayastor-snapshot` uses driver `io.openebs.csi-mayastor`,
  deletionPolicy `Delete`. Customizable via `spec.mayastor.snapshotClassName`.

---

## CI/CD note

Run this check whenever engine image defaults are bumped in `component.go`.
Remember the checklist covers BOTH controller AND node templates plus any
configmap/extra template — the node-side and configmap drift is what
historically got missed.
