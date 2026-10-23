### Kubernetes Style Guide

> **Scope**: Kubernetes manifests (raw YAML, Kustomize output, Helm-rendered YAML), CustomResourceDefinitions with CEL validation, admission policies and webhooks, RBAC and Pod security, and Go operators/controllers built on client-go, controller-runtime and Kubebuilder. This is the **version-agnostic** guide: use it when the target Kubernetes minor is unknown. Every rule here holds on all supported releases; whenever a rule depends on a feature that appeared in a specific minor, the minor is named so the reviewer can check it against the cluster first.
> **Release status (October 2026)**: the newest minor is **Kubernetes 1.37** (1.37.0 released 2026-08-26, latest patch **v1.37.1**). Supported minors are **1.37, 1.36, 1.35** and **1.34** (1.34 is in maintenance mode and reaches end of life on **2026-10-27**). 1.33 and older are out of upstream support. Kubernetes 1.38 is in development (latest pre-release `v1.38.0-alpha.2`) and is planned for 2026-12-16.[^1][^2]
> **Note**: This document is self-contained. Manifests in it were applied (or `--dry-run=server`-applied) to real kube-apiserver builds 1.29.5, 1.30.3, 1.31.0, 1.32.0, 1.33.0, 1.34.1, 1.35.0, 1.36.2 and 1.37.0; the Go controller compiles and passes its unit and envtest tests against controller-runtime v0.25.2 / k8s.io v0.37.1 (Go 1.26) and v0.23.3 / v0.35.9 (Go 1.25). Kubelet, scheduler and controller-manager *runtime* behaviour is taken from the official documentation and release notes, not executed.

---

#### Table of Contents

