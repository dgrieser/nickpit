### Kubernetes 1.29 — Manifests, Operators & Controllers Review Guide

> **Version**: Kubernetes v1.29.0 "Mandala", released **2023-12-13**.[^1] Final patch release: **v1.29.15**, published **2025-03-12** — after the end-of-life date, only to rebuild 1.29 with Go 1.23 (`https://dl.k8s.io/release/stable-1.29.txt` reports `v1.29.15`; the release data on kubernetes.io still records v1.29.14, 2025-02-13, as the final patch).[^2][^3]
> **Support**: 1.29 is **long out of support**. Its end-of-life date was **2025-02-28**; no further patches (including security fixes) are published. As of October 2026 the supported minors are **1.34** (end of life 2026-10-27), **1.35**, **1.36** and **1.37** (newest: 1.37, released 2026-08-26, latest patch v1.37.1). Managed providers may still sell extended support for 1.29 — that does not change upstream status.[^2]
> **Matching libraries**: `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`, `k8s.io/apiextensions-apiserver` **v0.29.x** (final v0.29.15, 2025-03-12, `go 1.21` directive), **controller-runtime v0.17.x** (final v0.17.6, 2024-08-12; its `go.mod` requires `k8s.io/* v0.29.2` and `go 1.21`), **controller-tools v0.14.0** (the only v0.14 release, 2024-01-08). Kubebuilder **v3.14.0–v3.15.x** scaffolds controller-runtime v0.17 (v3.14.0: v0.17.0 + controller-tools v0.14.0 + envtest 1.29.0); Kubebuilder v4.0.0 already moves to controller-runtime v0.18 / Kubernetes 1.30 libraries.[^4][^5]
> **Scope**: Kubernetes manifests (plain YAML, Kustomize output, Helm-rendered YAML), CustomResourceDefinitions with CEL validation, admission configuration, and Go operators/controllers built with controller-runtime, client-go or Kubebuilder that target a 1.29 cluster.
> **Note**: This document is self-contained. House rules for operator design, CRDs, status, finalizers, RBAC, webhooks, observability and testing are carried forward; sections marked **[NEW in 1.29]** cover behaviour specific to this release. Every manifest, CRD and policy example was applied (or `--dry-run=server`-applied) against a real v1.29.5 kube-apiserver (the 1.29 envtest build); upper-boundary claims were re-checked on a v1.30.3 apiserver. Lower-boundary claims ("new in 1.29") were checked against the v1.28.0/v1.29.0 feature-gate sources and the CHANGELOG and executed on a v1.28.0 kube-apiserver (the controller-tools `envtest-v1.28.0` build). Go snippets compile against k8s.io/* v0.29.15 + controller-runtime v0.17.6, and the reconciler was exercised with envtest on the v1.29.5 binaries. Pod *runtime* behaviour (kubelet, scheduler, controllers) is sourced from the release notes, KEPs and documentation, not executed.

---

#### Table of Contents

1. [Version-aware review](#version-aware-review)
2. [What's New in Kubernetes 1.29](#whats-new-in-kubernetes-129)
3. [API versions and feature gates on a 1.29 cluster](#api-versions-and-feature-gates-on-a-129-cluster)
4. [Do not use or recommend (1.30+ features)](#do-not-use-or-recommend-130-features)
5. [Deprecated and removed — what 1.29 code must not rely on](#deprecated-and-removed--what-129-code-must-not-rely-on)
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
20. [Admission: Webhooks and (opt-in) ValidatingAdmissionPolicy](#admission-webhooks-and-opt-in-validatingadmissionpolicy)
21. [RBAC & Least Privilege](#rbac--least-privilege)
22. [Workload Manifests: Pods](#workload-manifests-pods)
23. [Workload Manifests: Controllers (Deployment, StatefulSet, Job, PDB, HPA)](#workload-manifests-controllers-deployment-statefulset-job-pdb-hpa)
24. [Networking: Services & NetworkPolicy](#networking-services--networkpolicy)
25. [Storage](#storage)
26. [Security Hardening](#security-hardening)
27. [API Priority and Fairness (flowcontrol v1)](#api-priority-and-fairness-flowcontrol-v1)
28. [client-go & apimachinery in v0.29](#client-go--apimachinery-in-v029)
29. [Observability](#observability)
30. [Error Handling & Requeue Strategy](#error-handling--requeue-strategy)
31. [Testing](#testing)
32. [kubectl 1.29 and Manifest Tooling](#kubectl-129-and-manifest-tooling)
33. [Removed / Changed in 1.29 — Migration Notes](#removed--changed-in-129--migration-notes)
34. [Production / Review Checklist](#production--review-checklist)

---

#### Version-aware review

The target is a **1.29 control plane** and **k8s.io/\* v0.29.x** client libraries. Before claiming that a field, API version or library function is "not available", check it against the pinned version:

- For manifests: the version comes from the cluster (`kubectl version`), the Helm chart's `kubeVersion`, CI's `kind`/`envtest` image, or a Kustomize/Helm render target. A field that exists in the OpenAPI schema is **not** proof that it works. `kubectl explain` on 1.29 lists alpha-gated fields such as `pod.spec.containers.lifecycle.preStop.sleep`, `pod.spec.hostUsers`, `pod.spec.containers.resizePolicy` and `persistentvolumeclaim.spec.volumeAttributesClassName`, but a default 1.29 apiserver **drops** them on create (verified with `kubectl apply --dry-run=server -o yaml`). Only a server-side dry run against a 1.29 apiserver with the cluster's real feature gates proves a field is honoured.
- Three different failure modes exist, and all were observed on v1.29.5:
  1. A field the 1.29 schema does not know at all is **rejected**: `strict decoding error: unknown field "spec.securityContext.appArmorProfile"` (kubectl ≥ 1.27 requests strict server-side field validation). This is what happens to 1.30+ fields such as `appArmorProfile`, `trafficDistribution`, Job `successPolicy`/`managedBy`, `recursiveReadOnly` and CRD `selectableFields`.
  2. A field the schema knows but whose feature gate is off is **accepted and dropped without any warning** (`hostUsers`, `resizePolicy`, `procMount: Unmasked` → `Default`, pod-affinity `matchLabelKeys`, StatefulSet `rollingUpdate.maxUnavailable`, PVC `volumeAttributesClassName`, Service `status.loadBalancer.ingress[].ipMode`, CRD rule `optionalOldSelf`).
  3. A dropped field can make the **remaining object invalid**: `lifecycle.preStop.sleep` is dropped (alpha in 1.29), the now-empty `preStop` handler fails validation, and the whole Pod/Deployment is rejected with `spec.containers[0].lifecycle.preStop: Required value: must specify a handler type`. Because a handler holds exactly one action, every `preStop.sleep`/`postStart.sleep` is rejected on a default 1.29 cluster. (A 1.28 apiserver does not know the field at all and rejects it as `unknown field "spec.containers[0].lifecycle.preStop.sleep"` — verified.)
- For Go code: the version comes from `go.mod` (`k8s.io/api v0.29.x`, `sigs.k8s.io/controller-runtime v0.17.x`). Verify a symbol with `go doc sigs.k8s.io/controller-runtime/pkg/cache Options` (or the relevant package) in the module before flagging it.
- Say "not available in 1.29" only when the pinned version proves it (unknown field, `Unsupported value`, dropped field, `no matches for kind`, missing package/function). Say "available but off by default" for alpha features and for beta **APIs** (since 1.24, new beta API versions are not served unless `--runtime-config` enables them).[^6] On 1.29 this matters most for **ValidatingAdmissionPolicy**, which is beta but **not served by default** (see below).
- A cluster may enable feature gates. If the repository contains cluster configuration (kubeadm `ClusterConfiguration`, kind config, managed-cluster settings) that enables a gate or an API, review against that configuration, not against the defaults below.
- **Patch level matters**: `optionalOldSelf` was accidentally settable on v1.29.0 and is gated from v1.29.1; watch requests without `resourceVersion` are served from the watch cache again from v1.29.4 (`WatchFromStorageWithoutResourceVersion`, beta/off); CVE fixes landed in v1.29.4, v1.29.7 and v1.29.13 (see "Security Hardening").[^3]
- **The release is end-of-life** (since 2025-02-28, more than 18 months ago). Treat "upgrade off 1.29" as a legitimate review note for infrastructure code (cluster versions, kind/kubeadm pins); do not flag every application manifest for it.

---

#### What's New in Kubernetes 1.29

Kubernetes 1.29 contains 49 enhancements: 11 stable, 19 beta and 19 alpha.[^1] The tables list the changes that a manifest author, CRD author or controller author can observe. "Default" is the state on an unmodified v1.29 control plane; gates were read from `kube-apiserver --help` of the v1.29.5 and v1.30.3 binaries and from `pkg/features/kube_features.go` (plus the apiserver and apiextensions gate files) at v1.28.0 and v1.29.0, and API behaviour was tested on the v1.29.5 and v1.30.3 apiservers.

##### Graduated to stable (GA) in 1.29

| Feature | What it means for manifests / controllers | KEP |
|---|---|---|
| **CRD validation rules** (`CustomResourceValidationExpressions`, locked on) | `x-kubernetes-validations` with CEL `rule`, `message`, `messageExpression`, `reason`, `fieldPath`, transition rules (`oldSelf`) — first choice for CRD invariants (beta and on by default since 1.25) | 2876 |
| **`ReadWriteOncePod` access mode** (`ReadWriteOncePod`, locked on) | A PVC with `accessModes: [ReadWriteOncePod]` can be used by exactly one Pod in the whole cluster (CSI volumes only) | 2485 |
| **API Priority and Fairness** (`APIPriorityAndFairness`, locked on) | `flowcontrol.apiserver.k8s.io/v1` FlowSchema / PriorityLevelConfiguration; `v1beta3` deprecated, `v1beta2` removed | 1040 |
| **KMS v2** (`KMSv2`, `KMSv2KDF` locked on) | Encryption at rest with KMS v2; **KMS v1 is disabled by default** (`KMSv1=false`, deprecated) | 3299 |
| **Paginated LIST** (`APIListChunking`, `RemainingItemCount` locked on) | `limit`/`continue` and `remainingItemCount` are GA | 365 |
| **Track ready Pods in Job status** (`JobReadyPods`, locked on) | `Job.status.ready` is always populated | 2879 |
| **NodePort static/dynamic sub-ranges** (`ServiceNodePortStaticSubrange`, locked on) | Dynamic NodePort allocation prefers the upper band so explicitly chosen low NodePorts collide less | 3668 |
| **CSI node-expand secret** (`CSINodeExpandSecret`, locked on) | `StorageClass` parameter `csi.storage.k8s.io/node-expand-secret-name`/`-namespace` | 3107 |
| **Kubelet resource metrics endpoint**, **component SLIs** (`/metrics/slis`), stable load-balancer node set in the cloud service controller | Node / control-plane operations | 727, 3466, 3458 |

##### Beta in 1.29 (with default)

| Feature / gate | Default in 1.29 | Notes |
|---|---|---|
| **Native sidecar containers** (`SidecarContainers`) | **on** (alpha/off in 1.28) | `initContainers[].restartPolicy: Always`; verified kept on 1.29 |
| Job **`backoffLimitPerIndex`** / `maxFailedIndexes` (`JobBackoffLimitPerIndex`) | **on** (alpha/off in 1.28) | Indexed Jobs only; `FailIndex` action in `podFailurePolicy` |
| Job **`podReplacementPolicy`** (`JobPodReplacementPolicy`) | **on** (alpha/off in 1.28) | `TerminatingOrFailed` (default) or `Failed`; adds `status.terminating` |
| **Legacy service-account token clean-up** (`LegacyServiceAccountTokenCleanUp`) | **on** (alpha/off in 1.28) | Unused auto-generated SA token Secrets get `kubernetes.io/legacy-token-invalid-since` after one year (then rejected) and are deleted one year later |
| **Pod `status.hostIPs`** (`PodHostIPs`) | **on** (alpha/off in 1.28) | Dual-stack host IPs; Downward API `status.hostIPs` |
| `PodReadyToStartContainers` Pod condition (`PodReadyToStartContainersCondition`) | **on** | Sandbox + network ready |
| `PersistentVolume.status.lastPhaseTransitionTime` (`PersistentVolumeLastPhaseTransitionTime`) | **on** | Set on the next phase change |
| **In-tree cloud providers disabled** (`DisableCloudProviders`, `DisableKubeletCloudCredentialProviders`) | **on** (alpha/off in 1.28) | `--cloud-provider=aws\|gce\|azure\|vsphere\|openstack` no longer works unless the gates are set back to `false` — run an external cloud-controller-manager |
| Taint-based eviction split from the node lifecycle controller (`SeparateTaintEvictionController`) | on | Controller-manager internals |
| Dual-stack `--node-ip` for external cloud providers (`CloudDualStackNodeIPs`), CDI devices in device plugins (`DevicePluginCDIDevices`) | on | Node configuration |
| HTTP/2 DoS mitigation for unauthenticated clients (`UnauthenticatedHTTP2DOSMitigation`) | on | CVE-2023-44487 / CVE-2023-39325 |
| `SchedulerQueueingHints` | **beta, off** (it was beta/on in 1.28.0) | Scheduler internals |
| `ZeroLimitedNominalConcurrencyShares` | beta, off | Explicit `nominalConcurrencyShares: 0` in flowcontrol `v1` is still rejected on 1.29 (verified) |
| **ValidatingAdmissionPolicy** (`admissionregistration.k8s.io/v1beta1`) | **beta, off — API not served** | Needs `--feature-gates=ValidatingAdmissionPolicy=true --runtime-config=admissionregistration.k8s.io/v1beta1=true` (verified); GA as `v1` in 1.30 |

Beta-and-on features from earlier releases that 1.29 code can rely on: webhook **`matchConditions`** (`AdmissionWebhookMatchConditions`, beta-on since 1.28), Pod **`schedulingGates`** (`PodSchedulingReadiness`, beta-on since 1.27), topology-spread **`minDomains`** (beta-on since 1.27) and **`matchLabelKeys`** (beta-on since 1.27), **Job `podFailurePolicy`** and the **`DisruptionTarget`** condition (beta-on since 1.26), **PDB `unhealthyPodEvictionPolicy`** (beta-on since 1.27), **StatefulSet `ordinals.start`** (beta-on since 1.27), StatefulSet **`persistentVolumeClaimRetentionPolicy`** (beta-on since 1.27), **elastic Indexed Jobs** (beta-on since 1.27), HPA **`ContainerResource`** metrics (beta-on since 1.27), the **`apps.kubernetes.io/pod-index`** label (`PodIndexLabel`, beta-on since 1.28), **AppArmor via annotations** (beta since 1.4), `controller.kubernetes.io/pod-deletion-cost` (beta since 1.22) and randomized ReplicaSet scale-down (`LogarithmicScaleDown`, beta since 1.22). The API fields among them (`matchConditions`, `schedulingGates`, `minDomains`, topology-spread `matchLabelKeys`, `podFailurePolicy`, `unhealthyPodEvictionPolicy`, `ordinals.start`, `persistentVolumeClaimRetentionPolicy`) were verified to be kept by the 1.29 apiserver.

Boundary checks on a v1.28.0 apiserver (verified): `initContainers[].restartPolicy: Always`, Job `backoffLimitPerIndex`/`maxFailedIndexes`/`podReplacementPolicy` and `hostUsers` are silently dropped (gates off); pod-affinity `matchLabelKeys`, `preStop.sleep` and PVC `volumeAttributesClassName` are unknown fields; `CRON_TZ=` in a CronJob schedule only produces a warning; load-balancer status on a `ClusterIP` Service is accepted; `flowcontrol.apiserver.k8s.io/v1` is not served (`v1beta2` and `v1beta3` are); CRD rules using `quantity()`, CEL optionals or int/double comparisons fail to compile. Manifests and CRDs written for 1.29 therefore do not always work on 1.28 — relevant for charts that declare `kubeVersion: ">=1.28"`.

##### Alpha in 1.29 (off by default — treat as unavailable)

New alpha features in 1.29: **`lifecycle.preStop.sleep` / `postStart.sleep`** (`PodLifecycleSleepAction`); **`matchLabelKeys` / `mismatchLabelKeys` in pod (anti-)affinity** (`MatchLabelKeysInPodAffinity`); **kube-proxy nftables mode** (`NFTablesProxyMode`); **Service `status.loadBalancer.ingress[].ipMode`** (`LoadBalancerIPMode`); **VolumeAttributesClass** (`storage.k8s.io/v1alpha1`, `volumeAttributesClassName`, `VolumeAttributesClass`); **ServiceCIDR** (`networking.k8s.io/v1alpha1`, part of `MultiCIDRServiceAllocator`); ClusterTrustBundle projection into Pods (`ClusterTrustBundleProjection`); structured authentication / authorization configuration files (`StructuredAuthenticationConfiguration`, `StructuredAuthorizationConfiguration`); service-account token `jti`, node binding and node info (`ServiceAccountTokenJTI`, `ServiceAccountTokenNodeBinding`, `ServiceAccountTokenNodeBindingValidation`, `ServiceAccountTokenPodNodeInfo`); `DisableNodeKubeProxyVersion`; image GC by age (`ImageMaximumGCAge`); split image filesystem (`KubeletSeparateDiskGC`); image pull per RuntimeClass (`RuntimeClassInImageCriApi`); WebSocket stream translation (`TranslateStreamCloseWebsocketRequests`); `UserNamespacesPodSecurityStandards`; in-place Pod resize for Windows (still `InPlacePodVerticalScaling`, alpha since 1.27).[^1][^3]

Alpha from earlier releases and still off on 1.29: **user namespaces** (`hostUsers: false`, `UserNamespacesSupport`), **in-place Pod resize** (`resizePolicy`), `ProcMountType`, **CRD validation ratcheting** and `optionalOldSelf` (`CRDValidationRatcheting`), consistent reads from the watch cache (`ConsistentListFromCache`), `WatchList`, **DRA** (`resource.k8s.io/v1alpha2`, `DynamicResourceAllocation`), `ClusterTrustBundle` (`certificates.k8s.io/v1alpha1`), StatefulSet `maxUnavailable` (`MaxUnavailableStatefulSet`), `HonorPVReclaimPolicy`, `RecoverVolumeExpansionFailure`, `HPAScaleToZero`, `NodeLogQuery`, `ContainerCheckpoint`, `StorageVersionAPI`.

##### Other 1.29 changes that affect reviews

- **`flowcontrol.apiserver.k8s.io/v1beta2` is no longer served** (verified: `no matches for kind "PriorityLevelConfiguration" in version "flowcontrol.apiserver.k8s.io/v1beta2"`). Use `flowcontrol.apiserver.k8s.io/v1`; `v1beta3` still works but warns (`deprecated in v1.29+, unavailable in v1.32+`).[^7]
- **CronJob `TZ=`/`CRON_TZ=` in `spec.schedule` is rejected for new objects** (verified: `cannot use TZ or CRON_TZ in schedule, use timeZone field instead`). Use `spec.timeZone` (GA since 1.27).[^3]
- **Service `status.loadBalancer` may only be set when `spec.type: LoadBalancer`** (verified: `status.loadBalancer.ingress: Forbidden: may only be used when spec.type is 'LoadBalancer'`). Controllers that wrote load-balancer status on `ClusterIP`/`NodePort` Services must stop; the deprecated `AllowServiceLBStatusOnNonLB` gate restores the old behaviour.[^3]
- **`Node.status.nodeInfo.kubeProxyVersion` is deprecated** (KEP-4004): the kubelet never knew the real kube-proxy version. Do not read it in controllers.[^1]
- **The networking alpha API `ClusterCIDR` is removed** (verified: with `networking.k8s.io/v1alpha1` enabled, a 1.29 apiserver serves only `IPAddress` and `ServiceCIDR`).[^3]
- **CEL in CRDs**: the CEL environment of 1.29 adds the **Kubernetes quantity library** (`quantity()`, `isQuantity()`), **CEL optional types** (`self.?field.orValue(...)`) and **cross-type numeric comparisons** for *new* expressions; the cost estimator now uses the longest `enum` value instead of treating enums as unbounded strings, and per-expression cost checks are skipped for unchanged CRD versions on update.[^3][^8]
- **Validation rules gained `optionalOldSelf`** (alpha, gated by `CRDValidationRatcheting` from v1.29.1). On a default 1.29 cluster the field is silently dropped (verified), so a rule written as `oldSelf.hasValue()` fails to compile.[^3]
- **Pod Security Standards v1.29**: the baseline and restricted policies now allow the `net.ipv4.tcp_keepalive_time`, `net.ipv4.tcp_fin_timeout`, `net.ipv4.tcp_keepalive_intvl` and `net.ipv4.tcp_keepalive_probes` sysctls (verified: `baseline:v1.28` forbids `net.ipv4.tcp_keepalive_time`, `baseline:v1.29` allows it).[^3]
- **Sidecar termination is serialized**: each sidecar receives `SIGTERM` only after all main containers and later-started sidecars have terminated; restartable init containers count towards the HPA's resource calculations.[^3]
- **kube-proxy**: new `--init-only` mode for a privileged init container; `--bind-address` is documented as *not* a bind address (it chooses the node IP); new conntrack options (`nf_conntrack_tcp_be_liberal`, UDP timeouts).[^3]
- **kube-scheduler**: the `kubescheduler.config.k8s.io/v1beta3` configuration API and the `SelectorSpread` plugin are removed — use `kubescheduler.config.k8s.io/v1` and `PodTopologySpread`.[^3]
- **kubeadm**: `admin.conf` is now bound to the `kubeadm:cluster-admins` group (RBAC `cluster-admin`); the break-glass `system:masters` credential moves to a separate `super-admin.conf`. kubeadm also tolerates kubelets three minors older than the control plane.[^3]
- **Go API change**: `PersistentVolumeClaimSpec.Resources` is now of type `corev1.VolumeResourceRequirements` (was `ResourceRequirements`) in `k8s.io/api` v0.29 — Go code that builds PVCs must change when it moves to v0.29 (verified: the old type no longer compiles against v0.29.15).[^3]
- **Client-side apply** in kubectl uses OpenAPI v3; `kubectl delete --interactive` (alpha) is visible; `kubectl wait --for=jsonpath` accepts simple filter expressions.[^3]

---

#### API versions and feature gates on a 1.29 cluster

##### API group versions served by a default v1.29 apiserver

Verified with `kubectl api-versions` on stock v1.29.5 and v1.30.3 apiservers:

```text
admissionregistration.k8s.io/v1   apiextensions.k8s.io/v1   apiregistration.k8s.io/v1
apps/v1   authentication.k8s.io/v1   authorization.k8s.io/v1   autoscaling/v1   autoscaling/v2
batch/v1   certificates.k8s.io/v1   coordination.k8s.io/v1   discovery.k8s.io/v1
events.k8s.io/v1   flowcontrol.apiserver.k8s.io/v1   flowcontrol.apiserver.k8s.io/v1beta3
networking.k8s.io/v1   node.k8s.io/v1   policy/v1   rbac.authorization.k8s.io/v1
scheduling.k8s.io/v1   storage.k8s.io/v1   v1
```

1.30 serves the identical list — but on 1.30 `admissionregistration.k8s.io/v1` additionally contains `ValidatingAdmissionPolicy`/`ValidatingAdmissionPolicyBinding` (verified). On 1.29, `admissionregistration.k8s.io/v1` contains only the webhook configurations. `flowcontrol.apiserver.k8s.io/v1beta2` is gone (removed in 1.29); `v1beta3` follows in 1.32.[^7]

On a default 1.29 cluster these are **not** served and manifests using them fail with `no matches for kind`:

| API | Status in 1.29 | How to enable (verified on v1.29.5) |
|---|---|---|
| `admissionregistration.k8s.io/v1beta1` ValidatingAdmissionPolicy, ValidatingAdmissionPolicyBinding | beta, **off** | `--feature-gates=ValidatingAdmissionPolicy=true --runtime-config=admissionregistration.k8s.io/v1beta1=true` |
| `admissionregistration.k8s.io/v1` ValidatingAdmissionPolicy | **does not exist** | 1.30 (GA) |
| `admissionregistration.k8s.io/v1alpha1` ValidatingAdmissionPolicy | alpha, off | gate + runtime-config — never use it for new manifests |
| `networking.k8s.io/v1alpha1` ServiceCIDR, IPAddress | alpha, off | `MultiCIDRServiceAllocator=true` + runtime-config |
| `resource.k8s.io/v1alpha2` ResourceClass, ResourceClaim, ResourceClaimTemplate, PodSchedulingContext | alpha, off | `DynamicResourceAllocation=true` + runtime-config; this "classic" DRA API was replaced by the incompatible structured-parameters API in 1.31 |
| `certificates.k8s.io/v1alpha1` ClusterTrustBundle | alpha, off | `ClusterTrustBundle=true` + runtime-config |
| `storage.k8s.io/v1alpha1` VolumeAttributesClass | alpha, off | `VolumeAttributesClass=true` + runtime-config |
| MutatingAdmissionPolicy, StorageVersionMigration, LeaseCandidate | **do not exist** | 1.32 / 1.30 / 1.31 (alpha) |

##### Selected feature gates (kube-apiserver v1.29.5 defaults)

```text
SidecarContainers=true (BETA)                 JobBackoffLimitPerIndex=true (BETA)
JobPodReplacementPolicy=true (BETA)           JobPodFailurePolicy=true (BETA)
PodDisruptionConditions=true (BETA)           ElasticIndexedJob=true (BETA)
PDBUnhealthyPodEvictionPolicy=true (BETA)     StatefulSetStartOrdinal=true (BETA)
StatefulSetAutoDeletePVC=true (BETA)          PodIndexLabel=true (BETA)
PodSchedulingReadiness=true (BETA)            MinDomainsInPodTopologySpread=true (BETA)
MatchLabelKeysInPodTopologySpread=true (BETA) NodeInclusionPolicyInPodTopologySpread=true (BETA)
PodHostIPs=true (BETA)                        PodReadyToStartContainersCondition=true (BETA)
HPAContainerMetrics=true (BETA)               AppArmor=true (BETA)
AdmissionWebhookMatchConditions=true (BETA)   AggregatedDiscoveryEndpoint=true (BETA)
LegacyServiceAccountTokenCleanUp=true (BETA)  PersistentVolumeLastPhaseTransitionTime=true (BETA)
DisableCloudProviders=true (BETA)             DisableKubeletCloudCredentialProviders=true (BETA)
LogarithmicScaleDown=true (BETA)              UnauthenticatedHTTP2DOSMitigation=true (BETA)
ValidatingAdmissionPolicy=false (BETA)        SchedulerQueueingHints=false (BETA)
ZeroLimitedNominalConcurrencyShares=false (BETA)  NodeSwap=false (BETA)
WatchFromStorageWithoutResourceVersion=false (BETA)   # added in v1.29.4
CRDValidationRatcheting=false (ALPHA)         ConsistentListFromCache=false (ALPHA)
PodLifecycleSleepAction=false (ALPHA)         MatchLabelKeysInPodAffinity=false (ALPHA)
UserNamespacesSupport=false (ALPHA)           ProcMountType=false (ALPHA)
InPlacePodVerticalScaling=false (ALPHA)       MaxUnavailableStatefulSet=false (ALPHA)
LoadBalancerIPMode=false (ALPHA)              VolumeAttributesClass=false (ALPHA)
MultiCIDRServiceAllocator=false (ALPHA)       DynamicResourceAllocation=false (ALPHA)
ClusterTrustBundle=false (ALPHA)              HonorPVReclaimPolicy=false (ALPHA)
NFTablesProxyMode=false (ALPHA)               ServiceAccountTokenNodeBinding=false (ALPHA)
StructuredAuthenticationConfiguration=false (ALPHA)  StructuredAuthorizationConfiguration=false (ALPHA)
WatchList=false (ALPHA)                       HPAScaleToZero=false (ALPHA)
```

GA gates (for example `CustomResourceValidationExpressions`, `ReadWriteOncePod`, `APIPriorityAndFairness`, `KMSv2`, `JobReadyPods`, `CronJobTimeZone`, `ServerSideApply`, `ServerSideFieldValidation`) are locked on; most no longer appear in `--help`.

##### Already-GA features from earlier releases that 1.29 code should use

| Feature | GA since | Use it for |
|---|---|---|
| `policy/v1` PodDisruptionBudget, `autoscaling/v2` HPA, `batch/v1` CronJob | 1.21 / 1.23 / 1.21 | The only served versions |
| `seccompProfile` field, `RuntimeDefault` | 1.19 | Pod Security restricted requires it |
| Pod Security admission (`pod-security.kubernetes.io/*` labels) | 1.25 | Namespace-level enforcement (PodSecurityPolicy was removed in 1.25) |
| CronJob `timeZone` | 1.27 | Explicit schedules instead of the controller-manager's local time |
| Server-side field validation (`fieldValidation=Strict`) | 1.27 | Unknown/duplicate fields become errors (kubectl's default) |
| Server-side apply | 1.22 | Field ownership for controllers |
| `kubectl auth whoami` / SelfSubjectReview `authentication.k8s.io/v1` | 1.28 | Debugging identities |
| Non-graceful node shutdown (`node.kubernetes.io/out-of-service` taint) | 1.28 | StatefulSet failover |
| Retroactive default StorageClass assignment | 1.28 | PVCs without `storageClassName` get the default class once one exists |
| `ReadWriteOncePod`, CRD CEL validation rules, flowcontrol `v1`, KMS v2, Job `status.ready` | **1.29** | See above |

---

#### Do not use or recommend (1.30+ features)

The cluster is 1.29. Never suggest the following as available; they are rejected as unknown fields, silently dropped, return `no matches for kind`, or need a newer library. (Versions are taken from the 1.30–1.37 release notes; every 1.30 boundary below was verified on the v1.30.3 apiserver.)[^9]

**Kubernetes 1.30 (released 2024-04-17):**

- **`admissionregistration.k8s.io/v1` ValidatingAdmissionPolicy** (GA in 1.30). On 1.29 the `v1` kinds do not exist; the `v1beta1` API exists but is **not served by default**. A chart that ships a VAP for "1.29+" is uninstallable on a stock 1.29 cluster.
- **`securityContext.appArmorProfile`** (pod and container level; beta-on in 1.30, GA 1.31). Verified: rejected on 1.29 with `strict decoding error: unknown field "spec.securityContext.appArmorProfile"`. On 1.29 use the annotation `container.apparmor.security.beta.kubernetes.io/<container>`.
- **`lifecycle.preStop.sleep` / `postStart.sleep`** honoured (`PodLifecycleSleepAction` beta-on in 1.30). Verified: on 1.29 the `sleep` action is dropped and the Pod is **rejected** (`preStop: Required value: must specify a handler type`) — always, since a hook handler can hold only one action.
- **CRD validation ratcheting** on by default (`CRDValidationRatcheting` beta-on in 1.30, GA 1.33) and **`optionalOldSelf`**. Verified: on 1.29 a tightened rule blocks *every* update of a stored object that violates it (1.30 only warns); `optionalOldSelf: true` is dropped on 1.29.
- **CEL string formatting** `'%d'.format([...])`, `strings.quote()` and the **sets library** (`sets.contains()`, `sets.intersects()`, `sets.equivalent()`) in new CRD rules (strings library version 2, 1.30). Verified: 1.29 rejects `'got %d'.format([self.num])` with `undeclared reference to 'format'` and `sets.contains(...)` with `found no matching overload`; 1.30 accepts both.
- **Explicit `nominalConcurrencyShares: 0`** in `flowcontrol.apiserver.k8s.io/v1` (`ZeroLimitedNominalConcurrencyShares` GA in 1.30). Verified: 1.29 rejects it (`must be positive`), 1.30 accepts it.
- **Service `status.loadBalancer.ingress[].ipMode`** honoured (`LoadBalancerIPMode` beta-on in 1.30). Verified: dropped on 1.29, kept on 1.30.
- **CRD `selectableFields`** (field selectors for custom resources, alpha 1.30, beta-on 1.31), **Job `successPolicy`** and **`managedBy`** (alpha 1.30), **Service `trafficDistribution`** (alpha 1.30, beta-on 1.31), **`recursiveReadOnly`** mounts (alpha 1.30, beta-on 1.31). Verified: all rejected as unknown fields on 1.29.
- Other 1.30 graduations — do not describe them as GA on 1.29: webhook `matchConditions` (GA 1.30), Pod `schedulingGates` (GA 1.30), `minDomains` (GA 1.30), HPA `ContainerResource` (GA 1.30), `status.hostIPs` (GA 1.30), legacy SA token clean-up (GA 1.30), structured authentication/authorization configuration (beta-on 1.30), user namespaces (beta, off, in 1.30), contextual logging beta.
- `kubectl` 1.30: exec/attach over **WebSockets** by default (`TranslateStreamCloseWebsocketRequests` beta-on).

**Kubernetes 1.31 (released 2024-08-13):** AppArmor fields **GA** (annotations deprecated with a warning since 1.30); Job `podFailurePolicy`, `DisruptionTarget`, PDB `unhealthyPodEvictionPolicy`, elastic Indexed Jobs, StatefulSet `ordinals.start` and `LogarithmicScaleDown` **GA** (all beta-on and usable on 1.29 — do not call them GA); `trafficDistribution: PreferClose`, Job `successPolicy`, CRD field selectors, `recursiveReadOnly`, pod-affinity `matchLabelKeys`, `HonorPVReclaimPolicy`, `RetryGenerateName`, `ConsistentListFromCache` beta-on; CEL **IP/CIDR library** (`isIP()`, `ip()`, `isCIDR()`, `cidr()`) usable in new rules; `kubectl exec POD CMD` without `--` removed; `kubectl wait --for=create`; in-tree cloud providers removed entirely; `cephfs`/`rbd` volume plugins removed.

**Kubernetes 1.32 (released 2024-12-11):** MutatingAdmissionPolicy (alpha); `preStop.sleep.seconds: 0`; relaxed environment-variable names (leading digit); Job `managedBy` beta-on; Pod-level `spec.resources` (alpha); CEL name-format library (`format.dns1123Label()`); `flowcontrol.apiserver.k8s.io/v1beta3` **removed**; DRA `resource.k8s.io/v1beta1`.

**Kubernetes 1.33–1.37:** native sidecars GA (1.33); in-place Pod resize beta-on (`pods/resize`, 1.33) and GA (1.35); user namespaces on by default (1.33) and GA (1.36); CEL two-variable comprehensions (1.33) and `isSemver()`/`semver()` (1.34); Job `backoffLimitPerIndex`/`successPolicy` GA (1.33); DRA GA `resource.k8s.io/v1` and VolumeAttributesClass GA `storage.k8s.io/v1` (1.34); `PodLifecycleSleepAction` GA (1.34); Job `podReplacementPolicy` GA (1.34); MutatingAdmissionPolicy beta (1.34) and `v1` (1.36); `PreferSameZone`/`PreferSameNode` traffic distribution (1.34+); Pod `metadata.generation`/`status.observedGeneration` (1.33 alpha → 1.35 GA); image volumes (`volumes[].image`, alpha 1.31 → GA 1.36); `supplementalGroupsPolicy`; container `restartPolicyRules`; `env[].valueFrom.fileKeyRef`; StatefulSet `updateStrategy.type: Recreate` (1.37); KYAML output (`kubectl get -o kyaml`, kubectl 1.34 opt-in).[^9]

**Library versions to reject for a 1.29-only target:** controller-runtime **v0.18+** APIs do not exist in v0.17: typed/generic predicates and handlers (`predicate.TypedPredicate[T]`, `handler.TypedEventHandler`, `source.Kind(cache, obj, handler, preds...)` — v0.18), `cache.TransformStripManagedFields()` (v0.18), `client.Client.Apply(ctx, applyConfiguration)` (v0.22), `mgr.GetEventRecorder` for events.k8s.io (v0.23), the generic `ctrl.NewWebhookManagedBy(mgr, obj)` and `admission.Validator[T]` (v0.23), `reconcile.Result.Priority` (v0.23). Newer client-go/apimachinery symbols such as `fake.NewClientset` and `utilruntime.HandleErrorWithContext` (v0.31) or `cache.WaitForNamedCacheSyncWithContext` (v0.33) do not exist in v0.29 (verified in the module sources). Never suggest them for a v0.17 / v0.29 module.

**Never recommend** `appArmorProfile`, `preStop.sleep`, `trafficDistribution`, `hostUsers: false`, `selectableFields`, Job `successPolicy`/`managedBy`, `recursiveReadOnly`, VAP `admissionregistration.k8s.io/v1`, CEL `format()`/`strings.quote()`/`sets`/`isIP()`/`isCIDR()`/`format.dns1123Label()`/`isSemver()`, `kubectl wait --for=create`, `kubectl logs --all-pods` or `kubectl patch --subresource resize` for a 1.29 cluster.

---

#### Deprecated and removed — what 1.29 code must not rely on

| Item | Status on 1.29 | Replacement |
|---|---|---|
| `flowcontrol.apiserver.k8s.io/v1beta2` | **Removed** (not served) | `flowcontrol.apiserver.k8s.io/v1` |
| `flowcontrol.apiserver.k8s.io/v1beta3` | Deprecated (warning), removed in 1.32 | `flowcontrol.apiserver.k8s.io/v1` |
| CronJob `TZ=`/`CRON_TZ=` prefixes in `spec.schedule` | **Rejected** on create | `spec.timeZone` |
| Service load-balancer status on non-`LoadBalancer` Services | **Rejected** | Only write `status.loadBalancer` for `type: LoadBalancer` |
| In-tree cloud providers (`--cloud-provider=aws\|gce\|azure\|vsphere\|openstack`) | **Disabled by default** (opt back in with `DisableCloudProviders=false`, `DisableKubeletCloudCredentialProviders=false`); removed in 1.31 | External cloud-controller-manager, `--cloud-provider=external` |
| kube-apiserver `--cloud-provider` / `--cloud-config` | Deprecated | Remove the flags |
| `Node.status.nodeInfo.kubeProxyVersion` | Deprecated | Query kube-proxy directly (DaemonSet image / metrics) |
| KMS v1 encryption provider | Deprecated, **disabled by default** (`KMSv1=false`) | KMS v2 (`apiVersion: v2` in the provider config) |
| `networking.k8s.io/v1alpha1` ClusterCIDR (alpha) | **Removed** | — |
| `kubescheduler.config.k8s.io/v1beta3`, `SelectorSpread` plugin | **Removed** | `kubescheduler.config.k8s.io/v1`, `PodTopologySpread` |
| Legacy auto-generated SA token Secrets | Labelled invalid after 1 year unused, deleted a year later | `kubectl create token` / projected tokens / explicit `kubernetes.io/service-account-token` Secrets |
| `kubectl exec POD CMD` (no `--`) | Deprecated (prints a notice) — removed in kubectl 1.31 | `kubectl exec POD -- CMD` |
| Legacy package repositories `apt.kubernetes.io` / `yum.kubernetes.io` | Frozen (2023-09), removed (2024) | `pkgs.k8s.io` |
| kubelet resource metric `scrape_error` | Deprecated (renamed) | `resource_scrape_error` |
| scheduler metric `pod_scheduling_duration_seconds` | Deprecated | `pod_scheduling_sli_duration_seconds` |
| controller-runtime `admission.Validator` / `admission.Defaulter` (methods on the API type), `ValidatingWebhookFor` / `DefaultingWebhookFor` | **Deprecated in v0.17** ("Deprecated: Use CustomValidator instead"), removed in v0.20 | `admission.CustomValidator` / `CustomDefaulter` + `WithValidator` / `WithDefaulter` |
| `k8s.io/utils/pointer` | Deprecated | `k8s.io/utils/ptr` (`ptr.To`, `ptr.Deref`) |
| `wait.Poll`, `wait.PollImmediate`, … (`k8s.io/apimachinery/pkg/util/wait`) | Deprecated | `wait.PollUntilContextTimeout` / `PollUntilContextCancel` |
| PodSecurityPolicy (`policy/v1beta1`) | Removed since 1.25 | Pod Security admission / VAP / policy engines |
| `gitRepo` volumes | Deprecated since 1.11 (still functional on 1.29; CVE-2024-10220 before kubelet v1.29.7; CVE-2025-1767 unfixed) | init container / git-sync |
| Service `externalIPs` | Still supported; CVE-2020-8554 | Block with `DenyServiceExternalIPs` or a policy engine; use LoadBalancer/Gateway API |

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

##### Recommended Scaffold (Kubebuilder v3 `go/v4` plugin)

```
├── api/
│   └── v1/
│       ├── rediscluster_types.go        # API type definitions + markers
│       ├── groupversion_info.go
│       └── zz_generated.deepcopy.go     # generated, never hand-edited
├── internal/
│   └── controller/
│       └── rediscluster_controller.go   # Reconciler
├── api/v1/rediscluster_webhook.go       # Kubebuilder v3 puts webhooks next to the API type
├── config/
│   ├── crd/                             # Generated CRD manifests
│   ├── rbac/                            # Generated RBAC manifests
│   └── default/                         # Kustomize overlay
├── cmd/main.go
├── Dockerfile
└── Makefile                             # make generate, make manifests, make test
```

Kubebuilder v3.14/v3.15 webhook scaffolds still generate methods on the API type (`func (r *RedisCluster) ValidateCreate() (admission.Warnings, error)`), i.e. the `admission.Validator` interface that controller-runtime **v0.17 already deprecates**. New code should use a separate `CustomValidator` type (see "Admission").

##### Toolchain versions for a 1.29 target

| Component | Version for 1.29 | Notes |
|---|---|---|
| Go | ≥ 1.21 (module directive); build with a **supported** Go release | `k8s.io/* v0.29` and controller-runtime v0.17 declare `go 1.21`. Go 1.21 has been unsupported since August 2024 — build with a current Go toolchain; the modules compile with newer Go (verified with Go 1.24.7) |
| `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` | v0.29.x (keep all on the **same** patch, ideally v0.29.15) | Mixed minors cause compile or runtime scheme errors |
| controller-runtime | v0.17.x (final v0.17.6) | Its `go.mod` requires `k8s.io/* v0.29.2`; Go MVS raises it to your v0.29.x patch |
| controller-tools (`controller-gen`) | v0.14.0 | **No `messageExpression`, `reason` or `fieldPath` in `+kubebuilder:validation:XValidation`** (verified: `unknown argument "fieldPath"`; `messageExpression` arrives in v0.15, `reason`/`fieldPath` in v0.16) |
| Kubebuilder | v3.14.0–v3.15.x (scaffolds controller-runtime v0.17) | v4.0.0 scaffolds controller-runtime v0.18 (Kubernetes 1.30 libraries) |
| envtest binaries | 1.29.x (`setup-envtest use 1.29.x`) | Test against the version you ship to |

```go
// go.mod — all k8s.io modules on one patch release
module example.com/redis-operator

go 1.21

require (
	k8s.io/api v0.29.15
	k8s.io/apimachinery v0.29.15
	k8s.io/client-go v0.29.15
	sigs.k8s.io/controller-runtime v0.17.6
)
```

Flag in review:

- `k8s.io/client-go v0.30.x` (or controller-runtime v0.18+) in a module that claims to support a 1.29 cluster is usually fine on the wire (client/server skew of ±1 minor is supported), but it drags in 1.30 API fields that the 1.29 server rejects (`appArmorProfile`) or drops, and the generic predicate/handler API changes. Pin the library minor to the oldest cluster minor you support when you rely on new fields.
- A `toolchain go1.21.x` line or a CI image pinned to Go 1.21/1.22: those Go releases are out of support and miss security fixes; the module directive may stay at `go 1.21`.

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
| **Environment variable name** (1.29) | `env[].name`, keys imported through `envFrom` | `A–Z`, `a–z`, `0–9`, `_`, `-`, `.` | everything else, including a leading digit | must not start with a digit (regex `[-._a-zA-Z][-._a-zA-Z0-9]*`) | no hard limit |
| **ConfigMap / Secret data key** | `data` / `stringData` keys | `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` | `/`, whitespace, `:`, all other punctuation | any alphanumeric | 253 |
| **Path Segment Name** | `metadata.name` for objects exposed in URL paths | DNS Subdomain set MINUS `.` and `..` as full name | same as DNS Subdomain plus the literal names `.` and `..` | as DNS Subdomain | 253 |

> ⚠️ Underscore `_` is **never** valid in a DNS Subdomain or DNS Label name — that rules it out for `metadata.name`, `metadata.namespace`, API group, and Service name. Underscore IS valid in label/annotation keys (name part), label values, and ConfigMap/Secret data keys.

> ⚠️ Uppercase letters are **never** valid in DNS Subdomain or DNS Label names. They ARE valid in label keys (name part), label values, qualified names, and environment variable names.

> ⚠️ **Service names must start with a letter on 1.29.** Verified: 1.29 rejects `metadata.name: 1password-connect` with `a DNS-1035 label must consist of lower case alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character`.

> ⚠️ **Environment variable names on 1.29** may contain `.` and `-` but must not start with a digit (verified: `1VAR` is rejected with `must not start with a digit`; `my.var-1` is accepted). Names that are not C identifiers (`[A-Za-z_][A-Za-z0-9_]*`) are invisible to POSIX shells; keep C identifiers for anything a shell script reads. `envFrom` silently skips keys that are not valid environment variable names (the kubelet records an `InvalidEnvironmentVariableNames` event).

##### API Group, Kind, Resource

| Element | Rule | Example |
|---|---|---|
| **API Group** | Lowercase DNS subdomain you own | `cache.example.com` |
| **Kind** | CamelCase, singular | `RedisCluster` |
| **Resource (plural)** | Lowercase, plural | `redisclusters` |
| **Short name** | 2–5 lowercase chars; register in `.spec.names.shortNames`; check `kubectl api-resources` for collisions (`rc` is ReplicationController, `cm` ConfigMap, `ds` DaemonSet, `rs` ReplicaSet) | `rcl` |
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
| Never a YAML 1.1 boolean word as a property name | `count`, `enabled` | `y`, `n`, `on`, `off`, `yes`, `no` — verified: a CRD property written as `n:` is parsed as the key `false`, and every rule that references `self.n` then fails with `undefined field 'n'` |

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

Topology labels on Pods: the apiserver does **not** copy `topology.kubernetes.io/zone`/`region` from the Node onto Pods on 1.29 (that admission plugin arrives in 1.33). A Pod that needs its zone must read the Node (`get nodes` RBAC) or receive it from its controller; the Downward API exposes `spec.nodeName`, `status.hostIP` and (beta-on in 1.29) `status.hostIPs`.

---

#### CRD & API Design

##### Minimal Skeleton

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=rcl
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

Do not add `+kubebuilder:selectablefield` for a 1.29 target: controller-tools v0.14 does not know the marker, and a CRD that contains `spec.versions[].selectableFields` is **rejected** by a 1.29 apiserver (verified: `strict decoding error: unknown field "spec.versions[0].selectableFields"`). Field selectors on custom resources only work for `metadata.name` and `metadata.namespace` on 1.29; filter by spec fields with labels or client-side.

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
- **Bound everything** that CEL will touch: `maxLength` on strings, `maxItems` on arrays, `maxProperties` on maps. Without bounds the CEL cost estimator assumes worst-case sizes and rejects the CRD (see next section). Since 1.29 an `enum` also bounds a string for the estimator (the longest enum value is used).
- **`format: int32` is not a range check.** Add explicit `minimum`/`maximum` (`+kubebuilder:validation:Minimum/Maximum`).

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

**[GA in 1.29]** CRD validation rules (`x-kubernetes-validations`) graduate to GA in 1.29 (beta and on by default since 1.25) and are the first choice for invariants that the OpenAPI schema cannot express — on 1.29 they are also the only in-process validation that is on by default, because ValidatingAdmissionPolicy is not served.[^8] The following Go type was used as the source of the verified CRD below (controller-gen v0.14.0 generated `rule` and `message` from it):

```go
// RedisClusterSpec is the desired state.
// +kubebuilder:validation:XValidation:rule="self.tier != 'premium' || self.replicas >= 3",message="premium tier needs at least 3 replicas"
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

With controller-tools v0.14 the marker accepts only `rule` and `message`. To use `messageExpression`, `reason` or `fieldPath` (API fields that exist since 1.28), patch the generated CRD (Kustomize JSON patch) or upgrade controller-tools — v0.15 adds `messageExpression`, v0.16 adds `reason` and `fieldPath`; both still generate CRDs a 1.29 apiserver accepts.

The same kind of rules written directly in a CRD manifest, using only CEL functions that a 1.29 apiserver accepts in **new** rules (`quantity`, `url`/`isURL`, regex `matches`, strings extensions — all verified):

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
            required: [tier, replicas, image]
            x-kubernetes-validations:
            - rule: "!has(self.maxMemory) || quantity(string(self.maxMemory)).isGreaterThan(quantity('64Mi'))"
              message: maxMemory must be larger than 64Mi
            - rule: "self.tier != 'premium' || self.replicas >= 3"
              message: premium tier needs at least 3 replicas
              reason: FieldValueInvalid
            - rule: "!has(oldSelf.storageClassName) || has(self.storageClassName)"
              message: storageClassName cannot be removed once set
            properties:
              tier: {type: string, enum: [standard, premium]}
              replicas: {type: integer, format: int32, minimum: 1, maximum: 99}
              image:
                type: string
                maxLength: 512
                x-kubernetes-validations:
                - rule: "self.contains('@sha256:')"
                  message: image must be pinned by digest
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
                  maxLength: 18
                  x-kubernetes-validations:
                  # no IP/CIDR library on 1.29 (isCIDR() is 1.31+): use a regex
                  - rule: "self.matches('^([0-9]{1,3}\\\\.){3}[0-9]{1,3}/[0-9]{1,2}$')"
                    message: must be an IPv4 CIDR such as 10.0.0.0/8
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

Observed results on 1.29 (all errors are reported together). The "invalid value" of a CEL failure is the **type name** (`"object"`, `"string"`), not the value — tests that match on the rejected value must match on the message instead:

```text
The RedisCluster "bad" is invalid:
* spec: Invalid value: "object": maxMemory must be larger than 64Mi
* spec: Invalid value: "object": premium tier needs at least 3 replicas
* spec.allowedSources[1]: Invalid value: "string": must be an IPv4 CIDR such as 10.0.0.0/8   # fd00::/8
* spec.allowedSources[2]: Invalid value: "string": must be an IPv4 CIDR such as 10.0.0.0/8   # "nope"
* spec.endpoints[0].url: Invalid value: "string": url must be an https URL
* spec.image: Invalid value: "string": image must be pinned by digest
# updates: fast -> slow, then removal
* spec.storageClassName: Invalid value: "string": storageClassName is immutable
* spec: Invalid value: "object": storageClassName cannot be removed once set
```

##### **`fieldPath` is broken on 1.29 when a schema node has several rules**

`fieldPath` exists since 1.28, but on 1.29 it leaks into the *other* rules of the same `x-kubernetes-validations` list. Verified on v1.29.5 with the CRD above plus `fieldPath: .maxMemory` on the first rule and `fieldPath: .replicas` on the second:

```text
* spec.maxMemory: Invalid value: "object": maxMemory must be larger than 64Mi
* spec.maxMemory.replicas: Invalid value: "object": premium tier needs at least 3 replicas   # wrong path
```

The fix (kubernetes#123475, "providing a fieldPath to a CRD Validation Rule would erroneously affect the reported field path of other unrelated CRD Validation Rules") shipped in 1.30 and was **not** backported to any 1.29 patch; on v1.30.3 the same CRD reports `spec.replicas`.[^10] On 1.29, either put each `fieldPath` rule on a schema node of its own (for example move the rule onto the field and use transition rules there) or omit `fieldPath` and name the field in the message. Clients that map errors to form fields by path must not trust `fieldPath` on 1.29.

##### CEL library availability (new rules, verified per apiserver)

| Library / function | **1.29** | 1.30 |
|---|---|---|
| Standard macros (`has`, `all`, `exists`, `exists_one`, `map`, `filter`), `matches`, `size`, `contains`, `startsWith`, `duration()`, `timestamp()` | ✅ | ✅ |
| Kubernetes list library (`isSorted()`, `sum()`, `indexOf()`, `min()`, `max()`) | ✅ (verified: `self.li.sum() <= 10` compiles and rejects `[5, 6]`; also on 1.28) | ✅ |
| Kubernetes regex library (`find()`, `findAll()`) | ✅ | ✅ |
| Kubernetes URL library (`isURL()`, `url()`, `getScheme()`, `getHost()`, …) | ✅ | ✅ |
| Strings extensions (`lowerAscii()`, `upperAscii()`, `replace()`, `split()`, `trim()`, `charAt()`, `indexOf()`, list `join()`) | ✅ | ✅ |
| **Quantity library** (`quantity()`, `isQuantity()`, `isGreaterThan()`, …) | ✅ **new in 1.29** | ✅ |
| **CEL optional types** (`self.?field.orValue(x)`, `optional.of()`) | ✅ **new in 1.29** | ✅ |
| String formatting `'%d'.format([...])` and `strings.quote()` (strings library version 2) | ❌ (`undeclared reference to 'format'` / `'strings'`) | ✅ |
| Sets library (`sets.contains()`, `sets.intersects()`, `sets.equivalent()`) | ❌ | ✅ |
| IP / CIDR library (`isIP()`, `ip()`, `isCIDR()`, `cidr()`) | ❌ | ❌ (1.31) |
| Name formats (`format.dns1123Label()`, …) | ❌ | ❌ (1.32) |
| `isSemver()`, `semver()`; `base64.encode()`, `math.greatest()`, string `reverse()` | ❌ | ❌ |

The documentation lists the quantity library, CEL optional types and cross-type numeric comparisons as "Kubernetes versions 1.29+" for new expressions: a 1.28 apiserver rejects new rules that use them (verified on v1.28.0: the CRD above fails with `undeclared reference to 'quantity'`), while rules *stored* by a 1.29 apiserver keep evaluating after a rollback to 1.28.[^8] The cost estimator is a separate hurdle: functions that build new strings or lists (`string(int)`, `map()`, `join()` on a literal list) can push an otherwise valid rule over the budget even when all inputs are bounded (verified: `self.li.map(x, x * 2)` on an int list with `maxItems: 5` is rejected for cost on 1.29); test every rule against a 1.29 apiserver. Review rule: in a chart that targets 1.29, any CEL function from the ❌ rows makes the CRD (or policy) uninstallable; in a chart that also supports 1.28, the "new in 1.29" rows do as well.

##### CEL review rules (verified pitfalls)

1. **No numbers in `messageExpression` on 1.29.** `'%d'.format()` does not exist yet, and `'got ' + string(self.replicas)` (also `string(int(...))`, `string(list.size())`) makes the CRD **uninstallable**: `estimated messageExpression cost exceeds budget by factor of more than 100x` — even with `maximum: 99`. String concatenation of bounded strings and conditionals (`'need at least 3, got ' + (self.replicas == 0 ? 'zero' : 'fewer')`) are accepted. On 1.29 prefer a static `message`.
   ```yaml
   # BAD — CRD rejected on 1.29 (and 1.30)
   messageExpression: "'need >= 3 replicas, got ' + string(self.replicas)"
   # BAD — CRD rejected on 1.29 (format() is 1.30+)
   messageExpression: "'need >= 3 replicas, got %d'.format([self.replicas])"
   # GOOD on 1.29
   message: premium tier needs at least 3 replicas
   ```
2. **`has()` takes a field selection, not a variable.** `!has(oldSelf)` fails to compile. `optionalOldSelf: true` (which makes `oldSelf` an optional) is **dropped** on a default 1.29 apiserver (verified), so `oldSelf.hasValue()` / `oldSelf.value()` fail with `found no matching overload for 'hasValue'`. On 1.29 write transition rules that work without it: they only run when both old and new values exist.
3. **`self == oldSelf` does not stop removal.** Transition rules only run when both old and new values exist. Add a parent-level rule such as `!has(oldSelf.storageClassName) || has(self.storageClassName)` when "immutable" must also mean "cannot be unset" (verified: removal is then rejected with `spec: Invalid value: "object": storageClassName cannot be removed once set`).
4. **No validation ratcheting on 1.29.** `CRDValidationRatcheting` is alpha/off: after a CRD update tightens a rule, **every** update of a stored object that violates the new rule is rejected — even an unrelated change. Verified: after adding `self.startsWith('registry.example.com/')` to `image`, `kubectl patch rcl legacy -p '{"spec":{"replicas":4}}'` fails on 1.29 with `spec.image: Invalid value: "string": images must come from registry.example.com`; on 1.30 the same patch succeeds with only a warning. Tightening validation on 1.29 therefore needs a migration of existing objects first.
5. **Unbounded inputs make the CRD uninstallable**, not slow: the cost estimator assumes maximum sizes. Every string/array/map reached by a rule needs `maxLength`/`maxItems`/`maxProperties`.
6. **Prefer `reason`** (`FieldValueInvalid`, `FieldValueForbidden`, `FieldValueRequired`, `FieldValueDuplicate`) so clients see the right status cause; use `fieldPath` only with the 1.29 bug above in mind.
7. `x-kubernetes-int-or-string` fields are dynamic in CEL; guard with `type(self.x) == string` or convert with `string(self.x)` before `quantity()` and restrict the input with a `pattern` first.
8. **YAML 1.1 coercion in schemas.** An unquoted property name `n`, `y`, `on`, `off`, `yes` or `no` becomes a boolean key (verified: `n:` broke every rule referencing `self.n`). Quote such keys or rename them.

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

`client.MergeFrom` sends a JSON merge patch without a `resourceVersion` precondition. If two writers own different list entries (for example condition types), use `client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})` or server-side apply for status with a dedicated field manager. (controller-runtime v0.17 has no typed `Status().Apply`; use `r.Status().Patch(ctx, u, client.Apply, owner)` with an unstructured apply payload if you need SSA for status.)

##### observedGeneration

Always set `status.observedGeneration = metadata.generation` after a successful reconcile:

```go
obj.Status.ObservedGeneration = obj.Generation
```

This allows external tools to detect whether the controller has processed the latest spec change.

Pods on 1.29 do **not** maintain `metadata.generation`/`status.observedGeneration` (1.33 alpha → 1.35 GA). Code that waits for a Pod change must compare concrete status (`containerStatuses[].imageID`, readiness), not generations.

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

CRD status conditions (`Established`, `NamesAccepted`) carry no `observedGeneration` on 1.29. Tools that wait for a CRD update use `kubectl wait --for=condition=Established` plus a discovery check.

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
2. **Never tighten validation** on an existing field in a way that rejects previously valid values — on 1.29 there is no ratcheting, so a tightened rule also blocks unrelated updates of every stored object that violates it.
3. **Add fields as optional** (`omitempty`), never required.
4. Store only **one primary version** in etcd; all others are views via conversion.
5. Conversion must be **lossless** in both directions (round-trip unknown fields through annotations if needed).
6. For breaking changes: introduce a new version, mark the old as deprecated, maintain support for ≥ 1 year before removal.
7. Use a **conversion webhook** when schemas diverge between versions, with a valid `caBundle`.
8. **Before removing an old version from `spec.versions`**, every stored object must be rewritten to the new storage version and the old version removed from `status.storedVersions`. 1.29 has no in-tree `StorageVersionMigration` API; use the out-of-tree kube-storage-version-migrator or a read/replace loop (`kubectl get <plural> -A -o json | kubectl replace -f -`), then remove the old version from `status.storedVersions` with `kubectl patch crd <name> --subresource=status`.

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

##### Reconciler Skeleton (controller-runtime v0.17)

The following reconciler compiles against k8s.io/* v0.29.15 + controller-runtime v0.17.6 (`go build` and `go vet` clean) and was exercised with envtest on the v1.29.5 control-plane binaries: CEL rejection, SSA-created Deployment with one controller owner reference and `managedFields[0].manager == "rediscluster-controller"`, status condition transition to `Ready=True` after faking Deployment readiness, and finalizer-driven deletion.

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
- `GenerationChangedPredicate` drops metadata-only updates (labels, annotations, finalizer edits by others). Deletion of an object with finalizers still gets through: when the apiserver sets `deletionTimestamp` on a custom resource it also increments `metadata.generation` — verified in envtest on 1.29 (generation 1 → 2 on `Delete`), and the reconciler above ran its cleanup and removed the finalizer with this predicate in place.
- controller-runtime v0.17 predicates, handlers and sources are **not generic** (`predicate.Funcs` with `event.UpdateEvent`, `handler.EnqueueRequestsFromMapFunc(func(ctx, client.Object) []reconcile.Request)`); the `Typed*` variants arrive in v0.18.

##### Reconcile Return Conventions

| Return | Meaning |
|---|---|
| `ctrl.Result{}, nil` | Success — no requeue until a watched event fires |
| `ctrl.Result{}, err` | Transient error — requeued with per-item exponential backoff (5 ms doubling up to 1000 s by default) |
| `ctrl.Result{RequeueAfter: d}, nil` | Requeue after duration `d` (polling external state) |
| `ctrl.Result{Requeue: true}, nil` | Rate-limited requeue. Not deprecated in v0.17 (it is from v0.21), but prefer `RequeueAfter` or an error — its semantics are unclear to readers |
| `ctrl.Result{RequeueAfter: d}, err` | The error wins; `RequeueAfter` is ignored. Pick one |
| `ctrl.Result{}, reconcile.TerminalError(err)` | Logged and counted, never retried (exists in v0.17) |

controller-runtime v0.17 uses a plain rate-limited work queue (the priority queue is a v0.20 experimental option). A single key is never processed concurrently, but keys are not ordered — do not depend on ordering.

##### Manager Setup

```go
// stripManagedFields drops metadata.managedFields before objects enter the
// informer cache (controller-runtime v0.17 has no cache.TransformStripManagedFields).
func stripManagedFields(in any) (any, error) {
	if obj, ok := in.(client.Object); ok {
		obj.SetManagedFields(nil)
	}
	return in, nil
}

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
		DefaultTransform: stripManagedFields,
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

- `metricsserver.Options.BindAddress` defaults to **`:8080` over plain HTTP** in v0.17 (`SecureServing` is false by default). Serve metrics with `SecureServing: true` and the `filters.WithAuthenticationAndAuthorization` filter (both exist in v0.17 — verified); the kube-rbac-proxy sidecar used by older Kubebuilder scaffolds is deprecated. The filter needs RBAC for `authentication.k8s.io/tokenreviews` and `authorization.k8s.io/subjectaccessreviews` `create`.
- `cache.TransformStripManagedFields()` does not exist in v0.17 (verified; it arrives in v0.18); a hand-written `DefaultTransform` like the one above compiles and is the v0.17 equivalent. Never mutate anything else in a transform — the cached object is shared.
- `LeaderElection: true` for any controller that can run with more than one replica.
- `MaxConcurrentReconciles` > 1 is safe only if the reconciler holds no shared mutable state without locking; the queue guarantees a single key is never processed concurrently.
- v0.17 removed `apiutil.NewDiscoveryRESTMapper`; the cache uses the dynamic (lazy) REST mapper by default, and non-existing GroupVersions are no longer errors in the mapper.[^11]

---

#### Server-Side Apply from Controllers

Server-side apply (SSA) with a stable field manager is the preferred way for a controller to own child objects: it sends only the fields the controller cares about, merges with fields set by other actors (HPA replicas, mutating webhooks, users), and resolves conflicts per field. **controller-runtime v0.17 has no typed `client.Apply(ctx, applyConfiguration)` method** (added in v0.22). On v0.17 there are two correct ways:

1. Build a client-go **apply configuration** and send it as an Apply patch through an `unstructured.Unstructured` (shown below, verified in envtest on 1.29), or
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

**BAD — applying a typed struct.** `r.Patch(ctx, &appsv1.Deployment{...}, client.Apply, ...)` compiles and works, but the JSON of a typed struct contains zero values. Verified in envtest on 1.29: applying a typed Deployment that only set `replicas`, `selector`, template labels and one container made the field manager own `f:spec.f:strategy`, `f:spec.f:template.f:metadata.f:creationTimestamp` and the container's `f:resources` — fields it never meant to manage, which it will later reset or fight over with other managers.

SSA review rules:

- **Always send the complete desired intent** of the fields you own. A field omitted from a later apply is *removed* if this manager was its only owner. Building the apply configuration conditionally ("only set replicas if changed") is a bug.
- **Use apply configurations (or unstructured), not typed structs**, for SSA (see above).
- **One field manager per controller** (`client.FieldOwner`), constant across releases. Changing it orphans ownership and leaves stale fields behind.
- `client.ForceOwnership` is appropriate for a controller that is the source of truth for those fields; without it, a conflict (another manager set a different value) returns `409 Conflict`. Never force-apply fields that users are expected to tune (e.g. `replicas` when an HPA targets the Deployment) — omit them from the apply configuration instead.
- Do not mix SSA and `Update` on the same object from the same controller; `Update` takes ownership of every field (manager with operation `Update`) and later applies then fight the old manager entry.
- For CRDs without generated apply configurations, either generate them (client-go's `applyconfiguration-gen` v0.29) or use `client.MergeFrom` patches; never hand-build `unstructured` apply payloads without `apiVersion`/`kind`.
- **The controller-runtime v0.17 fake client does not support SSA.** Verified: `Patch(..., client.Apply, ...)` on an existing object returns `apply patches are not supported in the fake client. Follow https://github.com/kubernetes/kubernetes/issues/115598 for the current status`, and on a missing object it returns NotFound. Test SSA code paths with envtest (see "Testing").
- 1.29 fixed SSA, create and update handling of objects with **duplicate items in keyed lists** (for example two `env` entries with the same name): such requests no longer wipe `managedFields`, and an apply that touches the duplicated item replaces the duplicates with the single applied item.[^3]

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

Note: `GenerationChangedPredicate` ignores metadata-only updates (label, annotation and finalizer edits). If the controller must react to label or finalizer changes, combine predicates (`predicate.Or`) or write a custom one, as below.

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
// Watch a shared Secret; enqueue all CRs referencing it (v0.17 signature:
// Watches(client.Object, handler.EventHandler, ...WatchesOption))
Watches(
    &corev1.Secret{},
    handler.EnqueueRequestsFromMapFunc(r.findCRsForSecret), // func(ctx context.Context, obj client.Object) []reconcile.Request
    builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
)
```

`findCRsForSecret` must use a **field index** (`mgr.GetFieldIndexer().IndexField(...)` + `client.MatchingFields`) rather than listing every CR in the cluster on each Secret event. Watching all Secrets cluster-wide also caches all Secrets in memory — restrict the cache (`cache.Options.ByObject` with a label selector) or use `builder.OnlyMetadata`. Since v0.17 `client.MatchingFields` can combine several indexes.[^11]

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

Verified on 1.29: on a custom resource, `finalizers: [cleanup]` is accepted with `Warning: metadata.finalizers: "cleanup": prefer a domain-qualified finalizer name to avoid accidental conflicts with other finalizer writers`; on built-in types (e.g. a ConfigMap) it is rejected: `name is neither a standard finalizer name nor is it fully qualified`.

##### Rules

- Add the finalizer **before** creating any external resource; remove it only after cleanup succeeded.
- Cleanup logic **must be idempotent** — the reconcile loop may be called multiple times while `DeletionTimestamp` is set.
- Never add a new finalizer after `DeletionTimestamp` is set — only removal is allowed.
- Do not silently swallow errors; failing cleanup should be retried (return the error) and surfaced as a condition + Warning Event.
- Do not create resources inside a namespace that has `DeletionTimestamp` set.
- Removing the finalizer when the object is already gone returns NotFound — wrap with `client.IgnoreNotFound`.
- Remove finalizers through the main resource (`Patch`), as above. 1.29.0 fixed a bug where Services whose finalizer was removed through the status subresource held on to their ClusterIP/NodePort allocation longer than expected.[^3]

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
| Nodes can set owner references on themselves | CVE-2025-5187: a compromised kubelet can add an owner reference to its own Node and get it garbage-collected. Fixed only in v1.31.12/v1.32.8/v1.33.4 — **no 1.29 fix exists**; mitigate with the `OwnerReferencesPermissionEnforcement` admission plugin[^12] |

##### When to Use Finalizers vs. OwnerReferences

| Scenario | Use |
|---|---|
| Child resource in same namespace | `ownerReferences` (automatic GC) |
| Cluster-scoped child of a namespaced owner | `finalizer` on the owner (or use a cluster-scoped owner) |
| External resource (cloud, DNS) | `finalizer` |
| Cross-namespace cleanup needed | `finalizer` |

---

#### Events

controller-runtime v0.17's `mgr.GetEventRecorderFor(name)` returns a `k8s.io/client-go/tools/record.EventRecorder` that writes **core/v1 Events** (`""` API group). The events.k8s.io/v1 recorder (`mgr.GetEventRecorder`) only arrives in v0.23 — do not suggest it.

```go
// record.EventRecorder: object, type, reason, message format, args
r.Recorder.Eventf(rc, corev1.EventTypeWarning, "ApplyFailed", "apply Deployment: %v", err)
r.Recorder.Eventf(rc, corev1.EventTypeNormal, "Available", "all %d replicas ready", rc.Spec.Replicas)
```

Review points:

- **RBAC**: the controller needs `""` (core) `events` `create;patch` (`// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch`). Without it the recorder logs the failure and reconciliation continues — Events are silently lost.
- The message is a format string — never pass user-controlled text as the format (`Eventf(obj, t, r, err.Error())` is a format-string bug; `go vet` flags it); use `"%v", err` or `Event(obj, t, r, msg)`.
- `reason` is a CamelCase machine-readable word; do not put variable data in it (the broadcaster aggregates by reason).
- Emit Events on transitions (use the `changed` result of `meta.SetStatusCondition`), not on every reconcile.

---

#### Admission: Webhooks and (opt-in) ValidatingAdmissionPolicy

##### Validation Hierarchy (prefer left-to-right)

```
CRD schema + CEL rules  →  Validating webhook                →  ValidatingAdmissionPolicy
(GA 1.29, per CRD,          (matchConditions beta-on;           (beta, NOT served by default on 1.29;
 no extra process)           extra infra, TLS, HA)               only if the cluster enables it)
```

On a default 1.29 cluster the only admission mechanisms an application can ship are **CRD validation rules** and **admission webhooks** (or a policy engine that runs webhooks, such as Kyverno or Gatekeeper). Mutation is only possible with a **mutating webhook** — MutatingAdmissionPolicy does not exist before 1.32.

##### **[beta, off]** ValidatingAdmissionPolicy on 1.29 (verified)

`ValidatingAdmissionPolicy` is beta in 1.29 (`admissionregistration.k8s.io/v1beta1`, beta since 1.28) but, as a new beta API, **not served** unless the control plane runs with `--feature-gates=ValidatingAdmissionPolicy=true --runtime-config=admissionregistration.k8s.io/v1beta1=true`. On a stock 1.29 apiserver both `admissionregistration.k8s.io/v1` and `v1beta1` policies fail with `no matches for kind "ValidatingAdmissionPolicy"` (verified). Managed offerings differ — check the cluster before relying on it, and never ship a VAP as the *only* enforcement of a security invariant in a chart that claims to support 1.29.

With the gate and the API enabled on a v1.29.5 apiserver, this policy works:[^13]

```yaml
apiVersion: admissionregistration.k8s.io/v1beta1
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
apiVersion: admissionregistration.k8s.io/v1beta1
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

Review points for policies on 1.29:

- **Version**: `v1beta1` on 1.29; `v1` exists only from 1.30. A chart that must support 1.29 *and* 1.30+ should render the `apiVersion` from `.Capabilities.APIVersions.Has "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy"` and skip the policy entirely when neither version is served. Never use `v1alpha1`.
- A policy without a **binding** does nothing; the binding's `validationActions` decides `Deny`/`Warn`/`Audit`. `[Deny, Warn]` is rejected (verified: `must not contain both Deny and Warn`); roll out new policies with `[Warn, Audit]` first.
- Use `?`/`orValue` optional chaining for optional fields (CEL optionals are available in 1.29); plain `object.spec.x` on an absent field is a runtime error that triggers `failurePolicy`.
- Use `matchConditions` to exclude system users and controllers, otherwise a broken policy can block kube-system controllers.
- `paramKind` + `paramRef`: `paramRef.parameterNotFoundAction` is **required** (verified: `spec.paramRef.parameterNotFoundAction: Required value`) — `Deny` subjects requests without a matching param to the policy's `failurePolicy`, `Allow` lets them through. Choose deliberately. 1.29.2 fixed policies with CRD parameters that failed to synchronize.[^3]
- The CEL environment for policies on 1.29 is the same as for CRDs: no `'%d'.format()`, no sets, no IP/CIDR functions.

##### Admission webhooks (controller-runtime v0.17)

On v0.17 the webhook builder is **not** generic: `ctrl.NewWebhookManagedBy(mgr).For(&cachev1.RedisCluster{})` with an `admission.CustomValidator` / `admission.CustomDefaulter` whose methods take `runtime.Object`. The older `admission.Validator`/`Defaulter` interfaces (methods implemented on the API type itself, `webhook.Validator` in the Kubebuilder v3.14/v3.15 scaffolds) are **deprecated since v0.17** ("Deprecated: Use CustomValidator instead") and removed in v0.20.[^11]

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

- **Always check the type assertion** — an unchecked `obj.(*cachev1.RedisCluster)` panics if the webhook is ever registered for another type; v0.17 recovers the panic only when the builder's `RecoverPanic()` is called.
- `ctrl.NewWebhookManagedBy(mgr, &cachev1.RedisCluster{})` (two arguments) and `admission.Validator[*T]` are controller-runtime **v0.23** APIs; they do not compile against v0.17.
- The tier-downgrade check above is a simple transition rule (`self.tier == oldSelf.tier || oldSelf.tier != 'premium'`) and belongs in CEL; keep webhooks for checks that need external state.
- v0.17 limits admission request bodies to 7 MB (and TokenReview bodies to 1 MB).[^11]

##### Webhook configuration rules (verified on 1.29)

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata: {name: rediscluster-validator}
webhooks:
- name: vrediscluster.cache.example.com
  admissionReviewVersions: [v1]
  sideEffects: None
  failurePolicy: Fail
  matchPolicy: Equivalent
  timeoutSeconds: 5
  clientConfig:
    service: {name: webhook, namespace: redis-system, path: /validate-cache-example-com-v1-rediscluster}
  rules:
  - apiGroups: [cache.example.com]
    apiVersions: [v1]
    operations: [CREATE, UPDATE]
    resources: [redisclusters]
  namespaceSelector:
    matchExpressions:
    - {key: kubernetes.io/metadata.name, operator: NotIn, values: [kube-system, redis-system]}
  matchConditions:                      # beta, on by default since 1.28
  - name: not-system-users
    expression: "!request.userInfo.username.startsWith('system:')"
```

- Set `failurePolicy: Fail` for critical validation; `Ignore` only if the webhook is non-essential. Fail-closed means *every* matching request fails while the webhook is unreachable — verified on 1.29 with the configuration above and no backing Service: creating a RedisCluster returned `Internal error occurred: failed calling webhook "vrediscluster.cache.example.com": failed to call webhook`. Run webhooks with ≥ 2 replicas and a PDB.
- `sideEffects: None` (or `NoneOnDryRun`); webhooks with side effects break `kubectl apply --dry-run=server`.
- `matchPolicy: Equivalent` (the `v1` default) so the webhook fires on all API versions of the same resource.
- Handle `CREATE` **and** `UPDATE` for immutability rules; `DELETE` only for deletion guards.
- `timeoutSeconds` defaults to 10 and must be 1–30 (verified: `31` → `the timeout value must be between 1 and 30 seconds`); keep webhooks fast.
- `matchConditions` (beta-on since 1.28, GA in 1.30) skip irrelevant requests in-process; combine with `namespaceSelector`/`objectSelector` to exclude `kube-system` and the webhook's own namespace (a fail-closed webhook that matches its own Pods can deadlock the cluster).
- TLS certificates **must** be rotated; use cert-manager or managed CA bundle injection, and do not issue SHA-1 signed serving certificates (Go 1.18+ clients reject them unless `GODEBUG=x509sha1=1`).

---

#### RBAC & Least Privilege

##### Principles

1. **Least privilege** — grant only the verbs/resources explicitly required.
2. Prefer `Role` + `RoleBinding` (namespaced) over `ClusterRole` + `ClusterRoleBinding`.
3. **No wildcard verbs** (`*`) or **wildcard resources** (`*`).
4. **No `cluster-admin`** service accounts.
5. Mount service account tokens only when the workload needs API access (`automountServiceAccountToken: false` as default).
6. Treat `escalate`, `bind`, `impersonate`, `nodes/proxy`, `serviceaccounts/token` `create`, and `pods/exec` as privileged.

##### `pods/exec`, `pods/attach`, `pods/portforward`

kubectl 1.29 still uses SPDY (an HTTP `POST`, authorized with the **`create`** verb) for exec, attach and port-forward; WebSocket-based kubectl streaming only becomes the default in 1.30/1.31. The apiserver, however, also accepts **WebSocket** exec/attach requests, and a WebSocket upgrade is an HTTP `GET` that is authorized with the **`get`** verb. A role with only `get` on `pods/exec` therefore lets any WebSocket client exec into Pods on 1.29; this loophole is only closed in 1.35 (`AuthorizePodWebsocketUpgradeCreatePermission`).[^14]

```yaml
# BAD — looks "read-only", but allows exec through a WebSocket client
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

`verbs: ["get", "create"]` keeps every client working on 1.29 and on newer clusters. Audit custom read-only roles for `get` on these subresources — they are equivalent to exec rights.

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

// Events: controller-runtime v0.17 recorder writes core/v1 Events
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Leader election
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
```

Secrets: grant `get` on specific `resourceNames` where possible; `list`/`watch` on Secrets cluster-wide is equivalent to reading every Secret. RBAC cannot express "list only Secrets with label X" on 1.29 (authorization with selectors is a 1.31 alpha).

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
metadata:
  name: redis-operator
  namespace: redis-system
spec:
  serviceAccountName: redis-operator
  automountServiceAccountToken: true  # Explicitly opt in at pod level
  containers:
  - name: manager
    image: registry.example.com/redis-operator:1.0
```

**[NEW in 1.29 — beta, on]** **Legacy service-account token clean-up**: auto-generated `kubernetes.io/service-account-token` Secrets that are listed in a ServiceAccount's `.secrets`, are not mounted by any Pod and have not been used for `--legacy-service-account-token-clean-up-period` (default one year) get the label `kubernetes.io/legacy-token-invalid-since` — from then on authentication **rejects** them — and are deleted after another period of non-use. Removing the label re-activates a token. Tooling (CI systems, old dashboards) that still reads a long-lived token from such a Secret must move to `kubectl create token` / TokenRequest or to an explicitly created token Secret. Use of secret-based tokens is now recorded in the audit annotations `authentication.k8s.io/legacy-token-autogenerated-secret` / `legacy-token-manual-secret`.[^1][^3]

`kubectl create token SA --bound-object-kind` accepts only `Pod` and `Secret` on 1.29 (verified: `supported --bound-object-kind values are Pod, Secret`); node-bound tokens and the `jti` claim are alpha.

---

#### Workload Manifests: Pods

##### A production Deployment for 1.29 (verified)

Applied to a v1.29.5 apiserver into a namespace enforcing Pod Security `restricted:v1.29`; a Pod built from the template passes admission.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: payments
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: v1.29
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
      annotations:
        # AppArmor on 1.29 is configured with the beta annotation (fields arrive in 1.30)
        container.apparmor.security.beta.kubernetes.io/api: runtime/default
        container.apparmor.security.beta.kubernetes.io/log-shipper: runtime/default
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
      - name: log-shipper                      # native sidecar (beta, on by default in 1.29)
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
        - name: GOMAXPROCS
          valueFrom:
            resourceFieldRef: {resource: limits.cpu, divisor: "1"}
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
  unhealthyPodEvictionPolicy: AlwaysAllow       # beta, on by default since 1.27
  selector:
    matchLabels: {app.kubernetes.io/name: api}
```

A bare Pod in the same namespace without a security context is rejected: `violates PodSecurity "restricted:v1.29": allowPrivilegeEscalation != false ..., unrestricted capabilities ..., runAsNonRoot != true ..., seccompProfile ...`.

Review notes on this template:

- **No `lifecycle.preStop.sleep` on 1.29.** The usual "sleep a few seconds before shutdown so endpoints are removed first" must be implemented with `preStop: {exec: {command: ["/bin/sleep", "5"]}}` (only if the image contains a `sleep` binary) or inside the application (keep serving for a few seconds after `SIGTERM`). A manifest written for 1.30+ with `preStop.sleep` is **rejected** on 1.29 (verified, also when the container has other hooks such as a `postStart.exec`).
- `memory` requests = limits avoids OOM surprises for latency-critical services; `cpu` limits are a policy decision (throttling) — flag missing **memory** limits, not missing CPU limits.
- `matchLabelKeys: [pod-template-hash]` in topology spread (beta-on since 1.27) spreads each ReplicaSet revision independently. The same field in **pod (anti-)affinity** is alpha on 1.29 and silently dropped.
- `maxUnavailable: 0` + `maxSurge: 1` with a PDB of `minAvailable: 2` and three replicas keeps two Pods serving during rollouts and node drains.
- `GOMAXPROCS` from `resourceFieldRef` (rounded up to whole CPUs) matters for Go binaries built with Go < 1.25; Go ≥ 1.25 reads the cgroup CPU limit itself.[^15]
- Pinning `enforce-version` to `v1.29` keeps the rules stable across control-plane upgrades; the Pod Security version also decides which sysctls are allowed (see "What's new").

##### AppArmor on 1.29 (annotations, beta)

```yaml
metadata:
  annotations:
    container.apparmor.security.beta.kubernetes.io/app: runtime/default              # or localhost/<profile>
```

- On 1.29 AppArmor is configured **only** with the per-container annotation `container.apparmor.security.beta.kubernetes.io/<container-name>` (values `runtime/default`, `localhost/<profile>`, `unconfined`). The `securityContext.appArmorProfile` field is **rejected** as an unknown field on 1.29 (verified) — it arrives in 1.30.
- The annotation is accepted without any warning on 1.29; 1.30 starts warning `deprecated since v1.30; use the "appArmorProfile" field instead` (verified). A chart that supports 1.29 and 1.30+ should branch on the version (see the Helm example below) rather than drop AppArmor.
- The annotation key must name an existing container; a typo silently leaves that container on the runtime default. `localhost/` profiles must be loaded on every node. The Pod Security baseline policy forbids `unconfined`.

##### **[NEW in 1.29 — beta, on]** Native sidecar containers

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

- Sidecars start in init-container order before regular containers, keep running, are restarted independently, and support probes; a sidecar's `startupProbe` gates the next init container. They do not block Job completion.
- **[NEW in 1.29]** Termination is serialized: each sidecar receives `SIGTERM` only after all regular containers and all later-started sidecars have exited — sidecars must not exit early on `SIGTERM` if the main container still needs them during its own shutdown, and the whole sequence must fit into `terminationGracePeriodSeconds`.[^3]
- The feature is beta-on but still had bugs fixed during the 1.29 line: restart ordering relative to regular containers (1.29.0), a restartable init container that could not move from terminated to running in `restartPolicy: Never`/`OnFailure` Pods (v1.29.3), and sidecars that did not terminate gracefully when the Pod had not finished initialising (v1.29.8). Run ≥ v1.29.8 when relying on sidecars.[^3]
- Sidecar behaviour is implemented by the kubelet: it needs kubelets with `SidecarContainers` enabled (default from 1.29; alpha/off on 1.28 kubelets). Check node version skew before relying on sidecars while a 1.29 control plane still manages older nodes.
- Do not model sidecars as extra regular containers in Jobs — the Job never completes.

##### Fields that a default 1.29 apiserver silently drops

Verified with `kubectl apply --dry-run=server -o yaml` on v1.29.5 (no warning is printed):

| Manifest field | Gate (1.29) | What happens on 1.29 |
|---|---|---|
| `lifecycle.preStop.sleep` / `postStart.sleep` | `PodLifecycleSleepAction` alpha | Dropped — which empties the hook, so the Pod is **rejected** (`must specify a handler type`) |
| `hostUsers: false` | `UserNamespacesSupport` alpha | Dropped; the Pod runs in the host user namespace (container root = host root) |
| `containers[].securityContext.procMount: Unmasked` | `ProcMountType` alpha | Reset to `Default` |
| `containers[].resizePolicy` | `InPlacePodVerticalScaling` alpha | Dropped |
| `affinity.pod(Anti)Affinity.*.matchLabelKeys` / `mismatchLabelKeys` | `MatchLabelKeysInPodAffinity` alpha | Dropped — the term then matches Pods of *all* revisions |
| `StatefulSet.spec.updateStrategy.rollingUpdate.maxUnavailable` | `MaxUnavailableStatefulSet` alpha | Dropped (stored as `{"partition":0}`); updates proceed one Pod at a time |
| `PersistentVolumeClaim.spec.volumeAttributesClassName` | `VolumeAttributesClass` alpha | Dropped |
| Service `status.loadBalancer.ingress[].ipMode` | `LoadBalancerIPMode` alpha | Dropped |
| CRD `x-kubernetes-validations[].optionalOldSelf` | `CRDValidationRatcheting` alpha | Dropped |

A security reviewer must treat a manifest that relies on `hostUsers: false` for isolation as **unprotected** on a default 1.29 cluster.

##### Other Pod rules

- **`restartPolicy: Never` Pods** have no per-container restart rules on 1.29 (`restartPolicyRules` is 1.34+).
- **No in-place resize**: changing `resources` of a running Pod is rejected; change the controller's template (rollout) instead.
- **Pod scheduling gates** (`spec.schedulingGates`, beta-on since 1.27, verified kept) hold a Pod in `SchedulingGated` until an external controller removes the gates; gates can only be removed, never added after creation.
- **`status.hostIPs`** (beta-on in 1.29) lists both families on dual-stack nodes; prefer it over `status.hostIP` in new Downward-API consumers.
- **`IfNotPresent` and private images**: a Pod without pull credentials can start from a private image that another Pod pulled onto the same node (credential re-verification is a 1.33 alpha feature). Multi-tenant clusters must enforce `imagePullPolicy: Always` (the `AlwaysPullImages` admission plugin) to protect private images.
- **Image references**: always tag or pin by digest; `:latest` implies `imagePullPolicy: Always` by default.
- **Sysctls**: the Pod Security `v1.29` baseline allows `net.ipv4.tcp_keepalive_time`, `tcp_fin_timeout`, `tcp_keepalive_intvl` and `tcp_keepalive_probes` in addition to the earlier safe set; the kubelet allows them by default (`tcp_keepalive_time` needs kernel ≥ 4.5).[^3]

---

#### Workload Manifests: Controllers (Deployment, StatefulSet, Job, PDB, HPA)

##### Deployments and ReplicaSets

- `spec.selector` is immutable; changing labels in the selector requires a new Deployment.
- `progressDeadlineSeconds` only marks the `Progressing` condition `False`; it does not roll back.
- `LogarithmicScaleDown` (beta-on since 1.22): on scale-down the ReplicaSet controller groups Pods into logarithmic age buckets and picks randomly within a bucket. Use `controller.kubernetes.io/pod-deletion-cost` (beta since 1.22) to influence which Pods go first; never rely on "the newest Pod is removed".
- `status.terminatingReplicas` does not exist on 1.29 (1.33 alpha).

##### StatefulSets

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: kv
spec:
  serviceName: kv
  replicas: 3
  ordinals: {start: 5}              # beta-on since 1.27: Pods kv-5, kv-6, kv-7
  persistentVolumeClaimRetentionPolicy:
    whenDeleted: Delete             # beta-on since 1.27
    whenScaled: Retain
  selector: {matchLabels: {app: kv}}
  updateStrategy:
    type: RollingUpdate
  template:
    metadata: {labels: {app: kv}}
    spec:
      containers: [{name: kv, image: registry.example.com/kv:3.1}]
```

- `ordinals.start` lets a StatefulSet start at a non-zero ordinal (for slicing a StatefulSet across clusters or namespaces during migrations); verified kept on 1.29.
- `persistentVolumeClaimRetentionPolicy.whenDeleted: Delete` only for disposable data; the default `Retain` keeps PVCs.
- `rollingUpdate.maxUnavailable` is **dropped** on 1.29 (verified; `MaxUnavailableStatefulSet` alpha). Do not promise parallel StatefulSet updates.
- Pods get the `apps.kubernetes.io/pod-index` label (`PodIndexLabel`, beta-on) — prefer it over parsing the Pod name.
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
  backoffLimitPerIndex: 2          # NEW in 1.29: beta, on
  maxFailedIndexes: 2
  podReplacementPolicy: Failed     # NEW in 1.29: beta, on
  podFailurePolicy:                # beta, on since 1.26
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

Verified: 1.29 keeps every field; `successPolicy` and `managedBy` are rejected as unknown fields.

- **[NEW in 1.29 — beta, on]** `backoffLimitPerIndex` (requires `completionMode: Indexed`) counts failures per index; `maxFailedIndexes` fails the Job once too many indexes failed; the `FailIndex` action in `podFailurePolicy` fails just one index. Failed indexes are listed in `status.failedIndexes`.
- **[NEW in 1.29 — beta, on]** `podReplacementPolicy: Failed` creates a replacement Pod only after the old one is fully terminated (`TerminatingOrFailed` is the default); terminating Pods are counted in `status.terminating`. Use `Failed` for workloads that must never run two Pods of the same index at once (for example TensorFlow/JAX workers). With `podFailurePolicy` set, only `Failed` is allowed (verified: `spec.podReplacementPolicy: Unsupported value: "TerminatingOrFailed": supported values: "Failed"`).
- `podFailurePolicy` (beta-on) requires `restartPolicy: Never`. Combine `Ignore` on the `DisruptionTarget` condition with `FailJob` on non-retriable exit codes so preemption and node drains do not consume `backoffLimit`.
- **[GA in 1.29]** `status.ready` (`JobReadyPods`) is always populated — use it instead of counting Pods.
- Elastic Indexed Jobs (beta-on since 1.27): `completions` may be updated together with `parallelism` (they must remain equal).
- **Do not use `spec.managedBy` or `spec.successPolicy`** on 1.29 — they are rejected (see "Do not use").
- For CronJobs, always set `timeZone` (GA 1.27) and `concurrencyPolicy` explicitly; `startingDeadlineSeconds` avoids catch-up storms after controller downtime. **[NEW in 1.29]** `TZ=`/`CRON_TZ=` in `spec.schedule` is rejected for new CronJobs (verified):

```yaml
# BAD — rejected on 1.29: cannot use TZ or CRON_TZ in schedule, use timeZone field instead
spec:
  schedule: "CRON_TZ=Europe/Berlin 0 3 * * *"
# GOOD
spec:
  schedule: "0 3 * * *"
  timeZone: Europe/Berlin
```

##### PodDisruptionBudget — `unhealthyPodEvictionPolicy` (beta, on since 1.27)

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata: {name: api}
spec:
  minAvailable: 2
  unhealthyPodEvictionPolicy: AlwaysAllow
  selector: {matchLabels: {app: api}}
```

`AlwaysAllow` lets drains evict Pods that are running but not ready even when the budget is exhausted — the recommended setting for most workloads, because otherwise a crash-looping Deployment blocks node drains forever. Keep the default `IfHealthyBudget` only where an unready Pod still holds data that must not be lost. An empty selector (`{}`) selects all Pods in the namespace; 1.29 fixed drains that did not evict every such Pod.[^3]

##### HorizontalPodAutoscaler

- Use `autoscaling/v2`; `behavior.scaleUp/scaleDown` policies and `stabilizationWindowSeconds` are GA. Per-HPA `tolerance` does **not** exist (1.33 alpha); the global 10% tolerance applies.
- `ContainerResource` metrics (beta-on since 1.27, GA 1.30) let an HPA ignore sidecar usage. Since 1.29 restartable init containers (sidecars) are included in Pod-level `Resource` metric calculations — an HPA that targets Pod CPU utilization sees a different ratio once a sidecar is added.[^3]
- `minReplicas: 0` needs the alpha `HPAScaleToZero` gate — not available on a default 1.29 cluster.
- Do not set `spec.replicas` in a Deployment managed by an HPA when applying with SSA or `kubectl apply` — every apply resets the replica count. Omit the field.

---

#### Networking: Services & NetworkPolicy

##### Service names, IPs and `externalIPs`

- Service names must be RFC 1035 labels (no leading digit) — see Naming.
- Use canonical IP literals. 1.29 warns on `clusterIP: 10.0.0.010` (`IP address was accepted, but will be invalid in a future Kubernetes release`, verified); newer releases reject it.
- `spec.externalIPs` lets any user who can create Services intercept traffic for arbitrary IPs (CVE-2020-8554). Clusters should block it with the `DenyServiceExternalIPs` admission plugin or a policy engine; new manifests should use `LoadBalancer`, `NodePort` or Gateway API. (1.29 now allows `externalTrafficPolicy` on Services with `externalIPs`.)[^3]
- **[NEW in 1.29]** `status.loadBalancer` is only accepted for `type: LoadBalancer` Services (verified). Load-balancer controllers must clear their status before or when a Service changes type.

##### Topology-aware routing on 1.29

`spec.trafficDistribution` does not exist on 1.29 (rejected as unknown field). The 1.29 mechanism is the annotation `service.kubernetes.io/topology-mode: Auto` (topology-aware hints, `TopologyAwareHints` beta-on): the EndpointSlice controller adds zone hints only when endpoints are spread proportionally to node CPU capacity, and kube-proxy falls back to cluster-wide routing otherwise. `internalTrafficPolicy: Local` is a hard requirement (no fallback), not a preference — flag it where availability matters.

##### kube-proxy and load balancers on 1.29

- iptables is the default mode; IPVS works; nftables is **alpha** (`NFTablesProxyMode=true` plus `mode: nftables`) — do not recommend it for production 1.29 clusters.
- **[NEW in 1.29]** `kube-proxy --init-only` lets the privileged sysctl/conntrack setup run in an init container so the main container can use a stricter `securityContext`.
- `status.loadBalancer.ingress[].ipMode` is alpha and dropped (verified); cloud controllers that rely on `ipMode: Proxy` need 1.30+.
- A 1.29 regression around conntrack cleanup for UDP Services was fixed in v1.29.10, and a kube-proxy start-up race that could blackhole UDP traffic to a Service VIP was fixed in v1.29.9 — UDP-heavy clusters (DNS!) should run ≥ v1.29.10.[^3]

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
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny
  namespace: payments
spec:
  podSelector: {}
  policyTypes: [Ingress, Egress]
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

- Every namespace should have a default-deny policy plus explicit allows, including DNS egress to `kube-system` on UDP/TCP 53 (as above).
- `namespaceSelector` and `podSelector` in the **same** `from`/`to` element are ANDed; in separate list elements they are ORed — a frequent bug.
- NetworkPolicy has no effect unless the CNI enforces it.
- Restrict operator egress to the API server: `ports: [443, 6443]`; to pin the destination add an `ipBlock` with the API server endpoint IPs (`kubectl get endpointslices -n default -l kubernetes.io/service-name=kubernetes`).

##### Ingress

New HTTP routing should target Gateway API (`gateway.networking.k8s.io/v1`, installed as CRDs, not built into the apiserver). Flag `nginx.ingress.kubernetes.io/*-snippet` annotations (raw NGINX configuration injection); the ingress-nginx project has since been retired.

---

#### Storage

- **[GA in 1.29]** **`ReadWriteOncePod`**: a PVC with this access mode can be used by a single Pod in the whole cluster (CSI drivers only). Verified accepted on 1.29. Use it for single-writer data (databases, leader-only state) instead of `ReadWriteOnce`, which only restricts to one *node* and lets several Pods on that node write concurrently. A second Pod that uses the same claim cannot start (it stays pending).
- **[GA in 1.29]** CSI node-expansion secrets (`csi.storage.k8s.io/node-expand-secret-name` / `-namespace` StorageClass parameters).
- **[NEW in 1.29 — beta, on]** `PersistentVolume.status.lastPhaseTransitionTime` records when the PV last changed phase (populated on the first transition after the upgrade). Use it for "Released for > N days" cleanup reports instead of annotations.
- **VolumeAttributesClass** (`storage.k8s.io/v1alpha1`, PVC `volumeAttributesClassName`) is alpha: the API is not served and the PVC field is dropped on a default 1.29 cluster (verified). Changing IOPS/throughput of a bound volume needs driver-specific tooling on 1.29.
- **No reclaim-policy guarantee when the PV is deleted first** (`HonorPVReclaimPolicy` is alpha on 1.29): deleting a PV object before its PVC can **leak** the backing volume even with `persistentVolumeReclaimPolicy: Delete`. Delete the PVC and let the PV controller delete the PV.
- In-tree cloud volume types (`awsElasticBlockStore`, `gcePersistentDisk`, `azureDisk`, `azureFile`, `cinder`, `vsphereVolume`) are served through CSI migration; new manifests must use CSI `StorageClass`es. In-tree `cephfs` and `rbd` still exist on 1.29 (deprecated; removed in 1.31) — new manifests should use ceph-csi. `CSIMigrationPortworx` is beta but **off** on 1.29.
- A PVC with an explicitly empty `storageClassName: ""` asks for *no* class; since v1.29.2 the PV controller no longer assigns the default StorageClass to it.[^3]
- `RecoverVolumeExpansionFailure` is alpha on 1.29 (users cannot shrink a failed expansion back); plan PVC size increases carefully.
- Go code: build PVC specs with `corev1.VolumeResourceRequirements` (k8s.io/api v0.29) — `corev1.ResourceRequirements` no longer compiles there (verified: `cannot use corev1.ResourceRequirements{…} as "k8s.io/api/core/v1".VolumeResourceRequirements value`).

---

#### Security Hardening

##### Operator Pod Security Context

```yaml
metadata:
  annotations:
    container.apparmor.security.beta.kubernetes.io/manager: runtime/default
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

##### Pod Security Standards

Operator and workload namespaces should enforce **restricted**, pinned to the cluster version so a control-plane upgrade does not silently change the rules:

```yaml
pod-security.kubernetes.io/enforce: restricted
pod-security.kubernetes.io/enforce-version: v1.29
pod-security.kubernetes.io/warn: restricted
pod-security.kubernetes.io/audit: restricted
```

`UserNamespacesPodSecurityStandards` (alpha, new in 1.29) relaxes the `runAsNonRoot`/`runAsUser` checks for Pods with `hostUsers: false`; it is meant only for clusters where every node supports user namespaces and will never be enabled by default.[^3]

##### Secrets Management

- Never embed secrets in CRD `spec` fields; reference them (`secretRef: {name: ...}`).
- Do not log secret values, and do not put them in Event messages or condition messages.
- Use projected service-account tokens with an `audience` and short `expirationSeconds` instead of long-lived token Secrets (no longer auto-created since 1.24, and **cleaned up** on 1.29, see RBAC).
- **[GA in 1.29]** Encryption at rest with **KMS v2** (`apiVersion: v2` providers in the `EncryptionConfiguration`). KMS v1 providers are deprecated and **disabled by default** (`KMSv1=false`) — verified: an `EncryptionConfiguration` with a `kms` provider without `apiVersion: v2` makes kube-apiserver v1.29 **refuse to start** (`KMSv1 is deprecated and will only receive security updates going forward. Use KMSv2 instead.  Set --feature-gates=KMSv1=true to use the deprecated KMSv1 feature.`). With `--encryption-provider-config-automatic-reload` the configuration file is now re-read by polling (about once a minute) instead of inotify, so watch `apiserver_encryption_config_controller_automatic_reload_last_timestamp_seconds` to know when a change took effect.[^1][^3]
- Consider External Secrets Operator or Vault Agent for rotation.

##### Image Security

- Pin images by **digest** in production manifests; enforce it with a webhook policy engine (or VAP where the cluster enables it).
- Use distroless or scratch base images for operator binaries.
- Scan images (Trivy, Grype) in CI; never use `latest`.
- On multi-tenant nodes, enable the `AlwaysPullImages` admission plugin.

##### Known CVEs reviewers should map to 1.29 patch levels

| CVE | Affected 1.29 versions | Fixed in | Note |
|---|---|---|---|
| CVE-2024-3177 (mountable-secrets policy bypass via `envFrom`) | kube-apiserver v1.29.0–v1.29.3 | v1.29.4 | Only relevant with the `kubernetes.io/enforce-mountable-secrets` annotation |
| CVE-2024-10220 (arbitrary command execution through `gitRepo` volumes) | kubelet ≤ v1.29.6 | v1.29.7 | Block `gitRepo` volumes with policy; use an init container that clones |
| CVE-2024-5321 (Windows container log permissions) | kubelet ≤ v1.29.6 | v1.29.7 | Windows nodes only |
| CVE-2024-9042 (Windows nodes: command injection via `nodes/*/logs/query`) | kubelet ≤ v1.29.12 | v1.29.13 | Restrict `nodes/proxy` / log-query access |
| CVE-2025-5187 (node can delete itself via OwnerReference) | all 1.29 releases | **no 1.29 fix** (v1.31.12+) | Enable `OwnerReferencesPermissionEnforcement` or upgrade |
| CVE-2024-7598 (NetworkPolicy bypass while a namespace terminates) | all releases | **no fix** | Delete workloads before deleting the namespace, or add finalizers to NetworkPolicies |
| CVE-2025-1767 (gitRepo volumes can read other Pods' local repositories) | all releases | **no fix** (gitRepo is deprecated) | Forbid `gitRepo` volumes (Pod Security restricted, policy engine) |
| CVE-2025-13281 (kube-controller-manager half-blind SSRF via the in-tree Portworx StorageClass) | all 1.29 releases (`CSIMigrationPortworx` is off on 1.29) | **no 1.29 fix** (v1.32.10+) | Do not let untrusted users create PVCs with an in-tree Portworx StorageClass |
| CVE-2026-2270 (StatefulSet/ControllerRevision write access allows cross-namespace Pod creation) | kube-controller-manager, all 1.29 releases | **no 1.29 fix** (v1.34.12+) | Treat `controllerrevisions` write permission as privileged |
| CVE-2026-76654 (Windows nodes: `subPath` symlink to a UNC path leaks the kubelet's NTLM hash) | Windows kubelets, all 1.29 releases | **no 1.29 fix** (v1.34.12+) | Windows nodes only |
| CVE-2025-0426 (kubelet checkpoint API DoS) | only with the alpha `ContainerCheckpoint` gate enabled | kubelet v1.29.14 adds the fix | Not listed as affected: the checkpoint API is alpha/off on 1.29 |

Infrastructure code that pins `v1.29.x` below v1.29.13 (kind node images, kubeadm `kubernetesVersion`, managed-cluster version strings) is a finding; anything on 1.29 at all is end-of-life and accumulates unfixed CVEs (the rows above marked "no 1.29 fix" were all published after the end of life).[^3][^12][^16][^17]

##### Node-level 1.29 changes reviewers will meet in infrastructure code

- **In-tree cloud providers are off by default**: kube-apiserver, kube-controller-manager and kubelet need `--cloud-provider=external` plus an external cloud-controller-manager (or the `DisableCloudProviders=false` / `DisableKubeletCloudCredentialProviders=false` escape hatch, which disappears in 1.31).
- kubeadm: separate `super-admin.conf` (`system:masters`) vs. `admin.conf` (`kubeadm:cluster-admins` → `cluster-admin`); keep `super-admin.conf` off the node for daily use. kubeadm tolerates kubelets up to three minors older.
- kubelet: `imageMaximumGCAge` (alpha); `/sys/devices/virtual/powercap` is masked in containers from v1.29.7.
- kube-scheduler configuration must use `kubescheduler.config.k8s.io/v1`; the `SelectorSpread` plugin is gone.
- etcd 3.5.10 is the 1.29.0 default (3.5.12 from v1.29.3).[^3]

---

#### API Priority and Fairness (flowcontrol v1)

**[GA in 1.29]** API Priority and Fairness is GA and `flowcontrol.apiserver.k8s.io/v1` is served; `v1beta2` is removed and `v1beta3` is deprecated. Verified on 1.29:

```yaml
apiVersion: flowcontrol.apiserver.k8s.io/v1
kind: PriorityLevelConfiguration
metadata:
  name: batch-low
