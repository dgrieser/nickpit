package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
)

// publishedReviews stands in for what an MR/PR carries: an older review and a
// newer one with fewer findings, so ordering cannot be mistaken for a
// findings-count effect.
func publishedReviews() map[string]*model.ReviewResult {
	return map[string]*model.ReviewResult{
		"rev-old": {
			ReviewID: "rev-old", Revision: 1, Model: "model-a", NickpitVersion: "v0.0.1",
			CreatedAt:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			OverallCorrectness: "patch is correct",
			Findings: []model.Finding{
				{ID: "f1", Title: "older one", Body: "b1", CodeLocation: model.CodeLocation{FilePath: "a.go"}},
				{ID: "f2", Title: "older two", Body: "b2", CodeLocation: model.CodeLocation{FilePath: "b.go"}},
			},
		},
		"rev-new": {
			ReviewID: "rev-new", Revision: 2, Model: "model-b", NickpitVersion: "v0.0.2",
			CreatedAt:          time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			OverallCorrectness: "patch is incorrect",
			Findings: []model.Finding{
				{ID: "f3", Title: "newer only", Body: "b3", CodeLocation: model.CodeLocation{FilePath: "c.go"}},
			},
		},
	}
}

func gitlabFeedbackSource(reviews map[string]*model.ReviewResult, err error) feedbackSource {
	return feedbackSource{
		noun:   "merge request",
		origin: "GitLab MR grp/proj!42",
		load: func(context.Context) (map[string]*model.ReviewResult, error) {
			return reviews, err
		},
	}
}

func TestFeedbackPrintsNewestPublishedReview(t *testing.T) {
	var out bytes.Buffer
	a := &app{outputFormat: "raw"}
	if err := a.runFeedback(context.Background(), &out, feedbackOptions{}, gitlabFeedbackSource(publishedReviews(), nil)); err != nil {
		t.Fatal(err)
	}
	// The newest review wins even though the older one carries more findings.
	if !strings.Contains(out.String(), "newer only") || strings.Contains(out.String(), "older one") {
		t.Fatalf("printed the wrong review:\n%s", out.String())
	}

	out.Reset()
	if err := a.runFeedback(context.Background(), &out, feedbackOptions{reviewID: "rev-old"}, gitlabFeedbackSource(publishedReviews(), nil)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "older one") || strings.Contains(out.String(), "newer only") {
		t.Fatalf("--review-id did not select the older review:\n%s", out.String())
	}
}

func TestFeedbackAsJSON(t *testing.T) {
	var out bytes.Buffer
	a := &app{jsonOutput: true}
	if err := a.runFeedback(context.Background(), &out, feedbackOptions{}, gitlabFeedbackSource(publishedReviews(), nil)); err != nil {
		t.Fatal(err)
	}
	var result model.ReviewResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if result.ReviewID != "rev-new" || len(result.Findings) != 1 {
		t.Fatalf("decoded review = %+v", result)
	}
}

func TestFeedbackClipboardCopiesUnstyledReview(t *testing.T) {
	var copied []byte
	var out bytes.Buffer
	a := &app{outputFormat: "markdown"}
	a.clipboardCopy = func(_ context.Context, data []byte) (string, error) {
		copied = data
		return "test-helper", nil
	}
	if err := a.runFeedback(context.Background(), &out, feedbackOptions{clipboard: true}, gitlabFeedbackSource(publishedReviews(), nil)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(copied), "newer only") {
		t.Fatalf("clipboard payload missing the review:\n%s", copied)
	}
	if strings.ContainsRune(string(copied), '\x1b') {
		t.Fatalf("clipboard payload contains ANSI escapes:\n%q", copied)
	}
	// The confirmation replaces the review: printing both would defeat the copy.
	if strings.Contains(out.String(), "newer only") {
		t.Fatalf("review printed alongside the copy:\n%s", out.String())
	}
	for _, want := range []string{"Copied review of GitLab MR grp/proj!42", "via test-helper"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("confirmation %q missing %q", out.String(), want)
		}
	}
}

