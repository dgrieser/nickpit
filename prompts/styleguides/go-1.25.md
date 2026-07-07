### Go 1.25 — Complete Developer Guideline

> **Version**: Go 1.25.0 (released 2025-08-12)
> **Scope**: Full language spec, all 1.25 changes delta over 1.24, idioms, concurrency, performance, CLI, file I/O, testing, and best practices — with examples throughout.
> **Note**: This document is self-contained. All relevant prior-version material is carried forward or superseded. Sections marked **[NEW in 1.25]** cover additions specific to this version.

---

#### Table of Contents

1. [What's New in Go 1.25](#whats-new-in-go-125)
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
17. [Memory Management: Cleanup, Weak & unique](#memory-management-cleanup-weak--unique)
18. [Context Package](#context-package)
19. [File I/O & Filesystem](#file-io--filesystem)
20. [JSON — v1 and experimental v2](#json--v1-and-experimental-v2)
21. [HTTP Servers](#http-servers)
22. [Cryptography](#cryptography)
23. [Testing & Benchmarking](#testing--benchmarking)
24. [CLI Development & Tool Directive](#cli-development--tool-directive)
25. [Performance Caveats & PGO](#performance-caveats--pgo)
26. [Observability: Tracing & Profiling](#observability-tracing--profiling)
27. [Idioms & Things to Avoid](#idioms--things-to-avoid)

---

#### What's New in Go 1.25

Go 1.25.0 shipped 2025-08-12. There are **no language syntax changes** — Go 1.25 is a toolchain/runtime/stdlib release. The headline items are: **container-aware `GOMAXPROCS`** (transparent Kubernetes/container CPU limit respect), an experimental **Green Tea garbage collector** (10–40% GC overhead reduction), the now-stable **`testing/synctest`** package, an experimental **`encoding/json/v2`**, DWARF 5 debug info (faster linking, smaller binaries), the expanded **`os.Root`** API, and the new **`sync.WaitGroup.Go`** convenience method.

##### Language specification

No new syntax. The notion of **core types** is removed from the specification in favour of dedicated prose. This is a spec cleanup only — no programs need changes. The `~` constraint syntax and all existing generic code continues to work unchanged.

##### Important compiler bug fix [NEW in 1.25] ★★

A nil pointer bug, present since **Go 1.21 through 1.24**, has been fixed. The compiler previously incorrectly delayed nil checks past error checks:

```go
// PREVIOUSLY (Go 1.21–1.24): THIS WOULD NOT PANIC — incorrect behaviour
f, err := os.Open("nonExistentFile")
name := f.Name()  // f is nil when err != nil, but old compiler delayed the nil check
if err != nil {
    return
}
println(name)

// Go 1.25: CORRECTLY PANICS with nil pointer dereference on f.Name()
// FIX: always check err before using the result
f, err := os.Open("nonExistentFile")
if err != nil {
    return
}
name := f.Name()  // safe: f is guaranteed non-nil here
println(name)
```

**Action required if upgrading**: any code that used the result of a function before checking its error return may now panic where it previously did not. Run `go test ./...` after upgrading.

---

#### Tools

##### go doc -http [NEW in 1.25]

```bash
# Start a local documentation server and open the browser
go doc -http fmt.Println
go doc -http net/http.Server
go doc -http .              # docs for current package
# Opens http://localhost:PORT/pkg/... in the default browser
```

##### go version -m -json [NEW in 1.25]

```bash
# Print build info as JSON (was plain text before)
go version -m -json ./myapp
# Output: {"Path":"...","Main":{...},"Deps":[...],"Settings":[...]}
```

##### go.mod ignore directive [NEW in 1.25]

```
module myapp

go 1.25.0

ignore (
    ./vendor
    ./generated
    ./testdata/large
)
```

The `ignore` directive tells the `go` command to skip those directories and their subdirectories when matching patterns like `all` or `./...`. Files are still included in module zip files.

```bash
# These commands now skip ./generated:
go build ./...
go test all
go vet ./...
```

##### work package pattern [NEW in 1.25]

```bash
# In workspace mode: matches all packages in all workspace modules
go build work
go test work
go vet work

# Previously you had to use ./... in each module
```

##### go vet — new analysers [NEW in 1.25]

###### waitgroup — detects misplaced WaitGroup.Add

```go
// WRONG: Add called inside the goroutine — race between Add and Wait
var wg sync.WaitGroup
go func() {
    wg.Add(1)  // vet: waitgroup: sync.WaitGroup.Add called from within goroutine
    defer wg.Done()
    doWork()
}()
wg.Wait()

// CORRECT: Add called before launching goroutine
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    doWork()
}()
wg.Wait()

// Also caught: WaitGroup passed by value (copies miss the Add)
func launch(wg sync.WaitGroup) {  // vet: passes lock by value
    wg.Add(1)
    go func() { wg.Done() }()
}
```

###### hostport — IPv6-incompatible address construction

```go
// WRONG: IPv6 addresses contain colons — this produces "::1:8080" which is invalid
addr := fmt.Sprintf("%s:%d", host, port)  // vet: use net.JoinHostPort
conn, _ := net.Dial("tcp", addr)

// CORRECT: net.JoinHostPort handles IPv6 with square brackets
addr := net.JoinHostPort(host, strconv.Itoa(port))
// IPv4:  "192.168.1.1:8080"
// IPv6:  "[::1]:8080"

// Also correct for string port
addr2 := net.JoinHostPort(host, "8080")
```

##### go build -asan — leak detection [NEW in 1.25]

```bash
# Now defaults to C memory leak detection at program exit
go build -asan ./cmd/myapp
./myapp  # will report C memory leaks on exit

# Disable leak detection if it produces false positives with certain C libraries
ASAN_OPTIONS=detect_leaks=0 ./myapp
```

##### DWARF 5 [NEW in 1.25]

The compiler and linker now emit DWARF version 5 debug info by default. This reduces binary size (debugging section) and speeds up linking, especially for large programs. No action required.

```bash
# Revert to DWARF 4 if tooling doesn't support DWARF 5 yet
GOEXPERIMENT=nodwarf5 go build ./...
```

##### toolchain line no longer auto-added

```bash
# Before 1.25: go mod tidy / go get would add:
# toolchain go1.24.2
# to go.mod

# In 1.25: the toolchain line is NOT automatically added when updating the go line
# Existing toolchain lines are preserved; new ones not injected
```

---

#### Project Layout & Modules

```
myapp/
├── cmd/
│   └── myapp/
│       ├── main.go
│       └── default.pgo
├── internal/
├── generated/        # can be ignored via go.mod ignore
├── vendor/
├── go.mod
└── go.sum
```

##### go.mod with 1.25 features

```
module github.com/yourname/myapp

go 1.25.0

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

godebug asynctimerchan=0
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
m := min(x, 20)   // 1.21+
M := max(x, 20)   // 1.21+

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

#### Pointers & Weak Pointers

##### Strong pointers

```go
x := 42; p := &x; *p = 99
p1 := new(int)
p2 := &Point{X: 1, Y: 2}
```

##### Weak pointers (1.24+)

```go
import "weak"
strong := &ExpensiveObject{data: make([]byte, 1<<20)}
wp := weak.Make(strong)

if obj := wp.Value(); obj != nil { use(obj) }

strong = nil
runtime.GC()
fmt.Println(wp.Value() == nil) // true — GC'd
```

---

#### Control Flow

##### if / else

```go
if x > 0 { fmt.Println("positive") }
if err := doSomething(); err != nil { return err }
f, err := os.Open(name)
if err != nil { return err }  // check BEFORE using f
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

##### switch

```go
switch x {
case 1:    fmt.Println("one")
case 2, 3: fmt.Println("two or three")
default:   fmt.Println("other")
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
defer f.Close()
defer func() { log.Println(time.Since(start)) }()  // time in closure

defer func() {
    if r := recover(); r != nil { /* panic occurred */ }
}()
```

##### Repanic output improved [NEW in 1.25]

```go
// Before 1.25: confusing repeated message
// panic: PANIC [recovered]
//     panic: PANIC

// After 1.25: clear single line
// panic: PANIC [recovered, repanicked]
```

##### Defer inside iterator loops

```go
// WRONG: all defers run at function return
for path := range filePaths() {
    f, _ := os.Open(path)
    defer f.Close()  // accumulates!
}

// CORRECT: wrap each iteration
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
type User struct { ID int; Name, Email string; admin bool }
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

##### Functional options

```go
type Option func(*Server)
func WithTimeout(d time.Duration) Option { return func(s *Server) { s.timeout = d } }
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

##### New hash interfaces [NEW in 1.25]

```go
// hash.Cloner — implemented by ALL standard library hash.Hash implementations
type Cloner interface {
    Clone() hash.Hash
}

// Clone mid-hash to compute multiple digests from the same prefix
h := sha256.New()
h.Write(prefix)
h1 := h.(hash.Cloner).Clone() // snapshot state
h2 := h.(hash.Cloner).Clone()
h1.Write(suffix1)
h2.Write(suffix2)
d1 := h1.Sum(nil)
d2 := h2.Sum(nil)

// hash.XOF — extendable output functions (SHAKE etc.)
type XOF interface {
    hash.Hash
    Read(p []byte) (n int, err error)
    Reset()
}
```

##### io/fs.ReadLinkFS [NEW in 1.25]

```go
// New interface for filesystems that can read symlink targets
type ReadLinkFS interface {
    fs.FS
    ReadLink(name string) (string, error)
}

// os.DirFS and os.Root.FS now implement ReadLinkFS
fsys := os.DirFS("./src")
if rlfs, ok := fsys.(fs.ReadLinkFS); ok {
    target, err := rlfs.ReadLink("mylink")
}

// archive/tar Writer.AddFS now copies symlinks when FS implements ReadLinkFS
// os.CopyFS now copies symlinks when FS implements ReadLinkFS
```

##### crypto.MessageSigner [NEW in 1.25]

```go
// New signing interface for "one-shot" signers that hash internally
type MessageSigner interface {
    crypto.Signer
    SignMessage(rand io.Reader, msg []byte, opts crypto.SignerOpts) ([]byte, error)
}

// Use crypto.SignMessage to transparently support both Signer and MessageSigner
sig, err := crypto.SignMessage(key, rand.Reader, message, opts)
// Falls back to Signer.Sign if MessageSigner not implemented

// x509.CreateCertificate now accepts MessageSigner
cert, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pubKey, messageSigner)
```

---

#### Generics & Type Aliases

##### Generic type aliases (stable since 1.24)

```go
type Seq[V any] = func(yield func(V) bool)
type StringMap[V any] = map[string]V
type UserRepo = storage.Repository[User]
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

import "cmp"
host := cmp.Or(os.Getenv("HOST"), cfg.Host, "localhost")
```

---

#### Iterators — iter package

Stable since 1.23. Full detail in that guide; key summary:

```go
import "iter"

type Seq[V any]    = func(yield func(V) bool)
type Seq2[K, V any] = func(yield func(K, V) bool)

func Evens(n int) iter.Seq[int] {
    return func(yield func(int) bool) {
        for i := 0; i < n; i += 2 {
            if !yield(i) { return }
        }
    }
}
for v := range Evens(10) { fmt.Println(v) }

// Pull conversion
next, stop := iter.Pull(mySeq); defer stop()
for { v, ok := next(); if !ok { break }; process(v) }
```

##### strings and bytes iterator functions (1.24+)

```go
for line := range strings.Lines("hello\nworld\n") { fmt.Println(line) }
for part := range strings.SplitSeq("a,b,c", ",") { fmt.Println(part) }
for word := range strings.FieldsSeq("hello world") { fmt.Println(word) }
// Same functions in bytes package for []byte
```

---

#### Collection Types: Arrays, Slices, Maps

##### Slices — faster stack allocation [NEW in 1.25]

The compiler now allocates slice backing stores on the stack in more situations. This reduces heap allocations and GC pressure automatically. No code changes needed.

**Caveat**: if you use `unsafe.Pointer` on slice internals incorrectly, this optimisation may surface the bug (previously hidden by heap allocation). Use `go build -gcflags=all=-d=variablemakehash=n` to disable if you need to debug.

```go
// These may now be stack-allocated (transparent improvement):
func f() []int {
    s := make([]int, 10)  // may go on stack now
    for i := range s { s[i] = i }
    return s  // still heap-escaped if returned
}
```

##### slices package (1.21+, includes 1.23 iterator additions)

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

##### maps package (1.21+, includes 1.23 iterator additions)

```go
import "maps"
maps.Clone(m); maps.Copy(dst, src)
maps.DeleteFunc(m, fn); maps.Equal(m1, m2)
maps.All(m); maps.Keys(m); maps.Values(m)
maps.Insert(m, seq); maps.Collect(seq)
```

##### Maps use Swiss Tables (1.24+)

All maps use Swiss Tables automatically — 30%+ faster for large map access, 10–60% faster iteration. No code changes needed.

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

```go
s := "héllo"
fmt.Println(len(s))         // 6 bytes
fmt.Println(len([]rune(s))) // 5 runes

var b strings.Builder
b.Grow(1000); for range 1000 { b.WriteByte('x') }
result := b.String()

strings.Contains("foobar", "bar")
strings.HasPrefix("foobar", "foo")
strings.Split("a,b,c", ",")          // []string — full allocation
strings.SplitSeq("a,b,c", ",")       // 1.24+: lazy iter.Seq[string]
strings.Lines("line1\nline2\n")      // 1.24+: lazy iter.Seq[string]
strings.FieldsSeq("hello world")     // 1.24+: lazy iter.Seq[string]
strings.Join([]string{"a","b"}, "-")
strings.TrimSpace("  hello  ")
strings.ReplaceAll("aabbcc", "b", "x")
after, found  := strings.CutPrefix("Gopher", "Go")
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
slog.InfoContext(ctx, "processing", "id", id)

// Discard in tests/benchmarks (1.24+)
slog.SetDefault(slog.New(slog.DiscardHandler{}))
```

##### slog.GroupAttrs [NEW in 1.25]

```go
// GroupAttrs creates a group Attr from a slice of Attrs
attrs := []slog.Attr{
    slog.String("host", "db.example.com"),
    slog.Int("port", 5432),
    slog.Duration("latency", 12*time.Millisecond),
}
slog.Info("db query", slog.GroupAttrs("db", attrs...))
// Output: {"db":{"host":"db.example.com","port":5432,"latency":"12ms"}}

// Record.Source() — access source location from custom handlers
type MyHandler struct{ inner slog.Handler }
func (h MyHandler) Handle(ctx context.Context, r slog.Record) error {
    src := r.Source() // non-nil if AddSource: true
    if src != nil {
        fmt.Println(src.File, src.Line)
    }
    return h.inner.Handle(ctx, r)
}
```

##### slog best practices

- **Always use structured key-value pairs** — never format strings into the message.
- **Keys should be lowercase snake_case** for consistent log querying.
- **Pass `context.Context` through** and use `InfoContext`/`ErrorContext` for trace IDs.
- **Group related attributes** with `slog.Group`/`slog.GroupAttrs` or `logger.WithGroup`.
- **Never log secrets, tokens, credentials, or full request/response bodies** unless they are explicitly scrubbed.

---

#### Concurrency

##### sync.WaitGroup.Go [NEW in 1.25] ★

Combines `Add(1)` + `go func()` into a single call. Eliminates the classic Add-before-goroutine pattern.

```go
// Old pattern — verbose and error-prone (Add must be before go)
var wg sync.WaitGroup
for _, item := range items {
    wg.Add(1)
    go func(item Item) {
        defer wg.Done()
        process(item)
    }(item)
}
wg.Wait()

// NEW in 1.25 — concise and correct
var wg sync.WaitGroup
for _, item := range items {
    wg.Go(func() { process(item) })  // Add(1) + go + defer Done, all-in-one
}
wg.Wait()

// With error collection
var (
    wg   sync.WaitGroup
    mu   sync.Mutex
    errs []error
)
for _, item := range items {
    wg.Go(func() {
        if err := process(item); err != nil {
            mu.Lock(); errs = append(errs, err); mu.Unlock()
        }
    })
}
wg.Wait()
```

**Note**: `wg.Go(f)` calls `wg.Add(1)` and then `go func() { defer wg.Done(); f() }()`. The closure captures variables from the outer scope (1.22+: loop variables are per-iteration, so capture is safe).

##### Channels

```go
ch  := make(chan int)        // unbuffered
bch := make(chan string, 10) // buffered

go func() { ch <- 42 }()
val := <-ch; close(ch)
for v := range ch { fmt.Println(v) }

// Timers: unbuffered since 1.23
t := time.NewTimer(5 * time.Second); defer t.Stop()
select {
case <-t.C: fmt.Println("fired")
default:    fmt.Println("not yet")
}
t.Reset(10 * time.Second) // safe in 1.23+ modules
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

##### sync.Map (concurrent hash-trie since 1.24)

```go
var m sync.Map
m.Store("k", "v"); v, ok := m.Load("k")
m.Delete("k"); m.Clear() // 1.23+
m.Range(func(k, v any) bool { return true })
```

##### sync/atomic

```go
var counter atomic.Int64; counter.Add(1)
var ptr atomic.Pointer[Config]; ptr.Store(newConfig)
var flags atomic.Uint32
flags.Or(0b0100)           // 1.23+: set bit
flags.And(^uint32(0b0100)) // 1.23+: clear bit
```

##### sync.Once helpers (1.21+)

```go
initDB := sync.OnceFunc(func() { db = openDB() })
getConfig := sync.OnceValue(func() *Config { return loadConfig() })
getConn := sync.OnceValues(func() (*sql.DB, error) { return sql.Open("postgres", dsn) })
```

##### testing/synctest — now stable [NEW in 1.25] ★★

Was experimental under `GOEXPERIMENT=synctest` in 1.24. Now a stable standard library package.

```go
import "testing/synctest"

func TestDebounce(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        calls := 0
        fn := Debounce(func() { calls++ }, 100*time.Millisecond)

        fn(); fn(); fn()
        synctest.Wait() // all goroutines blocked

        // Advance fake clock — no real sleep
        time.Sleep(100 * time.Millisecond)
        synctest.Wait()

        if calls != 1 {
            t.Errorf("expected 1 call, got %d", calls)
        }
    })
}

func TestRateLimiter(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        rl := NewRateLimiter(10, time.Second)

        for range 10 { rl.Allow() }
        if rl.Allow() { t.Fatal("should be rate limited") }

        time.Sleep(time.Second) // instant in synctest bubble
        synctest.Wait()

        if !rl.Allow() { t.Fatal("should be allowed after 1s") }
    })
}
```

**Key properties:**
- Within a `synctest.Test` bubble: all `time` package functions use a fake clock.
- Fake clock advances instantaneously when all goroutines in the bubble are blocked.
- `synctest.Wait()` blocks until all goroutines in the bubble are blocked.
- `panic`, `return`, channel sends/receives work normally.
- The stable entry point is `synctest.Test(t, func(t *testing.T){ ... })`. The experimental `synctest.Run(func())` API (from `GOEXPERIMENT=synctest` in 1.24) still works in 1.25 but is deprecated and will be removed in Go 1.26 — prefer `synctest.Test` in new code.

##### Concurrency pitfalls (updated for 1.25)

| Pitfall | Fix |
|---|---|
| `wg.Add(1)` inside goroutine | `wg.Go(f)` or `wg.Add(1)` before `go` |
| `fmt.Sprintf("%s:%d", h, p)` | `net.JoinHostPort(h, strconv.Itoa(p))` |
| Using result before checking err | Check `err != nil` first (nil bug fixed in 1.25) |
| Goroutine leak | Context cancellation |
| Race condition | `go test -race ./...` |
| Timer `len(t.C) > 0` | `select { case <-t.C: default: }` |
| Loop variable capture (< 1.22) | Upgrade to go 1.22+ |

##### Classifying races before reporting

Separate true data races from lifecycle, shutdown, or ordering races. Only call
something a data race after confirming unsynchronized shared-memory access with
at least one write.

Check a library's concurrency contract before assuming concurrent method calls
are unsafe. Some Go types are explicitly safe for concurrent use, while others
require caller-side synchronization.

---

#### Memory Management: Cleanup, Weak & unique

##### runtime.AddCleanup (1.24+) — parallel in 1.25

```go
// In 1.25: cleanup functions now run CONCURRENTLY and in PARALLEL
// (previously ran sequentially)
// This makes cleanups viable for heavy use (e.g., unique package internals)
runtime.AddCleanup(obj, func(id int) {
    // Runs concurrently with other cleanups
    // If your cleanup must run long or block, shunt to a goroutine:
    go func() { expensiveCleanup(id) }()
}, obj.id)

// Debug cleanups with new GODEBUG setting:
// GODEBUG=checkfinalizers=1 ./myapp
// Reports: cleanup/finalizer queue lengths to stderr each GC cycle
// Flags: common misuse patterns
```

##### Weak pointer cache (1.24+)

```go
type Cache[K comparable, V any] struct {
    mu sync.Mutex
    m  map[K]weak.Pointer[V]
}

func (c *Cache[K, V]) GetOrLoad(key K, load func(K) *V) *V {
    c.mu.Lock(); defer c.mu.Unlock()
    if wp, ok := c.m[key]; ok {
        if v := wp.Value(); v != nil { return v }
    }
    v := load(key)
    c.m[key] = weak.Make(v)
    runtime.AddCleanup(v, func(k K) {
        c.mu.Lock(); defer c.mu.Unlock()
        if existing, ok := c.m[k]; ok && existing.Value() == nil {
            delete(c.m, k)
        }
    }, key)
    return v
}
```

##### unique package — improved in 1.25

```go
// unique.Make now reclaims values MORE EAGERLY and IN PARALLEL (1.25+)
// Previously: interning truly unique values could cause memory blow-up
// Now: GC reclaims unused interned values promptly in a single cycle

h1 := unique.Make("hello")
h2 := unique.Make("hello")
fmt.Println(h1 == h2)      // true — canonical handle
fmt.Println(h1.Value())    // "hello"
```

##### runtime.SetDefaultGOMAXPROCS [NEW in 1.25]

```go
// If GOMAXPROCS was set manually but you later want to restore
// the runtime default (including container-aware behaviour):
runtime.SetDefaultGOMAXPROCS()
// Equivalent to: unset GOMAXPROCS env var + container-aware logic
```

---

#### Context Package

```go
ctx := context.Background()
ctx, cancel := context.WithCancel(parent); defer cancel()
ctx, cancel  = context.WithTimeout(parent, 5*time.Second); defer cancel()
ctx          = context.WithValue(parent, key{}, value)
ctx, cancel  = context.WithCancelCause(parent); defer cancel(nil)
detached     := context.WithoutCancel(requestCtx)
ctx, cancel  = context.WithTimeoutCause(parent, 5*time.Second, reason)
stop         := context.AfterFunc(ctx, cleanup)
```

**Never store context in a struct. Never pass nil. Always `defer cancel()`.**

---

#### File I/O & Filesystem

##### os.Root — expanded API [NEW in 1.25] ★

`os.Root` gained many new methods in 1.25, making it a full filesystem API within a sandbox.

```go
root, err := os.OpenRoot("./uploads")
if err != nil { return err }
defer root.Close()

// Original 1.24 methods
f, err := root.Open("user/file.txt")
f2, _ := root.Create("user/new.txt")
root.Mkdir("newdir", 0755)
root.Stat("file.txt"); root.ReadDir("subdir")
root.Remove("old.txt")

// NEW in 1.25: full filesystem operations within the sandbox
root.ReadFile("config.json")                          // read entire file
root.WriteFile("out.json", data, 0644)                // write entire file
root.MkdirAll("a/b/c", 0755)                          // mkdir -p
root.RemoveAll("old_dir")                             // rm -rf
root.Rename("old.txt", "new.txt")                     // rename
root.Symlink("target", "link")                        // create symlink
root.Readlink("link")                                 // read symlink
root.Link("existing", "hardlink")                     // hard link
root.Chmod("file.txt", 0644)                          // chmod
root.Chown("file.txt", uid, gid)                      // chown
root.Lchown("link", uid, gid)                         // lchown (symlink)
root.Chtimes("file.txt", atime, mtime)                // timestamps
root.FS()                                             // io/fs.FS + ReadLinkFS

// Complete sandboxed HTTP file server
root, _ := os.OpenRoot("./public")
http.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
    path := r.URL.Path[len("/files/"):]
    f, err := root.Open(path)
    if err != nil { http.NotFound(w, r); return }
    defer f.Close()
    st, _ := f.Stat()
    http.ServeContent(w, r, st.Name(), st.ModTime(), f.(io.ReadSeeker))
})
```

##### ReadLinkFS [NEW in 1.25]

```go
// os.DirFS and os.Root.FS now implement io/fs.ReadLinkFS
fsys := os.DirFS("./src")
rl := fsys.(fs.ReadLinkFS)
target, err := rl.ReadLink("mylink") // read symlink target

// os.CopyFS now copies symlinks (not just dereferences them)
err = os.CopyFS("./dist", os.DirFS("./src")) // symlinks preserved
```

##### Standard I/O

```go
f, err := os.Open("file.txt")
if err != nil { return err }
defer f.Close()

scanner := bufio.NewScanner(f)
scanner.Buffer(make([]byte, 1<<20), 1<<20)
for scanner.Scan() { process(scanner.Text()) }
if err := scanner.Err(); err != nil { return err }

io.Copy(dst, src)
data, err := os.ReadFile("small.json")
```

---

#### JSON — v1 and experimental v2

##### JSON v1 (encoding/json) — unchanged API

```go
enc := json.NewEncoder(w); enc.SetIndent("", "  "); enc.Encode(data)
dec := json.NewDecoder(r); dec.Decode(&result)

type User struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Pass      string    `json:"-"`
    Age       int       `json:"age,omitempty"`
    CreatedAt time.Time `json:"created_at,omitzero"`  // 1.24+
}
```

##### encoding/json/v2 — experimental [NEW in 1.25]

Enable with `GOEXPERIMENT=jsonv2` at build time.

```go
// When jsonv2 GOEXPERIMENT is enabled:
// - encoding/json uses the new implementation (same API, error messages differ)
// - encoding/json/v2 is available with the new API
// - encoding/json/jsontext is available for low-level JSON manipulation

import "encoding/json/v2"
import "encoding/json/jsontext"

// v2 — key improvements over v1:
// 1. Decoding is substantially faster
// 2. Case-sensitive struct field matching by default
// 3. No silent data loss on unknown fields (controlled by options)
// 4. omitzero built-in
// 5. Better error messages

// Marshal
data, err := jsonv2.Marshal(user)

// Unmarshal — faster, stricter
err = jsonv2.Unmarshal(data, &user)

// Options — control behaviour
err = jsonv2.Unmarshal(data, &user,
    jsonv2.RejectUnknownMembers(true),   // error on unknown fields
    jsonv2.WithLeniency(true),            // relax some constraints
)

// jsontext — low-level token-by-token processing
dec := jsontext.NewDecoder(r)
for {
    tok, err := dec.ReadToken()
    if err == io.EOF { break }
    if err != nil { return err }
    fmt.Println(tok.Kind(), tok)  // '{', 's' (string), 'n' (number), etc.
}
```

**Try it in tests to identify compatibility issues before v2 becomes default:**
```bash
GOEXPERIMENT=jsonv2 go test ./...
```

---

#### HTTP Servers

##### CrossOriginProtection [NEW in 1.25] ★

```go
// CSRF protection using Fetch metadata — no tokens needed
import "net/http"

cop := http.NewCrossOriginProtection()
// Configure allowed origins
cop.AddAllowedOrigin("https://app.example.com")
cop.AddSafeOrigin("https://trusted-partner.com")
// Pattern-based bypass
cop.AddPermittedPattern("/api/webhooks/") // webhooks don't have Fetch metadata

handler := cop.Handler(mux) // wrap your existing handler
http.ListenAndServe(":8080", handler)
```

Rejects cross-origin non-safe browser requests using `Sec-Fetch-Site` and `Origin` headers. Works with modern browsers; falls back gracefully for non-browser clients.

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

##### HTTP protocol configuration (1.24+)

```go
srv := &http.Server{
    Addr:    ":8080",
    Handler: cop.Handler(mux),
    Protocols: &http.Protocols{HTTP1: true, HTTP2: true},
}
```

---

#### Cryptography

##### crypto/ecdsa — new low-level API [NEW in 1.25]

```go
import "crypto/ecdsa"

// New: low-level raw encoding (no need for crypto/elliptic or math/big)
privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

// Serialize private key
rawBytes, err := privKey.Bytes() // 32 bytes for P-256

// Deserialize
restoredKey, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), rawBytes)

// Public key
pubBytes, err := privKey.PublicKey.Bytes() // uncompressed: 65 bytes

// Parse public key
restoredPub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), pubBytes)
```

##### TLS — SHA-1 signatures disallowed [NEW in 1.25]

```go
// SHA-1 signature algorithms now rejected in TLS 1.2 handshakes (RFC 9155)
// Re-enable only for legacy compatibility:
// GODEBUG=tlssha1=1 ./myapp

// TLS 1.2 now requires Extended Master Secret in FIPS mode
// TLS servers prefer highest supported version

cfg := &tls.Config{
    MinVersion: tls.VersionTLS12,
    // GetEncryptedClientHelloKeys: new callback (1.25, replaces static field)
    GetEncryptedClientHelloKeys: func(hello *tls.ClientHelloInfo) ([]tls.EncryptedClientHelloKey, error) {
        return loadECHKeys()
    },
}
```

##### crypto/rsa — 3x faster key generation [NEW in 1.25]

```go
// Key generation is now 3x faster — no code changes needed
privKey, err := rsa.GenerateKey(rand.Reader, 2048) // ~3x faster than 1.24

// Note: PublicKey no longer claims modulus is treated as secret
// (was already public knowledge — mathematical attacks can recover it)
```

##### hash.Cloner — clone mid-computation [NEW in 1.25]

```go
// All standard library hash.Hash implementations now implement hash.Cloner
h := sha256.New()
h.Write(commonPrefix)

// Branch 1
h1 := h.(hash.Cloner).Clone()
h1.Write(suffix1)
digest1 := h1.Sum(nil)

// Branch 2
h2 := h.(hash.Cloner).Clone()
h2.Write(suffix2)
digest2 := h2.Sum(nil)

// Useful for Merkle trees, HMAC optimisation, etc.
```

---

#### Testing & Benchmarking

##### testing/synctest — now stable [NEW in 1.25]

See Concurrency section above for full examples.

##### T.Attr, B.Attr, F.Attr [NEW in 1.25]

```go
func TestProcess(t *testing.T) {
    // Emit structured attributes to test output
    t.Attr("input_size", "1MB")
    t.Attr("algorithm", "sha256")
    t.Attr("env", os.Getenv("CI"))

    // Output in plain mode: === ATTR TestProcess input_size 1MB
    // Output in -json mode: {"Action":"attr","Test":"TestProcess","Key":"input_size","Value":"1MB"}

    result, err := Process(data)
    if err != nil { t.Fatal(err) }
    t.Attr("output_size", strconv.Itoa(len(result)))
}
```

##### T.Output, B.Output [NEW in 1.25]

```go
func TestWithOutput(t *testing.T) {
    w := t.Output() // io.Writer — writes to test log stream (no file/line prefix)
    fmt.Fprintln(w, "detailed trace:")
    for i, step := range steps {
        fmt.Fprintf(w, "  step %d: %s\n", i, step)
    }
}
```

##### AllocsPerRun panics on parallel tests [NEW in 1.25]

```go
// AllocsPerRun now panics if any parallel tests are running
// (results were already meaningless in parallel context — now it's explicit)

func TestAllocs(t *testing.T) {
    // Don't call t.Parallel() before AllocsPerRun
    allocs := testing.AllocsPerRun(100, func() {
        Process(data)
    })
    if allocs > 0 { t.Errorf("expected 0 allocs, got %v", allocs) }
}
```

##### B.Loop (1.24+) — use in all new benchmarks

```go
func BenchmarkProcess(b *testing.B) {
    db := setupTestDB(b)   // once
    ctx := b.Context()     // 1.24+
    for b.Loop() {
        db.Query("SELECT 1")
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

Prefer the built-in test lifecycle helpers over manual bookkeeping. All of these
are available in Go 1.25: `t.Helper()`, `t.TempDir()`, `t.Setenv()`,
`t.Cleanup()`, `t.Context()`/`t.Chdir()` (1.24+), and `t.Attr()`/`t.Output()`
(1.25+).

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

##### go vet — all analysers (cumulative)

| Version | Analyser | What it catches |
|---|---|---|
| 1.22 | `slog` | mismatched key/value pairs |
| 1.22 | `timeformat` | `time.Since` in defer position |
| 1.23 | `stdversion` | symbols too new for `go` version in go.mod |
| 1.24 | `tests` | malformed test/benchmark/fuzz names |
| 1.24 | `printf` | `fmt.Printf(s)` with no args |
| 1.24 | `buildtag` | point-release in build constraint |
| 1.25 | `waitgroup` | `wg.Add` called inside goroutine |
| 1.25 | `hostport` | `Sprintf("%s:%d")` for network address |

---

#### CLI Development & Tool Directive

##### Tool directive in go.mod (1.24+)

```bash
go get -tool golang.org/x/tools/cmd/stringer@latest
go get -tool github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go tool stringer -type=Color .
go tool sqlc generate
go get tool      # upgrade all
go install tool  # install to GOBIN
```

```
tool (
    golang.org/x/tools/cmd/stringer
    github.com/golangci/golangci-lint/cmd/golangci-lint
)
```

##### go doc -http (1.25+)

```bash
go doc -http net/http.Server  # open browser with server docs
go doc -http .                # docs for current package
```

##### flag

```go
host    := flag.String("host", "localhost", "server host")
port    := flag.Int("port", 8080, "server port")
verbose := flag.Bool("verbose", false, "verbose")
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

##### Container-aware GOMAXPROCS [NEW in 1.25] ★★

```go
// Before 1.25: GOMAXPROCS = runtime.NumCPU() — ignores container CPU limits
// In Kubernetes, this meant a pod with 0.5 CPU limit used GOMAXPROCS=64 on a 64-core node
// Result: excessive goroutine scheduling overhead, CPU throttling

// After 1.25: GOMAXPROCS automatically set to min(NumCPU, cgroupCPUBandwidth)
// Kubernetes 0.5 CPU limit → GOMAXPROCS=1 (actually 0.5, rounded up to 1)
// Kubernetes 4 CPU limit on 64-core node → GOMAXPROCS=4

// Dynamic: runtime checks periodically, updates GOMAXPROCS if limits change
// (e.g., Kubernetes VPA vertical autoscaling adjusts CPU limits at runtime)

// Check current GOMAXPROCS
fmt.Println(runtime.GOMAXPROCS(0))

// Disable container-aware behaviour (e.g., for testing)
// GODEBUG=containermaxprocs=0,updatemaxprocs=0 ./myapp

// Re-enable after manually setting (e.g., in init code that pre-dates 1.25)
runtime.SetDefaultGOMAXPROCS() // NEW in 1.25: restore to runtime default
```

**Kubernetes deployment**: remove any GOMAXPROCS-forcing init containers or `automaxprocs` library usage — Go 1.25 handles this natively.

##### Experimental Green Tea GC [NEW in 1.25]

```bash
# Enable experimental GC (10–40% reduction in GC overhead for GC-heavy programs)
GOEXPERIMENT=greenteagc go build ./cmd/myapp
./myapp

# Monitor GC with the new collector
GODEBUG=gccheckmark=1,checkfinalizers=1 ./myapp
```

Design improves locality and CPU scalability for marking/scanning small objects. Try it and report feedback. The design will continue to evolve before it becomes the default.

##### DWARF 5 — faster linking [NEW in 1.25]

Automatic. Large programs link noticeably faster. No action required.

##### Slice stack allocation [NEW in 1.25]

Automatic. Small slices may now live on the stack, reducing GC pressure. No action required unless you use `unsafe.Pointer` on slice internals.

##### AMD64 fused multiply-add [NEW in 1.25]

```go
// In GOAMD64=v3 or higher: FMA instructions used automatically
// This changes the exact floating-point values produced
// (but produces more accurate results per IEEE 754-2008)

// If you need exact pre-1.25 float results, cast explicitly:
result := float64(a*b) + c   // forces separate multiply then add
// vs:
result := a*b + c            // may use FMA in v3 mode
```

##### PGO (GA since 1.21)

```bash
go build -o myapp ./cmd/myapp
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cmd/myapp/default.pgo
go build ./cmd/myapp  # auto-picks up default.pgo
```

##### sync.Pool

```go
var bufPool = sync.Pool{New: func() any { return &bytes.Buffer{} }}
func process(data []byte) string {
    buf := bufPool.Get().(*bytes.Buffer); buf.Reset()
    defer bufPool.Put(buf)
    buf.Write(data); return buf.String()
}
```

---

#### Observability: Tracing & Profiling

##### runtime/trace.FlightRecorder [NEW in 1.25] ★★

```go
import "runtime/trace"

// Create a flight recorder — captures trace into in-memory ring buffer
// NewFlightRecorder returns *FlightRecorder (no error); Start() returns the error.
fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{
    MinAge:   5 * time.Second, // keep at least the last 5 seconds of trace
    MaxBytes: 10 * 1 << 20,    // cap the ring buffer at 10 MB (takes precedence over MinAge)
})
if err := fr.Start(); err != nil { log.Fatal(err) }
defer fr.Stop()

// Normal operation — trace recorded continuously in ring buffer

// On significant event (error, SLO breach, etc.):
f, err := os.Create("/tmp/trace-snapshot.out")
if err != nil { log.Fatal(err) }
defer f.Close()
_, err = fr.WriteTo(f) // snapshot the last MinAge of trace
if err != nil { log.Fatal(err) }

// Analyse: go tool trace /tmp/trace-snapshot.out
```

**Key advantage over `runtime/trace`**: minimal overhead, never fills disk. The ring buffer is overwritten continuously; you only write to disk when something interesting happens. Ideal for production always-on tracing.

##### VMA naming on Linux [NEW in 1.25]

```bash
# On Linux kernels with CONFIG_ANON_VMA_NAME support,
# /proc/PID/maps now shows Go memory regions with labels:
# 7f0000000000-7f0001000000 [anon: Go: heap]
# 7f0001000000-7f0002000000 [anon: Go: stack]

# Very useful for: memory profiling, container memory analysis, debugging
# Disable with: GODEBUG=decoratemappings=0
```

##### GODEBUG=checkfinalizers=1 [NEW in 1.25]

```bash
# Enables runtime diagnostics on each GC cycle:
# - Reports finalizer and cleanup queue lengths to stderr
# - Flags common misuse patterns (reference cycles, long-running finalizers)
GODEBUG=checkfinalizers=1 ./myapp 2>&1 | grep -i finalizer
```

##### pprof — mutex profile improved [NEW in 1.25]

The mutex profile for runtime-internal lock contention now correctly points to the end of the critical section (consistent with `sync.Mutex` profile behaviour).

```bash
go tool pprof http://localhost:6060/debug/pprof/mutex
# Contention stacks now more accurately reflect the blocking call site
```

---

#### Idioms & Things to Avoid

##### Do (1.25 additions)

- **Use `wg.Go(f)`** instead of the verbose `wg.Add(1)` + `go func() { defer wg.Done(); ... }()` pattern.
- **Use `net.JoinHostPort(h, port)` always** — vet catches `Sprintf("%s:%d")` now.
- **Check `err != nil` before using any result** — the nil pointer delay bug is fixed in 1.25; code that previously ran may now panic.
- **Use `os.Root` for all sandboxed filesystem access** — now has the full `os` package API within its sandbox.
- **Use `testing/synctest`** for testing concurrent/time-dependent code — no more real `time.Sleep` in tests.
- **Test with `GOEXPERIMENT=jsonv2 go test ./...`** to find JSON compatibility issues early.
- **Use `go doc -http .`** to browse package docs locally.
- **Use `go.mod ignore`** for generated/vendor directories that you don't want to affect `./...` patterns.
- **Use `work` pattern** in workspaces instead of per-module `./...`.
- **Deploy Go 1.25 to Kubernetes without `automaxprocs`** — container-aware GOMAXPROCS is now built-in.
- **Use `runtime/trace.FlightRecorder`** for always-on production tracing with minimal overhead.
- **Use `hash.Cloner` for mid-stream hash branching** (Merkle trees, HMAC prefix sharing).
- **Use `slog.GroupAttrs(name, attrs...)`** to build structured log groups from slices.
- **Use `t.Attr("key", "value")`** to attach metadata to test results.
- **`GODEBUG=checkfinalizers=1`** when debugging GC / cleanup issues.

##### Don't

| Anti-pattern | Why | Instead |
|---|---|---|
| `fmt.Sprintf("%s:%d", h, p)` for network addr | Breaks IPv6 | `net.JoinHostPort(h, strconv.Itoa(p))` |
| Use result before checking error | Nil bug fixed in 1.25 — now panics | `if err != nil { return err }` first |
| `wg.Add(1)` inside goroutine | Race with `wg.Wait()` | `wg.Go(f)` or `wg.Add(1)` before `go` |
| Manual `GOMAXPROCS` in containers | 1.25 handles it | Remove; use `runtime.SetDefaultGOMAXPROCS()` |
| `runtime.SetFinalizer` for new code | One-per-object, cycle issues | `runtime.AddCleanup` |
| `tools.go` blank import | Obsolete since 1.24 | `go.mod` `tool` directive |
| `for range b.N` in benchmarks | Compiler may elide | `for b.Loop()` |
| `time.Sleep` in tests for timing | Non-deterministic | `testing/synctest` |
| `strings.Split` for large inputs | Allocates full `[]string` | `strings.SplitSeq` |
| `rand.Seed()` | No-op since 1.24 | `rand.New(rand.NewPCG(s1, s2))` |
| `var m map[string]int` | Nil, writes panic | `make(map[string]int)` |
| Log AND return error | Double-logging | Return; log at boundary |
| `panic` for business logic | Crashes | `return error` |
| Copying a mutex | Broken | Embed + pointer receiver |
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
type UserRepository interface{} // PascalCase
type httpClient struct{}         // camelCase unexported
type HTTPClient struct{}         // acronyms all-caps
var ErrNotFound = errors.New("not found")
type Stringer interface{ String() string }
func (u *User) IsAdmin() bool {}
```

##### Formatting & linting

```bash
gofmt -w .
goimports -w .
go vet ./...   # includes: waitgroup, hostport, stdversion, tests, printf, buildtag
staticcheck ./...
golangci-lint run
```

A `golangci-lint` v2 configuration (the tool version is independent of the Go
language version and lints Go 1.25 code fine; in 1.24+ you can pin and run it via
a `tool` directive and `go tool golangci-lint run`):

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

*End of Go 1.25 Complete Developer Guideline*
