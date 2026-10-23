### Kubernetes 1.33: Manifests, Policies and Go Controllers — Complete Review Guide

> **Version**: Kubernetes v1.33.0 "Octarine", released 2025-04-23. The last patch release is **v1.33.13** (release artifacts dated 2026-06-12; `dl.k8s.io/release/stable-1.33.txt` reports `v1.33.13`). There will be no further 1.33 patches.[^1][^2][^3]
> **Support**: 1.33 reached **end of life on 2026-06-28** and is **out of support** as of October 2026: no security fixes, no bug fixes, no CVE backports. The newest minor is **1.37** (v1.37.0 on 2026-08-26, latest patch v1.37.1). Supported minors as of 2026-10-22: 1.37, 1.36 (v1.36.5), 1.35 (v1.35.9) and 1.34 (v1.34.12, EOL **2026-10-27**). A cluster on 1.33 needs an upgrade plan; it cannot skip minors, so it must go 1.33 → 1.34 → 1.35 (…) one minor at a time.[^2]
> **Client libraries**: `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go` and the other staging modules at **v0.33.x** (final v0.33.13, tagged 2026-06-12); **controller-runtime v0.21.0** (2025-05-20, the only v0.21 release) and **controller-tools v0.18.0** (2025-05-08) pair with 1.33. All of them declare `go 1.24.0`. Kubernetes 1.33.0 was built with Go 1.24.2; from v1.33.11 the release binaries are built with Go 1.25 (v1.33.13: Go 1.25.11).[^4][^5]
> **Scope**: Kubernetes manifests (raw YAML, Kustomize output, Helm-rendered YAML), admission policies (ValidatingAdmissionPolicy, webhooks), CustomResourceDefinitions with CEL validation, and Go operators/controllers built with controller-runtime, client-go and Kubebuilder that target a 1.33 cluster.
> **Note**: This document is self-contained. It carries forward every still-correct house rule (operator principles, CRD design, status conditions, finalizers, RBAC, webhooks, observability, testing) and adds the 1.33-specific layer. Sections marked **[NEW in 1.33]** cover behaviour that differs from 1.32. Every manifest in this guide was applied (for real or with `kubectl apply --dry-run=server`) against a v1.33.0 kube-apiserver with etcd; boundary claims were re-checked against v1.32.0 and v1.34.1 apiservers; feature-gate states were read from the `kubernetes_feature_enabled` metric of those apiservers. Go snippets compile against k8s.io/* v0.33.13 and controller-runtime v0.21.0. Runtime (kubelet, scheduler, controller-manager) behaviour is sourced from the release notes and documentation, not executed.

---

#### Table of Contents

1. What's New in 1.33 (overview)
2. Do Not Use or Recommend (1.34+ features, and what 1.33 deprecated or removed)
3. Version-Aware Review
4. API Versions Served by a 1.33 Cluster
5. Feature-Gate Defaults That Matter to Authors
6. Usable Features GA in Earlier Releases (cumulative)
7. Foundational Principles
8. Project Layout, Toolchain and Dependency Pinning
9. Naming Conventions
10. Workload Manifests (Pods, Deployments, StatefulSets, Jobs, CronJobs, HPA, PDB)
11. Pod Security: securityContext, Pod Security Admission, User Namespaces
12. Networking: Services, ServiceCIDR, NetworkPolicy, Ingress
13. Admission Control: ValidatingAdmissionPolicy and Webhooks
14. CRD & API Design
15. CEL in CRDs and Policies
16. Spec vs. Status, Status Conditions
17. Versioning & API Evolution
18. Controller / Reconciler Patterns (controller-runtime v0.21)
19. Watches, Predicates, Caches and Informers (client-go v0.33)
20. Finalizers
21. Owner References & Garbage Collection
22. Validation & Admission Webhooks in Go
23. RBAC & Least Privilege
24. Security Hardening
25. Observability
26. Error Handling & Requeue Strategy
27. Testing
28. kubectl 1.33 for Reviewers
29. Removed / Changed in 1.33 — Migration Notes
30. Production / Review Checklist

---

#### What's New in 1.33

Kubernetes 1.33 contains 64 enhancements: 18 graduated to Stable, 20 entered Beta, 24 are new Alpha features, and 2 were deprecated or withdrawn.[^1] The tables list every change a manifest, policy or controller author can observe. "Default" means the state on a stock 1.33 control plane, read from the `kubernetes_feature_enabled` metric of a v1.33.0 kube-apiserver.

##### Graduated to Stable (GA) in 1.33

| Feature | What authors see | Gate in 1.33 |
|---|---|---|
| **Sidecar containers** (KEP-753) | `initContainers[].restartPolicy: Always` starts the container before the app containers, keeps it running (with probes) and stops it after them. | `SidecarContainers` GA, locked on |
| **Job backoff limit per index** (KEP-3850) | `spec.backoffLimitPerIndex` and `spec.maxFailedIndexes` for `completionMode: Indexed`. | `JobBackoffLimitPerIndex` GA |
| **Job success policy** (KEP-3998) | `spec.successPolicy.rules[].succeededIndexes` / `succeededCount`. | `JobSuccessPolicy` GA |
| **Multiple Service CIDRs** (KEP-1880) | `networking.k8s.io/v1` `ServiceCIDR` and `IPAddress`; admins add ClusterIP ranges at runtime. On 1.32 the `v1` kinds do not exist (verified: `no matches for kind "ServiceCIDR" in version "networking.k8s.io/v1"`). | `MultiCIDRServiceAllocator` GA |
| **Service `trafficDistribution: PreferClose`** (KEP-4444, KEP-2433) | GA. `PreferClose` is the **only** accepted value on 1.33. EndpointSlice `hints` are GA; the `service.kubernetes.io/topology-mode` annotation is deprecated. | `ServiceTrafficDistribution`, `TopologyAwareHints` GA |
| **`matchLabelKeys` / `mismatchLabelKeys` in pod (anti-)affinity** (KEP-3633) | Scope affinity terms to, for example, the current rollout revision (`pod-template-hash`). | `MatchLabelKeysInPodAffinity` GA |
| **Topology spread `nodeAffinityPolicy` / `nodeTaintsPolicy`** (KEP-3094) | Decide whether node affinity and taints count when computing skew. | `NodeInclusionPolicyInPodTopologySpread` GA |
| **CRD validation ratcheting** (KEP-4008) | Unchanged fields of existing custom resources are not re-validated against tightened schemas; 1.33 also ratchets the `status` subresource. | `CRDValidationRatcheting` GA |
| **Recursive read-only mounts** (KEP-3857) | `volumeMounts[].recursiveReadOnly: Enabled` / `IfPossible` makes submounts read-only too. | `RecursiveReadOnlyMounts` GA |
| **Bound service-account token improvements** (KEP-4193) | Tokens carry a JTI and node info; node-bound tokens. | `ServiceAccountTokenNodeBinding` GA |
| **kubectl `--subresource`** (KEP-2590) | `kubectl get/patch/edit/apply/replace --subresource status\|scale\|resize`. | GA |
| **nftables kube-proxy** (KEP-3866) | `--proxy-mode nftables`; iptables stays the default. | `NFTablesProxyMode` GA |
| **Volume populators** (KEP-1495) | `PersistentVolumeClaim.spec.dataSourceRef` to custom resources (needs the VolumePopulator CRD and populator controller). | `AnyVolumeDataSource` GA |
| **Always honor PV reclaim policy** (KEP-2644) | Finalizers on PVs ensure `Delete` is executed even when the PV is deleted before the PVC. | `HonorPVReclaimPolicy` GA |
| **Reject non-SMT-aligned workloads** (KEP-2625) | CPU manager policy options (node-level). | `CPUManagerPolicyOptions` GA |
| **`status.nodeInfo.kubeProxyVersion` removal** (KEP-4004) | The kubelet no longer sets the field. | `DisableNodeKubeProxyVersion` on |
| Portworx in-tree → CSI migration | All `portworxVolume` operations go to the `pxd.portworx.com` CSI driver. | `CSIMigrationPortworx` GA |

##### Promoted to Beta, or enabled by default, in 1.33

| Feature | What authors see | Default in 1.33 |
|---|---|---|
| **In-place Pod resize** (KEP-1287) **[NEW in 1.33]** | Container CPU/memory are changed through the new **`resize` subresource**; `resizePolicy` per resource. Sidecars can be resized. `status.resize` is deprecated in favour of the `PodResizePending` / `PodResizeInProgress` conditions. | beta, **on** (was alpha/off in 1.32) |
| **User namespaces** (KEP-127) **[NEW in 1.33]** | `spec.hostUsers: false` is kept by the apiserver (1.32 dropped it). Needs node support (see Pod Security). | beta, **on** (was beta/off) |
| **`procMount: Unmasked`** (KEP-4265) | Allowed only with `hostUsers: false`. | beta, **on** (was beta/off) |
| **Fine-grained supplemental groups** (KEP-3619) | `securityContext.supplementalGroupsPolicy: Strict` (or `Merge`). 1.32 dropped the field. The kubelet rejects `Strict` Pods on nodes that report `status.features.supplementalGroupsPolicy: false`. | beta, **on** |
| **Zero-second `preStop` sleep** (KEP-4818) | `lifecycle.preStop.sleep.seconds: 0`. 1.32 rejects it (`must be greater than 0`). | beta, **on** |
| **Relaxed DNS search validation** | `dnsConfig.searches` may contain `.` and `_` labels. 1.32 rejects them. | beta, **on** |
| **SELinux change policy** | `securityContext.seLinuxChangePolicy` (`MountOption` / `Recursive`). The cluster-wide `SELinuxMount` gate is beta but **off**. | `SELinuxChangePolicy` beta, on |
| **Ordered namespace deletion** | Pods are deleted before other namespaced resources when a namespace is deleted. | beta, on |
| **Kubelet fine-grained authorization** (KEP-2862) | `nodes/metrics`, `nodes/stats`, `nodes/pods`, `nodes/healthz`, `nodes/configz` instead of `nodes/proxy`. | beta, on |
| **ClusterTrustBundle** (KEP-3257) | `certificates.k8s.io/v1beta1`; beta API, **off by default** (needs `--runtime-config` and the gate). | beta, **off** |
| **Image volumes** (KEP-4639) | `volumes[].image` graduated to beta, but the gate is **off**: the apiserver silently drops the field (verified). | beta, **off** |
| **Coordinated leader election** | `coordination.k8s.io/v1beta1` `LeaseCandidate`; off by default. | beta, off |
| **DRA structured parameters** | New `resource.k8s.io/v1beta2`; `v1beta1` deprecated. DRA itself stays **beta and off** (`DynamicResourceAllocation`), and both beta API versions are disabled by default. Users with the namespaced `edit` role may manage ResourceClaims. | beta, **off** |
| Declarative validation (validation-gen) | Internal; mismatches are only reported via metrics. | `DeclarativeValidation` beta, on |
| Streaming list encoding | The apiserver streams large JSON/Protobuf list responses item by item (less memory). | beta, on |
| `SchedulerAsyncPreemption`, `SchedulerPopFromBackoffQ`, `WinDSR`, `RemoteRequestHeaderUID`, `DistributeCPUsAcrossNUMA` | Cluster/node level. | beta, on |

##### New Alpha in 1.33 (off by default — fields are dropped or values rejected on a stock cluster)

`HPAConfigurableTolerance` (`behavior.scaleUp/scaleDown.tolerance` — **dropped**, verified), `ContainerStopSignals` (`lifecycle.stopSignal`, requires `spec.os.name` — **dropped**, verified), `PodObservedGenerationTracking` (`status.observedGeneration` on Pods), `PreferSameTrafficDistribution` (`PreferSameZone`/`PreferSameNode` — **rejected**, verified), `StrictIPCIDRValidation` (only warnings without it), `DeploymentReplicaSetTerminatingReplicas` (`status.terminatingReplicas`), `KubeletEnsureSecretPulledImages` (re-check credentials for cached images), `KubeletServiceAccountTokenForCredentialProviders` (secret-less image pulls), `PodTopologyLabelsAdmission` (zone/region labels copied to Pods for the downward API), `DRADeviceTaints`, `DRAPrioritizedList` (`firstAvailable`), `DRAPartitionableDevices`, `MutableCSINodeAllocatableCount`, `StorageCapacityScoring`, `ReduceDefaultCrashLoopBackOffDecay`, `KubeletPSI`, `ListFromCacheSnapshot`, kubectl `kuberc` (opt-in with `KUBECTL_KUBERC=true`).[^6]

##### Changed defaults and behaviour in 1.33

- **Pods get `metadata.generation`.** New Pods start with `metadata.generation: 1`, and every accepted change to a mutable `spec` field (including a resize) increments it (verified: create → 1, resize → 2, `activeDeadlineSeconds` update → 3; on 1.32 the field stays empty). `status.observedGeneration` is still alpha/off, so tools cannot yet compare the two. **[NEW in 1.33]**
- **Container resources are changed through `pods/resize`.** `kubectl patch pod web --subresource resize --patch '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"500m"}}}]}}'` works on 1.33; a plain patch of `spec.containers[*].resources` is still rejected (`pod updates may not change fields other than ...`); 1.32 has no `resize` subresource (`the server could not find the requested resource`). **[NEW in 1.33]**
- **Memory limits can only shrink with `RestartContainer`.** On 1.33 a resize that lowers a memory limit fails with `memory limits cannot be decreased unless resizePolicy is RestartContainer` (verified); 1.34 accepts the same request. `resizePolicy` is immutable after creation. **[NEW in 1.33]**
- **`gitRepo` volumes are disabled by default on the kubelet.** The apiserver still admits them with `Warning: spec.volumes[0].gitRepo: deprecated in v1.11, and disabled by default in v1.33+` (verified); the kubelet refuses to run them unless the node re-enables `GitRepoVolumeDriver`. The in-tree driver is planned to be removed for good in v1.39.[^1]
- **The `v1` Endpoints API is deprecated.** Requests to `v1` Endpoints return `Warning: v1 Endpoints is deprecated in v1.33+; use discovery.k8s.io/v1 EndpointSlice` (verified on create; 1.32 is silent). The API will not be removed, but code should read EndpointSlices.[^7]
- **IP/CIDR warnings got clearer.** Non-canonical values are still accepted but warn: `non-standard CIDR value "010.0.0.0/8" will be considered invalid in a future Kubernetes release` and `CIDR value "192.168.0.5/24" is ambiguous in this context` (verified). They become errors only when `StrictIPCIDRValidation` is enabled (alpha in 1.33).
- **StatefulSet `serviceName` must be a valid DNS-1123 label on create** (verified: `serviceName: Bad_Name` is rejected on 1.33 with `a lowercase RFC 1123 label must consist of ...`, accepted on 1.32); previously such StatefulSets were stored but could never create Pods.
- **`kubectl apply` coerces `null` label/annotation values to `""`** instead of dropping all labels and annotations.
- **WatchList is off again on the apiserver** (`WatchList` beta, **off**) in favour of streaming list encoding; client-go's `WatchListClient` is beta/off as well.
- **CEL**: two-variable comprehensions (`all(i, v, ...)`, `exists(i, v, ...)`, `existsOne(i, v, ...)`) and `transformList` / `transformMap` / `transformMapEntry` are available in CRD rules and policies (verified on 1.33, rejected on 1.32). `cel-go` was bumped to v0.23.2.
- **Windows**: host-network support for Windows Pods (`WindowsHostNetwork`) was withdrawn; HostProcess containers are unaffected.
- **Removed feature gates** (setting one makes the component exit; verified: `--feature-gates=AppArmor=true` → kube-apiserver 1.33 fails with `unrecognized feature gate: AppArmor`): `AppArmor`, `AppArmorFields`, `CPUManager`, `JobPodFailurePolicy`, `PDBUnhealthyPodEvictionPolicy`, `PersistentVolumeLastPhaseTransitionTime`, `KubeProxyDrainingTerminatingNodes`, `DisableCloudProviders`, `DisableKubeletCloudCredentialProviders`; the kube-apiserver flags `--cloud-provider` and `--cloud-config` are gone. `admissionregistration.k8s.io/v1alpha1` ValidatingAdmissionPolicy serving was removed.[^6]

##### Patch-release fixes that matter (1.33.1–1.33.13)

- 1.33.1: restored the per-ResourceClaim device limit to 32 (1.33.0 regressed it to 16).
- 1.33.2: kubelet stops static Pods from referencing arbitrary ResourceClaims; clearer error for user-namespaced Pods on runtimes without support.
- 1.33.3: fixed a 1.33 regression where paginated LISTs fell back to etcd instead of the watch cache; fixed delayed Pod creation for new Jobs.
- 1.33.4: fixed a 1.33 kube-scheduler nil-pointer panic when aggregating container requests; NodeRestriction forbids nodes changing their own `ownerReferences`.
- 1.33.5: masked Linux thermal-interrupt info in `/proc` and `/sys`.
- 1.33.6: fixed a 1.33 regression in DaemonSet handling of orphaned Pods; fixed sidecar startup-probe races; nftables kube-proxy fixes.
- 1.33.7: fixed a data race that made ValidatingAdmissionPolicy evaluation non-deterministic, and spurious "namespace not found" errors for policies on objects in newly created namespaces; fixed an apiserver panic for CRDs that declare a status subresource but no `status` schema.
- 1.33.10: fixed a kube-controller-manager crash when a ValidatingAdmissionPolicy's object schema has `additionalProperties: true`.
- 1.33.11: built with Go 1.25; fixed apiserver start-up with many namespaces when `MultiCIDRServiceAllocator` is on.
- 1.33.13: built with Go 1.25.11; endpoint-controller panic fix for pre-dual-stack Services.[^3]

Anything still on 1.33 should at least run v1.33.13 — and should be upgraded, because no further fixes will be published.

***

#### Do Not Use or Recommend

##### Features from Kubernetes 1.34 and later (unavailable on a 1.33 cluster)

A 1.33 apiserver does not know these API versions, fields or values, or it knows the field but drops it because the gate is off. Never suggest them for code that targets 1.33. Verified on a v1.33.0 apiserver: unknown fields are rejected by `kubectl apply` (strict server-side field validation) and **silently dropped** when field validation is off (`--validate=false`, many Go clients, some CD tools — verified with `spec.hostnameOverride`).

| Do not use on 1.33 | Introduced in | What happens on 1.33 (verified) |
|---|---|---|
| `resource.k8s.io/v1` (DRA GA: DeviceClass, ResourceClaim, ResourceClaimTemplate, ResourceSlice) | 1.34 GA | `no matches for kind "DeviceClass" in version "resource.k8s.io/v1"`; 1.33 has only `v1beta1`/`v1beta2`, both off by default |
| `admissionregistration.k8s.io/v1beta1` MutatingAdmissionPolicy (1.34, off by default) and `v1` (GA 1.36) | 1.34 / 1.36 | `no matches for kind "MutatingAdmissionPolicy"`; 1.33 has only `v1alpha1` behind an alpha gate |
| Service `trafficDistribution: PreferSameZone` / `PreferSameNode` | 1.34 beta (GA 1.35) | `Unsupported value: "PreferSameNode": supported values: "PreferClose"` |
| Pod `spec.hostnameOverride` | 1.34 alpha | `unknown field "spec.hostnameOverride"` |
| `env[].valueFrom.fileKeyRef` (EnvFiles) | 1.34 alpha | `unknown field "spec.containers[0].env[0].valueFrom.fileKeyRef"` |
| Container `restartPolicy` + `restartPolicyRules` on regular containers | 1.34 alpha | `unknown field "spec.containers[0].restartPolicyRules"` |
| Pod-level `spec.resources` | 1.32 alpha, **1.34 beta/on** | field exists but is **dropped** on 1.33 (alpha/off) |
| HPA `behavior.*.tolerance` | 1.33 alpha (still alpha/off in 1.34), beta/on in 1.35 | **dropped** on 1.33 and 1.34 (1.32 rejects it as unknown) |
| HPA `minReplicas: 0` | alpha everywhere up to 1.36 | `must be greater than or equal to 1` |
| `lifecycle.stopSignal` | 1.33 alpha, beta later | **dropped** |
| Image volumes `volumes[].image` | beta/on from 1.35, GA 1.36 | **dropped** on 1.33 and 1.34 (beta/off) — the Pod would start without the volume source |
| StatefulSet `rollingUpdate.maxUnavailable` | beta/on in 1.35 | **dropped** on 1.32–1.34 (alpha/off); stored object keeps only `partition` |
| Pod `status.observedGeneration` populated by the kubelet | 1.34 beta/on, GA 1.35 | alpha/off: the field is in the schema but nobody sets it |
| Leading-digit Service names (`8080-web`) | 1.34 alpha, 1.36 beta/on | `a DNS-1035 label must ... start with an alphabetic character` |
| Decreasing a memory limit in place without `RestartContainer` | 1.34 | `memory limits cannot be decreased unless resizePolicy is RestartContainer` |
| CEL semver library (`isSemver`, `semver(...)`) | 1.34 | the CRD itself is rejected at creation |
| `storage.k8s.io/v1` VolumeAttributesClass | 1.34 GA | only `v1beta1`, off by default |
| PodCertificateRequest / `podCertificate` projected volumes | 1.34 alpha | not available |
| `status.terminatingReplicas` on Deployments/ReplicaSets | 1.33 alpha, 1.35 beta/on | alpha/off on 1.33 — not populated |
| KYAML output (`-o kyaml`) | kubectl 1.34 (opt-in via `KUBECTL_KYAML=true`), default from 1.35 | kubectl 1.33: `unable to match a printer suitable for the output format "kyaml"` |
| `kubectl diff --show-secrets`, `kubectl explain -R` | kubectl 1.36 | not in kubectl 1.33 (only `--recursive`; `diff` has no `--show-secrets`) |
| controller-runtime **v0.22+**, controller-tools **v0.19+**, k8s.io/* **v0.34+** | 1.34 line | pair with newer clusters; their API types contain fields a 1.33 server drops |
| controller-runtime `client.Client.Apply(ctx, applyConfiguration, ...)`, fake-client server-side apply | controller-runtime v0.22 | v0.21 has only `client.Patch(ctx, obj, client.Apply)`; the v0.21 fake client returns NotFound for absent objects and `apply patches are not supported in the fake client` for existing ones (verified) |
| controller-runtime typed webhooks (`admission.Validator[T]`), `GetEventRecorder` (events.k8s.io), `r.Status().Apply`, priority queue on by default | controller-runtime v0.23 | v0.21 uses `admission.CustomValidator`, `GetEventRecorderFor` (core/v1 Events); the priority queue exists but is opt-in |

Later releases also add structured `restartAllContainers` actions, workload-aware scheduling, `storagemigration.k8s.io/v1`, `metrics.k8s.io/v1`, image-volume GA, MutatingAdmissionPolicy GA and user namespaces GA; none of those exist on 1.33.

##### Deprecated or removed as of 1.33 — flag in review

| Item | Status | Replacement |
|---|---|---|
| `v1` Endpoints (reading/writing directly) | Deprecated in 1.33 (warning), not scheduled for removal | `discovery.k8s.io/v1` EndpointSlice |
| `gitRepo` volume | Disabled by default on the kubelet in 1.33 (deprecated since 1.11); planned removal in 1.39 | init container or git-sync sidecar into an `emptyDir` |
| `service.kubernetes.io/topology-mode` annotation | Deprecated; will not go GA | `spec.trafficDistribution: PreferClose` |
| Pod `status.resize` | Deprecated, no longer set | `PodResizePending` / `PodResizeInProgress` conditions |
| `node.status.nodeInfo.kubeProxyVersion` | No longer set by the kubelet | — (do not read it) |
| `resource.k8s.io/v1beta1` | Deprecated (removal planned for 1.36) | `v1beta2` on 1.33, `v1` from 1.34 |
| `admissionregistration.k8s.io/v1alpha1` ValidatingAdmissionPolicy | Removed | `admissionregistration.k8s.io/v1` (GA 1.30) |
| Windows `hostNetwork` Pods | Withdrawn | HostProcess containers |
| In-tree cloud-provider flags (`--cloud-provider`, `--cloud-config` on kube-apiserver) | Removed | external cloud controller manager |
| `WatchFromStorageWithoutResourceVersion`, `SeparateCacheWatchRPC`, `StorageNamespaceIndex` gates | Deprecated | — |
| controller-runtime `ctrl.Result{Requeue: true}` | **Deprecated in v0.21** ("Use `RequeueAfter` instead") | `RequeueAfter` or return an error |
| controller-runtime `WebhookBuilder.WithCustomPath` | Deprecated | default paths, or `WithValidatorCustomPath`/`WithDefaulterCustomPath` |
| client-go `fake.NewSimpleClientset` | Deprecated | `fake.NewClientset` (field management, SSA support) |

Older removals that still show up in charts and must be rejected: `extensions/v1beta1` and `networking.k8s.io/v1beta1` Ingress (1.22), `batch/v1beta1` CronJob, `policy/v1beta1` PodDisruptionBudget and PodSecurityPolicy (1.25), `autoscaling/v2beta2` HPA (1.26), `storage.k8s.io/v1beta1` CSIStorageCapacity (1.27), `flowcontrol.apiserver.k8s.io/v1beta2` (1.29), `flowcontrol.apiserver.k8s.io/v1beta3` (1.32). None of them is served by 1.33.[^8]

***

#### Version-Aware Review

- **Pin the target first.** Determine the cluster minor from the deployment target (`kubeVersion` in `Chart.yaml`, CI `kind`/envtest version, `k8s.io/*` versions in `go.mod`, managed-cluster version). Review against **1.33** only when that is the detected version.
- **Flag the support status once.** 1.33 is end-of-life since 2026-06-28. Point it out (upgrade to a supported minor, one minor at a time), but review the code for 1.33 as asked — do not rewrite it for 1.37.
- **Verify before claiming "unavailable".** Before saying an API, field or function is missing, check it against 1.33: `kubectl explain <type>.<field>` on a 1.33 apiserver, `kubectl api-versions`, the 1.33 OpenAPI, or `go doc` on v0.33.x modules. Say "not available in 1.33" only when the pinned version proves it.
- **`kubectl explain` lists gated fields.** On 1.33 `kubectl explain pod.spec.containers.lifecycle.stopSignal`, `pod.status.observedGeneration` and `deployment.status.terminatingReplicas` all succeed, yet the alpha gates are off and the values are dropped or never populated. Check the gate table, not only the schema.
- **Beta-on features are usable but gated.** In-place resize, `hostUsers: false`, `procMount`, `supplementalGroupsPolicy`, zero-second sleep, `seLinuxChangePolicy` are beta and enabled by default on 1.33. Accept them, but note that a managed provider may disable beta gates and that most need node/runtime support.
- **Beta APIs are off by default.** Since 1.24 new beta *API versions* are not served unless enabled with `--runtime-config` (for example `resource.k8s.io/v1beta2`, `certificates.k8s.io/v1beta1` ClusterTrustBundle, `coordination.k8s.io/v1beta1` LeaseCandidate, `storage.k8s.io/v1beta1` VolumeAttributesClass). A manifest that uses them needs a documented cluster prerequisite.
- **Client-library skew.** GA APIs work across neighbouring minors, but a binary built with v0.34+ types can send fields a 1.33 server drops. Keep `k8s.io/*` at v0.33.x and controller-runtime at v0.21.x for 1.33 targets.
- **Do not "upgrade" users into 1.34+ behaviour.** Examples: do not recommend `trafficDistribution: PreferSameNode`, pod-level `resources`, HPA `tolerance`, image volumes, `resource.k8s.io/v1`, or `client.Client.Apply` with apply configurations.

***

#### API Versions Served by a 1.33 Cluster

`kubectl api-versions` on a stock v1.33.0 kube-apiserver (no aggregated APIs or CRDs):

```text
admissionregistration.k8s.io/v1   apiextensions.k8s.io/v1   apiregistration.k8s.io/v1
apps/v1   authentication.k8s.io/v1   authorization.k8s.io/v1   autoscaling/v1   autoscaling/v2
batch/v1   certificates.k8s.io/v1   coordination.k8s.io/v1   discovery.k8s.io/v1   events.k8s.io/v1
flowcontrol.apiserver.k8s.io/v1   networking.k8s.io/v1   node.k8s.io/v1   policy/v1
rbac.authorization.k8s.io/v1   scheduling.k8s.io/v1   storage.k8s.io/v1   v1
```

The group/version list is identical on 1.32. 1.34 adds `resource.k8s.io/v1`. Within `networking.k8s.io/v1`, 1.33 adds the GA kinds `ServiceCIDR` and `IPAddress`.

Compiled in but **disabled by default** on 1.33 (verified by starting the apiserver with `--runtime-config=api/all=true`): `admissionregistration.k8s.io/v1alpha1` (MutatingAdmissionPolicy, alpha) and `v1beta1` (the old ValidatingAdmissionPolicy beta), `coordination.k8s.io/v1alpha2` and `v1beta1` (LeaseCandidate), `internal.apiserver.k8s.io/v1alpha1`, `networking.k8s.io/v1beta1` (the old ServiceCIDR/IPAddress betas), `resource.k8s.io/v1alpha3`, `v1beta1` (deprecated) and `v1beta2`, `storage.k8s.io/v1alpha1` and `v1beta1` (VolumeAttributesClass). Groups such as `certificates.k8s.io/v1beta1` (ClusterTrustBundle) and `storagemigration.k8s.io/v1alpha1` additionally need their feature gate before `--runtime-config` serves them.

Notable GA kinds for authors: `admissionregistration.k8s.io/v1` ValidatingAdmissionPolicy(Binding) (GA 1.30); `networking.k8s.io/v1` ServiceCIDR and IPAddress (GA 1.33); `coordination.k8s.io/v1` Lease; `flowcontrol.apiserver.k8s.io/v1` (GA 1.29); `batch/v1` Job with every GA Job feature listed below.

***

#### Feature-Gate Defaults That Matter to Authors

Values read from `kubernetes_feature_enabled` on v1.32.0, v1.33.0 and v1.34.1 apiservers.

| Gate | 1.32 | **1.33** | 1.34 | Effect on manifests/controllers |
|---|---|---|---|---|
| `SidecarContainers` | beta, on | **GA** | GA | `initContainers[].restartPolicy: Always` |
| `InPlacePodVerticalScaling` | alpha, off | **beta, on** | beta, on | `resizePolicy`, `pods/resize` |
| `UserNamespacesSupport` | beta, off | **beta, on** | beta, on | `hostUsers: false` (dropped on 1.32) |
| `UserNamespacesPodSecurityStandards` | alpha, off | **alpha, off** | alpha, off | PSA still checks `runAsNonRoot` for user-namespaced Pods |
| `ProcMountType` | beta, off | **beta, on** | beta, on | `procMount: Unmasked` |
| `SupplementalGroupsPolicy` | alpha, off | **beta, on** | beta, on | `supplementalGroupsPolicy` (dropped on 1.32) |
| `PodLifecycleSleepAction` | beta, on | beta, on | GA | `lifecycle.*.sleep` |
| `PodLifecycleSleepActionAllowZero` | alpha, off | **beta, on** | GA | `sleep.seconds: 0` |
| `RelaxedDNSSearchValidation` | alpha, off | **beta, on** | GA | `.`/`_` in `dnsConfig.searches` |
| `RelaxedEnvironmentVariableValidation` | beta, on | beta, on | GA | env names: any printable ASCII except `=` |
| `SELinuxChangePolicy` | alpha, off | **beta, on** | beta, on | `seLinuxChangePolicy` |
| `SELinuxMount` | alpha, off | **beta, off** | beta, off | cluster-wide mount-option relabeling |
| `ImageVolume` | alpha, off | **beta, off** | beta, off | `volumes[].image` dropped |
| `JobBackoffLimitPerIndex`, `JobSuccessPolicy` | beta, on | **GA** | GA | Job fields |
| `JobPodReplacementPolicy` | beta, on | beta, on | GA | `podReplacementPolicy` |
| `JobManagedBy` | beta, on | beta, on | beta, on | `spec.managedBy` |
| `MatchLabelKeysInPodAffinity` | beta, on | **GA** | GA | affinity `matchLabelKeys` |
| `MatchLabelKeysInPodTopologySpread` | beta, on | beta, on | beta, on | spread `matchLabelKeys` |
| `NodeInclusionPolicyInPodTopologySpread` | beta, on | **GA** | GA | `nodeTaintsPolicy`, `nodeAffinityPolicy` |
| `ServiceTrafficDistribution` | beta, on | **GA** | GA | `trafficDistribution: PreferClose` |
| `PreferSameTrafficDistribution` | — | alpha, off | beta, on | `PreferSameZone`/`PreferSameNode` rejected on 1.33 |
| `MultiCIDRServiceAllocator` | beta, off | **GA** | GA | ServiceCIDR/IPAddress `v1` |
| `CRDValidationRatcheting` | beta, on | **GA** | GA | ratcheting |
| `RecursiveReadOnlyMounts` | beta, on | **GA** | GA | `recursiveReadOnly` |
| `OrderedNamespaceDeletion` | — | **beta, on** | GA | Pods first on namespace deletion |
| `KubeletFineGrainedAuthz` | alpha, off | **beta, on** | beta, on | `nodes/metrics` etc. |
| `PodLevelResources` | alpha, off | alpha, off | beta, on | pod-level `resources` dropped |
| `HPAConfigurableTolerance` | — | alpha, off | alpha, off (beta/on in 1.35) | HPA `tolerance` dropped |
| `HPAScaleToZero` | alpha, off | alpha, off | alpha, off | `minReplicas: 0` rejected |
| `MaxUnavailableStatefulSet` | alpha, off | alpha, off | alpha, off | `maxUnavailable` dropped |
| `ContainerStopSignals` | — | alpha, off | alpha, off | `stopSignal` dropped |
| `PodObservedGenerationTracking` | — | alpha, off | beta, on | Pod `status.observedGeneration` |
| `DeploymentReplicaSetTerminatingReplicas` | — | alpha, off | alpha, off | `status.terminatingReplicas` |
| `StrictIPCIDRValidation` | — | alpha, off | alpha, off | warnings only |
| `DynamicResourceAllocation` | beta, off | beta, off | GA | DRA APIs |
| `MutatingAdmissionPolicy` | alpha, off | alpha, off | beta, off | MAP |
| `VolumeAttributesClass` | beta, off | beta, off | GA | VAC |
| `ClusterTrustBundle`, `ClusterTrustBundleProjection` | alpha, off | **beta, off** | beta, off | trust bundles |
| `CoordinatedLeaderElection` | alpha, off | **beta, off** | beta, off | LeaseCandidate |
| `GitRepoVolumeDriver` | — | deprecated, **off** | deprecated, off | kubelet refuses `gitRepo` |
| `KubeletEnsureSecretPulledImages` | — | alpha, off | alpha, off | image re-authentication |
| `WatchList` (apiserver) | beta, on | **beta, off** | beta, on | streaming watch-list |
| `InOrderInformers` (client-go) | — | beta, on | beta, on | informers process watch events in order |

***

#### Usable Features GA in Earlier Releases (cumulative)

Everything below is GA (or beta and on, where marked) and available on 1.33; recommend it where it fits.

| Area | Feature (GA since) |
|---|---|
| Pods | Native sidecars (1.33); `recursiveReadOnly` mounts (1.33); AppArmor via `securityContext.appArmorProfile` (1.30); `schedulingGates` (1.30); pod affinity `matchLabelKeys` (1.33); topology spread `minDomains` (1.30), `nodeAffinityPolicy`/`nodeTaintsPolicy` (1.33), `matchLabelKeys` (beta/on); `lifecycle.*.sleep` (beta/on); in-place resize (beta/on); `hostUsers: false` (beta/on); `os.name` (1.25); ephemeral containers (1.25); `ReadWriteOncePod` (1.29) |
| Workloads | Job `podFailurePolicy` (1.31), `backoffLimitPerIndex` (1.33), `successPolicy` (1.33), `podReplacementPolicy` (beta/on), `managedBy` (beta/on), Indexed Jobs and the `batch.kubernetes.io/job-completion-index` label (1.24), elastic Indexed Jobs (1.31); CronJob `timeZone` (1.27); StatefulSet `persistentVolumeClaimRetentionPolicy` (1.32), `minReadySeconds` (1.25), `ordinals.start` (1.31); PDB `unhealthyPodEvictionPolicy` (1.31); Pod index label `apps.kubernetes.io/pod-index` (1.32); HPA `autoscaling/v2` (1.23) |
| Services/network | `trafficDistribution: PreferClose` (1.33); ServiceCIDR/IPAddress (1.33); EndpointSlices (1.21); dual-stack (1.23); `loadBalancerClass` (1.24); LoadBalancer `ipMode` (1.32); nftables kube-proxy (1.33); `internalTrafficPolicy` (1.26) |
| Admission | ValidatingAdmissionPolicy (1.30); webhook `matchConditions` (1.30); Pod Security Admission (1.25) |
| CRDs | CEL `x-kubernetes-validations` (1.29) with `messageExpression`, `reason`, `fieldPath`, `optionalOldSelf` (1.30+); validation ratcheting (1.33); `selectableFields` / custom field selectors (1.32); `generateName` retries (`RetryGenerateName`, 1.32) |
| API machinery | Server-side apply (1.22); consistent reads from cache (beta/on); structured authorization config (1.32); structured authentication config (beta/on); bound SA tokens with node binding (1.33); `kubectl --subresource` (1.33) |
| Storage | Volume populators (1.33), always-honor reclaim policy (1.33), `ReadWriteOncePod` (1.29), recover from expansion failure (beta/on), VolumeAttributesClass (beta, **off**) |

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
| **Caches are stale** | An informer cache can lag behind your own writes. Never treat "not in cache" as "does not exist" when you just created it; prefer deterministic names, `AlreadyExists`/`Conflict` handling, and server-side apply. |
| **Declarative policy first** | Prefer CRD schema + CEL and ValidatingAdmissionPolicy over webhooks when the logic fits; they run in-process and cannot be taken down by a crashing webhook Pod. On 1.33 there is no GA in-process *mutating* policy — defaulting still needs CRD `default`s or a mutating webhook. |

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

- **Go 1.24** or newer. `k8s.io/client-go` v0.33.x, controller-runtime v0.21.0 and controller-tools v0.18.0 all declare `go 1.24.0`. Go 1.24 itself is out of upstream support since Go 1.26 was released, which is one more reason to move off 1.33.
- **controller-runtime v0.21.x** — reconciler lifecycle, informer caches, client, webhooks. v0.20 targets 1.32, v0.22 targets 1.34.
- **controller-gen (controller-tools) v0.18.x** — CRD, RBAC, webhook and DeepCopy generation for 1.33 (v0.17 pairs with 1.32, v0.19 with 1.34).
- **envtest** — `setup-envtest use 1.33.x` so integration tests run against a 1.33 apiserver and etcd.
- **kustomize** — kubectl 1.33 embeds Kustomize v5.6.0.

```go
// go.mod for a controller that targets Kubernetes 1.33
module example.com/widget-operator

go 1.24.0

require (
	k8s.io/api v0.33.13
	k8s.io/apimachinery v0.33.13
	k8s.io/client-go v0.33.13
	sigs.k8s.io/controller-runtime v0.21.0
)
```

Review rules:
- All `k8s.io/*` staging modules must share one minor (v0.33.x). Mixing `k8s.io/api v0.33` with `k8s.io/client-go v0.34` produces compile errors or scheme mismatches, and `go get` of controller-runtime v0.22 silently raises every `k8s.io/*` module to v0.34.
- Do not import `k8s.io/kubernetes` as a library; it is not intended for import.
- Prefer the final patch (v0.33.13).
- **controller-runtime v0.21 disables client-side rate limiting by default**: `ctrl.GetConfig()`/`GetConfigOrDie()` set `QPS = -1` when the kubeconfig leaves it at 0, relying on API Priority and Fairness instead. Operators upgraded from v0.20 that depended on the old 20 QPS / 30 burst throttle must set `cfg.QPS`/`cfg.Burst` explicitly.[^9]

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
| **DNS Label** (RFC 1123) | `metadata.namespace`, container names, port names (≤15), **StatefulSet `serviceName` (enforced since 1.33)** | lowercase `a–z`, digits, `-` | start AND end with `[a-z0-9]` | 63 |
| **DNS Label** (RFC 1035, stricter) | **Service names on 1.33** | lowercase `a–z`, digits, `-` | must start with `[a-z]` | 63 |
| **Qualified Name** | label keys, annotation keys, finalizer names | name: `[A-Za-z0-9-_.]`; optional DNS-subdomain prefix + `/` | name starts/ends alphanumeric | name ≤ 63; prefix ≤ 253 |
| **Label Value** | `metadata.labels` values | `[A-Za-z0-9-_.]` or empty | alphanumeric at both ends | 63 |
| **Annotation Value** | annotation values | any UTF-8 | n/a | all annotations together ≤ 256 KiB |
| **Env var name** | `env[].name` | any printable ASCII except `=` (`RelaxedEnvironmentVariableValidation`, beta/on) | n/a | — |
| **ConfigMap / Secret data key** | `data` / `stringData` keys | `[A-Za-z0-9-_.]` | — | 253 |

Service names must still start with a letter on 1.33: `8080-web` is rejected with `a DNS-1035 label must consist of lower case alphanumeric characters or '-', start with an alphabetic character` (verified on 1.32, 1.33 and 1.34). Do flag a leading-digit Service name for a 1.33 target.

Env var names such as `my.var-1` are accepted on 1.33 (verified) because the relaxed validation is beta and on; images that read them must cope with non-identifier names.

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

// Wrong
type rediscluster struct{ /* ... */ }
type RedisClusterSpecs struct{ /* ... */ }
```

| Rule | Good | Bad |
|---|---|---|
| camelCase JSON | `replicaCount` | `replica_count`, `ReplicaCount` |
| Declarative nouns | `replicas` | `setReplicas` |
| Name references | `secretName` | `secret` |
| Object references | `secretRef` | — |
| Durations | `timeoutSeconds` (`int32`) or a `metav1.Duration` string | ad hoc `timeout: "30s"` parsed by hand |
| Established prefixes | `maxReplicas`, `minReadySeconds`, `maxSurge` | novel abbreviations like `tgtUtil`, `cfgMapName` |

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
    cache.example.com/last-backup-time: "2025-06-01T00:00:00Z"
```

