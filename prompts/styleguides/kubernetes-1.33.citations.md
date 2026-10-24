### Citations: Kubernetes 1.33: Manifests, Policies and Go Controllers — Complete Review Guide

Sources for `kubernetes-1.33.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.33: Octarine — release announcement (2025-04-23), including graduations, deprecations and removals. https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/
    - Kubernetes 1.33: Manifests, Policies and Go Controllers — Complete Review Guide — quote block: "There will be no further 1.33 patches."
    - What's New in 1.33 — paragraph 1: "…are new Alpha features, and 2 were deprecated or withdrawn."
    - What's New in 1.33 › Changed defaults and behaviour in 1.33 — bullet 4: "…driver is planned to be removed for good in v1.39."
[^2]: Kubernetes releases, patch-release schedule and end-of-life data (1.33 EOL 2026-06-28, final patch 1.33.13; 1.37.0 on 2026-08-26). https://kubernetes.io/releases/ ; https://github.com/kubernetes/website/blob/main/data/releases/eol.yaml ; https://github.com/kubernetes/website/blob/main/data/releases/schedule.yaml ; https://dl.k8s.io/release/stable-1.33.txt
    - Kubernetes 1.33: Manifests, Policies and Go Controllers — Complete Review Guide — quote block: "There will be no further 1.33 patches."
    - Kubernetes 1.33: Manifests, Policies and Go Controllers — Complete Review Guide — quote block: "…→ 1.34 → 1.35 (…) one minor at a time."
[^3]: CHANGELOG-1.33 (v1.33.0 … v1.33.13). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.33.md
    - Kubernetes 1.33: Manifests, Policies and Go Controllers — Complete Review Guide — quote block: "There will be no further 1.33 patches."
    - What's New in 1.33 › Patch-release fixes that matter (1.33.1–1.33.13) — bullet 11: "…built with Go 1.25.11; endpoint-controller panic fix for pre-dual-stack Services."
[^4]: controller-runtime v0.21.0 release notes (k8s.io/* v0.33, Go 1.24, client-side rate limiting off by default, `Result.Requeue` deprecated, priority-queue changes). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.21.0
    - Kubernetes 1.33: Manifests, Policies and Go Controllers — Complete Review Guide — quote block: "Go 1.25.11)."
[^5]: Go module metadata for k8s.io/client-go v0.33.13, sigs.k8s.io/controller-runtime v0.21.0 and sigs.k8s.io/controller-tools v0.18.0 (go directives, dependency versions, tag dates). https://pkg.go.dev/k8s.io/client-go@v0.33.13 , https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0 , https://pkg.go.dev/sigs.k8s.io/controller-tools@v0.18.0
    - Kubernetes 1.33: Manifests, Policies and Go Controllers — Complete Review Guide — quote block: "Go 1.25.11)."
[^6]: CHANGELOG-1.33, v1.33.0 "Urgent Upgrade Notes", "Deprecation", "API Change", "Feature" and "Other (Cleanup or Flake)" sections. https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.33.md#v1330
    - What's New in 1.33 › New Alpha in 1.33 (off by default — fields are dropped or values rejected on a stock cluster) — paragraph 1: "…`MutableCSINodeAllocatableCount`, `StorageCapacityScoring`, `ReduceDefaultCrashLoopBackOffDecay`, `KubeletPSI`, `ListFromCacheSnapshot`, kubectl `kuberc` (opt-in with `KUBECTL_KUBERC=true`)."
    - What's New in 1.33 › Changed defaults and behaviour in 1.33 — bullet 15: "`admissionregistration.k8s.io/v1alpha1` ValidatingAdmissionPolicy serving was removed."
[^7]: Kubernetes v1.33: Continuing the transition from Endpoints to EndpointSlices; KEP-4974 Deprecate v1.Endpoints. https://kubernetes.io/blog/2025/04/24/endpoints-deprecation/ , https://github.com/kubernetes/enhancements/issues/4974
    - What's New in 1.33 › Changed defaults and behaviour in 1.33 — bullet 5: "…API will not be removed, but code should read EndpointSlices."
    - Networking: Services, ServiceCIDR, NetworkPolicy, Ingress › Services — bullet 3: "…a label that says so; hand-written Endpoints should become EndpointSlices."
[^8]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
    - Do Not Use or Recommend › Deprecated or removed as of 1.33 — flag in review — paragraph 1: "None of them is served by 1.33."
[^9]: controller-runtime v0.21.0 source: `pkg/client/config/config.go` (`GetConfigWithContext` sets `QPS = -1`), `pkg/reconcile/reconcile.go` (`Requeue` deprecation), `pkg/config/controller.go` (`UsePriorityQueue`, off by default), `pkg/client/fake/client.go` (apply patches unsupported). https://github.com/kubernetes-sigs/controller-runtime/tree/v0.21.0
    - Project Layout, Toolchain and Dependency Pinning › Toolchain Requirements — bullet 9: "…20 QPS / 30 burst throttle must set `cfg.QPS`/`cfg.Burst` explicitly."
[^10]: Sidecar Containers (stable since v1.33); KEP-753. https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/ , https://github.com/kubernetes/enhancements/issues/753
    - Workload Manifests › A 1.33 Pod That Passes `restricted` — bullet 1: "Named ports of sidecars are reachable through Services since 1.33."
[^11]: Kubernetes v1.33: In-Place Pod Resize Graduated to Beta; Resize CPU and Memory Resources assigned to Containers (v1.33 documentation, "Limitations"). https://kubernetes.io/blog/2025/05/16/kubernetes-v1-33-in-place-pod-resize-beta/ , https://v1-33.docs.kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/
    - Workload Manifests › Resource Requests, Limits and In-Place Resize **[NEW in 1.33: beta, on]** — bullet 3: "…and `status.containerStatuses[*].resources`/`allocatedResources`, not `spec`, to learn what is actually applied."
[^12]: Pod Security Admission and Pod Security Standards. https://kubernetes.io/docs/concepts/security/pod-security-admission/ , https://kubernetes.io/docs/concepts/security/pod-security-standards/
    - Pod Security: securityContext, Pod Security Admission, User Namespaces › Pod Security Admission — paragraph 1: "Label every namespace explicitly and pin the policy versions"
[^13]: Kubernetes v1.33: User Namespaces enabled by default; User Namespaces documentation (requirements, limitations). https://kubernetes.io/blog/2025/04/25/userns-enabled-by-default/ , https://v1-33.docs.kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
    - Pod Security: securityContext, Pod Security Admission, User Namespaces › User Namespaces (beta, on by default since 1.33) **[NEW in 1.33]** — paragraph 1: "…with user namespaces may not set `hostNetwork`, `hostIPC` or `hostPID`."
[^14]: Configure a Security Context — SELinux label change policy (`seLinuxChangePolicy`, `SELinuxMount`, `SELinuxWarningController`). https://v1-33.docs.kubernetes.io/docs/tasks/configure-pod-container/security-context/
    - Pod Security: securityContext, Pod Security Admission, User Namespaces › SELinux — bullet 2: "…and later releases turn mount-option relabeling on for all volumes."
[^15]: Kubelet authentication/authorization — fine-grained authorization (KEP-2862). https://kubernetes.io/docs/reference/access-authn-authz/kubelet-authn-authz/
    - Pod Security: securityContext, Pod Security Admission, User Namespaces › Fine-Grained Kubelet Authorization (beta, on since 1.33) — paragraph 2: "…fine-grained check is denied; every other path still requires `nodes/proxy`."
[^16]: Ingress NGINX Retirement: What You Need to Know. https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/
    - Networking: Services, ServiceCIDR, NetworkPolicy, Ingress › Ingress and Gateway API — bullet 2: "…a migration plan to Gateway API or a maintained controller."
[^17]: ValidatingAdmissionPolicy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
    - Admission Control: ValidatingAdmissionPolicy and Webhooks › ValidatingAdmissionPolicy — paragraph 1: "GA since 1.30."
[^18]: Common Expression Language in Kubernetes — libraries and availability. https://kubernetes.io/docs/reference/using-api/cel/
    - CEL in CRDs and Policies › Library Availability on 1.33 — paragraph 2: "…the estimated cost by `maxItems`/`maxProperties`, so keep those bounds small."
[^19]: CustomResourceDefinition validation rules (transition rules, optionalOldSelf, messageExpression, reason, fieldPath, ratcheting, selectableFields); KEP-4008 CRD validation ratcheting. https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/ , https://github.com/kubernetes/enhancements/issues/4008
    - CEL in CRDs and Policies › Validation Ratcheting (GA in 1.33) **[NEW in 1.33]** — paragraph 1: "…new objects and changed fields still fail, which breaks clients."
