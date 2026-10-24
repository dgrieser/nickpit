### Citations: Kubernetes 1.32 — Manifests, Operators & Controllers Review Guide

Sources for `kubernetes-1.32.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.32: Penelope — release announcement, 2024-12-11. https://kubernetes.io/blog/2024/12/11/kubernetes-v1-32-release/
    - Kubernetes 1.32 — Manifests, Operators & Controllers Review Guide — quote block: "Kubernetes v1.32.0 "Penelope", released **2024-12-11**."
    - What's New in Kubernetes 1.32 — paragraph 1: "…contains 44 enhancements: 13 stable, 12 beta and 19 alpha."
    - What's New in Kubernetes 1.32 › Alpha in 1.32 (off by default — treat as unavailable) — paragraph 1: "…**alpha** in 1.32 even though the `pods/resize` subresource was added."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 1: "…fails with `no matches for kind "FlowSchema" in version "flowcontrol.apiserver.k8s.io/v1beta3"`."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 2: "…`DRAControlPlaneController` gate and `PodSchedulingContext` are gone; only structured parameters remain."
[^2]: Kubernetes release data (data/releases/schedule.yaml and eol.yaml: release dates, end-of-life dates, final patch 1.32.13); stable-1.32.txt; GitHub release v1.32.13. https://kubernetes.io/releases/ , https://dl.k8s.io/release/stable-1.32.txt , https://github.com/kubernetes/kubernetes/releases/tag/v1.32.13
    - Kubernetes 1.32 — Manifests, Operators & Controllers Review Guide — quote block: "`https://dl.k8s.io/release/stable-1.32.txt` reports `v1.32.13`)."
    - Kubernetes 1.32 — Manifests, Operators & Controllers Review Guide — quote block: "…life on 2026-10-27, and 1.33 ended on 2026-06-28 (final v1.33.13)."
[^3]: CHANGELOG-1.32 (v1.32.0 API changes, features, deprecations, cleanup; patch notes v1.32.1–v1.32.13 including CVE-2024-9042, CVE-2025-0426, CVE-2025-4563, CVE-2025-5187). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.32.md
    - Kubernetes 1.32 — Manifests, Operators & Controllers Review Guide — quote block: "`https://dl.k8s.io/release/stable-1.32.txt` reports `v1.32.13`)."
    - Version-aware review — bullet 5: "…v1.32.2, v1.32.6 and v1.32.8 (see "Removed / changed in 1.32")."
    - What's New in Kubernetes 1.32 › Alpha in 1.32 (off by default — treat as unavailable) — paragraph 1: "…**alpha** in 1.32 even though the `pods/resize` subresource was added."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 3: "…to isolate access to mounted secrets` (no warning on 1.31)."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 4: "…rejected with `forbidden sysctls (net.ipv4.tcp_rmem)` in one pinned to `restricted:v1.31`."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 5: "…and fail on 1.31 (`undefined field 'namespace'` / `undeclared reference`)."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 7: "CSI volume limits come from `CSINode`."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 8: "…40 s to 50 s (Nodes are marked `NotReady` later)."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 9: "…pull back-off messages now include the pull error in `state.waiting.message`."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 10: "Do not read it."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 11: "…typos that were ignored on 1.31 now stop the component."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 12: "…warning as a bug: one source silently shadows the other."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 13: "…(alpha `RemoteRequestHeaderUID`) for front proxies that pass a user UID."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 14: "…v1.7.2 must be upgraded on nodes that use CDI devices."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 15: "…`MinDomainsInPodTopologySpread`, `CloudDualStackNodeIPs`, `LegacyServiceAccountTokenCleanUp`, `NewVolumeManagerReconstruction`, `NodeOutOfServiceVolumeDetach`, `StableLoadBalancerNodeSet`, `ZeroLimitedNominalConcurrencyShares`, `PodHostIPs`, `ComponentSLIs`, `AllowServiceLBStatusOnNonLB`."
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 16: "…<old> to <new>`; tooling that parses the old wording breaks."
    - What's New in Kubernetes 1.32 › Release-note discrepancy found while testing — paragraph 1: "…by default** in 1.32 (usable; seconds must be ≥ 1)."
    - CEL Validation Rules in CRDs › CEL library availability on 1.32 (verified by installing one CRD per function) — paragraph 2: "Rules relying on `containsCIDR` give wrong answers on v1.32.0–v1.32.3."
    - Owner References & Garbage Collection › Rules — paragraph 1: "…and get it garbage-collected (CVE-2025-5187, fixed by NodeRestriction in v1.32.8)."
    - Admission: CEL Policies and Webhooks › ValidatingAdmissionPolicy (verified on 1.32) — bullet 7: "…found" for objects in newly created namespaces; fixed in v1.32.11."
    - RBAC, Authentication & Least Privilege › `pods/exec`, `pods/attach`, `pods/portforward` on 1.32 — bullet 2: "…exec/attach/port-forward; tooling on older patch levels may see dropped sessions."
    - Workload Manifests: Pods › Pod lifecycle and preStop — bullet 2: "…that broke starting Pods with `postStart` hooks, fixed in v1.32.3."
    - Workload Manifests: Pods › In-place Pod resize — alpha on 1.32 — paragraph 1: "…integer CPU on static CPU/memory-manager nodes are blocked (`InPlacePodVerticalScalingExclusiveCPUs` off)."
    - Workload Manifests: Pods › Other Pod rules — bullet 7: "…survives an OOM at the expense of the main container."
    - Workload Manifests: Controllers (Deployment, StatefulSet, Job, CronJob, HPA) › StatefulSets — **[GA in 1.32]** PVC retention policy — bullet 6: "…fixed a kube-controller-manager panic for StatefulSets with a negative `revisionHistoryLimit`."
    - Workload Manifests: Controllers (Deployment, StatefulSet, Job, CronJob, HPA) › Jobs — bullet 4: "…updates failing with `startTime cannot be removed for unsuspended job`."
    - Security Hardening › Node and cluster security issues fixed in 1.32 patch releases — paragraph 1: "…flagged; any 1.32 pin should come with an upgrade plan."
    - client-go & apimachinery in v0.32 › Clients and versions — bullet 4: "…silently rounded float for large `int64` values (fixed in v0.32)."
    - Observability › Control-plane observability on 1.32 — paragraph 1: "…switched the kube-scheduler static Pod probes to `/livez` and `/readyz`."
