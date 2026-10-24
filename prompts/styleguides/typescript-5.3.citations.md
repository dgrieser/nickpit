### Citations: TypeScript 5.3: The Complete Production Developer Guide

Sources for `typescript-5.3.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: TypeScript 5.3 release notes (Handbook). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-3.html
    - Overview — quote block: "…patch is **5.3.3 (published to npm on December 6, 2023)**."
    - Overview — paragraph 1: "…non-normalized intersections, and `tsserverlibrary.js` reduced to a re-export of `typescript.js`."
    - New in 5.3: Import attributes — paragraph 1: "…a module is loaded, not just to check it afterwards."
    - New in 5.3: `resolution-mode` in import types and all module modes — paragraph 1: "…<reference types="pkg" resolution-mode="require" />`; 5.3 makes the import-attribute form stable:"
    - New in 5.3: `switch (true)` narrowing — paragraph 1: "…`default` receives the type narrowed by all cases that exited."
    - New in 5.3: Narrowing on comparisons to booleans — paragraph 1: "…type-guard call or a boolean expression with `true`/`false` now narrows."
    - New in 5.3: `instanceof` narrowing through `Symbol.hasInstance` — paragraph 1: "…by that predicate instead of to the class instance type."
    - New in 5.3: `super` property access checks — paragraph 1: "…at runtime. 5.3 reports this as **TS2855**; 5.2 accepted it."
    - New in 5.3: Editor, compiler and API changes — bullet 1: "…jump to the type's definition (needs a 5.3-based editor TypeScript)."
    - New in 5.3: Editor, compiler and API changes — bullet 2: "`autoImportSpecifierExcludeRegexes` is 5.6 and not available."
    - New in 5.3: Editor, compiler and API changes — bullet 3: "…language-service host; 5.3.3 also exposed it to `transpileModule` (issue #56625)."
    - New in 5.3: Editor, compiler and API changes — bullet 4: "…`T` use the original intersection form as a fast path."
    - New in 5.3: Editor, compiler and API changes — bullet 5: "Package size dropped by about 20 %."
[^2]: TypeScript 5.3.3 release on GitHub (published December 6, 2023). https://github.com/microsoft/TypeScript/releases/tag/v5.3.3
    - Overview — quote block: "…patch is **5.3.3 (published to npm on December 6, 2023)**."
    - TypeScript 5.3.x patch releases — paragraph 1: "Always pin `~5.3.3` (or exactly `5.3.3`), never `5.3.2`."
[^3]: npm registry metadata for `typescript` (version publish times and dist-tags: 5.3.2 2023-11-20, 5.3.3 2023-12-06, 5.4.2 2024-03-06, 6.0.2 2026-03-23, 6.0.3 2026-04-16, 7.0.2 2026-07-08). https://www.npmjs.com/package/typescript?activeTab=versions
    - Overview — quote block: "…patch is **5.3.3 (published to npm on December 6, 2023)**."
    - Overview — quote block: "…is the npm `latest` tag today (7.1 is in beta)."
[^4]: DefinitelyTyped README — Support window ("only tests packages on versions of TypeScript that are less than 2 years old"); `@types/node` package metadata (`typeScriptVersion`, `typesVersions`, dist-tag `ts5.3`). https://github.com/DefinitelyTyped/DefinitelyTyped#support-window , https://www.npmjs.com/package/@types/node
    - Overview — quote block: "…June 2026 new `@types/node` releases require TypeScript 5.6 (see [Tooling](#tooling-and-type-packages))."
    - Tooling and type packages › `@types/node` and other DefinitelyTyped packages — paragraph 1: "…fails with TS2504 "Type 'Dir' must have a '[Symbol.asyncIterator]()' method"."
[^5]: Announcing TypeScript 5.3 (Microsoft DevBlogs, November 20, 2023). https://devblogs.microsoft.com/typescript/announcing-typescript-5-3/
    - Overview — paragraph 1: "…non-normalized intersections, and `tsserverlibrary.js` reduced to a re-export of `typescript.js`."
[^6]: TC39 proposal: Import Attributes; TypeScript implementation PR #54242. https://github.com/tc39/proposal-import-attributes , https://github.com/microsoft/TypeScript/pull/54242
    - New in 5.3: Import attributes — paragraph 1: "…a module is loaded, not just to check it afterwards."
[^7]: `resolution-mode` in import types and all module modes, PR #55725. https://github.com/microsoft/TypeScript/pull/55725
    - New in 5.3: `resolution-mode` in import types and all module modes — paragraph 1: "…<reference types="pkg" resolution-mode="require" />`; 5.3 makes the import-attribute form stable:"
[^8]: `switch (true)` narrowing, PR #55991. https://github.com/microsoft/TypeScript/pull/55991
    - New in 5.3: `switch (true)` narrowing — paragraph 1: "…`default` receives the type narrowed by all cases that exited."
