### Kubernetes 1.31 — Manifests, Operators & Controllers Review Guide

> **Version**: Kubernetes v1.31.0 "Elli", released **2024-08-13**.[^1] Final patch release: **v1.31.14**, published **2025-11-12** (built with Go 1.24.9; `https://dl.k8s.io/release/stable-1.31.txt` reports `v1.31.14`).[^2][^3]
> **Support**: 1.31 is **out of support**. Its end-of-life date was **2025-11-11**; no further patches (including security fixes) are published. As of October 2026 the supported minors are **1.35**, **1.36** and **1.37** (newest: 1.37, released 2026-08-26, latest v1.37.1); 1.34 reaches end of life on 2026-10-27. Managed providers may still offer paid extended support for 1.31 — that does not change upstream status.[^2]
> **Matching libraries**: `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`, `k8s.io/apiextensions-apiserver` **v0.31.x** (final v0.31.14, `go 1.22.0` directive), **controller-runtime v0.19.x** (final v0.19.7, 2025-03-08), **controller-tools v0.16.x** (final v0.16.5). Kubebuilder **v4.2.0–v4.4.x** scaffolds controller-runtime v0.19 (v4.2.0: v0.19.0 + controller-tools v0.16.1, envtest 1.31.0).[^4][^5]
> **Scope**: Kubernetes manifests (plain YAML, Kustomize output, Helm-rendered YAML), CustomResourceDefinitions with CEL validation, admission policies, and Go operators/controllers built with controller-runtime, client-go or Kubebuilder that target a 1.31 cluster.
> **Note**: This document is self-contained. House rules for operator design, CRDs, status, finalizers, RBAC, webhooks, observability and testing are carried forward; sections marked **[NEW in 1.31]** cover behaviour specific to this release. Every manifest, CRD and policy example was applied (or `--dry-run=server`-applied) against a real v1.31.0 kube-apiserver; boundary claims were re-checked on v1.30.3 and v1.32.0 apiservers; Go snippets compile against k8s.io/* v0.31.14 + controller-runtime v0.19.7 and the reconciler was exercised with envtest on the v1.31.0 binaries. Pod *runtime* behaviour (kubelet, scheduler, controllers) is sourced from the release notes, KEPs and documentation, not executed.

---

#### Table of Contents

1. [Version-aware review](#version-aware-review)
2. [What's New in Kubernetes 1.31](#whats-new-in-kubernetes-131)
3. [API versions and feature gates on a 1.31 cluster](#api-versions-and-feature-gates-on-a-131-cluster)
4. [Do not use or recommend (1.32+ features)](#do-not-use-or-recommend-132-features)
5. [Deprecated and removed — what 1.31 code must not rely on](#deprecated-and-removed--what-131-code-must-not-rely-on)
6. [Foundational Principles](#foundational-principles)
7. [Project Layout & Tooling](#project-layout--tooling)
8. [Naming Conventions](#naming-conventions)
9. [CRD & API Design](#crd--api-design)
10. [CEL Validation Rules in CRDs](#cel-validation-rules-in-crds)
11. [Spec vs. Status](#spec-vs-status)
12. [Status Conditions](#status-conditions)
13. [Versioning & API Evolution](#versioning--api-evolution)
14. [Controller / Reconciler Patterns](#controller--reconciler-patterns)
15. [Server-Side Apply from Controllers](#server-side-apply-from-controllers)
16. [Watches & Predicates](#watches--predicates)
17. [Finalizers](#finalizers)
18. [Owner References & Garbage Collection](#owner-references--garbage-collection)
19. [Events](#events)
20. [Admission: CEL Policies and Webhooks](#admission-cel-policies-and-webhooks)
21. [RBAC & Least Privilege](#rbac--least-privilege)
22. [Workload Manifests: Pods](#workload-manifests-pods)
23. [Workload Manifests: Controllers (Deployment, StatefulSet, Job, PDB, HPA)](#workload-manifests-controllers-deployment-statefulset-job-pdb-hpa)
24. [Networking: Services & NetworkPolicy](#networking-services--networkpolicy)
25. [Storage](#storage)
26. [Security Hardening](#security-hardening)
27. [client-go & apimachinery in v0.31](#client-go--apimachinery-in-v031)
28. [Observability](#observability)
29. [Error Handling & Requeue Strategy](#error-handling--requeue-strategy)
30. [Testing](#testing)
31. [kubectl 1.31 and Manifest Tooling](#kubectl-131-and-manifest-tooling)
32. [Removed / Changed in 1.31 — Migration Notes](#removed--changed-in-131--migration-notes)
33. [Production / Review Checklist](#production--review-checklist)

---

#### Version-aware review

The target is a **1.31 control plane** and **k8s.io/\* v0.31.x** client libraries. Before claiming that a field, API version or library function is "not available", check it against the pinned version:

- For manifests: the version comes from the cluster (`kubectl version`), the Helm chart's `kubeVersion`, CI's `kind`/`envtest` image, or a Kustomize/Helm render target. A field that exists in the OpenAPI schema is **not** proof that it works. `kubectl explain` on 1.31 lists alpha-gated fields such as `pod.spec.volumes.image`, `pod.spec.securityContext.supplementalGroupsPolicy`, `pod.spec.containers.resizePolicy`, `job.spec.managedBy` and `statefulset.spec.updateStrategy.rollingUpdate.maxUnavailable`, but a default 1.31 apiserver **silently drops** them on create (verified with `kubectl apply --dry-run=server -o yaml`). Only a server-side dry run against a 1.31 apiserver with the cluster's real feature gates proves a field is honoured.
- Two different failure modes exist, and both were observed: a field the server's schema does not know at all is **rejected** (`strict decoding error: unknown field "spec.volumes[0].image"` on 1.30, because kubectl ≥ 1.27 requests strict server-side field validation), while a field the schema knows but whose feature gate is off is **accepted and dropped without any warning** (`image` volumes, `hostUsers`, `supplementalGroupsPolicy`, `resizePolicy`, `Job.spec.managedBy`, StatefulSet `rollingUpdate.maxUnavailable` on 1.31).
- For Go code: the version comes from `go.mod` (`k8s.io/api v0.31.x`, `sigs.k8s.io/controller-runtime v0.19.x`). Verify a symbol with `go doc sigs.k8s.io/controller-runtime/pkg/cluster Cluster` (or the relevant package) in the module before flagging it.
- Say "not available in 1.31" only when the pinned version proves it (unknown field, `Unsupported value`, dropped field, missing package/function). Say "available but off by default" for alpha features and for beta **APIs** (since 1.24, new beta API versions are not served unless `--runtime-config` enables them).[^6]
- A cluster may enable feature gates. If the repository contains cluster configuration (kubeadm `ClusterConfiguration`, kind config, managed-cluster settings) that enables a gate, review against that configuration, not against the defaults below.
- **Patch level matters**: `DisableNodeKubeProxyVersion` was beta/on in v1.31.0 and reverted to off (deprecated gate) in v1.31.1; the beta gate `OrderedNamespaceDeletion` (default off) only exists from v1.31.7; several CVE fixes landed late in the 1.31 line (see "Removed / changed in 1.31").[^3]
- **The release is end-of-life.** Treat "upgrade off 1.31" as a legitimate, low-priority review note for infrastructure code; do not flag every manifest for it.

---

#### What's New in Kubernetes 1.31

Kubernetes 1.31 contains 45 enhancements: 11 stable, 22 beta and 12 alpha.[^1] The tables list every change that a manifest author, CRD author or controller author can observe. "Default" is the state on an unmodified v1.31.0 control plane; gates were read from `kube-apiserver --help` of the v1.30.3, v1.31.0 and v1.32.0 binaries and from `pkg/features/kube_features.go` at v1.31.0 and v1.31.14, and API behaviour was tested on those apiservers.

##### Graduated to stable (GA) in 1.31

| Feature | What it means for manifests / controllers | KEP |
|---|---|---|
| **AppArmor** (`AppArmor`, `AppArmorFields` locked on) | `securityContext.appArmorProfile` (`RuntimeDefault`, `Localhost` + `localhostProfile`, `Unconfined`) at pod and container level; the `container.apparmor.security.beta.kubernetes.io/<container>` annotations are deprecated (warning since 1.30). The kubelet now hard-rejects Pods that request AppArmor on a node without AppArmor | 24 |
| **Job Pod failure policy** (`JobPodFailurePolicy`) | `spec.podFailurePolicy` with `FailJob`, `Ignore`, `Count` and `FailIndex` actions on exit codes / Pod conditions | 3329 |
| **Pod disruption conditions** (`PodDisruptionConditions`) | Pods deleted by preemption, eviction, taint manager or GC get the `DisruptionTarget` condition; use it in `podFailurePolicy` | 3329 |
| **PDB unhealthy pod eviction policy** (`PDBUnhealthyPodEvictionPolicy`) | `spec.unhealthyPodEvictionPolicy: IfHealthyBudget \| AlwaysAllow` | 3017 |
| **Elastic Indexed Jobs** (`ElasticIndexedJob`) | `spec.completions` of an Indexed Job may be changed together with `spec.parallelism` (they must stay equal) | 3715 |
| **StatefulSet start ordinal** (`StatefulSetStartOrdinal`) | `spec.ordinals.start` | 3335 |
| **Random Pod selection on ReplicaSet downscale** (`LogarithmicScaleDown`) | Scale-down picks victims among similarly-ranked Pods randomly in logarithmic buckets; do not assume "newest Pod is deleted first" | 2185 |
| **PersistentVolume last phase transition time** | `PersistentVolume.status.lastPhaseTransitionTime` (populated on the next phase change after the upgrade) | 3762 |
| **kube-proxy improved ingress connectivity reliability** (`KubeProxyDrainingTerminatingNodes`) | kube-proxy health checks fail for terminating Nodes so load balancers drain them (`externalTrafficPolicy: Cluster`) | 3836 |
| **CDI device IDs in the device-plugin API** (`DevicePluginCDIDevices`) | Device plugins can return CDI device names | 4009 |
| **Metric cardinality enforcement**; **cgroup v1 maintenance mode** | Component config / node operations | 2305, 4569 |

##### Beta in 1.31 (with default)

| Feature / gate | Default in 1.31.0 | Notes |
|---|---|---|
| Service `trafficDistribution: PreferClose` (`ServiceTrafficDistribution`) | **on** (alpha/off in 1.30) | Verified: 1.30 drops the field silently, 1.31 keeps it. `PreferClose` is the **only** accepted value on 1.31 and 1.32 |
| Job `successPolicy` (`JobSuccessPolicy`) | **on** (alpha/off in 1.30) | Verified: 1.30 drops it, 1.31 keeps it. Adds the `SuccessCriteriaMet` condition |
| `matchLabelKeys` / `mismatchLabelKeys` in pod (anti-)affinity (`MatchLabelKeysInPodAffinity`) | **on** (alpha/off in 1.30) | Verified: 1.30 drops the field from `podAntiAffinity` terms, 1.31 keeps it |
| CRD `selectableFields` / field selectors for custom resources (`CustomResourceFieldSelectors`) | **on** (alpha/off in 1.30) | Verified: 1.30 drops `selectableFields`; on 1.31 `kubectl get rcl --field-selector spec.tier=premium` works |
| `recursiveReadOnly` volume mounts (`RecursiveReadOnlyMounts`) | **on** (alpha/off in 1.30) | Needs runtime support (e.g. containerd ≥ 2.0) for `Enabled` |
| Always honor PV reclaim policy (`HonorPVReclaimPolicy`) | **on** (alpha/off in 1.30) | `Delete` reclaim policy is enforced regardless of PV/PVC deletion order; adds finalizers to PVs |
| Node-bound service account tokens (`ServiceAccountTokenNodeBinding`) | **on** | `kubectl create token --bound-object-kind Node` |
| `generateName` retries (`RetryGenerateName`) | **on** | The apiserver retries name generation on conflict instead of returning `AlreadyExists` |
| Consistent reads from the watch cache (`ConsistentListFromCache`) | **on** | Requires etcd ≥ 3.4.31 / 3.5.13 (progress notify); performance regression fixed in v1.31.7 |
| kube-proxy nftables mode (`NFTablesProxyMode`) | **on** (selectable with `mode: nftables`; iptables stays the default mode) | Kernel ≥ 5.13, `nft` ≥ 1.0.1 |
| `kubectl port-forward` over WebSockets (`PortForwardWebsockets`) | **on** | Exec/attach over WebSockets were already beta in 1.30 |
| Kubelet cgroup driver from CRI (`KubeletCgroupDriverFromCRI`), split image filesystem GC (`KubeletSeparateDiskGC`) | **on** | Node configuration |
| `ProcMountType` | **beta, off** | Verified: `procMount: Unmasked` is reset to `Default` on 1.31 |
| `UserNamespacesSupport` (`hostUsers: false`) | **beta, off** (beta since 1.30) | Verified: `hostUsers: false` is dropped on a default 1.31 apiserver |
| VolumeAttributesClass (`storage.k8s.io/v1beta1`) | **beta, off** (API not served) | Needs gate + `--runtime-config=storage.k8s.io/v1beta1=true` |
| Multiple Service CIDRs (`MultiCIDRServiceAllocator`, `networking.k8s.io/v1beta1` ServiceCIDR/IPAddress) | **beta, off** | Gate + runtime-config |
| Strict CEL cost enforcement for VAP / webhooks (`StrictCostEnforcementForVAP`, `StrictCostEnforcementForWebhooks`) | **beta, off** | GA in 1.32 |
| `DisableNodeKubeProxyVersion` | **on in v1.31.0, off from v1.31.1** | `Node.status.nodeInfo.kubeProxyVersion` is deprecated |

##### Alpha in 1.31 (off by default — treat as unavailable)

**Image volumes** `volumes[].image` (`ImageVolume`); **fine-grained supplemental groups** `securityContext.supplementalGroupsPolicy` (`SupplementalGroupsPolicy`); DRA with **structured parameters** `resource.k8s.io/v1alpha3` (`DynamicResourceAllocation`) and "classic" DRA (`DRAControlPlaneController`); device health in `containerStatuses[].allocatedResourcesStatus` (`ResourceHealthStatus`); **authorization with selectors** (`AuthorizeWithSelectors`, `AuthorizeNodeWithSelectors`); configurable anonymous-auth endpoints (`AnonymousAuthConfigurableEndpoints`); **coordinated leader election** `coordination.k8s.io/v1alpha1` LeaseCandidate (`CoordinatedLeaderElection`); Job `managedBy` (`JobManagedBy`, alpha since 1.30); in-place Pod resize (`InPlacePodVerticalScaling`, alpha since 1.27); StatefulSet `maxUnavailable` (`MaxUnavailableStatefulSet`); relaxed environment-variable names (`RelaxedEnvironmentVariableValidation`); `StorageVersionMigrator` (`storagemigration.k8s.io/v1alpha1`); `ClusterTrustBundle` (`certificates.k8s.io/v1alpha1`); `SELinuxMount`; `HPAScaleToZero`.[^1][^3]

The `MutatingAdmissionPolicy` gate is listed as alpha in the 1.31 binary, but the API does not exist in 1.31: verified, an apiserver started with `--feature-gates=MutatingAdmissionPolicy=true --runtime-config=admissionregistration.k8s.io/v1alpha1=true` serves `admissionregistration.k8s.io/v1alpha1` without any `mutatingadmissionpolicies` resource. The API arrives as alpha in 1.32.

##### Other 1.31 changes that affect reviews

- **All in-tree cloud-provider integrations are removed** (`DisableCloudProviders` and `DisableKubeletCloudCredentialProviders` are locked on; `k8s.io/legacy-cloud-providers` removed). `--cloud-provider=aws|gce|azure|…` on kube-apiserver/kube-controller-manager/kubelet no longer works — clusters must run an external cloud-controller-manager (`--cloud-provider=external`).[^1]
- **In-tree volume plugins `cephfs` and `rbd` are removed** — Pods/PVs using `cephfs:` or `rbd:` volume sources become non-functional; use ceph-csi. The `InTreePlugin*Unregister` gates for AWS/Azure/GCE/OpenStack/vSphere were removed.[^1]
- **`CSIMigrationPortworx` is now on by default**: install the Portworx CSI driver before upgrading if `portworxVolume` sources are in use.[^3]
- **kubelet `--keep-terminated-pod-volumes`, `--iptables-masquerade-bit` and `--iptables-drop-bit` removed**; kube-proxy `--proxy-port-range` removed; kube-controller-manager `--horizontal-pod-autoscaler-upscale-delay`/`-downscale-delay`, `--volume-host-cidr-denylist`, `--volume-host-allow-local-loopback` removed; admission plugin `PersistentVolumeLabel` removed; `kubectl drain --delete-local-data` removed (verified: kubectl 1.31 `unknown flag`, 1.30 prints a deprecation notice) — use `--delete-emptydir-data`.[^3]
- **Removed feature gates break component startup.** GA gates removed in 1.31 (`APIPriorityAndFairness`, `CSINodeExpandSecret`, `ConsistentHTTPGetHandlers`, `ReadWriteOncePod`, `ServiceNodePortStaticSubrange`, `JobReadyPods`, `DefaultHostNetworkHostPortsInPodTemplates`, `SkipReadOnlyValidationGCE`, the `InTreePlugin*Unregister` gates, `CustomResourceValidationExpressions`) are no longer accepted: verified, `kube-apiserver --feature-gates=APIPriorityAndFairness=true` exits with `unrecognized feature gate: APIPriorityAndFairness` on 1.31, while 1.30 only warns. Remove such entries from kubeadm `extraArgs`, kind configs and component configs before upgrading.[^3]
- **Node `status.nodeInfo.kubeProxyVersion` deprecated** (KEP-4004): the kubelet never knew the real kube-proxy version. Do not read it in controllers.[^1]
- **cgroup v1 in maintenance mode**: the kubelet logs a warning and emits an event on cgroup v1 hosts; a new `--fail-cgroupv1` flag/`failCgroupV1` option exists (default `false` in 1.31).[^1][^3]
- **SHA-1 certificates** (not a 1.31 change, but it bites the final 1.31 patch): since Kubernetes 1.24 (Go 1.18) webhooks and aggregated API servers serving SHA-1-signed certificates only work with `GODEBUG=x509sha1=1`. Go 1.24 removed that escape hatch, and the final patch v1.31.14 is the only 1.31 release built with Go 1.24 (v1.31.13 and earlier use Go 1.22/1.23), so upgrading to v1.31.14 breaks such webhooks.[^3]
- **CEL environment**: the default CEL compatibility version was raised to 1.30, so libraries introduced in 1.30 (notably the **IP and CIDR library**, `ip()`/`cidr()`/`isIP()`/`isCIDR()`) become usable in *new* CRD rules and policies on 1.31; libraries introduced in 1.31 (the **name formats library**, `format.dns1123Label()` etc.) are only usable for new expressions from 1.32. CEL expressions and `additionalProperties` are now allowed under nested quantifiers, and the CEL optional type is now named `optional_type` in error messages.[^3]
- **CRD `caBundle` validation**: CRDs created with an invalid non-empty conversion-webhook `caBundle` are not served until it is fixed, and updates may no longer replace a valid `caBundle` with an invalid one.[^3]
- **kubectl**: `kubectl exec POD CMD` without `--` is now an error; `kubectl wait --for=create`, `kubectl logs --all-pods`, and `kubectl debug --custom` (beta, on) plus `--keep-*` flags are new (all verified with kubectl 1.30 vs 1.31).
- **Server-side apply**: Ingress `spec.defaultBackend` is now an atomic struct; no-op SSA patches no longer bump `resourceVersion` for custom resources or objects with empty maps.[^3]
- **Pod Security Standards**: `container_engine_t` is now allowed as an SELinux type in the baseline policy.[^3]

---

#### API versions and feature gates on a 1.31 cluster

##### API group versions served by a default v1.31.0 apiserver

Verified with `kubectl api-versions` on stock v1.30.3, v1.31.0 and v1.32.0 apiservers:

```text
admissionregistration.k8s.io/v1   apiextensions.k8s.io/v1   apiregistration.k8s.io/v1
apps/v1   authentication.k8s.io/v1   authorization.k8s.io/v1   autoscaling/v1   autoscaling/v2
batch/v1   certificates.k8s.io/v1   coordination.k8s.io/v1   discovery.k8s.io/v1
events.k8s.io/v1   flowcontrol.apiserver.k8s.io/v1   flowcontrol.apiserver.k8s.io/v1beta3
networking.k8s.io/v1   node.k8s.io/v1   policy/v1   rbac.authorization.k8s.io/v1
scheduling.k8s.io/v1   storage.k8s.io/v1   v1
```

1.30 serves the identical list. 1.32 serves the same list **minus `flowcontrol.apiserver.k8s.io/v1beta3`**, which was removed in 1.32 (verified warning on 1.31: `flowcontrol.apiserver.k8s.io/v1beta3 PriorityLevelConfiguration is deprecated in v1.29+, unavailable in v1.32+; use flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration`) — `FlowSchema`/`PriorityLevelConfiguration` manifests must use `flowcontrol.apiserver.k8s.io/v1` (GA since 1.29) to survive the next upgrade.[^7]

On a default 1.31 cluster these are **not** served and manifests using them fail with `no matches for kind`:

| API | Status in 1.31 | How to enable |
|---|---|---|
| `storage.k8s.io/v1beta1` VolumeAttributesClass | beta, off | `--feature-gates=VolumeAttributesClass=true --runtime-config=storage.k8s.io/v1beta1=true` (GA `storage.k8s.io/v1` only in 1.34) |
| `networking.k8s.io/v1beta1` ServiceCIDR, IPAddress | beta, off | `MultiCIDRServiceAllocator=true` + runtime-config (GA in 1.33) |
| `resource.k8s.io/v1alpha3` ResourceClaim, ResourceClaimTemplate, ResourceSlice, DeviceClass | alpha, off | `DynamicResourceAllocation=true` + runtime-config; **incompatible** with the 1.30 `v1alpha2` API |
| `storagemigration.k8s.io/v1alpha1` StorageVersionMigration | alpha, off | gate + runtime-config |
| `certificates.k8s.io/v1alpha1` ClusterTrustBundle | alpha, off | gate + runtime-config |
| `coordination.k8s.io/v1alpha1` LeaseCandidate | alpha, off | `CoordinatedLeaderElection=true` + runtime-config |
| `admissionregistration.k8s.io/v1beta1` / `v1alpha1` | off (VAP is GA in `v1`) | Never needed for VAP on 1.31 — use `v1` |
| MutatingAdmissionPolicy (any version) | **does not exist** | 1.32 (alpha) |

All of the above opt-in APIs were verified to appear on a 1.31.0 apiserver started with the respective gates and `--runtime-config` entries.

##### Selected feature gates (kube-apiserver v1.31.0 defaults)

```text
ServiceTrafficDistribution=true (BETA)        JobSuccessPolicy=true (BETA)
JobBackoffLimitPerIndex=true (BETA)           JobPodReplacementPolicy=true (BETA)
MatchLabelKeysInPodAffinity=true (BETA)       MatchLabelKeysInPodTopologySpread=true (BETA)
CustomResourceFieldSelectors=true (BETA)      CRDValidationRatcheting=true (BETA)
SidecarContainers=true (BETA)                 PodLifecycleSleepAction=true (BETA)
RecursiveReadOnlyMounts=true (BETA)           HonorPVReclaimPolicy=true (BETA)
RetryGenerateName=true (BETA)                 ConsistentListFromCache=true (BETA)
StatefulSetAutoDeletePVC=true (BETA)          PodIndexLabel=true (BETA)
LoadBalancerIPMode=true (BETA)                ServiceAccountTokenNodeBinding=true (BETA)
NodeInclusionPolicyInPodTopologySpread=true (BETA)  PortForwardWebsockets=true (BETA)
NFTablesProxyMode=true (BETA)                 DisableNodeKubeProxyVersion=true (BETA) -> false from v1.31.1
UserNamespacesSupport=false (BETA)            ProcMountType=false (BETA)
VolumeAttributesClass=false (BETA)            MultiCIDRServiceAllocator=false (BETA)
StrictCostEnforcementForVAP=false (BETA)      StrictCostEnforcementForWebhooks=false (BETA)
WatchListClient=false (BETA)                  SchedulerQueueingHints=false (BETA)
ImageVolume=false (ALPHA)                     SupplementalGroupsPolicy=false (ALPHA)
InPlacePodVerticalScaling=false (ALPHA)       JobManagedBy=false (ALPHA)
MaxUnavailableStatefulSet=false (ALPHA)       RelaxedEnvironmentVariableValidation=false (ALPHA)
DynamicResourceAllocation=false (ALPHA)       MutatingAdmissionPolicy=false (ALPHA, no API)
AuthorizeWithSelectors=false (ALPHA)          AnonymousAuthConfigurableEndpoints=false (ALPHA)
CoordinatedLeaderElection=false (ALPHA)       StorageVersionMigrator=false (ALPHA)
ClusterTrustBundle=false (ALPHA)              SELinuxMount=false (ALPHA)   HPAScaleToZero=false (ALPHA)
WatchList=false (ALPHA)
OrderedNamespaceDeletion=false (BETA)  -> only present from v1.31.7
```

GA gates (for example `AppArmor`, `JobPodFailurePolicy`, `PodDisruptionConditions`, `PDBUnhealthyPodEvictionPolicy`, `ElasticIndexedJob`, `StatefulSetStartOrdinal`, `LogarithmicScaleDown`) are locked on and no longer appear in `--help`.

##### Already-GA features from earlier releases that 1.31 code should use

| Feature | GA since | Use it for |
|---|---|---|
| CronJob `timeZone` | 1.27 | Explicit schedules instead of the controller-manager's local time |
| Server-side field validation (`fieldValidation=Strict`) | 1.27 | Unknown/duplicate fields become errors (kubectl's default) |
| `kubectl auth whoami` / SelfSubjectReview `v1` | 1.28 | Debugging identities |
| Non-graceful node shutdown (`node.kubernetes.io/out-of-service` taint) | 1.28 | StatefulSet failover |
| CRD validation rules (`x-kubernetes-validations`, CEL) | 1.29 | Cross-field and transition validation instead of webhooks |
| `ReadWriteOncePod` access mode | 1.29 | Single-writer volumes |
| `flowcontrol.apiserver.k8s.io/v1` (API Priority and Fairness) | 1.29 | APF objects on `v1` |
| Legacy service-account token cleanup | 1.30 | Unused auto-generated token Secrets are labelled and deleted |
| ValidatingAdmissionPolicy (`admissionregistration.k8s.io/v1`) | 1.30 | In-process policy instead of validating webhooks |
| Webhook `matchConditions` | 1.30 | Skip webhook calls with CEL pre-filters |
| Pod scheduling gates (`schedulingGates`) | 1.30 | Hold Pods until an external controller is ready |
| `minDomains` in topology spread | 1.30 | Force spreading over N zones |
| HPA `ContainerResource` metrics | 1.30 | Scale on one container's usage (ignore sidecars) |
| `status.hostIPs` (dual-stack host IPs) | 1.30 | Downward API `status.hostIPs` |
| AppArmor fields, Job `podFailurePolicy`, PDB `unhealthyPodEvictionPolicy`, StatefulSet `ordinals.start`, elastic Indexed Jobs, `DisruptionTarget` condition | **1.31** | See below |

Beta-and-on features that are safe to use on 1.31 (the API surface is stable enough, and they are on by default): **native sidecars** (`initContainers[].restartPolicy: Always`, beta-on since 1.29), **`lifecycle.preStop.sleep`** (beta-on since 1.30; `seconds` must be ≥ 1), Job **`backoffLimitPerIndex`** and **`podReplacementPolicy`** (beta-on since 1.29), StatefulSet **`persistentVolumeClaimRetentionPolicy`** (beta-on since 1.27), **`matchLabelKeys`** in topology spread (beta-on since 1.27), CRD **validation ratcheting** (beta-on since 1.30), Service **`status.loadBalancer.ingress[].ipMode`** (beta-on since 1.30).

---

#### Do not use or recommend (1.32+ features)

The cluster is 1.31. Never suggest the following as available; they fail validation, are silently dropped, or need a newer apiserver/library. (Versions are taken from the 1.32–1.37 release notes; every 1.32 boundary below was verified on the v1.32.0 apiserver.)[^8]

**Kubernetes 1.32 (released 2024-12-11):**

- `admissionregistration.k8s.io/v1alpha1` **MutatingAdmissionPolicy** (alpha in 1.32; beta 1.34; GA 1.36). Absent on 1.31 in every configuration.
- **`lifecycle.preStop.sleep.seconds: 0`** (`PodLifecycleSleepActionAllowZero`, alpha in 1.32, beta-on 1.33). Verified: 1.31 rejects `0` with `must be greater than 0 and less than terminationGracePeriodSeconds (30)`.
- **Relaxed environment-variable names** (`RelaxedEnvironmentVariableValidation`, beta-on in 1.32, GA 1.34). Verified: 1.31 rejects `name: "1VAR"` (`must not start with a digit`); 1.32 accepts it.
- **Job `spec.managedBy`** honoured (`JobManagedBy` beta-on in 1.32, GA 1.35). Verified: 1.31 silently drops it — the built-in Job controller then runs the Job itself, the opposite of what a MultiKueue manifest intends.
- **Pod-level `spec.resources`** (`PodLevelResources`, alpha 1.32, beta-on 1.34).
- **StatefulSet `updateStrategy.rollingUpdate.maxUnavailable`** (`MaxUnavailableStatefulSet`): alpha/off on 1.30–1.34, so verified dropped on create by the v1.30.3, v1.31.0 and v1.32.0 apiservers (the stored `rollingUpdate` is just `{partition: 0}`); on by default only in v1.35.0–v1.35.3 and again in 1.37. Never suggest it to speed up 1.31 rollouts. Verified: `kubectl explain pod.spec.resources` fails on 1.31 (`field "resources" does not exist`).
- **CEL name formats library** in new rules (`format.dns1123Label().validate(self)` etc.; usable for new expressions from 1.32). Verified: a CRD rule using it fails to compile on 1.31 and is accepted on 1.32.
- `flowcontrol.apiserver.k8s.io/v1beta3` is **removed** in 1.32 (the reverse direction: it still works on 1.31, but new manifests must use `v1`).
- DRA `resource.k8s.io/v1beta1` (beta, off), `SchedulerQueueingHints` on, `WatchList` beta-on, `AuthorizeWithSelectors`/`AuthorizeNodeWithSelectors` beta-on, `AnonymousAuthConfigurableEndpoints` beta, CBOR serialization (alpha), `/statusz`/`/flagz` (alpha), `KubeletCrashLoopBackOffMax` (alpha), `RecoverVolumeExpansionFailure` beta-on, `SELinuxChangePolicy`, `RelaxedDNSSearchValidation`.
- GA in 1.32 (beta in 1.31 — usable, but do not call them GA): CRD field selectors, `StatefulSetAutoDeletePVC`, `PodIndexLabel`, `LoadBalancerIPMode`, `RetryGenerateName`, `MemoryManager`, structured authorization configuration, bound service-account token JTI/node info, strict CEL cost enforcement (on only from 1.32).
- Go: `k8s.io/* v0.32` / controller-runtime **v0.20** (both `go 1.23.0`; experimental `UsePriorityQueue` option; the deprecated `admission.Validator`/`admission.Defaulter` interfaces are removed).

**Kubernetes 1.33 (released 2025-04-23):** sidecar containers **GA**; in-place Pod resize **beta-on** (`pods/resize` subresource); user namespaces **on by default**; `trafficDistribution` GA field (`PreferClose`); HPA configurable `tolerance` (alpha); Deployment `status.terminatingReplicas` (alpha); container `lifecycle.stopSignal` (alpha); `MultiCIDRServiceAllocator` / `networking.k8s.io/v1` ServiceCIDR GA; Job `backoffLimitPerIndex`/`successPolicy` GA; CRD validation ratcheting GA; `recursiveReadOnly` GA; nftables kube-proxy GA; `supplementalGroupsPolicy` and `procMount` beta-on; `PreferSameZone`/`PreferSameNode` (alpha); `PodTopologyLabelsAdmission`, kubelet credential re-verification for cached images and Pod `observedGeneration` (alpha); `preStop.sleep.seconds: 0` beta-on; CEL two-variable comprehensions; `kubectl` `.kuberc` (alpha); ordered namespace deletion beta-on.

**Kubernetes 1.34 (released 2025-08-27):** DRA **GA** (`resource.k8s.io/v1`); VolumeAttributesClass **GA** (`storage.k8s.io/v1`); `PodLifecycleSleepAction` GA; Job `podReplacementPolicy` GA; pod-level resources beta-on; `MutatingAdmissionPolicy` beta; `PreferSameZone`/`PreferSameNode` traffic distribution (beta-on); relaxed environment-variable names GA; CEL `isSemver()`/`semver()` in new rules; AppArmor annotations no longer mirrored from fields.

**Kubernetes 1.35–1.37:** in-place Pod resize GA, Pod `metadata.generation`/`status.observedGeneration`, Job `managedBy` GA, `supplementalGroupsPolicy` GA, container `restartPolicyRules`, `env[].valueFrom.fileKeyRef`, image volumes beta-on (1.35) and GA (1.36), MutatingAdmissionPolicy `v1` (1.36), user namespaces GA (1.36), Service names starting with a digit (1.36), StatefulSet `updateStrategy.type: Recreate` (alpha in 1.37), `storagemigration.k8s.io/v1` (1.37), KYAML output (`kubectl get -o kyaml`).[^8]

**Library versions to reject for a 1.31-only target:** `k8s.io/client-go v0.32+` with controller-runtime v0.20+ is fine on the wire (±1 minor skew), but raises the module's `go` directive to 1.23.0 and brings 1.32 API fields that a 1.31 apiserver drops. controller-runtime v0.22+ APIs such as `client.Client.Apply(ctx, applyConfiguration, ...)`, `mgr.GetEventRecorder` (events.k8s.io, v0.23), `ctrl.NewWebhookManagedBy(mgr, obj)` (generic, v0.23), `reconcile.Result.Priority` (v0.23) **do not exist in v0.19** — never suggest them for a v0.19 module.

**Never recommend** `PreferSameZone`, `PreferSameNode`, `hostUsers: false` as default-on, `image` volumes, `supplementalGroupsPolicy`, `kubectl patch --subresource resize`, `fileKeyRef`, or `restartPolicyRules` for a 1.31 cluster.

---

#### Deprecated and removed — what 1.31 code must not rely on

| Item | Status | Replacement |
|---|---|---|
| `container.apparmor.security.beta.kubernetes.io/*` annotations | Deprecated (warning: `deprecated since v1.30; use the "appArmorProfile" field instead`) | `securityContext.appArmorProfile` (GA in 1.31) |
| In-tree cloud providers (`--cloud-provider=aws\|gce\|azure\|vsphere\|openstack`) | **Removed** | External cloud-controller-manager, `--cloud-provider=external` |
| `cephfs` and `rbd` in-tree volume sources | **Removed** (non-functional) | ceph-csi driver |
| In-tree `portworxVolume` | Migrated to CSI by default (`CSIMigrationPortworx` on) | Portworx CSI driver must be installed |
| `flowcontrol.apiserver.k8s.io/v1beta3` | Deprecated, removed in 1.32 | `flowcontrol.apiserver.k8s.io/v1` |
| `Node.status.nodeInfo.kubeProxyVersion` | Deprecated | Query kube-proxy directly (DaemonSet image / metrics) |
| kubelet `--keep-terminated-pod-volumes`, kube-proxy `--proxy-port-range`, KCM HPA delay flags and `--volume-host-*` flags | **Removed** | Delete the flags before upgrading |
| Admission plugin `PersistentVolumeLabel` | **Removed** | cloud-pv-admission-labeler / CSI topology |
| Scheduler plugins `AzureDiskLimits`, `CinderLimits`, `EBSLimits`, `GCEPDLimits` | Deprecated, removed from defaults | `NodeVolumeLimits` |
| PV annotation `volume.beta.kubernetes.io/mount-options` | Deprecated (warning on create/update since 1.31) | `spec.mountOptions` |
| `kubectl exec POD CMD` (no `--`) | **Removed** in kubectl 1.31 | `kubectl exec POD -- CMD` |
| cgroup v1 nodes | Maintenance mode (warnings and events) | cgroup v2 hosts |
| SHA-1 signed serving certificates for webhooks/aggregated APIs | Only with `GODEBUG=x509sha1=1`; impossible with v1.31.14 (built with Go 1.24) | SHA-256+ certificates |
| `k8s.io/client-go/kubernetes/fake.NewSimpleClientset` | Deprecated in v0.31 (no field management) | `fake.NewClientset` (new in v0.31) |
| controller-runtime `admission.Validator` / `admission.Defaulter` (methods on the API type), `ValidatingWebhookFor`/`DefaultingWebhookFor` | Deprecated in v0.19; removed in v0.20 | `admission.CustomValidator` / `CustomDefaulter` + `WithValidator` / `WithDefaulter` |
| `k8s.io/utils/pointer` | Deprecated | `k8s.io/utils/ptr` (`ptr.To`, `ptr.Deref`) |
| `gitRepo` volumes | Deprecated since 1.11 (still functional on 1.31) | init container / git-sync |
| Service `externalIPs` | Still supported; CVE-2020-8554 | Block with VAP or `DenyServiceExternalIPs`; use LoadBalancer/Gateway API |

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
| **Stale caches are normal** | Reads come from an informer cache that lags the API server. Writes must use optimistic concurrency (`resourceVersion`), patches, or server-side apply; never "read-modify-update" across two reconciles from cached data and assume it is current. |

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

##### Toolchain versions for a 1.31 target

| Component | Version for 1.31 | Notes |
|---|---|---|
| Go | ≥ 1.22 (module directive); build with a **supported** Go release | `k8s.io/* v0.31` and controller-runtime v0.19 declare `go 1.22.0`. Go 1.22 itself is unsupported since February 2025 — build with a current Go toolchain; the modules compile with newer Go (verified with Go 1.24 and 1.25) |
| `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` | v0.31.x (keep all on the **same** patch, ideally v0.31.14) | Mixed minors cause compile or runtime scheme errors |
| controller-runtime | v0.19.x (final v0.19.7) | Its `go.mod` requires `k8s.io/* v0.31.0`; Go MVS raises it to your v0.31.x patch |
| controller-tools (`controller-gen`) | v0.16.x (final v0.16.5); **≥ v0.16.4** for `+kubebuilder:selectablefield` | The selectable-field marker first appears in v0.16.4 (module sources: absent in v0.16.3, present in v0.16.4). Older versions **ignore it silently** — verified: controller-gen v0.16.0 exits 0 and emits the CRD without `selectableFields`, v0.16.5 emits it. Kubebuilder v4.2.0 scaffolds v0.16.1; v4.3.x scaffolds v0.16.4 |
| Kubebuilder | v4.2.0–v4.4.x (scaffolds controller-runtime v0.19) | v4.4 already scaffolds controller-tools v0.17 (k8s 1.32 libraries) |
| envtest binaries | 1.31.x (`setup-envtest use 1.31.x`) | Test against the version you ship to |

```go
// go.mod — all k8s.io modules on one patch release
module example.com/redis-operator

go 1.22.0

require (
	k8s.io/api v0.31.14
	k8s.io/apimachinery v0.31.14
	k8s.io/client-go v0.31.14
	sigs.k8s.io/controller-runtime v0.19.7
)
```

Flag in review:

- `k8s.io/client-go v0.32.x` (or controller-runtime v0.20+) in a module that claims to support a 1.31 cluster is usually fine on the wire (client/server skew of ±1 minor is supported), but drags in 1.32 API fields the 1.31 server drops and removes the deprecated webhook interfaces. Pin the library minor to the oldest cluster minor you support when you rely on new fields.
- A `toolchain go1.22.x` line or a CI image pinned to Go 1.22/1.23: those Go releases are out of support and miss security fixes; the module directive may stay at `go 1.22.0`.

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
| **DNS Label** (RFC 1035, stricter) | **Service names** (cannot start with a digit), some legacy fields | lowercase `a–z`, digits `0–9`, `-` | same as above PLUS cannot start with a digit | must start with `[a-z]`, end with alphanumeric | 63 |
| **Qualified Name** (label/annotation key, finalizer) | label keys, annotation keys, finalizer names | name part: `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` ; optional prefix part: DNS Subdomain followed by `/` | name part: whitespace, `/` (except as prefix separator), `:`, all other punctuation | name part must start AND end with alphanumeric | name ≤ 63; prefix ≤ 253; total ≤ 317 |
| **Label Value** | `metadata.labels` values | `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` (or empty string) | whitespace, `/`, `:`, all other punctuation, unicode | if non-empty: alphanumeric at both ends | 63 |
| **Annotation Value** | `metadata.annotations` values | any valid UTF-8 string (incl. whitespace, JSON, base64) | none per key | n/a | total across all annotations ≤ 256 KiB |
| **Environment variable name** (1.31) | `env[].name`, keys imported through `envFrom` | `A–Z`, `a–z`, `0–9`, `_`, `-`, `.` | everything else, including a leading digit | must not start with a digit (regex `[-._a-zA-Z][-._a-zA-Z0-9]*`) | no hard limit |
| **ConfigMap / Secret data key** | `data` / `stringData` keys | `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` | `/`, whitespace, `:`, all other punctuation | any alphanumeric | 253 |
| **Path Segment Name** | `metadata.name` for objects exposed in URL paths | DNS Subdomain set MINUS `.` and `..` as full name | same as DNS Subdomain plus the literal names `.` and `..` | as DNS Subdomain | 253 |

> ⚠️ Underscore `_` is **never** valid in a DNS Subdomain or DNS Label name — that rules it out for `metadata.name`, `metadata.namespace`, API group, and Service name. Underscore IS valid in label/annotation keys (name part), label values, and ConfigMap/Secret data keys.

> ⚠️ Uppercase letters are **never** valid in DNS Subdomain or DNS Label names. They ARE valid in label keys (name part), label values, qualified names, and environment variable names.

> ⚠️ **Service names must start with a letter on 1.31.** Verified: 1.31 (and 1.30, 1.32) reject `metadata.name: 1password-connect` with `a DNS-1035 label must ... start with an alphabetic character`. Relaxed Service names (`RelaxedServiceNameValidation`) are alpha in 1.34 and beta-on only in 1.36.

> ⚠️ **Environment variable names on 1.31** may contain `.` and `-` but must not start with a digit (verified: `1VAR` is rejected on 1.31, accepted on 1.32). Names that are not C identifiers (`[A-Za-z_][A-Za-z0-9_]*`) are invisible to POSIX shells; keep C identifiers for anything a shell script reads. `envFrom` silently skips keys that are not valid environment variable names (the kubelet records an `InvalidEnvironmentVariableNames` event).

##### API Group, Kind, Resource

| Element | Rule | Example |
|---|---|---|
| **API Group** | Lowercase DNS subdomain you own | `cache.example.com` |
| **Kind** | CamelCase, singular | `RedisCluster` |
| **Resource (plural)** | Lowercase, plural | `redisclusters` |
| **Short name** | 2–5 lowercase chars; register in `.spec.names.shortNames` | `rcl` → avoid conflicts with built-ins |
| **Category** | Lowercase; register in `.spec.names.categories` | `all` (use sparingly) |

> ⚠️ Do **not** use the empty group, single-word names, or `*.k8s.io` / `*.kubernetes.io` — those are reserved by the Kubernetes project (they require an `api-approved.kubernetes.io` annotation).

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
cache.example.com/last-backup-time: "2024-09-01T00:00:00Z"
```

Topology labels on Pods: on 1.31 the apiserver does **not** copy `topology.kubernetes.io/zone`/`region` from the Node onto Pods (`PodTopologyLabelsAdmission` is new in 1.33). A Pod that needs its zone must read the Node (`get nodes` RBAC) or receive it from its controller; the Downward API exposes only `spec.nodeName`.

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

**[NEW in 1.31 — beta, on]** `+kubebuilder:selectablefield` (controller-tools ≥ v0.16.4) becomes `spec.versions[].selectableFields`. On 1.31, `kubectl get rcl --field-selector spec.tier=premium` filters server-side (verified); a 1.30 apiserver **drops** `selectableFields` silently (gate alpha/off), and a field selector on an unlisted field fails. At most 8 selectable fields per version; each must point to a string, integer or boolean field. Charts that also install on ≤ 1.30 must not depend on CR field selectors. Check the controller-gen version in the Makefile: below v0.16.4 the marker is silently ignored and the generated CRD has no `selectableFields`.

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
- **`format: int32` is not a range check.** Numeric format ranges are only enforced from 1.36; on 1.31 add explicit `minimum`/`maximum` (`+kubebuilder:validation:Minimum/Maximum`).

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

CRD validation rules (`x-kubernetes-validations`) are GA since 1.29 and are the first choice for invariants that the OpenAPI schema cannot express.[^9] The following Go type compiles against k8s.io/* v0.31.14; its rules were exercised in envtest on the v1.31.0 binaries through the equivalent CRD schema (`premium tier needs at least 3 replicas, got 2`, `must be a CIDR`, `image must be pinned by digest` all reported together, `apierrors.IsInvalid` true):

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

	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:validation:items:MaxLength=43
	// +kubebuilder:validation:items:XValidation:rule="isCIDR(self)",message="must be a CIDR"
	// +optional
	AllowedSources []string `json:"allowedSources,omitempty"`
}
```

controller-gen v0.16.5 renders these markers as expected (verified): the two type-level rules land on `spec` with `fieldPath: .replicas`, `reason: FieldValueInvalid` and the `messageExpression`; `items:XValidation` lands on `allowedSources.items`; `required` becomes `[image, replicas, tier]`; `metav1.Condition` contributes `maxLength: 316` plus the type/reason patterns; `+kubebuilder:selectablefield` becomes `spec.versions[0].selectableFields`. Review the generated YAML, not only the markers — a typo in a marker name is silently ignored.

The same kind of rules written directly in a CRD manifest, with the CEL libraries that a 1.31 apiserver accepts in **new** rules (`quantity`, `url`/`isURL`, string `format`, IP/CIDR — all verified):

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
    additionalPrinterColumns:
    - {name: Tier, type: string, jsonPath: .spec.tier}
    - {name: Ready, type: string, jsonPath: '.status.conditions[?(@.type=="Ready")].status'}
    - {name: Age, type: date, jsonPath: .metadata.creationTimestamp}
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
            - rule: "!has(oldSelf.storageClassName) || has(self.storageClassName)"
              message: storageClassName cannot be removed once set
            properties:
              tier: {type: string, enum: [standard, premium]}
              replicas: {type: integer, format: int32, minimum: 1, maximum: 99}
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
              allowedSources:
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
              readyReplicas: {type: integer, format: int32}
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

Observed results on 1.31 (all errors are reported together). Note that 1.31 prints the **type name** (`"object"`, `"string"`) as the "invalid value" of a CEL failure, not the value itself — tests that match on the rejected value must match on the message instead:

```text
The RedisCluster "bad" is invalid:
* spec.maxMemory: Invalid value: "object": maxMemory must be larger than 64Mi
* spec.replicas: Invalid value: "object": premium tier needs at least 3 replicas, got 2
* spec.allowedSources[1]: Invalid value: "string": must be an IPv4 CIDR      # fd00::/8
* spec.allowedSources[2]: Invalid value: "string": must be an IPv4 CIDR      # "nope"
* spec.endpoints[0].url: Invalid value: "string": url must be an https URL
# update fast -> slow, and removal:
* spec.storageClassName: Invalid value: "string": storageClassName is immutable
* spec: Invalid value: "object": storageClassName cannot be removed once set
```

The same CRD is **rejected by a 1.30 apiserver** (`undeclared reference to 'isCIDR'`, `'cidr'`): the IP/CIDR library exists in 1.30 binaries but only becomes available to new expressions in 1.31.

##### CEL library availability (new rules, verified per apiserver)

| Library / function | 1.30 | **1.31** | 1.32 |
|---|---|---|---|
| `quantity()`, `isQuantity()` | ✅ | ✅ | ✅ |
| `url()`, `isURL()` | ✅ | ✅ | ✅ |
| strings ext incl. `'%d'.format([...])`, `lowerAscii()`; `find()`; lists `sum()`; `sets.contains()`; optionals `?.` / `orValue()` | ✅ | ✅ | ✅ |
| `ip()`, `isIP()`, `cidr()`, `isCIDR()` | ❌ | ✅ | ✅ |
| name formats `format.dns1123Label()`, `format.dns1123Subdomain()`, … | ❌ | ❌ (`undeclared reference`) | ✅ |
| `isSemver()`, `semver()` | ❌ | ❌ | ❌ (new rules from 1.34) |
| two-variable comprehensions `all(k, v, …)` | ❌ | ❌ | ❌ (from 1.33) |

"Available in the binary but not in the compatibility environment" matters for **rollbacks**: a rule stored by a 1.32 apiserver that uses `format.dns1123Label()` keeps working after a downgrade to 1.31 (stored expressions use the extended environment), but cannot be newly written there. Review rule: in a chart that targets 1.31, any CEL function from the ❌ rows makes the CRD or policy uninstallable.

##### CEL review rules (verified pitfalls)

1. **`string(<int>)` in `messageExpression` blows the cost budget.** On 1.31 (and 1.30) the CRD is rejected with `estimated messageExpression cost exceeds budget by factor of more than 100x` for `'got ' + string(self.replicas)` even though `replicas` has `maximum: 99`. Use `'got %d'.format([self.replicas])`, which is accepted.
   ```yaml
   # BAD — CRD rejected on 1.31
   messageExpression: "'need >= 3 replicas, got ' + string(self.replicas)"
   # GOOD
   messageExpression: "'premium tier needs at least 3 replicas, got %d'.format([self.replicas])"
   ```
2. **`has()` takes a field selection, not a variable.** `!has(oldSelf)` fails to compile (`compilation failed`). With `optionalOldSelf: true` the variable is an optional: use `oldSelf.hasValue()` / `oldSelf.value()` (verified accepted on 1.31), and `optionalOldSelf` may only be set when the rule references `oldSelf` (`may not be set if oldSelf is not used in rule`).
3. **`self == oldSelf` does not stop removal.** Transition rules only run when both old and new values exist. Add a parent-level rule such as `!has(oldSelf.storageClassName) || has(self.storageClassName)` when "immutable" must also mean "cannot be unset" (verified: removal is then rejected with `spec: Invalid value: "object": storageClassName cannot be removed once set`).
4. **Unbounded inputs make the CRD uninstallable**, not slow: the cost estimator assumes maximum sizes. Every string/array/map reached by a rule needs `maxLength`/`maxItems`/`maxProperties`.
5. **Prefer `fieldPath` + `reason`** (`FieldValueInvalid`, `FieldValueForbidden`, `FieldValueRequired`, `FieldValueDuplicate`) so clients see the right field and status cause.
6. **Ratcheting** (beta-on since 1.30): unchanged invalid values in stored objects do not block unrelated updates. You may tighten validation for *new* writes, but still treat it as a breaking change for clients that create objects. On clusters that disabled `CRDValidationRatcheting`, a tightened rule blocks every update of an object that violates it.
7. `x-kubernetes-int-or-string` fields are dynamic in CEL; guard with `type(self.x) == string` or use `quantity()` only after a `pattern` restricts the input.
8. **CEL reserved keywords as field names** (`namespace`, `in`, `return`, …): 1.31 only *stores* expressions that reference them directly; full runtime support for new expressions comes in 1.32. Keep using the `__namespace__`-style escaping on 1.31.[^3]

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

`client.MergeFrom` sends a JSON merge patch without a `resourceVersion` precondition. If two writers own different list entries (for example condition types), use `client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})` or server-side apply for status with a dedicated field manager. (controller-runtime v0.19 has no typed `Status().Apply`; use `r.Status().Patch(ctx, u, client.Apply, owner)` with an unstructured apply payload if you need SSA for status.)

##### observedGeneration

Always set `status.observedGeneration = metadata.generation` after a successful reconcile:

```go
obj.Status.ObservedGeneration = obj.Generation
```

This allows external tools to detect whether the controller has processed the latest spec change.

Pods on 1.31 do **not** maintain `metadata.generation`/`status.observedGeneration` (that is 1.33 alpha → 1.35 GA). Code that waits for a Pod change must compare concrete status (`containerStatuses[].imageID`, readiness), not generations.

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
// changed reports whether anything (status, reason, message, observedGeneration) changed;
// use it to emit Events only on transitions.
```

CRD status conditions (`Established`, `NamesAccepted`) carry no `observedGeneration` on 1.31 (that is `CRDObservedGenerationTracking`, beta-off in 1.35 and on in 1.36). Tools that wait for a CRD update use `kubectl wait --for=condition=Established` plus a discovery check.

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
7. Use a **conversion webhook** when schemas diverge between versions. On 1.31 an invalid `caBundle` in `spec.conversion.webhook.clientConfig` keeps the CRD out of discovery, and updates that replace a valid `caBundle` with an invalid one are rejected.
8. **Before removing an old version from `spec.versions`**, every stored object must be rewritten to the new storage version and the old version removed from `status.storedVersions`. On 1.31 the in-tree `StorageVersionMigration` API is alpha and not served; use the out-of-tree kube-storage-version-migrator or a read/replace loop (`kubectl get <plural> -A -o json | kubectl replace -f -`), then remove the old version from `status.storedVersions` with `kubectl patch crd <name> --subresource=status`.

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

##### Reconciler Skeleton (controller-runtime v0.19)

The following reconciler compiles against k8s.io/* v0.31.14 + controller-runtime v0.19.7 and was exercised with envtest on the v1.31.0 control-plane binaries (CEL rejection, SSA-created Deployment with owner reference, status condition transitions, an Event, finalizer-driven deletion).

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
		r.Recorder.Eventf(rc, corev1.EventTypeWarning, "ApplyFailed", "apply Deployment: %v", err)
		return ctrl.Result{}, err
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
- `GenerationChangedPredicate` drops metadata-only updates (labels, annotations, finalizer edits by others). Deletion still gets through: when the apiserver sets `deletionTimestamp` on an object with finalizers it also increments `metadata.generation` (verified in envtest on 1.31: generation 1 → 2 after `Delete`), so the finalizer path runs and the reconciler above removed its finalizer.

##### Reconcile Return Conventions

| Return | Meaning |
|---|---|
| `ctrl.Result{}, nil` | Success — no requeue until a watched event fires |
| `ctrl.Result{}, err` | Transient error — requeued with per-item exponential backoff (5 ms doubling up to 1000 s by default) |
| `ctrl.Result{RequeueAfter: d}, nil` | Requeue after duration `d` (polling external state) |
| `ctrl.Result{Requeue: true}, nil` | Rate-limited requeue. Not deprecated in v0.19 (deprecated from v0.21), but prefer `RequeueAfter` or an error — its semantics are unclear to readers |
| `ctrl.Result{RequeueAfter: d}, err` | The error wins; `RequeueAfter` is ignored (and controller-runtime logs a warning). Pick one |
| `ctrl.Result{}, reconcile.TerminalError(err)` | Logged and counted, never retried |

controller-runtime v0.19 uses a plain rate-limited FIFO work queue (the priority queue is v0.20 experimental, v0.23 default). A single key is never processed concurrently, but keys are not ordered — do not depend on ordering.

##### Manager Setup

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
	Cache: cache.Options{
		DefaultTransform: cache.TransformStripManagedFields(),
	},
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

- Serve metrics with `SecureServing: true` and the `filters.WithAuthenticationAndAuthorization` filter (both exist in v0.19; the kube-rbac-proxy sidecar used by older Kubebuilder scaffolds is deprecated). Plain-HTTP `:8080` metrics endpoints are a review finding. The filter needs RBAC for `authentication.k8s.io/tokenreviews` and `authorization.k8s.io/subjectaccessreviews` `create`.
- `LeaderElection: true` for any controller that can run with more than one replica.
- `MaxConcurrentReconciles` > 1 is safe only if the reconciler holds no shared mutable state without locking; the queue guarantees a single key is never processed concurrently.

---

#### Server-Side Apply from Controllers

Server-side apply (SSA) with a stable field manager is the preferred way for a controller to own child objects: it sends only the fields the controller cares about, merges with fields set by other actors (HPA replicas, mutating webhooks, users), and resolves conflicts per field. **controller-runtime v0.19 has no typed `client.Apply(ctx, applyConfiguration)` method** (added in v0.22). On v0.19 there are two correct ways:

1. Build a client-go **apply configuration** and send it as an Apply patch through an `unstructured.Unstructured` (shown below, verified in envtest on 1.31), or
2. Use a client-go typed clientset: `clientset.AppsV1().Deployments(ns).Apply(ctx, ac, metav1.ApplyOptions{FieldManager: "...", Force: true})`.

```go
import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	dep := appsv1ac.Deployment(rc.Name, rc.Namespace).
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
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dep)
	if err != nil {
		return err
	}
	return r.Patch(ctx, &unstructured.Unstructured{Object: obj}, client.Apply, fieldOwner, client.ForceOwnership)
}
```

Verified in envtest on 1.31: the Deployment is created with one controller owner reference and `managedFields[0]` = manager `rediscluster-controller`, operation `Apply`.

```go
// BAD — typed struct with client.Apply (compiles and succeeds on v0.19)
dep := &appsv1.Deployment{
	TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
	ObjectMeta: metav1.ObjectMeta{Name: "typed", Namespace: "default"},
	Spec:       appsv1.DeploymentSpec{Selector: sel, Template: tmpl},
}
err := r.Patch(ctx, dep, client.Apply, client.FieldOwner("typed"), client.ForceOwnership)
```

The typed struct serialises zero values: the request body contains `"creationTimestamp":null`, `"strategy":{}`, `"resources":{}` and `"status":{}`, and the resulting managed fields show the manager owning `f:strategy`, `f:template.f:metadata.f:creationTimestamp` and `f:resources` — fields it never meant to set (verified in envtest on 1.31). Later, another actor's change to `strategy` conflicts with this manager.

SSA review rules:

- **Always send the complete desired intent** of the fields you own. A field omitted from a later apply is *removed* if this manager was its only owner. Building the apply configuration conditionally ("only set replicas if changed") is a bug.
- **Use apply configurations (or unstructured), not typed structs**, for SSA. A typed `appsv1.Deployment{}` serialises zero values and takes ownership of fields you never meant to own (see above).
- **One field manager per controller** (`client.FieldOwner`), constant across releases. Changing it orphans ownership and leaves stale fields behind.
- `client.ForceOwnership` is appropriate for a controller that is the source of truth for those fields; without it, a conflict (another manager set a different value) returns `409 Conflict`. Never force-apply fields that users are expected to tune (e.g. `replicas` when an HPA targets the Deployment) — omit them from the apply configuration instead.
- Do not mix SSA and `Update` on the same object from the same controller; `Update` takes ownership of every field (manager with operation `Update`) and later applies then fight the old manager entry.
- For CRDs without generated apply configurations, either generate them (`controller-gen applyconfiguration`, or client-go's `applyconfiguration-gen` v0.31) or use `client.MergeFrom` patches; never hand-build `unstructured` apply payloads without `apiVersion`/`kind`.
- Since 1.31 a no-op apply no longer bumps `resourceVersion` for custom resources and objects with empty maps (fixed regression) — controllers that compared `resourceVersion` to detect "something changed" after an apply behaved differently on 1.30.[^3]

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

Note: `GenerationChangedPredicate` ignores metadata-only updates (label, annotation and finalizer edits). If the controller must react to label or finalizer changes, combine predicates (`predicate.Or`) or write a custom one, as below. Since controller-runtime v0.18 predicates, handlers and sources are generic (`predicate.TypedPredicate[T]`, `source.Kind(cache, obj, handler, preds...)`); the untyped names are aliases for `T = client.Object`.

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

**[NEW in 1.31]** CRD field selectors are served, so a **server-side** list/watch of custom resources can be filtered by a selectable field (`client.MatchingFields{"spec.tier": "premium"}` on an uncached client / `cache.Options.ByObject[...].Field`). The controller-runtime **cache** still needs a local field index for `MatchingFields` on cached reads; a server-side field selector only reduces what the informer receives.

Do not set a short global `SyncPeriod` as a substitute for proper watches; resyncs replay every cached object through the handlers.

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

Verified on 1.31: on a custom resource, `finalizers: [cleanup]` is accepted with `Warning: metadata.finalizers: "cleanup": prefer a domain-qualified finalizer name to avoid accidental conflicts with other finalizer writers`; on built-in types (e.g. a ConfigMap) it is rejected: `name is neither a standard finalizer name nor is it fully qualified`.

##### Rules

- Add the finalizer **before** creating any external resource; remove it only after cleanup succeeded.
- Cleanup logic **must be idempotent** — the reconcile loop may be called multiple times while `DeletionTimestamp` is set.
- Never add a new finalizer after `DeletionTimestamp` is set — only removal is allowed.
- Do not silently swallow errors; failing cleanup should be retried (return the error) and surfaced as a condition + Warning Event.
- Do not create resources inside a namespace that has `DeletionTimestamp` set.
- Removing the finalizer when the object is already gone returns NotFound — wrap with `client.IgnoreNotFound`.
- PersistentVolumes on 1.31 carry the finalizers `kubernetes.io/pv-controller` and `external-provisioner.volume.kubernetes.io/finalizer` (`HonorPVReclaimPolicy`, beta-on). Tools that "clean up stuck PVs" by stripping finalizers now leak the backing storage they were supposed to delete — flag it.

---

#### Owner References & Garbage Collection

##### Set Owner References for All Child Resources

```go
// controllerutil.SetControllerReference sets controller=true and blockOwnerDeletion=true
if err := controllerutil.SetControllerReference(owner, childDeployment, r.Scheme); err != nil {
    return err
}
```

With SSA, set the owner reference inside the apply configuration (as above) so the controller's field manager owns it.

##### Rules

| Rule | Notes |
|---|---|
| Owner must be in **same namespace** as the owned resource | A namespaced dependent whose same-namespace owner is missing is treated as having an absent owner → deleted by GC |
| A namespaced owner **cannot** own a cluster-scoped resource | The owner reference is unresolvable — the dependent is **not** garbage collected and gets an `OwnerRefInvalidNamespace` event; `SetControllerReference` returns an error |
| A cluster-scoped owner **can** own a cluster-scoped resource | Permitted |
| Use `controllerutil.SetControllerReference` (not hand-built refs) | Sets `controller: true` and `blockOwnerDeletion: true`, refuses a second controller owner |
| One resource should have **at most one controller owner** | Multiple owners only for non-controller (informational) references |
| `blockOwnerDeletion: true` requires `update` on the owner's `finalizers` subresource when the `OwnerReferencesPermissionEnforcement` admission plugin is enabled (not in the default plugin set, common in hardened/OpenShift clusters) | Grant `<owner-resource>/finalizers` `update` — Kubebuilder's marker does — or child creates fail with `cannot set blockOwnerDeletion if an ownerReference refers to a resource you can't set finalizers on` |
| Node objects must not carry owner references | CVE-2025-5187: before v1.31.12, a kubelet could add an owner reference to its own Node object and get it garbage-collected (deleted). Fixed in NodeRestriction in v1.31.12 (nodes may no longer change their `ownerReferences`) |

##### When to Use Finalizers vs. OwnerReferences

| Scenario | Use |
|---|---|
| Child resource in same namespace | `ownerReferences` (automatic GC) |
| Cluster-scoped child of a namespaced owner | `finalizer` on the owner (or use a cluster-scoped owner) |
| External resource (cloud, DNS) | `finalizer` |
| Cross-namespace cleanup needed | `finalizer` |

---

#### Events

controller-runtime v0.19's `mgr.GetEventRecorderFor(name)` returns a `k8s.io/client-go/tools/record.EventRecorder` that writes **core/v1 Events** (`""` API group). The events.k8s.io/v1 recorder (`mgr.GetEventRecorder`) only arrives in v0.23 — do not suggest it.

```go
// record.EventRecorder: object, type, reason, message format, args
r.Recorder.Eventf(rc, corev1.EventTypeWarning, "ApplyFailed", "apply Deployment: %v", err)
r.Recorder.Eventf(rc, corev1.EventTypeNormal, "Available", "all %d replicas ready", rc.Spec.Replicas)
```

Review points:

- **RBAC**: the controller needs `""` (core) `events` `create;patch` (`// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch`). Without it the recorder logs the failure and reconciliation continues — Events are silently lost.
- The message is a format string — never pass user-controlled text as the format (`Eventf(obj, t, r, err.Error())` is a format-string bug); use `"%v", err` or `Event(obj, t, r, msg)`.
- `reason` is a CamelCase machine-readable word; do not put variable data in it (the broadcaster aggregates by reason).
- Emit Events on transitions (use the `changed` result of `meta.SetStatusCondition`), not on every reconcile.

---

#### Admission: CEL Policies and Webhooks

##### Validation Hierarchy (prefer left-to-right)

```
CRD schema + CEL rules  →  ValidatingAdmissionPolicy (CEL, GA 1.30)  →  Validating webhook
(per-CRD, no extra process)  (cluster policy, no extra process)          (extra infra, TLS, HA)
```

Mutation on 1.31: a **mutating webhook** is the only mechanism. MutatingAdmissionPolicy does not exist in 1.31 (first alpha in 1.32), even though `--help` lists a `MutatingAdmissionPolicy` gate.

##### ValidatingAdmissionPolicy (verified on 1.31)[^10]

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

A Deployment whose init container uses a tag is rejected with:
`ValidatingAdmissionPolicy 'require-digest-and-limits' with binding 'require-digest-and-limits' denied request: all images must be pinned by digest: migrate`.

Review points for policies:

- A policy without a **binding** does nothing; a binding's `validationActions` decides `Deny`/`Warn`/`Audit`. `[Deny, Warn]` is rejected (verified: `must not contain both Deny and Warn`); roll out new policies with `[Warn, Audit]` first.
- Use `?`/`orValue` optional chaining (`object.spec.template.spec.?initContainers.orValue([])`) for optional fields; plain `object.spec.x` on an absent field is a runtime error that triggers `failurePolicy`.
- Use `matchConditions` to exclude system users and controllers, otherwise a broken policy can block kube-system controllers.
- `paramKind` + `paramRef` for per-namespace configuration; `paramRef.parameterNotFoundAction` is required — `Deny` subjects requests without a matching param to the policy's `failurePolicy`, `Allow` lets them through. Choose deliberately.
- `string(...)` in **VAP** expressions is fine (VAP has a per-expression runtime cost limit, not the CRD's static estimate); the CRD `messageExpression` pitfall above is specific to CRD validation rules.
- **Strict cost enforcement is off on 1.31** (`StrictCostEnforcementForVAP`/`ForWebhooks`, beta/off): expressions using extended libraries (`regex`, `url`, `quantity`, lists) are under-costed on 1.31 and may be rejected for cost after an upgrade to 1.32, where strict costing is GA. Keep policy expressions small and bounded.
- Use `admissionregistration.k8s.io/v1` for VAP. `v1beta1` is not served by default on 1.31.

##### Admission webhooks (controller-runtime v0.19)

On v0.19 the webhook builder is **not** generic: `ctrl.NewWebhookManagedBy(mgr).For(&cachev1.RedisCluster{})` with an `admission.CustomValidator` / `admission.CustomDefaulter` whose methods take `runtime.Object`. The older `admission.Validator`/`Defaulter` interfaces (methods implemented on the API type itself, `webhook.Validator` in Kubebuilder v3 scaffolds) are **deprecated** in v0.19 ("Deprecated: Use CustomValidator instead") and removed in v0.20.[^11]

```go
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

Review points:

- **Always check the type assertion** — an unchecked `obj.(*cachev1.RedisCluster)` panics if the webhook is ever registered for another type. controller-runtime v0.19 recovers webhook panics by default (`RecoverPanic` unset means recover) and answers with an HTTP 500 `panic: … [recovered]` error, so the request is rejected (or let through with `failurePolicy: Ignore`) instead of getting a meaningful message.
- `ctrl.NewWebhookManagedBy(mgr, &cachev1.RedisCluster{})` (two arguments) and `admission.Validator[*T]` are controller-runtime **v0.23** APIs; they do not compile against v0.19.
- The tier-downgrade check above is a simple transition rule (`self.tier == oldSelf.tier || oldSelf.tier != 'premium'`) and belongs in CEL; keep webhooks for checks that need external state.

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
- TLS certificates **must** be rotated; use cert-manager or a managed CA bundle injection, and never SHA-1 signed certificates (see deprecations).

---

#### RBAC & Least Privilege

##### Principles

1. **Least privilege** — grant only the verbs/resources explicitly required.
2. Prefer `Role` + `RoleBinding` (namespaced) over `ClusterRole` + `ClusterRoleBinding`.
3. **No wildcard verbs** (`*`) or **wildcard resources** (`*`).
4. **No `cluster-admin`** service accounts.
5. Mount service account tokens only when the workload needs API access (`automountServiceAccountToken: false` as default).
6. Treat `escalate`, `bind`, `impersonate`, `nodes/proxy`, `serviceaccounts/token` `create`, and `pods/exec` as privileged.

##### `pods/exec`, `pods/attach`, `pods/portforward` on 1.31

On 1.31, kubectl uses **WebSockets** for exec/attach (beta since 1.30) and port-forward (beta-on in 1.31). The apiserver authorizes a WebSocket upgrade, which is an HTTP `GET`, with the **`get`** verb, whereas SPDY requests (`POST`) need **`create`**. A role with only `get` on `pods/exec` therefore lets a WebSocket client exec into Pods on 1.31; this loophole is closed in 1.35 (`AuthorizePodWebsocketUpgradeCreatePermission`).[^12]

Verified on a 1.31.0 apiserver with a ServiceAccount bound only to `get` on `pods` and `pods/exec` (`kubectl auth can-i create pods/exec` → `no`):

```text
WebSocket upgrade (GET .../pods/web/exec?command=ls):  400 "pod web does not have a host assigned"   # authorization passed
SPDY-style POST  (same URL):                           403 cannot create resource "pods/exec"
```

```yaml
# BAD — looks "read-only", but allows exec over WebSockets on 1.31
rules:
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["get"]
# GOOD — grant exec explicitly (create) to the few subjects that need it, and
# never put pods/exec, pods/attach or pods/portforward in a "viewer" role
rules:
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["create"]
```

The `get` + `create` pair (`verbs: ["get", "create"]`) keeps tooling working on both 1.31 and ≥ 1.35. Audit custom read-only roles for `get` on these subresources — they are equivalent to exec rights on 1.31.

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

// Events: controller-runtime v0.19 recorder writes core/v1 Events
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Leader election
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
```

Secrets: grant `get` on specific `resourceNames` where possible; `list`/`watch` on Secrets cluster-wide is equivalent to reading every Secret. Authorization with field/label selectors (`AuthorizeWithSelectors`) is alpha on 1.31: RBAC cannot express "list only Secrets with label X".

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

**[NEW in 1.31 — beta, on]** Service-account tokens can be bound to a **Node** object (`kubectl create token SA --bound-object-kind Node --bound-object-name n1`); such a token becomes invalid when the Node is deleted (verified on 1.31; kubectl 1.30 rejects `--bound-object-kind Node`). Bound tokens also carry a `jti` and the node name/UID of the Pod (`ServiceAccountTokenJTI`, `ServiceAccountTokenPodNodeInfo`, beta since 1.30) for audit correlation.[^1]

---

#### Workload Manifests: Pods

##### A production Deployment for 1.31 (verified)

Applied to a 1.31.0 apiserver into a namespace enforcing Pod Security `restricted:v1.31`; a Pod built from the template passes admission.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: payments
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: v1.31
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
        appArmorProfile: {type: RuntimeDefault}   # GA in 1.31
      topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: ScheduleAnyway
        labelSelector:
          matchLabels: {app.kubernetes.io/name: api}
        matchLabelKeys: [pod-template-hash]
      initContainers:
      - name: log-shipper                      # native sidecar (beta, on since 1.29)
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
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef: {fieldPath: metadata.namespace}
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
            sleep: {seconds: 5}                # PodLifecycleSleepAction, beta-on since 1.30
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities: {drop: [ALL]}
        volumeMounts:
        - {name: tmp, mountPath: /tmp}
      volumes:
      - name: tmp
        emptyDir: {sizeLimit: 64Mi}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: api
  namespace: payments
spec:
  minAvailable: 2
  unhealthyPodEvictionPolicy: AlwaysAllow       # GA in 1.31
  selector:
    matchLabels: {app.kubernetes.io/name: api}
```

A bare Pod in the same namespace without a security context is rejected: `violates PodSecurity "restricted:v1.31": allowPrivilegeEscalation != false ..., unrestricted capabilities ..., runAsNonRoot != true ..., seccompProfile ...`.

Review notes on this template:

- On 1.31 the apiserver **mirrors** `appArmorProfile` fields into the deprecated `container.apparmor.security.beta.kubernetes.io/<container>` annotations on the Pod (verified: both `api` and `log-shipper` get `runtime/default`). Tools that diff Pod annotations must ignore them; the mirroring stops in 1.34.
- `memory` requests = limits avoids OOM surprises for latency-critical services; `cpu` limits are a policy decision (throttling) — flag missing **memory** limits, not missing CPU limits.
- `matchLabelKeys: [pod-template-hash]` spreads each ReplicaSet revision independently (beta-on since 1.27).
- `maxUnavailable: 0` + `maxSurge: 1` with a PDB of `minAvailable: 2` and three replicas keeps two Pods serving during rollouts and node drains.
- A Go ≥ 1.25 binary reads the cgroup CPU limit itself; for older Go binaries set `GOMAXPROCS` from `resourceFieldRef: {resource: limits.cpu}` (or use `automaxprocs`), otherwise the runtime uses the node's CPU count.[^13]
- Pinning `enforce-version` to `v1.31` keeps the rules stable across control-plane upgrades. A 1.30 apiserver also accepted `enforce-version: v1.31` (it evaluates unknown future versions with its latest policy) — that does not mean the 1.31 policy is enforced there.

##### **[GA in 1.31]** AppArmor fields

```yaml
spec:
  securityContext:
    appArmorProfile:
      type: RuntimeDefault                 # pod default
  containers:
  - name: app
    securityContext:
      appArmorProfile:
        type: Localhost
        localhostProfile: k8s-apparmor-example-deny-write   # must be loaded on every node
```

- **BAD**: new manifests that still use `container.apparmor.security.beta.kubernetes.io/<container>: runtime/default`. Verified on 1.31: `Warning: metadata.annotations[container.apparmor.security.beta.kubernetes.io/aa]: deprecated since v1.30; use the "appArmorProfile" field instead`; the apiserver copies the annotation into the container's `appArmorProfile` field on create.
- When both an annotation and a pod-level field are present, the container-level value derived from the annotation wins for that container (verified: annotation `runtime/default` + pod `Unconfined` → container `RuntimeDefault`). An annotation that contradicts the **container-level** field is rejected (verified: `appArmorProfile.type: Forbidden: apparmor type in annotation and field must match`). Do not mix the two mechanisms.
- `Localhost` profiles must exist on every node; since 1.31 the kubelet **rejects** Pods that request AppArmor on a node where AppArmor is disabled (previously ignored), so a DaemonSet with an AppArmor profile now fails on nodes without AppArmor (e.g. some SELinux-based distributions) instead of running unconfined.[^3]
- AppArmor fields are Linux-only; the Pod Security baseline policy forbids `Unconfined`.

##### Native sidecars (beta, on since 1.29)

```yaml
initContainers:
- name: proxy
  image: registry.example.com/proxy:1.4
  restartPolicy: Always          # makes it a sidecar
  startupProbe:
    httpGet: {path: /ready, port: 15021}
containers:
- name: app
  image: registry.example.com/app:1.0
```

- Sidecars start before regular containers, keep running, are restarted independently, and are stopped **after** the main containers; a sidecar's `startupProbe` gates the next init container. They no longer block Job completion (and since 1.31 their finish time counts towards the Job's completion time).[^3]
- Do not model sidecars as extra regular containers in Jobs — the Job never completes.
- Graceful-termination bug fixed in 1.31: sidecars of a Pod that had not finished initialising did not terminate gracefully.[^3]

##### Fields that a default 1.31 apiserver silently drops

Verified with `kubectl apply --dry-run=server -o yaml` on v1.31.0 (no warning is printed):

| Manifest field | Gate (1.31) | What happens on 1.31 |
|---|---|---|
| `volumes[].image` (OCI image volume) | `ImageVolume` alpha | The **whole volume is removed, and so is every `volumeMount` that referenced it** — the container starts without the mount and the application fails at runtime |
| `securityContext.supplementalGroupsPolicy` | `SupplementalGroupsPolicy` alpha | Dropped; image `/etc/group` memberships are merged as before |
| `hostUsers: false` | `UserNamespacesSupport` beta, **off** | Dropped; the Pod runs in the host user namespace (container root = host root) |
| `containers[].securityContext.procMount: Unmasked` | `ProcMountType` beta, off | Reset to `Default` |
| `containers[].resizePolicy` | `InPlacePodVerticalScaling` alpha | Dropped (`kubectl patch --subresource resize` is rejected client-side: `Must be one of [status scale]`) |
| `Job.spec.managedBy` | `JobManagedBy` alpha | Dropped — the built-in Job controller runs the Job |
| `StatefulSet.spec.updateStrategy.rollingUpdate.maxUnavailable` | `MaxUnavailableStatefulSet` alpha | Dropped (stored `rollingUpdate` is `{partition: 0}`, also on 1.30 and 1.32); updates proceed one Pod at a time |

A security reviewer must treat a manifest that relies on `hostUsers: false` or `supplementalGroupsPolicy: Strict` for isolation as **unprotected** on a default 1.31 cluster. `kubectl apply` keeps the dropped fields in the `kubectl.kubernetes.io/last-applied-configuration` annotation (verified), so the object *looks* configured when that annotation is inspected; check the live `spec` instead.

##### `recursiveReadOnly` mounts (beta, on in 1.31)

```yaml
volumeMounts:
- name: data
  mountPath: /data
  readOnly: true
  recursiveReadOnly: IfPossible    # or Enabled; requires readOnly: true
```

`readOnly: true` alone does **not** make sub-mounts read-only. `Enabled` fails the Pod when the runtime cannot do it (requires Linux ≥ 5.12 and a CRI runtime that reports the feature, e.g. containerd 2.0); `IfPossible` falls back silently. Use `Enabled` only when the node fleet supports it. On 1.30 the field is dropped (alpha/off).[^14]

##### Other Pod rules

- **`lifecycle.preStop.sleep`** (beta-on since 1.30) replaces `exec: ["sleep","5"]` in images without a shell; `seconds` must be `1 … terminationGracePeriodSeconds-1` on 1.31 (`0` is rejected).
- **`restartPolicy: Never` Pods** have no per-container restart rules on 1.31 (`restartPolicyRules` is 1.34+).
- **No in-place resize**: changing `resources` of a running Pod is rejected; change the controller's template (rollout) instead.
- **User namespaces** (`hostUsers: false`) need the `UserNamespacesSupport` gate on apiserver and kubelet, kernel ≥ 6.3 for idmap mounts on most filesystems, and a runtime that supports it. Do not assume it on 1.31.
- **`IfNotPresent` and private images**: on 1.31 a Pod without pull credentials can still start from a private image that another Pod pulled onto the same node (credential re-verification is a 1.33 alpha feature). Multi-tenant clusters must enforce `imagePullPolicy: Always` (the `AlwaysPullImages` admission plugin) to protect private images.
- **Image references**: always tag or pin by digest; `:latest` implies `imagePullPolicy: Always` by default.

---

#### Workload Manifests: Controllers (Deployment, StatefulSet, Job, PDB, HPA)

##### Deployments and ReplicaSets

- `spec.selector` is immutable; changing labels in the selector requires a new Deployment.
- `progressDeadlineSeconds` only marks the `Progressing` condition `False`; it does not roll back.
- **[GA in 1.31]** `LogarithmicScaleDown`: on scale-down the ReplicaSet controller groups Pods into logarithmic age buckets and picks randomly within a bucket. Use `controller.kubernetes.io/pod-deletion-cost` (beta since 1.22) to influence which Pods go first; never rely on "the newest Pod is removed".
- `status.terminatingReplicas` does not exist on 1.31 (1.33 alpha).

##### StatefulSets

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: kv
spec:
  serviceName: kv
  replicas: 3
  ordinals: {start: 5}              # GA in 1.31: Pods kv-5, kv-6, kv-7
  persistentVolumeClaimRetentionPolicy:
    whenDeleted: Delete             # beta-on since 1.27, GA 1.32
    whenScaled: Retain
  selector: {matchLabels: {app: kv}}
  updateStrategy:
    type: RollingUpdate
  template:
    metadata: {labels: {app: kv}}
    spec:
      containers: [{name: kv, image: registry.example.com/kv:3.1}]
```

- **[GA in 1.31]** `ordinals.start` lets a StatefulSet start at a non-zero ordinal (for slicing a StatefulSet across clusters or namespaces during migrations).
- `persistentVolumeClaimRetentionPolicy.whenDeleted: Delete` only for disposable data; the default `Retain` keeps PVCs. 1.31 fixed the auto-delete logic to respect PVCs whose controlling owner is not the StatefulSet.[^3]
- `rollingUpdate.maxUnavailable` is **dropped** by a default 1.31 apiserver (`MaxUnavailableStatefulSet` alpha/off; verified on v1.30.3, v1.31.0 and v1.32.0: the stored object only has `rollingUpdate: {partition: 0}`). Rolling updates replace one Pod at a time; a manifest or chart value that sets `maxUnavailable` silently does nothing and causes GitOps drift. It only works where the gate is enabled on both kube-apiserver and kube-controller-manager.
- `serviceName` must reference an existing headless Service for stable DNS.

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
  backoffLimitPerIndex: 2          # beta-on since 1.29
  maxFailedIndexes: 2
  podReplacementPolicy: Failed     # beta-on since 1.29
  successPolicy:                   # beta-on in 1.31
    rules:
    - succeededIndexes: "0"
  podFailurePolicy:                # GA in 1.31
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

Verified: 1.31 keeps every field; 1.30 silently drops `successPolicy`; a `managedBy` field is dropped on 1.31 and kept on 1.32.

- **[GA in 1.31]** `podFailurePolicy` requires `restartPolicy: Never`. Combine `Ignore` on the `DisruptionTarget` condition (GA in 1.31) with `FailJob` on non-retriable exit codes so preemption and node drains do not consume `backoffLimit`.
- **[NEW in 1.31 — beta, on]** `successPolicy` declares an Indexed Job successful when specific indexes (`succeededIndexes`) or a number of indexes (`succeededCount`) succeed; remaining Pods are terminated and the Job gets the `SuccessCriteriaMet` condition before `Complete`. With the gate on, Jobs that simply reach `completions` also get `SuccessCriteriaMet` (reason `CompletionsReached`). Controllers that wait for `Complete` keep working; controllers that treat *any* new condition as failure need fixing.[^3]
- **[GA in 1.31]** Elastic Indexed Jobs: `completions` may be updated together with `parallelism` (they must remain equal). Verified: patching both to 6 succeeds; patching only `completions` fails with `can only be modified in tandem with spec.parallelism`.
- Terminal Job conditions (`Failed`, `Complete`) are only set once all Pods are terminal since 1.31 (`.status.terminating` tracked faster); tools that saw `Failed` while Pods were still running now observe it later.[^3]
- `backoffLimitPerIndex` requires `completionMode: Indexed`.
- **Do not use `spec.managedBy`** on 1.31 — it is dropped and the built-in controller runs the Job (see "Do not use").
- For CronJobs, always set `timeZone` (GA 1.27) and `concurrencyPolicy` explicitly; `startingDeadlineSeconds` avoids catch-up storms after controller downtime. `lastSuccessfulTime` is set reliably from 1.31.

##### PodDisruptionBudget — **[GA in 1.31]** `unhealthyPodEvictionPolicy`

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata: {name: api}
spec:
  minAvailable: 2
  unhealthyPodEvictionPolicy: AlwaysAllow
  selector: {matchLabels: {app: api}}
```

`AlwaysAllow` lets drains evict Pods that are running but not ready even when the budget is exhausted — the recommended setting for most workloads, because otherwise a crash-looping Deployment blocks node drains forever. Keep the default `IfHealthyBudget` only where an unready Pod still holds data that must not be lost.

##### HorizontalPodAutoscaler

- Use `autoscaling/v2`; `behavior.scaleUp/scaleDown` policies and `stabilizationWindowSeconds` are GA. Per-HPA `tolerance` does **not** exist on 1.31 (1.33 alpha); the global 10% tolerance applies.
- `ContainerResource` metrics (GA 1.30) let an HPA ignore sidecar usage.
- `minReplicas: 0` needs the alpha `HPAScaleToZero` gate — not available on a default 1.31 cluster.
- Do not set `spec.replicas` in a Deployment managed by an HPA when applying with SSA or `kubectl apply` — every apply resets the replica count. Omit the field.

---

#### Networking: Services & NetworkPolicy

##### **[NEW in 1.31 — beta, on]** `trafficDistribution`

```yaml
apiVersion: v1
kind: Service
metadata:
  name: zonal-api
spec:
  selector: {app: api}
  ports: [{name: http, port: 80, targetPort: 8080}]
  trafficDistribution: PreferClose    # only accepted value on 1.31
```

- Verified: 1.31 keeps the field; **1.30 drops it silently** (gate alpha/off); `PreferSameZone`/`PreferSameNode` are rejected on 1.31 and 1.32 (`Unsupported value: "PreferSameZone": supported values: "PreferClose"`). They are alpha (gate off) in 1.33 and beta-on in 1.34.
- `PreferClose` is a *preference*: kube-proxy (and conforming dataplanes) routes to topologically close endpoints when they exist and falls back to remote ones. It requires a kube-proxy/CNI that implements it.
- Do not combine `trafficDistribution` with the `service.kubernetes.io/topology-mode: Auto` annotation; topology-aware routing hints take precedence and the combination is confusing.
- `internalTrafficPolicy: Local` is a hard requirement (no fallback), not a preference — flag it where availability matters.

##### Service names, IPs and `externalIPs`

- Service names must be RFC 1035 labels (no leading digit) — see Naming.
- Use canonical IP literals. 1.31 warns on `010.000.000.005` (`IP address was accepted, but will be invalid in a future Kubernetes release`); 1.32–1.35 still only warn, 1.36 rejects it (`StrictIPCIDRValidation` beta-on).
- `spec.externalIPs` lets any user who can create Services intercept traffic for arbitrary IPs (CVE-2020-8554). Clusters should block it with a ValidatingAdmissionPolicy (`!has(object.spec.externalIPs)`) or the `DenyServiceExternalIPs` admission plugin; new manifests should use `LoadBalancer`, `NodePort` or Gateway API.
- Services support field selectors on `spec.clusterIP` and `spec.type` since 1.31. Verified: `kubectl get svc --field-selector spec.type=LoadBalancer` works on 1.31; 1.30 answers `"spec.type" is not a known field selector`.

##### kube-proxy and load balancers on 1.31

- nftables mode is beta and selectable (`mode: nftables`); it listens for NodePorts only on the node's primary IPs by default. iptables remains the default; IPVS still works (deprecated only in 1.35).
- New `nodePortAddresses: ["primary"]` value for iptables/ipvs restricts NodePorts to primary node IPs — recommended for hardened clusters.[^3]
- `status.loadBalancer.ingress[].ipMode: Proxy` (beta-on since 1.30) tells kube-proxy not to short-circuit traffic to a proxy-mode load balancer; cloud controllers that implement PROXY protocol should set it.

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

##### Ingress

New HTTP routing should target Gateway API (`gateway.networking.k8s.io/v1`, installed as CRDs, not built into the apiserver). Since 1.31, SSA treats `Ingress.spec.defaultBackend` as an atomic struct: whoever applies it owns the whole struct (switching `port.number` ↔ `port.name` no longer leaves both set).[^3] Flag `nginx.ingress.kubernetes.io/*-snippet` annotations (raw NGINX configuration injection); the ingress-nginx project has since been retired.

---

#### Storage

- **[NEW in 1.31 — beta, on]** **Always honor the reclaim policy** (`HonorPVReclaimPolicy`, KEP-2644): a PV with `persistentVolumeReclaimPolicy: Delete` now has its backing volume deleted even when the PV object is deleted before its PVC. The PV controller and CSI external-provisioner add the finalizers `kubernetes.io/pv-controller` / `external-provisioner.volume.kubernetes.io/finalizer` to implement this. Consequences: `kubectl delete pv` on a `Delete` PV really destroys data; scripts that force-remove PV finalizers leak storage; set `Retain` on any PV whose data must outlive its claim.[^1]
- **[GA in 1.31]** `PersistentVolume.status.lastPhaseTransitionTime` records when the PV last changed phase (populated on the first transition after the upgrade). Use it for "Released for > N days" cleanup reports instead of annotations.
- **Removed**: `cephfs` and `rbd` in-tree volume sources (use ceph-csi). In-tree `awsElasticBlockStore`, `azureDisk`, `azureFile`, `gcePersistentDisk`, `cinder`, `vsphereVolume` sources are served through CSI migration (their in-tree code is gone) — new manifests must use CSI `StorageClass`es.
- **VolumeAttributesClass** (`storage.k8s.io/v1beta1`, `volumeAttributesClassName` on PVCs) is beta and **off**: the field is dropped and the API is not served on a default 1.31 cluster.
- PV annotation `volume.beta.kubernetes.io/mount-options` triggers a deprecation warning since 1.31 (verified: `deprecated since v1.31; use "mountOptions" attribute instead`; no warning on 1.30) — use `spec.mountOptions`.
- `RecoverVolumeExpansionFailure` is alpha on 1.31 (users cannot shrink a failed expansion back); plan PVC size increases carefully.

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
    appArmorProfile:
      type: RuntimeDefault
  containers:
    - name: manager
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: ["ALL"]
```

##### Pod Security Standards

Operator and workload namespaces should enforce **restricted**, pinned to the cluster version so a control-plane upgrade does not silently change the rules:

```yaml
pod-security.kubernetes.io/enforce: restricted
pod-security.kubernetes.io/enforce-version: v1.31
pod-security.kubernetes.io/warn: restricted
pod-security.kubernetes.io/audit: restricted
```

##### Secrets Management

- Never embed secrets in CRD `spec` fields; reference them (`secretRef: {name: ...}`).
- Do not log secret values, and do not put them in Event messages or condition messages.
- Use projected service-account tokens with an `audience` and short `expirationSeconds` instead of long-lived token Secrets (`kubernetes.io/service-account-token` Secrets are no longer auto-created since 1.24, and unused legacy ones are cleaned up by default since 1.29, GA in 1.30).
- Encryption at rest: since 1.31 the `--encryption-provider-config` file is decoded strictly — a typo or duplicate field now **fails** apiserver startup (or, with automatic reload, the new file is rejected and the last valid one stays active).[^3]
- Consider External Secrets Operator or Vault Agent for rotation.

##### Image Security

- Pin images by **digest** in production manifests; enforce it with a ValidatingAdmissionPolicy (see above).
- Use distroless or scratch base images for operator binaries.
- Scan images (Trivy, Grype) in CI; never use `latest`.
- On multi-tenant nodes, enable the `AlwaysPullImages` admission plugin — 1.31 has no kubelet re-verification of cached private images.

##### Known 1.31 CVEs reviewers should map to patch levels

| CVE | Affected 1.31 versions | Fixed in | Note |
|---|---|---|---|
| CVE-2024-9042 (Windows nodes: command injection via `nodes/*/logs/query`) | ≤ v1.31.4 | v1.31.5 | Restrict `nodes/proxy`/log-query access |
| CVE-2025-0426 (kubelet checkpoint API node DoS via read-only port) | v1.31.0–v1.31.5 | v1.31.6 | Disable the kubelet read-only port (`readOnlyPort: 0`) |
| CVE-2025-5187 (node can delete itself via OwnerReference) | v1.31.0–v1.31.11 | v1.31.12 | NodeRestriction fix |

Infrastructure code that pins `v1.31.x` below v1.31.12 (kind node images, kubeadm `kubernetesVersion`, managed-cluster version strings) is a finding; anything on 1.31 at all is end-of-life.[^3]

##### Node-level 1.31 changes reviewers will meet in infrastructure code

- kubelet: `--keep-terminated-pod-volumes` removed; `--fail-cgroupv1` / `failCgroupV1` added (default `false`); cgroup v1 logs warnings; `KubeletCgroupDriverFromCRI` beta-on (the `cgroupDriver` setting is ignored when the runtime reports it — containerd ≥ 2.0 / CRI-O ≥ 1.28); `KubeletSeparateDiskGC` beta-on.
- kube-apiserver/kube-controller-manager/kubelet: `--cloud-provider` must be `external` or empty; the in-tree providers are gone.
- kube-proxy: `--proxy-port-range` removed; nftables mode available (kernel ≥ 5.13).
- kubeadm: `v1beta4` configuration API introduced (v1beta3 still accepted); etcd must be ≥ 3.5.11 for kubeadm-managed probes.[^3]
- `/sys/devices/virtual/powercap` is masked by default in containers (power side-channel), and v1.31.13 masks thermal-interrupt info in `/proc` and `/sys`.[^3]

---

#### client-go & apimachinery in v0.31

##### Fake clients

```go
// GOOD — v0.31: field-managed fake, supports server-side apply semantics
cs := fake.NewClientset(existingObjects...)

