### Citations: Kubernetes 1.30 — Manifests, Operators & Controllers Review Guide

Sources for `kubernetes-1.30.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.30: Uwubernetes (UwU) — release announcement, 2024-04-17. https://kubernetes.io/blog/2024/04/17/kubernetes-v1-30-release/
    - Kubernetes 1.30 — Manifests, Operators & Controllers Review Guide — quote block: "Kubernetes v1.30.0 "Uwubernetes", released **2024-04-17**."
    - What's New in Kubernetes 1.30 — paragraph 1: "…contains 45 enhancements: 17 stable, 18 beta and 10 alpha."
    - What's New in Kubernetes 1.30 › Graduated to stable (GA) in 1.30 — paragraph 1: "Describe both as beta-on on 1.30."
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 2: "…it in `--enable-admission-plugins` fail to start; use Pod Security Admission."
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 11: "…clients for CRDs must be regenerated with the v0.30 scripts."
    - client-go & apimachinery in v0.30 › Clients and versions — bullet 2: "…1 — a codegen script still calling it is broken."
[^2]: Kubernetes releases: 1.30 release page (End of Life 2025-07-15, latest patch 1.30.14) and website data `data/releases/eol.yaml` / `schedule.yaml` (supported releases, 1.35–1.37 dates). https://kubernetes.io/releases/1.30/ , https://kubernetes.io/releases/
    - Kubernetes 1.30 — Manifests, Operators & Controllers Review Guide — quote block: "…artifacts on `dl.k8s.io` are dated 2025-06-18 UTC, `https://dl.k8s.io/release/stable-1.30.txt` reports `v1.30.14`)."
    - Kubernetes 1.30 — Manifests, Operators & Controllers Review Guide — quote block: "…2026-04-22), **1.35** (EOL 2027-02-28) and **1.34** (maintenance mode, EOL **2026-10-27**)."
[^3]: CHANGELOG-1.30 (v1.30.0 changes by kind; patch notes v1.30.1–v1.30.14 incl. CVE-2024-5321, CVE-2024-9042, CVE-2025-0426). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.30.md
    - Kubernetes 1.30 — Manifests, Operators & Controllers Review Guide — quote block: "…artifacts on `dl.k8s.io` are dated 2025-06-18 UTC, `https://dl.k8s.io/release/stable-1.30.txt` reports `v1.30.14`)."
    - Version-aware review — bullet 5: "…was backported in v1.30.11 (see "Removed / changed in 1.30")."
    - What's New in Kubernetes 1.30 › Alpha in 1.30 (off by default — treat as unavailable) — paragraph 1: "…(`InPlacePodVerticalScaling`), `MaxUnavailableStatefulSet`, `VolumeAttributesClass`, `DynamicResourceAllocation` (`resource.k8s.io/v1alpha2`), `MatchLabelKeysInPodAffinity`, `NFTablesProxyMode`, `ConsistentListFromCache`, `HonorPVReclaimPolicy`, `ProcMountType`."
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 1: "…(verified: 1.30 rejects `isIP(...)` with `undeclared reference`, 1.31 accepts it)."
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 3: "…verified that kubectl 1.30 against a 1.29 apiserver shows neither)."
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 4: "…means "no class / static binding" and stays that way."
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 5: "…0` is now accepted in `flowcontrol.apiserver.k8s.io/v1` and `v1beta3` (`ZeroLimitedNominalConcurrencyShares` GA)."
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 6: "Unmasked` now requires `hostUsers: false`."
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 7: "…NodePorts are no longer reachable on secondary interfaces unless configured."
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 8: "`LimitedSwap` on cgroup v1 makes the kubelet fail."
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 9: "…the node name/UID and a `jti`; audit logs record `authentication.kubernetes.io/credential-id`."
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 10: "…`TokenReview`, `SelfSubjectReview`, `LocalSubjectAccessReview` and `SubjectAccessReview` requests (excluded in all versions)."
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 12: "…with Go 1.22; patch releases from v1.30.11 use Go 1.23."
    - Naming Conventions › Label & Annotation Key Examples — paragraph 2: "…to Pods; selectors and tools should treat them as system-owned."
    - Events — bullet 4: "…value`. client-go v0.30 recommends `NewEventBroadcasterAdapterWithContext` over `NewEventBroadcasterAdapter` for contextual logging."
    - Workload Manifests: Pods › Native sidecar containers (beta, on since 1.29) — bullet 4: "…not finished initialising — run ≥ v1.30.4 for sidecar-heavy workloads."
    - Workload Manifests: Pods › Other Pod rules — bullet 1: "…the runtime supports it (the kubelet rejects the Pod otherwise)."
    - Workload Manifests: Controllers (Deployment, StatefulSet, Job, CronJob, HPA) › StatefulSets — bullet 2: "…v1.30.4 made auto-deletion respect other controlling owners of the PVC."
    - Workload Manifests: Controllers (Deployment, StatefulSet, Job, CronJob, HPA) › Jobs — bullet 3: "…diffs that compare stored objects may flap across that patch."
    - Security Hardening › Node-level 1.30 changes reviewers will meet in infrastructure code — bullet 3: "…and `containerLogMonitorInterval` (concurrent log rotation) are new kubelet configuration fields."
    - Security Hardening › Node-level 1.30 changes reviewers will meet in infrastructure code — bullet 6: "…below v1.30.10 is vulnerable; any 1.30 cluster misses later fixes."
    - client-go & apimachinery in v0.30 › Informers and caches — bullet 2: "…has started (`AddIndexers` no longer fails on a running informer)."
    - client-go & apimachinery in v0.30 › Informers and caches — bullet 3: "…it (the server side `WatchList` gate is alpha/off on 1.30)."
    - client-go & apimachinery in v0.30 › Server-side apply helpers — bullet 1: "…SSA can also clean up old `Update` managers on `/status`."
    - kubectl 1.30 and Manifest Tooling — bullet 1: "…scripts that break behind proxies should use a newer kubectl."
