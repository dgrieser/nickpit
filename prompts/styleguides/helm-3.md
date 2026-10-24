### Helm 3: Charts, Releases and CI — Complete Review Guide

> **Version**: Helm **v3.0.0** was released on **2019-11-13**. The last Helm 3 minor release is **v3.22.0 (2026-09-09**; the GitHub release is timestamped 2026-09-10 UTC), which contains only dependency and Kubernetes client-library updates plus bug fixes. The latest release as of 2026-10-22 is **v3.22.1 (2026-10-15)**; v3.22.2 is scheduled for 2026-11-11.
> **Support status**: Helm 3 is in its final, **security-only** phase. The Helm 4 announcement originally planned bug fixes until 2026-07-08 and security fixes until 2026-11-11; the Helm v3 end-of-life post (2026-06-02) moved the last minor to 2026-09-09 and extended **security fixes until 2027-02-10**. Bug fixes ended with 3.22.0. After 2027-02-10 Helm 3 receives no updates at all — no security patches and no Kubernetes client-library updates — and the maintainers "strongly encourage all users to migrate to Helm 4 before that date". The current major is Helm 4 (latest v4.3.1, 2026-10-15). Plan the Helm 4 migration now (see "Migrating to Helm 4").
> **Kubernetes skew**: Helm 3 supports the Kubernetes minor it was compiled against and three older ones (n-3): **3.22.x → 1.37–1.34**, 3.21.x → 1.36–1.33, 3.20.x → 1.35–1.32, 3.19.x → 1.34–1.31, 3.18.x → 1.33–1.30, 3.17.x → 1.32–1.29, 3.16.x → 1.31–1.28. Because there is no 3.23, clusters newer than 1.37 are outside Helm 3's support range.
> **Scope**: Helm charts (`Chart.yaml`, `Chart.lock`, `values.yaml`, `values.schema.json`, `templates/*.yaml`, `_helpers.tpl`, `templates/tests/`, `NOTES.txt`, `crds/`), library charts, chart CI (lint/template/package/push, OCI publishing, signing) and `helm` CLI usage in scripts and pipelines, for projects that pin **Helm 3.x** (`.tool-versions` `helm 3.x`, `azure/setup-helm` with `version: v3.x`, `alpine/helm:3.x` images, `HELM_VERSION=3.x`).
> **Note**: This guide is self-contained. Sections marked **[3.x]** describe behaviour that arrived in a specific Helm 3 minor; **Helm 4 only** marks things the reviewer must not suggest. Every chart example was rendered with `helm lint --strict`/`helm template`/`helm package` on **v3.22.1**; install/upgrade/rollback/uninstall semantics were exercised against a real **kube-apiserver v1.36.2** (no kubelet, scheduler or controllers, so Pods never run and Deployments never become available: wait behaviour on Pods is taken from the Helm 3.22.1 source and confirmed only as "times out"; Job completion was simulated by writing the Job status subresource). Version boundaries were re-checked on the official binaries of the latest patch of every Helm 3 minor (v3.0.3 … v3.21.4) plus v3.1.0, v3.3.0, v3.4.0, v3.11.0, v3.19.0 and v3.20.0, and Helm-4-only items were confirmed with v4.3.1 (v4.0.5 and v4.1.0 where the Helm 4 minor matters). Behaviour against vendor-suffixed Kubernetes versions (`v1.36.2-eks-…`) was checked through a proxy that rewrites the apiserver's `/version` response.

---

#### What's New Across Helm 3 (3.0 → 3.22)

##### Helm 3 fundamentals (3.0)

Helm 3.0 removed Tiller and changed the release model:

- **Client only.** `helm` talks to the Kubernetes API with the user's kubeconfig credentials; RBAC of the deploying identity is the only access control.
- **Releases are namespaced** and stored as Secrets of type `helm.sh/release.v1` named `sh.helm.release.v1.<release>.v<revision>` in the release namespace (`HELM_DRIVER=secret` is the default; `configmap`, `memory` and `sql` exist). Each revision Secret contains the chart, the user-supplied values and the rendered manifest (gzip + base64): anyone who can read Secrets in that namespace can read every value passed to Helm.
- **Three-way strategic merge patches** for upgrades and rollbacks (old manifest, live object, new manifest; section 5).
- **Chart `apiVersion: v2`**: dependencies move from `requirements.yaml` into `Chart.yaml`, `Chart.lock` replaces `requirements.lock`, `type: library` charts exist, `crds/` replaces the `crd-install` hook, and `values.schema.json` validates values.
- `-n` is `--namespace`; `.Release.Time` is gone (use `now`); there is no default chart repository.

##### Minor releases

