package dedupe

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

func finding(title, body, file string, start, end int) model.Finding {
	return model.Finding{
		Title: title,
		Body:  body,
		CodeLocation: model.CodeLocation{
			FilePath:  file,
			LineRange: model.LineRange{Start: start, End: end},
		},
	}
}

// Calibration corpus seeded from real runs against MR 1560
// (review-1560-duplicates-v2/v3.log). The duplicate pairs reached the final
// review output of v3 verbatim; the distinct pairs are findings v2 correctly
// kept separate. Threshold tuning must keep this table green.
func TestCompareCalibration(t *testing.T) {
	const script = "controllers/logrotate/logrotate.sh"
	const testFile = "controllers/logrotate_assets_integration_test.go"
	const controller = "controllers/projectgroup_controller_reconcile_logrotation_configmap.go"

	cases := []struct {
		name string
		a, b model.Finding
		want Verdict
	}{
		{
			name: "v3 dup: unscoped gz cleanup, reworded body",
			a: finding(
				"Unscoped *.gz cleanup overrides rotate 3650 retention policy",
				"Unscoped `*.gz` cleanup on line 70 of `logrotate.sh` deletes ALL compressed files older than 30 days under `log_root`. Directly conflicts with `rotate 3650` setting in generated config. Comment at lines 50-53 states intent NOT to delete rotated files automatically.",
				script, 70, 70,
			),
			b: finding(
				"Unscoped *.gz cleanup overrides rotate 3650 retention policy",
				"Config explicitly sets `rotate 3650` with comment stating 'We do not want to delete the files automatically'. Cleanup line deletes ALL .gz files after 30 days, overriding the 10-year retention policy.",
				script, 70, 70,
			),
			want: Duplicate,
		},
		{
			name: "v3 dup: strict mode flags, subset title",
			a: finding(
				"Bash script missing strict mode flags",
				"The script uses `#!/bin/sh` with `set -e` only. Pipeline failures are not detected.",
				script, 1, 5,
			),
			b: finding(
				"Bash script missing strict mode flags enabling silent failures",
				"Script only sets `set -e`; without `pipefail`, failures inside pipelines are silently ignored and the script continues.",
				script, 1, 3,
			),
			want: Duplicate,
		},
		{
			name: "v3 dup: container cleanup pattern, reworded title",
			a: finding(
				"Container cleanup pattern is redundant and scope-mismatched",
				"The container-specific cleanup on line 71 only matches two path levels while the exclusion applies at any depth; the general *.gz cleanup already covers it.",
				script, 71, 71,
			),
			b: finding(
				"Container cleanup pattern doesn't match exclusion scope and is redundant",
				"Cleanup for container logs matches only `container/*/*.gz` but the exclusion scope covers any depth. The line is also redundant with the unscoped *.gz cleanup.",
				script, 71, 71,
			),
			want: Duplicate,
		},
		{
			name: "v3 dup: configmap key removal, different file than script",
			a: finding(
				"ConfigMap structure change removes logrotate.conf key",
				"The logrotate.conf key disappears from the ConfigMap; consumers depending on it break without notice.",
				controller, 30, 45,
			),
			b: finding(
				"ConfigMap structure change removes logrotate.conf key without deprecation",
				"Removing the logrotate.conf key from the ConfigMap is a breaking API change for consumers; no deprecation or release note provided.",
				controller, 28, 40,
			),
			want: Duplicate,
		},
		{
			name: "v2 distinct: code issue vs test gap (different file)",
			a: finding(
				"Cronjobs cleanup pattern doesn't match exclusion scope",
				"Cleanup only removes `cronjobs/*.log` at one depth while the exclusion applies at any depth.",
				script, 69, 69,
			),
			b: finding(
				"Tests don't cover non-.log files under cronjobs directory",
				"No test fixture places non-.log files under cronjobs/, so the cleanup gap is never exercised.",
				testFile, 120, 140,
			),
			want: Distinct,
		},
		{
			name: "v2 distinct: adjacent cleanup lines, different defects",
			a: finding(
				"Unscoped *.gz cleanup overrides rotate 3650 retention policy",
				"Unscoped `*.gz` cleanup deletes ALL compressed files older than 30 days under `log_root`, conflicting with the 10-year retention configured via rotate 3650.",
				script, 70, 70,
			),
			b: finding(
				"Cronjobs cleanup pattern doesn't match exclusion scope",
				"Cleanup only removes `cronjobs/*.log` at one depth while the exclusion pattern applies to any depth, leaving nested logs behind.",
				script, 69, 69,
			),
			want: Distinct,
		},
		{
			name: "v2 distinct: missing validation vs missing test (different file)",
			a: finding(
				"Missing validation for log_root directory existence",
				"The script never checks that log_root exists before running find over it.",
				script, 10, 15,
			),
			b: finding(
				"No tests for empty or non-existent log_root directory",
				"There is no test covering script behavior when log_root is empty or missing.",
				testFile, 200, 220,
			),
			want: Distinct,
		},
		{
			name: "v7 cross-file: nested-subdirectory coverage gap in unit vs integration tests",
			a: finding(
				"Test coverage missing for nested subdirectories",
				"Test fixtures only create flat directories; nested paths under cronjobs/ and container/ are never exercised.",
				"controllers/logrotate_assets_test.go", 176, 182,
			),
			b: finding(
				"Test coverage missing for nested subdirectories under cronjobs and container",
				"Fixtures create only flat directories under cronjobs/ and container/; cleanup patterns for nested subdirectories are unverified.",
				testFile, 38, 60,
			),
			want: Possible,
		},
		{
			name: "v7 cross-file: flattening breaking change reported against script and test",
			a: finding(
				"Log directory flattening is breaking change without migration",
				"Patch flattens logs/project/*/... to logs/*/... without migration; existing paths break.",
				script, 69, 69,
			),
			b: finding(
				"Log directory flattening is a breaking change without migration",
				"Directory structure flattened without a migration strategy; old paths no longer match rotation patterns.",
				testFile, 38, 38,
			),
			want: Possible,
		},
		{
			name: "v8 cross-file: moderate strict-mode code and test gap",
			a: finding(
				"Bash script missing strict mode flags",
				"Script uses `#!/bin/sh` with `set -e` only; missing `-u` and `pipefail` means unset variables and pipeline failures pass silently.",
				script, 1, 3,
			),
			b: finding(
				"Bash strict mode flags not enabled per style guide",
				"Tests cover SIGTERM but not unset variables or pipeline failures caused by missing strict mode flags.",
				"controllers/logrotate_assets_test.go", 100, 120,
			),
			want: Possible,
		},
		{
			name: "cross-file moderate title alone stays distinct",
			a: finding(
				"Bash script missing strict mode flags",
				"Script starts with `set -e`.",
				script, 1, 3,
			),
			b: finding(
				"Bash strict mode flags not enabled per style guide",
				"Fixture setup writes temporary files before invoking the wrapper.",
				"controllers/logrotate_assets_test.go", 100, 120,
			),
			want: Distinct,
		},
		{
			name: "cross-file moderate body alone stays distinct",
			a: finding(
				"Runtime cleanup skips nested cronjobs",
				"Cleanup patterns use `cronjobs/*.log` while exclusion covers nested cronjobs paths, leaving nested logs unremoved.",
				script, 69, 69,
			),
			b: finding(
				"ConfigMap key removal breaks consumers",
				"Cleanup pattern scope and exclusion scope differ; nested cronjobs paths can accumulate because only flat files are removed.",
				controller, 32, 38,
			),
			want: Distinct,
		},
		{
			name: "possible: same line, related but different aspect",
			a: finding(
				"Cronjobs cleanup pattern doesn't match exclusion scope",
				"Cleanup only removes `cronjobs/*.log` at one depth while the exclusion applies recursively at any depth below cronjobs.",
				script, 69, 69,
			),
			b: finding(
				"Non-.log files under cronjobs not cleaned up",
				"The cronjobs cleanup matches `*.log` only; other file types below cronjobs accumulate forever and are never cleaned up.",
				script, 69, 69,
			),
			want: Possible,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(tc.a, tc.b)
			if got.Verdict != tc.want {
				t.Fatalf("Compare() verdict = %s, want %s (title=%.2f body=%.2f loc=%.2f reason=%q)",
					got.Verdict, tc.want, got.TitleSim, got.BodySim, got.LocationSim, got.Reason)
			}
			reversed := Compare(tc.b, tc.a)
			if reversed.Verdict != got.Verdict {
				t.Fatalf("Compare() not symmetric: %s vs %s", got.Verdict, reversed.Verdict)
			}
		})
	}
}

