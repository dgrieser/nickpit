### Go — Common Developer Guideline

> **Version**: Common ground for Go 1.19 – 1.26
> **Scope**: Language spec, idioms, concurrency, performance, CLI, file I/O, testing, and best practices — with examples throughout.
> **Note**: This is the base/fallback guide covering what holds across Go 1.19 – 1.26. Features introduced after Go 1.19 are tagged with the version that added them and are available only when the module's `go` directive is at least that version. For version-specific detail, consult the matching guide (`go-1.19.md` … `go-1.26.md`).

---

#### Table of Contents

1. [Version-Aware Go](#version-aware-go)
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
20. [Performance Caveats](#performance-caveats)
21. [Idioms & Things to Avoid](#idioms--things-to-avoid)

---

#### Version-Aware Go

Check the module's `go` directive before reviewing language semantics or
suggesting newer APIs. The `go` directive controls language version behavior;
the installed toolchain may be newer than the module target. A feature is
available only when the `go` directive is at least the version that introduced
it. Consult the matching version guide (`go-1.19.md` … `go-1.26.md`) for detail.

```go
module example.com/app

go 1.21.0
```

##### Feature timeline (1.19 – 1.26)

| Since | Key language / standard-library additions |
|---|---|
| 1.18/1.19 | Generics; type-safe `sync/atomic` types (`atomic.Int64`, `atomic.Pointer[T]`); `GOMEMLIMIT` soft memory limit |
| 1.20 | `errors.Join` and multiple `%w` in `fmt.Errorf`; `context.WithCancelCause`/`Cause`; slice-to-array value conversion |
| 1.21 | `slices`, `maps`, `cmp` packages; `clear`/`min`/`max` builtins; `log/slog`; `context.WithoutCancel`/`AfterFunc`; `sync.OnceFunc`/`OnceValue`/`OnceValues` |
| 1.22 | Per-iteration loop variables; range over integers; enhanced `net/http.ServeMux` routing; `math/rand/v2` |
| 1.23 | Range-over-function iterators + `iter`; `slices`/`maps` iterator functions; `unique`; `sync.Map.Clear`; `atomic.And`/`Or`; `os.CopyFS` |
| 1.24 | Generic type aliases; `go.mod` `tool` directive; `os.Root`; `testing.B.Loop`, `T.Context`, `T.Chdir`; `omitzero` JSON tag; `runtime.AddCleanup`; `weak` |
| 1.25 | `sync.WaitGroup.Go`; `testing/synctest` (stable); container-aware `GOMAXPROCS`; `T.Attr`/`Output`; expanded `os.Root` |
| 1.26 | `new(expr)`; recursive type constraints; `errors.AsType`; `crypto/hpke`; Green Tea GC default; `go fix` modernizers |

Verify library APIs against the actual module versions in `go.mod` before
claiming an API is missing or unavailable.

Examples:
- Do not claim a controller-runtime helper is unavailable without checking the
  pinned `sigs.k8s.io/controller-runtime` version.
- Say "this API is not available in vX.Y.Z" only when the module version proves
  it.
- Do not infer API availability from memory, a newer version's docs, or another
  project's dependency set.

---

#### Project Layout & Modules

##### Module initialisation

```bash
mkdir myapp && cd myapp
go mod init github.com/yourname/myapp
```

This creates `go.mod`. Never edit it manually for dependencies; use `go get`, `go mod tidy`.

For Go 1.24 and newer modules, use `tool` directives in `go.mod` to track
executable development tools instead of blank-import `tools.go` files.

##### Recommended directory layout

```
myapp/
├── cmd/
│   └── myapp/
│       └── main.go        # entry point(s)
├── internal/
│   ├── server/
│   └── storage/
├── pkg/
│   └── util/
├── api/                   # proto/OpenAPI specs
├── go.mod
└── go.sum
```

- `cmd/` — each subdirectory is a separate binary (`main` package).
- `internal/` — importable only within this module; enforced by the compiler.
- `pkg/` — public packages intended for external use.
- Avoid a flat layout for anything beyond toy programs.

##### Package naming rules

- Lowercase, single word, no underscores: `storage`, `httputil`, `parser`.
- The package name is the last element of the import path. `github.com/foo/bar/httpclient` -> package `httpclient`.
- File names are snake_case: `user_service.go`, `user_service_test.go`.

---

#### Basic Types & Variables

##### Numeric types

| Type | Size | Notes |
|---|---|---|
| `int8`, `int16`, `int32`, `int64` | 1–8 bytes | Signed |
| `uint8`, `uint16`, `uint32`, `uint64` | 1–8 bytes | Unsigned |
| `int`, `uint` | platform (32 or 64 bit) | Use for general indexing |
| `uintptr` | platform | Holds a pointer value as an integer |
| `float32`, `float64` | 4, 8 bytes | IEEE 754 |
| `complex64`, `complex128` | — | Rarely needed |
| `byte` = `uint8`, `rune` = `int32` | — | Aliases |

**Prefer `int` for loop counters and sizes** unless you have a specific reason to constrain range.

##### Declaration forms

```go
// var with explicit type
var x int = 10

// var with inferred type
var y = "hello"

// Short declaration (only inside functions)
z := 3.14

// Multiple assignment
a, b := 1, 2

// Blank identifier — discard a value
_, err := os.Open("file")

// Constants
const MaxSize = 1024
const Pi = 3.14159

// Typed constants
const StatusOK = 200
```

##### Zero values

Every type has a zero value. Declared but not assigned variables are not garbage:

| Type | Zero value |
|---|---|
| `int`, `float64` | `0` |
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

// Bit flags
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

#### Pointers

##### Basics

```go
x := 42
p := &x       // p is *int, holds address of x
fmt.Println(*p) // dereference -> 42
*p = 99
fmt.Println(x)  // 99 — x was modified through p
```

##### new vs &T{}

```go
// new allocates zeroed memory, returns pointer
p1 := new(int)        // *int pointing to 0

// composite literal address — preferred for structs
type Point struct{ X, Y int }
p2 := &Point{X: 1, Y: 2} // *Point
```

**Prefer `&T{...}` over `new(T)` for structs** — more readable and allows field initialization.

Go 1.26+ allows `new(expr)` to allocate and initialise from any expression
(e.g. `new(30)` returns a `*int` pointing to `30`); useful for optional
pointer fields. Not available before 1.26.

##### When to use pointers

- **Mutation**: a function that must modify its argument needs a pointer.
- **Large structs**: avoid copying a 1KB struct on every call — pass `*T`.
- **Optional values**: `*T` can be `nil` to represent absence (though prefer a sentinel or `bool` return when possible).
- **Interfaces**: if any method has a pointer receiver, the concrete type must be passed as a pointer to satisfy the interface.

##### Loop variable capture

In modules on **Go 1.21 or earlier** you MUST shadow the loop variable before
capturing it in a closure or goroutine; otherwise all iterations share one
variable. **Go 1.22+** makes loop variables per-iteration automatically, so the
copy is unnecessary (and `go vet` no longer flags the old pattern there).

```go
// Go 1.21 or earlier — WRONG: all goroutines share the same i
ptrs := make([]*int, 3)
for i := 0; i < 3; i++ {
    ptrs[i] = &i  // all point to the same address
}

// Go 1.21 or earlier — CORRECT: shadow i to create a new binding each iteration
for i := 0; i < 3; i++ {
    i := i
    ptrs[i] = &i
}

// Go 1.22+ — safe without the copy; each iteration has its own i
for i := 0; i < 3; i++ {
    ptrs[i] = &i
}
```

Do not add manual loop-variable copies in modules that require Go 1.22 or newer
unless the copy has another purpose.

---

#### Control Flow

##### if / else

```go
// Basic
if x > 0 {
    fmt.Println("positive")
} else if x < 0 {
    fmt.Println("negative")
} else {
    fmt.Println("zero")
}

// Init statement — scope of err is limited to the if block
if err := doSomething(); err != nil {
    return err
}

// Idiomatic: no else after return/continue/break
f, err := os.Open(name)
if err != nil {
    return err
}
// continue here — no else needed
use(f)
```

**Do NOT write `else` after a `return`** — it is unnecessary and considered non-idiomatic in Go.

##### for — the only loop

Go has exactly **one loop keyword**: `for`. It covers all classic loop shapes.

```go
// C-style
for i := 0; i < 10; i++ {
    fmt.Println(i)
}

// While-style (condition only)
for x < 100 {
    x *= 2
}

// Infinite loop
for {
    if done() {
        break
    }
}

// Range over slice (index, value)
s := []string{"a", "b", "c"}
for i, v := range s {
    fmt.Println(i, v)
}

// Range — value only (blank identifier)
for _, v := range s {
    fmt.Println(v)
}

// Range over map (order is random every run)
m := map[string]int{"x": 1, "y": 2}
for k, v := range m {
    fmt.Println(k, v)
}

// Range over string iterates runes (Unicode code points), not bytes
for i, r := range "héllo" {
    fmt.Printf("%d: %c\n", i, r)
}

// Range over channel — reads until channel is closed
ch := make(chan int, 3)
ch <- 1; ch <- 2; ch <- 3
close(ch)
for v := range ch {
    fmt.Println(v)
}
```

Go 1.22+ also allows ranging over an integer (`for i := range 10 { ... }`), and
Go 1.23+ allows ranging over iterator functions (`for v := range seq { ... }`).
Neither form is available in earlier modules — see the version guides.

##### break / continue / labels

```go
outer:
for i := 0; i < 5; i++ {
    for j := 0; j < 5; j++ {
        if j == 2 {
            continue outer // skip to next i
        }
        if i == 3 {
            break outer    // exit both loops
        }
    }
}
```

Labels are uncommon; use them only when the logic genuinely needs them.

##### switch

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

// No condition (replaces if-else chains)
switch {
case x < 0:
    fmt.Println("negative")
case x == 0:
    fmt.Println("zero")
default:
    fmt.Println("positive")
}

// Type switch — used with interfaces
func describe(i any) {
    switch v := i.(type) {
    case int:
        fmt.Printf("int: %d\n", v)
    case string:
        fmt.Printf("string: %s\n", v)
    default:
        fmt.Printf("unknown: %T\n", v)
    }
}
```

Go's `switch` does **not** fall through by default — no `break` needed. Use `fallthrough` only when genuinely necessary.

##### select — channels only

```go
select {
case msg := <-ch1:
    fmt.Println("received", msg)
case ch2 <- "hello":
    fmt.Println("sent")
case <-time.After(1 * time.Second):
    fmt.Println("timeout")
default:
    fmt.Println("no channel ready")
}
```

`select` with `default` is non-blocking. Without `default` it blocks until one case is ready.

---

#### Functions

##### Basics

```go
func add(a, b int) int {
    return a + b
}

// Multiple return values — idiomatic
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}

// Named return values (use sparingly — mostly useful with defer)
func minMax(a, b int) (min, max int) {
    if a < b {
        return a, b
    }
    min, max = b, a
    return // naked return — only for very short functions
}
```

**Avoid naked returns in long functions.** They hurt readability.

##### Variadic functions

```go
func sum(nums ...int) int {
    total := 0
    for _, n := range nums {
        total += n
    }
    return total
}

sum(1, 2, 3)
s := []int{1, 2, 3}
sum(s...)  // spread a slice
```

##### First-class functions and closures

```go
func counter() func() int {
    n := 0
    return func() int {
        n++
        return n
    }
}

c := counter()
fmt.Println(c(), c(), c()) // 1 2 3
```

**Watch out**: closures capture variables by reference. In modules on Go 1.21 or
earlier, shadow the loop variable before capturing it (Go 1.22+ does this
automatically):

```go
for i := 0; i < 3; i++ {
    i := i  // new variable each iteration (unnecessary on Go 1.22+)
    go func() { fmt.Println(i) }()
}
```

##### init functions

```go
func init() {
    // runs automatically before main, once per package
    // use for package-level setup (registering drivers, etc.)
}
```

Multiple `init` functions are allowed in the same file; they run in source order. Keep them simple and side-effect-free where possible.

---

#### Defer, Panic & Recover

##### defer

`defer` schedules a function call to run when the surrounding function returns, regardless of whether it returns normally or via panic. Multiple defers run in **LIFO** (last-in, first-out) order.

```go
func readFile(name string) error {
    f, err := os.Open(name)
    if err != nil {
        return err
    }
    defer f.Close() // guaranteed to run

    // process f...
    return nil
}
```

**Defer arguments are evaluated immediately**, not when the defer executes:

```go
x := 10
defer fmt.Println(x) // prints 10, not whatever x is at return time
x = 20
```

**Defer with named returns** — can modify the return value:

```go
func double(x int) (result int) {
    defer func() { result *= 2 }()
    result = x
    return
}
// double(5) == 10
```

**Avoid `defer` in tight loops** — each deferred call has overhead. Use it for cleanup, not inside hot paths.

##### panic

`panic` stops normal execution. The runtime unwinds the stack running deferred functions, then the program crashes with a stack trace.

```go
func mustPositive(x int) int {
    if x <= 0 {
        panic(fmt.Sprintf("expected positive, got %d", x))
    }
    return x
}
```

**When to panic**: truly unrecoverable programmer errors (invariant violation during init, nil pointer you cannot recover from). **Do not use panic for normal error flow.**

##### recover

`recover` stops a panic and returns the panic value. It only works inside a **deferred** function.

```go
func safeDiv(a, b int) (result int, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("recovered panic: %v", r)
        }
    }()
    return a / b, nil
}
```

**Practical rule**: only recover at package or API boundaries to convert panics to errors. Never silently swallow panics.

---

#### Structs & Methods

##### Struct definition

```go
type User struct {
    ID       int
    Name     string
    Email    string
    admin    bool   // unexported field (lowercase)
}

