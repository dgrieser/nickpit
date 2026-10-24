### Citations: Kubernetes 1.37: Manifests, CRDs, Policies and Controllers — Complete Review Guide

Sources for `kubernetes-1.37.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.37: Garhwal (release announcement, 2026-08-26). https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/
    - What's New in 1.37 — paragraph 1: "…to Beta, 27 enter Alpha, and 1 is a deprecation/removal."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 3: "…objects (built-in and CRs) after a storage-version or encryption change."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 4: "…a volume on one node can now block in `ContainerCreating`."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 10: "…`Node.status.declaredFeatures` lets the control plane handle version skew per node."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 11: "…request can be satisfied by a DRA driver via `DeviceClass.spec.extendedResourceName`."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 12: "Clients must honour `Retry-After`."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 13: "**KYAML** | `kubectl get -o kyaml` is stable."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 14: "`kubectl top` understands v1. v1beta1 keeps working."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — paragraph 1: "…absence on nodes or drivers that do not report health."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 1: "…or `External` metric; the HPA records `ScaledToZero` in its conditions."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 3: "…in 1.36 after a rollout bug, back on in 1.37)."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 4: "**PVC `Unused` condition** | `status.conditions[type=Unused]` with `lastTransitionTime` = "unused since"."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 8: "…conversion-webhook calls in parallel during watch-cache init (same total volume)."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 9: "**etcd RangeStream**, **WatchList gzip** | Server-side performance; no manifest change."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 10: "…needs the `unsafe-delete-ignore-read-errors` verb), now also as a dry run."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 11: "…(`CSIDriver.spec.preventPodSchedulingIfMissing`), **native histograms**, **kubelet in user namespace** | Node/scheduler behaviour."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — paragraph 1: "…by default (kube-apiserver 1.37.0 still lists the gate as alpha)."
    - What's New in 1.37 › Alpha in 1.37 — off by default (new fields are silently dropped, new enum values rejected) [NEW in 1.37] — paragraph 1: "…Pod sysctls, atomic-write volume `user` fields, Pod-level checkpoint/restore (CRI only)."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 2: "…no longer reference Secrets/ConfigMaps** (`PreventStaticPodAPIReferences` removed; the restriction is permanent)."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 8: "**kube-dns** is deprecated in favour of CoreDNS."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 12: "…to `true` (cgroup v1 nodes refuse to start unless overridden)."
    - Do Not Use or Recommend › Deprecated or removed older APIs and features — flag them — bullet 6: "…startup and is behind the deprecated `KubeProxyIPVS` gate since 1.37."
    - Pods and Workloads › StatefulSets — bullet 1: "…a Pod stuck forever, re-enabled in 1.37 with the fix."
    - Services, Networking and Traffic — bullet 9: "…the `KubeProxyIPVS` gate; expected default-off in 1.40, removal in 1.43)."
    - Autoscaling › HPA scale to zero (beta-on in 1.37) [NEW in 1.37] — bullet 2: "…scaling back up the condition becomes `False` with reason `NotScaledToZero`."
    - Storage — bullet 3: "…and never deletes anything — cleanup tools must decide themselves."
    - Versioning, Conversion & Storage Version Migration › Rules — bullet 5: "…initialising its watch cache — size the webhook for concurrency."
    - Security Hardening › Secrets — bullet 4: "…1.37 static Pods cannot reference Secrets or ConfigMaps at all."
    - client-go v0.37 — bullet 2: "…logging** is complete apart from a few auth-plugin log calls."
    - Error Handling, Requeue & API Server Back-Pressure — bullet 4: "…consistent LISTs return 429 instead of falling back to etcd."
    - Observability › Metrics — bullet 4: "…protobuf format; classic buckets stay available, so dashboards keep working."
[^2]: Kubernetes CHANGELOG-1.37 (v1.37.0 and v1.37.1 release notes). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.37.md
    - Kubernetes 1.37: Manifests, CRDs, Policies and Controllers — Complete Review Guide — quote block: "Kubernetes 1.37 itself is built with Go 1.26.x."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 1: "`PKIXPublicKey`/`ProofOfPossession` (deprecated in v1beta1) do not exist in v1."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 2: "…and the `clusterTrustBundle` projected volume source are on by default."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 5: "…sets the hostname seen inside the Pod, independent of `hostname`/`subdomain`."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 6: "Dots are still rejected."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 7: "…/ `scaleDown.tolerance` replace the cluster-wide 10 % tolerance per HPA."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 8: "…The `pods/resize` subresource also works for (restartable) init containers."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 9: "…now carries a diagnostic message (e.g. sandbox has no IP)."
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 14: "`kubectl top` understands v1. v1beta1 keeps working."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 2: "**HPA condition `observedGeneration`** | HPA conditions may carry `observedGeneration`."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 6: "…Admission webhooks no longer see `tokenreviews`, `subjectaccessreviews`, …, matching VAP/MAP."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 7: "…balancing** | With `--enable-aggregator-routing=true`, webhook calls are spread across endpoints."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 9: "**etcd RangeStream**, **WatchList gzip** | Server-side performance; no manifest change."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 10: "…needs the `unsafe-delete-ignore-read-errors` verb), now also as a dry run."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 11: "…(`CSIDriver.spec.preventPodSchedulingIfMissing`), **native histograms**, **kubelet in user namespace** | Node/scheduler behaviour."
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — paragraph 1: "…by default (kube-apiserver 1.37.0 still lists the gate as alpha)."
    - What's New in 1.37 › Alpha in 1.37 — off by default (new fields are silently dropped, new enum values rejected) [NEW in 1.37] — paragraph 1: "…Pod sysctls, atomic-write volume `user` fields, Pod-level checkpoint/restore (CRI only)."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 1: "Delete v1alpha2 objects before upgrading."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 3: "…`{requests: {}}`, `{limits: {}}`) no longer influences the QoS class."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 4: "…volumes**: an empty `image.reference` in Pod *templates* is now rejected."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 5: "…matches lists containing duplicates and `+` applies set/map merge semantics."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 7: "…and discovery APIs; the old ones are deprecated but stay."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 8: "**kube-dns** is deprecated in favour of CoreDNS."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 9: "`--proxy-url`."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 10: "`DeclarativeValidationTakeover` is locked."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 11: "`KubeProxyConfiguration.mode` defaults to `iptables` explicitly."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 12: "…to `true` (cgroup v1 nodes refuse to start unless overridden)."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 13: "`google.golang.org/grpc` v1.82.1 (strict path checking always on)."
    - Do Not Use or Recommend › Deprecated or removed older APIs and features — flag them — bullet 2: "**`scheduling.k8s.io/v1alpha2`** removed in 1.37."
    - Do Not Use or Recommend › Deprecated or removed older APIs and features — flag them — bullet 6: "…startup and is behind the deprecated `KubeProxyIPVS` gate since 1.37."
    - Pods and Workloads › Pod-level resources (beta-on since 1.34) — bullet 1: "…{}}`) no longer affects the QoS class (changed in 1.37)."
    - Pods and Workloads › Image volumes (GA 1.36) — bullet 3: "Required value`; 1.36 accepted it)."
    - Pods and Workloads › Probes, lifecycle and termination — bullet 5: "…exec readiness probes stopped running once a Pod began terminating."
    - Pods and Workloads › StatefulSets — bullet 3: "…to `updateRevision`; tooling that waited on that equality previously hung."
    - Pods and Workloads › Jobs and CronJobs — bullet 3: "…that rejected such edits before the `JobSuspended` condition was set."
    - Pods and Workloads › PodDisruptionBudgets — paragraph 1: "…structured `CauseType` values (1.37), so tools can stop string-matching messages."
    - Services, Networking and Traffic — bullet 2: "Since 1.37 the `services/proxy` subresource also routes via EndpointSlices."
    - Autoscaling › Other HPA rules — bullet 4: "…HPAs immediately instead of waiting for the 15 s resync."
    - Storage — bullet 8: "…writes files `0600`/directories `0700` because dumped logs may contain secrets."
    - Workload Identity: Pod Certificates & ClusterTrustBundles › Pod certificates (GA 1.37) [NEW in 1.37] — bullet 5: "…removed in v1); clients still setting them must be updated."
    - Workload Identity: Pod Certificates & ClusterTrustBundles › Pod certificates (GA 1.37) [NEW in 1.37] — bullet 6: "…actually mounts, unless authorised via `verb=request-podcertificate-signer` on the signer name."
    - CRD & API Design › Go types with markers (controller-gen v0.22, Kubebuilder v4.16 style) — bullet 1: "Do not flag either form."
    - CRD & API Design › PrintColumns — paragraph 1: "…crd` shows SCOPE, VERSIONS and CREATED AT for CRDs themselves."
    - CEL Validation Rules › CEL rules: what to check in review — bullet 9: "…+ ['x'] == oldSelf + ['x']`) can now evaluate differently."
    - Versioning, Conversion & Storage Version Migration › StorageVersionMigration (GA 1.37) [NEW in 1.37] — bullet 2: "…only edit `storedVersions` by hand when migrating with something else."
    - Admission: Policies and Webhooks › MutatingAdmissionPolicy (GA 1.36) — paragraph 1: "Since 1.37 MAP objects are stored in etcd as `admissionregistration.k8s.io/v1`."
    - Admission: Policies and Webhooks › Admission webhooks — bullet 8: "Service-account tokens bound to webhook configurations are alpha (`APIServerWebhookAuthenticationToken`)."
    - RBAC & Least Privilege — bullet 7: "…kubelet logs its effective configuration at startup, readable via `nodes/logs`."
    - Security Hardening › Container security context (restricted profile) — bullet 5: "…set `allowPrivilegeEscalation` together with `CAP_SYS_ADMIN`; creation still rejects that combination."
    - Security Hardening › Secrets — bullet 6: "…regression with env values from Secrets that contain non-UTF-8 bytes."
    - client-go v0.37 — bullet 1: "The old APIs are deprecated without a removal plan."
    - client-go v0.37 — bullet 4: "…on the test helper `cache.FakeCustomStore`) — do not suggest it."
    - client-go v0.37 — bullet 6: "`resourcelock.MultiLock`, `UnknownLeader`, `ConcatRawRecord` are deprecated in 1.37; use the `LeaseLock`."
    - client-go v0.37 — bullet 7: "**Discovery** now requests aggregated discovery `v2` only (no `v2beta1` fallback)."
    - client-go v0.37 — bullet 9: "…— earlier versions could create a world-readable `~/.kube/config` during migration."
    - Error Handling, Requeue & API Server Back-Pressure — bullet 4: "…consistent LISTs return 429 instead of falling back to etcd."
    - Observability › Metrics — bullet 5: "…the alpha `apiserver_cache_list_*` metrics are no longer exposed by default)."
    - Dynamic Resource Allocation (DRA) — bullet 1: "…fixed in 1.37.1 — require ≥ 1.37.1 before applying DeviceTaintRules."
    - Dynamic Resource Allocation (DRA) — bullet 3: "…— controllers that `Update` Pod status should still use patches."
    - Dynamic Resource Allocation (DRA) — bullet 4: "…be filtered by pool with the field selector `spec.pool.name` (1.37)."
    - Dynamic Resource Allocation (DRA) — bullet 5: "…API break); return `ErrHealthNotSupported` or disable the service with `HealthService(false)`."
