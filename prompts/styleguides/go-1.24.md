### Go 1.24 — Complete Developer Guideline

> **Version**: Go 1.24.0 (released 2025-02-10)
> **Scope**: Full language spec, all 1.24 changes delta over 1.23, idioms, concurrency, performance, CLI, file I/O, testing, and best practices — with examples throughout.
> **Note**: This document is self-contained. All relevant prior-version material is carried forward or superseded. Sections marked **[NEW in 1.24]** cover additions specific to this version.

---

#### Table of Contents

1. [What's New in Go 1.24](#whats-new-in-go-124)
2. [Project Layout & Modules](#project-layout--modules)
3. [Basic Types & Variables](#basic-types--variables)
4. [Pointers & Weak Pointers](#pointers--weak-pointers)
5. [Control Flow](#control-flow)
6. [Functions](#functions)
7. [Defer, Panic & Recover](#defer-panic--recover)
8. [Structs & Methods](#structs--methods)
9. [Interfaces & Embedding](#interfaces--embedding)
10. [Generics & Type Aliases](#generics--type-aliases)
11. [Iterators — iter package](#iterators--iter-package)
12. [Collection Types: Arrays, Slices, Maps](#collection-types-arrays-slices-maps)
13. [Strings & Bytes](#strings--bytes)
14. [Error Handling](#error-handling)
15. [Structured Logging with log/slog](#structured-logging-with-logslog)
16. [Concurrency](#concurrency)
17. [Memory Management: Cleanup & Weak Pointers](#memory-management-cleanup--weak-pointers)
18. [Context Package](#context-package)
19. [File I/O & Filesystem](#file-io--filesystem)
20. [HTTP Servers](#http-servers)
21. [Cryptography](#cryptography)
22. [Testing & Benchmarking](#testing--benchmarking)
23. [CLI Development & Tool Directive](#cli-development--tool-directive)
24. [Performance Caveats & PGO](#performance-caveats--pgo)
25. [Idioms & Things to Avoid](#idioms--things-to-avoid)

---

#### What's New in Go 1.24

Go 1.24.0 shipped 2025-02-10. It delivers **generic type aliases** (the final generics gap closed), **weak pointers** and improved finalizers, a new **Swiss Tables map** implementation (2–3% overall CPU improvement), the **`os.Root`** sandbox type, the **`testing.B.Loop`** method, new crypto packages (`crypto/mlkem`, `crypto/hkdf`, `crypto/pbkdf2`, `crypto/sha3`), and a completely reworked tool dependency system for `go.mod`.

##### Language changes

###### 1. Generic type aliases — now fully supported [NEW in 1.24] ★★

Previewed under `GOEXPERIMENT=aliastypeparams` in Go 1.23, now stable in 1.24.

```go
// A type alias can now have type parameters
type Seq[V any] = func(yield func(V) bool)

// Partially instantiated alias
type StringSeq = Seq[string]

// Alias for a constrained generic type
type ComparableMap[K comparable, V any] = map[K]V

// Partially apply a generic type
type StringMap[V any] = ComparableMap[string, V]
type StringIntMap = StringMap[int]

// Real-world: export an alias for an internal generic type
// package internal/storage
type Repository[T any] struct { /* ... */ }

// package api — re-export with a clearer name
type UserRepo = internal.Repository[User]
type PostRepo = internal.Repository[Post]
```

**Alias vs. defined type — key distinction:**

```go
type NewInt int        // defined type — NOT interchangeable with int
type AliasInt = int    // alias — completely interchangeable with int

// Generic alias: same rules
type MySlice[T any] = []T  // alias — []string and MySlice[string] are identical
type MyList[T any] []T     // defined type — NOT interchangeable with []T
```

**What generic aliases enable:**
- Re-exporting internal generic types with different names.
- Simplifying complex generic signatures for API consumers.
- Incremental migration of types across package boundaries.

**Remaining restriction** (removed in 1.25): aliases cannot yet be used cross-package with all type checker tools. For now, a disallow flag exists: `GOEXPERIMENT=noaliastypeparams`.

##### Runtime improvements [NEW in 1.24]

###### Swiss Tables map implementation

Maps are now backed by a Swiss Tables hash table. **No code changes required.** The improvement applies to all Go maps automatically.

| Operation | Improvement |
|---|---|
| Large map access/assignment | ~30% faster |
| Pre-sized map assignment | ~35% faster |
| Map iteration | 10–60% faster |

```go
// No API change — existing code benefits automatically
m := make(map[string]int, 1000)  // 35% faster assignment vs 1.23
for k, v := range m { }          // 10–60% faster iteration
```

To revert to the old implementation:
```bash
GOEXPERIMENT=noswissmap go build ./...
```

###### sync.Map — new concurrent hash-trie

`sync.Map` now uses a concurrent hash-trie. Modifications of disjoint key sets no longer contend on large maps. No ramp-up time for low-contention loads.

```bash
GOEXPERIMENT=nosynchashtriemap go build ./... # revert if needed
```

###### Small object allocation — more efficient

Small objects (less than a page) are now allocated more efficiently, reducing per-allocation overhead. Net effect: **2–3% CPU overhead reduction** across representative benchmarks.

###### New runtime-internal mutex

Lower-overhead spinbit mutex used internally. Overall CPU improvement is additive with map and allocation improvements.

##### Toolchain

###### Tool directive in go.mod [NEW in 1.24] ★

Replaces the `tools.go` blank-import workaround.

```bash
# Add a tool dependency
go get -tool golang.org/x/tools/cmd/stringer@latest
# Adds to go.mod:
# tool golang.org/x/tools/cmd/stringer

# Run a module tool (automatically uses the version in go.mod)
go tool stringer -type=Color .

# Run a built-in tool
go tool vet ./...
go tool pprof cpu.prof

# Upgrade all tools
go get tool

# Install all tools to GOBIN
go install tool

# go run and go tool executables are now cached in the build cache
```

**go.mod with tool directive:**
```
module myapp

go 1.24.0

require (
    github.com/spf13/cobra v1.8.0
)

tool (
    golang.org/x/tools/cmd/stringer
    github.com/sqlc-dev/sqlc/cmd/sqlc
    github.com/golangci/golangci-lint/cmd/golangci-lint
)
```

This is the **idiomatic replacement for the tools.go pattern**:

```go
// OLD — tools.go (no longer needed in 1.24)
//go:build tools

package tools

import (
    _ "golang.org/x/tools/cmd/stringer"
    _ "github.com/sqlc-dev/sqlc/cmd/sqlc"
)

// NEW — just use go.mod tool directive + go tool <name>
```

###### go build -json [NEW in 1.24]

```bash
go build -json ./...
go install -json ./...
# Emits structured JSON build events to stdout
# Test failures also emitted as JSON from go test -json (interleaved)

# If your CI parses go test -json and the new build JSON breaks it:
GODEBUG=gotestjsonbuildtext=1 go test -json ./...
```

###### GOAUTH — private module authentication [NEW in 1.24]

```bash
# Authenticate private module fetches via a custom command
GOAUTH="git credentials" go get github.com/corp/internal@latest
# or a program that responds to authentication challenges
GOAUTH="/path/to/auth-helper" go mod download
```

###### go build embeds VCS version [NEW in 1.24]

```bash
go build -o myapp ./cmd/myapp
# binary now contains version from VCS tag/commit
# e.g., "v1.2.3" or "v1.2.3-0.20250101+g1a2b3c4" or "v1.2.3+dirty"
go version -m ./myapp  # shows embedded version

# Opt out
go build -buildvcs=false ./cmd/myapp
```

###### go vet — new checks [NEW in 1.24]

```go
// 1. tests analyser: malformed test/benchmark/fuzz/example names or signatures
func testSomething(t *testing.T) {}     // vet: should be TestSomething
func BenchmarkFoo(b *testing.B, x int) {} // vet: wrong signature
func ExampleFoo_() {}                   // vet: malformed suffix

// 2. printf: fmt.Printf(s) with no extra args is almost always a bug
s := "hello %s"
fmt.Printf(s)     // vet: use fmt.Print instead (s may contain % verbs)
fmt.Print(s)      // correct

// 3. buildtag: point-release in build constraint is invalid
//go:build go1.23.1  // vet: use go1.23 instead

// 4. copylock: C-style for loop variable containing mutex is now detected
for i := getSomeLock(); !i.done; i = i.next() {
    // i is copied each iteration in 1.22+ — unsafe if i contains a sync.Mutex
}
```

##### Standard Library — new packages

| Package | Purpose |
|---|---|
| `weak` | Weak pointers (`weak.Pointer[T]`) |
| `crypto/mlkem` | ML-KEM-768 and ML-KEM-1024 (post-quantum key exchange, FIPS 203) |
| `crypto/hkdf` | HKDF key derivation (RFC 5869) |
| `crypto/pbkdf2` | PBKDF2 password hashing (RFC 8018) |
| `crypto/sha3` | SHA-3, SHAKE, cSHAKE hash functions (FIPS 202) |
| `testing/synctest` | Experimental: test concurrent code with fake time |

##### Standard Library — selected changes

| Package | Change |
|---|---|
| `os` | New `Root` type for sandboxed filesystem access |
| `runtime` | `AddCleanup` — better alternative to `SetFinalizer` |
| `testing` | `B.Loop()`, `T.Chdir()`, `B.Chdir()`, `B.Context()` |
| `encoding/json` | `omitzero` struct tag option |
| `strings` | `Lines`, `SplitSeq`, `SplitAfterSeq`, `FieldsSeq`, `FieldsFuncSeq` |
| `bytes` | `Lines`, `SplitSeq`, `SplitAfterSeq`, `FieldsSeq`, `FieldsFuncSeq` |
| `log/slog` | `slog.DiscardHandler` |
| `encoding` | New `TextAppender` and `BinaryAppender` interfaces |
| `hash/maphash` | `Comparable`, `WriteComparable` — hash any comparable value |
| `math/rand` | `Seed` is now a no-op (was deprecated since 1.20) |
| `sync` | `sync.Map` uses concurrent hash-trie |
| `crypto/tls` | Encrypted Client Hello (ECH); X25519MLKEM768 on by default |
| `crypto/ecdsa` | Deterministic signing (RFC 6979) when rand source is nil |
| `crypto/rand` | `Text()` function; `Read` guaranteed not to fail |
| `net/http` | `Server.Protocols`, `Transport.Protocols`; unencrypted HTTP/2 |

---

#### Project Layout & Modules

```
myapp/
├── cmd/
│   └── myapp/
│       ├── main.go
│       └── default.pgo
├── internal/
├── pkg/
├── go.mod   # now contains tool directive
└── go.sum
```

##### go.mod with 1.24 features

```
module github.com/yourname/myapp

go 1.24.0

toolchain go1.24.2

require (
    github.com/spf13/cobra v1.8.0
)

tool (
    golang.org/x/tools/cmd/stringer
    github.com/golangci/golangci-lint/cmd/golangci-lint
)

godebug asynctimerchan=0  // 1.23+: new timer channel semantics
```

##### Verify library APIs against the pinned version

Verify library APIs against the actual module versions in `go.mod` before
claiming an API is missing or unavailable. In Go 1.24, prefer `tool` directives
in `go.mod` (over the old blank-import `tools.go` file) to track executable
development tools.

- Do not claim a controller-runtime helper is unavailable without checking the
  pinned `sigs.k8s.io/controller-runtime` version.
- Say "this API is not available in vX.Y.Z" only when the module version proves
  it.
- Do not infer API availability from memory, a newer version's docs, or another
  project's dependency set.

---

#### Basic Types & Variables

```go
var x int = 10
y := "hello"
z := 3.14

m := min(x, 20)  // 10  (1.21+)
M := max(x, 20)  // 20  (1.21+)
```

##### iota

```go
type Direction int
const (
    North Direction = iota
    East; South; West
)

type Permission uint
const (
    Read    Permission = 1 << iota
    Write
    Execute
)
```

---

#### Pointers & Weak Pointers

##### Strong pointers

```go
x := 42
p := &x
*p = 99

p1 := new(int)
p2 := &Point{X: 1, Y: 2}
```

##### Weak pointers [NEW in 1.24]

A `weak.Pointer[T]` does not prevent the pointed-to object from being garbage-collected. When the GC collects the object, the weak pointer's `Value()` returns `nil`.

```go
import "weak"

type ExpensiveObject struct {
    data []byte
}

// Create a strong reference and a weak pointer to the same object
strong := &ExpensiveObject{data: make([]byte, 1<<20)}
wp := weak.Make(strong) // weak.Pointer[ExpensiveObject]

// Value returns the object if still alive, nil if GC'd
if obj := wp.Value(); obj != nil {
    use(obj)
} else {
    // object has been collected
}

// Drop the strong reference — GC may now collect the object
strong = nil
runtime.GC()
fmt.Println(wp.Value() == nil) // true — object was collected
```

**Weak pointer equality**: two `weak.Pointer[T]` are equal iff they refer to the same object or both are nil.

```go
wp1 := weak.Make(strong)
wp2 := weak.Make(strong)
fmt.Println(wp1 == wp2) // true — same object
```

##### Weak-pointer cache pattern

The canonical use case is a cache that doesn't force objects to remain alive:

```go
type Cache[K comparable, V any] struct {
    mu sync.Mutex
    m  map[K]weak.Pointer[V]
}

func (c *Cache[K, V]) Get(key K, load func() *V) *V {
    c.mu.Lock()
    defer c.mu.Unlock()

    if wp, ok := c.m[key]; ok {
        if v := wp.Value(); v != nil {
            return v // cached and still alive
        }
    }
    // cache miss or GC'd — reload
    v := load()
    c.m[key] = weak.Make(v)
    return v
}
```

This combined with `runtime.AddCleanup` (see Memory Management section) gives you auto-evicting caches.

**When to use `weak` vs `unique`:**
| | `unique.Handle[T]` | `weak.Pointer[T]` |
|---|---|---|
| Purpose | Intern/canonicalise values | Observe without retaining |
| GC behaviour | Keeps object alive | Does NOT keep alive |
| Use for | String interning, canonical IDs | Caches, observers |

---

#### Control Flow

##### if / else

```go
if x > 0 { fmt.Println("positive") }
if err := doSomething(); err != nil { return err }
f, err := os.Open(name)
if err != nil { return err }
use(f)
```

##### for

```go
for i := 0; i < 10; i++ {}
for x < 100 { x *= 2 }
for { if done() { break } }
for i, v := range s {}
for k, v := range m {}
for i, r := range "héllo" {}
for v := range ch {}
for i := range 10 {}            // 1.22+
for v := range myIter() {}      // 1.23+: iterator functions
for k, v := range myIter2() {}  // 1.23+
```

##### switch

```go
switch x {
case 1:    fmt.Println("one")
case 2, 3: fmt.Println("two or three")
default:   fmt.Println("other")
}

switch {
case x < 0:  fmt.Println("negative")
default:     fmt.Println("non-negative")
}

func describe(i any) {
    switch v := i.(type) {
    case int:    fmt.Printf("int: %d\n", v)
    case string: fmt.Printf("string: %s\n", v)
    default:     fmt.Printf("%T\n", v)
    }
}
```

##### select

```go
select {
case msg := <-ch1:              fmt.Println(msg)
case ch2 <- "hello":            fmt.Println("sent")
case <-time.After(time.Second): fmt.Println("timeout")
default:                        fmt.Println("non-blocking")
}
```

---

#### Functions

```go
func add(a, b int) int { return a + b }
func divide(a, b float64) (float64, error) {
    if b == 0 { return 0, errors.New("division by zero") }
    return a / b, nil
}
func sum(nums ...int) int {
    total := 0
    for _, n := range nums { total += n }
    return total
}
func counter() func() int {
    n := 0
    return func() int { n++; return n }
}
```

---

#### Defer, Panic & Recover

```go
func process(name string) error {
    f, err := os.Open(name)
    if err != nil { return err }
    defer f.Close()
    return nil
}

// time.Since must be in closure — vet catches bare defer
defer func() { log.Println(time.Since(start)) }()

// panic(nil) — 1.21+: recover() is non-nil for any panic
defer func() {
    if r := recover(); r != nil { /* a panic occurred */ }
}()
```

**Defer inside iterator loop bodies**: defers accumulate until the surrounding function returns, not when the iteration ends. Wrap in an anonymous function per iteration.

```go
// WRONG: all defers run at function end
for path := range filePaths() {
    f, _ := os.Open(path)
    defer f.Close()
}

// CORRECT
for path := range filePaths() {
    func() {
        f, err := os.Open(path)
        if err != nil { return }
        defer f.Close()
        process(f)
    }()
}
```

---

#### Structs & Methods

```go
type User struct {
    ID    int
    Name  string
    Email string
    admin bool
}

type Timestamps struct { CreatedAt, UpdatedAt time.Time }
type Post struct { ID int; Title string; Timestamps }

func (u User) Display() string { return u.Name }
func (u *User) Promote()       { u.admin = true }
```

##### Value vs. Pointer receivers

| Condition | Receiver |
|---|---|
| Mutates receiver | `*T` |
| Large struct | `*T` |
| Contains sync primitive | `*T` — never copy |
| Read-only, small | `T` |
| Any method uses `*T` | `*T` for ALL |

##### structs.HostLayout (1.23+)

```go
import "structs"
type NetworkHeader struct {
    _ structs.HostLayout
    Version  uint8
    Type     uint8
    Length   uint16
}
```

##### Functional options

```go
type Option func(*Server)
func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}
func NewServer(host string, port int, opts ...Option) *Server {
    s := &Server{host: host, port: port, timeout: 30 * time.Second}
    for _, o := range opts { o(s) }
    return s
}
```

---

#### Interfaces & Embedding

```go
type Writer interface { Write(p []byte) (n int, err error) }
type WriteCloser interface { Writer; io.Closer }

var x any = "hello"
s, ok := x.(string)

switch v := x.(type) {
case int:    fmt.Println("int", v)
case string: fmt.Println("string", v)
}
```

##### encoding.TextAppender and BinaryAppender [NEW in 1.24]

```go
// Two new interfaces in the encoding package
type TextAppender interface {
    AppendText(b []byte) ([]byte, error)
}
type BinaryAppender interface {
    AppendBinary(b []byte) ([]byte, error)
}

// Now implemented by: time.Time, net.IP, net/netip.Addr,
// net/netip.AddrPort, net/netip.Prefix, math/big.Float/Int/Rat,
// net/url.URL, regexp.Regexp, and all hash/crypto hash types.

// Use for zero-copy encoding into existing buffers
buf := make([]byte, 0, 256)
buf, err = time.Now().AppendText(buf)
buf = append(buf, ',')
addr := netip.MustParseAddr("192.168.1.1")
buf, err = addr.AppendText(buf)
```

---

#### Generics & Type Aliases

##### Generic type aliases (stable in 1.24)

```go
// Simple alias
type Seq[V any] = func(yield func(V) bool)

// Constraint-preserving alias
type SortedSlice[T cmp.Ordered] = []T

// Re-export with different name
type UserStore = storage.Repository[User]

// Partial instantiation
type StringMap[V any] = map[string]V
type Config = StringMap[string]
```

##### Regular generics

```go
func Map[T, U any](s []T, f func(T) U) []U {
    result := make([]U, len(s))
    for i, v := range s { result[i] = f(v) }
    return result
}

func Contains[T comparable](s []T, v T) bool {
    for _, e := range s { if e == v { return true } }
    return false
}

type Stack[T any] struct{ items []T }
func (s *Stack[T]) Push(v T)  { s.items = append(s.items, v) }
func (s *Stack[T]) Pop() T {
    n := len(s.items)
    v := s.items[n-1]
    s.items = s.items[:n-1]
    return v
}

import "cmp"
host := cmp.Or(os.Getenv("HOST"), cfg.Host, "localhost") // 1.22+
```

---

#### Iterators — iter package

From 1.23, stable. See Go 1.23 guide for full detail. Key summary:

```go
import "iter"

// Types
type Seq[V any]    = func(yield func(V) bool)
type Seq2[K, V any] = func(yield func(K, V) bool)

// Write iterators
func Evens(n int) iter.Seq[int] {
    return func(yield func(int) bool) {
        for i := 0; i < n; i += 2 {
            if !yield(i) { return }
        }
    }
}
for v := range Evens(10) { fmt.Println(v) }

// Pull iterator for zipping
next, stop := iter.Pull(mySeq)
defer stop()
for {
    v, ok := next()
    if !ok { break }
    process(v)
}
```

##### New iterator functions in strings and bytes [NEW in 1.24]

```go
import "strings"

// Lines — iterate over newline-terminated lines
for line := range strings.Lines("hello\nworld\n") {
    fmt.Printf("%q\n", line) // "hello\n", "world\n"
}

// SplitSeq — iterate over split parts (lazy, no slice allocation)
for part := range strings.SplitSeq("a,b,c,d", ",") {
    fmt.Println(part) // a, b, c, d
}

// SplitAfterSeq — keep separator
for part := range strings.SplitAfterSeq("a,b,c", ",") {
    fmt.Println(part) // "a,", "b,", "c"
}

// FieldsSeq — split on whitespace
for word := range strings.FieldsSeq("  hello   world  ") {
    fmt.Println(word) // hello, world
}

// FieldsFuncSeq — split on custom predicate
for word := range strings.FieldsFuncSeq("hello,world;go", func(r rune) bool {
    return r == ',' || r == ';'
}) {
    fmt.Println(word) // hello, world, go
}
```

Same functions exist in `bytes` package, operating on `[]byte` rather than `string`.

**Before 1.24**: `strings.Split` / `strings.Fields` always allocated a `[]string`. The new `*Seq` variants are lazy iterators — no slice allocated, ideal for large inputs.

---

#### Collection Types: Arrays, Slices, Maps

##### Arrays and Slices

```go
s := []int{1, 2, 3}
s2 := make([]int, 0, 10)
c := make([]int, len(s)); copy(c, s)
result := make([]string, 0, len(items))
for _, item := range items { result = append(result, process(item)) }
```

##### slices package (1.21+, updated through 1.23)

```go
import "slices"
slices.Sort(s)
slices.SortFunc(s, func(a, b T) int { return cmp.Compare(a, b) })
idx, found := slices.BinarySearch(sorted, v)
slices.Contains(s, v)
slices.Reverse(s)
slices.Compact(s)           // zeroes freed elements
slices.DeleteFunc(s, fn)    // zeroes freed elements
slices.Clip(s)
slices.Grow(s, n)
slices.Insert(s, i, vs...)
slices.Delete(s, i, j)     // zeroes freed elements
slices.Concat(ss...)
slices.Chunk(s, n)          // iter.Seq[[]E]
slices.All(s)               // iter.Seq2[int, E]
slices.Values(s)            // iter.Seq[E]
slices.Backward(s)          // iter.Seq2[int, E]
slices.Collect(seq)         // materialise iterator
slices.Repeat(s, n)
slices.Max(s); slices.Min(s)
```

##### Maps — Swiss Tables (auto, transparent)

```go
// All maps use Swiss Tables in 1.24 — 30%+ faster, no code change
m := make(map[string]int, 1000)
m["key"] = 1
v, ok := m["key"]
delete(m, "key")
clear(m)          // 1.21+: remove all entries

// Iteration — order still random
for k, v := range m { fmt.Println(k, v) }
```

##### maps package (1.21+, updated through 1.23)

```go
import "maps"
maps.Clone(m)
maps.Copy(dst, src)
maps.DeleteFunc(m, fn)
maps.Equal(m1, m2)
maps.All(m)        // iter.Seq2[K, V]
maps.Keys(m)       // iter.Seq[K]
maps.Values(m)     // iter.Seq[V]
maps.Insert(m, seq)
maps.Collect(seq)
```

##### Map rules and mutation during iteration

- `var m map[string]int` is nil — writing panics. Always use `make`.
- Iteration order is randomized every run.
- Maps are reference types — assignment shares the underlying map.
- Maps are **not safe for concurrent access** — use `sync.RWMutex` or `sync.Map`.

Deleting map entries while ranging over the same map is allowed. Only flag map
mutation during iteration when there is a concrete Go issue, for example:

- unsynchronized concurrent map access
- inserting or updating entries while depending on deterministic iteration
- deleting entries changes required business behavior
- callbacks are invoked while holding a lock and can re-enter the same lock

Do not apply generic "modifying a collection while iterating" rules from other
languages to Go map deletion.

##### hash/maphash.Comparable [NEW in 1.24]

```go
import "hash/maphash"

// Hash any comparable value — useful for custom hash maps
var h maphash.Hash
hash := maphash.Comparable(h.Seed(), "hello")   // uint64
hash2 := maphash.Comparable(h.Seed(), 42)
hash3 := maphash.Comparable(h.Seed(), MyStruct{X: 1, Y: 2})

// WriteComparable — add to existing hash computation
maphash.WriteComparable(&h, myKey)
result := h.Sum64()
```

---

#### Strings & Bytes

```go
s := "héllo"
fmt.Println(len(s))         // 6 bytes
fmt.Println(len([]rune(s))) // 5 runes
for i, r := range s { fmt.Printf("%d: %c\n", i, r) }

var b strings.Builder
b.Grow(1000)
for range 1000 { b.WriteByte('x') }
result := b.String()

strings.Contains("foobar", "bar")
strings.HasPrefix("foobar", "foo")
strings.Split("a,b,c", ",")             // allocates []string
strings.SplitSeq("a,b,c", ",")          // 1.24+: lazy iterator, no allocation
strings.Lines("line1\nline2\n")         // 1.24+: iterator over lines
strings.FieldsSeq("hello world")        // 1.24+: lazy whitespace split
strings.Join([]string{"a","b"}, "-")
strings.TrimSpace("  hello  ")
strings.ReplaceAll("aabbcc", "b", "x")
after, found := strings.CutPrefix("Gopher", "Go")
before, found := strings.CutSuffix("Gopher", "er")
```

---

#### Error Handling

```go
result, err := doSomething()
if err != nil { return fmt.Errorf("context: %w", err) }

var ErrNotFound = errors.New("not found")
if errors.Is(err, ErrNotFound) { /* handle */ }

type ValidationError struct { Field, Message string }
func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation: %s: %s", e.Field, e.Message)
}
var ve *ValidationError
if errors.As(err, &ve) { fmt.Println(ve.Field) }

combined := errors.Join(err1, err2)
wrapped  := fmt.Errorf("two: %w and %w", err1, err2)
if errors.Is(err, errors.ErrUnsupported) { /* fallback */ }
```

**Always check errors.** A few discards are idiomatic and should not be flagged:

- deferred `Close()` on files opened only for reading
- `defer tx.Rollback()`, which is a no-op after a successful commit
- writes to `bytes.Buffer` or `strings.Builder`, which never return an error

For everything else, handle the error or make the discard explicit with `_ =`.

---

#### Structured Logging with log/slog

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo, AddSource: true,
}))
slog.SetDefault(logger)

slog.Info("request",
    slog.String("method", "GET"),
    slog.Int("status", 200),
    slog.Duration("latency", 12*time.Millisecond),
)

reqLogger := logger.With("requestID", reqID)
dbLogger := logger.WithGroup("db").With("host", "localhost")
slog.InfoContext(ctx, "processing", "id", id)
```

##### slog best practices

- **Always use structured key-value pairs** — never format strings into the message.
- **Keys should be lowercase snake_case** for consistent log querying.
- **Pass `context.Context` through** and use `InfoContext`/`ErrorContext` for trace IDs.
- **Group related attributes** with `slog.Group` or `logger.WithGroup`.
- **Never log secrets, tokens, credentials, or full request/response bodies** unless they are explicitly scrubbed.

##### slog.DiscardHandler [NEW in 1.24]

```go
// Discard all log output — useful in tests and benchmarks
logger := slog.New(slog.DiscardHandler{})
slog.SetDefault(logger)

// Or pass as a no-op logger to a component
service := NewService(slog.New(slog.DiscardHandler{}))
```

---

#### Concurrency

##### Goroutines (1.22+: per-iteration loop variables)

```go
for i := range 5 { go func() { fmt.Println(i) }() }  // safe in 1.22+
for _, v := range items { go func() { process(v) }() }
```

##### Channels

```go
ch  := make(chan int)        // unbuffered
bch := make(chan string, 10) // buffered

go func() { ch <- 42 }()
val := <-ch
close(ch)
for v := range ch { fmt.Println(v) }

// Timer channels are unbuffered since 1.23
t := time.NewTimer(5 * time.Second)
defer t.Stop()
select {
case <-t.C: fmt.Println("fired")
default:    fmt.Println("not yet")
}
t.Reset(10 * time.Second) // safe in 1.23+ modules
```

##### sync.WaitGroup

```go
var wg sync.WaitGroup
for _, item := range items {
    wg.Add(1)
    go func(item Item) {
        defer wg.Done()
        process(item)
    }(item)
}
wg.Wait()
```

##### sync.Mutex / sync.RWMutex

```go
type SafeMap struct {
    mu sync.RWMutex
    m  map[string]int
}
func (s *SafeMap) Get(k string) (int, bool) {
    s.mu.RLock(); defer s.mu.RUnlock(); return s.m[k]
}
func (s *SafeMap) Set(k string, v int) {
    s.mu.Lock(); defer s.mu.Unlock(); s.m[k] = v
}
```

##### sync.Map — hash-trie, faster in 1.24

```go
var m sync.Map
m.Store("k", "v")
v, ok := m.Load("k")
m.Delete("k")
old, loaded := m.Swap("k", "new")
ok = m.CompareAndSwap("k", "old", "new")
ok = m.CompareAndDelete("k", "value")
m.Range(func(k, v any) bool { return true })
m.Clear() // 1.23+
```

##### sync.Once helpers (1.21+)

```go
initDB := sync.OnceFunc(func() { db = openDB() })
getConfig := sync.OnceValue(func() *Config { return loadConfig() })
getConn := sync.OnceValues(func() (*sql.DB, error) { return sql.Open("postgres", dsn) })
```

##### sync/atomic

```go
var counter atomic.Int64; counter.Add(1)
var ptr atomic.Pointer[Config]; ptr.Store(newConfig)
var flags atomic.Uint32
flags.Or(0b0100)   // 1.23+: set bit
flags.And(^uint32(0b0100)) // 1.23+: clear bit
```

##### Worker pool

```go
func workerPool(jobs <-chan Job, results chan<- Result, n int) {
    var wg sync.WaitGroup
    for range n {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs { results <- process(job) }
        }()
    }
    go func() { wg.Wait(); close(results) }()
}
```

##### testing/synctest — experimental [NEW in 1.24]

For testing concurrent code with fake time:

```go
// GOEXPERIMENT=synctest required
import "testing/synctest"

func TestRateLimit(t *testing.T) {
    synctest.Run(func() {
        // Within this bubble, time.Now() and time functions use a fake clock
        rl := NewRateLimiter(10, time.Second) // 10 requests per second

        for range 10 { rl.Allow() }
        if rl.Allow() { t.Fatal("expected rate limit") }

        // Advance fake clock by 1 second — no actual sleep
        time.Sleep(time.Second)
        synctest.Wait() // wait for all goroutines to block

        if !rl.Allow() { t.Fatal("expected allow after 1 second") }
    })
}
```

##### Classifying races before reporting

```bash
go test -race ./...  # always in CI
```

Separate true data races from lifecycle, shutdown, or ordering races. Only call
something a data race after confirming unsynchronized shared-memory access with
at least one write.

Check a library's concurrency contract before assuming concurrent method calls
are unsafe. Some Go types are explicitly safe for concurrent use, while others
require caller-side synchronization.

---

#### Memory Management: Cleanup & Weak Pointers

##### runtime.AddCleanup [NEW in 1.24] ★★

A better finalizer. Prefer `AddCleanup` over `runtime.SetFinalizer` in all new code.

```go
import "runtime"

// Signature: AddCleanup(ptr *T, fn func(arg A), arg A) runtime.Cleanup
type Resource struct {
    handle uintptr
}

func NewResource() *Resource {
    r := &Resource{handle: openHandle()}
    // Register cleanup: when r is collected, closeHandle(r.handle) is called
    cleanup := runtime.AddCleanup(r, closeHandle, r.handle)
    // cleanup.Stop() can be called to cancel the cleanup
    _ = cleanup
    return r
}

func closeHandle(h uintptr) {
    // called in a separate goroutine, sequentially with other cleanups
    syscall.Close(syscall.Handle(h))
}
```

##### AddCleanup vs SetFinalizer

| Feature | `SetFinalizer` | `AddCleanup` |
|---|---|---|
| Multiple per object | No — only one | Yes — unlimited |
| Interior pointers | No | Yes |
| Reference cycles | Prevents collection | Does NOT prevent collection |
| Delays object freeing | Yes | No |
| Runs in | Finalizer goroutine | Separate goroutine, sequentially |
| Cancellable | No | Yes — `cleanup.Stop()` |

```go
// Old — one finalizer, prevents collection of cycles
runtime.SetFinalizer(obj, func(o *MyObj) { o.Close() })

// New — multiple cleanups, no cycle issues
c1 := runtime.AddCleanup(obj, logClosed, obj.id)
c2 := runtime.AddCleanup(obj, closeHandle, obj.handle)
c3 := runtime.AddCleanup(obj, releaseMemory, obj.buf)
```

##### SetFinalizer — still available but discouraged

```go
// Still works — use only for compatibility
runtime.SetFinalizer(obj, (*MyObj).Close)
// SetFinalizer on an object in a cycle: object may never be collected
```

##### Combining weak.Pointer with AddCleanup

The canonical auto-evicting cache:

```go
type Cache[K comparable, V any] struct {
    mu sync.Mutex
    m  map[K]weak.Pointer[V]
}

func (c *Cache[K, V]) GetOrLoad(key K, load func(K) *V) *V {
    c.mu.Lock()
    defer c.mu.Unlock()

    if wp, ok := c.m[key]; ok {
        if v := wp.Value(); v != nil {
            return v
        }
    }

    v := load(key)
    wp := weak.Make(v)
    c.m[key] = wp

    // When v is collected, remove the stale entry from the cache
    runtime.AddCleanup(v, func(k K) {
        c.mu.Lock()
        defer c.mu.Unlock()
        // Only delete if still pointing to the collected object
        if existing, ok := c.m[k]; ok && existing.Value() == nil {
            delete(c.m, k)
        }
    }, key)

    return v
}
```

---

#### Context Package

```go
ctx := context.Background()
ctx, cancel := context.WithCancel(parent); defer cancel()
ctx, cancel  = context.WithTimeout(parent, 5*time.Second); defer cancel()
ctx, cancel  = context.WithDeadline(parent, deadline); defer cancel()
ctx          = context.WithValue(parent, key{}, value)
ctx, cancel  = context.WithCancelCause(parent); defer cancel(nil)   // 1.20+
detached     := context.WithoutCancel(requestCtx)                   // 1.21+
ctx, cancel  = context.WithTimeoutCause(parent, 5*time.Second, err) // 1.21+
stop         := context.AfterFunc(ctx, cleanup)                     // 1.21+
```

**Never store context in a struct. Never pass nil. Always `defer cancel()`.**

---

#### File I/O & Filesystem

##### Standard I/O patterns

```go
f, err := os.Open("file.txt")
if err != nil { return err }
defer f.Close()

w := bufio.NewWriter(f); defer w.Flush()

scanner := bufio.NewScanner(f)
scanner.Buffer(make([]byte, 1<<20), 1<<20)
for scanner.Scan() { process(scanner.Text()) }
if err := scanner.Err(); err != nil { return err }

io.Copy(dst, src)
data, err := os.ReadFile("small.json")
os.CopyFS("./output", os.DirFS("./source")) // 1.23+
```

##### os.Root — sandboxed filesystem [NEW in 1.24] ★

`os.Root` limits all filesystem operations to a specific directory. Symlinks that point outside the root are rejected. Essential for handling untrusted paths.

```go
import "os"

// Open a root-sandboxed directory
root, err := os.OpenRoot("./uploads")
if err != nil { return err }
defer root.Close()

// All operations are confined to ./uploads
f, err := root.Open("user123/avatar.png")    // ./uploads/user123/avatar.png
f2, err := root.Create("user456/profile.json")

// Symlink attacks are BLOCKED — unlike filepath.IsLocal
// If user123/link -> ../../etc/passwd:
_, err = root.Open("user123/link")  // error: path escapes root

// Methods mirror os package
root.Mkdir("newdir", 0755)
root.Stat("file.txt")
root.ReadDir("subdir")
root.Remove("old.txt")
root.Rename("old.txt", "new.txt")
root.Chmod("file.txt", 0644)
```

**Why `os.Root` over `filepath.IsLocal`:** `filepath.IsLocal` only checks the path string lexically; it does not protect against symlink traversal attacks. `os.Root` enforces the boundary at the OS level.

```go
// Old — vulnerable to symlink attacks
func serveFile(baseDir, userPath string) error {
    if !filepath.IsLocal(userPath) { return errors.New("invalid path") }
    f, err := os.Open(filepath.Join(baseDir, userPath)) // still vulnerable to symlinks
    // ...
}

// New — truly safe
func serveFile(root *os.Root, userPath string) error {
    f, err := root.Open(userPath) // symlink escapes blocked
    if err != nil { return err }
    defer f.Close()
    // ...
}
```

##### JSON

```go
enc := json.NewEncoder(w); enc.SetIndent("", "  "); enc.Encode(data)
dec := json.NewDecoder(r); dec.Decode(&result)

type User struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Pass       string   `json:"-"`
    Age        int      `json:"age,omitempty"`     // omit if zero (int: 0)
    CreatedAt  time.Time `json:"created_at,omitzero"` // 1.24+: omit zero time.Time
    UpdatedAt  time.Time `json:"updated_at,omitzero,omitempty"` // both flags ok
}
```

##### omitzero vs omitempty [NEW in 1.24]

| Tag | Omits when | time.Time zero | Struct zero |
|---|---|---|---|
| `omitempty` | Empty/nil for collections, 0 for numbers, "" for strings | **Never** (always has fields) | **Never** |
| `omitzero` | Value is zero (uses `IsZero()` if available) | **Yes** — time.IsZero() | **Yes** — if all fields zero |
| Both | Either condition | **Yes** | **Yes** |

```go
type Event struct {
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at,omitzero"`   // omitted if zero time
    Tags      []string  `json:"tags,omitempty"`          // omitted if nil/empty
    Score     float64   `json:"score,omitzero"`          // omitted if 0.0
}
```

---

#### HTTP Servers

##### Enhanced ServeMux (1.22+)

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    slog.Info("request", "pattern", r.Pattern) // 1.23+
    fmt.Fprintf(w, "user: %s\n", id)
})
mux.HandleFunc("POST /users", createUserHandler)
mux.HandleFunc("GET /files/{path...}", serveFileHandler)
mux.HandleFunc("GET /health/{$}", healthHandler)
```

##### HTTP protocol configuration [NEW in 1.24]

```go
// Configure exactly which HTTP protocols to support
srv := &http.Server{
    Addr:    ":8080",
    Handler: mux,
    Protocols: &http.Protocols{
        HTTP1: true,
        HTTP2: true,
        UnencryptedHTTP2: false, // h2c — plain-text HTTP/2
    },
}

// Client protocol configuration
transport := &http.Transport{
    Protocols: &http.Protocols{
        HTTP1: true,
        HTTP2: true,
    },
}
client := &http.Client{Transport: transport}
```

---

#### Cryptography

##### New packages [NEW in 1.24]

```go
// ML-KEM — post-quantum key exchange (FIPS 203)
import "crypto/mlkem"

// Alice generates a key pair
dk, err := mlkem.GenerateKey768()
encapsKey := dk.EncapsulationKey()

// Bob encapsulates a shared secret
ciphertext, bobSecret, err := mlkem.Encapsulate768(encapsKey)

// Alice decapsulates
aliceSecret, err := dk.Decapsulate(ciphertext)
// aliceSecret == bobSecret


// HKDF key derivation (RFC 5869)
import "crypto/hkdf"
key := hkdf.Key(sha256.New, inputKeyMaterial, salt, info, 32)


// PBKDF2 password hashing (RFC 8018)
import "crypto/pbkdf2"
hash, err := pbkdf2.Key(sha256.New, []byte("password"), salt, 100_000, 32)


// SHA-3 (FIPS 202)
import "crypto/sha3"
h := sha3.New256()
h.Write(data)
digest := h.Sum(nil)

shake := sha3.NewShake128()
shake.Write(data)
output := make([]byte, 64)
shake.Read(output)
```

##### FIPS 140-3 compliance [NEW in 1.24]

```bash
# Build with FIPS-approved algorithms only
GOFIPS140=v1.0.0 go build ./...

# Enable FIPS mode at runtime
GODEBUG=fips140=on ./myapp
```

No source code changes needed. The Go Cryptographic Module transparently uses FIPS-approved implementations.

##### crypto/rand improvements [NEW in 1.24]

```go
import "crypto/rand"

// crypto/rand.Read is now guaranteed not to fail — error return always nil
n, _ := rand.Read(buf)   // safe to ignore error

// NEW: Text — generate a cryptographically secure random string
// Uses base32 encoding (URL-safe, unambiguous alphabet)
token, err := rand.Text() // returns 26-character alphanumeric string
// e.g., "ABCDE23456FGHIJ78901KLMNO2"

// Use for: session IDs, CSRF tokens, API keys
sessionID, err := rand.Text()
```

##### TLS — Encrypted Client Hello and post-quantum [NEW in 1.24]

```go
// Encrypted Client Hello (ECH) — server side
cfg := &tls.Config{
    EncryptedClientHelloKeys: echKeys, // []tls.EncryptedClientHelloKey
}

// X25519MLKEM768 is now the DEFAULT key exchange (post-quantum)
// To disable (for compatibility with buggy servers):
cfg2 := &tls.Config{
    CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
}
// Or via GODEBUG:
// GODEBUG=tlsmlkem=0 ./myapp

// Fingerprint TLS clients via extension IDs
var trace httptrace.ClientTrace
trace.TLSHandshakeStart = func() {
    // ClientHelloInfo.Extensions now available for fingerprinting
}
```

##### crypto/ecdsa deterministic signing [NEW in 1.24]

```go
// Pass nil as rand to get deterministic RFC 6979 signature
sig, err := privKey.Sign(nil, digest, opts)
// Deterministic: same key + digest always produces same signature
// Useful for testing and audit logging
```

---

#### Testing & Benchmarking

##### testing.B.Loop [NEW in 1.24] ★★

Replaces the `for range b.N` pattern. Better in every way.

```go
// Old — common but has issues
func BenchmarkProcess(b *testing.B) {
    setup()           // called b.N times if in loop!
    for range b.N {   // setup + teardown inside loop is expensive per count
        Process(data)
    }
}

// NEW in 1.24 — preferred
func BenchmarkProcess(b *testing.B) {
    setup()           // called ONCE per -count
    for b.Loop() {    // loop is managed by the benchmark runner
        Process(data)
    }
    teardown()        // called ONCE per -count
}
```

**Advantages of `b.Loop()` over `for range b.N`:**
1. Setup/teardown outside the loop run **once per `-count`**, not b.N times.
2. The compiler keeps results alive — can't optimise away the loop body.
3. Cleaner, more readable API.

```go
// Benchmark with setup and teardown
func BenchmarkDB(b *testing.B) {
    db := setupTestDB(b)   // once
    defer db.Close()

    for b.Loop() {
        db.Query("SELECT 1")
    }
}

// Parallel benchmark
func BenchmarkParallel(b *testing.B) {
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() { Process(data) } // RunParallel still uses pb.Next()
    })
}
```

##### T.Chdir and B.Chdir [NEW in 1.24]

```go
func TestReadConfig(t *testing.T) {
    t.Chdir("testdata") // changes dir for this test, restores after
    // Now os.ReadFile("config.json") reads from testdata/config.json
    cfg, err := ReadConfig("config.json")
    if err != nil { t.Fatal(err) }
    _ = cfg
}
```

##### T.Context and B.Context [NEW in 1.24]

`T.Context`, `B.Context`, and `F.Context` are new in Go 1.24 (not available in
1.23). They return a context canceled after the test completes and before its
registered cleanup functions run.

```go
func TestWithContext(t *testing.T) {
    ctx := t.Context() // cancelled when test ends
    result, err := service.Call(ctx)
    if err != nil { t.Fatal(err) }
    _ = result
}

func BenchmarkWithContext(b *testing.B) {
    ctx := b.Context() // 1.24+: cancelled when benchmark ends
    for b.Loop() {
        service.Call(ctx)
    }
}
```

##### go vet — new test analyser [NEW in 1.24]

```go
// vet now catches:
func testFoo(t *testing.T) {}          // vet: should be TestFoo
func BenchmarkBar(b *testing.B, n int){} // vet: wrong signature
func ExampleBaz_() {}                  // vet: malformed example suffix

func ExampleBazDone() {                 // vet: ExampleBaz_Done not ExampleBazDone
    // (suffix after _ must be lowercase)
}
```

##### Test helpers

Helper functions should call `t.Helper()` so failures report the caller's line.

```go
func newTestUser(t *testing.T) *User {
    t.Helper()
    return &User{ID: uuid.New().String(), Name: "Test User", Email: "test@example.com"}
}

func assertNoError(t *testing.T, err error) {
    t.Helper()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}

func assertEqual[T comparable](t *testing.T, got, want T) {
    t.Helper()
    if got != want {
        t.Errorf("got %v; want %v", got, want)
    }
}
```

Prefer the built-in test lifecycle helpers over manual bookkeeping. In Go 1.24
all of these are available: `t.Helper()`, `t.TempDir()`, `t.Setenv()`,
`t.Cleanup()`, and the 1.24 additions `t.Context()`/`t.Chdir()` (see above).

##### Table-driven tests (1.22+: no shadow needed)

```go
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        got, err := Process(tt.input)
        if err != tt.wantErr { t.Fatalf("err=%v wantErr=%v", err, tt.wantErr) }
        if got != tt.want    { t.Errorf("got %v want %v", got, tt.want) }
    })
}
```

##### Fuzz testing

```go
func FuzzParse(f *testing.F) {
    f.Add("valid")
    f.Fuzz(func(t *testing.T, s string) { _, _ = Parse(s) })
}
// go test -fuzz=FuzzParse -fuzztime=30s
```

---

#### CLI Development & Tool Directive

##### Tool directive (replaces tools.go) [NEW in 1.24]

```bash
# Add tools to go.mod
go get -tool golang.org/x/tools/cmd/stringer@latest
go get -tool github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# Run
go tool stringer -type=Color .
go tool sqlc generate

# Upgrade all
go get tool

# Install to GOBIN
go install tool
```

##### flag

```go
host    := flag.String("host", "localhost", "server host")
port    := flag.Int("port", 8080, "server port")
verbose := flag.Bool("verbose", false, "verbose")
flag.BoolFunc("json", "JSON output", func(string) error { format = "json"; return nil })
flag.Parse()
```

##### Cobra

```go
var rootCmd = &cobra.Command{Use: "myapp"}
var serveCmd = &cobra.Command{
    Use:  "serve",
    RunE: func(cmd *cobra.Command, args []string) error {
        port, _ := cmd.Flags().GetInt("port")
        return startServer(port)
    },
}
func init() {
    serveCmd.Flags().IntP("port", "p", 8080, "port")
    serveCmd.MarkFlagRequired("port")
    rootCmd.AddCommand(serveCmd)
}
func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err); os.Exit(1)
    }
}
```

##### Clean main

```go
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
        os.Exit(1)
    }
}
func run() error { return nil }
```

---

#### Performance Caveats & PGO

##### Swiss Tables maps — transparent 2–30%+ improvement

No code changes needed. All `map` and `sync.Map` operations benefit automatically in Go 1.24.

##### PGO (GA since 1.21, reduced overhead in 1.23)

```bash
# Collect profile from production
go build -o myapp ./cmd/myapp
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cmd/myapp/default.pgo
# Rebuild with auto-PGO
go build ./cmd/myapp
```

PGO overhead at build time is now in single-digit percentages for large builds (improved in 1.23).

##### B.Loop replaces for range b.N

Use `b.Loop()` in all new benchmarks — prevents the compiler from eliding the loop body and ensures setup/teardown costs are not included in per-iteration measurement.

##### strings/bytes *Seq functions — no allocation

```go
// Prefer lazy iterators for large inputs
for part := range strings.SplitSeq(largeString, ",") {
    process(part) // no []string allocation
}

// Over the allocating form:
for _, part := range strings.Split(largeString, ",") { // allocates []string
    process(part)
}
```

##### slog.DiscardHandler in tests

```go
// Eliminates log overhead in benchmarks
b.ResetTimer()
slog.SetDefault(slog.New(slog.DiscardHandler{}))
for b.Loop() { processWithLogging(data) }
```

##### escape analysis

```bash
go build -gcflags="-m" ./...
```

##### sync.Pool

```go
var bufPool = sync.Pool{New: func() any { return &bytes.Buffer{} }}
func process(data []byte) string {
    buf := bufPool.Get().(*bytes.Buffer); buf.Reset()
    defer bufPool.Put(buf)
    buf.Write(data)
    return buf.String()
}
```

##### Struct layout

```go
type Bad    struct { A bool; B int64; C bool } // 24 bytes
type Better struct { B int64; A bool; C bool } // 16 bytes
```

---

#### Idioms & Things to Avoid

##### Do (1.24 additions)

- **Use generic type aliases** to re-export and simplify internal generic types.
- **Use `weak.Make(obj)`** for cache values — GC can reclaim without explicit eviction.
- **Use `runtime.AddCleanup`** instead of `runtime.SetFinalizer` for all new cleanup logic.
- **Use `os.Root`** to sandbox any filesystem access to untrusted user-supplied paths.
- **Use `omitzero`** for JSON struct fields where zero values (especially `time.Time`) should be omitted.
- **Use `b.Loop()`** in all new benchmarks.
- **Use `strings.SplitSeq`, `Lines`, `FieldsSeq`** for large-input string splitting — no slice allocation.
- **Use `go tool <name>` and the `tool` directive** instead of `tools.go` blank-import pattern.
- **Use `slog.DiscardHandler{}`** to silence logging in tests and benchmarks.
- **Use `crypto/rand.Text()`** for generating random tokens.
- **Use `crypto/hkdf.Key`, `crypto/pbkdf2.Key`, `crypto/sha3`** instead of `golang.org/x/crypto` equivalents.
- **Use `hash/maphash.Comparable`** to hash arbitrary comparable values.
- **Use `t.Chdir(dir)`** in tests that depend on working directory.

##### Don't

| Anti-pattern | Why | Instead |
|---|---|---|
| `runtime.SetFinalizer` for new code | Limited, cycle-unsafe | `runtime.AddCleanup` |
| Manual cache eviction with `sync.Map` | Verbose, leaks memory on GC | `weak.Pointer` + `AddCleanup` |
| `filepath.Join(base, user)` for sandboxing | Symlink attacks possible | `os.OpenRoot(base).Open(user)` |
| `json:",omitempty"` for time.Time fields | Never omits zero time | `json:",omitzero"` |
| `for range b.N { ... }` in benchmarks | Setup inside loop counted; compiler may elide | `for b.Loop() { ... }` |
| `tools.go` blank import pattern | Obsolete since 1.24 | `go.mod` `tool` directive |
| `rand.Seed()` | No-op since 1.24 (was deprecated since 1.20) | `rand.New(rand.NewPCG(s1,s2))` |
| `strings.Split` for large inputs | Allocates full `[]string` | `strings.SplitSeq` |
| `var m map[string]int` | Nil, writes panic | `make(map[string]int)` |
| Log AND return error | Double-logging | Return; log at boundary |
| `panic` for business logic | Crashes | `return error` |
| Copying a mutex | Broken | Embed + pointer receiver |
| Closing channel from receiver | Panic | Sender closes |
| String `+` in loops | O(n^2) | `strings.Builder` |
| Storing context in struct | Anti-pattern | First parameter |

##### Reachability and domain bounds

Only report overflow, panic, or invalid-value behavior when the failing path is
reachable for the declared type and the function's input domain.

- Do not claim byte formatting can index into ZiB/YiB units when an `int64`
  input cannot grow large enough to reach those unit indexes.
- Do not require negative byte handling when all callers pass values from
  `os.FileInfo.Size()` or another source with a non-negative contract.
- Do report missing negative handling when an exported/general-purpose function
  accepts user-controlled values and documents no narrower domain.
- Do not require defensive code for values that cannot be represented by the
  input type.

##### Security

Weak hashes are only security-relevant when the hash is used for a security
property such as authentication, authorization, integrity, signatures, password
storage, or collision resistance against attacker-controlled input.

For content-addressed storage, distinguish integrity/security boundaries from
ordinary maintenance behavior.

- Do not require hash verification on every listing unless the listing crosses
  a trust boundary or an attacker can write to the store.
- Delete-before-regenerate can be correct when replacing corrupt same-digest
  content and the store skips writes for content that already exists.
- Report content-addressed storage issues when untrusted data can be accepted
  under the wrong digest, when corruption is silently trusted as valid content,
  or when the repair order can lose the only valid copy.

##### Naming conventions

```go
type UserRepository interface{}  // PascalCase
type httpClient struct{}          // camelCase
type HTTPClient struct{}          // acronyms all-caps
var ErrNotFound = errors.New("not found")
type Stringer interface{ String() string }
func (u *User) IsAdmin() bool {}
```

##### Formatting & linting

```bash
gofmt -w .
goimports -w .
go vet ./...          # now includes: tests analyser, fmt.Printf(s), buildtag point-release
staticcheck ./...
golangci-lint run
go tool stringer ...  # if using tool directive
```

A `golangci-lint` v2 configuration (in Go 1.24 you can also pin and run it via a
`tool` directive: `go get -tool github.com/golangci/golangci-lint/cmd/golangci-lint`
then `go tool golangci-lint run`):

```yaml
# .golangci.yml (golangci-lint v2)
version: "2"

linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck # includes the former gosimple checks
    - unused
    - gocritic
  settings:
    govet:
      enable:
        - shadow
    errcheck:
      check-type-assertions: true
  exclusions:
    rules:
      - path: _test\.go
        linters:
          - errcheck

formatters:
  enable:
    - gofmt
    - goimports
```

---

*End of Go 1.24 Complete Developer Guideline*
