### PostgreSQL 18: The Complete Production SQL Review Guide

> **Version**: PostgreSQL 18.0 was released on **September 25, 2025**. The latest minor release is **18.6 (August 13, 2026)**; 18.5 was never shipped because of a regression, so 18.4 is followed directly by 18.6. The next scheduled minor release is November 12, 2026.[^1][^2][^3][^5]
> **Support**: PostgreSQL 18 is supported until **November 14, 2030** (five years of minor releases). As of October 2026 the supported majors are 18, 17, 16, 15 and 14; 14 receives its final release on November 12, 2026. PostgreSQL 19 is **not released**: 19 RC 1 shipped on October 15, 2026 and GA is planned for later this month.[^2][^4][^5]
> **Scope**: SQL that runs on a PostgreSQL 18 server: schema migrations, queries embedded in application code, functions/procedures/triggers, schema design, indexing, transactions, security, JSONB, partitioning, connection handling and upgrades. Every SQL example was executed on PostgreSQL 18.6; "new in 18" boundaries were checked against 17.11, and 19-only features were checked to fail on 18.6 and work on 19rc1.
> **Note**: This document is self-contained. Everything usable from PostgreSQL 14–17 is carried forward and marked with the version that introduced it. Sections marked **New in 18** cover behaviour specific to this release.

***

#### Table of Contents

1. Version-aware review
2. What's new in PostgreSQL 18 (overview)
3. New in 18: asynchronous I/O
4. New in 18: `uuidv7()` and UUID functions
5. New in 18: virtual generated columns (now the default)
6. New in 18: `OLD`/`NEW` in `RETURNING`
7. New in 18: temporal constraints (`WITHOUT OVERLAPS`, `PERIOD`)
8. New in 18: B-tree skip scan and other planner improvements
9. New in 18: `NOT NULL` constraints in `pg_constraint`, `NOT VALID`, `NOT ENFORCED`
10. New in 18: EXPLAIN, COPY, functions, collations
11. New in 18: authentication, security and operations changes
12. Features from 14–17 that a PostgreSQL 18 codebase can use
13. Schema migrations and locking safety
14. Data types and their pitfalls
15. Constraints and keys
16. Indexing
17. Query writing and performance
18. Reading EXPLAIN
19. Transactions, isolation and concurrency
20. JSONB and SQL/JSON
21. Partitioning
22. Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL
23. Functions, procedures and triggers
24. Application code: parameters, pooling, timeouts
25. Maintenance and observability
26. Removed / changed in 18 — migration notes
27. Production / review checklist

***

#### Version-aware review

The project pins PostgreSQL 18. Judge code against what 18.x actually does:

- **Verify before you claim something is unavailable.** Many "new" features are older than reviewers remember: `MERGE` is 15, `MERGE ... RETURNING` and `JSON_TABLE` are 17, `gen_random_uuid()` is core since 13, `CREATE INDEX CONCURRENTLY` and `REINDEX CONCURRENTLY` (12) are long available. Check the version table in "Features from 14–17" before asking for a workaround.
- **Say "not available in PostgreSQL 18" only when 18 proves it.** The "Do not use or recommend" list below is verified: each item fails on 18.6.
- **Minor releases matter.** 18.x minors fix security issues (18.6 alone fixes 28 CVEs) and wrong-result bugs; some require post-upgrade actions (`ANALYZE` on tables with parallel-built GIN indexes, `REINDEX` of certain `btree_gist`/`ltree` indexes in 18.6).[^3] A deployment pinned to `postgres:18.0` or any minor older than 18.6 is a finding.
- **Client and pooler versions are part of the contract.** Older `psql` clients have `\copy` problems with 18 servers' CSV `\.` handling; PgBouncer before 1.21 cannot use protocol-level prepared statements in transaction mode.[^1][^30]

##### Do not use or recommend (PostgreSQL 19 and later, or never released)

PostgreSQL 19 is at RC 1. Everything below fails on 18.6 (verified) — never suggest it for an 18 codebase:[^4]

| Feature | Introduced in | Error on 18.6 |
| --- | --- | --- |
| `REPACK` / `REPACK (CONCURRENTLY)` (replacement for `VACUUM FULL`/`CLUSTER`) | 19 | syntax error |
| `INSERT ... ON CONFLICT ... DO SELECT ... RETURNING` | 19 | syntax error |
| `IGNORE NULLS` / `RESPECT NULLS` for `lag`/`lead`/`first_value`/`last_value`/`nth_value` | 19 | syntax error |
| `WAIT FOR LSN ...` (read-your-writes on standbys) | 19 | syntax error |
| `COPY ... TO ... (FORMAT json)`, `COPY FROM ... (ON_ERROR set_null)`, `COPY TO` directly from a partitioned table | 19 | syntax error / unsupported |
| `random(date, date)`, `random(timestamptz, timestamptz)` | 19 | function does not exist |
| `error_on_null(anyelement)` | 19 | function does not exist |
| `encode(..., 'base64url')`, `'base32hex'` | 19 | unrecognized encoding |
| `oid8` type, `tid_block()`, `tid_offset()` | 19 | does not exist |
| casts between `uuid` and `bytea` | 19 | cannot cast |
| `range_minus_multi()`, `multirange_minus_multi()` | 19 | function does not exist |
| jsonpath string methods `.lower()`, `.upper()`, `.ltrim()`, `.rtrim()`, `.btrim()`, `.initcap()`, `.replace()`, `.split_part()` | 19 | jsonpath syntax error |
| `ALTER TABLE ... ALTER CONSTRAINT ... [NOT] ENFORCED` on a `CHECK` constraint (18 allows it only for foreign keys) | 19 | cannot alter enforceability |
| `EXPLAIN (ANALYZE, IO)` | 19 | unrecognized EXPLAIN option |
| `pg_plan_advice` / `pg_stash_advice` extensions, `pg_clear_extended_stats()`, `pg_restore_extended_stats()` | 19 | not present |
| `ALTER TABLE ... MERGE PARTITIONS` / `SPLIT PARTITION`, `UPDATE/DELETE ... FOR PORTION OF`, `GRAPH_TABLE` (SQL/PGQ) | not in any release (also fail on 19rc1) | syntax error |

Also do not assume 19 behaviour changes: in 18, `json_array(SELECT ... )` over zero rows returns `NULL` (19 returns `[]`), `standard_conforming_strings` can still be set to `off` (19 forces `on`), JIT is **on** by default, `max_locks_per_transaction` defaults to 64, `default_toast_compression` defaults to `pglz` and `log_lock_waits` defaults to `off` (all verified on 18.6; each changes in 19).[^4]

##### Deprecated in 18 (still works, flag new uses)

- **MD5 passwords.** `CREATE ROLE`/`ALTER ROLE` with an MD5-hashed password emits `WARNING: setting an MD5-encrypted password` (controlled by `md5_password_warnings`); support will be removed in a future major release. Use SCRAM (`password_encryption = scram-sha-256`, the default since 14).[^1][^17]
- **`ssl_ecdh_curve`** is renamed `ssl_groups` (old name still accepted).[^1]
- **`pg_recvlogical --two-phase`** is renamed `--enable-two-phase`.[^1]

***

#### What's new in PostgreSQL 18 (overview)

The release-note highlights, plus the incompatibilities a reviewer must know.[^1][^6]

| Area | Change | Section |
| --- | --- | --- |
| I/O | Asynchronous I/O subsystem (`io_method` = `worker` default, `io_uring`, `sync`); `effective_io_concurrency` and `maintenance_io_concurrency` default to 16 | Asynchronous I/O |
| UUIDs | `uuidv7()` (timestamp-ordered) and `uuidv4()` alias of `gen_random_uuid()` | uuidv7 |
| DDL | Generated columns are **virtual by default**; `STORED` must be explicit | Virtual generated columns |
| DML | `RETURNING old.*, new.*` in `INSERT`, `UPDATE`, `DELETE`, `MERGE` | OLD/NEW |
| Constraints | `PRIMARY KEY`/`UNIQUE ... WITHOUT OVERLAPS`, `FOREIGN KEY (..., PERIOD col)` | Temporal constraints |
| Constraints | `NOT NULL` constraints stored in `pg_constraint`, nameable, addable `NOT VALID`; `NOT ENFORCED` for `CHECK` and foreign keys; `NOT VALID` FKs on partitioned tables | NOT NULL / NOT ENFORCED |
| Planner | B-tree **skip scan**; `OR` lists and `IN (VALUES ...)` turned into `= ANY(array)`; self-join elimination; `DISTINCT` key reordering; Right Semi Join | Skip scan |
| EXPLAIN | `EXPLAIN ANALYZE` includes `BUFFERS` by default; `Index Searches:` per index node; fractional actual row counts; `Disabled: true` nodes | EXPLAIN |
| COPY | CSV `COPY FROM` no longer treats `\.` as end-of-data; `REJECT_LIMIT`; `LOG_VERBOSITY silent` | COPY |
| Functions | `casefold()`, `array_sort()`, `array_reverse()`, `reverse(bytea)`, `crc32()`, `crc32c()`, `gamma()`, `lgamma()`, integer↔`bytea` casts, `EXTRACT(WEEK FROM interval)`, `MIN`/`MAX` on arrays and composites, `to_number(..., 'RN')`, `json[b]_strip_nulls(..., strip_in_arrays)`; `jsonb` `null` casts to SQL `NULL` | Functions |
| Collation | `PG_UNICODE_FAST` builtin collation; `LIKE` and position functions work with nondeterministic collations | Collations |
| Auth | OAuth 2.0 (`oauth` in `pg_hba.conf`, `oauth_validator_libraries`); MD5 passwords deprecated | Security |
| Triggers | `AFTER` triggers run as the role that queued the event, not the role active at commit | Security |
| Operations | `initdb` enables data checksums by default; `pg_upgrade` keeps planner statistics and gains `--swap`; `VACUUM`/`ANALYZE` recurse into inheritance children (use `ONLY` to opt out); unlogged partitioned tables rejected | Removed / changed |
| Monitoring | `pg_aios`; `pg_stat_io` byte columns (`op_bytes` removed); `pg_stat_wal` loses `wal_write`/`wal_sync`/timing columns; vacuum/analyze time columns in `pg_stat_all_tables`; `log_lock_failures`; `log_connections` takes a list | Observability |
| Protocol | Wire protocol 3.2 (256-bit cancel keys); libpq `min_protocol_version`/`max_protocol_version` | Application code |