[^3]: Kubernetes Releases (supported series, latest patches, EOL dates). https://kubernetes.io/releases/
    - Kubernetes 1.37: Manifests, CRDs, Policies and Controllers — Complete Review Guide — quote block: "…produced a 1.37.2 yet; the next scheduled round targets 2026-11-10."
    - Kubernetes 1.37: Manifests, CRDs, Policies and Controllers — Complete Review Guide — quote block: "…but has not been published) and is planned for **2026-12-16**."
[^4]: Kubernetes Patch Releases (schedule, maintenance mode and EOL per minor). https://kubernetes.io/releases/patch-releases/
    - Kubernetes 1.37: Manifests, CRDs, Policies and Controllers — Complete Review Guide — quote block: "…produced a 1.37.2 yet; the next scheduled round targets 2026-11-10."
    - Kubernetes 1.37: Manifests, CRDs, Policies and Controllers — Complete Review Guide — quote block: "…but has not been published) and is planned for **2026-12-16**."
[^5]: Kubernetes 1.38 release schedule (SIG Release). https://github.com/kubernetes/sig-release/tree/master/releases/release-1.38
    - Kubernetes 1.37: Manifests, CRDs, Policies and Controllers — Complete Review Guide — quote block: "…but has not been published) and is planned for **2026-12-16**."
