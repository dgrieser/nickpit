### Citations: TypeScript 5.8: The Complete Production Developer Guide

Sources for `typescript-5.8.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: TypeScript 5.8 Release Notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html
    - TypeScript 5.8: The Complete Production Developer Guide — quote block: "…the latest and final patch is **5.8.3 (April 5, 2025)**."
    - Overview — paragraph 1: "Its changes are:"
    - What's New in TypeScript 5.8 › New in 5.8: Granular checks for branches in `return` expressions — paragraph 1: "…the direct operand of `return` against the declared return type."
    - What's New in TypeScript 5.8 › New in 5.8: `require()` of ES modules under `--module nodenext` — paragraph 1: "…`"type": "module"`) may `require()`/`import x = require()` an ES module."
    - What's New in TypeScript 5.8 › New in 5.8: `--module node18` — paragraph 1: "There is **no `node20` mode in 5.8** (added in 5.9)."
    - What's New in TypeScript 5.8 › New in 5.8: `--erasableSyntaxOnly` — paragraph 1: "The flag is unknown to 5.7.3 (TS5023)."
    - What's New in TypeScript 5.8 › New in 5.8: `--libReplacement` — paragraph 1: "`--libReplacement false` disables it."
    - What's New in TypeScript 5.8 › New in 5.8: Preserved computed property names in declaration files — paragraph 1: "…the declaration file degraded the class to an index signature."
    - What's New in TypeScript 5.8 › New in 5.8: `lib.d.ts` changes — bullet 1: "…occasional new errors in DOM-heavy code are expected on upgrade."
    - What's New in TypeScript 5.8 › New in 5.8: Performance — paragraph 1: "…— mainly visible in `--watch` and editors on large projects."
[^2]: npm registry, `typescript` package versions, publish times and dist-tags (5.8.2 2025-02-28, 5.8.3 2025-04-05, 5.9.2 2025-07-31, 5.9.3 2025-09-30, 6.0.2 2026-03-23, 6.0.3 2026-04-16, 7.0.2 2026-07-08, 7.1.0-beta 2026-10-20). https://www.npmjs.com/package/typescript?activeTab=versions
    - TypeScript 5.8: The Complete Production Developer Guide — quote block: "…the latest and final patch is **5.8.3 (April 5, 2025)**."
    - TypeScript 5.8: The Complete Production Developer Guide — quote block: "…`typescript@latest` on npm; 7.1 is in beta (October 20, 2026)."
[^3]: DefinitelyTyped README — Support Window. https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
    - TypeScript 5.8: The Complete Production Developer Guide — quote block: "…years), so 5.8 projects still get type updates for now."
[^4]: Announcing TypeScript 5.8 RC ("we have had to pull back some work on how functions with conditional return types are checked"). https://devblogs.microsoft.com/typescript/announcing-typescript-5-8-rc/
    - Overview — paragraph 2: "…conditional and indexed access types") was pulled before the RC."
[^5]: TypeScript 5.4, 5.5 and 5.6 Release Notes (NoInfer, closure narrowing, inferred type predicates, regex checking, isolatedDeclarations, disallowed truthy/nullish checks, iterator helpers, strictBuiltinIteratorReturn, noUncheckedSideEffectImports, noCheck). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html
    - Overview — paragraph 3: "…es2024`, generic `TypedArray`s and validated JSON imports under `nodenext` (5.7)."
    - Type System Essentials › Narrowing — paragraph 1: "…narrows `obj[key]` when both `obj` and `key` are effectively constant."
[^6]: TypeScript 5.7 Release Notes (never-initialized variables, rewriteRelativeImportExtensions, es2024, generic TypedArrays, validated JSON imports). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html
    - Overview — paragraph 3: "…es2024`, generic `TypedArray`s and validated JSON imports under `nodenext` (5.7)."
    - Modules and Module Resolution › JSON modules — bullet 1: "…the default import is available, and named imports are TS1544."
[^7]: microsoft/TypeScript PR #56941 — check return expressions branch by branch. https://github.com/microsoft/TypeScript/pull/56941
    - What's New in TypeScript 5.8 › New in 5.8: Granular checks for branches in `return` expressions — paragraph 1: "…the direct operand of `return` against the declared return type."
[^8]: Node.js documentation — Modules: CommonJS, "Loading ECMAScript modules using require()". https://nodejs.org/api/modules.html#loading-ecmascript-modules-using-require
    - What's New in TypeScript 5.8 › New in 5.8: `require()` of ES modules under `--module nodenext` — paragraph 1: "…in 22.12.0 and 20.19.0, warning-free since 22.13.0/20.19.0, non-experimental since 25.4.0)."
    - What's New in TypeScript 5.8 › New in 5.8: `require()` of ES modules under `--module nodenext` — bullet 2: "…ES module exports a value under the string name `"module.exports"`."
[^9]: microsoft/TypeScript PR #60761 — require(esm) and import-assertion changes under nodenext. https://github.com/microsoft/TypeScript/pull/60761
    - What's New in TypeScript 5.8 › New in 5.8: `require()` of ES modules under `--module nodenext` — paragraph 1: "…`"type": "module"`) may `require()`/`import x = require()` an ES module."
