### Citations: JavaScript on Node.js 26: The Complete Production Developer Guide

Sources for `javascript-node-26.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Node.js 26 changelog (26.0.0 – 26.12.1, notable changes, semver-major commits, security releases). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V26.md
    - JavaScript on Node.js 26: The Complete Production Developer Guide — quote block: "…SQLite **3.54.0**, ICU 78.3 / Unicode 17 and npm 11.20.0."
    - Overview — paragraph 2: "Headline changes in Node.js 26 relative to 25 and 24:"
    - What's New in Node.js 26 › Runtime changes in 26.0.0 **New in 26** — bullet 1: "…need it then throw `ERR_NO_TEMPORAL` (added 26.2.0; verified with `fs.statSync('.').mtimeInstant`)."
    - What's New in Node.js 26 › Additions in 26.x minor releases **New in 26** — paragraph 2: "…than 26.12.1 runs with known High-severity TLS and DNS vulnerabilities."
[^2]: Node.js v26.12.1 release notes (latest patch, 2026-10-21). https://nodejs.org/en/blog/release/v26.12.1
    - JavaScript on Node.js 26: The Complete Production Developer Guide — quote block: "…SQLite **3.54.0**, ICU 78.3 / Unicode 17 and npm 11.20.0."
[^3]: Node.js release schedule (schedule.json: 26 LTS 2026-10-28, maintenance 2027-10-20, EOL 2029-04-30; 27 alpha 2026-10-28, start 2027-04-22). https://github.com/nodejs/Release/blob/main/schedule.json
    - JavaScript on Node.js 26: The Complete Production Developer Guide — quote block: "…LTS until 2027-04-30, and Node.js 25 reached EOL on 2026-06-01."
    - Overview — paragraph 1: "…and Node.js 26.12.1 is the newest release of any line."
[^4]: Evolving the Node.js Release Schedule (one major per year from 27.x, Alpha/Current/LTS phases). https://nodejs.org/en/blog/announcements/evolving-the-nodejs-release-schedule
    - Overview — paragraph 1: "…and Node.js 26.12.1 is the newest release of any line."
[^5]: Node.js 26.0.0 release announcement (Temporal, V8 14.6, Undici 8, deprecations and removals). https://nodejs.org/en/blog/release/v26.0.0
    - Overview — paragraph 2: "Headline changes in Node.js 26 relative to 25 and 24:"
[^6]: TC39 finished proposals (ES2025/ES2026 features, Stage 4 proposals for ES2027). https://github.com/tc39/proposals/blob/main/finished-proposals.md
    - What's New in Node.js 26 › Language changes (V8 14.6) **New in 26** — paragraph 1: "…in ES2027 — ship in 26 (and already in 24)."
[^7]: TC39 proposals: Upsert and Iterator Sequencing. https://github.com/tc39/proposal-upsert , https://github.com/tc39/proposal-iterator-sequencing
    - What's New in Node.js 26 › Language changes (V8 14.6) **New in 26** — paragraph 1: "…in ES2027 — ship in 26 (and already in 24)."
[^8]: Node.js 26 errors reference (ERR_NO_TEMPORAL, ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX, ERR_REQUIRE_ASYNC_MODULE). https://nodejs.org/docs/latest-v26.x/api/errors.html
    - What's New in Node.js 26 › Runtime changes in 26.0.0 **New in 26** — bullet 1: "…need it then throw `ERR_NO_TEMPORAL` (added 26.2.0; verified with `fs.statSync('.').mtimeInstant`)."
[^9]: Undici v8.0.0 release (HTTP/2 enabled by default, removed legacy handler wrappers). https://github.com/nodejs/undici/releases/tag/v8.0.0
    - What's New in Node.js 26 › Runtime changes in 26.0.0 **New in 26** — bullet 2: "Legacy undici handler wrappers and "fake" `Blob` objects are gone."
[^10]: Node.js 25 changelog (25.0.0 – 25.9.0, deprecations and removals). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V25.md
    - What's New in Node.js 26 › New in 25 (inherited by 26, missing in 24) — paragraph 1: "Features that arrived in 25.x and are **not** in 24.22.1:"
[^11]: Node.js main branch API docs and deprecations (unreleased REPLACEME entries for 27.0.0). https://github.com/nodejs/node/blob/main/doc/api/deprecations.md
    - Do Not Use or Recommend › Node.js 27 (main branch, alpha channel from late October 2026 — unreleased) — paragraph 1: "…26.x release — check the installed version before claiming them:"
[^12]: ESLint v10.0.0 released (eslintrc removed, Node.js requirements). https://eslint.org/blog/2026/02/eslint-v10.0.0-released/
    - Tooling › ESLint 10 flat config — paragraph 1: "ESLint 9 reached end-of-life on 2026-08-06."
[^13]: Node.js 26 Modules: Loading ECMAScript modules using require(). https://nodejs.org/docs/latest-v26.x/api/modules.html#loading-ecmascript-modules-using-require
    - Modules: ESM, CommonJS and Interop › `require(esm)` — stable — paragraph 1: "…default since 23.0/22.12, marked stable in 25.4.0 and in 24.x):"
[^14]: Node.js 26 Modules: TypeScript (type stripping). https://nodejs.org/docs/latest-v26.x/api/typescript.html
    - TypeScript: Built-in Type Stripping — paragraph 1: "No warning, no flag:"
[^15]: Node.js 26 Permissions (permission model, permission.drop, audit mode). https://nodejs.org/docs/latest-v26.x/api/permissions.html
    - Security › The permission model — paragraph 1: "The permission model is **stable** (Stability 2) in Node 26."
[^16]: Node.js 26 SQLite (Database/Statement, DEP0210/DEP0211). https://nodejs.org/docs/latest-v26.x/api/sqlite.html
    - Data: `node:sqlite` — paragraph 1: "`node:sqlite` is unflagged and a **release candidate** (Stability 1.2)."
[^17]: nodejs/node#60441 — stream: readable read one buffer at a time. https://github.com/nodejs/node/pull/60441
    - Streams › `readable.read()` returns one chunk **New in 26** — paragraph 1: "…assumes it drained the buffer loses data on Node 26"
[^18]: nodejs/node#60016 — http: fix handling of HTTP upgrades with bodies. https://github.com/nodejs/node/pull/60016
    - HTTP Servers — bullet 6: "…(New in 26)**: request bodies on `Upgrade` requests are parsed;"
[^19]: Temporal documentation (TC39 proposal-temporal). https://tc39.es/proposal-temporal/docs/
    - Dates and Times: Temporal **New in 26** — paragraph 1: "…date/time code; keep `Date` only at boundaries that require it."
[^20]: Node.js 26 util (debounce, throttle, markPromiseAsHandled, parseArgs, styleText). https://nodejs.org/docs/latest-v26.x/api/util.html
    - Asynchronous Programming › `util.throttle()` and `util.debounce()` **New in 26.10** — paragraph 1: "…return functions that return **promises**, and both accept a `signal`."
[^21]: Node.js 26 Test runner. https://nodejs.org/docs/latest-v26.x/api/test.html
    - Testing with `node:test` — paragraph 1: "Run with `node --test`"
[^22]: Node.js 26 Deprecated APIs (DEP0005 … DEP0211). https://nodejs.org/docs/latest-v26.x/api/deprecations.html
    - Modules: ESM, CommonJS and Interop › Module customization hooks — paragraph 1: "…`module.register()` (off-thread async hooks) emits a runtime deprecation warning (DEP0205)."
[^23]: Node.js April 2024 security releases (CVE-2024-27980, batch file argument injection on Windows). https://nodejs.org/en/blog/vulnerability/april-2024-security-releases-2
    - Child Processes — bullet 2: "…them without `shell: true` since the 2024 security fix (CVE-2024-27980)"
[^24]: MDN: Intl.Locale.prototype.getWeekInfo() (accessor replaced by method); TC39 Intl Locale Info proposal. https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/Locale/getWeekInfo , https://github.com/tc39/proposal-intl-locale-info
    - Numbers, Strings and Binary Data › `Intl.Locale` getters removed **New in 26** — paragraph 1: "…(verified against 24.22.1 and 25.9.0, where they still return values):"
[^25]: TypeScript 5.8 release notes (--erasableSyntaxOnly). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html
    - TypeScript: Built-in Type Stripping — bullet 1: "…on `"erasableSyntaxOnly": true` (TypeScript 5.8+) so `tsc` rejects them too."
[^26]: Node.js 26 Diagnostics Channel (built-in channels, TracingChannel). https://nodejs.org/docs/latest-v26.x/api/diagnostics_channel.html
    - Observability › diagnostics_channel and tracing — paragraph 1: "OpenTelemetry instrumentations build on these channels"
[^27]: Node.js 24 changelog (24.x backports of 25/26 features, up to 24.22.1). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V24.md
    - What's New in Node.js 26 › Additions in 26.x minor releases **New in 26** — paragraph 1: "Many of these were later backported to Node.js 24.x"
[^28]: Undici documentation bundled with Node 26 (headersTimeout/bodyTimeout defaults). https://github.com/nodejs/undici/blob/main/docs/docs/getting-started.md
    - HTTP Client: `fetch()` and Undici 8 — bullet 3: "**There is no default overall timeout**"
