### Kubernetes Style Guide

#### Foundational Principles

| Principle | Description |
|---|---|
| **Level-based reconciliation** | React to current state, never to specific event types. The reconcile function must not receive or depend on event type information. |
| **Idempotency** | Every reconcile call must be safe to run repeatedly with the same outcome. |
| **Single responsibility** | One operator per application. One controller per root Kind. |
| **Declarative intent** | `spec` expresses *desired state*; the controller drives *actual state* toward it. |
| **Avoid edge-triggering** | Never build logic that depends on "what changed". Always reconcile the full object state. |
| **Open World assumption** | The system may miss events; design controllers that recover fully from a cold start. |

---

#### Project Layout & Tooling

##### Recommended Scaffold (Kubebuilder)

```
├── api/
│   └── v1alpha1/
│       ├── mykind_types.go          # API type definitions
│       ├── mykind_webhook.go        # Admission webhook logic
│       └── groupversion_info.go
├── internal/
│   └── controller/
│       └── mykind_controller.go     # Reconciler
├── config/
│   ├── crd/                         # Generated CRD manifests
│   ├── rbac/                        # Generated RBAC manifests
│   └── default/                     # Kustomize overlay
├── Dockerfile
└── Makefile                         # make generate, make manifests, make test
```

##### Toolchain Requirements

- **kubebuilder** — scaffolding, marker-based manifest generation
- **controller-gen** — CRD YAML, RBAC, DeepCopy code generation (`make generate`, `make manifests`)
- **controller-runtime** — reconciler lifecycle, informer caches, client abstraction
- **envtest** — in-process API server for unit/integration tests
- **kustomize** — layered config management for deploy targets

##### Makefile Targets (Mandatory)

```makefile
make generate      # DeepCopy, runtime.Object implementations
make manifests     # CRD YAML, RBAC, WebhookConfiguration
make install       # Apply CRDs to cluster
make test          # Unit + envtest integration tests
make docker-build  # Build operator image
```

---

#### Naming Conventions

##### Character Rules by Name Class

Kubernetes uses several distinct name classes, each with its own allowed character set. Use the table below to pick the right rule for the field you're naming.

| Name class | Used for | Allowed characters | Disallowed | Start/end | Max length |
|---|---|---|---|---|---|
| **DNS Subdomain** (RFC 1123) | API groups, most `metadata.name` values (e.g., Pods, Deployments, CRs), CRD `spec.group` | lowercase `a–z`, digits `0–9`, `-`, `.` | uppercase `A–Z`, `_`, whitespace, all other punctuation (`/`, `:`, `@`, `+`, `*`, etc.), unicode | must start AND end with alphanumeric (`[a-z0-9]`) | 253 |
| **DNS Label** (RFC 1123) | `metadata.namespace`, Service names, label *values* that must be DNS-safe, container names | lowercase `a–z`, digits `0–9`, `-` | uppercase, `_`, `.`, whitespace, all other punctuation, unicode | must start AND end with alphanumeric | 63 |
| **DNS Label** (RFC 1035, stricter) | older Service names, some legacy fields | lowercase `a–z`, digits `0–9`, `-` | same as above PLUS cannot start with a digit | must start with `[a-z]`, end with alphanumeric | 63 |
| **Qualified Name** (label/annotation key, finalizer) | label keys, annotation keys, finalizer names, CRD `categories` prefix | name part: `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` ; optional prefix part: DNS Subdomain followed by `/` | name part: whitespace, `/` (except as prefix separator), `:`, all other punctuation | name part must start AND end with alphanumeric | name ≤ 63; prefix ≤ 253; total ≤ 317 |
| **Label Value** | `metadata.labels` values | `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` (or empty string) | whitespace, `/`, `:`, all other punctuation, unicode | if non-empty: alphanumeric at both ends | 63 |
| **Annotation Value** | `metadata.annotations` values | any valid UTF-8 string (incl. whitespace, JSON, base64) | none per key | n/a | total across all annotations ≤ 256 KiB |
| **C-Identifier** | env var names, ConfigMap/Secret keys (when used as env vars) | `A–Z`, `a–z`, `0–9`, `_` | `-`, `.`, `/`, whitespace, all other punctuation | must start with `[A-Za-z_]` (not a digit) | no hard limit |
| **ConfigMap / Secret data key** | `data` / `stringData` keys | `a–z`, `A–Z`, `0–9`, `-`, `_`, `.` | `/`, whitespace, `:`, all other punctuation | any alphanumeric | 253 |
| **Path Segment Name** | `metadata.name` for objects exposed in URL paths | DNS Subdomain set MINUS `.` and `..` as full name | same as DNS Subdomain plus the literal names `.` and `..` | as DNS Subdomain | 253 |

