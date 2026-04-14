# NickPit (llm-review) — Full Implementation Plan

## 1. Project Overview

**NickPit** is a production-ready Go CLI tool that provides AI-powered code review via any OpenAI-compatible API. It operates in three modes: **local** (git diffs), **GitHub** (pull requests), and **GitLab** (merge requests). It uses a two-pass review strategy: an initial review pass followed by optional LLM-driven retrieval passes for deeper context.

---

## 2. Repository Structure

```
nickpit/
├── cmd/
│   └── llm-review/
│       └── main.go                    # Cobra entrypoint, wires all subcommands
├── internal/
│   ├── config/
│   │   ├── config.go                  # Config struct, YAML loader, env overlay
│   │   ├── profiles.go                # Named profile support
│   │   └── config_test.go
│   ├── git/
│   │   ├── diff.go                    # Uncommitted, commit-range, branch diff
│   │   ├── parser.go                  # Unified diff parser → normalized hunks
│   │   ├── git.go                     # Low-level git exec helpers
│   │   └── diff_test.go
│   ├── llm/
│   │   ├── client.go                  # OpenAI-compatible HTTP client
│   │   ├── prompt.go                  # Prompt assembly & template rendering
│   │   ├── schema.go                  # Request/response types, JSON schema
│   │   ├── retry.go                   # Retry/backoff/timeout logic
│   │   └── client_test.go
│   ├── review/
│   │   ├── engine.go                  # Orchestration: collect → normalize → trim → send → followup → emit
│   │   ├── context.go                 # ReviewContext, ReviewRequest, Finding types
│   │   ├── trimmer.go                 # Context ranking and truncation
│   │   ├── followup.go               # Follow-up retrieval round handling
│   │   └── engine_test.go
│   ├── retrieval/
│   │   ├── engine.go                  # RetrievalEngine implementation
│   │   ├── file.go                    # get_file, get_file_lines
│   │   ├── symbols.go                # Symbol/function expansion
│   │   ├── callgraph.go              # Caller/callee hierarchy traversal
│   │   ├── goparser/
│   │   │   ├── parser.go             # go/ast + go/types based extraction
│   │   │   ├── callgraph.go          # Go-specific call graph
│   │   │   └── parser_test.go
│   │   ├── fallback/
│   │   │   ├── regex.go              # Regex/heuristic fallback for non-Go
│   │   │   └── regex_test.go
│   │   └── engine_test.go
│   ├── scm/
│   │   ├── types.go                   # Shared SCM types (PRInfo, MRInfo, Comment, etc.)
│   │   ├── github/
│   │   │   ├── client.go             # GitHub REST API client
│   │   │   ├── pr.go                 # PR metadata, commits, files, diff, reviews, comments
│   │   │   ├── adapter.go            # Implements ReviewSource interface
│   │   │   └── client_test.go
│   │   └── gitlab/
│   │       ├── client.go             # GitLab REST API client
│   │       ├── mr.go                 # MR metadata, commits, changes, discussions
│   │       ├── adapter.go            # Implements ReviewSource interface
│   │       └── client_test.go
│   └── output/
│       ├── terminal.go                # Human-readable colored terminal output
│       ├── json.go                    # Structured JSON output
│       ├── sarif.go                   # Stub for future SARIF support
│       └── output_test.go
├── prompts/
│   ├── default_review.tmpl           # Default code review system prompt
│   └── followup_request.tmpl         # Follow-up retrieval instruction template
├── testdata/
│   ├── fixtures/
│   │   ├── github/                   # GitHub API response JSON fixtures
│   │   ├── gitlab/                   # GitLab API response JSON fixtures
│   │   └── diffs/                    # Sample unified diffs
│   └── golden/                       # Golden file outputs for snapshot testing
├── .github/
│   └── workflows/
│       ├── ci.yml                    # Test, lint, build binaries
│       └── docker.yml                # Build + publish multi-arch image
├── Dockerfile
├── .goreleaser.yml                   # Optional: cross-compilation config
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── .llm-review.yaml.example          # Example config file
```

---

## 3. Core Interfaces

These are the three stable interfaces the entire system is built around. Everything else is an implementation detail behind them.

### 3.1 ReviewSource

```go
// internal/review/context.go

type ReviewSource interface {
    // ResolveContext gathers all review material (metadata, diff, comments, files)
    // and returns a normalized ReviewContext.
    ResolveContext(ctx context.Context, req ReviewRequest) (*ReviewContext, error)
}
```

**Implementors:** `internal/git` (local mode), `internal/scm/github` (GitHub adapter), `internal/scm/gitlab` (GitLab adapter).

### 3.2 LLMClient

```go
// internal/llm/client.go

type LLMClient interface {
    // Review sends the assembled prompt + context and returns structured findings.
    Review(ctx context.Context, req *LLMReviewRequest) (*LLMReviewResponse, error)
}
```

**Single implementor:** OpenAI-compatible HTTP client in `internal/llm`.

### 3.3 RetrievalEngine

```go
// internal/retrieval/engine.go

type RetrievalEngine interface {
    GetFile(ctx context.Context, repoRoot, path string) (*FileContent, error)
    GetFileSlice(ctx context.Context, repoRoot, path string, start, end int) (*FileSlice, error)
    GetAdjacentFiles(ctx context.Context, repoRoot, path string, mode AdjacencyMode) ([]FileRef, error)
    GetSymbol(ctx context.Context, repoRoot string, symbol string) (*SymbolInfo, error)
    ExpandFunctions(ctx context.Context, repoRoot string, refs []FunctionRef, depth int) (*FunctionBundle, error)
    FindCallers(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error)
    FindCallees(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error)
}
```

**AdjacencyMode enum:** `SameDir`, `Imports`, `Siblings`.

---

## 4. Core Data Types

### 4.1 ReviewRequest

```go
type ReviewRequest struct {
    Mode            ReviewMode       // local | github | gitlab
    RepoRoot        string           // local filesystem path (local mode)
    Repo            string           // owner/repo (github) or group/project (gitlab)
    Identifier      int              // PR number or MR IID
    BaseRef         string           // base branch or commit
    HeadRef         string           // head branch or commit
    IncludeComments bool
    IncludeCommits  bool
    IncludeFullFiles bool
    MaxContextTokens int
    FollowUpRounds  int             // 0 = no follow-ups, N = up to N rounds
    SeverityThreshold string        // "info" | "warning" | "error" | "critical"
    PromptOverride  string           // path to custom prompt file
}
```

### 4.2 ReviewContext (Provider-Neutral)