spec:
  type: Limited
  limited:
    nominalConcurrencyShares: 5      # must be >= 1 on 1.29
    limitResponse:
      type: Queue
      queuing: {queues: 16, handSize: 4, queueLengthLimit: 50}
---
apiVersion: flowcontrol.apiserver.k8s.io/v1
kind: FlowSchema
metadata:
  name: batch-controllers
spec:
  priorityLevelConfiguration: {name: batch-low}
  matchingPrecedence: 2000
  distinguisherMethod: {type: ByUser}
  rules:
  - subjects:
    - kind: ServiceAccount
      serviceAccount: {name: batch-controller, namespace: batch-system}
    resourceRules:
    - verbs: ["*"]
      apiGroups: ["*"]
      resources: ["*"]
      clusterScope: true
      namespaces: ["*"]
```

- **`nominalConcurrencyShares: 0` is rejected in `v1` on 1.29** (verified: `must be positive`); 1.30 accepts it. In `v1beta3` an explicit `0` is silently **defaulted to 30** (verified), and an omitted value defaults to 30 in both versions. Migrating a `v1beta3` manifest that relied on `0 → 30` to `v1` therefore needs the value written out.
- `v1beta3` manifests trigger `Warning: flowcontrol.apiserver.k8s.io/v1beta3 PriorityLevelConfiguration is deprecated in v1.29+, unavailable in v1.32+` (verified) — migrate them now.
- Do not create FlowSchemas that match `system:masters` or the built-in `exempt` priority level for application traffic.

---

#### client-go & apimachinery in v0.29

##### Fake clients

```go
// client-go v0.29: the only typed fake constructor
cs := fake.NewSimpleClientset(existingObjects...)
```

- `fake.NewClientset` (field-managed fake with SSA support) is **new in client-go v0.31** and does not exist in v0.29 (verified). The v0.29 `NewSimpleClientset` does not implement server-side apply semantics.
- controller-runtime's fake client needs `WithStatusSubresource(&MyType{})` when the code writes status. Verified on v0.17: without it, `Status().Update` returns `not found`, while a plain `Update` *does* change `.status` — the opposite of a real apiserver, so tests pass for code that is broken in production.
- The v0.17 fake client rejects server-side apply (`apply patches are not supported in the fake client`, verified) and sets `TypeMeta` only on unstructured objects (v0.17 change).[^11]
- Fakes do not run admission, CEL validation, defaulting or garbage collection. Anything that depends on those belongs in an envtest test.

##### Informers, polling and caches

- `cache.WaitForNamedCacheSync(name, stopCh, synced...)` is the API in v0.29 (`WaitForNamedCacheSyncWithContext` does not exist yet).
- The streaming-list client (`WatchList`) is alpha: client-go v0.29 only enables it with `ENABLE_CLIENT_GO_WATCH_LIST_ALPHA`, and the server-side `WatchList` gate is off.
- `ConsistentListFromCache` is alpha/off on 1.29: a LIST without `resourceVersion` is served from etcd (consistent, more expensive); `resourceVersion="0"` is served from the watch cache (possibly stale). Do not add `resourceVersion="0"` to hot paths of code that needs fresh data.
- `wait.Poll`, `wait.PollImmediate`, `wait.PollInfinite` … are deprecated; use `wait.PollUntilContextTimeout(ctx, interval, timeout, immediate, condition)`. Since 1.29, `immediate=true` guarantees the condition runs once before the first wait.[^3]
- Prefer controller-runtime's cache options (`DefaultTransform` to strip `managedFields`, `ByObject` label/field selectors, `DefaultNamespaces`) to reduce memory instead of hand-rolled informers. v0.17 can also remove informers and set a `DefaultWatchErrorHandler`.

##### Clients and versions

- Keep `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` and controller-runtime on matching minors. A `go.mod` with `k8s.io/client-go v0.29.x` and `k8s.io/api v0.28.x` (or controller-runtime v0.16 with v0.29 libraries) is a review finding.
- Generated clientsets / listers / apply configurations for your CRDs must be regenerated with code-generator v0.29 when the libraries move to v0.29.
- `k8s.io/utils/pointer` is deprecated in favour of `k8s.io/utils/ptr` (`ptr.To(x)`, `ptr.Deref(p, def)`) — both exist in the `k8s.io/utils` version that v0.29 pins.
- `resource.Quantity` gained `Mul(int64) bool` in v0.29.
- `resourceVersion` values are opaque strings — never parse and compare them.
- Kubernetes API types implement gogo `ProtoMessage()`; do not pass them to `google.golang.org/protobuf` (`proto.Marshal`). Use `k8s.io/apimachinery/pkg/runtime/serializer/protobuf`.

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

controller-runtime already adds `controller`, `namespace`, `name`, `reconcileID` to the context logger; do not repeat them.

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
- Dashboards for 1.29 clusters: the kubelet resource metrics (`container_cpu_usage_seconds_total`, `container_memory_working_set_bytes`, `pod_cpu_usage_seconds_total`, …) are GA; `scrape_error` is renamed `resource_scrape_error`; the scheduler's `pod_scheduling_duration_seconds` is deprecated in favour of `pod_scheduling_sli_duration_seconds`; `apiserver_request_body_sizes` is renamed `apiserver_request_body_size_bytes`.[^3]

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
- `generateName` collisions: on 1.29 the apiserver returns `AlreadyExists` on a generated-name conflict (server-side retry, `RetryGenerateName`, arrives in 1.30/1.31); code that creates many objects with `generateName` must retry on `AlreadyExists`.

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
func TestValidator_RejectsPremiumDowngrade(t *testing.T) {
	v := &RedisClusterValidator{}
	oldRC := &cachev1.RedisCluster{Spec: cachev1.RedisClusterSpec{Tier: cachev1.TierPremium, Replicas: 3}}
	newRC := oldRC.DeepCopy()
	newRC.Spec.Tier = cachev1.TierStandard
	if _, err := v.ValidateUpdate(context.Background(), oldRC, newRC); err == nil {
		t.Fatal("expected downgrade to be rejected")
	}
}

func TestStatusNeedsSubresourceInFake(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = cachev1.AddToScheme(scheme)
	rc := &cachev1.RedisCluster{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "default"}}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rc).
		WithStatusSubresource(&cachev1.RedisCluster{}). // required when the code writes status
		Build()
	rc.Status.ReadyReplicas = 2
	if err := c.Status().Update(context.Background(), rc); err != nil {
		t.Fatal(err)
	}
}
```

