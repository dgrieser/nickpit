### Citations: TypeScript 5.6: The Complete Production Developer Guide

Sources for `typescript-5.6.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: TypeScript 5.6 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html
    - Overview — quote block: "…patch is **5.6.3 (published to npm on October 8, 2024)**."
    - Overview — paragraph 1: "…and `.tsbuildinfo` files written for every project in `--build` mode."
    - New in 5.6: Disallowed nullish and truthy checks — paragraph 1: "…operand of `??` always has the same truthiness or nullishness."
    - New in 5.6: Iterator helpers and `IteratorObject` — paragraph 1: "["esnext"]` (or `"esnext.iterator"`); there is no `es2025` lib in 5.6."
    - New in 5.6: Iterator helpers and `IteratorObject` — paragraph 2: "…`MapIterator<T>`, `SetIterator<T>`, `StringIterator<T>`, `RegExpStringIterator<T>`; generators return `Generator<…>`, which extends `IteratorObject`."
    - New in 5.6: `--strictBuiltinIteratorReturn` — paragraph 1: "…**`undefined` under `--strictBuiltinIteratorReturn`**, which is part of the `strict` family."
    - New in 5.6: Arbitrary module identifiers — paragraph 1: "…5.6 parses, checks and emits them (5.5 reports `Identifier expected`)."
    - New in 5.6: `--noUncheckedSideEffectImports` — paragraph 1: "The new flag makes it an error (TS2307)."
    - New in 5.6: `--noCheck` — paragraph 1: "…for all input files and still emits JavaScript (and declarations)."
    - New in 5.6: `--build` continues past errors — paragraph 1: "…continues building downstream projects**; the exit code is still non-zero."
    - New in 5.6: Module format detection in `node_modules` — paragraph 1: "…for files in `node_modules`, and `.mts`/`.cts` files are respected everywhere."
    - New in 5.6: `override` checks on computed members — paragraph 1: "Both lines compiled without error on 5.5."
    - New in 5.6: Editor features — paragraph 1: "These affect editors only, not `tsc` output:"
[^2]: Announcing TypeScript 5.6 (Microsoft DevBlogs, September 9, 2024). https://devblogs.microsoft.com/typescript/announcing-typescript-5-6/
    - Overview — quote block: "…patch is **5.6.3 (published to npm on October 8, 2024)**."
[^3]: npm registry — typescript package versions and publish dates (5.6.2: 2024-09-09, 5.6.3: 2024-10-08, 6.0.3: 2026-04-16, 7.0.2: 2026-07-08). https://www.npmjs.com/package/typescript?activeTab=versions
    - Overview — quote block: "…patch is **5.6.3 (published to npm on October 8, 2024)**."
    - Overview — quote block: "…July 8, 2026), which is the npm `latest` tag today."
[^4]: TypeScript 5.5 release notes (inferred type predicates, constant indexed access narrowing, regex checking, isolatedDeclarations, `${configDir}`, disabled 5.0 deprecations). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
    - Do not use or recommend (not in TypeScript 5.6) › Deprecated or removed as of 5.6 — bullet 1: "…`out` (use `outFile`), `prepend` in project references, implicitly OS-specific `newLine`."
    - Project configuration (tsconfig.json) › Shared base configs: `${configDir}` (5.5+) — paragraph 1: "…`paths`, `include` and similar resolve relative to the *extending* project:"
    - Narrowing and control flow › Inferred type predicates (5.5+) — paragraph 1: "`x => x !== undefined`."
    - Regular expression checking (5.5+) — paragraph 1: "…missing backreference groups, and flags or features newer than `target`:"
[^5]: TypeScript 5.7 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html
    - Do not use or recommend (not in TypeScript 5.6) — paragraph 1: "…silently do nothing in editors that bundle a newer TypeScript):"
[^6]: TypeScript 5.8 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html
    - Do not use or recommend (not in TypeScript 5.6) — paragraph 1: "…silently do nothing in editors that bundle a newer TypeScript):"
[^7]: TypeScript 5.9 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
    - Do not use or recommend (not in TypeScript 5.6) — paragraph 1: "…silently do nothing in editors that bundle a newer TypeScript):"
[^8]: TypeScript 5.0 release notes (decorators, const type parameters, verbatimModuleSyntax, bundler resolution, deprecations). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
    - Decorators — paragraph 1: "…and evaluation order and cannot be mixed in one project."
[^9]: TypeScript 6.0 release notes (new defaults, deprecations, preparing for 7.0). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html
    - Overview — quote block: "…July 8, 2026), which is the npm `latest` tag today."
    - New in 5.6: `--noUncheckedSideEffectImports` — paragraph 2: "It becomes the default in TypeScript 6.0."
    - Do not use or recommend (not in TypeScript 5.6) — paragraph 1: "…silently do nothing in editors that bundle a newer TypeScript):"
    - Do not use or recommend (not in TypeScript 5.6) › Future-proofing (works in 5.6, removed in 7.0) — paragraph 1: "…}` (use `with { … }`), and `/// <reference no-default-lib="true"/>`."