// Named fields — always use this form
u1 := User{ID: 1, Name: "Alice", Email: "alice@example.com"}
// Positional — fragile, avoid
u2 := User{1, "Bob", "bob@example.com", false}
```

Always use named fields when constructing structs. Positional breaks silently when fields are added.

##### Embedded structs (composition)

```go
type Timestamps struct {
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Post struct {
    ID    int
    Title string
    Timestamps            // embedded — fields promoted
}

p := Post{ID: 1, Title: "Hello"}
p.CreatedAt = time.Now() // promoted field access
```

Embedding is Go's alternative to inheritance. It is **composition**, not subclassing.

##### Methods

```go
// Value receiver — reads state, does not mutate
func (u User) Display() string {
    return fmt.Sprintf("%s <%s>", u.Name, u.Email)
}

// Pointer receiver — mutates state or avoids copy
func (u *User) Promote() {
    u.admin = true
}
```

##### Value vs. Pointer receivers — decision table

| Condition | Receiver |
|---|---|
| Method mutates receiver | `*T` |
| Struct is large (copying is costly) | `*T` |
| Struct contains `sync.Mutex` or similar | `*T` (never copy a mutex) |
| Method is read-only and struct is small | `T` |
| Any method on type uses `*T` | use `*T` for ALL methods for consistency |

**Never mix value and pointer receivers on the same type.** It creates confusing method sets and interface satisfaction surprises.

##### Struct alignment / memory layout

```go
// Wastes bytes due to padding
type Bad struct {
    Flag bool  // 1 byte + 7 padding
    Val  int64 // 8 bytes
} // 16 bytes total

// Group large fields first
type Better struct {
    Val  int64 // 8 bytes
    Flag bool  // 1 byte (+ 7 trailing padding)
}
```

For structs with many fields of mixed sizes, grouping fields of the same size minimises padding and total size.

##### Constructor functions

Go has no constructors. The idiomatic pattern is a `New` function:

```go
type Server struct {
    host    string
    port    int
    timeout time.Duration
}

func NewServer(host string, port int) *Server {
    return &Server{
        host:    host,
        port:    port,
        timeout: 30 * time.Second,
    }
}
```

For complex configuration use **functional options**:

```go
type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}