A reconciler that applies children with SSA (as above) **cannot** be unit-tested with the v0.17 fake client: the Apply patch fails (verified). Either hide the apply behind an interface and fake that, or test the reconciler with envtest. Logic that does not touch the API (building the apply configuration, computing conditions) belongs in plain unit tests.

##### Integration Tests (envtest) — pin the 1.29 binaries

```go
env := &envtest.Environment{
	CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd")},
	ErrorIfCRDPathMissing: true,
	// BinaryAssetsDirectory or KUBEBUILDER_ASSETS from `setup-envtest use 1.29.x -p path`
}
cfg, err := env.Start()
```

- Pin envtest to the cluster minor you ship to (`setup-envtest use 1.29.x`). `setup-envtest` picks the **newest** Kubernetes version by default — in October 2026 that is 1.37, which accepts manifests, CEL functions and fields (`format()`, `isCIDR()`, `appArmorProfile`, VAP `v1`) that a 1.29 cluster rejects or drops, and which has the `fieldPath` fix that 1.29 lacks.
- envtest has no kube-controller-manager: no garbage collection, no Deployment/ReplicaSet controllers, no namespace deletion, no default ServiceAccount. Tests must fake child status (e.g. update `Deployment.status.readyReplicas`, as the verification of this guide did) and must not expect owner-reference cascades.
- envtest is the place to test CEL rules, webhooks (with `WebhookInstallOptions`) and SSA ownership. To test a VAP on 1.29 the envtest apiserver must be started with the gate and runtime-config (`env.ControlPlane.GetAPIServer().Configure().Append("feature-gates", "ValidatingAdmissionPolicy=true")` and `"runtime-config", "admissionregistration.k8s.io/v1beta1=true"`), mirroring the production cluster.

