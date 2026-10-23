### Helm 4: Charts, Releases and CI — Complete Review Guide

> **Version**: Helm **v4.0.0** was released on **2025-11-12** (announced at KubeCon + CloudNativeCon North America), the first new major version in six years.[^1][^2] Minor releases since then: **4.1.0** (2026-01-21), **4.2.0** (2026-05-14) and **4.3.0** (2026-09-09). The latest release as of 2026-10-22 is **v4.3.1 (2026-10-15)**. The next scheduled releases are 4.3.2 (patch, 2026-11-11) and 4.4.0 (minor, 2027-01-13).[^3][^4] The newest 4.0.x patch is v4.0.5 (2026-01-14); Helm patches only the latest minor, so a pinned 4.0/4.1/4.2 should move to 4.3.x; 4.0.x–4.1.3 in particular lack the plugin-verification security fixes of 4.1.4 (section 2).
> **Helm 3 status**: Helm 3 is in its final, security-only phase. The original plan (bug fixes until 2026-07-08, security fixes until 2026-11-11) was extended: **v3.22.0 (2026-09-09, as scheduled in the EOL post; the GitHub release is timestamped 2026-09-10 UTC) is the last Helm 3 minor release**, bug fixes ended with it, and **security fixes continue until 2027-02-10**. After that Helm 3 receives no updates. The latest Helm 3 release is v3.22.1 (2026-10-15); v3.22.2 is scheduled for 2026-11-11.[^2][^5]
> **Kubernetes skew**: Helm 4 supports the Kubernetes minor it was compiled against and three older ones: 4.3.x → 1.37–1.34, 4.2.x → 1.36–1.33, 4.1.x → 1.35–1.32, 4.0.x → 1.34–1.31.[^6] `helm version` prints the compiled-in client-go version (`KubeClientVersion:"v1.37"` for v4.3.1).
> **Scope**: Helm charts (`Chart.yaml`, `Chart.lock`, `values.yaml`, `values.schema.json`, `templates/*.yaml`, `_helpers.tpl`, `templates/tests/`, `NOTES.txt`, `crds/`), library charts, chart CI (lint/template/package/push, OCI publishing, signing) and `helm` CLI usage in scripts and pipelines, for projects that pin **Helm 4.x** (`.tool-versions` `helm 4.x`, `azure/setup-helm` with `version: v4.x`, `alpine/helm:4.x` images, `HELM_VERSION=4.x`).
> **Note**: This guide is self-contained: it carries every still-correct general chart rule and adds the Helm 4 layer. Sections marked **[NEW in 4.0]** describe behaviour that differs from Helm 3; **[4.1]**, **[4.2]**, **[4.3]** mark changes in a minor release. Every example was rendered with `helm lint`/`helm template`/`helm package` on **v4.3.1**, boundary claims were re-run on **v4.0.5** and **v3.22.1**, and install/upgrade/uninstall semantics were exercised against a real **kube-apiserver v1.36.2** (no kubelet, so Pods never become Ready; wait behaviour on Pods is taken from the source and docs and was tested on non-Pod resources).

---

#### Table of Contents

1. What's New in Helm 4 (overview)
2. Changes in 4.1, 4.2 and 4.3
3. Do Not Use or Recommend
4. Version-Aware Review
5. Server-Side Apply (SSA)
6. Waiting: kstatus and Wait Strategies
7. Failure Handling: `--rollback-on-failure`, `--force-replace`, `--cleanup-on-fail`
8. Dry Runs, `helm template`, `lookup` and `.Capabilities`
9. Post-Renderers and Plugins
10. OCI Registries, Digests and Provenance
11. Values: Multi-Document Files, `--set-json`, Nulls and Merging
12. `Chart.yaml`: Metadata, Strict Parsing, SemVer and Dependencies
13. Chart Layout and Naming
14. `_helpers.tpl`, Names and Labels
15. Template Language: Whitespace, Scope, Functions and Pitfalls
16. Values Design
17. `values.schema.json`
18. Workload Manifests: Security, Resources, Probes, Availability
19. Configuration and Secrets
20. Hooks
21. Chart Tests
22. CRDs
23. Dependencies, Subcharts and Library Charts
24. `NOTES.txt`
25. Packaging, Publishing and Signing
26. CI Pipelines
27. Logging, Debugging and Output Formats
28. Removed / Changed in Helm 4 — Migration Notes
29. Production / Review Checklist

---

#### What's New in Helm 4

Helm 4 keeps the chart format (charts with `apiVersion: v2` render unchanged) but changes how releases are applied, how readiness is judged, how plugins and post-renderers work, and a number of CLI flags.[^1][^7][^8] The table lists what a chart author, CI maintainer or script reviewer can observe.

