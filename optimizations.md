# Nickpit Review Runtime Optimizations

## V9 Runtime Profile

| Area | v9-high | v9-low | Notes |
|---|---:|---:|---|
| Total runtime | 17m43s | 18m20s | Both stable, no network stalls |
| Lanes | 13m19s | 13m58s | Dominant wall-clock cost |
| Verify cumulative model time | ~51m | ~51m | Main token/time sink, parallelized by `--concurrency=10` |
| Review + extract cumulative time | ~45m | ~51m | Nudge/reasoning extraction also heavy |
| Merge | <1m | ~1m40s | Low had validation retry friction |
| Finalize + summarize | 2m53s | 2m43s | Serial but not dominant |

## Highest-Impact Optimizations

2. Add per-role effort and timeout controls.

   v9-high had three verifier calls hit the 5m reasoning cap. Set verifier effort to `medium` or cap verify reasoning at 120-180s. Set extract/finalize/summarize to lower effort (`low` or `none`) where quality allows. This is lower risk than pipeline reordering and likely saves several minutes on high-effort runs.

3. Reduce reasoning extraction and nudge cost.

   Extract calls consumed ~23-25m cumulative in v9. Lower extract effort, cap extract at 60-90s, or reduce nudge count. This has quality tradeoffs, but it is a strong speed lever.

5. Test higher solo-run concurrency.

   v9 had no real 429s. Try `--concurrency=12` or `--concurrency=15` on solo runs to shorten verifier waves. Keep `10` as safer default for concurrent runs.

## Lower Priority

- Merge is no longer a meaningful speed bottleneck.
- Summarize is stable under 1m.
- Finalize is serial and costs ~2m, but changing it is less valuable than reducing verifier/review work.
