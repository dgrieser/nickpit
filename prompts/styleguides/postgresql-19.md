### PostgreSQL 19: The Complete Production SQL Review Guide

> **Version**: PostgreSQL 19 is **not generally available yet**. As of October 22, 2026 the newest build is **19 RC 1, released October 15, 2026**. The project plans **19 RC 2 for November 12, 2026** (together with the next minor releases of the supported branches) and **general availability on November 19, 2026**, "unless an issue is discovered that requires a delay or an additional release candidate". A release candidate is "mostly identical to the initial release", but fixes can still land before GA, so RC behaviour may still change.[^1][^3]
> **Support**: Once released, 19 follows the versioning policy (five years of minor releases); its final-release date is not yet listed. The newest *released* major is **18 (latest minor 18.6, August 13, 2026)**; supported majors are 18, 17, 16, 15 and 14, and 14 receives its final release on November 12, 2026.[^2][^4][^5]
> **Scope**: SQL that runs on a PostgreSQL 19 server: schema migrations, queries embedded in application code, functions/procedures/triggers, schema design, indexing, transactions, security, JSON, partitioning, connection handling and upgrades from 18. Every SQL example was executed on **PostgreSQL 19rc1** (`postgres:19rc1` Docker image, `server_version_num = 190000`); every "new in 19" claim was additionally checked to fail or behave differently on **18.6**.
> **Note**: This document is self-contained. Everything usable from PostgreSQL 14–18 is carried forward and marked with the version that introduced it. Sections marked **New in 19** cover behaviour specific to this release.

***

#### Table of Contents

1. Version-aware review
2. What's new in PostgreSQL 19 (overview)
3. New in 19: `REPACK` (replaces `VACUUM FULL` / `CLUSTER`)
4. New in 19: `INSERT ... ON CONFLICT DO SELECT`
5. New in 19: `IGNORE NULLS` in window functions
6. New in 19: `COPY` changes (JSON output, `ON_ERROR set_null`, multi-line headers, partitioned tables)
7. New in 19: replication — `WAIT FOR LSN`, logical decoding without restart, sequences, `EXCEPT`
8. New in 19: data types and functions
9. New in 19: constraints, DDL and utility commands
10. New in 19: planner, `EXPLAIN`, plan advice
11. New in 19: I/O, autovacuum and changed defaults
12. New in 19: security, authentication and string literals
13. Features from 18 that a PostgreSQL 19 codebase uses
14. Features from 14–17 that a PostgreSQL 19 codebase can use
15. Schema migrations and locking safety
16. Data types and their pitfalls
17. Constraints and keys
18. Indexing
19. Query writing and performance
20. Reading EXPLAIN
21. Transactions, isolation and concurrency
22. JSONB and SQL/JSON
23. Partitioning
24. Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL
25. Functions, procedures and triggers
26. Application code: parameters, pooling, timeouts
27. Maintenance and observability
28. Removed / changed in 19 — migration notes from 18
29. Production / review checklist

***

#### Version-aware review

The project pins PostgreSQL 19. Judge code against what 19 actually does:

- **19 is a release candidate until GA.** A deployment or CI image pinned to `postgres:19rc1`, `19beta*` or any pre-release build is acceptable for testing only. Production must move to 19.0 (planned November 19, 2026) or the latest 19.x minor once available. Do not ship code that depends on a behaviour that the release notes describe as still being fixed (the RC 1 announcement lists fixes for `REPACK (CONCURRENTLY)`, `pg_plan_advice`, planner optimizations that rely on whether an expression can be `NULL`, automatic logical-replication enablement and parallel autovacuum during the beta period).[^3]
- **Verify before you claim something is unavailable.** Many "new" features are older than reviewers remember: `MERGE` is 15, `MERGE ... RETURNING` and `JSON_TABLE` are 17, `uuidv7()`, virtual generated columns and `RETURNING old/new` are 18, `gen_random_uuid()` is core since 13. Check the version tables in "Features from 18" and "Features from 14–17" before asking for a workaround.
- **Say "not available in PostgreSQL 19" only when 19 proves it.** The "Do not use or recommend" list below is verified: each item fails on 19rc1.
- **Code that must also run on 18** cannot use anything marked **New in 19** — each of those was verified to fail (or behave differently) on 18.6. Several 19 changes alter behaviour without any syntax difference (`json_array()` over zero rows, backslashes in old scripts that set `standard_conforming_strings = off`, `COPY ... WHERE` on system columns, `postgres_fdw` inheriting `READ ONLY`, defaults such as `jit = off`); see the migration notes.
- **Client and pooler versions are part of the contract.** Dumps made with a pre-19 `pg_dump` under `standard_conforming_strings = off` do not restore correctly into 19; use 19's `pg_dump`/`pg_dumpall` for the upgrade.[^1]

##### Do not use or recommend (PostgreSQL 20 development, or never committed)

PostgreSQL 20 is in development (`20devel`); its release notes are a placeholder and nothing below is released. Each item is documented in the official development documentation (built from the master branch on 2026-10-21) and fails on 19rc1 (verified). Never suggest these for a 19 codebase:[^24]

| Feature (committed to master for 20) | Error on 19rc1 |
| --- | --- |
| `JSON_TABLE(... PLAN (...))` / `PLAN DEFAULT (...)` clause for `NESTED PATH` joins | syntax error at or near "PLAN" |
| `min(uuid)` / `max(uuid)` aggregates | function max(uuid) does not exist |
| `uuid_extract_timestamp()` for version-6 UUIDs | returns `NULL` on 19 (only v1 and v7 supported) |
| Storage parameter `toast_value_type = oid8` | unrecognized parameter "toast_value_type" |
| `pg_get_database_ddl()`, `pg_get_role_ddl()`, `pg_get_tablespace_ddl()` | function does not exist |
| `pg_enable_data_checksums()` / `pg_disable_data_checksums()` (online checksum changes) | function does not exist |
| `pg_stat_get_backend_lock()`, `pg_stat_reset_single_index_counters()` | function does not exist |
| Settings `enable_groupagg`, `log_statement_max_length` | unrecognized configuration parameter |

The 20 development docs no longer list the `array_nulls` setting, which still exists in 19 (`SHOW array_nulls` → `on`); do not build anything on `array_nulls = off`.

Also not available — these are frequently assumed but are neither in 19 nor in the 20 development docs (all are syntax errors on 19rc1): `GROUP BY ALL` meaning "group by every non-aggregate output column"; `UPDATE/DELETE ... FOR PORTION OF` (temporal DML); `ALTER TABLE ... MERGE PARTITIONS` / `SPLIT PARTITION`; SQL/PGQ (`CREATE PROPERTY GRAPH`, `GRAPH_TABLE`). Do not confuse the first with the grouping-sets keyword of 14: `GROUP BY ALL a, b` (the explicit opposite of `GROUP BY DISTINCT`) is valid and groups by the listed elements only; a bare `GROUP BY ALL` is `syntax error at or near ";"` on 19rc1.

##### Deprecated in 19 (still works, flag new uses)

- **`VACUUM FULL` and `CLUSTER`** are superseded by `REPACK` ("The old commands have been retained for compatibility"). Both still run on 19rc1.[^1][^6]
- **MD5 passwords** (deprecated since 18). 19 now also warns at **login**: `WARNING: authenticated with an MD5-encrypted password` (verified). Both warnings are controlled by `md5_password_warnings` (default `on`; the setting itself exists since 18, where it only covers setting a password).[^1][^15]
- **`pg_verifybackup --wal-directory`** is deprecated in favour of `--wal-path`.[^1]
- **`pg_buffercache_numa_pages()`** remains only for backward compatibility; use `pg_buffercache_os_pages()`.[^1]

##### Error-text changes observed on 19rc1 (not in the release notes)

Tests that compare exact error output need updates. Verified differences against 18.6:

- "function ... does not exist" now carries `DETAIL: There is no function of that name.` or `DETAIL: No function of that name accepts the given argument types.` (18 put the explanation into the `HINT`: `No function matches the given name and argument types.`). "operator does not exist" and "operator is not unique" likewise moved the explanation into a `DETAIL` line (`Could not choose a best candidate operator.`), leaving only `You might need to add explicit type casts.` in the `HINT`.
- Lock-timeout and statement-timeout cancellations while waiting for a lock now add `CONTEXT: waiting for AccessShareLock on relation 16384 of database 5` (18 printed no context).

SQLSTATE codes did not change in these cases; match on SQLSTATE, not on message text.

***

#### What's new in PostgreSQL 19 (overview)

Release-note highlights plus the incompatibilities a reviewer must know.[^1]

| Area | Change | Section |
| --- | --- | --- |
| Maintenance | `REPACK` replaces `VACUUM FULL`/`CLUSTER`; `REPACK (CONCURRENTLY)` rewrites a table while reads and writes continue | REPACK |
| DML | `INSERT ... ON CONFLICT (...) DO SELECT [FOR UPDATE/SHARE] ... RETURNING` (atomic get-or-create) | ON CONFLICT DO SELECT |
| Queries | `IGNORE NULLS` / `RESPECT NULLS` for `lag`, `lead`, `first_value`, `last_value`, `nth_value` | IGNORE NULLS |
| COPY | `COPY TO ... (FORMAT json [, FORCE_ARRAY])`; `ON_ERROR set_null`; `HEADER n` on input; `COPY partitioned_table TO`; system columns rejected in `COPY FROM ... WHERE` | COPY |
| Replication | `WAIT FOR LSN` (read-your-writes on standbys); logical decoding with `wal_level = replica` (no restart); sequence synchronization; `FOR ALL SEQUENCES`; `FOR ALL TABLES EXCEPT (...)`; `retain_dead_tuples` | Replication |
| Types/functions | `oid8`; `uuid` ↔ `bytea` casts; `regdatabase`; `random(date/timestamp/timestamptz, ...)`; `encode/decode` `base64url`, `base32hex`; `error_on_null()`; `range_minus_multi()`; jsonpath string methods; `tid_block()`/`tid_offset()`; `IS JSON` on domains over `json`/`jsonb`/`bytea`; Polish and Esperanto stemmers | Types and functions |
| Behaviour | `json_array(SELECT ...)` over zero rows returns `[]` (18: `NULL`) | Types and functions |
| DDL | `ALTER CONSTRAINT ... [NOT] ENFORCED` for `CHECK`; `SET EXPRESSION` on virtual columns with `CHECK`; extended statistics on virtual columns; `GRANT/REVOKE ... GRANTED BY` another role; forward FK references in `CREATE SCHEMA`; `CHECKPOINT (MODE ..., FLUSH_UNLOGGED)`; list settings can be set to `NULL` | DDL |
| Planner | `NOT IN` → anti join when no `NULL`s are possible; eager aggregation (`enable_eager_aggregate`); more `LEFT JOIN` → anti join; Memoize for anti joins; faster FK checks; scans set all-visible bits | Planner |
| EXPLAIN | `EXPLAIN (ANALYZE, IO)`; WAL full-page-image bytes; Memoize estimates; `PLAN_ADVICE` option with `pg_plan_advice` | Planner |
| Contrib | `pg_plan_advice` (plan-advice mini-language), `pg_stash_advice` (per-query-id advice); `pg_stat_statements` generic/custom plan counts | Planner |
| Defaults | `jit = off`; `max_locks_per_transaction = 128`; `default_toast_compression = lz4`; `log_lock_waits = on`; `io_workers` replaced by `io_min_workers`/`io_max_workers` (dynamic) | Defaults |
| Autovacuum | Parallel index vacuuming (`autovacuum_max_parallel_workers`, `autovacuum_parallel_workers`); scoring-based table ordering (`pg_stat_autovacuum_scores`) | Defaults |
| Security | `standard_conforming_strings` forced `on`; RADIUS removed; MD5 login warning; password-expiry warning; CR/LF banned in database, role and tablespace names; `postgres_fdw` inherits `READ ONLY`; server-side SNI (`pg_hosts.conf`) | Security |
| Removed | `MULE_INTERNAL` encoding; `escape_string_warning`; `io_workers`; btree_gist as default `inet`/`cidr` GiST opclass | Migration notes |
| Monitoring | `pg_stat_lock`, `pg_stat_recovery`, `pg_stat_autovacuum_scores`, `pg_stat_progress_repack`, `stats_reset` columns, `log_min_messages` per process type, `log_autoanalyze_min_duration`; wait-event type `BufferPin` renamed `Buffer`; `pg_stat_subscription_stats.sync_error_count` renamed | Observability |

Smaller release-note items that are not covered in their own section:[^1]

- **Encodings and Unicode**: the client-side `GB18030` encoding is updated from the 2000 to the 2022 standard, so conversions of a few code points change for `client_encoding = GB18030` applications; Unicode data is updated to 17.0 (`SELECT unicode_version()` → `17.0` on 19rc1), which affects case mapping and normalization only for characters newly assigned since 16.0.
- **Queries**: `GROUP BY` handles target-list subqueries that reference non-grouped outer columns, and `GROUPING()` handles target-list subquery aliases correctly.
- **Performance without SQL changes**: `NOTIFY` wakes only backends listening on that channel; TID range scans (`WHERE ctid >= ... AND ctid < ...`) can run in parallel; radix sort; faster row deformation; `SELECT simple-expression INTO var` in PL/pgSQL is optimized.
- **PL/Python** can implement event triggers.
- **psql**: prompt escapes `%S` (current `search_path`, needs an 18+ server) and `%i` (hot-standby status); `\pset display_true`/`display_false`; comments shown by `\dRp+`, `\dRs+`, `\dX+`; variable `SERVICEFILE`.
- **Tools**: `pgbench --continue-on-error`; `vacuumdb --dry-run`; `pg_waldump` and `pg_verifybackup` read WAL from tar archives; `pg_upgrade` handles non-default tablespaces inside `PGDATA`; `pg_createsubscriber --logdir` and ignoring existing publications.
- `pg_stat_wal_receiver.status` can be `connecting`; new setting `debug_print_raw_parse`; `file_fdw` reads multi-line headers; `btree_gin` supports all B-tree cross-type comparisons.

***

#### New in 19: `REPACK` (replaces `VACUUM FULL` / `CLUSTER`)

`REPACK` rewrites a table (and all its indexes) into new files without dead tuples, returning the space to the operating system. With `USING INDEX` it also clusters the rows. `VACUUM FULL` and `CLUSTER` still work but are superseded.[^1][^6]

```sql
REPACK orders;                                     -- like VACUUM FULL: ACCESS EXCLUSIVE for the whole rewrite
REPACK orders USING INDEX orders_pkey;             -- like CLUSTER orders USING orders_pkey
REPACK orders USING INDEX;                         -- re-cluster on the recorded clustering index
REPACK (VERBOSE, ANALYZE) orders;                  -- ANALYZE afterwards (single non-partitioned table only)
REPACK (CONCURRENTLY) orders;                      -- reads and writes continue; brief ACCESS EXCLUSIVE at the end
```

Verified on 19rc1 (and every form is a syntax error on 18.6):