[^10]: Announcing TypeScript 7.0 (Microsoft DevBlogs, July 8, 2026). https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/
    - Overview — quote block: "…July 8, 2026), which is the npm `latest` tag today."
    - Do not use or recommend (not in TypeScript 5.6) — paragraph 1: "…silently do nothing in editors that bundle a newer TypeScript):"
[^11]: microsoft/TypeScript PR #59217 — Disallow truthy/nullish checks on syntactically never-nullish expressions. https://github.com/microsoft/TypeScript/pull/59217
    - New in 5.6: Disallowed nullish and truthy checks — paragraph 1: "…operand of `??` always has the same truthiness or nullishness."
[^12]: TC39 proposal: Iterator helpers. https://github.com/tc39/proposal-iterator-helpers
    - New in 5.6: Iterator helpers and `IteratorObject` — paragraph 1: "…a global `Iterator` constructor that can be subclassed, and `Iterator.from()`."
[^13]: microsoft/TypeScript PR #58222 — Iterator helper types (IteratorObject). https://github.com/microsoft/TypeScript/pull/58222
    - New in 5.6: Iterator helpers and `IteratorObject` — paragraph 2: "…`MapIterator<T>`, `SetIterator<T>`, `StringIterator<T>`, `RegExpStringIterator<T>`; generators return `Generator<…>`, which extends `IteratorObject`."
[^14]: DefinitelyTyped README — support window (packages are tested only on TypeScript versions less than two years old). https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
    - New in 5.6: Iterator helpers and `IteratorObject` › `@types/node` needs 5.6 now — paragraph 1: "…years old, and 5.6 left that window in September 2026."
[^15]: microsoft/TypeScript PR #58243 — BuiltinIteratorReturn and --strictBuiltinIteratorReturn. https://github.com/microsoft/TypeScript/pull/58243
    - New in 5.6: `--strictBuiltinIteratorReturn` — paragraph 1: "…**`undefined` under `--strictBuiltinIteratorReturn`**, which is part of the `strict` family."
[^16]: microsoft/TypeScript PR #58640 — Arbitrary module namespace identifiers. https://github.com/microsoft/TypeScript/pull/58640
    - New in 5.6: Arbitrary module identifiers — paragraph 1: "…5.6 parses, checks and emits them (5.5 reports `Identifier expected`)."
[^17]: microsoft/TypeScript PR #58941 — --noUncheckedSideEffectImports. https://github.com/microsoft/TypeScript/pull/58941
    - New in 5.6: `--noUncheckedSideEffectImports` — paragraph 1: "The new flag makes it an error (TS2307)."
[^18]: microsoft/TypeScript PR #58838 — Allow --build with intermediate errors (--stopBuildOnErrors). https://github.com/microsoft/TypeScript/pull/58838
    - New in 5.6: `--build` continues past errors — paragraph 1: "…continues building downstream projects**; the exit code is still non-zero."
[^19]: microsoft/TypeScript PR #58825 — Respect file extensions and package.json "type" in node_modules for all module modes. https://github.com/microsoft/TypeScript/pull/58825
    - New in 5.6: Module format detection in `node_modules` — paragraph 1: "…for files in `node_modules`, and `.mts`/`.cts` files are respected everywhere."
[^20]: TypeScript 5.4 release notes (preserved narrowing in closures, NoInfer, module preserve). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html
    - Narrowing and control flow › Narrowing in closures and after `await` — paragraph 1: "…created **after the last assignment** of a `let` or parameter:"
[^21]: TypeScript 5.2 release notes (`using` declarations, decorator metadata). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html
    - Explicit resource management: `using` and `await using` (5.2+) — paragraph 1: "…exception is thrown — the TypeScript form of `try/finally` cleanup."
[^22]: Node.js — Modules: TypeScript (type stripping); Node.js v22.18.0 release notes. https://nodejs.org/api/typescript.html , https://nodejs.org/en/blog/release/v22.18.0
    - Modules and module resolution › Node.js type stripping — paragraph 1: "…type checking; the feature is marked stable in Node 24.12/25.2."
[^23]: typescript-eslint — supported TypeScript versions (`SUPPORTED_TYPESCRIPT_VERSIONS = '>=4.8.4 <6.1.0'` in v8.72.0). https://typescript-eslint.io/users/dependency-versions
    - Tooling — bullet 1: "…v8 (8.72.0 supports TypeScript `>=4.8.4 <6.1.0`, so 5.6 is supported)."
[^24]: TSConfig reference. https://www.typescriptlang.org/tsconfig/
    - Project configuration (tsconfig.json) › Defaults in 5.6 you must override — paragraph 1: "Option semantics are documented in the TSConfig reference."
[^25]: TypeScript Modules reference (module, moduleResolution, nodenext, bundler, preserve). https://www.typescriptlang.org/docs/handbook/modules/reference.html
    - Modules and module resolution › Choose `module` by the runtime — paragraph 1: "…the Modules reference for the exact rules of each mode."
