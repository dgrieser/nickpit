package review

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/prompts"
)

func TestSecurityGuidesLoadAllGeneralFirst(t *testing.T) {
	guides, err := securityGuides()
	if err != nil {
		t.Fatal(err)
	}
	names, err := fs.Glob(prompts.FS, securityGuideDir+"/*.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(guides) != len(names) || len(guides) < 2 {
		t.Fatalf("guides = %d, embedded files = %d", len(guides), len(names))
	}
	if guides[0].Domain != securityGeneralGuide {
		t.Fatalf("first guide = %q, want %q", guides[0].Domain, securityGeneralGuide)
	}
	for i, guide := range guides {
		if i > 1 && guide.Domain < guides[i-1].Domain {
			t.Errorf("domain guides not sorted: %q after %q", guide.Domain, guides[i-1].Domain)
		}
		if !strings.HasPrefix(guide.Content, "### ") {
			t.Errorf("guide %q must start with a ### title line", guide.Domain)
		}
		if !strings.Contains(guide.Content, securityValidationHeading) {
			t.Errorf("guide %q lacks the validation section the verifier receives", guide.Domain)
		}
		if strings.Contains(guide.Content, "{{") || strings.Contains(guide.Content, "}}") {
			t.Errorf("guide %q contains template delimiters", guide.Domain)
		}
	}
}

func TestSecurityValidationGuidesKeepTitleAndValidationOnly(t *testing.T) {
	guides := securityValidationGuides([]securityGuide{
		{Domain: "x", Content: "### X security guide\nApplies to: x\n\n#### Hunt for\n- hunt item\n\n#### Validate before reporting\n- rule one\n"},
		{Domain: "y", Content: "### Y without section\n- nothing"},
	})
	if len(guides) != 1 {
		t.Fatalf("guides = %d, want 1 (a guide without a validation section is skipped)", len(guides))
	}
	want := "### X security guide\n#### Validate before reporting\n- rule one"
	if guides[0].Content != want {
		t.Fatalf("content = %q, want %q", guides[0].Content, want)
	}
}

func TestSecurityReviewerFocusRendersGuides(t *testing.T) {
	engine := &Engine{}
	guides, err := securityGuides()
	if err != nil {
		t.Fatal(err)
	}
	vector, _ := reviewVectorByID("security")
	if !vector.securityGuides {
		t.Fatal("security vector must request security guides")
	}
	rendered, err := engine.renderReviewerFocusSnippet(vector.focusFile, "- question", guides)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range append([]string{"### SECURITY GUIDES", "- question"}, guideTitles(guides)...) {
		if !strings.Contains(rendered, want) {
			t.Errorf("security focus prompt missing %q", want)
		}
	}
	bare, err := engine.renderReviewerFocusSnippet(vector.focusFile, "- question", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(bare, "### SECURITY GUIDES") {
		t.Error("security focus prompt renders the guides heading without guides")
	}
}

func TestVerifyFocusGuidancesOnlyForSecurityFindings(t *testing.T) {
	groups := []verifyGroup{{id: "codequality"}, {id: "security"}, {id: "published-review"}}
	refs := []findingRef{{vectorIdx: 0}, {vectorIdx: 1}, {vectorIdx: 1, findingIdx: 1}, {vectorIdx: 2}}
	guidances, err := verifyFocusGuidances(groups, refs)
	if err != nil {
		t.Fatal(err)
	}
	if guidances[0] != "" || guidances[3] != "" {
		t.Fatalf("non-security findings got guidance: %q / %q", guidances[0], guidances[3])
	}
	guides, err := securityGuides()
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{1, 2} {
		g := guidances[i]
		if !strings.HasPrefix(g, "## SECURITY VERIFICATION RULES") {
			t.Fatalf("security finding %d guidance = %q", i, g)
		}
		if strings.Contains(g, "#### Hunt for") {
			t.Fatalf("security finding %d guidance must carry validation sections only", i)
		}
		if got := strings.Count(g, securityValidationHeading); got != len(guides) {
			t.Fatalf("security finding %d guidance has %d validation sections, want %d", i, got, len(guides))
		}
	}
}

func guideTitles(guides []securityGuide) []string {
	titles := make([]string, 0, len(guides))
	for _, guide := range guides {
		title, _, _ := strings.Cut(guide.Content, "\n")
		titles = append(titles, title)
	}
	return titles
}