func NewServer(host string, port int, opts ...Option) *Server {
    s := &Server{host: host, port: port, timeout: 30 * time.Second}
    for _, o := range opts {
        o(s)
    }
    return s
}

// Usage
s := NewServer("localhost", 8080, WithTimeout(60*time.Second))
```

---

#### Interfaces & Embedding

##### Interface definition

```go
type Writer interface {
    Write(p []byte) (n int, err error)
}

type Closer interface {
    Close() error
}

// Compose interfaces
type WriteCloser interface {
    Writer
    Closer
}
```

##### Implicit satisfaction

A type satisfies an interface by having all the required methods — no `implements` keyword needed.

```go
type File struct{}

func (f *File) Write(p []byte) (int, error) { return len(p), nil }
func (f *File) Close() error                { return nil }

var wc WriteCloser = &File{} // compiles: *File has Write and Close
```

##### Interface usage patterns

```go
// Accept an interface, return a concrete type (Postel's law)
func Copy(dst Writer, src Reader) (int64, error) { /* ... */ }

// Small interfaces are better — prefer 1-2 methods
// io.Reader, io.Writer, fmt.Stringer are exemplary

// Empty interface — holds any value (use sparingly; prefer `any`)
var anything any = 42
anything = "now a string"

// Type assertion
s, ok := anything.(string)
if !ok {
    // not a string
}
```

##### The `error` interface

```go
type error interface {
    Error() string
}
```

This is the only interface you must implement to return an error.

##### Avoid fat interfaces

```go
// Bad — hard to satisfy, hard to mock
type UserRepository interface {
    Create(u User) error
    Update(u User) error
    Delete(id int) error
    FindByID(id int) (User, error)
    FindAll() ([]User, error)
    // ... 10 more methods
}