// BAD in new tests — deprecated in v0.31 ("NewClientset replaces this with support for field management")
cs := fake.NewSimpleClientset(existingObjects...)
```

- `fake.NewClientset` is **new in client-go v0.31**; it does not exist in v0.30 — do not suggest it for older modules.
- controller-runtime's fake client (v0.19) needs `WithStatusSubresource(&MyType{})` when the code writes status. Verified: without it, `Status().Patch` returns `NotFound` and a plain `Update` writes `status` together with `spec`; with it, `Status().Patch` works and `Update` ignores status changes (as the real apiserver does).
- **controller-runtime v0.19's fake client does not support server-side apply.** Verified: `Patch(ctx, obj, client.Apply, ...)` on a missing object returns `NotFound`, and on an existing object `apply patches are not supported in the fake client`. Apply patches are rejected by the fake client up to controller-runtime v0.21 (the check is gone in v0.22). Reconcilers that apply children must be tested with envtest.
- Fakes do not run admission, CEL validation, defaulting or garbage collection. Anything that depends on those belongs in an envtest test.

##### Informers and caches

- `cache.WaitForNamedCacheSync(name, stopCh, synced...)` is the API in v0.31 (`WaitForNamedCacheSyncWithContext` does not exist yet).
- `WatchListClient` is beta but **off** in client-go v0.31 — informers use paginated LIST + WATCH unless `KUBE_FEATURE_WatchListClient=true` is set; the server-side `WatchList` gate is alpha/off on 1.31.
- `ConsistentListFromCache` (beta-on) makes LIST without `resourceVersion` consistent *and* served from the watch cache — a stale-read concern is not a reason to add `resourceVersion=""` workarounds on 1.31. Requires etcd ≥ 3.5.13 (the apiserver warns on older etcd).[^3]
- Prefer controller-runtime's cache options (`DefaultTransform: cache.TransformStripManagedFields()`, `ByObject` label/field selectors) to reduce memory instead of hand-rolled informers.
- `k8s.io/apimachinery/pkg/util/runtime`: `HandleError`/`HandleCrash` gained context-aware variants (`HandleErrorWithContext`, `HandleCrashWithContext`) in v0.31; prefer them in code that already has a `ctx` (contextual logging).[^3]

##### Clients and versions

- Keep `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` and controller-runtime on matching minors. A `go.mod` with `k8s.io/client-go v0.31.x` and `k8s.io/api v0.30.x` (or controller-runtime v0.18 with v0.31 libraries) is a review finding.
- Generated clientsets / listers / apply configurations for your CRDs must be regenerated with code-generator v0.31 when the libraries move to v0.31; v0.31 `client-gen` fakes have the same field-management semantics as `fake.NewClientset`.
- `k8s.io/utils/pointer` is deprecated in favour of `k8s.io/utils/ptr` (`ptr.To(x)`, `ptr.Deref(p, def)`).
- `resourceVersion` values are opaque strings on 1.31 — never parse and compare them (comparable resource versions are a 1.35 feature).
- Kubernetes API types still implement gogo `ProtoMessage()` in v0.31; do not pass them to `google.golang.org/protobuf` (`proto.Marshal`) anyway — they were never valid `google.golang.org/protobuf` messages. Use `k8s.io/apimachinery/pkg/runtime/serializer/protobuf`.

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

controller-runtime already adds `controller`, `namespace`, `name`, `reconcileID` to the context logger; do not repeat them. Since 1.31 component-base also configures the `log/slog` default logger together with klog, so `slog` output from libraries ends up in the same stream.[^3]

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
- controller-runtime already exports `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds`, `controller_runtime_terminal_reconcile_errors_total` and workqueue metrics — do not duplicate them.

##### Health Probes

```go
utilruntime.Must(mgr.AddHealthzCheck("healthz", healthz.Ping))
utilruntime.Must(mgr.AddReadyzCheck("readyz", healthz.Ping))

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
- **Never** return a non-nil error together with `RequeueAfter`/`Requeue` — the error wins; choose one.
- Prefer `RequeueAfter` or an error over `Requeue: true`.
- Use `client.IgnoreNotFound(err)` when fetching objects that may not exist; use `apierrors.IsConflict(err)` → return the error (requeue) rather than retry loops inside Reconcile.
- The default per-item backoff starts at 5 ms and caps at 1000 s (~16 minutes) — rely on it rather than fighting it with custom requeue timing.
- A status write that fails after a successful external action must not repeat the external action non-idempotently on the next attempt.
- `generateName` collisions: with `RetryGenerateName` (beta-on in 1.31) the apiserver retries name generation itself; client-side retry loops on `AlreadyExists` for `generateName` creates are unnecessary on 1.31 (keep them only if the code also supports ≤ 1.30).

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