```go
type ReviewContext struct {
    Mode            ReviewMode
    Repository      RepositoryInfo
    Title           string
    Description     string
    Commits         []CommitSummary
    ChangedFiles    []ChangedFile
    Diff            string               // full unified diff
    DiffHunks       []DiffHunk           // parsed hunks with file + line metadata
    Comments        []Comment            // normalized from reviews/discussions
    SupplementalContext []SupplementalFile // from retrieval rounds
}

type ChangedFile struct {
    Path       string
    Status     FileStatus  // added | modified | deleted | renamed
    Additions  int
    Deletions  int
    PatchURL   string      // for remote fetching
}

type DiffHunk struct {
    FilePath    string
    OldStart    int
    OldLines    int
    NewStart    int
    NewLines    int
    Content     string
}

type Comment struct {
    Author    string
    Body      string
    Path      string     // empty if top-level
    Line      int        // 0 if not line-specific
    Side      string     // "LEFT" | "RIGHT"
    CreatedAt time.Time
    IsReview  bool       // true if part of a formal review
    ThreadID  string     // for grouping threaded discussions (GitLab)
}

type CommitSummary struct {
    SHA     string
    Message string
    Author  string
    Date    time.Time
}
```

### 4.3 LLM Request/Response

```go
type LLMReviewRequest struct {
    SystemPrompt    string
    UserContent     string           // assembled from ReviewContext
    Schema          *json.RawMessage // optional JSON schema for structured output
    Model           string
    MaxTokens       int
    Temperature     float64
}

type LLMReviewResponse struct {
    Findings         []Finding
    FollowUpRequests []FollowUpRequest  // model-requested additional context
    Summary          string
    RawResponse      string
    TokensUsed       TokenUsage
}

type Finding struct {
    ID          string          `json:"id"`
    Severity    Severity        `json:"severity"`    // info | warning | error | critical
    Category    string          `json:"category"`    // bug, security, performance, style, etc.
    FilePath    string          `json:"file_path"`
    StartLine   int             `json:"start_line"`
    EndLine     int             `json:"end_line"`
    Title       string          `json:"title"`
    Description string          `json:"description"`
    Suggestion  string          `json:"suggestion,omitempty"`
    Confidence  float64         `json:"confidence"`  // 0.0–1.0
}

type FollowUpRequest struct {
    Type       string `json:"type"`       // "file" | "lines" | "function" | "callers" | "callees"
    Path       string `json:"path,omitempty"`
    Symbol     string `json:"symbol,omitempty"`
    StartLine  int    `json:"start_line,omitempty"`
    EndLine    int    `json:"end_line,omitempty"`
    Depth      int    `json:"depth,omitempty"`
    Reason     string `json:"reason"`     // why the model wants this context
}

type TokenUsage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}
```

---

## 5. Package-by-Package Design

### 5.1 `cmd/llm-review` — CLI Entrypoint

**File:** `main.go`

**Responsibility:** Wire Cobra commands, parse flags, load config, instantiate dependencies, invoke review engine.

**Subcommand tree:**

```
llm-review
├── local
│   ├── uncommitted       # git diff (working tree + staged)
│   ├── commits           # --from <ref> --to <ref>
│   └── branch            # --base <branch> --head <branch>
├── github
│   └── pr                # --repo owner/repo --pr 123
├── gitlab
│   └── mr                # --project group/proj --mr 456
└── retrieve
    ├── file              # --path pkg/foo.go
    ├── lines             # --path pkg/foo.go --start 120 --end 220
    ├── callers           # --symbol parseDatetime [--depth 3]
    ├── callees           # --symbol parseDatetime [--depth 3]
    └── function-stack    # --symbol parseDatetime --direction callers --depth 4
```

**Global/persistent flags (registered on root):**

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `--model` | `LLM_REVIEW_MODEL` | `gpt-4o` | Model identifier |
| `--base-url` | `LLM_REVIEW_BASE_URL` | `https://api.openai.com/v1` | API base URL |
| `--api-key` | `LLM_REVIEW_API_KEY` | — | API key |
| `--profile` | — | `default` | Config profile name |
| `--max-context-tokens` | — | `120000` | Hard token budget |
| `--include-full-files` | — | `false` | Send full changed files |
| `--include-comments` | — | `true` | Include existing comments |
| `--include-commits` | — | `true` | Include commit messages |
| `--json` | — | `false` | JSON-only output |
| `--followups` | — | `1` | Max follow-up retrieval rounds |
| `--offline` | — | `false` | Skip remote SCM APIs |
| `--severity-threshold` | — | `info` | Minimum severity to display |
| `--prompt-file` | — | — | Path to custom prompt template |
| `--config` | — | `.llm-review.yaml` | Config file path |

**Initialization sequence in each command's `RunE`:**

1. Load config (file → env → flags, with flags winning).
2. Instantiate the appropriate `ReviewSource` (local/github/gitlab).
3. Instantiate `LLMClient` with resolved base_url, api_key, model.
4. Instantiate `RetrievalEngine` pointing at the repo root.
5. Instantiate `ReviewEngine` with all three.
6. Call `engine.Run(ctx, request)`.
7. Format output (terminal or JSON) via `internal/output`.

### 5.2 `internal/config` — Configuration

**Files:** `config.go`, `profiles.go`

**Config resolution order (later wins):**

1. Built-in defaults.
2. YAML config file (`~/.llm-review.yaml` or `--config` path).
3. Environment variables (`LLM_REVIEW_*` prefix).
4. CLI flags.

**Config struct:**

```go
type Config struct {
    ActiveProfile    string              `yaml:"active_profile"`
    Profiles         map[string]Profile  `yaml:"profiles"`
}

type Profile struct {
    Model            string `yaml:"model"`
    BaseURL          string `yaml:"base_url"`
    APIKey           string `yaml:"api_key"`
    MaxContextTokens int    `yaml:"max_context_tokens"`
    DefaultFollowUps int    `yaml:"default_followups"`
    GitHubToken      string `yaml:"github_token"`
    GitLabToken      string `yaml:"gitlab_token"`
    GitLabBaseURL    string `yaml:"gitlab_base_url"` // for self-hosted
    PromptFile       string `yaml:"prompt_file"`
}
```

**Example `.llm-review.yaml`:**

```yaml
active_profile: work
profiles:
  default:
    model: gpt-4o
    base_url: https://api.openai.com/v1
    max_context_tokens: 120000
  work:
    model: claude-sonnet-4-20250514
    base_url: https://api.anthropic.com/v1
    max_context_tokens: 180000
    github_token: ${GITHUB_TOKEN}
  local-llama:
    model: llama3
    base_url: http://localhost:11434/v1
    max_context_tokens: 8000
```