***

#### New in 18: Asynchronous I/O

PostgreSQL 18 adds an asynchronous I/O (AIO) subsystem. Backends can queue multiple reads ahead of need, which speeds up sequential scans, bitmap heap scans and vacuum, especially on cloud block storage with high latency.[^1][^13]

```sql
SHOW io_method;                   -- worker   (default; others: sync, io_uring)
SHOW io_workers;                  -- 3
SHOW io_combine_limit;            -- 128kB
SHOW effective_io_concurrency;    -- 16 (was 1 in 17)
SHOW maintenance_io_concurrency;  -- 16 (was 10 in 17)
SELECT count(*) FROM pg_aios;     -- in-flight AIO handles (new view)
```

Review notes:

- `io_method` requires a server restart. `io_uring` requires a build with `--with-liburing` (Linux only); `worker` works everywhere. `sync` restores 17-style synchronous I/O and is the fallback when diagnosing AIO problems.
- The higher `effective_io_concurrency` default changes plans (bitmap heap scan prefetch) and I/O load. Configuration files copied from 17 that set `effective_io_concurrency = 1` silently disable most of the benefit.
- AIO is a server feature; SQL does not change. Do not attribute query-level regressions to AIO without `EXPLAIN (ANALYZE)` evidence. In 18 there is no `EXPLAIN (IO)` option — that is 19.
- In 18, AIO is used for reads: the release notes name sequential scans, bitmap heap scans and vacuum. Write-heavy workloads should not expect gains from it.

***

#### New in 18: `uuidv7()` and UUID functions

```sql
SELECT uuidv7();                                   -- 0199...-7xxx-...: version 7, time-ordered
SELECT uuidv4();                                   -- same as gen_random_uuid()
SELECT uuid_extract_version(uuidv7());             -- 7   (function is 17+)
SELECT uuid_extract_timestamp(uuidv7());           -- creation time (17+, v1/v7 only)
SELECT uuidv7(interval '-1 hour');                 -- shift the embedded timestamp
```

`uuidv7()` puts a 48-bit Unix millisecond timestamp plus a sub-millisecond fraction in the high bits, followed by random bits.[^7] Consequences:

- **B-tree locality.** New keys go to the right edge of the index like a `bigint` identity instead of scattering across all leaf pages like v4. For high-insert tables with UUID primary keys, `uuidv7()` is the right default in 18.
- **Ordering.** In testing, 100,000 values generated in one statement were strictly increasing. The documentation promises only "time-ordered"; do not rely on strict ordering across sessions or servers, and never use a UUID as a substitute for a `created_at` column in business logic.
- **Information leak.** The creation time is readable with `uuid_extract_timestamp()`. Do not expose v7 IDs where creation time is sensitive; use `gen_random_uuid()` there.
- **Storage.** Store UUIDs as `uuid` (16 bytes), never as `text`/`varchar` (`pg_column_size(uuidv7()::text)` is 40 bytes) — the text form also sorts and compares more slowly.

```sql
-- GOOD (18): time-ordered UUID key generated by the server
CREATE TABLE orders (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- BAD: textual UUIDs, random v4 for a hot insert path
CREATE TABLE orders_bad (id varchar(36) PRIMARY KEY DEFAULT gen_random_uuid()::text);
```

On 17 and older `uuidv7()` and `uuidv4()` do not exist (verified: `function uuidv7() does not exist`). Code that must also run on 17 needs an application-side v7 generator.

***

#### New in 18: Virtual generated columns (now the default)

In 18, `GENERATED ALWAYS AS (expr)` without a keyword creates a **virtual** column: computed on read, no storage. `STORED` (the only kind in 12–17) must now be spelled out.[^8][^1]

```sql
CREATE TABLE g  (a int, b int GENERATED ALWAYS AS (a * 2));           -- virtual in 18
CREATE TABLE gs (a int, b int GENERATED ALWAYS AS (a * 2) STORED);    -- stored
SELECT attgenerated FROM pg_attribute WHERE attrelid = 'g'::regclass  AND attname = 'b';  -- v
SELECT attgenerated FROM pg_attribute WHERE attrelid = 'gs'::regclass AND attname = 'b';  -- s
```

On 17, the first statement is a syntax error, so migrations written for 18 without `STORED` do not run on 17 — and DDL copied from a 17 project that omitted nothing still gets `STORED` behaviour only if it said `STORED` (it had to). The real review risk is the opposite direction: a new 18 migration that omits the keyword gets a virtual column, which **cannot be indexed** and has other restrictions.

Verified restrictions of virtual generated columns on 18.6:

| Operation | Virtual | Stored |
| --- | --- | --- |
| Expression uses a user-defined function or type | **error**: "Virtual generated columns that make use of user-defined functions are not yet supported" | allowed (function must be immutable) |
| `CREATE INDEX` on the column | **error**: "indexes on virtual generated columns are not supported" | allowed |
| `UNIQUE` / `PRIMARY KEY` | **error** | allowed |
| `FOREIGN KEY` on the column | **error**: "foreign key constraints on virtual generated columns are not supported" | allowed |
| `CHECK` and `NOT NULL` constraints | allowed | allowed |
| `ALTER TABLE ... ADD COLUMN` | no table rewrite | **rewrites the table** under `ACCESS EXCLUSIVE` |
| `ALTER COLUMN ... SET EXPRESSION` (17+) | no rewrite | rewrites the table |
| `ALTER COLUMN ... DROP EXPRESSION` | **error** (not supported for virtual) | allowed |
| Logical replication (`publish_generated_columns`) | not supported | supported |
| Part of a partition key | not allowed | not allowed |

Other rules (from the documentation): the expression must be immutable, may not reference another generated column or system columns other than `tableoid`, and cannot be overridden on `INSERT`/`UPDATE` (only `DEFAULT` is accepted). Generated values are not visible in `BEFORE` triggers. Column privileges are separate from the base columns', but for virtual columns this is only secure if the expression uses leakproof functions.[^8]

```sql
-- BAD (18): intended to be indexed, but the omitted keyword made it virtual
ALTER TABLE users ADD COLUMN email_norm text GENERATED ALWAYS AS (lower(email));
CREATE INDEX ON users (email_norm);   -- ERROR: indexes on virtual generated columns are not supported

-- GOOD: stored column (table rewrite: plan it), or index the expression directly
CREATE INDEX users_email_lower ON users (lower(email));   -- no rewrite, usable by WHERE lower(email) = ...
```

Choose virtual for cheap derived values that are read rarely or only projected; choose stored (or an expression index) when the value is filtered, joined, indexed or replicated. Adding a virtual column is a metadata-only change — the cheapest way to add a derived column to a large table.

***

#### New in 18: `OLD` and `NEW` in `RETURNING`

`INSERT`, `UPDATE`, `DELETE` and `MERGE` can return the row before and after the change.[^9][^1]

```sql
CREATE TABLE acct (id int PRIMARY KEY, bal numeric NOT NULL);
INSERT INTO acct VALUES (1, 100);

UPDATE acct SET bal = bal - 30 WHERE id = 1
RETURNING old.bal AS before, new.bal AS after;              -- 100 | 70

-- Upsert: tell inserts from updates without the xmax trick
INSERT INTO acct VALUES (1, 5)
ON CONFLICT (id) DO UPDATE SET bal = excluded.bal
RETURNING old.bal, new.bal, (old.id IS NULL) AS inserted;  -- 70 | 5 | f

DELETE FROM acct WHERE id = 1 RETURNING old.bal, new.bal;  -- 5 | NULL

-- Rename the aliases when "old"/"new" clash with table names (e.g. in trigger functions)
UPDATE acct SET bal = 1 RETURNING WITH (OLD AS o, NEW AS n) o.bal, n.bal;
```

- For `INSERT`, `old.*` is all `NULL`; for `DELETE`, `new.*` is all `NULL`. Unqualified columns keep the previous meaning (new values for `INSERT`/`UPDATE`, old for `DELETE`).
- Replace `RETURNING (xmax = 0) AS inserted` hacks with `old.<pk> IS NULL` — `xmax` semantics are an implementation detail.
- Test against a non-nullable column (`old.id IS NULL`), not `old IS NULL`, which is true only if *every* column of the old row is null.
- On 17 this is `missing FROM-clause entry for table "old"` (verified).

***

#### New in 18: Temporal constraints (`WITHOUT OVERLAPS`, `PERIOD`)

A primary key or unique constraint can make its last column a range (or multirange) that must not overlap for equal values of the other columns; a foreign key can reference such a key with `PERIOD`, requiring the referenced ranges to cover the referencing range.[^1][^10]

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;   -- needed for the scalar (=) part of the GiST index

CREATE TABLE room (
    id    int,
    valid tstzrange,
    PRIMARY KEY (id, valid WITHOUT OVERLAPS)
);
INSERT INTO room VALUES (1, '[2025-01-01,2025-02-01)');
INSERT INTO room VALUES (1, '[2025-01-15,2025-03-01)');
-- ERROR:  conflicting key value violates exclusion constraint "room_pkey"

