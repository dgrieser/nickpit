package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/session"
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
	if err := a.runSessionTo(sessionOptions{}, nil, &out); err != nil {
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
	if err := a.runSessionTo(sessionOptions{sessionID: sess.ID}, nil, &out); err != nil {
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
	if err := a.runSessionTo(sessionOptions{}, []string{sess.ID}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "argument") {
		t.Fatalf("argument session not printed:\n%s", out.String())
	}
	if err := a.runSessionTo(sessionOptions{sessionID: sess.ID}, []string{sess.ID}, &out); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("argument/flag conflict error = %v", err)
	}

	empty := &app{sessionDir: t.TempDir()}
	if err := empty.runSessionTo(sessionOptions{}, nil, &out); err == nil || !strings.Contains(err.Error(), "no saved sessions") {
		t.Fatalf("empty store error = %v", err)
	}

	noResult := session.New()
	if err := store.Save(noResult); err != nil {
		t.Fatal(err)
	}
	if err := a.runSessionTo(sessionOptions{sessionID: noResult.ID}, nil, &out); err == nil || !strings.Contains(err.Error(), "has no saved review") {
		t.Fatalf("missing result error = %v", err)
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