**Key decisions:**

- Env vars in YAML values are expanded at load time (simple `${VAR}` substitution).
- No global variables; the resolved `Config` is passed explicitly to all constructors.
- `APIKey` is never logged or included in error messages.

### 5.3 `internal/git` — Local Git Operations

**Files:** `diff.go`, `parser.go`, `git.go`

**Responsibilities:**

- Execute git commands and capture output.
- Produce unified diffs for three local sub-modes.
- Parse unified diffs into `[]DiffHunk`.
- Implement `ReviewSource` for local mode.

**Git command mapping:**

| Sub-mode | Git command |
|---|---|
| `uncommitted` | `git diff HEAD` (combines staged + unstaged) |
| `commits` | `git diff <from>..<to>` |
| `branch` | `git diff <base>...<head>` (three-dot merge-base diff) |

**`LocalSource` struct (implements `ReviewSource`):**

```go
type LocalSource struct {
    repoRoot string
    git      GitRunner  // interface for testing
}

func (s *LocalSource) ResolveContext(ctx context.Context, req ReviewRequest) (*ReviewContext, error) {
    // 1. Determine diff command based on sub-mode
    // 2. Run git diff, capture raw output
    // 3. Parse diff into hunks
    // 4. Extract changed file list from diff headers
    // 5. Optionally run git log for commit summaries
    // 6. Assemble and return ReviewContext
}
```

**`GitRunner` interface (for testability):**

```go
type GitRunner interface {
    Run(ctx context.Context, args ...string) (string, error)
}
```

Production implementation shells out to `git`. Test implementation returns canned output.

**Diff parser details:**

- Parse `---`/`+++` headers to extract file paths.
- Parse `@@` hunk headers to extract line ranges.
- Track `FileStatus` from diff headers (`new file`, `deleted file`, `rename from/to`).
- Preserve original line numbers for every line in the hunk.
- Output `[]DiffHunk` with stable line numbers so findings can reference exact positions.

### 5.4 `internal/llm` — LLM Client

**Files:** `client.go`, `prompt.go`, `schema.go`, `retry.go`

#### 5.4.1 Client (`client.go`)

HTTP client for any OpenAI-compatible `/v1/chat/completions` endpoint.

```go
type OpenAIClient struct {
    baseURL    string
    apiKey     string
    model      string
    httpClient *http.Client
    retrier    *Retrier
}
```

**Request construction:**

- System message: rendered review prompt.
- User message: serialized `ReviewContext` content.
- If the model supports structured outputs (`response_format: { type: "json_schema", ... }`), attach the findings schema.
- Fallback: request `response_format: { type: "json_object" }` and validate after the fact.
- Set `max_tokens`, `temperature` (default 0.2 for review tasks).

**Response parsing:**

- Extract `choices[0].message.content`.
- Attempt JSON parse into `LLMReviewResponse`.
- If structured output fails, attempt to extract a JSON block from freeform text (regex for ```json...``` fences).
- If all parsing fails, return a single "parse_error" finding with the raw text.
- Capture token usage from the response.

#### 5.4.2 Prompt Assembly (`prompt.go`)

**Template rendering with `text/template`.**

**Default prompt structure (`prompts/default_review.tmpl`):**

```
You are a senior code reviewer. Analyze the following code changes and produce structured findings.

## Review Mode
{{.Mode}} review of {{.Repository.FullName}}

## Title
{{.Title}}

## Description
{{.Description}}

{{if .Commits}}
## Commits
{{range .Commits}}
- {{.SHA | truncate 8}}: {{.Message | firstLine}}
{{end}}
{{end}}

{{if .Comments}}
## Existing Review Comments
{{range .Comments}}
[{{.Author}}] {{if .Path}}({{.Path}}:{{.Line}}){{end}}: {{.Body}}
{{end}}
{{end}}

## Changed Files
{{range .ChangedFiles}}
- {{.Status}} {{.Path}} (+{{.Additions}}/-{{.Deletions}})
{{end}}

## Diff
```diff
{{.Diff}}
```

{{if .SupplementalContext}}
## Additional Context (from retrieval)
{{range .SupplementalContext}}
### {{.Path}} (lines {{.StartLine}}-{{.EndLine}})
```{{.Language}}
{{.Content}}
```
{{end}}
{{end}}

## Instructions
- Focus on bugs, security issues, performance problems, and significant design concerns.
- For each finding, provide: severity, category, file_path, start_line, end_line, title, description, suggestion, confidence.
- If you need more context to complete your review, include follow_up_requests specifying what files, line ranges, or function call stacks you need.
- Respond ONLY with valid JSON matching the provided schema.
```

**Prompt override:** If `--prompt-file` is set, load and render that template instead. The same template variables are available.

**Truncation (handled by `internal/review/trimmer.go` before prompt assembly):**

Priority order (highest to lowest):
1. Title, description (always kept in full).
2. Changed file list (always kept in full).
3. Diff headers and hunk metadata (always kept).
4. Diff content (truncated from largest files first).
5. Commit summaries (truncated to first line per commit).
6. Comments (summarized if too large; oldest dropped first).
7. Supplemental context (dropped if budget exhausted).

Token estimation: use a simple `len(text) / 4` heuristic (good enough for English + code). Make the estimator an interface for future tiktoken integration.

#### 5.4.3 Retry Logic (`retry.go`)

```go
type Retrier struct {
    MaxRetries     int           // default 3
    InitialBackoff time.Duration // default 1s
    MaxBackoff     time.Duration // default 30s
    RetryableHTTP  []int         // 429, 500, 502, 503, 504
}
```

- Exponential backoff with jitter.
- Honor `Retry-After` header if present (HTTP 429).
- Context-aware: stop retrying if `ctx` is cancelled.
- Log each retry attempt at debug level.

#### 5.4.4 Schema (`schema.go`)

Define the JSON schema for `LLMReviewResponse` as a Go struct that can be serialized to a JSON Schema document for structured output mode.

```go
var FindingsSchema = json.RawMessage(`{
    "type": "object",
    "properties": {
        "findings": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "id": {"type": "string"},
                    "severity": {"type": "string", "enum": ["info","warning","error","critical"]},
                    "category": {"type": "string"},
                    "file_path": {"type": "string"},
                    "start_line": {"type": "integer"},
                    "end_line": {"type": "integer"},
                    "title": {"type": "string"},
                    "description": {"type": "string"},
                    "suggestion": {"type": "string"},
                    "confidence": {"type": "number", "minimum": 0, "maximum": 1}
                },
                "required": ["severity","category","file_path","title","description","confidence"]
            }
        },
        "follow_up_requests": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "type": {"type": "string", "enum": ["file","lines","function","callers","callees"]},
                    "path": {"type": "string"},
                    "symbol": {"type": "string"},
                    "start_line": {"type": "integer"},
                    "end_line": {"type": "integer"},
                    "depth": {"type": "integer"},
                    "reason": {"type": "string"}
                },
                "required": ["type","reason"]
            }
        },
        "summary": {"type": "string"}
    },
    "required": ["findings","summary"]
}`)
```

