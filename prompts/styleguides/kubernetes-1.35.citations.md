### Citations: Kubernetes 1.35 — Manifests, Operators & Controllers Review Guide

Sources for `kubernetes-1.35.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.35: Timbernetes (The World Tree Release) — release announcement, 2025-12-17. https://kubernetes.io/blog/2025/12/17/kubernetes-v1-35-release/
    - Kubernetes 1.35 — Manifests, Operators & Controllers Review Guide — quote block: "Kubernetes v1.35.0 "Timbernetes (The World Tree Release)", released **2025-12-17**."
    - What's New in Kubernetes 1.35 — paragraph 1: "…contains 60 enhancements: 17 stable, 19 beta and 22 alpha."
    - What's New in Kubernetes 1.35 › Alpha in 1.35 (off by default — treat as unavailable) — paragraph 1: "CCM watch-based route reconciliation."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 1: "…preflight check errors on cgroup v1 for kubelet ≥ 1.35."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 2: "…`ipvs` mode is deprecated**; nftables is the recommended Linux backend."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 3: "…supports containerd 1.7; move to containerd ≥ 2.0 before 1.36."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 5: "…decimal integers that clients may compare; apimachinery adds `resourceversion.CompareResourceVersion` (KEP-5504)."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 8: "…and in Service endpoints across kubelet restarts and upgrades (KEP-4781)."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 10: "…such ResourceSlices before upgrading or downgrading between 1.34 and 1.35)."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 13: "…then archived); new ingress work should use the Gateway API."
    - Networking: Services & NetworkPolicy › Ingress — paragraph 1: "…receives fixes), especially `*-snippet` annotations, which inject raw NGINX configuration."
    - Security Hardening › Node-level 1.35 changes reviewers will meet in infrastructure code — bullet 4: "…on 1.35 but not on 1.36; check the `kubelet_cri_losing_support` metric."
[^2]: Kubernetes release schedule and patch releases (data/releases/schedule.yaml: 1.35 release date, patch dates, maintenance mode and end-of-life dates); GitHub release v1.35.9. https://kubernetes.io/releases/patch-releases/ , https://github.com/kubernetes/kubernetes/releases/tag/v1.35.9
    - Kubernetes 1.35 — Manifests, Operators & Controllers Review Guide — quote block: "…scheduled but not yet out — `https://dl.k8s.io/release/stable-1.35.txt` still reports `v1.35.9`)."
    - Kubernetes 1.35 — Manifests, Operators & Controllers Review Guide — quote block: "…2026-08-26, latest v1.37.1). 1.34 reaches end of life on 2026-10-27."
[^3]: CHANGELOG-1.35 (v1.35.0 urgent upgrade notes, API changes, features, deprecations; patch notes v1.35.1–v1.35.9). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md
    - Kubernetes 1.35 — Manifests, Operators & Controllers Review Guide — quote block: "…scheduled but not yet out — `https://dl.k8s.io/release/stable-1.35.txt` still reports `v1.35.9`)."
    - Version-aware review — bullet 5: "…was disabled in v1.35.1 (see "Removed / changed in 1.35")."
    - What's New in Kubernetes 1.35 › Alpha in 1.35 (off by default — treat as unavailable) — paragraph 1: "CCM watch-based route reconciliation."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 1: "…preflight check errors on cgroup v1 for kubelet ≥ 1.35."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 4: "**kubelet `--pod-infra-container-image` flag removed** (startup fails if still set)."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 6: "`resource.k8s.io/v1` is the storage version (downgrade to 1.33 unsupported)."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 7: "…rejected with the same message by a v1.34.1 apiserver (verified)."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 9: "…from apiserver `--feature-gates` before upgrading, because unknown gates fail startup."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 10: "…such ResourceSlices before upgrading or downgrading between 1.34 and 1.35)."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 11: "…v1.35.1; the non-UTF-8 Secret value regression is fixed in v1.35.6."
    - What's New in Kubernetes 1.35 › Other 1.35 changes that affect reviews — bullet 12: "…no longer works without the beta gates it depends on."
    - Workload Manifests: Pods › **[NEW in 1.35 — beta, on]** Environment variables from files (`fileKeyRef`) — paragraph 1: "…secrets: the values become ordinary environment variables visible in `/proc/<pid>/environ`."
    - Workload Manifests: Pods › Other Pod rules — bullet 6: "…— clusters on v1.35.0–v1.35.5 may fail to start such containers."
    - Workload Manifests: Controllers (Deployment, StatefulSet, Job, HPA) › StatefulSets — `maxUnavailable` depends on the patch release — paragraph 1: "…default only ship in 1.37; the 1.35 changelog contains neither."
    - Workload Manifests: Controllers (Deployment, StatefulSet, Job, HPA) › StatefulSets — `maxUnavailable` depends on the patch release — bullet 4: "…→ 1.34 upgrade; 1.35 fixes it behind `StatefulSetSemanticRevisionComparison` (beta, on)."
    - client-go & apimachinery in v0.35 › Informers and caches — bullet 3: "…`managedFields` with a transform to save memory, use ≥ v0.35.0."
    - client-go & apimachinery in v0.35 › **[NEW in 1.35]** API types no longer implement `ProtoMessage()` — paragraph 1: "…build tag `kubernetes_protomessage_one_more_release` in v0.35 and are removed in v0.36."
