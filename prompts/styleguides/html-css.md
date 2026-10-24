### HTML & CSS Style Guide

#### Overview

> **Edition**: The complete production review guide for HTML and CSS, Baseline status as of October 2026.
> **Versioning model**: HTML and CSS have no release numbers. This guide judges feature availability by **Baseline** status as published in the `web-features` data set (web-platform-dx, W3C WebDX Community Group) and shown on MDN and caniuse. All statuses below are taken from **`web-features` 3.44.0 (published 2026-10-22)**, not from memory.
> **Browsers at time of writing** (per that data set): Chrome 156 (2026-10-20), Edge 155 (2026-10-08), Firefox 158 (2026-10-13), Safari / Safari on iOS 27 (2026-09-14). The release data shows Chrome and Firefox shipping every two weeks since September 2026 (Chrome 152 → 153, Firefox 154 → 155), so version numbers now climb faster than in earlier years.
> **Scope**: semantic HTML, accessibility (WCAG 2.2 AA, ARIA), forms, dialogs/popovers, security-relevant markup, loading performance, the cascade and specificity, modern CSS, motion, colour schemes, layout and typography — in plain `.html`/`.css` files and in HTML/CSS embedded in templates and components (JSX, Vue, Svelte, Angular, Web Components, server templates).
> **Verification**: behavioural claims marked *(Chromium-verified)* were executed in headless **Chromium 141.0.7390.37** (Playwright 1.56.1) with `CSS.supports()`, `getComputedStyle()`, layout measurements, network interception and the Chromium accessibility tree. Chromium 141 is older than current Chrome 156, so it cannot exercise features that shipped later (e.g. interest invokers, `contrast-color()`, name-only container queries). Firefox and Safari behaviour is sourced from specifications, MDN and browser-compat-data, not executed.
> **Note**: This document is self-contained. It replaces the previous generic HTML/CSS guide and keeps its house rules (semantic structure, BEM naming, custom-property tokens, mobile-first layout) where they are still correct.

Global truths a reviewer should hold:

- **The platform is evergreen, but users are not all on the newest browser.** "Supported in Chrome" is not "safe to ship". The question is always: *what is the project's browser target, and what is the feature's Baseline status against it?*
- **HTML is fault tolerant; that hides bugs.** Invalid nesting, unknown attributes, duplicate IDs and typos in attribute values do not throw. They silently produce a different DOM, a different accessibility tree, or no effect at all.
- **CSS is fault tolerant too.** An unknown property, value or (in most contexts) selector drops that declaration or rule without an error. This enables progressive enhancement, and it also means a typo or an unsupported feature fails silently.
- **Native elements carry behaviour and semantics for free** (keyboard handling, focus, roles, states, form participation). Every ARIA attribute or `div`-based widget is a promise to rebuild that behaviour by hand.
- **Accessibility is a correctness property, not polish.** WCAG 2.2 AA is the legal baseline in many jurisdictions (see Accessibility).

***

#### Baseline Primer and Baseline-Aware Review

##### What the statuses mean

| Status | Definition (web-features) | Badge wording |
|---|---|---|
| **Widely available** (`baseline: "high"`) | Supported in all core browsers for at least **30 months** after it became Newly available | "Baseline Widely available" |
| **Newly available** (`baseline: "low"`) | Supported in the latest stable version of **every** core browser: Chrome, Chrome Android, Edge, Firefox, Firefox Android, Safari, Safari on iOS | "Baseline <year>" / "Newly available" |
| **Limited availability** (`baseline: false`) | Missing (or only partially/behind a flag) in at least one core browser | "Limited availability" |

A feature's "Baseline year" is the year it became Newly available (e.g. `:has()` is "Baseline 2023"). Widely-available date = Newly-available date + 30 months (e.g. `:has()`: Newly 2023-12-19, Widely 2026-06-19).

Key consequences:

- **Newly available ≠ safe for everyone.** It only says the *latest* versions support it. Users on Safari 17 or a two-year-old Android Chrome do not have it.
- **A feature's status is the weakest of its parts.** `web-features` computes a feature's status from every browser-compat-data key it contains. Anchor positioning is the prime example: most of its keys (`anchor-name`, `anchor()`, `@position-try`, `position-area`, …) are Newly available since 2026-01-13, but the feature as a whole is *Limited* because `position-visibility: anchor-valid | anchor-visible` only exist in Safari 27 (Chrome 157 is in beta). Individual keys can even lag the "core": `position-anchor` is recorded as fully supported only from Chrome/Firefox 151 and Safari 27 (2026-09-14), because earlier versions used a different initial value (`auto`/`none`/`implicit` instead of `normal`) — an explicit `position-anchor: --name` works since the core versions.
- **Non-core browsers** (Samsung Internet, Opera, in-app WebViews) are not part of the definition. Check them separately if analytics say they matter.
- **Baseline says nothing about bugs, accessibility support in screen readers, or performance.** `display: contents` on buttons, for example, has a long history of accessibility-tree bugs that compat data does not model as "unsupported".

As of 2026-10-22, `browserslist` 4.29.4 resolves `baseline widely available` to **Chrome/Edge ≥ 124, Firefox ≥ 125, Safari / iOS Safari ≥ 17.4** (plus the current Android Chrome/Firefox) *(executed with `npx browserslist`)*.

##### Features turning Widely available within the next months

These are Newly available today; do not describe them as Widely available before the date.

| Feature | Newly available | Widely available on |
|---|---|---|
| `light-dark()` | 2024-05-13 | 2026-11-13 |
| `text-wrap: balance` | 2024-05-13 | 2026-11-13 |
| `zoom` (standardised) | 2024-05-14 | 2026-11-14 |
| `round()`, `mod()`, `rem()` | 2024-05-17 | 2026-11-17 |
| Registered custom properties (`@property`) | 2024-07-09 | 2027-01-09 |
| `font-size-adjust` | 2024-07-25 | 2027-01-25 |
| `@starting-style`, `transition-behavior` | 2024-08-06 | 2027-02-06 |
| Exclusive `<details name>` | 2024-09-03 | 2027-03-03 |
| Relative colour syntax, `backdrop-filter` | 2024-09-16 | 2027-03-16 |
| `text-wrap` (longhands `text-wrap-mode`/`-style`) | 2024-10-17 | 2027-04-17 |
| `fetchpriority` | 2024-10-29 | 2027-04-29 |
| `scrollbar-gutter`, `scrollbar-width` | 2024-12-11 | 2027-06-11 |
| Popover API | 2025-01-27 | 2027-07-27 |

##### What changed in the last twelve months (2025-10-23 → 2026-10-22)

Newly available (HTML/CSS-relevant), per `web-features` 3.44.0:

- **2025-10-14** same-document view transitions, `view-transition-class` (Firefox 144) · **2025-12-12** invoker commands (`command`/`commandfor`), `scrollbar-color`, `scrollend` event, LCP and Event Timing entries (Safari 26.2)
- **2026-01-13** anchor-positioning *core properties* (feature as a whole still Limited), `:active-view-transition`, `startViewTransition({ types })`, Navigation API, `rch`/`rex`/`ric`/`rcap` units (Firefox 147)
- **2026-02-24** Trusted Types, `shape()` (Firefox 148) · **2026-03-13** `text-indent: hanging | each-line` · **2026-03-24** `@scope`, Custom Highlight API, CSP violation reports via the Reporting API (Safari 26.4)
- **2026-04-10** `contrast-color()` · **2026-05-07** name-only container queries, `image-rendering: crisp-edges` · **2026-05-11** `:open` · **2026-05-19** container style queries (Firefox 151)
- **2026-06-16** `field-sizing` (Firefox 152) · **2026-08-18** `sibling-index()`/`sibling-count()` and the `text-box`/`text-box-trim`/`text-box-edge` properties (Firefox 154) · **2026-09-01** `progress()`
- **2026-09-14** (Safari 27): `ariaNotify()`, `light-dark()` for images, `alpha()`, `revert-rule`, `overflow-anchor`; **2026-09-11** `autocorrect`

Became Widely available: `oklch()`/`oklab()`/`lab()`/`color()`, `color-mix()`, `:nth-child(of S)` (2025-11-09); `calc()` constants `pi`/`e`/`infinity` (2025-12-06); `animation-composition` (2026-01-04); two-value `display` (2026-01-21); **subgrid** (2026-03-15); `contain-intrinsic-size`, `image-set()`, `modulepreload`, `hyphens`, `@counter-style` (2026-03-18); `<search>` (2026-04-13); ARIA attribute reflection (2026-04-24); `:user-valid`/`:user-invalid`, `clip-path` box values (2026-05-02); `lh`/`rlh` (2026-05-21); `:dir()`, CSS masks, `exp()`/`pow()`… (2026-06-07); **CSS nesting**, `linear()` easing, `cap` unit, preloading responsive images (2026-06-11); **`:has()`** and **`loading="lazy"`** (2026-06-19); AVIF (2026-07-25); declarative shadow DOM (2026-08-20); `white-space-collapse`, `text-wrap: wrap | nowrap` (2026-09-19); `paint-order` (2026-09-22); `transform-box`, `align-content` in block layout (2026-10-16).

##### Version-aware (Baseline-aware) review rules

1. **Find the target first.** Look for `browserslist` in `package.json` / `.browserslistrc`, a `baseline` option in a build tool, a `@supports`/polyfill strategy, or a documented support policy. Judge against that target, not against "latest Chrome".
2. **Without a stated target, assume Baseline Widely available** and treat Newly-available features as *needing a fallback or progressive enhancement*.
3. **Check the status before claiming something is unavailable.** Do not say "not supported in Firefox" from memory — `:has()` (Firefox 121), `@scope` (Firefox 146), `field-sizing` (Firefox 152) and same-document view transitions (Firefox 144) all landed during 2023–2026. Say "not Baseline" only when the data proves it.
4. **Progressive enhancement is fine for non-Baseline features** as long as the fallback is acceptable: `text-wrap: pretty`, `interpolate-size`, `::scroll-marker` or speculation rules degrade to nothing harmful. A feature that carries *function* (layout, visibility, interaction, security) needs a real fallback or a `@supports` gate.
5. **Do not demand vendor prefixes for Baseline features** (`-webkit-backdrop-filter` is unnecessary since Safari 18; `backdrop-filter` is Baseline 2024). Do flag prefixed-only usage of features with an unprefixed standard (`-webkit-line-clamp` is the exception: the unprefixed `line-clamp` is in no browser).

***

#### Baseline Status Reference (web-features 3.44.0, 2026-10-22)

Browser versions are the first versions where *the whole feature* is supported according to `web-features` (C = Chrome, E = Edge, F = Firefox, S = Safari; Android/iOS versions match unless noted). "W" = Widely available, "N" = Newly available, "L" = Limited availability.

##### CSS cascade, selectors, at-rules

| Feature | Status | Since | Browsers |
|---|---|---|---|
| Cascade layers `@layer` | **W** | N 2022-03-14, W 2024-09-14 | C99 F97 S15.4 |
| `:is()`, `:where()` | **W** | N 2021-01-21 | C88 F82 S14 |
| `:not()` (selector lists) | **W** | N 2021-01-21 | C88 F84 S9 |
| `:has()` | **W** | N 2023-12-19, W 2026-06-19 | C105 F121 S15.4 |
| `:nth-child(… of S)` | **W** | W 2025-11-09 | C111 F113 S9 |
| `:focus-visible` | **W** | W 2024-09-14 | C86 F85 S15.4 |
| `:user-valid` / `:user-invalid` | **W** | N 2023-11-02, W 2026-05-02 | C119 F88 S16.5 |
| `:modal` | **W** | W 2025-03-02 | C105 F103 S15.6 |
| `:open` | **N** | 2026-05-11 | C133 F136 S26.5 |
| `::details-content` | **N** | 2025-09-16 | C131 F143 S18.4 |
| CSS nesting | **W** | N 2023-12-11, W 2026-06-11 | C120 F117 S17.2 |
| ↳ `CSSNestedDeclarations` (declarations after nested rules keep their order) | **N** | 2024-12-11 | C130 F132 S18.2 |
| `@scope` | **N** | 2026-03-24 | C143 F146 S26.4 (core `@scope` rule: C118 F146 S26.4) |
| Container size queries + `cq*` units | **W** | N 2023-02-14, W 2025-08-14 | C105 F110 S16 |
| Container style queries (custom properties) | **N** | 2026-05-19 | C111 F151 S18 |
| Name-only container queries | **N** | 2026-05-07 | C148 F149 S26.4 |
| Container scroll-state queries | **L** | — | C133 only |
| Registered custom properties `@property` | **N** | 2024-07-09 | C85 F128 S16.4 |
| Media query range syntax `(width >= 600px)` | **W** | W 2025-09-27 | C104 F102 S16.4 |
| `@starting-style` | **N** | 2024-08-06 | C117 F129 S17.5 |
| `@function`, `if()` | **L** | — | Chromium only (C139 / C137) |
| Custom media queries `@custom-media` | **L** | — | none |

##### CSS layout, sizing, text

