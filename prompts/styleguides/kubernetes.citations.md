### Citations: Kubernetes Style Guide

Sources for `kubernetes.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Kubernetes Releases (supported minors, latest patches, end-of-life dates). https://kubernetes.io/releases/
    - Kubernetes Style Guide — quote block: "…in development (latest pre-release `v1.38.0-alpha.2`) and is planned for 2026-12-16."
    - Release and Support Status — paragraph 1: "…of October 2026 (data from kubernetes.io release schedule and `dl.k8s.io/release/stable-1.N.txt`)."
[^2]: Kubernetes Patch Releases and release schedule data (`data/releases/schedule.yaml`, `eol.yaml`); current patch per minor: `https://dl.k8s.io/release/stable-1.N.txt`. https://kubernetes.io/releases/patch-releases/
    - Kubernetes Style Guide — quote block: "…in development (latest pre-release `v1.38.0-alpha.2`) and is planned for 2026-12-16."
    - Release and Support Status — paragraph 1: "…of October 2026 (data from kubernetes.io release schedule and `dl.k8s.io/release/stable-1.N.txt`)."
[^3]: Version Skew Policy. https://kubernetes.io/releases/version-skew-policy/
    - Version-Aware Review › Version skew — bullet 2: "…older** than the API server (kube-apiserver 1.37 with 1.34 nodes)."
[^4]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
    - Do Not Use or Recommend › Removed API versions — flag every occurrence — paragraph 1: "…any supported release and fail with `no matches for kind`:"
[^5]: Feature Gates reference (alpha/beta/GA stages per release). https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/
    - Version-Tied Features at a Glance — paragraph 1: "…the header; the rest come from release notes and KEPs."
[^6]: KEP-3136: Beta APIs Off by Default. https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/3136-beta-apis-off-by-default
    - Version-Aware Review › Why version mistakes are dangerous: fields are dropped silently — bullet 6: "…since 1.24** and need `--runtime-config` plus usually a feature gate."
[^7]: Common Expression Language in Kubernetes (library availability, cost estimation). https://kubernetes.io/docs/reference/using-api/cel/
    - Version-Tied Features at a Glance › CEL library availability — paragraph 1: "…an older API server — it does not degrade gracefully."
    - CEL Validation Rules — paragraph 1: "The same expression language drives ValidatingAdmissionPolicy, MutatingAdmissionPolicy and webhook `matchConditions`."
[^8]: Extend the Kubernetes API with CustomResourceDefinitions (validation rules, transition rules, ratcheting, selectable fields, pruning). https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
    - CEL Validation Rules — paragraph 1: "The same expression language drives ValidatingAdmissionPolicy, MutatingAdmissionPolicy and webhook `matchConditions`."
[^9]: Kubernetes API Conventions (spec/status, conditions, naming). https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
    - Spec, Status and Conditions — quote block: "…add new `phase` string fields; conditions are the API convention."
[^10]: Server-Side Apply. https://kubernetes.io/docs/reference/using-api/server-side-apply/
    - Controller Patterns › Why it looks like this — bullet 3: "**Server-side apply**"
[^11]: Validating Admission Policy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
    - Admission Control › ValidatingAdmissionPolicy (GA 1.30) — section heading
[^12]: Mutating Admission Policy. https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/
    - Admission Control › MutatingAdmissionPolicy (GA 1.36) — section heading
[^13]: Dynamic Admission Control (webhooks, failurePolicy, matchConditions, sideEffects). https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/
    - Admission Control › Admission webhooks — section heading
[^14]: Pod Security Standards and Pod Security Admission. https://kubernetes.io/docs/concepts/security/pod-security-standards/ , https://kubernetes.io/docs/concepts/security/pod-security-admission/
    - Pod Security › Pod Security Admission (GA since 1.25) — paragraph 1: "…no `*-version` label is set) to see upcoming violations early."
[^15]: Sidecar Containers. https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/
    - Workload Manifests › Native sidecar containers (beta-on 1.29, GA 1.33) — paragraph 1: "…**after** the app containers and does not block Job completion."
[^16]: Resize CPU and Memory Resources assigned to Containers. https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/
    - Workload Manifests › In-place resize (beta-on 1.33, GA 1.35) — paragraph 1: "…subresource**; a normal Pod update that changes resources is rejected."
[^17]: User Namespaces. https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
    - Pod Security › Version-tied hardening options — bullet 2: "…API server rejects it under every policy version, including `latest`)."
[^18]: Role Based Access Control Good Practices. https://kubernetes.io/docs/concepts/security/rbac-good-practices/
    - RBAC and Least Privilege › Rules — bullet 4: "…that namespace), `update` on `validatingwebhookconfigurations`/`mutatingwebhookconfigurations`/admission policies, and `list`/`watch` on Secrets."
