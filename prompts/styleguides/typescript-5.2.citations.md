### Citations: TypeScript 5.2: The Complete Production Developer Guide

Sources for `typescript-5.2.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Announcing TypeScript 5.2 (Microsoft DevBlogs, 2023-08-24). https://devblogs.microsoft.com/typescript/announcing-typescript-5-2/
    - TypeScript 5.2: The Complete Production Developer Guide — quote block: "…stable 5.2 release; no later 5.2.x patch was ever published."
    - What's New in TypeScript 5.2 — paragraph 1: "…several type-system relaxations, new library typings and four breaking changes."
    - Explicit Resource Management: `using` / `await using` [New in 5.2] › Semantics — paragraph 1: "…disposed in **reverse order of declaration** (last in, first out)."
    - Decorator Metadata [New in 5.2] — paragraph 1: "…all decorators of that class, and exposed afterwards as `TheClass[Symbol.metadata]`."
    - Performance of the Compiler and of Emitted Code › Compiler — bullet 1: "…release notes measured a drizzle-based case at about 1.5× faster)."
[^2]: TypeScript 5.2 release notes (TypeScript Handbook). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html
    - TypeScript 5.2: The Complete Production Developer Guide — quote block: "…stable 5.2 release; no later 5.2.x patch was ever published."
    - What's New in TypeScript 5.2 — paragraph 1: "…several type-system relaxations, new library typings and four breaking changes."
[^3]: npm registry — `typescript` package versions and publish times (5.2.0-beta, 5.2.1-rc, 5.2.2, 5.3.2, 6.0.3, 7.0.2, 7.1.0-beta). https://www.npmjs.com/package/typescript?activeTab=versions
    - TypeScript 5.2: The Complete Production Developer Guide — quote block: "…stable 5.2 release; no later 5.2.x patch was ever published."
    - TypeScript 5.2: The Complete Production Developer Guide — quote block: "…JavaScript-based line is 6.0 (6.0.3, 2026-04-16); 7.1.0-beta appeared on 2026-10-20."
[^4]: DefinitelyTyped README — support window ("only tests packages on versions of TypeScript that are less than 2 years old"). https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
    - TypeScript 5.2: The Complete Production Developer Guide — quote block: "…no longer guaranteed to compile under 5.2 (see Ecosystem compatibility)."
[^5]: TypeScript 5.0 release notes (decorators, const type parameters, bundler resolution, verbatimModuleSyntax, deprecations). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
    - What's New in TypeScript 5.2 — paragraph 3: "…functions, decoupled JSX element typing and namespaced JSX attributes (5.1)."
    - The 5.0 Deprecations (still accepted with `ignoreDeprecations: "5.0"`) — paragraph 1: "…errors unless `"ignoreDeprecations": "5.0"` is set; 5.5 removed them completely."
[^6]: TypeScript 5.1 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-1.html
    - What's New in TypeScript 5.2 — paragraph 3: "…functions, decoupled JSX element typing and namespaced JSX attributes (5.1)."
[^7]: TypeScript 5.3 release notes (import attributes, switch (true) narrowing). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-3.html
    - Version-Aware Review — Do Not Use or Recommend › Do not use or recommend (not available in 5.2) — bullet 1: "Interactive inlay hints."
[^8]: TypeScript 5.4 release notes (preserved narrowing in closures, NoInfer, Object.groupBy, module preserve). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html
    - Version-Aware Review — Do Not Use or Recommend › Do not use or recommend (not available in 5.2) — bullet 2: "`--module preserve`; checked import attributes."
[^9]: TypeScript 5.5 release notes (inferred type predicates, regex checking, isolatedDeclarations, removal of 5.0 deprecations). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
    - Version-Aware Review — Do Not Use or Recommend › Do not use or recommend (not available in 5.2) — bullet 3: "`target: "es2023"` (5.2 accepts only `es3`…`es2022` and `esnext` as targets)."
[^10]: TypeScript 5.6 release notes (truthy/nullish checks, iterator helpers, noUncheckedSideEffectImports). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html
    - Version-Aware Review — Do Not Use or Recommend › Do not use or recommend (not available in 5.2) — bullet 4: "`tsc --build` continuing past errors."
[^11]: TypeScript 5.7 release notes (rewriteRelativeImportExtensions, es2024, JSON imports under nodenext). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html
    - Version-Aware Review — Do Not Use or Recommend › Do not use or recommend (not available in 5.2) — bullet 5: "…and other ES2024 typings; the JSON-import attribute requirement under `nodenext`."
[^12]: TypeScript 5.8 and 5.9 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
    - Version-Aware Review — Do Not Use or Recommend › Do not use or recommend (not available in 5.2) — bullet 6: "`import defer`, `--module node20`, the minimal `tsc --init` (5.9)."
[^13]: Announcing TypeScript 6.0 / 7.0 (Microsoft DevBlogs). https://devblogs.microsoft.com/typescript/
    - Version-Aware Review — Do Not Use or Recommend › Do not use or recommend (not available in 5.2) — bullet 7: "None of this applies to a 5.2 build."
[^14]: ECMAScript Explicit Resource Management proposal. https://github.com/tc39/proposal-explicit-resource-management
    - Explicit Resource Management: `using` / `await using` [New in 5.2] › Semantics — paragraph 1: "…disposed in **reverse order of declaration** (last in, first out)."
[^15]: ECMAScript Decorator Metadata proposal. https://github.com/tc39/proposal-decorator-metadata
    - Decorator Metadata [New in 5.2] — paragraph 1: "…all decorators of that class, and exposed afterwards as `TheClass[Symbol.metadata]`."
[^16]: TSConfig reference (target, module, moduleResolution, useDefineForClassFields, esModuleInterop defaults). https://www.typescriptlang.org/tsconfig/
    - tsconfig.json: Defaults, Profiles and Strictness › Defaults in 5.2 you must override — paragraph 1: "Verified with `tsc` 5.2.2 (emit, `--traceResolution`) and the TSConfig reference."
    - tsconfig.json: Defaults, Profiles and Strictness › Defaults in 5.2 you must override — table row 6: "…`module` is `node16`/`nodenext` (verified in the 5.2 compiler source, `getESModuleInterop`)"
[^17]: Node.js ECMAScript modules — import attributes and JSON modules. https://nodejs.org/api/esm.html#import-attributes
    - Modules and Module Resolution › JSON modules: the 5.2 trap — paragraph 2: "…form that 5.2 accepts runs on Node.js 22 or later**"
[^18]: tslib releases (2.6.0 adds `__addDisposableResource`/`__disposeResources`). https://github.com/microsoft/tslib/releases
    - Ecosystem Compatibility of a 5.2 Toolchain — bullet 3: "…`using` needs `__addDisposableResource`/`__disposeResources`, which first appear in **tslib 2.6.0** (2023-06-26)."
[^19]: typescript-eslint — supported TypeScript versions. https://typescript-eslint.io/users/dependency-versions
    - Ecosystem Compatibility of a 5.2 Toolchain — bullet 2: "…2026) declares `typescript: ">=4.8.4 <6.1.0"` and therefore still supports 5.2."
[^20]: Zod documentation — requirements ("tested against TypeScript v5.5 and later"). https://zod.dev/
    - Types: Fundamentals › `any` vs `unknown` — paragraph 1: "…5.2 project may have to stay on an older major."
