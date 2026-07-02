package llm

import (
	"bufio"
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"
	"unicode/utf8"
)

// TestReasoningLoopCorpus replays reasoning traces extracted from real review
// logs through the loop detector and reports detection quality. It is a tuning
// harness, not a regression gate: it only runs when NICKPIT_LOOP_CORPUS points
// to a JSONL corpus produced by tools/loop_corpus_extract.py, which
// mines review_test-*.log files for reasoning blocks and labels them from the
// retry markers in the same log.
//
// Corpus record kinds:
//
//	loopdet - the run's detector caught a loop in this call (must re-catch)
//	timeout - the call hit max_reasoning_seconds (undetected loop, catch early)
//	chunk   - the provider aborted a repeated chunk (catch if visible)
//	empty   - reasoning-only empty response (loop-ish, catch when repetitive)
//	clean   - normal call (must NOT fire)
type corpusRecord struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Kind     string   `json:"kind"`
	Duration *float64 `json:"duration_s"`
	Chars    int      `json:"chars"`
	Text     string   `json:"text"`
}

func TestReasoningLoopCorpus(t *testing.T) {
	path := os.Getenv("NICKPIT_LOOP_CORPUS")
	if path == "" {
		t.Skip("set NICKPIT_LOOP_CORPUS to a corpus JSONL file to run the tuning harness")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open corpus: %v", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("close corpus: %v", err)
		}
	}()

	const budget = 300 * time.Second
	const fallbackCharsPerSecond = 150.0

	type result struct {
		id       string
		kind     string
		fired    bool
		fireFrac float64 // fraction of the trace consumed when fired
		fireSec  float64 // simulated seconds elapsed when fired
		durSec   float64
	}
	var results []result

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		var rec corpusRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			t.Fatalf("parse corpus line: %v", err)
		}
		dur := float64(len(rec.Text)) / fallbackCharsPerSecond
		if rec.Duration != nil && *rec.Duration > 0 {
			dur = *rec.Duration
		}

		base := time.Unix(0, 0)
		clock := base
		canceled := false
		d := newReasoningLoopDetector(func() { canceled = true }, budget)
		d.now = func() time.Time { return clock }

		text := rec.Text
		res := result{id: rec.ID, kind: rec.Kind, durSec: dur}
		const step = 100
		for off := 0; off < len(text); {
			end := min(off+step, len(text))
			// Never split a multi-byte rune across deltas: the range loop in
			// onDelta would decode the fragments as U+FFFD and distort both
			// the detector input and the replayed byte offsets.
			for end < len(text) && !utf8.RuneStart(text[end]) {
				end++
			}
			clock = base.Add(time.Duration(float64(end) / float64(len(text)) * dur * float64(time.Second)))
			d.onDelta(text[off:end])
			if canceled {
				res.fired = true
				res.fireFrac = float64(end) / float64(len(text))
				res.fireSec = clock.Sub(base).Seconds()
				break
			}
			off = end
		}
		results = append(results, res)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read corpus: %v", err)
	}

	byKind := map[string][]result{}
	for _, r := range results {
		byKind[r.kind] = append(byKind[r.kind], r)
	}
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		rs := byKind[kind]
		fired := 0
		var fracs, secs []float64
		for _, r := range rs {
			if r.fired {
				fired++
				fracs = append(fracs, r.fireFrac)
				secs = append(secs, r.fireSec)
			}
		}
		sort.Float64s(fracs)
		sort.Float64s(secs)
		med := func(v []float64) float64 {
			if len(v) == 0 {
				return 0
			}
			return v[len(v)/2]
		}
		t.Logf("%-8s n=%-4d fired=%-4d (%.0f%%)  median fire: %.0f%% of trace, %.0fs",
			kind, len(rs), fired, 100*float64(fired)/float64(len(rs)), 100*med(fracs), med(secs))
	}

	// Detail lines: every fired clean trace is a false positive to investigate;
	// every missed loopdet/timeout trace is a regression against the old
	// detector or a wasted-time case.
	for _, r := range results {
		switch {
		case r.kind == "clean" && r.fired:
			t.Logf("FP  %-22s fired at %.0f%% (%.0fs of %.0fs)", r.id, 100*r.fireFrac, r.fireSec, r.durSec)
		case (r.kind == "loopdet" || r.kind == "timeout") && !r.fired:
			t.Logf("MISS %-22s %s (%.0fs)", r.id, r.kind, r.durSec)
		case (r.kind == "loopdet" || r.kind == "timeout") && r.fired:
			t.Logf("HIT %-22s %-8s fired at %.0f%% (%.0fs of %.0fs)", r.id, r.kind, 100*r.fireFrac, r.fireSec, r.durSec)
		}
	}

	if os.Getenv("NICKPIT_LOOP_CORPUS_DUMP") != "" {
		out, err := os.Create(os.Getenv("NICKPIT_LOOP_CORPUS_DUMP"))
		if err != nil {
			t.Fatalf("create dump: %v", err)
		}
		defer func() {
			if err := out.Close(); err != nil {
				t.Errorf("close dump: %v", err)
			}
		}()
		enc := json.NewEncoder(out)
		for _, r := range results {
			_ = enc.Encode(map[string]any{
				"id": r.id, "kind": r.kind, "fired": r.fired,
				"fire_frac": r.fireFrac, "fire_sec": r.fireSec, "dur_sec": r.durSec,
			})
		}
	}
}