| Feature | Status | Since | Browsers |
|---|---|---|---|
| Subgrid | **W** | N 2023-09-15, W 2026-03-15 | C117 F71 S16 |
| Logical properties | **W** | W 2024-03-20 | C89 F66 S15 |
| `aspect-ratio` | **W** | W 2024-03-20 | C88 F89 S15 |
| `min()`, `max()`, `clamp()` | **W** | W 2023-01-28 | C79 F75 S13.1 |
| `round()`, `mod()`, `rem()` | **N** | 2024-05-17 | C125 F118 S17.2 |
| `svh`/`lvh`/`dvh` viewport units | **W** | W 2025-06-05 | C108 F101 S15.4 |
| `field-sizing` | **N** | 2026-06-16 | C123 F152 S26.2 |
| `text-wrap` shorthand + longhands | **N** | 2024-10-17 | C130 F124 S17.5 |
| `text-wrap: balance` | **N** | 2024-05-13 | C114 F121 S17.5 |
| `text-wrap: pretty` | **L** | — | C117 S26 (no Firefox) |
| `content-visibility` | **N** | 2025-09-15 | C108 F130 S26 |
| `contain-intrinsic-size` | **W** | W 2026-03-18 | C83 F107 S17 |
| `sibling-index()`, `sibling-count()` | **N** | 2026-08-18 | C138 F154 S26.2 |
| `scrollbar-gutter` | **N** | 2024-12-11 | C94 F97 S18.2 |
| `overscroll-behavior` | **L** | — | C63 F59 S16 for real scroll containers; full support (also on containers without scrollable overflow) only C144 F150, Safari partial |
| `line-clamp` (unprefixed) | **L** | — | none |
| `text-box` (whole feature) | **L** | — | C133 S18.2; the `text-box`, `text-box-trim`, `text-box-edge` properties are **N** since 2026-08-18 (C133 F154 S18.2), but the `<text-edge>` value keys (`cap`, `alphabetic`, `ex`, `text`) are not recorded for Firefox |
| Grid lanes (formerly "masonry") | **L** | — | none |
| `reading-flow` | **L** | — | C137 only |
| Anchor positioning (whole feature) | **L** | — | S27 only; core properties are **N** since 2026-01-13 (C125 F147 S26; `position-area` C129) |

##### CSS colour, motion, effects

| Feature | Status | Since | Browsers |
|---|---|---|---|
| `oklab()`/`oklch()`, `lab()`/`lch()`, `color()` | **W** | N 2023-05-09, W 2025-11-09 | C111 F113 S15.4 (`lab`/`color`: S15) |
| `color-mix()` (two colours) | **W** | W 2025-11-09 | C111 F113 S16.2 |
| `color-mix()` with more than two colours | **L** | — | F150 S27 |
| Relative colour syntax `rgb(from …)` | **N** | 2024-09-16 | C125 F128 S18 |
| `light-dark()` (colours) | **N** | 2024-05-13 | C123 F120 S17.5 |
| `light-dark()` for images | **N** | 2026-09-14 | C150 F150 S27 |
| `contrast-color()` | **N** | 2026-04-10 | C147 F146 S26 |
| `color-scheme` | **W** | W 2024-08-03 | C98 F96 S13 |
| `prefers-color-scheme`, `prefers-reduced-motion` | **W** | W 2022-07-15 | C76/C74 F67/F63 S12.1/S10.1 |
| `prefers-contrast` | **W** | W 2024-11-30 | C96 F101 S14.1 |
| `forced-colors` | **W** | W 2025-03-12 | C89 F89 S16 |
| View transitions (same-document) | **N** | 2025-10-14 | C111 F144 S18 |
| `view-transition-class` | **N** | 2025-10-14 | C125 F144 S18.2 |
| `:active-view-transition`, `startViewTransition({types})` | **N** | 2026-01-13 | C125 F147 S18.2 |
| Cross-document view transitions `@view-transition` | **L** | — | C126 S18.2 (no Firefox) |
| Element-scoped view transitions | **L** | — | C147 only |
| `transition-behavior: allow-discrete` | **N** | 2024-08-06 | C117 F129 S17.4 |
| Animating `display` (`display-animation`) | **L** | — | C117 S18 (no Firefox) |
| `overlay` property | **L** | — | C117 only (no Firefox, no Safari) |
| `interpolate-size`, `calc-size()` | **L** | — | C129 only |
| Scroll-driven animations | **L** | — | C115 S26 (no Firefox) |
| `backdrop-filter` | **N** | 2024-09-16 | C76 F103 S18 |
| `accent-color` | **L** | — | C93 F92 S26.2 (Chrome Android only partial: no minimum-contrast adjustment) |
| Customizable `<select>` (`appearance: base-select`) | **L** | — | C135 S27 |
| `zoom` | **N** | 2024-05-14 | C1 F126 S3.1 |

##### HTML and DOM

| Feature | Status | Since | Browsers |
|---|---|---|---|
| `<dialog>` (`showModal()`, `::backdrop`) | **W** | W 2024-09-14 | C37 F98 S15.4 |
| `dialog.requestClose()` | **N** | 2025-05-27 | C134 F139 S18.4 |
| `<dialog closedby>` | **L** | — | C134 F141 (no Safari) |
| Popover API (`popover`, `popovertarget`) | **N** | 2025-01-27 | C116 F125 S17 (iOS 18.3; iOS 17–18.2 lacked tap-outside light dismiss) |
| `popover="hint"` | **L** | — | C151 F153 (C133–150 and F149–152 implemented an older, inconsistent version) |
| Invoker commands (`command`/`commandfor`) | **N** | 2025-12-12 | C135 F144 S26.2 |
| Interest invokers (`interestfor`) | **L** | — | C142 only |
| `inert` | **W** | N 2023-04-11, W 2025-10-11 | C102 F112 S15.5 |
| `<details>`/`<summary>` | **W** | W 2022-07-15 | — |
| `<details name>` (exclusive accordion) | **N** | 2024-09-03 | C120 F130 S17.2 |
| `hidden="until-found"` | **L** | — | C102 F148; Safari 26.2 partial (does not scroll to the match correctly) |
| `<search>` | **W** | N 2023-10-13, W 2026-04-13 | C118 F118 S17 |
| Date/time inputs (`date`, `time`, `datetime-local`) | **W** | W 2024-04-05 | C20 F93 S14.1 |
| `<input type="color">`, `<datalist>` | **L** | — | Firefox Android lacks them |
| `inputmode`, `enterkeyhint` | **W** | W 2024 | — |
| Declarative shadow DOM | **W** | W 2026-08-20 | C111 F123 S16.4 |
| ARIA attribute reflection (string attributes) | **W** | W 2026-04-24 | C103 F119 S16.4 |
| `ariaNotify()` | **N** | 2026-09-14 | C141 F150 S27 |

##### Loading and security

| Feature | Status | Since | Browsers |
|---|---|---|---|
| `loading="lazy"` (img, iframe) | **W** | N 2023-12-19, W 2026-06-19 | C77 F121 S16.4 |
| `fetchpriority` | **N** | 2024-10-29 | C103 F132 S17.2 |
| `<link rel="preload">` | **W** | W 2023-07-26 | C50 F85 S11.1 |
| `<link rel="preconnect">` | **W** | W 2022-07-15 | C46 F39 S11.1 |
| `<link rel="modulepreload">` | **W** | W 2026-03-18 | C66 F115 S17 |
| `srcset`/`sizes`, `<picture>` | **W** | since 2019/2018 | — |
| AVIF | **W** | W 2026-07-25 | C85 F113 S16.4 |
| `sizes="auto"` | **L** | — | C126 F150 (no Safari) |
| `blocking="render"` | **L** | — | C105 S18.2 |
| Speculation rules | **L** | — | Chromium only |
| Implicit `noopener` for `target="_blank"` | **W** | N 2021-01-21 | C88 F79 S12.1 |
| Referrer policy | **W** | W 2024-03-20 | C85 F87 S15 |
| Subresource integrity | **W** | W 2020-10-30 | — |
| Sandboxed iframes | **W** | since 2015 | — |
| CSP (incl. `nonce`) | **W** | — | — |
| Trusted Types | **N** | 2026-02-24 | C83 F148 S26 |
| Credentialless iframes | **L** | — | Chromium only |

***

#### Do Not Rely On Without Fallback

These are **not Baseline** (Limited availability) as of 2026-10-22. Never present them as cross-browser; accept them only behind `@supports`, feature detection, or where the missing behaviour is purely cosmetic.