| Minor | Released | Notable for reviewers |
|---|---|---|
| 3.1 | 2020-02-13 | `lookup` template function; `--post-renderer` (executable); `helm lint --with-subcharts`; `--disable-openapi-validation` |
| 3.2 | 2020-04-22 | `helm install --create-namespace`; `fromYamlArray`, `fromJsonArray`; `upgrade --skip-crds`; `helm.sh/resource-policy: keep` honoured on upgrade; `Chart.lock` packaged; fixes a critical `lookup` vulnerability in 3.1.x |
| 3.3 | 2020-08-11 | Upgrading a release whose last revision failed is allowed; `.Capabilities.HelmVersion`; `Chart.yaml` lint type checks for `version`/`appVersion` |
| 3.4 | 2020-10-26 | `helm list --selector`/`--time-format`; `helm status --show-desc`; impersonation flags (`--kube-as-user`, `--kube-as-group`); `helm repo add` no longer overwrites an existing repository unless `--force-update` is given (`--no-update` is ignored; breaking, first shipped in the 3.3.2 security release); warnings for the retired `stable`/`incubator` repositories |
| 3.5 | 2021-01-13 | `--wait-for-jobs`; Sprig 3.2 functions; OCI dependencies (`oci://`, still experimental); `helm dependency build --skip-refresh`; `--kube-ca-file` |
| 3.6 | 2021-05-27 | `--kube-version` for `helm template`; stricter `Chart.yaml` metadata validation (SemVer, printable characters); Apple silicon builds. **3.6.1**: repository credentials are no longer sent to other hosts unless `--pass-credentials` is given (CVE-2021-32690) |
| 3.7 | 2021-09-15 | OCI rework (`helm push`, `helm pull oci://`, new manifest format; older experimental OCI charts must be re-pushed); `helm uninstall --wait`; `helm show crds`; `.Chart.IsRoot`; `helm repo add --password-stdin` |
| 3.8 | 2022-01-24 | **OCI support GA** (no `HELM_EXPERIMENTAL_OCI`); version ranges for OCI dependencies; `+` in SemVer replaced by `_` in OCI tags; release names validated on install |
| 3.9 | 2022-05-18 | `--post-renderer-args`; `helm lint --quiet`; `upgrade --dependency-update` honoured |
| 3.10 | 2022-09-21 | **`--set-json`**; `helm list --no-headers`; `--burst-limit` |
| 3.11 | 2023-01-18 | `helm status --show-resources`; comma-separated `template --api-versions`. **3.11.1**: `getHostByName` returns `""` unless `--enable-dns` is passed (CVE-2023-25165) |
| 3.12 | 2023-05-10 | **`--set-literal`**; `helm uninstall --cascade`; `--insecure-skip-tls-verify`/custom CA for OCI; OCI annotations on pushed charts |
| 3.13 | 2023-09-27 | **`--dry-run=client\|server\|none`** (server mode runs `lookup`); `helm get metadata`; `--labels` on install/upgrade; `--plain-http` for OCI; `helm repo index --json`; values precedence and `null` handling made consistent (regression fixed in 3.13.1) |
| 3.14 | 2024-01-17 | **`--reset-then-reuse-values`**; `helm lint --kube-version`; `helm search --fail-on-no-result`; `--qps`/`HELM_QPS`; nested `tpl` sees outer `define`s; dependency names/aliases must be unique; readiness checks verify `observedGeneration` |
| 3.15 | 2024-05-15 | **`--hide-secret`** (with `--dry-run`); more robust wait checks; OCI getter proxy support |
| 3.16 | 2024-09-11 | **`--skip-schema-validation`** (install/upgrade/template/lint); `sha512sum`; `--hide-notes`; proxy environment respected on install/upgrade |
| 3.17 | 2025-01-15 | **Install/pull by OCI digest**; **`--take-ownership`**; `toYamlPretty`; `fromToml`; `helm get metadata` shows annotations and dependencies; username/password for `helm push` and dependency commands |
| 3.18 | 2025-05-19 | **JSON Schema draft 2020-12** (new validator `santhosh-tekuri/jsonschema/v6`); **`helm.sh/hook-output-log-policy`**; ORAS v2 with automatic HTTP fallback; CPU/memory profiling via `HELM_PPROF_CPU_PROFILE`/`HELM_PPROF_MEM_PROFILE`; several OCI regressions fixed in 3.18.1–3.19.0 |
| 3.19 | 2025-09-11 | `helm create` scaffolds a Gateway API `HTTPRoute`; `helm lint` accepts `http(s)` `$ref` in schemas again (3.18 regression); all hook deletions are processed on failure; `--kube-version` keeps the given form (`1.31` → `v1.31`) and drops vendor suffixes (`1.31.2-eks-…` → `v1.31.2`) |
| 3.20 | 2026-01-21 | `helm uninstall --keep-history` supersedes earlier deployed revisions; unresolved URN `$ref`s in `values.schema.json` warn and are ignored; Kubernetes 1.35 libraries; Go 1.25. **3.20.1**: user `null` preserved under an empty-map chart default; pulling charts from OCI image indexes fixed. Its notes also list a tag+digest backport (#31601), but that commit is in no v3 tag: the bug was Helm 4-only, and 3.17.4 and 3.20.0 already resolve `chart:1.4.0@sha256:…` (verified against a local registry). **3.20.2**: CVE-2026-35206 |
| 3.21 | 2026-05-14 | Kubernetes 1.36 libraries; nil/empty-map fix and OCI index pull fix carried forward; 3.21.1 moves to Go 1.26 and fixes a `helm template` panic |
| 3.22 | 2026-09-09 | Last Helm 3 minor: Kubernetes 1.37 libraries, `pull,push` token scope for registry pushes, registry credentials kept on the plain-HTTP fallback, rollback tolerates resources that no longer exist, `Files.Lines` panic on empty files and a nil-client panic during CRD installation fixed, provenance code moved to `ProtonMail/go-crypto`, dependency CVE bumps. 3.22.1 (2026-10-15): dependency/CVE updates only |

Boundary checks executed for this guide (flags and functions fail with "unknown flag" or "function … not defined" on the older binary; the other rows show the old behaviour in parentheses):

| Feature | Absent in | Present in |
|---|---|---|
| `appVersion`/`version` must be strings (`helm lint`) | 3.2.4 | 3.3.0 |
| `--set-json` | 3.9.4 | 3.10.3 |
| `--enable-dns` | 3.11.0 | 3.11.3 |
| `--set-literal` | 3.11.3 | 3.12.3 |
| `--dry-run=server` | 3.12.3 (`strconv.ParseBool: parsing "server"`) | 3.13.3 |
| `--reset-then-reuse-values`, `lint --kube-version` | 3.13.3 | 3.14.4 |
| `--hide-secret` | 3.14.4 | 3.15.4 |
| `--skip-schema-validation`, `--hide-notes`, `sha512sum` | 3.15.4 | 3.16.4 |
| `--take-ownership`, `toYamlPretty`, `fromToml`, `oci://…@sha256:` | 3.16.4 (`invalid_reference: invalid tag`) | 3.17.4 |
| JSON Schema 2020-12 keywords (`prefixItems`, `unevaluatedProperties`) | 3.17.4 (silently mis-validates, see section 16) | 3.18.6 |
| `--kube-version 1.31.2-eks-…` stripped to `v1.31.2` | 3.18.6 (suffix kept) | 3.19.0 |
| Unresolved URN `$ref` in `values.schema.json` warns and is ignored | 3.18.6 (`no URLLoader registered`) | 3.20.0 |
| User `null` under an empty-map default kept | 3.20.0 (key dropped) | 3.20.2, 3.22.1 |

***

#### Security Releases

Pin at least the patch that fixes each advisory relevant to the project; on Helm 3 that now means **3.22.1** (only the latest minor is patched).

| Advisory | Fixed in | Issue |
|---|---|---|
| CVE-2021-32690 | 3.6.1 | Repository credentials were sent to other domains listed in `index.yaml`; now only with `--pass-credentials` |
| CVE-2022-36055 | 3.9.4 | Crafted `--set`/`strvals` input causes a denial of service (panic) |
| CVE-2022-23524 / CVE-2022-23525 / CVE-2022-23526 | 3.10.3 | Stack overflow or panic from crafted `strvals` input, repository `index.yaml` or chart schema files |
| CVE-2023-25165 | 3.11.1 | `getHostByName` could leak information during rendering; DNS lookups now require `--enable-dns` |
| CVE-2024-25620 | 3.14.1 | Path traversal when saving a chart whose `name` contains a relative path |
| CVE-2024-26147 | 3.14.2 | Panic when an `index.yaml` or a plugin's `plugin.yaml` lacks expected metadata (a malicious plugin breaks every Helm command) |
| CVE-2025-32386 / CVE-2025-32387 | 3.17.3 | Chart archive decompression bomb (OOM) / deeply nested schema `$ref` stack overflow |
| CVE-2025-53547 | 3.17.4, 3.18.4 | `helm dependency update` with a malicious `Chart.yaml` and a symlinked `Chart.lock` writes attacker content to the symlink target (local code execution) |
| CVE-2025-55198 / CVE-2025-55199 | 3.18.5 | Panic on malformed `Chart.yaml`/`index.yaml`; schema `$ref` to `/dev/zero` exhausts memory |
| CVE-2026-35206 | 3.20.2 | A chart named `.` or `..` makes `helm pull --untar` extract into the output directory itself |

Review rule: CI that runs `helm dependency update`, `helm lint` or `helm template` on charts from forks or untrusted sources must use a patched Helm (≥ 3.18.5 for the schema/YAML DoS fixes, ≥ 3.20.2 for the untar fix).

***

#### Do Not Use or Recommend

##### Helm 4 only — never suggest these for a Helm 3 project

Each was confirmed absent in v3.22.1 (unknown flag, parse error or ignored); the flags, subcommands and functions were confirmed present in v4.3.1:

| Helm 4 feature | Helm 3.22.1 behaviour | Helm 3 equivalent |
|---|---|---|
| Server-side apply, `--server-side`, `--force-conflicts` | unknown flag | none; Helm 3 always uses client-side three-way merge (section 5) |
| `--wait=watcher`, `--wait=legacy`, `--wait=hookOnly` (kstatus) | `invalid argument "watcher" for "--wait" flag: strconv.ParseBool` | boolean `--wait` (section 6) |
| `--rollback-on-failure` | unknown flag | `--atomic` |
| `--force-replace` | unknown flag | `--force` |
| Post-renderer as a plugin name (`postrenderer/v1`) | the argument is treated as an executable path | `--post-renderer ./script.sh` |
| Plugin `apiVersion: v1` format, `extism/v1` WebAssembly runtime, `helm plugin package`/`verify`, signed plugin tarballs | `json: unknown field "apiVersion"` when loading such a plugin; no `package`/`verify` subcommands | legacy `plugin.yaml` |
| Multi-document values files (`---`) | **silently uses only the first document** | one document per file; several `-f` files |
| `--set-json '{"a":{"b":1}}'` (whole JSON object) | `failed parsing --set-json data` | `--set-json 'a={"b":1}'` |
| `mustToYaml`, `mustToToml`, `durationSeconds`/`mustToDuration` and the other `duration*` helpers | `function "…" not defined` (parse error) | `toYaml`; validate inputs with the schema. Note that `mustToJson`, `mustToPrettyJson`, `mustToRawJson` **are** available in Helm 3 (they come from Sprig) |
| `helm rollback --description`, `helm history --show-rollback-revision`, `helm repo list --no-headers`, `--color`/`NO_COLOR` handling, `--content-cache` | unknown flag | — |
| `SOURCE_DATE_EPOCH` for reproducible `helm package` | ignored: every run embeds the current time (verified: four packages of the same chart, four digests) | none; publish once and pin the digest |
| `pubring.kbx` keyrings for `--verify`/`helm verify` | `failed to load keyring: openpgp: invalid data: tag byte does not have MSB set` | `gpg --export > pubring.gpg` |
| Uninstall ownership check (Helm 4.3) | Helm 3 deletes every object listed in the release manifest, even one another release has taken over | avoid shared object names |
| Chart `apiVersion: v3` | `helm lint`: `apiVersion 'v3' is not valid. The value must be either "v1" or "v2"` | `apiVersion: v2` |

##### Features newer than the pinned Helm 3 minor

A project pinned to an older 3.x must not use what its minor lacks. The most commonly suggested by mistake:

| Feature | Needs |
|---|---|
| `lookup` | 3.1 |
| `--create-namespace` | 3.2 |
| `.Capabilities.HelmVersion` | 3.3 |
| `--wait-for-jobs` | 3.5 |
| `--kube-version` on `helm template` | 3.6 |
| `.Chart.IsRoot`, `helm uninstall --wait`, `helm show crds` | 3.7 |
| OCI without `HELM_EXPERIMENTAL_OCI` | 3.8 |
| `--post-renderer-args` | 3.9 |
| `--set-json` | 3.10 |
| `--enable-dns` (and `getHostByName` returning `""` without it) | 3.11.1 |
| `--set-literal`, `uninstall --cascade` | 3.12 |
| `--dry-run=server`, `helm get metadata`, `--labels`, `--plain-http` | 3.13 |
| `--reset-then-reuse-values`, `helm lint --kube-version` | 3.14 |
| `--hide-secret` | 3.15 |
| `--skip-schema-validation`, `--hide-notes`, `sha512sum` | 3.16 |
| `--take-ownership`, `toYamlPretty`, `fromToml`, `oci://…@sha256:<digest>` | 3.17 |
| JSON Schema 2020-12 in `values.schema.json`, `helm.sh/hook-output-log-policy`, `HELM_PPROF_CPU_PROFILE`/`HELM_PPROF_MEM_PROFILE` | 3.18 |
| Unresolvable URN `$ref` in `values.schema.json` tolerated (warning) | 3.20 |

##### Deprecated or obsolete in Helm 3

- `apiVersion: v1` charts with `requirements.yaml` still load but are the Helm 2 format; new charts use `v2`.
- `crd-install` hooks are not supported (since 3.0); use `crds/`.
- `HELM_EXPERIMENTAL_OCI=1` is not needed since 3.8; the `helm chart save/push/pull/export` commands were removed in 3.7.
- `helm.sh/hook: test-success` is the Helm 2 spelling; use `test`.
- `kubernetes.io/ingress.class` annotation (use `spec.ingressClassName`) and `extensions/v1beta1`/`networking.k8s.io/v1beta1` Ingress (removed in Kubernetes 1.22).
- `helm rollback --recreate-pods` still exists in 3.22.1 but forces Pod deletion outside the Deployment's rollout strategy; use a pod-template checksum annotation instead.
- The `stable`/`incubator` repositories (`kubernetes-charts.storage.googleapis.com`) are retired.

***

#### Version-Aware Review

- **Find the pinned version first**: `.tool-versions` (`helm 3.22.1`), `mise.toml`, `azure/setup-helm` `with: version:`, container images (`alpine/helm:3.22.1`), `HELM_VERSION` in Makefiles or Dockerfiles, Renovate/Dependabot configuration. A claim like "not available" is only valid against the pinned minor.
- **An unpinned installer now installs Helm 4.** `azure/setup-helm` defaults to `latest`; since Helm 4 is the current major, a job without `version:` silently switches to Helm 4 and its different apply, wait and flag semantics (and if the version lookup fails, the action falls back to a hard-coded, outdated 3.x release). Flag a missing or `latest` version in any pipeline that expects Helm 3.
- **Say "not available in 3.x" only when the pinned version proves it** (unknown flag, `function "…" not defined`). Do not infer from the current helm.sh documentation: the default docs describe Helm 4. Helm 3 documentation lives under `https://helm.sh/docs/v3/`.
- **Charts are often consumed by both majors.** A chart that must work on Helm 3 may not use Helm 4-only functions (parse error on Helm 3) or multi-document `values.yaml` (silently truncated on Helm 3). Conversely, a chart written for Helm 3 can fail on Helm 4 because server-side apply rejects unknown fields and duplicate list keys (section 5); fix those even while on Helm 3.
- **`.Capabilities.HelmVersion.Version`** reports the running Helm (`v3.22.1`). A chart can require a minimum Helm 3 minor, but the guard cannot protect a template that references an unknown function: Go templates fail at parse time, before any `if` runs.

```yaml
{{- if not (semverCompare ">=3.16.0-0" .Capabilities.HelmVersion.Version) }}
{{- fail "this chart requires Helm >= 3.16.0" }}
{{- end }}
```

***

#### How Helm 3 Applies Changes: Client-Side Three-Way Merge

On `helm upgrade` and `helm rollback`, Helm 3 compares, for every object, the manifest of the **previous release revision**, the **live object** and the **new manifest**, and sends a PATCH (field manager `helm`, operation `Update`). It never uses server-side apply. Install creates objects (POST); objects that already exist and are not owned by the release fail the install (section 18).

##### Built-in kinds: strategic merge patch

Verified on the apiserver: after installing the example chart (`replicas: 2`), someone ran `kubectl scale --replicas=5`, added an annotation and appended a `sidecar` container. A plain `helm upgrade` with unchanged values produced:

```console
$ kubectl -n web get deploy r-web -o jsonpath='{.spec.replicas} {.metadata.annotations.extra\.example\.com/x} {range .spec.template.spec.containers[*]}{.name},{end}'
2 1 web,sidecar,
```

- Fields the chart renders are **reset to the chart's value** even when the chart did not change them (`replicas` went back from 5 to 2). Drift on chart-owned fields is silently overwritten, with no warning.
- Fields the chart never rendered (the extra annotation, the injected sidecar) are kept: they are not in the old manifest, so the patch does not delete them.
- A field removed from the chart is deleted from the live object, because it was in the old manifest.

Review consequences:

- **Do not render fields another controller owns.** With an HPA or KEDA, a rendered `spec.replicas` resets the scale on every deploy (a traffic spike plus a deploy equals an outage). Omit it when autoscaling is enabled:

```yaml
# BAD: every upgrade resets the HPA's replica count to replicaCount
spec:
  replicas: {{ .Values.replicaCount }}

# GOOD
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
```

- The same applies to webhook-filled `caBundle` fields (cert-manager's cainjector), `Service.spec.clusterIP`, and annotations written by tools: leave out what another component fills in.
- Hand edits survive only until they touch a chart-rendered field. Fix drift through the chart, not with `kubectl edit`.

##### Custom resources: two-way JSON merge patch

For objects without a built-in Go type (custom resources), Helm 3.22.1 builds a **JSON merge patch between the old and the new manifest only**; the live object is not consulted (`createPatch` in `pkg/kube/client.go`). Verified: after `kubectl patch gadget g1 -p '{"spec":{"size":5,"extra":"x"}}'`, a `helm upgrade` with an unchanged chart left `{"extra":"x","size":5}` — the chart's `size: 1` was **not** restored. Drift on custom resources is never repaired by Helm 3 unless the chart value itself changes. JSON merge patches also replace lists wholesale.

##### Duplicate keys in keyed lists

The apiserver accepts duplicate `env` names (with a warning), and a strategic merge patch then edits the wrong entry. Verified with a Deployment rendering `A=1`, `A=2`, `B=b`:

```console
$ helm install de ./d -n t
I1023 05:30:41.721959   10592 warnings.go:107] "Warning: spec.template.spec.containers[0].env[1]: hides previous definition of \"A\", which may be dropped when using apply"
$ helm upgrade de ./d -n t --set second=3      # changes the second "A" to 3
$ kubectl -n t get deploy de -o jsonpath='{.spec.template.spec.containers[0].env}'
[{"name":"A","value":"3"},{"name":"A","value":"2"},{"name":"B","value":"b"}]
```

The patch changed the **first** `A`; the container still sees `A=2` (the later definition wins), so the upgrade had no effect. Under Helm 4 the same install fails outright (`duplicate entries for key [name="A"]`). This typically happens when a chart appends user-supplied `extraEnv` to built-in variables: merge by name (a dict keyed by variable name) or document that overrides must not repeat built-in names.

##### Unknown fields are dropped with a warning

A misspelled or mis-indented field is pruned by the apiserver and the release succeeds:

```console
$ helm install t-unknown ./t -n t --set mode=unknown          # renders "datax:" instead of "data:"
I1023 05:30:20.369445    8169 warnings.go:107] "Warning: unknown field \"datax\""
STATUS: deployed
```

The ConfigMap is created without data. CI must treat `unknown field` warnings as errors (or validate rendered manifests with a schema validator); Helm 4 fails these installs, so they also block a later migration.

##### Duplicate objects fail the install — after partial creation

If two templates render the same kind/namespace/name, Helm 3 fails: `Error: INSTALLATION FAILED: 1 error occurred: * configmaps "t-dup-cm" already exists`. The first copy **is** created and the release is left `failed`. `helm lint` and `helm template` do not detect duplicates (verified: lint passes). Check names for collisions in review (two templates using `include "x.fullname"` without a suffix, subcharts with colliding `fullnameOverride`).

##### Upgrades are not transactional

Helm 3 applies objects one by one. When one fails, the ones already patched stay changed. Verified: an upgrade that added a version label to the selector labels failed on the Deployment (`spec.selector: ... field is immutable`), but the Service and PodDisruptionBudget had already been patched to the new selector:

```console
$ kubectl -n f get svc fr-web -o jsonpath='{.spec.selector}'
{"app.kubernetes.io/instance":"fr","app.kubernetes.io/name":"web","app.kubernetes.io/version":"2.7.1"}
```

The Service now selects no Pods (the running Pods lack the new label) while the release shows `failed`. Use `--atomic` in automated deploys (section 6) and review selector changes with extreme care.

##### `--force`

`helm upgrade --force` (and `helm rollback --force`) **replaces** each object with a full PUT of the rendered manifest instead of patching it. Verified: an annotation added with `kubectl annotate` disappeared after `--force`; the Service kept its `clusterIP` (the apiserver preserves allocated IPs on update). Every field the chart does not render — values written by controllers, webhooks or other tools — is reset, and changes to immutable fields still fail. `--force` is a repair tool, not a default deploy flag.

***

#### Waiting and Failure Handling: `--wait`, `--atomic`, `--cleanup-on-fail`

##### What Helm 3 waits for

Without `--wait`, `helm install`/`upgrade` returns as soon as the objects are accepted by the apiserver — but **hook Pods and Jobs are always waited for** (up to `--timeout`, default `5m0s`). With `--wait`, Helm 3.22.1 polls these readiness rules (`pkg/kube/ready.go`):

| Kind | Ready when |
|---|---|
| Pod | condition `Ready=True` |
| Deployment | `observedGeneration` matches and the **new ReplicaSet** has at least `replicas - maxUnavailable` ready Pods |
| StatefulSet | `observedGeneration` matches; updated replicas reach `replicas - partition`, **all** `replicas` are ready, and with `partition: 0` `currentRevision` equals `updateRevision`; `OnDelete` strategy is skipped |
| DaemonSet | `observedGeneration` matches; with `RollingUpdate`, ready Pods reach `desired - maxUnavailable`; `OnDelete` is skipped |
| ReplicaSet, ReplicationController | `observedGeneration` matches and every Pod matched by the selector is `Ready` |
| PersistentVolumeClaim | phase `Bound` |
| Service | has a ClusterIP; type `LoadBalancer` additionally needs `status.loadBalancer.ingress` (or `externalIPs`); `ExternalName` is ready immediately |
| CustomResourceDefinition | `Established` |
| Job | ready immediately, unless `--wait-for-jobs` is set: then `succeeded >= completions`, and an error once `failed > backoffLimit` |
| Any other kind (custom resources, Ingress, ConfigMap …) | ready immediately |
| Paused Deployment | ready |

Verified against the apiserver (no controllers, so nothing ever becomes ready): a regular Job with `--wait` → `deployed`; a `LoadBalancer` Service or an unbound PVC with `--wait --timeout 8s` → `Error: INSTALLATION FAILED: context deadline exceeded`; a custom resource whose `status.conditions` say `Ready=False` → `deployed` (Helm 3 ignores it); `--wait-for-jobs` **without** `--wait` → `deployed` (the flag only has an effect together with `--wait`).

Review consequences:

- `--wait` does not mean "fully rolled out": with the default `maxUnavailable: 25%` and `replicas: 4`, three ready new Pods satisfy it, and old Pods may still be terminating.
- Custom resources managed by operators (certificates, databases, Kafka topics) are never waited for; add an explicit check (`kubectl wait --for=condition=Ready …`) after the deploy if later steps depend on them.
- Regular (non-hook) Jobs need `--wait --wait-for-jobs` to be awaited.
- The error message is only `context deadline exceeded`; add `--debug` in CI to see which resource Helm was waiting for (`Deployment is not ready: ns/name. 0 out of 1 expected pods are ready`).

##### `--atomic`

```bash
# BAD: on failure the release stays half-applied (see "Upgrades are not transactional")
helm upgrade --install web ./chart -n web -f values/prod.yaml

# GOOD
helm upgrade --install web ./chart \
  --namespace web --create-namespace \
  -f values/prod.yaml \
  --atomic --timeout 10m
```

- `--atomic` implies `--wait`. On an install failure the release is uninstalled; on an upgrade failure Helm rolls back to the previous revision. Verified: `Error: INSTALLATION FAILED: release w2 failed, and has been uninstalled due to atomic being set: context deadline exceeded`.
- **The automatic rollback also waits, with the same `--timeout`.** If the previous revision cannot become ready either (cluster out of capacity, a dependency down), the rollback fails too and the release ends `failed` at a new revision:

```console
Error: UPGRADE FAILED: an error occurred while rolling back the release. original upgrade error: context deadline exceeded: release w3 failed: context deadline exceeded
REVISION  STATUS    DESCRIPTION
1         deployed  Install complete
2         failed    Upgrade "w3" failed: context deadline exceeded
3         failed    Release "w3" failed: context deadline exceeded
```

  (Revision 1 still shows `deployed` because the rollback never completed.)
- `--atomic` does not undo hooks: a `pre-upgrade` migration that ran is not reverted by the rollback (which runs `pre-rollback`/`post-rollback` hooks).
- `--atomic` is called `--rollback-on-failure` in Helm 4; do not rename it while on Helm 3.

##### `--cleanup-on-fail`

Deletes objects **created** by a failed upgrade. Verified: an upgrade that introduced a NetworkPolicy and then failed on another object left the NetworkPolicy behind without the flag and removed it with `--cleanup-on-fail`. Objects that were patched (not created) are not reverted; only `--atomic` does that.

##### Stuck releases: `pending-install`, `pending-upgrade`, `pending-rollback`

A killed CI job leaves the release in a `pending-*` state. Verified (helm killed with SIGKILL during `--wait`):

```console
$ helm list -n w                 # default: deployed and failed only — the stuck release is invisible
$ helm list -n w --pending
p1  w  1  ...  pending-install
$ helm upgrade --install p1 ./web -n w
Error: UPGRADE FAILED: another operation (install/upgrade/rollback) is in progress
$ helm rollback p1 -n w
Error: release has no 0 version
```

Recovery: for a stuck first install, `helm uninstall`; for a stuck upgrade, `helm rollback <release> <last-good-revision>` (find it with `helm history`). Do not delete `sh.helm.release.v1.*` Secrets by hand. Helm 3 handles SIGTERM on install/upgrade since 3.7 and marks the release failed; SIGKILL (CI timeouts that kill the process group hard) leaves it pending. Give CI steps a job timeout longer than `--timeout`.

- `helm list` shows only `deployed` and `failed` releases by default; `-a/--all` shows every status, `--pending`, `--superseded`, `--uninstalled` filter. It also returns at most 256 releases unless `-m/--max` is raised: scripts that iterate over releases must page or raise the limit.
- A release whose only revision failed can be upgraded (since 3.3); `helm upgrade --install` on it upgrades rather than reinstalling.
- `--history-max` (default 10) bounds stored revisions; `0` means unlimited and lets release Secrets accumulate.

***

#### Dry Runs, `helm template`, `lookup` and `.Capabilities`

##### Modes

Verified on v3.22.1 against a v1.36.2 apiserver with an existing Secret `lk-secret`:

| Command | Cluster needed | `lookup` | `.Capabilities.KubeVersion` | `APIVersions.Has "apps/v1/Deployment"` |
|---|---|---|---|---|
| `helm template` | no | empty | `v1.37.0` (built-in default) | `false` |
| `helm template --validate` | yes | empty | `v1.36.2` | `true` |
| `helm template --dry-run=server` [3.13] | yes | **real** | `v1.37.0` (still offline!) | `false` |
| `helm install --dry-run` / `--dry-run=client` | **yes** | empty | `v1.36.2` | `true` |
| `helm install --dry-run=server` [3.13] | yes | real | `v1.36.2` | `true` |

- Despite the help text ("it will not attempt cluster connections"), `helm install --dry-run` and `--dry-run=client` contact the cluster: without a reachable cluster they fail with `Kubernetes cluster unreachable`. Only `helm template` (without `--validate`/`--dry-run=server`) is fully offline.
- No dry-run mode sends objects to the apiserver for a server-side dry-run. Verified: a Pod with an integer env value and a ConfigMap with a misspelled field both passed `helm install --dry-run=server` (`STATUS: pending-install`) and only failed (or lost the field) on the real install. Validate rendered output with `kubectl apply --dry-run=server -f -` or an offline schema validator.
- `--hide-secret` [3.15] hides Secret contents in dry-run output and is rejected without `--dry-run`.

##### `lookup`

```yaml
{{- $existing := lookup "v1" "Secret" .Release.Namespace "lk-secret" }}
annotations:
  found: {{ empty $existing | ternary "no" "yes" | quote }}
```

- Returns an empty map when the object does not exist **or** when there is no cluster (`helm template`, `helm lint`, client dry runs).
- An empty name returns a list of all objects of that kind in the namespace (needs list RBAC; can be all Secrets of the namespace).
- GitOps tools that render with `helm template` semantics (Argo CD, Flux with some configurations) always take the "not found" branch. A chart must render a valid, stable manifest when `lookup` is empty.

##### `.Capabilities` offline

- Offline `.Capabilities.KubeVersion` is the client-go version Helm was built with: `v1.32.0` on 3.17.4, `v1.33.0` on 3.18.6, `v1.37.0` on 3.22.1. Gates on the Kubernetes version therefore change when CI upgrades Helm. Pass `--kube-version` explicitly.
- `--kube-version` keeps the given form since 3.19: `--kube-version 1.31` gives `.Capabilities.KubeVersion.Version = "v1.31"` on 3.22.1 (`"v1.31.0"` on 3.18.6). `semverCompare` still works (`semverCompare ">=1.28.0-0" "v1.31"` is `true`), but string comparisons do not.
- Since 3.19, Helm 3 strips vendor suffixes from `--kube-version`: `--kube-version 1.31.2-eks-7f9249a` gives `v1.31.2` on 3.19.0/3.22.1 (3.18.6 and earlier keep `v1.31.2-eks-7f9249a`). A live EKS/GKE apiserver reports the suffix, so on 3.19+ offline CI renders and the real deploy can disagree (section 11).
- Offline `.Capabilities.APIVersions` contains only `group/version` entries (`policy/v1` → `true`) but not `group/version/Kind` entries (`apps/v1/Deployment` → `false`); against a cluster both forms exist. Gate on `group/version`, and pass CRD-provided APIs with `--api-versions`:

```bash
helm template web ./chart --kube-version 1.36.0 \
  --api-versions monitoring.coreos.com/v1 \
  --api-versions gateway.networking.k8s.io/v1
```

```yaml
{{- if and .Values.metrics.serviceMonitor.enabled (.Capabilities.APIVersions.Has "monitoring.coreos.com/v1") }}
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
...
{{- end }}
```

##### `generateName`

Helm tracks objects by name; `metadata.generateName` without `metadata.name` fails even in `--dry-run=server` on 3.22.1: `could not get information about the resource ConfigMap "" in namespace "l": resource name may not be empty`. Flag it.

***

#### Post-Renderers and Plugins

##### Executable post-renderers

In Helm 3, `--post-renderer` takes a path to an executable that reads all rendered manifests on stdin and writes the result to stdout; `--post-renderer-args` [3.9] passes arguments.

```sh
#!/bin/sh
# ci/post-render.sh: append a suffix to every metadata.name (demonstration only)
sed 's/^  name: \(.*\)$/  name: \1-pr/'
```

```console
$ helm template web ./web --post-renderer ./ci/post-render.sh | grep -E '^  name:'
  name: web-pr
  name: web-pr
  name: web-secret-pr
  ...
  name: web-test-connection
  name: web-migrate
```

- **Hooks and test Pods are not post-rendered in Helm 3** (`web-test-connection` and the `web-migrate` hook Job above are unchanged). An overlay that adds labels, sidecars or security settings does not reach hook Jobs. Helm 4 passes hooks too, so overlays change behaviour on migration.
- Post-renderers see manifests after template-level references were resolved: renaming an object (as above) silently breaks every reference to it (`secretKeyRef` still points to `web-secret`).
- **Empty post-renderer output is not an error in Helm 3.** Verified on 3.22.1: a post-renderer that prints nothing (a failed `kustomize` piped into a `cat`, a script that swallows its input) makes `helm install` report `STATUS: deployed` with no objects created, and `helm upgrade` **deletes every object of the release** and still reports `deployed`; `helm template` silently prints only hooks and tests. Only a non-zero exit code fails the command. Make the script fail on empty output (`[ -s "$out" ] || exit 1`) and run it with `set -eu`; Helm 4 rejects empty output (`post-renderer … produced empty output`).
- Only one `--post-renderer`. Commit the script, pin the tools it calls (`kustomize`, `yq`).
- In Helm 4 `--post-renderer ./script.sh` fails (`plugin ... not found`): when preparing a migration, wrap the script as a `postrenderer/v1` plugin.

##### Plugins

- Helm 3 plugins use the legacy `plugin.yaml` (no `apiVersion`; `command`, `platformCommand`, `hooks`, `downloaders`). A single Helm 4 v1 plugin in the plugin directory breaks Helm 3.22.1 (verified): `helm plugin list` fails with `Error: failed to load plugin at ".../plugin.yaml": error unmarshaling JSON: while decoding JSON: json: unknown field "apiVersion"`, and every other command prints `failed to load plugins: …` and runs without any plugin. Do not share `HELM_PLUGINS`/`HELM_DATA_HOME` between Helm 3 and Helm 4 installations.
- `helm plugin install <url> --version <tag>` pins a version; plugins run with the user's credentials and are not signature-checked in Helm 3. Install only reviewed plugins, from pinned versions, in CI.
- `HELM_NO_PLUGINS=1` disables plugins for reproducible renders.

***

#### OCI Registries, Digests, Packaging and Provenance

##### Publishing to OCI [GA in 3.8]

```bash
set -euo pipefail
helm dependency build ./charts/web
helm lint --strict ./charts/web -f ci/values/prod.yaml
helm package ./charts/web -d dist/
echo "$REGISTRY_TOKEN" | helm registry login ghcr.io -u "$REGISTRY_USER" --password-stdin
helm push dist/web-1.4.0.tgz oci://ghcr.io/acme/charts
```

Verified against a local `registry:2`:

```console
$ helm push dist/web-1.4.0.tgz oci://127.0.0.1:5055/charts --plain-http
Pushed: 127.0.0.1:5055/charts/web:1.4.0
Digest: sha256:53e8b6f56e25dd8231c6b14c8f26b5693e0fa2cbd68715d3a6385c6d51b33bb3
```

- `helm push` requires a packaged `.tgz`. The tag is the chart `version` (`+` is replaced by `_`).
- `helm registry login` with `--password-stdin`; `-p/--password` prints `WARNING: Using --password via the CLI is insecure` and leaks the token into the process list and shell history. Helm 3 also accepts a repository path (`helm registry login registry.example.com/team`) — Helm 4 rejects it, so use the host only.
- Credentials are stored in `~/.config/helm/registry/config.json` (or `$HELM_REGISTRY_CONFIG`) as base64 `auth` — effectively plaintext. CI runners must not persist this file between jobs.
- `--plain-http` [3.13] and `--insecure-skip-tls-verify` belong only in local test setups.
- Without `--version`, `helm pull`/`install` of an OCI chart picks the highest SemVer tag.
- During `helm template`, `Pulled:`/`Digest:` messages go to stderr and do not pollute the rendered YAML (verified).

##### Pin deployments by digest [3.17]

```bash
# GOOD: immutable deploy input (Helm >= 3.17)
helm upgrade --install web oci://ghcr.io/acme/charts/web@sha256:<digest printed by helm push> -n web -f values/prod.yaml --atomic

# BAD: mutable tag only — the registry can serve different content for the same tag
helm upgrade --install web oci://ghcr.io/acme/charts/web --version 1.4.0
```

Verified on 3.22.1: `chart@sha256:<good>` and `chart:1.4.0@sha256:<good>` render; `chart:1.4.0@sha256:<wrong>` fails with `chart reference digest mismatch`. 3.16.4 rejects any digest reference (`invalid_reference: invalid tag`).

##### Packages are not reproducible in Helm 3

`helm package` stamps every archive entry with the current time, so packaging unchanged sources twice yields different sha256 digests (verified; `SOURCE_DATE_EPOCH` is ignored). Consequences: build and push a chart version **once**, record the digest from `helm push`, and deploy that digest; do not rebuild the "same" version in a later pipeline and expect the same digest. Never republish an existing version with different content.

##### Provenance (`.prov`)

```bash
gpg --export-secret-keys > secring.gpg      # Helm 3 needs legacy keyring files, not pubring.kbx
gpg --export > pubring.gpg
helm package ./web --sign --key 'Chart Signer' --keyring secring.gpg -d signed/
helm verify signed/web-1.4.0.tgz --keyring pubring.gpg
helm push signed/web-1.4.0.tgz oci://registry.example.com/charts    # pushes the .prov alongside
helm pull oci://registry.example.com/charts/web --version 1.4.0 --verify --keyring pubring.gpg
```

Verified: `helm verify` prints `Signed by: …` and `Chart Hash Verified: sha256:…`; a modified archive fails with `sha256 sum does not match`; `helm push` uploads an existing `.prov` next to the `.tgz`, and `helm pull --verify` of a version pushed without provenance fails (`failed to fetch provenance "oci://…/web:1.4.0.prov"`). Helm signs with Go's OpenPGP implementation, so the key must be exported, not used through `gpg-agent`. Sigstore/cosign signatures on the OCI artifact are a common alternative; either way, verification must happen in the deploy path (`helm install --verify`, `cosign verify`), not only at publish time.

##### Classic chart repositories

`helm repo add` / `helm repo update` / `index.yaml` still work. Repository credentials go only to the repository host unless `--pass-credentials` is given (3.6.1, CVE-2021-32690) — flag that flag. `helm repo update` without `--fail-on-repo-update-fail` ignores a failing repository and continues with a stale index.

***

#### Values: Sources, Precedence, Nulls and Typing

##### Precedence

Chart `values.yaml` < parent chart values for a subchart < `-f/--values` files (left to right) < `--set-json` < `--set` < `--set-string` < `--set-file` < `--set-literal` (from `pkg/cli/values/options.go`; since 3.13 parent values consistently override subchart defaults).

- `--set` splits on commas and interprets types: `--set a=x,y` fails (`key "y" has no value`); `--set-literal a=x,y` [3.12] sets the literal string `"x,y"` (verified).
- `--set port=8080` yields an `int64`, `--set-string port=8080` a string, while `port: 8080` in a YAML file yields `float64` (verified with `kindOf`). `--set x=3.0` yields the **string** `"3.0"`.
- `--set-json 'resources={"limits":{"memory":"1Gi"}}'` [3.10] sets structured values; Helm 3 does **not** accept a whole JSON object without a key (`failed parsing --set-json data`).

##### Only the first YAML document of a values file counts

```yaml
# values file with two documents
c: 1
---
c: 2
```

Helm 3.22.1 renders `c: 1` and ignores the rest without any warning (Helm 4 merges all documents). Remove stray `---` separators from values files, especially files shared with Helm 4 users.

##### Deleting a default with `null`

```bash
helm template v ./v --set resources.limits=null   # removes the limits block from the defaults
```

- A user `null` removes the key from a non-empty chart default (`resources: {limits: {cpu: 1, memory: 1Gi}}` + `cpu: null` → `{limits: {memory: 1Gi}}`).
- Under an **empty-map** default (`annotations: {}`), a user `a: null` is kept as `a: null` on 3.20.1+ (verified on 3.22.1) and silently dropped on 3.20.0 and earlier. Templates that range over user maps must tolerate `nil` values: `{{- if not (kindIs "invalid" $v) }}`.

##### Implicit `--reuse-values`

`helm upgrade` **without any** `-f`/`--set*` flag reuses the previous release's user values; with at least one values flag, only the chart defaults plus the new flags are used. Verified:

```console
$ helm install rv ./lk -n rv --set foo=bar
$ helm upgrade rv ./lk -n rv                 # no values flags
$ helm get values rv -n rv
foo: bar                                     # reused
$ helm upgrade rv ./lk -n rv --set other=1
$ helm get values rv -n rv
other: 1                                     # foo is gone
$ helm upgrade rv ./lk -n rv --reset-then-reuse-values --set third=1
other: 1
third: 1
```

CI should always pass the complete values (`-f values/prod.yaml`). Flag `--reuse-values` in pipelines: it keeps values of keys the new chart version renamed or removed and ignores new chart defaults; `--reset-then-reuse-values` [3.14] applies the new chart defaults first.

##### YAML typing traps

Values are parsed as YAML 1.1 and converted through JSON. Verified on 3.22.1:

| values.yaml | `.Values` sees | Fix |
|---|---|---|
| `country: NO` | `false` | `country: "NO"` |
| `list: [x, y]` | `["x", true]` | `["x", "y"]` |
| `keys: {y: 2}` | key `"true"` | `{"y": 2}` |
| `version: 1.10` | `1.1` | `version: "1.10"` |
| `big: 12345678901234567` | `12345678901234568` (float64; `{{ .Values.big \| quote }}` prints `"1.2345678901234568e+16"`) | keep as a quoted string |
| `port: 8080` | `float64` | fine for numbers; `{{ .Values.port \| quote }}` prints `"8080"` |

The same YAML 1.1 rules apply to the rendered manifests: `name: n` in a template is the boolean `false`, and the install fails with `json: cannot unmarshal bool into Go struct field ObjectMeta.metadata.name of type string` (verified). Quote rendered names: `name: {{ .Values.name | quote }}`.

***

#### `Chart.yaml`: Metadata, SemVer and `kubeVersion`

```yaml
apiVersion: v2                 # v2 for every Helm 3 chart
name: web                      # lowercase letters, digits, dashes; equals the directory name
description: Example web application chart
type: application              # or "library"
version: 1.4.0                 # chart version, SemVer 2 — bump on every chart change
appVersion: "2.7.1"            # quote it; informational, used as default image tag
kubeVersion: ">=1.30.0-0"      # "-0" lets pre-release/vendor versions (1.31.2-eks-…) match
home: https://example.com/web
icon: https://example.com/web/icon.png
sources:
  - https://github.com/example/web
maintainers:
  - name: Platform Team
    email: platform@example.com
annotations:
  artifacthub.io/license: Apache-2.0
```

- **Helm 3 does not report unknown `Chart.yaml` fields.** Verified: a `Chart.yaml` with `appversion:` (lowercase) and `unknownField:` passes `helm lint --strict` on 3.22.1 without any warning (only the `icon` INFO), and the misspelled key is silently ignored (Helm 4 warns `failed to strictly parse chart metadata file`). Check spelling in review: `appVersion`, `kubeVersion`, `dependencies`.
- **`version` must be SemVer 2.** `version: 1.0` (a YAML float) is a lint error (`version should be of type string but it's of type float64`). `version: "1.0"` passes Helm 3.22.1 lint silently (Helm 4 lint `--strict` fails on it). Use three components.
- **Quote `appVersion`.** Unquoted `appVersion: 2.10` is a lint error (`appVersion should be of type string but it's of type float64`), but `helm template`/`install` proceed with `appVersion: "2.1"` — and the default image tag becomes `2.1`.
- **`kubeVersion` needs `-0`.** A constraint without a pre-release part excludes pre-release versions in SemVer, and vendor builds look like pre-releases (`v1.31.2-eks-7f9249a`): `semverCompare ">=1.30.0" "v1.31.2-eks-7f9249a"` is `false`, `">=1.30.0-0"` is `true` (verified). Against a live cluster Helm 3 checks `kubeVersion` against the server's full `GitVersion` (`getCapabilities` in `pkg/action/action.go`), so on an EKS/GKE cluster `kubeVersion: ">=1.30.0"` fails. Verified on 3.22.1 and 3.18.6 against an apiserver reporting `v1.36.2-eks-7f9249a`: `Error: INSTALLATION FAILED: chart requires kubeVersion: >=1.30.0 which is incompatible with Kubernetes v1.36.2-eks-7f9249a`, while `>=1.30.0-0` installs. Offline CI on 3.19+ still passes because those versions strip the suffix from `--kube-version`. The same `-0` rule applies to every `semverCompare` against `.Capabilities.KubeVersion.Version`.
- A mismatch fails rendering and install: `Error: chart requires kubeVersion: >=1.30.0 which is incompatible with Kubernetes v1.29.0`.
- `icon` missing is an `[INFO]` lint message only.
- `apiVersion: v1` charts still load (Helm 2 format); `apiVersion: v3` is a lint error.

***

#### Chart Layout and Naming

```text
web/
├── Chart.yaml
├── Chart.lock              # committed when dependencies exist
├── values.yaml             # documented defaults
├── values.schema.json      # validated on install/upgrade/template/lint
├── README.md               # values table (helm-docs or similar)
├── .helmignore             # keep CI files, test data, .git out of the package
├── crds/                   # plain CRD YAML: installed once, never templated/upgraded
├── charts/                 # dependencies built from Chart.lock (usually gitignored)
└── templates/
    ├── _helpers.tpl
    ├── NOTES.txt
    ├── deployment.yaml
    ├── service.yaml
    ├── serviceaccount.yaml
    ├── configmap.yaml
    ├── secret.yaml
    ├── hpa.yaml
    ├── pdb.yaml
    ├── networkpolicy.yaml
    ├── ingress.yaml
    ├── migrate-job.yaml
    └── tests/
        └── test-connection.yaml
```

Rules from the chart best practices:

- **Chart names**: lowercase letters, digits and dashes (`my-app`); no uppercase, underscores or dots. The directory name equals `name`.
- **Template files**: dashed lowercase (`pod-disruption-budget.yaml`), `.yaml` for YAML output, `.tpl` for helpers without output. One resource kind per file; the file name reflects the kind.
- **Defined template names are global** across the parent and all subcharts: prefix them with the chart name (`web.fullname`, not `fullname`). Verified: when a parent and a subchart both define `fullname`, the subchart's own `include "fullname"` returns the **parent's** definition.
- **Values keys**: camelCase starting with a lowercase letter (`replicaCount`). Keys with dashes cannot be accessed with dot notation (`index .Values "my-key"` works).
- **Resource names**: produced by `*.fullname`, at most 63 characters, `trimSuffix "-"`. Objects that append a suffix must budget for it: `{{ include "web.fullname" . | trunc 55 | trimSuffix "-" }}-migrate`.
- Files in `templates/` starting with `_` produce no output; `NOTES.txt` is rendered and printed after install/upgrade.
- `.helmignore` excludes files from `helm package`; secrets, `.env`, CI files and large fixtures must not end up in the published chart.

***

#### `_helpers.tpl`, Names and Labels

The helpers produced by `helm create` in 3.22.1 (comments shortened; `selectorLabels` moved up, `if` replaced by `with`), plus `web.image` and `web.secretName` (rendered and linted on 3.22.1):

```yaml
{{/*
Expand the name of the chart.
*/}}
{{- define "web.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Fully qualified app name, truncated to 63 chars (DNS-1123 label limit).
*/}}
{{- define "web.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "web.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Selector labels: immutable once a Deployment exists. Never add version labels here.
*/}}
{{- define "web.selectorLabels" -}}
app.kubernetes.io/name: {{ include "web.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "web.labels" -}}
helm.sh/chart: {{ include "web.chart" . }}
{{ include "web.selectorLabels" . }}
{{- with .Chart.AppVersion }}
app.kubernetes.io/version: {{ . | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "web.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "web.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image reference: registry/repository[:tag][@digest]. Digest wins over tag.
*/}}
{{- define "web.image" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.image.registry -}}
{{- $ref := printf "%s/%s" $registry .Values.image.repository | trimPrefix "/" -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" $ref .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" $ref (.Values.image.tag | default .Chart.AppVersion) -}}
{{- end -}}
{{- end }}

{{/*
Secret holding the password: the user's existingSecret or the chart-managed one.
*/}}
{{- define "web.secretName" -}}
{{- .Values.existingSecret | default (printf "%s-secret" (include "web.fullname" .)) }}
{{- end }}
```

Review points:

- **Selector labels are immutable.** Adding `app.kubernetes.io/version`, `helm.sh/chart` or any value-driven label to `selectorLabels` breaks the next upgrade (`spec.selector: Invalid value: …: field is immutable`) — and, because Helm 3 upgrades are not transactional, Services and PDBs may already have switched to the new selector (section 5). Selectors contain only `app.kubernetes.io/name` and `app.kubernetes.io/instance` (plus `component` for multi-workload charts).
- Use the Kubernetes recommended labels (`app.kubernetes.io/name`, `instance`, `version`, `component`, `part-of`, `managed-by`) on every object. `{{ .Release.Service }}` renders `Helm`.
- The `helm create` scaffold puts the full `labels` (including `helm.sh/chart` and `app.kubernetes.io/version`) on the pod template, so every chart or app version bump rolls all Pods. Accept it deliberately or use selector labels plus stable labels on the pod template.
- Use `include`, not `template`: `include` returns a string that can be piped. `{{ template "x" . | nindent 4 }}` pipes the argument into `nindent` and fails: `executing ... at <4>: wrong type for value; expected string; got chartutil.Values` (verified).
- Pass the right context: `include "web.fullname" .` at top level, `include "web.fullname" $` inside `range`/`with`. Helpers with extra parameters take a dict: `include "web.componentName" (dict "root" $ "component" "worker")`.

***

#### Template Language: Whitespace, Scope, Functions and Pitfalls

Helm 3.22.1 templates are Go `text/template` plus Sprig v3.3.0 (without `env` and `expandenv`) plus Helm's functions: `include`, `tpl`, `required`, `lookup` [3.1], `toYaml`, `toYamlPretty` [3.17], `fromYaml`, `fromYamlArray` [3.2], `toJson`, `fromJson`, `fromJsonArray` [3.2], `toToml`, `fromToml` [3.17]. The `.Files` object (`Get`, `GetBytes`, `Glob`, `Lines`, `AsConfig`, `AsSecrets`) reads non-template chart files.

##### Whitespace and indentation

- `{{-` trims whitespace and newlines before the action, `-}}` after. Use `{{-` on lines that contain only an action.
- `nindent N` emits a newline and indents every line by N; use it after `{{-` on its own line. `indent N` indents without the leading newline.
- N must equal the column where the block's keys belong. A wrong N produces either invalid YAML or, worse, valid YAML with keys under the wrong parent:

```yaml
# BAD: nindent after "a:" on the same line — "A" becomes a sibling of "a", and "a" is null
data:
  a: {{ .Values.env | toYaml | nindent 2 }}
# renders:
#   data:
#     a:
#     A: "1"

# GOOD
data:
  {{- toYaml .Values.env | nindent 2 }}
```

- For embedded files use a `|` block scalar plus `nindent`:

```yaml
data:
  feature-flags.json: |
    {{- .Values.config.featureFlags | toJson | nindent 4 }}
```

##### Scope: `with`, `range` and `$`

`with` and `range` rebind `.`. Inside them, use `$` for the root context:

```yaml
# BAD
data:
  {{- range $k, $v := .Values.env }}
  {{ $k }}: {{ $v | quote }}
  ns: {{ .Release.Namespace }}
  {{- end }}
# Error: template: tp/templates/cm.yaml:8:17: executing "tp/templates/cm.yaml" at <.Release.Namespace>: can't evaluate field Release in type interface {}

# GOOD
  ns: {{ $.Release.Namespace }}
```

Helm 3 prints template execution errors on one line starting with `Error: template:`; `required`/`fail` produce `Error: execution error at (file:line:col): message`.

##### `with` and `default` treat `false`, `0` and `""` as empty

```yaml
# BAD: maxUnavailable: 0 is a valid PDB setting, but "with" skips it — the PDB renders with neither field
spec:
  {{- with .Values.podDisruptionBudget.maxUnavailable }}
  maxUnavailable: {{ . }}
  {{- end }}

# GOOD
spec:
  {{- if hasKey .Values.podDisruptionBudget "maxUnavailable" }}
  maxUnavailable: {{ .Values.podDisruptionBudget.maxUnavailable }}
  {{- end }}
```

```yaml
# values.yaml: enabled: false, replicas: 0
badDefaultBool: {{ .Values.enabled | default true }}    # true  <- the user's false is ignored
badDefaultInt:  {{ .Values.replicas | default 3 }}      # 3     <- the user's 0 is ignored

# GOOD: distinguish "unset" from "false/0"
enabled: {{ hasKey .Values "enabled" | ternary .Values.enabled true }}
```

All verified on 3.22.1. Better still: put defaults in `values.yaml` and avoid `default` for booleans and numbers.

##### `required` and `fail`

```yaml
host: {{ required "ingress.host is required when ingress.enabled=true" .Values.ingress.host }}
```

`required` fails on `nil` and on the empty string (`Error: execution error at (tp/templates/cm.yaml:6:8): host is required`), not on `false` or `0`. For cross-field rules use `fail`:

```yaml
{{- if and (hasKey .Values.podDisruptionBudget "minAvailable") (hasKey .Values.podDisruptionBudget "maxUnavailable") }}
{{- fail "set either podDisruptionBudget.minAvailable or maxUnavailable, not both" }}
{{- end }}
```

Prefer `values.schema.json` for type, enum and range checks: it reports all violations at once.

##### `quote`, types and `toJson`

- Quote every free-form string: `{{ .Values.someString | quote }}`. Unquoted `yes`, `on`, `1e3`, `0777` change type when the rendered manifest is parsed.
- Env values, annotation values and label values must be strings. `value: {{ .Values.port }}` fails on install: `Pod in version "v1" cannot be handled as a Pod: json: cannot unmarshal number into Go struct field EnvVar.spec.containers.env.value of type string`.
- Do not quote integers the API expects as numbers: `containerPort: "8080"` fails with `cannot unmarshal string into Go struct field ContainerPort.spec.containers.ports.containerPort of type int32`. Neither error is caught by `helm lint`, `helm template` or `--dry-run=server`.
- Helm's `toJson`/`toYaml`/`toToml` return an empty string on marshal errors. Where silent empty output is dangerous, Sprig's `mustToJson` (available in Helm 3) fails instead; `mustToYaml` is Helm 4 only.
- `.Values` is a `chartutil.Values`, not a `map[string]interface{}`. Sprig's `dig` fails on it: `error calling dig: interface conversion: interface {} is chartutil.Values, not map[string]interface {}`. Use `.Values.AsMap`:

```yaml
timeout: {{ dig "server" "timeout" "30s" .Values.AsMap | quote }}
```

- Numbers from YAML values are `float64`; `int`/`int64` casts lose precision above 2^53 (`12345678901234567` → `12345678901234568`).

##### `tpl`

```yaml
# values.yaml
tplValue: "{{ .Release.Name }}-svc"
# template
service: {{ tpl .Values.tplValue . | quote }}     # "rel-svc"
```

Use `tpl` only for values documented as templatable (hostnames, affinity blocks: `{{- tpl (toYaml .) $ | nindent 8 }}`). It can call `lookup` (cluster reads during install), `include` any helper and `fail`; never feed it untrusted input. Nested `tpl` sees the outer template's `define`s since 3.14.

##### Random and time functions

`randAlphaNum`, `uuidv4`, `now`, `date` produce a different value on every render. Used directly in a Secret, ConfigMap or annotation, they change the object on every upgrade (rotating a password, restarting Pods). Use them only with a `lookup` that preserves an existing value (section 18), and remember `lookup` is empty in `helm template`.

##### Network access during render

`getHostByName` returns `""` unless `--enable-dns` is passed (3.11.1+, verified: `""` without the flag, `"127.0.0.1"` with it for `localhost`). A chart that needs DNS at render time is not reproducible; flag `--enable-dns` in pipelines.

***

#### Values Design

The example chart's `values.yaml` (rendered and installed on 3.22.1 against a v1.36 apiserver in a namespace with Pod Security Admission `enforce=restricted` and `warn=restricted`, with no warnings):

```yaml
# -- Number of replicas. Ignored when autoscaling.enabled is true.
replicaCount: 2

image:
  # -- Registry host; overridden by global.imageRegistry when set.
  registry: registry.example.com
  repository: web/app
  # -- Image tag; defaults to .Chart.AppVersion when empty.
  tag: ""
  # -- Optional digest (sha256:...). Takes precedence over tag.
  digest: ""
  pullPolicy: IfNotPresent

imagePullSecrets: []
nameOverride: ""
fullnameOverride: ""

serviceAccount:
  create: true
  automount: false
  annotations: {}
  name: ""

podAnnotations: {}
podLabels: {}

podSecurityContext:
  runAsNonRoot: true
  runAsUser: 10001
  runAsGroup: 10001
  fsGroup: 10001
  seccompProfile:
    type: RuntimeDefault

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop: ["ALL"]

service:
  type: ClusterIP
  port: 80
  targetPort: 8080

# -- Application configuration rendered into a ConfigMap.
config:
  logLevel: info
  featureFlags: {}

# -- Name of an existing Secret with key "password". When empty, the chart creates one.
existingSecret: ""

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    memory: 256Mi

livenessProbe:
  httpGet: {path: /healthz, port: http}
  periodSeconds: 10
readinessProbe:
  httpGet: {path: /readyz, port: http}
  periodSeconds: 5
startupProbe:
  httpGet: {path: /healthz, port: http}
  failureThreshold: 30
  periodSeconds: 2

autoscaling:
  enabled: false
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80

podDisruptionBudget:
  enabled: true
  # -- Use either minAvailable or maxUnavailable, not both.
  maxUnavailable: 1

networkPolicy:
  enabled: false

migrations:
  enabled: true
  # -- Seconds after which a finished migration Job is garbage-collected.
  ttlSecondsAfterFinished: 600

ingress:
  enabled: false
  className: ""
  annotations: {}
  hosts:
    - host: web.example.com
      paths:
        - path: /
          pathType: Prefix
  tls: []

nodeSelector: {}
tolerations: []
affinity: {}
topologySpreadConstraints: []

global:
  imageRegistry: ""
```

Rules:

- **Document every key** (comment directly above it; `# --` is the helm-docs convention).
- **Secure defaults**: non-root, `readOnlyRootFilesystem`, all capabilities dropped, `seccompProfile: RuntimeDefault`, service account token not mounted unless needed. Users opt out of security, not in.
- **Prefer maps over lists for collections users override.** `--set` and values-file merging replace lists wholesale, and duplicate list keys misbehave on upgrade (section 5). A map (`env: {LOG_LEVEL: info}`) merges key by key.
- **Every optional feature has an `enabled` flag** with a safe default; templates gate on it.
- **No secrets in `values.yaml` defaults.** Support `existingSecret`; never ship a default password.
- **Pass Kubernetes structures through unchanged** (`affinity`, `tolerations`, `resources`) with `toYaml`; do not re-model them as separate scalar values without reason.
- **`global`** is shared with every subchart; use it only for values that genuinely apply to all (registry, pull secrets).
- Defaults must render a working install with `helm install x ./chart` alone, or fail early with `required`.

***

#### `values.schema.json`

Helm validates the merged values against `values.schema.json` on `install`, `upgrade`, `template` and `lint`; each subchart's schema validates that subchart's values. `--skip-schema-validation` [3.16] disables it — flag it in pipelines.

The example chart's schema (tested on 3.22.1):

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["image", "service"],
  "properties": {
    "replicaCount": {"type": "integer", "minimum": 0},
    "image": {
      "type": "object",
      "additionalProperties": false,
      "required": ["repository"],
      "properties": {
        "registry": {"type": "string"},
        "repository": {"type": "string", "minLength": 1},
        "tag": {"type": "string"},
        "digest": {"type": "string", "pattern": "^(sha256:[a-f0-9]{64})?$"},
        "pullPolicy": {"enum": ["Always", "IfNotPresent", "Never"]}
      }
    },
    "service": {
      "type": "object",
      "properties": {
        "type": {"enum": ["ClusterIP", "NodePort", "LoadBalancer"]},
        "port": {"type": "integer", "minimum": 1, "maximum": 65535},
        "targetPort": {"type": ["integer", "string"]}
      }
    },
    "config": {
      "type": "object",
      "properties": {
        "logLevel": {"enum": ["debug", "info", "warn", "error"]}
      }
    },
    "podDisruptionBudget": {
      "type": "object",
      "not": {"required": ["minAvailable", "maxUnavailable"]}
    },
    "resources": {"type": "object"}
  }
}
```

```console
$ helm upgrade w1 ./web -n w --set replicaCount=-1
Error: UPGRADE FAILED: values don't meet the specifications of the schema(s) in the following chart(s):
web:
- at '/replicaCount': minimum: got -1, want 0
```

##### Draft support depends on the minor [3.18]

Helm 3.18 replaced the validator with `santhosh-tekuri/jsonschema/v6`, which supports drafts 4, 6, 7, 2019-09 and 2020-12. **Helm 3.17 and earlier only understand draft-07 and older and silently misinterpret 2020-12 keywords.** Verified with `"prefixItems": [{"type": "string"}, {"type": "integer"}], "items": false` and the valid value `pair: ["a", 1]`:

| | 3.17.4 | 3.18.6 / 3.22.1 |
|---|---|---|
| `pair: ["a", 1]` | **rejected**: `pair.0: False always fails validation` (`prefixItems` ignored, `items: false` applied to every element) | accepted |
| `pair: ["a", "b"]` | rejected | rejected: `at '/pair/1': got string, want integer` |
| `cfg.b` with `"unevaluatedProperties": false` | not checked | rejected: `at '/cfg/b': false schema` |

A chart with a 2020-12 schema therefore requires Helm ≥ 3.18; for charts consumed by older Helm 3 users, write draft-07 (`"$schema": "http://json-schema.org/draft-07/schema#"`, `items` as a list for tuples, no `unevaluatedProperties`). Error message formats also changed in 3.18 (`at '/path': …` instead of `path: …`) — update tests that match on them.

##### `$ref`

- `$ref` to `http(s)://` URLs is resolved over the network at validation time (a 3.18 regression made `helm lint` reject them; fixed in 3.19.0); vendor referenced schemas into the chart instead.
- Unresolvable URN `$ref`s warn and are ignored since 3.20 (`WARNING: unresolved URN reference ignored; using permissive schema`); 3.18.6 fails with `no URLLoader registered for "urn:…"` and 3.17.4 with `must be canonical`.
- A `$ref` to a local device file was a DoS (CVE-2025-55199, fixed in 3.18.5).

Review points:

- `"additionalProperties": false` at the **root** rejects keys for subcharts and `global` unless they are listed. Use it on nested objects you own (it catches typos such as `replicaCont`), not at the root.
- `"type": "integer"` accepts `3` and `3.0` from a YAML file and `--set replicaCount=3`, rejects `3.5` (`got number, want integer`), and rejects `--set replicaCount=3.0` and any `--set-string` value (`--set` only converts integers, booleans and `null`; everything else stays a string: `got string, want integer`).
- Allow both forms where Kubernetes does (`targetPort`: integer or named port).
- Validation runs on merged values; schema `required` is for keys without defaults.

***

#### Workload Manifests: Security, Resources, Probes, Availability

The example Deployment (rendered on 3.22.1; installed into a `restricted` namespace without warnings):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "web.fullname" . }}
  labels:
    {{- include "web.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  revisionHistoryLimit: 5
  selector:
    matchLabels:
      {{- include "web.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      annotations:
        checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
        {{- with .Values.podAnnotations }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      labels:
        {{- include "web.labels" . | nindent 8 }}
        {{- with .Values.podLabels }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
    spec:
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "web.serviceAccountName" . }}
      automountServiceAccountToken: {{ .Values.serviceAccount.automount }}
      {{- with .Values.podSecurityContext }}
      securityContext:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: {{ .Chart.Name }}
          image: {{ include "web.image" . | quote }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          {{- with .Values.securityContext }}
          securityContext:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          ports:
            - name: http
              containerPort: {{ .Values.service.targetPort }}
              protocol: TCP
          envFrom:
            - configMapRef:
                name: {{ include "web.fullname" . }}
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ include "web.secretName" . }}
                  key: password
          {{- with .Values.livenessProbe }}
          livenessProbe:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          {{- with .Values.readinessProbe }}
          readinessProbe:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          {{- with .Values.startupProbe }}
          startupProbe:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          {{- with .Values.resources }}
          resources:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          volumeMounts:
            - name: tmp
              mountPath: /tmp
      volumes:
        - name: tmp
          emptyDir: {}
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.topologySpreadConstraints }}
      topologySpreadConstraints:
        {{- tpl (toYaml .) $ | nindent 8 }}
      {{- end }}
```

The matching PodDisruptionBudget (uses `hasKey` so `0` is honoured):

```yaml
{{- if .Values.podDisruptionBudget.enabled }}
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "web.fullname" . }}
  labels:
    {{- include "web.labels" . | nindent 4 }}