CREATE TABLE booking (
    bid     int,
    room_id int,
    during  tstzrange,
    FOREIGN KEY (room_id, PERIOD during) REFERENCES room (id, PERIOD valid)
);
INSERT INTO booking VALUES (1, 1, '[2025-01-05,2025-01-10)');   -- ok: covered
INSERT INTO booking VALUES (2, 1, '[2025-01-25,2025-02-10)');   -- ERROR: not covered
```

- The constraint is implemented as a GiST exclusion constraint; the error message says "exclusion constraint". Without `btree_gist` the `int` part has no GiST operator class and creation fails.
- It replaces hand-written `EXCLUDE USING gist (id WITH =, valid WITH &&)` plus triggers for temporal FKs.
- 18 has no `UPDATE ... FOR PORTION OF` (not in 18 or 19rc1): splitting periods is still application/SQL work.
- On 17, `WITHOUT OVERLAPS` and `PERIOD` are syntax errors (verified).

***

#### New in 18: B-tree skip scan and other planner improvements

##### Skip scan

A multicolumn B-tree index can now be used when the leading column has no equality condition, by "skipping" through its distinct values.[^1][^14]

```sql
CREATE TABLE u (id int PRIMARY KEY, email text, status text, created_at timestamptz, org int);
INSERT INTO u SELECT g, 'user'||g||'@ex.com',
       CASE WHEN g % 50 = 0 THEN 'active' ELSE 'inactive' END,
       '2025-01-01'::timestamptz + g * interval '1 minute', g % 100
FROM generate_series(1, 100000) g;
CREATE INDEX u_org_created ON u (org, created_at);
ANALYZE u;

EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, SUMMARY OFF, BUFFERS OFF)
SELECT * FROM u WHERE created_at = '2025-01-10 00:00+00';
```

```text
-- 18.6
 Index Scan using u_org_created on u (actual rows=1.00 loops=1)
   Index Cond: (created_at = '2025-01-10 00:00:00+00'::timestamp with time zone)
   Index Searches: 101
-- 17.11, same data
 Seq Scan on u (actual rows=1 loops=1)
   Filter: (created_at = '2025-01-10 00:00:00+00'::timestamp with time zone)
   Rows Removed by Filter: 99999
```

Review implications:

- Skip scan pays off only when the skipped leading column has **few distinct values** (here 100 → 101 index searches). With a high-cardinality leading column the planner still chooses a sequential scan (verified: `(a, b)` index with 200,000 distinct `a`, `WHERE b = 3` → `Seq Scan`). Do not accept "18 has skip scan" as a reason to skip an index whose leading column matches the predicate.
- It can make a separate single-column index redundant when the leading column is low-cardinality (tenant, status, region). Check `pg_stat_user_indexes.idx_scan` before dropping anything.
- `Index Searches:` in `EXPLAIN ANALYZE` (new in 18) shows how many descents happened; a large number on a hot query means the index column order is wrong.

##### Other planner changes worth knowing (verified plans on 18.6 vs 17.11)

```sql
EXPLAIN (COSTS OFF) SELECT * FROM u WHERE id = 1 OR id = 2 OR id = 3;
--  18: Index Scan using u_pkey ... Index Cond: (id = ANY ('{1,2,3}'::integer[]))
--  17: Bitmap Heap Scan with a BitmapOr of three Bitmap Index Scans

EXPLAIN (COSTS OFF) SELECT * FROM u WHERE id IN (VALUES (1),(2),(3));
--  18: Index Scan ... Index Cond: (id = ANY ('{1,2,3}'::integer[]))
--  17: Nested Loop over HashAggregate of a Values Scan