##### e2e Tests (kind / k3s)

- Use a node image of the target minor (`kindest/node:v1.29.x`).
- Deploy the full operator image; test the complete lifecycle: Create → Reconcile → Update → Delete with finalizer.
- Use `kubectl wait --for=condition=Ready` assertions and `--for=jsonpath='{.status.observedGeneration}'=N` for generation checks. `kubectl wait --for=create` does not exist in kubectl 1.29 (verified: `unrecognized condition: "create"`) — poll with `kubectl get` until the object exists, then wait.

##### Testing Rules

- Test idempotency: run `Reconcile` twice and assert the same outcome.
- Test the deletion path including finalizer removal.
- Test that status is updated correctly on both success and error paths.
- Test that invalid CRs are rejected by CEL rules/webhooks (`apierrors.IsInvalid`).
- Run manifests through `kubectl apply --dry-run=server` against a 1.29 apiserver in CI (envtest binaries or kind) — client-side schema validation does not catch dropped gated fields, Pod Security admission, or CEL compile/cost errors.

---

#### kubectl 1.29 and Manifest Tooling

- `kubectl exec POD CMD` without `--` still works in kubectl 1.29 but prints `kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version` (verified); kubectl 1.31 removes it. Fix scripts now: `kubectl exec POD -- CMD`.
- `kubectl wait --for=create` (1.31), `kubectl logs --all-pods` (1.31) and `kubectl debug --custom` (alpha in kubectl 1.30) do not exist in kubectl 1.29 (verified: `unrecognized condition: "create"`, `unknown flag: --all-pods`, `unknown flag: --custom` even with `KUBECTL_DEBUG_CUSTOM_PROFILE=true`). `kubectl debug --profile` accepts `legacy` (default), `general`, `baseline`, `netadmin`, `restricted`; prefer `general`/`baseline`/`restricted` over the legacy default.
- `kubectl auth whoami` (SelfSubjectReview, GA in 1.28) works (verified).
- `kubectl delete --interactive` (alpha flag, visible since 1.29) asks for confirmation.
- `kubectl wait --for=jsonpath=...` accepts simple filter expressions since 1.29, for example `--for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True` (verified).
- `-o kyaml` and `.kuberc` preferences do not exist in kubectl 1.29.
- YAML coercion is a real manifest bug, not a style issue. Verified with kubectl 1.29: `metadata: {name: n}` fails with `json: cannot unmarshal bool into Go struct field ObjectMeta.metadata.name of type string`; a ConfigMap value `enabled: on` fails with `cannot unmarshal bool into Go struct field ConfigMap.data of type string`; `version: 1.10` fails with `cannot unmarshal number`; an `env` entry `value: y` fails with `cannot unmarshal bool into Go struct field EnvVar.spec.containers.env.value of type string`. Quote values such as `y`, `n`, `yes`, `no`, `on`, `off`, `true`, `false`, version strings (`"1.10"`), and octal-looking numbers in labels, annotations, env values and ConfigMap data.
- `kubectl explain` shows alpha fields that the server will drop; use `--dry-run=server -o yaml` to prove a field survives.
- Helm: declare `kubeVersion` in `Chart.yaml` when a chart depends on 1.29 behaviour, and gate version-specific fields with `semverCompare` on `.Capabilities.KubeVersion.Version`. `helm template` without `--kube-version` renders for a built-in default (v1.37.0 for Helm v3.22.1 and v4.3.1), not your cluster — always pass `--kube-version 1.29.x` in CI renders. Verified with Helm v3.22.1 and v4.3.1: `kubeVersion: ">=1.29.0-0"` rejects `--kube-version 1.28.15` (`chart requires kubeVersion: >=1.29.0-0 which is incompatible with Kubernetes v1.28.15`) and accepts `v1.29.15-eks-abc`; the template below renders the annotation for `--kube-version 1.29.15` and the field without `--kube-version`.