spec:
  {{- /* hasKey, not "with": 0 is a valid value and "with" would drop it */}}
  {{- if hasKey .Values.podDisruptionBudget "minAvailable" }}
  minAvailable: {{ .Values.podDisruptionBudget.minAvailable }}
  {{- end }}
  {{- if hasKey .Values.podDisruptionBudget "maxUnavailable" }}
  maxUnavailable: {{ .Values.podDisruptionBudget.maxUnavailable }}
  {{- end }}
  selector:
    matchLabels:
      {{- include "web.selectorLabels" . | nindent 6 }}
{{- end }}
```

`--set podDisruptionBudget.maxUnavailable=null --set podDisruptionBudget.minAvailable=50%` switches modes (the `null` removes the default key).

##### Security context

- Default to the Pod Security Standards **restricted** profile: `runAsNonRoot: true`, a numeric non-zero `runAsUser`, `allowPrivilegeEscalation: false`, `capabilities.drop: ["ALL"]`, `seccompProfile.type: RuntimeDefault`; add `readOnlyRootFilesystem: true` and an `emptyDir` for `/tmp`.
- The plain `helm create` scaffold does not meet `restricted`. Verified: its Deployment installs with a warning (`would violate PodSecurity "restricted:latest": allowPrivilegeEscalation != false …`), and its test Pod is rejected by `helm test` (`pods "sc-scaffold-test-connection" is forbidden: violates PodSecurity "restricted:latest"`). Helm 3 prints apiserver warnings through klog (`I1023 … warnings.go:107] "Warning: …"`) and still reports `STATUS: deployed`; CI must grep for them or install into a namespace with `enforce=restricted` in a test cluster.
- Flag `privileged: true`, `hostNetwork`, `hostPID`, `hostIPC`, `hostPath` volumes, added capabilities and `runAsUser: 0` unless the chart's purpose requires them and they sit behind explicit values.
- `automountServiceAccountToken: false` unless the app talks to the Kubernetes API; then ship a least-privilege `Role`/`RoleBinding` behind `rbac.create`.

##### Resources

- Always set requests (CPU and memory) and a memory limit; a CPU limit is a policy choice (throttling hurts latency-sensitive services).
- `resources: {}` as default (the scaffold) is acceptable only for example charts.

##### Probes

- Readiness gates traffic, liveness restarts, startup protects slow starts. A liveness probe that depends on downstream services causes cascading restarts.
- Use the named port (`port: http`); prefer `startupProbe` over a long `initialDelaySeconds`.
- Helm 3 `--wait` relies on readiness: without a readiness probe a Pod is Ready as soon as its containers start, so `--wait`/`--atomic` cannot detect a broken release.

##### Availability

- PodDisruptionBudget for workloads with more than one replica, with `minAvailable` **or** `maxUnavailable` (the API rejects both: `minAvailable and maxUnavailable cannot be both set`). A PDB with `minAvailable` equal to the replica count blocks node drains.
- HPA with `autoscaling/v2`; omit `spec.replicas` when enabled (section 5).
- `topologySpreadConstraints` or anti-affinity for spreading; `revisionHistoryLimit` to bound old ReplicaSets.
- `strategy.rollingUpdate.maxUnavailable: 0` with `maxSurge: 1` for zero-downtime services — this also makes Helm 3's `--wait` require all replicas of the new ReplicaSet to be ready.
- `terminationGracePeriodSeconds` longer than the app's shutdown time; a `preStop` sleep when load balancers need time to deregister.

##### Networking

- Gate `Ingress`, Gateway API `HTTPRoute` (scaffolded by `helm create` since 3.19) and `NetworkPolicy` behind values. Use `ingressClassName`.
- A NetworkPolicy must allow the traffic the chart needs (metrics scrapers, ingress controllers); kubelet probes are not subject to NetworkPolicy.

***

#### Configuration and Secrets

##### Restart Pods when configuration changes

```yaml
annotations:
  checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