The controller-runtime v0.19 fake client rejects server-side apply (see client-go section). Use it for reconcilers that write children with `Create`/`Patch(MergeFrom)` and for status logic; for SSA-based reconcilers the following test **fails** on v0.19 with `deployments.apps "test" not found` (verified) — run it against envtest instead.

```go
// Works only if the reconciler does not use client.Apply (fake client v0.19 has no SSA)
func TestReconcile_CreatesDeployment(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = cachev1.AddToScheme(scheme)

	rc := &cachev1.RedisCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: cachev1.RedisClusterSpec{Tier: cachev1.TierPremium, Replicas: 3,
			Image: "registry.example.com/redis@sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rc).
		WithStatusSubresource(&cachev1.RedisCluster{}). // required when the reconciler writes status
		Build()
	r := &RedisClusterReconciler{Client: c, Scheme: scheme, Recorder: record.NewFakeRecorder(10)}

	key := types.NamespacedName{Name: "test", Namespace: "default"}
	for i := 0; i < 2; i++ { // idempotency: reconcile twice
		if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: key}); err != nil {
			t.Fatal(err)
		}
	}
	dep := &appsv1.Deployment{}
	if err := c.Get(context.Background(), key, dep); err != nil {
		t.Fatal(err)
	}
	if *dep.Spec.Replicas != 3 {
		t.Fatalf("replicas = %d", *dep.Spec.Replicas)
	}
}
```