- Plain `REPACK` holds `AccessExclusiveLock` (plus a `ShareLock`) on the table for the whole rewrite (`pg_locks` inside `BEGIN; REPACK r2; ...`) — the same outage as `VACUUM FULL`.
- `REPACK (CONCURRENTLY)` holds `ShareUpdateExclusiveLock` while copying: in a test on a 750,000-row table, six `INSERT`s issued during the rewrite all succeeded immediately and all six rows were present afterwards. Concurrent changes are captured with logical decoding, which works on the default `wal_level = replica` in 19 (no restart, no permanent slot left behind).
- **The final swap still needs `ACCESS EXCLUSIVE`.** With another session holding a plain `SELECT`'s `AccessShareLock` in an open transaction, `REPACK (CONCURRENTLY)` waited for it, and a concurrent `INSERT` queued **behind** the waiting `REPACK` for about three seconds — the same lock-queue outage as any `ALTER TABLE`. With `SET lock_timeout = '1s'`, `REPACK (CONCURRENTLY)` failed with `canceling statement due to lock timeout` / `CONTEXT: waiting for AccessExclusiveLock on relation ...` **after** doing the copy work.
- `REPACK (CONCURRENTLY)` refuses: tables without a primary key or index-based replica identity (`DETAIL: Relation "rp_nopk" has no identity index.`), unlogged tables (`This operation is only supported for permanent relations.`), and execution inside a transaction block. The documentation also excludes partitioned tables, materialized views, system catalogs, TOAST tables, non-heap access methods and `user_catalog_table` tables.[^6]

Review rules:

- **`REPACK (CONCURRENTLY)` is not MVCC-safe**: "After the repacking transaction commits, the table will appear empty to concurrent transactions, if they are using a snapshot taken before repacking started" (documented). Do not run it while long `REPEATABLE READ`/`SERIALIZABLE` transactions may still read the table.[^6]
- Run it with `lock_timeout` and retry logic, outside peak hours, and only when no long transaction touches the table; otherwise the end-of-run lock request blocks all traffic.
- It needs free disk space for a full copy of the table plus all indexes, and more with `USING INDEX` when the planner sorts. It "might fail to complete if DDL commands are executed on the table by other transactions", and fails if more than about 104 million rows are updated or deleted concurrently.[^6]
- Each concurrent run uses a replication slot; `max_repack_replication_slots` (default 5, restart required) limits how many run at once.
- `REPACK` refuses tables with an invalid index — drop or rebuild failed `CREATE INDEX CONCURRENTLY` leftovers first.
- `REPACK` is a single long transaction for vacuum purposes; it is not a substitute for `VACUUM` against wraparound.[^6]
- Progress is in `pg_stat_progress_repack` (column `command` distinguishes `CLUSTER`/`VACUUM FULL`/`REPACK`); `pg_stat_progress_cluster` still exists.
- Replace third-party `pg_repack` jobs only after testing: the concurrency model (logical decoding, final `ACCESS EXCLUSIVE`) is different.

```sql
-- BAD (19): blocking rewrite on a busy table
VACUUM FULL orders;

-- GOOD (19): online rewrite, give up instead of blocking traffic at the swap
SET lock_timeout = '2s';
REPACK (CONCURRENTLY) orders;            -- outside a transaction block; table needs a PK or replica identity index
```

***

#### New in 19: `INSERT ... ON CONFLICT DO SELECT`

`DO SELECT` returns the existing conflicting row instead of doing nothing or updating it — an atomic "get or create".[^1][^7]

```sql
CREATE TABLE tags (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, name text UNIQUE NOT NULL);

INSERT INTO tags (name) VALUES ('a')
ON CONFLICT (name) DO SELECT
RETURNING id, name;                       -- inserts: 1 | a

INSERT INTO tags (name) VALUES ('a')
ON CONFLICT (name) DO SELECT
RETURNING id, name, old.id AS old_id, new.id AS new_id;   -- existing row: 1 | a | 1 | 1

INSERT INTO tags (name) VALUES ('a')
ON CONFLICT (name) DO SELECT FOR UPDATE   -- also lock the existing row (FOR NO KEY UPDATE / SHARE / KEY SHARE allowed)
RETURNING id;
```

Verified on 19rc1 (all forms are `syntax error at or near "SELECT"` on 18.6):

- `RETURNING` is mandatory (`ON CONFLICT DO SELECT requires a RETURNING clause`), and so is a conflict target (`ON CONFLICT DO SELECT requires inference specification or constraint name`). Exclusion constraints and deferrable constraints cannot be arbiters.
- The command tag counts selected rows (`INSERT 0 1` when the existing row is returned).
- **Concurrency**: while another transaction had inserted `'k'` but not committed, `DO SELECT` waited and then returned that transaction's row (`id = 1`). The same race with `MERGE ... WHEN NOT MATCHED THEN INSERT` failed with `duplicate key value violates unique constraint`. After a concurrent `DELETE` committed, `DO SELECT FOR UPDATE` inserted a new row.
- `DO SELECT WHERE cond` returns nothing (and inserts nothing) for conflicting rows that fail `cond`: `ON CONFLICT (name) DO SELECT WHERE t.name <> excluded.name` gave `INSERT 0 0` and zero rows.
- Unlike `DO UPDATE`, two input rows with the same key in one statement do **not** raise "command cannot affect row a second time": `VALUES ('b'), ('b')` inserted one row and returned it twice.
- `RETURNING old.*` and `new.*` (18) both show the existing row on the `SELECT` path; `old.id IS NULL` still identifies inserted rows.
- **Sequence values are consumed on every attempt**: after the statements above (two rows actually inserted) the identity sequence's `last_value` was 6. Never rely on gap-free ids.
- Privileges: `SELECT` on the table is required; `FOR UPDATE/SHARE` additionally needs `UPDATE` on at least one column.[^7]

```sql
-- BAD (pre-19 idioms): racy, or writes a new row version just to get the id
SELECT id FROM tags WHERE name = $1;                    -- then INSERT if missing: race
INSERT INTO tags (name) VALUES ($1)
ON CONFLICT (name) DO UPDATE SET name = excluded.name   -- dead tuple + triggers fire on every call
RETURNING id;

-- GOOD (19)
INSERT INTO tags (name) VALUES ($1)
ON CONFLICT (name) DO SELECT
RETURNING id;
```

`DO NOTHING RETURNING` still returns nothing for skipped rows; code that "fixes" this with a follow-up `SELECT` can use `DO SELECT` on 19.

***

#### New in 19: `IGNORE NULLS` in window functions

`lag()`, `lead()`, `first_value()`, `last_value()` and `nth_value()` accept the SQL-standard `IGNORE NULLS` / `RESPECT NULLS` (default) clause.[^1][^10]

```sql
SELECT i, x,
       lag(x)        IGNORE NULLS OVER (ORDER BY i) AS prev_non_null,
       last_value(x) IGNORE NULLS OVER (ORDER BY i) AS carried_forward
FROM (VALUES (1,10),(2,NULL),(3,NULL),(4,40)) v(i,x);
--  i | x  | prev_non_null | carried_forward
--  1 | 10 |               | 10
--  2 |    | 10            | 10
--  3 |    | 10            | 10
--  4 | 40 | 10            | 40
```

- This replaces the "count non-nulls, then `first_value` per group" or correlated-subquery workarounds for gap filling (last observation carried forward).
- `last_value()` with the default frame (`RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW`) carries the last non-null value **up to the current row**, which is usually what gap filling wants; with `ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING` it returns the last non-null of the whole partition (verified: `30` on every row for `10, NULL, 30, NULL`; `nth_value(x, 2) IGNORE NULLS` gave `30` as well).
- Only these five functions accept the clause: `sum(x) IGNORE NULLS OVER ()` fails with `aggregate functions do not accept RESPECT/IGNORE NULLS`; `row_number() IGNORE NULLS` with `function row_number does not allow RESPECT/IGNORE NULLS` (verified).
- On 18.6 the clause is `syntax error at or near "NULLS"`.

***

#### New in 19: `COPY` changes

##### JSON output

```sql
CREATE TABLE cj (id int, name text, doc jsonb, n numeric);
INSERT INTO cj VALUES (1, 'a"b', '{"k":1}', 1.50), (2, NULL, 'null', NULL);

COPY cj TO STDOUT (FORMAT json);
-- {"id":1,"name":"a\"b","doc":{"k": 1},"n":1.50}
-- {"id":2,"name":null,"doc":null,"n":null}

COPY cj TO STDOUT (FORMAT json, FORCE_ARRAY);
-- [
--  {"id":1,"name":"a\"b","doc":{"k": 1},"n":1.50}
-- ,{"id":2,"name":null,"doc":null,"n":null}
-- ]

COPY (SELECT id AS ident FROM cj) TO STDOUT (FORMAT json);   -- {"ident":1} ... keys are the query's column aliases
```

Verified on 19rc1 (18.6: `syntax error at or near "FORMAT"` for `FORMAT json`):

- **JSON is output-only**: `COPY cj FROM STDIN (FORMAT json)` → `COPY FORMAT JSON is not supported for COPY FROM`. Use `json_populate_recordset`/`JSON_TABLE` to load JSON.
- SQL `NULL` and a `json`/`jsonb` value that is JSON `null` both become `null` — indistinguishable in the output (row 2 above). Consumers that need the difference must not rely on `COPY ... (FORMAT json)`.[^9]
- `jsonb` columns are embedded as JSON (not as strings); `numeric` keeps its scale (`1.50`). JavaScript consumers lose precision for numbers beyond 2^53.
- `HEADER`, `DELIMITER`, `NULL` and `DEFAULT` are rejected with JSON (`cannot specify HEADER in JSON mode`).
- The default form is newline-delimited JSON (one object per line); `FORCE_ARRAY` produces one JSON array with **leading commas**, as shown.

##### `ON_ERROR set_null`

```sql
CREATE TABLE cn (a int, b int NOT NULL, d date);
COPY cn FROM STDIN (ON_ERROR set_null);
1	2	2025-01-01
x	3	notadate
\.
-- NOTICE:  in 1 row, columns were set to null due to data type incompatibility
-- COPY 2      → row 2 is stored as (NULL, 3, NULL)
```

Verified pitfalls:

- **Bad data is stored as `NULL` silently** (only a summary `NOTICE`). Use it only for columns where `NULL` is an acceptable "unknown"; add `LOG_VERBOSITY verbose` to get one notice per replaced value.
- **Domain `CHECK` violations are also nulled**: a `-5` loaded into a domain `posint AS int CHECK (VALUE > 0)` became `NULL` instead of failing. Domains no longer protect such columns during `COPY ... ON_ERROR set_null`.
- A `NOT NULL` column still fails the whole `COPY` when its value is replaced (`null value in column "b" ... violates not-null constraint`); `set_null` does not skip the row.
- `REJECT_LIMIT` works only with `ON_ERROR ignore`. Both `ignore` and `set_null` are for `text`/`csv` formats only.[^9]
- On 18.6: `COPY ON_ERROR "set_null" not recognized`.

##### Multi-line headers, partitioned tables, `WHERE` restrictions

```sql
COPY ch FROM STDIN (FORMAT csv, HEADER 2);   -- skips two header lines (18: "header requires a Boolean value or "match"")
COPY pt TO STDOUT;                            -- pt is partitioned: copies all partitions (18: "cannot copy from partitioned table")
```

- `HEADER n` with `n > 1` is input-only: `COPY ... TO ... (HEADER 2)` → `cannot use multi-line header in COPY TO`.
- `COPY partitioned TO` copies the same rows as `SELECT * FROM partitioned`; for plain tables `COPY t TO` still means `ONLY t` (inheritance children are not included).[^9]
- **System columns in `COPY FROM ... WHERE` are now rejected**: `WHERE tableoid > 0` loaded rows on 18.6 and fails on 19 with `system columns are not supported in COPY FROM WHERE conditions`. `xmin` was already rejected in 18 (`cannot retrieve a system column in this context`).
- `COPY FROM` for `text` and `csv` uses SIMD and is faster; nothing to change in SQL.[^1]
- `psql` (18.6+ and 19) skips inline data after a scripted `COPY ... FROM STDIN` that fails at startup — **up to the next `\.` line**. A failing `COPY ... FROM STDIN` without data followed by more SQL swallowed the rest of the script in testing. Always terminate inline data with `\.`.

***

#### New in 19: Replication — `WAIT FOR LSN`, logical decoding without restart, sequences, `EXCEPT`

##### Read-your-writes on standbys: `WAIT FOR LSN`

```sql
-- on the primary, after the write transaction committed:
SELECT pg_current_wal_insert_lsn();                 -- e.g. 0/0306EE20, hand it to the reader

-- on the standby, before reading:
WAIT FOR LSN '0/0306EE20' WITH (TIMEOUT '100ms', NO_THROW);   -- status: success | timeout | not in recovery
```

Options are `MODE 'standby_replay'` (default), `'standby_write'`, `'standby_flush'` and `'primary_flush'`, `TIMEOUT '...'` and `NO_THROW`.[^8] Verified on a 19rc1 primary (no standby available in the test environment, so standby modes were not executed):

- On a primary the default mode fails: `ERROR: recovery is not in progress` / `HINT: Waiting for the standby_replay LSN can only be executed during recovery.` With `NO_THROW` it returns `not in recovery` instead.
- `WITH (MODE 'primary_flush')` on an already flushed LSN returned `success`; `WITH (MODE 'primary_flush', TIMEOUT '50ms', NO_THROW)` on an unreachable LSN returned `timeout`.
- `WAIT` must be a top-level statement: inside `DO`/functions it fails (`WAIT can only be executed as a top-level statement`). It cannot run while the transaction holds a snapshot, and on a standby it is rejected when the session already holds locks and the target is not yet reached (deadlock avoidance). Issue it **first**, outside a transaction or as the first statement.[^8]
- **Without `TIMEOUT`, it waits forever.** Application code must set a timeout and handle `timeout` (with `NO_THROW`, check the returned status — it does not raise).
- `WAIT` compares only the numeric LSN, not the timeline; after a failover the status `not in recovery` means "re-evaluate", not "done".[^8]
- On 18.6: `syntax error at or near "WAIT"`.

##### Logical decoding with `wal_level = replica`

```sql
SHOW wal_level;                                                         -- replica (default)
SELECT slot_name FROM pg_create_logical_replication_slot('s1', 'test_decoding');   -- works in 19
SHOW effective_wal_level;                                               -- logical (while a logical slot exists)
SELECT pg_drop_replication_slot('s1');                                  -- effective_wal_level returns to replica
```

Verified: on 18.6 the slot creation fails with `logical decoding requires "wal_level" >= "logical"`. In 19, `effective_wal_level` (read-only) reports what is in force; it switched to `logical` while a logical slot existed and back to `replica` about a second after dropping it. Setting up CDC (Debezium, logical replication, `REPACK (CONCURRENTLY)`) no longer needs a restart, but WAL volume rises while any logical slot exists — and an abandoned slot still retains WAL (`idle_replication_slot_timeout`, 18, default `0` = off).[^1][^17]

##### Other replication changes

