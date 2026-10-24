### Citations: Helm 4: Charts, Releases and CI — Complete Review Guide

Footnote sources for `helm-4.md`. Not sent to the model; kept for maintainers.

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
[^11]: helm/helm PR #31601 — handle digest algorithm prefix for tag@digest OCI references (milestone 4.1.3; the v3.20.1 notes list a backport, but the commit is in no v3 tag and Helm 3 never had the bug: 3.17.4 and 3.20.0 already resolve `chart:tag@sha256:…`). https://github.com/helm/helm/pull/31601
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