[^4]: Go module proxy listings for k8s.io/client-go v0.35.x, sigs.k8s.io/controller-runtime v0.23.x (go.mod requires k8s.io/* v0.35.0, go 1.25.0), sigs.k8s.io/controller-tools v0.20.x. https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3 , https://pkg.go.dev/k8s.io/client-go@v0.35.9
    - Kubernetes 1.35 — Manifests, Operators & Controllers Review Guide — quote block: "Go **1.25** is the minimum toolchain for these modules."
[^5]: Kubebuilder v4.11–v4.16 scaffold versions (pkg/plugins/golang/v4/scaffolds/init.go: ControllerRuntimeVersion v0.23.x, ControllerToolsVersion v0.20.x). https://github.com/kubernetes-sigs/kubebuilder/releases
    - Kubernetes 1.35 — Manifests, Operators & Controllers Review Guide — quote block: "Go **1.25** is the minimum toolchain for these modules."
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
    - Version-aware review — bullet 3: "…beta API versions are not served unless `--runtime-config` enables them)."
[^7]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
    - API versions and feature gates on a 1.35 cluster › API group versions served by a default v1.35.0 apiserver — paragraph 3: "…of 1.37); 1.35 only removes the two alpha versions above."
[^8]: Kubernetes v1.36 release announcement (2026-04-22) and CHANGELOG-1.36. https://kubernetes.io/blog/2026/04/22/kubernetes-v1-36-release/ , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.36.md
    - Do not use or recommend (1.36+ features) — paragraph 1: "…verified on the v1.36.2 apiserver where a boundary is involved.)"
    - Workload Manifests: Controllers (Deployment, StatefulSet, Job, HPA) › StatefulSets — `maxUnavailable` depends on the patch release — paragraph 1: "…default only ship in 1.37; the 1.35 changelog contains neither."
[^9]: Kubernetes v1.37 release announcement (2026-08-26) and CHANGELOG-1.37. https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/ , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.37.md
    - Do not use or recommend (1.36+ features) — paragraph 1: "…verified on the v1.36.2 apiserver where a boundary is involved.)"
    - Workload Manifests: Controllers (Deployment, StatefulSet, Job, HPA) › StatefulSets — `maxUnavailable` depends on the patch release — paragraph 1: "…default only ship in 1.37; the 1.35 changelog contains neither."
[^10]: controller-runtime v0.23.0 release notes (events API migration, generic webhooks, priority queue default, subresource apply, FieldOwner/FieldValidation client options). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.23.0
    - Server-Side Apply from Controllers — paragraph 1: "…**apply configurations**; v0.23 adds **subresource apply** (`r.Status().Apply(...)`) and default `client.Options.FieldOwner`/`FieldValidation`."
    - Events (events.k8s.io) — paragraph 1: "…events API and will be removed in a future release"."
    - Admission: CEL Policies and Webhooks › Admission webhooks (controller-runtime v0.23 typed API) — paragraph 1: "`WithCustomValidator`/`WithCustomDefaulter` and the `runtime.Object`-based `admission.CustomValidator`/`CustomDefaulter` interfaces are deprecated."
[^11]: Go 1.25 release notes — container-aware GOMAXPROCS. https://go.dev/doc/go1.25#container-aware-gomaxprocs
    - Workload Manifests: Pods › A production Deployment for 1.35 (verified) — bullet 1: "…an env var; the Go runtime re-reads the limit periodically)."
[^12]: Kubernetes 1.35: In-Place Pod Resize Graduates to Stable (2025-12-19); Resize CPU and Memory Resources assigned to Containers. https://kubernetes.io/blog/2025/12/19/kubernetes-v1-35-in-place-pod-resize-ga/ , https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/
    - Workload Manifests: Pods › **[NEW in 1.35 — GA]** In-place Pod resize — paragraph 2: "…(verified where marked, otherwise from the GA announcement and docs):"
[^13]: KEP-5504: Comparable resource versions; apimachinery `pkg/util/resourceversion`. https://github.com/kubernetes/enhancements/issues/5504 , https://pkg.go.dev/k8s.io/apimachinery/pkg/util/resourceversion
    - client-go & apimachinery in v0.35 › **[NEW in 1.35]** Comparable resource versions — paragraph 1: "…that clients may compare (KEP-5504). apimachinery v0.35 adds a helper:"
[^14]: Kubernetes API reference: CustomResourceDefinition validation rules (CEL), validation ratcheting, selectable fields. https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
    - CEL Validation Rules in CRDs — paragraph 1: "…first choice for invariants that the OpenAPI schema cannot express."
[^15]: Validating Admission Policy and Mutating Admission Policy documentation. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/ , https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/
    - Admission: CEL Policies and Webhooks › ValidatingAdmissionPolicy (verified on 1.35) — section heading
