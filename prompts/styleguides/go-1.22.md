### Go 1.22 — Complete Developer Guideline

> **Version**: Go 1.22.0 (released 2024-02-06)
> **Scope**: Full language spec, all 1.22 changes delta over 1.21, idioms, concurrency, performance, CLI, file I/O, testing, and best practices — with examples throughout.
> **Note**: This document is self-contained. All relevant 1.21/1.20/1.19 material is carried forward or superseded. Sections marked **[NEW in 1.22]** cover additions specific to this version.

---

#### Table of Contents

1. [What's New in Go 1.22](#whats-new-in-go-122)
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
18. [HTTP Servers](#http-servers)
19. [Testing & Benchmarking](#testing--benchmarking)
20. [CLI Development](#cli-development)
21. [Performance Caveats & PGO](#performance-caveats--pgo)
22. [Idioms & Things to Avoid](#idioms--things-to-avoid)

---

#### What's New in Go 1.22

Go 1.22.0 shipped 2024-02-06. It brings **two significant language changes** to `for` loops, the **first v2 standard library package** (`math/rand/v2`), enhanced HTTP routing in `net/http.ServeMux`, and a range of library and performance improvements.

##### Language changes

###### 1. Loop variables are now per-iteration [NEW in 1.22] ★★★

This is the most impactful change in 1.22 and resolves a decade-old Go gotcha.

**Before Go 1.22**: loop variables were created **once** and updated by each iteration. Closures or goroutines that captured the variable all shared the same memory location.

**Go 1.22+**: each iteration creates **new variables**. Closures and goroutines see a fresh binding per iteration.

```go
// Go 1.21 and earlier — BROKEN: all goroutines print the same value
values := []string{"a", "b", "c"}
for _, v := range values {
    go func() { fmt.Println(v) }() // all print "c"
}

// Go 1.22 — CORRECT: each goroutine captures its own v
for _, v := range values {
    go func() { fmt.Println(v) }() // prints "a", "b", "c" in some order
}

// Go 1.21 workaround — NO LONGER NEEDED in 1.22
for _, v := range values {
    v := v // shadow — obsolete in 1.22
    go func() { fmt.Println(v) }()
}
```

This applies to **all three for loop forms**:
```go
// Range-based (most common)
for i, v := range s { go func() { use(i, v) }() } // safe in 1.22

// C-style
for i := 0; i < 10; i++ { go func() { fmt.Println(i) }() } // safe in 1.22

// While-style (only applies if variables declared in for)
```

**Important nuance for C-style loops**: in 1.22, each iteration of `for i := 0; ...` gets its own `i` — initialized from the previous iteration's final value, then the post-statement (`i++`) runs on that per-iteration copy. So closures and goroutines launched in the loop body capture the value for their own iteration, not the loop's final value.

**Activation**: This change only applies to packages with `go 1.22` or later in `go.mod`. Packages with `go 1.21` or earlier retain the old semantics automatically for backward compatibility.

```
// go.mod
go 1.22.0  // opt in to per-iteration loop variables
```

```bash
# Remove obsolete i := i shadows after upgrading
# go vet no longer warns about loop variable capture in 1.22 files
go vet ./...
```

**Remove old workarounds** after upgrading to `go 1.22` in your `go.mod`:
```go
// Before: needed
for _, tt := range tests {
    tt := tt  // remove this
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        _ = tt.input // was using tt from previous iteration in 1.21
    })
}

// After: clean in 1.22
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        _ = tt.input // each tt is its own binding
    })
}
```

###### 2. Range over integers [NEW in 1.22]

You can now range over an integer to iterate from 0 to N-1:

```go
// New in 1.22 — cleaner countup/countdown
for i := range 10 {
    fmt.Println(i) // 0, 1, 2, ..., 9
}

// Previous idiom — still works but unnecessary
for i := 0; i < 10; i++ { fmt.Println(i) }

// Both loop variable forms work:
for i := range 5 { fmt.Printf("%d ", i) } // "0 1 2 3 4"
for range 3     { fmt.Println("hi") }     // "hi" three times
```

This pairs nicely with `min`/`max` (from 1.21):
```go
n := min(len(s), 10)
for i := range n { process(s[i]) }
```

###### 3. Range-over-function iterators (preview) [NEW in 1.22]

An experimental preview of ranging over functions is available with `GOEXPERIMENT=rangefunc`. This is not yet stable — it becomes stable in Go 1.23.

```go
// GOEXPERIMENT=rangefunc required in Go 1.22
// (enabled by default in Go 1.23)

// Iterator function signature:
// func(yield func(K, V) bool) — two-value yield
// func(yield func(V) bool)    — single-value yield
// func(yield func() bool)     — no-value yield

func Fibonacci() iter.Seq[int] {
    return func(yield func(int) bool) {
        a, b := 0, 1
        for {
            if !yield(a) { return }
            a, b = b, a+b
        }
    }
}

// GOEXPERIMENT=rangefunc in 1.22 only, standard in 1.23:
for n := range Fibonacci() {
    if n > 100 { break }
    fmt.Println(n)
}
```

##### Toolchain changes

###### go test -cover improvement [NEW in 1.22]

```bash
# Now reports 0.0% for packages with no test files (instead of skipping them)
go test -cover ./...
# Before 1.22: "? mymod/pkg [no test files]"
# After  1.22: "mymod/pkg coverage: 0.0% of statements"
```

###### go work vendor [NEW in 1.22]

Workspace vendor directories are now supported:
```bash
go work vendor   # creates vendor/ for the entire workspace
go build -mod=vendor ./...  # uses workspace vendor
```

##### Vet improvements [NEW in 1.22]

Three new vet checks:

```go
// 1. append with no values — almost always a mistake
s = append(s) // vet: "append with no values"

// 2. time.Since in defer — evaluated at defer call, not at function return
t := time.Now()
defer log.Println(time.Since(t))    // WRONG: measures time to defer statement
defer func() {
    log.Println(time.Since(t))      // CORRECT: measures time to function return
}()

// 3. log/slog mismatched key-value pairs
slog.Info("msg", "key")            // vet: final key "key" has no value
slog.Info("msg", 42, "value")      // vet: key 42 is not string or slog.Attr
slog.Info("msg", "key", "value")   // ok
```

##### Runtime

- GC metadata moved closer to heap objects: **1–3% CPU improvement**, ~1% less memory overhead.
- Heap allocator size class boundaries adjusted — some objects move to next size class (small potential regression in allocation size).
- Addresses previously aligned to 16 bytes may now be only 8-byte aligned.
- New stop-the-world pause metrics in `runtime/metrics`.
- Execution tracer completely overhauled — streaming, OS-clock based, goroutine thread info.

##### Standard Library — new packages

| Package | Purpose |
|---|---|
| `math/rand/v2` | First standard library v2; cleaner API, faster algorithms (ChaCha8/PCG) |
| `go/version` | Validate and compare Go version strings |

##### Standard Library — selected changes

| Package | Change |
|---|---|
| `net/http.ServeMux` | Method + wildcard routing patterns; `Request.PathValue` |
| `net/http` | `ServeFileFS`, `FileServerFS`, `NewFileTransportFS` |
| `slices` | `Concat`; shrinking functions zero freed elements; `Insert` always panics on out-of-range |
| `cmp` | `Or` — returns first non-zero value |
| `database/sql` | Generic `Null[T]` for nullable columns |
| `log/slog` | `SetLogLoggerLevel` function |
| `reflect` | `TypeFor[T]()` — cleaner way to get `reflect.Type` |
| `archive/tar`, `archive/zip` | `Writer.AddFS` — add `fs.FS` contents to archive |
| `encoding/base32`, `base64`, `hex` | `AppendEncode`, `AppendDecode` |
| `go/types` | `Alias` type for type aliases; `TypeFor[T]()` |

---

#### Project Layout & Modules

```
myapp/
├── cmd/
│   └── myapp/
│       ├── main.go
│       └── default.pgo     # PGO profile — auto-used by go build
├── internal/
├── pkg/
├── go.mod
└── go.sum
```

##### go.mod minimum version

```
module github.com/yourname/myapp

go 1.22.0

toolchain go1.22.3
```

Setting `go 1.22.0` activates per-iteration loop variables for all files in the module. If you rely on the old loop semantics anywhere, audit before bumping.

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

Lowercase, single word, no underscores. File names: snake_case.

---

#### Basic Types & Variables

##### Numeric types

| Type | Size | Notes |
|---|---|---|
| `int8`–`int64`, `uint8`–`uint64` | 1–8 bytes | Fixed-width |
| `int`, `uint` | platform | Indexing, general counts |
| `float32`, `float64` | 4, 8 bytes | IEEE 754 |
| `byte` = `uint8`, `rune` = `int32` | — | Aliases |

```go
var x int = 10
y := "hello"
z := 3.14

// Built-in min/max (1.21+)
m := min(x, 20) // 10
M := max(x, 20) // 20
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
fmt.Println(*p)  // 42
*p = 99
fmt.Println(x)   // 99

p1 := new(int)
p2 := &Point{X: 1, Y: 2}
```

##### Loop variable capture — solved in 1.22

```go
// Go 1.22 with go 1.22.0 in go.mod — no workaround needed
for i := 0; i < 3; i++ {
    go func() { fmt.Println(i) }() // safe: i is per-iteration
}

// If go.mod says go 1.21 or earlier — still need shadow:
for i := 0; i < 3; i++ {
    i := i  // required for go 1.21 semantics
    go func() { fmt.Println(i) }()
}
```

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

##### for — all forms

```go
for i := 0; i < 10; i++ {}         // C-style
for x < 100 { x *= 2 }             // while
for { if done() { break } }        // infinite

// Range over slice
for i, v := range s { }
for i := range s  { }
for _, v := range s { }

// Range over map (random order)
for k, v := range m { }

// Range over string (rune iteration)
for i, r := range "héllo" { }

// Range over channel
for v := range ch { }

// NEW in 1.22: range over integer
for i := range 10 { }   // i = 0..9
for range 5 { }          // 5 times, no index
```

##### switch

```go
switch x {
case 1:    fmt.Println("one")
case 2, 3: fmt.Println("two or three")
default:   fmt.Println("other")
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
    default:     fmt.Printf("%T\n", v)
    }
}
```

No fallthrough by default. `fallthrough` is explicit and rare.

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
for i := range 5 {       // 1.22: range over integer
    for j := range 5 {
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

##### init

```go
func init() { /* runs once before main */ }
```

---

#### Defer, Panic & Recover

##### defer — LIFO

```go
func readFile(name string) error {
    f, err := os.Open(name)
    if err != nil { return err }
    defer f.Close()
    return nil
}

// Arguments evaluated at defer call
x := 10
defer fmt.Println(x) // 10
x = 20
```

**Avoid defer in hot inner loops** — overhead per call.

##### time.Since in defer — vet warning in 1.22

```go
// WRONG — time.Since evaluated when defer is called, not when fn returns
t := time.Now()
defer log.Println(time.Since(t))  // measures ~0ns, not function duration

// CORRECT
defer func() { log.Println(time.Since(t)) }()
```

##### panic and recover

```go
// panic(nil) 1.21+ semantics: recover() is non-nil for any panic
defer func() {
    if r := recover(); r != nil { /* a panic DID occur */ }
}()

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

---

#### Structs & Methods

```go
type User struct {
    ID    int
    Name  string
    Email string
    admin bool
}

// Embedding
type Timestamps struct { CreatedAt, UpdatedAt time.Time }
type Post struct {
    ID    int
    Title string
    Timestamps
}

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

// Type assertion
var x any = "hello"
s, ok := x.(string)

// Type switch
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

type Stack[T any] struct{ items []T }
func (s *Stack[T]) Push(v T)  { s.items = append(s.items, v) }
func (s *Stack[T]) Pop() T {
    n := len(s.items)
    v := s.items[n-1]
    s.items = s.items[:n-1]
    return v
}
```

##### cmp.Or [NEW in 1.22]

```go
import "cmp"

// Or returns the first non-zero value
cmp.Or("", "", "default")        // "default"
cmp.Or(0, 0, 42)                 // 42
cmp.Or("first", "second")        // "first"

// Great for fallback chains
host := cmp.Or(os.Getenv("HOST"), cfg.Host, "localhost")
timeout := cmp.Or(flags.Timeout, cfg.Timeout, 30*time.Second)
```

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

// Share memory — dangerous!
sl := arr[1:4]
sl[0] = 99 // modifies arr

// Independent copy
c := make([]int, len(sl))
copy(c, sl)

// Preallocate
result := make([]string, 0, len(items))
for _, item := range items { result = append(result, process(item)) }
```

##### slices package (1.21+, updated in 1.22)

```go
import "slices"

slices.Sort([]int{3, 1, 4, 1, 5})
slices.SortFunc(users, func(a, b User) int { return cmp.Compare(a.Name, b.Name) })
slices.SortStableFunc(items, cmpFn)

idx, found := slices.BinarySearch(sorted, target)
slices.Contains(s, "hello")
slices.Index(s, "hello")

slices.Reverse(s)
slices.Compact(s)
slices.Clip(s)
slices.Grow(s, 10)
slices.Insert(s, idx, values...) // panics on out-of-range (changed in 1.22)
slices.Delete(s, i, j)           // now zeroes freed elements (changed in 1.22)
slices.DeleteFunc(s, fn)         // now zeroes freed elements (changed in 1.22)
slices.Replace(s, i, j, vals...) // now zeroes freed elements (changed in 1.22)

slices.Max(s); slices.Min(s)
slices.MaxFunc(s, cmpFn); slices.MinFunc(s, cmpFn)

// NEW in 1.22: Concat
import "slices"
s1 := []string{"one", "two"}
s2 := []string{"three"}
s3 := []string{"four"}
result := slices.Concat(s1, s2, s3)
// ["one","two","three","four"] — pre-allocates exact capacity
```

**Important 1.22 change**: `slices.Delete`, `slices.DeleteFunc`, `slices.Compact`, `slices.CompactFunc`, and `slices.Replace` now **zero the elements between new length and old length**. This prevents memory leaks when storing pointers:

```go
type Node struct{ data *BigData }
nodes := []*Node{a, b, c, d}
nodes = slices.Delete(nodes, 1, 3) // b and c are now nil — GC can collect them
// Before 1.22: b and c remained in backing array, preventing GC
```

###### Modifying during iteration

```go
// Safe: modify in place
for i := range s { s[i] *= 2 }

// Filter in place — idiomatic (1.21+)
s = slices.DeleteFunc(s, func(v int) bool { return v < 0 })

// Manual filter
n := 0
for _, v := range s {
    if keep(v) { s[n] = v; n++ }
}
s = s[:n]
```

##### Maps

```go
m := make(map[string]int)
v, ok := m["key"] // always use two-value form
delete(m, "key")
clear(m)          // 1.21+: removes all entries
```

##### maps package (1.21+)

```go
import "maps"
m2 := maps.Clone(m)
maps.Copy(dst, src)
maps.DeleteFunc(m, func(k string, v int) bool { return v < 0 })
maps.Equal(m1, m2)
maps.EqualFunc(m1, m2, fn)
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

#### Strings

```go
s := "héllo"
fmt.Println(len(s))         // 6 bytes
fmt.Println(len([]rune(s))) // 5 runes
for i, r := range s { fmt.Printf("%d: %c\n", i, r) }

var b strings.Builder
b.Grow(1000)
for range 1000 { b.WriteByte('x') } // 1.22: range over integer
result := b.String()

strings.Contains("foobar", "bar")
strings.HasPrefix("foobar", "foo")
strings.Split("a,b,c", ",")
strings.Join([]string{"a","b"}, "-")
strings.TrimSpace("  hello  ")
strings.ToUpper("hello")
strings.ReplaceAll("aabbcc", "b", "x")
after, found := strings.CutPrefix("Gopher", "Go")   // "pher", true (1.20+)
before, found := strings.CutSuffix("Gopher", "er")  // "Goph", true (1.20+)
```

---

#### Error Handling

```go
type error interface { Error() string }

result, err := doSomething()
if err != nil { return fmt.Errorf("context: %w", err) }

var ErrNotFound = errors.New("not found")
if errors.Is(err, ErrNotFound) { /* handle */ }

// Custom type
type ValidationError struct { Field, Message string }
func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation: %s: %s", e.Field, e.Message)
}
var ve *ValidationError
if errors.As(err, &ve) { fmt.Println(ve.Field) }

// Multi-error (1.20+)
combined := errors.Join(err1, err2)
wrapped  := fmt.Errorf("two: %w and %w", err1, err2)

// errors.ErrUnsupported (1.21+)
if errors.Is(err, errors.ErrUnsupported) { /* fallback */ }
```

**Always check errors.** A few discards are idiomatic and should not be flagged:

- deferred `Close()` on files opened only for reading
- `defer tx.Rollback()`, which is a no-op after a successful commit
- writes to `bytes.Buffer` or `strings.Builder`, which never return an error

For everything else, handle the error or make the discard explicit with `_ =`.

---

#### math/rand/v2 [NEW in 1.22]

The first standard library v2 package. Use it for all new code.

```go
import "math/rand/v2"

// Global RNG (auto-seeded, ChaCha8 algorithm):
n := rand.IntN(100)       // [0, 100)
f := rand.Float64()       // [0.0, 1.0)
b := rand.Int32()         // random int32
u := rand.Uint64()        // random uint64

// NEW generic function N — works for any integer type or duration
d := rand.N(5 * time.Minute) // random duration [0, 5 min)
i := rand.N(10)               // random int [0, 10)
u8 := rand.N(uint8(255))      // random uint8 [0, 255)

// Reproducible sequences — create your own RNG
rng := rand.New(rand.NewPCG(seed1, seed2)) // PCG algorithm
// or
rng2 := rand.New(rand.NewChaCha8([32]byte{...})) // ChaCha8 algorithm

n2 := rng.IntN(100)
rng.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
```

##### v1 vs v2 API differences

| math/rand (v1) | math/rand/v2 | Notes |
|---|---|---|
| `rand.Intn(n)` | `rand.IntN(n)` | Idiomatic Go naming |
| `rand.Int31()` | `rand.Int32()` | |
| `rand.Int31n(n)` | `rand.Int32N(n)` | |
| `rand.Int63()` | `rand.Int64()` | |
| `rand.Int64n(n)` | `rand.Int64N(n)` | |
| — | `rand.N(n)` | Generic, works on any integer type or Duration |
| `rand.Seed(n)` (deprecated) | Not present | Global always auto-seeded |
| `rand.Read(b)` (deprecated) | Not present | Use `crypto/rand.Read` |
| `rand.NewSource(seed)` | `rand.NewPCG(s1, s2)` or `rand.NewChaCha8(seed)` | |
| LFSR source | ChaCha8 / PCG | Faster, better distribution |

##### When to use which

```go
// Non-cryptographic randomness (games, simulations, tests)
rand.IntN(100)           // math/rand/v2
rand.N(5 * time.Minute)  // math/rand/v2 generic

// Cryptographic randomness (tokens, keys, nonces)
import "crypto/rand"
crypto/rand.Read(buf)
```

---

#### Structured Logging with log/slog

##### Basics (from 1.21)

```go
import "log/slog"

slog.Info("user logged in", "userID", 42, "ip", "1.2.3.4")
slog.Warn("high latency", "ms", 312)
slog.Error("connection failed", "err", err)

// JSON handler for log aggregators
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
    AddSource: true,
}))
slog.SetDefault(logger)
```

##### SetLogLoggerLevel [NEW in 1.22]

```go
// Control the minimum level for calls through the old log package
// AND for top-level slog functions
slog.SetLogLoggerLevel(slog.LevelDebug)

// Now both of these respect the level:
slog.Debug("message")           // was filtered before, now shown
log.Println("legacy log msg")   // also filtered by slog level
```

##### Key practices

```go
// Structured key-value — never format in message
slog.Info("connected", "host", host, "port", port)

// Type-safe attributes
slog.Info("request",
    slog.String("method", "GET"),
    slog.Int("status", 200),
    slog.Duration("latency", 12*time.Millisecond),
)

// Child logger with common fields
reqLogger := logger.With("requestID", reqID)
reqLogger.Info("handler started")

// Groups
dbLogger := logger.WithGroup("db").With("host", "localhost")
dbLogger.Error("query failed", "err", err)

// Context-aware
slog.InfoContext(ctx, "processing", "id", id)
```

##### slog best practices

- **Always use structured key-value pairs** — never format strings into the message.
- **Keys should be lowercase snake_case** for consistent log querying.
- **Pass `context.Context` through** and use `InfoContext`/`ErrorContext` for trace IDs.
- **Group related attributes** with `slog.Group` or `logger.WithGroup`.
- **Never log secrets, tokens, credentials, or full request/response bodies** unless they are explicitly scrubbed.

##### Vet check for slog calls (1.22)

```go
// go vet now catches these:
slog.Info("msg", "key")            // missing value for key
slog.Info("msg", 42, "value")      // key is not string or slog.Attr
slog.With("a", "b", "c")           // odd number of args
```

---

#### Concurrency

##### Goroutines — 1.22 loop fix

```go
// Safe in 1.22 (go 1.22.0 in go.mod):
for i := range 5 { go func() { fmt.Println(i) }() } // prints 0-4
for _, v := range items { go func() { process(v) }() }

// Still need to shadow when NOT using 1.22 semantics (older go.mod):
for i := 0; i < 5; i++ { i := i; go func() { fmt.Println(i) }() }
```

##### Channels

```go
ch := make(chan int)       // unbuffered
bch := make(chan string, 10) // buffered

go func() { ch <- 42 }()
val := <-ch

close(ch)
for v := range ch { fmt.Println(v) }
v, ok := <-ch; if !ok { /* closed */ }
```

**Rules**: sender closes; sending to closed panics; receiving from closed returns zero value.

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
    s.mu.RLock(); defer s.mu.RUnlock()
    return s.m[k]
}
func (s *SafeMap) Set(k string, v int) {
    s.mu.Lock(); defer s.mu.Unlock()
    s.m[k] = v
}
```

**Never copy a mutex.**

##### sync.Once and helpers (1.21+)

```go
var once sync.Once
once.Do(func() { db = openDB() })

initDB := sync.OnceFunc(func() { db = openDB() })
initDB()

getConfig := sync.OnceValue(func() *Config { return loadConfig() })
cfg := getConfig()

getConn := sync.OnceValues(func() (*sql.DB, error) { return sql.Open("postgres", dsn) })
db, err := getConn()
```

##### sync.Map with 1.20+ atomic methods

```go
var m sync.Map
m.Store("k", "v")
v, ok := m.Load("k")
m.Delete("k")
old, loaded := m.Swap("k", "new")
ok = m.CompareAndSwap("k", "old", "new")
ok = m.CompareAndDelete("k", "value")
m.Range(func(k, v any) bool { return true })
```

##### Atomic types

```go
var counter atomic.Int64
counter.Add(1)
fmt.Println(counter.Load())

var ptr atomic.Pointer[Config]
ptr.Store(newConfig)
cfg := ptr.Load()
```

##### Worker pool

```go
func workerPool(jobs <-chan Job, results chan<- Result, n int) {
    var wg sync.WaitGroup
    for range n { // 1.22: range over integer
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs { results <- process(job) }
        }()
    }
    go func() { wg.Wait(); close(results) }()
}
```

##### Mutex profile improvement (1.22)

In 1.22, mutex profiles scale contention **by goroutine count**. If 100 goroutines wait 10ms on a mutex, the profile now records 1 second of delay (not 10ms). This gives a more accurate picture of mutex bottlenecks.

##### Concurrency pitfalls

| Pitfall | Fix |
|---|---|
| Goroutine leak | Context cancellation; always provide exit |
| Race condition | `go test -race ./...` |
| Deadlock | No circular waits |
| Closing closed channel | Sender closes; `sync.Once` if ambiguous |
| Loop variable capture (< 1.22) | Shadow `i := i` or pass as arg |
| Copying mutex | Embed + pointer receiver |

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

// 1.20+: cancel with cause
ctx, cancel = context.WithCancelCause(parent); defer cancel(nil)
cancel(ErrRateLimited)
cause := context.Cause(ctx)

// 1.21+: detach from parent cancellation
detached := context.WithoutCancel(requestCtx)

// 1.21+: cancel cause on timeout
ctx, cancel = context.WithTimeoutCause(parent, 5*time.Second, errors.New("rpc timeout"))

// 1.21+: run cleanup goroutine on ctx done
stop := context.AfterFunc(ctx, cleanupResources)
```

**Never store context in a struct. Never pass nil. Always `defer cancel()`.**

---

#### File I/O & Streaming

```go
f, err := os.Open("file.txt")
if err != nil { return err }
defer f.Close()

w := bufio.NewWriter(f)
defer w.Flush()
fmt.Fprintln(w, "buffered output")

scanner := bufio.NewScanner(f)
scanner.Buffer(make([]byte, 1<<20), 1<<20) // large lines
for scanner.Scan() { process(scanner.Text()) }
if err := scanner.Err(); err != nil { return err }

data, err := os.ReadFile("small.json")

io.Copy(dst, src)
limited := io.LimitReader(src, 1<<20)
tee := io.TeeReader(src, &buf)
ow := io.NewOffsetWriter(file, 512) // 1.20+

// Walk
filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
    if err != nil { return err }
    if shouldStop(path) { return filepath.SkipAll } // 1.20+
    return nil
})
```

##### fs.FS additions [NEW in 1.22]

```go
// archive/tar and archive/zip: Writer.AddFS
tw := tar.NewWriter(f)
if err := tw.AddFS(os.DirFS("./dist")); err != nil { return err }

// net/http: serve from fs.FS
http.Handle("/static/", http.FileServerFS(os.DirFS("./static")))
http.ServeFileFS(w, r, os.DirFS("./public"), "index.html")
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

##### Enhanced ServeMux routing [NEW in 1.22] ★★★

The standard `net/http.ServeMux` now supports method prefixes and path wildcards, reducing the need for third-party routers.

```go
mux := http.NewServeMux()

// Method + path — only matches GET requests
mux.HandleFunc("GET /users", listUsersHandler)
mux.HandleFunc("POST /users", createUserHandler)

// Path wildcard — {id} captures a single segment
mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    fmt.Fprintf(w, "user: %s\n", id)
})

// Multi-segment wildcard — {path...} must be last
mux.HandleFunc("GET /files/{path...}", func(w http.ResponseWriter, r *http.Request) {
    path := r.PathValue("path") // e.g., "subdir/file.txt"
    http.ServeFile(w, r, filepath.Join("./static", path))
})

// Exact match with trailing slash — {$} anchors to exact path
mux.HandleFunc("GET /health/{$}", healthHandler) // matches /health/ only, not /health/anything

// Pattern without method — matches all methods
mux.HandleFunc("/metrics", metricsHandler)
```

###### Routing rules

- **Method + path** takes precedence over path-only.
- **More specific patterns** take precedence over less specific ones regardless of registration order.
- **Conflicts** (two patterns that match the same request without one being more specific) cause a **panic at registration time** — detected early, not at request time.
- `GET` pattern also matches `HEAD` automatically.

```go
// This panics: both match GET /users/123, neither is more specific
mux.HandleFunc("GET /users/{id}", h1)
mux.HandleFunc("GET /{resource}/{id}", h2) // PANIC at startup
```

###### Backwards compatibility

If you use `{` or `}` in existing patterns, or rely on old escape behaviour:
```bash
GODEBUG=httpmuxgo121=1 ./myapp  # restore old routing behaviour
```

###### SetPathValue — testing and middleware

```go
// In tests or middleware: manually set captured values
r = r.WithContext(r.Context())
r.SetPathValue("id", "42") // 1.22+
```

##### ResponseController (1.20+)

```go
func streamHandler(w http.ResponseWriter, r *http.Request) {
    rc := http.NewResponseController(w)
    rc.SetWriteDeadline(time.Now().Add(60 * time.Second))
    for _, chunk := range chunks {
        w.Write(chunk)
        rc.Flush()
    }
}
```

##### Middleware pattern

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        slog.Info("request",
            slog.String("method", r.Method),
            slog.String("path", r.URL.Path),
            slog.Duration("latency", time.Since(start)),
        )
    })
}

