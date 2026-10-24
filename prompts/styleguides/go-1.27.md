### Go 1.27 — Complete Developer Guideline

> **Version**: Go 1.27.0 (released 2026-08-19); latest patch at time of writing: Go 1.27.3 (2026-10-19)
> **Scope**: Full language spec, all 1.27 changes delta over 1.26, idioms, concurrency, performance, CLI, file I/O, testing, and best practices — with examples throughout.
> **Note**: This document is self-contained. All relevant prior-version material is carried forward or superseded. Sections marked **[NEW in 1.27]** cover additions specific to this version. Supported releases as of Go 1.27: **1.27 and 1.26** (1.25 is out of support).

---

#### What's New in Go 1.27

Go 1.27.0 shipped 2026-08-19. It has **three language changes**: **generic methods**, **promoted fields as struct-literal keys**, and **function type inference in every assignment context**. It makes **`encoding/json/v2` generally available** (and re-implements `encoding/json` on top of it), adds the **`uuid`** and **`crypto/mldsa`** packages, an experimental portable **`simd`** package, makes the **goroutine leak profile GA**, ships **size-specialised small-object allocation**, runs the **`stdversion` vet check under `go test`**, and **permanently removes seven GODEBUG settings** (including `asynctimerchan`).

##### Language changes

###### 1. Generic methods [NEW in 1.27] ★★★

A method declaration may now declare its own type parameters, in addition to any the receiver type has.

```go
type List[E any] []E

// Before 1.27: had to be a package-level function
func Map[E, F any](l List[E], f func(E) F) List[F] { /* ... */ }

// Go 1.27: a generic method
func (l List[E]) Map[F any](f func(E) F) List[F] {
    r := make(List[F], len(l))
    for i, x := range l {
        r[i] = f(x)
    }
    return r
}

nums := List[int]{1, 2, 3}
strs := nums.Map(strconv.Itoa)           // F inferred as string
strs2 := nums.Map[string](strconv.Itoa)  // explicit instantiation also works
```

Generic methods also work on non-generic types:

```go
type Registry struct{ m map[string]any }

func (r *Registry) Get[T any](key string) (T, bool) {
    v, ok := r.m[key].(T)
    return v, ok
}

port, ok := r.Get[int]("port")   // T cannot be inferred from arguments → explicit
```

**Rules (verified on go1.27.3):**

| Rule | Consequence |
|---|---|
| Interface methods **cannot** declare type parameters | `interface{ Get[T any]() T }` → `interface method must have no type parameters` |
| A generic method **never** satisfies an interface method | `have Convert[T any](int) string, want Convert(int) string` — even if `T` is unused |
| Generic methods are **not in the method set** seen by `reflect` | `reflect.Type.NumMethod()` / `Methods()` skip them; `reflect` cannot call them |
| Must be instantiated (explicitly or by inference) before being called or used as a value | `b.Zero()` with an uninferable `T` → `cannot infer T` |
| Promotion through embedding works | `Outer{Base}` exposes `o.Show(42)` when `Base` has `Show[T any]` |
| Method values and expressions work once instantiated | `f := r.Get[string]`, `List[int].Map[string]` |
| Gated by `go 1.27` in go.mod | `generic method requires go1.27 or later (-lang was set to go1.26)` |

The standard library uses this in exactly one place in 1.27: `math/rand/v2.(*Rand).N[Int intType](Int) Int`, the method counterpart of the top-level `rand.N`. The per-type methods (`IntN`, `Int64N`, …) remain.

```go
rng := rand.New(rand.NewPCG(1, 2))
d := rng.N(5 * time.Second)   // time.Duration in [0, 5s)
i := rng.N(int64(100))        // int64 in [0, 100)
```

**Design guidance**

- Use a generic method when the operation belongs to the type's namespace (e.g. `list.Map`, `cache.GetAs`). Keep package-level functions when the operation is symmetric between types or must be usable through an interface.
- **Do not** design an interface around a method you plan to make generic — it cannot be expressed. If a type must satisfy an interface, keep a non-generic method for that and add a generic convenience method next to it.
- Generic methods are invisible to `reflect`, so frameworks that discover methods reflectively (RPC registration, template method calls, some DI containers and mock generators) will not see them.

###### 2. Struct literal keys may be promoted fields [NEW in 1.27] ★★

A key in a struct composite literal may now be any valid field selector — including a field promoted through embedded structs — not only a top-level field name.

```go
type Meta struct{ Name, Owner string }
type Spec struct{ Replicas int }
type Resource struct {
    Meta
    Spec
    Created time.Time
}

// Before 1.27
r := Resource{Meta: Meta{Name: "web", Owner: "team-a"}, Spec: Spec{Replicas: 3}}

// Go 1.27
r = Resource{Name: "web", Owner: "team-a", Replicas: 3}

// Works through multiple levels and across packages (exported fields only)
type Client struct {
    http.Client
    Retries int
}
c := Client{Timeout: 5 * time.Second, Retries: 3}
```

**Restrictions (each verified as a compile error on go1.27.3):**

```go
type Server struct{ *http.Server; Debug bool }
_ = Server{Addr: ":8080"}
// invalid implicit pointer indirection to reach Addr
// — embedded fields traversed to reach the key must not be pointers

_ = Resource{Meta: Meta{Owner: "x"}, Name: "y"}
// cannot specify promoted field Name and enclosing embedded field Meta

_ = Resource{Meta.Name: "y"}
// invalid field name Meta.Name in struct literal — the key is the bare field name, not a dotted path

type T struct{ M1; M2 }   // both have field ID
_ = T{ID: 1}
// unknown field ID in struct literal of type T — ambiguous promoted names are not selectable
```

`go fix`'s new `embedlit` modernizer rewrites `Obj{Base: Base{Name: "x"}, N: 1}` to `Obj{Name: "x", N: 1}`.

###### 3. Function type inference in all assignment contexts [NEW in 1.27] ★

Since 1.21, a generic function could be assigned to a typed variable, passed as an argument, or returned without instantiation. Go 1.27 extends that to **every** context where the function must be assignable to a (non-generic) function type: composite-literal elements and fields, map and slice values, conversions, channel sends — and generic **method** values.

```go
func Identity[T any](v T) T { return v }

type Handlers struct {
    OnInt func(int) int
    OnStr func(string) string
}

h := Handlers{OnInt: Identity, OnStr: Identity}           // 1.27
m := map[string]func(float64) float64{"id": Identity}       // 1.27
f := (func(int) int)(Identity)                              // 1.27
ch := make(chan func(int) int, 1); ch <- Identity           // 1.27

var g func(string) string = Identity                        // already valid since 1.21

var b B                                                     // B has Show[T any](T) string
var show func(int) string = b.Show                          // 1.27: method type arg inferred
```

> **Reviewer caution — not version-gated.** Unlike generic methods and promoted literal keys, the go1.27.3 compiler accepts these inference cases even when go.mod says `go 1.22`; a Go 1.26 toolchain rejects them with `cannot use generic function Identity without instantiation`. A library that declares `go 1.26` but relies on this compiles on 1.27 and breaks for 1.26 users. Flag it, or require `go 1.27`.

---

#### Project Layout & Modules

##### go mod init writes the full toolchain version

```bash
# Using a Go 1.27.3 toolchain:
go mod init example.com/myapp
# go.mod: go 1.27.3
```

