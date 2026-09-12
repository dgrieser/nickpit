package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

// A project reviewed by another group's bot carries markers this token cannot
// claim: the strict reader refuses them, the read-only one shows them.
func TestReviewResultsAnyAuthorReadsForeignMarkers(t *testing.T) {
	result := &model.ReviewResult{ReviewID: "r-1", OverallCorrectness: "patch is correct"}
	body, ok := reviewmd.NewRenderer("").SummaryBodyCarried(result)
	if !ok {
		t.Fatal("the summary marker did not fit")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/user"):
			_, _ = w.Write([]byte(`{"id":909,"username":"group_326_bot"}`))
		default:
			// Published by a different bot than this token's user.
			_, _ = fmt.Fprintf(w, `[{"id":1,"body":%q,"author":{"id":324,"username":"group_324_bot"}}]`, body)
		}
	}))
	defer server.Close()

	adapter := NewAdapter(NewClient(server.URL, "token"), "")
	strict, err := adapter.ReviewResults(context.Background(), "grp/proj", 17)
	if err != nil {
		t.Fatal(err)
	}
	if len(strict) != 0 {
		t.Fatalf("strict read = %v, want the foreign markers refused", strict)
	}
	any, err := adapter.ReviewResultsAnyAuthor(context.Background(), "grp/proj", 17)
	if err != nil {
		t.Fatal(err)
	}
	if len(any) != 1 || any["r-1"] == nil || any["r-1"].OverallCorrectness != "patch is correct" {
		t.Fatalf("read = %v, want the published review", any)
	}
}