// Good — split by usage context
type UserReader interface {
    FindByID(id int) (User, error)
}

type UserWriter interface {
    Create(u User) error
    Update(u User) error
}
```

---

#### Generics

Go 1.18+ supports generics; they are available in every module in this range.

##### Type parameters

```go
func Map[T, U any](s []T, f func(T) U) []U {
    result := make([]U, len(s))
    for i, v := range s {
        result[i] = f(v)
    }
    return result
}

nums := []int{1, 2, 3}
strs := Map(nums, strconv.Itoa) // ["1","2","3"]
```

##### Constraints

```go
// Built-in `comparable` — types that support == and !=
func Contains[T comparable](s []T, target T) bool {
    for _, v := range s {
        if v == target {
            return true
        }
    }
    return false
}

// Custom constraint with union
type Number interface {
    ~int | ~int64 | ~float64
}

func Sum[T Number](s []T) T {
    var total T
    for _, v := range s {
        total += v
    }
    return total
}
```

The `~` prefix means "any type whose underlying type is T" — e.g., `~int` includes `type MyInt int`.

Use `any` instead of `interface{}` for unconstrained type parameters. Keep
constraints small and local when possible.

##### Generic structs

```go
type Stack[T any] struct {
    items []T
}

func (s *Stack[T]) Push(v T)  { s.items = append(s.items, v) }
func (s *Stack[T]) Pop() T {
    n := len(s.items)
    v := s.items[n-1]
    s.items = s.items[:n-1]
    return v
}
```

##### When to use generics

Use generics for utility functions on slices/maps/channels, generic data
structures (Stack, Queue, Set), and algorithms identical across many types.
Prefer ordinary functions and interfaces when there is only one concrete caller
or behavior differs by type.

##### Version notes

- **Type parameters on methods** are not allowed (only on the receiver type) in any version — use a free function.
- **Generic type aliases** (`type MySlice[T any] = []T`) require Go 1.24+.
- **Recursive type constraints** (a type referring to itself in its own type parameter list) require Go 1.26+.

---

#### Collection Types: Arrays, Slices, Maps

##### Arrays

Arrays have **fixed length** baked into their type. `[3]int` and `[4]int` are different types. Arrays are **value types** — copied on assignment.

```go
var a [3]int           // [0, 0, 0]
b := [3]int{1, 2, 3}
c := [...]int{4, 5, 6} // compiler infers length

d := b     // d is a copy
d[0] = 99
fmt.Println(b[0]) // still 1
```

##### Slices

A slice is a **view** into an underlying array: it has a pointer, a length, and a capacity.

```go
s := []int{1, 2, 3}

// make([]T, len, cap)
s2 := make([]int, 0, 10) // length 0, capacity 10

// From array
arr := [5]int{0, 1, 2, 3, 4}
sl := arr[1:4] // [1,2,3] — shares memory with arr

// Append
s2 = append(s2, 1, 2, 3)
extra := []int{4, 5}
s2 = append(s2, extra...)
```

Slicing `s[1:4]` does NOT copy data; it creates a new header pointing into the same array. **Modifying through the slice modifies the backing array.**

```go
a := []int{1, 2, 3, 4, 5}
b := a[1:3]   // [2, 3] — shares backing array
b[0] = 99
fmt.Println(a) // [1, 99, 3, 4, 5] — a was modified!

// To get an independent copy:
c := make([]int, len(b))
copy(c, b)
```

###### Preallocate when size is known

```go
// Bad — O(n) reallocations
var result []string
for _, item := range items {
    result = append(result, process(item))
}

// Good — single allocation
result := make([]string, 0, len(items))
for _, item := range items {
    result = append(result, process(item))
}
```

Always capture the return of `append` — the backing pointer may change.

###### Modifying a slice while iterating

Modifying slice **elements** during a range loop is safe with index-based access. **Appending** during a range loop is dangerous — range evaluates the slice header at entry:

```go
// DO NOT do this
for _, v := range s {
    if v > 0 {
        s = append(s, v*2) // risky: may reallocate; new elements not visited
    }
}

// DO this: collect then append
var extra []int
for _, v := range s {
    if v > 0 {
        extra = append(extra, v*2)
    }
}
s = append(s, extra...)
```

##### Standard library helpers (Go 1.21+)

For Go 1.21 and newer modules, prefer standard library helpers and language
builtins over local generic utilities:

- `slices` for sorting, searching, cloning, compacting, and comparing slices.
- `maps` for cloning, copying, deleting, and iterating map contents.
- `cmp` for ordered comparisons.
- `clear` builtin for clearing maps or zeroing slice contents.
- `min` and `max` builtins for simple ordered comparisons.

For modules on Go 1.19 or 1.20, use the `sort` package and hand-written helpers
instead; `slices`/`maps`/`cmp` and `clear`/`min`/`max` are not available there.

##### Maps

```go
// NEVER use var m map[string]int — it is nil; writing to it panics
m := make(map[string]int)