### 5.5 `internal/review` — Review Engine

**Files:** `engine.go`, `context.go`, `trimmer.go`, `followup.go`

This is the orchestration heart of the application. It owns the pipeline.

#### Pipeline Steps

```go
type Engine struct {
    source    ReviewSource
    llm       LLMClient
    retrieval RetrievalEngine
    config    *config.Config
    output    output.Formatter
}

func (e *Engine) Run(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
    // Step 1: Collect review context from source
    reviewCtx, err := e.source.ResolveContext(ctx, req)

    // Step 2: Trim and rank context to fit token budget
    trimmed := e.trimContext(reviewCtx, req.MaxContextTokens)

    // Step 3: Assemble prompt
    prompt := e.assemblePrompt(trimmed, req)

    // Step 4: Send to LLM (initial pass)
    response, err := e.llm.Review(ctx, prompt)

    // Step 5: Follow-up retrieval rounds (0 to N)
    for round := 0; round < req.FollowUpRounds; round++ {
        if len(response.FollowUpRequests) == 0 {
            break
        }
        supplemental := e.executeRetrievals(ctx, req.RepoRoot, response.FollowUpRequests)
        reviewCtx.SupplementalContext = append(reviewCtx.SupplementalContext, supplemental...)
        trimmed = e.trimContext(reviewCtx, req.MaxContextTokens)
        prompt = e.assemblePrompt(trimmed, req)
        response, err = e.llm.Review(ctx, prompt)
    }

    // Step 6: Filter findings by severity threshold
    filtered := e.filterBySeverity(response.Findings, req.SeverityThreshold)

    // Step 7: Emit output
    result := &ReviewResult{
        Findings:   filtered,
        Summary:    response.Summary,
        TokensUsed: response.TokensUsed,
    }
    return result, nil
}
```

#### Context Trimmer (`trimmer.go`)

```go
type Trimmer struct {
    maxTokens     int
    tokenEstimator TokenEstimator
}

type TokenEstimator interface {
    Estimate(text string) int
}

// SimpleEstimator: len(text) / 4
type SimpleEstimator struct{}
```

**Trimming strategy:**

1. Compute token budget for each section based on priority weights.
2. Always preserve: title, description, changed file list, diff hunk headers.
3. If diff exceeds its budget: drop hunks from the largest files first, keeping the header.
4. If commits exceed budget: keep only SHA + first line of message.
5. If comments exceed budget: keep most recent N, summarize older ones as "(N older comments omitted)".
6. Generated files (e.g., `go.sum`, `package-lock.json`): drop entirely, note in context.

#### Follow-Up Handler (`followup.go`)

```go
func (e *Engine) executeRetrievals(
    ctx context.Context,
    repoRoot string,
    requests []FollowUpRequest,
) []SupplementalFile {
    var results []SupplementalFile
    for _, req := range requests {
        switch req.Type {
        case "file":
            content, err := e.retrieval.GetFile(ctx, repoRoot, req.Path)
            // ...
        case "lines":
            slice, err := e.retrieval.GetFileSlice(ctx, repoRoot, req.Path, req.StartLine, req.EndLine)
            // ...
        case "function":
            info, err := e.retrieval.GetSymbol(ctx, repoRoot, req.Symbol)
            // ...
        case "callers":
            hierarchy, err := e.retrieval.FindCallers(ctx, repoRoot, SymbolRef{Name: req.Symbol}, req.Depth)
            // ...
        case "callees":
            hierarchy, err := e.retrieval.FindCallees(ctx, repoRoot, SymbolRef{Name: req.Symbol}, req.Depth)
            // ...
        }
    }
    return results
}
```

### 5.6 `internal/retrieval` — Retrieval Engine

**Files:** `engine.go`, `file.go`, `symbols.go`, `callgraph.go`, `goparser/`, `fallback/`

#### 5.6.1 File Retrieval (`file.go`)

```go
func (e *Engine) GetFile(ctx context.Context, repoRoot, path string) (*FileContent, error) {
    fullPath := filepath.Join(repoRoot, path)
    data, err := os.ReadFile(fullPath)
    // Return with stable 1-indexed line numbers
    return &FileContent{
        Path:    path,
        Lines:   splitLines(data),
        LineMap: buildLineMap(data),  // line number → offset for precise referencing
    }, nil
}

func (e *Engine) GetFileSlice(ctx context.Context, repoRoot, path string, start, end int) (*FileSlice, error) {
    full, err := e.GetFile(ctx, repoRoot, path)
    // Return lines[start-1:end] with original line numbers preserved
    return &FileSlice{
        Path:      path,
        StartLine: start,
        EndLine:   end,
        Lines:     full.Lines[start-1 : end],
    }, nil
}
```

**Important:** All line numbers are 1-indexed and stable. When the LLM references "line 42", it means the same line 42 in the original file. This is critical because GitHub review comments attach to unified diff hunks with specific line positions.

#### 5.6.2 Adjacent File Discovery

```go
type AdjacencyMode int
const (
    SameDir AdjacencyMode = iota  // other files in the same directory
    Imports                        // files imported/required by this file
    Siblings                       // files with similar names (e.g., foo.go → foo_test.go)
)

func (e *Engine) GetAdjacentFiles(ctx context.Context, repoRoot, path string, mode AdjacencyMode) ([]FileRef, error) {
    switch mode {
    case SameDir:
        // List directory, return all non-test files
    case Imports:
        // Parse the file, extract imports, resolve to local paths
    case Siblings:
        // Match base name patterns (e.g., _test.go, _mock.go)
    }
}
```

#### 5.6.3 Go-Specific Parser (`goparser/`)

This is the primary language backend. Uses the Go standard library's AST tooling.

**Capabilities:**

| Feature | Implementation |
|---|---|
| Parse functions | `go/parser.ParseFile` → walk AST for `*ast.FuncDecl` |
| Extract symbols | Map function names to file:line positions |
| Build call graph | `go/packages.Load` with `NeedTypesInfo` → scan `*ast.CallExpr` nodes |
| Find callers | Reverse index of call graph |
| Find callees | Forward traversal of call graph |
| Resolve types | `go/types.Info.Uses` to resolve cross-package references |

