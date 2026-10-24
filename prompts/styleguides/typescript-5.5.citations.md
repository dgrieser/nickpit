### Citations: TypeScript 5.5: The Complete Production Developer Guide

Sources for `typescript-5.5.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: TypeScript 5.5 release notes (Microsoft). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html ; Announcing TypeScript 5.5 (June 20, 2024). https://devblogs.microsoft.com/typescript/announcing-typescript-5-5/
    - TypeScript 5.5: The Complete Production Developer Guide — quote block: "…latest and final patch release is **5.5.4 (July 22, 2024)**."
    - What's New in TypeScript 5.5 — paragraph 1: "Its headline changes are:"
    - What's New in TypeScript 5.5 › New in 5.5: Inferred Type Predicates — paragraph 1: "…a refinement of a parameter now gets a type-predicate signature."
    - What's New in TypeScript 5.5 › New in 5.5: Control-Flow Narrowing for Constant Indexed Accesses — paragraph 1: "…and `key` are effectively constant (parameters or `const`s never reassigned)."
    - What's New in TypeScript 5.5 › New in 5.5: The JSDoc `@import` Tag — paragraph 1: "…import syntax inside a JSDoc comment, with no runtime import:"
    - What's New in TypeScript 5.5 › New in 5.5: Regular-Expression Syntax Checking — paragraph 1: "`new RegExp("...")` strings are not."
    - What's New in TypeScript 5.5 › New in 5.5: ECMAScript `Set` Methods — paragraph 1: "["esnext"]` or `"esnext.collection"`."
    - What's New in TypeScript 5.5 › New in 5.5: `--isolatedDeclarations` — paragraph 1: "…(oxc, swc, the `transpileDeclaration` API) generate `.d.ts` files in parallel."
    - What's New in TypeScript 5.5 › New in 5.5: The `${configDir}` Template Variable — paragraph 1: "…being compiled — the *extending* config, not the shared base."
    - What's New in TypeScript 5.5 › New in 5.5: Compiler API, Editor and Performance Changes — paragraph 1: "…what code compiles, but they matter when reviewing build tooling:"
    - What's New in TypeScript 5.5 › New in 5.5: Behaviour Changes That Break Builds — paragraph 1: "Verified behaviour on 5.5.4:"
    - What's New in TypeScript 5.5 › New in 5.5: Behaviour Changes That Break Builds — paragraph 8: "…typings were regenerated; expect occasional new errors in DOM-heavy code."
    - Performance (Compiler and Runtime) › Type-Checking Performance — bullet 7: "…checking — roughly 2x faster in the TypeScript team's tests."
[^2]: npm registry, `typescript` package version history and dist-tags (`npm view typescript time`). https://www.npmjs.com/package/typescript?activeTab=versions ; GitHub release v5.5.4. https://github.com/microsoft/TypeScript/releases/tag/v5.5.4
    - TypeScript 5.5: The Complete Production Developer Guide — quote block: "…latest and final patch release is **5.5.4 (July 22, 2024)**."
    - TypeScript 5.5: The Complete Production Developer Guide — quote block: "…7.1 is in beta (`7.1.0-beta` on the npm `beta` tag)."
    - Toolchain Compatibility and Release Facts — table row 1: "…**5.5.2 (June 20)**, 5.5.3 (July 1), **5.5.4 (July 22, 2024)**"
[^3]: Announcing TypeScript 7.0 / TypeScript 7.0 released (July 2026). https://devblogs.microsoft.com/typescript/ ; InfoWorld, "Go-based TypeScript 7.0 arrives". https://www.infoworld.com/article/4196378/go-based-typescript-7-0-arrives.html
    - TypeScript 5.5: The Complete Production Developer Guide — quote block: "…7.1 is in beta (`7.1.0-beta` on the npm `beta` tag)."
[^4]: DefinitelyTyped README — Support Window and `ts<version>` dist-tags. https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
    - TypeScript 5.5: The Complete Production Developer Guide — quote block: "Pin `@types/*` to their `ts5.5` dist-tags (see Toolchain Compatibility)."
[^5]: TypeScript 5.0–5.4 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html (and the linked 5.0–5.3 pages)
    - What's New in TypeScript 5.5 — paragraph 2: "…the `NoInfer<T>` utility type, `Object.groupBy`/`Map.groupBy` typings and `--module preserve` (5.4)."
[^6]: microsoft/TypeScript PR #57465 — Infer type predicates from function bodies; Dan Vanderkam, "The Making of a TypeScript Feature: Inferring Type Predicates". https://github.com/microsoft/TypeScript/pull/57465 , https://effectivetypescript.com/2024/04/16/inferring-a-type-predicate/
    - What's New in TypeScript 5.5 › New in 5.5: Inferred Type Predicates — paragraph 1: "…a refinement of a parameter now gets a type-predicate signature."
[^7]: TC39 proposal-set-methods (ES2025). https://github.com/tc39/proposal-set-methods
    - What's New in TypeScript 5.5 › New in 5.5: ECMAScript `Set` Methods — paragraph 1: "["esnext"]` or `"esnext.collection"`."
[^8]: Isolated Declarations: State of the Feature (microsoft/TypeScript #58944). https://github.com/microsoft/TypeScript/issues/58944
    - What's New in TypeScript 5.5 › New in 5.5: `--isolatedDeclarations` — paragraph 1: "…(oxc, swc, the `transpileDeclaration` API) generate `.d.ts` files in parallel."
[^9]: microsoft/TypeScript PR #58042 — `${configDir}` substitution in tsconfig. https://github.com/microsoft/TypeScript/pull/58042
    - What's New in TypeScript 5.5 › New in 5.5: The `${configDir}` Template Variable — paragraph 1: "…being compiled — the *extending* config, not the shared base."
[^10]: TypeScript Flag Deprecation Plan (#51000) and 5.0 deprecations (#51909). https://github.com/microsoft/TypeScript/issues/51000 , https://github.com/microsoft/TypeScript/issues/51909
    - What's New in TypeScript 5.5 › New in 5.5: Behaviour Changes That Break Builds — paragraph 1: "Verified behaviour on 5.5.4:"
[^11]: TypeScript 5.6, 5.7, 5.8, 5.9 and 6.0 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html
    - Do Not Use or Recommend — paragraph 1: "…fail on 5.5.4 and to work on the listed version):"
[^12]: Node.js documentation — Modules: TypeScript (type stripping history and limitations). https://nodejs.org/api/typescript.html
    - Toolchain Compatibility and Release Facts — table row 7: "…5.8+ with `erasableSyntaxOnly` and `rewriteRelativeImportExtensions`, which 5.5 lacks (see Tooling)."
    - Tooling, Linting and Formatting — bullet 6: "…enums, value namespaces, parameter properties, `import =` aliases and decorators."
[^13]: TypeScript handbook — Modules reference (module, moduleResolution, nodenext, bundler). https://www.typescriptlang.org/docs/handbook/modules/reference.html
    - Modules and Module Resolution › Choosing `module`/`moduleResolution` — paragraph 1: "…imports are resolved and which output format each file gets."
[^14]: TSConfig reference. https://www.typescriptlang.org/tsconfig/
    - tsconfig for 5.5 › Defaults You Must Not Assume — paragraph 1: "…the TSConfig reference for 5.5, and were checked by compiling."
[^15]: typescript-eslint — supported TypeScript versions and rules. https://typescript-eslint.io/users/dependency-versions/ , https://typescript-eslint.io/rules/
    - Tooling, Linting and Formatting — bullet 2: "**typescript-eslint v8** supports 5.5."