- **Sequences**: `CREATE PUBLICATION ... FOR ALL SEQUENCES`; subscribers synchronize sequence values at `CREATE SUBSCRIPTION`, `ALTER SUBSCRIPTION ... REFRESH PUBLICATION` and `ALTER SUBSCRIPTION ... REFRESH SEQUENCES`. This closes the classic "sequences are behind after logical-replication failover" gap — only at those refresh points, not continuously.[^1][^18]
- **`FOR ALL TABLES EXCEPT (TABLE t1, t2)`** (verified: the excluded table is absent from `pg_publication_tables`; 18.6: syntax error).
- **`retain_dead_tuples`** subscription option (with `max_retention_duration`) for `update_deleted` conflict detection; `CREATE SUBSCRIPTION ... SERVER foreign_server` reuses `postgres_fdw` connection parameters; `wal_receiver_timeout` can be set per subscription/role.[^1]
- `pg_stat_subscription_stats.sync_error_count` is renamed `sync_table_error_count`; `sync_seq_error_count` and `confl_update_deleted` are new (verified in the catalog).
- `wal_sender_shutdown_timeout` (default `-1`: senders still wait forever at shutdown).
- `pg_sync_replication_slots()` now waits for synchronization to complete.[^1]

***

#### New in 19: Data types and functions

All verified on 19rc1; each fails on 18.6 (`function ... does not exist`, `type "oid8" does not exist`, `cannot cast type uuid to bytea`, `unrecognized encoding: "base64url"`).

```sql
SELECT random('2025-01-01'::date, '2025-01-31'::date);         -- random date in the range (18: function random(date, date) does not exist)
SELECT random('2025-01-01'::timestamptz, '2025-01-02'::timestamptz);   -- also timestamp

SELECT error_on_null(42);                                       -- 42
SELECT error_on_null(NULL::int);                                -- ERROR:  null value not allowed

SELECT encode('\xfbff'::bytea, 'base64'), encode('\xfbff'::bytea, 'base64url');   -- +/8= | -_8
SELECT decode('-_8', 'base64url');                              -- \xfbff
SELECT encode('hello'::bytea, 'base32hex');                     -- D1IMOR3F (order-preserving)

SELECT 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'::uuid::bytea;     -- \xa0eebc99...  (18: cannot cast type uuid to bytea)
SELECT '\xa0eebc999c0b4ef8bb6d6bb9bd380a11'::bytea::uuid;       -- a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11

SELECT range_minus_multi('[1,10)'::int4range, '[3,5)'::int4range);        -- two rows: [1,3) and [5,10)
SELECT multirange_minus_multi('{[1,10)}'::int4multirange, '{[3,5)}'::int4multirange);  -- {[1,3),[5,10)}

SELECT '18446744073709551615'::oid8, pg_column_size('1'::oid8); -- unsigned 64-bit, 8 bytes
SELECT tid_block('(42,7)'::tid), tid_offset('(42,7)'::tid);     -- 42 | 7
SELECT 'postgres'::regdatabase, to_regdatabase('nope');         -- postgres | NULL
```

Pitfalls found while testing:

- `'\x01'::bytea::uuid` fails (`invalid input length for type uuid`, `DETAIL: Expected 16 bytes, got 1.`). The cast is a 16-byte binary reinterpretation, **not** a text parse: `'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'::text::bytea::uuid` fails (`Expected 16 bytes, got 36.`, SQLSTATE `22P03`). Use it for binary UUID columns migrated from other databases, and keep `uuid` as the storage type.
- There is **no `'base32'`** encoding — only `base32hex`[^20]; the error hint lists `"base32hex", "base64", "base64url", "escape", and "hex"`. `base64url` output omits padding (`-_8`, not `-_8=`).
- `'-1'::oid8` is accepted and yields `18446744073709551615` (wrap-around like `oid`); do not use `oid8` as a general unsigned integer for user data — use `bigint`/`numeric` with a `CHECK`.
- `error_on_null()` raises SQLSTATE `22004` (`null value not allowed`); it asserts non-null values inline, e.g. `SELECT error_on_null((SELECT id FROM t WHERE ...))` in a SQL-language function body.
- The `-` operator on ranges still fails when the result would have a gap (`result of range difference would not be contiguous`); `range_minus_multi()` returns a set of ranges instead (zero rows when nothing remains).[^21]

##### jsonpath string methods

```sql
SELECT jsonb_path_query('"  Hello World "', '$.btrim().lower()');           -- "hello world"
SELECT jsonb_path_query('"a-b-c"', '$.split_part("-", 2)');                 -- "b"
SELECT jsonb_path_query('"aXa"', '$.replace("a","b")');                     -- "bXb"
SELECT jsonb_path_exists('{"e":"Foo@X.com"}', '$.e ? (@.lower() == "foo@x.com")');   -- t
SELECT jsonb_path_query('42', '$.lower()');
-- ERROR:  jsonpath item method .lower() can only be applied to a string
```

New methods: `.ltrim()`, `.rtrim()`, `.btrim()`, `.lower()`, `.upper()`, `.initcap()`, `.replace()`, `.split_part()` — immutable like their SQL counterparts (release notes). On 18.6 every one is `syntax error at or near "(" of jsonpath input`. In non-`silent` mode they raise on non-string items; filter with `? (@.type() == "string")` or pass `silent => true` where data is heterogeneous.[^19]

##### SQL/JSON behaviour changes

```sql
SELECT json_array(SELECT 1 WHERE false);          -- 19: []      18: NULL
SELECT '{"a":1}'::my_jsonb_domain IS JSON OBJECT; -- 19: t       18: cannot use type my_jsonb_domain in IS JSON predicate
```

- **`json_array(query)` over zero rows returns an empty array** (`[]`, also with `RETURNING jsonb`) instead of `NULL` (verified both versions). Code that did `coalesce(json_array(SELECT ...), '[]')` keeps working; code that tested `IS NULL` to detect "no rows" now silently takes the other branch.
- `IS JSON` accepts domains over `json`, `jsonb` and `bytea` (verified: fail on 18.6). Domains over `text` already worked on 18.6. Domains over other types are still rejected (`cannot use type dint in IS JSON predicate`).
- `JSON_SERIALIZE()` on a `jsonb` argument still returns garbage on 19rc1 (`JSON_SERIALIZE('{"a":1}'::jsonb)` → `\x01`); use `doc::text`.

##### Text search

New built-in configurations `polish` and `esperanto` (with `polish_stem`/`esperanto_stem` dictionaries; 18: `text search configuration "polish" does not exist`). The Dutch stemmer behind the `dutch` configuration and `dutch_stem` was updated. The old algorithm is not a configuration (`to_tsvector('dutch_porter', ...)` fails with `text search configuration "dutch_porter" does not exist`) but a Snowball language: `CREATE TEXT SEARCH DICTIONARY dutch_old (TEMPLATE = snowball, LANGUAGE = dutch_porter)` (verified). After upgrading, rebuild Dutch full-text indexes and stored `tsvector` columns, because stems can differ — or build a configuration on such a dictionary to keep the old stems.[^1]

***

#### New in 19: Constraints, DDL and utility commands

##### `CHECK` constraints can change enforceability

```sql
CREATE TABLE ck (a int, CONSTRAINT ck_pos CHECK (a > 0));
ALTER TABLE ck ALTER CONSTRAINT ck_pos NOT ENFORCED;    -- 18: cannot alter enforceability of constraint "ck_pos"
INSERT INTO ck VALUES (-1);                              -- accepted
-- pg_constraint: conenforced = f, convalidated = f
ALTER TABLE ck ALTER CONSTRAINT ck_pos ENFORCED;
-- ERROR:  check constraint "ck_pos" of relation "ck" is violated by some row
```

`ALTER CONSTRAINT ... ENFORCED` scans the whole table while holding **`AccessExclusiveLock`** (verified with `pg_locks`) and fails on the first violating row. For large tables prefer `DROP CONSTRAINT` + `ADD CONSTRAINT ... NOT VALID` + `VALIDATE CONSTRAINT` (`SHARE UPDATE EXCLUSIVE`). Flag `NOT ENFORCED` used to "speed up a bulk load" — the constraint is simply off for every session until re-enabled.

##### Virtual generated columns (18) get fewer restrictions

- `ALTER TABLE ... ALTER COLUMN b SET EXPRESSION AS (...)` succeeds on a virtual column that has a `CHECK` constraint (18: `ALTER TABLE / SET EXPRESSION is not supported for virtual generated columns in tables with check constraints`).
- `CREATE STATISTICS` accepts virtual generated columns (18: `statistics creation on virtual generated columns is not supported`).
- Still not allowed in 19 (verified): indexes on virtual columns, user-defined functions in the expression (`generation expression uses user-defined function`).

##### `GRANT` / `REVOKE ... GRANTED BY`

```sql
GRANT SELECT ON gt TO greader GRANTED BY gowner;   -- 18: ERROR: grantor must be current user
-- relacl: {gowner=arwdDxtm/gowner,greader=r/gowner}
REVOKE SELECT ON gt FROM greader GRANTED BY gowner;
```

Migration and admin scripts can now record the owner as grantor, so `REVOKE` by the owner later works. Review that the executing role really should act on behalf of the named role.

##### Other DDL and utility changes (verified)

- **`CREATE SCHEMA ... CREATE TABLE child (... REFERENCES parent) CREATE TABLE parent (...)`** works with forward references (18: `relation "parent" does not exist`).
- **`CHECKPOINT (MODE FAST)`** (the default behaviour), `CHECKPOINT (MODE SPREAD, FLUSH_UNLOGGED)` (18: syntax error). `MODE SPREAD` spreads the writes like a scheduled checkpoint, which "can reduce the rate of I/O"; `FLUSH_UNLOGGED` also flushes unlogged tables. `CHECKPOINT` needs superuser or `pg_checkpoint`.
- **List-valued settings can be emptied with `NULL`**: `SET search_path = NULL` gives an empty `search_path` (18: syntax error). An empty `search_path` means every unqualified name fails to resolve — treat it like `search_path = ''`.
- **`log_min_messages` per process type**: `SET log_min_messages = 'warning, checkpointer:debug1'` (18: invalid value).
- `CREATE AGGREGATE ... SUPPORT`, `CREATE FOREIGN DATA WRAPPER ... CONNECTION` (extension authors).[^1]
- `VACUUM (VERBOSE)` and autovacuum logs report memory usage and parallelism.

***

#### New in 19: Planner, `EXPLAIN`, plan advice

##### Optimizer improvements (verified plans, 19rc1 vs 18.6)

```sql
-- a1.id and b1.a_id are NOT NULL
EXPLAIN (COSTS OFF) SELECT * FROM a1 WHERE id NOT IN (SELECT a_id FROM b1);
--  19: Hash Anti Join  (Hash Cond: (a1.id = b1.a_id))
--  18: Seq Scan on a1, Filter: (NOT (ANY (id = (hashed SubPlan 1).col1)))

-- b1.n is nullable: 19 keeps the hashed SubPlan (NOT IN semantics with NULL)
EXPLAIN (COSTS OFF) SELECT * FROM a1 WHERE id NOT IN (SELECT n FROM b1);

-- Eager aggregation: aggregate the big side before the join
EXPLAIN (COSTS OFF) SELECT d.name, sum(f.v) FROM fact f JOIN dim d ON d.id = f.dim_id GROUP BY d.name;
--  19: Finalize GroupAggregate -> Sort -> Hash Join -> Partial HashAggregate (Group Key: f.dim_id) on fact
--  18: HashAggregate -> Hash Join -> Seq Scan on fact (200,000 rows into the join)
```

- **`NOT IN` is still a correctness trap**: with a nullable subquery column it still returns no rows when a `NULL` is present. The anti-join conversion only happens when the planner can prove there are no `NULL`s (`NOT NULL` columns). Keep recommending `NOT EXISTS`; do recommend declaring `NOT NULL` where it is true.
- Eager aggregation is controlled by `enable_eager_aggregate` (default `on`) and `min_eager_agg_group_size` (default 8). Plan-regression tests that pin plan shapes will change.
- Also new: more `LEFT JOIN ... WHERE right IS NULL` → anti join, Memoize for anti joins with unique inner sides, constant folding of `IS [NOT] DISTINCT FROM NULL` and `IS [NOT] NULL` on provably non-null expressions, better semijoin planning, faster join selectivity with large statistics targets, startup costs for partial paths, faster foreign-key checks, radix sort.[^1]
- **Plain scans set visibility-map bits**: after `INSERT` of 50,000 rows into a table with autovacuum off, two `SELECT count(*)` scans changed `pg_visibility_map_summary()` from `0` all-visible pages to `221` on 19 (still `0` on 18.6). Index-only scans become effective sooner after bulk loads; `VACUUM` is still required for freezing and dead-tuple removal.

##### `EXPLAIN (ANALYZE, IO)`

```sql
EXPLAIN (ANALYZE, IO, COSTS OFF, TIMING OFF, SUMMARY OFF) SELECT count(*) FROM big;   -- cold cache
--  Seq Scan on big (actual rows=300000.00 loops=1)
--    Prefetch: avg=78.77 max=89 capacity=94
--    I/O: count=164 waits=10 size=15.61 in-progress=4.77
--    Buffers: shared hit=2499 read=2560 dirtied=2501 written=4
```

- `IO` requires `ANALYZE` (`EXPLAIN option IO requires ANALYZE`); 18.6: `unrecognized EXPLAIN option "io"`. `auto_explain.log_io` adds it to `auto_explain`.[^1][^13]
- The documented fields are the prefetch queue (average/maximum distance, capacity) and the issued I/O requests (`count`, average `size` in blocks, number of I/O `waits`, average number of concurrent requests). In text format only non-zero values are printed. Many `waits` relative to `count` on a hot query suggest read-ahead did not keep up (check `effective_io_concurrency`, `io_max_workers`, storage latency).[^13]
- `EXPLAIN (ANALYZE, WAL)` also reports full-page-image bytes; Memoize nodes show cache estimates.

##### `pg_plan_advice` and `pg_stash_advice` (contrib)

`pg_plan_advice` describes and constrains planner decisions with a "plan advice" mini-language; it is loaded with `LOAD 'pg_plan_advice'`, `session_preload_libraries` or `shared_preload_libraries` (no `CREATE EXTENSION`).[^11]

```sql
EXPLAIN (COSTS OFF, PLAN_ADVICE)
SELECT * FROM join_fact f JOIN join_dim d ON f.dim_id = d.id WHERE d.id < 5;
--  Hash Join ...
--  Generated Plan Advice:
--    JOIN_ORDER(f d)
--    HASH_JOIN(d)
--    SEQ_SCAN(f)
--    INDEX_SCAN(d public.join_dim_pkey)
--    NO_GATHER(f d)

SET pg_plan_advice.advice = 'NESTED_LOOP_PLAIN(f)';
EXPLAIN (COSTS OFF) SELECT ...;     -- Nested Loop ... Supplied Plan Advice: NESTED_LOOP_PLAIN(f) /* matched */

SET pg_plan_advice.advice = 'JOIN_ORDER(x f d)';   -- x is not in the query
EXPLAIN (COSTS OFF) SELECT ...;     -- Nested Loop, "Disabled: true", JOIN_ORDER(x f d) /* partially matched */
```

Review notes (verified on 19rc1 unless stated):

