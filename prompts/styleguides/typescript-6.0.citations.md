### Citations: TypeScript 6.0: The Complete Production Developer Guide

Sources for `typescript-6.0.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: TypeScript 6.0 release notes and "Announcing TypeScript 6.0" (2026-03-23). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html , https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/
    - TypeScript 6.0: The Complete Production Developer Guide — quote block: "…build **6.0.2 (2026-03-23)**; latest and probably final patch **6.0.3 (2026-04-16)**."
    - What's New in TypeScript 6.0 — paragraph 1: "…flags that help projects move to the 7.0 native compiler."
    - What's New in TypeScript 6.0 — paragraph 3: "…`tsc file.ts` in a directory that contains a `tsconfig.json` (TS5112)."
    - What's New in TypeScript 6.0 — paragraph 5: "…— tools built on the `typescript` npm package keep working."
    - tsconfig.json in 6.0 — New Defaults › `types` → `[]` **[New in 6.0]** — paragraph 2: "…on projects that used to load hundreds of `@types` packages):"
    - Tooling: Linters, Runners, Type Stripping, Editors › Other tools — bullet 3: "…member) rewrites `baseUrl`-based `paths` and adds `rootDir` across a repository."
    - Preparing for TypeScript 7.0 › `--stableTypeOrdering` **[New in 6.0]** — paragraph 2: "…order — fix those with explicit type arguments or annotations."
[^2]: npm registry — typescript versions and publish dates (6.0.0-beta 2026-02-11, 6.0.1-rc 2026-03-06, 6.0.2 2026-03-23, 6.0.3 2026-04-16, 7.0.2 2026-07-08, 7.1.0-beta 2026-10-20); GitHub release v6.0.3. https://www.npmjs.com/package/typescript?activeTab=versions , https://github.com/microsoft/TypeScript/releases/tag/v6.0.3
    - TypeScript 6.0: The Complete Production Developer Guide — quote block: "…build **6.0.2 (2026-03-23)**; latest and probably final patch **6.0.3 (2026-04-16)**."
[^3]: Announcing TypeScript 7.0 (2026-07-08) — new defaults, removed options, no API, `@typescript/typescript6`. https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/
    - TypeScript 6.0: The Complete Production Developer Guide — quote block: "…6.0, either directly or through the `@typescript/typescript6` compatibility package (`tsc6`)."
    - Version-Aware Review — Do Not Use or Recommend › Do not use or recommend (not available in 6.0) — bullet 1: "…6.0 it is a deprecation error that `ignoreDeprecations` can silence."
    - Performance of the Compiler and of Emitted Code › Compiler performance — bullet 6: "…6.0 deprecations and no longer depends on the compiler API."
    - Tooling: Linters, Runners, Type Stripping, Editors › typescript-eslint — paragraph 2: "…`tsc6` binary, so it can sit next to 7.0's `tsc`."
    - Tooling: Linters, Runners, Type Stripping, Editors › Other tools — bullet 2: "…continues to need 6.0 because 7.0 offers no API yet."
    - Preparing for TypeScript 7.0 — bullet 3: "…no stable API; those tools stay on 6.0 via `@typescript/typescript6`."
[^4]: @typescript/typescript6 — TypeScript 6.0 compatibility package (`tsc6`). https://www.npmjs.com/package/@typescript/typescript6
    - TypeScript 6.0: The Complete Production Developer Guide — quote block: "…6.0, either directly or through the `@typescript/typescript6` compatibility package (`tsc6`)."
    - Tooling: Linters, Runners, Type Stripping, Editors › typescript-eslint — paragraph 2: "…`tsc6` binary, so it can sit next to 7.0's `tsc`."
[^5]: typescript-go CHANGES.md — behavioural differences of the native compiler. https://github.com/microsoft/typescript-go/blob/main/CHANGES.md
    - Version-Aware Review — Do Not Use or Recommend › Do not use or recommend (not available in 6.0) — bullet 1: "…6.0 it is a deprecation error that `ignoreDeprecations` can silence."
    - JavaScript Checking (`checkJs`, JSDoc) — bullet 2: "Treat them as migration debt in checked JS."
    - Preparing for TypeScript 7.0 — bullet 4: "…JavaScript does not depend on Closure-style JSDoc or constructor functions."
    - Preparing for TypeScript 7.0 › Differences that appear only on 7.0 (not applicable to 6.0 builds) — bullet 4: "…`.d.ts` and source declarations are reported at every non-`.d.ts` site."
[^6]: Node.js documentation — Modules: TypeScript (type stripping history and limitations). https://nodejs.org/api/typescript.html
    - Version-Aware Review — Do Not Use or Recommend › Do not use or recommend (not available in 6.0) — bullet 6: "…enums, namespaces with values, parameter properties or `import =` aliases."
    - Tooling: Linters, Runners, Type Stripping, Editors › Node.js type stripping — paragraph 1: "`--experimental-transform-types` was removed in 26.0.0."
[^7]: TypeScript 5.5 release notes — Disabling Features Deprecated in TypeScript 5.0. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
    - Version-Aware Review — Do Not Use or Recommend › Deprecated or removed older features (do not introduce in 6.0 code) — bullet 1: "…reference, and silently tolerates `importsNotUsedAsValues: "remove"`, the old default value.)"
[^8]: Node.js v25.4.0 and v24.14.0 release notes — "module: allow subpath imports that start with #/" (nodejs/node#60864). https://nodejs.org/blog/release/v25.4.0 , https://nodejs.org/blog/release/v24.14.0
    - Modules and Module Resolution › Subpath imports and `#/` **[New in 6.0]** — bullet 2: "…shipped in Node.js 25.4.0 and was back-ported to 24.14.0 (2026-02-24)."