Rules for fake-client tests on v0.19: register `WithStatusSubresource` for every type whose status is written; use `record.NewFakeRecorder(n)` and drain `rec.Events` to assert Events (verified: `Normal Available all 3 replicas ready`); do not test CEL, defaulting, admission, GC or SSA with fakes.

##### Integration Tests (envtest) — pin the 1.31 binaries

```go
env := &envtest.Environment{
	CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd")},
	ErrorIfCRDPathMissing: true,
	// BinaryAssetsDirectory or KUBEBUILDER_ASSETS from `setup-envtest use 1.31.x -p path`
}
cfg, err := env.Start()
```

- Pin envtest to the cluster minor you ship to (`setup-envtest use 1.31.x`). `setup-envtest` picks the **newest** Kubernetes version by default — in October 2026 that is 1.37, which accepts manifests, CEL functions and fields that a 1.31 cluster rejects or drops.
- envtest has no kube-controller-manager: no garbage collection, no Deployment/ReplicaSet controllers, no namespace deletion, no default ServiceAccount. Tests must fake child status (e.g. update `Deployment.status.readyReplicas`) and must not expect owner-reference cascades.
- envtest is the place to test CEL rules, webhooks (with `WebhookInstallOptions`) and SSA ownership.

##### e2e Tests (kind / k3s)