> ⚠️ Underscore `_` is **never** valid in a DNS Subdomain or DNS Label name — that rules it out for `metadata.name`, `metadata.namespace`, API group, and Service name. Underscore IS valid in label/annotation keys (name part), label values, and ConfigMap/Secret data keys.

> ⚠️ Uppercase letters are **never** valid in DNS Subdomain or DNS Label names. They ARE valid in label keys (name part), label values, qualified names, and C-identifiers.

##### API Group, Kind, Resource

| Element | Rule | Example |
|---|---|---|
| **API Group** | Lowercase DNS subdomain you own | `cache.example.com` |
| **Kind** | CamelCase, singular | `RedisCluster` |
| **Resource (plural)** | Lowercase, plural | `redisclusters` |
| **Short name** | 2–5 lowercase chars; register in `.spec.names.shortNames` | `rc` → avoid conflicts with built-ins |
| **Category** | Lowercase; register in `.spec.names.categories` | `all` (use sparingly) |

> ⚠️ Do **not** use the empty group, single-word names, or `*.k8s.io` — those are reserved by the Kubernetes project.

##### Go Type Names

```go
// ✅ Correct
type RedisCluster struct { ... }
type RedisClusterSpec struct { ... }
type RedisClusterStatus struct { ... }
type RedisClusterList struct { ... }

// ❌ Wrong
type rediscluster struct { ... }
type RedisClusterSpecs struct { ... }
```

##### JSON Field Names

| Rule | ✅ Good | ❌ Bad |
|---|---|---|
| camelCase | `replicaCount` | `replica_count`, `ReplicaCount` |
| Declarative nouns (desired state) | `replicas` | `setReplicas` |
| Name references | `secretName` | `secretRef.name` (unless cross-namespace) |
| Object references | `secretRef` | `secret` |
| Duration | `timeoutSeconds` (`int32`) | `timeout: "30s"` in spec primitive fields |
| Avoid abbreviations | `maximumRetries` | `maxRetries` (except established: `url`, `ip`, `id`) |

##### Constant (Enum) Values

```go
// CamelCase string alias — never Go iota enums in API types
type PhaseType string

const (
    PhasePending   PhaseType = "Pending"
    PhaseRunning   PhaseType = "Running"
    PhaseFailed    PhaseType = "Failed"
    PhaseSucceeded PhaseType = "Succeeded"
)
```

##### `client.ObjectKey` in Go Code

When code uses `sigs.k8s.io/controller-runtime/pkg/client.ObjectKey`, assume `Namespace` and `Name` came from Kubernetes object identity (DNS Subdomain / DNS Label rules above) unless there is evidence otherwise.

##### Label & Annotation Key Examples

Use the Qualified Name rules from the character table above. Conventional prefixes:

```
# Labels (user-facing, for selection)
app.kubernetes.io/name: redis
app.kubernetes.io/component: cache
app.kubernetes.io/managed-by: redis-operator

# Annotations (tooling metadata)
cache.example.com/last-backup-time: "2025-01-01T00:00:00Z"
```

---

#### CRD & API Design

##### Minimal Skeleton

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=rc,categories=all
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type RedisCluster struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   RedisClusterSpec   `json:"spec,omitempty"`
    Status RedisClusterStatus `json:"status,omitempty"`
}
```

##### Spec Design Rules

- Use `// +optional` and `omitempty` for optional fields; `// +required` for required.
- Prefer **lists of named subobjects** over maps for portability with strategic merge patch:
  ```yaml
  # ✅ Preferred
  ports:
    - name: redis
      port: 6379
  # ❌ Avoid
  ports:
    redis: 6379
  ```