1. [Version-Aware Review](#version-aware-review)
2. [Release and Support Status](#release-and-support-status)
3. [Client Library Compatibility](#client-library-compatibility)
4. [Version-Tied Features at a Glance](#version-tied-features-at-a-glance)
5. [Do Not Use or Recommend](#do-not-use-or-recommend)
6. [Foundational Principles](#foundational-principles)
7. [Naming Conventions](#naming-conventions)
8. [YAML Pitfalls](#yaml-pitfalls)
9. [Workload Manifests](#workload-manifests)
10. [Pod Security](#pod-security)
11. [Services, Ingress, Gateway API and NetworkPolicy](#services-ingress-gateway-api-and-networkpolicy)
12. [Configuration and Secrets](#configuration-and-secrets)
13. [RBAC and Least Privilege](#rbac-and-least-privilege)
14. [CRD and API Design](#crd-and-api-design)
15. [CEL Validation Rules](#cel-validation-rules)
16. [Spec, Status and Conditions](#spec-status-and-conditions)
17. [API Versioning, Conversion and Storage Migration](#api-versioning-conversion-and-storage-migration)
18. [Admission Control](#admission-control)
19. [Controller Patterns](#controller-patterns)
20. [Finalizers, Owner References and Garbage Collection](#finalizers-owner-references-and-garbage-collection)
21. [Errors, Requeue and Back-Pressure](#errors-requeue-and-back-pressure)
22. [Caches, Informers and Leader Election](#caches-informers-and-leader-election)
23. [Observability](#observability)
24. [Testing](#testing)
25. [Helm and Kustomize](#helm-and-kustomize)
26. [Production / Review Checklist](#production--review-checklist)

---

#### Version-Aware Review

The single most common reviewer mistake with Kubernetes is recommending a field, API version or library function that the target cluster does not have. Before suggesting anything newer than Kubernetes 1.29, establish the target.

##### Determine the target version first

Look for, in this order:

- `k8s.io/api` / `k8s.io/client-go` in `go.mod`: library `v0.N.x` pairs with Kubernetes `1.N` (see [Client Library Compatibility](#client-library-compatibility)).
- `ENVTEST_K8S_VERSION` in a Makefile, `setup-envtest use 1.N`, `kindest/node:v1.N.x` images, `KUBERNETES_VERSION` in CI.
- `kubeVersion:` in a Helm `Chart.yaml`, `.Capabilities.KubeVersion` branches in templates.
- Managed-cluster versions in IaC (Terraform `kubernetes_version`, `cluster_version`, eksctl/AKS/GKE config), kubeadm `ClusterConfiguration.kubernetesVersion`.
- Pod Security `enforce-version` labels, `kubectl` pins in `.tool-versions`/`mise.toml`.

If nothing pins a version, assume the oldest **supported** minor (1.34 in October 2026) for "is this available" questions, and say explicitly that the recommendation depends on the cluster version.

##### Why version mistakes are dangerous: fields are dropped silently

A newer field does not always fail loudly on an older API server. There are two different failure modes, both verified:

| Situation on the older server | `kubectl apply` (strict field validation, the default) | Clients without strict validation (`kubectl --validate=false`, most Go clients, many CD tools) |
|---|---|---|
| Field **unknown** to that minor's schema | Rejected: `strict decoding error: unknown field "spec.containers[0].restartPolicyRules"` (1.33) | Field dropped, object stored without it; the server only returns a `Warning: unknown field …` header (its default `fieldValidation` is `Warn`; verified with a raw POST on 1.37: `201` plus `warning: 299 - "unknown field \"bogus\""`), which most Go clients merely log |
| Field **known but feature-gated off** (alpha, or beta-off) | **Silently dropped**, no warning | Silently dropped |

Verified examples of the second, most dangerous case:

- `spec.hostUsers: false` on 1.30–1.32 (user namespaces beta but off): the Pod is stored **without** it and would run in the host user namespace — a security degradation, not a validation error.
- `volumes[].image` on 1.34 (image volumes beta-off): the whole volume **and every `volumeMount` that references it** disappear; the container starts without its data.
- `restartPolicyRules` on 1.34, `trafficDistribution` on 1.30, CRD `selectableFields` on 1.30, StatefulSet `rollingUpdate.maxUnavailable` on 1.34 and 1.36, `emptyDir.mode` on 1.37 (alpha): all dropped without a warning.

Therefore:

- **Never** tell an author that a version-tied field "works" without naming the minor where it is beta-on/GA and asking them to confirm the cluster version.
- `kubectl explain` proves a field **exists**, not that it is **enabled**. Only `kubectl apply --dry-run=server -o yaml` against the real cluster (or an envtest/kind of the same minor and feature gates) proves a field survives.
- New **beta APIs** (new group versions) are **off by default since 1.24** and need `--runtime-config` plus usually a feature gate.[^6] A manifest that uses any `…/v1beta*` or `…/v1alpha*` version of a built-in group needs a documented cluster prerequisite.
- Managed providers may disable beta features; "beta-on" means "on in upstream defaults".

##### Say "not available" only with evidence

Say "not available on 1.N" only when the feature table below, the release notes or a dry-run against 1.N proves it. Otherwise say "requires Kubernetes ≥ 1.M (beta-on) / 1.K (GA); confirm the cluster version". Do not flag a feature as unavailable just because it is new to you.

##### Version skew

- kubectl is supported within **±1 minor** of the API server.
- kubelets may be up to **three minors older** than the API server (kube-apiserver 1.37 with 1.34 nodes).[^3] A field the API server accepts (sidecars, user namespaces, resize, image volumes, restart rules) only works on nodes new enough to implement it.
- Client libraries are forward- and backward-compatible on the wire for GA APIs, but a controller built against `k8s.io/api` v0.37 can construct fields an older server drops.

---

#### Release and Support Status

As of October 2026 (data from kubernetes.io release schedule and `dl.k8s.io/release/stable-1.N.txt`).[^1][^2]

| Minor | Released | Latest / final patch | Maintenance mode | End of life | Status (Oct 2026) |
|---|---|---|---|---|---|
| **1.37** | 2026-08-26 | v1.37.1 (published 2026-09-23) | 2027-08-28 | 2027-10-28 | supported, newest |
| **1.36** | 2026-04-22 | v1.36.5 (published 2026-09-23) | 2027-04-28 | 2027-06-28 | supported |
| **1.35** | 2025-12-17 | v1.35.9 (published 2026-09-23) | 2026-12-28 | 2027-02-28 | supported |
| **1.34** | 2025-08-27 | v1.34.12 (published 2026-09-23) | since 2026-08-27 | **2026-10-27** | maintenance mode, EOL imminent |
| 1.33 | 2025-04-23 | v1.33.13 (final) | — | 2026-06-28 | out of support |
| 1.32 | 2024-12-11 | v1.32.13 (final, 2026-02-26) | — | 2026-02-28 | out of support |
| 1.31 | 2024-08-13 | v1.31.14 (final) | — | 2025-11-11 | out of support |
| 1.30 | 2024-04-17 | v1.30.14 (final) | — | 2025-07-15 | out of support |
| 1.29 | 2023-12-13 | v1.29.15 (Go-only rebuild after EOL; the release data lists v1.29.14 as final) | — | 2025-02-28 | out of support |

- The October 2026 patch round (v1.37.2, v1.36.6, v1.35.10, v1.34.13, scheduled for 2026-10-13) had not been published as of 2026-10-22; `dl.k8s.io` still reports the September patches.
- Kubernetes **1.38** exists only as alpha pre-releases (`v1.38.0-alpha.2`); never recommend 1.38 features.
- Clusters cannot skip minors on upgrade (1.32 → 1.33 → 1.34 …). A cluster on an out-of-support minor is itself a finding; managed "extended support" does not change upstream status.

---

#### Client Library Compatibility

The `k8s.io/*` staging modules use `v0.N.x` for Kubernetes `1.N`. controller-runtime, controller-tools and Kubebuilder each target one Kubernetes minor per release line.[^23][^24][^25]

| Kubernetes | `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` | Go directive of those modules | controller-runtime | controller-tools (`controller-gen`) | Kubebuilder scaffolds it from |
|---|---|---|---|---|---|
| 1.37 | v0.37.x (v0.37.1) | `go 1.26.0` | v0.25.x (v0.25.2) | v0.22.0, v0.23.0 | v4.16 |
| 1.36 | v0.36.x (v0.36.5) | `go 1.26.0` | v0.24.x (v0.24.1) | v0.21.x | v4.15 |
| 1.35 | v0.35.x (v0.35.9) | `go 1.25.0` | v0.23.x (v0.23.3) | v0.20.x | v4.11–v4.14 |
| 1.34 | v0.34.x (v0.34.12) | `go 1.25.0` | v0.22.x (v0.22.5) | v0.19.x | v4.9–v4.10 |
| 1.33 | v0.33.x (v0.33.13) | `go 1.24.0` | v0.21.0 | v0.18.0 | v4.6–v4.8 |
| 1.32 | v0.32.x (v0.32.13) | `go 1.23.0` | v0.20.x (v0.20.4) | v0.17.x | v4.5 |
| 1.31 | v0.31.x (v0.31.14) | `go 1.22.0` | v0.19.x (v0.19.7) | v0.16.x | v4.2–v4.4 |
| 1.30 | v0.30.x (v0.30.14) | `go 1.22.0` | v0.18.x (v0.18.7) | v0.15.0 | v4.0–v4.1 |
| 1.29 | v0.29.x (v0.29.15) | `go 1.21` | v0.17.x (v0.17.6) | v0.14.0 | v3.14–v3.15 |

Review rules:

- **All `k8s.io/*` staging modules on the same minor** (ideally the same patch). `k8s.io/api v0.37.1` with `k8s.io/client-go v0.36.5` is a finding.
- **controller-runtime must match the `k8s.io/*` minor** of the table; mixing (e.g. controller-runtime v0.24 with `k8s.io/*` v0.37) produces compile errors or subtle scheme mismatches.
- `controller-gen` output (`zz_generated.deepcopy.go`, `config/crd/bases/*.yaml`, `config/rbac/role.yaml`) must be regenerated in the same change as the Go types or markers they derive from.
- `go.mod` must satisfy the libraries' `go` directive. With an older local Go, `GOTOOLCHAIN=auto` (the default) silently downloads a newer toolchain and `GOTOOLCHAIN=local` fails (`go.mod requires go >= 1.25.0 (running go 1.24.7; GOTOOLCHAIN=local)`) — CI images and Dockerfiles must use a matching Go.
- Do not import `k8s.io/kubernetes` as a library; it is not meant to be consumed.
- Flag leftover `replace k8s.io/... =>` pins from old upgrades.
- Pin envtest to the minor you ship to; `setup-envtest` picks the newest version when unpinned, which accepts fields an older cluster drops.

controller-runtime API boundaries that reviewers meet often (verified in the module sources):

| API | Available since | Notes |
|---|---|---|
| `client.Client.Apply(ctx, applyConfiguration, …)` (typed server-side apply) | v0.22 | the old `Patch(ctx, obj, client.Apply)` form is deprecated |
| `controller.Options.ReconciliationTimeout` | v0.22 | deadline on each `Reconcile` context |
| `mgr.GetEventRecorder(name)` (`events.k8s.io/v1`) | v0.23 | `GetEventRecorderFor` (core v1 Events) is deprecated; v0.22 has no `GetEventRecorder` (verified compile failure) |
| Generic webhooks `admission.Validator[T]` / `Defaulter[T]`, `ctrl.NewWebhookManagedBy(mgr, &T{})` | v0.23 | `CustomValidator`/`CustomDefaulter` deprecated |
| Priority queue on by default (`UsePriorityQueue`) | v0.23 | v0.20–v0.22: opt-in beta |
| `reconcile.Result.Requeue` deprecated ("Use `RequeueAfter` instead") | v0.21 | |
| `reconcile.TerminalError(err)` | v0.15 | error that is logged and counted but not retried |
| Fake client `WithStatusSubresource` required for status writes | v0.15 | |
| Fake client supports `Apply` | v0.22 | |
| `cache.TransformStripManagedFields()` | v0.18 | |
| Experimental `client.CacheOptions.EnableReadYourWritesConsistency` | v0.25 | opt-in |

---

#### Version-Tied Features at a Glance

"Beta-on" = beta and enabled by default (usable, but an administrator can disable it); "GA" = stable, gate locked on or removed. Boundaries marked ✔ were reproduced on the API server builds listed in the header; the rest come from release notes and KEPs.[^5][^28] **Always check the target cluster minor before recommending anything in this table.**

##### Pods and workloads

| Feature | Manifest surface | Beta-on | GA | Older servers |
|---|---|---|---|---|
| Native sidecar containers | `initContainers[].restartPolicy: Always` | 1.29 | 1.33 | ✔ kept on 1.29 |
| Lifecycle `sleep` action | `lifecycle.preStop.sleep.seconds` | 1.30 | 1.34 | ✔ 1.29 drops it, then rejects the hook: `preStop: Required value: must specify a handler type` |
| `sleep.seconds: 0` | — | 1.33 | 1.34 | ✔ 1.32 rejects: `must be greater than 0` |
| AppArmor fields (replace annotations) | `securityContext.appArmorProfile` | 1.30 | 1.31 | ✔ 1.29: unknown field |
| In-place Pod resize | `resizePolicy`, `pods/resize` subresource | 1.33 | 1.35 | ✔ no `pods/resize` on 1.32 |
| User namespaces | `spec.hostUsers: false` | 1.33 (beta-**off** 1.30–1.32) | 1.36 | ✔ silently dropped on 1.30–1.32 |
| Fine-grained supplemental groups | `securityContext.supplementalGroupsPolicy` | 1.33 | 1.35 | ✔ dropped on 1.31–1.32, unknown on 1.30 |
| Pod-level resources | `spec.resources` | 1.34 | — (beta) | ✔ dropped on 1.33, unknown on ≤ 1.31 |
| Image volumes | `volumes[].image` | 1.35 (beta-**off** 1.33–1.34) | 1.36 | ✔ volume **and mounts** dropped on 1.34 |
| Container restart rules | container `restartPolicy` + `restartPolicyRules` | 1.35 | — (beta) | ✔ dropped on 1.34, unknown on 1.33 |
| Env vars from files | `env[].valueFrom.fileKeyRef` | 1.35 | 1.38 (planned) | 1.34 rejects the field |
| Relaxed env var names (any printable ASCII except `=`) | `env[].name` | 1.32 | 1.34 | 1.31 rejects |
| Pod scheduling gates | `spec.schedulingGates` | 1.27 | 1.30 | |
| Topology spread `minDomains` | `topologySpreadConstraints[].minDomains` | 1.27 | 1.30 | |
| Topology spread `matchLabelKeys` | `topologySpreadConstraints[].matchLabelKeys` | 1.27 | — (still beta in 1.37) | |
| `matchLabelKeys` in pod (anti-)affinity | `podAffinity…matchLabelKeys` | 1.31 | 1.33 | |
| Recursive read-only mounts | `volumeMounts[].recursiveReadOnly` | 1.31 | 1.33 | needs runtime support |
| `ReadWriteOncePod` access mode | PVC `accessModes` | — | 1.29 | |
| PDB unhealthy-pod eviction policy | `spec.unhealthyPodEvictionPolicy` | 1.27 | 1.31 | |
| Job pod failure policy | `spec.podFailurePolicy` | 1.26 | 1.31 | |
| Job backoff limit per index | `spec.backoffLimitPerIndex` | 1.29 | 1.33 | |
| Job success policy | `spec.successPolicy` | 1.31 | 1.33 | |
| Job pod replacement policy | `spec.podReplacementPolicy` | 1.29 | 1.34 | |
| Job `managedBy` | `spec.managedBy` | 1.32 | 1.35 | |
| StatefulSet PVC retention | `spec.persistentVolumeClaimRetentionPolicy` | 1.27 | 1.32 | |
| StatefulSet `maxUnavailable` | `updateStrategy.rollingUpdate.maxUnavailable` | 1.35.0–1.35.3, again 1.37 (**off** in 1.35.4+ and 1.36) | — | ✔ dropped on 1.34 and 1.36, kept on 1.35.0 and 1.37 |
| HPA container resource metrics | `type: ContainerResource` | 1.27 | 1.30 | |
| HPA per-direction tolerance | `behavior.scaleUp/scaleDown.tolerance` | 1.35 | 1.37 | |
| HPA scale to zero | `minReplicas: 0` + Object/External metric | 1.37 | — | ✔ 1.36: `must be greater than or equal to 1` |

##### Services, networking, API machinery

| Feature | Surface | Beta-on | GA | Older servers |
|---|---|---|---|---|
| `trafficDistribution: PreferClose` | Service `spec.trafficDistribution` | 1.31 | 1.33 | ✔ dropped on 1.30 |
| `PreferSameZone` / `PreferSameNode` (`PreferClose` deprecated, warns from 1.35) | same | 1.34 | 1.35 | ✔ 1.33: `supported values: "PreferClose"` |
| Multiple Service CIDRs | `networking.k8s.io/v1` `ServiceCIDR`, `IPAddress` | beta-off 1.31–1.32 | 1.33 | ✔ v1 served from 1.33 |
| Service names may start with a digit | `metadata.name` of Services | 1.36 | 1.37 | ✔ 1.35: `a DNS-1035 label must … start with an alphabetic character` |
| Strict IP/CIDR validation | every IP/CIDR field | 1.36 | 1.38 (planned) | ✔ 1.33–1.35 warn (`CIDR value "10.0.0.1/8" is ambiguous`); ≤ 1.32 accept silently |
| Server-side field validation (strict by default in kubectl) | `fieldValidation` | — | 1.27 | |
| DRA core | `resource.k8s.io/v1` | beta-off ≤ 1.33 | 1.34 | ✔ `deviceclasses` v1 from 1.34 |
| VolumeAttributesClass | `storage.k8s.io/v1` | beta-off | 1.34 | ✔ |
| StorageVersionMigration | `storagemigration.k8s.io/v1` | beta-off 1.35–1.36 | 1.37 | ✔ served only on 1.37 |
| Pod certificates, ClusterTrustBundle | `certificates.k8s.io/v1` | beta-off 1.35–1.36 | 1.37 | ✔ |
| WebSocket exec/attach/port-forward need `create` | RBAC on `pods/exec` etc. | 1.35 | — | ✔ 1.34 authorizes a WebSocket exec with `get` only |
| Fine-grained kubelet authorization | `nodes/pods`, `nodes/healthz`, `nodes/configz` | 1.33 | 1.36 | |
| KYAML output | `kubectl get -o kyaml` | kubectl 1.35 | kubectl 1.37 | |

##### CRDs and admission

| Feature | Surface | Beta-on | GA | Older servers |
|---|---|---|---|---|
| CRD validation rules (CEL) | `x-kubernetes-validations` | 1.25 | 1.29 | |
| CRD validation ratcheting (incl. `optionalOldSelf`) | — | 1.30 | 1.33 | |
| CRD field selectors | `versions[].selectableFields` | 1.31 | 1.32 | ✔ dropped on 1.30, unknown on 1.29 |
| CRD numeric `format` range checks (`int32`, `int64`, `float`, `double`) | schema `format` | — | 1.36 (behaviour change) | 1.35 stores out-of-range values |
| ValidatingAdmissionPolicy | `admissionregistration.k8s.io/v1` | beta-off 1.28–1.29 | 1.30 | ✔ not served on 1.29 |
| Webhook `matchConditions` | `webhooks[].matchConditions` | 1.28 | 1.30 | |
| Strict CEL cost enforcement for VAP/webhooks | — | — | 1.32 | |
| MutatingAdmissionPolicy | `admissionregistration.k8s.io/v1` | beta-off 1.34–1.35 (`v1beta1`) | 1.36 | ✔ v1 not served on 1.35 |
| Manifest-based (static) admission config | `staticManifestsDir` | 1.37 | — | |

##### CEL library availability

A CRD or policy that uses a newer library is **rejected at write time** by an older API server — it does not degrade gracefully.[^7]

| Library / feature | Available since | Verified boundary |
|---|---|---|
| Standard macros, list, regex, URL, authorizer libraries | all versions with CEL | |
| Quantity library (`isQuantity`, `quantity()`), CEL optional types (`?.`, `orValue`), cross-type numeric comparisons | 1.29 | ✔ quantity accepted on 1.30 |
| Extended strings library v2 (incl. `'…'.format([…])`) | 1.30 | ✔ `format()` rejected on 1.29 |
| IP and CIDR libraries (`isIP`, `ip()`, `isCIDR`, `cidr()`) | 1.31 | ✔ rejected on 1.30 |
| Format library (`format.dns1123Label()` …) | 1.32 | ✔ rejected on 1.31 |
| Two-variable comprehensions (`all(k, v, …)`, `transformMap`) | 1.33 | |
| Semver library (`isSemver`, `semver()`) | 1.34 | ✔ rejected on 1.33 |

---

#### Do Not Use or Recommend

##### Never recommend (no supported release has them)

- Anything only in Kubernetes **1.38** pre-releases, e.g. ML-DSA keys for Pod certificates/CSRs, the managed-fields opt-out (`ManagedFieldsOptOut`), `staticOrder` for static mutating webhooks, apimachinery v0.38 `Quantity` helpers.
- **Alpha** features on clusters that do not explicitly enable the gate — in 1.37 for example `emptyDir.mode`, probe `httpGet.protocol: HTTP2`, StatefulSet `updateStrategy.type: Recreate`, Pod `evictionResponders`, container `lifecycle.stopSignal`, Job `spec.scheduling`. They are dropped or rejected.
- `scheduling.k8s.io` Workload/PodGroup gang scheduling (alpha or beta-off everywhere).

##### Removed API versions — flag every occurrence

These are not served by any supported release and fail with `no matches for kind`:[^4]

| Removed | In | Use |
|---|---|---|
| `extensions/v1beta1`, `networking.k8s.io/v1beta1` Ingress | 1.22 | `networking.k8s.io/v1` |
| `apiextensions.k8s.io/v1beta1` CRDs, `admissionregistration.k8s.io/v1beta1` webhooks, `rbac.authorization.k8s.io/v1beta1`, `certificates.k8s.io/v1beta1` CSR, `coordination.k8s.io/v1beta1` | 1.22 | `…/v1` |
| `batch/v1beta1` CronJob, `policy/v1beta1` PodDisruptionBudget, `discovery.k8s.io/v1beta1`, `events.k8s.io/v1beta1`, `autoscaling/v2beta1`, `node.k8s.io/v1beta1` | 1.25 | `batch/v1`, `policy/v1`, `discovery.k8s.io/v1`, `events.k8s.io/v1`, `autoscaling/v2`, `node.k8s.io/v1` |
| **PodSecurityPolicy** (`policy/v1beta1`) | 1.25 | Pod Security Admission, ValidatingAdmissionPolicy |
| `autoscaling/v2beta2` | 1.26 | `autoscaling/v2` |
| `storage.k8s.io/v1beta1` CSIStorageCapacity | 1.27 | `storage.k8s.io/v1` |
| `flowcontrol.apiserver.k8s.io/v1beta2` / `v1beta3` | 1.29 / 1.32 | `flowcontrol.apiserver.k8s.io/v1` |

##### Deprecated — flag in new code

- `v1` **Endpoints** (deprecated since 1.33; every access warns) — read `discovery.k8s.io/v1` **EndpointSlices** (label `kubernetes.io/service-name`; there can be several per Service).
- AppArmor annotations `container.apparmor.security.beta.kubernetes.io/*` — use `securityContext.appArmorProfile` (1.30+).
- `trafficDistribution: PreferClose` — `PreferSameZone` on 1.34+ clusters.
- Service `spec.externalIPs` — deprecated in 1.36 (removal planned for 1.43); allows traffic interception (CVE-2020-8554).
- `gitRepo` volumes — deprecated since 1.11; the plugin can no longer be enabled from 1.36. Clone in an init container instead.
- `kubernetes.io/ingress.class` annotation — use `spec.ingressClassName`.
- kube-proxy `ipvs` mode (deprecated since 1.35) — `nftables` (GA 1.33) or `iptables`.
- The community **Ingress NGINX** controller was retired on 2026-03-24 and receives no fixes; new routing should use Gateway API or a maintained controller.[^22]
- Long-lived `kubernetes.io/service-account-token` Secrets — use projected, audience-bound tokens.
- controller-runtime: `Result.Requeue`, `GetEventRecorderFor`, `CustomValidator`/`CustomDefaulter`, `Patch(…, client.Apply)`; client-go: untyped workqueues (`workqueue.New`, `NewRateLimitingQueue`, `DefaultControllerRateLimiter`), `fake.NewSimpleClientset` (no field management — use `fake.NewClientset`), `k8s.io/utils/pointer` (use `k8s.io/utils/ptr`).

---

#### Foundational Principles

| Principle | Description |
|---|---|
| **Level-based reconciliation** | React to current state, never to specific event types. The reconcile function must not receive or depend on event type information. |
| **Idempotency** | Every reconcile call must be safe to run repeatedly with the same outcome. |
| **Single responsibility** | One operator per application. One controller per root Kind. |
| **Declarative intent** | `spec` expresses *desired state*; the controller drives *actual state* toward it. |
| **Avoid edge-triggering** | Never build logic that depends on "what changed". Always reconcile the full object state. |
| **Open-world assumption** | The system may miss events; design controllers that recover fully from a cold start. |
| **Caches are eventually consistent** | Informer caches lag behind the controller's own writes. Never conclude "does not exist" from a cache miss right after a create; use deterministic names, server-side apply and `AlreadyExists`/`Conflict` handling. |
| **The API server pushes back** | Expect 409 Conflict, 429 Too Many Requests (API Priority and Fairness, watch-cache initialisation) and 5xx; retry with backoff and honour `Retry-After`. |
| **Declarative policy first** | CRD schema + CEL, then ValidatingAdmissionPolicy/MutatingAdmissionPolicy, then webhooks. In-process policies cannot be taken down by a crashing webhook Pod. |

---

#### Naming Conventions

##### Character rules by name class

| Name class | Used for | Allowed characters | Start/end | Max length |
|---|---|---|---|---|
| **DNS subdomain** (RFC 1123) | most `metadata.name` values (Pods, Deployments, ConfigMaps, CRs), API groups, CRD `spec.group` | lowercase `a–z`, digits, `-`, `.` | alphanumeric at both ends | 253 |
| **DNS label** (RFC 1123) | `metadata.namespace`, container and port names (ports ≤ 15), **Service names on 1.36+** | lowercase `a–z`, digits, `-` | alphanumeric at both ends | 63 |
| **DNS label** (RFC 1035) | **Service names on ≤ 1.35** | lowercase `a–z`, digits, `-` | must start with `[a-z]` | 63 |
| **Qualified name** | label keys, annotation keys, finalizers | name part `A–Z a–z 0–9 - _ .`; optional DNS-subdomain prefix + `/` | name part alphanumeric at both ends | name ≤ 63, prefix ≤ 253 |
| **Label value** | `metadata.labels` values | `A–Z a–z 0–9 - _ .` or empty | alphanumeric at both ends if non-empty | 63 |
| **Annotation value** | annotation values | any string | — | all annotations of an object together ≤ 256 KiB |
| **Env var name** | `env[].name` | C identifier on ≤ 1.31; any printable ASCII except `=` on 1.34+ (beta-on 1.32) | — | — |
| **ConfigMap / Secret key** | `data` / `stringData` keys | `A–Z a–z 0–9 - _ .` | — | 253 |

> ⚠️ `_` and uppercase letters are **never** valid in DNS subdomain or DNS label names (`metadata.name`, namespaces, API groups, Service names).

Service names starting with a digit (`1st-api`) are accepted from 1.36 and rejected on 1.35 and older (verified). Flag them only when the manifest must also install on ≤ 1.35. Env var names that are not C identifiers are invisible to POSIX shells even where the API accepts them.

##### API group, Kind, resource, short names

| Element | Rule | Example |
|---|---|---|
| **API group** | lowercase DNS subdomain you own | `cache.example.com` |
| **Kind** | CamelCase, singular | `RedisCluster` |
| **Resource (plural)** | lowercase, plural | `redisclusters` |
| **Short name** | 2–5 lowercase chars, **must not collide** with built-in short names | `rdc` |
| **Category** | lowercase; `all` only if `kubectl get all` should list it | `cache` |

> ⚠️ Do not use the empty group, single-word groups, `*.k8s.io` or `*.kubernetes.io` — those are reserved (using them requires the `api-approved.kubernetes.io` annotation).

Built-in short names on a 1.37 API server (`kubectl api-resources`): `cj cm crd crds cs csr deploy ds ep ev hpa ing ip limits netpol no ns pc pdb po pv pvc quota rc rs sa sc sts svc vac`; add-ons and CRDs add more (Gateway API alone adds `gc`, `gtw`, `lset`, `refgrant`, `btlspolicy`).

```yaml
# BAD: "rc" is ReplicationController's short name
names: {kind: RedisCluster, plural: redisclusters, shortNames: [rc]}
```

With that CRD installed, `kubectl get rc demo` prints `Warning: short name "rc" could also match lower priority resource redisclusters.cache2.example.com` and then looks up a **ReplicationController** (`replicationcontrollers "demo" not found`) — built-in short names always win (verified on 1.37). Use an unused name such as `rdc` and check `kubectl api-resources` on the target cluster, including installed CRDs.

##### Go types and JSON fields

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

| Rule | ✅ Good | ❌ Bad |
|---|---|---|
| camelCase JSON | `replicaCount` | `replica_count`, `ReplicaCount` |
| Declarative nouns | `replicas` | `setReplicas` |
| Same-namespace name references | `secretName` | `secret` |
| Object references (kind/namespace may vary) | `secretRef: {name, namespace}` | flat strings |
| Durations | `timeoutSeconds` (`int32`) or `metav1.Duration` | ad-hoc `timeout: "30s"` parsed by hand |
| Established prefixes and abbreviations | `maxReplicas`, `minReadySeconds`, `maxSurge`, `url`, `ip`, `id` | `tgtUtilization`, `cfgMapName` |

Enums are string type aliases with CamelCase (or consistently lowercase) values, never Go `iota` integers:

```go
// +kubebuilder:validation:Enum=small;medium;large
type Tier string

const (
	TierSmall  Tier = "small"
	TierMedium Tier = "medium"
	TierLarge  Tier = "large"
)
```

##### Labels and annotations

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

- Use the recommended `app.kubernetes.io/*` labels;[^32] put your own keys under a domain you own.
- Labels are for selection (bounded, indexed); annotations are for tool metadata. Do not put large or high-cardinality data in labels.
- When code uses `client.ObjectKey` / `types.NamespacedName`, assume `Namespace` and `Name` already satisfy the DNS rules above unless the value comes from user input.

---

#### YAML Pitfalls

`kubectl`, Helm and Kustomize parse YAML with YAML 1.1 rules (via `sigs.k8s.io/yaml`). Unquoted `yes`, `no`, `on`, `off`, `y`, `n`, `true`, `false` become **booleans**, `1.10` becomes the number `1.1`, `0700` is octal. String fields then reject the value — or worse, a numeric field silently gets a different value. Verified on 1.37:

```yaml
# BAD
apiVersion: v1
kind: ConfigMap
metadata: {name: flags, namespace: shop}
data:
  enabled: yes        # "json: cannot unmarshal bool into Go struct field ConfigMap.data of type string"
  version: 1.10       # "json: cannot unmarshal number into Go struct field ConfigMap.data of type string"
```

Even object names are affected: an HPA written as `metadata: {name: n}` fails with `json: cannot unmarshal bool into Go struct field ObjectMeta.metadata.name of type string` (verified).

```yaml
# GOOD: quote every scalar that must stay a string
apiVersion: v1
kind: ConfigMap
metadata: {name: flags, namespace: shop}
data:
  enabled: "yes"
  version: "1.10"
```

- The same applies to label and annotation values (`"true"`, `"1"`), `env[].value`, and Helm values that end up in those fields (`{{ .Values.flag | quote }}`).
- File modes (`defaultMode: 0644`) are intentionally octal in YAML; in JSON they must be decimal (`420`).
- `kubectl get -o kyaml` (beta in kubectl 1.35, GA in 1.37) emits an always-quoted, unambiguous YAML subset that any older kubectl can read; it is a good format for generated manifests.

---

#### Workload Manifests

##### A production Deployment

Applied on 1.34 and 1.37 into a namespace enforcing Pod Security `restricted`; a Pod built from the template passes admission. On 1.29 it is rejected only because of `preStop.sleep` (beta-on from 1.30).

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: shop
  labels: {app.kubernetes.io/name: web, app.kubernetes.io/part-of: shop}
spec:
  replicas: 3                       # omit when an HPA owns the replica count
  revisionHistoryLimit: 5
  selector:
    matchLabels: {app.kubernetes.io/name: web}   # immutable after creation
  strategy:
    type: RollingUpdate
    rollingUpdate: {maxSurge: 25%, maxUnavailable: 0}
  template:
    metadata:
      labels: {app.kubernetes.io/name: web, app.kubernetes.io/part-of: shop}
    spec:
      serviceAccountName: web
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
            matchLabels: {app.kubernetes.io/name: web}
          matchLabelKeys: [pod-template-hash]   # spread each rollout revision on its own
      initContainers:
        - name: log-shipper                     # native sidecar: beta-on 1.29, GA 1.33
          image: registry.example.com/shipper@sha256:0000000000000000000000000000000000000000000000000000000000000000
          restartPolicy: Always
          resources:
            requests: {cpu: 20m, memory: 32Mi}
            limits: {memory: 64Mi}
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: {drop: [ALL]}
      containers:
        - name: web
          image: registry.example.com/web@sha256:0000000000000000000000000000000000000000000000000000000000000000
          ports: [{name: http, containerPort: 8080}]
          resources:
            requests: {cpu: 250m, memory: 256Mi}
            limits: {memory: 256Mi}
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
            timeoutSeconds: 2
            failureThreshold: 3
          lifecycle:
            preStop:
              sleep: {seconds: 10}              # beta-on 1.30, GA 1.34; no shell needed
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

What a reviewer checks in every workload:

- **Images** pinned by digest (or at least an immutable tag), never `:latest` (which also defaults `imagePullPolicy` to `Always`). Sidecars and init containers too.
- **Resources**: every container has CPU and memory **requests**; memory **limit** set (usually equal to the request for predictable OOM behaviour). CPU limits are a policy decision — they throttle latency-sensitive code; flag missing memory limits, not missing CPU limits. A container without requests makes the Pod `BestEffort` (first to be evicted) and breaks HPA utilisation targets.
- **Selector** (`spec.selector`) is immutable (verified: `spec.selector: Invalid value: …: field is immutable`) and must match the template labels. Changing it means a new object.
- **No `spec.replicas`** in an applied manifest when an HPA manages the workload — every `kubectl apply`/GitOps sync resets it.
- **`maxUnavailable: 0` + `maxSurge > 0`** for zero-downtime rollouts of stateless services; `progressDeadlineSeconds` only marks `Progressing=False`, it never rolls back.
- **Topology spread**[^35] (or anti-affinity) across zones for replicated services; `whenUnsatisfiable: ScheduleAnyway` unless a hard guarantee is worth unschedulable Pods.
- **`automountServiceAccountToken: false`** unless the container calls the API server.
- **Writable paths** come from `emptyDir` (with `sizeLimit`) when `readOnlyRootFilesystem: true`.

##### Probes, lifecycle and graceful termination

- **Liveness must not depend on downstream services**[^33] (database, other APIs) — an outage there restarts every replica. Readiness may.
- Use a **startupProbe** for slow starts instead of large `initialDelaySeconds` on liveness; liveness and readiness start only after it succeeds.
- `timeoutSeconds` defaults to 1 s; slow health handlers under load cause restart storms.
- Termination sequence: the Pod is removed from EndpointSlices *and* receives SIGTERM concurrently. A short `preStop` delay lets load balancers and kube-proxy stop sending traffic first. `lifecycle.preStop.sleep` (beta-on 1.30, GA 1.34; `seconds: 0` allowed from 1.33/1.34) replaces `exec: ["sleep", "10"]`, which fails in distroless images. On older clusters keep the `exec` form only where the image has a shell.
- `terminationGracePeriodSeconds` must exceed the preStop delay plus the application's shutdown time; the app must handle SIGTERM (stop accepting, drain, exit).
- HTTP probes on named ports (`port: http`) survive port renumbering.

##### Native sidecar containers (beta-on 1.29, GA 1.33)

A sidecar is an **init container with `restartPolicy: Always`**. It starts before later init containers and the app containers (after its `startupProbe` succeeds, if any), keeps running, restarts independently, is stopped **after** the app containers and does not block Job completion.[^15]

```yaml
# BAD: "sidecar" as a regular container: no start ordering, a Job Pod never completes,
# and the proxy/log shipper is killed together with the app (lost logs on shutdown)
spec:
  containers:
    - name: app
      image: registry.example.com/batch:1.0
    - name: log-shipper
      image: registry.example.com/shipper:2.3
```

```yaml
# GOOD (1.29+): init container with restartPolicy: Always
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

Sidecar resource requests are **added** to the app containers' requests (classic init containers only count as a maximum). Do not confuse this with per-container `restartPolicy` on *regular* containers, which is the separate container-restart-rules feature (beta-on 1.35).

##### In-place resize (beta-on 1.33, GA 1.35)

CPU and memory of a running Pod change only through the **`resize` subresource**; a normal Pod update that changes resources is rejected.[^16]

```bash
# GOOD (1.33+): strategic-merge patch against the resize subresource
kubectl patch pod web-0 --subresource resize \
  -p '{"spec":{"containers":[{"name":"web","resources":{"requests":{"cpu":"500m"},"limits":{"memory":"512Mi"}}}]}}'
```

- Only `cpu` and `memory`; the QoS class may not change; existing requests/limits cannot be removed. A `--type merge` patch replaces the whole `containers` list and therefore "removes" resources — use strategic merge.
- Progress is reported via the Pod conditions `PodResizePending` (`Deferred`/`Infeasible`) and `PodResizeInProgress`; `status.resize` is deprecated. From 1.35 Pods carry `metadata.generation`/`status.observedGeneration` (GA), so wait for `observedGeneration >= generation`.
- RBAC: `patch`/`update` on `pods/resize` — not `update` on `pods`.
- Changing resources in a Deployment/StatefulSet **template** still rolls the workload; in-place resize is for VPA-like controllers acting on Pods.

##### StatefulSets

```yaml
apiVersion: v1
kind: Service
metadata: {name: db, namespace: shop}
spec:
  clusterIP: None                       # headless Service for stable Pod DNS
  selector: {app.kubernetes.io/name: db}
  ports: [{name: pg, port: 5432}]
---
apiVersion: apps/v1
kind: StatefulSet
metadata: {name: db, namespace: shop}
spec:
  serviceName: db
  replicas: 3
  selector: {matchLabels: {app.kubernetes.io/name: db}}
  updateStrategy:
    type: RollingUpdate
  persistentVolumeClaimRetentionPolicy:   # GA 1.32
    whenDeleted: Retain
    whenScaled: Retain
  template:
    metadata: {labels: {app.kubernetes.io/name: db}}
    spec:
      serviceAccountName: web
      securityContext:
        runAsNonRoot: true
        runAsUser: 999
        fsGroup: 999
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: db
          image: registry.example.com/postgres@sha256:0000000000000000000000000000000000000000000000000000000000000000
          ports: [{name: pg, containerPort: 5432}]
          resources:
            requests: {cpu: 500m, memory: 1Gi}
            limits: {memory: 1Gi}
          securityContext:
            allowPrivilegeEscalation: false
            capabilities: {drop: [ALL]}
          volumeMounts: [{name: data, mountPath: /var/lib/postgresql/data}]
  volumeClaimTemplates:
    - metadata: {name: data}
      spec:
        accessModes: [ReadWriteOncePod]  # GA 1.29: one Pod, not just one node
        resources: {requests: {storage: 20Gi}}
```

- `serviceName` must name an existing headless Service.
- `volumeClaimTemplates` are immutable; changing them requires `kubectl delete sts --cascade=orphan` and re-creating the StatefulSet.
- `persistentVolumeClaimRetentionPolicy.whenScaled: Delete` deletes the data of scaled-down ordinals — flag it for databases unless intended.
- `ReadWriteOnce` guarantees a single **node**, `ReadWriteOncePod` (CSI volumes) a single **Pod**.
- **`rollingUpdate.maxUnavailable`** is not dependable across versions: honoured on 1.35.0–1.35.3 and 1.37, **silently dropped** on 1.34, 1.35.4+ and 1.36 (verified on 1.34.1, 1.35.0, 1.36.2, 1.37.0; the 1.35.4 change is from the changelog). Do not rely on it for correctness, and remove it from manifests for clusters that drop it (it causes permanent GitOps drift).

##### Jobs and CronJobs

```yaml
apiVersion: batch/v1
kind: Job
metadata: {name: import, namespace: shop}
spec:
  completionMode: Indexed
  completions: 10
  parallelism: 5
  backoffLimitPerIndex: 2           # beta-on 1.29, GA 1.33
  maxFailedIndexes: 3
  podFailurePolicy:                 # GA 1.31; requires restartPolicy: Never
    rules:
      - action: FailIndex
        onExitCodes: {containerName: importer, operator: In, values: [42]}
      - action: Ignore
        onPodConditions: [{type: DisruptionTarget}]
  activeDeadlineSeconds: 3600
  ttlSecondsAfterFinished: 86400
  template:
    spec:
      restartPolicy: Never
      serviceAccountName: web
      securityContext:
        runAsNonRoot: true
        runAsUser: 10001
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: importer
          image: registry.example.com/importer@sha256:0000000000000000000000000000000000000000000000000000000000000000
          resources:
            requests: {cpu: 500m, memory: 512Mi}
            limits: {memory: 512Mi}
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: {drop: [ALL]}
---
apiVersion: batch/v1
kind: CronJob
metadata: {name: report, namespace: shop}
spec:
  schedule: "15 2 * * *"
  timeZone: Europe/Berlin            # GA 1.27; never rely on the controller's local zone
  concurrencyPolicy: Forbid
  startingDeadlineSeconds: 300
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      backoffLimit: 2
      activeDeadlineSeconds: 1800
      template:
        spec:
          restartPolicy: OnFailure
          serviceAccountName: web
          securityContext:
            runAsNonRoot: true
            runAsUser: 10001
            seccompProfile: {type: RuntimeDefault}
          containers:
            - name: report
              image: registry.example.com/report@sha256:0000000000000000000000000000000000000000000000000000000000000000
              securityContext:
                allowPrivilegeEscalation: false
                capabilities: {drop: [ALL]}
```

- `podFailurePolicy` requires `restartPolicy: Never`; `backoffLimitPerIndex` requires `completionMode: Indexed`.
- Always bound runtime (`activeDeadlineSeconds`) and clean up (`ttlSecondsAfterFinished`); unbounded Jobs pile up Pods and objects.
- CronJobs: set `timeZone`, `concurrencyPolicy` and `startingDeadlineSeconds` explicitly; the schedule string must be quoted.
- `spec.managedBy` (beta-on 1.32, GA 1.35, immutable) hands the Job to an external controller; the built-in Job controller then ignores it entirely. Flag it outside MultiKueue-style setups.
- Idempotent one-shot Jobs and CronJobs may rely on termination and safe re-runs; explicit graceful shutdown is only required when the job holds a lease, lock, transaction or external reservation, or performs non-idempotent remote writes. External calls still need timeouts or `activeDeadlineSeconds`; absence of signal handling alone is not a bug.
- One-shot Jobs may report through logs and exit status when there is no owned object status to update.

##### PodDisruptionBudgets

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata: {name: web, namespace: shop}
spec:
  maxUnavailable: 1
  unhealthyPodEvictionPolicy: AlwaysAllow   # GA 1.31 (beta-on since 1.27)
  selector:
    matchLabels: {app.kubernetes.io/name: web}
```

- Every replicated, availability-sensitive workload gets a PDB.[^34]
- `maxUnavailable: 0` or `minAvailable` equal to the replica count blocks node drains forever — flag it.
- `unhealthyPodEvictionPolicy: AlwaysAllow` lets drains evict crash-looping Pods instead of blocking on them.
- In `policy/v1`, an empty `selector: {}` selects **all** Pods in the namespace.
- PDBs only govern voluntary evictions (drain, autoscaler), not node failures or direct deletes.

##### HorizontalPodAutoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: {name: web, namespace: shop}
spec:
  scaleTargetRef: {apiVersion: apps/v1, kind: Deployment, name: web}
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target: {type: Utilization, averageUtilization: 70}
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies: [{type: Percent, value: 50, periodSeconds: 60}]
```

- Use `autoscaling/v2` (`v2beta2` was removed in 1.26).
- `Utilization` targets require a request for that resource on every container measured; use `type: ContainerResource` (GA 1.30) when sidecars distort the Pod average.
- `minReplicas: 0` needs 1.37 (HPA scale-to-zero beta-on) **and** at least one `Object` or `External` metric; 1.36 and older reject it (verified).
- `behavior.*.tolerance` needs 1.35 (beta-on) / 1.37 (GA); it is stored as a canonical quantity (`0.05` → `50m`), which shows up as GitOps drift unless written as `50m`.

##### Storage

- PVC `storageClassName` is immutable; `volumeAttributesClassName` (`storage.k8s.io/v1` VolumeAttributesClass, GA 1.34) changes performance parameters of a bound volume.
- Size `emptyDir` with `sizeLimit`; `medium: Memory` counts against the container memory limit.
- Review `reclaimPolicy: Delete` on StorageClasses used for irreplaceable data.

---

#### Pod Security

##### Pod Security Admission (GA since 1.25)

Label every namespace. Pin `enforce-version` so a control-plane upgrade does not silently change the rules; leave `warn`/`audit` on `latest` (the default when no `*-version` label is set) to see upcoming violations early.[^14]

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: shop
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: v1.34   # pin to the oldest cluster minor you run
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/audit: restricted
```

A bare Pod in that namespace is rejected with the full list of missing settings (verified): `violates PodSecurity "restricted:v1.34": allowPrivilegeEscalation != false …, unrestricted capabilities …, runAsNonRoot != true …, seccompProfile …`. PSA checks **Pods**: a Deployment with a non-compliant template is admitted (with a `would violate PodSecurity "restricted:latest"` warning from the `warn` label) and then its ReplicaSet fails to create Pods — the failure only shows up in ReplicaSet events.

##### The restricted baseline every workload should meet

```yaml
spec:
  securityContext:                      # Pod level
    runAsNonRoot: true
    runAsUser: 10001                     # numeric UID; image USER names cannot be verified
    runAsGroup: 10001
    fsGroup: 10001
    seccompProfile: {type: RuntimeDefault}
  containers:
    - name: app
      image: registry.example.com/app@sha256:0000000000000000000000000000000000000000000000000000000000000000
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true     # not required by PSA, strongly recommended
        capabilities: {drop: [ALL]}      # restricted allows adding back only NET_BIND_SERVICE
```

Flag: `privileged: true`, `hostNetwork`/`hostPID`/`hostIPC`, `hostPath` volumes, added capabilities (`SYS_ADMIN`, `NET_ADMIN`), `seccompProfile: Unconfined`, `procMount: Unmasked`, running as UID 0 — each needs an explicit justification and a namespace with a weaker PSA level.

##### Version-tied hardening options

- `securityContext.appArmorProfile: {type: RuntimeDefault}` — 1.30+ (GA 1.31); replaces the deprecated annotations.
- `spec.hostUsers: false` (user namespaces) — beta-on 1.33, GA 1.36; **silently dropped on 1.30–1.32** (verified), and requires node support (Linux ≥ 6.3 with idmap mounts on the volume filesystems, a recent containerd/CRI-O). Root inside a user namespace is not root on the host; for such Pods PSA skips the `runAsNonRoot`/`runAsUser` checks, but only for policy versions **v1.35 and later** (verified on 1.37: a root Pod with `hostUsers: false` is rejected by `restricted:v1.34` with `runAsNonRoot != true` and admitted by `restricted:v1.35` and `restricted:latest`).[^17] Cannot be combined with `hostNetwork`/`hostPID`/`hostIPC`.
- `supplementalGroupsPolicy: Strict` — beta-on 1.33, GA 1.35; stops groups from the image's `/etc/group` being added.
- `seLinuxChangePolicy` (GA 1.36) and the `SELinuxMount` default change in 1.37: on SELinux nodes, Pods with *different* SELinux labels sharing one volume can block in `ContainerCreating` on 1.37; such Pods need `seLinuxChangePolicy: Recursive`.

---

#### Services, Ingress, Gateway API and NetworkPolicy

##### Services

```yaml
apiVersion: v1
kind: Service
metadata: {name: web, namespace: shop}
spec:
  selector: {app.kubernetes.io/name: web}
  ports:
    - name: http
      port: 80
      targetPort: http                  # named port survives containerPort changes
      appProtocol: http
```

- Selector labels must match the Pod template labels exactly; a typo yields a Service with zero endpoints and no error.
- Name every port; multi-port Services require names.
- `type: LoadBalancer` needs `loadBalancerSourceRanges` or a firewall when the backend is not public; `externalTrafficPolicy: Local` preserves the client IP but only routes to nodes with ready endpoints.
- `internalTrafficPolicy: Local` is a hard requirement (no fallback); `trafficDistribution` is a preference with fallback — `PreferClose` on 1.31+, `PreferSameZone`/`PreferSameNode` on 1.34+ (verified rejected on 1.33). Same-zone routing does not balance load across zones; flag it when endpoints are unevenly spread.
- Do not use `spec.externalIPs` (deprecated 1.36, CVE-2020-8554); clusters should enable the `DenyServiceExternalIPs` admission plugin or a policy.
- Write canonical IPs/CIDRs: from 1.36 the API rejects `10.0.0.1/8` (`must not have bits set beyond the prefix length`) and leading zeros (`010.0.0.0/8`); 1.33–1.35 accept them with a warning, ≤ 1.32 silently (verified). Controllers that write IPs must canonicalise them (`netip.Prefix.Masked()`).

##### Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata: {name: web, namespace: shop}
spec:
  ingressClassName: example         # not the deprecated kubernetes.io/ingress.class annotation
  tls:
    - hosts: [shop.example.com]
      secretName: shop-tls
  rules:
    - host: shop.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web
                port: {name: http}
```

- `networking.k8s.io/v1` Ingress is GA and feature-frozen; controller-specific behaviour lives in annotations, which do not port between controllers.
- Flag `nginx.ingress.kubernetes.io/*-snippet` annotations (raw config injection) and any new dependency on the retired community Ingress NGINX controller.[^22]

##### Gateway API

Gateway API (`gateway.networking.k8s.io/v1`) is the successor to Ingress. It is **not built into the API server**: its CRDs are installed separately (v1.6.3 is the latest release as of October 2026) and an implementation (a controller) must be present.[^21] Verified by installing the v1.6.3 standard-channel CRDs on 1.37:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata: {name: public, namespace: gateway}
spec:
  gatewayClassName: example
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      hostname: "*.example.com"
      tls:
        mode: Terminate
        certificateRefs: [{name: wildcard-tls}]
      allowedRoutes:
        namespaces:
          from: Selector
          selector:
            matchLabels: {gateway.example.com/expose: "true"}
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata: {name: web, namespace: shop}
spec:
  parentRefs:
    - {name: public, namespace: gateway, sectionName: https}
  hostnames: [shop.example.com]
  rules:
    - matches:
        - path: {type: PathPrefix, value: /}
      backendRefs:
        - {name: web, port: 80}
```

- Restrict `allowedRoutes` (`from: Same` or a label `Selector`) — `from: All` lets any namespace attach routes to a shared Gateway.
- Cross-namespace references from a route to a Service or Secret in another namespace need a `ReferenceGrant` in the target namespace.
- Check `status.parents[].conditions` (`Accepted`, `ResolvedRefs`) of routes; a route that is not accepted silently serves nothing.
- Use only standard-channel resources unless the cluster deliberately installs the experimental channel.

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
          podSelector:                    # same element: namespace AND pod
            matchLabels: {k8s-app: kube-dns}
      ports:
        - {port: 53, protocol: UDP}
        - {port: 53, protocol: TCP}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: web-from-gateway, namespace: shop}
spec:
  podSelector:
    matchLabels: {app.kubernetes.io/name: web}
  policyTypes: [Ingress]
  ingress:
    - from:
        - namespaceSelector:
            matchLabels: {kubernetes.io/metadata.name: gateway}
      ports: [{port: http, protocol: TCP}]
```

- Policies are additive allow-lists;[^20] a Pod selected by any policy of a type is isolated for that type.
- **`namespaceSelector` and `podSelector` in the same `from`/`to` element are ANDed; as separate list elements they are ORed** — a classic review bug that silently opens traffic.
- A default-deny egress policy without a DNS allowance (UDP **and** TCP 53) breaks name resolution.
- `kubernetes.io/metadata.name` is set automatically on every namespace and is the safe way to select a namespace by name.
- NetworkPolicy does nothing unless the CNI enforces it; it has no L7 or FQDN rules (CNI-specific CRDs do).
- Operator egress: allow the API server (port 443/6443) and DNS; pin the API server with an `ipBlock` from `kubectl get endpointslices -n default -l kubernetes.io/service-name=kubernetes` for a tighter policy.

---

#### Configuration and Secrets

```yaml
apiVersion: v1
kind: ConfigMap
metadata: {name: web-config-5f2c9a, namespace: shop}   # content hash in the name
immutable: true
data:
  LOG_LEVEL: info
  FEATURE_X_ENABLED: "true"         # quoted: unquoted true/yes/on become booleans
  API_VERSION: "1.10"               # quoted: unquoted 1.10 becomes the number 1.1
---
apiVersion: v1
kind: Secret
metadata: {name: web-db, namespace: shop}
type: Opaque
immutable: true
stringData:
  DB_PASSWORD: change-me            # in Git only via SOPS/Sealed Secrets/External Secrets
```

- **Immutable, content-addressed** ConfigMaps/Secrets (Kustomize `configMapGenerator`, Helm checksum annotations) make config changes roll the workload and stop the kubelet from watching them. Edits are rejected (verified: `data: Forbidden: field is immutable when immutable is set`).
- Environment variables from ConfigMaps/Secrets are read **once at container start**; volume-mounted keys update eventually (not with `subPath`). Code that expects live reload from env vars is a bug.
- **Never commit plaintext Secrets.**[^36] Base64 in `data` is encoding, not encryption. Use SOPS, Sealed Secrets, External Secrets Operator or a CSI secrets-store driver; ensure encryption at rest is enabled on the cluster.
- Never put secrets in ConfigMaps, CR `spec` fields, annotations, labels, command-line arguments, env `value:` literals, Event notes or condition messages.
- Grant `get` on named Secrets (`resourceNames`) rather than `list`/`watch` on all Secrets — `list` returns every value.
- Prefer projected ServiceAccount tokens with an `audience` and short `expirationSeconds` for workload identity; never create `kubernetes.io/service-account-token` Secrets for new code. Pod certificates (`podCertificate` projected volume) are GA only from 1.37.
- Env files (`fileKeyRef`, beta-on 1.35) and `emptyDir` are not secret stores.

---

#### RBAC and Least Privilege

##### Rules

1. Grant only the verbs and resources the code uses. Generate operator roles from `+kubebuilder:rbac` markers and review the diff of `config/rbac/role.yaml`.
2. Prefer `Role`/`RoleBinding` (namespaced) over `ClusterRole`/`ClusterRoleBinding`; use cluster scope only for cluster-scoped resources or all-namespace watches.
3. No wildcard verbs or resources, no `cluster-admin` bindings for workloads.
4. Treat as privileged (needs justification): `escalate`, `bind`, `impersonate`, `create` on `serviceaccounts/token`, `pods/exec`, `pods/attach`, `pods/portforward`, `nodes/proxy`, `create` on Pods/workloads in privileged namespaces (it allows mounting any ServiceAccount of that namespace), `update` on `validatingwebhookconfigurations`/`mutatingwebhookconfigurations`/admission policies, and `list`/`watch` on Secrets.[^18]
5. Status and finalizers are separate subresources: `<resource>/status` (`get;update;patch`) and `<resource>/finalizers` (`update`) — the latter is needed to set `blockOwnerDeletion: true` on children when the `OwnerReferencesPermissionEnforcement` admission plugin is enabled.
6. Events: controller-runtime ≥ v0.23's recorder writes **`events.k8s.io/v1`** Events — grant `events.k8s.io` `events` `create;patch`. The core-group (`""`) rule alone silently loses Events (the recorder only logs the failure).
7. Leader election needs `coordination.k8s.io` `leases` in the election namespace.
8. Verify with `kubectl auth can-i --as=system:serviceaccount:<ns>:<sa> <verb> <resource>` and `kubectl auth can-i --list --as=…`.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata: {name: redis-operator}
rules:
  - apiGroups: [cache.example.com]
    resources: [redisclusters]
    verbs: [get, list, watch, update, patch]
  - apiGroups: [cache.example.com]
    resources: [redisclusters/status]
    verbs: [get, update, patch]
  - apiGroups: [cache.example.com]
    resources: [redisclusters/finalizers]   # needed to set blockOwnerDeletion on children
    verbs: [update]
  - apiGroups: [apps]
    resources: [statefulsets]
    verbs: [get, list, watch, create, update, patch, delete]
  - apiGroups: [events.k8s.io]              # controller-runtime >= v0.23 recorder
    resources: [events]
    verbs: [create, patch]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: {name: redis-operator-leader-election, namespace: redis-system}
rules:
  - apiGroups: [coordination.k8s.io]
    resources: [leases]
    verbs: [get, list, watch, create, update, patch, delete]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: {name: read-tls-secret, namespace: shop}
rules:
  - apiGroups: [""]
    resources: [secrets]
    resourceNames: [shop-tls]               # get on one named Secret, no list/watch
    verbs: [get]
```

`resourceNames` cannot restrict `create` or `deletecollection` (the name is not known at authorization time); a `list`/`watch` grant with `resourceNames` only works for clients that send a matching `metadata.name` field selector. A plain `list`/`watch` grant covers every object of the type.

##### The `pods/exec` `get` loophole (fixed by default in 1.35)

Interactive `exec`, `attach` and `port-forward` use HTTP `POST` (SPDY) or a WebSocket upgrade, which is an HTTP `GET`. Before 1.35, the API server authorized the WebSocket `GET` with the RBAC verb **`get`** — so a "read-only" role that granted `get` on `pods/exec` (or on `pods/*`) allowed command execution in any Pod of the namespace. With `AuthorizePodWebsocketUpgradeCreatePermission` (beta-on from **1.35**) the WebSocket upgrade additionally requires `create`.[^19] Verified with a raw WebSocket upgrade by a user bound only to `get` on `pods` and `pods/exec`:

```text
1.34.1:  "pod web does not have a host assigned"   (authorization passed; failed later)
1.35.0:  pods "web" is forbidden                     (403 at authorization)
1.37.0:  pods "web" is forbidden
```

```yaml
# BAD: on clusters <= 1.34 this "read-only" rule allows exec over WebSockets
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "pods/exec"]
    verbs: ["get", "list", "watch"]
# GOOD: read-only roles never name exec/attach/portforward; grant exec explicitly with create
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log"]
    verbs: ["get", "list", "watch"]
```

Review rules: never include `pods/exec`, `pods/attach`, `pods/portforward` or `pods/*` in read-only roles regardless of verb; on clusters ≤ 1.34 (or with the gate disabled) treat `get` on them as exec permission. Conversely, tooling that relied on `get` alone breaks on 1.35+ and needs `create`. `nodes/proxy` is broader still (it reaches the kubelet API directly); use the fine-grained `nodes/metrics`, `nodes/stats`, `nodes/pods`, `nodes/healthz` subresources (GA 1.36, beta 1.33) for monitoring agents.

##### Service accounts

```yaml
apiVersion: v1
kind: ServiceAccount
metadata: {name: web, namespace: shop}
automountServiceAccountToken: false
```

One ServiceAccount per workload (not `default`), automounting off by default and enabled on the Pod only when the code calls the API server.

---

#### CRD and API Design

##### Go types with Kubebuilder markers

Compiles against `k8s.io/apimachinery` v0.37.1 and v0.35.9; `controller-gen` v0.22.0 generates a CRD from it that kube-apiserver 1.37 accepts (`selectablefield` needs a 1.31+/1.32+ server, see below).

```go
// RedisClusterSpec defines the desired state of RedisCluster.
// +kubebuilder:validation:XValidation:rule="!has(self.minReplicas) || self.minReplicas <= self.replicas",messageExpression="'minReplicas (%d) must be <= replicas (%d)'.format([self.minReplicas, self.replicas])",fieldPath=".minReplicas",reason="FieldValueInvalid"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.storageClassName) || has(self.storageClassName)",message="storageClassName cannot be removed once set"
type RedisClusterSpec struct {
	// replicas is the desired number of Redis nodes.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +required
	Replicas int32 `json:"replicas"`

	// minReplicas is the lowest replica count the operator may scale to.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// tier selects the instance size.
	// +kubebuilder:default=small
	// +optional
	Tier Tier `json:"tier,omitempty"`

	// storageClassName for data volumes. Immutable once set.
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="storageClassName is immutable"
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// image is the Redis image, pinned by digest.
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:XValidation:rule="self.contains('@sha256:')",message="image must be pinned by digest"
	// +required
	Image string `json:"image"`

	// ports exposed by each node.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=8
	// +optional
	Ports []RedisPort `json:"ports,omitempty"`
}

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
	// +kubebuilder:validation:MaxItems=16
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
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisClusterSpec   `json:"spec"`
	Status RedisClusterStatus `json:"status,omitempty"`
}
```

- Newer Kubebuilder scaffolds write `json:"metadata,omitzero"`/`json:"status,omitzero"` (Go 1.24+) and `k8s.io/api` 1.37 tags embedded `TypeMeta` with `json:""`; both forms produce the same JSON as the classic tags. Do not flag either.
- Use `// +optional` / `// +required` explicitly; `controller-gen` derives the schema's `required` list from them.
- Use pointers (`*int32`) when "unset" and "zero" must be distinguishable; otherwise `omitempty` on `0` makes the value impossible to set.

##### Spec design rules

- Prefer **lists of named sub-objects** with `+listType=map` / `+listMapKey=name` over maps — they merge per entry under server-side apply. Use `+listType=set` for unique scalars and `atomic` only for lists that are always replaced whole.
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
- **Do not use `bool` where an enum will be needed** — a string type alias from day one.
- **No raw maps** except labels, annotations and opaque user data.
- **Only `apiVersion`, `kind`, `metadata`, `spec`, `status`** at the top level.
- **Bound every string, list and map** (`MaxLength`, `MaxItems`, `MaxProperties`). Without bounds, CEL rules on those fields exceed the cost budget and the CRD is rejected, and unbounded objects hurt etcd and every watch cache.
- **Numeric formats**: from 1.36 the API server rejects values outside `format: int32`/`int64` ranges; 1.35 and older store them. Always add explicit `minimum`/`maximum` and keep schema format and Go type aligned.
- Reference same-namespace objects by name (`secretName`); never embed secret material.
- Optional fields with defaults (`+kubebuilder:default`) are applied by the API server on create *and* when reading old stored objects.

##### Structural schemas, pruning and unknown fields

`apiextensions.k8s.io/v1` requires a **structural** schema: every field has a `type`, and unknown fields are **pruned** unless `x-kubernetes-preserve-unknown-fields: true`. Verified on 1.37 with a CR that sets an undeclared `spec.replicaCount`:

- `kubectl apply` (strict field validation): `strict decoding error: unknown field "spec.replicaCount"`.
- `kubectl apply --validate=warn`: `Warning: unknown field "spec.replicaCount"`, object stored **without** it.
- `kubectl apply --validate=false` and clients that do not request validation: stored without it, no message.

Review rules: flag `x-kubernetes-preserve-unknown-fields: true` on anything except genuinely opaque sub-trees (embedded user config), and `x-kubernetes-embedded-resource` without bounds. A typo in a CR field is silently lost for any client that does not request strict validation.

##### The CRD manifest

The CRD that the markers above describe, written by hand. Applied on 1.34 and 1.37 (on 1.29 `selectableFields` is an unknown field and the CRD is rejected; on 1.30 it would be silently dropped).

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: redisclusters.cache.example.com
spec:
  group: cache.example.com
  scope: Namespaced
  names:
    kind: RedisCluster
    listKind: RedisClusterList
    plural: redisclusters
    singular: rediscluster
    shortNames: [rdc]                 # "rc" is ReplicationController
    categories: [cache]
  versions:
    - name: v1
      served: true
      storage: true
      subresources:
        status: {}
      selectableFields:               # GA 1.32 (beta-on 1.31)
        - jsonPath: .spec.tier
      additionalPrinterColumns:
        - {name: Tier, type: string, jsonPath: .spec.tier}
        - {name: Ready, type: string, jsonPath: '.status.conditions[?(@.type=="Ready")].status'}
        - {name: Age, type: date, jsonPath: .metadata.creationTimestamp}
      schema:
        openAPIV3Schema:
          type: object
          required: [spec]
          properties:
            spec:
              type: object
              required: [replicas]
              x-kubernetes-validations:
                - rule: "!has(self.minReplicas) || self.minReplicas <= self.replicas"
                  messageExpression: "'minReplicas (%d) must be <= replicas (%d)'.format([self.minReplicas, self.replicas])"
                  fieldPath: .minReplicas
                  reason: FieldValueInvalid
                - rule: "!has(oldSelf.storageClassName) || has(self.storageClassName)"
                  message: storageClassName cannot be removed once set
                - rule: "self.tier != 'large' || self.replicas >= 3"
                  message: tier large needs at least 3 replicas
                  fieldPath: .replicas
              properties:
                replicas: {type: integer, format: int32, minimum: 1, maximum: 100}
                minReplicas: {type: integer, format: int32, minimum: 0, maximum: 100}
                tier:
                  type: string
                  enum: [small, medium, large]
                  default: small
                storageClassName:
                  type: string
                  maxLength: 253
                  x-kubernetes-validations:
                    - rule: "self == oldSelf"
                      message: storageClassName is immutable
                image:
                  type: string
                  maxLength: 512
                  x-kubernetes-validations:
                    - rule: "self.contains('@sha256:')"
                      message: image must be pinned by digest
                memory:
                  type: string
                  maxLength: 16
                  x-kubernetes-validations:
                    - rule: "isQuantity(self) && quantity(self).isGreaterThan(quantity('64Mi'))"
                      message: memory must be a quantity above 64Mi
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
                      port: {type: integer, format: int32, minimum: 1, maximum: 65535}
                tls:
                  type: object
                  properties:
                    secretName: {type: string, maxLength: 253}
                  x-kubernetes-validations:
                    - rule: "has(self.secretName) && self.secretName != ''"
                      message: tls.secretName is required when tls is set
            status:
              type: object
              properties:
                observedGeneration: {type: integer, format: int64}
                readyReplicas: {type: integer, format: int32}
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
                      reason: {type: string, minLength: 1, maxLength: 1024}
                      message: {type: string, maxLength: 32768}
```

An invalid object is rejected with all violations at once (verified on 1.37):

```yaml
apiVersion: cache.example.com/v1
kind: RedisCluster
metadata: {name: bad, namespace: default}
spec:
  replicas: 2
  minReplicas: 5
  tier: large
  storageClassName: fast
  image: registry.example.com/redis:7.2
  memory: 32Mi
  tls: {}
```

```text
The RedisCluster "bad" is invalid:
* spec.minReplicas: Invalid value: minReplicas (5) must be <= replicas (2)
* spec.replicas: Invalid value: tier large needs at least 3 replicas
* spec.image: Invalid value: "registry.example.com/redis:7.2": image must be pinned by digest
* spec.memory: Invalid value: "32Mi": memory must be a quantity above 64Mi
* spec.tls: Invalid value: tls.secretName is required when tls is set
```

Design checklist for this manifest:

- `subresources.status: {}` is present, so main-resource writes ignore `status` and controllers write through `/status`.
- `additionalPrinterColumns`: at least a state column (from a condition) and `Age`.
- `selectableFields` (GA 1.32, beta-on 1.31; up to 8 per version; string, integer or boolean fields) enable server-side `--field-selector`: verified `kubectl get rdc --field-selector spec.tier=small` works, `spec.replicas=3` fails with `field label not supported: spec.replicas`.
- `categories: [cache]` instead of `all`, short name that does not collide.

---

#### CEL Validation Rules

CRD validation rules (`x-kubernetes-validations`, GA 1.29) are the first choice for invariants the OpenAPI schema cannot express. The same expression language drives ValidatingAdmissionPolicy, MutatingAdmissionPolicy and webhook `matchConditions`.[^7][^8]

##### Cost budget

The API server **estimates** the worst-case cost of every rule when the CRD is written (per-rule limit 10,000,000 and per-CRD limit 100,000,000 cost units in `k8s.io/apiextensions-apiserver` v0.37) and, at runtime, limits each evaluation (1,000,000 per call, 10,000,000 per object, `k8s.io/apiserver` `PerCallLimit`/`RuntimeCELCostBudget`). Unbounded inputs make the estimate explode. Verified on 1.37:

```yaml
# BAD: both rules make the CRD uninstallable
x-kubernetes-validations:
  - rule: "self.a <= self.b"
    messageExpression: "'a (' + string(self.a) + ') must be <= b'"   # string(int) is costed as unbounded
properties:
  a: {type: integer, format: int32, minimum: 0, maximum: 100}
  b: {type: integer, format: int32, minimum: 0, maximum: 100}
  names:
    type: array                      # no maxItems
    items: {type: string}            # no maxLength
    x-kubernetes-validations:
      - rule: "self.all(n, n.matches('^[a-z]+$'))"
```

```text
* …properties[names].x-kubernetes-validations[0].rule: Forbidden: estimated rule cost exceeds budget by factor of more than 100x (try simplifying the rule, or adding maxItems, maxProperties, and maxLength where arrays, maps, and strings are declared)
* …x-kubernetes-validations[0].messageExpression: Forbidden: estimated messageExpression cost exceeds budget by factor of more than 100x …
```

```yaml
# GOOD: bounded inputs and format() instead of string concatenation
x-kubernetes-validations:
  - rule: "self.a <= self.b"
    messageExpression: "'a (%d) must be <= b (%d)'.format([self.a, self.b])"
properties:
  names:
    type: array
    maxItems: 16
    items: {type: string, maxLength: 63}
```

`'…'.format([…])` needs 1.30+ (verified: 1.29 rejects the `messageExpression` at CRD write time, 1.30 accepts it and reports `a (5) must be <= b (1)`); for 1.29 use a static `message` with `fieldPath`.

##### Transition rules and immutability

- A rule that references `oldSelf` is a **transition rule**: it runs only on update and only when the old value exists. `self == oldSelf` on the field is the canonical immutability rule.
- **`self == oldSelf` does not prevent removing the field.** Verified on 1.37: with only the field-level rule, a JSON patch `remove /spec/storageClassName` succeeds; adding the parent-level rule `!has(oldSelf.storageClassName) || has(self.storageClassName)` rejects it with `storageClassName cannot be removed once set`.
- For "set once": `optionalOldSelf: true` (1.30+) with `!oldSelf.hasValue() || oldSelf.value() == self`. `has(oldSelf)` does not compile (`has()` needs a field selection), and `optionalOldSelf` may only be set on rules that use `oldSelf`.
- Guard optional fields: `has(self.f)` or `self.?f.orValue(x)` (optional types, 1.29+). Accessing an absent field is a runtime error (`no such key`) that fails **every** update of such objects — including a controller's own finalizer patches.

##### Ratcheting (beta-on 1.30, GA 1.33)

Unchanged fields of existing objects are not re-validated against a tightened schema on update, so adding a stricter rule does not block unrelated updates (e.g. a controller adding a finalizer). New objects and changed fields are still validated strictly — tightening validation in a served version remains a **breaking change** for clients. On ≤ 1.29 there is no ratcheting: a new rule breaks every update of objects that violate it.

##### Other rules

- `reason` is one of `FieldValueInvalid` (default), `FieldValueForbidden`, `FieldValueRequired`, `FieldValueDuplicate`; `fieldPath` points the error at a sub-field (`spec.minReplicas: …` instead of `spec: …`).
- `messageExpression` must return a string; if it fails, `message` is used.
- Put rules at the narrowest level: a rule on an item of a list runs (and is costed) `maxItems` times.
- Check the [CEL library table](#cel-library-availability) against the oldest cluster: `semver()` needs 1.34 (verified rejected on 1.33), `format.*` 1.32, `ip()`/`cidr()` 1.31.
- Prefer CEL in the CRD over a validating webhook for anything that only needs the object itself.
- 1.37 fixed CEL equality and `+` for `x-kubernetes-list-type: set`/`map` lists; rules that compared or concatenated such lists can evaluate differently after an upgrade.

---

#### Spec, Status and Conditions

| | `spec` | `status` |
|---|---|---|
| **Written by** | users, GitOps, CI | controllers only |
| **Represents** | desired state | observed state |
| **Subresource** | main resource | `/status` — always enable it |
| **Main-resource writes** | update spec/metadata | `status` changes ignored |

- Set `status.observedGeneration = metadata.generation` after reconciling, and `observedGeneration` on every condition; tools (`kubectl wait`, Argo CD, Flux) use it to know whether status describes the latest spec. `metadata.generation` increments on spec changes, not on metadata or status changes (verified: a label/finalizer patch keeps generation 1).
- Use `[]metav1.Condition` with `+listType=map` / `+listMapKey=type`; set them with `meta.SetStatusCondition`, which only changes `lastTransitionTime` when `status` changes and returns whether anything changed — use that to skip no-op writes and duplicate Events.

| Condition field | Rule |
|---|---|
| `type` | PascalCase adjective or past participle: `Ready`, `Available`, `Progressing`, `Degraded` |
| `status` | `True`, `False` or `Unknown` — never empty |
| `reason` | CamelCase, machine-readable, required (`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`, ≤ 1024) |
| `message` | human-readable, ≤ 32768, never secrets |
| `observedGeneration` | `metadata.generation` when the condition was computed |
| `lastTransitionTime` | changes only when `status` changes |

Standard types: `Ready`/`Available` (True when healthy), `Progressing`/`Reconciling` (True while converging), `Degraded`/`Stalled` (True when something is wrong; `Stalled` means a human must act). Pick one polarity per type and document it.

> ⚠️ Do not add new `phase` string fields; conditions are the API convention.[^9] Existing `phase` fields may stay for compatibility, derived from conditions.

---

#### API Versioning, Conversion and Storage Migration

| Stage | Stability | Built-in API served by default | Compatibility |
|---|---|---|---|
| `v1alphaX` | experimental | no | none |
| `v1betaX` | pre-release | **no** for beta APIs introduced since 1.24 | best effort, deprecation policy |
| `v1` | stable | yes | required |

For CRDs, "enabled" is simply the `served` flag the author sets per version.

Rules:

1. Never remove or rename fields in a served version; add new fields as optional.
2. Never tighten validation so previously valid objects become invalid; ratcheting (1.33 GA) protects stored objects on unrelated updates, not clients that create new ones.
3. Exactly one version has `storage: true`; conversion must be lossless in both directions (round-trip fields that do not exist in the other version through annotations).
4. Use `spec.conversion.strategy: Webhook` (with `conversionReviewVersions: [v1]`) when schemas differ; `None` only when schemas are identical apart from the version name. The conversion webhook is on the read path of **every** request for that resource — size it for concurrency and availability.
5. Deprecate with `deprecated: true` and a `deprecationWarning`, keep the old version served long enough for clients to migrate (built-in beta APIs follow the deprecation policy[^37]), then set `served: false`.
6. **Before dropping a version from `spec.versions`**, rewrite all stored objects to the current storage version and remove the old version from `status.storedVersions`. On **1.37+** create a `storagemigration.k8s.io/v1` `StorageVersionMigration` (GA 1.37) and wait for its `Succeeded` condition — the migrator also prunes `storedVersions` for CRDs. On ≤ 1.36 the API is not served by default; use the out-of-tree kube-storage-version-migrator or a read/replace loop, then `kubectl patch crd <name> --subresource=status` to fix `storedVersions`.[^27]

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

#### Admission Control

##### Choose the mechanism (prefer left to right)

```
CRD schema + CEL   →   ValidatingAdmissionPolicy (GA 1.30)   →   MutatingAdmissionPolicy (GA 1.36)   →   admission webhook
(own CRDs only)        (any resource, in-process CEL)            (any resource, in-process CEL)          (external service)
```

Webhooks are justified only for what CEL cannot express: calls to external systems, lookups of other objects beyond `params`/`authorizer`, image signature verification. Check the cluster version before replacing a webhook: VAP needs **1.30** (not served on 1.29, verified), MAP `v1` needs **1.36** (not served on 1.35, verified; `v1beta1` on 1.34–1.35 is off by default).

##### ValidatingAdmissionPolicy (GA 1.30)[^11]

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata: {name: require-image-digest}
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups: [apps]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [deployments, statefulsets, daemonsets]
  variables:
    - name: containers
      expression: "object.spec.template.spec.containers + object.spec.template.spec.?initContainers.orValue([])"
  validations:
    - expression: "variables.containers.all(c, c.image.contains('@sha256:'))"
      messageExpression: "'images must be pinned by digest: ' + variables.containers.filter(c, !c.image.contains('@sha256:')).map(c, c.name).join(', ')"
      reason: Invalid
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata: {name: require-image-digest}
spec:
  policyName: require-image-digest
  validationActions: [Warn, Audit]          # switch to [Deny] after the audit period
  matchResources:
    namespaceSelector:
      matchLabels: {policy.example.com/enforce-digests: "true"}
```

Verified on 1.37: with `[Warn, Audit]` a tagged image produces `Warning: Validation failed for ValidatingAdmissionPolicy 'require-image-digest' with binding 'require-image-digest': images must be pinned by digest: nginx`; with `[Deny]` the request fails with `… denied request: images must be pinned by digest: nginx`; `[Deny, Warn]` is rejected (`must not contain both Deny and Warn`).

Review points:

- A policy does nothing without a **binding**; roll out with `[Warn, Audit]`, then `[Deny]`.
- Use `object.spec.?field.orValue(…)` for optional fields; `oldObject` is `null` on CREATE. A runtime error triggers `failurePolicy`.
- Exempt system namespaces and controllers deliberately with `matchConditions` or binding selectors; remember that Pods are created by controllers (`system:serviceaccount:kube-system:*`), so blanket `system:` exemptions can disable the policy for every workload Pod.
- Match `pods` to police workloads regardless of the owning kind, but expect rejections to surface only in ReplicaSet/Job events.
- `paramKind` + `paramRef` (with a deliberate `parameterNotFoundAction`) make policies configurable.
- Read `status.typeChecking` after creating a policy to catch type errors.
- There is no `vap` short name; use `validatingadmissionpolicies`.

##### MutatingAdmissionPolicy (GA 1.36)[^12]

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingAdmissionPolicy
metadata: {name: default-team-label}
spec:
  failurePolicy: Fail
  reinvocationPolicy: IfNeeded            # required: Never | IfNeeded
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: [v1]
        operations: [CREATE]
        resources: [configmaps]
  matchConditions:                        # makes the mutation idempotent: only if absent
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
metadata: {name: default-team-label}
spec:
  policyName: default-team-label
  matchResources:
    namespaceSelector:
      matchLabels: {kubernetes.io/metadata.name: shop}
```

Verified on 1.37: a ConfigMap created in `shop` gets `example.com/team: unassigned`; on 1.34 and 1.35 the manifest fails with `no matches for kind "MutatingAdmissionPolicy" in version "admissionregistration.k8s.io/v1"`. Prefer `ApplyConfiguration` (merge semantics); `JSONPatch` `add` to a missing parent path fails, so guard it with `has()`. Mutations must default only when absent and must be idempotent.

##### Admission webhooks[^13]

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: vrediscluster.cache.example.com
  annotations:
    cert-manager.io/inject-ca-from: redis-system/webhook-cert   # CA bundle injected and rotated
webhooks:
  - name: vrediscluster.cache.example.com
    admissionReviewVersions: [v1]
    sideEffects: None                     # required; enables --dry-run=server
    failurePolicy: Fail                   # v1 default; Ignore only for best-effort checks
    matchPolicy: Equivalent               # v1 default
    timeoutSeconds: 5                     # default 10, max 30
    clientConfig:
      service:
        name: redis-operator-webhook
        namespace: redis-system
        path: /validate-cache-example-com-v1-rediscluster
        port: 443
    rules:
      - apiGroups: [cache.example.com]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [redisclusters]
        scope: Namespaced
    namespaceSelector:                    # never intercept the webhook's own namespace
      matchExpressions:
        - {key: kubernetes.io/metadata.name, operator: NotIn, values: [kube-system, redis-system]}
    matchConditions:                      # GA 1.30: CEL pre-filter, no HTTP call when false
      - name: skip-node-identities
        expression: "!request.userInfo.username.startsWith('system:node:')"
```

Defaults in `admissionregistration.k8s.io/v1` (verified on 1.37 by applying a minimal configuration): `failurePolicy: Fail`, `timeoutSeconds: 10`, `matchPolicy: Equivalent`, `namespaceSelector: {}` (all namespaces).

- **`failurePolicy: Fail` + a selector that matches the webhook's own namespace (or `kube-system`) can deadlock the cluster**: the webhook Pod cannot be recreated while the webhook is down. Exclude them with `namespaceSelector`/`objectSelector`.
- A webhook whose `rules` match `*` resources, or `pods` cluster-wide, is in the critical path of the whole cluster — demand HA (≥ 2 replicas, PDB, anti-affinity) and short timeouts.
- `sideEffects: None` (or `NoneOnDryRun`) is required for `--dry-run=server`; webhooks with side effects must honour `dryRun`.
- Mutating webhooks must be idempotent and set `reinvocationPolicy: IfNeeded` when other mutators may change the fields they depend on.
- TLS: rotate certificates (cert-manager, controller-runtime `certwatcher`), never `insecureSkipTLSVerify`; `clientConfig.url` webhooks outside the cluster need strict network controls.
- Handle `CREATE` **and** `UPDATE` for immutability checks; `DELETE` only for deletion guards.
- Do not do uncached `List` calls per admission request in large clusters.

---

#### Controller Patterns

The reconciler below compiles and passes a fake-client unit test and an envtest integration test (real 1.37 / 1.35 API server, CEL rejection, SSA-created child with owner reference and field manager, status, finalizer-driven deletion) against controller-runtime **v0.25.2** and **v0.23.3**. It needs controller-runtime ≥ v0.22 for `Client.Apply` and ≥ v0.23 for `GetEventRecorder`.

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
	fieldOwner     = client.FieldOwner("redis-operator") // constant across releases
	cleanupFinal   = "cache.example.com/external-cleanup"
	conditionReady = "Ready"
)

// errPermanent marks errors that retrying cannot fix (invalid user input).
var errPermanent = errors.New("permanent")

// BackupAPI is the external system the operator cleans up on deletion.
type BackupAPI interface {
	DeleteBackups(ctx context.Context, namespace, name string) error
}

// RedisClusterReconciler reconciles RedisCluster objects.
type RedisClusterReconciler struct {
	client.Client
	Recorder events.EventRecorder // mgr.GetEventRecorder(...) (controller-runtime >= v0.23)
	Backups  BackupAPI
}

// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

func (r *RedisClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch. NotFound means the object is gone: nothing to do.
	rc := &cachev1.RedisCluster{}
	if err := r.Get(ctx, req.NamespacedName, rc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Deletion: idempotent external cleanup, then release the finalizer.
	if !rc.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, rc)
	}

	// 3. Ensure the finalizer before anything external exists. Keep going in
	// the same pass: a metadata-only change does not bump the generation, so
	// GenerationChangedPredicate would filter the resulting watch event.
	if !controllerutil.ContainsFinalizer(rc, cleanupFinal) {
		base := rc.DeepCopy()
		controllerutil.AddFinalizer(rc, cleanupFinal)
		if err := r.Patch(ctx, rc, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})); err != nil {
			return ctrl.Result{}, err // conflict: retried with backoff on a fresh copy
		}
	}

	// 4. Converge children with server-side apply.
	if err := r.applyStatefulSet(ctx, rc); err != nil {
		if errors.Is(err, errPermanent) {
			r.Recorder.Eventf(rc, nil, corev1.EventTypeWarning, "InvalidSpec", "Reconcile", "%v", err)
			return ctrl.Result{}, r.setReady(ctx, rc, metav1.ConditionFalse, "InvalidSpec", err.Error())
		}
		return ctrl.Result{}, fmt.Errorf("applying StatefulSet: %w", err)
	}

	// 5. Observe and report.
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, req.NamespacedName, sts); err != nil {
		return ctrl.Result{}, err // NotFound right after apply: stale cache, retry
	}
	rc.Status.ReadyReplicas = sts.Status.ReadyReplicas
	if sts.Status.ReadyReplicas < rc.Spec.Replicas {
		log.V(1).Info("waiting for replicas", "ready", sts.Status.ReadyReplicas, "desired", rc.Spec.Replicas)
		return ctrl.Result{}, r.setReady(ctx, rc, metav1.ConditionFalse, "Progressing",
			fmt.Sprintf("%d/%d replicas ready", sts.Status.ReadyReplicas, rc.Spec.Replicas))
	}
	return ctrl.Result{}, r.setReady(ctx, rc, metav1.ConditionTrue, "Available", "all replicas ready")
}

func (r *RedisClusterReconciler) applyStatefulSet(ctx context.Context, rc *cachev1.RedisCluster) error {
	if rc.Spec.Image == "" {
		return fmt.Errorf("%w: spec.image is empty", errPermanent)
	}
	labels := map[string]string{
		"app.kubernetes.io/name":       "redis",
		"app.kubernetes.io/instance":   rc.Name,
		"app.kubernetes.io/managed-by": "redis-operator",
	}
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
					WithAutomountServiceAccountToken(false).
					WithContainers(corev1ac.Container().
						WithName("redis").
						WithImage(rc.Spec.Image).
						WithSecurityContext(corev1ac.SecurityContext().
							WithAllowPrivilegeEscalation(false).
							WithRunAsNonRoot(true).
							WithCapabilities(corev1ac.Capabilities().WithDrop("ALL")).
							WithSeccompProfile(corev1ac.SeccompProfile().
								WithType(corev1.SeccompProfileTypeRuntimeDefault)))))))
	// Send the complete intent for the fields this manager owns every time:
	// fields left out are removed if this manager was their only owner.
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
	rc.Status.ObservedGeneration = rc.Generation
	if !changed && base.Status.ObservedGeneration == rc.Generation && base.Status.ReadyReplicas == rc.Status.ReadyReplicas {
		return nil // no-op: avoid status write storms
	}
	return r.Status().Patch(ctx, rc, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{}))
}

func (r *RedisClusterReconciler) finalize(ctx context.Context, rc *cachev1.RedisCluster) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(rc, cleanupFinal) {
		return ctrl.Result{}, nil
	}
	if err := r.Backups.DeleteBackups(ctx, rc.Namespace, rc.Name); err != nil {
		if d, ok := apierrors.SuggestsClientDelay(err); ok {
			return ctrl.Result{RequeueAfter: time.Duration(d) * time.Second}, nil
		}
		return ctrl.Result{}, fmt.Errorf("deleting backups: %w", err)
	}
	base := rc.DeepCopy()
	controllerutil.RemoveFinalizer(rc, cleanupFinal)
	return ctrl.Result{}, client.IgnoreNotFound(
		r.Patch(ctx, rc, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})))
}

// SetupWithManager registers the controller and its watches.
func (r *RedisClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1.RedisCluster{}, builder.WithPredicates(predicate.Or(
			predicate.GenerationChangedPredicate{}, // skip our own status writes
			predicate.AnnotationChangedPredicate{},
		))).
		Owns(&appsv1.StatefulSet{}). // no predicate: child status changes matter
		WithOptions(controller.Options{MaxConcurrentReconciles: 4}).
		Complete(r)
}
```

##### Why it looks like this

- **Level-triggered**: every pass re-reads the object and recomputes everything; nothing depends on the event type or on "what changed".
- **Fetch → NotFound → deletion → ensure finalizer → apply children → status**, each step idempotent; every return either succeeds or returns an error for backoff.
- **Server-side apply**[^10] for children with a **stable field manager**: no read-modify-write races; fields owned by others (HPA-managed `replicas`, mutating policies) are kept as long as the controller does not set them; `ForceOwnership` only for fields the controller exclusively owns. Verified with `kubectl apply --server-side`: a second manager setting the same field gets `Apply failed with 1 conflict: conflict with "team-a": .data.mode`, and a field that the sole owner omits in its next apply is **removed**. Never build the apply configuration conditionally ("only set replicas if it changed") — that deletes fields.
- **Apply configurations, not typed structs**: applying a full `appsv1.StatefulSet{}` serialises zero values and claims ownership of defaulted fields. The legacy `r.Patch(ctx, obj, client.Apply)` is deprecated.
- **Owner reference inside the apply configuration** (the SSA equivalent of `controllerutil.SetControllerReference`) so the controller's field manager owns it; `apiVersion` must be `group/version` (`v1` for core types).
- **Status through `/status` with a merge patch and optimistic lock**, only when something changed. `client.MergeFrom` without the lock sends no `resourceVersion` and can overwrite concurrent writers' list entries.
- **Finalizer patches with optimistic lock** instead of `Update` from a possibly stale cached copy.
- **`GenerationChangedPredicate` on `For()` only**: it skips the controller's own status writes. It also drops metadata-only updates (labels, finalizers) — which is why the reconciler continues after adding its finalizer instead of returning. Deletion still passes: setting `deletionTimestamp` on an object with finalizers increments `metadata.generation` (verified on 1.37: 1 → 2). Never put it on `Owns()` when child status drives the parent's status.
- **Events on transitions** via the `events.k8s.io/v1` recorder (`Eventf(regarding, related, type, reason, action, note, args…)`): `action` is required by `events.k8s.io/v1`; never pass user-controlled text as the format string (`"%v", err`).

##### Manager setup

```go
mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
	Scheme: scheme,
	Metrics: metricsserver.Options{
		BindAddress:    ":8443",
		SecureServing:  true,
		FilterProvider: filters.WithAuthenticationAndAuthorization, // authn/authz on /metrics
	},
	HealthProbeBindAddress: ":8081",
	LeaderElection:         true,
	LeaderElectionID:       "redis-operator.cache.example.com",
	Cache: cache.Options{
		DefaultTransform: cache.TransformStripManagedFields(), // less memory
		ByObject: map[client.Object]cache.ByObject{
			&appsv1.StatefulSet{}: {Label: labels.SelectorFromSet(labels.Set{
				"app.kubernetes.io/managed-by": "redis-operator"})}, // cache only what we own
		},
	},
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

- Metrics over HTTPS with authn/authz (the current Kubebuilder scaffold); plain `:8080` metrics or a kube-rbac-proxy sidecar are older patterns.
- `mgr.GetCache().WaitForCacheSync` is not a `healthz.Checker`; wrap it in a `func(*http.Request) error` if readiness must wait for caches.
- `MaxConcurrentReconciles > 1` is safe only when the reconciler holds no unsynchronised shared state; the workqueue never processes the same key concurrently.
- Never block in `Reconcile` (`time.Sleep`, unbounded external calls); pass `ctx` and use deadlines (`controller.Options.ReconciliationTimeout`, v0.22+, or per-call timeouts).

##### Typed webhooks (controller-runtime ≥ v0.23)

```go
type RedisClusterValidator struct{ Reader client.Reader }

var _ admission.Validator[*cachev1.RedisCluster] = &RedisClusterValidator{}

func (v *RedisClusterValidator) ValidateCreate(ctx context.Context, rc *cachev1.RedisCluster) (admission.Warnings, error) {
	return nil, v.checkUniqueSecret(ctx, rc) // needs other objects: CEL cannot do this
}

func (v *RedisClusterValidator) ValidateUpdate(ctx context.Context, _, rc *cachev1.RedisCluster) (admission.Warnings, error) {
	return nil, v.checkUniqueSecret(ctx, rc)
}

func (v *RedisClusterValidator) ValidateDelete(context.Context, *cachev1.RedisCluster) (admission.Warnings, error) {
	return nil, nil
}

func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &cachev1.RedisCluster{}).
		WithValidator(&RedisClusterValidator{Reader: mgr.GetAPIReader()}).
		Complete()
}
```

Before v0.23 the builder was `ctrl.NewWebhookManagedBy(mgr).For(&T{})` with `runtime.Object`-based `CustomValidator`; that shape no longer compiles on v0.23+. Anything that only needs the object itself belongs in CEL, not in a webhook.

---

#### Finalizers, Owner References and Garbage Collection

##### Finalizers

Use finalizers only for cleanup that garbage collection cannot do: external resources (cloud, DNS, databases) and cluster-scoped or cross-namespace objects owned by a namespaced object. Do not add finalizers for in-cluster children that owner references cover.[^31]

- Names are qualified: `cache.example.com/external-cleanup` ✅, `cleanup` ❌. Verified on 1.37: a built-in object with `finalizers: [cleanup]` is rejected (`name is neither a standard finalizer name nor is it fully qualified`); custom resources only get a warning.
- Add the finalizer **before** creating the external resource; remove it only after cleanup succeeded.
- Cleanup must be idempotent: the object is reconciled repeatedly while `deletionTimestamp` is set.
- New finalizers cannot be added once deletion started (verified: `no new finalizers can be added if the object is being deleted`).
- A failing cleanup must surface (condition + Warning Event + returned error) — a stuck finalizer blocks namespace deletion forever. Never strip other controllers' finalizers.
- Do not create objects in a terminating namespace. Since 1.34 (ordered namespace deletion, GA) Pods are deleted before other namespaced resources.

##### Owner references

| Rule | Notes |
|---|---|
| Owner and dependent in the **same namespace** | a namespaced dependent whose same-namespace owner is missing is garbage-collected |
| A namespaced owner **cannot** own a cluster-scoped object | the reference is unresolvable; the dependent is not collected and gets an `OwnerRefInvalidNamespace` event |
| Cluster-scoped owners may own cluster-scoped or namespaced objects | permitted |
| At most one `controller: true` owner | `SetControllerReference` returns `AlreadyOwnedError` otherwise |
| `blockOwnerDeletion: true` | needs `update` on `<owner>/finalizers` when `OwnerReferencesPermissionEnforcement` is enabled |
| `apiVersion` is `group/version` (`v1` for core) | a wrong value (`core/v1`) breaks garbage collection |

| Scenario | Use |
|---|---|
| child in the same namespace | owner reference |
| cluster-scoped child of a namespaced owner | finalizer on the owner (or a cluster-scoped owner) |
| external resource | finalizer |
| cross-namespace cleanup | finalizer |

`Owns(&T{})` only maps children whose **controller** owner reference points at the `For` type. Default deletion propagation for most clients is background; use `foreground` when callers must wait for dependents, `orphan` deliberately (e.g. replacing a StatefulSet).

---

#### Errors, Requeue and Back-Pressure

| Return | Meaning |
|---|---|
| `ctrl.Result{}, nil` | done until the next watch event |
| `ctrl.Result{}, err` | transient failure: requeued with per-item exponential backoff (5 ms doubling, capped at 1000 s, combined with an overall 10 qps / burst 100 limiter — client-go `DefaultTypedControllerRateLimiter`) |
| `ctrl.Result{RequeueAfter: d}, nil` | poll again after `d` (external state that cannot be watched, expiring certificates) |
| `ctrl.Result{}, reconcile.TerminalError(err)` | logged and counted, never retried |
| `ctrl.Result{Requeue: true}, nil` | deprecated since v0.21 — use `RequeueAfter` or return the error |

- **Never return a non-zero `Result` together with an error**: the result is ignored and controller-runtime logs a warning.
- **Permanent errors** (invalid user input CEL could not catch): set a condition, emit one Warning Event, return `nil` (or a `TerminalError`) — do not hot-loop.
- **Do not swallow transient errors** (`log.Error(err, …)` followed by `return ctrl.Result{}, nil` loses the retry).
- `client.IgnoreNotFound(err)` on the initial `Get`; on `IsConflict` return the error and let the next pass re-read — no retry loops inside `Reconcile`.
- **HTTP 429**: API Priority and Fairness and (from 1.37 GA) watch-cache initialisation answer with 429 + `Retry-After`. client-go retries such responses itself; custom HTTP clients and calls to external APIs must honour `Retry-After` (`apierrors.SuggestsClientDelay`).
- A status write that fails after a successful external action must not repeat the external action non-idempotently.

```go
// BAD: hot loop on a permanent error, swallowed transient error
if err := validate(rc); err != nil {
	return ctrl.Result{Requeue: true}, nil
}
if err := r.Update(ctx, sts); err != nil {
	log.Error(err, "update failed") // lost: no retry
}

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

#### Caches, Informers and Leader Election

##### Caches and informers

- Reads through the manager client come from an **informer cache** that lags behind writes. After creating a child, do not create it again because the cache does not show it yet; use deterministic names and treat `AlreadyExists` as success, or use server-side apply. For "create N children" logic, use expectations or read through `mgr.GetAPIReader()` for the critical decision. Never delete "orphans" based on a cache that may be behind your own creates.
- A watched type is cached **cluster-wide** by default: an operator that touches Secrets, ConfigMaps or Pods holds all of them in memory (and needs `list`/`watch` on all of them). Scope caches with `cache.Options.ByObject` (label/field selectors), `DefaultNamespaces`, `builder.OnlyMetadata`, and `DefaultTransform: cache.TransformStripManagedFields()`.
- Map functions in `Watches(…, handler.EnqueueRequestsFromMapFunc(fn))` must use a **field index** (`mgr.GetFieldIndexer().IndexField`) instead of listing everything per event. CRD `selectableFields` (1.32+) allow server-side field selectors for custom resources.
- Do not set a short `SyncPeriod` as a substitute for proper watches; resyncs replay every cached object.
- client-go: use typed workqueues (`workqueue.NewTypedRateLimitingQueueWithConfig[T]`, `DefaultTypedControllerRateLimiter[T]()`), `cache.WaitForNamedCacheSyncWithContext`, and pass the caller's `ctx` instead of `context.TODO()`.
- Hand-written `cache.Store` implementations break when client-go adds interface methods (v0.36 added `Bookmark` and `LastStoreSyncResourceVersion`); embed a real store instead.

##### Leader election

- Enable leader election for any controller Deployment that can run more than one replica; two active reconcilers fight over the same objects.
- Leases live in `coordination.k8s.io/v1`;[^30] the election namespace is auto-detected in-cluster and must be set (`LeaderElectionNamespace`) when running out of cluster.
- `LeaderElectionID` must be unique per controller (domain-qualified).
- `LeaderElectionReleaseOnCancel: true` speeds up fail-over, but is safe only if the process exits right after `Start` returns.
- Webhook servers run on every replica regardless of leadership; only reconcilers are leader-gated (controller-runtime runnables can opt out with `NeedLeaderElection`).
- client-go `resourcelock.MultiLock` is deprecated (1.37); use the Lease lock.

---

#### Observability

##### Structured logging

```go
log := logf.FromContext(ctx).WithValues("generation", obj.Generation)
log.Info("starting reconcile")       // controller-runtime already adds controller, namespace, name, reconcileID
log.Error(err, "failed to apply StatefulSet", "statefulset", sts.Name)

// ❌ Avoid
log.Info(fmt.Sprintf("reconciling %s/%s", req.Namespace, req.Name))
```

Levels: `Info` (V(0)) for lifecycle changes, `V(1)` per-reconcile summaries, `V(2)` decisions, `V(4)` dumps. Never log Secret data or whole objects that may contain it; redact `Authorization` headers.

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

- controller-runtime already exports `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds` and workqueue metrics — do not duplicate them.
- No object names, namespaces or UIDs as label values on unbounded populations (cardinality).
- Metric names follow Prometheus conventions (`_total` for counters, base units such as `_seconds`).

##### Health probes for operators

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

- Emit Events for user-visible transitions (`Normal`) and failures that need attention (`Warning`), not on every reconcile.
- A controller that changes a custom resource's state updates status **and** emits an Event for the transition.
- Missing optional resources in a cleanup pass do not need Events unless users must act.
- Events are best-effort, rate-limited and expire (default TTL one hour); never use them as a state store.

---

#### Testing

##### Pyramid

```
            ┌──────────────────────────┐
            │  e2e (kind, target minor) │  full operator, real kubelet and controllers
            ├──────────────────────────┤
            │  integration (envtest)    │  real etcd + kube-apiserver, no controllers
            ├──────────────────────────┤
            │  unit (fake client)       │  reconcile logic
            └──────────────────────────┘
```

##### Unit tests with the fake client

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
		Spec: cachev1.RedisClusterSpec{Replicas: 3,
			Image: "registry.example.com/redis@sha256:0000000000000000000000000000000000000000000000000000000000000000"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rc).
		WithStatusSubresource(&cachev1.RedisCluster{}). // without it Status().Patch returns NotFound
		Build()
	rec := events.NewFakeRecorder(10)
	r := &RedisClusterReconciler{Client: c, Recorder: rec, Backups: &fakeBackups{}}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "demo"}}

	for i := 0; i < 2; i++ { // the second pass must be a no-op
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("reconcile %d: %v", i, err)
		}
	}
	var sts appsv1.StatefulSet
	if err := c.Get(context.Background(), req.NamespacedName, &sts); err != nil {
		t.Fatalf("StatefulSet not applied: %v", err)
	}
	if *sts.Spec.Replicas != 3 || len(sts.OwnerReferences) != 1 {
		t.Fatalf("unexpected StatefulSet: replicas=%d owners=%d", *sts.Spec.Replicas, len(sts.OwnerReferences))
	}
	var got cachev1.RedisCluster
	if err := c.Get(context.Background(), req.NamespacedName, &got); err != nil {
		t.Fatal(err)
	}
	cond := meta.FindStatusCondition(got.Status.Conditions, conditionReady)
	if cond == nil || cond.Reason != "Progressing" || got.Status.ObservedGeneration != got.Generation {
		t.Fatalf("unexpected status: %+v", got.Status)
	}
}
```

**Fake client limits** — it does not run: CEL or OpenAPI validation, defaulting, admission (PSA, VAP, webhooks), garbage collection, namespace finalization, or any controller (no ReplicaSets or Pods behind a Deployment, no StatefulSet status). It supports server-side apply from v0.22 but does not reproduce every field-manager nuance. A green fake-client test proves reconcile *logic*, not API behaviour. Use `events.NewFakeRecorder` (client-go `tools/events`) for the `events.k8s.io` recorder; `record.NewFakeRecorder` only fits the deprecated `GetEventRecorderFor` type.

##### Integration tests with envtest

envtest starts a real etcd and kube-apiserver (binaries from `setup-envtest use <minor>`, exposed via `KUBEBUILDER_ASSETS`).[^26]

```go
env := &envtest.Environment{
	CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
	ErrorIfCRDPathMissing: true,
}
cfg, err := env.Start()
if err != nil {
	t.Fatalf("starting envtest: %v", err)
}
t.Cleanup(func() { _ = env.Stop() })
```

- Pin `ENVTEST_K8S_VERSION` / `setup-envtest use 1.N.x` to the **cluster minor you ship to**; CEL libraries, defaulting, gated fields and admission behave per version.
- envtest has **no controllers**: no garbage collection (verified: the owned StatefulSet survives deletion of its owner), no Deployment → ReplicaSet → Pod chain, no namespace deletion, and no `default` ServiceAccount in new namespaces. Fake child status where the reconciler depends on it; assert on objects your controller writes.
- Use `Eventually`-style polling with timeouts, never fixed sleeps.
- Test: idempotency (two reconciles, second is a no-op), the deletion path with finalizer removal, status on success and error paths (including `observedGeneration`), and that invalid objects are rejected (`apierrors.IsInvalid`).

##### Manifest checks in CI

- `kubectl apply --dry-run=server -f …` against an API server of the target minor (envtest binaries or kind) catches unknown fields, removed API versions, CEL errors, PSA and admission-policy denials. `--dry-run=client` catches none of these.
- Diff the server dry-run output (`-o yaml`) against the input to detect **silently dropped** gated fields.
- Keep strict field validation (`--validate=strict`, kubectl's default since 1.27); `--validate=false` hides typos and unknown fields.
- Run the same checks for every minor the manifests claim to support.

---

#### Helm and Kustomize

- Declare `kubeVersion:` in `Chart.yaml` for the range you actually test,[^29] and gate version-tied fields with `semverCompare` on `.Capabilities.KubeVersion.Version`:
  ```yaml
  {{- if semverCompare ">=1.36.0-0" .Capabilities.KubeVersion.Version }}
  apiVersion: admissionregistration.k8s.io/v1
  kind: MutatingAdmissionPolicy
  {{- end }}
  ```
  The `-0` suffix makes pre-release and vendor builds (`v1.36.5-eks-…`) match.
- `helm template` without `--kube-version` renders for a **built-in default**, not your cluster: verified Helm v3.22.1 and v4.3.1 assume `v1.37.0`, Helm v4.0.5 assumes `v1.34.0`. Always pass `--kube-version` in CI renders, or use `helm install --dry-run=server`.
- Quote values that land in string fields (`{{ .Values.x | quote }}`); use `required` for values whose absence produces invalid manifests.
- Kustomize: use `configMapGenerator`/`secretGenerator` (hash suffixes roll workloads on change); `kubectl kustomize` embeds a specific Kustomize version (v5.8.1 in kubectl 1.36/1.37), which may differ from the standalone binary in CI.
- Helm-rendered and Kustomize output must pass the same server-side dry-run as raw YAML.

---

#### Production / Review Checklist

##### Version and compatibility

- [ ] Target Kubernetes minor identified (go.mod, envtest, kind image, Chart `kubeVersion`, IaC); every version-tied feature checked against it
- [ ] No removed API versions (`extensions/v1beta1`, `batch/v1beta1`, `policy/v1beta1`, `autoscaling/v2beta2`, `flowcontrol…/v1beta3`, PSP)
- [ ] Beta API versions only with a documented `--runtime-config`/feature-gate prerequisite
- [ ] No alpha fields, no 1.38-only features
- [ ] `k8s.io/*` modules on one minor; controller-runtime / controller-tools match it; Go satisfies the modules' `go` directive
- [ ] Generated files (`zz_generated.*`, CRDs, RBAC) regenerated with the code change
- [ ] Manifests pass `kubectl apply --dry-run=server` on every supported minor; dropped fields checked

##### Workloads

- [ ] Images pinned (digest), no `:latest`, including sidecars and init containers
- [ ] CPU/memory requests on every container, memory limits set
- [ ] Startup/readiness/liveness probes sensible; liveness independent of downstream services
- [ ] Graceful termination: preStop delay, `terminationGracePeriodSeconds`, SIGTERM handling
- [ ] Sidecars as init containers with `restartPolicy: Always` (1.29+)
- [ ] Selectors match template labels; no selector changes on existing objects
- [ ] No `spec.replicas` on HPA-managed workloads
- [ ] PDB for replicated workloads, never blocking drains; topology spread across zones
- [ ] Jobs bounded (`activeDeadlineSeconds`, `backoffLimit`, `ttlSecondsAfterFinished`); CronJobs with `timeZone`, `concurrencyPolicy`, `startingDeadlineSeconds`
- [ ] YAML 1.1 traps quoted (`"yes"`, `"1.10"`, names like `n`)

##### Security

- [ ] Namespaces labelled for Pod Security (`restricted`, `enforce-version` pinned)
- [ ] `runAsNonRoot`, numeric `runAsUser`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, `seccompProfile: RuntimeDefault`, `readOnlyRootFilesystem`
- [ ] No privileged/host namespaces/hostPath without justification
- [ ] RBAC least privilege: no wildcards, no `cluster-admin`, namespaced roles where possible, Secrets by `resourceNames`
- [ ] No `pods/exec`/`attach`/`portforward` (any verb) in read-only roles — `get` allowed exec on ≤ 1.34
- [ ] `automountServiceAccountToken: false` unless needed; one ServiceAccount per workload
- [ ] No plaintext Secrets in Git or in CR specs, env literals, logs, Events or conditions
- [ ] Default-deny NetworkPolicy plus explicit allows including DNS; selector AND/OR semantics correct

##### CRDs, policies and webhooks

- [ ] Status subresource; `observedGeneration`; `metav1.Condition` list-map
- [ ] Every string/list/map bounded; CEL within cost budget; `format()` instead of `string(int)`
- [ ] Immutability: `self == oldSelf` plus a "cannot be removed" parent rule where needed
- [ ] CEL libraries available on the oldest supported cluster
- [ ] No `x-kubernetes-preserve-unknown-fields` except opaque sub-trees
- [ ] Short names do not collide with built-ins (`rc`, `ip`, `ev`, `cm`, …) or installed CRDs
- [ ] One storage version; deprecated versions warn; storage migrated and `storedVersions` cleaned before dropping a version
- [ ] VAP (1.30+) / MAP (1.36+) with bindings, rolled out `Warn`/`Audit` first; MAP sets `reinvocationPolicy`
- [ ] Webhooks: `sideEffects: None`, short timeouts, `matchConditions`, own namespace and `kube-system` excluded, HA, rotated TLS

##### Controllers

- [ ] Level-triggered, idempotent: fetch → NotFound → deletion/finalizer → apply → status
- [ ] Server-side apply with a stable field manager and apply configurations; owner references set
- [ ] Status via `/status`, optimistic locking, only on change
- [ ] Transient errors returned; permanent errors → condition + Event, no hot loop; no `Requeue: true`; never result + error
- [ ] Finalizers qualified, added before external resources, removed after successful cleanup
- [ ] Predicates correct (`GenerationChangedPredicate` on `For` only); caches scoped; field indexes for map functions
- [ ] Leader election on; metrics secured; healthz/readyz wired; `events.k8s.io` RBAC for the v0.23+ recorder
- [ ] Unit tests (fake client with `WithStatusSubresource`) and envtest pinned to the target minor

---

[^1]: Kubernetes Releases (supported minors, latest patches, end-of-life dates). https://kubernetes.io/releases/
[^2]: Kubernetes Patch Releases and release schedule data (`data/releases/schedule.yaml`, `eol.yaml`); current patch per minor: `https://dl.k8s.io/release/stable-1.N.txt`. https://kubernetes.io/releases/patch-releases/
[^3]: Version Skew Policy. https://kubernetes.io/releases/version-skew-policy/
[^4]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^5]: Feature Gates reference (alpha/beta/GA stages per release). https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
[^7]: Common Expression Language in Kubernetes (library availability, cost estimation). https://kubernetes.io/docs/reference/using-api/cel/
[^8]: Extend the Kubernetes API with CustomResourceDefinitions (validation rules, transition rules, ratcheting, selectable fields, pruning). https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
[^9]: Kubernetes API Conventions (spec/status, conditions, naming). https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
[^10]: Server-Side Apply. https://kubernetes.io/docs/reference/using-api/server-side-apply/
[^11]: Validating Admission Policy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
[^12]: Mutating Admission Policy. https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/
[^13]: Dynamic Admission Control (webhooks, failurePolicy, matchConditions, sideEffects). https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/
[^14]: Pod Security Standards and Pod Security Admission. https://kubernetes.io/docs/concepts/security/pod-security-standards/ , https://kubernetes.io/docs/concepts/security/pod-security-admission/
[^15]: Sidecar Containers. https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/
[^16]: Resize CPU and Memory Resources assigned to Containers. https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/
[^17]: User Namespaces. https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
[^18]: Role Based Access Control Good Practices. https://kubernetes.io/docs/concepts/security/rbac-good-practices/
[^19]: CHANGELOG-1.35 (`AuthorizePodWebsocketUpgradeCreatePermission` beta, on); KEP-4006 Transition from SPDY to WebSockets. https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md , https://github.com/kubernetes/enhancements/issues/4006
[^20]: Network Policies. https://kubernetes.io/docs/concepts/services-networking/network-policies/
[^21]: Gateway API (releases, standard channel, ReferenceGrant). https://gateway-api.sigs.k8s.io/
[^22]: Ingress NGINX Retirement: What You Need to Know. https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/
[^23]: controller-runtime releases (compatibility with k8s.io/* minors). https://github.com/kubernetes-sigs/controller-runtime/releases
[^24]: Kubebuilder releases and scaffold versions. https://github.com/kubernetes-sigs/kubebuilder/releases
[^25]: Go module proxy version metadata for k8s.io/api, controller-runtime, controller-tools, kubebuilder, gateway-api. https://proxy.golang.org/
[^26]: Kubebuilder Book — Configuring envtest for integration tests. https://book.kubebuilder.io/reference/envtest
[^27]: Migrate Kubernetes Objects Using Storage Version Migration. https://kubernetes.io/docs/tasks/manage-kubernetes-objects/storage-version-migration/
[^28]: Kubernetes CHANGELOGs 1.29–1.37. https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG
[^29]: Helm — Built-in Objects (`.Capabilities.KubeVersion`) and Chart `kubeVersion`. https://helm.sh/docs/chart_template_guide/builtin_objects/
[^30]: Leases (leader election). https://kubernetes.io/docs/concepts/architecture/leases/
[^31]: Finalizers; Owners and Dependents; Garbage Collection. https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/ , https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/ , https://kubernetes.io/docs/concepts/architecture/garbage-collection/
[^32]: Recommended Labels. https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
[^33]: Configure Liveness, Readiness and Startup Probes. https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/
[^34]: Specifying a Disruption Budget for your Application. https://kubernetes.io/docs/tasks/run-application/configure-pdb/
[^35]: Pod Topology Spread Constraints. https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
[^36]: Good practices for Kubernetes Secrets. https://kubernetes.io/docs/concepts/security/secrets-good-practices/
[^37]: Kubernetes Deprecation Policy. https://kubernetes.io/docs/reference/using-api/deprecation-policy/
