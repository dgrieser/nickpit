### Citations: Kubernetes 1.32 — Manifests, Operators & Controllers Review Guide

Footnote sources for `kubernetes-1.32.md`. Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.32: Penelope — release announcement, 2024-12-11. https://kubernetes.io/blog/2024/12/11/kubernetes-v1-32-release/
[^2]: Kubernetes release data (data/releases/schedule.yaml and eol.yaml: release dates, end-of-life dates, final patch 1.32.13); stable-1.32.txt; GitHub release v1.32.13. https://kubernetes.io/releases/ , https://dl.k8s.io/release/stable-1.32.txt , https://github.com/kubernetes/kubernetes/releases/tag/v1.32.13
[^3]: CHANGELOG-1.32 (v1.32.0 API changes, features, deprecations, cleanup; patch notes v1.32.1–v1.32.13 including CVE-2024-9042, CVE-2025-0426, CVE-2025-4563, CVE-2025-5187). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.32.md
[^4]: Go module proxy metadata for k8s.io/client-go v0.32.13 (2026-02-27, go 1.23.0), sigs.k8s.io/controller-runtime v0.20.0/v0.20.4 (go.mod requires k8s.io/* v0.32.0/v0.32.1), sigs.k8s.io/controller-tools v0.17.0–v0.17.3. https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4 , https://pkg.go.dev/k8s.io/client-go@v0.32.13
[^5]: Kubebuilder v4.5.0/v4.5.2 scaffold versions (pkg/plugins/golang/v4/scaffolds/init.go: ControllerRuntimeVersion v0.20.0/v0.20.4, ControllerToolsVersion v0.17.1/v0.17.2). https://github.com/kubernetes-sigs/kubebuilder/releases
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
[^7]: Deprecated API Migration Guide — flowcontrol resources v1.32. https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132
[^8]: Kubernetes v1.34 release announcement (2025-08-27), list of graduations to stable. https://kubernetes.io/blog/2025/08/27/kubernetes-v1-34-release/
[^9]: Kubernetes v1.33 release announcement (2025-04-23) and CHANGELOG-1.33. https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/ , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.33.md
[^10]: Kubernetes v1.35, v1.36 and v1.37 release announcements and changelogs. https://kubernetes.io/blog/2025/12/17/kubernetes-v1-35-release/ , https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG
[^11]: Kubernetes documentation: CustomResourceDefinition validation rules (CEL), validation ratcheting, selectable fields; CEL in Kubernetes (libraries). https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/ , https://kubernetes.io/docs/reference/using-api/cel/
[^12]: controller-runtime v0.20.0 release notes (k8s.io v1.32 libraries, removal of Defaulter/Validator, CustomDefaulter unknown fields, warning deduplication, priority queue, EnableWatchBookmarks, HasOwnerReference). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.20.0
[^13]: Validating Admission Policy and Mutating Admission Policy documentation. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/ , https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/
[^14]: KEP-4006: Transition from SPDY to WebSockets (kubectl exec/attach/port-forward), and the 1.35 `AuthorizePodWebsocketUpgradeCreatePermission` change. https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/4006-transition-spdy-to-websockets
[^15]: Go 1.25 release notes — container-aware GOMAXPROCS. https://go.dev/doc/go1.25#container-aware-gomaxprocs
[^16]: Ingress NGINX Retirement: What You Need to Know (Kubernetes blog, 2025-11-11). https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/
