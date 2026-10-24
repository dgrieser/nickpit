### Kubernetes 1.36: Manifests, Policies and Go Controllers — Complete Review Guide

> **Version**: Kubernetes v1.36.0 "ハル (Haru)", released 2026-04-22. Latest patch as of 2026-10-22: **v1.36.5** (git tag dated 2026-09-23; `dl.k8s.io/release/stable-1.36.txt` reports `v1.36.5`). The release schedule lists v1.36.6 as the next patch (target 2026-10-13); treat it as unreleased until it shows up on `dl.k8s.io`.
> **Support**: 1.36 is in standard support. It enters maintenance mode on **2027-04-28** and reaches end of life on **2027-06-28**. The newest minor is **1.37** (v1.37.0 on 2026-08-26, latest patch v1.37.1). Supported minors as of 2026-10-22: 1.37, 1.36, 1.35 (EOL 2027-02-28) and 1.34 (EOL **2026-10-27**).
> **Client libraries**: `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go` and the other staging modules at **v0.36.x** (latest v0.36.5); **controller-runtime v0.24.x** (v0.24.0 on 2026-04-30, v0.24.1 on 2026-05-11) and **controller-tools v0.21.x** pair with 1.36. All of them declare `go 1.26.0` in `go.mod`; Kubernetes 1.36 itself is built with Go 1.26 (1.36.5 uses Go 1.26.8).
> **Scope**: Kubernetes manifests (raw YAML, Kustomize output, Helm-rendered YAML), admission policies (ValidatingAdmissionPolicy, MutatingAdmissionPolicy, webhooks), CustomResourceDefinitions with CEL validation, and Go operators/controllers built with controller-runtime, client-go and Kubebuilder that target a 1.36 cluster.
> **Note**: This document is self-contained. It carries forward every still-correct house rule (operator principles, CRD design, status conditions, finalizers, RBAC, webhooks, observability, testing) and adds the 1.36-specific layer. Sections marked **[NEW in 1.36]** cover behaviour that differs from 1.35. Every manifest in this guide was applied with `kubectl apply --dry-run=server` (or for real) against a v1.36.2 kube-apiserver; boundary claims were re-checked against v1.35.0 and v1.37.0 apiservers; Go snippets compile against k8s.io/* v0.36.5 and controller-runtime v0.24.1 with Go 1.26. Runtime (kubelet) behaviour is sourced from the release notes and documentation, not executed.

---

#### What's New in 1.36

Kubernetes 1.36 contains 70 enhancements: 18 graduated to Stable, 25 entered Beta and 25 are new Alpha features. It also deprecates Service `.spec.externalIPs` and permanently disables the `gitRepo` volume plugin. The table lists every change that a manifest, policy or controller author can observe. "Default" means the state on a stock 1.36 control plane (verified on a v1.36.2 kube-apiserver through the `kubernetes_feature_enabled` metric).

##### Graduated to Stable (GA) in 1.36

| Feature | What authors see | Gate state in 1.36 |
|---|---|---|
| **MutatingAdmissionPolicy** (KEP-3962) | `admissionregistration.k8s.io/v1` `MutatingAdmissionPolicy` and `MutatingAdmissionPolicyBinding`, served by default. In 1.35 only `v1beta1` existed and was off by default. | `MutatingAdmissionPolicy` GA, on |
| **User namespaces** (KEP-127) | `pod.spec.hostUsers: false` is GA. | `UserNamespacesSupport` GA, locked on |
| **procMount** (`ProcMountType`) | `securityContext.procMount: Unmasked` is GA; it still requires `hostUsers: false`. | GA |
| **Image volumes** (KEP-4639) | `volumes[].image: {reference, pullPolicy}` mounts an OCI image/artifact read-only. | `ImageVolume` GA |
| **SELinux change policy** | `securityContext.seLinuxChangePolicy` (`MountOption`/`Recursive`) is GA; the `SELinuxMountReadWriteOncePod` behaviour is GA. The cluster-wide `SELinuxMount` gate stays **beta and off** in 1.36 (it becomes GA/on in 1.37). | `SELinuxChangePolicy` GA |
| **Fine-grained kubelet API authorization** (KEP-2862) | Monitoring agents can be granted `nodes/metrics`, `nodes/stats`, `nodes/pods`, `nodes/healthz`, `nodes/configz` instead of the broad `nodes/proxy`. | `KubeletFineGrainedAuthz` GA |
| **External ServiceAccount token signer** (KEP-740) | Cluster-level; no manifest changes. | GA |
| **Node log query** (KEP-2258) | `kubectl get --raw "/api/v1/nodes/<node>/proxy/logs/?query=kubelet"`; also needs `enableSystemLogQuery` in the kubelet config. | `NodeLogQuery` GA |
| **PSI metrics** (KEP-4205) | Kubelet Summary API exposes cgroup v2 pressure stall information. | `KubeletPSI` GA |
| **VolumeGroupSnapshot** (KEP-3476) | GA of the external-snapshotter group-snapshot APIs (CRDs installed by the CSI snapshot controller, not built into the apiserver). | n/a (CRDs) |
| **Mutable CSINode allocatable** (KEP-4876) | CSI drivers update volume attach limits at runtime. | `MutableCSINodeAllocatableCount` GA |
| **DRA admin access, prioritized lists** (KEP-5018, KEP-4816) | `resource.k8s.io/v1` `adminAccess` and `firstAvailable` subrequests are GA. | `DRAAdminAccess`, `DRAPrioritizedList` GA |
| **CSI service-account tokens via `secrets` field** | CSI drivers opt in to receiving tokens through the `secrets` field. | `CSIServiceAccountTokenSecrets` GA |
| **Declarative validation (validation-gen)** (KEP-5073) | Internal: built-in types are validated from `+k8s:` markers. No manifest change; error messages for a few fields changed wording. | `DeclarativeValidation` GA |
| **gogo protobuf removal** (KEP-5589) | Go: the temporary build-tagged `ProtoMessage()` marker methods were removed from `k8s.io/api` types (they made the types look like standard protobuf messages); use `k8s.io/apimachinery/pkg/runtime/serializer/protobuf`. | n/a |
| Portworx in-tree → CSI, split L3 cache CPU manager, DRA in PodResources | Node/cluster-level. | GA |

##### Promoted to Beta (enabled by default unless noted) in 1.36

| Feature | What authors see |
|---|---|
| **StrictIPCIDRValidation** (KEP-4858) **[NEW in 1.36]** | API fields **reject** IPs with leading zeros (`010.0.0.5`) and CIDRs with host bits set (`192.168.0.5/24`). In 1.35 these were accepted with a warning. Existing stored objects keep working (ratcheting). |
| **RelaxedServiceNameValidation** **[NEW in 1.36]** | Service names are validated as RFC 1123 DNS labels, so `8080-web` (leading digit) is now accepted. 1.35 rejects it. |
| **MutablePodResourcesForSuspendedJobs** (KEP-5440) **[NEW in 1.36]** | Container CPU/memory/GPU/extended-resource requests and limits in `spec.template` may be changed while a Job is suspended. 1.35 rejects the update as immutable. |
| **MutableSchedulingDirectivesForSuspendedJobs** | Scheduling directives of a suspended Job may be changed even after it started. |
| **InPlacePodLevelResourcesVerticalScaling** | Pod-level `spec.resources` can be resized in place through the `resize` subresource. |
| **InPlacePodVerticalScalingInitContainers** | Running non-sidecar init containers can be resized in place. |
| **RestartAllContainersOnContainerExits** | `restartPolicyRules` action `RestartAllContainers`. |
| **ConstrainedImpersonation** (KEP-5284) | Impersonators need separate `impersonate:*` permissions for the actions they perform; old rules still work. |
| **Staleness mitigation** (KEP-5647) | kube-controller-manager's DaemonSet, Job, ReplicaSet and StatefulSet controllers wait until their caches observed their own writes; client-go adds `AtomicFIFO`, `UnlockWhileProcessingFIFO`, `Store.LastStoreSyncResourceVersion()`. |
| **ComponentStatusz / ComponentFlagz** | `/statusz` and `/flagz` on components (`config.k8s.io/v1beta1`), readable by `system:monitoring`. |
| **UnknownVersionInteroperabilityProxy** | Mixed-version apiserver proxying (needs `--peer-ca-file`). |
| **ExtendWebSocketsToKubelet** | WebSocket `exec/attach/portforward` proxied straight to kubelets that advertise support. |
| **NodeDeclaredFeatures** | `node.status.declaredFeatures`. |
| **DRA**: partitionable devices, consumable capacity, device taints (ResourceSlice), binding conditions, extended resources, resource health status, granular status authorization | `resource.k8s.io/v1`. `DeviceTaintRules` need `resource.k8s.io/v1beta2` plus the `DRADeviceTaintRules` gate (beta, **off**). **ACTION REQUIRED**: with `DRAResourceClaimGranularStatusAuthorization` (beta, on), DRA drivers need `resourceclaims/driver` permissions and schedulers/controllers `resourceclaims/binding`. |
| `kubectl kuberc` | `~/.kube/kuberc` stays beta; 1.36 adds `credentialPluginPolicy` / `credentialPluginAllowlist`. |
| `WatchCacheInitializationPostStartHook` | Default flipped to on (apiserver readiness waits for watch caches). |

##### New Alpha in 1.36 (off by default — do not rely on in production manifests)

`HPAScaleToZero` improvements (`minReplicas: 0` is still **rejected** on a stock 1.36 cluster), Workload-Aware Scheduling (`scheduling.k8s.io/v1alpha2` Workload/PodGroup, Pod `spec.schedulingGroup.podGroupName` replacing 1.35's alpha `spec.workloadRef`, `GenericWorkload`, `TopologyAwareWorkloadScheduling`, `WorkloadAwarePreemption`, `WorkloadWithJob`; `scheduling.k8s.io/v1alpha1` Workload was removed), `ManifestBasedAdmissionControlConfig` (admission policies loaded from `staticManifestsDir`), `PersistentVolumeClaimUnusedSinceTime` (PVC `Unused` condition), `ShardedListAndWatch`, `NativeHistograms`, `PodLevelResourceManagers`, `CRIListStreaming`, `ImageVolumeWithDigest`, `DRAListTypeAttributes`, `DRANodeAllocatableResources`, `DRAResourcePoolStatus`, `DRAWorkloadResourceClaims`, `PodsAPI` (kubelet gRPC).

##### Changed defaults and behaviour in 1.36

- **StatefulSet `maxUnavailable` is OFF again.** `MaxUnavailableStatefulSet` became beta-on in 1.35.0 but is **disabled by default in 1.36** (kubernetes#137904) to work around a 1.35 regression in `podManagementPolicy: Parallel` rollouts (the same change was backported to 1.35.4, kubernetes#137926, so 1.35.4+ also drops the field). The actual Parallel-rollout fix (kubernetes#137666) and the re-enabled default (kubernetes#139466) ship only in 1.37; the 1.37 announcement links kubernetes#137409 for the bug. On a 1.36 apiserver `spec.updateStrategy.rollingUpdate.maxUnavailable` is **silently dropped** on create (verified: the stored object only has `partition: 0`). It is back on by default in 1.37. **[NEW in 1.36]**
- **Service `.spec.externalIPs` is deprecated.** The apiserver returns `Warning: spec.externalIPs is deprecated and may no longer be implemented in some clusters`. Removal is planned for v1.43 (KEP-5707). 1.35 accepts the field silently. **[NEW in 1.36]**
- **`gitRepo` volumes cannot be enabled anymore.** The `GitRepoVolumeDriver` gate is locked to false (`--feature-gates=GitRepoVolumeDriver=true` makes kube-apiserver 1.36 exit with "feature is locked to false"; 1.35 accepted it). The apiserver **still admits** Pods with `gitRepo` (warning: `deprecated in v1.11, and disabled by default in v1.33+`) — they fail on the node. Reviewers must flag every `gitRepo` volume. **[NEW in 1.36]**
- **CRD numeric formats are range-checked.** `format: int32`/`int64`/`float`/`double` in a CRD schema now reject out-of-range values (`replicas: 3000000000` with `format: int32` → `Checked value must be of type integer with format int32`). 1.35 stored the value. Existing objects are protected by validation ratcheting. **[NEW in 1.36]**
- **client-go informers**: with `AtomicFIFO` (beta, on) the store is updated with the whole list/relist before individual `OnAdd`/`OnUpdate`/`OnDelete` handlers run, resync contends less for locks, and `UnlockWhileProcessingFIFO` lets new watch events queue while handlers run. Handlers that assumed the old ordering or timing can observe differences. **[NEW in 1.36]**
- **client-go transport**: CA bundles referenced by file are reloaded from disk (`ClientsAllowCARotation`), and unused TLS transports are garbage-collected (`ClientsAllowTLSCacheGC`; disable with `KUBE_FEATURE_ClientsAllowTLSCacheGC=false`).
- **`kubectl debug`** default profile changed from `legacy` to `general` (`legacy` is planned for removal in v1.39).
- **`kubectl describe`** of multiple objects no longer lists events unless `--show-events` is passed.
- **Metric renames (ACTION REQUIRED)**: `volume_operation_total_errors` → `volume_operation_errors_total` (kube-controller-manager), `etcd_bookmark_counts` → `etcd_bookmark_total` (kube-apiserver). Update dashboards and alerts.
- **Audit logging**: `--audit-log-maxage` now defaults to 366 days and `--audit-log-maxbackup` to 100; `--audit-log-maxsize=0` disables rotation.
- **Ingress NGINX** (the community `kubernetes/ingress-nginx` controller) was retired on **2026-03-24**: no further releases or security fixes. Manifests that depend on its annotations need a migration plan (Gateway API or another controller).

##### Patch-release fixes that matter (1.36.1–1.36.5)

- 1.36.2: fixed a 1.36 regression that rejected scheduling-directive changes on suspended Jobs before the `JobSuspended` condition was set; fixed a 1.34+ regression with env values from Secrets containing non-UTF-8 data.
- 1.36.3: fixed a 1.36 **server-side apply** regression where patching a list/map field could fail with `422 required`; fixed a kubelet memory leak introduced in 1.36.
- 1.36.4: `cache.FakeCustomStore` implements the new `Bookmark` and `LastStoreSyncResourceVersion` methods again; fixed a scheduler preemption race that could leave a preemptor Pod unschedulable; fixed duplicate CDI device IDs after a retried DRA `PrepareResources`; `golang.org/x/net` and `golang.org/x/text` security updates.
- 1.36.5: built with Go 1.26.8; fixed a kube-scheduler crash in the DRA allocator when a ResourceClaim's device request counts overflow (oversized `firstAvailable` alternatives now fall through); fixed a Windows kube-proxy panic.

Run **1.36.3 or later** if controllers rely on server-side apply of list or map fields.

***

#### Do Not Use or Recommend

##### Features from Kubernetes 1.37 and later (unavailable on a 1.36 cluster)

A 1.36 apiserver does not know these API versions, fields or behaviours. Never suggest them for code that targets 1.36. Verified on a 1.36.2 apiserver: unknown fields are rejected by `kubectl apply` (strict server-side field validation) and **silently dropped** when field validation is off (`--validate=false`, many Go clients, some CD tools).

| Do not use on 1.36 | Introduced in | What happens on 1.36 |
|---|---|---|
| `storagemigration.k8s.io/v1` `StorageVersionMigration` (GA, on by default in 1.37) | 1.37 | `no matches for kind "StorageVersionMigration" in version "storagemigration.k8s.io/v1"` |
| PodCertificateRequest `v1`, `podCertificate` projected volumes, `ClusterTrustBundle` GA (`certificates.k8s.io`) | 1.37 GA | 1.36 has only beta APIs, off by default (gates `PodCertificateRequest`, `ClusterTrustBundle`, `ClusterTrustBundleProjection` beta/off) |
| `HorizontalPodAutoscaler` `minReplicas: 0` (scale to zero, beta/on in 1.37), `ScaledToZero` condition | 1.37 beta | `spec.minReplicas: Invalid value: 0: must be greater than or equal to 1` |
| `metrics.k8s.io/v1` (Metrics API GA) | 1.37 | 1.36 serves only `metrics.k8s.io/v1beta1` (via metrics-server) |
| `httpGet.protocol: HTTP2` probes (H2C) | 1.37 alpha | `unknown field "spec.containers[0].readinessProbe.httpGet.protocol"` |
| `volumeMounts[].bindMountOptions: [noexec, nosuid, nodev]` | 1.37 alpha | unknown field |
| `emptyDir.mode` (directory permission bits) | 1.37 alpha | unknown field |
| StatefulSet `updateStrategy.type: Recreate` | 1.37 alpha | `must be 'RollingUpdate' or 'OnDelete'` |
| Pod `spec.evictionResponders`, EvictionRequest API | 1.37 alpha | unknown field |
| `scheduling.k8s.io/v1beta1` Workload/PodGroup, CompositePodGroup (`v1alpha3`), Job `spec.scheduling` | 1.37 | 1.36 has only `scheduling.k8s.io/v1alpha2` Workload/PodGroup and Pod `spec.schedulingGroup` (both alpha, `GenericWorkload` off; the field is dropped on a stock 1.36 cluster). 1.35's alpha Pod `spec.workloadRef` was removed in 1.36 (tombstoned) |
| Node lifecycle conditions (`DrainInProgress`, `Drained`, `MaintenancePlanned`, …) | 1.37 alpha | not set by any 1.36 component |
| DRA device taints/tolerations GA, extended resources via DRA GA, `resource.kubernetes.io/numaNode`, derived attributes, compatibility groups | 1.37 | beta (taints in ResourceSlice, extended resources) or absent |
| `hostnameOverride` as GA, `HPAConfigurableTolerance` as GA, `RelaxedServiceNameValidation` as GA | 1.37 GA | all three are **beta and enabled** in 1.36 — usable, but still gated |
| KYAML output as Stable (`kubectl get -o kyaml`) | 1.37 GA | works in kubectl 1.36 (beta) — fine for humans, do not build tooling that requires GA guarantees |
| `SELinuxMount` enabled by default (mount `-o context=` for all volumes) | 1.37 GA | beta, **off** in 1.36 |
| `MemoryQoS` beta/on, `KubeletInUserNamespace` beta, `PodLevelResourceManagers` beta | 1.37 | alpha/off in 1.36 (the 1.36 announcement lists the tiered Memory QoS work under "Beta", but the `MemoryQoS` gate is alpha and off on 1.36) |
| `WatchListCompression`, `EtcdRangeStream`, `ConcurrentWatchObjectDecode` on by default | 1.37 | absent or off |
| Static Pods referencing ConfigMaps/Secrets strictly prohibited, `PreventStaticPodAPIReferences` gate removed | 1.37 | 1.36 already prevents it by default (beta gate, can be opted out) |
| controller-runtime **v0.25.x**, controller-tools **v0.22+**, k8s.io/* **v0.37.x** | 1.37 line | pair with 1.37; their API types contain fields a 1.36 server drops |
| client-go `*WithContext` discovery/RESTMapper methods (for example `DeferredDiscoveryRESTMapper.RESTMappingWithContext`) | client-go v0.37 | do not exist in v0.36. (The 1.37 announcement also mentions a `BookmarkFunc` on `ResourceEventHandlerFuncs`; client-go v0.37.1 does not have it — only `cache.FakeCustomStore.BookmarkFunc`.) |

Also deprecated in 1.37 (still fine on 1.36, but do not build new dependencies on them): `kube-dns`, `kubectl run -f`; kube-proxy `ipvs` mode is already deprecated since 1.35 and from 1.37 also logs a startup warning and sits behind the deprecated `KubeProxyIPVS` gate.

##### Deprecated or removed as of 1.36 — flag in review

Sources: the 1.36 release notes and the corresponding KEPs.

| Item | Status | Replacement |
|---|---|---|
| Service `.spec.externalIPs` | Deprecated in 1.36 (warning), removal planned for 1.43; CVE-2020-8554 MITM risk | `type: LoadBalancer`, `NodePort`, Gateway API |
| `gitRepo` volume | Plugin permanently disabled in 1.36 (deprecated since 1.11) | init container or git-sync sidecar into an `emptyDir` |
| In-tree Portworx volume plugin | Removed; redirected to CSI | Portworx CSI driver |
| `scheduling.k8s.io/v1alpha1` Workload | Removed | `v1alpha2` (alpha) |
| `apiserver.config.k8s.io/v1alpha1` `WebhookAdmissionConfiguration` | Removed | `apiserver.config.k8s.io/v1` |
| `WatchFromStorageWithoutResourceVersion`, `HonorPVReclaimPolicy`, `CSIMigrationPortworx` gates | Removed | — |
| PodCertificateRequest `spec.pkixPublicKey`, `spec.proofOfPossession` (beta API) | Deprecated | `spec.stubPKCS10Request` |
| `metav1.FieldsV1.Raw` direct access (Go) | Deprecated in v0.36 | `GetRawBytes()`, `GetRawString()`, `SetRawBytes()`, `NewFieldsV1()` |
| `k8s.io/apimachinery/pkg/util/httpstream` (`spdy`, `wsstream`) | Deprecated wrapper | `k8s.io/streaming` |
| controller-runtime `GetEventRecorderFor` (core/v1 Events) | Deprecated since v0.23 | `GetEventRecorder` (events.k8s.io) |
| controller-runtime `ctrl.Result{Requeue: true}` | Deprecated | `RequeueAfter` or return an error |
| controller-runtime `admission.CustomValidator`/`CustomDefaulter`, `WithCustomValidator` | Deprecated since v0.23 | generic `admission.Validator[T]`, `admission.Defaulter[T]` |
| controller-runtime `scheme.Builder` (`pkg/scheme`) | **Deprecated in v0.24** | `runtime.NewSchemeBuilder` from apimachinery |
| controller-runtime `client.Apply` patch type (`client.Patch(ctx, obj, client.Apply)`) | Deprecated | `client.Client.Apply()` with apply configurations |

Older removals that still show up in charts and must be rejected: `extensions/v1beta1` and `networking.k8s.io/v1beta1` Ingress (1.22), `batch/v1beta1` CronJob, `policy/v1beta1` PodDisruptionBudget and PodSecurityPolicy (1.25), `autoscaling/v2beta2` HPA (1.26), `flowcontrol.apiserver.k8s.io/v1beta2` (1.29), `flowcontrol.apiserver.k8s.io/v1beta3` (1.32), `storage.k8s.io/v1beta1` CSIStorageCapacity (1.27). All return `no matches for kind` on 1.36 (verified).

***

#### Version-Aware Review

- **Pin the target first.** Determine the cluster minor from the deployment target (`kubeVersion` in `Chart.yaml`, CI `kind`/envtest version, `k8s.io/*` versions in `go.mod`). Review against **1.36** only when that is the detected version.
- **Verify before claiming "unavailable".** Before saying an API, field or function is missing, check it against 1.36: `kubectl explain <type>.<field>` on a 1.36 apiserver, `kubectl api-versions`, the 1.36 OpenAPI, or `go doc` on v0.36.x modules. Say "not available in 1.36" only when the pinned version proves it. Fields that exist in `kubectl explain` may still be **alpha and dropped** unless the gate is enabled (for example `pod.spec.schedulingGroup`, which `kubectl explain` shows on 1.36 but the apiserver drops while `GenericWorkload` is off); check the gate table below.
- **Beta-on features are usable but gated.** `hostnameOverride`, `restartPolicyRules`, `fileKeyRef` env, pod-level `resources`, HPA `tolerance` are beta and enabled by default in 1.36. Accept them, but note that a managed provider may disable beta gates.
- **Beta APIs are off by default.** Since 1.24 new beta *API versions* are not served unless enabled with `--runtime-config` (for example `resource.k8s.io/v1beta2` for `DeviceTaintRule`, `certificates.k8s.io/v1beta1` for `ClusterTrustBundle`). A manifest using them needs a documented cluster prerequisite.
- **Client-library skew.** GA APIs work across neighbouring client/server minors, but a binary built with v0.37 types may send fields a 1.36 server drops. Keep `k8s.io/*` at v0.36.x and controller-runtime at v0.24.x for 1.36 targets.
- **Do not "upgrade" users into 1.37 behaviour.** Example: do not recommend `minReplicas: 0` for an HPA, `StatefulSet.updateStrategy.rollingUpdate.maxUnavailable` (dropped in 1.36), or `storagemigration.k8s.io/v1`.

***

#### API Versions Served by a 1.36 Cluster

`kubectl api-versions` on a stock v1.36.2 kube-apiserver (no aggregated APIs or CRDs):

```text
admissionregistration.k8s.io/v1   apiextensions.k8s.io/v1   apiregistration.k8s.io/v1
apps/v1   authentication.k8s.io/v1   authorization.k8s.io/v1   autoscaling/v1   autoscaling/v2
batch/v1   certificates.k8s.io/v1   coordination.k8s.io/v1   discovery.k8s.io/v1   events.k8s.io/v1
flowcontrol.apiserver.k8s.io/v1   networking.k8s.io/v1   node.k8s.io/v1   policy/v1
rbac.authorization.k8s.io/v1   resource.k8s.io/v1   scheduling.k8s.io/v1   storage.k8s.io/v1   v1
```

Every served group is GA-only by default. 1.35 serves the same list; 1.37 adds `storagemigration.k8s.io/v1`.

Compiled in but **disabled by default** in 1.36 (enable with `--runtime-config`, plus the feature gate where one exists): `admissionregistration.k8s.io/v1alpha1` and `v1beta1`, `coordination.k8s.io/v1alpha2` and `v1beta1` (coordinated leader election), `internal.apiserver.k8s.io/v1alpha1`, `networking.k8s.io/v1beta1` (IPAddress/ServiceCIDR betas; the GA `v1` versions are served), `resource.k8s.io/v1alpha3`, `v1beta1`, `v1beta2`, `storage.k8s.io/v1beta1`. Gated groups such as `scheduling.k8s.io/v1alpha2` (Workload/PodGroup), `certificates.k8s.io/v1alpha1` and `v1beta1` (ClusterTrustBundle, PodCertificateRequest) and `storagemigration.k8s.io/v1beta1` additionally need their feature gate. (Verified: `--runtime-config=api/all=true` with all alpha/beta gates on a v1.36.2 apiserver serves exactly these 14 non-GA versions.)

Notable GA kinds for authors: `admissionregistration.k8s.io/v1` ValidatingAdmissionPolicy(Binding) (GA 1.30) and **MutatingAdmissionPolicy(Binding) (GA 1.36)**; `resource.k8s.io/v1` DeviceClass/ResourceClaim/ResourceClaimTemplate/ResourceSlice (GA 1.34); `storage.k8s.io/v1` VolumeAttributesClass (GA 1.34; the `VolumeAttributesClass` gate is locked on in 1.36 and `v1` is the preferred storage version); `networking.k8s.io/v1` ServiceCIDR and IPAddress (GA 1.33); `coordination.k8s.io/v1` Lease; `flowcontrol.apiserver.k8s.io/v1` (GA 1.29).

***

#### Feature-Gate Defaults That Matter to Authors

Values read from `kubernetes_feature_enabled` on v1.35.0, v1.36.2 and v1.37.0 apiservers.

| Gate | 1.35 | **1.36** | 1.37 | Effect on manifests/controllers |
|---|---|---|---|---|
| `MutatingAdmissionPolicy` | beta, off | **GA, on** | GA | `admissionregistration.k8s.io/v1` MAP |
| `UserNamespacesSupport` | beta, on | **GA** | GA | `hostUsers: false` |
| `ProcMountType` | beta, on | **GA** | GA | `procMount: Unmasked` |
| `ImageVolume` | beta, on | **GA** | GA | `volumes[].image` |
| `StrictIPCIDRValidation` | alpha, off | **beta, on** | beta, on | rejects `010.0.0.5`, `10.0.0.5/8` |
| `RelaxedServiceNameValidation` | alpha, off | **beta, on** | GA | Service names may start with a digit |
| `MaxUnavailableStatefulSet` | beta, on in 1.35.0–1.35.3; off from 1.35.4 | **beta, off** | beta, on | `rollingUpdate.maxUnavailable` dropped on 1.36 |
| `MutablePodResourcesForSuspendedJobs` | alpha, off | **beta, on** | beta, on | resize suspended Jobs |
| `InPlacePodLevelResourcesVerticalScaling` | alpha, off | **beta, on** | beta, on | resize pod-level resources |
| `InPlacePodVerticalScalingInitContainers` | — | **beta, on** | GA | resize init containers |
| `RestartAllContainersOnContainerExits` | alpha, off | **beta, on** | beta | `restartPolicyRules` `RestartAllContainers` |
| `ContainerRestartRules` | beta, on | beta, on | beta | `restartPolicyRules` |
| `EnvFiles` | beta, on | beta, on | beta | `env[].valueFrom.fileKeyRef` |
| `HostnameOverride` | beta, on | beta, on | GA | `spec.hostnameOverride` |
| `PodLevelResources` | beta, on | beta, on | beta | pod-level `spec.resources` |
| `HPAConfigurableTolerance` | beta, on | beta, on | GA | `behavior.scaleUp/scaleDown.tolerance` |
| `HPAScaleToZero` | alpha, off | alpha, off | beta, on | `minReplicas: 0` |
| `DeploymentReplicaSetTerminatingReplicas` | beta, on | beta, on | beta | `.status.terminatingReplicas` |
| `ConstrainedImpersonation` | alpha, off | **beta, on** | beta | impersonation RBAC |
| `SELinuxChangePolicy` | beta, on | **GA** | GA | `seLinuxChangePolicy` |
| `SELinuxMount` | beta, off | beta, off | GA, on | mount-option relabeling for all volumes |
| `GitRepoVolumeDriver` | deprecated, off | **deprecated, locked off** | deprecated, off | `gitRepo` volumes never work |
| `AllowServiceExternalIPs` | — | GA, on | GA | externalIPs still functional (deprecated) |
| `PodCertificateRequest`, `ClusterTrustBundle(Projection)` | beta, off | beta, off | GA, on | pod certificates |
| `StorageVersionMigrator` | beta, off | beta, off | GA, on | `StorageVersionMigration` |
| `CRDObservedGenerationTracking` | beta, off | beta, on | beta | `observedGeneration` field on CRD conditions |
| `AtomicFIFO`, `UnlockWhileProcessingFIFO`, `InOrderInformers` (GA) | — | beta, on | beta | informer ordering/timing |
| `StaleControllerConsistency{DaemonSet,Job,ReplicaSet,StatefulSet}` | — | beta, on | beta | KCM read-your-writes |
| `ManifestBasedAdmissionControlConfig` | — | alpha, off | beta, on | static admission manifests |
| `PersistentVolumeClaimUnusedSinceTime` | — | alpha, off | beta, on | PVC `Unused` condition |

***

#### Usable Features GA in Earlier Releases (cumulative)

Everything below is GA and available on 1.36; recommend it where it fits.

| Area | Feature (GA since) |
|---|---|
| Pods | Native sidecars: `initContainers[].restartPolicy: Always` (1.33); `lifecycle.*.sleep` and `sleep.seconds: 0` (1.34); in-place container resize via the `resize` subresource and `resizePolicy` (1.35); `status.observedGeneration` on Pods (1.35); `supplementalGroupsPolicy: Strict` (1.35); `recursiveReadOnly` mounts (1.33); AppArmor via `securityContext.appArmorProfile` (1.30 beta, GA 1.31); `schedulingGates` (1.30); `matchLabelKeys` in pod (anti-)affinity (1.33) and topology spread (1.27 beta, `MatchLabelKeysInPodTopologySpread` still beta/on); relaxed env var names (any printable ASCII except `=`, 1.34); relaxed DNS search validation (1.34); image pull secret re-verification `KubeletEnsureSecretPulledImages` (beta/on) |
| Workloads | Job `podFailurePolicy` (1.31), `backoffLimitPerIndex` (1.33), `successPolicy` (1.33), `podReplacementPolicy` (1.34), `managedBy` (1.35); CronJob `timeZone` (1.27); StatefulSet `persistentVolumeClaimRetentionPolicy` (1.32), `minReadySeconds` (1.25), `ordinals.start` (1.31); PDB `unhealthyPodEvictionPolicy` (1.31); Deployment/ReplicaSet `podDeletionCost` annotation (beta/on); HPA `autoscaling/v2` (1.23) |
| Services/network | `trafficDistribution: PreferSameZone`/`PreferSameNode` (1.35; `PreferClose` is the older alias of `PreferSameZone`); EndpointSlices (Endpoints API deprecated since 1.33); dual-stack (1.23); `ServiceCIDR`/`IPAddress` (1.33); `loadBalancerClass` (1.24); nftables kube-proxy (1.33) |
| Admission | ValidatingAdmissionPolicy (1.30); webhook `matchConditions` (1.30); **MutatingAdmissionPolicy (1.36)** |
| CRDs | CEL `x-kubernetes-validations` (1.29) with `messageExpression`, `reason`, `fieldPath`, `optionalOldSelf` (1.30+); validation ratcheting (1.33); `selectableFields` / custom field selectors (1.32); `retryGenerateName` (1.32) |
| API machinery | Server-side apply (1.22); `WatchList` streaming (beta/on); consistent reads from cache (1.34 GA); ordered namespace deletion (1.34); `kubectl --subresource` (1.33); structured authentication/authorization config (1.34/1.32) |
| DRA | `resource.k8s.io/v1` (1.34); admin access, prioritized lists (1.36) |
| Storage | VolumeAttributesClass (1.34), volume populators (1.33), recover from expansion failure (1.34), ReadWriteOncePod (1.29), image volumes (1.36) |

***

#### Foundational Principles

| Principle | Description |
|---|---|
| **Level-based reconciliation** | React to current state, never to specific event types. The reconcile function must not receive or depend on event type information. |
| **Idempotency** | Every reconcile call must be safe to run repeatedly with the same outcome. |
| **Single responsibility** | One operator per application. One controller per root Kind. |
| **Declarative intent** | `spec` expresses *desired state*; the controller drives *actual state* toward it. |
| **Avoid edge-triggering** | Never build logic that depends on "what changed". Always reconcile the full object state. |
| **Open World assumption** | The system may miss events; design controllers that recover fully from a cold start. |
| **Caches are stale** | An informer cache can lag behind your own writes. Never treat "not in cache" as "does not exist" when you just created it; prefer deterministic names, server-side apply, and `AlreadyExists`/`Conflict` handling. 1.36's built-in controllers gained read-your-writes protection (KEP-5647) for exactly this reason. |
| **Declarative policy first** | Prefer CRD schema + CEL, ValidatingAdmissionPolicy and (new GA in 1.36) MutatingAdmissionPolicy over webhooks when the logic fits; they run in-process and cannot be taken down by a crashing webhook pod. |

---

#### Project Layout, Toolchain and Dependency Pinning

##### Recommended Scaffold (Kubebuilder)

```text
├── api/
│   └── v1alpha1/
│       ├── mykind_types.go          # API type definitions
│       ├── zz_generated.deepcopy.go # generated by controller-gen
│       └── groupversion_info.go
├── internal/
│   ├── controller/
│   │   └── mykind_controller.go     # Reconciler
│   └── webhook/
│       └── v1alpha1/mykind_webhook.go
├── config/
│   ├── crd/                         # Generated CRD manifests
│   ├── rbac/                        # Generated RBAC manifests
│   ├── webhook/
│   └── default/                     # Kustomize overlay
├── Dockerfile
└── Makefile                         # make generate, make manifests, make test
```

##### Toolchain Requirements

- **Go 1.26** or newer. `k8s.io/client-go` v0.36.x, controller-runtime v0.24.x and controller-tools v0.21.x all declare `go 1.26.0`; with Go 1.25 the `go` command refuses to build (`GOTOOLCHAIN=local`) or downloads go1.26 (`GOTOOLCHAIN=auto`, the default), and `go get k8s.io/client-go@v0.36.5` raises the module's `go` line to `1.26.0`. Container images and CI must use a Go 1.26 toolchain.
- **controller-runtime v0.24.x** — reconciler lifecycle, informer caches, client, webhooks. v0.23 targets 1.35, v0.25 targets 1.37.
- **controller-gen (controller-tools) v0.21.x** — CRD, RBAC, webhook and DeepCopy generation for 1.36 (v0.20 pairs with 1.35, v0.22/v0.23 with 1.37).
- **envtest** — `setup-envtest use 1.36.x` so integration tests run against a 1.36 apiserver and etcd.
- **kustomize** — kubectl 1.36 embeds Kustomize v5.8.1.

```go
// go.mod for a controller that targets Kubernetes 1.36
module example.com/widget-operator

go 1.26.0

require (
	k8s.io/api v0.36.5
	k8s.io/apimachinery v0.36.5
	k8s.io/client-go v0.36.5
	sigs.k8s.io/controller-runtime v0.24.1
)
```

Review rules:
- All `k8s.io/*` staging modules must share one minor (v0.36.x). Mixing `k8s.io/api v0.36` with `k8s.io/client-go v0.35` produces compile errors or subtle scheme mismatches.
- Do not pin `k8s.io/kubernetes` as a library dependency; it is not intended for import.
- Prefer the latest patch (v0.36.5 at the time of writing): v0.36.3 fixed a server-side-apply regression and v0.36.4 the `FakeCustomStore` interface break.

##### Makefile Targets (Mandatory)

```makefile
make generate      # DeepCopy, runtime.Object implementations
make manifests     # CRD YAML, RBAC, WebhookConfiguration
make install       # Apply CRDs to cluster
make test          # Unit + envtest integration tests
make docker-build  # Build operator image
```

---

#### Naming Conventions

##### Character Rules by Name Class

| Name class | Used for | Allowed characters | Start/end | Max length |
|---|---|---|---|---|
| **DNS Subdomain** (RFC 1123) | API groups, most `metadata.name` values (Pods, Deployments, ConfigMaps, CRs), CRD `spec.group` | lowercase `a–z`, digits, `-`, `.` | must start AND end with `[a-z0-9]` | 253 |
| **DNS Label** (RFC 1123) | `metadata.namespace`, container names, port names (≤15), **Service names in 1.36** | lowercase `a–z`, digits, `-` | start AND end with `[a-z0-9]` | 63 |
| **DNS Label** (RFC 1035, stricter) | Service names **before 1.36** (still the rule on 1.35 and older clusters) | lowercase `a–z`, digits, `-` | must start with `[a-z]` | 63 |
| **Qualified Name** | label keys, annotation keys, finalizer names | name: `[A-Za-z0-9-_.]`; optional DNS-subdomain prefix + `/` | name starts/ends alphanumeric | name ≤ 63; prefix ≤ 253 |
| **Label Value** | `metadata.labels` values | `[A-Za-z0-9-_.]` or empty | alphanumeric at both ends | 63 |
| **Annotation Value** | annotation values | any UTF-8 | n/a | all annotations together ≤ 256 KiB |
| **Env var name** | `env[].name` | any printable ASCII except `=` (GA since 1.34) | n/a | — |
| **ConfigMap / Secret data key** | `data` / `stringData` keys | `[A-Za-z0-9-_.]` | — | 253 |

**[NEW in 1.36]** With `RelaxedServiceNameValidation` (beta, on) the apiserver validates Service names as RFC 1123 labels: `8080-web` is accepted on 1.36 and rejected on 1.35 (`a DNS-1035 label must ... start with an alphabetic character`). Charts that must also install on ≤1.35 clusters should keep Service names starting with a letter. Do not flag a leading-digit Service name as invalid on a 1.36-only target.

> Underscore `_` and uppercase letters are never valid in DNS Subdomain or DNS Label names — that rules them out for `metadata.name`, namespaces, API groups and Service names.

##### API Group, Kind, Resource

| Element | Rule | Example |
|---|---|---|
| **API Group** | Lowercase DNS subdomain you own | `cache.example.com` |
| **Kind** | CamelCase, singular | `RedisCluster` |
| **Resource (plural)** | Lowercase, plural | `redisclusters` |
| **Short name** | 2–5 lowercase chars, no conflict with built-ins | `rc` |
| **Category** | Lowercase; `all` only if `kubectl get all` should list it | `cache` |

Do not use the empty group, single-word groups, or `*.k8s.io` / `*.kubernetes.io` — those are reserved (using them requires the `api-approved.kubernetes.io` annotation).

##### Go Type and JSON Field Names

```go
// Correct
type RedisCluster struct{ /* ... */ }
type RedisClusterSpec struct{ /* ... */ }
type RedisClusterStatus struct{ /* ... */ }
type RedisClusterList struct{ /* ... */ }
```

| Rule | Good | Bad |
|---|---|---|
| camelCase JSON | `replicaCount` | `replica_count`, `ReplicaCount` |
| Declarative nouns | `replicas` | `setReplicas` |
| Name references | `secretName` | `secret` |
| Object references | `secretRef` | — |
| Durations | `timeoutSeconds` (`int32`) or a `metav1.Duration` string | ad hoc `timeout: "30s"` parsed by hand |
| Established prefixes | `maxReplicas`, `minReadySeconds`, `maxSurge` | novel abbreviations like `tgtUtil` |

Enums are string type aliases with CamelCase values, never Go `iota` integers:

```go
type PhaseType string