func TestFeedbackListOrdersNewestFirst(t *testing.T) {
	var out bytes.Buffer
	a := &app{outputFormat: "raw"}
	if err := a.runFeedback(context.Background(), &out, feedbackOptions{list: true}, gitlabFeedbackSource(publishedReviews(), nil)); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	newest, oldest := strings.Index(text, "rev-new"), strings.Index(text, "rev-old")
	if newest < 0 || oldest < 0 || newest > oldest {
		t.Fatalf("listing is not newest first:\n%s", text)
	}
	// The listing has to say enough to pick one with --review-id.
	for _, want := range []string{"2 review(s)", "model-b", "v0.0.2", "patch is incorrect"} {
		if !strings.Contains(text, want) {
			t.Fatalf("listing %q missing %q", text, want)
		}
	}

	out.Reset()
	a = &app{jsonOutput: true}
	if err := a.runFeedback(context.Background(), &out, feedbackOptions{list: true}, gitlabFeedbackSource(publishedReviews(), nil)); err != nil {
		t.Fatal(err)
	}
	var entries []reviewListEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(entries) != 2 || entries[0].ReviewID != "rev-new" || entries[0].Findings != 1 || entries[0].Revision != 2 {
		t.Fatalf("JSON listing = %+v", entries)
	}

	// An empty request lists nothing instead of failing: --list answers "what is
	// on this request", and "nothing" is a valid answer.
	out.Reset()
	a = &app{outputFormat: "raw"}
	if err := a.runFeedback(context.Background(), &out, feedbackOptions{list: true}, gitlabFeedbackSource(nil, nil)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No complete NickPit review found on the merge request") {
		t.Fatalf("empty listing = %q", out.String())
	}
}

func TestFeedbackErrorsNameTheRequest(t *testing.T) {
	a := &app{outputFormat: "raw"}
	var out bytes.Buffer

	err := a.runFeedback(context.Background(), &out, feedbackOptions{}, gitlabFeedbackSource(nil, nil))
	if err == nil || !strings.Contains(err.Error(), "no complete nickpit review found on the merge request") {
		t.Fatalf("empty request error = %v", err)
	}

	err = a.runFeedback(context.Background(), &out, feedbackOptions{reviewID: "nope"}, gitlabFeedbackSource(publishedReviews(), nil))
	if err == nil || !strings.Contains(err.Error(), `review id "nope" not found on the merge request`) {
		t.Fatalf("unknown review id error = %v", err)
	}

	err = a.runFeedback(context.Background(), &out, feedbackOptions{}, gitlabFeedbackSource(nil, errors.New("401 Unauthorized")))
	if err == nil || !strings.Contains(err.Error(), "reading published reviews: 401 Unauthorized") {
		t.Fatalf("load error = %v", err)
	}

	// The pull-request wording follows the platform, not the GitLab default.
	err = a.runFeedback(context.Background(), &out, feedbackOptions{}, feedbackSource{
		noun: "pull request",
		load: func(context.Context) (map[string]*model.ReviewResult, error) { return nil, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "on the pull request") {
		t.Fatalf("github wording error = %v", err)
	}
}

func TestFeedbackRequestFlagValidation(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"url with id", []string{"--url", "https://gitlab.example.com/grp/proj/-/merge_requests/42", "--id", "42"}, "--url can not be combined with --id"},
		{"url with repo", []string{"--url", "https://gitlab.example.com/grp/proj/-/merge_requests/42", "--repo", "grp/proj"}, "--url can not be combined with --repo"},
		{"missing id", []string{"--repo", "grp/proj"}, "--id must be a positive integer"},
		{"unparsable url", []string{"--url", "https://gitlab.example.com/grp/proj"}, "must be a GitLab MR URL"},
		{"list with clipboard", []string{"--repo", "grp/proj", "--id", "42", "--list", "--clipboard"}, "clipboard"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &app{outputFormat: "raw"}
			cmd := a.newGitLabFeedbackCmd()
			cmd.SetArgs(tc.args)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