```

ConfigMaps and Secrets consumed via `envFrom`/`env` are read only at container start; the checksum annotation changes the pod template when the rendered ConfigMap changes. Do not checksum a Secret that is generated randomly on every render.

##### Generated secrets that survive upgrades

```yaml
{{- if not .Values.existingSecret }}
{{- $name := include "web.secretName" . }}
{{- $existing := lookup "v1" "Secret" .Release.Namespace $name }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ $name }}
  labels:
    {{- include "web.labels" . | nindent 4 }}
  annotations:
    # keep the generated password if the release is uninstalled
    helm.sh/resource-policy: keep
type: Opaque
data:
  {{- if and $existing $existing.data (hasKey $existing.data "password") }}
  password: {{ index $existing.data "password" }}
  {{- else }}
  password: {{ randAlphaNum 32 | b64enc | quote }}
  {{- end }}
{{- end }}
```

Limitations:

- Under `helm template` (Argo CD, Flux with `helm template` semantics) `lookup` is empty, so the password changes on every sync. For GitOps, require `existingSecret` (External Secrets Operator, Sealed Secrets, SOPS).
- `helm.sh/resource-policy: keep` leaves the Secret behind on uninstall (Helm prints `These resources were kept due to the resource policy: [Secret] p1-web-secret`) with its ownership metadata. Verified: reinstalling the **same release name in the same namespace** adopts it and the `lookup` keeps the old password (also across upgrades); a release with a different name fails with the ownership error below unless `--take-ownership` is used.
- Never put secrets into `values.yaml`, `NOTES.txt` output, ConfigMaps, annotations or labels. Release Secrets contain every value passed to Helm.

##### Ownership and adoption

Install refuses to touch an existing object that lacks Helm's ownership metadata. Verified:

```console
$ helm install o1 ./own -n o
Error: INSTALLATION FAILED: Unable to continue with install: ConfigMap "own-cm" in namespace "o" exists and cannot be imported into the current release: invalid ownership metadata; label validation error: missing key "app.kubernetes.io/managed-by": must be set to "Helm"; annotation validation error: missing key "meta.helm.sh/release-name": must be set to "o1"; annotation validation error: missing key "meta.helm.sh/release-namespace": must be set to "o"
```

- Adopt deliberately either by labelling/annotating the object (`app.kubernetes.io/managed-by=Helm`, `meta.helm.sh/release-name`, `meta.helm.sh/release-namespace`) or with `--take-ownership` [3.17], which skips the check and rewrites the metadata.
- **`--take-ownership` steals objects from other releases**, and Helm 3 uninstall does not check ownership. Verified: release `o2` took `own-cm` from `o1` with `--take-ownership`; `helm uninstall o1` then deleted `own-cm`, breaking `o2` (still `deployed`). Flag `--take-ownership` as a permanent CI flag.

##### ConfigMaps and files

- `| quote` for scalar values, `|` block scalars with `nindent` for files.
- ConfigMaps and Secrets larger than 1 MiB are rejected; a release Secret is also bounded by 1 MiB (compressed), so very large charts or embedded files fail to install.
- `.Files.Get "files/app.conf"` and `(.Files.Glob "files/*.conf").AsConfig` read packaged files; files in `.helmignore` and under `templates/` are not available through `.Files`.

***

#### Hooks

```yaml
{{- if .Values.migrations.enabled }}
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ include "web.fullname" . | trunc 55 | trimSuffix "-" }}-migrate
  labels:
    {{- include "web.labels" . | nindent 4 }}
  annotations:
    # post-install, not pre-install: on the first install the Secret and ServiceAccount
    # are regular chart resources and do not exist before the pre-install phase
    "helm.sh/hook": post-install,pre-upgrade
    "helm.sh/hook-weight": "0"
    "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
