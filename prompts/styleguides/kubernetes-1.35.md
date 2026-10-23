### Kubernetes 1.35 — Manifests, Operators & Controllers Review Guide

> **Version**: Kubernetes v1.35.0 "Timbernetes (The World Tree Release)", released **2025-12-17**.[^1] Latest patch at time of writing: **v1.35.9**, published 2026-09-23 (scheduled for 2026-09-15; the October 2026 patch v1.35.10 is scheduled but not yet out — `https://dl.k8s.io/release/stable-1.35.txt` still reports `v1.35.9`).[^2][^3]
> **Support**: 1.35 is a supported minor until **2027-02-28** (end of life); it enters maintenance mode on **2026-12-28**. Newer minors exist: **1.36** (released 2026-04-22, latest v1.36.5) and **1.37** (released 2026-08-26, latest v1.37.1). 1.34 reaches end of life on 2026-10-27.[^2]
> **Matching libraries**: `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`, `k8s.io/apiextensions-apiserver` **v0.35.x** (latest v0.35.9, `go 1.25.0` directive), **controller-runtime v0.23.x** (latest v0.23.3, 2026-03-05), **controller-tools v0.20.x** (latest v0.20.1), Kubebuilder **v4.11.0–v4.14.x** scaffolds controller-runtime v0.23 (v4.15 moves to v0.24, v4.16 to v0.25). Go **1.25** is the minimum toolchain for these modules.[^4][^5]
> **Scope**: Kubernetes manifests (plain YAML, Kustomize output, Helm-rendered YAML), CustomResourceDefinitions with CEL validation, admission policies, and Go operators/controllers built with controller-runtime, client-go or Kubebuilder that target a 1.35 cluster.
> **Note**: This document is self-contained. House rules for operator design, CRDs, status, finalizers, RBAC, webhooks, observability and testing are carried forward; sections marked **[NEW in 1.35]** cover behaviour specific to this release. Every manifest, CRD and policy example was applied (or `--dry-run=server`-applied) against a real v1.35.0 kube-apiserver; boundary claims were re-checked on v1.34.1 and v1.36.2 apiservers; Go snippets compile against k8s.io/* v0.35.9 + controller-runtime v0.23.3 and the controller was exercised with envtest on the 1.35 binaries. Pod *runtime* behaviour (kubelet, scheduler, controllers) is sourced from the release notes and KEPs, not executed.

---

#### Table of Contents

1. [Version-aware review](#version-aware-review)
2. [What's New in Kubernetes 1.35](#whats-new-in-kubernetes-135)
3. [API versions and feature gates on a 1.35 cluster](#api-versions-and-feature-gates-on-a-135-cluster)
4. [Do not use or recommend (1.36+ features)](#do-not-use-or-recommend-136-features)
5. [Deprecated and removed — what 1.35 code must not rely on](#deprecated-and-removed--what-135-code-must-not-rely-on)
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
19. [Events (events.k8s.io)](#events-eventsk8sio)
20. [Admission: CEL Policies and Webhooks](#admission-cel-policies-and-webhooks)
21. [RBAC & Least Privilege](#rbac--least-privilege)
22. [Workload Manifests: Pods](#workload-manifests-pods)
23. [Workload Manifests: Controllers (Deployment, StatefulSet, Job, HPA)](#workload-manifests-controllers-deployment-statefulset-job-hpa)
24. [Networking: Services & NetworkPolicy](#networking-services--networkpolicy)
25. [Security Hardening](#security-hardening)
26. [client-go & apimachinery in v0.35](#client-go--apimachinery-in-v035)
27. [Observability](#observability)
28. [Error Handling & Requeue Strategy](#error-handling--requeue-strategy)
29. [Testing](#testing)
30. [kubectl 1.35 and Manifest Tooling](#kubectl-135-and-manifest-tooling)
31. [Removed / Changed in 1.35 — Migration Notes](#removed--changed-in-135--migration-notes)
32. [Production / Review Checklist](#production--review-checklist)

---

#### Version-aware review

The target is a **1.35 control plane** and **k8s.io/\* v0.35.x** client libraries. Before claiming that a field, API version or library function is "not available", check it against the pinned version:

- For manifests: the version comes from the cluster (`kubectl version`), the Helm chart's `kubeVersion`, CI's `kind`/`envtest` image, or a Kustomize/Helm render target. A field that exists in the OpenAPI schema is **not** proof that it works: `kubectl explain` on 1.35 lists alpha-gated fields such as `pod.spec.containers.lifecycle.stopSignal` and `pod.spec.volumes.projected.sources.podCertificate`, but a default 1.35 apiserver **silently drops** them on create (verified: `stopSignal` vanishes; the `podCertificate` projection becomes an empty source with the warning `volume "id" (Projected) has no sources provided`). Only `kubectl apply --dry-run=server -o yaml` against a 1.35 apiserver with the cluster's real feature gates proves a field is honoured.
- For Go code: the version comes from `go.mod` (`k8s.io/api v0.35.x`, `sigs.k8s.io/controller-runtime v0.23.x`). Verify a symbol with `go doc k8s.io/client-go/tools/cache WaitForNamedCacheSyncWithContext` in the module before flagging it.
- Say "not available in 1.35" only when the pinned version proves it (unknown field, `Unsupported value`, dropped field, missing package/function). Say "available but off by default" for alpha features and for beta **APIs** (since 1.24, new beta API versions are not served unless `--runtime-config` enables them).[^6]
- A cluster may enable feature gates. If the repository contains cluster configuration (kubeadm `ClusterConfiguration`, kind config, managed-cluster settings) that enables a gate, review against that configuration, not against the defaults below.
- **Patch level matters**: a few 1.35 defaults changed in patch releases — `MaxUnavailableStatefulSet` was turned **off** again in v1.35.4, and `SchedulerAsyncAPICalls` was disabled in v1.35.1 (see "Removed / changed in 1.35").[^3]

---

#### What's New in Kubernetes 1.35

Kubernetes 1.35 contains 60 enhancements: 17 stable, 19 beta and 22 alpha.[^1] The table lists every change that a manifest author, CRD author or controller author can observe. "Default" is the state on an unmodified v1.35.0 control plane; gates were read from `kube-apiserver --help` of the v1.34.1, v1.35.0 and v1.36.2 binaries, and API behaviour was tested on those apiservers.

##### Graduated to stable (GA) in 1.35

| Feature | What it means for manifests / controllers | KEP |
|---|---|---|
| **In-place Pod resize** (`InPlacePodVerticalScaling`) | CPU/memory `requests`/`limits` of running Pods change through the `pods/resize` subresource; `resizePolicy` per resource; memory-limit decreases are now allowed (best effort); resize status via the `PodResizePending` / `PodResizeInProgress` conditions | 1287 |
| **Pod generation** (`PodObservedGenerationTracking`) | `metadata.generation` on Pods increments on every spec change (including resize); kubelet reports `status.observedGeneration` and per-condition `observedGeneration` | 5067 |
| **`PreferSameZone` / `PreferSameNode`** Service `trafficDistribution` | `PreferClose` is deprecated in favour of `PreferSameZone` (the apiserver now returns a deprecation warning) | 3015 |
| **Job `spec.managedBy`** (`JobManagedBy` locked on) | Delegates Job status sync to an external controller (e.g. MultiKueue) | 4368 |
| **Fine-grained `supplementalGroupsPolicy`** | `pod.spec.securityContext.supplementalGroupsPolicy: Merge \| Strict` | 3619 |
| WebSockets for exec/attach/port-forward (SPDY → WebSockets transition) | Client-side streaming uses WebSockets; see the RBAC change for `pods/exec` below | 4006 |
| kubelet drop-in config directory, image GC maximum age, parallel image-pull limit, CPU-manager `strict-cpu-reservation`, topology-manager `max-allowable-numa-nodes` | Node configuration (not manifest-visible) | 3983, 4210, 3673, 4540, 4622 |
| kubectl command headers | kubectl sends `Kubectl-Command`/`Kubectl-Session` headers | 859 |
| Removal of gogo protobuf from API types | `k8s.io/api` types no longer implement `ProtoMessage()` (see client-go section) | — |

##### Beta in 1.35 (with default)

| Feature / gate | Default in 1.35.0 | Notes |
|---|---|---|
| Container restart rules — `containers[].restartPolicy` + `restartPolicyRules` (`ContainerRestartRules`) | **on** (was alpha/off in 1.34) | Per-container restart on specific exit codes; only action `Restart` |
| Env vars from files — `env[].valueFrom.fileKeyRef` (`EnvFiles`) | **on** (was alpha/off) | 1.34 rejects the field: `must specify one of: fieldRef, resourceFieldRef, configMapKeyRef or secretKeyRef` |
| Image volumes — `volumes[].image` (`ImageVolume`) | **on** (was beta/off in 1.34) | Needs a container runtime that supports it (containerd ≥ 2.1) |
| `hostnameOverride` (`HostnameOverride`) | **on** (was alpha/off) | |
| StatefulSet `rollingUpdate.maxUnavailable` (`MaxUnavailableStatefulSet`) | **on in v1.35.0–v1.35.3, off from v1.35.4** | Regression fix; also off in 1.36, back on in 1.37 |
| HPA `behavior.scaleUp/scaleDown.tolerance` (`HPAConfigurableTolerance`) | **on** (was alpha/off) | |
| Deployment/ReplicaSet `status.terminatingReplicas` (`DeploymentReplicaSetTerminatingReplicas`) | **on** | |
| Pod topology labels admission (`PodTopologyLabelsAdmission`) | **on** | `topology.kubernetes.io/zone`/`region` copied from the Node onto the Pod at binding |
| Kubelet credential verification for cached images (`KubeletEnsureSecretPulledImages`) | **on** | `IfNotPresent` no longer lets a Pod without pull credentials reuse a private image pulled by another Pod |
| CSI `serviceAccountTokenInSecrets` (`CSIServiceAccountTokenSecrets`) | **on** | CSIDriver opt-in |
| `pods/exec`, `pods/attach`, `pods/portforward` need `create` for WebSocket too (`AuthorizePodWebsocketUpgradeCreatePermission`) | **on** | RBAC-breaking for roles that granted only `get` |
| Mutable CSINode allocatable count (`MutableCSINodeAllocatableCount`) | **on** | |
| Kubelet CrashLoopBackOff max (`KubeletCrashLoopBackOffMax`) | **on** | Node config `crashLoopBackOff.maxContainerRestartPeriod` |
| Opportunistic batching in the scheduler (`OpportunisticBatching`) | **on** | No manifest change |
| `status.nominatedNodeName` semantics (`NominatedNodeNameForExpectation` in kube-scheduler, `ClearingNominatedNodeNameAfterBinding` in kube-apiserver) | **on** (both alpha/off in 1.34) | The scheduler may set `nominatedNodeName` as an expectation (not only for preemption) and the apiserver clears it on binding; external schedulers/autoscalers that read the field must not treat it as "preemption pending" |
| kuberc credential plugin policy (`credentialPluginPolicy`, `credentialPluginAllowlist`) | kubectl (kuberc `v1beta1`) | Restricts which `exec` credential plugins a kubeconfig may run; see kubectl section |
| `WatchListClient` (client-go streaming lists) | **on** | Informers use WatchList when the server supports it |
| KYAML output (`kubectl get -o kyaml`) | **on** (kubectl) | Disable with `KUBECTL_KYAML=false` |
| Pod certificates — `PodCertificateRequest` API + `podCertificate` projected volume source | **off** (beta API `certificates.k8s.io/v1beta1` not served by default) | Do not depend on it without cluster opt-in |
| Storage version migration — `storagemigration.k8s.io/v1beta1` (`StorageVersionMigrator`) | **off** (gate beta/off, API not served by default) | `v1alpha1` was removed. The release blog says "enabled by default", but a stock v1.35.0 apiserver serves no `storagemigration.k8s.io` version |
| CRD condition `observedGeneration` (`CRDObservedGenerationTracking`) | **off** | On by default in 1.36 |
| User namespaces (`hostUsers: false`) | on (beta since 1.33) | GA in 1.36 |

##### Alpha in 1.35 (off by default — treat as unavailable)

Gang scheduling and the `scheduling.k8s.io/v1alpha1` **Workload** API (`GangScheduling`, `GenericWorkload`); **numeric toleration operators** `Gt`/`Lt` (`TaintTolerationComparisonOperators`); **Node declared features** `node.status.declaredFeatures` (`NodeDeclaredFeatures`); **constrained impersonation** `impersonate-on:<mode>:<verb>` (`ConstrainedImpersonation`); restart-all-containers action (`RestartAllContainersOnContainerExits`); in-place resize of **pod-level** resources (`InPlacePodLevelResourcesVerticalScaling`); mutable pod resources and scheduling directives for **suspended Jobs** (`MutablePodResourcesForSuspendedJobs`, `MutableSchedulingDirectivesForSuspendedJobs`); mutable PersistentVolume node affinity (`MutablePVNodeAffinity`); structured `/statusz` and `/flagz` (`ComponentStatusz`, `ComponentFlagz`); `UserNamespacesHostNetworkSupport`; CCM watch-based route reconciliation.[^1][^3]

##### Other 1.35 changes that affect reviews

- **cgroup v1**: the kubelet's `failCgroupV1` now defaults to `true`; a 1.35 kubelet refuses to start on a cgroup v1 host unless `failCgroupV1: false` is set. kubeadm's preflight check errors on cgroup v1 for kubelet ≥ 1.35.[^1][^3]
- **kube-proxy `ipvs` mode is deprecated**; nftables is the recommended Linux backend.[^1]
- **containerd 1.x**: 1.35 is the last minor that supports containerd 1.7; move to containerd ≥ 2.0 before 1.36.[^1]
- **kubelet `--pod-infra-container-image` flag removed** (startup fails if still set).[^3]
- **Comparable resource versions**: all in-tree resource versions are now decimal integers that clients may compare; apimachinery adds `resourceversion.CompareResourceVersion` (KEP-5504).[^1]
- **DRA**: core Dynamic Resource Allocation (`resource.k8s.io/v1`, GA in 1.34) can no longer be disabled; `resource.k8s.io/v1` is the storage version (downgrade to 1.33 unsupported).[^3]
- **Pod resize only for CPU and memory** — 1.35 forbids resizing every other resource type (`only cpu and memory resources are mutable`). The common case (`ephemeral-storage`) was already rejected with the same message by a v1.34.1 apiserver (verified).[^3]
- **Kubelet restarts no longer flip Pods to NotReady**: the kubelet restores container state from the runtime on startup, so healthy Pods stay Ready and in Service endpoints across kubelet restarts and upgrades (KEP-4781).[^1]
- **`UserNamespacesPodSecurityStandards` gate removed** ("the minimum supported Kubernetes version for kubelet is now v1.31, so the gate is no longer needed"); remove it from apiserver `--feature-gates` before upgrading, because unknown gates fail startup.[^3]
- **DRA alpha changes**: DeviceTaintRule support moved to its own gate `DRADeviceTaintRules` (alpha/off), a new taint effect `None` allows a "dry run" before `NoExecute`; Partitionable Devices changed incompatibly (remove such ResourceSlices before upgrading or downgrading between 1.34 and 1.35).[^1][^3]
- **Patch-level env-var fixes**: a 1.34 regression starting Pods whose env values contain `$` followed by a multi-byte character is fixed in v1.35.1; the non-UTF-8 Secret value regression is fixed in v1.35.6.[^3]
- **Feature-gate dependencies are validated** at startup; `AllAlpha=true` alone no longer works without the beta gates it depends on.[^3]
- **Ingress NGINX** is retired (best-effort maintenance until March 2026, then archived); new ingress work should use the Gateway API.[^1]

---

#### API versions and feature gates on a 1.35 cluster

##### API group versions served by a default v1.35.0 apiserver

Verified with `kubectl api-versions` on a stock v1.35.0 apiserver (identical list on v1.34.1 and v1.36.2):

```text
admissionregistration.k8s.io/v1   apiextensions.k8s.io/v1   apiregistration.k8s.io/v1
apps/v1   authentication.k8s.io/v1   authorization.k8s.io/v1   autoscaling/v1   autoscaling/v2
batch/v1   certificates.k8s.io/v1   coordination.k8s.io/v1   discovery.k8s.io/v1
events.k8s.io/v1   flowcontrol.apiserver.k8s.io/v1   networking.k8s.io/v1   node.k8s.io/v1
policy/v1   rbac.authorization.k8s.io/v1   resource.k8s.io/v1   scheduling.k8s.io/v1
storage.k8s.io/v1   v1
```

No `v1beta1`/`v1alpha1` group version is served by default. In particular, on a default 1.35 cluster these are **not** served and manifests using them fail with `no matches for kind`:

| API | Status in 1.35 | How to enable |
|---|---|---|
| `admissionregistration.k8s.io/v1beta1` MutatingAdmissionPolicy(Binding) | beta, off | `--feature-gates=MutatingAdmissionPolicy=true --runtime-config=admissionregistration.k8s.io/v1beta1=true` (GA `v1` in 1.36) |
| `storagemigration.k8s.io/v1beta1` StorageVersionMigration | beta, off | `--feature-gates=StorageVersionMigrator=true --runtime-config=storagemigration.k8s.io/v1beta1=true` (`v1` GA in 1.37) |
| `certificates.k8s.io/v1beta1` PodCertificateRequest, ClusterTrustBundle | beta, off | gate + runtime-config |
| `scheduling.k8s.io/v1alpha1` Workload | alpha, off | removed again in 1.36 (`v1alpha2`), `v1beta1` in 1.37 |
| `resource.k8s.io/v1beta1`, `v1beta2` | off; `resource.k8s.io/v1` is the served DRA version | — |
| `storage.k8s.io/v1alpha1` VolumeAttributesClass | **removed** in 1.35 (use `storage.k8s.io/v1`, GA since 1.34) | — |
| `storagemigration.k8s.io/v1alpha1` | **removed** in 1.35 | — |

No beta or GA API version has been removed since 1.32 (the Deprecated API Migration Guide lists no removals after the flowcontrol `v1beta3` APIs in 1.32, as of 1.37); 1.35 only removes the two alpha versions above.[^7]

##### Selected feature gates (kube-apiserver v1.35.0 defaults)

```text
ContainerRestartRules=true (BETA)              EnvFiles=true (BETA)
HostnameOverride=true (BETA)                   ImageVolume=true (BETA)
MaxUnavailableStatefulSet=true (BETA)  -> false from v1.35.4
HPAConfigurableTolerance=true (BETA)           DeploymentReplicaSetTerminatingReplicas=true (BETA)
PodTopologyLabelsAdmission=true (BETA)         AuthorizePodWebsocketUpgradeCreatePermission=true (BETA)
KubeletEnsureSecretPulledImages=true (BETA)    CSIServiceAccountTokenSecrets=true (BETA)
PodLevelResources=true (BETA)                  UserNamespacesSupport=true (BETA)
MatchLabelKeysInPodTopologySpread=true (BETA)  DRAPrioritizedList=true (BETA)   DRAAdminAccess=true (BETA)
WatchListClient=true (BETA)                    OpportunisticBatching=true (BETA)
MutatingAdmissionPolicy=false (BETA)           StorageVersionMigrator=false (BETA)
PodCertificateRequest=false (BETA)             ClusterTrustBundle=false (BETA)
CRDObservedGenerationTracking=false (BETA)     SELinuxMount=false (BETA)
TaintTolerationComparisonOperators=false (ALPHA)  NodeDeclaredFeatures=false (ALPHA)
GangScheduling=false (ALPHA)  GenericWorkload=false (ALPHA)  ConstrainedImpersonation=false (ALPHA)
RestartAllContainersOnContainerExits=false (ALPHA)  InPlacePodLevelResourcesVerticalScaling=false (ALPHA)
RelaxedServiceNameValidation=false (ALPHA)     StrictIPCIDRValidation=false (ALPHA)
ContainerStopSignals=false (ALPHA)             DRAConsumableCapacity/DRADeviceTaints/DRAExtendedResource/DRAPartitionableDevices=false (ALPHA)
```

GA gates (for example `InPlacePodVerticalScaling`, `PodObservedGenerationTracking`, `JobManagedBy`, `SupplementalGroupsPolicy`, `PreferSameTrafficDistribution`) are locked on and no longer appear in `--help`.

##### Already-GA features from earlier releases that 1.35 code should use

| Feature | GA since | Use it for |
|---|---|---|
| CRD validation rules (`x-kubernetes-validations`, CEL) | 1.29 | Cross-field and transition validation instead of webhooks |
| ValidatingAdmissionPolicy (`admissionregistration.k8s.io/v1`) | 1.30 | In-process policy instead of validating webhooks |
| Webhook `matchConditions` | 1.30 | Skip webhook calls with CEL pre-filters |
| Pod scheduling gates (`schedulingGates`) | 1.30 | Hold Pods until an external controller is ready |
| AppArmor `securityContext.appArmorProfile` fields | 1.31 | Replaces the `container.apparmor.security.beta.kubernetes.io/*` annotations (no longer mirrored since 1.34) |
| Job `podFailurePolicy` | 1.31 | Fail fast / ignore disruptions |
| StatefulSet `ordinals.start`, PVC retention policy | 1.31 / 1.32 | Migrations, cleanup of volumes |
| CRD `selectableFields` (field selectors for CRs) | 1.32 | `kubectl get --field-selector spec.x=y` and cache field indexes server-side |
| Sidecar containers (`initContainers[].restartPolicy: Always`) | 1.33 | Log shippers, proxies |
| Job `backoffLimitPerIndex`, `successPolicy` | 1.33 | Indexed Jobs |
| CRD validation ratcheting | 1.33 | Tightening validation without breaking stored objects on unrelated updates |
| `recursiveReadOnly` volume mounts | 1.33 | Truly read-only mounts |
| Service `trafficDistribution` field (`PreferClose`) | 1.33 | Zonal routing — now spell it `PreferSameZone` |
| Job `podReplacementPolicy` | 1.34 | Avoid overlapping replacement Pods |
| `PodLifecycleSleepAction` (`lifecycle.preStop.sleep`), including `seconds: 0` | 1.34 | Graceful drain without a shell in the image |
| VolumeAttributesClass (`storage.k8s.io/v1`) | 1.34 | Mutable volume performance classes |
| Core DRA (`resource.k8s.io/v1`) | 1.34 | Device claims |
| Ordered namespace deletion | 1.34 | Pods deleted before the policies that protect them |
| Structured authentication configuration | 1.34 | apiserver JWT config |

---

#### Do not use or recommend (1.36+ features)

The cluster is 1.35. Never suggest the following as available; they fail validation, are silently dropped, or need a newer apiserver/library. (Versions are taken from the 1.36 and 1.37 release notes and verified on the v1.36.2 apiserver where a boundary is involved.)[^8][^9]

**Kubernetes 1.36 (released 2026-04-22):**

- `admissionregistration.k8s.io/v1` **MutatingAdmissionPolicy** (GA in 1.36). On 1.35 only `v1beta1`, and only when explicitly enabled.
- **User namespaces GA** and **image volumes GA** (both are beta-and-on in 1.35 — usable, but do not describe them as GA).
- **Service names starting with a digit** (`RelaxedServiceNameValidation`, beta-on in 1.36). Verified: 1.35 rejects `metadata.name: 1password-connect` with `a DNS-1035 label must ... start with an alphabetic character`; 1.36 accepts it.
- **Strict IP/CIDR validation** (`StrictIPCIDRValidation`, on in 1.36): 1.36 rejects `externalIPs: ["010.000.000.005"]` (`must not have leading 0s`); 1.35 only warns (`non-standard IP address ... will be considered invalid in a future Kubernetes release`). Write canonical IPs anyway.
- **CRD numeric format range enforcement**: 1.36 rejects `3000000000` for a `format: int32` field (`Checked value must be of type integer with format int32`); 1.35 stores it. Do not rely on `format: int32` as a range check on 1.35 — add `minimum`/`maximum`.
- `CRDObservedGenerationTracking` on by default, `scheduling.k8s.io/v1alpha2` Workload/PodGroup, DRA list-type attributes and `.includes()`, `ResourcePoolStatusRequest`, `ManifestBasedAdmissionControlConfig` (`staticManifestsDir`), `PersistentVolumeClaimUnusedSinceTime`, `ImageVolumeWithDigest`, `MemoryReservationPolicy`, `--tls-curve-preferences`, `kubectl explain -r`, `kubectl wait` with multiple conditions, `kubectl kuberc` as a non-alpha command, and the kuberc allowlist key `command` (1.35 uses `name`).
- Constrained impersonation beta-on, `ExtendWebSocketsToKubelet`, `ComponentStatusz`/`ComponentFlagz` beta, `UnknownVersionInteroperabilityProxy` beta, pod-level in-place resize beta (`InPlacePodLevelResourcesVerticalScaling`), resizing running non-sidecar init containers, `RestartAllContainers` restart action (beta in 1.36), DRA consumable capacity / device taints / partitionable devices / extended resources as beta-on, `NodeDeclaredFeatures` beta, `MutablePodResourcesForSuspendedJobs` on by default.
- Go: `k8s.io/* v0.36` / controller-runtime **v0.24** (both require Go 1.26), `metav1.NewFieldsV1` / `FieldsV1.GetRawBytes()` accessors, `LastStoreSyncResourceVersion` on informer stores, the `k8s.io/streaming` module.

**Kubernetes 1.37 (released 2026-08-26):**

- `storagemigration.k8s.io/v1` StorageVersionMigration (GA); `metrics.k8s.io/v1`; Pod certificates and ClusterTrustBundle **GA** (`certificates.k8s.io/v1` PodCertificateRequest); `scheduling.k8s.io/v1beta1` Workload/PodGroup; KYAML **GA**; `SELinuxMount` GA.
- StatefulSet `updateStrategy.type: Recreate`; `httpGet.protocol` (H2C probes); `emptyDir.mode`; per-mount bind options (`noexec`/`nodev`/`nosuid`); `.spec.evictionResponders` / EvictionRequest; Node lifecycle conditions (`DrainInProgress`, `MaintenancePlanned`, …); `DefaultPodSysctls`; DRA device compatibility groups and derived attributes; HPA scale-to-zero beta; pod-level checkpoint/restore.
- Go: controller-runtime **v0.25** (k8s.io v0.37), client-go context-everywhere APIs.

**Never recommend** removing `failCgroupV1: false` workarounds, switching kube-proxy to nftables, or replacing `PreferClose` *as a requirement* without context — those are migrations, not bugs. Do recommend them as low-priority notes.

---

#### Deprecated and removed — what 1.35 code must not rely on

| Item | Status | Replacement |
|---|---|---|
| Service `trafficDistribution: PreferClose` | Deprecated in 1.35 (warning on create/update) | `PreferSameZone` (identical semantics) |
| kube-proxy `mode: ipvs` | Deprecated in 1.35 (warning at startup) | `nftables` (GA since 1.33) or `iptables` |
| cgroup v1 nodes | kubelet fails by default (`failCgroupV1: true`) | cgroup v2 hosts |
| containerd 1.7 | Last supported in 1.35 | containerd ≥ 2.0 (≥ 2.1 for image volumes) |
| kubelet `--pod-infra-container-image` | Removed | CRI runtime config (`sandbox_image`) |
| `storagemigration.k8s.io/v1alpha1`, `storage.k8s.io/v1alpha1` VolumeAttributesClass | Removed | `v1beta1` (opt-in) / `storage.k8s.io/v1` |
| `status.resize` on Pods | Deprecated (field still present) | `PodResizePending` / `PodResizeInProgress` conditions |
| `ProtoMessage()` on k8s.io/api types | Removed (build tag `kubernetes_protomessage_one_more_release` restores it for 1.35 only) | `k8s.io/apimachinery/pkg/runtime/serializer/protobuf` |
| client-go `cache.WaitForNamedCacheSync` | Superseded | `cache.WaitForNamedCacheSyncWithContext(ctx, ...)` |
| controller-runtime `GetEventRecorderFor` / `record.EventRecorder` | Deprecated in v0.23 | `GetEventRecorder` → `events.EventRecorder` (events.k8s.io/v1) |
| controller-runtime `reconcile.Result.Requeue` | Deprecated | `RequeueAfter` or return an error |
| controller-runtime `WithCustomValidator` / `WithCustomDefaulter`, `admission.CustomValidator` / `CustomDefaulter` | Deprecated in v0.23 | `WithValidator` / `WithDefaulter` with typed `admission.Validator[T]` / `Defaulter[T]` |
| `client-go/kubernetes/fake.NewSimpleClientset` | Superseded (no field management / SSA) | `fake.NewClientset` |
| Ingress NGINX controller | Retired (archived after March 2026) | Gateway API implementations |
| `gitRepo` volumes | Deprecated since 1.11; still technically usable on 1.35 nodes | init container / git-sync. 1.36 disables the plugin permanently |
| Service `externalIPs` | Still supported in 1.35; deprecated in 1.36 (removal planned for 1.43), CVE-2020-8554 | LoadBalancer, NodePort, Gateway API |
| kubeadm config `kubeadm.k8s.io/v1beta3` | Deprecated since 1.31, still accepted by kubeadm 1.35; removed in kubeadm 1.37 | `kubeadm.k8s.io/v1beta4` (`kubeadm config migrate` with a ≤ 1.36 kubeadm) |

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
│       └── rediscluster_webhook.go      # typed Validator / Defaulter
├── config/
│   ├── crd/                             # Generated CRD manifests
│   ├── rbac/                            # Generated RBAC manifests
│   └── default/                         # Kustomize overlay
├── cmd/main.go
├── Dockerfile
└── Makefile                             # make generate, make manifests, make test
```

##### Toolchain versions for a 1.35 target

| Component | Version for 1.35 | Notes |
|---|---|---|
| Go | ≥ 1.25 | `k8s.io/* v0.35` and controller-runtime v0.23 declare `go 1.25.0`; v0.36/v0.24 need Go 1.26 |
| `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` | v0.35.x (keep all on the **same** patch) | Mixed minors cause compile or runtime scheme errors |
| controller-runtime | v0.23.x | Pinned to `k8s.io/* v0.35.0`; Go MVS raises it to your v0.35.x patch |
| controller-tools (`controller-gen`) | v0.20.x | Generates CRDs with `selectableFields`, CEL `reason`/`fieldPath`/`messageExpression` |
| Kubebuilder | v4.11.0–v4.14.x (scaffold controller-runtime v0.23.x, controller-tools v0.20.x); v4.15.0+ scaffolds controller-runtime v0.24+ (1.36 libraries) | |
| envtest binaries | 1.35.x (`setup-envtest use 1.35.x`) | Test against the version you ship to |

```go
// go.mod — all k8s.io modules on one patch release
module example.com/redis-operator

go 1.25.0

require (
	k8s.io/api v0.35.9
	k8s.io/apimachinery v0.35.9
	k8s.io/client-go v0.35.9
	sigs.k8s.io/controller-runtime v0.23.3
)
```

Flag in review: `k8s.io/client-go v0.36.x` (or controller-runtime v0.24+) in a module that claims to support a 1.35 cluster is usually fine on the wire (client/server skew of ±1 minor is supported), but it drags in a Go 1.26 requirement and 1.36 API fields the 1.35 server will drop. Pin the library minor to the oldest cluster minor you support when you rely on new fields.

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
| **DNS Label** (RFC 1035, stricter) | **Service names on 1.35** (cannot start with a digit), some legacy fields | lowercase `a–z`, digits `0–9`, `-` | same as above PLUS cannot start with a digit | must start with `[a-z]`, end with alphanumeric | 63 |
| **Qualified Name** (label/annotation key, finalizer) | label keys, annotation keys, finalizer names | name part: `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` ; optional prefix part: DNS Subdomain followed by `/` | name part: whitespace, `/` (except as prefix separator), `:`, all other punctuation | name part must start AND end with alphanumeric | name ≤ 63; prefix ≤ 253; total ≤ 317 |
| **Label Value** | `metadata.labels` values | `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` (or empty string) | whitespace, `/`, `:`, all other punctuation, unicode | if non-empty: alphanumeric at both ends | 63 |
| **Annotation Value** | `metadata.annotations` values | any valid UTF-8 string (incl. whitespace, JSON, base64) | none per key | n/a | total across all annotations ≤ 256 KiB |
| **C-Identifier** | env var names that must be portable, ConfigMap/Secret keys used with `envFrom` in older clusters | `A–Z`, `a–z`, `0–9`, `_` | `-`, `.`, `/`, whitespace, all other punctuation | must start with `[A-Za-z_]` (not a digit) | no hard limit |
| **ConfigMap / Secret data key** | `data` / `stringData` keys | `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` | `/`, whitespace, `:`, all other punctuation | any alphanumeric | 253 |
| **Path Segment Name** | `metadata.name` for objects exposed in URL paths | DNS Subdomain set MINUS `.` and `..` as full name | same as DNS Subdomain plus the literal names `.` and `..` | as DNS Subdomain | 253 |

> ⚠️ Underscore `_` is **never** valid in a DNS Subdomain or DNS Label name — that rules it out for `metadata.name`, `metadata.namespace`, API group, and Service name. Underscore IS valid in label/annotation keys (name part), label values, and ConfigMap/Secret data keys.

> ⚠️ Uppercase letters are **never** valid in DNS Subdomain or DNS Label names. They ARE valid in label keys (name part), label values, qualified names, and C-identifiers.

> ⚠️ **Service names on 1.35 must start with a letter.** `RelaxedServiceNameValidation` (allows RFC 1123 labels such as `1password-connect`) is alpha/off in 1.35 and only becomes beta-on in 1.36. Verified: 1.35 rejects `1password-connect`; 1.36 accepts it.

> Environment variable names: since 1.34 (`RelaxedEnvironmentVariableValidation` GA) any printable ASCII except `=` is accepted, but names that are not C identifiers are invisible to POSIX shells. Keep C identifiers for anything a shell script reads.

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

**[NEW in 1.35]** With `PodTopologyLabelsAdmission` (beta, on), the apiserver copies `topology.kubernetes.io/zone` and `topology.kubernetes.io/region` from the Node onto each Pod when the Pod is bound through the `pods/<name>/binding` subresource (the path the scheduler uses). Verified: after binding to a Node labelled `topology.kubernetes.io/zone: eu-central-1a`, the Pod carries both labels. Consequences for review:

- Do not treat those two labels on Pods as user-owned; tools that diff Pod labels (or controllers that "fix" unexpected labels) must ignore them.
- Pods can now read their zone through the Downward API (`fieldRef: {fieldPath: "metadata.labels['topology.kubernetes.io/zone']"}`) instead of needing `get nodes` RBAC. A Pod created before binding (or bound through the legacy `POST /api/v1/namespaces/<ns>/bindings` endpoint, which the admission plugin does not handle — verified) does not get them; code must tolerate an empty value.

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

controller-gen v0.20.1 turns `+kubebuilder:selectablefield` into `spec.versions[].selectableFields` (GA since 1.32); `kubectl get rcl --field-selector spec.tier=premium` then works server-side (verified on 1.35). At most 8 selectable fields per version; each must point to a string (enum/format allowed), integer or boolean field outside `metadata`.

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
- **`format: int32` is not a range check on 1.35.** Verified: a 1.35 apiserver stores `3000000000` in a field declared `type: integer, format: int32`; only 1.36 starts enforcing numeric format ranges. Add explicit `minimum`/`maximum` (`+kubebuilder:validation:Minimum/Maximum`).

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

CRD validation rules (`x-kubernetes-validations`) are GA since 1.29 and are the first choice for invariants that the OpenAPI schema cannot express.[^14] The following was verified against a 1.35.0 apiserver with a controller-gen v0.20.1-generated CRD:

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

The same rules written directly in a CRD manifest, plus the CEL libraries available on 1.35 (`quantity`, `url`, `isURL`, `isSemver`/`semver`, `format`):

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
            required: [tier, replicas, version]
            x-kubernetes-validations:
            - rule: "!has(self.maxMemory) || quantity(self.maxMemory).isGreaterThan(quantity('64Mi'))"
              message: maxMemory must be larger than 64Mi
              fieldPath: .maxMemory
            - rule: "self.tier != 'premium' || self.replicas >= 3"
              messageExpression: "'premium tier needs at least 3 replicas, got %d'.format([self.replicas])"
              reason: FieldValueInvalid
              fieldPath: .replicas
            - rule: "semver(self.version, true).compareTo(semver(oldSelf.version, true)) >= 0"
              message: version downgrades are not allowed
              fieldPath: .version
            properties:
              tier: {type: string, enum: [standard, premium]}
              replicas: {type: integer, format: int32, minimum: 1, maximum: 99}
              version:
                type: string
                maxLength: 32
                x-kubernetes-validations:
                - rule: "isSemver(self, true)"
                  message: version must be a semantic version
              storageClassName:
                type: string
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

Observed results on 1.35 (all errors are reported together):

```text
* spec.maxMemory: Invalid value: maxMemory must be larger than 64Mi
* spec.replicas: Invalid value: premium tier needs at least 3 replicas, got 2
* spec.version: Invalid value: "seven": version must be a semantic version
* spec.endpoints[0].url: Invalid value: "http://redis.example.com": url must be an https URL
# update 7.2.4 -> 7.0.0 and fast -> slow:
* spec.version: Invalid value: version downgrades are not allowed
* spec.storageClassName: Invalid value: "slow": storageClassName is immutable
```

##### CEL review rules (verified pitfalls)

1. **`string(<int>)` in `messageExpression` blows the cost budget.** On 1.35 (and 1.36) the CRD is rejected with `estimated messageExpression cost exceeds budget by factor of more than 100x` for `'got ' + string(self.replicas)` even though `replicas` has `maximum: 99`. Use `'got %d'.format([self.replicas])`, which is accepted.
   ```yaml
   # BAD — CRD rejected on 1.35
   messageExpression: "'premium tier needs >= 3 replicas, got ' + string(self.replicas)"
   # GOOD
   messageExpression: "'premium tier needs at least 3 replicas, got %d'.format([self.replicas])"
   ```
2. **`has()` takes a field selection, not a variable.** `!has(oldSelf)` fails to compile (`invalid argument to has() macro`). With `optionalOldSelf: true` the variable is an optional: use `oldSelf.hasValue()` / `oldSelf.value()`, and `optionalOldSelf` may only be set when the rule references `oldSelf` (`may not be set if oldSelf is not used in rule`).
3. **Optional `oldSelf` on a string loses its bounds.** `semver(self, true).compareTo(semver(oldSelf.value(), true))` on a `maxLength: 32` string with `optionalOldSelf: true` was rejected for exceeding the cost budget on 1.35. Put transition rules one level up (on the parent object, comparing `self.version` with `oldSelf.version`), which compiles cheaply and only runs on update.
4. **`self == oldSelf` does not stop removal.** Transition rules only run when both old and new values exist. Verified: a merge patch setting `storageClassName: null` succeeds despite the immutability rule. Add a parent-level rule such as `!has(oldSelf.storageClassName) || has(self.storageClassName)` when "immutable" must also mean "cannot be unset" (verified: removal is then rejected with `spec: Invalid value: storageClassName cannot be removed once set`).
5. **Unbounded inputs make the CRD uninstallable**, not slow: the cost estimator assumes maximum sizes. Every string/array/map reached by a rule needs `maxLength`/`maxItems`/`maxProperties`.
6. **Prefer `fieldPath` + `reason`** (`FieldValueInvalid`, `FieldValueForbidden`, `FieldValueRequired`, `FieldValueDuplicate`) so clients see the right field and status cause.
7. **Ratcheting** (GA since 1.33): unchanged invalid values in stored objects do not block unrelated updates. You may tighten validation for *new* writes, but still treat it as a breaking change for clients that create objects.
8. `x-kubernetes-int-or-string` fields are dynamic in CEL; guard with `type(self.x) == string` or use `quantity()` only after a `pattern` restricts the input.
9. **Guard optional fields with `has()`** in object-level rules. A rule such as `semver(self.version, true).compareTo(semver(oldSelf.version, true)) >= 0` is fine while `version` is `required`; if the field is optional, every update of an object without it fails with `spec: Invalid value: "object": no such key: version evaluating rule: version downgrades are not allowed` — including a controller's own finalizer or metadata patches (verified on 1.35: a merge patch that only adds a finalizer is rejected, and an envtest reconciler looped on this error). Write `!has(self.version) || !has(oldSelf.version) || ...`.

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

`client.MergeFrom` sends a JSON merge patch without a `resourceVersion` precondition. If two writers own different list entries (for example condition types), use `client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})` or server-side apply for status with a dedicated field manager.

##### observedGeneration

Always set `status.observedGeneration = metadata.generation` after a successful reconcile:

```go
obj.Status.ObservedGeneration = obj.Generation
```

This allows external tools to detect whether the controller has processed the latest spec change.

**[NEW in 1.35]** Pods follow the same contract: `metadata.generation` is maintained for Pods (GA) and increments on every spec change, including in-place resizes through `pods/resize` (verified: generation 1 → 2 after an `activeDeadlineSeconds` update → 3 after a resize). The kubelet reports `status.observedGeneration` and `status.conditions[].observedGeneration`. Code that waits for a Pod change (resize, image update) must compare `pod.Status.ObservedGeneration >= pod.Generation` instead of sleeping or comparing resource values.

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

**[NEW in 1.35]** `CustomResourceDefinition.status.conditions[].observedGeneration` exists in the API (`CRDObservedGenerationTracking`), but the gate is beta and **off** in 1.35 (on in 1.36). Verified: on 1.35 the CRD's `Established`/`NamesAccepted` conditions carry no `observedGeneration`. Tools that wait for a CRD update must keep using `kubectl wait --for=condition=Established` plus a discovery check, not `observedGeneration`.

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
7. Use a **conversion webhook** when schemas diverge between versions. controller-runtime v0.23 lets conversion functions live outside the API package (`WithConverter` on the webhook builder).
8. **Before removing an old version from `spec.versions`**, every stored object must be rewritten to the new storage version and the old version removed from `status.storedVersions`. On a default 1.35 cluster the in-tree `StorageVersionMigration` API is **not** served (beta, off); use the out-of-tree kube-storage-version-migrator or a read/replace loop (`kubectl get <plural> -A -o json | kubectl replace -f -`), then remove the old version from `status.storedVersions` with `kubectl patch crd <name> --subresource=status`.

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

##### Reconciler Skeleton (controller-runtime v0.23)

The following reconciler compiles against k8s.io/* v0.35.9 + controller-runtime v0.23.3 and was exercised with envtest on the 1.35 control-plane binaries (CEL rejection, SSA-created Deployment with owner reference, status condition transitions, an `events.k8s.io/v1` Event, finalizer-driven deletion).

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
	"k8s.io/client-go/tools/events"
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
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

type RedisClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder // mgr.GetEventRecorder(...), events.k8s.io/v1
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
		r.Recorder.Eventf(rc, nil, corev1.EventTypeNormal, "Available", "Reconcile", "all %d replicas ready", rc.Spec.Replicas)
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

// fail records a transient failure as Ready=False + Warning Event and returns
// the error so the request is retried with backoff.
func (r *RedisClusterReconciler) fail(ctx context.Context, rc *cachev1.RedisCluster, reason string, cause error) (ctrl.Result, error) {
	before := rc.DeepCopy()
	if meta.SetStatusCondition(&rc.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            cause.Error(),
		ObservedGeneration: rc.Generation,
	}) {
		r.Recorder.Eventf(rc, nil, corev1.EventTypeWarning, reason, "Reconcile", "%v", cause)
	}
	if err := r.Status().Patch(ctx, rc, client.MergeFrom(before)); err != nil {
		log.FromContext(ctx).Error(err, "status update failed")
	}
	return ctrl.Result{}, cause
}
```

Review points:

- Finalizer add/remove uses `Patch(client.MergeFrom(...))`, not `Update`, so a stale cached copy cannot overwrite spec changes made between cache sync and write. (A merge patch replaces the whole `finalizers` list; if other controllers add finalizers concurrently, use `client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})`.)
- `GenerationChangedPredicate` on `For()` skips status-only updates (our own status writes would otherwise re-trigger the reconciler). Do not put it on `Owns()` for Deployments: Deployment status changes (ready replicas) are exactly the events the owner must see.
- `GenerationChangedPredicate` drops metadata-only updates (labels, annotations, finalizer edits by others). Deletion still gets through: when the apiserver sets `deletionTimestamp` it also increments `metadata.generation` (apiserver `BeforeDelete`: "we need to bump object's Generation"), so the finalizer path runs — verified in envtest, where the reconciler above removed its finalizer after `Delete` with this predicate in place.

##### Reconcile Return Conventions

| Return | Meaning |
|---|---|
| `ctrl.Result{}, nil` | Success — no requeue until a watched event fires |
| `ctrl.Result{}, err` | Transient error — requeued with per-item exponential backoff (5 ms doubling up to 1000 s by default) |
| `ctrl.Result{RequeueAfter: d}, nil` | Requeue after duration `d` (polling external state) |
| `ctrl.Result{Requeue: true}, nil` | **Deprecated** since controller-runtime v0.21 (`Deprecated: Use RequeueAfter instead.`) — rate-limited requeue with no clear semantics |
| `ctrl.Result{RequeueAfter: d}, err` | The error wins; `RequeueAfter` is ignored and controller-runtime logs a warning. Pick one |

**[NEW in controller-runtime v0.23]** The controller work queue is the **priority queue by default** (`config.Controller.UsePriorityQueue`: "This flag is enabled by default"). `reconcile.Result` gained `Priority *int`; the built-in handlers enqueue create events from the initial list and resync updates whose `resourceVersion` did not change with `handler.LowPriority`, so real changes are handled first after a restart. Do not write code that depends on FIFO ordering between keys.

##### Manager Setup

```go
mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
	Scheme:                 scheme,
	Metrics:                metricsserver.Options{BindAddress: ":8443", SecureServing: true},
	HealthProbeBindAddress: ":8081",
	LeaderElection:         true,
	LeaderElectionID:       "rediscluster.cache.example.com",
})
if err != nil {
	os.Exit(1)
}
if err := (&controller.RedisClusterReconciler{
	Client:   mgr.GetClient(),
	Scheme:   mgr.GetScheme(),
	Recorder: mgr.GetEventRecorder("rediscluster-controller"),
}).SetupWithManager(mgr); err != nil {
	os.Exit(1)
}
```

- Serve metrics with `SecureServing: true` (and the `filters.WithAuthenticationAndAuthorization` filter in production) — plain-HTTP `:8080` metrics endpoints are a review finding.
- `LeaderElection: true` for any controller that can run with more than one replica.
- `MaxConcurrentReconciles` > 1 is safe only if the reconciler holds no shared mutable state without locking; the queue guarantees a single key is never processed concurrently.

---

#### Server-Side Apply from Controllers

Server-side apply (SSA) with a stable field manager is the preferred way for a controller to own child objects: it sends only the fields the controller cares about, merges with fields set by other actors (HPA replicas, mutating webhooks, users), and resolves conflicts per field. controller-runtime v0.22+ has a typed `client.Apply` method that accepts generated **apply configurations**; v0.23 adds **subresource apply** (`r.Status().Apply(...)`) and default `client.Options.FieldOwner`/`FieldValidation`.[^10]

```go
import (
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
	return r.Apply(ctx, dep, fieldOwner, client.ForceOwnership)
}
```

Verified in envtest on 1.35: the Deployment is created with one controller owner reference and `managedFields[0].manager == "rediscluster-controller"`.

SSA review rules:

- **Always send the complete desired intent** of the fields you own. A field omitted from a later apply is *removed* if this manager was its only owner. Building the apply configuration conditionally ("only set replicas if changed") is a bug.
- **Use apply configurations, not typed structs**, for SSA. Applying a typed `appsv1.Deployment{}` serialises zero values (`replicas: 0` if the pointer is set, empty `strategy: {}`) and takes ownership of fields you never meant to own. The legacy `r.Patch(ctx, obj, client.Apply, ...)` form is the pattern to flag: the `client.Apply` patch type is deprecated in v0.23 ("Use client.Client.Apply() and client.Client.SubResource(\"subrsource\").Apply() instead").
- **One field manager per controller** (`client.FieldOwner`), constant across releases. Changing it orphans ownership and leaves stale fields behind.
- `client.ForceOwnership` is appropriate for a controller that is the source of truth for those fields; without it, a conflict (another manager set a different value) returns `409 Conflict`. Never force-apply fields that users are expected to tune (e.g. `replicas` when an HPA targets the Deployment) — omit them from the apply configuration instead.
- Do not mix SSA and `Update` on the same object from the same controller; `Update` takes ownership of every field (manager with operation `Update`) and later applies then fight the old manager entry.
- For CRDs without generated apply configurations, either generate them (`controller-gen applyconfiguration`) or use `client.MergeFrom` patches; do not hand-build `unstructured` apply payloads without `apiVersion`/`kind`.

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

**[NEW in controller-runtime v0.23]** `source.Kind` waits until event handlers have synced before reconciling, and `cache.Options` accepts finer-grained `SyncPeriod` configuration. Do not set a short global `SyncPeriod` as a substitute for proper watches; resyncs replay every cached object through the handlers.

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

Verified on 1.35: on a custom resource, `finalizers: [cleanup]` is accepted with `Warning: metadata.finalizers: "cleanup": prefer a domain-qualified finalizer name to avoid accidental conflicts with other finalizer writers`; on built-in types (e.g. a ConfigMap) it is rejected: `name is neither a standard finalizer name nor is it fully qualified`.

##### Rules

- Add the finalizer **before** creating any external resource; remove it only after cleanup succeeded.
- Cleanup logic **must be idempotent** — the reconcile loop may be called multiple times while `DeletionTimestamp` is set.
- Never add a new finalizer after `DeletionTimestamp` is set — only removal is allowed (verified: `no new finalizers can be added if the object is being deleted`).
- Do not silently swallow errors; failing cleanup should be retried (return the error) and surfaced as a condition + Warning Event.
- Do not create resources inside a namespace that has `DeletionTimestamp` set.
- Removing the finalizer when the object is already gone returns NotFound — wrap with `client.IgnoreNotFound`.

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

##### When to Use Finalizers vs. OwnerReferences

| Scenario | Use |
|---|---|
| Child resource in same namespace | `ownerReferences` (automatic GC) |
| Cluster-scoped child of a namespaced owner | `finalizer` on the owner (or use a cluster-scoped owner) |
| External resource (cloud, DNS) | `finalizer` |
| Cross-namespace cleanup needed | `finalizer` |

---

#### Events (events.k8s.io)

**[NEW in controller-runtime v0.23]** The manager's recorder moved to the `events.k8s.io/v1` API. `mgr.GetEventRecorder(name)` returns a `k8s.io/client-go/tools/events.EventRecorder`; the old `mgr.GetEventRecorderFor(name)` (core/v1 `record.EventRecorder`) is deprecated: "this uses the old events API and will be removed in a future release".[^10]

```go
// events.EventRecorder: regarding, related, type, reason, action, note format, args
r.Recorder.Eventf(rc, nil, corev1.EventTypeWarning, "ApplyFailed", "Reconcile", "%v", err)
r.Recorder.Eventf(rc, nil, corev1.EventTypeNormal, "Available", "Reconcile", "all %d replicas ready", rc.Spec.Replicas)
```

Review points:

- **RBAC changes**: the controller now needs `events.k8s.io` `events` `create;patch`. A Kubebuilder project upgraded to v0.23 that still has only `// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch` silently loses its Events (the recorder logs the failure; reconciliation continues). Verified in envtest: Events created via `GetEventRecorder` appear in `events.k8s.io/v1` with `reason` and `action` set.
- `action` is required by `events.k8s.io/v1` (verified: an Event without it is rejected with `action: Required value`); it says what the controller did or failed to do (`Reconcile`, `Provision`, `Delete`), `reason` says why. Never pass an empty `action`.
- `note` is a format string — never pass user-controlled text as the format (`Eventf(obj, nil, t, r, a, err.Error())` is a format-string bug); use `"%v", err`.
- Emit Events on transitions (use the `changed` result of `meta.SetStatusCondition`), not on every reconcile.

---

#### Admission: CEL Policies and Webhooks

##### Validation Hierarchy (prefer left-to-right)

```
CRD schema + CEL rules  →  ValidatingAdmissionPolicy (CEL, GA 1.30)  →  Validating webhook
(per-CRD, no extra process)  (cluster policy, no extra process)          (extra infra, TLS, HA)
```

Mutation on 1.35: a **mutating webhook** is still the default mechanism. `MutatingAdmissionPolicy` exists only as `admissionregistration.k8s.io/v1beta1` and is **not served** by default (gate `MutatingAdmissionPolicy` beta/off, beta API needs `--runtime-config`); it becomes GA `v1` in 1.36. Do not ship a chart that requires MutatingAdmissionPolicy to a stock 1.35 cluster.

##### ValidatingAdmissionPolicy (verified on 1.35)[^15]

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
`ValidatingAdmissionPolicy 'require-digest-and-limits' with binding 'require-digest-and-limits' denied request: all images must be pinned by digest: migrate` (only the first failing validation was reported in this test).

Review points for policies:

- A policy without a **binding** does nothing; a binding's `validationActions` decides `Deny`/`Warn`/`Audit`. `[Deny, Warn]` is rejected (`must not contain both Deny and Warn`); roll out new policies with `[Warn, Audit]` first.
- Use `?`/`orValue` optional chaining (`object.spec.template.spec.?initContainers.orValue([])`) for optional fields; plain `object.spec.x` on an absent field is a runtime error that triggers `failurePolicy`.
- Use `matchConditions` to exclude system users and controllers, otherwise a broken policy can block kube-system controllers.
- `paramKind` + `paramRef` for per-namespace configuration; `paramRef.parameterNotFoundAction` is required — `Deny` subjects requests without a matching param to the policy's `failurePolicy`, `Allow` lets them through. Choose deliberately.
- `string(...)` in **VAP** expressions is fine (VAP has a per-expression runtime cost limit, not the CRD's static estimate); the CRD `messageExpression` pitfall above is specific to CRD validation rules.

##### MutatingAdmissionPolicy — only where the cluster opted in

Verified on a 1.35.0 apiserver started with `--feature-gates=MutatingAdmissionPolicy=true --runtime-config=admissionregistration.k8s.io/v1beta1=true`; on a default 1.35 apiserver the kind does not exist (`the server doesn't have a resource type "mutatingadmissionpolicy"`).

```yaml
apiVersion: admissionregistration.k8s.io/v1beta1
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
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingAdmissionPolicyBinding
metadata:
  name: default-seccomp
spec:
  policyName: default-seccomp
  matchResources:
    namespaceSelector:
      matchLabels: {seccomp-default: "true"}
```

##### Admission webhooks (controller-runtime v0.23 typed API)

**[NEW in controller-runtime v0.23]** The webhook builder is generic: `ctrl.NewWebhookManagedBy(mgr, &cachev1.RedisCluster{})` takes the object as a second argument, and validators/defaulters receive concrete types (`admission.Validator[*cachev1.RedisCluster]`, `admission.Defaulter[*cachev1.RedisCluster]`). `WithCustomValidator`/`WithCustomDefaulter` and the `runtime.Object`-based `admission.CustomValidator`/`CustomDefaulter` interfaces are deprecated.[^10]

```go
type RedisClusterValidator struct{}

var _ admission.Validator[*cachev1.RedisCluster] = &RedisClusterValidator{}

func (v *RedisClusterValidator) ValidateCreate(ctx context.Context, rc *cachev1.RedisCluster) (admission.Warnings, error) {
	if rc.Spec.Tier == cachev1.TierStandard && rc.Spec.Replicas > 5 {
		return admission.Warnings{"standard tier is not tuned for more than 5 replicas"}, nil
	}
	return nil, nil
}

func (v *RedisClusterValidator) ValidateUpdate(ctx context.Context, oldRC, newRC *cachev1.RedisCluster) (admission.Warnings, error) {
	if oldRC.Spec.Tier == cachev1.TierPremium && newRC.Spec.Tier != cachev1.TierPremium {
		return nil, fmt.Errorf("downgrading from premium is not supported")
	}
	return v.ValidateCreate(ctx, newRC)
}

func (v *RedisClusterValidator) ValidateDelete(ctx context.Context, rc *cachev1.RedisCluster) (admission.Warnings, error) {
	return nil, nil
}

func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &cachev1.RedisCluster{}).
		WithValidator(&RedisClusterValidator{}).
		Complete()
}
```

```go
// BAD — pre-v0.23 shape; fails to compile against v0.23:
//   not enough arguments in call to ctrl.NewWebhookManagedBy
//       have (controllerruntime.Manager)
//       want (manager.Manager, T)
ctrl.NewWebhookManagedBy(mgr).For(&cachev1.RedisCluster{}).WithValidator(&RedisClusterValidator{}).Complete()
```

The rule from the hierarchy above still applies: the tier-downgrade check is a simple transition rule (`self.tier == oldSelf.tier || oldSelf.tier != 'premium'`, verified on 1.35 to reject premium → standard) and belongs in CEL; keep webhooks for checks that need external state.

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
- TLS certificates **must** be rotated; use cert-manager or a managed CA bundle injection.

---

#### RBAC & Least Privilege

##### Principles

1. **Least privilege** — grant only the verbs/resources explicitly required.
2. Prefer `Role` + `RoleBinding` (namespaced) over `ClusterRole` + `ClusterRoleBinding`.
3. **No wildcard verbs** (`*`) or **wildcard resources** (`*`).
4. **No `cluster-admin`** service accounts.
5. Mount service account tokens only when the workload needs API access (`automountServiceAccountToken: false` as default).
6. Treat `escalate`, `bind`, `impersonate`, `nodes/proxy`, `serviceaccounts/token` `create`, and `pods/exec` as privileged.

##### **[NEW in 1.35]** `pods/exec`, `pods/attach`, `pods/portforward` require `create` for WebSocket requests

With `AuthorizePodWebsocketUpgradeCreatePermission` (beta, on), the apiserver authorizes WebSocket upgrade requests for these subresources as `create`, matching SPDY. Before 1.35, a WebSocket `GET` needed only `get`. Verified with a raw WebSocket upgrade request from a user bound to `verbs: [get]` on `pods/exec`:

```text
1.34.1:  "pod web does not have a host assigned"   (authorization passed; failed later)
1.35.0:  pods "web" is forbidden: ...              (403 at authorization)
```

```yaml
# BAD on 1.35 — exec silently stops working for tooling that used WebSockets
rules:
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["get"]
# GOOD
rules:
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["create"]
```

Conversely, anyone who could exec with only `get` before is now blocked — audit custom "read-only" roles that accidentally relied on this.

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

// Events: controller-runtime v0.23 recorder writes events.k8s.io/v1
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

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

**[NEW in 1.35, alpha]** Constrained impersonation (`impersonate-on:<mode>:<verb>` verbs) is off by default; on 1.35 the `impersonate` verb is still all-or-nothing. Do not write RBAC that relies on the new verbs.

---

#### Workload Manifests: Pods

##### A production Deployment for 1.35 (verified)

Applied to a 1.35.0 apiserver into a namespace enforcing Pod Security `restricted:v1.35`; a Pod built from the template passes admission.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: payments
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: v1.35
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
        supplementalGroupsPolicy: Strict
        seccompProfile: {type: RuntimeDefault}
      topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: ScheduleAnyway
        labelSelector:
          matchLabels: {app.kubernetes.io/name: api}
        matchLabelKeys: [pod-template-hash]
      initContainers:
      - name: log-shipper                      # native sidecar (GA 1.33)
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
        - name: ZONE                           # filled by PodTopologyLabelsAdmission (1.35 beta)
          valueFrom:
            fieldRef: {fieldPath: "metadata.labels['topology.kubernetes.io/zone']"}
        - name: GOMAXPROCS
          valueFrom:
            resourceFieldRef: {resource: limits.cpu, divisor: "1"}
        resources:
          requests: {cpu: 250m, memory: 256Mi}
          limits: {cpu: "1", memory: 256Mi}
        resizePolicy:
        - {resourceName: cpu, restartPolicy: NotRequired}
        - {resourceName: memory, restartPolicy: RestartContainer}
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
            sleep: {seconds: 5}                # PodLifecycleSleepAction, GA 1.34
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities: {drop: [ALL]}
        volumeMounts:
        - {name: tmp, mountPath: /tmp}
      volumes:
      - name: tmp
        emptyDir: {sizeLimit: 64Mi}
```

A bare Pod in the same namespace without a security context is rejected: `violates PodSecurity "restricted:v1.35": allowPrivilegeEscalation != false ..., unrestricted capabilities ..., runAsNonRoot != true ..., seccompProfile ...`.

Review notes on this template:

- `GOMAXPROCS` from `limits.cpu`: since Go 1.25 the runtime reads the cgroup CPU limit itself (container-aware `GOMAXPROCS`), so for Go ≥ 1.25 binaries this env var is redundant and freezes the value (an in-place CPU resize will not update an env var; the Go runtime re-reads the limit periodically).[^11]
- `memory` requests = limits avoids OOM surprises for latency-critical services; `cpu` limits are a policy decision (throttling) — flag missing **memory** limits, not missing CPU limits.
- `matchLabelKeys: [pod-template-hash]` spreads each ReplicaSet revision independently. `MatchLabelKeysInPodTopologySpread` is still **beta** (on) in 1.35; since 1.34 the apiserver merges the keys into `labelSelector` at Pod creation.
- `maxUnavailable: 0` + `maxSurge: 1` with a PDB of `minAvailable: 2` and three replicas keeps two Pods serving during rollouts and node drains.

##### **[NEW in 1.35 — GA]** In-place Pod resize

CPU and memory `requests`/`limits` of a running Pod are changed through the `resize` subresource; the main Pod resource still rejects resource edits:

```bash
# GOOD — resize subresource with a strategic-merge patch (containers merge by name)
kubectl patch pod web --subresource resize \
  -p '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"300m"},"limits":{"cpu":"600m"}}}]}}'
# pod/web patched   (metadata.generation incremented)

# BAD — main resource: "pod updates may not change fields other than spec.containers[*].image, ..."
kubectl patch pod web -p '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"350m"}}}]}}'

# BAD — JSON merge patch replaces the whole containers list (and each container's
# resources), so any request/limit not repeated counts as removed:
#   "spec.containers[0].resources.requests: Forbidden: resource requests cannot be removed"
#   "spec.containers[0].resources.limits: Forbidden: resource limits cannot be removed"
kubectl patch pod web --subresource resize --type=merge -p '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"350m"}}}]}}'

# BAD — only cpu and memory are resizable:
#   "spec: Forbidden: only cpu and memory resources are mutable"
kubectl patch pod web --subresource resize -p '{"spec":{"containers":[{"name":"app","resources":{"requests":{"ephemeral-storage":"2Gi"}}}]}}'
```

Rules (verified where marked, otherwise from the GA announcement and docs):[^12]

- A resize must not change the Pod's **QoS class** (verified: `Pod QOS Class may not change as a result of resizing`). Guaranteed Pods must keep requests == limits.
- Requests/limits present at creation cannot be removed by a resize (verified).
- RBAC: resizing needs `patch`/`update` on `pods/resize`; VPA-like controllers need exactly that, not `pods` `update`.
- Progress is reported through Pod conditions `PodResizePending` (reasons `Deferred`, `Infeasible`) and `PodResizeInProgress` (reason `Error` on failure); `status.resize` is deprecated. `status.containerStatuses[].resources` shows what is actually applied. Wait for `status.observedGeneration >= metadata.generation` and the absence of both conditions.
- Memory-limit **decreases are allowed in 1.35** (prohibited in the 1.33/1.34 beta). The kubelet only applies them if current usage is below the new limit (best effort); otherwise the resize stays in progress. The apiserver does not police this: both v1.34.1 and v1.35.0 accept a `pods/resize` patch that lowers the memory limit (verified), so a successful patch is not proof that the node applied it — check the resize conditions. Use `resizePolicy` `RestartContainer` for memory when the runtime (JVM, Python) cannot shrink in place.
- Resize is not supported together with swap, the static CPU manager or static memory manager policies.
- Pod-level `spec.resources` (`PodLevelResources`, beta-on since 1.34) **cannot** be resized in place on 1.35 (`InPlacePodLevelResourcesVerticalScaling` is alpha).
- Changing `resources` in a Deployment/StatefulSet **template** still triggers a rollout; in-place resize is for direct Pod updates by autoscalers/operators.

##### **[NEW in 1.35 — beta, on]** Container restart rules

A container may override the Pod's restart policy and restart only on specific exit codes; this avoids rescheduling an entire `restartPolicy: Never` Pod for a retriable failure.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: trainer
spec:
  restartPolicy: Never
  containers:
  - name: trainer
    image: registry.example.com/trainer:1.4.2
    restartPolicy: Never          # required when restartPolicyRules is set
    restartPolicyRules:
    - action: Restart
      exitCodes:
        operator: In
        values: [42]              # retriable error code
```

Verified on 1.35: `restartPolicyRules` without a container `restartPolicy` → `must specify restartPolicy when restart rules are used`; `action: RestartAllContainers` → `Unsupported value ... supported values: "Restart"` (that action is alpha in 1.35, beta in 1.36). On 1.34 the apiserver silently **drops** both fields — a manifest written for 1.35 that reaches a 1.34 cluster loses its restart rules without an error.

##### **[NEW in 1.35 — beta, on]** Environment variables from files (`fileKeyRef`)

```yaml
spec:
  initContainers:
  - name: render-config
    image: registry.example.com/config-render:1.0
    command: ["/render", "--out=/config/config.env"]
    volumeMounts: [{name: config, mountPath: /config}]
  containers:
  - name: app
    image: registry.example.com/app:1.0
    env:
    - name: CONFIG_MODE
      valueFrom:
        fileKeyRef:
          volumeName: config        # an emptyDir volume
          path: config.env
          key: CONFIG_MODE
          # optional: false (defaulted)
  volumes:
  - name: config
    emptyDir: {}
```

The file is read by the kubelet when the container starts; per the 1.35 release note, the file syntax is a restricted subset of POSIX shell syntax in which **all values must be single-quoted** (`CONFIG_MODE='strict'`). A 1.34 apiserver rejects the field (`must specify one of: fieldRef, resourceFieldRef, configMapKeyRef or secretKeyRef`). Do not use it for secrets: the values become ordinary environment variables visible in `/proc/<pid>/environ`.[^3]

##### **[NEW in 1.35 — beta, on]** Image volumes

```yaml
volumes:
- name: model
  image:
    reference: registry.example.com/models/resnet@sha256:...   # pin by digest
    pullPolicy: IfNotPresent
containers:
- name: server
  volumeMounts:
  - {name: model, mountPath: /models, readOnly: true}
```

Accepted by a default 1.35 apiserver. A default 1.34 apiserver (gate beta/off) silently drops the **whole volume and every `volumeMount` that references it** (verified on v1.34.1: the Pod is created without the `model` volume and without the `/models` mount), so the container starts without its data instead of failing admission. Requires container-runtime support (the release notes name containerd v2.1 or later); on an older runtime the Pod fails at container creation, not at admission. Image volumes are always read-only, honour `imagePullSecrets`, and — with `KubeletEnsureSecretPulledImages` — are subject to the same cached-image credential checks.

##### **[NEW in 1.35 — beta, on]** `hostnameOverride`

`spec.hostnameOverride` sets the Pod's hostname to any RFC 1123 subdomain (it is not added to cluster DNS). Verified: it is rejected together with `setHostnameAsFQDN: true` (`may not be specified when setHostnameAsFQDN is true`) and with `hostNetwork: true`. Dropped by a 1.34 apiserver.

##### **[GA in 1.35]** `supplementalGroupsPolicy`

`securityContext.supplementalGroupsPolicy: Strict` attaches only `runAsGroup`, `fsGroup` and `supplementalGroups` to the container process, ignoring groups defined in the image's `/etc/group`. Prefer `Strict` for hardened workloads; `Merge` (default) keeps the historical behaviour. The kubelet reports the effective identity in `status.containerStatuses[].user.linux`. Nodes whose runtime lacks support reject `Strict` Pods at the kubelet.

##### Pod-level resources (beta since 1.34)

```yaml
spec:
  resources:                 # pod-level budget shared by all containers
    limits: {cpu: "1", memory: 512Mi}
  containers:
  - name: app
    image: registry.example.com/app:1
```

Verified on 1.35: the apiserver defaults pod-level `requests` from `limits`; other resources are rejected (`spec.resources.limits[ephemeral-storage]: Unsupported value ... supported values: "cpu", "hugepages-", "memory"`). Not resizable in place on 1.35.

##### Other Pod rules

- **Sidecars** are `initContainers` with `restartPolicy: Always` (GA 1.33). They start before regular containers, keep running, and are terminated after them; a sidecar's `startupProbe` gates the next init container. Do not model sidecars as extra regular containers in Jobs — the Job never completes.
- **`lifecycle.preStop.sleep`** (GA 1.34) replaces `exec: ["sleep","5"]` in images without a shell; `seconds: 0` is valid.
- **`lifecycle.stopSignal`** exists in the schema but is alpha (`ContainerStopSignals`) and **dropped** on 1.35 — handle `SIGTERM` in the application instead.
- **User namespaces** (`hostUsers: false`, beta-on since 1.33) map container root to an unprivileged host UID; requires idmap-mount support in the node kernel and filesystems plus runtime support; on unsupported nodes the Pod fails at the kubelet. Recommended for workloads that need root inside the container.
- **`IfNotPresent` and private images**: with `KubeletEnsureSecretPulledImages` (beta, on in 1.35; kubelet policy default `NeverVerifyPreloadedImages`), a Pod can only use a cached private image if it presents credentials that could pull it. Workloads that silently relied on another tenant's pull secret now fail with an image pull error — add the pull secret, do not switch to `imagePullPolicy: Never`.
- **`envFrom`/`env` from Secrets** containing non-UTF-8 bytes regressed in 1.34 and was fixed in v1.35.6 — clusters on v1.35.0–v1.35.5 may fail to start such containers.[^3]

---

#### Workload Manifests: Controllers (Deployment, StatefulSet, Job, HPA)

##### Deployments and ReplicaSets

- **[NEW in 1.35 — beta, on]** `status.terminatingReplicas` counts Pods with a deletion timestamp that still exist. Rollout tooling that must wait until old Pods are really gone (licence-limited software, exclusive volumes) should check `terminatingReplicas == 0` in addition to `updatedReplicas`/`availableReplicas`. The field is absent (nil) when the gate is off — treat nil as "unknown", not zero.
- `spec.selector` is immutable; changing labels in the selector requires a new Deployment.
- `progressDeadlineSeconds` only marks the `Progressing` condition `False`; it does not roll back.

##### StatefulSets — `maxUnavailable` depends on the patch release

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: kv
spec:
  serviceName: kv
  replicas: 5
  podManagementPolicy: Parallel
  selector: {matchLabels: {app: kv}}
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 2          # beta field
  template:
    metadata: {labels: {app: kv}}
    spec:
      containers:
      - name: kv
        image: registry.example.com/kv:3.1
```

Verified by `--dry-run=server -o yaml`: the v1.35.0 and v1.37.0 apiservers keep `maxUnavailable: 2`; v1.34.1 (gate alpha/off) and v1.36.2 (gate beta/off) **drop** it silently (the stored `rollingUpdate` is just `{partition: 0}`). Inside the 1.35 line the default changed: v1.35.4 "Fixes a 1.35 regression in StatefulSet Parallel pod management by disabling the MaxUnavailableStatefulSet feature by default" (kubernetes#137926, the 1.35 backport of #137904 that also ships in 1.36.0). With the gate off, a v1.35.4+ apiserver drops the field exactly like 1.36 does — expect permanent GitOps drift on that field. The actual bug fix (with `Parallel`, unavailable Pods of an *older* revision were counted toward the `maxUnavailable` budget, kubernetes#137666) and the re-enabled default only ship in 1.37; the 1.35 changelog contains neither.[^3][^8][^9] (Only v1.35.0 binaries were available for execution; the v1.35.4+ behaviour is taken from the changelog.)

Review rule: on 1.35 treat `rollingUpdate.maxUnavailable` as **unreliable** — it is honoured only on v1.35.0–v1.35.3 (which carry the Parallel-management regression) or where the gate is explicitly enabled; on v1.35.4+ the field is dropped and updates proceed one Pod at a time. Never rely on it for correctness (for example to finish an update within a maintenance window), and remove it from manifests that target v1.35.4+ unless the cluster enables `MaxUnavailableStatefulSet`.

Other StatefulSet rules:

- `persistentVolumeClaimRetentionPolicy` (GA 1.32): set `whenDeleted: Delete` only for disposable data; the default `Retain` keeps PVCs.
- `ordinals.start` (GA 1.31) for slicing a StatefulSet across clusters/migrations.
- `serviceName` must reference an existing headless Service for stable DNS.
- A v1.34 controller-manager regression caused spurious rollouts of existing StatefulSets after a 1.33 → 1.34 upgrade; 1.35 fixes it behind `StatefulSetSemanticRevisionComparison` (beta, on).[^3]

##### Jobs

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: shard-train
spec:
  managedBy: kueue.x-k8s.io/multikueue   # GA in 1.35; immutable
  completionMode: Indexed
  completions: 8
  parallelism: 8
  backoffLimitPerIndex: 2
  maxFailedIndexes: 2
  podReplacementPolicy: Failed
  successPolicy:
    rules:
    - succeededIndexes: "0"
  podFailurePolicy:
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

Accepted by both 1.34 (where `JobManagedBy` was already beta-on) and 1.35.

- **`managedBy`** (GA 1.35, immutable — verified `spec.managedBy: ... field is immutable`): when set to anything other than `kubernetes.io/job-controller`, the built-in Job controller ignores the Job entirely — no Pods are created unless the named controller exists. Flag `managedBy` in manifests that are not part of a MultiKueue (or similar) setup.
- `podFailurePolicy` requires `restartPolicy: Never`.
- `backoffLimitPerIndex` requires `completionMode: Indexed`.
- **Mutating resources of a suspended Job** (`MutablePodResourcesForSuspendedJobs`) and **scheduling directives** are alpha/off in 1.35: verified, editing `spec.template.spec.containers[].resources` of a suspended Job is rejected (`spec.template: Invalid value: ...: field is immutable`); delete and recreate the Job.
- For CronJobs, always set `timeZone` (GA 1.27) and `concurrencyPolicy` explicitly; `startingDeadlineSeconds` avoids catch-up storms after controller downtime.

##### HorizontalPodAutoscaler — **[NEW in 1.35 — beta, on]** per-direction tolerance

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
  behavior:
    scaleUp:
      tolerance: "0.05"          # react to a 5% deviation
    scaleDown:
      tolerance: "0.15"
      stabilizationWindowSeconds: 300
```

- On 1.35 the apiserver stores the tolerances as canonical quantities — `0.05` becomes `50m` and `0.15` becomes `150m` (verified). GitOps tools that compare the live object with the manifest will report a permanent diff unless they normalise quantities (Argo CD/Flux SSA-based diffs do); write `50m`/`150m` to avoid the noise.
- On 1.34 the field is dropped (gate alpha/off), and the global 10% tolerance applies.
- Never set `tolerance: 0`; tiny metric noise would cause constant scaling.
- Do not set `spec.replicas` in a Deployment managed by an HPA when applying with SSA or `kubectl apply` — every apply resets the replica count. Omit the field.

---

#### Networking: Services & NetworkPolicy

##### **[GA in 1.35]** `trafficDistribution`

```yaml
apiVersion: v1
kind: Service
metadata:
  name: node-local-cache
spec:
  selector: {app: cache}
  ports: [{name: redis, port: 6379, targetPort: 6379}]
  trafficDistribution: PreferSameNode    # GA 1.35 (beta-on in 1.34)
---
apiVersion: v1
kind: Service
metadata:
  name: zonal-api
spec:
  selector: {app: api}
  ports: [{name: http, port: 80, targetPort: 8080}]
  trafficDistribution: PreferSameZone    # GA 1.35; replaces PreferClose
```

- `PreferClose` still works but returns `Warning: spec.trafficDistribution: "PreferClose" is deprecated; use "PreferSameZone"` on 1.35 (no warning on 1.34). Replace it in manifests that target ≥ 1.34; keep `PreferClose` only when the chart must also install on ≤ 1.33.
- These are *preferences*: kube-proxy falls back to remote endpoints when no local/zonal endpoint is ready. `PreferSameNode` suits node-local caches/DaemonSets; for most Deployments `PreferSameZone` gives cost savings without hot-spotting a single node.
- Do not combine `trafficDistribution` with the `service.kubernetes.io/topology-mode: Auto` annotation; topology-aware routing hints take precedence and the combination is confusing.
- `internalTrafficPolicy: Local` is a hard requirement (no fallback), not a preference — flag it where availability matters.

##### Service names, IPs and `externalIPs`

- Service names must be RFC 1035 labels on 1.35 (no leading digit) — see Naming.
- Use canonical IP literals. 1.35 warns on `010.000.000.005` (`non-standard IP address ... will be considered invalid in a future Kubernetes release: use "10.0.0.5"`); 1.36 rejects it.
- `spec.externalIPs` lets any user who can create Services intercept traffic for arbitrary IPs (CVE-2020-8554). On 1.35 it works without warning; 1.36 deprecates it. Clusters should block it with a ValidatingAdmissionPolicy (`!has(object.spec.externalIPs)`) or the `DenyServiceExternalIPs` admission plugin; new manifests should use `LoadBalancer`, `NodePort` or Gateway API.

##### Ingress

Ingress NGINX is retired (best-effort maintenance ended March 2026). New HTTP routing should target Gateway API (`gateway.networking.k8s.io/v1`, installed as CRDs, not built into the apiserver). Flag new uses of `nginx.ingress.kubernetes.io/*` annotations (the controller no longer receives fixes), especially `*-snippet` annotations, which inject raw NGINX configuration.[^1]

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
    supplementalGroupsPolicy: Strict
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
pod-security.kubernetes.io/enforce-version: v1.35
pod-security.kubernetes.io/warn: restricted
pod-security.kubernetes.io/audit: restricted
```

##### Secrets Management

- Never embed secrets in CRD `spec` fields; reference them (`secretRef: {name: ...}`).
- Do not log secret values, and do not put them in Event notes or condition messages.
- Use projected service-account tokens with an `audience` and short `expirationSeconds` instead of long-lived token Secrets (`kubernetes.io/service-account-token` Secrets are no longer auto-created since 1.24).
- Pod certificates (`podCertificate` projected volume, 1.35 beta) would remove the need for cert-manager/SPIFFE sidecars, but the API is **off by default** on 1.35 — keep existing mechanisms unless the cluster explicitly enables `PodCertificateRequest` and `certificates.k8s.io/v1beta1`.
- CSI drivers that need workload tokens should set `CSIDriver.spec.serviceAccountTokenInSecrets: true` (1.35 beta, on) so tokens arrive in the `secrets` field of `NodePublishVolume` instead of `volume_context`, which drivers commonly log.
- Consider External Secrets Operator or Vault Agent for rotation.

##### Image Security

- Pin images by **digest** in production manifests; enforce it with a ValidatingAdmissionPolicy (see above).
- Use distroless or scratch base images for operator binaries.
- Scan images (Trivy, Grype) in CI; never use `latest`.
- Private images on shared nodes: rely on `KubeletEnsureSecretPulledImages` (1.35 beta, on) — node policy `imagePullCredentialsVerificationPolicy` (`NeverVerify`, `NeverVerifyPreloadedImages` (default), `NeverVerifyAllowlistedImages`, `AlwaysVerify`) controls how strict the check is.

##### Node-level 1.35 changes reviewers will meet in infrastructure code

- kubelet configuration: `failCgroupV1` defaults to `true` (cgroup v1 nodes fail); `crashLoopBackOff.maxContainerRestartPeriod` (1–300 s, beta-on) lowers the CrashLoopBackOff cap per node; drop-in directory `--config-dir` is GA.
- kubeadm/kubelet flags: remove `--pod-infra-container-image` before upgrading kubelets to 1.35 (startup fails otherwise).
- kube-proxy: `mode: ipvs` logs a deprecation warning; plan nftables.
- containerd: 1.7 still works on 1.35 but not on 1.36; check the `kubelet_cri_losing_support` metric.[^1]

---

#### client-go & apimachinery in v0.35

##### **[NEW in 1.35]** Comparable resource versions

Since 1.35 every in-tree resource version is a decimal integer that clients may compare (KEP-5504). apimachinery v0.35 adds a helper:[^13]

```go
import "k8s.io/apimachinery/pkg/util/resourceversion"

// -1 if a < b, 0 if equal, +1 if a > b; error if either is not a well-formed
// positive integer without leading zeros (e.g. "" or "0").
cmp, err := resourceversion.CompareResourceVersion(cached.GetResourceVersion(), fresh.GetResourceVersion())
if err != nil {
	return err // e.g. an aggregated API server with opaque resource versions
}
if cmp < 0 {
	// cache is behind the object we just wrote: requeue instead of acting on stale data
}
```

Rules:

- Only compare resource versions **of the same resource** (same group/resource); values from different resources share no ordering guarantee.
- Aggregated API servers and older clusters may still return opaque strings — handle the error path; never `strconv.Atoi` + compare by hand.
- Do not compare resource versions to decide *whether an object changed* in a predicate when `generation` is the right signal.

##### Informers and caches

- `cache.WaitForNamedCacheSyncWithContext(ctx, synced...)` replaces `WaitForNamedCacheSync(name, stopCh, synced...)` for contextual logging (the controller name comes from the logger in `ctx`).
- `WatchListClient` is beta and **on** in client-go v0.35: informers open a streaming watch-list instead of a paginated LIST when the server supports it (1.35 servers do). Custom `ListWatch` implementations that rely on `ListFunc` being called must cope with that; disable via `KUBE_FEATURE_WatchListClient=false` only as a workaround.
- client-go v0.35 fixes a 1.34 regression where informers ignored configured `TransformFunc`s; if you strip `managedFields` with a transform to save memory, use ≥ v0.35.0.[^3]
- Prefer controller-runtime's cache options (`DefaultTransform: cache.TransformStripManagedFields()`, `ByObject` label/field selectors) to reduce memory instead of hand-rolled informers.

##### **[NEW in 1.35]** API types no longer implement `ProtoMessage()`

`k8s.io/api` types dropped the gogo-protobuf runtime registration; the `ProtoMessage()` marker methods are only compiled with the build tag `kubernetes_protomessage_one_more_release` in v0.35 and are removed in v0.36.[^3]

```go
// BAD — compiles on v0.34, fails on v0.35 without the build tag:
//   *"k8s.io/api/core/v1".Pod does not implement interface{ProtoMessage()} (missing method ProtoMessage)
var _ interface{ ProtoMessage() } = &corev1.Pod{}

// GOOD — encode API objects to protobuf with the apimachinery serializer
import "k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
s := protobuf.NewSerializer(scheme, scheme)
err := s.Encode(pod, &buf)
```

Never pass Kubernetes API structs to `google.golang.org/protobuf` (`proto.Marshal`) — they were never valid `google.golang.org/protobuf` messages.

##### Fake clients

- `k8s.io/client-go/kubernetes/fake.NewClientset` (field tracking, supports server-side apply) supersedes `NewSimpleClientset`; prefer it in new tests.
- controller-runtime's fake client needs `WithStatusSubresource(&MyType{})` (since v0.15) when the code writes status; otherwise `Status().Update/Patch` returns NotFound. Since v0.23 the fake client's SSA create also sets `resourceVersion`.
- Fakes do not run admission, CEL validation, defaulting or garbage collection. Anything that depends on those belongs in an envtest test.

##### Clients and versions

- Keep `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` and controller-runtime on matching minors. A `go.mod` with `k8s.io/client-go v0.35.x` and `k8s.io/api v0.34.x` (or controller-runtime v0.22 with v0.35 libraries) is a review finding.
- Generated clientsets / listers / apply configurations for your CRDs must be regenerated with code-generator v0.35 when the libraries move to v0.35 (the generated `zz_generated.model_name.go`/`OpenAPIModelName()` files are new in 1.35 for in-tree types).
- `k8s.io/utils/pointer` is deprecated in favour of `k8s.io/utils/ptr` (`ptr.To(x)`, `ptr.Deref(p, def)`).

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
- controller-runtime already exports `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds`, workqueue metrics, and (new in v0.23) `controller_runtime_reconcile_timeouts_total` — do not duplicate them.

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

See [Events (events.k8s.io)](#events-eventsk8sio). Rules:

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
- **Never** return a non-nil error together with `RequeueAfter`/`Requeue` — the error wins and controller-runtime logs a warning; choose one.
- Do not use `Requeue: true` (deprecated); use `RequeueAfter` or return an error.
- Use `client.IgnoreNotFound(err)` when fetching objects that may not exist; use `apierrors.IsConflict(err)` → return the error (requeue) rather than retry loops inside Reconcile.
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

##### Unit Tests (fake client)

```go
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
	rec := events.NewFakeRecorder(10) // k8s.io/client-go/tools/events
	r := &RedisClusterReconciler{Client: c, Scheme: scheme, Recorder: rec}

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

The controller-runtime fake client supports `client.Apply` (SSA), but it does not run CEL rules, defaulting, admission or GC.

##### Integration Tests (envtest) — pin the 1.35 binaries

```go
env := &envtest.Environment{
	CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd")},
	ErrorIfCRDPathMissing: true,
	// BinaryAssetsDirectory or KUBEBUILDER_ASSETS from `setup-envtest use 1.35.x -p path`
}
cfg, err := env.Start()
```

- Pin envtest to the cluster minor you ship to (`setup-envtest use 1.35.x`). controller-runtime v0.23's `setup-envtest` picks the **newest** Kubernetes version by default — on 2026-10-22 that is 1.37, which accepts manifests and fields a 1.35 cluster rejects or drops.
- envtest has no kube-controller-manager: no garbage collection, no Deployment/ReplicaSet controllers, no namespace deletion, no default ServiceAccount. Tests must fake child status (e.g. update `Deployment.status.readyReplicas`) and must not expect owner-reference cascades.
- envtest is the place to test CEL rules, webhooks (with `WebhookInstallOptions`) and SSA ownership.

##### e2e Tests (kind / k3s)

- Use a node image of the target minor (`kindest/node:v1.35.x`).
- Deploy the full operator image; test the complete lifecycle: Create → Reconcile → Update → Delete with finalizer.
- Use `kubectl wait --for=condition=Ready` assertions, and `--for=jsonpath='{.status.observedGeneration}'=N` for generation checks.

##### Testing Rules

- Test idempotency: run `Reconcile` twice and assert the same outcome.
- Test the deletion path including finalizer removal.
- Test that status is updated correctly on both success and error paths.
- Test that invalid CRs are rejected by CEL rules/webhooks (`apierrors.IsInvalid`).
- Run manifests through `kubectl apply --dry-run=server` against a 1.35 apiserver in CI (envtest binaries or kind) — client-side schema validation does not catch dropped gated fields, Pod Security admission, or CEL cost errors.

---

#### kubectl 1.35 and Manifest Tooling

- **[NEW in 1.35]** `kubectl get -o kyaml` is on by default (`KUBECTL_KYAML=false` disables it). Verified: kubectl v1.34.1 rejects `-o kyaml` unless `KUBECTL_KYAML=true`. KYAML output quotes every string, so YAML 1.1 coercion cannot bite. Any KYAML document is valid YAML input for older kubectl.
- YAML coercion is a real manifest bug, not a style issue. Verified: `metadata: {name: n}` fails with `json: cannot unmarshal bool into Go struct field ObjectMeta.metadata.name of type string` because kubectl's YAML parser reads unquoted `n` as a boolean. Quote values such as `y`, `n`, `yes`, `no`, `on`, `off`, `true`, `false`, version strings (`"1.10"`), and octal-looking numbers in labels, annotations, env values and ConfigMap data.
- **`kubectl kuberc` is alpha in 1.35**: the commands are `kubectl alpha kuberc view|set` (the top-level `kubectl kuberc` exists from 1.36). The kuberc `Preference` file (`kubectl.config.k8s.io/v1beta1`) gained `credentialPluginPolicy` (`AllowAll`, `DenyAll`, `Allowlist`) and `credentialPluginAllowlist`; on 1.35 an allowlist entry uses the key **`name`** (verified; 1.36 renames it to `command`, and kubectl 1.35 rejects `command` with `unknown field` and then `misconfigured credential plugin allowlist: empty allowlist entry #1`).
  ```yaml
  apiVersion: kubectl.config.k8s.io/v1beta1
  kind: Preference
  credentialPluginPolicy: Allowlist
  credentialPluginAllowlist:
  - name: /usr/local/bin/kubelogin
  defaults:
  - command: apply
    options:
    - name: server-side
      default: "true"
  ```
- `kubectl exec POD CMD` without `--` is rejected (`exec [POD] [COMMAND] is not supported anymore`) — already true for kubectl 1.32–1.34 clients; scripts must use `kubectl exec POD -- CMD`.
- `kubectl explain` shows alpha fields that the server will drop; use `--dry-run=server -o yaml` to prove a field survives.
- `--subresource` is available on `get`, `patch`, `edit`, `replace` and (with `--server-side`) `apply`; verified with `kubectl patch pod web --subresource resize -p ...` for in-place resize.
- Helm: declare `kubeVersion` in `Chart.yaml` when a chart depends on 1.35 behaviour, and gate optional 1.35-only fields with `semverCompare` on `.Capabilities.KubeVersion.Version`. `helm template` without `--kube-version` renders for a built-in default, not your cluster — verified: Helm v3.22.1 assumes `v1.37.0`, Helm v4.0.5 assumes `v1.34.0` (so `kubeVersion: ">=1.35.0-0"` fails with `chart requires kubeVersion ... incompatible with Kubernetes v1.34.0`). Always pass `--kube-version 1.35.x` in CI renders. Helm v3 strips vendor suffixes (`v1.35.4-eks-abc` → `v1.35.4`) in `.Capabilities.KubeVersion.Version`; Helm v4.3 keeps them.

---

#### Removed / Changed in 1.35 — Migration Notes

| Change | Who is affected | Action |
|---|---|---|
| `pods/exec`, `pods/attach`, `pods/portforward` need `create` over WebSockets | Roles granting only `get` (debug tooling, CI) | Grant `create`; audit "read-only" roles that unintentionally allowed exec |
| `PreferClose` deprecated | Services with `trafficDistribution: PreferClose` | Use `PreferSameZone` (needs ≥ 1.34 cluster) |
| controller-runtime v0.23: events.k8s.io recorder | Operators using `GetEventRecorderFor` | Switch to `GetEventRecorder` + `events.EventRecorder.Eventf(regarding, related, type, reason, action, note, ...)`; add `events.k8s.io` RBAC |
| controller-runtime v0.23: generic webhooks | `NewWebhookManagedBy(mgr).For(obj)` / `CustomValidator` | `NewWebhookManagedBy(mgr, obj).WithValidator(...)` with typed `Validator[T]` |
| controller-runtime v0.23: priority queue default | Controllers assuming FIFO | Remove ordering assumptions; opt out with `UsePriorityQueue: ptr.To(false)` only if needed |
| `ProtoMessage()` removed from API types | Code treating API structs as protobuf messages | Use apimachinery's protobuf serializer; build tag `kubernetes_protomessage_one_more_release` is a one-release escape hatch |
| `storagemigration.k8s.io/v1alpha1` removed | Clusters with alpha migrations | Delete `v1alpha1` objects before upgrading; `v1beta1` is opt-in |
| `storage.k8s.io/v1alpha1` VolumeAttributesClass removed | Manifests still on alpha | `storage.k8s.io/v1` |
| `failCgroupV1: true` default | cgroup v1 nodes | Migrate to cgroup v2 or set `failCgroupV1: false` temporarily |
| `--pod-infra-container-image` removed | kubelet flags / kubeadm `extraArgs` | Remove the flag before upgrading kubelets |
| kube-proxy ipvs deprecated | `KubeProxyConfiguration.mode: ipvs` | Plan nftables |
| containerd 1.7 last supported | Nodes on containerd 1.x | Upgrade to containerd 2.x before 1.36 |
| `MaxUnavailableStatefulSet` off again from v1.35.4 | StatefulSets using `rollingUpdate.maxUnavailable` | On ≥ v1.35.4 the apiserver drops the field (verified for the same gate state on 1.36.2); expect one-at-a-time updates and remove the field to avoid GitOps drift |
| `SchedulerAsyncAPICalls` disabled in v1.35.1 | Scheduler performance regression in v1.35.0 | Run ≥ v1.35.1 |
| Pod resize limited to CPU/memory | Tools resizing other resources | Recreate Pods for other resource changes |
| Kubelet credential verification for cached images | Pods relying on images pulled with someone else's secret | Add `imagePullSecrets` to every Pod that uses a private image |
| `AllAlpha=true` without dependencies | Test clusters | Also enable `AllBeta=true` or the specific beta gates |

---

#### Production / Review Checklist

##### Version & API availability

- [ ] Manifests use only API versions served by a default 1.35 apiserver (no `v1beta1` MutatingAdmissionPolicy, StorageVersionMigration, PodCertificateRequest without documented cluster opt-in)
- [ ] No 1.36+ fields or behaviour assumed (Service names starting with digits, MutatingAdmissionPolicy `v1`, StatefulSet `Recreate`, `httpGet.protocol`, `emptyDir.mode`, …)
- [ ] Gated 1.35 fields checked against the cluster's actual gates and patch level (`rollingUpdate.maxUnavailable` off from v1.35.4)
- [ ] `go.mod`: all `k8s.io/*` on the same v0.35.x patch, controller-runtime v0.23.x, Go ≥ 1.25
- [ ] envtest/kind pinned to 1.35, not "latest"

##### CRD Design

- [ ] Kind: CamelCase singular; Resource: lowercase plural; group is a DNS subdomain you own (not `*.k8s.io`)
- [ ] `// +kubebuilder:subresource:status`; `status.conditions` (`listType=map`, key `type`) + `status.observedGeneration`
- [ ] All enum fields use string type aliases (not `bool`); no floats in `spec`
- [ ] Every string/array/map reachable from CEL has `maxLength`/`maxItems`/`maxProperties`
- [ ] Numeric fields have `minimum`/`maximum` (1.35 does not enforce `format: int32` ranges)
- [ ] CEL `messageExpression` uses `'%d'.format([...])`, not `string(int)`; transition rules handle removal of the field
- [ ] Rules use `fieldPath` and `reason`; `optionalOldSelf` only with `oldSelf.hasValue()`
- [ ] At least one printer column with state info; `selectableFields` for commonly filtered fields
- [ ] `omitempty` + `// +optional` on all optional fields

##### Controller

- [ ] Fetch → `IgnoreNotFound` → deletion/finalizer → converge → status
- [ ] Children applied with SSA apply configurations and a constant field owner (or patched), not `Update` from cached objects
- [ ] Status written through `Status().Patch`/`Status().Apply`; `observedGeneration` set; conditions via `meta.SetStatusCondition`
- [ ] Transient errors returned; permanent errors → condition + Warning Event + `nil`/`TerminalError`; no `Requeue: true`; no error + `RequeueAfter`
- [ ] Event recorder from `GetEventRecorder` with `events.k8s.io` RBAC; Events only on transitions; `action` set; no user text as format string
- [ ] Webhooks use the typed v0.23 builder; CEL preferred where possible
- [ ] Watches on shared objects use field indexes and restricted caches
- [ ] Metrics served securely; leader election enabled

##### Workloads

- [ ] Pod Security `restricted` passes (runAsNonRoot, seccomp `RuntimeDefault`, drop `ALL`, no privilege escalation); `enforce-version` pinned
- [ ] Images pinned by digest; private images have `imagePullSecrets` on every Pod
- [ ] Memory requests/limits set; resize via `pods/resize` only for CPU/memory and without QoS change
- [ ] Probes: startup + readiness + liveness with sane thresholds; `preStop.sleep` instead of shell sleeps
- [ ] Sidecars as `initContainers` with `restartPolicy: Always`
- [ ] Container `restartPolicyRules` always paired with a container `restartPolicy`
- [ ] Services use `PreferSameZone`/`PreferSameNode`, not `PreferClose`; no `externalIPs`
- [ ] HPA-managed Deployments do not set `spec.replicas` in applied manifests; HPA tolerances written as canonical quantities
- [ ] Job `managedBy` only when an external Job controller exists
- [ ] Unquoted YAML scalars that coerce (`n`, `yes`, `on`, `1.10`) are quoted

##### Security / RBAC

- [ ] No wildcard verbs/resources; namespace-scoped roles where possible
- [ ] `pods/exec`/`attach`/`portforward` granted with `create` (1.35 WebSocket authorization), and only to those who need it
- [ ] Service account tokens not automounted unless needed
- [ ] Secrets referenced by name — never inlined in CRD spec, logs, Events or conditions
- [ ] Default-deny NetworkPolicy plus explicit allows (including DNS)
- [ ] Admission webhooks exclude their own namespace and `kube-system`; `sideEffects: None`; timeouts ≤ 10 s

##### Finalizers & Versioning

- [ ] Finalizer name is fully qualified (`domain.com/name`), added before external resources are created, removed only after successful idempotent cleanup
- [ ] New CRD fields optional; no fields removed or renamed in served versions
- [ ] `deprecationWarning` set on deprecated CRD versions; conversion webhook when schemas diverge
- [ ] `storage: true` on exactly one version; stored objects migrated and `status.storedVersions` cleaned before dropping a version

---

[^1]: Kubernetes v1.35: Timbernetes (The World Tree Release) — release announcement, 2025-12-17. https://kubernetes.io/blog/2025/12/17/kubernetes-v1-35-release/
[^2]: Kubernetes release schedule and patch releases (data/releases/schedule.yaml: 1.35 release date, patch dates, maintenance mode and end-of-life dates); GitHub release v1.35.9. https://kubernetes.io/releases/patch-releases/ , https://github.com/kubernetes/kubernetes/releases/tag/v1.35.9
[^3]: CHANGELOG-1.35 (v1.35.0 urgent upgrade notes, API changes, features, deprecations; patch notes v1.35.1–v1.35.9). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md
[^4]: Go module proxy listings for k8s.io/client-go v0.35.x, sigs.k8s.io/controller-runtime v0.23.x (go.mod requires k8s.io/* v0.35.0, go 1.25.0), sigs.k8s.io/controller-tools v0.20.x. https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3 , https://pkg.go.dev/k8s.io/client-go@v0.35.9
[^5]: Kubebuilder v4.11–v4.16 scaffold versions (pkg/plugins/golang/v4/scaffolds/init.go: ControllerRuntimeVersion v0.23.x, ControllerToolsVersion v0.20.x). https://github.com/kubernetes-sigs/kubebuilder/releases
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
[^7]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^8]: Kubernetes v1.36 release announcement (2026-04-22) and CHANGELOG-1.36. https://kubernetes.io/blog/2026/04/22/kubernetes-v1-36-release/ , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.36.md
[^9]: Kubernetes v1.37 release announcement (2026-08-26) and CHANGELOG-1.37. https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/ , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.37.md
[^10]: controller-runtime v0.23.0 release notes (events API migration, generic webhooks, priority queue default, subresource apply, FieldOwner/FieldValidation client options). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.23.0
[^11]: Go 1.25 release notes — container-aware GOMAXPROCS. https://go.dev/doc/go1.25#container-aware-gomaxprocs
[^12]: Kubernetes 1.35: In-Place Pod Resize Graduates to Stable (2025-12-19); Resize CPU and Memory Resources assigned to Containers. https://kubernetes.io/blog/2025/12/19/kubernetes-v1-35-in-place-pod-resize-ga/ , https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/
[^13]: KEP-5504: Comparable resource versions; apimachinery `pkg/util/resourceversion`. https://github.com/kubernetes/enhancements/issues/5504 , https://pkg.go.dev/k8s.io/apimachinery/pkg/util/resourceversion
[^14]: Kubernetes API reference: CustomResourceDefinition validation rules (CEL), validation ratcheting, selectable fields. https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
[^15]: Validating Admission Policy and Mutating Admission Policy documentation. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/ , https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/