- **Never use floating-point** fields in `spec`. Use `int32`/`int64` or string with units.
- **Do not use `bool` where an enum will be needed** — use a string type alias from day one.
- **No raw maps** except for labels, annotations, and opaque user data.
- **Object-level fields only**: do not add top-level fields other than `metadata`, `spec`, `status`.

##### OpenAPI / Kubebuilder Validation Markers

```go
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=100
// +kubebuilder:validation:Enum=small;medium;large
// +kubebuilder:validation:Pattern=`^[a-z][a-z0-9-]*$`
// +kubebuilder:validation:XValidation:rule="self.maxReplicas >= self.minReplicas",message="maxReplicas must be >= minReplicas"
Replicas int32 `json:"replicas"`
```

CEL cross-field validation (prefer over webhooks for simple invariants):
```go
// +kubebuilder:validation:XValidation:rule="!has(self.foo) || self.foo != self.bar",message="foo and bar must differ"
```

Immutability via CEL:
```go
// +kubebuilder:validation:XValidation:rule="self.storageClass == oldSelf.storageClass",message="storageClass is immutable"
StorageClass string `json:"storageClass"`
```

##### PrintColumns

Always define at minimum:
1. A human-readable state/phase column
2. The `Age` column from `metadata.creationTimestamp`

---

#### Spec vs. Status

```
┌──────────────────────────────────────────────────────────────┐
│                       ETCD                                    │
│  spec  ← user/agent writes (desired state)                   │
│  status ← controller writes via /status subresource          │
└──────────────────────────────────────────────────────────────┘
```

| | `spec` | `status` |
|---|---|---|
| **Written by** | Users / agents / CI | Controllers only |
| **Represents** | Desired state | Observed / current state |
| **Subresource** | No | **Yes** — always enable with `// +kubebuilder:subresource:status` |
| **PUT/POST behaviour** | Updated | Ignored (must use `/status` endpoint) |
| **Read access** | Operators need read | Operators write |

##### Updating Status (controller-runtime)

```go
// ✅ Always update status through the subresource
if err := r.Status().Update(ctx, myResource); err != nil {
    return ctrl.Result{}, err
}
// ❌ Never
r.Update(ctx, myResource) // this ignores status changes or overwrites spec
```

##### observedGeneration

Always set `status.observedGeneration = metadata.generation` after a successful reconcile:

```go
myResource.Status.ObservedGeneration = myResource.Generation
```

This allows external tools to detect whether the controller has processed the latest spec change.

---

#### Status Conditions

Use `metav1.Condition` from `k8s.io/apimachinery/pkg/apis/meta/v1`:

```go
// In your Status struct:
// +listType=map
// +listMapKey=type
// +patchStrategy=merge
// +patchMergeKey=type
// +optional
Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
```

##### Condition Field Rules

| Field | Rule |
|---|---|
| `Type` | PascalCase; short, adjective or past-tense verb: `Ready`, `Degraded`, `Synced`, `Failed` |
| `Status` | `True`, `False`, or `Unknown` — never empty |
| `Reason` | CamelCase, one word, machine-readable: `ReconcileError`, `ImagePullBackOff` |
| `Message` | Human-readable sentence; may contain specifics |
| `ObservedGeneration` | Copy from `metadata.generation` at the time of observation |
| `LastTransitionTime` | Update **only** when status value transitions; not on every reconcile |

##### Standard Condition Types

| Type | Normal Status | Meaning |
|---|---|---|
| `Ready` | `True` | Object is fully operational |
| `Reconciling` | `True` | In-progress reconciliation; desired ≠ actual |
| `Stalled` | `False` | Cannot make progress; human attention needed |
| `Degraded` | `False` | Running but with reduced capacity |
| `Available` | `True` | Operational and serving |

> ⚠️ **Deprecated:** Do **not** use `phase` string fields (e.g., `"Running"`, `"Pending"`). Conditions are the approved pattern — phase is a state machine anti-pattern that hinders backward compatibility.

##### Setting Conditions (example with controller-runtime meta/v1)

```go
meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
    Type:               "Ready",
    Status:             metav1.ConditionFalse,
    ObservedGeneration: resource.Generation,
    Reason:             "ReconcileError",
    Message:            fmt.Sprintf("failed to create Service: %v", err),
})
```