EXPLAIN (COSTS OFF) SELECT a.id, b.email FROM u a JOIN u b ON a.id = b.id WHERE a.org = 5;
--  18: single Bitmap Heap Scan on u b   (self-join removed; enable_self_join_elimination)
--  17: Hash Join of two scans of u
```

Also new: `SELECT DISTINCT` key reordering (`enable_distinct_reordering`), functionally dependent `GROUP BY` columns dropped for any unique index (not only the primary key), Right Semi Join plans, incremental sort in merge joins, faster planning with many partitions, better plan caching for `LANGUAGE sql` functions, and better `generate_series()` row estimates for `numeric` and `timestamp`.[^1] Do not ask authors to rewrite `OR` chains into `= ANY(...)` or to hand-remove redundant self-joins purely for performance on 18 — the planner does it. (Readability is still a valid reason.)

***

#### New in 18: `NOT NULL` constraints in `pg_constraint`, `NOT VALID`, `NOT ENFORCED`

##### NOT NULL constraints are real constraints

`NOT NULL` constraints now appear in `pg_constraint` (`contype = 'n'`), have names, and can be added as `NOT VALID` and validated later.[^1][^11]

```sql
CREATE TABLE nnn (a int NOT NULL, b int CONSTRAINT b_required NOT NULL);
SELECT conname, contype FROM pg_constraint WHERE conrelid = 'nnn'::regclass ORDER BY 1;
--  b_required     | n
--  nnn_a_not_null | n
```

Tools that read `pg_constraint` and assumed only `c/f/p/u/t/x` types (schema diff tools, ORMs' introspection, custom migration checkers) see new rows; check that they handle `contype = 'n'`.

##### Adding NOT NULL to a big table without a long exclusive lock (New in 18)

```sql
-- Step 1: instant metadata change (brief ACCESS EXCLUSIVE lock); new rows are checked immediately
ALTER TABLE child ADD CONSTRAINT child_id_nn NOT NULL id NOT VALID;
-- Step 2: scan existing rows under SHARE UPDATE EXCLUSIVE (reads and writes continue)
ALTER TABLE child VALIDATE CONSTRAINT child_id_nn;
```

Verified on 18.6: step 1 holds `AccessExclusiveLock` only for the catalog update, inserts of `NULL` fail right after it, and `VALIDATE CONSTRAINT` holds `ShareUpdateExclusiveLock`; after validation `attnotnull` is true and `\d` shows the column as `not null`. Plain `ALTER COLUMN ... SET NOT NULL` still takes `ACCESS EXCLUSIVE` for the full-table scan. On 17 the `NOT NULL ... NOT VALID` syntax does not exist; the pre-18 technique (still valid) is a validated `CHECK (col IS NOT NULL)` constraint, which lets `SET NOT NULL` skip the scan:

```sql
ALTER TABLE child ADD CONSTRAINT bid_nn CHECK (big_id IS NOT NULL) NOT VALID;
ALTER TABLE child VALIDATE CONSTRAINT bid_nn;
ALTER TABLE child ALTER COLUMN big_id SET NOT NULL;
-- DEBUG: existing constraints on column "child.big_id" are sufficient to prove that it does not contain nulls
ALTER TABLE child DROP CONSTRAINT bid_nn;
```

##### `NOT ENFORCED` (New in 18)

`CHECK` and foreign-key constraints can be declared `NOT ENFORCED`: they are recorded (`pg_constraint.conenforced = false`) but not checked.[^1]

```sql
CREATE TABLE ne (a int, CONSTRAINT pos CHECK (a > 0) NOT ENFORCED);
INSERT INTO ne VALUES (-1);   -- succeeds
```

A `NOT ENFORCED` constraint is documentation, not protection. Flag it on data that the application relies on. In 18, `ALTER CONSTRAINT ... ENFORCED`/`NOT ENFORCED` works for foreign keys only; for `CHECK` constraints it fails ("cannot alter enforceability") — that is a 19 feature.

##### Foreign keys on partitioned tables

`NOT VALID` foreign keys can now be added to partitioned tables (18), and `ALTER TABLE ... DROP CONSTRAINT ... ONLY` works on partitioned tables.[^1]

***

#### New in 18: EXPLAIN, COPY, functions, collations

##### EXPLAIN

- `EXPLAIN ANALYZE` includes buffer counts automatically; pass `BUFFERS OFF` to suppress them.
- Each index scan node reports `Index Searches: N`.
- Actual row counts are printed with two decimals (`actual rows=1.00`), so tooling that parses `rows=(\d+)` from ANALYZE output must be updated.
- Nodes disabled by `enable_*` settings show `Disabled: true` instead of a 1e10 cost penalty.
- Memory/disk usage for Material, Window Aggregate and CTE nodes; window function argument details; `EXPLAIN (WAL)` reports full WAL buffers.[^1]

##### COPY

```sql
CREATE TABLE cp (a int);
COPY cp FROM STDIN WITH (ON_ERROR ignore, REJECT_LIMIT 1);   -- 17: ON_ERROR; 18: REJECT_LIMIT
-- fails once more than 1 row is rejected:
-- ERROR:  skipped more than REJECT_LIMIT (1) rows due to data type incompatibility
```

- **Behaviour change:** in CSV mode, `COPY FROM` no longer treats a `\.` line as end-of-data; a server-side file containing `\.` now loads it as data (verified: `invalid input syntax for type integer: "\."` on a line that 17 would have treated as the end). `psql`'s `\copy ... FROM STDIN` still treats `\.` as the terminator, and older `psql` clients may misbehave against 18 servers.[^1][^34]
- `LOG_VERBOSITY silent` suppresses the per-row notices of `ON_ERROR ignore`.
- `COPY ... FREEZE` on foreign tables is now an error (it was silently ignored).
- `COPY TO` works from populated materialized views.

##### New and changed functions (all verified on 18.6, absent on 17.11)

```sql
SELECT casefold('Straße' COLLATE pg_unicode_fast);         -- strasse  (lower() gives straße)
SELECT array_sort(ARRAY[3,1,2]), array_reverse(ARRAY[1,2,3]); -- {1,2,3} | {3,2,1}
SELECT crc32('abc'::bytea), crc32c('abc'::bytea);
SELECT reverse('\x0102'::bytea), gamma(5), lgamma(5);
SELECT 258::bytea, '\x0102'::bytea::int;                   -- \x00000102 | 258
SELECT extract(week FROM interval '15 days');               -- 2 (17: "unit week not supported for type interval")
SELECT to_number('XIV', 'RN');                              -- 14
SELECT json_strip_nulls('[1,null,{"a":null}]', strip_in_arrays => true);  -- [1,{}]
SELECT ('{"a":null}'::jsonb -> 'a')::int;                   -- NULL (17: ERROR cannot cast jsonb null to type integer)
```

- `casefold()` is the correct tool for case-insensitive comparison (handles `ß`, Greek final sigma, etc.) but its result depends on the collation: with a libc collation (the `postgres` Docker image default, `en_US.utf8`) `casefold('Straße')` returns `straße`; with `pg_unicode_fast` it returns `strasse`.
- The `jsonb` null-cast change can hide bugs: code that relied on the cast *failing* for JSON `null` now silently gets SQL `NULL`.
- `MIN()`/`MAX()` accept arrays and composite types. Regular-expression functions accept named arguments. PL/pgSQL accepts `=>` for named cursor arguments.[^1]

##### Collations

- New builtin collation `PG_UNICODE_FAST` (`pg_unicode_fast`): code-point sort order with full Unicode case mapping (`upper('ß' COLLATE pg_unicode_fast)` is `SS`; with `pg_c_utf8` it stays `ß`).
- `LIKE` now works with nondeterministic collations (17 raised an error), e.g. a case-insensitive ICU collation `und-u-ks-level2`.[^1][^38]
- Primary-key/foreign-key pairs must use deterministic collations or the same nondeterministic collation.
- Full-text search now reads configuration files and dictionaries with the cluster's default collation provider rather than always libc; after `pg_upgrade` of an ICU/builtin cluster, reindex full-text and `pg_trgm` indexes.[^1]

***

#### New in 18: Authentication, security and operations changes

- **OAuth 2.0 authentication.** `pg_hba.conf` method `oauth` with options `issuer`, `scope`, optional `validator`, `map`, `delegate_ident_mapping`; tokens are verified by a validator module listed in `oauth_validator_libraries` (empty by default — the core ships no validator). libpq clients need a build with OAuth support (`--with-libcurl`). `issuer` must match the client's `oauth_issuer` exactly. `delegate_ident_mapping = 1` hands role authorization to the validator — review it like code that grants superuser.[^16] Not executable in this test environment.
- **MD5 deprecated** (see above). An `md5` line in `pg_hba.conf` still authenticates SCRAM-hashed passwords with SCRAM, so migrate passwords first, then change the method to `scram-sha-256`.[^17]
- **`AFTER` triggers run as the queuing role.** A deferred trigger queued by role `app` now executes as `app` even if the session switched role before `COMMIT` (verified: audit row shows `app` on 18.6, `attacker` on 17.11). Security-sensitive trigger code that used `current_user` behaves differently after upgrade.[^1]
- **Data checksums on by default** for new clusters (`initdb --no-data-checksums` to opt out). The official Docker image on 18 reports `data_checksums = on`, on 17 `off`. `pg_upgrade` requires matching checksum settings, so upgrading a non-checksum 17 cluster needs `initdb --no-data-checksums` for the new cluster (or enabling checksums offline with `pg_checksums` first).[^1][^37]
- **`pg_upgrade` keeps optimizer statistics** (except `CREATE STATISTICS` extended statistics and cumulative counters). Recommended follow-up: `vacuumdb --all --analyze-in-stages --missing-stats-only`, then `vacuumdb --all --analyze-only`. New `--swap` mode moves data directories (fast, but the old cluster is unusable once file transfer starts) and parallel checks via `--jobs`.[^15]
- **`pg_dump`/`pg_restore`/`pg_dumpall`** gain `--statistics`, `--statistics-only`, `--no-statistics`, `--no-data`, `--no-schema`, `--no-policies`, and `pg_dump --sequence-data`.[^1]
- **New predefined role** `pg_signal_autovacuum_worker`; `pg_get_acl()`; `has_largeobject_privilege()`; `ALTER DEFAULT PRIVILEGES` for large objects.[^1]
- **Wire protocol 3.2** with 256-bit cancel keys; libpq negotiates it only when `max_protocol_version` allows (default still 3.0 in 18's libpq).[^1]

***

#### Features from 14–17 that a PostgreSQL 18 codebase can use

Each row was verified to fail on the previous major and work on the listed one (13.x for 14 features through 17.11).[^31]

| Since | Feature |
| --- | --- |
| 14 | Multirange types (`int4multirange`, `range_agg()`), SQL-standard function bodies (`RETURN expr`, `BEGIN ATOMIC ... END`), `jsonb` subscripting (`doc['a']`), `date_bin()`, `ALTER TABLE ... DETACH PARTITION ... CONCURRENTLY`, `pg_read_all_data`/`pg_write_all_data`, `idle_session_timeout`, `SEARCH`/`CYCLE` in recursive CTEs, `GROUP BY DISTINCT`, LZ4 TOAST compression (`default_toast_compression`), `string_to_table()`, `password_encryption` default `scram-sha-256` |
| 15 | `MERGE`, `UNIQUE NULLS NOT DISTINCT`, views `WITH (security_invoker = true)`, `PUBLIC` loses `CREATE` on schema `public` (owned by `pg_database_owner`), `regexp_count/regexp_instr/regexp_like/regexp_substr`, `ON DELETE SET NULL (col, ...)` column lists, `pg_checkpoint` role, `jsonlog` log format; numeric literals followed directly by identifiers are rejected |
| 16 | SQL/JSON constructors `JSON_OBJECT`, `JSON_ARRAY`, `JSON_OBJECTAGG`, `JSON_ARRAYAGG` and `IS JSON [OBJECT|ARRAY|SCALAR]`, `any_value()`, numeric literals with underscores (`1_000_000`) and `0x`/`0o`/`0b` integers, `pg_input_is_valid()`/`pg_input_error_info()`, subqueries in `FROM` without alias, `EXPLAIN (GENERIC_PLAN)`, `pg_stat_io`, `SYSTEM_USER`, `date_add()`/`date_subtract()`, `array_shuffle()`/`array_sample()`, `random_normal()`, `pg_create_subscription` and `pg_use_reserved_connections` roles |
| 17 | `JSON_TABLE`, `JSON_QUERY`, `JSON_VALUE`, `JSON_EXISTS`, `JSON()`, `JSON_SCALAR()`, `JSON_SERIALIZE()`, jsonpath `.integer()`/`.bigint()`/`.number()`/`.string()`/`.boolean()`/`.date()`/`.time()`/`.timestamp()`..., `MERGE ... RETURNING` with `merge_action()`, `WHEN NOT MATCHED BY SOURCE`, `MERGE` into updatable views, `COPY ... (ON_ERROR ignore)`, `transaction_timeout`, `MAINTAIN` privilege and `pg_maintain` role, `EXPLAIN (SERIALIZE, MEMORY)`, `ALTER COLUMN ... SET EXPRESSION`, exclusion constraints on partitioned tables, `random(min, max)`, `uuid_extract_timestamp()`/`uuid_extract_version()`, `AT LOCAL`, `+/-infinity` intervals, `pg_stat_checkpointer`, login event triggers, incremental base backups (`pg_basebackup --incremental`, `pg_combinebackup`) |

Pitfall uncovered while verifying the table: on 14, `SELECT 1_000, 0x1F;` does **not** fail — it parses as `SELECT 1 AS _000, 0 AS x1F` and returns `1 | 0`. 15 rejects it, 16 gives it the new meaning. Code that must also run on 14 must not use these literals.

***
#### Schema migrations and locking safety

Most production incidents caused by migrations are lock problems, not data problems. Every `ALTER TABLE` takes a table-level lock; the dangerous ones take `ACCESS EXCLUSIVE`, which conflicts with **every** other lock including plain `SELECT`.[^12][^11]

##### The lock queue: why a "fast" ALTER takes the site down

An `ALTER TABLE` that needs `ACCESS EXCLUSIVE` waits behind any open transaction that touched the table — and every later query on that table queues **behind the waiting ALTER**. Verified on 18.6:

```text
session A: BEGIN; SELECT count(*) FROM lq; SELECT pg_sleep(6); COMMIT;     -- holds ACCESS SHARE
session B: ALTER TABLE lq ADD COLUMN x int;                                  -- waits (blocked by A)
session C: SET statement_timeout='2s'; SELECT count(*) FROM lq;              -- waits behind B
           ERROR:  canceling statement due to statement timeout