**Call graph data structure:**

```go
type CallGraph struct {
    Functions map[string]*FunctionNode  // keyed by "pkg.FuncName"
}

type FunctionNode struct {
    Name     string
    FilePath string
    StartLine int
    EndLine   int
    Source    string        // full source text
    Callers  []*FunctionNode
    Callees  []*FunctionNode
}
```

**`function-stack` example output for `parseDatetime`:**

```
parseDatetime           (pkg/parser/datetime.go:45-82)
├── parseJSON           (pkg/parser/json.go:112-145)    [caller]
│   ├── parseConfiguration (pkg/config/loader.go:30-67) [caller]
│   │   └── init        (cmd/app/main.go:15-28)         [caller]
```

Each node includes full function source, file path, and line range.

#### 5.6.4 Fallback Parser (`fallback/`)

For non-Go repositories, provide a regex/heuristic-based parser.

**Strategy:** Use language-specific regex patterns to find function definitions.

```go
type FallbackParser struct {
    patterns map[string]*LanguagePattern
}

type LanguagePattern struct {
    FuncDef    *regexp.Regexp  // matches function/method declarations
    FuncCall   *regexp.Regexp  // matches function calls (approximate)
    Extensions []string
}
```

**Built-in patterns for:**

- Go: `func\s+(\w+)\s*\(` and `func\s+\([^)]+\)\s+(\w+)\s*\(`
- Python: `def\s+(\w+)\s*\(`
- JavaScript/TypeScript: `function\s+(\w+)`, `const\s+(\w+)\s*=\s*(?:async\s*)?\(`
- Rust: `fn\s+(\w+)\s*[<(]`
- Java: `(?:public|private|protected)\s+\w+\s+(\w+)\s*\(`

**Limitations documented clearly:** Call graph is approximate; no type resolution; may produce false positives.

**Design for extensibility:** A `LanguageBackend` interface:

```go
type LanguageBackend interface {
    ParseFunctions(ctx context.Context, repoRoot string, paths []string) ([]FunctionInfo, error)
    BuildCallGraph(ctx context.Context, repoRoot string) (*CallGraph, error)
    Language() string
}
```

The Go parser implements this fully. The fallback implements it partially. Future tree-sitter backends will implement it fully for each language.

### 5.7 `internal/scm/github` — GitHub Provider

**Files:** `client.go`, `pr.go`, `adapter.go`

#### API Endpoints Used

| Data | Endpoint | Notes |
|---|---|---|
| PR metadata | `GET /repos/{owner}/{repo}/pulls/{pull_number}` | Title, description, state, base/head |
| Commits | `GET /repos/{owner}/{repo}/pulls/{pull_number}/commits` | Paginated |
| Changed files | `GET /repos/{owner}/{repo}/pulls/{pull_number}/files` | Includes patch/diff per file |
| Reviews | `GET /repos/{owner}/{repo}/pulls/{pull_number}/reviews` | Approval state + body |
| Review comments | `GET /repos/{owner}/{repo}/pulls/{pull_number}/comments` | Line-specific comments |
| Issue comments | `GET /repos/{owner}/{repo}/issues/{issue_number}/comments` | Top-level PR discussion |
| File content | `GET /repos/{owner}/{repo}/contents/{path}?ref={sha}` | For full-file retrieval |

#### Client Design

```go
type GitHubClient struct {
    baseURL    string        // default: https://api.github.com
    token      string
    httpClient *http.Client
}
```

- All methods accept `context.Context` for cancellation.
- Pagination: follow `Link` header for all list endpoints.
- Rate limiting: check `X-RateLimit-Remaining`, log warnings at low thresholds.
- Auth: `Authorization: Bearer <token>` header.

#### Adapter (implements `ReviewSource`)

```go
type GitHubAdapter struct {
    client *GitHubClient
}

func (a *GitHubAdapter) ResolveContext(ctx context.Context, req ReviewRequest) (*ReviewContext, error) {
    // 1. Fetch PR metadata → title, description, base/head refs
    // 2. Fetch commits → []CommitSummary
    // 3. Fetch changed files → []ChangedFile + aggregated diff
    // 4. If include_comments:
    //    a. Fetch reviews → normalize to []Comment
    //    b. Fetch review comments → normalize to []Comment (with path+line)
    //    c. Optionally fetch issue comments → normalize to []Comment
    // 5. Assemble ReviewContext
}
```

**Comment normalization:** GitHub review comments have `path`, `position` (deprecated), `line`, `side`, `in_reply_to_id`. Map these to the generic `Comment` type. Group replies by `in_reply_to_id` for thread context.

### 5.8 `internal/scm/gitlab` — GitLab Provider

**Files:** `client.go`, `mr.go`, `adapter.go`

#### API Endpoints Used

| Data | Endpoint | Notes |
|---|---|---|
| MR metadata | `GET /projects/:id/merge_requests/:mr_iid` | Title, description, state |
| Commits | `GET /projects/:id/merge_requests/:mr_iid/commits` | Paginated |
| Changes/diff | `GET /projects/:id/merge_requests/:mr_iid/changes` | Includes diffs per file |
| Discussions | `GET /projects/:id/merge_requests/:mr_iid/discussions` | Threaded comments |
| File content | `GET /projects/:id/repository/files/:file_path/raw?ref=:sha` | URL-encoded path |

#### Client Design

```go
type GitLabClient struct {
    baseURL    string        // default: https://gitlab.com/api/v4
    token      string
    httpClient *http.Client
}
```

- Auth: `PRIVATE-TOKEN: <token>` header.
- Project ID: URL-encode `group/project` as `group%2Fproject`.
- Pagination: `X-Page`, `X-Next-Page`, `X-Total-Pages` headers.

#### Adapter (implements `ReviewSource`)

Same pattern as GitHub. Key difference: GitLab discussions are inherently threaded. Each discussion has a list of notes. Map the first note as the comment, subsequent notes as replies. Preserve `ThreadID` for the review engine to understand conversation context.

### 5.9 `internal/output` — Output Formatting

**Files:** `terminal.go`, `json.go`, `sarif.go`

#### Formatter Interface

```go
type Formatter interface {
    FormatFindings(findings []Finding, summary string, tokens TokenUsage) error
}
```

#### Terminal Output (`terminal.go`)

Uses ANSI colors. Example output:

```
╔══════════════════════════════════════════════════════════════╗
║  NickPit Review — owner/repo PR #123                        ║
╚══════════════════════════════════════════════════════════════╝

  3 findings (1 error, 1 warning, 1 info)

  ERROR   pkg/auth/token.go:45-52  [security]
  ──────  Hardcoded secret in token validation
          The JWT signing key is hardcoded. Move to environment
          variable or secrets manager.
          Confidence: 0.95

  WARNING pkg/api/handler.go:112   [performance]
  ──────  Unbounded SQL query
          This query has no LIMIT clause and could return
          millions of rows. Add pagination.
          Confidence: 0.82

  INFO    internal/util/strings.go:28  [style]
  ──────  Unused parameter
          The 'prefix' parameter is never used in the function body.
          Confidence: 0.70

  Summary: The PR introduces a new auth module with a critical
  security concern around key management. The API handler needs
  pagination for production readiness.

  Tokens: 4,521 prompt / 892 completion / 5,413 total
```

Color mapping: `critical` = red bold, `error` = red, `warning` = yellow, `info` = cyan.

Respect `NO_COLOR` environment variable and `--json` flag.

#### JSON Output (`json.go`)

When `--json` is set, output the full `ReviewResult` as pretty-printed JSON to stdout.

```json
{
  "findings": [...],
  "summary": "...",
  "tokens_used": { "prompt": 4521, "completion": 892, "total": 5413 },
  "metadata": {
    "model": "gpt-4o",
    "mode": "github",
    "repo": "owner/repo",
    "pr": 123,
    "followup_rounds": 1
  }
}
```

#### SARIF Stub (`sarif.go`)

Placeholder for future SARIF 2.1.0 output format. Implement the interface but return an error: "SARIF output not yet implemented."

---

## 6. Default Review Prompt Design

The default prompt in `prompts/default_review.tmpl` should instruct the model to:

1. Act as a senior engineer performing a thorough code review.
2. Focus on: bugs, security vulnerabilities, performance issues, race conditions, error handling gaps, API design problems, and significant style issues.
3. Ignore: minor formatting, trivial naming preferences, and auto-generated code.
4. For each finding, provide all fields in the `Finding` schema.
5. Assign a confidence score (0.0–1.0) reflecting how certain the finding is.
6. If more context is needed, include `follow_up_requests` specifying exactly what to retrieve.
7. End with a brief `summary` of the overall PR quality and key concerns.

The prompt template receives the full `ReviewContext` struct and renders it using Go's `text/template`.

**Follow-up prompt (`prompts/followup_request.tmpl`):** After retrieval, prepend the supplemental context and re-state: "You previously requested additional context. Here it is. Please update your review findings accordingly. Do NOT repeat findings you already made unless the new context changes them."

---

## 7. Two-Pass Review Flow — Detailed Sequence

```
┌─────────────┐
│  CLI Input   │
└──────┬──────┘
       │
       ▼
┌──────────────┐     ┌───────────────┐
│ ReviewSource │────▶│ ReviewContext  │
│ (local/gh/gl)│     │ (normalized)  │
└──────────────┘     └───────┬───────┘
                             │
                             ▼
                     ┌───────────────┐
                     │   Trimmer     │
                     │ (fit budget)  │
                     └───────┬───────┘
                             │
                             ▼
                     ┌───────────────┐
                     │ Prompt Asm.   │
                     │ (template)    │
                     └───────┬───────┘
                             │
                     ┌───────▼───────┐
              ┌─────▶│   LLM Call    │──────┐
              │      │  (Pass 1)     │      │
              │      └───────────────┘      │
              │                             │
              │         Has follow-up       │
              │         requests?           │
              │              │              │
              │         yes  │  no          │
              │              ▼              ▼
              │      ┌───────────────┐  ┌──────────┐
              │      │  Retrieval    │  │  Output   │
              │      │  Engine       │  │          │
              │      └───────┬───────┘  └──────────┘
              │              │
              │     Append to context
              │              │
              └──────────────┘
                  (up to N rounds)
```

---

## 8. Testing Strategy

### 8.1 Unit Tests

| Package | What to test | Approach |
|---|---|---|
| `internal/config` | Config loading, env override, profile merging | Table-driven tests with temp YAML files |
| `internal/git` | Diff parsing, hunk extraction, line numbering | Golden files of real diffs in `testdata/diffs/` |
| `internal/llm` | Request construction, response parsing, retry logic | HTTP test server returning canned responses |
| `internal/review` | Trimmer logic, follow-up handling, severity filtering | Mock ReviewSource + mock LLMClient |
| `internal/retrieval` | File slice, symbol extraction, call graph | Real Go source files in `testdata/` |
| `internal/scm/github` | API response parsing, pagination, comment normalization | JSON fixtures in `testdata/fixtures/github/` |
| `internal/scm/gitlab` | API response parsing, discussion threading | JSON fixtures in `testdata/fixtures/gitlab/` |
| `internal/output` | Terminal formatting, JSON serialization | Golden file comparison |

### 8.2 Golden Tests

For the output package and the diff parser, use golden file testing:

```go
func TestTerminalOutput(t *testing.T) {
    findings := loadFindings("testdata/fixtures/sample_findings.json")
    var buf bytes.Buffer
    formatter := terminal.NewFormatter(&buf, false) // no color for golden comparison
    formatter.FormatFindings(findings, "Test summary", TokenUsage{})

    golden := filepath.Join("testdata", "golden", t.Name()+".txt")
    if *update {
        os.WriteFile(golden, buf.Bytes(), 0644)
    }
    expected, _ := os.ReadFile(golden)
    assert.Equal(t, string(expected), buf.String())
}
```

### 8.3 Integration Tests (build-tagged)

Tag integration tests with `//go:build integration` so they don't run in CI without real credentials.

- Test GitHub adapter against a known public PR.
- Test GitLab adapter against a known public MR.
- Test local mode against a real git repository created in a temp dir.

### 8.4 Fixtures

**`testdata/fixtures/github/`:**
- `pr_metadata.json` — sample `GET /pulls/123` response
- `pr_commits.json` — sample commits list
- `pr_files.json` — sample changed files
- `pr_reviews.json` — sample reviews
- `pr_review_comments.json` — sample line-level comments
- `pr_issue_comments.json` — sample top-level comments

**`testdata/fixtures/gitlab/`:**
- `mr_metadata.json` — sample `GET /merge_requests/456` response
- `mr_commits.json` — sample commits
- `mr_changes.json` — sample changes with diffs
- `mr_discussions.json` — sample threaded discussions

**`testdata/diffs/`:**
- `simple_add.diff` — single file addition
- `multi_file.diff` — changes across several files
- `rename.diff` — file rename with changes
- `binary.diff` — diff with binary file markers
- `large.diff` — stress test for the trimmer

### 8.5 Test Helpers

Create a `testutil` package with:

- `LoadFixture(t, path) []byte` — read fixture file
- `MustParseJSON[T](t, data []byte) T` — parse JSON or fail
- `NewTestGitRepo(t) (path string, cleanup func())` — create a temp git repo with some commits
- `AssertGolden(t, got string, goldenPath string)` — golden file comparison with `-update` flag

---

## 9. Error Handling Patterns

**Consistent error wrapping with `fmt.Errorf`:**

```go
return fmt.Errorf("github: fetching PR %d: %w", prNumber, err)
```

**Error categories:**

- `ErrAuth` — authentication failures (missing/invalid token)
- `ErrNotFound` — PR/MR/file not found (404)
- `ErrRateLimit` — rate limit exceeded (429, with retry info)
- `ErrAPI` — unexpected API error (5xx)
- `ErrParse` — failed to parse LLM response
- `ErrGit` — git command failure
- `ErrConfig` — invalid configuration

**User-facing messages:** The CLI catches errors and prints them with context. Never expose raw HTTP response bodies or stack traces to the user. Example:

```
Error: GitHub API returned 404 for PR #999 in owner/repo.
       Check that the PR exists and your GITHUB_TOKEN has read access.
```

---

## 10. CI/CD

### 10.1 GitHub Actions CI (`.github/workflows/ci.yml`)

```yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go vet ./...
      - run: go test -race -coverprofile=coverage.out ./...
      - uses: actions/upload-artifact@v4
        with:
          name: coverage
          path: coverage.out

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - uses: golangci/golangci-lint-action@v4
        with:
          version: latest

  build:
    runs-on: ubuntu-latest
    needs: [test, lint]
    strategy:
      matrix:
        goos: [linux, darwin, windows]
        goarch: [amd64, arm64]
        exclude:
          - goos: windows
            goarch: arm64
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: |
          GOOS=${{ matrix.goos }} GOARCH=${{ matrix.goarch }} \
          go build -ldflags="-s -w" -o llm-review-${{ matrix.goos }}-${{ matrix.goarch }} \
          ./cmd/llm-review
      - uses: actions/upload-artifact@v4
        with:
          name: llm-review-${{ matrix.goos }}-${{ matrix.goarch }}
          path: llm-review-*
```

### 10.2 Docker Workflow (`.github/workflows/docker.yml`)

```yaml
name: Docker
on:
  push:
    tags: ['v*']

jobs:
  docker:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/build-push-action@v5
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ghcr.io/${{ github.repository }}:${{ github.ref_name }}
            ghcr.io/${{ github.repository }}:latest
```

### 10.3 Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /llm-review ./cmd/llm-review