When code uses `client.ObjectKey`/`types.NamespacedName`, assume `Namespace` and `Name` already satisfy the DNS rules above unless the value comes from user input.

---

#### Workload Manifests

##### A 1.33 Pod That Passes `restricted`

The following Pod was accepted by a v1.33.0 apiserver in a namespace labelled `pod-security.kubernetes.io/enforce: restricted` with `enforce-version: v1.33`. The stored object kept `hostUsers: false`, `supplementalGroupsPolicy: Strict`, `recursiveReadOnly: Enabled` and `metadata.generation: 1`. Every beta field in it is enabled by default on 1.33.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: modern
  namespace: shop
  labels: {app: web}
spec:
  hostUsers: false                       # user namespaces: beta, on by default since 1.33
  automountServiceAccountToken: false
  securityContext:
    runAsNonRoot: true                   # still required by restricted on 1.33, even with hostUsers: false
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
    supplementalGroupsPolicy: Strict     # beta, on by default since 1.33
    seccompProfile: {type: RuntimeDefault}
  initContainers:
    - name: log-shipper                  # native sidecar, GA in 1.33
      image: registry.example.com/shipper:2.3.1
      restartPolicy: Always
      startupProbe:
        exec: {command: ["/bin/shipper", "--check"]}
      securityContext:
        allowPrivilegeEscalation: false
        capabilities: {drop: ["ALL"]}
        readOnlyRootFilesystem: true
  containers:
    - name: app
      image: registry.example.com/app@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
      ports: [{name: http, containerPort: 8080}]
      resources:
        requests: {cpu: 250m, memory: 256Mi}
        limits: {memory: 256Mi}
      resizePolicy:                      # in-place resize, beta and on by default in 1.33
        - {resourceName: cpu, restartPolicy: NotRequired}
        - {resourceName: memory, restartPolicy: RestartContainer}
      lifecycle:
        preStop:
          sleep: {seconds: 5}            # sleep action: beta/on; seconds: 0 allowed since 1.33
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
        - {name: tmp, mountPath: /tmp}
        - {name: certs, mountPath: /etc/certs, readOnly: true, recursiveReadOnly: Enabled}
  volumes:
    - name: tmp
      emptyDir: {sizeLimit: 64Mi}
    - name: certs
      secret: {secretName: app-certs}