```yaml
# templates/pod.yaml — AppArmor via annotation on 1.29, via field on >= 1.30
apiVersion: v1
kind: Pod
metadata:
  name: demo
  {{- if semverCompare "<1.30.0-0" .Capabilities.KubeVersion.Version }}
  annotations:
    container.apparmor.security.beta.kubernetes.io/app: runtime/default
  {{- end }}
spec:
  securityContext:
    runAsNonRoot: true
    seccompProfile: {type: RuntimeDefault}
    {{- if semverCompare ">=1.30.0-0" .Capabilities.KubeVersion.Version }}
    appArmorProfile: {type: RuntimeDefault}
    {{- end }}
  containers:
  - name: app
    image: registry.example.com/app:1.0
```

---

#### Removed / Changed in 1.29 — Migration Notes

| Change | Who is affected | Action |
|---|---|---|
| `flowcontrol.apiserver.k8s.io/v1beta2` removed | FlowSchema / PriorityLevelConfiguration manifests, client code | Move to `v1` (write `nominalConcurrencyShares` ≥ 1 explicitly) before upgrading |
| In-tree cloud providers disabled by default | Clusters using `--cloud-provider=aws\|gce\|azure\|vsphere\|openstack` | Deploy the provider's cloud-controller-manager and set `--cloud-provider=external` (temporary escape hatch: `DisableCloudProviders=false`) |
| KMS v1 disabled by default | `EncryptionConfiguration` with KMS v1 providers | Migrate to KMS v2 (or set `KMSv1=true` temporarily) |
| CronJob `TZ=`/`CRON_TZ=` rejected | CronJob manifests | Use `spec.timeZone` |
| LB status only for `type: LoadBalancer` | Custom load-balancer controllers | Do not write `status.loadBalancer` for other Service types |
| `kubescheduler.config.k8s.io/v1beta3` and `SelectorSpread` removed | Custom scheduler configuration | Use `v1` and `PodTopologySpread` |
| ClusterCIDR alpha API removed | Experiments with `networking.k8s.io/v1alpha1` ClusterCIDR | Remove the objects before upgrading |
| Legacy SA token clean-up beta-on | Tools reading auto-generated token Secrets | Use TokenRequest / `kubectl create token`; watch for the `kubernetes.io/legacy-token-invalid-since` label |
| Native sidecars beta-on | Pods with `initContainers[].restartPolicy: Always` written for 1.28 alpha | The field now takes effect everywhere; mind serialized termination |
| `JobBackoffLimitPerIndex`, `JobPodReplacementPolicy` beta-on | Indexed Jobs, ML training operators | Fields now take effect; new status fields (`failedIndexes`, `terminating`) appear |
| CRD validation rules GA | CRDs | Prefer CEL over validating webhooks; mind the 1.29 `fieldPath` bug and missing `format()` |
| PSS v1.29 allows more net sysctls | Namespaces pinned to `enforce-version: latest` | Review whether the new sysctls are acceptable; pin the version |
| kubeadm `super-admin.conf` | Automation using `admin.conf` as `system:masters` | Expect RBAC-bound `kubeadm:cluster-admins`; use `super-admin.conf` only for break-glass |
| `k8s.io/api` v0.29: PVC `Resources` is `VolumeResourceRequirements` | Go code building PVCs | Change the type when bumping to v0.29 |
| controller-runtime v0.17: `admission.Validator`/`Defaulter` deprecated, `NewDiscoveryRESTMapper` removed, fake client `TypeMeta` change | Operators | Move to `CustomValidator`/`CustomDefaulter`; drop custom REST mapper code; do not assert `TypeMeta` on typed objects from the fake |
| Patch-level fixes (v1.29.4 CVE, v1.29.7 CVEs, v1.29.8 sidecar termination, v1.29.10 UDP conntrack, v1.29.13 CVE) | Pinned 1.29 versions | Run ≥ v1.29.13 (ideally v1.29.15) if still on 1.29; plan the upgrade off 1.29 (EOL since 2025-02-28) |