- Use a node image of the target minor (`kindest/node:v1.31.x`).
- Deploy the full operator image; test the complete lifecycle: Create → Reconcile → Update → Delete with finalizer.
- Use `kubectl wait --for=condition=Ready` assertions, `--for=jsonpath='{.status.observedGeneration}'=N` for generation checks, and (new in kubectl 1.31) `--for=create` to wait for an object to appear instead of polling loops.

##### Testing Rules

- Test idempotency: run `Reconcile` twice and assert the same outcome.
- Test the deletion path including finalizer removal.
- Test that status is updated correctly on both success and error paths.
- Test that invalid CRs are rejected by CEL rules/webhooks (`apierrors.IsInvalid`).
- Run manifests through `kubectl apply --dry-run=server` against a 1.31 apiserver in CI (envtest binaries or kind) — client-side schema validation does not catch dropped gated fields, Pod Security admission, or CEL compile/cost errors.

---

#### kubectl 1.31 and Manifest Tooling

- **[NEW in 1.31]** `kubectl exec POD CMD` without `--` is **removed**. Verified: kubectl 1.30 prints a deprecation notice and runs the command; kubectl 1.31 fails with `error: exec [POD] [COMMAND] is not supported anymore. Use exec [POD] -- [COMMAND] instead`. Fix scripts, Makefiles and CI jobs: `kubectl exec POD -- CMD`.
- **[NEW in 1.31]** `kubectl wait --for=create` (verified: kubectl 1.30 → `unrecognized condition: "create"`).
- **[NEW in 1.31]** `kubectl logs --all-pods` for workloads with several Pods (`kubectl logs deploy/api --all-pods`).
- **[NEW in 1.31 — beta, on]** `kubectl debug --custom=<file.json|yaml>` with partial container specs, and `--keep-labels`, `--keep-annotations`, `--keep-liveness`, `--keep-readiness`, `--keep-startup`, `--keep-init-containers` for `--copy-to` debugging.
- `kubectl port-forward` uses WebSockets by default (falls back to SPDY); proxies/load balancers in front of the apiserver must allow WebSocket upgrades. Set `KUBECTL_PORT_FORWARD_WEBSOCKETS=false` only as a workaround.
- `-o kyaml` and `.kuberc` preferences do not exist in kubectl 1.31.
- YAML coercion is a real manifest bug, not a style issue. Verified: `metadata: {name: n}` fails with `json: cannot unmarshal bool into Go struct field ObjectMeta.metadata.name of type string` because kubectl's YAML parser reads unquoted `n` as a boolean. Map **keys** are affected too: a hand-written CRD schema property `n:` (or `y:`, `on:`, `off:`) becomes the key `false`/`true`, and a CEL rule `self.n` then fails with `undefined field 'n'` (verified on 1.31). Quote values such as `y`, `n`, `yes`, `no`, `on`, `off`, `true`, `false`, version strings (`"1.10"`), and octal-looking numbers in labels, annotations, env values and ConfigMap data.
- `kubectl explain` shows alpha fields that the server will drop; use `--dry-run=server -o yaml` to prove a field survives.
- `--subresource` (beta) works for `status` and `scale` with `get`, `patch`, `edit` and `replace`; `resize` is rejected client-side by kubectl 1.31.
- Helm: declare `kubeVersion` in `Chart.yaml` when a chart depends on 1.31 behaviour, and gate optional fields with `semverCompare` on `.Capabilities.KubeVersion.Version`. `helm template` without `--kube-version` renders for a built-in default, not your cluster — verified: Helm v3.22.1 and v4.3.1 assume `v1.37.0`, Helm v4.0.5 assumes `v1.34.0`, so a template that branches on the version renders 1.34+/1.37 fields for a 1.31 target. Always pass `--kube-version 1.31.x` in CI renders. Verified: `kubeVersion: ">=1.31.0-0"` rejects `--kube-version 1.30.14`; Helm v3 strips vendor suffixes (`v1.31.14-eks-abc` → `v1.31.14`) in `.Capabilities.KubeVersion.Version`, Helm v4.3 keeps them.

