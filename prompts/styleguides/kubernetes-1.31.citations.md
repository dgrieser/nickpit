### Citations: Kubernetes 1.31 — Manifests, Operators & Controllers Review Guide

Sources for `kubernetes-1.31.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.31: Elli — release announcement, 2024-08-13. https://kubernetes.io/blog/2024/08/13/kubernetes-v1-31-release/
    - Kubernetes 1.31 — Manifests, Operators & Controllers Review Guide — quote block: "Kubernetes v1.31.0 "Elli", released **2024-08-13**."
    - What's New in Kubernetes 1.31 — paragraph 1: "…contains 45 enhancements: 11 stable, 22 beta and 12 alpha."
    - What's New in Kubernetes 1.31 › Alpha in 1.31 (off by default — treat as unavailable) — paragraph 1: "`HPAScaleToZero`."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 1: "…longer works — clusters must run an external cloud-controller-manager (`--cloud-provider=external`)."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 2: "The `InTreePlugin*Unregister` gates for AWS/Azure/GCE/OpenStack/vSphere were removed."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 6: "Do not read it in controllers."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 7: "…a new `--fail-cgroupv1` flag/`failCgroupV1` option exists (default `false` in 1.31)."
    - RBAC & Least Privilege › Service Account Configuration — paragraph 1: "…the Pod (`ServiceAccountTokenJTI`, `ServiceAccountTokenPodNodeInfo`, beta since 1.30) for audit correlation."
    - Storage — bullet 1: "…`Retain` on any PV whose data must outlive its claim."
[^2]: Kubernetes release data (data/releases/eol.yaml: 1.31 end of life 2025-11-11, final patch 1.31.14; data/releases/schedule.yaml for 1.34–1.37). https://kubernetes.io/releases/ , https://github.com/kubernetes/website/blob/main/data/releases/eol.yaml
    - Kubernetes 1.31 — Manifests, Operators & Controllers Review Guide — quote block: "`https://dl.k8s.io/release/stable-1.31.txt` reports `v1.31.14`)."
    - Kubernetes 1.31 — Manifests, Operators & Controllers Review Guide — quote block: "…support for 1.31 — that does not change upstream status."
[^3]: CHANGELOG-1.31 (v1.31.0 urgent upgrade notes, API changes, features, deprecations, bug fixes; patch notes v1.31.1–v1.31.14 including CVE-2024-9042, CVE-2025-0426, CVE-2025-5187). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.31.md
    - Kubernetes 1.31 — Manifests, Operators & Controllers Review Guide — quote block: "`https://dl.k8s.io/release/stable-1.31.txt` reports `v1.31.14`)."
    - Version-aware review — bullet 6: "…in the 1.31 line (see "Removed / changed in 1.31")."
    - What's New in Kubernetes 1.31 › Alpha in 1.31 (off by default — treat as unavailable) — paragraph 1: "`HPAScaleToZero`."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 3: "…CSI driver before upgrading if `portworxVolume` sources are in use."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 4: "…`unknown flag`, 1.30 prints a deprecation notice) — use `--delete-emptydir-data`."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 5: "…from kubeadm `extraArgs`, kind configs and component configs before upgrading."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 7: "…a new `--fail-cgroupv1` flag/`failCgroupV1` option exists (default `false` in 1.31)."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 8: "…use Go 1.22/1.23), so upgrading to v1.31.14 breaks such webhooks."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 9: "…CEL optional type is now named `optional_type` in error messages."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 10: "…no longer replace a valid `caBundle` with an invalid one."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 12: "…bump `resourceVersion` for custom resources or objects with empty maps."
    - What's New in Kubernetes 1.31 › Other 1.31 changes that affect reviews — bullet 13: "…now allowed as an SELinux type in the baseline policy."
    - CEL Validation Rules in CRDs › CEL review rules (verified pitfalls) — bullet 8: "Keep using the `__namespace__`-style escaping on 1.31."
    - Server-Side Apply from Controllers — bullet 9: "…detect "something changed" after an apply behaved differently on 1.30."
    - Workload Manifests: Pods › **[GA in 1.31]** AppArmor fields — bullet 3: "…without AppArmor (e.g. some SELinux-based distributions) instead of running unconfined."
    - Workload Manifests: Pods › Native sidecars (beta, on since 1.29) — bullet 1: "…1.31 their finish time counts towards the Job's completion time)."
    - Workload Manifests: Pods › Native sidecars (beta, on since 1.29) — bullet 3: "…Pod that had not finished initialising did not terminate gracefully."
    - Workload Manifests: Controllers (Deployment, StatefulSet, Job, PDB, HPA) › StatefulSets — bullet 2: "…to respect PVCs whose controlling owner is not the StatefulSet."
    - Workload Manifests: Controllers (Deployment, StatefulSet, Job, PDB, HPA) › Jobs — bullet 2: "…controllers that treat *any* new condition as failure need fixing."
    - Workload Manifests: Controllers (Deployment, StatefulSet, Job, PDB, HPA) › Jobs — bullet 4: "…`Failed` while Pods were still running now observe it later."
    - Networking: Services & NetworkPolicy › kube-proxy and load balancers on 1.31 — bullet 2: "…NodePorts to primary node IPs — recommended for hardened clusters."
    - Networking: Services & NetworkPolicy › Ingress — paragraph 1: "…struct (switching `port.number` ↔ `port.name` no longer leaves both set)."
    - Security Hardening › Secrets Management — bullet 4: "…file is rejected and the last valid one stays active)."
    - Security Hardening › Known 1.31 CVEs reviewers should map to patch levels — paragraph 1: "…is a finding; anything on 1.31 at all is end-of-life."
    - Security Hardening › Node-level 1.31 changes reviewers will meet in infrastructure code — bullet 4: "…still accepted); etcd must be ≥ 3.5.11 for kubeadm-managed probes."
    - Security Hardening › Node-level 1.31 changes reviewers will meet in infrastructure code — bullet 5: "…side-channel), and v1.31.13 masks thermal-interrupt info in `/proc` and `/sys`."
    - client-go & apimachinery in v0.31 › Informers and caches — bullet 3: "Requires etcd ≥ 3.5.13 (the apiserver warns on older etcd)."
    - client-go & apimachinery in v0.31 › Informers and caches — bullet 5: "…them in code that already has a `ctx` (contextual logging)."
    - Observability › Structured Logging (logr) — paragraph 1: "…`slog` output from libraries ends up in the same stream."
