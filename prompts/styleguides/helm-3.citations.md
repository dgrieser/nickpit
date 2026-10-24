### Citations: Helm 3: Charts, Releases and CI — Complete Review Guide

Sources for `helm-3.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Helm v3.0.0 release (2019-11-13). https://github.com/helm/helm/releases/tag/v3.0.0
    - Helm 3: Charts, Releases and CI — Complete Review Guide — quote block: "Helm **v3.0.0** was released on **2019-11-13**."
    - What's New Across Helm 3 (3.0 → 3.22) › Helm 3 fundamentals (3.0) — paragraph 1: "Helm 3.0 removed Tiller and changed the release model:"
[^2]: Helm v3 End of Life (Helm blog, 2026-06-02: last minor on 2026-09-09, security fixes until 2027-02-10). https://helm.sh/blog/helm-v3-end-of-life
    - Helm 3: Charts, Releases and CI — Complete Review Guide — quote block: "…all users to migrate to Helm 4 before that date"."
    - Migrating to Helm 4 — Notes — paragraph 1: "Helm 3 receives security fixes only until 2027-02-10."
[^3]: Helm 4 Released (Helm blog; original Helm 3 support dates). https://helm.sh/blog/helm-4-released
    - Helm 3: Charts, Releases and CI — Complete Review Guide — quote block: "…planned bug fixes until 2026-07-08 and security fixes until 2026-11-11;"
[^4]: Helm v3.22.0 release notes ("the last Helm 3 minor release"). https://github.com/helm/helm/releases/tag/v3.22.0
    - Helm 3: Charts, Releases and CI — Complete Review Guide — quote block: "…contains only dependency and Kubernetes client-library updates plus bug fixes."
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 22: "…to `ProtonMail/go-crypto`, dependency CVE bumps. 3.22.1 (2026-10-15): dependency/CVE updates only"
[^5]: Helm v3.22.1 release notes (2026-10-15; next patches 4.3.2/3.22.2 on 2026-11-11). https://github.com/helm/helm/releases/tag/v3.22.1
    - Helm 3: Charts, Releases and CI — Complete Review Guide — quote block: "…of 2026-10-22 is **v3.22.1 (2026-10-15)**; v3.22.2 is scheduled for 2026-11-11."
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 22: "…to `ProtonMail/go-crypto`, dependency CVE bumps. 3.22.1 (2026-10-15): dependency/CVE updates only"
[^6]: Helm 3 Version Support Policy (n-3 skew table). https://helm.sh/docs/v3/topics/version_skew
    - Helm 3: Charts, Releases and CI — Complete Review Guide — quote block: "…1.34–1.31, 3.18.x → 1.33–1.30, 3.17.x → 1.32–1.29, 3.16.x → 1.31–1.28."
[^7]: Helm tags and releases (dates of 3.x releases). https://github.com/helm/helm/tags
    - Helm 3: Charts, Releases and CI — Complete Review Guide — quote block: "…of 2026-10-22 is **v3.22.1 (2026-10-15)**; v3.22.2 is scheduled for 2026-11-11."
[^8]: Helm v3.1.0 release notes. https://github.com/helm/helm/releases/tag/v3.1.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 1: "`--disable-openapi-validation`"
[^9]: Helm v3.2.0 release notes. https://github.com/helm/helm/releases/tag/v3.2.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 2: "`Chart.lock` packaged; fixes a critical `lookup` vulnerability in 3.1.x"
[^10]: Helm v3.5.0 release notes. https://github.com/helm/helm/releases/tag/v3.5.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 5: "`--kube-ca-file`"
[^11]: Helm v3.7.0 release notes (OCI changes, HIP 6). https://github.com/helm/helm/releases/tag/v3.7.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 7: "`helm repo add --password-stdin`"
[^12]: Helm v3.8.0 release notes (OCI GA). https://github.com/helm/helm/releases/tag/v3.8.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 8: "…by `_` in OCI tags; release names validated on install"
[^13]: Helm v3.10.0 release notes (--set-json). https://github.com/helm/helm/releases/tag/v3.10.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 10: "`--burst-limit`"
[^14]: Helm v3.12.0 release notes (--set-literal, --cascade). https://github.com/helm/helm/releases/tag/v3.12.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 12: "OCI annotations on pushed charts"
[^15]: Helm v3.13.0 release notes (--dry-run options, helm get metadata). https://github.com/helm/helm/releases/tag/v3.13.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 13: "…precedence and `null` handling made consistent (regression fixed in 3.13.1)"
[^16]: Helm v3.14.0 release notes. https://github.com/helm/helm/releases/tag/v3.14.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 14: "…`define`s; dependency names/aliases must be unique; readiness checks verify `observedGeneration`"
[^17]: Helm v3.15.0 release notes (hide secrets on dry-run). https://github.com/helm/helm/releases/tag/v3.15.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 15: "OCI getter proxy support"
[^18]: Helm v3.16.0 release notes (sha512sum, --skip-schema-validation). https://github.com/helm/helm/releases/tag/v3.16.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 16: "`--hide-notes`; proxy environment respected on install/upgrade"
[^19]: Helm v3.17.0 release notes (OCI digests, --take-ownership, toYamlPretty). https://github.com/helm/helm/releases/tag/v3.17.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 17: "…annotations and dependencies; username/password for `helm push` and dependency commands"
[^20]: Helm v3.18.0 release notes (JSON Schema 2020, hook log output). https://github.com/helm/helm/releases/tag/v3.18.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 18: "CPU/memory profiling via `HELM_PPROF_CPU_PROFILE`/`HELM_PPROF_MEM_PROFILE`; several OCI regressions fixed in 3.18.1–3.19.0"
[^21]: Helm v3.19.0 release notes (HTTPRoute scaffold, $ref regression fix). https://github.com/helm/helm/releases/tag/v3.19.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 19: "…(`1.31` → `v1.31`) and drops vendor suffixes (`1.31.2-eks-…` → `v1.31.2`)"
[^22]: Helm v3.20.0 release notes. https://github.com/helm/helm/releases/tag/v3.20.0
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 20: "CVE-2026-35206"
[^23]: Helm v3.20.1 release notes (nil values under empty-map defaults, tag+digest references). https://github.com/helm/helm/releases/tag/v3.20.1
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 20: "CVE-2026-35206"
[^24]: Helm v3.20.2 release notes (GHSA-hr2v-4r36-88hr). https://github.com/helm/helm/releases/tag/v3.20.2
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 20: "CVE-2026-35206"
[^25]: Helm v3.21.0 and v3.21.1 release notes. https://github.com/helm/helm/releases/tag/v3.21.0 , https://github.com/helm/helm/releases/tag/v3.21.1
    - What's New Across Helm 3 (3.0 → 3.22) › Minor releases — table row 21: "…moves to Go 1.26 and fixes a `helm template` panic"
[^26]: Helm security advisories (CVE-2021-32690, CVE-2022-36055, CVE-2022-23524, CVE-2022-23525, CVE-2022-23526, CVE-2023-25165, CVE-2024-25620, CVE-2024-26147, CVE-2025-32386, CVE-2025-32387, CVE-2025-53547, CVE-2025-55198, CVE-2025-55199, CVE-2026-35206). https://github.com/helm/helm/security/advisories
    - Security Releases — paragraph 1: "…that now means **3.22.1** (only the latest minor is patched)."
[^27]: azure/setup-helm (version input defaults to "latest"). https://github.com/Azure/setup-helm
    - Version-Aware Review — bullet 2: "…the action falls back to a hard-coded, outdated 3.x release)."
[^28]: Helm v3.22.1 source — pkg/kube/client.go (createPatch, updateResource). https://github.com/helm/helm/blob/v3.22.1/pkg/kube/client.go
    - How Helm 3 Applies Changes: Client-Side Three-Way Merge — paragraph 1: "…manifest**, and sends a PATCH (field manager `helm`, operation `Update`)."
    - How Helm 3 Applies Changes: Client-Side Three-Way Merge › Custom resources: two-way JSON merge patch — paragraph 1: "…only**; the live object is not consulted (`createPatch` in `pkg/kube/client.go`)."
[^29]: Helm v3.22.1 source — pkg/kube/ready.go (ReadyChecker). https://github.com/helm/helm/blob/v3.22.1/pkg/kube/ready.go
    - Waiting and Failure Handling: `--wait`, `--atomic`, `--cleanup-on-fail` › What Helm 3 waits for — paragraph 1: "With `--wait`, Helm 3.22.1 polls these readiness rules (`pkg/kube/ready.go`):"
[^30]: YAML 1.1 boolean type (y/n/yes/no/on/off). https://yaml.org/type/bool.html
    - Values: Sources, Precedence, Nulls and Typing › YAML typing traps — paragraph 1: "Values are parsed as YAML 1.1 and converted through JSON."
[^31]: Helm 3 Chart Best Practices Guide. https://helm.sh/docs/v3/chart_best_practices/
    - Chart Layout and Naming — paragraph 1: "Rules from the chart best practices:"
[^32]: Kubernetes recommended labels. https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
    - `_helpers.tpl`, Names and Labels — bullet 2: "…labels (`app.kubernetes.io/name`, `instance`, `version`, `component`, `part-of`, `managed-by`) on every object."
[^33]: Helm 3 Template Function List; Helm v3.22.1 source pkg/engine/funcs.go. https://helm.sh/docs/v3/chart_template_guide/function_list/ , https://github.com/helm/helm/blob/v3.22.1/pkg/engine/funcs.go
    - Template Language: Whitespace, Scope, Functions and Pitfalls — paragraph 1: "…`fromYaml`, `fromYamlArray` [3.2], `toJson`, `fromJson`, `fromJsonArray` [3.2], `toToml`, `fromToml` [3.17]."
[^34]: Kubernetes Pod Security Standards. https://kubernetes.io/docs/concepts/security/pod-security-standards/
    - Workload Manifests: Security, Resources, Probes, Availability › Security context — bullet 1: "Default to the Pod Security Standards **restricted** profile:"
[^35]: Helm 3 Chart Hooks; Helm v3.22.1 source pkg/action/hooks.go. https://helm.sh/docs/v3/topics/charts_hooks , https://github.com/helm/helm/blob/v3.22.1/pkg/action/hooks.go
    - Hooks — paragraph 1: "`pre-install`, `post-install`, `pre-upgrade`, `post-upgrade`, `pre-rollback`, `post-rollback`, `pre-delete`, `post-delete`, `test`."
[^36]: Helm 3 Charts topic (Chart.yaml fields, dependencies, crds/, schema, library charts). https://helm.sh/docs/v3/topics/charts/
    - CRDs — bullet 2: "…`helm uninstall`; the custom resources rendered from `templates/` were deleted)."
[^37]: Helm 4 Overview (breaking changes, renamed flags). https://helm.sh/docs/overview/
    - Migrating to Helm 4 — Notes — paragraph 1: "…release on client-side apply until it is upgraded with `--server-side=true`."
[^38]: HIP-0023: Utilize Server Side Apply for installs/upgrades; HIP-0022: Wait with kstatus. https://github.com/helm/community/blob/main/hips/hip-0023.md , https://github.com/helm/community/blob/main/hips/hip-0022.md
    - Migrating to Helm 4 — Notes — paragraph 1: "…release on client-side apply until it is upgraded with `--server-side=true`."