---

#### Production / Review Checklist

##### Version & API availability

- [ ] Manifests use only API versions served by a default 1.29 apiserver (no VAP `v1`/`v1beta1` unless the cluster enables it; no `networking.k8s.io/v1alpha1`, `resource.k8s.io/v1alpha2`, `storage.k8s.io/v1alpha1`; no `flowcontrol.apiserver.k8s.io/v1beta2`)
- [ ] No 1.30+ fields (`appArmorProfile`, `trafficDistribution`, Job `successPolicy`/`managedBy`, `recursiveReadOnly`, CRD `selectableFields`) — they are rejected as unknown fields
- [ ] No reliance on silently-dropped alpha fields (`preStop.sleep`, `hostUsers: false`, `resizePolicy`, `procMount: Unmasked`, affinity `matchLabelKeys`, StatefulSet `maxUnavailable`, `volumeAttributesClassName`, `optionalOldSelf`) unless the cluster config enables the gate
- [ ] `FlowSchema`/`PriorityLevelConfiguration` on `flowcontrol.apiserver.k8s.io/v1` with `nominalConcurrencyShares` ≥ 1
- [ ] `go.mod`: all `k8s.io/*` on the same v0.29.x patch, controller-runtime v0.17.x, built with a supported Go toolchain
- [ ] envtest/kind/Helm renders pinned to 1.29, not "latest"
- [ ] Cluster/infra code: ≥ v1.29.13 if 1.29 is unavoidable; upgrade plan exists (1.29 is EOL since 2025-02-28)

