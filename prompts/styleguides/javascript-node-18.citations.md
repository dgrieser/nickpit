### Citations: JavaScript on Node.js 18: The Complete Production Developer Guide

Sources for `javascript-node-18.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Node.js 18 changelog (CHANGELOG_V18.md, versions 18.0.0–18.20.8). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V18.md
    - Overview — paragraph 3: "…Linux binaries need glibc ≥ 2.28; macOS binaries need 10.15+."
    - What's New in Node 18 (vs. 16.20.2 and 17.9.1) — paragraph 1: "…that relies on them still needs ≥ that 16 minor."
    - What's New in Node 18 (vs. 16.20.2 and 17.9.1) — paragraph 2: "…was removed (DEP0157), and `ERR_INVALID_CALLBACK` is gone (DEP0159; use `ERR_INVALID_ARG_TYPE`)."
    - Language and Syntax (V8 10.2 / ES2022) › Symbols as WeakMap Keys — Do Not Rely On It — paragraph 1: "…on it in code that must run on any 18.x."
    - `fetch` and Web APIs on Node 18 › New in 18: `fetch` Is Experimental — Know Its Behavior — bullet 11: "…and later undici CVEs are not fixed in any 18.x."
    - HTTP Servers and Clients › HTTP/2 — paragraph 1: "…crash on malformed HEADERS frames (CVE-2025-59465) has no 18.x fix."
    - File System and Paths › Path Traversal — bullet 3: "…later Windows path fixes (2025–2026) are not in any 18.x."
    - Security › Node 18 Is End-of-Life — Treat It as a Finding — bullet 2: "…of upstream support; certificate store updates stopped at NSS 3.108."
[^2]: Node.js Release schedule (schedule.json: start, LTS, maintenance and end dates per line). https://github.com/nodejs/Release/blob/main/schedule.json
    - Overview — paragraph 1: "…**October 18, 2023**, and reached **End-of-Life on April 30, 2025**."
    - Overview — paragraph 2: "…an alpha channel scheduled to start on October 28, 2026."
[^3]: Node.js distribution index (release dates and bundled versions, e.g. v18.20.8 2025-03-27, v26.12.1 2026-10-21). https://nodejs.org/dist/index.json
    - Overview — paragraph 1: "…is **18.20.8 (March 27, 2025)**; there will be no 18.20.9."
    - Overview — paragraph 2: "…an alpha channel scheduled to start on October 28, 2026."
[^4]: Node.js 18 is now available! (release announcement, April 19, 2022). https://nodejs.org/en/blog/announcements/v18-release-announce
    - Overview — paragraph 3: "…Linux binaries need glibc ≥ 2.28; macOS binaries need 10.15+."
    - `fetch` and Web APIs on Node 18 › New in 18: `fetch` Is Experimental — Know Its Behavior — paragraph 1: "…removed in 18.13.0; it became stable only in Node 21."
    - Performance — bullet 7: "…10.1 (Node 18); no need to avoid them for performance."
[^5]: Node.js 17.0.0 release notes (OpenSSL 3, `dns.lookup` verbatim, `structuredClone`, readline promises). https://nodejs.org/en/blog/release/v17.0.0
    - What's New in Node 18 (vs. 16.20.2 and 17.9.1) — paragraph 1: "…that relies on them still needs ≥ that 16 minor."
    - HTTP Servers and Clients › New Since 16: DNS Ordering and `localhost` (17.0) — paragraph 1: "…on 18.13+ but is **off by default** until Node 20."
[^6]: Updates on CVE assignment for End-of-Life versions. https://nodejs.org/en/blog/vulnerability/updates-cve-for-end-of-life
    - Overview — paragraph 1: "…which fixed three high-severity issues) shipped only for supported lines."
    - Security › Node 18 Is End-of-Life — Treat It as a Finding — bullet 1: "CVEs published since then are assumed to affect 18.x."
[^7]: Node.js security releases published January 13, 2026 (CVE-2025-55131, CVE-2025-59465, CVE-2025-59466, CVE-2026-21637, …). https://nodejs.org/en/blog/vulnerability/december-2025-security-releases
    - Overview — paragraph 1: "…which fixed three high-severity issues) shipped only for supported lines."
    - HTTP Servers and Clients › HTTP/2 — paragraph 1: "…crash on malformed HEADERS frames (CVE-2025-59465) has no 18.x fix."
    - Security › Node 18 Is End-of-Life — Treat It as a Finding — bullet 1: "…overflow via `async_hooks`, CVE-2026-21637 TLS callback exceptions bypassing error handlers)."
[^8]: Node.js v18 documentation — Global objects (`fetch`, `structuredClone`, `CustomEvent`, Web Crypto flags). https://nodejs.org/docs/latest-v18.x/api/globals.html
    - Overview — bullet 5: "…from browsers and from later Node lines in documented ways."
    - `fetch` and Web APIs on Node 18 › New in 18: `fetch` Is Experimental — Know Its Behavior — paragraph 1: "…removed in 18.13.0; it became stable only in Node 21."
[^9]: Node.js v18 documentation — Deprecated APIs (DEP0005, DEP0147, DEP0156–DEP0178). https://nodejs.org/docs/latest-v18.x/api/deprecations.html
    - What's New in Node 18 (vs. 16.20.2 and 17.9.1) — paragraph 2: "…was removed (DEP0157), and `ERR_INVALID_CALLBACK` is gone (DEP0159; use `ERR_INVALID_ARG_TYPE`)."
    - What's New in Node 18 (vs. 16.20.2 and 17.9.1) › Deprecated or Removed by Node 18 — bullet 16: "…a deprecation warning, and `assert` was removed in Node 22."
[^10]: V8 — Finding elements in arrays: `findLast` and `findLastIndex`. https://v8.dev/features/finding-in-arrays
    - Language and Syntax (V8 10.2 / ES2022) — paragraph 1: "From ES2023 only `findLast()`/`findLastIndex()` are present."
[^11]: Node.js v18 documentation — ECMAScript modules (import attributes, `import.meta.resolve`, CommonJS interop). https://nodejs.org/docs/latest-v18.x/api/esm.html
    - Modules: CommonJS, ES Modules and Interop › Importing CommonJS from ESM — paragraph 1: "…CommonJS work only for exports that `cjs-module-lexer` can detect statically"
[^12]: Node.js 18.20.0 release notes (import attributes `with`, `dirent.parentPath`, vm compile cache). https://nodejs.org/en/blog/release/v18.20.0
    - What's New in Node 18 (vs. 16.20.2 and 17.9.1) › Deprecated or Removed by Node 18 — bullet 16: "…a deprecation warning, and `assert` was removed in Node 22."
    - Modules: CommonJS, ES Modules and Interop › New in 18.20: JSON Modules — `with` vs `assert` Depends on the 18.x Minor — paragraph 1: "…18.0–18.19 column follows the 18.20.0 release notes, which introduced `with`.)"
[^13]: Node.js 18.19.0 release notes (`module.register`, `import.meta.resolve` unflagged, `--experimental-default-type`, npm 10, test runner backports). https://nodejs.org/en/blog/release/v18.19.0
    - Modules: CommonJS, ES Modules and Interop › New in 18.18/18.19: `--import` and `module.register()` Customization Hooks — bullet 3: "`globalPreload` is replaced by `initialize`."
[^14]: Node.js v18 documentation — `node:module` (customization hooks, `register`). https://nodejs.org/docs/latest-v18.x/api/module.html
    - Modules: CommonJS, ES Modules and Interop › New in 18.18/18.19: `--import` and `module.register()` Customization Hooks — bullet 3: "`globalPreload` is replaced by `initialize`."
[^15]: Node.js v18 documentation — Command-line options (`--watch`, `--import`, `--test-*`, `--openssl-legacy-provider`). https://nodejs.org/docs/latest-v18.x/api/cli.html
    - Tooling › `package.json`, Engines and Version Files — bullet 2: "Use it for development only."
[^16]: Node.js v18 documentation — Test runner. https://nodejs.org/docs/latest-v18.x/api/test.html
    - New in 18: Testing with `node:test` › Feature Availability by 18.x Minor — bullet 5: "…`test-*.*`, `test.*`, and every `.js/.cjs/.mjs` under `test/` directories (`node_modules` excluded)."
[^17]: Node.js v18 documentation — HTTP (`server.requestTimeout`, `closeIdleConnections`, `writeEarlyHints`). https://nodejs.org/docs/latest-v18.x/api/http.html
    - HTTP Servers and Clients › New in 18: Server Timeouts and Defaults — paragraph 1: "…or streaming responses beyond 5 minutes need an explicit `requestTimeout`."
[^18]: Node.js v18 documentation — Crypto. https://nodejs.org/docs/latest-v18.x/api/crypto.html
    - Security › Cryptography Idioms — bullet 5: "`crypto.hash()` (one-shot) is Node 21.7+; on 18 use `createHash(alg).update(data).digest()`."
[^19]: Node.js February 2024 security releases (CVE-2023-46809 Marvin attack, PKCS#1 v1.5 padding). https://nodejs.org/en/blog/vulnerability/february-2024-security-releases
    - Security › New Since 16: OpenSSL 3 Implications (Node 17+) — bullet 2: "…v1.5 decryption — flag it; migrate to `RSA_PKCS1_OAEP_PADDING` with SHA-256."
[^20]: Node.js April 2024 security releases (CVE-2024-27980, Windows batch-file command injection). https://nodejs.org/en/blog/vulnerability/april-2024-security-releases-2
    - Child Processes and Worker Threads — bullet 2: "…with carefully escaped arguments — or better, avoid batch files."
[^21]: Node.js current documentation (introduction versions of post-18 APIs: `require(esm)`, `fs.glob`, `util.styleText`, `URL.parse`, `node:sqlite`, …). https://nodejs.org/docs/latest/api/
    - What's New in Node 18 (vs. 16.20.2 and 17.9.1) › Do Not Use or Recommend (Newer Than Node 18) — paragraph 1: "…unflagged; verified by running every major from 19 to 26.)"
[^22]: TC39 finished proposals (ES2023–ES2026 features: change-array-by-copy, array grouping, Set methods, iterator helpers, explicit resource management, …). https://github.com/tc39/proposals/blob/main/finished-proposals.md
    - What's New in Node 18 (vs. 16.20.2 and 17.9.1) › Do Not Use or Recommend (Newer Than Node 18) — paragraph 1: "…unflagged; verified by running every major from 19 to 26.)"
[^23]: Node.js 19 changelog (CHANGELOG_V19.md). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V19.md
    - Overview — paragraph 4: "Where this matters it is called out explicitly."
[^24]: OpenSSL release strategy (3.0 listed as no longer supported). https://openssl-library.org/policies/releasestrat/
    - Overview — paragraph 1: "The bundled **OpenSSL 3.0** branch has itself left upstream support."
    - Security › Node 18 Is End-of-Life — Treat It as a Finding — bullet 2: "…of upstream support; certificate store updates stopped at NSS 3.108."
[^25]: @tsconfig/node18 (target es2022, lib es2023, module node16). https://github.com/tsconfig/bases/blob/main/bases/node18.json
    - Version-Aware Review — bullet 4: "…types `Array.prototype.toSorted()`, which **throws `TypeError` on Node 18** (see Tooling)."
    - Tooling › TypeScript Configuration for a Node 18 Runtime — paragraph 1: "["es2023"]`, `module`/`moduleResolution: "node16"`."
