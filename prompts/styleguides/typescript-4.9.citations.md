### Citations: TypeScript 4.x (4.0–4.9): The Complete Production Developer Guide

Sources for `typescript-4.9.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Announcing TypeScript 4.9 / TypeScript 4.9 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-9.html
    - Overview — quote block: "…which teaches 4.9 to accept `"ignoreDeprecations": "5.0"` for smoother upgrades."
    - What's New, Minor by Minor › 4.9 (November 2022) — section heading
[^2]: npm registry, `typescript` package version history and publish dates (4.0.2 … 4.9.5, 5.0.2, 6.0.3, 7.0.2, 7.1.0-beta). https://www.npmjs.com/package/typescript?activeTab=versions
    - Overview — quote block: "…which teaches 4.9 to accept `"ignoreDeprecations": "5.0"` for smoother upgrades."
    - Overview — quote block: "…native Go compiler, July 8, 2026), with 7.1 in beta."
[^3]: GitHub release TypeScript 4.9.5 ("Port ignore deprecations to 4.9", #52419; "Port timestamp fix to 4.9", #52426). https://github.com/microsoft/TypeScript/releases/tag/v4.9.5
    - Overview — quote block: "…which teaches 4.9 to accept `"ignoreDeprecations": "5.0"` for smoother upgrades."
    - What's New, Minor by Minor › 4.9 (November 2022) — bullet 7: "4.9.5 accepts `"ignoreDeprecations": "5.0"` (4.8.4 rejects it with TS5023)."
[^4]: Announcing TypeScript 7.0 and TypeScript 6.0 (TypeScript blog). https://devblogs.microsoft.com/typescript/
    - Overview — quote block: "…native Go compiler, July 8, 2026), with 7.1 in beta."
[^5]: DefinitelyTyped README — support window ("only tests packages on versions of TypeScript that are less than 2 years old") and `ts4.x` dist-tags. https://github.com/DefinitelyTyped/DefinitelyTyped#support-window ; https://www.npmjs.com/package/@types/node?activeTab=versions
    - Overview — quote block: "…syntax and **fail to parse on 4.x** (see [`@types` pinning](#types-packages-must-be-pinned))."
    - Compiler Configuration (tsconfig.json) › `@types` packages must be pinned — paragraph 2: "…likewise (`ts4.2` → 18.15.3, `ts4.7` → 20.14.8, `ts4.8` → 22.9.0)."
[^6]: TypeScript 4.0 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-0.html
    - What's New, Minor by Minor › 4.0 (August 2020) — section heading
[^7]: TypeScript 4.1 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-1.html
    - What's New, Minor by Minor › 4.1 (November 2020) — section heading
[^8]: TypeScript 4.2 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-2.html
    - What's New, Minor by Minor › 4.2 (February 2021) — section heading
[^9]: TypeScript 4.3 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-3.html
    - What's New, Minor by Minor › 4.3 (May 2021) — section heading
[^10]: TypeScript 4.4 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-4.html
    - What's New, Minor by Minor › 4.4 (August 2021) — section heading
[^11]: TypeScript 4.5 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-5.html
    - What's New, Minor by Minor › 4.5 (November 2021) — section heading
[^12]: TypeScript 4.6 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-6.html
    - What's New, Minor by Minor › 4.6 (February 2022) — section heading
[^13]: TypeScript 4.7 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-7.html
    - What's New, Minor by Minor › 4.7 (May 2022) — section heading
[^14]: TypeScript 4.8 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-8.html
    - What's New, Minor by Minor › 4.8 (August 2022) — section heading
[^15]: Zod 3.22 README ("TypeScript 4.5+!") and Zod 4 documentation (tested against TypeScript 5.5+). https://unpkg.com/zod@3.22.4/README.md ; https://zod.dev
    - Security › Validate untrusted input — bullet 2: "Zod 4 is tested only against TypeScript 5.5+."
[^16]: typescript-eslint — dependency versions and `SUPPORTED_TYPESCRIPT_VERSIONS` in `@typescript-eslint/typescript-estree` (5.62.0, 6.21.0, 7.18.0, 8.15.0, 8.16.0, 8.72.0). https://typescript-eslint.io/users/dependency-versions
    - Tooling › Linting: `typescript-eslint` versions for 4.x — paragraph 1: "…running compiler against its supported range and warns outside it."
[^17]: TypeScript 5.0 release notes — Breaking Changes and Deprecations. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
    - Migrating from 4.x to 5.0 (and beyond) — paragraph 1: "…is the next release after 4.9; there is no 4.10."
    - Migrating from 4.x to 5.0 (and beyond) › 5.0 deprecations (errors TS5101/TS5107 in 5.0–5.4, no effect in 5.5, hard errors from 5.6) — paragraph 1: "…the default values such as `importsNotUsedAsValues: "remove"` are not reported):"
[^18]: TSConfig reference (`useDefineForClassFields`, `moduleResolution`, `target`, `strict`, `ignoreDeprecations`). https://www.typescriptlang.org/tsconfig
    - Compiler Configuration (tsconfig.json) › The `strict` family by version — paragraph 1: "…was verified by passing each flag to every 4.x compiler."
[^19]: Modules reference — theory and `node16`/`nodenext`. https://www.typescriptlang.org/docs/handbook/modules/reference.html
    - Modules and Module Resolution › Resolution modes in 4.x — paragraph 1: "See the TypeScript modules reference for the full algorithm."
[^20]: ts-jest, ts-node and tslib package metadata (peer dependencies, helper exports). https://www.npmjs.com/package/ts-jest ; https://www.npmjs.com/package/ts-node ; https://www.npmjs.com/package/tslib
    - Tooling › Compiler and runners — bullet 4: "**`ts-node` 10.9.2** (peer `typescript >= 2.7`)"
