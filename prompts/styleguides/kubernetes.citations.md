### Citations: Kubernetes Style Guide

Footnote sources for `kubernetes.md`. Not sent to the model; kept for maintainers.

[^1]: Kubernetes Releases (supported minors, latest patches, end-of-life dates). https://kubernetes.io/releases/
[^2]: Kubernetes Patch Releases and release schedule data (`data/releases/schedule.yaml`, `eol.yaml`); current patch per minor: `https://dl.k8s.io/release/stable-1.N.txt`. https://kubernetes.io/releases/patch-releases/
[^3]: Version Skew Policy. https://kubernetes.io/releases/version-skew-policy/
[^4]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^5]: Feature Gates reference (alpha/beta/GA stages per release). https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
[^7]: Common Expression Language in Kubernetes (library availability, cost estimation). https://kubernetes.io/docs/reference/using-api/cel/
[^8]: Extend the Kubernetes API with CustomResourceDefinitions (validation rules, transition rules, ratcheting, selectable fields, pruning). https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
[^9]: Kubernetes API Conventions (spec/status, conditions, naming). https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
[^10]: Server-Side Apply. https://kubernetes.io/docs/reference/using-api/server-side-apply/
[^11]: Validating Admission Policy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
[^12]: Mutating Admission Policy. https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/
[^13]: Dynamic Admission Control (webhooks, failurePolicy, matchConditions, sideEffects). https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/
[^14]: Pod Security Standards and Pod Security Admission. https://kubernetes.io/docs/concepts/security/pod-security-standards/ , https://kubernetes.io/docs/concepts/security/pod-security-admission/
[^15]: Sidecar Containers. https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/
[^16]: Resize CPU and Memory Resources assigned to Containers. https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/
[^17]: User Namespaces. https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
[^18]: Role Based Access Control Good Practices. https://kubernetes.io/docs/concepts/security/rbac-good-practices/
[^19]: CHANGELOG-1.35 (`AuthorizePodWebsocketUpgradeCreatePermission` beta, on); KEP-4006 Transition from SPDY to WebSockets. https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md , https://github.com/kubernetes/enhancements/issues/4006
[^20]: Network Policies. https://kubernetes.io/docs/concepts/services-networking/network-policies/
[^21]: Gateway API (releases, standard channel, ReferenceGrant). https://gateway-api.sigs.k8s.io/
[^22]: Ingress NGINX Retirement: What You Need to Know. https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/
[^23]: controller-runtime releases (compatibility with k8s.io/* minors). https://github.com/kubernetes-sigs/controller-runtime/releases
[^24]: Kubebuilder releases and scaffold versions. https://github.com/kubernetes-sigs/kubebuilder/releases
[^25]: Go module proxy version metadata for k8s.io/api, controller-runtime, controller-tools, kubebuilder, gateway-api. https://proxy.golang.org/
[^26]: Kubebuilder Book — Configuring envtest for integration tests. https://book.kubebuilder.io/reference/envtest
[^27]: Migrate Kubernetes Objects Using Storage Version Migration. https://kubernetes.io/docs/tasks/manage-kubernetes-objects/storage-version-migration/
[^28]: Kubernetes CHANGELOGs 1.29–1.37. https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG
[^29]: Helm — Built-in Objects (`.Capabilities.KubeVersion`) and Chart `kubeVersion`. https://helm.sh/docs/chart_template_guide/builtin_objects/
[^30]: Leases (leader election). https://kubernetes.io/docs/concepts/architecture/leases/
[^31]: Finalizers; Owners and Dependents; Garbage Collection. https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/ , https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/ , https://kubernetes.io/docs/concepts/architecture/garbage-collection/
[^32]: Recommended Labels. https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
[^33]: Configure Liveness, Readiness and Startup Probes. https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/
[^34]: Specifying a Disruption Budget for your Application. https://kubernetes.io/docs/tasks/run-application/configure-pdb/
[^35]: Pod Topology Spread Constraints. https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
[^36]: Good practices for Kubernetes Secrets. https://kubernetes.io/docs/concepts/security/secrets-good-practices/
[^37]: Kubernetes Deprecation Policy. https://kubernetes.io/docs/reference/using-api/deprecation-policy/