[^9]: TypeScript 5.3.3 milestone issues: #56482 (boolean comparison narrowing regression), #56478, #56477, #56625. https://github.com/microsoft/TypeScript/issues?q=milestone%3A%22TypeScript+5.3.3%22
    - New in 5.3: Narrowing on comparisons to booleans — paragraph 2: "…"possibly undefined" errors; it was fixed in **5.3.3** (issue #56482)."
    - New in 5.3: Editor, compiler and API changes — bullet 3: "…language-service host; 5.3.3 also exposed it to `transpileModule` (issue #56625)."
    - New in 5.3: Editor, compiler and API changes — bullet 6: "…under `exactOptionalPropertyTypes`; both were fixed in 5.3.3 (issues #56477, #56478)."
    - TypeScript 5.3.x patch releases — paragraph 1: "Always pin `~5.3.3` (or exactly `5.3.3`), never `5.3.2`."
[^10]: `instanceof` narrowing through `Symbol.hasInstance`, PR #55052. https://github.com/microsoft/TypeScript/pull/55052
    - New in 5.3: `instanceof` narrowing through `Symbol.hasInstance` — paragraph 1: "…by that predicate instead of to the class instance type."
[^11]: Checks for `super` property accesses on instance fields, PR #54056. https://github.com/microsoft/TypeScript/pull/54056
    - New in 5.3: `super` property access checks — paragraph 1: "…at runtime. 5.3 reports this as **TS2855**; 5.2 accepted it."
[^12]: TypeScript wiki — API Breaking Changes (5.3: `tsserverlibrary.js` is a thin wrapper around `typescript.js`). https://github.com/microsoft/TypeScript/wiki/API-Breaking-Changes
    - New in 5.3: Editor, compiler and API changes — bullet 5: "Package size dropped by about 20 %."
[^13]: TypeScript 5.4 release notes (NoInfer, preserved narrowing in closures, Object.groupBy, --module preserve, notable behavioral changes). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html
    - Do not use or recommend (not in TypeScript 5.3) — paragraph 1: "…5.3.3 (error) and, for the 5.4 items, on 5.4.5 (accepted)."
    - Narrowing (cumulative to 5.3) › Closures and narrowing in 5.3 (no "preserved narrowing") — paragraph 1: "**5.3 does not**."
[^14]: Announcing TypeScript 5.4. https://devblogs.microsoft.com/typescript/announcing-typescript-5-4/
    - Do not use or recommend (not in TypeScript 5.3) — paragraph 1: "…5.3.3 (error) and, for the 5.4 items, on 5.4.5 (accepted)."
    - Removed / changed in 5.3 — migration notes — bullet 9: "…last release where `ignoreDeprecations: "5.0"` keeps the 5.0-deprecated options working."
[^15]: TypeScript 5.0 release notes — Deprecations and Default Changes; Flag Deprecation Plan #51000. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html , https://github.com/microsoft/TypeScript/issues/51000
    - Deprecated in 5.3: the 5.0 deprecation set — paragraph 1: "…still reports them unless `ignoreDeprecations` is set; verified on 5.5.4)."
[^16]: TypeScript Handbook — Decorators; TC39 decorators proposal. https://www.typescriptlang.org/docs/handbook/decorators.html , https://github.com/tc39/proposal-decorators
    - Classes › Decorators: standard (5.0) versus `experimentalDecorators` — paragraph 1: "…libraries of both kinds in one project does not work."
[^17]: TypeScript 5.2 release notes (using declarations, decorator metadata, union array methods, tuple labels). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html
    - Classes › Explicit resource management: `using` and `await using` (5.2) — paragraph 1: "`await using` awaits `[Symbol.asyncDispose]()`."
[^18]: DOM lib updates for TypeScript 5.3, PR #55798. https://github.com/microsoft/TypeScript/pull/55798
    - Removed / changed in 5.3 — migration notes — bullet 6: "…`trackIdentifier`); test doubles that construct such objects may need updates."
[^19]: TypeScript 5.5 release notes (Disabling Features Deprecated in TypeScript 5.0, inferred type predicates, regex checking). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
    - Deprecated in 5.3: the 5.0 deprecation set — paragraph 1: "…still reports them unless `ignoreDeprecations` is set; verified on 5.5.4)."
    - Removed / changed in 5.3 — migration notes — bullet 10: "`undefined` no longer definable as a type name."
[^20]: Total TypeScript — "The TypeScript 5.3 feature they didn't tell you about" (const type parameters and `satisfies` with mutable array constraints). https://www.totaltypescript.com/the-typescript-5-3-feature-they-didn-t-tell-you-about
    - `satisfies`, `as const` and `const` type parameters — bullet 2: "…readonly — that form works the same on every version."
