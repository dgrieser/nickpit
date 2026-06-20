# TODO

All 9 completed successfully. Runtime outliers are `20-05-56` and `21-09-15`; both are single-stage stalls, not general validation failures.

| log | runtime | findings | 429s | JSON retries | slowest logged stage | delay read |
|---|---:|---:|---:|---:|---|---|
| `18-56-12` | 14m34s | 13 | 0 | 6 | Architecture review `4m38s` | Normal model latency plus retries; no rate-limit issue. |
| `19-10-46` | 13m37s | 11 | 3 | 5 | Best Practices verify `2m52s` | Mild 429 waits, otherwise no major outlier. |
| `19-24-24` | 19m25s | 11 | 7 | 8 | Finalize `6m55s` | Slower finalizer plus 429s; includes the dedupe ID typo recovery. |
| `19-43-49` | 12m52s | 9 | 0 | 2 | Code Quality review `4m26s` | No rate limits; delay is just slower review/nudge calls. |
| `19-56-42` | 9m13s | 10 | 3 | 3 | Code Quality review `2m48s` | Cleanest run; 429s overlapped enough not to hurt much. |
| `20-05-56` | 41m29s | 10 | 13 | 8 | Finalize `30m27s` | Dominated by finalizer stall; also some 429s and a `7m17s` Best Practices call. |
| `20-47-26` | 10m05s | 10 | 14 | 4 | Testing review `3m30s` | Many 429s, but parallelism hid most of the cost. No single bad stall. |
| `20-57-31` | 11m44s | 10 | 15 | 3 | Security verify `5m17s` | 429-heavy plus a few 4-5m review/verify calls. |
| `21-09-15` | 39m57s | 12 | 0 | 2 | Performance review `31m47s` | Dominated by one Performance reviewer stall; dedupe also logs `31m22s`, likely downstream/wall-clock coupled to that slow path. |

Main takeaways:

- Validation/ID fixes are not causing runtime pain here. JSON retries are small compared with the big stalls.
- 429 count is not enough to predict runtime: `20-47` and `20-57` had the most 429s but stayed near 10-12m because waits overlapped.
- The real runtime risk is a single long model call in late aggregation/review stages: `finalize` in `20-05`, `review: Performance` in `21-09`.

Next best runtime fix: add per-call soft timeout/abort or retry-on-stall for finalizer and reviewer calls that exceed a threshold like 8-10 minutes without needing 429/error evidence.


- Add feature to give additional instructions to agents
- Add max turns for agents; unlimited by default
- Add suggestion agent to format suggestions for gitlab/github
- Remove unnecessary arguments/config.

- Add man pages for commands used/called
