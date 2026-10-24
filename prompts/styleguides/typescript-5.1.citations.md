### Citations: TypeScript 5.1: The Complete Production Developer Guide

Sources for `typescript-5.1.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: TypeScript 5.1 release notes (Handbook). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-1.html
    - TypeScript 5.1: The Complete Production Developer Guide — quote block: "…(2023-06-27; 5.1.4 was skipped) and the final patch **5.1.6 (2023-06-28)**."
    - What's New in TypeScript 5.1 — paragraph 1: "…the environment (the compiler's own Node.js requirement) and about `typeRoots`."
    - New in 5.1: Language and Checker Changes › Easier implicit returns for `undefined`-returning functions [New in 5.1] — paragraph 1: "…undefined;` (or `return;` plus the annotation). 5.1 relaxes three rules:"
    - New in 5.1: Language and Checker Changes › Unrelated types for getters and setters [New in 5.1] — paragraph 1: "…drops that requirement **when both accessors carry explicit type annotations**."
    - New in 5.1: JSX Changes › `JSX.ElementType` — decoupled checking of JSX tags [New in 5.1] — paragraph 1: "…while `JSX.Element` stays the type *produced* by a JSX expression."
    - New in 5.1: JSX Changes › Namespaced JSX attribute and tag names [New in 5.1] — paragraph 1: "…with a lowercase first segment are looked up in `JSX.IntrinsicElements["a:b"]`."
    - New in 5.1: Module Resolution and `typeRoots` › `typeRoots` are consulted in module resolution [New in 5.1] — paragraph 1: "…by `--traceResolution`: "Module name 'foo' was successfully resolved to …/types/foo/index.d.ts")."
    - New in 5.1: Module Resolution and `typeRoots` › Breaking: an explicit `typeRoots` no longer walks up to parent `node_modules/@types` [New in 5.1] — paragraph 1: "…parent directories looking in each `node_modules/@types`. 5.1 stops doing that."
    - New in 5.1: Editor Features, Performance, Runtime Requirement, `lib.d.ts` › Editor features [New in 5.1] — bullet 1: "VS Code needs `"editor.linkedEditing": true`."
    - New in 5.1: Editor Features, Performance, Runtime Requirement, `lib.d.ts` › Performance [New in 5.1] — paragraph 1: "…s to ~0.4 s), and scans JSDoc in larger chunks."
    - New in 5.1: Editor Features, Performance, Runtime Requirement, `lib.d.ts` › The compiler requires Node.js 14.17+ [New in 5.1] — paragraph 1: "Unexpected token '?'`, and npm/yarn print `EBADENGINE`/"engine incompatible" errors."
[^2]: npm registry metadata for `typescript` (publish times of 5.1.0-beta … 5.1.6, 5.2.2, 6.0.3, 7.0.2, 7.1.0-beta; dist-tags). https://registry.npmjs.org/typescript
    - TypeScript 5.1: The Complete Production Developer Guide — quote block: "…(2023-06-27; 5.1.4 was skipped) and the final patch **5.1.6 (2023-06-28)**."
    - TypeScript 5.1: The Complete Production Developer Guide — quote block: "…stable release) have shipped; 7.1.0-beta appeared on npm on 2026-10-20."
[^3]: DefinitelyTyped README — Support Window and `ts<version>` dist-tags. https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
    - TypeScript 5.1: The Complete Production Developer Guide — quote block: "…`@types/*` releases no longer compile with 5.1 (see Version-Aware Review)."
    - Version-Aware Review — Do Not Use or Recommend › Do not use or recommend (not available in 5.1) — bullet 9: "…iterator" on 5.1.6, while `@types/node@24.1.0` types `e` as `Dirent<string>` (verified)."
[^4]: TypeScript wiki — Breaking Changes (TypeScript 5.1) and API Breaking Changes. https://github.com/microsoft/TypeScript/wiki/Breaking-Changes#typescript-51
    - What's New in TypeScript 5.1 — paragraph 1: "…the environment (the compiler's own Node.js requirement) and about `typeRoots`."
    - New in 5.1: Editor Features, Performance, Runtime Requirement, `lib.d.ts` › Editor features [New in 5.1] — bullet 4: "…request and `LanguageService.getOccurrencesAtPosition` were removed; editor plugins must use `documentHighlights`/`getDocumentHighlights`."
[^5]: TypeScript 5.1.6 GitHub release and the "TypeScript 5.1.5" / "TypeScript 5.1.6" milestones (fixed issues). https://github.com/microsoft/TypeScript/releases/tag/v5.1.6 , https://github.com/microsoft/TypeScript/issues?q=milestone%3A%22TypeScript+5.1.5%22+is%3Aclosed , https://github.com/microsoft/TypeScript/issues?q=milestone%3A%22TypeScript+5.1.6%22+is%3Aclosed
    - What's New in TypeScript 5.1 — paragraph 2: "…regression in 5.1.3 and order-sensitive constraints of generic tuple types."