##### CRD Design

- [ ] Kind: CamelCase singular; Resource: lowercase plural; group is a DNS subdomain you own (not `*.k8s.io`); short name does not collide with built-ins
- [ ] `// +kubebuilder:subresource:status`; `status.conditions` (`listType=map`, key `type`) + `status.observedGeneration`
- [ ] All enum fields use string type aliases (not `bool`); no floats in `spec`
- [ ] Every string/array/map reachable from CEL has `maxLength`/`maxItems`/`maxProperties`
- [ ] Numeric fields have `minimum`/`maximum` (`format: int32` is not a range check)
- [ ] CEL uses only functions available on 1.29 (standard macros, regex, URL, strings v1, lists, quantity, optionals); no `format()`, `strings.quote()`, sets, IP/CIDR, `isSemver()`
- [ ] No `string(<number>)` in `messageExpression`; `fieldPath` not combined with other rules on the same node (1.29 bug); no `optionalOldSelf`
- [ ] Tightened rules come with a migration of stored objects (no ratcheting on 1.29)
- [ ] controller-tools v0.14 markers use only `rule`/`message` (or the CRD is patched)
- [ ] At least one printer column with state info; no `selectableFields`
- [ ] `omitempty` + `// +optional` on all optional fields

##### Controller

- [ ] Fetch → `IgnoreNotFound` → deletion/finalizer → converge → status
- [ ] Children applied with SSA (apply configuration → unstructured, or clientset `Apply`) and a constant field owner; never typed structs with `client.Apply`; no `Update` from cached objects
- [ ] Status written through `Status().Patch`; `observedGeneration` set; conditions via `meta.SetStatusCondition`
- [ ] Transient errors returned; permanent errors → condition + Warning Event + `nil`/`TerminalError`; no error + `RequeueAfter`
- [ ] Event recorder from `GetEventRecorderFor` with core `events` RBAC; Events only on transitions; no user text as format string
- [ ] Webhooks use `CustomValidator`/`CustomDefaulter` (not the deprecated `admission.Validator` on the type); type assertions checked; CEL preferred where possible
- [ ] Watches on shared objects use field indexes and restricted caches; `managedFields` stripped by a `DefaultTransform`
- [ ] Metrics served securely (`SecureServing` + `filters.WithAuthenticationAndAuthorization`, not the plain-HTTP `:8080` default); leader election enabled
- [ ] SSA code paths tested with envtest, not the v0.17 fake client