---

#### Versioning & API Evolution

##### Version Lifecycle

```
v1alpha1 → v1alpha2 → v1beta1 → v1beta2 → v1
```

| Stage | Stability | Enabled by Default | Backward Compat |
|---|---|---|---|
| `v1alphaX` | Experimental | No | Not guaranteed |
| `v1betaX` | Pre-release | Yes | Best-effort |
| `v1` (GA) | Stable | Yes | **Required** |

##### Rules

1. **Never remove or rename existing fields** in a served version.
2. **Never tighten validation** on an existing field in a way that rejects previously valid values.
3. **Add fields as optional** (`omitempty`), never required.
4. Store only **one primary version** in etcd; all others are views via conversion.
5. Conversion must be **lossless** in both directions.
6. For breaking changes: introduce a new version, mark the old as deprecated, maintain support for ≥ 1 year before removal.
7. Use a **ConversionWebhook** when schemas diverge between versions.

##### CRD Served/Storage Annotation

```yaml
versions:
  - name: v1beta1
    served: true
    storage: false    # Old version still served but not stored
    deprecated: true
    deprecationWarning: "v1beta1 is deprecated. Use v1."
  - name: v1
    served: true
    storage: true     # This is the canonical storage version
```

---

#### Controller / Reconciler Patterns

##### Reconciler Skeleton (Go / controller-runtime)

```go
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/finalizers,verbs=update

func (r *RedisClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)

    // 1. Fetch the resource
    obj := &cachev1.RedisCluster{}
    if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 2. Initialize defaults (if not using mutating webhook)
    if !r.isInitialized(obj) {
        if err := r.Client.Update(ctx, obj); err != nil {
            return ctrl.Result{}, err
        }
        return ctrl.Result{}, nil // triggers another reconcile
    }

    // 3. Handle deletion via finalizer
    if !obj.DeletionTimestamp.IsZero() {
        return r.handleDeletion(ctx, obj)
    }

    // 4. Validate semantics (beyond OpenAPI)
    if err := r.validate(obj); err != nil {
        return r.setFailedCondition(ctx, obj, err)
    }

    // 5. Business logic — ensure desired state
    if err := r.reconcileDeployment(ctx, obj); err != nil {
        return r.setFailedCondition(ctx, obj, err)
    }

    // 6. Update status
    meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
        Type:               "Ready",
        Status:             metav1.ConditionTrue,
        Reason:             "Reconciled",
        ObservedGeneration: obj.Generation,
        Message:            "All resources are in sync.",
    })
    obj.Status.ObservedGeneration = obj.Generation
    if err := r.Status().Update(ctx, obj); err != nil {
        return ctrl.Result{}, err
    }

    return ctrl.Result{}, nil
}
```

##### Reconcile Return Conventions

| Return | Meaning |
|---|---|
| `ctrl.Result{}, nil` | Success — no requeue needed until a watched event fires |
| `ctrl.Result{}, err` | Transient error — requeued with exponential backoff |
| `ctrl.Result{RequeueAfter: d}, nil` | Requeue after duration `d` (e.g., polling external state) |
| `ctrl.Result{Requeue: true}, nil` | Requeue immediately (avoid; prefer event-driven) |

> **Rule:** Return `ctrl.Result{}, err` for errors so the work queue applies backoff. Only use `RequeueAfter` for intentional polling patterns (e.g., waiting for an external API).

##### Manager Setup

```go
func (r *RedisClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&cachev1.RedisCluster{}).
        Owns(&appsv1.Deployment{}).   // watch owned resources
        Owns(&corev1.Service{}).
        WithOptions(controller.Options{
            MaxConcurrentReconciles: 5,
        }).
        Complete(r)
}
```

---

#### Watches & Predicates

##### Use Predicates to Reduce Noise

```go
// Only reconcile when spec actually changes (generation increment)
ctrl.NewControllerManagedBy(mgr).
    For(&cachev1.RedisCluster{},
        builder.WithPredicates(predicate.GenerationChangedPredicate{}),
    ).
    Complete(r)
```

##### Custom Predicate Example