```

Review notes for this template:
- **Native sidecars (GA in 1.33)**: an init container with `restartPolicy: Always` starts before the app containers (the next init container waits for its `startupProbe`), keeps running, is restarted when it exits, and is stopped after the main containers. Flag the old pattern of a sidecar in `containers` for Jobs — it keeps the Job's Pod from completing. Probes are allowed on sidecars only; a probe on a regular init container is rejected (`may not be set for init containers without restartPolicy=Always`, verified). Named ports of sidecars are reachable through Services since 1.33.[^10]
- **`hostUsers: false`** needs node support (see Pod Security). On a node without user-namespace support the Pod fails to start; 1.33.2 improved that error message.
- **`supplementalGroupsPolicy: Strict`** makes the kubelet reject the Pod on nodes whose runtime cannot honour it (`.status.features.supplementalGroupsPolicy: false` on the Node). Check node/runtime support before rolling it out.
- **`recursiveReadOnly: Enabled`** requires `readOnly: true` (otherwise `may only be specified when readOnly is true`, verified) and a runtime that supports recursive read-only mounts; `IfPossible` degrades silently instead of failing. It is GA in 1.33.
- **Do not add `volumes[].image`** on 1.33 (dropped: beta/off), nor pod-level `spec.resources` (dropped: alpha/off), nor `lifecycle.stopSignal` (dropped: alpha/off).

##### Resource Requests, Limits and In-Place Resize **[NEW in 1.33: beta, on]**

```yaml
# BAD: no requests (BestEffort), CPU limit throttles latency-sensitive code
resources:
  limits: {cpu: 200m}

