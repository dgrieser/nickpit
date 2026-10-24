### Citations: TypeScript 5.9: The Complete Production Developer Guide

Sources for `typescript-5.9.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: TypeScript 5.9 release notes (tsc --init, import defer, --module node20, DOM summaries, expandable hovers, hover length, optimizations, lib.d.ts ArrayBuffer changes, type argument inference changes). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
    - Overview — quote block: "July 24, 2025)."
    - What's New in TypeScript 5.9 — paragraph 1: "…no changes between the release candidate and the stable release."
    - Compiler Configuration (tsconfig.json) › New in 5.9: `tsc --init` output — paragraph 1: "…`tsconfig.json` (5.8 wrote 113 lines, mostly comments) with prescriptive settings:"
    - Modules › Module and moduleResolution modes — bullet 1: "…es2023`, `moduleResolution: node16`, `resolveJsonModule: true`, `esModuleInterop`, `resolvePackageJsonExports`/`Imports` and `moduleDetection: force`."
    - Modules › New in 5.9: `import defer` — paragraph 1: "…it until an export of the namespace is first accessed."
    - Generics › New in 5.9: type-argument inference changes — paragraph 1: "…errors; the documented fix is to add explicit type arguments."
    - New in 5.9: ArrayBuffer, Typed Arrays and `Buffer` — paragraph 1: "…typed with the default `ArrayBufferLike` is no longer a `BufferSource`."
    - Performance › Compiler performance — bullet 1: "…11% faster in some large projects, per the release notes)."
    - Tooling › Editor and language service — bullet 1: "DOM APIs show MDN-derived summary descriptions."
[^2]: npm registry, package `typescript` — publication times of 5.9.0-beta, 5.9.1-rc, 5.9.2, 5.9.3, 6.0.2, 6.0.3, 7.0.2 and current dist-tags (`npm view typescript time`). https://www.npmjs.com/package/typescript?activeTab=versions
    - Overview — quote block: "July 24, 2025)."
    - Overview — quote block: "…30, 2025 (the GitHub release is dated October 1, 2025)."
    - Overview — quote block: "…released July 8, 2026 (latest 7.0.2; 7.1 is in beta)."
[^3]: TypeScript 5.9.3 release on GitHub. https://github.com/microsoft/TypeScript/releases/tag/v5.9.3
    - Overview — quote block: "…30, 2025 (the GitHub release is dated October 1, 2025)."
[^4]: Announcing TypeScript 6.0 (changed defaults, deprecations, ignoreDeprecations "6.0", es2025, Temporal, getOrInsert, #/ imports, --stableTypeOrdering, --ignoreConfig). https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/
    - Overview — quote block: "…released July 8, 2026 (latest 7.0.2; 7.1 is in beta)."
    - What's New in TypeScript 5.9 › Do not use or recommend (newer than 5.9) — bullet 1: "…and `moduleResolution: bundler` together with `module: commonjs` (TS5095 in 5.9)."
    - What's New in TypeScript 5.9 › Do not use or recommend (newer than 5.9) — bullet 2: "…tsconfig directory) — a 5.9 tsconfig must spell them out."
    - Deprecations That Become Errors in 6.0 — paragraph 1: "…review and prefer the replacements, which all work in 5.9."
[^5]: Announcing TypeScript 7.0 (native compiler, no API in 7.0, side-by-side 6.0). https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/
    - Overview — quote block: "…released July 8, 2026 (latest 7.0.2; 7.1 is in beta)."
    - What's New in TypeScript 5.9 › Do not use or recommend (newer than 5.9) — bullet 5: "…(typescript-eslint, ts-morph, many bundler plugins) need the 5.x/6.0 JavaScript compiler."
    - Removed / Changed in 5.9 — Migration Notes — paragraph 2: "…6.0 installation side by side, since 7.0 has no API."
[^6]: DefinitelyTyped README — Support Window (TypeScript versions less than 2 years old). https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
    - Overview — quote block: "…years old, so 5.9 leaves that window around August 2027."
[^7]: Announcing TypeScript 5.9 (August 1, 2025; no changes since the RC). https://devblogs.microsoft.com/typescript/announcing-typescript-5-9/
    - What's New in TypeScript 5.9 — paragraph 1: "…no changes between the release candidate and the stable release."
[^8]: TypeScript 5.8 release notes (require(esm) under nodenext, --module node18, --erasableSyntaxOnly, --libReplacement, granular return checks, computed property names in declarations, import assertion restrictions) and the 5.0–5.7 release notes linked from it. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html
    - What's New in TypeScript 5.9 — paragraph 2: "…`--erasableSyntaxOnly`, `--libReplacement` and per-branch checking of conditional `return` expressions (5.8)."
    - Declaration Files and Publishing Libraries › Declaration hygiene — bullet 3: "…with the oldest supported compiler (`typesVersions` can redirect old compilers)."
[^9]: TypeScript 5.0 release notes — Deprecations and Default Changes (options scheduled to stop working in 5.5). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
    - What's New in TypeScript 5.9 › Do not use or recommend (newer than 5.9) — bullet 7: "Replace the first two with `verbatimModuleSyntax`."
[^10]: TSConfig Reference (target, lib, module, moduleResolution, strict family, verbatimModuleSyntax, isolatedDeclarations). https://www.typescriptlang.org/tsconfig/
    - Compiler Configuration (tsconfig.json) › Target and lib — paragraph 1: "`lib` only declares types; it never polyfills."
[^11]: Node.js release schedule (Node.js 20 EOL 2026-04-30; 22, 24, 26 dates). https://github.com/nodejs/Release/blob/main/schedule.json
    - Compiler Configuration (tsconfig.json) › Target and lib — paragraph 2: "Node.js release dates are from the Node.js release schedule."
[^12]: Node.js documentation — Modules: loading ECMAScript modules using require(). https://nodejs.org/api/modules.html#loading-ecmascript-modules-using-require
    - Modules › Module and moduleResolution modes — bullet 3: "…and 20.19; modules with top-level `await` still cannot be `require`d."
[^13]: TC39 proposal "Deferring Module Evaluation" (import defer). https://github.com/tc39/proposal-defer-import-eval
    - Modules › New in 5.9: `import defer` — paragraph 1: "…it until an export of the namespace is first accessed."
[^14]: microsoft/TypeScript#61668 — Fix type variable leaks and cache inconsistencies. https://github.com/microsoft/TypeScript/pull/61668
    - Generics › New in 5.9: type-argument inference changes — paragraph 1: "…errors; the documented fix is to add explicit type arguments."
[^15]: TypeScript 5.0 release notes — Decorators; TypeScript 5.2 release notes — Decorator Metadata and `using` declarations. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html
    - Decorators — paragraph 1: "TypeScript 5.9 supports two incompatible decorator systems:"
[^16]: Node.js documentation — Modules: TypeScript (type stripping history, unsupported syntax, recommended tsconfig). https://nodejs.org/api/typescript.html
    - Tooling › Running TypeScript directly — table row 1: "…refuses `.ts` under `node_modules`; type-only imports must use `import type`."
[^17]: typescript-eslint — Dependency Versions (supported TypeScript range `>=4.8.4 <6.1.0`). https://typescript-eslint.io/users/dependency-versions
    - Tooling › Linting — bullet 1: "…(any leaks), `no-unnecessary-condition`, `switch-exhaustiveness-check`, `consistent-type-imports`, `no-non-null-assertion`, `prefer-nullish-coalescing`, `restrict-template-expressions`, `unbound-method`, `only-throw-error`."