spec:
  backoffLimit: 2
  activeDeadlineSeconds: 600
  ttlSecondsAfterFinished: {{ .Values.migrations.ttlSecondsAfterFinished }}
  template:
    metadata:
      labels:
        {{- include "web.selectorLabels" . | nindent 8 }}
        app.kubernetes.io/component: migrate
    spec:
      restartPolicy: Never
      serviceAccountName: {{ include "web.serviceAccountName" . }}
      automountServiceAccountToken: false
      {{- with .Values.podSecurityContext }}
      securityContext:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: migrate
          image: {{ include "web.image" . | quote }}
          args: ["migrate", "up"]
          {{- with .Values.securityContext }}
          securityContext:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ include "web.secretName" . }}
                  key: password
{{- end }}
```

Hook types: `pre-install`, `post-install`, `pre-upgrade`, `post-upgrade`, `pre-rollback`, `post-rollback`, `pre-delete`, `post-delete`, `test`.

Review points:

- **Hook Jobs and Pods are always waited for**, with or without `--wait`, until complete or `--timeout` (default 5m). Verified: a post-install Job that never completed failed the install after 5 minutes with `Error: INSTALLATION FAILED: failed post-install: 1 error occurred: * timed out waiting for the condition`, leaving the release `failed`. Set `backoffLimit` and `activeDeadlineSeconds`; hook kinds other than Job and Pod are not waited for.
- **Hook resources are not part of the release.** They are not upgraded in place, not deleted on uninstall (verified: three ConfigMap hooks remained after `helm uninstall`), and not listed by `helm get manifest` (`helm get hooks` shows them). Always set `helm.sh/hook-delete-policy`; without it Helm applies `before-hook-creation`.
- `hook-succeeded` deletes the evidence of success. Keep logs with `"helm.sh/hook-output-log-policy": hook-failed` (or `hook-succeeded`) [3.18], which prints the hook Pod/Job container logs.
- **Ordering**: hooks of one event run by ascending `helm.sh/hook-weight` (a string, `"-5"`), then by **name**. The documentation says weight, kind, name, but Helm 3.22.1 sorts equal weights by name only (`hookByWeight` in `pkg/action/hooks.go`). Verified with equal weights: `a-cm` (ConfigMap), `m-secret` (Secret), `z-sa` (ServiceAccount) ran in that order — the ServiceAccount last. Use distinct weights when a hook depends on another hook.
- A `pre-install` hook runs before any regular resource exists: a migration Job that mounts a Secret or uses a ServiceAccount from `templates/` fails on the first install. Make the dependencies hooks with lower weights, or use `post-install,pre-upgrade` as above.
- `before-hook-creation` deletes the previous hook object and waits for the deletion before creating the new one.
- Hooks are not post-rendered in Helm 3 (section 8).
- `--no-hooks` skips hooks; flag it in CI unless intended.
- Migrations should be idempotent and backwards-compatible: a failed `pre-upgrade` aborts the upgrade, and `--atomic` rollback does not reverse a migration.
- CRDs must not be hooks; use `crds/` or a separate chart. Helm never deletes a CRD hook (deleting a CRD would cascade to all its custom resources).

***

#### Chart Tests

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: {{ include "web.fullname" . }}-test-connection
  labels:
    {{- include "web.labels" . | nindent 4 }}
  annotations:
    "helm.sh/hook": test
    "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
spec:
  restartPolicy: Never
  automountServiceAccountToken: false
  securityContext:
    runAsNonRoot: true
    runAsUser: 65534
    seccompProfile:
      type: RuntimeDefault
  containers:
    - name: wget
      image: busybox:1.37
      command: ["wget", "-q", "-O-", "http://{{ include "web.fullname" . }}:{{ .Values.service.port }}/readyz"]
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: ["ALL"]
```