```go
var annotationPredicate = predicate.Funcs{
    UpdateFunc: func(e event.UpdateEvent) bool {
        // Only trigger if generation OR finalizers changed
        return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration() ||
            !reflect.DeepEqual(e.ObjectNew.GetFinalizers(), e.ObjectOld.GetFinalizers())
    },
    CreateFunc:  func(e event.CreateEvent) bool  { return true },
    DeleteFunc:  func(e event.DeleteEvent) bool  { return true },
    GenericFunc: func(e event.GenericEvent) bool { return false },
}
```

##### Enqueue Requests for Owned Objects

```go
// Watch Deployments owned by RedisCluster; reconcile the owner
Owns(&appsv1.Deployment{})
```

##### Fan-Out Watches (Many-to-One)

```go
// Watch a shared Secret; enqueue all CRs referencing it
Watches(
    &corev1.Secret{},
    handler.EnqueueRequestsFromMapFunc(r.findCRsForSecret),
    builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
)
```

---

#### Finalizers

##### When to Use

Use finalizers **only** when the controller manages resources outside of Kubernetes (cloud APIs, databases, DNS records) that must be explicitly cleaned up before the CR is deleted.

> Do **not** use finalizers for in-cluster resources already covered by `ownerReferences`.

##### Naming Rule

Finalizer names **must** be fully qualified:

```
example.com/redis-external-cleanup    ✅
cleanup                               ❌  (not qualified)
```

##### Finalizer Lifecycle Pattern

```go
const myFinalizer = "cache.example.com/cleanup"

func (r *Reconciler) handleDeletion(ctx context.Context, obj *cachev1.RedisCluster) (ctrl.Result, error) {
    if !controllerutil.ContainsFinalizer(obj, myFinalizer) {
        return ctrl.Result{}, nil
    }

    // Perform idempotent cleanup
    if err := r.cleanupExternalResources(ctx, obj); err != nil {
        return ctrl.Result{}, err // retry
    }

    // Remove finalizer — triggers actual deletion
    controllerutil.RemoveFinalizer(obj, myFinalizer)
    return ctrl.Result{}, r.Update(ctx, obj)
}

// Add finalizer at initialization
func (r *Reconciler) isInitialized(obj *cachev1.RedisCluster) bool {
    if !controllerutil.ContainsFinalizer(obj, myFinalizer) {
        controllerutil.AddFinalizer(obj, myFinalizer)
        return false
    }
    return true
}
```

##### Rules

- Cleanup logic **must be idempotent** — the reconcile loop may be called multiple times while `DeletionTimestamp` is set.
- Never add a new finalizer after `DeletionTimestamp` is set — only removal is allowed.
- Do not silently swallow errors; failing cleanup should be retried.
- Do not create resources inside a namespace that has `DeletionTimestamp` set.

---

#### Owner References & Garbage Collection

##### Set Owner References for All Child Resources

```go
// controllerutil.SetControllerReference sets ownerReferences and blockOwnerDeletion=true
if err := controllerutil.SetControllerReference(owner, childDeployment, r.Scheme); err != nil {
    return err
}
```

##### Rules

| Rule | Notes |
|---|---|
| Owner must be in **same namespace** as the owned resource | Enforced by Kubernetes GC |
| A namespaced owner **cannot** own a cluster-scoped resource | Treated as absent owner → deletion |
| A cluster-scoped owner **can** own a cluster-scoped resource | Permitted |
| Use `controllerutil.SetControllerReference` (not manual ownerRef) | Sets `controller: true` and `blockOwnerDeletion: true` |
| One resource should have **at most one controller owner** | Multiple owners only for non-controller (informational) references |

##### When to Use Finalizers vs. OwnerReferences

| Scenario | Use |
|---|---|
| Child resource in same namespace | `ownerReferences` (automatic GC) |
| Child resource is cluster-scoped | `ownerReferences` + `controllerutil.SetControllerReference` |
| External resource (cloud, DNS) | `finalizer` |
| Cross-namespace cleanup needed | `finalizer` |

---

#### Validation & Admission Webhooks

##### Validation Hierarchy (prefer left-to-right)

```
CEL XValidation markers  →  OpenAPI schema markers  →  Validating Webhook
(no extra process)            (no extra process)          (extra infra needed)
```