[^10]: microsoft/TypeScript PR #60722 — `--module node18`. https://github.com/microsoft/TypeScript/pull/60722
    - What's New in TypeScript 5.8 › New in 5.8: `--module node18` — paragraph 1: "There is **no `node20` mode in 5.8** (added in 5.9)."
[^11]: TypeScript 6.0 Release Notes (new defaults, deprecations, es2025, Temporal, getOrInsert, RegExp.escape, `#/` imports, stableTypeOrdering). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html
    - What's New in TypeScript 5.8 › New in 5.8: Import assertions are an error under `--module nodenext` — paragraph 1: "(TypeScript 6.0 deprecates `assert` on imports everywhere.)"
    - What's New in TypeScript 5.8 › New in 5.8: `--libReplacement` — paragraph 1: "TypeScript 6.0 changes the default to `false`."
    - Do Not Use or Recommend (5.9 and later) — paragraph 1: "…fail on 5.8.3 and to work on the version named."
[^12]: Node.js documentation — Modules: TypeScript (type stripping history, unsupported syntax, recommended tsconfig). https://nodejs.org/api/typescript.html
    - What's New in TypeScript 5.8 › New in 5.8: `--erasableSyntaxOnly` — paragraph 1: "TypeScript syntax that generates JavaScript throws `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX` at load time."
    - What's New in TypeScript 5.8 › New in 5.8: `--erasableSyntaxOnly` — bullet 3: "…Node.js 22–26 a file with a decorator fails to parse."
    - What's New in TypeScript 5.8 › New in 5.8: `--erasableSyntaxOnly` — bullet 4: "…5.8 or newer" — see [Running TypeScript directly on Node.js](#running-typescript-directly-on-nodejs)."
    - tsconfig Recommendations › Running TypeScript directly on Node.js — paragraph 1: "`tsconfig.json` is ignored at runtime, so `tsc` only type-checks."
[^13]: microsoft/TypeScript PR #61011 — `--erasableSyntaxOnly`. https://github.com/microsoft/TypeScript/pull/61011
    - What's New in TypeScript 5.8 › New in 5.8: `--erasableSyntaxOnly` — paragraph 1: "The flag is unknown to 5.7.3 (TS5023)."
[^14]: microsoft/TypeScript v5.8.2...v5.8.3 (PRs #61244, #61452, #61392, #61342, #61261). https://github.com/microsoft/TypeScript/compare/v5.8.2...v5.8.3
    - What's New in TypeScript 5.8 › New in 5.8: `--erasableSyntaxOnly` — bullet 1: "…`typescript` to `~5.8.3`, not `5.8.2`, when relying on this flag."
    - What's New in TypeScript 5.8 › Patch release 5.8.3 — paragraph 1: "…comments from one file leaking into another file's declaration output."
[^15]: TypeScript 5.9 Release Notes (import defer, node20, tsc --init, lib.d.ts ArrayBuffer change, inference changes). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
    - Do Not Use or Recommend (5.9 and later) — paragraph 1: "…fail on 5.8.3 and to work on the version named."
[^16]: TypeScript 5.0 Release Notes — Deprecations and Default Changes; TypeScript 5.5 Release Notes — Disabling Features Deprecated in TypeScript 5.0. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
    - Do Not Use or Recommend (5.9 and later) › Deprecated or removed before 5.8 (still relevant in review) — bullet 1: "…default `remove`), and `prepend` in project references (TS5102 in `references`)."
[^17]: Are The Types Wrong? (`@arethetypeswrong/cli`). https://github.com/arethetypeswrong/arethetypeswrong.github.io
    - Modules and Module Resolution › `package.json` `exports`, `imports`, and types — bullet 2: "…of them ("masquerading as ESM/CJS"); check with `@arethetypeswrong/cli` (`attw --pack`)."
    - Tooling and Linting › Formatting, transpilers, runtimes — bullet 5: "…--pack .`) and `publint` catch broken `exports`/types mappings before publishing."
[^18]: TypeScript 5.0 Release Notes — Decorators. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
    - Classes › Decorators — paragraph 1: "`--experimentalDecorators` switches to the legacy implementation."
[^19]: Zod documentation — Requirements. https://zod.dev/
    - Runtime Validation at Boundaries — bullet 1: "…is tested against TypeScript 5.5 and later and requires `strict`)."
[^20]: typescript-eslint — Dependency Versions. https://typescript-eslint.io/users/dependency-versions/
    - Tooling and Linting › typescript-eslint — paragraph 1: "…TypeScript `>=4.8.4 <6.1.0`, so it covers 5.8 (and not 7.0)."
[^21]: Node.js Release Working Group schedule. https://github.com/nodejs/Release/blob/main/schedule.json
    - Removed / Changed in 5.8 — Migration Notes (5.7 → 5.8) — bullet 6: "…28, 2026) are the supported lines as of October 2026."
[^22]: TSConfig Reference. https://www.typescriptlang.org/tsconfig/
    - tsconfig Recommendations — paragraph 1: "…an explicit configuration; option semantics are in the TSConfig reference."
[^23]: A 10x Faster TypeScript (announcement of the native port). https://devblogs.microsoft.com/typescript/typescript-native-port/
    - Performance › Compile-time performance — bullet 8: "…shipped as TypeScript 7.0, was announced as roughly 10× faster,"
