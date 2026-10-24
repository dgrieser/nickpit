### Citations: Rust Style Guide

Sources for `rust.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Rust release notes (RELEASES.md), versions 1.0–1.99.1. https://github.com/rust-lang/rust/blob/master/RELEASES.md and https://doc.rust-lang.org/stable/releases.html
    - Rust Style Guide — quote block: "…in 1.85.0, February 20, 2025); no later edition is stable."
    - Release and Support Status (October 2026) — bullet 1: "…documented soundness fixes and bug fixes listed under "Compatibility Notes"."
    - Release and Support Status (October 2026) — paragraph 1: "…miscompilation, 1.96.1 a MIR-optimisation miscompilation plus libssh2 CVEs in Cargo."
    - Review-Relevant Stabilisations 1.56 – 1.99 — paragraph 1: "Release dates and items come from the official release notes."
[^2]: Announcing Rust 1.99.0 (October 1, 2026). https://blog.rust-lang.org/2026/10/01/Rust-1.99.0/
    - Rust Style Guide — quote block: "…in 1.85.0, February 20, 2025); no later edition is stable."
[^3]: Announcing Rust 1.99.1 (October 15, 2026). https://blog.rust-lang.org/2026/10/15/Rust-1.99.1/
    - Rust Style Guide — quote block: "…in 1.85.0, February 20, 2025); no later edition is stable."
    - Release and Support Status (October 2026) — paragraph 1: "…miscompilation, 1.96.1 a MIR-optimisation miscompilation plus libssh2 CVEs in Cargo."
[^4]: The Rust Edition Guide (Rust 2018, 2021, 2024 chapters). https://doc.rust-lang.org/edition-guide/
    - Editions — paragraph 1: "…the next edition, after which `edition` is bumped by hand."
    - Traits, Generics and Trait Objects › Return-position `impl Trait` and lifetime capture — paragraph 1: "`+ use<..>` (1.82) states the captures explicitly in every edition."
[^5]: rustfmt configuration (`style_edition`) and the Rust Style Guide. https://rust-lang.github.io/rustfmt/ , https://doc.rust-lang.org/style-guide/
    - Formatting and Naming — bullet 1: "…name); without it the style edition follows the crate edition."
[^6]: thiserror documentation (2.0). https://docs.rs/thiserror ; anyhow documentation. https://docs.rs/anyhow
    - Error Handling › `thiserror` (libraries) and `anyhow` (applications) — paragraph 1: "`#[error(transparent)]` forwards `Display` and `source` to the inner error."
[^7]: The Rust Reference — Behavior considered undefined. https://doc.rust-lang.org/reference/behavior-considered-undefined.html ; The Rustonomicon. https://doc.rust-lang.org/nomicon/
    - Unsafe Code and Soundness › Undefined behaviour categories (Rust Reference) — section heading
[^8]: Tokio README — supported Rust versions and LTS releases. https://github.com/tokio-rs/tokio#supported-rust-versions
    - Async Rust and Tokio › Model — bullet 2: "…and 1.53.x until September 2027) drives futures on worker threads."
[^9]: Tokio documentation — `tokio::select!` cancellation safety. https://docs.rs/tokio/latest/tokio/macro.select.html#cancellation-safety
    - Async Rust and Tokio › Cancellation safety — bullet 1: "…fn` that awaits more than once while holding partial state."
[^10]: The Cargo Book — SemVer Compatibility. https://doc.rust-lang.org/cargo/reference/semver.html
    - Public API Design and Semver — paragraph 1: "The Cargo book's SemVer chapter lists what is breaking."
[^11]: cargo-semver-checks. https://github.com/obi1kenobi/cargo-semver-checks
    - Public API Design and Semver — bullet 1: "…lost auto traits and enum variants added to exhaustive enums."
[^12]: The Cargo Book — FAQ: Why have Cargo.lock in version control? https://doc.rust-lang.org/cargo/faq.html#why-have-cargolock-in-version-control ; Resolver versions. https://doc.rust-lang.org/cargo/reference/resolver.html#resolver-versions
    - Cargo: Manifests, Features, Workspaces, Lockfiles, Build Scripts › Lockfile policy — bullet 1: "…(`cargo update` then test) and one that tests minimal versions."
[^13]: Supply chain attack on arrayref (August 20, 2026). https://blog.rust-lang.org/2026/08/20/supply-chain-attack-on-arrayref/
    - Cargo: Manifests, Features, Workspaces, Lockfiles, Build Scripts › Build scripts and proc macros — bullet 1: "…arrayref incident of August 2026 used a malicious build script)."
    - Security and Supply Chain › Dependencies — bullet 3: "…downloaded a payload; the versions were online for 86–107 minutes."
[^14]: GitHub Actions leaking secrets when Miri output is cached (September 21, 2026). https://blog.rust-lang.org/2026/09/21/github-actions-leaking-secrets-when-miri-output-is-cached/
    - Testing › Miri and loom — bullet 1: "…that run Miri and cache `target/` (fixed in nightly 2026-09-22)."
    - Security and Supply Chain › Dependencies — bullet 5: "…not have secrets in the environment (see the Miri incident)."
[^15]: Clippy README — lint categories. https://github.com/rust-lang/rust-clippy#clippy ; Clippy lint list. https://rust-lang.github.io/rust-clippy/stable/index.html
    - Lints: rustc, Clippy, rustfmt › Clippy — paragraph 1: "`clippy::all` covers correctness, suspicious, style, complexity and perf."
[^16]: RustSec advisory database and cargo-audit. https://rustsec.org/ ; cargo-deny. https://embarkstudios.github.io/cargo-deny/
    - Security and Supply Chain › Dependencies — bullet 1: "…and on a schedule; new advisories appear for unchanged lockfiles."
[^17]: crates.io development updates: Trusted Publishing (July 2025) and GitLab support/enforcement (January 2026). https://blog.rust-lang.org/2025/07/11/crates-io-development-update-2025-07/ , https://blog.rust-lang.org/2026/01/21/crates-io-development-update/
    - Security and Supply Chain › Dependencies — bullet 4: "…2026) instead of long-lived API tokens; owners can enforce trusted-publishing-only."
[^18]: Cargo changelog (Cargo 1.100, 2026-11-12; lockfile v4 default in Cargo 1.83). https://doc.rust-lang.org/nightly/cargo/CHANGELOG.html ; beta channel manifest (rustc 1.100.0-beta.7). https://static.rust-lang.org/dist/channel-rust-beta.toml
    - Rust Style Guide — quote block: "…in 1.85.0, February 20, 2025); no later edition is stable."
