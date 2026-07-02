### Go Style Guide

Go idioms and conventions for clean, maintainable code.

#### gofmt and Standard Formatting

##### Always Use gofmt

```bash
# Format a single file
gofmt -w file.go

# Format entire project
gofmt -w .

# Use goimports for imports management
goimports -w .
```

##### Formatting Rules (Enforced by gofmt)

- Tabs for indentation
- No trailing whitespace
- Consistent brace placement
- Standardized spacing

#### Version-Aware Go

Check the module's `go` directive before reviewing language semantics or
suggesting newer APIs. The `go` directive controls language version behavior;
the installed toolchain may be newer than the module target.

```go
module example.com/app

go 1.24.0

tool github.com/golangci/golangci-lint/cmd/golangci-lint
```

For Go 1.24 and newer modules, use `tool` directives in `go.mod` to track
executable development tools instead of blank-import `tools.go` files.

Verify library APIs against the actual module versions in `go.mod` before
claiming an API is missing or unavailable.

Examples:
- Do not claim a controller-runtime helper is unavailable without checking the
  pinned `sigs.k8s.io/controller-runtime` version.
- Say "this API is not available in vX.Y.Z" only when the module version proves
  it.
- Do not infer API availability from memory, a newer version's docs, or another
  project's dependency set.

##### Generics

Use generics when they remove duplication across real types or express a
container/helper API. Prefer ordinary functions and interfaces when there is
only one concrete caller or behavior differs by type.

```go
func Keys[K comparable, V any](m map[K]V) []K {
    keys := make([]K, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    return keys
}
```

Keep constraints small and local when possible. Use `any` instead of
`interface{}` for unconstrained type parameters.

##### Standard Library Helpers

Prefer standard library helpers over local generic utilities when they match
the need:

- `slices` for sorting, searching, cloning, compacting, and comparing slices.
- `maps` for cloning, copying, deleting, and iterating map contents.
- `cmp` for ordered comparisons.
- `clear` for clearing maps or zeroing slice contents.
- `min` and `max` for simple ordered comparisons.

##### Map Iteration Semantics

Deleting map entries while ranging over the same map is allowed in Go. Do not
flag code like this as a bug by itself:

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

##### Reachability and Domain Bounds

Only report overflow, panic, or invalid-value behavior when the failing path is
reachable for the declared type and the function's input domain.

Examples:
- Do not claim byte formatting can index into ZiB/YiB units when an `int64`
  input cannot grow large enough to reach those unit indexes.
- Do not require negative byte handling when all callers pass values from
  `os.FileInfo.Size()` or another source with a non-negative contract.
- Do report missing negative handling when an exported/general-purpose function
  accepts user-controlled values and documents no narrower domain.
- Do not require defensive code for values that cannot be represented by the
  input type.

#### Error Handling

##### Explicit Error Checking

```go
// Always check errors explicitly
file, err := os.Open(filename)
if err != nil {
    return fmt.Errorf("opening file %s: %w", filename, err)
}
defer file.Close()

// Don't ignore errors with _
// Bad
data, _ := json.Marshal(obj)

// Good
data, err := json.Marshal(obj)
if err != nil {
    return nil, fmt.Errorf("marshaling object: %w", err)
}
```

##### Error Wrapping

```go
// Use %w to wrap errors for unwrapping later
func processFile(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("reading file %s: %w", path, err)
    }

    if err := validate(data); err != nil {
        return fmt.Errorf("validating data: %w", err)
    }

    return nil
}

// Check wrapped errors
if errors.Is(err, os.ErrNotExist) {
    // Handle file not found
}

var validationErr *ValidationError
if errors.As(err, &validationErr) {
    // Handle validation error
}
```

##### Multiple Errors

Use `errors.Join` or multiple `%w` operands when an operation can fail in more
than one independent way and callers should still be able to use `errors.Is`
or `errors.As`.