mux := http.NewServeMux()
mux.HandleFunc("GET /api/users/{id}", getUserHandler)
http.ListenAndServe(":8080", LoggingMiddleware(mux))
```

---

#### database/sql — Null[T] [NEW in 1.22]

```go
import "database/sql"

// Before 1.22: separate NullString, NullInt64, NullBool, etc.
var name sql.NullString
row.Scan(&name)

// After 1.22: generic Null[T] for any type
var name sql.Null[string]
var count sql.Null[int64]
var active sql.Null[bool]
var score sql.Null[float64]

err := row.Scan(&name, &count, &active, &score)
if name.Valid {
    fmt.Println("Name:", name.V)
}

// Custom nullable type
var createdAt sql.Null[time.Time]
row.Scan(&createdAt)
if createdAt.Valid {
    fmt.Println(createdAt.V.Format(time.DateTime))
}
```

---

#### reflect.TypeFor [NEW in 1.22]

```go
import "reflect"

// Before 1.22: awkward
t := reflect.TypeOf((*http.Request)(nil)).Elem()

// After 1.22: clean
t := reflect.TypeFor[http.Request]()
t2 := reflect.TypeFor[[]byte]()
t3 := reflect.TypeFor[map[string]int]()
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

##### Table-driven (1.22: no shadow needed)

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
        // In 1.22: no need for tt := tt before t.Parallel()
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
available in Go 1.22: `t.TempDir()` (1.15), `t.Setenv()` (1.17), and
`t.Cleanup()` (1.14).