The "N-1" default (`go 1.25.0` from a 1.26 toolchain) existed **only in Go 1.26.0**; it was reverted in 1.26.1 (golang/go#77653, #77860) and Go 1.27 writes the running toolchain's version. Lower it explicitly when a library must support the previous release:

```bash
go mod edit -go=1.26.0
```

```
module github.com/yourname/myapp

go 1.27.0

require (
    github.com/spf13/cobra v1.8.0
)

tool (
    golang.org/x/tools/cmd/stringer
    github.com/golangci/golangci-lint/v2/cmd/golangci-lint
)

ignore (
    ./node_modules
    ./testdata/generated
)
```

##### go mod tidy consolidates require blocks [NEW in 1.27]

For modules declaring `go 1.27` or later, `go mod tidy` merges all `require` blocks into **at most two**: one for direct and one for indirect dependencies. Comments attached to directives are kept.

```
# before tidy                         # after tidy (go 1.27)
require golang.org/x/text v0.20.0     require (
                                          // pinned for reason X
// pinned for reason X                    golang.org/x/mod v0.22.0
require golang.org/x/mod v0.22.0          golang.org/x/text v0.20.0
                                      )
```

Expect a one-time go.mod diff when a module moves to `go 1.27`; do not flag that churn as unrelated.

##### Other module/toolchain changes [NEW in 1.27]

- The `go` command no longer supports **Bazaar (bzr)**; modules hosted only on bzr can no longer be fetched directly.
- `go doc` accepts `pkg@version` (`go doc golang.org/x/mod/semver@v0.22.0 Compare`) and `-ex` to list examples; naming an example (`go doc bytes.ExampleBuffer`) prints its source.
- Building Go 1.27 from source still requires **Go 1.24.6 or later** (`minBootstrap = "go1.24.6"`). Go 1.28 is expected to require a 1.26.x bootstrap.
- macOS: Go 1.27 requires **macOS 13 Ventura or later**.

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

The same applies to the standard library: a symbol added in 1.27 (`strings.CutLast`, `uuid`, `encoding/json/v2`, …) is only valid when the file's effective Go version (go.mod `go` line, or a `//go:build go1.27` tag) is at least 1.27. `go build` does not enforce this; `go vet` and (since 1.27) `go test` do.

---

#### Basic Types & Variables

```go
var x int = 10
y := "hello"
z := 3.14
lo, hi := min(x, 20), max(x, 20)

type Direction int
const (
    North Direction = iota
    East
    South
    West
)

type Permission uint
const (
    Read Permission = 1 << iota
    Write
    Execute
)
```

The spec now formally says `any` is **not** a named type (it is an alias for `interface{}`); behaviour is unchanged.

---

#### Pointers — new(expr)

##### new(expr) (1.26+)

```go
// Pre-1.26 patterns — still valid, but verbose
x := 42; p := &x
val := "hello"; s := struct{ V *string }{V: &val}

// 1.26+: new accepts an expression
s2 := struct{ V *string }{V: new("hello")}
p1 := new(42)            // *int
p2 := new(time.Second)   // *time.Duration
pt := new(Point{X: 1})   // *Point
// new(nil) is a compile error
```

A `ptr[T any](v T) *T` helper is obsolete for modules on `go 1.26`+; `go fix` (`newexpr`) finds such `func varOf(x T) *T { return &x }` helpers, turns them into `//go:fix inline` wrappers around `new(x)`, and inlines their call sites.

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
for i, v := range slices.Backward(s) {}  // 1.23+; go fix (slicesbackward) suggests this for reverse index loops
```

Each iteration has its own loop variable (1.22+), so `x := x` copies and `tt := tt` shadows inside loops are redundant (`go fix` `forvar` removes them in `range` loops; copies in 3-clause loops are left alone).

##### switch / select

```go
switch x { case 1: default: }
switch { case x < 0: default: }
func describe(i any) {
    switch v := i.(type) {
    case int:    fmt.Printf("int: %d\n", v)
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
    total := 0
    for _, n := range nums { total += n }
    return total
}
func counter() func() int { n := 0; return func() int { n++; return n } }
```

##### Function literal names & code pointers [NEW in 1.27]

The compiler now gives closures stable, simpler symbol names regardless of inlining and may **share code between instances of the same function literal**. Do not compare function code pointers (via `reflect.Value.Pointer` or `unsafe`) to decide whether two closures are "the same": closures with different captured data can now have equal code pointers more often. Tests that assert on closure symbol names (`main.main.func1`, etc.) may need updating.

---

#### Defer, Panic & Recover

```go
defer f.Close()
defer func() { log.Println(time.Since(start)) }()
defer func() {
    if r := recover(); r != nil { /* panic occurred */ }
}()
```

##### Defer inside loops

```go
// WRONG: all defers run at function return
for path := range filePaths() { f, _ := os.Open(path); defer f.Close() }

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

##### Tracebacks include goroutine labels [NEW in 1.27]

For modules on `go 1.27`, crash tracebacks and `runtime.Stack` headers include `runtime/pprof` goroutine labels:

```
goroutine 1 [running] {request_id: abc123}:
```

Opt out with `GODEBUG=tracebacklabels=0` (kept indefinitely). Treat labels as **logged data**: do not put tokens, emails or other secrets into `pprof.Labels` — they now end up in crash output. Labels applied with `pprof.Do` are restored by a `defer` during panic unwinding, so a panic inside `pprof.Do` usually prints *without* them; labels set with `pprof.SetGoroutineLabels` (or inherited by child goroutines) do appear.

---

#### Structs & Methods — Promoted-Field Literal Keys

```go
type User struct { ID int; Name, Email string; admin bool }
func (u User) Display() string { return u.Name }
func (u *User) Promote()       { u.admin = true }

// new(expr) shines with optional struct fields (1.26+)
type ServerConfig struct {
    Host  string
    Port  *int  `json:"port,omitempty"`
    Debug *bool `json:"debug,omitempty"`
}
cfg := ServerConfig{Host: "localhost", Port: new(8080), Debug: new(true)}
```

##### Promoted-field keys (1.27+)

```go
type Base struct { ID string; CreatedAt time.Time }
type Order struct {
    Base
    Total int
}

o := Order{ID: "o-1", CreatedAt: now, Total: 42}   // 1.27
o2 := Order{Base: Base{ID: "o-1"}, Total: 42}      // still valid
// Order{Base: b, ID: "x"}                          // error: overlapping keys
```

Unkeyed literals are unaffected and still list embedded structs positionally.

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

##### Interfaces and generic methods (1.27)

Interface methods cannot have type parameters, and a generic method cannot implement one. Keep the interface method concrete and layer a generic helper on top:

```go
type Store interface {
    GetRaw(key string) ([]byte, error)   // satisfies interfaces
}

type RedisStore struct{ /* ... */ }

func (s *RedisStore) GetRaw(key string) ([]byte, error) { /* ... */ }

// convenience; NOT part of any interface
func (s *RedisStore) GetJSON[T any](key string) (T, error) {
    var v T
    b, err := s.GetRaw(key)
    if err != nil { return v, err }
    return v, json.Unmarshal(b, &v)
}
```

##### KEM / key-exchange interfaces (1.26+)

```go
// crypto
type Encapsulator interface {
    Bytes() []byte
    Encapsulate() (sharedKey, ciphertext []byte)
}
type Decapsulator interface {
    Encapsulator() Encapsulator
    Decapsulate(ciphertext []byte) (sharedKey []byte, err error)
}

// crypto/ecdh — abstract (e.g. hardware-held) ECDH private keys
type KeyExchanger interface {
    PublicKey() *ecdh.PublicKey
    Curve() ecdh.Curve
    ECDH(*ecdh.PublicKey) ([]byte, error)
}

// crypto.MessageSigner (1.25+)
type MessageSigner interface {
    crypto.Signer
    SignMessage(rand io.Reader, msg []byte, opts crypto.SignerOpts) ([]byte, error)
}
```

##### slog.NewMultiHandler (1.26+)

```go
h1 := slog.NewJSONHandler(os.Stdout, nil)
h2 := slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelWarn})
slog.SetDefault(slog.New(slog.NewMultiHandler(h1, h2)))
```

---

#### Generics — Generic Methods & Inference

```go
// Generic methods (1.27+)
type Set[T comparable] map[T]struct{}

func (s Set[T]) MapTo[U comparable](f func(T) U) Set[U] {
    out := make(Set[U], len(s))
    for k := range s { out[f(k)] = struct{}{} }
    return out
}

// Recursive type constraints (1.26+)
type Ordered[T Ordered[T]] interface { Less(T) bool }
type Tree[T Ordered[T]] struct { nodes []T }
t := Tree[netip.Addr]{}   // netip.Addr has Less(Addr) bool

// Regular generic functions (unchanged)
func Map[T, U any](s []T, f func(T) U) []U {
    r := make([]U, len(s))
    for i, v := range s { r[i] = f(v) }
    return r
}

// Generic type aliases (1.24+)
type Seq[V any] = func(yield func(V) bool)
type UserRepo = storage.Repository[User]

host := cmp.Or(os.Getenv("HOST"), cfg.Host, "localhost")
```

##### hash/maphash.Hasher — custom hash/equality contracts [NEW in 1.27]

`maphash.Hasher[T]` is the standard interface between a value type and future hash-based containers (it is not yet consumed by a standard container). `ComparableHasher[T]` implements it with `==`. `go/types.Hasher` implements it for `types.Type`.

```go
type Hasher[T any] interface {
    Hash(*maphash.Hash, T)
    Equal(x, y T) bool
}

// Case-insensitive keys
type CaseInsensitive struct{}
func (CaseInsensitive) Hash(h *maphash.Hash, s string) { h.WriteString(strings.ToLower(s)) }
func (CaseInsensitive) Equal(x, y string) bool         { return strings.ToLower(x) == strings.ToLower(y) }
```

Invariant to check in review: values that are `Equal` **must** hash identically, and hashers must be stateless.

---

#### Iterators — iter package

Stable since 1.23.

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

next, stop := iter.Pull(mySeq)
defer stop()
for { v, ok := next(); if !ok { break }; process(v) }
```

The spec now states that the `yield` function of a range-over-func iterator **cannot be variadic**.

##### strings / bytes lazy iterators (1.24+)

```go
for line := range strings.Lines("hello\nworld\n") { fmt.Println(line) }
for part := range strings.SplitSeq("a,b,c", ",") { fmt.Println(part) }
for word := range strings.FieldsSeq("hello world") { fmt.Println(word) }
```

##### reflect.Type/Value iterators (1.26+)

```go
typ := reflect.TypeFor[http.Client]()
for f := range typ.Fields() { fmt.Println(f.Name, f.Type) }

ptrTyp := reflect.TypeFor[*http.Client]()   // methods live on the pointer type
for m := range ptrTyp.Methods() { fmt.Println(m.Name) }

fnTyp := reflect.TypeFor[filepath.WalkFunc]()
for p := range fnTyp.Ins()  { fmt.Println("in:", p) }
for p := range fnTyp.Outs() { fmt.Println("out:", p) }

val := reflect.ValueOf(&http.Client{})
for f, v := range val.Elem().Fields() { fmt.Println(f.Name, v.Kind()) }
for m, v := range val.Methods()       { fmt.Println(m.Name, v.Kind()) }
```

Generic methods (1.27) never appear in these iterators.

---

#### Collection Types: Arrays, Slices, Maps

##### slices package (cumulative)

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

##### maps package (cumulative)

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

#### Strings & Bytes — CutLast

##### strings.CutLast / bytes.CutLast [NEW in 1.27] ★

`CutLast(s, sep)` slices around the **last** occurrence of `sep`. Not found → `s, "", false`.

```go
dir, file, ok := strings.CutLast("a/b/c.txt", "/")   // "a/b", "c.txt", true
b, a, ok := strings.CutLast("noslash", "/")          // "noslash", "", false
k, v, _ := bytes.CutLast([]byte("k=v=w"), []byte("=")) // "k=v", "w"

// Replaces the LastIndex dance:
if i := strings.LastIndex(s, "."); i >= 0 { name, ext = s[:i], s[i+1:] }
name, ext, _ = strings.CutLast(s, ".")
```

Do not use it to split host and port: `CutLast("[::1]:8080", ":")` returns `"[::1]"` with brackets. Use `net.SplitHostPort`.

##### bytes.Buffer.Peek (1.26+)

```go
buf := bytes.NewBufferString("hello world")
sample, err := buf.Peek(5)   // "hello", nil — does not advance
buf.Next(6)
sample, err = buf.Peek(100)  // "world", io.EOF — fewer than n bytes remain
// The returned slice aliases the buffer — valid only until the next read/write.
```

##### Strings operations

```go
strings.Contains("foobar", "bar")
strings.HasPrefix("foobar", "foo")
strings.Split("a,b,c", ",")          // allocates []string
strings.SplitSeq("a,b,c", ",")       // 1.24+: lazy iterator
strings.Join([]string{"a", "b"}, "-")
strings.TrimSpace("  hello  ")
strings.Cut("k=v", "=")              // first separator
strings.CutLast("a.b.c", ".")        // 1.27+: last separator
strings.CutPrefix("Gopher", "Go")
strings.CutSuffix("Gopher", "er")

var b strings.Builder
b.Grow(1000)
for range 1000 { b.WriteByte('x') }
result := b.String()
```

##### Trim cutsets and empty Split

`Trim`/`TrimLeft`/`TrimRight` take a **set of runes**, not a prefix or suffix.
`Split` never returns an empty slice, so `len(parts) > 0` is always true (the
same holds for the first yield of `SplitSeq`).

```go
strings.Trim("gogolang.go", ".go")       // "lan" — every '.', 'g', 'o' stripped
strings.TrimLeft("gogolang.go", "go")    // "lang.go"
strings.TrimSuffix("gogolang.go", ".go") // "gogolang" — what was probably meant
strings.TrimPrefix("gogolang.go", "go")  // "golang.go"

strings.Split("", ",")                   // [""] — length 1, not 0
```

##### Unicode 17 [NEW in 1.27]

`unicode`, `regexp`, `strconv` and friends moved from Unicode 15 to **Unicode 17.0.0** (`unicode.Version == "17.0.0"`), adding scripts such as `unicode.Garay`, `unicode.Todhri`, `unicode.Sunuwar`. Results of `unicode.IsLetter`, `\p{L}` etc. can change for newly assigned code points.

---

#### Error Handling — errors.AsType

##### errors.AsType (1.26+) ★★

```go
// errors.As needs a target variable
var ve *ValidationError
if errors.As(err, &ve) { fmt.Println(ve.Field) }

// errors.AsType — generic, scoped, no reflection
if ve, ok := errors.AsType[*ValidationError](err); ok {
    fmt.Println(ve.Field)
}

if opErr, ok := errors.AsType[*net.OpError](err); ok {
    fmt.Println("network:", opErr.Op)
} else if dnsErr, ok := errors.AsType[*net.DNSError](err); ok {
    fmt.Println("dns:", dnsErr.Name)
}
```

`AsType[E error]` constrains `E` to implement `error`, so misuse is a compile error instead of the runtime panic `errors.As` produces:

```go
type ValidationError struct{ Field string }
func (e *ValidationError) Error() string { return "invalid " + e.Field }

errors.AsType[ValidationError](err)   // compile error: ValidationError does not satisfy error
                                      // (method Error has pointer receiver)
```

`go fix` (`errorsastype`) rewrites eligible `errors.As` calls.

##### Wrapping with %w — vet now flags pointer-to-error [NEW in 1.27]

The `printf` analyzer reports `fmt.Errorf("...: %w", p)` when `p` is `*E` but `E` itself implements `error`. Such wrapping makes `errors.Is`/`errors.As`/`errors.AsType[E]` checks silently fail.

```go
type NotFound struct{ Key string }
func (e NotFound) Error() string { return "not found: " + e.Key }

func lookup(e *NotFound) error {
    return fmt.Errorf("lookup: %w", e)
    // vet: %w wants operand of error type NotFound, not pointer type *NotFound (defeats errors.Is)
}
// Fix: wrap the value (`*e`), or declare Error on *NotFound and use *NotFound consistently.
```

No report when `*E` is the type that implements `error`.

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

// Fan out to several destinations (1.26+)
slog.SetDefault(slog.New(slog.NewMultiHandler(
    slog.NewJSONHandler(os.Stdout, nil),
    slog.NewTextHandler(alertFile, &slog.HandlerOptions{Level: slog.LevelError}),
)))

reqLogger := logger.WithGroup("request").With("id", reqID)
slog.InfoContext(ctx, "handler", "status", 200)

// Discard in benchmarks (1.24+) — DiscardHandler is a variable, not a type
slog.SetDefault(slog.New(slog.DiscardHandler))
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

`go fix`'s modernizer for this is now named **`waitgroupgo`** (renamed from `waitgroup` in 1.27). The separate `go vet` analyzer `waitgroup` (misplaced `wg.Add`) keeps its name.

##### Timer channels are always synchronous [NEW in 1.27]

Since 1.23, `time.Timer`/`time.Ticker` channels are unbuffered for modules on `go 1.23`+; `asynctimerchan` allowed opting back. In 1.27 the setting is **removed**: timer channels are unbuffered for every module, and `Reset`/`Stop` never leave a stale value behind.

```go
t := time.NewTimer(5 * time.Second)
defer t.Stop()
t.Reset(10 * time.Second)   // no drain needed before Reset (1.23+ semantics, now universal)
fmt.Println(cap(t.C))       // 0
```

Drain-before-`Reset` code (`if !t.Stop() { <-t.C }`) is now redundant: if the value has not been received, `Stop` returns `true` (even when the timer already expired), so the drain never runs; `Stop` returns `false` only when the value was already received or the timer was already stopped — and then a bare `<-t.C` blocks forever, as it always did. Suggest removing the idiom; do not report it as a new deadlock. `GODEBUG=asynctimerchan=1` now **aborts the program** — see [GODEBUG Removals](#godebug-removals).

##### sync.Mutex / sync.RWMutex

```go
type SafeMap struct {
    mu sync.RWMutex
    m  map[string]int
}
func (s *SafeMap) Get(k string) (int, bool) {
    s.mu.RLock(); defer s.mu.RUnlock()
    v, ok := s.m[k]
    return v, ok
}
func (s *SafeMap) Set(k string, v int) {
    s.mu.Lock(); defer s.mu.Unlock()
    s.m[k] = v
}
```

##### sync/atomic — prefer atomic types

```go
type Server struct{ hits atomic.Int64 }   // not: hits int64 + atomic.AddInt64(&s.hits, 1)
func (s *Server) Hit() { s.hits.Add(1) }

var flags atomic.Uint32
flags.Or(0b0100)           // set bit (1.23+)
flags.And(^uint32(0b0100)) // clear bit
```

The new `atomictypes` modernizer performs this rewrite. Atomic types contain a `noCopy` marker, so a struct holding one must not be copied (vet `copylocks`).

##### testing/synctest (stable since 1.25) — synctest.Sleep [NEW in 1.27]

```go
func TestDebounce(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        calls := 0
        fn := Debounce(func() { calls++ }, 100*time.Millisecond)
        fn(); fn(); fn()
        synctest.Sleep(100 * time.Millisecond)   // == time.Sleep(d); synctest.Wait()
        if calls != 1 { t.Errorf("got %d calls", calls) }
    })
}
```

Plain `time.Sleep` in the test goroutine races with a system-under-test goroutine sleeping the same duration; `synctest.Sleep` also waits for the bubble to settle. Prefer it over `time.Sleep` + manual `Wait` pairs.

##### runtime/secret (experimental, 1.26+)

Goroutines started while in secret mode now inherit secret mode (1.27).

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

##### runtime.AddCleanup (1.24+)

```go
cleanup := runtime.AddCleanup(obj, func(id int) {
    closeHandle(id)   // must not reference obj, or obj is never collected
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

##### os/signal.NotifyContext — signal cause (1.26+)

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
<-ctx.Done()
fmt.Println(context.Cause(ctx))   // e.g. "interrupt signal received"
```

In tests, prefer `t.Context()` (1.24+); `go fix` (`testingcontext`) replaces `context.WithCancel` boilerplate in tests.

---

#### File I/O & Filesystem

##### Path joining — `filepath.Join` never resets on an absolute element

`filepath.Join` glues every element together with a separator and then runs
`filepath.Clean` over the result. A leading `/` on a later element is treated as
a separator, not as a new root.

```go
filepath.Join("/target/mount", "/etc/passwd") // "/target/mount/etc/passwd"
path.Join("/target/mount", "/etc/passwd")     // "/target/mount/etc/passwd"
```

```go
filepath.Join("/target/mount", "../../etc/passwd") // "/etc/passwd"
filepath.Join("/target/mount", "/../etc/passwd")   // "/target/etc/passwd"
```

For containment, use `os.Root` (1.24+) rather than any lexical check. `Clean`,
`IsLocal` and `Localize` never resolve symlinks; `os.Root` does.

##### Path containment checks

`strings.HasPrefix` is not a containment check, and `filepath.Rel` does not
report an escape as an error — it returns a `..` path with `err == nil`.

```go
strings.HasPrefix(filepath.Clean("/baseball/x"), "/base") // true — not contained
filepath.Rel("/base", "/etc/passwd")                      // "../etc/passwd", nil
filepath.Rel("/base", "/baseball/x")                      // "../baseball/x", nil

// Inspect the returned path, not the error:
rel, err := filepath.Rel(base, target)
ok := err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
```

`filepath.EvalSymlinks` requires every element to exist, so it cannot validate a
not-yet-created file, and resolve-then-open is TOCTOU-racy. `os.Root` has
neither problem: it resolves at the OS level and the boundary holds for every
operation on the handle.

##### Path decomposition and empty inputs

Nothing here errors — empty or degenerate input silently becomes `"."` or the
base directory. Reject an empty path before joining it.

```go
filepath.Clean("")        // "."
filepath.Base("")         // "."     Base("/") // "/"
filepath.Dir("a")         // "."
filepath.Join("base", "") // "base" — an empty user path yields the base dir
filepath.IsLocal("")      // false   IsLocal(".") // true

// Ext: suffix after the FINAL dot, dot included, last path element only
filepath.Ext("archive.tar.gz") // ".gz" — not ".tar.gz"
filepath.Ext("Makefile")       // ""
filepath.Ext(".bashrc")        // ".bashrc"
filepath.Ext("dir.d/file")     // ""
```

##### path vs path/filepath, and glob matching

`path` always uses `/`; `path/filepath` uses the OS separator. Use `path` for
`io/fs`, `embed.FS`, `os.DirFS`, `net/url` paths and `tar`/`zip` entry names; use
`path/filepath` for names on the local filesystem.

```go
filepath.IsAbs("/foo")   // true on Unix, false on Windows (no volume name)
filepath.IsAbs(`C:\foo`) // false on Unix, true on Windows

// No ** in Match/Glob — a ** component is just a *, matching a single level:
filepath.Match("a/*", "a/b/c")     // false, nil
filepath.Match("*.go", "sub/x.go") // false, nil
filepath.Glob("**/*.go")           // same result as "*/*.go"
```

Recursive matching needs `filepath.WalkDir` plus `Match` on each entry.

##### Standard I/O

```go
f, err := os.Open("file.txt"); if err != nil { return err }; defer f.Close()
scanner := bufio.NewScanner(f)
scanner.Buffer(make([]byte, 1<<20), 1<<20)
for scanner.Scan() { process(scanner.Text()) }
if err := scanner.Err(); err != nil { return err }
io.Copy(dst, src)
data, err := os.ReadFile("small.json")
data, err = io.ReadAll(r)   // ~2x faster, about half the memory since 1.26
```

##### os.Root — sandboxed filesystem (1.24+, expanded 1.25)

```go
root, err := os.OpenRoot("./uploads")
if err != nil { return err }
defer root.Close()
f, _ := root.Open("user/file.txt")      // escapes via .. or symlinks are rejected
f2, _ := root.Create("user/new.txt")
root.MkdirAll("a/b/c", 0o755)
root.RemoveAll("old_dir")
root.Rename("old.txt", "new.txt")
root.ReadFile("config.json")
root.WriteFile("out.json", data, 0o644)
```

##### os.Process.WithHandle (1.26+)

```go
// Runs f with a pidfd (Linux 5.4+) or process Handle (Windows) for the process.
err := proc.WithHandle(func(handle uintptr) {
    // OS-specific use of handle; valid only for the duration of f
})
if errors.Is(err, os.ErrNoHandle) { /* platform/kernel lacks handles */ }
```

##### net.UnixConn reads return io.EOF directly [NEW in 1.27]

`UnixConn` read methods now return bare `io.EOF` instead of a `*net.OpError` wrapping it. `err == io.EOF` checks start working; `errors.Is(err, io.EOF)` worked before and still does — prefer it.

##### net/url

```go
// 1.26+: colons outside a bracketed IPv6 host are rejected
_, err := url.Parse("http://::1/")            // error (GODEBUG=urlstrictcolons=0 restores)
_, err  = url.Parse("http://[::1]/")          // ok

// 1.27+: deep copies
u2 := u.Clone()        // *URL — copies User too; mutating u2 never touches u
v2 := values.Clone()   // url.Values — slices are copied, not shared
```

Copying a `url.Values` map (assignment, or a `for k, v := range` copy loop) shares its `[]string` slices — a frequent aliasing bug when building per-request queries from a base; use `Values.Clone()` on 1.27. `u2 := *u` for a `*url.URL` is fine (`*Userinfo` is immutable); `u.Clone()` is simply the clearer spelling.

---

#### JSON — encoding/json/v2 is GA

##### Status in 1.27 [NEW in 1.27] ★★★

- `encoding/json/v2` and `encoding/json/jsontext` are **generally available** — no `GOEXPERIMENT` needed.
- `encoding/json` (v1) is **re-implemented on top of v2** with options that preserve v1 behaviour. Output is unchanged, but **error message text may differ**; unmarshal is significantly faster. `GOEXPERIMENT=nojsonv2` restores the old v1 implementation temporarily.
- v1 remains supported forever; migrating is optional. New code should prefer v2.

Do not write tests or logic that string-match `encoding/json` error messages; use `errors.As`/`errors.AsType` on `*json.UnmarshalTypeError`, `*json.SyntaxError`, `*jsonv2.SemanticError`, `*jsontext.SyntacticError`.

##### v2 API

```go
import (
    "encoding/json/jsontext"
    jsonv2 "encoding/json/v2"
)

b, err := jsonv2.Marshal(v)                          // variadic Options
err = jsonv2.Unmarshal(b, &v,
    jsonv2.RejectUnknownMembers(true),
)
err = jsonv2.MarshalWrite(w, v, jsontext.WithIndent("  "))
err = jsonv2.UnmarshalRead(r, &v)                    // reads to EOF
err = jsonv2.MarshalEncode(enc, v)                   // *jsontext.Encoder
err = jsonv2.UnmarshalDecode(dec, &v)                // *jsontext.Decoder (streaming)
```

##### v1 vs v2 default differences (verified on go1.27.3)

| Behaviour | v1 `encoding/json` | v2 default | v2 option to get v1 behaviour |
|---|---|---|---|
| Duplicate object names | last wins | **error** (`duplicate object member name`) | `jsontext.AllowDuplicateNames(true)` |
| Invalid UTF-8 | replaced with U+FFFD | **error** | `jsontext.AllowInvalidUTF8(true)` |
| Field name matching on unmarshal | case-insensitive | **case-sensitive** | `jsonv2.MatchCaseInsensitiveNames(true)` + `json.MatchCaseSensitiveDelimiter(true)` (v2's case-insensitive mode and `case:ignore` also ignore `_` and `-`, which v1 never did) |
| nil slice / nil map | `null` | **`[]` / `{}`** | `jsonv2.FormatNilSliceAsNull(true)` / `FormatNilMapAsNull(true)` |
| `time.Duration` | integer nanoseconds | **error: no default representation** | `json.FormatDurationAsNano(true)` |
| `omitempty` | Go-empty (`0`, `false`, nil ptr, …) | **JSON-empty** (`null`, `""`, `{}`, `[]`) | `json.OmitEmptyWithLegacySemantics(true)`; for new code use `omitzero` |
| HTML characters `<>&` | escaped (`\u003c`) | **not escaped** | `jsontext.EscapeForHTML(true)` |
| Map key order | sorted | **non-deterministic** | `jsonv2.Deterministic(true)` |
| `[N]byte` | JSON array of numbers | **base64 string** | `json.FormatByteArrayAsArray(true)` |
| Array from wrong-length JSON array | accepted | **error** | `json.UnmarshalArrayFromAnyLength(true)` |
| Pointer-receiver `MarshalJSON` on non-addressable value | not called | **called** | `json.CallMethodsWithLegacySemantics(true)` |
| Unknown object members | ignored | ignored | `jsonv2.RejectUnknownMembers(true)` to reject |

```go
// Use v2 with all v1 semantics, then opt into one v2 behaviour:
jsonv2.Marshal(v, json.DefaultOptionsV1(), jsontext.AllowDuplicateNames(false))
```

**Migration review points**

- A `time.Duration` field (or nested type containing one) makes `jsonv2.Marshal` fail at runtime. Pass `json.FormatDurationAsNano(true)`, register a custom marshaler (`jsonv2.WithMarshalers(jsonv2.MarshalToFunc(...))`), or use a string/custom type. Do **not** suggest a `format:` tag option — it was removed and now errors.
- `omitempty` on bool/number/pointer/interface fields changes meaning in v2 — use `omitzero` (identical in v1 and v2).
- Clients that test `== null` for empty lists will see `[]` from v2.
- Rejecting duplicate names is a **security improvement** (it closes parser-differential attacks); don't turn it off without a reason.

##### Struct tag options (v2)

```go
type Event struct {
    Name      string            `json:"name"`
    CreatedAt time.Time         `json:"created_at,omitzero"`   // uses IsZero()
    Tags      []string          `json:"tags,omitempty"`        // omitted if JSON-empty
    Count     int64             `json:"count,string"`          // quoted number
    Kind      string            `json:"kind,case:ignore"`      // case-insensitive match
    Meta      Meta              `json:",embed"`                // flatten into parent
    Extra     jsontext.Value    `json:",embed"`                // fallback: unknown members (one per struct)
    Secret    string            `json:"-"`
}
```

> **The `inline` option was renamed to `embed`, and `format` and `unknown` were removed** during the experiment. v2 **errors** on `format:` but **silently ignores other unrecognised options** — a leftover `json:",inline"` no longer flattens; the field is emitted as a nested object (`{"Inner":{"a":1}}` instead of `{"a":1}`). Grep for `,inline`, `,unknown`, `format:`, `DiscardUnknownMembers` and `SkipFunc` (the latter two were also removed) when moving code written against `GOEXPERIMENT=jsonv2` in 1.25/1.26.

v2 restricts `,string` to values that encode as JSON numbers; in v1 it also quoted strings and bools.

##### jsontext — streaming tokens

```go
dec := jsontext.NewDecoder(r)
for {
    tok, err := dec.ReadToken()
    if err == io.EOF { break }
    if err != nil { return err }
    switch tok.Kind() {
    case jsontext.KindNumber:
        n, err := tok.Int()   // 1.27: numeric accessors return (value, error)
        _ = n; _ = err
    }
}
```

##### encoding/json v1 additions [NEW in 1.27]

`json.UnmarshalTypeError` gains an `Err` field and `Unwrap`; `json.Number` implements v2's `MarshalerTo`/`UnmarshalerFrom`; `json.RawMessage` is now an alias of `jsontext.Value`; `json.Marshaler`/`Unmarshaler` are aliases of the v2 interfaces; `json.Options`/`DefaultOptionsV1` and the legacy-semantics option functions bridge to v2.

---

#### UUIDs — uuid package

##### uuid [NEW in 1.27] ★★

The standard library now has an RFC 9562 `uuid` package. Random bits come from a CSPRNG.

```go
import "uuid"

id := uuid.New()       // currently identical to NewV4
v4 := uuid.NewV4()     // 122 random bits
v7 := uuid.NewV7()     // 48-bit ms timestamp + ≥62 random bits; monotonic unless the clock goes backwards

u, err := uuid.Parse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
// also accepts {…}, urn:uuid:…, 32 hex digits without dashes; any case
u = uuid.MustParse("…")   // panics on bad input — use only for constants

uuid.Nil()   // 00000000-0000-0000-0000-000000000000 (a value, not Go nil)
uuid.Max()   // ffffffff-ffff-ffff-ffff-ffffffffffff
u.Compare(v) // -1/0/+1
u == v       // UUID is [16]byte — comparable, usable as a map key
```

`UUID` implements `encoding.TextMarshaler`/`TextUnmarshaler`/`TextAppender`, so it marshals to JSON as a string and works in `fmt`.

**Guidance**

- **Prefer `uuid.NewV7()` for database primary keys** — time-ordered keys keep B-tree indexes compact. Use `uuid.New()`/`NewV4()` when ordering or embedded creation time would leak information.
- Projects on `go 1.27` can drop `github.com/google/uuid` for generate/parse/compare. Keep it only for features the stdlib lacks (v1/v3/v5/v6 generation, `uuid.Version()` accessors, SQL `Scanner`/`Valuer`).
- `uuid.UUID{}` equals `uuid.Nil()`; validate that IDs taken from input are not Nil when that matters.
- Do not use `MustParse` on user input.

---

#### HTTP Clients & Servers

##### Response bodies are drained on Close [NEW in 1.27]

HTTP/1 `Response.Body.Close()` now reads and discards leftover body bytes, up to a conservative limit (256 KiB / 50 ms in the 1.27 source), so the connection can be reused.

```go
resp, err := client.Get(url)
if err != nil { return err }
defer resp.Body.Close()      // on 1.27, enough for connection reuse for small bodies
// io.Copy(io.Discard, resp.Body)  — no longer needed for typical bodies;
// still useful (with io.LimitReader) if you must reuse connections after large unread bodies
```

Still **always close** the body. If a program shows a slowdown after upgrading, the usual root cause is misconfiguration (e.g. `Transport.MaxIdleConns = 0` or a new `Client`/`Transport` per request); `Transport.DisableKeepAlives = true` turns reuse off.

##### Server limits and priorities [NEW in 1.27]

```go
srv := &http.Server{
    Addr:                ":8080",
    Handler:             mux,
    ReadHeaderTimeout:   5 * time.Second,
    MaxHeaderValueCount: 200,     // default http.DefaultMaxHeaderValueCount = 500
    // DisableClientPriority: true, // HTTP/2: ignore RFC 9218 priority signals (default honours them)
}
```

- `MaxHeaderValueCount` caps the number of header values parsed per request (a comma-separated line counts once; repeated header lines count each time).
- The HTTP/2 server now honours RFC 9218 client priority signals.
- `Transport`/`Server` negotiate ALPN on user-supplied `net.Conn`s that implement `ConnectionState() tls.ConnectionState`.

##### CrossOriginProtection (1.25+)

```go
cop := http.NewCrossOriginProtection()
cop.AddTrustedOrigin("https://app.example.com")
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
// Trailing-slash redirects use 307 since 1.26 (method-preserving).
```

##### httputil.ReverseProxy — use Rewrite

`Director` is deprecated (1.26): a client can make hop-by-hop handling strip headers the proxy added. Use `Rewrite`:

```go
proxy := &httputil.ReverseProxy{
    Rewrite: func(r *httputil.ProxyRequest) {
        r.SetURL(target)
        r.SetXForwarded()
        r.Out.Header.Set("X-Custom", "value")
    },
}
```

##### httptest.NewTestServer — in-memory network [NEW in 1.27] ★

```go
func TestAPI(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        srv := httptest.NewTestServer(t, handler)   // closed automatically via t.Cleanup
        resp, err := srv.Client().Get("http://www.example.com/users/1")
        if err != nil { t.Fatal(err) }
        defer resp.Body.Close()
        // handler's time.Sleep/timeouts run on the bubble's fake clock
    })
}
```

- Uses an in-memory fake network — no loopback sockets; works inside `synctest` bubbles (it also works outside them).
- `srv.URL` is `http://example.com` (set on the first `Client()`/`Start()` call); the client from `srv.Client()` routes **every** host, http or https, to the test server.
- **Pitfall:** only `srv.Client()` is wired to the fake network. `http.Get(srv.URL)`, `http.DefaultClient`, or a client the code builds itself sends the request to the **real** `example.com` over the internet. Inject `srv.Client()` (or its `Transport`) into the code under test.
- A nil handler serves 500s (it does not fall back to `http.DefaultServeMux`).

