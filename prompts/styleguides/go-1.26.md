### Go 1.26 — Complete Developer Guideline

> **Version**: Go 1.26.0 (released 2026-02-10)
> **Scope**: Full language spec, all 1.26 changes delta over 1.25, idioms, concurrency, performance, CLI, file I/O, testing, and best practices — with examples throughout.
> **Note**: This document is self-contained. All relevant prior-version material is carried forward or superseded. Sections marked **[NEW in 1.26]** cover additions specific to this version.

---

#### Table of Contents

1. [What's New in Go 1.26](#whats-new-in-go-126)
2. [Project Layout & Modules](#project-layout--modules)
3. [Basic Types & Variables](#basic-types--variables)
4. [Pointers — new(expr)](#pointers--newexpr)
5. [Control Flow](#control-flow)
6. [Functions](#functions)
7. [Defer, Panic & Recover](#defer-panic--recover)
8. [Structs & Methods](#structs--methods)
9. [Interfaces & Embedding](#interfaces--embedding)
10. [Generics — Recursive Type Constraints](#generics--recursive-type-constraints)
11. [Iterators — iter package](#iterators--iter-package)
12. [Collection Types: Arrays, Slices, Maps](#collection-types-arrays-slices-maps)
13. [Strings & Bytes](#strings--bytes)
14. [Error Handling — errors.AsType](#error-handling--errorsastype)
15. [Structured Logging with log/slog](#structured-logging-with-logslog)
16. [Concurrency](#concurrency)
17. [Memory Management: Cleanup, Weak & unique](#memory-management-cleanup-weak--unique)
18. [Context Package](#context-package)
19. [File I/O & Filesystem](#file-io--filesystem)
20. [JSON — v1 and experimental v2](#json--v1-and-experimental-v2)
21. [HTTP Servers](#http-servers)
22. [Cryptography](#cryptography)
23. [Testing & Benchmarking](#testing--benchmarking)
24. [CLI Development & Toolchain](#cli-development--toolchain)
25. [Performance Caveats & PGO](#performance-caveats--pgo)
26. [Observability: Tracing, Profiling & Goroutine Leaks](#observability-tracing-profiling--goroutine-leaks)
27. [Idioms & Things to Avoid](#idioms--things-to-avoid)

---

#### What's New in Go 1.26

Go 1.26.0 shipped 2026-02-10. It has **two language changes**, makes the **Green Tea GC default**, slashes **cgo call overhead by ~30%**, ships the new **`go fix` modernizer**, adds **`errors.AsType`**, introduces experimental **SIMD** and **`runtime/secret`** packages, adds an experimental **goroutine leak pprof profile**, a new **`crypto/hpke`** package, and makes most crypto APIs **reader-less** (random `io.Reader` parameters now ignored).

##### Language changes

###### 1. new(expr) — pointer to a value [NEW in 1.26] ★★

Previously `new` only accepted a type. Now it accepts any expression, initialising the pointed-to variable.

```go
// Before 1.26: verbose — need a temp variable to get a pointer
name := "Alice"
type User struct { Name *string; Age *int }
u := User{Name: &name}

// Or a helper function:
func ptr[T any](v T) *T { return &v }
u2 := User{Name: ptr("Alice"), Age: ptr(30)}

// Go 1.26: new(expr) — clean one-liner
u3 := User{
    Name: new("Alice"),
    Age:  new(30),
}
```

**Full semantics**: `new(expr)` allocates a variable of type `T` (the type of `expr`), initialises it to the value of `expr`, and returns `*T`. Works with literals, composite values, function calls, and constant expressions.

```go
// Literals
p1 := new(42)           // *int
p2 := new(true)         // *bool
p3 := new("hello")      // *string
p4 := new(3.14)         // *float64
p5 := new(time.Second)  // *time.Duration

// Composite values
sl := new([]int{1, 2, 3})  // *[]int
type Point struct{ X, Y int }
pt := new(Point{X: 1, Y: 2})  // *Point

// Function calls
f := func() string { return "go" }
ps := new(f())  // *string, value "go"

// Not allowed: nil, untyped nil
// p := new(nil)  // compile error
```

**Prime use case — optional JSON/proto fields:**

```go
type Config struct {
    Host    string  `json:"host"`
    Port    *int    `json:"port,omitempty"`     // nil means "not set"
    Verbose *bool   `json:"verbose,omitempty"`  // nil means "not set"
    Timeout *int    `json:"timeout,omitzero"`   // nil means "not set"
}

// Before 1.26: clunky
port := 8080; verbose := true
cfg := Config{Host: "localhost", Port: &port, Verbose: &verbose}

// After 1.26: clean
cfg := Config{
    Host:    "localhost",
    Port:    new(8080),
    Verbose: new(true),
    Timeout: new(30),
}
```

###### 2. Recursive type constraints [NEW in 1.26] ★

Generic types may now refer to themselves in their own type parameter list. Previously this caused a compile error: `invalid recursive type`.

```go
// A type that can be compared/added with instances of its own type
type Ordered[T Ordered[T]] interface {
    Less(T) bool
}

type Adder[A Adder[A]] interface {
    Add(A) A
}

// Generic algorithm parameterised by a self-referential constraint
func algo[A Adder[A]](x, y A) A {
    return x.Add(y)
}

// A tree that requires its values to be self-comparable
type Tree[T Ordered[T]] struct {
    nodes []T
}

// netip.Addr has a Less(Addr) bool method — satisfies Ordered[netip.Addr]
t := Tree[netip.Addr]{}
```

**Common patterns this enables:**

```go
// Builder pattern with fluent interface
type Builder[B Builder[B]] interface {
    SetName(string) B
    SetTimeout(time.Duration) B
    Build() Service
}

// Observable with typed self
type Observable[T Observable[T]] interface {
    Subscribe(func(T))
    Notify(T)
}

// Comparable value objects
type Value[V Value[V]] interface {
    Equal(V) bool
    Hash() uint64
}
```

---

#### Project Layout & Modules

##### go mod init now defaults to N-1 [NEW in 1.26]

```bash
# Using Go 1.26 toolchain:
go mod init myapp
# Creates go.mod with: go 1.25.0 (not 1.26.0)
# This encourages modules compatible with currently supported Go versions.

# To override to the current toolchain version:
go mod init myapp && go get go@1.26.0
```

```
module github.com/yourname/myapp

go 1.25.0   // <-- N-1 default from go mod init with 1.26 toolchain

require (
    github.com/spf13/cobra v1.8.0
)

tool (
    golang.org/x/tools/cmd/stringer
    github.com/golangci/golangci-lint/cmd/golangci-lint
)

ignore (
    ./vendor
    ./generated
)
```

##### Bootstrap requirement [NEW in 1.26]

Go 1.26 requires Go 1.24.6 or later to bootstrap.

##### go tool doc [NEW in 1.26]

`cmd/doc` and `go tool doc` have been removed. Use `go doc` directly — it now accepts the same flags and arguments.

```bash
# Previously:
go tool doc fmt.Println
# Now:
go doc fmt.Println       # same thing, -http flag supported
go doc -http fmt.Println # open browser
```

##### Verify library APIs against the pinned version

Verify library APIs against the actual module versions in `go.mod` before
claiming an API is missing or unavailable. Track executable development tools
with `tool` directives (1.24+) rather than a blank-import `tools.go` file.

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
m := min(x, 20); M := max(x, 20)

type Direction int
const (
    North Direction = iota
    East; South; West
)

type Permission uint
const (
    Read    Permission = 1 << iota
    Write; Execute
)
```

---

#### Pointers — new(expr)

##### Strong pointers

```go
// Pre-1.26 patterns — still valid, but often verbose
x := 42; p := &x

// Temp-variable for struct field
val := "hello"; s := struct{ V *string }{V: &val}

// Generic helper — still works but no longer necessary for scalars
func ptr[T any](v T) *T { return &v }

// 1.26+: use new(expr) directly
s2 := struct{ V *string }{V: new("hello")}
cfg := Config{Timeout: new(30), Enabled: new(true)}
```

##### Weak pointers (1.24+)

```go
import "weak"
strong := &ExpensiveObject{data: make([]byte, 1<<20)}
wp := weak.Make(strong)
if obj := wp.Value(); obj != nil { use(obj) }
```

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
for v := range myIter() {}      // 1.23+
for k, v := range myIter2() {}  // 1.23+
```

##### switch / select

```go
switch x { case 1: default: }
switch { case x < 0: default: }
func describe(i any) {
    switch v := i.(type) {
    case int: fmt.Printf("int: %d\n", v)
    case string: fmt.Printf("string: %s\n", v)
    }
}

select {
case msg := <-ch1: fmt.Println(msg)
case ch2 <- "hello": fmt.Println("sent")
case <-time.After(time.Second): fmt.Println("timeout")
default: fmt.Println("non-blocking")
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
    total := 0; for _, n := range nums { total += n }; return total
}
func counter() func() int { n := 0; return func() int { n++; return n } }
```

---

#### Defer, Panic & Recover

```go
defer f.Close()
defer func() { log.Println(time.Since(start)) }()
defer func() {
    if r := recover(); r != nil { /* panic occurred */ }
}()
```

##### Defer inside iterator loops

```go
// WRONG: all defers run at function return
for path := range filePaths() { f, _ := os.Open(path); defer f.Close() }

// CORRECT
for path := range filePaths() {
    func() {
        f, err := os.Open(path); if err != nil { return }
        defer f.Close(); process(f)
    }()
}
```

---

#### Structs & Methods

```go
type User struct { ID int; Name, Email string; admin bool }
func (u User) Display() string { return u.Name }
func (u *User) Promote()       { u.admin = true }

// new(expr) shines with optional struct fields
type ServerConfig struct {
    Host    string
    Port    *int    `json:"port,omitempty"`
    Debug   *bool   `json:"debug,omitempty"`
}
cfg := ServerConfig{Host: "localhost", Port: new(8080), Debug: new(true)}
```

##### Value vs. Pointer receivers

| Condition | Receiver |
|---|---|
| Mutates receiver | `*T` |
| Large struct | `*T` |
| Contains sync primitive | `*T` — never copy |
| Read-only, small | `T` |
| Any method uses `*T` | `*T` for ALL |

---

#### Interfaces & Embedding

```go
type Writer interface { Write(p []byte) (n int, err error) }
type WriteCloser interface { Writer; io.Closer }
var x any = "hello"
s, ok := x.(string)
switch v := x.(type) { case int: _ = v }
```

##### crypto.Encapsulator / Decapsulator [NEW in 1.26]

```go
// New interfaces for accepting abstract KEM keys
type Encapsulator interface { Encapsulate() (ciphertext, sharedSecret []byte, err error) }
type Decapsulator interface { Decapsulate(ciphertext []byte) (sharedSecret []byte, err error) }

// KeyExchanger — abstract ECDH private keys (hardware keys, etc.)
type KeyExchanger interface { ECDH(remote *ecdh.PublicKey) ([]byte, error) }
// Implemented by *ecdh.PrivateKey

// crypto.MessageSigner — used by TLS 1.2+ (1.25+)
type MessageSigner interface {
    crypto.Signer
    SignMessage(rand io.Reader, msg []byte, opts crypto.SignerOpts) ([]byte, error)
}
```

##### slog.NewMultiHandler [NEW in 1.26]

```go
// Fan out log records to multiple handlers
h1 := slog.NewJSONHandler(os.Stdout, nil)
h2 := slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelWarn})

multi := slog.NewMultiHandler(h1, h2)
logger := slog.New(multi)
slog.SetDefault(logger)

// Enabled() returns true if ANY handler is enabled
// Handle/WithAttrs/WithGroup call each enabled handler
slog.Info("this goes to stdout only")
slog.Warn("this goes to both stdout and file")
```

---

#### Generics — Recursive Type Constraints

```go
// Self-referential: T Ordered[T]
type Ordered[T Ordered[T]] interface { Less(T) bool }
type Tree[T Ordered[T]] struct { nodes []T }

// Fluent builder
type RequestBuilder[B RequestBuilder[B]] interface {
    WithHeader(k, v string) B
    WithTimeout(time.Duration) B
    Build() *http.Request
}

// Regular generics (unchanged)
func Map[T, U any](s []T, f func(T) U) []U {
    r := make([]U, len(s)); for i, v := range s { r[i] = f(v) }; return r
}

func Contains[T comparable](s []T, v T) bool {
    for _, e := range s { if e == v { return true } }; return false
}

// Generic type aliases (stable since 1.24)
type Seq[V any] = func(yield func(V) bool)
type UserRepo = storage.Repository[User]

import "cmp"
host := cmp.Or(os.Getenv("HOST"), cfg.Host, "localhost")
```

---

#### Iterators — iter package

Stable since 1.23. Summary:

```go
import "iter"
type Seq[V any]     = func(yield func(V) bool)
type Seq2[K, V any] = func(yield func(K, V) bool)

func Evens(n int) iter.Seq[int] {
    return func(yield func(int) bool) {
        for i := 0; i < n; i += 2 { if !yield(i) { return } }
    }
}
for v := range Evens(10) { fmt.Println(v) }

next, stop := iter.Pull(mySeq); defer stop()
for { v, ok := next(); if !ok { break }; process(v) }
```

##### strings / bytes lazy iterators (1.24+)

```go
for line := range strings.Lines("hello\nworld\n") { fmt.Println(line) }
for part := range strings.SplitSeq("a,b,c", ",") { fmt.Println(part) }
for word := range strings.FieldsSeq("hello world") { fmt.Println(word) }
```

##### reflect.Type/Value iterators [NEW in 1.26]

```go
import "reflect"

// Type.Fields — iterate struct fields
typ := reflect.TypeFor[http.Client]()
for f := range typ.Fields() {
    fmt.Println(f.Name, f.Type)
}

// Type.Methods — iterate methods
for m := range typ.Methods() {
    fmt.Println(m.Name, m.Type)
}

// Type.Ins / Type.Outs — function parameters
fnTyp := reflect.TypeFor[filepath.WalkFunc]()
for p := range fnTyp.Ins()  { fmt.Println("in:", p.Name()) }
for p := range fnTyp.Outs() { fmt.Println("out:", p.Name()) }

// Value.Fields / Value.Methods
val := reflect.ValueOf(&http.Client{})
for f, v := range val.Elem().Fields()  { fmt.Println(f.Name, v.Kind()) }
for m, v := range val.Methods()        { fmt.Println(m.Name, v.Kind()) }
```

---

#### Collection Types: Arrays, Slices, Maps

##### Slices — further expanded stack allocation [NEW in 1.26]

The compiler allocates slice backing stores on the stack in even more situations than 1.25. Net effect: less heap pressure, fewer GC cycles. No code changes required.

```bash
# If this causes trouble, bisect to find the problematic allocation:
go test -gcflags=all=-d=variablemakehash=n ./...
# Or via bisect:
bisect -compile=variablemake go test ./...
```

##### slices package (cumulative through 1.23)

```go
import "slices"
slices.Sort(s); slices.SortFunc(s, cmpFn)
idx, found := slices.BinarySearch(sorted, v)
slices.Contains(s, v); slices.Reverse(s)
slices.Compact(s); slices.DeleteFunc(s, fn)
slices.Clip(s); slices.Grow(s, n)
slices.Insert(s, i, vs...); slices.Delete(s, i, j)
slices.Concat(ss...); slices.Repeat(s, n)
slices.Chunk(s, n)       // iter.Seq[[]E]
slices.All(s)            // iter.Seq2[int, E]
slices.Values(s)         // iter.Seq[E]
slices.Backward(s)       // iter.Seq2[int, E]
slices.Collect(seq); slices.AppendSeq(s, seq)
slices.Sorted(seq); slices.SortedFunc(seq, fn)
slices.Max(s); slices.Min(s)
```

##### maps package (cumulative through 1.23)

```go
import "maps"
maps.Clone(m); maps.Copy(dst, src)
maps.DeleteFunc(m, fn); maps.Equal(m1, m2)
maps.All(m); maps.Keys(m); maps.Values(m)
maps.Insert(m, seq); maps.Collect(seq)
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

#### Strings & Bytes

##### bytes.Buffer.Peek [NEW in 1.26]

```go
buf := bytes.NewBufferString("hello world")

// Peek returns next n bytes WITHOUT advancing the buffer
sample, err := buf.Peek(5)
fmt.Printf("peek=%s err=%v\n", sample, err) // peek=hello err=<nil>

buf.Next(6) // advance past "hello "

sample, err = buf.Peek(5)
fmt.Printf("peek=%s err=%v\n", sample, err) // peek=world err=<nil>

// If fewer than n bytes remain, returns them all + io.EOF
sample, err = buf.Peek(100)
fmt.Printf("peek=%s err=%v\n", sample, err) // peek=world err=EOF

// WARNING: returned slice points into buffer memory — only valid until next read/write
sample[0] = 'W' // modifies the buffer!
```

##### Strings operations

```go
s := "héllo"
strings.Contains("foobar", "bar")
strings.HasPrefix("foobar", "foo")
strings.Split("a,b,c", ",")          // allocates []string
strings.SplitSeq("a,b,c", ",")       // 1.24+: lazy iterator
strings.Lines("line1\nline2\n")      // 1.24+: lazy iterator
strings.FieldsSeq("hello world")     // 1.24+: lazy iterator
strings.Join([]string{"a","b"}, "-")
strings.TrimSpace("  hello  ")
strings.CutPrefix("Gopher", "Go")
strings.CutSuffix("Gopher", "er")

var b strings.Builder
b.Grow(1000); for range 1000 { b.WriteByte('x') }; result := b.String()
```

---

#### Error Handling — errors.AsType

##### errors.AsType [NEW in 1.26] ★★

```go
// Before 1.26: errors.As requires a temporary pointer variable
var ve *ValidationError
if errors.As(err, &ve) {
    fmt.Println(ve.Field)
}

// Go 1.26: errors.AsType — type-safe generic version
if ve, ok := errors.AsType[*ValidationError](err); ok {
    fmt.Println(ve.Field)  // ve is already *ValidationError — no cast needed
}
```

**Why prefer `errors.AsType`:**

| Feature | `errors.As` | `errors.AsType` |
|---|---|---|
| Type safety | Runtime panic if misused | Compile-time error |
| Verbosity | Needs temp var | Single expression |
| Uses reflect | Yes — slower | No — faster |
| Benchmark | ~95 ns, 40B, 2 allocs | ~30 ns, 24B, 1 alloc |
| Variable scope | Leaks outside `if` | Scoped to `if` block |

```go
// Multi-error type checking — much cleaner
if connErr, ok := errors.AsType[*net.OpError](err); ok {
    fmt.Println("Network:", connErr.Op)
} else if dnsErr, ok := errors.AsType[*net.DNSError](err); ok {
    fmt.Println("DNS:", dnsErr.Name)
} else if dbErr, ok := errors.AsType[*pgconn.PgError](err); ok {
    fmt.Println("DB:", dbErr.Code)
} else {
    fmt.Println("Unknown:", err)
}
```

**Compile-time error for wrong usage** (non-pointer type without error interface):

```go
// errors.As: runtime panic
var target ValidationError  // not a pointer
errors.As(err, &target)     // panic: *target must implement error

// errors.AsType: compile-time error
errors.AsType[ValidationError](err)  // compile error: ValidationError does not satisfy error
```

##### Standard error handling

```go
result, err := doSomething()
if err != nil { return fmt.Errorf("context: %w", err) }

var ErrNotFound = errors.New("not found")
if errors.Is(err, ErrNotFound) { /* handle */ }

combined := errors.Join(err1, err2)
wrapped  := fmt.Errorf("two: %w and %w", err1, err2)
```

**Always check errors.** A few discards are idiomatic and should not be flagged:

- deferred `Close()` on files opened only for reading
- `defer tx.Rollback()`, which is a no-op after a successful commit
- writes to `bytes.Buffer` or `strings.Builder`, which never return an error

For everything else, handle the error or make the discard explicit with `_ =`.

---

#### Structured Logging with log/slog

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo, AddSource: true,
}))
slog.SetDefault(logger)

slog.Info("request",
    slog.String("method", "GET"),
    slog.Int("status", 200),
    slog.Duration("latency", 12*time.Millisecond),
)

// Multi-handler: fan out to multiple destinations (1.26+)
h1 := slog.NewJSONHandler(os.Stdout, nil)
h2 := slog.NewTextHandler(alertFile, &slog.HandlerOptions{Level: slog.LevelError})
slog.SetDefault(slog.New(slog.NewMultiHandler(h1, h2)))

// Context-aware, with group
reqLogger := logger.WithGroup("request").With("id", reqID)
slog.InfoContext(ctx, "handler", "status", 200)

// Discard in benchmarks (1.24+)
slog.SetDefault(slog.New(slog.DiscardHandler{}))
```

##### slog best practices

- **Always use structured key-value pairs** — never format strings into the message.
- **Keys should be lowercase snake_case** for consistent log querying.
- **Pass `context.Context` through** and use `InfoContext`/`ErrorContext` for trace IDs.
- **Group related attributes** with `slog.Group`/`slog.GroupAttrs` or `logger.WithGroup`.
- **Never log secrets, tokens, credentials, or full request/response bodies** unless they are explicitly scrubbed.

---

#### Concurrency

##### sync.WaitGroup.Go (1.25+)

```go
var wg sync.WaitGroup
for _, item := range items {
    wg.Go(func() { process(item) })  // Add(1) + go + defer Done
}
wg.Wait()
```

##### Channels (timer channels: unbuffered since 1.23)

```go
ch := make(chan int); bch := make(chan string, 10)
t := time.NewTimer(5 * time.Second); defer t.Stop()
select { case <-t.C: default: }
t.Reset(10 * time.Second)
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

##### sync/atomic

```go
var counter atomic.Int64; counter.Add(1)
var flags atomic.Uint32
flags.Or(0b0100)           // set bit
flags.And(^uint32(0b0100)) // clear bit
```

##### testing/synctest (stable since 1.25)

```go
import "testing/synctest"
func TestDebounce(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        calls := 0
        fn := Debounce(func() { calls++ }, 100*time.Millisecond)
        fn(); fn(); fn()
        synctest.Wait()
        time.Sleep(100 * time.Millisecond) // instant in bubble
        synctest.Wait()
        if calls != 1 { t.Errorf("got %d calls", calls) }
    })
}
```

##### Goroutine leak profile [NEW in 1.26] (experimental) ★★

```bash
# Enable at build time
GOEXPERIMENT=goroutineleakprofile go build ./cmd/myapp
```

```go
// Access via pprof
prof := pprof.Lookup("goroutineleak")
prof.WriteTo(os.Stdout, 2)
// Output: goroutine 7 [chan send (leaked)]:
//   main.processWorkItems.func1()
//     /app/main.go:42 +0x1e

// HTTP endpoint (when experiment enabled)
http.HandleFunc("/debug/pprof/goroutineleak", pprof.Handler("goroutineleak").ServeHTTP)

// Common leak pattern — early return from goroutine fan-out
func processItems(items []Item) ([]Result, error) {
    ch := make(chan result)
    for _, item := range items {
        go func() {
            res, err := process(item)
            ch <- result{res, err} // LEAK if caller returns early!
        }()
    }
    var results []Result
    for range len(items) {
        r := <-ch
        if r.err != nil { return nil, r.err } // remaining goroutines leak
        results = append(results, r.res)
    }
    return results, nil
}

// FIX: use buffered channel or context cancellation
func processItems(ctx context.Context, items []Item) ([]Result, error) {
    ch := make(chan result, len(items)) // buffered — goroutines never block
    var wg sync.WaitGroup
    for _, item := range items {
        wg.Go(func() { res, err := process(item); ch <- result{res, err} })
    }
    go func() { wg.Wait(); close(ch) }()
    var results []Result
    for r := range ch {
        if r.err != nil { return nil, r.err }
        results = append(results, r.res)
    }
    return results, nil
}
```

**Intended for Go 1.27 default.** Try it now to catch leaks in production.

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

#### Memory Management: Cleanup, Weak & unique

##### runtime.AddCleanup (1.24+) — parallel since 1.25

```go
cleanup := runtime.AddCleanup(obj, func(id int) {
    closeHandle(id) // runs concurrently with other cleanups
}, obj.id)
// cleanup.Stop() cancels if not yet run
```

##### Weak pointer cache (1.24+)

```go
type Cache[K comparable, V any] struct {
    mu sync.Mutex
    m  map[K]weak.Pointer[V]
}
func (c *Cache[K, V]) GetOrLoad(key K, load func(K) *V) *V {
    c.mu.Lock(); defer c.mu.Unlock()
    if wp, ok := c.m[key]; ok { if v := wp.Value(); v != nil { return v } }
    v := load(key)
    c.m[key] = weak.Make(v)
    runtime.AddCleanup(v, func(k K) {
        c.mu.Lock(); defer c.mu.Unlock()
        if e, ok := c.m[k]; ok && e.Value() == nil { delete(c.m, k) }
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
ctx, cancel  = context.WithCancelCause(parent); defer cancel(nil)
detached     := context.WithoutCancel(requestCtx)
```

##### os/signal.NotifyContext — signal cause [NEW in 1.26]

```go
// NotifyContext now uses CancelCauseFunc — you can retrieve which signal fired
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

<-ctx.Done()
cause := context.Cause(ctx)
fmt.Printf("shutting down due to: %v\n", cause)
// Output: shutting down due to: signal: interrupt
```

---

#### File I/O & Filesystem

##### Standard I/O

```go
f, err := os.Open("file.txt"); if err != nil { return err }; defer f.Close()
scanner := bufio.NewScanner(f)
scanner.Buffer(make([]byte, 1<<20), 1<<20)
for scanner.Scan() { process(scanner.Text()) }
if err := scanner.Err(); err != nil { return err }
io.Copy(dst, src)
data, err := os.ReadFile("small.json")
```

##### io.ReadAll — 2x faster [NEW in 1.26]

```go
// io.ReadAll is now about 2x faster, allocates ~50% less memory
// No code changes needed — existing code benefits automatically
data, err := io.ReadAll(resp.Body)  // transparent improvement

// Under the hood: allocates less intermediate memory,
// returns minimally-sized final slice (less over-allocation)
```

##### os.Root — sandboxed filesystem (1.24+)

Full API now includes all `os` package operations within a sandbox:

```go
root, err := os.OpenRoot("./uploads"); defer root.Close()
f, _ := root.Open("user/file.txt")      // symlink attacks blocked
f2, _ := root.Create("user/new.txt")
root.MkdirAll("a/b/c", 0755)
root.RemoveAll("old_dir")
root.Rename("old.txt", "new.txt")
root.Symlink("target", "link")
root.Readlink("link")
root.ReadFile("config.json")
root.WriteFile("out.json", data, 0644)
```

##### os.Process.WithHandle [NEW in 1.26]

```go
// Access to OS process handle (pidfd on Linux 5.4+, Handle on Windows)
proc, err := os.StartProcess("/bin/myapp", []string{"myapp"}, attr)
if err != nil { return err }
defer proc.Wait()

// Get the underlying OS handle
if handle, ok := proc.WithHandle(); ok {
    // Use handle for OS-specific process management operations
    // e.g., send signals via pidfd, query process status
    fmt.Println("process handle available")
}
```

##### net/url.Parse strictness [NEW in 1.26]

```go
// Now rejects malformed URLs with colons in host subcomponent
_, err := url.Parse("http://::1/")           // now returns error
_, err  = url.Parse("http://localhost:80:80/") // now returns error
_, err  = url.Parse("http://[::1]/")          // still accepted (bracketed IPv6)

// Restore old lenient behavior if needed:
// GODEBUG=urlstrictcolons=0 ./myapp
```

##### net/http/httputil.ReverseProxy.Director deprecated [NEW in 1.26]

```go
// DEPRECATED: Director allows malicious clients to remove proxy-added headers
proxy := &httputil.ReverseProxy{
    Director: func(req *http.Request) {  // deprecated
        req.Header.Set("X-Forwarded-For", req.RemoteAddr)
    },
}

// PREFERRED: Rewrite (introduced in 1.20)
proxy = &httputil.ReverseProxy{
    Rewrite: func(r *httputil.ProxyRequest) {
        r.SetXForwarded()
        r.Out.Header.Set("X-Custom", "value")
        // r.In is the original request, r.Out is the outbound request
    },
}
```

##### JSON

```go
type Event struct {
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at,omitzero"`  // 1.24+
    Tags      []string  `json:"tags,omitempty"`
    Count     *int      `json:"count,omitempty"`
    Enabled   *bool     `json:"enabled,omitempty"`
}
// 1.26: clean pointer fields with new(expr)
e := Event{Name: "signup", Count: new(42), Enabled: new(true)}
```

---

#### JSON — v1 and experimental v2

##### JSON v1 (encoding/json) — fmt.Errorf optimisation [NEW in 1.26]

```go
// Unformatted strings in fmt.Errorf are now as efficient as errors.New
// e.g., fmt.Errorf("not found") now matches errors.New("not found")
// No code changes needed
err := fmt.Errorf("user %d not found", id) // still uses formatting
err  = fmt.Errorf("not found")              // now zero extra alloc
```

##### encoding/json/v2 — experimental (GOEXPERIMENT=jsonv2)

```go
// Test compatibility before v2 becomes default
// GOEXPERIMENT=jsonv2 go test ./...

import "encoding/json/v2"
data, err := jsonv2.Marshal(user)
err = jsonv2.Unmarshal(data, &user,
    jsonv2.RejectUnknownMembers(true),
)
```

---

#### HTTP Servers

##### CrossOriginProtection (1.25+)

```go
cop := http.NewCrossOriginProtection()
cop.AddAllowedOrigin("https://app.example.com")
cop.AddPermittedPattern("/api/webhooks/")
handler := cop.Handler(mux)
```

##### Enhanced ServeMux (1.22+)

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    slog.Info("request", "pattern", r.Pattern)
    fmt.Fprintf(w, "user: %s\n", id)
})
```

##### ServeMux trailing-slash redirect [NEW in 1.26]

```go
// ServeMux now redirects trailing slashes with 307 Temporary Redirect
// instead of 301 Moved Permanently
// (307 preserves the HTTP method for POST/PUT/DELETE requests)
mux.HandleFunc("/api/users/", handler) // trailing-slash redirect is now 307
```

##### httptest.Server example.com redirect [NEW in 1.26]

```go
// Server.Client now redirects example.com (and subdomains) to the test server
func TestMyHandler(t *testing.T) {
    ts := httptest.NewServer(myHandler)
    defer ts.Close()
    client := ts.Client()

    // Before 1.26: had to use ts.URL directly for all requests
    // After 1.26: requests to example.com (and *.example.com) are redirected to ts
    resp, err := client.Get("https://example.com/api/test")
    // equivalent to client.Get(ts.URL + "/api/test")
    // Extremely useful for testing code that hardcodes example.com
}
```

---

#### Cryptography

##### crypto/hpke — Hybrid Public Key Encryption [NEW in 1.26] ★★

```go
import "crypto/hpke"

// One-shot API: Seal/Open
kem, kdf, aead := hpke.MLKEM768X25519(), hpke.HKDFSHA256(), hpke.AES256GCM()

// Recipient: generate key pair
key, err := kem.GenerateKey()
pubKeyBytes := key.PublicKey().Bytes()

// Sender: encrypt
pubKey, _ := kem.NewPublicKey(pubKeyBytes)
ciphertext, err := hpke.Seal(pubKey, kdf, aead, []byte("public info"), []byte("secret message"))

// Recipient: decrypt
plaintext, err := hpke.Open(key, kdf, aead, []byte("public info"), ciphertext)
fmt.Println(string(plaintext)) // secret message

// Available KEM constructors
hpke.MLKEM768X25519()     // post-quantum hybrid, a.k.a. X-Wing (recommended)
hpke.MLKEM768P256()       // post-quantum hybrid over P-256
hpke.MLKEM768()           // ML-KEM-768 (post-quantum only)
hpke.MLKEM1024()          // ML-KEM-1024 (higher security)
hpke.MLKEM1024P384()      // post-quantum hybrid over P-384
hpke.DHKEM(ecdh.X25519()) // classical DHKEM (also P-256/P-384/P-521)
// KDFs: hpke.HKDFSHA256/384/512, hpke.SHAKE128/256
// AEADs: hpke.AES128GCM, hpke.AES256GCM, hpke.ChaCha20Poly1305
```

**Use HPKE instead of RSA encryption for all new code. As Filippo Valsorda says: HPKE is now the right way to do public key encryption.**

##### Reader-less crypto APIs [NEW in 1.26] ★★

Most `crypto` package functions that previously accepted `io.Reader` as a randomness source now **ignore** the parameter and always use the system secure random source.

```go
// BEFORE 1.26: rand parameter was used
key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
sig, _ := ecdsa.SignASN1(rand.Reader, privKey, hash)
rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
p, _ := randPkg.Prime(rand.Reader, 256)

// AFTER 1.26: pass nil (or anything — rand parameter is ignored)
key, _ = ecdsa.GenerateKey(elliptic.P256(), nil)   // nil is fine
sig, _ = ecdsa.SignASN1(nil, privKey, hash)         // nil is fine
rsaKey, _ = rsa.GenerateKey(nil, 2048)             // nil is fine
p, _ = randPkg.Prime(nil, 256)                     // nil is fine

// Affected functions:
// crypto/dsa:    GenerateKey
// crypto/ecdh:   Curve.GenerateKey
// crypto/ecdsa:  GenerateKey, SignASN1, Sign, PrivateKey.Sign
// crypto/rsa:    GenerateKey, GenerateMultiPrimeKey, EncryptPKCS1v15
// crypto/rand:   Prime

// Exception: crypto/ed25519.GenerateKey still uses rand if non-nil

// For deterministic testing: testing/cryptotest.SetGlobalRandom
func TestCrypto(t *testing.T) {
    cryptotest.SetGlobalRandom(t, 42) // deterministic seed for this test
    key, _ := ecdsa.GenerateKey(elliptic.P256(), nil)
    _ = key // same key every run with seed 42
}

// Restore old rand-respecting behavior temporarily:
// GODEBUG=cryptocustomrand=1 ./myapp  (will be removed in future)
```

##### TLS — new post-quantum defaults [NEW in 1.26]

```go
// SecP256r1MLKEM768 and SecP384r1MLKEM1024 are now enabled by default
// (in addition to X25519MLKEM768 from 1.24)
// Disable via CurvePreferences or GODEBUG:
// GODEBUG=tlssecpmlkem=0 ./myapp

cfg := &tls.Config{
    MinVersion: tls.VersionTLS12,
    // ClientHelloInfo.HelloRetryRequest now available
    // ConnectionState.HelloRetryRequest now available
}
```

##### FIPS 140-3 — selective enforcement [NEW in 1.26]

```go
import "crypto/fips140"

// Run most code in FIPS mode, but selectively disable for specific operations
GODEBUG=fips140=only ./myapp  // strict FIPS

// In code: temporarily disable strict checks
fips140.WithoutEnforcement(func() {
    // Non-FIPS algorithms allowed here
    h := md5.New() // would normally panic in fips140=only mode
    h.Write(legacyData)
})

// Check module version
v := fips140.Version() // e.g., "v1.26.0"
```

##### crypto/rsa — PKCS #1 v1.5 encryption deprecated [NEW in 1.26]

```go
// DEPRECATED: EncryptPKCS1v15, DecryptPKCS1v15, DecryptPKCS1v15SessionKey
// These are unsafe — use OAEP instead
ciphertext, err := rsa.EncryptPKCS1v15(nil, pubKey, msg) // deprecated
plaintext, err  := rsa.DecryptPKCS1v15(nil, privKey, ciphertext) // deprecated

// PREFERRED: OAEP
ciphertext, err = rsa.EncryptOAEP(sha256.New(), nil, pubKey, msg, nil)
plaintext, err  = rsa.DecryptOAEP(sha256.New(), nil, privKey, ciphertext, nil)

// NEW: EncryptOAEPWithOptions — separate hash functions for padding and MGF1
ciphertext, err = rsa.EncryptOAEPWithOptions(pubKey, msg, &rsa.OAEPOptions{
    Hash:    crypto.SHA256,
    MGFHash: crypto.SHA1, // legacy compatibility
    Label:   nil,
})
```

---

#### Testing & Benchmarking

##### T.ArtifactDir, B.ArtifactDir, F.ArtifactDir [NEW in 1.26]

```go
func TestGenerateReport(t *testing.T) {
    // Write test output files (screenshots, logs, fixtures) here
    dir := t.ArtifactDir()
    // With -artifacts flag: dir is persistent (under -outputdir)
    // Without: dir is temp, removed after test

    report := generateReport(testData)
    err := os.WriteFile(filepath.Join(dir, "report.html"), report, 0644)
    if err != nil { t.Fatal(err) }
    // === ARTIFACTS TestGenerateReport /tmp/artifacts/TestGenerateReport/
}
```

```bash
go test -artifacts -outputdir=/tmp/testart ./...
# Artifacts are kept in /tmp/testart/TestXxx/
```

##### B.Loop — inlining fixed [NEW in 1.26]

```go
// In 1.24/1.25, b.Loop() prevented function call inlining in the loop body
// In 1.26, this is FIXED — inlining works normally in b.Loop() bodies
func BenchmarkProcess(b *testing.B) {
    for b.Loop() {
        // Inline-able functions are now inlined — more accurate benchmarks
        result := smallInlineableFunction(data)
        _ = result
    }
}

// Also: variables assigned inside b.Loop() are kept alive (compiler can't elide)
func BenchmarkNoElide(b *testing.B) {
    for b.Loop() {
        x := compute(data) // x is kept alive — benchmark is accurate
        _ = x
    }
}
```

##### testing/synctest (stable since 1.25) + goroutine leak detection

```go
import "testing/synctest"

// Use synctest to detect leaks during tests
func TestNoLeak(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        result := make(chan int, 1) // buffered — no leak
        go func() { result <- compute(42) }()
        synctest.Wait() // all goroutines blocked → goroutines finished
        if <-result != 42 { t.Fatal("wrong result") }
    })
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

Prefer the built-in test lifecycle helpers over manual bookkeeping. All are
available in Go 1.26: `t.Helper()`, `t.TempDir()`, `t.Setenv()`, `t.Cleanup()`,
`t.Context()`/`t.Chdir()` (1.24+), `t.Attr()`/`t.Output()` (1.25+), and
`t.ArtifactDir()` (1.26+).

##### Table-driven tests (1.22+: no shadow needed)

```go
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        got, err := Process(tt.input)
        if err != tt.wantErr { t.Fatalf("err=%v", err) }
        if got != tt.want    { t.Errorf("got %v want %v", got, tt.want) }
    })
}
```

##### go vet — cumulative analysers

| Version | Analyser | What it catches |
|---|---|---|
| 1.22 | `slog` | mismatched key/value pairs |
| 1.23 | `stdversion` | symbols newer than go.mod declares |
| 1.24 | `tests` | malformed test/benchmark names |
| 1.24 | `printf` | `fmt.Printf(s)` with no args |
| 1.25 | `waitgroup` | `wg.Add` inside goroutine |
| 1.25 | `hostport` | `Sprintf("%s:%d")` for network addr |

---

#### CLI Development & Toolchain

##### go fix — modernizer rewrite [NEW in 1.26] ★★

```bash
# go fix is now a full code modernizer built on the go analysis framework
go fix ./...

# What it fixes automatically (dozens of fixers):
# - errors.As → errors.AsType
# - ptr[T](v) helper → new(v)
# - for range b.N → for b.Loop()
# - tools.go pattern → go.mod tool directive
# - deprecated API replacements (e.g., rand.Seed → rand.New(rand.NewPCG(...)))
# - sync.Mutex zero value usage improvements
# - ...and dozens more

# The //go:fix inline directive — inline specific function calls
//go:fix inline
func ptr[T any](v T) *T { return &v }
// go fix will replace all call sites: ptr(42) → new(42)
```

##### go.mod init defaults to N-1

```bash
# With 1.26 toolchain:
go mod init myapp   # go.mod says "go 1.25.0" (not 1.26.0)
go get go@1.26.0    # upgrade if you need 1.26 features specifically
```

##### Tool directive in go.mod (1.24+)

```bash
go get -tool golang.org/x/tools/cmd/stringer@latest
go tool stringer -type=Color .
```

##### flag / Cobra

```go
host    := flag.String("host", "localhost", "server host")
port    := flag.Int("port", 8080, "server port")
flag.Parse()
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

##### Green Tea GC — now default [NEW in 1.26] ★★★

```bash
# Enabled by default — no action needed
# Expect 10–40% reduction in GC overhead for GC-heavy programs
# Additional ~10% on amd64 CPUs with AVX-512 (Intel Ice Lake, AMD Zen 4+)

# Revert to old GC (expected to be removed in 1.27):
GOEXPERIMENT=nogreenteagc go build ./...

# If you see GC regressions, file an issue and use the above temporarily
```

##### cgo calls — 30% faster [NEW in 1.26]

```bash
# No code changes needed
# If your app calls cgo (SQLite, OpenGL, OS APIs): transparent ~30% improvement
# The `syscall` state has been removed from the scheduler's processor state machine
# Effect: lower overhead for all cgo and syscall transitions
```

##### Size-specialised malloc — up to 30% faster small allocs [NEW in 1.26]

```bash
# Enabled by default
# Small allocations (under ~80 bytes): up to 30% faster
# Overall improvement: ~1% for allocation-heavy production workloads
# Cost: ~60 KB larger binary (workload-independent)

# Disable:
GOEXPERIMENT=nosizespecializedmalloc go build ./...
```

##### Heap base address randomisation [NEW in 1.26]

```bash
# On 64-bit platforms: heap base is now randomised at startup (ASLR for heap)
# Security improvement for cgo-heavy programs
# If it causes issues (rare):
GOEXPERIMENT=norandomizedheapbase64 go build ./...
```

##### Slice stack allocation — improved again [NEW in 1.26]

Compiler allocates slice backing stores on the stack in more situations than 1.25. Automatic, no code changes.

##### image/jpeg — faster + more accurate [NEW in 1.26]

```go
// JPEG encoder and decoder replaced with faster, more accurate implementations
// If your code depends on bit-exact JPEG output, results may differ
// No code changes needed for normal use
data, err := os.ReadFile("photo.jpg")
img, _, err := image.Decode(bytes.NewReader(data)) // faster, more accurate
```

##### PGO (GA since 1.21)

```bash
go build -o myapp ./cmd/myapp
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cmd/myapp/default.pgo
go build ./cmd/myapp
```

##### Container-aware GOMAXPROCS (1.25+)

```bash
# Automatic — reads cgroup CPU bandwidth
# Remove automaxprocs library when upgrading to 1.25+
runtime.GOMAXPROCS(0) // query current value
runtime.SetDefaultGOMAXPROCS() // restore to runtime default
```

---

#### Observability: Tracing, Profiling & Goroutine Leaks

##### pprof flame graph is now the default view [NEW in 1.26]

```bash
go tool pprof -http=:6060 cpu.prof
# Opens directly to flame graph (was call graph before)
# Graph view still available: View -> Graph or /ui/graph
```

##### Goroutine leak profile (experimental) [NEW in 1.26]

```bash
GOEXPERIMENT=goroutineleakprofile go build ./cmd/myapp

# HTTP endpoint
curl http://localhost:6060/debug/pprof/goroutineleak
```

##### New goroutine metrics [NEW in 1.26]

```go
import "runtime/metrics"

samples := []metrics.Sample{
    {Name: "/sched/goroutines-created:goroutines"},
    {Name: "/sched/goroutines/runnable:goroutines"},
    {Name: "/sched/goroutines/running:goroutines"},
    {Name: "/sched/goroutines/waiting:goroutines"},
    {Name: "/sched/goroutines/not-in-go:goroutines"},
    {Name: "/sched/threads:threads"},
}
metrics.Read(samples)
for _, s := range samples {
    fmt.Printf("%s = %v\n", s.Name, s.Value.Uint64())
}

// Use in dashboards:
// waiting ↑ → lock contention
// not-in-go ↑ → goroutines stuck in syscalls/cgo
// runnable ↑ → CPUs can't keep up with demand
```

##### FlightRecorder (1.25+)

```go
fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{ // returns *FlightRecorder (no error)
    MinAge: 5 * time.Second, MaxBytes: 10 * 1 << 20,
})
if err := fr.Start(); err != nil { log.Fatal(err) }
defer fr.Stop()
// On anomaly: fr.WriteTo(outFile)
```

---

#### Idioms & Things to Avoid

##### Do (1.26 additions)

- **Use `new(expr)`** for pointer-to-value: `new(42)`, `new(true)`, `new("hello")`. Eliminates all `ptr[T](v)` helpers.
- **Use `errors.AsType[T](err)`** instead of `errors.As` for all new error type checks — faster, type-safe, scoped.
- **Use `slog.NewMultiHandler`** to fan out logs to multiple backends.
- **Use `crypto/hpke`** for all new public-key encryption — HPKE is the correct modern approach.
- **Pass `nil` as the rand parameter** to `crypto/ecdsa`, `crypto/rsa`, `crypto/rand` functions — it's now the preferred pattern.
- **Use `testing/cryptotest.SetGlobalRandom(t, seed)`** for deterministic crypto tests.
- **Use `errors.AsType`** instead of `errors.As` — the generic version is faster, cleaner, and catches misuse at compile time.
- **Run `go fix ./...`** after upgrading — the modernizer will auto-apply dozens of idiom updates.
- **Use `//go:fix inline`** on your `ptr[T]` helpers — `go fix` will replace call sites with `new(expr)` automatically.
- **Enable `GOEXPERIMENT=goroutineleakprofile`** in production staging — goroutine leak detection is production-ready.
- **Set `t.ArtifactDir()`** for test output files (reports, screenshots) instead of `os.TempDir()`.
- **Add `go test -artifacts`** to CI pipelines that generate test artifacts.
- **Use `Rewrite` instead of `Director`** in `httputil.ReverseProxy` — `Director` is now deprecated.
- **Expect `io.ReadAll` to be 2x faster** — no code change needed.
- **Watch `asynctimerchan`** GODEBUG removal: 1.27 removes it — unbuffered timer channels become permanent.

##### Don't

| Anti-pattern | Why | Instead |
|---|---|---|
| `ptr[T](v)` helper | Obsolete since 1.26 | `new(v)` |
| `var ve *T; errors.As(err, &ve)` | Verbose, leaks var scope, slower | `errors.AsType[*T](err)` |
| `ecdsa.GenerateKey(curve, rand.Reader)` | rand param now ignored | `ecdsa.GenerateKey(curve, nil)` |
| `rsa.EncryptPKCS1v15` | Deprecated, unsafe | `rsa.EncryptOAEP` |
| `Director` in `ReverseProxy` | Deprecated — header injection risk | `ReverseProxy.Rewrite` |
| `url.Parse("http://::1/")` | Now returns error | Fix malformed URL |
| `fmt.Sprintf("%s:%d", h, p)` | IPv6 unsafe | `net.JoinHostPort(h, strconv.Itoa(p))` |
| Using result before checking error | Nil bug fixed in 1.25 | Check `err != nil` first |
| `wg.Add(1)` inside goroutine | Race with Wait | `wg.Go(f)` |
| Manual GOMAXPROCS in containers | Built-in since 1.25 | Remove; use `SetDefaultGOMAXPROCS()` |
| `runtime.SetFinalizer` for new code | One-per-object, cycle issues | `runtime.AddCleanup` |
| `tools.go` blank import | Obsolete since 1.24 | `go.mod tool` directive |
| `for range b.N` in benchmarks | Old style | `for b.Loop()` |
| `time.Sleep` in tests | Non-deterministic | `testing/synctest` |
| `strings.Split` for large inputs | Full allocation | `strings.SplitSeq` |
| `rand.Seed()` | No-op since 1.24 | `rand.New(rand.NewPCG(s1, s2))` |
| `var m map[string]int` | Nil, writes panic | `make(map[string]int)` |
| String `+` in loops | O(n²) | `strings.Builder` |
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
type httpClient struct{}          // camelCase unexported
type HTTPClient struct{}          // acronyms all-caps
var ErrNotFound = errors.New("not found")
type Stringer interface{ String() string }
func (u *User) IsAdmin() bool {}
```

##### Formatting & linting

```bash
gofmt -w .
goimports -w .
go fix ./...   # 1.26+: runs all modernizers automatically
go vet ./...
staticcheck ./...
golangci-lint run
```

A `golangci-lint` v2 configuration (the tool version is independent of the Go
language version and lints Go 1.26 code fine; pin and run it via a `tool`
directive with `go tool golangci-lint run`):

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

*End of Go 1.26 Complete Developer Guideline*