```go
var errs []error
if err := closer.Close(); err != nil {
    errs = append(errs, fmt.Errorf("closing file: %w", err))
}
if err := cleanup(); err != nil {
    errs = append(errs, fmt.Errorf("cleaning up: %w", err))
}
return errors.Join(errs...)
```

##### Custom Error Types

```go
// Sentinel errors for expected conditions
var (
    ErrNotFound     = errors.New("resource not found")
    ErrUnauthorized = errors.New("unauthorized access")
    ErrInvalidInput = errors.New("invalid input")
)

// Custom error type with additional context
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// Error constructor
func NewValidationError(field, message string) error {
    return &ValidationError{Field: field, Message: message}
}
```

#### Interfaces

##### Small, Focused Interfaces

```go
// Good: Single-method interface
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

// Compose interfaces
type ReadWriter interface {
    Reader
    Writer
}

// Bad: Large interfaces
type Repository interface {
    Find(id string) (*User, error)
    FindAll() ([]*User, error)
    Create(user *User) error
    Update(user *User) error
    Delete(id string) error
    FindByEmail(email string) (*User, error)
    // Too many methods - hard to implement and test
}
```

##### Accept Interfaces, Return Structs

```go
// Good: Accept interface, return concrete type
func NewUserService(repo UserRepository) *UserService {
    return &UserService{repo: repo}
}

// Interface defined by consumer
type UserRepository interface {
    Find(ctx context.Context, id string) (*User, error)
    Save(ctx context.Context, user *User) error
}

// Concrete implementation
type PostgresUserRepo struct {
    db *sql.DB
}

func (r *PostgresUserRepo) Find(ctx context.Context, id string) (*User, error) {
    // Implementation
}
```

##### Interface Naming

```go
// Single-method interfaces: method name + "er"
type Reader interface { Read(p []byte) (n int, err error) }
type Writer interface { Write(p []byte) (n int, err error) }
type Closer interface { Close() error }
type Stringer interface { String() string }

// Multi-method interfaces: descriptive name
type UserStore interface {
    Get(ctx context.Context, id string) (*User, error)
    Put(ctx context.Context, user *User) error
}
```

#### Package Structure

##### Standard Layout

```
myproject/
├── cmd/
│   └── myapp/
│       └── main.go         # Application entry point
├── internal/
│   ├── auth/
│   │   ├── auth.go
│   │   └── auth_test.go
│   ├── user/
│   │   ├── user.go
│   │   ├── repository.go
│   │   └── service.go
│   └── config/
│       └── config.go
├── pkg/                    # Public packages (optional)
│   └── api/
│       └── client.go
├── go.mod
├── go.sum
└── README.md
```

##### Package Guidelines

```go
// Package names: short, lowercase, no underscores
package user      // Good
package userService // Bad
package user_service // Bad

// Package comment at top of primary file
// Package user provides user management functionality.
package user

// Group imports: stdlib, external, internal
import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/lib/pq"

    "myproject/internal/config"
)
```

##### Internal Packages

```go
// internal/ packages cannot be imported from outside the module
// Use for implementation details you don't want to expose

// myproject/internal/cache/cache.go
package cache

// This can only be imported by code in myproject/
```

#### Testing

##### Test File Organization

```go
// user_test.go - same package
package user

import (
    "testing"
)

func TestUserValidation(t *testing.T) {
    // Test implementation details
}

// user_integration_test.go - external test package
package user_test

import (
    "testing"

    "myproject/internal/user"
)

func TestUserService(t *testing.T) {
    // Test public API
}
```