`httptest.NewServer` keeps listening on loopback; since 1.26 its `Client()` also redirects `example.com` requests to the test server.

##### Proxy and pprof hygiene

- `go tool trace -http=:6060` now binds **localhost only** (like `pprof -http`); use `-http=0.0.0.0:6060` explicitly to expose it.
- Never expose `/debug/pprof/*` (now including `/debug/pprof/goroutineleak`) on a public listener.

---

#### Cryptography

##### crypto/mldsa — post-quantum signatures [NEW in 1.27] ★★

FIPS 204 ML-DSA. Parameter sets `MLDSA44()`, `MLDSA65()`, `MLDSA87()`.

```go
import "crypto/mldsa"

sk, err := mldsa.GenerateKey(mldsa.MLDSA65())
if err != nil { return err }

msg := []byte("release-manifest")
sig, err := sk.Sign(nil, msg, nil)                // rand ignored; nil opts = sign msg directly
err = mldsa.Verify(sk.PublicKey(), msg, sig, nil) // nil on success

// Domain separation with a context string — must match on verify
sig, _ = sk.Sign(nil, msg, &mldsa.Options{Context: "app/v1"})
err = mldsa.Verify(pk, msg, sig, &mldsa.Options{Context: "app/v1"})

seed := sk.Bytes()                                // 32-byte seed (mldsa.PrivateKeySize)
sk2, err := mldsa.NewPrivateKey(mldsa.MLDSA65(), seed)
pk, err := mldsa.NewPublicKey(mldsa.MLDSA65(), sk.PublicKey().Bytes())

det, _ := sk.SignDeterministic(msg, nil)          // deterministic variant
```

