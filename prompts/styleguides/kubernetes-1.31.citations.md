### Citations: Kubernetes 1.31 — Manifests, Operators & Controllers Review Guide

Footnote sources for `kubernetes-1.31.md`. Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.31: Elli — release announcement, 2024-08-13. https://kubernetes.io/blog/2024/08/13/kubernetes-v1-31-release/
[^2]: Kubernetes release data (data/releases/eol.yaml: 1.31 end of life 2025-11-11, final patch 1.31.14; data/releases/schedule.yaml for 1.34–1.37). https://kubernetes.io/releases/ , https://github.com/kubernetes/website/blob/main/data/releases/eol.yaml
[^3]: CHANGELOG-1.31 (v1.31.0 urgent upgrade notes, API changes, features, deprecations, bug fixes; patch notes v1.31.1–v1.31.14 including CVE-2024-9042, CVE-2025-0426, CVE-2025-5187). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.31.md
[^4]: Go module proxy data for k8s.io/client-go v0.31.x (v0.31.0 2024-08-13, v0.31.14 2025-11-12, `go 1.22.0`), sigs.k8s.io/controller-runtime v0.19.x (v0.19.7 2025-03-08, requires k8s.io/* v0.31.0), sigs.k8s.io/controller-tools v0.16.x. https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.7 , https://pkg.go.dev/k8s.io/client-go@v0.31.14
[^5]: Kubebuilder scaffold versions (pkg/plugins/golang/v4/scaffolds/init.go at v4.2.0, v4.3.x, v4.4.0). https://github.com/kubernetes-sigs/kubebuilder/releases
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
[^7]: Deprecated API Migration Guide (flowcontrol.apiserver.k8s.io/v1beta3 removed in v1.32). https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^8]: Kubernetes v1.32 (2024-12-11), v1.33 (2025-04-23), v1.34 (2025-08-27), v1.35, v1.36 and v1.37 release announcements and CHANGELOGs. https://kubernetes.io/blog/2024/12/11/kubernetes-v1-32-release/ , https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/ , https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG
[^9]: CustomResourceDefinition validation rules (CEL), validation ratcheting, selectable fields; Kubernetes CEL libraries. https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/ , https://kubernetes.io/docs/reference/using-api/cel/
[^10]: Validating Admission Policy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
[^11]: controller-runtime v0.19 package documentation (`webhook/admission`: Validator/Defaulter deprecated; `builder.WebhookBuilder`; `cluster.Cluster.GetEventRecorderFor`; `metrics/filters`). https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.7
[^12]: KEP-4006: Transition from SPDY to WebSockets; Kubernetes 1.35 CHANGELOG (`AuthorizePodWebsocketUpgradeCreatePermission`). https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/4006-transition-spdy-to-websockets , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md
[^13]: Go 1.25 release notes — container-aware GOMAXPROCS. https://go.dev/doc/go1.25#container-aware-gomaxprocs
[^14]: Volumes — read-only mounts and `recursiveReadOnly`; Kubernetes 1.30 blog "Read-only volume mounts can be finally literally read-only". https://kubernetes.io/docs/concepts/storage/volumes/#read-only-mounts , https://kubernetes.io/blog/2024/04/23/recursive-read-only-mounts/
