### Helm Style Guide

> **Version**: This is the **version-independent** Helm guide. It applies when the project's Helm version cannot be determined, and covers charts with `apiVersion: v2` as used by Helm 3 (3.0–3.22) and Helm 4 (4.0–4.3). The current major is **Helm 4**: v4.0.0 was released on **2025-11-12**; the latest release as of October 2026 is **v4.3.1 (2026-10-15)**.[^1][^2] **Helm 3 is in a security-only phase**: v3.22.0 (2026-09-09) is its last minor release, the latest patch is v3.22.1 (2026-10-15), and security fixes end on **2027-02-10**.[^3][^4]
> **Scope**: Version-independent review rules for Helm charts (`Chart.yaml`, `Chart.lock`, `values.yaml`, `values.schema.json`, `templates/`, `_helpers.tpl`, `NOTES.txt`, `crds/`, chart tests, library charts) and for `helm` usage in scripts and CI: upgrade/rollback semantics, hooks, secrets, OCI publishing, provenance, testing and security.
> **Version rule**: Every feature tied to a Helm version is labelled with the version that introduced it, for example "`--set-json` (3.10)", "`--dry-run=server` (3.13)", "install by OCI digest (3.17)", "`--rollback-on-failure` (4.0)", "`mustToToml` (4.2)". Before suggesting such a feature, determine the project's Helm version (see "Determine the Helm Version First"). Never recommend a feature the pinned version lacks, and never "fix" a Helm 3 flag into its Helm 4 spelling (or the reverse) without knowing which major runs the command.
> **Note**: This document is self-contained. Every chart example was linted (`helm lint --strict`) and rendered (`helm template`) on **Helm v3.22.1, v4.0.5 and v4.3.1**; install, upgrade, drift, wait, hook and dry-run behaviour was exercised against a real **kube-apiserver v1.36.2** (no kubelet or controllers, so Pods never run; readiness of Pods and Deployments is described from the Helm sources and documentation). Behaviour against vendor-suffixed Kubernetes versions (`v1.36.2-eks-…`) was checked through a proxy that rewrites the apiserver's `/version` response. Tooling outside Helm itself (`helm-unittest` 1.2.1 was run; `ct`/chart-testing and kubeconform were not) is marked where it is only sourced.

---

#### Determine the Helm Version First

Helm behaviour differs substantially between the two majors (apply method, wait semantics, flag names, post-renderers, values parsing). Find the version before commenting on flags, functions or deploy scripts:

- `.tool-versions` / `mise.toml` (`helm 3.22.1`), `aqua.yaml`, `devbox.json`.
- CI: `azure/setup-helm` `with: version:`, `helm/kind-action`, `HELM_VERSION` in Makefiles and Dockerfiles, container images (`alpine/helm:4.3.1`, `dtzar/helm-kubectl:…`).
- GitOps controllers render charts with their own embedded Helm library (Argo CD, Flux helm-controller); the CLI version in CI is then irrelevant for rendering. Check the controller's release notes for the Helm version it bundles.
- Flags in scripts reveal the major: `--atomic`, `--force`, `--post-renderer ./script.sh`, `helm list -a` are Helm 3 spellings; `--rollback-on-failure`, `--force-replace`, `--server-side`, `--force-conflicts`, `--wait=watcher` exist only in Helm 4.

Review rules:

- **An unpinned installer installs Helm 4.** `azure/setup-helm` defaults to `latest`, so a job without `version:` switched majors when Helm 4 was released, with different apply, wait and flag semantics.[^5] Flag a missing or `latest` version.
- **Say "not available" only when the pinned version proves it** (unknown flag, `function "…" not defined`). The default documentation at helm.sh describes Helm 4; Helm 3 documentation lives under `https://helm.sh/docs/v3/`.
- **A chart is often consumed by both majors.** A chart published for others must render on Helm 3 and Helm 4 unless it declares otherwise; a chart that only one pipeline consumes may use that pipeline's version.
- `.Capabilities.HelmVersion.Version` (3.3) reports the running Helm (`v3.22.1`, `v4.3.1`). A chart can enforce a minimum version:

```yaml
{{- if not (semverCompare ">=3.17.0-0" .Capabilities.HelmVersion.Version) }}
{{- fail "this chart requires Helm >= 3.17.0" }}
{{- end }}
```

  The guard cannot protect a template that calls a function the running Helm lacks: Go templates fail at **parse** time, before any `if` runs (`parse error at (…): function "mustToYaml" not defined` on 3.22.1).

***

#### Release and Support Status (October 2026)

| Line | First release | Latest release | Status | Supported Kubernetes (n-3) |
|---|---|---|---|---|
| Helm 4.3 | 4.3.0, 2026-09-09 | **4.3.1, 2026-10-15** | Current; next patch 4.3.2 scheduled 2026-11-11, 4.4.0 scheduled 2027-01-13 | 1.34–1.37 |
| Helm 4.2 | 4.2.0, 2026-05-14 | 4.2.4, 2026-08-13 | No further patches (only the latest minor is patched) | 1.33–1.36 |
| Helm 4.1 | 4.1.0, 2026-01-21 | 4.1.4, 2026-04-09 | No further patches | 1.32–1.35 |
| Helm 4.0 | 4.0.0, 2025-11-12 | 4.0.5, 2026-01-14 | No further patches; lacks the 4.1.4 plugin-verification security fixes | 1.31–1.34 |
| Helm 3.22 | 3.22.0, 2026-09-09 | **3.22.1, 2026-10-15** | **Last Helm 3 minor; security fixes only, until 2027-02-10**; 3.22.2 scheduled 2026-11-11 | 1.34–1.37 |
| Helm 3.0–3.21 | 3.0.0, 2019-11-13 | — | Unsupported | — |

Sources: release pages, the Helm v3 end-of-life announcement and the version skew policy.[^1][^2][^3][^4][^6] The Helm 4 announcement originally planned Helm 3 bug fixes until 2026-07-08 and security fixes until 2026-11-11; the end-of-life post of 2026-06-02 moved the last minor to 2026-09-09 and extended security fixes to 2027-02-10.[^3][^7] After that date Helm 3 receives no updates, including no Kubernetes client-library updates, so clusters newer than 1.37 are outside Helm 3's support range.

Review consequences:

- Pin the **latest patch of a supported line**: 4.3.x, or 3.22.x while migrating. Any other Helm 3 minor and any Helm 4 minor older than 4.3 receives no fixes.
- A project still on Helm 3 needs a migration plan to Helm 4 before 2027-02-10.
- `helm version` on Helm 4 also prints `KubeClientVersion` (`v1.37` for 4.3.1, `v1.34` for 4.0.5).

***

#### Helm 3 vs Helm 4: Differences a Reviewer Must Know

Helm 4 keeps the chart format: `apiVersion: v2` charts render unchanged. It changes how releases are applied, how readiness is judged, how post-renderers and plugins work, and several CLI flags.[^8][^9] The apply, drift, unknown-field, roll-back, replace, wait, post-renderer, values-file, `--set-json`, template-function, template-error, `helm lint` and `helm list` rows were re-verified on v3.22.1 and v4.0.5/v4.3.1; the remaining rows come from the release notes, changelog and the version-specific verification.