- **Anchor positioning as a whole** (`position-visibility: anchor-valid | anchor-visible` missing outside Safari 27). The core (`anchor-name`, `position-anchor`, `anchor()`, `anchor-size()`, `position-area`, `@position-try`, `position-try-fallbacks`) is only *Newly* available (Firefox 147, Safari 26) — still gate it for older browsers.
- **Cross-document view transitions** (`@view-transition { navigation: auto }`) — no Firefox. Harmless to add (it is ignored), but do not build navigation feedback that depends on it.
- **Animating `display` / `overlay`** with `transition-behavior: allow-discrete` — `transition-behavior` itself is Newly available, but `display` animation lacks Firefox and the `overlay` property is Chromium-only: exit animations simply do not run (or the element leaves the top layer before its exit animation finishes) there.
- **`interpolate-size: allow-keywords` and `calc-size()`** (animating to `height: auto`) — Chromium only.
- **Scroll-driven animations** (`animation-timeline: scroll()/view()`, `ScrollTimeline`) — no Firefox.
- **Container scroll-state queries** (`@container scroll-state(stuck: top)`) — Chromium only.
- **`if()`, `@function`, typed `attr()`** (`attr(data-x type(<length>))`, Chrome 133, Firefox 155, no Safari) — `attr()` outside `content` is not Baseline.
- **`text-wrap: pretty`** — no Firefox. Cosmetic: fine as enhancement.
- **`text-box` (`text-box-trim`/`-edge`)** — the properties are Newly available (2026-08-18, Firefox 154), but the feature stays Limited because the `<text-edge>` values (`cap`, `alphabetic`, `ex`, `text`) are not recorded for Firefox. Cosmetic: fine as enhancement.
- **`reading-flow`, scroll markers (`::scroll-marker`, `::scroll-button()`)**, `corner-shape`, `@custom-media`, grid lanes/masonry — Chromium-only or no implementation.
- **Customizable `<select>`** (`appearance: base-select`, `::picker(select)`) — Chrome 135 and Safari 27, not Firefox.
- **`<dialog closedby>`** — no Safari; use `requestClose()` (Newly) or your own light-dismiss handler.
- **`popover="hint"`** and **interest invokers** (`interestfor`) — Chromium (+ Firefox 153 for `hint`) only; Chrome 133–150 and Firefox 149–152 shipped an older, inconsistent `hint` behaviour.
- **`hidden="until-found"`** and the `beforematch` event — Chrome 102, Firefox 148; Safari 26.2 supports it only partially (find-in-page does not scroll to the match correctly), and older Safari treats it like plain `hidden` (content not findable).
- **`accent-color`** — only partial on Chrome Android per BCD (the control's minimum contrast is not maintained), so check contrast yourself; acceptable as cosmetic enhancement.
- **`overscroll-behavior`** — works on real scroll containers everywhere (Chrome 63, Firefox 59, Safari 16); it is Limited because Safari (and Chrome < 144, Firefox < 150) ignore it on scroll containers without scrollable overflow, e.g. a modal's `overflow: auto` wrapper whose content fits.
- **`sizes="auto"`** — no Safari; keep an explicit `sizes` fallback (`sizes="auto, 100vw"`).
- **Speculation rules**, **`blocking="render"`**, **credentialless iframes**, **`CloseWatcher`**, **`moveBefore()`** — not Baseline.
- **`<input type="color">` and `<datalist>`** — missing on Firefox Android per BCD.
- **`-webkit-line-clamp` substitutes**: the unprefixed `line-clamp` is unsupported everywhere; the prefixed form needs `display: -webkit-box; -webkit-box-orient: vertical`.
- **`color-mix()` with three or more colours** — Firefox 150 and Safari 27 only.
- **`clip`** — deprecated by CSS Masking; use `clip-path` (Widely available).

Feature detection patterns (all *Chromium-verified* to parse):

```css
/* Gate layout-critical new features */
@supports (anchor-name: --a) {
  .tooltip { position-anchor: --trigger; position-area: block-start; }
}

/* Selector support: note that CSS.supports('selector(...)') never applies
   forgiving parsing, so test one selector at a time */
@supports selector(:has(> img)) {
  .card:has(> img) { grid-template-rows: auto 1fr; }
}

/* Gate on the negative to add a fallback */
@supports not (field-sizing: content) {
  textarea.autosize { min-block-size: 6lh; }
}
```

```js
// HTML/DOM features: detect the IDL member, not the browser
const hasCommand = 'command' in HTMLButtonElement.prototype;
const hasPopover = HTMLElement.prototype.hasOwnProperty('popover');
const hasRequestClose = typeof HTMLDialogElement.prototype.requestClose === 'function';
```

***

#### Document Structure and Metadata

##### Minimal correct document

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Order history – Example Shop</title>
    <meta name="description" content="Your past orders and invoices.">
    <meta name="color-scheme" content="light dark">
    <link rel="stylesheet" href="/css/app.css">
    <script src="/js/app.js" type="module"></script>
  </head>
  <body>
    <a class="skip-link" href="#main">Skip to main content</a>
    <header>…<nav aria-label="Main">…</nav></header>
    <main id="main">…</main>
    <footer>…</footer>
  </body>
</html>
```

Review rules:

- **`<!doctype html>` is mandatory.** Without it the page renders in quirks mode (different box sizing for tables, line-height quirks, `%` heights).
- **`<html lang>` is required** (WCAG 3.1.1 Language of Page, Level A). Screen readers pick pronunciation from it; `hyphens: auto` and `:lang()` depend on it. Mark passages in other languages with `lang` too (3.1.2, AA).
- **`<meta charset="utf-8">` must be within the first 1024 bytes** of the document.
- **Every page needs a unique, descriptive `<title>`** (WCAG 2.4.2, Level A). In single-page apps, update `document.title` on route change.
- **Viewport**: `width=device-width, initial-scale=1`. **Never** add `maximum-scale=1`, `minimum-scale`, or `user-scalable=no` — they block pinch zoom where honoured and fail WCAG 1.4.4 Resize Text.
- **Script loading**: `type="module"` scripts are deferred by default; classic scripts in `<head>` need `defer` (or `async` for independent scripts like analytics). A classic `<script src>` without either blocks parsing.
- **Do not use `<meta http-equiv="refresh">`** for timed redirects or reloads (WCAG 2.2.1 Timing Adjustable / 3.2.5); redirect on the server.
- **Duplicate IDs are bugs.** `getElementById`, `<label for>`, `aria-labelledby`, `aria-describedby`, `aria-controls`, fragment links and `popovertarget`/`commandfor` all resolve to the first match. Components rendered more than once must generate IDs (`useId()` in React, etc.). WCAG 2.2 removed 4.1.1 Parsing, but duplicate IDs that break name/role/value still fail 1.3.1 and 4.1.2.

##### Social and structured metadata (house rules)

```html
<!-- Open Graph: absolute URLs only -->
<meta property="og:title" content="Order history">
<meta property="og:description" content="Your past orders and invoices.">
<meta property="og:image" content="https://example.com/og/orders.png">
<meta property="og:url" content="https://example.com/orders">
<link rel="canonical" href="https://example.com/orders">

<!-- Structured data: prefer JSON-LD over microdata; it is easier to keep in sync -->
<script type="application/ld+json">
{ "@context": "https://schema.org", "@type": "Article",
  "headline": "Article title", "datePublished": "2026-01-15",
  "author": { "@type": "Person", "name": "Author Name" } }
</script>
```

Microdata (`itemscope`/`itemprop`) remains valid; when used, keep the visible text and the machine-readable value in agreement (`<time itemprop="datePublished" datetime="2026-01-15">15 January 2026</time>`). JSON-LD that is injected from user data must be serialised with `<` escaped as `\u003c`, or a value containing `</script>` ends the `<script>` element early (see Security).

***

#### Semantic HTML

##### Landmarks and sectioning

| Element | Implicit role | Notes |
|---|---|---|
| `<header>` | `banner` only when not inside `article`/`aside`/`main`/`nav`/`section` | One page-level banner |
| `<nav>` | `navigation` | Label each when there is more than one (`aria-label="Main"`, `"Breadcrumb"`) |
| `<main>` | `main` | Exactly one visible `<main>` per page |
| `<aside>` | `complementary` | |
| `<footer>` | `contentinfo` only at page level | |
| `<section>` | `region` **only when it has an accessible name** | *(Chromium-verified: unnamed `<section>` is exposed as a generic container; with `aria-labelledby` it becomes `region`)* |
| `<form>` | `form` only when named | |
| `<search>` | `search` | **W** since 2026-04-13; replaces `<form role="search">` / `<div role="search">` |
| `<article>` | `article` | Self-contained content (post, comment, card that stands alone) |

```html
<!-- BAD: section used as a styling div; region landmark noise if named, meaningless if not -->
<section class="flex-row">…</section>

<!-- GOOD: div for styling, section for a titled part of the document -->
<div class="flex-row">…</div>
<section aria-labelledby="features-h">
  <h2 id="features-h">Features</h2>
  …
</section>
```

##### Headings

- One `<h1>` describing the page, then a hierarchy without gaps that reflects structure (WCAG 1.3.1, 2.4.6). Choose the heading level for structure, the CSS for size.
- The HTML "outline algorithm" never shipped in any browser and has been removed from the spec: nested `<section><h1>` does **not** become a level-2 heading. Use explicit `h1`–`h6`.
- Do not use headings for visual emphasis (`<h4 class="small-bold">Note</h4>` in body text).

##### Buttons vs. links

```html
<!-- BAD: link that performs an action; div/span "buttons" -->
<a href="#" onclick="save()">Save</a>
<span class="btn" onclick="save()">Save</span>

<!-- GOOD: actions are buttons, navigation is links -->
<button type="button" id="save">Save</button>
<a href="/orders/42">Order 42</a>
```

- A `<button>` inside a `<form>` defaults to **`type="submit"`** *(Chromium-verified: clicking `<button>Toggle</button>` in a form submits it)*. Every non-submit button in or near a form needs `type="button"`.
- `<a>` without `href` is not focusable and not a link *(Chromium-verified: exposed as generic, `focus()` has no effect)*. Placeholder links need `href` or should be buttons.
- Never nest interactive content (a `<button>` inside an `<a>`, a link inside a `<button>`, an `<input>` inside `<summary>` beyond the label). The parser or the accessibility tree produces surprising results.

##### Lists, tables, figures, time

```html
<!-- Lists: navigation menus, steps, card grids are lists -->
<ul class="card-grid">
  <li><article class="card">…</article></li>
</ul>

<!-- Data tables: caption, th with scope; never use tables for layout -->
<table>
  <caption>Invoices, 2026</caption>
  <thead><tr><th scope="col">Number</th><th scope="col">Amount</th></tr></thead>
  <tbody><tr><th scope="row">INV-001</th><td>€120.00</td></tr></tbody>
</table>

<!-- Figures -->
<figure>
  <img src="chart.png" alt="Revenue rose from €1.2 M in Q1 to €1.6 M in Q4 2026">
  <figcaption>Quarterly revenue, 2026</figcaption>
</figure>

<!-- Machine-readable dates -->
<time datetime="2026-01-15">15 January 2026</time>

<!-- Definition / key-value lists -->
<dl>
  <dt>Status</dt><dd>Shipped</dd>
  <dt>Carrier</dt><dd>DHL</dd>
</dl>
```

- `list-style: none` removes list semantics in Safari/VoiceOver (a deliberate WebKit heuristic); Chromium keeps the list role *(Chromium-verified)*. When the count/"list" announcement matters, add `role="list"` to the `<ul>` — this is one of the few redundant-looking roles that is justified.
- `<address>` is for contact information of the nearest `<article>`/`<body>`, not for arbitrary postal addresses; it is not allowed inside `<p>`.
- `<b>`/`<i>` are valid for stylistic offsets; use `<strong>` for importance and `<em>` for stress emphasis.

##### Disclosure and accordions

```html
<!-- Native disclosure: keyboard, state and semantics for free -->
<details>
  <summary>Shipping details</summary>
  <p>Ships in 2–3 business days.</p>
</details>

<!-- Exclusive accordion: Baseline Newly available (2024-09-03) — older
     browsers simply allow several open panels, which is an acceptable fallback -->
<details name="faq" open><summary>Returns</summary>…</details>
<details name="faq"><summary>Warranty</summary>…</details>
```

*(Chromium-verified: opening the second `<details name="faq">` closes the first.)* `::details-content` (Newly, 2025-09-16) lets you style/animate the panel; `:open` (Newly, 2026-05-11) matches open `<details>`, `<dialog>`, `<select>` and `<input>` pickers — use `[open]` for wider support on `<details>`/`<dialog>`.

***

#### Accessibility (WCAG 2.2 AA)

##### Status of the standards

- **WCAG 2.2** became a W3C Recommendation on 5 October 2023; the current edition is dated **12 December 2024**. It is also **ISO/IEC 40500:2025** (announced by W3C on 21 October 2025).
- WCAG 2.2 **removed 4.1.1 Parsing** and added nine success criteria (six at A/AA).
- In the EU, the **European Accessibility Act** (Directive (EU) 2019/882) applies since **28 June 2025**. The harmonised standard **EN 301 549 V4.1.1** (published 2 September 2026) references **WCAG 2.2 Level AA**; until the European Commission cites it in the Official Journal (expected December 2026), V3.2.1 (WCAG 2.1) remains the cited version.
- WCAG 3 is a Working Draft and is not a conformance target.

##### New A/AA criteria in WCAG 2.2 that reviewers must check

| SC | Level | What to look for in code |
|---|---|---|
| **2.4.11 Focus Not Obscured (Minimum)** | AA | Sticky headers/footers, cookie banners and chat widgets must not *entirely* cover the focused element. Fix with `scroll-padding-block` matching sticky bars. |
| **2.5.7 Dragging Movements** | AA | Drag-and-drop (sortable lists, sliders, maps) needs a single-pointer alternative (buttons, click-to-move). |
| **2.5.8 Target Size (Minimum)** | AA | Pointer targets at least **24 × 24 CSS px**, or spaced so a 24 px circle centred on each does not intersect another target/circle; exceptions for inline links in text, an equivalent control, UA-default controls, essential presentation. |
| **3.2.6 Consistent Help** | A | Help links/contact mechanisms repeated across pages stay in the same relative order. |
| **3.3.7 Redundant Entry** | A | Do not make users re-type information they already gave in the same process (auto-populate or offer to select, e.g. "Billing same as shipping"). |
| **3.3.8 Accessible Authentication (Minimum)** | AA | No cognitive function test without an alternative or assistance: **do not block paste or password managers** on login, OTP and password fields; support `autocomplete` (see Forms). CAPTCHAs need a non-cognitive alternative (object recognition is allowed). |

Long-standing AA criteria that code review catches most often: 1.1.1 text alternatives; 1.3.1 info and relationships (labels, headings, table headers); 1.3.5 identify input purpose (`autocomplete`); 1.4.3 contrast **4.5:1** for text, **3:1** for large text (≥ 24 px, or ≥ 18.66 px bold); 1.4.4 resize text to 200 %; 1.4.10 reflow at **320 CSS px** without two-dimensional scrolling; 1.4.11 non-text contrast 3:1 (borders of inputs, focus indicators, icons); 1.4.12 text spacing (line height 1.5, paragraph spacing 2 em, letter spacing 0.12 em, word spacing 0.16 em must not break layout — so no fixed heights on text containers); 1.4.13 content on hover/focus (dismissible with Esc, hoverable, persistent); 2.1.1 keyboard; 2.1.2 no keyboard trap; 2.4.3 focus order; 2.4.7 focus visible; 2.5.3 label in name; 3.3.1/3.3.2 error identification and labels; 4.1.2 name, role, value; 4.1.3 status messages.

##### The rules of ARIA

From *Using ARIA* and *ARIA in HTML*:

1. **Use a native element** (`<button>`, `<a href>`, `<input type=checkbox>`, `<dialog>`, `<details>`) instead of re-purposing a `div` with a role.
2. **Do not change native semantics** unless you must (`<h2 role="tab">` is wrong; wrap instead).
3. **All interactive ARIA controls must be keyboard operable** — a `role="button"` needs `tabindex="0"` *and* Enter and Space handling.
4. **Do not use `role="presentation"`/`"none"` or `aria-hidden="true"` on a focusable element.** *(Chromium-verified: a `<button aria-hidden="true">` disappears from the accessibility tree but still receives focus — a screen-reader user tabs onto "nothing".)* Use `inert` to remove a subtree from both.
5. **All interactive elements must have an accessible name.**

```html
<!-- BAD: ARIA reimplementing a button, missing keyboard support -->
<div role="button" onclick="toggle()">Menu</div>

<!-- Still worse than native: needs tabindex + Enter/Space key handling -->
<div role="button" tabindex="0">Menu</div>

<!-- GOOD -->
<button type="button" aria-expanded="false" aria-controls="menu">Menu</button>
<ul id="menu" hidden>…</ul>
```

Common ARIA review findings:

- **Redundant roles** on native elements (`<button role="button">`, `<nav role="navigation">`, `<img alt="" role="presentation">`) are noise; ARIA in HTML says authors *should not* set them. (`<ul role="list">` with `list-style: none` is the pragmatic exception above.)
- **`aria-label` on generic elements** (`div`, `span` without a role) is *prohibited* by ARIA 1.2 and inconsistently announced. *(Chromium 141 still computes a name for it, which hides the bug in Chromium-only testing.)* Give the element a role or put the text in a real label.
- **`aria-label` overrides visible text.** The accessible name must contain the visible label text (WCAG 2.5.3 Label in Name), or voice-control users cannot activate the control by what they see.
- **`aria-describedby`/`aria-labelledby` pointing to a missing ID** silently produce no description.
- **Live regions must exist in the DOM before content is injected.** Inserting `<div role="alert">Saved</div>` and filling it in the same task is often not announced. Render the empty region at page load and update its text. Use `role="status"` (polite) for confirmations, `role="alert"` (assertive) only for errors needing immediate attention. `ariaNotify()` is only Newly available (2026-09-14).
- **`aria-disabled="true"` vs `disabled`**: `disabled` removes the control from tab order and from form submission; `aria-disabled` keeps it focusable (good for explaining *why* it is disabled) but does **not** prevent activation — the click handler must check it. *(Chromium-verified: both expose `disabled=true`; only the `aria-disabled` button is focusable.)*
- **`aria-expanded` must be kept in sync** by script. Popover invokers (`popovertarget`) get the expanded state automatically *(Chromium-verified: exposed as `expanded=false` without any attribute)* — do not add a manual `aria-expanded` that can drift.
- **`role="application"`**, `aria-owns`, and custom `aria-roledescription` are rarely right in product UI; ask for justification.
- **`tabindex` > 0 is always a bug** (it reorders the whole page). `tabindex="-1"` is for programmatic focus targets (headings after route change, error summaries).
- **Placeholder is not a label.** Chromium falls back to the placeholder for the accessible name *(Chromium-verified)*, which hides the bug, but the text disappears on input, usually fails contrast, and is not reliably announced across screen readers.

##### Accessible names, images and icons

```html
<!-- Informative image: describe the information, not the pixels -->
<img src="q4.png" alt="Sales increased 20 % in Q4 2026">

<!-- Decorative image: empty alt (no role needed) -->
<img src="divider.svg" alt="">

<!-- Missing alt is never correct: Chromium exposes an unnamed image and
     screen readers may read the file name -->
<img src="IMG_2381.jpg">  <!-- BAD -->

<!-- Icon-only button: name on the button, icon hidden -->
<button type="button" aria-label="Close dialog">
  <svg aria-hidden="true" focusable="false" viewBox="0 0 24 24">…</svg>
</button>

<!-- Better: visible-to-AT text, works with translation tools -->
<button type="button">
  <svg aria-hidden="true" viewBox="0 0 24 24">…</svg>
  <span class="visually-hidden">Close dialog</span>
</button>

<!-- Link wrapping only an image: the image alt IS the link text -->
<a href="/"><img src="logo.svg" alt="Example Shop home"></a>

<!-- Complex image: short alt + long description nearby -->
<figure>
  <img src="flow.png" alt="Registration flow, described below" aria-describedby="flow-desc">
  <figcaption id="flow-desc">Step 1: enter email. Step 2: verify. Step 3: set password.</figcaption>
</figure>
```

- A link whose only content is `<img alt="">` gets its name from `title` or nothing *(Chromium-verified: named "tooltip only" from `title`)*. `title` is not a substitute for real text: it is not shown on touch and not reliably announced.
- Inline SVG used as an informative image: `role="img"` plus `aria-label` (or `<title>` as the first child).
- CSS `content` text (e.g. `::before { content: "New" }`) is exposed to assistive technology; use the alternative-text syntax `content: "★" / ""` for decorative glyphs (Newly available 2024-07-09; Firefox 128, Safari 17.4; *Chromium-verified* to parse). Older browsers drop the whole declaration, so put a plain `content: "★";` before it.

##### Visually hidden text

```css
/* Updated pattern: clip is deprecated (CSS Masking); clip-path is Widely available */
.visually-hidden:not(:focus):not(:active) {
  position: absolute;
  inline-size: 1px;
  block-size: 1px;
  margin: -1px;
  padding: 0;
  border: 0;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}
```

*(Chromium-verified: text styled this way stays in the accessibility tree.)* The deprecated `clip: rect(0 0 0 0)` of older versions of this pattern still works but should be migrated. Do not use `display: none`, `visibility: hidden`, `hidden`, or `font-size: 0` for text meant for screen readers — those remove it. Conversely, `aria-hidden="true"` hides from assistive technology without any CSS; it does not hide visually.

##### Keyboard and focus

```css
/* BAD: removes the only focus indicator */
:focus { outline: none; }
button:focus { outline: 0; }

/* GOOD: keyboard-only ring, high contrast, never clipped */
:focus-visible {
  outline: 3px solid var(--focus-ring, currentColor);
  outline-offset: 2px;
}

/* Keep the focused element visible under sticky bars (WCAG 2.4.11) */
html { scroll-padding-block-start: var(--sticky-header-height, 4rem); }
```

- `:focus-visible` (Widely) matches keyboard focus and not mouse clicks on buttons *(Chromium-verified: Tab → matches; mouse click → focused but does not match)*. Text inputs match `:focus-visible` on click too, by design.
- Use `outline` rather than `box-shadow` for focus rings: `outline` stays visible in Windows forced-colors mode; `box-shadow` is removed there. If a design requires `box-shadow`, add `outline: 2px solid transparent` so forced-colors mode draws it.
- Skip link: first focusable element, targets `<main id="main">`; it may be visually hidden until focused.
- **Focus management in SPAs**: after client-side navigation move focus to the new page's `<h1>` (with `tabindex="-1"`) or main region, and update `document.title`. After deleting an item, move focus to a sensible neighbour, not `<body>`.
- **Focus order follows DOM order.** CSS `order`, `flex-direction: row-reverse`, grid placement and absolute positioning change the visual order but not the tab order (WCAG 2.4.3, 1.3.2). `reading-flow` addresses this but is Chromium-only.
- **Hover/focus content (tooltips, mega menus)**: dismissible with Esc without moving focus, hoverable (pointer can move onto it), persistent until dismissed (WCAG 1.4.13).

##### Target size (WCAG 2.5.8)

```css
/* Icon buttons: ensure at least 24×24 CSS px hit area (44×44 recommended for touch) */
.icon-button {
  min-inline-size: 24px;
  min-block-size: 24px;
  display: inline-grid;
  place-items: center;
}
/* Enlarge the hit area of a small visual without changing layout */
.icon-button--tiny { position: relative; }
.icon-button--tiny::after { content: ""; position: absolute; inset: -6px; }
```

***

#### Forms

##### Labels, groups and descriptions

```html
<form action="/account" method="post" novalidate>
  <!-- Explicit label -->
  <label for="email">Email address</label>
  <input id="email" name="email" type="email" autocomplete="email" required
         aria-describedby="email-hint email-error">
  <p id="email-hint" class="hint">We will send the receipt here.</p>
  <p id="email-error" class="error" hidden></p>

  <!-- Radio/checkbox groups: fieldset + legend -->
  <fieldset>
    <legend>Preferred contact method</legend>
    <label><input type="radio" name="contact" value="email"> Email</label>
    <label><input type="radio" name="contact" value="phone"> Phone</label>
  </fieldset>

  <!-- Native select; first option as an explicit empty choice -->
  <label for="country">Country</label>
  <select id="country" name="country" autocomplete="country" required>
    <option value="">Select a country</option>
    <option value="DE">Germany</option>
  </select>

  <button type="submit">Save</button>
</form>
```

*(Chromium-verified: the input is named "Email address" with description "We will send the receipt here."; the fieldset is exposed as a group named by its legend.)*

Review rules:

- Every form control has a programmatic label (`<label for>`, wrapping `<label>`, or `aria-labelledby`). `aria-label` is a last resort for controls with visible context (search fields with a visible button).
- Required fields: `required` (native) or `aria-required="true"` (custom); indicate it visually in text, not only by colour or an asterisk without explanation.
- Error messages: associate with `aria-describedby` (`aria-errormessage` support is still uneven), set `aria-invalid="true"` only after validation fails, move focus to an error summary or the first invalid field on submit, and describe how to fix the error (WCAG 3.3.1, 3.3.3).
- **Do not validate on every keystroke with error styling.** `:invalid` matches immediately on page load for empty required fields *(Chromium-verified)*; use **`:user-invalid`** (Widely available since 2026-05-02), which matches only after the user has interacted *(Chromium-verified: false on load, true after typing "nope" into a required `type=email` and leaving the field)*.
- Do not disable the submit button until the form is valid — users cannot discover what is wrong. Validate on submit and report.
- `novalidate` on the `<form>` disables native bubbles; then the code must perform and announce validation itself.

##### Input types, `autocomplete`, `inputmode`

```html
<input type="email"    name="email" autocomplete="email">
<input type="tel"      name="phone" autocomplete="tel">
<input type="url"      name="site">
<input type="text"     name="postcode" autocomplete="postal-code" inputmode="numeric">
<input type="text"     name="otp" autocomplete="one-time-code" inputmode="numeric" maxlength="6">
<input type="date"     name="dob" autocomplete="bday" min="1900-01-01">
<input type="search"   name="q" enterkeyhint="search">

<!-- Login: let password managers work (WCAG 3.3.8) -->
<input type="text"     name="username" autocomplete="username">
<input type="password" name="password" autocomplete="current-password">

<!-- Sign-up / change password -->
<input type="password" name="new-password" autocomplete="new-password" minlength="12">

<!-- Passkeys via conditional UI -->
<input type="text" name="username" autocomplete="username webauthn">

<!-- Payment -->
<input name="cc-number" autocomplete="cc-number" inputmode="numeric">
<input name="cc-exp"    autocomplete="cc-exp">
```

- **`autocomplete` tokens** come from the HTML spec's autofill field list (`name`, `given-name`, `family-name`, `email`, `tel`, `street-address`, `address-line1`, `postal-code`, `country`, `bday`, `username`, `current-password`, `new-password`, `one-time-code`, `cc-name`, `cc-number`, `cc-exp`, `cc-csc`, …), optionally with section/`shipping`/`billing` prefixes. Invented tokens (`autocomplete="nope"`, `"disabled"`) are ignored. They are required for WCAG 1.3.5 Identify Input Purpose.
- **`autocomplete="off"` on login fields does not stop password managers** (browsers deliberately ignore it for credentials). Do not use it to "secure" passwords; it only harms WCAG 3.3.8.
- **Do not block paste** (`onpaste="return false"`) on password, email-confirmation or OTP fields.
- **Use `type="number"` only for real quantities.** For postcodes, card numbers, IDs, phone numbers use `type="text" inputmode="numeric"`: `number` drops leading zeros, accepts `e`, changes value on scroll-wheel, and shows spinners.
- Date/time inputs are Widely available (2024-04-05); `<input type="color">` and `<datalist>` are not Baseline (Firefox Android).
- **`pattern` is compiled with the RegExp `v` flag.** Unescaped `-` (or other "syntax characters") inside a character class make the pattern **invalid, and an invalid pattern is ignored entirely** — every value passes *(Chromium-verified: `pattern="[a-z-]+"` gives no `patternMismatch` for "ABC" and logs a console error; `pattern="[a-z\-]+"` works)*. The pattern is anchored implicitly (`^(?:…)$`).
- Client-side validation is UX only; the server must re-validate everything.

##### `field-sizing` (Newly available 2026-06-16)

```css
/* Auto-growing textarea without JavaScript (Chrome 123, Firefox 152, Safari 26.2) */
textarea {
  field-sizing: content;
  min-block-size: 3lh;       /* field-sizing: content shrinks an empty field to almost nothing */
  max-block-size: 20lh;
  min-inline-size: 20ch;
}
```

*(Chromium-verified: an empty `field-sizing: content` textarea measured 7 px wide; after typing one short sentence it grew to fit the text, about 280 px — always set minimum sizes.)* Older browsers ignore it and keep the fixed size, which is an acceptable fallback.

##### Styling form controls

- `appearance: none` (Widely) removes native styling — then you must restore focus, checked, disabled, invalid and forced-colors states yourself.
- `accent-color` is **not Baseline** (only partial on Chrome Android, which does not keep the control legible); treat as cosmetic and check contrast.
- Customizable `<select>` (`appearance: base-select`) is Chrome 135 / Safari 27 only; do not ship a design that relies on it without the native look as fallback.

***

#### Dialogs, Popovers, Invokers and `inert`

##### `<dialog>` (Widely available)

Specification and reference: HTML Standard, MDN.

```html
<button type="button" id="open-delete">Delete project…</button>

<dialog id="confirm-delete" aria-labelledby="cd-title">
  <h2 id="cd-title">Delete project?</h2>
  <p>This cannot be undone.</p>
  <form method="dialog">
    <button value="cancel" autofocus>Cancel</button>
    <button value="delete">Delete</button>
  </form>
</dialog>

<script type="module">
  const dlg = document.getElementById('confirm-delete');
  document.getElementById('open-delete').addEventListener('click', () => dlg.showModal());
  dlg.addEventListener('close', () => {
    if (dlg.returnValue === 'delete') deleteProject();
  });
</script>
```

Behaviour *(all Chromium-verified)*:

- `showModal()` puts the dialog in the **top layer** (above every `z-index`), matches `:modal`, and makes the rest of the document **inert** — focus cannot leave the dialog.
- Initial focus goes to the first `autofocus` descendant, otherwise to the first focusable descendant. Put `autofocus` on the least destructive action.
- **Esc closes a modal dialog** (fires `cancel`, then `close`) and focus returns to the previously focused element.
- `<form method="dialog">` closes the dialog and sets `returnValue` to the submitting button's `value`.
- Setting the **`open` attribute directly** (or `show()`) creates a **non-modal** dialog: no top layer, no inert background, no Esc handling. Do not toggle `open` for modals.

Review rules:

- **BAD**: `<div class="modal" role="dialog">` with a hand-written focus trap — replace with `<dialog>` + `showModal()`.
- Give the dialog an accessible name (`aria-labelledby` its heading). `aria-modal="true"` is redundant with `showModal()`.
- Do not put `tabindex` on `<dialog>` itself.
- Styling: style `dialog::backdrop` for the overlay. The UA stylesheet sets `display: none` on closed dialogs — **`dialog { display: flex; }` makes a closed dialog visible**; use `dialog[open] { display: flex; }`.
- Light dismiss (click on backdrop): `closedby="any"` is **not Baseline** (no Safari). Implement it by listening for clicks whose `target === dlg` and whose coordinates fall outside the dialog's box, or use a full-size inner wrapper.
- `requestClose()` (Newly, 2025-05-27) fires `cancel` so the dialog can veto closing ("unsaved changes?") *(Chromium-verified: `preventDefault()` in `cancel` keeps the dialog open)*; `close()` fires no `cancel` and cannot be vetoed. Browsers may ignore repeated `cancel` vetoes on Esc without intervening user activation, so never use a veto as the only way to keep data.
- Prevent background scroll while modal: `html:has(dialog:modal) { overflow: hidden; }` (`:has()` Widely, `:modal` Widely).

##### Popover API (Newly available 2025-01-27)

```html
<button type="button" popovertarget="account-menu">Account</button>
<div id="account-menu" popover>
  <a href="/settings">Settings</a>
  <a href="/logout">Log out</a>
</div>
```

Behaviour *(Chromium-verified)*: `popover` (= `"auto"`) renders in the top layer, closes on Esc and on click outside ("light dismiss"), closes other auto popovers, and the invoker button is exposed as `expanded=false/true` automatically.

```css
/* BAD: an author display value overrides the UA [popover]:not(:popover-open) { display: none } */
[popover].menu { display: flex; }        /* the menu is visible while "closed" */

/* GOOD */
.menu:popover-open { display: flex; }
```

*(Chromium-verified: the closed `[popover].menu { display: flex }` element has a non-zero height.)*

- A popover is **not modal**: no inert background, no focus trap. Use `<dialog>.showModal()` when the user must respond.
- `popover="manual"` has no light dismiss; you must provide a close control.
- Keep the popover in the DOM right after its invoker so the tab order makes sense (focus is not moved into it automatically).
- Popover is Newly available — Chrome 114 (the whole feature from 116), Firefox 125, Safari 17. Safari on iOS 17–18.2 supports it, but tapping outside does not light-dismiss the popover (fixed in 18.3, which is why the feature counts as Newly available only from 2025-01-27). On those iOS versions always provide a visible close control. For audiences with pre-2024 browsers, keep a non-popover fallback or polyfill.
- `popover="hint"`, `interestfor` (hover/focus-triggered popovers) are not Baseline.
- Positioning popovers next to the invoker: anchor positioning is Newly available for its core but not Baseline as a whole; fall back to a centred or JS-positioned popover.

##### Invoker commands (Newly available 2025-12-12)

```html
<!-- Declarative: no JS needed to open a modal dialog (Chrome 135, Firefox 144, Safari 26.2) -->
<button type="button" commandfor="settings" command="show-modal">Settings</button>
<dialog id="settings">
  …
  <button type="button" commandfor="settings" command="close">Close</button>
</dialog>
```

Built-in commands: `show-modal`, `close`, `request-close` (dialogs; `request-close` only from Chrome 139); `show-popover`, `hide-popover`, `toggle-popover` (popovers). Custom commands start with `--` and dispatch a `command` event on the target. *(Chromium-verified: `command="show-modal"` opened the dialog as `:modal` and focused its `autofocus` button.)* Because this is only Newly available, keep a `click` handler fallback when the target audience includes Safari < 26.2 / Firefox < 144:

```js
if (!('command' in HTMLButtonElement.prototype)) {
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('button[commandfor]');
    if (!btn) return;
    const target = document.getElementById(btn.getAttribute('commandfor'));
    if (btn.getAttribute('command') === 'show-modal') target.showModal();
    if (btn.getAttribute('command') === 'close') target.close();
  });
}
```

##### `inert` (Widely available 2025-10-11)

```html
<!-- Off-canvas navigation: hidden visually AND from keyboard/AT while closed -->
<nav id="drawer" class="drawer" inert>…</nav>
```

`inert` removes the subtree from focus, pointer interaction, find-in-page and the accessibility tree *(Chromium-verified: `focus()` on a button inside an `inert` container has no effect and the button is absent from the accessibility tree)*. Use it for off-screen drawers, carousels' hidden slides, and content behind a non-`<dialog>` overlay. Do not use `aria-hidden` for this (it does not stop focus).

##### Hidden content: which mechanism?

| Need | Use |
|---|---|
| Hidden from everyone | `hidden` attribute or `display: none` |
| Hidden visually, read by screen readers | `.visually-hidden` |
| Visible, hidden from screen readers (decorative) | `aria-hidden="true"` (never on focusable content) |
| Visible but non-interactive and hidden from AT | `inert` |
| Hidden but findable with Ctrl+F | `hidden="until-found"` — **not Baseline** (Safari only partial since 26.2); acceptable as enhancement |

`hidden` is overridden by any author `display` value: `.panel { display: grid }` makes `<div class="panel" hidden>` visible. Add `[hidden] { display: none !important; }` in the reset or write `.panel:not([hidden])`.

***

#### Security in Markup

##### Links and windows

- **`target="_blank"` implies `rel="noopener"`** in all current browsers (Widely available since 2021: Chrome 88, Firefox 79, Safari 12.1) *(Chromium-verified: `window.opener` is `null` in the new tab; `rel="opener"` restores it)*. Adding `rel="noopener"` is harmless and still recommended where very old browsers matter; `rel="noreferrer"` additionally strips the `Referer` header.
- **`window.open()` does *not* imply `noopener`.** *(Chromium-verified: the opened page sees `window.opener`.)* Pass `'noopener'` in the features string (it then returns `null`) when opening untrusted URLs.
- **Never render untrusted URLs into `href`/`src`/`action`/`formaction` without scheme allow-listing.** `javascript:` and `data:` URLs execute script. Allow `https:`, `mailto:`, `tel:` and relative URLs only.
- `rel="nofollow ugc"` on user-generated links (SEO hygiene, not security).

##### Content Security Policy-friendly markup

```html
<!-- BAD: inline handlers and inline styles need 'unsafe-inline' / 'unsafe-hashes' -->
<button onclick="save()">Save</button>
<p style="color: red">Error</p>
<a href="javascript:void(0)">…</a>

