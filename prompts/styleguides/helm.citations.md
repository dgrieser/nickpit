### Citations: Helm Style Guide

Sources for `helm.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Helm v4.0.0 release (2025-11-12) and Helm 4 announcement. https://github.com/helm/helm/releases/tag/v4.0.0 , https://helm.sh/blog/helm-4-released
    - Helm Style Guide — quote block: "…the latest release as of October 2026 is **v4.3.1 (2026-10-15)**."
    - Release and Support Status (October 2026) — paragraph 1: "…the Helm v3 end-of-life announcement and the version skew policy."
[^2]: Helm v4.3.1 release notes (2026-10-15; next releases 4.3.2/3.22.2 on 2026-11-11, 4.4.0 on 2027-01-13) and releases list. https://github.com/helm/helm/releases/tag/v4.3.1 , https://github.com/helm/helm/releases
    - Helm Style Guide — quote block: "…the latest release as of October 2026 is **v4.3.1 (2026-10-15)**."
    - Release and Support Status (October 2026) — paragraph 1: "…the Helm v3 end-of-life announcement and the version skew policy."
    - Version-Tied Features: Check Before Recommending › Helm 4 (never available in Helm 3) — paragraph 1: "Helm 4 release notes and changelog."
[^3]: Helm v3 End of Life (Helm blog, 2026-06-02: last minor 2026-09-09, security fixes until 2027-02-10). https://helm.sh/blog/helm-v3-end-of-life
    - Helm Style Guide — quote block: "…patch is v3.22.1 (2026-10-15), and security fixes end on **2027-02-10**."
    - Release and Support Status (October 2026) — paragraph 1: "…the Helm v3 end-of-life announcement and the version skew policy."
    - Release and Support Status (October 2026) — paragraph 1: "…last minor to 2026-09-09 and extended security fixes to 2027-02-10."
[^4]: Helm v3.22.0 ("the last Helm 3 minor release") and v3.22.1 release notes. https://github.com/helm/helm/releases/tag/v3.22.0 , https://github.com/helm/helm/releases/tag/v3.22.1
    - Helm Style Guide — quote block: "…patch is v3.22.1 (2026-10-15), and security fixes end on **2027-02-10**."
    - Release and Support Status (October 2026) — paragraph 1: "…the Helm v3 end-of-life announcement and the version skew policy."
[^5]: azure/setup-helm (the `version` input defaults to "latest"). https://github.com/Azure/setup-helm
    - Determine the Helm Version First — bullet 5: "…4 was released, with different apply, wait and flag semantics."
[^6]: Helm Version Support Policy (Kubernetes skew tables for Helm 4 and Helm 3). https://helm.sh/docs/topics/version_skew/ , https://helm.sh/docs/v3/topics/version_skew
    - Release and Support Status (October 2026) — paragraph 1: "…the Helm v3 end-of-life announcement and the version skew policy."
[^7]: Helm 4 Released (Helm blog; original Helm 3 support dates). https://helm.sh/blog/helm-4-released
    - Release and Support Status (October 2026) — paragraph 1: "…last minor to 2026-09-09 and extended security fixes to 2027-02-10."
[^8]: Helm 4 Overview (new features, breaking changes, renamed flags) and full changelog. https://helm.sh/docs/overview/ , https://helm.sh/docs/changelog/
    - Helm 3 vs Helm 4: Differences a Reviewer Must Know — paragraph 1: "…judged, how post-renderers and plugins work, and several CLI flags."
    - Version-Tied Features: Check Before Recommending › Helm 4 (never available in Helm 3) — paragraph 1: "Helm 4 release notes and changelog."
[^9]: HIP-0023: Utilize Server Side Apply for installs/upgrades; HIP-0022: Wait with kstatus. https://github.com/helm/community/blob/main/hips/hip-0023.md , https://github.com/helm/community/blob/main/hips/hip-0022.md
    - Helm 3 vs Helm 4: Differences a Reviewer Must Know — paragraph 1: "…judged, how post-renderers and plugins work, and several CLI flags."
    - Helm 3 vs Helm 4: Differences a Reviewer Must Know — bullet 5: "Helm 4 then keeps `csa` until `--server-side=true` is passed."
[^10]: HIP-0020: Charts v3 Enablement. https://github.com/helm/community/blob/main/hips/hip-0020.md
    - Helm 3 vs Helm 4: Differences a Reviewer Must Know — table row 21: "**Charts must use `apiVersion: v2`.**"
[^11]: Helm 3 release notes 3.1–3.22 (e.g. v3.13.0 dry-run modes, v3.17.0 OCI digests, v3.18.0 JSON Schema 2020-12). https://github.com/helm/helm/releases?q=v3
    - Version-Tied Features: Check Before Recommending › Helm 3 minors — paragraph 1: "…3 guide's verification on the latest patch of each minor."
    - `values.schema.json` › Draft support depends on the version — bullet 1: "`unevaluatedProperties` not enforced)."
[^12]: Helm v4.1.0, v4.1.3, v4.1.4, v4.2.0 and v4.3.0 release notes. https://github.com/helm/helm/releases/tag/v4.1.3 , https://github.com/helm/helm/releases/tag/v4.2.0 , https://github.com/helm/helm/releases/tag/v4.3.0
    - Version-Tied Features: Check Before Recommending › Helm 4 (never available in Helm 3) — paragraph 1: "Helm 4 release notes and changelog."
[^13]: Helm Chart Best Practices Guide. https://helm.sh/docs/chart_best_practices/
    - Chart Layout, Naming and Kubernetes Short Names — paragraph 1: "…(from the chart best practices, still valid for both majors):"
[^14]: YAML 1.1 boolean type (y/n/yes/no/on/off). https://yaml.org/type/bool.html
    - Values Design › YAML typing traps — paragraph 1: "Values are parsed as YAML 1.1 and converted through JSON."
[^15]: Helm Template Function List. https://helm.sh/docs/chart_template_guide/function_list/
    - Template Language: Idioms and Pitfalls — paragraph 1: "…(`include`, `tpl`, `required`, `lookup`, `toYaml`, `fromYaml`, `toJson`, `fromJson`, `toToml`, …)."
[^16]: Kubernetes recommended labels. https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
    - `_helpers.tpl`, Names and Labels — bullet 1: "`app.kubernetes.io/name`, `instance`, `version`, `component`, `part-of`, `managed-by`."
[^17]: Kubernetes Pod Security Standards. https://kubernetes.io/docs/concepts/security/pod-security-standards/
    - Workloads: Security, Resources, Probes, Availability › Security context — bullet 1: "Default to the Pod Security Standards **restricted** profile:"
[^18]: Helm Chart Hooks. https://helm.sh/docs/topics/charts_hooks/
    - Hooks and Deletion Policies — paragraph 2: "`pre-install`, `post-install`, `pre-upgrade`, `post-upgrade`, `pre-rollback`, `post-rollback`, `pre-delete`, `post-delete`, `test`."
[^19]: Helm Charts topic (Chart.yaml fields, dependencies, crds/, schema files, library charts). https://helm.sh/docs/topics/charts/
    - CRDs — bullet 1: "…only**, **never upgraded** and **never deleted** by Helm (both majors)."
[^20]: Kubernetes documentation — Server-Side Apply (field management, conflicts). https://kubernetes.io/docs/reference/using-api/server-side-apply/
    - Install, Upgrade and Rollback Semantics › How changes are applied — paragraph 1: "…apply after a Helm 4 upgrade until `--server-side=true` is passed."
[^21]: helm-unittest (test suite format and assertions). https://github.com/helm-unittest/helm-unittest
    - Testing and CI: `helm lint`, `helm template`, `helm-unittest`, `ct` › `helm-unittest` — paragraph 1: "…`tests/` (verified with helm-unittest 1.2.1 on Helm 3.22.1 and 4.3.1):"
[^22]: chart-testing (`ct lint`, `ct install` documentation). https://github.com/helm/chart-testing , https://github.com/helm/chart-testing/blob/main/doc/ct_lint.md
    - Testing and CI: `helm lint`, `helm template`, `helm-unittest`, `ct` › `ct` (chart-testing) — paragraph 1: "Sourced from the chart-testing documentation, not executed here:"
[^23]: Helm security advisories (CVE-2021-32690, CVE-2025-53547, CVE-2025-55198/55199, CVE-2026-35204/35205/35206 and others). https://github.com/helm/helm/security/advisories
    - Security — bullet 4: "…and the Helm 4 plugin-verification bypass (CVE-2026-35205, fixed in 4.1.4)."