- ML-DSA signs the **message itself**. `sk.Sign(nil, msg, crypto.SHA256)` returns `mldsa: invalid SignerOpts` — do not pre-hash as with ECDSA/RSA. `crypto.MLDSAMu` signals an externally computed μ (RFC 9881 "External μ").
- Sizes (bytes): public key 1312/1952/2592; signature 2420/3309/4627 for 44/65/87. Size DB columns and network frames accordingly.
- `*mldsa.PrivateKey` implements `crypto.Signer`. `crypto/x509` parses and verifies ML-DSA keys and certificates (`x509.MLDSA`, `x509.MLDSA44/65/87`); TLS 1.3 supports `tls.MLDSA44/65/87` signature schemes.
- Unavailable (functions return errors) when running against FIPS 140-3 module v1.0.0.

##### crypto/hpke — Hybrid Public Key Encryption (1.26+)

```go
kem, kdf, aead := hpke.MLKEM768X25519(), hpke.HKDFSHA256(), hpke.AES256GCM()

key, err := kem.GenerateKey()                       // recipient
pub, err := kem.NewPublicKey(key.PublicKey().Bytes())
ct, err := hpke.Seal(pub, kdf, aead, []byte("info"), []byte("secret"))
pt, err := hpke.Open(key, kdf, aead, []byte("info"), ct)
```

