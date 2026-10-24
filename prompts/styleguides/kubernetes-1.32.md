### Kubernetes 1.32 — Manifests, Operators & Controllers Review Guide

> **Version**: Kubernetes v1.32.0 "Penelope", released **2024-12-11**. Final patch release: **v1.32.13**, published **2026-02-26** (a Go-only rebuild with Go 1.24.13; `https://dl.k8s.io/release/stable-1.32.txt` reports `v1.32.13`).
> **Support**: 1.32 reached **end of life on 2026-02-28** and is **out of support** as of October 2026 — no further patches, including security fixes. Supported minors are **1.35** (latest v1.35.9), **1.36** (latest v1.36.5) and **1.37** (released 2026-08-26, latest v1.37.1, the newest release); 1.34 (latest v1.34.12) reaches end of life on 2026-10-27, and 1.33 ended on 2026-06-28 (final v1.33.13). Code that targets 1.32 should be reviewed for 1.32 correctness *and* flagged for an upgrade plan.
> **Matching libraries**: `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`, `k8s.io/apiextensions-apiserver` **v0.32.x** (final v0.32.13, `go 1.23.0` directive); **controller-runtime v0.20.x** (v0.20.0 released 2025-01-16, final v0.20.4 on 2025-03-24); **controller-tools v0.17.x** (final v0.17.3); Kubebuilder **v4.5.x** scaffolds controller-runtime v0.20 and controller-tools v0.17. Go **1.23** is the minimum toolchain for these modules.
> **Scope**: Kubernetes manifests (plain YAML, Kustomize output, Helm-rendered YAML), CustomResourceDefinitions with CEL validation, admission policies, and Go operators/controllers built with controller-runtime, client-go or Kubebuilder that target a 1.32 cluster.
> **Note**: This document is self-contained. House rules for operator design, CRDs, status, finalizers, RBAC, webhooks, observability and testing are carried forward; sections marked **[NEW in 1.32]** cover behaviour specific to this release. Every manifest, CRD and policy example was applied (or `--dry-run=server`-applied) against a real v1.32.0 kube-apiserver; boundary claims were re-checked on v1.31.0 and v1.33.0 apiservers; Go snippets compile against k8s.io/* v0.32.13 + controller-runtime v0.20.4 and the reconciler was exercised with envtest on the 1.32 binaries. Pod *runtime* behaviour (kubelet, scheduler, controllers) is sourced from the release notes, KEPs and documentation, not executed.

---

#### Version-aware review

The target is a **1.32 control plane** and **k8s.io/\* v0.32.x** client libraries. Before claiming that a field, API version or library function is "not available", check it against the pinned version:

- For manifests: the version comes from the cluster (`kubectl version`), the Helm chart's `kubeVersion`, CI's `kind`/`envtest` image, or a Kustomize/Helm render target. A field that exists in the OpenAPI schema is **not** proof that it works. `kubectl explain` on 1.32 documents alpha-gated fields such as `pod.spec.resources` (pod-level resources), `pod.spec.hostUsers`, `securityContext.supplementalGroupsPolicy` and `volumes[].image`, but a default 1.32 apiserver **silently drops** them on create. Verified: a Pod using all four is accepted with `rc=0` and no warning, and the stored object contains none of them — the image volume *and the container's `volumeMount` that referenced it* both disappear. Only `kubectl apply --dry-run=server -o yaml` against a 1.32 apiserver with the cluster's real feature gates proves a field is honoured.
- For Go code: the version comes from `go.mod` (`k8s.io/api v0.32.x`, `sigs.k8s.io/controller-runtime v0.20.x`). Verify a symbol with `go doc` in the module before flagging it (for example `go doc sigs.k8s.io/controller-runtime/pkg/webhook/admission CustomValidator` exists on v0.20, `admission.Validator` does not).
- Say "not available in 1.32" only when the pinned version proves it (unknown field, `Unsupported value`, dropped field, `no matches for kind`, missing package/function). Say "available but off by default" for alpha features and for beta **APIs** (since 1.24, new beta API versions are not served unless `--runtime-config` enables them).
- A cluster may enable feature gates. If the repository contains cluster configuration (kubeadm `ClusterConfiguration`, kind config, managed-cluster settings) that enables a gate, review against that configuration, not against the defaults below.
- **Patch level matters.** The 1.32 line changed defaults and fixed regressions in patch releases: `ServiceAccountNodeAudienceRestriction` was turned **off** in v1.32.2; `OrderedNamespaceDeletion` was added (off by default) in v1.32.3; v1.32.3 fixed 1.32 regressions with `postStart` hooks and kubelet status reporting after restart; four CVEs were fixed in v1.32.1, v1.32.2, v1.32.6 and v1.32.8 (see "Removed / changed in 1.32").
- **Managed 1.32 clusters** (EKS/GKE/AKS extended-support channels) may run 1.32 beyond the upstream end of life; that support comes from the vendor, not from the Kubernetes project. Do not describe upstream 1.32 as supported.

---

#### What's New in Kubernetes 1.32

Kubernetes 1.32 contains 44 enhancements: 13 stable, 12 beta and 19 alpha. The tables list every change a manifest author, CRD author or controller author can observe. "Default" is the state on an unmodified v1.32.0 control plane; gates were read from `kube-apiserver --help` of the v1.31.0, v1.32.0 and v1.33.0 binaries and from `pkg/features/versioned_kube_features.go` at the v1.32.0 and v1.32.13 tags, and API behaviour was tested on those apiservers.

##### Graduated to stable (GA) in 1.32

| Feature | What it means for manifests / controllers | KEP |
|---|---|---|
| **Custom resource field selectors** (`CustomResourceFieldSelectors`) | CRD `spec.versions[].selectableFields`; `kubectl get --field-selector spec.x=y` and field-selector watches on CRs work server-side | 4358 |
| **StatefulSet PVC auto-deletion** (`StatefulSetAutoDeletePVC`) | `spec.persistentVolumeClaimRetentionPolicy.whenDeleted` / `whenScaled`: `Retain` (default) or `Delete` | 1847 |
| **Size memory-backed volumes** (`SizeMemoryBackedVolumes`) | `emptyDir: {medium: Memory}` is sized by the Pod's memory limit (or `sizeLimit` when smaller) instead of half the node's RAM | 1967 |
| **Bound service-account token improvements** (`ServiceAccountTokenJTI`, `ServiceAccountTokenPodNodeInfo`, `ServiceAccountTokenNodeBindingValidation`) | Tokens carry a `jti` and the node name/UID; the apiserver exposes them as user extra `authentication.kubernetes.io/credential-id`, `.../node-name`, `.../node-uid` for authorization and ValidatingAdmissionPolicy | 4193 |
| **Structured authorization configuration** (`StructuredAuthorizationConfiguration`) | `--authorization-config` accepts `apiserver.config.k8s.io/v1` `AuthorizationConfiguration` (multiple webhooks, CEL `matchConditions`) | 3221 |
| **Retry generateName** (`RetryGenerateName`) | The apiserver retries name generation on a collision instead of returning `409 AlreadyExists` for `generateName` creates | 4420 |
| **LoadBalancer IP mode** (`LoadBalancerIPMode`) | `status.loadBalancer.ingress[].ipMode: VIP \| Proxy` tells kube-proxy whether to short-circuit traffic to the LB IP | 1860 |
| `status.hostIPs` on Pods | Dual-stack host IPs. Listed as a 1.32 graduation in the release announcement, but the `PodHostIPs` gate was already GA and locked on since 1.30; 1.32 only removes the gate | 2681 |
| **Custom profile in `kubectl debug`** | `kubectl debug --custom=<json/yaml partial container spec>` | 4292 |
| CronJob scheduled-timestamp annotation (`CronJobsScheduledAnnotation`) | Jobs created by a CronJob carry `batch.kubernetes.io/cronjob-scheduled-timestamp` (RFC 3339) | 4026 |
| Pod index label (`PodIndexLabel`) | StatefulSet Pods get `apps.kubernetes.io/pod-index`; Indexed Job Pods get `batch.kubernetes.io/job-completion-index` | 4017 |
| Kubelet memory manager, topology-manager policy options | Node configuration (not manifest-visible) | 1769, 3545 |

##### Beta in 1.32 (with default)

| Feature / gate | Default in 1.32 | Notes |
|---|---|---|
| Job `spec.managedBy` (`JobManagedBy`) | **on** (was alpha/off in 1.31) | Verified: 1.31 drops the field silently; 1.32 keeps it |
| Env var names with any printable ASCII except `=` (`RelaxedEnvironmentVariableValidation`) | **on** (was alpha/off) | Verified: `name: "1st var"` accepted on 1.32, rejected on 1.31 |
| Anonymous auth limited to configured endpoints (`AnonymousAuthConfigurableEndpoints`) | **on** | `AuthenticationConfiguration.anonymous.conditions[].path` |
| Field/label selectors in authorization (`AuthorizeWithSelectors`, `AuthorizeNodeWithSelectors`) | **on** | Node authorizer limits kubelets to their own Node and their bound Pods |
| Recover from volume expansion failure (`RecoverVolumeExpansionFailure`) | **on** | Retry a failed PVC expansion with a smaller size |
| Scheduler QueueingHints (`SchedulerQueueingHints`) | **on** (was beta/off) | No manifest change |
| Streaming watch-list on the apiserver (`WatchList`) | **on** (server); `WatchListClient` in client-go stays **off** | |
| B-tree watch cache (`BtreeWatchCache`) | **on** | |
| Kubelet node-audience restriction for SA tokens (`ServiceAccountNodeAudienceRestriction`) | **on in v1.32.0–v1.32.1, off from v1.32.2** | Reverted because `azureFile` in-tree volumes broke |
| systemd watchdog for the kubelet (`SystemdWatchdog`) | **on** | |
| **DRA structured parameters** — `resource.k8s.io/v1beta1` (`DynamicResourceAllocation`) | **off** (gate beta/off, API not served by default) | Classic DRA removed; `v1beta1` becomes the storage version when enabled |
| Volume group snapshots (CSI external CRDs `groupsnapshot.storage.k8s.io/v1beta1`) | n/a (out-of-tree CRDs) | Requires the external snapshotter |

##### Alpha in 1.32 (off by default — treat as unavailable)

**MutatingAdmissionPolicy** `admissionregistration.k8s.io/v1alpha1` (`MutatingAdmissionPolicy` + runtime-config); **pod-level resources** `spec.resources` (`PodLevelResources`); **zero-second `preStop.sleep`** (`PodLifecycleSleepActionAllowZero`); asynchronous scheduler preemption (`SchedulerAsyncPreemption`); DRA `ResourceClaim.status.devices` (`DRAResourceClaimDeviceStatus`) and `DRAAdminAccess`; `/statusz` and `/flagz` (`ComponentStatusz`, `ComponentFlagz`); `PodLogOptions.stream` stdout/stderr split (`PodLogsQuerySplitStreams`); DNS search domains `.` and `_` (`RelaxedDNSSearchValidation`); `securityContext.seLinuxChangePolicy` (`SELinuxChangePolicy`); kubelet fine-grained authorization `nodes/healthz`, `nodes/pods`, `nodes/configz` (`KubeletFineGrainedAuthz`); per-node CrashLoopBackOff cap (`KubeletCrashLoopBackOffMax`); CBOR serving (`CBORServingAndStorage`, `ClientsAllowCBOR`, `ClientsPreferCBOR`); unsafe deletion of corrupt objects (`AllowUnsafeMalformedObjectDeletion`, delete option `ignoreStoreReadErrorWithClusterBreakingPotential`); external SA token signer (`ExternalServiceAccountTokenSigner`); Windows graceful node shutdown and CPU/memory affinity; `coordination.k8s.io/v1alpha2` LeaseCandidate (replaces `v1alpha1`). In-place Pod resize (`InPlacePodVerticalScaling`) stays **alpha** in 1.32 even though the `pods/resize` subresource was added.

##### Other 1.32 changes that affect reviews

- **API removal**: `flowcontrol.apiserver.k8s.io/v1beta3` FlowSchema/PriorityLevelConfiguration is no longer served (use `v1`, available since 1.29). Verified: 1.31 accepts it with `Warning: ... deprecated in v1.29+, unavailable in v1.32+`; 1.32 fails with `no matches for kind "FlowSchema" in version "flowcontrol.apiserver.k8s.io/v1beta3"`.
- **Classic DRA withdrawn**: the `DRAControlPlaneController` gate and `PodSchedulingContext` are gone; only structured parameters remain.
- **`kubernetes.io/enforce-mountable-secrets` deprecated**: verified, 1.32 returns `Warning: metadata.annotations[kubernetes.io/enforce-mountable-secrets]: deprecated in v1.32+; prefer separate namespaces to isolate access to mounted secrets` (no warning on 1.31).
- **Pod Security**: `baseline` and `restricted` at version `v1.32` allow the `net.ipv4.tcp_rmem` and `net.ipv4.tcp_wmem` sysctls. Verified: the same Pod is admitted in a namespace pinned to `restricted:v1.32` and rejected with `forbidden sysctls (net.ipv4.tcp_rmem)` in one pinned to `restricted:v1.31`.
- **CRD CEL**: field names that are CEL reserved words (`namespace`, `in`, `as`, `break`, …) can be used directly (`self.namespace`); the Kubernetes **name-format library** (`format.dns1123Label().validate(...)` etc.) is new. Verified: both compile on 1.32 and fail on 1.31 (`undefined field 'namespace'` / `undeclared reference`).
- **`kubectl apply --server-side --subresource`** (for example `--subresource=status`) is new in kubectl 1.32 (kubectl 1.31: `unknown flag: --subresource`). `kubectl debug --custom` is GA.
- **Scheduler**: `AzureDiskLimits`, `CinderLimits`, `EBSLimits` and `GCEPDLimits` plugins were removed — scheduler configs that enable them fail; CSI volume limits come from `CSINode`.
- **kube-controller-manager**: legacy in-tree cloud-provider controllers are removed; `--cloud-provider` accepts only `external` or empty; the `--node-monitor-grace-period` default rose from 40 s to 50 s (Nodes are marked `NotReady` later).
- **Kubelet**: `--runonce` removed; `singleProcessOOMKill` config option added (cgroup v2); image pull back-off messages now include the pull error in `state.waiting.message`.
- **`DisableNodeKubeProxyVersion`** reverted to off: `node.status.nodeInfo.kubeProxyVersion` is still populated in 1.32 (removed in 1.33). Do not read it.
- **Strict config decoding**: `--admission-control-config-file` (including its `ResourceQuotaConfiguration` section), `--egress-selector-config-file`, `--tracing-config-file`, kube-controller-manager `--leader-migration-config` and the kubelet `--image-credential-provider-config` reject unknown or duplicate fields — typos that were ignored on 1.31 now stop the component.
- **Overlapping volume paths warn**: a `projected`, `configMap`, `secret` or `downwardAPI` volume whose items overlap (`x` and `x/y`) now returns a warning. Verified on 1.32: `Warning: volume "p" (Projected): overlapping paths: "x/y" (Secret "b") with "x" (ConfigMap "a")`; exact duplicates remain an error (`conflicting duplicate paths`). Treat the warning as a bug: one source silently shadows the other.
- **Structured authentication configuration** rejects `claimMappings.extra` keys in the `k8s.io` and `kubernetes.io` namespaces; kube-apiserver gains `--requestheader-uid-headers` (alpha `RemoteRequestHeaderUID`) for front proxies that pass a user UID.
- **Device plugins / CDI**: the kubelet no longer passes CDI device information through annotations; containerd older than v1.7.2 must be upgraded on nodes that use CDI devices.
- **Feature gates removed after GA** (setting them now fails at startup): `ValidatingAdmissionPolicy`, `HPAContainerMetrics`, `ServerSideApply`, `ServerSideFieldValidation`, `KMSv2`, `KMSv2KDF`, `MinDomainsInPodTopologySpread`, `CloudDualStackNodeIPs`, `LegacyServiceAccountTokenCleanUp`, `NewVolumeManagerReconstruction`, `NodeOutOfServiceVolumeDetach`, `StableLoadBalancerNodeSet`, `ZeroLimitedNominalConcurrencyShares`, `PodHostIPs`, `ComponentSLIs`, `AllowServiceLBStatusOnNonLB`.
- **`ScalingReplicaSet` event text** changed to `Scaled up replica set <name> from <old> to <new>`; tooling that parses the old wording breaks.

##### Release-note discrepancy found while testing

The v1.32.0 changelog says "PodLifecycleSleepAction is graduated to GA", but the v1.32.0 apiserver still reports `PodLifecycleSleepAction=true|false (BETA - default=true)`, the v1.32.13 source keeps it at beta, and the v1.34 release announcement lists "Introducing Sleep Action for PreStop Hook" as graduating to stable in **1.34**. Treat `lifecycle.preStop.sleep` as **beta, on by default** in 1.32 (usable; seconds must be ≥ 1).

---

#### API versions and feature gates on a 1.32 cluster

##### API group versions served by a default v1.32.0 apiserver

Verified with `kubectl api-versions` on a stock v1.32.0 apiserver (v1.31.0 additionally serves `flowcontrol.apiserver.k8s.io/v1beta3`; v1.33.0 serves exactly the same list as 1.32):

```text
admissionregistration.k8s.io/v1   apiextensions.k8s.io/v1   apiregistration.k8s.io/v1
apps/v1   authentication.k8s.io/v1   authorization.k8s.io/v1   autoscaling/v1   autoscaling/v2
batch/v1   certificates.k8s.io/v1   coordination.k8s.io/v1   discovery.k8s.io/v1
events.k8s.io/v1   flowcontrol.apiserver.k8s.io/v1   networking.k8s.io/v1   node.k8s.io/v1
policy/v1   rbac.authorization.k8s.io/v1   scheduling.k8s.io/v1   storage.k8s.io/v1   v1
```

No `v1beta1`/`v1alpha*` group version is served by default. On a default 1.32 cluster these are **not** served (verified with `kubectl get --raw /apis/<group>/<version>` → NotFound) and manifests using them fail with `no matches for kind`:

| API | Status in 1.32 | How to enable / what to use |
|---|---|---|
| `admissionregistration.k8s.io/v1alpha1` MutatingAdmissionPolicy(Binding) | alpha, off | `--feature-gates=MutatingAdmissionPolicy=true --runtime-config=admissionregistration.k8s.io/v1alpha1=true` (verified); beta `v1beta1` in 1.34, GA `v1` in 1.36 |
| `resource.k8s.io/v1beta1` (DRA structured parameters) | beta, off | gate `DynamicResourceAllocation` + runtime-config; `v1alpha3` also off; GA `v1` in 1.34 |
| `storage.k8s.io/v1beta1` VolumeAttributesClass | beta, off | gate `VolumeAttributesClass` + runtime-config; GA `v1` in 1.34 |
| `networking.k8s.io/v1beta1` ServiceCIDR / IPAddress | beta, off (`MultiCIDRServiceAllocator`) | GA `networking.k8s.io/v1` in 1.33 |
| `certificates.k8s.io/v1alpha1` ClusterTrustBundle | alpha, off | beta in 1.33 |
| `storagemigration.k8s.io/v1alpha1` StorageVersionMigration | alpha, off | use the out-of-tree kube-storage-version-migrator |
| `coordination.k8s.io/v1alpha2` LeaseCandidate | alpha, off | `v1alpha1` was dropped in 1.32 — delete `v1alpha1` objects before upgrading |
| `flowcontrol.apiserver.k8s.io/v1beta3` | **removed** in 1.32 | `flowcontrol.apiserver.k8s.io/v1` |

##### Selected feature gates (kube-apiserver v1.32.0 defaults)

```text
JobManagedBy=true (BETA)                         RelaxedEnvironmentVariableValidation=true (BETA)
SidecarContainers=true (BETA)                    PodLifecycleSleepAction=true (BETA)
JobBackoffLimitPerIndex=true (BETA)              JobSuccessPolicy=true (BETA)
JobPodReplacementPolicy=true (BETA)              CRDValidationRatcheting=true (BETA)
RecursiveReadOnlyMounts=true (BETA)              ServiceTrafficDistribution=true (BETA)
MatchLabelKeysInPodTopologySpread=true (BETA)    MatchLabelKeysInPodAffinity=true (BETA)
NodeInclusionPolicyInPodTopologySpread=true (BETA)  HonorPVReclaimPolicy=true (BETA)
NFTablesProxyMode=true (BETA)                    RecoverVolumeExpansionFailure=true (BETA)
AnonymousAuthConfigurableEndpoints=true (BETA)   AuthorizeWithSelectors=true (BETA)
AuthorizeNodeWithSelectors=true (BETA)           StructuredAuthenticationConfiguration=true (BETA)
ServiceAccountTokenNodeBinding=true (BETA)       ServiceAccountNodeAudienceRestriction=true (BETA) -> false from v1.32.2
WatchList=true (BETA)                            WatchListClient=false (BETA)
ConsistentListFromCache=true (BETA)              NodeSwap=true (BETA)
UserNamespacesSupport=false (BETA)               ProcMountType=false (BETA)
VolumeAttributesClass=false (BETA)               MultiCIDRServiceAllocator=false (BETA)
DynamicResourceAllocation=false (BETA)
InPlacePodVerticalScaling=false (ALPHA)          PodLevelResources=false (ALPHA)
PodLifecycleSleepActionAllowZero=false (ALPHA)   ImageVolume=false (ALPHA)
SupplementalGroupsPolicy=false (ALPHA)           MaxUnavailableStatefulSet=false (ALPHA)
MutatingAdmissionPolicy=false (ALPHA)            StorageVersionMigrator=false (ALPHA)
ClusterTrustBundle=false (ALPHA)                 RelaxedDNSSearchValidation=false (ALPHA)
SELinuxChangePolicy=false (ALPHA)                SELinuxMount=false (ALPHA)
KubeletFineGrainedAuthz=false (ALPHA)            CoordinatedLeaderElection=false (ALPHA)
OrderedNamespaceDeletion=false (BETA, added in v1.32.3)
```

GA gates (for example `CustomResourceFieldSelectors`, `StatefulSetAutoDeletePVC`, `RetryGenerateName`, `LoadBalancerIPMode`, `SizeMemoryBackedVolumes`, `StructuredAuthorizationConfiguration`) are locked on and no longer appear in `--help`.

##### Already-GA or beta-on features from earlier releases that 1.32 code should use

| Feature | Status on 1.32 | Use it for |
|---|---|---|
| CRD validation rules (`x-kubernetes-validations`, CEL) | GA since 1.29 | Cross-field and transition validation instead of webhooks |
| ValidatingAdmissionPolicy (`admissionregistration.k8s.io/v1`) | GA since 1.30 | In-process policy instead of validating webhooks |
| Webhook `matchConditions` | GA since 1.30 | Skip webhook calls with CEL pre-filters |
| Pod scheduling gates (`schedulingGates`) | GA since 1.30 | Hold Pods until an external controller is ready |
| AppArmor `securityContext.appArmorProfile` | GA since 1.31 (beta in 1.30) | Replaces the `container.apparmor.security.beta.kubernetes.io/*` annotations |
| `minDomains` in topology spread | GA since 1.30 | Force spreading across N zones |
| Job `podFailurePolicy`, Pod disruption conditions | GA since 1.31 | Fail fast / ignore disruptions |
| StatefulSet `ordinals.start` | GA since 1.31 | Migrations, slicing across clusters |
| CRD `selectableFields`, StatefulSet PVC retention | **GA in 1.32** | Field selectors on CRs, PVC cleanup |
| Sidecar containers (`initContainers[].restartPolicy: Always`) | beta, on (since 1.29) | Log shippers, proxies (GA 1.33) |
| Job `backoffLimitPerIndex`, `successPolicy`, `podReplacementPolicy` | beta, on | Indexed Jobs (GA 1.33/1.33/1.34) |
| CRD validation ratcheting | beta, on (since 1.30) | Tightening validation without breaking stored objects (GA 1.33) |
| `recursiveReadOnly` volume mounts | beta, on (since 1.31) | Truly read-only mounts (GA 1.33) |
| Service `trafficDistribution: PreferClose` | beta, on (since 1.31) | Zonal routing (GA 1.33) |
| `lifecycle.preStop.sleep` (seconds ≥ 1) | beta, on (since 1.30) | Graceful drain without a shell (GA 1.34) |
| `matchLabelKeys` in topology spread and pod (anti-)affinity | beta, on | Per-revision spreading/affinity |

---

#### Do not use or recommend (1.33+ features)

The cluster is 1.32. Never suggest the following as available; they fail validation, are silently dropped, or need a newer apiserver/library. Versions are taken from the 1.33–1.37 release notes; boundaries marked "verified" were tested on v1.32.0 and v1.33.0 apiservers.

**Kubernetes 1.33 (released 2025-04-23):**

- **In-place Pod resize** (beta-on in 1.33). On 1.32 the gate is alpha/off: verified, `kubectl patch pod web --subresource resize ...` returns `Error from server (NotFound): the server could not find the requested resource`, and `containers[].resizePolicy` is silently dropped (kept on 1.33).
- **User namespaces on by default** (`hostUsers: false`): dropped by a default 1.32 apiserver, kept on 1.33 (verified).
- **`supplementalGroupsPolicy`** (beta-on in 1.33): dropped on 1.32, kept on 1.33 (verified).
- **`preStop.sleep.seconds: 0`** (beta-on in 1.33): verified, 1.32 rejects with `must be greater than 0 and less than terminationGracePeriodSeconds (30). Enable AllowPodLifecycleSleepActionZeroValue feature gate for zero sleep` (note the message names a gate that does not exist — the real gate is `PodLifecycleSleepActionAllowZero`); 1.33 accepts it.
- **DNS search entries `.` and `_tcp.example.com`** (`RelaxedDNSSearchValidation`, beta-on in 1.33): verified rejected on 1.32, accepted on 1.33.
- **HPA `behavior.scaleUp/scaleDown.tolerance`** (alpha in 1.33, beta-on in 1.35): verified, 1.32 rejects with `strict decoding error: unknown field "spec.behavior.scaleUp.tolerance"`.
- `networking.k8s.io/v1` **ServiceCIDR** (GA 1.33), ClusterTrustBundle beta, `resource.k8s.io/v1beta2`, ordered namespace deletion on by default, `kuberc` (alpha 1.33), DRA prioritized lists / device taints / partitionable devices, `ContainerStopSignals` (`lifecycle.stopSignal`), Pod `metadata.generation`/`status.observedGeneration` tracking (alpha 1.33, GA 1.35), `PodTopologyLabelsAdmission`, `DeploymentReplicaSetTerminatingReplicas` (`status.terminatingReplicas`).
- **Removal in 1.33**: `node.status.nodeInfo.kubeProxyVersion`; the in-tree `gitRepo` volume driver is disabled by default on 1.33 kubelets (still usable on 1.32 nodes — but do not use it); Endpoints API deprecated (EndpointSlices recommended — already the right choice on 1.32).
- Go: `k8s.io/* v0.33` / controller-runtime **v0.21** (both require Go 1.24), `cache.WaitForNamedCacheSyncWithContext`.

**Kubernetes 1.34 (released 2025-08-27):**

- **DRA GA** `resource.k8s.io/v1`; **VolumeAttributesClass GA** `storage.k8s.io/v1`; `admissionregistration.k8s.io/v1beta1` MutatingAdmissionPolicy; `trafficDistribution: PreferSameZone` / `PreferSameNode` (verified: 1.32 and 1.33 reject them with `Unsupported value: "PreferSameZone": supported values: "PreferClose"`); **pod-level resources beta-on** (`spec.resources` — dropped on 1.32 and 1.33, verified); preStop sleep GA; AppArmor annotations no longer mirrored; KYAML output (`kubectl get -o kyaml`; kubectl 1.32: `unable to match a printer suitable for the output format "kyaml"`, verified); `matchLabelKeys` merged into topology-spread `labelSelector` at Pod creation.
- Go: controller-runtime **v0.22** typed `client.Client.Apply(ctx, applyConfiguration, ...)`, deprecation of `Result.Requeue` (v0.21).

**Kubernetes 1.35 (released 2025-12-17) and later:**

- In-place resize **GA**, Pod generation tracking GA, `PreferSameZone` GA, Job `managedBy` GA, container `restartPolicyRules`, `env[].valueFrom.fileKeyRef`, image volumes on by default (`volumes[].image` is dropped on 1.32, verified), `hostnameOverride`, `pods/exec` WebSocket requests needing `create`, comparable resource versions (`resourceversion.CompareResourceVersion`), controller-runtime **v0.23** generic webhooks (`ctrl.NewWebhookManagedBy(mgr, obj)`, `admission.Validator[T]`) and `mgr.GetEventRecorder` (events.k8s.io), priority queue on by default.
- 1.36: MutatingAdmissionPolicy **v1**, Service names starting with a digit (verified: rejected on 1.32 with `a DNS-1035 label must ... start with an alphabetic character`), strict IP/CIDR validation, numeric `format` range enforcement in CRDs. 1.37: StorageVersionMigration `v1`, Pod certificates GA, StatefulSet `updateStrategy.type: Recreate`.
- StatefulSet `rollingUpdate.maxUnavailable` (`MaxUnavailableStatefulSet`) is alpha/off on 1.32 (and 1.33/1.34): verified, the field is silently dropped (the stored `rollingUpdate` keeps only `partition: 0`). It was beta-on only in v1.35.0–v1.35.3, switched off again in v1.35.4 and 1.36 after a `podManagementPolicy: Parallel` rollout regression, and is on by default from 1.37. Do not rely on it.

**Never recommend** dropping `PreferClose` in favour of `PreferSameZone`, replacing a mutating webhook with a MutatingAdmissionPolicy, or using `kubectl patch --subresource resize` for a 1.32 target — none of these work on a stock 1.32 cluster. Do recommend the **upgrade** itself: 1.32 is out of support.

---

#### Deprecated and removed — what 1.32 code must not rely on

| Item | Status | Replacement |
|---|---|---|
| `flowcontrol.apiserver.k8s.io/v1beta3` | **Removed** in 1.32 | `flowcontrol.apiserver.k8s.io/v1` (note: `v1` keeps an explicit `nominalConcurrencyShares: 0`) |
| Classic DRA (`DRAControlPlaneController`, `PodSchedulingContext`) | **Removed** in 1.32 | Structured parameters (`resource.k8s.io/v1beta1`, opt-in) |
| `coordination.k8s.io/v1alpha1` | **Removed** in 1.32 | `v1alpha2` (alpha) |
| ServiceAccount annotation `kubernetes.io/enforce-mountable-secrets` | Deprecated in 1.32 (warning) | Separate namespaces for secret isolation |
| `node.status.nodeInfo.kubeProxyVersion` | Deprecated (still populated in 1.32, removed in 1.33) | None — the value was never reliable |
| Scheduler plugins `AzureDiskLimits`, `CinderLimits`, `EBSLimits`, `GCEPDLimits` | **Removed** | `NodeVolumeLimits` (CSI) |
| In-tree cloud-provider controllers in kube-controller-manager | **Removed** | External cloud-controller-manager |
| Kubelet `--runonce` | **Removed** | — |
| `gitRepo` volumes | Deprecated since 1.11; still works on 1.32 nodes; disabled by default from 1.33 | init container / git-sync |
| `container.apparmor.security.beta.kubernetes.io/*` annotations | Deprecated (field GA since 1.31); the apiserver still mirrors field ↔ annotation on 1.32 (verified) | `securityContext.appArmorProfile` |
| Service `spec.externalIPs` | Supported, but a known MITM vector (CVE-2020-8554) | LoadBalancer, NodePort, Gateway API; block with `DenyServiceExternalIPs` or a policy |
| controller-runtime `admission.Defaulter` / `admission.Validator` (object-method interfaces) | **Removed** in v0.20 | `admission.CustomDefaulter` / `admission.CustomValidator` |
| controller-runtime `cluster.Options.SyncPeriod` (deprecated top-level) | **Removed** in v0.20 | `cache.Options.SyncPeriod` |
| `k8s.io/utils/pointer` | Deprecated | `k8s.io/utils/ptr` (`ptr.To`, `ptr.Deref`) |
| `client-go/kubernetes/fake.NewSimpleClientset` | Superseded | `fake.NewClientset` (field management, apply) |

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
| **Stale caches are normal** | Reads come from an informer cache that lags the API server. Writes must use optimistic concurrency (`resourceVersion`), patches, or server-side apply; never "read-modify-update" across two reconciles from cached data and assume it is current. Resource versions are **opaque strings** on 1.32 — never parse or compare them numerically. |

---

#### Project Layout & Tooling

##### Recommended Scaffold (Kubebuilder v4)

```
├── api/
│   └── v1/
│       ├── rediscluster_types.go        # API type definitions + markers
│       ├── groupversion_info.go
│       └── zz_generated.deepcopy.go     # generated, never hand-edited
├── internal/
│   ├── controller/
│   │   └── rediscluster_controller.go   # Reconciler
│   └── webhook/v1/
│       └── rediscluster_webhook.go      # CustomValidator / CustomDefaulter
├── config/
│   ├── crd/                             # Generated CRD manifests
│   ├── rbac/                            # Generated RBAC manifests
│   └── default/                         # Kustomize overlay
├── cmd/main.go
├── Dockerfile
└── Makefile                             # make generate, make manifests, make test
```

##### Toolchain versions for a 1.32 target

| Component | Version for 1.32 | Notes |
|---|---|---|
| Go | ≥ 1.23 | `k8s.io/* v0.32`, controller-runtime v0.20 and controller-tools v0.17 declare `go 1.23.0`; v0.33/v0.21/v0.18 need Go 1.24 |
| `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` | v0.32.x (keep all on the **same** patch; v0.32.13 is final) | Mixed minors cause compile or runtime scheme errors |
| controller-runtime | v0.20.x (v0.20.4 final) | v0.20.0 requires `k8s.io/* v0.32.0`, v0.20.4 requires v0.32.1; Go MVS raises them to your v0.32.x patch |
| controller-tools (`controller-gen`) | v0.17.x (v0.17.3 final) | Generates `selectableFields` (`+kubebuilder:selectablefield`), CEL `XValidation` with `messageExpression`/`fieldPath`/`reason` |
| Kubebuilder | v4.5.x (scaffolds controller-runtime v0.20.0–v0.20.4, controller-tools v0.17.1–v0.17.2) | v4.6 moves to controller-runtime v0.21 |
| envtest binaries | 1.32.x (`setup-envtest use 1.32.x`) | Test against the version you ship to |

```go
// go.mod — all k8s.io modules on one patch release
module example.com/redis-operator

go 1.23.0

require (
	k8s.io/api v0.32.13
	k8s.io/apimachinery v0.32.13
	k8s.io/client-go v0.32.13
	sigs.k8s.io/controller-runtime v0.20.4
)
```

Flag in review: `k8s.io/client-go v0.33+` (or controller-runtime v0.21+) in a module that claims to support a 1.32 cluster is usually fine on the wire (client/server skew of ±1 minor is supported), but it drags in a Go 1.24 requirement and 1.33+ API fields the 1.32 server drops. Conversely, a project pinned to v0.32 is itself unmaintained (no fixes since February 2026) — note the upgrade as a low-priority finding, not a bug.

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

Kubernetes uses several distinct name classes, each with its own allowed character set. Use the table below to pick the right rule for the field you're naming.

| Name class | Used for | Allowed characters | Disallowed | Start/end | Max length |
|---|---|---|---|---|---|
| **DNS Subdomain** (RFC 1123) | API groups, most `metadata.name` values (e.g., Pods, Deployments, CRs), CRD `spec.group` | lowercase `a–z`, digits `0–9`, `-`, `.` | uppercase `A–Z`, `_`, whitespace, all other punctuation (`/`, `:`, `@`, `+`, `*`, etc.), unicode | must start AND end with alphanumeric (`[a-z0-9]`) | 253 |
| **DNS Label** (RFC 1123) | `metadata.namespace`, label *values* that must be DNS-safe, container names | lowercase `a–z`, digits `0–9`, `-` | uppercase, `_`, `.`, whitespace, all other punctuation, unicode | must start AND end with alphanumeric | 63 |
| **DNS Label** (RFC 1035, stricter) | **Service names on 1.32** (cannot start with a digit), some legacy fields | lowercase `a–z`, digits `0–9`, `-` | same as above PLUS cannot start with a digit | must start with `[a-z]`, end with alphanumeric | 63 |
| **Qualified Name** (label/annotation key, finalizer) | label keys, annotation keys, finalizer names | name part: `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` ; optional prefix part: DNS Subdomain followed by `/` | name part: whitespace, `/` (except as prefix separator), `:`, all other punctuation | name part must start AND end with alphanumeric | name ≤ 63; prefix ≤ 253 |
| **Label Value** | `metadata.labels` values | `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` (or empty string) | whitespace, `/`, `:`, all other punctuation, unicode | if non-empty: alphanumeric at both ends | 63 |
| **Annotation Value** | `metadata.annotations` values | any valid UTF-8 string (incl. whitespace, JSON, base64) | none per key | n/a | total across all annotations ≤ 256 KiB |
| **Environment variable name** | `env[].name` | **1.32 (beta, on)**: any printable ASCII except `=`; 1.31 and older: `[-._a-zA-Z][-._a-zA-Z0-9]*` | `=` | — | — |
| **ConfigMap / Secret data key** | `data` / `stringData` keys | `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` | `/`, whitespace, `:`, all other punctuation | any alphanumeric | 253 |
| **Path Segment Name** | `metadata.name` for objects exposed in URL paths | DNS Subdomain set MINUS `.` and `..` as full name | same as DNS Subdomain plus the literal names `.` and `..` | as DNS Subdomain | 253 |

> ⚠️ Underscore `_` is **never** valid in a DNS Subdomain or DNS Label name — that rules it out for `metadata.name`, `metadata.namespace`, API group, and Service name. Underscore IS valid in label/annotation keys (name part), label values, and ConfigMap/Secret data keys.

> ⚠️ Uppercase letters are **never** valid in DNS Subdomain or DNS Label names. They ARE valid in label keys (name part), label values, and qualified names.

> ⚠️ **Service names on 1.32 must start with a letter.** Verified: 1.32 rejects `metadata.name: 1password-connect` (`a DNS-1035 label must consist of lower case alphanumeric characters or '-', start with an alphabetic character`). Relaxed Service names arrive only in 1.36.

> **[NEW in 1.32]** Environment variable names: `RelaxedEnvironmentVariableValidation` is beta and on, so 1.32 accepts names such as `1st var` (verified; 1.31 rejects them with `must not start with a digit`). Do not use this: names that are not C identifiers (`[A-Za-z_][A-Za-z0-9_]*`) are invisible to POSIX shells, and the same manifest fails on a 1.31 cluster.

##### API Group, Kind, Resource

| Element | Rule | Example |
|---|---|---|
| **API Group** | Lowercase DNS subdomain you own | `cache.example.com` |
| **Kind** | CamelCase, singular | `RedisCluster` |
| **Resource (plural)** | Lowercase, plural | `redisclusters` |
| **Short name** | 2–5 lowercase chars; register in `.spec.names.shortNames` | `rcl` → avoid conflicts with built-ins |
| **Category** | Lowercase; register in `.spec.names.categories` | `all` (use sparingly) |

> ⚠️ Do **not** use the empty group, single-word names, or `*.k8s.io` / `*.kubernetes.io` — those are reserved by the Kubernetes project (they require an `api-approved.kubernetes.io` annotation).

> ⚠️ **Quote one-letter kinds and names.** YAML 1.1 turns unquoted `y`, `Y`, `n`, `N`, `yes`, `no`, `on`, `off` into booleans. Verified on 1.32: a CRD with `names: {kind: N, plural: ns}` fails with `json: cannot unmarshal bool into Go struct field CustomResourceDefinitionNames.spec.names.kind of type string`, and `metadata: {name: n}` fails the same way for `ObjectMeta.metadata.name`.

##### Go Type Names

```go
// ✅ Correct
type RedisCluster struct { ... }
type RedisClusterSpec struct { ... }
type RedisClusterStatus struct { ... }
type RedisClusterList struct { ... }

// ❌ Wrong
type rediscluster struct { ... }
type RedisClusterSpecs struct { ... }
```

##### JSON Field Names

| Rule | ✅ Good | ❌ Bad |
|---|---|---|
| camelCase | `replicaCount` | `replica_count`, `ReplicaCount` |
| Declarative nouns (desired state) | `replicas` | `setReplicas` |
| Name references | `secretName` | `secretRef.name` (unless cross-namespace) |
| Object references | `secretRef` | `secret` |
| Duration | `timeoutSeconds` (`int32`) | `timeout: "30s"` in spec primitive fields |
| Avoid novel abbreviations — `max`/`min` prefixes ARE the established idiom (`maxReplicas`, `maxUnavailable`, `maxSurge`, `minReadySeconds`), as are `url`, `ip`, `id` | `maxRetries`, `minReadySeconds` | `tgtUtilization`, `cfgMapName` (novel abbreviations) |

##### Constant (Enum) Values

```go
// CamelCase string alias — never Go iota enums in API types
type PhaseType string

const (
    PhasePending   PhaseType = "Pending"
    PhaseRunning   PhaseType = "Running"
    PhaseFailed    PhaseType = "Failed"
    PhaseSucceeded PhaseType = "Succeeded"
)
```

##### `client.ObjectKey` in Go Code

When code uses `sigs.k8s.io/controller-runtime/pkg/client.ObjectKey`, assume `Namespace` and `Name` came from Kubernetes object identity (DNS Subdomain / DNS Label rules above) unless there is evidence otherwise.

##### Label & Annotation Key Examples

Use the Qualified Name rules from the character table above. Conventional prefixes:

```
# Labels (user-facing, for selection)
app.kubernetes.io/name: redis
app.kubernetes.io/component: cache
app.kubernetes.io/managed-by: redis-operator

# Annotations (tooling metadata)
cache.example.com/last-backup-time: "2025-01-01T00:00:00Z"
```

**[GA in 1.32]** System-set labels and annotations that controllers may rely on:

- `apps.kubernetes.io/pod-index` on StatefulSet Pods (the ordinal as a label — select a single replica with a label selector instead of parsing the Pod name).
- `batch.kubernetes.io/job-completion-index` on Pods of Indexed Jobs.
- `batch.kubernetes.io/cronjob-scheduled-timestamp` on Jobs created by a CronJob (RFC 3339 scheduled time — use it instead of `creationTimestamp` to detect late or catch-up runs).

Treat these as system-owned: do not set them in templates and do not let a "label fixer" controller remove them.

---

#### CRD & API Design

##### Minimal Skeleton

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=rcl
// +kubebuilder:selectablefield:JSONPath=`.spec.tier`
// +kubebuilder:printcolumn:name="Tier",type=string,JSONPath=`.spec.tier`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// RedisCluster is the Schema for the redisclusters API.
type RedisCluster struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   RedisClusterSpec   `json:"spec"`
    Status RedisClusterStatus `json:"status,omitempty"`
}
```

**[GA in 1.32]** controller-gen v0.17.3 turns `+kubebuilder:selectablefield` into `spec.versions[].selectableFields`; `kubectl get rcl --field-selector spec.tier=premium` then filters server-side (verified on 1.32). A field that is not declared selectable is rejected: verified, `--field-selector spec.replicas=3` returns `field label not supported: spec.replicas`. At most 8 selectable fields per version; each must point to a string (enum/format allowed), integer or boolean field outside `metadata`. Verified: on a 1.31 apiserver the same CRD also works (beta, on); a 1.30 apiserver (gate alpha/off) silently drops `selectableFields`, and the query fails with `field label not supported: spec.tier`.

Controllers benefit too: a controller-runtime cache can be restricted with a field selector on a selectable field (`cache.Options.ByObject[&RedisCluster{}].Field`), which is evaluated server-side instead of filtering after a full LIST.

##### Spec Design Rules

- Use `// +optional` and `omitempty` for optional fields; `// +required` for required.
- Prefer **lists of named subobjects** over maps for portability with strategic merge patch and SSA, and mark them `+listType=map` + `+listMapKey=name`:
  ```yaml
  # ✅ Preferred
  ports:
    - name: redis
      port: 6379
  # ❌ Avoid
  ports:
    redis: 6379
  ```
- **Never use floating-point** fields in `spec`. Use `int32`/`int64`, `resource.Quantity`, or a string with units.
- **Do not use `bool` where an enum will be needed** — use a string type alias from day one.
- **No raw maps** except for labels, annotations, and opaque user data.
- **Object-level fields only**: do not add top-level fields other than `metadata`, `spec`, `status`.
- **Bound everything** that CEL will touch: `maxLength` on strings, `maxItems` on arrays, `maxProperties` on maps. Without bounds the CEL cost estimator assumes worst-case sizes and rejects the CRD (see next section).
- **`format: int32` is not a range check.** Verified on 1.32: a field declared `type: integer, format: int32` without `maximum` stores `3000000000`. Add explicit `minimum`/`maximum` (`+kubebuilder:validation:Minimum/Maximum`); with `maximum: 99` the same value is rejected (`spec.replicas in body should be less than or equal to 99`).

##### OpenAPI / Kubebuilder Validation Markers

```go
// Numeric markers belong on numeric fields
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=99
Replicas int32 `json:"replicas"`

// Enum and Pattern markers belong on string fields (or on the type alias)
// +kubebuilder:validation:Enum=standard;premium
type Tier string

// +kubebuilder:validation:MaxLength=63
// +kubebuilder:validation:Pattern=`^[a-z][a-z0-9-]*$`
NodePrefix string `json:"nodePrefix,omitempty"`
```

##### PrintColumns

Always define at minimum:
1. A human-readable state column (for example the `Ready` condition status)
2. The `Age` column from `metadata.creationTimestamp`

---

#### CEL Validation Rules in CRDs

CRD validation rules (`x-kubernetes-validations`) are GA since 1.29 and are the first choice for invariants that the OpenAPI schema cannot express. The following Go types were run through controller-gen v0.17.3 and the generated CRD was tested on a 1.32.0 apiserver:

```go
// RedisClusterSpec is the desired state.
// +kubebuilder:validation:XValidation:rule="self.tier != 'premium' || self.replicas >= 3",messageExpression="'premium tier needs at least 3 replicas, got %d'.format([self.replicas])",fieldPath=".replicas",reason=FieldValueInvalid
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.storageClassName) || has(self.storageClassName)",message="storageClassName cannot be removed once set"
type RedisClusterSpec struct {
	// +required
	Tier Tier `json:"tier"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=99
	// +required
	Replicas int32 `json:"replicas"`

	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:XValidation:rule="self.contains('@sha256:')",message="image must be pinned by digest"
	// +required
	Image string `json:"image"`

	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="storageClassName is immutable"
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`
}
```

Observed on 1.32 (all errors are reported together; note that 1.32 prints the *type* — `"object"`, `"string"` — in place of the offending value for CEL failures):

```text
* spec.replicas: Invalid value: "object": premium tier needs at least 3 replicas, got 2
* spec.image: Invalid value: "string": image must be pinned by digest
# update fast -> slow, then removal via merge patch {"storageClassName": null}:
* spec.storageClassName: Invalid value: "string": storageClassName is immutable
* spec: Invalid value: "object": storageClassName cannot be removed once set
```

The same kind of rules written directly in a CRD manifest, using the CEL libraries available on 1.32 (`quantity`, `url`/`isURL`, `ip`/`cidr`, string `format`, and the **new** Kubernetes name-format library), plus a reserved-word field name:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: redisclusters.cache.example.com
spec:
  group: cache.example.com
  names: {kind: RedisCluster, listKind: RedisClusterList, plural: redisclusters, singular: rediscluster, shortNames: [rcl]}
  scope: Namespaced
  versions:
  - name: v1
    served: true
    storage: true
    subresources:
      status: {}
    selectableFields:
    - jsonPath: .spec.tier
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            required: [tier, replicas]
            x-kubernetes-validations:
            - rule: "!has(self.maxMemory) || quantity(self.maxMemory).isGreaterThan(quantity('64Mi'))"
              message: maxMemory must be larger than 64Mi
              fieldPath: .maxMemory
            - rule: "self.tier != 'premium' || self.replicas >= 3"
              messageExpression: "'premium tier needs at least 3 replicas, got %d'.format([self.replicas])"
              reason: FieldValueInvalid
              fieldPath: .replicas
            - rule: "!has(self.namespace) || self.namespace != 'kube-system'"   # reserved word, 1.32+
              message: namespace must not be kube-system
              fieldPath: .namespace
            properties:
              tier: {type: string, enum: [standard, premium]}
              replicas: {type: integer, format: int32, minimum: 1, maximum: 99}
              namespace:
                type: string
                maxLength: 63
                x-kubernetes-validations:
                - rule: "!format.dns1123Label().validate(self).hasValue()"      # name-format library, 1.32+
                  message: namespace must be a DNS-1123 label
              storageClassName:
                type: string
                maxLength: 253
                x-kubernetes-validations:
                - rule: "self == oldSelf"
                  message: storageClassName is immutable
              maxMemory:
                x-kubernetes-int-or-string: true
                anyOf: [{type: integer}, {type: string}]
                pattern: '^[0-9]+(Ki|Mi|Gi)?$'
              allowedCIDRs:
                type: array
                maxItems: 8
                items:
                  type: string
                  maxLength: 43
                  x-kubernetes-validations:
                  - rule: "isCIDR(self) && cidr(self).ip().family() == 4"
                    message: must be an IPv4 CIDR
              endpoints:
                type: array
                maxItems: 16
                x-kubernetes-list-type: map
                x-kubernetes-list-map-keys: [name]
                items:
                  type: object
                  required: [name, url]
                  properties:
                    name: {type: string, maxLength: 63}
                    url:
                      type: string
                      maxLength: 2048
                      x-kubernetes-validations:
                      - rule: "isURL(self) && url(self).getScheme() == 'https'"
                        message: url must be an https URL
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
                    reason: {type: string, maxLength: 1024}
                    message: {type: string, maxLength: 32768}
```

Observed results on 1.32 for one invalid object:

```text
* spec.maxMemory: Invalid value: "object": maxMemory must be larger than 64Mi
* spec.replicas: Invalid value: "object": premium tier needs at least 3 replicas, got 2
* spec.endpoints[0].url: Invalid value: "string": url must be an https URL
* spec.namespace: Invalid value: "string": namespace must be a DNS-1123 label
* spec.allowedCIDRs[1]: Invalid value: "string": must be an IPv4 CIDR      # 2001:db8::/32
* spec.allowedCIDRs[2]: Invalid value: "string": must be an IPv4 CIDR      # 10.0.0.1 (not a CIDR)
```

##### CEL library availability on 1.32 (verified by installing one CRD per function)

| Expression | 1.31 | 1.32 | 1.33 |
|---|---|---|---|
| `quantity(...)`, `isURL`/`url(...)`, `isIP`/`ip(...)`, `isCIDR`/`cidr(...)`, `'%d'.format([...])`, `matches`, `lowerAscii`/`split`, `sets.contains` | ✅ | ✅ | ✅ |
| `format.dns1123Label().validate(self)` (name-format library) | ❌ `undeclared reference` | ✅ | ✅ |
| Reserved word as field name (`self.namespace`) | ❌ `undefined field 'namespace'` | ✅ | ✅ |
| `isSemver(self)` / `semver(...)` | ❌ | ❌ `undeclared reference to 'isSemver'` | ❌ |
| `['a'].first()` (lists `first`/`last`) | ❌ | ❌ | ✅ |
| `[3,1].sort()`, `math.abs(-1)`, `self.reverse()` | ❌ | ❌ | ❌ |

Do not paste CEL from newer-version examples (semver checks, list `sort`/`first`) into a 1.32 CRD: the CRD install fails, which in a Helm release means the whole chart fails.

Patch-level CEL fix: before v1.32.4, `cidr(...).containsCIDR(...)` wrongly reported a subnet contained in another CIDR as *not* contained (kubernetes#130773). Rules relying on `containsCIDR` give wrong answers on v1.32.0–v1.32.3.

##### CEL review rules (verified pitfalls)

1. **`string(<int>)` in `messageExpression` blows the cost budget.** Verified on 1.32 (and 1.31/1.33): `'got ' + string(self.size())` on a `maxLength: 64` string is rejected with `estimated messageExpression cost exceeds budget by factor of more than 100x`. Use `'got %d'.format([self.size()])`, which is accepted.
   ```yaml
   # BAD — CRD rejected
   messageExpression: "'premium tier needs >= 3 replicas, got ' + string(self.replicas)"
   # GOOD
   messageExpression: "'premium tier needs at least 3 replicas, got %d'.format([self.replicas])"
   ```
2. **Name-format results are expensive in messages, too.** `messageExpression: "format.dns1123Label().validate(self).value().join(', ')"` on a 63-character string is rejected for exceeding the per-expression *and* the whole-schema budget (verified). Use a static `message`.
3. **`has()` takes a field selection, not a variable.** `!has(oldSelf) || self == oldSelf` fails to compile (verified on 1.31–1.33). With `optionalOldSelf: true` (ratcheting, beta-on in 1.32) `oldSelf` is an optional: use `oldSelf.hasValue()` / `oldSelf.value()`.
4. **`self == oldSelf` does not stop removal.** Transition rules only run when both old and new values exist. Verified on 1.32: a merge patch setting `storageClassName: null` succeeds despite the immutability rule, and a later patch can set a *different* value. Add a parent-level rule such as `!has(oldSelf.storageClassName) || has(self.storageClassName)` when "immutable" must also mean "cannot be unset" (verified: removal is then rejected with `spec: Invalid value: "object": storageClassName cannot be removed once set`).
5. **Unbounded inputs make the CRD uninstallable**, not slow: the cost estimator assumes maximum sizes. Every string/array/map reached by a rule needs `maxLength`/`maxItems`/`maxProperties`.
6. **Prefer `fieldPath` + `reason`** (`FieldValueInvalid`, `FieldValueForbidden`, `FieldValueRequired`, `FieldValueDuplicate`) so clients see the right field and status cause.
7. **Ratcheting** (`CRDValidationRatcheting`, beta and on in 1.32; GA 1.33): unchanged invalid values in stored objects do not block unrelated updates. Clusters can disable a beta gate — do not make a migration plan depend on it — and still treat tightened validation as breaking for clients that create objects.
8. `x-kubernetes-int-or-string` fields are dynamic in CEL; guard with `type(self.x) == string` or use `quantity()` only after a `pattern` restricts the input.
9. **Reserved-word field names** (`namespace`, `in`, `as`, `break`, `const`, `continue`, `else`, `for`, `function`, `if`, `import`, `let`, `loop`, `package`, `return`, `var`, `void`, `while`) are directly addressable from 1.32. A CRD that must also install on 1.31 has to use the escaped form `self.__namespace__`.

---

#### Spec vs. Status

```
┌──────────────────────────────────────────────────────────────┐
│                       ETCD                                    │
│  spec  ← user/agent writes (desired state)                   │
│  status ← controller writes via /status subresource          │
└──────────────────────────────────────────────────────────────┘
```

| | `spec` | `status` |
|---|---|---|
| **Written by** | Users / agents / CI | Controllers only |
| **Represents** | Desired state | Observed / current state |
| **Subresource** | No | **Yes** — always enable with `// +kubebuilder:subresource:status` |
| **PUT/POST behaviour** | Updated | Ignored on the main resource (must use `/status` endpoint) |
| **RBAC** | Users need write | Controller needs `<resource>/status` `update`/`patch` |

##### Updating Status (controller-runtime)

```go
// ✅ Always write status through the subresource; prefer a patch so a stale
// cached object cannot clobber concurrent status writers.
before := obj.DeepCopy()
obj.Status.ObservedGeneration = obj.Generation
if err := r.Status().Patch(ctx, obj, client.MergeFrom(before)); err != nil {
    return ctrl.Result{}, err
}

// ❌ Never
r.Update(ctx, obj) // status changes are ignored when the status subresource is enabled
```

`client.MergeFrom` sends a JSON merge patch without a `resourceVersion` precondition. If two writers own different list entries (for example condition types), use `client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})` or server-side apply of the status subresource with a dedicated field manager: `r.Status().Patch(ctx, u, client.Apply, client.FieldOwner("rediscluster-status"), client.ForceOwnership)` with an `unstructured.Unstructured` carrying `apiVersion`, `kind`, `metadata.name/namespace` and only the status fields you own (verified in envtest on 1.32: `managedFields` gains manager `rediscluster-status`, operation `Apply`, subresource `status`).

##### observedGeneration

Always set `status.observedGeneration = metadata.generation` after a successful reconcile:

```go
obj.Status.ObservedGeneration = obj.Generation
```

This allows external tools to detect whether the controller has processed the latest spec change. On 1.32 **Pods have no `metadata.generation`** (verified: a stored Pod carries no `generation` field; Pod generation tracking is 1.33 alpha / 1.35 GA) — code that waits for a Pod change must compare the actual fields (`status.containerStatuses[].imageID`, readiness), not a Pod `observedGeneration`.

---

#### Status Conditions

Use `metav1.Condition` from `k8s.io/apimachinery/pkg/apis/meta/v1`:

```go
// In your Status struct:
// +listType=map
// +listMapKey=type
// +optional
Conditions []metav1.Condition `json:"conditions,omitempty"`
```

(`+patchStrategy=merge`/`+patchMergeKey=type` only matter for strategic-merge-patch of built-in types; CRDs do not support strategic merge patch, so for CRDs `listType=map` is what SSA and merge semantics use.)

##### Condition Field Rules

| Field | Rule |
|---|---|
| `Type` | PascalCase; short, adjective or past-tense verb: `Ready`, `Degraded`, `Synced`, `Failed` |
| `Status` | `True`, `False`, or `Unknown` — never empty |
| `Reason` | CamelCase, one word, machine-readable (`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`, required, ≤ 1024 chars): `ReconcileError`, `ImagePullBackOff` |
| `Message` | Human-readable sentence; may contain specifics (≤ 32768 chars) |
| `ObservedGeneration` | Copy from `metadata.generation` at the time of observation |
| `LastTransitionTime` | Update **only** when status value transitions; `meta.SetStatusCondition` does this for you |

##### Standard Condition Types

| Type | Normal Status | Meaning |
|---|---|---|
| `Ready` | `True` | Object is fully operational |
| `Reconciling` | `True` | In-progress reconciliation; desired ≠ actual |
| `Stalled` | `False` | Cannot make progress; human attention needed |
| `Degraded` | `False` | Running but with reduced capacity |
| `Available` | `True` | Operational and serving |

> ⚠️ **Deprecated:** Do **not** add new `phase` string fields (e.g., `"Running"`, `"Pending"`). Conditions are the approved pattern — phase is a state machine anti-pattern that hinders backward compatibility.

##### Setting Conditions

```go
changed := meta.SetStatusCondition(&rc.Status.Conditions, metav1.Condition{
    Type:               "Ready",
    Status:             metav1.ConditionFalse,
    ObservedGeneration: rc.Generation,
    Reason:             "ReconcileError",
    Message:            fmt.Sprintf("failed to apply Deployment: %v", err),
})
// changed (apimachinery v0.32: "returns true if the conditions are changed by this call")
// lets you emit Events only on transitions.
```

CRD `status.conditions` (`Established`, `NamesAccepted`) carry **no** `observedGeneration` on 1.32 (verified; `CRDObservedGenerationTracking` appears only in 1.35 as beta/off). Tools that wait for a CRD update must use `kubectl wait --for=condition=Established` plus a discovery check.

---

#### Versioning & API Evolution

##### Version Lifecycle

```
v1alpha1 → v1alpha2 → v1beta1 → v1beta2 → v1
```

| Stage | Stability | Enabled by Default | Backward Compat |
|---|---|---|---|
| `v1alphaX` | Experimental | No | Not guaranteed |
| `v1betaX` | Pre-release | New built-in beta APIs: **No** since v1.24 (KEP-3136) | Best-effort |
| `v1` (GA) | Stable | Yes | **Required** |

For CRDs, "enabled" is not a cluster policy at all — it is simply the `served` flag the CRD author sets on each version.

##### Rules

1. **Never remove or rename existing fields** in a served version.
2. **Never tighten validation** on an existing field in a way that rejects previously valid values (ratcheting protects stored objects on unrelated updates, not clients that create new ones).
3. **Add fields as optional** (`omitempty`), never required.
4. Store only **one primary version** in etcd; all others are views via conversion.
5. Conversion must be **lossless** in both directions (round-trip unknown fields through annotations if needed).
6. For breaking changes: introduce a new version, mark the old as deprecated, maintain support for ≥ 1 year before removal.
7. Use a **conversion webhook** when schemas diverge between versions (controller-runtime: implement `conversion.Hub`/`conversion.Convertible` on the API types and register the webhook).
8. **Before removing an old version from `spec.versions`**, every stored object must be rewritten to the new storage version and the old version removed from `status.storedVersions`. 1.32 has no usable in-tree `StorageVersionMigration` API (`storagemigration.k8s.io/v1alpha1`, alpha/off); use the out-of-tree kube-storage-version-migrator or a read/replace loop (`kubectl get <plural> -A -o json | kubectl replace -f -`), then remove the old version from `status.storedVersions` with `kubectl patch crd <name> --subresource=status`.

##### CRD Served/Storage Annotation

```yaml
versions:
  - name: v1beta1
    served: true
    storage: false    # Old version still served but not stored
    deprecated: true
    deprecationWarning: "cache.example.com/v1beta1 RedisCluster is deprecated; use cache.example.com/v1"
  - name: v1
    served: true
    storage: true     # This is the canonical storage version
```

---

#### Controller / Reconciler Patterns

##### Reconciler Skeleton (controller-runtime v0.20)

The following reconciler compiles against k8s.io/* v0.32.13 + controller-runtime v0.20.4 and was exercised with envtest on the 1.32 control-plane binaries (CEL rejection, SSA-created Deployment with owner reference, status condition transitions, an Event, finalizer-driven deletion).

```go
package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	cachev1 "example.com/op/api/v1"
)

const (
	fieldOwner    = client.FieldOwner("rediscluster-controller")
	finalizerName = "cache.example.com/external-cleanup"
)

// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

type RedisClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder // mgr.GetEventRecorderFor(...), core/v1 Events
	Cleanup  func(ctx context.Context, rc *cachev1.RedisCluster) error
}

func (r *RedisClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch; a missing object is not an error (it was deleted).
	rc := &cachev1.RedisCluster{}
	if err := r.Get(ctx, req.NamespacedName, rc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Deletion: run idempotent cleanup, then drop our finalizer.
	if !rc.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(rc, finalizerName) {
			return ctrl.Result{}, nil
		}
		if r.Cleanup != nil {
			if err := r.Cleanup(ctx, rc); err != nil {
				return ctrl.Result{}, err // retried with backoff
			}
		}
		patch := client.MergeFrom(rc.DeepCopy())
		controllerutil.RemoveFinalizer(rc, finalizerName)
		return ctrl.Result{}, client.IgnoreNotFound(r.Patch(ctx, rc, patch))
	}

	// 3. Ensure the finalizer before creating anything external.
	if !controllerutil.ContainsFinalizer(rc, finalizerName) {
		patch := client.MergeFrom(rc.DeepCopy())
		controllerutil.AddFinalizer(rc, finalizerName)
		if err := r.Patch(ctx, rc, patch); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 4. Converge children (server-side apply, see next section).
	if err := r.applyDeployment(ctx, rc); err != nil {
		return r.fail(ctx, rc, "ApplyFailed", err)
	}

	// 5. Observe and report.
	current := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(rc), current); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	before := rc.DeepCopy()
	rc.Status.ObservedGeneration = rc.Generation
	rc.Status.ReadyReplicas = current.Status.ReadyReplicas
	status, reason := metav1.ConditionFalse, "Progressing"
	if current.Status.ReadyReplicas == rc.Spec.Replicas {
		status, reason = metav1.ConditionTrue, "Available"
	}
	changed := meta.SetStatusCondition(&rc.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             status,
		Reason:             reason,
		Message:            fmt.Sprintf("%d/%d replicas ready", current.Status.ReadyReplicas, rc.Spec.Replicas),
		ObservedGeneration: rc.Generation,
	})
	if err := r.Status().Patch(ctx, rc, client.MergeFrom(before)); err != nil {
		return ctrl.Result{}, err
	}
	if changed && status == metav1.ConditionTrue {
		r.Recorder.Eventf(rc, corev1.EventTypeNormal, "Available", "all %d replicas ready", rc.Spec.Replicas)
	}
	logger.V(1).Info("reconciled", "readyReplicas", current.Status.ReadyReplicas)
	return ctrl.Result{}, nil
}

func (r *RedisClusterReconciler) fail(ctx context.Context, rc *cachev1.RedisCluster, reason string, err error) (ctrl.Result, error) {
	before := rc.DeepCopy()
	if meta.SetStatusCondition(&rc.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            err.Error(),
		ObservedGeneration: rc.Generation,
	}) {
		r.Recorder.Eventf(rc, corev1.EventTypeWarning, reason, "%v", err)
	}
	if perr := r.Status().Patch(ctx, rc, client.MergeFrom(before)); perr != nil {
		return ctrl.Result{}, perr
	}
	return ctrl.Result{}, err
}

func (r *RedisClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1.RedisCluster{}, builder.WithPredicates(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		))).
		Owns(&appsv1.Deployment{}).
		Named("rediscluster").
		Complete(r)
}
```

Review points:

- Finalizer add/remove uses `Patch(client.MergeFrom(...))`, not `Update`, so a stale cached copy cannot overwrite spec changes made between cache sync and write. (A merge patch replaces the whole `finalizers` list; if other controllers add finalizers concurrently, use `client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})`.)
- `GenerationChangedPredicate` on `For()` skips status-only updates (our own status writes would otherwise re-trigger the reconciler). Do not put it on `Owns()` for Deployments: Deployment status changes (ready replicas) are exactly the events the owner must see.
- `GenerationChangedPredicate` drops metadata-only updates (labels, annotations, finalizer edits by others). Deletion still gets through: when the apiserver sets `deletionTimestamp` on an object with finalizers it also increments `metadata.generation` — verified in envtest on 1.32, where the reconciler above removed its finalizer after `Delete` with this predicate in place.
- The `Recorder.Eventf` message is a format string: pass `"%v", err`, never `err.Error()` as the format.

##### Reconcile Return Conventions

| Return | Meaning |
|---|---|
| `ctrl.Result{}, nil` | Success — no requeue until a watched event fires |
| `ctrl.Result{}, err` | Transient error — requeued with per-item exponential backoff (5 ms doubling up to 1000 s by default) |
| `ctrl.Result{RequeueAfter: d}, nil` | Requeue after duration `d` (polling external state) |
| `ctrl.Result{Requeue: true}, nil` | Rate-limited requeue. **Not** deprecated in v0.20 (deprecated from v0.21), but it has no clear semantics — prefer `RequeueAfter` or returning an error |
| `ctrl.Result{RequeueAfter: d}, err` | The error wins; `RequeueAfter` is ignored and controller-runtime logs a warning. Pick one |
| `ctrl.Result{}, reconcile.TerminalError(err)` | Logged and counted, never retried |

controller-runtime v0.20 adds an **opt-in priority queue** (`config.Controller.UsePriorityQueue`, off by default; the v0.20.0 release calls it experimental). Do not enable it in production on v0.20, and do not write code that depends on FIFO ordering between keys either way.

##### Manager Setup

Compiles against controller-runtime v0.20.4:

```go
mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
	Scheme: scheme,
	Metrics: metricsserver.Options{
		BindAddress:    ":8443",
		SecureServing:  true,
		FilterProvider: filters.WithAuthenticationAndAuthorization, // sigs.k8s.io/controller-runtime/pkg/metrics/filters
	},
	HealthProbeBindAddress: ":8081",
	LeaderElection:         true,
	LeaderElectionID:       "rediscluster.cache.example.com",
	Cache:                  cache.Options{DefaultTransform: cache.TransformStripManagedFields()},
})
if err != nil {
	os.Exit(1)
}
if err := (&controller.RedisClusterReconciler{
	Client:   mgr.GetClient(),
	Scheme:   mgr.GetScheme(),
	Recorder: mgr.GetEventRecorderFor("rediscluster-controller"),
}).SetupWithManager(mgr); err != nil {
	os.Exit(1)
}
```

- Serve metrics with `SecureServing: true` and the authn/authz `FilterProvider` — plain-HTTP `:8080` metrics endpoints are a review finding. The filter needs RBAC for `tokenreviews` and `subjectaccessreviews` `create`. Importing `pkg/metrics/filters` pulls in `k8s.io/apiserver`; run `go mod tidy` (verified: without it the build fails with `missing go.sum entry for module providing package k8s.io/apiserver/...`).
- `LeaderElection: true` for any controller that can run with more than one replica.
- `MaxConcurrentReconciles` > 1 is safe only if the reconciler holds no shared mutable state without locking; the queue guarantees a single key is never processed concurrently.
- `cache.Options.SyncPeriod` is the only place for a resync period in v0.20 (the old top-level manager option was removed). Do not set a short period as a substitute for proper watches.

---

#### Server-Side Apply from Controllers

Server-side apply (SSA) with a stable field manager is the preferred way for a controller to own child objects: it sends only the fields the controller cares about, merges with fields set by other actors (HPA replicas, mutating webhooks, users), and resolves conflicts per field. **controller-runtime v0.20 has no typed `Apply` method** (`client.Client.Apply(ctx, applyConfiguration, ...)` arrives in v0.22). On v0.20, SSA means `Patch(ctx, obj, client.Apply, ...)`. Build the object from client-go **apply configurations** and convert it to unstructured, so that only the fields you set are sent:

```go
import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	appsv1ac "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
)

func (r *RedisClusterReconciler) applyDeployment(ctx context.Context, rc *cachev1.RedisCluster) error {
	labels := map[string]string{
		"app.kubernetes.io/name":       "redis",
		"app.kubernetes.io/instance":   rc.Name,
		"app.kubernetes.io/managed-by": "rediscluster-controller",
	}
	gvk, err := r.GroupVersionKindFor(rc)
	if err != nil {
		return err
	}
	dep := appsv1ac.Deployment(rc.Name, rc.Namespace). // sets apiVersion/kind
		WithLabels(labels).
		WithOwnerReferences(metav1ac.OwnerReference().
			WithAPIVersion(gvk.GroupVersion().String()).
			WithKind(gvk.Kind).
			WithName(rc.Name).
			WithUID(rc.UID).
			WithController(true).
			WithBlockOwnerDeletion(true)).
		WithSpec(appsv1ac.DeploymentSpec().
			WithReplicas(rc.Spec.Replicas).
			WithSelector(metav1ac.LabelSelector().WithMatchLabels(labels)).
			WithTemplate(corev1ac.PodTemplateSpec().
				WithLabels(labels).
				WithSpec(corev1ac.PodSpec().
					WithAutomountServiceAccountToken(false).
					WithContainers(corev1ac.Container().
						WithName("redis").
						WithImage(rc.Spec.Image).
						WithSecurityContext(corev1ac.SecurityContext().
							WithAllowPrivilegeEscalation(false).
							WithReadOnlyRootFilesystem(true).
							WithRunAsNonRoot(true).
							WithCapabilities(corev1ac.Capabilities().WithDrop("ALL")).
							WithSeccompProfile(corev1ac.SeccompProfile().WithType(corev1.SeccompProfileTypeRuntimeDefault)))))))

	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dep)
	if err != nil {
		return err
	}
	return r.Patch(ctx, &unstructured.Unstructured{Object: content}, client.Apply, fieldOwner, client.ForceOwnership)
}
```

Verified in envtest on 1.32: the Deployment is created with one controller owner reference and `managedFields[0]` = manager `rediscluster-controller`, operation `Apply`; a second reconcile is a no-op.

SSA review rules:

- **Always send the complete desired intent** of the fields you own. A field omitted from a later apply is *removed* if this manager was its only owner. Building the apply payload conditionally ("only set replicas if changed") is a bug.
- **Do not apply typed structs.** `r.Patch(ctx, &appsv1.Deployment{...}, client.Apply, ...)` serialises zero values (`replicas: 0` when the pointer is set, `strategy: {}`, `creationTimestamp: null`) and takes ownership of fields you never meant to own; it also fails unless `TypeMeta` (`apiVersion`/`kind`) is filled in. Use apply configurations (as above) or a hand-built `unstructured.Unstructured` with `apiVersion` and `kind`.
- Do not recommend `r.Apply(ctx, ac, ...)` or `r.Status().Apply(...)` for a v0.20 project — verified: `c.Apply undefined (type client.Client has no field or method Apply)`.
- **One field manager per controller** (`client.FieldOwner`), constant across releases. Changing it orphans ownership and leaves stale fields behind.
- `client.ForceOwnership` is appropriate for a controller that is the source of truth for those fields; without it, a conflict (another manager set a different value) returns `409 Conflict`. Never force-apply fields that users are expected to tune (e.g. `replicas` when an HPA targets the Deployment) — omit them from the apply configuration instead.
- Do not mix SSA and `Update` on the same object from the same controller; `Update` takes ownership of every field (manager with operation `Update`) and later applies then fight the old manager entry.
- **Unit tests**: the controller-runtime v0.20 fake client does not implement apply patches (see Testing); SSA code paths need envtest.

---

#### Watches & Predicates

##### Use Predicates to Reduce Noise

```go
// Only reconcile when spec actually changes (generation increment) or annotations change
ctrl.NewControllerManagedBy(mgr).
    For(&cachev1.RedisCluster{},
        builder.WithPredicates(predicate.Or(
            predicate.GenerationChangedPredicate{},
            predicate.AnnotationChangedPredicate{},
        )),
    ).
    Complete(r)
```

Note: `GenerationChangedPredicate` ignores metadata-only updates (label, annotation and finalizer edits). It does not hide deletions of objects with finalizers, because setting `deletionTimestamp` bumps `metadata.generation`. If the controller must react to label or finalizer changes, combine predicates (`predicate.Or`) or write a custom one, as below.

##### Custom Predicate Example

```go
var specOrDeletion = predicate.Funcs{
    UpdateFunc: func(e event.UpdateEvent) bool {
        return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration() ||
            !e.ObjectNew.GetDeletionTimestamp().IsZero() ||
            !reflect.DeepEqual(e.ObjectNew.GetFinalizers(), e.ObjectOld.GetFinalizers())
    },
    CreateFunc:  func(e event.CreateEvent) bool { return true },
    DeleteFunc:  func(e event.DeleteEvent) bool { return true },
    GenericFunc: func(e event.GenericEvent) bool { return false },
}
```

controller-runtime v0.19+ also has typed variants (`predicate.TypedFuncs[*cachev1.RedisCluster]`, `handler.TypedEnqueueRequestsFromMapFunc`, `builder.TypedControllerManagedBy[request]`) that avoid type assertions; both styles are valid on v0.20.

##### Enqueue Requests for Owned Objects

```go
// Watch Deployments whose controller owner is a RedisCluster; reconcile the owner
Owns(&appsv1.Deployment{})
```

##### Fan-Out Watches (Many-to-One)

```go
// Watch a shared Secret; enqueue all CRs referencing it
Watches(
    &corev1.Secret{},
    handler.EnqueueRequestsFromMapFunc(r.findCRsForSecret),
    builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
)
```

`findCRsForSecret` must use a **field index** (`mgr.GetFieldIndexer().IndexField(...)` + `client.MatchingFields`) rather than listing every CR in the cluster on each Secret event. Watching all Secrets cluster-wide also caches all Secrets in memory — restrict the cache (`cache.Options.ByObject` with a label selector) or use `builder.OnlyMetadata`.

**[NEW in controller-runtime v0.20]** `cache.Options` gains `DefaultEnableWatchBookmarks` (default true; per type via `ByObject[...].EnableWatchBookmarks`; the option was also backported to later v0.19.x patches, e.g. v0.19.7, but is absent from v0.19.0) and an exported `NewInformer` hook; the fake client can add indexes at runtime with the package function `fake.AddIndex(c, obj, field, extractValue)` (the builder's `WithIndex` predates v0.20) and supports the `serviceaccounts/token` subresource. `controllerutil.HasOwnerReference` is also new in v0.20.

---

#### Finalizers

##### When to Use

Use finalizers **only** when the controller manages resources outside of Kubernetes (cloud APIs, databases, DNS records) or cluster-scoped/cross-namespace objects that owner references cannot cover.

> Do **not** use finalizers for in-cluster resources already covered by `ownerReferences`.

##### Naming Rule

Finalizer names **must** be fully qualified:

```
example.com/redis-external-cleanup    ✅
cleanup                               ❌  (not qualified)
```

Verified on 1.32: on a custom resource, `finalizers: [cleanup]` is accepted with `Warning: metadata.finalizers: "cleanup": prefer a domain-qualified finalizer name to avoid accidental conflicts with other finalizer writers`; on built-in types (e.g. a ConfigMap) it is rejected: `name is neither a standard finalizer name nor is it fully qualified`.

##### Rules

- Add the finalizer **before** creating any external resource; remove it only after cleanup succeeded.
- Cleanup logic **must be idempotent** — the reconcile loop may be called multiple times while `DeletionTimestamp` is set.
- Never add a new finalizer after `DeletionTimestamp` is set — only removal is allowed.
- Do not silently swallow errors; failing cleanup should be retried (return the error) and surfaced as a condition + Warning Event.
- Do not create resources inside a namespace that has `DeletionTimestamp` set.
- Removing the finalizer when the object is already gone returns NotFound — wrap with `client.IgnoreNotFound`.
- On 1.32 the alpha `AllowUnsafeMalformedObjectDeletion` delete option (`ignoreStoreReadErrorWithClusterBreakingPotential`) **skips finalizers**; it is off by default and must never be part of normal tooling.

---

#### Owner References & Garbage Collection

##### Set Owner References for All Child Resources

```go
// controllerutil.SetControllerReference sets controller=true and blockOwnerDeletion=true
if err := controllerutil.SetControllerReference(owner, childDeployment, r.Scheme); err != nil {
    return err
}
```

With SSA, set the owner reference inside the apply configuration (as above) so the controller's field manager owns it. `controllerutil.HasOwnerReference(refs, obj, scheme)` is new in controller-runtime v0.20.

##### Rules

| Rule | Notes |
|---|---|
| Owner must be in **same namespace** as the owned resource | A namespaced dependent whose same-namespace owner is missing is treated as having an absent owner → deleted by GC |
| A namespaced owner **cannot** own a cluster-scoped resource | The owner reference is unresolvable — the dependent is **not** garbage collected and gets an `OwnerRefInvalidNamespace` event; `SetControllerReference` returns an error |
| A cluster-scoped owner **can** own a cluster-scoped resource | Permitted |
| Use `controllerutil.SetControllerReference` (not hand-built refs) | Sets `controller: true` and `blockOwnerDeletion: true`, refuses a second controller owner |
| One resource should have **at most one controller owner** | Multiple owners only for non-controller (informational) references |
| `blockOwnerDeletion: true` requires `update` on the owner's `finalizers` subresource when the `OwnerReferencesPermissionEnforcement` admission plugin is enabled (not in the default plugin set, common in hardened/OpenShift clusters) | Grant `<owner-resource>/finalizers` `update` — Kubebuilder's marker does |

Security note: before v1.32.8, a compromised kubelet could add an owner reference to its own Node object and get it garbage-collected (CVE-2025-5187, fixed by NodeRestriction in v1.32.8). Clusters on 1.32.0–1.32.7 are affected.

##### When to Use Finalizers vs. OwnerReferences

| Scenario | Use |
|---|---|
| Child resource in same namespace | `ownerReferences` (automatic GC) |
| Cluster-scoped child of a namespaced owner | `finalizer` on the owner (or use a cluster-scoped owner) |
| External resource (cloud, DNS) | `finalizer` |
| Cross-namespace cleanup needed | `finalizer` |

---

#### Events

controller-runtime v0.20's manager exposes only the **core/v1** recorder: `mgr.GetEventRecorderFor(name)` returns a `k8s.io/client-go/tools/record.EventRecorder` (`Event`, `Eventf`, `AnnotatedEventf`). The `events.k8s.io/v1` recorder (`mgr.GetEventRecorder`) arrives in v0.23 — verified: `mgr.GetEventRecorder undefined` on v0.20.

```go
// record.EventRecorder: object, type, reason, message format, args
r.Recorder.Eventf(rc, corev1.EventTypeWarning, "ApplyFailed", "%v", err)
r.Recorder.Eventf(rc, corev1.EventTypeNormal, "Available", "all %d replicas ready", rc.Spec.Replicas)
```

Review points:

- RBAC: the core recorder writes `events` in the core group — `// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch`. Missing RBAC does not fail reconciliation; the recorder only logs the error, so Events silently disappear.
- Code that writes `events.k8s.io/v1` Events directly must set `action`, `reportingController`, `reportingInstance` and `eventTime` (verified on 1.32: an Event without `action` is rejected with `action: Required value`).
- The message is a format string — never pass user-controlled text as the format.
- Emit Events on transitions (use the `changed` result of `meta.SetStatusCondition`), not on every reconcile.

---

#### Admission: CEL Policies and Webhooks

##### Validation Hierarchy (prefer left-to-right)

```
CRD schema + CEL rules  →  ValidatingAdmissionPolicy (CEL, GA 1.30)  →  Validating webhook
(per-CRD, no extra process)  (cluster policy, no extra process)          (extra infra, TLS, HA)
```

Mutation on 1.32: a **mutating webhook** is the only production mechanism. `MutatingAdmissionPolicy` is **new in 1.32 as alpha** (`admissionregistration.k8s.io/v1alpha1`) and is not served by default; it becomes beta `v1beta1` in 1.34 and GA `v1` in 1.36. Do not ship a chart that requires MutatingAdmissionPolicy to a stock 1.32 cluster.

##### ValidatingAdmissionPolicy (verified on 1.32)

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: require-digest-and-limits
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
    - apiGroups: ["apps"]
      apiVersions: ["v1"]
      operations: ["CREATE", "UPDATE"]
      resources: ["deployments", "statefulsets"]
  matchConditions:
  - name: not-system
    expression: "!request.userInfo.username.startsWith('system:')"
  variables:
  - name: containers
    expression: "object.spec.template.spec.containers + object.spec.template.spec.?initContainers.orValue([])"
  validations:
  - expression: "variables.containers.all(c, c.image.contains('@sha256:'))"
    messageExpression: "'all images must be pinned by digest: ' + variables.containers.filter(c, !c.image.contains('@sha256:')).map(c, c.name).join(', ')"
    reason: Invalid
  - expression: "variables.containers.all(c, has(c.resources) && has(c.resources.limits) && 'memory' in c.resources.limits)"
    message: every container needs a memory limit
  auditAnnotations:
  - key: unpinned-images
    valueExpression: "string(variables.containers.filter(c, !c.image.contains('@sha256:')).size())"
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: require-digest-and-limits
spec:
  policyName: require-digest-and-limits
  validationActions: [Deny, Audit]
  matchResources:
    namespaceSelector:
      matchExpressions:
      - {key: kubernetes.io/metadata.name, operator: NotIn, values: [kube-system]}
```

A Deployment whose init container uses a tag is rejected on 1.32 with:
`ValidatingAdmissionPolicy 'require-digest-and-limits' with binding 'require-digest-and-limits' denied request: all images must be pinned by digest: migrate`.

Review points for policies:

- A policy without a **binding** does nothing; a binding's `validationActions` decides `Deny`/`Warn`/`Audit`. `[Deny, Warn]` is rejected (verified: `must not contain both Deny and Warn`); roll out new policies with `[Warn, Audit]` first.
- Use `?`/`orValue` optional chaining (`object.spec.template.spec.?initContainers.orValue([])`) for optional fields; plain `object.spec.x` on an absent field is a runtime error that triggers `failurePolicy`.
- Use `matchConditions` to exclude system users and controllers, otherwise a broken policy can block kube-system controllers.
- `paramKind` + `paramRef` for per-namespace configuration; `paramRef.parameterNotFoundAction` is required — `Deny` subjects requests without a matching param to the policy's `failurePolicy`, `Allow` lets them through.
- **[NEW in 1.32]** Strict CEL cost enforcement for VAP and webhook `matchConditions` is always on (the `StrictCostEnforcementForVAP`/`StrictCostEnforcementForWebhooks` gates were promoted and locked): policies whose cost was only *estimated* leniently on 1.31 can now fail at runtime cost limits. Re-test expensive policies after upgrading.
- **[GA in 1.32]** Service-account tokens carry node information, so a policy can restrict what a node-bound identity may do, e.g. `!request.userInfo.extra[?'authentication.kubernetes.io/node-name'].hasValue()` (compiles in a 1.32 ValidatingAdmissionPolicy, verified).
- Before v1.32.11, VAP/MAP could fail with a spurious "namespace not found" for objects in newly created namespaces; fixed in v1.32.11.

##### MutatingAdmissionPolicy — alpha, only where the cluster opted in

Verified on a 1.32.0 apiserver started with `--feature-gates=MutatingAdmissionPolicy=true --runtime-config=admissionregistration.k8s.io/v1alpha1=true`: a Pod created in a namespace labelled `seccomp-default: "true"` gets `securityContext.seccompProfile.type: RuntimeDefault`. On a default 1.32 apiserver the kind does not exist.

```yaml
apiVersion: admissionregistration.k8s.io/v1alpha1
kind: MutatingAdmissionPolicy
metadata:
  name: default-seccomp
spec:
  failurePolicy: Fail
  reinvocationPolicy: IfNeeded
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: ["CREATE"]
      resources: ["pods"]
  matchConditions:
  - name: no-seccomp-set
    expression: "!has(object.spec.securityContext) || !has(object.spec.securityContext.seccompProfile)"
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{
          spec: Object.spec{
            securityContext: Object.spec.securityContext{
              seccompProfile: Object.spec.securityContext.seccompProfile{type: "RuntimeDefault"}
            }
          }
        }
---
apiVersion: admissionregistration.k8s.io/v1alpha1
kind: MutatingAdmissionPolicyBinding
metadata:
  name: default-seccomp
spec:
  policyName: default-seccomp
  matchResources:
    namespaceSelector:
      matchLabels: {seccomp-default: "true"}
```

Alpha APIs have no upgrade guarantee: a 1.34 cluster serves `v1beta1`, not `v1alpha1`, so the manifest must change on upgrade. Treat any `v1alpha1` MutatingAdmissionPolicy in a chart as a review finding unless the chart documents the cluster opt-in.

##### Admission webhooks (controller-runtime v0.20 API)

controller-runtime v0.20 uses the **`CustomValidator` / `CustomDefaulter`** interfaces with `runtime.Object` parameters and the builder `ctrl.NewWebhookManagedBy(mgr).For(obj)`. v0.20 **removed** the older `admission.Validator` / `admission.Defaulter` interfaces (methods on the API type itself).

```go
// RedisClusterValidator implements the v0.20 CustomValidator interface.
type RedisClusterValidator struct{}

var _ admission.CustomValidator = &RedisClusterValidator{}

func (v *RedisClusterValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	rc, ok := obj.(*cachev1.RedisCluster)
	if !ok {
		return nil, fmt.Errorf("expected a RedisCluster, got %T", obj)
	}
	if rc.Spec.Tier == cachev1.TierStandard && rc.Spec.Replicas > 5 {
		return admission.Warnings{"standard tier is not tuned for more than 5 replicas"}, nil
	}
	return nil, nil
}

func (v *RedisClusterValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldRC, ok1 := oldObj.(*cachev1.RedisCluster)
	newRC, ok2 := newObj.(*cachev1.RedisCluster)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("expected RedisCluster objects, got %T and %T", oldObj, newObj)
	}
	if oldRC.Spec.Tier == cachev1.TierPremium && newRC.Spec.Tier != cachev1.TierPremium {
		return nil, fmt.Errorf("downgrading from premium is not supported")
	}
	return v.ValidateCreate(ctx, newRC)
}

func (v *RedisClusterValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&cachev1.RedisCluster{}).
		WithValidator(&RedisClusterValidator{}).
		Complete()
}
```

```go
// BAD on v0.20 — each line fails to compile (verified):
var _ admission.Validator = nil                              // undefined: admission.Validator (removed in v0.20)
ctrl.NewWebhookManagedBy(mgr, &cachev1.RedisCluster{})       // too many arguments (generic form is v0.23)
```

- Always check the type assertion in `CustomValidator`/`CustomDefaulter` methods; an unchecked `obj.(*T)` panics the webhook server when the webhook is accidentally registered for a second kind.
- **[CHANGED in v0.20]** `CustomDefaulter` no longer deletes unknown fields from the object by default (v0.20.0: "Stop deleting unknown fields in CustomDefaulter"); pass `admission.DefaulterRemoveUnknownOrOmitableFields` to `WithDefaulter` only if you really want fields that your Go type does not know to be pruned. An operator built against an older API version than the installed CRD would otherwise silently drop the newer fields on every request.
- The tier-downgrade check is a simple transition rule (`self.tier == oldSelf.tier || oldSelf.tier != 'premium'`) and belongs in CEL; keep webhooks for checks that need external state.

##### Webhook configuration rules

```go
// +kubebuilder:webhook:path=/validate-cache-example-com-v1-rediscluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=cache.example.com,resources=redisclusters,verbs=create;update,versions=v1,name=vrediscluster.kb.io,admissionReviewVersions=v1
```

- Set `failurePolicy: Fail` for critical validation; `Ignore` only if the webhook is non-essential.
- `sideEffects: None` (or `NoneOnDryRun`); webhooks with side effects break `kubectl apply --dry-run=server`.
- `matchPolicy: Equivalent` (the `v1` default) so the webhook fires on all API versions of the same resource.
- Handle `CREATE` **and** `UPDATE` for immutability rules; `DELETE` only for deletion guards.
- `timeoutSeconds` defaults to 10 and may not exceed 30; keep webhooks fast.
- Use `matchConditions` (GA since 1.30) to skip irrelevant requests in-process, and `namespaceSelector`/`objectSelector` to exclude `kube-system` and the webhook's own namespace (a fail-closed webhook that matches its own Pods can deadlock the cluster).
- TLS certificates **must** be rotated; use cert-manager or a managed CA bundle injection (controller-runtime v0.20's cert watcher also polls for changes).

---

#### RBAC, Authentication & Least Privilege

##### Principles

1. **Least privilege** — grant only the verbs/resources explicitly required.
2. Prefer `Role` + `RoleBinding` (namespaced) over `ClusterRole` + `ClusterRoleBinding`.
3. **No wildcard verbs** (`*`) or **wildcard resources** (`*`).
4. **No `cluster-admin`** service accounts.
5. Mount service account tokens only when the workload needs API access (`automountServiceAccountToken: false` as default).
6. Treat `escalate`, `bind`, `impersonate`, `nodes/proxy`, `serviceaccounts/token` `create`, and `pods/exec` as privileged.

##### `pods/exec`, `pods/attach`, `pods/portforward` on 1.32

On 1.32 a WebSocket `GET` upgrade request for these subresources is authorized as **`get`**, while SPDY `POST` requests need `create` (kubectl 1.32 uses WebSockets for exec/attach/port-forward by default). A role that grants only `get` on `pods/exec` therefore still allows exec over WebSockets on 1.32 — and stops working on 1.35, where `AuthorizePodWebsocketUpgradeCreatePermission` requires `create` for both. Verified on 1.32 with a user bound to `verbs: [get]` on `pods/exec`:

```text
WebSocket GET upgrade:  "pod web does not have a host assigned"   (authorization passed; failed later)
SPDY-style POST:        pods "web" is forbidden: User "bob" cannot create resource "pods/exec" ...
no RBAC at all:         pods "web" is forbidden: User "alice" cannot get resource "pods/exec" ...
```

Review rules:

```yaml
# Write exec permissions as create (works on 1.32 and later):
rules:
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["create"]
```

- Treat `get` on `pods/exec`/`pods/attach`/`pods/portforward` as **exec permission** on 1.32 when auditing "read-only" roles.
- v1.32.3 fixed a 1.30+ regression in connection stability for WebSocket exec/attach/port-forward; tooling on older patch levels may see dropped sessions.

##### **[NEW in 1.32 — alpha]** Kubelet fine-grained authorization

`nodes/healthz`, `nodes/pods` and `nodes/configz` exist only with `KubeletFineGrainedAuthz` (alpha, off). On 1.32, monitoring agents that read the kubelet `/pods` or `/configz` endpoints still need `nodes/proxy`, which also allows exec into every container on the node. Flag `nodes/proxy` grants, document why they are needed, and plan to switch to the fine-grained subresources after upgrading (beta-on in 1.33).

##### **[NEW in 1.32 — beta, on]** Selector-aware authorization

- `AuthorizeNodeWithSelectors`: the node authorizer only lets a kubelet credential get/list/watch **its own** Node and the Pods bound to it. DaemonSets or tools that (ab)use kubelet client certificates to list all Pods or Nodes break on 1.32 — give them their own ServiceAccount and RBAC.
- `AuthorizeWithSelectors`: webhook authorizers receive the label/field selectors of list/watch requests, so an authorizer can allow "list Pods with `spec.nodeName=<x>`" without allowing an unrestricted list.

##### **[GA in 1.32]** Structured authorization configuration

```yaml
# --authorization-config=/etc/kubernetes/authz.yaml (kube-apiserver flag; mutually exclusive with --authorization-mode)
apiVersion: apiserver.config.k8s.io/v1
kind: AuthorizationConfiguration
authorizers:
- type: Node
  name: node
- type: RBAC
  name: rbac
- type: Webhook
  name: policy-engine
  webhook:
    timeout: 3s
    subjectAccessReviewVersion: v1
    matchConditionSubjectAccessReviewVersion: v1
    failurePolicy: NoOpinion
    connectionInfo:
      type: KubeConfigFile
      kubeConfigFile: /etc/kubernetes/authz-webhook.kubeconfig
    matchConditions:
    - expression: has(request.resourceAttributes)
    - expression: "!request.user.startsWith('system:')"
```

`apiserver.config.k8s.io/v1` is identical to `v1beta1`; prefer `v1` in 1.32 cluster configuration. Put `Node` and `RBAC` before webhooks; use `failurePolicy: NoOpinion` unless the webhook must deny on failure, and `matchConditions` so system components never depend on the webhook.

##### **[NEW in 1.32 — beta, on]** Anonymous access limited to configured endpoints

With `AnonymousAuthConfigurableEndpoints`, the structured `AuthenticationConfiguration` (`--authentication-config`, `apiserver.config.k8s.io/v1beta1` on 1.32) can allow anonymous requests only for listed paths:

```yaml
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
anonymous:
  enabled: true
  conditions:
  - path: /livez
  - path: /readyz
  - path: /healthz
```

Verified on a 1.32.0 apiserver started with this file: anonymous `/livez` returns 200, while `/version` and `/api` (normally readable anonymously through the default `system:public-info-viewer` and discovery bindings) return 401 — clients or probes that call those paths unauthenticated break. This prevents an RBAC mistake (a binding to `system:anonymous` or `system:unauthenticated`) from exposing the API. Flag any RoleBinding/ClusterRoleBinding whose subject is `system:anonymous` or `system:unauthenticated` beyond the default health/discovery bindings.

##### RBAC Marker Pattern (Kubebuilder)

```go
// Core resources the operator reads
// +kubebuilder:rbac:groups="",resources=pods;services;configmaps,verbs=get;list;watch

// CRD the operator owns
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/finalizers,verbs=update

// Apps it manages
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete

// Events: controller-runtime v0.20 recorder writes core/v1 Events
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Leader election
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
```

Secrets: grant `get` on specific `resourceNames` where possible; `list`/`watch` on Secrets cluster-wide is equivalent to reading every Secret.

##### Service Account Configuration

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: redis-operator
  namespace: redis-system
automountServiceAccountToken: false  # Opt-in only
---
apiVersion: v1
kind: Pod
spec:
  serviceAccountName: redis-operator
  automountServiceAccountToken: true  # Explicitly opt in at pod level
```

- **[NEW in 1.32]** Do not add `kubernetes.io/enforce-mountable-secrets: "true"` to new ServiceAccounts: it is deprecated and produces a warning on every create/update (verified). Isolate secrets by namespace.
- **[GA in 1.32]** Projected SA tokens include the node name and a unique `jti`; audit logs record `authentication.kubernetes.io/credential-id`. Use these for incident response instead of logging token contents.
- `kubectl create token <sa> --bound-object-kind Node --bound-object-name <node>` (node-bound tokens, `ServiceAccountTokenNodeBinding` beta-on in 1.32) produces tokens that become invalid when the Node object is deleted (verified on 1.32: the token carries `kubernetes.io.node.{name,uid}` and a `jti`; after `kubectl delete node` the same token gets `401 Unauthorized`).

---

#### Workload Manifests: Pods

##### A production Deployment for 1.32 (verified)

Applied to a 1.32.0 apiserver into a namespace enforcing Pod Security `restricted:v1.32`; a Pod built from the template passes admission.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: payments
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: v1.32
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/audit: restricted
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: api
  namespace: payments
automountServiceAccountToken: false
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: payments
  labels: {app.kubernetes.io/name: api, app.kubernetes.io/part-of: payments}
spec:
  replicas: 3
  revisionHistoryLimit: 5
  selector:
    matchLabels: {app.kubernetes.io/name: api}
  strategy:
    type: RollingUpdate
    rollingUpdate: {maxUnavailable: 0, maxSurge: 1}
  template:
    metadata:
      labels: {app.kubernetes.io/name: api, app.kubernetes.io/part-of: payments}
    spec:
      serviceAccountName: api
      automountServiceAccountToken: false
      terminationGracePeriodSeconds: 45
      securityContext:
        runAsNonRoot: true
        runAsUser: 10001
        runAsGroup: 10001
        fsGroup: 10001
        seccompProfile: {type: RuntimeDefault}
      topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: ScheduleAnyway
        labelSelector:
          matchLabels: {app.kubernetes.io/name: api}
        matchLabelKeys: [pod-template-hash]
      initContainers:
      - name: log-shipper                      # native sidecar (beta, on by default in 1.32)
        image: registry.example.com/log-shipper@sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
        restartPolicy: Always
        resources:
          requests: {cpu: 20m, memory: 32Mi}
          limits: {memory: 64Mi}
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities: {drop: [ALL]}
      containers:
      - name: api
        image: registry.example.com/payments-api@sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
        ports: [{name: http, containerPort: 8080}]
        resources:
          requests: {cpu: 250m, memory: 256Mi}
          limits: {cpu: "1", memory: 256Mi}
        startupProbe:
          httpGet: {path: /healthz, port: http}
          failureThreshold: 30
          periodSeconds: 2
        readinessProbe:
          httpGet: {path: /readyz, port: http}
          periodSeconds: 5
        livenessProbe:
          httpGet: {path: /healthz, port: http}
          periodSeconds: 10
          failureThreshold: 3
        lifecycle:
          preStop:
            sleep: {seconds: 5}                # PodLifecycleSleepAction, beta-on; seconds >= 1 on 1.32
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities: {drop: [ALL]}
        volumeMounts:
        - {name: tmp, mountPath: /tmp}
        - {name: config, mountPath: /etc/api, readOnly: true, recursiveReadOnly: IfPossible}
      volumes:
      - name: tmp
        emptyDir: {medium: Memory, sizeLimit: 64Mi}   # counted against the container memory limit
      - name: config
        configMap: {name: api-config}
```

A bare Pod in the same namespace without a security context is rejected: `violates PodSecurity "restricted:v1.32": allowPrivilegeEscalation != false ..., unrestricted capabilities ..., runAsNonRoot != true ..., seccompProfile ...`.

Review notes on this template:

- **Sidecars** (`initContainers[].restartPolicy: Always`) are beta and on by default since 1.29 and work on 1.32; they become GA in 1.33. A cluster that disabled `SidecarContainers` does not reject the field but **silently drops** `initContainers[].restartPolicy` (verified on a 1.32.0 apiserver with `--feature-gates=SidecarContainers=false`): the sidecar becomes an ordinary init container that never exits, so the main containers never start. Check cluster config for managed platforms that pin gates.
- `matchLabelKeys: [pod-template-hash]` spreads each ReplicaSet revision independently. On 1.32 the apiserver does **not** merge `matchLabelKeys` into the topology-spread `labelSelector` (verified: the stored Pod keeps `labelSelector` unchanged; the scheduler applies the keys). For pod (anti-)affinity, 1.32 **does** merge them into `labelSelector.matchExpressions` at creation (verified: `{key: pod-template-hash, operator: In, values: [abc]}` is added). Tools that diff live Pods against templates must expect this.
- **[GA in 1.32]** Memory-backed `emptyDir` volumes are sized to the Pod's memory limit (or `sizeLimit` when smaller) and their usage counts as container memory — a 64Mi tmpfs inside a 256Mi container leaves 192Mi for the process. Always set `sizeLimit` on `medium: Memory`.
- `recursiveReadOnly: IfPossible` (beta, on) makes read-only mounts read-only for sub-mounts too where the runtime supports it; `Enabled` fails the Pod when unsupported. Requires `readOnly: true` (and `mountPropagation: None`).
- `memory` requests = limits avoids OOM surprises for latency-critical services; `cpu` limits are a policy decision (throttling) — flag missing **memory** limits, not missing CPU limits.
- `GOMAXPROCS`: Go ≤ 1.24 binaries do not read the cgroup CPU limit; set `GOMAXPROCS` from `resourceFieldRef: {resource: limits.cpu, divisor: "1"}` or use `go.uber.org/automaxprocs` (Go 1.25 made the runtime container-aware).
- `maxUnavailable: 0` + `maxSurge: 1` with a PDB of `minAvailable: 2` and three replicas keeps two Pods serving during rollouts and node drains.

##### Features that a 1.32 apiserver silently drops (verified)

All of the following are accepted with `rc=0` by a default 1.32 apiserver, but the field is **not stored** — the workload runs without it:

| Field | Gate on 1.32 | Behaviour on 1.32 | Usable from |
|---|---|---|---|
| `spec.resources` (pod-level) | `PodLevelResources` alpha | dropped (1.31 rejects it as `unknown field "spec.resources"`) | beta-on 1.34 |
| `spec.hostUsers: false` | `UserNamespacesSupport` beta/**off** | dropped | on by default 1.33 |
| `securityContext.supplementalGroupsPolicy` | alpha | dropped | beta-on 1.33 |
| `containers[].resizePolicy` | `InPlacePodVerticalScaling` alpha | dropped | beta-on 1.33 |
| `volumes[].image` | `ImageVolume` alpha | **the volume and the `volumeMount` that references it are both removed** | on by default 1.35 |
| StatefulSet `updateStrategy.rollingUpdate.maxUnavailable` | `MaxUnavailableStatefulSet` alpha | dropped | on only in v1.35.0–v1.35.3 (off again from v1.35.4 and in 1.36); on by default in 1.37 |

The image-volume case is the dangerous one: the container starts without the mount and reads an empty directory path instead of failing. Review rule: a 1.32 manifest must not contain these fields unless the cluster configuration enables the gate; CI must run `kubectl apply --dry-run=server -o yaml` against a 1.32 apiserver and diff the output.

##### Pod lifecycle and preStop

- `lifecycle.preStop.sleep` (beta, on since 1.30) replaces `exec: ["sleep","5"]` in images without a shell. `seconds` must be ≥ 1 and less than `terminationGracePeriodSeconds`; `seconds: 0` is rejected on 1.32 (verified; allowed from 1.33).
- v1.32.0–v1.32.2 had a kubelet regression that broke starting Pods with `postStart` hooks, fixed in v1.32.3. Flag `postStart` usage on clusters below v1.32.3.
- **Pod generation**: 1.32 Pods have no `metadata.generation`; in-place resize is alpha. Changing `resources` in a Deployment/StatefulSet template triggers a rollout; direct Pod resource edits are rejected (`pod updates may not change fields other than ...`).

##### In-place Pod resize — alpha on 1.32

The `pods/resize` subresource exists in the 1.32 code base, but the gate is alpha/off. Verified on a default 1.32 apiserver: `kubectl patch pod web --subresource resize -p ...` → `Error from server (NotFound): the server could not find the requested resource`. With `--feature-gates=InPlacePodVerticalScaling=true` the same strategic-merge patch updates `requests.cpu`/`limits.cpu` (verified). With the gate on, the 1.32 apiserver also accepts a memory-limit *decrease* with the default `resizePolicy` (verified); 1.33 (beta) rejects such decreases unless the container's memory `resizePolicy` is `RestartContainer`, and 1.34 allows them again (kubernetes#133012). Removing requests or limits through `resize` is rejected on 1.32 (`resource limits cannot be removed`, verified). Never ship autoscaler or operator code that depends on resize to a stock 1.32 cluster; in 1.32 alpha, resize is not supported for Windows Pods, and Guaranteed Pods with integer CPU on static CPU/memory-manager nodes are blocked (`InPlacePodVerticalScalingExclusiveCPUs` off).

##### Other Pod rules

- **AppArmor**: use `securityContext.appArmorProfile` (GA 1.31). Verified on 1.32: the apiserver still mirrors the field into the deprecated `container.apparmor.security.beta.kubernetes.io/<container>` annotation; do not write the annotation by hand and do not let tools strip it as drift.
- **User namespaces** (`hostUsers: false`) are beta but **off** by default on 1.32 — the field is dropped. Do not claim the workload runs in a user namespace unless the cluster enables `UserNamespacesSupport`.
- **DNS**: search domains must be RFC 1123 subdomains on 1.32; `.` and `_srv`-style entries are rejected (relaxed in 1.33).
- **Sysctls**: under Pod Security `baseline`/`restricted` at `v1.32`, `net.ipv4.tcp_rmem` and `net.ipv4.tcp_wmem` are allowed (namespaced since kernel 4.15). Pin `enforce-version` so a `v1.31` pin keeps rejecting them, as verified.
- **Image pull errors**: since 1.32 the actual registry error is appended to `state.waiting.message` for `ImagePullBackOff`; alerting that matches the exact old text `Back-off pulling image` must use `reason` instead.
- **`gitRepo` volumes**: deprecated, a known node-root RCE vector; still mounted by 1.32 kubelets. Flag every use; replace with an init container or git-sync.
- **Sidecar OOM score**: 1.32 aligns the sidecar `oom_score_adj` with the regular containers, so a memory-hungry sidecar no longer survives an OOM at the expense of the main container.

---

#### Workload Manifests: Controllers (Deployment, StatefulSet, Job, CronJob, HPA)

##### Deployments and ReplicaSets

- `spec.selector` is immutable; changing labels in the selector requires a new Deployment.
- `progressDeadlineSeconds` only marks the `Progressing` condition `False`; it does not roll back.
- `status.terminatingReplicas` does **not** exist on 1.32 (alpha 1.33). Rollout tooling must not wait on it.
- **[CHANGED in 1.32]** The `ScalingReplicaSet` Event message reads `Scaled up replica set web-5d4f from 1 to 3` (old: `... to 3 from 1`); regex-based dashboards break.

##### StatefulSets — **[GA in 1.32]** PVC retention policy

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: kv
spec:
  serviceName: kv
  replicas: 3
  selector: {matchLabels: {app: kv}}
  persistentVolumeClaimRetentionPolicy:
    whenDeleted: Delete        # delete PVCs when the StatefulSet is deleted
    whenScaled: Retain         # keep PVCs of scaled-down replicas
  template:
    metadata: {labels: {app: kv}}
    spec:
      containers:
      - name: kv
        image: registry.example.com/kv:3.1
  volumeClaimTemplates:
  - metadata: {name: data}
    spec:
      accessModes: [ReadWriteOnce]
      resources: {requests: {storage: 10Gi}}
```

- The default is `Retain`/`Retain`. `whenDeleted: Delete` deletes data on `kubectl delete sts` — set it only for disposable data (caches, CI), and make it explicit in review.
- The PVCs get an owner reference to the StatefulSet (or Pod for `whenScaled: Delete`); `kubectl delete sts --cascade=orphan` leaves the PVCs alone.
- `rollingUpdate.maxUnavailable` is alpha/off and silently dropped (verified) — updates are one Pod at a time.
- `ordinals.start` (GA 1.31) for slicing a StatefulSet across clusters/migrations; `apps.kubernetes.io/pod-index` label (GA 1.32) for selecting one replica.
- `serviceName` must reference an existing headless Service for stable DNS.
- v1.32.1 fixed a kube-controller-manager panic for StatefulSets with a negative `revisionHistoryLimit`.

##### Jobs

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: shard-train
spec:
  completionMode: Indexed
  completions: 8
  parallelism: 8
  backoffLimitPerIndex: 2        # beta, on
  maxFailedIndexes: 2
  podReplacementPolicy: Failed   # beta, on
  successPolicy:                 # beta, on
    rules:
    - succeededIndexes: "0"
  podFailurePolicy:              # GA 1.31
    rules:
    - action: FailJob
      onExitCodes: {containerName: main, operator: In, values: [3]}
    - action: Ignore
      onPodConditions: [{type: DisruptionTarget}]
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: main
        image: registry.example.com/train:1.0
```

Accepted by both 1.31 and 1.32 (verified).

- **[NEW in 1.32 — beta, on]** **`managedBy`**: verified, 1.32 stores `spec.managedBy: kueue.x-k8s.io/multikueue`; 1.31 silently drops it. When set to anything other than `kubernetes.io/job-controller`, the built-in Job controller ignores the Job entirely — no Pods are created unless the named controller exists. The field is immutable. Flag `managedBy` in manifests that are not part of a MultiKueue (or similar) setup, and remember that a 1.31 cluster would run such a Job *locally*.
- `podFailurePolicy` requires `restartPolicy: Never`; `backoffLimitPerIndex` requires `completionMode: Indexed`.
- The Job controller counts sidecar restarts when removing Pods (1.32); sidecars in Jobs must still be `initContainers` with `restartPolicy: Always`, not regular containers, or the Job never completes.
- v1.32.7 fixed Jobs with `suspend: true` and `completions: 0` not getting the `Complete` condition, and a delay in creating Pods for new Jobs; v1.32.10 fixed status updates failing with `startTime cannot be removed for unsuspended job`.

##### CronJobs

- Always set `timeZone` (GA 1.27) and `concurrencyPolicy` explicitly; `startingDeadlineSeconds` avoids catch-up storms after controller downtime.
- **[GA in 1.32]** Use the `batch.kubernetes.io/cronjob-scheduled-timestamp` annotation on the created Job to know which schedule slot a run belongs to.

##### HorizontalPodAutoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api
spec:
  scaleTargetRef: {apiVersion: apps/v1, kind: Deployment, name: api}
  minReplicas: 2
  maxReplicas: 20
  metrics:
  - type: Resource
    resource: {name: cpu, target: {type: Utilization, averageUtilization: 70}}
  - type: ContainerResource            # GA since 1.30; gate removed in 1.32
    containerResource: {name: memory, container: api, target: {type: Utilization, averageUtilization: 80}}
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
```

- `behavior.scaleUp/scaleDown.tolerance` does **not** exist on 1.32 (verified: `strict decoding error: unknown field "spec.behavior.scaleUp.tolerance"`); the global 10% tolerance applies.
- `minReplicas: 0` needs the alpha `HPAScaleToZero` gate (off): verified, 1.32 rejects it with `spec.minReplicas: Invalid value: 0: must be greater than or equal to 1` (and, even with the gate, requires an Object or External metric).
- Prefer `ContainerResource` metrics when sidecars distort Pod-level utilization.
- Do not set `spec.replicas` in a Deployment managed by an HPA when applying with SSA or `kubectl apply` — every apply resets the replica count. Omit the field.

---

#### Networking: Services & NetworkPolicy

##### `trafficDistribution` on 1.32

```yaml
apiVersion: v1
kind: Service
metadata:
  name: zonal-api
spec:
  selector: {app: api}
  ports: [{name: http, port: 80, targetPort: 8080}]
  trafficDistribution: PreferClose     # beta, on (1.31+); the only accepted value on 1.32
```

- `PreferClose` is the correct spelling on 1.32. `PreferSameZone`/`PreferSameNode` are rejected (verified: `Unsupported value: "PreferSameZone": supported values: "PreferClose"`) — they arrive in 1.34 (beta) and `PreferClose` is deprecated only from 1.35. A chart that must support 1.32 keeps `PreferClose`.
- These are *preferences*: kube-proxy falls back to remote endpoints when no zonal endpoint is ready.
- Do not combine `trafficDistribution` with the `service.kubernetes.io/topology-mode: Auto` annotation.
- `internalTrafficPolicy: Local` is a hard requirement (no fallback), not a preference — flag it where availability matters.

##### **[GA in 1.32]** LoadBalancer `ipMode`

Cloud-controller-managers and LB controllers set `status.loadBalancer.ingress[].ipMode`:

- `VIP` (default when unset): kube-proxy short-circuits traffic to the LB IP inside the cluster.
- `Proxy`: traffic to the LB IP is sent to the load balancer (needed when the LB terminates PROXY protocol or TLS).

Verified on 1.32: the status subresource accepts `{"ip":"203.0.113.10","ipMode":"Proxy"}` and rejects `ipMode` on a hostname-only ingress (`ipMode: Forbidden: may not be specified when 'ip' is not set`). LB controllers written for 1.32 must set `ipMode` explicitly when they proxy.

##### Service names, IPs and `externalIPs`

- Service names must be RFC 1035 labels on 1.32 (no leading digit) — see Naming.
- Use canonical IP literals (no leading zeros); later releases warn (1.33+) and reject (1.36) non-canonical forms.
- `spec.externalIPs` lets any user who can create Services intercept traffic for arbitrary IPs (CVE-2020-8554). Clusters should block it with a ValidatingAdmissionPolicy (`!has(object.spec.externalIPs)`) or the `DenyServiceExternalIPs` admission plugin; new manifests should use `LoadBalancer`, `NodePort` or Gateway API.
- Multiple Service CIDRs (`networking.k8s.io/v1beta1` ServiceCIDR) are beta/**off** on 1.32; do not manage ServiceCIDR objects in 1.32 charts.

##### Ingress and Gateway API

New HTTP routing should target Gateway API (`gateway.networking.k8s.io/v1`, installed as CRDs). Ingress NGINX was retired by the Kubernetes project (best-effort maintenance ended March 2026); flag new `nginx.ingress.kubernetes.io/*-snippet` annotations, which inject raw NGINX configuration.

##### NetworkPolicy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: api-ingress
  namespace: payments
spec:
  podSelector:
    matchLabels: {app.kubernetes.io/name: api}
  policyTypes: [Ingress]
  ingress:
  - from:
    - namespaceSelector:
        matchLabels: {kubernetes.io/metadata.name: gateway}
    ports: [{port: http, protocol: TCP}]   # named container port
```

- Every namespace should have a default-deny policy (`podSelector: {}`, `policyTypes: [Ingress, Egress]`) plus explicit allows, including DNS egress to `kube-system` on UDP/TCP 53.
- `namespaceSelector` and `podSelector` in the **same** `from` element are ANDed; in separate list elements they are ORed — a frequent bug.
- NetworkPolicy has no effect unless the CNI enforces it.
- Restrict operator egress to the API server: `ports: [443, 6443]`; to pin the destination add an `ipBlock` with the API server endpoint IPs (`kubectl get endpointslices -n default -l kubernetes.io/service-name=kubernetes`).

---

#### Security Hardening

##### Operator Pod Security Context

```yaml
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 65532
    runAsGroup: 65532
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

(`supplementalGroupsPolicy: Strict` would be dropped on 1.32 — do not rely on it to ignore image `/etc/group` entries.)

##### Pod Security Standards

Operator and workload namespaces should enforce **restricted**, pinned to the cluster version so a control-plane upgrade does not silently change the rules:

```yaml
pod-security.kubernetes.io/enforce: restricted
pod-security.kubernetes.io/enforce-version: v1.32
pod-security.kubernetes.io/warn: restricted
pod-security.kubernetes.io/audit: restricted
```

##### Secrets Management

- Never embed secrets in CRD `spec` fields; reference them (`secretRef: {name: ...}`).
- Do not log secret values, and do not put them in Event messages or condition messages.
- Use projected service-account tokens with an `audience` and short `expirationSeconds` instead of long-lived token Secrets (`kubernetes.io/service-account-token` Secrets are no longer auto-created since 1.24, and unused legacy ones are cleaned up).
- Use KMS v2 for envelope encryption at rest (GA since 1.29; the `KMSv2`/`KMSv2KDF` gates were removed in 1.32). KMS v1 (`kms` provider with `apiVersion: v1`) is deprecated and disabled by default since 1.29 (`KMSv1` gate, deprecated, default false in the v1.32 source).
- Consider External Secrets Operator or Vault Agent for rotation.

##### Image Security

- Pin images by **digest** in production manifests; enforce it with a ValidatingAdmissionPolicy (see above).
- Use distroless or scratch base images for operator binaries.
- Scan images (Trivy, Grype) in CI; never use `latest`.
- On 1.32 a Pod with `imagePullPolicy: IfNotPresent` can start from a private image already cached on the node **without presenting pull credentials** (credential verification for cached images, `KubeletEnsureSecretPulledImages`, is alpha in 1.33 and on by default only in 1.35). On multi-tenant 1.32 nodes use `imagePullPolicy: Always` for sensitive private images or the `AlwaysPullImages` admission plugin.

##### Node and cluster security issues fixed in 1.32 patch releases

| Fixed in | CVE | Impact |
|---|---|---|
| v1.32.1 | CVE-2024-9042 | Command injection on **Windows** nodes via `nodes/*/logs/query` |
| v1.32.2 | CVE-2025-0426 | Node DoS via the kubelet checkpoint API on the read-only port |
| v1.32.6 | CVE-2025-4563 | NodeRestriction bypass for DRA claims in mirror Pods |
| v1.32.8 | CVE-2025-5187 | A node could delete its own Node object via an owner reference |

1.32 is end of life, so later vulnerabilities are not fixed on this line at all. Infrastructure code that pins a 1.32.x patch below v1.32.13 should be flagged; any 1.32 pin should come with an upgrade plan.

##### Node-level 1.32 changes reviewers will meet in infrastructure code

- Kubelet: `--runonce` removed (startup error); `singleProcessOOMKill` option for cgroup v2; systemd watchdog integration (`SystemdWatchdog`, beta-on) restarts a hung kubelet when `WatchdogSec` is configured; strict decoding of `--image-credential-provider-config`.
- kube-controller-manager: `--cloud-provider` only `external` or empty; `node-monitor-grace-period` default 50 s.
- kube-apiserver: strict decoding of admission, egress-selector and tracing config files; `/statusz` and `/flagz` alpha.
- kube-proxy: no longer needs the `conntrack` binary; `nftables` mode is beta (the default mode remains `iptables`).
- kubeadm: `EtcdLearnerMode` locked on; `kubeadm upgrade apply phase ...` subcommands; removal of `--experimental-output`.

---

#### client-go & apimachinery in v0.32

##### Resource versions are opaque

On 1.32 clients must treat `metadata.resourceVersion` as an opaque string: compare for equality only, pass it back as a precondition or watch starting point, never parse or order it. Comparable resource versions and `k8s.io/apimachinery/pkg/util/resourceversion` arrive only in 1.35.

```go
// BAD on 1.32 — ordering resource versions is undefined behaviour
a, _ := strconv.ParseInt(cached.GetResourceVersion(), 10, 64)
b, _ := strconv.ParseInt(fresh.GetResourceVersion(), 10, 64)
if a < b { /* ... */ }