```yaml
# templates/service.yaml — render trafficDistribution only for >= 1.31
spec:
  selector: {app: demo}
  ports: [{port: 80}]
  {{- if semverCompare ">=1.31.0-0" .Capabilities.KubeVersion.Version }}
  trafficDistribution: PreferClose
  {{- end }}
```

---

#### Removed / Changed in 1.31 — Migration Notes

| Change | Who is affected | Action |
|---|---|---|
| In-tree cloud providers removed | Clusters using `--cloud-provider=aws\|gce\|azure\|…` | Deploy the provider's cloud-controller-manager and set `--cloud-provider=external` before upgrading |
| `cephfs` / `rbd` volume plugins removed | Pods/PVs with `cephfs:`/`rbd:` sources | Migrate to ceph-csi and redeploy before upgrading |
| `CSIMigrationPortworx` on by default | `portworxVolume` users | Install the Portworx CSI driver first |
| AppArmor GA; annotations deprecated | Manifests using `container.apparmor.security.beta.kubernetes.io/*` | Use `securityContext.appArmorProfile`; nodes without AppArmor now reject such Pods |
| `kubectl exec POD CMD` removed | Scripts without `--` | `kubectl exec POD -- CMD` |
| `HonorPVReclaimPolicy` beta-on | Tooling that deletes PVs before PVCs or strips PV finalizers | Expect backing storage to be deleted for `Delete` PVs; never strip the new finalizers |
| `JobSuccessPolicy` beta-on | Controllers watching Job conditions | Accept the new `SuccessCriteriaMet` condition (also added on normal completion) |
| Terminal Job conditions delayed until Pods are terminal | Tools reacting to `Failed`/`Complete` | Expect the condition slightly later |
| `ServiceTrafficDistribution` beta-on | Services with `trafficDistribution` written for 1.30 | Field now takes effect (it was dropped on 1.30) |
| CEL compatibility env 1.30 | CRDs/policies | IP/CIDR library usable; name-format library still not |
| Strict decoding of encryption config | `--encryption-provider-config` with typos | Fix the file before upgrading |
| Removed flags (`--keep-terminated-pod-volumes`, kubelet `--iptables-masquerade-bit`/`--iptables-drop-bit`, `--proxy-port-range`, KCM HPA delays, `--volume-host-*`, `kubectl drain --delete-local-data`) | kubelet/kube-proxy/KCM configs, kubeadm `extraArgs`, scripts | Remove before upgrading |
| Removed GA feature gates (`APIPriorityAndFairness`, `ReadWriteOncePod`, `CSINodeExpandSecret`, `ConsistentHTTPGetHandlers`, `InTreePlugin*Unregister`, …) | `--feature-gates` in component configs | Components refuse to start with `unrecognized feature gate`; delete the entries |
| `PersistentVolumeLabel` admission plugin removed | `--enable-admission-plugins` lists | Remove it |
| Non-CSI volume-limit scheduler plugins deprecated | Custom scheduler configs | Use `NodeVolumeLimits` |
| `DisableNodeKubeProxyVersion` on in v1.31.0, off from v1.31.1 | Tools reading `nodeInfo.kubeProxyVersion` | Stop reading it |
| `OrderedNamespaceDeletion` gate (beta, off) added in v1.31.7 | Security-sensitive namespace teardown | Enable to delete Pods before other namespaced objects (on by default from 1.33) |
| Patch-level CVE fixes (v1.31.5, v1.31.6, v1.31.12) | Pinned 1.31 versions | Run ≥ v1.31.12 if still on 1.31; plan the upgrade off 1.31 (EOL) |
| client-go v0.31 `fake.NewClientset` | Tests using `NewSimpleClientset` | Migrate tests to `NewClientset` for SSA support |
| controller-runtime v0.19 deprecates `admission.Validator`/`Defaulter` | Kubebuilder v3-style webhooks (`func (r *T) ValidateCreate() ...`) | Move to `CustomValidator`/`CustomDefaulter` (required for v0.20) |
| v1.31.14 (Go 1.24) drops SHA-1 cert support | Webhooks/aggregated APIs with SHA-1 certificates | Reissue certificates with SHA-256+ |