- Advice only **constrains** the planner. A string that does not parse is rejected when it is set (`SET pg_plan_advice.advice = 'BOGUS_TAG(f)'` → `ERROR: invalid value for parameter "pg_plan_advice.advice"` / `DETAIL: Could not parse advice: syntax error at or near "BOGUS_TAG"`), but well-formed advice that names unknown aliases or indexes, or asks for something impossible, never fails planning: it is reported as `/* not matched */`, `/* partially matched */`, `/* inapplicable */`, `/* conflicting */` or `/* failed */` and can yield a plan with `Disabled: true` nodes that is much worse than no advice. Every stored advice string needs a test that checks the feedback is `/* matched */`; `pg_plan_advice.feedback_warnings = on` (default `off`) turns unenforced advice into warnings.[^11]
- **On 18 (or when the module is not loaded) `SET pg_plan_advice.advice = '...'` succeeds silently** (custom placeholder setting) and does nothing; `EXPLAIN (PLAN_ADVICE)` fails with `unrecognized EXPLAIN option "plan_advice"`.
- Advice cannot control aggregation strategy or set operations (documented limitation).[^11]
- `pg_stash_advice` (`CREATE EXTENSION pg_stash_advice`, module preloaded) maps query ids to advice strings in shared memory (`pg_create_advice_stash()`, `pg_set_stashed_advice()`, setting `pg_stash_advice.stash_name`). Any user can set the stash name for their session and may see stashed advice, so do not embed sensitive values in advice strings.[^12]
- Prefer fixing statistics (`ANALYZE`, `CREATE STATISTICS`, now restorable with `pg_restore_extended_stats()` and removable with `pg_clear_extended_stats()`) over pinning plans. Pinned plans do not adapt to data growth.

##### `pg_stat_statements` 1.13

New `generic_plan_calls` and `custom_plan_calls` columns (verified: a prepared statement run twice with `force_generic_plan` and once with `force_custom_plan` showed `2 | 1`); `FETCH` sizes are normalized to constants. Constant lists in `IN (...)`/`= ANY(ARRAY[...])` are still squashed into one entry (`IN ($1 /*, ... */)`, 18 behaviour, re-verified). Run `ALTER EXTENSION pg_stat_statements UPDATE` after `pg_upgrade`.[^22]

***

#### New in 19: I/O, autovacuum and changed defaults

##### Defaults that changed (verified with `SHOW` on 19rc1 vs 18.6)

| Setting | 18 | 19 | Review impact |
| --- | --- | --- | --- |
| `jit` | `on` | **`off`** | Large analytical queries that benefited from JIT get slower unless `jit = on` is set explicitly (per role/database) |
| `max_locks_per_transaction` | 64 | **128** | Lock memory allocation changed; existing explicit settings must be **doubled** to keep the same capacity |
| `default_toast_compression` | `pglz` | **`lz4`** | New TOASTed values compress with LZ4 (verified with `pg_column_compression()`); existing values keep their method |
| `log_lock_waits` | `off` | **`on`** | Lock waits longer than `deadlock_timeout` (1s) are logged; expect more log volume |
| `io_workers` | 3 | **removed** | Replaced by `io_min_workers` (2) and `io_max_workers` (8); the pool scales automatically |
| `wal_compression = on` | pglz | best available method | |
| xid/multixact wraparound warnings | at 40 million | at 100 million remaining | |

**`io_workers` in a configuration file prevents startup** (verified): `postgresql.conf` with `io_workers = 4` → `LOG: unrecognized configuration parameter "io_workers" in file ... line 934` and `FATAL: configuration file ".../postgresql.conf" contains errors`; on the command line (`-c io_workers=4`) the server exits with `FATAL: unrecognized configuration parameter "io_workers"`. The same happens for `escape_string_warning`, and `standard_conforming_strings = off` is fatal at startup too (`FATAL: non-standard string literals are not supported`). Remove these lines from Helm values, Ansible templates and Docker `command:` arguments **before** upgrading. Also note that the Docker image's data directory is `/var/lib/postgresql/19/docker` (the per-major layout introduced with the 18 images).

##### Asynchronous I/O (18) in 19

`io_method` (default `worker`; `io_uring`, `sync`) is unchanged. In 19 the worker pool is dynamic: `io_min_workers` (2), `io_max_workers` (8), `io_worker_idle_timeout` (60s) and `io_worker_launch_interval` (100ms). Read-ahead scheduling for large requests is improved, and hash-index bulk deletion, GIN vacuuming, `bloom` and `pgstattuple` use streaming reads.[^1][^16] As in 18, AIO is a server feature — do not attribute query regressions to it without `EXPLAIN (ANALYZE, IO)` evidence.

##### Autovacuum

- **Parallel index vacuuming in autovacuum**: `autovacuum_max_parallel_workers` (default `0` = off) and the per-table storage parameter `autovacuum_parallel_workers` (verified: `ALTER TABLE b1 SET (autovacuum_parallel_workers = 2)` works on 19, `unrecognized parameter` on 18.6).
- **Scoring**: autovacuum processes tables in order of a score; weights `autovacuum_freeze_score_weight`, `autovacuum_multixact_freeze_score_weight`, `autovacuum_vacuum_score_weight`, `autovacuum_vacuum_insert_score_weight`, `autovacuum_analyze_score_weight` (all default 1). `pg_stat_autovacuum_scores` shows each table's scores and whether it is due.
- `log_autovacuum_min_duration` now covers only vacuum; autoanalyze logging uses the new `log_autoanalyze_min_duration` (default 10min). Monitoring that relied on autoanalyze lines must set the new parameter.
- `vacuumdb --analyze-only` / `--analyze-in-stages` without targets now analyze partitioned parents (verified with `--dry-run`, new in 19: `ANALYZE ONLY public.pt;` appears; 18.6's `--echo` output had no parent).[^1]

***

#### New in 19: Security, authentication and string literals

##### `standard_conforming_strings` is always `on`

```sql
SET standard_conforming_strings = off;
-- 19: ERROR:  non-standard string literals are not supported
-- 18: SET
```

This is a **silent data change** for old dumps and scripts: a file that starts with `SET standard_conforming_strings = off;` and then inserts `'C:\\temp\\new'` stored `C:\temp\new` (11 characters) on 18.6. On 19 the `SET` fails, psql continues, and the same `INSERT` stores `C:\\temp\\new` (13 characters). Restore dumps with 19's `pg_dump`/`pg_restore`, run scripts with `ON_ERROR_STOP`, and use `E'...'` for backslash escapes. `escape_string_warning` is gone (`unrecognized configuration parameter`). Client libraries still support old servers with the setting off.[^1]

##### Authentication

- **RADIUS removed**: a `pg_hba.conf` line with `radius` shows `invalid authentication method "radius"` in `pg_hba_file_rules.error`, and a reload logs `pg_hba.conf was not reloaded` (verified) — the whole file is rejected. Migrate to LDAP, certificates, OAuth or SCRAM before upgrading.
- **MD5**: successful MD5 logins emit `WARNING: authenticated with an MD5-encrypted password` / `DETAIL: MD5 password support is deprecated and will be removed in a future release of PostgreSQL.` (verified; 18.6 warns only when setting the password). Applications that treat any server warning as an error, or log it per connection, will notice.
- **Password expiry warning**: `password_expiration_warning_threshold` (default 7 days) — a SCRAM login of a role whose `VALID UNTIL` is less than the threshold away produced `WARNING: role password will expire soon` / `DETAIL: The password for role "exp" will expire in 1 day.` (verified; nothing on 18.6). `0` disables it.[^15]
- **OAuth** (18): validators can register custom `pg_hba.conf` options and supply failure details; libpq gains `oauth_ca_file` (`PGOAUTHCAFILE`) and a v2 bearer-token hook.[^1]
- **Server-side SNI**: `ssl_sni` (default `off`) with host/key pairs in `pg_hosts.conf` (setting `hosts_file`).
- **CR/LF in names**: `CREATE ROLE "bad<newline>name"` → `role name "..." contains a newline or carriage return character` (verified; 18.6 accepted it). The same applies to database and tablespace names, and `pg_upgrade` refuses clusters that contain such names.[^1]
- **`pg_read_all_data` / `pg_write_all_data`** now cover large objects.

##### `postgres_fdw` sessions inherit `READ ONLY` / `DEFERRABLE`

Verified with a loopback server: in `BEGIN READ ONLY`, `SELECT * FROM foreign_view` where the remote view calls a function that inserts into an audit table succeeded on 18.6 (the remote row was written) and fails on 19 with `cannot execute INSERT in a read-only transaction` (`remote SQL command: SELECT x FROM public.rv`). Read-only reporting transactions that relied on side effects on the remote server break; that is the intended fix.[^1]

`ANALYZE` on `postgres_fdw` foreign tables can import remote statistics with the `import_stats` option instead of sampling rows; it runs as the foreign table owner.[^1][^3]

***

#### Features from 18 that a PostgreSQL 19 codebase uses

Most code reviewed for 19 is written with 18 idioms. These 18 features are available in 19 unchanged unless noted (behaviour re-verified on 19rc1).[^25]

##### `uuidv7()` (18)[^45]

```sql
CREATE TABLE orders (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),    -- time-ordered: right-edge B-tree inserts like bigint identity
    created_at timestamptz NOT NULL DEFAULT now()
);
SELECT uuid_extract_version(uuidv7()), uuid_extract_timestamp(uuidv7());   -- 7 | creation time (functions are 17+)
SELECT uuidv7(interval '-1 hour');                                          -- shifted timestamp
```

- `uuidv7()` exposes the creation time to anyone holding the id; use `gen_random_uuid()` (alias `uuidv4()`, 18) where that matters.
- Store as `uuid` (16 bytes), never as text (`pg_column_size(uuidv7()::text)` is 40).
- `uuidv7()` is volatile: `ALTER TABLE ... ADD COLUMN c uuid DEFAULT uuidv7()` **rewrites the table** (verified with `pg_relation_filenode` on 19rc1), exactly like `gen_random_uuid()`. Add the column without a default, then `SET DEFAULT`, then backfill in batches.
- 19 adds `uuid` ↔ `bytea` casts; `min()/max()` on `uuid` are **not** in 19 (20devel).

##### Virtual generated columns are the default (18)

`GENERATED ALWAYS AS (expr)` without `STORED` creates a **virtual** column (computed on read). Verified on 19rc1: an index on it fails (`indexes on virtual generated columns are not supported`) and a user-defined function in the expression fails (`Virtual generated columns that make use of user-defined functions are not yet supported`). Adding one is metadata-only; adding a `STORED` one rewrites the table. Virtual columns cannot be part of a `UNIQUE`/`PRIMARY KEY` (`unique constraints on virtual generated columns are not supported`), a `FOREIGN KEY` or a partition key, and cannot be published (`cannot use virtual generated column "b" in publication column list`) — all verified on 19rc1.[^46]

```sql
-- BAD: meant to be indexed, but the omitted keyword made it virtual
ALTER TABLE users ADD COLUMN email_norm text GENERATED ALWAYS AS (lower(email));
CREATE INDEX ON users (email_norm);   -- ERROR: indexes on virtual generated columns are not supported

-- GOOD: index the expression directly (no rewrite), or write STORED explicitly (rewrite: plan it)
CREATE INDEX users_email_lower ON users (lower(email));
```

19 relaxes two restrictions (extended statistics on virtual columns; `SET EXPRESSION` with `CHECK` constraints), see "Constraints, DDL and utility commands".

##### `RETURNING old.*` / `new.*` (18)

```sql
UPDATE acct SET bal = bal - 30 WHERE id = 1 RETURNING old.bal AS before, new.bal AS after;   -- 100 | 70
INSERT INTO acct VALUES (1, 5) ON CONFLICT (id) DO UPDATE SET bal = excluded.bal
RETURNING old.bal, new.bal, (old.id IS NULL) AS inserted;                                     -- 70 | 5 | f
UPDATE acct SET bal = 1 RETURNING WITH (OLD AS o, NEW AS n) o.bal, n.bal;                    -- rename on clashes
```

Replace `RETURNING (xmax = 0) AS inserted` hacks with `old.<pk> IS NULL`; test a non-nullable column, not `old IS NULL`. Works for `INSERT`, `UPDATE`, `DELETE`, `MERGE` and, in 19, `ON CONFLICT DO SELECT`.[^47]

##### Temporal constraints (18)

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;     -- for the scalar (=) part of the GiST index
CREATE TABLE room (id int, valid tstzrange, PRIMARY KEY (id, valid WITHOUT OVERLAPS));
INSERT INTO room VALUES (1, '[2025-01-01,2025-02-01)');
INSERT INTO room VALUES (1, '[2025-01-15,2025-03-01)');
-- ERROR:  conflicting key value violates exclusion constraint "room_pkey"
CREATE TABLE booking (bid int, room_id int, during tstzrange,
    FOREIGN KEY (room_id, PERIOD during) REFERENCES room (id, PERIOD valid));
```

The constraint is a GiST exclusion constraint. There is still no `UPDATE ... FOR PORTION OF` in 19; splitting periods stays application work. 19's `range_minus_multi()` helps compute the remaining pieces.[^48]

##### `NOT NULL ... NOT VALID`, `NOT ENFORCED` (18)

```sql
ALTER TABLE child ADD CONSTRAINT child_id_nn NOT NULL id NOT VALID;   -- brief ACCESS EXCLUSIVE; new rows checked
ALTER TABLE child VALIDATE CONSTRAINT child_id_nn;                    -- SHARE UPDATE EXCLUSIVE scan; attnotnull = t afterwards
```

- `NOT NULL` constraints live in `pg_constraint` (`contype = 'n'`) with names; adding a second one under another name fails (`A not-null constraint named "child_id_nn" already exists for this column.`, verified on 19rc1).
- `NOT ENFORCED` (18) is accepted for `CHECK` and foreign keys only. In 18 only FKs could be switched with `ALTER CONSTRAINT ... [NOT] ENFORCED`; 19 adds `CHECK`. Re-enforcing scans the table under `ACCESS EXCLUSIVE`.[^14]

##### Planner and `EXPLAIN` (18)

Re-verified on 19rc1 (100,000-row table):

```sql
CREATE INDEX u_org_created ON u (org, created_at);      -- org has 100 distinct values
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, SUMMARY OFF, BUFFERS OFF)
SELECT * FROM u WHERE created_at = '2025-01-10 00:00+00';
--  Index Scan using u_org_created on u (actual rows=1.00 loops=1)
--    Index Cond: (created_at = '2025-01-10 00:00:00+00'::timestamp with time zone)
--    Index Searches: 101                               -- B-tree skip scan (18)