KEMs: `MLKEM768X25519` (X-Wing, recommended), `MLKEM768P256`, `MLKEM1024P384`, `MLKEM768`, `MLKEM1024`, `DHKEM(ecdh.X25519())`. Prefer HPKE over RSA encryption for new code.

##### Reader-less crypto APIs (1.26+)

Most `crypto/...` functions that take a random `io.Reader` **ignore** it and use the system CSPRNG (`cryptocustomrand=1` reverts). The covered set is: `dsa.GenerateKey`, `ecdh.Curve.GenerateKey`, `ecdsa.GenerateKey`/`SignASN1`/`Sign`, `rsa.GenerateKey`/`GenerateMultiPrimeKey`/`EncryptPKCS1v15`, and `crypto/rand.Prime`. Some functions already ignored the reader (`rsa.DecryptOAEP`, `rsa.SignPKCS1v15`, `mldsa.PrivateKey.Sign`), and `ed25519.GenerateKey(nil)` uses a secure source. Note `ecdsa.PrivateKey.Sign`: a non-nil reader is ignored, but `nil` switches to **deterministic RFC 6979** signatures. Functions that still use the reader — notably `rsa.EncryptOAEP`/`EncryptOAEPWithOptions` and `rsa.SignPSS` — need `crypto/rand.Reader`; do not suggest `nil` there.

