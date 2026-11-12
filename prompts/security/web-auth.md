### Web, HTTP, and authentication security guide
Applies to: HTTP, proxies, caches, sessions, tokens, SSO, MFA, API keys, mTLS.

#### Hunt for
- **Desync**: proxy/backend split on duplicate `Content-Length`, `Transfer-Encoding`, HTTP/2 downgrade, header normalization, CR/LF.
- **Cache**: unkeyed forwarded host, cookies, query normalization, auth state; private paths cached as static via suffix.
- **Forwarded**: `Host`, `Forwarded`, `X-Forwarded-*` trusted for URLs, reset links, tenant routing, IP checks without ingress stripping.
- **Header injection**: CR/LF into `Location`, `Set-Cookie`, CSP.
- **CSRF**: cookie-authenticated mutation (any body type, method-override, login) lacking token, strict Origin/Referer, SameSite.
- **Sessions**: ID kept across login, MFA, account switch, impersonation; sessions, refresh tokens, websockets surviving logout, password change.
- **Cookies**: broad `Domain`/`Path`, insecure transport, same-name ambiguity.
- **JWT**: unpinned algorithm/key; unchecked `exp`, `nbf`, `aud`, `iss`; trusted `kid`, `jku`, `x5u`; decode without verify; foreign tokens accepted.
- **OAuth/OIDC**: inexact `redirect_uri`; unbound `state`; no PKCE; ID-token issuer, audience, signature, nonce, IdP unchecked on callback, retry, deep link.
- **SAML**: signed element not identity element; unsigned fallback; unsafe XML parser; validity, audience, recipient, replay.
- **MFA**: factor changes, recovery codes, trusted devices without fresh authentication; disabled factors still accepted; challenge unbound to principal, session, operation, expiry, single use; alternate route skipping step-up.
- **WebAuthn**: unchecked challenge, RP ID, origin, credential-to-user, userHandle, user verification.
- **Recovery**: linking lacks current session, new-identity proof, initiator-bound state; reset tokens weak, unbound, non-expiring, reusable, via untrusted URL, older tokens, sessions kept.
- **API keys**: prefix-only check, publishable/secret confusion, parameters overriding scope, stale revocation cache; secret keys in bundles, URLs, logs (publishable identifier not secret).
- **mTLS**: identity headers from any peer; attacker subject mapped to account, wrong trust domain, fail-open to bearer/anonymous.

#### Validate before reporting
- Desync/cache: name both components, divergent value.
- Credentials: cite verification line, missing binding, accepted invalid credential or transition.
- CSRF: ambient credential, mutating route, accepted cross-site shape; not read-only or bearer-only routes.
- Missing header, cookie flag, MFA prompt, rate limit alone: hardening; need accepted invalid request, cross-principal impact, assurance downgrade, or credential disclosure. Check framework defaults.
- OAuth role: redirect allowlist, code issuance belong to authorization server, not relying-party client.
