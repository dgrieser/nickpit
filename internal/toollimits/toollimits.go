// Package toollimits holds the limits and defaults that define what the agent
// tools accept and return: schemas, execution and result pruning all read them
// from here so they cannot drift apart.
//
// It deliberately imports nothing. The catalog in internal/tools describes the
// tools to the model and therefore depends on internal/llm; keeping the numbers
// separate lets low-level packages — git plumbing, config loading, retrieval —
// honor the same limits without taking a dependency on the LLM layer.
//
// Limits that are not part of any tool's contract stay with the code that
// enforces them: internal/git owns its patch-byte, deepen and ambiguity caps,
// which bound git command output rather than a tool's request or response.
// Token limits are context-dependent and live in the review/config layers.
package toollimits

const (
	DefaultListFilesDepth      = 1
	DefaultSearchContextLines  = 5
	MaxSearchStructuralLookups = 20
	// MaxFallbackSearchResults bounds the literal searches the engine runs on
	// its own behalf when a structural lookup degrades — a common identifier
	// would otherwise stream every match in the repository into the result.
	MaxFallbackSearchResults = 100
	// MaxOpportunisticGoLoadFiles is the largest Go file count an opportunistic
	// lookup (one that set AvoidGoLoad) may trigger a whole-repository
	// type-check for. Above it the snapshot is used only when already cached:
	// packages.Load over a monorepo takes minutes, which an explicit
	// find_references may spend but a rewritten literal search must not.
	MaxOpportunisticGoLoadFiles  = 500
	MaxFindLinesMatches          = 100
	DefaultCallHierarchyDepth    = 10
	MaxCallHierarchyDepth        = 50
	MaxReferenceFunctions        = 25
	MaxAmbiguousReferenceTargets = 10
	MaxRetrievedFileBytes        = 5 << 20

	// DefaultStaticGraphCacheEntries bounds how many distinct (language,
	// repoRoot, scope) call graphs one run memoizes.
	DefaultStaticGraphCacheEntries = 64
	// DefaultReferenceCacheEntries counts repository roots, and applies to the
	// parsed-source snapshot and the type-checked Go snapshot separately, so
	// two roots can retain up to two of each. Each snapshot holds a whole
	// repository, so this stays small: one root covers a review, the second is
	// headroom for a concurrent one.
	DefaultReferenceCacheEntries = 2

	DefaultGitLogLimit    = 20
	MaxGitLogLimit        = 200
	DefaultGitShowCommits = 10
	MaxGitShowCommits     = 50

	// DefaultMaxToolCalls is 0, meaning unlimited calls per agent.
	DefaultMaxToolCalls          = 0
	DefaultMaxDuplicateToolCalls = 5
)
