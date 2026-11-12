### General security guide
Applies to: every change handling input, identity, files, secrets, state.

#### Hunt for
- **Injection**: untrusted input to SQL, shell, template, path, redirect, deserialization, format-string, LDAP/XPath, log sinks; include field names, keys, headers, metadata; second-order stored data (slug to path, field to JSON path, string to URL/regex/template/policy).
- **Access control**: wrong permission or resource checked; sibling path to same state change with weaker check; authentication without authorization; body fields overriding scope; bulk/batch/export/import skipping per-item checks.
- **Files and resources**: path traversal via symlinks, encoding, null bytes; SSRF via redirects, DNS rebinding, URL parser differentials; zip slip; unsafe temp files; TOCTOU.
- **Cryptography and secrets**: weak randomness for tokens, keys, nonces; hardcoded secrets; secrets in logs, errors, URLs, responses; missing HMAC check; nonce reuse; non-constant-time comparison; ECB, static IV, unauthenticated encryption; error paths falling back to no crypto.
- **Business logic**: skipped, reversed, replayed steps; partial failure without rollback; non-atomic check-then-act (double spend, lost update); negative, zero, overflowing, coerced quantities; expiry and clock-boundary errors; insecure defaults when config, flag, or dependency missing.
- **Implicit trust**: stored, config, plugin, or peer data treated as validated though another path writes it; compare what A guarantees (normalization, truncation, type coercion, units, tenant scope) with what B assumes.
- **Feature abuse and leakage**: imports overwriting or skipping validation; search/filter/sort as oracle on hidden fields; distinguishable not-found vs forbidden; broad preview tokens; user-set webhook/callback URLs fetched without internal-network check, including after redirects.
- **Scope growth**: token, API-key, plugin, OAuth, agent scope growing after delegation, refresh, caching, role change; name operation resulting principal should not have.
- **Wildcard**: legacy, half-finished, compatibility code; API calls UI never makes; hidden parameters, headers, routes; untested feature combinations; irreversible or confirmation-gated actions where approval unbound to final principal, action, resource; clock, DNS, case-sensitivity assumptions; unverified safety comments.
- **Obvious exposures**: hardcoded credentials (`password`, `secret`, `token`, `Bearer`, `-----BEGIN`); security TODO/FIXME/HACK; debug mode switchable via env, query, header; seed/test credentials in production; unprotected debug/admin/metrics/env/config routes; committed `.env` or keys; dynamic input to `eval`, `exec`, `Function`, process spawn; credentialed CORS reflecting origin; open redirect via `redirect`, `next`, `url`, `return`; stack traces or SQL errors in responses; unpinned or vulnerable dependencies.

#### Validate before reporting
- Cover sibling, legacy, batch, retry, error, cancellation paths to same effect.
- Sad paths only where interface accepts them: absent, empty, zero, negative, max, duplicate, mixed-encoding, stale, revoked, concurrent, partially migrated.
- Missing cookie flag: show cookie carries security-sensitive data and JS needs no access by design.
- Multi-step chain: establish each prerequisite output; never assume later step or boundary.
- One finding per root cause; establish each variant's conditions independently.
