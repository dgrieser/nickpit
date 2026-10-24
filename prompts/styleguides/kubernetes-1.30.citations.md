### Citations: Kubernetes 1.30 — Manifests, Operators & Controllers Review Guide

Footnote sources for `kubernetes-1.30.md`. Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.30: Uwubernetes (UwU) — release announcement, 2024-04-17. https://kubernetes.io/blog/2024/04/17/kubernetes-v1-30-release/
[^2]: Kubernetes releases: 1.30 release page (End of Life 2025-07-15, latest patch 1.30.14) and website data `data/releases/eol.yaml` / `schedule.yaml` (supported releases, 1.35–1.37 dates). https://kubernetes.io/releases/1.30/ , https://kubernetes.io/releases/
[^3]: CHANGELOG-1.30 (v1.30.0 changes by kind; patch notes v1.30.1–v1.30.14 incl. CVE-2024-5321, CVE-2024-9042, CVE-2025-0426). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.30.md
[^4]: Go module proxy data: k8s.io/client-go v0.30.0 (2024-04-17) … v0.30.14 (`go 1.22.0`); sigs.k8s.io/controller-runtime v0.18.0 (2024-04-23) … v0.18.7 (2025-01-06; requires k8s.io/* v0.30.1, `go 1.22.0`); sigs.k8s.io/controller-tools v0.15.0. https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.7 , https://pkg.go.dev/k8s.io/client-go@v0.30.14
[^5]: Kubebuilder v4.0.0 release notes and scaffold versions (`pkg/plugins/golang/v4/scaffolds/init.go`: v4.0.0 → controller-runtime v0.18.2, v4.1.x → v0.18.4, controller-tools v0.15.0, envtest 1.30.0; v4.2.0 → v0.19.0). https://github.com/kubernetes-sigs/kubebuilder/releases/tag/v4.0.0
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
[^7]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^8]: Kubernetes v1.31 release announcement (2024-08-13) and CHANGELOG-1.31. https://kubernetes.io/blog/2024/08/13/kubernetes-v1-31-release/ , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.31.md
[^9]: Kubernetes v1.32 (2024-12-11) and v1.33 (2025-04-23) release announcements. https://kubernetes.io/blog/2024/12/11/kubernetes-v1-32-release/ , https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/
[^10]: controller-runtime v0.18.0 release notes (k8s.io v1.30, ComponentConfig removal, `admission.Decoder` interface, generic source/event/predicate/handler, `TransformStripManagedFields`, `client.WithFieldOwner`, `NewQueue`). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.18.0
[^11]: controller-tools v0.15.0 / v0.16.0 marker definitions (`XValidation` arguments; `fieldPath`/`reason` added in v0.16.0). https://github.com/kubernetes-sigs/controller-tools/releases
[^12]: KEP-3488: CEL for Admission Control; KEP-3716: Admission webhook match conditions. https://github.com/kubernetes/enhancements/issues/3488 , https://github.com/kubernetes/enhancements/issues/3716
[^13]: KEP-24: AppArmor support; Restrict a Container's Access to Resources with AppArmor. https://github.com/kubernetes/enhancements/issues/24 , https://kubernetes.io/docs/tutorials/security/apparmor/
[^14]: Kubernetes documentation: CustomResourceDefinition validation rules (CEL), validation ratcheting. https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
[^15]: Common Expression Language in Kubernetes (libraries and their introduction versions); `k8s.io/apiserver/pkg/cel/environment/base.go` at v0.30.14 (`DefaultCompatibilityVersion` 1.29; IP/CIDR introduced at 1.30). https://kubernetes.io/docs/reference/using-api/cel/
[^16]: Validating Admission Policy documentation. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
[^17]: KEP-2799: Reduction of Secret-based Service Account Tokens; Managing Service Accounts (legacy token cleanup). https://github.com/kubernetes/enhancements/issues/2799 , https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/
[^18]: Go 1.25 release notes — container-aware GOMAXPROCS. https://go.dev/doc/go1.25#container-aware-gomaxprocs
[^19]: AppArmor tutorial (field and annotation semantics, node requirements). https://kubernetes.io/docs/tutorials/security/apparmor/
[^20]: Pod Scheduling Readiness (scheduling gates, mutable scheduling directives). https://kubernetes.io/docs/concepts/scheduling-eviction/pod-scheduling-readiness/
[^21]: Pod Topology Spread Constraints (`minDomains`, `nodeAffinityPolicy`, `nodeTaintsPolicy`, `matchLabelKeys`). https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
[^22]: Topology Aware Routing. https://kubernetes.io/docs/concepts/services-networking/topology-aware-routing/
