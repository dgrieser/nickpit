### Citations: TypeScript Style Guide

Sources for `typescript.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: TypeScript release notes (per version) and the TypeScript blog. https://www.typescriptlang.org/docs/handbook/release-notes/overview.html , https://devblogs.microsoft.com/typescript/
    - TypeScript Style Guide — quote block: "**7.1** is in beta (7.1.0-beta, 2026-10-20)."
    - TypeScript Style Guide — quote block: "…every older minor line is frozen at its last patch."
[^2]: npm registry — `typescript` versions and publish dates (`npm view typescript time`). https://www.npmjs.com/package/typescript?activeTab=versions
    - TypeScript Style Guide — quote block: "**7.1** is in beta (7.1.0-beta, 2026-10-20)."
    - Version-Aware Review › Release table (status as of October 2026) — paragraph 1: "Dates are npm publish dates."
[^3]: DefinitelyTyped README — "Support window" (TypeScript versions younger than two years) and `tsX.Y` dist-tags. https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
    - Version-Aware Review › Release table (status as of October 2026) — paragraph 1: "…the matching `tsX.Y` dist-tag (for example `npm i -D @types/node@ts4.9`)."
[^4]: TypeScript 6.0 release notes — new defaults, deprecations, `ignoreDeprecations`, `--stableTypeOrdering`. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html
    - `tsconfig.json` Hygiene and Strictness Flags › Set the options whose defaults changed — paragraph 1: "…defaults below changed in 6.0 and stay changed in 7.0:"
    - Upgrading Across Versions — Migration Notes › 5.x → 6.0 — bullet 4: "…next to a `tsconfig.json` is TS5112; use `tsc -p .`."
[^5]: Announcing TypeScript 7.0 — native compiler, removed options, no API, `@typescript/typescript6`. https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/
    - `tsconfig.json` Hygiene and Strictness Flags › Set the options whose defaults changed — paragraph 1: "…defaults below changed in 6.0 and stay changed in 7.0:"
    - Linting and Tooling › typescript-eslint — bullet 1: "…install 6.0 next to it (`"typescript": "npm:@typescript/typescript6@^6.0.2"` plus `"@typescript/native": "npm:typescript@^7.0.2"`)."
    - Performance and Observability › Compiler performance — bullet 5: "…do not need the compiler API in the same process."
    - Upgrading Across Versions — Migration Notes › 6.0 → 7.0 — bullet 2: "…Angular's compiler, Vue/Svelte/Astro/MDX tooling keep using 6.0 via `@typescript/typescript6` (`tsc6`)."
[^6]: TSConfig reference — `esModuleInterop`, `module`, `moduleResolution`, `verbatimModuleSyntax`, `useDefineForClassFields`. https://www.typescriptlang.org/tsconfig/
    - Modules, Module Resolution and ESM/CommonJS Interop › CommonJS interop: `esModuleInterop` and default imports — bullet 2: "…6.0 as a bug (verified with 6.0.3 and Node.js 24)."
[^7]: typescript-eslint — Dependency Versions (supported TypeScript range). https://typescript-eslint.io/users/dependency-versions/
    - Linting and Tooling › typescript-eslint — bullet 1: "…install 6.0 next to it (`"typescript": "npm:@typescript/typescript6@^6.0.2"` plus `"@typescript/native": "npm:typescript@^7.0.2"`)."
[^8]: Node.js documentation — Modules: TypeScript (type stripping and its limitations). https://nodejs.org/api/typescript.html
    - Linting and Tooling › Transpilers and type stripping — bullet 2: "…`erasableSyntaxOnly` (5.8+), `verbatimModuleSyntax`, and `allowImportingTsExtensions` with `rewriteRelativeImportExtensions` (5.7+) or `noEmit`."