##### Workloads

- [ ] Pod Security `restricted` passes (runAsNonRoot, seccomp `RuntimeDefault`, drop `ALL`, no privilege escalation); `enforce-version` pinned
- [ ] AppArmor via `container.apparmor.security.beta.kubernetes.io/<container>` annotations (container names match); no `appArmorProfile` field
- [ ] Images pinned by digest; private images on shared nodes protected by `AlwaysPullImages`
- [ ] Memory requests/limits set; no in-place resize assumed
- [ ] Probes: startup + readiness + liveness with sane thresholds; no `preStop.sleep` (use an `exec` sleep or application-level drain delay)
- [ ] Sidecars as `initContainers` with `restartPolicy: Always`; nodes run ≥ 1.29 kubelets (≥ v1.29.8 for the termination fix)
- [ ] Jobs: `podFailurePolicy` ignores `DisruptionTarget`; `backoffLimitPerIndex` only with `completionMode: Indexed`; no `successPolicy`/`managedBy`
- [ ] CronJobs use `timeZone`, never `TZ=`/`CRON_TZ=`
- [ ] PDBs set `unhealthyPodEvictionPolicy: AlwaysAllow` unless there is a reason not to
- [ ] Services: no `trafficDistribution` (use `service.kubernetes.io/topology-mode: Auto`); no `externalIPs`; LB status only on `type: LoadBalancer`
- [ ] HPA-managed Deployments do not set `spec.replicas` in applied manifests
- [ ] Single-writer volumes use `ReadWriteOncePod`; PVCs are deleted before PVs; PVs whose data must survive use `Retain`
- [ ] Unquoted YAML scalars that coerce (`n`, `y`, `yes`, `on`, `1.10`) are quoted

##### Security / RBAC

- [ ] No wildcard verbs/resources; namespace-scoped roles where possible
- [ ] No `get` on `pods/exec`/`attach`/`portforward` in "read-only" roles (WebSocket exec is authorized with `get`)
- [ ] Service account tokens not automounted unless needed; no reliance on auto-generated legacy token Secrets
- [ ] Secrets referenced by name — never inlined in CRD spec, logs, Events or conditions
- [ ] Default-deny NetworkPolicy plus explicit allows (including DNS)
- [ ] Admission webhooks exclude their own namespace and `kube-system`; `sideEffects: None`; `timeoutSeconds` ≤ 10; `matchConditions` to skip system users
- [ ] No `gitRepo` volumes (CVE-2024-10220, CVE-2025-1767); encryption at rest via KMS v2

##### Finalizers & Versioning

- [ ] Finalizer name is fully qualified (`domain.com/name`), added before external resources are created, removed only after successful idempotent cleanup
- [ ] New CRD fields optional; no fields removed or renamed in served versions
- [ ] `deprecationWarning` set on deprecated CRD versions; conversion webhook when schemas diverge; valid `caBundle`
- [ ] `storage: true` on exactly one version; stored objects migrated and `status.storedVersions` cleaned before dropping a version

---

[^1]: Kubernetes v1.29: Mandala — release announcement, 2023-12-13 (49 enhancements: 11 stable, 19 beta, 19 alpha). https://kubernetes.io/blog/2023/12/13/kubernetes-v1-29-release/
[^2]: Kubernetes release data (data/releases/eol.yaml: 1.29 end of life 2025-02-28; data/releases/schedule.yaml for 1.35–1.37) and https://dl.k8s.io/release/stable-1.29.txt. https://kubernetes.io/releases/ , https://github.com/kubernetes/website/blob/main/data/releases/eol.yaml
[^3]: CHANGELOG-1.29 (v1.29.0 urgent upgrade notes, API changes, features, deprecations, bug fixes; patch notes v1.29.1–v1.29.15 including CVE-2024-3177, CVE-2024-5321, CVE-2024-9042 and "Kubernetes 1.29 is now built with go 1.23" in v1.29.15). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.29.md , https://github.com/kubernetes/kubernetes/releases/tag/v1.29.15
[^4]: Go module proxy data for k8s.io/client-go v0.29.x (v0.29.0 2023-12-13, v0.29.15 2025-03-12, `go 1.21`), sigs.k8s.io/controller-runtime v0.17.x (v0.17.0 2024-01-16, v0.17.6 2024-08-12, requires k8s.io/* v0.29.2), sigs.k8s.io/controller-tools v0.14.0 (2024-01-08). https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.6 , https://pkg.go.dev/k8s.io/client-go@v0.29.15
[^5]: Kubebuilder scaffold versions (pkg/plugins/golang/v4/scaffolds/init.go at v3.13.0, v3.14.0–v3.15.1, v4.0.0; webhook template at v3.14.0). https://github.com/kubernetes-sigs/kubebuilder/releases
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
[^7]: Deprecated API Migration Guide (flowcontrol.apiserver.k8s.io/v1beta2 removed in v1.29, v1beta3 removed in v1.32). https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^8]: Common Expression Language in Kubernetes (library availability table: quantity library, CEL optional types and cross-type numeric comparisons "1.29+", strings library version 2 "1.30+", IP/CIDR "1.31+", format "1.32+"); CustomResourceDefinition validation rules. https://kubernetes.io/docs/reference/using-api/cel/ , https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules
[^9]: Kubernetes v1.30 (2024-04-17), v1.31 (2024-08-13), v1.32 (2024-12-11), v1.33 (2025-04-23), v1.34 (2025-08-27), v1.35–v1.37 release announcements and CHANGELOGs. https://kubernetes.io/blog/2024/04/17/kubernetes-v1-30-release/ , https://kubernetes.io/blog/2024/08/13/kubernetes-v1-31-release/ , https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG
[^10]: CHANGELOG-1.30: "Fixed bug where providing a FieldPath to a CRD Validation Rule would erroneously affect the reported field path of other unrelated CRD Validation Rules on the same schema" (kubernetes/kubernetes#123475); CHANGELOG-1.28: `reason` and `fieldPath` added to CRD validation rules (#118041). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.30.md
[^11]: controller-runtime v0.17.0 release notes (k8s 1.29 dependencies, `admission.Validator`/`Defaulter` deprecated, DynamicRESTMapper default, fake client TypeMeta change, webhook request size limits, `MatchingFields` with multiple indexes) and v0.17.6 package documentation. https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.17.0 , https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.6
[^12]: Security advisory CVE-2025-5187: Nodes can delete themselves by adding an OwnerReference (affected up to v1.31.11, v1.32.7, v1.33.3). https://discuss.kubernetes.io/t/security-advisory-cve-2025-5187-nodes-can-delete-themselves-by-adding-an-ownerreference/32947
[^13]: Validating Admission Policy (feature state per version; enabling `admissionregistration.k8s.io/v1beta1`). https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
[^14]: KEP-4006: Transition from SPDY to WebSockets; Kubernetes 1.35 CHANGELOG (`AuthorizePodWebsocketUpgradeCreatePermission`). https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/4006-transition-spdy-to-websockets , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md
[^15]: Go 1.25 release notes — container-aware GOMAXPROCS. https://go.dev/doc/go1.25#container-aware-gomaxprocs
[^16]: Security advisories CVE-2024-10220 (gitRepo volume, fixed in kubelet v1.29.7) and CVE-2025-0426 (kubelet checkpoint API; affected v1.30–v1.32 only, hardening backported to kubelet v1.29.14 via kubernetes#130014). https://discuss.kubernetes.io/t/security-advisory-cve-2024-10220-arbitrary-command-execution-through-gitrepo-volume/30571 , https://discuss.kubernetes.io/t/security-advisory-cve-2025-0426-node-denial-of-service-via-kubelet-checkpoint-api/31587
[^17]: Official Kubernetes CVE feed (CVE-2024-7598, CVE-2025-1767, CVE-2025-13281, CVE-2026-2270, CVE-2026-76654 with affected and fixed versions). https://kubernetes.io/docs/reference/issues-security/official-cve-feed/