m2 := map[string]int{
    "alpha": 1,
    "beta":  2,
}

// Two-value form — detect missing keys
v, ok := m["alpha"]
if !ok {
    // key not present
}

m["gamma"] = 3
delete(m, "alpha")
```

Map iteration order is randomised every run. To iterate deterministically,
collect and sort the keys first (`slices.Sort` on Go 1.21+, `sort.Strings` on
1.19/1.20).

###### Modifying a map while iterating

Deleting keys during `range m` is allowed. Do not flag it as a bug by itself:

```go
for key := range values {
    delete(values, key)
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

###### Maps are reference types and not concurrency-safe

```go
a := map[string]int{"x": 1}
b := a           // b and a point to the same map
b["x"] = 99
fmt.Println(a["x"]) // 99

// Concurrent access requires synchronization
var mu sync.RWMutex
var mp = make(map[string]int)

mu.Lock();  mp["a"] = 1; mu.Unlock()
mu.RLock(); _ = mp["a"]; mu.RUnlock()
```

###### Reachability and domain bounds

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

---

#### Strings

##### Key facts

- A Go string is an **immutable** sequence of bytes (not characters).
- String literals are UTF-8 by default.
- `len(s)` returns **bytes**, not Unicode code points (runes).
- `range` on a string iterates **runes**.

```go
s := "héllo"
fmt.Println(len(s))           // 6 (not 5 — é is 2 bytes in UTF-8)
fmt.Println(len([]rune(s)))   // 5

for i, r := range s {
    fmt.Printf("byte index %d: %c (U+%04X)\n", i, r, r)
}
```

##### Efficient string building

```go
// Bad: O(n^2) allocations
s := ""
for i := 0; i < 1000; i++ {
    s += "x"
}

// Good: strings.Builder
var b strings.Builder
b.Grow(1000) // optional pre-allocation
for i := 0; i < 1000; i++ {
    b.WriteByte('x')
}
result := b.String()
```

##### Common operations

```go
import "strings"

strings.Contains("foobar", "bar")        // true
strings.HasPrefix("foobar", "foo")       // true
strings.HasSuffix("foobar", "bar")       // true
strings.Split("a,b,c", ",")             // ["a","b","c"]
strings.Join([]string{"a","b"}, "-")    // "a-b"
strings.TrimSpace("  hello  ")          // "hello"
strings.ToUpper("hello")                // "HELLO"
strings.ReplaceAll("aabbcc", "b", "x")  // "aaxxcc"

// Formatting
s := fmt.Sprintf("user %d: %s", id, name)
```

---

#### Error Handling

##### The error interface

```go
type error interface {
    Error() string
}
```

Errors are **values**. The idiomatic contract: if a function can fail, return `error` as the last value.

##### Explicit error checking

```go
result, err := doSomething()
if err != nil {
    return fmt.Errorf("doSomething failed: %w", err)
}
use(result)
```

**Always check errors.** A few discards are idiomatic and should not be flagged:

- deferred `Close()` on files opened only for reading
- `defer tx.Rollback()`, which is a no-op after a successful commit
- writes to `bytes.Buffer` or `strings.Builder`, which never return an error

For everything else, handle the error or make the discard explicit with `_ =`.

##### Error wrapping with %w

```go
// Wrap — preserves the original error in the chain
return fmt.Errorf("opening config: %w", err)

// errors.Is traverses the chain
errors.Is(wrappedErr, os.ErrNotExist)

// errors.As extracts a type from the chain
var pe *os.PathError
errors.As(wrappedErr, &pe)
```

Do not use `%v` when you need chain inspection later — it breaks the chain.
(Go 1.26+ adds `errors.AsType[T](err)`, a type-safe generic alternative to
`errors.As`.)

##### Sentinel and custom error types

```go
var ErrNotFound = errors.New("not found")

func FindUser(id int) (*User, error) {
    if id == 0 {
        return nil, ErrNotFound
    }
    // ...
}

type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

var ve *ValidationError
if errors.As(err, &ve) {
    fmt.Println("bad field:", ve.Field)
}
```

##### Multiple errors (Go 1.20+)

Use `errors.Join` or multiple `%w` operands in one `fmt.Errorf` (both Go 1.20+)
when an operation can fail in more than one independent way and callers should
still be able to use `errors.Is` or `errors.As`. In modules on Go 1.19,
aggregate failures manually or wrap a single primary cause with `%w`.

```go
var errs []error
if err := closer.Close(); err != nil {
    errs = append(errs, fmt.Errorf("closing file: %w", err))
}
if err := cleanup(); err != nil {
    errs = append(errs, fmt.Errorf("cleaning up: %w", err))
}
return errors.Join(errs...) // Go 1.20+
```

##### What NOT to do

Do not log AND return the same error — pick one (return the error; log at the
top boundary). Otherwise every layer logs it again.

##### Concurrent error collection

```go
import "golang.org/x/sync/errgroup"

g, ctx := errgroup.WithContext(context.Background())
for _, url := range urls {
    url := url // unnecessary on Go 1.22+
    g.Go(func() error {
        return fetch(ctx, url)
    })
}
if err := g.Wait(); err != nil {
    return err
}
```

---

#### Structured Logging with log/slog

`log/slog` is available in **Go 1.21+**. In modules targeting Go 1.20 or
earlier, use the standard `log` package or a third-party structured logger.

Use `log/slog` for structured application logs. Prefer stable key names and
typed values over formatted strings.

```go
logger.InfoContext(ctx, "request complete",
    "method", r.Method,
    "path", r.URL.Path,
    "status", status,
    "duration", elapsed,
)
```

Best practices:

- **Always use structured key-value pairs** — never format strings into the message.
- **Keys should be lowercase snake_case** for consistent log querying.
- **Pass `context.Context` through** and use `InfoContext`/`ErrorContext` for trace IDs.
- **Group related attributes** with `slog.Group` or `logger.WithGroup`.
- **Never log secrets, tokens, credentials, or full request/response bodies** unless they are explicitly scrubbed.

---

#### Concurrency

Go's concurrency model is based on **goroutines** (lightweight threads) and **channels** (typed pipes), inspired by CSP.

> _Don't communicate by sharing memory; share memory by communicating._

##### Goroutines

```go
go func() {
    fmt.Println("running concurrently")
}()

// On Go 1.21 or earlier, pass the loop variable as an argument to avoid capture
for i := 0; i < 5; i++ {
    go func(n int) {
        fmt.Println(n)
    }(i)
}
```

Goroutines start with ~2-4 KB of stack, grown dynamically. You can safely have thousands.

##### Channels

```go
// Unbuffered — synchronises sender and receiver
ch := make(chan int)
go func() { ch <- 42 }()
val := <-ch

// Buffered — send blocks only when buffer is full
bch := make(chan string, 10)
bch <- "hello"  // non-blocking if buffer not full

// Close — signals no more values
close(ch)

// Range over channel until closed
for v := range ch {
    fmt.Println(v)
}

// Check if closed
v, ok := <-ch
if !ok { fmt.Println("closed") }
```

**Channel rules:**
- Only the **sender** should close a channel.
- Sending to a closed channel panics.
- Receiving from a closed channel returns the zero value immediately.

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

On **Go 1.25+**, `wg.Go(func() { process(item) })` combines `Add(1)`, the
goroutine launch, and `defer Done()` into one call.

##### sync.Mutex / sync.RWMutex

```go
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

**Never copy a `sync.Mutex` or `sync.RWMutex`** after first use. Embed them in structs used through pointer receivers.

##### sync.Once and sync.Map

```go
var once sync.Once
func GetConfig() *Config {
    once.Do(func() { instance = loadConfig() })
    return instance
}

// sync.Map — concurrent map for read-heavy or disjoint-key workloads
var m sync.Map
m.Store("key", "value")
v, ok := m.Load("key")
m.Delete("key")
```

`sync.OnceFunc`/`OnceValue`/`OnceValues` (Go 1.21+) memoize lazy initialisation.
`sync.Map.Clear` and `atomic.And`/`atomic.Or` are Go 1.23+.

##### Atomic types

```go
import "sync/atomic"

var counter atomic.Int64
counter.Add(1)
fmt.Println(counter.Load())

// Type-safe atomic pointer
var ptr atomic.Pointer[Config]
ptr.Store(&Config{})
cfg := ptr.Load() // *Config, nil-safe
```

##### Worker pool pattern

```go
func workerPool(jobs <-chan Job, results chan<- Result, numWorkers int) {
    var wg sync.WaitGroup
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs {
                results <- process(job)
            }
        }()
    }
    go func() {
        wg.Wait()
        close(results)
    }()
}
```

##### Concurrency pitfalls

| Pitfall | Symptom | Fix |
|---|---|---|
| Goroutine leak | Memory grows indefinitely | Always provide exit path; use context cancellation |
| Race condition | Occasional wrong values / crashes | Run `go test -race` |
| Deadlock | Program hangs | Ensure every send has a receive; avoid circular locks |
| Closing closed channel | Panic | Only sender closes; use `sync.Once` if multiple potential closers |
| Loop variable capture (Go ≤1.21) | Wrong values in goroutines | Shadow `i := i`, or upgrade to Go 1.22+ |
| Copying mutex | Silently broken locking | Use pointer receivers when struct has a mutex |

##### Race detector

```bash
go test -race ./...
go run -race main.go
```

**Always test concurrent code with `-race`.** Run it in CI.

##### Classifying races before reporting

Separate true data races from lifecycle, shutdown, or ordering races. Only call
something a data race after confirming unsynchronized shared-memory access with
at least one write.

Check a library's concurrency contract before assuming concurrent method calls
are unsafe. Some Go types are explicitly safe for concurrent use, while others
require caller-side synchronization.

---

#### Context Package

`context.Context` is the standard mechanism for cancellation signals, deadlines, and request-scoped values across API boundaries.

##### Creation

```go
// Root contexts
ctx := context.Background() // never cancelled; use at the top of call trees
ctx := context.TODO()       // placeholder when context is undecided

