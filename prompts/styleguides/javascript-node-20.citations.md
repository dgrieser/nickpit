### Citations: JavaScript on Node.js 20: The Complete Production Developer Guide

Sources for `javascript-node-20.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Node.js 20 release announcement and v20.0.0 changelog (V8 11.3, Permission Model, stable test runner, SEA blob, semver-major changes). https://nodejs.org/en/blog/announcements/v20-release-announce , https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V20.md#20.0.0
    - Overview — paragraph 1: "…October 22, 2024, and reached **end-of-life on April 30, 2026**."
    - Overview — paragraph 3: "…and Corepack 0.34.6; 20.20.2 bundles ICU 78.2 / Unicode 17."
    - What's New in Node.js 20 (vs. 18 and 19) › Runtime highlights of 20.0.0 — section heading
[^2]: Node.js Release Working Group — release schedule (schedule.json: 20.x start 2023-04-18, LTS 2023-10-24, maintenance 2024-10-22, end 2026-04-30; 21.x end 2024-06-01; 22.x end 2027-04-30; 24.x maintenance 2026-10-20; 26.x LTS 2026-10-28; 27.x alpha 2026-10-28). https://github.com/nodejs/Release/blob/main/schedule.json
    - Overview — paragraph 1: "…October 22, 2024, and reached **end-of-life on April 30, 2026**."
    - Overview — paragraph 1: "…from October 28, 2026) ahead of an April 2027 release."
[^3]: Node.js distribution index (release dates and bundled versions: 20.20.2 on 2026-03-24, 26.12.1, 24.22.1, 22.23.4). https://nodejs.org/dist/index.json
    - Overview — paragraph 1: "…a security release; no 20.x release has been published since."
    - Overview — paragraph 1: "…from October 28, 2026) ahead of an April 2027 release."
[^4]: Node.js v20.20.2 release notes and the March 2026 security releases. https://nodejs.org/en/blog/release/v20.20.2 , https://nodejs.org/en/blog/vulnerability/march-2026-security-releases
    - Overview — paragraph 1: "…a security release; no 20.x release has been published since."
    - HTTP, `fetch()` and Networking › HTTP Servers — bullet 4: "…objects since 20.20.2; a `__proto__` header crashed earlier releases (CVE-2026-21710)."
    - Security › The Runtime Itself: End-of-Life — paragraph 1: "…crash (CVE-2026-21710) and `SNICallback` exceptions bypassing TLS error handlers (CVE-2026-21637)."
[^5]: Evolving the Node.js Release Schedule (one major per year from 27.x, alpha channel). https://nodejs.org/en/blog/announcements/evolving-the-nodejs-release-schedule
    - Overview — paragraph 1: "…from October 28, 2026) ahead of an April 2027 release."
[^6]: Node.js security releases of June, July and October 2026 (fixed only for 22.x/24.x/26.x; "End-of-Life versions are always affected"). https://nodejs.org/en/blog/vulnerability/june-2026-security-releases , https://nodejs.org/en/blog/vulnerability/july-2026-security-releases , https://nodejs.org/en/blog/vulnerability/october-2026-security-releases
    - Overview — paragraph 2: "…states that end-of-life lines are always affected by such releases."
    - Security › The Runtime Itself: End-of-Life — paragraph 1: "…July and October 2026 releases) are not fixed for 20."
[^7]: Node.js 20 changelog (all 20.x minors, notable changes and semver-minor commits). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V20.md
    - Overview — paragraph 3: "…and Corepack 0.34.6; 20.20.2 bundles ICU 78.2 / Unicode 17."
    - What's New in Node.js 20 (vs. 18 and 19) › Runtime additions in 20.x minors (exact versions) — section heading
    - Performance — bullet 2: "…and `fs.writeFileSync` with UTF-8 strings got fast paths in 20.5/20.11."
[^8]: Node.js v20.19.0 release notes (require(esm) and module syntax detection enabled by default, module-sync condition). https://nodejs.org/en/blog/release/v20.19.0
    - Overview — bullet 2: "…synchronous ES modules and ambiguous files are syntax-detected (see Modules)."
    - Modules: ESM, CommonJS and Interop › New in 20.19: `require()` of ES Modules (on by default) — paragraph 1: "Node 18, 19 and **21** throw `ERR_REQUIRE_ESM` instead."
    - Modules: ESM, CommonJS and Interop › New in 20.19: ESM Syntax Detection (on by default) — paragraph 1: "…was added in 20.10.0 and became the default in 20.19.0."
[^9]: Node.js 20 documentation — API "added in"/"changes" metadata (all.json for v20.20.2). https://nodejs.org/docs/v20.20.2/api/all.json
    - What's New in Node.js 20 (vs. 18 and 19) › Runtime highlights of 20.0.0 — section heading
[^10]: Node.js 19.0.0 changelog (keep-alive default, server.close idle connections, global Web Crypto, removal of --experimental-specifier-resolution). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V19.md#19.0.0
    - What's New in Node.js 20 (vs. 18 and 19) › Runtime highlights of 20.0.0 — bullet 6: "`performance.*` methods must be called with `performance` as receiver."
[^11]: Node.js release posts for 20.6.0, 20.10.0, 20.11.0, 20.12.0, 20.13.0, 20.15.0, 20.16.0, 20.17.0, 20.18.0. https://nodejs.org/en/blog/release/v20.6.0 , https://nodejs.org/en/blog/release/v20.12.0 , https://nodejs.org/en/blog/release/v20.18.0
    - What's New in Node.js 20 (vs. 18 and 19) › Runtime additions in 20.x minors (exact versions) — section heading
[^12]: Node.js 21 changelog (V8 11.8, stable fetch/WebStreams, navigator, test runner globs, globalPreload removal). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V21.md
    - Do Not Use or Recommend (Not in Node.js 20) — paragraph 1: "…(absent) and on the first release line that has it."
[^13]: Node.js 22 documentation (added-in versions of fs.glob, node:sqlite, module.registerHooks, util.getCallSites, --run, --permission, etc.). https://nodejs.org/docs/latest-v22.x/api/
    - Do Not Use or Recommend (Not in Node.js 20) — paragraph 1: "…(absent) and on the first release line that has it."
[^14]: Node.js 20 deprecations list (DEP0005, DEP0040, DEP0106, DEP0147, DEP0164, DEP0169, DEP0170, DEP0173–DEP0185). https://nodejs.org/docs/latest-v20.x/api/deprecations.html
    - Do Not Use or Recommend (Not in Node.js 20) › Deprecated or removed APIs (older features) — paragraph 1: "…`[DEP0106] DeprecationWarning: crypto.createCipher is deprecated.`, `[DEP0173] DeprecationWarning: assert.CallTracker is deprecated.`"
[^15]: TC39 finished proposals (Change Array by Copy, RegExp v flag, Well-Formed Unicode Strings, Resizable ArrayBuffer, Array Grouping, Promise.withResolvers, Set methods, Iterator helpers). https://github.com/tc39/proposals/blob/main/finished-proposals.md
    - New in 20: Change Array by Copy — paragraph 1: "…untouched; the same methods (except `toSpliced`) exist on typed arrays."
[^16]: V8 — RegExp v flag with set notation and properties of strings. https://v8.dev/features/regexp-v-flag
    - New in 20: Change Array by Copy › New in 20: The RegExp `v` Flag — paragraph 1: "…(`&&`), nested classes and properties of strings such as `\p{RGI_Emoji}`."
[^17]: Node.js 20 Modules documentation — Loading ECMAScript modules using require(). https://nodejs.org/docs/latest-v20.x/api/modules.html#loading-ecmascript-modules-using-require
    - Modules: ESM, CommonJS and Interop › New in 20.19: `require()` of ES Modules (on by default) — paragraph 1: "Node 18, 19 and **21** throw `ERR_REQUIRE_ESM` instead."
[^18]: npm registry metadata (engines) for eslint 10.13.0, vite 8.3.5/7.3.7, mocha 12.0.3, vitest 4.1.12/5.0.5, jest 30.5.2, undici 7.30.1/8.12.3, @types/node 20.19.43, @tsconfig/node20 20.1.10, as of October 2026. https://registry.npmjs.org/
    - Modules: ESM, CommonJS and Interop › New in 20.19: `require()` of ES Modules (on by default) — paragraph 2: "Vitest 5 and undici 8 dropped Node 20 entirely)."
    - Testing with `node:test` (Stable in 20) — paragraph 3: "…a Vitest major upgrade is blocked on the runtime upgrade."
[^19]: Node.js 20 ES modules documentation — customization hooks, import attributes, import.meta.dirname/filename, import.meta.resolve. https://nodejs.org/docs/latest-v20.x/api/esm.html , https://nodejs.org/docs/latest-v20.x/api/module.html#customization-hooks
    - Modules: ESM, CommonJS and Interop › Module Customization Hooks: `--import` + `module.register()` — paragraph 1: "…`globalPreload` hook is deprecated on 20 and removed in 21."
[^20]: Node.js 20.4.0 changelog — explicit resource management symbols on Node resources. https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V20.md#20.4.0
    - Resource Management Without `using` — paragraph 1: "…`net`/`http` servers, sockets, child processes and timers (all marked experimental)."
[^21]: Node.js January 2025 and July 2025 security releases (CVE-2025-23084, CVE-2025-27210 path traversal on Windows). https://nodejs.org/en/blog/vulnerability/january-2025-security-releases , https://nodejs.org/en/blog/vulnerability/july-2025-security-releases
    - File System › Path Traversal — paragraph 1: "…— another reason Node 20's lack of further fixes matters."
[^22]: Node.js 20 globals documentation (fetch Stability 1 – Experimental, Web Streams, WebSocket, EventSource). https://nodejs.org/docs/latest-v20.x/api/globals.html
    - HTTP, `fetch()` and Networking › `fetch()` on Node 20 — paragraph 1: "…but it is enabled by default and prints no warning."
[^23]: HashDoS in V8 array index strings (CVE-2026-21717). https://nodejs.org/en/blog/vulnerability/march-2026-hashdos
    - Security › The Runtime Itself: End-of-Life — paragraph 1: "…crash (CVE-2026-21710) and `SNICallback` exceptions bypassing TLS error handlers (CVE-2026-21637)."
[^24]: Node.js April 2024 security releases (CVE-2024-27980, child_process on Windows). https://nodejs.org/en/blog/vulnerability/april-2024-security-releases-2
    - Security › The Runtime Itself: End-of-Life — bullet 2: "…"BatBadBut"); passing user input with `shell: true` is still injection."
[^25]: Node.js v20.20.0 release notes (January 2026 security release). https://nodejs.org/en/blog/release/v20.20.0
    - Security › The Runtime Itself: End-of-Life — bullet 3: "…Model blocks `futimes` and requires full read+write for symlink APIs."
[^26]: Node.js 20 Permissions documentation. https://nodejs.org/docs/latest-v20.x/api/permissions.html
    - Security › New in 20: The Permission Model (Experimental) — paragraph 1: "…and the stable `--permission` spelling does not exist (bad option)."
[^27]: Node.js 20 test runner documentation. https://nodejs.org/docs/latest-v20.x/api/test.html
    - Testing with `node:test` (Stable in 20) — paragraph 1: "…stable since 20.0.0 and covers most unit-test needs without dependencies."
[^28]: Node.js 20 Single executable applications documentation. https://nodejs.org/docs/latest-v20.x/api/single-executable-applications.html
    - Single Executable Applications (SEA) — paragraph 1: "The feature is Experimental (Stability 1.1)."