- Run with `helm test <release> --logs --timeout 2m`; `--filter name=…` selects tests.
- **`--logs` and `hook-succeeded` do not mix in Helm 3.** The logs are fetched after the tests ran, and by then `hook-succeeded` has deleted every passing test Pod. Verified with the Pod above (phase set to `Succeeded`): the test reports `Phase: Succeeded`, then the command fails with `Error: unable to get pod logs for rt-web-test-connection: pods "rt-web-test-connection" not found` and exit code 1 — a green test turns the CI job red. Use only `before-hook-creation` on test Pods when CI runs `helm test --logs` (Helm 4.0.5+ collects logs before deletion).
- Test Pods must pass the namespace's Pod Security level (verified: this test Pod is admitted in a `restricted` namespace, the scaffold's is not).
- Pin test images; check behaviour (HTTP 200 on a readiness endpoint), not only TCP reachability.

***

#### CRDs

Verified on 3.22.1:

- Files in `crds/` are plain YAML (not templated), installed **before** the templates on the first install, and Helm waits (up to 60 s) until they are established, so custom resources in `templates/` of the same chart can be created in that install.
- They are **never upgraded** (a changed `crds/` file left the cluster CRD unchanged after `helm upgrade`) and **never deleted** (the CRD survived `helm uninstall`; the custom resources rendered from `templates/` were deleted).
- `--skip-crds` skips them; `helm template --include-crds` renders them; `helm show crds` [3.7] prints them.
- CRDs that must evolve belong in a separate chart (templated, upgraded deliberately) or are managed by the operator. Deleting a CRD deletes every custom resource of that kind cluster-wide.
- Custom resources whose CRD is not installed yet fail the whole install before anything is created (verified): `unable to build kubernetes objects from release manifest: resource mapping not found for name: "nope" namespace: "" from "": no matches for kind "Nope" in version "example.com/v1"` / `ensure CRDs are installed first`. Gate them with `.Capabilities.APIVersions.Has` when the CRD comes from another release.
- Drift on custom resources is not repaired by upgrades (section 5).

