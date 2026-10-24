### Citations: TypeScript 5.4: The Complete Production Developer Guide

Sources for `typescript-5.4.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: TypeScript 5.4 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html
    - TypeScript 5.4: The Complete Production Developer Guide — quote block: "…the latest and final patch is **5.4.5 (April 10, 2024)**."
    - What's New in TypeScript 5.4 — paragraph 1: "Its changes against 5.3:"
    - What's New in TypeScript 5.4 › Deprecated and on the way out — bullet 1: "…notes suggest) setting them is error TS5102 "has been removed"."
    - New in 5.4: Preserved Narrowing in Closures After the Last Assignment — paragraph 1: "…and the variable is **not assigned inside any nested function**."
    - New in 5.4: The `NoInfer<T>` Utility Type — paragraph 1: "…a type parameter and another must merely conform to it."
    - New in 5.4: `Object.groupBy`, `Map.groupBy` and `Promise.withResolvers` — paragraph 1: "…no `es2024` target or lib in 5.4 (added in 5.7)."
    - New in 5.4: `--module preserve` and `require()` with `--moduleResolution bundler` — paragraph 1: "…the `"import"` or `"require"` condition of the dependency's `package.json` `"exports"`."
    - New in 5.4: Checked Import Attributes — paragraph 1: "Runtimes or projects can declare which attributes they accept:"
    - New in 5.4: Behavioural Changes That Can Break a Build — paragraph 1: "…from 5.3 can surface new errors (or remove old ones)."
    - New in 5.4: Behavioural Changes That Can Break a Build › Emit changes — paragraph 1: "…attributes in `import()` types go through the normal emit pipeline."
    - Modules and Module Resolution › Subpath imports (`package.json` `"imports"`) instead of `paths` — paragraph 1: "…in 5.4: auto-import completions in editors suggest these `#...` specifiers."
    - Tooling › Compiler and editor — bullet 3: "…optional parameters" quick fixes for calls with too many arguments."
[^2]: npm registry metadata for `typescript` (publish times of 5.4.2–5.4.5, 5.9.3, 6.0.3, 7.0.2; dist-tags). https://registry.npmjs.org/typescript
    - TypeScript 5.4: The Complete Production Developer Guide — quote block: "…the latest and final patch is **5.4.5 (April 10, 2024)**."
    - TypeScript 5.4: The Complete Production Developer Guide — quote block: "…on npm); 7.1 is in beta (7.1.0-beta, October 20, 2026)."
[^3]: DefinitelyTyped README — Support Window ("only tests packages on versions of TypeScript that are less than 2 years old"). https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
    - TypeScript 5.4: The Complete Production Developer Guide — quote block: "…`@types` releases may no longer compile with it (see Tooling)."
    - Tooling › `@types` packages: pin versions that still support 5.4 — paragraph 1: "…`typeScriptVersion`, and stops testing compiler versions older than two years."
[^4]: Announcing TypeScript 5.4 (March 6, 2024; includes auto-import support for subpath imports). https://devblogs.microsoft.com/typescript/announcing-typescript-5-4/
    - What's New in TypeScript 5.4 — paragraph 1: "Its changes against 5.3:"
[^5]: microsoft/TypeScript PR #57853 "Revert PR 56161" (milestone TypeScript 5.4.3, cherry-picked to release-5.4) and PR #56161. https://github.com/microsoft/TypeScript/pull/57853 , https://github.com/microsoft/TypeScript/pull/56161
    - What's New in TypeScript 5.4 — paragraph 2: "…#57853), so 5.4.3–5.4.5 accept those names again, exactly like 5.3."
[^6]: TypeScript 5.5 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
    - What's New in TypeScript 5.4 › Do not use or recommend (not available in 5.4) — paragraph 1: "…does not compile, or the option is rejected as unknown):"
[^7]: TypeScript 5.5 release notes — Disabling Features Deprecated in TypeScript 5.0; TypeScript issue #51909 (deprecation plan). https://github.com/microsoft/TypeScript/issues/51909
    - What's New in TypeScript 5.4 › Do not use or recommend (not available in 5.4) — paragraph 1: "…does not compile, or the option is rejected as unknown):"
    - What's New in TypeScript 5.4 › Deprecated and on the way out — bullet 1: "…notes suggest) setting them is error TS5102 "has been removed"."