pg_stat_activity: B wait_event_type=Lock blocked by {A}; C wait_event_type=Lock blocked by {B}
```

The ALTER itself was instantaneous; the outage is the queue. Always set `lock_timeout` in migrations and retry:

```sql
-- GOOD: give up quickly instead of blocking all traffic; retry from the migration runner
SET lock_timeout = '2s';
SET statement_timeout = '30s';     -- for statements that must not run long
ALTER TABLE lq ADD COLUMN y int;
-- ERROR:  canceling statement due to lock timeout   (when a long transaction holds the table)
```

Defaults on 18 are `lock_timeout = 0`, `statement_timeout = 0`, `idle_in_transaction_session_timeout = 0`, `transaction_timeout = 0` (17+): all disabled. Use `SET LOCAL` inside a transaction so the setting does not leak to a pooled connection.

##### Lock levels of common DDL (verified with `pg_locks` on 18.6)

| Statement | Lock on table | Blocks reads? | Blocks writes? |
| --- | --- | --- | --- |
| `ALTER TABLE ... ADD COLUMN` (any kind), `DROP COLUMN`, `ALTER COLUMN TYPE`, `SET/DROP DEFAULT`, `SET NOT NULL`, `RENAME` | `ACCESS EXCLUSIVE` | yes | yes |
| `ADD CONSTRAINT ... CHECK ... [NOT VALID]`, `ADD CONSTRAINT ... NOT NULL ... NOT VALID` (18) | `ACCESS EXCLUSIVE` (brief if `NOT VALID`) | yes | yes |
| `ADD CONSTRAINT ... FOREIGN KEY ... [NOT VALID]` | `SHARE ROW EXCLUSIVE` on **both** tables | no | yes |
| `VALIDATE CONSTRAINT` (check, NOT NULL, FK) | `SHARE UPDATE EXCLUSIVE` (FK: `ROW SHARE` on referenced table) | no | no |
| `CREATE INDEX` | `SHARE` | no | **yes**, for the whole build |
| `CREATE INDEX CONCURRENTLY`, `REINDEX CONCURRENTLY`, `DROP INDEX CONCURRENTLY` | `SHARE UPDATE EXCLUSIVE` | no | no |
| `VACUUM` (plain), `ANALYZE` | `SHARE UPDATE EXCLUSIVE` | no | no |
| `ALTER COLUMN ... SET STATISTICS` | `SHARE UPDATE EXCLUSIVE` | no | no |
| `ADD CONSTRAINT ... UNIQUE/PRIMARY KEY USING INDEX` | `ACCESS EXCLUSIVE` (brief, no scan) | yes | yes |
| `REINDEX INDEX` / `REINDEX TABLE` (plain) | `SHARE` on the table, `ACCESS EXCLUSIVE` on each index | queries that would use the index block | yes |
| `DROP INDEX` (plain) | `ACCESS EXCLUSIVE` on the table | yes | yes |
| `VACUUM FULL`, `CLUSTER`, `TRUNCATE` | `ACCESS EXCLUSIVE` | yes | yes |

##### Which ALTERs rewrite the table (verified with `pg_relation_filenode` on 18.6)

A rewrite holds `ACCESS EXCLUSIVE` for the time it takes to copy the whole table and rebuild every index.

| Change | Rewrite? |
| --- | --- |
| `ADD COLUMN c int` / `DEFAULT 42` / `DEFAULT now()` (constant or stable default, "fast default" since 11) | no |
| `ADD COLUMN c uuid DEFAULT gen_random_uuid()` (volatile default) | **yes** |
| `ADD COLUMN ... GENERATED ALWAYS AS (...)` (virtual, 18 default) | no |
| `ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` | **yes** |
| `ALTER COLUMN x TYPE bigint` from `int` | **yes** |
| `ALTER COLUMN x TYPE numeric(20,0)` from `bigint` | **yes** |
| `varchar(10)` → `varchar(20)` or → `text` | no |
| `varchar(20)` → `varchar(10)` (narrowing) | **yes** |
| `timestamp` → `timestamptz` with session `TimeZone = 'UTC'` | no (otherwise yes) |
| `DROP COLUMN` | no (space reclaimed only by later rewrites) |

```sql
-- BAD: one statement, rewrite of a large table under ACCESS EXCLUSIVE
ALTER TABLE events ADD COLUMN public_id uuid NOT NULL DEFAULT gen_random_uuid();

-- GOOD: nullable column (instant), backfill in batches, then constrain without a long lock
ALTER TABLE events ADD COLUMN public_id uuid;                 -- instant
ALTER TABLE events ALTER COLUMN public_id SET DEFAULT uuidv7(); -- instant; applies to new rows
-- backfill in batches of a few thousand rows, each in its own transaction:
UPDATE events SET public_id = uuidv7()
WHERE id IN (SELECT id FROM events WHERE public_id IS NULL ORDER BY id LIMIT 5000);
ALTER TABLE events ADD CONSTRAINT events_public_id_nn NOT NULL public_id NOT VALID;   -- 18
ALTER TABLE events VALIDATE CONSTRAINT events_public_id_nn;
```

`int` → `bigint` primary-key migrations need a new column, batched backfill, a trigger to keep both in sync, a concurrently built unique index, and a short swap transaction. Flag any migration that does `ALTER COLUMN id TYPE bigint` on a large table as a full rewrite plus rebuild of every index and every referencing foreign key check.

##### Indexes in migrations

```sql
-- BAD on a busy table: blocks all INSERT/UPDATE/DELETE for the whole build
CREATE INDEX orders_customer_idx ON orders (customer_id);

-- GOOD
CREATE INDEX CONCURRENTLY orders_customer_idx ON orders (customer_id);
```

- `CREATE INDEX CONCURRENTLY` cannot run inside a transaction block (`ERROR: CREATE INDEX CONCURRENTLY cannot run inside a transaction block`). Migration frameworks that wrap each migration in a transaction need their "no transaction" mode (e.g. `disable_ddl_transaction!` in Rails, `-- +goose NO TRANSACTION`, Flyway `executeInTransaction=false`, Alembic `autocommit_block()`).
- If it fails (deadlock, unique violation, cancel), it leaves an **INVALID** index that still costs write overhead. The migration must check `pg_index.indisvalid` and `DROP INDEX CONCURRENTLY` + recreate, or `REINDEX INDEX CONCURRENTLY`.
- It waits for all transactions that could use the old snapshot; a long-running transaction anywhere in the database stalls it.
- It cannot be used on a **partitioned** table (`ERROR: cannot create index on partitioned table "m" concurrently`); see Partitioning for the `ON ONLY` + `ATTACH PARTITION` pattern.
- Unique constraints: build `CREATE UNIQUE INDEX CONCURRENTLY`, then `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE USING INDEX ...` (instant; renames the index). The same works for `PRIMARY KEY USING INDEX`.

##### Constraints in migrations

```sql
-- Foreign key without a long write block on both tables:
ALTER TABLE child ADD CONSTRAINT child_big_fk FOREIGN KEY (big_id) REFERENCES big (id) NOT VALID;
ALTER TABLE child VALIDATE CONSTRAINT child_big_fk;   -- SHARE UPDATE EXCLUSIVE; reads and writes continue

-- CHECK constraint:
ALTER TABLE child ADD CONSTRAINT child_id_pos CHECK (id > 0) NOT VALID;
ALTER TABLE child VALIDATE CONSTRAINT child_id_pos;
```

`NOT VALID` constraints are enforced for new and updated rows immediately; only the scan of existing rows is deferred.

##### Other migration rules

- **One risky operation per transaction.** Several `ACCESS EXCLUSIVE` statements in one transaction hold the strongest lock until `COMMIT`.
- **Renames and drops break running code.** Deploy code that no longer uses the column first (expand/contract). `DROP COLUMN` is instant but irreversible.
- **`ALTER TYPE ... ADD VALUE`** works inside a transaction block, but the new value cannot be used until the transaction commits. Enum values cannot be removed; prefer a lookup table or `CHECK` constraint for value sets that change.
- **`VACUUM FULL`/`CLUSTER`** rewrite under `ACCESS EXCLUSIVE`. In 18 there is no `REPACK` (19); use `pg_repack` (extension) or accept downtime.
- **`TRUNCATE`** takes `ACCESS EXCLUSIVE` and is not MVCC-safe for concurrent readers in other snapshots.
- **Data backfills** belong in batches with commits between them; one giant `UPDATE` creates a long transaction, doubles table size with dead tuples, and blocks `CREATE INDEX CONCURRENTLY` elsewhere.

***

#### Data types and their pitfalls

##### Integers, identity and sequences

```sql
-- GOOD: SQL-standard identity; bigint for anything that can grow
CREATE TABLE s2 (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, v text);
INSERT INTO s2 (id, v) VALUES (5, 'x');
-- ERROR:  cannot insert a non-DEFAULT value into column "id"
-- HINT:  Use OVERRIDING SYSTEM VALUE to override.

-- Legacy: serial creates a separately owned sequence plus a DEFAULT; avoid in new schemas
CREATE TABLE s1 (id serial PRIMARY KEY, v text);
```

- Prefer `GENERATED ALWAYS AS IDENTITY` (rejects accidental explicit ids) or `BY DEFAULT` (allows them — then data imports must reset the sequence with `setval`).
- Use `bigint` keys for tables that can exceed 2^31 rows over their lifetime; converting later is a rewrite.
- Sequences are not transactional: a rolled-back insert consumes a value (verified: rollback, then next insert gets `id = 2`). Never rely on gap-free ids; use a counter row with `UPDATE ... RETURNING` under a lock for invoice numbers.
- Integer division truncates: `1/3` is `0`, `-7/2` is `-3`, `-7 % 2` is `-1`. Cast one side to `numeric` for decimal results.

##### `numeric` vs floating point vs `money`

```sql
SELECT 0.1::float8 + 0.2::float8 = 0.3::float8;   -- f
SELECT 0.1::numeric + 0.2 = 0.3;                  -- t
SELECT round(2.5::numeric), round(2.5::float8);   -- 3 | 2  (numeric rounds half away from zero, float8 to even)
SELECT 'NaN'::numeric = 'NaN'::numeric;           -- t   (PostgreSQL treats NaN as equal to itself and greater than all numbers)
```

- Money and quantities that must add up exactly: `numeric(p,s)` (or integer minor units). Never `float8`/`real`.
- Avoid the `money` type: its output and input depend on `lc_monetary`, and it has fixed fractional precision.[^32]
- `numeric` without precision stores any scale; add `(p,s)` when the domain has one so bad data fails on insert.

##### Text types

```sql
SELECT 'ab '::char(5) = 'ab'::char(5), length('ab '::char(5));   -- t | 2  (trailing spaces are insignificant)
SELECT 'abc'::varchar(2);                                         -- ab  (explicit cast silently truncates!)
INSERT INTO vt VALUES ('abc');    -- with vt(v varchar(2)): ERROR: value too long for type character varying(2)
```

- Use `text` (or `varchar` without a limit) by default. A length limit is a business rule; if it matters, `CHECK (length(col) <= n)` can be changed without a rewrite, while widening `varchar(n)` is also metadata-only but narrowing is a rewrite and validation.
- Never use `char(n)`: it pads with spaces, strips them in comparisons and `length()`, and wastes space.[^32]
- `''` is not `NULL` in PostgreSQL (unlike Oracle).
- Case-insensitive uniqueness: a unique index on `lower(email)` (or `casefold(email)` with a suitable collation in 18), or a nondeterministic ICU collation; not `citext` for new code unless already in use.

##### Date and time

```sql
SET timezone = 'Europe/Berlin';
SELECT '2025-03-30 02:30'::timestamptz;                       -- 2025-03-30 03:30:00+02  (nonexistent local time shifted)
SELECT timestamptz '2025-03-29 12:00' + interval '1 day',     -- 2025-03-30 12:00:00+02  (calendar day)
       timestamptz '2025-03-29 12:00' + interval '24 hours';  -- 2025-03-30 13:00:00+02  (elapsed time)