// Derived contexts — always defer cancel
ctx, cancel := context.WithCancel(parent)
defer cancel()

ctx, cancel = context.WithTimeout(parent, 5*time.Second)
defer cancel()

ctx, cancel = context.WithDeadline(parent, time.Now().Add(10*time.Second))
defer cancel()

ctx = context.WithValue(parent, myKey{}, "value")
```

Use cancellation causes (`context.WithCancelCause`/`context.Cause`, Go 1.20+)
when callers need to distinguish why a context ended. `context.WithoutCancel`
and `context.AfterFunc` are Go 1.21+. In modules on Go 1.19, carry the reason
out of band.

##### Convention: context as first parameter

```go
func FetchUser(ctx context.Context, id int) (*User, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    // ...
}
```

##### Checking cancellation in long loops

```go
func processLargeDataset(ctx context.Context, items []Item) error {
    for _, item := range items {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        if err := processItem(ctx, item); err != nil {
            return err
        }
    }
    return nil
}
```

##### context.WithValue

```go
// Use unexported type for keys to prevent cross-package collisions
type contextKey string
const requestIDKey contextKey = "requestID"

ctx = context.WithValue(ctx, requestIDKey, "abc-123")

if id, ok := ctx.Value(requestIDKey).(string); ok {
    log.Println("request", id)
}
```

**Rules:**
- Only store request-scoped data (trace IDs, auth tokens).
- Never store optional function parameters or mutable data.
- Always use an unexported key type.
- **Never store context in a struct** — pass it as the first function parameter.
- **Never pass a nil context** — use `context.Background()`.

---

#### File I/O & Streaming

##### Open / Close with defer

```go
f, err := os.Open("file.txt")
if err != nil {
    return err
}
defer f.Close()
```

**Always `defer f.Close()`** immediately after the error check.

##### Write a file

```go
f, err := os.Create("output.txt")
if err != nil {
    return err
}
defer f.Close()

