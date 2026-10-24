### Citations: TypeScript 5.0: The Complete Production Developer Guide

Sources for `typescript-5.0.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Announcing TypeScript 5.0 / TypeScript 5.0 release notes. Microsoft. https://devblogs.microsoft.com/typescript/announcing-typescript-5-0/ , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
    - Overview — quote block: "March 1, 2023)."
    - What's New in TypeScript 5.0 — paragraph 1: "…internally) and reports 10–20% faster builds on most code bases."
    - Compiler Configuration (tsconfig.json) › New in 5.0: Multiple configuration files in `extends` — paragraph 1: "…ones, and the file's own `compilerOptions` override all of them."
    - Compiler Configuration (tsconfig.json) › New in 5.0: Default changes — bullet 1: "…inference (`crlf` on Windows) is gone and cannot be selected."
    - Compiler Configuration (tsconfig.json) › New in 5.0: Default changes — bullet 2: "…for `mod.ts` is an error on case-insensitive file systems too."
    - Compiler Configuration (tsconfig.json) › New in 5.0: Deprecations and `ignoreDeprecations` — paragraph 1: "…any other value (`"5.1"`) is TS5103 *Invalid value* in 5.0."
    - Modules › New in 5.0: `--moduleResolution bundler` — paragraph 1: "…`package.json` `exports`/`imports` are honoured with the `import` condition like ESM."
    - Modules › New in 5.0: `--moduleResolution bundler` — bullet 2: "…extensions and CJS/ESM mismatches that break consumers on plain Node.js."
    - Modules › New in 5.0: Resolution customization flags — paragraph 1: "…works through CommonJS-style lookup and fails in ESM under `node16`)."
    - Modules › New in 5.0: `--verbatimModuleSyntax` — paragraph 1: "…based on usage, so what you read is what runs."
    - Narrowing and Control-Flow Analysis › Discriminated unions and exhaustiveness — paragraph 1: "…that scaffold all missing `case` labels for a literal union."
    - Functions › New in 5.0: Forbidden implicit coercions in relational operators — paragraph 1: "…may be strings when compared to numbers (4.9 accepted them):"
    - Generics › New in 5.0: `const` type parameters — paragraph 1: "…infers arguments as if they were written with `as const`:"
    - New in 5.0: All Enums Are Union Enums — paragraph 1: "Two new errors come with it:"
    - New in 5.0: Decorators — paragraph 1: "…3), which becomes the default whenever `--experimentalDecorators` is **not** set."
    - New in 5.0: Decorators › Legacy `--experimentalDecorators` — paragraph 1: "…accurately — the `key` argument is `undefined` for constructor parameters:"
    - JavaScript Projects (`checkJs` and JSDoc) — paragraph 1: "…`// @ts-check`) type-check JavaScript through JSDoc. 5.0 adds two tags:"
    - Performance › Compiler performance — bullet 1: "…for large projects and a package 59% of the size)."
    - Tooling › `tsc` and builds — bullet 2: "…for a release build while the tsconfig keeps declarations off."
    - Tooling › Editor and language service — bullet 1: "…fights ESLint's `sort-imports`; finer control is experimental (`typescript.unstable.organizeImportsIgnoreCase`, `organizeImportsCollation`, …)."
    - Removed / Changed in 5.0 — Migration Notes — bullet 8: "…moved between interfaces; fix the type errors rather than casting."
[^2]: npm registry: typescript package versions and publish times (5.0.0-beta … 7.1.0-beta). https://registry.npmjs.org/typescript
    - Overview — quote block: "March 1, 2023)."
    - Overview — quote block: "…on April 7, 2023 (5.0.3 followed on March 30, 2023)."
    - Overview — quote block: "…`latest` on npm; 7.1 is in beta (October 20, 2026)."
    - What's New in TypeScript 5.0 › Do not use or recommend (newer than 5.0) — bullet 7: "…project — a 5.0 tsconfig must spell every setting out."
[^3]: GitHub releases v5.0.3 and v5.0.4 and the closed issues/PRs of the "TypeScript 5.0.3" and "TypeScript 5.0.4" milestones. https://github.com/microsoft/TypeScript/releases/tag/v5.0.4 , https://github.com/microsoft/TypeScript/issues?q=milestone%3A%22TypeScript+5.0.4%22+is%3Aclosed
    - Overview — quote block: "…on April 7, 2023 (5.0.3 followed on March 30, 2023)."
    - Compiler Configuration (tsconfig.json) › Recommended baseline — bullet 1: "…it was reported as an error (fixed in 5.0.4, #53601)."
    - Compiler Configuration (tsconfig.json) › New in 5.0: Multiple configuration files in `extends` — paragraph 2: "…and added support for wildcard `exports` in tsconfig lookups (#53443)."
    - New in 5.0: Decorators — bullet 6: "…a double emit of constructors when getters are decorated (#53448)."
    - Tooling › Editor and language service — bullet 4: "…(JSX recovery in unary expressions, private identifiers in array assignments)."
    - 5.0.x Patch Releases and Known Issues — paragraph 1: "Always use 5.0.4 if 5.0 is required."