[^9]: typescript-eslint — Dependency Versions (TypeScript `>=4.8.4 <6.1.0`). https://typescript-eslint.io/users/dependency-versions/
    - Tooling: Linters, Runners, Type Stripping, Editors › typescript-eslint — paragraph 1: "…it cannot use 7.0's native compiler, which has no API."
[^10]: Announcing TypeScript 6.0 Beta (2026-02-11) — "the last release based on the current JavaScript codebase". https://devblogs.microsoft.com/typescript/announcing-typescript-6-0-beta/
    - TypeScript 6.0: The Complete Production Developer Guide — quote block: "…(October 2026)**: 6.0 is the **last compiler written in TypeScript/JavaScript**"
[^11]: TypeScript 5.9 release notes — `import defer`, `--module node20`, `lib.d.ts` ArrayBuffer changes, inference changes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
    - Modules and Module Resolution › `import defer` (5.9+) — paragraph 1: "TypeScript does not down-level it,"
[^12]: TypeScript 5.8 release notes — `require()` of ESM, `--module node18`, `--erasableSyntaxOnly`, `--libReplacement`, conditional return checks. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html
    - Narrowing and Control-Flow Analysis › Checks the compiler adds (5.6–5.8) — paragraph 1: "…functions (TS2454) from 5.7, the per-branch return check from 5.8."
    - Enums, Namespaces and Erasable Syntax › `erasableSyntaxOnly` (5.8+) — section heading
[^13]: TypeScript 5.7 release notes — never-initialized variables, `--rewriteRelativeImportExtensions`, generic TypedArrays, JSON import validation. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html
    - Narrowing and Control-Flow Analysis › Checks the compiler adds (5.6–5.8) — paragraph 1: "…functions (TS2454) from 5.7, the per-branch return check from 5.8."
[^14]: TypeScript 5.6 release notes — truthy/nullish checks, iterator helpers, `--strictBuiltinIteratorReturn`, `--noUncheckedSideEffectImports`, `--noCheck`. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html
    - Narrowing and Control-Flow Analysis › Checks the compiler adds (5.6–5.8) — paragraph 1: "…functions (TS2454) from 5.7, the per-branch return check from 5.8."
[^15]: TypeScript 5.4 release notes — preserved narrowing in closures, `NoInfer`, `--module preserve`. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html
    - Generics, Inference and `satisfies` › `NoInfer<T>` (5.4+) — section heading
[^16]: TypeScript 5.0 release notes — standard decorators, `const` type parameters, `--moduleResolution bundler`, `--verbatimModuleSyntax`. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
    - Generics, Inference and `satisfies` › `const` type parameters (5.0+) — section heading
    - Classes, Decorators and `accessor` › Standard decorators (5.0+) — section heading
[^17]: TypeScript 5.2 release notes — `using` declarations and explicit resource management. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html
    - Resource Management: `using` / `await using` — paragraph 1: "`using` (5.2+)"
[^18]: TypeScript 5.5 release notes — inferred type predicates, constant indexed access narrowing, `@import`, regex checking, `isolatedDeclarations`. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
    - Narrowing and Control-Flow Analysis › Inferred type predicates (5.5+) — section heading
