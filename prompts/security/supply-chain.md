### Supply chain and release security guide
Applies to: manifests, lockfiles, builds, Dockerfiles, CI, release, signing, updaters, plugins.

#### Hunt for
- **Dependency confusion**: resolver config, registry/mirror order, names, source URLs letting public or fallback source serve private name; lockfile, checksums.
- **Mutable inputs**: branches, floating actions, unpinned image tags, unverified submodules, remote includes, downloaded tools reaching trusted output.
- **Generated/vendored**: generated clients, archives, binary blobs skipping review and integrity; who controls generator input.
- **Build context**: context or ignore rules shipping secrets, local config, repo metadata, developer artifacts; shipped artifact must expose real credential, private data, privileged config.
- **Privileged workflows**: fork pull request, `pull_request_target`, issue comment, external event running contributor ref with secrets, write tokens, deploy authority, trusted runner; approval gates, permission narrowing.
- **Expression injection**: branch names, commit messages, PR/issue titles and bodies, artifact names, matrix values interpolated into shell, paths, privileged inputs instead of validated env.
- **Cache/artifact mixing**: lower-trust job writing cache, artifact, workspace, output higher-trust job executes or releases; digest binding, producer identity.
- **Automation identity**: CI token, cloud credential broader than operation, repository, environment, duration; untrusted job input selecting affected resource. Excess scope alone hardening; need reachable privileged action.
- **Build-to-release substitution**: test, sign, publish by mutable tag, filename, channel instead of one digest across copy, repack, multi-arch merge.
- **Signing**: valid signature accepted without attestation identity (workflow, repository, branch, environment, key role, threshold); expiry, revocation failing open.
- **Updaters**: payload bytes signed but version, platform, channel, target path, expiry unauthenticated; rollback; metadata and payload from different releases; signature call without policy binding; non-atomic install.
- **Plugins**: exceeding declared scope; publisher identity takeover; install hooks before authenticity check.

#### Validate before reporting
- Mutable or vulnerable dependency alone not a finding; need who steers resolution, consuming build, later execution or release.
- Artifact identity must break across handoff: mutable name or unbound digest reaches real consumer.
- Checksum from same untrusted location as artifact is not integrity; name trusted root, failure behavior.
- Installing arbitrary same-user plugins by design is not a boundary.