// GOOD — use generation / observedGeneration to detect spec changes,
// and optimistic concurrency (Update with resourceVersion, or MergeFromWithOptimisticLock) to detect races.
if obj.Status.ObservedGeneration < obj.Generation { /* not yet processed */ }
```

##### Informers and caches

- `cache.WaitForNamedCacheSync(name, stopCh, synced...)` is the API in client-go v0.32; `WaitForNamedCacheSyncWithContext` does not exist yet (verified: `undefined: cache.WaitForNamedCacheSyncWithContext`).
- **WatchList**: the apiserver supports streaming lists (`WatchList` beta, on) but the client-go gate `WatchListClient` is **beta and off** in v0.32 (`features/known_features.go`); informers still use paginated LIST unless `KUBE_FEATURE_WatchListClient=true` is set. Do not enable it in production on v0.32 without load testing.
- CBOR encoding (`ClientsAllowCBOR`, `ClientsPreferCBOR`) is alpha and off; keep JSON/protobuf.
- Prefer controller-runtime's cache options (`DefaultTransform: cache.TransformStripManagedFields()`, `ByObject` label/field selectors) to reduce memory instead of hand-rolled informers.

##### **[CHANGED in v0.32]** Generated fake clients are generic

client-go v0.32 generates typed fakes from a common generic implementation; the per-resource structs became private (v0.31: `type FakePods struct`; v0.32: `type fakePods struct`). Code that constructs or type-asserts `*fakecorev1.FakePods` no longer compiles — obtain fakes through `fake.NewClientset(...)` and `clientset.CoreV1().Pods(ns)`.

##### Fake clients

- `k8s.io/client-go/kubernetes/fake.NewClientset` is backed by a field-managed object tracker (`testing.NewFieldManagedObjectTracker`) and supports apply; prefer it over `NewSimpleClientset` in new tests.
- controller-runtime's fake client needs `WithStatusSubresource(&MyType{})` (since v0.15) when the code writes status; otherwise `Status().Update/Patch` returns NotFound.
- Fakes do not run admission, CEL validation, defaulting or garbage collection. Anything that depends on those belongs in an envtest test.

##### Clients and versions

- Keep `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` and controller-runtime on matching minors. A `go.mod` with `k8s.io/client-go v0.32.x` and `k8s.io/api v0.31.x` (or controller-runtime v0.19 with v0.32 libraries) is a review finding.
- Generated clientsets / listers / apply configurations for your CRDs must be regenerated with code-generator v0.32 when the libraries move to v0.32 (generic fakes, register-gen import fix in v0.32.3).
- `k8s.io/utils/pointer` is deprecated in favour of `k8s.io/utils/ptr` (`ptr.To(x)`, `ptr.Deref(p, def)`).
- `unstructured.NestedNumberAsFloat64` now returns an error instead of a silently rounded float for large `int64` values (fixed in v0.32).
- The `client-go/rest` client logs requests/responses through contextual logging with the caller's source location; pass a context logger (`klog.NewContext`) for useful output.

---

#### Observability

##### Structured Logging (logr)

```go
// ✅ Structured key-value pairs — never fmt.Sprintf for log fields
logger := log.FromContext(ctx).WithValues("generation", obj.Generation)
logger.Info("Starting reconcile")
logger.Error(err, "Failed to apply Deployment", "deployment", deploymentName)