##### When Webhooks Are Needed

- **MutatingWebhook** — Defaulting complex fields that CEL cannot express; injecting sidecars.
- **ValidatingWebhook** — Cross-resource validation (e.g., quota checks), user-identity-based rules.

##### Webhook Rules

```go
// +kubebuilder:webhook:path=/validate-cache-example-com-v1-rediscluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=cache.example.com,resources=redisclusters,verbs=create;update,versions=v1,name=vrediscluster.kb.io,admissionReviewVersions=v1
```

- Set `failurePolicy: Fail` for critical validation; `Ignore` only if the webhook is non-essential.
- Set `sideEffects: None` unless you have side effects (required for webhooks that need dry-run support).
- Set `matchPolicy: Equivalent` so the webhook fires on all API versions of the same resource.
- Always handle `CREATE` **and** `UPDATE` for immutability rules; `DELETE` for cleanup checks.
- Use `timeoutSeconds: 10` (max 30); fast webhooks maintain cluster stability.
- TLS certificates **must** be rotated; use cert-manager or OLM-managed certs.

##### Immutability Example (prefer CEL)

```go
// +kubebuilder:validation:XValidation:rule="self.storageClass == oldSelf.storageClass",message="storageClass is immutable after creation"
StorageClass string `json:"storageClass"`
```

---

#### RBAC & Least Privilege

##### Principles

1. **Least privilege** — grant only the verbs/resources explicitly required.
2. Prefer `Role` + `RoleBinding` (namespaced) over `ClusterRole` + `ClusterRoleBinding`.
3. **No wildcard verbs** (`*`) or **wildcard resources** (`*`).
4. **No `cluster-admin`** service accounts.
5. Mount service account tokens only when the operator needs API access (`automountServiceAccountToken: false` as default).

##### RBAC Marker Pattern (Kubebuilder)

```go
// Core resources the operator reads
// +kubebuilder:rbac:groups="",resources=pods;services;configmaps;secrets,verbs=get;list;watch

// CRD the operator owns
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.example.com,resources=redisclusters/finalizers,verbs=update

// Apps it manages
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete

// Events for observability
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
```

##### Service Account Configuration

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: redis-operator
  namespace: redis-system
automountServiceAccountToken: false  # Opt-in only

---
apiVersion: v1
kind: Pod
spec:
  serviceAccountName: redis-operator
  automountServiceAccountToken: true  # Explicitly opt in at pod level
```

---

#### Security Hardening

##### Operator Pod Security Context

```yaml
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
    seccompProfile:
      type: RuntimeDefault

  containers:
    - name: manager
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: ["ALL"]
        runAsNonRoot: true
```

##### Pod Security Standards

Operator pods should comply with at minimum the **Restricted** profile:

```yaml
# Namespace label
pod-security.kubernetes.io/enforce: restricted
pod-security.kubernetes.io/warn: restricted
```

##### Secrets Management

- Never embed secrets in CRD `spec` fields.
- Reference secrets by name: `secretName: my-secret`.
- Do not log secret values.
- Use projected volumes for short-lived tokens rather than long-lived static secrets.
- Consider External Secrets Operator or Vault Agent for rotation.

##### Network Policies

```yaml
# Restrict operator egress to API server only
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: redis-operator-egress
spec:
  podSelector:
    matchLabels:
      control-plane: controller-manager
  policyTypes: [Egress]
  egress:
    - ports:
        - port: 443    # Kubernetes API server
          protocol: TCP
        - port: 6443
          protocol: TCP
```

##### Image Security

- Use **distroless** or **scratch** base images for the operator binary.
- Pin images to SHA256 digest in production manifests.
- Scan images with Trivy, Grype, or Snyk in CI.
- Never use `latest` tag.

---

#### Observability

##### Structured Logging (logr / zap)

```go
// ✅ Structured key-value pairs — never fmt.Sprintf for log fields
log := log.FromContext(ctx).WithValues(
    "namespace", req.Namespace,
    "name", req.Name,
    "generation", obj.Generation,
)
log.Info("Starting reconcile")
log.Error(err, "Failed to create Deployment", "deployment", deploymentName)