if _, err := fmt.Fprintln(f, "hello world"); err != nil {
    return err
}
```

##### Buffered I/O with bufio

Raw `os.File` makes a syscall per read/write. Wrap with `bufio` to batch:

```go
// Buffered writer — MUST flush before close
w := bufio.NewWriter(f)
defer w.Flush()

for _, line := range lines {
    fmt.Fprintln(w, line)
}

// Line-by-line reader
scanner := bufio.NewScanner(f)
for scanner.Scan() {
    process(scanner.Text())
}
if err := scanner.Err(); err != nil {
    return err
}

// For lines > 64KB (default buffer):
scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1 MB max line
```

##### Read entire file

```go
// Small files only — reads entire file into memory
data, err := os.ReadFile("config.json")
if err != nil {
    return err
}
```

##### Streaming with io.Reader / io.Writer

```go
written, err := io.Copy(dst, src)

// Limit reader — max N bytes
limited := io.LimitReader(src, 1<<20) // 1 MB

// Tee reader — read and simultaneously write to a writer
var buf bytes.Buffer
tee := io.TeeReader(src, &buf)
io.Copy(dst, tee)
```

##### Temporary files

```go
tmp, err := os.CreateTemp("", "prefix-*.json")
if err != nil {
    return err
}
defer os.Remove(tmp.Name())
defer tmp.Close()
```

##### Walk a directory tree

```go
import "io/fs"

err = filepath.WalkDir("./data", func(path string, d fs.DirEntry, err error) error {
    if err != nil {
        return err
    }
    if !d.IsDir() {
        fmt.Println(path)
    }
    return nil
})
```

##### JSON encoding / decoding

```go
enc := json.NewEncoder(w)
enc.SetIndent("", "  ")
if err := enc.Encode(data); err != nil {
    return err
}

dec := json.NewDecoder(r)
var result MyStruct
if err := dec.Decode(&result); err != nil {
    return err
}

// Struct tags
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email,omitempty"` // omit if empty
    pass  string // unexported — never marshalled
}
```

The `omitzero` struct tag (which correctly omits zero `time.Time` values) is Go 1.24+.

---

#### Testing & Benchmarking

##### Unit tests

Test files end in `_test.go` and live in the same package (or `package foo_test` for black-box testing).

```go
// add_test.go
package math

import "testing"

