package review

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/prompts"
)

// securityGuide is one guide from prompts/security/. Every security review
// gets all of them: general.md first, then the domain guides by name.
type securityGuide struct {
	Domain  string
	Content string
}

const (
	securityGuideDir          = "security"
	securityGeneralGuide      = "general"
	securityValidationHeading = "#### Validate before reporting"
)

var (
	securityGuidesOnce   sync.Once
	securityGuidesCached []securityGuide
	securityGuidesErr    error

	securityVerifyOnce     sync.Once
	securityVerifyGuidance string
	securityVerifyErr      error
)

// securityGuides returns every embedded security guide, general first. The
// set is fixed at build time, so it is loaded once.
func securityGuides() ([]securityGuide, error) {
	securityGuidesOnce.Do(func() {
		securityGuidesCached, securityGuidesErr = loadSecurityGuides()
	})
	return securityGuidesCached, securityGuidesErr
}

func loadSecurityGuides() ([]securityGuide, error) {
	names, err := fs.Glob(prompts.FS, securityGuideDir+"/*.md")
	if err != nil {
		return nil, fmt.Errorf("review: listing security guides: %w", err)
	}
	guides := make([]securityGuide, 0, len(names))
	for _, name := range names {
		content, err := prompts.Load(name)
		if err != nil {
			return nil, fmt.Errorf("review: loading security guide: %w", err)
		}
		guides = append(guides, securityGuide{
			Domain:  strings.TrimSuffix(path.Base(name), ".md"),
			Content: strings.TrimSpace(content),
		})
	}
	slices.SortStableFunc(guides, func(a, b securityGuide) int {
		switch {
		case a.Domain == b.Domain:
			return 0
		case a.Domain == securityGeneralGuide:
			return -1
		case b.Domain == securityGeneralGuide:
			return 1
		}
		return strings.Compare(a.Domain, b.Domain)
	})
	return guides, nil
}

// securityValidationGuides reduces each guide to its title line and its
// "Validate before reporting" section: the verifier judges one finding and
// needs the rules that separate a real finding from a non-finding, not the
// hunting lists.
func securityValidationGuides(guides []securityGuide) []securityGuide {
	out := make([]securityGuide, 0, len(guides))
	for _, guide := range guides {
		_, section, ok := strings.Cut(guide.Content, securityValidationHeading)
		if !ok {
			continue
		}
		title, _, _ := strings.Cut(guide.Content, "\n")
		out = append(out, securityGuide{
			Domain:  guide.Domain,
			Content: strings.TrimSpace(title) + "\n" + securityValidationHeading + "\n" + strings.TrimSpace(section),
		})
	}
	return out
}

// securityVerifyRules renders the verifier rules for findings of the security
// reviewer with the validation sections of every guide. The result does not
// depend on the review, so it is rendered once.
func securityVerifyRules() (string, error) {
	securityVerifyOnce.Do(func() {
		securityVerifyGuidance, securityVerifyErr = renderSecurityVerifyRules()
	})
	return securityVerifyGuidance, securityVerifyErr
}

func renderSecurityVerifyRules() (string, error) {
	guides, err := securityGuides()
	if err != nil {
		return "", err
	}
	tmpl, err := prompts.Load("agent_verify_security_snippet.tmpl")
	if err != nil {
		return "", err
	}
	rendered, err := llm.RenderPrompt(tmpl, struct {
		SecurityGuides []securityGuide
	}{
		SecurityGuides: securityValidationGuides(guides),
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering security verify rules: %w", err)
	}
	return strings.TrimSpace(rendered), nil
}

// verifyFocusGuidances returns, aligned with refs, the focus-specific verifier
// guidance for each finding's group: findings of a reviewer vector with
// security guides get the security rules, every other finding none.
func verifyFocusGuidances(groups []verifyGroup, refs []findingRef) ([]string, error) {
	out := make([]string, len(refs))
	for i, ref := range refs {
		vector, ok := reviewVectorByID(groupAt(groups, ref.vectorIdx).id)
		if !ok || !vector.securityGuides {
			continue
		}
		rules, err := securityVerifyRules()
		if err != nil {
			return nil, err
		}
		out[i] = rules
	}
	return out, nil
}
