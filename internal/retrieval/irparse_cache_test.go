package retrieval

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/dgrieser/nickpit/internal/retrieval/tsparser"
)

func TestParseFileIRServesRepeatedParsesFromCache(t *testing.T) {
	src := []byte("def handler(payload):\n    return payload\n")

	first, err := parseFileIR("mod.py", src)
	if err != nil {
		t.Fatalf("parseFileIR: %v", err)
	}
	second, err := parseFileIR("mod.py", src)
	if err != nil {
		t.Fatalf("parseFileIR: %v", err)
	}
	if first != second {
		t.Fatal("identical path and content parsed twice")
	}
}

// The key includes the content hash: a rewritten file must not answer from the
// previous parse, or a review would reason about the pre-edit sources.
func TestParseFileIRReparsesChangedContent(t *testing.T) {
	first, err := parseFileIR("mod.py", []byte("def a():\n    return 1\n"))
	if err != nil {
		t.Fatalf("parseFileIR: %v", err)
	}
	second, err := parseFileIR("mod.py", []byte("def b():\n    return 2\n"))
	if err != nil {
		t.Fatalf("parseFileIR: %v", err)
	}
	if first == second {
		t.Fatal("changed content served the stale parse")
	}
	if got := second.Symbols[0].Name; got != "b" {
		t.Fatalf("symbol %q, want b", got)
	}
}