SELECT '2025-01-31'::date + interval '1 month';               -- 2025-02-28 00:00:00  (clamped)
SELECT '2025-01-31 23:59:59.5'::timestamp BETWEEN '2025-01-01' AND '2025-01-31';  -- f
```

- **Store instants as `timestamptz`**, never `timestamp` (without time zone): `timestamptz` stores UTC and converts on input/output using the session `TimeZone`; `timestamp` stores whatever wall-clock value the client sent, losing the offset.[^32][^39]
- Use half-open ranges for periods: `created_at >= $1 AND created_at < $2`. `BETWEEN '2025-01-01' AND '2025-01-31'` misses everything after midnight on the 31st.
- `now()`/`current_timestamp` are the transaction start time — identical for every row and statement in a transaction. Use `clock_timestamp()` for wall-clock time inside long transactions.
- `date + interval '1 month'` clamps to the month end; repeated monthly addition drifts (`Jan 31 → Feb 28 → Mar 28`). Compute from the anchor date.
- `timestamptz::date` depends on the session `TimeZone`; the same query returns different days for different clients. Use `(ts AT TIME ZONE 'Europe/Berlin')::date` explicitly.
- Do not use `timetz` (time with time zone): an offset without a date is meaningless across DST.[^32]
- 18 changed time zone abbreviation resolution: abbreviations of the session's `TimeZone` are checked before `timezone_abbreviations`.[^1]

##### UUIDs

Use the `uuid` type; prefer `uuidv7()` for keys in 18 (see above). Do not compare UUIDs as strings — case and hyphen differences make equal values unequal.

##### NULL semantics

```sql
SELECT 1 WHERE 1 NOT IN (SELECT unnest(ARRAY[2, NULL]));                        -- 0 rows!
SELECT 1 WHERE NOT EXISTS (SELECT 1 FROM unnest(ARRAY[2, NULL]) u WHERE u = 1); -- 1 row
SELECT count(*), count(x) FROM (VALUES (1), (NULL)) v(x);                        -- 2 | 1
SELECT sum(x) FROM (SELECT 1 WHERE false) v(x);                                  -- NULL, not 0
SELECT NULL = NULL, NULL IS DISTINCT FROM NULL;                                  -- NULL | f
```

- **`NOT IN (subquery)` returns no rows if the subquery yields any `NULL`.** Use `NOT EXISTS`. This is one of the most common real bugs in SQL review.
- `sum()`, `avg()`, `max()` over zero rows return `NULL`: wrap in `coalesce(..., 0)` where the application expects a number.
- `col = NULL` is never true; use `IS NULL` / `IS NOT DISTINCT FROM`.
- Unique constraints allow many `NULL`s unless declared `NULLS NOT DISTINCT` (15+).

##### Enums, arrays, JSON as schema

- Enums: values can be added (`ALTER TYPE ... ADD VALUE`) but not removed or reordered without recreating the type; ordering is declaration order, not alphabetical.
- Arrays of foreign keys cannot have referential integrity; use a junction table.
- `jsonb` is for genuinely schemaless data; columns the application filters/joins on belong in real columns with types and constraints.

***

#### Constraints and keys

- **Every table has a primary key.** Logical replication of `UPDATE`/`DELETE` needs a replica identity; tables without a key cannot be deduplicated safely.
- **Index foreign-key columns on the referencing side.** PostgreSQL indexes the referenced key (it must be unique) but **not** the referencing column. Without an index, every `DELETE`/key `UPDATE` on the parent scans the child table, and `ON DELETE CASCADE` becomes a sequential scan per parent row.
- **`ON DELETE CASCADE` deliberately.** It is correct for owned children (order lines) and dangerous for shared references (deleting a customer must not delete invoices). `ON DELETE SET NULL (col)` (15+) nulls only the listed columns of a composite FK.
- **Deferrable constraints** (`DEFERRABLE INITIALLY DEFERRED`) are checked at commit; a deferrable unique/PK constraint cannot be used as an `ON CONFLICT` arbiter and its index is slower to check. Only use them where cyclic updates need it.
- **`CHECK` constraints must be immutable expressions of the row**; they are not re-checked when referenced data elsewhere changes. Do not call functions that read other tables.
- **Exclusion constraints** (`EXCLUDE USING gist`) and 18's `WITHOUT OVERLAPS` prevent overlaps that application-level checks cannot (race conditions).
- **`UNIQUE NULLS NOT DISTINCT`** (15+) when `NULL` should be one value:

```sql
CREATE TABLE nd (a int, b int, UNIQUE NULLS NOT DISTINCT (a, b));
INSERT INTO nd VALUES (1, NULL);
INSERT INTO nd VALUES (1, NULL);   -- ERROR: duplicate key value violates unique constraint "nd_a_b_key"
```

***

#### Indexing

Index types: B-tree (default; equality, ranges, sorting, `LIKE 'prefix%'` with the right operator class), Hash (equality only), GIN (`jsonb`, arrays, full-text, trigram), GiST (ranges, geometry, exclusion constraints, trigram), SP-GiST, BRIN (huge append-only tables whose physical order correlates with the column).[^27]

```sql
CREATE INDEX orders_user_status ON orders (user_id, status);            -- composite: column order matters
CREATE INDEX users_active_email ON users (email) WHERE status = 'active'; -- partial
CREATE INDEX users_lower_email ON users (lower(email));                  -- expression
CREATE INDEX users_email_cov ON users (email) INCLUDE (name, created_at); -- covering (index-only scans)
CREATE INDEX posts_fts ON posts USING gin (to_tsvector('english', title || ' ' || body));
CREATE INDEX events_doc ON events USING gin (doc jsonb_path_ops);
```

Verified behaviours on 18.6 (100,000-row table, default `en_US.utf8` collation):

| Query | Index | Plan |
| --- | --- | --- |
| `WHERE email LIKE 'user42%'` | plain `(email)` | **Seq Scan** — a B-tree with a non-C collation cannot serve `LIKE` |
| same | `(email text_pattern_ops)` | Bitmap Index Scan with `~>=~`/`~<~` range |
| `WHERE lower(email) = '...'` | plain `(email)` | Seq Scan — needs an index on `lower(email)` |
| `WHERE id = 42::numeric` (id is `int`) | PK | **Seq Scan**: `(id)::numeric = '42'` — the cast is on the column |
| `WHERE id::text = '42'` | PK | Seq Scan |
| `WHERE email LIKE '%r4242%'` | `gin (email gin_trgm_ops)` (pg_trgm) | Bitmap Index Scan |
| `WHERE status = $1 AND created_at > ...` with generic plan | partial index `WHERE status = 'active'` | **Seq Scan** — a partial index is unusable when the predicate value is a parameter |
| `WHERE (created_at, id) < ($1, $2) ORDER BY created_at DESC, id DESC LIMIT 20` | `(created_at DESC, id DESC)` | Index Only Scan, no sort |

Review rules:

- **Composite order**: equality columns first, then the range/sort column. 18's skip scan helps only when leading columns are low-cardinality.
- **Mismatched types in predicates disable indexes.** Bind parameters with the column's type (an ORM sending `numeric` for an `int` column, or `text` for a `uuid`, causes casts on the column side in some drivers).
- **Partial indexes and prepared statements**: the planner can only use a partial index when it can prove the predicate at plan time; a generic plan with `status = $1` cannot. Inline the constant or use a non-partial index.
- **Every index costs writes** and blocks HOT updates on its columns. Remove duplicates and unused indexes (`pg_stat_user_indexes.idx_scan = 0` over a full business cycle, not on replicas only — stats are per-server).
- **Index-only scans** need a recently vacuumed table (visibility map). `INCLUDE` columns cannot be used for search or ordering.
- **Hash indexes** are WAL-logged and crash-safe since 10, but rarely beat B-tree; they cannot be unique or multi-column.
- **BRIN** only helps if physical order correlates with the column (`pg_stats.correlation` near ±1).
- **GIN on jsonb**: the default `jsonb_ops` supports `@>`, `?`, `?|`, `?&`, `@?`, `@@`; `jsonb_path_ops` is smaller and faster but supports only `@>`, `@?`, `@@` (verified: `doc ? 'type'` → Seq Scan with a `jsonb_path_ops` index). Neither helps `doc->>'type' = 'x'`; that needs an expression B-tree index on `(doc->>'type')`.

***

#### Query writing and performance

##### Select what you need

`SELECT *` is acceptable in ad-hoc queries and narrow tables. In application code it matters for wide tables (TOASTed columns are fetched and decompressed), for index-only scans, and for stable column contracts when columns are added. It is not a defect by itself.

##### Eliminate N+1 queries

```python
# BAD: one query per user
for user in users:
    cur.execute("SELECT * FROM orders WHERE user_id = %s", (user.id,))

