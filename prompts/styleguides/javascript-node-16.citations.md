### Citations: JavaScript on Legacy Node.js (16 and Older: 14, 15, 17) — Production Review Guide

Sources for `javascript-node-16.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Node.js Release Working Group — release schedule (`schedule.json`: start, LTS, maintenance and end dates for v14–v27). https://github.com/nodejs/Release/blob/main/schedule.json
    - JavaScript on Legacy Node.js (16 and Older: 14, 15, 17) — Production Review Guide — quote block: "**end-of-life since 2023-09-11**."
    - JavaScript on Legacy Node.js (16 and Older: 14, 15, 17) — Production Review Guide — quote block: "…Node.js 17 (final 17.9.1, EOL 2022-06-01) were odd-numbered, never-LTS lines."
    - JavaScript on Legacy Node.js (16 and Older: 14, 15, 17) — Production Review Guide — quote block: "…(scheduled to start 2026-10-28; first stable 27.0.0 planned for 2027-04-22)."
    - Support status and upgrade advice — paragraph 1: "…14 ended on 2023-04-30 with 14.21.3 as its last release."
[^2]: Bringing forward the End-of-Life Date for Node.js 16 (Node.js blog). https://nodejs.org/en/blog/announcements/nodejs16-eol
    - JavaScript on Legacy Node.js (16 and Older: 14, 15, 17) — Production Review Guide — quote block: "**end-of-life since 2023-09-11**."
    - Support status and upgrade advice — paragraph 1: "…moved Node 16's end-of-life forward by seven months to **2023-09-11**."
[^3]: Node.js distribution index (release dates, V8, npm, OpenSSL versions per release). https://nodejs.org/dist/index.json
    - JavaScript on Legacy Node.js (16 and Older: 14, 15, 17) — Production Review Guide — quote block: "**end-of-life since 2023-09-11**."
    - JavaScript on Legacy Node.js (16 and Older: 14, 15, 17) — Production Review Guide — quote block: "…Node.js 17 (final 17.9.1, EOL 2022-06-01) were odd-numbered, never-LTS lines."
    - JavaScript on Legacy Node.js (16 and Older: 14, 15, 17) — Production Review Guide — quote block: "…(scheduled to start 2026-10-28; first stable 27.0.0 planned for 2027-04-22)."
    - JavaScript on Legacy Node.js (16 and Older: 14, 15, 17) — Production Review Guide — quote block: "…and the per-release V8 versions in the Node.js release index."
    - Support status and upgrade advice — paragraph 1: "…a security release; no 16.x release has been made since."
    - Support status and upgrade advice — paragraph 1: "…14 ended on 2023-04-30 with 14.21.3 as its last release."
    - Version-aware review › Minors matter inside 16.x — paragraph 2: "…from the Node.js release index; features from the 16.x changelog."
    - Version-aware review › Minors matter inside 16.x — paragraph 3: "…`timeout` and `dns.setDefaultResultOrder()` **14.18**, `exports` pattern trailers and Corepack **14.19**."
    - What's new in Node.js 16 (vs. 14 and 15) › JavaScript (V8 8.4 → 9.4) — paragraph 1: "…notes and the per-release V8 versions in the release index."
    - What's new in Node.js 16 (vs. 14 and 15) › Platform — bullet 1: "Node 18 uses V8 10.x."
[^4]: Node.js 16 changelog (16.0.0–16.20.2). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V16.md
    - JavaScript on Legacy Node.js (16 and Older: 14, 15, 17) — Production Review Guide — quote block: "…and the per-release V8 versions in the Node.js release index."
    - Support status and upgrade advice — paragraph 1: "…a security release; no 16.x release has been made since."
    - Version-aware review › Minors matter inside 16.x — paragraph 2: "…from the Node.js release index; features from the 16.x changelog."
    - What's new in Node.js 16 (vs. 14 and 15) › Node.js APIs — paragraph 1: "…changelogs and the `added:` metadata of the 16.20.2 API docs."
    - What's new in Node.js 16 (vs. 14 and 15) › Platform — bullet 4: "…the first line with prebuilt Apple Silicon (arm64 macOS) binaries."
    - Modules: CommonJS, ESM and interop › Module loading pitfalls — bullet 4: "…early 16 (`getFormat`, `getSource`, `transformSource`) do not work on 16.12+."
    - Async programming › Async context: `AsyncLocalStorage` — paragraph 1: "`run()`/`getStore()` were marked stable in 16.4."
    - Testing › `node:test` — available only on 16.17+ (and 18+), experimental, minimal — paragraph 1: "…to 18.7.0"); hooks (`before`, `after`, `beforeEach`, `afterEach`) arrived in **16.18.0**."
[^5]: Node.js 14 changelog (14.0.0–14.21.3). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V14.md
    - JavaScript on Legacy Node.js (16 and Older: 14, 15, 17) — Production Review Guide — quote block: "…and the per-release V8 versions in the Node.js release index."
    - Version-aware review › Minors matter inside 16.x — paragraph 3: "…`timeout` and `dns.setDefaultResultOrder()` **14.18**, `exports` pattern trailers and Corepack **14.19**."
[^6]: Node.js October 2023 Security Releases (fixed only in 18.18.2 and 20.8.1). https://nodejs.org/en/blog/vulnerability/october-2023-security-releases
    - Support status and upgrade advice — bullet 1: "…18.x and 20.x — Node 16 never received those fixes."
[^7]: Node.js — Updates on CVE handling for End-of-Life versions. https://nodejs.org/en/blog/vulnerability/updates-cve-for-end-of-life
    - Support status and upgrade advice — bullet 1: "…it does not assess security reports for EOL release lines."
[^8]: V8 feature explainers: logical assignment, `Promise.any`, RegExp match indices, private brand checks, `.at()`, `Object.hasOwn`, error cause, class static initializer blocks. https://v8.dev/features
    - What's new in Node.js 16 (vs. 14 and 15) › JavaScript (V8 8.4 → 9.4) — paragraph 1: "…notes and the per-release V8 versions in the release index."
[^9]: Node.js v16 deprecations (DEP0005, DEP0018, DEP0040, DEP0106, DEP0116, DEP0137, DEP0147, DEP0148, DEP0151, DEP0156, DEP0158). https://nodejs.org/docs/latest-v16.x/api/deprecations.html
    - What's new in Node.js 16 (vs. 14 and 15) › Node.js APIs — paragraph 1: "…changelogs and the `added:` metadata of the 16.20.2 API docs."
    - Do not use or recommend › Old APIs that are deprecated in 16 — paragraph 1: "…other `util.is*()` (doc-only), `buffer.slice()` (doc-only DEP0158 since 16.15; prefer `subarray()`)."
    - Modules: CommonJS, ESM and interop › `exports` and `imports` in `package.json` — bullet 2: "…(DEP0148 warning, verified) and **fail in 17+** — flag them."
    - Streams, files and buffers › Buffers — paragraph 1: "…favour of `subarray()` (same semantics); new code should use `subarray()`."
    - HTTP and networking without fetch › Core `http` client with timeouts and cancellation — bullet 4: "…16.12 (DEP0156); listen for `'close'` and check `res.complete` / `writableFinished`."
    - Removed / changed — migration notes › Deprecated APIs in 16 — replacements — paragraph 1: "…16.20.2 deprecations document; the runtime ones were reproduced on 16.20.2."
[^10]: Node.js v16 API documentation — globals, util, stream, timers, events, crypto (`added:` metadata and stability indexes). https://nodejs.org/docs/latest-v16.x/api/
    - Version-aware review › Minors matter inside 16.x — paragraph 3: "…`timeout` and `dns.setDefaultResultOrder()` **14.18**, `exports` pattern trailers and Corepack **14.19**."
    - What's new in Node.js 16 (vs. 14 and 15) › Node.js APIs — paragraph 1: "…changelogs and the `added:` metadata of the 16.20.2 API docs."
[^11]: Node.js 17 changelog (OpenSSL 3.0, `dns.lookup` verbatim default, V8 9.5). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V17.md
    - What's new in Node.js 16 (vs. 14 and 15) › Platform — bullet 2: "…in both 14 and 16 (17 switched to OpenSSL 3.0)."
[^12]: Node.js v16 ECMAScript modules and Packages documentation (`exports`, `imports`, conditional exports, dual package hazard). https://nodejs.org/docs/latest-v16.x/api/esm.html , https://nodejs.org/docs/latest-v16.x/api/packages.html
    - Modules: CommonJS, ESM and interop › Module type decision — paragraph 1: "…named-export detection exists since 14.13, unflagged top-level `await` since 14.8."
[^13]: npm registry metadata (`engines.node`) for undici, node-fetch, got, chalk, execa, nanoid, jest, ts-jest, vitest, mocha, c8, nyc, supertest, msw, eslint, eslint-plugin-n, typescript, prettier, pnpm, npm, dotenv, lru-cache. https://www.npmjs.com/
    - Modules: CommonJS, ESM and interop › No `require()` of ES modules in 14/15/16/17 — paragraph 2: "…major (`node-fetch@2`, `chalk@4`, `execa@5`, `got@11`, `nanoid@3`) is the usual fix."
    - HTTP and networking without fetch › No global `fetch` in 16 — bullet 1: "`undici@6` needs 18.17+, `undici@7` needs 20.18.1+)."
    - Testing › `node:test` — available only on 16.17+ (and 18+), experimental, minimal — paragraph 3: "…npm registry metadata of the latest release of each major."
    - Tooling, packaging and TypeScript › Package managers — bullet 1: "…supports `^14.17.0 || ^16.13.0 || >=18`; npm 10+ requires 18.17+."
    - Tooling, packaging and TypeScript › Package managers — bullet 3: "**Yarn 1** runs everywhere."
    - Tooling, packaging and TypeScript › ESLint — bullet 1: "ESLint 9 requires 18.18+."
    - Tooling, packaging and TypeScript › TypeScript — bullet 3: "…still runs on 14.17+ (TS 5.9 and 6.0 declare `>=14.17`)."
[^14]: cjs-module-lexer — CommonJS named-export detection used by Node.js. https://github.com/nodejs/cjs-module-lexer
    - Modules: CommonJS, ESM and interop › Importing CommonJS from ESM: named exports are a static guess — paragraph 1: "Named imports work only for names that `cjs-module-lexer` detects statically."
[^15]: Node.js v16 Packages — Dual package hazard. https://nodejs.org/docs/latest-v16.x/api/packages.html#dual-package-hazard
    - Modules: CommonJS, ESM and interop › `exports` and `imports` in `package.json` — bullet 6: "…implementation with a thin ESM wrapper re-exporting the CJS build."
[^16]: Node.js 15 changelog — "Throw On Unhandled Rejections" (#33021), npm 7, V8 8.6. https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V15.md
    - Async programming › Unhandled promise rejections — the biggest 14 → 16 behaviour change — paragraph 1: "…`DEP0018` and the process **keeps running** with exit code 0."
[^17]: Node.js v16 Stream documentation (experimental Readable helpers, `stream/promises`). https://nodejs.org/docs/latest-v16.x/api/stream.html
    - Streams, files and buffers › Readable helpers are experimental in 16 — paragraph 1: "…pinned to 16.17+ it is acceptable but should be noted."
[^18]: Node.js 18 changelog (`server.requestTimeout` default 300000 ms). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V18.md
    - HTTP and networking without fetch › Server timeouts (Node 16 defaults are unsafe) — paragraph 1: "Node 18.0 changed `requestTimeout` to 300 000 ms."
[^19]: nodejs/node#39987 — dns: default to verbatim=true in dns.lookup(). https://github.com/nodejs/node/pull/39987
    - HTTP and networking without fetch › DNS: IPv4-first ordering in 16, verbatim in 17+ — paragraph 1: "…to services bound only to `127.0.0.1` fail with `ECONNREFUSED ::1`."
[^20]: tsconfig/bases — `@tsconfig/node14` and `@tsconfig/node16`. https://github.com/tsconfig/bases
    - Tooling, packaging and TypeScript › TypeScript — bullet 2: "`@tsconfig/node14` uses `es2020`."
[^21]: V8 release v8.4 (WeakRefs and FinalizationRegistry, private methods and accessors). https://v8.dev/blog/v8-release-84
    - What's new in Node.js 16 (vs. 14 and 15) › JavaScript (V8 8.4 → 9.4) — paragraph 1: "…notes and the per-release V8 versions in the release index."