# GOOD: memory request == limit, CPU request without a limit for latency-sensitive services
resources:
  requests: {cpu: 250m, memory: 512Mi}
  limits: {memory: 512Mi}
```

- Resize goes through the **`resize` subresource** (new in 1.33): `kubectl patch pod web --subresource resize --patch '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"500m"}}}]}}'`. A plain `update`/`patch` of `spec.containers[*].resources` is still rejected. RBAC for tools that resize: `patch` (or `update`) on `pods/resize`.
- **Memory-limit decreases** are rejected unless that container's memory `resizePolicy` is `RestartContainer` (verified on 1.33; allowed on 1.34). `resizePolicy` is immutable, so decide it at creation time.
- Progress is reported by Pod conditions `PodResizePending` (reason `Deferred` or `Infeasible`) and `PodResizeInProgress`; `status.resize` is deprecated and no longer set. Controllers must read the conditions and `status.containerStatuses[*].resources`/`allocatedResources`, not `spec`, to learn what is actually applied.[^11]
- Other 1.33 limits (documented for 1.33): only CPU and memory can be resized; the QoS class cannot change (a Guaranteed Pod must keep requests == limits, a BestEffort Pod cannot gain resources); requests/limits cannot be removed once set; regular init containers and ephemeral containers cannot be resized (sidecars can); Windows Pods, Pods under static CPU/Memory manager policies, and swap-using containers (unless memory is `RestartContainer`) cannot be resized in place. `kubectl --subresource resize` needs kubectl ≥ 1.32.
- Workload controllers (Deployments etc.) never resize existing Pods; a template change still rolls the workload. VPA in-place mode needs a VPA release that supports the 1.33 `resize` subresource.

##### Deployments

The Deployment, StatefulSet and CronJob examples below leave out the security context from the Pod above to stay short. In a namespace that enforces `restricted`, the apiserver admits these controller objects but warns (`would violate PodSecurity "restricted:v1.33": ...`) and the Pods they create are then rejected — copy the `securityContext` blocks into real templates.

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
          nodeTaintsPolicy: Honor               # GA in 1.33: skip nodes whose taints the Pod does not tolerate
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                topologyKey: kubernetes.io/hostname
                labelSelector: {matchLabels: {app: web}}
                matchLabelKeys: [pod-template-hash]   # GA in 1.33
      terminationGracePeriodSeconds: 30
      containers:
        - name: web
          image: registry.example.com/web:3.1.4
          ports: [{name: http, containerPort: 8080}]
          resources:
            requests: {cpu: 100m, memory: 128Mi}
            limits: {memory: 128Mi}
          lifecycle:
            preStop: {sleep: {seconds: 0}}       # no-op hook; rejected on 1.32
```

- `spec.selector` is immutable; changing it requires a new Deployment.
- `matchLabelKeys` merges the values of the listed labels from the incoming Pod into the selector, so old and new ReplicaSets are spread (or repelled) independently during a rollout. Without it, a rolling update can be blocked by anti-affinity against Pods of the old revision.
- `nodeTaintsPolicy: Honor` stops tainted nodes the Pod cannot schedule on from counting as empty topology domains (the default `Ignore` can leave Pods pending on `DoNotSchedule` constraints).
- `preStop.sleep.seconds: 0` is accepted on 1.33 and rejected by 1.32 (`must be greater than 0 ... Enable AllowPodLifecycleSleepActionZeroValue feature gate`), so charts that also target 1.32 must use `seconds: 1` or omit the hook.
- `status.terminatingReplicas` is alpha/off on 1.33; do not build rollout gates on it.
- When an HPA manages a Deployment, do not set `replicas` in the applied manifest — every `kubectl apply`/GitOps sync resets the HPA's value.

##### StatefulSets

```yaml
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
      partition: 0             # maxUnavailable is alpha (off) on 1.33 and silently dropped
  persistentVolumeClaimRetentionPolicy:
    whenDeleted: Retain
    whenScaled: Retain
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

```yaml
# BAD on 1.33: dropped on create (MaxUnavailableStatefulSet is alpha/off) — rollouts stay one Pod at a time
updateStrategy:
  type: RollingUpdate
  rollingUpdate: {maxUnavailable: 2}
```

- Verified: `rollingUpdate.maxUnavailable: 2` is stored as `{"partition":0}` on 1.32, 1.33 and 1.34. GitOps tools that diff against the stored object show permanent drift; remove the field for 1.33 targets.
- `serviceName` must be a valid DNS-1123 label (enforced on create since 1.33).
- `persistentVolumeClaimRetentionPolicy` (GA 1.32): `whenScaled: Delete` deletes PVCs of scaled-down ordinals — flag it for databases unless data loss on scale-down is intended.
- `volumeClaimTemplates` are immutable; changing them needs an orphaning delete (`kubectl delete sts --cascade=orphan`) and re-create.

##### Jobs and CronJobs

```yaml
apiVersion: batch/v1
kind: Job
metadata: {name: train, namespace: ml}
spec:
  completions: 4
  parallelism: 4
  completionMode: Indexed
  backoffLimitPerIndex: 1           # GA in 1.33
  maxFailedIndexes: 1
  podReplacementPolicy: Failed      # beta, on: no replacement until the old Pod is fully terminated
  successPolicy:                    # GA in 1.33
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

- **`backoffLimitPerIndex`** (GA 1.33) counts failures per index; with `maxFailedIndexes` the Job fails once too many indexes failed. It requires `completionMode: Indexed`. The `FailIndex` action in `podFailurePolicy` fails a single index without retries.
- **`successPolicy`** (GA 1.33) declares the Job successful once the listed indexes (`succeededIndexes`) or a number of them (`succeededCount`) succeeded; remaining Pods are terminated. It requires `completionMode: Indexed`. Leader/worker jobs ("only index 0 matters") should use it instead of custom exit logic.
- `podReplacementPolicy: Failed` (beta/on) waits until a failed Pod has fully terminated before creating its replacement — important for workloads that cannot tolerate two copies of the same index.
- `managedBy` (beta/on) hands a Job to an external controller (for example MultiKueue); the built-in Job controller then ignores it. The field is immutable.
- While a Job is suspended its Pod template resources cannot be changed on 1.33 (that arrives in later releases).
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

Idempotent one-shot Jobs and CronJobs can rely on Kubernetes termination and safe re-runs. Require explicit graceful shutdown only when there is cleanup that process exit cannot handle safely (held leases, locks, transactions, external reservations, non-idempotent remote writes). External calls still need contexts, timeouts or `activeDeadlineSeconds`; absence of signal handling alone is not a bug. A read-only scan or idempotent garbage-collection job may simply exit on termination and let the next schedule retry.

##### HorizontalPodAutoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: {name: web, namespace: shop}
spec:
  scaleTargetRef: {apiVersion: apps/v1, kind: Deployment, name: web}
  minReplicas: 2                     # 0 is rejected on 1.33 (HPAScaleToZero is alpha)
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target: {type: Utilization, averageUtilization: 70}
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 0
      policies: [{type: Percent, value: 100, periodSeconds: 15}]
    scaleDown:
      stabilizationWindowSeconds: 300
      policies: [{type: Pods, value: 2, periodSeconds: 60}]
```

- **Do not use `behavior.scaleUp/scaleDown.tolerance` on 1.33.** The field is new but alpha (`HPAConfigurableTolerance`, off): the apiserver accepts the manifest and silently drops the value (verified), so the cluster-wide 10 % tolerance still applies. 1.32 rejects the field as unknown.
- `minReplicas: 0` fails validation (`must be greater than or equal to 1`).
- CPU-utilization targets require CPU requests on every container of the target, including sidecars.
- `kubectl autoscale` in 1.33 creates `autoscaling/v2` objects (falls back to v1).

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
- `stringData` is a write-only convenience; reviewers should not expect it in `kubectl get -o yaml`.

##### Deprecated volume types

```yaml
# BAD: gitRepo is disabled by default on 1.33 kubelets — admitted with a warning, but the Pod never starts
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

