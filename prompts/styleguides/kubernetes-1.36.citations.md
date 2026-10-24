### Citations: Kubernetes 1.36: Manifests, Policies and Go Controllers — Complete Review Guide

Sources for `kubernetes-1.36.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.36: ハル (Haru) — release announcement (2026-04-22), including graduations, deprecations and the Ingress NGINX note. https://kubernetes.io/blog/2026/04/22/kubernetes-v1-36-release/
    - Kubernetes 1.36: Manifests, Policies and Go Controllers — Complete Review Guide — quote block: "…treat it as unreleased until it shows up on `dl.k8s.io`."
    - What's New in 1.36 — paragraph 1: "…deprecates Service `.spec.externalIPs` and permanently disables the `gitRepo` volume plugin."
    - What's New in 1.36 › Changed defaults and behaviour in 1.36 — bullet 11: "…annotations need a migration plan (Gateway API or another controller)."
[^2]: Kubernetes releases and patch-release schedule (1.36 maintenance mode 2027-04-28, EOL 2027-06-28; 1.37.0 on 2026-08-26). https://kubernetes.io/releases/ , https://kubernetes.io/releases/patch-releases/ ; schedule data: https://github.com/kubernetes/website/blob/main/data/releases/schedule.yaml ; current patch: https://dl.k8s.io/release/stable-1.36.txt
    - Kubernetes 1.36: Manifests, Policies and Go Controllers — Complete Review Guide — quote block: "…treat it as unreleased until it shows up on `dl.k8s.io`."
    - Kubernetes 1.36: Manifests, Policies and Go Controllers — Complete Review Guide — quote block: "…2026-10-22: 1.37, 1.36, 1.35 (EOL 2027-02-28) and 1.34 (EOL **2026-10-27**)."
[^3]: CHANGELOG-1.36 (v1.36.0 … v1.36.5). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.36.md
    - Kubernetes 1.36: Manifests, Policies and Go Controllers — Complete Review Guide — quote block: "…treat it as unreleased until it shows up on `dl.k8s.io`."
[^4]: controller-runtime releases v0.24.0 / v0.24.1 (k8s.io/* v0.36, Go 1.26) and v0.23.0 (typed webhooks, events API, subresource apply, priority queue). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.24.0 , https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.23.0
    - Kubernetes 1.36: Manifests, Policies and Go Controllers — Complete Review Guide — quote block: "…itself is built with Go 1.26 (1.36.5 uses Go 1.26.8)."
[^5]: Go module metadata for k8s.io/client-go v0.36.x, sigs.k8s.io/controller-runtime v0.24.x and sigs.k8s.io/controller-tools v0.21.0 (go directives, dependency versions, tag dates). https://pkg.go.dev/k8s.io/client-go@v0.36.5 , https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1 , https://pkg.go.dev/sigs.k8s.io/controller-tools@v0.21.0
    - Kubernetes 1.36: Manifests, Policies and Go Controllers — Complete Review Guide — quote block: "…itself is built with Go 1.26 (1.36.5 uses Go 1.26.8)."
[^6]: CHANGELOG-1.36, v1.36.0 "API Change", "Feature", "Deprecation" and "Urgent Upgrade Notes" sections. https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.36.md#v1360
    - What's New in 1.36 › New Alpha in 1.36 (off by default — do not rely on in production manifests) — paragraph 1: "…`PodLevelResourceManagers`, `CRIListStreaming`, `ImageVolumeWithDigest`, `DRAListTypeAttributes`, `DRANodeAllocatableResources`, `DRAResourcePoolStatus`, `DRAWorkloadResourceClaims`, `PodsAPI` (kubelet gRPC)."
    - What's New in 1.36 › Changed defaults and behaviour in 1.36 — bullet 1: "**[NEW in 1.36]**"
    - Do Not Use or Recommend › Deprecated or removed as of 1.36 — flag in review — paragraph 1: "Sources: the 1.36 release notes and the corresponding KEPs."
[^7]: Kubernetes v1.37: Garhwal — release announcement (maxUnavailable for StatefulSets back on by default; kubernetes#137409). https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/
    - What's New in 1.36 › Changed defaults and behaviour in 1.36 — bullet 1: "**[NEW in 1.36]**"
[^8]: Ingress NGINX Retirement: What You Need to Know; Announcing Ingress2Gateway 1.0. https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/ , https://kubernetes.io/blog/2026/03/20/ingress2gateway-1-0-release/
    - What's New in 1.36 › Changed defaults and behaviour in 1.36 — bullet 11: "…annotations need a migration plan (Gateway API or another controller)."
    - Networking: Services, NetworkPolicy, Ingress and Gateway API › Ingress and Gateway API — bullet 2: "Ingress2Gateway 1.0 helps translate them."
[^9]: CHANGELOG-1.36, patch releases v1.36.1–v1.36.5. https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.36.md#v1365
    - What's New in 1.36 › Patch-release fixes that matter (1.36.1–1.36.5) — bullet 4: "…`firstAvailable` alternatives now fall through); fixed a Windows kube-proxy panic."
[^10]: CHANGELOG-1.37 (v1.37.0 API changes, feature graduations, deprecations). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.37.md
    - Do Not Use or Recommend › Features from Kubernetes 1.37 and later (unavailable on a 1.36 cluster) — paragraph 2: "…a startup warning and sits behind the deprecated `KubeProxyIPVS` gate."
[^11]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
    - Do Not Use or Recommend › Deprecated or removed as of 1.36 — flag in review — paragraph 2: "All return `no matches for kind` on 1.36 (verified)."
[^12]: Volumes — `image` volume source. https://kubernetes.io/docs/concepts/storage/volumes/#image ; KEP-4639 OCI VolumeSource. https://github.com/kubernetes/enhancements/issues/4639
    - Workload Manifests › A 1.36 Pod Template That Passes `restricted` — bullet 1: "`pullPolicy` follows the same rules as container images."
[^13]: User Namespaces (stable since v1.36; requirements, limitations, integration with Pod Security Admission). https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
    - Pod Security: securityContext, Pod Security Admission, User Namespaces › User Namespaces (GA in 1.36) **[NEW in 1.36]** — paragraph 1: "NFS volumes cannot be mounted."
    - Pod Security: securityContext, Pod Security Admission, User Namespaces › User Namespaces (GA in 1.36) **[NEW in 1.36]** — paragraph 2: "Restricted still requires the default `procMount`."
[^14]: SELinux Volume Label Changes goes GA (and likely implications in v1.37). https://kubernetes.io/blog/2026/04/22/breaking-changes-in-selinux-volume-labeling/
    - Pod Security: securityContext, Pod Security Admission, User Namespaces › SELinux — bullet 2: "…before upgrading to 1.37, or they can fail to start."
[^15]: Kubelet authentication/authorization — fine-grained authorization (stable since v1.36). https://kubernetes.io/docs/reference/access-authn-authz/kubelet-authn-authz/
    - Pod Security: securityContext, Pod Security Admission, User Namespaces › Fine-Grained Kubelet Authorization (GA in 1.36) — paragraph 2: "…`/healthz` → `nodes/healthz`, `/configz` → `nodes/configz`; everything else needs `nodes/proxy`."
[^16]: Common Expression Language in Kubernetes — library availability table. https://kubernetes.io/docs/reference/using-api/cel/
    - CEL in CRDs and Policies › Library Availability on 1.36 — paragraph 1: "…...)`, `transformList`, `transformMap` (1.33); semver library — `isSemver`, `semver(...)` (1.34)."