[^8]: TypeScript 5.6 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html
    - What's New in TypeScript 5.4 › Do not use or recommend (not available in 5.4) — paragraph 1: "…does not compile, or the option is rejected as unknown):"
[^9]: TypeScript 5.7 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html
    - What's New in TypeScript 5.4 › Do not use or recommend (not available in 5.4) — paragraph 1: "…does not compile, or the option is rejected as unknown):"
[^10]: TypeScript 5.8 and 5.9 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
    - What's New in TypeScript 5.4 › Do not use or recommend (not available in 5.4) — paragraph 1: "…does not compile, or the option is rejected as unknown):"
[^11]: TypeScript 6.0 release notes (new defaults, deprecations, preparing for 7.0). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html
    - What's New in TypeScript 5.4 › Do not use or recommend (not available in 5.4) — paragraph 1: "…does not compile, or the option is rejected as unknown):"
    - What's New in TypeScript 5.4 › Deprecated and on the way out — bullet 2: "…es5`, `downlevelIteration`, `module` `amd`/`umd`/`system`, `outFile` and the `assert` import syntax."
[^12]: microsoft/TypeScript PR #56908 — Preserve narrowing in closures following last assignment. https://github.com/microsoft/TypeScript/pull/56908
    - New in 5.4: Preserved Narrowing in Closures After the Last Assignment — paragraph 1: "…and the variable is **not assigned inside any nested function**."
[^13]: microsoft/TypeScript PR #56794 — NoInfer intrinsic type. https://github.com/microsoft/TypeScript/pull/56794
    - New in 5.4: The `NoInfer<T>` Utility Type — paragraph 1: "…a type parameter and another must merely conform to it."
[^14]: microsoft/TypeScript PR #56805 — groupBy declarations; MDN Object.groupBy / Map.groupBy / Promise.withResolvers. https://github.com/microsoft/TypeScript/pull/56805 , https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Object/groupBy
    - New in 5.4: `Object.groupBy`, `Map.groupBy` and `Promise.withResolvers` — paragraph 1: "…no `es2024` target or lib in 5.4 (added in 5.7)."
[^15]: microsoft/TypeScript PR #56785 — `--module preserve`. https://github.com/microsoft/TypeScript/pull/56785
    - New in 5.4: `--module preserve` and `require()` with `--moduleResolution bundler` — paragraph 1: "…the `"import"` or `"require"` condition of the dependency's `package.json` `"exports"`."
[^16]: microsoft/TypeScript PR #56034 — Check import attributes against the global ImportAttributes type. https://github.com/microsoft/TypeScript/pull/56034
    - New in 5.4: Checked Import Attributes — paragraph 1: "Runtimes or projects can declare which attributes they accept:"
[^17]: microsoft/TypeScript PR #57027 — DOM lib updates for 5.4. https://github.com/microsoft/TypeScript/pull/57027
    - New in 5.4: Behavioural Changes That Can Break a Build › `lib.d.ts` — paragraph 1: "…that declared these APIs itself may now see duplicate-declaration errors."
[^18]: Zod documentation — Requirements ("tested against TypeScript v5.5 and later"). https://zod.dev/
    - Runtime Validation and Data Boundaries — paragraph 2: "…states it is tested against TypeScript 5.5 and later only."
[^19]: typescript-eslint — supported TypeScript versions (peerDependencies of typescript-eslint 8.72.0 on npm). https://typescript-eslint.io/users/dependency-versions/
    - Tooling › Linting: typescript-eslint — paragraph 1: "…<6.1.0` (peer dependency of 8.72.0), so it works with 5.4."
[^20]: Node.js 22.18.0 release notes (type stripping enabled by default) and Node.js TypeScript documentation. https://nodejs.org/en/blog/release/v22.18.0 , https://nodejs.org/api/typescript.html
    - Tooling › Running TypeScript without a build step — bullet 2: "`--experimental-transform-types` (22.7+) was removed in Node.js 26."
[^21]: Node.js ECMAScript modules documentation — `import.meta.dirname` / `import.meta.filename` (added in v21.2.0, v20.11.0), import attributes, CommonJS interop. https://nodejs.org/api/esm.html
    - Modules and Module Resolution › File format under `nodenext` — bullet 5: "Use `import.meta.dirname`/`import.meta.filename`"