EXPLAIN (COSTS OFF) SELECT * FROM u WHERE id = 1 OR id = 2 OR id = 3;
--  Index Scan using u_pkey on u  Index Cond: (id = ANY ('{1,2,3}'::integer[]))
EXPLAIN (COSTS OFF) SELECT a.id, b.email FROM u a JOIN u b ON a.id = b.id WHERE a.org = 5;
--  Bitmap Heap Scan on u b ...                         -- self-join eliminated
```

- Skip scan helps only with a **low-cardinality** leading column; with 200,000 distinct leading values the planner chose a `Seq Scan` (verified on 19rc1).
- `EXPLAIN ANALYZE` prints `Buffers:` by default and fractional row counts (`rows=1.00`); plan parsers that match `rows=(\d+)` break.
- Do not ask authors to rewrite `OR` chains into `= ANY(...)` or to remove redundant self-joins for performance — the planner does it.[^49]

##### Other 18 features in daily use

| Feature (18) | Note |
| --- | --- |
| Asynchronous I/O (`io_method`), `effective_io_concurrency`/`maintenance_io_concurrency` default 16 | Remove stale `effective_io_concurrency = 1` overrides |
| `casefold()`, `PG_UNICODE_FAST` collation | `casefold('Straße' COLLATE pg_unicode_fast)` → `strasse`; with the Docker image's libc default collation → `straße` (verified on 19rc1) |
| `array_sort()`, `array_reverse()`, `crc32()`, `crc32c()`, `reverse(bytea)`, integer↔`bytea` casts | |
| `jsonb` `null` → SQL `NULL` on scalar casts | `('{"a":null}'::jsonb -> 'a')::int` is `NULL`, not an error |
| `COPY ... REJECT_LIMIT`, `LOG_VERBOSITY silent`; CSV `COPY FROM` no longer treats `\.` as end-of-data | |
| `MIN`/`MAX` on composite types | `max(ROW(1,2))` |
| OAuth authentication (`oauth` in `pg_hba.conf`) | Not executable in the test environment |
| `AFTER` triggers run as the role that queued the event | Verified on 19rc1: a deferred trigger queued as `app` recorded `app` although the session switched role before `COMMIT` |
| Data checksums on by default in `initdb` | `data_checksums = on` in the 19 Docker image |
| `pg_upgrade` keeps planner statistics; `--swap` | 18's `pg_upgrade` skipped `CREATE STATISTICS` statistics; 19's `pg_dump` can include extended statistics and 19's `pg_upgrade` documentation no longer lists them as an exception (upgrade not executed here) |
| `uuidv4()`, `gamma()`, `lgamma()`, `to_number(..., 'RN')`, `json[b]_strip_nulls(..., strip_in_arrays)`, `EXTRACT(WEEK FROM interval)` | |
| Wire protocol 3.2 (opt-in via libpq `max_protocol_version`; 19's libpq still defaults to 3.0) | |

***

#### Features from 14–17 that a PostgreSQL 19 codebase can use

The version that introduced each feature is taken from the release notes; representative items of every row (`date_bin()`, `MERGE`, `UNIQUE NULLS NOT DISTINCT`, `security_invoker`, `1_000`/`0x1F` literals, `JSON_OBJECT`, `JSON_TABLE`, `JSON_VALUE`, `MERGE ... RETURNING`, `transaction_timeout`) were executed on 19rc1.[^38]

| Since | Feature |
| --- | --- |
| 14 | Multirange types (`int4multirange`, `range_agg()`), SQL-standard function bodies (`RETURN expr`, `BEGIN ATOMIC ... END`), `jsonb` subscripting (`doc['a']`), `date_bin()`, `ALTER TABLE ... DETACH PARTITION ... CONCURRENTLY`, `pg_read_all_data`/`pg_write_all_data`, `idle_session_timeout`, `SEARCH`/`CYCLE` in recursive CTEs, `GROUP BY DISTINCT`, LZ4 TOAST compression (`default_toast_compression`), `string_to_table()`, `password_encryption` default `scram-sha-256` |
| 15 | `MERGE`, `UNIQUE NULLS NOT DISTINCT`, views `WITH (security_invoker = true)`, `PUBLIC` loses `CREATE` on schema `public` (owned by `pg_database_owner`), `regexp_count/regexp_instr/regexp_like/regexp_substr`, `ON DELETE SET NULL (col, ...)` column lists, `pg_checkpoint` role, `jsonlog` log format; numeric literals followed directly by identifiers are rejected |
| 16 | SQL/JSON constructors `JSON_OBJECT`, `JSON_ARRAY`, `JSON_OBJECTAGG`, `JSON_ARRAYAGG` and `IS JSON [OBJECT|ARRAY|SCALAR]`, `any_value()`, numeric literals with underscores (`1_000_000`) and `0x`/`0o`/`0b` integers, `pg_input_is_valid()`/`pg_input_error_info()`, subqueries in `FROM` without alias, `EXPLAIN (GENERIC_PLAN)`, `pg_stat_io`, `SYSTEM_USER`, `date_add()`/`date_subtract()`, `array_shuffle()`/`array_sample()`, `random_normal()`, `pg_create_subscription` and `pg_use_reserved_connections` roles |
| 17 | `JSON_TABLE`, `JSON_QUERY`, `JSON_VALUE`, `JSON_EXISTS`, `JSON()`, `JSON_SCALAR()`, `JSON_SERIALIZE()`, jsonpath `.integer()`/`.bigint()`/`.number()`/`.string()`/`.boolean()`/`.date()`/`.time()`/`.timestamp()`..., `MERGE ... RETURNING` with `merge_action()`, `WHEN NOT MATCHED BY SOURCE`, `MERGE` into updatable views, `COPY ... (ON_ERROR ignore)`, `transaction_timeout`, `MAINTAIN` privilege and `pg_maintain` role, `EXPLAIN (SERIALIZE, MEMORY)`, `ALTER COLUMN ... SET EXPRESSION`, exclusion constraints on partitioned tables, `random(min, max)`, `uuid_extract_timestamp()`/`uuid_extract_version()`, `AT LOCAL`, `+/-infinity` intervals, `pg_stat_checkpointer`, login event triggers, incremental base backups (`pg_basebackup --incremental`, `pg_combinebackup`) |

On 19rc1, `SELECT 1_000, 0x1F;` returns `1000 | 31`. Code that must also run on 14 or 15 must not use these literals (16+ syntax).

***

#### Schema migrations and locking safety

Most production incidents caused by migrations are lock problems, not data problems. Every `ALTER TABLE` takes a table-level lock; the dangerous ones take `ACCESS EXCLUSIVE`, which conflicts with **every** other lock including plain `SELECT`.[^26][^14]

##### The lock queue: why a "fast" ALTER takes the site down

An `ALTER TABLE` that needs `ACCESS EXCLUSIVE` waits behind any open transaction that touched the table — and every later query on that table queues **behind the waiting ALTER**. Verified on 19rc1 (19 adds `CONTEXT: waiting for AccessShareLock on relation ...` to the timeout errors):

```text
session A: BEGIN; SELECT count(*) FROM lq; SELECT pg_sleep(6); COMMIT;     -- holds ACCESS SHARE
session B: ALTER TABLE lq ADD COLUMN x int;                                  -- waits (blocked by A)
session C: SET statement_timeout='2s'; SELECT count(*) FROM lq;              -- waits behind B
           ERROR:  canceling statement due to statement timeout
           CONTEXT:  waiting for AccessShareLock on relation 16384 of database 5
```

The ALTER itself was instantaneous; the outage is the queue. Always set `lock_timeout` in migrations and retry:

```sql
-- GOOD: give up quickly instead of blocking all traffic; retry from the migration runner
SET lock_timeout = '2s';
SET statement_timeout = '30s';     -- for statements that must not run long
ALTER TABLE lq ADD COLUMN y int;
-- ERROR:  canceling statement due to lock timeout   (when a long transaction holds the table)
```

Defaults on 19 are `lock_timeout = 0`, `statement_timeout = 0`, `idle_in_transaction_session_timeout = 0`, `transaction_timeout = 0` (17+): all disabled. Use `SET LOCAL` inside a transaction so the setting does not leak to a pooled connection.

##### Lock levels of common DDL (verified with `pg_locks` on 19rc1 where the statement can run in a transaction block; the rest from the documentation)

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
| `VACUUM FULL`, `CLUSTER`, plain `REPACK` (19), `TRUNCATE` | `ACCESS EXCLUSIVE` | yes | yes |
| `REPACK (CONCURRENTLY)` (19) | `SHARE UPDATE EXCLUSIVE`, then `ACCESS EXCLUSIVE` for the final swap | only during the swap | only during the swap |
| `ALTER CONSTRAINT ... ENFORCED` (FK 18, `CHECK` 19) | `ACCESS EXCLUSIVE` for the full validation scan | yes | yes |

##### Which ALTERs rewrite the table (verified with `pg_relation_filenode` on 19rc1)

A rewrite holds `ACCESS EXCLUSIVE` for the time it takes to copy the whole table and rebuild every index.

| Change | Rewrite? |
| --- | --- |
| `ADD COLUMN c int` / `DEFAULT 42` / `DEFAULT now()` (constant or stable default, "fast default" since 11) | no |
| `ADD COLUMN c uuid DEFAULT gen_random_uuid()` or `DEFAULT uuidv7()` (volatile default) | **yes** |
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
- **`VACUUM FULL`/`CLUSTER`/plain `REPACK`** rewrite under `ACCESS EXCLUSIVE`. On busy tables use `REPACK (CONCURRENTLY)` (19) with `lock_timeout`, outside a transaction block.
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
-- DETAIL:  Column "id" is an identity column defined as GENERATED ALWAYS.
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
- Avoid the `money` type: its output and input depend on `lc_monetary`, and it has fixed fractional precision.[^39]
- `numeric` without precision stores any scale; add `(p,s)` when the domain has one so bad data fails on insert.

##### Text types

```sql
SELECT 'ab '::char(5) = 'ab'::char(5), length('ab '::char(5));   -- t | 2  (trailing spaces are insignificant)
SELECT 'abc'::varchar(2);                                         -- ab  (explicit cast silently truncates!)
INSERT INTO vt VALUES ('abc');    -- with vt(v varchar(2)): ERROR: value too long for type character varying(2)
```

- Use `text` (or `varchar` without a limit) by default. A length limit is a business rule; if it matters, `CHECK (length(col) <= n)` can be changed without a rewrite, while widening `varchar(n)` is also metadata-only but narrowing is a rewrite and validation.
- Never use `char(n)`: it pads with spaces, strips them in comparisons and `length()`, and wastes space.[^39]
- `''` is not `NULL` in PostgreSQL (unlike Oracle).
- Case-insensitive uniqueness: a unique index on `lower(email)` (or `casefold(email)` (18) with a suitable collation), or a nondeterministic ICU collation; the `citext` extension also works but is slower and has its own operator pitfalls.

##### Date and time

```sql
SET timezone = 'Europe/Berlin';
SELECT '2025-03-30 02:30'::timestamptz;                       -- 2025-03-30 03:30:00+02  (nonexistent local time shifted)
SELECT timestamptz '2025-03-29 12:00' + interval '1 day',     -- 2025-03-30 12:00:00+02  (calendar day)
       timestamptz '2025-03-29 12:00' + interval '24 hours';  -- 2025-03-30 13:00:00+02  (elapsed time)
SELECT '2025-01-31'::date + interval '1 month';               -- 2025-02-28 00:00:00  (clamped)
SELECT '2025-01-31 23:59:59.5'::timestamp BETWEEN '2025-01-01' AND '2025-01-31';  -- f
```

- **Store instants as `timestamptz`**, never `timestamp` (without time zone): `timestamptz` stores UTC and converts on input/output using the session `TimeZone`; `timestamp` stores whatever wall-clock value the client sent, losing the offset.[^39][^42]
- Use half-open ranges for periods: `created_at >= $1 AND created_at < $2`. `BETWEEN '2025-01-01' AND '2025-01-31'` misses everything after midnight on the 31st.
- `now()`/`current_timestamp` are the transaction start time — identical for every row and statement in a transaction. Use `clock_timestamp()` for wall-clock time inside long transactions.
- `date + interval '1 month'` clamps to the month end; repeated monthly addition drifts (`Jan 31 → Feb 28 → Mar 28`). Compute from the anchor date.
- `timestamptz::date` depends on the session `TimeZone`; the same query returns different days for different clients. Use `(ts AT TIME ZONE 'Europe/Berlin')::date` explicitly.
- Do not use `timetz` (time with time zone): an offset without a date is meaningless across DST.[^39]
- Since 18, time zone abbreviation resolution changed: abbreviations of the session's `TimeZone` are checked before `timezone_abbreviations`.[^25]

##### UUIDs

Use the `uuid` type; prefer `uuidv7()` (18) for keys (see "Features from 18"). Do not compare UUIDs as strings — case and hyphen differences make equal values unequal.

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

Index types: B-tree (default; equality, ranges, sorting, `LIKE 'prefix%'` with the right operator class), Hash (equality only), GIN (`jsonb`, arrays, full-text, trigram), GiST (ranges, geometry, exclusion constraints, trigram), SP-GiST, BRIN (huge append-only tables whose physical order correlates with the column).[^34]

```sql
CREATE INDEX orders_user_status ON orders (user_id, status);            -- composite: column order matters
CREATE INDEX users_active_email ON users (email) WHERE status = 'active'; -- partial
CREATE INDEX users_lower_email ON users (lower(email));                  -- expression
CREATE INDEX users_email_cov ON users (email) INCLUDE (name, created_at); -- covering (index-only scans)
CREATE INDEX posts_fts ON posts USING gin (to_tsvector('english', title || ' ' || body));
CREATE INDEX events_doc ON events USING gin (doc jsonb_path_ops);
```

Verified behaviours on 19rc1 (100,000-row table, default `en_US.utf8` collation):

| Query | Index | Plan |
| --- | --- | --- |
| `WHERE email LIKE 'user42%'` | plain `(email)` | **Seq Scan** — a B-tree with a non-C collation cannot serve `LIKE` |
| same | `(email text_pattern_ops)` | Bitmap Index Scan with `~>=~`/`~<~` range |
| `WHERE lower(email) = '...'` | plain `(email)` | Seq Scan — needs an index on `lower(email)` |
| `WHERE id = 42::numeric` (id is `int`) | PK | **Seq Scan**: `(id)::numeric = '42'` — the cast is on the column |
| `WHERE id::text = '42'` | PK | Seq Scan |
| `WHERE email LIKE '%r4242%'` | `gin (email gin_trgm_ops)` (pg_trgm) | Bitmap Index Scan |
| `WHERE status = $1 AND created_at > ...` with generic plan | partial index `WHERE status = 'active'` | **Seq Scan** — a partial index is unusable when the predicate value is a parameter |
| `WHERE (created_at, id) < ($1, $2) ORDER BY created_at DESC, id DESC LIMIT 20` | `(created_at DESC, id DESC)` | Index Scan (Index Only Scan when only `created_at, id` are selected), no sort |

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

`= ANY($1::int[])` with an array parameter is preferable to building `IN ($1, $2, ..., $n)` with a variable number of placeholders: one statement text, one prepared plan. (Since 18, `pg_stat_statements` squashes constant lists: verified on 19rc1, `IN (1,2)`, `IN (1,2,3,4,5)` and `= ANY(ARRAY[1,2,3])` all land in one entry shown as `IN ($1 /*, ... */)` with 3 calls.)[^25][^22]

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

- Since 12, a non-recursive, side-effect-free CTE referenced once is **inlined** (verified: `WITH c AS (SELECT * FROM ct) SELECT * FROM c WHERE id = 5` → Index Scan). `WITH ... AS MATERIALIZED` forces an optimization fence (CTE Scan + Seq Scan). Do not request "rewrite the CTE as a subquery for performance" on 19 unless the plan proves it.
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

`REFRESH MATERIALIZED VIEW` takes `ACCESS EXCLUSIVE` and blocks readers; `REFRESH MATERIALIZED VIEW CONCURRENTLY` needs a unique index on the view (since 18 the unique index may also be a non-B-tree index that supports equality) and allows reads during the refresh.[^25]

##### Planner settings

`SET enable_nestloop = off` (or any `enable_*`) in application code affects every later query in the session — and in a pooled connection, other clients. Since 18, `EXPLAIN` marks such nodes `Disabled: true`. Fix statistics (`ANALYZE`, `ALTER TABLE ... ALTER COLUMN ... SET STATISTICS`, `CREATE STATISTICS` for correlated columns) instead of disabling planner features. There are no query hints in the SQL grammar; `pg_hint_plan` is a third-party extension, and 19 ships the contrib module `pg_plan_advice` (see "New in 19: Planner") — use it only with tests that check the advice still matches.

***

#### Reading EXPLAIN

```sql
EXPLAIN SELECT ...;                                  -- plan + estimates, does not execute
EXPLAIN (ANALYZE) SELECT ...;                        -- executes; includes BUFFERS automatically (18+)
EXPLAIN (ANALYZE, BUFFERS OFF, TIMING OFF) ...;      -- quieter output
EXPLAIN (GENERIC_PLAN) SELECT * FROM t WHERE id = $1;  -- 16+: plan for a parameterized statement
EXPLAIN (ANALYZE, MEMORY, SERIALIZE) ...;            -- 17+: planner memory, output serialization cost
EXPLAIN (ANALYZE, IO) ...;                           -- 19: asynchronous I/O / prefetch statistics
```

`EXPLAIN ANALYZE` **executes** the statement: wrap `INSERT/UPDATE/DELETE/MERGE` in `BEGIN; ... ROLLBACK;`.

What to look for:[^35]

- **Estimated vs actual rows** off by orders of magnitude → stale or insufficient statistics (`ANALYZE`, extended statistics for correlated columns).
- **`Rows Removed by Filter`** large on a Seq Scan or Index Scan → missing or wrong index, or non-sargable predicate.
- **`Index Searches:`** (18) large → skip scan over a high-cardinality prefix or an `= ANY` with many elements; consider column order.
- **`Buffers: shared read=`** high → cold cache or too much data touched; compare with `hit=`.
- **`Sort Method: external merge  Disk:`** → `work_mem` too small for this sort (raise per query/role, not globally).
- **`Heap Fetches:`** on Index Only Scan high → visibility map stale; vacuum.
- **Nested Loop with a large outer side** and inner Seq Scan → missing join index or misestimate.
- **`Subplans Removed: N`** under Append → runtime partition pruning worked (verified with a generic plan on a range-partitioned table).
- Actual rows are printed with two decimals since 18 (`rows=1.00`); averaged per loop.
- **`I/O: ... waits=`** (19, `IO` option) high relative to `count` → read-ahead did not keep up.

Example (19rc1, `EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, SUMMARY OFF)`):

```text
 Index Scan using u_pkey on u (actual rows=3.00 loops=1)
   Index Cond: (id = ANY ('{1,500,90000}'::integer[]))
   Index Searches: 3
   Buffers: shared hit=13