[^4]: Go module proxy data: k8s.io/client-go v0.30.0 (2024-04-17) … v0.30.14 (`go 1.22.0`); sigs.k8s.io/controller-runtime v0.18.0 (2024-04-23) … v0.18.7 (2025-01-06; requires k8s.io/* v0.30.1, `go 1.22.0`); sigs.k8s.io/controller-tools v0.15.0. https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.7 , https://pkg.go.dev/k8s.io/client-go@v0.30.14
    - Kubernetes 1.30 — Manifests, Operators & Controllers Review Guide — quote block: "Go **1.22** is the minimum toolchain for these modules."
[^5]: Kubebuilder v4.0.0 release notes and scaffold versions (`pkg/plugins/golang/v4/scaffolds/init.go`: v4.0.0 → controller-runtime v0.18.2, v4.1.x → v0.18.4, controller-tools v0.15.0, envtest 1.30.0; v4.2.0 → v0.19.0). https://github.com/kubernetes-sigs/kubebuilder/releases/tag/v4.0.0
    - Kubernetes 1.30 — Manifests, Operators & Controllers Review Guide — quote block: "Go **1.22** is the minimum toolchain for these modules."
    - Project Layout & Tooling › Recommended Scaffold (Kubebuilder v4) — paragraph 1: "…the `go/v3` layout (`controllers/`, `apis/`) must be migrated by re-scaffolding."
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
    - Version-aware review — bullet 3: "…beta API versions are not served unless `--runtime-config` enables them)."
[^7]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
    - API versions and feature gates on a 1.30 cluster › API group versions served by a default v1.30 apiserver — paragraph 4: "…was `flowcontrol.apiserver.k8s.io/v1beta2` in 1.29; the next is `flowcontrol.apiserver.k8s.io/v1beta3` in 1.32."
[^8]: Kubernetes v1.31 release announcement (2024-08-13) and CHANGELOG-1.31. https://kubernetes.io/blog/2024/08/13/kubernetes-v1-31-release/ , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.31.md
    - Do not use or recommend (1.31+ features) — paragraph 1: "…v1.31.0 apiserver; later items come from the respective release notes."
[^9]: Kubernetes v1.32 (2024-12-11) and v1.33 (2025-04-23) release announcements. https://kubernetes.io/blog/2024/12/11/kubernetes-v1-32-release/ , https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/
    - Do not use or recommend (1.31+ features) — paragraph 1: "…v1.31.0 apiserver; later items come from the respective release notes."
[^10]: controller-runtime v0.18.0 release notes (k8s.io v1.30, ComponentConfig removal, `admission.Decoder` interface, generic source/event/predicate/handler, `TransformStripManagedFields`, `client.WithFieldOwner`, `NewQueue`). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.18.0
    - Controller / Reconciler Patterns › Manager Setup (verified to compile on v0.18.7) — bullet 4: "**[controller-runtime v0.18]**"
[^11]: controller-tools v0.15.0 / v0.16.0 marker definitions (`XValidation` arguments; `fieldPath`/`reason` added in v0.16.0). https://github.com/kubernetes-sigs/controller-tools/releases
    - CEL Validation Rules in CRDs — paragraph 3: "…`unknown argument "fieldPath"`; those marker arguments arrived in controller-tools v0.16.0."
[^12]: KEP-3488: CEL for Admission Control; KEP-3716: Admission webhook match conditions. https://github.com/kubernetes/enhancements/issues/3488 , https://github.com/kubernetes/enhancements/issues/3716
    - Admission: ValidatingAdmissionPolicy and Webhooks › **[NEW in 1.30 — GA]** ValidatingAdmissionPolicy (verified on 1.30) — section heading
[^13]: KEP-24: AppArmor support; Restrict a Container's Access to Resources with AppArmor. https://github.com/kubernetes/enhancements/issues/24 , https://kubernetes.io/docs/tutorials/security/apparmor/
    - Workload Manifests: Pods › **[NEW in 1.30 — beta, on]** AppArmor as a `securityContext` field — section heading
[^14]: Kubernetes documentation: CustomResourceDefinition validation rules (CEL), validation ratcheting. https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
    - What's New in Kubernetes 1.30 › Other 1.30 changes that affect reviews — bullet 1: "…(verified: 1.30 rejects `isIP(...)` with `undeclared reference`, 1.31 accepts it)."
    - CEL Validation Rules in CRDs — paragraph 1: "…first choice for invariants that the OpenAPI schema cannot express."
[^15]: Common Expression Language in Kubernetes (libraries and their introduction versions); `k8s.io/apiserver/pkg/cel/environment/base.go` at v0.30.14 (`DefaultCompatibilityVersion` 1.29; IP/CIDR introduced at 1.30). https://kubernetes.io/docs/reference/using-api/cel/
    - CEL Validation Rules in CRDs — paragraph 1: "…can use everything introduced up to the 1.29 CEL environment."
[^16]: Validating Admission Policy documentation. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
    - Admission: ValidatingAdmissionPolicy and Webhooks › **[NEW in 1.30 — GA]** ValidatingAdmissionPolicy (verified on 1.30) — section heading
[^17]: KEP-2799: Reduction of Secret-based Service Account Tokens; Managing Service Accounts (legacy token cleanup). https://github.com/kubernetes/enhancements/issues/2799 , https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/
    - RBAC & Least Privilege › Service Account Configuration — paragraph 1: "…still possible but yields a non-expiring credential — flag it."
[^18]: Go 1.25 release notes — container-aware GOMAXPROCS. https://go.dev/doc/go1.25#container-aware-gomaxprocs
    - Workload Manifests: Pods › A production Deployment for 1.30 (verified) — bullet 5: "…divisor: "1"}` (rounded up to whole CPUs) or use `go.uber.org/automaxprocs`."
[^19]: AppArmor tutorial (field and annotation semantics, node requirements). https://kubernetes.io/docs/tutorials/security/apparmor/
    - Workload Manifests: Pods › **[NEW in 1.30 — beta, on]** AppArmor as a `securityContext` field — bullet 7: "…a `Localhost` profile that is not loaded (runtime behaviour, sourced)."
[^20]: Pod Scheduling Readiness (scheduling gates, mutable scheduling directives). https://kubernetes.io/docs/concepts/scheduling-eviction/pod-scheduling-readiness/
    - Workload Manifests: Pods › **[GA in 1.30]** Pod scheduling gates — bullet 1: "…the scheduler (`PodScheduled=False`, reason `SchedulingGated`) until **all** gates are removed."
    - Workload Manifests: Pods › **[GA in 1.30]** Pod scheduling gates — bullet 2: "…— this is how queue controllers (Kueue) place gated Pods."
[^21]: Pod Topology Spread Constraints (`minDomains`, `nodeAffinityPolicy`, `nodeTaintsPolicy`, `matchLabelKeys`). https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
    - Workload Manifests: Pods › **[GA in 1.30]** `minDomains` in topology spread constraints — bullet 2: "…autoscaler that can add domains it only produces Pending Pods."
[^22]: Topology Aware Routing. https://kubernetes.io/docs/concepts/services-networking/topology-aware-routing/
    - Networking: Services & NetworkPolicy › Topology-aware routing instead of `trafficDistribution` — paragraph 2: "It is a hint, not a guarantee."