// ❌ Avoid
logger.Info(fmt.Sprintf("reconciling %s/%s", req.Namespace, req.Name))
```

controller-runtime already adds `controller`, `controllerGroup`, `controllerKind`, `namespace`, `name` and `reconcileID` to the context logger; do not repeat them. **[CHANGED in v0.20]** API server warnings are no longer deduplicated by default ("Stop deduplicating API warnings by default") — a controller that triggers a deprecation warning on every reconcile now logs it every time; fix the cause (for example a deprecated API version) rather than raising the log level.

Log levels:
- `V(0)` / `Info` — significant lifecycle events
- `V(1)` — per-reconcile summaries
- `V(2)` — detailed decision logging
- `V(4)` — trace / dump level

##### Prometheus Metrics

Use the controller-runtime metrics registry (served by the manager's metrics server, `:8443` with `SecureServing`):

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

- Do not use object names or namespaces as label values for unbounded populations; that is a cardinality bug.
- controller-runtime already exports `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds` and workqueue metrics — do not duplicate them.

##### Health Probes

```go
utilruntime.Must(mgr.AddHealthzCheck("healthz", healthz.Ping))

// To gate readiness on cache sync, wrap it in a healthz.Checker:
utilruntime.Must(mgr.AddReadyzCheck("cache-sync", func(req *http.Request) error {
    if !mgr.GetCache().WaitForCacheSync(req.Context()) {
        return errors.New("informer caches not synced")
    }
    return nil
}))
```

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

See [Events](#events). Rules:

- Use `Warning` for errors requiring user attention, `Normal` for successful transitions.
- Do not emit Events on every reconcile — only on state changes.
- One-shot Jobs/CronJobs may rely on structured logs and exit status when there is no owned object status to update.
- Missing optional resources in a cleanup pass do not need Events unless users must take action.

##### Control-plane observability on 1.32

`/statusz` and `/flagz` on kube-apiserver are alpha and off; do not build monitoring on them. The scheduler and kube-controller-manager still expose `/metrics` and `/healthz`; kubeadm 1.32 switched the kube-scheduler static Pod probes to `/livez` and `/readyz`.

---

#### Error Handling & Requeue Strategy

##### Error Classification

```go
// Transient error — return it; the queue retries with per-item exponential backoff
return ctrl.Result{}, fmt.Errorf("apply Deployment: %w", err)