Note: `t.Context()` and `t.Chdir()` are Go 1.24 additions and are not available
in Go 1.22.

##### go test -cover improvement (1.22)

```bash
go test -cover ./...
# Now shows: "mymod/pkg coverage: 0.0% of statements" for untested packages
# Previously: "? mymod/pkg [no test files]" — silently omitted
```

##### Benchmarks

```go
func BenchmarkProcess(b *testing.B) {
    for range b.N { // 1.22: range over integer
        Process(data)
    }
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

##### Vet checks to know (cumulative)

```go
// 1.20: loop variable capture after t.Parallel (removed in 1.22 when go 1.22 in go.mod)
// 1.20: wrong time format (2006-02-01 vs 2006-01-02)
// 1.22: append with no values
// 1.22: time.Since in defer
// 1.22: slog mismatched key/value
go vet ./...
```

---

#### CLI Development

##### Standard flag

```go
host    := flag.String("host", "localhost", "server host")
port    := flag.Int("port", 8080, "server port")
verbose := flag.Bool("verbose", false, "verbose")
flag.BoolFunc("json", "JSON output", func(string) error { format = "json"; return nil }) // 1.21+

flag.Parse()
```

##### Cobra (production CLIs)

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

#### go/version package [NEW in 1.22]

```go
import "go/version"

