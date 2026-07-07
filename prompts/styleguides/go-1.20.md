# Go 1.20 — Complete Developer Guideline

> **Version**: Go 1.20 (released 2023-02-01)
> **Scope**: Full language spec, all 1.20 changes delta over 1.19, idioms, concurrency, performance, CLI, file I/O, testing, and best practices — with examples throughout.
> **Note**: This document is self-contained. All material from 1.19 is either carried forward or superseded here. Sections marked **[NEW in 1.20]** cover additions specific to this version.

---

## Table of Contents

1. [What's New in Go 1.20](#whats-new-in-go-120)
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
14. [Concurrency](#concurrency)
15. [Context Package](#context-package)
16. [File I/O & Streaming](#file-io--streaming)
17. [Testing & Benchmarking](#testing--benchmarking)
18. [CLI Development](#cli-development)
19. [Performance Caveats & PGO](#performance-caveats--pgo)
20. [Idioms & Things to Avoid](#idioms--things-to-avoid)

---

## What's New in Go 1.20

Go 1.20 shipped 2023-02-01, six months after Go 1.19. It includes **four language changes**, major stdlib additions, and notable toolchain improvements. Build speed improved ~10% vs 1.19.

### Language changes

#### 1. Slice to array conversion [NEW in 1.20]

Go 1.17 allowed conversion from slice to *array pointer*. Go 1.20 extends this to allow direct **slice to array** conversion (a value copy, not a pointer):

```go
s := []byte{1, 2, 3, 4, 5}

// Go 1.17+: slice to array pointer
p := (*[4]byte)(s)   // *[4]byte, shares backing memory

// Go 1.20+: slice to array VALUE (copies data)
a := [4]byte(s)      // [4]byte, independent copy
// panics at runtime if len(s) < 4
```

The conversion panics at runtime if the slice is shorter than the target array. This is a compile-time safe conversion — the check is at runtime.

```go
// Safe pattern: check length first
func toArray(s []byte) ([4]byte, bool) {
    if len(s) < 4 {
        return [4]byte{}, false
    }
    return [4]byte(s), true
}
```

#### 2. New unsafe functions [NEW in 1.20]

Three new functions complete the ability to construct and deconstruct slice and string values without relying on their internal layout:

```go
// unsafe.SliceData(s []E) *E
// Returns pointer to underlying array of slice s.
// If cap(s) > 0: returns &s[:1][0]
// If s == nil:   returns nil
// Otherwise:     returns a non-nil pointer to unspecified memory
s := []int{1, 2, 3}
ptr := unsafe.SliceData(s)  // *int, points to s[0]

// unsafe.String(ptr *byte, len IntegerType) string
// Constructs a string from a *byte and length without a copy.
// Replaces the unsafe *(*string)(unsafe.Pointer(&reflect.StringHeader{...})) pattern.
buf := []byte("hello")
str := unsafe.String(&buf[0], len(buf)) // "hello", no copy

// unsafe.StringData(s string) *byte
// Returns pointer to underlying bytes of string s.
// If s == "": returns nil or a non-nil pointer to unspecified memory.
p2 := unsafe.StringData("hello") // *byte pointing to 'h'
```

These functions should only be used in low-level code (e.g., high-performance serialisers). Prefer standard `string([]byte)` / `[]byte(string)` conversions in normal code.

#### 3. Comparable constraint relaxation [NEW in 1.20]

Comparable types (including ordinary interfaces) may now satisfy `comparable` constraints, even if the underlying type is not strictly comparable (comparison might panic at runtime):

```go
// Before 1.20: compile error — interface{} does not implement comparable
// After 1.20:  compiles, but may panic at runtime if two non-comparable values are compared
type Set[T comparable] struct {
    m map[T]struct{}
}

var s Set[any] // now allowed
s.m = map[any]struct{}{}
s.m[42] = struct{}{}
s.m["hello"] = struct{}{} // ok
// s.m[[]int{1,2,3}] = struct{}{} // runtime panic: unhashable type
```

This relaxation makes it possible to use interface types as generic map keys. The trade-off: you may get a runtime panic instead of a compile error. **Be careful** when using interface types as `comparable` type arguments.

#### 4. Struct and array comparison spec clarification [NEW in 1.20]

The spec now formally states:
- Structs are compared **one field at a time**, in declaration order, stopping at the first mismatch.
- Arrays are compared **one element at a time**, in index order.

This was always the implementation behaviour; the spec change prevents a comparison from being required to evaluate fields/elements after the first mismatch, which can affect whether a comparison panics:

```go
type T struct {
    A int
    B interface{} // may be non-comparable
}

t1 := T{A: 1, B: func() {}}
t2 := T{A: 2, B: func() {}}
// Comparing t1 == t2 will NOT panic in 1.20:
// A differs (1 != 2), so comparison stops before reaching B.
// In theory, old spec could have required evaluating B and panicking.
```

### Toolchain

#### Profile-Guided Optimization (PGO) — preview [NEW in 1.20]

```bash
# Step 1: build and run your program to collect a CPU profile
go build -o myapp .
GOPROFILE=cpu.pprof ./myapp   # or use net/http/pprof

# Step 2: place profile as default.pgo in main package directory
cp cpu.pprof cmd/myapp/default.pgo

# Step 3: build with PGO (auto-detects default.pgo)
go build -pgo=auto ./cmd/myapp
# or explicitly:
go build -pgo=cpu.pprof ./cmd/myapp
```

PGO enables more aggressive function inlining at hot call sites. Benchmarks show **3–4% performance improvement** on representative Go programs. Build speed also improved ~10% vs 1.19.

#### Code coverage for binaries [NEW in 1.20]

Previously coverage only worked for unit tests. 1.20 extends it to full binaries:

```bash
go build -cover -o myapp .
mkdir /tmp/covdata
GOCOVERDIR=/tmp/covdata ./myapp
go tool covdata percent -i=/tmp/covdata
go tool covdata textfmt -i=/tmp/covdata -o coverage.txt
```

#### go command flags [NEW in 1.20]

```bash
# -C: change directory before executing command (useful in scripts)
go -C ./subdir build .

# -skip: skip matching tests or subtests
go test -skip TestSlow ./...

# go generate -skip: skip matching directives
go generate -skip "//go:generate stringer" ./...
```

#### vet improvements [NEW in 1.20]

The `vet` tool now detects:
- Loop variable capture in subtests after `t.Parallel()` calls.
- Incorrect time formats: using `2006-02-01` (yyyy-dd-mm) when you likely meant `2006-01-02` (ISO 8601 yyyy-mm-dd).

```bash
go vet ./...
```

### Runtime

- GC internal data structures reorganised: up to **2% CPU improvement** and reduced memory overhead.
- GC goroutine assists are less erratic.
- New `runtime/coverage` package for coverage data from long-running programs.
- Linux linker now selects `glibc` or `musl` dynamic interpreter at link time.
- `math/rand` global RNG is now **auto-seeded** with a random value by default (see Standard Library section).

### Standard Library — selected new additions

| Package | Addition | Notes |
|---|---|---|
| `errors` | `errors.Join(errs ...error) error` | Wraps multiple errors into one |
| `fmt` | Multiple `%w` in `fmt.Errorf` | Returns multi-wrapped error |
| `context` | `context.WithCancelCause`, `context.Cause` | Cancel with a reason |
| `strings` | `strings.CutPrefix`, `strings.CutSuffix` | Like Trim* but reports if trim occurred |
| `bytes` | `bytes.CutPrefix`, `bytes.CutSuffix`, `bytes.Clone` | Mirrors strings additions |
| `time` | `time.DateTime`, `time.DateOnly`, `time.TimeOnly` layout constants | Common layouts as constants |
| `time` | `time.Compare` method | Compare two Time values |
| `sync` | `sync.Map.Swap`, `sync.Map.CompareAndSwap`, `sync.Map.CompareAndDelete` | Atomic map entry updates |
| `net/http` | `http.ResponseController` | Per-request deadline/flush control |
| `crypto/ecdh` | New package | ECDH key exchanges over NIST curves and Curve25519 |
| `io` | `io.OffsetWriter` | Wraps WriterAt with a fixed offset |
| `io/fs` | `fs.SkipAll` | Terminates WalkDir immediately and successfully |
| `path/filepath` | `filepath.SkipAll`, `filepath.IsLocal` | Walk terminator + local path check |
| `math/rand` | Auto-seeded global RNG; `Seed` and `Read` deprecated | Use `rand.New(rand.NewSource(seed))` for reproducible sequences |

---

## Project Layout & Modules

### Module initialisation

```bash
mkdir myapp && cd myapp
go mod init github.com/yourname/myapp
```

This creates `go.mod`. Never edit it manually for dependencies; use `go get`, `go mod tidy`.

### Verify library APIs against the pinned version

Verify library APIs against the actual module versions in `go.mod` before
claiming an API is missing or unavailable.

- Do not claim a controller-runtime helper is unavailable without checking the
  pinned `sigs.k8s.io/controller-runtime` version.
- Say "this API is not available in vX.Y.Z" only when the module version proves
  it.
- Do not infer API availability from memory, a newer version's docs, or another
  project's dependency set.

### Recommended directory layout

```
myapp/
├── cmd/
│   └── myapp/
│       ├── main.go         # entry point
│       └── default.pgo     # PGO profile (Go 1.20+, optional)
├── internal/
│   ├── server/
│   └── storage/
├── pkg/
│   └── util/
├── api/
├── go.mod
└── go.sum
```

- `cmd/` — each subdirectory is a separate binary (`main` package).
- `internal/` — importable only within this module; enforced by the compiler.
- `pkg/` — public packages intended for external use.
- `default.pgo` in `cmd/<binary>/` — automatically used by `go build -pgo=auto` (1.20+).

### Package naming rules

- Lowercase, single word, no underscores: `storage`, `httputil`, `parser`.
- The package name is the last element of the import path.
- File names are snake_case: `user_service.go`, `user_service_test.go`.

---

## Basic Types & Variables

### Numeric types

| Type | Size | Notes |
|---|---|---|
| `int8`, `int16`, `int32`, `int64` | 1–8 bytes | Signed |
| `uint8`, `uint16`, `uint32`, `uint64` | 1–8 bytes | Unsigned |
| `int`, `uint` | platform (32 or 64 bit) | Use for general indexing |
| `uintptr` | platform | Holds a pointer value as integer |
| `float32`, `float64` | 4, 8 bytes | IEEE 754 |
| `complex64`, `complex128` | — | Rarely needed |
| `byte` = `uint8`, `rune` = `int32` | — | Aliases |

**Prefer `int` for loop counters and sizes** unless you have a specific reason to constrain range.

### Declaration forms

```go
var x int = 10
var y = "hello"
z := 3.14          // short declaration — inside functions only
a, b := 1, 2       // multiple assignment
_, err := os.Open("file") // blank identifier

const MaxSize = 1024
const Pi = 3.14159
```

### Zero values

| Type | Zero value |
|---|---|
| `int`, `float64` | `0` |
| `bool` | `false` |
| `string` | `""` |
| pointer, slice, map, chan, func, interface | `nil` |

### iota — enum-like constants

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

p := Read | Write
fmt.Println(p&Read != 0) // true
```

Go has no `enum` keyword. `iota` + typed constants is the idiomatic replacement.

---

## Pointers

### Basics

```go
x := 42
p := &x
fmt.Println(*p) // 42
*p = 99
fmt.Println(x)  // 99
```

### new vs &T{}

```go
p1 := new(int)          // *int pointing to 0
p2 := &Point{X: 1, Y: 2} // preferred for structs
```

### When to use pointers

- **Mutation**: function must modify its argument.
- **Large structs**: avoid copying on every call.
- **Optional values**: `*T` can be `nil` to represent absence.
- **Interfaces**: if any method has a pointer receiver, pass `*T` to satisfy the interface.

### Loop variable capture (critical in Go 1.20)

```go
// WRONG in Go 1.20 — all closures share the same i variable
for i := 0; i < 3; i++ {
    go func() { fmt.Println(i) }() // all may print 3
}

// CORRECT — shadow i to create a new binding each iteration
for i := 0; i < 3; i++ {
    i := i
    go func() { fmt.Println(i) }()
}

// Also correct — pass as argument
for i := 0; i < 3; i++ {
    go func(n int) { fmt.Println(n) }(i)
}
// Note: Go 1.22+ fixes loop variable semantics automatically.
// In Go 1.20 you MUST shadow or pass as argument.
```

---

## Control Flow

### if / else

```go
if x > 0 {
    fmt.Println("positive")
} else if x < 0 {
    fmt.Println("negative")
} else {
    fmt.Println("zero")
}

// Init statement — err scoped to the if block
if err := doSomething(); err != nil {
    return err
}

// Idiomatic: no else after return
f, err := os.Open(name)
if err != nil {
    return err
}
use(f) // no else needed
```

**Do NOT write `else` after a `return`** — unnecessary and non-idiomatic in Go.

### for — the only loop

```go
// C-style
for i := 0; i < 10; i++ { fmt.Println(i) }

// While-style
for x < 100 { x *= 2 }

// Infinite loop
for {
    if done() { break }
}

// Range over slice
s := []string{"a", "b", "c"}
for i, v := range s { fmt.Println(i, v) }
for i := range s    { fmt.Println(i) }
for _, v := range s { fmt.Println(v) }

// Range over map (order randomised every run)
m := map[string]int{"x": 1, "y": 2}
for k, v := range m { fmt.Println(k, v) }

// Range over string — iterates runes, not bytes
for i, r := range "héllo" {
    fmt.Printf("%d: %c\n", i, r)
}

// Range over channel — reads until closed
for v := range ch { fmt.Println(v) }
```

### break / continue / labels

```go
outer:
for i := 0; i < 5; i++ {
    for j := 0; j < 5; j++ {
        if j == 2 { continue outer }
        if i == 3 { break outer }
    }
}
```

### switch

```go
// Expression switch — no fallthrough by default
switch x {
case 1:
    fmt.Println("one")
case 2, 3:
    fmt.Println("two or three")
default:
    fmt.Println("other")
}

// Init statement
switch os := runtime.GOOS; os {
case "linux":  fmt.Println("Linux")
case "darwin": fmt.Println("macOS")
}

// No condition — replaces if-else chains
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
    default:     fmt.Printf("unknown: %T\n", v)
    }
}

// Explicit fallthrough (rare)
switch x {
case 1:
    fmt.Println("one")
    fallthrough
case 2:
    fmt.Println("also runs for 1 and 2")
}
```

Go's `switch` does **not** fall through by default — no `break` needed.

### select — channels only

```go
select {
case msg := <-ch1:
    fmt.Println("received", msg)
case ch2 <- "hello":
    fmt.Println("sent")
case <-time.After(1 * time.Second):
    fmt.Println("timeout")
default:
    fmt.Println("no channel ready — non-blocking")
}
```

---

## Functions

### Basics

```go
func add(a, b int) int { return a + b }

// Multiple return values
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}

// Named returns — use sparingly, only for short functions
func minMax(a, b int) (min, max int) {
    if a < b { return a, b }
    min, max = b, a
    return
}
```

### Variadic

```go
func sum(nums ...int) int {
    total := 0
    for _, n := range nums { total += n }
    return total
}
sum(1, 2, 3)

s := []int{1, 2, 3}
sum(s...) // spread
```

### First-class functions & closures

```go
type Transformer func(string) string

func apply(s string, fns ...Transformer) string {
    for _, fn := range fns { s = fn(s) }
    return s
}

result := apply("hello", strings.ToUpper, func(s string) string { return s + "!" })
// "HELLO!"

// Closure — captures by reference
func counter() func() int {
    n := 0
    return func() int { n++; return n }
}
c := counter()
fmt.Println(c(), c(), c()) // 1 2 3
```

**Always shadow loop variables before passing to goroutines** (see Pointers section).

### init functions

```go
func init() {
    // runs before main, once per package, in source order
    // use for: registering drivers, validating static config
}
```

---

## Defer, Panic & Recover

### defer

Schedules a call to run when the surrounding function returns. Multiple defers run **LIFO**.

```go
func readFile(name string) error {
    f, err := os.Open(name)
    if err != nil { return err }
    defer f.Close() // guaranteed

    // process f
    return nil
}
```

```go
// LIFO demonstration
func demo() {
    defer fmt.Println("runs last")
    defer fmt.Println("runs second")
    defer fmt.Println("runs first")
    fmt.Println("body")
}
// body / runs first / runs second / runs last
```

**Defer arguments are evaluated immediately:**

```go
x := 10
defer fmt.Println(x) // prints 10, not 20
x = 20
```

**Defer modifying named returns:**

```go
func double(x int) (result int) {
    defer func() { result *= 2 }()
    result = x
    return
}
// double(5) == 10
```

**Avoid defer in tight loops** — overhead per call.

### panic

```go
func mustPositive(x int) int {
    if x <= 0 {
        panic(fmt.Sprintf("expected positive, got %d", x))
    }
    return x
}
```

Use panic only for unrecoverable programmer errors. **Never panic for business logic.**

### recover

```go
func safeDiv(a, b int) (result int, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("recovered: %v", r)
        }
    }()
    return a / b, nil
}
```

Only recover at package/API boundaries. Never silently swallow panics.

---

## Structs & Methods

### Definition

```go
type User struct {
    ID    int
    Name  string
    Email string
    admin bool // unexported
}

// Named fields — always use this form
u := User{ID: 1, Name: "Alice", Email: "alice@example.com"}
```

### Embedded structs (composition)

```go
type Timestamps struct {
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Post struct {
    ID    int
    Title string
    Timestamps // promoted fields
}

p := Post{ID: 1, Title: "Hello"}
p.CreatedAt = time.Now()
```

### Methods

```go
func (u User) Display() string {
    return fmt.Sprintf("%s <%s>", u.Name, u.Email)
}

func (u *User) Promote() {
    u.admin = true
}
```

### Value vs. Pointer receivers

| Condition | Receiver |
|---|---|
| Method mutates receiver | `*T` |
| Struct is large | `*T` |
| Contains `sync.Mutex` or similar | `*T` (never copy a mutex) |
| Read-only, small struct | `T` |
| Any method uses `*T` | Use `*T` for ALL methods |

**Never mix value and pointer receivers on the same type.**

### Struct comparison in 1.20

As of 1.20, the spec formally guarantees comparison stops at the first mismatched field/element. Practically: don't compare structs with non-comparable fields (like slices or maps) — use `reflect.DeepEqual` or a custom method.

### Constructor pattern

```go
func NewServer(host string, port int) *Server {
    return &Server{host: host, port: port, timeout: 30 * time.Second}
}

// Functional options for complex configuration
type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}

s := NewServer("localhost", 8080, WithTimeout(60*time.Second))
```

---

## Interfaces & Embedding

### Definition & implicit satisfaction

```go
type Writer interface {
    Write(p []byte) (n int, err error)
}

type WriteCloser interface {
    Writer
    io.Closer
}

type File struct{}
func (f *File) Write(p []byte) (int, error) { return len(p), nil }
func (f *File) Close() error                { return nil }

var wc WriteCloser = &File{} // compiles: *File satisfies both
```

### Usage patterns

```go
// Accept interface, return concrete type
func Copy(dst io.Writer, src io.Reader) (int64, error) { /*...*/ }

// Empty interface / any
var x any = 42
x = "now a string"

s, ok := x.(string) // type assertion
if !ok { /* not a string */ }
```

### Comparable interfaces in generics (1.20)

```go
// Now allowed: instantiate comparable with interface type
type Set[T comparable] struct{ m map[T]struct{} }

var s Set[any]             // OK in 1.20
s.m = map[any]struct{}{}
s.m["key"] = struct{}{}   // OK
// s.m[[]int{1}] = struct{}{} // runtime panic — slices are not comparable
```

### Avoid fat interfaces

```go
// Good: small, composable interfaces
type UserReader interface { FindByID(id int) (User, error) }
type UserWriter interface { Create(u User) error; Update(u User) error }
```

---

## Generics

### Type parameters & constraints

```go
func Map[T, U any](s []T, f func(T) U) []U {
    result := make([]U, len(s))
    for i, v := range s { result[i] = f(v) }
    return result
}

strs := Map([]int{1, 2, 3}, strconv.Itoa) // ["1","2","3"]

// comparable constraint
func Contains[T comparable](s []T, target T) bool {
    for _, v := range s {
        if v == target { return true }
    }
    return false
}

// Custom number constraint
type Number interface { ~int | ~int64 | ~float64 }

func Sum[T Number](s []T) T {
    var total T
    for _, v := range s { total += v }
    return total
}
```

### Generic structs

```go
type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T)  { s.items = append(s.items, v) }
func (s *Stack[T]) Pop() T {
    n := len(s.items)
    v := s.items[n-1]
    s.items = s.items[:n-1]
    return v
}
func (s *Stack[T]) Len() int { return len(s.items) }
```

### Type declarations inside generic functions [NEW in 1.20]

Go 1.20's compiler upgrade now allows type declarations inside generic functions and methods:

```go
func Process[T any](items []T) {
    type result struct { // type declaration inside generic function — allowed in 1.20
        item  T
        index int
    }
    results := make([]result, len(items))
    for i, v := range items {
        results[i] = result{item: v, index: i}
    }
    _ = results
}
```

### Comparable interface as constraint (1.20)

```go
// Now compiles — interface types satisfy comparable
type Registry[K comparable, V any] struct {
    m map[K]V
}

var r Registry[any, string] // K=any, which satisfies comparable in 1.20
```

### When to use generics

Use for:
- Utility functions on slices/maps/channels (filter, map, reduce).
- Generic data structures (Stack, Queue, Set).
- Functions with identical algorithm across many types.

Avoid when:
- A simple interface is equally clear.
- Writing a concrete domain model.
- You'd need to constrain to methods — use an interface instead.

### 1.20 limitations

- No generic methods on non-generic types (type params on methods only when the receiver type is also generic).
- Type inference is partial — sometimes types must be specified explicitly.

---

## Collection Types: Arrays, Slices, Maps

### Arrays

Fixed-length, value type. `[3]int` and `[4]int` are different types. Copied on assignment.

```go
var a [3]int          // [0, 0, 0]
b := [3]int{1, 2, 3}
c := [...]int{4, 5, 6}

// Slice to array conversion — GO 1.20 (value copy)
s := []byte{1, 2, 3, 4, 5}
arr := [4]byte(s)   // independent copy; panics if len(s) < 4
// arr is [1,2,3,4]; modifying arr does NOT affect s
```

### Slices

A **view** into an underlying array: pointer + length + capacity.

```go
s := []int{1, 2, 3}
s2 := make([]int, 0, 10) // len=0, cap=10

arr := [5]int{0,1,2,3,4}
sl := arr[1:4]      // shares memory
sl[0] = 99
fmt.Println(arr[1]) // 99 — arr was modified!

// Independent copy
c := make([]int, len(sl))
copy(c, sl)

// Append — always capture the return value
s2 = append(s2, 1, 2, 3)
extra := []int{4, 5}
s2 = append(s2, extra...)
```

#### Slice growth

When `append` exceeds capacity: Go allocates a new backing array and copies. Doubles up to ~1024 elements, then ~25% growth.

#### Preallocate when size is known

```go
// Bad — O(n) reallocations
var result []string
for _, item := range items { result = append(result, process(item)) }

// Good — single allocation
result := make([]string, 0, len(items))
for _, item := range items { result = append(result, process(item)) }
```

#### Modification during iteration

```go
// Safe: modify elements in place
for i := range s { s[i] *= 2 }

// Dangerous: append during range (slice header captured at start)
// Instead: collect then append
var extra []int
for _, v := range s {
    if v > 0 { extra = append(extra, v*2) }
}
s = append(s, extra...)

// Delete preserving order
s = append(s[:i], s[i+1:]...)

// Delete without preserving order (faster)
s[i] = s[len(s)-1]
s = s[:len(s)-1]
```

### Maps

```go
// Never: var m map[string]int — nil map, writes panic
m := make(map[string]int)
m2 := map[string]int{"alpha": 1, "beta": 2}

v, ok := m["alpha"] // two-value form: detect missing keys
m["gamma"] = 3
delete(m, "alpha")
fmt.Println(len(m))
```

#### Iteration — order is randomised

```go
for k, v := range m { fmt.Println(k, v) }

// Sorted
keys := make([]string, 0, len(m))
for k := range m { keys = append(keys, k) }
sort.Strings(keys)
for _, k := range keys { fmt.Println(k, m[k]) }
```

#### Modification during iteration

Deleting keys during range is safe. Adding keys during iteration may or may not be visited.

```go
for k := range m {
    if shouldDelete(k) { delete(m, k) } // safe
}
```

Only flag map mutation during iteration when there is a concrete Go issue, for
example:

- unsynchronized concurrent map access
- inserting or updating entries while depending on deterministic iteration
- deleting entries changes required business behavior
- callbacks are invoked while holding a lock and can re-enter the same lock

Do not apply generic "modifying a collection while iterating" rules from other
languages to Go map deletion.

#### Maps are reference types

```go
a := map[string]int{"x": 1}
b := a         // same map
b["x"] = 99
fmt.Println(a["x"]) // 99
```

#### Maps are NOT safe for concurrent access

```go
// Use sync.RWMutex or sync.Map
var mu sync.RWMutex
mu.Lock()
m["a"] = 1
mu.Unlock()
```

---

## Strings

### Key facts

- Immutable sequence of bytes; UTF-8 by default.
- `len(s)` = bytes, not runes.
- `range` on a string iterates runes (Unicode code points).

```go
s := "héllo"
fmt.Println(len(s))         // 6 (é = 2 bytes)
fmt.Println(len([]rune(s))) // 5

for i, r := range s {
    fmt.Printf("byte %d: %c (U+%04X)\n", i, r, r)
}
```

### Efficient building

```go
var b strings.Builder
b.Grow(1000)
for i := 0; i < 1000; i++ { b.WriteByte('x') }
result := b.String()
```

### New in 1.20: CutPrefix / CutSuffix

```go
// strings.CutPrefix — like TrimPrefix but also reports if trim happened
after, found := strings.CutPrefix("Gopher", "Go")
// after="pher", found=true

// strings.CutSuffix
before, found := strings.CutSuffix("Gopher", "er")
// before="Goph", found=true

// Compare to TrimPrefix — no found indicator
strings.TrimPrefix("Gopher", "Go") // "pher" (no way to tell if trim occurred)
```

### Common operations

```go
strings.Contains("foobar", "bar")       // true
strings.HasPrefix("foobar", "foo")      // true
strings.HasSuffix("foobar", "bar")      // true
strings.Split("a,b,c", ",")            // ["a","b","c"]
strings.Join([]string{"a","b"}, "-")   // "a-b"
strings.TrimSpace("  hello  ")         // "hello"
strings.ToUpper("hello")               // "HELLO"
strings.ReplaceAll("aabbcc", "b", "x") // "aaxxcc"
```

---

## Error Handling

### The basics

```go
type error interface { Error() string }

result, err := doSomething()
if err != nil {
    return fmt.Errorf("doSomething failed: %w", err)
}
```

**Always check errors.** A few discards are idiomatic and should not be flagged:

- deferred `Close()` on files opened only for reading
- `defer tx.Rollback()`, which is a no-op after a successful commit
- writes to `bytes.Buffer` or `strings.Builder`, which never return an error

For everything else, handle the error or make the discard explicit with `_ =`.

### Sentinel errors

```go
var ErrNotFound = errors.New("not found")

if errors.Is(err, ErrNotFound) { /* handle */ }
```

### Custom error types

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation: %s: %s", e.Field, e.Message)
}

var ve *ValidationError
if errors.As(err, &ve) { fmt.Println("bad field:", ve.Field) }
```

### Multi-error wrapping [NEW in 1.20]

Go 1.20 adds two ways to wrap multiple errors:

```go
// errors.Join — combines multiple errors
err1 := errors.New("database error")
err2 := errors.New("cache error")

combined := errors.Join(err1, err2)
// combined.Error() == "database error\ncache error"

// errors.Is and errors.As now inspect multiply-wrapped errors
fmt.Println(errors.Is(combined, err1)) // true
fmt.Println(errors.Is(combined, err2)) // true

// fmt.Errorf with multiple %w [NEW in 1.20]
wrapped := fmt.Errorf("two failures: %w and %w", err1, err2)
fmt.Println(errors.Is(wrapped, err1)) // true
fmt.Println(errors.Is(wrapped, err2)) // true

// Custom multi-error type — implement Unwrap() []error
type MultiError struct {
    Errors []error
}
func (m *MultiError) Error() string {
    msgs := make([]string, len(m.Errors))
    for i, e := range m.Errors { msgs[i] = e.Error() }
    return strings.Join(msgs, "; ")
}
func (m *MultiError) Unwrap() []error { return m.Errors }
```

### When to use errors.Join vs fmt.Errorf with %w

| Situation | Use |
|---|---|
| Combining unrelated parallel errors | `errors.Join(err1, err2, ...)` |
| Adding context to two related errors | `fmt.Errorf("context: %w and %w", err1, err2)` |
| Single error with context | `fmt.Errorf("context: %w", err)` |
| List of errors from goroutines | `errgroup` or collect into `[]error` then `errors.Join` |

### Don't log AND return

```go
// Bad
if err != nil {
    log.Println(err) // logs here
    return err       // and propagates — logged again by caller
}

// Good: return the error; log at the top boundary only
```

### Concurrent error collection

```go
import "golang.org/x/sync/errgroup"

g, ctx := errgroup.WithContext(context.Background())
for _, url := range urls {
    url := url
    g.Go(func() error { return fetch(ctx, url) })
}
if err := g.Wait(); err != nil {
    return err
}
```

---

## Concurrency

### Goroutines

```go
go func() { fmt.Println("concurrent") }()

// Pass loop variable as argument
for i := 0; i < 5; i++ {
    go func(n int) { fmt.Println(n) }(i)
}
```

Goroutines start with ~2-4 KB stack, grown dynamically. Safely create thousands.

### Channels

```go
// Unbuffered
ch := make(chan int)
go func() { ch <- 42 }()
val := <-ch

// Buffered
bch := make(chan string, 10)
bch <- "hello"

// Close and range
close(ch)
for v := range ch { fmt.Println(v) }

// Check if closed
v, ok := <-ch
if !ok { fmt.Println("closed") }
```

**Channel rules:**
- Only the **sender** closes a channel.
- Sending to a closed channel panics.
- Receiving from a closed channel returns zero value immediately.

### sync.WaitGroup

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

### sync.Mutex / sync.RWMutex

```go
type SafeCounter struct {
    mu sync.Mutex
    v  map[string]int
}

func (c *SafeCounter) Inc(key string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.v[key]++
}

type Cache struct {
    mu   sync.RWMutex
    data map[string]string
}

func (c *Cache) Get(key string) (string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    v, ok := c.data[key]
    return v, ok
}

func (c *Cache) Set(key, val string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.data[key] = val
}
```

**Never copy a mutex.** Embed it; use pointer receivers.

### sync.Once

```go
var (
    instance *Config
    once     sync.Once
)

func GetConfig() *Config {
    once.Do(func() { instance = loadConfig() })
    return instance
}
```

### sync.Map — new methods in 1.20 [NEW in 1.20]

```go
var m sync.Map

m.Store("key", "value")
v, ok := m.Load("key")
m.Delete("key")

// NEW in 1.20: Swap
old, loaded := m.Swap("key", "newvalue")
// old = previous value if it existed, loaded = true if it existed

// NEW in 1.20: CompareAndSwap
swapped := m.CompareAndSwap("key", "oldvalue", "newvalue")
// swapped = true only if current value equals "oldvalue"

// NEW in 1.20: CompareAndDelete
deleted := m.CompareAndDelete("key", "value")
// deleted = true only if current value equals "value"

m.Range(func(k, v any) bool {
    fmt.Println(k, v)
    return true // false to stop
})
```

### Atomic types (from 1.19)

```go
var counter atomic.Int64
counter.Add(1)
fmt.Println(counter.Load())

var ptr atomic.Pointer[Config]
ptr.Store(&Config{})
cfg := ptr.Load()
```

### Worker pool pattern

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

jobs    := make(chan Job, 100)
results := make(chan Result, 100)
workerPool(jobs, results, runtime.NumCPU())
for _, j := range work { jobs <- j }
close(jobs)
for r := range results { handle(r) }
```

### Concurrency pitfalls

| Pitfall | Symptom | Fix |
|---|---|---|
| Goroutine leak | Memory grows indefinitely | Provide exit path; use context |
| Race condition | Occasional wrong values / crash | `go test -race` |
| Deadlock | Program hangs | Ensure every send has a receive; avoid circular locks |
| Closing closed channel | Panic | Only sender closes; use `sync.Once` |
| Loop variable capture | Wrong goroutine values | Shadow: `i := i` or pass as arg |
| Copying mutex | Silently broken | Pointer receivers + embedding |

### Race detector

```bash
go test -race ./...
go run -race main.go
```

**Always run with `-race` in CI.**

### Classifying races before reporting

Separate true data races from lifecycle, shutdown, or ordering races. Only call
something a data race after confirming unsynchronized shared-memory access with
at least one write.

Check a library's concurrency contract before assuming concurrent method calls
are unsafe. Some Go types are explicitly safe for concurrent use, while others
require caller-side synchronization.

---

## Context Package

### Creation

```go
ctx := context.Background() // root; never cancelled
ctx := context.TODO()       // placeholder

ctx, cancel := context.WithCancel(parent)
defer cancel()

ctx, cancel = context.WithTimeout(parent, 5*time.Second)
defer cancel()

ctx, cancel = context.WithDeadline(parent, time.Now().Add(10*time.Second))
defer cancel()

ctx = context.WithValue(parent, myKey{}, "value")
```

### WithCancelCause — NEW in 1.20

```go
// Pass a specific error as the cancellation reason
ctx, cancel := context.WithCancelCause(parent)
defer cancel(nil) // nil = no specific cause

// Cancel with a reason
cancel(errors.New("rate limit exceeded"))

// Retrieve the cause
cause := context.Cause(ctx)
// cause = "rate limit exceeded" error
// ctx.Err() = context.Canceled (always, regardless of cause)
```

This is useful for distinguishing *why* a context was cancelled without using `context.WithValue`:

```go
func fetchWithCause(ctx context.Context, url string) error {
    ctx, cancel := context.WithCancelCause(ctx)
    go func() {
        if rateLimited() {
            cancel(ErrRateLimited)
        }
    }()
    defer cancel(nil)

    err := doFetch(ctx, url)
    if err != nil {
        if cause := context.Cause(ctx); cause != nil {
            return fmt.Errorf("fetch cancelled: %w", cause)
        }
        return err
    }
    return nil
}
```

### Convention: context as first parameter

```go
func FetchUser(ctx context.Context, id int) (*User, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    // ...
}
```

### Checking cancellation in long loops

```go
func processLargeDataset(ctx context.Context, items []Item) error {
    for _, item := range items {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        if err := processItem(ctx, item); err != nil { return err }
    }
    return nil
}
```

### WithValue keys

```go
type contextKey string
const requestIDKey contextKey = "requestID"

ctx = context.WithValue(ctx, requestIDKey, "abc-123")
if id, ok := ctx.Value(requestIDKey).(string); ok {
    log.Println("request", id)
}
```

### Rules

- **Never store context in a struct** — pass as first parameter.
- **Never pass nil context** — use `context.Background()`.
- **Always `defer cancel()`** after `WithCancel`/`WithTimeout`/`WithDeadline`/`WithCancelCause`.
- Only store request-scoped data in `WithValue`.

---

## File I/O & Streaming

### Open / Close with defer

```go
f, err := os.Open("file.txt")
if err != nil { return err }
defer f.Close()
```

### Write a file

```go
f, err := os.Create("output.txt")
if err != nil { return err }
defer f.Close()

if _, err := fmt.Fprintln(f, "hello world"); err != nil { return err }
```

### Buffered I/O with bufio

```go
// Buffered writer
w := bufio.NewWriter(f)
defer w.Flush() // MUST flush before close

for _, line := range lines { fmt.Fprintln(w, line) }

// Line-by-line scanner
scanner := bufio.NewScanner(f)
for scanner.Scan() { process(scanner.Text()) }
if err := scanner.Err(); err != nil { return err }

// Large lines (> 64KB default)
scanner.Buffer(make([]byte, 1<<20), 1<<20)
```

### Read entire file

```go
data, err := os.ReadFile("config.json") // small files only
```

### Streaming

```go
// Copy
written, err := io.Copy(dst, src)

// Limit reader
limited := io.LimitReader(src, 1<<20)

// Tee reader — read and write simultaneously
tee := io.TeeReader(src, &buf)
io.Copy(dst, tee)

// OffsetWriter — NEW in 1.20
// Wraps a WriterAt and adjusts all offsets by a fixed amount
ow := io.NewOffsetWriter(file, 512) // all writes offset by 512 bytes
ow.Write(data)
```

### Walk a directory tree

```go
err = filepath.WalkDir("./data", func(path string, d fs.DirEntry, err error) error {
    if err != nil { return err }
    if !d.IsDir() { fmt.Println(path) }
    return nil
})
```

#### SkipAll — NEW in 1.20

```go
// Terminate walk immediately and successfully (no error)
err = filepath.WalkDir("./data", func(path string, d fs.DirEntry, err error) error {
    if shouldStop(path) {
        return filepath.SkipAll // stop entire walk without error
    }
    return nil
})
// err == nil even though walk was cut short
```

#### IsLocal — NEW in 1.20

```go
// Reports whether a path is lexically local to a directory
filepath.IsLocal("subdir/file.txt") // true
filepath.IsLocal("../other")        // false — escapes current dir
filepath.IsLocal("/abs/path")       // false — absolute path
filepath.IsLocal(".")               // true
```

### JSON encoding / decoding

```go
enc := json.NewEncoder(w)
enc.SetIndent("", "  ")
if err := enc.Encode(data); err != nil { return err }

dec := json.NewDecoder(r)
var result MyStruct
if err := dec.Decode(&result); err != nil { return err }

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email,omitempty"`
    pass  string // unexported — never marshalled
}
```

### Temporary files

```go
tmp, err := os.CreateTemp("", "prefix-*.json")
if err != nil { return err }
defer os.Remove(tmp.Name())
defer tmp.Close()
```

---

## Testing & Benchmarking

### Unit tests

```go
func TestAdd(t *testing.T) {
    got := Add(2, 3)
    if got != 5 {
        t.Errorf("Add(2,3) = %d; want 5", got)
    }
}
```

```bash
go test ./...
go test -v ./...
go test -run TestAdd ./...

# NEW in 1.20: skip matching tests
go test -skip TestSlow ./...
```

### Table-driven tests (idiomatic)

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
            got, err := Divide(tt.a, tt.b)
            if (err != nil) != tt.wantErr {
                t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if !tt.wantErr && got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Test helpers

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
available in Go 1.20: `t.TempDir()` (1.15), `t.Setenv()` (1.17), and
`t.Cleanup()` (1.14).

Note: `t.Context()` and `t.Chdir()` are Go 1.24 additions and are not available
in Go 1.20.

### Benchmarks

```go
func BenchmarkAdd(b *testing.B) {
    for i := 0; i < b.N; i++ { Add(2, 3) }
}

// NEW in 1.20: B.Elapsed for rate calculations
func BenchmarkProcess(b *testing.B) {
    for i := 0; i < b.N; i++ {
        ProcessItems(items)
    }
    elapsed := b.Elapsed()
    b.ReportMetric(float64(len(items))/elapsed.Seconds(), "items/sec")
}
```

```bash
go test -bench=. -benchmem ./...
```

### Code coverage for binaries [NEW in 1.20]

```bash
# Build with coverage instrumentation
go build -cover -o myapp ./cmd/myapp

# Run and collect coverage
mkdir /tmp/covdata
GOCOVERDIR=/tmp/covdata ./myapp --do-stuff

# Analyse
go tool covdata percent -i=/tmp/covdata
go tool covdata textfmt  -i=/tmp/covdata -o coverage.out
go tool cover -html=coverage.out
```

### Fuzz testing

```go
func FuzzParseURL(f *testing.F) {
    f.Add("https://example.com/path?q=1")
    f.Fuzz(func(t *testing.T, input string) {
        _, _ = url.Parse(input) // must not panic
    })
}
```

```bash
go test -fuzz=FuzzParseURL -fuzztime=30s
```

### Loop variable capture detection [NEW in 1.20]

The `vet` tool now detects loop variable capture in subtests after `t.Parallel()`:

```go
// go vet will warn about this in 1.20
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        // tt is captured by reference — may be wrong value!
        got := run(tt.input) // BUG in < 1.20, detected by vet in 1.20
        _ = got
    })
}

