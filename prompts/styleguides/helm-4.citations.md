### Citations: Helm 4: Charts, Releases and CI — Complete Review Guide

Sources for `helm-4.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Helm v4.0.0 release (2025-11-12). https://github.com/helm/helm/releases/tag/v4.0.0
    - Helm 4: Charts, Releases and CI — Complete Review Guide — quote block: "…North America), the first new major version in six years."
    - What's New in Helm 4 — paragraph 1: "…plugins and post-renderers work, and a number of CLI flags."
[^2]: Helm 4 Released (Helm blog, Helm 3 support dates as originally announced). https://helm.sh/blog/helm-4-released
    - Helm 4: Charts, Releases and CI — Complete Review Guide — quote block: "…North America), the first new major version in six years."
    - Helm 4: Charts, Releases and CI — Complete Review Guide — quote block: "…3 release is v3.22.1 (2026-10-15); v3.22.2 is scheduled for 2026-11-11."
[^3]: Helm v4.3.1 release notes (2026-10-15; next releases 4.3.2/3.22.2 on 2026-11-11 and 4.4.0 on 2027-01-13). https://github.com/helm/helm/releases/tag/v4.3.1
    - Helm 4: Charts, Releases and CI — Complete Review Guide — quote block: "…scheduled releases are 4.3.2 (patch, 2026-11-11) and 4.4.0 (minor, 2027-01-13)."
[^4]: Helm releases list (4.0.x–4.3.x and 3.20.x–3.22.x dates). https://github.com/helm/helm/releases
    - Helm 4: Charts, Releases and CI — Complete Review Guide — quote block: "…scheduled releases are 4.3.2 (patch, 2026-11-11) and 4.4.0 (minor, 2027-01-13)."
[^5]: Helm v3 End of Life (Helm blog, 2026-06-02: last minor 3.22 on 2026-09-09, security fixes until 2027-02-10). https://helm.sh/blog/helm-v3-end-of-life
    - Helm 4: Charts, Releases and CI — Complete Review Guide — quote block: "…3 release is v3.22.1 (2026-10-15); v3.22.2 is scheduled for 2026-11-11."
[^6]: Helm Version Support Policy (version skew table). https://helm.sh/docs/topics/version_skew/
    - Helm 4: Charts, Releases and CI — Complete Review Guide — quote block: "…1.37–1.34, 4.2.x → 1.36–1.33, 4.1.x → 1.35–1.32, 4.0.x → 1.34–1.31."
[^7]: Helm 4 Full Changelog. https://helm.sh/docs/changelog/
    - What's New in Helm 4 — paragraph 1: "…plugins and post-renderers work, and a number of CLI flags."
    - Post-Renderers and Plugins [NEW in 4.0] › Post-renderers are plugins — paragraph 1: "…takes the **name of an installed plugin of type `postrenderer/v1`**:"
    - Chart Layout and Naming — bullet 2: "Helm 4 lint also lints `.yml` files."
[^8]: Helm 4 Overview (new features, breaking changes, renamed flags, charts v3). https://helm.sh/docs/overview/
    - What's New in Helm 4 — paragraph 1: "…plugins and post-renderers work, and a number of CLI flags."
[^9]: Helm v4.1.0 release notes. https://github.com/helm/helm/releases/tag/v4.1.0
    - Changes in 4.1, 4.2 and 4.3 › 4.1 (4.1.0 on 2026-01-21; last patch 4.1.4 on 2026-04-09) — section heading
[^10]: helm/helm PR #31901 — restore deprecated `--atomic` flag on install (milestone 4.1.3). https://github.com/helm/helm/pull/31901
    - Changes in 4.1, 4.2 and 4.3 › 4.1 (4.1.0 on 2026-01-21; last patch 4.1.4 on 2026-04-09) — bullet 6: "…deprecated alias (in 4.0.x–4.1.1 it fails with `unknown flag: --atomic`)"
[^11]: helm/helm PR #31601 — handle digest algorithm prefix for tag@digest OCI references (milestone 4.1.3; the v3.20.1 notes list a backport, but the commit is in no v3 tag and Helm 3 never had the bug: 3.17.4 and 3.20.0 already resolve `chart:tag@sha256:…`). https://github.com/helm/helm/pull/31601
    - Changes in 4.1, 4.2 and 4.3 › 4.1 (4.1.0 on 2026-01-21; last patch 4.1.4 on 2026-04-09) — bullet 6: "U+0073 's'`)"
[^12]: helm/helm PR #31644 — preserve nil values when chart default is an empty map (milestone 4.1.3). https://github.com/helm/helm/pull/31644
    - Changes in 4.1, 4.2 and 4.3 › 4.1 (4.1.0 on 2026-01-21; last patch 4.1.4 on 2026-04-09) — bullet 6: "…`null` under an empty-map chart default (4.0.x dropped the key)."
[^13]: Helm v4.2.0 release notes. https://github.com/helm/helm/releases/tag/v4.2.0
    - Changes in 4.1, 4.2 and 4.3 › 4.2 (4.2.0 on 2026-05-14; last patch 4.2.4 on 2026-08-13) — section heading
[^14]: Helm v4.3.0 release notes. https://github.com/helm/helm/releases/tag/v4.3.0
    - Changes in 4.1, 4.2 and 4.3 › 4.3 (4.3.0 on 2026-09-09; 4.3.1 on 2026-10-15) — section heading