[^6]: TypeScript 5.0 release notes (decorators, const type parameters, bundler resolution, verbatimModuleSyntax, enums, deprecations). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
    - Version-Aware Review — Do Not Use or Recommend › Deprecated or removed older features (do not introduce in 5.1 code) — bullet 1: "…`suppressImplicitAnyIndexErrors`, `noStrictGenericChecks`, `charset`, `importsNotUsedAsValues`, `preserveValueImports`, and `prepend` in project references."
    - The 5.0 Foundation a 5.1 Codebase Builds On — paragraph 1: "…compiles on 5.1.6 and fails (or behaves differently) on 4.9.5."
[^7]: microsoft/TypeScript PR #53607 — Allow implicit return with explicit undefined return type (issue #36288). https://github.com/microsoft/TypeScript/pull/53607
    - New in 5.1: Language and Checker Changes › Easier implicit returns for `undefined`-returning functions [New in 5.1] — paragraph 1: "…undefined;` (or `return;` plus the annotation). 5.1 relaxes three rules:"
[^8]: microsoft/TypeScript PR #53417 — Allow unrelated types for getters and setters. https://github.com/microsoft/TypeScript/pull/53417
    - New in 5.1: Language and Checker Changes › Unrelated types for getters and setters [New in 5.1] — paragraph 1: "…drops that requirement **when both accessors carry explicit type annotations**."
[^9]: microsoft/TypeScript PR #51328 — `JSX.ElementType`. https://github.com/microsoft/TypeScript/pull/51328
    - New in 5.1: JSX Changes › `JSX.ElementType` — decoupled checking of JSX tags [New in 5.1] — paragraph 1: "…while `JSX.Element` stays the type *produced* by a JSX expression."
[^10]: microsoft/TypeScript PR #51715 — typeRoots in module resolution / no upward walk. https://github.com/microsoft/TypeScript/pull/51715
    - New in 5.1: Module Resolution and `typeRoots` › `typeRoots` are consulted in module resolution [New in 5.1] — paragraph 1: "…by `--traceResolution`: "Module name 'foo' was successfully resolved to …/types/foo/index.d.ts")."
[^11]: Node.js release schedule (end-of-life dates). https://github.com/nodejs/Release/blob/main/schedule.json
    - New in 5.1: Editor Features, Performance, Runtime Requirement, `lib.d.ts` › The compiler requires Node.js 14.17+ [New in 5.1] — paragraph 1: "…on 2026-04-30; build machines should run Node 22 or 24."
[^12]: microsoft/tslib PR #202 — Reverse order of decorator-injected initializers (released in tslib 2.5.1). https://github.com/microsoft/tslib/pull/202
    - New in 5.1: Editor Features, Performance, Runtime Requirement, `lib.d.ts` › Decorator initializer order [New in 5.1] — paragraph 2: "…2.5.1 (2023-05-17, "Reverse order of decorator-injected initializers") produces `"0AB"` (verified)."
[^13]: microsoft/TypeScript issue #53286 — "Property is used before its initialization" only reported for ESNext, not ES2022; fixed by PR #53885. https://github.com/microsoft/TypeScript/issues/53286
    - Classes and Class Fields › Parameter properties read in field initializers — a bug 5.1 does not report under `es2022` — paragraph 1: "…for `es2022` too (TypeScript issue #53286, fixed by PR #53885)."
[^14]: typescript-eslint — Dependency Versions; npm metadata for `@typescript-eslint/parser` 8.72.0. https://typescript-eslint.io/users/dependency-versions/
    - Tooling: Linters, Runners, Bundlers, Editors — bullet 1: "…with 5.1; it requires Node `^18.18.0 || ^20.9.0 || >=21.1.0`."
[^15]: TypeScript 5.2–5.9 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html (and the 5-3 … 5-9 pages)
    - Upgrading Beyond 5.1 — paragraph 2: "…to `latest`) → 6.0 (adopt new defaults explicitly) → 7.0."
[^16]: Announcing TypeScript 5.1 (Microsoft DevBlogs, 2023-06-01). https://devblogs.microsoft.com/typescript/announcing-typescript-5-1/
    - TypeScript 5.1: The Complete Production Developer Guide — quote block: "…(2023-06-27; 5.1.4 was skipped) and the final patch **5.1.6 (2023-06-28)**."
[^17]: Node.js documentation — Modules: TypeScript (type stripping) and ECMAScript modules (import attributes). https://nodejs.org/api/typescript.html , https://nodejs.org/api/esm.html#import-attributes
    - Modules and Module Resolution › ESM rules under `node16` / `nodenext` — bullet 5: "Node.js 22 removed `assert`"
    - Enums, Namespaces and Other Emitting Syntax › Namespaces, parameter properties and type-stripping runtimes — paragraph 1: "…`.ts` files directly by **stripping** types; they reject such syntax:"
[^18]: TSConfig reference — `useDefineForClassFields`, `verbatimModuleSyntax`, `moduleResolution`, `typeRoots`. https://www.typescriptlang.org/tsconfig/
    - Classes and Class Fields › `useDefineForClassFields` and `target` — paragraph 1: "…defaults to `false` and fields are emitted as constructor assignments."
[^19]: TC39 decorators proposal. https://github.com/tc39/proposal-decorators
    - Decorators: Standard (TC39) vs `experimentalDecorators` › Which system is in use — table row 1: "Spec | Stage 3 TC39 proposal"
