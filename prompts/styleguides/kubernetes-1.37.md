### Kubernetes 1.37: Manifests, CRDs, Policies and Controllers — Complete Review Guide

> **Version**: Kubernetes 1.37.0 "Garhwal" (released 2026-08-26). Latest patch as of 2026-10-22: **1.37.1** (scheduled for 2026-09-15; the `v1.37.1` tags and GitHub release were published 2026-09-23). The October 2026 patch round (target 2026-10-13) has not produced a 1.37.2 yet; the next scheduled round targets 2026-11-10.[^3][^4]
> **Support**: 1.37 is the newest minor. Maintenance mode starts 2027-08-28, end of life is **2027-10-28**. Supported minors in October 2026: 1.37, 1.36 (EOL 2027-06-28), 1.35 (EOL 2027-02-28) and 1.34 (maintenance mode, **EOL 2026-10-27**). 1.33 and older are out of support. Kubernetes 1.38 is in development (latest pre-release `v1.38.0-alpha.2`, 2026-10-07; `alpha.3` was scheduled for 2026-10-21 but has not been published) and is planned for **2026-12-16**.[^3][^4][^5]
> **Client libraries**: `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`, `k8s.io/apiextensions-apiserver` **v0.37.x** (v0.37.0 2026-08-26, v0.37.1 2026-09-23; their `go.mod` says `go 1.26.0`). **controller-runtime v0.25.x** (v0.25.0 2026-09-03 is the first release on v0.37; latest v0.25.2 2026-09-30). **controller-tools v0.22.0** (2026-08-31) and v0.23.0 (2026-10-14) both build against v0.37. **Kubebuilder v4.16.0** (2026-09-10) scaffolds controller-runtime v0.25.0, controller-tools v0.22.0, `go 1.26.0` and derives `ENVTEST_K8S_VERSION=1.37` from `k8s.io/api`.[^11][^12][^29] Kubernetes 1.37 itself is built with Go 1.26.x.[^2]
> **Scope**: Kubernetes manifests (raw YAML, Kustomize output, Helm-rendered YAML), CustomResourceDefinitions with CEL, admission policies, RBAC and Pod security, and Go operators/controllers built on client-go / controller-runtime / Kubebuilder that target a 1.37 cluster. Sections marked **[NEW in 1.37]** cover behaviour introduced or changed in 1.37; everything else is valid on 1.37 and carried forward.
> **Note**: This document is self-contained. Version boundaries were checked against real kube-apiserver/kubectl builds 1.29.5, 1.30.3, 1.31.0, 1.32.0, 1.33.0, 1.34.1, 1.35.0, 1.36.2 and 1.37.0 (feature-gate state read from each server's `kubernetes_feature_enabled` metric). Pod *runtime* behaviour (kubelet, scheduler, controllers) is documented from the release notes and KEPs, not executed.

---

#### What's New in 1.37

Kubernetes 1.37 contains 67 tracked enhancements: 16 graduate to Stable, 23 to Beta, 27 enter Alpha, and 1 is a deprecation/removal.[^1] For manifest and controller authors the release is mostly about **graduations of things that already existed** plus a few **default changes that can break existing workloads**. Nothing in the core `apps/v1`, `batch/v1` or `v1` workload APIs is removed.

##### Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37]

| Feature | What it means for manifests / controllers | Gate (now GA, locked on) |
|---|---|---|
| **Pod certificates** (`PodCertificateRequest`, `podCertificate` projected volume) | `certificates.k8s.io/v1` `PodCertificateRequest` is served by default; Pods can mount kubelet-managed, auto-rotated key + X.509 chain. `PKIXPublicKey`/`ProofOfPossession` (deprecated in v1beta1) do not exist in v1.[^2][^21] | `PodCertificateRequest` |
| **ClusterTrustBundles** | `certificates.k8s.io/v1` `ClusterTrustBundle` and the `clusterTrustBundle` projected volume source are on by default.[^2] | `ClusterTrustBundle`, `ClusterTrustBundleProjection` |
| **StorageVersionMigration** | `storagemigration.k8s.io/v1` is served by default; the in-tree migrator rewrites stored objects (built-in and CRs) after a storage-version or encryption change.[^1][^25] | `StorageVersionMigrator` |
| **SELinuxMount** (+ `seLinuxChangePolicy`) | **Default behaviour change on SELinux nodes**: volumes of CSI drivers with `CSIDriver.spec.seLinuxMount: true` are mounted with `-o context=…` instead of being relabelled recursively. Pods with different SELinux labels sharing such a volume on one node can now block in `ContainerCreating`.[^1][^10] | `SELinuxMount` |
| **Pod hostname override** | `spec.hostnameOverride` sets the hostname seen inside the Pod, independent of `hostname`/`subdomain`.[^2] | `HostnameOverride` |
| **Relaxed Service name validation** | Service names only need to be RFC 1123 labels, so they may now **start with a digit** (`1st-api`). Dots are still rejected.[^2] | `RelaxedServiceNameValidation` |
| **HPA configurable tolerance** | `spec.behavior.scaleUp.tolerance` / `scaleDown.tolerance` replace the cluster-wide 10 % tolerance per HPA.[^2] | `HPAConfigurableTolerance` |
| **In-place resize of init/sidecar containers** | The `pods/resize` subresource also works for (restartable) init containers.[^2] | `InPlacePodVerticalScalingInitContainers` |
| **`PodReadyToStartContainers` condition** | The Pod condition is GA; when `False` it now carries a diagnostic message (e.g. sandbox has no IP).[^2] | `PodReadyToStartContainersCondition` |
| **Node declared features** | `Node.status.declaredFeatures` lets the control plane handle version skew per node.[^1] | `NodeDeclaredFeatures` |
| **DRA**: device taints/tolerations + `DeviceTaintRule`, extended-resource requests via DRA, `ResourceClaim.status.devices`, standard `resource.kubernetes.io/numaNode` attribute | `resource.k8s.io/v1` `DeviceTaintRule` is served by default; a classic `example.com/gpu: 1` request can be satisfied by a DRA driver via `DeviceClass.spec.extendedResourceName`.[^1][^28] | `DRADeviceTaints`, `DRADeviceTaintRules`, `DRAExtendedResource`, `DRAResourceClaimDeviceStatus` |
| **Resilient watch-cache initialisation** | During watch-cache (re)initialisation kube-apiserver rejects expensive LIST/WATCH with **HTTP 429** instead of overloading etcd. Clients must honour `Retry-After`.[^1] | `WatchCacheInitializationPostStartHook` |
| **KYAML** | `kubectl get -o kyaml` is stable.[^1][^30] | (kubectl) |
| **metrics.k8s.io/v1** | The resource-metrics API gets a v1 (identical to v1beta1). It is served by metrics-server, not kube-apiserver; `kubectl top` understands v1. v1beta1 keeps working.[^1][^2] | — |

The release blog also lists **resource health status in Pod status** (KEP-4680: `status.containerStatuses[].allocatedResourcesStatus` reports the health of device-plugin and DRA devices) among the 1.37 graduations, but the 1.37.0 kube-apiserver still reports `ResourceHealthStatus` as **beta** (enabled by default, as in 1.36). The field is populated by the kubelet; controllers may read it but must tolerate its absence on nodes or drivers that do not report health.[^1]

##### Beta and enabled by default in 1.37 [NEW in 1.37]

| Feature | Effect | Gate |
|---|---|---|
| **HPA scale to zero** | `minReplicas: 0` is accepted when the HPA has at least one `Object` or `External` metric; the HPA records `ScaledToZero` in its conditions.[^1][^23] | `HPAScaleToZero` (alpha since 1.16) |
| **HPA condition `observedGeneration`** | HPA conditions may carry `observedGeneration`.[^2] | `HPAGeneration` |
| **StatefulSet `maxUnavailable`** | `spec.updateStrategy.rollingUpdate.maxUnavailable` is honoured again (it was on in 1.35, switched off in 1.36 after a rollout bug, back on in 1.37).[^1] | `MaxUnavailableStatefulSet` |
| **PVC `Unused` condition** | `status.conditions[type=Unused]` with `lastTransitionTime` = "unused since".[^1] | `PersistentVolumeClaimUnusedSinceTime` |
| **Manifest-based admission configuration** | Webhooks/VAP/MAP can be loaded from files via `staticManifestsDir`; names ending in `.static.k8s.io` are reserved.[^24] | `ManifestBasedAdmissionControlConfig` |
| **Webhooks skip auth virtual resources** | Admission webhooks no longer see `tokenreviews`, `subjectaccessreviews`, …, matching VAP/MAP.[^2] | `ExcludeAdmissionWebhookVirtualResources` |
| **Webhook endpoint load balancing** | With `--enable-aggregator-routing=true`, webhook calls are spread across endpoints.[^2] | `WebhookRoundTripLoadBalancing` |
| **Concurrent watch decode** | Up to 10 conversion-webhook calls in parallel during watch-cache init (same total volume).[^1] | `ConcurrentWatchObjectDecode` |
| **etcd RangeStream**, **WatchList gzip** | Server-side performance; no manifest change.[^1][^2] | `EtcdRangeStream`, `WatchListCompression` |
| **Unsafe deletion of undecryptable objects** | Admins can delete objects that can no longer be decrypted/decoded through the API (`DeleteOptions.ignoreStoreReadErrorWithClusterBreakingPotential: true`, needs the `unsafe-delete-ignore-read-errors` verb), now also as a dry run.[^1][^2] | `AllowUnsafeMalformedObjectDeletion` |
| **Memory QoS (cgroup v2)**, **Storage capacity scoring**, **CSI volume-limit scaling** (`CSIDriver.spec.preventPodSchedulingIfMissing`), **native histograms**, **kubelet in user namespace** | Node/scheduler behaviour.[^1][^2] | `MemoryQoS`, `StorageCapacityScoring`, `VolumeLimitScaling`, `NativeHistograms`, `KubeletInUserNamespace` |

Beta but **disabled** by default in 1.37 (do not rely on them): `GenericWorkload` (Workload/PodGroup gang scheduling, `scheduling.k8s.io/v1beta1`), `DRAWorkloadResourceClaims`, `PodLevelResourceManagers` (the changelog says "enabled by default"; the 1.37.0 binary reports it disabled, matching the release blog), `PodAndContainerStatsFromCRI`, `DRAFractionalCapacityRange`. Watch-based route reconciliation in cloud-controller-manager (`CloudControllerManagerWatchBasedRoutesReconciliation`) is announced as beta but is disabled by default (kube-apiserver 1.37.0 still lists the gate as alpha).[^1][^2]

##### Alpha in 1.37 — off by default (new fields are silently dropped, new enum values rejected) [NEW in 1.37]

`StatefulSet` `updateStrategy.type: Recreate` (`StatefulSetRecreateStrategy`), `emptyDir.mode` (`EmptyDirVolumeMode`), `httpGet.protocol: HTTP2` for probes (`H2CContainerProbe`), gRPC probe TLS (`GRPCContainerProbeTLS`), per-mount bind options (`VolumeBindMountOptions`), `spec.evictionResponders` + EvictionRequest API (`EvictionRequestAPI`), memory-backed volume resize (`InPlacePodVerticalScalingMemoryBackedVolumes`), scheduler preemption for resize (`InPlacePodVerticalScalingSchedulerPreemption`), Node lifecycle conditions (`NodeLifecycleConditions`), CompositePodGroup, `batch/v1` Job `spec.scheduling` (`WorkloadWithJob`), DRA derived attributes / compatibility groups / optional node operations, default Pod sysctls, atomic-write volume `user` fields, Pod-level checkpoint/restore (CRI only).[^1][^2] See [Do Not Use or Recommend](#do-not-use-or-recommend).

##### Behaviour and API changes that need action [NEW in 1.37]

- **`scheduling.k8s.io/v1alpha2` is gone**; Workload/PodGroup moved to `v1beta1` (off by default) and `v1alpha3`; `DisruptionMode` became a struct; PodGroup condition `PodGroupScheduled` was renamed `PodGroupInitiallyScheduled`. Delete v1alpha2 objects before upgrading.[^2]
- **Static Pods can no longer reference Secrets/ConfigMaps** (`PreventStaticPodAPIReferences` removed; the restriction is permanent).[^1]
- **Pod-level resources and QoS**: an empty `spec.resources` (`{}`, `{requests: {}}`, `{limits: {}}`) no longer influences the QoS class.[^2]
- **Image volumes**: an empty `image.reference` in Pod *templates* is now rejected.[^2]
- **CEL list semantics fix**: for `x-kubernetes-list-type: set`/`map` lists, `==` no longer matches lists containing duplicates and `+` applies set/map merge semantics.[^2]
- **controller-runtime v0.25** (the 1.37 pairing) deprecates `GetEventRecorderFor`, `CustomDefaulter`/`CustomValidator` and `client.Apply` patch type; see [Controllers](#controllers-with-controller-runtime-v025).[^11]
- **client-go v0.37** removed nearly all internal `context.TODO()` calls and adds `…WithContext` REST-mapping and discovery APIs; the old ones are deprecated but stay.[^2]
- **kube-proxy**: a warning is logged when no proxy mode is set (nftables will become the default); `ipvs` mode (deprecated since 1.35) now logs a deprecation warning at startup and is guarded by the new `KubeProxyIPVS` gate (stage `DEPRECATED`, still on; expected off by default in 1.40, removed in 1.43). **kube-dns** is deprecated in favour of CoreDNS.[^1][^2]
- **kubectl**: `kubectl run -f/--filename` is deprecated (it was always ignored); `kubectl explain --recursive --max-depth N`; `kubectl get crd` shows SCOPE/VERSIONS/CREATED AT columns; `kubectl top` supports `metrics.k8s.io/v1`; `--proxy-url`.[^2]
- **Removed feature gates** (setting them now fails the component, e.g. `unrecognized feature gate: SidecarContainers`): `SidecarContainers`, `AnyVolumeDataSource`, `RetryGenerateName`, `BtreeWatchCache`, `OrderedNamespaceDeletion`, `StreamingCollectionEncodingToJSON`, `StreamingCollectionEncodingToProtobuf`, `APIServerTracing`, `ResilientWatchCacheInitialization`, `ConsistentListFromCache`, `RelaxedDNSSearchValidation`, `PreventStaticPodAPIReferences`, `GangScheduling`, `WorkloadAwarePreemption`; `DeclarativeValidationTakeover` is locked.[^2]
- **kubeadm**: the `v1beta3` config API is removed (use `v1beta4`); `KubeProxyConfiguration.mode` defaults to `iptables` explicitly.[^2]
- **kubelet**: `eventRecordQPS: 0` now means *unlimited*; the effective configuration is logged at startup (restrict `nodes/logs`); deprecated cAdvisor flags make the kubelet fail to start; `failCgroupV1` still defaults to `true` (cgroup v1 nodes refuse to start unless overridden).[^1][^2]
- **Dependencies**: default etcd 3.7.0; `google.golang.org/grpc` v1.82.1 (strict path checking always on).[^2]

---

#### Do Not Use or Recommend

The target cluster is **1.37**. Do not suggest anything that needs a newer release or a non-default feature gate unless the repository explicitly enables that gate (check kube-apiserver/kubelet flags, kind/kubeadm config, managed-cluster settings).

##### From Kubernetes 1.38 (not released; alpha builds only)

These exist only in `v1.38.0-alpha.*` builds and the matching `k8s.io/*` v0.38 pre-releases:[^6]

- ML-DSA keys for Pod certificates / `PodCertificateRequest` and for `CertificateSigningRequest` (`CertificateSigningRequestMLDSA`), ML-DSA kubelet certificate key algorithms, `crypto/mldsa` support in client-go (needs Go 1.27).
- `resource.MaxMilliQuantity()`, `Quantity.AsScaledInt64()` / `AsMilliInt64()`, saturating `Value`/`MilliValue`/`ScaledValue` (apimachinery v0.38).
- `Accept: application/json;drop=metadata.managedFields` (`ManagedFieldsOptOut`, alpha in 1.38).
- Resource versions returned on DELETE, on Pod binding and on eviction responses.
- `staticOrder` (`StaticFirst`/`StaticLast`/`ByName`) for manifest-based mutating webhooks — the KEP documents it as added in 1.38; 1.37 always runs manifest-based webhooks first.[^24]
- Env files (`EnvFiles`, `fileKeyRef`) and strict IP/CIDR validation (`StrictIPCIDRValidation`) become **GA** only in 1.38; in 1.37 they are beta and on by default, so they work but can still be switched off by a cluster admin.
- `KubeProxyNFTablesLocalhostNodePorts` on by default (beta in 1.38; alpha/off in 1.37).
- Removal of `CustomResourceFieldSelectors`, `CRDValidationRatcheting` and `DynamicResourceAllocation` gates, of `apidiscovery.k8s.io/v2beta1` types and of `metrics.k8s.io/v1alpha1` happen in 1.38 — on 1.37 those gates still exist (locked on).
- Mutable `CompositePodGroup.spec.schedulingPolicy.gang.minGroupCount`, CompositePodGroup status and finalizer protection.

##### Alpha in 1.37 — unavailable unless the gate is explicitly enabled

The API server **accepts the fields and silently drops them** when the gate is off; `kubectl apply` prints no warning, the field only survives in the `last-applied-configuration` annotation. Verified on a default 1.37.0 apiserver:

```yaml
# BAD on a default 1.37 cluster: all three fields are dropped without any warning
apiVersion: v1
kind: Pod
metadata: {name: alpha, namespace: default}
spec:
  evictionResponders:                       # EvictionRequestAPI (alpha)
    - {name: example.com/drainer, priority: 10000}
  containers:
    - name: app
      image: registry.example.com/app:1.0
      livenessProbe:
        httpGet: {path: /healthz, port: 8080, protocol: HTTP2}   # H2CContainerProbe (alpha)
      volumeMounts:
        - {name: scratch, mountPath: /scratch}
  volumes:
    - name: scratch
      emptyDir: {medium: Memory, sizeLimit: 64Mi, mode: 0700}    # EmptyDirVolumeMode (alpha)
```

`kubectl apply --dry-run=server -o jsonpath='{.spec.evictionResponders}|{.spec.containers[0].livenessProbe.httpGet}|{.spec.volumes[0].emptyDir}'` returns `|{"path":"/healthz","port":8080,"scheme":"HTTP"}|{"medium":"Memory","sizeLimit":"64Mi"}`. On 1.36 the same manifest fails loudly instead (`strict decoding error: unknown field "spec.evictionResponders"`, …) because the fields do not exist in that schema.

Do not recommend, on a default 1.37 cluster:

| Alpha (or beta-off) item | Gate | Status in 1.37 |
|---|---|---|
| StatefulSet `updateStrategy.type: Recreate` | `StatefulSetRecreateStrategy` | alpha; rejected: `must be 'RollingUpdate' or 'OnDelete'` |
| `emptyDir.mode` | `EmptyDirVolumeMode` | alpha; dropped |
| Probe `httpGet.protocol: HTTP2` | `H2CContainerProbe` | alpha; dropped |
| gRPC probe TLS | `GRPCContainerProbeTLS` | alpha |
| Per-mount `noexec`/`nodev`/`nosuid` bind options | `VolumeBindMountOptions` | alpha |
| `spec.evictionResponders`, EvictionRequest/Eviction resources | `EvictionRequestAPI` | alpha; dropped |
| Container `lifecycle.stopSignal` | `ContainerStopSignals` | alpha since 1.33, still alpha |
| `emptyDir` `sizeLimit` resize via `pods/resize` | `InPlacePodVerticalScalingMemoryBackedVolumes` | alpha |
| Projected-volume `user` / `defaultUser` file owner | `AtomicWriteVolumeUserFields` | alpha |
| Job `spec.scheduling` (gang / topology / disruption) | `WorkloadWithJob` | alpha |
| `scheduling.k8s.io` Workload / PodGroup / CompositePodGroup, Pod `spec.schedulingGroup` | `GenericWorkload` (beta, **off**), `CompositePodGroup` (alpha) | not served by default |
| Pod-level CPU/memory/topology managers | `PodLevelResourceManagers` | beta, **off** |
| CBOR wire format for clients | `CBORServingAndStorage`, `ClientsAllowCBOR` | alpha |
| DRA node-allocatable resources, derived attributes, compatibility groups, `SkipNodeOperations`, resource pool status | `DRANodeAllocatableResources`, `DRADerivedAttributes`, `DRADeviceCompatibilityGroups`, `DRAOptionalNodeOperations`, `DRAResourcePoolStatus` | alpha |
| HPA scale-to-zero on **CPU/memory** metrics | — | never supported: `must specify at least one Object or External metric to support scaling to zero replicas` |

##### Deprecated or removed older APIs and features — flag them

- **Removed beta API versions** (not served by any supported release): `flowcontrol.apiserver.k8s.io/v1beta3` (removed 1.32), `v1beta2` (1.29); `storage.k8s.io/v1beta1` CSIStorageCapacity (1.27); `autoscaling/v2beta2` (1.26); `batch/v1beta1` CronJob, `discovery.k8s.io/v1beta1`, `events.k8s.io/v1beta1`, `autoscaling/v2beta1`, `policy/v1beta1` PDB and PodSecurityPolicy, `node.k8s.io/v1beta1` (1.25); `extensions/v1beta1` and `networking.k8s.io/v1beta1` Ingress, `admissionregistration.k8s.io/v1beta1` webhooks, `apiextensions.k8s.io/v1beta1` CRDs, `rbac.authorization.k8s.io/v1beta1`, `certificates.k8s.io/v1beta1`, `coordination.k8s.io/v1beta1`, `scheduling.k8s.io/v1beta1` PriorityClass (1.22).[^8]
- **`scheduling.k8s.io/v1alpha2`** removed in 1.37.[^2]
- **`v1` Endpoints** is deprecated since 1.33; every request returns `Warning: v1 Endpoints is deprecated in v1.33+; use discovery.k8s.io/v1 EndpointSlice` (verified on 1.37). Controllers must read EndpointSlices.
- **PodSecurityPolicy** (gone since 1.25) — use Pod Security Admission or VAP.
- **AppArmor annotations** `container.apparmor.security.beta.kubernetes.io/*` — deprecated; use `securityContext.appArmorProfile` (GA 1.31).
- **`kubectl run -f`** and **kube-dns** — deprecated in 1.37; **kube-proxy `ipvs` mode** — deprecated since 1.35, warns at startup and is behind the deprecated `KubeProxyIPVS` gate since 1.37.[^1][^2]
- **controller-runtime**: `Result.Requeue`, `GetEventRecorderFor`, `admission.CustomDefaulter`/`CustomValidator`, `WithCustomDefaulter`/`WithCustomValidator`, `client.Apply` patch type — deprecated (see below).
- **client-go**: untyped workqueues (`workqueue.New`, `NewRateLimitingQueue`, `DefaultControllerRateLimiter`, …), `cache.ListFunc`/`WatchFunc` without context, `resourcelock.MultiLock` (deprecated in 1.37).

---

#### Version-Aware Review

- **Determine the target.** The cluster version comes from `k8s.io/api` in `go.mod` (`v0.37.x` ⇒ 1.37), `ENVTEST_K8S_VERSION`, `kindest/node:v1.37.x`, `kubectl` in `.tool-versions`/`mise.toml`, Helm `kubeVersion:` constraints, or a managed-cluster version in IaC. Client libraries `v0.N` pair with Kubernetes `1.N`.
- **Skew.** kubectl supports ±1 minor against the API server; kubelets may be up to three minors older than kube-apiserver (1.34–1.37 nodes on a 1.37 control plane).[^32] A manifest that relies on a *kubelet* feature (sidecars, user namespaces, resize, image volumes) only works on nodes new enough to implement it, even if the API server accepts the field.
- **Verify before claiming "not available in 1.37".** Use the server: `kubectl explain <type>.<field>` (field exists), `kubectl api-resources` / `kubectl api-versions` (group/version served), `kubectl apply --dry-run=server -o yaml` (field survives defaulting), and `kubectl get --raw /metrics | grep kubernetes_feature_enabled` (gate state). Do not rely on blog posts alone — e.g. the 1.37 changelog says `PodLevelResourceManagers` is enabled by default but the 1.37.0 binary reports it disabled.
- **Field existence ≠ feature enabled.** `kubectl explain` on 1.37 shows `emptyDir.mode`, `httpGet.protocol`, `evictionResponders`, `job.spec.scheduling` and Pod `schedulingGroup` even though their gates are off. The field description usually says "alpha" and names the gate; some descriptions are stale (the `clusterTrustBundle` projection still says "Alpha, gated by the ClusterTrustBundleProjection feature gate" although it is GA in 1.37).
- **Say "not available in 1.37"** only for items in [Do Not Use or Recommend](#do-not-use-or-recommend) or when the pinned server proves it. Say "requires feature gate X (alpha/beta-off in 1.37)" when a gate exists.
- **Library ≠ server.** A controller compiled against `k8s.io/api` v0.37 can construct structs with fields the target server ignores (alpha) or rejects (older server). Gate code paths on discovery or on documented minimum versions.
- **Go toolchain.** `k8s.io/*` v0.37 and controller-runtime v0.25 require Go ≥ 1.26.0 (`go 1.26.0` in their `go.mod`); with Go 1.25 the `go` command switches toolchains automatically (`GOTOOLCHAIN=auto`) or fails with `GOTOOLCHAIN=local`.

---

#### API Versions Served by 1.37

Default `kubectl api-versions` on kube-apiserver 1.37.0 (no `--runtime-config`):

```
admissionregistration.k8s.io/v1   apiextensions.k8s.io/v1    apiregistration.k8s.io/v1
apps/v1                           authentication.k8s.io/v1   authorization.k8s.io/v1
autoscaling/v1  autoscaling/v2    batch/v1                   certificates.k8s.io/v1
coordination.k8s.io/v1            discovery.k8s.io/v1        events.k8s.io/v1
flowcontrol.apiserver.k8s.io/v1   networking.k8s.io/v1       node.k8s.io/v1
policy/v1                         rbac.authorization.k8s.io/v1
resource.k8s.io/v1                scheduling.k8s.io/v1       storage.k8s.io/v1
storagemigration.k8s.io/v1        v1
```

Compared with 1.36.2 the only new group/version is `storagemigration.k8s.io/v1`; new resources in existing groups are `certificates.k8s.io/v1` `clustertrustbundles` and `podcertificaterequests`, and `resource.k8s.io/v1` `devicetaintrules`. Every served version is GA: since 1.24 new beta APIs are off by default, so a manifest using any `…/v1beta*` or `…/v1alpha*` group needs `--runtime-config` on the API server.

Notable resources by group (1.37 defaults):

| Group/version | Resources worth knowing |
|---|---|
| `admissionregistration.k8s.io/v1` | `validatingadmissionpolicies` + bindings (GA 1.30), `mutatingadmissionpolicies` + bindings (GA 1.36), webhook configurations |
| `resource.k8s.io/v1` | `deviceclasses`, `resourceclaims`, `resourceclaimtemplates`, `resourceslices` (GA 1.34), `devicetaintrules` (GA 1.37) |
| `networking.k8s.io/v1` | `ingresses`, `ingressclasses`, `networkpolicies`, `ipaddresses`, `servicecidrs` (GA 1.33) |
| `storage.k8s.io/v1` | `volumeattributesclasses` (GA 1.34), `csidrivers`, `csinodes`, `csistoragecapacities`, `storageclasses`, `volumeattachments` |
| `certificates.k8s.io/v1` | `certificatesigningrequests`, `clustertrustbundles` (GA 1.37), `podcertificaterequests` (GA 1.37) |
| `storagemigration.k8s.io/v1` | `storageversionmigrations` (GA 1.37) |

`metrics.k8s.io` (`v1beta1`, and `v1` from metrics-server releases that implement it) and `custom.metrics.k8s.io`/`external.metrics.k8s.io` are aggregated APIs served by add-ons, not by kube-apiserver.

---

#### Features Usable on 1.37 (GA 1.29–1.37, Beta-On)

Everything below is enabled on a default 1.37 cluster. "GA" = the gate is locked on (or already removed); "beta-on" = beta and enabled by default (a cluster admin *can* switch it off). Versions come from the feature-gate state reported by each API server build 1.29–1.37.

| Feature | Manifest / API surface | Status on 1.37 |
|---|---|---|
| CRD validation rules (CEL) | `x-kubernetes-validations` | GA 1.29 (gate removed) |
| ValidatingAdmissionPolicy | `admissionregistration.k8s.io/v1` | GA 1.30 |
| Webhook `matchConditions` | `webhooks[].matchConditions` | GA 1.30 |
| Pod scheduling gates | `spec.schedulingGates` | GA 1.30 |
| `minDomains` in topology spread | `topologySpreadConstraints[].minDomains` | GA 1.30 |
| HPA container resource metrics | `type: ContainerResource` | GA 1.30 |
| AppArmor fields | `securityContext.appArmorProfile` | GA 1.31 |
| Job pod failure policy, `PodDisruptionConditions` | `spec.podFailurePolicy` | GA 1.31 |
| PDB unhealthy pod eviction policy | `spec.unhealthyPodEvictionPolicy` | GA 1.31 |
| StatefulSet start ordinal | `spec.ordinals.start` | GA 1.31 |
| PV `lastPhaseTransitionTime` | `status.lastPhaseTransitionTime` | GA 1.31 |
| CRD field selectors | `versions[].selectableFields` | GA 1.32 |
| StatefulSet PVC auto-delete | `spec.persistentVolumeClaimRetentionPolicy` | GA 1.32 |
| Pod index label | `apps.kubernetes.io/pod-index`, `batch.kubernetes.io/job-completion-index` labels | GA 1.32 |
| LoadBalancer IP mode | `status.loadBalancer.ingress[].ipMode` | GA 1.32 |
| Strict CEL cost enforcement for VAP/webhooks | — | GA 1.32 |
| Bound SA token improvements (JTI, node binding) | token claims | GA 1.32–1.33 |
| **Sidecar containers** | init container `restartPolicy: Always` | GA 1.33 (gate removed in 1.37) |
| CRD validation ratcheting | unchanged invalid fields pass on update | GA 1.33 |
| Job `backoffLimitPerIndex`, `successPolicy` | `spec.backoffLimitPerIndex`, `spec.successPolicy` | GA 1.33 |
| `matchLabelKeys` in pod affinity, `nodeInclusionPolicy` in spread | `podAffinity…matchLabelKeys` | GA 1.33 |
| Service `trafficDistribution` (`PreferClose`, now a deprecated alias of `PreferSameZone`) | `spec.trafficDistribution` | GA 1.33 |
| Multiple Service CIDRs | `networking.k8s.io/v1` `ServiceCIDR`, `IPAddress` | GA 1.33 |
| Recursive read-only mounts | `volumeMounts[].recursiveReadOnly` | GA 1.33 |
| nftables kube-proxy mode | `mode: nftables` | GA 1.33 |
| **DRA core** | `resource.k8s.io/v1` | GA 1.34 |
| Job `podReplacementPolicy` | `spec.podReplacementPolicy` | GA 1.34 |
| `preStop`/`postStart` `sleep` action (incl. 0 s) | `lifecycle.preStop.sleep` | GA 1.34 |
| VolumeAttributesClass | `storage.k8s.io/v1`, PVC `volumeAttributesClassName` | GA 1.34 |
| Volume expansion failure recovery | shrink a failed expansion request | GA 1.34 |
| Relaxed env var names (any printable ASCII except `=`) | `env[].name` | GA 1.34 |
| Node swap (LimitedSwap) | kubelet config | GA 1.34 |
| Structured authentication configuration | `--authentication-config` | GA 1.34 |
| **In-place Pod resize** | `resizePolicy`, `pods/resize` subresource | GA 1.35 |
| `trafficDistribution: PreferSameZone` / `PreferSameNode` | `spec.trafficDistribution` | GA 1.35 |
| Job `managedBy` | `spec.managedBy` | GA 1.35 |
| `supplementalGroupsPolicy` | `securityContext.supplementalGroupsPolicy` | GA 1.35 |
| Pod `metadata.generation` / `status.observedGeneration` | Pod status | GA 1.35 |
| **MutatingAdmissionPolicy** | `admissionregistration.k8s.io/v1` | GA 1.36 |
| **User namespaces** | `spec.hostUsers: false` | GA 1.36 |
| **Image volumes** | `volumes[].image` | GA 1.36 |
| `procMount: Unmasked` | `securityContext.procMount` | GA 1.36 |
| `seLinuxChangePolicy` | `securityContext.seLinuxChangePolicy` | GA 1.36 |
| DRA prioritized list (`firstAvailable`), admin access | ResourceClaim requests | GA 1.36 |
| Pod certificates, ClusterTrustBundles, StorageVersionMigration, SELinuxMount, hostname override, relaxed Service names, HPA tolerance | see [What's New](#whats-new-in-137) | GA 1.37 |
| **Pod-level resources** | `spec.resources` | beta-on since 1.34 |
| In-place resize of Pod-level resources | `pods/resize` on `spec.resources` | beta-on since 1.36 |
| Container restart rules | container `restartPolicy`, `restartPolicyRules` | beta-on since 1.35 |
| Restart all containers on exit | `restartPolicyRules[].action: RestartAllContainers` | beta-on since 1.36 |
| Env files | `env[].valueFrom.fileKeyRef` | beta-on since 1.35 (GA in 1.38) |
| Deployment/ReplicaSet `status.terminatingReplicas` | status | beta-on since 1.35 |
| Mutable resources / scheduling directives of suspended Jobs | Job `spec.template` | beta-on since 1.36 |
| Strict IP/CIDR validation | rejects `010.0.0.0/8`, `10.0.0.1/8` | beta-on since 1.36 (GA in 1.38) |
| Image pull credential verification | kubelet `imagePullCredentialsVerificationPolicy` | beta-on since 1.35 |
| Kubelet CrashLoopBackOff max | kubelet `crashLoopBackOff.maxContainerRestartPeriod` | beta-on since 1.35 |
| HPA scale to zero, StatefulSet `maxUnavailable`, PVC `Unused` condition | see [What's New](#whats-new-in-137) | beta-on in 1.37 |

---

#### Foundational Principles

| Principle | Description |
|---|---|
| **Level-based reconciliation** | React to current state, never to specific event types. The reconcile function must not receive or depend on event type information. |
| **Idempotency** | Every reconcile call must be safe to run repeatedly with the same outcome. |
| **Single responsibility** | One operator per application. One controller per root Kind. |
| **Declarative intent** | `spec` expresses *desired state*; the controller drives *actual state* toward it. |
| **Avoid edge-triggering** | Never build logic that depends on "what changed". Always reconcile the full object state. |
| **Open World assumption** | The system may miss events; design controllers that recover fully from a cold start. |
| **Caches are eventually consistent** | An informer may lag behind your own writes (seconds to minutes under load). Never conclude "object does not exist" or "my update was lost" from one cache read right after a write; requeue or use optimistic concurrency. 1.37 adds read-your-writes mitigation to several built-in controllers for exactly this reason.[^36] |
| **The API server pushes back** | Expect HTTP 409 (conflict), 429 (APF / watch-cache initialisation) and 5xx; retry with backoff and honour `Retry-After`. |

---

#### Project Layout & Tooling

##### Recommended Scaffold (Kubebuilder v4.16)

```
├── api/
│   └── v1/
│       ├── rediscluster_types.go      # API types + markers
│       ├── groupversion_info.go
│       └── zz_generated.deepcopy.go   # generated
├── internal/
│   ├── controller/
│   │   └── rediscluster_controller.go # Reconciler
│   └── webhook/v1/
│       └── rediscluster_webhook.go    # typed admission.Validator[T] / Defaulter[T]
├── cmd/main.go
├── config/
│   ├── crd/bases/                     # generated CRDs
│   ├── rbac/                          # generated RBAC
│   ├── webhook/                       # generated webhook configuration
│   └── default/                       # Kustomize overlay
├── Dockerfile
└── Makefile                           # make generate, make manifests, make test
```

##### Toolchain pinned for 1.37

| Tool | Version that pairs with 1.37 | Notes |
|---|---|---|
| Go | ≥ 1.26.0 | required by `k8s.io/*` v0.37 and controller-runtime v0.25 |
| `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` | v0.37.1 | keep all `k8s.io/*` staging modules on the **same** minor and patch |
| `sigs.k8s.io/controller-runtime` | v0.25.x (v0.25.2) | v0.24 is built on v0.36; mixing v0.24 with v0.37 does not compile cleanly |
| `sigs.k8s.io/controller-tools` (`controller-gen`) | v0.22.0 or v0.23.0 | both built on v0.37 |
| Kubebuilder | v4.16.0 | scaffold also pins golangci-lint v2.13.1 and kustomize v5.8.1 |
| envtest assets | `ENVTEST_K8S_VERSION=1.37` (1.37.0) | Kubebuilder derives it from the `k8s.io/api` minor |

```makefile
make generate      # DeepCopy, runtime.Object implementations (controller-gen object)
make manifests     # CRD YAML, RBAC, WebhookConfiguration (controller-gen crd rbac webhook)
make install       # Apply CRDs to cluster
make test          # Unit + envtest integration tests
make docker-build  # Build operator image
```

```go
// go.mod of a 1.37 operator
module example.com/redis-operator

go 1.26.0

require (
	k8s.io/api v0.37.1
	k8s.io/apimachinery v0.37.1
	k8s.io/client-go v0.37.1
	sigs.k8s.io/controller-runtime v0.25.2
)
```

Review rules:
- Flag `k8s.io/*` modules on different minors (e.g. `k8s.io/api v0.37.1` with `k8s.io/client-go v0.36.5`), and controller-runtime/controller-tools versions that do not pair with the `k8s.io/*` minor.
- Flag `replace k8s.io/... => ...` pins left behind from earlier upgrades.
- Generated files (`zz_generated.deepcopy.go`, `config/crd/bases/*.yaml`, `config/rbac/role.yaml`) must be regenerated in the same change as the Go types/markers they derive from.

---

#### Naming Conventions

##### Character Rules by Name Class

| Name class | Used for | Allowed characters | Start/end | Max length |
|---|---|---|---|---|
| **DNS Subdomain** (RFC 1123) | API groups, most `metadata.name` values (Pods, Deployments, ConfigMaps, CRs), CRD `spec.group` | lowercase `a–z`, digits, `-`, `.` | alphanumeric at both ends | 253 |
| **DNS Label** (RFC 1123) | `metadata.namespace`, container/port names, **Service names (1.37)** | lowercase `a–z`, digits, `-` | alphanumeric at both ends (may start with a digit) | 63 |
| **DNS Label** (RFC 1035) | a few legacy fields; **Service names before the relaxed validation** (1.35 and older) | lowercase `a–z`, digits, `-` | must start with `[a-z]` | 63 |
| **Qualified Name** (label/annotation key, finalizer) | label keys, annotation keys, finalizer names | name part: `A–Z a–z 0–9 - _ .`; optional DNS-subdomain prefix + `/` | name part alphanumeric at both ends | name ≤ 63, prefix ≤ 253 |
| **Label Value** | `metadata.labels` values | `A–Z a–z 0–9 - _ .` or empty | alphanumeric at both ends if non-empty | 63 |
| **Annotation Value** | annotation values | any UTF-8 | — | all annotations together ≤ 256 KiB |
| **Env var name** | `env[].name` | **any printable ASCII except `=`** (GA 1.34; before: C identifier) | — | — |
| **ConfigMap / Secret data key** | `data` / `stringData` keys | `A–Z a–z 0–9 - _ .` | — | 253 |
| **Path Segment Name** | names exposed in URL paths | DNS subdomain, and not `.` or `..` | — | 253 |

> ⚠️ `_` and uppercase are **never** valid in DNS subdomain or DNS label names (`metadata.name`, namespaces, API groups, Service names). They are valid in label keys (name part), label values and data keys.

##### Service names [NEW in 1.37]

`RelaxedServiceNameValidation` is GA in 1.37 (beta-on since 1.36): a Service name only has to be an RFC 1123 label. Verified on 1.37: `1st-api` is accepted, `api.v2` is rejected with `must not contain dots`. On 1.35 and older (verified on 1.33) `1st-api` is rejected as an invalid DNS-1035 label. Only flag a digit-leading Service name when the manifests must also deploy to ≤ 1.35 clusters (Helm charts with `kubeVersion: ">=1.33"` etc.).

##### API Group, Kind, Resource

| Element | Rule | Example |
|---|---|---|
| **API Group** | Lowercase DNS subdomain you own | `cache.example.com` |
| **Kind** | CamelCase, singular | `RedisCluster` |
| **Resource (plural)** | Lowercase, plural | `redisclusters` |
| **Short name** | 2–5 lowercase chars; must **not** collide with built-ins | `rdc` |
| **Category** | Lowercase; register in `.spec.names.categories` | `cache` (avoid `all`) |

> ⚠️ Do **not** use the empty group, single-word groups, or `*.k8s.io` / `*.kubernetes.io` — those are reserved by the Kubernetes project.

```yaml
# BAD: "rc" is already the short name of ReplicationController
names: {kind: RedisCluster, plural: redisclusters, shortNames: [rc]}
```

With that CRD installed on 1.37, `kubectl get rc demo` prints `Warning: short name "rc" could also match lower priority resource redisclusters.cache.example.com` and then looks up a **ReplicationController** (`replicationcontrollers "demo" not found`). Built-in short names always win. Check `kubectl api-resources` before choosing one.

##### Go Type Names

```go
// ✅ Correct
type RedisCluster struct{ /* ... */ }
type RedisClusterSpec struct{ /* ... */ }
type RedisClusterStatus struct{ /* ... */ }
type RedisClusterList struct{ /* ... */ }

// ❌ Wrong
type rediscluster struct{ /* ... */ }
type RedisClusterSpecs struct{ /* ... */ }
```

##### JSON Field Names

| Rule | ✅ Good | ❌ Bad |
|---|---|---|
| camelCase | `replicaCount` | `replica_count`, `ReplicaCount` |
| Declarative nouns (desired state) | `replicas` | `setReplicas` |
| Name references | `secretName` | `secretRef.name` (unless cross-namespace) |
| Object references | `secretRef` | `secret` |
| Duration | `timeoutSeconds` (`int32`) | `timeout: "30s"` in spec primitive fields |
| No novel abbreviations; `max`/`min` prefixes and `url`, `ip`, `id` are established | `maxRetries`, `minReadySeconds` | `tgtUtilization`, `cfgMapName` |

##### Constant (Enum) Values

```go
// CamelCase string alias — never Go iota enums in API types
// +kubebuilder:validation:Enum=Pending;Running;Failed
type PhaseType string

const (
	PhasePending PhaseType = "Pending"
	PhaseRunning PhaseType = "Running"
	PhaseFailed  PhaseType = "Failed"
)
```

##### Labels & Annotations

```yaml
metadata:
  labels:
    app.kubernetes.io/name: redis
    app.kubernetes.io/instance: cache-prod
    app.kubernetes.io/component: cache
    app.kubernetes.io/part-of: checkout
    app.kubernetes.io/managed-by: redis-operator
  annotations:
    cache.example.com/last-backup-time: "2026-10-01T00:00:00Z"
```

When code uses `client.ObjectKey`, assume `Namespace` and `Name` already satisfy the DNS rules above unless they come from user input.

---

#### YAML Pitfalls & KYAML

`kubectl` (and Helm, Kustomize) parse YAML with YAML 1.1 rules through `sigs.k8s.io/yaml`. Unquoted `yes`, `no`, `on`, `off`, `y`, `n`, `true`, `false` become **booleans**, `1.10` becomes the float `1.1`, and `0700` is octal (448). Kubernetes string fields then reject the value. Verified on 1.37:

```yaml
# BAD: ConfigMap data values must be strings
apiVersion: v1
kind: ConfigMap
metadata: {name: flags}
data:
  enabled: yes          # → bool → "json: cannot unmarshal bool into Go struct field ConfigMap.data of type string"
  country: NO           # → bool (the "Norway problem")
  version: 1.10         # → number 1.1 → "cannot unmarshal number into Go struct field ConfigMap.data of type string"
```

```yaml
# GOOD: quote every scalar that must stay a string
apiVersion: v1
kind: ConfigMap
metadata: {name: flags}
data:
  enabled: "yes"
  country: "NO"
  version: "1.10"
```

The same applies to label and annotation values (`"true"`, `"1"`), env var `value:` fields, and Helm values that end up in those fields (`{{ .Values.flag | quote }}`).

##### KYAML (stable in 1.37) [NEW in 1.37]

KYAML is a strict, unambiguous subset of YAML (always-quoted strings, flow style, trailing commas). Every KYAML document is valid YAML, so any kubectl/Helm/Kustomize version reads it.[^30]

```
$ kubectl create cm flags --from-literal=enabled=yes --from-literal=country=NO --dry-run=client -o kyaml
---
{
  kind: "ConfigMap",
  apiVersion: "v1",
  metadata: {
    name: "flags",
  },
  data: {
    country: "NO",
    enabled: "yes",
  },
}
```

`-o kyaml` already works in kubectl 1.35/1.36 (beta; kubectl 1.34 rejects the format without the alpha opt-in); 1.37 makes it stable. Do not ask authors to rewrite existing YAML; recommend KYAML for generated output and for hand-written files that keep hitting the YAML 1.1 traps.

---

#### Pods and Workloads

##### A production Pod template for 1.37

All fields below are GA or beta-on in 1.37; this exact Pod passes `kubectl apply --dry-run=server` on a default 1.37.0 API server (QoS class `Burstable`). Most workloads need only a subset — use it as a reference for field placement and defaults.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: web
  namespace: default
spec:
  hostUsers: false                         # user namespace (GA 1.36)
  hostnameOverride: web-primary            # GA 1.37
  automountServiceAccountToken: false
  securityContext:
    runAsNonRoot: true
    runAsUser: 10001
    runAsGroup: 10001
    fsGroup: 10001
    seccompProfile: {type: RuntimeDefault}
    seLinuxChangePolicy: MountOption       # value accepted only with SELinuxMount on (default since 1.37)
    supplementalGroupsPolicy: Strict       # GA 1.35
  resources:                               # Pod-level resources (beta-on since 1.34)
    requests: {cpu: "1", memory: 512Mi}
    limits: {memory: 1Gi}
  initContainers:
    - name: log-shipper                    # sidecar (GA 1.33)
      image: registry.example.com/shipper@sha256:0000000000000000000000000000000000000000000000000000000000000000
      restartPolicy: Always
      resources:
        requests: {cpu: 100m, memory: 64Mi}
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities: {drop: [ALL]}
  containers:
    - name: app
      image: registry.example.com/app@sha256:0000000000000000000000000000000000000000000000000000000000000000
      ports: [{name: http, containerPort: 8080}]
      resizePolicy:                        # in-place resize (GA 1.35)
        - {resourceName: cpu, restartPolicy: NotRequired}
        - {resourceName: memory, restartPolicy: RestartContainer}
      lifecycle:
        preStop:
          sleep: {seconds: 5}              # GA 1.34; no shell needed in the image
      readinessProbe:
        httpGet: {path: /readyz, port: http}
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities: {drop: [ALL]}
      volumeMounts:
        - {name: model, mountPath: /models, readOnly: true}
        - {name: tls, mountPath: /var/run/tls, readOnly: true}
  volumes:
    - name: model                          # image volume (GA 1.36)
      image:
        reference: registry.example.com/models/llm@sha256:0000000000000000000000000000000000000000000000000000000000000000
        pullPolicy: IfNotPresent
    - name: tls
      projected:
        sources:
          - podCertificate:                # GA 1.37
              signerName: example.com/workload-identity
              keyType: ECDSAP256
              credentialBundlePath: credentialbundle.pem
          - clusterTrustBundle:            # GA 1.37
              signerName: example.com/workload-identity
              labelSelector: {matchLabels: {example.com/bundle: current}}
              path: trust-anchors.pem
```

On 1.36 the same Pod is rejected (`spec.securityContext.seLinuxChangePolicy: Unsupported value: "MountOption": supported values: "Recursive"`); with `Recursive` it is accepted but both projected sources are dropped (`Warning: volume "tls" (Projected) has no sources provided`) because Pod certificates and ClusterTrustBundle projection are beta-off there.

Note: `kubectl explain pod.spec.resources` on 1.37 still says "This is an alpha field and requires enabling the PodLevelResources feature gate"; the gate is beta and **on** since 1.34. Trust the gate metric, not stale field descriptions.

##### Sidecar containers (GA 1.33)

A sidecar is an **init container with `restartPolicy: Always`**. It starts before later init containers and the app containers (once its `startupProbe` succeeds, if it has one), keeps running for the life of the Pod, is restarted independently, and is stopped **after** the app containers when the Pod terminates. Sidecars do not block Job completion.[^17]

```yaml
# BAD: "sidecar" as a regular container — no start ordering, keeps a Job Pod alive forever,
# and gets SIGTERM at the same time as the app (log loss on shutdown)
spec:
  containers:
    - name: app
      image: registry.example.com/batch:1.0
    - name: log-shipper
      image: registry.example.com/shipper:2.3
```

```yaml
# GOOD
spec:
  initContainers:
    - name: log-shipper
      image: registry.example.com/shipper:2.3
      restartPolicy: Always
      startupProbe:
        httpGet: {path: /ready, port: 9000}
  containers:
    - name: app
      image: registry.example.com/batch:1.0
```

Review points:
- On an init container, `restartPolicy: Always` is what makes it a sidecar; a plain init container leaves `restartPolicy` unset. Per-container `restartPolicy` on regular containers is a separate, beta feature (container restart rules, below) — do not confuse the two.
- Sidecar resources count towards the Pod's effective request together with the app containers (unlike classic init containers, whose request is a max).
- In-place resize also covers sidecars (`InPlacePodVerticalScalingInitContainers`, beta-on in 1.36, GA in 1.37).
- Do not gate on the `SidecarContainers` feature gate: it was removed in 1.37 (passing it to a component now fails).

##### In-place resize (GA 1.35)

CPU and memory of running Pods are changed through the **`resize` subresource**, never through a normal Pod update. Verified on 1.37:

```bash
# GOOD: strategic-merge patch against the resize subresource
kubectl patch pod resize-demo --subresource resize \
  -p '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"800m"},"limits":{"cpu":"1500m"}}}]}}'

# BAD: a normal update is rejected
#   "pod updates may not change fields other than `spec.containers[*].image`, ..."
kubectl patch pod resize-demo -p '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"900m"}}}]}}'

# BAD: --type merge replaces the whole containers list → "resource requests cannot be removed"
kubectl patch pod resize-demo --subresource resize --type merge \
  -p '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"800m"}}}]}}'
```

From controllers, update the `resize` subresource: `c.SubResource("resize").Update(ctx, pod)` (controller-runtime) or `clientset.CoreV1().Pods(ns).UpdateResize(ctx, pod, opts)` (client-go). Each resize increments `metadata.generation`; progress is reported through the Pod conditions **`PodResizePending`** (reason `Deferred` or `Infeasible`) and **`PodResizeInProgress`**, with `observedGeneration` on each condition. `status.resize` is deprecated — do not read it.[^18]

Limits in 1.37: only `cpu` and `memory`; the QoS class cannot change (Guaranteed stays requests == limits; BestEffort cannot gain resources); requests/limits can be changed but not removed; non-restartable init containers and ephemeral containers cannot be resized; Windows Pods, Pods under static CPU/Memory manager policies and memory resizes of Pods using swap (unless `RestartContainer`) are not resizable. A memory *decrease* with `NotRequired` is best-effort.[^18] Scheduler preemption to satisfy a `Deferred` resize is alpha (`InPlacePodVerticalScalingSchedulerPreemption`).

Workload controllers (Deployment, StatefulSet, …) still roll Pods when the template changes; in-place resize is for direct Pod patches by VPA-like controllers or operators. Flag operators that call `Update` on running Pods to change resources.

##### Pod-level resources (beta-on since 1.34)

`spec.resources` sets a CPU/memory/hugepages budget for the whole Pod; containers without their own requests/limits share it. Pod-level values must be ≥ the sum of container-level values. In 1.37:

- An **empty** `spec.resources` (`{}`, `{requests: {}}`, `{limits: {}}`) no longer affects the QoS class (changed in 1.37).[^2] Verified: a Pod with `resources: {requests: {}}` at Pod level and containers with requests == limits is `BestEffort` on 1.36 but `Guaranteed` on 1.37.
- In-place resize of Pod-level resources is beta-on since 1.36.
- Pod-level CPU/memory/topology **managers** (`PodLevelResourceManagers`) are beta but **off** — do not assume NUMA-aligned exclusive CPUs for Pod-level budgets.
- ResourceClaims cannot be expressed in `spec.resources`.

##### User namespaces (GA 1.36)

`spec.hostUsers: false` runs the Pod in its own user namespace: UID 0 in the container maps to an unprivileged UID on the host, so a container breakout does not yield host root. Requirements live on the node (Linux ≥ 6.3 with idmap mounts on the volume filesystems, a container runtime that supports it).[^19] It cannot be combined with `hostNetwork`, `hostPID`, `hostIPC` (verified: `spec.hostNetwork: Forbidden: when `hostUsers` is false`) or raw block `volumeDevices`. Recommend it for workloads that must run as root inside the container: Pod Security Admission does not check `runAsNonRoot`/`runAsUser` for such Pods even under `restricted` (root in the container is never root on the host).[^19]

##### Image volumes (GA 1.36)

`volumes[].image` mounts an OCI image/artifact read-only (models, static assets, plugins) without baking it into the app image.

```yaml
volumes:
  - name: model
    image:
      reference: registry.example.com/models/llm@sha256:0000000000000000000000000000000000000000000000000000000000000000
      pullPolicy: IfNotPresent
```

- Pin by digest; `pullPolicy` follows the same rules as container images.
- The mount is always read-only and the kubelet needs the image pull credentials (Pod `imagePullSecrets` or node credentials).
- **1.37 rejects an empty `image.reference` in Pod templates** of Deployments, StatefulSets, DaemonSets, Jobs, etc. (`spec.template.spec.volumes[0].image.reference: Required value`; 1.36 accepted it).[^2] Helm templates that render `reference: {{ .Values.model.image }}` with an empty default now fail at apply time — use `required`.

##### Container restart rules (beta-on since 1.35)

Regular containers may override the Pod `restartPolicy`, and `restartPolicyRules` restart a container (or, beta-on since 1.36, all containers) on specific exit codes — useful for "retryable" exit codes in batch/ML jobs without restarting the whole Pod.

```yaml
apiVersion: v1
kind: Pod
metadata: {name: trainer, namespace: default}
spec:
  restartPolicy: Never
  containers:
    - name: worker
      image: registry.example.com/trainer:1.0
      restartPolicy: Never
      restartPolicyRules:
        - action: Restart                 # or RestartAllContainers (beta-on since 1.36)
          exitCodes: {operator: In, values: [42]}
```

These are beta: they work on default 1.37 clusters, but nodes older than 1.35 ignore them (kubelet skew).

##### Env files (beta-on since 1.35; GA only in 1.38)

`env[].valueFrom.fileKeyRef` reads a variable from an env file that an init container writes into an `emptyDir`; the consuming container does not need to mount the volume.

```yaml
apiVersion: v1
kind: Pod
metadata: {name: envfile, namespace: default}
spec:
  initContainers:
    - name: render-env
      image: registry.example.com/config-renderer:1.0
      volumeMounts: [{name: config, mountPath: /data}]
  containers:
    - name: app
      image: registry.example.com/app:1.0
      env:
        - name: DB_ADDRESS
          valueFrom:
            fileKeyRef: {volumeName: config, path: config.env, key: DB_ADDRESS, optional: false}
  volumes:
    - name: config
      emptyDir: {}
```

Do not use env files for secrets: `emptyDir` content has none of the protections of a Secret (the upstream docs say so explicitly).

##### Probes, lifecycle and termination

```yaml
containers:
  - name: app
    image: registry.example.com/app:1.0
    startupProbe:                       # protects slow starts; liveness starts after it succeeds
      httpGet: {path: /healthz, port: 8080}
      failureThreshold: 30
      periodSeconds: 2
    readinessProbe:
      httpGet: {path: /readyz, port: 8080}
    livenessProbe:
      httpGet: {path: /healthz, port: 8080}
      periodSeconds: 10
      timeoutSeconds: 2
    lifecycle:
      preStop:
        sleep: {seconds: 10}            # let endpoints/LB drain before SIGTERM
terminationGracePeriodSeconds: 45       # must exceed preStop sleep + shutdown time
```

- Liveness probes must not depend on downstream services; readiness may.
- `preStop.sleep` (GA 1.34, zero allowed) replaces `exec: ["sleep","10"]`, which fails in distroless images.
- HTTP/2 cleartext probes (`httpGet.protocol: HTTP2`) and gRPC probe TLS are **alpha** in 1.37 — use `grpc:` probes (plaintext) or plain HTTP/1.1.
- Container `lifecycle.stopSignal` is still alpha (`ContainerStopSignals`) — set the stop signal in the image (`STOPSIGNAL`) instead.
- 1.37 fixes a 1.35 regression where exec readiness probes stopped running once a Pod began terminating.[^2]

##### Deployments

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: default
  labels: {app.kubernetes.io/name: api}
spec:
  replicas: 3
  revisionHistoryLimit: 5
  selector:
    matchLabels: {app.kubernetes.io/name: api}
  strategy:
    type: RollingUpdate
    rollingUpdate: {maxSurge: 25%, maxUnavailable: 0}
  template:
    metadata:
      labels: {app.kubernetes.io/name: api}
    spec:
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: topology.kubernetes.io/zone
          whenUnsatisfiable: ScheduleAnyway
          labelSelector:
            matchLabels: {app.kubernetes.io/name: api}
          matchLabelKeys: [pod-template-hash]   # spread each ReplicaSet revision separately
      containers:
        - name: api
          image: registry.example.com/api@sha256:0000000000000000000000000000000000000000000000000000000000000000
          resources:
            requests: {cpu: 250m, memory: 256Mi}
            limits: {memory: 256Mi}
```

- `spec.selector` is immutable; template labels must match it.
- Do not set `spec.replicas` when an HPA manages the Deployment — every `kubectl apply`/GitOps sync resets the replica count.
- `status.terminatingReplicas` (beta-on since 1.35) shows Pods still shutting down; dashboards that compare `replicas` with ready Pods should account for it.

##### StatefulSets

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata: {name: db, namespace: default}
spec:
  serviceName: db
  replicas: 3
  podManagementPolicy: Parallel
  selector: {matchLabels: {app: db}}
  updateStrategy:
    type: RollingUpdate
    rollingUpdate: {maxUnavailable: 1}      # honoured again in 1.37 (beta-on)
  persistentVolumeClaimRetentionPolicy:     # GA 1.32
    whenDeleted: Retain
    whenScaled: Delete
  template:
    metadata: {labels: {app: db}}
    spec:
      containers:
        - name: db
          image: registry.example.com/db:1.0
          volumeMounts: [{name: data, mountPath: /var/lib/db}]
  volumeClaimTemplates:
    - metadata: {name: data}
      spec:
        accessModes: [ReadWriteOncePod]
        resources: {requests: {storage: 10Gi}}
```

- **`maxUnavailable` [NEW in 1.37]**: on in 1.35, switched off in 1.36 because a broken first revision could leave a Pod stuck forever, re-enabled in 1.37 with the fix.[^1] On a 1.36 API server the field is silently **dropped** from the stored object (verified), so rollouts proceed one Pod at a time.
- `updateStrategy.type: Recreate` is **alpha** in 1.37 and rejected on default clusters (`must be 'RollingUpdate' or 'OnDelete'`).
- With `OnDelete`, 1.37 fixes `status.currentRevision` never catching up to `updateRevision`; tooling that waited on that equality previously hung.[^2]
- The headless Service named in `serviceName` must exist; `apps.kubernetes.io/pod-index` labels (GA 1.32) are set on Pods.

##### Jobs and CronJobs

```yaml
apiVersion: batch/v1
kind: Job
metadata: {name: shard-import, namespace: default}
spec:
  completionMode: Indexed
  completions: 10
  parallelism: 5
  backoffLimitPerIndex: 2          # GA 1.33: retries per index, not per Job
  maxFailedIndexes: 3
  podReplacementPolicy: Failed     # GA 1.34: no replacement while the old Pod is still terminating
  successPolicy:                   # GA 1.33
    rules:
      - succeededIndexes: "0"      # leader index decides success
  podFailurePolicy:                # GA 1.31
    rules:
      - action: FailIndex
        onExitCodes: {containerName: importer, operator: In, values: [42]}
      - action: Ignore
        onPodConditions: [{type: DisruptionTarget}]
  ttlSecondsAfterFinished: 3600
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: importer
          image: registry.example.com/importer:1.0
```

- `podFailurePolicy` requires `restartPolicy: Never`.
- `spec.managedBy` (GA 1.35) hands a Job to an external controller (e.g. MultiKueue); the built-in Job controller then ignores it. It is immutable.
- Suspended Jobs (`suspend: true`) may have their Pod template resources and scheduling directives changed (beta-on since 1.36); 1.37 fixes a 1.36 regression that rejected such edits before the `JobSuspended` condition was set.[^2]
- `spec.scheduling` (gang scheduling for Jobs) is **alpha** in 1.37 (`WorkloadWithJob`).
- CronJobs: always set `concurrencyPolicy`, `startingDeadlineSeconds` and `timeZone` (IANA name) explicitly.
- One-shot Jobs/CronJobs may rely on structured logs and exit status when there is no owned object status to update; idempotent Jobs can rely on termination and safe reruns. A job that holds a lease, lock, transaction or external reservation must release it on SIGTERM; external calls still need timeouts or `activeDeadlineSeconds`.

##### PodDisruptionBudgets

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata: {name: api, namespace: default}
spec:
  maxUnavailable: 1
  unhealthyPodEvictionPolicy: AlwaysAllow   # GA 1.31: unhealthy Pods never block drains
  selector:
    matchLabels: {app.kubernetes.io/name: api}
```

`policy/v1`: an empty `selector: {}` selects **all** Pods in the namespace. Eviction `Forbidden` errors caused by PDBs now carry structured `CauseType` values (1.37), so tools can stop string-matching messages.[^2]

---

#### Services, Networking and Traffic

```yaml
apiVersion: v1
kind: Service
metadata: {name: api, namespace: default}
spec:
  selector: {app.kubernetes.io/name: api}
  ports:
    - name: http
      port: 80
      targetPort: http                  # named port: survives containerPort changes
  trafficDistribution: PreferSameZone   # GA (PreferSameZone/PreferSameNode since 1.35)
```

- **`trafficDistribution`**: `PreferSameZone` keeps traffic in the client's zone when local endpoints exist; `PreferSameNode` prefers node-local endpoints, falling back to zone then cluster; `PreferClose` is the older, deprecated alias of `PreferSameZone`. If the `service.kubernetes.io/topology-mode: Auto` annotation is also set, the annotation wins.[^35] Same-zone routing does not balance load across zones — flag it on Services whose endpoints are not spread evenly.
- **EndpointSlices, not Endpoints**: `v1` Endpoints is deprecated since 1.33 and every access returns a deprecation warning. Since 1.37 the `services/proxy` subresource also routes via EndpointSlices.[^2] Controllers must watch `discovery.k8s.io/v1` EndpointSlices (label `kubernetes.io/service-name`), which may be several per Service.
- **Service names** may start with a digit on 1.36+ (see [Naming](#naming-conventions)).
- **Strict IP/CIDR validation** (beta-on since 1.36) rejects ambiguous values in new objects. Verified on 1.37 with a NetworkPolicy `ipBlock`:
  - `10.0.0.1/8` → `must not have bits set beyond the prefix length`
  - `010.0.0.0/8` → `must not have leading 0s in IP or prefix length`

```yaml
# GOOD: canonical CIDRs
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: egress-internal, namespace: default}
spec:
  podSelector: {}
  policyTypes: [Egress]
  egress:
    - to: [{ipBlock: {cidr: 10.0.0.0/8}}]
```

- **Multiple Service CIDRs** (GA 1.33): cluster admins extend the Service IP range by creating `networking.k8s.io/v1` `ServiceCIDR` objects; hard-coded assumptions about a single Service range in controllers or NetworkPolicies are wrong.
- **LoadBalancer `ipMode`** (GA 1.32): `status.loadBalancer.ingress[].ipMode: Proxy` tells kube-proxy not to short-circuit traffic to the LB IP; cloud controllers should set it when the LB terminates/proxies.
- **kube-proxy mode**: set `mode:` explicitly (`nftables` is GA since 1.33; 1.37 warns when no mode is set because the default will change from `iptables` to `nftables`). `ipvs` is deprecated since 1.35 (1.37 adds the `KubeProxyIPVS` gate; expected default-off in 1.40, removal in 1.43).[^1] In nftables mode, `localhost:<NodePort>` does not work unless the opt-in userspace proxy is enabled (alpha `KubeProxyNFTablesLocalhostNodePorts` gate plus a loopback address in `--nodeport-addresses`; the gate becomes beta/on only in 1.38).
- **NetworkPolicy** is namespaced and additive; an empty `podSelector: {}` with `policyTypes: [Ingress]` and no rules denies all ingress. Remember DNS egress (UDP+TCP 53 to the cluster DNS) when adding default-deny egress.

---

#### Autoscaling

##### HPA scale to zero (beta-on in 1.37) [NEW in 1.37]

`minReplicas: 0` is accepted on 1.37 when at least one metric is of type `Object` or `External`. Verified on 1.37 (and rejected on 1.36, where the gate is alpha/off):

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: {name: worker, namespace: default}
spec:
  scaleTargetRef: {apiVersion: apps/v1, kind: Deployment, name: worker}
  minReplicas: 0
  maxReplicas: 20
  metrics:
    - type: External
      external:
        metric:
          name: queue_messages_ready
          selector: {matchLabels: {queue: jobs}}
        target: {type: AverageValue, averageValue: "30"}
  behavior:
    scaleUp:
      tolerance: "0.05"        # GA 1.37; stored as "50m"
    scaleDown:
      tolerance: "0.15"        # stored as "150m"
      stabilizationWindowSeconds: 300
```

- With only `Resource`/`ContainerResource` (CPU/memory) metrics, `minReplicas: 0` is rejected: `spec.metrics: Forbidden: must specify at least one Object or External metric to support scaling to zero replicas` — CPU/memory metrics need running Pods.
- While the HPA holds the target at zero it sets the condition **`ScaledToZero=True`**; it uses that condition to tell its own scale-to-zero apart from a workload an operator scaled to 0 by hand. After scaling back up the condition becomes `False` with reason `NotScaledToZero`.[^1][^23] Operators/dashboards should not treat `replicas: 0` as "disabled" without checking this condition.
- A Deployment at zero replicas has no Pods to receive traffic: front such workloads with a queue or an activator, not a Service that clients call synchronously.
- On a 1.36 cluster this manifest is rejected (`minReplicas` must be ≥ 1). Charts that support both must branch on `.Capabilities.KubeVersion`.

##### HPA tolerance (GA 1.37)

`behavior.scaleUp.tolerance` / `behavior.scaleDown.tolerance` (a Quantity ≥ 0) override the cluster-wide 10 % tolerance per direction. Write them as strings (`"0.05"`); the API server canonicalises them to milli-units (`50m`), which shows up as a perpetual diff in GitOps tools that compare raw manifests — write `50m` directly or configure the tool to ignore canonicalisation.

##### Other HPA rules

- Use `autoscaling/v2` (`v2beta2` was removed in 1.26).
- Prefer `ContainerResource` metrics (GA 1.30) when sidecars distort Pod-level CPU averages.
- Every container that the HPA measures by `Utilization` must declare a request for that resource.
- HPA conditions may now carry `observedGeneration` (beta-on 1.37); the HPA controller also reconciles new/changed HPAs immediately instead of waiting for the 15 s resync.[^2]
- Never set `spec.replicas` in a manifest whose target is HPA-managed.

---

#### Storage

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata: {name: data, namespace: default}
spec:
  accessModes: [ReadWriteOncePod]       # GA since 1.29: exactly one Pod
  storageClassName: fast-ssd
  volumeAttributesClassName: gold       # GA 1.34: mutable IOPS/throughput class
  resources:
    requests: {storage: 50Gi}
```

- **`ReadWriteOncePod`** guarantees a single Pod (RWO only guarantees a single *node*). Prefer it for single-writer databases.
- **VolumeAttributesClass** (`storage.k8s.io/v1`, GA 1.34): change performance parameters by switching `volumeAttributesClassName` on a bound PVC; `storageClassName` stays immutable.
- **PVC `Unused` condition** [NEW in 1.37] (beta-on): the PVC-protection controller sets `status.conditions[type=Unused]` to `True` (reason `NoPodsUsingPVC`) when no non-terminal Pod references the PVC and back to `False` (`PodUsingPVC`); `lastTransitionTime` is the "unused since" time. It slightly under-reports idle time and never deletes anything — cleanup tools must decide themselves.[^1]
- **Volume expansion recovery** (GA 1.34): after a failed expansion the requested size can be lowered again (not below the original size).
- **SELinux mount behaviour change** [NEW in 1.37]: see [Security Hardening](#security-hardening). CSI driver authors should set `CSIDriver.spec.seLinuxMount: true` only if the driver honours `-o context=`.
- **CSI drivers**: `CSIDriver.spec.preventPodSchedulingIfMissing` (beta-on in 1.37, `VolumeLimitScaling`) stops the scheduler from placing Pods on nodes where the driver has not registered yet.
- **Memory-backed `emptyDir`**: `sizeLimit` counts against the container memory limit; resizing it in place is alpha.
- 1.37 `kubectl cluster-info dump --output-directory` writes files `0600`/directories `0700` because dumped logs may contain secrets.[^2]

---

#### Workload Identity: Pod Certificates & ClusterTrustBundles

##### Pod certificates (GA 1.37) [NEW in 1.37]

A `podCertificate` projected-volume source makes the kubelet generate a key pair, create a `certificates.k8s.io/v1` **PodCertificateRequest** addressed to `signerName`, and write the issued key + chain into the Pod. The Pod does not start until each such source has a certificate; the kubelet rotates it from `status.beginRefreshAt`.[^21] A signer controller (yours or a third party) must watch PodCertificateRequests for its signer name, issue certificates and maintain ClusterTrustBundles.

```yaml
volumes:
  - name: tls
    projected:
      sources:
        - podCertificate:
            signerName: example.com/workload-identity
            keyType: ECDSAP256              # RSA3072 RSA4096 ECDSAP256 ECDSAP384 ECDSAP521 ED25519
            credentialBundlePath: credentialbundle.pem
            maxExpirationSeconds: 86400     # default 86400; min 3600; max 7862400
```

- Prefer **`credentialBundlePath`** (one PEM file: PKCS#8 key first, then the chain) over separate `keyPath` + `certificateChainPath`: two files can be read mid-rotation and not match.
- `kubernetes.io/*` signers never issue certificates longer than 24 h.
- `userAnnotations` are copied verbatim into `spec.unverifiedUserAnnotations` of the request; signers must reject keys they do not know.
- ML-DSA key types are **1.38** (beta there) — not available on 1.37.
- The v1 API has **no** `pkixPublicKey`/`proofOfPossession` fields (they were deprecated in v1beta1 and removed in v1); clients still setting them must be updated.[^2]
- NodeRestriction (1.37): a node may only create a PodCertificateRequest for a signer that one of its Pods actually mounts, unless authorised via `verb=request-podcertificate-signer` on the signer name.[^2]

##### ClusterTrustBundles (GA 1.37) [NEW in 1.37]

```yaml
apiVersion: certificates.k8s.io/v1
kind: ClusterTrustBundle
metadata:
  name: example.com:workload-identity:current   # MUST start with the signer name, "/" → ":"
  labels: {example.com/bundle: current}
spec:
  signerName: example.com/workload-identity
  trustBundle: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

Verified on 1.37: a bundle with `signerName: example.com/workload-identity` named `workload-identity-current` is rejected with `ClusterTrustBundle for signerName example.com/workload-identity must be named with prefix example.com:workload-identity:`. Pods consume bundles via the `clusterTrustBundle` projected source, selected either by `name` or by `signerName` + `labelSelector` (an unset selector matches nothing, `{}` matches all); the kubelet deduplicates certificates and may reorder them. Mark the source `optional: true` only if the app can start without trust anchors.

---

#### CRD & API Design

##### Go types with markers (controller-gen v0.22, Kubebuilder v4.16 style)

The following compiles against `k8s.io/apimachinery` v0.37.1 and generates a valid CRD with controller-gen v0.22.0 that kube-apiserver 1.37.0 accepts.

```go
// RedisClusterSpec defines the desired state of RedisCluster.
// +kubebuilder:validation:XValidation:rule="self.minReplicas <= self.replicas",messageExpression="'minReplicas (%d) must be <= replicas (%d)'.format([self.minReplicas, self.replicas])",fieldPath=".minReplicas",reason="FieldValueInvalid"
type RedisClusterSpec struct {
	// replicas is the desired number of Redis nodes.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +required
	Replicas int32 `json:"replicas"`

	// minReplicas is the lowest replica count the operator may scale to.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// tier selects the instance size.
	// +kubebuilder:default=small
	// +optional
	Tier Tier `json:"tier,omitempty"`

	// storageClass is the StorageClass for data volumes. Immutable.
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="storageClass is immutable"
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="storageClass must be a DNS-1123 subdomain"
	// +required
	StorageClass string `json:"storageClass"`

	// version is the Redis version (semantic version).
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:XValidation:rule="isSemver(self, true)",message="version must be a semantic version"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf || semver(self, true).compareTo(semver(oldSelf, true)) >= 0",message="version must not be downgraded"
	// +optional
	Version string `json:"version,omitempty"`

	// ports exposed by each node.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=8
	// +optional
	Ports []RedisPort `json:"ports,omitempty"`

	// tls configures TLS for client connections.
	// +optional
	TLS *RedisTLS `json:"tls,omitempty"`
}

// RedisTLS references the Secret that holds the serving certificate.
type RedisTLS struct {
	// +kubebuilder:validation:MaxLength=253
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// Tier is the size class of a cluster.
// +kubebuilder:validation:Enum=small;medium;large
type Tier string

// RedisPort is a named port.
type RedisPort struct {
	// +kubebuilder:validation:MaxLength=15
	// +required
	Name string `json:"name"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +required
	Port int32 `json:"port"`
}

// RedisClusterStatus defines the observed state of RedisCluster.
type RedisClusterStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=rdc,categories=cache
// +kubebuilder:selectablefield:JSONPath=`.spec.tier`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// RedisCluster is the Schema for the redisclusters API.
type RedisCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   RedisClusterSpec   `json:"spec"`
	Status RedisClusterStatus `json:"status,omitzero"`
}
```

Notes on 1.37 specifics:
- `k8s.io/api` 1.37 switched the tag of embedded `TypeMeta` from `json:",inline"` to `json:""`. `inline` was never an `encoding/json` option, so both produce identical JSON; Kubebuilder v4.16 still scaffolds `",inline"`. Do not flag either form.[^2]
- Kubebuilder v4.16 scaffolds `omitzero` (Go 1.24+) for `metadata` and `status`; `omitempty` on a struct field never omits it.
- Use `// +required` / `// +optional` explicitly; controller-gen derives `required:` from them.

##### Spec design rules

- Prefer **lists of named sub-objects** with `+listType=map` / `+listMapKey=name` over maps — they merge correctly under server-side apply and strategic merge patch.
  ```yaml
  # ✅ Preferred
  ports:
    - name: redis
      port: 6379
  # ❌ Avoid
  ports:
    redis: 6379
  ```
- **Never use floating point** in `spec`; use integers, `resource.Quantity` or strings with units.
- **Do not use `bool` where an enum will be needed** — use a string type alias from day one.
- **No raw maps** except labels, annotations and opaque user data.
- **Only `metadata`, `spec`, `status`** at the top level.
- **Bound every string, list and map** (`MaxLength`, `MaxItems`, `MaxProperties`). Without bounds, CEL rules on those fields are rejected for exceeding the estimated cost budget (see below), and unbounded objects hurt etcd and watch caches.
- Use `+listType=set` only for scalar lists that must be unique; use `atomic` for lists replaced as a whole.

##### Field selectors on custom resources (GA 1.32)

`selectableFields` (marker `+kubebuilder:selectablefield:JSONPath=`) lets clients filter server-side. Only the declared paths work; verified on 1.37:

```
$ kubectl get rdc --field-selector spec.tier=small        # declared → works
$ kubectl get rdc --field-selector spec.replicas=3
Error from server (BadRequest): ... field label not supported: spec.replicas
```

Up to 8 selectable fields per version; the fields must be strings, integers or booleans. Controllers can use them through `client.MatchingFields{"spec.tier": "small"}` against the API server (the controller-runtime cache needs its own field index for the same lookup).

##### PrintColumns

Always define at minimum a state column (`Ready` condition status) and `Age` from `metadata.creationTimestamp`. 1.37 `kubectl get crd` shows SCOPE, VERSIONS and CREATED AT for CRDs themselves.[^2]

---

#### CEL Validation Rules

CRD validation rules (`x-kubernetes-validations`) are GA since 1.29; ratcheting (unchanged invalid values pass on update) since 1.33. A complete CRD with the rules discussed here — applied, then exercised with valid and invalid objects on kube-apiserver 1.37.0:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: redisclusters.cache.example.com
spec:
  group: cache.example.com
  names:
    kind: RedisCluster
    listKind: RedisClusterList
    plural: redisclusters
    singular: rediscluster
    shortNames: [rdc]
    categories: [cache]
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      subresources:
        status: {}
      selectableFields:
        - jsonPath: .spec.tier
      additionalPrinterColumns:
        - name: Ready
          type: string
          jsonPath: .status.conditions[?(@.type=="Ready")].status
        - name: Age
          type: date
          jsonPath: .metadata.creationTimestamp
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required: [replicas, storageClass]
              x-kubernetes-validations:
                - rule: "self.minReplicas <= self.replicas"
                  messageExpression: "'minReplicas (%d) must be <= replicas (%d)'.format([self.minReplicas, self.replicas])"
                  fieldPath: .minReplicas
                  reason: FieldValueInvalid
                - rule: "!has(self.tls) || self.tls.secretName != ''"
                  message: tls.secretName must be set when tls is configured
              properties:
                replicas: {type: integer, format: int32, minimum: 1, maximum: 100}
                minReplicas: {type: integer, format: int32, minimum: 0, default: 1}
                tier: {type: string, enum: [small, medium, large], default: small}
                storageClass:
                  type: string
                  maxLength: 253
                  x-kubernetes-validations:
                    - rule: "self == oldSelf"
                      message: storageClass is immutable
                    - rule: "!format.dns1123Subdomain().validate(self).hasValue()"
                      message: storageClass must be a DNS-1123 subdomain
                memory:
                  x-kubernetes-int-or-string: true
                  x-kubernetes-validations:
                    - rule: "isQuantity(self) && quantity(self).isGreaterThan(quantity('64Mi'))"
                      message: memory must be a quantity above 64Mi
                version:
                  type: string
                  maxLength: 32
                  x-kubernetes-validations:
                    - rule: "isSemver(self, true)"
                      message: version must be a semantic version
                    - rule: "self == oldSelf || semver(self, true).compareTo(semver(oldSelf, true)) >= 0"
                      message: version must not be downgraded
                allowedCIDRs:
                  type: array
                  maxItems: 16
                  x-kubernetes-list-type: set
                  items:
                    type: string
                    maxLength: 64
                    x-kubernetes-validations:
                      - rule: "isCIDR(self) && cidr(self).prefixLength() >= 8"
                        message: each entry must be a CIDR with prefix length >= 8
                ports:
                  type: array
                  maxItems: 8
                  x-kubernetes-list-type: map
                  x-kubernetes-list-map-keys: [name]
                  items:
                    type: object
                    required: [name, port]
                    properties:
                      name: {type: string, maxLength: 15}
                      port: {type: integer, minimum: 1, maximum: 65535}
                labels:
                  type: object
                  maxProperties: 16
                  additionalProperties: {type: string, maxLength: 63}
                  x-kubernetes-validations:
                    - rule: "self.all(k, v, k.size() <= 63 && !k.startsWith('kubernetes.io/'))"
                      message: label keys must be <= 63 chars and must not use the kubernetes.io/ prefix
                tls:
                  type: object
                  properties:
                    secretName: {type: string, maxLength: 253}
            status:
              type: object
              properties:
                observedGeneration: {type: integer, format: int64}
                conditions:
                  type: array
                  x-kubernetes-list-type: map
                  x-kubernetes-list-map-keys: [type]
                  items:
                    type: object
                    required: [type, status, lastTransitionTime, reason, message]
                    properties:
                      type: {type: string, maxLength: 316}
                      status: {type: string, enum: ["True", "False", "Unknown"]}
                      observedGeneration: {type: integer, format: int64, minimum: 0}
                      lastTransitionTime: {type: string, format: date-time}
                      reason: {type: string, minLength: 1, maxLength: 1024}
                      message: {type: string, maxLength: 32768}
```

Creating an invalid object returns every violated rule at once:

```
The RedisCluster "bad" is invalid:
* spec.minReplicas: Invalid value: minReplicas (5) must be <= replicas (3)
* spec: Invalid value: tls.secretName must be set when tls is configured
* spec.labels: Invalid value: label keys must be <= 63 chars and must not use the kubernetes.io/ prefix
* spec.memory: Invalid value: "32Mi": memory must be a quantity above 64Mi
* spec.storageClass: Invalid value: "Fast_SSD": storageClass must be a DNS-1123 subdomain
* spec.allowedCIDRs[0]: Invalid value: "10.0.0.0/4": each entry must be a CIDR with prefix length >= 8
* spec.allowedCIDRs[1]: Invalid value: "nonsense": each entry must be a CIDR with prefix length >= 8
```

and an update that changes `storageClass` and lowers `version` from `7.2.4` to `7.0.0` returns `storageClass is immutable` and `version must not be downgraded`.

##### CEL library availability (all present on 1.37)

| Library / feature | Since | Examples |
|---|---|---|
| Standard macros, `has`, `all`, `exists`, `exists_one`, `map`, `filter` | all | `self.all(x, x > 0)` |
| Kubernetes list, regex, URL, authorizer libraries | all | `self.isSorted()`, `self.matches('^a')`, `url(self).getHost()` |
| Extended strings (v2), CEL optional types, cross-type numeric comparisons, quantity library | 1.29–1.30 | `self.lowerAscii()`, `self.?foo.orValue('')`, `quantity(self).isLessThan(quantity('1Gi'))` |
| IP and CIDR libraries | 1.31 | `isIP(self)`, `ip(self).family() == 4`, `cidr(self).containsIP(ip(x))` |
| Format library | 1.32 | `format.dns1123Label().validate(self)` |
| Two-variable comprehensions | 1.33 | `self.all(k, v, …)` over maps, `transformMap` |
| Semver library | 1.34 | `isSemver(self, true)`, `semver(self).major()` |

Source: Kubernetes CEL reference.[^9] Rules that use a newer library are rejected by older API servers at CRD write time; CRDs shipped to 1.33 clusters must not use `semver()`.

##### CEL rules: what to check in review

- **Cost budget.** The API server estimates the worst-case cost of every rule and `messageExpression` when the CRD is written and rejects it (`estimated rule cost exceeds budget by factor of more than 100x`) — the fix is almost always a missing `maxLength`/`maxItems`/`maxProperties`. Verified on 1.37:
  - `messageExpression: "'a (' + string(self.a) + ') must be <= b'"` on two `int32` fields is **rejected** (the estimator treats the result of `string(int)` as unbounded); `"'a=%d b=%d'.format([self.a, self.b])"` is accepted.
  - The same rejection happens on 1.36 — it is not a 1.37 regression.
  - Returning a list from `messageExpression`, e.g. `format.dns1123Subdomain().validate(self).value().join(', ')`, is also rejected; use a static `message`.
- **`oldSelf` rules are transition rules**: they run only on update, and only when the field existed before. `self == oldSelf` is the canonical immutability rule; put it on the field, not on the parent, so ratcheting and error paths stay precise.
- **`optionalOldSelf: true`** turns `oldSelf` into an optional; the rule must then use `oldSelf.hasValue()` / `oldSelf.value()` / `oldSelf.orValue(x)`. `has(oldSelf)` does not compile (`invalid argument to has() macro`), and setting `optionalOldSelf` on a rule that does not use `oldSelf` is rejected.
- **`isSemver(s, true)` normalises**: with `normalize=true`, `7.2` and `v7.2` are accepted as `7.2.0`. Use `isSemver(s)` for strict SemVer.
- **`fieldPath`** must point at a field below the rule's location; **`reason`** is one of `FieldValueInvalid` (default), `FieldValueForbidden`, `FieldValueRequired`, `FieldValueDuplicate`.
- **List semantics changed in 1.37**: for `x-kubernetes-list-type: set` and `map` lists, `==` no longer treats lists with duplicates as equal, and `+` applies set/map merge semantics. Rules that relied on the old behaviour (e.g. `self + ['x'] == oldSelf + ['x']`) can now evaluate differently.[^2]
- **Optional fields**: guard with `has(self.f)` or use `self.?f.orValue(default)`; accessing an absent field is a runtime error that fails validation.
- Prefer CEL in the CRD over a validating webhook for anything that only needs the object itself.

---

#### Spec vs. Status, Status Conditions

| | `spec` | `status` |
|---|---|---|
| **Written by** | Users / GitOps / CI | Controllers only |
| **Represents** | Desired state | Observed state |
| **Subresource** | — | **always** enable `// +kubebuilder:subresource:status` |
| **Effect of main-resource writes** | updated | ignored (writes go through `/status`) |

- Always set `status.observedGeneration = metadata.generation` after a successful reconcile, and `observedGeneration` on each condition.
- Use `metav1.Condition` with `+listType=map` / `+listMapKey=type`.

| Field | Rule |
|---|---|
| `Type` | PascalCase adjective or past participle: `Ready`, `Available`, `Degraded`, `Progressing` |
| `Status` | `True`, `False` or `Unknown` — never empty |
| `Reason` | CamelCase, machine-readable, non-empty (the schema requires `minLength: 1`) |
| `Message` | human-readable |
| `ObservedGeneration` | `metadata.generation` at the time of observation |
| `LastTransitionTime` | only changes when `Status` changes — `meta.SetStatusCondition` handles this |

Standard types: `Ready` (True = fully operational), `Available`, `Progressing`/`Reconciling` (True while converging), `Degraded` (normally False), `Stalled` (normally False; True = needs human attention). Built-in objects follow the same pattern: Pods report `PodReadyToStartContainers` (GA 1.37, now with a message when False), `PodResizePending`/`PodResizeInProgress`; PVCs report `Unused` (1.37); HPAs `ScaledToZero` (1.37).

> ⚠️ Do **not** add new `phase` string fields; conditions are the API convention. Existing `phase` fields may stay for compatibility.[^26]

---

#### Versioning, Conversion & Storage Version Migration

##### Version lifecycle

```
v1alpha1 → v1alpha2 → v1beta1 → v1beta2 → v1
```

| Stage | Stability | Built-in API enabled by default | Backward compatibility |
|---|---|---|---|
| `v1alphaX` | Experimental | No | Not guaranteed |
| `v1betaX` | Pre-release | **No** for new beta APIs since 1.24 | Best-effort |
| `v1` | Stable | Yes | Required |

For CRDs, "enabled" is the `served` flag the CRD author sets per version.

##### Rules

1. Never remove or rename fields in a served version.
2. Never tighten validation on an existing field so that stored objects become invalid. CEL ratcheting (GA 1.33) lets unchanged invalid values pass on update, but `create` and changed fields are validated strictly.
3. Add new fields as optional.
4. Exactly one version has `storage: true`; conversion must be lossless both ways (use annotations to round-trip fields that do not exist in the other version).
5. Use a conversion webhook (`spec.conversion.strategy: Webhook`) when schemas differ. Since 1.37, kube-apiserver may call it with up to 10 concurrent requests while initialising its watch cache — size the webhook for concurrency.[^1]
6. Deprecate old versions with `deprecated: true` and `deprecationWarning`, keep them served for at least a year, then set `served: false`, migrate storage, and only then drop the version from `status.storedVersions` (the in-tree migrator does that for CRDs, see below).

```yaml
versions:
  - name: v1beta1
    served: true
    storage: false
    deprecated: true
    deprecationWarning: "cache.example.com/v1beta1 RedisCluster is deprecated; use cache.example.com/v1"
  - name: v1
    served: true
    storage: true
```

##### StorageVersionMigration (GA 1.37) [NEW in 1.37]

After changing the storage version, objects already in etcd stay in the old version until rewritten. 1.37 ships the migrator in-tree: create a `storagemigration.k8s.io/v1` object and the controller rewrites every object of that resource to the current storage version (also useful after rotating encryption-at-rest keys).[^25]

```yaml
apiVersion: storagemigration.k8s.io/v1
kind: StorageVersionMigration
metadata:
  name: redisclusters-to-v1
spec:
  resource:
    group: cache.example.com
    resource: redisclusters
```

- `spec.resource` is immutable and names a group + resource, not a version: the target is always the current storage version.
- 1.37 reports progress (number of processed objects) in the migration's conditions; wait for the `Succeeded` condition. For a CRD the migrator then removes the migrated old versions from `status.storedVersions` itself (behaviour added in 1.36); only edit `storedVersions` by hand when migrating with something else.[^2]
- This replaces the out-of-tree `kube-storage-version-migrator` and hand-written "get all, put all" scripts. On ≤ 1.36 clusters the API is not served by default (`storagemigration.k8s.io/v1beta1` was off-by-default beta).
- RBAC: the migrator controller runs in kube-controller-manager; users creating migrations need `create` on `storageversionmigrations`.

---

#### Admission: Policies and Webhooks

##### Choose the mechanism (prefer left-to-right)

```
CRD schema / CEL rules  →  ValidatingAdmissionPolicy / MutatingAdmissionPolicy  →  Admission webhook
(own CRDs only)            (in-process CEL, any resource)                           (extra service, network hop)
```

Use webhooks only for what CEL cannot express: calls to external systems, cross-object lookups beyond `params`/`authorizer`, image signature verification.

##### ValidatingAdmissionPolicy (GA 1.30)

Verified on 1.37 — denies Deployments/StatefulSets/DaemonSets with images not pinned by digest in namespaces labelled `policy.example.com/enforce-digests=true`:

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: require-image-digest
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["deployments", "statefulsets", "daemonsets"]
  matchConditions:
    - name: skip-system-namespaces
      expression: "!(request.namespace in ['kube-system', 'kube-public'])"
  variables:
    - name: containers
      expression: "object.spec.template.spec.containers + object.spec.template.spec.?initContainers.orValue([])"
  validations:
    - expression: "variables.containers.all(c, c.image.contains('@sha256:'))"
      messageExpression: "'all images must be pinned by digest; offending: ' + variables.containers.filter(c, !c.image.contains('@sha256:')).map(c, c.name).join(', ')"
      reason: Invalid
  auditAnnotations:
    - key: unpinned-images
      valueExpression: "variables.containers.filter(c, !c.image.contains('@sha256:')).map(c, c.name).join(',')"
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: require-image-digest
spec:
  policyName: require-image-digest
  validationActions: [Deny, Audit]
  matchResources:
    namespaceSelector:
      matchLabels:
        policy.example.com/enforce-digests: "true"
```

Result: `deployments.apps "bad" is forbidden: ValidatingAdmissionPolicy 'require-image-digest' with binding 'require-image-digest' denied request: all images must be pinned by digest; offending: nginx`.

Review points:
- A policy does nothing without a **binding**; `validationActions` is required (`Deny`, `Warn`, `Audit`; `Deny` and `Warn` cannot be combined).
- Roll out new policies with `[Warn, Audit]` first, then switch to `Deny`.
- `auditAnnotations[].valueExpression` must return a string; a conditional such as `cond ? 'true' : null` does not type-check (`found no matching overload for '_?_:_' applied to '(bool, string, null)'`).
- Use `object.spec.?field.orValue(...)` for optional fields; `oldObject` is `null` on CREATE.
- `paramKind` + `paramRef` (with required `parameterNotFoundAction`) make policies configurable per namespace.
- There is no `vap` short name: use `kubectl get validatingadmissionpolicies`.

##### MutatingAdmissionPolicy (GA 1.36)

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingAdmissionPolicy
metadata:
  name: default-team-label
spec:
  failurePolicy: Fail
  reinvocationPolicy: IfNeeded           # REQUIRED in v1 (Never | IfNeeded)
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["configmaps"]
  matchConditions:
    - name: no-team-label
      expression: "!has(object.metadata.labels) || !('example.com/team' in object.metadata.labels)"
  mutations:
    - patchType: ApplyConfiguration
      applyConfiguration:
        expression: >
          Object{
            metadata: Object.metadata{
              labels: {"example.com/team": "unassigned"}
            }
          }
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingAdmissionPolicyBinding
metadata:
  name: default-team-label
spec:
  policyName: default-team-label
  matchResources:
    namespaceSelector:
      matchLabels:
        policy.example.com/enforce-digests: "true"
```

Verified on 1.37: a ConfigMap created in a matching namespace gets `{"example.com/team":"unassigned"}`. Omitting `reinvocationPolicy` fails with `spec.reinvocationPolicy: Required value`. `patchType: JSONPatch` with a `jsonPatch.expression` returning `[JSONPatch{op: "add", path: "...", value: ...}]` is the alternative for list appends. Prefer `ApplyConfiguration` (merge semantics) and always add a `matchCondition` that skips objects that are already mutated, so the policy is idempotent. Since 1.37 MAP objects are stored in etcd as `admissionregistration.k8s.io/v1`.[^2]

##### Manifest-based admission configuration (beta-on in 1.37) [NEW in 1.37]

Cluster admins can load webhooks, VAPs and MAPs from files referenced by `staticManifestsDir` in the `AdmissionConfiguration`. They apply from API-server start, keep working when etcd is unavailable, cannot be deleted through the API, and can protect the API-based admission objects themselves. Verified on 1.37 with `--admission-control-config-file`:[^24]

```yaml
apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
  - name: ValidatingAdmissionPolicy
    configuration:
      apiVersion: apiserver.config.k8s.io/v1
      kind: ValidatingAdmissionPolicyConfiguration
      staticManifestsDir: /etc/kubernetes/admission/policies   # absolute; direct *.yaml|*.yml|*.json children
```

```yaml
# /etc/kubernetes/admission/policies/deny-privileged.yaml — policy and binding in ONE file
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: deny-privileged.static.k8s.io          # names MUST end in .static.k8s.io
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["pods"]
  validations:
    - expression: "object.spec.containers.all(c, !has(c.securityContext) || !has(c.securityContext.privileged) || !c.securityContext.privileged)"
      message: privileged containers are not allowed
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: deny-privileged-binding.static.k8s.io
spec:
  policyName: deny-privileged.static.k8s.io
  validationActions: [Deny]
```

- Static objects do not appear in `kubectl get validatingadmissionpolicies`.
- On 1.36 (gate alpha/off) API objects with the `.static.k8s.io` suffix are still accepted, only with a `Warning: metadata.name: names ending with ".static.k8s.io" are reserved …` (verified on 1.36.2) — rename any such objects before upgrading.
- Creating an API object whose name ends in `.static.k8s.io` is rejected: `names ending with ".static.k8s.io" are reserved for static manifest-based configurations and cannot be managed via the API`.
- Files are watched; an invalid update keeps the previous configuration. Put related objects in one file so they reload atomically.
- `MutatingAdmissionWebhook` `staticOrder` is a 1.38 addition; in 1.37 static mutating webhooks always run first.

##### Admission webhooks

```go
// +kubebuilder:webhook:path=/validate-cache-example-com-v1-rediscluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=cache.example.com,resources=redisclusters,verbs=create;update,versions=v1,name=vrediscluster-v1.cache.example.com,admissionReviewVersions=v1,matchPolicy=Equivalent,timeoutSeconds=5
```

- `failurePolicy: Fail` for security-relevant checks; `Ignore` only for best-effort ones. Combine `Fail` with a `namespaceSelector`/`objectSelector` that excludes the webhook's own namespace and `kube-system`, or a webhook outage can block its own recovery.
- `sideEffects: None` (or `NoneOnDryRun`) is required for `kubectl --dry-run=server` to call the webhook.
- `matchPolicy: Equivalent` (default in v1) so the webhook also fires for other versions of the resource.
- `matchConditions` (GA 1.30) filter requests with CEL before the HTTP call — cheaper than filtering inside the webhook.
- `timeoutSeconds` ≤ 30 (default 10); keep handlers fast and do not call the API server for every request without a cache.
- Handle `CREATE` and `UPDATE` for immutability; `DELETE` only when needed.
- TLS certificates must rotate (cert-manager or similar); controller-runtime's `certwatcher` reloads them without a restart.
- **1.37 changes**: webhooks no longer receive `tokenreviews`, `subjectaccessreviews` and other auth/authz virtual resources (`ExcludeAdmissionWebhookVirtualResources`, beta-on), matching VAP/MAP; with `--enable-aggregator-routing=true` requests are load-balanced across webhook endpoints (`WebhookRoundTripLoadBalancing`). Service-account tokens bound to webhook configurations are alpha (`APIServerWebhookAuthenticationToken`).[^2]

---

#### RBAC & Least Privilege

1. Grant only the verbs/resources the code uses; generate them from `+kubebuilder:rbac` markers and review the diff of `config/rbac/role.yaml`.
2. Prefer `Role` + `RoleBinding` (namespaced) over cluster-wide roles; use a ClusterRole only for cluster-scoped resources or all-namespace watches.
3. No wildcard verbs or resources, no `cluster-admin`, no `escalate`/`bind`/`impersonate` unless the component's purpose requires it.
4. `secrets` `list`/`watch` exposes every secret in scope — prefer `get` on named secrets, or scope the manager's cache to labelled secrets.
5. Status and finalizers are separate subresources: `redisclusters/status` (`get;update;patch`) and `redisclusters/finalizers` (`update`) — the latter is required to set `blockOwnerDeletion: true` on children when the `OwnerReferencesPermissionEnforcement` admission plugin is enabled (OpenShift and some distributions).
6. Events: controller-runtime v0.25's `GetEventRecorder` writes **`events.k8s.io/v1`** Events — grant `events.k8s.io` `events` `create;patch`. The deprecated `GetEventRecorderFor` writes core `v1` Events (`""` group).
7. Restrict `nodes/logs` and `nodes/proxy`: since 1.37 the kubelet logs its effective configuration at startup, readable via `nodes/logs`.[^2]
8. `kubectl auth can-i --as=system:serviceaccount:<ns>:<sa> …` and `kubectl auth whoami` verify bindings.

```go
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
```

controller-gen v0.22.0 turns these into one `ClusterRole` with exactly those rules (verified). Add `+kubebuilder:rbac:...,namespace=<ns>` to generate a namespaced Role instead.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: redis-operator
  namespace: redis-system
automountServiceAccountToken: false     # opt in per Pod
---
# in the operator Pod spec
spec:
  serviceAccountName: redis-operator
  automountServiceAccountToken: true
```

Use projected, audience-bound tokens (`serviceAccountToken` projection with `audience` and `expirationSeconds`) for talking to anything other than the API server; never create long-lived `kubernetes.io/service-account-token` Secrets (unused ones are auto-cleaned since 1.30).

---

#### Security Hardening

##### Pod Security Standards

Label namespaces with the Pod Security Admission levels; operator and app namespaces should enforce `restricted`:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: redis-system
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: v1.37
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/audit: restricted
```

Pin `enforce-version` to a concrete minor (or accept that `latest` changes rules on upgrade).[^27]

##### Container security context (restricted profile)

```yaml
securityContext:                       # Pod level
  runAsNonRoot: true
  runAsUser: 10001
  runAsGroup: 10001
  fsGroup: 10001
  seccompProfile: {type: RuntimeDefault}
containers:
  - name: manager
    image: registry.example.com/operator@sha256:0000000000000000000000000000000000000000000000000000000000000000
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities: {drop: [ALL]}
```

- `appArmorProfile: {type: RuntimeDefault}` (GA 1.31) replaces the deprecated AppArmor annotations.
- `procMount: Unmasked` (GA 1.36) is only accepted together with `hostUsers: false` (otherwise the Pod is rejected: "`hostUsers` must be false to use `Unmasked`"); it is meant for nested containers and image builders, and the `restricted` standard still forbids it.
- `supplementalGroupsPolicy: Strict` (GA 1.35) stops the image's `/etc/group` from adding unexpected supplementary groups.
- `hostUsers: false` (GA 1.36) is the strongest mitigation for workloads that need root inside the container.
- 1.37 relaxes **updates** that set `allowPrivilegeEscalation` together with `CAP_SYS_ADMIN`; creation still rejects that combination.[^2]

##### SELinux volume labelling changes in 1.37 [NEW in 1.37]

On SELinux-enforcing nodes, 1.37 enables `SELinuxMount` by default. A volume is mounted with `-o context=<label>` instead of being relabelled recursively when: its CSI driver sets `CSIDriver.spec.seLinuxMount: true` (or it is an in-tree `fc`/`iscsi` volume), the Pod (or all its containers) sets at least `seLinuxOptions.level`, and `seLinuxChangePolicy` is unset or `MountOption`.[^10] Consequence: Pods with **different** SELinux labels (for example a privileged and an unprivileged Pod) that share such a volume on the same node can no longer coexist — one stays in `ContainerCreating`.

```yaml
# GOOD: a Pod that must share a volume with differently-labelled Pods opts out
spec:
  securityContext:
    seLinuxChangePolicy: Recursive     # keep recursive relabelling (old behaviour)
    seLinuxOptions: {level: "s0:c123,c456"}
```

- Review shared RWX volumes (NFS/CephFS CSI drivers with `seLinuxMount: true`) used by several workloads with different security contexts.
- The `selinux-warning-controller` in kube-controller-manager (enable with `--controllers=*,selinux-warning-controller`) emits Events and the `selinux_warning_controller_selinux_volume_conflict` metric for conflicting Pods.[^10]
- The behaviour is not locked until 1.38, so admins can still disable `SELinuxMount` cluster-wide on 1.37. Nodes without SELinux are unaffected.

##### Secrets

- Never put secret values in CR `spec`; reference Secrets by name (same namespace) and grant only `get` on them.
- Do not log secret data, tokens or full objects that contain them; redact `Authorization` headers in HTTP client logging.
- Env files (`fileKeyRef`) and `emptyDir` are not secret stores.
- Since 1.37 static Pods cannot reference Secrets or ConfigMaps at all.[^1]
- Prefer short-lived credentials: Pod certificates (GA 1.37), projected SA tokens, external secret managers.
- 1.37 fixes a 1.34+ regression with env values from Secrets that contain non-UTF-8 bytes.[^2]

##### Network policies for operators

```yaml
# Restrict operator egress to ports 443/6443 (any destination). To pin egress to the
# API server, add a `to:` ipBlock with the API server endpoint IPs.
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: redis-operator-egress
  namespace: redis-system
spec:
  podSelector:
    matchLabels:
      control-plane: controller-manager
  policyTypes: [Egress]
  egress:
    - ports:
        - {port: 443, protocol: TCP}
        - {port: 6443, protocol: TCP}
    - ports:                              # cluster DNS
        - {port: 53, protocol: UDP}
        - {port: 53, protocol: TCP}
```

##### Images

- Distroless or scratch base images for Go operators; pin by digest; never `:latest`; scan in CI.
- Image volumes and sidecars are images too — pin and scan them.
- Kubelet image-pull credential verification (beta-on since 1.35) means a Pod without pull credentials can no longer reuse a private image another Pod pulled on the same node (depending on `imagePullCredentialsVerificationPolicy`); give every Pod the `imagePullSecrets` it needs.

---

#### Controllers with controller-runtime v0.25

controller-runtime v0.25 is the release built on `k8s.io/*` v0.37 (Kubernetes 1.37). Its main changes for reviewers:[^11]

| Area | v0.25 state | Review rule |
|---|---|---|
| Events | `mgr.GetEventRecorder(name)` returns a recorder for **`events.k8s.io/v1`** (`Eventf(regarding, related, type, reason, action, note, args...)`, plus `AnnotatedEventf`); `GetEventRecorderFor` (core v1 Events) is **deprecated** | new code uses `GetEventRecorder`; RBAC on `events.k8s.io` |
| Server-side apply | `client.Client.Apply(ctx, applyConfiguration, opts...)` and `Status().Apply(...)`; the `client.Apply` patch type is **deprecated** | use typed apply configurations (`k8s.io/client-go/applyconfigurations/...`) with a stable `client.FieldOwner` |
| Webhooks | generic `admission.Validator[T]` / `admission.Defaulter[T]`, builder `ctrl.NewWebhookManagedBy(mgr, &T{}).WithValidator(...)`/`WithDefaulter(...)`; `CustomValidator`, `CustomDefaulter`, `WithCustomValidator`, `WithCustomDefaulter` are **deprecated** | no `runtime.Object` type assertions in new webhooks |
| Requeue | `Result.Requeue` **deprecated**; `RequeueAfter` or returning an error | flag new uses of `Requeue: true` |
| Queue | the controller-runtime **priority queue is on by default** (`UsePriorityQueue`); `Result.Priority` sets requeue priority | do not rely on FIFO ordering between items |
| Read-your-writes | experimental `client.CacheOptions.EnableReadYourWritesConsistency` blocks cache reads until earlier writes are visible; `DisableReadYourWritesConsistency` per request | opt-in; not correct when a resource is cached for more than one namespace |
| Timeouts | `controller.Options.ReconciliationTimeout` puts a deadline on each `Reconcile` context | set it when reconciles call external systems |
| Warm-up | `EnableWarmup` starts sources before leader election is won | reduces fail-over latency for large caches |
| Metrics / webhook server | client-go REST client metrics are opt-in; webhook port `-1` disables the webhook server | — |
| Fake client | `WithStatusSubresource` still required for status writes; supports `Apply` | unit tests that write status must register it |

##### Reconciler (compiles against controller-runtime v0.25.2 / client-go v0.37.1)

```go
package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1ac "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	cachev1 "example.com/redis-operator/api/v1"
)

const (
	fieldOwner     = client.FieldOwner("redis-operator")
	cleanupFinal   = "cache.example.com/external-cleanup"
	conditionReady = "Ready"
)

// errPermanent marks errors that retrying cannot fix (bad user input).
var errPermanent = errors.New("permanent")

// RedisClusterReconciler reconciles RedisCluster objects.
type RedisClusterReconciler struct {
	client.Client
	Recorder events.EventRecorder
	Backups  BackupAPI
}

// BackupAPI is the external system the operator cleans up on deletion.
type BackupAPI interface {
	DeleteBackups(ctx context.Context, namespace, name string) error
}

func (r *RedisClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	rc := &cachev1.RedisCluster{}
	if err := r.Get(ctx, req.NamespacedName, rc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !rc.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, rc)
	}
	if controllerutil.AddFinalizer(rc, cleanupFinal) {
		// Update returns a conflict if the cached copy is stale; the
		// controller retries with backoff.
		return ctrl.Result{}, r.Update(ctx, rc)
	}

	if err := r.applyStatefulSet(ctx, rc); err != nil {
		if errors.Is(err, errPermanent) {
			r.Recorder.Eventf(rc, nil, corev1.EventTypeWarning, "InvalidSpec", "Reconcile", "cannot reconcile: %v", err)
			return ctrl.Result{}, r.setReady(ctx, rc, metav1.ConditionFalse, "InvalidSpec", err.Error())
		}
		return ctrl.Result{}, fmt.Errorf("applying StatefulSet: %w", err)
	}

	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, req.NamespacedName, sts); err != nil {
		return ctrl.Result{}, err
	}
	rc.Status.ReadyReplicas = sts.Status.ReadyReplicas
	if sts.Status.ReadyReplicas < rc.Spec.Replicas {
		log.V(1).Info("waiting for replicas", "ready", sts.Status.ReadyReplicas, "desired", rc.Spec.Replicas)
		return ctrl.Result{}, r.setReady(ctx, rc, metav1.ConditionFalse, "Progressing",
			fmt.Sprintf("%d/%d replicas ready", sts.Status.ReadyReplicas, rc.Spec.Replicas))
	}
	return ctrl.Result{}, r.setReady(ctx, rc, metav1.ConditionTrue, "Reconciled", "all replicas ready")
}

func (r *RedisClusterReconciler) applyStatefulSet(ctx context.Context, rc *cachev1.RedisCluster) error {
	if rc.Spec.Replicas < 1 {
		return fmt.Errorf("%w: replicas must be >= 1", errPermanent)
	}
	labels := map[string]string{"app.kubernetes.io/name": "redis", "app.kubernetes.io/instance": rc.Name}
	sts := appsv1ac.StatefulSet(rc.Name, rc.Namespace).
		WithLabels(labels).
		WithOwnerReferences(metav1ac.OwnerReference().
			WithAPIVersion(cachev1.GroupVersion.String()).
			WithKind("RedisCluster").
			WithName(rc.Name).
			WithUID(rc.UID).
			WithController(true).
			WithBlockOwnerDeletion(true)).
		WithSpec(appsv1ac.StatefulSetSpec().
			WithReplicas(rc.Spec.Replicas).
			WithServiceName(rc.Name).
			WithSelector(metav1ac.LabelSelector().WithMatchLabels(labels)).
			WithTemplate(corev1ac.PodTemplateSpec().
				WithLabels(labels).
				WithSpec(corev1ac.PodSpec().
					WithContainers(corev1ac.Container().
						WithName("redis").
						WithImage("registry.example.com/redis:" + rc.Spec.Version)))))
	// Server-side apply: only the fields set above are owned by this
	// manager; fields set by others (HPA, mutating policies) are kept.
	return r.Apply(ctx, sts, fieldOwner, client.ForceOwnership)
}

func (r *RedisClusterReconciler) setReady(ctx context.Context, rc *cachev1.RedisCluster, s metav1.ConditionStatus, reason, msg string) error {
	base := rc.DeepCopy()
	changed := meta.SetStatusCondition(&rc.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             s,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: rc.Generation,
	})
	if !changed && rc.Status.ObservedGeneration == rc.Generation && rc.Status.ReadyReplicas == base.Status.ReadyReplicas {
		return nil
	}
	rc.Status.ObservedGeneration = rc.Generation
	// Optimistic lock: fail with a conflict instead of overwriting a newer status.
	return r.Status().Patch(ctx, rc, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{}))
}

func (r *RedisClusterReconciler) finalize(ctx context.Context, rc *cachev1.RedisCluster) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(rc, cleanupFinal) {
		return ctrl.Result{}, nil
	}
	if err := r.Backups.DeleteBackups(ctx, rc.Namespace, rc.Name); err != nil {
		// Honour Retry-After from 429 responses instead of hammering.
		if d, ok := apierrors.SuggestsClientDelay(err); ok && apierrors.IsTooManyRequests(err) {
			return ctrl.Result{RequeueAfter: time.Duration(d) * time.Second}, nil
		}
		return ctrl.Result{}, err
	}
	controllerutil.RemoveFinalizer(rc, cleanupFinal)
	return ctrl.Result{}, r.Update(ctx, rc)
}

// SetupWithManager registers the controller.
func (r *RedisClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1.RedisCluster{}, builder.WithPredicates(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		))).
		Owns(&appsv1.StatefulSet{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 4,
			ReconciliationTimeout:   2 * time.Minute,
		}).
		Complete(r)
}
```

Why it looks like this:
- **Fetch → NotFound → deletion → finalizer → apply children → status.** Each step is idempotent; every return path either succeeds or returns an error for backoff.
- **Server-side apply with a fixed field manager** for children: no read-modify-write races, and fields owned by other actors (HPA `replicas`, mutating policies) are not clobbered as long as the controller does not set them. `ForceOwnership` is right for fields the controller exclusively owns.
- **Owner reference via the apply configuration** (`WithController(true)`, `WithBlockOwnerDeletion(true)`) — the SSA equivalent of `controllerutil.SetControllerReference`.
- **Status through `Status().Patch` with `MergeFromWithOptimisticLock`** and only when something changed: avoids status-write storms and lost updates.
- **`GenerationChangedPredicate`** skips status-only updates of the primary object; when an annotation drives behaviour, OR it with `AnnotationChangedPredicate`. Deletion is still delivered: setting `deletionTimestamp` on an object with finalizers increments `metadata.generation` (verified on 1.37: 1 → 2 for a custom resource and for a Deployment).
- **Events only on state changes**, via the `events.k8s.io/v1` recorder.

##### Manager setup

```go
mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
	Scheme: scheme,
	Metrics: metricsserver.Options{
		BindAddress:    ":8443",
		SecureServing:  true,
		FilterProvider: filters.WithAuthenticationAndAuthorization, // authn/authz on /metrics
	},
	HealthProbeBindAddress:        ":8081",
	LeaderElection:                true,
	LeaderElectionID:              "redis-operator.cache.example.com",
	LeaderElectionReleaseOnCancel: true,
})
if err != nil {
	setupLog.Error(err, "unable to create manager")
	os.Exit(1)
}
if err := (&controller.RedisClusterReconciler{
	Client:   mgr.GetClient(),
	Recorder: mgr.GetEventRecorder("redis-operator"), // events.k8s.io/v1
	Backups:  backups,
}).SetupWithManager(mgr); err != nil {
	setupLog.Error(err, "unable to create controller")
	os.Exit(1)
}
_ = mgr.AddHealthzCheck("healthz", healthz.Ping)
_ = mgr.AddReadyzCheck("readyz", healthz.Ping)
if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
	setupLog.Error(err, "manager exited with error")
	os.Exit(1)
}
```

- `LeaderElectionReleaseOnCancel: true` is safe only if the process exits right after `Start` returns.
- The reporting controller name passed to `GetEventRecorder` must be a valid qualified name and `len(name)+1+len(hostname)` ≤ 128 (it becomes `reportingInstance`).
- Gate readiness on cache sync with a custom checker if needed (`mgr.GetCache().WaitForCacheSync(req.Context())` inside a `healthz.Checker`).

##### Typed validating webhook (v0.25 API)

```go
// RedisClusterValidator checks invariants that need cluster state
// (CEL in the CRD handles everything that only needs the object itself).
type RedisClusterValidator struct {
	Reader client.Reader
}

var _ admission.Validator[*cachev1.RedisCluster] = &RedisClusterValidator{}

func (v *RedisClusterValidator) ValidateCreate(ctx context.Context, rc *cachev1.RedisCluster) (admission.Warnings, error) {
	return nil, v.checkSecret(ctx, rc)
}

func (v *RedisClusterValidator) ValidateUpdate(ctx context.Context, _, rc *cachev1.RedisCluster) (admission.Warnings, error) {
	return nil, v.checkSecret(ctx, rc)
}

func (v *RedisClusterValidator) ValidateDelete(context.Context, *cachev1.RedisCluster) (admission.Warnings, error) {
	return nil, nil
}

func (v *RedisClusterValidator) checkSecret(ctx context.Context, rc *cachev1.RedisCluster) error {
	if rc.Spec.TLS == nil || rc.Spec.TLS.SecretName == "" {
		return nil
	}
	var list cachev1.RedisClusterList
	if err := v.Reader.List(ctx, &list, client.InNamespace(rc.Namespace)); err != nil {
		return fmt.Errorf("listing RedisClusters: %w", err)
	}
	for _, other := range list.Items {
		if other.Name != rc.Name && other.Spec.TLS != nil && other.Spec.TLS.SecretName == rc.Spec.TLS.SecretName {
			return fmt.Errorf("spec.tls.secretName %q is already used by %s", rc.Spec.TLS.SecretName, other.Name)
		}
	}
	return nil
}

// SetupWithManager registers the typed validating webhook.
func SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &cachev1.RedisCluster{}).
		WithValidator(&RedisClusterValidator{Reader: mgr.GetAPIReader()}).
		Complete()
}
```

```go
// BAD (deprecated in v0.25): untyped CustomValidator with type assertions
func (v *validator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	rc, ok := obj.(*cachev1.RedisCluster)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", obj)
	}
	// ...
}
```

Webhooks should not do uncached `List` calls on hot paths in large namespaces; the example uses `GetAPIReader()` for correctness across replicas — use an indexed cache lookup when volume matters.

##### Watches, predicates and mapping

```go
// Watch a shared Secret; enqueue every RedisCluster that references it.
ctrl.NewControllerManagedBy(mgr).
	For(&cachev1.RedisCluster{}).
	Watches(&corev1.Secret{},
		handler.EnqueueRequestsFromMapFunc(r.clustersForSecret),
		builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
	Complete(r)
```

- Map functions must be cheap and use a cache **field index** (`mgr.GetFieldIndexer().IndexField(...)`), not a full `List` + filter.
- Restrict caches for high-cardinality types (`cache.Options.ByObject` with label/field selectors, `DefaultNamespaces`) — an operator that watches all Secrets/Pods/ConfigMaps cluster-wide holds them all in memory.
- Use `builder.OnlyMetadata` for types where only metadata is needed.
- `Owns()` only maps objects whose controller owner reference points to the `For` type.

---

#### client-go v0.37

- **Context everywhere [NEW in 1.37]**: nearly all internal `context.TODO()` calls are gone; new context-aware APIs exist for REST mapping and discovery — `meta.RESTMapperWithContext`, `discovery.DiscoveryInterfaceWithContext`, `restmapper.NewDiscoveryRESTMapperWithContext`, `restmapper.GetAPIGroupResourcesWithContext`, `restmapper.NewShortcutExpanderWithContext` (absent in v0.36). The old APIs are deprecated without a removal plan.[^2] Pass the caller's `ctx` (and logger via `klog.FromContext`) instead of `context.Background()`.
- **Contextual logging** is complete apart from a few auth-plugin log calls.[^1]
- **Typed workqueues**: use `workqueue.NewTypedRateLimitingQueueWithConfig[T]` and `workqueue.DefaultTypedControllerRateLimiter[T]()`; the untyped constructors are deprecated.
- **Informers**: use `cache.ListWatch` with `ListWithContextFunc`/`WatchFuncWithContext`; the context-less `ListFunc`/`WatchFunc` are deprecated. `cache.Store` has `Bookmark` and `LastStoreSyncResourceVersion` (since v0.36), which the stale-controller mitigation builds on. The 1.37 release blog says `ResourceEventHandlerFuncs` gets a `BookmarkFunc` callback, but client-go v0.37.1 does not have one (`ResourceEventHandlerFuncs` still has only `AddFunc`/`UpdateFunc`/`DeleteFunc`; the only `BookmarkFunc` is on the test helper `cache.FakeCustomStore`) — do not suggest it.[^36][^2]
- **Events**: the `events.k8s.io` broadcaster has `StartRecordingToSinkWithContext` and `StartLogging(logger)`; `AnnotatedEventf` (new) attaches annotations to Events. The old `record` package still works but is the legacy API.
- **Leader election**: `resourcelock.MultiLock`, `UnknownLeader`, `ConcatRawRecord` are deprecated in 1.37; use the `LeaseLock`.[^2]
- **Discovery** now requests aggregated discovery `v2` only (no `v2beta1` fallback).[^2]
- **WatchList** (streaming initial list) is beta-on in clients since 1.35 (`WatchListClient`); 1.37 servers can gzip WatchList responses.
- **kubeconfig migration** preserves the source file mode (1.37 fix) — earlier versions could create a world-readable `~/.kube/config` during migration.[^2]

---

#### Finalizers, Owner References & Garbage Collection

##### Finalizers

Use finalizers only for cleanup outside the cluster (cloud resources, DNS, external databases) or cross-namespace/cluster-scoped objects that owner references cannot cover.

- Names are qualified: `cache.example.com/external-cleanup` ✅, `cleanup` ❌.
- Add the finalizer **before** creating the external resource; remove it only after cleanup succeeded.
- Cleanup must be idempotent; return the error to retry; a permanently failing cleanup sets a `Stalled` condition + Warning Event.
- Never add a finalizer to an object that already has `deletionTimestamp`.
- Do not create objects in a namespace that is terminating.
- Ordered namespace deletion (GA 1.34) deletes Pods before other resources in a terminating namespace; it does not order your CRs — finalizers still must not depend on other namespace objects that may already be gone.

##### Owner references

| Rule | Notes |
|---|---|
| Owner and dependent in the **same namespace** | a namespaced dependent pointing at a missing same-namespace owner is garbage-collected |
| Namespaced owner **cannot** own a cluster-scoped object | the reference is unresolvable; the dependent is not GC'd and gets an `OwnerRefInvalidNamespace` event |
| Cluster-scoped owner **can** own cluster-scoped or namespaced objects | permitted |
| At most one `controller: true` owner | other owners are informational |
| Use `controllerutil.SetControllerReference` (or the SSA owner-reference builder) | sets `controller` and `blockOwnerDeletion` |

| Scenario | Use |
|---|---|
| Child in the same namespace | `ownerReferences` |
| Cluster-scoped child of a namespaced owner | finalizer on the owner (or a cluster-scoped owner) |
| External resource | finalizer |
| Cross-namespace cleanup | finalizer |

---

#### Error Handling, Requeue & API Server Back-Pressure

| Return | Meaning |
|---|---|
| `ctrl.Result{}, nil` | done until the next event |
| `ctrl.Result{}, err` | transient error — requeued with per-item exponential backoff (5 ms … ~1000 s) |
| `ctrl.Result{RequeueAfter: d}, nil` | poll again after `d` (external state, expiring certificates) |
| `ctrl.Result{Requeue: true}, nil` | **deprecated** in v0.25 — rate-limited requeue; use `RequeueAfter` |

- Never return both a non-zero `Result` and an error: `RequeueAfter`/`Requeue` are ignored when the error is non-nil, and controller-runtime logs `Warning: Reconciler returned both a result with either RequeueAfter or Requeue set and a non-nil error`.
- Permanent errors (invalid user input): set a condition, emit one Warning Event, return `nil` — do not hot-loop.
- `client.IgnoreNotFound(err)` on the initial `Get`; `apierrors.IsConflict(err)` → just return the error and let the next reconcile re-read.
- **HTTP 429 [NEW in 1.37 emphasis]**: with `WatchCacheInitializationPostStartHook` GA, a restarting or re-initialising kube-apiserver answers expensive LIST/WATCH requests with 429 + `Retry-After` instead of overloading etcd; API Priority and Fairness does the same under load. client-go's REST client retries 429/5xx responses that carry a `Retry-After` header (up to 10 times by default); custom HTTP clients, scripts and reconcilers that call external APIs must honour `Retry-After` (`apierrors.SuggestsClientDelay`). An Alpha gate (`ConsistentListFromCacheSkipTimeoutFallback`) can make consistent LISTs return 429 instead of falling back to etcd.[^1][^2]
- **Stale caches**: 1.37 extends read-your-writes "stale controller" mitigation to the HPA controller; the same pattern applies to your controllers — record the resourceVersion of your own writes and requeue until the informer has caught up, or opt into controller-runtime's experimental `EnableReadYourWritesConsistency`.[^36]

```go
// BAD: hot loop on a permanent error and swallowed transient error
if err := validate(rc); err != nil {
	return ctrl.Result{Requeue: true}, nil
}
if err := r.Update(ctx, sts); err != nil {
	log.Error(err, "update failed") // lost: no retry
}
```

```go
// GOOD
if err := validate(rc); err != nil {
	r.Recorder.Eventf(rc, nil, corev1.EventTypeWarning, "InvalidSpec", "Validate", "%v", err)
	return ctrl.Result{}, r.setReady(ctx, rc, metav1.ConditionFalse, "InvalidSpec", err.Error())
}
if err := r.Update(ctx, sts); err != nil {
	return ctrl.Result{}, fmt.Errorf("updating StatefulSet %s: %w", sts.Name, err)
}
```

---

#### Observability

##### Structured logging

```go
log := logf.FromContext(ctx).WithValues("generation", obj.Generation)
log.Info("starting reconcile")                         // controller-runtime adds controller, namespace, name
log.Error(err, "failed to apply StatefulSet", "statefulset", sts.Name)

// ❌ Avoid
log.Info(fmt.Sprintf("reconciling %s/%s", req.Namespace, req.Name))
```

Levels: `Info` (V(0)) for lifecycle changes, `V(1)` per-reconcile summaries, `V(2)` decisions, `V(4)` dumps. Never log Secret data or full objects that may contain it.

##### Metrics

```go
var reconcileErrors = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "redis_operator_reconcile_errors_total",
		Help: "Total number of reconcile errors.",
	},
	[]string{"reason"},
)

func init() {
	metrics.Registry.MustRegister(reconcileErrors)
}
```

- controller-runtime already exports `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds`, workqueue depth/latency — do not duplicate them.
- Avoid `namespace`/`name` labels on custom metrics in large clusters (unbounded cardinality); use reasons/outcomes.
- Serve metrics over HTTPS with authn/authz (`filters.WithAuthenticationAndAuthorization`, Kubebuilder default) — the old plain `:8080` + kube-rbac-proxy pattern is gone from the scaffold.
- 1.37 control-plane components can expose Prometheus **native histograms** (beta-on) when scraped with the protobuf format; classic buckets stay available, so dashboards keep working.[^1]
- Useful 1.37 server metrics: `apiserver_watch_cache_initialization_duration_seconds`, `apiserver_storage_list_*` (with `storage`/`index` labels; the alpha `apiserver_cache_list_*` metrics are no longer exposed by default).[^2]

##### Health probes

```yaml
livenessProbe:
  httpGet: {path: /healthz, port: 8081}
  initialDelaySeconds: 15
  periodSeconds: 20
readinessProbe:
  httpGet: {path: /readyz, port: 8081}
  initialDelaySeconds: 5
  periodSeconds: 10
```

##### Events

Emit Events for user-visible transitions, not every reconcile: `Warning` for errors needing attention, `Normal` for successful transitions. Missing optional resources in a cleanup pass do not need Events unless users must act. With the `events.k8s.io/v1` recorder, `reason` is UpperCamelCase and `action` names what the controller did (`Reconcile`, `Scale`, `Delete`).

---

#### Testing

##### Unit tests with the fake client (verified with controller-runtime v0.25.2)

```go
func TestReconcileIsIdempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := cachev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	rc := &cachev1.RedisCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1},
		Spec:       cachev1.RedisClusterSpec{Replicas: 3, StorageClass: "fast", Version: "7.2.4"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rc).
		WithStatusSubresource(&cachev1.RedisCluster{}). // required for Status() writes
		Build()
	r := &RedisClusterReconciler{Client: c, Recorder: events.NewFakeRecorder(10), Backups: &fakeBackups{}}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "demo"}}

	for i := range 3 { // 1st adds the finalizer, then two identical passes
		if _, err := r.Reconcile(t.Context(), req); err != nil {
			t.Fatalf("reconcile %d: %v", i, err)
		}
	}
	var sts appsv1.StatefulSet
	if err := c.Get(t.Context(), req.NamespacedName, &sts); err != nil {
		t.Fatalf("StatefulSet not created: %v", err)
	}
	if got := *sts.Spec.Replicas; got != 3 {
		t.Fatalf("replicas = %d, want 3", got)
	}
	var got cachev1.RedisCluster
	if err := c.Get(t.Context(), req.NamespacedName, &got); err != nil {
		t.Fatal(err)
	}
	cond := meta.FindStatusCondition(got.Status.Conditions, conditionReady)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != "Progressing" {
		t.Fatalf("unexpected Ready condition: %+v", cond)
	}
}
```

`events.NewFakeRecorder` (client-go `tools/events`) matches the new recorder interface; `record.NewFakeRecorder` only fits the deprecated `GetEventRecorderFor` type. The fake client does not run CEL, defaulting, admission or garbage collection — test those with envtest.

##### Integration tests with envtest (1.37 control plane)

```go
func TestCRDValidationOnRealAPIServer(t *testing.T) {
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := env.Start() // KUBEBUILDER_ASSETS points at etcd + kube-apiserver 1.37
	if err != nil {
		t.Fatalf("starting envtest: %v", err)
	}
	t.Cleanup(func() { _ = env.Stop() })

	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	bad := &cachev1.RedisCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "default"},
		Spec:       cachev1.RedisClusterSpec{Replicas: 2, MinReplicas: 5, StorageClass: "fast"},
	}
	if err := c.Create(t.Context(), bad); !apierrors.IsInvalid(err) ||
		!strings.Contains(err.Error(), "minReplicas (5) must be <= replicas (2)") {
		t.Fatalf("expected CEL rejection, got %v", err)
	}
}
```

- Set `ENVTEST_K8S_VERSION=1.37` (Kubebuilder derives it from `k8s.io/api`) so CEL libraries, defaults and validation match the target cluster; testing against an older envtest hides 1.37 behaviour and vice versa.
- envtest has no controllers: no garbage collection, no Pods running, no namespace deletion — assert on API behaviour only, or run e2e on kind (`kindest/node:v1.37.x`).
- Test idempotency (reconcile twice), the deletion path with finalizer removal, status on success and error, and that invalid objects are rejected by CEL/webhooks.
- `t.Context()` (Go 1.24+) is cancelled when the test ends — prefer it over `context.TODO()`.

##### Manifest checks in CI

- `kubectl apply --dry-run=server` against a 1.37 API server (envtest binaries or kind) catches unknown fields, CEL errors and admission denials; `--dry-run=client` does not.
- For alpha fields, diff the dry-run output against the input: a dropped field means the gate is off.
- `kubectl apply --validate=strict` (default `strict` since 1.27) rejects unknown and duplicate fields.
- Helm: `helm template` + server-side dry run; set `kubeVersion` in `Chart.yaml` to the range you actually test.

---

#### Dynamic Resource Allocation (DRA)

DRA (`resource.k8s.io/v1`, GA 1.34) replaces device-plugin counting with claims. On 1.37:

| Capability | Status |
|---|---|
| `DeviceClass`, `ResourceClaim`, `ResourceClaimTemplate`, `ResourceSlice` (v1) | GA 1.34 |
| `firstAvailable` prioritized alternatives, `adminAccess` | GA 1.36 |
| Device taints/tolerations, `DeviceTaintRule` | GA 1.37 |
| Extended-resource requests satisfied by DRA (`DeviceClass.spec.extendedResourceName`) | GA 1.37 |
| `ResourceClaim.status.devices` (driver-reported device data, e.g. network interface IPs) | GA 1.37 |
| Standard attribute `resource.kubernetes.io/numaNode` | GA 1.37 (naming only) |
| Partitionable devices, consumable capacity, device binding conditions | beta-on since 1.36 |
| Workload/PodGroup claims, fractional capacity range | beta, **off** |
| Node-allocatable resources (CPU/memory via DRA), derived attributes, compatibility groups, optional node operations | alpha |

```yaml
apiVersion: resource.k8s.io/v1
kind: DeviceTaintRule
metadata:
  name: gpu-maintenance
spec:
  deviceSelector:
    driver: gpu.example.com
    pool: node-17
  taint:
    key: example.com/maintenance
    value: firmware-upgrade
    effect: NoSchedule          # NoSchedule | NoExecute | None
```

- An explicit empty `deviceSelector: {}` matches **every device of every driver** (kubectl 1.38 starts warning about it) — always scope the selector. An *omitted* `deviceSelector` is supposed to match nothing, but 1.37.0 had a bug that made it match every device cluster-wide (all DRA devices became unschedulable for `NoSchedule`/`NoExecute`); fixed in 1.37.1 — require ≥ 1.37.1 before applying DeviceTaintRules.[^2]
- CEL selectors in DeviceClasses/claims must handle missing attributes with `has()`/`orValue()`; 1.37 improves the error message but a missing attribute is still a runtime error.
- 1.37 preserves DRA Pod status fields (`resourceClaimStatuses`, `extendedResourceClaimStatus`, …) when older clients update Pod status without them — controllers that `Update` Pod status should still use patches.[^2]
- ResourceSlices can be filtered by pool with the field selector `spec.pool.name` (1.37).[^2]
- DRA driver authors: `kubeletplugin` makes `WatchHealthStatus` a mandatory `DRAPlugin` method (one-time Go API break); return `ErrHealthNotSupported` or disable the service with `HealthService(false)`.[^2]

---

#### Removed / Changed in 1.37 — Migration Notes

| Change | Who is affected | Action |
|---|---|---|
| `SELinuxMount` GA and on | SELinux nodes, shared volumes with CSI `seLinuxMount: true` | audit on 1.36 with `selinux-warning-controller`; set `seLinuxChangePolicy: Recursive` on conflicting Pods |
| `scheduling.k8s.io/v1alpha2` removed; Workload/PodGroup → `v1beta1`/`v1alpha3`; `DisruptionMode` is a struct; `PodGroupScheduled` → `PodGroupInitiallyScheduled` | gang-scheduling early adopters | delete v1alpha2 objects before upgrade; update clients |
| Static Pods cannot reference Secrets/ConfigMaps | static Pod manifests on nodes | inline config or use files on the host |
| Empty Pod-level `resources` ignored for QoS | Pods with `resources: {}` at Pod level | none, unless code computed QoS itself |
| Empty `image.reference` rejected in Pod templates | Helm/Kustomize templates with image volumes | make the value required |
| CEL set/map list `==`/`+` fix | CRDs/VAPs comparing set/map lists | re-test rules that compare or concatenate such lists |
| `PodCertificateRequest` v1 has no `pkixPublicKey`/`proofOfPossession` | signer implementations built on v1beta1 | migrate to v1 types |
| HPA scale-to-zero on by default | tools that treat `replicas: 0` as "disabled" | check the HPA `ScaledToZero` condition |
| StatefulSet `maxUnavailable` honoured again | StatefulSets that set it (silently ignored on 1.36) | verify rollout speed expectations |
| Admission webhooks no longer see auth virtual resources | webhooks that audited `tokenreviews`/`subjectaccessreviews` | move to audit logging or VAP |
| Concurrent watch decode (10 parallel conversions) | CRD conversion webhooks with low concurrency limits | raise limits / replicas |
| Feature gates removed (see [What's New](#whats-new-in-137)) | `--feature-gates` flags, kind/kubeadm configs | drop the flags or the component fails to start |
| kubeadm `v1beta3` removed | kubeadm config files | migrate with `kubeadm config migrate` from an older kubeadm (the release notes name the 1.35 binary), use `v1beta4` |
| kubelet `eventRecordQPS: 0` = unlimited; deprecated cAdvisor flags fatal | node configuration | set explicit QPS; remove cAdvisor flags |
| `kubectl run -f`, kube-dns deprecated; kube-proxy `ipvs` (deprecated since 1.35) now warns, `KubeProxyIPVS` gate | scripts, cluster setups | plan migration (nftables, CoreDNS) |
| controller-runtime v0.25 deprecations | operators upgrading from v0.24 | `GetEventRecorder`, `Validator[T]`/`Defaulter[T]`, `Client.Apply`, `RequeueAfter` |
| client-go `…WithContext` APIs | discovery/RESTMapper users | switch when touching the code; old APIs stay |
| `DRAPlugin.WatchHealthStatus` mandatory | DRA driver authors | implement or disable health service |
| Go ≥ 1.26 for `k8s.io/*` v0.37 | operator builds | bump `go` directive and CI images |

---

#### Production / Review Checklist

##### Version & compatibility

- [ ] `k8s.io/*` modules on one minor/patch (v0.37.x), controller-runtime v0.25.x, controller-tools v0.22+/v0.23, Go ≥ 1.26
- [ ] No API versions outside the [served list](#api-versions-served-by-137); no removed beta APIs; no `scheduling.k8s.io/v1alpha2`
- [ ] No alpha or beta-off fields relied upon (`evictionResponders`, `emptyDir.mode`, `httpGet.protocol: HTTP2`, StatefulSet `Recreate`, Job `spec.scheduling`, `lifecycle.stopSignal`, Workload/PodGroup) unless the gate is enabled in the repo's cluster config
- [ ] No 1.38 features (ML-DSA keys, `MaxMilliQuantity`, managedFields opt-out, `staticOrder`)
- [ ] Charts that also target ≤ 1.36 branch on `.Capabilities.KubeVersion` for 1.37-only behaviour (HPA `minReplicas: 0`, `storagemigration.k8s.io/v1`, Pod certificates)
- [ ] Removed feature gates are not passed to components

##### Manifests

- [ ] YAML 1.1 traps quoted (`"yes"`, `"NO"`, `"1.10"`, octal modes intended)
- [ ] Images pinned by digest (containers, sidecars, image volumes); no `:latest`
- [ ] Requests on every container (and Pod-level resources consistent with them); memory limit set
- [ ] Sidecars as init containers with `restartPolicy: Always`
- [ ] Probes: startup for slow starts, liveness without downstream dependencies, `preStop.sleep` + adequate `terminationGracePeriodSeconds`
- [ ] Resizes through `pods/resize`; `resizePolicy` set where restarts are unacceptable
- [ ] No `spec.replicas` on HPA-managed workloads; HPA scale-to-zero only with Object/External metrics
- [ ] PDB with `unhealthyPodEvictionPolicy: AlwaysAllow` for stateless apps
- [ ] Services use named target ports; EndpointSlices (not Endpoints) consumed by controllers
- [ ] CIDRs canonical (no host bits, no leading zeros)
- [ ] Short names do not collide with built-ins

##### CRDs & policies

- [ ] `status` subresource; `observedGeneration`; `metav1.Condition` list-map
- [ ] Every string/list/map bounded; CEL rules compile within cost budget (`format()` instead of `string(int)` concatenation in `messageExpression`)
- [ ] Immutability via `self == oldSelf` on the field; `optionalOldSelf` rules use `oldSelf.hasValue()`
- [ ] CEL libraries available on the oldest supported cluster (semver needs 1.34+)
- [ ] Storage version changes accompanied by a `StorageVersionMigration` and `storedVersions` cleanup
- [ ] VAP/MAP have bindings; MAP sets `reinvocationPolicy`; VAP rolled out with `Warn`/`Audit` first
- [ ] Webhooks: `sideEffects: None`, `matchConditions`, timeouts ≤ 10 s, own namespace excluded, cert rotation
- [ ] Conversion webhooks sized for concurrent calls

##### Controllers

- [ ] Fetch → NotFound → deletion/finalizer → apply → status; idempotent
- [ ] SSA with a stable field manager for owned children; owner references set
- [ ] Status via `/status` with optimistic locking; only when changed
- [ ] Errors returned for transient failures; no `Requeue: true`; permanent errors set conditions
- [ ] 429/`Retry-After` honoured for non-client-go calls
- [ ] `GetEventRecorder` (events.k8s.io) with matching RBAC; Events on transitions only
- [ ] Typed `Validator[T]`/`Defaulter[T]` webhooks
- [ ] Caches scoped (namespaces, label selectors, metadata-only); field indexes for map functions
- [ ] `ReconciliationTimeout` or per-call timeouts for external systems
- [ ] Unit tests with `WithStatusSubresource`; envtest at `ENVTEST_K8S_VERSION=1.37`

##### Security

- [ ] Namespaces labelled `pod-security.kubernetes.io/enforce: restricted` (version pinned)
- [ ] `runAsNonRoot`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, `readOnlyRootFilesystem`, `seccompProfile: RuntimeDefault`
- [ ] `hostUsers: false` for workloads that need root in the container
- [ ] SELinux shared-volume conflicts reviewed for 1.37
- [ ] RBAC least privilege, no wildcards, no secrets `list` cluster-wide; `nodes/logs` restricted
- [ ] `automountServiceAccountToken: false` by default; audience-bound projected tokens
- [ ] Secrets referenced, never inlined in CR specs or env files; nothing secret in logs
