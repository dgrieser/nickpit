### Citations: Kubernetes 1.33: Manifests, Policies and Go Controllers — Complete Review Guide

Footnote sources for `kubernetes-1.33.md`. Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.33: Octarine — release announcement (2025-04-23), including graduations, deprecations and removals. https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/
[^2]: Kubernetes releases, patch-release schedule and end-of-life data (1.33 EOL 2026-06-28, final patch 1.33.13; 1.37.0 on 2026-08-26). https://kubernetes.io/releases/ ; https://github.com/kubernetes/website/blob/main/data/releases/eol.yaml ; https://github.com/kubernetes/website/blob/main/data/releases/schedule.yaml ; https://dl.k8s.io/release/stable-1.33.txt
[^3]: CHANGELOG-1.33 (v1.33.0 … v1.33.13). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.33.md
[^4]: controller-runtime v0.21.0 release notes (k8s.io/* v0.33, Go 1.24, client-side rate limiting off by default, `Result.Requeue` deprecated, priority-queue changes). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.21.0
[^5]: Go module metadata for k8s.io/client-go v0.33.13, sigs.k8s.io/controller-runtime v0.21.0 and sigs.k8s.io/controller-tools v0.18.0 (go directives, dependency versions, tag dates). https://pkg.go.dev/k8s.io/client-go@v0.33.13 , https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0 , https://pkg.go.dev/sigs.k8s.io/controller-tools@v0.18.0
[^6]: CHANGELOG-1.33, v1.33.0 "Urgent Upgrade Notes", "Deprecation", "API Change", "Feature" and "Other (Cleanup or Flake)" sections. https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.33.md#v1330
[^7]: Kubernetes v1.33: Continuing the transition from Endpoints to EndpointSlices; KEP-4974 Deprecate v1.Endpoints. https://kubernetes.io/blog/2025/04/24/endpoints-deprecation/ , https://github.com/kubernetes/enhancements/issues/4974
[^8]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^9]: controller-runtime v0.21.0 source: `pkg/client/config/config.go` (`GetConfigWithContext` sets `QPS = -1`), `pkg/reconcile/reconcile.go` (`Requeue` deprecation), `pkg/config/controller.go` (`UsePriorityQueue`, off by default), `pkg/client/fake/client.go` (apply patches unsupported). https://github.com/kubernetes-sigs/controller-runtime/tree/v0.21.0
[^10]: Sidecar Containers (stable since v1.33); KEP-753. https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/ , https://github.com/kubernetes/enhancements/issues/753
[^11]: Kubernetes v1.33: In-Place Pod Resize Graduated to Beta; Resize CPU and Memory Resources assigned to Containers (v1.33 documentation, "Limitations"). https://kubernetes.io/blog/2025/05/16/kubernetes-v1-33-in-place-pod-resize-beta/ , https://v1-33.docs.kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/
[^12]: Pod Security Admission and Pod Security Standards. https://kubernetes.io/docs/concepts/security/pod-security-admission/ , https://kubernetes.io/docs/concepts/security/pod-security-standards/
[^13]: Kubernetes v1.33: User Namespaces enabled by default; User Namespaces documentation (requirements, limitations). https://kubernetes.io/blog/2025/04/25/userns-enabled-by-default/ , https://v1-33.docs.kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
[^14]: Configure a Security Context — SELinux label change policy (`seLinuxChangePolicy`, `SELinuxMount`, `SELinuxWarningController`). https://v1-33.docs.kubernetes.io/docs/tasks/configure-pod-container/security-context/
[^15]: Kubelet authentication/authorization — fine-grained authorization (KEP-2862). https://kubernetes.io/docs/reference/access-authn-authz/kubelet-authn-authz/
[^16]: Ingress NGINX Retirement: What You Need to Know. https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/
[^17]: ValidatingAdmissionPolicy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
[^18]: Common Expression Language in Kubernetes — libraries and availability. https://kubernetes.io/docs/reference/using-api/cel/
[^19]: CustomResourceDefinition validation rules (transition rules, optionalOldSelf, messageExpression, reason, fieldPath, ratcheting, selectableFields); KEP-4008 CRD validation ratcheting. https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/ , https://github.com/kubernetes/enhancements/issues/4008
