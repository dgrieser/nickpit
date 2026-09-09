package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/session"
	"github.com/spf13/cobra"
)

func TestSessionLatestAsRawMarkdown(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionReview(t, store, "older")
	latest := saveSessionReview(t, store, "latest")

	var out bytes.Buffer
	a := &app{sessionDir: dir, outputFormat: "raw"}
	if err := a.runSessionTo(context.Background(), sessionOptions{}, nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "latest") || strings.Contains(out.String(), "older") {
		t.Fatalf("printed wrong session:\n%s", out.String())
	}
	if strings.ContainsRune(out.String(), '\x1b') {
		t.Fatalf("raw Markdown contains ANSI escapes:\n%q", out.String())
	}
	if latest.Result == nil || !strings.Contains(out.String(), "### latest") {
		t.Fatalf("missing raw Markdown title:\n%s", out.String())
	}
}

func TestSessionExplicitAsJSON(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sess := saveSessionReview(t, store, "chosen")
	saveSessionReview(t, store, "other")

	var out bytes.Buffer
	a := &app{sessionDir: dir, jsonOutput: true}
	if err := a.runSessionTo(context.Background(), sessionOptions{sessionID: sess.ID}, nil, &out); err != nil {
		t.Fatal(err)
	}
	var result model.ReviewResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(result.Findings) != 1 || result.Findings[0].Title != "chosen" {
		t.Fatalf("printed result = %+v", result.Findings)
	}
}

func TestSessionArgumentAndErrors(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sess := saveSessionReview(t, store, "argument")
	a := &app{sessionDir: dir, outputFormat: "raw"}

	var out bytes.Buffer
	if err := a.runSessionTo(context.Background(), sessionOptions{}, []string{sess.ID}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "argument") {
		t.Fatalf("argument session not printed:\n%s", out.String())
	}
	if err := a.runSessionTo(context.Background(), sessionOptions{sessionID: sess.ID}, []string{sess.ID}, &out); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("argument/flag conflict error = %v", err)
	}

	empty := &app{sessionDir: t.TempDir()}
	if err := empty.runSessionTo(context.Background(), sessionOptions{}, nil, &out); err == nil || !strings.Contains(err.Error(), "no saved sessions") {
		t.Fatalf("empty store error = %v", err)
	}

	noResult := session.New()
	if err := store.Save(noResult); err != nil {
		t.Fatal(err)
	}
	if err := a.runSessionTo(context.Background(), sessionOptions{sessionID: noResult.ID}, nil, &out); err == nil || !strings.Contains(err.Error(), "has no saved review") {
		t.Fatalf("missing result error = %v", err)
	}
}

func TestSessionClipboardCopiesUnstyledReview(t *testing.T) {
	// markdown and raw both have to reach the clipboard as Markdown source: the
	// styled variant only ever renders to a terminal, never to the clipboard.
	for _, format := range []string{"markdown", "raw"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			store, err := session.NewStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			sess := saveSessionReview(t, store, "copied")

			var copied []byte
			var out bytes.Buffer
			a := &app{sessionDir: dir, outputFormat: format}
			a.clipboardCopy = func(_ context.Context, data []byte) (string, error) {
				copied = data
				return "test-helper", nil
			}
			if err := a.runSessionTo(context.Background(), sessionOptions{clipboard: true}, nil, &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(copied), "### copied") {
				t.Fatalf("clipboard payload missing Markdown review:\n%s", copied)
			}
			if strings.ContainsRune(string(copied), '\x1b') {
				t.Fatalf("clipboard payload contains ANSI escapes:\n%q", copied)
			}
			// The confirmation replaces the review: printing both would defeat the copy.
			if strings.Contains(out.String(), "### copied") {
				t.Fatalf("review printed alongside the copy:\n%s", out.String())
			}
			for _, want := range []string{"Copied review of session " + sess.ID, "via test-helper"} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("confirmation %q missing %q", out.String(), want)
				}
			}
		})
	}
}

// failingWriter stands in for a closed pipe or a full disk.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestSessionClipboardSurvivesUnwritableConfirmation(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionReview(t, store, "copied")

	copied := false
	a := &app{sessionDir: dir, outputFormat: "raw"}
	a.clipboardCopy = func(_ context.Context, _ []byte) (string, error) {
		copied = true
		return "test-helper", nil
	}
	// The review is already on the clipboard, so a confirmation that cannot be
	// written must not report the command as failed.
	if err := a.runSessionTo(context.Background(), sessionOptions{clipboard: true}, nil, failingWriter{}); err != nil {
		t.Fatalf("copy reported as failed after an unwritable confirmation: %v", err)
	}
	if !copied {
		t.Fatal("clipboard was never written")
	}
}