<!-- GOOD: external module + event listeners; classes instead of style attributes -->
<button type="button" id="save">Save</button>
<p class="error">Error</p>
<script type="module" src="/js/app.js"></script>

<!-- When inline script is unavoidable: per-response nonce -->
<script nonce="{{ csp_nonce }}">…</script>
```

Behaviour under `Content-Security-Policy: script-src 'nonce-…'; style-src 'self'` *(Chromium-verified)*:

- Inline scripts without the nonce, **inline event handlers** (`onclick`) and `style=""` attributes are blocked.
- Setting styles through the CSSOM (`el.style.color = …`) is **not** blocked by `style-src` — prefer it (or classes) over generated `style` attributes.
- With a **header-delivered** CSP, `getAttribute('nonce')` returns `""` (nonce hiding) while the `nonce` IDL property keeps the value; with a `<meta>`-delivered CSP the attribute stays readable. Never copy nonces from the DOM into dynamically created scripts — use `'strict-dynamic'`.
- A `<meta http-equiv="Content-Security-Policy">` **ignores `frame-ancestors`, `report-uri` and `sandbox`** *(Chromium-verified for the first two: console error "ignored when delivered via a <meta> element")*. Clickjacking protection needs the HTTP header (`frame-ancestors`, or `X-Frame-Options`).
- Nonces must be unpredictable and unique per response; a nonce baked into a cached HTML page or a template constant is no protection.
- Trusted Types (`require-trusted-types-for 'script'`) is Newly available (2026-02-24; Firefox 148, Safari 26) — good defence-in-depth, but do not assume enforcement in older browsers.

##### Subresource integrity and third-party code

```html
<script src="https://cdn.example.net/lib@4.2.1/lib.min.js"
        integrity="sha384-…base64…"
        crossorigin="anonymous"
        referrerpolicy="no-referrer"></script>
