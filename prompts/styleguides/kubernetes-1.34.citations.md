### Citations: Kubernetes 1.34 — Manifests, CRDs and Go Controllers

Footnote sources for `kubernetes-1.34.md`. Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.34: Of Wind & Will (O' WaW) — release announcement (enhancement counts, GA/beta/alpha features, deprecations). https://kubernetes.io/blog/2025/08/27/kubernetes-v1-34-release/
[^2]: Kubernetes CHANGELOG-1.34 (v1.34.0 through v1.34.12: API changes, urgent upgrade notes, patch fixes). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.34.md
[^3]: Kubernetes patch releases and support periods (maintenance mode 2026-08-27, end of life 2026-10-27; schedule data). https://kubernetes.io/releases/patch-releases/ , https://github.com/kubernetes/website/blob/main/data/releases/schedule.yaml
[^4]: Kubernetes GitHub releases (v1.34.12, v1.35.9, v1.36.5, v1.37.1 published 2026-09-23; v1.38.0-alpha.2). https://github.com/kubernetes/kubernetes/releases
[^5]: controller-runtime releases and compatibility (v0.22.x ↔ k8s.io/* v0.34). https://github.com/kubernetes-sigs/controller-runtime/releases ; controller-tools releases: https://github.com/kubernetes-sigs/controller-tools/releases
[^6]: Kubebuilder v4 scaffold versions (v4.9.0: controller-runtime v0.22.1; v4.10.0: v0.22.4; controller-tools v0.19.0). https://github.com/kubernetes-sigs/kubebuilder/blob/v4.10.0/pkg/plugins/golang/v4/scaffolds/init.go
[^7]: Pod Security Standards (probe and lifecycle host restriction). https://kubernetes.io/docs/concepts/security/pod-security-standards/ ; KEP-4940: https://github.com/kubernetes/enhancements/issues/4940
[^8]: Common Expression Language in Kubernetes (library availability by version, semver library 1.34+, format library). https://kubernetes.io/docs/reference/using-api/cel/
[^9]: Kubernetes v1.35 release announcement and CHANGELOG-1.35. https://kubernetes.io/blog/2025/12/17/kubernetes-v1-35-release/ , https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md
[^10]: Kubernetes v1.36 release announcement. https://kubernetes.io/blog/2026/04/22/kubernetes-v1-36-release/
[^11]: Kubernetes v1.37 release announcement. https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/
[^12]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^13]: Dynamic Resource Allocation. https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/
[^14]: Resize CPU and Memory Resources assigned to Containers. https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/
[^15]: Server-Side Apply. https://kubernetes.io/docs/reference/using-api/server-side-apply/
[^16]: Validating Admission Policy and Mutating Admission Policy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/ , https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/
[^17]: Extend the Kubernetes API with CustomResourceDefinitions (validation rules, transition rules, ratcheting, selectable fields). https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
[^18]: client-go feature gates (WatchListClient, InOrderInformers, CBOR). https://github.com/kubernetes/client-go/blob/v0.34.12/features/known_features.go
[^19]: Helm built-in objects (.Capabilities) and Chart.yaml kubeVersion. https://helm.sh/docs/chart_template_guide/builtin_objects/ , https://helm.sh/docs/topics/charts/
[^20]: controller-runtime envtest and setup-envtest. https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest , https://github.com/kubernetes-sigs/controller-runtime/tree/main/tools/setup-envtest
[^21]: Feature gates reference (stages and defaults per release). https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/
