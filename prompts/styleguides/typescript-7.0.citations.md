### Citations: TypeScript 7.0: The Complete Production Developer Guide

Sources for `typescript-7.0.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Announcing TypeScript 7.0 (July 8, 2026) — defaults, removed options, `--checkers`/`--builders`/`--singleThreaded`, `@typescript/typescript6`, JavaScript differences, editor status. https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/
    - TypeScript 7.0: The Complete Production Developer Guide — quote block: "…tag `7.0.0-dev.20260421.2`, April 21, 2026), not as a `typescript` version."
    - TypeScript 7.0: The Complete Production Developer Guide — paragraph 1: "…a rebuilt `--watch` mode and a new LSP-based language server."
    - TypeScript 7.0: The Complete Production Developer Guide — bullet 1: "…are rejected — and `"ignoreDeprecations": "6.0"` no longer silences anything."
    - TypeScript 7.0: The Complete Production Developer Guide — bullet 2: "…side by side; the new API is planned for 7.1."
    - TypeScript 7.0: The Complete Production Developer Guide — bullet 4: "…declaration conflicts are reported at every site even under `skipLibCheck`."
    - What's New in TypeScript 7.0 › The native compiler and its command line (New in 7.0) — bullet 1: "…identically, so results are deterministic for a given worker count."
    - What's New in TypeScript 7.0 › The native compiler and its command line (New in 7.0) — bullet 2: "…pin the value if local and CI results must match."
    - What's New in TypeScript 7.0 › The native compiler and its command line (New in 7.0) — bullet 3: "Changing it must not change results."
    - What's New in TypeScript 7.0 › The native compiler and its command line (New in 7.0) — bullet 4: "…(debugging, constrained CI, or when an outer tool already parallelises)."
    - What's New in TypeScript 7.0 › The native compiler and its command line (New in 7.0) — bullet 6: "…Go port of the Parcel file watcher, replacing polling-heavy watching."
    - What's New in TypeScript 7.0 › The native compiler and its command line (New in 7.0) — bullet 7: "…is still accepted by `tsc` but nothing loads the plugins."
    - What's New in TypeScript 7.0 › Removed options and syntax (hard errors in 7.0) — paragraph 1: "…by `"ignoreDeprecations": "6.0"`) and 7.0.2 (error that `ignoreDeprecations` cannot silence):"
    - What's New in TypeScript 7.0 › Type-checking differences versus 6.0 (New in 7.0) — paragraph 1: "…under 6.0 *with `stableTypeOrdering` on and no `ignoreDeprecations`* compiles identically."
    - What's New in TypeScript 7.0 › Type-checking differences versus 6.0 (New in 7.0) — bullet 1: "…(a `Len<"😀">` utility gives `1` in 7.0, `2` in 6.0)."
    - What's New in TypeScript 7.0 › The compiler API is gone (New in 7.0) — paragraph 1: "…6.0 under the `typescript` name and 7.0 under another name:"
    - What's New in TypeScript 7.0 › The compiler API is gone (New in 7.0) — table row 4: "Vue/Svelte/Astro/MDX: stay on 6.0 for now."
    - Do Not Use or Recommend › Features from 7.1 and later (not in 7.0) — paragraph 2: "…pre-release channel; nightlies are now `typescript@next` (currently 7.1 dev builds)."
    - JSX (React and others) — bullet 5: "…TypeScript 6.0 tooling (their language tools need the 6.0 API)."
    - Tooling: Linting, Testing, Running › Linting — bullet 1: "…agrees with 7.0 for code that is clean under `stableTypeOrdering`."
    - Tooling: Linting, Testing, Running › Editors — paragraph 1: "…(or the reverse), check which TypeScript version each side runs."
[^2]: npm registry, `typescript` package versions and publish times (6.0.2 2026-03-23, 6.0.3 2026-04-16, 7.0.1-rc 2026-06-18, 7.0.2 2026-07-08, 7.1.0-beta 2026-10-20). https://www.npmjs.com/package/typescript?activeTab=versions
    - TypeScript 7.0: The Complete Production Developer Guide — quote block: "…tag `7.0.0-dev.20260421.2`, April 21, 2026), not as a `typescript` version."
    - TypeScript 7.0: The Complete Production Developer Guide — quote block: "…— nothing from 7.1 is available in a 7.0 project."
[^3]: Announcing TypeScript 6.0 (March 23, 2026) — es2025 target/lib, Temporal, getOrInsert, `#/` imports, default changes and deprecations. https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/
    - TypeScript 7.0: The Complete Production Developer Guide — quote block: "…— nothing from 7.1 is available in a 7.0 project."
    - TypeScript 7.0: The Complete Production Developer Guide — bullet 1: "…are rejected — and `"ignoreDeprecations": "6.0"` no longer silences anything."
    - What's New in TypeScript 7.0 › Removed options and syntax (hard errors in 7.0) — paragraph 1: "…by `"ignoreDeprecations": "6.0"`) and 7.0.2 (error that `ignoreDeprecations` cannot silence):"
    - What's New in TypeScript 7.0 › Inherited from 6.0 — new to anyone coming from 5.x — bullet 1: "…(`Iterator.prototype.map/filter/take/drop/flatMap/reduce/toArray/…`), `Set` methods (`union`, `intersection`, `difference`, `symmetricDifference`, `isSubsetOf`, …), `Float16Array`/`Math.f16round`."
    - What's New in TypeScript 7.0 › Inherited from 6.0 — new to anyone coming from 5.x — bullet 2: "…`Array.fromAsync` and `Disposable`/`AsyncDisposable`/`Symbol.dispose` (the disposable declarations are **not** in `es2025`)."
    - What's New in TypeScript 7.0 › Inherited from 6.0 — new to anyone coming from 5.x — bullet 3: "…and `dom.asynciterable`**; those lib names still exist but are empty."
    - What's New in TypeScript 7.0 › Inherited from 6.0 — new to anyone coming from 5.x — bullet 4: "…use `#/` in code that must run on Node.js 22."
    - What's New in TypeScript 7.0 › Inherited from 6.0 — new to anyone coming from 5.x — bullet 5: "TS5095)."
    - What's New in TypeScript 7.0 › Inherited from 6.0 — new to anyone coming from 5.x — bullet 6: "TS18046 "'y' is of type 'unknown'")."