[^26]: npm registry metadata (`engines` of eslint, mocha, vitest, jest, undici, npm). https://registry.npmjs.org/
    - What's New in Node 18 (vs. 16.20.2 and 17.9.1) › Do Not Use or Recommend (Newer Than Node 18) — paragraph 6: "…no longer support Node 18** (from their published `engines` fields):"
[^27]: Node.js v18 documentation — File system (`readdir` recursive, `Dirent.parentPath`, `statfs`, `fs.watch`). https://nodejs.org/docs/latest-v18.x/api/fs.html
    - File System and Paths › Prefer `fs/promises` and Close Every `FileHandle` — bullet 2: "`fs.statfs()` requires 18.15.0."
[^28]: Node.js v18 documentation — Stream. https://nodejs.org/docs/latest-v18.x/api/stream.html
    - Streams › Stream Helpers and Consumers — bullet 6: "`Readable.toWeb()`, `Readable.fromWeb()`, `Writable.toWeb()`, `Duplex.from(webStreamPair)` (18.17.0)."
[^29]: Node.js v18 documentation — Child process. https://nodejs.org/docs/latest-v18.x/api/child_process.html
    - Child Processes and Worker Threads — bullet 3: "…'ignore'`); a child blocked on a full pipe never exits."
[^30]: Node.js v18 documentation — DNS (`verbatim`, `setDefaultResultOrder`, `getDefaultResultOrder`). https://nodejs.org/docs/latest-v18.x/api/dns.html
    - HTTP Servers and Clients › New Since 16: DNS Ordering and `localhost` (17.0) — paragraph 1: "…on 18.13+ but is **off by default** until Node 20."
[^31]: Node.js v18 documentation — Net (`autoSelectFamily`). https://nodejs.org/docs/latest-v18.x/api/net.html
    - HTTP Servers and Clients › New Since 16: DNS Ordering and `localhost` (17.0) — paragraph 1: "…on 18.13+ but is **off by default** until Node 20."
[^32]: Node.js January 2025 security releases (CVE-2025-22150 undici `fetch`, CVE-2025-23084, CVE-2025-23085). https://nodejs.org/en/blog/vulnerability/january-2025-security-releases
    - `fetch` and Web APIs on Node 18 › New in 18: `fetch` Is Experimental — Know Its Behavior — bullet 11: "…and later undici CVEs are not fixed in any 18.x."
[^33]: Node.js July 2022 security releases (CVE-2022-32223, `nodejs_conf` OpenSSL configuration). https://nodejs.org/en/blog/vulnerability/july-2022-security-releases
    - Security › New Since 16: OpenSSL 3 Implications (Node 17+) — bullet 3: "`--openssl-shared-config` restores the shared section and its risks."