[^4]: DefinitelyTyped README — "Support window" (TypeScript versions less than two years old) and the `typeScriptVersion` field in published `@types` packages. https://github.com/DefinitelyTyped/DefinitelyTyped#support-window , https://registry.npmjs.org/@types/node
    - Overview — quote block: "…support in spring 2025 (see [`@types` packages and TypeScript 5.0](#types-packages-and-typescript-50))."
    - Modules › `@types` packages and TypeScript 5.0 — paragraph 1: "…package records the oldest supported version in its `typeScriptVersion` field."
[^5]: typescript 5.0.4 package.json (`"engines": { "node": ">=12.20" }`) and the API Breaking Changes wiki (ES2018 / Node 12.20). https://github.com/microsoft/TypeScript/wiki/API-Breaking-Changes#typescript-50
    - Overview — bullet 7: "`engines.node` is `>=12.20`; 4.9 declared `>=4.2.0`)."
    - Tooling › `tsc` and builds — bullet 1: "`tsc` 5.0 runs on Node.js ≥ 12.20 (5.1 requires 14.17)."
[^6]: TypeScript release notes 3.7–4.9. https://www.typescriptlang.org/docs/handbook/release-notes/overview.html
    - What's New in TypeScript 5.0 — paragraph 2: "…keyword, unlisted-property narrowing with `in` and `NaN` equality checks (4.9)."
[^7]: TypeScript 5.1 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-1.html
    - What's New in TypeScript 5.0 › Do not use or recommend (newer than 5.0) — bullet 1: "…raised the compiler's runtime floor to Node.js 14.17 / ES2020."
[^8]: TypeScript 5.2 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html
    - What's New in TypeScript 5.0 › Do not use or recommend (newer than 5.0) — bullet 2: "…es2023 in 5.2); type-only imports with `.ts` extensions without `allowImportingTsExtensions`."
    - Compiler Configuration (tsconfig.json) › Recommended baseline — bullet 5: "Flag it now so the upgrade does not break."
[^9]: TypeScript 5.3 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-3.html
    - What's New in TypeScript 5.0 › Do not use or recommend (newer than 5.0) — bullet 3: "…interactive inlay hints and the "prefer `type` auto-imports" editor setting."
[^10]: TypeScript 5.4 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html
    - What's New in TypeScript 5.0 › Do not use or recommend (newer than 5.0) — bullet 4: "…`Object.groupBy`/`Map.groupBy` types, `Promise.withResolvers` types, `module: preserve`, `require()` in `bundler` resolution."
[^11]: TypeScript 5.5 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
    - What's New in TypeScript 5.0 › Do not use or recommend (newer than 5.0) — bullet 5: "…(5.0 accepts `lib: es2023` but rejects `target: es2023` with TS6046)."
    - Compiler Configuration (tsconfig.json) › New in 5.0: Deprecations and `ignoreDeprecations` — paragraph 1: "…keeps rejecting them (verified on 5.4, 5.5, 5.6 and 6.0)."
[^12]: TypeScript 5.6, 5.7, 5.8 and 5.9 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
    - What's New in TypeScript 5.0 › Do not use or recommend (newer than 5.0) — bullet 6: "`import defer`, `--module node20`, the minimal `tsc --init` (5.9)."
[^13]: TypeScript wiki — API Breaking Changes, TypeScript 5.0 section. https://github.com/microsoft/TypeScript/wiki/API-Breaking-Changes
    - What's New in TypeScript 5.0 › Do not use or recommend (newer than 5.0) — bullet 9: "…and `enableAutoDiscovery` in tsconfig/jsconfig are no longer supported (use `typeAcquisition`)."
    - What's New in TypeScript 5.0 › Do not use or recommend (newer than 5.0) — bullet 10: "…types and the old top-level node factory functions were removed."
    - Tooling › `tsc` and builds — bullet 5: "…that import those paths (old ts-loader/fork-ts-checker versions, custom scripts) break."
    - Removed / Changed in 5.0 — Migration Notes — bullet 10: "…ts-morph, ts-patch and custom transformers to versions that support 5.0."
[^14]: TypeScript 5.0.4 `lib/lib.es2023.array.d.ts` and the TSConfig reference for `target`/`lib`. https://www.typescriptlang.org/tsconfig#target
    - Compiler Configuration (tsconfig.json) › Target and lib — paragraph 1: "…and contains only `Array.prototype.findLast` and `findLastIndex` (and their typed-array forms)."
[^15]: TC39 proposal-decorators (stage 3 at the time of 5.0) and the implementing pull request microsoft/TypeScript#50820. https://github.com/tc39/proposal-decorators , https://github.com/microsoft/TypeScript/pull/50820
    - New in 5.0: Decorators — paragraph 1: "…3), which becomes the default whenever `--experimentalDecorators` is **not** set."
[^16]: typescript-eslint — supported TypeScript versions (`SUPPORTED_TYPESCRIPT_VERSIONS` in `@typescript-eslint/typescript-estree` 6.21.0, 7.18.0 and 8.72.0). https://typescript-eslint.io/users/dependency-versions/
    - Tooling › Linting — paragraph 1: "…5.0; typescript-eslint 6.x supported `>=4.3.5 <5.4.0` and 7.x `>=4.7.4 <5.6.0`."
