package forgejo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchPRStatus(t *testing.T) {
	tests := []struct {
		name string
		body string
		want PRStatus
	}{
		{
			name: "open",
			body: `{"state":"open","merged":false,"draft":false,"head":{"sha":"head1"},"base":{"sha":"tip1"},"merge_base":"fork1"}`,
			want: PRStatus{State: "open", HeadSHA: "head1", BaseSHA: "fork1"},
		},
		{
			// Forgejo has no merged state: a merged pull request is closed.
			name: "closed and merged",
			body: `{"state":"closed","merged":true,"draft":false,"head":{"sha":"head2"},"base":{"sha":"tip2"},"merge_base":"fork2"}`,
			want: PRStatus{State: "closed", Merged: true, HeadSHA: "head2", BaseSHA: "fork2"},
		},
		{
			name: "closed without merging",
			body: `{"state":"closed","merged":false,"head":{"sha":"head3"},"base":{"sha":"tip3"},"merge_base":"fork3"}`,
			want: PRStatus{State: "closed", HeadSHA: "head3", BaseSHA: "fork3"},
		},
		{
			name: "draft",
			body: `{"state":"open","draft":true,"head":{"sha":"head4"},"base":{"sha":"tip4"},"merge_base":"fork4"}`,
			want: PRStatus{State: "open", Draft: true, HeadSHA: "head4", BaseSHA: "fork4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			status, err := NewClient(server.URL, "token").FetchPRStatus(context.Background(), "owner/repo", 7)
			if err != nil {
				t.Fatal(err)
			}
			if gotMethod != http.MethodGet || gotPath != "/api/v1/repos/owner/repo/pulls/7" {
				t.Fatalf("request = %s %s", gotMethod, gotPath)
			}
			// The base is the merge base, never base.sha: that one is the base
			// branch's tip and moves without the pull request's diff changing.
			if *status != tt.want {
				t.Fatalf("status = %+v, want %+v", *status, tt.want)
			}
		})
	}
}

// The status base is what FetchPR records as the context's diff base, so a
// cached context can be compared against the live pull request.
func TestFetchPRStatusMatchesContextDiffRefs(t *testing.T) {
	server, _ := fixtureServer(t)
	client := NewClient(server.URL, "token")
	status, err := client.FetchPRStatus(context.Background(), "owner/repo", 123)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := client.FetchPR(context.Background(), "owner/repo", 123, false)
	if err != nil {
		t.Fatal(err)
	}
	if status.HeadSHA != ctx.DiffHeadSHA || status.BaseSHA != ctx.DiffBaseSHA {
		t.Fatalf("status = %s..%s, context = %s..%s", status.BaseSHA, status.HeadSHA, ctx.DiffBaseSHA, ctx.DiffHeadSHA)
	}
}

func TestFetchPRStatusSurfacesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	status, err := NewClient(server.URL, "token").FetchPRStatus(context.Background(), "owner/repo", 7)
	if status != nil || !IsNotFound(err) {
		t.Fatalf("status = %+v, err = %v, want a 404", status, err)
	}
}
