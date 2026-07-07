// Package versionmatch selects a version-specific styleguide by matching a
// detected toolchain version (from go.mod, Dockerfile, CI, etc.) against
// configured version keys. Detected version strings are messy in practice
// ("1.19", "1.19.3", "go1.21.0", "1.21-alpine", ">=3.10", "^18", "latest"), so
// matching prefers real semver-constraint evaluation and falls back to a plain
// exact/minor-prefix string match when either side is not semver.
//
// It is a leaf package: it imports only Masterminds/semver and the standard
// library, so both the mappings package (built-in guide selection) and the
// review engine (user-config guide gating) can depend on it without cycles.
package versionmatch

import (
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// coreRe extracts the leading dotted-numeric core of a version token, e.g.
// "1.21.0" from "1.21.0-alpine" or "1.21" from "1.21-slim". Up to three
// components (major[.minor[.patch]]), with an optional leading "v".
var coreRe = regexp.MustCompile(`^v?(\d+(?:\.\d+){0,2})`)

// bareVersionRe matches a plain version with no operator or wildcard, e.g.
// "1", "1.19", "v1.19.3". Such a configured key is treated as a minor-range
// wildcard (see normalizeConstraintKey) so "1.19" matches "1.19.3".
var bareVersionRe = regexp.MustCompile(`^v?\d+(?:\.\d+){0,2}$`)

// coreVersion returns the numeric core of a raw version string, stripping a
// leading "go" language prefix (as emitted by go.mod's toolchain directive,
// e.g. "go1.21.0") and any suffix such as "-alpine". It returns "" when the
// string has no leading numeric core (e.g. "latest", "^18", ">=3.10").
func coreVersion(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	if len(s) >= 3 && strings.HasPrefix(s, "go") && s[2] >= '0' && s[2] <= '9' {
		s = s[2:]
	}
	return coreRe.FindString(s)
}

// normalizeConstraintKey turns a bare "major.minor" (or "major") key into a
// wildcard range so it matches every patch/minor beneath it: "1.19" becomes
// "1.19.x" (>=1.19.0 <1.20.0), matching detected "1.19", "1.19.3", etc. A full
// "1.19.3" stays an exact match, and operator/range keys (">=1.22", "^1.2")
// pass through untouched.
func normalizeConstraintKey(key string) string {
	k := strings.TrimSpace(key)
	if bareVersionRe.MatchString(k) && strings.Count(strings.TrimPrefix(k, "v"), ".") < 2 {
		return k + ".x"
	}
	return k
}

// Matches reports whether a detected version satisfies a configured key.
//
// Semver path: when the detected string has a numeric core that parses as a
// version and the (normalized) key parses as a constraint, the constraint is
// evaluated against the core. Only the numeric core is parsed — never the raw
// tag — so a Docker/CI tag like "1.21-alpine" is not misread as a semver
// prerelease (which would fail ">=1.21").
//
// Fallback path (either side not semver): case-insensitive exact match, then a
// minor-version prefix match on the extracted cores ("1.21" matches "1.21.0").
func Matches(key, detected string) bool {
	dCore := coreVersion(detected)
	if dCore != "" {
		if v, err := semver.NewVersion(dCore); err == nil {
			if c, err := semver.NewConstraint(normalizeConstraintKey(key)); err == nil {
				return c.Check(v)
			}
		}
	}
	dl := strings.TrimSpace(strings.ToLower(detected))
	kl := strings.TrimSpace(strings.ToLower(key))
	if dl == kl {
		return true
	}
	kCore := coreVersion(key)
	if kCore != "" && dCore != "" {
		return dCore == kCore || strings.HasPrefix(dCore, kCore+".")
	}
	return false
}

// SelectLowest implements "lowest detected version wins". It sorts the detected
// versions ascending (semver-aware; unparseable versions sort last), then for
// the lowest detected version returns the first key that matches it, trying
// keys in ascending sorted order for a deterministic result when a version
// satisfies several overlapping keys. It returns ("", false) when no detected
// version matches any key.
func SelectLowest(detected []string, keys []string) (string, bool) {
	if len(detected) == 0 || len(keys) == 0 {
		return "", false
	}
	sortedDetected := append([]string(nil), detected...)
	sort.SliceStable(sortedDetected, func(i, j int) bool {
		return lessVersion(sortedDetected[i], sortedDetected[j])
	})
	sortedKeys := append([]string(nil), keys...)
	sort.SliceStable(sortedKeys, func(i, j int) bool {
		return lessVersion(sortedKeys[i], sortedKeys[j])
	})
	for _, d := range sortedDetected {
		for _, k := range sortedKeys {
			if Matches(k, d) {
				return k, true
			}
		}
	}
	return "", false
}

// lessVersion orders two version-ish strings ascending: both parseable ->
// semver compare; a parseable core sorts before an unparseable one; otherwise
// lexical. Keys carrying operators (">=1.22") have no parseable core and sort
// after plain versions, which is fine — they are only a tie-break within a
// single detected version's match loop.
func lessVersion(a, b string) bool {
	av, aok := parseCore(a)
	bv, bok := parseCore(b)
	switch {
	case aok && bok:
		if c := av.Compare(bv); c != 0 {
			return c < 0
		}
		return a < b
	case aok:
		return true
	case bok:
		return false
	default:
		return a < b
	}
}

func parseCore(raw string) (*semver.Version, bool) {
	core := coreVersion(raw)
	if core == "" {
		return nil, false
	}
	v, err := semver.NewVersion(core)
	if err != nil {
		return nil, false
	}
	return v, true
}