// ❌ Avoid
log.Info(fmt.Sprintf("reconciling %s/%s", req.Namespace, req.Name))
```

Log levels:
- `V(0)` / `Info` — significant lifecycle events
- `V(1)` — per-reconcile summaries
- `V(2)` — detailed decision logging
- `V(4)` — trace / dump level

##### Prometheus Metrics

Use the controller-runtime built-in metrics endpoint (`:8080/metrics`):

```go
// Register custom metrics
var reconcileErrors = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "redis_operator_reconcile_errors_total",
        Help: "Total number of reconcile errors.",
    },
    []string{"namespace", "name"},
)

func init() {
    metrics.Registry.MustRegister(reconcileErrors)
}
```

**Standard metrics to expose:**
- `*_reconcile_total` — reconcile invocations
- `*_reconcile_errors_total` — error count
- `*_reconcile_duration_seconds` — histogram of reconcile latency
- `*_managed_resources` — gauge of CRs under management

##### Health Probes

```go
mgr, _ := ctrl.NewManager(cfg, ctrl.Options{
    HealthProbeBindAddress: ":8081",
})

mgr.AddHealthzCheck("healthz", healthz.Ping)                   // liveness
mgr.AddReadyzCheck("readyz", mgr.GetCache().WaitForCacheSync)  // readiness
```

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8081
  initialDelaySeconds: 15
  periodSeconds: 20
readinessProbe:
  httpGet:
    path: /readyz
    port: 8081
  initialDelaySeconds: 5
  periodSeconds: 10
```

##### Events

Emit Events for user-visible state changes (not every reconcile):

```go
r.Recorder.Event(obj, corev1.EventTypeWarning, "ReconcileError",
    fmt.Sprintf("Failed to create Deployment: %v", err))
r.Recorder.Event(obj, corev1.EventTypeNormal, "Reconciled",
    "Successfully synced all resources")
```

- Use `Warning` for errors requiring user attention.
- Use `Normal` for successful transitions.
- Do not emit Events on every reconcile — only on state changes.

---

#### Error Handling & Requeue Strategy

##### Error Classification

```go
// Permanent error — user must fix; do not retry automatically
type PermanentError struct{ Err error }
func (e PermanentError) Error() string { return e.Err.Error() }

// Transient error — retry with backoff
return ctrl.Result{}, fmt.Errorf("API temporarily unavailable: %w", err)

// Permanent (do not requeue)
return ctrl.Result{}, nil  // After setting a Degraded condition
```

##### Backoff Requeue Pattern

```go
func (r *Reconciler) ManageError(ctx context.Context, obj *cachev1.RedisCluster, err error) (ctrl.Result, error) {
    meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
        Type:               "Ready",
        Status:             metav1.ConditionFalse,
        Reason:             "ReconcileError",
        Message:            err.Error(),
        ObservedGeneration: obj.Generation,
    })
    _ = r.Status().Update(ctx, obj)

    r.Recorder.Event(obj, corev1.EventTypeWarning, "ReconcileError", err.Error())
    return ctrl.Result{}, err  // controller-runtime applies exponential backoff
}
```

##### Rules

- **Always** return the error from `Reconcile` for transient failures — do not swallow errors.
- **Never** return `ctrl.Result{Requeue: true}, err` — choose one or the other.
- For permanent errors (bad user input), set a `Stalled` condition, record an Event, and return `ctrl.Result{}, nil` to avoid infinite retry loops.
- Use `client.IgnoreNotFound(err)` when fetching objects that may not exist.
- Apply backoff cap of ~6 hours for long-running error states (align with event TTL).

---

#### Testing

##### Testing Pyramid

```
                    ┌────────────────────┐
                    │    e2e (kind/k3s)  │  <- full operator in real cluster
                    ├────────────────────┤
                    │  integration (envtest) │ <- real API server, no network
                    ├────────────────────┤
                    │   unit tests       │  <- pure logic, fake client
                    └────────────────────┘
```

##### Unit Tests (fake client)

