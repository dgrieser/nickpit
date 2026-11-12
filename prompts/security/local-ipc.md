### Desktop, mobile, and local IPC security guide
Applies to: desktop/mobile apps, privileged helpers, updaters, local daemons, webviews, native messaging.

#### Hunt for
- **Deep links**: foreign link mutates state, completes login, picks account without one-time session binding; normalization, duplicate params, scheme/host/path match.
- **Handoff**: OAuth, SSO, magic-link, pairing, payment callbacks unbound to initiating app, session, account, operation, expiry.
- **File-open/intents**: file associations, share intents, drag/drop, clipboard trigger privileged action; unchecked type, user intent, final target.
- **Bridge origin**: remote frames reach packaged-content bridge; check origin per call and after navigation, redirect, subframe, popup, error page, not by URL prefix.
- **Broad bridge**: web content picks arbitrary files, commands, methods, credentials; no allowlist, normalization, confirmation; return values disclose data.
- **Webview access**: file/universal access, debug interface, custom schemes expose local files, internal origins.
- **Peer authentication**: sockets, pipes, XPC, Binder, D-Bus, loopback: verify OS credentials, code signature, sandbox token, not path, process name, bundle ID; request fields must not override peer identity.
- **Exported components**: services, activities, receivers, providers exposed via intent filters, permission level, path grants, aliases; check final merged manifest.
- **IPC lifecycle**: predictable request IDs, reused handles, inherited descriptors, world-writable or stale sockets let one peer answer, cancel, reuse another's operation.
- **Confused-deputy helper**: sudo, polkit, UAC, XPC helper lets caller pick command, file, user, setting; no per-operation authorization on normalized arguments.
- **Installer**: reads lower-trust-writable manifests, scripts, symlinks after authorization; need descriptor-relative opens, final ownership check.
- **Local secrets**: tokens in keychain, files, backups, clipboard exposed to lower-trust app or user.
- **Account switch**: caches, webview storage, biometric approvals survive logout; deferred action differs from what was shown, runs after expiry, or uses another profile's pending state.

#### Validate before reporting
- Attacker: other app, OS user, sandboxed child, untrusted document, remote origin; same account and authority not boundary violation.
- Cite origin/peer check and privileged sink; follow helper argument to final normalized target.
- Visible sandbox, signing, entitlements, keychain ACLs, consent prompts are controls; plaintext readable only by same OS account not vulnerability.