// Validate
version.IsValid("go1.22.0")  // true
version.IsValid("1.22.0")    // false — must start with "go"

// Compare
version.Compare("go1.22.0", "go1.21.0") // 1 (1.22 > 1.21)
version.Compare("go1.21.0", "go1.22.0") // -1
version.Compare("go1.22.0", "go1.22.0") // 0

// Get the language version from a go.mod module version
version.Lang("go1.22.3") // "go1.22"
version.Lang("go1.22")   // "go1.22"
```

Useful for: build tools, CI scripts, code that needs to adapt based on the Go version in `go.mod`.

---

#### Performance Caveats & PGO

##### PGO (GA since 1.21, improved in 1.22)

PGO is auto-enabled when `default.pgo` exists in the main package directory.

```bash
# Collect profile
go build -o myapp ./cmd/myapp
./myapp &
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cmd/myapp/default.pgo
kill %1

# Build (auto-PGO)
go build ./cmd/myapp   # uses default.pgo automatically

# Merge profiles from multiple instances
go tool pprof -proto a.pprof b.pprof > merged.pprof
mv merged.pprof cmd/myapp/default.pgo
```

In 1.22, PGO can devirtualise a **higher proportion** of interface method calls. Combined with interleaved devirtualisation and inlining, most programs see **2–14% improvement** with PGO.

##### GC metadata improvement (1.22)

Metadata is now stored closer to heap objects:
- **1–3% CPU improvement** in most programs.
- ~1% memory reduction.
- Side effect: some heap addresses are now 8-byte aligned instead of 16-byte. If you have assembly code that assumes 16-byte heap alignment, audit it.

##### slices package performance

`slices.Sort` is faster than `sort.Slice` in almost all cases:
- No `interface{}` boxing for comparison.
- `slices.Concat` pre-allocates exact capacity (no reallocation).

##### slices shrinking functions zero freed elements (1.22)

After `slices.Delete`, `slices.Compact`, etc., freed slots are now nil/zero. This is slightly slower but prevents memory leaks with pointer slices:

```go
// Critical for pointer slices — prevents GC from being blocked
records = slices.DeleteFunc(records, func(r *Record) bool { return r.stale })
// Freed slots now nil — GC can collect *Record objects
```

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

##### Escape analysis

```bash
go build -gcflags="-m" ./...
```

##### Struct layout

```go
type Bad    struct { A bool; B int64; C bool } // 24 bytes
type Better struct { B int64; A bool; C bool } // 16 bytes
```

##### math/rand/v2 performance

ChaCha8 and PCG (v2) are faster than the LFSR used in math/rand (v1) with better statistical properties. Prefer v2 for all new code.

##### Profiling

```bash
go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof
go tool pprof cpu.prof