```go
func TestReconcile_CreatesDeployment(t *testing.T) {
    scheme := runtime.NewScheme()
    _ = cachev1.AddToScheme(scheme)
    _ = appsv1.AddToScheme(scheme)

    cr := &cachev1.RedisCluster{
        ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
        Spec:       cachev1.RedisClusterSpec{Replicas: 3},
    }
    fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr).Build()
    r := &RedisClusterReconciler{Client: fakeClient, Scheme: scheme}

    _, err := r.Reconcile(context.TODO(), reconcile.Request{
        NamespacedName: types.NamespacedName{Name: "test", Namespace: "default"},
    })
    require.NoError(t, err)

    dep := &appsv1.Deployment{}
    require.NoError(t, fakeClient.Get(context.TODO(),
        types.NamespacedName{Name: "test", Namespace: "default"}, dep))
    assert.Equal(t, int32(3), *dep.Spec.Replicas)
}
```

##### Integration Tests (envtest)

```go
var _ = Describe("RedisCluster Controller", func() {
    It("should create a Deployment", func() {
        cr := &cachev1.RedisCluster{...}
        Expect(k8sClient.Create(ctx, cr)).To(Succeed())
        Eventually(func() bool {
            dep := &appsv1.Deployment{}
            err := k8sClient.Get(ctx, types.NamespacedName{...}, dep)
            return err == nil
        }, timeout, interval).Should(BeTrue())
    })
})
```

##### e2e Tests (kind / k3s)

- Deploy the full operator image.
- Test the complete lifecycle: Create → Reconcile → Update → Delete with finalizer.
- Use `kubectl wait --for=condition=Ready` assertions.

##### Testing Rules

- Test idempotency: run `Reconcile` twice and assert the same outcome.
- Test deletion path including finalizer removal.
- Test that status is updated correctly on both success and error paths.
- Test that invalid CRs are rejected by webhooks.

---

#### Quick-Reference Checklists

##### CRD Design Checklist

- [ ] Kind: CamelCase singular; Resource: lowercase plural
- [ ] API group is a DNS subdomain you own (not `*.k8s.io`)
- [ ] `// +kubebuilder:subresource:status` marker present
- [ ] `spec` contains only desired-state fields
- [ ] `status` has `conditions []metav1.Condition` + `observedGeneration int64`
- [ ] All enum fields use string type aliases (not `bool`)
- [ ] No floating-point fields in `spec`
- [ ] OpenAPI/CEL validation markers on all constrained fields
- [ ] At least one `// +kubebuilder:printcolumn` with state info
- [ ] `omitempty` on all optional fields; `// +optional` marker present

##### Controller Checklist

- [ ] Fetch → guard `NotFound` → handle deletion → reconcile → update status
- [ ] Return `ctrl.Result{}, err` for transient errors
- [ ] Set `observedGeneration` in status after every reconcile
- [ ] Use `Status().Update()` — never `Update()` for status changes
- [ ] Idempotent reconcile (safe to run N times)
- [ ] `GenerationChangedPredicate` or equivalent to skip status-only updates
- [ ] Structured logging with key-value pairs via `logr`
- [ ] Events emitted on state transitions (not on every reconcile)

##### Security Checklist

- [ ] Operator pod: `runAsNonRoot: true`, `allowPrivilegeEscalation: false`
- [ ] Operator pod: `capabilities.drop: [ALL]`, `readOnlyRootFilesystem: true`
- [ ] RBAC: no wildcard verbs/resources; namespace-scoped where possible
- [ ] Service account: `automountServiceAccountToken: false` at SA level; opt-in at pod
- [ ] Secrets referenced by name — never inlined in CRD spec
- [ ] NetworkPolicy restricting operator egress to API server only
- [ ] Image pinned to SHA256 digest; scanned in CI

##### Finalizer Checklist

- [ ] Finalizer name is fully qualified (`domain.com/name`)
- [ ] Finalizer added during initialization, before any external resource creation
- [ ] Cleanup logic is idempotent
- [ ] Finalizer removed only after successful cleanup
- [ ] Permanent cleanup failures result in a `Stalled` condition + Event

##### Versioning Checklist

- [ ] New fields added as optional with `omitempty`
- [ ] No fields removed or renamed in served versions
- [ ] Deprecation warnings set in CRD `spec.versions[].deprecationWarning`
- [ ] Conversion webhook present when schemas diverge
- [ ] `storage: true` set on exactly one version
- [ ] Old versions kept served for ≥ 1 year after deprecation
