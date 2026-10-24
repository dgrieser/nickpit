### Citations: HTML & CSS Style Guide

Sources for `html-css.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Baseline (web-features) — definitions of Widely/Newly available and the 30-month rule; web-features data package (3.44.0). https://web-platform-dx.github.io/web-features/ , https://github.com/web-platform-dx/web-features , https://www.npmjs.com/package/web-features
    - Overview — quote block: "…W3C WebDX Community Group) and shown on MDN and caniuse."
    - Baseline Primer and Baseline-Aware Review › What the statuses mean — paragraph 1: "Newly 2023-12-19, Widely 2026-06-19)."
[^2]: MDN Glossary: Baseline (compatibility). https://developer.mozilla.org/en-US/docs/Glossary/Baseline/Compatibility
    - Overview — quote block: "…W3C WebDX Community Group) and shown on MDN and caniuse."
[^3]: web.dev — Baseline. https://web.dev/baseline
    - Baseline Primer and Baseline-Aware Review › What the statuses mean — paragraph 1: "Newly 2023-12-19, Widely 2026-06-19)."
[^4]: Browserslist — queries including `baseline widely available` / `baseline newly available` / `baseline YYYY`. https://github.com/browserslist/browserslist#queries
    - Baseline Primer and Baseline-Aware Review › What the statuses mean — paragraph 3: "…17.4** (plus the current Android Chrome/Firefox) *(executed with `npx browserslist`)*."
[^5]: Web Content Accessibility Guidelines (WCAG) 2.2, W3C Recommendation (12 December 2024 edition; 4.1.1 removed). https://www.w3.org/TR/WCAG22/
    - Document Structure and Metadata › Minimal correct document — bullet 8: "…duplicate IDs that break name/role/value still fail 1.3.1 and 4.1.2."
    - Accessibility (WCAG 2.2 AA) › Status of the standards — bullet 1: "…also **ISO/IEC 40500:2025** (announced by W3C on 21 October 2025)."
[^6]: W3C WAI — WCAG 2.2 Approved as an ISO Standard (ISO/IEC 40500:2025), 21 October 2025. https://www.w3.org/WAI/news/2025-10-21/wcag22-iso
    - Accessibility (WCAG 2.2 AA) › Status of the standards — bullet 1: "…also **ISO/IEC 40500:2025** (announced by W3C on 21 October 2025)."
[^7]: National Disability Authority (Ireland) — EN 301 549 V4.1.1 published (2 September 2026; WCAG 2.2 AA; expected OJEU citation). https://nda.ie/news/en301549-published ; Directive (EU) 2019/882 (European Accessibility Act). https://eur-lex.europa.eu/eli/dir/2019/882/oj
    - Accessibility (WCAG 2.2 AA) › Status of the standards — bullet 3: "…(expected December 2026), V3.2.1 (WCAG 2.1) remains the cited version."
[^8]: Using ARIA — the rules of ARIA use. https://www.w3.org/TR/using-aria/
    - Accessibility (WCAG 2.2 AA) › The rules of ARIA — paragraph 1: "From *Using ARIA* and *ARIA in HTML*:"
[^9]: ARIA in HTML (implicit roles, allowed and prohibited attributes). https://www.w3.org/TR/html-aria/ ; WAI-ARIA 1.2 (naming prohibited on generic). https://www.w3.org/TR/wai-aria-1.2/
    - Accessibility (WCAG 2.2 AA) › The rules of ARIA — paragraph 1: "From *Using ARIA* and *ARIA in HTML*:"
[^10]: HTML Standard — Autofill (autocomplete tokens). https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#autofill
    - Forms › Input types, `autocomplete`, `inputmode` — bullet 1: "…`one-time-code`, `cc-name`, `cc-number`, `cc-exp`, `cc-csc`, …), optionally with section/`shipping`/`billing` prefixes."
[^11]: HTML Standard — The dialog element; popover attribute; invoker commands (`command`/`commandfor`). https://html.spec.whatwg.org/multipage/interactive-elements.html#the-dialog-element , https://html.spec.whatwg.org/multipage/popover.html
    - Dialogs, Popovers, Invokers and `inert` › `<dialog>` (Widely available) — paragraph 1: "HTML Standard, MDN."
[^12]: MDN — `<dialog>`, Popover API, `inert`. https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/dialog , https://developer.mozilla.org/en-US/docs/Web/API/Popover_API , https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Global_attributes/inert
    - Dialogs, Popovers, Invokers and `inert` › `<dialog>` (Widely available) — paragraph 1: "HTML Standard, MDN."
[^13]: HTML Standard — `iframe` `sandbox` (warning about `allow-scripts` + `allow-same-origin`). https://html.spec.whatwg.org/multipage/iframe-embed-object.html#attr-iframe-sandbox
    - Security in Markup › Iframes — bullet 1: "**`sandbox="allow-scripts allow-same-origin"` on same-origin content provides no sandbox**"
[^14]: Content Security Policy Level 3 (nonce hiding, directives ignored in `<meta>`). https://www.w3.org/TR/CSP3/
    - Security in Markup › Content Security Policy-friendly markup — paragraph 1: "Behaviour under `Content-Security-Policy: script-src 'nonce-…'; style-src 'self'` *(Chromium-verified)*:"
[^15]: Subresource Integrity. https://www.w3.org/TR/SRI/
    - Security in Markup › Subresource integrity and third-party code — bullet 1: "…"Failed to find a valid digest".)* Cross-origin SRI requires CORS"
[^16]: Referrer Policy (default `strict-origin-when-cross-origin`). https://www.w3.org/TR/referrer-policy/ ; MDN `Referrer-Policy`. https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Referrer-Policy
    - Security in Markup › Referrer policy — bullet 1: "The default policy is **`strict-origin-when-cross-origin`**"
[^17]: web.dev — Optimize Largest Contentful Paint; Lazy-loading images and LCP; Optimize CLS; Fetch Priority. https://web.dev/articles/optimize-lcp , https://web.dev/articles/lcp-lazy-loading , https://web.dev/articles/optimize-cls , https://web.dev/articles/fetch-priority
    - Loading Performance and Layout Stability › Images — bullet 2: "**Never `loading="lazy"` the LCP/hero image**"
[^18]: CSS Cascading and Inheritance Level 5 (cascade layers) and Level 6 (`@scope`). https://www.w3.org/TR/css-cascade-5/ , https://www.w3.org/TR/css-cascade-6/
    - The Cascade and Specificity › Order of precedence (simplified) — paragraph 1: "Per CSS Cascading Level 5/6."
[^19]: Selectors Level 4 (`:is()`, `:where()`, `:has()`, forgiving selector lists, specificity). https://www.w3.org/TR/selectors-4/
    - The Cascade and Specificity › Specificity rules that cause real bugs — bullet 2: "**Selector lists are invalidated by a single bad selector**"
[^20]: CSS Nesting Module (relaxed syntax, `CSSNestedDeclarations`). https://www.w3.org/TR/css-nesting-1/
    - Modern CSS Features (with Baseline status) › CSS nesting — Widely available (2026-06-11) — bullet 1: "**Native nesting is not Sass**"
[^21]: CSS Containment Module Level 3 (container queries). https://www.w3.org/TR/css-contain-3/
    - Modern CSS Features (with Baseline status) › Container queries — bullet 5: "**An element cannot query itself.**"
[^22]: CSS Custom Properties Level 1 (invalid at computed-value time) and CSS Properties and Values API (`@property`). https://www.w3.org/TR/css-variables-1/ , https://www.w3.org/TR/css-properties-values-api-1/
    - The Cascade and Specificity › Custom properties pitfalls — paragraph 1: "*(Chromium-verified.)* The fallback in `var(--x, fallback)`"
[^23]: CSS Color Level 4 and 5 (`oklch()`, `color-mix()`, relative colour, `light-dark()`, `contrast-color()`). https://www.w3.org/TR/css-color-4/ , https://www.w3.org/TR/css-color-5/
    - Modern CSS Features (with Baseline status) › Colour: `oklch()`, `color-mix()`, relative colours, `contrast-color()` — paragraph 1: "Facts *(Chromium-verified)*:"
[^24]: CSS View Transitions Level 1 and 2. https://www.w3.org/TR/css-view-transitions-1/ , https://www.w3.org/TR/css-view-transitions-2/
    - Modern CSS Features (with Baseline status) › View transitions — bullet 2: "**Cross-document (MPA) view transitions — not Baseline (no Firefox).**"
[^25]: CSS Anchor Positioning. https://www.w3.org/TR/css-anchor-position-1/
    - Modern CSS Features (with Baseline status) › Anchor positioning — core Newly available (2026-01-13), whole feature Limited — paragraph 1: "…not.)* Anchors must be laid out before the positioned element"
[^26]: CSS Transitions Level 2 (`@starting-style`, `transition-behavior`). https://www.w3.org/TR/css-transitions-2/
    - Modern CSS Features (with Baseline status) › `@starting-style` and discrete transitions — Newly available (2024-08-06) — paragraph 1: "…below 1 and reached 1 after the transition.)* Without `@starting-style`,"
[^27]: CSS Basic User Interface Level 4 (`field-sizing`, `accent-color`). https://www.w3.org/TR/css-ui-4/
    - Forms › `field-sizing` (Newly available 2026-06-16) — paragraph 1: "…and keep the fixed size, which is an acceptable fallback."
[^28]: CSS Text Level 4 (`text-wrap`); MDN `text-wrap-style` (balance line limits). https://www.w3.org/TR/css-text-4/ , https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/text-wrap-style
    - Modern CSS Features (with Baseline status) › `text-wrap` — paragraph 1: "`pretty` costs layout time."
[^29]: Media Queries Level 5 (`prefers-reduced-motion`, `prefers-color-scheme`, `forced-colors`, `prefers-contrast`). https://www.w3.org/TR/mediaqueries-5/
    - Motion and `prefers-reduced-motion` — paragraph 1: "`prefers-reduced-motion` is Widely available (2022)."
[^30]: CSS Masking (deprecation of `clip`). https://www.w3.org/TR/css-masking-1/#clip-property
    - Accessibility (WCAG 2.2 AA) › Visually hidden text — paragraph 1: "…versions of this pattern still works but should be migrated."
[^31]: HTML Standard — `pattern` attribute compiled with the `v` flag. https://html.spec.whatwg.org/multipage/input.html#the-pattern-attribute
    - Forms › Input types, `autocomplete`, `inputmode` — bullet 6: "**`pattern` is compiled with the RegExp `v` flag.**"
[^32]: HTML Standard — `target="_blank"` implies `noopener`; `window.open()` features. https://html.spec.whatwg.org/multipage/links.html#link-type-noopener , https://html.spec.whatwg.org/multipage/nav-history-apis.html#dom-open
    - Security in Markup › Links and windows — bullet 1: "**`target="_blank"` implies `rel="noopener"`**"
[^33]: ESLint CSS plugin (`use-baseline` rule). https://github.com/eslint/css ; stylelint-plugin-use-baseline. https://github.com/ryo-manba/stylelint-plugin-use-baseline
    - Tooling — bullet 1: "**Baseline linting**"
