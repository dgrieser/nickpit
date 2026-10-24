### Kubernetes 1.34 — Manifests, CRDs and Go Controllers

> **Version**: Kubernetes v1.34 ("Of Wind & Will", O' WaW), released **2025-08-27**. Latest patch: **v1.34.12, published 2026-09-23** (it was scheduled for 2026-09-15; the October patch v1.34.13, scheduled for 2026-10-13, has not been tagged as of late October 2026).
> **Support status (October 2026)**: 1.34 entered **maintenance mode on 2026-08-27** and reaches **end of life on 2026-10-27**. After that date no further patches are planned. The newest minor is **1.37** (released 2026-08-26, latest patch v1.37.1 on 2026-09-23); 1.36 (2026-04-22) and 1.35 (2025-12-17) are also supported; 1.38 is at alpha (v1.38.0-alpha.2).
> **Matching libraries**: `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`, `k8s.io/apiextensions-apiserver` **v0.34.x** (latest v0.34.12, 2026-09-23); **controller-runtime v0.22.x** (latest v0.22.5, 2026-01-19); **controller-tools (controller-gen) v0.19.x**; Kubebuilder **v4.9.x/v4.10.x** scaffolds controller-runtime v0.22 and controller-tools v0.19.
> **Scope**: Kubernetes YAML manifests and Helm-rendered output applied to a 1.34 cluster, CustomResourceDefinitions with CEL validation, admission policies, and Go operators/controllers (controller-runtime, client-go, Kubebuilder) built against `k8s.io/*` v0.34.x.
> **Note**: This document is self-contained. It carries forward the general operator, CRD, RBAC, security and testing rules and adds the 1.34-specific layer. Sections marked **[NEW in 1.34]** cover behaviour that is new or changed in this release. Server-side behaviour in this guide was checked against a real kube-apiserver v1.34.1 (envtest build) with boundary checks on v1.33.0 and v1.35.0; Go snippets compile against k8s.io/* v0.34.12 and controller-runtime v0.22.5. Pod *runtime* behaviour (kubelet, scheduler, container runtime) is sourced from the release notes and documentation, not executed.

---

#### What's New in 1.34

Kubernetes 1.34 contains 58 enhancements: 23 graduated to stable, 22 entered beta, 13 entered alpha. The changes that matter to people writing manifests, CRDs, policies and controllers:

##### Graduated to GA (stable) in 1.34

| Feature | What it means for authors | KEP |
|---|---|---|
| **Dynamic Resource Allocation (structured parameters)** | `resource.k8s.io/v1` (`DeviceClass`, `ResourceClaim`, `ResourceClaimTemplate`, `ResourceSlice`) is served by default; Pods use `spec.resourceClaims` + `resources.claims`. | 4381 |
| **Job pod replacement policy** | `spec.podReplacementPolicy: Failed` (create replacements only after the old Pod reached phase `Failed`). | 3939 |
| **VolumeAttributesClass** | `storage.k8s.io/v1` `VolumeAttributesClass`; PVC `spec.volumeAttributesClassName` (needs a CSI driver with ModifyVolume). | 3751 |
| **Recovery from volume expansion failure** | A PVC whose expansion failed may be retried with a smaller size. | 1790 |
| **Sleep action for lifecycle hooks** (+ zero duration) | `lifecycle.preStop.sleep.seconds` / `postStart.sleep`; `seconds: 0` is allowed (no-op). | 3960, 4818 |
| **Relaxed environment variable names** | Env var names may contain almost any printable ASCII except `=` (e.g. `Logging:LogLevel:Default`), also via `envFrom`. | 4369 |
| **Relaxed DNS search validation** | `spec.dnsConfig.searches` may contain `_` and a single `.` entry. | 4427 |
| **Ordered namespace deletion** | Pods are deleted before other namespaced resources (mitigates CVE-2024-7598). | 5080 |
| **Structured authentication configuration** | `apiserver.config.k8s.io/v1` `AuthenticationConfiguration` (JWT authenticators, CEL claim rules). | 3331 |
| **Authorize with field and label selectors** | Authorizers (webhook, node) can decide on `list`/`watch`/`deletecollection` selectors. | 4601 |
| **Anonymous auth only for configured endpoints** | `anonymous.conditions` in the authentication config. | 4633 |
| **Streaming encoding for LIST responses**, **consistent reads from cache**, **resilient watch-cache initialization** | Lower apiserver memory and etcd load for big lists; no API change. | 5116, 4568 |
| **Node swap support** (`LimitedSwap`), **kubelet and API server tracing**, **cgroup driver from CRI**, **AppArmor support** (the `appArmorProfile` fields have had no gate since 1.31), **per-plugin scheduler requeue hints**, **Windows DSR/overlay**, **separate taint-eviction controller** | Node/cluster-operator features; mostly no manifest change. | 2400, 4033, 4247, 5100, 3902 |

##### Beta in 1.34 (state of the gate matters — see the gate table)

| Feature | Gate / default in 1.34 | Notes |
|---|---|---|
| **Pod-level resources** `spec.resources` | `PodLevelResources` **beta, on** (new default in 1.34) | CPU/memory budget for the whole Pod; HPA understands it; rejected for `spec.os.name: windows`. |
| **In-place Pod resize** (`resize` subresource) | `InPlacePodVerticalScaling` beta, on (since 1.33) | 1.34 adds memory-limit decrease with `NotRequired` and resize prioritisation. GA only in 1.35. |
| **Pod `metadata.generation` / `status.observedGeneration`** | `PodObservedGenerationTracking` **beta, on** (new) | Kubelet reports which generation it acted on. GA in 1.35. |
| **`trafficDistribution: PreferSameZone` / `PreferSameNode`** | `PreferSameTrafficDistribution` **beta, on** (new) | `PreferClose` is deprecated in favour of `PreferSameZone`; 1.34 does not yet warn (1.35 does). |
| **DRA prioritized list** `firstAvailable` | `DRAPrioritizedList` **beta, on** (new) | Ordered alternatives in a device request. |
| **DRA admin access** `adminAccess: true` | `DRAAdminAccess` **beta, on** (new) | Requires namespace label `resource.kubernetes.io/admin-access: "true"` (label renamed from `resource.k8s.io/admin-access`). |
| **MutatingAdmissionPolicy** `admissionregistration.k8s.io/v1beta1` | `MutatingAdmissionPolicy` beta, **off**; API group version also **off** | Needs the gate *and* `--runtime-config=admissionregistration.k8s.io/v1beta1=true`. |
| **Projected SA tokens for image credential providers** | `KubeletServiceAccountTokenForCredentialProviders` beta, on | Kubelet-side. |
| **External ServiceAccount token signer** | `ExternalServiceAccountTokenSigner` beta, on | apiserver flag `--service-account-signing-endpoint`. |
| **kuberc** (`~/.kube/kuberc`) | beta, on in kubectl 1.34 | Aliases and default flags, separate from kubeconfig. |
| **WatchList** (streaming informer initial list) | apiserver `WatchList` beta, **on** (new); client-go `WatchListClient` beta, **off** | Used by kube-controller-manager; opt-in for your clients. |
| **Kubelet PodResources API reports DRA allocations** (KEP-3695) | `KubeletPodResourcesDynamicResources`, `KubeletPodResourcesGet` beta, on | Node monitoring agents can see devices allocated through DRA. |
| **Declarative validation of built-in types** (KEP-5073) | `DeclarativeValidation` beta, on (since 1.33) | Internal; some built-in validation error messages changed wording. |
| **List from cache snapshot**, **size-based LIST cost (APF)**, **async scheduler API calls** (turned off again in v1.34.2), **PSI metrics**, **Windows graceful shutdown**, **mutable CSINode allocatable count** (off) | Server/node internals. | |

##### New alpha in 1.34 (off by default — the apiserver silently drops or rejects these fields)

| Feature | Gate | Field |
|---|---|---|
| Container restart rules | `ContainerRestartRules` | `containers[].restartPolicy` on regular containers, `containers[].restartPolicyRules` |
| Env vars from files | `EnvFiles` | `env[].valueFrom.fileKeyRef` |
| Pod hostname override | `HostnameOverride` | `spec.hostnameOverride` |
| Pod certificates | `PodCertificateRequest` | `projected.sources[].podCertificate`, `certificates.k8s.io/v1alpha1` `PodCertificateRequest` |
| DRA consumable capacity, device binding conditions, extended-resource mapping, resource health status | `DRAConsumableCapacity`, `DRADeviceBindingConditions`, `DRAExtendedResource`, `ResourceHealthStatus` | `DeviceClass.spec.extendedResourceName`, capacity sharing fields, `status.containerStatuses[].allocatedResourcesStatus` |
| Relaxed Service name validation | `RelaxedServiceNameValidation` | Service names validated as DNS-1123 labels (may start with a digit) |
| `nominatedNodeName` as placement hint | `NominatedNodeNameForExpectation`, `ClearingNominatedNodeNameAfterBinding` | scheduler behaviour |
| KYAML output | kubectl, `KUBECTL_KYAML=true` | `kubectl get -o kyaml` |

##### Behaviour changes, deprecations and removals in 1.34

- **Pod Security Standards**: the **Baseline and Restricted** levels now forbid setting `host` on probe and lifecycle `httpGet`/`tcpSocket` handlers (KEP-4940). Namespaces pinned to `pod-security.kubernetes.io/enforce-version: v1.33` or older do not get the new check.
- **AppArmor**: profiles in `securityContext.appArmorProfile` are **no longer copied into the deprecated `container.apparmor.security.beta.kubernetes.io/*` annotations**. Anything that reads those annotations must read the fields.
- **CRDs**: an **unrecognized OpenAPI `format`** in a CRD schema now produces a warning on write (write still succeeds). v1.34.0 also warned wrongly on `int32`/`int64`/`float`/`double` formats of number/integer properties; fixed in v1.34.1. The changelog also lists new `k8s-short-name`/`k8s-long-name` formats, but they are registered with "introduced in 1.34" and CRD validation uses the n-1 compatibility version, so a 1.34 apiserver still treats them as unrecognized (warning, not enforced); they take effect in 1.35.
- **CEL**: the **Kubernetes semver library** (`isSemver()`, `semver()`) is available in CRD rules and admission policies from 1.34.
- **API Go types**: zero-valued `metadata.creationTimestamp` is **omitted** from JSON/YAML/CBOR (no more `creationTimestamp: null`); all generated apply configurations implement the new `runtime.ApplyConfiguration` interface; `k8s.io/utils/pointer` usages were replaced by `k8s.io/utils/ptr` across the code base.
- **DRA**: `resource.k8s.io/v1alpha3` now only contains `DeviceTaintRule`; the v1alpha4 kubelet gRPC API was removed and the v1beta1 kubelet DRA gRPC API is deprecated (v1 is GA).
- **Kubelet configuration**: `cgroupDriver` in the kubelet config file is deprecated (automatic detection from the CRI is GA); the `--cloud-config` and `--register-schedulable` kubelet flags are removed; `StreamingConnectionIdleTimeout` is deprecated. Kubernetes 1.35 is the last release supporting containerd 1.x.
- **Metrics**: many apiserver/etcd metrics changed labels (`resource_prefix`/`type`/`kind` replaced by `group` + `resource`); `apiserver_storage_objects` is deprecated in favour of `apiserver_resource_objects`. Dashboards and alerts must be updated.
- **Default APF configuration** no longer has the `endpoint-controller` and `workload-leader-election` FlowSchemas — migrate leader election from `configmapsleases`/`endpointsleases` to `leases`.
- **Aggregated API servers**: kube-apiserver now proxies using the **EndpointSlices** of the APIService's Service, not Endpoints. Hand-written Endpoints for an aggregated API need a matching EndpointSlice.
- **`topologySpreadConstraints[].matchLabelKeys`** is merged into `labelSelector` by the apiserver (`MatchLabelKeysInPodTopologySpreadSelectorMerge` beta, on). Upgrade 1.32 → 1.33 → 1.34 one minor at a time.
- **Headless Services**: setting `externalIPs`, `loadBalancerIP` or `sessionAffinity: ClientIP` on a headless Service now returns a warning (they are ignored). v1.34.0/v1.34.1 print `Warning: spec.SessionAffinity is ignored for headless services` for **every** headless Service (the defaulted `None` triggered it; verified on v1.34.1); v1.34.2 fixed the spurious warning — do not "fix" manifests because of it.
- **Scheduler / `nominatedNodeName`**: the KEP-5278 changes (scheduler uses `status.nominatedNodeName` as a placement hint, the apiserver clears it on binding) sit behind `NominatedNodeNameForExpectation` and `ClearingNominatedNodeNameAfterBinding`, which 1.34 **demoted to alpha/off** before release (verified in `kube-apiserver -h` of v1.34.1); the default 1.34 behaviour is unchanged. Both are beta/on in 1.35.
- **Kubelet**: static Pods that reference API objects (ConfigMaps, Secrets, ServiceAccounts, ResourceClaims, …) are now rejected by the kubelet instead of running without a mirror Pod.
- **kubectl / kubeconfig**: the `preferences` field in kubeconfig is deprecated in favour of `kuberc`; `kubectl api-resources -o json|yaml`, `kubectl autoscale --cpu/--memory` (deprecating `--cpu-percent`) and `kubectl top --show-swap` are new.
- **etcd**: the default etcd version moves to the **v3.6** line (v3.6.1 at release; etcd client library v3.6.4).
- **Gates removed** (feature permanently on): `LegacySidecarContainers`, `PodDisruptionConditions`, `DevicePluginCDIDevices`; `JobPodReplacementPolicy`, `AuthorizeWithSelectors`, `RelaxedEnvironmentVariableValidation`, `MultiCIDRServiceAllocator` locked on.
- **client-go**: API streaming support was removed from typed/dynamic/metadata `List()` (streaming is only through informers/reflector with `WatchListClient`).

##### Notable 1.34 patch-release fixes (pin at least the listed patch)

| Fixed in | Problem |
|---|---|
| v1.34.1 | Spurious CRD warnings about `int32`/`int64` formats on integer/number properties. |
| v1.34.2 | **Spurious rollout of all existing StatefulSets** after upgrading the control plane 1.33 → 1.34 (fix behind `StatefulSetSemanticRevisionComparison`, on). Job status update failure "startTime cannot be removed for unsuspended job" after resume. The `SchedulerAsyncAPICalls` gate was turned **off** again to avoid a scheduler performance regression. Spurious `spec.SessionAffinity is ignored for headless services` warning on every headless Service removed. |
| v1.34.3 | **client-go regression: informers ignored configured `Transformer` functions** (affects controller-runtime cache transforms). ValidatingAdmissionPolicy/MutatingAdmissionPolicy spurious "namespace not found" for objects in brand-new namespaces. |
| v1.34.4 | Possible apiserver panic validating a custom resource whose CRD declares a status subresource but no `status` property in the schema. Pods with env values containing `$` followed by a multi-byte character failed to start (1.34 regression). DRA double allocation under fast scheduling. |
| v1.34.7 | Built with Go 1.25; audit-log request-latency annotation regression (1.34) fixed; kube-proxy nftables mode works with nft 1.1.3. |
| v1.34.9 | Env values from Secrets with binary (non-UTF-8) data broke (1.34 regression). |
| v1.34.11 | Built with Go 1.26; `golang.org/x/net`/`x/text` security updates; **`k8s.io/*` v0.34.11+ modules declare `go 1.25.0`** (v0.34.0–v0.34.10 declare `go 1.24.0`). |
| v1.34.12 | kube-scheduler crash (integer overflow) for ResourceClaims whose device counts add up past the int range; duplicate CDI device IDs after a retried DRA `PrepareResources`. |

---

#### Version-Aware Review & "Do Not Use" List

##### How to review against a pinned version

- Determine the target version from the repository: `go.mod` (`k8s.io/api v0.34.x`, `sigs.k8s.io/controller-runtime v0.22.x`), the Helm chart `kubeVersion`, CI `kind`/`envtest` versions (`ENVTEST_K8S_VERSION=1.34`), or cluster docs.
- Before claiming a field or API "does not exist", check it against **1.34**: `kubectl explain <resource>.<field>` on a 1.34 cluster, the 1.34 API reference, or the `k8s.io/api` v0.34 Go types. A field present in the OpenAPI schema can still be **silently dropped** when its feature gate is off (see below) — "exists in the schema" and "works by default" are different statements.
- Say "not available in 1.34" only for features that 1.34 provably lacks (listed below). Say "alpha/off by default in 1.34" for gated fields, and point out that the cluster operator must enable the gate.
- Distinguish **API server** features (validated on write: fields, CEL, admission) from **node** features (kubelet, container runtime: e.g. user namespaces need runtime support, image volumes need containerd ≥ 2.1). A manifest can be accepted and still not work at runtime.

##### Alpha fields are dropped silently, not rejected

When a gate is off, the apiserver strips the gated field on create (unless the existing object already uses it). Verified on kube-apiserver v1.34.1 with `kubectl create --dry-run=server -o yaml`:

```yaml
# Submitted to a default 1.34 apiserver
spec:
  hostnameOverride: my-host              # HostnameOverride alpha  -> dropped
  containers:
    - name: app
      image: registry.example.com/app:1.4.0
      restartPolicy: Never               # ContainerRestartRules alpha -> dropped
      restartPolicyRules:                # dropped
        - action: Restart
          exitCodes: {operator: In, values: [42]}
      volumeMounts: [{name: data, mountPath: /data}]   # dropped together with the volume
  volumes:
    - name: data
      image: {reference: registry.example.com/data:1.0}  # ImageVolume beta but OFF -> volume removed
```

The Pod is created without any of these fields and without an error. Other observed cases on 1.34: StatefulSet `spec.updateStrategy.rollingUpdate.maxUnavailable` (alpha `MaxUnavailableStatefulSet`) and HPA `spec.behavior.scaleUp.tolerance` (alpha `HPAConfigurableTolerance`) are dropped; `env[].valueFrom.fileKeyRef` is dropped and the remaining empty `valueFrom` then fails validation ("must specify one of: `fieldRef`, `resourceFieldRef`, `configMapKeyRef` or `secretKeyRef`"). On 1.33 the same Pod fields (`restartPolicyRules`, `hostnameOverride`) are rejected with "strict decoding error: unknown field" because they are not in the 1.33 schema at all. Flag such fields in review unless the repository documents that the gate is enabled.

##### Do not use or recommend — not available in Kubernetes 1.34

These come from newer releases. In 1.34 they are absent, rejected, or alpha-off-and-dropped:

**From 1.35 (released 2025-12-17)**
- In-place Pod resize as **GA**, and in-place resize of pod-level resources (`InPlacePodLevelResourcesVerticalScaling`, alpha in 1.35; new Pod `status.resources`/`status.allocatedResources`) — 1.34 has container-level resize as beta only.
- `restartPolicy`/`restartPolicyRules` on regular containers enabled by default (`ContainerRestartRules` beta/on in 1.35), `env[].valueFrom.fileKeyRef` enabled by default (`EnvFiles`), `spec.hostnameOverride` enabled by default (`HostnameOverride`) — all alpha/off in 1.34.
- Image volumes (`volumes[].image`) **enabled by default** — in 1.34 `ImageVolume` is beta but **off**, so the volume is dropped.
- StatefulSet `rollingUpdate.maxUnavailable` (`MaxUnavailableStatefulSet`: beta/on only in v1.35.0–v1.35.3, turned off again in v1.35.4 and in 1.36, on by default again in 1.37), HPA `behavior.*.tolerance` on by default (`HPAConfigurableTolerance`), Deployment/ReplicaSet `status.terminatingReplicas` on by default.
- Alpha features of 1.35 that do not exist at all in 1.34: Pod `spec.workloadRef`, the `scheduling.k8s.io/v1alpha1` `Workload` API and gang scheduling; Node `status.declaredFeatures`; toleration operators `Gt`/`Lt` (`TaintTolerationComparisonOperators`); constrained impersonation (`impersonate-on:` verbs).
- `CSIDriver.spec.serviceAccountTokenInSecrets`; `podCertificate.userAnnotations`; PodCertificateRequest as `certificates.k8s.io/v1beta1`.
- CRD `status.observedGeneration` and `status.conditions[].observedGeneration` on CustomResourceDefinition objects.
- `storagemigration.k8s.io/v1beta1` `StorageVersionMigration` (1.34 only has the alpha API behind a gate).
- **Enforced `k8s-short-name` / `k8s-long-name` formats** in CRDs: although CHANGELOG-1.34 announces them, on 1.34.1 these formats produce `Warning: unrecognized format "k8s-short-name"` and are **not enforced** (a value `Bad_Name` was accepted) because CRD format support is evaluated at the n-1 compatibility version; on 1.35.0 the warning is gone. Use `pattern`/`maxLength` or a CEL `format.dns1123Label()` rule on 1.34.
- A deprecation warning for `trafficDistribution: PreferClose` (added in 1.35; 1.34 accepts it silently). `PreferSameZone`/`PreferSameNode` reached GA in 1.35 but are already beta/on in 1.34.
- `kubectl get -o kyaml` without `KUBECTL_KYAML=true` (1.35 enables it by default). Comparable resource versions: before 1.35 clients may only compare `resourceVersion` for equality.
- `DynamicResourceAllocation` locked on (in 1.34 it is GA but can still be disabled).

**From 1.36 (2026-04-22) and 1.37 (2026-08-26)**
- GA of user namespaces, MutatingAdmissionPolicy (`admissionregistration.k8s.io/v1`), image volumes, VolumeGroupSnapshot, DRA admin access / prioritized list (GA), PSI, ProcMount, KYAML (1.37), Pod certificates and ClusterTrustBundles (GA in 1.37), storage version migration GA (1.37), HPA scale-to-zero (beta in 1.37).
- The gitRepo volume plugin is permanently removed in 1.36; in 1.34 the apiserver still accepts it with `Warning: spec.volumes[0].gitRepo: deprecated in v1.11, and disabled by default in v1.33+` and the kubelet refuses to mount it by default. Do not use it either way.
- `Service.spec.externalIPs` is deprecated with a warning from 1.36 — in 1.34 it is accepted without warning (except on headless Services); still avoid it (CVE-2020-8554).

**Library versions that do not pair with 1.34**
- controller-runtime **v0.23+** (built on k8s.io/* v0.35): generic `ctrl.NewWebhookManagedBy(mgr, &T{})`/`admission.Validator[T]`, `mgr.GetEventRecorder()` (events/v1 recorder). With v0.22 use `ctrl.NewWebhookManagedBy(mgr).For(&T{}).WithValidator(admission.CustomValidator)` and `mgr.GetEventRecorderFor(name)`.
- controller-tools **v0.20+** (paired with 1.35).
- Mixing `k8s.io/client-go v0.35+` with `k8s.io/api v0.34` (or any minor skew among `k8s.io/*` staging modules) — keep all `k8s.io/*` staging modules on the same v0.34.x patch.

##### Deprecated or removed by the time of 1.34 (flag if used)

- Beta API versions removed in earlier releases (complete list in "API Versions Served by 1.34"): `extensions/v1beta1`, `apps/v1beta1|v1beta2`, `policy/v1beta1` (PSP, PDB), `batch/v1beta1` CronJob, `autoscaling/v2beta1|v2beta2`, `networking.k8s.io/v1beta1` Ingress, `flowcontrol.apiserver.k8s.io/v1beta3` and older, `storage.k8s.io/v1beta1` CSIStorageCapacity, `discovery.k8s.io/v1beta1`, `events.k8s.io/v1beta1`, `node.k8s.io/v1beta1`, `admissionregistration.k8s.io/v1beta1` webhooks, `apiextensions.k8s.io/v1beta1`.
- PodSecurityPolicy (removed 1.25) — use Pod Security Admission or a policy engine.
- AppArmor annotations (`container.apparmor.security.beta.kubernetes.io/<c>`) — deprecated since 1.30, warned on write; use `securityContext.appArmorProfile`.
- `service.alpha.kubernetes.io/tolerate-unready-endpoints` (use `spec.publishNotReadyAddresses`) and `seccomp.security.alpha.kubernetes.io/*` annotations (use `securityContext.seccompProfile`).
- `trafficDistribution: PreferClose` — prefer `PreferSameZone` (same semantics).
- `k8s.io/utils/pointer` (deprecated; use `k8s.io/utils/ptr`), `wait.PollImmediate*`/`wait.Poll*` without context (deprecated; use `wait.PollUntilContextTimeout`/`PollUntilContextCancel`), untyped `workqueue.RateLimitingInterface` (use `TypedRateLimitingInterface[T]`), `ctrl.Result{Requeue: true}` (deprecated in controller-runtime v0.22: "Use `RequeueAfter` instead").

---

#### API Versions Served by 1.34

`kubectl api-versions` on a default kube-apiserver v1.34.1 (no `--runtime-config`):

```
admissionregistration.k8s.io/v1    apiextensions.k8s.io/v1      apiregistration.k8s.io/v1
apps/v1                            authentication.k8s.io/v1     authorization.k8s.io/v1
autoscaling/v1                     autoscaling/v2               batch/v1
certificates.k8s.io/v1             coordination.k8s.io/v1       discovery.k8s.io/v1
events.k8s.io/v1                   flowcontrol.apiserver.k8s.io/v1
networking.k8s.io/v1               node.k8s.io/v1               policy/v1
rbac.authorization.k8s.io/v1       resource.k8s.io/v1           scheduling.k8s.io/v1
storage.k8s.io/v1                  v1
```

Compared with v1.33.0 the only new group-version is **`resource.k8s.io/v1`** (DRA); new kinds are `DeviceClass`, `ResourceClaim`, `ResourceClaimTemplate`, `ResourceSlice` (resource.k8s.io/v1) and **`VolumeAttributesClass` in `storage.k8s.io/v1`**. A default v1.35.0 serves exactly the same group-versions.

Important consequences:

- **No beta API is served by default.** Since 1.24 new beta APIs are off by default. `resource.k8s.io/v1beta1` and `v1beta2`, `admissionregistration.k8s.io/v1beta1` (MutatingAdmissionPolicy), `storage.k8s.io/v1beta1` (VolumeAttributesClass beta), `certificates.k8s.io/v1beta1` (ClusterTrustBundle), `coordination.k8s.io/v1beta1` (LeaseCandidate) and `networking.k8s.io/v1beta1` (IPAddress/ServiceCIDR beta) require `--runtime-config`. A manifest with `apiVersion: resource.k8s.io/v1beta2` fails on a default 1.34 cluster with `no matches for kind "ResourceClaim" in version "resource.k8s.io/v1beta2"`.
- **Write DRA objects as `resource.k8s.io/v1`** — the v1 shape differs from v1beta1 (see the DRA section).
- `autoscaling/v2` is the only HPA version with `behavior` and multiple metrics; `autoscaling/v1` still exists but should not be used for new manifests.
- `flowcontrol.apiserver.k8s.io/v1` is the only served FlowSchema/PriorityLevelConfiguration version (v1beta3 removed in 1.32).

Removed API versions you may still find in old charts (all **rejected** by 1.34):

| Removed in | API version → replacement |
|---|---|
| 1.16 | `extensions/v1beta1` and `apps/v1beta1`/`v1beta2` Deployment/DaemonSet/StatefulSet/ReplicaSet → `apps/v1`; `extensions/v1beta1` NetworkPolicy → `networking.k8s.io/v1` |
| 1.22 | `admissionregistration.k8s.io/v1beta1` webhooks, `apiextensions.k8s.io/v1beta1` CRD, `apiregistration.k8s.io/v1beta1`, `authentication`/`authorization.k8s.io/v1beta1`, `certificates.k8s.io/v1beta1`, `coordination.k8s.io/v1beta1` Lease, `extensions/v1beta1` and `networking.k8s.io/v1beta1` Ingress/IngressClass, `rbac.authorization.k8s.io/v1beta1`, `scheduling.k8s.io/v1beta1`, `storage.k8s.io/v1beta1` CSIDriver/CSINode/StorageClass/VolumeAttachment → `v1` |
| 1.25 | `batch/v1beta1` CronJob, `discovery.k8s.io/v1beta1` EndpointSlice, `events.k8s.io/v1beta1`, `autoscaling/v2beta1`, `policy/v1beta1` PodDisruptionBudget and PodSecurityPolicy (PSP has no replacement API), `node.k8s.io/v1beta1` RuntimeClass |
| 1.26 | `flowcontrol.apiserver.k8s.io/v1beta1`, `autoscaling/v2beta2` |
| 1.27 | `storage.k8s.io/v1beta1` CSIStorageCapacity |
| 1.29 | `flowcontrol.apiserver.k8s.io/v1beta2` |
| 1.32 | `flowcontrol.apiserver.k8s.io/v1beta3` |

No built-in GA API version is removed in 1.34.

---

#### Feature-Gate Defaults that Change What Manifests Mean

Defaults read from `kube-apiserver -h` of v1.33.0, v1.34.1 and v1.35.0 (GA gates are not listed; "—" means the gate no longer exists because the feature is GA and always on).

| Gate | 1.33 | **1.34** | 1.35 | Manifest impact on 1.34 |
|---|---|---|---|---|
| `DynamicResourceAllocation` | beta, off | **GA** | GA (locked) | `resource.k8s.io/v1`, Pod `resourceClaims` work |
| `DRAPrioritizedList` | alpha | **beta, on** | beta, on | `firstAvailable` accepted |
| `DRAAdminAccess` | alpha | **beta, on** | beta, on | `adminAccess: true` accepted in labelled namespaces |
| `DRAConsumableCapacity`, `DRAExtendedResource`, `DRADeviceBindingConditions`, `DRADeviceTaints`, `DRAPartitionableDevices` | — / alpha | alpha, off | alpha, off | fields dropped |
| `PodLevelResources` | alpha, off | **beta, on** | beta, on | `spec.resources` kept (dropped on 1.33) |
| `InPlacePodVerticalScaling` | beta, on | beta, on | — (GA) | `resize` subresource, `resizePolicy` |
| `PodObservedGenerationTracking` | alpha | **beta, on** | — (GA) | Pod `metadata.generation` increments on spec updates |
| `PreferSameTrafficDistribution` | alpha | **beta, on** | GA | `PreferSameZone`/`PreferSameNode` accepted (1.33 rejects `PreferSameNode`) |
| `ImageVolume` | beta, **off** | beta, **off** | beta, on | `volumes[].image` dropped with its mounts |
| `UserNamespacesSupport` | beta, on | beta, on | beta, on | `hostUsers: false` accepted; needs runtime support |
| `ProcMountType` | beta, on | beta, on | beta, on | `procMount: Unmasked` (requires `hostUsers: false`) |
| `SupplementalGroupsPolicy` | beta, on | beta, on | — (GA) | `securityContext.supplementalGroupsPolicy: Strict` |
| `MaxUnavailableStatefulSet` | alpha | alpha | beta, on (v1.35.0–v1.35.3; off from v1.35.4) | `rollingUpdate.maxUnavailable` dropped |
| `HPAConfigurableTolerance` | alpha | alpha | beta, on | `behavior.*.tolerance` dropped |
| `DeploymentReplicaSetTerminatingReplicas` | alpha | alpha | beta, on | `status.terminatingReplicas` not populated |
| `JobManagedBy` | beta, on | beta, on | — (GA) | `spec.managedBy` honoured |
| `ContainerRestartRules`, `EnvFiles`, `HostnameOverride` | — | **alpha, off** | beta, on | fields dropped |
| `PodCertificateRequest` | — | alpha, off | beta, off | `podCertificate` projection dropped |
| `MutatingAdmissionPolicy` | alpha, off | **beta, off** | beta, off | API not served by default |
| `ClusterTrustBundle`, `ClusterTrustBundleProjection` | beta, off | beta, off | beta, off | not served by default |
| `SELinuxMount` / `SELinuxChangePolicy` | beta off / beta on | beta off / beta on | same | `seLinuxChangePolicy: Recursive|MountOption` |
| `RelaxedServiceNameValidation` | — | alpha, off | alpha, off | Service names still DNS-1035 (must start with a letter) |
| `StorageVersionMigrator` | alpha | alpha | beta, off | — |
| `WatchList` (apiserver) | beta, off | **beta, on** | beta, on | streaming initial list available to clients that opt in |
| `MatchLabelKeysInPodTopologySpread` | beta, on | beta, on | beta, on | `matchLabelKeys` merged into the selector |

GA in 1.33 (their gates are gone in 1.33/1.34) and therefore usable with no gate: native sidecar containers (`initContainers[].restartPolicy: Always`), Job `successPolicy` and `backoffLimitPerIndex`/`maxFailedIndexes`, CRD validation ratcheting, `nftables` kube-proxy mode, recursive read-only mounts (`recursiveReadOnly`), `topologySpreadConstraints[].nodeAffinityPolicy`/`nodeTaintsPolicy`, `spec.trafficDistribution` itself (`ServiceTrafficDistribution`, value `PreferClose`). GA in 1.32: CRD `selectableFields`, StatefulSet `persistentVolumeClaimRetentionPolicy`. GA in 1.31: AppArmor `securityContext.appArmorProfile`, CEL IP/CIDR libraries. GA in 1.30: ValidatingAdmissionPolicy. Available since 1.29: CEL quantity library and CEL optional types.

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
| **Server is the source of truth** | Validate on the server (OpenAPI + CEL + admission), not only in clients or Helm templates. Verify with `kubectl apply --dry-run=server`. |

---

#### Project Layout, Toolchain & Dependency Pinning

##### Recommended Scaffold (Kubebuilder v4)

```
├── api/
│   └── v1/
│       ├── rediscluster_types.go        # API type definitions + markers
│       ├── groupversion_info.go
│       └── zz_generated.deepcopy.go     # generated, never edit
├── internal/
│   ├── controller/
│   │   └── rediscluster_controller.go   # Reconciler
│   └── webhook/v1/
│       └── rediscluster_webhook.go      # Admission webhook (only if CEL is not enough)
├── config/
│   ├── crd/                             # Generated CRD manifests
│   ├── rbac/                            # Generated RBAC manifests
│   └── default/                         # Kustomize overlay
├── cmd/main.go
├── Dockerfile
└── Makefile                             # make generate, make manifests, make test
```

##### Toolchain Requirements for 1.34

| Component | Version for a 1.34 target | Notes |
|---|---|---|
| `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` | **v0.34.x, all the same patch** (latest v0.34.12) | v0.34.0–v0.34.10 need Go ≥ 1.24.0; **v0.34.11+ need Go ≥ 1.25.0** (`go 1.25.0` directive). Use ≥ v0.34.3 for the informer `Transformer` fix. |
| `sigs.k8s.io/controller-runtime` | **v0.22.x** (latest v0.22.5) | v0.22.0/0.22.1 require client-go v0.34.0, v0.22.2–v0.22.4 v0.34.1, v0.22.5 v0.34.3. v0.23 belongs to 1.35. |
| `sigs.k8s.io/controller-tools` (`controller-gen`) | **v0.19.x** | Built against k8s.io/* v0.34.0; v0.20 belongs to 1.35. |
| Kubebuilder | v4.9.x (scaffolds controller-runtime v0.22.1) or v4.10.x (v0.22.4) | Both scaffold controller-tools v0.19.0. |
| envtest binaries | `ENVTEST_K8S_VERSION=1.34` (setup-envtest) | Matches the server you test against. |
| kubectl | 1.33–1.35 (±1 minor skew) | kubectl 1.34 bundles kustomize v5.7.1. |
| Go | 1.24+ (1.25+ with k8s.io/* ≥ v0.34.11) | Kubernetes 1.34 itself was built with Go 1.24.x up to v1.34.6, Go 1.25.x in v1.34.7–v1.34.10 and Go 1.26 from v1.34.11. |

```
// go.mod — a consistent 1.34 dependency set (builds with Go 1.25)
module example.com/redis-operator

go 1.25.0

require (
	k8s.io/api v0.34.12
	k8s.io/apimachinery v0.34.12
	k8s.io/client-go v0.34.12
	sigs.k8s.io/controller-runtime v0.22.5
)
```

Review rules:
- **All `k8s.io/*` staging modules must share one minor and should share one patch.** `k8s.io/api v0.34.x` with `k8s.io/client-go v0.35.x` (or a `replace` that mixes them) compiles sometimes and fails subtly at runtime (scheme registration, apply configurations, generated clients).
- controller-runtime's minor fixes the k8s.io minor: bumping `k8s.io/*` to v0.35 requires controller-runtime v0.23.
- Generated code (`zz_generated.deepcopy.go`, CRD YAML, RBAC YAML, apply configurations) must be regenerated with the pinned `controller-gen` and committed; a diff between markers and committed YAML is a bug.

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

Kubernetes uses several distinct name classes, each with its own allowed character set.

| Name class | Used for | Allowed characters | Start/end | Max length |
|---|---|---|---|---|
| **DNS Subdomain** (RFC 1123) | most `metadata.name` values (Pods, Deployments, ConfigMaps, CRs), API groups, CRD `spec.group` | lowercase `a–z`, digits, `-`, `.` | alphanumeric at both ends | 253 |
| **DNS Label** (RFC 1123) | `metadata.namespace`, container names, port names (≤ 15) | lowercase `a–z`, digits, `-` | alphanumeric at both ends | 63 |
| **DNS Label** (RFC 1035) | **Service names** — still enforced in 1.34 (`RelaxedServiceNameValidation` is alpha/off) | lowercase `a–z`, digits, `-` | must start with a letter, end alphanumeric | 63 |
| **Qualified Name** | label keys, annotation keys, finalizer names | name part: `a–z A–Z 0–9 - _ .`; optional prefix: DNS subdomain + `/` | name part alphanumeric at both ends | name ≤ 63, prefix ≤ 253 |
| **Label Value** | `metadata.labels` values | `a–z A–Z 0–9 - _ .` or empty | alphanumeric at both ends if non-empty | 63 |
| **Annotation Value** | annotations | any UTF-8 | n/a | all annotations together ≤ 256 KiB |
| **Env var name** **[NEW in 1.34: GA]** | `env[].name`, keys imported by `envFrom` | any printable ASCII except `=` (e.g. `Logging:LogLevel:Default`, `my.var`) | must not be empty | — |
| **ConfigMap / Secret data key** | `data` / `stringData` / `binaryData` keys | `a–z A–Z 0–9 - _ .` | — | 253 |

> Underscore `_` and uppercase letters are **never** valid in DNS subdomain or DNS label names (`metadata.name`, namespaces, Service names, API groups). They are valid in label/annotation keys (name part), label values and ConfigMap keys.

Verified on kube-apiserver v1.34.1: a Service named `1web` is rejected ("a DNS-1035 label must … start with an alphabetic character"), and an env var named `Logging:LogLevel:Default` is accepted (it was already accepted on 1.33 where the gate was beta/on; clusters ≤ 1.31 reject it). Do not rely on relaxed env var names if the same manifests must also apply to clusters older than 1.32.

##### API Group, Kind, Resource

| Element | Rule | Example |
|---|---|---|
| **API Group** | Lowercase DNS subdomain you own | `cache.example.com` |
| **Kind** | CamelCase, singular | `RedisCluster` |
| **Resource (plural)** | Lowercase, plural | `redisclusters` |
| **Short name** | Short lowercase, **must not collide with built-in short names** | `rdc` (not `rc`) |
| **Category** | Lowercase; `.spec.names.categories` | avoid `all` for anything users did not ask to see in `kubectl get all` |

> Do **not** use the empty group, single-word groups, or `*.k8s.io` / `*.kubernetes.io` groups — those are reserved for the Kubernetes project (protected by the `api-approved.kubernetes.io` annotation check).

Short-name collisions are a real bug: a CRD that declares `shortNames: [rc]` is accepted, but `kubectl get rc foo` resolves to the built-in **ReplicationController** and prints `Warning: short name "rc" could also match lower priority resource redisclusters.cache.example.com` followed by `replicationcontrollers "foo" not found` (verified on 1.34.1). Check new short names against `kubectl api-resources` (`po`, `svc`, `rc`, `rs`, `ds`, `sts`, `deploy`, `ing`, `netpol`, `pv`, `pvc`, `sa`, `cm`, `ns`, `no`, `ep`, `ev`, `hpa`, `pdb`, `pc`, `sc`, `csr`, `crd`, `ip`, …).

##### Go Type Names

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

##### JSON Field Names

| Rule | Good | Bad |
|---|---|---|
| camelCase | `replicaCount` | `replica_count`, `ReplicaCount` |
| Declarative nouns (desired state) | `replicas` | `setReplicas` |
| Name references (same namespace) | `secretName` or `secretRef: {name: …}` | `secret` |
| Duration | `timeoutSeconds` (`int32`) or a `metav1.Duration` string | ambiguous units |
| Established abbreviations only | `maxReplicas`, `minReadySeconds`, `url`, `ip`, `id` | `tgtUtilization`, `cfgMapName` |

##### Constant (Enum) Values

```go
// CamelCase string alias — never Go iota enums in API types
// +kubebuilder:validation:Enum=Pending;Running;Failed
type Phase string

const (
	PhasePending Phase = "Pending"
	PhaseRunning Phase = "Running"
	PhaseFailed  Phase = "Failed"
)
```

##### `client.ObjectKey` in Go Code

When code uses `sigs.k8s.io/controller-runtime/pkg/client.ObjectKey`, assume `Namespace` and `Name` came from Kubernetes object identity (DNS Subdomain / DNS Label rules above) unless there is evidence otherwise.

##### Label & Annotation Key Examples

```yaml
# Labels (user-facing, for selection)
app.kubernetes.io/name: redis
app.kubernetes.io/instance: cache-prod
app.kubernetes.io/component: cache
app.kubernetes.io/managed-by: redis-operator

# Annotations (tooling metadata)
cache.example.com/last-backup-time: "2025-01-01T00:00:00Z"
```

---

#### CRD & API Design (CEL in 1.34)

##### Kubebuilder Types (compiled with controller-runtime v0.22.5, CRD generated by controller-gen v0.19.0, applied to 1.34.1)

```go
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RedisClusterSpec defines the desired state of a RedisCluster.
// +kubebuilder:validation:XValidation:rule="self.minReplicas <= self.replicas",messageExpression="'replicas (%d) must be >= minReplicas (%d)'.format([self.replicas, self.minReplicas])",fieldPath=".replicas",reason="FieldValueInvalid"
type RedisClusterSpec struct {
	// Version is the Redis version (semantic version, e.g. 7.2.4). Downgrades are rejected.
	// +required
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule="isSemver(self)",message="version must be a semantic version such as 7.2.4"
	// +kubebuilder:validation:XValidation:rule="!semver(self).isLessThan(semver(oldSelf))",message="version must not be downgraded"
	Version string `json:"version"`

	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=9
	Replicas int32 `json:"replicas"`

	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// StorageClassName is immutable after creation.
	// +required
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="storageClassName is immutable"
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="storageClassName must be a DNS-1123 subdomain"
	StorageClassName string `json:"storageClassName"`

	// PasswordSecretRef names a Secret in the same namespace that holds the password.
	// +optional
	PasswordSecretRef *SecretKeyRef `json:"passwordSecretRef,omitempty"`
}

// SecretKeyRef references a key of a Secret in the object's namespace.
type SecretKeyRef struct {
	// +required
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
	// +required
	// +kubebuilder:validation:MaxLength=253
	Key string `json:"key"`
}

// RedisClusterStatus defines the observed state of a RedisCluster.
type RedisClusterStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	Replicas int32 `json:"replicas,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=16
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:resource:scope=Namespaced,shortName=rdc
// +kubebuilder:selectablefield:JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// RedisCluster is the Schema for the redisclusters API.
type RedisCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisClusterSpec   `json:"spec"`
	Status RedisClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisClusterList contains a list of RedisCluster.
type RedisClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RedisCluster{}, &RedisClusterList{})
}
```

Observed on 1.34.1 with this CRD: creating an object with `replicas: 1, minReplicas: 2, version: "v7", storageClassName: "Fast_SSD"` fails with all three messages at once (`spec.replicas: … replicas (1) must be >= minReplicas (2)`, `spec.version: … must be a semantic version`, `spec.storageClassName: … DNS-1123 subdomain`); changing `version` from `7.2.4` to `7.0.0` fails with "version must not be downgraded"; `client.MatchingFields{"spec.version": "7.2.4"}` lists through the selectable field; `kubectl scale` works through the scale subresource. On 1.33.0 the same CRD is **rejected** at write time: `undeclared reference to 'isSemver'`.

##### Spec Design Rules

- Use `// +optional` and `omitempty` for optional fields; `// +required` for required ones. Pointer types (`*int32`, `*bool`) for optional fields where the zero value is meaningful.
- Prefer **lists of named subobjects** over maps, and mark lists with `+listType=map` + `+listMapKey=name` (or `+listType=set`/`atomic`) so server-side apply merges them correctly:
  ```yaml
  # Preferred
  ports:
    - name: redis
      port: 6379
  # Avoid
  ports:
    redis: 6379
  ```
- **Never use floating-point** fields in `spec`. Use `int32`/`int64`, `resource.Quantity`, or strings with units.
- **Do not use `bool` where an enum will be needed** — use a string type alias from day one.
- **No raw maps** except for labels, annotations, and opaque user data; bound them with `MaxProperties`.
- **Object-level fields only**: no top-level fields besides `apiVersion`, `kind`, `metadata`, `spec`, `status`.
- **Bound every string, list and map** (`MaxLength`, `MaxItems`, `MaxProperties`). CEL cost estimation depends on it (see below), and unbounded fields are a denial-of-service vector.

##### CEL Validation Rules — What 1.34 Supports

Libraries and language features available to `x-kubernetes-validations` (and admission policies) on 1.34:

| Library / feature | Since | Examples |
|---|---|---|
| Standard macros & functions, Kubernetes list/regex/URL libraries | all | `self.all(x, ...)`, `self.exists_one(...)`, `self.matches('^[a-z]+$')`, `url(self).getHost()` |
| Extended strings v2 | 1.30 | `self.lowerAscii()`, `self.split('/')`, `'%d of %d'.format([a, b])` |
| Quantity library, optional types (`?.`, `.orValue()`, `.hasValue()`), cross-type numeric comparisons | 1.29 | `quantity(self).isGreaterThan(quantity('64Mi'))`, `self.?spec.?replicas.orValue(1)` |
| IP and CIDR libraries | 1.31 | `isCIDR(self) && cidr(self).ip().family() == 4`, `ip(self).isLoopback()` |
| Format library | 1.32 | `!format.dns1123Label().validate(self).hasValue()` |
| Two-variable comprehensions | 1.33 | `self.all(k, v, k.startsWith('x/') || v != '')` |
| **Semver library** | **1.34** | `isSemver(self)`, `semver(self).isLessThan(semver(oldSelf))`, `semver(self).major()` |

Every expression in that table compiles in CRD rules on 1.34.1; the CIDR, quantity, format, two-variable and semver rules were also exercised against objects and enforced as expected. The semver functions accept an optional normalize flag (`semver(self, true)`, `isSemver(self, true)`) that strips a leading `v` and fills missing minor/patch numbers. Rules referencing a newer library are **rejected when the CRD is written** to an older apiserver (1.33: `undeclared reference to 'isSemver'`), so a chart that must install on 1.33 cannot use the semver library.

##### CEL Cost Budget Pitfalls (found by testing on 1.34.1)

The apiserver estimates the worst-case cost of every rule and `messageExpression` when the CRD is written and rejects it with "estimated rule cost exceeds budget by factor of more than 100x". The estimate uses `maxLength`/`maxItems`/`maxProperties`; anything without bounds is assumed huge.

```yaml
# BAD — rejected: string concatenation with string(int) has an unbounded estimated size
x-kubernetes-validations:
  - rule: "self.minReplicas <= self.replicas"
    messageExpression: "'replicas (' + string(self.replicas) + ') must be >= minReplicas'"

# GOOD — accepted: format() over a bounded list
x-kubernetes-validations:
  - rule: "self.minReplicas <= self.replicas"
    messageExpression: "'replicas (%d) must be >= minReplicas (%d)'.format([self.replicas, self.minReplicas])"
```

```yaml
# BAD — rejected: the optional returned by format...validate() wraps an unbounded list
messageExpression: "format.dns1123Subdomain().validate(self).value().join(', ')"
# GOOD
message: "storageClassName must be a DNS-1123 subdomain"
```

```yaml
# BAD — on a string field with maxLength: 64, rejected for cost:
# oldSelf.value() loses the maxLength bound
- rule: "!oldSelf.hasValue() || !semver(self).isLessThan(semver(oldSelf.value()))"
  optionalOldSelf: true
# GOOD — a plain transition rule; it only runs on UPDATE when the field existed before
- rule: "!semver(self).isLessThan(semver(oldSelf))"
```

Other CEL rules a reviewer must check:
- `has(oldSelf)` is invalid ("invalid argument to has() macro"). With `optionalOldSelf: true` use `oldSelf.hasValue()` / `oldSelf.value()`; `optionalOldSelf: true` without any `oldSelf` in the rule is rejected ("may not be set if oldSelf is not used in rule").
- A rule that reads an optional field must guard it: `!has(self.minReplicas) || self.minReplicas <= self.replicas`. Without the guard every object that omits the field is rejected with `no such key: minReplicas evaluating rule: …` (verified on 1.34.1). In the example above `minReplicas` has `default: 1`, so it is always present after defaulting.
- Transition rules (`oldSelf`) are skipped on CREATE and whenever the old or new value is absent. A field-level `self == oldSelf` on an optional field therefore blocks *changing* it but not *adding* it later or *removing* it (both verified on 1.34.1). Put a rule such as `has(self.zone) == has(oldSelf.zone)` on the parent object when adding/removing must be blocked as well.
- With validation ratcheting (GA since 1.33), unchanged invalid values in existing objects do not block unrelated updates. Tightening a rule is still a breaking change for new writes.
- Use `reason` (`FieldValueInvalid`, `FieldValueForbidden`, `FieldValueRequired`, `FieldValueDuplicate`) and `fieldPath` so errors point at the right field.

##### OpenAPI Formats — **[NEW in 1.34]** warnings

1.34 warns on CRD writes that use a `format` the server does not recognise, but still stores the CRD. On 1.34.1, `format: k8s-short-name` produced `Warning: unrecognized format "k8s-short-name"` and was **not enforced**. Treat any such warning as a review finding: the author believes a value is validated when it is not. Use `pattern`, `maxLength`, `enum` or CEL instead. String formats accepted without a warning on 1.34.1 include `byte`, `date`, `date-time`, `duration`, `uuid`, `email`, `hostname`, `ipv4`, `ipv6`, `cidr`, `mac`, `uri` and `password`; `uri-reference`, `k8s-long-name` and the numeric formats (`int32`, `int64`, `float`, `double`) on a `type: string` field all warn. On `type: integer`/`number` fields 1.34.1 does not warn about formats (the v1.34.0 false positives for `int32`/`int64` were fixed in v1.34.1).

##### PrintColumns and Scale

Always define at minimum a readiness/state column and `Age` from `metadata.creationTimestamp`. Enable the `scale` subresource for anything with replicas so `kubectl scale`, HPA and PDB-aware tooling work.

---

#### Spec vs. Status, Conditions, observedGeneration

| | `spec` | `status` |
|---|---|---|
| **Written by** | Users / GitOps / other controllers | The owning controller only |
| **Represents** | Desired state | Observed / current state |
| **Subresource** | No | **Yes** — always `// +kubebuilder:subresource:status` |
| **Main-resource PUT/PATCH** | Updated | `status` changes ignored |
| **`/status` PUT/PATCH** | `spec` changes ignored | Updated |

##### Updating Status (controller-runtime)

```go
// GOOD — status through the subresource
if err := r.Status().Update(ctx, rc); err != nil {
	return ctrl.Result{}, err
}

// BAD — with a status subresource, Update() silently ignores status changes
_ = r.Update(ctx, rc)
```

Also flag: writing `spec` from the controller after defaulting (prefer CRD `default:` markers or a mutating policy), and updating status on every reconcile even when nothing changed (wasted writes, resourceVersion churn, conflicts).

##### observedGeneration

Set `status.observedGeneration = metadata.generation` after processing a generation and per condition (`Condition.ObservedGeneration`). Tools such as `kubectl wait` and GitOps health checks compare them.

**[NEW in 1.34]** Pods follow the same convention by default: `PodObservedGenerationTracking` is beta/on, Pod `metadata.generation` increments on spec changes (verified: a Pod went from generation 1 → 2 after a `spec.activeDeadlineSeconds` patch and → 3 after a `resize` subresource patch), and the kubelet reports `status.observedGeneration` plus `status.conditions[].observedGeneration`. Controllers that wait for a resize to be applied should compare those fields instead of polling resource values.

##### Status Conditions

Use `metav1.Condition` from `k8s.io/apimachinery/pkg/apis/meta/v1`:

```go
// +optional
// +listType=map
// +listMapKey=type
// +kubebuilder:validation:MaxItems=16
Conditions []metav1.Condition `json:"conditions,omitempty"`
```

| Field | Rule |
|---|---|
| `Type` | PascalCase: `Ready`, `Degraded`, `Progressing`, `Available` |
| `Status` | `True`, `False`, or `Unknown` — never empty |
| `Reason` | CamelCase machine-readable token: `ReconcileError`, `DeploymentApplyFailed` (required, 1–1024 chars, `^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`) |
| `Message` | Human-readable; may contain details (≤ 32768 chars) |
| `ObservedGeneration` | `metadata.generation` at the time of observation |
| `LastTransitionTime` | Changes **only** when `Status` changes — `meta.SetStatusCondition` handles this |

| Type | Normal Status | Meaning |
|---|---|---|
| `Ready` | `True` | Object is fully operational |
| `Progressing` / `Reconciling` | `True` while working | desired ≠ actual |
| `Stalled` | `False` | Cannot make progress; human attention needed |
| `Degraded` | `False` | Running with reduced capacity |
| `Available` | `True` | Operational and serving |

> Do not add new `phase` string fields; conditions are the API convention. `phase` is an anti-pattern for evolution (clients hard-code state machines).

`meta.SetStatusCondition` returns `bool` (changed or not) — use it to skip unnecessary status writes.

---

#### Versioning & API Evolution

```
v1alpha1 → v1alpha2 → v1beta1 → v1beta2 → v1
```

| Stage | Stability | Built-in APIs enabled by default | Compatibility |
|---|---|---|---|
| `v1alphaX` | Experimental | No | none |
| `v1betaX` | Pre-release | **No** for new beta APIs since 1.24 | best-effort, deprecation period |
| `v1` | Stable | Yes | required |

For CRDs, "enabled" is the `served` flag you set per version.

Rules:
1. **Never remove or rename fields** in a served version.
2. **Never tighten validation** in a way that rejects previously valid values (ratcheting only protects unchanged values).
3. **Add fields as optional** (`omitempty`), never required.
4. Exactly one version has `storage: true`; others are served via conversion.
5. Conversion must be **lossless** in both directions (round-trip unknown fields through annotations if needed).
6. Use a **conversion webhook** when schemas differ (`strategy: Webhook`); `None` only when the schemas are identical.
7. After changing the storage version, migrate stored objects (rewrite them, or `storagemigration.k8s.io` — alpha/off in 1.34, beta in 1.35) before removing the old version from `status.storedVersions`.

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

---

#### Controller / Reconciler Patterns (controller-runtime v0.22)

##### Reconciler (compiles against controller-runtime v0.22.5; exercised with envtest 1.34.1)

This file continues in the "Server-Side Apply from Go" (`applyDeployment`) and "Watches" (`SetupWithManager`) sections; together they need the additional imports `time`, `appsv1`, `types`, the `applyconfigurations` packages, and controller-runtime's `builder`, `controller`, `handler`, `predicate` and `reconcile` packages.

```go
package controller

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cachev1 "example.com/redis-operator/api/v1"
)

const (
	fieldOwner     = client.FieldOwner("redis-operator")
	cleanupFinal   = "cache.example.com/external-cleanup"
	conditionReady = "Ready"
)

// RedisClusterReconciler reconciles RedisCluster objects.
type RedisClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	External ExternalAPI
}

// ExternalAPI is whatever out-of-cluster system the operator manages.
type ExternalAPI interface {
	EnsureDNS(ctx context.Context, name string) error
	DeleteDNS(ctx context.Context, name string) error
}

// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *RedisClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	rc := &cachev1.RedisCluster{}
	if err := r.Get(ctx, req.NamespacedName, rc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !rc.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(rc, cleanupFinal) {
			return ctrl.Result{}, nil
		}
		if err := r.External.DeleteDNS(ctx, rc.Name); err != nil {
			return ctrl.Result{}, fmt.Errorf("deleting DNS record: %w", err)
		}
		controllerutil.RemoveFinalizer(rc, cleanupFinal)
		return ctrl.Result{}, r.Update(ctx, rc)
	}

	// Add the finalizer before creating anything external.
	if controllerutil.AddFinalizer(rc, cleanupFinal) {
		if err := r.Update(ctx, rc); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.External.EnsureDNS(ctx, rc.Name); err != nil {
		return r.setReady(ctx, rc, metav1.ConditionFalse, "DNSError", err.Error(), err)
	}
	if err := r.applyDeployment(ctx, rc); err != nil {
		return r.setReady(ctx, rc, metav1.ConditionFalse, "DeploymentApplyFailed", err.Error(), err)
	}

	logger.V(1).Info("reconciled", "generation", rc.Generation)
	return r.setReady(ctx, rc, metav1.ConditionTrue, "Reconciled", "all resources are in sync", nil)
}

func (r *RedisClusterReconciler) setReady(ctx context.Context, rc *cachev1.RedisCluster,
	status metav1.ConditionStatus, reason, msg string, cause error) (ctrl.Result, error) {
	changed := meta.SetStatusCondition(&rc.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: rc.Generation,
	})
	if rc.Status.ObservedGeneration != rc.Generation {
		rc.Status.ObservedGeneration = rc.Generation
		changed = true
	}
	if changed { // avoid a status write on every reconcile
		if err := r.Status().Update(ctx, rc); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{}, err // requeued with backoff; the next attempt reads the new object
			}
			return ctrl.Result{}, errors.Join(cause, err)
		}
		if status == metav1.ConditionFalse && r.Recorder != nil {
			r.Recorder.Event(rc, corev1.EventTypeWarning, reason, msg)
		}
	}
	return ctrl.Result{}, cause // non-nil cause => requeue with exponential backoff
}
```

Running this reconciler twice against envtest 1.34.1 produced exactly one `Apply` field manager (`redis-operator/Apply`) on the Deployment, a controller owner reference, `status.observedGeneration == metadata.generation`, `Ready=True`, and after `Delete` one `DeleteDNS` call followed by the object disappearing.

Review rules:
- Fetch → `IgnoreNotFound` → deletion/finalizer → ensure children → status. Never return early in a way that skips status for failures.
- The object read from the cached client is a **copy** in controller-runtime (`client.Client.Get` deep-copies), but objects returned from informers/listers in plain client-go are **shared** — never mutate them without `DeepCopy()`.
- Every external call needs a timeout. controller-runtime v0.22 has `controller.Options.ReconciliationTimeout` (sets a deadline on the `ctx` passed to `Reconcile`; default: none).
- Do not `time.Sleep` in `Reconcile`; return `RequeueAfter`.
- Conflicts (`409`) are normal: return the error (backoff) or retry with `retry.RetryOnConflict` around a fresh `Get`.

##### Reconcile Return Conventions (controller-runtime v0.22)

| Return | Meaning |
|---|---|
| `ctrl.Result{}, nil` | Success — wait for the next watch event |
| `ctrl.Result{}, err` | Failure — requeued with per-item exponential backoff (5 ms doubling up to 1000 s, plus a 10 qps / 100 burst bucket) |
| `ctrl.Result{RequeueAfter: d}, nil` | Re-check after `d` (polling external state) |
| `ctrl.Result{Requeue: true}, nil` | **Deprecated in v0.22** ("Use `RequeueAfter` instead"); rate-limited requeue |
| `reconcile.TerminalError(err)` | Do not requeue; error is logged and counted in `controller_runtime_terminal_reconcile_errors_total` |

> Never return both `RequeueAfter`/`Requeue` and a non-nil error — controller-runtime ignores the `Result` and logs a warning when `err != nil`. Choose one.

##### Manager Setup

```go
package main

import (
	"errors"
	"net/http"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	cachev1 "example.com/redis-operator/api/v1"
	"example.com/redis-operator/internal/controller"
)

func main() {
	ctrl.SetLogger(zap.New())
	scheme := runtime.NewScheme()
	if err := errors.Join(clientgoscheme.AddToScheme(scheme), cachev1.AddToScheme(scheme)); err != nil {
		panic(err)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                        scheme,
		Metrics:                       metricsserver.Options{BindAddress: ":8443", SecureServing: true},
		HealthProbeBindAddress:        ":8081",
		LeaderElection:                true,
		LeaderElectionID:              "redis-operator.cache.example.com",
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		ctrl.Log.Error(err, "unable to create manager")
		os.Exit(1)
	}
	if err := (&controller.RedisClusterReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("redis-operator"),
	}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to set up controller")
		os.Exit(1)
	}
	_ = mgr.AddHealthzCheck("healthz", healthz.Ping)
	_ = mgr.AddReadyzCheck("cache-sync", func(req *http.Request) error {
		if !mgr.GetCache().WaitForCacheSync(req.Context()) {
			return errors.New("informer caches not synced")
		}
		return nil
	})
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "manager exited")
		os.Exit(1)
	}
}
```

Notes for v0.22:
- Leader election uses `coordination.k8s.io/v1` Leases (the default since v0.16). RBAC needs `leases` get/create/update in the manager's namespace. 1.34 removed the default APF FlowSchemas `endpoint-controller` and `workload-leader-election` that covered the legacy `configmapsleases`/`endpointsleases` locks — flag any `LeaderElectionResourceLock` override to those values.
- Metrics: serve over HTTPS with `SecureServing: true` and protect them with `filters.WithAuthenticationAndAuthorization` (`sigs.k8s.io/controller-runtime/pkg/metrics/filters`), as Kubebuilder v4 scaffolds; do not expose plain `:8080/metrics` to the cluster network.
- `controller.Options.UsePriorityQueue` (beta, off by default in v0.22), `EnableWarmup` and `SkipNameValidation` exist; controller names must be unique per manager unless `SkipNameValidation` is set (tests that create two managers hit this).
- Use `mgr.GetEventRecorderFor(name)` (core/v1 Events). `mgr.GetEventRecorder()` (events.k8s.io/v1) exists only from v0.23.

---

#### Server-Side Apply from Go

**[NEW in v0.34 / controller-runtime v0.22]** All generated apply configurations implement `runtime.ApplyConfiguration`, and controller-runtime v0.22 adds `client.Client.Apply(ctx, runtime.ApplyConfiguration, ...client.ApplyOption)`. The fake client supports it too.

```go
import (
	appsv1ac "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
)

// applyDeployment declares the full desired Deployment.
// Fields this manager omitted are released (and removed if no one else owns them).
func (r *RedisClusterReconciler) applyDeployment(ctx context.Context, rc *cachev1.RedisCluster) error {
	labels := map[string]string{
		"app.kubernetes.io/name":       "redis",
		"app.kubernetes.io/instance":   rc.Name,
		"app.kubernetes.io/managed-by": "redis-operator",
	}
	gvk := cachev1.GroupVersion.WithKind("RedisCluster")
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
					WithSecurityContext(corev1ac.PodSecurityContext().
						WithRunAsNonRoot(true).
						WithSeccompProfile(corev1ac.SeccompProfile().WithType(corev1.SeccompProfileTypeRuntimeDefault))).
					WithContainers(corev1ac.Container().
						WithName("redis").
						WithImage("registry.example.com/redis:" + rc.Spec.Version).
						WithSecurityContext(corev1ac.SecurityContext().
							WithAllowPrivilegeEscalation(false).
							WithReadOnlyRootFilesystem(true).
							WithCapabilities(corev1ac.Capabilities().WithDrop("ALL")))))))
	return r.Apply(ctx, dep, fieldOwner, client.ForceOwnership)
}
```

SSA review rules:
- **Use a stable, unique field manager** (`client.FieldOwner`). Changing it leaves orphaned ownership; two controllers with the same manager name overwrite each other.
- **Always send the complete desired intent.** With SSA, omitting a field you previously applied removes it. Building an apply configuration from a `Get`-ed object (e.g. `ExtractDeployment` then partial edits) is fine only with `Extract*` functions that filter by your manager.
- **Never apply a typed Go struct** (`appsv1.Deployment`) with `client.Apply` patch semantics: zero values (`replicas: 0`, `false`, empty strings without `omitempty`) become owned values. Use apply configurations (pointer fields), or unstructured.
- `client.ForceOwnership` is right for controllers that own the fields; without it a conflict with another manager (e.g. a human `kubectl edit`) returns 409.
- For your own CRDs, generate apply configurations (`controller-gen applyconfiguration`) or use `client.ApplyConfigurationFromUnstructured`; do not hand-roll JSON.
- `controllerutil.CreateOrUpdate`/`CreateOrPatch` remain valid for simple cases; they read-modify-write through the cache and need the mutate function to be idempotent.

---

#### Watches, Predicates, Indexes

```go
// SetupWithManager wires watches. Secrets are mapped back to the clusters that reference them.
func (r *RedisClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &cachev1.RedisCluster{},
		".spec.passwordSecretRef.name", func(o client.Object) []string {
			rc := o.(*cachev1.RedisCluster)
			if rc.Spec.PasswordSecretRef == nil {
				return nil
			}
			return []string{rc.Spec.PasswordSecretRef.Name}
		}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1.RedisCluster{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{}))).
		Owns(&appsv1.Deployment{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.clustersForSecret)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 4,
			ReconciliationTimeout:   2 * time.Minute,
		}).
		Complete(r)
}

func (r *RedisClusterReconciler) clustersForSecret(ctx context.Context, o client.Object) []reconcile.Request {
	var list cachev1.RedisClusterList
	if err := r.List(ctx, &list, client.InNamespace(o.GetNamespace()),
		client.MatchingFields{".spec.passwordSecretRef.name": o.GetName()}); err != nil {
		log.FromContext(ctx).Error(err, "listing RedisClusters for Secret")
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(list.Items))
	for _, rc := range list.Items {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: rc.Namespace, Name: rc.Name}})
	}
	return reqs
}
```

Rules:
- `GenerationChangedPredicate` on the primary resource skips status-only and metadata-only updates. Do **not** put it on `Owns()` for kinds whose status you react to (Deployment availability is in status, generation does not change).
- Watching all Secrets/ConfigMaps cluster-wide caches all of them in memory. Restrict the cache (`cache.Options.ByObject` with label/field selectors, `DefaultNamespaces`) or use `WatchesMetadata` when only metadata is needed.
- Cache transforms (`cache.Options.DefaultTransform`, `ByObject[...].Transform`) to strip fields were broken by a client-go v0.34.0–v0.34.2 regression (transformers ignored); require ≥ v0.34.3 when relying on them.
- Map functions must be cheap and must use indexed `List` (`MatchingFields`), not full scans.
- The `client.MatchingFields` key must be the exact string registered with `IndexField`. For CRD `selectableFields` the server-side field selector key has no leading dot (`spec.version`), while a cache index key is whatever you registered.

---

#### Finalizers, Owner References & Garbage Collection

##### Finalizers

- Use finalizers **only** for cleanup of things outside Kubernetes or outside the owner-reference graph (cloud resources, DNS records, cross-namespace objects).
- Names are qualified: `cache.example.com/external-cleanup` (not `cleanup`).
- Add the finalizer **before** creating the external resource; remove it **only after** cleanup succeeded.
- Cleanup must be idempotent and tolerate "already gone".
- Never add a finalizer to an object that has `deletionTimestamp` set (the apiserver rejects adding new finalizers during deletion).
- Do not create resources in a namespace that is terminating. **Ordered namespace deletion (GA in 1.34)** deletes Pods first and the remaining resources afterwards; a finalizer that waits on objects in a namespace that is itself being deleted can still deadlock.
- A stuck finalizer blocks namespace deletion forever. Report `Stalled`/Events for permanent failures so a human can act.

##### Owner References

```go
// Sets controller: true and blockOwnerDeletion: true
if err := controllerutil.SetControllerReference(owner, child, r.Scheme); err != nil {
	return err
}
```

| Rule | Notes |
|---|---|
| Owner in the **same namespace** as the dependent, or cluster-scoped | A namespaced owner of a cluster-scoped object is invalid: the GC reports `OwnerRefInvalidNamespace` and does not collect it |
| At most **one** `controller: true` reference | `SetControllerReference` returns an `AlreadyOwnedError` if another controller owns it |
| `blockOwnerDeletion: true` requires `update` on `<owner>/finalizers` | hence the `redisclusters/finalizers` RBAC marker |
| Owner UID must be the live owner's UID | a stale UID (owner deleted and recreated with the same name) makes the GC treat the dependent as orphaned and delete it |

| Scenario | Use |
|---|---|
| Child in the same namespace | owner reference (automatic GC) |
| Cluster-scoped child of a namespaced owner | finalizer on the owner |
| External resource | finalizer |
| Cross-namespace child | finalizer + label pointing back at the owner |

---

#### Admission: CEL Policies and Webhooks

##### Validation Hierarchy (prefer left-to-right)

```
OpenAPI schema + CEL in the CRD  →  ValidatingAdmissionPolicy (CEL, in-process)  →  Validating webhook
(per-object invariants)               (cross-cutting rules, any kind)                 (needs other objects / external data)
```

##### ValidatingAdmissionPolicy (`admissionregistration.k8s.io/v1`, GA since 1.30) — verified on 1.34.1

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: deployment-baseline.example.com
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["deployments"]
  matchConditions:
    - name: not-kube-system
      expression: "request.namespace != 'kube-system'"
  variables:
    - name: containers
      expression: "object.spec.template.spec.containers + object.spec.template.spec.?initContainers.orValue([])"
  validations:
    - expression: >-
        variables.containers.all(c, c.image.contains('@sha256:') ||
          (c.image.lastIndexOf(':') > c.image.lastIndexOf('/') && !c.image.endsWith(':latest')))
      message: "every container image must be pinned to a tag or digest (no :latest)"
      reason: Invalid
    - expression: "object.metadata.?labels['app.kubernetes.io/name'].hasValue()"
      messageExpression: "'deployment ' + object.metadata.name + ' must set label app.kubernetes.io/name'"
  auditAnnotations:
    - key: replicas
      valueExpression: "string(object.spec.replicas)"
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: deployment-baseline.example.com
spec:
  policyName: deployment-baseline.example.com
  validationActions: [Deny, Audit]
  matchResources:
    namespaceSelector:
      matchExpressions:
        - {key: kubernetes.io/metadata.name, operator: NotIn, values: [kube-system]}
```

Results on 1.34.1 (`kubectl create --dry-run=server`): `nginx:latest`, `nginx` and `registry.example.com:5000/web` are denied; `registry.example.com:5000/web:1.2` and `…/web@sha256:…` are allowed. A naive `c.image.contains(':')` check wrongly accepts `registry.example.com:5000/web` (registry port) — a common review finding.

Rules:
- A policy without a **binding** does nothing. Check `validationActions` (`Deny`, `Warn`, `Audit`; `Deny` and `Warn` together are not allowed).
- Guard optional fields with `?.`/`has()`; a runtime error is handled by `failurePolicy` (`Fail` denies).
- `matchConditions` filter before validations; use them to exclude system namespaces/users instead of `||` chains in every rule.
- Admission policies run on `--dry-run=server` too — use dry-run in CI to test them.
- VAP/MAP for objects in a namespace created a moment ago could fail with a spurious "namespace not found" before **v1.34.3**.

##### MutatingAdmissionPolicy — beta but **not available by default** in 1.34

`admissionregistration.k8s.io/v1beta1` `MutatingAdmissionPolicy` requires the cluster operator to set **both** `--feature-gates=MutatingAdmissionPolicy=true` and `--runtime-config=admissionregistration.k8s.io/v1beta1=true`. On a default 1.34 apiserver the kind does not exist. The 1.34 storage version is still alpha, so enabling beta requires a storage migration later. Verified with the gate enabled on 1.34.1:

```yaml
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingAdmissionPolicy
metadata:
  name: always-pull-latest.example.com
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
    - name: uses-latest
      expression: "object.spec.containers.exists(c, c.image.endsWith(':latest'))"
  mutations:
    - patchType: ApplyConfiguration
      applyConfiguration:
        expression: >
          Object{
            spec: Object.spec{
              containers: object.spec.containers.filter(c, c.image.endsWith(':latest')).map(c,
                Object.spec.containers{name: c.name, imagePullPolicy: "Always"})
            }
          }
---
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingAdmissionPolicyBinding
metadata:
  name: always-pull-latest.example.com
spec:
  policyName: always-pull-latest.example.com
```

A Pod with containers `nginx:latest` (IfNotPresent) and `registry.example.com/x:1.0` came back with `imagePullPolicy` `Always` and `IfNotPresent` respectively — the apply-configuration merge touched only the listed container (containers merge by `name`). Flag MAP manifests in charts that must run on a default 1.34 cluster; use a mutating webhook or CRD defaults there.

##### Webhooks (controller-runtime v0.22 API)

```go
package webhook

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	cachev1 "example.com/redis-operator/api/v1"
)

// +kubebuilder:webhook:path=/validate-cache-example-com-v1-rediscluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=cache.example.com,resources=redisclusters,verbs=create;update,versions=v1,name=vrediscluster.cache.example.com,admissionReviewVersions=v1,matchPolicy=Equivalent,timeoutSeconds=5

// RedisClusterValidator does checks CEL cannot: it reads other objects.
type RedisClusterValidator struct {
	Reader client.Reader
}

var _ admission.CustomValidator = &RedisClusterValidator{}

func SetupWebhookWithManager(mgr ctrl.Manager) error {
	// v0.22: non-generic builder + For(); v0.23 changes this to NewWebhookManagedBy(mgr, obj)
	return ctrl.NewWebhookManagedBy(mgr).
		For(&cachev1.RedisCluster{}).
		WithValidator(&RedisClusterValidator{Reader: mgr.GetClient()}).
		Complete()
}

func (v *RedisClusterValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	rc, ok := obj.(*cachev1.RedisCluster)
	if !ok {
		return nil, fmt.Errorf("expected RedisCluster, got %T", obj)
	}
	return v.validate(ctx, rc)
}

func (v *RedisClusterValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	rc, ok := newObj.(*cachev1.RedisCluster)
	if !ok {
		return nil, fmt.Errorf("expected RedisCluster, got %T", newObj)
	}
	return v.validate(ctx, rc)
}

func (v *RedisClusterValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *RedisClusterValidator) validate(ctx context.Context, rc *cachev1.RedisCluster) (admission.Warnings, error) {
	var errs field.ErrorList
	if ref := rc.Spec.PasswordSecretRef; ref != nil {
		var list cachev1.RedisClusterList
		if err := v.Reader.List(ctx, &list, client.InNamespace(rc.Namespace)); err != nil {
			return nil, apierrors.NewInternalError(err)
		}
		for _, other := range list.Items {
			if other.Name != rc.Name && other.Spec.PasswordSecretRef != nil && other.Spec.PasswordSecretRef.Name == ref.Name {
				errs = append(errs, field.Duplicate(field.NewPath("spec", "passwordSecretRef", "name"), ref.Name))
			}
		}
	}
	if len(errs) > 0 {
		return nil, apierrors.NewInvalid(cachev1.GroupVersion.WithKind("RedisCluster").GroupKind(), rc.Name, errs)
	}
	return nil, nil
}
```

Webhook rules:
- `failurePolicy: Fail` for security/integrity checks; `Ignore` only for best-effort. With `Fail`, an unavailable webhook blocks writes — run ≥ 2 replicas with a PodDisruptionBudget and exclude the webhook's own namespace.
- `sideEffects: None` (or `NoneOnDryRun`) — otherwise `--dry-run=server` requests are rejected.
- `matchPolicy: Equivalent` so the webhook also sees requests through other versions of the resource.
- `timeoutSeconds` small (1–10; the maximum is 30). Use `namespaceSelector`/`objectSelector`/`matchConditions` (CEL) to avoid calling the webhook for irrelevant objects.
- Mutating webhooks must be idempotent and reinvocation-safe (`reinvocationPolicy: IfNeeded`). **[NEW in 1.34]** a mutating webhook response whose patch cannot be decoded now counts as a webhook call failure and follows `failurePolicy` (previously a different error path).
- Certificates must rotate (cert-manager or controller-runtime's cert watcher); do not bake certs into images.
- A webhook must not intercept the objects it needs to start (its own Deployment, Pods, Secrets, Leases); exclude its namespace with `namespaceSelector`, otherwise a restart with `failurePolicy: Fail` deadlocks.

---

#### Workload Manifests in 1.34

##### A Production Deployment for 1.34 (passes `kubectl apply --dry-run=server` on 1.34.1 in a `restricted` namespace, no warnings)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: apps
  labels: {app.kubernetes.io/name: web}
spec:
  replicas: 3
  revisionHistoryLimit: 5
  selector:
    matchLabels: {app.kubernetes.io/name: web}
  strategy:
    type: RollingUpdate
    rollingUpdate: {maxUnavailable: 0, maxSurge: 1}
  template:
    metadata:
      labels: {app.kubernetes.io/name: web}
    spec:
      serviceAccountName: web
      automountServiceAccountToken: false
      terminationGracePeriodSeconds: 30
      securityContext:
        runAsNonRoot: true
        runAsUser: 10001
        runAsGroup: 10001
        fsGroup: 10001
        seccompProfile: {type: RuntimeDefault}
      resources:                      # pod-level budget (beta, on by default in 1.34)
        requests: {cpu: 500m, memory: 512Mi}
        limits: {memory: 512Mi}
      initContainers:
        - name: log-forwarder         # native sidecar: starts before, stops after the app
          image: registry.example.com/log-forwarder:2.3.1
          restartPolicy: Always
          resources:
            requests: {cpu: 50m, memory: 64Mi}
          securityContext: &csc
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: {drop: ["ALL"]}
      containers:
        - name: web
          image: registry.example.com/web:1.27.2
          ports: [{name: http, containerPort: 8080}]
          startupProbe:
            httpGet: {path: /healthz, port: http}
            failureThreshold: 30
            periodSeconds: 2
          readinessProbe:
            httpGet: {path: /readyz, port: http}
          livenessProbe:
            httpGet: {path: /healthz, port: http}
          lifecycle:
            preStop:
              sleep: {seconds: 5}     # GA in 1.34: no shell or sleep binary needed
          resizePolicy:
            - {resourceName: cpu, restartPolicy: NotRequired}
          securityContext: *csc
          volumeMounts: [{name: tmp, mountPath: /tmp}]
      volumes:
        - name: tmp
          emptyDir: {sizeLimit: 64Mi}
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: topology.kubernetes.io/zone
          whenUnsatisfiable: ScheduleAnyway
          labelSelector:
            matchLabels: {app.kubernetes.io/name: web}
          matchLabelKeys: [pod-template-hash]
---
apiVersion: v1
kind: ServiceAccount
metadata: {name: web, namespace: apps}
automountServiceAccountToken: false
---
apiVersion: v1
kind: Service
metadata: {name: web, namespace: apps}
spec:
  selector: {app.kubernetes.io/name: web}
  ports: [{name: http, port: 80, targetPort: http}]
  trafficDistribution: PreferSameZone   # beta/on in 1.34; use PreferClose if 1.33 must be supported
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata: {name: web, namespace: apps}
spec:
  maxUnavailable: 1
  unhealthyPodEvictionPolicy: AlwaysAllow
  selector:
    matchLabels: {app.kubernetes.io/name: web}
```

Pod Security Admission **enforces** only on Pods. For a Deployment it only *warns* (`Warning: would violate PodSecurity "restricted:latest": allowPrivilegeEscalation != false …`) and the Deployment is created — the ReplicaSet controller then fails to create Pods. Treat PSA warnings in CI dry-runs as errors.

##### Pod-Level Resources — **[NEW in 1.34: beta, on by default]**

- `spec.resources.requests/limits` (CPU, memory, hugepages) set a budget for the whole Pod; containers without their own values share it.
- On 1.33 the field is silently dropped (gate alpha/off) — a manifest that relies on it has no limits at all there.
- Rejected when `spec.os.name: windows`. The Downward API `resourceFieldRef` falls back to pod-level limits before node allocatable. CPU/Memory/Topology Manager alignment is not performed for Pods that use pod-level resources.
- HPA `Resource` metrics use pod-level requests when they are set.
- Pod-level values cannot be resized in place in 1.34 (in-place resize of pod-level resources is 1.35).

##### In-Place Resize (beta since 1.33)

- Resize through the `resize` subresource: `kubectl patch pod rs --subresource resize -p '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"400m"},"limits":{"cpu":"800m"}}}]}}'` (verified on 1.34.1). A normal `PATCH` of `spec.containers[].resources` is rejected ("pod updates may not change fields other than …").
- Use a **strategic-merge** patch (kubectl's default) or SSA. A JSON merge patch replaces the whole `containers` list and fails validation ("resource requests cannot be removed", "only cpu and memory resources are mutable").
- `resizePolicy[].restartPolicy`: `NotRequired` (default) or `RestartContainer`. **[NEW in 1.34]** memory limits may be *decreased* with `NotRequired`; the kubelet makes a best-effort check that usage is below the new limit. Guaranteed Pods with the static memory manager policy still cannot resize memory unless `InPlacePodVerticalScalingExclusiveMemory` (alpha) is enabled.
- Resize progress is reported through Pod conditions `PodResizePending` / `PodResizeInProgress` and, with generation tracking (beta in 1.34), through `status.observedGeneration`.

##### Sidecars, Lifecycle, Probes

- Native sidecars: `initContainers[].restartPolicy: Always` (GA since 1.33). They start in order before regular containers, keep running, and are terminated after them. Do not use the old pattern of a regular container plus custom shutdown coordination for log shippers/proxies.
- `preStop.sleep.seconds` (GA in 1.34; `0` allowed and is a no-op). Prefer it over `exec: ["sleep", "5"]`, which fails in distroless images.
- Probe and lifecycle `httpGet.host`/`tcpSocket.host` are **forbidden by the Baseline and Restricted PSS in 1.34** (see Security). Always probe the Pod IP (leave `host` unset) and use `port` names.
- A liveness probe must not depend on downstream services; readiness may. Use `startupProbe` for slow starts instead of large `initialDelaySeconds`.
- 1.34.2 fixed a race where main containers stayed in "Initializing" when a sidecar's startup probe failed initially in Pods with `restartPolicy: Never`.

##### Jobs

```yaml
apiVersion: batch/v1
kind: Job
metadata: {name: train, namespace: default}
spec:
  completionMode: Indexed
  completions: 4
  parallelism: 4
  podReplacementPolicy: Failed      # GA in 1.34: wait until the old Pod is Failed
  backoffLimitPerIndex: 1           # GA in 1.33
  successPolicy:                    # GA in 1.33
    rules: [{succeededIndexes: "0"}]
  podFailurePolicy:
    rules:
      - action: FailIndex
        onExitCodes: {operator: In, values: [42]}
  template:
    spec:
      restartPolicy: Never
      containers: [{name: t, image: registry.example.com/train:2.0}]
```

- `podReplacementPolicy: Failed` avoids two Pods per index at once (frameworks such as TensorFlow/JAX need that). The default is `TerminatingOrFailed`; with a `podFailurePolicy` the default becomes `Failed` and `TerminatingOrFailed` is rejected (`Unsupported value: "TerminatingOrFailed": supported values: "Failed"`, verified on 1.34.1).
- `spec.managedBy` (beta/on) hands a Job to an external controller (e.g. MultiKueue); the built-in Job controller ignores such Jobs. Do not set it unless that controller exists.
- Container-level `restartPolicy`/`restartPolicyRules` for regular containers is alpha/off in 1.34 — use `podFailurePolicy` instead.
- Always set `ttlSecondsAfterFinished` (or let a CronJob's history limits clean up) and `activeDeadlineSeconds` for bounded work.

##### StatefulSets

- `persistentVolumeClaimRetentionPolicy` (GA since 1.32) controls PVC deletion on scale-down/delete.
- `rollingUpdate.maxUnavailable` is **dropped** on 1.34 (alpha; verified: the stored `rollingUpdate` is only `{partition: 0}`) — updates proceed one Pod at a time regardless of the manifest. Do not promise it after an upgrade either: it is honoured by default only on v1.35.0–v1.35.3 (disabled again from v1.35.4 and in 1.36 because of a `Parallel` pod-management regression) and again from 1.37.
- `minReadySeconds` is now honoured by StatefulSets (1.34 fix).
- Upgrading the control plane to 1.34.0/1.34.1 could trigger a **rollout of every existing StatefulSet**; fixed in v1.34.2.

##### Services and Traffic

- `trafficDistribution`: `PreferSameZone` (preferred spelling), `PreferSameNode` (beta/on in 1.34), `PreferClose` (deprecated alias of `PreferSameZone`; 1.33 only knows `PreferClose` and rejects `PreferSameNode` with `Unsupported value`).
- Headless Services (`clusterIP: None`) ignore `externalIPs`, `loadBalancerIP` and `sessionAffinity`; 1.34 warns (`Warning: spec.externalIPs is ignored for headless services`). On v1.34.0/v1.34.1 every headless Service also gets a spurious `spec.SessionAffinity is ignored` warning (fixed in v1.34.2) — not a manifest bug.
- Avoid `externalIPs` entirely (CVE-2020-8554; deprecated from 1.36).
- Service names must still be DNS-1035 labels in 1.34.
- `topologySpreadConstraints[].matchLabelKeys` values are merged into `labelSelector` by the apiserver (observed: `pod-template-hash: abc123` became a `matchExpressions` entry). Keys listed in `matchLabelKeys` must not also appear in `labelSelector` (rejected: `exists in both matchLabelKeys and labelSelector`).

##### Volumes and Storage

- `VolumeAttributesClass` is **GA** (`storage.k8s.io/v1`); PVC `spec.volumeAttributesClassName` may now be changed back to nil. The CSI driver must implement `ModifyVolume`.
- Image volumes (`volumes[].image`) are beta but **off** in 1.34: the apiserver removes the volume **and its `volumeMounts`** without an error (verified). Do not ship them for 1.34 unless the cluster enables `ImageVolume` and runs containerd ≥ 2.1 / CRI-O with support.
- `gitRepo` volumes: deprecated, the kubelet driver is disabled by default since 1.33 (apiserver warns), removed in 1.36. Use an init container or git-sync.
- `emptyDir.sizeLimit` on every scratch volume; `recursiveReadOnly: Enabled` (GA 1.33) with `readOnly: true` for mounts that must be read-only including submounts.
- Volume expansion: since 1.34 (GA) a failed expansion can be retried with a *smaller* size, but never smaller than the original capacity.

##### User Namespaces

`spec.hostUsers: false` (beta/on) maps container root to an unprivileged host UID. The apiserver accepts it on any 1.34 cluster, but the node needs Linux ≥ 6.3 for idmap mounts and a runtime that supports it; otherwise Pod creation fails at the kubelet. **[NEW in 1.34]** combining `hostUsers: false` with `volumeDevices` is rejected.

---

#### Dynamic Resource Allocation (resource.k8s.io/v1)

**[NEW in 1.34: GA]** Write DRA objects with `resource.k8s.io/v1`. The v1 request shape wraps the device selection in `exactly` (or `firstAvailable`) — the flat v1beta1 shape is rejected (`strict decoding error: unknown field "spec.devices.requests[0].deviceClassName"`).

```yaml
apiVersion: resource.k8s.io/v1
kind: DeviceClass
metadata:
  name: gpu.example.com
spec:
  selectors:
    - cel:
        expression: device.driver == "gpu.example.com"
---
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata: {name: one-gpu, namespace: default}
spec:
  spec:
    devices:
      requests:
        - name: gpu
          exactly:
            deviceClassName: gpu.example.com
            allocationMode: ExactCount
            count: 1
            selectors:
              - cel:
                  expression: device.capacity["gpu.example.com"].memory.compareTo(quantity("40Gi")) >= 0
---
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata: {name: big-or-two-small, namespace: default}
spec:
  spec:
    devices:
      requests:
        - name: gpu
          firstAvailable:              # DRAPrioritizedList: beta/on in 1.34
            - name: big
              deviceClassName: gpu.example.com
              selectors:
                - cel: {expression: 'device.attributes["gpu.example.com"].model == "H100"'}
            - name: small
              deviceClassName: gpu.example.com
              count: 2
---
apiVersion: v1
kind: Pod
metadata: {name: trainer, namespace: default}
spec:
  resourceClaims:
    - name: gpu
      resourceClaimTemplateName: one-gpu
  containers:
    - name: trainer
      image: registry.example.com/trainer:1.0.0
      resources:
        claims:
          - name: gpu
```

All four objects are accepted by 1.34.1; on 1.33.0 the `resource.k8s.io/v1` kinds do not exist (and the DRA beta APIs are off by default there).

Review rules:
- `adminAccess: true` is only accepted in namespaces labelled `resource.kubernetes.io/admin-access: "true"` (verified error: "admin access to devices requires the `resource.kubernetes.io/admin-access: true` label on the containing namespace"). The label key changed in 1.34 from `resource.k8s.io/admin-access`; set both during a 1.33 → 1.34 upgrade.
- A ResourceClaim allocates at most **32 devices** (1.33 accidentally limited it to 16; restored in 1.34).
- Device selectors are CEL with `device.driver`, `device.attributes["<domain>"].<name>`, `device.capacity["<domain>"].<name>` (quantities: use `compareTo(quantity(...))`).
- `ResourceClaimTemplate` per Pod, `ResourceClaim` for sharing one allocation across Pods.
- Static Pods may not reference ResourceClaims (1.34 closes that loophole; the kubelet now rejects static Pods that reference any API object).
- Drivers: use the `k8s.io/dynamic-resource-allocation/kubeletplugin` helper from 1.34 (supports the v1 and the deprecated v1beta1 kubelet gRPC API; v1alpha4 is removed).
- Extended-resource mapping (`DeviceClass.spec.extendedResourceName`), consumable capacity and binding conditions are alpha/off in 1.34.

---

#### RBAC & Least Privilege

1. Grant only the verbs/resources needed; prefer `Role`/`RoleBinding` over cluster-wide bindings.
2. **No wildcards** in verbs, resources or apiGroups for workloads/operators.
3. **No `cluster-admin`** for service accounts; no `escalate`, `bind`, `impersonate` unless that is the controller's purpose.
4. `secrets` `list`/`watch` across all namespaces is equivalent to reading every secret — restrict by namespace or `resourceNames`. `resourceNames` cannot restrict `create` or `deletecollection`; a `list`/`watch` restricted by `resourceNames` is only authorized when the client sends a matching `metadata.name` field selector.
5. `pods/exec`, `pods/attach`, `nodes/proxy`, `serviceaccounts/token` `create` are privilege-escalation paths; flag them.
6. Status and finalizers are separate resources: `<resource>/status` (`update`, `patch`), `<resource>/finalizers` (`update`, needed for `blockOwnerDeletion`).
7. Leader election: `coordination.k8s.io` `leases` (`get`, `create`, `update`) in the manager namespace; Events: `create`, `patch` on `events` (core) or `events.k8s.io`.

```go
// +kubebuilder:rbac:groups="",resources=pods;services;configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
```

**[NEW in 1.34: GA]** Authorization with field and label selectors: webhook authorizers and the node authorizer can allow `list`/`watch` only when the request carries a specific selector (for example `spec.nodeName=<node>`). A client restricted that way must send the selector, or the request is forbidden. CEL in admission policies can check it with `authorizer.group('').resource('pods').fieldSelector('spec.nodeName=mynode').check('list').allowed()`. Built-in RBAC itself does not support selector-scoped rules.

Service account defaults:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: redis-operator
  namespace: redis-system
automountServiceAccountToken: false   # opt in per Pod that needs API access
```

---

#### Security Hardening & Pod Security Admission

##### Pod Security Standards in 1.34

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: apps
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: v1.34   # or "latest"
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/audit: restricted
```

**[NEW in 1.34]** Probe/lifecycle host check (KEP-4940), verified on 1.34.1:

```yaml
# BAD — rejected in a restricted (and baseline) namespace on 1.34:
#   violates PodSecurity "restricted:v1.34": probe or lifecycle host
#   (container "api" uses probe or lifecycle host "169.254.169.254")
livenessProbe:
  httpGet: {path: /healthz, port: http, host: 169.254.169.254}

# GOOD — no host: the kubelet probes the Pod IP
livenessProbe:
  httpGet: {path: /healthz, port: http}
```

With `enforce-version: v1.33` the same Pod is **accepted** (only a `warn` label at `latest` reports it). Review namespaces pinned to old PSS versions: pinning is meant as a temporary migration aid and silently disables new checks.

Restricted container baseline:

```yaml
securityContext:              # Pod
  runAsNonRoot: true
  seccompProfile: {type: RuntimeDefault}
containers:
  - securityContext:          # every container, init container and ephemeral container
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true      # not required by PSS, but expected
      capabilities: {drop: ["ALL"]}     # only NET_BIND_SERVICE may be added back
```

- AppArmor: use `securityContext.appArmorProfile: {type: RuntimeDefault | Localhost, localhostProfile: …}`. **[NEW in 1.34]** the field is no longer mirrored into the deprecated annotation (verified: no `container.apparmor.security.beta.kubernetes.io/*` annotation on 1.34.1; 1.33.0 still added it). Writing the annotation still populates the field, with a deprecation warning.
- `procMount: Unmasked` only with `hostUsers: false`; `privileged`, `hostPID`, `hostIPC`, `hostNetwork`, `hostPath` require Privileged level and explicit justification.
- `supplementalGroupsPolicy: Strict` (beta/on) prevents image `/etc/group` from adding groups.
- `seLinuxChangePolicy: MountOption` (beta/on) speeds up SELinux relabeling where all Pods sharing a volume use the same label; mixed labels on a shared volume need `Recursive`.

##### Secrets

- Never put secrets in CR `spec`, ConfigMaps, env literals, or Helm values committed to Git. Reference Secrets by name; prefer mounted files over env vars (env leaks into crash dumps and child processes).
- Projected service-account tokens with `audience` and `expirationSeconds`; never long-lived `kubernetes.io/service-account-token` Secrets.
- **[NEW in 1.34, beta]** kubelet image credential providers can use short-lived, audience-bound ServiceAccount tokens instead of node-wide pull secrets (`tokenAttributes.cacheType` is required in the v1 `CredentialProviderConfig`).
- Do not log secret values or whole objects that contain them.

##### Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: default-deny, namespace: apps}
spec:
  podSelector: {}
  policyTypes: [Ingress, Egress]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: allow-dns-and-api, namespace: apps}
spec:
  podSelector: {matchLabels: {app.kubernetes.io/name: web}}
  policyTypes: [Egress]
  egress:
    - to:
        - namespaceSelector: {matchLabels: {kubernetes.io/metadata.name: kube-system}}
          podSelector: {matchLabels: {k8s-app: kube-dns}}
      ports: [{port: 53, protocol: UDP}, {port: 53, protocol: TCP}]
```

Start from default-deny; allow DNS explicitly; the API server is reached through the `kubernetes` Service, whose endpoints are host IPs (use `ipBlock` for them, NetworkPolicy cannot select the control plane by label).

##### Images

- Pin images by digest in production (or at least an immutable tag); never `latest` (implies `imagePullPolicy: Always`).
- Distroless/scratch bases for operators; scan in CI; sign and verify (policy engine or admission).
- Note for multi-tenant clusters: `imagePullPolicy: IfNotPresent` lets any Pod on a node use a cached private image; credential verification for cached images is alpha in 1.34 (`KubeletEnsureSecretPulledImages`) and beta/on in 1.35. Use `Always` for sensitive private images on shared nodes.

---

#### client-go Specifics for v0.34

##### A Plain client-go Controller (compiles against client-go v0.34.12)

```go
package clientgo

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type Controller struct {
	queue   workqueue.TypedRateLimitingInterface[cache.ObjectName]
	lister  listersv1.ConfigMapLister
	synced  cache.InformerSynced
	handler func(ctx context.Context, cm *corev1.ConfigMap) error
}

func New(ctx context.Context, cs kubernetes.Interface, ns string, h func(context.Context, *corev1.ConfigMap) error) (*Controller, informers.SharedInformerFactory, error) {
	factory := informers.NewSharedInformerFactoryWithOptions(cs, 10*time.Minute, informers.WithNamespace(ns))
	inf := factory.Core().V1().ConfigMaps()
	c := &Controller{
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[cache.ObjectName](),
			workqueue.TypedRateLimitingQueueConfig[cache.ObjectName]{Name: "configmaps"}),
		lister:  inf.Lister(),
		synced:  inf.Informer().HasSynced,
		handler: h,
	}
	enqueue := func(obj any) {
		if name, err := cache.DeletionHandlingObjectToName(obj); err == nil {
			c.queue.Add(name)
		} else {
			utilruntime.HandleErrorWithContext(ctx, err, "cannot compute key")
		}
	}
	if _, err := inf.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    enqueue,
		UpdateFunc: func(_, newObj any) { enqueue(newObj) },
		DeleteFunc: enqueue,
	}); err != nil {
		return nil, nil, err
	}
	return c, factory, nil // caller: factory.Start(ctx.Done()); go c.Run(ctx, 2)
}

func (c *Controller) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrashWithContext(ctx)
	defer c.queue.ShutDown()
	if !cache.WaitForCacheSync(ctx.Done(), c.synced) {
		return fmt.Errorf("caches did not sync")
	}
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, time.Second)
	}
	<-ctx.Done()
	return nil
}

func (c *Controller) worker(ctx context.Context) {
	for c.next(ctx) {
	}
}

func (c *Controller) next(ctx context.Context) bool {
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(key)
	cm, err := c.lister.ConfigMaps(key.Namespace).Get(key.Name)
	switch {
	case apierrors.IsNotFound(err):
		c.queue.Forget(key)
		return true
	case err != nil:
		c.queue.AddRateLimited(key)
		return true
	}
	// Never mutate objects from the lister: they are shared cache entries.
	if err := c.handler(ctx, cm.DeepCopy()); err != nil {
		klog.FromContext(ctx).Error(err, "sync failed", "key", key)
		c.queue.AddRateLimited(key)
		return true
	}
	c.queue.Forget(key)
	return true
}
```

Review rules for client-go v0.34:
- **Typed workqueues** (`workqueue.TypedRateLimitingInterface[T]`, `NewTypedRateLimitingQueueWithConfig`, `DefaultTypedControllerRateLimiter[T]`) — the untyped `RateLimitingInterface`/`NewRateLimitingQueue*` are deprecated. `cache.ObjectName` + `cache.DeletionHandlingObjectToName` replace string keys and `MetaNamespaceKeyFunc` parsing.
- Every `Get` from the queue needs a matching `Done`; call `Forget` on success, otherwise the per-item backoff keeps growing.
- `AddEventHandler` returns `(ResourceEventHandlerRegistration, error)`; check the error. Use `registration.HasSynced` when the handler was added after the informer started.
- Context-aware helpers: `wait.PollUntilContextTimeout`/`PollUntilContextCancel` instead of the deprecated `wait.Poll*`/`PollImmediate*`; `utilruntime.HandleErrorWithContext`/`HandleCrashWithContext`; `klog.FromContext(ctx)` for contextual logging.
- `k8s.io/utils/ptr` (`ptr.To`, `ptr.Deref`) instead of the deprecated `k8s.io/utils/pointer` — the Kubernetes code base finished this migration in 1.34.
- **JSON/YAML output changes**: zero `metadata.creationTimestamp` is omitted (`json:"creationTimestamp,omitempty,omitzero"` in apimachinery v0.34; marshalling `metav1.ObjectMeta{Name: "x"}` yields `{"name":"x"}` with v0.34.12 and `{"name":"x","creationTimestamp":null}` with v0.33.5). Golden files, generated manifests and diff-based tests that contain `creationTimestamp: null` change when upgrading.
- Validation error strings from the apiserver were shortened in 1.34 (redundant field names removed). Tests must assert with `apierrors.IsInvalid`, `apierrors.IsConflict` and the `StatusDetails.Causes`, not with full message strings.
- **Streaming lists**: client-go v0.34 removed the API-streaming path from typed, dynamic and metadata `List()`. Streaming initial lists remain available to informers/reflectors when `WatchListClient` is enabled (client-go feature gate, beta, **default off**; enable with the environment variable `KUBE_FEATURE_WatchListClient=true`). The 1.34 apiserver serves WatchList by default.
- Other client-go gates in v0.34: `InOrderInformers` beta/on, `InformerResourceVersion` alpha, `ClientsAllowCBOR`/`ClientsPreferCBOR` alpha. Do not enable CBOR against servers without `CBORServingAndStorage` (alpha in 1.34).
- `MessageCountMap` and `CreateAggregateFromMessageCountMap` in `k8s.io/apimachinery/pkg/util/errors` are deprecated in 1.34.
- Scheduler-plugin authors: framework types (`CycleState`, `NodeInfo`, `QueueingHint`, `Code`, `Status`, …) moved to `k8s.io/kube-scheduler/framework` in 1.34.
- Leader election LeaseLocks can carry custom labels in 1.34.

---

#### Observability

##### Structured Logging (logr)

```go
// GOOD — structured key/value pairs
logger := log.FromContext(ctx).WithValues("redisCluster", klog.KObj(rc), "generation", rc.Generation)
logger.Info("starting reconcile")
logger.Error(err, "failed to apply Deployment", "deployment", rc.Name)

// BAD — formatted strings defeat log processing
logger.Info(fmt.Sprintf("reconciling %s/%s", req.Namespace, req.Name))
```

- controller-runtime already adds `controller`, `namespace`, `name`, `reconcileID` to the reconcile logger.
- Levels: `Info` (V(0)) for lifecycle events, `V(1)` per-reconcile summaries, `V(2)`+ details. Never log Secret data, tokens or full objects containing them.

##### Metrics

- controller-runtime exposes `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_terminal_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds`, `workqueue_*` on the metrics endpoint. Register custom metrics on `sigs.k8s.io/controller-runtime/pkg/metrics.Registry`.
- Do not label metrics with unbounded values (object names in large clusters, UIDs, error strings).
- **[NEW in 1.34]** apiserver/etcd metric labels changed: `apiserver_cache_list_*`, `etcd_request_duration_seconds`, `etcd_requests_total`, `etcd_request_errors_total`, `apiserver_watch_events_*`, `apiserver_request_body_size_bytes` and many watch-cache metrics now use `group` + `resource` labels instead of `resource_prefix`, `type` or `kind`; `apiserver_storage_objects` is deprecated in favour of `apiserver_resource_objects`. Recording rules and alerts written for ≤ 1.33 silently stop matching.

```go
var reconcileErrors = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "redis_operator_reconcile_errors_total",
		Help: "Total number of reconcile errors by reason.",
	},
	[]string{"reason"}, // bounded label set
)

func init() { metrics.Registry.MustRegister(reconcileErrors) }
```

##### Health Probes

```go
_ = mgr.AddHealthzCheck("healthz", healthz.Ping)
_ = mgr.AddReadyzCheck("cache-sync", func(req *http.Request) error {
	if !mgr.GetCache().WaitForCacheSync(req.Context()) {
		return errors.New("informer caches not synced")
	}
	return nil
})
```

`WaitForCacheSync` is not a `healthz.Checker` — wrap it as above. Webhook servers should add `mgr.GetWebhookServer().StartedChecker()` to readiness.

##### Events

- Emit Events on user-visible transitions (Warning for failures that need action, Normal for completed transitions), not on every reconcile.
- One-shot Jobs/CronJobs may rely on logs and exit status when there is no owned object status to update.
- Missing optional resources in a cleanup pass do not need Events unless users must act.

---

#### Error Handling & Requeue Strategy

```go
// Transient — return the error, controller-runtime backs off per item
return ctrl.Result{}, fmt.Errorf("creating Service: %w", err)

// Permanent (bad user input) — record it, don't spin
meta.SetStatusCondition(&rc.Status.Conditions, metav1.Condition{
	Type: conditionReady, Status: metav1.ConditionFalse, Reason: "InvalidSpec",
	Message: err.Error(), ObservedGeneration: rc.Generation,
})
if err := r.Status().Update(ctx, rc); err != nil {
	return ctrl.Result{}, err
}
return ctrl.Result{}, reconcile.TerminalError(err) // not requeued; counted as terminal

// Polling external state
return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
```

Rules:
- Do not swallow errors (`return ctrl.Result{}, nil` after a failed write hides it and stops retries).
- Do not return `Requeue: true` together with an error; `Requeue` itself is deprecated in v0.22.
- Use `client.IgnoreNotFound` for fetches of objects that may be gone; `apierrors.IsAlreadyExists` on creates in racy paths.
- Bound retries in webhooks and external calls with contexts; `ReconciliationTimeout` or explicit `context.WithTimeout`.
- The default per-item backoff caps at 1000 s (~16.7 min); do not fight it with ad-hoc sleeps.

##### Jobs and CronJobs Written as Controllers

Idempotent one-shot Jobs and CronJobs can rely on Kubernetes termination and safe reruns. Require explicit graceful shutdown only when there is cleanup that process exit cannot handle (held leases or locks, external reservations, non-idempotent remote writes). External calls still need contexts, timeouts or `activeDeadlineSeconds`; absence of signal handling alone is not a bug.

---

#### Testing (fake client, envtest 1.34)

##### Unit Tests with the Fake Client (controller-runtime v0.22.5)

```go
func TestReconcileFakeClient(t *testing.T) {
	s := runtime.NewScheme()
	if err := errors.Join(clientgoscheme.AddToScheme(s), cachev1.AddToScheme(s)); err != nil {
		t.Fatal(err)
	}
	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(&cachev1.RedisCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default"},
			Spec:       cachev1.RedisClusterSpec{Version: "7.2.4", Replicas: 3, MinReplicas: 1, StorageClassName: "fast"},
		}).
		WithStatusSubresource(&cachev1.RedisCluster{}). // required for Status().Update since v0.15
		Build()
	r := &RedisClusterReconciler{Client: c, Scheme: s, External: &fakeExternal{}}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "cache"}}
	for i := 0; i < 2; i++ { // idempotency: reconcile twice
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("reconcile %d: %v", i, err)
		}
	}
	var dep appsv1.Deployment
	if err := c.Get(context.Background(), req.NamespacedName, &dep); err != nil {
		t.Fatal(err)
	}
	if *dep.Spec.Replicas != 3 {
		t.Fatalf("replicas = %d", *dep.Spec.Replicas)
	}
}
```

The v0.22 fake client implements `Apply` (server-side apply) using the client-go type converter for built-in types and a *deduced* converter for others. It does **not** run OpenAPI/CEL validation, defaulting, admission or garbage collection — anything that depends on those must be tested with envtest.

##### Integration Tests with envtest

```go
func TestEnvtest(t *testing.T) {
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd")},
		ErrorIfCRDPathMissing: true,
		// setup-envtest use 1.34.x --bin-dir ./bin  (or KUBEBUILDER_ASSETS)
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s", "1.34.1-linux-amd64"),
	}
	cfg, err := env.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = env.Stop() })
	c, err := client.New(cfg, client.Options{Scheme: newScheme(t)})
	if err != nil {
		t.Fatal(err)
	}
	bad := sample()
	bad.Spec.Replicas, bad.Spec.MinReplicas = 1, 2
	if err := c.Create(context.Background(), bad); !apierrors.IsInvalid(err) {
		t.Fatalf("want Invalid from CEL rule, got %v", err)
	}
}
```

envtest runs a real kube-apiserver + etcd: CRD schemas, CEL rules, defaulting, SSA, field selectors on `selectableFields`, finalizers (deletion is blocked until removal) and admission policies behave exactly as in a cluster. It has **no controllers**: no garbage collection of owned objects, no namespace deletion, no Deployment → ReplicaSet → Pod chain, no ServiceAccount token controller. Tests that wait for those to happen will hang.

##### Testing Rules

- Run the envtest binaries of the cluster minor you target (`ENVTEST_K8S_VERSION=1.34`); a 1.33 envtest rejects 1.34 CEL (semver) and DRA v1 objects.
- Test idempotency (reconcile twice), the deletion path including finalizer removal, status on success and failure, and invalid objects rejected by the CRD.
- Assert errors with `apierrors.Is*` helpers, not message strings (1.34 shortened several validation messages).
- Check generated artifacts in CI: `make generate manifests && git diff --exit-code`.
- Validate rendered manifests against a real 1.34 apiserver (`kubectl apply --dry-run=server`, kind, or envtest), not only with client-side schema validation — client-side tools do not know which gated fields the server will drop.

---

#### Helm and Rendered Manifests

- Set `kubeVersion` in `Chart.yaml` when the chart uses 1.34-only APIs or fields: `kubeVersion: ">=1.34.0-0"` (the `-0` admits pre-release/vendor suffixes such as `v1.34.12-eks-…`). Verified with Helm v3.22.1: rendering with `--kube-version 1.33.0` fails with `chart requires kubeVersion: >=1.34.0-0 … which is incompatible with Kubernetes v1.33.0`.
- `.Capabilities.APIVersions.Has "resource.k8s.io/v1/ResourceClaimTemplate"` is true against a live 1.34 cluster (`helm install --dry-run=server`), but **`helm template --kube-version 1.34.1` does not add `resource.k8s.io/v1` to the offline capability list** (Helm v3.22.1, v4.0.5 and v4.3.1 all rendered the `else` branch). Offline rendering needs `--api-versions resource.k8s.io/v1/ResourceClaimTemplate`; otherwise CI renders a different manifest than the cluster gets.
- Do not gate on `.Capabilities.KubeVersion` for features that depend on feature gates (pod-level resources, image volumes, MAP): the version says nothing about the gate. Gate on an explicit value (`features.imageVolumes: false`) and document it.
- Charts that template `trafficDistribution: PreferSameNode`, `restartPolicyRules`, `fileKeyRef`, `hostnameOverride`, StatefulSet `maxUnavailable` or HPA `tolerance` for 1.34 produce objects without those settings (dropped) — a values option that silently does nothing is a bug.
- Validate the rendered output with `kubectl apply --dry-run=server` against 1.34 in CI.

---

#### Removed / Changed in 1.34 — Migration Notes

| Area | Before (≤ 1.33) | 1.34 | Action |
|---|---|---|---|
| DRA API | `resource.k8s.io/v1beta1`/`v1beta2`, off by default; v1beta1 shape `requests[].deviceClassName` | `resource.k8s.io/v1` served by default; `requests[].exactly.deviceClassName` / `firstAvailable` | Migrate manifests and Go code to `k8s.io/api/resource/v1`; `resource.k8s.io/v1alpha3` only has `DeviceTaintRule` now |
| DRA admin namespace label | `resource.k8s.io/admin-access` | `resource.kubernetes.io/admin-access` | Set both during the upgrade, then drop the old one |
| VolumeAttributesClass | `storage.k8s.io/v1beta1` (off by default) | `storage.k8s.io/v1` GA | Switch `apiVersion` |
| PSS probe host | allowed | forbidden in Baseline/Restricted | Remove `host` from probes/lifecycle handlers; review `enforce-version` pins |
| AppArmor annotations | mirrored from fields | not mirrored | Read `securityContext.appArmorProfile` |
| `creationTimestamp: null` | emitted for zero values | omitted (`omitzero`) | Regenerate golden files |
| CRD unknown `format` | silently ignored | warning on write | Replace with `pattern`/CEL; treat warnings as errors in CI |
| `PreferClose` | only value | deprecated alias of `PreferSameZone`; `PreferSameNode` added | Use `PreferSameZone` once 1.33 support is dropped |
| Pod-level resources | dropped (alpha) | honoured (beta/on) | Verify capacity planning and quotas; Windows Pods may not use it |
| Leader-election lock | `configmapsleases`/`endpointsleases` had APF FlowSchemas | FlowSchemas removed | Use `leases` |
| Aggregated APIs | Endpoints used for proxying | EndpointSlices used | Provide EndpointSlices for hand-managed APIService backends |
| Metrics | `resource_prefix`, `type`, `kind` labels | `group` + `resource` labels | Update dashboards/alerts |
| Kubelet | `cgroupDriver` config, `--cloud-config`, `--register-schedulable` | `cgroupDriver` deprecated, flags removed | Rely on CRI cgroup-driver detection; containerd 2.x before 1.36 |
| client-go `List()` streaming | available behind `WatchListClient` | removed from `List()`; informers only | Use informers for streaming |
| controller-runtime | v0.21 | v0.22: `client.Apply`, `ReconciliationTimeout`, `EnableWarmup`; `Result.Requeue` deprecated | Replace `Requeue: true` with `RequeueAfter` or an error |
| Go | k8s.io/* v0.33 needs Go 1.24 | v0.34.0–v0.34.10 Go 1.24, v0.34.11+ Go 1.25 | Bump the Go toolchain with the patch |

Upgrade path: 1.33 → 1.34 directly (one minor at a time; never skip from 1.32, because the `matchLabelKeys` selector merge requires Pods created on 1.32 to be scheduled before reaching 1.34). Use at least **v1.34.4** (StatefulSet rollout, informer transformer, CR status-schema panic and env-var regressions fixed; v1.34.9 for binary Secret env values) — in practice the latest v1.34.12, and plan the move to 1.35+ because 1.34 is end of life on 2026-10-27.

---

#### Production / Review Checklists

##### Version & Dependencies

- [ ] All `k8s.io/*` staging modules on the same v0.34.x patch (≥ v0.34.3); controller-runtime v0.22.x; controller-gen v0.19.x
- [ ] Go toolchain ≥ 1.25 when `k8s.io/*` ≥ v0.34.11
- [ ] No API version that 1.34 does not serve by default (`*/v1beta*` for DRA, MAP, VolumeAttributesClass; removed betas)
- [ ] No 1.35+ features (in-place pod-level resize, `workloadRef`, toleration `Gt`/`Lt`, `serviceAccountTokenInSecrets`, enforced `k8s-short-name`, controller-runtime v0.23 generic webhooks)
- [ ] No alpha/off fields that 1.34 silently drops (`restartPolicyRules`, `fileKeyRef`, `hostnameOverride`, image volumes, StatefulSet `maxUnavailable`, HPA `tolerance`) unless the gate is documented as enabled
- [ ] Cluster on a patched 1.34.x (≥ v1.34.4) and an upgrade plan beyond the 2026-10-27 EOL

##### CRD Design

- [ ] Kind CamelCase singular; plural lowercase; short names do not collide with built-ins (`rc`, `rs`, `ds`, `pv`, …)
- [ ] API group is a DNS subdomain you own (not `*.k8s.io`)
- [ ] `+kubebuilder:subresource:status` (and `scale` where relevant)
- [ ] `status` has `conditions []metav1.Condition` (`listType=map`, `listMapKey=type`) + `observedGeneration`
- [ ] Every string/list/map bounded (`MaxLength`, `MaxItems`, `MaxProperties`)
- [ ] CEL rules guard optional fields; transition rules understood (skip on create/absent); no `has(oldSelf)`
- [ ] CEL uses only libraries available on the lowest supported server (semver needs 1.34)
- [ ] No unrecognised `format` (no write warnings)
- [ ] Enum-like fields are string aliases with `Enum`; no floats; optional fields `omitempty`
- [ ] Print columns include readiness and `Age`; selectable fields for common filters

##### Controller

- [ ] Fetch → `IgnoreNotFound` → deletion/finalizer → children → status
- [ ] Errors returned (not swallowed); no `Requeue: true`; no `Result` together with an error
- [ ] Status written via `Status()` only when changed; `observedGeneration` set
- [ ] SSA with a stable `FieldOwner` and complete intent; no typed structs applied
- [ ] Owner references via `SetControllerReference`; finalizers only for external cleanup
- [ ] Watches restricted (namespaces/selectors, metadata-only where possible); map functions use indexes
- [ ] Timeouts on external calls (`ReconciliationTimeout` or contexts)
- [ ] Leader election with Leases; metrics served securely; readiness gated on cache sync
- [ ] Structured logging; Events on transitions only; bounded metric labels

##### Manifests & Security

- [ ] PSS `restricted` labels on application namespaces, `enforce-version` not pinned below v1.34 without reason
- [ ] No `host` in probes/lifecycle handlers
- [ ] `runAsNonRoot`, `seccompProfile: RuntimeDefault`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, read-only root FS
- [ ] AppArmor via fields, not annotations
- [ ] `automountServiceAccountToken: false` unless needed; least-privilege RBAC without wildcards
- [ ] Images pinned (digest or immutable tag), never `latest`
- [ ] Requests on every container (or pod-level resources on 1.34+), memory limits set; PDB for replicated workloads
- [ ] `trafficDistribution` value valid for every supported cluster version
- [ ] No `gitRepo` volumes, no `externalIPs`
- [ ] NetworkPolicies default-deny with explicit DNS egress

##### Admission

- [ ] Prefer CRD CEL / ValidatingAdmissionPolicy over webhooks; every policy has a binding
- [ ] No MutatingAdmissionPolicy in charts for default 1.34 clusters
- [ ] Webhooks: `sideEffects: None`, `matchPolicy: Equivalent`, short timeouts, HA + PDB, rotated certs, self-exclusion

##### Testing

- [ ] envtest with 1.34 binaries for CRD/CEL/SSA behaviour; fake client only for pure logic
- [ ] Reconcile idempotency, deletion path, failure status tested
- [ ] Rendered manifests validated with `--dry-run=server` against 1.34; generated files up to date