##### Table-Driven Tests

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive numbers", 2, 3, 5},
        {"negative numbers", -1, -1, -2},
        {"mixed numbers", -1, 5, 4},
        {"zeros", 0, 0, 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Add(tt.a, tt.b)
            if result != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d",
                    tt.a, tt.b, result, tt.expected)
            }
        })
    }
}
```

##### Test Helpers

```go
// Helper functions should call t.Helper()
func newTestUser(t *testing.T) *User {
    t.Helper()
    return &User{
        ID:    uuid.New().String(),
        Name:  "Test User",
        Email: "test@example.com",
    }
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

##### Test Context and Cleanup

Use `t.Context()` in Go 1.24 and newer when code under test needs a context
that is canceled before registered cleanup functions run.

```go
func TestWorkerStops(t *testing.T) {
    worker := NewWorker()
    go worker.Run(t.Context())

    t.Cleanup(func() {
        worker.Wait()
    })
}
```

Use `t.TempDir()`, `t.Setenv()`, `t.Chdir()`, and `t.Cleanup()` instead of
manual cleanup where available.

##### Fuzzing

Add fuzz tests for parsers, decoders, validators, and other code that accepts
complex external input.

```go
func FuzzParse(f *testing.F) {
    f.Add("valid input")
    f.Fuzz(func(t *testing.T, input string) {
        _, _ = Parse(input)
    })
}
```

##### Mocking with Interfaces

```go
// Define interface for dependency
type UserRepository interface {
    Find(ctx context.Context, id string) (*User, error)
    Save(ctx context.Context, user *User) error
}

// Mock implementation for testing
type mockUserRepo struct {
    users map[string]*User
}

func newMockUserRepo() *mockUserRepo {
    return &mockUserRepo{users: make(map[string]*User)}
}

func (m *mockUserRepo) Find(ctx context.Context, id string) (*User, error) {
    user, ok := m.users[id]
    if !ok {
        return nil, ErrNotFound
    }
    return user, nil
}

func (m *mockUserRepo) Save(ctx context.Context, user *User) error {
    m.users[user.ID] = user
    return nil
}

// Test using mock
func TestUserService_GetUser(t *testing.T) {
    repo := newMockUserRepo()
    repo.users["123"] = &User{ID: "123", Name: "Test"}

    service := NewUserService(repo)
    user, err := service.GetUser(context.Background(), "123")

    assertNoError(t, err)
    assertEqual(t, user.Name, "Test")
}
```

#### Common Patterns

##### Range Loop Semantics

Go 1.22 changed loop variables declared by `for` loops. When a loop uses `:=`,
each iteration gets fresh variables, so closures and goroutines capture the
iteration's values instead of one shared variable.

```go
// Go 1.22+: safe without v := v inside the loop.
for _, v := range values {
    go func() {
        process(v)
    }()
}

// Also applies to maps.
for k, v := range m {
    callbacks = append(callbacks, func() {
        log.Printf("%s=%v", k, v)
    })
}
```

Do not add manual loop-variable copies in modules that require Go 1.22 or
newer unless the copy has another purpose. In modules targeting older Go
versions, keep the copy before capturing loop variables:

```go
for _, v := range values {
    v := v
    go func() {
        process(v)
    }()
}
```

Map range behavior remains intentionally unspecified:

- Iteration order is not stable.
- Deleting an entry before it is reached means it will not be produced.
- Adding an entry during iteration may or may not produce it.
- Concurrent map reads and writes are unsafe without synchronization.

Prefer collecting and sorting keys when deterministic map output is required:

```go
keys := make([]string, 0, len(m))
for k := range m {
    keys = append(keys, k)
}
slices.Sort(keys)

for _, k := range keys {
    fmt.Println(k, m[k])
}
```

Go 1.22 also added integer ranges:

```go
for i := range 10 {
    fmt.Println(i)
}
```

Go 1.23 added range over iterator functions. Use it for custom collections
when it makes iteration clearer than exposing internal storage:

```go
func (s *Set[E]) All() iter.Seq[E] {
    return func(yield func(E) bool) {
        for v := range s.m {
            if !yield(v) {
                return
            }
        }
    }
}

for v := range set.All() {
    fmt.Println(v)
}
```

##### Options Pattern

```go
// Option function type
type ServerOption func(*Server)

// Option functions
func WithPort(port int) ServerOption {
    return func(s *Server) {
        s.port = port
    }
}

func WithTimeout(timeout time.Duration) ServerOption {
    return func(s *Server) {
        s.timeout = timeout
    }
}

func WithLogger(logger *slog.Logger) ServerOption {
    return func(s *Server) {
        s.logger = logger
    }
}

// Constructor using options
func NewServer(opts ...ServerOption) *Server {
    s := &Server{
        port:    8080,           // defaults
        timeout: 30 * time.Second,
        logger:  slog.Default(),
    }

    for _, opt := range opts {
        opt(s)
    }

    return s
}

// Usage
server := NewServer(
    WithPort(9000),
    WithTimeout(time.Minute),
)
```

##### Context Usage

```go
// Always pass context as first parameter
func (s *Service) ProcessRequest(ctx context.Context, req *Request) (*Response, error) {
    // Check for cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Use context for timeouts
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    result, err := s.repo.Find(ctx, req.ID)
    if err != nil {
        return nil, fmt.Errorf("finding item: %w", err)
    }

    return &Response{Data: result}, nil
}
```

Use cancellation causes when callers need to distinguish why a context ended.

```go
ctx, cancel := context.WithCancelCause(parent)
defer cancel(nil)

if err := work(ctx); err != nil {
    cancel(fmt.Errorf("worker failed: %w", err))
}

if cause := context.Cause(ctx); cause != nil {
    return cause
}
```

##### Structured Logging

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

Avoid logging secrets, tokens, credentials, or full request/response bodies
unless they are explicitly scrubbed.

##### Defer for Cleanup

```go
func processFile(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return err
    }
    defer file.Close()  // Always executed on return

    // Process file...
    return nil
}

// Multiple defers execute in LIFO order
func transaction(db *sql.DB) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback() // Safe: no-op if committed

    // Do work...

    return tx.Commit()
}
```

##### Concurrency Patterns

```go
// Worker pool
func processItems(items []Item, workers int) []Result {
    jobs := make(chan Item, len(items))
    results := make(chan Result, len(items))

    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for item := range jobs {
                results <- process(item)
            }
        }()
    }

    // Send jobs
    for _, item := range items {
        jobs <- item
    }
    close(jobs)

    // Wait and collect
    go func() {
        wg.Wait()
        close(results)
    }()

    var out []Result
    for r := range results {
        out = append(out, r)
    }
    return out
}
```

#### Code Quality

##### Linting with golangci-lint

```yaml
# .golangci.yml
linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gosimple
    - gocritic
    - gofmt
    - goimports

linters-settings:
  govet:
    check-shadowing: true
  errcheck:
    check-type-assertions: true

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
```

##### Common Commands

```bash
# Format code
go fmt ./...

# Run linter
golangci-lint run

# Run tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Check for race conditions
go test -race ./...

# Build
go build ./...
```

##### Security

Weak hashes are only security-relevant when the hash is used for a security
property such as authentication, authorization, integrity, signatures, password
storage, or collision resistance against attacker-controlled input.

For content-addressed storage, distinguish integrity/security boundaries from
ordinary maintenance behavior.

Examples:
- Do not require hash verification on every listing unless the listing crosses
  a trust boundary or an attacker can write to the store.
- Delete-before-regenerate can be correct when replacing corrupt same-digest
  content and the store skips writes for content that already exists.
- Report content-addressed storage issues when untrusted data can be accepted
  under the wrong digest, when corruption is silently trusted as valid content,
  or when the repair order can lose the only valid copy.

##### Races

Separate true data races from lifecycle, shutdown, or ordering races. Only call
something a data race after confirming unsynchronized shared-memory access with
at least one write.

Check a library's concurrency contract before assuming concurrent method calls
are unsafe. Some Go types are explicitly safe for concurrent use, while others
require caller-side synchronization.