```

***

#### Transactions, isolation and concurrency

The default isolation level is **READ COMMITTED**: each statement sees data committed before it started.[^30]

##### Lost updates

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

Verified on 19rc1: a `REPEATABLE READ` transaction updating a row that another transaction changed after its snapshot fails with `ERROR: could not serialize access due to concurrent update` (SQLSTATE `40001`). Under `SERIALIZABLE`, two doctors each checking "at least two on call" and going off call (write skew) — one fails with `could not serialize access due to read/write dependencies among transactions`, and the invariant holds.

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

- `ON CONFLICT` is atomic against concurrent inserts of the same key. **`MERGE` is not**: verified, two concurrent `MERGE ... WHEN NOT MATCHED THEN INSERT` statements for the same key → one fails with `duplicate key value violates unique constraint`, while the same race with `ON CONFLICT DO UPDATE` succeeds. Use `ON CONFLICT` for upserts; use `MERGE` (15+) for set-based synchronization where conflicts are handled or impossible.[^7][^31]
- `ON CONFLICT DO UPDATE` affecting the same key twice in one statement fails (`command cannot affect row a second time`): deduplicate the input first.
- `ON CONFLICT DO NOTHING RETURNING` returns nothing for skipped rows. In 19, use `ON CONFLICT (k) DO SELECT RETURNING ...` to get the existing row atomically, instead of a follow-up `SELECT` or `DO UPDATE SET k = excluded.k` (which writes a new row version — bloat, triggers).
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
- Exception blocks in PL/pgSQL and `SAVEPOINT`s create subtransactions; more than 64 in one transaction (e.g. one per row in a loop) overflow the per-backend subtransaction cache and slow down snapshot handling, notably on hot standbys. Avoid per-row savepoints in drivers (some ORMs' "autosave" modes).

***

#### JSONB and SQL/JSON

Use `jsonb`, not `json`, unless exact input text (key order, duplicate keys, whitespace) must be preserved.[^32]

```sql
SELECT '{"a":1,"a":2}'::jsonb, '{"a":1,"a":2}'::json;   -- {"a": 2} | {"a":1,"a":2}
```

##### Operators and their traps (verified on 19rc1)

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
- **`JSON_SERIALIZE()` on a `jsonb` argument returns garbage** on 18.6 and 19rc1 (verified: `JSON_SERIALIZE('{"a":1}'::jsonb)` → `\x01`); it works for `json`/text input. Use `doc::text` to serialize `jsonb`.
- Prefer `jsonb_path_query`/`@?`/`@@` (jsonpath) over hand-written `jsonb_array_elements` loops for filtering.

***

#### Partitioning

Declarative partitioning (`PARTITION BY RANGE | LIST | HASH`) is for very large tables where partition pruning or dropping whole partitions (retention) pays off. It adds planning overhead and constraints; do not partition small tables.[^33]

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

Verified rules and traps on 19rc1:

- **Unique constraints and primary keys must include all partition key columns** (`UNIQUE constraint on partitioned table must include all partitioning columns`). Global uniqueness of `id` alone is not enforceable; use an identity/sequence and accept it, or a separate lookup table.
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
- **Unlogged partitioned tables are rejected** since 18 (`partitioned tables cannot be unlogged`).
- **Statistics**: autovacuum analyzes partitions but never the partitioned parent; queries that need parent-level statistics (joins, `GROUP BY` across partitions) require a scheduled `ANALYZE parent`. Since 18, `VACUUM`/`ANALYZE` of a parent recurse into children by default; `ANALYZE ONLY parent` analyzes just the parent. In 19, `vacuumdb --analyze-only`/`--analyze-in-stages` also issue `ANALYZE ONLY parent` for partitioned tables.[^25][^41]
- `NOT VALID` foreign keys can be added to partitioned tables (18). Identity columns and exclusion constraints on partitioned tables are supported (exclusion constraints since 17).
- `ALTER TABLE ... MERGE PARTITIONS` / `SPLIT PARTITION` do not exist in 19 (syntax errors on 19rc1); re-partitioning means create-new-partitions-and-move-rows.
- `COPY partitioned_table TO ...` works in 19; `REPACK (CONCURRENTLY)` does not accept a partitioned table (repack partitions individually).

***

#### Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL

##### Roles and privileges

- **Applications never connect as a superuser or as the table owner.** Superusers and roles with `BYPASSRLS` skip row-level security; owners skip it unless `FORCE ROW LEVEL SECURITY` is set. Use separate roles for migrations (owner) and runtime (DML only).
- **Schema `public`**: since 15, `PUBLIC` has no `CREATE` privilege on `public` in new databases and the schema is owned by `pg_database_owner` (verified on 19rc1: `permission denied for schema public` for an ordinary role). Databases upgraded from ≤14 with `pg_upgrade` keep the old, permissive ACL — check `\dn+ public` and `REVOKE CREATE ON SCHEMA public FROM PUBLIC`.[^40]
- **Functions are executable by `PUBLIC` by default** (verified: a fresh function is executable by any role). `REVOKE EXECUTE ON FUNCTION ... FROM PUBLIC` for anything sensitive — especially `SECURITY DEFINER` functions — or set `ALTER DEFAULT PRIVILEGES REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC`.
- `GRANT SELECT ON ALL TABLES IN SCHEMA s` covers existing tables only; future tables need `ALTER DEFAULT PRIVILEGES ... GRANT ...` (per creating role).
- Predefined roles instead of superuser: `pg_read_all_data`/`pg_write_all_data` (14), `pg_checkpoint` (15), `pg_create_subscription`, `pg_use_reserved_connections` (16), `pg_maintain` (17), `pg_signal_autovacuum_worker` (18; in 19 `pg_read_all_data`/`pg_write_all_data` also cover large objects), plus older `pg_monitor`, `pg_read_server_files`, `pg_signal_backend`.
- Passwords: SCRAM only. Never put passwords in migration files (`CREATE ROLE app PASSWORD 'literal'` lands in version control and, with `log_statement = 'ddl'` or `'all'`, in the server log); set them out of band (`\password` in psql sends a hash).

##### Row-level security

```sql
CREATE TABLE docs (id int, tenant text, body text);
ALTER TABLE docs ENABLE ROW LEVEL SECURITY;
ALTER TABLE docs FORCE ROW LEVEL SECURITY;              -- also applies to the table owner
CREATE POLICY tenant_iso ON docs
    USING (tenant = current_setting('app.tenant', true))
    WITH CHECK (tenant = current_setting('app.tenant', true));
```

Verified on 19rc1:

- Without `FORCE`, the **table owner sees all rows**; with `FORCE` and no tenant set, the owner sees 0 rows. Superusers always bypass.
- A policy with only `USING` also applies its expression as `WITH CHECK` for `INSERT`/`UPDATE` (inserting another tenant's row fails with `new row violates row-level security policy`). Write `WITH CHECK` explicitly anyway: it is clearer and required when the read and write rules differ.
- **Views bypass RLS of their owner**: a normal view owned by a superuser (or the table owner) showed all rows to an unprivileged role; the same view `WITH (security_invoker = true)` (15+) applied the caller's privileges and the policy (0 rows for a reader without tenant context; a caller without a grant on the table gets `permission denied for table docs`). Views over RLS tables must be `security_invoker` or owned by a role subject to the policies.[^43]
- `current_setting('app.tenant', true)` returns `NULL` when unset (policy matches nothing — fail closed). Set it with `SET LOCAL app.tenant = ...` or `set_config('app.tenant', $1, true)` **inside the transaction**: a session-level `SET` leaks to the next client of a pooled connection.
- Functions used in policies should be `LEAKPROOF` or simple operators; non-leakproof functions in a query can be evaluated before the policy filter and leak values through errors.[^28]

##### `SECURITY DEFINER` functions and `search_path`

A `SECURITY DEFINER` function runs with its owner's privileges. Any unqualified name it uses is resolved through the **caller's** `search_path` unless the function pins it.[^27]

```sql
-- BAD: unqualified lower() resolved via the caller's search_path
CREATE FUNCTION sec.bad_count(name text) RETURNS bigint
SECURITY DEFINER LANGUAGE plpgsql AS $$
BEGIN
  RETURN (SELECT count(*) FROM pg_catalog.pg_class WHERE relname = lower(name));
END $$;

-- GOOD: pinned search_path with pg_temp last, schema-qualified objects, EXECUTE revoked from PUBLIC
CREATE FUNCTION sec.good_count(name text) RETURNS bigint
SECURITY DEFINER SET search_path = pg_catalog, pg_temp LANGUAGE plpgsql AS $$
BEGIN
  RETURN (SELECT count(*) FROM pg_class WHERE relname = lower(name));
END $$;
REVOKE ALL ON FUNCTION sec.bad_count(text), sec.good_count(text) FROM PUBLIC;
```

Verified exploit on 19rc1: an unprivileged role with its own schema `evil` creates `evil.lower(text)` that reads `sec.secrets`, runs `SET search_path = evil, pg_catalog`, and calls `sec.bad_count('x')` — the attacker's function runs **as the superuser owner** and prints the secret table; `sec.good_count` is unaffected. A function with `SET search_path = pg_temp, pg_catalog` (pg_temp first) was hijacked by a temporary table named `pg_class`. Rules:

- Every `SECURITY DEFINER` function has `SET search_path = ...` with trusted schemas only and `pg_temp` **last** (or `search_path = ''` plus fully qualified names).
- `pg_catalog` is searched first implicitly only if it is *not* listed; listing it after an attacker-writable schema puts it behind.
- Owner should be a dedicated low-privilege role, not a superuser.
- `REVOKE EXECUTE ... FROM PUBLIC`, then grant to specific roles.
- Note: `SECURITY DEFINER` and `SET` clauses both prevent inlining of `LANGUAGE sql` functions; do not add `SET search_path` to hot-path SQL functions that rely on inlining unless they need it.

##### SQL injection in dynamic SQL

```sql
-- BAD: concatenating values into SQL (PL/pgSQL)
EXECUTE 'SELECT count(*) FROM pg_class WHERE relname = ''' || val || '''' INTO n;
-- with val = x' OR '1'='1 → counted every row of pg_class instead of 0

-- GOOD: identifiers through format(%I), values as parameters
EXECUTE format('SELECT count(*) FROM %I WHERE relname = $1', tbl) INTO n USING val;
```

- `format('%I', ...)` quotes identifiers (`"my table"`, `"Users"`); `%L` quotes literals (`'O''Reilly'`) and renders `NULL` as `NULL`; **`%s` inserts raw text** — `format('... = ''%s''', val)` is injectable (verified).
- Prefer `EXECUTE ... USING` parameters over `%L`: no quoting at all, and types are preserved.
- `quote_literal(NULL)` returns `NULL`, so `'... ' || quote_literal(v)` yields a `NULL` query string (`query string argument of EXECUTE is null`). Use `quote_nullable()` or `%L`.
- Identifiers from users must additionally be validated against an allow-list (or `regclass` cast: `$1::regclass`) — quoting prevents injection, not access to other tables.
- In application code: bind parameters only. `ORDER BY` columns and directions cannot be parameters; map user input to a fixed allow-list.[^29]

***

#### Functions, procedures and triggers

##### Volatility and strictness

- `IMMUTABLE` promises the same output for the same input forever; the planner pre-evaluates it and indexes trust it. PostgreSQL does not verify the claim: verified, a SQL function `RETURNS timestamptz IMMUTABLE AS 'select now()'` is accepted into an index expression — the index is then silently wrong. Built-ins that depend on settings are only `STABLE` (`to_char(timestamptz, ...)` and `timestamptz::date` are rejected in index expressions: `functions in index expression must be marked IMMUTABLE`).
- `STABLE` for functions that read tables or settings; `VOLATILE` (default) for anything with side effects or `random()`/`clock_timestamp()`.
- `STRICT` functions return `NULL` on any `NULL` argument without executing.
- `PARALLEL SAFE` must be declared explicitly for user functions to allow parallel plans; a wrong `PARALLEL SAFE` on a function that writes is a bug.
- SQL-standard bodies (14+) are parsed at definition time and track dependencies:

```sql
CREATE FUNCTION add1(i int) RETURNS int LANGUAGE sql IMMUTABLE RETURN i + 1;
CREATE PROCEDURE p_atomic() LANGUAGE sql BEGIN ATOMIC SELECT 1; END;
```

##### PL/pgSQL traps (verified)

```sql
CREATE TABLE pu (id int, name text);
INSERT INTO pu VALUES (1, 'a'), (1, 'b');