func TestSessionClipboardJSONAndFailure(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionReview(t, store, "as-json")

	var copied []byte
	var out bytes.Buffer
	a := &app{sessionDir: dir, outputFormat: "json"}
	a.clipboardCopy = func(_ context.Context, data []byte) (string, error) {
		copied = data
		return "test-helper", nil
	}
	if err := a.runSessionTo(context.Background(), sessionOptions{clipboard: true}, nil, &out); err != nil {
		t.Fatal(err)
	}
	var result model.ReviewResult
	if err := json.Unmarshal(copied, &result); err != nil {
		t.Fatalf("clipboard payload is not the JSON output: %v\n%s", err, copied)
	}

	a.clipboardCopy = func(_ context.Context, _ []byte) (string, error) {
		return "", errors.New("no clipboard helper found in PATH")
	}
	err = a.runSessionTo(context.Background(), sessionOptions{clipboard: true}, nil, &out)
	if err == nil || !strings.Contains(err.Error(), "no clipboard helper found in PATH") {
		t.Fatalf("clipboard failure error = %v", err)
	}
}

func TestSessionRejectsJSONWithConflictingOutput(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"session", "--json", "--output", "raw"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("conflicting output flags error = %v", err)
	}
}

func TestResolveOutputFormat(t *testing.T) {
	tests := []struct {
		name       string
		app        app
		outputSet  bool
		wantFormat string
		wantJSON   bool
		wantErr    string
	}{
		{name: "default markdown", app: app{outputFormat: "markdown"}, wantFormat: "markdown"},
		{name: "short raw", app: app{outputFormat: " RAW "}, outputSet: true, wantFormat: "raw"},
		{name: "output json", app: app{outputFormat: "json"}, outputSet: true, wantFormat: "json", wantJSON: true},
		{name: "legacy json", app: app{outputFormat: "markdown", jsonOutput: true}, wantFormat: "json", wantJSON: true},
		{name: "legacy and explicit json", app: app{outputFormat: "json", jsonOutput: true}, outputSet: true, wantFormat: "json", wantJSON: true},
		{name: "legacy conflict", app: app{outputFormat: "raw", jsonOutput: true}, outputSet: true, wantErr: "cannot be combined"},
		{name: "invalid", app: app{outputFormat: "yaml"}, outputSet: true, wantErr: "expected markdown, json, or raw"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.app.resolveOutputFormat(tc.outputSet)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.app.outputFormat != tc.wantFormat || tc.app.jsonOutput != tc.wantJSON {
				t.Fatalf("format/json = %q/%v, want %q/%v", tc.app.outputFormat, tc.app.jsonOutput, tc.wantFormat, tc.wantJSON)
			}
		})
	}
}

func TestCompleteSessionIDs(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := saveSessionReview(t, store, "first")
	second := saveSessionReview(t, store, "second")

	a := &app{sessionDir: dir}
	got, directive := a.completeSessionIDs("")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v", directive)
	}
	if len(got) != 2 || got[0] != second.ID || got[1] != first.ID {
		t.Fatalf("candidates = %v, want newest first [%s %s]", got, second.ID, first.ID)
	}
	got, _ = a.completeSessionIDs(first.ID[:8])
	if len(got) != 1 || got[0] != first.ID {
		t.Fatalf("prefix candidates = %v, want [%s]", got, first.ID)
	}
	got, _ = a.completeSessionIDs("does-not-match")
	if len(got) != 0 {
		t.Fatalf("nonmatching candidates = %v", got)
	}
}

func saveSessionReview(t *testing.T, store *session.Store, title string) *session.Session {
	t.Helper()
	priority := 1
	sess := session.New()
	sess.Result = &model.ReviewResult{
		OverallCorrectness: "patch is incorrect",
		Findings: []model.Finding{{
			Title: title, Body: title + " body", Priority: &priority,
			CodeLocation: model.CodeLocation{FilePath: title + ".go", LineRange: model.LineRange{Start: 1, End: 1}},
		}},
	}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	return sess
}

func saveSessionWarnings(t *testing.T, store *session.Store, warnings ...string) *session.Session {
	t.Helper()
	sess := saveSessionReview(t, store, "warned")
	sess.Result.Warnings = warnings
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	return sess
}