| Area | Helm 3 (3.22) | Helm 4 (4.0+) |
|---|---|---|
| **Apply method** | Client-side three-way merge patch (field manager `helm`, operation `Update`) | **Server-side apply** for new releases (manager `helm`, operation `Apply`). Upgrades/rollbacks keep the release's previous method (`--server-side=auto`). The method is recorded per release (`helm get metadata` shows `applyMethod: ssa`/`csa`). |
| **Conflicts** | Live drift on chart-managed fields is silently overwritten | An upgrade that changes a field owned by another field manager **fails** with `conflict with "<manager>"`; `--force-conflicts` takes ownership. |
| **`--wait`** | Boolean; polls Pods, PVCs, Services, Deployments/StatefulSets/ReplicaSets | `--wait=<strategy>`: `watcher` (kstatus; bare `--wait` means `watcher`), `legacy` (Helm 3 polling) or `hookOnly` (default when omitted). `watcher` waits for **every** resource kstatus understands, including custom resources with `Ready` conditions. |
| **Atomic** | `--atomic` | `--rollback-on-failure` (old flag hidden and deprecated; on `install` it was missing entirely before 4.1.3, see below) |
| **Force** | `--force` (replace) | `--force-replace` (old flag deprecated); new `--force-conflicts` for SSA |
| **Post-renderers** | `--post-renderer <executable>` | `--post-renderer <plugin-name>`; post-renderers are `postrenderer/v1` plugins and also see **hook** manifests |
| **Plugins** | `plugin.yaml` without `apiVersion` | New `apiVersion: v1` plugin format with `type` (`cli/v1`, `getter/v1`, `postrenderer/v1`) and `runtime` (`subprocess` or `extism/v1` WebAssembly). Legacy plugins still load. Tarball installs require a `.prov` signature by default. |
| **Values files** | Only the first YAML document of a values file is used | **Multi-document values**: all `---`-separated documents are merged in order (both `-f` files and the chart's `values.yaml`) |
| **`--set-json`** | `key=json` only | Also a whole JSON object: `--set-json '{"a":{"b":1}}'` |
| **Template functions** | — | `mustToYaml`, `mustToJson` (4.0); `mustToToml` (4.2); duration helpers (4.3) |
| **`helm lint`** | Path optional (defaults to `.`) | **Path required**; non-SemVer-2 `version` is a warning; unknown `Chart.yaml` fields are reported (warning) |
| **`helm list`** | Shows deployed+failed; `-a/--all` for everything | **Shows every status by default**; `-a/--all` is removed |
| **Registry login** | `helm registry login host/path` accepted | Host only; a path is rejected |
| **OCI** | Install/pull by digest since 3.17 | Same (tag+digest form needs ≥ 4.1.3), plus a content-addressed local cache (`--content-cache`) and `[pull,push]` token-scope fixes for pushes |
| **Uninstall** | Deletes every resource in the release manifest | **[4.3]** Skips resources whose `meta.helm.sh/release-name`/`-namespace` annotations point to another release |
| **Logging** | Ad-hoc `log` output | `log/slog` (`level=WARN msg=...`); colour via `--color`/`HELM_COLOR`/`NO_COLOR` |
| **Removed flags** | — | `helm version --client`, `helm status --show-desc/--show-resources`, `helm repo add --no-update`, `helm repo update --fail-on-repo-update-fail`, `helm rollback --recreate-pods`, `helm list -a/--all`; `helm template --validate` is deprecated (use `--dry-run=server`) |
| **Chart archives** | Every `helm package` run embeds the current time (different digest each run) | Deterministic: entries carry the files' modification times, so packaging unchanged files twice is byte-identical; **[4.3]** `SOURCE_DATE_EPOCH` overrides the times |
| **Chart API v3** | — | Experimental only, behind `HELM_EXPERIMENTAL_CHART_V3=1`; not usable for real charts (see "Do Not Use") |
| **Template output** | `helm template` default `.Capabilities.KubeVersion` = v1.37.0 (3.22.1) | v1.34.0 in 4.0.5, v1.37.0 in 4.3.1 — the default tracks the compiled-in client-go |

Verified on the apiserver (v4.3.1 vs v3.22.1):

```console
$ helm install r4 ./app -n t4          # Helm 4.3.1
$ kubectl get cm r4-cm -n t4 --show-managed-fields \
    -o jsonpath='{range .metadata.managedFields[*]}{.manager} {.operation}{"\n"}{end}'
helm Apply
$ helm get metadata r4 -n t4 -o yaml | grep applyMethod
applyMethod: ssa

$ helm3 install r3 ./app -n t3        # Helm 3.22.1
...
helm Update
```

***

#### Changes in 4.1, 4.2 and 4.3

Minor releases add features; only the latest minor receives patches. Review a chart or pipeline against the exact pinned minor.

##### 4.1 (4.1.0 on 2026-01-21; last patch 4.1.4 on 2026-04-09)[^9]

- `--wait` accepts an explicit `hookOnly` strategy (`--wait=hookOnly`); 4.0.x accepts only `watcher` and `legacy` (verified: v4.0.5 rejects `--wait=hookOnly` with `Valid inputs are watcher, and legacy`).
- `helm repo list --no-headers`.
- `--kube-version` keeps vendor suffixes (`1.34.1-eks-abc`) in `.Capabilities.KubeVersion` (section 8).
- Kubernetes client libraries 1.35.
- Fixes listed in the 4.1.0 notes that were already backported to **4.0.5** (verified on v4.0.5 where testable): `helm upgrade --install --server-side=...` passes the flag through to the install path, status waiting watches namespace-scoped (no cluster-wide LIST permission needed), plugin names that shadow a built-in command are rejected at load time (`name conflicts`), and `helm test --logs` works with `hook-succeeded`/`hook-failed` delete policies. Unresolved URN `$ref`s in `values.schema.json` warn and are ignored since **4.0.2**.
- There is no 4.1.2 release (a tag exists, but release automation failed); **4.1.3** (2026-03-11) restores `helm install --atomic` as a deprecated alias (in 4.0.x–4.1.1 it fails with `unknown flag: --atomic`)[^10], fixes `chart:tag@sha256:…` OCI references (4.0.x fails with `encoding/hex: invalid byte: U+0073 's'`)[^11], makes `--dry-run=server` accept `metadata.generateName`, fixes pulling charts from OCI image indexes, fixes YAML corruption when template trimming glues content to a `---` separator (seen after post-rendering), and restores Helm 3 behaviour for user-supplied `null` under an empty-map chart default (4.0.x dropped the key).[^12][^30]
- **4.1.4** (2026-04-09) is a security release:[^31] CVE-2026-35205 (GHSA-q5jf-9vfq-h4h7, critical — `helm plugin install`/`update` of an archive **without** a `.prov` file succeeded even when verification was required; 4.0.5 only prints `WARNING: No provenance file found for plugin`), CVE-2026-35204 (GHSA-vmx8-mqv2-9gmg, high — a plugin `version` such as `../../x` wrote files outside the plugin directory; plugin versions must now be SemVer, verified: v4.0.5 installs such a plugin, v4.3.1 rejects it with `must be valid semver`) and CVE-2026-35206 (GHSA-hr2v-4r36-88hr, moderate — a chart named `.`/`..` made `helm pull --untar` extract into the output directory itself; also fixed in 3.20.2). Helm 4.0.x never received these fixes.

##### 4.2 (4.2.0 on 2026-05-14; last patch 4.2.4 on 2026-08-13)[^13]

- `mustToToml` template function.
- `--dry-run=server` respects `metadata.generateName` (listed in the 4.2.0 notes, but already shipped in 4.1.3; real installs still reject it; see section 8).
- `helm template --hide-notes` and `--render-subchart-notes` are deprecated no-ops ("will be removed in Helm 5").
- `helm create --chart-api-version` (gated by `HELM_EXPERIMENTAL_CHART_V3`).
- Release builds switched to GoReleaser; Kubernetes client libraries 1.36; Go 1.26.
- **4.2.1**: registry client messages (`Pulled:`, `Pushed:`, `Digest:`) go to stdout instead of stderr (which also polluted `helm template` output until 4.3.0 routed them back to stderr there).
- **4.2.4**: server-side apply retries on conflicts caused by concurrent writers; `helm push` requests the correct `pull,push` token scope from token-auth registries; `helm test --logs` shows logs of all containers; `helm template` no longer drops blank lines at the end of a document (section 15).

##### 4.3 (4.3.0 on 2026-09-09; 4.3.1 on 2026-10-15)[^14]

- Duration template functions: `mustToDuration`, `durationSeconds`, `durationMilliseconds`, `durationMicroseconds`, `durationNanoseconds`, `durationMinutes`, `durationHours`, `durationDays`, `durationWeeks`, `durationRoundTo`, `durationTruncateTo`.
- `helm uninstall` verifies ownership annotations before deleting each resource.
- `helm rollback --description` (max 256 characters) and `helm history --show-rollback-revision`.
- `helm package` honours `SOURCE_DATE_EPOCH` (byte-identical archives).
- Provenance verification accepts GnuPG 2 `pubring.kbx` keyrings.
- The engine logs at debug level when `lookup` finds nothing (only when rendering against a cluster).
- Status computation is concurrent; the watcher's resync period is lowered from 1 h to 3 min.
- Registry client messages go to stderr again during `helm template`/`helm show` (stdout elsewhere, since 4.2.1).
- `helm test` skips non-Pod test hooks when fetching logs.
- The experimental chart v3 code is removed (section 3).
- A values file whose size is an exact multiple of 4096 bytes no longer loses its last line.
- **4.3.1**: hook-only waiting also waits for CRDs to become `Established`.
- Kubernetes client libraries 1.37.

***

#### Do Not Use or Recommend

##### Not available in the pinned 4.x minor

Check the pinned minor before suggesting any of these:

| Feature | First available | Behaviour on an older 4.x |
|---|---|---|
| `mustToToml` | 4.2 | `function "mustToToml" not defined` |
| `durationSeconds`, `mustToDuration`, other `duration*` helpers | 4.3 | `function "durationSeconds" not defined` |
| `--wait=hookOnly` written explicitly | 4.1 | rejected by 4.0.x (omit `--wait` instead) |
| `helm install --atomic` (deprecated alias) | 4.1.3 | `unknown flag: --atomic` before 4.1.3; use `--rollback-on-failure` everywhere |
| `oci://…/chart:1.2.3@sha256:…` (tag **and** digest) | 4.1.3 | `encoding/hex: invalid byte` on 4.0.x; digest-only `chart@sha256:…` works on every 4.x |
| `helm repo list --no-headers` | 4.1 | unknown flag |
| `--dry-run=server` with `generateName` | 4.1.3 | `resource name may not be empty` |
| `helm create --chart-api-version` | 4.2 | unknown flag |
| `helm rollback --description`, `helm history --show-rollback-revision` | 4.3 | unknown flag |
| `SOURCE_DATE_EPOCH` for reproducible `helm package` | 4.3 | ignored; archive entries carry the files' modification times (a fresh checkout gives a new digest) |
| Unsigned plugin archives rejected by `helm plugin install` | 4.1.4 | installed with only a warning (CVE-2026-35205); pin ≥ 4.1.4 |
| `pubring.kbx` as `--keyring` | 4.3 | `failed to load keyring: openpgp: invalid data` |
| Uninstall ownership check | 4.3 | 4.0–4.2 delete every resource listed in the release manifest |

Also never recommend:

- **Chart `apiVersion: v3`.** Charts v3 (HIP-0020) is experimental. In v4.3.1, `HELM_EXPERIMENTAL_CHART_V3=1 helm create --chart-api-version=v3` writes `apiVersion: v3`, but `helm template` and `helm install` on that chart fail with `Error: invalid chart apiVersion` (the experimental v3 code was removed from 4.3). Charts must use `apiVersion: v2`.[^15]
- **Template functions supplied by plugins.** Custom template functions exist only in the Go SDK (`action.Configuration.CustomTemplateFuncs`); there is no CLI plugin type for them in 4.3 (plugin types are `cli/v1`, `getter/v1`, `postrenderer/v1`).
- **Helm 5 features.** Nothing from Helm 5 exists; flags documented as "will be removed in Helm 5" still work.

##### Removed or deprecated in Helm 4 (flag in scripts and CI)

| Helm 3 usage | Helm 4 |
|---|---|
| `--atomic` (install/upgrade) | Deprecated alias; use `--rollback-on-failure` |
| `--force` (upgrade/rollback) | Deprecated alias; use `--force-replace` |
| `--post-renderer ./script.sh` | **Error**: `plugin: {Name:./script.sh Type:postrenderer/v1} not found` |
| `helm registry login registry.example.com/team` | **Error**: `invalid reference: invalid registry` (4.3 also warns and suggests the host) |
| `helm list -a` / `--all` | **Error**: `unknown shorthand flag: 'a'`; default output already includes every status |
| `helm version --client` / `-c` | **Error**: `unknown flag: --client` |
| `helm status --show-resources` / `--show-desc` | **Error**; both are always shown |
| `helm repo add --no-update` | **Error**: unknown flag |
| `helm repo update --fail-on-repo-update-fail` | **Error**: unknown flag; `repo update` now always fails if any repository fails |
| `helm rollback --recreate-pods` | **Error**: unknown flag |
| `helm lint` (no path) | **Error**: `"helm lint" requires at least 1 argument` |
| `helm template --validate` | Deprecated; use `--dry-run=server` |
| bare `--dry-run` | Deprecated (`--dry-run is deprecated and should be replaced with '--dry-run=client'`) |
| `--wait=true` / `--wait=false` | Deprecated booleans (warning suggests `watcher` for `true`; for `false` 4.3 suggests `hookOnly`, 4.0 suggests omitting `--wait`) |
| `helm template --hide-notes`, `--render-subchart-notes` | No-ops, deprecated in 4.2 |
| `HELM_EXPERIMENTAL_OCI` | Not needed (OCI is GA since 3.8) |

***

#### Version-Aware Review

- **Find the pinned version first**: `.tool-versions` (`helm 4.3.1`), `mise.toml`, `azure/setup-helm` `with: version:`, container images (`alpine/helm:4.3.1`), `HELM_VERSION` in Makefiles or Dockerfiles, Renovate/Dependabot config. A guide claim like "not available" is only valid against the pinned minor.
- **Pin an exact version in CI.** `azure/setup-helm` defaults to `latest` when `version` is omitted, so an unpinned job can switch between major versions without a code change.[^16] Flag `version: latest` or a missing `version`.
- **Say "not available in 4.x" only when the pinned version proves it** (unknown flag, `function "…" not defined`). Do not infer from Helm 3 docs: several Helm 3 docs pages describe behaviour (implicit `helm lint .`, `helm list -a`, executable post-renderers) that Helm 4 removed.
- **Charts are usually shared between Helm 3 and Helm 4 users.** A chart published for others must render on both unless it declares otherwise. Helm 4-only template functions (`mustToYaml`, `mustToJson`, `mustToToml`, `duration*`) make the chart fail on Helm 3 with `function "…" not defined`; multi-document `values.yaml` is silently truncated to the first document by Helm 3. Treat both as breaking for a chart consumed by Helm 3 users; acceptable when the chart is internal and every consumer runs Helm ≥ the required minor (document it in the README or enforce it with a `fail` on `.Capabilities.HelmVersion`).
- **`.Capabilities.HelmVersion.Version`** reports the running Helm (`v4.3.1`). A chart can guard a Helm 4-only feature:

```yaml
{{- if not (semverCompare ">=4.3.0-0" .Capabilities.HelmVersion.Version) }}
{{- fail "this chart requires Helm >= 4.3.0" }}
{{- end }}
```

  Note that the guard cannot protect a template that already references an unknown function: Go templates fail at **parse** time, before any `if` runs. Put Helm 4-only function calls only in charts that require Helm 4.

***

#### Server-Side Apply (SSA) [NEW in 4.0]

Helm 4 implements HIP-0023: new releases are applied with Kubernetes server-side apply under the field manager `helm`.[^17][^18] The apiserver tracks which manager owns each field (`metadata.managedFields`) and refuses an apply that would change a field another manager owns, unless the apply forces the conflict.

##### Which method a release uses

| Command | Default | Result |
|---|---|---|
| `helm install` | `--server-side=true` | SSA |
| `helm upgrade` / `helm rollback` | `--server-side=auto` | Same method as the previous revision. A release created by Helm 3 stays on client-side apply (`csa`) after a Helm 4 upgrade. |
| `helm upgrade --server-side=true` | — | Switches the release to SSA (Helm migrates the `Update` managed-field entries of `helm` to `Apply`) |
| `helm upgrade --server-side=false` | — | Forces client-side three-way merge |
| `helm upgrade --install` | install path uses `--server-side` as given (fixed in 4.0.5; verified) | — |

The method is stored in the release and shown by `helm get metadata` (`applyMethod: ssa` or `applyMethod: csa`). Verified: a Helm 3-installed release upgraded by Helm 4.3.1 keeps `applyMethod: csa` and its ConfigMap keeps the `helm Update` managed-field entry; `--server-side=true` converts it to `helm Apply`, and fields dropped from the chart in that same upgrade are removed correctly.

##### Drift now fails upgrades instead of being overwritten

With client-side apply (Helm 3, or a `csa` release), Helm computes a three-way patch between the previous manifest, the live object and the new manifest; a live value changed by someone else is overwritten with the chart's value. With SSA, a field touched by another manager — `kubectl edit`/`patch`/`scale`, an HPA, an operator, a mutating controller — belongs to that manager, and Helm's next apply of a **different** value fails:

```console
$ kubectl scale deploy d4 -n s4 --replicas=3        # Deployment installed by Helm 4 with replicas: 1
$ helm upgrade d4 ./app -n s4 --set image=nginx:1.28
Error: UPGRADE FAILED: conflict occurred while applying object s4/d4 apps/v1, Kind=Deployment:
Apply failed with 1 conflict: conflict with "kubectl" with subresource "scale" using apps/v1: .spec.replicas
```

The same chart and commands under Helm 3.22.1 succeed and silently reset `replicas` back to 1. With `kubectl patch` the conflict names `kubectl-patch`; with `kubectl apply --server-side --field-manager=other` it names `other`.

Review consequences:

- **Do not render fields another controller owns.** The classic case is `spec.replicas` on a Deployment/StatefulSet scaled by an HPA or KEDA: omit it when autoscaling is enabled. Under Helm 4 this is no longer just a correctness nicety — every upgrade that renders a different `replicas` fails.

```yaml
# BAD: replicas always rendered; conflicts with the HPA's scale subresource writes
spec:
  replicas: {{ .Values.replicaCount }}

# GOOD
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
```

- Other fields commonly owned by someone else: `metadata.annotations` written by tools (`kubectl.kubernetes.io/restartedAt` from `kubectl rollout restart` lives under the pod template), webhook-injected sidecars and caBundles (`caBundle` in `ValidatingWebhookConfiguration`/`MutatingWebhookConfiguration`/`APIService`/CRD conversion webhooks filled in by cert-manager's cainjector), `Service.spec.clusterIP`, and `status`. Do not template a placeholder for a value another component fills in: leave the field out.
- **`--force-conflicts`** makes Helm take ownership and overwrite. It is the right tool for a one-off migration or to repair manual hot-fixes; it is wrong as a permanent CI flag, because it silently reverts every legitimate writer (HPA, operators) on each deploy. Flag `--force-conflicts` in a pipeline unless a comment explains why.
- **Hand edits become sticky.** After `kubectl edit`, the edited field belongs to `kubectl-edit`; Helm cannot change it again without `--force-conflicts`. Fix drift through the chart, not the cluster.
- SSA builds a typed patch on the server, so manifests that client-side apply tolerated can now fail. Duplicate keys in a keyed list are the common case. Two `env` entries with the same `name` only produce a warning under Helm 3.22.1 (`hides previous definition of "A"`), but fail under Helm 4.3.1:

```console
Error: INSTALLATION FAILED: server-side apply failed for object de4/de apps/v1, Kind=Deployment:
failed to create typed patch object (...): .spec.template.spec.containers[name="c"].env: duplicate entries for key [name="A"]
```

  This typically happens when a chart appends user-supplied `extraEnv` to built-in variables. Merge them by name (a dict keyed by variable name) or document that overrides must not repeat built-in names.
- **Duplicate objects in one release are no longer detected.** If two templates render the same kind/namespace/name, Helm 3.22.1 fails the install (`configmaps "same" already exists`); Helm 4.3.1 applies both documents to one object and reports `deployed` — the content depends on apply order. Catch duplicate names in review (two templates using the same `include "x.fullname"` without a suffix, subcharts with `fullnameOverride` collisions); `helm lint` does not detect them.
- SSA conflicts caused by concurrent writers (not by other field managers' ownership) are retried since 4.2.4.

##### `--force-replace` is not `--force-conflicts`

`--force-replace` (formerly `--force`) replaces each object with a full `PUT` of the rendered manifest instead of applying or patching it, so every field the chart does not render — values written by controllers, webhooks or other tools — is reset; changes to immutable fields still fail. `--force-conflicts` keeps the server-side apply semantics and only takes over field ownership. They solve different problems; neither belongs in a default deploy command.

***

#### Waiting: kstatus and Wait Strategies [NEW in 4.0]

Helm 4 implements HIP-0022: readiness is computed with kstatus (`fluxcd/cli-utils` kstatus in 4.3.1), driven by watches instead of polling.[^19][^20]

| Strategy | How to select | What Helm waits for |
|---|---|---|
| `hookOnly` | default when `--wait` is omitted; explicit `--wait=hookOnly` needs ≥ 4.1 | Hook Pods/Jobs complete (and, since 4.3.1, CRDs are `Established`). Regular chart resources are not waited for. |
| `watcher` | bare `--wait`, or `--wait=watcher` | Every resource reaches kstatus `Current`: Deployments/StatefulSets/DaemonSets/ReplicaSets rolled out and available, Pods ready, PVCs `Bound`, LoadBalancer Services with a ClusterIP; Jobs count as ready once **started**, or once **completed** with `--wait-for-jobs` (a failed Job is reported as `Failed`); **custom resources are judged by their `Ready`/`Reconciling`/`Stalled` conditions and `status.observedGeneration`** (a CR without status is `Current`). Paused Deployments are skipped. |
| `legacy` | `--wait=legacy` | Helm 3 algorithm: polls Pods, PVCs, Services, minimum available Pods of Deployments/StatefulSets/ReplicaSets/DaemonSets; ignores custom resources |

Verified with a CRD whose object carries `status.conditions: [{type: Ready, status: "False", message: waiting for backend}]`:

```console
$ helm install ks-watcher ./ks --wait=watcher --timeout 10s
Error: INSTALLATION FAILED: resource Gadget/k/g not ready. status: InProgress, message: waiting for backend
$ helm install ks-legacy ./ks --wait=legacy --timeout 10s
STATUS: deployed
$ helm3 install ks3 ./ks --wait --timeout 10s
STATUS: deployed
```

Review consequences:

- `--wait` under Helm 4 means more than under Helm 3. A chart that ships custom resources for an operator (Certificates, Kafka topics, database clusters) now blocks until the operator marks them Ready. That is usually what you want; if the operator is installed by the same pipeline later, or the CR legitimately never becomes Ready (e.g. waits for external DNS), the deploy times out. Choose the strategy deliberately and set `--timeout` explicitly.
- Error messages name the resource and its kstatus message (`resource Deployment/s4/w1 not ready. status: InProgress, message: Replicas: 0/1` on 4.3.1; 4.0.5 prints `resource not ready, name: g, kind: Gadget, status: InProgress`), which makes CI failures diagnosable. `legacy` only says `context deadline exceeded`.
- `--wait` without `--timeout` uses the default timeout of 5m0s.
- `--wait-for-jobs` still applies: without it, a regular (non-hook) Job counts as ready as soon as it has started; with it, Helm waits for `Complete` and fails fast on `Failed`.
- `--wait=true`/`--wait=false` are deprecated boolean spellings; write `--wait` / `--wait=watcher`, or omit the flag.
- `helm uninstall --wait` waits for deletion; since 4.2 the help recommends `--cascade foreground` with `--wait` so resources with finalizers are fully gone before the command returns.

***

#### Failure Handling: `--rollback-on-failure`, `--force-replace`, `--cleanup-on-fail`

```bash
# BAD (Helm 4): deprecated aliases; 'helm install --atomic' is an unknown flag before 4.1.3
helm upgrade --install web ./chart --atomic --force

# GOOD
helm upgrade --install web ./chart \
  --namespace web --create-namespace \
  --rollback-on-failure --timeout 10m \
  -f values/prod.yaml
```

- `--rollback-on-failure` implies `--wait=watcher` unless another strategy is given. On install failure the release is uninstalled (`release w3 failed, and has been uninstalled due to rollback-on-failure being set: resource Deployment/s4/w3 not ready...`); on upgrade failure Helm rolls back to the last successful revision.
- `--cleanup-on-fail` deletes resources created by the failed upgrade; combine it with `--rollback-on-failure` when new objects must not linger.
- `helm rollback --description "reason"` (4.3, ≤ 256 characters) records why; `helm history --show-rollback-revision` (4.3) shows which revision a rollback restored.
- `helm list` in Helm 4 lists **all** statuses by default (deployed, failed, pending-*, superseded, uninstalling, uninstalled with `--keep-history`). A script that relied on Helm 3's "deployed or failed only" default must add `--deployed`/`--failed` filters. A release stuck in `pending-install`/`pending-upgrade` (killed CI job) still blocks further operations with `another operation (install/upgrade/rollback) is in progress`; fix with `helm rollback` or `helm history` + `helm uninstall`, not by deleting release Secrets by hand.

***

#### Dry Runs, `helm template`, `lookup` and `.Capabilities`

##### Dry-run modes

| Command | Cluster contact | `lookup` | What is checked |
|---|---|---|---|
| `helm template` | none | returns empty map | rendering only (offline `.Capabilities`) |
| `helm template --dry-run=server` (replaces the deprecated `--validate`) | yes | real data | rendering with real `.Capabilities`; kinds must be known to the cluster |
| `helm install/upgrade --dry-run=client` (bare `--dry-run` is deprecated) | only for release-history checks | returns empty map | rendering |
| `helm install/upgrade --dry-run=server` | yes | real data | rendering, kinds known to the cluster (`no matches for kind "Nope" in version "example.com/v1"` / `ensure CRDs are installed first`), ownership of existing objects |
| `--dry-run=none` | — | — | normal execution (the default) |

None of the dry-run modes sends the objects to the apiserver for a server-side dry-run apply. Verified on 4.3.1: a ConfigMap with a misspelled top-level field (`datax:`) and a Pod with an integer env value both pass `helm install --dry-run=server` (`STATUS: pending-install`) and only fail on the real install. To validate objects against the cluster's schemas and admission chain, pipe the rendered output to `kubectl apply --dry-run=server -f -` (or validate offline with kubeconform and the target cluster's OpenAPI schemas). `--hide-secret` hides Secret contents in dry-run output.

**[NEW in 4.0] Unknown fields fail the install.** Server-side apply builds a typed patch, so a field not in the schema is an error. Helm 3's client-side apply only logged a warning and dropped the field:

```console
# Helm 4.3.1
Error: INSTALLATION FAILED: server-side apply failed for object default/t1 /v1, Kind=ConfigMap:
failed to create typed patch object (default/t1; /v1, Kind=ConfigMap): .datax: field not declared in schema
# Helm 3.22.1
Warning: unknown field "datax"
STATUS: deployed
```

A chart that "worked" on Helm 3 with a mis-indented or misspelled field (the field was silently ignored) fails on Helm 4 — which is good, but the migration surfaces these latent bugs on the first Helm 4 install. Releases on client-side apply (Helm 3-created releases upgraded with `--server-side=auto`, or `--server-side=false`) keep the old behaviour: verified on 4.3.1, `helm install --server-side=false` of the same chart logs `Warning: unknown field "datax"` and deploys.

`lookup` behaviour, verified on v4.3.1 against an existing Secret:

```yaml
{{- $existing := lookup "v1" "Secret" .Release.Namespace "lk-secret" }}
annotations:
  found: {{ empty $existing | ternary "no" "yes" | quote }}
# helm template:            found: "no"
# --dry-run=client:         found: "no"
# --dry-run=server/install: found: "yes"
```

Never design a chart so that `helm template` output is wrong or invalid when `lookup` returns empty: GitOps tools (Argo CD, Flux with post-renderers) render with `helm template` semantics, so `lookup`-dependent branches always take the "not found" path there. Since 4.3 the engine logs `lookup` misses at debug level (`level=DEBUG msg="lookup: resource not found" ... name=...`) when rendering against a cluster (`install`/`upgrade --debug`, `--dry-run=server --debug`); plain `helm template` never calls the cluster, so there is nothing to log.

##### `generateName`

Helm tracks resources by name, so `metadata.generateName` without `metadata.name` is unsupported. `--dry-run=server` accepts it since 4.1.3 (4.0.5 rejects even the dry run), but a real install still fails on v4.3.1:

```console
$ helm install gnr ./lk -n l
Error: INSTALLATION FAILED: server-side apply failed for object l/ /v1, Kind=ConfigMap: resource name may not be empty
```

Flag `generateName` in chart templates.

##### `.Capabilities` offline vs. online

- Offline (`helm template`, `helm lint`, `--dry-run=client`), `.Capabilities.KubeVersion` defaults to the client-go version Helm was built with: **v1.34.0 on 4.0.5**, **v1.37.0 on 4.3.1** (and v1.37.0 on 3.22.1). A `kubeVersion` constraint or `semverCompare` gate therefore renders differently on different Helm minors in CI. Pass `--kube-version` explicitly in CI to render for the target cluster. Vendor suffixes are preserved since 4.1: on 4.3.1, `--kube-version 1.34.1-eks-abc` gives `.Capabilities.KubeVersion.Version` and `.GitVersion` = `v1.34.1-eks-abc` (4.0.5 strips the suffix), so `semverCompare` constraints need a `-0` suffix (`">=1.33.0-0"`).
- Offline `.Capabilities.APIVersions` contains only **group/version** entries of built-in APIs (57 entries on 4.3.1), not `group/version/Kind`. Against a cluster it contains both forms (about 100 entries on a bare 1.36 apiserver). So `.Capabilities.APIVersions.Has "apps/v1/Deployment"` is `false` under `helm template` and `true` on install, while `Has "policy/v1"` is `true` in both. Gate on group/version, or pass `--api-versions` for CRD-provided APIs:

```bash
helm template web ./chart --kube-version 1.36.0 \
  --api-versions gateway.networking.k8s.io/v1 \
  --api-versions monitoring.coreos.com/v1
```

```yaml
{{- if .Capabilities.APIVersions.Has "monitoring.coreos.com/v1" }}
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
...
{{- end }}
```

- Gate optional CRD-based resources on capabilities **and** an explicit value (`{{- if and .Values.metrics.serviceMonitor.enabled (.Capabilities.APIVersions.Has "monitoring.coreos.com/v1") }}`), so GitOps renders stay deterministic.

***

#### Post-Renderers and Plugins [NEW in 4.0]

##### Post-renderers are plugins

Helm 4 removes executable post-renderers. `--post-renderer` takes the **name of an installed plugin of type `postrenderer/v1`**:[^7][^21]

```console
$ helm template x ./app --post-renderer ./kustomize-wrapper.sh
Error: invalid argument "./kustomize-wrapper.sh" for "--post-renderer" flag: plugin: {Name:./kustomize-wrapper.sh Type:postrenderer/v1} not found
```

A post-renderer plugin that runs a Kustomize overlay (tested on 4.3.1; the plugin format is the same on 4.0.5):

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
# stdin: all rendered manifests (including hooks); stdout: the post-rendered stream
set -eu
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
cat > "$dir/all.yaml"
cp "$HELM_PLUGIN_DIR/kustomization.yaml" "$dir/"
kubectl kustomize "$dir"
```

```bash
helm plugin install ./plugins/kustomize-env   # local directory: installed as "local dev", no signature needed
helm upgrade --install web ./chart --post-renderer kustomize-env
```

`--post-renderer-args` passes extra arguments to the plugin command.

- Post-renderers in Helm 4 also receive **hook** manifests (Helm 3 passed only regular templates). Verified: a `sed`-based post-renderer modified both `templates/cm.yaml` and the `pre-install` hook in 4.0.5/4.3.1, but only `cm.yaml` in 3.22.1; the Kustomize plugin above annotates both documents of that chart on 4.3.1. An overlay that patches "all Deployments" now also patches hook Jobs/Pods; check that its selectors are precise.
- An empty post-renderer output is an error, and only one `--post-renderer` may be given.
- In CI, install the plugin explicitly (`helm plugin install <path|url|oci://…>`) before the deploy step, pin its version (`--version`), and keep plugin code in the repository or a signed artifact.

##### The new plugin format

| Field | Values |
|---|---|
| `apiVersion` | `v1` (new format). Files without `apiVersion` are legacy Helm 3 plugins and still load as `cli/v1` or `getter/v1`. |
| `type` | `cli/v1` (adds a `helm <name>` subcommand), `getter/v1` (downloads `protocol://` URLs), `postrenderer/v1` |
| `runtime` | `subprocess` (executes `runtimeConfig.platformCommand`) or `extism/v1` (sandboxed WebAssembly, HIP-0026) |
| `version` | Required; must be SemVer since 4.1.4 (CVE-2026-35204) |

- A plugin directory shared with Helm 3 breaks Helm 3: v3.22.1 aborts `helm plugin list` with `failed to load plugin at ".../plugin.yaml": error unmarshaling JSON: while decoding JSON: json: unknown field "apiVersion"`. Do not point `HELM_PLUGINS`/`HELM_DATA_HOME` of both majors at the same directory once v1 plugins are installed.
- `helm plugin install` from a tarball (`.tgz`) **verifies a `.prov` signature by default** and fails if it is missing or invalid (`plugin verification failed: no provenance file (.prov) found`) — but only since **4.1.4**; 4.0.x–4.1.3 install an unsigned archive with a warning (CVE-2026-35205). `--verify=false` disables the check and should be flagged in review. Local directories and VCS installs are not signature-checked. `helm plugin package` and `helm plugin verify` create and check signed plugin archives.
- Plugins can be installed from OCI registries (`helm plugin install oci://…`), HTTP(S) tarballs, VCS URLs and local paths.
- `HELM_NO_PLUGINS=1` disables all plugins, useful for reproducible CI renders.
- A plugin name may not shadow a built-in command (since 4.0.5: such a plugin installs, then fails to load with `name conflicts`).

***

#### OCI Registries, Digests and Provenance

##### Pin charts by digest

OCI chart references can carry a digest; Helm verifies it and refuses a mismatch. Verified against a local `registry:2`:

| Reference | v3.22.1 | v4.0.5 | v4.3.1 |
|---|---|---|---|
| `oci://reg/charts/app@sha256:<good>` | renders | renders | renders |
| `oci://reg/charts/app:0.1.0@sha256:<good>` | renders | `encoding/hex: invalid byte: U+0073 's'` | renders |
| `oci://reg/charts/app@sha256:<wrong>` | `not found` | `not found` | `not found` |
| `oci://reg/charts/app:0.1.0@sha256:<wrong>` | `chart reference digest mismatch` | same | same |

```bash
# GOOD: immutable, reviewable deploy input (4.1.3+ for tag@digest; digest-only works on all 4.x)
helm upgrade --install web oci://ghcr.io/acme/charts/web:1.4.0@sha256:<digest printed by helm push>

# BAD: mutable tag only — the registry can serve different content for the same tag
helm upgrade --install web oci://ghcr.io/acme/charts/web --version 1.4.0
```

`helm push` prints the digest (`Digest: sha256:…`); record it in the release pipeline output and in the GitOps repository.

##### Registry authentication

```bash
# GOOD (Helm 4): host only, password from stdin
echo "$GITHUB_TOKEN" | helm registry login ghcr.io -u "$GITHUB_USER" --password-stdin

# BAD (Helm 4): repository path -> "invalid reference: invalid registry"
helm registry login ghcr.io/acme/charts -u "$GITHUB_USER" -p "$GITHUB_TOKEN"   # also leaks the token via argv
```

- Credentials are stored in `$HELM_REGISTRY_CONFIG` (default `~/.config/helm/registry/config.json`) as base64 `auth`, i.e. plaintext; CI runners must not persist this file between jobs. Prefer short-lived tokens (GitHub `GITHUB_TOKEN`, cloud workload identity).
- `--plain-http` and `--insecure-skip-tls-verify` belong only in local test setups.
- `helm pull`/`install` of an OCI chart without `--version` picks the highest SemVer tag; Helm 4 warns when it falls back to a different version than requested.
- Charts referenced in `Chart.yaml` dependencies may use `repository: oci://registry/charts`; `Chart.lock` pins the resolved version and a digest of the dependency set.

##### Provenance (`.prov`)

```bash
helm package ./web --sign --key 'Chart Signer' --keyring ~/.gnupg/secring.gpg
helm verify web-1.4.0.tgz --keyring ./ci/pubring.gpg
helm install web ./web-1.4.0.tgz --verify --keyring ./ci/pubring.gpg
```

- `--sign` needs a legacy secret keyring export (`gpg --export-secret-keys > secring.gpg`); Helm uses Go's OpenPGP implementation, not `gpg`.
- Since **4.3**, `--keyring` may be a GnuPG 2 `pubring.kbx`; on 4.0.5 and 3.22.1 it fails with `failed to load keyring: openpgp: invalid data: tag byte does not have MSB set` — export with `gpg --export > pubring.gpg` there.
- `helm push` uploads the `.prov` file alongside the chart when it exists next to the archive. Many teams sign OCI charts with Sigstore/cosign instead; either way, verification must happen in the deploy path, not only at publish time.

##### Reproducible archives [NEW in 4.0; `SOURCE_DATE_EPOCH` in 4.3]

Helm 4 writes deterministic archives: packaging the same files twice gives the same sha256, and each entry carries the file's **modification time** (verified on 4.0.5 and 4.3.1; 3.22.1 embeds the current time and produces a new digest on every run). Touching a file, or a fresh `git checkout` in CI (mtime = checkout time), therefore still changes the digest. Since **4.3**, `SOURCE_DATE_EPOCH` replaces the file times (verified: identical sha256 across checkouts, entries dated `2023-11-14 22:13` for `SOURCE_DATE_EPOCH=1700000000`); 4.0.5 ignores the variable.

```bash
export SOURCE_DATE_EPOCH="$(git log -1 --format=%ct)"
helm package ./charts/web -d dist/
```

***

#### Values: Multi-Document Files, `--set-json`, Nulls and Merging

##### Multi-document values [NEW in 4.0]

Helm 4 reads every YAML document in a values file and deep-merges them in order. This applies to `-f/--values` files **and** to the chart's own `values.yaml`. Helm 3 silently uses only the first document.

```yaml
# values.yaml
a: 1
nested: {x: 1}
---
b: 2
nested: {x: 10}
```

| | Helm 3.22.1 | Helm 4.0.5 / 4.3.1 |
|---|---|---|
| `.Values` | `{a: 1, nested: {x: 1}}` | `{a: 1, b: 2, nested: {x: 10}}` |

Review: a `---` inside a chart's `values.yaml` makes the chart behave differently under Helm 3 and Helm 4 without any error. Do not use multi-document `values.yaml` in charts that may be installed by Helm 3; in environment overlays used only with Helm 4 it is fine.

##### `--set-json` with a JSON object [NEW in 4.0]

```bash
helm upgrade --install web ./chart --set-json '{"podAnnotations":{"team":"payments"},"replicaCount":3}'
```

Helm 3.22.1 rejects this form (`failed parsing --set-json data`). Precedence is unchanged: `-f` files, then `--set-json`, then `--set`, then `--set-string`, then `--set-file`, then `--set-literal`, each later source overriding earlier ones (verified: `-f` + `--set-json c=10` + `--set c=20` gives `c: 20`).

##### Deleting a default with `null`

A user value of `null` removes the key from the chart default:

```bash
helm template nv ./nv --set resources.limits=null   # drops the limits block from the defaults
```

With a values file, `cpu: null` under a non-empty default removes `cpu`. If the chart default is an **empty map** (`annotations: {}`), a user `a: null` is kept as `a: null` in 3.22.1 and 4.3.1 (4.0.5 dropped it; fixed in 4.1.3). Templates that range over user maps must therefore tolerate `nil` values (`{{- if not (kindIs "invalid" $v) }}`), or the rendered manifest gets `a: null`.

##### Implicit `--reuse-values` on upgrade

`helm upgrade` **without any** `-f`/`--set*` flag reuses the previous release's values (verified on 3.22.1 and 4.3.1: a value set with `--set` at install time survives a bare `helm upgrade`). With at least one values flag, only chart defaults plus the new flags are used. CI should always pass the full values (`-f values/prod.yaml`) and use `--reset-values` or `--reset-then-reuse-values` explicitly when the intent matters. Flag `--reuse-values` in pipelines: it also keeps stale values of keys the new chart version renamed or removed.

##### YAML typing traps (all Helm versions)

Values are parsed as YAML 1.1 and then converted through JSON, so:[^22]

| values.yaml | `.Values` sees | Fix |
|---|---|---|
| `country: NO` | `false` | `country: "NO"` |
| `list: [x, y]` | `["x", true]` (`y` is boolean) | quote: `["x", "y"]` |
| `nested: {y: 2}` | key `"true"` | quote keys: `{"y": 2}` |
| `version: 1.10` | `1.1` | `version: "1.10"` |
| `big: 12345678901234567` | `1.2345678901234568e+16` (float64) | quote and keep as string |
| `port: 8080` + `{{ .Values.port \| quote }}` | `"8080"` | fine; but `{{ .Values.port }}` in an env value renders an int — env values must be strings |

All rows were reproduced on 4.3.1 (and the same on 3.22.1). `.Values` integers are `float64` after JSON conversion; `int`/`int64` casts lose precision above 2^53.

***

#### `Chart.yaml`: Metadata, Strict Parsing, SemVer and Dependencies

```yaml
apiVersion: v2                 # v2 is the only chart API for real charts in Helm 4
name: web                      # lowercase letters, digits, dashes; equals the directory name
description: Example web application chart
type: application              # or "library"
version: 1.4.0                 # chart version, SemVer 2 — bump on every chart change
appVersion: "2.7.1"            # quote it; informational, used as default image tag
kubeVersion: ">=1.30.0-0"      # "-0" lets pre-release/vendor versions (1.33.2-eks-…) match
home: https://example.com/web
sources:
  - https://github.com/example/web
maintainers:
  - name: Platform Team
    email: platform@example.com
annotations:
  artifacthub.io/license: Apache-2.0
```

- **Unknown fields** are reported by Helm 4 lint as a warning: `[WARNING] Chart.yaml: failed to strictly parse chart metadata file ... json: unknown field "unknownField"`. It does not fail `helm lint` without `--strict`, and `helm template`/`package` still work, but typos such as `dependency:`, `appversion:` or `kubeversion:` are now visible. Treat this warning as an error in review: a misspelled key is a silently ignored setting.
- **`version` must be SemVer 2.** `version: 1.0` (YAML float) is an error in both majors (`version should be of type string but it's of type float64`); `version: "1.0"` passes Helm 3.22.1 silently but Helm 4 warns `version '1.0' is not a valid SemVerV2`, and `helm lint --strict` fails. OCI tags and `Chart.lock` resolution rely on valid SemVer.
- **`kubeVersion`** and pre-release/vendor suffixes: against a live cluster, Helm 3 and Helm 4 (verified on 4.0.5 and 4.3.1 with an apiserver reporting `v1.31.2-eks-7f9249a`) compare the full version, where `-eks-…` makes it a SemVer pre-release that `>=1.30.0` excludes: `Error: chart requires kubeVersion: >=1.30.0 which is incompatible with Kubernetes v1.31.2-eks-7f9249a`. Only an offline `--kube-version 1.31.2-eks-…` is normalized (4.1+), so `helm template` in CI passes while the real deploy fails. Write `>=1.30.0-0`. A mismatch fails install and render: `Error: chart requires kubeVersion: >=1.30.0-0 which is incompatible with Kubernetes v1.29.0`.
- In templates, `semverCompare` against `.Capabilities.KubeVersion.Version` **always** needs the `-0` suffix: on 4.3.1, `--kube-version 1.31.2-eks-7f9249a` yields `.Capabilities.KubeVersion.Version = "v1.31.2-eks-7f9249a"` and `semverCompare ">=1.30.0"` is `false` (4.0.5 and 3.22.1 strip the suffix from `--kube-version` and return `true`; against a real EKS/GKE apiserver the suffix is present in every version).
- `appVersion` must be quoted: unquoted `appVersion: 2.10` is a lint error (`appVersion should be of type string but it's of type float64`), yet `helm template`/`install` proceed with `appVersion: "2.1"` — and the default image tag becomes `2.1`.
- `icon` missing is an `[INFO]` lint message only.
- **Dependencies**: see "Dependencies, Subcharts and Library Charts". Commit `Chart.lock`; build with `helm dependency build` (uses the lock) in CI, and run `helm dependency update` only when intentionally changing versions. `helm template` with a missing dependency fails with `found in Chart.yaml, but missing in charts/ directory`.
- `apiVersion: v1` charts (Helm 2 format, `requirements.yaml`) still load, but new charts must use `v2`; `apiVersion: v3` is experimental and fails (see "Do Not Use").

***

#### Chart Layout and Naming

```text
web/
├── Chart.yaml
├── Chart.lock              # committed when dependencies exist
├── values.yaml             # documented defaults
├── values.schema.json      # validated on install/upgrade/template/lint
├── README.md               # values table (helm-docs or similar)
├── .helmignore             # keep CI files, tests data, .git out of the package
├── crds/                   # plain CRD YAML: installed once, never templated/upgraded
├── charts/                 # vendored/downloaded dependencies (usually gitignored, built from Chart.lock)
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

Rules from the chart best practices, still valid for Helm 4:[^23]

- **Chart names**: lowercase letters, digits and dashes (`my-app`); no uppercase, no underscores, no dots. The directory name equals `name`.
- **Template files**: dashed lowercase (`pod-disruption-budget.yaml`), `.yaml` for YAML output, `.tpl` for files without output. Helm 4 lint also lints `.yml` files.[^7] One resource kind per file; the file name reflects the kind.
- **Defined template names are global** across the parent and all subcharts: always prefix them with the chart name (`web.fullname`, not `fullname`). Two subcharts defining `fullname` silently override each other.
- **Values keys**: camelCase starting with a lowercase letter (`replicaCount`, `podSecurityContext`). Keys with dashes cannot be accessed with dot notation (`.Values.my-key` is a parse error; `index .Values "my-key"` works).
- **Resource names**: produced by `*.fullname`, at most 63 characters (DNS-1123 label), `trimSuffix "-"`. Objects that need a suffix (`-migrate`, `-headless`) must budget for it: `{{ include "web.fullname" . | trunc 55 | trimSuffix "-" }}-migrate`.
- `templates/` files starting with `_` produce no output; `NOTES.txt` is rendered and printed after install/upgrade.
- `.helmignore` excludes files from `helm package`; secrets, `.env`, CI files and large test fixtures must not end up in the published chart.

***

#### `_helpers.tpl`, Names and Labels

The helper set produced by `helm create` (identical in 3.22.1 and 4.3.1) is the baseline, plus `web.image` and `web.secretName`. Tested on 4.3.1:

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

- **Selector labels are immutable.** `spec.selector` of Deployments, StatefulSets and DaemonSets cannot change; putting `helm.sh/chart`, `app.kubernetes.io/version` or any value-driven label into `selectorLabels` breaks every upgrade that changes it (`field is immutable`). Selectors contain only `app.kubernetes.io/name` and `app.kubernetes.io/instance` (plus `component` for multi-workload charts).
- Use the Kubernetes recommended labels (`app.kubernetes.io/name`, `instance`, `version`, `component`, `part-of`, `managed-by`) on every object.[^24] `app.kubernetes.io/managed-by: {{ .Release.Service }}` renders `Helm`.
- The `helm create` scaffold puts the full `labels` (including `helm.sh/chart` and `app.kubernetes.io/version`) on the **pod template**. That makes every chart or app version bump roll all Pods even when the image did not change. It is a deliberate choice in the scaffold; accept it or use `selectorLabels` plus stable labels on the pod template.
- `include` returns a string and can be piped (`| nindent 4`); `template` cannot: `{{ template "x" . | nindent 4 }}` pipes the *argument* into `nindent` and fails at render time (`wrong type for value; expected string; got common.Values`). Always use `include`.
- Pass the right context: `include "web.fullname" .` at top level, `include "web.fullname" $` inside `range`/`with`. Helpers that need extra parameters take a dict: `include "web.componentName" (dict "root" $ "component" "worker")`.

***

#### Template Language: Whitespace, Scope, Functions and Pitfalls

Helm templates are Go `text/template` plus the Sprig v3.3.0 function library (without `env` and `expandenv`) plus Helm functions (`include`, `tpl`, `required`, `lookup`, `toYaml`, `fromYaml`, `fromYamlArray`, `toJson`, `fromJson`, `fromJsonArray`, `toToml`, `fromToml`, `toYamlPretty`, and the Helm 4 additions below).[^25][^26]

##### Helm 4 template functions

| Function | Since | Notes |
|---|---|---|
| `mustToYaml`, `mustToJson` | 4.0 | Like `toYaml`/`toJson` but fail rendering on marshal errors instead of returning `""` |
| `mustToToml` | 4.2 | Failing variant of `toToml` |
| `mustToDuration` | 4.3 | Parses Go durations (`"1m30s"`), plain numbers (seconds) and numeric strings; fails on invalid input |
| `durationSeconds`, `durationMilliseconds`, `durationMicroseconds`, `durationNanoseconds`, `durationMinutes`, `durationHours`, `durationDays`, `durationWeeks` | 4.3 | Return a number; **return 0 on invalid input** |
| `durationRoundTo`, `durationTruncateTo` | 4.3 | `durationRoundTo "1h29m" "1h"` → `1h0m0s` |

Verified on 4.3.1:

```yaml
# values: timeout: 1m30s, grace: 45
terminationGracePeriodSeconds: {{ durationSeconds .Values.grace }}     # 45
timeoutSeconds: {{ durationSeconds .Values.timeout }}                    # 90
minutes: {{ durationMinutes .Values.timeout | quote }}                   # "1.5"
dur: {{ mustToDuration .Values.grace | quote }}                          # "45s"
bad: {{ durationSeconds "nonsense" | quote }}                            # "0"  <- silent
```

Review: `durationSeconds` on a typo silently yields `0`; for fields where 0 is dangerous (timeouts, grace periods), validate with `mustToDuration` first or constrain the value in the schema. These functions make the chart Helm ≥ 4.3-only (`function "durationSeconds" not defined` on 4.0.5 and 3.22.1).

Use the `must*` variants where an empty string would produce a broken but syntactically valid manifest:

```yaml
# BAD: toJson swallows errors and renders an empty string
config.json: {{ .Values.config | toJson | quote }}
# GOOD (Helm 4 only): rendering fails loudly
config.json: {{ .Values.config | mustToJson | quote }}
```

##### Whitespace and indentation

- `{{-` trims whitespace and newlines before the action, `-}}` after. Use `{{-` on lines that contain only an action so they leave no blank line.
- `nindent N` emits a newline and indents every line by N; use it after `{{-` on its own line. `indent N` indents without the leading newline.
- N must equal the column where the block's keys belong (parent key indentation + 2 in this guide's style). A wrong N produces either invalid YAML (`helm template` error) or, worse, valid YAML with keys attached to the wrong parent.

```yaml
# GOOD
metadata:
  labels:
    {{- include "web.labels" . | nindent 4 }}
spec:
  template:
    spec:
      containers:
        - name: app
          {{- with .Values.resources }}
          resources:
            {{- toYaml . | nindent 12 }}
          {{- end }}
```

- For embedded file contents, use a `|` block scalar plus `nindent`:

```yaml
data:
  feature-flags.json: |
    {{- .Values.config.featureFlags | mustToJson | nindent 4 }}
```

- Since 4.2.4 the `helm template` output keeps blank lines that a template leaves at the end of a document (for example after `{{- end }}`); 4.0.5 and 3.22.1 strip them (verified on 4.3.1 vs 4.0.5/3.22.1). The YAML is equivalent, but golden-file tests comparing raw `helm template` output must be regenerated when moving between these versions.

##### Scope: `with`, `range` and `$`

`with` and `range` rebind `.`. Inside them, use `$` for the root context:

```yaml
# BAD: .Release is looked up on the map value -> error
data:
  {{- range $k, $v := .Values.env }}
  {{ $k }}: {{ $v | quote }}
  ns: {{ .Release.Namespace }}
  {{- end }}
# Error: tp/templates/cm.yaml:8:17
#   executing "tp/templates/cm.yaml" at <.Release.Namespace>:
#     can't evaluate field Release in type interface {}

# GOOD
  ns: {{ $.Release.Namespace }}
```

Helm 4 prints execution errors from the template engine itself (field evaluation, wrong argument types, errors inside `include`) on several lines — `file:line:col`, then `executing ... at <...>:`, then the cause, repeated for every `include` level — where Helm 3 printed one line starting with `Error: template:`. Errors returned by a function keep the one-line Helm 3 format in Helm 4 (`Error: template: ...: error calling dig: ...`), and `required`/`fail` (`execution error at (...)`) and parse errors (`parse error at (...)`) are unchanged. CI scripts that grep for `Error: template:` miss part of the Helm 4 failures; match on the exit code instead.

##### `default` treats false and 0 as empty

```yaml
# values.yaml: enabled: false, replicas: 0
badDefaultBool: {{ .Values.enabled | default true }}    # true  <- user's false is ignored
badDefaultInt:  {{ .Values.replicas | default 3 }}      # 3     <- user's 0 is ignored

# GOOD: distinguish "unset" from "false/0"
enabled: {{ hasKey .Values "enabled" | ternary .Values.enabled true }}
replicas: {{ if kindIs "invalid" .Values.replicas }}3{{ else }}{{ .Values.replicas }}{{ end }}
```

Better still: put the default in `values.yaml` and avoid `default` for booleans and numbers entirely.

##### `required` and `fail`

```yaml
host: {{ required "ingress.host is required when ingress.enabled=true" .Values.ingress.host }}
```

`required` fails on `nil` **and** on the empty string (`Error: execution error at (pf/templates/cm.yaml:14:11): host is required`). It does not fail on `false` or `0`. For cross-field rules use `fail`:

```yaml
{{- if and .Values.podDisruptionBudget.minAvailable .Values.podDisruptionBudget.maxUnavailable }}
{{- fail "set either podDisruptionBudget.minAvailable or maxUnavailable, not both" }}
{{- end }}
```

Prefer `values.schema.json` for type/enum/range checks: it reports all violations at once and is visible to tooling.

##### `quote`, `toString` and types

- Quote every free-form string value: `{{ .Values.someString | quote }}`. Unquoted values like `yes`, `on`, `1e3` or `0777` change type when the rendered manifest is parsed.
- Environment variable values, annotation values and label values must be strings in Kubernetes. `value: {{ .Values.port }}` renders `value: 8080` and the install fails — on 4.3.1 with `.spec.containers[name="c"].env[name="PORT"].value: expected string, got &value.valueUnstructured{Value:8080}`, on 3.22.1 with `json: cannot unmarshal number into Go struct field EnvVar...value of type string`. Write `value: {{ .Values.port | quote }}`.
- Do not `quote` integers that Kubernetes expects as numbers (`containerPort`, `replicas`): `containerPort: "8080"` fails with `containerPort: expected numeric (int or float), got string` (4.3.1). Neither error is caught by `--dry-run=server`.
- `.Values` maps are of type `common.Values` (Helm 4) / `chartutil.Values` (Helm 3), not `map[string]interface{}`. Sprig's `dig` therefore fails on `.Values` directly (`error calling dig: interface conversion: interface {} is common.Values, not map[string]interface {}`); use `.Values.AsMap`:

```yaml
timeout: {{ dig "server" "timeout" "30s" .Values.AsMap | quote }}
```

##### `tpl`

`tpl` renders a string as a template, letting users put template expressions into values:

```yaml
# values.yaml
tplValue: "{{ .Release.Name }}-svc"
# template
service: {{ tpl .Values.tplValue . | quote }}     # "rel-svc"
```

Use it for values documented as templatable (hostnames, affinity blocks: `{{- tpl (toYaml .) $ | nindent 8 }}`). Never pass untrusted input to `tpl`: it can call `lookup` (cluster reads during install), `include` any helper, and `fail`.

##### Random and time functions

`randAlphaNum`, `uuidv4`, `now`, `date` produce a different value on every render. Used directly in a Secret, ConfigMap or annotation, they change the object on **every** upgrade (rotating a password, restarting Pods). Use them only together with a `lookup` that preserves an existing value (see "Configuration and Secrets"), and remember `lookup` is empty in `helm template`/GitOps renders.

##### `lookup`

```yaml
{{- $existing := lookup "v1" "Secret" .Release.Namespace $name }}
```

- Returns an empty map when the object does not exist **or** when there is no cluster (`helm template`, `--dry-run=client`, `helm lint`).
- An empty name (`lookup "v1" "Secret" .Release.Namespace ""`) returns a list of every object of that kind in the namespace; it needs list RBAC and can be large (all Secrets!). Prefer a specific name.
- Results reflect the cluster at render time; a chart must still produce a valid manifest when `lookup` is empty.

##### Network access during render

`getHostByName` is disabled unless `--enable-dns` is passed; a chart that needs DNS at render time is not reproducible. Flag `--enable-dns` in pipelines.

***

#### Values Design

The `values.yaml` of the example chart (rendered and installed on 4.3.1 against a v1.36 apiserver with Pod Security Admission `restricted`, no warnings):

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
- **Secure defaults**: non-root, `readOnlyRootFilesystem`, all capabilities dropped, `seccompProfile: RuntimeDefault`, service account token not mounted unless needed. A user should have to opt *out* of security, not in.
- **Prefer maps over lists for things users override.** `--set` and values-file merging replace lists wholesale: overriding one entry of `env: [...]` requires repeating the whole list. A map (`env: {LOG_LEVEL: info}`) merges key by key and avoids duplicate-name SSA failures.
- **Flat where possible, nested where grouped**: `image.repository`, `service.port`; avoid deep nesting that makes `--set` paths long.
- **Every optional feature has an explicit `enabled` flag** with a safe default; templates gate on it.
- **No secrets in `values.yaml` defaults.** Support `existingSecret` references; never ship a default password.
- **Do not mirror whole Kubernetes specs as free-form values** without reason; when you do (`affinity`, `tolerations`, `resources`), pass them through `toYaml` unmodified.
- **`global`** is shared with every subchart; use it only for values that genuinely apply to all (registry, pull secrets).
- Defaults must render a working install with `helm install x ./chart` and nothing else (or fail early with `required`).

***

#### `values.schema.json`

Helm validates merged values against `values.schema.json` on `install`, `upgrade`, `template` and `lint` (skip with `--skip-schema-validation`, available on 3.22.1 and 4.x). Each subchart's schema validates that subchart's values. Helm 3.22.1 and Helm 4 use the same validator (`santhosh-tekuri/jsonschema/v6`) and both accept JSON Schema draft 2020-12 (`prefixItems`, `unevaluatedProperties`), draft-07 and earlier. Both resolve `$ref` to `http(s)://` URLs at validation time — which makes rendering depend on network access, so vendor referenced schemas into the chart instead; unresolvable URN `$ref`s produce a warning and are ignored (since 4.0.2).

The example chart's schema (tested on 4.3.1):

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
$ helm upgrade r ./web -n web --set config.logLevel=verbose
web:
- at '/config/logLevel': value must be one of 'debug', 'info', 'warn', 'error'
$ helm upgrade r ./web -n web --set podDisruptionBudget.minAvailable=1
web:
- at '/podDisruptionBudget': 'not' failed
```

Review points:

- `"additionalProperties": false` at the **root** rejects keys for subcharts and `global` unless they are listed (`additional properties 'typo' not allowed`). Either list every subchart key and `global`, or restrict `additionalProperties: false` to nested objects you own. It is still the best defence against typos (`replicaCont`), so use it on nested objects.
- Numbers from `--set` are integers when they look like integers; from YAML they are numbers. `"type": "integer"` accepts `3` and `3.0` but rejects `"3"`; `--set-string` values are always strings.
- Allow both forms where Kubernetes does (`targetPort`: integer or named port).
- Validation runs on merged values, so required keys present in `values.yaml` always pass; schema `required` is for keys without defaults.
- The schema does not see values computed in templates; keep cross-field logic small (`not`/`if`-`then`) and readable.

***

#### Workload Manifests: Security, Resources, Probes, Availability

The example Deployment (rendered on 4.3.1, installed against a v1.36 apiserver in a namespace labelled `pod-security.kubernetes.io/enforce=restricted` without warnings):

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

##### Security context

- Default to the Pod Security Standards **restricted** profile:[^27] `runAsNonRoot: true`, a numeric non-zero `runAsUser`, `allowPrivilegeEscalation: false`, `capabilities.drop: ["ALL"]`, `seccompProfile.type: RuntimeDefault`. Add `readOnlyRootFilesystem: true` and an `emptyDir` for `/tmp`.
- Flag `privileged: true`, `hostNetwork`, `hostPID`, `hostIPC`, `hostPath` volumes, added capabilities (`NET_ADMIN`, `SYS_ADMIN`) and `runAsUser: 0` unless the chart's purpose requires them (CNI, node agents) and they are behind explicit values.
- `automountServiceAccountToken: false` unless the app talks to the Kubernetes API; when it does, ship a least-privilege `Role`/`RoleBinding` behind `rbac.create`, never a `ClusterRole` with `*` verbs.
- Verify locally with a namespace labelled `pod-security.kubernetes.io/enforce=restricted` and a real install or `kubectl apply --dry-run=server` of the rendered output: Pod Security Admission warns on Deployments and rejects Pods.

##### Resources

- Always set **requests** (CPU and memory). Set a **memory limit**; a CPU limit is a policy choice (CPU throttling hurts latency-sensitive services) — many platforms set requests only for CPU.
- Defaults must be realistic for the app; `resources: {}` as default (the `helm create` scaffold) is acceptable only for example charts.
- Pass `resources` through `toYaml` unchanged; do not build it from separate `cpu`/`memory` values unless needed.

##### Probes

- Readiness gates traffic, liveness restarts, startup protects slow starts. A liveness probe that depends on downstream services (database) causes cascading restarts — liveness must check only the process itself.
- Use the named port (`port: http`) so changing `targetPort` does not break probes.
- `startupProbe` instead of a long `initialDelaySeconds` on liveness.

##### Availability

- **PodDisruptionBudget** for workloads with more than one replica; `minAvailable` **or** `maxUnavailable` (not both — the API rejects it; validate in the schema). A PDB with `minAvailable` equal to the replica count blocks node drains.
- **HPA**: `autoscaling/v2`; omit `spec.replicas` when enabled (mandatory under Helm 4 SSA, see section 5).
- `topologySpreadConstraints` or pod anti-affinity for zone/node spreading; `revisionHistoryLimit` to bound old ReplicaSets.
- `strategy.rollingUpdate.maxUnavailable: 0` with `maxSurge: 1` for zero-downtime services that have readiness probes.
- `terminationGracePeriodSeconds` larger than the app's shutdown time, and a `preStop` sleep when load balancers need time to deregister.

##### Networking

- Gate `Ingress`, Gateway API `HTTPRoute` (`gateway.networking.k8s.io/v1`, in the 3.22/4.x `helm create` scaffold) and `NetworkPolicy` behind values. Use `ingressClassName`, not the deprecated `kubernetes.io/ingress.class` annotation.
- A `NetworkPolicy` that selects the Pods must allow the traffic the chart needs (probes come from the kubelet and are not subject to NetworkPolicy; metrics scrapers and ingress controllers are).

***

#### Configuration and Secrets

##### Restart Pods when configuration changes

```yaml
annotations:
  checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
```

A ConfigMap or Secret consumed via `envFrom`/`env` is read only at container start. The checksum annotation changes the pod template whenever the rendered ConfigMap changes, triggering a rollout. Do not checksum a Secret that is generated randomly on every render (it would roll Pods on every upgrade).

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

Verified on 4.3.1: the password is unchanged across `helm upgrade`. Limitations reviewers must know:

- Under `helm template` (Argo CD, Flux with `helm template` semantics) `lookup` is empty, so the password changes on every sync. For GitOps, require `existingSecret` (External Secrets Operator, Sealed Secrets, SOPS) instead.
- `helm.sh/resource-policy: keep` leaves the Secret behind on uninstall with its ownership annotations intact. Verified on 4.3.1: reinstalling the **same release name in the same namespace** adopts it and the `lookup` keeps the old password; a release with a different name fails (`exists and cannot be imported into the current release: invalid ownership metadata`) unless it uses `--take-ownership` (since 3.17, also in 4.x).
- Never put secrets into `values.yaml`, `NOTES.txt` output, ConfigMaps, annotations, or labels. Release Secrets (`sh.helm.release.v1.<name>.v<rev>`) contain the full values and rendered manifests (gzip+base64), so anyone who can read Secrets in the release namespace can read every value passed to Helm. Prefer references to externally managed Secrets.

##### ConfigMaps

- Use `| quote` for scalar values and `|` block scalars with `nindent` for files.
- ConfigMaps larger than 1 MiB are rejected; very large embedded files belong in an image or volume.
- `.Files.Get "files/app.conf"` and `.Files.Glob "files/*.conf" | .AsConfig` read files packaged with the chart; files listed in `.helmignore` and files under `templates/` are not available through `.Files`.

***

#### Hooks

```yaml
{{- if .Values.migrations.enabled }}
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ include "web.fullname" . }}-migrate
  labels:
    {{- include "web.labels" . | nindent 4 }}
  annotations:
    # post-install, not pre-install: on the first install the Secret and ServiceAccount
    # below are regular chart resources and do not exist before the pre-install phase
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

Hook types: `pre-install`, `post-install`, `pre-upgrade`, `post-upgrade`, `pre-rollback`, `post-rollback`, `pre-delete`, `post-delete`, `test`.[^28]

Review points:

- **Hook resources are not part of the release.** They are not upgraded in place, not deleted on uninstall (verified on 4.3.1: a `pre-install` ConfigMap hook remains after `helm uninstall`), and not listed by `helm get manifest`. Always set `helm.sh/hook-delete-policy`. Without the annotation, Helm applies `before-hook-creation` (delete the previous instance before creating a new one).
- `hook-delete-policy: hook-succeeded` removes the evidence of success; keep `before-hook-creation` alone if you need logs, or add `"helm.sh/hook-output-log-policy": hook-failed` (or `hook-succeeded`) so Helm prints the hook Pod/Job container logs.
- **Ordering**: hooks run by ascending `helm.sh/hook-weight` (a string: `"-5"`), then by resource name (the source sorts by name; the docs also mention kind). A `pre-install` hook runs **before** any regular resources exist: a migration Job that mounts a Secret or ServiceAccount from `templates/` fails because they are not created yet. Either make those dependencies hooks with a lower weight, or create them outside the chart.
- **Helm waits for hook Jobs/Pods** (the default `hookOnly` strategy). A hook that never completes blocks the release until `--timeout` (default 5m). Set `backoffLimit` and `activeDeadlineSeconds`.
- Under Helm 4, **post-renderers also modify hooks** (see section 9).
- Hooks are applied with server-side apply in Helm 4 like regular resources (manager `helm`, operation `Apply`).
- `helm install --no-hooks` / `helm upgrade --no-hooks` skip hooks; flag it in CI unless intended.
- Prefer idempotent migrations that tolerate concurrent runs; a failed `pre-upgrade` aborts the upgrade, and with `--rollback-on-failure` triggers a rollback (which runs `pre-rollback`/`post-rollback` hooks, not the migration in reverse).
- CRDs must not be hooks; use `crds/` or a separate chart.

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

- Run with `helm test <release> --logs --timeout 2m`. Since 4.0.5/4.1.0, `--logs` works together with `hook-succeeded`/`hook-failed` delete policies (logs are collected before deletion); since 4.2.4 logs of all containers in test Pods are shown, and since 4.3.0 non-Pod test hooks are skipped when fetching logs.
- Test Pods must satisfy the namespace's Pod Security level too (the scaffold's `busybox` test with no securityContext is rejected in `restricted` namespaces).
- Pin test images (`busybox:1.37`, better a digest); `latest` makes tests non-reproducible.
- Tests should check behaviour (HTTP 200 on a readiness endpoint), not only TCP reachability.
- `helm test --filter name=…` selects tests.

***

#### CRDs

- Files in `crds/` are plain YAML (not templated), installed before the templates on **first install** only, never upgraded and never deleted by Helm (verified on 4.3.1: changing `crds/` and upgrading leaves the CRD unchanged; uninstall keeps it).[^29] `--skip-crds` skips them.
- CRDs that must evolve with the chart belong in a separate chart (templated, upgraded deliberately) or are managed by the operator itself. Deleting a CRD deletes every custom resource of that kind cluster-wide — never template CRDs in a chart that users uninstall casually.
- A chart that ships CRs for a CRD it installs in `templates/` cannot render them in the same release reliably: the resource mapping is built before the CRD exists (`no matches for kind ... ensure CRDs are installed first`). Use `crds/`, a separate chart, or `.Capabilities.APIVersions.Has` gates.
- Since 4.3.1, the hook-only wait also waits for CRDs to be `Established`.

***

#### Dependencies, Subcharts and Library Charts

```yaml
# Chart.yaml
dependencies:
  - name: postgresql
    version: "16.4.1"                        # exact pin (house rule); ranges are resolved into Chart.lock
    repository: oci://registry-1.docker.io/bitnamicharts
    condition: postgresql.enabled
  - name: common
    version: "~1.0.0"
    repository: "file://../lib"
```

- **Commit `Chart.lock`**; CI runs `helm dependency build` (honours the lock), not `update`. Helm 4 builds dependencies concurrently with atomic file writes.
- Prefer exact versions (house rule for reproducibility); when ranges are used, review `Chart.lock` diffs on every update.
- `condition: <sub>.enabled` and `tags:` toggle subcharts; `alias:` installs the same chart twice under different names.
- Parent values for a subchart live under its name (or alias): `postgresql.auth.database`. `global` is visible to all. `import-values` copies child values up into the parent.
- Subchart templates are rendered with their own `.Chart` and `.Values`, but **defined template names are global**: prefix every `define`.
- **Library charts** (`type: library`) contain only `define`s and cannot be installed (`Error: library charts are not installable`). Templates in the parent call them with the parent's context:

```yaml
# lib/templates/_labels.tpl  (chart "common", type: library)
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

  Verified on 4.3.1: `helm dependency update ./parent` writes `Chart.lock` with the resolved `1.0.0` and a `digest:`; rendering yields `app.kubernetes.io/name: parent`.
- Vendored subcharts in `charts/` as directories bypass `Chart.lock` verification; prefer packaged dependencies resolved from the lock.

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

- Print commands, not secret values: `NOTES.txt` output is stored in the release and shown by `helm status`/`helm get notes`.
- `helm template` never prints notes (the `--hide-notes`/`--render-subchart-notes` flags are no-ops there since 4.2); `helm install --hide-notes` suppresses them on install.

***

#### Packaging, Publishing and Signing

```bash
set -euo pipefail
export SOURCE_DATE_EPOCH="$(git log -1 --format=%ct)"   # reproducible archive (4.3+)

helm dependency build ./charts/web
helm lint ./charts/web --strict -f ci/values/prod.yaml
helm package ./charts/web -d dist/
echo "$REGISTRY_TOKEN" | helm registry login ghcr.io -u "$REGISTRY_USER" --password-stdin
helm push dist/web-1.4.0.tgz oci://ghcr.io/acme/charts     # prints "Digest: sha256:..."
```

- **Bump `version` on every chart change.** Republishing the same version with different content breaks caches and digest pins; OCI registries may also reject or silently overwrite it.
- Publish to OCI registries (GA since Helm 3.8). Classic `index.yaml` repositories still work (`helm repo add/update`); `helm repo update` in Helm 4 fails if any configured repository fails, so CI should only configure the repositories it needs.
- Sign: `.prov` via `helm package --sign` (GnuPG keyring) or Sigstore/cosign on the OCI artifact; verify in the deploy path (`helm install --verify`, `cosign verify`).
- Record the pushed digest and deploy by digest (section 10).
- `helm push` requires a packaged `.tgz` (`cannot push directory, must provide chart archive (.tgz)`).

***

#### CI Pipelines

A minimal GitHub Actions job for a Helm 4 chart repository:

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
          version: v4.3.1            # pin: omitting it means "latest"
      - name: Lint and render
        run: |
          set -euo pipefail
          helm dependency build charts/web
          helm lint charts/web --strict -f ci/values/prod.yaml
          helm template web charts/web -n web \
            -f ci/values/prod.yaml \
            --kube-version 1.36.0 \
            --api-versions gateway.networking.k8s.io/v1 > rendered.yaml
          # validate rendered.yaml against the target cluster's schemas here (kubeconform or kubectl --dry-run=server)
```

Review points for pipelines:

- **Pin Helm exactly** (`v4.3.1`, image `alpine/helm:4.3.1`) and pin actions to a commit SHA in security-sensitive repositories.
- `helm lint` needs a path in Helm 4; use `--strict` so warnings (non-SemVer version, unknown `Chart.yaml` fields) fail the build. Lint with each production values file, not only defaults.
- Render with `--kube-version` matching the target cluster; the offline default (v1.34.0 on 4.0.5, v1.37.0 on 4.3.1) silently changes when Helm is upgraded.
- `helm template` output is not validated; `helm install --dry-run=server` validates kinds but not fields (section 8). Validate rendered manifests separately.
- Deploy with explicit values, explicit namespace, `--rollback-on-failure` (not `--atomic`), an explicit `--wait` strategy and `--timeout`:

```bash
helm upgrade --install web oci://ghcr.io/acme/charts/web@sha256:<digest> \
  -n web --create-namespace \
  -f values/prod.yaml \
  --rollback-on-failure --wait=watcher --timeout 10m
```

- Flag in deploy scripts: `--force-conflicts` or `--force-replace` as defaults, `--reuse-values`, missing `-f` (implicit value reuse), `--no-hooks`, `--skip-schema-validation`, `--disable-openapi-validation`, `--insecure-skip-tls-verify`, `--plain-http`, `--enable-dns`, `--post-renderer` pointing to a path, `helm registry login` with `-p`/`--password` (token in process list and shell history), `helm list -a`.
- Run `helm` 3 and 4 in parallel only with separate config/data directories (`HELM_CONFIG_HOME`, `HELM_DATA_HOME`, `HELM_CACHE_HOME`); Helm 4 v1 plugins break Helm 3's plugin loading.
- Release Secrets are stored in the release namespace (`HELM_DRIVER=secret` default); `--history-max` (default 10) bounds them. Do not set `0` (unlimited) on frequently deployed releases.

***

#### Logging, Debugging and Output Formats

- Helm 4 logs through `log/slog`: warnings look like `level=WARN msg="upgrade failed" name=r4 error="..."`; `--debug` (or `HELM_DEBUG=1`) adds `level=DEBUG` lines on stderr (`level=DEBUG msg="Chart path" path=...`). Scripts must parse stdout (`-o json`/`-o yaml`), never log lines.
- Template errors are multi-line in Helm 4 (section 15); `helm template --debug` still renders invalid YAML for inspection.
- Colour: `--color never|auto|always` (alias `--colour`), `HELM_COLOR`, and `NO_COLOR` (overrides). Set `NO_COLOR=1` in CI when logs are parsed.
- `helm status` always includes the resource list and description (the Helm 3 `--show-resources`/`--show-desc` flags are gone); use `helm status -o json` for scripts.
- Registry client messages (`Pulled: …`, `Digest: …`) go to stdout since 4.2.1 (4.0.5 writes them to stderr); since 4.3.0 `helm template`/`helm show` send them to stderr again so they do not corrupt rendered YAML (verified on 4.3.1).
- `helm version` prints `KubeClientVersion` in addition to the Helm and Go versions; `helm version --short` remains.

***

#### Removed / Changed in Helm 4 — Migration Notes

| Area | Helm 3 behaviour | Helm 4 behaviour | Action |
|---|---|---|---|
| Apply | Client-side 3-way merge | SSA for new releases; existing releases keep CSA until `--server-side=true` | Switch existing releases deliberately; expect field-ownership conflicts where other controllers write |
| Drift | Overwritten silently | Upgrade fails on fields owned by other managers | Stop rendering controller-owned fields (`replicas` with HPA); use `--force-conflicts` once, not permanently |
| Unknown/misspelled fields | Warning, field dropped | Install/upgrade fails (`field not declared in schema`) | Fix the manifests |
| Duplicate objects in a release | Install fails (`already exists`) | Applied twice, last write wins, no error | Review names for collisions |
| Keyed-list duplicates (`env` names) | Warning | Fails (`duplicate entries for key`) | Deduplicate |
| `--atomic`, `--force` | Supported | Deprecated aliases (`install --atomic` missing before 4.1.3) | `--rollback-on-failure`, `--force-replace` |
| `--wait` | Boolean, Helm 3 readiness rules | Strategies; `watcher` waits for CRs too | Choose `watcher`/`legacy`; set `--timeout` |
| Post-renderer | Executable path; hooks not post-rendered | Plugin name; hooks post-rendered | Package the renderer as `postrenderer/v1` plugin; check its selectors |
| Plugins | Legacy `plugin.yaml` | v1 format, WASM runtime, signed tarballs | Legacy still loads; reinstall with `.prov` or `--verify=false` (review) |
| Registry login | Host or path | Host only | Drop the path |
| `helm list` | Deployed+failed; `-a` for all | All statuses; `-a` removed | Filter with `--deployed` etc. |
| `helm lint` | Defaults to `.` | Path required; SemVer and strict `Chart.yaml` warnings | `helm lint .` |
| Values files | First YAML document only | All documents merged | Remove stray `---` from values files shared with Helm 3 |
| `--set-json` | `key=json` | Also whole JSON object | — |
| `helm template --validate` | Validates against cluster | Deprecated | `--dry-run=server` |
| Removed flags | — | `version --client`, `status --show-desc/--show-resources`, `repo add --no-update`, `repo update --fail-on-repo-update-fail`, `rollback --recreate-pods` | Remove from scripts |
| Offline `.Capabilities.KubeVersion` | Built-in default | Tracks compiled client-go (v1.34 on 4.0, v1.37 on 4.3) | Pass `--kube-version` |
| Template error format | One line `Error: template: …` | Multi-line for engine execution errors; function errors still one line | Match on exit codes, not on message text |
| Uninstall | Deletes all manifest resources | Skips resources owned by other releases (4.3) | — |
| Chart API | v2 | v2 (v3 experimental, not usable) | Keep `apiVersion: v2` |
| Kubernetes support | 3.22: n-3 of client-go 1.37 | 4.x: n-3 of its client-go (4.3: 1.34–1.37) | Upgrade Helm minor with the cluster |

Helm 4 manages Helm 3 releases in place (same release Secret format); there is no `helm 2to3`-style conversion. Mixing majors on one release is possible but changes its apply method: verified on a v1.36 apiserver, a Helm 3.22.1 upgrade of an SSA release created by Helm 4.3.1 succeeds with client-side apply (the object then has both `helm Apply` and `helm Update` managed-field entries), the new revision carries no `applyMethod`, and the next Helm 4 upgrade with `--server-side=auto` treats the release as `csa`. Keep one major per release and re-enable SSA with `--server-side=true` after any Helm 3 intervention.

***

#### Production / Review Checklist

##### Helm 4 specifics

- [ ] Helm pinned to an exact 4.x version in every CI job and container image
- [ ] No Helm 4-minor feature used beyond the pinned minor (`mustToToml` 4.2, `duration*` 4.3, `--wait=hookOnly` 4.1, `install --atomic` 4.1.3, tag@digest 4.1.3, `rollback --description` 4.3, `SOURCE_DATE_EPOCH` 4.3)
- [ ] Helm ≥ 4.1.4 wherever plugins are installed (CVE-2026-35205/35204)
- [ ] Charts consumed by Helm 3 users avoid Helm 4-only functions and multi-document `values.yaml`
- [ ] `--rollback-on-failure`/`--force-replace` instead of `--atomic`/`--force`
- [ ] Explicit `--wait` strategy and `--timeout`; custom resources in the chart have operators that set Ready conditions
- [ ] No rendered fields owned by other controllers (`replicas` with HPA/KEDA, injected `caBundle`, sidecars); `--force-conflicts` not used by default
- [ ] No duplicate resource names across templates/subcharts; no duplicate `env` names
- [ ] Post-renderers are `postrenderer/v1` plugins, installed and pinned in CI; overlays expect hooks in their input
- [ ] `helm registry login <host>` with `--password-stdin`; no `-p`
- [ ] No removed flags (`list -a`, `version --client`, `status --show-resources`, `repo add --no-update`, `repo update --fail-on-repo-update-fail`, `rollback --recreate-pods`, `lint` without path)
- [ ] `--kube-version` (and `--api-versions` for CRD APIs) passed to `helm template` in CI
- [ ] Rendered manifests validated against cluster schemas (`--dry-run=server` alone does not validate fields)
- [ ] `apiVersion: v2` in `Chart.yaml`; no `apiVersion: v3`

##### Chart metadata and structure

- [ ] SemVer 2 `version` (quoted), bumped on every change; quoted `appVersion`
- [ ] `kubeVersion` with `-0` suffix; `semverCompare` constraints with `-0`
- [ ] No unknown `Chart.yaml` fields (`helm lint --strict` clean)
- [ ] `Chart.lock` committed; dependencies pinned; CI uses `helm dependency build`
- [ ] Chart, template and value names follow conventions; every `define` prefixed with the chart name
- [ ] `.helmignore` excludes secrets, CI and test data

##### Templates

- [ ] `include` (not `template`) with `nindent`; correct indentation verified by rendering
- [ ] `$` used inside `range`/`with` for root context
- [ ] `default` not used for booleans/zero values; `required`/`fail`/schema for mandatory and cross-field rules
- [ ] Strings quoted; numbers that the API expects as integers not quoted; env/annotation/label values are strings
- [ ] Names truncated to 63 characters including suffixes
- [ ] Selector labels limited to name/instance (immutable)
- [ ] No `generateName`; no random/time functions without `lookup`-based preservation; output valid when `lookup` is empty
- [ ] `tpl` only on documented, trusted values; no `--enable-dns` dependency
- [ ] `must*` variants where silent empty output would be harmful (Helm 4-only charts)

##### Values and schema

- [ ] Every value documented; secure defaults; no secrets in defaults; `existingSecret` supported
- [ ] Maps preferred over lists for user-overridable collections
- [ ] YAML 1.1 traps quoted (`"NO"`, `"y"`, `"1.10"`, large integers)
- [ ] `values.schema.json` with types, enums, ranges; `additionalProperties: false` on owned nested objects; root allows subchart keys and `global`

##### Workloads and security

- [ ] Restricted Pod Security profile: non-root, no privilege escalation, drop ALL, RuntimeDefault seccomp, read-only root filesystem
- [ ] `automountServiceAccountToken: false` unless needed; least-privilege RBAC behind a value
- [ ] Requests set; memory limit set; probes on named ports; liveness independent of dependencies
- [ ] PDB (one of minAvailable/maxUnavailable), spread constraints, HPA with `replicas` omitted
- [ ] Images pinned by tag and preferably digest; `imagePullPolicy` sensible
- [ ] Config checksum annotations for env-consumed ConfigMaps

##### Hooks, tests, CRDs

- [ ] Every hook has `hook-delete-policy`, weights as strings, bounded Jobs (`backoffLimit`, `activeDeadlineSeconds`)
- [ ] `pre-install` hooks do not depend on regular chart resources
- [ ] Test Pods pass the namespace's Pod Security level; pinned images; `helm test --logs` in CI
- [ ] CRDs in `crds/` (or a separate chart), never hooks; CR gating with `.Capabilities`

##### Supply chain

- [ ] Charts pushed to OCI; deployments reference a digest
- [ ] Charts signed (`.prov` or cosign) and verified on deploy; plugin tarballs verified (`--verify` not disabled)
- [ ] Reproducible packages (`SOURCE_DATE_EPOCH` on 4.3+)
- [ ] Registry credentials short-lived and not persisted on runners; release Secrets treated as sensitive

***

[^1]: Helm v4.0.0 release (2025-11-12). https://github.com/helm/helm/releases/tag/v4.0.0
[^2]: Helm 4 Released (Helm blog, Helm 3 support dates as originally announced). https://helm.sh/blog/helm-4-released
[^3]: Helm v4.3.1 release notes (2026-10-15; next releases 4.3.2/3.22.2 on 2026-11-11 and 4.4.0 on 2027-01-13). https://github.com/helm/helm/releases/tag/v4.3.1
[^4]: Helm releases list (4.0.x–4.3.x and 3.20.x–3.22.x dates). https://github.com/helm/helm/releases
[^5]: Helm v3 End of Life (Helm blog, 2026-06-02: last minor 3.22 on 2026-09-09, security fixes until 2027-02-10). https://helm.sh/blog/helm-v3-end-of-life
[^6]: Helm Version Support Policy (version skew table). https://helm.sh/docs/topics/version_skew/
[^7]: Helm 4 Full Changelog. https://helm.sh/docs/changelog/
[^8]: Helm 4 Overview (new features, breaking changes, renamed flags, charts v3). https://helm.sh/docs/overview/
[^9]: Helm v4.1.0 release notes. https://github.com/helm/helm/releases/tag/v4.1.0
[^10]: helm/helm PR #31901 — restore deprecated `--atomic` flag on install (milestone 4.1.3). https://github.com/helm/helm/pull/31901
[^11]: helm/helm PR #31601 — handle digest algorithm prefix for tag@digest OCI references (milestone 4.1.3, backported to 3.20.1). https://github.com/helm/helm/pull/31601
[^12]: helm/helm PR #31644 — preserve nil values when chart default is an empty map (milestone 4.1.3). https://github.com/helm/helm/pull/31644
[^13]: Helm v4.2.0 release notes. https://github.com/helm/helm/releases/tag/v4.2.0
[^14]: Helm v4.3.0 release notes. https://github.com/helm/helm/releases/tag/v4.3.0
[^15]: HIP-0020: Charts v3 Enablement. https://github.com/helm/community/blob/main/hips/hip-0020.md
[^16]: azure/setup-helm (version input, default "latest"). https://github.com/Azure/setup-helm
[^17]: HIP-0023: Utilize Server Side Apply for installs/upgrades. https://github.com/helm/community/blob/main/hips/hip-0023.md
[^18]: Kubernetes documentation — Server-Side Apply (field management, conflicts). https://kubernetes.io/docs/reference/using-api/server-side-apply/
[^19]: HIP-0022: Wait With kstatus. https://github.com/helm/community/blob/main/hips/hip-0022.md
[^20]: kstatus (status computation rules used by Helm's watcher). https://github.com/kubernetes-sigs/cli-utils/blob/master/pkg/kstatus/README.md
[^21]: HIP-0026: Wasm plugin system; Helm plugins documentation. https://github.com/helm/community/blob/main/hips/hip-0026.md , https://helm.sh/docs/plugins/
[^22]: YAML 1.1 boolean type (y/n/yes/no/on/off). https://yaml.org/type/bool.html
[^23]: Helm Chart Best Practices Guide. https://helm.sh/docs/chart_best_practices/
[^24]: Kubernetes recommended labels. https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
[^25]: Helm Template Function List. https://helm.sh/docs/chart_template_guide/function_list/
[^26]: Helm v4.3.1 source — pkg/engine/funcs.go (function map incl. must*/duration helpers). https://github.com/helm/helm/blob/v4.3.1/pkg/engine/funcs.go
[^27]: Kubernetes Pod Security Standards. https://kubernetes.io/docs/concepts/security/pod-security-standards/
[^28]: Helm Chart Hooks. https://helm.sh/docs/topics/charts_hooks/
[^29]: Helm Charts topic (Chart.yaml fields, dependencies, crds/, schema, library charts). https://helm.sh/docs/topics/charts/
[^30]: Helm v4.1.3 release notes ("there was no 4.1.2 release due to a release automation issue"). https://github.com/helm/helm/releases/tag/v4.1.3
[^31]: Helm v4.1.4 release and security advisories GHSA-q5jf-9vfq-h4h7 (CVE-2026-35205), GHSA-vmx8-mqv2-9gmg (CVE-2026-35204), GHSA-hr2v-4r36-88hr (CVE-2026-35206). https://github.com/helm/helm/releases/tag/v4.1.4 , https://github.com/helm/helm/security/advisories