-- SELECT INTO without STRICT silently takes the first row, or NULL if none
DO $$ DECLARE n text; BEGIN
  SELECT name INTO n FROM pu WHERE id = 1;          -- two rows exist: n = 'a', no error
  RAISE NOTICE 'got %', n;
END $$;
DO $$ DECLARE n text; BEGIN
  SELECT name INTO STRICT n FROM pu WHERE id = 1;   -- ERROR: query returned more than one row
END $$;
DO $$ DECLARE n text; BEGIN
  SELECT name INTO STRICT n FROM pu WHERE id = 2;   -- ERROR: query returned no rows
END $$;
```

- Use `INTO STRICT` when exactly one row is expected; otherwise check `FOUND`.
- Raise domain errors with a SQLSTATE the application can map: `RAISE EXCEPTION 'insufficient funds for account %', id USING ERRCODE = 'P0001', HINT = '...'`.
- `EXCEPTION` blocks start a subtransaction each time they are entered; do not put one inside a per-row loop.
- `EXECUTE` dynamic SQL is replanned every time; static SQL in PL/pgSQL is prepared and cached (custom vs generic plans per `plan_cache_mode`).
- Row-by-row loops (`FOR r IN SELECT ... LOOP UPDATE ... WHERE id = r.id`) are an anti-pattern; use one set-based statement.
- Procedures (`CALL`, 11+) can `COMMIT` inside; functions cannot.

##### Triggers

- `BEFORE ... FOR EACH ROW` returning `NULL` silently skips the row; returning `NEW` is required to proceed. Generated columns are not visible in `BEFORE` triggers.
- `AFTER` row triggers queue events in memory for the whole statement; bulk statements over millions of rows can exhaust memory. Statement-level triggers with transition tables (`REFERENCING NEW TABLE AS ...`) scale better.
- Since 18, `AFTER` triggers execute as the role that queued the event (verified on 19rc1, see "Features from 18").
- Triggers that write to other tables hide work from the reader of the application code and can cascade; keep them for invariants and auditing, not business workflows.
- `updated_at` maintenance: a `BEFORE UPDATE` trigger setting `NEW.updated_at = now()` is fine; remember `now()` is the transaction start.
- Event triggers (including login event triggers, 17) run for every matching DDL or connection; a failing login trigger blocks logins (escape hatch: `event_triggers = false` in single-user mode or as superuser).

***

#### Application code: parameters, pooling, timeouts

##### Parameterized queries only

```python
# BAD
cur.execute(f"SELECT * FROM users WHERE email = '{email}'")
# GOOD
cur.execute("SELECT * FROM users WHERE email = %s", (email,))
```

Also: no string-built `LIKE` patterns without escaping `%`/`_` (`LIKE $1 ESCAPE '\'` with the value escaped), no user-controlled `ORDER BY` text, no user-controlled `COPY ... FROM '/path'` or `lo_import`.

##### Connection pooling

Each PostgreSQL connection is an OS process; hundreds of idle connections cost memory and snapshot overhead. Use a pooler (PgBouncer, pgcat, Odyssey, or the driver's pool) and size the server pool to roughly the number of concurrently active queries the hardware sustains, not to the number of application threads.

In PgBouncer **transaction pooling** mode, consecutive transactions of one client can run on different server connections:[^36][^37]

| Feature | Transaction pooling |
| --- | --- |
| `SET`/`RESET` (session level) | broken: leaks to other clients / lost. Use `SET LOCAL` or `set_config(..., true)` |
| `LISTEN` | broken |
| Session advisory locks | broken; use `pg_advisory_xact_lock` |
| SQL `PREPARE`/`DEALLOCATE` | broken |
| Protocol-level prepared statements | work with PgBouncer ≥ 1.21 and `max_prepared_statements` > 0 (default 200 since 1.24) |
| `WITH HOLD` cursors, temp tables with `PRESERVE`/`DELETE ROWS` | broken |
| Temp tables `ON COMMIT DROP` | work |

Review any `SET search_path`, `SET statement_timeout`, `SET ROLE`, `SET app.tenant` executed outside a transaction in code that runs behind a transaction pooler.

##### Timeouts and retries

```sql
ALTER ROLE app SET statement_timeout = '15s';
ALTER ROLE app SET idle_in_transaction_session_timeout = '60s';
ALTER ROLE app SET transaction_timeout = '5min';     -- 17+
ALTER ROLE app SET lock_timeout = '5s';
```

- Retry on `40001` (serialization failure) and `40P01` (deadlock) by re-running the whole transaction; do not retry on `23505` (unique violation) blindly.
- A client-side timeout without a server-side `statement_timeout` leaves the query running on the server.
- Set `application_name` per service; it appears in `pg_stat_activity` and logs (since 18 `log_line_prefix` also supports `%L` for the client IP).

##### TLS and connection strings

`sslmode=verify-full` (with `sslrootcert`) in production; libpq's default `prefer` allows unencrypted fallback and does not verify the server. Do not commit connection strings with passwords. libpq (18+) supports `sslkeylogfile` for debugging — never enable it in production. Wire protocol 3.2 (18) is opt-in via `max_protocol_version=3.2`/`latest` (19's libpq still defaults to 3.0); poolers and proxies must support it before you enable it. 19's libpq adds the `servicefile` connection parameter.[^44]

***

#### Maintenance and observability

##### Vacuum and statistics

- Autovacuum must stay on. Tables with heavy updates/deletes need per-table tuning (`ALTER TABLE t SET (autovacuum_vacuum_scale_factor = 0.02)`). 18 added `autovacuum_vacuum_max_threshold` (default 100,000,000 dead tuples), which caps the threshold for huge tables, and eager freezing of all-visible pages during normal vacuums (`vacuum_max_eager_freeze_failure_rate`).[^25][^41]
- Long transactions, abandoned replication slots and prepared transactions prevent vacuum from removing dead rows and advancing `relfrozenxid` (wraparound risk). 18 added `idle_replication_slot_timeout` (default `0` = disabled) to invalidate idle slots.
- `VACUUM FULL` is a blocking rewrite; regular `VACUUM` does not shrink files except trailing empty pages (`vacuum_truncate`, also a server setting since 18). Use `REPACK` (19) to shrink a bloated table.
- `REINDEX ... CONCURRENTLY` (12+) for bloated indexes; plain `REINDEX` blocks writes.
- 19 autovacuum can vacuum indexes in parallel (`autovacuum_max_parallel_workers`, default `0`) and orders tables by score (`pg_stat_autovacuum_scores`).

##### Useful queries (column names verified on 19rc1)

```sql
-- Slowest statements (pg_stat_statements must be in shared_preload_libraries; columns renamed in 13)
SELECT query, calls, total_exec_time, mean_exec_time, rows, shared_blks_read
FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT 10;

-- Tables with many sequential scans (note: relname, not tablename)
SELECT schemaname, relname, seq_scan, seq_tup_read, idx_scan,
       total_vacuum_time, total_autovacuum_time          -- 18
FROM pg_stat_user_tables ORDER BY seq_tup_read DESC LIMIT 10;

-- Unused indexes (relname = table, indexrelname = index)
SELECT schemaname, relname, indexrelname, idx_scan, last_idx_scan,      -- last_idx_scan: 16+
       pg_size_pretty(pg_relation_size(indexrelid)) AS size
FROM pg_stat_user_indexes WHERE idx_scan = 0
ORDER BY pg_relation_size(indexrelid) DESC;

-- Who blocks whom
SELECT pid, wait_event_type, state, pg_blocking_pids(pid) AS blocked_by, left(query, 60)
FROM pg_stat_activity WHERE cardinality(pg_blocking_pids(pid)) > 0;
```

`pg_stat_user_tables` and `pg_stat_user_indexes` have `relname`/`indexrelname`; queries using `tablename`/`indexname` (columns of `pg_tables`/`pg_indexes`) fail.

##### 19 monitoring changes (verified in the 19rc1 catalog)

- **New views**: `pg_stat_lock` (per lock type: `waits`, `wait_time`, `fastpath_exceeded`), `pg_stat_recovery` (replay status on standbys), `pg_stat_autovacuum_scores`, `pg_stat_progress_repack`, `pg_dsm_registry_allocations`.
- **New columns**: `stats_reset` in `pg_stat_all_tables`/`_indexes`, `pg_statio_all_*` (and the `sys`/`user` variants), `pg_stat_user_functions`, `pg_stat_database_conflicts`; `started_by` in `pg_stat_progress_vacuum` and `pg_stat_progress_analyze`, `mode` in `pg_stat_progress_vacuum`; `wal_fpi_bytes` in `pg_stat_wal`; `mem_exceeded_count`, `slotsync_skip_count`, `slotsync_last_skip` in `pg_stat_replication_slots`; `slotsync_skip_reason` in `pg_replication_slots`; `backup_type` in `pg_stat_progress_basebackup`; `location` in `pg_available_extensions`; `tableid`/`attnum` in `pg_stats`.
- **Renamed**: `pg_stat_subscription_stats.sync_error_count` → `sync_table_error_count`; wait-event type `BufferPin` → `Buffer` (`SELECT DISTINCT type FROM pg_wait_events` lists `Buffer` on 19, `BufferPin` on 18.6). Dashboards filtering `wait_event_type = 'BufferPin'` silently show nothing.
- **Logging**: `log_lock_waits` on by default; `log_min_messages` per process type; `log_autoanalyze_min_duration`; messages from remote servers (replication, `postgres_fdw`, `dblink`) use the local log format; new I/O wait events for `COPY` to/from files, pipes and programs.
- **Functions**: `pg_get_multixact_stats()`, `pg_get_sequence_data()` returns the page LSN, `pg_buffercache_os_pages()`, `pg_buffercache_mark_dirty*()`.
- `pg_stat_statements` 1.13: `generic_plan_calls`, `custom_plan_calls`.[^1][^23]

```sql
-- 19: which lock types make sessions wait
SELECT locktype, waits, wait_time, fastpath_exceeded FROM pg_stat_lock WHERE waits > 0 ORDER BY wait_time DESC;
-- 19: what autovacuum will process next
SELECT relname, score, do_vacuum, do_analyze, for_wraparound FROM pg_stat_autovacuum_scores ORDER BY score DESC LIMIT 10;
```

***

#### Removed / changed in 19 — migration notes from 18

Behaviour changes that can break code or operations when moving from 18 to 19 (verified on 19rc1 vs 18.6 where executable):[^1]

| Change | Impact | Action |
| --- | --- | --- |
| `standard_conforming_strings` always `on`; `escape_string_warning` removed | Old dumps/scripts with `SET standard_conforming_strings = off` load backslashes differently (verified data change); the settings in `postgresql.conf` or `-c` prevent startup | Dump with 19's `pg_dump`; use `E'...'`; remove both settings |
| `io_workers` removed (`io_min_workers`/`io_max_workers`) | `postgresql.conf` with `io_workers` → `FATAL: configuration file ... contains errors` (verified) | Replace before upgrading |
| RADIUS authentication removed | `pg_hba.conf` with `radius` is not loaded | Migrate authentication first |
| `MULE_INTERNAL` encoding removed | `CREATE DATABASE ... ENCODING 'MULE_INTERNAL'` → `MULE_INTERNAL is not a valid encoding name`; such databases must be dumped and restored with another encoding | Convert before upgrading |
| CR/LF forbidden in database, role, tablespace names | `CREATE ROLE` fails; `pg_upgrade` refuses such clusters | Rename before upgrading |
| Default GiST opclass for `inet`/`cidr` is the core `inet_ops` (btree_gist's `gist_inet_ops`/`gist_cidr_ops` are broken: they can exclude rows that should be returned) | `CREATE INDEX ... USING gist (inet_col)` works without an opclass on 19 (18 without btree_gist: no default operator class; with btree_gist: `gist_inet_ops`); `pg_upgrade` refuses clusters with btree_gist `inet`/`cidr` indexes | On 18, recreate them with `USING gist (col inet_ops)` before `pg_upgrade` |
| System columns rejected in `COPY FROM ... WHERE` | `WHERE tableoid ...` worked on 18 | Filter after loading |
| `json_array(query)` over zero rows → `[]` | `IS NULL` checks change meaning | Test for `'[]'` / `json_array_length() = 0` |
| `postgres_fdw` remote sessions inherit `READ ONLY`/`DEFERRABLE` | Side effects on the remote server inside read-only transactions now fail | Use read-write transactions where remote writes are intended |
| `jit` default `off` | Long analytical queries may slow down | `ALTER ROLE analytics SET jit = on` where measured to help |
| `max_locks_per_transaction` default 128; lock allocation changed | An explicit setting gives half its old capacity | Double explicit values |
| `default_toast_compression` = `lz4` | Newly stored TOAST values use LZ4 | Set `default_toast_compression` explicitly if a specific method is required |
| `log_lock_waits` on; `log_autovacuum_min_duration` no longer covers autoanalyze | More lock-wait log lines; autoanalyze logging needs `log_autoanalyze_min_duration` | Adjust logging and alerting |
| `pg_stat_subscription_stats.sync_error_count` renamed; wait-event type `BufferPin` → `Buffer` | Monitoring queries break or return nothing | Update queries |
| MD5 logins warn; password-expiry warnings | New `WARNING`s at connect time | Migrate to SCRAM; rotate expiring passwords |
| `vacuumdb --analyze-only`/`--analyze-in-stages` include partitioned parents | Longer post-upgrade analyze runs | Expected |
| `pg_createsubscriber` writes recovery parameters to `pg_createsubscriber.conf` | Scripts reading `postgresql.auto.conf` | Update scripts |
| psql `\crosstabview` honours `\pset null` | Output of scripted reports changes | Review report scripts |
| Dutch stemmer updated (old one: `dutch_porter`) | Different stems | Rebuild Dutch FTS data |
| Error message details changed (function-not-found `DETAIL`, lock-wait `CONTEXT`) | Tests matching message text fail | Match SQLSTATE |
| Index AM API (`IndexAmRoutine` static), planner hooks (`get_relation_info_hook` removed) | Out-of-tree extensions need 19-compatible releases | Upgrade extensions |
| Build: C11, Meson ≥ 0.57.2, Visual Studio 2019+ | Old build environments fail | Update toolchain |

Upgrade path: `pg_upgrade` (checks: CR/LF names, btree_gist `inet`/`cidr` indexes, logical slots, checksums must match — 18 `initdb` enables them by default), dump/restore with **19's** `pg_dump`, or logical replication (19 also synchronizes sequences). After `pg_upgrade`, run `vacuumdb --all --analyze-in-stages --missing-stats-only` and then `vacuumdb --all --analyze-only`; 19's `pg_dump` can include restorable extended statistics, and 19's `pg_upgrade` documentation no longer lists `CREATE STATISTICS` statistics among the ones it does not transfer (18's did); custom statistics added by extensions and cumulative statistics are still not transferred. Update extensions (`ALTER EXTENSION ... UPDATE`; e.g. `pg_stat_statements` 1.13, `btree_gist` 1.9, `postgres_fdw` 1.3 ship with 19rc1).[^1][^50]