// Cross-file pairs cap at Possible: identical text in two files must still be
// judged by the merge agent, never folded mechanically (mechanical merging
// assumes a single file when extending line ranges).
func TestCompareCrossFileCapsAtPossible(t *testing.T) {
	a := finding("Identical finding title", "identical body text", "a.go", 10, 12)
	b := finding("Identical finding title", "identical body text", "b.go", 10, 12)
	got := Compare(a, b)
	if got.Verdict != Possible {
		t.Fatalf("verdict = %s, want possible (cross-file ceiling)", got.Verdict)
	}
	if got.LocationSim != 0 {
		t.Fatalf("LocationSim = %.2f, want 0 across files", got.LocationSim)
	}
}

func TestCompareCrossFileRootCauseTier(t *testing.T) {
	cases := []struct {
		name       string
		a, b       model.Finding
		want       Verdict
		wantReason string
	}{
		{
			name: "same root cause across related templates routes to LLM",
			a: finding(
				"Unquoted path expansion breaks target-dir restore",
				"The target-dir template leaves `$TMP` unquoted in `find`; `mkdir` and `chown` also receive unquoted paths. Paths containing spaces split into separate arguments and the restore fails.",
				"pkg/restore/restore-directory-target-dir.tpl", 10, 18,
			),
			b: finding(
				"Directory template mishandles paths containing spaces",
				"The directory template passes `$TMP` to `find` without quotes, then calls `mkdir` and `chown` with unquoted destination paths. Whitespace in a path causes word splitting and command failure.",
				"pkg/restore/restore-directory.tpl", 12, 20,
			),
			want:       Possible,
			wantReason: "same root-cause signals across related files",
		},
		{
			name: "same test gap across unit and integration tests routes to LLM",
			a: finding(
				"Tests miss nested policy inheritance",
				"Unit tests only cover flat fixtures, so missing `PolicyID` propagation through `resolvePolicy` for nested objects is not exercised.",
				"pkg/policy/policy_test.go", 40, 80,
			),
			b: finding(
				"Integration coverage omits nested policy propagation",
				"Integration tests never build nested objects, leaving the missing `PolicyID` propagation through `resolvePolicy` untested.",
				"pkg/policy/policy_integration_test.go", 20, 65,
			),
			want:       Possible,
			wantReason: "same root-cause signals across related files",
		},
		{
			name: "same directory but different causes stays distinct",
			a: finding(
				"Unquoted path expansion breaks target-dir restore",
				"The target-dir template leaves `$TMP` unquoted in `find`; `mkdir` and `chown` also receive unquoted paths. Paths containing spaces split into separate arguments and the restore fails.",
				"pkg/restore/restore-directory-target-dir.tpl", 10, 18,
			),
			b: finding(
				"Dot directory target escapes validation",
				"`lastPathElement` returns `.` for a dot-only input, so `validateTarget` accepts the current directory as a restore target.",
				"pkg/restore/restore-directory.tpl", 42, 50,
			),
			want: Distinct,
		},
		{
			name: "shared generic words without anchors stays distinct",
			a: finding(
				"Template path handling issue",
				"The template path handling is inconsistent and may surprise callers in unusual cases.",
				"pkg/render/item.tpl", 10, 12,
			),
			b: finding(
				"Rendering output needs cleanup",
				"The rendering output is hard to follow and could be improved for maintainability.",
				"pkg/render/item_alt.tpl", 14, 16,
			),
			want: Distinct,
		},
		{
			name: "security scope and cleanup scope stay distinct",
			a: finding(
				"Template command injection through untrusted arguments",
				"`CommandArgs` are concatenated into `renderCommand` without escaping, allowing shell metacharacters to execute unintended commands.",
				"pkg/render/command.tpl", 20, 28,
			),
			b: finding(
				"Template cleanup leaves temporary arguments behind",
				"`CommandArgs` temporary files are not removed after `renderCommand` succeeds, leaving stale cleanup state.",
				"pkg/render/command_cleanup.tpl", 30, 36,
			),
			want: Distinct,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(tc.a, tc.b)
			if got.Verdict != tc.want {
				t.Fatalf("Compare() verdict = %s, want %s (title=%.2f body=%.2f root=%.2f loc=%.2f reason=%q)",
					got.Verdict, tc.want, got.TitleSim, got.BodySim, got.RootCauseSim, got.LocationSim, got.Reason)
			}
			if tc.wantReason != "" && got.Reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q (title=%.2f body=%.2f root=%.2f)",
					got.Reason, tc.wantReason, got.TitleSim, got.BodySim, got.RootCauseSim)
			}
		})
	}
}

