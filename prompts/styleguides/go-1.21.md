### Go 1.21 — Complete Developer Guideline

> **Version**: Go 1.21.0 (released 2023-08-08)
> **Scope**: Full language spec, all 1.21 changes delta over 1.20, idioms, concurrency, performance, CLI, file I/O, testing, and best practices — with examples throughout.
> **Note**: This document is self-contained. All relevant 1.20 and 1.19 material is carried forward or superseded. Sections marked **[NEW in 1.21]** cover additions specific to this version.

---

#### Table of Contents

1. [What's New in Go 1.21](#whats-new-in-go-121)
2. [Project Layout & Modules](#project-layout--modules)
3. [Basic Types & Variables](#basic-types--variables)
4. [Pointers](#pointers)
5. [Control Flow](#control-flow)
6. [Functions](#functions)
7. [Defer, Panic & Recover](#defer-panic--recover)
8. [Structs & Methods](#structs--methods)
9. [Interfaces & Embedding](#interfaces--embedding)
10. [Generics](#generics)
11. [Collection Types: Arrays, Slices, Maps](#collection-types-arrays-slices-maps)
12. [Strings](#strings)
13. [Error Handling](#error-handling)
14. [Structured Logging with log/slog](#structured-logging-with-logslog)
15. [Concurrency](#concurrency)
16. [Context Package](#context-package)
17. [File I/O & Streaming](#file-io--streaming)
18. [Testing & Benchmarking](#testing--benchmarking)
19. [CLI Development](#cli-development)
20. [Performance Caveats & PGO](#performance-caveats--pgo)
21. [Idioms & Things to Avoid](#idioms--things-to-avoid)

---

#### What's New in Go 1.21

Go 1.21.0 shipped 2023-08-08. It is a significant release with **new built-in functions**, **four major new standard library packages**, a general-availability PGO, improved toolchain versioning, and a ~40% reduction in GC tail latency.

##### Version numbering change

Starting with Go 1.21, the first release in a cycle is Go 1.N**.0** (not Go 1.N). The `go` command reports `go1.21.0`. This affects `go.mod` minimum version requirements.

##### Language changes

###### 1. New built-in: min and max [NEW in 1.21]

```go
// min and max work on any ordered type (integers, floats, strings)
// At least one argument is required.
n := min(3, 1, 4, 1, 5) // 1
m := max(3, 1, 4, 1, 5) // 5

s := min("banana", "apple", "cherry") // "apple"

// Mixed untyped constants: type is inferred (like constant expressions)
x := min(1, 2.5) // float64(1.0)

// Works with generics' cmp.Ordered types — no more hand-rolled minInt/maxInt helpers
```

**Before 1.21**: every codebase had its own `minInt`, `maxFloat64`, etc. Those are now dead code.

###### 2. New built-in: clear [NEW in 1.21]

```go
// clear on a MAP: removes all key-value pairs (equivalent to range-delete loop)
m := map[string]int{"a": 1, "b": 2}
clear(m)
fmt.Println(len(m)) // 0

// clear on a SLICE: zeroes all elements, does NOT change length or capacity
s := []int{1, 2, 3, 4, 5}
clear(s)
fmt.Println(s)      // [0 0 0 0 0]
fmt.Println(len(s)) // 5 — unchanged!

// This is different from s = s[:0] which sets length to 0 but keeps cap
// clear is for erasing data (e.g., sensitive buffers) while keeping the allocation
```

**Common mistake**: using `clear(s)` expecting it to empty a slice like a map. It does NOT change the length. To "empty" a slice: `s = s[:0]`. To zero sensitive data AND keep allocation: use `clear(s)`.

###### 3. panic(nil) behaviour change [NEW in 1.21]

In Go 1.20 and earlier, `panic(nil)` was indistinguishable from a normal function return to `recover()`:

```go
// Before 1.21: recover() returned nil for both "no panic" and "panic(nil)"
defer func() {
    r := recover()
    if r == nil {
        // Was this no-panic? Or panic(nil)? Impossible to tell.
    }
}()
panic(nil) // in 1.20: recover() returns nil, ambiguous
```

In Go 1.21, `panic(nil)` causes a runtime panic of type `*runtime.PanicNilError`. `recover()` is now guaranteed to return non-nil when a panic actually occurred:

```go
// Go 1.21+
defer func() {
    r := recover()
    if r == nil {
        // Now this ONLY means: no panic occurred
    }
    // r is a *runtime.PanicNilError if panic(nil) was called
}()
panic(nil) // recover() returns *runtime.PanicNilError{}, not nil
```

**Migration**: If you have code that calls `panic(nil)` intentionally and checks `recover() == nil`, update it. For backwards compatibility with modules declaring `go 1.20` or earlier in `go.mod`, the old behaviour is preserved automatically.

To explicitly re-enable old behaviour:
```bash
GODEBUG=panicnil=1 ./myapp
```

###### 4. Improved type inference for generics [NEW in 1.21]

Several inference improvements:

```go
// 1. Generic functions can be passed as arguments without explicit instantiation
slices.IndexFunc(items, isValid) // isValid can itself be generic

// 2. Generic functions can be assigned/returned without explicit type args
var f func(int) bool = slices.Contains[[]int] // type args inferred

// 3. Type inference considers interface methods
// 4. Mixed untyped constants infer the common type (min/max rely on this)
```

##### Package initialization order [NEW in 1.21]

The order is now formally specified:
1. Sort all packages by import path.
2. Repeat: find first package whose imports are all already initialized, initialize it.

This may change order in programs that relied on unspecified ordering. The new order is deterministic.

##### Toolchain management [NEW in 1.21]

The `go` command can now automatically download and use a different Go toolchain version if required:

```go
// go.mod
module myapp

go 1.21.0     // minimum Go version required (strict in 1.21+)
toolchain go1.21.3  // suggested minimum toolchain (may be newer than go line)
```

```bash
# If your go.mod requires go 1.21.3 but you have go1.21.0 installed,
# go will automatically download and use go1.21.3
go build ./...  // auto-selects correct toolchain

# Control toolchain selection
GOTOOLCHAIN=local go build    # use only local toolchain
GOTOOLCHAIN=go1.21.3 go build # use specific version
GOTOOLCHAIN=auto go build     # default: auto-download if needed
```

**Backwards compatibility with GODEBUG**: When you upgrade Go but keep the `go` line in `go.mod` at an older version, Go preserves the behaviour of that older version for GODEBUG-controlled features.

##### Runtime

- GC tail latency reduction: up to **40% reduction** in tail latency from GC tuning.
- On Linux with transparent huge pages: better heap management, small heaps use up to 50% less memory in pathological cases.
- New `runtime.Pinner` type: allows pinning Go memory for use by C code without violating cgo pointer rules.
- Stack traces now include the goroutine IDs of creator goroutines.
- Deeply recursive stacks: now shows first 50 + last 50 frames (was first 100).

##### Compiler

- PGO **generally available** (was preview in 1.20). `-pgo=auto` is now the default.
- PGO now devirtualises some interface method calls.
- Build speed improved ~6% (compiler itself built with PGO).

##### Standard Library — new packages

| Package | Purpose |
|---|---|
| `log/slog` | Structured, levelled logging with key-value pairs |
| `slices` | Generic functions for slices (sort, search, compare, modify) |
| `maps` | Generic functions for maps (clone, delete, copy, equal) |
| `cmp` | `Ordered` constraint + `Compare` and `Less` generic functions |

##### Standard Library — selected changes

| Package | Change |
|---|---|
| `context` | `WithoutCancel`, `WithDeadlineCause`, `WithTimeoutCause`, `AfterFunc` |
| `errors` | `errors.ErrUnsupported` sentinel |
| `sync` | `sync.OnceFunc`, `sync.OnceValue`, `sync.OnceValues` |
| `encoding/binary` | `binary.NativeEndian` |
| `reflect` | `Value.Clear`; `SliceHeader`/`StringHeader` deprecated |
| `regexp` | `Regexp.MarshalText`/`UnmarshalText` |
| `net/http` | `ResponseController.EnableFullDuplex` |
| `flag` | `BoolFunc`/`FlagSet.BoolFunc` |
| `testing` | `testing.Testing()` function; `-test.fullpath` flag |

---

#### Project Layout & Modules

```
myapp/
├── cmd/
│   └── myapp/
│       ├── main.go
│       └── default.pgo     # PGO profile — auto-used by go build
├── internal/
│   ├── server/
│   └── storage/
├── pkg/
│   └── util/
├── go.mod
└── go.sum
```

##### go.mod with 1.21 toolchain directive

```
module github.com/yourname/myapp

go 1.21.0

toolchain go1.21.3
```

The `go` line is now a strict minimum. If someone tries to build with Go 1.20, they get a clear error rather than a silent failure.

##### Verify library APIs against the pinned version

Verify library APIs against the actual module versions in `go.mod` before
claiming an API is missing or unavailable.

- Do not claim a controller-runtime helper is unavailable without checking the
  pinned `sigs.k8s.io/controller-runtime` version.
- Say "this API is not available in vX.Y.Z" only when the module version proves
  it.
- Do not infer API availability from memory, a newer version's docs, or another
  project's dependency set.

##### Package naming

- Lowercase, single word, no underscores.
- Package name = last path element.
- File names: snake_case.

---

#### Basic Types & Variables

##### Numeric types

| Type | Size | Notes |
|---|---|---|
| `int8`–`int64`, `uint8`–`uint64` | 1–8 bytes | Fixed-width |
| `int`, `uint` | platform | Use for indexing and general counts |
| `float32`, `float64` | 4, 8 bytes | IEEE 754 |
| `byte` = `uint8`, `rune` = `int32` | — | Aliases |

##### Declaration

```go
var x int = 10
var y = "hello"
z := 3.14

// min/max now built-in (1.21)
maxVal := max(x, 20, 15) // 20
minVal := min(x, 20, 15) // 10
```

##### Zero values

| Type | Zero |
|---|---|
| numeric | `0` |
| `bool` | `false` |
| `string` | `""` |
| pointer, slice, map, chan, func, interface | `nil` |

##### iota — enum-like constants

```go
type Direction int
const (
    North Direction = iota // 0
    East                   // 1
    South                  // 2
    West                   // 3
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
fmt.Println(x)  // 99
```

##### new vs &T{}

```go
p1 := new(int)            // *int = 0
p2 := &Point{X: 1, Y: 2} // preferred for structs
```

##### When to use pointers

- Mutating the receiver/argument.
- Large structs (avoid copy cost).
- Representing optional values (nil = absent).
- Satisfying interface with pointer-receiver methods.

##### Loop variable capture (still required in 1.21)

```go
// WRONG — all goroutines share the same i
for i := 0; i < 3; i++ {
    go func() { fmt.Println(i) }()
}

// CORRECT — shadow to create per-iteration binding
for i := 0; i < 3; i++ {
    i := i
    go func() { fmt.Println(i) }()
}

// ALSO CORRECT — pass as argument
for i := 0; i < 3; i++ {
    go func(n int) { fmt.Println(n) }(i)
}
```

Go 1.21 ships an **experimental** fix (`GOEXPERIMENT=loopvar`) that makes loop variables per-iteration automatically. This becomes the default in Go 1.22. In Go 1.21 production code: still shadow manually.

---

#### Control Flow

##### if / else

```go
if x > 0 {
    fmt.Println("positive")
} else if x < 0 {
    fmt.Println("negative")
} else {
    fmt.Println("zero")
}

// Init statement
if err := doSomething(); err != nil { return err }

// No else after return — idiomatic
f, err := os.Open(name)
if err != nil { return err }
use(f)
```

##### for

```go
for i := 0; i < 10; i++ { }          // C-style
for x < 100 { x *= 2 }               // while-style
for { if done() { break } }           // infinite
for i, v := range s { }              // slice: index + value
for i := range s { }                 // slice: index only
for _, v := range s { }              // slice: value only
for k, v := range m { }              // map: key + value (random order)
for i, r := range "héllo" { }       // string: rune iteration
for v := range ch { }                // channel: until close
```

##### switch

```go
switch x {
case 1:          fmt.Println("one")
case 2, 3:       fmt.Println("two or three")
default:         fmt.Println("other")
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

No fallthrough by default. `fallthrough` keyword is explicit and rare.

##### select

```go
select {
case msg := <-ch1:              fmt.Println("received", msg)
case ch2 <- "hello":            fmt.Println("sent")
case <-time.After(time.Second): fmt.Println("timeout")
default:                        fmt.Println("non-blocking")
}
```

##### break / continue / labels

```go
outer:
for i := 0; i < 5; i++ {
    for j := 0; j < 5; j++ {
        if j == 2 { continue outer }
        if i == 3 { break outer }
    }
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

// Named returns — sparingly
func minMax(a, b int) (min, max int) {
    if a < b { return a, b }
    min, max = b, a
    return
}

// Variadic
func sum(nums ...int) int {
    total := 0
    for _, n := range nums { total += n }
    return total
}
sum([]int{1, 2, 3}...)

// First-class + closures
func counter() func() int {
    n := 0
    return func() int { n++; return n }
}
```

##### init

```go
func init() { /* runs once before main, use for registering drivers etc. */ }
```

---

#### Defer, Panic & Recover

##### defer — LIFO execution

```go
func process(name string) error {
    f, err := os.Open(name)
    if err != nil { return err }
    defer f.Close()
    // ...
    return nil
}

// Arguments evaluated immediately at defer statement
x := 10
defer fmt.Println(x) // prints 10, not whatever x is at return
x = 20
```

Avoid defer in hot inner loops — overhead per call.

##### panic(nil) — CHANGED in 1.21

```go
// 1.21+: panic(nil) is no longer ambiguous with "no panic"
defer func() {
    r := recover()
    if r == nil {
        // 1.21 guarantee: this means NO panic occurred
    }
    // *runtime.PanicNilError if panic(nil) was called
}()
```

##### recover

```go
func safe(fn func()) (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("recovered: %v", r)
        }
    }()
    fn()
    return nil
}
```

Only recover at API/package boundaries. Never silently swallow panics.

---

#### Structs & Methods

```go
type User struct {
    ID    int
    Name  string
    Email string
    admin bool
}

u := User{ID: 1, Name: "Alice", Email: "alice@example.com"}

// Embedding
type Timestamps struct { CreatedAt, UpdatedAt time.Time }
type Post struct {
    ID    int
    Title string
    Timestamps
}
p := Post{}
p.CreatedAt = time.Now() // promoted field

// Methods
func (u User) Display() string           { return u.Name }
func (u *User) Promote()                 { u.admin = true }
```

##### Value vs. Pointer receivers

| Condition | Receiver |
|---|---|
| Mutates receiver | `*T` |
| Large struct | `*T` |
| Contains mutex or sync type | `*T` (never copy) |
| Read-only small struct | `T` |
| Any method uses `*T` | Use `*T` for ALL methods |

##### Constructor + functional options

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

// Implicit satisfaction — no implements keyword
type File struct{}
func (f *File) Write(p []byte) (int, error) { return len(p), nil }
func (f *File) Close() error                { return nil }
var wc WriteCloser = &File{}

// Type assertion
var x any = "hello"
s, ok := x.(string) // ok = true

// Type switch
switch v := x.(type) {
case int:    fmt.Println("int", v)
case string: fmt.Println("string", v)
}
```

Small interfaces (1-2 methods) are best. Accept interfaces, return concrete types.

---

#### Generics

##### Type parameters & constraints

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

// cmp.Ordered — NEW in 1.21: the canonical ordered constraint
import "cmp"
func Min[T cmp.Ordered](a, b T) T {
    if a < b { return a }
    return b
}
```

##### Generic structs

```go
type Stack[T any] struct{ items []T }
func (s *Stack[T]) Push(v T)  { s.items = append(s.items, v) }
func (s *Stack[T]) Pop() T {
    n := len(s.items)
    v := s.items[n-1]
    s.items = s.items[:n-1]
    return v
}
```

##### Improved type inference (1.21)

```go
// Generic function passed as argument — type args inferred
slices.IndexFunc(items, matchFunc) // matchFunc can be generic

// Assign/return generic function without explicit instantiation
var finder func([]int, int) bool = slices.Contains // inferred
```

##### When to use generics

Use for: utility functions over collections, generic data structures, algorithms that work identically across types.

Avoid for: domain models, when a plain interface is simpler, when you'd need to constrain to methods (use interface instead).

---

#### Collection Types: Arrays, Slices, Maps

##### Arrays

```go
b := [3]int{1, 2, 3}
c := [...]int{4, 5, 6}

// Slice to array value (1.20+)
s := []byte{1, 2, 3, 4, 5}
arr := [4]byte(s) // copy; panics if len(s) < 4
```

##### Slices

```go
s := []int{1, 2, 3}
s2 := make([]int, 0, 10)

sl := arr[1:4] // view — shares backing array
s = append(s, 4, 5)
s = append(s, other...)
```

**Slice header**: pointer + length + capacity. Slicing shares memory.

```go
// Independent copy
c := make([]int, len(s))
copy(c, s)

// Preallocate
result := make([]string, 0, len(items))
for _, item := range items { result = append(result, process(item)) }
```

###### The slices package [NEW in 1.21]

```go
import "slices"

// Sort — faster than sort.Slice, type-safe
slices.Sort([]int{3, 1, 4, 1, 5}) // in-place sort

// Sort with comparator
slices.SortFunc(users, func(a, b User) int {
    return cmp.Compare(a.Name, b.Name) // cmp.Compare is new in 1.21
})

// Stable sort
slices.SortStableFunc(items, cmpFn)

// Binary search (slice must be sorted)
idx, found := slices.BinarySearch(sorted, target)
idx, found = slices.BinarySearchFunc(sorted, target, cmpFn)

// Contains / Index
slices.Contains(s, "hello")           // bool
slices.ContainsFunc(s, isVowel)       // bool
slices.Index(s, "hello")              // int index, -1 if not found
slices.IndexFunc(s, isVowel)          // first index where fn returns true

// Compare
slices.Equal(a, b)                    // element-wise equality
slices.EqualFunc(a, b, fn)
slices.Compare(a, b)                  // lexicographic compare

// Transformations
slices.Reverse(s)                     // in-place
slices.Compact(s)                     // remove consecutive duplicates
slices.CompactFunc(s, fn)
slices.Clip(s)                        // trim unused capacity: s[:len(s):len(s)]

// Growing / inserting
slices.Grow(s, 10)                    // ensure cap >= len+10
slices.Insert(s, idx, values...)      // insert at index
slices.Delete(s, i, j)               // delete s[i:j]
slices.Replace(s, i, j, values...)   // replace s[i:j] with values

// Max / Min (require cmp.Ordered)
slices.Max(s)                         // maximum element
slices.Min(s)                         // minimum element
slices.MaxFunc(s, cmpFn)
slices.MinFunc(s, cmpFn)
```

**Important**: `slices.Delete`, `slices.Insert`, etc. return a new slice header — always capture the return value.

###### Modifying a slice during iteration

```go
// Safe: modify elements in place
for i := range s { s[i] *= 2 }

// Safe: delete during range with index
n := 0
for _, v := range s {
    if keep(v) { s[n] = v; n++ }
}
s = s[:n]

// Dangerous: append during range — use slices.DeleteFunc in 1.21
// slices.DeleteFunc removes all elements satisfying fn, returns new slice
s = slices.DeleteFunc(s, func(v int) bool { return v < 0 })
```

##### Maps

```go
m := make(map[string]int)
m2 := map[string]int{"a": 1}
v, ok := m["key"]
delete(m, "key")
```

###### The maps package [NEW in 1.21]

```go
import "maps"

// Clone — shallow copy
m2 := maps.Clone(m)

// Copy — copy all key-value pairs from src to dst
maps.Copy(dst, src)

// Delete matching keys
maps.DeleteFunc(m, func(k string, v int) bool { return v < 0 })

// Equal — all keys/values match
maps.Equal(m1, m2)         // requires comparable value type
maps.EqualFunc(m1, m2, fn) // custom value comparison

// Keys and Values were intentionally NOT included in 1.21
// (names reserved for iterator-based versions in 1.22+)
// Use a range loop instead:
keys := make([]string, 0, len(m))
for k := range m { keys = append(keys, k) }
```

###### Map rules

- `var m map[string]int` is nil — writing panics. Always use `make`.
- Iteration order is randomised every run.
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

#### Strings

##### Key facts

- Immutable byte sequence, UTF-8 by default.
- `len(s)` = bytes. `len([]rune(s))` = code points.
- `range` iterates runes.

```go
s := "héllo"
fmt.Println(len(s))         // 6
fmt.Println(len([]rune(s))) // 5
for i, r := range s { fmt.Printf("%d: %c\n", i, r) }
```

##### Efficient building

```go
var b strings.Builder
b.Grow(1000)
for i := 0; i < 1000; i++ { b.WriteByte('x') }
result := b.String()
```

##### Key operations

```go
strings.Contains("foobar", "bar")
strings.HasPrefix("foobar", "foo")
strings.HasSuffix("foobar", "bar")
strings.Split("a,b,c", ",")
strings.Join([]string{"a","b"}, "-")
strings.TrimSpace("  hello  ")
strings.ToUpper("hello")
strings.ReplaceAll("aabbcc", "b", "x")

// 1.20+
after, found := strings.CutPrefix("Gopher", "Go")   // "pher", true
before, found := strings.CutSuffix("Gopher", "er")  // "Goph", true
```

---

#### Error Handling

```go
type error interface { Error() string }

// Basics
result, err := doSomething()
if err != nil { return fmt.Errorf("context: %w", err) }

// Sentinel errors
var ErrNotFound = errors.New("not found")
if errors.Is(err, ErrNotFound) { /* handle */ }

// Custom error type
type ValidationError struct { Field, Message string }
func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation: %s: %s", e.Field, e.Message)
}
var ve *ValidationError
if errors.As(err, &ve) { fmt.Println(ve.Field) }

// Multi-error (1.20+)
combined := errors.Join(err1, err2)
wrapped  := fmt.Errorf("two: %w and %w", err1, err2)
```

**Always check errors.** A few discards are idiomatic and should not be flagged:

- deferred `Close()` on files opened only for reading
- `defer tx.Rollback()`, which is a no-op after a successful commit
- writes to `bytes.Buffer` or `strings.Builder`, which never return an error

For everything else, handle the error or make the discard explicit with `_ =`.

##### errors.ErrUnsupported [NEW in 1.21]

```go
// Standardised "not supported" sentinel
import "errors"

// Return when an operation is not supported
func (f *MyFS) Symlink(oldname, newname string) error {
    return fmt.Errorf("symlink: %w", errors.ErrUnsupported)
}

// Check if the error is "unsupported" (works through wrapping)
if errors.Is(err, errors.ErrUnsupported) {
    // try fallback
}

// The net/http package also uses this:
errors.Is(http.ErrNotSupported, errors.ErrUnsupported) // true
```

##### Don't log AND return

```go
// Bad: double-logging
if err != nil { log.Println(err); return err }

// Good: return and log at the boundary
```

##### Concurrent error collection

```go
import "golang.org/x/sync/errgroup"

g, ctx := errgroup.WithContext(context.Background())
for _, url := range urls {
    url := url
    g.Go(func() error { return fetch(ctx, url) })
}
if err := g.Wait(); err != nil { return err }
```

---

#### Structured Logging with log/slog

`log/slog` is the most significant new standard library addition in 1.21. It provides structured, levelled logging with pluggable handlers.

##### Basic usage

```go
import "log/slog"

// Default logger writes JSON or text to os.Stderr
slog.Info("user logged in", "userID", 42, "ip", "1.2.3.4")
// 2023/08/08 12:00:00 INFO user logged in userID=42 ip=1.2.3.4

slog.Warn("high latency", "ms", 312)
slog.Error("failed to connect", "err", err)
slog.Debug("processing", "count", len(items)) // not shown unless level <= Debug
```

##### Log levels

```go
// Levels: Debug(-4), Info(0), Warn(4), Error(8)
slog.SetLogLoggerLevel(slog.LevelDebug) // set global minimum level
```

##### Creating a logger

```go
// Text handler (human-readable)
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

// JSON handler (structured, for log aggregators like Loki, Datadog)
logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
    // Add source file/line to every record
    AddSource: true,
}))

// Set as default
slog.SetDefault(logger)
```

##### Structured attributes

```go
// Key-value pairs (alternating string key + value)
slog.Info("request", "method", "GET", "path", "/api/v1/users", "status", 200)

// slog.Attr for type-safe attributes
slog.Info("request",
    slog.String("method", "GET"),
    slog.Int("status", 200),
    slog.Duration("latency", 12*time.Millisecond),
    slog.Time("at", time.Now()),
    slog.Any("err", err),
)

// Group attributes
slog.Info("request",
    slog.Group("http",
        slog.String("method", "GET"),
        slog.Int("status", 200),
    ),
)
// JSON: {"time":"...","level":"INFO","msg":"request","http":{"method":"GET","status":200}}
```

##### Logger with context (child loggers)

```go
// With: returns a new logger with added attributes on every record
reqLogger := logger.With("requestID", reqID, "userID", userID)
reqLogger.Info("handler started")
reqLogger.Info("handler done", "duration", time.Since(start))

// WithGroup: namespaces subsequent attributes
dbLogger := logger.WithGroup("db").With("host", "localhost")
dbLogger.Error("query failed", "err", err, "query", sql)
// JSON: {..., "db": {"host":"localhost", "err":"...", "query":"..."}}
```

##### Context-aware logging

```go
// Pass context to attach trace IDs etc. from context values
slog.InfoContext(ctx, "processing request", "id", id)

// In handlers: check ctx.Done() for logging cancellation
```

##### Custom handler

```go
// Implement slog.Handler for custom sinks (e.g., send to external service)
type MyHandler struct {
    opts   slog.HandlerOptions
    attrs  []slog.Attr
    groups []string
}

func (h *MyHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return level >= h.opts.Level.Level()
}

func (h *MyHandler) Handle(ctx context.Context, r slog.Record) error {
    // send to custom sink
    return nil
}

func (h *MyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    // return new handler with attrs added
    return &MyHandler{attrs: append(h.attrs, attrs...)}
}

func (h *MyHandler) WithGroup(name string) slog.Handler {
    return &MyHandler{groups: append(h.groups, name)}
}

// Use testing/slogtest to validate your handler
import "testing/slogtest"
func TestMyHandler(t *testing.T) {
    results := func(t *testing.T) map[string]any { /*parse output*/ }
    if err := slogtest.TestHandler(handler, results); err != nil {
        t.Fatal(err)
    }
}
```

##### Migration from log

```go
// Old
log.Printf("connected to %s", host)
log.Fatalf("failed: %v", err)

// New — slog
slog.Info("connected", "host", host)
slog.Error("failed", "err", err)
os.Exit(1) // slog has no Fatal; call os.Exit yourself

// Bridge: direct existing log output to slog
slog.NewLogLogger(handler, slog.LevelInfo) // returns a *log.Logger
```

##### Best practices for slog

- **Always use structured key-value pairs** — never format strings in the message.
- **Keys should be lowercase snake_case** for consistent log querying.
- **Use `slog.Default()`** for packages that don't own a logger; let the application inject one.
- **Pass `context.Context` through** and use `InfoContext`/`ErrorContext` to carry trace IDs.
- **Do not use `%v` formatting in keys** — `slog.Any("err", err)` not `"err", fmt.Sprintf("%v", err)`.
- **Group related attributes** with `slog.Group` or `logger.WithGroup`.
- **Never log secrets, tokens, credentials, or full request/response bodies** unless they are explicitly scrubbed.

---

#### Concurrency

##### Goroutines

```go
go func() { fmt.Println("concurrent") }()

for i := 0; i < 5; i++ {
    go func(n int) { fmt.Println(n) }(i)
}
```

##### Channels

```go
ch := make(chan int)          // unbuffered
bch := make(chan string, 10)  // buffered

go func() { ch <- 42 }()
val := <-ch

close(ch)
for v := range ch { fmt.Println(v) }

v, ok := <-ch
if !ok { fmt.Println("closed") }
```

**Rules**: only sender closes; sending to closed panics; receiving from closed returns zero.

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
    s.mu.RLock()
    defer s.mu.RUnlock()
    v, ok := s.m[k]
    return v, ok
}

func (s *SafeMap) Set(k string, v int) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.m[k] = v
}
```

**Never copy a mutex.**

##### sync.Once — and new helpers [NEW in 1.21]

```go
// Classic Once
var once sync.Once
once.Do(func() { instance = loadConfig() })

// NEW in 1.21: OnceFunc — memoize a zero-return function
initDB := sync.OnceFunc(func() { db = openDB() })
initDB() // safe to call multiple times; runs once

// NEW in 1.21: OnceValue — memoize a single return value
getConfig := sync.OnceValue(func() *Config { return loadConfig() })
cfg := getConfig() // *Config; computed once, cached forever

// NEW in 1.21: OnceValues — memoize two return values
getConn := sync.OnceValues(func() (*sql.DB, error) { return sql.Open("postgres", dsn) })
db, err := getConn() // called many times; connects only once
```

##### sync.Map — with 1.20 atomic additions

```go
var m sync.Map
m.Store("key", "value")
v, ok := m.Load("key")
m.Delete("key")
old, loaded := m.Swap("key", "new")            // 1.20+
ok = m.CompareAndSwap("key", "old", "new")    // 1.20+
ok = m.CompareAndDelete("key", "value")       // 1.20+
m.Range(func(k, v any) bool { return true })
```

##### Atomic types

```go
var counter atomic.Int64
counter.Add(1)
fmt.Println(counter.Load())

var ptr atomic.Pointer[Config]
ptr.Store(&Config{})
cfg := ptr.Load()
```

##### Worker pool

```go
func workerPool(jobs <-chan Job, results chan<- Result, n int) {
    var wg sync.WaitGroup
    for i := 0; i < n; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs { results <- process(job) }
        }()
    }
    go func() { wg.Wait(); close(results) }()
}
```

##### Concurrency pitfalls

| Pitfall | Fix |
|---|---|
| Goroutine leak | Context cancellation; always provide exit |
| Race condition | `-race` flag |
| Deadlock | No circular waits; check every send has receive |
| Closing closed channel | Sender closes; `sync.Once` if ambiguous |
| Loop variable capture | Shadow `i := i` or pass as arg |
| Copying mutex | Pointer receiver + embed |

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

##### Creation

```go
ctx := context.Background()
ctx, cancel := context.WithCancel(parent); defer cancel()
ctx, cancel  = context.WithTimeout(parent, 5*time.Second); defer cancel()
ctx, cancel  = context.WithDeadline(parent, deadline); defer cancel()
ctx          = context.WithValue(parent, key{}, value)
```

##### WithCancelCause (1.20+)

```go
ctx, cancel := context.WithCancelCause(parent)
defer cancel(nil)

cancel(ErrRateLimited)             // cancel with reason
cause := context.Cause(ctx)       // ErrRateLimited
ctx.Err()                          // context.Canceled
```

##### New context functions [NEW in 1.21]

```go
// WithoutCancel — detach from parent's cancellation
// Use when you need to continue work after the request context is cancelled
// (e.g., logging, metrics, cleanup that should not be interrupted)
detached := context.WithoutCancel(requestCtx)
go func() { logToSlowSink(detached, entry) }()

// WithDeadlineCause — set cause when deadline expires
ctx, cancel := context.WithDeadlineCause(parent, deadline, errors.New("db timeout"))
defer cancel(nil)
cause := context.Cause(ctx) // "db timeout" if deadline exceeded

// WithTimeoutCause — same but duration-based
ctx, cancel = context.WithTimeoutCause(parent, 5*time.Second, errors.New("rpc timeout"))
defer cancel(nil)

// AfterFunc — run f in a new goroutine after ctx is cancelled/done
stop := context.AfterFunc(ctx, func() {
    // called once when ctx is done; runs in own goroutine
    cleanupResources()
})
// To prevent f from running:
if !stop() {
    // f already started — wait for it if needed
}
```

##### Convention: context as first parameter

```go
func FetchUser(ctx context.Context, id int) (*User, error) { /*...*/ }
```

**Never store context in a struct. Never pass nil context.**

---

#### File I/O & Streaming

```go
f, err := os.Open("file.txt")
if err != nil { return err }
defer f.Close()

// Write
f, err = os.Create("out.txt")
defer f.Close()
fmt.Fprintln(f, "hello")

// Buffered
w := bufio.NewWriter(f)
defer w.Flush()
fmt.Fprintln(w, "buffered")

scanner := bufio.NewScanner(f)
for scanner.Scan() { process(scanner.Text()) }
if err := scanner.Err(); err != nil { return err }
scanner.Buffer(make([]byte, 1<<20), 1<<20) // increase max line size

// Read all (small files)
data, err := os.ReadFile("config.json")

// Streaming
io.Copy(dst, src)
limited := io.LimitReader(src, 1<<20)
tee := io.TeeReader(src, &buf)
ow := io.NewOffsetWriter(file, 512) // 1.20+
```

##### Walk + SkipAll (1.20+)

```go
filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
    if err != nil { return err }
    if shouldStop(path) { return filepath.SkipAll }
    fmt.Println(path)
    return nil
})
```

##### JSON

```go
enc := json.NewEncoder(w)
enc.SetIndent("", "  ")
enc.Encode(data)

dec := json.NewDecoder(r)
dec.Decode(&result)

type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
    Pass string `json:"-"`           // always omit
    Age  int    `json:"age,omitempty"` // omit if zero
}
```

##### Temporary files

```go
tmp, err := os.CreateTemp("", "prefix-*.json")
defer os.Remove(tmp.Name())
defer tmp.Close()
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

##### Table-driven (idiomatic)

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
        tt := tt // shadow for parallel safety
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            got, err := Divide(tt.a, tt.b)
            if (err != nil) != tt.wantErr { t.Fatalf("unexpected error: %v", err) }
            if !tt.wantErr && got != tt.want { t.Errorf("got %v, want %v", got, tt.want) }
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
available in Go 1.21: `t.TempDir()` (1.15), `t.Setenv()` (1.17), and
`t.Cleanup()` (1.14).

Note: `t.Context()` and `t.Chdir()` are Go 1.24 additions and are not available
in Go 1.21.

##### testing.Testing() [NEW in 1.21]

```go
// Detect if running inside a test binary
if testing.Testing() {
    // use test-specific configuration
}
```

##### Benchmarks

```go
func BenchmarkProcess(b *testing.B) {
    for i := 0; i < b.N; i++ { Process(data) }
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

##### Subtests with t.Parallel() — vet warning from 1.20

```go
// go vet flags this: loop variable tt captured after t.Parallel()
for _, tt := range tests {
    tt := tt // always shadow before t.Parallel()
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        // safe: tt is per-iteration
    })
}
```

##### Coverage for binaries (1.20+)

```bash
go build -cover -o myapp ./cmd/myapp
GOCOVERDIR=/tmp/cov ./myapp
go tool covdata percent -i=/tmp/cov
```

```bash
go test -race ./...  # always in CI
go vet ./...
```

---

#### CLI Development

##### Standard flag

```go
host    := flag.String("host", "localhost", "server host")
port    := flag.Int("port", 8080, "server port")
verbose := flag.Bool("verbose", false, "verbose")

// NEW in 1.21: BoolFunc — flag without value argument
flag.BoolFunc("json", "output JSON", func(s string) error {
    outputFormat = "json"
    return nil
})

flag.Parse()
args := flag.Args()
```

##### Cobra (production)

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
    rootCmd.PersistentFlags().Bool("verbose", false, "verbose")
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

##### Clean main pattern

```go
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
        os.Exit(1)
    }
}

func run() error {
    // all application logic
    return nil
}
```

---

#### Performance Caveats & PGO

##### PGO — Generally Available [NEW in 1.21]

PGO is now the default when `default.pgo` exists in the main package directory.

```bash
# Collect profile from production (best: cpu profile over real traffic)
go build -o myapp ./cmd/myapp
./myapp &    # run with real traffic
curl http://localhost:6060/debug/pprof/profile?seconds=30 > default.pgo
kill %1

# Move profile to main package
mv default.pgo cmd/myapp/default.pgo

# Build with PGO (auto-enabled now)
go build ./cmd/myapp

# Explicit control
go build -pgo=auto ./cmd/myapp   # default in 1.21
go build -pgo=off  ./cmd/myapp   # disable
go build -pgo=cpu.pprof ./cmd/myapp # explicit file
```

**PGO benefits in 1.21:**
- 2-7% performance improvement for most programs.
- **Interface devirtualisation**: calls to interface methods are now potentially inlined when the hot callee is known from the profile.
- The Go compiler itself is built with PGO — builds are 2-4% faster.

**Tips:**
- Commit `default.pgo` to version control.
- Re-collect periodically as code evolves.
- Profiles are additive — merge profiles from multiple instances: `go tool pprof -proto *.pprof > merged.pprof`.

##### GC improvements (1.21)

- Up to **40% reduction in tail latency** from GC tuning.
- Small heaps use up to 50% less memory (transparent huge pages optimisation on Linux).

```go
// Tune GC
debug.SetMemoryLimit(512 << 20) // soft 512 MiB limit (1.19+)
debug.SetGCPercent(200)         // less frequent GC
```

##### Escape analysis

```bash
go build -gcflags="-m" ./...
```

Values escape to heap when returned as pointer, stored in interface, sent on channel, captured by escaping closure.

##### sync.Pool

```go
var bufPool = sync.Pool{New: func() any { return &bytes.Buffer{} }}

func process(data []byte) string {
    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset()
    defer bufPool.Put(buf)
    buf.Write(data)
    return buf.String()
}
```

##### slices vs sort performance

In 1.21, `slices.Sort` is generally **faster** than `sort.Slice` because:
- It's generic, so no `interface{}` boxing.
- Uses pdqsort (pattern-defeating quicksort) like the existing `sort` package but with less overhead.

```go
// Prefer in new code
slices.Sort(ints)
slices.SortFunc(users, func(a, b User) int { return cmp.Compare(a.Name, b.Name) })

// Old way (still works)
sort.Ints(ints)
sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })
```

##### String building

```go
var b strings.Builder
b.Grow(n)
for i := 0; i < n; i++ { b.WriteByte('x') }
```

##### Struct layout

```go
// Bad: bool wastes 7 bytes before int64
type Bad struct { A bool; B int64; C bool } // 24 bytes
// Better: largest first
type Better struct { B int64; A bool; C bool } // 16 bytes
```

##### math/rand (from 1.20)

```go
// Global RNG is auto-seeded — just use it
n := rand.Intn(100)

// Reproducible: own source
rng := rand.New(rand.NewSource(42))

// rand.Seed() and rand.Read() are deprecated since 1.20
```

##### reflect.ValueOf — heap allocation improvement (1.21)

In 1.21, `reflect.ValueOf` no longer forces its argument to the heap in many cases — stack allocation is now possible. Code that uses reflection heavily may see reduced GC pressure.

##### Profiling

```bash
go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof
go tool pprof cpu.prof
go tool pprof mem.prof

import _ "net/http/pprof"
go http.ListenAndServe(":6060", nil)
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

---

#### Idioms & Things to Avoid

##### Do

- **Use `min`/`max` built-ins** — no more hand-rolled helpers.
- **Use `clear(m)` to empty maps** — idiomatic and clear (but know it zeroes slices, not shortens them).
- **Use `slices.Sort`/`slices.SortFunc`** instead of `sort.Slice` in new code — faster, type-safe.
- **Use `maps.Clone`/`maps.DeleteFunc`/`maps.Equal`** from the new `maps` package.
- **Use `slog`** for all structured logging in new services.
- **Use `sync.OnceValue`/`sync.OnceValues`** for lazy initialisation that returns values.
- **Use `context.WithoutCancel`** when you need to detach from request context for cleanup.
- **Use `context.AfterFunc`** to run cleanup when a context expires.
- **Use `errors.ErrUnsupported`** to signal unsupported operations.
- **Return errors as values.** Never panic for expected failures.
- **Accept interfaces, return concrete types.**
- **Error early** — return on failure, no nesting.
- **Use `defer` for all cleanup.**
- **Preallocate** slices/maps when size is known.
- **Shadow loop variables** before goroutines: `i := i`.
- **Always `defer cancel()`** after any cancellable context.
- **Use PGO** with `default.pgo` for production binaries.

##### Don't

| Anti-pattern | Why | Instead |
|---|---|---|
| Hand-rolled min/max helpers | Built-in now | `min()`, `max()` |
| `clear(s)` to "empty" a slice | Only zeroes; len unchanged | `s = s[:0]` to empty |
| `sort.Slice` in new code | Less ergonomic | `slices.Sort` / `slices.SortFunc` |
| `var m map[string]int` | Nil, writes panic | `make(map[string]int)` |
| Mixing value and pointer receivers | Confusing method sets | Pick one |
| Ignoring `error` with `_` | Silent failures | Handle or propagate |
| Logging AND returning error | Double-logging | Return; log at top boundary |
| `rand.Seed()` | Deprecated in 1.20 | `rand.New(rand.NewSource(n))` |
| `panic` for business logic | Crashes program | `return error` |
| Copying a mutex | Broken locking | Embed + pointer receivers |
| Closing channel from receiver | Panic if double-close | Sender closes |
| String `+` in loops | O(n^2) | `strings.Builder` |
| Storing context in struct | Anti-pattern | First function parameter |
| `panic(nil)` without expecting `*runtime.PanicNilError` | Changed semantics in 1.21 | `panic(errors.New("reason"))` |
| `reflect.SliceHeader` / `reflect.StringHeader` | Deprecated in 1.21 | `unsafe.Slice`, `unsafe.String`, `unsafe.SliceData`, `unsafe.StringData` |

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
// Exported: PascalCase
type UserRepository interface{}
func ParseConfig() {}

// Unexported: camelCase
type httpClient struct{}

// Acronyms: all caps
type HTTPClient struct{}; var userID int

// Errors: Err prefix
var ErrNotFound = errors.New("not found")

// Single-method interfaces: -er suffix
type Stringer interface{ String() string }

// Boolean: is/has prefix
func (u *User) IsAdmin() bool {}

// No stuttering: user.Name not user.UserName
```

##### Formatting

```bash
gofmt -w .
goimports -w .
```

##### Linting

```bash
go vet ./...
staticcheck ./...
golangci-lint run
```

A `golangci-lint` v2 configuration (the tool version is independent of the Go
language version and lints Go 1.21 code fine):

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

*End of Go 1.21 Complete Developer Guideline*