[^19]: CHANGELOG-1.35 (`AuthorizePodWebsocketUpgradeCreatePermission` beta, on); KEP-4006 Transition from SPDY to WebSockets. https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md , https://github.com/kubernetes/enhancements/issues/4006
    - RBAC and Least Privilege › The `pods/exec` `get` loophole (fixed by default in 1.35) — paragraph 1: "…resource "pods/exec"`, a user with `get` and `create` passes authorization)."
[^20]: Network Policies. https://kubernetes.io/docs/concepts/services-networking/network-policies/
    - Services, Ingress, Gateway API and NetworkPolicy › NetworkPolicy — bullet 1: "Policies are additive allow-lists;"
[^21]: Gateway API (releases, standard channel, ReferenceGrant). https://gateway-api.sigs.k8s.io/
    - Services, Ingress, Gateway API and NetworkPolicy › Gateway API — paragraph 1: "…October 2026) and an implementation (a controller) must be present."
[^22]: Ingress NGINX Retirement: What You Need to Know. https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/
    - Do Not Use or Recommend › Deprecated — flag in new code — bullet 8: "…new routing should use Gateway API or a maintained controller."
    - Services, Ingress, Gateway API and NetworkPolicy › Ingress — bullet 2: "…any new dependency on the retired community Ingress NGINX controller."
[^23]: controller-runtime releases (compatibility with k8s.io/* minors). https://github.com/kubernetes-sigs/controller-runtime/releases
    - Client Library Compatibility — paragraph 1: "…and Kubebuilder each target one Kubernetes minor per release line."
[^24]: Kubebuilder releases and scaffold versions. https://github.com/kubernetes-sigs/kubebuilder/releases
    - Client Library Compatibility — paragraph 1: "…and Kubebuilder each target one Kubernetes minor per release line."
[^25]: Go module proxy version metadata for k8s.io/api, controller-runtime, controller-tools, kubebuilder, gateway-api. https://proxy.golang.org/
    - Client Library Compatibility — paragraph 1: "…and Kubebuilder each target one Kubernetes minor per release line."
[^26]: Kubebuilder Book — Configuring envtest for integration tests. https://book.kubebuilder.io/reference/envtest
    - Testing › Integration tests with envtest — paragraph 1: "…and kube-apiserver (binaries from `setup-envtest use <minor>`, exposed via `KUBEBUILDER_ASSETS`)."
[^27]: Migrate Kubernetes Objects Using Storage Version Migration. https://kubernetes.io/docs/tasks/manage-kubernetes-objects/storage-version-migration/
    - API Versioning, Conversion and Storage Migration — bullet 6: "…loop, then `kubectl patch crd <name> --subresource=status` to fix `storedVersions`."
[^28]: Kubernetes CHANGELOGs 1.29–1.37. https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG
    - Version-Tied Features at a Glance — paragraph 1: "…the header; the rest come from release notes and KEPs."
[^29]: Helm — Built-in Objects (`.Capabilities.KubeVersion`) and Chart `kubeVersion`. https://helm.sh/docs/chart_template_guide/builtin_objects/
    - Helm and Kustomize — bullet 1: "Declare `kubeVersion:` in `Chart.yaml` for the range you actually test,"
[^30]: Leases (leader election). https://kubernetes.io/docs/concepts/architecture/leases/
    - Caches, Informers and Leader Election › Leader election — bullet 2: "Leases live in `coordination.k8s.io/v1`;"
[^31]: Finalizers; Owners and Dependents; Garbage Collection. https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/ , https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/ , https://kubernetes.io/docs/concepts/architecture/garbage-collection/
    - Finalizers, Owner References and Garbage Collection › Finalizers — paragraph 1: "…not add finalizers for in-cluster children that owner references cover."
[^32]: Recommended Labels. https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
    - Naming Conventions › Labels and annotations — bullet 1: "Use the recommended `app.kubernetes.io/*` labels;"
[^33]: Configure Liveness, Readiness and Startup Probes. https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/
    - Workload Manifests › Probes, lifecycle and graceful termination — bullet 1: "**Liveness must not depend on downstream services**"
[^34]: Specifying a Disruption Budget for your Application. https://kubernetes.io/docs/tasks/run-application/configure-pdb/
    - Workload Manifests › PodDisruptionBudgets — bullet 1: "Every replicated, availability-sensitive workload gets a PDB."
[^35]: Pod Topology Spread Constraints. https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
    - Workload Manifests › A production Deployment — bullet 6: "**Topology spread**"
[^36]: Good practices for Kubernetes Secrets. https://kubernetes.io/docs/concepts/security/secrets-good-practices/
    - Configuration and Secrets — bullet 3: "**Never commit plaintext Secrets.**"
[^37]: Kubernetes Deprecation Policy. https://kubernetes.io/docs/reference/using-api/deprecation-policy/
    - API Versioning, Conversion and Storage Migration — bullet 5: "…clients to migrate (built-in beta APIs follow the deprecation policy"