```go
key, _ := ecdsa.GenerateKey(elliptic.P256(), nil)
rsaKey, _ := rsa.GenerateKey(nil, 2048)

// Deterministic tests:
func TestSign(t *testing.T) {
    cryptotest.SetGlobalRandom(t, 42)
    key, _ := ecdsa.GenerateKey(elliptic.P256(), nil)
    _ = key
}
```

##### crypto/tls [NEW in 1.27]

```go
cfg := &tls.Config{
    MinVersion:       tls.VersionTLS12,
    CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.MLKEM1024, tls.X25519},
    // Rand: ...  // DEPRECATED in 1.27 — leave nil; use cryptotest.SetGlobalRandom in tests
}
state := conn.ConnectionState()
_ = state.LocalCertificate   // 1.27: chain we presented
```

- `MLKEM1024` (pure post-quantum) key exchange is supported when listed in `CurvePreferences`.
- Hybrid PQ curves listed explicitly in `CurvePreferences` are honoured even with `tlsmlkem=0`/`tlssecpmlkem=0` (those only affect the default set).
- `QUICConfig.ClientHelloInfoConn` sets `ClientHelloInfo.Conn` during QUIC server handshakes.
- `tlsrsakex`, `tls3des`, `tls10server`, `tlsunsafeekm`, `x509keypairleaf` GODEBUGs are **gone**: RSA key exchange, 3DES suites and server-side TLS 1.0/1.1 by default cannot be re-enabled via GODEBUG, and `tls.X509KeyPair`/`LoadX509KeyPair` always populate `Certificate.Leaf`.

##### crypto/x509 [NEW in 1.27]

- `Certificate.RawSignatureAlgorithm` (also on `CertificateRequest`, `RevocationList`) exposes the DER AlgorithmIdentifier, even for `UnknownSignatureAlgorithm`.
- On **Windows and macOS**, `SystemCertPool` now honours `SSL_CERT_FILE`/`SSL_CERT_DIR` and switches to the Go verifier when they are set (`GODEBUG=x509sslcertoverrideplatform=0` reverts; planned removal in 1.31). A stray `SSL_CERT_FILE` in a developer or CI environment now changes trust decisions on those platforms.
- `pkix.Name.String()` renders string-typed values of unknown OIDs as strings instead of hex DER, and parsing into `pkix.Name` accepts more `AttributeTypeAndValue.Value` types, storing unknown ones as `asn1.RawValue` — code that type-asserts these values (e.g. to `string`) can change behaviour.
- `ecdsa.PrivateKey.Sign` validates the digest length when non-nil `SignerOpts` are passed.

##### crypto/rsa

```go
// PKCS #1 v1.5 encryption is deprecated (1.26) — use OAEP
ct, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, msg, nil)
pt, err := rsa.DecryptOAEP(sha256.New(), nil, priv, ct, nil)

// Separate MGF1 hash (1.26+)
ct, err = rsa.EncryptOAEPWithOptions(rand.Reader, pub, msg, &rsa.OAEPOptions{
    Hash:    crypto.SHA256,
    MGFHash: crypto.SHA1,   // legacy interop
})
```

OAEP **encryption still uses its `random` argument**: `rsa.EncryptOAEP(h, nil, …)` panics with a nil-pointer dereference on go1.27.3. Pass `crypto/rand.Reader`.

##### FIPS 140-3

```bash
GODEBUG=fips140=only ./myapp    # strict mode: non-approved algorithms fail
```

```go
fips140.WithoutEnforcement(func() {
    // non-approved algorithms permitted inside (1.26+)
})
fmt.Println(fips140.Version())
```

`GODEBUG=fips140ems=0` (1.27, backported to 1.26.6/1.25.13) disables Extended Master Secret enforcement in FIPS mode.

---

#### Testing & Benchmarking

##### go test runs the stdversion check [NEW in 1.27] ★

`go test` now includes the `stdversion` vet analyzer, which reports standard-library symbols newer than the file's Go version. **`go build` still succeeds**, so this is often the first place the error shows up:

```
# go.mod: go 1.25
./a.go:5:60: strings.CutLast requires go1.27 or later (module is go1.25)
FAIL    sv [build failed]
```

Fix by raising the `go` directive or guarding the file with `//go:build go1.27`. When reviewing a library that supports older Go versions, check that new 1.27 symbols (`strings.CutLast`, `uuid`, `encoding/json/v2`, `net/url.(*URL).Clone`, …) are not used below `go 1.27`.

##### go test -json output types [NEW in 1.27]

`"Action":"output"` events may carry `"OutputType"` (`"error"`, `"error-continue"`, `"frame"`) so tools can identify failure messages without parsing text.

##### T.ArtifactDir (1.26+)

```go
func TestGenerateReport(t *testing.T) {
    dir := t.ArtifactDir()   // kept with `go test -artifacts`, temp otherwise
    if err := os.WriteFile(filepath.Join(dir, "report.html"), report, 0o644); err != nil {
        t.Fatal(err)
    }
}
```

##### B.Loop (1.24+, inlining fixed in 1.26)

```go
func BenchmarkProcess(b *testing.B) {
    for b.Loop() {
        _ = process(data)   // results kept alive; inlining works since 1.26
    }
}
```

##### Testing HTTP with the fake network (1.27)

Prefer `httptest.NewTestServer` inside `synctest.Test` for handler tests involving timeouts, retries or slow clients — they run instantly and deterministically. Always use `srv.Client()`.

##### Test helpers

Helper functions should call `t.Helper()` so failures report the caller's line.

