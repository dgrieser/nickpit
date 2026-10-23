### Rust Style Guide

> **Version**: This is the **edition- and version-independent** Rust guide. It applies when a Rust change is reviewed but the project's edition or compiler version cannot be determined. It covers Rust 1.31 (the first release with edition 2018) through Rust 1.99 and the editions 2015, 2018, 2021 and 2024. The newest stable release is **Rust 1.99.0 (October 1, 2026)**; its latest patch is **1.99.1 (October 15, 2026)**. Rust **1.100.0 is in beta** and is scheduled for November 12, 2026. The newest stable edition is **Rust 2024** (stabilised in 1.85.0, February 20, 2025); no later edition is stable.[^1][^2][^3]
> **Scope**: Version-independent review rules for production Rust: ownership, borrowing and lifetimes, error handling, panics, `unsafe` and soundness, FFI, threads and atomics, async (Tokio), traits and generics, iterators, collections, strings and paths, integer arithmetic and casts, allocation and performance, public API design and semver, Cargo (features, resolver, workspaces, lockfiles, build scripts), testing (unit, integration, doc tests, property tests, Miri, loom), Clippy, rustfmt, and supply-chain security.
> **Version rule**: Every feature tied to an edition or a compiler release is labelled with it, for example "`let`-`else` (1.65)", "let chains (1.88, edition 2024 only)", "`LazyLock` (1.80)", "C-string literals `c"..."` (1.77, edition 2021+)". Before suggesting such a feature, check `edition` and `rust-version` in `Cargo.toml` (and the workspace root) and the channel in `rust-toolchain.toml` (see [Determine the Version First](#determine-the-version-first)). Never recommend a feature the project's edition or minimum supported Rust version (MSRV) does not have.
> **Note**: This document is self-contained. Every example was compiled (and, where it has a `main`, run) with **rustc 1.99.1** and checked with **Clippy 0.1.99** on x86_64 Linux, edition 2024 unless the block says otherwise. Edition boundaries were checked with `--edition` flags on 1.99.1 and with the toolchains that introduced each edition (1.31.0, 1.56.0, 1.85.0); release boundaries come from the official release notes and were bracketed by compiling probes on 1.56.0, 1.85.0 and 1.99.1. Blocks marked `compile_fail` are expected to be rejected; blocks marked `ignore` need external crates (`tokio` 1.54, `thiserror` 2.0, `anyhow` 1.0, `proptest` 1.12) and were built in a Cargo project.

---

#### Table of Contents

1. [Determine the Version First](#determine-the-version-first)
2. [Release and Support Status (October 2026)](#release-and-support-status-october-2026)
3. [Editions](#editions)
4. [Review-Relevant Stabilisations 1.56 – 1.99](#review-relevant-stabilisations-156--199)
5. [Do Not Use or Recommend Without a Version Check](#do-not-use-or-recommend-without-a-version-check)
6. [Deprecated, Removed and Changed Behaviour](#deprecated-removed-and-changed-behaviour)
7. [Formatting and Naming](#formatting-and-naming)
8. [Ownership, Borrowing and Lifetimes](#ownership-borrowing-and-lifetimes)
9. [Error Handling](#error-handling)
10. [Panics](#panics)
11. [Traits, Generics and Trait Objects](#traits-generics-and-trait-objects)
12. [Iterators and Closures](#iterators-and-closures)
13. [Collections](#collections)
14. [Strings, UTF-8, `OsStr` and `Path`](#strings-utf-8-osstr-and-path)
15. [Integers, Overflow, Casts and Floats](#integers-overflow-casts-and-floats)
16. [Unsafe Code and Soundness](#unsafe-code-and-soundness)
17. [FFI](#ffi)
18. [Concurrency: Threads, Locks and Atomics](#concurrency-threads-locks-and-atomics)
19. [Async Rust and Tokio](#async-rust-and-tokio)
20. [Public API Design and Semver](#public-api-design-and-semver)
21. [Cargo: Manifests, Features, Workspaces, Lockfiles, Build Scripts](#cargo-manifests-features-workspaces-lockfiles-build-scripts)
22. [Performance and Allocation](#performance-and-allocation)
23. [Testing](#testing)
24. [Lints: rustc, Clippy, rustfmt](#lints-rustc-clippy-rustfmt)
25. [Security and Supply Chain](#security-and-supply-chain)
26. [Logging and Observability](#logging-and-observability)
27. [Edition Migration Notes](#edition-migration-notes)
28. [Production / Review Checklist](#production--review-checklist)

---

#### Determine the Version First

Two independent numbers decide what code is valid: the **edition** (a per-crate language mode) and the **compiler release** (which APIs and syntax exist). A crate on edition 2021 built with rustc 1.99 has every 1.99 library API, but none of the edition-2024 language changes; a crate on edition 2024 needs rustc ≥ 1.85.

##### Where the version comes from

| Source | What it tells you |
|---|---|
| `[package] edition = "2021"` in `Cargo.toml` | Language edition of that package. **If the key is missing, Cargo uses edition 2015** and warns. |
| `edition.workspace = true` + `[workspace.package] edition = "…"` in the root `Cargo.toml` | Inherited edition (workspace inheritance, Cargo 1.64). |
| `[package] rust-version = "1.75"` | The MSRV the crate declares (Cargo 1.56). Cargo refuses to build the crate with an older toolchain; the MSRV-aware resolver (1.84) prefers dependency versions compatible with it. |
| `rust-toolchain.toml` (`[toolchain] channel = "1.88.0"`) or legacy `rust-toolchain` | The toolchain rustup selects in that directory. Pins the compiler used in development and CI, not the MSRV. |
| CI files (`dtolnay/rust-toolchain@1.80`, `rustup toolchain install …`), `Dockerfile` (`FROM rust:1.85`) | The compiler actually used to build and test. |
| `Cargo.lock` `version = 4` | Lockfile v4 is the default since Cargo 1.78; versions before 1.78 cannot read it. |
| `rustfmt.toml` `style_edition = "2024"` | Formatting style edition (rustfmt in 1.85+); independent of the crate edition. |

```toml
# Cargo.toml — the three keys to read first
[package]
name = "billing"
version = "0.4.2"
edition = "2021"          # language edition
rust-version = "1.75"     # MSRV: async fn in traits is available, LazyLock (1.80) is not
```

```toml
# rust-toolchain.toml
[toolchain]
channel = "1.88.0"
components = ["clippy", "rustfmt"]
```

##### Rules for version-aware review

- Read `edition`, `rust-version` and `rust-toolchain.toml` before claiming that an API or syntax is unavailable. If only `rust-toolchain.toml` exists, its channel is the effective minimum. If nothing pins the version, assume the latest stable (1.99) for APIs and edition 2015 for a package without `edition` (which itself deserves a finding).
- The MSRV is the binding constraint for libraries: code must compile on `rust-version`, so a new API used in a library requires raising `rust-version` in the same change. For applications pinned by `rust-toolchain.toml`, the pinned channel decides.
- Say "not available in X" only when the pinned edition/version proves it. Check the release tables below or the item's documentation (the "since" marker next to every stable std item).
- An edition is per crate. Workspace members can be on different editions; macros from a dependency expand with the edition of the crate that defined them.
- Do not "modernise" code in an unrelated change: replacing `once_cell::sync::Lazy` with `std::sync::LazyLock` raises the MSRV to 1.80 and is a breaking change for libraries with a lower declared MSRV.
- Clippy respects the MSRV: it reads `rust-version` (or `msrv` in `clippy.toml`) and does not suggest APIs that are newer. For example, `manual_is_multiple_of` suggests `n.is_multiple_of(2)` (1.87) for `n % 2 != 0` only when no older `rust-version` is declared. A Clippy suggestion that ignores the MSRV usually means `rust-version` is missing.

***

#### Release and Support Status (October 2026)

- Rust ships a new stable minor release **every six weeks** from `master` via the nightly → beta → stable train. Each release is backwards compatible with earlier releases of the same edition, apart from documented soundness fixes and bug fixes listed under "Compatibility Notes".[^1]
- **Only the latest stable release is supported.** There is no LTS: fixes, including security fixes, ship in the next stable or in a point release of the current one (for example 1.99.1). Projects stay current by moving to each new stable; `rust-version` documents how far back they still compile.
- Editions are not support channels: every current compiler supports all editions (2015, 2018, 2021, 2024), and crates on different editions link together.

| Channel | Version | Date |
|---|---|---|
| Stable | **1.99.1** | 2026-10-15 (1.99.0 on 2026-10-01) |
| Beta | 1.100.0 | stable release scheduled for 2026-11-12 |
| Previous stable | 1.98.1 | 2026-09-03 (1.98.0 on 2026-08-20) |

Recent point releases fix miscompilations: 1.99.1 fixes an x86_64 miscompilation of FFI `bool` return values (upper bits of the return register were left unspecified), 1.98.1 a vtable miscompilation, 1.97.1 an LLVM miscompilation, 1.96.1 a MIR-optimisation miscompilation plus libssh2 CVEs in Cargo.[^1][^3] CI pinned to an `x.y.0` release should move to the latest patch.

***

#### Editions

An edition changes how the compiler parses and interprets a crate; it never changes the standard library's API set. `cargo fix --edition` migrates code to the next edition, after which `edition` is bumped by hand.[^4]

| Edition | Stable since | Key changes a reviewer must know |
|---|---|---|
| **2015** | 1.0 (May 15, 2015) | Default when `edition` is missing. `extern crate` required; paths in `use` are relative to the crate root; `dyn` is optional for trait objects; `async`/`await`/`try` are identifiers; anonymous trait parameters (`fn f(u8);`) allowed. Non-lexical lifetimes (NLL) also apply to 2015 since 1.36. |
| **2018** | **1.31.0** (Dec 6, 2018) | New path system (`crate::`, no `extern crate` for normal dependencies, `use` paths start with a crate name or `crate`/`self`/`super`); `dyn Trait` (bare trait objects warn; error from 2021); `async`, `await`, `dyn`, `try` reserved keywords; anonymous trait parameters rejected; NLL. |
| **2021** | **1.56.0** (Oct 21, 2021) | Cargo feature resolver `"2"` by default; **disjoint closure captures** (closures capture fields, not whole variables, which changes drop order and `Send`/`Sync`); `array.into_iter()` yields values; prelude adds `TryFrom`, `TryInto`, `FromIterator`; `panic!("{}")` always formats (single-argument `panic!(s)` with a non-literal is an error); reserved prefixes (`k#ident`, `f"…"`, `c"…"`) so C-string literals (1.77) and raw lifetimes (`'r#a`, 1.83) work only from 2021; `$x:pat` matches `A | B`; `bare_trait_objects` and `ellipsis_inclusive_range_patterns` (`...` in patterns) are hard errors. |
| **2024** | **1.85.0** (Feb 20, 2025) | `impl Trait` in return position captures all in-scope lifetimes (opt out with `use<..>`); `if let` scrutinee temporaries drop before `else`; block tail-expression temporaries drop before locals; let chains (`if let … && …`, stable in 1.88 **for 2024 only**); `extern` blocks must be `unsafe extern`; `#[no_mangle]`, `#[export_name]`, `#[link_section]` must be written `#[unsafe(...)]`; `unsafe_op_in_unsafe_fn` warns; references to `static mut` are denied (`static_mut_refs`); never-type fallback is `!` instead of `()`; `std::env::set_var`/`remove_var` and `CommandExt::before_exec` are `unsafe`; `gen` is reserved; `$x:expr` also matches `const {}` and `_`; `Box<[T]>::into_iter` yields values; prelude adds `Future` and `IntoFuture`; Cargo resolver `"3"` (MSRV-aware) by default; rustdoc merges doctests; rustfmt style edition 2024. |

No edition after 2024 is stable as of rustc 1.99.1: `--edition` accepts `2015|2018|2021|2024|future`, and `future` is rejected on stable ("the latest edition supported by this Rust version is 2024").

***

#### Review-Relevant Stabilisations 1.56 – 1.99

Release dates and items come from the official release notes.[^1] An item works in every edition unless an edition is named.

| Release (date) | Language / tooling | Library |
|---|---|---|
| 1.56 (2021-10-21) | **Edition 2021**; `rust-version` in Cargo.toml; bindings after `@` (`x @ Some(y)`) | — |
| 1.57 (2021-12-02) | `panic!` in `const` evaluation; Cargo custom profiles | `Vec::try_reserve` |
| 1.58 (2022-01-13) | Captured identifiers in format strings: `format!("{name}")` (for `panic!` only in edition 2021) | `Option/Result::unwrap_unchecked` |
| 1.59 (2022-02-24) | `asm!`/`global_asm!` (x86, ARM, AArch64, RISC-V); destructuring assignment `(a, b) = (b, a)`; const generic defaults; Cargo `strip` profile option | `thread::available_parallelism`, `iter::zip` |
| 1.60 (2022-04-07) | Cargo `dep:` namespaced features and `pkg?/feat` weak features; `cargo --timings` | `abs_diff` |
| 1.61 (2022-05-19) | `const fn` with trait bounds, `impl Trait`, function pointers | `process::ExitCode`, `Termination`, `Vec::retain_mut` |
| 1.62 (2022-06-30) | `#[derive(Default)]` on enums with `#[default]`; `cargo add` | `bool::then_some`, `f32/f64::total_cmp` |
| 1.63 (2022-08-11) | — | **`std::thread::scope`**; `const` `Mutex::new`/`RwLock::new`/`Condvar::new`; I/O-safety types `OwnedFd`, `BorrowedFd`, `AsFd`; `array::from_fn` |
| 1.64 (2022-09-22) | **Cargo workspace inheritance** (`version.workspace = true`, `[workspace.dependencies]`) | `IntoFuture`; C types in `core::ffi` (`c_int`, `CStr`) |
| 1.65 (2022-11-03) | **`let`-`else`**; **generic associated types (GATs)**; `break` from labelled blocks; uninitialised integers/floats/raw pointers are immediate UB | `std::backtrace::Backtrace`, `io::read_to_string` |
| 1.66 (2022-12-15) | Explicit discriminants on enums with fields (`#[repr(u8)]`); `..=X` patterns; `cargo remove` | `hint::black_box`, `BTreeMap::first_key_value`/`pop_first`, `Duration::try_from_secs_f64` |
| 1.67 (2023-01-26) | `#[must_use]` on `async fn` applies to the output | `std::sync::mpsc` reimplemented on crossbeam-channel; `{integer}::ilog2/ilog10/checked_ilog` |
| 1.68 (2023-03-09) | Sparse registry protocol (default in 1.70) | `std::pin::pin!` |
| 1.69 (2023-04-20) | Cargo suggests `cargo fix`/`cargo clippy --fix` | `CStr::from_bytes_until_nul` |
| 1.70 (2023-06-01) | Sparse protocol default for crates.io | **`OnceCell`, `OnceLock`**, `IsTerminal`, `Option::is_some_and`, `Result::is_ok_and`, `Arc::into_inner` |
| 1.71 (2023-07-13) | **`extern "C-unwind"`** and other `-unwind` ABIs; `raw-dylib` (Windows) | `BuildHasher::hash_one`; `From` between arrays and tuples (arity 1–12) |
| 1.72 (2023-08-24) | `invalid_reference_casting` and other lints uplifted from Clippy | `mpsc::Sender: Sync` |
| 1.73 (2023-10-05) | `invalid_reference_casting` deny-by-default | unsigned `div_ceil`, `next_multiple_of`; `LocalKey<Cell/RefCell>::get/set/take/replace` |
| 1.74 (2023-11-16) | **`[lints]` table in Cargo.toml**; credential providers / registry auth | `core::num::Saturating`; `From<OwnedFd>` for `ChildStdin/Stdout/Stderr` |
| 1.75 (2023-12-28) | **`async fn` and return-position `impl Trait` in traits** | `Option::as_slice`; `ptr` byte-offset methods (`byte_add`, …) |
| 1.76 (2024-02-08) | Rust ABI compatibility documented | `ptr::from_ref`/`from_mut`, `Result::inspect`/`inspect_err`, `Option::inspect`, `Arc/Rc::unwrap_or_clone`, `any::type_name_of_val` |
| 1.77 (2024-03-21) | **C-string literals `c"…"`** (edition 2021+); async recursion with indirection; `static_mut_refs` lint (warn) | `mem::offset_of!`, `slice::first_chunk`/`split_first_chunk`, `Mutex::clear_poison` |
| 1.78 (2024-05-02) | `#[diagnostic::on_unimplemented]`; unsafe-precondition checks run in debug builds of user code; lockfile v4 | `impl Read for &Stdin` |
| 1.79 (2024-06-13) | **Inline `const { … }` blocks**; associated type bounds (`T: Trait<Assoc: Bound>`); temporary lifetime extension through `if`/`match` | `NonZero<T>` (generic), `{integer}::unchecked_add/sub/mul`, `CStr::count_bytes` |
| 1.80 (2024-07-25) | Exclusive range patterns (`0..10 =>`); `--check-cfg` always on (`unexpected_cfgs` lint) | **`LazyCell`, `LazyLock`**, `Option::take_if`, `Vec<[T; N]>::into_flattened`, `IntoIterator for Box<[T]>` |
| 1.81 (2024-09-05) | **`#[expect(lint)]`**; panics escaping `extern "C"` functions abort; new sort implementations (may panic on a non-total `Ord`) | `core::error::Error`, `hint::assert_unchecked`, `fs::exists`, `PanicHookInfo` |
| 1.82 (2024-10-17) | **`&raw const`/`&raw mut`**; **`unsafe extern` blocks**; **unsafe attributes `#[unsafe(no_mangle)]`**; **precise capturing `+ use<'a, T>`**; omitting patterns of empty types; `cargo info` | `Option::is_none_or`, `iter::repeat_n`, `slice::is_sorted`, `Box::new_uninit` |
| 1.83 (2024-11-28) | `&mut`, `*mut`, `&Cell` in `const`; raw lifetimes `'r#a` (2021+) | `ControlFlow` API, `Option::get_or_insert_default`, `Waker::data`, many `const fn`s |
| 1.84 (2025-01-09) | **Cargo resolver v3 / MSRV-aware resolution** (`resolver = "3"` or `resolver.incompatible-rust-versions = "fallback"`) | Strict-provenance pointer APIs (`ptr::without_provenance`, `addr`, `with_addr`, `expose_provenance`), `Ipv6Addr::is_unique_local` |
| 1.85 (2025-02-20) | **Edition 2024**; **async closures** (`async \|\| {}`, `AsyncFn*` traits, in the prelude of all editions); `#[diagnostic::do_not_recommend]` | `{float}::midpoint`, unsigned `midpoint`, `Waker::noop`, tuple `FromIterator`/`Extend` up to 12 |
| 1.86 (2025-04-03) | **Trait-object upcasting** (`&dyn Sub` → `&dyn Super`); `#[target_feature]` on safe functions; debug null-pointer checks on dereference | `slice::get_disjoint_mut`, `HashMap::get_disjoint_mut`, `Vec::pop_if`, `OnceLock::wait`, `str::split_at_checked` |
| 1.87 (2025-05-15) | `use<..>` in trait RPITs; `asm_goto` | **`std::io::pipe`**, `Vec::extract_if`, `LinkedList::extract_if`, `slice::split_off*`, `{integer}::is_multiple_of`, `unbounded_shl/shr`, `cast_signed/cast_unsigned`, signed `midpoint` |
| 1.88 (2025-06-26) | **Let chains (edition 2024 only)**; naked functions (`#[unsafe(naked)]`); `cfg(true)`/`cfg(false)`; Cargo automatic cache garbage collection | `HashMap/HashSet::extract_if`, `slice::as_chunks`, `Cell::update`, `hint::select_unpredictable` |
| 1.89 (2025-08-07) | `_` as an inferred const argument (`f::<_>()`, `[0; _]`); `mismatched_lifetime_syntaxes` lint; `#[repr(u128)]`; doctests run when cross-compiling | **`File::lock`/`lock_shared`/`try_lock`/`unlock`**, `NonNull::from_ref`, `Result::flatten` |
| 1.90 (2025-09-18) | **`lld` is the default linker on `x86_64-unknown-linux-gnu`**; `cargo publish --workspace` | `u{n}::checked_sub_signed` and friends; `CStr`/`CString` comparisons |
| 1.91 (2025-10-30) | Cargo `build.build-dir`; `--target host-tuple`; `semicolon_in_expressions_from_macros` deny; `dangling_pointers_from_locals` lint | `{integer}::strict_*` (panicking arithmetic in all profiles), `Path::file_prefix`, `PathBuf::add_extension`, `str::floor_char_boundary`/`ceil_char_boundary`, `BTreeMap::extract_if`, `Duration::from_mins/from_hours`, `carrying_add`/`borrowing_sub` |
| 1.92 (2025-12-11) | Never-type fallback lints deny-by-default; `-C panic=abort` builds have unwind tables (backtraces) | `RwLockWriteGuard::downgrade`, `Box/Rc/Arc::new_zeroed`, `btree_map::Entry::insert_entry` |
| 1.93 (2026-01-22) | `deref_nullptr` deny-by-default; `const_item_interior_mutations` lint; bundled musl 1.2.5 | `Vec/String::into_raw_parts`, `slice::as_array`, `VecDeque::pop_front_if/pop_back_if`, `fmt::from_fn`, `[MaybeUninit<T>]::write_copy_of_slice` |
| 1.94 (2026-03-05) | Unicode 17; Cargo config `include`; TOML 1.1 in manifests | `slice::array_windows`, `LazyLock/LazyCell::get/get_mut/force_mut`, `Peekable::next_if_map` |
| 1.95 (2026-04-16) | **`if let` guards on match arms**; `cfg_select!` | `AtomicXxx::update/try_update`, `Vec::push_mut`/`insert_mut`, `core::range` (new `Copy` range types), `hint::cold_path`, `bool: TryFrom<{integer}>` |
| 1.96 (2026-05-28) | `cfg` accepts `expr` metavariables | **`assert_matches!`/`debug_assert_matches!`**, `From<T>` for `LazyLock`/`LazyCell`, `core::range::Range*` |
| 1.97 (2026-07-09) | v0 symbol mangling by default; Cargo `build.warnings`; `resolver.lockfile-path`; `pin!` no longer deref-coerces | `{integer}::bit_width`, `highest_one`, `lowest_one`, `isolate_*_one` |
| 1.98 (2026-08-20) | `invalid_runtime_symbol_definitions` (deny) lint | `str::substr_range`, `{integer}::format_into` + `fmt::NumBuffer`, `String::from_utf16le/be`, `bool::ok_or`/`ok_or_else`, `str::strip_circumfix`, `NonZero::from_str_radix` |
| 1.99 (2026-10-01) | **C-variadic function definitions** (`unsafe extern "C" fn f(args: ...)`); `#[my_attr] mod foo;`; Cargo `debug` profile; edition-2024 members can override inherited `default-features` | `Vec::from_parts/into_parts`, `Box::into_non_null/from_non_null`, `String::from_utf8_lossy_owned`, `fs::set_times`, `mem::size_of_val_raw`, `VecDeque::retain_back` |

***

#### Do Not Use or Recommend Without a Version Check

Each item below is unavailable before the listed release (or edition). Recommend it only when `rust-version`/the toolchain is at least that release; otherwise recommend the older idiom in the last column.

| Feature | Needs | Older alternative |
|---|---|---|
| `let`-`else` | 1.65 | `let x = match … { Some(x) => x, None => return … };` |
| Let chains `if let Some(x) = a && x > 0` | 1.88 **and edition 2024** | Nested `if let`, or `match` |
| `if let` guards in `match` (`Some(x) if let Ok(y) = f(x) =>`) | 1.95 | Nested `match` inside the arm |
| GATs (`type Item<'a> where Self: 'a;`) | 1.65 | Lifetime on the trait |
| `async fn` / `-> impl Trait` in traits | 1.75 | `async-trait` crate, boxed futures |
| Async closures `async \|x\| …`, `AsyncFn` bounds | 1.85 | Closure returning `async move {}` block |
| Trait-object upcasting `&dyn Sub as &dyn Super` | 1.86 | `fn as_super(&self) -> &dyn Super` method |
| `+ use<'a>` precise capturing | 1.82 (in traits 1.87) | `Captures` trick, explicit lifetimes |
| `&raw const` / `&raw mut` | 1.82 | `ptr::addr_of!` / `addr_of_mut!` (1.51) |
| `unsafe extern "C" { … }`, `#[unsafe(no_mangle)]` | 1.82 (required in 2024) | `extern "C" { … }`, `#[no_mangle]` (2015–2021) |
| Inline `const { … }` | 1.79 | Named `const` item |
| C-string literal `c"…"` | 1.77 **and edition 2021+** | `CStr::from_bytes_with_nul(b"…\0")` |
| `#[expect(lint)]` | 1.81 | `#[allow(lint)]` |
| `std::sync::LazyLock`, `std::cell::LazyCell` | 1.80 | `OnceLock` + accessor (1.70), `once_cell`, `lazy_static` |
| `OnceLock`, `OnceCell` | 1.70 | `once_cell` crate |
| `std::thread::scope` | 1.63 | `crossbeam::scope` |
| `std::io::pipe` | 1.87 | `os_pipe` crate |
| `File::lock` / `try_lock` | 1.89 | `fs4`/`fd-lock` crates |
| `assert_matches!` | 1.96 | `assert!(matches!(…))` |
| `{integer}::strict_add` etc. | 1.91 | `checked_add(..).expect(..)` |
| `Option::is_none_or` | 1.82 | `map_or(true, …)` |
| `Option::is_some_and` | 1.70 | `map_or(false, …)` |
| `slice::get_disjoint_mut` | 1.86 | `split_at_mut` |
| `Vec::extract_if` | 1.87 | `retain` + separate collect, or index loop |
| `Vec::pop_if` | 1.86 | `if v.last().is_some_and(..) { v.pop() }` |
| `str::floor_char_boundary` | 1.91 | loop with `is_char_boundary` |
| `iter::repeat_n` | 1.82 | `iter::repeat(x).take(n)` |
| `core::error::Error` (`no_std` errors) | 1.81 | `std::error::Error` only |
| `mem::offset_of!` | 1.77 | `memoffset` crate |
| `Cargo.toml [lints]` | 1.74 | `#![deny(...)]` in `lib.rs`, `RUSTFLAGS` |
| `resolver = "3"` | 1.84 | `resolver = "2"` |
| `dep:` / `crate?/feature` in `[features]` | 1.60 | Implicit features named after optional deps |
| Workspace inheritance `*.workspace = true` | 1.64 | Repeat values in each member |
| `#[derive(Default)]` + `#[default]` on enums | 1.62 | Manual `impl Default` |
| Format-string captures `"{x}"` | 1.58 | `"{}", x` |

Nightly-only features must never be recommended for stable code: `#![feature(...)]`, `try` blocks, `gen` blocks, `!` as a type outside function returns, specialization, `async` drop, `Mutex` without poisoning (`std::sync::nonpoison`), `f16`/`f128`, the `Allocator` trait / `Vec::new_in`, `Box::new_uninit_slice_in`, `cargo -Z` flags, `build-std`, and Miri itself (which runs on nightly only).

***

#### Deprecated, Removed and Changed Behaviour

| Item | Status | Replacement |
|---|---|---|
| `std::i32::MAX`, `std::u64::MIN`, `std::f64::EPSILON` (legacy numeric modules) | Fully deprecated in 1.99 (warns) | `i32::MAX`, `f64::EPSILON` |
| `std::char` constants and functions (`std::char::from_u32`, `std::char::MAX`) | Marked for deprecation in 1.97 (warns only under `#![warn(deprecated_in_future)]` in 1.99) | `char::from_u32`, `char::MAX` |
| `mem::uninitialized` | Deprecated since 1.39; UB for almost every type. Current std fills the memory with `0x01` bytes and panics for types where that is still invalid (references, uninhabited types) — a `bool` silently becomes `true` | `MaybeUninit` |
| `try!` macro | Deprecated since 1.39; `try` is a keyword from 2018 | `?` |
| `description()`/`cause()` on `Error` | Deprecated | `Display`, `source()` |
| `#[bench]` | Removed from stable (deny-by-default FCW, fully destabilised in 1.88) | `criterion`, `divan` |
| `std::env::home_dir` | Deprecated in 1.29; Windows behaviour fixed in 1.85; un-deprecated in 1.87 | `std::env::home_dir` (1.87+), the `home`/`dirs` crates before |
| `Eq::assert_receiver_is_total_eq` (manual impls) | Deprecated in 1.95 | Derive `Eq` |
| `lazy_static!` | Not std; superseded | `LazyLock` (1.80) |
| `async-trait` | Still needed for `dyn`-compatible async traits | Native `async fn` in traits (1.75) where `dyn` is not needed |
| `extern crate foo;` | Unneeded since edition 2018 (except `alloc`, `proc_macro` in some setups) | `use foo::…` |

Behaviour changes that break code silently:

- **1.81**: the new `sort`/`sort_unstable`/`select_nth_unstable` implementations **may panic** when `Ord`/the comparator is not a total order (previously the result was just unspecified). Comparators such as `|a, b| a.partial_cmp(b).unwrap()` on floats with NaN, or ones that return `Less` for equal elements, are bugs.
- **1.81**: a panic escaping an `extern "C"` function **aborts** the process (previously UB). Use `extern "C-unwind"` (1.71) when unwinding across the boundary is intended.
- **1.90**: `lld` is the default linker on `x86_64-unknown-linux-gnu`; custom linker scripts or `-C link-arg` flags written for GNU `ld`/`gold` may need adjusting (`-C linker-features=-lld` opts out).
- **1.90**: `UnixStream` sets `MSG_NOSIGNAL`: writing to a closed socket returns `EPIPE` instead of killing the process with `SIGPIPE`.
- **1.97**: v0 symbol mangling is the default; old profilers/debuggers may not demangle.
- **1.99**: the observable state of an exhausted `RangeInclusive` iterator (`start()`, `end()`, slicing with it) changed; never rely on it.
- **1.99**: Cargo disables incremental compilation by default when the `CI` environment variable is set.
- Edition 2021 disjoint captures and edition 2024 temporary-scope changes alter **drop order** (see [Edition Migration Notes](#edition-migration-notes)).

***

#### Formatting and Naming

- Code is formatted by `rustfmt` (`cargo fmt`; CI runs `cargo fmt --check`). Do not hand-align or argue about formatting that rustfmt owns. A `rustfmt.toml` with `style_edition = "2024"` (rustfmt in 1.85+) opts into the 2024 style (version-sorted imports, raw identifiers sorted by name); without it the style edition follows the crate edition.[^5]
- Naming (RFC 430, enforced by the `non_snake_case`, `non_camel_case_types`, `non_upper_case_globals` lints):

| Item | Convention | Example |
|---|---|---|
| Crates, modules, functions, methods, locals, fields | `snake_case` | `parse_header`, `mod wire_format` |
| Types, traits, enum variants, type parameters | `UpperCamelCase` | `HttpClient`, `IntoIter`, `T`, `K` |
| `const`, `static` | `SCREAMING_SNAKE_CASE` | `MAX_RETRIES` |
| Lifetimes | short lowercase | `'a`, `'de`, `'src` |
| Acronyms | one word | `Uuid`, `HttpServer`, not `UUID`, `HTTPServer` |
| Conversions | `as_` (cheap, borrowed), `to_` (expensive or owned), `into_` (consumes `self`) | `as_str`, `to_string`, `into_bytes` |
| Getters | no `get_` prefix | `fn name(&self) -> &str` |
| Iterators | `iter`, `iter_mut`, `into_iter` | |

- Crate names on crates.io use `-`; in code they become `_` (Cargo normalises `lib.name` dashes to underscores since 1.79).
- Prefer `use` of the item's module for functions (`fs::read_to_string`) and the item itself for types (`HashMap`). Avoid glob imports outside preludes and `use super::*` in test modules; glob-imported names can become ambiguous when a dependency adds an item (1.94 turned several of these ambiguities into errors).
- Doc comments: `///` on every public item, `//!` for the crate/module. Public functions that can fail or panic document it under `# Errors` and `# Panics`; `unsafe fn` documents its contract under `# Safety` (Clippy `missing_errors_doc`, `missing_panics_doc`, `missing_safety_doc`).

***

#### Ownership, Borrowing and Lifetimes

##### Core rules

- Each value has one owner; assignment and by-value passing **move** unless the type is `Copy`. Use after move is a compile error, not a runtime bug.
- At any time a value has **either** one `&mut` **or** any number of `&`. References never outlive the referent. Non-lexical lifetimes (NLL; edition 2018, and 2015 since 1.36) end a borrow at its last use.
- `Drop` runs at the end of the owning scope in reverse declaration order; temporaries drop at the end of the enclosing statement (with edition-2024 refinements, see migration notes).

##### Borrow instead of clone

```rust
// BAD: forces callers to allocate, and clones to dodge the borrow checker.
fn count_words_bad(text: String) -> usize {
    text.split_whitespace().count()
}

// GOOD: borrow the most general form (&str, &[T], &Path, impl AsRef<..>).
fn count_words(text: &str) -> usize {
    text.split_whitespace().count()
}

fn main() {
    let s = String::from("a b c");
    assert_eq!(count_words(&s), 3);
    assert_eq!(count_words_bad(s.clone()), 3);
}
```

Review heuristics:

- `.clone()` on a `String`, `Vec` or large struct inside a loop or hot path, added to "satisfy the borrow checker", is a finding: restructure borrows, take ownership earlier, or share with `Rc`/`Arc`.
- Parameters typed `&String`, `&Vec<T>`, `&Box<T>` should be `&str`, `&[T]`, `&T` (Clippy `ptr_arg`, `borrowed_box`).
- Take ownership (`String`, `Vec<T>`) only when the function stores the value; then let the caller decide whether to clone. `impl Into<String>` is acceptable for constructors.
- `Cow<'_, str>` avoids allocation when a value is usually borrowed but sometimes modified.

##### Lifetimes

```rust
// Elision: one input reference -> the output borrows from it.
fn first_word(s: &str) -> &str {
    s.split(' ').next().unwrap_or("")
}

// Two inputs: the output must name which one it borrows from.
fn longer<'a>(a: &'a str, b: &'a str) -> &'a str {
    if a.len() >= b.len() { a } else { b }
}

// A struct holding a borrow carries the lifetime; it cannot outlive the source.
struct Parser<'src> {
    input: &'src str,
    pos: usize,
}

impl<'src> Parser<'src> {
    fn rest(&self) -> &'src str {
        &self.input[self.pos..]
    }
}

fn main() {
    assert_eq!(first_word("hello world"), "hello");
    assert_eq!(longer("ab", "abc"), "abc");
    let p = Parser { input: "abc", pos: 1 };
    assert_eq!(p.rest(), "bc");
}
```

- `'static` in a bound (`T: 'static`) means "contains no non-static borrows", not "lives forever". `&'static str` is a string literal or leaked data.
- Lifetime parameters on long-lived structs (`struct Service<'a>`) are a design smell in application code; owned data or `Arc` is usually simpler.
- Hide elided lifetimes consistently: `fn iter(&self) -> Iter<'_, T>`. The `mismatched_lifetime_syntaxes` lint (1.89, warn) flags signatures that hide a lifetime in the output that is visible in the input or vice versa.
- `Box::leak` to obtain `&'static` is acceptable for process-lifetime configuration only; 1.99 documentation recommends against later reclaiming leaked memory (use `Box::into_raw`/`Box::into_non_null` if it must be freed).

##### Interior mutability

| Type | Thread-safe | Failure mode |
|---|---|---|
| `Cell<T>` | No | none (`T: Copy` for `get`, or `replace`/`take`) |
| `RefCell<T>` | No | **panics** on conflicting `borrow`/`borrow_mut` at runtime |
| `OnceCell<T>` (1.70), `LazyCell<T>` (1.80) | No | init once |
| `Mutex<T>`, `RwLock<T>` | Yes | blocking, poisoning, deadlock |
| `OnceLock<T>` (1.70), `LazyLock<T>` (1.80) | Yes | init once, blocks concurrent initialisers |
| `Atomic*` | Yes | ordering bugs |

```rust
use std::cell::RefCell;

fn main() {
    let v = RefCell::new(vec![1, 2, 3]);
    // BAD: holding a `Ref` while borrowing mutably panics at runtime:
    // let first = v.borrow(); v.borrow_mut().push(4); // BorrowMutError
    // GOOD: keep borrows short; try_borrow_mut reports instead of panicking.
    let len = v.borrow().len();
    v.borrow_mut().push(len as i32 + 1);
    let r = v.borrow();
    assert!(v.try_borrow_mut().is_err());
    assert_eq!(*r, vec![1, 2, 3, 4]);
}
```

##### Smart pointers

- `Box<T>`: single owner on the heap; recursive types; trait objects.
- `Rc<T>`/`Arc<T>`: shared ownership; `Rc` is `!Send`. Cycles leak: use `Weak` for back-references (parent pointers, observer lists).
- `Arc<Mutex<T>>` is the default for shared mutable state across threads; `Arc<RwLock<T>>` only when reads dominate and are long; `Arc<T>` with immutable `T` needs no lock.
- `Arc::clone(&a)` over `a.clone()` makes the cheap reference-count increment visible (Clippy `clone_on_ref_ptr`, restriction group).
- `Rc::make_mut`/`Arc::make_mut` implement clone-on-write; `Arc::unwrap_or_clone` (1.76) and `Arc::into_inner` (1.70) extract the value.

***

#### Error Handling

##### `Result`, `?` and error types

- Recoverable failures return `Result<T, E>`. `?` propagates the error and converts it with `From::from`.
- **Libraries** expose concrete error types (an `enum` implementing `std::error::Error`) so callers can match. **Applications** may use a dynamic error (`anyhow::Error`, `Box<dyn Error + Send + Sync>`) and add context.
- Error types are `Send + Sync + 'static` so they cross threads and tasks; `Box<dyn Error>` without `Send + Sync` cannot be returned from a spawned task.
- `Display` messages are lowercase, without trailing punctuation, and describe *this* layer only; the cause is reachable via `source()`. Do not repeat the source's message in `Display` **and** return it from `source()` — reporters print the chain and the text appears twice.

```rust
use std::error::Error;
use std::fmt;
use std::num::ParseIntError;

#[derive(Debug)]
pub enum ConfigError {
    Missing(&'static str),
    BadPort { value: String, source: ParseIntError },
}

impl fmt::Display for ConfigError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Missing(key) => write!(f, "missing key `{key}`"),
            Self::BadPort { value, .. } => write!(f, "invalid port {value:?}"),
        }
    }
}

impl Error for ConfigError {
    fn source(&self) -> Option<&(dyn Error + 'static)> {
        match self {
            Self::BadPort { source, .. } => Some(source),
            Self::Missing(_) => None,
        }
    }
}

fn port(raw: Option<&str>) -> Result<u16, ConfigError> {
    let value = raw.ok_or(ConfigError::Missing("port"))?;
    value.parse().map_err(|source| ConfigError::BadPort { value: value.to_owned(), source })
}

fn main() {
    assert_eq!(port(Some("8080")).unwrap(), 8080);
    let err = port(Some("80x")).unwrap_err();
    assert_eq!(err.to_string(), "invalid port \"80x\"");
    assert!(err.source().is_some());
    assert!(matches!(port(None), Err(ConfigError::Missing("port"))));
}
```

##### `thiserror` (libraries) and `anyhow` (applications)

`thiserror` 2.x derives the boilerplate above; `#[from]` generates a `From` impl **and** marks the field as `source`; `#[error(transparent)]` forwards `Display` and `source` to the inner error.[^6]

```rust,ignore
use thiserror::Error;

#[derive(Debug, Error)]
pub enum StoreError {
    #[error("record {id} not found")]
    NotFound { id: u64 },
    #[error("failed to read {path}")]
    Io {
        path: std::path::PathBuf,
        #[source]
        source: std::io::Error,
    },
    #[error(transparent)]
    Parse(#[from] std::num::ParseIntError),
}

fn load(path: &std::path::Path) -> Result<u64, StoreError> {
    let text = std::fs::read_to_string(path)
        .map_err(|source| StoreError::Io { path: path.to_owned(), source })?;
    Ok(text.trim().parse()?) // ParseIntError -> StoreError::Parse via #[from]
}
```

Review points for `thiserror`:

- BAD: `#[error("io error: {0}")] Io(#[from] std::io::Error)` — the message duplicates the source. GOOD: `#[error("failed to read config")]` plus `#[source]`/`#[from]`.
- A blanket `#[from] std::io::Error` loses *which* file failed. Prefer a variant with a `path` field built by `map_err`.
- Adding a variant to a public error `enum` is a breaking change unless the enum is `#[non_exhaustive]`.

```rust,ignore
use anyhow::{bail, Context, Result};

fn read_port(path: &std::path::Path) -> Result<u16> {
    let text = std::fs::read_to_string(path)
        .with_context(|| format!("reading {}", path.display()))?;
    let port: u16 = text.trim().parse().context("parsing port")?;
    if port == 0 {
        bail!("port must be non-zero");
    }
    Ok(port)
}

fn main() -> Result<()> {
    if let Err(e) = read_port(std::path::Path::new("/nonexistent")) {
        // {:#} prints the whole chain on one line: "reading /nonexistent: No such file ..."
        eprintln!("{e:#}");
    }
    Ok(())
}
```

- `.context("…")` with a `format!` argument allocates even on success; use `.with_context(|| format!(…))`.
- `anyhow::Error` in a library's public API takes away the caller's ability to match; keep it in binaries.
- `main() -> Result<(), E>` prints `Error: {e:?}` (the `Debug` form) and exits with code 1. For user-facing CLIs return `std::process::ExitCode` (1.61) and print the `Display` chain yourself.

##### Patterns and anti-patterns

```rust
use std::collections::HashMap;

fn lookup(map: &HashMap<String, u32>, key: &str) -> Option<u32> {
    // GOOD: Option combinators instead of unwrap.
    map.get(key).copied()
}

fn parse_all(items: &[&str]) -> Result<Vec<u32>, std::num::ParseIntError> {
    // collect() into Result stops at the first error.
    items.iter().map(|s| s.parse::<u32>()).collect()
}

fn main() {
    let mut m = HashMap::new();
    m.insert("a".to_string(), 1);
    assert_eq!(lookup(&m, "a"), Some(1));
    assert!(parse_all(&["1", "x", "3"]).is_err());
    assert_eq!(parse_all(&["1", "2"]).unwrap(), vec![1, 2]);

    // BAD: errors silently discarded.
    let _ = std::fs::remove_file("/tmp/does-not-exist");
    // GOOD: ignore only the expected error kind.
    match std::fs::remove_file("/tmp/does-not-exist") {
        Ok(()) => {}
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
        Err(e) => eprintln!("cleanup failed: {e}"),
    }
}
```

- `let _ = fallible();` and `.ok();` discard errors; each needs a reason. `#[must_use]` on `Result` makes a bare `fallible();` warn (`unused_must_use`).
- `.unwrap_or_default()` on a parse result turns bad input into `0`/empty — a silent data bug in validation code.
- `Option<Result<T, E>>` ↔ `Result<Option<T>, E>` conversions: `transpose()`.
- Matching error strings (`e.to_string().contains("not found")`) is brittle; match `ErrorKind` or variants, or `downcast_ref` for dynamic errors.
- `Box<dyn Error>` from `?` on a `&str`/`String` works via `From`; prefer typed errors in libraries.

***

#### Panics

- A panic is for **bugs** (violated invariants, impossible states), not for invalid input, I/O failure, or anything a caller can trigger. Library code that panics on input needs a `# Panics` doc section and usually a fallible alternative.
- `unwrap()` is acceptable in tests, in examples, after a check that proves success, and for compile-time-known values (`"127.0.0.1".parse::<IpAddr>().unwrap()`). Elsewhere prefer `expect("why this cannot fail")` that states the invariant, or propagate. Clippy `unwrap_used`/`expect_used` (restriction) can enforce this per crate.
- Hidden panics a reviewer must spot: indexing `v[i]`, slicing `s[a..b]` (out of bounds **or** not on a UTF-8 char boundary), integer division/remainder by zero, `RefCell` double borrow, `.unwrap()` on `Mutex::lock()` when poisoned, `Duration` subtraction underflow (`d1 - d2`; `Instant - Instant` saturates to zero since 1.60, but `Instant - Duration` can still panic), `Vec::with_capacity` overflow, `chunks(0)`/`windows(0)` (since 1.99 the deny-by-default `unconditional_panic` lint rejects a zero **const-generic** size such as `array_windows::<0>()` or `as_chunks::<0>()`, but not a runtime `chunks(0)`), `Iterator::step_by(0)`, `str::repeat` overflow, arithmetic overflow in debug builds, and `iter::repeat(x).last()`/`count()` which panic since 1.92.
- `panic = "abort"` in `[profile.release]` turns every panic into an abort: destructors do not run, `catch_unwind` stops working, and tests (which always unwind) behave differently from release. Use it deliberately (embedded, size-sensitive binaries).
- Panics in `Drop` while already unwinding abort the process. Destructors must not panic.
- A panic in a spawned thread is returned by `JoinHandle::join()` as `Err`; a panic in a Tokio task is returned as `JoinError::is_panic()`. Ignoring the join result hides the crash.
- `std::panic::catch_unwind` is for FFI boundaries and thread-pool isolation, not for error handling; it requires `UnwindSafe` and does nothing under `panic = "abort"`.
- 1.81+: a panic that escapes an `extern "C"` function aborts. Wrap the body in `catch_unwind` and return an error code (see [FFI](#ffi)).

```rust
fn checked_ratio(a: u32, b: u32) -> Option<u32> {
    // BAD: `a / b` panics when b == 0.
    a.checked_div(b)
}

fn main() {
    let v = [10, 20, 30];
    // BAD: v[5] panics. GOOD: get() returns Option.
    assert_eq!(v.get(5), None);
    assert_eq!(checked_ratio(10, 0), None);

    let s = "héllo";
    // BAD: &s[0..2] panics: byte 2 is inside 'é'.
    assert!(s.get(0..2).is_none());
    assert_eq!(s.get(0..3), Some("hé"));
}
```

***

#### Traits, Generics and Trait Objects

##### Static vs. dynamic dispatch

```rust
use std::fmt::Display;

// Static dispatch: monomorphised per type, inlinable, no allocation.
fn show_all<T: Display>(items: &[T]) -> String {
    items.iter().map(|i| i.to_string()).collect::<Vec<_>>().join(",")
}

// `impl Trait` in argument position is sugar for a generic parameter.
fn print_it(x: impl Display) -> String {
    format!("<{x}>")
}

// Dynamic dispatch: one copy of the code, vtable call, heterogeneous collections.
fn show_dyn(items: &[Box<dyn Display>]) -> String {
    items.iter().map(|i| i.to_string()).collect::<Vec<_>>().join(",")
}

fn main() {
    assert_eq!(show_all(&[1, 2]), "1,2");
    assert_eq!(print_it("x"), "<x>");
    let mixed: Vec<Box<dyn Display>> = vec![Box::new(1), Box::new("a")];
    assert_eq!(show_dyn(&mixed), "1,a");
}
```

- Prefer generics in hot code and public APIs that callers use with one type; prefer `dyn Trait` for plugin-style heterogeneity, to cut compile time/binary size, or when generic parameters would leak through many layers.
- `dyn Trait` requires a **dyn-compatible** (formerly "object-safe") trait: no generic methods without `where Self: Sized`, no `Self` by value in arguments/returns, no associated consts. `async fn` in traits (1.75) makes a trait not dyn-compatible; use the `async-trait` crate or return `Pin<Box<dyn Future<Output = T> + Send + '_>>` when `dyn` is needed.
- `Box<dyn Trait>` defaults to `Box<dyn Trait + 'static>`; `&'a dyn Trait` defaults to `+ 'a`. Add `+ Send + Sync` when the object crosses threads.
- Trait-object upcasting (`&dyn Sub` → `&dyn Super`) is stable since 1.86. Before 1.86, add an `as_super()` method.

##### Trait design

- Implement standard traits eagerly on public types: `Debug` (always), `Clone`, `Copy` (small plain data), `PartialEq`/`Eq`, `Hash`, `PartialOrd`/`Ord`, `Default`, `Display` (user-facing types), `From`/`TryFrom`, `AsRef`, `Send`/`Sync` (auto). Missing `Debug` on a public type is a finding (`missing_debug_implementations`).
- `Hash` must agree with `Eq`: `a == b` implies `hash(a) == hash(b)`. `Ord` must agree with `PartialOrd` and `Eq`. Since 1.98 a type that derives both `PartialOrd` and `Ord` implements `partial_cmp` through `Ord::cmp`, so a field type whose `PartialOrd` and `Ord` disagree changes the result of `<` on the outer type (verified: `W(K(1)) < W(K(2))` is `true` on 1.85 and `false` on 1.99 when `K` has a derived `PartialOrd` and a reversed manual `Ord`). Since 1.81 sorting may panic on a non-total `Ord`.
- Implement `From`, not `Into` (the blanket impl provides `Into`). Implement `TryFrom` for fallible conversions; `FromStr` for parsing.
- Sealed traits (a public trait with a private supertrait) prevent downstream implementations so methods can be added without a breaking change.
- Use associated types when there is exactly one natural choice per implementing type (`Iterator::Item`); generic parameters when a type can implement the trait several times (`From<T>`).
- Blanket impls (`impl<T: Display> MyTrait for T`) block every other impl that could overlap; adding one to a published crate is a breaking change.
- The orphan rule allows `impl ForeignTrait for LocalType` and `impl LocalTrait for ForeignType`, never `impl ForeignTrait for ForeignType`; wrap in a newtype.

##### Return-position `impl Trait` and lifetime capture

In **edition 2024**, `-> impl Trait` captures **all** in-scope generic parameters, including lifetimes. In editions 2015–2021, it captures type parameters but **not** lifetime parameters unless they appear in the bounds. `+ use<..>` (1.82) states the captures explicitly in every edition.[^4]

```rust
// Edition 2024: the hidden type may borrow from `v`; callers cannot drop `v`
// while the iterator lives. `use<>` says "captures nothing".
fn indices(v: &[u8]) -> impl Iterator<Item = usize> + use<> {
    0..v.len()
}

fn main() {
    let it;
    {
        let v = vec![1u8, 2, 3];
        it = indices(&v);
    } // v dropped; fine because the iterator captures no lifetime
    assert_eq!(it.count(), 3);
}
```

##### Generics hygiene

- Put bounds where they are needed: on `impl` blocks and functions, not on struct definitions (`struct Wrapper<T: Clone>` forces every user to repeat the bound).
- `where` clauses for more than one or two bounds.
- `T: ?Sized` on functions that only use `&T` so they accept `str`, `[T]`, `dyn Trait`.
- Const generics (`[T; N]`, 1.51) instead of macro-generated impls for array sizes.
- GATs (1.65) for lending iterators and borrowed associated types.
- Associated type bounds `T: Iterator<Item: Display>` (1.79) shorten `where T: Iterator, T::Item: Display`.

***

#### Iterators and Closures

- Iterator chains are zero-cost after optimisation and avoid bounds checks; prefer them over index loops for transformations. Plain `for` loops are fine (and clearer) for side effects.
- Iterators are **lazy**: `iter.map(f);` without a consumer does nothing (`unused_must_use` warns).
- `collect()` needs a target type; `collect::<Result<Vec<_>, _>>()` stops at the first error, `collect::<Option<Vec<_>>>()` at the first `None`.
- `iter()` borrows, `iter_mut()` borrows mutably, `into_iter()` consumes (or borrows, for `&Vec`).
- `into_iter()` on an **array** by value yields `T` from edition 2021; in 2015/2018, `array.into_iter()` (method-call syntax) resolves to the slice iterator and yields `&T` (the `array_into_iter` lint warns). `Box<[T]>::into_iter()` yields `T` only from edition 2024 (`IntoIterator for Box<[T]>` exists since 1.80; method-call syntax in 2015–2021 still yields `&T`).

```rust
fn main() {
    let words = ["alpha", "beta", "gamma", "delta"];

    let lens: Vec<usize> = words.iter().map(|w| w.len()).collect();
    assert_eq!(lens, [5, 4, 5, 5]);

    // filter_map, find, position, any/all, fold, sum, max_by_key, rev, zip, chain
    let first_long = words.iter().find(|w| w.len() > 4);
    assert_eq!(first_long, Some(&"alpha"));
    let total: usize = words.iter().map(|w| w.len()).sum();
    assert_eq!(total, 19);
    let longest = words.iter().max_by_key(|w| w.len()).unwrap();
    assert_eq!(*longest, "delta"); // max_by_key returns the LAST maximum

    // enumerate instead of a manual counter
    for (i, w) in words.iter().enumerate().skip(3) {
        assert_eq!((i, *w), (3, "delta"));
    }

    // Edition 2021+: arrays iterate by value.
    let owned: Vec<String> = ["x", "y"].map(String::from).into_iter().collect();
    assert_eq!(owned, ["x", "y"]);
}
```

Review points:

- `max_by_key`/`max_by` return the **last** maximal element, `min_by_key` the **first** minimal one.
- `.collect::<Vec<_>>()` followed by `.len()`, `.is_empty()`, `.iter()` or a single `for` loop is a needless allocation (Clippy `needless_collect`).
- `.iter().count()` on a slice or `Vec` → `.len()`.
- `zip` silently stops at the shorter iterator; assert lengths when they must match.
- `.chars().rev()` reverses code points, not grapheme clusters.
- `for i in 0..v.len() { v[i] … }` → `for x in &v` or `iter().enumerate()` (Clippy `needless_range_loop`).
- `Iterator::step_by(0)` panics.
- Closures in `map`/`filter` run lazily and only for the elements the consumer pulls (`find`, `any`, `take_while` stop early); never rely on their side effects. `inspect` is for debugging.
- `sum()`/`product()` overflow panics in debug and wraps in release like other arithmetic.

Closures:

- `Fn` (shared borrow), `FnMut` (mutable borrow), `FnOnce` (consumes captures). Accept the most permissive bound the code needs: `FnOnce` for callbacks called once, `FnMut` for repeated calls.
- `move` closures take ownership of captures; required for threads (`thread::spawn`) and `'static` tasks. Clone an `Arc` *before* the `move` closure.
- Edition 2021+ captures disjoint fields (`self.a` alone) instead of the whole variable; in 2018 a closure using `s.a` captures all of `s`. This affects borrow conflicts, `Send`-ness and drop order.

***

#### Collections

| Need | Type | Notes |
|---|---|---|
| Growable list | `Vec<T>` | `with_capacity` when size known; `swap_remove` is O(1), `remove(0)` is O(n) |
| Queue / deque | `VecDeque<T>` | `pop_front` O(1); not `Vec::remove(0)` |
| Key → value, fast | `HashMap<K, V>` | SipHash-1-3, randomly seeded (HashDoS-resistant); **iteration order is random** |
| Ordered map | `BTreeMap<K, V>` | sorted iteration, range queries, deterministic output |
| Set | `HashSet<T>` / `BTreeSet<T>` | |
| Priority queue | `BinaryHeap<T>` | max-heap; `Reverse(x)` for min-heap |
| Small inline vector | `smallvec`/`arrayvec` crates | measure first |

```rust
use std::collections::{BTreeMap, HashMap};

fn main() {
    let text = "a b a c b a";

    // Entry API: one hash lookup instead of contains_key + insert.
    let mut counts: HashMap<&str, usize> = HashMap::new();
    for w in text.split_whitespace() {
        *counts.entry(w).or_insert(0) += 1;
    }
    assert_eq!(counts["a"], 3);

    // Deterministic output (tests, files, hashes): use BTreeMap or sort.
    let sorted: BTreeMap<_, _> = counts.into_iter().collect();
    let line: Vec<String> = sorted.iter().map(|(k, v)| format!("{k}={v}")).collect();
    assert_eq!(line.join(" "), "a=3 b=2 c=1");

    // retain instead of collecting indices and removing.
    let mut v: Vec<i32> = (1..=10).collect();
    v.retain(|x| x % 2 == 0);
    assert_eq!(v, [2, 4, 6, 8, 10]);

    // Remove duplicates: dedup only removes *consecutive* duplicates.
    let mut d = vec![3, 1, 3, 2, 1];
    d.sort_unstable();
    d.dedup();
    assert_eq!(d, [1, 2, 3]);
}
```

Review points:

- Never depend on `HashMap`/`HashSet` iteration order in output, tests, serialisation or hashing; it changes between runs.
- `HashMap` with a custom fast hasher (`FxHashMap`, `ahash`) on **attacker-controlled keys** reintroduces HashDoS.
- `map.contains_key(k)` followed by `map.insert(k, …)` or `map[k]` → `entry` API (Clippy `map_entry`).
- `map[&key]` panics when the key is missing; use `get`.
- Keys must not be mutated (via interior mutability) in a way that changes `Hash`/`Ord` while in the map.
- `sort()` is stable (allocates), `sort_unstable()` is in-place and faster; use `sort_by_key` with `cmp::Reverse` for descending; `sort_by_cached_key` when the key is expensive.
- `Vec<Vec<T>>` for a fixed-width matrix → one `Vec<T>` with index arithmetic, or `chunks_exact`.
- Unbounded collections fed by external input (caches, per-client maps, queues) need a size limit or eviction.

***

#### Strings, UTF-8, `OsStr` and `Path`

- `String`/`&str` are always valid UTF-8. `len()` is the **byte** length; indexing by integer (`s[0]`) does not compile; slicing `&s[a..b]` panics unless both ends are on char boundaries. Use `s.get(a..b)`, `char_indices()`, or `floor_char_boundary` (1.91) when truncating.
- `chars()` iterates Unicode scalar values, not user-perceived characters; use the `unicode-segmentation` crate for graphemes (truncating display names, cursor movement).
- `to_lowercase`/`to_uppercase` are Unicode-aware and can change length (`"ß".to_uppercase() == "SS"`); `eq_ignore_ascii_case`/`to_ascii_lowercase` are for protocol tokens (HTTP headers, hex).
- `String::from_utf8(bytes)` validates; `String::from_utf8_lossy` replaces invalid sequences with U+FFFD (data loss: never for round-tripping); `from_utf8_unchecked` is `unsafe` and UB on invalid input.
- Build strings with `push_str`, `write!` into a `String`, or `format!`; `s = s + &t` in a loop is fine (amortised) but `format!("{s}{t}")` in a loop reallocates each time. `collect::<String>()` and `join` for sequences.
- `to_owned()`, `String::from` and `to_string()` are equivalent for `&str` (std specialises `to_string` for `str`); pick one spelling per codebase.

```rust
fn truncate_bytes(s: &str, max: usize) -> &str {
    // GOOD: never split a code point.
    if s.len() <= max {
        return s;
    }
    let mut end = max;
    while !s.is_char_boundary(end) {
        end -= 1;
    }
    &s[..end]
}

fn main() {
    let s = "naïve café";
    assert_eq!(s.len(), 12); // bytes
    assert_eq!(s.chars().count(), 10); // scalar values
    assert_eq!(truncate_bytes(s, 3), "na"); // byte 3 is inside 'ï'
    assert_eq!("ß".to_uppercase(), "SS");
    assert!("Content-Type".eq_ignore_ascii_case("content-type"));

    let bytes = vec![0x66, 0x6f, 0xff];
    assert!(String::from_utf8(bytes.clone()).is_err());
    assert_eq!(String::from_utf8_lossy(&bytes), "fo\u{FFFD}");
}
```

##### OS strings and paths

- File names, environment variables and command-line arguments are **not** guaranteed UTF-8. Use `OsStr`/`OsString`, `Path`/`PathBuf`, `std::env::args_os()`, `std::env::var_os()`.
- `std::env::args()` **panics** on a non-UTF-8 argument; `env::var()` returns `Err(NotUnicode)`.
- `path.display()` and `to_string_lossy()` are for **display only**; converting a path to `String` and back corrupts non-UTF-8 names.
- Join with `Path::join`, not `format!("{dir}/{file}")`. `join` with an **absolute** second argument replaces the base: `Path::new("/srv/data").join("/etc/passwd") == "/etc/passwd"` — a path-traversal hole when the argument is user input.
- `Path::starts_with` compares components, not strings, and does not resolve `..`. Canonicalise (`fs::canonicalize`) or reject `..`/absolute components before checking containment.

```rust
use std::path::{Component, Path, PathBuf};

/// Joins an untrusted relative path under `base`, rejecting traversal.
fn safe_join(base: &Path, untrusted: &str) -> Option<PathBuf> {
    let rel = Path::new(untrusted);
    if rel.components().any(|c| !matches!(c, Component::Normal(_) | Component::CurDir)) {
        return None; // rejects "..", "/", "C:\" prefixes
    }
    Some(base.join(rel))
}

fn main() {
    let base = Path::new("/srv/data");
    assert_eq!(base.join("/etc/passwd"), Path::new("/etc/passwd")); // BAD pattern
    assert_eq!(safe_join(base, "a/b.txt"), Some(PathBuf::from("/srv/data/a/b.txt")));
    assert_eq!(safe_join(base, "../etc/passwd"), None);
    assert_eq!(safe_join(base, "/etc/passwd"), None);
}
```

Symlinks can still escape a checked directory (TOCTOU). For untrusted archives or uploads use an `openat`-style API (`cap-std`) or canonicalise after creation and re-check.

***

#### Integers, Overflow, Casts and Floats

##### Overflow

- Arithmetic overflow **panics in debug builds** (`overflow-checks = true` in the `dev` and `test` profiles) and **wraps silently in release** (`overflow-checks = false` by default). Tests therefore catch overflow that production silently wraps.
- Make intent explicit with `checked_*` (→ `Option`), `saturating_*`, `wrapping_*`, `overflowing_*`, the `Wrapping<T>`/`Saturating<T>` (1.74) types, or `strict_*` (1.91, panics in every profile). Security-relevant size computations (buffer lengths, offsets, allocation sizes, money) should use `checked_*` or enable `overflow-checks = true` in `[profile.release]`.
- `abs()` on `i32::MIN`, `-x` on `MIN`, `MIN / -1`, and shifts by ≥ bit width overflow too. `unsigned_abs()` and `checked_neg()` avoid them.
- `usize` is 32-bit on 32-bit targets (wasm32, armv7); `u64` → `usize` conversions need `try_from` in portable code.

```rust
fn total_len(a: usize, b: usize) -> Option<usize> {
    a.checked_add(b) // GOOD: None instead of wrap/panic
}

fn main() {
    assert_eq!(total_len(usize::MAX, 1), None);
    assert_eq!(250u8.saturating_add(10), 255);
    assert_eq!(250u8.wrapping_add(10), 4);
    assert_eq!(250u8.overflowing_add(10), (4, true));
    assert_eq!(i32::MIN.checked_abs(), None);
    assert_eq!(i32::MIN.unsigned_abs(), 2_147_483_648u32);
    assert_eq!(1u32.checked_shl(32), None);
    assert_eq!(7u32.div_ceil(2), 4); // unsigned div_ceil: 1.73
}
```

##### `as` casts

`as` never fails and never panics, which makes it dangerous:

| Cast | Behaviour |
|---|---|
| Wider int → narrower int (`300i32 as u8`) | truncates (keeps low bits): `44` |
| Signed ↔ unsigned same width (`-1i32 as u32`) | reinterprets bits: `4294967295` |
| Float → int (`1e10 as i32`, `-1.0 as u8`) | **saturates** (since 1.45); `NaN as i32 == 0` |
| Int → float (`u64::MAX as f32`) | rounds to nearest |
| `f64` → `f32` | rounds; overflow becomes `inf` |
| `bool`/`char` → int | `true as u8 == 1`, `'é' as u8 == 233` (truncates the scalar value) |
| Enum → int | discriminant |
| Pointer ↔ integer | exposes provenance (see unsafe section) |

```rust
use std::convert::TryFrom;

fn main() {
    let len: i64 = -1;
    // BAD: silently becomes a huge number.
    assert_eq!(len as usize, usize::MAX);
    // GOOD: fallible conversion.
    assert!(usize::try_from(len).is_err());
    assert_eq!(u8::try_from(300i32).ok(), None);
    assert_eq!(300i32 as u8, 44);
    assert_eq!(1e10_f64 as i32, i32::MAX);
    assert_eq!(f64::NAN as i32, 0);
    assert_eq!('é' as u8, 233);
    // Lossless widening: prefer From, which documents that it cannot fail.
    let wide: u64 = u64::from(7u32);
    assert_eq!(wide, 7);
}
```

- Every narrowing `as` on externally influenced values (lengths from the network, file offsets, user IDs) is a finding: use `try_from`/`try_into`. Clippy's pedantic `cast_possible_truncation`, `cast_sign_loss`, `cast_possible_wrap` and `cast_lossless` lints locate them.
- `use std::convert::TryFrom;` is only needed on edition 2015/2018; edition 2021 adds `TryFrom`/`TryInto` to the prelude.
- `cast_signed()`/`cast_unsigned()` (1.87) express a deliberate same-width reinterpretation.

##### Floats

- `f32`/`f64` are `PartialOrd`, not `Ord`; `NaN != NaN`. Sort floats with `sort_by(|a, b| a.total_cmp(b))` (`total_cmp`, 1.62), not `partial_cmp(b).unwrap()` (panics on NaN, and since 1.81 the sort itself may panic on an inconsistent comparator).
- Never compare computed floats with `==` (Clippy `float_cmp`, pedantic); compare against a tolerance appropriate to the magnitude.
- Never use floats for money; use integer minor units or a decimal crate.
- `0.1 + 0.2 != 0.3`; `(x as i64)` truncates toward zero; use `round()`, `floor()` explicitly.

***

#### Unsafe Code and Soundness

##### What `unsafe` means

`unsafe` unlocks a small set of operations: dereferencing raw pointers, calling `unsafe` functions (including FFI), accessing `static mut` and `extern` statics, reading `union` fields, and (with `unsafe impl`) implementing `unsafe` traits (`Send`, `Sync`, `GlobalAlloc`). The compiler still checks everything else; the programmer promises the unchecked preconditions.

A function or module is **sound** if no safe caller can cause undefined behaviour (UB) through it. An unsound safe API is a critical finding even if current callers use it correctly.

##### Undefined behaviour categories (Rust Reference)[^7]

- **Data races** (unsynchronised concurrent access where at least one is a write).
- **Dangling or misaligned access**: reading/writing through a pointer that is dangling, null, or not aligned for the type.
- **Out-of-bounds pointer arithmetic** (`ptr.add(n)` leaving the allocation, except one-past-the-end).
- **Aliasing violations**: mutating through or alongside a live `&T` (outside `UnsafeCell`), or any access to memory while an unrelated `&mut T` to it is live. `Box<T>` is treated like a unique `&mut`.
- **Mutating immutable data**: string literals, `const`-promoted values, data behind `&T`, non-`mut` statics.
- **Invalid values**: a `bool` other than 0/1, a `char` outside the scalar-value range, an invalid enum discriminant, a null or dangling reference/`Box`, uninitialised integers, floats or raw pointers (immediate UB since 1.65), a `str` that is not UTF-8 where library invariants require it, a wide pointer with invalid metadata.
- **Wrong call ABI** or unwinding across a frame that does not allow it (calling a `"C-unwind"` function as `"C"`).
- Executing code compiled with **target features** the CPU does not support.
- Incorrect **inline assembly**; violating runtime assumptions (e.g. `longjmp` over frames with destructors).

UB is not "may crash": the optimiser assumes it never happens and may miscompile unrelated code.

##### Rules for reviewing `unsafe`

1. **Every `unsafe` block has a `// SAFETY:` comment** that names each precondition of the operation and why it holds here (Clippy `undocumented_unsafe_blocks`, restriction). Every `unsafe fn` and `unsafe trait` has a `# Safety` doc section (Clippy `missing_safety_doc`, style).
2. **Keep blocks minimal**: one operation per block. In edition 2024 the body of an `unsafe fn` is no longer an implicit `unsafe` block (`unsafe_op_in_unsafe_fn` warns); enable `#![deny(unsafe_op_in_unsafe_fn)]` in older editions.
3. **Encapsulate**: unsafe code establishes invariants that private fields protect. Any safe function in the same module that can break the invariant (e.g. setting `len` beyond initialised elements) is part of the unsafe surface.
4. **Prefer safe alternatives**: `split_at_mut`, `get_disjoint_mut` (1.86), `bytemuck`/`zerocopy` instead of `transmute`, `MaybeUninit` instead of `mem::zeroed`, `std::ptr::NonNull`, `Cell`/atomics instead of `static mut`.
5. **`#![forbid(unsafe_code)]`** at the crate root of crates that need no `unsafe`; `#![deny(unsafe_code)]` with targeted `#[allow]` otherwise. New `unsafe` in a PR needs explicit justification.
6. Run tests under **Miri** (`cargo +nightly miri test`) for crates with unsafe code; it detects most UB categories above, including Stacked/Tree Borrows aliasing violations.

```rust
/// Returns the first and last element mutably.
fn first_last_mut<T>(v: &mut [T]) -> Option<(&mut T, &mut T)> {
    if v.len() < 2 {
        return None;
    }
    // GOOD: safe API instead of two raw-pointer derefs.
    let (head, tail) = v.split_at_mut(1);
    Some((&mut head[0], tail.last_mut()?))
}

/// Reads a little-endian u32 from the start of `buf`.
fn read_u32_le(buf: &[u8]) -> Option<u32> {
    // GOOD: no unsafe needed; first_chunk (1.77) returns Option<&[u8; 4]>.
    buf.first_chunk::<4>().map(|b| u32::from_le_bytes(*b))
}

/// # Safety
/// `idx` must be less than `v.len()`.
unsafe fn get_fast(v: &[u32], idx: usize) -> u32 {
    // SAFETY: the caller guarantees `idx < v.len()` (see `# Safety`).
    unsafe { *v.get_unchecked(idx) }
}

fn main() {
    let mut v = [1, 2, 3];
    if let Some((a, b)) = first_last_mut(&mut v) {
        std::mem::swap(a, b);
    }
    assert_eq!(v, [3, 2, 1]);
    assert_eq!(read_u32_le(&[1, 0, 0, 0, 9]), Some(1));
    assert_eq!(read_u32_le(&[1, 0]), None);
    let data = [10, 20];
    // SAFETY: 1 < data.len().
    assert_eq!(unsafe { get_fast(&data, 1) }, 20);
}
```

##### Common unsound patterns

```rust,compile_fail
// BAD: casting & to &mut. Rejected by the deny-by-default
// `invalid_reference_casting` lint (1.73); UB even when the lint is silenced.
fn bump(x: &u32) {
    let p = x as *const u32 as *mut u32;
    unsafe { *p += 1 };
}
fn main() {}
```

- `mem::transmute` between types with different layout or validity (`u8` → `bool`, `&T` → `&mut T`, `Vec<T>` → `Vec<U>`), or to extend a lifetime. Prefer `from_ne_bytes`, `bytemuck::cast`, `zerocopy`, `ptr::cast` with documented layout.
- `#[repr(Rust)]` structs have **unspecified layout**; any transmute or pointer cast relying on field order needs `#[repr(C)]` or `#[repr(transparent)]`.
- `Vec::set_len` beyond initialised elements; `slice::from_raw_parts` with a null pointer (use `NonNull::dangling()` for empty slices), an unaligned pointer, or a length over `isize::MAX` bytes.
- `MaybeUninit::assume_init` before every byte that the type requires is initialised; `mem::zeroed::<&T>()` or zeroed `NonNull`/`Box` (null is invalid). `mem::uninitialized` is always wrong.
- Returning `&mut` to the same data twice from `&self` via `UnsafeCell` without runtime exclusion.
- `unsafe impl Send/Sync` on a type containing `Rc`, raw pointers or `Cell` without reasoning about every method; this is how most real-world unsoundness enters.
- `static mut`: references to it are denied in edition 2024 (`static_mut_refs`, warn since 1.77 in older editions). Use atomics, `Mutex`, `OnceLock`, or `&raw const`/`&raw mut` (1.82) for FFI-owned globals.
- Raw pointers obtained from a reference keep that reference's permissions: a pointer derived from `&x` must not be used to write.
- Pointer-integer round trips: `ptr as usize as *const T` loses provenance under the strict-provenance model; use `ptr.addr()`, `ptr.with_addr()`, `ptr::without_provenance` (1.84), or `expose_provenance`/`with_exposed_provenance` when interacting with integer-address APIs.
- `#[repr(packed)]` fields: taking a reference to an unaligned field is an error; read with `ptr::read_unaligned(&raw const s.field)`.
- Panics inside unsafe code that leave a data structure half-updated (exception safety): if a user-supplied closure or `Clone`/`Drop` can panic mid-operation, the invariants must hold at every panic point (use guard objects that fix up on drop).

```rust
#[repr(C, packed)]
struct Header {
    tag: u8,
    len: u32,
}

fn main() {
    let h = Header { tag: 1, len: 0x0102_0304 };
    // &h.len would be an unaligned reference (compile error E0793).
    // GOOD: copy the field out, or read through a raw pointer (1.82 syntax).
    let len = h.len;
    let len2 = unsafe { std::ptr::read_unaligned(&raw const h.len) };
    assert_eq!(len, len2);
    assert_eq!(h.tag, 1);
}
```

##### Edition-dependent unsafe syntax

| Construct | Edition 2015–2021 | Edition 2024 (rustc ≥ 1.85) |
|---|---|---|
| `extern "C" { fn f(); }` | accepted (optional `unsafe extern`, 1.82) | must be `unsafe extern "C" { … }`; items may be marked `safe fn` |
| `#[no_mangle]`, `#[export_name]`, `#[link_section]` | accepted (optional `#[unsafe(no_mangle)]`, 1.82) | must be `#[unsafe(no_mangle)]` |
| Unsafe ops in `unsafe fn` body | allowed without a block | `unsafe_op_in_unsafe_fn` warns |
| `&STATIC_MUT` / `&mut STATIC_MUT` | `static_mut_refs` warns (1.77+) | denied by default |
| `std::env::set_var`/`remove_var` | safe to call (unsound with concurrent `getenv`) | `unsafe fn` |

`std::env::set_var` is unsafe in edition 2024 because other threads (or C code) may read the environment concurrently; on older editions the call compiles but has the same race. Set environment variables for child processes with `Command::env`, not `set_var`.

***

#### FFI

- Declare foreign functions in `unsafe extern "C"` blocks (required form in 2024; available since 1.82) and wrap each in a safe Rust function that checks every precondition. Do not expose raw `extern` functions as the public API.
- Use the C types from `core::ffi` (1.64; `c_int`, `c_char`, `c_void`, `CStr`), or `libc`. `c_char` is `i8` on x86_64 Linux but `u8` on AArch64 Linux; never hard-code it.
- Types passed by value or pointer must be `#[repr(C)]` (or `#[repr(transparent)]` wrappers). `bool` maps to C `_Bool`; Rust `enum`s without `#[repr(C)]`/`#[repr(u8)]` must not cross FFI, and a C value that is not a valid discriminant is UB — receive integers and convert with `TryFrom`.
- `Option<&T>`, `Option<NonNull<T>>`, `Option<Box<T>>` and `Option<extern "C" fn(..)>` are guaranteed to be pointer-sized with `None` as null; plain `&T` must never be null.
- Strings: Rust → C needs a NUL-terminated `CString`/`&CStr` (`CString::new` fails on interior NUL). C → Rust: `CStr::from_ptr` (unsafe; pointer must be valid and NUL-terminated for the lifetime used), then `to_str()` (fallible) or `to_string_lossy()`. C-string literals `c"…"` (1.77, edition 2021+) produce `&'static CStr`.
- **Lifetime of `CString`**: `CString::new(s).unwrap().as_ptr()` creates a temporary that is dropped at the end of the statement, leaving a dangling pointer (the `dangling_pointers_from_temporaries` lint warns).
- Memory ownership crosses the boundary explicitly: memory allocated by Rust is freed by Rust (export a `free` function using `Box::from_raw`/`CString::from_raw`); memory from C is freed with the C allocator.
- **Unwinding**: a Rust panic must not unwind into C. Since 1.81 a panic escaping `extern "C"` aborts; to unwind deliberately use `extern "C-unwind"` (1.71) on both sides. For callbacks, catch panics and return an error code.
- Callbacks that receive a `*mut c_void` user-data pointer must restore the exact type it was created from.
- Generate bindings with `bindgen` (C → Rust) or `cbindgen` (Rust → C) rather than writing them by hand; check them in or generate in `build.rs`.
- C-variadic **definitions** (`unsafe extern "C" fn f(mut args: ...)`, read with `args.next_arg::<T>()`) are stable only from 1.99 (any edition); declaring and calling C variadics (`printf`) has always been possible.

```rust
use std::ffi::{c_char, c_int, CStr, CString};

unsafe extern "C" {
    // Declared `safe`: calling it cannot cause UB for any argument (edition 2024 syntax,
    // also accepted in older editions since 1.82).
    safe fn abs(x: c_int) -> c_int;
    fn strlen(s: *const c_char) -> usize;
}

fn c_len(s: &CStr) -> usize {
    // SAFETY: `s` is a valid NUL-terminated string that outlives the call.
    unsafe { strlen(s.as_ptr()) }
}

fn main() {
    assert_eq!(abs(-3), 3);
    let owned = CString::new("hello").expect("no interior NUL");
    assert_eq!(c_len(&owned), 5); // `owned` outlives the pointer use
    assert_eq!(c_len(c"hi"), 2);
    assert!(CString::new("a\0b").is_err());
}
```

```rust
use std::ffi::c_int;
use std::panic::catch_unwind;

/// Exported callback: never lets a panic cross the C boundary.
#[unsafe(no_mangle)]
pub extern "C" fn plugin_compute(x: c_int) -> c_int {
    // Report a panic as an error code instead of unwinding into C.
    catch_unwind(|| compute(x)).unwrap_or(-1)
}

fn compute(x: c_int) -> c_int {
    x.checked_mul(2).expect("overflow")
}

fn main() {
    assert_eq!(plugin_compute(21), 42);
    // The panic message is printed by the default hook; the caller sees -1.
    assert_eq!(plugin_compute(c_int::MAX), -1);
}
```

***

#### Concurrency: Threads, Locks and Atomics

##### `Send` and `Sync`

- `T: Send` — ownership can move to another thread. `T: Sync` — `&T` can be shared between threads (`T: Sync` ⇔ `&T: Send`).
- Auto traits: a struct is `Send`/`Sync` when all fields are. `Rc<T>`, `Cell`/`RefCell` (not `Sync`), raw pointers and `MutexGuard` (not `Send`) opt out.
- `unsafe impl Send/Sync for X {}` asserts thread safety the compiler could not prove; review it like any `unsafe` block, with a `// SAFETY:` comment.
- The compiler guarantees freedom from **data races** in safe code; it does not prevent **race conditions** (check-then-act), deadlocks, or starvation.

##### Spawning and joining

```rust
use std::sync::{Arc, Mutex};
use std::thread;

fn main() {
    // Scoped threads (1.63) may borrow from the enclosing stack frame and are
    // joined automatically at the end of the scope.
    let data = [1, 2, 3, 4];
    let mut sums = [0; 2];
    thread::scope(|s| {
        for (chunk, out) in data.chunks(2).zip(sums.iter_mut()) {
            s.spawn(move || *out = chunk.iter().sum());
        }
    });
    assert_eq!(sums, [3, 7]);

    // 'static threads need owned data; clone the Arc before `move`.
    let counter = Arc::new(Mutex::new(0));
    let handles: Vec<_> = (0..4)
        .map(|_| {
            let counter = Arc::clone(&counter);
            thread::spawn(move || {
                *counter.lock().unwrap() += 1;
            })
        })
        .collect();
    for h in handles {
        h.join().expect("worker panicked"); // do not ignore join results
    }
    assert_eq!(*counter.lock().unwrap(), 4);
}
```

- Detached `thread::spawn` without keeping the `JoinHandle` loses panics and lets work be cut off at process exit (`main` returning kills other threads).
- Size thread pools from `thread::available_parallelism()` (1.59; respects cgroup quotas since 1.61/1.64), not a hard-coded number; for data parallelism use `rayon`.
- Blocking I/O or `thread::sleep` inside a `rayon` closure starves the pool.

##### Mutex, RwLock and poisoning

- `std::sync::Mutex::lock()` returns `Err(PoisonError)` if a thread panicked while holding the lock. `lock().unwrap()` propagates that panic, which is the usual and acceptable choice; to recover, use `lock().unwrap_or_else(PoisonError::into_inner)` and re-validate the data, and `Mutex::clear_poison` (1.77). `parking_lot` locks do not poison.
- Keep critical sections short. Do not do I/O, `await`, logging with blocking sinks, or call unknown callbacks while holding a lock.
- **Guard lifetime surprises**: in `match m.lock().unwrap().get(..) { … }` and `if let Some(x) = m.lock().unwrap().pop() { … } else { … }` the guard (a temporary in the scrutinee) lives to the end of the whole `match` / `if let` (the `else` branch included) in editions ≤ 2021. Edition 2024 drops `if let` scrutinee temporaries before `else`, but `match` scrutinee temporaries still live for the whole `match`. Bind the value first: `let item = m.lock().unwrap().pop();`.
- `let _ = mutex.lock();` drops the guard immediately (the lock protects nothing); `let _guard = mutex.lock();` holds it to the end of the scope. Rustc's `let_underscore_lock` (deny-by-default, 1.65) rejects the first form.
- Lock ordering: acquire multiple locks in one global order everywhere; otherwise two threads deadlock. Re-locking a `std::sync::Mutex` on the same thread deadlocks or panics (unspecified).
- `RwLock` fairness is platform-dependent: a steady stream of readers can starve writers, and taking a read lock while the same thread already holds one may deadlock (if a writer is waiting) or panic. Use `RwLock` only for long, read-dominated sections; `Mutex` is usually faster.
- `RwLockWriteGuard::downgrade` (1.92) atomically turns a write lock into a read lock.
- `Condvar::wait` must be in a loop that re-checks the predicate (spurious wakeups); use `wait_while`.

```rust
use std::sync::{Mutex, PoisonError};

fn pop_item(queue: &Mutex<Vec<u32>>) -> Option<u32> {
    // GOOD: the guard is a temporary of this statement only.
    let item = queue.lock().unwrap_or_else(PoisonError::into_inner).pop();
    // ... slow work with `item` happens without holding the lock
    item
}

fn main() {
    let q = Mutex::new(vec![1, 2]);
    assert_eq!(pop_item(&q), Some(2));
    assert_eq!(pop_item(&q), Some(1));
    assert_eq!(pop_item(&q), None);
}
```

##### Atomics and memory ordering

| Ordering | Use |
|---|---|
| `Relaxed` | Counters and statistics where no other memory depends on the value |
| `Release` (store) / `Acquire` (load) | Publishing data: writes before the `Release` store are visible after an `Acquire` load that sees it |
| `AcqRel` | Read-modify-write that both consumes and publishes (`fetch_add` in a refcount drop path, `compare_exchange` in lock-free structures) |
| `SeqCst` | Single total order across all `SeqCst` operations; needed only when several atomics must be observed in a consistent global order |

```rust
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::Arc;
use std::thread;

static REQUESTS: AtomicU64 = AtomicU64::new(0);

fn main() {
    REQUESTS.fetch_add(1, Ordering::Relaxed); // statistics only

    // Publish data with Release, observe with Acquire.
    let ready = Arc::new(AtomicBool::new(false));
    let data = Arc::new(AtomicU64::new(0));
    let (r, d) = (Arc::clone(&ready), Arc::clone(&data));
    let t = thread::spawn(move || {
        d.store(42, Ordering::Relaxed);
        r.store(true, Ordering::Release);
    });
    while !ready.load(Ordering::Acquire) {
        std::hint::spin_loop();
    }
    assert_eq!(data.load(Ordering::Relaxed), 42);
    t.join().unwrap();
    assert_eq!(REQUESTS.load(Ordering::Relaxed), 1);
}
```

Review points:

- `Relaxed` on a flag that guards other data ("ready", "initialised", "shutdown + read buffer") is a bug; use `Release`/`Acquire`.
- `load` followed by `store` is not atomic; use `fetch_add`, `compare_exchange` (in a loop, or `compare_exchange_weak` in a loop), `fetch_update`, or `update`/`try_update` (1.95).
- `compare_exchange` loops on pointers are subject to ABA; use epoch reclamation (`crossbeam-epoch`) or hazard pointers rather than inventing memory reclamation.
- Spin loops without back-off burn CPU; use `std::sync` primitives or `parking_lot` unless the critical section is a few instructions.
- `SeqCst` everywhere "to be safe" hides an unexamined design; it is not a fix for missing synchronisation of non-atomic data.
- Test lock-free code with `loom` (exhaustive interleavings) and Miri.

##### Channels

- `std::sync::mpsc` is multi-producer single-consumer (crossbeam-based since 1.67); `crossbeam-channel` or `flume` for MPMC and `select!`.
- Unbounded channels (`mpsc::channel`) grow without limit when the consumer is slower; use `sync_channel(n)`/bounded channels for back-pressure.
- `recv()` returns `Err` when all senders are dropped — that is the shutdown signal; a sender kept alive by accident (e.g. the original `tx` in `main` after cloning it for workers) makes the receiver wait forever.

##### Global state and initialisation

```rust
use std::collections::HashMap;
use std::sync::{LazyLock, OnceLock};

// LazyLock (1.80): initialised on first access, thread-safe.
static DEFAULTS: LazyLock<HashMap<&'static str, u32>> =
    LazyLock::new(|| HashMap::from([("timeout", 30), ("retries", 3)]));

// OnceLock (1.70): set once at runtime (e.g. from configuration).
static API_BASE: OnceLock<String> = OnceLock::new();

fn api_base() -> &'static str {
    API_BASE.get().map(String::as_str).unwrap_or("https://localhost")
}

fn main() {
    assert_eq!(DEFAULTS["retries"], 3);
    assert_eq!(api_base(), "https://localhost");
    API_BASE.set("https://api.example.test".into()).unwrap();
    assert!(API_BASE.set("again".into()).is_err()); // only the first set wins
    assert_eq!(api_base(), "https://api.example.test");
}
```

- If the `LazyLock` initialiser panics, the lock is poisoned and every later access panics.
- An initialiser that (indirectly) accesses the same `LazyLock`/`OnceLock` deadlocks.
- MSRV < 1.80: `once_cell::sync::Lazy`; MSRV < 1.70: `once_cell::sync::OnceCell`.

***

#### Async Rust and Tokio

##### Model

- `async fn` returns a `Future` that does nothing until polled. Dropping a future cancels it at its current `.await` point; there is no other cancellation mechanism.
- A runtime (Tokio, the de-facto standard; current release 1.54.1, MSRV 1.85, LTS lines 1.51.x until March 2027 and 1.53.x until September 2027) drives futures on worker threads.[^8] Do not mix runtimes (`async-std` is discontinued; `smol` is an alternative).
- `#[tokio::main]` uses the multi-thread runtime; `#[tokio::main(flavor = "current_thread")]` a single thread; `#[tokio::test]` defaults to `current_thread`.
- Futures that are `.await`ed run **sequentially**; use `tokio::join!`/`try_join!` for concurrency within a task and `tokio::spawn`/`JoinSet` for parallelism across worker threads.

##### Blocking the executor

The worker threads must never block. Each blocking call stalls every task scheduled on that worker.

```rust,ignore
// BAD: blocks a runtime worker thread.
async fn hash_password_bad(pw: String) -> String {
    std::thread::sleep(std::time::Duration::from_millis(50)); // blocking
    let _ = std::fs::read_to_string("/etc/hostname");          // blocking I/O
    expensive_hash(&pw)                                        // CPU-bound
}

// GOOD: async sleep/I/O, and CPU-bound or blocking work on the blocking pool.
async fn hash_password(pw: String) -> anyhow::Result<String> {
    tokio::time::sleep(std::time::Duration::from_millis(50)).await;
    let _ = tokio::fs::read_to_string("/etc/hostname").await;
    Ok(tokio::task::spawn_blocking(move || expensive_hash(&pw)).await?)
}

fn expensive_hash(s: &str) -> String {
    s.chars().rev().collect()
}
```

Blocking calls to flag inside `async` code: `std::thread::sleep`, `std::fs::*`, `std::net::*`, `std::sync::mpsc::Receiver::recv`, `reqwest::blocking`, synchronous database drivers, `Mutex::lock` on contended locks held across long work, heavy loops (JSON of megabytes, compression, crypto), and `block_on` (calling `Runtime::block_on` or `Handle::block_on` from inside the runtime panics).

##### `Send` futures and locks across `.await`

`tokio::spawn` requires `F: Future + Send + 'static`. A future is `Send` only if everything held across an `.await` is `Send`.

```rust,ignore
use std::sync::{Arc, Mutex};

async fn update_bad(state: Arc<Mutex<Vec<u32>>>) {
    let mut guard = state.lock().unwrap();
    guard.push(1);
    some_io().await; // BAD: std MutexGuard held across .await -> future is !Send
    guard.push(2);   // and the lock blocks other tasks for the whole I/O
}

async fn update_good(state: Arc<Mutex<Vec<u32>>>) {
    {
        let mut guard = state.lock().unwrap();
        guard.push(1);
    } // guard dropped before .await
    some_io().await;
    state.lock().unwrap().push(2);
}

async fn some_io() {
    tokio::task::yield_now().await;
}
```

- Clippy's `await_holding_lock` (suspicious, warn) and `await_holding_refcell_ref` flag guards held across `.await`.
- Use `std::sync::Mutex` (or `parking_lot`) for short critical sections that never span `.await`; use `tokio::sync::Mutex` only when the lock must be held across `.await` (it is slower, but its guard can be held across `.await` inside a spawnable future).
- `Rc`, `RefCell` borrows and `!Send` guards across `.await` make the future `!Send`; the compiler error points at `tokio::spawn`, not at the culprit — look for the value "held across an await point".
- `drop(guard)` before `.await` is **not** enough: rustc 1.99.1 still reports the `async fn` future as `!Send` because the guard's scope extends over the `.await`. Put the guard in its own block (as in `update_good`), which works on every version.

##### Cancellation safety

Dropping a future at an `.await` loses whatever it had done so far. This matters in `tokio::select!`, `tokio::time::timeout`, and any code that drops futures.

```rust,ignore
use tokio::sync::mpsc;

async fn pump(
    mut rx: mpsc::Receiver<String>,
    mut lines: tokio::io::Lines<tokio::io::BufReader<tokio::io::Stdin>>,
    mut shutdown: tokio::sync::watch::Receiver<bool>,
) {
    loop {
        tokio::select! {
            // mpsc::Receiver::recv is cancel-safe: nothing is lost if another branch wins.
            Some(msg) = rx.recv() => println!("msg {msg}"),
            // Lines::next_line is cancel-safe as documented by Tokio.
            Ok(Some(line)) = lines.next_line() => println!("line {line}"),
            _ = shutdown.changed() => break,
        }
    }
}
```

- Check the "Cancel safety" section of each method used in a `select!` branch. Not cancel-safe in Tokio: `AsyncReadExt::read_exact`, `read_to_end`, `read_to_string`, `AsyncWriteExt::write_all`, `tokio::sync::Mutex::lock` (loses queue position), `Semaphore::acquire` (loses queue position), and any hand-written `async fn` that awaits more than once while holding partial state.[^9]
- Re-creating a non-cancel-safe future on every loop iteration of `select!` silently loses data. Create it once outside the loop, pin it (`tokio::pin!(fut)` or `std::pin::pin!`, 1.68), and poll `&mut fut` in the branch.
- `select!` picks a random ready branch by default; add `biased;` for priority (shutdown first).
- `select!` branches whose pattern does not match (`Some(msg) = rx.recv()` when the channel is closed) are disabled; add an `else => break` branch or the loop can spin or panic when every branch is disabled.
- `tokio::time::timeout(d, fut)` drops `fut` on timeout — the same cancellation rules apply.
- Tasks spawned with `tokio::spawn` are **not** cancelled when the `JoinHandle` is dropped; they keep running detached. Use `JoinSet` (aborts all on drop), `handle.abort()`, or a `CancellationToken` (`tokio-util`) for structured shutdown.
- Graceful shutdown: listen for `tokio::signal::ctrl_c()` / SIGTERM, stop accepting work, signal tasks via `CancellationToken` or `watch`, then await them with a timeout.

##### Other async review points

- An `async fn` that never `.await`s anything blocking still runs to completion without yielding; long CPU loops in async code need `spawn_blocking`, `rayon`, or `tokio::task::yield_now().await` checkpoints.
- Unbounded `tokio::sync::mpsc::unbounded_channel` and spawning a task per incoming message without a limit are memory DoS vectors; use bounded channels and a `Semaphore` to cap concurrency.
- `async fn` in public traits (1.75): the returned future's `Send`-ness is not part of the signature; generic callers that spawn need `trait Service { fn call(&self) -> impl Future<Output = ()> + Send; }` or the `trait-variant` crate. The `async_fn_in_trait` lint warns about this on public traits.
- Async closures (`async |x| …`, 1.85) can borrow from their captures across `.await`, which closures returning `async move {}` cannot; accept them with `impl AsyncFn(T) -> U`.
- Library code must not create or block on runtimes: `Runtime::new().block_on(..)` or `Handle::block_on(..)` inside a Tokio runtime panics ("Cannot start a runtime from within a runtime"), and `futures::executor::block_on` stalls the worker thread and can deadlock. Libraries expose `async` APIs (or blocking ones) and let the application own the runtime.
- Recursion in `async fn` needs indirection (`Box::pin`) since 1.77; before 1.77 use the `async-recursion` crate.
- `tokio::spawn` in a library requires the caller to run inside a Tokio runtime; `Handle::try_current()` tells you if one exists.

***

#### Public API Design and Semver

Cargo assumes SemVer: for `1.x.y`, any `1.*` release must be compatible; for `0.x.y`, the **minor** number is the compatibility boundary (`0.4.1` → `0.5.0` may break). The Cargo book's SemVer chapter lists what is breaking.[^10]

| Change to a published library | Classification |
|---|---|
| Removing/renaming a public item, changing a function signature, adding a required argument | **major** |
| Adding a public field to a struct whose fields are all public (breaks struct literals and exhaustive patterns) | **major** |
| Adding an enum variant to an enum without `#[non_exhaustive]` | **major** |
| Adding `#[non_exhaustive]` to an existing public type | **major** |
| Adding a trait method without a default; adding a non-defaulted associated type/const | **major** |
| Adding a field (even private) that makes a type lose `Send`, `Sync`, `Unpin`, `UnwindSafe` | **major** |
| Tightening generic bounds; changing `repr`/layout callers rely on | **major** |
| Removing a Cargo feature, or moving functionality out of the default features | **major** |
| Bumping a **public** dependency (its types appear in your API) to an incompatible version | **major** |
| Adding a public item, a defaulted trait method, an inherent method | minor (can still cause ambiguity with glob imports or trait methods of the same name) |
| Adding a trait impl for an existing type | minor, possibly breaking (type-inference ambiguity) |
| Raising `rust-version` | minor by convention; document it (Cargo's MSRV-aware resolver keeps older users on older versions) |

Practices:

- Run **`cargo-semver-checks`** (0.51.0, Oct 2026) in CI before publishing; it compares the rustdoc JSON of the new version against the last published one and reports breaking changes such as removed items, newly required trait items, lost auto traits and enum variants added to exhaustive enums.[^11]
- Mark public enums that will grow (`ErrorKind`, config options) and structs with public fields `#[non_exhaustive]` from the first release.
- Keep fields private and expose constructors/builders; accept `impl Into<…>`/`impl AsRef<…>` at the edges, return concrete types.
- Do not expose third-party types in the public API unless you intend to follow their major versions (re-export them, `pub use dep;`, if you do).
- Seal traits that users should call but not implement.
- `#[must_use]` on functions whose result is the point (builders, pure functions, `Result`-like types).
- Deprecate before removing: `#[deprecated(since = "1.4.0", note = "use `foo` instead")]`.
- Every public item has docs, with an example for non-trivial ones; `#![warn(missing_docs)]` in libraries.
- `pub(crate)` for crate-internal items; the `unreachable_pub` lint finds `pub` items that are not actually exported.
- A builder or constructor that validates should return `Result`, not panic.

***

#### Cargo: Manifests, Features, Workspaces, Lockfiles, Build Scripts

##### Manifest checklist

```toml
[package]
name = "acme-ingest"
version = "0.7.0"
edition = "2021"
rust-version = "1.80"
license = "MIT OR Apache-2.0"
description = "Ingestion pipeline"
repository = "https://example.com/acme/ingest"
publish = false                 # internal crates: prevents accidental crates.io publish

[dependencies]
serde = { version = "1.0.200", features = ["derive"] }
# Edition 2021 means resolver 2, which ignores rust-version: `cargo update` picks
# tokio 1.54 (MSRV 1.85) and the MSRV build breaks. See the Resolver section.
tokio = { version = "1.47", default-features = false, features = ["rt-multi-thread", "macros", "net"] }
tracing = "0.1"

[dev-dependencies]
proptest = "1"

[features]
default = []
metrics = ["dep:prometheus"]    # `dep:` (1.60) hides the implicit `prometheus` feature

[dependencies.prometheus]
version = "0.14"
optional = true

[lints.rust]
unsafe_code = "forbid"
missing_docs = "warn"

[lints.clippy]
all = { level = "warn", priority = -1 }
unwrap_used = "warn"

[profile.release]
overflow-checks = true          # keep arithmetic panics in production where it matters
debug = "line-tables-only"      # usable backtraces at small size cost
```

- Version requirements are caret by default: `"1.0.200"` means `>=1.0.200, <2.0.0`. Use the lowest version that actually provides what the code uses; `"*"` is rejected by crates.io.
- Git dependencies without `rev`/`tag` follow a moving branch; path dependencies cannot be published unless they also have a `version`.
- `[patch]` and `[replace]` in a library are ignored by downstream users; they only affect the workspace that declares them.
- `edition` missing → 2015. `rust-version` missing → Clippy and the resolver cannot respect the MSRV.

##### Features

- Features must be **additive**: enabling a feature may add API or functionality, never remove or change it. Cargo unifies features across the whole dependency graph, so if any crate enables `foo/std`, every user of `foo` gets it.
- Mutually exclusive features (`backend-a` vs `backend-b`, `compile_error!` when both are set) break when two dependents choose differently. Use runtime configuration, separate crates, or a single default with an additive opt-in.
- `no_std` support is expressed as an additive `std` feature (`default = ["std"]`), not a `no_std` feature.
- Libraries depend on other libraries with `default-features = false` plus the features they need, so applications can trim defaults.
- `cfg(feature = "…")` names must exist in `[features]`; since 1.80 `unexpected_cfgs` warns about unknown names. Custom `cfg`s set by build scripts must be declared (`cargo::rustc-check-cfg=cfg(has_foo)`, or `[lints.rust] unexpected_cfgs = { level = "warn", check-cfg = ['cfg(has_foo)'] }`).
- Test the feature matrix in CI (`cargo hack --each-feature check` or at least `--no-default-features` and `--all-features`).

##### Resolver

| `resolver` | Default for | Behaviour |
|---|---|---|
| `"1"` | edition 2015/2018 packages, **virtual workspaces without a `resolver` key** | Unifies features across build-dependencies, dev-dependencies and target-specific deps |
| `"2"` | edition 2021 | Does not unify features of build-deps/proc-macros with normal deps, or of dev-deps unless building tests, or of deps for other targets |
| `"3"` (1.84) | edition 2024 | `"2"` plus MSRV-aware version selection (`incompatible-rust-versions = "fallback"`) |

MSRV-aware selection matters in practice: with `edition = "2021"`, `rust-version = "1.80"` and `tokio = "1.47"`, Cargo 1.99 locks tokio 1.54.1 (which requires Rust 1.85); with `[resolver] incompatible-rust-versions = "fallback"` in `.cargo/config.toml` (Cargo ≥ 1.84) it locks 1.53.3 and reports "available: v1.54.1, requires Rust 1.85".

A **virtual workspace** (root `Cargo.toml` with only `[workspace]`) has no edition, so it falls back to resolver `"1"` and Cargo warns. Always set `resolver` explicitly in a virtual workspace root.

##### Workspaces and inheritance (1.64)

```toml
# Root Cargo.toml (virtual workspace)
[workspace]
members = ["crates/*"]
resolver = "3"                  # needs Cargo 1.84; use "2" for older toolchains

[workspace.package]
edition = "2024"
rust-version = "1.85"
license = "MIT OR Apache-2.0"

[workspace.dependencies]
serde = { version = "1.0.200", features = ["derive"] }

[workspace.lints.clippy]
pedantic = { level = "warn", priority = -1 }
```

```toml
# crates/api/Cargo.toml
[package]
name = "api"
version = "0.1.0"
edition.workspace = true
rust-version.workspace = true

[dependencies]
serde = { workspace = true, features = ["rc"] }   # features are additive to the workspace's

[lints]
workspace = true
```

- `default-features = false` on a member's `{ workspace = true }` dependency is **ignored** (with a warning) when the workspace entry enables default features — except for members on edition 2024 with Cargo ≥ 1.99, where it now takes effect. Edition 2024 turns an *unused* inherited `default-features` key into an error.
- A single `Cargo.lock` and `target/` exist at the workspace root.
- Lint groups need `priority = -1` so that individual lints in the same table can override them (Clippy's `lint_groups_priority` lint flags the mistake).

##### Lockfile policy

- `cargo new` creates projects with `Cargo.lock` tracked; the Cargo FAQ (since 2023) recommends deciding per project rather than "binaries commit, libraries don't". Applications and anything deployed **must** commit `Cargo.lock`. Libraries usually commit it too and add a CI job that tests the latest compatible versions (`cargo update` then test) and one that tests minimal versions.[^12]
- `Cargo.lock` does not affect downstream users of a library; only the version requirements in `Cargo.toml` do.
- CI and release builds use `--locked` (fail if the lockfile would change); `cargo install --locked` for tools. Review `Cargo.lock` diffs: a PR that changes many unrelated versions or adds unknown crates deserves scrutiny.
- Lockfile v4 (default since 1.78) cannot be read by Cargo < 1.78; with a lower MSRV, CI on the MSRV toolchain fails unless `rust-version` makes Cargo write an older format.

##### Build scripts and proc macros

- `build.rs` and procedural macros run **arbitrary code at build time** with the developer's and CI's privileges. Treat new ones in dependencies as security-relevant (the arrayref incident of August 2026 used a malicious build script).[^13]
- Build scripts write only to `OUT_DIR`, never to the source tree.
- Emit precise rerun directives (`cargo::rerun-if-changed=path`, `cargo::rerun-if-env-changed=VAR`); without any, the script reruns whenever any file in the package changes. The `cargo::` prefix needs Cargo 1.77; use `cargo:` (single colon) for an older MSRV.
- Prefer `pkg-config`/`cc`/`bindgen` crates over hand-written compiler invocations; respect `TARGET` and `HOST` for cross-compilation.
- Network access in build scripts makes builds non-reproducible and fails in offline/vendored builds.

***

#### Performance and Allocation

- Benchmark and profile **release** builds (`cargo build --release`, `perf`, `cargo flamegraph`, `samply`); debug builds are often 10–100× slower and meaningless for performance claims.
- Reserve capacity when the final size is known (`Vec::with_capacity`, `String::with_capacity`, `extend` from an exact-size iterator). `shrink_to_fit` only for long-lived oversized buffers.
- Avoid repeated allocation in loops: reuse buffers (`buf.clear()`), use `write!` into an existing `String`, pass `&str`/`&[T]`.
- **Buffer I/O**: `File` and `TcpStream` are unbuffered; wrap in `BufReader`/`BufWriter`. `BufWriter` **ignores errors when dropped** — call `flush()` (or `into_inner()`) and handle the result. `println!` locks stdout for each call; for heavy output lock once (`io::stdout().lock()`) and wrap in `BufWriter`.
- `Box<dyn Trait>` and `Arc` clones are cheap but not free; `Rc` is cheaper than `Arc` in single-threaded code.
- `#[inline]` matters for small non-generic functions called across crates without LTO; generic functions are already inlinable. `#[inline(always)]` needs a measurement.
- Release profile knobs: `lto = "thin"` or `"fat"`, `codegen-units = 1`, `panic = "abort"` (behaviour change, see Panics), `opt-level = 3`, PGO via `cargo-pgo`. Each trades compile time.
- Use `std::hint::black_box` (1.66) in benchmarks so the optimiser does not delete the measured work; `#[bench]` is not available on stable — use `criterion` (0.8) or `divan`.
- Large values on the stack (`[u8; 1 << 20]` locals, big structs moved by value) risk stack overflow (the main thread typically gets 8 MiB on Linux via `ulimit -s`; spawned threads get 2 MiB unless `thread::Builder::stack_size` or `RUST_MIN_STACK` says otherwise); box them or use `Vec`.
- `HashMap` with the default SipHash is safe against HashDoS but slower; switching to `FxHash`/`ahash` is acceptable only for keys that are not attacker-controlled.
- `String` formatting (`format!`, `to_string`) in hot paths and logging at disabled levels (with eagerly formatted arguments) are common hidden costs.
- `clone()` of `Vec<String>` / `HashMap` per request; `collect()` just to iterate again; `Vec<Box<T>>` where `Vec<T>` suffices.
- `lld` is the default linker on `x86_64-unknown-linux-gnu` since 1.90, which speeds up linking; custom linker flags written for GNU `ld` may need review.

***

#### Testing

##### Layout

```rust
pub fn parse_kv(s: &str) -> Option<(&str, &str)> {
    let (k, v) = s.split_once('=')?;
    let k = k.trim();
    if k.is_empty() { None } else { Some((k, v.trim())) }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_pair() {
        assert_eq!(parse_kv(" a = 1 "), Some(("a", "1")));
    }

    #[test]
    fn rejects_empty_key() {
        assert_eq!(parse_kv("=1"), None);
    }

    // Tests may return Result and use `?`.
    #[test]
    fn parses_number() -> Result<(), std::num::ParseIntError> {
        let (_, v) = parse_kv("n=42").unwrap();
        assert_eq!(v.parse::<i32>()?, 42);
        Ok(())
    }

    #[test]
    #[should_panic(expected = "attempt to subtract with overflow")]
    fn overflow_panics_in_test_profile() {
        let zero = std::hint::black_box(0u8);
        let _ = zero - 1;
    }
}
```

- **Unit tests** live in `#[cfg(test)] mod tests` beside the code and can test private items.
- **Integration tests** in `tests/*.rs` are separate crates that see only the public API; shared helpers go in `tests/common/mod.rs` (not `tests/common.rs`, which becomes its own test target).
- **Doc tests** (examples in `///` comments) run with `cargo test` for **library** targets only. They are the public examples; keep them compiling. Use `no_run` for examples that need network/files, `compile_fail` for misuse examples, `ignore` sparingly. In edition 2024, doctests are merged into one binary per crate (faster); a doctest that depends on process-global state can opt out with `standalone_crate`.
- `#[should_panic]` without `expected = "…"` passes on *any* panic, including the wrong one.
- Tests run in **parallel threads of one process**: tests that change environment variables, the current directory, or global state race. Isolate (temp dirs via `tempfile`, explicit configuration parameters), serialise with a `Mutex`/`serial_test`, or run with `--test-threads=1`. `std::env::set_var` in tests is `unsafe` in edition 2024 for this reason.
- `#[ignore = "reason"]` for slow/external tests; run them explicitly in CI with `cargo test -- --ignored`.
- `cargo-nextest` runs each test in its own process (isolation, retries) but does not run doctests; keep `cargo test --doc` in CI.
- Snapshot tests (`insta`) for large structured output; review `.snap` diffs like code.

##### Property-based testing

```rust,ignore
use proptest::prelude::*;

fn encode(v: &[u8]) -> String {
    v.iter().map(|b| format!("{b:02x}")).collect()
}

fn decode(s: &str) -> Option<Vec<u8>> {
    // Clippy suggests `is_multiple_of` (1.87) here unless `rust-version` is older.
    if s.len() % 2 != 0 {
        return None;
    }
    (0..s.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(s.get(i..i + 2)?, 16).ok())
        .collect()
}

proptest! {
    #[test]
    fn hex_round_trip(data in proptest::collection::vec(any::<u8>(), 0..256)) {
        prop_assert_eq!(decode(&encode(&data)), Some(data));
    }

    #[test]
    fn decode_never_panics(s in "\\PC*") {
        let _ = decode(&s);
    }
}
```

Property tests (`proptest` 1.12, `quickcheck`) suit parsers, encoders, and anything with an inverse or invariant. Commit the `proptest-regressions/` files they write so found failures are re-run. Fuzz untrusted-input parsers with `cargo fuzz` (libFuzzer; nightly).

##### Miri and loom

- **Miri** interprets the program and reports UB: out-of-bounds and use-after-free, invalid values, uninitialised reads, aliasing violations (Stacked/Tree Borrows), data races, memory leaks, misaligned access. It requires nightly: `rustup +nightly component add miri` then `cargo +nightly miri test`. It is slow; mark tests that do I/O or heavy computation `#[cfg_attr(miri, ignore)]`. Any crate with `unsafe` should run its tests under Miri in CI. Miri used to store all environment variables in `target/`; do not expose secrets to jobs that run Miri and cache `target/` (fixed in nightly 2026-09-22).[^14]
- **loom** (0.7) model-checks concurrent code by exploring all interleavings permitted by the memory model. Code under test uses `loom::sync` types under `cfg(loom)` and runs with `RUSTFLAGS="--cfg loom" cargo test --release`. Use it for lock-free structures and custom synchronisation primitives.
- Sanitizers (`-Zsanitizer=address/thread`) are nightly-only.

##### Coverage and mutation testing

`cargo llvm-cov` for coverage (stable `-C instrument-coverage`), `cargo mutants` to find tests that do not check results.

***

#### Lints: rustc, Clippy, rustfmt

##### rustc lints

- Treat warnings as errors in CI, not in `lib.rs`: `#![deny(warnings)]` in source breaks the crate for downstream users whenever a new compiler adds a warning. Use `RUSTFLAGS="-D warnings"` in CI or Cargo's `build.warnings = "deny"` config (1.97).
- Useful opt-in rustc lints: `unsafe_code`, `unsafe_op_in_unsafe_fn` (default warn in 2024), `missing_docs`, `missing_debug_implementations`, `unreachable_pub`, `unused_qualifications`, `rust_2018_idioms` (group; flags elided lifetimes in paths such as `Formatter` without `<'_>`, and `extern crate`), `let_underscore_drop`, `single_use_lifetimes`.
- `#[expect(lint, reason = "…")]` (1.81) instead of `#[allow]`: it warns (`unfulfilled_lint_expectations`) when the suppression is no longer needed. Before 1.81 use `#[allow(lint)]` with a comment.
- Deny-by-default lints that indicate real bugs: `invalid_reference_casting`, `let_underscore_lock`, `deref_nullptr` (deny since 1.93), `invalid_from_utf8_unchecked`, `unconditional_panic`, `arithmetic_overflow`, `invalid_runtime_symbol_definitions` (1.98), `never_type_fallback_flowing_into_unsafe` (deny since 1.92). Never `allow` them without a detailed reason.

```rust,compile_fail
use std::sync::Mutex;

fn main() {
    let m = Mutex::new(0);
    // Rejected by the deny-by-default `let_underscore_lock` lint:
    // the guard is dropped immediately, so nothing is protected.
    let _ = m.lock();
    *m.lock().unwrap() += 1;
}
```

##### Clippy

| Group | Default | Use |
|---|---|---|
| `clippy::correctness` | **deny** | Outright wrong code — never allow without a reason |
| `clippy::suspicious` | warn | Most likely wrong |
| `clippy::style`, `clippy::complexity`, `clippy::perf` | warn | Idiom, simplification, speed |
| `clippy::pedantic` | allow | Stricter; enable as a group with `priority = -1` and allow the noisy ones |
| `clippy::restriction` | allow | **Never enable as a group**; cherry-pick (`unwrap_used`, `expect_used`, `undocumented_unsafe_blocks`, `indexing_slicing`, `dbg_macro`, `print_stdout`, `clone_on_ref_ptr`) |
| `clippy::nursery` | allow | Unstable lints with false positives |
| `clippy::cargo` | allow | Manifest metadata and duplicate dependency versions |

`clippy::all` covers correctness, suspicious, style, complexity and perf.[^15] Run `cargo clippy --all-targets --all-features -- -D warnings` in CI; `cargo clippy --fix` applies machine-applicable suggestions (same target selection as `cargo build` since 1.89). Configure `msrv` (or rely on `rust-version`) so Clippy does not suggest newer APIs. Clippy suggestions are not authoritative: a reviewer still checks that a suggested rewrite preserves behaviour and the MSRV.

```rust
// Clippy flags each of these (default groups):
fn examples(v: &Vec<i32>, s: &String) -> usize {
    //          ^ ptr_arg: use &[i32]   ^ ptr_arg: use &str
    let mut n = 0;
    for i in 0..v.len() {
        // needless_range_loop
        n += v[i] as usize;
    }
    if s.len() == 0 {
        // len_zero: use is_empty()
        n += 1;
    }
    n
}

fn main() {
    assert_eq!(examples(&vec![1, 2], &String::new()), 4);
}
```

##### rustfmt

- `cargo fmt --check` in CI. Keep `rustfmt.toml` minimal; options marked unstable require nightly rustfmt and are ignored with a warning on stable.
- `style_edition = "2024"` (rustfmt 1.85+) changes import sorting to version sort (`u8` < `u16` < `u32`) and other details; switching style editions reformats many files — do it in a dedicated commit.
- `#[rustfmt::skip]` only for hand-aligned tables/matrices, with a reason.

***

#### Security and Supply Chain

##### Dependencies

- **`cargo audit`** (cargo-audit 0.22, RustSec advisory database) or **`cargo deny check`** (cargo-deny 0.20: advisories, licenses, banned crates, duplicate versions, allowed registries/git sources) in CI on every PR and on a schedule; new advisories appear for unchanged lockfiles.[^16]
- New dependencies deserve review: maintenance status, download count, owners, `unsafe` usage (`cargo geiger`), build scripts and proc macros, transitive tree size (`cargo tree -e normal --duplicates`). Prefer well-known crates for crypto, TLS, parsing.
- Typosquats and compromised accounts are real: the August 20, 2026 incident republished `arrayref` 0.3.10 (and `internment` 0.8.7, `append-only-vec` 0.1.9) depending on a malicious `proc-macro1` crate whose build script downloaded a payload; the versions were online for 86–107 minutes.[^13] `cargo vet`/`cargo crev`, pinning via `Cargo.lock`, and delaying adoption of brand-new versions reduce exposure.
- Publish from CI with crates.io **Trusted Publishing** (OIDC from GitHub Actions; GitLab.com since January 2026) instead of long-lived API tokens; owners can enforce trusted-publishing-only.[^17] Never pass the token as a `cargo login <token>` argument (deprecated since 1.86; it ends up in shell history) — use `cargo login` from stdin or `CARGO_REGISTRY_TOKEN`.
- CI jobs that can write caches shared with PR builds must not have secrets in the environment (see the Miri incident).[^14]

##### Code-level security

- **Command execution**: `std::process::Command::new(program).arg(user_input)` passes arguments without a shell — safe from shell injection. `Command::new("sh").arg("-c").arg(format!("… {user_input}"))` is shell injection. On Windows, `.bat`/`.cmd` argument escaping was fixed in 1.77.2 (CVE-2024-24576); older toolchains must not pass untrusted arguments to batch files.
- **SQL**: bind parameters (`sqlx::query("… WHERE id = $1").bind(id)`, `query!` macros, Diesel); never `format!` user input into SQL.
- **Path traversal**: see [OS strings and paths](#os-strings-and-paths). `fs::remove_dir_all` had a symlink race (CVE-2022-21658) fixed in 1.58.1.
- **Deserialisation**: `serde` is memory-safe, but untrusted input still needs size limits (body size, string/collection lengths, recursion depth). `#[serde(deny_unknown_fields)]` for strict configs. Formats such as `bincode` with untrusted length prefixes can allocate huge buffers — configure limits.
- **Regex**: the `regex` crate guarantees linear-time matching (no catastrophic backtracking); `fancy-regex` (backreferences/lookaround) does not — avoid it on untrusted patterns or input. Compile regexes once (`LazyLock<Regex>`), not per call; untrusted patterns need `RegexBuilder::size_limit`.
- **Secrets**: do not derive `Debug` on structs holding passwords/tokens (they leak into logs and panic messages); implement a redacting `Debug` or use `secrecy::SecretString`. Zero sensitive buffers with `zeroize`. Compare secrets in constant time (`subtle::ConstantTimeEq`, or the comparison function of the crypto library), not with `==`.
- **Randomness**: use `getrandom`, `rand::rngs::SysRng` (rand 0.10; named `OsRng` in rand 0.8/0.9), `rand::rng()`/`thread_rng()` (a CSPRNG seeded from the OS) or the crypto library's RNG for keys, tokens and nonces; never seed a PRNG from time, and never use `SmallRng`/`StdRng::seed_from_u64` for secrets.
- **Crypto and TLS**: use maintained libraries (`rustls`, `ring`, `aws-lc-rs`, RustCrypto crates, `argon2` for passwords); never disable certificate verification (`danger_accept_invalid_certs(true)`) outside tests.
- **Integer handling** in parsers: lengths from input must be checked (`checked_*`, `try_from`) before allocation and slicing; see [Integers](#integers-overflow-casts-and-floats).
- **DoS**: bounded channels, request/body size limits, timeouts on every network call (`tokio::time::timeout`, client timeouts), limits on concurrent tasks, and no `unwrap` on input-derived values in servers (a panic in a request handler may take down the task or the process with `panic = "abort"`).

***

#### Logging and Observability

- Libraries log through a facade (`log` or `tracing`) and never install a logger or subscriber; binaries install exactly one (`tracing_subscriber::fmt().with_env_filter(...)`, `env_logger`).
- Prefer `tracing` for async services: spans carry request context across `.await`. `#[tracing::instrument(skip(password, body), fields(user_id = %id))]` — skip large or secret arguments explicitly; by default `instrument` records **all** arguments with `Debug`.
- Use structured fields (`info!(user_id, elapsed_ms = t.as_millis(), "request done")`) rather than formatting values into the message.
- Do not hold a span guard from `span.enter()` across `.await` (it attaches the wrong span to other tasks); use `.instrument(span)` on the future or `#[instrument]`.
- Log errors once, at the layer that handles them, with the full chain (`{err:#}` for `anyhow`, or iterate `source()`); do not log-and-return the same error at every layer.
- `eprintln!`/`println!`/`dbg!` in library or service code is a finding (Clippy `print_stdout`, `print_stderr`, `dbg_macro`).
- Panics: install a panic hook (or rely on `tracing-panic`) so panics reach the structured log; set `RUST_BACKTRACE=1` (or capture `std::backtrace::Backtrace`, 1.65) for diagnostics. Release builds need `debug = "line-tables-only"` or split debuginfo for readable backtraces; `-C panic=abort` builds get unwind tables (and therefore backtraces) by default since 1.92.
- Metrics (`metrics`, `prometheus`, OpenTelemetry) for rates, latencies and queue depths; label cardinality must be bounded (no user IDs as labels).

***

#### Edition Migration Notes

Migrate one edition at a time: `cargo fix --edition` (applies the migration lints while still on the old edition), bump `edition`, then `cargo build`, `cargo test`, `cargo clippy`, and `cargo fmt`. The automatic fixes are conservative: they preserve behaviour by rewriting code, sometimes into an awkward form that deserves a manual cleanup. Review an edition bump PR for **behaviour** changes the compiler does not flag.

##### 2015 → 2018 (rustc ≥ 1.31)

- Paths: `use crate::module::Item;` for crate-local paths; `extern crate` lines removed (keep `extern crate alloc;` in `no_std` crates).
- `dyn Trait` for trait objects; `async`/`await`/`dyn`/`try` cannot be identifiers (use `r#try`).
- Anonymous trait parameters (`fn f(u8);` in a trait) must be named.

##### 2018 → 2021 (rustc ≥ 1.56)

- **Disjoint closure captures** change drop order and `Send`-ness: a closure that uses `x.field` no longer captures (and therefore no longer keeps alive or drops) all of `x`. The migration inserts `let _ = &x;` where the old behaviour matters; review whether those lines are still needed.
- `array.into_iter()` yields values; code iterating `&T` via `into_iter()` on an array now gets `T`.
- `panic!("{}")` with a single argument is a format string; `panic!(msg_string)` no longer compiles (use `panic!("{msg}")` or `std::panic::panic_any`).
- Cargo resolver 2 becomes the default: build-dependency and dev-dependency features are no longer unified into normal builds — a crate that compiled only because a dev-dependency enabled a feature now fails in `cargo build`.
- Reserved prefixes: `ident"…"`, `ident'…'`, `ident#…` are errors (macros that relied on `z"…"` tokenising as two tokens need a space).
- Prelude adds `TryFrom`, `TryInto`, `FromIterator`; user traits with methods named `try_into`/`try_from`/`from_iter` can become ambiguous.

```rust,edition2021
// Edition 2021: the closure captures only `p.name`, so `p.data` can still be used.
struct Pair {
    name: String,
    data: Vec<u8>,
}

fn main() {
    let mut p = Pair { name: "x".into(), data: vec![1] };
    let print_name = || p.name.len();
    p.data.push(2); // in edition 2018 this is E0502: `p` is borrowed by the closure
    assert_eq!(print_name(), 1);
    assert_eq!(p.data, [1, 2]);
}
```

##### 2021 → 2024 (rustc ≥ 1.85)

- **RPIT lifetime capture**: `fn f(&self) -> impl Iterator<Item = u32>` now captures the `&self` lifetime. Callers that kept the iterator after mutating or dropping `self` stop compiling; add `+ use<>` (or `use<T>`) when the returned value does not borrow.
- **`if let` scrutinee temporaries** drop before the `else` block. Code like `if let Some(x) = lock.lock().unwrap().get(..) { … } else { lock.lock() … }` deadlocked in 2021 and works in 2024; conversely, code that relied on the guard living through `else` changes behaviour.
- **Tail-expression temporaries** drop before the block's locals: `c.borrow().len()` as the last expression now compiles; destructors with side effects (guards, spans, `Drop` logging) run at a different time. The `tail_expr_drop_order` lint lists the affected places during migration.
- **Never-type fallback** becomes `!`: unconstrained diverging expressions no longer default to `()`. Code such as `f()?;` where `f` is generic over its return type, or `panic!()` in a generic position, can fail to infer; the lints `never_type_fallback_flowing_into_unsafe` and `dependency_on_unit_never_type_fallback` (deny-by-default since 1.92) flag it.
- **Unsafe**: `unsafe extern`, `#[unsafe(no_mangle)]`, `unsafe { }` blocks inside `unsafe fn`, `static mut` references denied, `env::set_var`/`remove_var` unsafe.
- **Macros**: `$e:expr` also matches `const { … }` and `_`; macros that dispatch on those tokens after an `expr` arm change behaviour — the migration rewrites such fragments to `expr_2021` (available in all editions since 1.83). Missing fragment specifiers (`$x` without `:kind`) are errors in 2024; since 1.89 they are errors in every edition (1.85 accepted them in 2021).
- **Match ergonomics**: `mut`, `ref`, `ref mut` and `&` patterns are rejected after the default binding mode has switched to by-reference (`let [ref x] = &[1];` is an error; write `let &[ref x] = &[1];` or `let [x] = &[1];`).
- **Keywords and syntax**: `gen` is reserved (`r#gen` for `rand`'s `Rng::gen`, renamed `random` in `rand` 0.9); `#"…"#` and `##` tokens are reserved.
- **Library**: `Box<[T]>::into_iter()` yields values; `Future` and `IntoFuture` are in the prelude (user traits with `poll`/`into_future` methods may become ambiguous).
- **Cargo**: resolver 3 (MSRV-aware), `[project]`, `dev_dependencies`, `default_features` and other underscore spellings removed, unused inherited `default-features = false` rejected (until 1.99, which makes it take effect instead).
- **Rustdoc**: doctests merged into one binary per crate; doctests run when cross-compiling only from 1.89.
- **rustfmt**: style edition 2024 (version sorting of imports).

```rust
use std::cell::RefCell;

// Compiles in edition 2024; in 2021 the `Ref` temporary outlives `c` (E0597).
fn len_of_cell() -> usize {
    let c = RefCell::new("..");
    c.borrow().len()
}

fn main() {
    assert_eq!(len_of_cell(), 2);
}
```

```rust
use std::sync::Mutex;

// Edition 2024: the guard from the scrutinee is dropped before `else`, so the
// second lock() succeeds. In edition 2021 this deadlocks (or panics).
fn take_or_refill(m: &Mutex<Vec<u32>>) -> u32 {
    if let Some(x) = m.lock().unwrap().pop() {
        x
    } else {
        m.lock().unwrap().push(7);
        0
    }
}

fn main() {
    let m = Mutex::new(vec![]);
    assert_eq!(take_or_refill(&m), 0);
    assert_eq!(take_or_refill(&m), 7);
}
```

***

#### Production / Review Checklist

##### Version

- [ ] `edition`, `rust-version`, `rust-toolchain.toml` read before judging syntax/API availability; package has an explicit `edition`
- [ ] New APIs/syntax are available in the declared `rust-version` (raise it in the same change if not)
- [ ] Edition-2024-only constructs (let chains, `use<>` defaults, unsafe attribute requirements) not suggested for older editions
- [ ] CI uses the latest patch of the pinned toolchain and tests the MSRV

##### Correctness

- [ ] No `unwrap`/`expect`/indexing/slicing/division that input can make panic; `expect` messages state the invariant
- [ ] Errors propagated with context; no silently discarded `Result` (`let _ =`, `.ok()`)
- [ ] Narrowing `as` casts replaced by `try_from` where values come from outside; overflow handled explicitly in size/offset/money arithmetic
- [ ] Strings sliced only on char boundaries; paths and env vars handled as `OsStr`/`Path`
- [ ] No reliance on `HashMap` iteration order; `Ord`/`Hash`/`Eq` impls consistent; float sorting uses `total_cmp`
- [ ] Drop order and temporary lifetimes checked for guards in `match`/`if let` scrutinees and tail expressions

##### Unsafe and FFI

- [ ] Every `unsafe` block has a `// SAFETY:` comment; every `unsafe fn` a `# Safety` section; blocks are minimal
- [ ] No `&`→`&mut` casts, invalid transmutes, uninitialised reads, unaligned references, `static mut` references
- [ ] `unsafe impl Send/Sync` justified; safe wrappers sound for all inputs
- [ ] FFI types `#[repr(C)]`; no panic unwinds into C; ownership of allocations documented; `CString` outlives its pointer
- [ ] Unsafe code tested under Miri; `#![forbid(unsafe_code)]` where possible

##### Concurrency and async

- [ ] No blocking calls on async worker threads; CPU-heavy work in `spawn_blocking`/`rayon`
- [ ] No `std::sync::MutexGuard`/`RefCell` borrow held across `.await`; lock scopes minimal; consistent lock order
- [ ] `select!` branches cancel-safe; non-cancel-safe futures pinned outside loops; spawned tasks joined, aborted or tracked
- [ ] Bounded channels/semaphores; timeouts on network I/O; graceful shutdown path
- [ ] Atomic orderings justified (`Release`/`Acquire` for publication); lock-free code tested with loom

##### API and Cargo

- [ ] Public API changes classified with SemVer (`cargo-semver-checks`); `#[non_exhaustive]` on growing enums/structs
- [ ] Features additive; `default-features = false` for library dependencies; feature matrix tested
- [ ] Virtual workspaces set `resolver`; workspace inheritance used consistently; `[lints]` configured
- [ ] `Cargo.lock` committed for deployable artifacts; CI uses `--locked`
- [ ] Build scripts minimal, with `rerun-if-changed`, no network, writing only to `OUT_DIR`

##### Tooling, security, observability

- [ ] `cargo fmt --check`, `cargo clippy --all-targets --all-features -- -D warnings`, `cargo test` (including doctests) in CI
- [ ] `cargo audit`/`cargo deny` in CI and on a schedule; new dependencies reviewed (build scripts, proc macros, unsafe, maintainers)
- [ ] No shell interpolation of input, no SQL string building, no disabled TLS verification
- [ ] Secrets not `Debug`-printed, logged or kept in CI caches; constant-time comparison for tokens
- [ ] Libraries use `log`/`tracing` facades only; `#[instrument]` skips secrets and large arguments; errors logged once with their chain

***

[^1]: Rust release notes (RELEASES.md), versions 1.0–1.99.1. https://github.com/rust-lang/rust/blob/master/RELEASES.md and https://doc.rust-lang.org/stable/releases.html
[^2]: Announcing Rust 1.99.0 (October 1, 2026). https://blog.rust-lang.org/2026/10/01/Rust-1.99.0/
[^3]: Announcing Rust 1.99.1 (October 15, 2026). https://blog.rust-lang.org/2026/10/15/Rust-1.99.1/
[^4]: The Rust Edition Guide (Rust 2018, 2021, 2024 chapters). https://doc.rust-lang.org/edition-guide/
[^5]: rustfmt configuration (`style_edition`) and the Rust Style Guide. https://rust-lang.github.io/rustfmt/ , https://doc.rust-lang.org/style-guide/
[^6]: thiserror documentation (2.0). https://docs.rs/thiserror ; anyhow documentation. https://docs.rs/anyhow
[^7]: The Rust Reference — Behavior considered undefined. https://doc.rust-lang.org/reference/behavior-considered-undefined.html ; The Rustonomicon. https://doc.rust-lang.org/nomicon/
[^8]: Tokio README — supported Rust versions and LTS releases. https://github.com/tokio-rs/tokio#supported-rust-versions
[^9]: Tokio documentation — `tokio::select!` cancellation safety. https://docs.rs/tokio/latest/tokio/macro.select.html#cancellation-safety
[^10]: The Cargo Book — SemVer Compatibility. https://doc.rust-lang.org/cargo/reference/semver.html
[^11]: cargo-semver-checks. https://github.com/obi1kenobi/cargo-semver-checks
[^12]: The Cargo Book — FAQ: Why have Cargo.lock in version control? https://doc.rust-lang.org/cargo/faq.html#why-have-cargolock-in-version-control ; Resolver versions. https://doc.rust-lang.org/cargo/reference/resolver.html#resolver-versions
[^13]: Supply chain attack on arrayref (August 20, 2026). https://blog.rust-lang.org/2026/08/20/supply-chain-attack-on-arrayref/
[^14]: GitHub Actions leaking secrets when Miri output is cached (September 21, 2026). https://blog.rust-lang.org/2026/09/21/github-actions-leaking-secrets-when-miri-output-is-cached/
[^15]: Clippy README — lint categories. https://github.com/rust-lang/rust-clippy#clippy ; Clippy lint list. https://rust-lang.github.io/rust-clippy/stable/index.html
[^16]: RustSec advisory database and cargo-audit. https://rustsec.org/ ; cargo-deny. https://embarkstudios.github.io/cargo-deny/
[^17]: crates.io development updates: Trusted Publishing (July 2025) and GitLab support/enforcement (January 2026). https://blog.rust-lang.org/2025/07/11/crates-io-development-update-2025-07/ , https://blog.rust-lang.org/2026/01/21/crates-io-development-update/
