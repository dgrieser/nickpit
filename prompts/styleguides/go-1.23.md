### Go 1.23 — Complete Developer Guideline

> **Version**: Go 1.23.0 (released 2024-08-13)
> **Scope**: Full language spec, all 1.23 changes delta over 1.22, idioms, concurrency, performance, CLI, file I/O, testing, and best practices — with examples throughout.
> **Note**: This document is self-contained. All relevant prior-version material is carried forward or superseded. Sections marked **[NEW in 1.23]** cover additions specific to this version.

---

#### Table of Contents

1. [What's New in Go 1.23](#whats-new-in-go-123)
2. [Project Layout & Modules](#project-layout--modules)
3. [Basic Types & Variables](#basic-types--variables)
4. [Pointers](#pointers)
5. [Control Flow](#control-flow)
6. [Functions](#functions)
7. [Defer, Panic & Recover](#defer-panic--recover)
8. [Structs & Methods](#structs--methods)
9. [Interfaces & Embedding](#interfaces--embedding)
10. [Generics](#generics)
11. [Iterators — iter package](#iterators--iter-package)
12. [Collection Types: Arrays, Slices, Maps](#collection-types-arrays-slices-maps)
13. [Strings](#strings)
14. [Error Handling](#error-handling)
15. [Structured Logging with log/slog](#structured-logging-with-logslog)
16. [Concurrency](#concurrency)
17. [Context Package](#context-package)
18. [File I/O & Streaming](#file-io--streaming)
19. [HTTP Servers](#http-servers)
20. [Testing & Benchmarking](#testing--benchmarking)
21. [CLI Development](#cli-development)
22. [Performance Caveats & PGO](#performance-caveats--pgo)
23. [Idioms & Things to Avoid](#idioms--things-to-avoid)

---

#### What's New in Go 1.23

Go 1.23.0 shipped 2024-08-13. Its headline feature is **range-over-function iterators**, which reached stable language status after previewing in 1.22. The release also ships the new `iter`, `unique`, and `structs` packages, significant timer behaviour fixes, and important compiler improvements.

##### Language changes

###### 1. Range-over-function iterators [NEW in 1.23] ★★★

`for-range` now accepts iterator functions as range expressions. This is the most significant language addition since generics.

**Three iterator shapes are supported:**

```go
// Shape 1: no yielded value
func(yield func() bool)

// Shape 2: one yielded value
func(yield func(V) bool)

// Shape 3: two yielded values (key/value pair)
func(yield func(K, V) bool)
```

The `yield` function is called for each iteration. The loop body runs as the body of `yield`. Returning `false` from `yield` stops iteration (equivalent to `break`).

**Simple example — backward iteration:**

```go
import "slices"

s := []string{"hello", "world", "go"}
for i, v := range slices.Backward(s) {
    fmt.Println(i, v)
}
// 2 go
// 1 world
// 0 hello
```

**Writing your own iterator:**

```go
import "iter"

// Single-value iterator (iter.Seq[V])
func IntsFrom(start, step, n int) iter.Seq[int] {
    return func(yield func(int) bool) {
        for i := range n {
            if !yield(start + i*step) {
                return // break in the loop
            }
        }
    }
}

for v := range IntsFrom(10, 5, 4) {
    fmt.Println(v) // 10, 15, 20, 25
}

// Two-value iterator (iter.Seq2[K, V])
func Enumerate[V any](s []V) iter.Seq2[int, V] {
    return func(yield func(int, V) bool) {
        for i, v := range s {
            if !yield(i, v) { return }
        }
    }
}

for i, v := range Enumerate([]string{"a", "b", "c"}) {
    fmt.Printf("%d=%s\n", i, v)
}
```

**Control flow in iterator loops works normally:**

```go
for v := range myIter() {
    if v > 10 { break }       // stops iteration cleanly
    if v < 0  { continue }    // skips to next iteration
}

func findFirst(seq iter.Seq[int], target int) (int, bool) {
    for v := range seq {
        if v == target { return v, true } // return exits loop and function
    }
    return 0, false
}
```

**Important**: `panic` and `return` inside a loop body are handled correctly. `defer` inside a loop body executes when the *surrounding function* returns, not when the iterator exits.

###### 2. Generic type aliases (preview) [NEW in 1.23]

Available with `GOEXPERIMENT=aliastypeparams`. Cross-package use not yet supported. Becomes stable in 1.24.

```go
// With GOEXPERIMENT=aliastypeparams:
type MySlice[T any] = []T  // generic type alias — preview only in 1.23
```

##### Timer behaviour changes [NEW in 1.23] ★★

Two breaking-but-correct changes to `time.Timer` and `time.Ticker`. Only active for modules with `go 1.23.0` or later in `go.mod`.

###### Change 1: GC-eligible immediately

Before 1.23: an unstopped `Timer` would not be garbage-collected until it fired. An unstopped `Ticker` was *never* collected.

After 1.23: both are GC-eligible as soon as no references exist, even if `Stop` was never called.

```go
// Before 1.23: this leaked until the timer fired
t := time.NewTimer(24 * time.Hour)
// ... t goes out of scope — not collected until 24h later

// After 1.23: collected whenever GC runs — no leak
t := time.NewTimer(24 * time.Hour)
// still good practice to call t.Stop() when done, but no longer a leak
```

###### Change 2: Timer channels are now unbuffered (capacity 0)

Before 1.23: timer channels had buffer size 1, making correct `Reset`/`Stop` patterns difficult.

After 1.23: timer channels have capacity 0. `Reset` and `Stop` are now guaranteed safe — no stale values can arrive after the call.

```go
// Old code that polled len() — BROKEN in 1.23
t := time.NewTimer(time.Second)
if len(t.C) > 0 {       // was sometimes 1 before fire; now always 0
    <-t.C               // no longer reliable
}

// Correct pattern in 1.23: non-blocking receive
t := time.NewTimer(time.Second)
select {
case <-t.C:
    // timer fired
default:
    // timer not yet fired
}

// Reset — now always safe without draining first
t.Reset(5 * time.Second) // correct in 1.23+

// Old Reset pattern — no longer needed in 1.23
// if !t.Stop() { <-t.C }  // remove this in 1.23 modules
// t.Reset(d)
```

**Backwards compatibility:**
```bash
GODEBUG=asynctimerchan=1 ./myapp  # restore 1.22 buffered timer channels
```

##### GODEBUG in go.mod [NEW in 1.23]

`go.mod` and `go.work` now support a `godebug` directive:

```
module myapp

go 1.23.0

godebug asynctimerchan=0  // explicitly opt in to new timer behaviour
godebug panicnil=0        // explicitly use 1.21+ panic(nil) semantics
```

This makes GODEBUG settings part of your module definition rather than requiring environment variables in deployment scripts.

##### Toolchain

###### go vet: stdversion analyser [NEW in 1.23]

```bash
# Flags references to symbols introduced in a newer Go than your go.mod declares
go vet ./...
```

```go
// If go.mod says go 1.21 but you reference reflect.TypeFor (added in 1.22):
t := reflect.TypeFor[int]() // vet: "reflect.TypeFor requires go1.22 or later"
```

This is extremely useful when maintaining libraries that support multiple Go versions.

###### go telemetry [NEW in 1.23]

```bash
go telemetry on    # opt in to anonymous usage reporting
go telemetry off   # opt out
go telemetry local # default: collect locally only, not uploaded
```

###### go mod tidy -diff [NEW in 1.23]

```bash
# Show what tidy would change without modifying files
go mod tidy -diff
# Useful in CI: fail if go.mod/go.sum are not tidy
go mod tidy -diff || (echo "go.mod/go.sum not tidy" && exit 1)
```

###### go env -changed [NEW in 1.23]

```bash
# Show only environment settings that differ from defaults
go env -changed
```

##### Compiler

- PGO build-time overhead reduced from 100%+ to **single-digit percentages** for large builds.
- Stack frame slot overlapping: local variables accessed in disjoint regions now share stack slots — **reduced stack usage**.
- Hot block alignment for 386/amd64: 1–1.5% additional performance from PGO.
- Linker now restricts `//go:linkname` to access internal stdlib symbols — prevents accidental reliance on private implementations.

##### Standard Library — new packages

| Package | Purpose |
|---|---|
| `iter` | `Seq[V]` and `Seq2[K,V]` types; `Pull`/`Pull2` converters |
| `unique` | Value canonicalisation/interning via `Handle[T]` |
| `structs` | Struct field types that control memory layout (`HostLayout`) |

##### Standard Library — selected changes

| Package | Change |
|---|---|
| `slices` | `All`, `Values`, `Backward`, `Collect`, `AppendSeq`, `Sorted`, `SortedFunc`, `SortedStableFunc`, `Chunk`, `Repeat` |
| `maps` | `All`, `Keys`, `Values`, `Insert`, `Collect` |
| `sync` | `Map.Clear` |
| `sync/atomic` | `And`, `Or` bitwise operations |
| `os` | `CopyFS` |
| `path/filepath` | `Localize` |
| `testing/fstest` | `TestFS` returns an unwrappable structured error |
| `math/rand/v2` | `Uint`, `Rand.Uint`, `ChaCha8.Read` |
| `net/http` | `Request.Pattern`, `ParseCookie`, `ParseSetCookie`, `Cookie.Quoted`, `Request.CookiesNamed` |
| `encoding/binary` | `Encode`, `Decode`, `Append` |
| `text/template` | `else with` action |
| `runtime/debug` | `SetCrashOutput` |
| `go/ast` | `Preorder` — iterator over syntax tree nodes |

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
├── go.mod
└── go.sum
```

##### go.mod with 1.23 features

```
module github.com/yourname/myapp

go 1.23.0

toolchain go1.23.3

godebug asynctimerchan=0  // use new timer semantics
```

Setting `go 1.23.0` activates:
- Per-iteration loop variables (since 1.22).
- New timer channel behaviour (unbuffered, GC-eligible).

##### Verify library APIs against the pinned version

Verify library APIs against the actual module versions in `go.mod` before
claiming an API is missing or unavailable.

- Do not claim a controller-runtime helper is unavailable without checking the
  pinned `sigs.k8s.io/controller-runtime` version.
- Say "this API is not available in vX.Y.Z" only when the module version proves
  it.
- Do not infer API availability from memory, a newer version's docs, or another
  project's dependency set.

---

#### Basic Types & Variables

##### Numeric types

| Type | Size | Notes |
|---|---|---|
| `int8`–`int64`, `uint8`–`uint64` | 1–8 bytes | Fixed-width |
| `int`, `uint` | platform | Indexing and general counts |
| `float32`, `float64` | 4, 8 bytes | IEEE 754 |
| `byte` = `uint8`, `rune` = `int32` | — | Aliases |

```go
var x int = 10
y := "hello"
z := 3.14

m := min(x, 20)  // 10 (1.21+)
M := max(x, 20)  // 20 (1.21+)
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
    Read    Permission = 1 << iota // 1
    Write                          // 2
    Execute                        // 4
)
```

---

#### Pointers

```go
x := 42
p := &x
fmt.Println(*p) // 42
*p = 99

p1 := new(int)
p2 := &Point{X: 1, Y: 2}
```

##### Loop variable capture — solved in 1.22+

```go
// Safe with go 1.22+ in go.mod — no shadow needed
for i := 0; i < 3; i++ {
    go func() { fmt.Println(i) }()
}
```

---

#### Control Flow

##### if / else

```go
if x > 0 { fmt.Println("positive") }
if err := doSomething(); err != nil { return err }
// No else after return — idiomatic
f, err := os.Open(name)
if err != nil { return err }
use(f)
```

##### for — all forms

```go
for i := 0; i < 10; i++ {}
for x < 100 { x *= 2 }
for { if done() { break } }
for i, v := range s {}
for k, v := range m {}
for i, r := range "héllo" {}
for v := range ch {}
for i := range 10 {} // 1.22+: range over integer

// NEW in 1.23: range over iterator function
for v := range myIter() {}
for k, v := range myIter2() {}
```

##### switch

```go
switch x {
case 1:    fmt.Println("one")
case 2, 3: fmt.Println("two or three")
default:   fmt.Println("other")
}

// No condition
switch {
case x < 0:  fmt.Println("negative")
case x == 0: fmt.Println("zero")
default:     fmt.Println("positive")
}

// Type switch
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
case msg := <-ch1:              fmt.Println("received", msg)
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

// time.Since must be in closure (1.22 vet catches this)
defer func() { log.Println(time.Since(start)) }()

// panic(nil) — 1.21+: recover() is non-nil for any panic
defer func() {
    if r := recover(); r != nil { /* a panic occurred */ }
}()
```

**Defer inside iterator loops**: `defer` inside a `for-range` iterator loop runs when the *surrounding function* returns, not when the iterator stops. Avoid `defer` inside tight iterator loops; use explicit cleanup instead.

```go
// WRONG: defer runs after the whole function, not after each iteration
for path := range filePaths() {
    f, _ := os.Open(path)
    defer f.Close()       // all deferred until function return!
}

// CORRECT: explicit close each iteration
for path := range filePaths() {
    func() {
        f, err := os.Open(path)
        if err != nil { return }
        defer f.Close()   // runs when anonymous func returns
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
| Any method uses `*T` | Use `*T` for ALL |

##### structs.HostLayout [NEW in 1.23]

Use when passing structs to C/OS APIs that require a specific memory layout:

```go
import "structs"

// HostLayout signals that this struct's field order must match
// the host platform's ABI expectations (e.g., for C interop).
type NetworkPacketHeader struct {
    _ structs.HostLayout // embed to enforce host memory layout
    Version  uint8
    Type     uint8
    Length   uint16
    Checksum uint32
}
```

Without `HostLayout`, the Go spec does not guarantee field order in memory (even though current implementations happen to match).

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

type File struct{}
func (f *File) Write(p []byte) (int, error) { return len(p), nil }
func (f *File) Close() error                { return nil }
var wc WriteCloser = &File{}

var x any = "hello"
s, ok := x.(string)

switch v := x.(type) {
case int:    fmt.Println("int", v)
case string: fmt.Println("string", v)
}
```

---

#### Generics

```go
func Map[T, U any](s []T, f func(T) U) []U {
    result := make([]U, len(s))
    for i, v := range s { result[i] = f(v) }
    return result
}

func Contains[T comparable](s []T, target T) bool {
    for _, v := range s { if v == target { return true } }
    return false
}

import "cmp"
func MinVal[T cmp.Ordered](a, b T) T {
    if a < b { return a }
    return b
}

// cmp.Or — first non-zero value (1.22+)
host := cmp.Or(os.Getenv("HOST"), cfg.Host, "localhost")
```

---

#### Iterators — iter package

The `iter` package (new in 1.23) defines the canonical iterator types and helpers.

##### Types

```go
import "iter"

// Seq[V] — single-value push iterator
type Seq[V any] func(yield func(V) bool)

// Seq2[K, V] — two-value push iterator (key/value)
type Seq2[K, V any] func(yield func(K, V) bool)
```

##### Writing iterators

```go
// Seq — single value
func Naturals() iter.Seq[int] {
    return func(yield func(int) bool) {
        for i := 0; ; i++ {
            if !yield(i) { return }
        }
    }
}

// Seq2 — key/value
func MapSorted[K cmp.Ordered, V any](m map[K]V) iter.Seq2[K, V] {
    return func(yield func(K, V) bool) {
        keys := make([]K, 0, len(m))
        for k := range m { keys = append(keys, k) }
        slices.Sort(keys)
        for _, k := range keys {
            if !yield(k, m[k]) { return }
        }
    }
}

// Usage
for k, v := range MapSorted(m) { fmt.Println(k, v) }
```

##### Custom collection with iterator methods

```go
type OrderedSet[T cmp.Ordered] struct{ items []T }

func (s *OrderedSet[T]) Add(v T) {
    i, found := slices.BinarySearch(s.items, v)
    if !found { s.items = slices.Insert(s.items, i, v) }
}

// All returns a Seq — enables for range
func (s *OrderedSet[T]) All() iter.Seq[T] {
    return slices.Values(s.items) // slices.Values is a Seq function (1.23+)
}

func (s *OrderedSet[T]) Backward() iter.Seq[T] {
    return func(yield func(T) bool) {
        for i := len(s.items) - 1; i >= 0; i-- {
            if !yield(s.items[i]) { return }
        }
    }
}

// Consumer code
os := &OrderedSet[int]{}
os.Add(3); os.Add(1); os.Add(4)
for v := range os.All() { fmt.Println(v) } // 1, 3, 4
```

##### Pull iterators — iter.Pull and iter.Pull2

Sometimes you need to drive iteration manually (zip, merge, etc.). Use `iter.Pull` to convert a push iterator to a pull iterator:

```go
// Pull converts a Seq to a (next, stop) pair
next, stop := iter.Pull(mySeq)
defer stop() // MUST call stop when done — even after early exit

for {
    v, ok := next()
    if !ok { break }
    process(v)
}

// Pull2 for Seq2
next2, stop2 := iter.Pull2(mySeq2)
defer stop2()
for {
    k, v, ok := next2()
    if !ok { break }
    process(k, v)
}
```

**Always `defer stop()`** — if you don't call `stop`, the iterator goroutine (if any) may leak.

##### Combining iterators

```go
// Zip two sequences
func Zip[T1, T2 any](s1 iter.Seq[T1], s2 iter.Seq[T2]) iter.Seq2[T1, T2] {
    return func(yield func(T1, T2) bool) {
        n1, stop1 := iter.Pull(s1)
        defer stop1()
        n2, stop2 := iter.Pull(s2)
        defer stop2()
        for {
            v1, ok1 := n1()
            v2, ok2 := n2()
            if !ok1 || !ok2 { return }
            if !yield(v1, v2) { return }
        }
    }
}

// Filter
func Filter[V any](seq iter.Seq[V], keep func(V) bool) iter.Seq[V] {
    return func(yield func(V) bool) {
        for v := range seq {
            if keep(v) {
                if !yield(v) { return }
            }
        }
    }
}

// Map
func MapSeq[T, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U] {
    return func(yield func(U) bool) {
        for v := range seq {
            if !yield(f(v)) { return }
        }
    }
}
```

##### Streaming database queries with iterators

```go
func QueryUsers(ctx context.Context, db *sql.DB) iter.Seq2[User, error] {
    return func(yield func(User, error) bool) {
        rows, err := db.QueryContext(ctx, "SELECT id, name FROM users")
        if err != nil { yield(User{}, err); return }
        defer rows.Close()
        for rows.Next() {
            var u User
            if err := rows.Scan(&u.ID, &u.Name); err != nil {
                yield(User{}, err); return
            }
            if !yield(u, nil) { return }
        }
        if err := rows.Err(); err != nil { yield(User{}, err) }
    }
}

// Consume — streams rows without loading all into memory
for user, err := range QueryUsers(ctx, db) {
    if err != nil { return err }
    fmt.Println(user.ID, user.Name)
}
```

##### Performance of iterators

The compiler inlines and optimises iterator loops aggressively. For simple iterators over slices, the compiled output is equivalent to a hand-written loop. No goroutines are involved in push iterators — the `yield` function call is just a function call in the same goroutine. Pull iterators (`iter.Pull`) do use goroutines internally.

---

#### Collection Types: Arrays, Slices, Maps

##### Arrays

```go
b := [3]int{1, 2, 3}
c := [...]int{4, 5, 6}
s := []byte{1, 2, 3, 4, 5}
arr := [4]byte(s) // 1.20+: value copy; panics if len(s) < 4
```

##### Slices

```go
s := []int{1, 2, 3}
s2 := make([]int, 0, 10)
c := make([]int, len(s)); copy(c, s) // independent copy
result := make([]string, 0, len(items))
for _, item := range items { result = append(result, process(item)) }
```

##### slices package (1.21+, updated in 1.22 and 1.23)

```go
import "slices"

// Sorting
slices.Sort(s)
slices.SortFunc(s, func(a, b T) int { return cmp.Compare(a, b) })
slices.SortStableFunc(s, cmpFn)

// Search
idx, found := slices.BinarySearch(sorted, v)
slices.Contains(s, v)
slices.Index(s, v)
slices.ContainsFunc(s, fn)
slices.IndexFunc(s, fn)

// Transformation
slices.Reverse(s)               // in-place
slices.Compact(s)               // remove consecutive duplicates, zeroes freed
slices.DeleteFunc(s, fn)        // filter out, zeroes freed (1.22+)
slices.Clip(s)                  // trim spare capacity
slices.Grow(s, n)               // ensure cap >= len+n
slices.Insert(s, i, vs...)      // insert at index
slices.Delete(s, i, j)         // delete range, zeroes freed (1.22+)
slices.Replace(s, i, j, vs...) // replace, zeroes freed (1.22+)
slices.Concat(ss...)            // 1.22+: concatenate with exact allocation

// Min/Max
slices.Max(s); slices.Min(s)
slices.MaxFunc(s, fn); slices.MinFunc(s, fn)

// NEW in 1.23: Iterator-returning functions
slices.All(s)            // iter.Seq2[int, E] — index + value
slices.Values(s)         // iter.Seq[E] — value only
slices.Backward(s)       // iter.Seq2[int, E] — reverse index + value
slices.Chunk(s, n)       // iter.Seq[[]E] — consecutive sub-slices of ≤n elements

// NEW in 1.23: Iterator-consuming functions
slices.Collect(seq)           // collect Seq[E] into new []E
slices.AppendSeq(s, seq)      // append values from Seq[E] to existing slice
slices.Sorted(seq)            // Collect + Sort
slices.SortedFunc(seq, fn)    // Collect + SortFunc
slices.SortedStableFunc(seq, fn)

// NEW in 1.23: Repeat
slices.Repeat(s, n)  // new slice containing s repeated n times
// e.g., slices.Repeat([]int{1,2}, 3) == []int{1,2,1,2,1,2}
```

##### Chunk example

```go
s := []int{1, 2, 3, 4, 5, 6, 7}
for chunk := range slices.Chunk(s, 3) {
    fmt.Println(chunk)
}
// [1 2 3]
// [4 5 6]
// [7]
```

##### maps package (1.21+, updated in 1.23)

```go
import "maps"

maps.Clone(m)
maps.Copy(dst, src)
maps.DeleteFunc(m, fn)
maps.Equal(m1, m2)
maps.EqualFunc(m1, m2, fn)

// NEW in 1.23: Iterator-returning functions
maps.All(m)     // iter.Seq2[K, V] — key-value pairs
maps.Keys(m)    // iter.Seq[K] — keys only
maps.Values(m)  // iter.Seq[V] — values only

// NEW in 1.23: Iterator-consuming functions
maps.Insert(m, seq)      // add key-value pairs from Seq2 to existing map
maps.Collect(seq)        // collect Seq2[K,V] into a new map

// Build a map from filtered entries
filtered := maps.Collect(
    Filter2(maps.All(m), func(k string, v int) bool { return v > 0 }),
)
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

---

#### unique package [NEW in 1.23]

Canonicalises (interns) values of any comparable type. Two `Handle[T]` are equal iff the values that created them are equal — and comparison is a single pointer comparison.

```go
import "unique"

// Make — returns Handle[T] for the canonical copy of the value
h1 := unique.Make("hello")
h2 := unique.Make("hello")
h3 := unique.Make("world")

fmt.Println(h1 == h2)  // true  — same canonical string
fmt.Println(h1 == h3)  // false

// Value — retrieve the value
fmt.Println(h1.Value()) // "hello"

// Works for structs
type IPPort struct{ IP net.IP; Port int }
h := unique.Make(IPPort{...})
```

##### When to use unique

- **Large repeated strings** (hostnames, IP addresses, tags, enum-like strings) — internment saves memory and makes equality O(1) instead of O(len).
- **Cache keys** — use `Handle[T]` as map keys for efficient lookup.
- **Replacing `sync.Map` string caches** — `unique.Make` is the idiomatic replacement.

```go
// Before: manual interning with sync.Map
var internPool sync.Map
func intern(s string) string {
    actual, _ := internPool.LoadOrStore(s, s)
    return actual.(string)
}

// After: unique.Make
func intern(s string) unique.Handle[string] {
    return unique.Make(s)
}
```

---

#### Strings

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
strings.Split("a,b,c", ",")
strings.Join([]string{"a","b"}, "-")
strings.TrimSpace("  hello  ")
strings.ReplaceAll("aabbcc", "b", "x")
after, found := strings.CutPrefix("Gopher", "Go")   // 1.20+
before, found := strings.CutSuffix("Gopher", "er")  // 1.20+
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

combined := errors.Join(err1, err2)                       // 1.20+
wrapped  := fmt.Errorf("two: %w and %w", err1, err2)     // 1.20+
if errors.Is(err, errors.ErrUnsupported) { /* fallback */ } // 1.21+
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

slog.Info("user logged in", "userID", 42, "ip", "1.2.3.4")
slog.Warn("high latency", "ms", 312)
slog.Error("connection failed", "err", err)

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo, AddSource: true,
}))
slog.SetDefault(logger)

// Type-safe attrs
slog.Info("request",
    slog.String("method", "GET"),
    slog.Int("status", 200),
    slog.Duration("latency", 12*time.Millisecond),
)

// Child logger
reqLogger := logger.With("requestID", reqID)
reqLogger.Info("handler done", "duration", time.Since(start))

// WithGroup
dbLogger := logger.WithGroup("db").With("host", "localhost")

// Context-aware
slog.InfoContext(ctx, "processing", "id", id)
```

##### slog best practices

- **Always use structured key-value pairs** — never format strings into the message.
- **Keys should be lowercase snake_case** for consistent log querying.
- **Pass `context.Context` through** and use `InfoContext`/`ErrorContext` for trace IDs.
- **Group related attributes** with `slog.Group` or `logger.WithGroup`.
- **Never log secrets, tokens, credentials, or full request/response bodies** unless they are explicitly scrubbed.

---

#### Concurrency

##### Goroutines

```go
// Safe in 1.22+ (go 1.22+ in go.mod):
for i := range 5 { go func() { fmt.Println(i) }() }
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
```

**Timer channels in 1.23:** unbuffered (capacity 0), always use non-blocking receive with `select`.

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

##### sync.Map — with Clear [NEW in 1.23]

```go
var m sync.Map
m.Store("k", "v")
v, ok := m.Load("k")
old, loaded := m.Swap("k", "new")
ok = m.CompareAndSwap("k", "old", "new")
ok = m.CompareAndDelete("k", "value")
m.Range(func(k, v any) bool { return true })
m.Clear() // NEW in 1.23: delete all entries at once
```

##### sync/atomic — And and Or [NEW in 1.23]

```go
var flags atomic.Uint32

// Set bit 2 (OR): returns old value
old := flags.Or(0b0100)

// Clear bit 2 (AND with mask): returns old value
old = flags.And(^uint32(0b0100))

// These are equivalent to:
// old = flags.Load(); flags.Store(old | mask)  (but atomic)
```

##### sync.Once helpers (1.21+)

```go
initDB := sync.OnceFunc(func() { db = openDB() })
getConfig := sync.OnceValue(func() *Config { return loadConfig() })
getConn := sync.OnceValues(func() (*sql.DB, error) { return sql.Open("postgres", dsn) })
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

##### Timer patterns in 1.23

```go
// Creating timers
t := time.NewTimer(5 * time.Second)
defer t.Stop()

// Receiving — always use select for non-blocking check
select {
case <-t.C:
    fmt.Println("fired")
default:
    fmt.Println("not yet")
}

// Reset — safe in 1.23 without draining first
t.Reset(10 * time.Second) // correct in 1.23+ modules

// Ticker
tick := time.NewTicker(time.Second)
defer tick.Stop()
for range 5 {
    <-tick.C
    fmt.Println("tick")
}

// time.After — watch for GC with 1.23 semantics:
// timer is GC-eligible immediately when select/receive completes
select {
case result := <-workCh:
    use(result)
case <-time.After(10 * time.Second):
    return errors.New("timeout")
}
```

##### Concurrency pitfalls

| Pitfall | Fix |
|---|---|
| Goroutine leak | Context cancellation; always provide exit |
| Race condition | `go test -race ./...` |
| Deadlock | No circular waits |
| Timer leak (< 1.23) | Always Stop; in 1.23: GC-eligible automatically |
| Polling `len(t.C)` | Use `select` with `default` |
| Old Reset pattern | Remove drain-before-reset in 1.23 modules |
| Loop variable capture (< 1.22) | Shadow `i := i` or pass as arg |

```bash
go test -race ./...  # always in CI
```

##### Classifying races before reporting

Separate true data races from lifecycle, shutdown, or ordering races. Only call
something a data race after confirming unsynchronized shared-memory access with
at least one write.

Check a library's concurrency contract before assuming concurrent method calls
are unsafe. Some Go types are explicitly safe for concurrent use, while others
require caller-side synchronization.

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
ctx, cancel  = context.WithTimeoutCause(parent, 5*time.Second, reason) // 1.21+
stop         := context.AfterFunc(ctx, cleanup)                     // 1.21+
```

---

#### File I/O & Streaming

```go
f, err := os.Open("file.txt")
if err != nil { return err }
defer f.Close()

w := bufio.NewWriter(f); defer w.Flush()
scanner := bufio.NewScanner(f)
scanner.Buffer(make([]byte, 1<<20), 1<<20)
for scanner.Scan() { process(scanner.Text()) }
if err := scanner.Err(); err != nil { return err }

data, err := os.ReadFile("small.json")
io.Copy(dst, src)
ow := io.NewOffsetWriter(file, 512) // 1.20+
```

##### os.CopyFS [NEW in 1.23]

```go
// Copy an fs.FS to the local filesystem
err := os.CopyFS("./output", os.DirFS("./source"))

// Copy embedded files to disk
//go:embed templates/*
var templates embed.FS
err = os.CopyFS("./dist/templates", templates)
```

##### path/filepath.Localize [NEW in 1.23]

```go
// Convert a slash-separated path to the OS path separator — safely
// Returns error if path would escape or is not representable
local, err := filepath.Localize("subdir/file.txt")
// On Windows: "subdir\file.txt"
// On Unix:    "subdir/file.txt"

// Compare to filepath.FromSlash — Localize also validates
filepath.FromSlash("../secret") // just replaces /, no validation
filepath.Localize("../secret")  // error: path escapes parent
```

##### encoding/binary — Encode/Decode/Append [NEW in 1.23]

```go
import "encoding/binary"

// Encode — like Write but to a []byte (no io.Writer needed)
buf, err := binary.Append(nil, binary.LittleEndian, uint32(42))

// Append multiple values
buf, err = binary.Append(buf, binary.LittleEndian, float32(3.14), uint16(7))

// Decode — like Read but from []byte
var v uint32
err = binary.Decode(buf, binary.LittleEndian, &v)
```

##### JSON

```go
enc := json.NewEncoder(w); enc.SetIndent("", "  "); enc.Encode(data)
dec := json.NewDecoder(r); dec.Decode(&result)
type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
    Pass string `json:"-"`
    Age  int    `json:"age,omitempty"`
}
```

---

#### HTTP Servers

##### Enhanced ServeMux (1.22+)

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    fmt.Fprintf(w, "user: %s\n", id)
})
mux.HandleFunc("POST /users", createUserHandler)
mux.HandleFunc("GET /files/{path...}", serveFileHandler)
mux.HandleFunc("GET /health/{$}", healthHandler) // exact match
```

##### Request.Pattern [NEW in 1.23]

```go
// The pattern that matched this request is now available
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        next.ServeHTTP(w, r)
        slog.Info("request",
            "pattern", r.Pattern, // NEW in 1.23: "GET /users/{id}"
            "path", r.URL.Path,
            "method", r.Method,
        )
    })
}
```

##### Cookie improvements [NEW in 1.23]

```go
// Preserved quoted values
c := &http.Cookie{Name: "session", Value: `"abc123"`, Quoted: true}
// Value is stored with quotes preserved

// ParseCookie — parse a Cookie header value
cookies, err := http.ParseCookie("session=abc123; lang=en")

// ParseSetCookie — parse a Set-Cookie header
cookie, err := http.ParseSetCookie("session=abc123; Path=/; HttpOnly")

// Get all cookies with a given name
r.CookiesNamed("session") // returns []*http.Cookie

// Partitioned cookies (CHIPS)
c2 := &http.Cookie{
    Name:        "session",
    Value:       "abc123",
    Partitioned: true,
}
```

##### Middleware pattern

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        slog.Info("request",
            slog.String("pattern", r.Pattern),
            slog.Duration("latency", time.Since(start)),
        )
    })
}

mux := http.NewServeMux()
http.ListenAndServe(":8080", LoggingMiddleware(mux))
```

---

#### Testing & Benchmarking

##### Unit tests

```go
func TestAdd(t *testing.T) {
    if got := Add(2, 3); got != 5 {
        t.Errorf("got %d, want 5", got)
    }
}
```

##### Table-driven (1.22+: no shadow)

```go
func TestDivide(t *testing.T) {
    tests := []struct {
        name    string
        a, b    float64
        want    float64
        wantErr bool
    }{
        {"normal", 10, 2, 5, false},
        {"by zero", 10, 0, 0, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()  // no tt := tt needed in 1.22+
            got, err := Divide(tt.a, tt.b)
            if (err != nil) != tt.wantErr { t.Fatalf("err: %v", err) }
            if !tt.wantErr && got != tt.want { t.Errorf("got %v want %v", got, tt.want) }
        })
    }
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

Prefer the built-in test lifecycle helpers over manual bookkeeping where
available in Go 1.23: `t.TempDir()` (1.15), `t.Setenv()` (1.17), and
`t.Cleanup()` (1.14).

Note: `t.Context()` and `t.Chdir()` are Go 1.24 additions and are **not**
available in Go 1.23. In Go 1.23 tests, derive a context explicitly (e.g.
`context.WithCancel`/`WithTimeout`) and register cleanup with `t.Cleanup`.

##### Benchmarks

```go
func BenchmarkProcess(b *testing.B) {
    for range b.N { Process(data) }
}
// go test -bench=. -benchmem ./...
```

##### Fuzz testing

```go
func FuzzParse(f *testing.F) {
    f.Add("valid-input")
    f.Fuzz(func(t *testing.T, s string) { _, _ = Parse(s) })
}
// go test -fuzz=FuzzParse -fuzztime=30s
```

##### Vet checks (cumulative)

```go
// 1.22: append no values, time.Since in defer, slog key mismatch
// 1.23: stdversion — reference to symbol too new for go.mod go version
go vet ./...
```

---

#### CLI Development

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
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
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

##### PGO (GA since 1.21, faster builds in 1.23)

```bash
go build -o myapp ./cmd/myapp
# collect profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cmd/myapp/default.pgo
# rebuild with PGO (auto)
go build ./cmd/myapp
```

**1.23 improvement**: PGO build overhead reduced from 100%+ to single-digit percentages for large builds.

##### Stack frame overlapping [NEW in 1.23]

The compiler now overlaps stack slots for local variables used in disjoint regions of a function. No code changes needed — binary is smaller, less stack is used.

##### Hot block alignment [NEW in 1.23]

PGO-informed alignment of hot loop blocks on 386/amd64. 1–1.5% additional improvement, 0.1% larger binary. Disable with `-gcflags=-d=alignhot=0`.

##### Iterator performance

Push iterators (plain functions with `yield`) are inlined by the compiler for simple cases — performance is equivalent to hand-written loops. Use them freely. `iter.Pull` uses goroutines internally — avoid in tight loops or high-frequency paths.

##### sync/atomic.And / Or

Prefer new `atomic.And`/`atomic.Or` over compare-and-swap loops for bitwise flag operations — lower overhead, simpler code.

##### unique.Handle comparisons

`unique.Handle[T]` equality is a pointer comparison — O(1) regardless of the size of T. Much faster than comparing large structs or long strings directly.

##### GC — deep stack traces

`runtime/pprof` max stack depth raised from 32 to 128 frames — better visibility in profiles for deeply nested programs.

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

---

#### Idioms & Things to Avoid

##### Do (1.23 additions)

- **Use `for v := range myIter()`** — standard, compiler-optimised iteration.
- **Expose `All() iter.Seq[T]`** and `Backward() iter.Seq[T]` on custom collection types.
- **Use `slices.Collect(seq)`** to materialise an iterator into a slice.
- **Use `maps.Keys(m)`** and `maps.Values(m)`** to iterate without allocating a slice.
- **Use `unique.Make(v)`** for interning repeated comparable values (strings, structs).
- **Call `t.Helper()`** in test helpers so failures point at the caller's line.
- **Always `defer stop()`** after `iter.Pull`/`iter.Pull2`.
- **Use `go mod tidy -diff`** in CI to assert go.mod/go.sum are tidy.
- **Use `godebug` in go.mod** rather than env vars for GODEBUG settings.
- **Wrap `defer` in a closure** when timing — `defer func() { log(time.Since(t)) }()`.
- **Remove drain-before-Reset pattern** in 1.23 modules — timer channels are now unbuffered.
- **Use non-blocking `select`** to check timers instead of `len(t.C)`.
- **Use `structs.HostLayout`** for structs passed to C/OS APIs.
- **Use `os.CopyFS`** for directory copies.
- **Use `filepath.Localize`** instead of `filepath.FromSlash` for user-provided paths.

##### Don't

| Anti-pattern | Why | Instead |
|---|---|---|
| Callback-based iteration | No `break`/`return` control; verbose | `iter.Seq` + `for range` |
| `defer f.Close()` inside iterator loop body | All defers run at function return | Wrap each iteration in anonymous func |
| `len(t.C) > 0` | Capacity is 0 in 1.23 | `select { case <-t.C: default: }` |
| `if !t.Stop() { <-t.C }` before Reset | Unnecessary in 1.23 | `t.Reset(d)` directly |
| `i := i` shadow in 1.22+ | Loop vars are per-iteration | Remove |
| `math/rand` (v1) in new code | Deprecated | `math/rand/v2` |
| `var m map[string]int` | Nil, writes panic | `make(map[string]int)` |
| `clear(s)` to empty a slice | Only zeroes; len unchanged | `s = s[:0]` |
| Manual string interning | Verbose, racy | `unique.Make(s)` |
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
type UserRepository interface{}  // PascalCase exported
type httpClient struct{}         // camelCase unexported
type HTTPClient struct{}         // acronyms all-caps
var userID int                   // acronym in variable
var ErrNotFound = errors.New("not found")  // Err prefix
type Stringer interface{ String() string } // -er suffix
func (u *User) IsAdmin() bool {}           // is/has bool prefix
// user.Name not user.UserName             // no stuttering
```

##### Formatting & linting

```bash
gofmt -w .
goimports -w .
go vet ./...
staticcheck ./...
golangci-lint run
```

A `golangci-lint` v2 configuration (the tool version is independent of the Go
language version and lints Go 1.23 code fine):

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

*End of Go 1.23 Complete Developer Guideline*