```go
func newTestUser(t *testing.T) *User {
    t.Helper()
    return &User{ID: uuid.NewV7().String(), Name: "Test User", Email: "test@example.com"}
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
available in Go 1.27: `t.Helper()`, `t.TempDir()`, `t.Setenv()`, `t.Cleanup()`,
`t.Context()`/`t.Chdir()` (1.24+), `t.Attr()`/`t.Output()` (1.25+),
`t.ArtifactDir()` (1.26+), and `synctest.Sleep` (1.27+).

##### Table-driven tests (1.22+: no shadow needed)

```go
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        got, err := Process(tt.input)
        if !errors.Is(err, tt.wantErr) { t.Fatalf("err=%v", err) }
        if got != tt.want { t.Errorf("got %v want %v", got, tt.want) }
    })
}
```

##### go vet — cumulative analysers

| Version | Analyser | What it catches |
|---|---|---|
| 1.22 | `slog` | mismatched key/value pairs |
| 1.23 | `stdversion` | symbols newer than go.mod declares (**run by `go test` since 1.27**) |
| 1.24 | `tests` | malformed test/benchmark names |
| 1.24 | `printf` | `fmt.Printf(s)` with no args |
| 1.25 | `waitgroup` | `wg.Add` inside goroutine |
| 1.25 | `hostport` | `Sprintf("%s:%d")` for network addr |
| 1.27 | `printf` | `%w` operand of type `*E` where `E` implements `error` |

---

#### CLI Development & Toolchain

##### go fix modernizers (1.26+, extended in 1.27) ★★

```bash
go fix ./...          # apply all modernizers
go fix -diff ./...    # preview
go tool fix help      # list analyzers
```

New in 1.27:

| Analyzer | Rewrites |
|---|---|
| `atomictypes` | `x int64` + `atomic.AddInt64(&x, 1)` → `x atomic.Int64` + `x.Add(1)` |
| `embedlit` | `T{Base: Base{Name: "x"}}` → `T{Name: "x"}` (needs `go 1.27`) |
| `slicesbackward` | `for i := len(s)-1; i >= 0; i--` → `for i, v := range slices.Backward(s)` |
| `unsafefuncs` | `unsafe.Pointer(uintptr(p) + uintptr(n))` → `unsafe.Add(p, n)` |

Also in 1.27: the `fmtappendf` analyzer was **removed**, and `waitgroup` was **renamed `waitgroupgo`**. Scripts passing `-fmtappendf` or `-waitgroup` to `go fix` now fail with `flag provided but not defined`. Other fixers include `any`, `errorsastype`, `forvar`, `inline` (`//go:fix inline`), `mapsloop`, `minmax`, `newexpr`, `omitzero`, `rangeint`, `reflecttypefor`, `slicescontains`, `slicessort`, `stditerators`, `stringsbuilder`, `stringscut`, `stringscutprefix`, `stringsseq`, `testingcontext`.

Review modernizer diffs like any other change — e.g. `atomictypes` changes a struct field's type, which affects copying and exported APIs.

##### gofmt alignment fix [NEW in 1.27]

gofmt's column-alignment logic was fixed (it previously varied slightly by CPU architecture). Running Go 1.27 gofmt on already-formatted code can produce **small whitespace-only diffs** in aligned comments/fields. Treat these as formatter churn, not as a review finding; CI that pins an older gofmt may disagree with 1.27 output.

##### Response files for low-level tools [NEW in 1.27]

`compile`, `link`, `asm`, `cgo`, `cover`, and `pack` accept GCC-compatible `@file` response files (relevant to Bazel/build-system integrations with very long command lines).

##### //line directives resolve relative paths [NEW in 1.27]

The compiler now resolves a relative filename in a `//line` or `/*line*/` directive against the directory of the file containing the directive (matching `go/scanner`). Code generators that emitted paths relative to the working directory may now produce wrong positions in errors, coverage and stack traces; emit absolute or file-relative paths.

##### Other 1.27 API additions

- `math/big.(*Int).Divide(x, y, r, mode)` with rounding modes `big.Trunc`, `Floor`, `Round` (to nearest even), `Ceil`: `Divide(-7, 2, …, Floor)` → q=-4, r=1; `Trunc` → q=-3, r=-1.
- `database/sql.ConvertAssign` and `driver.RowsColumnScanner` let drivers reuse `Rows.Scan` conversions and scan directly into destinations (driver authors only).
- `go/types.Hasher`/`HasherIgnoreTags`, `go/constant.StringLen`, `go/scanner.(*Scanner).End`, `go/token.(*File).String` for tool authors.
- Linker `-macos`/`-macsdk` flags set `LC_BUILD_VERSION` (defaults: macOS 13.0.0, SDK 26.2.0); linux/ppc64 (big-endian) now uses ELFv2 and supports cgo, PIE and external linking; Plan 9 defines `syscall.Errno`.

##### Tool directive in go.mod (1.24+)

```bash
go get -tool golang.org/x/tools/cmd/stringer@latest
go tool stringer -type=Color .
```

##### flag