```

- *(Chromium-verified: a mismatching `integrity` hash blocks execution and logs "Failed to find a valid digest".)* Cross-origin SRI requires CORS (`crossorigin="anonymous"`) and a CDN that sends `Access-Control-Allow-Origin`.
- Pin exact versions in CDN URLs; `integrity` on a "latest" URL breaks on the next release.
- `integrity` covers only that one file: modules it imports are not checked unless an import map's `integrity` section lists them (that section is not tracked as a Baseline feature — check your targets).

##### Referrer policy

- The default policy is **`strict-origin-when-cross-origin`**: cross-origin requests receive only the origin *(Chromium-verified: a navigation from `https://a.test/index.html` to another origin sent `Referer: https://a.test/`)*.
- Do not put secrets or personal data in URLs regardless; same-origin requests still receive the full URL. Use `referrerpolicy="no-referrer"` on links/resources to third parties when even the origin is sensitive. Avoid `unsafe-url`.

##### Iframes

```html
<!-- Third-party or user-generated HTML -->
<iframe src="https://widgets.example.net/embed/123"
        title="Shipment tracking widget"
        sandbox="allow-scripts allow-popups"
        allow="clipboard-write"
        referrerpolicy="strict-origin-when-cross-origin"
        loading="lazy"
        width="600" height="400"></iframe>
```

- **`sandbox="allow-scripts allow-same-origin"` on same-origin content provides no sandbox**: the framed document can reach `window.frameElement` and remove the attribute (the HTML spec warns about this). *(Chromium-verified: with both tokens the child has origin `https://a.test` and a non-null `frameElement`; with `allow-scripts` alone its origin is `null` and accessing `parent.document` throws `SecurityError`.)* Serve untrusted HTML from a separate origin.
- Grant the minimum `sandbox` tokens; `allow-top-navigation` and `allow-popups-to-escape-sandbox` deserve scrutiny.
- `allow=""` (Permissions Policy) controls camera, microphone, geolocation, fullscreen, clipboard etc. for the frame; list only what the embed needs.
- Every `<iframe>` needs a `title` (accessible name).
- `srcdoc` with untrusted HTML is still script injection unless sandboxed without `allow-same-origin`.

##### Forms and data in markup

- `action` URLs must be HTTPS; mixed-content form submission triggers browser warnings.
- `<input type="hidden">` values are fully user-controlled; never trust them for prices, roles or IDs without server checks. CSRF tokens in hidden fields are fine.
- `<a download>` only works for same-origin or `blob:`/`data:` URLs; the server should still send `Content-Disposition`.
- Escaping: template engines must HTML-escape text and attribute values (including in unquoted attributes — always quote attributes). Data embedded in `<script>` (JSON for hydration, JSON-LD) must escape `<` as `\u003c` to prevent `</script>` break-out.
- `<base href>` injected by untrusted input rewrites every relative URL; CSP `base-uri 'none'` (or `'self'`) blocks that.

***

#### Loading Performance and Layout Stability

##### Images

```html
<!-- LCP / hero image: eager, high priority, sized -->
<img src="/img/hero-1200.avif"
     srcset="/img/hero-800.avif 800w, /img/hero-1200.avif 1200w, /img/hero-2000.avif 2000w"
     sizes="(width <= 700px) 100vw, 1200px"
     width="1200" height="600"
     fetchpriority="high"
     alt="Spring collection: three linen shirts on a rail">

<!-- Below-the-fold images: lazy, sized -->
<img src="/img/card-400.webp"
     srcset="/img/card-400.webp 400w, /img/card-800.webp 800w"
     sizes="(width <= 700px) 50vw, 300px"
     width="400" height="300" loading="lazy" decoding="async" alt="…">

<!-- Format fallback with picture -->
<picture>
  <source type="image/avif" srcset="/img/team.avif">
  <source type="image/webp" srcset="/img/team.webp">
  <img src="/img/team.jpg" width="800" height="500" alt="Our team at the 2026 offsite">
</picture>
```

- **Always set `width` and `height` attributes** (the intrinsic pixel size, not the display size). Browsers map them to `aspect-ratio: auto <w> / <h>` so space is reserved before the image loads, preventing layout shift (CLS). *(Chromium-verified: computed `aspect-ratio` is `auto 800 / 600`.)* Combine with `img { max-inline-size: 100%; block-size: auto; }`. Once the image loads, its natural ratio takes over (`auto`), so mismatched attributes only matter before load.
- **Never `loading="lazy"` the LCP/hero image** or anything above the fold — it delays the fetch until layout. Lazy loading is Widely available (2026-06-19) for `img` and `iframe`. *(Chromium-verified: an `img loading="lazy"` 5000 px below the fold was not fetched.)*
- `fetchpriority="high"` on the LCP image (Newly available; ignored by older browsers — safe). Use `fetchpriority="low"` for non-critical carousels' later slides. Do not mark many resources `high`.
- `srcset` with `w` descriptors **requires `sizes`** that match the CSS layout; without `sizes` the browser assumes `100vw` and downloads oversized images. *(Chromium-verified: at DPR 2 with `sizes="… 400px"`, the 800w candidate was selected.)* `sizes="auto"` (needs `loading="lazy"`) is not Baseline.
- AVIF is Widely available (2026-07-25); WebP since 2023. Keep a JPEG/PNG `img src` fallback only if the target includes older browsers.
- CSS background images are discovered late; avoid them for LCP content, or `preload` them.

##### Resource hints