func TestIsTestLikeFileCommonPatterns(t *testing.T) {
	cases := []struct {
		file string
		want bool
	}{
		{file: "pkg/policy/policy_test.go", want: true},
		{file: "pkg/policy/policy.test.ts", want: true},
		{file: "pkg/policy/test_policy.py", want: true},
		{file: "pkg/policy/test-policy.rb", want: true},
		{file: "pkg/policy/policy_spec.rb", want: true},
		{file: "pkg/policy/policy.spec.ts", want: true},
		{file: "pkg/__tests__/policy.js", want: true},
		{file: "pkg/Spec/policy.rb", want: true},
		{file: "pkg/specs/policy.rb", want: true},
		{file: "pkg/policy/contest.go", want: false},
		{file: "pkg/policy/latest_specimen.rb", want: false},
		{file: "pkg/policy/policy.go", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			if got := isTestLikeFile(tc.file); got != tc.want {
				t.Fatalf("isTestLikeFile(%q) = %v, want %v", tc.file, got, tc.want)
			}
		})
	}
}

func TestCompareIdentical(t *testing.T) {
	f := finding("Title", "Body", "a.go", 1, 2)
	if got := Compare(f, f); got.Verdict != Identical {
		t.Fatalf("verdict = %s, want identical", got.Verdict)
	}
}