| Area | Helm 3 | Helm 4 |
|---|---|---|
| **Apply method** | Client-side three-way merge patch (field manager `helm`, operation `Update`) | **Server-side apply** for new releases (manager `helm`, operation `Apply`); upgrades and rollbacks keep the release's previous method (`--server-side=auto`); `helm get metadata` shows `APPLY_METHOD: server-side apply` |
| **Drift on chart fields** | Silently reset to the chart value on the next upgrade | Upgrade **fails** with `conflict with "<manager>"` when the rendered value differs from a live value another manager owns; `--force-conflicts` takes ownership |
| **Unknown/misspelled manifest fields** | Warning (`unknown field "datax"`), field dropped, release `deployed` | Install/upgrade fails: `.datax: field not declared in schema` |
| **Duplicate keys in keyed lists** (two `env` entries with the same `name`) | Warning; later upgrades may patch the wrong entry | Fails: `duplicate entries for key [name="A"]` |
| **Roll back on failure** | `--atomic` | `--rollback-on-failure`; `--atomic` is a deprecated alias on `upgrade`, and on `install` only since 4.1.3 (`unknown flag: --atomic` on 4.0.5) |
| **Replace objects** | `--force` | `--force-replace`; `--force` prints `Flag --force has been deprecated, use --force-replace instead` |
| **`--wait`** | Boolean; polls Pods, PVCs, Services, Deployments, StatefulSets, DaemonSets, ReplicaSets; ignores custom resources | Strategy: bare `--wait` = `watcher` (kstatus, also waits for custom resources' `Ready` conditions), `legacy` (Helm 3 rules), `hookOnly` (the default when `--wait` is omitted; explicit spelling since 4.1) |
| **Post-renderer** | `--post-renderer ./script.sh` (executable); hooks are **not** post-rendered; empty output accepted | `--post-renderer <plugin-name>` (`postrenderer/v1` plugin); hooks **are** post-rendered; empty output is an error |
| **Plugins** | Legacy `plugin.yaml` | New `apiVersion: v1` plugin format (types `cli/v1`, `getter/v1`, `postrenderer/v1`; runtimes `subprocess`, `extism/v1` WebAssembly); legacy plugins still load; tarball installs require a `.prov` signature (enforced since 4.1.4); VCS installs need `--verify=false` |
| **Values files** | Only the **first** YAML document of a values file is used | All `---`-separated documents are merged in order (also the chart's own `values.yaml`) |
| **`--set-json`** | `key=<json>` only (3.10) | Also a whole JSON object: `--set-json '{"a":{"b":1}}'` |
| **Template functions** | Sprig + Helm functions | Adds `mustToYaml` (4.0), `mustToToml` (4.2), `durationSeconds`, `mustToDuration` and other `duration*` helpers (4.3) |
| **Template errors** | One line `Error: template: file:line:col: executing …` | Most engine execution errors (wrong argument type, `can't evaluate field`) are multi-line (`Error: file:line:col` then details); function-call errors (`error calling dig: …`), `required`/`fail` and parse errors keep the one-line form |
| **`helm lint`** | Path optional (defaults to `.`); unknown `Chart.yaml` fields ignored | **Path required**; unknown `Chart.yaml` fields and non-SemVer-2 versions produce warnings (fail with `--strict`) |
| **`helm list`** | `deployed` and `failed` only; `-a/--all` for everything | Every status by default; `-a` is removed (`unknown shorthand flag: 'a'`) |
| **`helm registry login`** | Host or host/path | Host only |
| **Removed flags** | — | `helm version --client`, `helm status --show-desc/--show-resources`, `helm repo add --no-update`, `helm repo update --fail-on-repo-update-fail`, `helm rollback --recreate-pods`; `helm template --validate` deprecated (use `--dry-run=server`) |
| **Chart archives** | Every `helm package` embeds the current time (new digest each run) | Deterministic (file modification times); `SOURCE_DATE_EPOCH` honoured since 4.3 |
| **Uninstall** | Deletes every object in the release manifest | 4.3 skips objects whose `meta.helm.sh/release-name`/`-namespace` point to another release |
| **Logging** | klog-style `[debug]` lines and `warnings.go` | `log/slog` (`level=WARN msg=…`); `--color`, `HELM_COLOR`, `NO_COLOR` |
| **Chart API v3** | `helm lint`: `apiVersion 'v3' is not valid` | Experimental only (HIP-0020, behind `HELM_EXPERIMENTAL_CHART_V3`); in 4.3.1 `helm template`/`install` of an `apiVersion: v3` chart fails with `invalid chart apiVersion`. **Charts must use `apiVersion: v2`.**[^10] |

Verified examples:

```console
# Same chart, Deployment scaled with "kubectl scale --replicas=5", then "helm upgrade" without changes
$ helm upgrade r web -n w3 ...          # Helm 3.22.1: STATUS: deployed, replicas reset to 2
$ helm upgrade r web -n w43 ...         # Helm 4.3.1:
Error: UPGRADE FAILED: conflict occurred while applying object w43/r-web apps/v1, Kind=Deployment:
Apply failed with 1 conflict: conflict with "kubectl" with subresource "scale" using apps/v1: .spec.replicas

# Post-renderer that prints nothing
$ helm install pe t --post-renderer ./empty.sh       # Helm 3.22.1: STATUS: deployed, no objects created
$ helm template x t --post-renderer empty-pr         # Helm 4.0.5 / 4.3.1:
Error: error while running post render on files: post-renderer "empty-pr" produced empty output

# Custom resource with status.conditions Ready=False, "--wait --timeout 10s"
Helm 3.22.1: STATUS: deployed
Helm 4.0.5:  Error: INSTALLATION FAILED: resource not ready, name: g, kind: Gadget, status: InProgress
Helm 4.3.1:  Error: INSTALLATION FAILED: resource Gadget/k/g not ready. status: InProgress, message: waiting for backend
```

Review consequences that hold for every chart, whichever major runs it:

- Do not render fields another controller owns (`spec.replicas` with an HPA/KEDA, webhook-injected `caBundle`): Helm 3 silently fights the controller, Helm 4 fails the upgrade.
- Treat `unknown field` warnings and duplicate list keys as errors on Helm 3; they become hard failures on Helm 4.
- Do not use multi-document `values.yaml` in a chart that Helm 3 users may install: Helm 3 silently truncates it to the first document.
- Helm 4-only template functions make the chart unusable on Helm 3 (parse error).
- Do not mix majors on one release. A Helm 3 upgrade of a release that Helm 4 manages with server-side apply switches it back to client-side apply; Helm 4 then keeps `csa` until `--server-side=true` is passed.[^9]
- Run Helm 3 and Helm 4 side by side only with separate `HELM_CONFIG_HOME`, `HELM_DATA_HOME` and `HELM_CACHE_HOME`: a single v1-format plugin makes Helm 3.22.1 fail to load plugins (`json: unknown field "apiVersion"`).

***

#### Version-Tied Features: Check Before Recommending

Only suggest a feature from these tables after confirming the pinned version has it. On an older version, flags fail with `unknown flag`, functions with `function "…" not defined`.

##### Helm 3 minors

| Feature | Needs |
|---|---|
| `lookup`, `--post-renderer` (executable), `--disable-openapi-validation` | 3.1 |
| `--create-namespace`, `fromYamlArray`, `fromJsonArray` | 3.2 |
| `.Capabilities.HelmVersion`; lint type checks for `version`/`appVersion` | 3.3 |
| `--wait-for-jobs` | 3.5 |
| `--kube-version` on `helm template` | 3.6 |
| `.Chart.IsRoot`, `helm uninstall --wait`, `helm show crds`, `helm push`/`helm pull oci://` | 3.7 |
| OCI support GA (no `HELM_EXPERIMENTAL_OCI`) | 3.8 |
| `--post-renderer-args` | 3.9 |
| `--set-json key=<json>` | 3.10 |
| `getHostByName` returns `""` unless `--enable-dns` | 3.11.1 |
| `--set-literal`, `helm uninstall --cascade` | 3.12 |
| `--dry-run=client\|server\|none`, `helm get metadata`, `--labels`, `--plain-http` | 3.13 |
| `--reset-then-reuse-values`, `helm lint --kube-version` | 3.14 |
| `--hide-secret` | 3.15 |
| `--skip-schema-validation`, `--hide-notes`, `sha512sum` | 3.16 |
| `oci://…@sha256:<digest>` references, `--take-ownership`, `toYamlPretty`, `fromToml` | 3.17 |
| JSON Schema draft 2019-09/2020-12 in `values.schema.json`, `helm.sh/hook-output-log-policy` | 3.18 |
| Gateway API `HTTPRoute` in the `helm create` scaffold | 3.19 |
| Unresolvable URN `$ref` in a schema tolerated (warning) | 3.20 |

Sources: Helm 3 release notes, boundaries re-checked in the Helm 3 guide's verification on the latest patch of each minor.[^11]

##### Helm 4 (never available in Helm 3)

| Feature | Needs |
|---|---|
| Server-side apply, `--server-side`, `--force-conflicts` | 4.0 |
| `--rollback-on-failure`, `--force-replace` | 4.0 |
| `--wait=watcher\|legacy` (kstatus) | 4.0 |
| `--wait=hookOnly` written explicitly | 4.1 |
| `helm install --atomic` as a deprecated alias | 4.1.3 (4.0.x–4.1.1: unknown flag) |
| Post-renderer plugins (`postrenderer/v1`), v1 plugin format, `helm plugin package/verify` | 4.0 |
| Unsigned plugin tarballs rejected by `helm plugin install` | 4.1.4 (CVE-2026-35205) |
| Multi-document values files, `--set-json '<object>'` | 4.0 |
| `mustToYaml` | 4.0 |
| `mustToToml` | 4.2 |
| `durationSeconds`, `mustToDuration`, `durationRoundTo` and the other `duration*` helpers | 4.3 |
| `oci://…/chart:1.2.3@sha256:…` (tag **and** digest) | Helm 3.17+ and Helm 4.1.3+ (broken in 4.0.x: `encoding/hex: invalid byte`) |
| `helm repo list --no-headers` | 4.1 |
| `helm rollback --description`, `helm history --show-rollback-revision` | 4.3 |
| `SOURCE_DATE_EPOCH` for reproducible `helm package` | 4.3 |
| `pubring.kbx` keyrings for `--verify`/`helm verify` | 4.3 |
| Uninstall ownership check | 4.3 |

Sources: Helm 4 release notes and changelog.[^2][^8][^12]

Also never recommend:

- **`apiVersion: v3` charts** (experimental, unusable in 4.3.1) or anything described as "Helm 5".
- **Template functions from plugins**: custom functions exist only in the Go SDK; there is no CLI plugin type for them.
- **`HELM_EXPERIMENTAL_OCI=1`** (not needed since 3.8) or `helm chart save/push` (removed in 3.7).
- **`crd-install` hooks** (Helm 2; ignored since 3.0) and `helm.sh/hook: test-success` (Helm 2 spelling of `test`).
- **`helm rollback --recreate-pods`**: still in Helm 3, removed in Helm 4; use a pod-template checksum annotation.
- **The retired `stable`/`incubator` repositories** (`kubernetes-charts.storage.googleapis.com`).

***

#### `Chart.yaml`

```yaml
apiVersion: v2                 # the only chart API for real charts in Helm 3 and Helm 4
name: web                      # lowercase letters, digits, dashes; equals the directory name
description: Example web application chart
type: application              # or "library"
version: 1.4.0                 # chart version, SemVer 2; bump on every chart change
appVersion: "2.7.1"            # quoted; informational, used as the default image tag
kubeVersion: ">=1.30.0-0"      # "-0" lets vendor builds (v1.31.2-eks-…) match
home: https://example.com/web
icon: https://example.com/web/icon.png
sources:
  - https://github.com/example/web
maintainers:
  - name: Platform Team
    email: platform@example.com
```

Rules:

- **`apiVersion: v2`.** `v1` (Helm 2 format with `requirements.yaml`) still loads but must not be used for new charts; `v3` is a lint error on Helm 3 and fails to render on Helm 4.3.
- **`version` is SemVer 2 with three components**, quoted only if needed for YAML. `version: 1.0` (a YAML float) is a lint error since 3.3 and on Helm 4 (`version should be of type string but it's of type float64`). `version: "1.0"` passes Helm 3 lint silently but Helm 4 warns `version '1.0' is not a valid SemVerV2` and `helm lint --strict` fails. OCI tags, `Chart.lock` resolution and dependency ranges rely on valid SemVer.
- **Bump `version` on every change** to the chart. Republishing the same version with different content breaks caches, digest pins and `Chart.lock` digests.
- **`version` and `appVersion` are independent.** `version` describes the chart, `appVersion` the packaged application. Do not conflate them.
- **Quote `appVersion`.** Unquoted `appVersion: 2.10` is a lint error (`appVersion should be of type string but it's of type float64`), yet `helm template`/`install` proceed with `appVersion: "2.1"` — and the default image tag becomes `2.1`.
- **Spell keys exactly.** Helm 3 silently ignores unknown keys (`appversion:`, `kubeversion:`, `dependency:`); Helm 4 lint warns `failed to strictly parse chart metadata file … unknown field`. Review spelling either way.
- `icon` missing is only an `[INFO]` lint message.

##### `kubeVersion` and the `-0` pre-release trap

Managed Kubernetes reports versions with a vendor suffix (`v1.31.2-eks-7f9249a`, `v1.30.5-gke.1014001`). In SemVer the suffix makes it a **pre-release**, and a constraint without a pre-release part excludes pre-releases. Verified against an apiserver reporting `v1.36.2-eks-7f9249a`, on 3.22.1, 4.0.5 and 4.3.1 alike:

```console
# kubeVersion: ">=1.30.0"
Error: INSTALLATION FAILED: chart requires kubeVersion: >=1.30.0 which is incompatible with Kubernetes v1.36.2-eks-7f9249a
# kubeVersion: ">=1.30.0-0": the same `helm install --dry-run=server` renders and succeeds
```

- **Always write `kubeVersion` with `-0`**: `">=1.30.0-0"`, `">=1.30.0-0 <1.38.0-0"`.
- **CI does not catch this.** Rendering offline with `helm template --kube-version 1.31.2-eks-7f9249a` passes the `kubeVersion: ">=1.30.0"` check on all three versions tested (the chart check compares without the suffix), so the failure only appears on the real cluster.
- The same rule applies to every `semverCompare` against `.Capabilities.KubeVersion.Version`: against a live EKS/GKE cluster the suffix is always present, and `semverCompare ">=1.30.0" "v1.36.2-eks-7f9249a"` is `false`. Offline, `--kube-version` with a suffix is stripped to `v1.31.2` by Helm 3.19+ and 4.0.x, but kept by Helm 4.1+ (verified on 4.3.1), so the same template renders differently in CI depending on the Helm version. Write `-0` everywhere.
- A genuine mismatch fails rendering and install: `Error: chart requires kubeVersion: >=1.30.0-0 which is incompatible with Kubernetes v1.29.0`.

##### Dependencies

```yaml
dependencies:
  - name: postgresql
    version: "16.4.1"                         # exact pin (house rule); ranges are resolved into Chart.lock
    repository: oci://registry-1.docker.io/bitnamicharts
    condition: postgresql.enabled
  - name: common
    version: "~1.0.0"
    repository: "file://../common"
```

- Prefer exact dependency pins (a house rule for reproducibility, not a Helm requirement). When ranges are used, `Chart.lock` pins the resolved versions; review its diff on every update.
- **Commit `Chart.lock`.** CI runs `helm dependency build` (installs exactly the locked versions and fails when `Chart.yaml` and the lock disagree: `the lock file (Chart.lock) is out of sync with the dependencies file (Chart.yaml)`); `helm dependency update` is for intentional version changes only.
- Without built dependencies `helm template` fails: `found in Chart.yaml, but missing in charts/ directory: …`.
- `oci://` dependency repositories work since 3.8 (ranges included). Since 3.14 dependency names and aliases must be unique.
- See "Dependencies, Subcharts and Library Charts" for values scoping and template-name collisions.

***

#### Chart Layout, Naming and Kubernetes Short Names

```text
web/
├── Chart.yaml
├── Chart.lock              # committed when dependencies exist
├── values.yaml             # documented defaults
├── values.schema.json      # validated on install/upgrade/template/lint
├── README.md               # values table (helm-docs or similar)
├── .helmignore             # keeps CI files, unit tests, secrets out of the package
├── crds/                   # plain CRD YAML: installed once, never templated or upgraded
├── charts/                 # built dependencies (usually gitignored, rebuilt from Chart.lock)
├── tests/                  # helm-unittest suites (not packaged, see .helmignore)
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
    ├── migrate-job.yaml
    └── tests/
        └── test-connection.yaml
```

Naming rules (from the chart best practices, still valid for both majors):[^13]

- **Chart names**: lowercase letters, digits and dashes (`my-app`); no uppercase, underscores or dots. The directory name equals `name`.
- **Template files**: dashed lowercase (`pod-disruption-budget.yaml`); `.yaml` for manifests, `.tpl` for helpers without output. One resource kind per file; the file name reflects the kind. Files starting with `_` produce no output.
- **Defined template names are global** across the parent and all subcharts: prefix every `define` with the chart name (`web.fullname`, not `fullname`). When two charts define the same name, one silently overrides the other.
- **Values keys**: camelCase starting with a lowercase letter (`replicaCount`, `podSecurityContext`). Keys with dashes cannot be read with dot notation (`.Values.my-key` is a parse error; `index .Values "my-key"` works).
- **Resource names**: lowercase with hyphens (`my-app-worker`, not `myAppWorker`), produced by the `*.fullname` helper, at most 63 characters (DNS-1123 label) with `trimSuffix "-"`. Names that append a suffix must budget for it: `{{ include "web.fullname" . | trunc 55 | trimSuffix "-" }}-migrate`.
- Chart test Pods live in `templates/tests/`; `helm-unittest` suites live in a top-level `tests/` directory.

##### `.helmignore` pitfall: `tests/` also drops chart tests

`.helmignore` patterns without a leading slash match at any depth. Verified on 3.22.1, 4.0.5 and 4.3.1: a `.helmignore` line `tests/` excludes **both** the unit-test directory and `templates/tests/`, so the packaged chart silently loses its `helm test` Pods.

```text
# BAD: also removes templates/tests/test-connection.yaml from the package
tests/

# GOOD: anchored to the chart root
/tests/
```

Also exclude `.git/`, CI configuration, `.env` files, local values files with secrets and large fixtures; everything not ignored ends up in the published archive.

##### Kubernetes short names

When an abbreviation of a kind appears in resource-name suffixes, helper names, values keys, comments or `NOTES.txt` commands, use the official `kubectl` short names (verified with `kubectl api-resources` on v1.36):

| Kind | Short name |
|---|---|
| ConfigMap | `cm` |
| Service | `svc` |
| ServiceAccount | `sa` |
| Deployment | `deploy` |
| StatefulSet | `sts` |
| DaemonSet | `ds` |
| ReplicaSet | `rs` |
| **ReplicationController** | **`rc`** |
| CronJob | `cj` |
| PersistentVolumeClaim / PersistentVolume | `pvc` / `pv` |
| HorizontalPodAutoscaler | `hpa` |
| PodDisruptionBudget | `pdb` |
| Ingress | `ing` |
| NetworkPolicy | `netpol` |
| CustomResourceDefinition | `crd` |
| Namespace | `ns` |
| Pod | `po` |

`rc` means ReplicationController and nothing else: never use it for a ConfigMap, a "release candidate", a "resource config" or a role binding (RoleBinding and ClusterRoleBinding have no short name; spell them out). Kinds without a short name (Job, Secret, Role, RoleBinding, ClusterRole) are written in full.

***

#### Values Design

The example chart's `values.yaml` (rendered on all three versions; installed into a namespace with Pod Security Admission `enforce=restricted` without warnings):

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

# -- Extra environment variables as a map (name -> value). Merged by key.
env: {}

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

- **Document every key** with a comment directly above it (`# --` is the helm-docs convention).
- **Secure defaults**: non-root, no privilege escalation, all capabilities dropped, `seccompProfile: RuntimeDefault`, read-only root filesystem, service-account token not mounted. Users opt *out* of security, not in.
- **Every optional feature has an `enabled` flag** with a safe default; templates gate on it.
- **No secrets in defaults.** Support `existingSecret`; never ship a default password.
- **Prefer maps over lists for collections users override.** Values merging and `--set` replace lists wholesale (overriding one `env` entry means repeating the whole list), and duplicate list keys break on Helm 4. A map (`env: {LOG_LEVEL: info}`) merges key by key.
- **Flat where possible, nested where grouped** (`image.repository`, `service.port`); deep nesting makes `--set` paths unwieldy.
- **Pass Kubernetes structures through unchanged** (`resources`, `affinity`, `tolerations`) with `toYaml`; do not re-model them as separate scalars without a reason.
- **`global`** is shared with every subchart; use it only for values that genuinely apply to all (registry, pull secrets).
- **Defaults must install**: `helm install x ./chart` alone must produce a working release or fail early with `required`/schema errors.

##### Precedence and merging

Chart `values.yaml` < parent chart values for a subchart < `-f/--values` files (left to right) < `--set-json` < `--set` < `--set-string` < `--set-file` < `--set-literal`. Maps are deep-merged; lists and scalars are replaced.

- `--set a=x,y` splits on commas (`key "y" has no value`); `--set-literal` (3.12) sets the literal string.
- `--set port=8080` yields an integer, `--set-string port=8080` a string, `port: 8080` in a YAML file a `float64`.
- `--set-json 'resources={"limits":{"memory":"1Gi"}}'` (3.10) sets structured values; the whole-object form `--set-json '{"a":1}'` is Helm 4 only (Helm 3: `failed parsing --set-json data`).

##### Deleting a default with `null`

`--set resources.limits=null` (or `limits: null` in a values file) removes the key from a non-empty chart default. Under an **empty-map** default (`annotations: {}`), a user `a: null` is kept as `a: null` on 3.20.1+, 3.22.1 and 4.1.3+ (verified on 3.22.1 and 4.3.1) but dropped on 4.0.5 and Helm ≤ 3.20.0. Templates that range over user maps must tolerate `nil` values (`{{- if not (kindIs "invalid" $v) }}`).

##### Implicit value reuse on upgrade

`helm upgrade` **without any** `-f`/`--set*` flag reuses the previous release's user values; with at least one values flag, only chart defaults plus the new flags apply (same on both majors). Consequences:

- CI must always pass the complete values (`-f values/prod.yaml`), so the result does not depend on whether a flag happens to be present.
- Flag `--reuse-values` in pipelines: it keeps values of keys the new chart version renamed or removed and ignores new chart defaults. `--reset-then-reuse-values` (3.14) applies new defaults first.

##### YAML typing traps

Values are parsed as YAML 1.1 and converted through JSON.[^14] Verified identically on 3.22.1, 4.0.5 and 4.3.1:

| values.yaml | `.Values` sees | Fix |
|---|---|---|
| `country: NO` | `false` | `country: "NO"` |
| `list: [x, y]` | `["x", true]` | `["x", "y"]` |
| `keys: {y: 2}` | key `"true"` | `{"y": 2}` |
| `version: 1.10` | `1.1` | `version: "1.10"` |
| `big: 12345678901234567` | `float64`; `quote` prints `"1.2345678901234568e+16"` | keep as a quoted string |
| `port: 8080` | `float64`; `quote` prints `"8080"` | fine for numbers |

The same YAML rules apply to rendered manifests: an unquoted `name: n` renders the boolean `false`. Quote free-form strings in templates.

***

#### `values.schema.json`

Helm validates the **merged** values against `values.schema.json` on `install`, `upgrade`, `template` and `lint`; each subchart's schema validates that subchart's values. `--skip-schema-validation` (3.16) disables it — flag it in pipelines.

The example chart's schema (draft-07 for compatibility with every Helm 3 and Helm 4 version):

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
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
    "env": {
      "type": "object",
      "additionalProperties": {"type": ["string", "number", "boolean"]}
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
$ helm template r web --set podDisruptionBudget.minAvailable=1
Error: values don't meet the specifications of the schema(s) in the following chart(s):
web:
- at '/podDisruptionBudget': 'not' failed
```

##### Draft support depends on the version

- Helm **3.18+ and Helm 4** use `santhosh-tekuri/jsonschema/v6`, which supports drafts 4, 6, 7, 2019-09 and 2020-12. **Helm ≤ 3.17 understands draft-07 and older only** and silently misinterprets 2020-12 keywords (`prefixItems` ignored, so `items: false` rejects every element; `unevaluatedProperties` not enforced).[^11]
- A chart consumed by unknown Helm versions should use **draft-07**. A `"$schema": "https://json-schema.org/draft/2020-12/schema"` chart requires Helm ≥ 3.18; say so in the README.
- Error message format changed in 3.18 (`at '/path': …` instead of `path: …`); tests that match messages must be updated.
- `$ref` to `http(s)://` URLs is fetched over the network during validation; vendor referenced schemas into the chart. Unresolvable URN `$ref`s are tolerated with a warning since 3.20 and 4.0.2.

Review points:

- `"additionalProperties": false` at the **root** rejects keys for subcharts and `global` unless they are listed. Use it on nested objects the chart owns (it catches typos such as `replicaCont`), not at the root.
- `"type": "integer"` accepts `3` and `3.0` from YAML and `--set replicaCount=3`, rejects `3.5`, `--set replicaCount=3.0` (a string) and any `--set-string` value.
- Allow both forms where Kubernetes does (`targetPort`: integer or named port).
- Validation sees merged values: schema `required` only matters for keys without defaults.
- Prefer the schema over template `fail` for type, enum and range checks: it reports all violations at once and is visible to tooling.

***

#### Template Language: Idioms and Pitfalls

Templates are Go `text/template` plus the Sprig function library (without `env` and `expandenv`) plus Helm's own functions (`include`, `tpl`, `required`, `lookup`, `toYaml`, `fromYaml`, `toJson`, `fromJson`, `toToml`, …).[^15] Examples in this section were rendered on 3.22.1, 4.0.5 and 4.3.1.

##### `include`, not `template`

`include` returns a string that can be piped; `template` writes directly to the output and cannot be piped. `{{ template "x" . | nindent 4 }}` pipes the **argument** into `nindent` and fails:

```console
Helm 3.22.1: Error: template: e/templates/a.yaml:7:40: executing "e/templates/a.yaml" at <4>: wrong type for value; expected string; got chartutil.Values
Helm 4.3.1:  Error: e/templates/a.yaml:7:40 (followed by the same details on further lines)
```

Always use `include` (`{{- include "web.labels" . | nindent 4 }}`).

##### Whitespace and indentation

YAML is whitespace-sensitive; a wrong indent produces either invalid YAML or, worse, valid YAML with keys under the wrong parent.

- `{{-` trims whitespace and newlines **before** the action, `-}}` **after**. Use `{{-` on lines that contain only an action so they leave no blank line.
- `nindent N` emits a newline and then indents every line by N; use it after `{{-` on a line of its own. `indent N` indents every line, including the first, without a leading newline; use it only when the action starts at column 0 of its own line.
- N is the column where the block's keys belong (parent key indentation + 2 in this guide's style). Verify by rendering.

```yaml
# GOOD
metadata:
  labels:
    {{- include "web.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}

# BAD: nindent on the same line as the key: "a" becomes null and the map's keys become siblings of "a"
data:
  a: {{ .Values.env | toYaml | nindent 2 }}
```

- Use `with` + `toYaml` for optional blocks so an empty value renders nothing:

```yaml
          {{- with .Values.resources }}
          resources:
            {{- toYaml . | nindent 12 }}
          {{- end }}
```

- Embed file contents in a `|` block scalar:

```yaml
data:
  feature-flags.json: |
    {{- .Values.config.featureFlags | toJson | nindent 4 }}
```

- Helm 4.2.4+ keeps blank lines that a template leaves at the end of a document; 3.22.1 and 4.0.5 strip them. The YAML is equivalent, but golden files comparing raw `helm template` output must be regenerated when the Helm version changes.

##### Scope: `with`, `range` and `$`

`with` and `range` rebind `.`. Use `$` for the root context inside them:

```yaml
# BAD: .Release is looked up on the map value
data:
  {{- range $k, $v := .Values.env }}
  {{ $k }}: {{ $v | quote }}
  ns: {{ .Release.Namespace }}
  {{- end }}
# Helm 3.22.1: ... at <.Release.Namespace>: can't evaluate field Release in type interface {}

# GOOD
  ns: {{ $.Release.Namespace }}
```

Helpers take the right context: `include "web.fullname" .` at top level, `include "web.fullname" $` inside `range`/`with`; helpers with parameters take a dict: `include "web.componentName" (dict "root" $ "component" "worker")`.

##### `default` and `with` treat `false`, `0` and `""` as empty

```yaml
# values.yaml: enabled: false, replicas: 0
badDefaultBool: {{ .Values.enabled | default true }}    # true  <- the user's false is ignored
badDefaultInt:  {{ .Values.replicas | default 3 }}      # 3     <- the user's 0 is ignored

# GOOD: distinguish "unset" from "false/0"
enabled: {{ hasKey .Values "enabled" | ternary .Values.enabled true }}
```

```yaml
# BAD: maxUnavailable: 0 is valid, but "with" skips it
  {{- with .Values.podDisruptionBudget.maxUnavailable }}
  maxUnavailable: {{ . }}
  {{- end }}

# GOOD
  {{- if hasKey .Values.podDisruptionBudget "maxUnavailable" }}
  maxUnavailable: {{ .Values.podDisruptionBudget.maxUnavailable }}
  {{- end }}
```

Better still: put defaults in `values.yaml` and use `default` only for strings (`.Values.image.tag | default .Chart.AppVersion`).

##### `required` and `fail`

```yaml
host: {{ required "ingress.host is required when ingress.enabled=true" .Values.ingress.host }}
```

`required` fails on `nil` **and** on the empty string (`Error: execution error at (e/templates/a.yaml:13:11): host is required`, identical on both majors), not on `false` or `0`. For cross-field rules use `fail`:

```yaml
{{- if and (hasKey .Values.podDisruptionBudget "minAvailable") (hasKey .Values.podDisruptionBudget "maxUnavailable") }}
{{- fail "set either podDisruptionBudget.minAvailable or maxUnavailable, not both" }}
{{- end }}
```

##### `quote`, `toString` and types

- Quote every free-form string: `{{ .Values.someString | quote }}`. Unquoted `yes`, `on`, `no`, `1e3`, `0777` change type when the manifest is parsed.
- Environment variable values, annotation values and label values must be strings. `value: {{ .Values.port }}` renders `value: 8080` and the install fails (Helm 3: `json: cannot unmarshal number into Go struct field EnvVar…value of type string`; Helm 4: `.env[name="PORT"].value: expected string`). Write `{{ .Values.port | quote }}` (or `| toString | quote` for values that may be numbers or booleans).
- Do **not** quote integers the API expects as numbers (`containerPort`, `replicas`, `port`): `containerPort: "8080"` fails on install. `helm lint`, `helm template` and `--dry-run=server` catch neither error.
- `toYaml`, `toJson` and `toToml` return an empty string on marshal errors. Sprig's `mustToJson` (available on every Helm version) fails instead; `mustToYaml` needs Helm 4.0 and `mustToToml` Helm 4.2.
- `.Values` is a `chartutil.Values` (Helm 3) / `common.Values` (Helm 4), not a plain map; Sprig's `dig` fails on it (`error calling dig: interface conversion: …`). Use `.Values.AsMap`:

```yaml
timeout: {{ dig "server" "timeout" "30s" .Values.AsMap | quote }}
```

- Numbers from YAML values are `float64`; `int`/`int64` casts lose precision above 2^53.

##### `tpl`

`tpl` renders a string as a template, so users can put expressions into values:

```yaml
# values.yaml
tplValue: "{{ .Release.Name }}-svc"
# template
service: {{ tpl .Values.tplValue . | quote }}     # "r-svc"
```

Use it only for values documented as templatable (hostnames, affinity blocks: `{{- tpl (toYaml .) $ | nindent 8 }}`). `tpl` can call `lookup` (cluster reads during install), `include` any helper and `fail`: never feed it input from untrusted sources, and remember that values become code.

##### `lookup`

```yaml
{{- $existing := lookup "v1" "Secret" .Release.Namespace "lk-secret" }}
```

- Available since 3.1. Returns an empty map when the object does not exist **or** when there is no cluster: `helm template`, `helm lint`, `--dry-run=client`. It returns real data only on install/upgrade and `--dry-run=server` (3.13).
- An empty name returns a list of **all** objects of that kind in the namespace (needs list RBAC; can be every Secret).
- GitOps tools that render with `helm template` semantics always take the "not found" branch. A chart must render a valid, **stable** manifest when `lookup` is empty.

##### Random and time functions

`randAlphaNum`, `uuidv4`, `now` and `date` produce a different value on every render. Used directly in a Secret, ConfigMap or pod annotation, they change the object on every upgrade (rotating a password, restarting Pods). Use them only together with a `lookup` that preserves an existing value (see "Configuration and Secrets").

##### Network access during render

Since 3.11.1, `getHostByName` returns `""` unless `--enable-dns` is passed (verified on 3.22.1, 4.0.5 and 4.3.1: `""` without the flag, `"127.0.0.1"` for `localhost` with it). A chart that needs DNS at render time is not reproducible; flag `--enable-dns` in pipelines.

##### Helm 4-only functions

| Function | Since | Note |
|---|---|---|
| `mustToYaml` | 4.0 | Fails on marshal errors instead of returning `""` |
| `mustToToml` | 4.2 | Failing variant of `toToml` |
| `mustToDuration`, `durationSeconds`, `durationMinutes`, `durationRoundTo`, … | 4.3 | `duration*` helpers return `0` on invalid input — validate first |

Recommend them only for charts whose consumers all run that Helm version or newer.

***

#### `_helpers.tpl`, Names and Labels

Define reusable logic in `_helpers.tpl`; do not repeat the same expression across templates. The helper set below (based on the `helm create` scaffold, identical in 3.22.1 and 4.3.1, plus `web.image` and `web.secretName`) was linted and rendered on all three versions:

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

- **Apply the recommended labels on every object via helpers**: `app.kubernetes.io/name`, `instance`, `version`, `component`, `part-of`, `managed-by`.[^16] Never hard-code label values in individual templates. `{{ .Release.Service }}` renders `Helm`.
- **Selector labels contain only `name` and `instance`** (plus `component` for charts with several workloads). They are immutable (next section).
- The `helm create` scaffold puts the full label set (including `helm.sh/chart` and `app.kubernetes.io/version`) on the **pod template**, so every chart or app version bump restarts all Pods even without an image change. The example Deployment in this guide uses `selectorLabels` plus `podLabels` on the pod template instead; either is acceptable if chosen deliberately.
- The 63-character limit applies to names **and** label values; `trunc 63 | trimSuffix "-"` handles both. `helm.sh/chart` replaces `+` (invalid in label values) with `_`.
- Keep helpers free of side effects (no `fail` inside a name helper that every template calls, unless intended).

***

#### Resource Correctness

##### Selectors are immutable — and upgrades are not transactional

`spec.selector` of Deployments, StatefulSets, DaemonSets and Jobs cannot change. Adding a value-driven or version label to `selectorLabels` breaks the next upgrade. Verified on 3.22.1 and 4.3.1 by adding `app.kubernetes.io/version` to the selector:

```console
Error: UPGRADE FAILED: cannot patch "r-web" with kind Deployment: Deployment.apps "r-web" is invalid:
spec.selector: Invalid value: {...,"app.kubernetes.io/version":"2.7.1"}: field is immutable
$ kubectl get svc r-web -o jsonpath='{.spec.selector}'
{"app.kubernetes.io/instance":"r","app.kubernetes.io/name":"web","app.kubernetes.io/version":"2.7.1"}
```

On **both** majors the Service had already been switched to the new selector before the Deployment failed: it now selects no Pods while the release shows `failed`. Treat any change to selector labels, `fullname` logic or `nameOverride` defaults as a breaking change (it needs a new release or a delete-and-recreate plan); use `--atomic`/`--rollback-on-failure` so a failed upgrade is rolled back.

Other immutable fields that commonly break upgrades: StatefulSet `volumeClaimTemplates` and `serviceName`, Job `spec.template`, Service `clusterIP`, PVC `storageClassName` and decreasing size, and `metadata.name` (a renamed object is a delete plus a create).

##### Restart Pods when configuration changes

Containers read ConfigMaps and Secrets consumed through `env`/`envFrom` only at start. Add a checksum of the rendered configuration to the **pod template** so changes roll the Deployment:

```yaml
  template:
    metadata:
      annotations:
        checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
```

Verified: changing `config.logLevel` changes the annotation. Do not checksum a Secret that is regenerated randomly on every render (it would roll Pods on every upgrade), and do not put `now`/`randAlphaNum` into pod annotations to "force" restarts.

##### Do not render fields owned by other controllers

```yaml
# BAD: every upgrade resets (Helm 3) or conflicts with (Helm 4) the HPA's replica count
spec:
  replicas: {{ .Values.replicaCount }}

# GOOD
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
```

The same applies to `caBundle` fields filled by cert-manager's cainjector, injected sidecars, `Service.spec.clusterIP`, and annotations written by tools. Under Helm 4 even an unchanged chart value conflicts when it differs from the live value another manager set (verified above).

##### Duplicate objects and keys

- Two templates rendering the same kind/namespace/name: Helm 3 fails the install after creating the first copy (`already exists`); Helm 4 applies both to one object without error. `helm lint` detects neither. Check for two templates using `include "x.fullname"` without a suffix and for subchart `fullnameOverride` collisions.
- Duplicate keys in keyed lists (`env` names, `ports` by `containerPort`, `volumeMounts` by `mountPath`) produce a warning and wrong upgrades on Helm 3 and a hard failure on Helm 4. Charts that append user `extraEnv` to built-in variables should model `env` as a map, skip reserved names, or document them:

```yaml
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ include "web.secretName" . }}
                  key: password
            {{- range $name, $value := .Values.env }}
            {{- if ne $name "DB_PASSWORD" }}
            - name: {{ $name | quote }}
              value: {{ $value | toString | quote }}
            {{- end }}
            {{- end }}
```

##### `generateName` is unsupported

Helm tracks objects by name; `metadata.generateName` without `metadata.name` fails on install (`resource name may not be empty`) on both majors. Flag it.

##### API versions

Use current API versions (`apps/v1`, `policy/v1` PodDisruptionBudget, `autoscaling/v2` HPA, `networking.k8s.io/v1` Ingress, `batch/v1` CronJob). Removed versions (`extensions/v1beta1`, `policy/v1beta1` PDB, `autoscaling/v2beta2`) fail on current clusters. Gate optional APIs on `.Capabilities.APIVersions.Has` (see "Dry Runs").

***

#### Workloads: Security, Resources, Probes, Availability

The example Deployment (rendered on all three versions; the full chart output, including the hook Job and the test Pod, was accepted by `kubectl apply --dry-run=server` in a namespace labelled `pod-security.kubernetes.io/enforce=restricted`):

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
        {{- include "web.selectorLabels" . | nindent 8 }}
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
            {{- range $name, $value := .Values.env }}
            {{- if ne $name "DB_PASSWORD" }}
            - name: {{ $name | quote }}
              value: {{ $value | toString | quote }}
            {{- end }}
            {{- end }}
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

The matching PodDisruptionBudget honours `0`:

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

- Default to the Pod Security Standards **restricted** profile:[^17] `runAsNonRoot: true`, a numeric non-zero `runAsUser`, `allowPrivilegeEscalation: false`, `capabilities.drop: ["ALL"]`, `seccompProfile.type: RuntimeDefault`; add `readOnlyRootFilesystem: true` with an `emptyDir` for `/tmp`.
- Apply the same settings to hook Jobs and test Pods: the `helm create` scaffold's test Pod has no security context and is rejected in `restricted` namespaces.
- Flag `privileged: true`, `hostNetwork`, `hostPID`, `hostIPC`, `hostPath` volumes, added capabilities (`NET_ADMIN`, `SYS_ADMIN`) and `runAsUser: 0` unless the chart's purpose requires them (CNI, node agents) and they sit behind explicit values.
- `automountServiceAccountToken: false` unless the app calls the Kubernetes API; then ship a least-privilege `Role`/`RoleBinding` behind `rbac.create`, never a `ClusterRole` with `*` verbs.

##### Resources

- Always set requests (CPU and memory) and a memory limit; a CPU limit is a policy decision (throttling hurts latency-sensitive services).
- `resources: {}` as default (the scaffold) is acceptable only for example charts.

##### Probes

- Readiness gates traffic, liveness restarts, startup protects slow starts. A liveness probe that depends on downstream services causes cascading restarts.
- Use the named port (`port: http`), and `startupProbe` instead of a long `initialDelaySeconds`.
- `--wait` relies on readiness: without a readiness probe a Pod is Ready as soon as its containers start, so a broken release looks healthy.

##### Availability

- PodDisruptionBudget for workloads with more than one replica, with `minAvailable` **or** `maxUnavailable` (the API rejects both). `minAvailable` equal to the replica count blocks node drains.
- HPA with `autoscaling/v2`; omit `spec.replicas` when enabled.
- `topologySpreadConstraints` or anti-affinity for zone/node spreading; `revisionHistoryLimit` to bound old ReplicaSets.
- `strategy.rollingUpdate.maxUnavailable: 0` with `maxSurge: 1` for zero-downtime services.
- `terminationGracePeriodSeconds` longer than the app's shutdown time; a `preStop` sleep when load balancers need time to deregister.

##### Networking

- Gate `Ingress`, Gateway API `HTTPRoute` and `NetworkPolicy` behind values. Use `spec.ingressClassName`, not the deprecated `kubernetes.io/ingress.class` annotation.
- A NetworkPolicy must allow the traffic the chart needs (metrics scrapers, ingress controllers); kubelet probes are not subject to NetworkPolicy.

***

#### Configuration and Secrets

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

Limitations reviewers must know:

- Under `helm template` (Argo CD, Flux configurations that render with template semantics) `lookup` is empty, so the password changes on every sync. For GitOps, require `existingSecret` backed by External Secrets Operator, Sealed Secrets or SOPS.
- `helm.sh/resource-policy: keep` leaves the Secret behind on uninstall, with its ownership metadata. Reinstalling the **same release name in the same namespace** adopts it; another release name fails with `invalid ownership metadata` unless `--take-ownership` (3.17) is used.

##### Secrets handling rules

- **Never put secrets in `values.yaml` defaults, `NOTES.txt` output, ConfigMaps, annotations or labels.**
- **Release Secrets contain everything.** Helm stores each revision as a Secret `sh.helm.release.v1.<release>.v<revision>` in the release namespace (`HELM_DRIVER=secret`, the default), holding the chart, all user-supplied values and the rendered manifest (gzip + base64). Anyone who can `get secrets` in that namespace can read every value passed with `--set` or `-f`. Prefer references to externally managed Secrets; restrict Secret read access in release namespaces.
- `HELM_DRIVER=configmap` stores the same data in ConfigMaps — readable by far more identities. Flag it.
- `--set password=…` on a command line also leaks into shell history and CI logs; use `-f` with a file from a secret store, or `--set-file`.
- `--hide-secret` (3.15) hides Secret contents in `--dry-run` output; use it in CI that prints dry runs.
- `helm get values`, `helm get manifest` and `helm history` expose the same data to whoever can run them.

##### Ownership and adoption

Install refuses to touch an existing object that lacks Helm's ownership metadata (`exists and cannot be imported into the current release: invalid ownership metadata; label validation error: missing key "app.kubernetes.io/managed-by": must be set to "Helm"; …`). Adopt deliberately by adding `app.kubernetes.io/managed-by=Helm`, `meta.helm.sh/release-name` and `meta.helm.sh/release-namespace`, or with `--take-ownership` (3.17). `--take-ownership` steals objects from other releases; on Helm 3 and Helm 4 before 4.3, uninstalling the original release then deletes the stolen object. Flag it as a permanent CI flag.

##### ConfigMaps and files

- `| quote` for scalar values, `|` block scalars with `nindent` for files.
- ConfigMaps and Secrets are limited to 1 MiB; the release Secret is bounded by the same limit, so very large embedded files make a chart uninstallable.
- `.Files.Get "files/app.conf"` and `(.Files.Glob "files/*.conf").AsConfig` read packaged files; files listed in `.helmignore` and files under `templates/` are not available through `.Files`.

***

#### Hooks and Deletion Policies

Use hooks for work that must run at a specific point of the release lifecycle (database migrations, one-off setup). Example migration hook (rendered on all three versions):

```yaml
{{- if .Values.migrations.enabled }}
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ include "web.fullname" . | trunc 55 | trimSuffix "-" }}-migrate
  labels:
    {{- include "web.labels" . | nindent 4 }}
    app.kubernetes.io/component: migrate
  annotations:
    # post-install, not pre-install: on the first install the Secret and ServiceAccount
    # are regular chart resources and do not exist yet in the pre-install phase
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

Hook events: `pre-install`, `post-install`, `pre-upgrade`, `post-upgrade`, `pre-rollback`, `post-rollback`, `pre-delete`, `post-delete`, `test`.[^18]

##### Deletion policies

| `helm.sh/hook-delete-policy` | Effect |
|---|---|
| `before-hook-creation` | Delete the previous instance before creating the new one (**default when the annotation is absent**) |
| `hook-succeeded` | Delete after the hook succeeded |
| `hook-failed` | Delete if the hook failed |

- **Hook resources are not part of the release.** They are not upgraded in place, not deleted on uninstall, and not listed by `helm get manifest` (`helm get hooks` shows them). Verified on 3.22.1 and 4.3.1: a `pre-install` ConfigMap hook without a delete policy remained after `helm uninstall`. Always set a delete policy; use `ttlSecondsAfterFinished` on Jobs as well.
- `hook-succeeded` removes the evidence of success. To keep logs, use `"helm.sh/hook-output-log-policy": hook-failed` (or `hook-succeeded`), which prints the hook's container logs — **3.18+ and Helm 4 only**.
- When setting the policy explicitly, include `before-hook-creation` for any hook that runs more than once (`pre-upgrade`). Verified with a `pre-install,pre-upgrade` ConfigMap hook annotated only `hook-failed`: on Helm 3.22.1 the first upgrade fails with `pre-upgrade hooks failed: … configmaps "hk-hook" already exists`; Helm 4.0.5/4.3.1 apply over the leftover object and succeed. Charts must work on both.

##### Ordering and waiting

- Hooks of one event run by ascending `helm.sh/hook-weight` (a **string**: `"-5"`), then by name (the documentation says weight, kind, name; the Helm 3.22.1 source sorts equal weights by name only). Use distinct weights when one hook depends on another.
- **A `pre-install` hook runs before any regular resource exists.** A migration Job that mounts a Secret or uses a ServiceAccount from `templates/` fails on the first install. Make those dependencies hooks with lower weights, or run the migration as `post-install,pre-upgrade` as above.
- **Helm waits for hook Jobs and Pods** to complete, with or without `--wait`, up to `--timeout` (default 5m). A hook that never completes fails the release (`timed out waiting for the condition`). Set `backoffLimit` and `activeDeadlineSeconds`. Other hook kinds are not waited for (since 4.3.1 the default hook-only wait also waits for CRDs to become `Established`).
- `--no-hooks` skips hooks; flag it in CI unless intended.

##### Semantics reviewers get wrong

- A failed `pre-upgrade` hook aborts the upgrade. Rolling back (`--atomic`/`--rollback-on-failure` or `helm rollback`) runs `pre-rollback`/`post-rollback` hooks; **it does not reverse a migration**. Migrations must be backwards-compatible with the previous application version and idempotent.
- Post-renderers modify hooks only on Helm 4; on Helm 3 hook manifests bypass the post-renderer.
- CRDs must not be hooks; use `crds/` or a separate chart.
- Helm 4 applies hooks with server-side apply like regular resources.

***

#### Chart Tests (`helm test`)

Every chart should ship at least one test that checks behaviour, not only TCP reachability:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: {{ include "web.fullname" . | trunc 47 | trimSuffix "-" }}-test-connection
  labels:
    {{- include "web.labels" . | nindent 4 }}
  annotations:
    "helm.sh/hook": test
    "helm.sh/hook-delete-policy": before-hook-creation
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
- **`--logs` with `hook-succeeded` breaks on Helm 3**: the passing test Pod is deleted before Helm fetches its logs, and `helm test` exits 1 with `unable to get pod logs … not found`. Helm 4.0.5+ collects logs before deletion. For charts tested by unknown Helm versions, use `before-hook-creation` alone on test Pods (as above).
- Test Pods must pass the namespace's Pod Security level (the example is admitted under `restricted`).
- Pin test images (`busybox:1.37`, better a digest).

***

#### CRDs

- Files in `crds/` are plain YAML (**not templated**), installed before the templates on the **first install only**, **never upgraded** and **never deleted** by Helm (both majors).[^19] `--skip-crds` skips them; `helm template --include-crds` renders them; `helm show crds` (3.7) prints them.
- CRDs that must evolve with the chart belong in a separate chart (templated, upgraded deliberately) or are managed by the operator itself. Deleting a CRD deletes every custom resource of that kind cluster-wide: never template CRDs in a chart that users uninstall casually, and set `helm.sh/resource-policy: keep` on templated CRDs.
- Custom resources whose CRD is not yet known to the cluster fail the install before anything is created (`no matches for kind "…" in version "…"` / `ensure CRDs are installed first`). Gate them with `.Capabilities.APIVersions.Has` when the CRD comes from another release.
- Under Helm 4 `--wait` (watcher) also waits for custom resources' `Ready` conditions; make sure the operator that sets them is installed first, or choose the wait strategy deliberately.

***

#### Dependencies, Subcharts and Library Charts

- Parent values for a subchart live under its name or alias (`postgresql.auth.database`); `global` is visible to all; `import-values` copies child values up.
- `condition: <sub>.enabled` and `tags:` toggle subcharts; `alias:` installs the same chart twice under different names.
- Subchart templates render with their own `.Chart` and `.Values`, but **defined template names are global**: a subchart's `include "fullname"` can resolve to the parent's definition. Prefix every `define`.
- `.Chart.IsRoot` (3.7) is `false` inside subcharts.
- Directories vendored into `charts/` by hand bypass `Chart.lock` verification; prefer packaged dependencies built from the lock.

**Library charts** (`type: library`) contain only `define`s and cannot be installed (`library charts are not installable`). Their templates run with the caller's context:

```yaml
# common/templates/_labels.tpl  (chart "common", type: library)
{{- define "common.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

# parent/templates/cm.yaml
metadata:
  labels:
    {{- include "common.labels" . | nindent 4 }}   # .Chart.Name is the *parent's* name here
```

***

#### Install, Upgrade and Rollback Semantics

##### How changes are applied

| | Helm 3 (all releases) / Helm 4 `csa` releases | Helm 4 `ssa` releases (default for new installs) |
|---|---|---|
| Mechanism | Three-way strategic merge patch: previous release manifest × live object × new manifest | Server-side apply, field manager `helm` |
| Live drift on rendered fields | Reset to the chart value silently | Conflict error if another manager owns the field |
| Fields the chart never rendered (injected sidecars, extra annotations) | Kept | Kept |
| Field removed from the chart | Deleted from the live object | Deleted (Helm no longer applies it) |
| Custom resources | Two-way JSON merge patch (old vs new manifest; live drift **not** repaired, lists replaced) | Server-side apply |

Helm 4 records the method per release (`helm get metadata` → `APPLY_METHOD`); `helm upgrade` defaults to `--server-side=auto` (keep the previous method). Releases created by Helm 3 stay on client-side apply after a Helm 4 upgrade until `--server-side=true` is passed.[^20]

##### Replacing and forcing

- `--force` (Helm 3) / `--force-replace` (Helm 4) **replaces** each object with a full PUT instead of patching: every field the chart does not render is reset (controller- and webhook-written values), and immutable-field changes still fail. A repair tool, not a deploy default.
- `--force-conflicts` (Helm 4) keeps server-side apply semantics and takes field ownership from other managers. Right for a one-off migration, wrong as a permanent CI flag (it silently reverts HPAs, operators and webhooks on every deploy).
- Neither flag fixes an immutable-selector change.

##### Rolling back on failure

```bash
# Helm 3
helm upgrade --install web ./chart -n web --create-namespace \
  -f values/prod.yaml --atomic --timeout 10m

# Helm 4
helm upgrade --install web ./chart -n web --create-namespace \
  -f values/prod.yaml --rollback-on-failure --timeout 10m
```

- `--atomic` (Helm 3) and `--rollback-on-failure` (Helm 4) imply waiting (Helm 4: `--wait=watcher` unless another strategy is given). On install failure the release is uninstalled; on upgrade failure Helm rolls back to the last successful revision.
- The rollback also waits with the same `--timeout`; if the previous revision cannot become ready either, the release ends `failed` at a new revision.
- Do not rename `--atomic` to `--rollback-on-failure` in a Helm 3 project (unknown flag), and do not use `helm install --atomic` with Helm 4.0.x–4.1.1 (unknown flag). `helm upgrade --atomic` works on every Helm 4 version as a deprecated alias.
- `--cleanup-on-fail` deletes objects **created** by a failed upgrade; patched objects are reverted only by the rollback.
- Upgrades apply objects one by one and are **not transactional** on either major (see "Resource Correctness"): without automatic rollback, a failed upgrade leaves a mix of old and new objects.

##### Waiting

| | Helm 3 `--wait` | Helm 4 `--wait` / `--wait=watcher` |
|---|---|---|
| Deployments, StatefulSets, DaemonSets | Minimum available Pods of the new revision | kstatus: rolled out and available |
| Pods, PVCs, Services | Ready / `Bound` / ClusterIP (LoadBalancer: ingress assigned) | Same, via kstatus |
| Jobs | Ready immediately; with `--wait-for-jobs` (3.5): completed | Started; with `--wait-for-jobs`: completed (failed Job = error) |
| Custom resources | Ignored | `Ready`/`Reconciling`/`Stalled` conditions and `observedGeneration` |
| Without `--wait` | Only hooks are awaited | Only hooks are awaited (`hookOnly`) |

- `--wait-for-jobs` has no effect without `--wait` on Helm 3.
- Always set `--timeout` explicitly (default `5m0s`); give the CI step a longer timeout than Helm's so Helm can roll back instead of being killed.
- Helm 3 failures only say `context deadline exceeded` (add `--debug` to see which resource); Helm 4 names the resource and its kstatus message.

##### Rollback, history and stuck releases

- `helm rollback <release> <revision>` re-applies an older revision's manifest as a **new** revision; it does not restore data, reverse hook side effects or CRDs.
- A CI job killed hard (SIGKILL) during install/upgrade leaves the release in `pending-install`/`pending-upgrade`; further operations fail with `another operation (install/upgrade/rollback) is in progress`. Recover with `helm rollback` (or `helm uninstall` for a stuck first install); never delete `sh.helm.release.v1.*` Secrets by hand.
- `helm list` hides pending releases on Helm 3 (`--pending` or `-a` shows them); Helm 4 lists every status by default and removed `-a`.
- `--history-max` (default 10) bounds stored revisions; `0` means unlimited and lets release Secrets accumulate.

***

#### Dry Runs, `helm template`, `lookup` and `.Capabilities`

| Command | Cluster contact | `lookup` | What is checked |
|---|---|---|---|
| `helm template` | none | empty | rendering, schema, `kubeVersion` against the offline default |
| `helm template --validate` (Helm 3; deprecated in Helm 4) | yes | empty | kinds known to the cluster |
| `helm install/upgrade --dry-run` / `--dry-run=client` (3.13) | yes (release history) | empty | rendering |
| `helm install/upgrade --dry-run=server` (3.13), `helm template --dry-run=server` | yes | **real data** | rendering, kinds known to the cluster, ownership of existing objects |

- **No dry-run mode validates object fields.** A misspelled field or an integer env value passes `--dry-run=server` and fails (Helm 4) or is silently dropped (Helm 3) on the real install. Validate rendered output with `kubectl apply --dry-run=server -f -` against a real cluster, or offline with a schema validator such as kubeconform and the target cluster's schemas.
- Bare `--dry-run` is deprecated in Helm 4 (use `--dry-run=client`).

##### `.Capabilities` offline vs. online

- Offline, `.Capabilities.KubeVersion` is the client-go version Helm was built with: `v1.37.0` on 3.22.1 and 4.3.1, `v1.34.0` on 4.0.5 (verified). Version gates therefore change when CI upgrades Helm. **Pass `--kube-version` explicitly in CI** (3.6).
- Offline `.Capabilities.APIVersions` contains only `group/version` entries: `Has "policy/v1"` is `true`, `Has "apps/v1/Deployment"` is `false` (verified on all three versions); against a cluster both forms exist. Gate on `group/version`, and pass CRD-provided APIs explicitly:

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

- Combine capability checks with an explicit value so GitOps renders stay deterministic.

***

#### Post-Renderers and Plugins

##### Post-renderers

A post-renderer receives all rendered manifests on stdin and writes the result to stdout (typically Kustomize).

| | Helm 3 (3.1+) | Helm 4 |
|---|---|---|
| `--post-renderer` value | path to an executable | name of an installed `postrenderer/v1` plugin (a path fails: `plugin: {Name:./script.sh Type:postrenderer/v1} not found`) |
| Arguments | `--post-renderer-args` (3.9) | `--post-renderer-args` |
| Hooks and test Pods | **not** post-rendered | post-rendered |
| Empty output | accepted: install deploys nothing, **upgrade deletes every release object**, both report `deployed` | error: `post-renderer "…" produced empty output` |

Helm 4 post-renderer plugin with a script that also works as a Helm 3 executable post-renderer (verified: on 3.22.1 the Kustomize `commonAnnotations` reach every object except the hook Job and the test Pod; on 4.0.5 and 4.3.1 they reach all of them):

```yaml
# plugins/kustomize-env/plugin.yaml
apiVersion: v1
type: postrenderer/v1
name: kustomize-env
version: 0.1.0
runtime: subprocess
runtimeConfig:
  platformCommand:
    - command: "${HELM_PLUGIN_DIR}/render.sh"
```

```yaml
# plugins/kustomize-env/kustomization.yaml
resources:
  - all.yaml
commonAnnotations:
  team: payments
```

```sh
#!/bin/sh
# plugins/kustomize-env/render.sh
# Works as a Helm 3 executable post-renderer and as the Helm 4 plugin command.
set -eu
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
cat > "$dir/all.yaml"
[ -s "$dir/all.yaml" ] || { echo "post-renderer: empty input" >&2; exit 1; }
cp "${HELM_PLUGIN_DIR:-.}/kustomization.yaml" "$dir/"
kubectl kustomize "$dir" > "$dir/out.yaml"
[ -s "$dir/out.yaml" ] || { echo "post-renderer: empty output" >&2; exit 1; }
cat "$dir/out.yaml"
```

Review points:

- Install the plugin with `helm plugin install ./plugins/kustomize-env` (Helm 4) or pass `--post-renderer ./plugins/kustomize-env/render.sh` (Helm 3, run from that directory so `kustomization.yaml` is found).
- On Helm 3, a post-renderer **must fail on empty output** (`set -eu` plus a non-empty check); otherwise a broken pipeline step deletes the whole release on upgrade.
- Overlays that patch "all Deployments" or add sidecars also hit hook Jobs and test Pods on Helm 4; keep selectors precise.
- Post-renderers see resolved names: renaming objects breaks references (`secretKeyRef`, `serviceName`) that the templates already rendered.
- Only one `--post-renderer` per command; commit the script/plugin and pin the tools it calls.

##### Plugins

- Plugins run with the user's credentials. Install only reviewed plugins from pinned versions (`helm plugin install <url> --version <tag>`).
- Helm 4 plugin installs: tarballs need a `.prov` signature (enforced since 4.1.4; 4.0.x–4.1.3 install unsigned archives with only a warning, CVE-2026-35205); **VCS URLs fail without `--verify=false`** (`plugin source does not support verification`, verified on 4.0.5 and 4.3.1); local directories install without a signature. A `--verify=false` in CI deserves a comment explaining the trust decision.
- Helm 4 rejects plugin versions that are not SemVer since 4.1.4 (CVE-2026-35204, path traversal via `version`).
- `HELM_NO_PLUGINS=1` disables plugins for reproducible renders.

***

#### OCI Registries, Packaging and Provenance

##### Publishing

```bash
set -euo pipefail
helm dependency build ./charts/web
helm lint --strict ./charts/web -f ci/values/prod.yaml
helm package ./charts/web -d dist/
echo "$REGISTRY_TOKEN" | helm registry login ghcr.io -u "$REGISTRY_USER" --password-stdin
helm push dist/web-1.4.0.tgz oci://ghcr.io/acme/charts      # prints "Digest: sha256:..."
```

- OCI support is GA since **3.8**; `helm push`/`helm pull oci://` since 3.7. Classic `index.yaml` repositories still work.
- `helm push` needs a packaged `.tgz`; the tag is the chart `version` (`+` becomes `_`).
- `helm registry login` with `--password-stdin`: `-p/--password` leaks the token into the process list and shell history. Use the **host only** (Helm 4 rejects `host/path`).
- Credentials are stored in `~/.config/helm/registry/config.json` (or `$HELM_REGISTRY_CONFIG`) as base64, effectively plaintext; CI runners must not persist the file. Prefer short-lived tokens (workload identity, `GITHUB_TOKEN`).
- `--plain-http` (3.13) and `--insecure-skip-tls-verify` belong only in local test setups.
- `--pass-credentials` sends repository credentials to other hosts listed in an `index.yaml` (default off since 3.6.1, CVE-2021-32690); flag it.

##### Deploy by digest

```bash
# GOOD: immutable deploy input (Helm >= 3.17; tag@digest needs Helm 4.1.3+ on Helm 4)
helm upgrade --install web oci://ghcr.io/acme/charts/web@sha256:<digest printed by helm push> -f values/prod.yaml

# BAD: mutable tag — the registry can serve different content for the same version
helm upgrade --install web oci://ghcr.io/acme/charts/web --version 1.4.0
```

A wrong digest fails (`chart reference digest mismatch` for `tag@digest`, `not found` for digest-only). Record the digest from `helm push` in the release output and the GitOps repository.

##### Reproducibility

- Helm 3 stamps every archive entry with the current time: packaging unchanged sources twice yields different digests, and `SOURCE_DATE_EPOCH` is ignored. Build and push each version **once** and deploy the recorded digest.
- Helm 4 archives are deterministic (entries carry file modification times, so a fresh checkout still changes the digest); Helm 4.3 honours `SOURCE_DATE_EPOCH` (`export SOURCE_DATE_EPOCH="$(git log -1 --format=%ct)"`).
- Never republish an existing version with different content.

##### Provenance

```bash
gpg --export-secret-keys > secring.gpg      # Helm signs with Go's OpenPGP: export legacy keyring files
gpg --export > pubring.gpg
helm package ./web --sign --key 'Chart Signer' --keyring secring.gpg -d signed/
helm verify signed/web-1.4.0.tgz --keyring pubring.gpg
helm push signed/web-1.4.0.tgz oci://registry.example.com/charts    # uploads the .prov alongside
helm install web oci://registry.example.com/charts/web --version 1.4.0 --verify --keyring pubring.gpg
```

- `pubring.kbx` (GnuPG 2 keybox) works only on Helm 4.3+; older versions fail with `openpgp: invalid data`. Export `pubring.gpg` for portability.
- Sigstore/cosign signatures on the OCI artifact are a common alternative. Either way, **verification must happen in the deploy path** (`--verify`, `cosign verify`), not only at publish time.

***

#### Testing and CI: `helm lint`, `helm template`, `helm-unittest`, `ct`

##### Layers

| Tool | Catches | Misses |
|---|---|---|
| `helm lint --strict <path>` | YAML/template errors, schema violations, `Chart.yaml` type errors; on Helm 4 unknown `Chart.yaml` keys and non-SemVer versions | wrong field names in manifests, wrong value types for the API, duplicate objects |
| `helm template` with production values and `--kube-version`/`--api-versions` | rendering errors for real inputs | API validation |
| Schema validation of rendered output (kubeconform with the target cluster's schemas, or `kubectl apply --dry-run=server`) | unknown fields, wrong types, removed API versions, admission (Pod Security) with `--dry-run=server` | runtime behaviour |
| `helm-unittest` | template logic: conditions, defaults, name truncation, schema failures | cluster behaviour |
| `ct` (chart-testing) | version bump, maintainers, yamllint/yamale on `Chart.yaml` and values; `ct install` installs and runs `helm test` in a real cluster | — |
| `helm test` in a test cluster | the deployed service actually works | — |

##### `helm lint`

- Helm 4 requires the path (`"helm lint" requires at least 1 argument`); write `helm lint --strict ./charts/web` in scripts shared by both majors.
- Lint with every production values file (`-f ci/values/prod.yaml`), not only the defaults.
- `--strict` turns warnings into failures; on Helm 4 that includes unknown `Chart.yaml` keys.

##### `helm-unittest`

Unit tests in YAML under `tests/` (verified with helm-unittest 1.2.1 on Helm 3.22.1 and 4.3.1):[^21]

```yaml
# tests/deployment_test.yaml
suite: deployment
templates:
  - templates/deployment.yaml
  - templates/configmap.yaml
tests:
  - it: omits replicas when autoscaling is enabled
    template: templates/deployment.yaml
    set:
      autoscaling.enabled: true
    asserts:
      - notExists:
          path: spec.replicas
  - it: keeps selector labels stable
    template: templates/deployment.yaml
    release:
      name: prod
    asserts:
      - equal:
          path: spec.selector.matchLabels
          value:
            app.kubernetes.io/name: web
            app.kubernetes.io/instance: prod
  - it: prefers the digest over the tag
    template: templates/deployment.yaml
    set:
      image.digest: sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
    asserts:
      - matchRegex:
          path: spec.template.spec.containers[0].image
          pattern: "@sha256:[a-f0-9]{64}$"
  - it: rejects an unknown log level
    template: templates/deployment.yaml
    set:
      config.logLevel: verbose
    asserts:
      - failedTemplate: {}
```

```console
$ helm unittest web
 PASS  deployment	web/tests/deployment_test.yaml
Tests:       4 passed, 4 total
```

- The deployment template includes the ConfigMap for its checksum, so `configmap.yaml` must be listed under `templates:`.
- Add `/tests/` (anchored) to `.helmignore` — see "Chart Layout".
- Installing the plugin from its Git URL on Helm 4 needs `--verify=false`; pin the version with `--version`.

##### `ct` (chart-testing)

Sourced from the chart-testing documentation, not executed here:[^22]

- `ct lint` detects charts changed against `--target-branch` (default `main`) and by default checks the version increment (`--check-version-increment`), validates maintainers (`--validate-maintainers`: `name` must be a GitHub/GitLab/Bitbucket account, so team names fail unless the check is disabled), validates `Chart.yaml` with Yamale and lints `Chart.yaml` and values files with yamllint, then runs `helm lint`.
- `ct install` installs each changed chart into a real cluster (commonly kind) and runs `helm test`; values files in the chart's `ci/` directory (`ci/*-values.yaml`) define the tested configurations.
- `--all` or `--charts` disable changed-chart detection and the version-increment check.

##### Example CI job

```yaml
jobs:
  chart:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v5
      - uses: azure/setup-helm@v5
        with:
          version: v4.3.1            # pin; omitting it means "latest"
      - name: Lint, unit-test and render
        run: |
          set -euo pipefail
          helm dependency build charts/web
          helm lint --strict charts/web -f ci/values/prod.yaml
          helm template web charts/web -n web \
            -f ci/values/prod.yaml \
            --kube-version 1.36.0 \
            --api-versions gateway.networking.k8s.io/v1 > rendered.yaml
          # validate rendered.yaml against the target cluster's schemas (kubeconform or kubectl --dry-run=server)
```

Flag in pipelines and deploy scripts:

- unpinned Helm (`latest`, missing `version:`), actions not pinned to a SHA in security-sensitive repositories;
- `helm lint` without a path (fails on Helm 4), lint only with default values;
- `helm template` without `--kube-version`;
- deploys without explicit values files, namespace, `--timeout` and automatic rollback (`--atomic` on Helm 3, `--rollback-on-failure` on Helm 4);
- `--force`/`--force-replace`/`--force-conflicts`/`--take-ownership` as defaults, `--reuse-values`, `--no-hooks`, `--skip-schema-validation`, `--disable-openapi-validation`, `--insecure-skip-tls-verify`, `--plain-http`, `--enable-dns`, `--pass-credentials`, `--verify=false`, `helm registry login -p`, `--history-max 0`;
- Helm 4-only flags in a Helm 3 pipeline and removed Helm 3 flags (`list -a`, `version --client`, `status --show-resources`, `rollback --recreate-pods`, `--post-renderer ./script`) in a Helm 4 pipeline;
- scripts that parse human output or log lines instead of `-o json` (`helm list -o json`, `helm status -o json`, `helm get values -o yaml`), or that grep `Error: template:` (Helm 4 prints engine errors differently; use the exit code).

***

#### Security

- **Helm has no access control of its own.** It acts with the kubeconfig identity of whoever runs it; that identity's RBAC is the only boundary. CI deploy identities should be namespace-scoped, not `cluster-admin`.
- **Release Secrets and `helm get values` expose every value** ever passed to a release (see "Configuration and Secrets").
- **Untrusted charts are code.** `tpl`, `lookup` and `.Files` run with the deployer's credentials during install; `lookup` with an empty name reads every object of a kind. Review third-party charts before installing them with privileged identities.
- **Keep Helm patched where untrusted input is processed.** Historical advisories include path traversal on `helm pull --untar` with a chart named `.`/`..` (CVE-2026-35206, fixed in 3.20.2 and 4.1.4), symlinked `Chart.lock` writes during `helm dependency update` (CVE-2025-53547, fixed in 3.17.4/3.18.4), schema `$ref` and decompression-bomb DoS (fixed in 3.17.3/3.18.5), and the Helm 4 plugin-verification bypass (CVE-2026-35205, fixed in 4.1.4).[^23] CI that lints or renders charts from forks needs the latest patch of a supported line.
- **Supply chain**: deploy charts by digest, verify signatures in the deploy path, pin images by digest where possible, pin plugin versions.
- **Workloads**: restricted Pod Security defaults, no service-account token unless needed, least-privilege RBAC behind values, no `hostPath`/privileged mode by default.
- **Render-time network access** (`--enable-dns`, schema `$ref` URLs) makes rendering non-reproducible and leaks information; avoid both.
- **Logs**: `--debug` output and `helm template` output in CI logs can contain Secret data; use `--hide-secret` (3.15) with dry runs and avoid printing rendered Secrets.

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
- Use correct kubectl resource names and short names (`svc`, `deploy`, `cm`; never `rc` for anything but ReplicationControllers).
- `--hide-notes` (3.16) suppresses notes on install/upgrade.

***

#### Production / Review Checklist

##### Version

- [ ] Helm version determined (pin files, CI, images, GitOps controller) before any version-specific comment
- [ ] Helm pinned exactly to a supported line (4.3.x, or 3.22.x while migrating; Helm 3 security support ends 2027-02-10)
- [ ] No feature newer than the pinned version (see the version tables); no Helm 4 spellings in Helm 3 scripts or removed Helm 3 flags in Helm 4 scripts
- [ ] Charts consumed by both majors avoid Helm 4-only functions and multi-document `values.yaml`; schemas use draft-07 unless Helm ≥ 3.18 is required

##### Chart metadata and structure

- [ ] `apiVersion: v2`; SemVer 2 `version` with three components, bumped on every change; quoted `appVersion`
- [ ] `Chart.yaml` keys spelled correctly
- [ ] `kubeVersion` and every `semverCompare` on the Kubernetes version use the `-0` suffix
- [ ] `Chart.lock` committed; dependencies pinned; CI uses `helm dependency build`
- [ ] Chart, file, value and resource names follow conventions; every `define` prefixed with the chart name; official kind short names
- [ ] `.helmignore` excludes secrets, CI files and `/tests/` (anchored, so `templates/tests/` stays)

##### Templates

- [ ] `include` (not `template`) with `nindent`; indentation verified by rendering
- [ ] `$` inside `range`/`with` for the root context
- [ ] No `default`/`with` where `false`/`0` are valid; `required`, `fail` and the schema for mandatory and cross-field rules
- [ ] Strings quoted; numbers the API expects not quoted; env/annotation/label values are strings
- [ ] Names truncated to 63 characters including suffixes; selector labels limited to name/instance
- [ ] No `generateName`; no random/time functions without `lookup` preservation; valid output when `lookup` is empty
- [ ] `tpl` only on documented, trusted values; `dig` on `.Values.AsMap`; no `--enable-dns` dependency

##### Values and schema

- [ ] Every value documented; secure defaults; no secrets in defaults; `existingSecret` supported
- [ ] Maps preferred over lists for user-overridable collections; no duplicate list keys possible
- [ ] YAML 1.1 traps quoted (`"NO"`, `"y"`, `"1.10"`, large integers)
- [ ] `values.schema.json` with types, enums and ranges; `additionalProperties: false` only on owned nested objects; no network `$ref`

##### Resources and workloads

- [ ] Selector and other immutable-field changes treated as breaking
- [ ] Checksum annotations for env-consumed ConfigMaps/Secrets
- [ ] No rendered fields owned by other controllers (`replicas` with HPA/KEDA, injected `caBundle`)
- [ ] No duplicate object names across templates and subcharts
- [ ] Restricted Pod Security profile for workloads, hooks and test Pods; token not automounted; least-privilege RBAC
- [ ] Requests and memory limit; readiness, liveness and startup probes on named ports
- [ ] PDB with one of `minAvailable`/`maxUnavailable` (0 honoured); spread constraints; current API versions

##### Releases and deploys

- [ ] `helm upgrade --install` with explicit namespace, complete values files, `--timeout` and automatic rollback in the major's spelling
- [ ] Wait strategy understood (Helm 4 `watcher` waits for custom resources; Helm 3 ignores them and needs `--wait-for-jobs` with `--wait` for Jobs)
- [ ] No `--force`/`--force-replace`/`--force-conflicts`/`--take-ownership`/`--reuse-values` as defaults
- [ ] Post-renderers fail on empty output (Helm 3) or are pinned `postrenderer/v1` plugins (Helm 4)
- [ ] Stuck `pending-*` releases recovered with `helm rollback`/`helm uninstall`, not by deleting release Secrets

##### Hooks, tests, CRDs

- [ ] Every hook has a delete policy; weights are strings and distinct where order matters; hook Jobs bounded (`backoffLimit`, `activeDeadlineSeconds`)
- [ ] `pre-install` hooks do not depend on regular chart resources; migrations backwards-compatible and idempotent
- [ ] Test Pods pinned and Pod Security compliant; no `hook-succeeded` on test Pods when CI runs `helm test --logs` on Helm 3
- [ ] CRDs in `crds/` (never upgraded) or a separate chart; custom resources gated with `.Capabilities`

##### Supply chain and security

- [ ] Charts pushed to OCI once per version; deployments reference the digest
- [ ] Charts signed and verified in the deploy path; plugins pinned and verified (`--verify=false` justified)
- [ ] Registry login via `--password-stdin` with short-lived tokens; credentials not persisted on runners
- [ ] Release Secrets treated as sensitive; deploy identity namespace-scoped
- [ ] CI validates rendered manifests against cluster schemas and runs unit tests