```html
<!-- Preconnect only to the few origins needed for the first render -->
<link rel="preconnect" href="https://fonts.example-cdn.com" crossorigin>
<link rel="dns-prefetch" href="https://analytics.example.com">

<!-- Fonts: preload MUST have crossorigin (fonts are always fetched in CORS mode) -->
<link rel="preload" href="/fonts/inter-var.woff2" as="font" type="font/woff2" crossorigin>

<!-- Responsive LCP image discovered late (e.g. CSS background or JS-rendered) -->
<link rel="preload" as="image" href="/img/hero-1200.avif"
      imagesrcset="/img/hero-800.avif 800w, /img/hero-1200.avif 1200w"
      imagesizes="(width <= 700px) 100vw, 1200px" fetchpriority="high">

<!-- ES modules -->
<link rel="modulepreload" href="/js/app.js">
```

- *(Chromium-verified: a font `preload` without `crossorigin` is **not used** — the font is downloaded twice and the console warns "credentials mode does not match".)*
- `preconnect` to a font/CORS origin needs `crossorigin`, otherwise the warmed connection is not reused for CORS requests.
- Every `preload` must be consumed within a few seconds or Chrome warns; preloads compete with critical resources — do not preload everything.
- `as` is mandatory for `preload`; a wrong `as` causes a double fetch.
- Speculation rules (`<script type="speculationrules">`) are Chromium-only — fine as enhancement, never as a requirement.

##### Scripts and CSS

- Render-blocking: every `<link rel="stylesheet">` in `<head>` blocks first paint; keep critical CSS small and avoid `@import` chains (each level is a serial round trip).
- `media` on `<link rel="stylesheet" media="print">` makes non-matching sheets non-blocking.
- Classic scripts need `defer`; `async` scripts run in arbitrary order (fine for analytics, wrong for dependent code).
- Third-party tags (chat, A/B testing) blocking the main thread or injecting layout above content are frequent CLS/INP regressions.

##### Fonts and CLS

```css
@font-face {
  font-family: "Inter";
  src: url("/fonts/inter-var.woff2") format("woff2");
  font-weight: 100 900;
  font-display: swap;            /* or optional for non-brand fonts */
}

/* Metric-compatible fallback reduces the shift when the web font swaps in */
@font-face {
  font-family: "Inter Fallback";
  src: local("Arial");
  size-adjust: 107%;
  ascent-override: 90%;
}
body { font-family: "Inter", "Inter Fallback", system-ui, sans-serif; }
```

- `font-display` is Widely available. Without it, text can be invisible for up to ~3 s (block period).
- The `size-adjust` descriptor is Widely available (2026-03-18 per its key; Safari 17). `ascent-override`/`descent-override`/`line-gap-override` ("font metric overrides") are **not Baseline** (no Safari) — harmless where unsupported, but the fallback metrics then differ in Safari.
- Reserve space for ads, embeds and late-loading banners (`min-block-size`, `aspect-ratio`); never insert content above existing content after load unless in response to user input.

##### Rendering performance

```css
/* Animate compositor-friendly properties */
.toast { transition: transform 200ms ease-out, opacity 200ms ease-out; }

/* BAD: animating layout properties every frame */
@keyframes slide-bad { to { left: 100px; top: 100px; } }
/* GOOD */
@keyframes slide-good { to { transform: translate(100px, 100px); } }

/* will-change: only on elements about to animate, and remove afterwards */
.drawer.is-opening { will-change: transform; }

/* Skip rendering of off-screen sections — Newly available (2025-09-15) */
.feed-item {
  content-visibility: auto;
  contain-intrinsic-size: auto 400px;   /* "auto" remembers the last rendered size */
}

/* Containment for independent widgets (Widely available) */
.widget { contain: layout paint; }
```

- `content-visibility: auto` is only **Newly** available (Safari 26, Firefox 130). It is a safe enhancement because unsupporting browsers render normally. Without `contain-intrinsic-size` the scrollbar jumps as sections render.
- `content-visibility: hidden` content is still in the DOM but not in the accessibility tree and not findable; do not use it for content users must reach.
- A permanent `will-change` on many elements wastes memory (each becomes a layer).

***

#### The Cascade and Specificity

##### Order of precedence (simplified)

Per CSS Cascading Level 5/6.

1. Highest first: transitions → `!important` UA → `!important` user → **`!important` author (layers in *reverse* order, unlayered last)** → animations → normal author (**unlayered beats all layers**, later layers beat earlier ones) → user → UA.
2. Within the same origin, importance and layer: specificity, then `@scope` proximity, then source order *(Chromium-verified: a farther `@scope (.outer) { p.x }` beats a closer `@scope (.inner) { p }`; with equal specificity the closer scope beats the later rule)*.
3. Inline `style` attributes beat any selector specificity (but not `!important` in a stylesheet).

All of the following are *Chromium-verified*:

```css
@layer base, components;

@layer components { .x { color: blue; } }
.x { color: red; }                        /* RED: unlayered normal beats layered normal,
                                              regardless of specificity or order */

@layer base { .imp { color: green !important; } }
.imp { color: red !important; }           /* GREEN: for !important the order flips —
                                              earlier layers beat later layers and unlayered */
```

Review consequences:

- When adopting layers, put **all** first-party CSS in layers or none of it; one unlayered legacy rule silently beats every layered rule.
- Third-party CSS can be layered on import: `@import url("vendor.css") layer(vendor);` — `@import` must come before all rules except `@charset` and `@layer` statements.
- `!important` inside a low-priority layer (e.g. a reset) wins against *everything* later — never put `!important` in resets/base layers unless you mean "unoverridable".
- `revert-layer` rolls a property back to the previous layer's value.

##### Specificity rules that cause real bugs

```css
/* :is() takes the specificity of its MOST specific argument */
:is(#hero, .banner) .title { color: red; }   /* (1,1,0) even when matching .banner */
.banner.banner .title     { color: blue; }   /* (0,3,0) loses — Chromium-verified */

/* :where() always contributes zero — ideal for defaults/resets */
:where(.prose) h2 { margin-block: 1.5em 0.5em; }

/* :not(), :has() also take their most specific argument */
.btn:not(#legacy) { }                         /* (1,1,0) */

/* Nesting: & behaves like :is(<parent list>) */
.card, #promo {
  & .title { color: red; }                    /* = :is(.card, #promo) .title → (1,1,0) */
}
```

*(Chromium-verified: the nested rule above beats `.p .c.c` (0,3,0) for an element under `.card`.)*

- **ID selectors in component CSS** create specificity wars; prefer classes, or wrap in `:where()`.
- **Selector lists are invalidated by a single bad selector**: `.a:has(.b), .a:unknown-thing { … }` drops the whole rule *(Chromium-verified)*. `:is()` and `:where()` use *forgiving* lists (`:is(.a, :unknown)` still matches `.a`), but **`:has()` is unforgiving** (`.c:has(.k, :unknown)` is dropped) and **`:has()` cannot be nested** (`:has(:has(…))` is invalid) — all *Chromium-verified*. Do not mix experimental pseudo-classes into shared selector lists.
- `CSS.supports('selector(:is(a, :unknown))')` returns `false` even though the stylesheet would accept it — `@supports selector()` does not apply forgiving parsing.

##### Custom properties pitfalls

```css
.parent { color: rgb(9 9 9); }
.child {
  --gap: 20px;
  color: var(--gap, red);   /* NOT red: the var() resolves (to 20px), which is
                               invalid for color → "invalid at computed-value time"
                               → behaves like unset → inherits rgb(9 9 9) */
}
```

*(Chromium-verified.)* The fallback in `var(--x, fallback)` is used only when `--x` is *not defined* (guaranteed-invalid), not when its value is unsuitable. Typed custom properties fix this:

```css
/* Registered custom property — Newly available (2024-07-09; Firefox 128) */
@property --brand {
  syntax: "<color>";
  inherits: false;
  initial-value: rebeccapurple;
}
.x { --brand: 20px; color: var(--brand); }   /* invalid at parse time → initial-value */
```

*(Chromium-verified: the element gets the registered `initial-value`.)* `@property` also makes custom properties animatable (`transition: --angle 1s`). `syntax` and `inherits` are always required, and `initial-value` is required unless `syntax` is `"*"`; an invalid `@property` rule is ignored entirely.

- Custom properties are **inherited by default** and resolved per element; a token defined on `:root` but overridden by a component's `--color` leaks into all descendants.
- Do not store units separately (`--size: 16; width: calc(var(--size) * 1px)`) unless needed for math; prefer full values.

***

#### Modern CSS Features (with Baseline status)

##### Cascade layers — Widely available (2024-09-14)

```css
/* Declare order once, first thing in the entry stylesheet */
@layer reset, base, vendor, components, utilities;

@import url("normalize.css") layer(reset);

@layer components {
  .button { padding-block: 0.5rem; }
}
@layer utilities {
  .mt-0 { margin-block-start: 0; }   /* wins over components without !important */
}
```

Review: layer order is fixed by the **first** occurrence of each name; a later `@layer utilities, components;` does not reorder. Anonymous `@layer { }` blocks create a new layer each time (cannot be appended to later).

##### `:has()` — Widely available (2026-06-19)

```css
/* Parent/previous-sibling selection */
.card:has(> img) { grid-template-rows: auto 1fr; }
.field:has(input:user-invalid) .field__hint { color: var(--danger); }
h2:has(+ p) { margin-block-end: 0.25rem; }
html:has(dialog:modal) { overflow: hidden; }
```

Rules: unforgiving argument list; no nesting of `:has()`; pseudo-elements are not allowed inside (`:has(::before)` is invalid) *(Chromium-verified)*. Avoid unanchored `:has()` on `body`/`html` with broad descendant arguments in large, frequently mutating DOMs (each mutation may require re-evaluation); anchor to a nearby container and prefer the child combinator.

##### Container queries

- **Size queries — Widely available (2025-08-14)**; units `cqi`, `cqb`, `cqw`, `cqh`, `cqmin`, `cqmax` come with them.
- **Style queries for custom properties — Newly available (2026-05-19; Firefox 151).**
- **Name-only queries (`@container sidebar { }`) — Newly (2026-05-07).**
- **Scroll-state queries — not Baseline (Chromium only).**

```css
.card-list { container: cards / inline-size; }

@container cards (inline-size >= 40rem) {
  .card { grid-template-columns: 12rem 1fr; }
}

.card__title { font-size: clamp(1rem, 4cqi, 1.5rem); }

/* Style query — gate or accept the fallback for pre-2026 browsers */
@container style(--variant: compact) {
  .card { padding: 0.5rem; }
}
```

Pitfalls *(Chromium-verified)*:

- **An element cannot query itself.** `.self { container-type: inline-size }` plus `@container (…) { .self { … } }` never applies to `.self` — only to descendants.
- **`container-type: size` applies size containment in both axes**: a container without an explicit height collapses to **0 px** high even with content. Use `inline-size` unless you set a block size.
- `container-type: inline-size` also establishes inline-size containment: the container's width can no longer depend on its children (a shrink-to-fit `inline-block` container with text content measured **0 px** wide).
- Container query units fall back to the small viewport units when no container exists (`width: 50cqw` without a container measured 500 px in a 1000 px viewport).

##### CSS nesting — Widely available (2026-06-11)

```css
.card {
  padding: 1rem;

  & .card__title { font-weight: 600; }
  &:hover { background: var(--surface-hover); }
  &.card--featured { border-color: var(--accent); }
  .theme-dark & { background: var(--surface-dark); }    /* parent after */

  @media (width >= 48rem) { padding: 2rem; }            /* nested conditional rules */

  h2 { margin: 0; }   /* type selectors may start a nested rule (relaxed syntax) */
}
```

Pitfalls (all *Chromium-verified* unless noted):

- **Native nesting is not Sass**: `&__title` (BEM suffix concatenation) **does not work** — the rule is dropped and `.card__title` is not styled. Write the full class (`.card__title { }`) at top level or `& .card__title`.
- `&` has `:is()` specificity (see Specificity).
- **Declarations after nested rules**: since `CSSNestedDeclarations` (Newly, 2024-12-11; Chrome 130, Firefox 132, Safari 18.2) they apply in source order. In `.a { & { color: X } color: Y }` current browsers apply Y; browsers before those versions hoisted Y above the nested rule and applied X. Keep declarations *before* nested rules to get identical results everywhere.
- The relaxed syntax (nested rules starting with a type selector) arrived with the versions that make nesting Baseline (Chrome 120, Safari 17.2); Chrome 112–119 and Safari 16.5–17.1 required `&` or another symbol first. For the Widely-available target this no longer matters.
- Deep nesting reproduces the specificity and coupling problems of preprocessor CSS; keep nesting to one or two levels.

##### `@scope` — Newly available (2026-03-24)

```css
/* Donut scope: style the card but not nested cards' content */
@scope (.card) to (.card__slot) {
  p { margin-block: 0.5em; }
  :scope { border: 1px solid var(--border); }
}
```

*(Chromium-verified: a `<p>` inside the `to` boundary is excluded.)* Browser-compat-data records the `@scope` rule in Chrome 118, Firefox 146 and Safari 26.4 (Safari exposed `CSSScopeRule` from 17.4); the whole feature, including `&` inside `@scope`, became Baseline on 2026-03-24. Scoping does **not** add specificity; proximity only breaks ties between otherwise equal declarations. For Widely-available targets keep BEM-style class scoping. `@supports at-rule(@scope)` cannot be used as a gate (`at-rule()` in `@supports` is itself not Baseline); a browser without `@scope` drops the whole block, so the unscoped base styles must be acceptable on their own.