// The same path with the same bytes must never be parsed twice at the same
// time: concurrent lanes asking for one large file is exactly the OOM this
// cache exists to prevent, so all callers have to land on one entry.
func TestParseFileIRSingleFlightsConcurrentCallers(t *testing.T) {
	src := []byte("def concurrent_handler(payload):\n    return payload\n")
	const callers = 8

	results := make([]*tsparser.FileIR, callers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range callers {
		wg.Go(func() {
			<-start
			ir, err := parseFileIR("concurrent.py", src)
			if err != nil {
				t.Error(err)
				return
			}
			results[i] = ir
		})
	}
	close(start)
	wg.Wait()

	for i, ir := range results {
		if ir == nil {
			t.Fatalf("caller %d got no IR", i)
		}
		if ir != results[0] {
			t.Fatalf("caller %d got its own parse; the single-flight did not hold", i)
		}
	}
}

func TestFileIRCacheEvictsLeastRecentlyUsed(t *testing.T) {
	t.Setenv("NICKPIT_IR_CACHE_MAX_ENTRIES", "2")
	cache := referenceCacheStore[fileIRCacheEntry]{
		capEnv:      "NICKPIT_IR_CACHE_MAX_ENTRIES",
		capFallback: 2,
	}
	for i := range 5 {
		entry := cache.entry(fmt.Sprintf("f%d.py", i))
		entry.parsed.Store(true) // a finished parse, so the entry is evictable
	}
	if len(cache.entries) != 2 {
		t.Fatalf("cache holds %d entries, want 2", len(cache.entries))
	}
}

// Evicting an in-flight parse would let the next caller for the same bytes
// start a second one alongside it — the concurrent duplicate this cache exists
// to prevent. Cap pressure must therefore leave unfinished entries alone.
func TestFileIRCacheKeepsInFlightEntriesUnderCapPressure(t *testing.T) {
	t.Setenv("NICKPIT_IR_CACHE_MAX_ENTRIES", "2")
	cache := referenceCacheStore[fileIRCacheEntry]{
		capEnv:      "NICKPIT_IR_CACHE_MAX_ENTRIES",
		capFallback: 2,
	}
	inFlight := cache.entry("in-flight.py") // never marked parsed
	for i := range 5 {
		entry := cache.entry(fmt.Sprintf("f%d.py", i))
		entry.parsed.Store(true)
	}
	if got := cache.entry("in-flight.py"); got != inFlight {
		t.Fatal("in-flight entry was evicted, so a second concurrent parse of the same file is possible")
	}
	inFlight.parsed.Store(true)
	cache.entry("after.py")
	if _, ok := cache.entries["in-flight.py"]; ok && len(cache.entries) > 2 {
		t.Fatalf("finished entry stayed pinned: cache holds %d entries", len(cache.entries))
	}
}

// The store must not spin when every entry is still being built: it holds more
// than the cap for that moment instead. That overshoot has to be temporary —
// see TestFileIRCacheCompactsAfterInFlightParsesComplete.
func TestFileIRCacheHoldsMoreThanCapWhileEverythingIsInFlight(t *testing.T) {
	t.Setenv("NICKPIT_IR_CACHE_MAX_ENTRIES", "2")
	cache := referenceCacheStore[fileIRCacheEntry]{
		capEnv:      "NICKPIT_IR_CACHE_MAX_ENTRIES",
		capFallback: 2,
	}
	for i := range 5 {
		cache.entry(fmt.Sprintf("f%d.py", i))
	}
	if len(cache.entries) != 5 {
		t.Fatalf("cache holds %d entries, want all 5 in-flight entries retained", len(cache.entries))
	}
}

// Insertion cannot evict a retained entry, so a burst of concurrent first-time
// parses ends over the cap. Nothing else would bring it back down: a run whose
// remaining lookups all hit the cache would hold every one of those entries —
// and the source text in them — for the life of the process.
func TestFileIRCacheCompactsAfterInFlightParsesComplete(t *testing.T) {
	t.Setenv("NICKPIT_IR_CACHE_MAX_ENTRIES", "2")
	cache := referenceCacheStore[fileIRCacheEntry]{
		capEnv:      "NICKPIT_IR_CACHE_MAX_ENTRIES",
		capFallback: 2,
	}
	entries := make([]*fileIRCacheEntry, 0, 5)
	for i := range 5 {
		entries = append(entries, cache.entry(fmt.Sprintf("f%d.py", i)))
	}
	for _, entry := range entries {
		entry.parsed.Store(true)
		cache.compact()
	}
	if len(cache.entries) != 2 {
		t.Fatalf("cache holds %d entries after every parse finished, want 2", len(cache.entries))
	}
}

// The same through the real call path: five concurrent parses under a cap of
// two must not leave the process-wide cache over that cap.
func TestParseFileIRKeepsTheCacheAtItsCapAfterConcurrentParses(t *testing.T) {
	t.Setenv("NICKPIT_IR_CACHE_MAX_ENTRIES", "2")

	var wg sync.WaitGroup
	for i := range 5 {
		wg.Go(func() {
			src := fmt.Appendf(nil, "def handler_%d(payload):\n    return payload\n", i)
			if _, err := parseFileIR(fmt.Sprintf("burst%d.py", i), src); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()

	fileIRCache.mu.Lock()
	held := len(fileIRCache.entries)
	fileIRCache.mu.Unlock()
	if held > 2 {
		t.Fatalf("cache holds %d entries after the burst, want at most 2", held)
	}
}

// A panicking parse must not leave its entry pinned forever: sync.Once counts
// the call as done, so the entry would answer every later caller with an empty
// result and never leave the cache.
func TestParseFileIRReleasesEntryAfterPanic(t *testing.T) {
	entry := fileIRCache.entry("panic-probe.py")
	func() {
		defer func() { _ = recover() }()
		entry.once.Do(func() {
			defer entry.parsed.Store(true)
			panic("parser exploded")
		})
	}()
	if entry.retainInCache() {
		t.Fatal("entry stayed pinned after a panicking parse")
	}
}

// The IR cache holds thousands of files while the reference caches hold two
// repository roots, so one cap must not bound the other.
func TestCacheCapsAreIndependent(t *testing.T) {
	t.Setenv("NICKPIT_REFERENCE_CACHE_MAX_ENTRIES", "1")
	t.Setenv("NICKPIT_IR_CACHE_MAX_ENTRIES", "4")

	irCache := referenceCacheStore[fileIRCacheEntry]{capEnv: "NICKPIT_IR_CACHE_MAX_ENTRIES", capFallback: 1}
	var refCache referenceCacheStore[parsedReferenceCacheEntry]
	for i := range 4 {
		irCache.entry(strconv.Itoa(i))
		refCache.entry(strconv.Itoa(i))
	}
	if len(irCache.entries) != 4 {
		t.Fatalf("IR cache holds %d entries, want 4", len(irCache.entries))
	}
	if len(refCache.entries) != 1 {
		t.Fatalf("reference cache holds %d roots, want 1", len(refCache.entries))
	}
}

// An oversize file must reach the model as "not analyzed", never as "symbol
// absent": the model would otherwise conclude the code does not exist.
func TestUnparsedFileReasonReportsSizeSkip(t *testing.T) {
	repoRoot := t.TempDir()
	var b strings.Builder
	for i := range 200 {
		fmt.Fprintf(&b, "def handler_%d(payload):\n    return {\"a\": payload[\"u\"]}\n\n", i)
	}
	writeRetrievalFile(t, repoRoot, "big.py", b.String())
	t.Setenv("NICKPIT_MAX_STRUCTURAL_PARSE_BYTES", "64")

	reason := unparsedFileReason(repoRoot, "big.py")
	if reason == "" {
		t.Fatal("size-skipped file reported no reason")
	}
	if !strings.Contains(reason, "exceeds") {
		t.Fatalf("reason %q does not explain the skip", reason)
	}
}

func TestUnparsedFileReasonEmptyForParsedFile(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "small.py", "def handler(payload):\n    return payload\n")

	if reason := unparsedFileReason(repoRoot, "small.py"); reason != "" {
		t.Fatalf("parsed file reported reason %q", reason)
	}
}

// A lookup that lands on a size-skipped file must say the analysis never ran.
// Reporting a bare "not found" would let a reviewer conclude the symbol does
// not exist, which is exactly the wrong inference for a file nobody parsed.
func TestFindCallersOnSizeSkippedFileExplainsTheSkip(t *testing.T) {
	repoRoot := t.TempDir()
	var b strings.Builder
	for i := range 200 {
		fmt.Fprintf(&b, "def handler_%d(payload):\n    return {\"a\": payload[\"u\"]}\n\n", i)
	}
	writeRetrievalFile(t, repoRoot, "bot.py", b.String())
	t.Setenv("NICKPIT_MAX_STRUCTURAL_PARSE_BYTES", "64")

	engine := NewLocalEngine()
	_, err := engine.FindCallers(context.Background(), repoRoot, SymbolRef{Name: "handler_7", Path: "bot.py"}, 1)
	if err == nil {
		t.Fatal("expected a lookup failure for an unparsed file")
	}
	// The type is what routes the tool layer to the literal-search fallback; a
	// plain not-found would reach the model as a retrieval failure instead.
	var skipped *StructuralAnalysisSkippedError
	if !errors.As(err, &skipped) {
		t.Fatalf("error %q is not a StructuralAnalysisSkippedError", err)
	}
	if _, ok := errors.AsType[*SymbolNotFoundError](err); ok {
		t.Fatalf("a skipped parse must not claim the symbol is absent: %v", err)
	}
	if !strings.Contains(skipped.Reason, "unparsed") {
		t.Fatalf("reason %q does not disclose that the file was left unparsed", skipped.Reason)
	}
	if !strings.Contains(skipped.Reason, "literal search") {
		t.Fatalf("reason %q does not point at the fallback", skipped.Reason)
	}
}

// A directory- or repository-scoped lookup whose declaration lives only in a
// size-skipped file must still disclose the skip. Resolution fails before any
// call-graph traversal, so the note has to come from the resolver itself.
func TestFindCallersOnDirectoryScopeExplainsUnparsedFiles(t *testing.T) {
	repoRoot := t.TempDir()
	var b strings.Builder
	for i := range 200 {
		fmt.Fprintf(&b, "def handler_%d(payload):\n    return {\"a\": payload[\"u\"]}\n\n", i)
	}
	writeRetrievalFile(t, repoRoot, "pkg/bot.py", b.String())
	writeRetrievalFile(t, repoRoot, "pkg/small.py", "def helper(x):\n    return x\n")
	t.Setenv("NICKPIT_MAX_STRUCTURAL_PARSE_BYTES", "64")

	for _, scope := range []string{"pkg", ""} {
		engine := NewLocalEngine()
		_, err := engine.FindCallers(context.Background(), repoRoot, SymbolRef{Name: "handler_7", Path: scope}, 1)
		if err == nil {
			t.Fatalf("scope %q: expected a lookup failure", scope)
		}
		var skipped *StructuralAnalysisSkippedError
		if !errors.As(err, &skipped) {
			t.Fatalf("scope %q: error %q is not a StructuralAnalysisSkippedError", scope, err)
		}
		if !strings.Contains(skipped.Reason, "pkg/bot.py") {
			t.Fatalf("scope %q: reason %q does not name the unparsed file", scope, skipped.Reason)
		}
		if !strings.Contains(skipped.Reason, "literal search") {
			t.Fatalf("scope %q: reason %q does not point at the fallback", scope, skipped.Reason)
		}
	}
}

// A parser runtime failure and a size skip need different actions from whoever
// reads the note, so the recorded reason must survive rather than every file
// being reported as oversize.
func TestUnparsedNoteKeepsEachRecordedReason(t *testing.T) {
	note := unparsedNote(map[string]string{
		"big.py":    "source exceeds the tree-sitter parse size cap: 99 bytes exceeds the 64 byte cap",
		"broken.py": "parser runtime failed: grammar unavailable",
	})
	if !strings.Contains(note, "exceeds the tree-sitter parse size cap") {
		t.Fatalf("note %q lost the size-cap reason", note)
	}
	if !strings.Contains(note, "parser runtime failed") {
		t.Fatalf("note %q reported a parser failure as a size skip", note)
	}
	if !strings.Contains(note, "2 file(s)") {
		t.Fatalf("note %q does not count the files", note)
	}
}

func TestUnparsedNoteNamesASingleFileDirectly(t *testing.T) {
	note := unparsedNote(map[string]string{"bot.py": "parser returned no tree"})
	if !strings.Contains(note, "bot.py was left unparsed (parser returned no tree)") {
		t.Fatalf("note %q does not read as one file's skip", note)
	}
}

func TestUnparsedNoteBoundsTheFileListing(t *testing.T) {
	reasons := map[string]string{}
	for i := range 12 {
		reasons[fmt.Sprintf("f%d.py", i)] = "source exceeds the tree-sitter parse size cap"
	}
	note := unparsedNote(reasons)
	if strings.Count(note, ".py") > maxListedUnparsedFiles {
		t.Fatalf("note lists more than %d files: %q", maxListedUnparsedFiles, note)
	}
	if !strings.Contains(note, "and 7 more") {
		t.Fatalf("note %q does not account for the files it left out", note)
	}
}

func TestUnparsedNoteEmptyWhenEverythingParsed(t *testing.T) {
	if note := unparsedNote(nil); note != "" {
		t.Fatalf("note = %q, want empty", note)
	}
}
