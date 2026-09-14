# Review budget calibration — 2026-09-14

Calibration used local saved runs and the production GitLab serve logs. No
prompts, patches, credentials, or individual findings are reproduced here.

## Sample and limitations

- Local inventory: 640 saved sessions; 74 real runs and 566 fixtures. Also scanned
  518 full logs, including 246 timing-bearing logs that overlap saved sessions;
  61 snippets were excluded. Overlapping records were not independent runs.
- Production: Loki's retained September 7–14 window, ending at 08:42:30 UTC.
  There were 47 starts: 44 successful runs, one cancellation, and two ongoing
  runs at the cutoff.
- Production was v0.1.2, primarily Qwen3.8-27B-NVFP4 at xhigh, with
  Qwen3.6-35B-A3B-FP8 at none for the small model. Local runs span other model
  settings, so their latency distributions are not interchangeable.
- Deadline-limited observations are censored: a timeout does not establish how
  much longer successful work would have needed. These are initial operational
  allocations, not a claim that all workloads now fit.

## Observations

Among 45 ended production runs, context entered urgent mode 26 times and hit
its deadline five times. There were 79 reviewer deadline stops, nine verification
phase deadlines, and ten dedupe phase deadlines; these are agent/phase events,
not distinct run counts.

The classifier's production p95 was 16 seconds against a 94.5-second allocation.
Verification reached its roughly 535-second allocation at p95. The post-review
pipeline's p95 was 191 seconds and maximum 268 seconds, while finalization's p95
was 63 seconds. Historical local finalization p95 was 136 seconds and body
summarization p95 was 47 seconds, arguing against caps based only on the faster
production small model.

Production reasoning mining had 1,164 successful observations (p95 2 seconds,
maximum 18); compilation had 326 (p95 4 seconds, maximum 16). Local reasoning
mining p95 was 10 seconds. The new context/verification reasoning summarizer has
no historical measurements of its own; its 25%-of-urgent-remainder, 30-second cap
is provisional and should be measured after rollout.

## Default changes

| Budget | Before | After |
| --- | --- | --- |
| Context | 300 s | 360 s |
| Each parallel review lane | 2,100 s | 2,400 s |
| Review / verify / dedupe lane shares | implicit 55 / 30 / 15% | unchanged |
| Classifier within verify | 15% | 5%, capped at 30 s |
| Verifier within verify | 85% | 95% |
| Post-review pipeline | 1,200 s | 900 s |
| Merge / finalize / verdict / summarize shares | 30 / 40 / 20 / 10% | 50 / 30 / 10 / 10% |

The new lane allocation is 1,320 seconds for review, 720 for verification
(684 for evidence verification), and 360 for dedupe. The pipeline allocations
are 450 / 270 / 90 / 90 seconds. Parent deadlines still apply, and overlapping
pipeline stages do not turn these shares into additive execution times.

The outer sequential bound is 6 + 40 + 15 = 61 minutes. Ordinary speed-up
thresholds remain at 80%. Review retains its *implicit* 55% remainder: writing
an explicit review weight would also enable its internal phase split.

After rollout, compare urgent-completion success, helper timeouts and token
cost, verification outcomes, and deadline counts by model and workload size.