```go
host := flag.String("host", "localhost", "server host")
port := flag.Int("port", 8080, "server port")
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

#### GODEBUG Removals

##### Removed settings [NEW in 1.27] ★★

| Setting | Added | Old value now rejected | Behaviour now permanent |
|---|---|---|---|
| `asynctimerchan` | 1.23 | `1` (and `2`) | timer/ticker channels unbuffered |
| `gotypesalias` | 1.22 | `0` | `go/types` always produces `*types.Alias` |
| `tlsrsakex` | 1.22 | `1` | no RSA key-exchange suites by default |
| `tls10server` | 1.22 | `1` | server minimum TLS 1.2 by default |
| `tlsunsafeekm` | 1.22 | `1` | EKM requires EMS or TLS 1.3 |
| `tls3des` | 1.23 | `1` | no 3DES suites by default |
| `x509keypairleaf` | 1.23 | `0` | `X509KeyPair` always sets `Leaf` |

(`tlskyber` and `x509sha1` were removed in 1.24.)

**What happens when an old value is still set (verified on go1.27.3):**

```bash
# go.mod: godebug asynctimerchan=1  →  build fails
go: error loading go.mod: go.mod:5: removed GODEBUG "asynctimerchan" set to old value "1" (https://go.dev/doc/godebug#go-127)

# //go:debug asynctimerchan=1  →  compile error
./main.go:1:1: invalid //go:debug: removed GODEBUG "asynctimerchan" set to old value "1" (https://go.dev/doc/godebug#go-127)

# GODEBUG=asynctimerchan=1 in the ENVIRONMENT  →  the program aborts at startup
fatal error: removed GODEBUG "asynctimerchan" set to old value "1" in environment (https://go.dev/doc/godebug#go-127)
```

The final default value (e.g. `asynctimerchan=0`, `x509keypairleaf=1`) is still accepted and is a no-op, so existing go.mod files that pinned the new behaviour keep building.

> **Deployment check:** before rolling out a binary built with Go 1.27, search Dockerfiles, Kubernetes manifests, systemd units, CI configs, Helm values and `.env` files for `GODEBUG=` containing any setting above with its old value. The binary will **crash on start** — the general rule that unknown `GODEBUG` env settings are ignored does not apply to removed settings set to an old value. This hits **every** Go 1.27 binary, even one that never uses the affected package (`GODEBUG=gotypesalias=0` aborts a program that does not import `go/types`).

##### New and changed settings [NEW in 1.27]

- `tracebacklabels` now defaults to `1` for `go 1.27` modules.
- `x509sslcertoverrideplatform` (default `1`) — see crypto/x509.
- `httpservecontentmaxranges` (default `200`, backported to 1.27.2 and 1.26.9) — `ServeContent`/`ServeFile`/`FileServer` ignore a `Range` header with more ranges than this; `0` disables the limit.
- `htmlmetacontenturlescape` (default `1`, backported to 1.25.8/1.26.1) — html/template escapes URLs in `<meta content="…;url=…">`.
- `fips140ems` — see FIPS.

---

#### Performance Caveats, SIMD & PGO

##### Size-specialised small allocations [NEW in 1.27] ★

The compiler calls size-specialised allocation routines, making some small (<80 byte) allocations up to **30% cheaper** (~1% overall in allocation-heavy programs). Cost: about **60 KB** larger binaries. Opt out with `GOEXPERIMENT=nosizespecializedmalloc` (expected to be removed in 1.28).

##### Green Tea GC (default since 1.26)

10–40% lower GC overhead for GC-heavy programs (more on AVX-512 CPUs). The `GOEXPERIMENT=nogreenteagc` opt-out was announced for removal in 1.27 but is **still accepted and functional in go1.27.3**; do not rely on it long-term.

##### Experimental portable SIMD — simd package [NEW in 1.27]

Enabled with `GOEXPERIMENT=simd`. Vector-size-agnostic types (`Int8s`, `Float32s`, …, length ≥128 bits, fixed per execution); uses hardware SIMD where available, otherwise emulation. Available on all architectures.

```go
//go:build goexperiment.simd

import "simd"

func Add(dst, a, b []float32) {
    for i := 0; i < len(a); {
        va, n := simd.LoadFloat32sPart(a[i:])
        vb, _ := simd.LoadFloat32sPart(b[i:])
        va.Add(vb).StorePart(dst[i:])
        i += n
    }
}
```

Without the experiment, `import "simd"` fails (`build constraints exclude all Go files`) — guard such files with `//go:build goexperiment.simd` and keep a scalar fallback. `simd/archsimd` (architecture-specific) now also covers arm64 Neon and wasm 128-bit; its amd64 API changed in 1.27. Neither API is covered by the Go 1 compatibility promise yet.

##### compress/flate output changed [NEW in 1.27]

Faster DEFLATE compression, but **compressed bytes differ** from 1.26 — and so do `compress/gzip`, `compress/zlib`, `archive/zip`, and `image/png` outputs. Decompression results are identical. Tests comparing compressed output against golden files, or content hashes/ETags/cache keys computed over compressed bytes, need updating; compare decompressed content instead.

##### Other carry-forward performance notes

- cgo call overhead ~30% lower (1.26).
- Heap base address randomised on 64-bit (1.26; `GOEXPERIMENT=norandomizedheapbase64` opts out).
- Stack-allocated slice backing stores in more cases (1.25/1.26).
- `image/jpeg` encoder/decoder replaced (1.26) — bit-exact output differs.
- Container-aware `GOMAXPROCS` (1.25+) — remove `automaxprocs`.

##### PGO (GA since 1.21)

```bash
curl -o cmd/myapp/default.pgo "http://localhost:6060/debug/pprof/profile?seconds=30"
go build ./cmd/myapp   # default.pgo in the main package dir is picked up automatically
```

---

#### Observability: Tracing, Profiling & Goroutine Leaks

##### Goroutine leak profile — generally available [NEW in 1.27] ★★

No `GOEXPERIMENT` required (the `goroutineleakprofile` experiment is deleted — setting it is now an error: `unknown GOEXPERIMENT goroutineleakprofile`).

```go
import "runtime/pprof"

pprof.Lookup("goroutineleak").WriteTo(os.Stdout, 1)
// goroutineleak profile: total 1
// #   main.leak.func1+0x1d   /app/main.go:14
```

```bash
curl http://localhost:6060/debug/pprof/goroutineleak   # via net/http/pprof
```

Detection is GC-based: a goroutine blocked on a channel/mutex/cond that is **unreachable** from any runnable goroutine is reported. Leaks on primitives reachable from **globals** or from locals of runnable goroutines are not detected — a clean profile is not proof of no leaks.

```go
// Classic leak: early return leaves senders blocked forever
func first(items []Item) (Result, error) {
    ch := make(chan result)                 // unbuffered
    for _, it := range items {
        go func() { ch <- process(it) }()
    }
    r := <-ch                               // remaining senders leak
    return r.res, r.err
}

// Fix: buffer for every sender (or cancel via context)
ch := make(chan result, len(items))
```

##### Traceback labels (1.27)

See [Defer, Panic & Recover](#tracebacks-include-goroutine-labels-new-in-127): labels appear in crash output for `go 1.27` modules.

##### Goroutine metrics (1.26+)

```go
samples := []metrics.Sample{
    {Name: "/sched/goroutines-created:goroutines"},
    {Name: "/sched/goroutines/runnable:goroutines"},
    {Name: "/sched/goroutines/running:goroutines"},
    {Name: "/sched/goroutines/waiting:goroutines"},
    {Name: "/sched/goroutines/not-in-go:goroutines"},
    {Name: "/sched/threads/total:threads"},
}
metrics.Read(samples)
```

##### FlightRecorder (1.25+)

```go
fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{
    MinAge: 5 * time.Second, MaxBytes: 10 << 20,
})
if err := fr.Start(); err != nil { log.Fatal(err) }
defer fr.Stop()
// On anomaly: fr.WriteTo(outFile)
```

##### pprof

`go tool pprof -http=:6060` opens the flame graph by default (1.26).

---

#### Idioms & Things to Avoid

##### Do (1.27 additions)

- **Use generic methods** for type-scoped generic operations (`list.Map`, `store.GetAs[T]`) — but keep interface-facing methods non-generic.
- **Use promoted-field keys** (`Resource{Name: "web", Replicas: 3}`) to flatten literals of embedded structs; run `go fix` (`embedlit`).
- **Use `strings.CutLast` / `bytes.CutLast`** instead of `LastIndex` + slicing.
- **Use `uuid.NewV7()`** for sortable IDs and `uuid.New()` for opaque ones; drop third-party UUID libraries where the stdlib suffices.
- **Use `encoding/json/v2`** for new code; use `omitzero`, an explicit `time.Duration` representation, and `,embed` (not `,inline`).
- **Use `url.URL.Clone()` / `url.Values.Clone()`** instead of struct copies.
- **Use `httptest.NewTestServer` + `synctest`** for time-dependent HTTP tests, with `srv.Client()`.
- **Use `synctest.Sleep`** instead of `time.Sleep` + `synctest.Wait`.
- **Use `crypto/mldsa`** where post-quantum signatures are required; sign raw messages.
- **Use the `goroutineleak` profile** in staging/production diagnostics — no build flag needed.
- **Use `errors.AsType[T]`** and `new(expr)` (1.26) everywhere they apply.
- **Run `go fix ./...`** after upgrading and review the diff.
- **Scrub deployment configs** for removed `GODEBUG` settings before rolling out 1.27 binaries.

##### Don't

| Anti-pattern | Why | Instead |
|---|---|---|
| Generic method in an interface | Not allowed | Concrete interface method + generic helper |
| Expecting a generic method to satisfy an interface | Never does | Keep a non-generic method |
| `Resource{Meta: m, Name: "x"}` | Overlapping keys — compile error | Set one or the other |
| Promoted key through embedded `*T` | Compile error | Initialise the pointer field explicitly |
| Generic-func inference in literals with `go 1.26` in go.mod | Compiles on 1.27, breaks on 1.26 | Require `go 1.27` or instantiate explicitly |
| `strings.LastIndex` + manual slicing | Verbose, off-by-one prone | `strings.CutLast` |
| `CutLast(addr, ":")` for host/port | Keeps IPv6 brackets | `net.SplitHostPort` |
| `json:",inline"` with v2 | Silently ignored — no flattening | `json:",embed"` |
| `time.Duration` field marshalled with v2 | Runtime error | `json.FormatDurationAsNano(true)` or a custom marshaler — not a `format:` tag |
| `omitempty` on bool/int/pointer for v2 | Semantics changed | `omitzero` |
| String-matching `encoding/json` error text | Messages changed in 1.27 | `errors.As`/`errors.AsType` on error types |
| Copying `url.Values` by assignment or a range loop | Shares the `[]string` slices | `vals.Clone()` (1.27) |
| `http.Get(srv.URL)` with `NewTestServer` | Hits the real example.com | `srv.Client().Get(...)` |
| `GODEBUG=asynctimerchan=1` (or other removed setting with old value) | Binary aborts at startup | Remove it |
| `if !t.Stop() { <-t.C }` before `Reset` | Redundant since 1.23 (drain never runs; blocks if value already received) | Just `t.Reset(d)` |
| `sk.Sign(nil, msg, crypto.SHA256)` with ML-DSA | Error — ML-DSA signs the message | `nil` or `*mldsa.Options` |
| Comparing closure code pointers | More sharing in 1.27 | Compare explicit identifiers |
| Golden files of gzip/zlib/zip/png bytes | Encoder output changed | Compare decompressed content |
| Secrets in `pprof.Labels` | Printed in tracebacks | Non-sensitive labels only |
| `tls.Config.Rand` | Deprecated | `cryptotest.SetGlobalRandom` in tests |
| `GOEXPERIMENT=goroutineleakprofile` | Now an error | Nothing — profile is GA |
| `ptr[T](v)` helper | Obsolete since 1.26 | `new(v)` |
| `var ve *T; errors.As(err, &ve)` | Verbose, leaks scope | `errors.AsType[*T](err)` |
| `rsa.EncryptPKCS1v15` | Deprecated, unsafe | `rsa.EncryptOAEP` |
| `Director` in `ReverseProxy` | Deprecated | `Rewrite` |
| `fmt.Sprintf("%s:%d", h, p)` | IPv6 unsafe | `net.JoinHostPort(h, strconv.Itoa(p))` |
| `wg.Add(1)` + `go` + `defer wg.Done()` | Boilerplate | `wg.Go(f)` |
| `runtime.SetFinalizer` for new code | One per object, resurrection issues | `runtime.AddCleanup` |
| `tools.go` blank import | Obsolete since 1.24 | `tool` directive |
| `for range b.N` / `for i := 0; i < b.N; i++` | Old style | `for b.Loop()` |
| `time.Sleep` in tests | Non-deterministic | `testing/synctest` |
| `rand.Seed()` | No-op since 1.24 | `rand.New(rand.NewPCG(s1, s2))` |
| `strings.HasPrefix(path, base)` as a containment check | `/base` also prefixes `/baseball` | `filepath.Rel` + reject a `..` result |
| `strings.Trim(s, ".ext")` to drop a suffix | Cutset — strips any of those runes from both ends | `strings.TrimSuffix` |

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

Before reporting a path traversal, check what the joined path is actually
exposed to.

- `filepath.Join(base, userPath)` does not escape `base` when `userPath` is
  absolute — only `..` elements escape. Do not report the absolute-element case.
- `http.ServeFile` and `http.ServeFileFS` reject any request whose `r.URL.Path`
  contains a `..` element, so a `Join` fed from `r.URL.Path` or `r.PathValue` is
  already guarded. That guard does not extend to query parameters, headers, form
  fields, or to `http.ServeContent`.
- Report traversal when a `..`-bearing path from an unguarded source reaches the
  filesystem, when containment is checked with `strings.HasPrefix` instead of
  `filepath.Rel`, or when a lexical check stands in for symlink resolution on a
  directory an attacker can write to.

JSON parser differentials: v2 rejects duplicate names and invalid UTF-8 by default. Case-insensitive matching (v1 default; v2 `MatchCaseInsensitiveNames`/`case:ignore`, which also ignore `_`/`-`) is another differential vector. Flag code that re-enables `AllowDuplicateNames`/`AllowInvalidUTF8`/case-insensitive matching (or keeps v1) on a request path where another component also parses the same JSON for authorization decisions.

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
go fix ./...   # modernizers (1.26+, more in 1.27)
go vet ./...
staticcheck ./...
golangci-lint run
```

A `golangci-lint` v2 configuration (the tool version is independent of the Go
language version, but it must be **built with Go ≥ 1.27** to load code that
uses 1.27 language features such as generic methods; pin and run it via a
`tool` directive with `go tool golangci-lint run`):

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

*End of Go 1.27 Complete Developer Guideline*