##### Subgrid — Widely available (2026-03-15)

```css
.cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr)); gap: 1rem; }
.card  { display: grid; grid-row: span 3; grid-template-rows: subgrid; }  /* title/body/footer align across cards */
```

*(Chromium-verified: computed `grid-template-rows` is `subgrid`.)* The subgridded item must span the tracks it adopts (`grid-row: span 3`).

##### Logical properties — Widely available (2024-03-20)

```css
.notice {
  margin-inline-start: 1rem;     /* left in LTR, right in RTL */
  padding-block: 0.5rem 1rem;
  border-inline-start: 4px solid var(--accent);
  inset-inline-end: 0;
  inline-size: min(100%, 40rem);
}
```

*(Chromium-verified: in `dir="rtl"` `margin-inline-start: 10px` computes to `margin-right: 10px`.)* Prefer logical properties in any project that is or may become multilingual; mixing physical and logical properties for the same side creates order-dependent overrides.

##### Colour: `oklch()`, `color-mix()`, relative colours, `contrast-color()`

```css
:root {
  /* OKLCH: perceptually uniform lightness — Widely available (2025-11-09) */
  --brand: oklch(55% 0.18 260);
  --brand-hover: color-mix(in oklch, var(--brand), black 15%);  /* Widely (2025-11-09) */
  --brand-subtle: color-mix(in oklab, var(--brand) 12%, transparent);
}

/* Relative colour syntax — Newly available (2024-09-16; Firefox 128, Safari 18) */
.badge { background: oklch(from var(--brand) calc(l + 0.2) c h); }

/* contrast-color() — Newly available (2026-04-10): only black or white, and only
   guaranteed to pick the higher-contrast one, not a WCAG-passing one */
.chip { color: contrast-color(var(--chip-bg)); }
```

Facts *(Chromium-verified)*:

- `color-mix(in srgb, red, blue)` computes to `color(srgb 0.5 0 0.5)`; `color-mix(in oklab, …)` to an `oklab()` value. The current CSS Color 5 draft makes the `in <space>` part optional (default Oklab), but Chromium 141 rejects `color-mix(red, blue)` and the omission is not tracked as Baseline: always write the interpolation space.
- `oklch(70% 0.4 150)` is accepted and stays out of gamut in the computed value; the browser gamut-maps when painting. Keep chroma within the target gamut (sRGB chroma is usually ≤ 0.15–0.2 depending on hue) or provide a `@media (color-gamut: p3)` variant.
- Relative colour results compute to `color(srgb …)` for `rgb(from …)`.
- `contrast-color()` was not available in Chromium 141 (Chrome 147+); `color-mix()` with three colours parses in neither Chromium 141 nor any Baseline browser set.

##### `light-dark()` and `color-scheme` — see Colour Schemes below.

##### View transitions

- **Same-document view transitions — Newly available (2025-10-14; Firefox 144, Safari 18).**
- **Cross-document (MPA) view transitions — not Baseline (no Firefox).**

```js
// Progressive enhancement: always perform the update, animate when possible
function navigate(update) {
  if (!document.startViewTransition ||
      matchMedia('(prefers-reduced-motion: reduce)').matches) {
    update();
    return;
  }
  document.startViewTransition(update);
}
```

```css
.product-image { view-transition-name: hero; }          /* must be unique per page at transition time */
::view-transition-group(hero) { animation-duration: 250ms; }

/* MPA opt-in: ignored where unsupported */
@view-transition { navigation: auto; }
```

Review: duplicate `view-transition-name` values on rendered elements abort the transition — the update callback still runs, `transition.ready` rejects with `InvalidStateError`, and the console logs "Unexpected duplicate view-transition-name" *(Chromium-verified)*. Generated names in lists need unique suffixes or `view-transition-name: match-element` (Newly). Respect `prefers-reduced-motion`.

##### Anchor positioning — core Newly available (2026-01-13), whole feature Limited

```css
.trigger { anchor-name: --account; }
.menu {
  position: absolute;               /* or fixed / a popover in the top layer */
  position-anchor: --account;
  position-area: block-end span-inline-end;
  position-try-fallbacks: flip-block, flip-inline;
  margin-block-start: 0.25rem;
}

/* Fallback for browsers without anchor positioning */
@supports not (anchor-name: --a) {
  .menu { inset-block-start: 100%; inset-inline-start: 0; }   /* parent is position: relative */
}
```

*(Chromium-verified: `anchor-name`, `position-area`, `position-try-fallbacks`, `anchor()` parse; `position-visibility: anchor-visible` does not.)* Anchors must be laid out before the positioned element and be in the same containing-block chain (or the positioned element must be in the top layer). Chrome 125–128 used the older `inset-area` name for `position-area` (renamed in Chrome 129).

##### `@starting-style` and discrete transitions — Newly available (2024-08-06)

```css
/* Fade a dialog in (and out where display/overlay animation is supported) */
dialog {
  opacity: 1;
  transition:
    opacity 200ms ease-out,
    display 200ms allow-discrete,      /* Limited: no Firefox — exit animation skipped there */
    overlay 200ms allow-discrete;
}
dialog:not([open]) { opacity: 0; }
@starting-style { dialog[open] { opacity: 0; } }

@media (prefers-reduced-motion: reduce) {
  dialog { transition: none; }
}
```

*(Chromium-verified: two frames after `showModal()` the opacity was still well below 1 and reached 1 after the transition.)* Without `@starting-style`, an element that goes from `display: none` to visible has no "before" style and does not transition. Put `@starting-style` *after* the base rule (or give it equal-or-higher specificity) — it participates in the normal cascade.

##### `text-wrap`

```css
h1, h2, h3, figcaption { text-wrap: balance; }   /* Newly (2024-05-13); Widely on 2026-11-13 */
p { text-wrap: pretty; }                         /* NOT Baseline (no Firefox) — cosmetic only */
.nowrap { text-wrap: nowrap; }                   /* the wrap/nowrap values alone are Widely available (2026-09-19) */
```

`balance` is limited to short blocks (per MDN: six lines or fewer in Chromium, ten or fewer in Firefox); `pretty` costs layout time. These are pure enhancements; never flag their absence as a bug, and never treat `pretty` as Baseline.

##### Other recent additions worth recognising

| Feature | Status | Note |
|---|---|---|
| `sibling-index()`/`sibling-count()` | Newly (2026-08-18) | Staggered animations without inline styles |
| `round()`, `mod()`, `rem()` | Newly (2024-05-17) | Widely on 2026-11-17 |
| `shape()` | Newly (2026-02-24) | Responsive `clip-path` shapes |
| `progress()` | Newly (2026-09-01) | |
| `:open` | Newly (2026-05-11) | `[open]` works everywhere for `details`/`dialog` |
| `if()`, `@function` | Limited | Chromium only |
| `field-sizing` | Newly (2026-06-16) | See Forms |

***

#### Motion and `prefers-reduced-motion`

`prefers-reduced-motion` is Widely available (2022). WCAG 2.3.3 (motion from interactions) is AAA, but 2.2.2 Pause, Stop, Hide (A) requires a way to stop auto-playing motion lasting more than five seconds, and vestibular disorders make large parallax/zoom effects a real harm.

```css
/* Motion as an enhancement: no motion by default, add it when allowed */
.hero__image { opacity: 1; }
@media (prefers-reduced-motion: no-preference) {
  .hero__image { animation: fade-up 600ms ease-out both; }
}

/* Global safety net (keeps transitionend/animationend events firing) */
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
    scroll-behavior: auto !important;
  }
}
```

*(Chromium-verified with emulated `reducedMotion: 'reduce'`: the media query matches and `transitionDuration` computes to `1e-05s`.)*

- Use `0.01ms` rather than `0s`/`none` when JavaScript waits for `transitionend`/`animationend`.
- Reduced motion means *reduce*, not remove: opacity cross-fades are usually fine; large translations, zoom, parallax, auto-playing carousels and smooth scrolling are not.
- `scroll-behavior: smooth` on `html` should be inside `@media (prefers-reduced-motion: no-preference)`.
- Auto-playing video/GIF backgrounds need a visible pause control (2.2.2).
- In JavaScript-driven animation (Web Animations API, libraries, `startViewTransition`), check `matchMedia('(prefers-reduced-motion: reduce)')` and listen for `change`.

***

#### Colour Schemes and Dark Mode

```html
<!-- Tells the UA both schemes are supported: form controls, scrollbars and the
     canvas background follow the user's preference before CSS loads -->
<meta name="color-scheme" content="light dark">
```

```css
:root {
  color-scheme: light dark;   /* light-dark() needs this or the color-scheme <meta> to pick the dark value */

  --surface: light-dark(#ffffff, oklch(20% 0.01 260));
  --text:    light-dark(oklch(25% 0.02 260), oklch(92% 0.01 260));
  --accent:  light-dark(oklch(50% 0.2 260), oklch(75% 0.14 260));
}
body { background: var(--surface); color: var(--text); }

/* User-selected theme overrides the OS preference */
:root[data-theme="light"] { color-scheme: light; }
:root[data-theme="dark"]  { color-scheme: dark; }
```

Facts *(Chromium-verified)*:

- **`light-dark()` uses the element's used colour scheme, not the media query.** Without any colour-scheme opt-in it always returns the light value, even when the OS is dark.
- The opt-in can come from the `color-scheme` property **or** from `<meta name="color-scheme" content="light dark">`: elements whose computed `color-scheme` is `normal` follow the page colour scheme set by the meta element (CSS Color Adjustment 1). With only the meta element, the canvas turns dark, default text white, and `light-dark()` returns its dark value although the computed property stays `normal`. A `color-scheme: light` on an element (e.g. a `[data-theme="light"]` override) wins over the meta for its subtree.
- `light-dark()` is **Newly** available (2024-05-13; Chrome 123, Firefox 120, Safari 17.5) and becomes Widely available on 2026-11-13. For older browsers, the `@media (prefers-color-scheme: dark)` custom-property pattern (Widely) is the fallback:

```css
:root { --surface: #fff; --text: #1a1a1a; }
@media (prefers-color-scheme: dark) {
  :root { --surface: #121212; --text: #ececec; }
}
```

- `light-dark()` accepts only colours (image support is Newly available since 2026-09-14).
- Check contrast for **both** schemes (WCAG 1.4.3/1.4.11), including focus rings, borders and disabled states.
- Avoid pure `#000` backgrounds with pure `#fff` text for long-form text in dark mode (halation); this is guidance, not a WCAG requirement.
- **Forced colors (Windows High Contrast)**: `@media (forced-colors: active)` (Widely). The UA replaces colours with system colours and forces `box-shadow`, `text-shadow` and gradient `background-image` to `none`, while `url()` background images are kept *(Chromium-verified with emulated forced colors)*. Use system colour keywords (`CanvasText`, `ButtonText`, `Highlight`, `LinkText`) for custom indicators, keep real borders/outlines (transparent borders become visible), and use `forced-color-adjust: none` only on small swatches that must keep their colour.
- `prefers-contrast: more` (Widely) can strengthen borders and text colours.

***

#### Layout, Responsive Design and Typography

##### Mobile-first with modern media queries (house rule, updated)

```css
.grid { display: grid; gap: 1rem; grid-template-columns: 1fr; }

/* Range syntax — Widely available (2025-09-27) */
@media (width >= 48rem) { .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (width >= 64rem) { .grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }

/* Intrinsic alternative: no breakpoints */
.card-grid { display: grid; gap: 1.5rem; grid-template-columns: repeat(auto-fill, minmax(min(18rem, 100%), 1fr)); }
```

- Use `rem`/`em` breakpoints so they scale with the user's font size.
- `min(18rem, 100%)` inside `minmax()` prevents overflow on viewports narrower than 18 rem (WCAG 1.4.10 reflow at 320 px).
- Prefer container queries for components that appear in different-width slots; media queries for page layout.

##### Overflow traps *(all Chromium-verified)*

```css
/* 1. Flex items default to min-width: auto — long words/URLs/code overflow */
.row { display: flex; }
.row > .content { min-inline-size: 0; }        /* allow shrinking */

/* 2. Grid 1fr = minmax(auto, 1fr) — same problem */
.layout { grid-template-columns: 1fr; }               /* BAD: track grows to the text width (~450px) in a 100px grid */
.layout { grid-template-columns: minmax(0, 1fr); }    /* GOOD: stays 100px */

/* 3. Long unbreakable strings */
.prose { overflow-wrap: anywhere; }
```

In the measurements (font-dependent), a nowrap child inside a 100 px flex container produced a ~450 px scroll width; a 100 px `1fr` grid's track grew to ~450 px while `minmax(0, 1fr)` stayed at 100 px.

##### Containing-block and stacking surprises