const (
	PhasePending PhaseType = "Pending"
	PhaseRunning PhaseType = "Running"
)
```

##### Labels and Annotations

```yaml
metadata:
  labels:
    app.kubernetes.io/name: redis
    app.kubernetes.io/instance: redis-prod
    app.kubernetes.io/component: cache
    app.kubernetes.io/part-of: checkout
    app.kubernetes.io/managed-by: redis-operator
  annotations:
    cache.example.com/last-backup-time: "2026-01-01T00:00:00Z"
```

When code uses `client.ObjectKey`/`types.NamespacedName`, assume `Namespace` and `Name` already satisfy the DNS rules above unless the value comes from user input.

---

#### Workload Manifests

##### A 1.36 Pod Template That Passes `restricted`

The following Pod was accepted by a 1.36.2 apiserver in a namespace labelled `pod-security.kubernetes.io/enforce: restricted` (`enforce-version: v1.36`). Every beta field in it is enabled by default on 1.36.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: modern
  namespace: shop
  labels: {app: web}
spec:
  hostUsers: false                       # user namespaces, GA in 1.36
  automountServiceAccountToken: false
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
    supplementalGroupsPolicy: Strict     # GA in 1.35
    seccompProfile: {type: RuntimeDefault}
  resources:                             # pod-level resources (beta, on)
    requests: {cpu: 500m, memory: 256Mi}
    limits: {cpu: "1", memory: 512Mi}
  initContainers:
    - name: log-shipper                  # native sidecar (GA in 1.33)
      image: registry.example.com/shipper:2.3.1
      restartPolicy: Always
      securityContext:
        allowPrivilegeEscalation: false
        capabilities: {drop: ["ALL"]}
        readOnlyRootFilesystem: true
  containers:
    - name: app
      image: registry.example.com/app@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
      ports: [{name: http, containerPort: 8080}]
      resizePolicy:                      # in-place resize, GA in 1.35
        - {resourceName: cpu, restartPolicy: NotRequired}
        - {resourceName: memory, restartPolicy: RestartContainer}
      restartPolicy: Always              # required when restartPolicyRules is set
      restartPolicyRules:                # ContainerRestartRules (beta, on)
        - action: Restart
          exitCodes: {operator: In, values: [42]}
      env:
        - name: DB_URL
          valueFrom:
            fileKeyRef: {volumeName: config, path: app.env, key: DB_URL}   # EnvFiles (beta, on)
      lifecycle:
        preStop:
          sleep: {seconds: 5}            # GA in 1.34
      readinessProbe:
        httpGet: {path: /readyz, port: http}
      livenessProbe:
        httpGet: {path: /healthz, port: http}
        periodSeconds: 20
      securityContext:
        allowPrivilegeEscalation: false
        capabilities: {drop: ["ALL"]}
        readOnlyRootFilesystem: true
      volumeMounts:
        - {name: model, mountPath: /models, readOnly: true}
        - {name: config, mountPath: /config}
  volumes:
    - name: model
      image:                              # image volume, GA in 1.36
        reference: registry.example.com/models/resnet:1.0
        pullPolicy: IfNotPresent
    - name: config
      emptyDir: {sizeLimit: 1Mi}
```