[^4]: Go module proxy metadata for k8s.io/client-go v0.32.13 (2026-02-27, go 1.23.0), sigs.k8s.io/controller-runtime v0.20.0/v0.20.4 (go.mod requires k8s.io/* v0.32.0/v0.32.1), sigs.k8s.io/controller-tools v0.17.0–v0.17.3. https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4 , https://pkg.go.dev/k8s.io/client-go@v0.32.13
    - Kubernetes 1.32 — Manifests, Operators & Controllers Review Guide — quote block: "Go **1.23** is the minimum toolchain for these modules."
[^5]: Kubebuilder v4.5.0/v4.5.2 scaffold versions (pkg/plugins/golang/v4/scaffolds/init.go: ControllerRuntimeVersion v0.20.0/v0.20.4, ControllerToolsVersion v0.17.1/v0.17.2). https://github.com/kubernetes-sigs/kubebuilder/releases
    - Kubernetes 1.32 — Manifests, Operators & Controllers Review Guide — quote block: "Go **1.23** is the minimum toolchain for these modules."
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
    - Version-aware review — bullet 3: "…beta API versions are not served unless `--runtime-config` enables them)."
[^7]: Deprecated API Migration Guide — flowcontrol resources v1.32. https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132
    - What's New in Kubernetes 1.32 › Other 1.32 changes that affect reviews — bullet 1: "…fails with `no matches for kind "FlowSchema" in version "flowcontrol.apiserver.k8s.io/v1beta3"`."
[^8]: Kubernetes v1.34 release announcement (2025-08-27), list of graduations to stable. https://kubernetes.io/blog/2025/08/27/kubernetes-v1-34-release/
    - What's New in Kubernetes 1.32 › Release-note discrepancy found while testing — paragraph 1: "…by default** in 1.32 (usable; seconds must be ≥ 1)."
    - Do not use or recommend (1.33+ features) — paragraph 1: "…boundaries marked "verified" were tested on v1.32.0 and v1.33.0 apiservers."
[^9]: Kubernetes v1.33 release announcement (2025-04-23) and CHANGELOG-1.33. https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/ , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.33.md
    - Do not use or recommend (1.33+ features) — paragraph 1: "…boundaries marked "verified" were tested on v1.32.0 and v1.33.0 apiservers."
[^10]: Kubernetes v1.35, v1.36 and v1.37 release announcements and changelogs. https://kubernetes.io/blog/2025/12/17/kubernetes-v1-35-release/ , https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG
    - Do not use or recommend (1.33+ features) — paragraph 1: "…boundaries marked "verified" were tested on v1.32.0 and v1.33.0 apiservers."
[^11]: Kubernetes documentation: CustomResourceDefinition validation rules (CEL), validation ratcheting, selectable fields; CEL in Kubernetes (libraries). https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/ , https://kubernetes.io/docs/reference/using-api/cel/
    - CEL Validation Rules in CRDs — paragraph 1: "…first choice for invariants that the OpenAPI schema cannot express."
[^12]: controller-runtime v0.20.0 release notes (k8s.io v1.32 libraries, removal of Defaulter/Validator, CustomDefaulter unknown fields, warning deduplication, priority queue, EnableWatchBookmarks, HasOwnerReference). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.20.0
    - Watches & Predicates › Fan-Out Watches (Many-to-One) — paragraph 2: "`controllerutil.HasOwnerReference` is also new in v0.20."
    - Admission: CEL Policies and Webhooks › Admission webhooks (controller-runtime v0.20 API) — paragraph 1: "…`admission.Validator` / `admission.Defaulter` interfaces (methods on the API type itself)."
    - Observability › Structured Logging (logr) — paragraph 1: "…a deprecated API version) rather than raising the log level."
[^13]: Validating Admission Policy and Mutating Admission Policy documentation. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/ , https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/
    - Admission: CEL Policies and Webhooks › ValidatingAdmissionPolicy (verified on 1.32) — section heading
[^14]: KEP-4006: Transition from SPDY to WebSockets (kubectl exec/attach/port-forward), and the 1.35 `AuthorizePodWebsocketUpgradeCreatePermission` change. https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/4006-transition-spdy-to-websockets
    - RBAC, Authentication & Least Privilege › `pods/exec`, `pods/attach`, `pods/portforward` on 1.32 — paragraph 1: "…stops working on 1.35, where `AuthorizePodWebsocketUpgradeCreatePermission` requires `create` for both."
[^15]: Go 1.25 release notes — container-aware GOMAXPROCS. https://go.dev/doc/go1.25#container-aware-gomaxprocs
    - Workload Manifests: Pods › A production Deployment for 1.32 (verified) — bullet 6: "…"1"}` or use `go.uber.org/automaxprocs` (Go 1.25 made the runtime container-aware)."
[^16]: Ingress NGINX Retirement: What You Need to Know (Kubernetes blog, 2025-11-11). https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/
    - Networking: Services & NetworkPolicy › Ingress and Gateway API — paragraph 1: "…2026); flag new `nginx.ingress.kubernetes.io/*-snippet` annotations, which inject raw NGINX configuration."
