### Memory safety and binary security guide
Applies to: untrusted bytes in C/C++, Rust `unsafe`, cgo/JNI/FFI, kernel drivers, loaders, plugins, JITs.

#### Hunt for
- **Out-of-bounds**: unbounded length, offset, index; headroom after prefixes, padding, terminators; source and destination capacity; declared length trusted before short input checked.
- **Integer errors**: `a - b` with `b > a`, `count * size` overflow, negative-to-unsigned, 64-to-32-bit narrowing, `-1` sentinels into allocation, copy, index; check value actually used; bytes vs elements vs code units mixed across check, allocation, copy; check in same wrong unit as allocation still wrong.
- **Uninitialized data**: buffers, padding, spare capacity returned, hashed, serialized across trust boundary unwritten; need observable consumer, realistic output length.
- **UAF/double free**: callbacks, timers, iterators, raw pointers outlive owner on error, cancel, close, realloc.
- **Type confusion**: tag, discriminator, foreign handle checked unlike read; unchecked downcasts, stale tags.
- **Races**: non-atomic refcounts, lazy init, signal handlers, teardown invalidating bounds or pointers; inconsistent lock order; locks held across callbacks or blocking I/O.
- **FFI contract**: each `extern`, cgo, JNI binding: allocator, freer, pinning; pointer lifetime; null, zero length; retained callbacks.
- **ABI/layout**: packing, `repr`, alignment, endianness, enum/integer widths, callback signatures per architecture and build flags.
- **Unwind/threads**: panic or exception across no-unwind ABI; callback after teardown; single-thread API off-thread.
- **Loader trust**: privileged process loads library from lower-trust-writable path or bare name via cwd or environment; verified file not mapped image (re-resolution, replacement, architecture slices).
- **Binary metadata**: offsets, counts, sections, relocations, symbols, bytecode trusted before range and overlap checks.
- **JIT/unload**: validator vs optimized code disagree on types or bounds; pointers or threads survive unload.
- **Kernel**: double fetch from user memory, unchecked copy size or dispatch index, device nodes checking shape not caller authority.

#### Validate before reporting
- Re-derive bound from all callers with worst accepted input.
- Guard in one caller, architecture, flag, compatibility path does not protect others.
- State exact effect: invalid read or write, stale alias, uninitialized output, unauthorized load, deadlock, safe abort.
- Own plugin in own process not boundary violation; safely rejected input not finding.
