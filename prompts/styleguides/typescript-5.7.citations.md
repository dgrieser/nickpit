### Citations: TypeScript 5.7: The Complete Production Developer Guide

Sources for `typescript-5.7.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: TypeScript 5.7 Release Notes (TypeScript handbook). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html
    - TypeScript 5.7: The Complete Production Developer Guide — quote block: "…newest stable release (7.0.2, 2026-07-08)**. 7.1 is in beta (2026-10-20)."
    - Overview — paragraph 1: "TypeScript 5.7 is a small, mostly "plumbing" release."
    - What's New in TypeScript 5.7 › New in 5.7: Checks for Never-Initialized Variables — paragraph 1: "…reports the read when the variable is **never assigned anywhere**."
    - What's New in TypeScript 5.7 › New in 5.7: `--rewriteRelativeImportExtensions` — paragraph 2: "…declaration files) to `.js`, `.jsx`, `.mjs` or `.cjs` on emit."
    - What's New in TypeScript 5.7 › New in 5.7: `--rewriteRelativeImportExtensions` — paragraph 6: "…unscoped `"development"` condition collides with dependencies that ship their own."
    - What's New in TypeScript 5.7 › New in 5.7: Validated JSON Imports Under `--module nodenext` — paragraph 1: "Under `--module nodenext`, 5.7 enforces both rules in ESM files:"
    - What's New in TypeScript 5.7 › New in 5.7: Index Signatures From Non-Literal Computed Method Names — paragraph 1: "Previously the member was invisible to the type system."
    - What's New in TypeScript 5.7 › New in 5.7: Editor Project Ownership — bullet 1: "…pick a solution-style root config that references the right project."
    - What's New in TypeScript 5.7 › New in 5.7: V8 Compile Caching — paragraph 1: "…`tsc --version` dropping from about 122 ms to 48 ms."
    - What's New in TypeScript 5.7 › `lib.d.ts` (DOM) Updates — paragraph 1: "…for 5.7 and can introduce new errors in browser code."
[^2]: Announcing TypeScript 5.7 (Microsoft DevBlogs, 2024-11-22). https://devblogs.microsoft.com/typescript/announcing-typescript-5-7/
    - TypeScript 5.7: The Complete Production Developer Guide — quote block: "…newest stable release (7.0.2, 2026-07-08)**. 7.1 is in beta (2026-10-20)."
[^3]: typescript on npm — version history and publish times (5.7.2 2024-11-22, 5.7.3 2025-01-08, 5.8.2 2025-02-28, 5.9.2 2025-07-31, 6.0.2 2026-03-23, 6.0.3 2026-04-16, 7.0.2 2026-07-08, 7.1.0-beta 2026-10-20). https://www.npmjs.com/package/typescript?activeTab=versions ; GitHub release v5.7.3: https://github.com/microsoft/TypeScript/releases/tag/v5.7.3
    - TypeScript 5.7: The Complete Production Developer Guide — quote block: "…newest stable release (7.0.2, 2026-07-08)**. 7.1 is in beta (2026-10-20)."
[^4]: TSConfig Reference — strict and its sub-options, defaults for target/module/moduleResolution. https://www.typescriptlang.org/tsconfig/
    - Overview — bullet 27: "…`noImplicitAny`, `strictNullChecks`, `strictFunctionTypes`, `strictBindCallApply`, `strictPropertyInitialization`, `noImplicitThis`, `useUnknownInCatchVariables`, `alwaysStrict` and `strictBuiltinIteratorReturn`."
[^5]: microsoft/TypeScript PR #59767 — rewriteRelativeImportExtensions. https://github.com/microsoft/TypeScript/pull/59767
    - What's New in TypeScript 5.7 › New in 5.7: `--rewriteRelativeImportExtensions` — paragraph 2: "…declaration files) to `.js`, `.jsx`, `.mjs` or `.cjs` on emit."
[^6]: Node.js documentation — Modules: TypeScript (type stripping history, unsupported syntax, recommended tsconfig). https://nodejs.org/api/typescript.html
    - What's New in TypeScript 5.7 › New in 5.7: `--rewriteRelativeImportExtensions` — paragraph 8: "…`no-namespace`, `parameter-properties` with `prefer: "class-property"`, `no-require-imports`, plus `no-restricted-syntax` for `TSEnumDeclaration`."
    - tsconfig Recommendations for 5.7 › Running `.ts` Directly with Node.js (Type Stripping) on 5.7 — paragraph 1: "…runtime `namespace`, parameter properties, `import =`/`export =`, decorators, or `paths`."
[^7]: Node.js release schedule (EOL dates for 18, 20, 22, 24, 26). https://github.com/nodejs/Release/blob/main/schedule.json
    - What's New in TypeScript 5.7 › New in 5.7: `--target es2024` and `--lib es2024` — paragraph 4: "Flag that mismatch."
[^8]: Node.js documentation — Modules: ECMAScript modules, JSON modules and import attributes. https://nodejs.org/api/esm.html#json-modules
    - What's New in TypeScript 5.7 › New in 5.7: Validated JSON Imports Under `--module nodenext` — paragraph 1: "Under `--module nodenext`, 5.7 enforces both rules in ESM files:"
[^9]: Node.js documentation — module.enableCompileCache() and NODE_DISABLE_COMPILE_CACHE. https://nodejs.org/api/module.html#module-compile-cache
    - What's New in TypeScript 5.7 › New in 5.7: V8 Compile Caching — paragraph 1: "…opt out, for example on read-only or shared temp directories."
[^10]: TypeScript 5.8 Release Notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html
    - Do Not Use or Recommend › TypeScript 5.8 (2025-02-28) — section heading
[^11]: TypeScript 5.9 Release Notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
    - Do Not Use or Recommend › TypeScript 5.9 (2025-07-31) — section heading
[^12]: TypeScript 6.0 Release Notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html
    - Do Not Use or Recommend › TypeScript 6.0 (2026-03-23) — section heading
[^13]: Announcing TypeScript 7.0 (Microsoft DevBlogs, 2026-07-08). https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/
    - Do Not Use or Recommend › TypeScript 7.0 (2026-07-08) — paragraph 1: "…API (`@typescript/typescript6`), and a new API is expected in 7.1."
[^14]: typescript-eslint — supported TypeScript versions (8.16.0: `>=4.8.4 <5.8.0`) and shared configs. https://typescript-eslint.io/users/dependency-versions/ ; https://typescript-eslint.io/users/configs/
    - Do Not Use or Recommend › Version-Aware Review — bullet 5: "…releases print an "unsupported TypeScript version" warning and may misparse."
    - Tooling and Linting — bullet 1: "…with `tseslint.configs.strictTypeChecked` (or `recommendedTypeChecked`) and `parserOptions: { projectService: true }`."
[^15]: DefinitelyTyped README — Support window ("only tests packages on versions of TypeScript that are less than 2 years old"). https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
    - Do Not Use or Recommend › Version-Aware Review — bullet 6: "…tests `@types/*` only on TypeScript versions younger than two years."
[^16]: Angular version compatibility table (TypeScript ranges per Angular release). https://angular.dev/reference/versions
    - Removed / Changed in 5.7 — Migration Notes (5.6 → 5.7) — bullet 10: "…≥ 5.8, so an Angular upgrade forces a TypeScript upgrade."