func TestSessionWarningsOnlyPrintsEveryWarning(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionWarnings(t, store,
		"Time budget for step review exhausted after 300s",
		"Verify step failed: context deadline exceeded",
	)

	var out bytes.Buffer
	a := &app{sessionDir: dir, outputFormat: "raw"}
	if err := a.runSessionTo(context.Background(), sessionOptions{warnings: true}, nil, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"! Warnings: 2 (Budget: 1, Verify: 1)",
		"[Budget] Time budget for step review exhausted after 300s",
		"[Verify] Verify step failed: context deadline exceeded",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("warnings output %q missing %q", out.String(), want)
		}
	}
	// --warnings replaces the review: the findings must not be printed too.
	if strings.Contains(out.String(), "### warned") {
		t.Fatalf("review printed alongside the warnings:\n%s", out.String())
	}
}

func TestSessionWarningsOnlyWithoutWarnings(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionReview(t, store, "clean")

	var out bytes.Buffer
	a := &app{sessionDir: dir, outputFormat: "raw"}
	if err := a.runSessionTo(context.Background(), sessionOptions{warnings: true}, nil, &out); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "No warnings." {
		t.Fatalf("output for a warning-free session = %q", out.String())
	}
}

func TestSessionWarningsOnlyAsJSON(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionWarnings(t, store, "Publish failed: 403")

	var out bytes.Buffer
	a := &app{sessionDir: dir, jsonOutput: true}
	if err := a.runSessionTo(context.Background(), sessionOptions{warnings: true}, nil, &out); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Warnings []string        `json:"warnings"`
		Findings []model.Finding `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(payload.Warnings) != 1 || payload.Warnings[0] != "Publish failed: 403" {
		t.Fatalf("warnings = %#v", payload.Warnings)
	}
	if len(payload.Findings) != 0 {
		t.Fatalf("findings leaked into the warnings output: %#v", payload.Findings)
	}
}

func TestSessionWarningsToClipboard(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sess := saveSessionWarnings(t, store, "Verify step failed: context deadline exceeded")

	var copied []byte
	var out bytes.Buffer
	a := &app{sessionDir: dir, outputFormat: "markdown"}
	a.clipboardCopy = func(_ context.Context, data []byte) (string, error) {
		copied = data
		return "test-helper", nil
	}
	if err := a.runSessionTo(context.Background(), sessionOptions{warnings: true, clipboard: true}, nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(copied), "[Verify] Verify step failed") {
		t.Fatalf("clipboard payload missing the warnings:\n%s", copied)
	}
	if strings.ContainsRune(string(copied), '\x1b') {
		t.Fatalf("clipboard payload contains ANSI escapes:\n%q", copied)
	}
	if !strings.Contains(out.String(), "Copied warnings of session "+sess.ID) {
		t.Fatalf("confirmation does not name the warnings: %q", out.String())
	}
}

func TestSessionReviewHistoryOutput(t *testing.T) {
	dir := t.TempDir()
	store, _ := session.NewStore(dir)
	sess := saveSessionReview(t, store, "original version")
	next, _ := sess.Result.Clone()
	next.Revision = 1
	next.Findings[0].Title = "corrected version"
	if err := sess.RecordReviewUpdate(next, "Evidence explains correction."); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"raw", "markdown", "json"} {
		for _, copy := range []bool{false, true} {
			var out bytes.Buffer
			a := &app{sessionDir: dir, outputFormat: format}
			var copied string
			a.clipboardCopy = func(_ context.Context, b []byte) (string, error) { copied = string(b); return "test", nil }
			if err := a.runSessionTo(context.Background(), sessionOptions{sessionID: sess.ID, history: true, clipboard: copy}, nil, &out); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			if copy {
				text = copied
			}
			if !strings.Contains(text, "original version") || strings.Contains(text, "corrected version") || !strings.Contains(text, "Evidence explains correction") {
				t.Fatalf("wrong history: %s", text)
			}
			if format == "json" {
				var entries []session.ReviewRevision
				if err := json.Unmarshal([]byte(text), &entries); err != nil || len(entries) != 1 {
					t.Fatalf("%s %v", text, err)
				}
			}
		}
	}
	var out bytes.Buffer
	a := &app{sessionDir: dir}
	if err := a.runSessionTo(context.Background(), sessionOptions{history: true, warnings: true}, nil, &out); err == nil {
		t.Fatal("conflicting flags accepted")
	}
	a.outputFormat = "json"
	if err := a.formatReviewHistory(&out, nil); err != nil || strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("empty history: %q %v", out.String(), err)
	}
}