***

#### Dependencies, Subcharts and Library Charts

```yaml
# Chart.yaml
dependencies:
  - name: common
    version: "~1.0.0"
    repository: "file://../lib"
  - name: sub
    version: "0.2.0"                     # exact pin (house rule); ranges are resolved into Chart.lock
    repository: "file://../sub"
    condition: sub.enabled
  - name: sub
    alias: sub2                          # the same chart a second time, with its own values key
    version: "0.2.0"
    repository: "file://../sub"
```

Verified on 3.22.1:

- `helm template` without built dependencies fails: `found in Chart.yaml, but missing in charts/ directory: common, sub, sub`.
- `helm dependency update` resolves versions and writes `Chart.lock` (with a `digest:` of the dependency set); `helm dependency build` installs exactly the locked versions and fails when `Chart.yaml` changed: `the lock file (Chart.lock) is out of sync with the dependencies file (Chart.yaml). Please update the dependencies`.
- **Commit `Chart.lock`; CI runs `helm dependency build`**, not `update`. Review `Chart.lock` diffs. Never run `helm dependency update` on untrusted charts with an old Helm (CVE-2025-53547).
- Prefer exact versions (house rule for reproducibility); for OCI dependencies (`repository: oci://registry/charts`) ranges work since 3.8.
- Parent values for a subchart live under its name or alias (`sub.port`, `sub2.port`); `global` is visible to all (`global.region` rendered in both subcharts); `import-values` copies child values up.
- `condition:` and `tags:` toggle subcharts (`--set sub.enabled=false` removed `sub` but kept `sub2`).
- **Defined template names are global**: the subchart's `{{ include "fullname" . }}` rendered the parent's `parent-fullname`. Prefix every `define` with the chart name.
- `.Chart.IsRoot` [3.7] is `false` inside subcharts.
- **Library charts** (`type: library`) contain only `define`s and cannot be installed (`Error: INSTALLATION FAILED: library charts are not installable`). Their templates run with the caller's context:

```yaml
# lib/templates/_labels.tpl (chart "common", type: library)
{{- define "common.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

# parent/templates/cm.yaml
metadata:
  labels:
    {{- include "common.labels" . | nindent 4 }}   # .Chart.Name is the *parent's* name: "parent"
```

- `helm package` includes the built dependencies under `charts/` and `Chart.lock`. Directories vendored into `charts/` by hand bypass lock verification; prefer packaged dependencies from the lock.
- Since 3.14 dependency names/aliases must be unique.

***

#### `NOTES.txt`

```text
{{ include "web.fullname" . }} {{ .Chart.AppVersion }} is deployed in namespace {{ .Release.Namespace }}.

Port-forward the service:
  kubectl -n {{ .Release.Namespace }} port-forward svc/{{ include "web.fullname" . }} 8080:{{ .Values.service.port }}
{{- if not .Values.existingSecret }}

Read the generated password:
  kubectl -n {{ .Release.Namespace }} get secret {{ include "web.secretName" . }} -o jsonpath='{.data.password}' | base64 -d
{{- end }}
```