// Permanent error (invalid user input that CEL could not catch):
// set Stalled/Ready=False with a clear reason, emit a Warning Event, do NOT return the error
return ctrl.Result{}, nil

// Waiting on an external system with no watch
return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

// reconcile.TerminalError(err) — logged and counted, never retried
return ctrl.Result{}, reconcile.TerminalError(err)
```

##### Rules

- **Always** return the error from `Reconcile` for transient failures — do not swallow errors.
- **Never** return a non-nil error together with `RequeueAfter`/`Requeue` — the error wins and controller-runtime logs a warning; choose one.
- Avoid `Requeue: true`; use `RequeueAfter` or return an error (it is deprecated from v0.21, so new code written against v0.20 should not introduce it).
- Use `client.IgnoreNotFound(err)` when fetching objects that may not exist; use `apierrors.IsConflict(err)` → return the error (requeue) rather than retry loops inside Reconcile.
- The default per-item backoff starts at 5 ms and caps at 1000 s (~16 minutes) — rely on it rather than fighting it with custom requeue timing.
- A status write that fails after a successful external action must not repeat the external action non-idempotently on the next attempt.
- **[GA in 1.32]** `generateName` collisions are retried by the apiserver; code no longer needs its own retry loop around `Create` with `generateName`, but must still handle `AlreadyExists` for fixed names.

##### Jobs and CronJobs

Idempotent one-shot Jobs and CronJobs can rely on Kubernetes termination and safe reruns. Require explicit graceful shutdown only when there is cleanup work that process exit cannot handle safely.

Examples:
- A read-only scan or idempotent garbage-collection job may exit on termination and let the next schedule retry.
- A job that holds a lease, lock, transaction, or external reservation must release or renew it correctly during shutdown.
- A job that performs non-idempotent remote writes needs clear interruption and retry semantics.
- External API calls should still use contexts, timeouts, or Kubernetes `activeDeadlineSeconds`; absence of signal handling alone is not a bug.

---

#### Testing

##### Testing Pyramid

```
                    ┌────────────────────────┐
                    │    e2e (kind/k3s)      │  <- full operator in real cluster
                    ├────────────────────────┤
                    │ integration (envtest)  │  <- real kube-apiserver + etcd, no kubelet/controllers
                    ├────────────────────────┤
                    │   unit tests           │  <- pure logic, fake client
                    └────────────────────────┘