[^6]: Kubernetes CHANGELOG-1.38 (v1.38.0-alpha.1 and alpha.2). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.38.md
    - Do Not Use or Recommend › From Kubernetes 1.38 (not released; alpha builds only) — paragraph 1: "…only in `v1.38.0-alpha.*` builds and the matching `k8s.io/*` v0.38 pre-releases:"
[^7]: Feature Gates reference. https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/
    - (not cited in the guide)
[^8]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
    - Do Not Use or Recommend › Deprecated or removed older APIs and features — flag them — bullet 1: "…`admissionregistration.k8s.io/v1beta1` webhooks, `apiextensions.k8s.io/v1beta1` CRDs, `rbac.authorization.k8s.io/v1beta1`, `certificates.k8s.io/v1beta1`, `coordination.k8s.io/v1beta1`, `scheduling.k8s.io/v1beta1` PriorityClass (1.22)."
[^9]: Common Expression Language in Kubernetes (libraries and availability). https://kubernetes.io/docs/reference/using-api/cel/
    - CEL Validation Rules › CEL library availability (all present on 1.37) — paragraph 1: "Kubernetes CEL reference."
[^10]: SELinux Volume Label Changes goes GA (and likely implications in v1.37), 2026-04-22. https://kubernetes.io/blog/2026/04/22/breaking-changes-in-selinux-volume-labeling/
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 4: "…a volume on one node can now block in `ContainerCreating`."
    - Security Hardening › SELinux volume labelling changes in 1.37 [NEW in 1.37] — paragraph 1: "…sets at least `seLinuxOptions.level`, and `seLinuxChangePolicy` is unset or `MountOption`."
    - Security Hardening › SELinux volume labelling changes in 1.37 [NEW in 1.37] — bullet 2: "…`--controllers=*,selinux-warning-controller`) emits Events and the `selinux_warning_controller_selinux_volume_conflict` metric for conflicting Pods."
