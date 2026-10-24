### Citations: Kubernetes 1.35 — Manifests, Operators & Controllers Review Guide

Footnote sources for `kubernetes-1.35.md`. Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.35: Timbernetes (The World Tree Release) — release announcement, 2025-12-17. https://kubernetes.io/blog/2025/12/17/kubernetes-v1-35-release/
[^2]: Kubernetes release schedule and patch releases (data/releases/schedule.yaml: 1.35 release date, patch dates, maintenance mode and end-of-life dates); GitHub release v1.35.9. https://kubernetes.io/releases/patch-releases/ , https://github.com/kubernetes/kubernetes/releases/tag/v1.35.9
[^3]: CHANGELOG-1.35 (v1.35.0 urgent upgrade notes, API changes, features, deprecations; patch notes v1.35.1–v1.35.9). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md
[^4]: Go module proxy listings for k8s.io/client-go v0.35.x, sigs.k8s.io/controller-runtime v0.23.x (go.mod requires k8s.io/* v0.35.0, go 1.25.0), sigs.k8s.io/controller-tools v0.20.x. https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3 , https://pkg.go.dev/k8s.io/client-go@v0.35.9
[^5]: Kubebuilder v4.11–v4.16 scaffold versions (pkg/plugins/golang/v4/scaffolds/init.go: ControllerRuntimeVersion v0.23.x, ControllerToolsVersion v0.20.x). https://github.com/kubernetes-sigs/kubebuilder/releases
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
[^7]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^8]: Kubernetes v1.36 release announcement (2026-04-22) and CHANGELOG-1.36. https://kubernetes.io/blog/2026/04/22/kubernetes-v1-36-release/ , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.36.md
[^9]: Kubernetes v1.37 release announcement (2026-08-26) and CHANGELOG-1.37. https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/ , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.37.md
[^10]: controller-runtime v0.23.0 release notes (events API migration, generic webhooks, priority queue default, subresource apply, FieldOwner/FieldValidation client options). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.23.0
[^11]: Go 1.25 release notes — container-aware GOMAXPROCS. https://go.dev/doc/go1.25#container-aware-gomaxprocs
[^12]: Kubernetes 1.35: In-Place Pod Resize Graduates to Stable (2025-12-19); Resize CPU and Memory Resources assigned to Containers. https://kubernetes.io/blog/2025/12/19/kubernetes-v1-35-in-place-pod-resize-ga/ , https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/
[^13]: KEP-5504: Comparable resource versions; apimachinery `pkg/util/resourceversion`. https://github.com/kubernetes/enhancements/issues/5504 , https://pkg.go.dev/k8s.io/apimachinery/pkg/util/resourceversion
[^14]: Kubernetes API reference: CustomResourceDefinition validation rules (CEL), validation ratcheting, selectable fields. https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
[^15]: Validating Admission Policy and Mutating Admission Policy documentation. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/ , https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/