# GOOD: one round trip; the array is a single bind parameter
cur.execute("SELECT * FROM orders WHERE user_id = ANY(%s)", (user_ids,))
```

`= ANY($1::int[])` with an array parameter is preferable to building `IN ($1, $2, ..., $n)` with a variable number of placeholders: one statement text, one prepared plan. (In 18, `pg_stat_statements` squashes constant lists: verified, `IN (1,2)`, `IN (1,2,3,4,5)` and `= ANY(ARRAY[1,2,3])` all land in one entry shown as `IN ($1 /*, ... */)` with 3 calls — 17 kept one entry per list length.)[^1][^40]

##### Pagination

```sql
-- BAD for deep pages: reads and discards 100,000 rows
SELECT * FROM users ORDER BY created_at DESC LIMIT 20 OFFSET 100000;

-- GOOD: keyset pagination with a unique tie-breaker and a matching index
CREATE INDEX users_created_id ON users (created_at DESC, id DESC);
SELECT * FROM users
WHERE (created_at, id) < ($1, $2)          -- last row of the previous page
ORDER BY created_at DESC, id DESC
LIMIT 20;
```

Row-value comparison `(a, b) < (x, y)` uses the composite index as one range (verified: `Index Cond: (ROW(created_at, id) < ROW(...))`). The sort key must be unique (add the primary key) or rows are skipped/duplicated across pages. `LIMIT` without `ORDER BY` returns an arbitrary, unstable subset.

##### Counting

`SELECT count(*) FROM big_table` scans the table (or an index-only scan of the smallest index). For UI counters use an estimate (`SELECT reltuples::bigint FROM pg_class WHERE oid = 'big_table'::regclass`) or a maintained counter. `count(col)` skips `NULL`s — not the same as `count(*)`.

##### Sargable predicates

```sql
-- BAD: function/cast on the indexed column
WHERE date(created_at) = '2025-02-03'
WHERE created_at::date = '2025-02-03'
WHERE lower(email) = $1                 -- unless an index on lower(email) exists
WHERE id::text = $1

-- GOOD: compare the raw column with a range or a correctly typed parameter
WHERE created_at >= '2025-02-03' AND created_at < '2025-02-04'
```

On partitioned tables the same rule decides partition pruning: `created_at::date = ...` scans every partition (verified).

##### CTEs, subqueries and joins

- Since 12, a non-recursive, side-effect-free CTE referenced once is **inlined** (verified: `WITH c AS (SELECT * FROM ct) SELECT * FROM c WHERE id = 5` → Index Scan). `WITH ... AS MATERIALIZED` forces an optimization fence (CTE Scan + Seq Scan). Do not request "rewrite the CTE as a subquery for performance" on 18 unless the plan proves it.
- `NOT IN (subquery)` → `NOT EXISTS` (correctness, see NULL semantics).
- Top-N per group: `CROSS JOIN LATERAL (... ORDER BY ... LIMIT n)` with an index on `(group_col, sort_col)`, or `DISTINCT ON (group_col) ... ORDER BY group_col, sort_col DESC` for n = 1.
- `UNION` sorts/hashes to remove duplicates; use `UNION ALL` when duplicates are impossible or wanted.
- Explicit `JOIN ... ON` is a readability/safety preference over comma joins; the planner treats both the same.
- Keep non-aggregate filters in `WHERE`, not `HAVING` (readability; the planner moves them anyway, and 18 pushes some `HAVING` clauses on grouping sets down too).

##### Batch writes

```sql
-- GOOD: multi-row insert, or COPY for bulk loads
INSERT INTO users (name, email) VALUES ('Alice', 'a@example.com'), ('Bob', 'b@example.com');

-- From the client: COPY ... FROM STDIN (driver API) or psql \copy.
-- Server-side COPY FROM '/path' reads the server's file system and needs
-- pg_read_server_files (or superuser): never build it from user input.

-- Batch UPDATE from a VALUES list or a temp table
UPDATE users u SET status = v.status
FROM (VALUES (1, 'active'), (2, 'inactive')) AS v(id, status)
WHERE u.id = v.id;
```

Run `ANALYZE` after bulk loads into new tables; autovacuum's analyze may come too late for the next query.

##### Materialized views

`REFRESH MATERIALIZED VIEW` takes `ACCESS EXCLUSIVE` and blocks readers; `REFRESH MATERIALIZED VIEW CONCURRENTLY` needs a unique index on the view (in 18 the unique index may also be a non-B-tree index that supports equality) and allows reads during the refresh.[^1]

##### Planner settings

`SET enable_nestloop = off` (or any `enable_*`) in application code affects every later query in the session — and in a pooled connection, other clients. In 18, `EXPLAIN` marks such nodes `Disabled: true`. Fix statistics (`ANALYZE`, `ALTER TABLE ... ALTER COLUMN ... SET STATISTICS`, `CREATE STATISTICS` for correlated columns) instead of disabling planner features. There are no core query hints; `pg_hint_plan` is an extension, and `pg_plan_advice` is 19.

***

#### Reading EXPLAIN

```sql
EXPLAIN SELECT ...;                                  -- plan + estimates, does not execute
EXPLAIN (ANALYZE) SELECT ...;                        -- executes; 18 includes BUFFERS automatically
EXPLAIN (ANALYZE, BUFFERS OFF, TIMING OFF) ...;      -- quieter output
EXPLAIN (GENERIC_PLAN) SELECT * FROM t WHERE id = $1;  -- 16+: plan for a parameterized statement
EXPLAIN (ANALYZE, MEMORY, SERIALIZE) ...;            -- 17+: planner memory, output serialization cost
```

`EXPLAIN ANALYZE` **executes** the statement: wrap `INSERT/UPDATE/DELETE/MERGE` in `BEGIN; ... ROLLBACK;`.

What to look for:[^28]

- **Estimated vs actual rows** off by orders of magnitude → stale or insufficient statistics (`ANALYZE`, extended statistics for correlated columns).
- **`Rows Removed by Filter`** large on a Seq Scan or Index Scan → missing or wrong index, or non-sargable predicate.
- **`Index Searches:`** (18) large → skip scan over a high-cardinality prefix or an `= ANY` with many elements; consider column order.
- **`Buffers: shared read=`** high → cold cache or too much data touched; compare with `hit=`.
- **`Sort Method: external merge  Disk:`** → `work_mem` too small for this sort (raise per query/role, not globally).
- **`Heap Fetches:`** on Index Only Scan high → visibility map stale; vacuum.
- **Nested Loop with a large outer side** and inner Seq Scan → missing join index or misestimate.
- **`Subplans Removed: N`** under Append → runtime partition pruning worked (verified with a generic plan on a range-partitioned table).
- Actual rows are printed with two decimals in 18 (`rows=1.00`); averaged per loop.

Example (18.6, `EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, SUMMARY OFF)`):

```text
 Index Scan using u_pkey on u (actual rows=3.00 loops=1)
   Index Cond: (id = ANY ('{1,500,90000}'::integer[]))
   Index Searches: 3
   Buffers: shared hit=9
 Planning:
   Buffers: shared hit=8
```

***

#### Transactions, isolation and concurrency

The default isolation level is **READ COMMITTED**: each statement sees data committed before it started.[^21]

##### Lost updates (verified)

```sql
-- BAD (application read-modify-write under READ COMMITTED):
BEGIN;
SELECT n FROM ctr WHERE id = 1;          -- both sessions read 0
UPDATE ctr SET n = 1 WHERE id = 1;       -- both write 0 + 1
COMMIT;                                   -- final value 1, one increment lost

-- GOOD: let the database compute it atomically
UPDATE ctr SET n = n + 1 WHERE id = 1 RETURNING n;

-- GOOD: lock the row when the new value is computed in application code
BEGIN;
SELECT n FROM ctr WHERE id = 1 FOR UPDATE;
UPDATE ctr SET n = $1 WHERE id = 1;
COMMIT;

-- GOOD: optimistic concurrency with a version column
UPDATE doc SET body = $1, version = version + 1 WHERE id = $2 AND version = $3;  -- 0 rows → conflict
```

##### REPEATABLE READ and SERIALIZABLE need retries

Verified on 18.6: a `REPEATABLE READ` transaction updating a row that another transaction changed after its snapshot fails with `ERROR: could not serialize access due to concurrent update` (SQLSTATE `40001`). Under `SERIALIZABLE`, two doctors each checking "at least two on call" and going off call (write skew) — one fails with `could not serialize access due to read/write dependencies among transactions`, and the invariant holds.

- Code that uses `REPEATABLE READ` or `SERIALIZABLE` **must retry the whole transaction** on `40001` (and on deadlock `40P01`). Without a retry loop these levels just turn races into errors.
- `SERIALIZABLE` is the simplest correct choice for complex invariants; keep transactions short and read-only ones `READ ONLY` (or `DEFERRABLE` for long reports).

##### Upsert: `INSERT ... ON CONFLICT` vs `MERGE`

```sql
INSERT INTO kv (k, v, updated_at) VALUES ($1, $2, $3)
ON CONFLICT (k) DO UPDATE
   SET v = excluded.v, updated_at = excluded.updated_at
 WHERE kv.updated_at < excluded.updated_at        -- ignore stale writes