func TestAdd(t *testing.T) {
    got := Add(2, 3)
    want := 5
    if got != want {
        t.Errorf("Add(2,3) = %d; want %d", got, want)
    }
}
```

##### Table-driven tests (idiomatic Go)

```go
func TestDivide(t *testing.T) {
    tests := []struct {
        name    string
        a, b    float64
        want    float64
        wantErr bool
    }{
        {"normal", 10, 2, 5, false},
        {"division by zero", 10, 0, 0, true},
        {"negative", -6, 2, -3, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Divide(tt.a, tt.b)
            if (err != nil) != tt.wantErr {
                t.Fatalf("Divide() error = %v, wantErr %v", err, tt.wantErr)
            }
            if !tt.wantErr && got != tt.want {
                t.Errorf("Divide() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

##### Benchmarks

```go
func BenchmarkAdd(b *testing.B) {
    for i := 0; i < b.N; i++ {
        Add(2, 3)
    }
}
```

```bash
go test -bench=. -benchmem ./...
```

`-benchmem` shows allocations per operation. On **Go 1.24+**, prefer
`for b.Loop() { ... }` over `for i := 0; i < b.N; i++`.

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

Prefer the built-in lifecycle helpers over manual cleanup: `t.TempDir()`
(1.15+), `t.Setenv()` (1.17+), and `t.Cleanup()` (1.14+). `t.Context()` and
`t.Chdir()` require Go 1.24+.

##### Fuzzing

Add fuzz tests (Go 1.18+) for parsers, decoders, validators, and other code that accepts complex external input.

```go
func FuzzParse(f *testing.F) {
    f.Add("valid input")
    f.Fuzz(func(t *testing.T, input string) {
        _, _ = Parse(input) // must not panic
    })
}
```

`testing/synctest` (Go 1.25+) tests concurrent, time-dependent code with a fake
clock instead of real `time.Sleep`.

##### Race detector in tests

```bash
go test -race ./...
```

Always run this in CI pipelines.

---

#### CLI Development

##### Standard `flag` package (simple CLIs)

```go
func main() {
    host    := flag.String("host", "localhost", "server host")
    port    := flag.Int("port", 8080, "server port")
    verbose := flag.Bool("verbose", false, "enable verbose output")

    flag.Parse()
    args := flag.Args() // positional arguments after flags

    if *verbose {
        fmt.Fprintf(os.Stderr, "connecting to %s:%d\n", *host, *port)
    }
    _ = args
}
```

##### Cobra (recommended for production CLIs)

Used by `kubectl`, GitHub CLI, Docker, and most serious Go CLI tools.

```go
var rootCmd = &cobra.Command{
    Use:   "myapp",
    Short: "My CLI application",
}

var serveCmd = &cobra.Command{
    Use:   "serve",
    Short: "Start the server",
    RunE: func(cmd *cobra.Command, args []string) error {
        port, _ := cmd.Flags().GetInt("port")
        return startServer(port)
    },
}

func init() {
    rootCmd.PersistentFlags().Bool("verbose", false, "verbose output")
    serveCmd.Flags().IntP("port", "p", 8080, "port to listen on")
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

##### Exit codes and stderr

```go
// Recommended main pattern — all logic in run()
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
        os.Exit(1)
    }
}

func run() error {
    // application logic here
    return nil
}
```

Always write errors to stderr and use a non-zero exit code on failure.

---

#### Performance Caveats

##### Escape analysis — stack vs. heap

Go decides at compile time whether a value lives on the stack (fast, auto-cleaned) or heap (GC-managed):

```bash
go build -gcflags="-m" ./...
# outputs: "moved to heap: x"
```

Values escape to the heap when a pointer is returned, stored in an interface, sent on a channel, or captured by an escaping closure.

##### sync.Pool — reuse objects

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

`sync.Pool` objects may be evicted by GC at any time — never use for persistent state.

##### Interface boxing costs

Storing a concrete value in `interface{}` (or `any`) allocates a new object on the heap (the "box"). In tight loops prefer concrete types.

##### Struct memory layout

```go
// 24 bytes (bool has 7 bytes padding before int64)
type Unoptimised struct {
    A bool  // 1 + 7 padding
    B int64 // 8
    C bool  // 1 + 7 padding
}

// 16 bytes — largest fields first minimises padding
type Optimised struct {
    B int64 // 8
    A bool  // 1
    C bool  // 1 (+ 6 trailing padding)
}
```

##### GC tuning

```go
// GOGC: % heap growth before GC (default 100)
debug.SetGCPercent(200) // less frequent GC, more memory used

// GOMEMLIMIT (Go 1.19+): soft total memory limit
debug.SetMemoryLimit(512 << 20) // 512 MiB
```

In containerised deployments: set `GOMEMLIMIT` to ~90% of your container's memory limit to avoid OOM kills while letting the GC run efficiently.

##### Profiling

```bash
# CPU / memory profiles
go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof
go tool pprof cpu.prof

# HTTP pprof endpoint (add to long-running servers)
import _ "net/http/pprof"
go http.ListenAndServe(":6060", nil)
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

---

#### Idioms & Things to Avoid

##### Do

- **Return errors as values, not exceptions.** Never panic for expected failure conditions.
- **Accept interfaces, return concrete types.** Callers get flexibility; API stays concrete and easy to test.
- **Error early** — return in the error branch; continue the happy path without nesting.
- **Use `defer` for all cleanup** (file closes, mutex unlocks, connection releases).
- **Name packages for what they provide**, not what they contain.
- **Write table-driven tests** for any function with multiple input shapes.
- **Use `context.Context` as the first parameter** for any function that does I/O or can be cancelled.
- **Preallocate slices and maps** when the final size is known: `make([]T, 0, n)`.
- **Use `strings.Builder`** for multi-step string construction.
- **Use `fmt.Errorf("context: %w", err)`** to preserve error chains.
- **Always `defer cancel()`** immediately after `context.WithCancel/Timeout/Deadline`.
- **Shadow loop variables** before goroutines on Go ≤1.21 (`i := i`); unnecessary on Go 1.22+.

##### Don't

| Anti-pattern | Why | Instead |
|---|---|---|
| `var m map[string]int` (nil map) | Writing panics | `m := make(map[string]int)` |
| Mixing value and pointer receivers | Confuses method sets | Pick one, stay consistent |
| Ignoring errors with `_` | Silent failures | Always handle or propagate |
| Logging AND returning an error | Double-logging at every layer | Return the error; log at the boundary |
| `time.Sleep` to synchronise goroutines | Racy | `sync.WaitGroup` or channels |
| Global mutable state | Untestable, racy | Dependency injection |
| `interface{}` everywhere | No type safety, boxing overhead | Typed interfaces or generics |
| `panic` for business logic errors | Crashes program | `return error` |
| Copying a `sync.Mutex` | Broken locking | Embed mutex, use pointer receivers |
| Closing a channel from the receiver | Panic if double-closed | Sender closes; use `sync.Once` if needed |
| String concatenation with `+` in loops | O(n^2) allocations | `strings.Builder` |
| Storing context in a struct | Anti-pattern | Pass as first function parameter |
| Appending to a slice during range | Undefined, confusing | Collect then append after loop |

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

// Acronyms stay all-caps
type HTTPClient struct{}
var userID int

// Error variables: Err prefix
var ErrNotFound = errors.New("not found")

// Single-method interfaces: -er suffix
type Reader interface { Read([]byte) (int, error) }
type Stringer interface { String() string }

// Boolean funcs/fields: is/has/can prefix
func (u *User) IsAdmin() bool {}

// No stuttering: user.Name not user.UserName
```

##### Formatting

Go has one formatting style. Run `gofmt` (or `goimports`) on every save. Never debate style — `gofmt` decides.

```bash
gofmt -w .
goimports -w .   # also fixes import ordering and adds missing imports
```

##### Linting

```bash
go vet ./...         # built-in: catches common mistakes
staticcheck ./...    # deeper static analysis
golangci-lint run    # comprehensive lint suite (recommended for CI)
```

A `golangci-lint` v2 configuration (the tool version is independent of the Go
language version):

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

*End of Go Common Developer Guideline*