---

#### Production / Review Checklist

##### Version & API availability

- [ ] Manifests use only API versions served by a default 1.31 apiserver (no `storage.k8s.io/v1beta1` VAC, `networking.k8s.io/v1beta1`, `resource.k8s.io/v1alpha3`, MutatingAdmissionPolicy)
- [ ] No 1.32+ fields or behaviour assumed (`PreferSameZone`, `preStop.sleep.seconds: 0`, Job `managedBy`, pod-level `resources`, `pods/resize`, CEL `format.dns1123Label()`/`isSemver()`, …)
- [ ] No reliance on silently-dropped alpha/off fields (`image` volumes, `hostUsers: false`, `supplementalGroupsPolicy`, `resizePolicy`, `procMount: Unmasked`, Job `managedBy`, StatefulSet `rollingUpdate.maxUnavailable`) unless the cluster config enables the gate
- [ ] `FlowSchema`/`PriorityLevelConfiguration` on `flowcontrol.apiserver.k8s.io/v1`, not `v1beta3`
- [ ] `go.mod`: all `k8s.io/*` on the same v0.31.x patch, controller-runtime v0.19.x, built with a supported Go toolchain
- [ ] envtest/kind/Helm renders pinned to 1.31, not "latest"
- [ ] Cluster/infra code: ≥ v1.31.12 if 1.31 is unavoidable; upgrade plan exists (1.31 is EOL since 2025-11-11)