func TestCompareUnknownRangeNeutral(t *testing.T) {
	a := finding("Bash script missing strict mode flags", "set -e only", "a.sh", 0, 0)
	b := finding("Bash script missing strict mode flags enabling silent failures", "set -e only, pipefail missing", "a.sh", 1, 3)
	got := Compare(a, b)
	if got.LocationSim != LocSameRegion {
		t.Fatalf("LocationSim = %.2f, want neutral %.2f", got.LocationSim, LocSameRegion)
	}
	if got.Verdict != Duplicate {
		t.Fatalf("verdict = %s, want duplicate (near-identical title in neutral region)", got.Verdict)
	}
}

// Non-ASCII letters are token characters, not delimiters; without Unicode
// support, accented and non-Latin findings degrade to near-zero similarity.
func TestCompareUnicodeTitles(t *testing.T) {
	a := finding(
		"Größenprüfung für Übergabeparameter fehlt",
		"Die Funktion prüft die Größe der Übergabeparameter nicht vor dem Zugriff.",
		"prüfung.go", 10, 12,
	)
	b := finding(
		"Größenprüfung für Übergabeparameter fehlt komplett",
		"Vor dem Zugriff wird die Größe der Übergabeparameter nicht geprüft.",
		"prüfung.go", 10, 14,
	)
	got := Compare(a, b)
	if got.Verdict < Duplicate {
		t.Fatalf("verdict = %s (title=%.2f body=%.2f loc=%.2f), want >= duplicate for near-identical German findings",
			got.Verdict, got.TitleSim, got.BodySim, got.LocationSim)
	}
	if got.TitleSim < TitleStrong {
		t.Fatalf("TitleSim = %.2f, want >= %.2f — umlauts must not split tokens", got.TitleSim, TitleStrong)
	}
}

func TestFindBest(t *testing.T) {
	target := finding("Bash script missing strict mode flags", "set -e only", "a.sh", 1, 3)
	pool := []model.Finding{
		finding("Unrelated finding about other things entirely", "different defect text", "a.sh", 200, 210),
		finding("Bash script missing strict mode flags enabling silent failures", "set -e without pipefail", "a.sh", 1, 5),
	}
	idx, m := FindBest(target, pool, Duplicate)
	if idx != 1 || m.Verdict < Duplicate {
		t.Fatalf("FindBest = (%d, %s), want (1, >=duplicate)", idx, m.Verdict)
	}
	if idx, _ := FindBest(target, pool[:1], Duplicate); idx != -1 {
		t.Fatalf("FindBest on unrelated pool = %d, want -1", idx)
	}
}

func TestClusters(t *testing.T) {
	findings := []model.Finding{
		finding("Bash script missing strict mode flags", "set -e only", "a.sh", 1, 3),
		finding("Completely different topic in another file", "other body", "b.go", 10, 20),
		finding("Bash script missing strict mode flags enabling silent failures", "set -e without pipefail", "a.sh", 1, 5),
	}
	clusters := Clusters(findings, Duplicate)
	if len(clusters) != 2 {
		t.Fatalf("clusters = %v, want 2 groups", clusters)
	}
	if len(clusters[0]) != 2 || clusters[0][0] != 0 || clusters[0][1] != 2 {
		t.Fatalf("first cluster = %v, want [0 2]", clusters[0])
	}
	if len(clusters[1]) != 1 || clusters[1][0] != 1 {
		t.Fatalf("second cluster = %v, want [1]", clusters[1])
	}
}