FROM alpine:3.19
RUN apk add --no-cache git ca-certificates
COPY --from=builder /llm-review /usr/local/bin/llm-review
COPY prompts/ /etc/llm-review/prompts/
ENTRYPOINT ["llm-review"]
```

---

## 11. Implementation Phases

### Phase 1: Scaffolding & Core Types (Days 1–2)

**Goal:** Compilable skeleton with all interfaces and types defined.

| Task | Files | Deliverable |
|---|---|---|
| Initialize Go module | `go.mod` | `module github.com/<org>/nickpit` |
| Add Cobra dependency | `go.mod` | `cobra` imported |
| Define all core types | `internal/review/context.go` | All structs from §4 |
| Define all interfaces | `internal/review/context.go`, `internal/llm/client.go`, `internal/retrieval/engine.go` | `ReviewSource`, `LLMClient`, `RetrievalEngine` |
| Stub all packages | Every `*.go` file | Compiles but returns "not implemented" |
| Create CLI skeleton | `cmd/llm-review/main.go` | All subcommands wired, print "not implemented" |
| Config loader | `internal/config/config.go` | YAML + env + flag merging works |
| Makefile | `Makefile` | `make build`, `make test`, `make lint` |

**Exit criteria:** `go build ./...` succeeds. `llm-review local uncommitted` prints "not implemented". Config loads from file.

### Phase 2: Local Mode (Days 3–5)

**Goal:** Full local review pipeline working end-to-end.

| Task | Files | Deliverable |
|---|---|---|
| Git runner | `internal/git/git.go` | Execute git commands, capture output |
| Diff extraction | `internal/git/diff.go` | `uncommitted`, `commits`, `branch` sub-modes |
| Diff parser | `internal/git/parser.go` | Unified diff → `[]DiffHunk` with stable line numbers |
| LocalSource adapter | `internal/git/diff.go` | Implements `ReviewSource` |
| LLM client | `internal/llm/client.go` | Working HTTP client with structured output + JSON fallback |
| Retry logic | `internal/llm/retry.go` | Exponential backoff with jitter |
| Prompt assembly | `internal/llm/prompt.go` | Template rendering |
| Default prompt | `prompts/default_review.tmpl` | Complete review prompt |
| Review engine | `internal/review/engine.go` | Full pipeline (no follow-ups yet) |
| Context trimmer | `internal/review/trimmer.go` | Token budget enforcement |
| Terminal output | `internal/output/terminal.go` | Colored findings display |
| JSON output | `internal/output/json.go` | `--json` flag works |
| Unit tests | `*_test.go` | Diff parser, config, trimmer, output formatting |

**Exit criteria:** `llm-review local uncommitted` produces a real review from a real LLM. `llm-review local commits --from HEAD~3 --to HEAD` works. `--json` output is valid JSON. Tests pass.

### Phase 3: GitHub & GitLab Adapters (Days 6–9)

**Goal:** Remote SCM review modes work end-to-end.

| Task | Files | Deliverable |
|---|---|---|
| GitHub HTTP client | `internal/scm/github/client.go` | Auth, pagination, rate limit awareness |
| GitHub PR fetcher | `internal/scm/github/pr.go` | All 6 endpoint calls |
| GitHub adapter | `internal/scm/github/adapter.go` | Implements `ReviewSource` |
| GitHub fixtures | `testdata/fixtures/github/*.json` | All API response fixtures |
| GitHub tests | `internal/scm/github/client_test.go` | Fixture-based tests with httptest |
| GitLab HTTP client | `internal/scm/gitlab/client.go` | Auth, pagination |
| GitLab MR fetcher | `internal/scm/gitlab/mr.go` | All 4 endpoint calls |
| GitLab adapter | `internal/scm/gitlab/adapter.go` | Implements `ReviewSource` |
| GitLab fixtures | `testdata/fixtures/gitlab/*.json` | All API response fixtures |
| GitLab tests | `internal/scm/gitlab/client_test.go` | Fixture-based tests with httptest |

**Exit criteria:** `llm-review github pr --repo owner/repo --pr 123` produces a review. Same for GitLab. Comment threading works. Pagination tested.

### Phase 4: Retrieval Engine (Days 10–13)

**Goal:** All retrieval primitives work. Follow-up rounds work.

| Task | Files | Deliverable |
|---|---|---|
| File retrieval | `internal/retrieval/file.go` | `GetFile`, `GetFileSlice` with stable line numbers |
| Adjacent files | `internal/retrieval/file.go` | `GetAdjacentFiles` (SameDir, Imports, Siblings) |
| Go parser | `internal/retrieval/goparser/parser.go` | Function extraction from Go AST |
| Go call graph | `internal/retrieval/goparser/callgraph.go` | Caller/callee analysis |
| Fallback parser | `internal/retrieval/fallback/regex.go` | Regex-based function finder |
| Symbol expansion | `internal/retrieval/symbols.go` | `GetSymbol`, `ExpandFunctions` |
| Call hierarchy | `internal/retrieval/callgraph.go` | `FindCallers`, `FindCallees` |
| Retrieval CLI commands | `cmd/llm-review/main.go` | `retrieve file`, `lines`, `callers`, `function-stack` |
| Follow-up handler | `internal/review/followup.go` | Execute LLM follow-up requests |
| Wire follow-ups into engine | `internal/review/engine.go` | Two-pass review works |
| Follow-up prompt | `prompts/followup_request.tmpl` | Supplemental context prompt |
| Retrieval tests | `internal/retrieval/*_test.go` | Unit tests for all primitives |
| Go parser tests | `internal/retrieval/goparser/parser_test.go` | Real Go files as test fixtures |

**Exit criteria:** `llm-review retrieve function-stack --symbol parseDatetime --direction callers --depth 4` returns the expected hierarchy. A review with `--followups 1` performs a retrieval round when the model requests it.

### Phase 5: Testing, Polish & CI (Days 14–16)

**Goal:** Production-ready quality.

| Task | Files | Deliverable |
|---|---|---|
| Golden tests | `testdata/golden/` | Snapshot tests for all output formats |
| Integration tests | `*_integration_test.go` | Build-tagged tests against real repos |
| Error message audit | All packages | Every error path produces a clear, actionable message |
| `--offline` flag | `cmd/`, `internal/review/` | Skip SCM API calls when set |
| `--severity-threshold` | `internal/review/` | Filter findings in output |
| `NO_COLOR` support | `internal/output/terminal.go` | Respect env var |
| CI workflow | `.github/workflows/ci.yml` | Test + lint + build matrix |
| Docker workflow | `.github/workflows/docker.yml` | Multi-arch image build |
| Dockerfile | `Dockerfile` | Multi-stage Alpine build |
| README | `README.md` | Full setup, config, and usage docs |
| Example config | `.llm-review.yaml.example` | Annotated example |
| Makefile polish | `Makefile` | All standard targets |

**Exit criteria:** CI passes. Docker image builds. README is complete. All tests pass with `-race`. `go vet` and `golangci-lint` produce zero warnings.

---

## 12. Key Design Decisions & Rationale

| Decision | Rationale |
|---|---|
| Provider-agnostic OpenAI client | Works with OpenAI, Anthropic (via proxy), Ollama, vLLM, LiteLLM, etc. |
| Two-pass review | Keeps initial context small; allows targeted deep dives without dumping the whole repo |
| Stable line numbers everywhere | Critical for precise findings that map back to PR/MR diff positions |
| Go AST parser as primary backend | Uses stdlib, no external dependencies, production-quality |
| Regex fallback for other languages | Gets partial functionality immediately; tree-sitter planned for later |
| Token estimation via `len/4` | Good enough for code; avoids tiktoken dependency; replaceable via interface |
| Config profiles | Lets users switch between OpenAI/local/work setups with one flag |
| No global state | Every dependency is injected; every function takes `context.Context` |
| Golden file testing | Catches output regressions; easy to update with `-update` flag |
| SARIF stub | Signals the intent without blocking v1; useful for future IDE/CI integration |

---

## 13. Dependencies

| Dependency | Purpose | Version |
|---|---|---|
| `github.com/spf13/cobra` | CLI framework | latest |
| `github.com/spf13/viper` | Config/env handling (optional, can use raw YAML) | latest |
| `gopkg.in/yaml.v3` | YAML config parsing | v3 |
| `github.com/stretchr/testify` | Test assertions | latest |
| `golang.org/x/tools/go/packages` | Go call graph analysis | latest |
| `github.com/fatih/color` | Terminal colors (respects NO_COLOR) | latest |
| `github.com/mattn/go-isatty` | Detect terminal for color auto-detection | latest |

No other external dependencies. The GitHub and GitLab clients use `net/http` directly (no SDK dependencies). The LLM client uses `net/http` directly.

---

## 14. Configuration Reference

### Environment Variables

| Variable | Description |
|---|---|
| `LLM_REVIEW_API_KEY` | API key for the LLM provider |
| `LLM_REVIEW_BASE_URL` | Base URL for the LLM API |
| `LLM_REVIEW_MODEL` | Model identifier |
| `GITHUB_TOKEN` | GitHub personal access token |
| `GITLAB_TOKEN` | GitLab personal access token |
| `GITLAB_BASE_URL` | GitLab instance URL (for self-hosted) |
| `NO_COLOR` | Disable terminal colors (any value) |

### CLI Flag → Env Var → Config File Mapping

| CLI Flag | Env Var | YAML Key | Default |
|---|---|---|---|
| `--api-key` | `LLM_REVIEW_API_KEY` | `profiles.<name>.api_key` | — |
| `--base-url` | `LLM_REVIEW_BASE_URL` | `profiles.<name>.base_url` | `https://api.openai.com/v1` |
| `--model` | `LLM_REVIEW_MODEL` | `profiles.<name>.model` | `gpt-4o` |
| `--max-context-tokens` | — | `profiles.<name>.max_context_tokens` | `120000` |
| `--followups` | — | `profiles.<name>.default_followups` | `1` |

---

## 15. Future Work (Out of Scope for v1)

- **Tree-sitter integration** for language-agnostic AST parsing.
- **SARIF output** for IDE and CI integration.
- **GitHub Actions action** (`uses: nickpit/review-action@v1`).
- **GitLab CI component** for pipeline integration.
- **PR comment posting** (write findings back as review comments).
- **Streaming output** for long reviews.
- **MCP server mode** for integration with Claude Code and other tools.
- **Multi-model review** (send to two models, compare findings).
- **Custom review profiles** (security-focused, performance-focused, etc.).
- **Incremental review** (only re-review changed hunks since last review).
