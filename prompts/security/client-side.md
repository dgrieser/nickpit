### Client-side and browser security guide
Applies to: SPAs, extensions, webviews, service workers, storage, `postMessage`, WebSockets, DOM.

#### Hunt for
- **DOM XSS**: `location`, `document.referrer`, `window.name`, messages, storage to `innerHTML`, `outerHTML`, `document.write`, eval-like APIs, executable URLs, jQuery HTML, framework escape hatches.
- **DOM clobbering**: sanitized-through `id`/`name` shadowing trusted global, form property, config, flag.
- **Prototype pollution**: `__proto__`, `constructor` keys into deep merge or path set, plus gadget using it for authorization, execution, navigation, rendering.
- **`postMessage`**: no exact origin allowlist (`event.source` for same-origin frames); substring, prefix, suffix, unanchored-regex match; secrets sent to `*`.
- **WebSocket**: upgrade trusting ambient cookies without `Origin` check or channel token, plus handler reading or mutating victim data.
- **Service workers**: attacker-set script URL, broad `Service-Worker-Allowed`, imports without integrity; personalized cache unkeyed by account, tenant, auth; served after logout, account switch, offline/error fallback.
- **Storage**: tokens, private data, cached authorization in web or extension storage readable by lower-trust same-origin code, other account, or valid after logout.
- **Cross-tab**: `storage` events, `BroadcastChannel`, shared workers carrying identity or commands unbound to session; stale tab overwriting newer auth.
- **XS-Leaks**: credentialed cross-origin state visible via load/error events, frame count, redirects, cache, timing, size.
- **Opener**: retained `window.opener` or named windows steering privileged navigation.
- **Clickjacking**: framable mutations without `frame-ancestors`/`X-Frame-Options`; action must complete when framed; read-only pages hardening.
- **Navigation**: links or redirects allowing `javascript:`, `data:`.

#### Validate before reporting
- Self-injection, own-data disclosure: not findings.
- Check framework escaping, same-origin policy, CSP, COOP/CORP, SameSite, worker scope, implicit `noopener`.
- Prototype pollution: recursive write plus security gadget; `JSON.parse` or shallow copy alone insufficient. Clobbering: surviving markup plus trusted use.
- Storage/workers: who writes, reads, clears each record; must reach other account, tenant, later authorization. Stored token alone insufficient.
- Browsers reject wildcard CORS with credentials; need reflected or allowed origin.
- XS-Leaks: one concrete secret-bearing predicate (private object, role, account exists); generic timing variance, public-resource availability not findings.