[^15]: HIP-0020: Charts v3 Enablement. https://github.com/helm/community/blob/main/hips/hip-0020.md
    - Do Not Use or Recommend › Not available in the pinned 4.x minor — bullet 1: "Charts must use `apiVersion: v2`."
[^16]: azure/setup-helm (version input, default "latest"). https://github.com/Azure/setup-helm
    - Version-Aware Review — bullet 2: "…job can switch between major versions without a code change."
[^17]: HIP-0023: Utilize Server Side Apply for installs/upgrades. https://github.com/helm/community/blob/main/hips/hip-0023.md
    - Server-Side Apply (SSA) [NEW in 4.0] — paragraph 1: "…applied with Kubernetes server-side apply under the field manager `helm`."
[^18]: Kubernetes documentation — Server-Side Apply (field management, conflicts). https://kubernetes.io/docs/reference/using-api/server-side-apply/
    - Server-Side Apply (SSA) [NEW in 4.0] — paragraph 1: "…applied with Kubernetes server-side apply under the field manager `helm`."
[^19]: HIP-0022: Wait With kstatus. https://github.com/helm/community/blob/main/hips/hip-0022.md
    - Waiting: kstatus and Wait Strategies [NEW in 4.0] — paragraph 1: "…(`fluxcd/cli-utils` kstatus in 4.3.1), driven by watches instead of polling."
[^20]: kstatus (status computation rules used by Helm's watcher). https://github.com/kubernetes-sigs/cli-utils/blob/master/pkg/kstatus/README.md
    - Waiting: kstatus and Wait Strategies [NEW in 4.0] — paragraph 1: "…(`fluxcd/cli-utils` kstatus in 4.3.1), driven by watches instead of polling."
[^21]: HIP-0026: Wasm plugin system; Helm plugins documentation. https://github.com/helm/community/blob/main/hips/hip-0026.md , https://helm.sh/docs/plugins/
    - Post-Renderers and Plugins [NEW in 4.0] › Post-renderers are plugins — paragraph 1: "…takes the **name of an installed plugin of type `postrenderer/v1`**:"
[^22]: YAML 1.1 boolean type (y/n/yes/no/on/off). https://yaml.org/type/bool.html
    - Values: Multi-Document Files, `--set-json`, Nulls and Merging › YAML typing traps (all Helm versions) — paragraph 1: "…parsed as YAML 1.1 and then converted through JSON, so:"
[^23]: Helm Chart Best Practices Guide. https://helm.sh/docs/chart_best_practices/
    - Chart Layout and Naming — paragraph 1: "…from the chart best practices, still valid for Helm 4:"
[^24]: Kubernetes recommended labels. https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
    - `_helpers.tpl`, Names and Labels — bullet 2: "…labels (`app.kubernetes.io/name`, `instance`, `version`, `component`, `part-of`, `managed-by`) on every object."
[^25]: Helm Template Function List. https://helm.sh/docs/chart_template_guide/function_list/
    - Template Language: Whitespace, Scope, Functions and Pitfalls — paragraph 1: "…`fromJsonArray`, `toToml`, `fromToml`, `toYamlPretty`, and the Helm 4 additions below)."
[^26]: Helm v4.3.1 source — pkg/engine/funcs.go (function map incl. must*/duration helpers). https://github.com/helm/helm/blob/v4.3.1/pkg/engine/funcs.go
    - Template Language: Whitespace, Scope, Functions and Pitfalls — paragraph 1: "…`fromJsonArray`, `toToml`, `fromToml`, `toYamlPretty`, and the Helm 4 additions below)."
[^27]: Kubernetes Pod Security Standards. https://kubernetes.io/docs/concepts/security/pod-security-standards/
    - Workload Manifests: Security, Resources, Probes, Availability › Security context — bullet 1: "Default to the Pod Security Standards **restricted** profile:"
[^28]: Helm Chart Hooks. https://helm.sh/docs/topics/charts_hooks/
    - Hooks — paragraph 1: "`pre-install`, `post-install`, `pre-upgrade`, `post-upgrade`, `pre-rollback`, `post-rollback`, `pre-delete`, `post-delete`, `test`."
[^29]: Helm Charts topic (Chart.yaml fields, dependencies, crds/, schema, library charts). https://helm.sh/docs/topics/charts/
    - CRDs — bullet 1: "…`crds/` and upgrading leaves the CRD unchanged; uninstall keeps it)."
[^30]: Helm v4.1.3 release notes ("there was no 4.1.2 release due to a release automation issue"). https://github.com/helm/helm/releases/tag/v4.1.3
    - Changes in 4.1, 4.2 and 4.3 › 4.1 (4.1.0 on 2026-01-21; last patch 4.1.4 on 2026-04-09) — bullet 6: "…`null` under an empty-map chart default (4.0.x dropped the key)."
[^31]: Helm v4.1.4 release and security advisories GHSA-q5jf-9vfq-h4h7 (CVE-2026-35205), GHSA-vmx8-mqv2-9gmg (CVE-2026-35204), GHSA-hr2v-4r36-88hr (CVE-2026-35206). https://github.com/helm/helm/releases/tag/v4.1.4 , https://github.com/helm/helm/security/advisories
    - Changes in 4.1, 4.2 and 4.3 › 4.1 (4.1.0 on 2026-01-21; last patch 4.1.4 on 2026-04-09) — bullet 7: "**4.1.4** (2026-04-09) is a security release:"