[^11]: controller-runtime v0.25.0 release notes (and v0.25.2 source). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.25.0
    - Kubernetes 1.37: Manifests, CRDs, Policies and Controllers — Complete Review Guide — quote block: "…v0.25.0, controller-tools v0.22.0, `go 1.26.0` and derives `ENVTEST_K8S_VERSION=1.37` from `k8s.io/api`."
    - What's New in 1.37 › Behaviour and API changes that need action [NEW in 1.37] — bullet 6: "…pairing) deprecates `GetEventRecorderFor`, `CustomDefaulter`/`CustomValidator` and `client.Apply` patch type; see [Controllers](#controllers-with-controller-runtime-v025)."
    - Controllers with controller-runtime v0.25 — paragraph 1: "Its main changes for reviewers:"
[^12]: Kubebuilder v4.16.0 release (scaffold versions in `pkg/plugins/golang/v4/scaffolds/init.go`). https://github.com/kubernetes-sigs/kubebuilder/releases/tag/v4.16.0
    - Kubernetes 1.37: Manifests, CRDs, Policies and Controllers — Complete Review Guide — quote block: "…v0.25.0, controller-tools v0.22.0, `go 1.26.0` and derives `ENVTEST_K8S_VERSION=1.37` from `k8s.io/api`."
[^13]: controller-tools releases (v0.22.0, v0.23.0). https://github.com/kubernetes-sigs/controller-tools/releases
    - (not cited in the guide)
[^14]: Validating Admission Policy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
    - (not cited in the guide)
[^15]: Mutating Admission Policy. https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/
    - (not cited in the guide)
[^16]: Extend the Kubernetes API with CustomResourceDefinitions (validation rules, ratcheting, selectable fields). https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
    - (not cited in the guide)
[^17]: Sidecar Containers. https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/
    - Pods and Workloads › Sidecar containers (GA 1.33) — paragraph 1: "Sidecars do not block Job completion."
[^18]: Resize CPU and Memory Resources assigned to Containers (limitations for 1.37). https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/
    - Pods and Workloads › In-place resize (GA 1.35) — paragraph 2: "`status.resize` is deprecated — do not read it."
    - Pods and Workloads › In-place resize (GA 1.35) — paragraph 3: "A memory *decrease* with `NotRequired` is best-effort."
[^19]: User Namespaces. https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
    - Pods and Workloads › User namespaces (GA 1.36) — paragraph 1: "…on the volume filesystems, a container runtime that supports it)."
    - Pods and Workloads › User namespaces (GA 1.36) — paragraph 1: "…(root in the container is never root on the host)."
[^20]: Volumes (image volumes). https://kubernetes.io/docs/concepts/storage/volumes/
    - (not cited in the guide)
[^21]: Projected Volumes (podCertificate, clusterTrustBundle). https://kubernetes.io/docs/concepts/storage/projected-volumes/
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 1: "`PKIXPublicKey`/`ProofOfPossession` (deprecated in v1beta1) do not exist in v1."
    - Workload Identity: Pod Certificates & ClusterTrustBundles › Pod certificates (GA 1.37) [NEW in 1.37] — paragraph 1: "…source has a certificate; the kubelet rotates it from `status.beginRefreshAt`."
[^22]: Horizontal Pod Autoscaling. https://kubernetes.io/docs/concepts/workloads/autoscaling/horizontal-pod-autoscale/
    - (not cited in the guide)