**Before GA**: the RC 1 announcement asks for a major-version upgrade strategy (`pg_upgrade` or `pg_dump`/`pg_restore`) to move to RC 1 "from earlier versions of PostgreSQL"; do not put data you cannot reload on a pre-release build.[^3]

***

#### Production / review checklist

##### Version and deployment

- [ ] Production targets 19.0 or the latest 19.x after GA (planned November 19, 2026); `19rc1`/beta images only in test environments
- [ ] No PostgreSQL 20-devel features (`JSON_TABLE ... PLAN`, `max(uuid)`, `toast_value_type`, `pg_get_*_ddl()`, online checksum functions, `enable_groupagg`, `log_statement_max_length`) and no `GROUP BY ALL`, `FOR PORTION OF`, `MERGE/SPLIT PARTITION`, `GRAPH_TABLE`
- [ ] Code shared with 18 servers avoids `REPACK`, `ON CONFLICT DO SELECT`, `IGNORE NULLS`, `WAIT FOR`, `COPY (FORMAT json)`/`set_null`/`HEADER n`, `random(date, date)`, `error_on_null`, `base64url`, `oid8`, `uuid::bytea`, jsonpath string methods, `ALTER CONSTRAINT` on `CHECK`, `EXPLAIN (IO)`, `GRANTED BY` other roles, `CHECKPOINT (...)`
- [ ] Configuration free of `io_workers`, `escape_string_warning`, `standard_conforming_strings = off`, RADIUS lines; `max_locks_per_transaction` doubled if set; `jit` set deliberately

##### Migrations

- [ ] `SET lock_timeout` (and `statement_timeout`) in every migration touching a busy table; runner retries on lock timeout
- [ ] No table rewrites on large tables: no volatile defaults (`gen_random_uuid()`, `uuidv7()`), no `STORED` generated column adds, no `int`→`bigint` in place, no varchar narrowing
- [ ] Indexes created/dropped `CONCURRENTLY`, outside a transaction; failed builds (`indisvalid = false`) cleaned up (also required before `REPACK`)
- [ ] Partitioned-table indexes built with `ON ONLY` + per-partition `CONCURRENTLY` + `ATTACH PARTITION`
- [ ] FKs/CHECKs added `NOT VALID` then `VALIDATE CONSTRAINT`; NOT NULL via `ADD CONSTRAINT ... NOT NULL ... NOT VALID` (18) or validated CHECK; no `ALTER CONSTRAINT ... ENFORCED` on large tables
- [ ] Unique constraints via `CREATE UNIQUE INDEX CONCURRENTLY` + `ADD CONSTRAINT ... USING INDEX`
- [ ] Bloat removal with `REPACK (CONCURRENTLY)` + `lock_timeout`, not `VACUUM FULL`/`CLUSTER`/plain `REPACK` on busy tables; no long transactions or `REPEATABLE READ` readers on the table during it
- [ ] Backfills batched; expand/contract for renames and drops
- [ ] Generated columns: `STORED` explicitly where the value is indexed, unique, referenced or replicated

##### Schema and types

- [ ] Primary key on every table (also required for `REPACK (CONCURRENTLY)` and logical replication); `bigint`/`uuid` keys; identity instead of `serial`
- [ ] `uuid` type (not text); `uuidv7()` for insert-heavy keys unless creation time must stay private
- [ ] `timestamptz` for instants; half-open time ranges; explicit time zone for day boundaries
- [ ] `numeric` for money; no `float` for exact values; no `money`, `char(n)`, `timetz`; no `oid8` for user data
- [ ] Foreign-key columns on the referencing side indexed; `ON DELETE` actions deliberate
- [ ] `NOT ENFORCED` constraints not relied upon for integrity

##### Queries

- [ ] `NOT EXISTS` instead of `NOT IN (subquery)` (19's anti-join conversion does not change `NULL` semantics); `coalesce` around aggregates that may see zero rows
- [ ] Sargable predicates; parameter types match column types
- [ ] Keyset pagination with a unique tie-breaker for deep pages; `ORDER BY` with every `LIMIT`
- [ ] `= ANY($1)` arrays instead of variable-length `IN` lists; no N+1 loops
- [ ] Gap filling with `IGNORE NULLS` (19) rather than correlated subqueries
- [ ] `json_array(query)` callers expect `[]` for zero rows
- [ ] Plans checked with `EXPLAIN (ANALYZE[, IO])` (DML inside `BEGIN ... ROLLBACK`); plan advice, if any, verified `/* matched */`
- [ ] No session-level `enable_*` toggles in application code

##### Concurrency

- [ ] Atomic updates (`SET n = n + 1`) or `SELECT ... FOR UPDATE`/version checks for read-modify-write
- [ ] Get-or-create uses `ON CONFLICT ... DO SELECT RETURNING` (19); upserts use `ON CONFLICT DO UPDATE`, not `MERGE` or select-then-insert
- [ ] `MERGE ... WHEN NOT MATCHED BY SOURCE` scoped by a condition
- [ ] Retry loops for `40001`/`40P01` with `REPEATABLE READ`/`SERIALIZABLE`
- [ ] `WAIT FOR LSN` always with `TIMEOUT`, issued before any lock-taking statement; `NO_THROW` results checked
- [ ] Queues use `FOR UPDATE SKIP LOCKED`; advisory locks are transaction-scoped behind poolers
- [ ] `idle_in_transaction_session_timeout`, `statement_timeout`, `transaction_timeout` (17+) set for app roles

##### Data loading and export

- [ ] `COPY ... ON_ERROR set_null` only where silent `NULL`s are acceptable (also nulls domain-constraint violations)
- [ ] `COPY ... (FORMAT json)` consumers do not need to distinguish SQL `NULL` from JSON `null`
- [ ] Scripted `COPY ... FROM STDIN` data terminated with `\.`

##### JSON

- [ ] `jsonb` not `json`; filtered fields have expression indexes or queries use `@>` with GIN
- [ ] No `jsonb_set(..., NULL)` data loss; `->>` vs `->` correct
- [ ] `JSON_VALUE`/`JSON_QUERY` use `ERROR ON ERROR` where silent `NULL` is wrong; no `JSON_SERIALIZE(jsonb)`

##### Security

- [ ] App role is not superuser/owner/`BYPASSRLS`; DML-only grants; default privileges set
- [ ] RLS tables use `FORCE ROW LEVEL SECURITY`, explicit `WITH CHECK`, `SET LOCAL`/`set_config(..., true)` for tenant context
- [ ] Views over protected tables are `security_invoker`
- [ ] `SECURITY DEFINER` functions: `SET search_path` with `pg_temp` last, qualified names, `EXECUTE` revoked from `PUBLIC`
- [ ] Dynamic SQL uses `format('%I')` + `EXECUTE ... USING`; no `%s` or `||` with values; identifiers allow-listed
- [ ] SCRAM passwords (MD5 now warns at login); no passwords in migrations; `sslmode=verify-full`
- [ ] No RADIUS; OAuth validators reviewed; `GRANTED BY` used only by roles entitled to act for the named grantor
- [ ] `pg_stash_advice` strings contain nothing sensitive

##### Operations

- [ ] Pooler mode compatible with the code (no session state in transaction pooling; PgBouncer ≥ 1.21 for prepared statements)
- [ ] Monitoring queries updated for 19 (`sync_table_error_count`, `Buffer` wait-event type, new `stats_reset` columns, `log_autoanalyze_min_duration`)
- [ ] Logical slots monitored (19 enables logical decoding on demand with `wal_level = replica`; abandoned slots retain WAL)
- [ ] Partitioned parents analyzed on a schedule; partitions pre-created; retention by detach/drop
- [ ] After `pg_upgrade`: `vacuumdb --analyze-in-stages --missing-stats-only`, then `--analyze-only`; extensions updated; extended statistics checked (`pg_stats_ext`)

***

[^1]: PostgreSQL 19 Release Notes (E.1, draft "as of 2026-10-09", incl. "Migration to Version 19"). https://www.postgresql.org/docs/19/release-19.html
[^2]: PostgreSQL Versioning Policy (supported versions, current minors, final release dates). https://www.postgresql.org/support/versioning/
[^3]: PostgreSQL 19 RC 1 Released! (October 15, 2026; RC 2 on November 12, GA planned November 19, 2026; changes since Beta 4). https://www.postgresql.org/about/news/postgresql-19-rc-1-released-3409/
[^4]: PostgreSQL Roadmap (minor release schedule: November 12, 2026; February 11, May 13, August 12, 2027). https://www.postgresql.org/developer/roadmap/
[^5]: PostgreSQL 18.6, 17.11, 16.15, 15.19, 14.24 and 19 Beta 3 Released (August 13, 2026). https://www.postgresql.org/about/news/postgresql-186-1711-1615-1519-1424-and-19-beta-3-released-3365/
[^6]: REPACK. https://www.postgresql.org/docs/19/sql-repack.html
[^7]: INSERT (ON CONFLICT DO SELECT / DO UPDATE). https://www.postgresql.org/docs/19/sql-insert.html
[^8]: WAIT FOR. https://www.postgresql.org/docs/19/sql-wait.html
[^9]: COPY (FORMAT json, FORCE_ARRAY, ON_ERROR set_null, HEADER). https://www.postgresql.org/docs/19/sql-copy.html
[^10]: Window Functions (IGNORE NULLS / RESPECT NULLS). https://www.postgresql.org/docs/19/functions-window.html
[^11]: pg_plan_advice — help the planner get the right plan. https://www.postgresql.org/docs/19/pgplanadvice.html
[^12]: pg_stash_advice — store and automatically apply plan advice. https://www.postgresql.org/docs/19/pgstashadvice.html
[^13]: EXPLAIN. https://www.postgresql.org/docs/19/sql-explain.html
[^14]: ALTER TABLE (lock levels, NOT VALID, ALTER CONSTRAINT ... ENFORCED). https://www.postgresql.org/docs/19/sql-altertable.html
[^15]: Connections and Authentication (password_expiration_warning_threshold, md5_password_warnings). https://www.postgresql.org/docs/19/runtime-config-connection.html
[^16]: Resource Consumption (io_method, io_min_workers, io_max_workers, autovacuum workers). https://www.postgresql.org/docs/19/runtime-config-resource.html
[^17]: Write Ahead Log (wal_level, effective_wal_level). https://www.postgresql.org/docs/19/runtime-config-wal.html
[^18]: CREATE PUBLICATION (ALL SEQUENCES, ALL TABLES EXCEPT). https://www.postgresql.org/docs/19/sql-createpublication.html
[^19]: JSON Functions and Operators (jsonpath string methods, SQL/JSON). https://www.postgresql.org/docs/19/functions-json.html
[^20]: Binary String Functions and Operators (encode/decode base64url, base32hex). https://www.postgresql.org/docs/19/functions-binarystring.html
[^21]: Range/Multirange Functions and Operators (range_minus_multi, multirange_minus_multi). https://www.postgresql.org/docs/19/functions-range.html
[^22]: pg_stat_statements. https://www.postgresql.org/docs/19/pgstatstatements.html
[^23]: The Cumulative Statistics System. https://www.postgresql.org/docs/19/monitoring-stats.html
[^24]: PostgreSQL 20devel documentation (built 2026-10-21 from master; release notes placeholder, index, JSON_TABLE PLAN, toast_value_type). https://www.postgresql.org/docs/devel/release-20.html , https://www.postgresql.org/docs/devel/bookindex.html , https://www.postgresql.org/docs/devel/functions-json.html , https://www.postgresql.org/docs/devel/sql-createtable.html
[^25]: PostgreSQL 18 Release Notes. https://www.postgresql.org/docs/18/release-18.html
[^26]: Explicit Locking (table- and row-level lock modes). https://www.postgresql.org/docs/19/explicit-locking.html
[^27]: CREATE FUNCTION — Writing SECURITY DEFINER Functions Safely. https://www.postgresql.org/docs/19/sql-createfunction.html
[^28]: Row Security Policies. https://www.postgresql.org/docs/19/ddl-rowsecurity.html
[^29]: PL/pgSQL — Executing Dynamic Commands. https://www.postgresql.org/docs/19/plpgsql-statements.html
[^30]: Transaction Isolation. https://www.postgresql.org/docs/19/transaction-iso.html
[^31]: MERGE. https://www.postgresql.org/docs/19/sql-merge.html
[^32]: JSON Types (jsonb indexing, operator classes). https://www.postgresql.org/docs/19/datatype-json.html
[^33]: Table Partitioning. https://www.postgresql.org/docs/19/ddl-partitioning.html
[^34]: CREATE INDEX (CONCURRENTLY, partitioned tables, ON ONLY). https://www.postgresql.org/docs/19/sql-createindex.html
[^35]: Using EXPLAIN. https://www.postgresql.org/docs/19/using-explain.html
[^36]: PgBouncer — SQL feature map for pooling modes. https://www.pgbouncer.org/features.html
[^37]: PgBouncer changelog (1.21.0 prepared statements in transaction mode; 1.24.0 max_prepared_statements default 200). https://www.pgbouncer.org/changelog.html
[^38]: PostgreSQL 14, 15, 16 and 17 Release Notes. https://www.postgresql.org/docs/14/release-14.html , https://www.postgresql.org/docs/15/release-15.html , https://www.postgresql.org/docs/16/release-16.html , https://www.postgresql.org/docs/17/release-17.html
[^39]: PostgreSQL Wiki — Don't Do This (timestamp, timetz, char(n), money, NOT IN, BETWEEN). https://wiki.postgresql.org/wiki/Don%27t_Do_This
[^40]: Schemas — The Public Schema and Usage Patterns. https://www.postgresql.org/docs/19/ddl-schemas.html
[^41]: Routine Vacuuming (autovacuum, partitioned tables, wraparound). https://www.postgresql.org/docs/19/routine-vacuuming.html
[^42]: Date/Time Types. https://www.postgresql.org/docs/19/datatype-datetime.html
[^43]: CREATE VIEW (security_invoker, security_barrier). https://www.postgresql.org/docs/19/sql-createview.html
[^44]: libpq — Connection Strings (sslmode, min/max_protocol_version, sslkeylogfile, servicefile). https://www.postgresql.org/docs/19/libpq-connect.html
[^45]: UUID Functions (uuidv7, uuidv4, uuid_extract_timestamp). https://www.postgresql.org/docs/19/functions-uuid.html
[^46]: Generated Columns. https://www.postgresql.org/docs/19/ddl-generated-columns.html
[^47]: Returning Data from Modified Rows (OLD/NEW). https://www.postgresql.org/docs/19/dml-returning.html
[^48]: CREATE TABLE (WITHOUT OVERLAPS, PERIOD, NOT ENFORCED, generated columns). https://www.postgresql.org/docs/19/sql-createtable.html
[^49]: Multicolumn Indexes (skip scan). https://www.postgresql.org/docs/19/indexes-multicolumn.html
[^50]: pg_upgrade. https://www.postgresql.org/docs/19/pgupgrade.html