`gitRepo` has a history of node-level remote-code-execution issues (it runs `git` as root on the node). Do not ask cluster admins to re-enable `GitRepoVolumeDriver`.

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

Label every namespace explicitly and pin the policy versions[^12] so a control-plane upgrade does not silently change the policy. A `warn` or `audit` label without its `-version` label uses `latest` (verified: warnings then say `restricted:latest`).

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: shop
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: v1.33
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/warn-version: v1.33
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/audit-version: v1.33
```

##### User Namespaces (beta, on by default since 1.33) **[NEW in 1.33]**

`hostUsers: false` maps container UIDs/GIDs to an unprivileged range on the host, so root in the container is not root on the node. On 1.32 the apiserver dropped the field (gate beta/off); on 1.33 it is stored (verified). The node needs a runtime with user-namespace support (containerd ≥ 2.0 or CRI-O ≥ 1.25) and an OCI runtime that supports idmapped mounts (runc ≥ 1.2 or crun ≥ 1.9). Every filesystem used by the Pod's volumes must support idmap mounts (in practice Linux ≥ 6.3, because tmpfs — used by Secrets and projected volumes — gained it there). Pods with user namespaces may not set `hostNetwork`, `hostIPC` or `hostPID`.[^13]

**Pod Security Admission does not relax its checks for user-namespaced Pods on a stock 1.33 cluster**: the relaxation is behind `UserNamespacesPodSecurityStandards`, which is alpha and off. Verified on 1.33: this Pod is rejected by `restricted:v1.33` with `runAsNonRoot != true (pod or container "app" must set securityContext.runAsNonRoot=true)`.

```yaml
# Rejected by restricted:v1.33 — hostUsers: false does not replace runAsNonRoot on 1.33
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

- Use `hostUsers: false` **in addition to** `runAsNonRoot`, not instead of it, on 1.33.
- `procMount: Unmasked` (beta, on since 1.33) is rejected by validation unless `hostUsers: false` (`` `hostUsers` must be false to use `Unmasked` ``, verified), and Baseline/Restricted require the default `procMount` (the relaxation is behind the same alpha gate). It is meant for nested container runtimes in unprivileged Pods.
- Do not tell authors that `hostUsers: false` is GA on 1.33 (GA came in 1.36), nor that it is "alpha, off".

##### SELinux

- `securityContext.seLinuxChangePolicy` (beta, on in 1.33): `MountOption` (fast, mount with `-o context=`) or `Recursive` (relabel every file). Pods that share a volume on the same node but run with different SELinux labels must use `Recursive`.
- The cluster-wide `SELinuxMount` switch is beta and **off** in 1.33. Start auditing now: the `SELinuxWarningController` in kube-controller-manager reports conflicts, and later releases turn mount-option relabeling on for all volumes.[^14]

##### Fine-Grained Kubelet Authorization (beta, on since 1.33)

Monitoring agents should get only the kubelet subresources they need instead of `nodes/proxy` (which also allows `exec` into any container):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata: {name: node-metrics-reader}
rules:
  - apiGroups: [""]
    resources: ["nodes/metrics", "nodes/stats", "nodes/pods", "nodes/healthz"]
    verbs: ["get"]