##### CRD Design

- [ ] Kind: CamelCase singular; Resource: lowercase plural; group is a DNS subdomain you own (not `*.k8s.io`)
- [ ] `// +kubebuilder:subresource:status`; `status.conditions` (`listType=map`, key `type`) + `status.observedGeneration`
- [ ] All enum fields use string type aliases (not `bool`); no floats in `spec`
- [ ] Every string/array/map reachable from CEL has `maxLength`/`maxItems`/`maxProperties`
- [ ] Numeric fields have `minimum`/`maximum` (`format: int32` is not a range check)
- [ ] CEL uses only functions available on 1.31 (quantity, url, strings/lists/sets, optionals, IP/CIDR); `messageExpression` uses `'%d'.format([...])`, not `string(int)`; transition rules handle removal of the field
- [ ] Rules use `fieldPath` and `reason`; `optionalOldSelf` only with `oldSelf.hasValue()`
- [ ] At least one printer column with state info; `selectableFields` only if ≤ 1.30 clusters are not supported
- [ ] `omitempty` + `// +optional` on all optional fields

##### Controller

- [ ] Fetch → `IgnoreNotFound` → deletion/finalizer → converge → status
- [ ] Children applied with SSA (apply configuration → unstructured, or clientset `Apply`) and a constant field owner; never typed structs with `client.Apply`; no `Update` from cached objects
- [ ] Status written through `Status().Patch`; `observedGeneration` set; conditions via `meta.SetStatusCondition`
- [ ] Transient errors returned; permanent errors → condition + Warning Event + `nil`/`TerminalError`; no error + `RequeueAfter`
- [ ] Event recorder from `GetEventRecorderFor` with core `events` RBAC; Events only on transitions; no user text as format string
- [ ] Webhooks use `CustomValidator`/`CustomDefaulter` (not the deprecated `admission.Validator` on the type); type assertions checked; CEL preferred where possible
- [ ] Watches on shared objects use field indexes and restricted caches
- [ ] Metrics served securely (`SecureServing` + `filters.WithAuthenticationAndAuthorization`); leader election enabled

##### Workloads

- [ ] Pod Security `restricted` passes (runAsNonRoot, seccomp `RuntimeDefault`, drop `ALL`, no privilege escalation); `enforce-version` pinned
- [ ] AppArmor via `appArmorProfile` fields, not annotations
- [ ] Images pinned by digest; private images on shared nodes protected by `AlwaysPullImages`
- [ ] Memory requests/limits set; no in-place resize assumed
- [ ] Probes: startup + readiness + liveness with sane thresholds; `preStop.sleep` (≥ 1 s) instead of shell sleeps
- [ ] Sidecars as `initContainers` with `restartPolicy: Always`
- [ ] Jobs: `podFailurePolicy` ignores `DisruptionTarget`; `successPolicy` only for Indexed Jobs; no `managedBy`
- [ ] PDBs set `unhealthyPodEvictionPolicy: AlwaysAllow` unless there is a reason not to
- [ ] Services use `trafficDistribution: PreferClose` (not `PreferSameZone`); no `externalIPs`
- [ ] HPA-managed Deployments do not set `spec.replicas` in applied manifests
- [ ] PVs whose data must survive use `Retain`; nothing strips PV finalizers
- [ ] Unquoted YAML scalars that coerce (`n`, `yes`, `on`, `1.10`) are quoted

##### Security / RBAC

- [ ] No wildcard verbs/resources; namespace-scoped roles where possible
- [ ] No `get` on `pods/exec`/`attach`/`portforward` in "read-only" roles (WebSocket exec is authorized with `get` on 1.31)
- [ ] Service account tokens not automounted unless needed
- [ ] Secrets referenced by name — never inlined in CRD spec, logs, Events or conditions
- [ ] Default-deny NetworkPolicy plus explicit allows (including DNS)
- [ ] Admission webhooks exclude their own namespace and `kube-system`; `sideEffects: None`; timeouts ≤ 10 s; certificates not SHA-1

##### Finalizers & Versioning

- [ ] Finalizer name is fully qualified (`domain.com/name`), added before external resources are created, removed only after successful idempotent cleanup
- [ ] New CRD fields optional; no fields removed or renamed in served versions
- [ ] `deprecationWarning` set on deprecated CRD versions; conversion webhook when schemas diverge; valid `caBundle`
- [ ] `storage: true` on exactly one version; stored objects migrated and `status.storedVersions` cleaned before dropping a version

---

[^1]: Kubernetes v1.31: Elli — release announcement, 2024-08-13. https://kubernetes.io/blog/2024/08/13/kubernetes-v1-31-release/
[^2]: Kubernetes release data (data/releases/eol.yaml: 1.31 end of life 2025-11-11, final patch 1.31.14; data/releases/schedule.yaml for 1.34–1.37). https://kubernetes.io/releases/ , https://github.com/kubernetes/website/blob/main/data/releases/eol.yaml
[^3]: CHANGELOG-1.31 (v1.31.0 urgent upgrade notes, API changes, features, deprecations, bug fixes; patch notes v1.31.1–v1.31.14 including CVE-2024-9042, CVE-2025-0426, CVE-2025-5187). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.31.md
[^4]: Go module proxy data for k8s.io/client-go v0.31.x (v0.31.0 2024-08-13, v0.31.14 2025-11-12, `go 1.22.0`), sigs.k8s.io/controller-runtime v0.19.x (v0.19.7 2025-03-08, requires k8s.io/* v0.31.0), sigs.k8s.io/controller-tools v0.16.x. https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.7 , https://pkg.go.dev/k8s.io/client-go@v0.31.14
[^5]: Kubebuilder scaffold versions (pkg/plugins/golang/v4/scaffolds/init.go at v4.2.0, v4.3.x, v4.4.0). https://github.com/kubernetes-sigs/kubebuilder/releases
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
[^7]: Deprecated API Migration Guide (flowcontrol.apiserver.k8s.io/v1beta3 removed in v1.32). https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^8]: Kubernetes v1.32 (2024-12-11), v1.33 (2025-04-23), v1.34 (2025-08-27), v1.35, v1.36 and v1.37 release announcements and CHANGELOGs. https://kubernetes.io/blog/2024/12/11/kubernetes-v1-32-release/ , https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/ , https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG
[^9]: CustomResourceDefinition validation rules (CEL), validation ratcheting, selectable fields; Kubernetes CEL libraries. https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/ , https://kubernetes.io/docs/reference/using-api/cel/
[^10]: Validating Admission Policy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
[^11]: controller-runtime v0.19 package documentation (`webhook/admission`: Validator/Defaulter deprecated; `builder.WebhookBuilder`; `cluster.Cluster.GetEventRecorderFor`; `metrics/filters`). https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.7
[^12]: KEP-4006: Transition from SPDY to WebSockets; Kubernetes 1.35 CHANGELOG (`AuthorizePodWebsocketUpgradeCreatePermission`). https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/4006-transition-spdy-to-websockets , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md
[^13]: Go 1.25 release notes — container-aware GOMAXPROCS. https://go.dev/doc/go1.25#container-aware-gomaxprocs
[^14]: Volumes — read-only mounts and `recursiveReadOnly`; Kubernetes 1.30 blog "Read-only volume mounts can be finally literally read-only". https://kubernetes.io/docs/concepts/storage/volumes/#read-only-mounts , https://kubernetes.io/blog/2024/04/23/recursive-read-only-mounts/