[^4]: Announcing TypeScript 7.1 Beta (October 20, 2026). https://devblogs.microsoft.com/typescript/announcing-typescript-7-1-beta/
    - TypeScript 7.0: The Complete Production Developer Guide — quote block: "…— nothing from 7.1 is available in a 7.0 project."
    - What's New in TypeScript 7.0 › The native compiler and its command line (New in 7.0) — bullet 9: "…the API under `typescript/sync`, `typescript/async`, `typescript/ast`, `typescript/fs` with different functions."
    - Do Not Use or Recommend › Features from 7.1 and later (not in 7.0) — paragraph 1: "…— never suggest them for a project pinned to 7.0:"
[^5]: typescript-eslint issue #12518 "TypeScript 7.0.2 Support" (closed as duplicate of #10940). https://github.com/typescript-eslint/typescript-eslint/issues/12518
    - TypeScript 7.0: The Complete Production Developer Guide — bullet 2: "…side by side; the new API is planned for 7.1."
    - What's New in TypeScript 7.0 › The compiler API is gone (New in 7.0) — table row 1: "…when TS 7 is detected and the parser option `onUnsupportedTypeScriptVersion`."
    - Tooling: Linting, Testing, Running › Linting — bullet 1: "…agrees with 7.0 for code that is clean under `stableTypeOrdering`."
[^6]: microsoft/typescript-go CHANGES.md — intentional differences between TypeScript 6.0 and 7.0. https://github.com/microsoft/typescript-go/blob/main/CHANGES.md
    - TypeScript 7.0: The Complete Production Developer Guide — bullet 3: "…functions, `@enum`, `@class`, Closure-style JSDoc types and values-used-as-types are gone."
    - TypeScript 7.0: The Complete Production Developer Guide — bullet 4: "…declaration conflicts are reported at every site even under `skipLibCheck`."
    - What's New in TypeScript 7.0 › The native compiler and its command line (New in 7.0) — bullet 8: "…offsets** internally (relevant only to tools consuming the unstable API)."
    - What's New in TypeScript 7.0 › Type-checking differences versus 6.0 (New in 7.0) — bullet 1: "…(a `Len<"😀">` utility gives `1` in 7.0, `2` in 6.0)."
    - What's New in TypeScript 7.0 › Type-checking differences versus 6.0 (New in 7.0) — bullet 2: "…site was in a `.d.ts`) now appear in `.ts` files."
    - What's New in TypeScript 7.0 › Type-checking differences versus 6.0 (New in 7.0) — bullet 8: "AST positions are UTF-8 byte offsets."
    - What's New in TypeScript 7.0 › JavaScript and JSDoc checking (New in 7.0) — paragraph 1: "…7.0.2 vs 6.0.3 (details and fixes in the JavaScript section):"
    - JavaScript and JSDoc Projects (`allowJs` / `checkJs`) › Rest parameters, `arguments`, optional parameters — paragraph 1: "…callers omit arguments for parameters typed `undefined`, `unknown` or `any`."
[^7]: Node.js documentation — Modules: TypeScript (type stripping, unsupported syntax, recommended tsconfig). https://nodejs.org/api/typescript.html
    - Modules and Module Resolution › `erasableSyntaxOnly`: TypeScript that strip-only runtimes can run — paragraph 1: "…replace types with whitespace and do not transform anything else."
    - Modules and Module Resolution › `erasableSyntaxOnly`: TypeScript that strip-only runtimes can run — paragraph 2: "…no `paths`, no down-levelling, and relative imports must spell `.ts`."
    - Tooling: Linting, Testing, Running › Running TypeScript — table row 2: "…erasable syntax and `.ts` extensions in relative imports; ignores `tsconfig.json`."
[^8]: Announcing TypeScript 6.0 Beta — `--stableTypeOrdering` cost and purpose, deprecation plan. https://devblogs.microsoft.com/typescript/announcing-typescript-6-0-beta/
    - Migration Notes: 5.x / 6.0 → 7.0 — bullet 2: "…up to 25 %; it is a migration aid only.)"
[^9]: Oxc blog — Type-aware linting is stable (tsgolint v7 built on TypeScript 7.0.2). https://oxc.rs/blog/2026-07-22-type-aware-linting-stable
    - What's New in TypeScript 7.0 › The compiler API is gone (New in 7.0) — table row 5: "…typescript-go (7.0.2) — works with 7.0 without the 6.0 alias."
    - Tooling: Linting, Testing, Running › Linting — bullet 3: "…or `"options": { "typeAware": true }` in the oxlint config."
[^10]: typescript-eslint CHANGELOG — 8.65.0 (2026-07-20): "add warning when TS 7 is detected", `onUnsupportedTypeScriptVersion`. https://github.com/typescript-eslint/typescript-eslint/blob/main/CHANGELOG.md
    - What's New in TypeScript 7.0 › The compiler API is gone (New in 7.0) — table row 1: "…when TS 7 is detected and the parser option `onUnsupportedTypeScriptVersion`."
