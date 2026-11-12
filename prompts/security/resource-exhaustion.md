### Resource exhaustion and availability security guide
Applies to: untrusted input driving CPU, memory, disk, connections, workers, paid API calls, queues, or crashing shared process.

#### Hunt for
- **Superlinear work**: regex backtracking, recursive parsing, template expansion, graph traversal, adversarial hashing; derive accepted depth and cardinality.
- **Expansion**: archives, images, YAML anchors, XML entities, compression exceed checked size; limit after every stage.
- **Fan-out**: broad scans, unbounded sort/aggregation, no pagination cap, many downstream calls; not when authorization intentionally permits that scope.
- **Unbounded growth**: bodies (`io.ReadAll`), uploads, buffers, sessions, cache keys, metric labels, subscriptions, jobs lack per-item and aggregate caps or cleanup on disconnect, timeout, partial parse.
- **Leaks**: error/cancel paths skip closing sockets, files, response bodies, DB rows, timers, subprocesses, temp files; goroutines blocked; shared pool drains.
- **Detached work**: context/deadline not propagated; DB, model, network calls outlive disconnect or failed authorization.
- **Pre-auth cost**: parsing, decompression, crypto, outbound requests before auth and size/rate gate.
- **Quota scope**: keyed on attacker-influenceable IP, route, key prefix, task ID; escape budget or spend another's; overflow, races, reconnects.
- **Starvation**: attacker jobs hold shared locks, workers, DB connections, event loop past deadline or skip fairness.
- **Crash/deadlock**: input reaches panic (nil dereference, index out of range, unchecked type assertion, unrecovered goroutine), `log.Fatal`/`os.Exit`, lock cycle, infinite loop.
- **Retry storms**: unbounded or synchronized retries; no backoff, jitter, ceiling, circuit breaker, dedup; one bounded failure source must create persistent aggregate work.
- **Poison records**: malformed record fails repeatedly at head of shared queue, startup scan, migration; no quarantine; other tenants share blocked unit.
- **Unsafe recovery**: restart/restore rebuilds unbounded state, ignores quotas, replays failing input.

#### Validate before reporting
- Need input-to-cost path, missing bound, impact on another user, shared service, or operator-paid spend.
- Missing rate limit alone not enough; check size caps, concurrency limits, deadlines, tenant quotas on path.
- Many small items pass per-message cap yet exhaust aggregate; compare both.
- One input with superlinear or persistent shared effect outranks volume flooding.
- Leaks: show where cleanup belongs, why skipped. Crash confined by recover/supervisor: lower impact, still defect.