- **`transform`, `filter`, `backdrop-filter`, `perspective`, `contain: paint`/`layout` and `will-change: transform` on an ancestor make it the containing block for `position: fixed` descendants.** *(Chromium-verified: a `position: fixed; left: 0` child of a transformed element at x = 58 px rendered at x = 58 px, not 0; the same with `backdrop-filter`. `container-type: inline-size` did not have this effect.)* Modals/toasts inside animated containers then scroll with the content. Render them in the top layer (`<dialog>`, `popover`) or at the document root.
- `z-index` only works within a stacking context; `opacity < 1`, `transform`, `filter`, `isolation: isolate`, `position: fixed/sticky` create new ones. Top-layer elements (`<dialog>` modal, popovers, fullscreen) are above every `z-index` — prefer them over `z-index: 99999`.
- `position: sticky` fails silently when an ancestor between it and the intended scroller has `overflow: hidden`/`auto`/`scroll` (the sticky element sticks within that scroller) or when no inset (`top`) is set.
- `overflow: clip` (Widely) clips like `hidden` but without creating a scroll container, so it does not capture sticky descendants or allow programmatic scrolling.

##### Viewport units

```css
/* 100vh is the LARGE viewport on mobile (toolbars hidden) → content hidden behind toolbars */
.hero { min-block-size: 100vh; }          /* BAD on mobile */
.hero { min-block-size: 100svh; }         /* GOOD: always fits the small viewport */
.app  { block-size: 100dvh; }             /* dynamic: changes as toolbars show/hide (relayouts) */
```

`svh`/`lvh`/`dvh` are Widely available (2025-06-05).

##### Sizing and units

- Do not set `html { font-size: 16px }` or `62.5%`-tricks with pixel assumptions; leave the root font size to the user (WCAG 1.4.4). Use `rem` for type and spacing, `ch` for measure (`max-inline-size: 65ch`), `lh`/`rlh` (Widely) for vertical rhythm.
- Fluid type: `clamp(1.25rem, 1rem + 1.5vw, 2rem)` — always include a `rem` term. A pure `vw` font size does not grow with browser zoom (zooming shrinks the viewport in CSS px, so the computed size shrinks with it) and ignores the user's font-size setting, failing 1.4.4.
- Use unitless `line-height` (inherits as a ratio).
- **No fixed heights on text containers** (`height: 40px` on a button with text breaks WCAG 1.4.12 text spacing and translation); use `min-block-size` and padding.
- `aspect-ratio` (Widely) instead of the padding-top hack.
- `box-sizing: border-box` globally (`*, *::before, *::after { box-sizing: border-box; }`).

##### Typography

- `hyphens: auto` requires a correct `lang`.
- Never justify text without hyphenation (rivers) — WCAG 1.4.8 (AAA) recommends against justified text.
- System font stack: `font-family: system-ui, sans-serif;` and `ui-monospace, monospace` for code.
- `text-wrap: balance` for headings is a harmless enhancement (see Modern CSS).

***

#### Naming and Architecture (house rules)

##### BEM

```css
.card {}                      /* Block */
.card__title {}               /* Element: double underscore */
.card--featured {}            /* Modifier: double hyphen */
.card__title--centered {}
```

```html
<article class="card card--featured">
  <header class="card__header"><h2 class="card__title">Title</h2></header>
  <div class="card__body"><p class="card__text">Content</p></div>
  <footer class="card__footer">
    <button type="button" class="card__button card__button--primary">Action</button>
  </footer>
</article>
```

- Do not chain elements (`.card__header__title__icon`); flatten to `.card__title-icon`.
- Style classes, not tag names inside blocks (`.card h2` couples CSS to markup and raises specificity).
- Modifiers extend the block; always keep the base class (`class="button button--primary"`).
- **Native CSS nesting cannot generate BEM names** (`&__title` does not work — see Nesting). Sass/Less still can; review which one the file is.
- State is better expressed through real attributes/pseudo-classes where they exist (`[aria-expanded="true"]`, `:disabled`, `[open]`, `:popover-open`, `:user-invalid`) than through parallel `is-open` classes that can drift from the actual state.

##### Design tokens with custom properties

```css
:root {
  /* Colour */
  --color-primary: oklch(52% 0.18 255);
  --color-primary-strong: color-mix(in oklch, var(--color-primary), black 20%);
  --color-danger: oklch(55% 0.2 27);

  /* Typography */
  --font-sans: system-ui, sans-serif;
  --font-mono: ui-monospace, monospace;
  --step-0: 1rem;
  --step-1: clamp(1.125rem, 1rem + 0.5vw, 1.375rem);

  /* Space */
  --space-xs: 0.25rem;
  --space-s: 0.5rem;
  --space-m: 1rem;
  --space-l: 1.5rem;

  /* Shape and elevation */
  --radius: 0.375rem;
  --shadow-1: 0 1px 2px oklch(0% 0 0 / 0.1);
}

.button {
  background: var(--color-primary);
  padding: var(--space-s) var(--space-m);
  border-radius: var(--radius);
}
```

Keep raw values in tokens and semantic aliases (`--surface`, `--text`, `--border`) that switch with the colour scheme; components consume only semantic aliases.

***

#### HTML and CSS in Templates and Components

- **JSX**: `className`, `htmlFor`, `tabIndex`, camel-cased ARIA is *not* used (`aria-label` stays kebab-case). Boolean attributes: `hidden={false}` omits the attribute; `aria-hidden={false}` renders `aria-hidden="false"` (usually pointless). `dangerouslySetInnerHTML` with unsanitised input is XSS.
- **Vue** `v-html`, **Svelte** `{@html}`, **Angular** `[innerHTML]` (sanitised, but `bypassSecurityTrustHtml` disables it), server templates with "raw"/"safe" filters (`|safe`, `{!! !!}`, `<%- %>`) — each one needs a sanitiser (DOMPurify or equivalent) and a reason.
- **Generated IDs**: repeated components must generate unique IDs (React `useId()`, Vue `useId()` in 3.5+, server-side counters) for `label for`, `aria-*` references, `popovertarget`, `commandfor`, `anchor-name` values.
- **Conditional rendering vs `hidden`**: removing an element from the DOM loses focus and live-region history; toggling `hidden` keeps references valid. For live regions, keep the container mounted.
- **Shadow DOM**: ID references (`aria-labelledby`, `for`, `aria-describedby`, `popovertarget`) do **not** cross shadow boundaries. Use `ElementInternals` (`attachInternals()`, form-associated custom elements — Widely) for roles/states, element-reference ARIA properties (`ariaLabelledByElements`, Newly 2025-04-04), or keep label and control in the same tree. `referenceTarget` is not Baseline.
- **Declarative shadow DOM** (`<template shadowrootmode="open">`) is Widely available (2026-08-20); `innerHTML` does not parse it — `setHTMLUnsafe()`/`Document.parseHTMLUnsafe()` do (Newly available, 2025-09-15). As the names say, they do not sanitise.
- **Scoped styles**: Vue `scoped` and Angular emulated encapsulation add a generated attribute selector to each selector (one extra attribute of specificity); CSS Modules rename classes instead. Overrides of child components need `:deep()` (Vue; `::v-deep` is the deprecated spelling), `:global()` (CSS Modules) or `::ng-deep` (Angular, deprecated) and should be rare.
- **Tailwind/utility CSS**: still check semantics and accessibility — utilities do not add focus styles back after `outline-none`; `sr-only` is the visually-hidden pattern; `motion-safe:`/`motion-reduce:` variants map to `prefers-reduced-motion`.
- **Inline `style` attributes generated from data** need CSP `style-src 'unsafe-inline'` (or `'unsafe-hashes'` plus hashes) and are an injection vector when values are not validated (`url(javascript:…)` no longer executes in modern browsers, but `url()` can exfiltrate data). Prefer CSS custom properties set via CSSOM (`el.style.setProperty('--x', value)`), which is allowed under CSP.

***

#### Tooling

- **Baseline linting**: `@eslint/css` (2.0.1) ships the `use-baseline` rule (default `available: "widely"`; also `"newly"` or a year) *(rule options read from the installed package)*; `stylelint-plugin-use-baseline` (1.4.8) does the same for Stylelint 16/17. `eslint-plugin-baseline-js` covers JavaScript APIs.
- **Browser targets**: `browserslist` supports `baseline widely available`, `baseline newly available` and `baseline 2024`-style queries *(executed with browserslist 4.29.4)*; Autoprefixer and Lightning CSS (`browserslistToTargets`) consume it, and Vite's `build.target` accepts `'baseline-widely-available'` *(string found in the Vite 8.3.5 build)*.
- **Accessibility**: axe-core (4.14.0) in unit/E2E tests (`@axe-core/playwright`), plus manual keyboard and screen-reader passes; automated rules cannot judge alt-text quality, focus order, or whether names make sense.
- **HTML validation**: the Nu HTML Checker (`vnu`) or `@html-eslint/eslint-plugin` catch invalid nesting, duplicate IDs, obsolete attributes.
- **Performance**: Lighthouse/PageSpeed Insights for LCP/CLS/INP lab data; field data (CrUX, RUM) decides.
- **CSS formatting**: Prettier or Stylelint's formatting conventions; do not hand-align values.

***

#### Production / Review Checklist

##### Document and semantics

- [ ] `<!doctype html>`, `<html lang>`, `<meta charset="utf-8">`, viewport without zoom restrictions, unique `<title>`
- [ ] Landmarks: one `<main>`, labelled multiple `<nav>`s, `<search>` instead of `role="search"` where Widely-available is the target
- [ ] Heading hierarchy without gaps; one `<h1>`
- [ ] Buttons for actions (`type="button"` outside submit), links with `href` for navigation; no nested interactive content
- [ ] No duplicate IDs; generated IDs in repeated components
- [ ] Data tables with `<caption>`, `<th scope>`; no layout tables

##### Accessibility (WCAG 2.2 AA)

- [ ] Every image has appropriate `alt` (empty for decorative); icon-only buttons have names
- [ ] No `aria-hidden`/`role="presentation"` on focusable elements; `inert` for hidden interactive regions
- [ ] No redundant or prohibited ARIA (`aria-label` on generic `div`/`span`)
- [ ] Visible label text contained in the accessible name (2.5.3)
- [ ] Visible focus indicator (`:focus-visible`), not removed by `outline: none`; not obscured by sticky UI (2.4.11)
- [ ] Targets ≥ 24 × 24 CSS px or sufficiently spaced (2.5.8)
- [ ] Drag-only interactions have a single-pointer alternative (2.5.7)
- [ ] Contrast 4.5:1 text / 3:1 large text and UI in both colour schemes
- [ ] Reflow at 320 px, 200 % zoom, text-spacing overrides do not clip text
- [ ] Live regions mounted before updates; status messages announced (4.1.3)
- [ ] Focus moved sensibly after route changes, deletions, dialog close

##### Forms

- [ ] Every control labelled; groups in `<fieldset>`/`<legend>`
- [ ] Correct `autocomplete` tokens; `current-password`/`new-password`/`one-time-code`; paste allowed (3.3.8)
- [ ] `type="number"` only for quantities; `inputmode` for numeric text
- [ ] `pattern` valid under the `v` flag (escape `-` in classes)
- [ ] Errors associated via `aria-describedby`, `aria-invalid` after validation; `:user-invalid` not `:invalid` for styling
- [ ] Server-side validation exists

##### Dialogs and overlays

- [ ] Modals use `<dialog>.showModal()` (not `open` attribute, not div + focus trap)
- [ ] `[popover]`/`dialog` styles use `:popover-open`/`[open]` when setting `display`
- [ ] Popover / invoker commands / `requestClose()` only with fallback when targeting pre-2025/2026 browsers
- [ ] `closedby`, `popover="hint"`, `interestfor` not relied on

##### Security

- [ ] No inline event handlers or `javascript:` URLs; CSP-compatible markup; nonces per response
- [ ] Untrusted URLs scheme-allow-listed; untrusted HTML sanitised
- [ ] `window.open(url, '_blank', 'noopener')` for untrusted URLs
- [ ] Iframes: minimal `sandbox`, never `allow-scripts allow-same-origin` for same-origin untrusted content; `title`; minimal `allow`
- [ ] Third-party scripts pinned with `integrity` + `crossorigin`
- [ ] Clickjacking protection via HTTP header, not `<meta>` CSP
- [ ] JSON in `<script>` escapes `<`

##### Performance

- [ ] LCP image not lazy, `fetchpriority="high"`, discoverable in HTML
- [ ] `width`/`height` on images, iframes, video; reserved space for embeds/ads
- [ ] `srcset` with accurate `sizes`; modern formats with fallback when needed
- [ ] Font preloads have `crossorigin`; `font-display` set
- [ ] Scripts `defer`/`async`/`type="module"`; no `@import` chains
- [ ] Animations on `transform`/`opacity`; `will-change` scoped

##### CSS

- [ ] Layer strategy consistent (no stray unlayered overrides; no `!important` in low layers)
- [ ] No ID selectors in component CSS; no specificity hacks (`.a.a.a`)
- [ ] No `&__suffix` in native CSS nesting; declarations before nested rules
- [ ] `var()` fallbacks not relied on for invalid values; `@property` for typed tokens
- [ ] Container queries do not query the container itself; `inline-size` unless height is fixed
- [ ] `light-dark()` accompanied by a colour-scheme opt-in (`color-scheme: light dark` or `<meta name="color-scheme">`)
- [ ] `prefers-reduced-motion` honoured for animations, smooth scroll, view transitions
- [ ] Logical properties in multilingual UIs; `min-inline-size: 0`/`minmax(0, 1fr)` where content can overflow
- [ ] Every Newly-available feature has an acceptable fallback for the project's target; no Limited-availability feature carries essential function