RETURNING old.v, new.v;                            -- 18
```

- `ON CONFLICT` is atomic against concurrent inserts of the same key. **`MERGE` is not**: verified, two concurrent `MERGE ... WHEN NOT MATCHED THEN INSERT` statements for the same key → one fails with `duplicate key value violates unique constraint`, while the same race with `ON CONFLICT DO UPDATE` succeeds. Use `ON CONFLICT` for upserts; use `MERGE` (15+) for set-based synchronization where conflicts are handled or impossible.[^22][^23]
- `ON CONFLICT DO UPDATE` affecting the same key twice in one statement fails (`command cannot affect row a second time`): deduplicate the input first.
- `ON CONFLICT DO NOTHING RETURNING` returns nothing for skipped rows. There is no `DO SELECT` in 18 (19). To get the existing id, follow up with a `SELECT` or use `DO UPDATE SET k = excluded.k` (which writes a new row version — beware of bloat and triggers).
- A deferrable unique constraint cannot be an arbiter (`ON CONFLICT does not support deferrable unique constraints/exclusion constraints as arbiters`).
- `MERGE` (17+) supports `RETURNING merge_action(), ...`, `WHEN NOT MATCHED BY SOURCE THEN DELETE/UPDATE`, and targets updatable views:

```sql
MERGE INTO tgt t
USING (VALUES (1, 10), (4, 40)) AS s(id, v) ON t.id = s.id
WHEN MATCHED THEN UPDATE SET v = s.v
WHEN NOT MATCHED BY TARGET THEN INSERT VALUES (s.id, s.v)
WHEN NOT MATCHED BY SOURCE THEN DELETE                      -- deletes all other rows of tgt!
RETURNING merge_action(), t.*;
```

`WHEN NOT MATCHED BY SOURCE` acts on **every** target row not in the source; without an extra condition (`AND t.tenant_id = ...`) it deletes the rest of the table.

##### Queues, locks and deadlocks

```sql
-- Job queue: concurrent workers do not block each other
BEGIN;
SELECT id FROM jobs WHERE NOT done ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED;
-- ... process, then
UPDATE jobs SET done = true WHERE id = $1;
COMMIT;
```

- Use `FOR NO KEY UPDATE` instead of `FOR UPDATE` when not changing key columns; foreign-key checks take `FOR KEY SHARE` on the parent row, which conflicts with `FOR UPDATE` but not with `FOR NO KEY UPDATE`.
- Deadlocks come from locking rows in different orders. Lock in a consistent order (e.g. `ORDER BY id ... FOR UPDATE`) and retry on `40P01`.
- Advisory locks: prefer transaction-scoped `pg_advisory_xact_lock()` / `pg_try_advisory_xact_lock()`. Session-level `pg_advisory_lock()` leaks across a transaction-pooled connection (PgBouncer) and survives errors until explicit unlock.
- Long-running transactions (including `idle in transaction`) hold back vacuum for the whole cluster, block `CREATE INDEX CONCURRENTLY`, and hold locks. Set `idle_in_transaction_session_timeout` and, from 17, `transaction_timeout` for application roles.
- `now()` is fixed for the whole transaction (verified); `clock_timestamp()` moves.
- Exception blocks in PL/pgSQL and `SAVEPOINT`s create subtransactions; many of them per transaction (e.g. one per row in a loop) degrade performance cluster-wide (subtransaction SLRU overflow on replicas). Avoid per-row savepoints in drivers (some ORMs' "autosave" modes).

***

#### JSONB and SQL/JSON

Use `jsonb`, not `json`, unless exact input text (key order, duplicate keys, whitespace) must be preserved.[^25]

```sql
SELECT '{"a":1,"a":2}'::jsonb, '{"a":1,"a":2}'::json;   -- {"a": 2} | {"a":1,"a":2}
```

##### Operators and their traps (verified on 18.6)

```sql
SELECT doc -> 'a'        -- jsonb
     , doc ->> 'a'       -- text
     , doc #>> '{a,b}'   -- text at path
     , doc['a']          -- jsonb subscripting (14+)
FROM (SELECT '{"a":{"b":"x"}}'::jsonb AS doc) s;

SELECT ('{"n":"5"}'::jsonb -> 'n')::int;   -- ERROR: cannot cast jsonb string to type integer
SELECT ('{"n":"5"}'::jsonb ->> 'n')::int;  -- 5
SELECT ('{"n":null}'::jsonb -> 'n')::int;  -- NULL (18; 17 raised an error)

SELECT jsonb_set('{"a":1}', '{b}', NULL);        -- NULL: the whole document is lost!
SELECT jsonb_set_lax('{"a":1}', '{b}', NULL);    -- {"a": 1, "b": null}   (13+)
```

- **`jsonb_set(doc, path, NULL)` returns `NULL`** (it is strict). `UPDATE t SET doc = jsonb_set(doc, '{x}', to_jsonb($1))` wipes the column when `$1` is `NULL`. Use `jsonb_set_lax`, `coalesce`, or `||`.
- `->>` returns `text`; compare with text or cast the text. `->` returns `jsonb`; compare with `jsonb` literals.
- Updating one key rewrites the whole (TOASTed) value. Large documents updated frequently are a schema smell.
- `jsonb` numbers are `numeric`; JavaScript clients lose precision above 2^53.

##### Indexing jsonb

```sql
CREATE INDEX ev_gin  ON ev USING gin (doc jsonb_path_ops);   -- serves doc @> '{"type":"t7"}'
CREATE INDEX ev_type ON ev ((doc ->> 'type'));               -- serves doc->>'type' = 't7'
```

Verified: with only the GIN index, `doc @> '{"type":"t7"}'` uses it but `doc->>'type' = 't7'` and `doc ? 'type'` (not supported by `jsonb_path_ops`) are sequential scans. Write containment queries (`@>`) or add expression indexes for the access paths you use.

##### SQL/JSON (16, 17)

```sql
SELECT JSON_OBJECT('a': 1, 'b': NULL ABSENT ON NULL);        -- {"a" : 1}           (16)
SELECT '{"x":1}' IS JSON OBJECT;                               -- t                   (16)
SELECT JSON_VALUE('{"a":"5"}', '$.a' RETURNING int);           -- 5                   (17)
SELECT JSON_QUERY('{"a":[1,2]}', '$.a');                       -- [1, 2]              (17)
SELECT JSON_EXISTS('{"a":1}', '$.b');                          -- f                   (17)
SELECT * FROM JSON_TABLE(
  '[{"id":1,"n":"a"},{"id":2,"n":"b"}]', '$[*]'
  COLUMNS (id int PATH '$.id', n text PATH '$.n', ord FOR ORDINALITY));       -- (17)
```

- **`JSON_VALUE`/`JSON_QUERY` default to `NULL ON ERROR`**: `JSON_VALUE('{"a":"x"}', '$.a' RETURNING int)` returns `NULL` silently. Add `ERROR ON ERROR` where bad data must not be hidden.
- **`JSON_SERIALIZE()` on a `jsonb` argument returns garbage** on 17.11, 18.6 and 19rc1 (verified: `JSON_SERIALIZE('{"a":1}'::jsonb)` → `\x01`); it works for `json`/text input. Use `doc::text` to serialize `jsonb`.
- Prefer `jsonb_path_query`/`@?`/`@@` (jsonpath) over hand-written `jsonb_array_elements` loops for filtering.

***

#### Partitioning

Declarative partitioning (`PARTITION BY RANGE | LIST | HASH`) is for very large tables where partition pruning or dropping whole partitions (retention) pays off. It adds planning overhead and constraints; do not partition small tables.[^26]

```sql
CREATE TABLE m (
    id         bigint GENERATED ALWAYS AS IDENTITY,
    created_at timestamptz NOT NULL,
    v          int,
    PRIMARY KEY (id, created_at)              -- must include the partition key
) PARTITION BY RANGE (created_at);

CREATE TABLE m_2025_01 PARTITION OF m FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE m_2025_02 PARTITION OF m FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
```

Verified rules and traps on 18.6:

- **Unique constraints and primary keys must include all partition key columns** (`unique constraint on partitioned table must include all partitioning columns`). Global uniqueness of `id` alone is not enforceable; use an identity/sequence and accept it, or a separate lookup table.
- **`CREATE INDEX CONCURRENTLY` is not supported on the partitioned parent.** Build it without blocking writes like this:

```sql
CREATE INDEX m_v_idx ON ONLY m (v);                      -- invalid, parent only, instant
CREATE INDEX CONCURRENTLY m_2025_01_v ON m_2025_01 (v);  -- per partition
ALTER INDEX m_v_idx ATTACH PARTITION m_2025_01_v;
CREATE INDEX CONCURRENTLY m_2025_02_v ON m_2025_02 (v);
ALTER INDEX m_v_idx ATTACH PARTITION m_2025_02_v;        -- parent index becomes valid when all are attached
```

- **Pruning needs the raw key**: `WHERE created_at >= $1 AND created_at < $2` prunes (also at runtime for generic plans: `Subplans Removed: 2`); `WHERE created_at::date = ...` scans all partitions.
- **Default partition**: rows that match no partition go there; creating a new partition then has to scan the default partition to verify no rows belong to the new range, and `DETACH PARTITION ... CONCURRENTLY` (14+) is refused while a default partition exists (`cannot detach partitions concurrently when a default partition exists`). Prefer pre-creating partitions (pg_partman or a scheduled job) over a catch-all default.
- **Retention**: `ALTER TABLE m DETACH PARTITION m_2024_01 CONCURRENTLY; DROP TABLE m_2024_01;` instead of `DELETE` — no bloat, no long transaction.
- **Unlogged partitioned tables are rejected in 18** (`partitioned tables cannot be unlogged`; 17 accepted it without effect).
- **Statistics**: autovacuum analyzes partitions but never the partitioned parent; queries that need parent-level statistics (joins, `GROUP BY` across partitions) require a scheduled `ANALYZE parent`. In 18, `VACUUM`/`ANALYZE` of a parent recurse into children by default; `ANALYZE ONLY parent` analyzes just the parent.[^1][^36]
- `NOT VALID` foreign keys can be added to partitioned tables (18). Identity columns and exclusion constraints on partitioned tables are supported (exclusion constraints since 17).
- `ALTER TABLE ... MERGE PARTITIONS` / `SPLIT PARTITION` do not exist in 18 (or 19rc1); re-partitioning means create-new-partitions-and-move-rows.

***