- Print commands, not secret values: notes are stored in the release and shown by `helm status`/`helm get notes`.
- `--hide-notes` [3.16] suppresses notes on install/upgrade; `--render-subchart-notes` renders subchart notes.

***

#### CI Pipelines

A minimal GitHub Actions job for a Helm 3 chart repository:

```yaml
jobs:
  chart:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v5
      - uses: azure/setup-helm@v5
        with:
          version: v3.22.1          # pin: omitting it now installs Helm 4
      - name: Lint and render
        run: |
          set -euo pipefail
          helm dependency build charts/web
          helm lint --strict charts/web -f ci/values/prod.yaml
          helm template web charts/web -n web \
            -f ci/values/prod.yaml \
            --kube-version 1.36.0 \
            --api-versions gateway.networking.k8s.io/v1 > rendered.yaml
          # validate rendered.yaml against the target cluster's schemas here
          # (kubeconform, or kubectl apply --dry-run=server -f rendered.yaml)
```

Review points for pipelines:

- **Pin Helm exactly** (`v3.22.1`, `alpine/helm:3.22.1`); pin actions to a commit SHA in security-sensitive repositories. Use the latest 3.22.x patch: older 3.x minors receive no fixes.
- `helm lint --strict` with every production values file. Remember what lint does **not** catch on Helm 3: misspelled `Chart.yaml` keys, non-three-part versions, duplicate objects, unknown manifest fields, wrong value types in manifests (all verified to pass).
- Render with `--kube-version` matching the target cluster; the offline default changes with the Helm patch you install.
- `helm template` output is not validated; `--dry-run=server` validates kinds and ownership but not fields. Validate rendered manifests separately and fail on `Warning: unknown field`.
- Deploy with explicit values, explicit namespace, `--atomic` and `--timeout`, and give the CI step a timeout longer than `--timeout` (so Helm can roll back instead of being killed into `pending-*`):

```bash
helm upgrade --install web oci://ghcr.io/acme/charts/web@sha256:<digest> \
  -n web --create-namespace \
  -f values/prod.yaml \
  --atomic --timeout 10m
```

- Flag in deploy scripts: `--force` and `--take-ownership` as defaults, `--reuse-values`, a missing `-f` (implicit value reuse), `--no-hooks`, `--skip-schema-validation`, `--disable-openapi-validation`, `--insecure-skip-tls-verify`, `--plain-http`, `--enable-dns`, `--pass-credentials`, `helm registry login` with `-p`, `--wait-for-jobs` without `--wait`, `--history-max 0`, and any Helm 4 flag (`--server-side`, `--rollback-on-failure`, `--force-replace`, `--wait=watcher`).
- Run Helm 3 and Helm 4 side by side only with separate `HELM_CONFIG_HOME`, `HELM_DATA_HOME` and `HELM_CACHE_HOME`.
- Release Secrets live in the release namespace; restrict `get secrets` there, because they contain all values.

***

#### Logging and Debugging

- `--debug` (or `HELM_DEBUG=true`) prints the Helm client's actions on stderr in klog style: `client.go:144: 2026-10-23 05:32:24.354844 +0000 UTC m=+0.157468 [debug] creating 1 resource(s)` (timestamps shortened), and the ready checker's reasons (`Deployment is not ready: …`). Use it in CI so `context deadline exceeded` failures are diagnosable.
- Apiserver warnings (Pod Security, deprecated APIs, unknown fields) appear as `I… warnings.go:107] "Warning: …"` lines on stderr; the command still succeeds.
- `helm template --debug` renders invalid YAML for inspection (`Use --debug flag to render out invalid YAML`).
- Scripts must parse structured output (`helm list -o json`, `helm status -o json`, `helm get values -o yaml`, `helm get metadata -o json` [3.13]), not human tables.
- `helm status --show-resources` [3.11] lists the release's resources; `--show-desc` the last description.
- `HELM_PPROF_CPU_PROFILE=<file>` / `HELM_PPROF_MEM_PROFILE=<file>` [3.18] write CPU/memory profiles.

***

#### Migrating to Helm 4 — Notes

Helm 3 receives security fixes only until 2027-02-10. Helm 4 manages existing Helm 3 releases in place (same release Secret format; no conversion step), and by default keeps each existing release on client-side apply until it is upgraded with `--server-side=true`. Prepare charts and pipelines while still on Helm 3:

| Area | Helm 3 behaviour | Helm 4 behaviour | Do now on Helm 3 |
|---|---|---|---|
| Apply | Client-side three-way merge (manager `helm`, `Update`) | Server-side apply for new releases; field-ownership conflicts fail the upgrade | Stop rendering controller-owned fields (`replicas` with HPA, injected `caBundle`) |
| Unknown/misspelled fields | Warning, field dropped | Install fails (`field not declared in schema`) | Treat `unknown field` warnings as errors |
| Duplicate keyed-list entries | Warning; upgrades edit the wrong entry | Fails (`duplicate entries for key`) | Deduplicate `env`, `ports`, `volumeMounts` |
| Duplicate objects in one release | Install fails (`already exists`) | Applied twice without error | Review names for collisions |
| `--atomic`, `--force` | Supported | Deprecated aliases of `--rollback-on-failure`, `--force-replace` (`install --atomic` missing before 4.1.3) | Keep, but plan the rename |
| `--wait` | Boolean; ignores custom resources and Job completion | Strategies; bare `--wait` means `watcher` (kstatus), which also waits for custom resources' `Ready` conditions; without `--wait` only hooks are awaited | Make sure operators set `Ready` conditions; set `--timeout` explicitly |
| Post-renderer | Executable path; hooks not post-rendered; empty output accepted | Plugin name (`postrenderer/v1`); hooks post-rendered; empty output is an error | Package the script as a plugin; make overlay selectors precise |
| Plugins | Legacy `plugin.yaml` | v1 format, signed tarballs required by default (4.1.4+) | Inventory plugins; separate data directories |
| Values files | First YAML document only | All documents merged | Remove stray `---` |
| `--set-json` | `key=json` | Also a whole JSON object | — |
| `helm lint` | Path optional; unknown `Chart.yaml` fields ignored; `"1.0"` accepted | Path required; unknown fields and non-SemVer versions warned (fail with `--strict`) | Use `helm lint <path>`; fix `Chart.yaml` keys and versions |
| `helm list` | deployed+failed by default; `-a` for all | All statuses by default; `-a` removed | Use explicit filters (`--deployed`, `--failed`) |
| `helm registry login host/path` | Accepted | Rejected | Use the host only |
| Removed flags | — | `version --client`, `status --show-desc/--show-resources`, `repo add --no-update`, `repo update --fail-on-repo-update-fail`, `rollback --recreate-pods`; `template --validate` deprecated | Remove them from scripts |
| Template errors | One line `Error: template: …` | Multi-line for engine execution errors | Match on exit codes, not messages |
| Chart archives | New digest on every `helm package` | Deterministic; `SOURCE_DATE_EPOCH` in 4.3 | — |
| Uninstall | Deletes every listed object | 4.3 skips objects owned by other releases | Avoid `--take-ownership` |
| Chart API | v2 | v2 (v3 experimental, not usable) | Keep `apiVersion: v2` |

Do not mix majors on one release: a Helm 3 upgrade of a release that Helm 4 manages with server-side apply silently switches it back to client-side apply. Migrate one environment at a time with Helm pinned per pipeline.

***

#### Production / Review Checklist

##### Helm 3 version and support

- [ ] Helm pinned to an exact 3.22.x version (latest patch) in every CI job and image; no `latest` installer (now Helm 4)
- [ ] No feature newer than the pinned minor (`--set-json` 3.10, `--set-literal` 3.12, `--dry-run=server` 3.13, `--reset-then-reuse-values` 3.14, `--hide-secret` 3.15, `--skip-schema-validation`/`sha512sum` 3.16, `--take-ownership`/digests/`toYamlPretty`/`fromToml` 3.17, JSON Schema 2020-12 and `hook-output-log-policy` 3.18)
- [ ] No Helm 4-only flags or functions (`--server-side`, `--force-conflicts`, `--rollback-on-failure`, `--force-replace`, `--wait=watcher`, `mustToYaml`, `mustToToml`, `duration*`, multi-document values)
- [ ] A Helm 4 migration plan exists (security support ends 2027-02-10)

##### Releases and deploys

- [ ] Post-renderer scripts fail on empty output (Helm 3 otherwise deploys nothing or deletes every release object)
- [ ] `helm upgrade --install` with explicit namespace, full values files, `--atomic`, explicit `--timeout`; CI step timeout longer than `--timeout`
- [ ] No rendered fields owned by other controllers (`replicas` with HPA/KEDA, injected `caBundle`, sidecars)
- [ ] No duplicate object names, no duplicate `env`/`ports` keys; `unknown field` warnings treated as errors
- [ ] Selector changes reviewed as breaking (partial apply on failure)
- [ ] `--wait` limits understood: custom resources and regular Jobs need explicit checks (`--wait-for-jobs` only with `--wait`)
- [ ] No `--force`, `--take-ownership`, `--reuse-values`, `--no-hooks`, `--skip-schema-validation`, `--enable-dns`, `--pass-credentials` as defaults
- [ ] Stuck `pending-*` releases handled with `helm rollback`/`helm uninstall`, not by deleting release Secrets

##### Chart metadata and structure

- [ ] `apiVersion: v2`; SemVer 2 `version` (three parts, quoted), bumped on every change; quoted `appVersion`
- [ ] `Chart.yaml` keys spelled correctly (Helm 3 lint does not report unknown keys)
- [ ] `kubeVersion` and every `semverCompare` against the Kubernetes version use the `-0` suffix
- [ ] `Chart.lock` committed; CI uses `helm dependency build`
- [ ] Chart, template and value names follow conventions; every `define` prefixed with the chart name
- [ ] `.helmignore` excludes secrets, CI and test data

##### Templates

- [ ] `include` (not `template`) with `nindent`; indentation verified by rendering
- [ ] `$` used inside `range`/`with` for the root context
- [ ] No `default`/`with` on values where `false`/`0` are valid; `hasKey`/`kindIs` instead
- [ ] `required`/`fail`/schema for mandatory and cross-field rules
- [ ] Strings quoted; integers the API expects as numbers not quoted; env/annotation/label values are strings
- [ ] Names truncated to 63 characters including suffixes; selector labels limited to name/instance
- [ ] No `generateName`; no random/time functions without `lookup` preservation; valid output when `lookup` is empty
- [ ] `tpl` only on documented, trusted values; `dig` on `.Values.AsMap`

##### Values and schema

- [ ] Every value documented; secure defaults; no secrets in defaults; `existingSecret` supported
- [ ] Maps preferred over lists for user-overridable collections
- [ ] YAML 1.1 traps quoted (`"NO"`, `"y"`, `"1.10"`, large integers); single-document values files
- [ ] `values.schema.json` with types, enums, ranges; draft matches the minimum Helm (2020-12 needs ≥ 3.18); `additionalProperties: false` only on owned nested objects; no network `$ref`

##### Workloads and security

- [ ] Restricted Pod Security profile for workloads, hooks and test Pods
- [ ] `automountServiceAccountToken: false` unless needed; least-privilege RBAC behind a value
- [ ] Requests and memory limit; readiness, liveness, startup probes on named ports
- [ ] PDB with one of `minAvailable`/`maxUnavailable` (0 honoured), spread constraints, HPA without rendered `replicas`
- [ ] Images pinned by tag and preferably digest; checksum annotations for env-consumed ConfigMaps

##### Hooks, tests, CRDs

- [ ] Every hook has `hook-delete-policy`; weights are strings and distinct where order matters (equal weights sort by name, not kind)
- [ ] Hook Jobs bounded (`backoffLimit`, `activeDeadlineSeconds`); `pre-install` hooks do not depend on regular resources
- [ ] Test Pods pass Pod Security; pinned images; no `hook-succeeded` on test Pods when CI runs `helm test --logs`
- [ ] CRDs in `crds/` (never upgraded by Helm) or a separate chart; CRs gated with `.Capabilities`

##### Supply chain

- [ ] Charts pushed to OCI once per version; deployments reference the pushed digest (≥ 3.17)
- [ ] Charts signed (`.prov` with an exported legacy keyring, or cosign) and verified in the deploy path
- [ ] Registry login via `--password-stdin` with short-lived tokens; credentials not persisted on runners
- [ ] Helm ≥ 3.20.2 (better: latest 3.22.x) wherever untrusted charts are pulled, linted or have dependencies updated