import _ "net/http/pprof"
go http.ListenAndServe(":6060", nil)
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

---

#### Idioms & Things to Avoid

##### Do (1.22 additions)

- **Remove `i := i` shadow patterns** after bumping `go.mod` to 1.22 — they're dead code.
- **Use `for i := range N`** instead of `for i := 0; i < N; i++` for simple countups.
- **Use `slices.Concat`** instead of multiple appends — pre-allocates exactly.
- **Use `math/rand/v2`** (`rand.N`, `rand.IntN`, etc.) for all new random number generation.
- **Use `cmp.Or`** for first-non-zero fallback chains.
- **Use `reflect.TypeFor[T]()`** instead of `reflect.TypeOf((*T)(nil)).Elem()`.
- **Use `database/sql.Null[T]`** for nullable columns of any type.
- **Use enhanced ServeMux routing** with method + wildcard patterns in `net/http`.
- **Wrap deferred `time.Since`** in a closure — `defer func() { log.Println(time.Since(t)) }()`.
- **Use `go vet`** — it now catches slog mismatches, append no-ops, and deferred `time.Since`.
- **Use `go/version`** for build tools that parse or compare Go versions.

##### Don't

| Anti-pattern | Why | Instead |
|---|---|---|
| `i := i` shadow in 1.22 | Unnecessary; loop vars are per-iteration | Remove the shadow |
| `for i := 0; i < N; i++` for simple countup | Verbose | `for i := range N` |
| `math/rand` (v1) in new code | Deprecated; slower; no generic N | `math/rand/v2` |
| `rand.Seed()` | Deprecated since 1.20 | `rand.New(rand.NewPCG(s1, s2))` |
| `var m map[string]int` | Nil, writes panic | `make(map[string]int)` |
| `clear(s)` to empty a slice | Only zeroes elements, keeps length | `s = s[:0]` |
| Mixing value and pointer receivers | Confusing method sets | Pick one |
| Log AND return error | Double-logging | Return; log at boundary |
| `panic` for business logic | Crashes program | `return error` |
| Copying a mutex | Broken | Embed + pointer receiver |
| Closing channel from receiver | Panic | Sender closes |
| String `+` in loops | O(n^2) | `strings.Builder` |
| Storing context in struct | Anti-pattern | First function parameter |
| Manual `reflect.TypeOf((*T)(nil)).Elem()` | Verbose | `reflect.TypeFor[T]()` |
| `defer log.Println(time.Since(t))` | Evaluates at defer, not return | Wrap in closure |
| `append(s)` with no values | No-op, almost always a bug (vet catches) | Remove the call |
| Assuming 16-byte heap alignment in assembly | 1.22 aligns to 8 bytes | Fix alignment assumptions |

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
type UserRepository interface{}; func ParseConfig() {}

// Unexported: camelCase
type httpClient struct{}

// Acronyms: all caps
type HTTPClient struct{}; var userID int

// Errors: Err prefix
var ErrNotFound = errors.New("not found")

// Interface: -er suffix
type Stringer interface{ String() string }

// No stuttering: user.Name not user.UserName
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
language version and lints Go 1.22 code fine):

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

*End of Go 1.22 Complete Developer Guideline*