```

##### Unit Tests (fake client)

```go
func TestReconcile_DeletionRunsCleanupAndDropsFinalizer(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = cachev1.AddToScheme(scheme)

	now := metav1.Now()
	rc := &cachev1.RedisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test", Namespace: "default",
			Finalizers:        []string{finalizerName},
			DeletionTimestamp: &now, // fake client accepts this only together with a finalizer
		},
		Spec: cachev1.RedisClusterSpec{Tier: cachev1.TierPremium, Replicas: 3,
			Image: "registry.example.com/redis@sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rc).
		WithStatusSubresource(&cachev1.RedisCluster{}). // required when the reconciler writes status
		Build()
	cleanups := 0
	r := &RedisClusterReconciler{
		Client: c, Scheme: scheme, Recorder: record.NewFakeRecorder(10),
		Cleanup: func(context.Context, *cachev1.RedisCluster) error { cleanups++; return nil },
	}

	key := types.NamespacedName{Name: "test", Namespace: "default"}
	for i := 0; i < 2; i++ { // idempotency: the second call must be a no-op
		if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: key}); err != nil {
			t.Fatal(err)
		}
	}
	if cleanups != 1 {
		t.Fatalf("cleanup ran %d times, want 1", cleanups)
	}
	if err := c.Get(context.Background(), key, &cachev1.RedisCluster{}); !apierrors.IsNotFound(err) {
		t.Fatalf("object still present after finalizer removal: %v", err)
	}
}
```

The controller-runtime **v0.20** fake client does not implement server-side apply: verified, running the reconciler above against it fails at the apply step with `deployments.apps "test" not found` (an apply patch on a missing object is treated as a patch, not a create). Test SSA code paths with envtest, or hide the apply call behind an interface in unit tests. (Fake-client SSA support arrives in later controller-runtime releases; do not suggest it for v0.20.) The fake client also does not run CEL rules, defaulting, admission or GC.

##### Integration Tests (envtest) — pin the 1.32 binaries

```go
env := &envtest.Environment{
	CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd")},
	ErrorIfCRDPathMissing: true,
	BinaryAssetsDirectory: os.Getenv("KUBEBUILDER_ASSETS"), // from `setup-envtest use 1.32.x -p path`
}
cfg, err := env.Start()
if err != nil {
	t.Fatal(err)
}
defer func() { _ = env.Stop() }()
```

- Pin envtest to the cluster minor you ship to (`setup-envtest use 1.32.x`). An unpinned `setup-envtest use` picks the newest available Kubernetes version (1.37 in October 2026), which accepts fields a 1.32 cluster drops (pod-level resources, `PreferSameZone`, HPA tolerances, …).
- envtest has no kube-controller-manager: no garbage collection, no Deployment/ReplicaSet controllers, no namespace deletion, no default ServiceAccount. Tests must fake child status (e.g. update `Deployment.status.readyReplicas`) and must not expect owner-reference cascades.
- envtest is the place to test CEL rules, webhooks (with `WebhookInstallOptions`), SSA ownership and the deletion path.

Verified with the reconciler from this guide on envtest 1.32.0 binaries: an invalid CR is rejected (`apierrors.IsInvalid` true, both CEL messages returned); the Deployment is created by SSA with one controller owner reference and manager `rediscluster-controller` (operation `Apply`); after faking `status.readyReplicas`, the CR reaches `Ready=True`, `observedGeneration: 1`, and exactly one `Normal Available` Event is recorded; `Delete` bumps `metadata.generation` from 1 to 2 and sets `deletionTimestamp`; the next reconcile removes the finalizer and the object disappears.

##### e2e Tests (kind / k3s)

- Use a node image of the target minor (`kindest/node:v1.32.x`).
- Deploy the full operator image; test the complete lifecycle: Create → Reconcile → Update → Delete with finalizer.
- Use `kubectl wait --for=condition=Ready` assertions, and `--for=jsonpath='{.status.observedGeneration}'=N` for generation checks on your own CRs.

##### Testing Rules

- Test idempotency: run `Reconcile` twice and assert the same outcome.
- Test the deletion path including finalizer removal.
- Test that status is updated correctly on both success and error paths.
- Test that invalid CRs are rejected by CEL rules/webhooks (`apierrors.IsInvalid`).
- Run manifests through `kubectl apply --dry-run=server -o yaml` against a 1.32 apiserver in CI (envtest binaries or kind) and diff the result — client-side schema validation does not catch dropped gated fields, Pod Security admission, or CEL cost errors.

---

#### kubectl 1.32 and Manifest Tooling

- **[NEW in kubectl 1.32]** `kubectl apply --server-side --subresource=status` (and other subresources) — verified; kubectl 1.31 fails with `unknown flag: --subresource`. `--subresource` also works on `get`, `patch`, `edit`, `replace`.
- **[GA in 1.32]** `kubectl debug --custom=profile.yaml` applies a partial container spec (for example extra capabilities or env) to the debug container; the default `legacy` profile now prints a warning that it is planned for deprecation — use `--profile=general` (or `restricted`, `baseline`, `netadmin`, `sysadmin`) explicitly in runbooks.
- `kubectl get -o kyaml` does not exist (verified: `unable to match a printer suitable for the output format "kyaml"`); KYAML is 1.34+.
- `kubectl exec POD CMD` without `--` is rejected (verified: `exec [POD] [COMMAND] is not supported anymore. Use exec [POD] -- [COMMAND] instead`); scripts must use `kubectl exec POD -- CMD`.
- `kubectl explain` shows alpha fields that the server will drop (verified for `pod.spec.resources`); use `--dry-run=server -o yaml` to prove a field survives. `kubectl explain -o plaintext-openapiv2` gained the `-o` shorthand in 1.32.
- `kubectl wait --for=create` waits until an object exists. It works since kubectl 1.31 (verified: kubectl 1.30 fails with `unrecognized condition: "create"`); 1.32 only adds it to the help text.
- `kubectl top node` prints percentages as `(%)`; parsers must be updated.
- YAML coercion is a real manifest bug, not a style issue: verified, `metadata: {name: n}` fails with `json: cannot unmarshal bool into Go struct field ObjectMeta.metadata.name of type string`. Quote values such as `y`, `n`, `yes`, `no`, `on`, `off`, `true`, `false`, version strings (`"1.10"`), and octal-looking numbers in labels, annotations, env values and ConfigMap data.
- kubectl 1.32 embeds kustomize v5.5.0 (`kubectl kustomize`, `kubectl apply -k`).
- **Helm**: declare `kubeVersion` in `Chart.yaml` when a chart depends on 1.32 behaviour, and gate version-specific fields with `semverCompare` on `.Capabilities.KubeVersion.Version`:
  ```yaml
  # templates/service.yaml
  {{- if semverCompare ">=1.34.0-0" .Capabilities.KubeVersion.Version }}
    trafficDistribution: PreferSameZone
  {{- else }}
    trafficDistribution: PreferClose
  {{- end }}
  ```
  `helm template` without `--kube-version` renders for a built-in default, not your cluster — verified with a chart declaring `kubeVersion: ">=1.32.0-0 <1.33.0-0"`: Helm v3.22.1 and v4.3.1 fail with `incompatible with Kubernetes v1.37.0`, Helm v4.0.5 with `... v1.34.0`; with `--kube-version 1.32.13` all three render `PreferClose`. Always pass `--kube-version 1.32.x` in CI renders.

---

#### Removed / Changed in 1.32 — Migration Notes

| Change | Who is affected | Action |
|---|---|---|
| `flowcontrol.apiserver.k8s.io/v1beta3` removed | FlowSchema/PriorityLevelConfiguration manifests, API clients | Switch to `flowcontrol.apiserver.k8s.io/v1` (explicit `nominalConcurrencyShares: 0` is kept as 0) |
| Classic DRA removed | Clusters with `DRAControlPlaneController` workloads | Remove those workloads and `PodSchedulingContext` objects before upgrading |
| `coordination.k8s.io/v1alpha1` dropped | Clusters that enabled it | Delete `v1alpha1` objects before upgrading |
| `kubernetes.io/enforce-mountable-secrets` deprecated | ServiceAccounts using it | Isolate by namespace; remove the annotation |
| Scheduler volume-limit plugins removed | Custom scheduler configs | Remove `AzureDiskLimits`/`CinderLimits`/`EBSLimits`/`GCEPDLimits` |
| In-tree cloud controllers removed from KCM | `--cloud-provider=<cloud>` | Run an external cloud-controller-manager, `--cloud-provider=external` |
| Kubelet `--runonce` removed | Bootstrap scripts | Remove the flag |
| Strict decoding of apiserver/kubelet config files | Configs with typos/unknown fields | Validate config files before upgrading |
| GA feature gates removed (`ValidatingAdmissionPolicy`, `ServerSideApply`, `KMSv2`, …) | Component flags | Remove the gates from `--feature-gates` |
| Pod Security `v1.32` allows `tcp_rmem`/`tcp_wmem` sysctls | Namespaces with `enforce-version: latest` | Pin `enforce-version` deliberately |
| `ServiceAccountNodeAudienceRestriction` off from v1.32.2 | CSI drivers/in-tree volumes requesting tokens | Run ≥ v1.32.2 when using in-tree `azureFile` |
| `postStart`/kubelet status regressions | Clusters on v1.32.0–v1.32.2 | Upgrade to ≥ v1.32.3 (ideally v1.32.13) |
| controller-runtime v0.20: `admission.Defaulter`/`Validator` removed | Webhooks implemented as methods on API types | Move to `CustomDefaulter`/`CustomValidator` structs registered with `WithDefaulter`/`WithValidator` |
| controller-runtime v0.20: `CustomDefaulter` keeps unknown fields | Defaulting webhooks | Add `admission.DefaulterRemoveUnknownOrOmitableFields` only if pruning was intended |
| controller-runtime v0.20: API warnings not deduplicated | Log volume | Fix deprecated API usage |
| client-go v0.32: fake typed clients private | Tests using `FakePods` etc. directly | Use `fake.NewClientset` |
| `ScalingReplicaSet` event text changed | Event parsers | Match on `reason`, not message text |
| **1.32 end of life (2026-02-28)** | Everyone | Plan the upgrade path 1.32 → 1.33 → 1.34 → 1.35+ (one minor at a time for the control plane) |

---

#### Production / Review Checklist

##### Version & API availability

- [ ] Manifests use only API versions served by a default 1.32 apiserver (no `flowcontrol.apiserver.k8s.io/v1beta3`; no `v1alpha1` MutatingAdmissionPolicy, `v1beta1` DRA/VolumeAttributesClass/ServiceCIDR without documented cluster opt-in)
- [ ] No 1.33+ fields or behaviour assumed (in-place resize, `PreferSameZone`, HPA tolerances, pod-level resources, `preStop.sleep.seconds: 0`, Service names starting with digits, `semver`/`sort()` in CEL, …)
- [ ] No silently dropped gated fields (`hostUsers`, `supplementalGroupsPolicy`, `resizePolicy`, `volumes[].image`, StatefulSet `maxUnavailable`) unless the cluster enables the gate
- [ ] Cluster patch level ≥ v1.32.3 (regressions) and preferably v1.32.13; an upgrade plan exists because 1.32 is end of life
- [ ] `go.mod`: all `k8s.io/*` on the same v0.32.x patch, controller-runtime v0.20.x, Go ≥ 1.23
- [ ] envtest/kind pinned to 1.32, not "latest"; Helm CI renders with `--kube-version 1.32.x`

##### CRD Design

- [ ] Kind: CamelCase singular; Resource: lowercase plural; group is a DNS subdomain you own (not `*.k8s.io`)
- [ ] `// +kubebuilder:subresource:status`; `status.conditions` (`listType=map`, key `type`) + `status.observedGeneration`
- [ ] All enum fields use string type aliases (not `bool`); no floats in `spec`
- [ ] Every string/array/map reachable from CEL has `maxLength`/`maxItems`/`maxProperties`
- [ ] Numeric fields have `minimum`/`maximum` (`format: int32` is not a range check)
- [ ] CEL `messageExpression` uses `'%d'.format([...])`, not `string(int)`; no expensive library calls in messages; transition rules handle removal of the field
- [ ] Only CEL functions available on 1.32 (no `isSemver`, list `sort`/`first`, `math.*`); reserved-word fields only if 1.31 support is not required
- [ ] Rules use `fieldPath` and `reason`; `optionalOldSelf` only with `oldSelf.hasValue()`
- [ ] At least one printer column with state info; `selectableFields` for commonly filtered fields
- [ ] `omitempty` + `// +optional` on all optional fields

##### Controller

- [ ] Fetch → `IgnoreNotFound` → deletion/finalizer → converge → status
- [ ] Children applied with SSA (`Patch(..., client.Apply, FieldOwner, ForceOwnership)` from apply configurations converted to unstructured) or patched — never typed structs applied, never `Update` from cached objects
- [ ] Status written through `Status().Patch`; `observedGeneration` set; conditions via `meta.SetStatusCondition`
- [ ] Transient errors returned; permanent errors → condition + Warning Event + `nil`/`TerminalError`; no error + `RequeueAfter`
- [ ] Recorder from `GetEventRecorderFor` with core `events` RBAC; Events only on transitions; no user text as format string
- [ ] Webhooks use `CustomValidator`/`CustomDefaulter` with checked type assertions; CEL preferred where possible
- [ ] Watches on shared objects use field indexes and restricted caches
- [ ] Metrics served securely (`SecureServing` + `filters.WithAuthenticationAndAuthorization`); leader election enabled; priority queue not enabled on v0.20

##### Workloads

- [ ] Pod Security `restricted` passes (runAsNonRoot, seccomp `RuntimeDefault`, drop `ALL`, no privilege escalation); `enforce-version` pinned
- [ ] Images pinned by digest; sensitive private images on shared nodes use `imagePullPolicy: Always` (no cached-image credential check on 1.32)
- [ ] Memory requests/limits set; memory-backed `emptyDir` has `sizeLimit`
- [ ] Probes: startup + readiness + liveness with sane thresholds; `preStop.sleep` with `seconds ≥ 1`
- [ ] Sidecars as `initContainers` with `restartPolicy: Always`
- [ ] Services use `trafficDistribution: PreferClose` (not `PreferSameZone`); no `externalIPs`; LB controllers set `ipMode`
- [ ] HPA-managed Deployments do not set `spec.replicas` in applied manifests; no HPA `tolerance`
- [ ] Job `managedBy` only when an external Job controller exists
- [ ] StatefulSet `persistentVolumeClaimRetentionPolicy.whenDeleted: Delete` only for disposable data
- [ ] Unquoted YAML scalars that coerce (`n`, `N`, `yes`, `on`, `1.10`) are quoted; env var names are C identifiers

##### Security / RBAC

- [ ] No wildcard verbs/resources; namespace-scoped roles where possible
- [ ] `get` on `pods/exec`/`attach`/`portforward` treated as exec permission (WebSocket on 1.32); exec granted as `create` and only to those who need it
- [ ] `nodes/proxy` grants justified (fine-grained kubelet authz is alpha on 1.32)
- [ ] No bindings to `system:anonymous`/`system:unauthenticated` beyond defaults; anonymous access limited via `AuthenticationConfiguration` where possible
- [ ] Service account tokens not automounted unless needed; no `enforce-mountable-secrets` annotation
- [ ] Secrets referenced by name — never inlined in CRD spec, logs, Events or conditions
- [ ] Default-deny NetworkPolicy plus explicit allows (including DNS)
- [ ] Admission webhooks exclude their own namespace and `kube-system`; `sideEffects: None`; timeouts ≤ 10 s

##### Finalizers & Versioning

- [ ] Finalizer name is fully qualified (`domain.com/name`), added before external resources are created, removed only after successful idempotent cleanup
- [ ] New CRD fields optional; no fields removed or renamed in served versions
- [ ] `deprecationWarning` set on deprecated CRD versions; conversion webhook when schemas diverge
- [ ] `storage: true` on exactly one version; stored objects migrated and `status.storedVersions` cleaned before dropping a version