[^4]: Go module proxy data for k8s.io/client-go v0.31.x (v0.31.0 2024-08-13, v0.31.14 2025-11-12, `go 1.22.0`), sigs.k8s.io/controller-runtime v0.19.x (v0.19.7 2025-03-08, requires k8s.io/* v0.31.0), sigs.k8s.io/controller-tools v0.16.x. https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.7 , https://pkg.go.dev/k8s.io/client-go@v0.31.14
    - Kubernetes 1.31 — Manifests, Operators & Controllers Review Guide — quote block: "…scaffolds controller-runtime v0.19 (v4.2.0: v0.19.0 + controller-tools v0.16.1, envtest 1.31.0)."
[^5]: Kubebuilder scaffold versions (pkg/plugins/golang/v4/scaffolds/init.go at v4.2.0, v4.3.x, v4.4.0). https://github.com/kubernetes-sigs/kubebuilder/releases
    - Kubernetes 1.31 — Manifests, Operators & Controllers Review Guide — quote block: "…scaffolds controller-runtime v0.19 (v4.2.0: v0.19.0 + controller-tools v0.16.1, envtest 1.31.0)."
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
    - Version-aware review — bullet 4: "…beta API versions are not served unless `--runtime-config` enables them)."
[^7]: Deprecated API Migration Guide (flowcontrol.apiserver.k8s.io/v1beta3 removed in v1.32). https://kubernetes.io/docs/reference/using-api/deprecation-guide/
    - API versions and feature gates on a 1.31 cluster › API group versions served by a default v1.31.0 apiserver — paragraph 2: "…use `flowcontrol.apiserver.k8s.io/v1` (GA since 1.29) to survive the next upgrade."
[^8]: Kubernetes v1.32 (2024-12-11), v1.33 (2025-04-23), v1.34 (2025-08-27), v1.35, v1.36 and v1.37 release announcements and CHANGELOGs. https://kubernetes.io/blog/2024/12/11/kubernetes-v1-32-release/ , https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/ , https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG
    - Do not use or recommend (1.32+ features) — paragraph 1: "…every 1.32 boundary below was verified on the v1.32.0 apiserver.)"
    - Do not use or recommend (1.32+ features) — paragraph 5: "…in 1.37), `storagemigration.k8s.io/v1` (1.37), KYAML output (`kubectl get -o kyaml`)."
[^9]: CustomResourceDefinition validation rules (CEL), validation ratcheting, selectable fields; Kubernetes CEL libraries. https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/ , https://kubernetes.io/docs/reference/using-api/cel/
    - CEL Validation Rules in CRDs — paragraph 1: "…first choice for invariants that the OpenAPI schema cannot express."
[^10]: Validating Admission Policy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
    - Admission: CEL Policies and Webhooks › ValidatingAdmissionPolicy (verified on 1.31) — section heading
[^11]: controller-runtime v0.19 package documentation (`webhook/admission`: Validator/Defaulter deprecated; `builder.WebhookBuilder`; `cluster.Cluster.GetEventRecorderFor`; `metrics/filters`). https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.7
    - Admission: CEL Policies and Webhooks › Admission webhooks (controller-runtime v0.19) — paragraph 1: "Use CustomValidator instead") and removed in v0.20."
[^12]: KEP-4006: Transition from SPDY to WebSockets; Kubernetes 1.35 CHANGELOG (`AuthorizePodWebsocketUpgradeCreatePermission`). https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/4006-transition-spdy-to-websockets , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md
    - RBAC & Least Privilege › `pods/exec`, `pods/attach`, `pods/portforward` on 1.31 — paragraph 1: "…Pods on 1.31; this loophole is closed in 1.35 (`AuthorizePodWebsocketUpgradeCreatePermission`)."
[^13]: Go 1.25 release notes — container-aware GOMAXPROCS. https://go.dev/doc/go1.25#container-aware-gomaxprocs
    - Workload Manifests: Pods › A production Deployment for 1.31 (verified) — bullet 5: "…use `automaxprocs`), otherwise the runtime uses the node's CPU count."
[^14]: Volumes — read-only mounts and `recursiveReadOnly`; Kubernetes 1.30 blog "Read-only volume mounts can be finally literally read-only". https://kubernetes.io/docs/concepts/storage/volumes/#read-only-mounts , https://kubernetes.io/blog/2024/04/23/recursive-read-only-mounts/
    - Workload Manifests: Pods › `recursiveReadOnly` mounts (beta, on in 1.31) — paragraph 1: "On 1.30 the field is dropped (alpha/off)."