Review notes for this template:
- **Image volumes (GA in 1.36)** mount an OCI image or artifact read-only. They need a container runtime that implements image volumes (check the runtime's release notes; the apiserver accepts the field regardless); pin the `reference` by digest for reproducibility; `pullPolicy` follows the same rules as container images.
- **`fileKeyRef`** (beta) reads `KEY='value'` lines from a file in an `emptyDir` that an init container wrote; values must be single-quoted (a POSIX-shell subset since 1.35). An invalid key prevents the Pod from starting. Flag it if the writing init container is missing.
- **`restartPolicyRules`** requires an explicit container `restartPolicy` (the apiserver rejects rules without it). Only `exitCodes` conditions exist; at most 20 rules.
- **Pod-level `resources`** (beta) set a budget for the whole Pod; container limits must not exceed it. In 1.36 pod-level CPU/memory can also be resized in place (`InPlacePodLevelResourcesVerticalScaling`, beta).
- `supplementalGroupsPolicy: Strict` makes kubelet reject the Pod on nodes whose runtime cannot honour it — check node/runtime support before rolling out.

##### Resource Requests, Limits and In-Place Resize

```yaml
# BAD: no requests (BestEffort), CPU limit throttles latency-sensitive code
resources:
  limits: {cpu: 200m}

# GOOD: memory request == limit, CPU request without a limit for latency-sensitive services
resources:
  requests: {cpu: 250m, memory: 512Mi}
  limits: {memory: 512Mi}
```

- In-place resize (GA in 1.35) is requested through the `resize` subresource: `kubectl patch pod web --subresource resize --patch '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"500m"}}}]}}'`. Editing `spec.containers[*].resources` with a plain `update`/`patch` on the main resource is still rejected.
- **[NEW in 1.36]** A resize that exceeds the node's allocatable capacity, or targets a node OS that does not support resize, fails in admission instead of being marked `Infeasible` later.
- **[NEW in 1.36]** Running non-sidecar init containers may be resized (beta), and pod-level `resources` may be resized (beta).
- **[NEW in 1.36]** A non-sidecar init container may not declare `resizePolicy` with `restartPolicy: RestartContainer` (`must not be set to 'RestartContainer' for non-sidecar initContainers`); 1.35 accepted it. Remove such entries from init containers (sidecars with `restartPolicy: Always` may keep them).
- Controllers that write Pod templates (Deployments etc.) never resize existing Pods; a template change still rolls the workload.

##### Deployments

The Deployment, StatefulSet and CronJob examples below leave out the security context from the template above to stay short. In a namespace that enforces `restricted`, the apiserver admits the controller objects but warns (`would violate PodSecurity "restricted:latest"` — the `warn` label of the Namespace below has no `warn-version`, so it evaluates `latest`) and the controllers' Pods are then rejected — copy the `securityContext` blocks into real templates.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: shop}
spec:
  replicas: 3
  revisionHistoryLimit: 5
  selector: {matchLabels: {app: web}}
  strategy:
    type: RollingUpdate
    rollingUpdate: {maxSurge: 25%, maxUnavailable: 0}
  template:
    metadata: {labels: {app: web}}
    spec:
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: topology.kubernetes.io/zone
          whenUnsatisfiable: ScheduleAnyway
          labelSelector: {matchLabels: {app: web}}
          matchLabelKeys: [pod-template-hash]   # spread each rollout revision separately
      terminationGracePeriodSeconds: 30
      containers:
        - name: web
          image: registry.example.com/web:3.1.4
          ports: [{name: http, containerPort: 8080}]
          resources:
            requests: {cpu: 100m, memory: 128Mi}
            limits: {memory: 128Mi}
          lifecycle:
            preStop: {sleep: {seconds: 0}}       # sleep 0 is valid since 1.34
```

- `spec.selector` is immutable; changing it requires a new Deployment.
- `status.terminatingReplicas` (beta, on since 1.35) reports Pods still terminating; rollout tooling can wait on it.
- When an HPA manages a Deployment, do not set `replicas` in the applied manifest — every `kubectl apply`/GitOps sync resets the HPA's value. Remove the field (or leave it out of the server-side-apply field set).

##### StatefulSets **[NEW in 1.36: `maxUnavailable` disabled]**

```yaml
# BAD on 1.36: maxUnavailable is silently dropped (MaxUnavailableStatefulSet beta, OFF in 1.36)
apiVersion: apps/v1
kind: StatefulSet
metadata: {name: db, namespace: shop}
spec:
  serviceName: db
  replicas: 3
  podManagementPolicy: Parallel
  selector: {matchLabels: {app: db}}
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 2        # stored as {"partition":0} on 1.36 and 1.35.4+; kept on 1.35.0–1.35.3 and 1.37
  persistentVolumeClaimRetentionPolicy:
    whenDeleted: Retain
    whenScaled: Delete
  minReadySeconds: 10
  template:
    metadata: {labels: {app: db}}
    spec:
      containers:
        - name: db
          image: registry.example.com/db:16.4
          volumeMounts: [{name: data, mountPath: /var/lib/db}]
  volumeClaimTemplates:
    - metadata: {name: data}
      spec:
        accessModes: [ReadWriteOncePod]
        resources: {requests: {storage: 10Gi}}
```

- On 1.36 rollouts proceed one Pod at a time regardless of `maxUnavailable`. A GitOps tool that diffs against the stored object reports permanent drift for the dropped field — remove it for 1.36 targets.
- `persistentVolumeClaimRetentionPolicy` (GA 1.32): `whenScaled: Delete` deletes PVCs of scaled-down ordinals — flag it for databases unless data loss on scale-down is intended.
- `updateStrategy.type: Recreate` does not exist on 1.36 (alpha in 1.37).
- `volumeClaimTemplates` are immutable; changing them needs an orphaning delete (`kubectl delete sts --cascade=orphan`) and re-create.

##### Jobs and CronJobs

```yaml
apiVersion: batch/v1
kind: Job
metadata: {name: train, namespace: ml}
spec:
  suspend: true                     # let a queue controller (e.g. Kueue) admit it
  completions: 4
  parallelism: 4
  completionMode: Indexed
  backoffLimitPerIndex: 1           # GA 1.33
  maxFailedIndexes: 1
  podReplacementPolicy: Failed      # GA 1.34: no replacement until the old Pod is fully gone
  successPolicy:                    # GA 1.33
    rules:
      - succeededIndexes: "0"
  podFailurePolicy:                 # GA 1.31
    rules:
      - action: FailJob
        onExitCodes: {containerName: main, operator: In, values: [42]}
      - action: Ignore
        onPodConditions: [{type: DisruptionTarget}]
  ttlSecondsAfterFinished: 3600
  template:
    spec:
      restartPolicy: Never          # podFailurePolicy requires Never
      containers:
        - name: main
          image: registry.example.com/train:1.0
          resources:
            requests: {cpu: "1", memory: 1Gi}
            limits: {memory: 1Gi}
```

**[NEW in 1.36]** While `suspend: true`, the container `resources` of `spec.template` may be changed (`MutablePodResourcesForSuspendedJobs`, beta, on). Verified: patching `requests.cpu` on a suspended Job succeeds on 1.36 and fails on 1.35 with `field is immutable`; after `suspend: false` the template is immutable again. Queue controllers can therefore right-size a Job before admission; do not assume it on 1.35.

- `managedBy` (GA 1.35) hands a Job to an external controller (for example MultiKueue); the built-in Job controller then ignores it. The field is immutable.
- CronJob: always set `timeZone` (IANA name) instead of relying on the controller's local time; `concurrencyPolicy: Forbid` for jobs that must not overlap; `startingDeadlineSeconds` to bound catch-up.

```yaml
apiVersion: batch/v1
kind: CronJob
metadata: {name: report, namespace: shop}
spec:
  schedule: "15 2 * * *"
  timeZone: Europe/Berlin
  concurrencyPolicy: Forbid
  startingDeadlineSeconds: 300
  jobTemplate:
    spec:
      backoffLimit: 2
      template:
        spec:
          restartPolicy: OnFailure
          containers: [{name: report, image: registry.example.com/report:1.2}]
```

Idempotent one-shot Jobs and CronJobs can rely on Kubernetes termination and safe re-runs. Require explicit graceful shutdown only when there is cleanup that process exit cannot handle (held leases, locks, transactions, non-idempotent remote writes). External calls still need timeouts or `activeDeadlineSeconds`; absence of signal handling alone is not a bug.

##### HorizontalPodAutoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: {name: web, namespace: shop}
spec:
  scaleTargetRef: {apiVersion: apps/v1, kind: Deployment, name: web}
  minReplicas: 2                     # 0 is rejected on 1.36 (HPAScaleToZero is alpha)
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target: {type: Utilization, averageUtilization: 70}
  behavior:
    scaleUp:
      tolerance: 0.05                # HPAConfigurableTolerance, beta/on; stored as "50m"
      stabilizationWindowSeconds: 0
      policies: [{type: Percent, value: 100, periodSeconds: 15}]
    scaleDown:
      tolerance: 0.15
      stabilizationWindowSeconds: 300
```

- `tolerance` is a `resource.Quantity`; `0.05` round-trips as `"50m"`. Without it the cluster-wide default (10%) applies.
- `minReplicas: 0` fails validation on 1.36 (`must be greater than or equal to 1`) unless the cluster enables the alpha `HPAScaleToZero` gate — and even then 1.36 requires at least one `Object` or `External` metric (`must specify at least one Object or External metric to support scaling to zero replicas`); it is beta/on only from 1.37.
- CPU-utilization targets require CPU requests on every container of the target.

##### PodDisruptionBudgets

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata: {name: web, namespace: shop}
spec:
  maxUnavailable: 1
  unhealthyPodEvictionPolicy: AlwaysAllow   # GA 1.31: do not block drains on crash-looping Pods
  selector: {matchLabels: {app: web}}
```

Flag PDBs with `maxUnavailable: 0` or `minAvailable` equal to the replica count — they block node drains forever.

##### ConfigMaps and Secrets

- Mark generated, content-addressed ConfigMaps/Secrets `immutable: true`; the kubelet stops watching them and accidental edits are rejected.
- Never put secrets in ConfigMaps, CRD specs, annotations or command-line arguments.
- `stringData` is write-only convenience; reviewers should not expect it in `kubectl get -o yaml`. `kubectl diff` masks Secret values (`*** (before)` / `*** (after)`, also in kubectl 1.35); kubectl 1.36 adds `--show-secrets` to reveal them.

##### Removed volume types

```yaml
# BAD: gitRepo is permanently disabled on 1.36 — the Pod is admitted (with a warning) but never starts
volumes:
  - name: src
    gitRepo: {repository: https://example.com/repo.git}

# GOOD: clone in an init container into an emptyDir
initContainers:
  - name: clone
    image: registry.example.com/git:2.47
    args: ["clone", "--depth=1", "https://example.com/repo.git", "/src"]
    volumeMounts: [{name: src, mountPath: /src}]
volumes:
  - name: src
    emptyDir: {}
```

---

#### Pod Security: securityContext, Pod Security Admission, User Namespaces

##### Operator / Workload Security Context

```yaml
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
    seccompProfile:
      type: RuntimeDefault
  containers:
    - name: manager
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: ["ALL"]
```

##### Pod Security Admission

Label every namespace explicitly; pin `enforce-version` so a control-plane upgrade does not silently change the policy:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: shop
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: v1.36
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/audit: restricted
```

##### User Namespaces (GA in 1.36) **[NEW in 1.36]**

`hostUsers: false` maps container UIDs/GIDs to an unprivileged host range, so root in the container is not root on the node. The API feature is GA and locked on, but nodes need Linux ≥ 6.3 with idmap-mount support on all volume filesystems, containerd ≥ 2.0 or CRI-O ≥ 1.25, and runc ≥ 1.2 or crun ≥ 1.9. Pods using user namespaces may not set `hostNetwork`, `hostIPC` or `hostPID` and may not use `volumeDevices`; NFS volumes cannot be mounted.

Pod Security Admission relaxes checks for user-namespaced Pods: `runAsNonRoot` and `runAsUser` are not checked under Baseline or Restricted, and Baseline also allows `procMount: Unmasked`. Restricted still requires the default `procMount`. Verified on 1.36: a root Pod with `hostUsers: false` and no `runAsNonRoot` is admitted by `restricted` with `enforce-version` `v1.35`, `v1.36` or `latest`, but rejected under `v1.34` and older policy versions (`runAsNonRoot != true`). The same Pod with `hostUsers: true` is rejected.

```yaml
# Accepted by restricted:v1.36 — root inside a user namespace
apiVersion: v1
kind: Pod
metadata: {name: userns-root, namespace: shop}
spec:
  hostUsers: false
  securityContext:
    seccompProfile: {type: RuntimeDefault}
  containers:
    - name: app
      image: registry.example.com/app:1.0
      securityContext:
        allowPrivilegeEscalation: false
        capabilities: {drop: ["ALL"]}
```

- `procMount: Unmasked` (GA in 1.36) is rejected by validation unless `hostUsers: false` (`` `hostUsers` must be false to use `Unmasked` ``), allowed by Baseline for user-namespaced Pods, and always rejected by Restricted.
- Do not tell authors that `hostUsers: false` is "alpha" or "beta" on 1.36; it is GA. Do tell them it needs node support — a cluster on cgroup/kernel/runtime versions below the requirements fails at Pod start.
- `UserNamespacesHostNetworkSupport` (combining `hostNetwork: true` with `hostUsers: false`) is alpha in 1.36 — do not use.

##### SELinux

- `securityContext.seLinuxChangePolicy` (GA in 1.36): `MountOption` (fast, mount with `-o context=`) or `Recursive` (relabel every file). Pods that share a volume on the same node but run with different SELinux labels must use `Recursive`.
- The cluster-wide switch `SELinuxMount` stays beta/off in 1.36 and becomes GA/on in 1.37 (the 1.36 announcement's wording that mount-option labelling "defaults to all volumes" describes the graduation of `SELinuxChangePolicy`/`SELinuxMountReadWriteOncePod`; the `kubernetes_feature_enabled` metric of a 1.36 apiserver reports `SELinuxMount` beta and disabled). **Use 1.36 to audit**: set `seLinuxChangePolicy: Recursive` on Pods (including Pod templates in Deployments, StatefulSets, DaemonSets and CRDs) that share volumes with differently-labelled Pods before upgrading to 1.37, or they can fail to start.

##### Fine-Grained Kubelet Authorization (GA in 1.36)

Monitoring agents should get only the kubelet subresources they need instead of `nodes/proxy` (which allows `exec` into any container, even with the `get` verb over WebSockets):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata: {name: node-metrics-reader}
rules:
  - apiGroups: [""]
    resources: ["nodes/metrics", "nodes/stats", "nodes/pods", "nodes/healthz"]
    verbs: ["get"]
```

Kubelet path → subresource mapping: `/stats/*` → `nodes/stats`, `/metrics/*` → `nodes/metrics`, `/logs/*` → `nodes/log`, `/pods` and `/runningPods/` → `nodes/pods` (falls back to `nodes/proxy`), `/healthz` → `nodes/healthz`, `/configz` → `nodes/configz`; everything else needs `nodes/proxy`. Flag any new `nodes/proxy` grant.

---

#### Networking: Services, NetworkPolicy, Ingress and Gateway API

##### Services

```yaml
apiVersion: v1
kind: Service
metadata: {name: web, namespace: shop}
spec:
  selector: {app: web}
  ports: [{name: http, port: 80, targetPort: http, appProtocol: kubernetes.io/h2c}]
  trafficDistribution: PreferSameNode     # GA in 1.35 (PreferSameZone, PreferSameNode)
  ipFamilyPolicy: PreferDualStack
```

- **[NEW in 1.36] `spec.externalIPs` is deprecated.** It produces `Warning: spec.externalIPs is deprecated and may no longer be implemented in some clusters`, and is scheduled for removal in v1.43 (KEP-5707). It enables traffic interception (CVE-2020-8554). Replace with `type: LoadBalancer`, `NodePort`, or Gateway API. Clusters should also run the `DenyServiceExternalIPs` admission plugin.
- `trafficDistribution: PreferClose` still works and means the same as `PreferSameZone`; prefer the explicit names.
- Use named `targetPort`s; set `appProtocol` for L7-aware proxies.
- The `Endpoints` API is deprecated (since 1.33) in favour of `discovery.k8s.io/v1` EndpointSlices; controllers that read Endpoints should migrate.

##### Strict IP and CIDR Validation **[NEW in 1.36]**

`StrictIPCIDRValidation` (beta, on) turns 1.35's warnings into errors for **new** values in every IP/CIDR API field (Services, Pods, NetworkPolicies, EndpointSlices, …):

```yaml
# BAD on 1.36 — rejected
spec:
  loadBalancerSourceRanges: ["010.0.0.0/8"]     # "must not have leading 0s in IP or prefix length"
---
spec:
  ingress:
    - from:
        - ipBlock: {cidr: 192.168.0.5/24}       # "must not have bits set beyond the prefix length"

# GOOD
spec:
  loadBalancerSourceRanges: ["10.0.0.0/8"]
---
spec:
  ingress:
    - from:
        - ipBlock: {cidr: 192.168.0.0/24}       # or 192.168.0.5/32 for a single host
```

On 1.35 the same objects are stored with warnings such as `CIDR value "192.168.0.5/24" is ambiguous in this context`. Already-stored objects stay valid; controllers that write IPs back must canonicalize them (`netip.Prefix.Masked()`, `netip.Addr.String()`).

##### NetworkPolicy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: default-deny, namespace: shop}
spec:
  podSelector: {}
  policyTypes: [Ingress, Egress]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: allow-dns, namespace: shop}
spec:
  podSelector: {}
  policyTypes: [Egress]
  egress:
    - to:
        - namespaceSelector:
            matchLabels: {kubernetes.io/metadata.name: kube-system}
          podSelector:
            matchLabels: {k8s-app: kube-dns}
      ports:
        - {port: 53, protocol: UDP}
        - {port: 53, protocol: TCP}
```

- A default-deny egress policy without a DNS allowance breaks name resolution.
- `namespaceSelector` and `podSelector` in the **same** `to`/`from` element are ANDed; in separate list elements they are ORed — a classic review bug.
- NetworkPolicy has no effect without a CNI that enforces it.

##### Ingress and Gateway API

- `networking.k8s.io/v1` Ingress is GA and frozen; new features land in Gateway API (CRDs from `gateway.networking.k8s.io`, not built into the apiserver).
- **The community Ingress NGINX controller was retired on 2026-03-24** — no releases, bug fixes or security fixes since then. Manifests that rely on `nginx.ingress.kubernetes.io/*` annotations (especially `configuration-snippet`/`server-snippet`, the source of CVE-2025-1974-class issues) need a migration plan to Gateway API or a maintained controller; Ingress2Gateway 1.0 helps translate them.
- Always set `ingressClassName`; the `kubernetes.io/ingress.class` annotation is deprecated.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata: {name: web, namespace: shop}
spec:
  ingressClassName: example-gateway-class
  rules:
    - host: web.example.com
      http:
        paths:
          - {path: /, pathType: Prefix, backend: {service: {name: web, port: {name: http}}}}
```

---

#### Admission Control: ValidatingAdmissionPolicy, MutatingAdmissionPolicy, Webhooks

##### Choosing a Mechanism

```text
CRD schema + CEL (x-kubernetes-validations)   → validation of your own types, no extra objects
ValidatingAdmissionPolicy (GA 1.30)           → validation of any type, in-process CEL
MutatingAdmissionPolicy (GA 1.36)             → defaulting/mutation of any type, in-process CEL
Admission webhook                             → needs external data, side effects, or logic CEL cannot express
```

Prefer left to right. In-process policies have no network hop, no TLS certificates to rotate, and no failure mode where a crashed webhook Deployment blocks every write.

##### MutatingAdmissionPolicy (GA in 1.36) **[NEW in 1.36]**

`admissionregistration.k8s.io/v1` `MutatingAdmissionPolicy` and `MutatingAdmissionPolicyBinding` are served and enabled by default on 1.36. On 1.35 the `v1` kinds do not exist (`no matches for kind "MutatingAdmissionPolicy" in version "admissionregistration.k8s.io/v1"`), and the `v1beta1` API and gate were off by default.

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingAdmissionPolicy
metadata:
  name: default-team-label
spec:
  failurePolicy: Fail
  reinvocationPolicy: IfNeeded
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["configmaps"]
  matchConditions:
    - name: no-team-label
      expression: "!has(object.metadata.labels) || !('team' in object.metadata.labels)"
  mutations:
    - patchType: ApplyConfiguration
      applyConfiguration:
        expression: >
          Object{
            metadata: Object.metadata{
              labels: {"team": "unassigned"}
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
        kubernetes.io/metadata.name: shop
```

JSON Patch variant (use `jsonpatch.escapeKey()` for keys containing `/` or `~`):

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingAdmissionPolicy
metadata:
  name: add-annotation-jsonpatch
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["configmaps"]
  reinvocationPolicy: Never
  mutations:
    - patchType: JSONPatch
      jsonPatch:
        expression: >
          has(object.metadata.annotations) ?
          [JSONPatch{op: "add", path: "/metadata/annotations/" + jsonpatch.escapeKey("example.com/seen"), value: "true"}] :
          [JSONPatch{op: "add", path: "/metadata/annotations", value: {"example.com/seen": "true"}}]
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingAdmissionPolicyBinding
metadata:
  name: add-annotation-jsonpatch
spec:
  policyName: add-annotation-jsonpatch
  matchResources:
    namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: shop
```

Both policies were applied to a 1.36 apiserver; a ConfigMap created afterwards in the bound namespace carried `team: unassigned` and `example.com/seen: "true"`.

Review rules for MutatingAdmissionPolicy:
- `reinvocationPolicy` is **required** (`Never` or `IfNeeded`); `mutations` must not be empty; `patchType` is `ApplyConfiguration` or `JSONPatch`.
- Prefer `ApplyConfiguration` (structured merge, idempotent by construction). `JSONPatch` `add` on a missing parent path fails — guard with `has(...)` as above.
- Guard with `matchConditions` so the mutation is idempotent and does not override user-set values ("default only if absent").
- A policy cannot match `MutatingAdmissionPolicy`/`MutatingAdmissionPolicyBinding` objects themselves, and `DELETE` cannot be matched.
- `failurePolicy` defaults to `Fail`. A typo in a CEL expression with `Fail` blocks every matching write — test policies with `--dry-run=server` before binding them cluster-wide.
- Mutations run before validation; ValidatingAdmissionPolicies and validating webhooks see the mutated object.
- Do not migrate a 1.35 `v1beta1` manifest by only changing `apiVersion` without testing — validate it against a 1.36 apiserver.

##### ValidatingAdmissionPolicy

GA since 1.30.

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: require-resources-and-digest
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["deployments", "statefulsets", "daemonsets"]
  matchConditions:
    - name: exclude-system-users
      expression: "!request.userInfo.username.startsWith('system:')"
  variables:
    - name: containers
      expression: "object.spec.template.spec.containers + (has(object.spec.template.spec.initContainers) ? object.spec.template.spec.initContainers : [])"
  validations:
    - expression: "variables.containers.all(c, has(c.resources) && has(c.resources.requests) && 'memory' in c.resources.requests)"
      message: every container must request memory
      reason: Invalid
    - expression: "variables.containers.all(c, c.image.contains('@sha256:'))"
      messageExpression: "'images must be pinned by digest: ' + variables.containers.filter(c, !c.image.contains('@sha256:')).map(c, c.name).join(', ')"
  auditAnnotations:
    - key: unpinned-images
      valueExpression: "string(size(variables.containers.filter(c, !c.image.contains('@sha256:'))))"
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: require-resources-and-digest
spec:
  policyName: require-resources-and-digest
  validationActions: [Deny, Audit]
  matchResources:
    namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: shop
```

Verified on 1.36: a Deployment with `image: registry.example.com/web:3.1.4` is rejected with `ValidatingAdmissionPolicy 'require-resources-and-digest' with binding 'require-resources-and-digest' denied request: images must be pinned by digest: web`.

- `validationActions` combinations: `[Deny]`, `[Warn]`, `[Audit]`, `[Deny, Audit]`, `[Warn, Audit]`. `[Deny, Warn]` is rejected (`must not contain both Deny and Warn`).
- Roll out new policies with `[Warn, Audit]` first, then switch to `Deny`.
- `matchConditions` filter before validation; they are the right place for user/namespace exemptions. Do not exempt `system:` users blindly if controllers create the objects you want to police (Deployments → ReplicaSets → Pods are created by `system:serviceaccount:kube-system:*`).
- To police Pods regardless of the owning workload, match `pods` (CREATE) instead of every workload kind — but expect Pod-level rejections to surface only as ReplicaSet events.
- `status.typeChecking` is filled by kube-controller-manager; read it after creating a policy to catch type errors that `failurePolicy: Ignore` would otherwise hide.
- There is no short name like `vap`; use `validatingadmissionpolicies` in kubectl and RBAC.

##### Admission Webhooks (Manifests)

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: vwidget.example.com
  annotations:
    cert-manager.io/inject-ca-from: widget-system/widget-webhook-cert
webhooks:
  - name: vwidget.example.com
    admissionReviewVersions: ["v1"]
    sideEffects: None
    failurePolicy: Fail
    matchPolicy: Equivalent
    timeoutSeconds: 5
    clientConfig:
      service:
        name: widget-webhook
        namespace: widget-system
        path: /validate-example-com-v1-widget
        port: 443
    rules:
      - apiGroups: ["example.com"]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["widgets"]
        scope: Namespaced
    namespaceSelector:
      matchExpressions:
        - {key: kubernetes.io/metadata.name, operator: NotIn, values: [kube-system, widget-system]}
    matchConditions:
      - name: skip-node-users
        expression: "!request.userInfo.username.startsWith('system:node:')"
```

- `failurePolicy: Fail` for security-relevant validation; `Ignore` only for best-effort webhooks. Never `Fail` with a selector that matches the webhook's own namespace (the webhook cannot restart).
- `sideEffects: None` (or `NoneOnDryRun`) is required for `--dry-run=server` to call the webhook.
- `timeoutSeconds` defaults to 10 (max 30); keep it short.
- `matchPolicy: Equivalent` (the default) also sends requests made through other versions of the same resource.
- Use `matchConditions` (GA 1.30) for cheap CEL pre-filtering instead of filtering inside the webhook.
- TLS: rotate certificates (cert-manager or the platform); never `insecureSkipTLSVerify`.
- Consider replacing simple webhooks with ValidatingAdmissionPolicy or MutatingAdmissionPolicy now that both are GA on 1.36.

##### Static (Manifest-Based) Admission — Alpha, Do Not Rely On

`ManifestBasedAdmissionControlConfig` (alpha, off in 1.36; beta/on in 1.37) loads policies and webhooks from `staticManifestsDir` in the `AdmissionConfiguration` file so they survive deletion through the API. On 1.36 protect critical policies with RBAC instead (restrict `delete` on `validatingadmissionpolicies`, `mutatingadmissionpolicies`, `*webhookconfigurations`).

---

#### CRD & API Design

##### Minimal Skeleton (Go, Kubebuilder markers)

```go
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	GroupVersion  = schema.GroupVersion{Group: "example.com", Version: "v1"}
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion, &Widget{}, &WidgetList{})
	metav1.AddToGroupVersion(s, GroupVersion)
	return nil
}

// Tier selects a size class.
// +kubebuilder:validation:Enum=small;medium;large
type Tier string

const (
	TierSmall  Tier = "small"
	TierMedium Tier = "medium"
	TierLarge  Tier = "large"
)

// WidgetSpec is the desired state.
// +kubebuilder:validation:XValidation:rule="self.maxReplicas >= self.replicas",message="maxReplicas must be >= replicas"
type WidgetSpec struct {
	// +kubebuilder:validation:Minimum=0
	// +required
	Replicas int32 `json:"replicas"`
	// +kubebuilder:default=10
	// +optional
	MaxReplicas int32 `json:"maxReplicas,omitempty"`
	// +required
	Tier Tier `json:"tier"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="storageClass is immutable"
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

// WidgetStatus is the observed state.
type WidgetStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=wdg
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Widget is the Schema for the widgets API.
type Widget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WidgetSpec   `json:"spec"`
	Status            WidgetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WidgetList contains a list of Widget.
type WidgetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Widget `json:"items"`
}
```

**[NEW in controller-runtime v0.24]** `sigs.k8s.io/controller-runtime/pkg/scheme.Builder` is deprecated: API packages should depend only on apimachinery, so use `runtime.NewSchemeBuilder` as above. Existing Kubebuilder scaffolds (`groupversion_info.go`) still use the old helper; it compiles but `staticcheck` reports SA1019.

##### Spec Design Rules

- `// +optional` with `omitempty` for optional fields; `// +required` for required ones. Use pointers (`*int32`) when "unset" and "zero" must be distinguishable.
- Prefer **lists of named subobjects** with `+listType=map` / `+listMapKey=name` over maps; they merge correctly under server-side apply and strategic merge patch.
- **Never use floating-point** in `spec`; use integers, `resource.Quantity`, or strings with units.
- **Do not use `bool` where an enum will be needed** — start with a string type alias.
- **No raw maps** except for labels, annotations and opaque user data.
- Only `metadata`, `spec`, `status` (and `apiVersion`/`kind`) at the top level.
- Every string, list and map needs `maxLength` / `maxItems` / `maxProperties` — CEL cost estimation depends on them, and unbounded fields invite abuse.

##### Generated CRD Schema Checklist (YAML)

The CRD below was installed on a 1.36 apiserver; the custom resources further down were validated against it.

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  scope: Namespaced
  names: {plural: widgets, singular: widget, kind: Widget, shortNames: [wdg]}
  versions:
    - name: v1
      served: true
      storage: true
      subresources:
        status: {}
      additionalPrinterColumns:
        - {name: Ready, type: string, jsonPath: '.status.conditions[?(@.type=="Ready")].status'}
        - {name: Age, type: date, jsonPath: .metadata.creationTimestamp}
      selectableFields:                       # GA 1.32: kubectl get --field-selector spec.tier=small
        - jsonPath: .spec.tier
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required: [replicas, tier]
              x-kubernetes-validations:
                - rule: "self.maxReplicas >= self.replicas"
                  messageExpression: "'maxReplicas must be greater than or equal to replicas'"
                  fieldPath: .maxReplicas
                  reason: FieldValueInvalid
                - rule: "!has(oldSelf.storageClass) || has(self.storageClass)"
                  message: storageClass cannot be unset once set
              properties:
                replicas:
                  type: integer
                  format: int32
                  minimum: 0
                maxReplicas:
                  type: integer
                  format: int32
                  default: 10
                tier:
                  type: string
                  enum: [small, medium, large]
                storageClass:
                  type: string
                  x-kubernetes-validations:
                    - rule: "self == oldSelf"
                      message: storageClass is immutable
                memory:
                  x-kubernetes-int-or-string: true
                  maxLength: 32
                  x-kubernetes-validations:
                    - rule: "isQuantity(string(self)) && quantity(string(self)).isLessThan(quantity('64Gi'))"
                      message: memory must be a quantity below 64Gi
                endpoint:
                  type: string
                  maxLength: 2048
                  x-kubernetes-validations:
                    - rule: "isURL(self) && url(self).getScheme() == 'https'"
                      message: endpoint must be an https URL
                cidr:
                  type: string
                  maxLength: 64
                  x-kubernetes-validations:
                    - rule: "isCIDR(self) && cidr(self).ip().family() == 4"
                      message: cidr must be an IPv4 CIDR
                hostname:
                  type: string
                  maxLength: 253
                  x-kubernetes-validations:
                    - rule: "!format.dns1123Subdomain().validate(self).hasValue()"
                      message: hostname must be a DNS-1123 subdomain
                version:
                  type: string
                  maxLength: 64
                  x-kubernetes-validations:
                    - rule: "isSemver(self, true) && semver(self, true).isGreaterThan(semver('1.0.0'))"
                      message: version must be a semver > 1.0.0
                ports:
                  type: array
                  maxItems: 16
                  x-kubernetes-list-type: map
                  x-kubernetes-list-map-keys: [name]
                  x-kubernetes-validations:
                    - rule: "self.all(i, p, self.all(j, q, i == j || p.port != q.port))"
                      message: ports must be unique
                  items:
                    type: object
                    required: [name, port]
                    properties:
                      name: {type: string, maxLength: 15}
                      port: {type: integer, format: int32, minimum: 1, maximum: 65535}
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
                      reason: {type: string, maxLength: 1024, pattern: '^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$'}
                      message: {type: string, maxLength: 32768}
```

A custom resource that violates the rules is rejected with all messages at once (verified on 1.36):

```yaml
apiVersion: example.com/v1
kind: Widget
metadata: {name: w-bad, namespace: shop}
spec:
  replicas: 1
  tier: small
  memory: 100Gi          # memory must be a quantity below 64Gi
  endpoint: http://api.example.com           # endpoint must be an https URL
  cidr: 2001:db8::/32    # cidr must be an IPv4 CIDR
  hostname: Bad_Host     # hostname must be a DNS-1123 subdomain
  version: 0.9.0         # version must be a semver > 1.0.0
  ports: [{name: a, port: 80}, {name: b, port: 80}]   # ports must be unique
```

##### Numeric Formats Are Enforced **[NEW in 1.36]**

```yaml
# Rejected on 1.36, stored on 1.35:
apiVersion: example.com/v1
kind: Widget
metadata: {name: w-int32, namespace: shop}
spec:
  replicas: 3000000000      # schema: type integer, format int32
  maxReplicas: 3000000001
  tier: small
```

1.36 returns `Checked value must be of type integer with format int32 in spec.replicas` (the error path is reported oddly as `<nil>`). Range checks apply to `int32`, `int64`, `float` and `double`. Objects stored before the upgrade keep validating thanks to ratcheting as long as the out-of-range field is unchanged. Review consequences: a Go field typed `int32` whose CRD schema lacked `format: int32` (hand-written CRDs) could previously hold values that overflow when the controller decodes them; on 1.36 make schema format and Go type agree, and add explicit `minimum`/`maximum` anyway for a readable error.

##### PrintColumns

Always define at least a state column (from a condition) and `Age` from `.metadata.creationTimestamp`. Use `priority: 1` for wide-only columns.

##### Field Selectors on CRDs

`selectableFields` (GA 1.32) lets clients and informers filter server-side: `kubectl get widgets --field-selector spec.tier=small`, or `cache.Options.ByObject[...].Field` in controller-runtime. Up to 8 fields; only string, integer and boolean leaf fields.

---

#### CEL in CRDs and Policies

##### Library Availability on 1.36

Everything in this table is available on a 1.36 apiserver (introduction version in parentheses): standard macros and functions; Kubernetes list, regex, URL and authorizer libraries; extended strings library v2 (1.30); quantity library (1.29); CEL optional types and cross-type numeric comparisons (1.29); IP and CIDR libraries (1.31); format library — `format.dns1123Label()`, `format.dns1123Subdomain()`, `format.named(...)` (1.32); two-variable comprehensions `all(i, v, ...)`, `exists(i, v, ...)`, `transformList`, `transformMap` (1.33); semver library — `isSemver`, `semver(...)` (1.34). All of `isQuantity`, `quantity().isLessThan`, `isURL`, `url().getScheme`, `isCIDR`, `cidr().ip().family`, `format.dns1123Subdomain().validate`, `isSemver(s, true)`/`semver(s, true)` and two-variable `all` were exercised in the CRD above on 1.36.

The `semver(s, true)` / `isSemver(s, true)` "normalize" overload accepts `v` prefixes and missing minor/patch parts. Without `true`, `"1.4"` is not a valid semver.

##### Transition Rules and Immutability

Semantics are documented in the CRD validation-rules reference.

```yaml
storageClass:
  type: string
  x-kubernetes-validations:
    - rule: "self == oldSelf"               # transition rule: only evaluated on UPDATE when the field existed
      message: storageClass is immutable
```

- A transition rule (`oldSelf`) is skipped on CREATE and when the old object lacked the field. `self == oldSelf` alone does **not** prevent *removing* the field; add a parent-level rule such as `!has(oldSelf.storageClass) || has(self.storageClass)` (verified: removal is then rejected with `storageClass cannot be unset once set`).
- `optionalOldSelf: true` evaluates the rule even when the old value is absent; `oldSelf` is then an optional. Pattern for "set once":

```yaml
region:
  type: string
  maxLength: 32
  x-kubernetes-validations:
    - rule: "!oldSelf.hasValue() || oldSelf.value() == self"
      optionalOldSelf: true
      message: region is immutable once set
```

Verified on 1.36: setting `region` on an existing object without it succeeds; changing it afterwards fails with `region is immutable once set`.

##### Cost Budget Pitfalls

- Unbounded strings/lists/maps make the **estimated** cost explode and the CRD is rejected at creation with `estimated rule cost exceeds budget by factor of more than 100x`. Add `maxLength`, `maxItems`, `maxProperties`.
- **`string(<int>)` in a `messageExpression` is costed as unbounded.** Verified on 1.35 and 1.36: `messageExpression: "'maxReplicas (' + string(self.maxReplicas) + ') must be >= replicas (' + string(self.replicas) + ')'"` is rejected for exceeding the budget even with `minimum`/`maximum` on the integers, while the same rule with a constant message, or a message that concatenates bounded *string* fields (`'mode ' + self.mode`, `maxLength: 32`), is accepted. Use a constant `message` (plus `fieldPath`) for numeric comparisons.
- `x-kubernetes-int-or-string` fields need `maxLength` before `string(self)` can be used cheaply.
- Rules on items of large arrays are multiplied by `maxItems`; push rules to the narrowest level.

##### `messageExpression`, `reason`, `fieldPath`

- `reason` is one of `FieldValueInvalid` (default), `FieldValueForbidden`, `FieldValueRequired`, `FieldValueDuplicate`; it sets the HTTP 422 cause type.
- `fieldPath` points the error at a sub-field (`.maxReplicas`), so `kubectl` shows `spec.maxReplicas: Invalid value: ...` instead of `spec: Invalid value`.
- `messageExpression` must return a string; if it errors, `message` is used.

##### Validation Ratcheting (GA 1.33)

Unchanged fields of existing objects are not re-validated against tightened schema rules on update. This makes adding new constraints to a CRD safe for existing objects, but **do not rely on ratcheting to justify tightening validation in a served version** — new objects and changed fields still fail, which breaks clients.

---

#### Spec vs. Status

```text
┌──────────────────────────────────────────────────────────────┐
│                       etcd                                   │
│  spec   ← users / GitOps / other controllers (desired state) │
│  status ← this controller via the /status subresource        │
└──────────────────────────────────────────────────────────────┘
```

| | `spec` | `status` |
|---|---|---|
| **Written by** | Users / agents / CI | Controllers only |
| **Represents** | Desired state | Observed / current state |
| **Subresource** | main resource | `/status` — always enable `// +kubebuilder:subresource:status` |
| **Writes to the main resource** | Update spec/metadata | `status` changes are ignored |

##### Updating Status (controller-runtime)

```go
// Correct: write status through the subresource
if err := r.Status().Update(ctx, obj); err != nil {
	return ctrl.Result{}, err
}
// Wrong: r.Update(ctx, obj) silently drops status changes when the status subresource is enabled
```

Server-side apply on status (`r.Status().Apply(ctx, applyConfig, ...)`) is available since controller-runtime v0.23 and avoids conflicts when several controllers own different conditions.

##### observedGeneration

Set `status.observedGeneration = metadata.generation` after a successful reconcile, and set `ObservedGeneration` on every condition you write. Tools (`kubectl wait`, Argo CD, Flux) use it to tell whether the status describes the latest spec. Pods carry `status.observedGeneration` natively since 1.35 (GA).

---

#### Status Conditions

```go
// In your Status struct:
// +listType=map
// +listMapKey=type
// +optional
Conditions []metav1.Condition `json:"conditions,omitempty"`
```

| Field | Rule |
|---|---|
| `Type` | PascalCase: `Ready`, `Degraded`, `Progressing`, `Available` |
| `Status` | `True`, `False`, or `Unknown` — never empty |
| `Reason` | CamelCase, machine-readable (`ReconcileError`); must match `^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$` |
| `Message` | Human-readable; may contain specifics; never secrets |
| `ObservedGeneration` | `metadata.generation` at the time of observation |
| `LastTransitionTime` | Changes only when `Status` changes — `meta.SetStatusCondition` handles this |

Standard condition types (abnormal-true vs. normal-true): `Ready`/`Available` (True when healthy), `Progressing`/`Reconciling` (True while working), `Degraded`/`Stalled` (True when something is wrong). Pick one polarity per type and document it.

Do not add new `phase` string fields; conditions are the API convention. Keep an existing `phase` for compatibility but derive it from conditions.

```go
changed := meta.SetStatusCondition(&w.Status.Conditions, metav1.Condition{
	Type:               "Ready",
	Status:             metav1.ConditionTrue,
	Reason:             "Reconciled",
	Message:            "Deployment is up to date",
	ObservedGeneration: w.Generation,
})
```

`meta.SetStatusCondition` returns whether anything changed; use it to skip no-op status writes and duplicate Events.

---

#### Versioning & API Evolution

##### Version Lifecycle

```text
v1alpha1 → v1alpha2 → v1beta1 → v1beta2 → v1
```

| Stage | Stability | Built-in APIs served by default | Compatibility |
|---|---|---|---|
| `v1alphaX` | Experimental | No | none |
| `v1betaX` | Pre-release | **No** for beta APIs introduced since 1.24 (KEP-3136) | best effort, deprecation policy applies |
| `v1` | Stable | Yes | required |

For CRDs, "enabled" is the `served` flag the CRD author sets per version.

##### Rules

1. Never remove or rename fields in a served version.
2. Never tighten validation on an existing field in a way that rejects previously valid values.
3. Add new fields as optional.
4. Exactly one version has `storage: true`; others are converted.
5. Conversion must be lossless in both directions (round-trip unknown fields through annotations if needed).
6. For breaking changes: add a new version, mark the old one `deprecated: true` with a `deprecationWarning`, and keep serving it long enough for clients to migrate.
7. Use a conversion webhook when schemas differ between versions.
8. After changing the storage version, rewrite stored objects and then remove the old version from `status.storedVersions`. **[NEW in 1.36]** When the (beta, off by default) StorageVersionMigrator migrates a CRD, the apiserver removes the migrated versions from `status.storedVersions` itself. On a stock 1.36 cluster use the out-of-tree kube-storage-version-migrator or a read/write loop; `storagemigration.k8s.io/v1` is 1.37.

```yaml
versions:
  - name: v1beta1
    served: true
    storage: false
    deprecated: true
    deprecationWarning: "example.com/v1beta1 Widget is deprecated; use example.com/v1"
  - name: v1
    served: true
    storage: true
```

---

#### Controller / Reconciler Patterns (controller-runtime v0.24)

##### Reconciler Skeleton

Compiles against controller-runtime v0.24.1 and k8s.io/* v0.36.5; the unit test in the Testing section runs it three times against the fake client.

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
	"k8s.io/apimachinery/pkg/runtime"
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

	examplev1 "example.com/widget-operator/api/v1"
)

const (
	widgetFinalizer = "example.com/external-cleanup"
	fieldOwner      = client.FieldOwner("widget-controller")
)

// WidgetReconciler reconciles Widget objects.
type WidgetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	External ExternalAPI
}

// ExternalAPI is an out-of-cluster dependency that needs cleanup.
type ExternalAPI interface {
	Delete(ctx context.Context, id string) error
}

// +kubebuilder:rbac:groups=example.com,resources=widgets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=example.com,resources=widgets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=example.com,resources=widgets/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

func (r *WidgetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch; a missing object is not an error.
	w := &examplev1.Widget{}
	if err := r.Get(ctx, req.NamespacedName, w); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Deletion: clean up external state, then release the finalizer.
	if !w.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(w, widgetFinalizer) {
			return ctrl.Result{}, nil
		}
		if err := r.External.Delete(ctx, string(w.UID)); err != nil {
			return ctrl.Result{}, fmt.Errorf("external cleanup: %w", err)
		}
		controllerutil.RemoveFinalizer(w, widgetFinalizer)
		return ctrl.Result{}, r.Update(ctx, w)
	}

	// 3. Ensure the finalizer before creating anything external.
	if controllerutil.AddFinalizer(w, widgetFinalizer) {
		// Continue in the same pass: with GenerationChangedPredicate on For(),
		// a metadata-only update does NOT trigger another reconcile.
		if err := r.Update(ctx, w); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 4. Server-side apply the owned Deployment: declare only the fields we own.
	dep := appsv1ac.Deployment(w.Name, w.Namespace).
		WithLabels(map[string]string{"app.kubernetes.io/managed-by": "widget-controller"}).
		WithOwnerReferences(metav1ac.OwnerReference().
			WithAPIVersion(examplev1.GroupVersion.String()).
			WithKind("Widget").
			WithName(w.Name).
			WithUID(w.UID).
			WithController(true).
			WithBlockOwnerDeletion(true)).
		WithSpec(appsv1ac.DeploymentSpec().
			WithReplicas(w.Spec.Replicas).
			WithSelector(metav1ac.LabelSelector().WithMatchLabels(map[string]string{"widget": w.Name})).
			WithTemplate(corev1ac.PodTemplateSpec().
				WithLabels(map[string]string{"widget": w.Name}).
				WithSpec(corev1ac.PodSpec().
					WithContainers(corev1ac.Container().
						WithName("widget").
						WithImage("registry.example.com/widget:1.0")))))
	if err := r.Apply(ctx, dep, fieldOwner, client.ForceOwnership); err != nil {
		return r.fail(ctx, w, "ApplyFailed", err)
	}

	// 5. Status: only write when something changed.
	changed := meta.SetStatusCondition(&w.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "Deployment is up to date",
		ObservedGeneration: w.Generation,
	})
	if changed || w.Status.ObservedGeneration != w.Generation {
		w.Status.ObservedGeneration = w.Generation
		if err := r.Status().Update(ctx, w); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			return ctrl.Result{}, err
		}
		if changed {
			r.Recorder.Eventf(w, nil, corev1.EventTypeNormal, "Reconciled", "Reconcile", "Deployment %s is up to date", w.Name)
		}
	}
	log.V(1).Info("reconciled", "replicas", w.Spec.Replicas)
	return ctrl.Result{}, nil
}

var errPermanent = errors.New("permanent")

func (r *WidgetReconciler) fail(ctx context.Context, w *examplev1.Widget, reason string, err error) (ctrl.Result, error) {
	meta.SetStatusCondition(&w.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: reason,
		Message: err.Error(), ObservedGeneration: w.Generation,
	})
	if uerr := r.Status().Update(ctx, w); uerr != nil {
		return ctrl.Result{}, errors.Join(err, uerr)
	}
	r.Recorder.Eventf(w, nil, corev1.EventTypeWarning, reason, "Reconcile", "%v", err)
	if errors.Is(err, errPermanent) {
		return ctrl.Result{}, nil // wait for a spec change instead of hot-looping
	}
	return ctrl.Result{}, err
}

// SetupWithManager wires watches.
func (r *WidgetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&examplev1.Widget{}, builder.WithPredicates(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		))).
		Owns(&appsv1.Deployment{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 4}).
		Complete(r)
}
```

Key points a reviewer checks:
- **Server-side apply with typed apply configurations** (`client.Client.Apply`, controller-runtime ≥ v0.22) is the preferred way to own child objects: idempotent, no read-modify-write, no lost updates, and other actors (HPA, users) keep their fields. Always pass a stable `client.FieldOwner`; `client.ForceOwnership` is appropriate for fields the controller exclusively owns. Never apply a fully populated struct converted from a Go object — it claims ownership of every defaulted field. The old `r.Patch(ctx, obj, client.Apply)` form is deprecated.
- Apply only the fields you care about. If an HPA scales the Deployment, omit `WithReplicas`.
- **Run 1.36.3+**: a 1.36.0–1.36.2 regression made some applies of list/map fields fail with `422 required`.
- Status writes go through `r.Status()`; check `meta.SetStatusCondition`'s return value to avoid write storms and duplicate Events.
- Never block in `Reconcile` (no `time.Sleep`, no unbounded external calls without a context deadline).

##### Reconcile Return Conventions

| Return | Meaning |
|---|---|
| `ctrl.Result{}, nil` | Success; wait for the next watch event |
| `ctrl.Result{}, err` | Transient failure; requeued with per-item exponential backoff (5 ms doubling, capped at 1000 s by the default rate limiter) |
| `ctrl.Result{RequeueAfter: d}, nil` | Poll again after `d` (external state you cannot watch) |
| `ctrl.Result{Requeue: true}, nil` | **Deprecated** ("causes confusion"); use `RequeueAfter` |
| `ctrl.Result{Priority: &p}` | v0.23+: priority for the priority queue, which is enabled by default since v0.23 |

Never return both a non-zero `Result` and an error — controller-runtime ignores the `Result` and logs a warning.

##### Manager Setup

```go
func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))
	log := ctrl.Log.WithName("setup")

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		log.Error(err, "add client-go scheme")
		os.Exit(1)
	}
	if err := examplev1.AddToScheme(scheme); err != nil {
		log.Error(err, "add example scheme")
		os.Exit(1)
	}

	managedBy := labels.SelectorFromSet(labels.Set{"app.kubernetes.io/managed-by": "widget-controller"})
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: ":8443", SecureServing: true},
		HealthProbeBindAddress: ":8081",
		LeaderElection:         true,
		LeaderElectionID:       "widget-controller.example.com",
		// auto-detected in-cluster; required when running out-of-cluster (make run)
		LeaderElectionNamespace: "widget-system",
		Cache: cache.Options{
			DefaultTransform: cache.TransformStripManagedFields(),
			ByObject: map[client.Object]cache.ByObject{
				&appsv1.Deployment{}: {Label: managedBy}, // cache only what we own
			},
		},
	})
	if err != nil {
		log.Error(err, "create manager")
		os.Exit(1)
	}

	if err := (&widgetctrl.WidgetReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorder("widget-controller"), // events.k8s.io/v1
		External: newCloudClient(),                          // must not be nil: the deletion path calls it
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "setup controller")
		os.Exit(1)
	}

	_ = mgr.AddHealthzCheck("healthz", healthz.Ping)
	_ = mgr.AddReadyzCheck("informers", func(req *http.Request) error {
		if !mgr.GetCache().WaitForCacheSync(req.Context()) {
			return errors.New("informer caches not synced")
		}
		return nil
	})

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "manager exited")
		os.Exit(1)
	}
}
```

- **Events**: `mgr.GetEventRecorder(name)` returns a `k8s.io/client-go/tools/events.EventRecorder` that writes `events.k8s.io/v1` Events with the signature `Eventf(regarding, related runtime.Object, eventtype, reason, action, note string, args ...)`. RBAC must grant `create;patch` on `events` in the **`events.k8s.io`** group (the core-group rule is not enough). `GetEventRecorderFor` (old core/v1 API, `record.EventRecorder`) is deprecated since v0.23.
- Cache only what you need: label/field selectors per type (`cache.ByObject`), `DefaultTransform: cache.TransformStripManagedFields()` to cut memory, `DefaultNamespaces` for namespace-scoped operators.
- Serve metrics over HTTPS with authn/authz (`SecureServing: true` plus `filters.WithAuthenticationAndAuthorization`), not plain `:8080`.
- Enable leader election for every Deployment with more than one replica.

##### controller-runtime Changes Between v0.22 and v0.24 (migration checklist)

| Version | Change | Review impact |
|---|---|---|
| v0.23 | Webhooks are generic: `ctrl.NewWebhookManagedBy(mgr, &v1.Widget{})` and `admission.Validator[T]` / `admission.Defaulter[T]` receive the concrete type | `NewWebhookManagedBy(mgr).For(obj)` no longer compiles; `CustomValidator` is deprecated |
| v0.23 | `GetEventRecorder` (events.k8s.io); `GetEventRecorderFor` deprecated | RBAC moves to `events.k8s.io` |
| v0.23 | Subresource apply (`r.Status().Apply`), `client.Options{FieldOwner, FieldValidation}` | prefer SSA for status owned by several controllers |
| v0.23 | Priority queue enabled by default; `Result.Priority` | reconcile order differs from FIFO |
| v0.23 | Fake client sets `resourceVersion` on SSA create | tests asserting empty RV break |
| v0.24 | k8s.io/* v0.36, Go 1.26 | bump the toolchain |
| v0.24 | `pkg/scheme.Builder` deprecated; custom webhook path functions deprecated | use `runtime.NewSchemeBuilder` |
| v0.24 | `cache.IndexField` no longer blocks until the informer synced; typed apply configurations are updated from the server response | do not rely on `IndexField` as a sync barrier |

---

#### Watches, Predicates, Caches and Informers (client-go v0.36)

##### Predicates

```go
ctrl.NewControllerManagedBy(mgr).
	For(&examplev1.Widget{}, builder.WithPredicates(predicate.Or(
		predicate.GenerationChangedPredicate{},  // spec changes
		predicate.AnnotationChangedPredicate{},  // e.g. "reconcile-now" annotations
	))).
	Owns(&appsv1.Deployment{}).                  // no predicate: react to child status changes
	Complete(r)
```

- `GenerationChangedPredicate` ignores status-only and metadata-only updates of the primary — correct for `For()`, **wrong** for `Owns()` when you aggregate child status (a Deployment's `status.readyReplicas` change does not bump its generation).
- Label, annotation and finalizer changes do not bump `metadata.generation`, so `GenerationChangedPredicate` filters them out — including the controller's own finalizer `Update`. Never `return` after adding a finalizer and wait for "the next event"; continue reconciling in the same pass (verified: with `return`, the Widget above kept its finalizer and never got a Deployment). Setting `deletionTimestamp` on an object with finalizers **does** bump the generation (verified on 1.36: 1 → 2), so deletion passes the predicate.

##### Fan-Out Watches

```go
Watches(
	&corev1.Secret{},
	handler.EnqueueRequestsFromMapFunc(r.findWidgetsForSecret),
	builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
)
```

Back the map function with a field index (`mgr.GetFieldIndexer().IndexField(...)`) instead of listing everything. Since v0.24 `IndexField` returns without waiting for the informer to sync.

##### Informer Behaviour Changes in client-go v0.36 **[NEW in 1.36]**

- **`AtomicFIFO` (beta, on)**: on a list or relist the store is replaced with the complete set first, then `OnAdd`/`OnUpdate`/`OnDelete` handlers run for individual items. A handler that inspects the store sees a consistent snapshot at one resourceVersion. Handlers that assumed they ran before the store reflected the whole relist may behave differently. Can be disabled temporarily via the client-go feature gate (`KUBE_FEATURE_AtomicFIFO=false`).
- **`UnlockWhileProcessingFIFO` (beta, on)**: new watch events can be queued while handlers process earlier ones — fewer dropped watches under load, more handler concurrency pressure on slow handlers.
- **`InOrderInformers` GA** via `RealFIFO`; `DeltaFIFO` will be phased out internally.
- **`cache.Store` interface grew two methods**: `LastStoreSyncResourceVersion() string` and `Bookmark(rv string)`. Any hand-written `cache.Store` implementation stops compiling:

```go
// Compiles with client-go v0.35, fails with v0.36:
// "legacyStore does not implement cache.Store (missing method Bookmark)"
type legacyStore struct{}

func (legacyStore) Add(obj interface{}) error                      { return nil }
func (legacyStore) Update(obj interface{}) error                   { return nil }
func (legacyStore) Delete(obj interface{}) error                   { return nil }
func (legacyStore) List() []interface{}                            { return nil }
func (legacyStore) ListKeys() []string                             { return nil }
func (legacyStore) Get(obj interface{}) (interface{}, bool, error) { return nil, false, nil }
func (legacyStore) GetByKey(key string) (interface{}, bool, error) { return nil, false, nil }
func (legacyStore) Replace(list []interface{}, rv string) error    { return nil }
func (legacyStore) Resync() error                                  { return nil }

var _ cache.Store = legacyStore{}
```

  Embed a real store (`cache.NewStore(cache.MetaNamespaceKeyFunc)`) and override only what you need instead of re-implementing the interface. `cache.FakeCustomStore` was fixed in v0.36.4.
- **Fake clientset**: `fake.NewClientset()` (field-tracking, supports SSA) is the recommended constructor; `NewSimpleClientset` has no field management. The client-go fake now supports separate List+Watch with `ResourceVersion` checks, so List metadata carries a `resourceVersion` and watches may deliver objects they previously missed — tests that asserted empty RVs or missing events need updating.
- **Transport**: CA files are reloaded on change and unused TLS transports are garbage-collected (`ClientsAllowTLSCacheGC`, disable with `KUBE_FEATURE_ClientsAllowTLSCacheGC=false`). Long-running binaries that create many `rest.Config`s no longer leak rotation goroutines.

##### Stale Caches: What 1.36 Teaches Controller Authors

1.36's kube-controller-manager defers DaemonSet, Job, ReplicaSet and StatefulSet syncs until the informer has observed the controller's own last write (`StaleControllerConsistency*` gates, beta, on; metrics such as `job_controller_stale_sync_skips_total`). Apply the same discipline in custom controllers:

- After creating a child, do not create it again just because the cache does not show it yet. Use deterministic names and treat `AlreadyExists` as success, or use server-side apply (idempotent by construction).
- For "create N children" logic, use expectations (record what you created; skip reconciliation until the cache shows it) or count via the API reader (`mgr.GetAPIReader()`) for the critical decision.
- Do not delete "orphans" based on a cache that may be behind your own creates.

---

#### Finalizers

##### When to Use

Use finalizers **only** for cleanup that garbage collection cannot do: cloud resources, DNS records, external databases, cross-namespace or cluster-scoped children of namespaced owners. Do not add finalizers for in-cluster children that `ownerReferences` already cover.

##### Naming Rule

Finalizer names must be qualified names with a domain prefix: `example.com/external-cleanup`, not `cleanup`. The apiserver warns on unqualified finalizers on custom resources.

##### Lifecycle

```go
if !w.DeletionTimestamp.IsZero() {
	if !controllerutil.ContainsFinalizer(w, widgetFinalizer) {
		return ctrl.Result{}, nil
	}
	if err := r.External.Delete(ctx, string(w.UID)); err != nil {
		return ctrl.Result{}, fmt.Errorf("external cleanup: %w", err)
	}
	controllerutil.RemoveFinalizer(w, widgetFinalizer)
	return ctrl.Result{}, r.Update(ctx, w)
}
if controllerutil.AddFinalizer(w, widgetFinalizer) {
	if err := r.Update(ctx, w); err != nil { // Update refreshes w (new resourceVersion)
		return ctrl.Result{}, err
	}
	// keep going — do not return and wait for an event that a predicate may filter out
}
```

- Cleanup must be idempotent (the deleted object can be reconciled many times).
- Add the finalizer **before** creating external resources; remove it only after cleanup succeeded.
- New finalizers cannot be added once `deletionTimestamp` is set (the apiserver rejects it).
- Failing cleanup must surface: set a condition, emit a Warning Event, return the error for backoff.
- Do not create objects in a namespace that is terminating; with ordered namespace deletion (GA 1.34) Pods are deleted before other resources, so a controller that recreates Pods during namespace deletion fights the deletion.
- `controllerutil.AddFinalizer`/`RemoveFinalizer` return whether they changed the object — use the result to skip unnecessary updates.

---

#### Owner References & Garbage Collection

```go
// SetControllerReference sets controller: true and blockOwnerDeletion: true
if err := controllerutil.SetControllerReference(owner, child, r.Scheme); err != nil {
	return err
}
```

| Rule | Notes |
|---|---|
| Owner and dependent in the same namespace | A namespaced dependent pointing at a missing same-namespace owner is garbage-collected |
| A namespaced owner cannot own a cluster-scoped object | The reference is unresolvable; the dependent is **not** collected and gets an `OwnerRefInvalidNamespace` event — use a finalizer instead |
| Cluster-scoped owners may own cluster-scoped or namespaced objects | Permitted |
| One controller owner per object | `SetControllerReference` fails if another controller already owns it (`AlreadyOwnedError`) |
| Owner reference `apiVersion` must be `group/version` (`v1` for core) | A wrong `apiVersion` (for example `core/v1`) breaks GC — 1.36 fixed exactly this bug in PodCertificateRequest owner refs |
| `blockOwnerDeletion: true` needs `update` on `<owner>/finalizers` | Hence the `widgets/finalizers` RBAC rule |

---

#### Validation & Admission Webhooks in Go

controller-runtime v0.23+ webhooks are typed:

```go
// WidgetValidator validates Widgets; v0.23+ webhooks receive the concrete type.
type WidgetValidator struct{}

var _ admission.Validator[*examplev1.Widget] = &WidgetValidator{}

func (v *WidgetValidator) ValidateCreate(ctx context.Context, w *examplev1.Widget) (admission.Warnings, error) {
	if w.Spec.Tier == examplev1.TierLarge && w.Spec.Replicas < 3 {
		return nil, fmt.Errorf("tier large requires at least 3 replicas")
	}
	return nil, nil
}

func (v *WidgetValidator) ValidateUpdate(ctx context.Context, oldW, newW *examplev1.Widget) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, newW)
}

func (v *WidgetValidator) ValidateDelete(ctx context.Context, w *examplev1.Widget) (admission.Warnings, error) {
	return nil, nil
}

// SetupWidgetWebhook registers the validating webhook.
func SetupWidgetWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &examplev1.Widget{}).
		WithValidator(&WidgetValidator{}).
		Complete()
}
```

- The rule above is expressible in CEL (`self.tier != 'large' || self.replicas >= 3`) — prefer the CRD rule and drop the webhook.
- Webhook logic must be deterministic and fast; no writes to the cluster from a validating webhook (`sideEffects: None`).
- Defaulting webhooks must be idempotent; prefer `+kubebuilder:default` markers or a MutatingAdmissionPolicy for static defaults.
- Handle CREATE **and** UPDATE for immutability rules; use DELETE only for deletion guards.

---

#### RBAC & Least Privilege

1. Grant only the verbs and resources used; no `*` verbs or resources.
2. Prefer `Role`/`RoleBinding` over cluster-wide bindings when the operator is namespace-scoped.
3. No `cluster-admin` service accounts; no `escalate`, `bind`, or `impersonate` unless the operator's job is RBAC management.
4. Status and finalizers are separate subresources: `widgets/status` (`get;update;patch`), `widgets/finalizers` (`update`).
5. Events: `events.k8s.io` group for the controller-runtime v0.23+ recorder.
6. `automountServiceAccountToken: false` on the ServiceAccount; opt in on the Pod that needs API access.
7. **[NEW in 1.36]** With `ConstrainedImpersonation` (beta, on), impersonation can be scoped: a tool needs permission to impersonate an identity *and* permission for the specific action on its behalf. Review any `impersonate` grant for scope.
8. **[NEW in 1.36]** DRA drivers and schedulers need the new granular subresources (`resourceclaims/driver` with `associated-node:update`/`arbitrary-node:update` verbs, `resourceclaims/binding`) when `DRAResourceClaimGranularStatusAuthorization` is enabled (beta, on).
9. Built-in `admin`/`edit`/`view` ClusterRoles gained permissions for workloads (`scheduling.k8s.io` Workload) in 1.36; aggregated custom roles should not duplicate them.

```go
// +kubebuilder:rbac:groups=example.com,resources=widgets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=example.com,resources=widgets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=example.com,resources=widgets/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
```

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: widget-operator
  namespace: widget-system
automountServiceAccountToken: false
---
apiVersion: v1
kind: Pod
metadata:
  name: widget-operator
  namespace: widget-system
spec:
  serviceAccountName: widget-operator
  automountServiceAccountToken: true
  containers:
    - name: manager
      image: registry.example.com/widget-operator@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

---

#### Security Hardening

##### Image Security

- Pin images by digest in production manifests; never `:latest` (which also defaults `imagePullPolicy` to `Always`).
- Distroless or scratch base images for operators.
- Scan in CI (Trivy, Grype).
- With `KubeletEnsureSecretPulledImages` (beta, on since 1.35) the kubelet re-checks pull credentials for images already on the node; do not rely on "the image is cached" to bypass registry auth.

##### Secrets

- Never embed secrets in CRD `spec`, annotations, labels, env literals or ConfigMaps; reference Secrets by name.
- Do not log secret values or whole objects that contain them.
- Prefer projected ServiceAccount tokens (bounded audience and lifetime) over long-lived token Secrets.
- `kubectl diff` masks Secret data unless `--show-secrets` (kubectl 1.36+) is given — never paste `--show-secrets` output into PRs or CI logs.

##### Network Policies for Operators

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: widget-operator-egress
  namespace: widget-system
spec:
  podSelector:
    matchLabels:
      control-plane: controller-manager
  policyTypes: [Egress]
  egress:
    - ports:
        - {port: 443, protocol: TCP}
        - {port: 6443, protocol: TCP}
```

This restricts egress to ports 443/6443 on any destination; pin `to:` to the API server endpoint IPs (`kubectl get endpointslices -n default -l kubernetes.io/service-name=kubernetes`) for a tighter policy, and add DNS egress if the operator resolves names.

---

#### Observability

##### Structured Logging

```go
log := logf.FromContext(ctx).WithValues("generation", obj.Generation)
log.Info("starting reconcile")
log.Error(err, "failed to apply Deployment", "deployment", name)

// Wrong: log.Info(fmt.Sprintf("reconciling %s/%s", req.Namespace, req.Name))
```

controller-runtime already adds the controller name, kind and `namespace`/`name` of the request to the context logger. Levels: `Info` (V(0)) for lifecycle events, `V(1)` for per-reconcile summaries, `V(2)`+ for detailed decisions.

##### Metrics

```go
var reconcileErrors = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "widget_operator_reconcile_errors_total",
		Help: "Total number of reconcile errors.",
	},
	[]string{"reason"},
)

func init() {
	metrics.Registry.MustRegister(reconcileErrors)
}
```

- controller-runtime exports `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds`, workqueue metrics and (v0.23+) `controller_runtime_reconcile_timeouts_total`. Do not duplicate them.
- Never use object names or UIDs as label values on high-volume metrics (cardinality).
- **[NEW in 1.36]** Dashboards: `volume_operation_total_errors` → `volume_operation_errors_total`, `etcd_bookmark_counts` → `etcd_bookmark_total`; workqueue, `rest_client_*`, `watch_list_duration_seconds` and several scheduler/EndpointSlice metrics were promoted to Beta stability (stable names and labels). New alpha client-go informer metrics: `informer_queued_items`, `informer_processing_latency_seconds`.

##### Health Probes

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

`mgr.GetCache().WaitForCacheSync` is not a `healthz.Checker`; wrap it as shown in Manager Setup.

##### Events

- Emit Events for user-visible transitions (`Normal`) and failures that need attention (`Warning`), not on every reconcile.
- Use the events.k8s.io recorder (`GetEventRecorder`): `Eventf(regarding, related, type, reason, action, note, args...)`; `reason` and `action` are CamelCase.
- One-shot Jobs/CronJobs may rely on logs and exit status when there is no owned object status to update; missing optional resources in a cleanup pass do not need Events unless users must act.

---

#### Error Handling & Requeue Strategy

```go
// Transient: return the error — the workqueue applies exponential backoff.
return ctrl.Result{}, fmt.Errorf("get external API: %w", err)

// Waiting on something you cannot watch: poll.
return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

// Permanent (invalid user input): record it, then stop retrying until the spec changes.
meta.SetStatusCondition(&w.Status.Conditions, metav1.Condition{
	Type: "Ready", Status: metav1.ConditionFalse, Reason: "InvalidSpec",
	Message: err.Error(), ObservedGeneration: w.Generation,
})
_ = r.Status().Update(ctx, w)
return ctrl.Result{}, nil
```

- Always return transient errors; do not swallow them.
- Never `return ctrl.Result{RequeueAfter: d}, err` — the result is ignored when an error is returned.
- `client.IgnoreNotFound(err)` on the initial `Get`.
- Handle `Conflict` on `Update` by requeueing (or switch to server-side apply / `Patch` with `MergeFrom` to avoid conflicts).
- The default per-item backoff caps at 1000 s (~16.7 min); do not add parallel custom retry loops inside `Reconcile`.
- The apiserver can answer `429 Too Many Requests` (API Priority and Fairness, watch-cache initialization); client-go honours `Retry-After` and retries — do not wrap clients in tight custom retry loops.

---

#### Testing

##### Testing Pyramid

```text
                    ┌──────────────────────────┐
                    │    e2e (kind 1.36)       │  full operator in a real cluster
                    ├──────────────────────────┤
                    │  integration (envtest)   │  real 1.36 apiserver + etcd, no kubelet
                    ├──────────────────────────┤
                    │   unit (fake client)     │  pure reconcile logic
                    └──────────────────────────┘
```

##### Unit Tests (fake client)

```go
func TestReconcileCreatesDeploymentAndIsIdempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := examplev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	w := &examplev1.Widget{
		ObjectMeta: metav1.ObjectMeta{Name: "w", Namespace: "default", Generation: 1},
		Spec:       examplev1.WidgetSpec{Replicas: 3, MaxReplicas: 5, Tier: examplev1.TierSmall},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(w).
		WithStatusSubresource(&examplev1.Widget{}). // required since v0.15 when the code writes status
		Build()
	rec := events.NewFakeRecorder(10)
	r := &WidgetReconciler{Client: c, Scheme: scheme, Recorder: rec, External: noopExternal{}}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "w"}}

	for i := 0; i < 2; i++ { // 1st adds finalizer and applies, 2nd must be a no-op
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("reconcile %d: %v", i, err)
		}
	}
	dep := &appsv1.Deployment{}
	if err := c.Get(context.Background(), req.NamespacedName, dep); err != nil {
		t.Fatal(err)
	}
	if *dep.Spec.Replicas != 3 {
		t.Fatalf("replicas = %d", *dep.Spec.Replicas)
	}
	got := &examplev1.Widget{}
	_ = c.Get(context.Background(), req.NamespacedName, got)
	if !meta.IsStatusConditionTrue(got.Status.Conditions, "Ready") || got.Status.ObservedGeneration != got.Generation {
		t.Fatalf("status = %+v", got.Status)
	}
	if n := len(rec.Events); n != 1 {
		t.Fatalf("want exactly one event, got %d", n)
	}
}
```

- The controller-runtime fake client supports `Apply` (server-side apply) with typed apply configurations; it does not run admission, CEL validation, defaulting or garbage collection. Use envtest for anything that depends on the apiserver.
- `events.NewFakeRecorder` (client-go `tools/events`) matches the new recorder interface; `record.NewFakeRecorder` is for the deprecated one.

##### Integration Tests (envtest)

- Pin envtest binaries to 1.36 (`setup-envtest use 1.36.x`); CEL rules, MutatingAdmissionPolicy v1, strict IP validation and numeric-format checks only behave like production when the test apiserver is 1.36.
- envtest has no controllers: no garbage collection, no namespace finalization, no Deployment → ReplicaSet → Pod chain, no default ServiceAccount. Assert on the objects your controller creates, not on Pods.
- Use `Eventually` with timeouts; never `time.Sleep`.

```go
var _ = Describe("Widget controller", func() {
	It("creates a Deployment", func(ctx SpecContext) {
		w := &examplev1.Widget{ /* ... */ }
		Expect(k8sClient.Create(ctx, w)).To(Succeed())
		Eventually(func(g Gomega) {
			dep := &appsv1.Deployment{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w), dep)).To(Succeed())
			g.Expect(*dep.Spec.Replicas).To(Equal(w.Spec.Replicas))
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})
})
```

##### Manifest Tests

- `kubectl apply --dry-run=server -f <dir>` against a 1.36 apiserver (envtest or kind) catches unknown fields, removed API versions, CEL failures, strict IP validation, admission policy rejections and PSA violations. `--dry-run=client` catches none of these.
- Keep server-side field validation strict (`kubectl` default `--validate=strict`); `--validate=false` silently drops fields the server does not know (verified with 1.37-only Pod fields on 1.36).
- For Helm: `helm template ... | kubectl apply --dry-run=server -f -`, or `helm install --dry-run=server`.

##### Testing Rules

- Reconcile twice and assert the second run is a no-op (no writes, no Events).
- Test the deletion path including finalizer removal.
- Test status on success and error paths, including `observedGeneration`.
- Test that invalid objects are rejected by CEL/VAP/webhooks.

---

#### kubectl 1.36 for Reviewers

| Command | Note |
|---|---|
| `kubectl explain -R deploy.spec` | **[NEW in 1.36]** short flag for `--recursive`. The release notes say `-r`; the shipped flag is **`-R`** (`kubectl explain -r` → `unknown shorthand flag: 'r'`). kubectl 1.35 has no short form. |
| `kubectl wait --for=condition=Ready --for=create pod/x` | **[NEW in 1.36]** multiple `--for` conditions, ANDed in order (`create` first) |
| `kubectl diff --show-secrets` | **[NEW in 1.36]** flag to print Secret values; masking by default already existed (kubectl 1.35 shows `*** (before)`/`*** (after)`) |
| `kubectl attach --detach-keys` / `kubectl run --detach-keys` | **[NEW in 1.36]** default `ctrl-p,ctrl-q` |
| `kubectl describe` on many objects | **[NEW in 1.36]** no events unless `--show-events` |
| `kubectl debug` | **[NEW in 1.36]** default profile `general` (was `legacy`) |
| `kubectl kuberc set --section credentialplugin --policy Allowlist --allowlist-entry command=<helper>` | **[NEW in 1.36]** restrict exec credential plugins; `AllowlistEntry.Name` was renamed to `Command`. Disable kuberc with `KUBECTL_KUBERC=false` or `KUBERC=off`. |
| `kubectl get ... -o kyaml` | KYAML output (beta in 1.35/1.36, Stable in 1.37) |
| `kubectl patch pod x --subresource resize ...` | in-place resize (GA 1.35) |
| `kubectl apply --dry-run=client` on existing objects | 1.36 fixed it to print the merged manifest instead of the server state |
| `kubectl rollout undo` on applied objects | 1.36 warns about `last-applied-configuration` mismatch |

---

#### Removed / Changed in 1.36 — Migration Notes

Upgrading manifests, charts and controllers from 1.35 to 1.36:

1. **StatefulSet `maxUnavailable`** — dropped on 1.36 (gate beta/off). Remove it from 1.36 manifests to stop GitOps drift; expect one-at-a-time rolling updates until 1.37.
2. **Strict IP/CIDR validation** — search manifests and controller code for IP literals with leading zeros and CIDRs with host bits (`10.1.2.3/16`). New writes fail on 1.36; fix them before upgrading, including values generated by controllers (canonicalize with `net/netip`).
3. **Service `externalIPs`** — deprecated; plan a migration to LoadBalancer/NodePort/Gateway API before the planned v1.43 removal. Treat new uses as review findings.
4. **`gitRepo` volumes** — replace with an init container or git-sync; the plugin can no longer be enabled.
5. **MutatingAdmissionPolicy** — switch `admissionregistration.k8s.io/v1beta1` manifests to `v1` (the beta API was never on by default). Consider replacing simple mutating webhooks.
6. **CRD numeric formats** — align `format: int32`/`int64` with Go field types and add `minimum`/`maximum`; values outside the format range are rejected on create/update of that field.
7. **Suspended Jobs** — queue controllers may now resize suspended Jobs' Pod templates; on 1.35 that update fails. Do not backport such logic to 1.35 without a feature check.
8. **Service names** — leading digits are allowed on 1.36 only; keep letter-first names in charts that support older clusters.
9. **Go modules** — bump to Go 1.26, `k8s.io/*` v0.36.x, controller-runtime v0.24.x, controller-tools v0.21.x; fix custom `cache.Store` implementations (`Bookmark`, `LastStoreSyncResourceVersion`); replace `FieldsV1.Raw` with accessors; replace `pkg/scheme.Builder`; replace `sets.String` uses in admission code (`NewLifecycle` now takes `sets.Set[string]`); stop relying on `ProtoMessage()` on API types.
10. **controller-runtime v0.23 changes** (if coming from v0.22): typed webhooks, events.k8s.io recorder and RBAC, fake-client SSA resourceVersion, priority queue default.
11. **Metrics** — rename `volume_operation_total_errors` and `etcd_bookmark_counts` in dashboards and alerts.
12. **Ingress NGINX** — retired upstream on 2026-03-24; inventory annotations and plan the migration.
13. **SELinux** — audit Pods that share volumes with different SELinux labels and set `seLinuxChangePolicy: Recursive` where needed before 1.37 turns on `SELinuxMount`.
14. **Init-container resize policy** — drop `resizePolicy` entries with `restartPolicy: RestartContainer` from non-sidecar init containers; 1.36 rejects them.
15. **Alpha workload scheduling** — 1.35 manifests that set the alpha Pod `spec.workloadRef` or use `scheduling.k8s.io/v1alpha1` Workload must move to `spec.schedulingGroup` and `v1alpha2` (still alpha, gate off by default).
16. **Patch level** — run ≥ 1.36.3 (SSA list/map regression, kubelet memory leak) and preferably the latest 1.36.x.

***

#### Production / Review Checklist

##### Version and Compatibility

- [ ] Target cluster minor confirmed as 1.36; nothing from 1.37+ (`storagemigration.k8s.io/v1`, HPA `minReplicas: 0`, `httpGet.protocol`, `bindMountOptions`, `emptyDir.mode`, StatefulSet `Recreate`, Pod certificates GA, `metrics.k8s.io/v1`)
- [ ] No removed API versions (`extensions/v1beta1`, `batch/v1beta1`, `policy/v1beta1`, `autoscaling/v2beta2`, `flowcontrol.apiserver.k8s.io/v1beta3`, …)
- [ ] Beta API versions only with a documented `--runtime-config` prerequisite
- [ ] No `gitRepo` volumes; no new `externalIPs`
- [ ] No StatefulSet `rollingUpdate.maxUnavailable` relied upon on 1.36
- [ ] IPs and CIDRs canonical (no leading zeros, no host bits)
- [ ] Manifests pass `kubectl apply --dry-run=server` on a 1.36 apiserver with strict field validation

##### Workloads

- [ ] Requests on every container; memory limit == request for predictable OOM behaviour
- [ ] Liveness/readiness (and startup where slow) probes on named ports; liveness does not depend on downstream services
- [ ] PDB for replicated workloads, `unhealthyPodEvictionPolicy: AlwaysAllow`, never blocks drains
- [ ] Topology spread or anti-affinity for HA; `matchLabelKeys: [pod-template-hash]`
- [ ] Images pinned by digest; no `:latest`
- [ ] Sidecars as `initContainers` with `restartPolicy: Always`
- [ ] Jobs: `podFailurePolicy`/`backoffLimitPerIndex` where retries are expensive; `ttlSecondsAfterFinished`
- [ ] CronJobs: `timeZone`, `concurrencyPolicy`, `startingDeadlineSeconds`
- [ ] HPA-managed workloads do not also set `replicas` through apply

##### Security

- [ ] Namespace PSA labels with pinned `enforce-version`; `restricted` for application namespaces
- [ ] `runAsNonRoot`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, `readOnlyRootFilesystem`, `seccompProfile: RuntimeDefault` — or `hostUsers: false` where nodes support user namespaces
- [ ] `procMount: Unmasked` only with `hostUsers: false` and only under Baseline
- [ ] RBAC: no wildcards; status/finalizers subresources; `events.k8s.io` for events; no new `nodes/proxy` (use fine-grained kubelet subresources)
- [ ] `automountServiceAccountToken: false` unless the Pod calls the API
- [ ] Secrets referenced, never inlined; not logged
- [ ] NetworkPolicies: default deny plus explicit allows including DNS
- [ ] Shared volumes with different SELinux labels use `seLinuxChangePolicy: Recursive`

##### CRDs and Policies

- [ ] Kind CamelCase singular; group is a domain you own (not `*.k8s.io`)
- [ ] Status subresource enabled; `observedGeneration` and `conditions` (`listType=map`, key `type`)
- [ ] All strings/lists/maps bounded (`maxLength`, `maxItems`, `maxProperties`)
- [ ] CEL rules for cross-field invariants; immutability via transition rules plus "cannot unset" parent rule; `fieldPath`/`reason` set
- [ ] No `string(<int>)` in `messageExpression` (cost budget)
- [ ] `format` matches the Go type; explicit `minimum`/`maximum`
- [ ] Exactly one storage version; deprecated versions carry `deprecationWarning`
- [ ] ValidatingAdmissionPolicy rolled out with `[Warn, Audit]` before `Deny`; never `[Deny, Warn]`
- [ ] MutatingAdmissionPolicy uses `admissionregistration.k8s.io/v1`, explicit `reinvocationPolicy`, idempotent mutations guarded by `matchConditions`
- [ ] Webhooks: `sideEffects: None`, short `timeoutSeconds`, `matchConditions`, namespace exclusions for the webhook's own namespace, rotated TLS

##### Controllers

- [ ] Go 1.26, `k8s.io/*` v0.36.x (all the same), controller-runtime v0.24.x, controller-tools v0.21.x, envtest 1.36
- [ ] Fetch → `IgnoreNotFound` → deletion/finalizer → ensure finalizer → apply children → status
- [ ] Children owned via `SetControllerReference` or apply-configuration owner references; server-side apply with a stable field owner
- [ ] Status via `Status().Update/Patch/Apply`; skip no-op writes; `ObservedGeneration` on conditions
- [ ] Transient errors returned; permanent errors recorded as conditions without hot-looping; no `Requeue: true`; never `RequeueAfter` together with an error
- [ ] Predicates appropriate: `GenerationChangedPredicate` on `For`, not on `Owns` whose status matters
- [ ] Cache scoped (`ByObject` selectors, `TransformStripManagedFields`)
- [ ] No reliance on the cache reflecting the controller's own writes immediately
- [ ] Events via `GetEventRecorder` (events.k8s.io), only on transitions
- [ ] Leader election on; metrics served securely; healthz/readyz wired
- [ ] Unit tests (fake client with `WithStatusSubresource`) prove idempotency; envtest covers CEL/admission behaviour

##### Observability

- [ ] Structured `logr` key/value logging from `logf.FromContext(ctx)`; no `fmt.Sprintf` messages
- [ ] Custom metrics low-cardinality; no duplication of controller-runtime metrics
- [ ] Dashboards updated for 1.36 metric renames
