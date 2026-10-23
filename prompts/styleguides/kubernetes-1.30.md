### Kubernetes 1.30 — Manifests, Operators & Controllers Review Guide

> **Version**: Kubernetes v1.30.0 "Uwubernetes", released **2024-04-17**.[^1] Final patch release: **v1.30.14** (June 2025; release artifacts on `dl.k8s.io` are dated 2025-06-18 UTC, `https://dl.k8s.io/release/stable-1.30.txt` reports `v1.30.14`).[^2][^3]
> **Support**: 1.30 reached **end of life on 2025-07-15** and receives no security fixes or bug fixes. As of late October 2026 the supported minors are **1.37** (newest, released 2026-08-26, latest patch v1.37.1), **1.36** (released 2026-04-22), **1.35** (EOL 2027-02-28) and **1.34** (maintenance mode, EOL **2026-10-27**).[^2] A cluster on 1.30 is a finding in itself: plan the upgrade path 1.30 → 1.31 → … one minor at a time.
> **Matching libraries**: `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`, `k8s.io/apiextensions-apiserver` **v0.30.x** (final v0.30.14, `go 1.22.0` directive), **controller-runtime v0.18.x** (final v0.18.7, 2025-01-06; built against k8s.io/* v0.30.1, `go 1.22.0`), **controller-tools v0.15.0** (the only v0.15 release), Kubebuilder **v4.0.0–v4.1.1** (scaffolds controller-runtime v0.18.2/v0.18.4, controller-tools v0.15.0, envtest 1.30.0). Go **1.22** is the minimum toolchain for these modules.[^4][^5]
> **Scope**: Kubernetes manifests (plain YAML, Kustomize output, Helm-rendered YAML), CustomResourceDefinitions with CEL validation, admission policies, and Go operators/controllers built with controller-runtime, client-go or Kubebuilder that target a 1.30 cluster.
> **Note**: This document is self-contained. House rules for operator design, CRDs, status, finalizers, RBAC, webhooks, observability and testing are carried forward; sections marked **[NEW in 1.30]** cover behaviour specific to this release. Every manifest, CRD and policy example was applied (or `--dry-run=server`-applied) against a real kube-apiserver **v1.30.3** (envtest build); boundary claims were re-checked on v1.29.5 and v1.31.0 apiservers; Go snippets compile against k8s.io/* v0.30.14 + controller-runtime v0.18.7, CRD YAML was generated with controller-gen v0.15.0, and the controller was exercised with envtest on the 1.30 binaries. Pod *runtime* behaviour (kubelet, scheduler, controllers) is sourced from the release notes, KEPs and documentation, not executed.

---

#### Table of Contents

1. [Version-aware review](#version-aware-review)
2. [What's New in Kubernetes 1.30](#whats-new-in-kubernetes-130)
3. [API versions and feature gates on a 1.30 cluster](#api-versions-and-feature-gates-on-a-130-cluster)
4. [Do not use or recommend (1.31+ features)](#do-not-use-or-recommend-131-features)
5. [Deprecated and removed — what 1.30 code must not rely on](#deprecated-and-removed--what-130-code-must-not-rely-on)
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
20. [Admission: ValidatingAdmissionPolicy and Webhooks](#admission-validatingadmissionpolicy-and-webhooks)
21. [RBAC & Least Privilege](#rbac--least-privilege)
22. [Workload Manifests: Pods](#workload-manifests-pods)
23. [Workload Manifests: Controllers (Deployment, StatefulSet, Job, CronJob, HPA)](#workload-manifests-controllers-deployment-statefulset-job-cronjob-hpa)
24. [Networking: Services & NetworkPolicy](#networking-services--networkpolicy)
25. [Security Hardening](#security-hardening)
26. [client-go & apimachinery in v0.30](#client-go--apimachinery-in-v030)
27. [Observability](#observability)
28. [Error Handling & Requeue Strategy](#error-handling--requeue-strategy)
29. [Testing](#testing)
30. [kubectl 1.30 and Manifest Tooling](#kubectl-130-and-manifest-tooling)
31. [Removed / Changed in 1.30 — Migration Notes](#removed--changed-in-130--migration-notes)
32. [Production / Review Checklist](#production--review-checklist)

---

#### Version-aware review

The target is a **1.30 control plane** and **k8s.io/\* v0.30.x** client libraries with **controller-runtime v0.18.x**. Before claiming that a field, API version or library function is "not available", check it against the pinned version:

- For manifests: the version comes from the cluster (`kubectl version`), the Helm chart's `kubeVersion`, CI's `kind`/`envtest` image, or a Kustomize/Helm render target. A field that exists in the OpenAPI schema is **not** proof that it works: `kubectl explain` on 1.30 lists alpha-gated fields such as `service.spec.trafficDistribution`, `job.spec.successPolicy`, `job.spec.managedBy`, `pod.spec.containers.volumeMounts.recursiveReadOnly`, `pod.spec.containers.resizePolicy`, `pod.spec.hostUsers` and `crd.spec.versions.selectableFields`, but a default 1.30 apiserver **silently drops** all of them on create (verified with `kubectl apply --dry-run=server -o json`). Only a server-side dry run against a 1.30 apiserver with the cluster's real feature gates proves a field is honoured.
- For Go code: the version comes from `go.mod` (`k8s.io/api v0.30.x`, `sigs.k8s.io/controller-runtime v0.18.x`). Verify a symbol with `go doc sigs.k8s.io/controller-runtime/pkg/client Client` in the module before flagging it. Much current controller-runtime documentation describes v0.22+ APIs (`client.Client.Apply`, typed `admission.Validator[T]`, `GetEventRecorder`) that **do not exist** in v0.18 — never suggest them for a v0.18 module.
- Say "not available in 1.30" only when the pinned version proves it (unknown field, `Unsupported value`, dropped field, CEL `undeclared reference`, missing package/function). Say "available but off by default" for alpha features and for beta **APIs** (since 1.24, new beta API versions are not served unless `--runtime-config` enables them).[^6] Some 1.30 *beta* feature gates are also off by default (`UserNamespacesSupport`, `NodeLogQuery`, `WatchListClient`, `StrictCostEnforcementForVAP`) — check the table below.
- A cluster may enable feature gates. If the repository contains cluster configuration (kubeadm `ClusterConfiguration`, kind config, managed-cluster settings) that enables a gate, review against that configuration, not against the defaults below.
- **Patch level matters**: some 1.30 behaviour changed in patch releases — `StrictCostEnforcementForVAP`/`StrictCostEnforcementForWebhooks` gates only exist from v1.30.2; `APIServingWithRoutine` was demoted to alpha/off in v1.30.4; Job `podFailurePolicy` rules stopped serialising `null` for unset `onExitCodes`/`onPodConditions` in v1.30.4; the `OrderedNamespaceDeletion` gate was backported in v1.30.11 (see "Removed / changed in 1.30").[^3]
- **End of life**: because 1.30 receives no fixes, CVEs published after July 2025 are unpatched on 1.30. Review findings about cluster version should recommend an upgrade, not workarounds that keep 1.30 alive.

---

#### What's New in Kubernetes 1.30

Kubernetes 1.30 contains 45 enhancements: 17 stable, 18 beta and 10 alpha.[^1] The tables list every change that a manifest author, CRD author or controller author can observe. "Default" is the state on an unmodified v1.30 control plane; gates were read from `kube-apiserver --help` of the v1.29.5, v1.30.3 and v1.31.0 binaries, and API behaviour was tested on those apiservers.

##### Graduated to stable (GA) in 1.30

| Feature | What it means for manifests / controllers | KEP |
|---|---|---|
| **ValidatingAdmissionPolicy** (CEL for admission control) | `admissionregistration.k8s.io/v1` `ValidatingAdmissionPolicy` / `ValidatingAdmissionPolicyBinding` are served by default (1.29 only had `v1beta1`, off by default) | 3488 |
| **Admission webhook `matchConditions`** | CEL pre-filters on `ValidatingWebhookConfiguration`/`MutatingWebhookConfiguration`; `AdmissionWebhookMatchConditions` locked on (it was already beta-on in 1.28/1.29) | 3716 |
| **Pod scheduling readiness** (`spec.schedulingGates`) | Pods with gates stay `SchedulingGated` until every gate is removed; `PodSchedulingReadiness` locked on | 3521 |
| **`minDomains` in topology spread constraints** | `MinDomainsInPodTopologySpread` locked on | 3022 |
| **HPA container resource metrics** (`type: ContainerResource`) | Scale on one container's CPU/memory instead of the Pod sum (sidecars no longer skew HPA) | 1610 |
| **`status.hostIPs`** on Pods | Dual-stack host IPs (`PodHostIPs` locked on) | 2681 |
| **Reduction of Secret-based ServiceAccount tokens** (`LegacyServiceAccountTokenCleanUp`) | Unused auto-generated legacy token Secrets are labelled invalid and later deleted by the control plane | 2799 |
| **Aggregated discovery** (`AggregatedDiscoveryEndpoint`) | Clients (kubectl, client-go discovery) fetch all group/version/resource data in one request per root instead of one per group version | 3352 |
| Prevent unauthorised volume-mode conversion on snapshot restore | Restoring a VolumeSnapshot into a PVC with a different `volumeMode` needs the `snapshot.storage.kubernetes.io/allow-volume-mode-change` annotation on the VolumeSnapshotContent (external-provisioner ≥ v4.0.0, external-snapshotter ≥ v7.0.0) | 3141 |
| Cloud dual-stack `--node-ip` (`CloudDualStackNodeIPs`), stable LoadBalancer node set (`StableLoadBalancerNodeSet`), robust VolumeManager reconstruction (`NewVolumeManagerReconstruction`), metric cardinality enforcement, `kubectl delete -i` | Node / cloud-provider / CLI changes | 3705, 3458, 3756, 2305, 3895 |
| Go workspaces for `k8s.io/kubernetes` | **code-generator flags changed**; downstream `hack/update-codegen.sh` must follow `kube_codegen.sh` | 4402 |

Two items in the release blog's "graduated to stable" list are **not** GA on a 1.30 apiserver: **AppArmor support** (`AppArmor` and the new `AppArmorFields` gate are both still *beta* (on) in v1.30.3 and only disappear from `--help` — i.e. lock to GA — in 1.31) and **API Server Tracing** (`APIServerTracing` is still listed as beta in v1.30.3). Describe both as beta-on on 1.30.[^1]

##### Beta in 1.30 (with default)

| Feature / gate | Default in 1.30 | Notes |
|---|---|---|
| **AppArmor fields** — `securityContext.appArmorProfile` (`AppArmorFields`) | **on** (new) | Replaces the `container.apparmor.security.beta.kubernetes.io/<container>` annotations, which now return a deprecation warning. 1.29 rejects the field (`strict decoding error: unknown field "spec.securityContext.appArmorProfile"`) |
| **CRD validation ratcheting** (`CRDValidationRatcheting`) | **on** (alpha/off in 1.29) | Unchanged invalid values no longer block unrelated updates; also enables `optionalOldSelf` in CEL rules |
| **`lifecycle.preStop/postStart.sleep`** (`PodLifecycleSleepAction`) | **on** (alpha/off in 1.29) | 1.29 drops the field and then rejects the hook: `preStop: Required value: must specify a handler type`. `seconds` must be ≥ 1 on 1.30 |
| **LoadBalancer `ipMode`** (`LoadBalancerIPMode`) | **on** (alpha/off in 1.29) | `status.loadBalancer.ingress[].ipMode: VIP \| Proxy`, written by cloud controllers; only valid together with `ip` |
| **Structured authentication configuration** (`--authentication-config`, `apiserver.config.k8s.io/v1beta1`) | **on** | Multiple JWT authenticators, dynamic reload, CEL claim mappings |
| **Structured authorization configuration** (`--authorization-config`) | **on** | Multiple webhooks with CEL `matchConditions`, reload on change |
| **Contextual logging** (`ContextualLogging`) | **on** | Component logs carry `WithValues`/`WithName` context |
| Bound ServiceAccount token improvements (`ServiceAccountTokenJTI`, `ServiceAccountTokenPodNodeInfo`, `ServiceAccountTokenNodeBindingValidation`) | **on** | Tokens carry a `jti` and the node name/UID of the Pod |
| kubectl exec/cp/attach over WebSockets (`TranslateStreamCloseWebsocketRequests`, kubectl `KUBECTL_REMOTE_COMMAND_WEBSOCKETS`) | **on** | Affects proxies and RBAC (see RBAC section) |
| kube-proxy draining terminating nodes (`KubeProxyDrainingTerminatingNodes`) | **on** | `externalTrafficPolicy: Cluster` LBs drain nodes being deleted |
| Image GC maximum age (`ImageMaximumGCAge`), container checkpoint API (`ContainerCheckpoint`), drop-in kubelet config dir, node swap (`NodeSwap`, `swapBehavior` default `NoSwap`) | **on** | Node configuration |
| Node log query (`NodeLogQuery`) | **off** (beta) | Needs the gate *and* kubelet `enableSystemLogQuery` |
| User namespaces (`UserNamespacesSupport`, `hostUsers: false`) | **off** (beta, off) | 1.30 **drops** `hostUsers: false` on a default apiserver (verified) |
| `WatchListClient` (client-go streaming lists) | **off** | client-side gate in client-go v0.30 |
| `StrictCostEnforcementForVAP` / `StrictCostEnforcementForWebhooks` | **off** (from v1.30.2) | Strict CEL cost accounting for extended libraries in VAP / webhook match conditions; turn on early |

Earlier betas that stay **on** in 1.30 and are commonly used: native sidecar containers (`SidecarContainers`, since 1.29), Job `backoffLimitPerIndex`, `podReplacementPolicy`, `podFailurePolicy`, elastic Indexed Jobs, `matchLabelKeys`/`nodeAffinityPolicy`/`nodeTaintsPolicy` in topology spread, StatefulSet `ordinals.start` and PVC retention policy, PDB `unhealthyPodEvictionPolicy`, `PodDisruptionConditions`, `PodIndexLabel`, `PodReadyToStartContainersCondition`.

##### Alpha in 1.30 (off by default — treat as unavailable)

Service **`trafficDistribution`** (`ServiceTrafficDistribution`); **Job success policy** `spec.successPolicy` (`JobSuccessPolicy`); **Job `spec.managedBy`** (`JobManagedBy`); **recursive read-only mounts** `volumeMounts[].recursiveReadOnly` (`RecursiveReadOnlyMounts`); **CRD selectable fields** `spec.versions[].selectableFields` (`CustomResourceFieldSelectors`); built-in **StorageVersionMigration** API `storagemigration.k8s.io/v1alpha1` (`StorageVersionMigrator`); `SELinuxMount` for all volumes; relaxed environment-variable names (`RelaxedEnvironmentVariableValidation`); apiserver retry on `generateName` conflicts (`RetryGenerateName`); kubectl port-forward over WebSockets (`PortForwardWebsockets`); the `MutatingAdmissionPolicy` gate exists but 1.30 has **no** MutatingAdmissionPolicy API (verified: with the gate on and `admissionregistration.k8s.io/v1alpha1` enabled, the group serves no such resource). Older alphas still off in 1.30: in-place Pod resize (`InPlacePodVerticalScaling`), `MaxUnavailableStatefulSet`, `VolumeAttributesClass`, `DynamicResourceAllocation` (`resource.k8s.io/v1alpha2`), `MatchLabelKeysInPodAffinity`, `NFTablesProxyMode`, `ConsistentListFromCache`, `HonorPVReclaimPolicy`, `ProcMountType`.[^3]

##### Other 1.30 changes that affect reviews

- **CEL environment moved up one version**: CRD validation rules and admission CEL expressions written on 1.30 may use the CEL features introduced for 1.29 — the newer **string extension functions** `'%d'.format([...])` and `strings.quote()` and the **`sets` library** (`sets.contains`, `sets.equivalent`, `sets.intersects`). Verified: a CRD whose `messageExpression` uses `.format()` is **rejected by 1.29** (`undeclared reference to 'format'`) and accepted by 1.30; the older string helpers (`lowerAscii`, `split`, `join`, `indexOf`, …) already work on 1.29. The IP/CIDR library (`isIP`, `ip()`, `isCIDR`, `cidr()`) was *added* in 1.30 but only becomes usable for new expressions in 1.31 (verified: 1.30 rejects `isIP(...)` with `undeclared reference`, 1.31 accepts it).[^3][^14]
- **`SecurityContextDeny` admission plugin removed** — clusters still listing it in `--enable-admission-plugins` fail to start; use Pod Security Admission.[^1]
- **kubectl**: `kubectl apply --prune-whitelist` removed (use `--prune-allowlist`); exec/cp/attach use WebSockets by default; `kubectl delete -i` (interactive) GA; `kubectl debug --profile=sysadmin`; `kubectl debug --custom=<json-file>` (partial container spec merged into a profile) exists only behind `KUBECTL_DEBUG_CUSTOM_PROFILE=true` in kubectl 1.30 (alpha; on by default from 1.31 — verified in `--help`). The 1.30 **apiserver's** table output adds a `STATUS` column to `kubectl get job` and a `TIMEZONE` column to `kubectl get cronjob` (server-side printing: verified that kubectl 1.30 against a 1.29 apiserver shows neither).[^3]
- **PersistentVolumeClaims with `storageClassName: ""`** are no longer assigned the default StorageClass by the PV controller (1.30.0 bug fix): an explicit empty string means "no class / static binding" and stays that way.[^3]
- **API Priority & Fairness**: `PriorityLevelConfiguration` `nominalConcurrencyShares: 0` is now accepted in `flowcontrol.apiserver.k8s.io/v1` and `v1beta3` (`ZeroLimitedNominalConcurrencyShares` GA).[^3]
- **Structured authentication configuration** (beta) supports up to 64 JWT authenticators, `discoveryURL` and multiple audiences; a CEL username mapping that evaluates to `""` now fails authentication instead of authenticating an empty user. `CSIDriver.spec.fsGroupPolicy` and `podInfoOnMount` became mutable. Setting the alpha `procMount: Unmasked` now requires `hostUsers: false`.[^3]
- **kube-proxy `--nodeport-addresses`** now defaults to the node's primary IP(s) instead of all node IPs — NodePorts are no longer reachable on secondary interfaces unless configured.[^3]
- **kubelet swap**: `memorySwap.swapBehavior` accepts `NoSwap` (the new default) and `LimitedSwap`; `UnlimitedSwap` was dropped; `LimitedSwap` on cgroup v1 makes the kubelet fail.[^3]
- **Pod-bound ServiceAccount tokens** now embed the node name/UID and a `jti`; audit logs record `authentication.kubernetes.io/credential-id`.[^3]
- **ValidatingAdmissionPolicy never sees** `TokenReview`, `SelfSubjectReview`, `LocalSubjectAccessReview` and `SubjectAccessReview` requests (excluded in all versions).[^3]
- **code-generator**: `kube_codegen.sh` flags changed with the move to Go workspaces; generated clients for CRDs must be regenerated with the v0.30 scripts.[^1]
- Kubernetes 1.30.0 was built with Go 1.22; patch releases from v1.30.11 use Go 1.23.[^3]

---

#### API versions and feature gates on a 1.30 cluster

##### API group versions served by a default v1.30 apiserver

Verified with `kubectl api-versions` on a stock v1.30.3 apiserver (identical list on v1.29.5 and v1.31.0):

```text
admissionregistration.k8s.io/v1   apiextensions.k8s.io/v1   apiregistration.k8s.io/v1
apps/v1   authentication.k8s.io/v1   authorization.k8s.io/v1   autoscaling/v1   autoscaling/v2
batch/v1   certificates.k8s.io/v1   coordination.k8s.io/v1   discovery.k8s.io/v1
events.k8s.io/v1   flowcontrol.apiserver.k8s.io/v1   flowcontrol.apiserver.k8s.io/v1beta3
networking.k8s.io/v1   node.k8s.io/v1   policy/v1   rbac.authorization.k8s.io/v1
scheduling.k8s.io/v1   storage.k8s.io/v1   v1
```

The group *list* is identical on 1.29, but the **resources** differ: `admissionregistration.k8s.io/v1` on 1.30 serves `validatingadmissionpolicies` and `validatingadmissionpolicybindings`; on 1.29 it does not (verified: applying a `v1` ValidatingAdmissionPolicyBinding to 1.29 fails with `no matches for kind "ValidatingAdmissionPolicyBinding" in version "admissionregistration.k8s.io/v1"`).

Not served on a default 1.30 cluster (manifests using them fail with `no matches for kind`):

| API | Status in 1.30 | Notes |
|---|---|---|
| `admissionregistration.k8s.io/v1beta1` ValidatingAdmissionPolicy | beta, off | Use `v1` |
| `admissionregistration.k8s.io/v1alpha1` | alpha, off | No MutatingAdmissionPolicy on 1.30 at all |
| `resource.k8s.io/v1alpha2` (DRA with structured parameters) | alpha, off | |
| `storagemigration.k8s.io/v1alpha1` StorageVersionMigration | alpha, off (new built-in API in 1.30) | |
| `storage.k8s.io/v1alpha1` VolumeAttributesClass | alpha, off | |
| `networking.k8s.io/v1alpha1` ServiceCIDR / IPAddress | alpha, off | |
| `certificates.k8s.io/v1alpha1` ClusterTrustBundle | alpha, off | |
| `internal.apiserver.k8s.io/v1alpha1` StorageVersion | alpha, off | |
| `flowcontrol.apiserver.k8s.io/v1beta3` | **served but deprecated** | Removed in 1.32 — use `flowcontrol.apiserver.k8s.io/v1` (GA since 1.29) |

No API version was removed in 1.30. The previous removal was `flowcontrol.apiserver.k8s.io/v1beta2` in 1.29; the next is `flowcontrol.apiserver.k8s.io/v1beta3` in 1.32.[^7]

##### Selected feature gates (kube-apiserver v1.30.3 defaults)

```text
AppArmor=true (BETA)                       AppArmorFields=true (BETA)
CRDValidationRatcheting=true (BETA)        PodLifecycleSleepAction=true (BETA)
SidecarContainers=true (BETA)              LoadBalancerIPMode=true (BETA)
JobBackoffLimitPerIndex=true (BETA)        JobPodReplacementPolicy=true (BETA)
JobPodFailurePolicy=true (BETA)            ElasticIndexedJob=true (BETA)
MatchLabelKeysInPodTopologySpread=true (BETA)  NodeInclusionPolicyInPodTopologySpread=true (BETA)
StatefulSetAutoDeletePVC=true (BETA)       StatefulSetStartOrdinal=true (BETA)
PDBUnhealthyPodEvictionPolicy=true (BETA)  PodDisruptionConditions=true (BETA)
PodIndexLabel=true (BETA)                  NodeSwap=true (BETA)
StructuredAuthenticationConfiguration=true (BETA)  StructuredAuthorizationConfiguration=true (BETA)
TranslateStreamCloseWebsocketRequests=true (BETA)  ContextualLogging=true (BETA)
APIServingWithRoutine=true (BETA)  -> demoted to alpha/off in v1.30.4
UserNamespacesSupport=false (BETA)         NodeLogQuery=false (BETA)
WatchListClient=false (BETA)               SchedulerQueueingHints=false (BETA)
StrictCostEnforcementForVAP=false (BETA)   StrictCostEnforcementForWebhooks=false (BETA)   (v1.30.2+)
ServiceTrafficDistribution=false (ALPHA)   JobSuccessPolicy=false (ALPHA)   JobManagedBy=false (ALPHA)
RecursiveReadOnlyMounts=false (ALPHA)      CustomResourceFieldSelectors=false (ALPHA)
StorageVersionMigrator=false (ALPHA)       MutatingAdmissionPolicy=false (ALPHA)
InPlacePodVerticalScaling=false (ALPHA)    MaxUnavailableStatefulSet=false (ALPHA)
VolumeAttributesClass=false (ALPHA)        DynamicResourceAllocation=false (ALPHA)
RelaxedEnvironmentVariableValidation=false (ALPHA)   RetryGenerateName=false (ALPHA)
MatchLabelKeysInPodAffinity=false (ALPHA)  NFTablesProxyMode=false (ALPHA)
ConsistentListFromCache=false (ALPHA)      PortForwardWebsockets=false (ALPHA)
```

GA gates locked on in 1.30 (no longer in `--help`): `ValidatingAdmissionPolicy`, `AdmissionWebhookMatchConditions`, `PodSchedulingReadiness`, `MinDomainsInPodTopologySpread`, `HPAContainerMetrics`, `PodHostIPs`, `LegacyServiceAccountTokenCleanUp`, `AggregatedDiscoveryEndpoint`, `CloudDualStackNodeIPs`, `StableLoadBalancerNodeSet`, `NewVolumeManagerReconstruction`, `ZeroLimitedNominalConcurrencyShares`.

##### Already-GA or beta-on features from earlier releases that 1.30 code should use

| Feature | Since | Use it for |
|---|---|---|
| Server-side apply | GA 1.22 | Controllers and GitOps with field ownership |
| Indexed Jobs (`completionMode: Indexed`) | GA 1.24 | Static work partitioning |
| Pod Security Admission | GA 1.25 | Namespace-level `restricted`/`baseline` enforcement |
| Ephemeral containers (`kubectl debug`) | GA 1.25 | Debugging distroless images |
| StatefulSet `minReadySeconds` | GA 1.25 | Slow-start replicas |
| CronJob `timeZone` | GA 1.27 | Schedules in local time |
| gRPC probes (`grpc:`) | GA 1.27 | Probing gRPC servers without a helper binary |
| CRD validation rules (`x-kubernetes-validations`, CEL) incl. `messageExpression`, `reason`, `fieldPath` | GA 1.29 | Cross-field and transition validation instead of webhooks |
| `ReadWriteOncePod` access mode | GA 1.29 | Single-writer volumes |
| `flowcontrol.apiserver.k8s.io/v1` | GA 1.29 | API Priority & Fairness objects |
| Native sidecar containers (`initContainers[].restartPolicy: Always`) | beta-on 1.29 | Log shippers, proxies, service-mesh sidecars |
| Job `podFailurePolicy`, `backoffLimitPerIndex`, `podReplacementPolicy` | beta-on (1.26 / 1.29 / 1.29) | Fail fast, per-index retries, avoid overlapping Pods |
| Topology spread `matchLabelKeys`, `nodeAffinityPolicy`, `nodeTaintsPolicy` | beta-on 1.27 / 1.26 | Per-revision spreading, honouring taints |
| StatefulSet `ordinals.start`, `persistentVolumeClaimRetentionPolicy` | beta-on 1.27 | Migrations, volume cleanup |
| PDB `unhealthyPodEvictionPolicy: AlwaysAllow` | beta-on 1.27 | Drains not blocked by crash-looping Pods |

---

#### Do not use or recommend (1.31+ features)

The cluster is 1.30. Never suggest the following as available; they fail validation, are silently dropped, or need a newer apiserver/library. Boundaries for 1.31 were verified on a v1.31.0 apiserver; later items come from the respective release notes.[^8][^9]

**Kubernetes 1.31 (released 2024-08-13):**

- **CEL IP/CIDR library** in CRD rules and admission expressions: `isIP()`, `ip()`, `ip(...).family()`, `isCIDR()`, `cidr()`, `.containsIP()`, `.prefixLength()`. Verified: 1.30 rejects `isIP(self.addr) && ip(self.addr).family() == 4` (`undeclared reference to 'isIP'`); 1.31 accepts it. On 1.30 validate address syntax with the OpenAPI `format: ipv4`/`ipv6` (enforced — verified) and anything semantic (ranges, families) with a webhook.
- **Service `trafficDistribution: PreferClose`** (beta-on in 1.31). 1.30 drops the field (verified); use the `service.kubernetes.io/topology-mode: Auto` annotation (topology-aware routing) on 1.30 instead.
- **CRD `selectableFields`** (beta-on in 1.31, GA 1.32). 1.30 silently drops `spec.versions[].selectableFields` (verified: stored CRD has none), so `kubectl get --field-selector spec.x=y` on custom resources fails on 1.30; 1.29 rejects the field as unknown.
- **`recursiveReadOnly: Enabled|IfPossible`** on volume mounts (beta-on in 1.31; dropped on 1.30 — verified).
- **Job `successPolicy`** (beta-on in 1.31; dropped on 1.30 — verified).
- **AppArmor GA** (fields work on 1.30 as beta-on — usable; just do not call them GA), Job `podFailurePolicy` GA, StatefulSet `ordinals` GA, elastic Indexed Jobs GA, PDB `unhealthyPodEvictionPolicy` GA, `PersistentVolumeLastPhaseTransitionTime` GA (all usable on 1.30 as beta-on).
- `MatchLabelKeysInPodAffinity` (beta-on 1.31; `matchLabelKeys`/`mismatchLabelKeys` in pod (anti-)affinity are alpha/off on 1.30), nftables kube-proxy mode beta, `kubectl port-forward` over WebSockets beta, `ConsistentListFromCache` beta, `RetryGenerateName` beta, VolumeAttributesClass beta (still off), image volumes (`volumes[].image`, alpha 1.31; 1.30 rejects `unknown field "spec.volumes[0].image"`), `supplementalGroupsPolicy` (alpha 1.31; 1.30 rejects `unknown field "spec.securityContext.supplementalGroupsPolicy"` — verified), `kubectl debug --custom` on by default (beta in kubectl 1.31, JSON or YAML), `AuthorizeWithSelectors`, `AnonymousAuthConfigurableEndpoints`.
- kubectl 1.31 **removes** `kubectl exec POD CMD` without `--` (verified: kubectl 1.31 errors `exec [POD] [COMMAND] is not supported anymore`; kubectl 1.30 only prints a deprecation notice).
- Go: `k8s.io/* v0.31` / **controller-runtime v0.19** / **controller-tools v0.16** (`fieldPath=`/`reason=` arguments of `+kubebuilder:validation:XValidation`).

**Kubernetes 1.32 (2024-12-11) and 1.33 (2025-04-23):**

- `admissionregistration.k8s.io/v1alpha1` **MutatingAdmissionPolicy** (alpha 1.32, beta 1.34, GA `v1` 1.36) — on 1.30 mutation needs a mutating webhook.
- Job `managedBy` (beta-on 1.32, GA 1.35); `preStop.sleep.seconds: 0` (`PodLifecycleSleepActionAllowZero`, alpha 1.32, beta-on 1.33; 1.30 rejects `0` with `must be greater than 0` — verified); relaxed environment-variable names (beta-on 1.32; 1.30 enforces `[-._a-zA-Z][-._a-zA-Z0-9]*` — verified); pod-level `spec.resources` (`PodLevelResources`, alpha 1.32; 1.30: `unknown field "spec.resources"` — verified); `flowcontrol v1beta3` removal (1.32).
- Sidecar containers GA, in-place Pod resize (`pods/resize`, beta-on 1.33), user namespaces on by default (1.33), Job `backoffLimitPerIndex` and `successPolicy` GA, CRD validation ratcheting GA, `recursiveReadOnly` GA, `trafficDistribution` GA, nftables GA, `OrderedNamespaceDeletion` beta-on.
- Later CEL libraries such as `isSemver`/`semver()` and the named-format library (`format.dns1123Label().validate(...)`) — both rejected by 1.30 with `undeclared reference` (verified).

**Kubernetes 1.34 and later:** DRA `resource.k8s.io/v1`, `PodLifecycleSleepAction` GA, VolumeAttributesClass GA (`storage.k8s.io/v1`), Job `podReplacementPolicy` GA, `trafficDistribution: PreferSameZone`/`PreferSameNode`, `fileKeyRef` env vars, container `restartPolicyRules`, KYAML (`kubectl get -o kyaml` — verified unsupported in kubectl 1.30), `pods/exec` WebSocket `create` authorization (1.35), StatefulSet `rollingUpdate.maxUnavailable` (beta-on only in v1.35.0–v1.35.3, off again from v1.35.4 and in 1.36, on by default in 1.37), Service names starting with a digit (1.36), MutatingAdmissionPolicy `v1` (1.36), StatefulSet `updateStrategy.type: Recreate` (alpha in 1.37).

**Library features newer than controller-runtime v0.18 — never suggest for a v0.18 module** (verified: each fails to compile against v0.18.7 / client-go v0.30.14):

| Newer API | First in | Error on v0.18 / v0.30 |
|---|---|---|
| `client.Client.Apply(ctx, applyConfiguration, ...)` | controller-runtime v0.22 | `c.Apply undefined (type client.Client has no field or method Apply)` |
| `ctrl.NewWebhookManagedBy(mgr, obj)` + `admission.Validator[T]` | v0.23 | `too many arguments in call to ctrl.NewWebhookManagedBy` |
| `mgr.GetEventRecorder(name)` (events.k8s.io) | v0.23 | `mgr.GetEventRecorder undefined` |
| `reconcile.Result.Priority` (priority work queue) | controller-runtime v0.23 | `unknown field Priority in struct literal` |
| `k8s.io/client-go/kubernetes/fake.NewClientset` | client-go v0.31 | `undefined: clientfake.NewClientset` |
| `cache.WaitForNamedCacheSyncWithContext` | client-go v0.33 | `undefined: toolscache.WaitForNamedCacheSyncWithContext` |
| `resourceversion.CompareResourceVersion` | apimachinery v0.35 | package does not exist |

**Never recommend** upgrading to a specific newer minor *as a code fix* in a review of application manifests; do flag the cluster version (1.30 is end of life) as a separate, high-priority operational note.

---

#### Deprecated and removed — what 1.30 code must not rely on

| Item | Status on 1.30 | Replacement |
|---|---|---|
| AppArmor annotations `container.apparmor.security.beta.kubernetes.io/<c>` | **Deprecated in 1.30** — verified warning: `deprecated since v1.30; use the "appArmorProfile" field instead` | `securityContext.appArmorProfile` (Pod or container) |
| `SecurityContextDeny` admission plugin | **Removed in 1.30** | Pod Security Admission |
| `kubectl apply --prune-whitelist` | **Removed in 1.30** (verified `unknown flag`) | `--prune-allowlist` |
| `kubectl exec POD CMD` (without `--`) | Deprecated (warning on kubectl 1.30); removed in kubectl 1.31 | `kubectl exec POD -- CMD` |
| `flowcontrol.apiserver.k8s.io/v1beta3` | Deprecated; removed in 1.32 | `flowcontrol.apiserver.k8s.io/v1` |
| `admissionregistration.k8s.io/v1beta1` ValidatingAdmissionPolicy | Superseded by `v1`; not served by default | `admissionregistration.k8s.io/v1` |
| kubelet `swapBehavior: UnlimitedSwap` | Removed | `NoSwap` (default) or `LimitedSwap` |
| Auto-generated legacy ServiceAccount token Secrets | Cleaned up when unused (GA) | `TokenRequest` / projected tokens; explicit `kubernetes.io/service-account-token` Secrets only when unavoidable |
| In-tree cloud providers | Disabled by default (`DisableCloudProviders`); removed in 1.31 | External cloud-controller-manager |
| In-tree CephFS and Ceph RBD volume plugins | Deprecated; removed in 1.31 | CSI drivers |
| kubeadm output API `output.kubeadm.k8s.io/v1alpha2` | Deprecated | `v1alpha3` |
| controller-runtime `v1alpha1.ControllerManagerConfiguration` (`ctrl.ConfigFile()`) | **Removed in v0.18** | Plain `ctrl.Options` populated from your own flags/config |
| controller-runtime `*admission.Decoder` | **Changed in v0.18**: `admission.Decoder` is now an interface | `admission.NewDecoder(scheme)` returning `admission.Decoder` |
| `k8s.io/utils/pointer` | Deprecated | `k8s.io/utils/ptr` (`ptr.To`, `ptr.Deref`) |
| `gitRepo` volumes | Deprecated since 1.11, still usable | init container / git-sync |

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
│       └── rediscluster_webhook.go      # admission.CustomValidator / CustomDefaulter
├── config/
│   ├── crd/                             # Generated CRD manifests
│   ├── rbac/                            # Generated RBAC manifests
│   └── default/                         # Kustomize overlay
├── cmd/main.go
├── Dockerfile
└── Makefile                             # make generate, make manifests, make test
```

Kubebuilder v4.0.0 is the release that scaffolds controller-runtime v0.18 (v0.18.2); it also removed the deprecated `go/v3` plugin and ComponentConfig support, so projects still on the `go/v3` layout (`controllers/`, `apis/`) must be migrated by re-scaffolding.[^5]

##### Toolchain versions for a 1.30 target

| Component | Version for 1.30 | Notes |
|---|---|---|
| Go | ≥ 1.22 | `k8s.io/* v0.30` and controller-runtime v0.18 declare `go 1.22.0`; v0.31/v0.19 also declare `go 1.22.0`; v0.32/v0.20 require Go 1.23 |
| `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` | v0.30.x (keep all on the **same** patch; final v0.30.14) | Mixed minors cause compile or runtime scheme errors |
| controller-runtime | v0.18.x (final v0.18.7) | Built against `k8s.io/* v0.30.1`; Go MVS raises it to your v0.30.x patch |
| controller-tools (`controller-gen`) | v0.15.0 | Supports `XValidation` with `rule`, `message`, `messageExpression` only — see CEL section |
| Kubebuilder | v4.0.0–v4.1.1 | v4.2.0 moves to controller-runtime v0.19 / k8s 1.31 |
| envtest binaries | 1.30.x (`setup-envtest use 1.30.x`) | Test against the version you ship to |

```go
// go.mod — all k8s.io modules on one patch release
module example.com/redis-operator

go 1.22.0

require (
	k8s.io/api v0.30.14
	k8s.io/apimachinery v0.30.14
	k8s.io/client-go v0.30.14
	sigs.k8s.io/controller-runtime v0.18.7
)
```

Flag in review:

- `k8s.io/client-go v0.31.x` or controller-runtime v0.19+ in a module that must support a 1.30 cluster is fine on the wire (client/server skew of ±1 minor is supported), but it compiles in 1.31 API fields (`trafficDistribution`, `successPolicy`, …) that a 1.30 server drops. Pin the library minor to the oldest cluster minor you rely on for new fields.
- Mixed k8s.io minors (`k8s.io/api v0.29.x` with `client-go v0.30.x`) or controller-runtime v0.17 with v0.30 libraries — a finding.
- All of these versions are out of support: controller-runtime only maintains recent minors, and k8s.io v0.30.x receives no fixes after v0.30.14. Security-sensitive operators should move with the cluster.

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
| **Qualified Name** (label/annotation key, finalizer, scheduling gate) | label keys, annotation keys, finalizer names, `schedulingGates[].name` | name part: `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` ; optional prefix part: DNS Subdomain followed by `/` | name part: whitespace, `/` (except as prefix separator), `:`, all other punctuation | name part must start AND end with alphanumeric | name ≤ 63; prefix ≤ 253; total ≤ 317 |
| **Label Value** | `metadata.labels` values | `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` (or empty string) | whitespace, `/`, `:`, all other punctuation, unicode | if non-empty: alphanumeric at both ends | 63 |
| **Annotation Value** | `metadata.annotations` values | any valid UTF-8 string (incl. whitespace, JSON, base64) | none per key | n/a | total across all annotations ≤ 256 KiB |
| **Environment variable name** | `env[].name` | `a–z`, `A–Z`, `0–9`, `_`, `-`, `.` (regex `[-._a-zA-Z][-._a-zA-Z0-9]*`) | must not start with a digit; whitespace, `=`, other punctuation | — | no hard limit |
| **C-Identifier** | env var names a shell must read, ConfigMap/Secret keys consumed via `envFrom` | `A–Z`, `a–z`, `0–9`, `_` | `-`, `.`, `/`, whitespace, all other punctuation | must start with `[A-Za-z_]` (not a digit) | no hard limit |
| **ConfigMap / Secret data key** | `data` / `stringData` keys | `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` | `/`, whitespace, `:`, all other punctuation | any alphanumeric | 253 |
| **Path Segment Name** | `metadata.name` for objects exposed in URL paths | DNS Subdomain set MINUS `.` and `..` as full name | same as DNS Subdomain plus the literal names `.` and `..` | as DNS Subdomain | 253 |

> ⚠️ Underscore `_` is **never** valid in a DNS Subdomain or DNS Label name — that rules it out for `metadata.name`, `metadata.namespace`, API group, and Service name. Underscore IS valid in label/annotation keys (name part), label values, and ConfigMap/Secret data keys.

> ⚠️ Uppercase letters are **never** valid in DNS Subdomain or DNS Label names. They ARE valid in label keys (name part), label values, qualified names, and C-identifiers.

> ⚠️ **Service names must start with a letter.** Verified on 1.30: `metadata.name: 1password-connect` is rejected with `a DNS-1035 label must consist of lower case alphanumeric characters or '-', start with an alphabetic character, ...`. (Only 1.36 relaxes this.)

> ⚠️ **Environment variable names on 1.30** are validated against `[-._a-zA-Z][-._a-zA-Z0-9]*` — verified: `1BAD` is rejected (`must not start with a digit`), `my.var-1` is accepted (but invisible to POSIX shells). The relaxed rule (any printable ASCII except `=`) is alpha/off in 1.30.

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
cache.example.com/last-backup-time: "2025-01-01T00:00:00Z"
```

`PodIndexLabel` (beta, on in 1.30) adds `batch.kubernetes.io/job-completion-index` (Indexed Jobs) and `apps.kubernetes.io/pod-index` (StatefulSets) labels to Pods; selectors and tools should treat them as system-owned.[^3]

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

No `+kubebuilder:selectablefield` marker here: CRD field selectors (`selectableFields`) are alpha/off on 1.30 and dropped by the apiserver, and controller-gen v0.15.0 has no such marker. To filter custom resources by a spec field on 1.30, use a **label** (set by the controller or a mutating webhook) or a controller-runtime **field index** on the cache (`mgr.GetFieldIndexer().IndexField`), not `--field-selector`.

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
- **`format: int32` is not a range check.** Verified on 1.30: a field declared `type: integer, format: int32` (no `maximum`) stores `3000000000`. Add explicit `minimum`/`maximum` (`+kubebuilder:validation:Minimum/Maximum`); with `maximum: 99` the same value is rejected (`should be less than or equal to 99`).
- **Quote YAML keys and values that coerce.** Verified: a CRD schema property written as `n:` (unquoted) is stored as the property **`"false"`** — YAML 1.1 turns `n`, `y`, `yes`, `no`, `on`, `off` into booleans even as map keys.

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

CRD validation rules (`x-kubernetes-validations`) are GA since 1.29 and are the first choice for invariants that the OpenAPI schema cannot express.[^14] The CEL features a rule may use depend on the apiserver's *compatibility version* for new expressions, which is one minor behind the server: on 1.30 new rules can use everything introduced up to the 1.29 CEL environment.[^15]

| CEL library / feature | Usable in new CRD rules on 1.29 | on 1.30 | on 1.31 |
|---|---|---|---|
| `self`, `oldSelf`, `has()`, macros (`all`, `exists`, `map`, `filter`), regex `matches`, `url()`/`isURL()`, lists (`isSorted`, `sum`, `indexOf`, …) | yes | yes | yes |
| Optional types (`?.`, `orValue`, `hasValue`), `quantity()`/`isQuantity()` | yes | yes | yes |
| Base string extensions: `lowerAscii`, `upperAscii`, `replace`, `split`, `join`, `substring`, `trim`, `indexOf`, `charAt` | yes (verified) | yes (verified) | yes |
| Newer string extensions: `'…'.format([...])`, `strings.quote()` | **no** (`undeclared reference to 'format'` / `undeclared reference to 'quote'` — verified) | **yes** (verified) | yes |
| `sets.contains`, `sets.equivalent`, `sets.intersects` | **no** (`found no matching overload for 'contains'` — verified) | **yes** (verified) | yes |
| `optionalOldSelf: true` | no (ignored; `oldSelf` stays a plain value, so `oldSelf.value()` fails to compile — verified) | **yes** (beta, with ratcheting — verified) | yes |
| IP/CIDR: `isIP`, `ip()`, `isCIDR`, `cidr()` | no | **no** (verified) | yes (verified) |
| `isSemver`/`semver()`, named formats `format.dns1123Label()` etc. | no | **no** (verified) | no (verified) |

The following was verified against a 1.30 apiserver with a CRD generated by controller-gen v0.15.0 (Go markers) and the hand-written equivalent below:

```go
// RedisClusterSpec is the desired state.
// +kubebuilder:validation:XValidation:rule="self.tier != 'premium' || self.replicas >= 3",messageExpression="'premium tier needs at least 3 replicas, got %d'.format([self.replicas])"
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

**controller-gen v0.15.0 limitation (verified):** the `XValidation` marker only accepts `rule`, `message` and `messageExpression`. Adding `fieldPath=".replicas",reason=FieldValueInvalid` fails generation with `unknown argument "fieldPath"`; those marker arguments arrived in controller-tools v0.16.0.[^11] The apiserver itself accepts `fieldPath` and `reason` on 1.30 (and 1.29). Options: write those rules directly in the CRD YAML (a Kustomize patch on the generated CRD), or run a newer controller-gen *as a build tool* — the generated YAML only uses fields 1.30 understands, but do not let that bump the controller-runtime dependency.

The same rules written directly in a CRD manifest, plus the CEL libraries available on 1.30 (`quantity`, `url`, `isURL`, string `format`):

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

Observed results on 1.30 (all errors are reported together):

```text
The RedisCluster "bad" is invalid:
* spec.maxMemory: Invalid value: "object": maxMemory must be larger than 64Mi
* spec.replicas: Invalid value: "object": premium tier needs at least 3 replicas, got 2
* spec.image: Invalid value: "string": image must be pinned by digest
* spec.endpoints[0].url: Invalid value: "string": url must be an https URL
# update storageClassName fast -> slow, then remove it:
spec.storageClassName: Invalid value: "string": storageClassName is immutable
spec: Invalid value: "object": storageClassName cannot be removed once set
```

On 1.30 the "Invalid value" of a CEL failure is the **type name** (`"object"`, `"string"`), not the offending value — put the value into `messageExpression` when users need it. Without `fieldPath` (the controller-gen v0.15 output), the premium-tier error is reported on `spec`, not `spec.replicas` (verified in envtest).

##### CEL review rules (verified pitfalls)

1. **`string(<int>)` in `messageExpression` blows the cost budget.** On 1.30 the CRD is rejected with `estimated messageExpression cost exceeds budget by factor of more than 100x` for `'got ' + string(self.replicas)` even though `replicas` has `maximum: 99`. Use `'got %d'.format([self.replicas])` — but only on ≥ 1.30: the same CRD is rejected by **1.29** (`undeclared reference to 'format'`). A chart that installs the CRD on both minors must avoid both forms (use a static `message`).
   ```yaml
   # BAD — CRD rejected on 1.30 (cost)
   messageExpression: "'premium tier needs >= 3 replicas, got ' + string(self.replicas)"
   # GOOD on 1.30 (rejected on 1.29)
   messageExpression: "'premium tier needs at least 3 replicas, got %d'.format([self.replicas])"
   ```
2. **No IP/CIDR helpers on 1.30.** `isIP()`, `ip()`, `isCIDR()`, `cidr()` fail with `undeclared reference`. For plain address syntax use the OpenAPI formats instead — verified on 1.30: `type: string, format: ipv4` rejects `999.1.1.1` with `spec.addr in body must be of type ipv4`. Checks such as "inside 10.0.0.0/8" need a webhook (or a bounded regex) until the cluster is on 1.31.
3. **`has()` takes a field selection, not a variable.** `!has(oldSelf)` fails to compile (`invalid argument to has() macro`). For "may be set once" rules use `optionalOldSelf: true` (beta on 1.30) with `oldSelf.hasValue()`/`oldSelf.value()`:
   ```yaml
   zone:
     type: string
     maxLength: 63
     x-kubernetes-validations:
     - rule: "!oldSelf.hasValue() || self == oldSelf.value()"
       optionalOldSelf: true
       message: zone may be set once and never changed
   ```
   Verified on 1.30: setting `zone` on an existing object succeeds, changing it afterwards fails. On **1.29** the same CRD is rejected (`found no matching overload for 'value' applied to 'string.()'`) because `optionalOldSelf` is ignored there.
4. **`self == oldSelf` does not stop removal.** Transition rules only run when both old and new values exist. Verified: without the parent-level rule, a merge patch setting `storageClassName: null` succeeds despite the immutability rule; with `!has(oldSelf.storageClassName) || has(self.storageClassName)` on the parent the removal is rejected.
5. **Unbounded inputs make the CRD uninstallable**, not slow: the cost estimator assumes maximum sizes. Every string/array/map reached by a rule needs `maxLength`/`maxItems`/`maxProperties`. Chained list-producing helpers multiply the estimate: verified on 1.30, `self.addr.lowerAscii().split('.').join('-') != ''` on a string with `maxLength: 64` is rejected (`estimated rule cost exceeds budget by factor of more than 100x`), while `self.addr.split('.').size() < 5` and `self.addr.lowerAscii() != 'x'` are accepted.
6. **Prefer `fieldPath` + `reason`** (`FieldValueInvalid`, `FieldValueForbidden`, `FieldValueRequired`, `FieldValueDuplicate`) in hand-written CRD YAML so clients see the right field and status cause (controller-gen v0.15 cannot emit them — see above).
7. **Ratcheting** (beta, on in 1.30): unchanged invalid values in stored objects do not block unrelated updates. Verified: after adding `maximum: 100` to `size`, an object storing `size: 3000000000` still accepts a patch to another field, while changing `size` to another invalid value is rejected. On 1.29 ratcheting is off and every update of such an object fails. Tightening validation is still a breaking change for clients that create objects.
8. `x-kubernetes-int-or-string` fields are dynamic in CEL; guard with `type(self.x) == string` or use `quantity()` only after a `pattern` restricts the input.

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

`client.MergeFrom` sends a JSON merge patch without a `resourceVersion` precondition. If two writers own different list entries (for example condition types), use `client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})`.

##### observedGeneration

Always set `status.observedGeneration = metadata.generation` after a successful reconcile:

```go
obj.Status.ObservedGeneration = obj.Generation
```

This allows external tools to detect whether the controller has processed the latest spec change. On 1.30 Pods do **not** follow this contract (Pod `status.observedGeneration` tracking only became GA in 1.35); code that waits for a Pod change must watch Pod conditions and container statuses.

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
7. Use a **conversion webhook** when schemas diverge between versions (controller-runtime: implement `conversion.Hub` on the storage version and `conversion.Convertible` on the others; `ctrl.NewWebhookManagedBy(mgr).For(&v1.RedisCluster{}).Complete()` registers it).
8. **Before removing an old version from `spec.versions`**, every stored object must be rewritten to the new storage version and the old version removed from `status.storedVersions`. On 1.30 the built-in `StorageVersionMigration` API is **alpha and not served**; use the out-of-tree kube-storage-version-migrator or a read/replace loop (`kubectl get <plural> -A -o json | kubectl replace -f -`), then remove the old version from `status.storedVersions` with `kubectl patch crd <name> --subresource=status`.

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

##### Reconciler Skeleton (controller-runtime v0.18)

The following reconciler compiles against k8s.io/* v0.30.14 + controller-runtime v0.18.7 and was exercised with envtest on the 1.30 control-plane binaries (CEL rejection, SSA-created Deployment with owner reference and `managedFields[0].manager == "rediscluster-controller"`, status condition transition after faking `readyReplicas`, a core/v1 Event, finalizer-driven deletion).

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
	Recorder record.EventRecorder // mgr.GetEventRecorderFor("rediscluster-controller")
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
- `GenerationChangedPredicate` drops metadata-only updates (labels, annotations, finalizer edits by others). The deletion of an object that carries a finalizer still reaches the reconciler — verified in envtest on 1.30: with this predicate in place, `Delete` triggered the cleanup function and the finalizer was removed. If the controller must react to label or finalizer changes, combine predicates.
- `Named("rediscluster")`: give every controller an explicit, unique name instead of relying on the lower-cased Kind (it labels metrics and logs; controller-runtime v0.19+ rejects duplicate names at startup).

##### Reconcile Return Conventions (v0.18)

| Return | Meaning |
|---|---|
| `ctrl.Result{}, nil` | Success — no requeue until a watched event fires |
| `ctrl.Result{}, err` | Transient error — requeued with per-item exponential backoff (5 ms doubling up to 1000 s by default) |
| `ctrl.Result{RequeueAfter: d}, nil` | Requeue after duration `d` (polling external state) |
| `ctrl.Result{Requeue: true}, nil` | Rate-limited requeue (not immediate). Still valid in v0.18 (it is only deprecated in later releases), but prefer `RequeueAfter` or returning an error |
| `ctrl.Result{}, reconcile.TerminalError(err)` | Logged and counted, **not** retried — for permanent errors |
| `ctrl.Result{RequeueAfter: d}, err` | The error wins; the result is ignored and v0.18 logs `Warning: Reconciler returned both a non-zero result and a non-nil error`. Pick one |

##### Manager Setup (verified to compile on v0.18.7)

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
		DefaultTransform: cache.TransformStripManagedFields(), // new in v0.18
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Secret{}: {Label: labels.SelectorFromSet(labels.Set{"app.kubernetes.io/managed-by": "redis-operator"})},
		},
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

- Serve metrics with `SecureServing: true` and the `filters.WithAuthenticationAndAuthorization` filter (the manager then needs RBAC for `tokenreviews` and `subjectaccessreviews` `create`); a plain-HTTP `:8080` metrics endpoint is a review finding.
- `LeaderElection: true` for any controller that can run with more than one replica.
- `MaxConcurrentReconciles` > 1 is safe only if the reconciler holds no shared mutable state without locking; the queue guarantees a single key is never processed concurrently.
- **[controller-runtime v0.18]**[^10] `ctrl.ConfigFile()` / `v1alpha1.ControllerManagerConfiguration` (ComponentConfig) was **removed**; old `main.go` files that load `--config` through it no longer compile. Map flags to `ctrl.Options` directly.
- `cache.TransformStripManagedFields()` (new in v0.18) drops `managedFields` from cached objects and saves a lot of memory; do not use it if the controller reads `managedFields` from the cache.

---

#### Server-Side Apply from Controllers

Server-side apply (SSA) with a stable field manager is the preferred way for a controller to own child objects: it sends only the fields the controller cares about, merges with fields set by other actors (HPA replicas, mutating webhooks, users), and resolves conflicts per field. **controller-runtime v0.18 has no typed `Apply` method** (`client.Client.Apply` arrives in v0.22); SSA is sent as a patch of type `client.Apply`. The safest way to build the payload is a client-go **apply configuration** converted to unstructured — it contains only the fields you set:

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
	// controller-runtime v0.18: convert the apply configuration and send an apply patch.
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dep)
	if err != nil {
		return err
	}
	return r.Patch(ctx, &unstructured.Unstructured{Object: obj}, client.Apply, fieldOwner, client.ForceOwnership)
}
```

Verified in envtest on 1.30: the Deployment is created with one controller owner reference and `managedFields[0]` = `{manager: rediscluster-controller, operation: Apply}`.

SSA review rules:

- **Always send the complete desired intent** of the fields you own. A field omitted from a later apply is *removed* if this manager was its only owner. Building the apply configuration conditionally ("only set replicas if changed") is a bug.
- **Do not apply typed structs.** Verified in envtest on 1.30: `r.Patch(ctx, &appsv1.Deployment{...}, client.Apply, ...)` without `TypeMeta` fails with `invalid object type: /, Kind=`; with `TypeMeta` set it succeeds, but the field manager now owns `f:strategy`, `f:template.f:metadata.f:creationTimestamp` and `f:resources` — empty structs serialised from Go zero values that the controller never meant to own. Apply configurations (or hand-built `unstructured` with `apiVersion`/`kind`) avoid both problems.
- **One field manager per controller** (`client.FieldOwner`), constant across releases. Changing it orphans ownership and leaves stale fields behind. v0.18.0 adds `client.WithFieldOwner(c, "name")` (also backported to v0.17.4+), which wraps a client so every write (not only applies) uses that manager.
- `client.ForceOwnership` is appropriate for a controller that is the source of truth for those fields; without it, a conflict (another manager set a different value) returns `409 Conflict`. Never force-apply fields that users are expected to tune (e.g. `replicas` when an HPA targets the Deployment) — omit them instead.
- Do not mix SSA and `Update` on the same object from the same controller; `Update` takes ownership of every field (operation `Update`) and later applies fight the old manager entry. Migrating an existing controller from `CreateOrUpdate` to SSA needs a managedFields upgrade (client-go v0.30's `csaupgrade` package, which now also handles subresources).
- **`controllerutil.CreateOrUpdate` / `CreateOrPatch`** remain acceptable on v0.18 for simple children, but their mutate function must set every field it cares about on the fetched object (never replace `obj.Spec` wholesale — that wipes defaulted fields and causes endless updates).
- **The v0.18 fake client does not implement server-side apply** — see Testing.

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

##### Fan-Out Watches (Many-to-One) with a field index

```go
const secretRefIndex = ".spec.secretName"

// assumes RedisClusterSpec has a SecretName string field
if err := mgr.GetFieldIndexer().IndexField(ctx, &cachev1.RedisCluster{}, secretRefIndex,
	func(o client.Object) []string { return []string{o.(*cachev1.RedisCluster).Spec.SecretName} }); err != nil {
	return err
}

return ctrl.NewControllerManagedBy(mgr).
	For(&cachev1.RedisCluster{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
	Owns(&appsv1.Deployment{}).
	Watches(&corev1.Secret{},
		handler.EnqueueRequestsFromMapFunc(r.findCRsForSecret),
		builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
	Complete(r)

func (r *RedisClusterReconciler) findCRsForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	var list cachev1.RedisClusterList
	if err := r.List(ctx, &list, client.InNamespace(obj.GetNamespace()),
		client.MatchingFields{secretRefIndex: obj.GetName()}); err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(list.Items))
	for _, rc := range list.Items {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: rc.Namespace, Name: rc.Name}})
	}
	return reqs
}
```

`findCRsForSecret` must use a **field index** rather than listing every CR on each Secret event. Watching all Secrets cluster-wide also caches every Secret in memory — restrict the cache (`cache.Options.ByObject` with a label selector, as in Manager Setup) or use `WatchesMetadata`. Since CRD `selectableFields` are not available on 1.30, cache field indexes are the only way to "select by spec field" for custom resources.

##### **[NEW in controller-runtime v0.18]** Typed (generic) sources, handlers and predicates

v0.18 made `source.Kind`, event handlers and predicates generic. `source.Kind` now takes the cache, a typed object, a typed handler and typed predicates; code written for v0.15–v0.17 (`source.Kind(cache, obj)` followed by `Watch(src, handler, preds...)`) no longer compiles. The builder methods `For`/`Owns`/`Watches` keep their untyped signatures.

```go
// v0.18: typed source for WatchesRawSource / controller.Watch
src := source.Kind(mgr.GetCache(), &corev1.ConfigMap{},
	handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, cm *corev1.ConfigMap) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: cm.Namespace, Name: cm.Labels["owner"]}}}
	}),
	predicate.TypedFuncs[*corev1.ConfigMap]{
		UpdateFunc: func(e event.TypedUpdateEvent[*corev1.ConfigMap]) bool {
			return !reflect.DeepEqual(e.ObjectOld.Data, e.ObjectNew.Data) // no type assertion needed
		},
	})
// ctrl.NewControllerManagedBy(mgr).For(...).WatchesRawSource(src).Complete(r)
```

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

Verified on 1.30: on a custom resource, `finalizers: [cleanup]` is accepted with `Warning: metadata.finalizers: "cleanup": prefer a domain-qualified finalizer name to avoid accidental conflicts with other finalizer writers`; on built-in types (e.g. a ConfigMap) it is rejected: `name is neither a standard finalizer name nor is it fully qualified`.

##### Rules

- Add the finalizer **before** creating any external resource; remove it only after cleanup succeeded.
- Cleanup logic **must be idempotent** — the reconcile loop may be called multiple times while `DeletionTimestamp` is set.
- Never add a new finalizer after `DeletionTimestamp` is set — only removal is allowed.
- Do not silently swallow errors; failing cleanup should be retried (return the error) and surfaced as a condition + Warning Event.
- Do not create resources inside a namespace that has `DeletionTimestamp` set.
- Removing the finalizer when the object is already gone returns NotFound — wrap with `client.IgnoreNotFound`.
- controller-runtime's `pkg/finalizer` (`finalizer.NewFinalizers()`, `Register`, `Finalize`) is an alternative for controllers with several independent finalizers.

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
| `blockOwnerDeletion: true` requires `update` on the owner's `finalizers` subresource when the `OwnerReferencesPermissionEnforcement` admission plugin is enabled (not in the default plugin set, common in hardened/OpenShift clusters) | Grant `<owner-resource>/finalizers` `update` — Kubebuilder's marker does — or child creates fail |

##### When to Use Finalizers vs. OwnerReferences

| Scenario | Use |
|---|---|
| Child resource in same namespace | `ownerReferences` (automatic GC) |
| Cluster-scoped child of a namespaced owner | `finalizer` on the owner (or use a cluster-scoped owner) |
| External resource (cloud, DNS) | `finalizer` |
| Cross-namespace cleanup needed | `finalizer` |

---

#### Events

controller-runtime v0.18's `mgr.GetEventRecorderFor(name)` returns a `k8s.io/client-go/tools/record.EventRecorder` that writes **core/v1** Events. That is the correct API for v0.18 (the events.k8s.io-based `GetEventRecorder` only exists from v0.23).

```go
// record.EventRecorder: object, type, reason, message format, args
r.Recorder.Eventf(rc, corev1.EventTypeWarning, "ApplyFailed", "apply Deployment: %v", err)
r.Recorder.Eventf(rc, corev1.EventTypeNormal, "Available", "all %d replicas ready", rc.Spec.Replicas)
```

Review points:

- RBAC: `// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch` (core group). Verified in envtest: the Event appears in `v1` `events` with `reason: Available`, `type: Normal`.
- The message is a format string — never pass user-controlled text as the format (`Eventf(obj, t, r, err.Error())` is a format-string bug); use `"%v", err`, or `Event(obj, t, r, msg)` for a literal message.
- Emit Events on transitions (use the `changed` result of `meta.SetStatusCondition`), not on every reconcile.
- If a controller writes `events.k8s.io/v1` Events directly (client-go `tools/events`), `action` is required — verified on 1.30: an Event without it is rejected with `action: Required value`. client-go v0.30 recommends `NewEventBroadcasterAdapterWithContext` over `NewEventBroadcasterAdapter` for contextual logging.[^3]

---

#### Admission: ValidatingAdmissionPolicy and Webhooks

##### Validation Hierarchy (prefer left-to-right)

```
CRD schema + CEL rules  →  ValidatingAdmissionPolicy (CEL, GA 1.30)  →  Validating webhook
(per-CRD, no extra process)  (cluster policy, no extra process)          (extra infra, TLS, HA)
```

Mutation on 1.30: a **mutating webhook** is the only mechanism. There is no MutatingAdmissionPolicy API on 1.30 (it appears as alpha in 1.32).

##### **[NEW in 1.30 — GA]** ValidatingAdmissionPolicy (verified on 1.30)[^12][^16]

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

The same manifests fail on 1.29 (`no matches for kind "ValidatingAdmissionPolicyBinding" in version "admissionregistration.k8s.io/v1"`). A chart that must also install on 1.29 needs `v1beta1` plus an explicitly enabled `ValidatingAdmissionPolicy` gate and runtime-config there — usually it is better to require ≥ 1.30 in `Chart.yaml`.

##### Parameterised policies

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata: {name: max-replicas}
spec:
  failurePolicy: Fail
  paramKind: {apiVersion: v1, kind: ConfigMap}
  matchConstraints:
    resourceRules:
    - {apiGroups: ["apps"], apiVersions: ["v1"], operations: ["CREATE", "UPDATE"], resources: ["deployments"]}
  validations:
  - expression: "!has(object.spec.replicas) || object.spec.replicas <= int(params.data.maxReplicas)"
    messageExpression: "'replicas must be <= ' + params.data.maxReplicas"
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata: {name: max-replicas}
spec:
  policyName: max-replicas
  validationActions: [Deny]
  paramRef:
    name: replica-limits            # no namespace: looked up in the namespace of the request
    parameterNotFoundAction: Deny
---
apiVersion: v1
kind: ConfigMap
metadata: {name: replica-limits, namespace: default}
data: {maxReplicas: "5"}
```

Verified on 1.30: in `default`, `replicas: 10` is denied with `replicas must be <= 5`; in a namespace without the ConfigMap the request is denied with `failed to configure binding: no params found for policy binding with 'Deny' parameterNotFoundAction`; a binding with `paramRef` but no `parameterNotFoundAction` is rejected (`spec.paramRef.parameterNotFoundAction: Required value`).

Review points for policies:

- A policy without a **binding** does nothing; a binding's `validationActions` decides `Deny`/`Warn`/`Audit`. `[Deny, Warn]` is rejected (verified: `must not contain both Deny and Warn`); roll out new policies with `[Warn, Audit]` first.
- Use `?`/`orValue` optional chaining (`object.spec.template.spec.?initContainers.orValue([])`) for optional fields; plain `object.spec.x` on an absent field is a runtime error that triggers `failurePolicy`.
- Use `matchConditions` to exclude system users and controllers, otherwise a broken policy can block kube-system controllers.
- `parameterNotFoundAction: Deny` with a per-namespace `paramRef.name` denies everything in namespaces that lack the parameter object — choose deliberately, or use `Allow`.
- Type checking: the controller-manager writes `status.typeChecking.expressionWarnings` for policies with typos (for example `object.spec.replica`); the apiserver itself accepts such policies. Check policy status in CI against a real cluster.
- `StrictCostEnforcementForVAP` (beta, **off**, from v1.30.2) changes how extended CEL library calls are costed; policies close to the cost limit may start failing when the gate is turned on (it is still beta/off in 1.31 and GA, locked on, in 1.32 — verified in the binaries' `--help`). Keep expressions cheap.
- `string(...)` in **VAP** expressions is fine (runtime cost limit per expression, not the CRD's static estimate).
- VAP never sees `TokenReview`, `SelfSubjectReview`, `SubjectAccessReview` and `LocalSubjectAccessReview` requests in 1.30.

##### Admission webhooks (controller-runtime v0.18)

controller-runtime v0.18 uses the untyped builder `ctrl.NewWebhookManagedBy(mgr).For(obj)` with `admission.CustomValidator` / `admission.CustomDefaulter`, whose methods take `runtime.Object` and return `(admission.Warnings, error)`. This is the correct shape for v0.18 (the generic `admission.Validator[T]` API is v0.23+). Verified to compile:

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

- **Unchecked type assertions** (`rc := obj.(*cachev1.RedisCluster)`) panic the webhook server on an unexpected type — require the `ok` form.
- The tier-downgrade check is a simple transition rule (`self.tier == oldSelf.tier || oldSelf.tier != 'premium'`) and belongs in CEL; keep webhooks for checks that need external state.
- **[controller-runtime v0.18] `admission.Decoder` is an interface.** Raw handlers that stored a `*admission.Decoder` no longer compile:
  ```go
  // BAD on v0.18: cannot use admission.NewDecoder(scheme) (value of interface type admission.Decoder)
  //               as *admission.Decoder value in struct literal
  type Handler struct{ decoder *admission.Decoder }

  // GOOD
  type Handler struct{ Decoder admission.Decoder }

  func NewHandler(scheme *runtime.Scheme) *Handler { return &Handler{Decoder: admission.NewDecoder(scheme)} }

  func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
  	pod := &corev1.Pod{}
  	if err := h.Decoder.Decode(req, pod); err != nil {
  		return admission.Errored(http.StatusBadRequest, err)
  	}
  	return admission.Allowed("")
  }
  ```

##### Webhook configuration rules

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
  timeoutSeconds: 10
  clientConfig:
    service: {name: webhook, namespace: redis-system, path: /validate, port: 443}
  rules:
  - {apiGroups: ["cache.example.com"], apiVersions: ["v1"], operations: ["CREATE", "UPDATE"], resources: ["redisclusters"]}
  namespaceSelector:
    matchExpressions:
    - {key: kubernetes.io/metadata.name, operator: NotIn, values: [kube-system, redis-system]}
  matchConditions:                       # GA in 1.30
  - name: skip-controllers
    expression: "!request.userInfo.username.startsWith('system:serviceaccount:redis-system:')"
  - name: only-premium
    expression: "request.operation == 'DELETE' || object.spec.tier == 'premium'"
```

- Set `failurePolicy: Fail` for critical validation; `Ignore` only if the webhook is non-essential.
- `sideEffects: None` (or `NoneOnDryRun`); webhooks with side effects break `kubectl apply --dry-run=server`.
- `matchPolicy: Equivalent` (the `v1` default) so the webhook fires on all API versions of the same resource.
- Handle `CREATE` **and** `UPDATE` for immutability rules; `DELETE` only for deletion guards.
- `timeoutSeconds` defaults to 10 and may not exceed 30 (verified: `31` → `the timeout value must be between 1 and 30 seconds`); keep webhooks fast.
- Use `matchConditions` (GA in 1.30, up to 64 per webhook) to skip irrelevant requests in-process, and `namespaceSelector`/`objectSelector` to exclude `kube-system` and the webhook's own namespace (a fail-closed webhook that matches its own Pods can deadlock the cluster). A `matchConditions` expression that errors at runtime follows `failurePolicy`.
- TLS certificates **must** be rotated; use cert-manager or a managed CA bundle injection.

---

#### RBAC & Least Privilege

##### Principles

1. **Least privilege** — grant only the verbs/resources explicitly required.
2. Prefer `Role` + `RoleBinding` (namespaced) over `ClusterRole` + `ClusterRoleBinding`.
3. **No wildcard verbs** (`*`) or **wildcard resources** (`*`).
4. **No `cluster-admin`** service accounts.
5. Mount service account tokens only when the workload needs API access (`automountServiceAccountToken: false` as default).
6. Treat `escalate`, `bind`, `impersonate`, `nodes/proxy`, `serviceaccounts/token` `create`, `pods/exec`, `pods/attach` and `pods/portforward` as privileged.

##### **[1.30 security note]** `get` on `pods/exec` is enough for WebSocket exec

On 1.30, kubectl uses WebSockets for `exec`, `attach` and `cp` by default (`KUBECTL_REMOTE_COMMAND_WEBSOCKETS`, beta-on), and the apiserver authorizes a WebSocket upgrade (an HTTP `GET`) on `pods/exec`/`pods/attach`/`pods/portforward` with the verb **`get`**. Verified with a raw WebSocket upgrade request from a user bound only to `verbs: [get]` on `pods` and `pods/exec`:

```text
1.29.5, 1.30.3, 1.31.0:  "pod web does not have a host assigned"   (authorization passed; failed later)
user without the role:   pods "web" is forbidden: User "carol" cannot get resource "pods/exec"
```

Consequences for review:

- A "read-only" role that grants `get` on `pods/*` or on `pods/exec` **grants interactive exec**. Grant read access with explicit resources (`pods`, `pods/log`, `pods/status`), never `pods/*`.
- Roles that are meant to allow exec should grant `create` *and* `get` on `pods/exec` so they keep working with SPDY clients on 1.30 and with WebSocket clients after the 1.35 change (from 1.35, WebSocket upgrades are authorized as `create`).

```yaml
# BAD on 1.30 — "read-only" role that silently allows exec over WebSockets
rules:
- apiGroups: [""]
  resources: ["pods", "pods/*"]
  verbs: ["get", "list", "watch"]
# GOOD — read-only
rules:
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch"]
```

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

// Events: controller-runtime v0.18 recorder writes core/v1 Events
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
metadata:
  name: redis-operator
  namespace: redis-system
spec:
  serviceAccountName: redis-operator
  automountServiceAccountToken: true  # Explicitly opt in at pod level
  containers:
  - name: manager
    image: registry.example.com/redis-operator@sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
```

**[GA in 1.30]** Legacy auto-generated ServiceAccount token Secrets (from clusters older than 1.24) that are not used for a year are labelled `kubernetes.io/legacy-token-invalid-since` and later deleted by the control plane (`LegacyServiceAccountTokenCleanUp`). Tooling that still reads a `<sa>-token-xxxxx` Secret will break; use `kubectl create token`, the TokenRequest API or projected volumes. Creating a `kubernetes.io/service-account-token` Secret by hand is still possible but yields a non-expiring credential — flag it.[^17]

---

#### Workload Manifests: Pods

##### A production Deployment for 1.30 (verified)

Applied to a 1.30 apiserver into a namespace enforcing Pod Security `restricted:v1.30`; a Pod built from the template passes admission.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: payments
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: v1.30
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
        appArmorProfile: {type: RuntimeDefault}     # field: beta (on) in 1.30; unknown field on 1.29
      topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: ScheduleAnyway
        labelSelector:
          matchLabels: {app.kubernetes.io/name: api}
        matchLabelKeys: [pod-template-hash]          # beta (on) since 1.27
      initContainers:
      - name: log-shipper                            # native sidecar: beta (on) since 1.29
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
            sleep: {seconds: 5}                      # PodLifecycleSleepAction: beta (on) in 1.30
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
metadata: {name: api, namespace: payments}
spec:
  minAvailable: 2
  unhealthyPodEvictionPolicy: AlwaysAllow           # beta (on) since 1.27
  selector:
    matchLabels: {app.kubernetes.io/name: api}
```

A bare Pod in the same namespace without a security context is rejected: `violates PodSecurity "restricted:v1.30": allowPrivilegeEscalation != false ..., unrestricted capabilities ..., runAsNonRoot != true ..., seccompProfile ...`.

Review notes on this template:

- `memory` requests = limits avoids OOM surprises for latency-critical services; `cpu` limits are a policy decision (throttling) — flag missing **memory** limits, not missing CPU limits.
- `matchLabelKeys: [pod-template-hash]` spreads each ReplicaSet revision independently. On 1.30 the scheduler evaluates the keys; the apiserver does **not** merge them into `labelSelector` (verified: the stored constraint keeps `matchLabelKeys` and an unchanged `labelSelector`).
- `maxUnavailable: 0` + `maxSurge: 1` with a PDB of `minAvailable: 2` and three replicas keeps two Pods serving during rollouts and node drains.
- Resource changes in the template trigger a rolling update; there is **no in-place resize** on 1.30 (`InPlacePodVerticalScaling` alpha/off; `resizePolicy` is dropped on create — verified).
- A Go binary built with Go < 1.25 does not read the cgroup CPU limit for `GOMAXPROCS`; for such images set `GOMAXPROCS` from `resourceFieldRef: {resource: limits.cpu, divisor: "1"}` (rounded up to whole CPUs) or use `go.uber.org/automaxprocs`.[^18]

##### **[NEW in 1.30 — beta, on]** AppArmor as a `securityContext` field[^13]

```yaml
spec:
  securityContext:
    appArmorProfile: {type: RuntimeDefault}         # Pod default
  containers:
  - name: app
    image: registry.example.com/app:1.0
    securityContext:
      appArmorProfile:                               # container override
        type: Localhost
        localhostProfile: k8s-app-v2                 # profile must be loaded on every node
```

Verified on 1.30:

- The annotation form `container.apparmor.security.beta.kubernetes.io/<container>` still works but returns `Warning: ... deprecated since v1.30; use the "appArmorProfile" field instead`.
- On create the apiserver **syncs** annotation and field: an annotation `localhost/foo` yields `securityContext.appArmorProfile: {type: Localhost, localhostProfile: foo}`, and a field `Unconfined` also produces the annotation (PSA then reports both).
- Annotation and field must agree: annotation `unconfined` + container field `RuntimeDefault` → `apparmor type in annotation and field must match`.
- `type: Localhost` without `localhostProfile` → `localhostProfile: Required value`.
- Pod Security `baseline`/`restricted` forbid `Unconfined` (verified: `forbidden AppArmor profiles (pod and annotation must not set AppArmor profile type to "Unconfined" ...)`).
- 1.29 rejects the field (`strict decoding error: unknown field "spec.securityContext.appArmorProfile"`). A chart that must install on 1.29 and 1.30 keeps the annotation; a 1.30-only chart uses the field.
- AppArmor only applies on Linux nodes with AppArmor enabled; the kubelet rejects a Pod with a `Localhost` profile that is not loaded (runtime behaviour, sourced).[^19]

##### **[GA in 1.30]** Pod scheduling gates

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: trainer
  labels: {app: trainer}
spec:
  schedulingGates:
  - name: example.com/quota-check                    # qualified name
  containers:
  - name: trainer
    image: registry.example.com/trainer:1.4.2
```

- A gated Pod is stored but ignored by the scheduler (`PodScheduled=False`, reason `SchedulingGated`) until **all** gates are removed.[^20] Gates can only be set at creation and then removed — verified: adding one to an existing Pod fails with `only deletion is allowed, but found new scheduling gate`.
- While a Pod is gated, its scheduling directives can be **tightened**: `nodeSelector` entries and `nodeAffinity` terms may be added/narrowed — this is how queue controllers (Kueue) place gated Pods.[^20] Verified: adding a `nodeSelector` to a gated Pod succeeds; after the gates are removed the same change is rejected (`pod updates may not change fields other than ...`).
- Gate names follow qualified-name rules (verified: `bad gate` → `name part must consist of alphanumeric characters, '-', '_' or '.'`).
- The controller that owns the gate must remove it even on its own failure paths; a forgotten gate leaves Pods pending forever with no scheduler event. Workload controllers (Deployment, Job) copy gates from the template to every Pod.
- Removing gates needs `patch`/`update` on `pods`; there is no subresource.

##### **[GA in 1.30]** `minDomains` in topology spread constraints

```yaml
topologySpreadConstraints:
- maxSkew: 1
  minDomains: 3                                      # treat < 3 zones as skew violation
  topologyKey: topology.kubernetes.io/zone
  whenUnsatisfiable: DoNotSchedule                   # required with minDomains
  labelSelector: {matchLabels: {app: web}}
  nodeAffinityPolicy: Honor
  nodeTaintsPolicy: Honor
```

- `minDomains` is only valid with `whenUnsatisfiable: DoNotSchedule` (verified: `can only use minDomains if whenUnsatisfiable=DoNotSchedule, not ScheduleAnyway`).
- With fewer eligible domains than `minDomains`, the global minimum is treated as 0, so Pods become unschedulable once each existing domain holds `maxSkew` Pods — intended to make a cluster autoscaler create nodes in new zones. Without an autoscaler that can add domains it only produces Pending Pods.[^21]

##### Native sidecar containers (beta, on since 1.29)

`initContainers[].restartPolicy: Always` starts the sidecar before the main containers, keeps it running, and stops it after them; Jobs complete when the main containers finish.

- Only init containers may set `restartPolicy` (verified: on a regular container → `may not be set for non-init containers`).
- Sidecars may have probes; ordinary init containers may not (verified: `readinessProbe: Forbidden: may not be set for init containers without restartPolicy=Always`).
- Sidecar resources count towards Pod resource requests like regular containers (plus init-container peaks), which matters for quota.
- v1.30.4 fixed a bug where sidecars could fail to terminate gracefully if the Pod had not finished initialising — run ≥ v1.30.4 for sidecar-heavy workloads.[^3]

##### **[NEW in 1.30 — beta, on]** `sleep` lifecycle handler

```yaml
lifecycle:
  preStop:
    sleep: {seconds: 5}
```

- Gives endpoints/load balancers time to drain without a shell or `sleep` binary in the image (distroless).
- `seconds` must be **≥ 1** and less than `terminationGracePeriodSeconds` (verified: `0` → `must be greater than 0 and less than terminationGracePeriodSeconds (30)`). `seconds: 0` needs 1.33.
- On 1.29 the field is dropped and the hook becomes empty — `preStop: Required value: must specify a handler type` (verified). Charts that also target 1.29 must keep `exec: {command: [sleep, "5"]}`.

##### Other Pod rules

- **`hostUsers: false` is dropped on a default 1.30 apiserver** (user namespaces beta, off — verified). Do not claim a Pod runs in a user namespace unless the cluster enables `UserNamespacesSupport` *and* the runtime supports it (the kubelet rejects the Pod otherwise).[^3]
- `recursiveReadOnly` on volume mounts, `resizePolicy`, `volumes[].image`, and pod-level `resources` are not available (dropped or `unknown field` — verified).
- `status.hostIPs` (GA) lists all host IPs for dual-stack nodes; use it instead of parsing `status.hostIP` when both families matter (the Downward API accepts `fieldRef: {fieldPath: status.hostIPs}` — verified).
- `imagePullPolicy: IfNotPresent` lets any Pod on a node reuse a private image pulled with someone else's credentials on 1.30 (the kubelet check for cached images arrives in later releases). Multi-tenant clusters should use `Always` for private images or the `AlwaysPullImages` admission plugin.
- Probes: always pair `startupProbe` with liveness for slow starters; `terminationGracePeriodSeconds` on a liveness/startup probe overrides the Pod value for probe-triggered kills.
- `emptyDir.sizeLimit` and `medium: Memory` (counts against the container memory limit).

---

#### Workload Manifests: Controllers (Deployment, StatefulSet, Job, CronJob, HPA)

##### Deployments and ReplicaSets

- `spec.selector` is immutable; changing labels in the selector requires a new Deployment.
- `progressDeadlineSeconds` only marks the `Progressing` condition `False`; it does not roll back.
- Do not set `spec.replicas` in a Deployment managed by an HPA when applying with SSA or `kubectl apply` — every apply resets the replica count. Omit the field.

##### StatefulSets

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata: {name: kv, namespace: default}
spec:
  serviceName: kv
  replicas: 5
  ordinals: {start: 10}                              # beta (on); Pods kv-10 … kv-14
  persistentVolumeClaimRetentionPolicy: {whenDeleted: Retain, whenScaled: Delete}   # beta (on)
  podManagementPolicy: Parallel
  selector: {matchLabels: {app: kv}}
  updateStrategy:
    type: RollingUpdate
  template:
    metadata: {labels: {app: kv}}
    spec:
      containers:
      - {name: kv, image: registry.example.com/kv:3.1}
```

- **`rollingUpdate.maxUnavailable` is alpha/off on 1.30** and silently dropped (verified: the stored object has only `partition: 0`). StatefulSet rolling updates replace one Pod at a time; do not rely on `maxUnavailable` to speed them up.
- `persistentVolumeClaimRetentionPolicy`: `whenScaled: Delete` destroys data when scaling down — only for disposable data. v1.30.4 made auto-deletion respect other controlling owners of the PVC.[^3]
- `serviceName` must reference an existing headless Service for stable DNS.

##### Jobs

```yaml
apiVersion: batch/v1
kind: Job
metadata: {name: shard-train, namespace: default}
spec:
  completionMode: Indexed
  completions: 8
  parallelism: 8
  backoffLimitPerIndex: 2                            # beta (on) since 1.29
  maxFailedIndexes: 2
  podReplacementPolicy: Failed                       # beta (on) since 1.29
  podFailurePolicy:                                  # beta (on) since 1.26
    rules:
    - action: FailIndex
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

Verified on 1.30: accepted as written.

- **Do not use `successPolicy` or `managedBy` on 1.30** — both are alpha and dropped (verified). A manifest written for 1.31+ that relies on `successPolicy` (e.g. "leader index 0 succeeded → Job done") silently loses that behaviour on 1.30: the Job then waits for all completions.
- `podFailurePolicy` requires `restartPolicy: Never`; `FailIndex` requires `backoffLimitPerIndex`; `backoffLimitPerIndex` requires `completionMode: Indexed`.
- On v1.30.0–v1.30.3 the stored `podFailurePolicy` rules contain `onExitCodes: null` / `onPodConditions: null` (verified on v1.30.3); v1.30.4 added `omitempty`. GitOps diffs that compare stored objects may flap across that patch.[^3]
- Elastic Indexed Jobs (beta, on): `completions` may be changed together with `parallelism` (equal values) for Indexed Jobs.
- `ttlSecondsAfterFinished` for cleanup; `activeDeadlineSeconds` caps runtime including retries.

##### CronJobs

```yaml
apiVersion: batch/v1
kind: CronJob
metadata: {name: nightly, namespace: default}
spec:
  schedule: "30 2 * * *"
  timeZone: Europe/Berlin                            # GA since 1.27
  concurrencyPolicy: Forbid
  startingDeadlineSeconds: 600
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers: [{name: c, image: registry.example.com/job:1}]
```

- Always set `timeZone` and `concurrencyPolicy` explicitly; `startingDeadlineSeconds` avoids catch-up storms after controller downtime. `CRON_TZ=`/`TZ=` inside `schedule` is rejected (verified: `cannot use TZ or CRON_TZ in schedule, use timeZone field instead`).
- `kubectl get cronjob` against a 1.30 apiserver shows a `TIMEZONE` column (server-side table output — verified; a 1.29 apiserver does not show it).

##### **[GA in 1.30]** HorizontalPodAutoscaler on container metrics

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: {name: api, namespace: default}
spec:
  scaleTargetRef: {apiVersion: apps/v1, kind: Deployment, name: api}
  minReplicas: 2
  maxReplicas: 20
  metrics:
  - type: ContainerResource
    containerResource: {name: cpu, container: app, target: {type: Utilization, averageUtilization: 70}}
  behavior:
    scaleDown: {stabilizationWindowSeconds: 300}
```

- Prefer `ContainerResource` over `Resource` when Pods have sidecars: a `Resource` metric averages over all containers, so an idle sidecar hides an overloaded application container.
- The named container must exist in every Pod and have a request for the resource; otherwise the HPA cannot compute utilisation.
- `minReplicas: 0` needs `HPAScaleToZero` (alpha, off).
- Per-direction `tolerance` does not exist on 1.30; the global 10 % tolerance applies.

---

#### Networking: Services & NetworkPolicy

##### Service names, IPs and `externalIPs`

- Service names must be RFC 1035 labels (no leading digit) — see Naming.
- Use canonical IP literals. 1.30 warns on `010.000.000.005` (verified: `IP address was accepted, but will be invalid in a future Kubernetes release: ParseAddr("010.000.000.005"): IPv4 field has octet with leading zero`).
- `spec.externalIPs` lets any user who can create Services intercept traffic for arbitrary IPs (CVE-2020-8554). Block it with a ValidatingAdmissionPolicy (`!has(object.spec.externalIPs)`) or the `DenyServiceExternalIPs` admission plugin; new manifests should use `LoadBalancer`, `NodePort` or Gateway API.
- `internalTrafficPolicy: Local` / `externalTrafficPolicy: Local` are hard requirements (no fallback) — flag them where availability matters.

##### Topology-aware routing instead of `trafficDistribution`

`spec.trafficDistribution` is alpha on 1.30 and dropped (verified). For zone-local routing on 1.30 use topology-aware routing:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: zonal-api
  annotations:
    service.kubernetes.io/topology-mode: Auto        # EndpointSlice hints; needs enough endpoints per zone
spec:
  selector: {app: api}
  ports: [{name: http, port: 80, targetPort: 8080}]
```

The EndpointSlice controller only adds zone hints when endpoints are spread proportionally to the zones' CPU capacity; otherwise it falls back to cluster-wide routing. It is a hint, not a guarantee.[^22]

##### **[NEW in 1.30 — beta, on]** LoadBalancer `ipMode`

Cloud controllers may set `status.loadBalancer.ingress[].ipMode`:

- `VIP` (default when unset): kube-proxy programs the LB IP on nodes, so in-cluster traffic to the LB IP short-circuits to the Service.
- `Proxy`: traffic to the LB IP is sent to the load balancer (needed when the LB does TLS termination or PROXY protocol).

Verified on 1.30: `ipMode: Proxy` is stored alongside `ip`; with only `hostname` it is rejected (`ipMode: Forbidden: may not be specified when 'ip' is not set`). On 1.29 the field is dropped. Controllers writing LB status must not set `ipMode` without `ip`.

##### Ingress

Ingress (`networking.k8s.io/v1`) is GA and frozen; new HTTP routing should prefer Gateway API (`gateway.networking.k8s.io/v1`, installed as CRDs, not built into the apiserver). Flag `nginx.ingress.kubernetes.io/*-snippet` annotations, which inject raw NGINX configuration.

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
pod-security.kubernetes.io/enforce-version: v1.30
pod-security.kubernetes.io/warn: restricted
pod-security.kubernetes.io/audit: restricted
```

The `SecurityContextDeny` admission plugin is **removed** in 1.30; Pod Security Admission (or a ValidatingAdmissionPolicy) is the replacement.

##### Secrets Management

- Never embed secrets in CRD `spec` fields; reference them (`secretRef: {name: ...}`).
- Do not log secret values, and do not put them in Event messages or condition messages.
- Use projected service-account tokens with an `audience` and short `expirationSeconds` instead of long-lived token Secrets (no longer auto-created since 1.24; unused legacy ones are cleaned up since 1.30).
- Bound tokens on 1.30 carry the node name/UID and a `jti` (`ServiceAccountTokenPodNodeInfo`, `ServiceAccountTokenJTI`, beta-on); audit logs record the credential ID — useful for incident response.
- Encrypt Secrets at rest with KMS v2 (GA since 1.29) on self-managed control planes.
- Consider External Secrets Operator or Vault Agent for rotation.

##### Image Security

- Pin images by **digest** in production manifests; enforce it with a ValidatingAdmissionPolicy (see above).
- Use distroless or scratch base images for operator binaries.
- Scan images (Trivy, Grype) in CI; never use `latest`.
- Private images on shared nodes: see the `IfNotPresent` note under Pods.

##### Node-level 1.30 changes reviewers will meet in infrastructure code

- kubelet `memorySwap.swapBehavior`: `NoSwap` (default) or `LimitedSwap`; `UnlimitedSwap` is gone; `LimitedSwap` on cgroup v1 fails the kubelet.
- kube-proxy `--nodeport-addresses` default is now the primary node IP(s); NodePorts on secondary NICs need explicit configuration.
- kubelet `podLogsDir` (custom Pod log directory), `containerLogMaxWorkers` and `containerLogMonitorInterval` (concurrent log rotation) are new kubelet configuration fields.[^3]
- `--enable-admission-plugins` must not contain `SecurityContextDeny` (apiserver fails to start).
- `--authentication-config` / `--authorization-config` (structured configuration, beta-on, `apiserver.config.k8s.io/v1beta1`) replace the OIDC flags and the single-webhook authorizer flags; the files are reloaded on change.
- 1.30 patch CVEs (CVE-2024-5321 Windows log permissions in v1.30.3, CVE-2024-9042 Windows log-query command injection in v1.30.9, CVE-2025-0426 kubelet checkpoint API DoS in v1.30.10) — a cluster below v1.30.10 is vulnerable; any 1.30 cluster misses later fixes.[^3]

---

#### client-go & apimachinery in v0.30

##### Informers and caches

- `cache.WaitForNamedCacheSync(name, stopCh, synced...)` is the v0.30 API; the context-based variant does not exist yet.
- **[NEW in client-go v0.30]** Indexers can be added after an informer has started (`AddIndexers` no longer fails on a running informer).[^3]
- **[NEW in client-go v0.30]** client-go has its own feature gates, controlled by environment variables (`KUBE_FEATURE_<Name>=true|false`). `WatchListClient` (streaming list instead of paginated LIST) is beta but **off** in v0.30; leave it off for 1.30 servers unless you have tested it (the server side `WatchList` gate is alpha/off on 1.30).[^3]
- Prefer controller-runtime's cache options (`DefaultTransform: cache.TransformStripManagedFields()`, `ByObject` label/field selectors) to reduce memory instead of hand-rolled informers.
- Contextual logging: client-go v0.30 helpers accept a `context.Context`/`klog.Logger`; prefer `events.NewEventBroadcasterAdapterWithContext` over `NewEventBroadcasterAdapter`.

##### Fake clients

- `k8s.io/client-go/kubernetes/fake.NewSimpleClientset` is the only clientset fake in v0.30 (`NewClientset` with field tracking arrives in v0.31). It does not support server-side apply semantics.
- controller-runtime's fake client needs `WithStatusSubresource(&MyType{})` (since v0.15) when the code writes status; otherwise `Status().Update/Patch` returns NotFound.
- **The controller-runtime v0.18 fake client does not implement server-side apply.** Verified: a reconciler that creates its Deployment with `Patch(..., client.Apply, ...)` gets `deployments.apps "test" not found` from the fake client (an apply patch to a missing object is treated as a patch, not a create); the dry-run path returns `apply patches are not supported in the fake client`. Test SSA-based reconcilers with envtest.
- Fakes do not run admission, CEL validation, defaulting or garbage collection. Anything that depends on those belongs in an envtest test.

##### Server-side apply helpers

- client-go v0.30 adds `csaupgrade` support for **subresource** fields, so a controller migrating its status writes from `Update` to SSA can also clean up old `Update` managers on `/status`.[^3]
- Generated typed clients offer `Apply(ctx, applyConfiguration, metav1.ApplyOptions{FieldManager: "...", Force: true})` (for example `clientset.AppsV1().Deployments(ns).Apply(...)`) — an alternative to controller-runtime's apply patch in non-controller-runtime code. `FieldManager` is required for apply.

##### Clients and versions

- Keep `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` and controller-runtime on matching minors.
- Generated clientsets / listers / informers / apply configurations for your CRDs must be regenerated with **code-generator v0.30**; `kube_codegen.sh` flags changed in 1.30 with the switch to Go workspaces (`kube::codegen::gen_helpers`, `kube::codegen::gen_client` with `--output-dir`/`--output-pkg`). In v0.30 `generate-groups.sh` is a stub that prints `ERROR: generate-groups.sh has been removed.` and exits 1 — a codegen script still calling it is broken.[^1]
- `k8s.io/utils/pointer` is deprecated in favour of `k8s.io/utils/ptr` (`ptr.To(x)`, `ptr.Deref(p, def)`), which is available in the `k8s.io/utils` version pinned by v0.30.

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

controller-runtime already adds `controller`, `controllerGroup`, `controllerKind`, the object key and `reconcileID` to the context logger; do not repeat them.

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
// set Stalled/Ready=False with a clear reason, emit a Warning Event, do NOT retry
return ctrl.Result{}, reconcile.TerminalError(err)   // logged and counted, never requeued

// Waiting on an external system with no watch
return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
```

##### Rules

- **Always** return the error from `Reconcile` for transient failures — do not swallow errors.
- **Never** return a non-nil error together with `RequeueAfter`/`Requeue` — the error wins and controller-runtime logs a warning; choose one.
- Prefer `RequeueAfter` or returning an error over `Requeue: true`.
- Use `client.IgnoreNotFound(err)` when fetching objects that may not exist; on `apierrors.IsConflict(err)` return the error (requeue) rather than retry loops inside Reconcile.
- The default per-item backoff starts at 5 ms and caps at 1000 s (~16 minutes) — rely on it rather than fighting it with custom requeue timing.
- A status write that fails after a successful external action must not repeat the external action non-idempotently on the next attempt.

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

##### Unit Tests (fake client) — not for SSA code on v0.18

```go
func TestReconcile_FakeClient(t *testing.T) {
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

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test", Namespace: "default"}})
	// With the SSA-based applyDeployment above, v0.18's fake client returns
	// `deployments.apps "test" not found` here (verified) — test that path with envtest.
	_ = err
}
```

Use the fake client for reconcilers that create children with `Create`/`CreateOrUpdate`/merge patches, for pure functions (building apply configurations, computing conditions), and for webhook validators called directly. Run anything with SSA, CEL, admission or defaulting in envtest.

##### Integration Tests (envtest) — pin the 1.30 binaries

```go
env := &envtest.Environment{
	CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd")},
	ErrorIfCRDPathMissing: true,
	// BinaryAssetsDirectory or KUBEBUILDER_ASSETS from `setup-envtest use 1.30.x -p path`
}
cfg, err := env.Start()
if err != nil {
	t.Fatal(err)
}
defer env.Stop()

mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme, Metrics: metricsserver.Options{BindAddress: "0"}})
```

- Pin envtest to the cluster minor you ship to (`setup-envtest use 1.30.x`). An unpinned `setup-envtest use` picks the newest available Kubernetes version, which accepts fields (`trafficDistribution`, `selectableFields`, `isIP()` rules) that a 1.30 cluster drops or rejects.
- Disable the metrics listener in tests (`BindAddress: "0"`) so parallel test packages do not fight over ports.
- envtest has no kube-controller-manager: no garbage collection, no Deployment/ReplicaSet controllers, no namespace deletion, no default ServiceAccount, no VAP type-checking status. Tests must fake child status (e.g. update `Deployment.status.readyReplicas`, as the verified test did) and must not expect owner-reference cascades.
- envtest is the place to test CEL rules, ValidatingAdmissionPolicies, webhooks (with `WebhookInstallOptions`) and SSA ownership.

##### e2e Tests (kind / k3s)

- Use a node image of the target minor (`kindest/node:v1.30.x`).
- Deploy the full operator image; test the complete lifecycle: Create → Reconcile → Update → Delete with finalizer.
- Use `kubectl wait --for=condition=Ready` assertions, and `--for=jsonpath='{.status.observedGeneration}'=N` for generation checks on your CRs.

##### Testing Rules

- Test idempotency: run `Reconcile` twice and assert the same outcome.
- Test the deletion path including finalizer removal.
- Test that status is updated correctly on both success and error paths.
- Test that invalid CRs are rejected by CEL rules/webhooks (`apierrors.IsInvalid`).
- Run manifests through `kubectl apply --dry-run=server` against a 1.30 apiserver in CI (envtest binaries or kind) — client-side schema validation does not catch dropped gated fields, Pod Security admission, ValidatingAdmissionPolicies, or CEL cost and compatibility errors.

---

#### kubectl 1.30 and Manifest Tooling

- **WebSockets for exec/attach/cp** are on by default in kubectl 1.30 (`KUBECTL_REMOTE_COMMAND_WEBSOCKETS=false` falls back to SPDY). v1.30.0–v1.30.3 kubectl failed exec/cp/attach through an HTTPS proxy (fixed in v1.30.4), and v1.30.11 fixed a 1.30 regression in WebSocket connection stability — scripts that break behind proxies should use a newer kubectl.[^3]
- `kubectl exec POD CMD` without `--` prints a deprecation notice on 1.30 (verified) and fails from kubectl 1.31; scripts must use `kubectl exec POD -- CMD`.
- `kubectl apply --prune-whitelist` is removed (verified `unknown flag`); use `--prune-allowlist`. Pruning (`--prune`) is still alpha-quality — prefer GitOps tools or `ApplySet` labels deliberately.
- `kubectl delete -i` (interactive confirmation) is GA; `kubectl debug --profile` accepts `legacy` (default), `general`, `baseline`, `netadmin`, `restricted`, `sysadmin` (verified in `--help`). Use `--profile=general` or `restricted`; `legacy` is kept only for compatibility. `kubectl debug --custom` is hidden behind `KUBECTL_DEBUG_CUSTOM_PROFILE=true` (alpha, JSON only) in kubectl 1.30; scripts that pass it without the variable fail.
- `kubectl get -o kyaml` does not exist in kubectl 1.30 (verified).
- `kubectl explain` shows alpha fields that the server will drop; use `--dry-run=server -o yaml` to prove a field survives.
- YAML coercion is a real manifest bug, not a style issue. Verified: an unquoted CRD schema property `n:` is stored as `"false"`, and an env `value: y` fails with `cannot unmarshal bool into Go struct field EnvVar.spec.containers.env.value of type string`. Quote values such as `y`, `n`, `yes`, `no`, `on`, `off`, `true`, `false`, version strings (`"1.10"`), and octal-looking numbers in labels, annotations, env values and ConfigMap data.
- Helm: declare `kubeVersion` in `Chart.yaml` when a chart depends on 1.30 behaviour (ValidatingAdmissionPolicy `v1`, `appArmorProfile`, `sleep` hooks), e.g. `kubeVersion: ">=1.30.0-0"`, and gate optional newer fields with `semverCompare` on `.Capabilities.KubeVersion.Version`. `helm template` without `--kube-version` renders for a built-in default, not your cluster — always pass `--kube-version 1.30.x` in CI renders. Use `.Capabilities.APIVersions.Has "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy"` to detect the policy API rather than comparing versions.

```yaml
# templates/policy.yaml — render the VAP only where the API exists
{{- if .Capabilities.APIVersions.Has "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy" }}
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
# ...
{{- end }}
```

---

#### Removed / Changed in 1.30 — Migration Notes

| Change | Who is affected | Action |
|---|---|---|
| ValidatingAdmissionPolicy GA (`v1`) | Charts shipping `v1beta1`/`v1alpha1` policies | Move to `admissionregistration.k8s.io/v1`; `v1beta1` is not served by default |
| AppArmor annotations deprecated, fields added (beta) | Manifests using `container.apparmor.security.beta.kubernetes.io/*` | Use `securityContext.appArmorProfile` once all clusters are ≥ 1.30 |
| `SecurityContextDeny` admission plugin removed | Clusters enabling it | Remove the flag; use Pod Security Admission |
| `kubectl apply --prune-whitelist` removed | CI scripts | `--prune-allowlist` |
| kubectl exec/cp/attach use WebSockets | Proxies, RBAC | Allow WebSocket upgrades through proxies; review `get` on `pods/exec` |
| Legacy SA token Secret cleanup GA | Tools reading `<sa>-token-*` Secrets | TokenRequest / projected tokens |
| CEL environment: string `format()`, `sets` usable | CRDs/VAPs that must also run on 1.29 | Avoid these functions if 1.29 is still supported |
| CRD validation ratcheting on by default | CRD authors tightening validation | Still treat tightening as breaking for creators; existing objects keep updating |
| `swapBehavior: UnlimitedSwap` removed, `NoSwap` default | kubelet configs | Use `LimitedSwap` (cgroup v2 only) or `NoSwap` |
| kube-proxy `--nodeport-addresses` default = primary IPs | NodePorts on secondary interfaces | Configure `nodePortAddresses` explicitly |
| code-generator / `kube_codegen.sh` flag changes | Projects generating CRD clients | Re-run code generation with the v0.30 scripts |
| controller-runtime v0.18: generic `source.Kind`/handlers/predicates, `admission.Decoder` interface, ComponentConfig removed | Operators upgrading from v0.17 | Update `source.Kind` calls, decoder fields and `main.go` |
| controller-runtime v0.18: `TransformStripManagedFields`, `client.WithFieldOwner`, `controller.Options.NewQueue` | Operators | Optional adoption |
| `StrictCostEnforcementForVAP`/`Webhooks` gates (v1.30.2, off) | Policies near the CEL cost limit | Test with the gates on |
| `APIServingWithRoutine` demoted to alpha/off (v1.30.4) | — | Run ≥ v1.30.4 |
| `OrderedNamespaceDeletion` gate backported (v1.30.11) | Namespace deletion order (Pods before policies) | Only where explicitly enabled; on by default from 1.33 |
| 1.30 end of life (2025-07-15) | Every 1.30 cluster | Upgrade minor by minor to a supported release |

---

#### Production / Review Checklist

##### Version & API availability

- [ ] Manifests use only API versions served by a default 1.30 apiserver (VAP `v1` is fine; no `v1alpha1` StorageVersionMigration, VolumeAttributesClass or DRA without documented cluster opt-in; no MutatingAdmissionPolicy at all)
- [ ] No 1.31+ fields or behaviour assumed (`trafficDistribution`, `successPolicy`, `managedBy`, `recursiveReadOnly`, `selectableFields`, CEL `isIP`/`cidr`, `sleep.seconds: 0`, pod-level `resources`, in-place resize)
- [ ] Gated 1.30 fields checked against the cluster's actual gates (`hostUsers: false` dropped unless `UserNamespacesSupport` is enabled)
- [ ] `go.mod`: all `k8s.io/*` on the same v0.30.x patch, controller-runtime v0.18.x, Go ≥ 1.22; no v0.22+/v0.23 controller-runtime APIs in code
- [ ] envtest/kind pinned to 1.30, not "latest"
- [ ] The cluster's end-of-life status (since 2025-07-15) is called out

##### CRD Design

- [ ] Kind: CamelCase singular; Resource: lowercase plural; group is a DNS subdomain you own (not `*.k8s.io`)
- [ ] `// +kubebuilder:subresource:status`; `status.conditions` (`listType=map`, key `type`) + `status.observedGeneration`
- [ ] All enum fields use string type aliases (not `bool`); no floats in `spec`
- [ ] Every string/array/map reachable from CEL has `maxLength`/`maxItems`/`maxProperties`
- [ ] Numeric fields have `minimum`/`maximum` (`format: int32` is not a range check)
- [ ] CEL uses only the 1.30 environment (no IP/CIDR, semver or named-format functions); `messageExpression` uses `'%d'.format([...])`, not `string(int)` — and neither if 1.29 must still be supported
- [ ] Transition rules handle removal of the field; `optionalOldSelf` only with `oldSelf.hasValue()`
- [ ] `fieldPath`/`reason` set in CRD YAML where useful (controller-gen v0.15 cannot emit them)
- [ ] At least one printer column with state info; no `selectableFields` (dropped on 1.30)
- [ ] `omitempty` + `// +optional` on all optional fields

##### Controller

- [ ] Fetch → `IgnoreNotFound` → deletion/finalizer → converge → status
- [ ] Children applied with SSA (apply configuration → unstructured → `client.Apply` patch) and a constant field owner, or patched — not `Update` from cached objects, not typed structs as apply payloads
- [ ] Status written through `Status().Patch`; `observedGeneration` set; conditions via `meta.SetStatusCondition`
- [ ] Transient errors returned; permanent errors → condition + Warning Event + `reconcile.TerminalError`; no error + `RequeueAfter`
- [ ] Event recorder from `GetEventRecorderFor` with core `events` RBAC; Events only on transitions; no user text as format string
- [ ] Webhooks use `CustomValidator`/`CustomDefaulter` with checked type assertions; `admission.Decoder` stored as an interface; CEL preferred where possible
- [ ] `source.Kind` uses the v0.18 generic signature; watches on shared objects use field indexes and restricted caches
- [ ] Metrics served securely (`SecureServing` + `filters.WithAuthenticationAndAuthorization`); leader election enabled
- [ ] SSA reconcilers tested with envtest (v0.18 fake client cannot apply)

##### Workloads

- [ ] Pod Security `restricted` passes (runAsNonRoot, seccomp `RuntimeDefault`, drop `ALL`, no privilege escalation, no `Unconfined` AppArmor); `enforce-version` pinned
- [ ] AppArmor via `appArmorProfile` field (or annotation only where 1.29 support is required); annotation and field never disagree
- [ ] Images pinned by digest; private images not shared across tenants via `IfNotPresent`
- [ ] Memory requests/limits set; no reliance on in-place resize
- [ ] Probes: startup + readiness + liveness with sane thresholds; `preStop.sleep` (≥ 1 s) instead of shell sleeps where 1.29 support is not needed
- [ ] Sidecars as `initContainers` with `restartPolicy: Always`
- [ ] `schedulingGates` always removed by an owning controller; `minDomains` only with `DoNotSchedule`
- [ ] Jobs: no `successPolicy`/`managedBy`; `podFailurePolicy` with `restartPolicy: Never`; `FailIndex` only with `backoffLimitPerIndex`
- [ ] StatefulSets do not rely on `rollingUpdate.maxUnavailable`
- [ ] HPAs on Pods with sidecars use `ContainerResource`; HPA-managed Deployments do not set `spec.replicas` in applied manifests
- [ ] Services: no `trafficDistribution` (use `topology-mode: Auto`); no `externalIPs`; names start with a letter
- [ ] Unquoted YAML scalars that coerce (`n`, `yes`, `on`, `1.10`) are quoted

##### Security / RBAC

- [ ] No wildcard verbs/resources; namespace-scoped roles where possible
- [ ] No `get` on `pods/*` or `pods/exec` in roles meant to be read-only (1.30 WebSocket exec only needs `get`)
- [ ] Service account tokens not automounted unless needed; no reliance on legacy token Secrets
- [ ] Secrets referenced by name — never inlined in CRD spec, logs, Events or conditions
- [ ] Default-deny NetworkPolicy plus explicit allows (including DNS)
- [ ] Admission webhooks exclude their own namespace and `kube-system`; `sideEffects: None`; timeouts ≤ 10 s; `matchConditions` used to narrow scope
- [ ] ValidatingAdmissionPolicies rolled out with `[Warn, Audit]` before `Deny`; `parameterNotFoundAction` chosen deliberately

##### Finalizers & Versioning

- [ ] Finalizer name is fully qualified (`domain.com/name`), added before external resources are created, removed only after successful idempotent cleanup
- [ ] New CRD fields optional; no fields removed or renamed in served versions
- [ ] `deprecationWarning` set on deprecated CRD versions; conversion webhook when schemas diverge
- [ ] `storage: true` on exactly one version; stored objects migrated (out-of-tree migrator or replace loop) and `status.storedVersions` cleaned before dropping a version

---

[^1]: Kubernetes v1.30: Uwubernetes (UwU) — release announcement, 2024-04-17. https://kubernetes.io/blog/2024/04/17/kubernetes-v1-30-release/
[^2]: Kubernetes releases: 1.30 release page (End of Life 2025-07-15, latest patch 1.30.14) and website data `data/releases/eol.yaml` / `schedule.yaml` (supported releases, 1.35–1.37 dates). https://kubernetes.io/releases/1.30/ , https://kubernetes.io/releases/
[^3]: CHANGELOG-1.30 (v1.30.0 changes by kind; patch notes v1.30.1–v1.30.14 incl. CVE-2024-5321, CVE-2024-9042, CVE-2025-0426). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.30.md
[^4]: Go module proxy data: k8s.io/client-go v0.30.0 (2024-04-17) … v0.30.14 (`go 1.22.0`); sigs.k8s.io/controller-runtime v0.18.0 (2024-04-23) … v0.18.7 (2025-01-06; requires k8s.io/* v0.30.1, `go 1.22.0`); sigs.k8s.io/controller-tools v0.15.0. https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.7 , https://pkg.go.dev/k8s.io/client-go@v0.30.14
[^5]: Kubebuilder v4.0.0 release notes and scaffold versions (`pkg/plugins/golang/v4/scaffolds/init.go`: v4.0.0 → controller-runtime v0.18.2, v4.1.x → v0.18.4, controller-tools v0.15.0, envtest 1.30.0; v4.2.0 → v0.19.0). https://github.com/kubernetes-sigs/kubebuilder/releases/tag/v4.0.0
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
[^7]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^8]: Kubernetes v1.31 release announcement (2024-08-13) and CHANGELOG-1.31. https://kubernetes.io/blog/2024/08/13/kubernetes-v1-31-release/ , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.31.md
[^9]: Kubernetes v1.32 (2024-12-11) and v1.33 (2025-04-23) release announcements. https://kubernetes.io/blog/2024/12/11/kubernetes-v1-32-release/ , https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/
[^10]: controller-runtime v0.18.0 release notes (k8s.io v1.30, ComponentConfig removal, `admission.Decoder` interface, generic source/event/predicate/handler, `TransformStripManagedFields`, `client.WithFieldOwner`, `NewQueue`). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.18.0
[^11]: controller-tools v0.15.0 / v0.16.0 marker definitions (`XValidation` arguments; `fieldPath`/`reason` added in v0.16.0). https://github.com/kubernetes-sigs/controller-tools/releases
[^12]: KEP-3488: CEL for Admission Control; KEP-3716: Admission webhook match conditions. https://github.com/kubernetes/enhancements/issues/3488 , https://github.com/kubernetes/enhancements/issues/3716
[^13]: KEP-24: AppArmor support; Restrict a Container's Access to Resources with AppArmor. https://github.com/kubernetes/enhancements/issues/24 , https://kubernetes.io/docs/tutorials/security/apparmor/
[^14]: Kubernetes documentation: CustomResourceDefinition validation rules (CEL), validation ratcheting. https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
[^15]: Common Expression Language in Kubernetes (libraries and their introduction versions); `k8s.io/apiserver/pkg/cel/environment/base.go` at v0.30.14 (`DefaultCompatibilityVersion` 1.29; IP/CIDR introduced at 1.30). https://kubernetes.io/docs/reference/using-api/cel/
[^16]: Validating Admission Policy documentation. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
[^17]: KEP-2799: Reduction of Secret-based Service Account Tokens; Managing Service Accounts (legacy token cleanup). https://github.com/kubernetes/enhancements/issues/2799 , https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/
[^18]: Go 1.25 release notes — container-aware GOMAXPROCS. https://go.dev/doc/go1.25#container-aware-gomaxprocs
[^19]: AppArmor tutorial (field and annotation semantics, node requirements). https://kubernetes.io/docs/tutorials/security/apparmor/
[^20]: Pod Scheduling Readiness (scheduling gates, mutable scheduling directives). https://kubernetes.io/docs/concepts/scheduling-eviction/pod-scheduling-readiness/
[^21]: Pod Topology Spread Constraints (`minDomains`, `nodeAffinityPolicy`, `nodeTaintsPolicy`, `matchLabelKeys`). https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
[^22]: Topology Aware Routing. https://kubernetes.io/docs/concepts/services-networking/topology-aware-routing/
