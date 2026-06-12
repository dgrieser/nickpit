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