[^23]: KEP-2021: HPA scale to zero. https://github.com/kubernetes/enhancements/issues/2021
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 1: "…or `External` metric; the HPA records `ScaledToZero` in its conditions."
    - Autoscaling › HPA scale to zero (beta-on in 1.37) [NEW in 1.37] — bullet 2: "…scaling back up the condition becomes `False` with reason `NotScaledToZero`."
[^24]: KEP-5793: Manifest-based admission control configuration. https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/5793-manifest-based-admission-control-config
    - What's New in 1.37 › Beta and enabled by default in 1.37 [NEW in 1.37] — table row 5: "…from files via `staticManifestsDir`; names ending in `.static.k8s.io` are reserved."
    - Do Not Use or Recommend › From Kubernetes 1.38 (not released; alpha builds only) — bullet 5: "…as added in 1.38; 1.37 always runs manifest-based webhooks first."
    - Admission: Policies and Webhooks › Manifest-based admission configuration (beta-on in 1.37) [NEW in 1.37] — paragraph 1: "Verified on 1.37 with `--admission-control-config-file`:"
[^25]: Migrate Kubernetes Objects Using Storage Version Migration; KEP-4192. https://kubernetes.io/docs/tasks/manage-kubernetes-objects/storage-version-migration/
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 3: "…objects (built-in and CRs) after a storage-version or encryption change."
    - Versioning, Conversion & Storage Version Migration › StorageVersionMigration (GA 1.37) [NEW in 1.37] — paragraph 1: "…the current storage version (also useful after rotating encryption-at-rest keys)."
[^26]: Kubernetes API Conventions. https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
    - Spec vs. Status, Status Conditions — quote block: "Existing `phase` fields may stay for compatibility."
[^27]: Pod Security Standards. https://kubernetes.io/docs/concepts/security/pod-security-standards/
    - Security Hardening › Pod Security Standards — paragraph 2: "…concrete minor (or accept that `latest` changes rules on upgrade)."
[^28]: Dynamic Resource Allocation. https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 11: "…request can be satisfied by a DRA driver via `DeviceClass.spec.extendedResourceName`."
[^29]: Go module proxy version metadata for k8s.io/api, controller-runtime, controller-tools, kubebuilder. https://proxy.golang.org/k8s.io/api/@v/list
    - Kubernetes 1.37: Manifests, CRDs, Policies and Controllers — Complete Review Guide — quote block: "…v0.25.0, controller-tools v0.22.0, `go 1.26.0` and derives `ENVTEST_K8S_VERSION=1.37` from `k8s.io/api`."
[^30]: KEP-5295: KYAML. https://github.com/kubernetes/enhancements/issues/5295
    - What's New in 1.37 › Stable (GA) in 1.37 — usable without feature gates [NEW in 1.37] — table row 13: "**KYAML** | `kubectl get -o kyaml` is stable."
    - YAML Pitfalls & KYAML › KYAML (stable in 1.37) [NEW in 1.37] — paragraph 1: "…document is valid YAML, so any kubectl/Helm/Kustomize version reads it."
[^31]: Server-Side Apply. https://kubernetes.io/docs/reference/using-api/server-side-apply/
    - (not cited in the guide)
[^32]: Version Skew Policy. https://kubernetes.io/releases/version-skew-policy/
    - Version-Aware Review — bullet 2: "…older than kube-apiserver (1.34–1.37 nodes on a 1.37 control plane)."
[^33]: Kubernetes Deprecation Policy. https://kubernetes.io/docs/reference/using-api/deprecation-policy/
    - (not cited in the guide)
[^34]: Jobs (pod failure policy, backoffLimitPerIndex, successPolicy, podReplacementPolicy, managedBy). https://kubernetes.io/docs/concepts/workloads/controllers/job/
    - (not cited in the guide)
[^35]: Virtual IPs and Service Proxies — traffic distribution. https://kubernetes.io/docs/reference/networking/virtual-ips/#traffic-distribution
    - Services, Networking and Traffic — bullet 1: "Auto` annotation is also set, the annotation wins."
[^36]: KEP-5647: Stale controller mitigation. https://github.com/kubernetes/enhancements/issues/5647
    - Foundational Principles — table row 7: "…read-your-writes mitigation to several built-in controllers for exactly this reason."
    - client-go v0.37 — bullet 4: "…on the test helper `cache.FakeCustomStore`) — do not suggest it."
    - Error Handling, Requeue & API Server Back-Pressure — bullet 5: "…informer has caught up, or opt into controller-runtime's experimental `EnableReadYourWritesConsistency`."
[^37]: Define Environment Variable Values Using An Init Container (env files). https://kubernetes.io/docs/tasks/inject-data-application/define-environment-variable-via-file/
    - (not cited in the guide)