[^17]: MutatingAdmissionPolicy. https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/ ; KEP-3962. https://github.com/kubernetes/enhancements/issues/3962
    - Admission Control: ValidatingAdmissionPolicy, MutatingAdmissionPolicy, Webhooks › MutatingAdmissionPolicy (GA in 1.36) **[NEW in 1.36]** — paragraph 1: "…and `MutatingAdmissionPolicyBinding` are served and enabled by default on 1.36."
[^18]: ValidatingAdmissionPolicy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
    - Admission Control: ValidatingAdmissionPolicy, MutatingAdmissionPolicy, Webhooks › ValidatingAdmissionPolicy — paragraph 1: "GA since 1.30."
[^19]: KEP-5707 Deprecate service.spec.externalIPs; KEP-5040 Remove gitRepo volume driver; KEP-4858 IP/CIDR validation; KEP-5440 Mutable Job pod resources. https://github.com/kubernetes/enhancements/issues/5707 , https://github.com/kubernetes/enhancements/issues/5040 , https://github.com/kubernetes/enhancements/issues/4858 , https://github.com/kubernetes/enhancements/issues/5440
    - Do Not Use or Recommend › Deprecated or removed as of 1.36 — flag in review — paragraph 1: "Sources: the 1.36 release notes and the corresponding KEPs."
[^20]: Pod Security Standards. https://kubernetes.io/docs/concepts/security/pod-security-standards/
    - Pod Security: securityContext, Pod Security Admission, User Namespaces › Pod Security Admission — paragraph 1: "Label every namespace explicitly; pin `enforce-version`"
[^21]: CustomResourceDefinition validation rules (transition rules, optionalOldSelf, messageExpression, reason, fieldPath, ratcheting, selectableFields). https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
    - CEL in CRDs and Policies › Transition Rules and Immutability — paragraph 1: "Semantics are documented in the CRD validation-rules reference."