```

Kubelet path → subresource mapping with `KubeletFineGrainedAuthz`: `/stats/*` → `nodes/stats`, `/metrics/*` → `nodes/metrics`, `/logs/*` → `nodes/log`, `/pods` and `/runningPods/` → `nodes/pods`, `/healthz` → `nodes/healthz`, `/configz` → `nodes/configz`; the kubelet falls back to `nodes/proxy` when the specific check is denied.[^15] Keep a `nodes/proxy` grant only where the agent must also run on clusters or nodes without the gate.

---

#### Networking: Services, ServiceCIDR, NetworkPolicy, Ingress

##### Services

```yaml
apiVersion: v1
kind: Service
metadata: {name: web, namespace: shop}
spec:
  selector: {app: web}
  ports: [{name: http, port: 80, targetPort: http, appProtocol: kubernetes.io/h2c}]
  trafficDistribution: PreferClose        # GA in 1.33; the only accepted value on 1.33
  ipFamilyPolicy: PreferDualStack
```

- **`trafficDistribution: PreferClose`** (GA in 1.33) prefers endpoints in the client's zone. `PreferSameZone` and `PreferSameNode` are rejected on 1.33 (`Unsupported value: "PreferSameNode": supported values: "PreferClose"`, verified). Replace the deprecated `service.kubernetes.io/topology-mode: Auto` annotation with this field.
- Use named `targetPort`s; set `appProtocol` for L7-aware proxies.
- **The `Endpoints` API is deprecated (1.33).** Reading or writing `v1` Endpoints returns a warning; controllers, scripts and monitoring should use `discovery.k8s.io/v1` EndpointSlices (one Service maps to several slices; select them with the label `kubernetes.io/service-name`). Endpoints objects written by the built-in Endpoints controller now carry a label that says so; hand-written Endpoints should become EndpointSlices.[^7]
- `spec.externalIPs` enables traffic interception (CVE-2020-8554); clusters should run the `DenyServiceExternalIPs` admission plugin, and new uses deserve a finding.

##### IP and CIDR Values

1.33 accepts non-canonical IPs and CIDRs but warns (verified):

```yaml
spec:
  loadBalancerSourceRanges:
    - "010.0.0.0/8"      # Warning: non-standard CIDR value "010.0.0.0/8" will be considered invalid in a future Kubernetes release: use "10.0.0.0/8"
    - "192.168.0.5/24"   # Warning: CIDR value "192.168.0.5/24" is ambiguous in this context (should be "192.168.0.0/24" or "192.168.0.5/32"?)
```

Treat the warnings as errors in review: later releases reject such values (`StrictIPCIDRValidation`, alpha in 1.33), and leading-zero IPv4 octets are interpreted differently by different parsers (CVE-2021-29923). Controllers that write IPs must canonicalize them (`netip.Prefix.Masked()`, `netip.Addr.String()`); 1.33's built-in controllers already do.

##### Multiple Service CIDRs (GA in 1.33) **[NEW in 1.33]**

`networking.k8s.io/v1` `ServiceCIDR` objects add ClusterIP ranges at runtime; every allocated ClusterIP is represented by an `IPAddress` object. Cluster admins should restrict who may create ServiceCIDRs (RBAC) and which ranges are allowed, for example with a ValidatingAdmissionPolicy:

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: servicecidrs-allowed-ranges
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups: ["networking.k8s.io"]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["servicecidrs"]
  validations:
    - expression: "object.spec.cidrs.all(c, cidr('10.96.0.0/12').containsCIDR(c))"
      message: ServiceCIDRs must be inside 10.96.0.0/12
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: servicecidrs-allowed-ranges
spec:
  policyName: servicecidrs-allowed-ranges
  validationActions: [Deny]
```

Verified on 1.33: a ServiceCIDR `192.168.0.0/24` is denied with `ServiceCIDRs must be inside 10.96.0.0/12`, `10.96.100.0/24` is created. Manifests that create ServiceCIDRs must not target 1.32 (the `v1` kind does not exist there; the beta was off by default).

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

- `networking.k8s.io/v1` Ingress is GA and frozen; new features land in Gateway API (CRDs from `gateway.networking.k8s.io`, installed separately).
- The community Ingress NGINX controller was retired in March 2026 (best-effort maintenance ended; no further releases, bug fixes or security fixes). Manifests that rely on `nginx.ingress.kubernetes.io/*` annotations — especially `configuration-snippet`/`server-snippet` — need a migration plan to Gateway API or a maintained controller.[^16]
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

#### Admission Control: ValidatingAdmissionPolicy and Webhooks

##### Choosing a Mechanism

```text
CRD schema + CEL (x-kubernetes-validations)   → validation of your own types, no extra objects
CRD `default` values                          → static defaults of your own types
ValidatingAdmissionPolicy (GA 1.30)           → validation of any type, in-process CEL
Mutating admission webhook                    → defaulting/mutation of other types (no GA in-process alternative on 1.33)
Validating admission webhook                  → needs external data, side effects, or logic CEL cannot express
```

Prefer the cheapest mechanism that works. In-process policies have no network hop, no TLS certificates to rotate, and no failure mode where a crashed webhook Deployment blocks every write. **MutatingAdmissionPolicy is alpha on 1.33** (`admissionregistration.k8s.io/v1alpha1`, gate off, API off): do not recommend it for a 1.33 target.

##### ValidatingAdmissionPolicy

GA since 1.30.[^17]

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

Verified on 1.33: a new Deployment with `image: registry.example.com/web:3.1.4` is rejected with `ValidatingAdmissionPolicy 'require-resources-and-digest' with binding 'require-resources-and-digest' denied request: images must be pinned by digest: web`. (`kubectl apply` of an *unchanged* object sends no request, so it is not re-validated.)

- `validationActions` combinations: `[Deny]`, `[Warn]`, `[Audit]`, `[Deny, Audit]`, `[Warn, Audit]`. `[Deny, Warn]` is rejected.
- Roll out new policies with `[Warn, Audit]` first, then switch to `Deny`.
- `matchConditions` filter before validation; they are the right place for user/namespace exemptions. Do not exempt `system:` users blindly if controllers create the objects you want to police (Deployments → ReplicaSets → Pods are created by `system:serviceaccount:kube-system:*`).
- `status.typeChecking` is filled by kube-controller-manager; read it after creating a policy to catch type errors that `failurePolicy: Ignore` would otherwise hide. Run ≥ 1.33.10: older 1.33 kube-controller-managers could crash type-checking a policy whose object schema has `additionalProperties: true`, and ≥ 1.33.7 for deterministic policy evaluation and correct behaviour in newly created namespaces.
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
- Replace simple validating webhooks with ValidatingAdmissionPolicy; keep mutating webhooks for defaulting of types you do not own (MAP is not an option on 1.33).

---

#### CRD & API Design

##### Minimal Skeleton (Go, Kubebuilder markers)

This is the Kubebuilder layout for controller-tools v0.18 / controller-runtime v0.21 (it compiles against v0.21.0; `zz_generated.deepcopy.go` comes from `controller-gen object`):

```go
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion  = schema.GroupVersion{Group: "example.com", Version: "v1"}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	AddToScheme   = SchemeBuilder.AddToScheme
)

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
	// +kubebuilder:validation:Maximum=1000
	// +required
	Replicas int32 `json:"replicas"`
	// +kubebuilder:default=10
	// +kubebuilder:validation:Maximum=1000
	// +optional
	MaxReplicas int32 `json:"maxReplicas,omitempty"`
	// +required
	Tier Tier `json:"tier"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="storageClass is immutable"
	// +kubebuilder:validation:MaxLength=253
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

func init() {
	SchemeBuilder.Register(&Widget{}, &WidgetList{})
}
```

`sigs.k8s.io/controller-runtime/pkg/scheme.Builder` is the normal, non-deprecated helper in v0.21 (it was deprecated only in later controller-runtime releases). Do not ask 1.33 projects to replace it; `runtime.NewSchemeBuilder` from apimachinery works too.

##### Spec Design Rules

- `// +optional` with `omitempty` for optional fields; `// +required` for required ones. Use pointers (`*int32`) when "unset" and "zero" must be distinguishable.
- Prefer **lists of named subobjects** with `+listType=map` / `+listMapKey=name` over maps; they merge correctly under server-side apply and strategic merge patch:

```yaml
# Preferred
ports:
  - name: redis
    port: 6379
# Avoid
ports:
  redis: 6379
```

- **Never use floating-point** in `spec`; use integers, `resource.Quantity`, or strings with units.
- **Do not use `bool` where an enum will be needed** — start with a string type alias.
- **No raw maps** except for labels, annotations and opaque user data.
- Only `metadata`, `spec`, `status` (and `apiVersion`/`kind`) at the top level.
- Every string, list and map needs `maxLength` / `maxItems` / `maxProperties` — CEL cost estimation depends on them, and unbounded fields invite abuse.
- On 1.33 the apiserver does **not** range-check `format: int32` / `int64` (later releases do): a value of 3000000000 in an `int32` field is stored and then overflows in a typed Go client. Always add explicit `minimum`/`maximum` that fit the Go type.

##### Generated CRD Schema (YAML)

The CRD below was installed on a 1.33 apiserver; the custom resources further down were validated against it.

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
                  message: maxReplicas must be greater than or equal to replicas
                  fieldPath: .maxReplicas
                  reason: FieldValueInvalid
                - rule: "!has(oldSelf.storageClass) || has(self.storageClass)"
                  message: storageClass cannot be unset once set
              properties:
                replicas:
                  type: integer
                  format: int32
                  minimum: 0
                  maximum: 1000
                maxReplicas:
                  type: integer
                  format: int32
                  minimum: 0
                  maximum: 1000
                  default: 10
                tier:
                  type: string
                  enum: [small, medium, large]
                storageClass:
                  type: string
                  maxLength: 253
                  x-kubernetes-validations:
                    - rule: "self == oldSelf"
                      message: storageClass is immutable
                region:
                  type: string
                  maxLength: 32
                  x-kubernetes-validations:
                    - rule: "!oldSelf.hasValue() || oldSelf.value() == self"
                      optionalOldSelf: true
                      message: region is immutable once set
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
                labels:
                  type: object
                  maxProperties: 16
                  additionalProperties: {type: string, maxLength: 63}
                  x-kubernetes-validations:
                    - rule: "self.all(k, v, k.startsWith('team-') || v != '')"   # two-variable comprehension, new in 1.33
                      message: only team- labels may have empty values
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
                  maxItems: 16
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

A custom resource that violates the rules is rejected with all messages at once (verified on 1.33):

```yaml
apiVersion: example.com/v1
kind: Widget
metadata: {name: w-bad, namespace: shop}
spec:
  replicas: 5
  maxReplicas: 3         # maxReplicas must be greater than or equal to replicas
  tier: small
  memory: 100Gi          # memory must be a quantity below 64Gi
  endpoint: http://api.example.com     # endpoint must be an https URL
  cidr: 2001:db8::/32    # cidr must be an IPv4 CIDR
  hostname: Bad_Host     # hostname must be a DNS-1123 subdomain
  labels: {owner: ""}    # only team- labels may have empty values
  ports: [{name: a, port: 80}, {name: b, port: 80}]   # ports must be unique
```

The error output lists `spec.maxReplicas: Invalid value: "object": maxReplicas must be greater than or equal to replicas` (the `fieldPath` moved the error from `spec` to `spec.maxReplicas`) together with the six other messages. A valid object (`replicas: 2`, `endpoint: https://api.example.com`, `cidr: 10.0.0.0/16`, `labels: {team-a: ""}`, …) is accepted, and `kubectl get widgets --field-selector spec.tier=small` filters server-side.

##### PrintColumns

Always define at least a state column (from a condition) and `Age` from `.metadata.creationTimestamp`. Use `priority: 1` for wide-only columns.

##### Field Selectors on CRDs

`selectableFields` (GA 1.32) lets clients and informers filter server-side: `kubectl get widgets --field-selector spec.tier=small`, or `cache.Options.ByObject[...].Field` in controller-runtime. Up to 8 fields; only string, integer and boolean leaf fields.

---

#### CEL in CRDs and Policies

##### Library Availability on 1.33

Verified by creating CRDs with each function on v1.32.0, v1.33.0 and v1.34.1 apiservers:

| Function family | 1.32 | **1.33** | 1.34 |
|---|---|---|---|
| Standard macros (`all`, `exists`, `exists_one`, `map`, `filter`), `has`, optionals (`optionalOldSelf`, `.?field`, `optional.of`) | yes | yes | yes |
| Kubernetes list (`sum`, `min`, `max`, `indexOf`, `isSorted`), regex (`find`, `findAll`), URL, quantity, IP/CIDR (`isIP`, `ip()`, `isCIDR`, `cidr().containsCIDR`), `format.*` (`format.dns1123Label()`, `format.dns1123Subdomain()`, `format.named(...)`), `sets.*`, extended strings (`lowerAscii`, `trim`, `split`, `join`, `quote`) | yes | yes | yes |
| **Two-variable comprehensions** `all(i, v, …)`, `exists(i, v, …)`, `existsOne(i, v, …)`; **`transformList`, `transformMap`, `transformMapEntry`** | no | **yes [NEW in 1.33]** | yes |
| Semver (`isSemver`, `semver(...)`, `semver(s, true)`) | no | **no** | yes |
| `string.reverse()`, list `sort()` / `slice()`, `base64.*`, `math.*` | no | no | no |

On maps, two-variable comprehensions bind key and value (`self.all(k, v, ...)`); on lists, index and element (`self.all(i, p, ...)`). They replace awkward index arithmetic for uniqueness and cross-entry rules — but every nested comprehension multiplies the estimated cost by `maxItems`/`maxProperties`, so keep those bounds small.[^18]

##### Transition Rules and Immutability

```yaml
storageClass:
  type: string
  maxLength: 253
  x-kubernetes-validations:
    - rule: "self == oldSelf"               # transition rule: only evaluated on UPDATE when the field existed
      message: storageClass is immutable
```

- A transition rule (`oldSelf`) is skipped on CREATE and when the old object lacked the field. `self == oldSelf` alone does **not** prevent *removing* the field; add a parent-level rule such as `!has(oldSelf.storageClass) || has(self.storageClass)`. Verified on 1.33: removing the field is rejected with `storageClass cannot be unset once set`, changing it with `spec.storageClass: Invalid value: "string": storageClass is immutable`.
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

Verified on 1.33: setting `region` on an existing object without it succeeds; changing it afterwards fails with `region is immutable once set`.

##### Cost Budget Pitfalls

- Unbounded strings/lists/maps make the **estimated** cost explode and the CRD is rejected at creation with `estimated rule cost exceeds budget by factor of more than 100x`. Verified on 1.33: even `self.split(',').join('-')` on a 64-character string is rejected for cost, because the result list of `split` is unbounded. Add `maxLength`, `maxItems`, `maxProperties`, and avoid building large intermediate lists.
- Prefer a constant `message` (plus `fieldPath`) over a `messageExpression` that converts numbers with `string(...)`; such expressions are costed pessimistically.
- `x-kubernetes-int-or-string` fields need `maxLength` before `string(self)` can be used cheaply.
- Rules on items of large arrays are multiplied by `maxItems`; push rules to the narrowest level.

##### `messageExpression`, `reason`, `fieldPath`

- `reason` is one of `FieldValueInvalid` (default), `FieldValueForbidden`, `FieldValueRequired`, `FieldValueDuplicate`; it sets the HTTP 422 cause type.
- `fieldPath` points the error at a sub-field (`.maxReplicas`), so `kubectl` shows `spec.maxReplicas: Invalid value: ...` instead of `spec: Invalid value`.
- `messageExpression` must return a string; if it errors, `message` is used.

##### Validation Ratcheting (GA in 1.33) **[NEW in 1.33]**

Unchanged fields of existing objects are not re-validated against tightened schema rules on update (including CEL rules without `oldSelf`), and 1.33 extends ratcheting to the `status` subresource. This makes adding constraints to a CRD safe for stored objects, but **do not rely on ratcheting to justify tightening validation in a served version** — new objects and changed fields still fail, which breaks clients.[^19]

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

Use `r.Status().Patch(ctx, obj, client.MergeFrom(orig))` when several actors write status, to avoid conflicts on unrelated fields. (`r.Status().Apply` with apply configurations does not exist in controller-runtime v0.21.)

##### observedGeneration

Set `status.observedGeneration = metadata.generation` after a successful reconcile, and set `ObservedGeneration` on every condition you write. Tools (`kubectl wait`, Argo CD, Flux) use it to tell whether the status describes the latest spec. Pods have `metadata.generation` since 1.33, but the kubelet does not fill Pod `status.observedGeneration` on a stock 1.33 cluster (alpha gate off) — do not wait on it.

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

Standard condition types: `Ready`/`Available` (True when healthy), `Progressing`/`Reconciling` (True while working), `Degraded`/`Stalled` (True when something is wrong). Pick one polarity per type and document it.

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
8. After changing the storage version, rewrite stored objects (kube-storage-version-migrator or a read/write loop) and then remove the old version from `status.storedVersions`. The in-tree `storagemigration.k8s.io/v1alpha1` API is alpha and off on 1.33.

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

#### Controller / Reconciler Patterns (controller-runtime v0.21)

##### Reconciler Skeleton

Compiles against controller-runtime v0.21.0 and k8s.io/* v0.33.13; the unit test in the Testing section runs it twice against the fake client.

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
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	examplev1 "example.com/widget-operator/api/v1"
)

const widgetFinalizer = "example.com/external-cleanup"

// WidgetReconciler reconciles Widget objects.
type WidgetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
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
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

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

	// 4. Create or update the owned Deployment; mutate only the fields we own.
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: w.Name, Namespace: w.Namespace}}
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		if dep.Labels == nil {
			dep.Labels = map[string]string{}
		}
		dep.Labels["app.kubernetes.io/managed-by"] = "widget-controller"
		dep.Spec.Replicas = ptr.To(w.Spec.Replicas)
		if dep.CreationTimestamp.IsZero() { // selector is immutable: set it only on create
			dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"widget": w.Name}}
		}
		dep.Spec.Template.Labels = map[string]string{"widget": w.Name}
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			dep.Spec.Template.Spec.Containers = []corev1.Container{{Name: "widget"}}
		}
		dep.Spec.Template.Spec.Containers[0].Image = "registry.example.com/widget:1.0"
		return controllerutil.SetControllerReference(w, dep, r.Scheme)
	})
	if err != nil {
		return r.fail(ctx, w, "DeploymentFailed", err)
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
			r.Recorder.Eventf(w, corev1.EventTypeNormal, "Reconciled", "Deployment %s is up to date", w.Name)
		}
	}
	log.V(1).Info("reconciled", "deployment", op)
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
	r.Recorder.Event(w, corev1.EventTypeWarning, reason, err.Error())
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
- **`controllerutil.CreateOrUpdate`** reads the object (from the cache), runs the mutate function, and only issues an `Update` when something changed. The mutate function must be deterministic and touch only fields the controller owns; overwriting the whole `Spec` wipes defaults and other actors' fields (HPA replicas, injected sidecars) and causes update loops. Never change immutable fields (Deployment `selector`) after creation.
- If an HPA scales the Deployment, do not set `Replicas` in the mutate function.
- Status writes go through `r.Status()`; check `meta.SetStatusCondition`'s return value to avoid write storms and duplicate Events.
- Never block in `Reconcile` (no `time.Sleep`, no unbounded external calls without a context deadline).

##### Server-Side Apply in controller-runtime v0.21

Server-side apply is the most robust way to own child objects (no read-modify-write, no lost updates, other managers keep their fields). In v0.21 it is a patch type on the regular client:

```go
// applyDeployment owns the Deployment through server-side apply (controller-runtime v0.21 form).
func (r *WidgetReconciler) applyDeployment(ctx context.Context, w *examplev1.Widget) error {
	dep := &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"}, // required for apply
		ObjectMeta: metav1.ObjectMeta{Name: w.Name, Namespace: w.Namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(w.Spec.Replicas),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"widget": w.Name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"widget": w.Name}},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name: "widget", Image: "registry.example.com/widget:1.0",
				}}},
			},
		},
	}
	if err := controllerutil.SetControllerReference(w, dep, r.Scheme); err != nil {
		return err
	}
	return r.Patch(ctx, dep, client.Apply, client.FieldOwner("widget-controller"), client.ForceOwnership)
}
```

- Set `TypeMeta` (apply needs `apiVersion`/`kind` in the body) and a stable `client.FieldOwner`.
- **Typed structs are a trap**: a Go struct serializes zero values of non-`omitempty` fields — a bare `appsv1.Deployment` marshals to `{"metadata":{"creationTimestamp":null},"spec":{"selector":null,"template":{"metadata":{"creationTimestamp":null},"spec":{"containers":null}},"strategy":{}},"status":{}}` (verified) — and the apply request then carries those fields. Build only the fields you own; for complex objects use `unstructured.Unstructured` or client-go's typed apply configurations (`k8s.io/client-go/applyconfigurations/...`) converted to an `unstructured` object or sent with the client-go clientset's `Apply` method.
- `client.Client.Apply(ctx, applyConfiguration, ...)` does not exist before controller-runtime v0.22. Do not suggest it for a v0.21 project.
- **The v0.21 fake client cannot apply** (verified in a unit test): when the object does not exist yet, `r.Patch(ctx, obj, client.Apply)` returns a NotFound error (`deployments.apps "w" not found`) instead of creating it; when it exists, it fails with `apply patches are not supported in the fake client. Follow https://github.com/kubernetes/kubernetes/issues/115598 for the current status`. Reconcilers that use SSA must be tested with envtest, or the apply must sit behind an interface.

##### Reconcile Return Conventions

| Return | Meaning |
|---|---|
| `ctrl.Result{}, nil` | Success; wait for the next watch event |
| `ctrl.Result{}, err` | Transient failure; requeued with per-item exponential backoff (5 ms doubling, capped at 1000 s by the default rate limiter) |
| `ctrl.Result{RequeueAfter: d}, nil` | Poll again after `d` (external state you cannot watch) |
| `ctrl.Result{Requeue: true}, nil` | **Deprecated in v0.21** (`Deprecated: Use RequeueAfter instead`); staticcheck reports SA1019 |

Never return both a non-zero `Result` and an error — controller-runtime ignores the `Result` (and logs a warning) when the error is non-nil.

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

	cfg := ctrl.GetConfigOrDie() // QPS -1: client-side rate limiting disabled since v0.21
	managedBy := labels.SelectorFromSet(labels.Set{"app.kubernetes.io/managed-by": "widget-controller"})
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:    ":8443",
			SecureServing:  true,
			FilterProvider: filters.WithAuthenticationAndAuthorization,
		},
		HealthProbeBindAddress:  ":8081",
		LeaderElection:          true,
		LeaderElectionID:        "widget-controller.example.com",
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
		Recorder: mgr.GetEventRecorderFor("widget-controller"), // core/v1 Events
		External: newCloudClient(),                             // must not be nil: the deletion path calls it
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

- **Events**: `mgr.GetEventRecorderFor(name)` returns a `record.EventRecorder` that writes core `v1` Events (`Event`, `Eventf(obj, type, reason, messageFmt, args...)`). RBAC needs `create;patch` on `events` in the core group (`""`). The events.k8s.io recorder (`GetEventRecorder`) only arrived in controller-runtime v0.23.
- **Rate limiting**: since v0.21 `GetConfig*` disables client-side throttling (`QPS = -1`) unless the kubeconfig sets a QPS. That is intended (API Priority and Fairness protects the apiserver), but a buggy hot loop now hits the apiserver at full speed — reviewers should look harder for requeue loops.
- **Priority queue**: available but **off** by default in v0.21 (`config.Controller{UsePriorityQueue: ptr.To(true)}` / `controller.Options`). When enabled, initial-list events get low priority so new changes are processed first.
- Cache only what you need: label/field selectors per type (`cache.ByObject`), `DefaultTransform: cache.TransformStripManagedFields()` to cut memory, `DefaultNamespaces` for namespace-scoped operators.
- Serve metrics over HTTPS with authn/authz (`SecureServing: true` plus `filters.WithAuthenticationAndAuthorization`), not plain `:8080`.
- Enable leader election for every Deployment with more than one replica; set `LeaderElectionNamespace` when running outside the cluster.

##### controller-runtime v0.20 → v0.21 Migration Checklist

| Change in v0.21 | Review impact |
|---|---|
| k8s.io/* v0.33, Go 1.24 | bump toolchain and all `k8s.io/*` modules together |
| Client-side rate limiting off by default | set `cfg.QPS`/`cfg.Burst` if the old 20/30 throttle was relied on |
| `Result.Requeue` deprecated | replace with `RequeueAfter` or return an error |
| Priority queue: priority set automatically by handlers, `IsInInitialList` on create events; builder option `WithLowPriorityWhenUnchanged` removed | only matters when `UsePriorityQueue` is enabled |
| `NewUnmanaged`/`NewTypedUnmanaged` no longer take a manager | update call sites |
| All Go runtime metrics are exposed | dashboards may see new `go_*` series |
| Multi-handlers return warnings; `WarningHandlerWithContext` | admission warnings from chained handlers are no longer lost |

---

#### Watches, Predicates, Caches and Informers (client-go v0.33)

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
- Label, annotation and finalizer changes do not bump `metadata.generation`, so `GenerationChangedPredicate` filters them out — including the controller's own finalizer `Update`. Never `return` after adding a finalizer and wait for "the next event"; continue reconciling in the same pass.
- **Pods have a generation since 1.33.** A predicate on Pods that compares generations now sees spec changes (resize, `activeDeadlineSeconds`, image updates) — useful, but Pods' status changes still do not bump it.

##### Custom Predicate

```go
var finalizerOrSpec = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration() ||
			!reflect.DeepEqual(e.ObjectNew.GetFinalizers(), e.ObjectOld.GetFinalizers())
	},
	CreateFunc:  func(event.CreateEvent) bool { return true },
	DeleteFunc:  func(event.DeleteEvent) bool { return true },
	GenericFunc: func(event.GenericEvent) bool { return false },
}
```

##### Fan-Out Watches

```go
Watches(
	&corev1.Secret{},
	handler.EnqueueRequestsFromMapFunc(r.findWidgetsForSecret),
	builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
)
```

Back the map function with a field index (`mgr.GetFieldIndexer().IndexField(...)`, registered before the manager starts) instead of listing everything.

##### Informers and Clients in client-go v0.33

- **`InOrderInformers` (beta, on in client-go v0.33)**: informers process watch events in the order received instead of grouping updates for the same key. Binaries that embed client-go can turn it off with the environment variable `KUBE_FEATURE_InOrderInformers=false` if a handler relied on the old grouping.
- `WatchListClient` (streaming initial lists for informers) is beta and **off** in client-go v0.33, and the 1.33 apiserver disables `WatchList` too; do not enable it client-side for 1.33 clusters.
- **Fake clientset**: `fake.NewSimpleClientset` is deprecated; `fake.NewClientset()` adds field management and supports server-side apply in tests.
- `client-go/rest` supports contextual logging; `BackoffManagerWithContext` replaces `BackoffManager` so callers can interrupt backoff sleeps.
- Use the typed workqueue (`workqueue.NewTypedRateLimitingQueue[T]`) for hand-written controllers; the untyped constructors are deprecated.

##### Stale Caches

- After creating a child, do not create it again just because the cache does not show it yet. Use deterministic names and treat `AlreadyExists` as success, or use server-side apply.
- For "create N children" logic, use expectations (record what you created; skip reconciliation until the cache shows it) or count via the API reader (`mgr.GetAPIReader()`) for the critical decision.
- Do not delete "orphans" based on a cache that may be behind your own creates.

---

#### Finalizers

##### When to Use

Use finalizers **only** for cleanup that garbage collection cannot do: cloud resources, DNS records, external databases, cross-namespace or cluster-scoped children of namespaced owners. Do not add finalizers for in-cluster children that `ownerReferences` already cover.

##### Naming Rule

Finalizer names must be qualified names with a domain prefix: `example.com/external-cleanup`, not `cleanup`. The apiserver warns about unqualified finalizers on custom resources.

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
- Do not create objects in a namespace that is terminating. With ordered namespace deletion (beta, on in 1.33) Pods are deleted before the other resources of the namespace, so a controller that recreates Pods during namespace deletion fights the deletion.
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
| Owner reference `apiVersion` must be `group/version` (`v1` for core) | A wrong `apiVersion` breaks GC |
| `blockOwnerDeletion: true` needs `update` on `<owner>/finalizers` | Hence the `widgets/finalizers` RBAC rule |

| Scenario | Use |
|---|---|
| Child resource in same namespace | `ownerReferences` (automatic GC) |
| Cluster-scoped child of a namespaced owner | finalizer on the owner (or a cluster-scoped owner) |
| External resource (cloud, DNS) | finalizer |
| Cross-namespace cleanup | finalizer |

---

#### Validation & Admission Webhooks in Go

controller-runtime v0.21 webhooks use the untyped `admission.CustomValidator` / `admission.CustomDefaulter` interfaces and receive `runtime.Object`:

```go
// WidgetValidator validates Widgets (controller-runtime v0.21: untyped CustomValidator).
type WidgetValidator struct{}

var _ admission.CustomValidator = &WidgetValidator{}

func (v *WidgetValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	w, ok := obj.(*examplev1.Widget)
	if !ok {
		return nil, fmt.Errorf("expected a Widget, got %T", obj)
	}
	if w.Spec.Tier == examplev1.TierLarge && w.Spec.Replicas < 3 {
		return nil, fmt.Errorf("tier large requires at least 3 replicas")
	}
	return nil, nil
}

func (v *WidgetValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, newObj)
}

func (v *WidgetValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// SetupWidgetWebhook registers the validating webhook.
func SetupWidgetWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&examplev1.Widget{}).
		WithValidator(&WidgetValidator{}).
		Complete()
}
```

- Always check the type assertion; an unchecked `obj.(*examplev1.Widget)` panics (the webhook recovers panics by default, but the request fails).
- Do not "upgrade" this to `ctrl.NewWebhookManagedBy(mgr, &examplev1.Widget{})` / `admission.Validator[*Widget]` — that generic API is controller-runtime v0.23+ and does not compile with v0.21.
- The rule above is expressible in CEL (`self.tier != 'large' || self.replicas >= 3`) — prefer the CRD rule and drop the webhook.
- Webhook logic must be deterministic and fast; no writes to the cluster from a validating webhook (`sideEffects: None`).
- Defaulting webhooks must be idempotent; prefer `+kubebuilder:default` markers for static defaults.
- `WithCustomPath` is deprecated; keep the default paths (`/validate-<group>-<version>-<kind>`) that the generated `ValidatingWebhookConfiguration` expects.

---

#### RBAC & Least Privilege

1. Grant only the verbs and resources used; no `*` verbs or resources.
2. Prefer `Role`/`RoleBinding` over cluster-wide bindings when the operator is namespace-scoped.
3. No `cluster-admin` service accounts; no `escalate`, `bind`, or `impersonate` unless the operator's job is RBAC management.
4. Status and finalizers are separate subresources: `widgets/status` (`get;update;patch`), `widgets/finalizers` (`update`).
5. Events: core group `""` `events` `create;patch` for the v0.21 recorder (`GetEventRecorderFor`).
6. `automountServiceAccountToken: false` on the ServiceAccount; opt in on the Pod that needs API access.
7. Tools that resize Pods need `patch` on **`pods/resize`** (new subresource in 1.33); `pods` `patch` alone is not enough, and granting `pods/resize` lets the holder change CPU/memory of running Pods — treat it like a quota-relevant permission.
8. Node monitoring: use the fine-grained kubelet subresources (`nodes/metrics`, `nodes/stats`, `nodes/pods`, `nodes/healthz`) instead of new `nodes/proxy` grants.
9. DRA on 1.33 (beta, off by default): the aggregated `edit` role now reads `resourceclaims`, `resourceclaims/status` and `resourceclaimtemplates` and writes `resourceclaims` and `resourceclaimtemplates`; the `adminAccess` field (alpha, `DRAAdminAccess` off) is restricted to specially labelled admin namespaces.

```go
// +kubebuilder:rbac:groups=example.com,resources=widgets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=example.com,resources=widgets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=example.com,resources=widgets/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
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
- On 1.33 the kubelet does **not** re-verify pull credentials for images already on the node (`KubeletEnsureSecretPulledImages` is alpha/off): a Pod without pull secrets can start from a private image another Pod pulled with `IfNotPresent`. Multi-tenant clusters should use `imagePullPolicy: Always` (or the `AlwaysPullImages` admission plugin) for private images.

##### Secrets

- Never embed secrets in CRD `spec`, annotations, labels, env literals or ConfigMaps; reference Secrets by name.
- Do not log secret values or whole objects that contain them.
- Prefer projected ServiceAccount tokens (bounded audience and lifetime) over long-lived token Secrets; 1.33 tokens carry a JTI and node binding information that audit logs can correlate.
- Consider External Secrets Operator or Vault Agent for rotation.

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
log.Error(err, "failed to update Deployment", "deployment", name)

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

- controller-runtime exports `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds` and workqueue metrics. Do not duplicate them.
- Never use object names or UIDs as label values on high-volume metrics (cardinality).
- 1.33 removed the scheduler metric `pod_scheduling_duration_seconds` (use `pod_scheduling_sli_duration_seconds`) and deprecated `scheduler_scheduler_cache_size` (use `scheduler_cache_size`); update dashboards.

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
- With the v0.21 recorder: `Recorder.Event(obj, corev1.EventTypeWarning, "ReconcileError", msg)`; `reason` is CamelCase.
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
- Never `return ctrl.Result{RequeueAfter: d}, err` or `ctrl.Result{Requeue: true}, err` — the result is ignored when an error is returned.
- `client.IgnoreNotFound(err)` on the initial `Get`.
- Handle `Conflict` on `Update` by requeueing (or switch to `Patch` with `client.MergeFrom` / server-side apply to avoid conflicts).
- The default per-item backoff caps at 1000 s (~16.7 min); do not add parallel custom retry loops inside `Reconcile`.
- The apiserver can answer `429 Too Many Requests` (API Priority and Fairness); client-go honours `Retry-After` and retries. With v0.21's disabled client-side rate limiter this is the main throttle.

---

#### Testing

##### Testing Pyramid

```text
                    ┌──────────────────────────┐
                    │    e2e (kind 1.33)       │  full operator in a real cluster
                    ├──────────────────────────┤
                    │  integration (envtest)   │  real 1.33 apiserver + etcd, no kubelet
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
	rec := record.NewFakeRecorder(10)
	r := &WidgetReconciler{Client: c, Scheme: scheme, Recorder: rec, External: noopExternal{}}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "w"}}

	for i := 0; i < 2; i++ { // 1st adds finalizer and creates, 2nd must be a no-op
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

- The controller-runtime v0.21 fake client does **not** support server-side apply, admission, CEL validation, defaulting or garbage collection. Use envtest for anything that depends on the apiserver.
- `record.NewFakeRecorder` matches the v0.21 recorder interface (`record.EventRecorder`).
- Without `WithStatusSubresource`, `Status().Update` on the fake client fails with a NotFound error (verified: `widgets.example.com "w" not found` although the object exists).

##### Integration Tests (envtest)

- Pin envtest binaries to 1.33 (`setup-envtest use 1.33.x`); CEL rules (two-variable comprehensions!), PSA behaviour, ServiceCIDR `v1`, dropped alpha fields and the resize subresource only behave like production when the test apiserver is 1.33.
- envtest has no controllers: no garbage collection, no namespace finalization, no Deployment → ReplicaSet → Pod chain, no `default` ServiceAccount (creating a Pod fails with `serviceaccount "default" not found` until you create one). Assert on the objects your controller creates, not on Pods.
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

- `kubectl apply --dry-run=server -f <dir>` against a 1.33 apiserver (envtest or kind) catches unknown fields, removed API versions, CEL failures, admission policy rejections and PSA violations. `--dry-run=client` catches none of these.
- **Dry-run does not catch dropped fields.** Gated fields (image volumes, pod-level resources, HPA `tolerance`, `stopSignal`, StatefulSet `maxUnavailable`) pass validation and disappear silently. Compare the object returned by `kubectl create --dry-run=server -o yaml` with the input, or diff after apply.
- Keep server-side field validation strict (`kubectl` default `--validate=strict`); `--validate=false` silently drops fields the server does not know (verified with `spec.hostnameOverride` on 1.33).
- For Helm: `helm template ... | kubectl apply --dry-run=server -f -`, or `helm install --dry-run=server`.

##### Testing Rules

- Reconcile twice and assert the second run is a no-op (no writes, no Events).
- Test the deletion path including finalizer removal.
- Test status on success and error paths, including `observedGeneration`.
- Test that invalid objects are rejected by CEL/VAP/webhooks.

---

#### kubectl 1.33 for Reviewers

| Command | Note |
|---|---|
| `kubectl get/patch/edit/apply/replace --subresource status\|scale\|resize` | **[NEW in 1.33]** `--subresource` is GA |
| `kubectl patch pod x --subresource resize --patch '...'` | in-place resize (beta, on); needs kubectl ≥ 1.32 |
| `KUBECTL_KUBERC=true kubectl --kuberc ~/.kube/kuberc ...` | **[NEW in 1.33]** alpha: aliases and default flags in `~/.kube/kuberc`, separate from kubeconfig. Do not commit team tooling that depends on it |
| `kubectl wait --for=create` | waits for an object to appear; 1.33 fixed it for label selectors |
| `kubectl autoscale` | creates `autoscaling/v2` HPAs (falls back to v1) |
| `kubectl apply` with `labels: {a: null}` | 1.33 coerces `null` to `""` instead of dropping all labels |
| `kubectl debug` | default profile is still `legacy`; pass `--profile=general` (or `baseline`/`restricted`) explicitly |
| `kubectl explain --recursive` | long form only (no `-R` in 1.33) |

Not in kubectl 1.33: `-o kyaml`, `kubectl diff --show-secrets`, `kubectl explain -R`.

---

#### Removed / Changed in 1.33 — Migration Notes

Upgrading manifests, charts and controllers from 1.32 to 1.33:

1. **Sidecars** — native sidecars are GA; migrate "sidecar in `containers`" patterns (especially in Jobs) to `initContainers` with `restartPolicy: Always`. Remove explicit `SidecarContainers` feature-gate settings (locked on, removal planned).
2. **In-place resize** — switch tooling from patching `spec.containers[*].resources` (never allowed) or alpha-era flows to the `pods/resize` subresource; read `PodResizePending`/`PodResizeInProgress` instead of `status.resize`; grant `pods/resize` RBAC; plan memory-limit decreases with `resizePolicy: RestartContainer`.
3. **User namespaces / procMount / supplemental groups** — `hostUsers: false`, `procMount`, `supplementalGroupsPolicy` are now kept by the apiserver. Manifests that carried them "for later" now take effect: check node kernel/runtime support before upgrading, and keep `runAsNonRoot` (PSA does not relax for user namespaces on 1.33).
4. **`gitRepo` volumes** — disabled on kubelets by default; replace with an init container or git-sync.
5. **Endpoints** — move readers/writers to EndpointSlices; the deprecation warning appears in every client call.
6. **`topology-mode` annotation** — replace with `trafficDistribution: PreferClose`.
7. **ServiceCIDR** — `networking.k8s.io/v1` GA; admins should add RBAC/VAP guardrails for who may add ranges.
8. **StatefulSet `serviceName`** — must be a valid DNS-1123 label for new objects.
9. **CEL** — two-variable comprehensions and `transform*` are now available; CRDs that use them cannot be installed on 1.32 (keep 1.32-compatible charts free of them).
10. **Removed feature gates and flags** — drop `AppArmor`, `AppArmorFields`, `CPUManager`, `JobPodFailurePolicy`, `PDBUnhealthyPodEvictionPolicy`, `PersistentVolumeLastPhaseTransitionTime`, `KubeProxyDrainingTerminatingNodes`, `DisableCloudProviders`, `DisableKubeletCloudCredentialProviders` from component configs, and `--cloud-provider`/`--cloud-config` from kube-apiserver; components refuse to start otherwise.
11. **DRA** — move `resource.k8s.io/v1beta1` manifests to `v1beta2` (both off by default); `All`-device requests no longer schedule onto nodes without devices.
12. **Go modules** — Go 1.24, `k8s.io/*` v0.33.x, controller-runtime v0.21.0, controller-tools v0.18.0; replace `Result{Requeue: true}`; decide on client-side rate limiting; replace `fake.NewSimpleClientset`.
13. **Metrics** — `pod_scheduling_duration_seconds` removed; `scheduler_scheduler_cache_size` deprecated.
14. **Patch level and support** — run v1.33.13; better, continue to 1.34 and beyond, because 1.33 receives no fixes after 2026-06-28.

***

#### Production / Review Checklist

##### Version and Compatibility

- [ ] Target cluster minor confirmed as 1.33; EOL status noted with an upgrade path
- [ ] Nothing from 1.34+ or gated off on 1.33: `resource.k8s.io/v1`, MutatingAdmissionPolicy, `PreferSameZone`/`PreferSameNode`, `hostnameOverride`, `fileKeyRef`, `restartPolicyRules`, pod-level `resources`, HPA `tolerance`, image volumes, StatefulSet `maxUnavailable`, CEL semver
- [ ] No removed API versions (`extensions/v1beta1`, `batch/v1beta1`, `policy/v1beta1`, `autoscaling/v2beta2`, `flowcontrol.apiserver.k8s.io/v1beta3`, …)
- [ ] Beta API versions only with a documented `--runtime-config` prerequisite
- [ ] No `gitRepo` volumes; no direct `Endpoints` usage in new code
- [ ] IPs and CIDRs canonical (no leading zeros, no host bits) — 1.33 only warns
- [ ] Manifests pass `kubectl apply --dry-run=server` on a 1.33 apiserver with strict field validation, and the returned object still contains every field (no silent drops)

##### Workloads

- [ ] Requests on every container; memory limit == request for predictable OOM behaviour
- [ ] Liveness/readiness (and startup where slow) probes on named ports; liveness does not depend on downstream services
- [ ] PDB for replicated workloads, `unhealthyPodEvictionPolicy: AlwaysAllow`, never blocks drains
- [ ] Topology spread or anti-affinity for HA; `matchLabelKeys: [pod-template-hash]`
- [ ] Images pinned by digest; no `:latest`
- [ ] Sidecars as `initContainers` with `restartPolicy: Always`
- [ ] `resizePolicy` chosen deliberately (memory `RestartContainer` if limits may shrink)
- [ ] Jobs: `podFailurePolicy`/`backoffLimitPerIndex`/`successPolicy` where they fit; `ttlSecondsAfterFinished`
- [ ] CronJobs: `timeZone`, `concurrencyPolicy`, `startingDeadlineSeconds`
- [ ] HPA-managed workloads do not also set `replicas` through apply

##### Security

- [ ] Namespace PSA labels with pinned `enforce-version`, `warn-version`, `audit-version`; `restricted` for application namespaces
- [ ] `runAsNonRoot`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, `readOnlyRootFilesystem`, `seccompProfile: RuntimeDefault`; `hostUsers: false` as an addition where nodes support it
- [ ] `procMount: Unmasked` only with `hostUsers: false` and only in privileged-profile namespaces on 1.33
- [ ] RBAC: no wildcards; status/finalizers subresources; core `events` for the v0.21 recorder; `pods/resize` only where needed; no new `nodes/proxy`
- [ ] `automountServiceAccountToken: false` unless the Pod calls the API
- [ ] Secrets referenced, never inlined; not logged
- [ ] NetworkPolicies: default deny plus explicit allows including DNS
- [ ] Private images in multi-tenant clusters pulled with `Always` (credential re-verification is alpha on 1.33)

##### CRDs and Policies

- [ ] Kind CamelCase singular; group is a domain you own (not `*.k8s.io`)
- [ ] Status subresource enabled; `observedGeneration` and `conditions` (`listType=map`, key `type`)
- [ ] All strings/lists/maps bounded (`maxLength`, `maxItems`, `maxProperties`)
- [ ] Integer fields have `minimum`/`maximum` matching the Go type (no `format` range check on 1.33)
- [ ] CEL rules for cross-field invariants; immutability via transition rules plus "cannot unset" parent rule; `fieldPath`/`reason` set; only CEL functions available on 1.33
- [ ] Exactly one storage version; deprecated versions carry `deprecationWarning`
- [ ] ValidatingAdmissionPolicy rolled out with `[Warn, Audit]` before `Deny`; never `[Deny, Warn]`
- [ ] Webhooks: `sideEffects: None`, short `timeoutSeconds`, `matchConditions`, namespace exclusions for the webhook's own namespace, rotated TLS

##### Controllers

- [ ] Go 1.24+, `k8s.io/*` v0.33.x (all the same), controller-runtime v0.21.x, controller-tools v0.18.x, envtest 1.33
- [ ] Fetch → `IgnoreNotFound` → deletion/finalizer → ensure finalizer → create/update or apply children → status
- [ ] Children owned via `SetControllerReference`; `CreateOrUpdate` mutates only owned fields, or SSA with a stable field owner and minimal objects
- [ ] Status via `Status().Update/Patch`; skip no-op writes; `ObservedGeneration` on conditions
- [ ] Transient errors returned; permanent errors recorded as conditions without hot-looping; no `Requeue: true`; never a `Result` together with an error
- [ ] Predicates appropriate: `GenerationChangedPredicate` on `For`, not on `Owns` whose status matters
- [ ] Cache scoped (`ByObject` selectors, `TransformStripManagedFields`)
- [ ] No reliance on the cache reflecting the controller's own writes immediately
- [ ] Client-side rate limiting decision explicit (v0.21 default is off)
- [ ] Events via `GetEventRecorderFor`, only on transitions
- [ ] Leader election on; metrics served securely; healthz/readyz wired
- [ ] Unit tests (fake client with `WithStatusSubresource`) prove idempotency; SSA and CEL/admission behaviour covered by envtest

##### Observability

- [ ] Structured `logr` key/value logging from `logf.FromContext(ctx)`; no `fmt.Sprintf` messages
- [ ] Custom metrics low-cardinality; no duplication of controller-runtime metrics
- [ ] Dashboards updated for the 1.33 scheduler metric removal

***

[^1]: Kubernetes v1.33: Octarine — release announcement (2025-04-23), including graduations, deprecations and removals. https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/
[^2]: Kubernetes releases, patch-release schedule and end-of-life data (1.33 EOL 2026-06-28, final patch 1.33.13; 1.37.0 on 2026-08-26). https://kubernetes.io/releases/ ; https://github.com/kubernetes/website/blob/main/data/releases/eol.yaml ; https://github.com/kubernetes/website/blob/main/data/releases/schedule.yaml ; https://dl.k8s.io/release/stable-1.33.txt
[^3]: CHANGELOG-1.33 (v1.33.0 … v1.33.13). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.33.md
[^4]: controller-runtime v0.21.0 release notes (k8s.io/* v0.33, Go 1.24, client-side rate limiting off by default, `Result.Requeue` deprecated, priority-queue changes). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.21.0
[^5]: Go module metadata for k8s.io/client-go v0.33.13, sigs.k8s.io/controller-runtime v0.21.0 and sigs.k8s.io/controller-tools v0.18.0 (go directives, dependency versions, tag dates). https://pkg.go.dev/k8s.io/client-go@v0.33.13 , https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0 , https://pkg.go.dev/sigs.k8s.io/controller-tools@v0.18.0
[^6]: CHANGELOG-1.33, v1.33.0 "Urgent Upgrade Notes", "Deprecation", "API Change", "Feature" and "Other (Cleanup or Flake)" sections. https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.33.md#v1330
[^7]: Kubernetes v1.33: Continuing the transition from Endpoints to EndpointSlices; KEP-4974 Deprecate v1.Endpoints. https://kubernetes.io/blog/2025/04/24/endpoints-deprecation/ , https://github.com/kubernetes/enhancements/issues/4974
[^8]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^9]: controller-runtime v0.21.0 source: `pkg/client/config/config.go` (`GetConfigWithContext` sets `QPS = -1`), `pkg/reconcile/reconcile.go` (`Requeue` deprecation), `pkg/config/controller.go` (`UsePriorityQueue`, off by default), `pkg/client/fake/client.go` (apply patches unsupported). https://github.com/kubernetes-sigs/controller-runtime/tree/v0.21.0
[^10]: Sidecar Containers (stable since v1.33); KEP-753. https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/ , https://github.com/kubernetes/enhancements/issues/753
[^11]: Kubernetes v1.33: In-Place Pod Resize Graduated to Beta; Resize CPU and Memory Resources assigned to Containers (v1.33 documentation, "Limitations"). https://kubernetes.io/blog/2025/05/16/kubernetes-v1-33-in-place-pod-resize-beta/ , https://v1-33.docs.kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/
[^12]: Pod Security Admission and Pod Security Standards. https://kubernetes.io/docs/concepts/security/pod-security-admission/ , https://kubernetes.io/docs/concepts/security/pod-security-standards/
[^13]: Kubernetes v1.33: User Namespaces enabled by default; User Namespaces documentation (requirements, limitations). https://kubernetes.io/blog/2025/04/25/userns-enabled-by-default/ , https://v1-33.docs.kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
[^14]: Configure a Security Context — SELinux label change policy (`seLinuxChangePolicy`, `SELinuxMount`, `SELinuxWarningController`). https://v1-33.docs.kubernetes.io/docs/tasks/configure-pod-container/security-context/
[^15]: Kubelet authentication/authorization — fine-grained authorization (KEP-2862). https://kubernetes.io/docs/reference/access-authn-authz/kubelet-authn-authz/
[^16]: Ingress NGINX Retirement: What You Need to Know. https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/
[^17]: ValidatingAdmissionPolicy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
[^18]: Common Expression Language in Kubernetes — libraries and availability. https://kubernetes.io/docs/reference/using-api/cel/
[^19]: CustomResourceDefinition validation rules (transition rules, optionalOldSelf, messageExpression, reason, fieldPath, ratcheting, selectableFields); KEP-4008 CRD validation ratcheting. https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/ , https://github.com/kubernetes/enhancements/issues/4008