// Correct: shadow tt
for _, tt := range tests {
    tt := tt // shadow
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        got := run(tt.input) // safe
        _ = got
    })
}
```

### Race detector

```bash
go test -race ./...
```

---

## CLI Development

### Standard `flag` package

```go
host    := flag.String("host", "localhost", "server host")
port    := flag.Int("port", 8080, "server port")
verbose := flag.Bool("verbose", false, "verbose output")

var t time.Time
flag.TextVar(&t, "time", time.Now(), "timestamp (RFC3339)") // 1.19+

flag.Parse()
args := flag.Args()
```

### Cobra (production CLIs)

```bash
go get github.com/spf13/cobra@latest
```

```go
var rootCmd = &cobra.Command{
    Use:   "myapp",
    Short: "My CLI application",
}

var serveCmd = &cobra.Command{
    Use:  "serve",
    RunE: func(cmd *cobra.Command, args []string) error {
        port, _ := cmd.Flags().GetInt("port")
        return startServer(port)
    },
}

func init() {
    rootCmd.PersistentFlags().Bool("verbose", false, "verbose output")
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

### Exit codes and stderr pattern

```go
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
        os.Exit(1)
    }
}

func run() error {
    // all application logic here
    return nil
}
```

---

## Performance Caveats & PGO

### Profile-Guided Optimization [NEW in 1.20]

PGO is the biggest performance addition in 1.20. It enables the compiler to inline more aggressively at hot call sites, giving ~3-4% average improvement.

```bash
# Step 1: collect a CPU profile from production or representative load
go build -o myapp ./cmd/myapp
./myapp &
go tool pprof -proto http://localhost:6060/debug/pprof/profile?seconds=30 > default.pgo

# Step 2: place default.pgo in your main package directory
mv default.pgo cmd/myapp/default.pgo

# Step 3: build with PGO auto-enabled
go build -pgo=auto ./cmd/myapp

# Explicitly disable PGO
go build -pgo=off ./cmd/myapp

# Verify PGO was used
go build -v -pgo=auto ./cmd/myapp 2>&1 | grep pgo
```

**PGO tips:**
- Collect profiles from real workloads, not synthetic benchmarks.
- `default.pgo` is committed to version control alongside your code.
- Re-collect profiles periodically as code changes.
- PGO is a **preview** in 1.20 — use with appropriate caution in production.

### Escape analysis

```bash
go build -gcflags="-m" ./...  # shows what escapes to heap
```

Values escape to heap when: a pointer is returned, stored in an interface, sent on a channel, or captured by an escaping closure.

### sync.Pool

```go
var bufPool = sync.Pool{
    New: func() any { return &bytes.Buffer{} },
}

func process(data []byte) string {
    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset()
    defer bufPool.Put(buf)
    buf.Write(data)
    return buf.String()
}
```

### String building

```go
// O(n^2) — avoid in loops
s := ""
for i := 0; i < n; i++ { s += "x" }

// O(n) — correct approach
var b strings.Builder
b.Grow(n)
for i := 0; i < n; i++ { b.WriteByte('x') }
```

### Slice preallocation

```go
result := make([]string, 0, len(items)) // single allocation
for _, item := range items {
    result = append(result, process(item))
}
```

### Interface boxing

Storing a concrete value in `any` / `interface{}` allocates on the heap. Avoid in hot paths.

### Struct memory layout

```go
// 24 bytes: bool gets 7 bytes padding before int64
type Bad struct {
    A bool  // 1 + 7 padding
    B int64 // 8
    C bool  // 1 + 7 padding
}

// Better: largest fields first
type Better struct {
    B int64 // 8
    A bool  // 1
    C bool  // 1 + 6 padding = 16 total
}
```

### GC tuning

```go
debug.SetMemoryLimit(512 << 20) // 512 MiB soft limit (from 1.19)
debug.SetGCPercent(200)         // less frequent GC, more memory

// For containers: set GOMEMLIMIT to ~90% of container memory limit
```

### math/rand auto-seeding [NEW in 1.20]

In 1.20, the global `math/rand` RNG is automatically seeded with a random value. The top-level `rand.Seed` and `rand.Read` are deprecated.

```go
// Old (1.19 and earlier): must seed manually for randomness
rand.Seed(time.Now().UnixNano()) // deprecated in 1.20

// New (1.20+): global RNG is auto-seeded — just use it
n := rand.Intn(100)   // random every run, no seed needed

// For reproducible sequences: allocate your own source
rng := rand.New(rand.NewSource(42))
n2 := rng.Intn(100)  // deterministic

// If you need the 1.19 behaviour (fixed seed at startup):
// GODEBUG=randautoseed=0
```

### Profiling

```bash
go test -bench=. -cpuprofile=cpu.prof
go test -bench=. -memprofile=mem.prof
go tool pprof cpu.prof
go tool pprof mem.prof

# HTTP endpoint in servers
import _ "net/http/pprof"
go http.ListenAndServe(":6060", nil)
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

---

## Time — New Constants [NEW in 1.20]

```go
// Before 1.20: had to remember the reference time format
t.Format("2006-01-02 15:04:05")
t.Format("2006-01-02")
t.Format("15:04:05")

// After 1.20: named constants
t.Format(time.DateTime)  // "2006-01-02 15:04:05"
t.Format(time.DateOnly)  // "2006-01-02"
t.Format(time.TimeOnly)  // "15:04:05"

time.Parse(time.DateTime, "2023-02-01 12:30:00")

// Time.Compare — NEW in 1.20
t1 := time.Now()
t2 := t1.Add(time.Hour)
t1.Compare(t2) // -1 (t1 before t2), 0 (equal), +1 (t1 after t2)
// Like strings.Compare but for time.Time
```

**Vet warning**: using `2006-02-01` (yyyy-dd-mm) in `Format`/`Parse` is now flagged by `go vet` — you almost certainly meant the ISO `2006-01-02`.

---

## HTTP — ResponseController [NEW in 1.20]

```go
// Old way: type-assert to discover optional interfaces
if flusher, ok := w.(http.Flusher); ok {
    flusher.Flush()
}

// New way: ResponseController — cleaner, more discoverable
func handler(w http.ResponseWriter, r *http.Request) {
    rc := http.NewResponseController(w)

    // Set per-request write deadline (overrides server-wide timeout)
    rc.SetWriteDeadline(time.Now().Add(30 * time.Second))

    // Flush buffered data to client
    rc.Flush()

    // Disable server write timeout for long-lived responses
    rc.SetWriteDeadline(time.Time{})
    io.Copy(w, bigDataReader)
}
```

---

## Idioms & Things to Avoid

### Do

- **Return errors as values.** Never panic for expected failures.
- **Accept interfaces, return concrete types.**
- **Error early** — return in the error branch; no nesting.
- **Use `defer` for all cleanup.**
- **Preallocate** slices/maps when size is known.
- **Shadow loop variables** before goroutines in Go 1.20: `i := i`.
- **Use `strings.Builder`** for multi-step string construction.
- **Use `fmt.Errorf("context: %w", err)`** to preserve error chains.
- **Use `errors.Join`** when aggregating multiple independent errors.
- **Use `context.WithCancelCause`** when callers need to know *why* something was cancelled.
- **Use `time.DateTime`/`time.DateOnly`/`time.TimeOnly`** instead of raw format strings.
- **Collect PGO profiles** from production to enable 3-4% free performance.
- **Always `defer cancel()`** after any context creation with cancellation.

### Don't

| Anti-pattern | Why | Instead |
|---|---|---|
| `var m map[string]int` (nil map) | Writing panics | `m := make(map[string]int)` |
| Mixing value and pointer receivers | Confuses method sets | Pick one type of receiver |
| Ignoring errors with `_` | Silent failures | Handle or propagate |
| Logging AND returning an error | Double-logging | Return; log at boundary |
| `rand.Seed(...)` in 1.20 | Deprecated; global RNG is auto-seeded | `rand.New(rand.NewSource(seed))` |
| `rand.Read(...)` in 1.20 | Deprecated | `crypto/rand.Read` for secure randomness |
| `time.Sleep` to synchronise goroutines | Racy | `sync.WaitGroup` or channels |
| Global mutable state | Untestable, racy | Dependency injection |
| `interface{}` / `any` everywhere | No type safety, boxing overhead | Typed interfaces or generics |
| `panic` for business logic | Crashes program | `return error` |
| Copying a mutex | Broken locking | Embed; pointer receivers |
| Closing channel from receiver | Panic if double-closed | Sender closes; `sync.Once` if needed |
| String `+` in loops | O(n^2) | `strings.Builder` |
| Storing context in struct | Anti-pattern | First function parameter |
| Appending to slice during range | Undefined behaviour | Collect then append |
| `[4]byte(s)` without len check | Runtime panic | Check `len(s) >= 4` first |
| Using `comparable` with interfaces carelessly | Runtime panic on hash | Document that type must be hashable |

### Reachability and domain bounds

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

### Security

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

### Naming conventions

```go
// Exported: PascalCase
type UserRepository interface{}
func ParseConfig() {}

// Unexported: camelCase
type httpClient struct{}

// Acronyms: all caps
type HTTPClient struct{}
var userID int

// Error vars: Err prefix
var ErrNotFound = errors.New("not found")

// Single-method interfaces: -er suffix
type Stringer interface{ String() string }

// Boolean: is/has/can prefix
func (u *User) IsAdmin() bool {}

// No stuttering: user.Name not user.UserName
```

### Formatting

```bash
gofmt -w .
goimports -w .   # also sorts imports
```

### Linting

```bash
go vet ./...         # built-in — run always
staticcheck ./...    # deeper analysis
golangci-lint run    # comprehensive suite for CI
```

A `golangci-lint` v2 configuration (the tool version is independent of the Go
language version and lints Go 1.20 code fine):

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

*End of Go 1.20 Complete Developer Guideline*
