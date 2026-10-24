### PostgreSQL 14: The Complete Review Guide

#### Overview

> **Version**: PostgreSQL 14.0 was released on **September 30, 2021**.[^1][^2] The latest minor release is **14.24 (August 13, 2026)**.[^4] The final 14.x minor release is scheduled for **November 12, 2026**, the same day as the next quarterly update of all branches.[^2][^3]
> **Support status (October 2026)**: **PostgreSQL 14 reaches end of life on November 12, 2026, within weeks.** After that date the community publishes no more bug or security fixes for 14.x.[^2][^32] Treat any production system on 14 as an upgrade project that is already due. PostgreSQL 13 has been end of life since November 13, 2025 (last release 13.23).[^2][^11]
> **Newest releases**: PostgreSQL **18** is the current stable major version (18.0 on September 25, 2025; current minor 18.6). 17 (17.11), 16 (16.15) and 15 (15.19) are also supported. PostgreSQL **19 is at Release Candidate 1** (October 15, 2026; RC 2 and general availability are planned for November 12 and November 19, 2026). It is pre-release software and not a deployment target.[^2][^34]
> **Scope**: SQL that runs on a PostgreSQL 14 server: schema migrations, queries embedded in application code, functions, procedures and triggers, schema design, indexing, transactions, security, JSON, partitioning, connection pooling and operations-facing SQL. The guide is also the one to use for projects pinned to **PostgreSQL 13 or older**: every feature that is new in 14 is marked **New in 14** and must not be used on 13.
> **Note**: This document is self-contained. It carries forward everything from older releases that is still correct, and it marks the version in which a feature arrived when that matters. Every example was executed on PostgreSQL 14.24 (official Docker image). "New in 14" claims were shown to fail (or behave differently) on 13.23, and every item on the "Do not use" list was shown to fail on 14.24; 15-specific behaviour changes were confirmed on 15.19.

PostgreSQL 14 brought **multirange types**, the SQL-standard **`SEARCH` and `CYCLE` clauses for recursive CTEs**, **subscripting for `jsonb`** (`doc['a']['b']`, also in `UPDATE ... SET doc['a'] = ...`), **`OUT` parameters for procedures**, **SQL-standard function bodies** (`BEGIN ATOMIC ... END`, `RETURN expr`), the functions **`date_bin()`**, **`string_to_table()`**, **`trim_array()`**, **`bit_count()`**, **`unistr()`** and **`bit_xor()`**, negative positions in **`split_part()`**, **`GROUP BY DISTINCT`**, **aliases for `JOIN ... USING`**, **`CREATE OR REPLACE TRIGGER`**, the predefined roles **`pg_read_all_data`**, **`pg_write_all_data`** and **`pg_database_owner`**, **`scram-sha-256` as the default `password_encryption`**, **`idle_session_timeout`** and **`client_connection_check_interval`**, **`REINDEX` on partitioned tables (also `CONCURRENTLY`)**, **`ALTER TABLE ... DETACH PARTITION ... CONCURRENTLY`**, **LZ4 compression for TOAST**, **extended statistics on expressions**, the **Memoize** plan node, **`compute_query_id`** (query IDs in `pg_stat_activity` and `EXPLAIN VERBOSE`), the views **`pg_stat_wal`**, **`pg_stat_replication_slots`**, **`pg_stat_progress_copy`** and **`pg_backend_memory_contexts`**, and **pipeline mode in libpq**.[^1]

PostgreSQL 14 also removed things that old code may use: **postfix operators** (including the factorial operators `!` and `!!`), the geometric containment operators `@` and `~`, the boolean-like values of `password_encryption`, the version 2 wire protocol, SSL compression, and the settings `vacuum_cleanup_index_scale_factor` and `operator_precedence_warning`. **`EXTRACT()` now returns `numeric`** instead of `double precision`, and `factorial()` of a negative number now raises an error.[^1]

Global truths about PostgreSQL 14:

- **MVCC.** Readers never block writers and writers never block readers at the row level. `UPDATE` writes a new row version and `DELETE` marks the old one dead. `VACUUM` (usually autovacuum) reclaims dead versions. Long transactions stop that reclamation everywhere in the cluster.
- **DDL is transactional, but it takes locks.** Almost every `ALTER TABLE` form takes an `ACCESS EXCLUSIVE` lock. The lock is held until commit, and it queues behind running queries while blocking new ones (see [Schema Migrations and Locking](#schema-migrations-and-locking)).
- **Default isolation is READ COMMITTED.** Each statement sees a fresh snapshot. A read-modify-write in application code is a lost update unless it is made atomic.
- **NULL is three-valued.** `NULL = NULL` is NULL, `NOT IN` with a NULL in the list never returns true, and `count(col)` skips NULLs.
- **Identifiers fold to lower case** unless they are double-quoted. String literals use single quotes.
- **Arrays are 1-based.** `arr[0]` is NULL, not an error. (jsonb subscripts are 0-based; see [jsonb subscripting](#new-in-14-jsonb-subscripting).)
- **Use `timestamptz`, `text`, `bigint` identity and `numeric` for money** by default. The rest of this guide explains why.

***

#### Version-Aware Review

The project pins PostgreSQL 14 (for example `image: postgres:14`, `postgresql 14.x` in `.tool-versions`, a managed-database engine version of 14, or a CI service container). Apply these rules when reviewing code for it:

- **Verify before you claim something is unavailable.** A feature you remember as "new" may already be in 14 (for example jsonb subscripting, multiranges, `date_bin()`, `string_to_table()`, `SEARCH`/`CYCLE`, `CREATE OR REPLACE TRIGGER`, procedure `OUT` parameters, `BEGIN ATOMIC` function bodies, `pg_read_all_data`). Say "not available in PostgreSQL 14" only for items on the [Do Not Use or Recommend](#do-not-use-or-recommend-postgresql-15-features) list or when the 14 documentation proves it.
- **Never suggest PostgreSQL 15 or newer features.** The most commonly suggested ones are `MERGE`, `UNIQUE NULLS NOT DISTINCT`, `security_invoker` views, `regexp_count()`/`regexp_like()`/`regexp_substr()`/`regexp_instr()`, `ON DELETE SET NULL (col)`, `GRANT SET ON PARAMETER`, the `pg_checkpoint` role, `log_destination = jsonlog`, `pg_backup_start()`, the SQL/JSON functions (`JSON_OBJECT`, `IS JSON`, `JSON_TABLE`, `JSON_VALUE`), `any_value()`, `pg_input_is_valid()`, `transaction_timeout` and `uuidv7()`. They fail on 14 with a syntax error, "does not exist" or "unrecognized parameter".
- **Watch for silent misparses.** On 14, `SELECT 1_000_000` returns **1** in a column named `_000_000`, and `SELECT 0x1F` returns **0** in a column named `x1f`. The lexer reads a number followed by an identifier as `number AS identifier`. Code written for 16+ (underscored or hexadecimal literals) does not fail on 14; it computes the wrong value. 15 turned this into an error (`trailing junk after numeric literal`).
- **Flag code that assumes newer defaults.** On 14, every role can create objects in the `public` schema (`PUBLIC` has `CREATE` there), a login role can grant its own role to others, `hash_mem_multiplier` is 1.0, and checkpoints are not logged by default. Code and runbooks written against 15+ defaults can be wrong or insecure on 14.
- **The 13 boundary.** If the project pins 13 or older, or the code must also run there (a library, an extension, a shared migration set), every **New in 14** feature in this guide is a bug: on 13.23, jsonb subscripting fails with `cannot subscript type jsonb because it is not an array`, `CREATE PROCEDURE ... OUT` fails with `procedures cannot have OUT arguments`, `BEGIN ATOMIC` and `RETURN expr` bodies are syntax errors, `REINDEX ... CONCURRENTLY` on a partitioned table fails with `REINDEX is not yet implemented for partitioned indexes`, and multirange types do not exist. 13 is out of support since November 13, 2025; a 13 pin is itself a finding.
- **14 is at end of life now.** Reviews of infrastructure code (Terraform, Helm values, Dockerfiles, CI service containers) that pin 14 must flag the November 12, 2026 end of life and the upgrade path (see [Upgrading Off 14](#upgrading-off-postgresql-14)). Do not accept "we will stay on 14 for now" for internet-facing or regulated systems: 14.24 alone fixed 28 CVEs, and later fixes will not be backported.
- **Client libraries are separate.** libpq pipeline mode (14) needs a 14+ libpq on the client; the server version does not matter. libpq 16 options such as `sslrootcert=system`, `require_auth` and `load_balance_hosts` depend on the client library, not on the 14 server. Do not recommend them unless the client side is verified.

***

#### What's New in PostgreSQL 14

All items below are taken from the official release notes.[^1] Items that are SQL-visible were executed on 14.24 and shown to fail or behave differently on 13.23.

##### SQL and data types

| Feature | Notes |
|---|---|
| Multirange types (`int4multirange`, `int8multirange`, `nummultirange`, `datemultirange`, `tsmultirange`, `tstzmultirange`) | Created automatically for every range type. `range_agg(range)` aggregates ranges into a multirange. (`range_agg` over multiranges is 15.) |
| `SEARCH { DEPTH \| BREADTH } FIRST BY ... SET col` and `CYCLE col SET flag USING path` | SQL-standard clauses for `WITH RECURSIVE`. |
| jsonb subscripting | `doc['a']['b']`, `UPDATE t SET doc['a'] = '1'`. Also `hstore` subscripting. Extension types can implement subscripting. |
| `OUT` parameters for procedures | `CALL` must pass a placeholder (usually `NULL`) for each `OUT` argument. |
| SQL-standard function bodies | `LANGUAGE sql BEGIN ATOMIC ... END` and `RETURN expr`. Parsed at creation time, with dependency tracking. |
| `date_bin(stride, source, origin)` | Fixed-length time buckets. Strides in months or years are rejected. |
| `string_to_table(text, delimiter [, null_string])` | Set-returning counterpart of `string_to_array()`. |
| `trim_array(array, n)`, `bit_count()`, `bit_xor()`, `unistr()`, `ltrim()`/`rtrim()` for `bytea` | |
| `split_part(s, delim, -1)` | Negative positions count from the end. On 13: `field position must be greater than zero`. |
| `substring(s SIMILAR pattern ESCAPE esc)` | SQL-standard spelling of the regex `substring()`. |
| `GROUP BY DISTINCT` | Removes duplicate grouping sets produced by `ROLLUP`/`CUBE`/`GROUPING SETS` combinations. |
| `JOIN ... USING (cols) AS alias` | The alias qualifies the merged columns. |
| `numeric` `Infinity` and `-Infinity` | Accepted as input; on 13 `invalid input syntax for type numeric: "Infinity"`. |
| `pg_lsn` + / - `numeric` | Byte arithmetic on LSNs. |
| `CURRENT_ROLE` wherever `CURRENT_USER` is accepted; `GRANT ... GRANTED BY` | |
| `CREATE OR REPLACE TRIGGER` | |
| `array_append()`, `array_cat()`, `array_position()`, ... take `anycompatiblearray` | `array_append(ARRAY[1,2], 3::bigint)` works on 14, fails on 13. |
| More keywords usable as column labels without `AS` | |

##### Privileges, security and authentication

- **`pg_read_all_data`** (read every table, view and sequence, with implicit `USAGE` on all schemas) and **`pg_write_all_data`** (`INSERT`/`UPDATE`/`DELETE` everywhere). Both still obey row-level security unless the role also has `BYPASSRLS`.[^18]
- **`pg_database_owner`**: a predefined role whose only member is the owner of the current database. On 14 nothing uses it by default; 15 makes it the owner of `public` in new databases.
- **`password_encryption` defaults to `scram-sha-256`.** New passwords are stored as SCRAM verifiers (`rolpassword` starts with `SCRAM-SHA-256$`). The old boolean-like values are rejected (`SET password_encryption = on` fails with `invalid value ... Available values: md5, scram-sha-256`).
- `pg_hba.conf`: `clientcert=verify-ca|verify-full` only (1/0/no-verify are gone), `clientname=DN` for certificate DN matching, lines can be continued with a trailing backslash. `ssl_crl_dir`/`sslcrldir`. Passwords of arbitrary length. SSL compression and the version 2 wire protocol are removed.
- Custom setting names must be valid identifiers: `SET "my-app.x" = 1` fails on 14 with `invalid configuration parameter name "my-app.x"`.

##### Server, performance and operations

- **Vacuum**: index vacuuming is skipped when few index entries are removable (`INDEX_CLEANUP` gains the default `auto`), a wraparound **failsafe** (`vacuum_failsafe_age`, `vacuum_multixact_failsafe_age`) makes vacuum skip inessential work near wraparound, `VACUUM (PROCESS_TOAST false)`, `vacuum_cost_page_miss` default lowered from 10 to 2.
- **B-tree bottom-up index deletion** removes expired index entries before page splits, reducing bloat on indexes of frequently updated columns.
- **BRIN** `minmax_multi` and `bloom` operator classes (`int4_minmax_multi_ops`, `int4_bloom_ops`, ...). **SP-GiST** supports `INCLUDE` columns. GiST indexes can be built by presorting.
- **Planner/executor**: Memoize nodes for parameterized nested loops (`enable_memoize`), hash lookups for long `IN (const, ...)` lists, TID range scans (`ctid >= '(1,0)' AND ctid < '(2,0)'` is a `Tid Range Scan`), incremental sort for window functions, extended statistics on expressions (`CREATE STATISTICS s ON (a + b), (a * b) FROM t`), faster `UPDATE`/`DELETE` on partitioned tables with many partitions, parallel `REFRESH MATERIALIZED VIEW`, asynchronous foreign scans for `postgres_fdw`.
- **Snapshot scalability**: computing MVCC snapshots is much cheaper with many connections, including many idle ones.
- **`checkpoint_completion_target` defaults to 0.9** (0.5 in 13).
- **New settings**: `idle_session_timeout`, `client_connection_check_interval`, `compute_query_id`, `default_toast_compression`, `remove_temp_files_after_crash`, `huge_page_size`, `min_dynamic_shared_memory`, `log_recovery_conflict_waits`, `recovery_init_sync_method`, `in_hot_standby` (read-only), `enable_memoize`, `track_wal_io_timing` (fills `pg_stat_wal.wal_write_time`/`wal_sync_time`), `debug_discard_caches`.
- **Removed settings**: `vacuum_cleanup_index_scale_factor`, `operator_precedence_warning` (`SHOW` fails with `unrecognized configuration parameter`).
- **TOAST compression**: `COMPRESSION lz4` per column and `default_toast_compression` (default still `pglz`).
- **Monitoring**: `pg_stat_wal`, `pg_stat_replication_slots`, `pg_stat_progress_copy`, `pg_backend_memory_contexts`, `pg_log_backend_memory_contexts(pid)`, session statistics in `pg_stat_database` (`sessions`, `session_time`, `active_time`, `idle_in_transaction_time`, `sessions_abandoned`, ...), `pg_locks.waitstart`, `pg_prepared_statements.generic_plans`/`custom_plans`, `pg_stat_activity.query_id`, the archiver in `pg_stat_activity`, `pg_stats_ext_exprs`, `pg_stat_statements` 1.9 (`toplevel` column, `pg_stat_statements_info`).
- **Utility**: `REINDEX` on partitioned tables and indexes (also `CONCURRENTLY`), `REINDEX (TABLESPACE ...)`, `ALTER TABLE ... DETACH PARTITION ... CONCURRENTLY` / `FINALIZE`, `pg_terminate_backend(pid, timeout)`, `TRUNCATE` on foreign tables, `ALTER SUBSCRIPTION ... ADD/DROP PUBLICATION`.
- **Logical replication**: streaming of large in-progress transactions (`CREATE SUBSCRIPTION ... WITH (streaming = on)`), binary transfer (`binary = true`), two-phase commit support in the decoding API.
- **Recovery and replication**: `pg_rewind` can use a standby as source, `restore_command` reloadable, recovery pauses instead of shutting down when the primary changes incompatible parameters.

##### Client side and tools

- **libpq pipeline mode** (`PQenterPipelineMode()`, `PQpipelineSync()`, ...). Only the extended query protocol works in a pipeline: pgbench 14 aborted a `\startpipeline` script in simple query mode with `cannot use pipeline mode with the simple query protocol` and ran it with `-M extended`.[^26]
- libpq `target_session_attrs` gains `read-only`, `primary`, `standby`, `prefer-standby`; Server Name Indication (`sslsni`) is sent by default.
- psql: `\dX` lists extended statistics, `\df`/`\do` accept argument types, `\e` with no changes no longer executes the buffer.
- `pg_amcheck` (new client), `pg_dump --extension`, `vacuumdb --no-index-cleanup --no-truncate`, pgbench `permute()` and pipeline meta-commands, `pg_upgrade` no longer writes `analyze_new_cluster` scripts.
- New contrib modules `pg_surgery` and `old_snapshot`; `amcheck` checks heap pages; `postgres_fdw` bulk inserts and `keep_connections`.

***

#### Do Not Use or Recommend (PostgreSQL 15+ Features)

The server is PostgreSQL 14. Everything in these tables was introduced in a later release and was shown to fail on 14.24 with the error given. Never suggest these, and flag them as bugs in code that targets 14.

##### PostgreSQL 15 (released October 13, 2022)[^7]

| PostgreSQL 15 feature | What happens on 14.24 | Use on 14 instead |
|---|---|---|
| `MERGE INTO ... USING ... WHEN [NOT] MATCHED` | `syntax error at or near "MERGE"` | `INSERT ... ON CONFLICT ... DO UPDATE` for upserts; separate `UPDATE`/`DELETE`/`INSERT` statements in one transaction for synchronisation |
| `UNIQUE NULLS NOT DISTINCT`, `CREATE UNIQUE INDEX ... NULLS NOT DISTINCT` | `syntax error at or near "NULLS"` | Partial unique index for the NULL case (`... (a) WHERE b IS NULL`) or a `coalesce()` expression index |
| `CREATE VIEW v WITH (security_invoker = true)` | `unrecognized parameter "security_invoker"` | See [Views and RLS on 14](#views-and-row-level-security-on-14) |
| `regexp_count()`, `regexp_instr()`, `regexp_like()`, `regexp_substr()`, `regexp_replace(s, p, r, start [, N])` | `function regexp_count(unknown, unknown) does not exist` | `s ~ p`, `(regexp_match(s, p))[1]`, `substring(s from p)`, `(SELECT count(*) FROM regexp_matches(s, p, 'g'))` |
| `ON DELETE SET NULL (col)` / `SET DEFAULT (col)` | `syntax error at or near "("` | A `BEFORE DELETE` trigger on the parent, or a non-composite key |
| `numeric(p, s)` with negative scale or scale > precision | `NUMERIC scale -2 must be between 0 and precision 4` | `round(x, -2)` explicitly |
| `COPY ... (FORMAT text, HEADER)`, `HEADER match` | `COPY HEADER available only in CSV mode`; `header requires a Boolean value` | CSV format; verify header columns in the client |
| `CREATE UNLOGGED SEQUENCE`, `ALTER TABLE ... SET ACCESS METHOD` | `unlogged sequences are not supported`; syntax error | |
| `CLUSTER` on a partitioned table | `cannot cluster a partitioned table` | Cluster each partition |
| `range_agg(multirange)`, `min()`/`max()` on `xid8` | `function range_agg(int4multirange) does not exist` | `range_agg()` over `unnest(multirange)` |
| `GRANT SET ON PARAMETER`, `GRANT ALTER SYSTEM ON PARAMETER`, `has_parameter_privilege()` | `syntax error at or near "work_mem"` | `ALTER ROLE ... SET` / `ALTER FUNCTION ... SET` for per-role or per-function settings |
| Predefined role `pg_checkpoint` | `role "pg_checkpoint" does not exist` | `CHECKPOINT` needs superuser on 14 |
| `log_destination = 'jsonlog'` | `invalid value for parameter "log_destination": "jsonlog"` | `csvlog` |
| `wal_compression = lz4 / zstd` | `parameter "wal_compression" requires a Boolean value` | `wal_compression = on` (pglz) |
| `pg_backup_start()`, `pg_backup_stop()` | function does not exist | `pg_start_backup(label, fast, false)` / `pg_stop_backup(false)` (non-exclusive mode) |
| ICU as database default (`CREATE DATABASE ... LOCALE_PROVIDER icu`), `pg_database.datcollversion`, `REFRESH COLLATION VERSION` for databases | `option "locale_provider" not recognized`; `column "datcollversion" does not exist` | libc default collation; ICU collations per column |
| `recovery_prefetch`, `log_startup_progress_interval`, `stats_fetch_consistency`, `shared_memory_size`, `recursive_worktable_factor`, `archive_library` | `unrecognized configuration parameter` | |
| `pg_stat_subscription_stats`, `pg_ident_file_mappings`, `pg_walinspect`, `pg_stat_statements` 1.10 columns (`temp_blk_read_time`, `jit_*`) | relation/column does not exist | `pg_stat_subscription`, `pg_stat_statements` 1.9 |
| `starts_with()` / `^@` using a B-tree index | Not an error, but planned as `Seq Scan ... Filter: starts_with(...)` | `col LIKE 'prefix%'` with a `C`-collation column or a `text_pattern_ops` index |
| `ALTER SUBSCRIPTION ... SKIP`, `disable_on_error`, row filters (`FOR TABLE t WHERE ...`), column lists (`FOR TABLE t (a, b)`), `FOR TABLES IN SCHEMA` | `syntax error at or near "WHERE"` / `"("` / `"TABLES"` | Publish whole tables only; keep data that must not leave the cluster in tables that are not published |

##### PostgreSQL 16 (released September 14, 2023)[^8]

| PostgreSQL 16 feature | What happens on 14.24 |
|---|---|
| Subquery in `FROM` without an alias: `SELECT * FROM (SELECT 1)` | `subquery in FROM must have an alias` |
| `JSON_OBJECT('a': 1)`, `JSON_ARRAY()`, `JSON_OBJECTAGG()`, `JSON_ARRAYAGG()` | `syntax error at or near ":"` / function does not exist. Use `jsonb_build_object()`, `jsonb_build_array()`, `jsonb_object_agg()`, `jsonb_agg()` |
| `x IS JSON [OBJECT \| ARRAY \| SCALAR]` | `syntax error at or near "JSON"` |
| `pg_input_is_valid()`, `pg_input_error_info()` | function does not exist |
| `any_value()` | function does not exist. Use `min()`/`max()` |
| Numeric literals with underscores `1_000_000`, non-decimal literals `0x1F`, `0o17`, `0b101` | **No error on 14**: `SELECT 1_000_000` returns `1` (column `_000_000`), `SELECT 0x1F` returns `0` (column `x1f`) |
| `array_sample()`, `array_shuffle()`, `random_normal()`, `erf()`, `erfc()` | function does not exist |
| `date_add()` / `date_subtract()` with a time zone, `generate_series(..., timezone)` | function does not exist |
| `SYSTEM_USER` | `column "system_user" does not exist` |
| `pg_stat_io`, `last_seq_scan`, `last_idx_scan` | relation/column does not exist |
| `GRANT r TO u WITH INHERIT FALSE`, `WITH SET FALSE`, `WITH ADMIN TRUE` | `syntax error at or near "INHERIT"` |
| `pg_create_subscription`, `pg_use_reserved_connections` roles | role does not exist |
| `COPY ... (DEFAULT '\D')` | `option "default" not recognized` |
| `VACUUM (BUFFER_USAGE_LIMIT ...)`, `PROCESS_MAIN`, `SKIP_DATABASE_STATS` | `unrecognized VACUUM option "buffer_usage_limit"` |
| `EXPLAIN (GENERIC_PLAN)` | `there is no parameter $1` |
| `CREATE STATISTICS ON a, b FROM t` (no name) | `syntax error at or near "ON"` |
| `debug_parallel_query` | unrecognized parameter (14 calls it `force_parallel_mode`) |
| `date_trunc(unit, timestamptz, zone)` in an index expression | The function exists on 14 but is `STABLE`: `functions in index expression must be marked IMMUTABLE` |
| libpq `require_auth`, `load_balance_hosts`, `sslrootcert=system` | Need libpq 16+ on the client |

##### PostgreSQL 17 (released September 26, 2024)[^9]

| PostgreSQL 17 feature | What happens on 14.24 |
|---|---|
| `JSON_TABLE()`, `JSON_VALUE()`, `JSON_QUERY()`, `JSON_EXISTS()` | `function json_value(jsonb, unknown) does not exist` / syntax error |
| `MERGE ... RETURNING`, `WHEN NOT MATCHED BY SOURCE` | No `MERGE` at all on 14 |
| `COPY ... (ON_ERROR ignore)` | `option "on_error" not recognized` |
| `random(min, max)` | function does not exist. Use `min + floor(random() * (max - min + 1))::int` |
| `transaction_timeout` | `unrecognized configuration parameter "transaction_timeout"` |
| `GRANT MAINTAIN`, `pg_maintain` | `unrecognized privilege type "maintain"` |
| `ts AT LOCAL` | `syntax error at or near "LOCAL"` |
| `pg_stat_checkpointer` | relation does not exist (use `pg_stat_bgwriter` on 14) |
| `ALTER TABLE ... SET EXPRESSION AS (...)`, `SET STATISTICS DEFAULT` | syntax error (use `SET STATISTICS -1`) |
| Exclusion constraints on partitioned tables | `exclusion constraints are not supported on partitioned tables` |
| `uuid_extract_timestamp()`, builtin collation provider, `EXPLAIN (SERIALIZE, MEMORY)`, incremental backup | Not in 14 |

##### PostgreSQL 18 (released September 25, 2025)[^10] and 19 (RC1)

| Feature | What happens on 14.24 |
|---|---|
| `uuidv7()`, `uuidv4()` | function does not exist. Use `gen_random_uuid()` (v4, core since 13) or generate v7 in the application |
| Virtual generated columns (`GENERATED ALWAYS AS (expr)` without `STORED`) | `syntax error at or near ")"` |
| `RETURNING old.*, new.*` | `missing FROM-clause entry for table "old"` |
| `WITHOUT OVERLAPS`, `NOT ENFORCED`, `NOT NULL ... NOT VALID`, `VACUUM ONLY` | syntax errors |
| `casefold()`, `array_sort()`, `array_reverse()`, `crc32()` | function does not exist |
| B-tree skip scan | Not in 14: an index on `(a, b)` does not serve `WHERE b = 500` (measured `Seq Scan`) |
| `GROUP BY ALL`, `REPACK` (19) | syntax errors |

`JSON('...')` parses on 14 only as a function-style cast to the `json` type; it is not the SQL/JSON constructor.

##### Removed or changed in 14 (migration from 13)

| Removed / changed in 14 | Effect on 14.24 | Replacement |
|---|---|---|
| Postfix operators (`CREATE OPERATOR` with only `LEFTARG`) | `operator right argument type must be specified` / `Postfix operators are not supported` | Prefix operator or a function. `pg_dump`/`pg_upgrade` warn about postfix operators in old databases |
| Factorial operators `5!` and `!!5`, `numeric_fac()` | `syntax error at or near ";"`; `operator does not exist: !! integer` | `factorial(5)` |
| `factorial(-1)` returned 1 | `factorial of a negative number is undefined` | Validate input |
| Geometric/contrib containment `@` and `~` | `operator does not exist: point @ box` | `<@` and `@>` |
| `EXTRACT()` returned `double precision` | Returns `numeric` (`extract(epoch from ...)` is `1704067200.123456`) | Drivers may now decode `Decimal`/`BigDecimal`; `date_part()` still returns `double precision` |
| `EXTRACT(hour FROM date)` returned 0 | `date units "hour" not supported` | Cast to `timestamp` first if you mean midnight |
| `password_encryption = on/true/yes` | `invalid value for parameter "password_encryption": "on"` | `scram-sha-256` (default) or `md5` |
| `password_encryption` default `md5` | Default `scram-sha-256` | Clients must support SCRAM (libpq 10+, current JDBC, npgsql, ...) |
| `vacuum_cleanup_index_scale_factor`, `operator_precedence_warning` | `unrecognized configuration parameter` | Remove from `postgresql.conf` |
| `array_append()` etc. declared on `anyarray` | Now `anycompatiblearray` | User-defined aggregates/operators that reference the old signatures must be dropped before and recreated after the upgrade |
| `to_tsquery('pg_class <-> pg')` gave `( 'pg' & 'class' ) <-> 'pg'` | `'pg' <-> 'class' <-> 'pg'` | Re-check stored `tsquery` values and tests |
| `\w` as a range endpoint in a bracket expression (`'[\w-z]'`) | `invalid regular expression: invalid character range` (13 accepted it) | Put `-` first or last (`[-\wz]` matched both `-` and `q`) |
| `\D`, `\W` did not match newline in newline-sensitive regex mode | They now match newlines (`E'a\nb' ~ '(?n)a\Wb'` is true on 14, false on 13) | `[^[:digit:]]`, `[^[:word:]]` for the old behaviour |
| `var_samp()`/`stddev_samp()` of a single `numeric` NaN returned NaN | Return NULL | |
| Custom setting names with arbitrary characters | Must be identifiers separated by dots | `myapp.tenant_id` style names |
| `CREATE LANGUAGE 'plpgsql'` (quoted name) | syntax error | Unquoted name |
| Composite types for sequences and TOAST tables | `type "sq" does not exist` | |
| `clientcert=1/0/no-verify` in `pg_hba.conf` | Rejected at reload | `verify-ca` / `verify-full` |
| contrib `intarray` `<@`/`@>` using GiST indexes | No longer use the index (heap scan is faster) | Drop indexes created only for that |
| `pg_standby` | Removed | `restore_command` + `primary_conninfo` |

##### Still present in 14 but removed or changed later (do not "fix" them prematurely, but plan for them)

- **Removed in 15**: `pg_start_backup()`/`pg_stop_backup()`/`pg_is_in_backup()`/`pg_backup_start_time()` and the **exclusive backup mode**, `stats_temp_directory`, PL/Python 2 (`plpythonu`, `plpython2u`). Measured on 14.24: `pg_start_backup('x')` (exclusive) still works and `pg_is_in_backup()` returns `t`; on 15.19 `pg_start_backup` does not exist.
- **Changed in 15**: `PUBLIC` loses `CREATE` on `public` in new databases; numeric literals followed by identifiers (`123abc`, `1_000`) become errors; `interval::text` is no longer immutable (an index on `(i::text)` builds on 14.24 and fails on 15.19); `interval '1.99 years'` is `1 year 11 mons` on 14 and `2 years` on 15; `chr(-1)` raises `requested character too large for encoding: -1` on 14 and `character number must be positive` on 15; `setseed(0.5); random()` gives `0.2499104186659835` on 13 and 14, `0.985167...` on 15; `hash_mem_multiplier` default 1.0 → 2.0; `log_checkpoints` off → on; `log_autovacuum_min_duration` -1 → 10min; a login role loses implicit `ADMIN OPTION` on itself; libpq's `PQsendQuery()` is no longer allowed in pipeline mode.
- **Removed in 16**: `force_parallel_mode` (renamed `debug_parallel_query`), `vacuum_defer_cleanup_age`, `promote_trigger_file`, `SHOW lc_collate`/`lc_ctype`, manual `ON SELECT` rules; `CREATEROLE` becomes much weaker.
- **Removed in 17**: `old_snapshot_threshold`, `adminpack`, `pg_stat_bgwriter.checkpoints_*`/`buffers_backend` (moved to `pg_stat_checkpointer`/`pg_stat_io`), `pg_stat_statements.blk_read_time` (renamed `shared_blk_read_time`). Maintenance commands run with `search_path = pg_catalog, pg_temp`.

Already gone before 14 and still common in old code: `pg_stat_statements.total_time` (renamed `total_exec_time` in 13; `column "total_time" does not exist` on 14), `recovery.conf` (merged into `postgresql.conf` in 12), `WITH OIDS` (removed in 12), `abstime`/`reltime` (removed in 12).

***

#### Features from PostgreSQL 11–13 You Can Rely On

All of these exist on 14 and were executed on 14.24.

- `ADD COLUMN ... DEFAULT <non-volatile>` without a table rewrite (11).
- Covering indexes `INCLUDE (...)` for B-tree (11) and GiST (12); SP-GiST gains it in 14.
- Stored generated columns `GENERATED ALWAYS AS (...) STORED` (12).
- CTE inlining (12): single-reference, side-effect-free CTEs are inlined; `AS MATERIALIZED` / `AS NOT MATERIALIZED` override it.
- `REINDEX ... CONCURRENTLY` (12), `CREATE INDEX CONCURRENTLY` (8.2).
- jsonpath (`@?`, `@@`, `jsonb_path_query()`, `jsonb_path_exists()`) (12), `jsonb_set_lax()` (13).
- `gen_random_uuid()` in core (13); B-tree deduplication (13); incremental sort (13); `FETCH FIRST ... WITH TIES` (13); `pg_stat_progress_analyze` and `pg_stat_progress_basebackup` (13).
- Partitioning: foreign keys referencing partitioned tables (12), `ATTACH PARTITION` with only `SHARE UPDATE EXCLUSIVE` on the parent (12), row-level `BEFORE` triggers on partitioned tables (13), logical replication of partitioned tables (13).
- `websearch_to_tsquery()` (11), procedures with transaction control (`CREATE PROCEDURE`, `COMMIT` inside, 11).

***

#### New in 14: Multirange Types

Every range type has a multirange type: an ordered set of non-overlapping, non-adjacent ranges.[^12]

```sql
SELECT '{[1,3),[5,8)}'::int4multirange;                          -- {[1,3),[5,8)}
SELECT int4multirange(int4range(1,3), int4range(2,6));           -- {[1,6)}   (normalized)
SELECT '{[1,3),[5,8)}'::int4multirange @> 6;                     -- t
SELECT range_agg(r)
FROM (VALUES (int4range(1,3)), (int4range(2,5)), (int4range(7,9))) v(r);   -- {[1,5),[7,9)}
SELECT '{[2024-01-01,2024-01-05)}'::datemultirange
     - '{[2024-01-02,2024-01-03)}'::datemultirange;
-- {[2024-01-01,2024-01-02),[2024-01-03,2024-01-05)}
```

- Range subtraction that would split a range raises an error for plain ranges (`result of range difference would not be contiguous`); multiranges represent the gap. Use them for availability calendars, opening hours and "free slots" computations instead of hand-written interval merging.
- `range_agg()` on 14 accepts **ranges only**; `range_agg(multirange)` is 15. `unnest(multirange)` returns the component ranges.
- GiST indexes support multirange operators (`&&`, `@>`, `<@`). An exclusion constraint still works on the underlying range column.
- Creating a range type creates a multirange type (`CREATE TYPE ... AS RANGE (..., multirange_type_name = ...)`). 14.23 fixed a missing `CREATE` privilege check on the multirange's schema (CVE-2026-6472).[^5]
- On 13: `type "int4multirange" does not exist`, `function range_agg(int4range) does not exist`.

***

#### New in 14: SEARCH and CYCLE in Recursive CTEs

```sql
CREATE TABLE edge(src int, dst int);
INSERT INTO edge VALUES (1,2), (2,3), (3,1);          -- contains a cycle

WITH RECURSIVE walk(n) AS (
  SELECT 1
  UNION ALL
  SELECT e.dst FROM walk w JOIN edge e ON e.src = w.n
) CYCLE n SET is_cycle USING path
SELECT n, is_cycle, path FROM walk;
--  1 | f | {(1)}
--  2 | f | {(1),(2)}
--  3 | f | {(1),(2),(3)}
--  1 | t | {(1),(2),(3),(1)}          <- recursion stops here

WITH RECURSIVE t(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM t WHERE n < 3)
  SEARCH BREADTH FIRST BY n SET ord
SELECT * FROM t ORDER BY ord;          -- deterministic breadth-first order
```

- Every recursive CTE over data that users or imports control needs cycle protection: `CYCLE`, a depth counter with a limit, or `UNION` (not `UNION ALL`) when rows are fully identical. Without it, one bad edge produces an endless query that only `statement_timeout` stops.
- `SEARCH DEPTH FIRST` / `BREADTH FIRST` produce a column to `ORDER BY`. Without `ORDER BY`, the output order of a recursive CTE is not guaranteed.
- The `path` column is an array of row values; it grows with depth. For deep graphs a depth limit is cheaper.
- On 13 both clauses are syntax errors. The portable form carries an explicit path array: `SELECT e.dst, w.path || e.dst FROM walk w JOIN edge e ON e.src = w.n WHERE NOT e.dst = ANY(w.path)` (on the graph above it stops after `{1,2,3}`).[^13]

***

#### New in 14: jsonb Subscripting

```sql
SELECT ('{"a":{"b":1}}'::jsonb)['a']['b'];        -- 1          (result type jsonb)
SELECT ('[1,2,3]'::jsonb)[-1];                    -- 3          (jsonb arrays are 0-based; negative counts from the end)
SELECT ('{"a":"x"}'::jsonb)['a'];                 -- "x"        (jsonb string, with quotes)
SELECT ('{"a":"x"}'::jsonb)['a']::text;           -- "x"        (still quoted!)
SELECT ('{"a":"x"}'::jsonb) ->> 'a';              -- x          (text)

UPDATE js SET doc['b'] = '"x"';                   -- value must be jsonb: note the inner quotes
UPDATE js SET doc['c']['d'] = '1' WHERE id = 1;   -- creates {"c": {"d": 1}} when "c" is missing
SELECT * FROM js WHERE doc['b'] = '"x"';          -- compare with a jsonb value
```

Behaviour verified on 14.24:[^14]

- **The result is always `jsonb`.** `doc['name']::text` returns the JSON text `"Alice"` with quotes. Code that needs a SQL string must use `doc->>'name'`. Comparisons must use jsonb literals (`doc['status'] = '"active"'`); `doc['status'] = 'active'` fails because `active` is not valid JSON.
- **Casts need parentheses**: `'{"a":1}'::jsonb['a']` is a syntax error; write `('{"a":1}'::jsonb)['a']`.
- **Assignment to a NULL column creates the document**: `UPDATE ... SET doc['a'] = '1'` on a row where `doc IS NULL` produced `{"a": 1}`. This differs from `jsonb_set()`, which returns NULL for a NULL input.
- **Assigning SQL NULL stores JSON `null`**: `SET doc['a'] = NULL` produced `{"a": null}`. With `jsonb_set(doc, '{a}', NULL)` the **whole document** becomes NULL (the function is strict).
- **Missing intermediate objects are created; scalars are not traversed.** `SET doc['a']['b'] = '1'` on `{"a": 1}` failed with `cannot replace existing key` / `The path assumes key is a composite object, but it is a scalar value.` The whole `UPDATE` aborts, so one malformed row blocks a bulk update.
- **Array assignment pads with nulls**: `SET doc['arr'][5] = '1'` on a document without `arr` produced `"arr": [null, null, null, null, null, 1]`. A user-controlled index can make documents arbitrarily large; validate indexes.
- **Reads are lenient, assignments are strict.** On read, a missing key, a non-integer subscript on an array (`('[1,2]'::jsonb)['x']`), a NULL subscript, and subscripting into a scalar (`('{"a":1}'::jsonb)['a']['b']`) all return SQL NULL. On assignment the same cases raise errors: `path element at position 1 is not an integer: "x"` (array addressed with text), `jsonb subscript in assignment must not be null`, and `cannot replace existing key` (scalar). A typo in a key or a NULL parameter therefore silently yields NULL in a `SELECT` or `WHERE`, but aborts an `UPDATE`.
- **Subscript types**: integer and text subscripts are accepted, and a text subscript that looks like an integer is used as an array index (`('[1,2,3]'::jsonb)['1']` is `2`); an object key that looks like a number is reached with an integer subscript too (`('{"1":5}'::jsonb)[1]` is `5`). Other types fail at parse time: `('[1,2,3]'::jsonb)[1.5]` raises `subscript type numeric is not supported`.
- **Negative indexes in assignment** count from the end only within the array: `SET doc['arr'][-1] = '2'` replaced the last element, while `SET doc['arr'][-100] = '1'` on a 6-element array failed with `path element at position 2 is out of range: -100` (it does not pad at the front).
- **Indexes**: subscripting is not one of the GIN-indexable operators. `WHERE doc['type'] = '"click"'` cannot use a `jsonb_path_ops` GIN index; write `doc @> '{"type":"click"}'` for GIN, or create an expression index on exactly `(doc['type'])` or `(doc->>'type')` and query with the same expression.
- `json` (not `jsonb`) has no subscripting: `cannot subscript type json because it does not support subscripting`. `hstore` supports `h['key']` once the extension is installed.
- On 13: `cannot subscript type jsonb because it is not an array`.

***

#### New in 14: Procedures with OUT Parameters and SQL-Standard Function Bodies

##### OUT parameters for procedures

```sql
CREATE PROCEDURE p_out(IN a int, OUT b int) LANGUAGE plpgsql AS $$
BEGIN
  b := a * 2;
END $$;

CALL p_out(21, NULL);      -- returns one row: b = 42
CALL p_out(21);            -- ERROR: procedure p_out(integer) does not exist

DO $$
DECLARE r int;
BEGIN
  CALL p_out(5, r);        -- PL/pgSQL passes a variable for the OUT argument
  RAISE NOTICE 'r=%', r;   -- r=10
END $$;
```

- **`CALL` on 14 must supply an argument for every `OUT` parameter** (by convention `NULL`). This differs from functions, where `OUT` parameters are omitted from the call. Drivers that call procedures generically must pass the placeholder.
- A procedure cannot be called with `SELECT` (`p_out(integer) is a procedure` / `HINT: To call a procedure, use CALL.`).
- On 13: `procedures cannot have OUT arguments`. Before 14 the only way to return data from a procedure was `INOUT`.[^15]

##### SQL-standard function bodies (BEGIN ATOMIC / RETURN)

```sql
CREATE TABLE logs(x int);
CREATE FUNCTION nlogs() RETURNS bigint LANGUAGE sql
BEGIN ATOMIC
  SELECT count(*) FROM logs;
END;

DROP TABLE logs;
-- ERROR:  cannot drop table logs because other objects depend on it
-- DETAIL:  function nlogs() depends on table logs

CREATE FUNCTION add1(a int) RETURNS int LANGUAGE sql IMMUTABLE RETURN a + 1;
```

- The body is parsed **at creation time** and stored as a parse tree. Referenced tables, functions and operators are resolved then and recorded as dependencies. A string-literal body (`AS $$ ... $$`) is only parsed at call time and has no dependencies.
- **Security benefit**: name resolution happens once, at creation, with the creator's `search_path`. A caller's `search_path` cannot redirect an unqualified name inside a `BEGIN ATOMIC` body later. Measured on 14.24: with `SET search_path = ev, public, pg_catalog` and a function `ev.lower(text)`, a `BEGIN ATOMIC` function calling `lower(s)` still returned `a`, while the same body as a string literal (`AS $$ ... $$`) returned the attacker's `HIJACKED`. The flip side: the **creator's** `search_path` at `CREATE` time is what counts. A `BEGIN ATOMIC` function created in a session with `search_path = ev, public, pg_catalog` bound `lower(s)` to `ev.lower` permanently (`pg_get_functiondef()` showed `SELECT ev.lower(s) AS lower`), and kept returning `HIJACKED` after `RESET search_path`. Migrations that create such functions must run with a trusted `search_path` (or schema-qualify names). Prefer this form for SQL functions used by `SECURITY DEFINER` code, indexes, constraints and views; still pin `SET search_path` on `SECURITY DEFINER` functions.
- Restrictions on 14.24: a `BEGIN ATOMIC`/`RETURN` body cannot have polymorphic arguments (`SQL function with unquoted function body cannot have polymorphic arguments` for `anyelement`); keep a string body there. Other languages reject it with `inline SQL function body only valid for language SQL`.
- Dependencies make migrations stricter: dropping or changing a referenced column or table now fails instead of breaking at runtime. Migration scripts must drop and recreate the function, or use `CASCADE` deliberately.
- Only `LANGUAGE sql` supports this syntax. On 13 both forms are syntax errors.[^16]

***

#### New in 14: date_bin, string_to_table and Other Functions

```sql
SELECT date_bin('15 minutes', timestamp '2024-01-01 10:37:12', timestamp '2000-01-01');
-- 2024-01-01 10:30:00
SELECT date_bin('1 hour', timestamp '2024-01-01 10:37', timestamp '2000-01-01 00:15');
-- 2024-01-01 10:15:00            (buckets are aligned to the origin)
SELECT date_bin('1 month', timestamp '2024-01-01 10:37', timestamp '2000-01-01');
-- ERROR:  timestamps cannot be binned into intervals containing months or years

SELECT string_to_table('a,b,,c', ',');          -- 4 rows: a, b, '' , c
SELECT string_to_table('a,b,,c', ',', '');      -- 4 rows: a, b, NULL, c
SELECT trim_array(ARRAY[1,2,3,4], 2);           -- {1,2}
SELECT split_part('a.b.c', '.', -1);            -- c
SELECT bit_count('\xff'::bytea), bit_count(B'1011');   -- 8, 3
SELECT unistr('d\0061t\+000061');               -- data
SELECT bit_xor(x) FROM (VALUES (5),(3)) v(x);   -- 6
```

- **`date_bin()` uses fixed-length strides, not calendar units.** With `timestamptz` and a DST-observing session time zone, "1 day" buckets drift by an hour across the DST change. Measured with `TimeZone = 'Europe/Berlin'` and origin `2024-01-01 00:00+01`: a value on March 31 binned to `2024-03-31 00:00:00+01`, a value on April 10 to `2024-04-10 01:00:00+02`. For local calendar days use `date_trunc('day', ts AT TIME ZONE 'Europe/Berlin')`; for months use `date_trunc('month', ...)`.[^17]
- `string_to_table()` is the set-returning form of `string_to_array()` and avoids `unnest(string_to_array(...))`. Splitting user input in SQL is still no substitute for proper array parameters.
- On 13 all of these fail with `function ... does not exist` (and `split_part()` with a negative position with `field position must be greater than zero`).

***

#### New in 14: GROUP BY DISTINCT, JOIN USING Aliases and Other Syntax

```sql
-- GROUP BY ROLLUP(a), ROLLUP(a, b) produces duplicate grouping sets
SELECT count(*) FROM (SELECT a FROM (VALUES (1,2)) v(a,b) GROUP BY ROLLUP(a), ROLLUP(a,b)) s;           -- 6
SELECT count(*) FROM (SELECT a FROM (VALUES (1,2)) v(a,b) GROUP BY DISTINCT ROLLUP(a), ROLLUP(a,b)) s;  -- 3

SELECT j.id FROM (VALUES (1)) a(id) JOIN (VALUES (1)) b(id) USING (id) AS j;
```

- `GROUP BY DISTINCT` only removes duplicate *grouping sets*; it does not deduplicate rows.
- `CREATE OR REPLACE TRIGGER` makes trigger migrations idempotent without `DROP TRIGGER IF EXISTS` (which briefly leaves the table without the trigger in non-transactional deployments).
- `numeric` accepts `'Infinity'`/`'-Infinity'`. Columns that previously could not hold infinity now can; add `CHECK (x <> 'Infinity' AND x <> '-Infinity')` where infinite amounts make no sense.
- All of these are syntax errors or invalid input on 13.

***

#### New in 14: Predefined Roles and Authentication

```sql
GRANT pg_read_all_data TO reporting;    -- SELECT on every table/view/sequence + USAGE on every schema
GRANT pg_write_all_data TO etl;         -- INSERT/UPDATE/DELETE everywhere (no SELECT!)
```

Measured on 14.24:[^18]

- A role with only `pg_read_all_data` read `app.secrets` without any explicit grant on the schema or table, but saw **0 rows** of an RLS-protected table: RLS still applies. Only `BYPASSRLS` (plus `pg_read_all_data`) saw all rows.
- A role with only `pg_write_all_data` could `INSERT` but `SELECT` failed with `permission denied for table secrets`. `UPDATE ... WHERE`, `DELETE ... WHERE` and `INSERT ... RETURNING` read columns and therefore failed with the same error. **Unqualified `UPDATE t SET ...` and `DELETE FROM t` succeeded** (they read no column), so the role can wipe or overwrite any table. `TRUNCATE` is not included (`permission denied`). On sequences, `pg_write_all_data` allowed `nextval()`; `pg_read_all_data` allowed `SELECT last_value` but not `nextval()` (`permission denied for sequence sq`). Neither role grants `EXECUTE` on functions.
- `pg_write_all_data` inserts are still checked by RLS: an insert into an RLS-enabled table with a non-matching row failed with `new row violates row-level security policy`.
- These roles cover **all current and future tables in all databases** of the cluster. They are for backup, reporting and maintenance tooling, not for application roles. Flag `GRANT pg_write_all_data` to an application role as excessive privilege.
- `password_encryption = scram-sha-256` is the default: `CREATE ROLE pw LOGIN PASSWORD 'secret'` stored `SCRAM-SHA-256$...`. Existing MD5 hashes keep working with `md5` in `pg_hba.conf` (the `md5` method also accepts SCRAM verifiers), but a `scram-sha-256` line rejects roles whose stored password is still MD5. Migrate by resetting passwords, then change `pg_hba.conf`.
- On 13 the three roles do not exist and `password_encryption` defaults to `md5`.

***

#### New in 14: Session Timeouts and Connection Checks

```sql
ALTER ROLE app_user SET idle_session_timeout = '30min';
ALTER ROLE app_user SET idle_in_transaction_session_timeout = '60s';
ALTER ROLE app_user SET statement_timeout = '30s';
```

- `idle_session_timeout` **terminates** a connection that is idle outside a transaction. Measured: after `SET idle_session_timeout = '1s'` and two seconds of inactivity, the next query failed with `FATAL: terminating connection due to idle-session timeout` and the connection was lost. Connection pools keep idle connections by design; set this only for roles that do not connect through a pool, or make sure the pool's idle lifetime is shorter and the driver validates connections.
- `client_connection_check_interval` (Linux) lets the server notice a vanished client during a long query and cancel it. It is off (`0`) by default.
- All timeouts default to `0` (disabled) on 14 (checked in `pg_settings`). There is no `transaction_timeout` (17).

***

#### New in 14: REINDEX on Partitioned Tables, DETACH PARTITION CONCURRENTLY

```sql
REINDEX TABLE CONCURRENTLY measurements;       -- partitioned table: reindexes every partition
REINDEX INDEX CONCURRENTLY measurements_at_idx; -- partitioned index
REINDEX (TABLESPACE fast_ssd) TABLE events;    -- move indexes while rebuilding (14)

ALTER TABLE measurements DETACH PARTITION measurements_2023 CONCURRENTLY;
-- if interrupted:
ALTER TABLE measurements DETACH PARTITION measurements_2023 FINALIZE;
```

Verified on 14.24 (and on 13.23 for the failures):[^21][^20]

- `REINDEX ... CONCURRENTLY` on a partitioned table or index works on 14. On 13 it fails with `REINDEX is not yet implemented for partitioned indexes`, and `REINDEX TABLE CONCURRENTLY` on the parent only prints `table "pt" has no indexes that can be reindexed concurrently`.
- `DETACH PARTITION ... CONCURRENTLY` takes only `SHARE UPDATE EXCLUSIVE` on the parent. It **cannot run inside a transaction block** (`ALTER TABLE ... DETACH CONCURRENTLY cannot run inside a transaction block`) and is refused when the table has a default partition (`cannot detach partitions concurrently when a default partition exists`). Migration tools that wrap everything in a transaction need an opt-out.
- If a concurrent detach is cancelled, the partition stays in a "detach pending" state; finish it with `... FINALIZE` (on a table without a pending detach: `cannot complete detaching partition` / `There's no pending concurrent detach`).
- 14.14 found that partitions **detached** from a partitioned table that has a foreign key to another partitioned table could be left with broken foreign-key catalog entries. The release notes describe the repair for such partitions if they were not dropped.[^5]

***

#### New in 14: LZ4 TOAST Compression

```sql
CREATE TABLE docs (body text COMPRESSION lz4);
ALTER TABLE events ALTER COLUMN payload SET COMPRESSION lz4;   -- metadata only
SET default_toast_compression = 'lz4';                         -- for newly created columns
SELECT pg_column_compression(body) FROM docs LIMIT 1;          -- lz4
```

- The official Docker images are built with LZ4. A server built without it rejects `lz4`. `zstd` is not a TOAST compression method on 14 (`invalid value for parameter "default_toast_compression": "zstd"` / `Available values: pglz, lz4.`).
- `ALTER COLUMN ... SET COMPRESSION` took `ACCESS EXCLUSIVE` and kept the relfilenode (no rewrite). **Existing values keep their old compression**: after the `ALTER`, `pg_column_compression()` still reported `pglz` for all old rows, and still did after `VACUUM FULL`. Only values written afterwards (new rows, or an `UPDATE` that produces a new value) used `lz4`. **Copying does not recompress either**: `INSERT INTO lz4_table SELECT payload FROM pglz_table` stored the values still `pglz`-compressed, because already-compressed values are copied as they are. To convert existing data, rewrite each value (`UPDATE ... SET col = col || ''` in batches recompressed with `lz4`; `SET col = col` did **not**, the old value was kept) or dump and restore (a plain `pg_dump`/`pg_restore` sends the values as text, so they are compressed anew with the target column's method). Do not promise space savings from the `ALTER` alone.
- LZ4 is faster to compress and decompress than pglz, usually with a similar ratio. It matters for large `text`/`jsonb` values that are read often.
- `pg_dump --no-toast-compression` omits the clauses, for restores into servers without LZ4. A replica or restore target built without LZ4 cannot read LZ4-compressed values.

***

#### New in 14: Planner, Statistics and Monitoring

##### Memoize

```sql
-- nested loop with a repeating inner key
EXPLAIN (COSTS OFF) SELECT * FROM small s JOIN look l ON l.k = s.k;   -- with hash/merge joins disabled
--  Nested Loop
--    ->  Seq Scan on small s
--    ->  Memoize
--          Cache Key: s.k
--          ->  Index Scan using look_pkey on look l
```

Memoize caches inner-side results of a parameterized nested loop. It helps when the outer side has few distinct keys. It relies on correct `n_distinct` estimates; a bad estimate can make it thrash. 14.4 hardened Memoize against data types with non-deterministic equality and fixed its cost estimates; `enable_memoize = off` is the escape hatch for a regression.[^6]

##### Extended statistics on expressions

```sql
CREATE STATISTICS es_s ON (a + b), (a * b) FROM es;   -- 14: expressions allowed; a name is required
ANALYZE es;
SELECT * FROM pg_stats_ext_exprs WHERE statistics_name = 'es_s';
```

On 13: `only simple column references are allowed in CREATE STATISTICS`. Use expression statistics when queries filter on an expression that has no index (an expression index already collects statistics).

##### Query IDs and pg_stat_statements 1.9

- `compute_query_id = auto` (default) computes query IDs when `pg_stat_statements` is loaded; `on` computes them always. `EXPLAIN (VERBOSE)` shows `Query Identifier: ...`, and `pg_stat_activity.query_id` links live sessions to `pg_stat_statements` rows. `log_line_prefix` accepts `%Q`.
- `pg_stat_statements` 1.9 adds `toplevel` (top-level vs. nested statements are tracked separately) and the `pg_stat_statements_info` view (`dealloc`, `stats_reset`). On 14.24: `temp_blk_read_time` and `jit_*` columns do not exist (15), and `total_time` does not exist (renamed `total_exec_time` in 13).[^27]

##### New monitoring views and columns

```sql
SELECT wal_records, wal_fpi, wal_bytes FROM pg_stat_wal;
SELECT sessions, session_time, active_time, idle_in_transaction_time, sessions_abandoned
FROM pg_stat_database WHERE datname = current_database();
SELECT pid, mode, waitstart FROM pg_locks WHERE NOT granted;      -- how long a lock has been waited for
SELECT name, generic_plans, custom_plans FROM pg_prepared_statements;
SELECT * FROM pg_stat_progress_copy;                               -- COPY progress
SELECT pg_log_backend_memory_contexts(pg_backend_pid());           -- writes memory contexts to the server log
```

None of these exist on 13 (`relation "pg_stat_wal" does not exist`, `column "waitstart" does not exist`, ...). Dashboards that must also query 13 servers need version checks.

***

#### New in 14: libpq Pipeline Mode

Pipeline mode lets a client send many queries without waiting for each result, which cuts latency over slow links.[^26]

- It is a **client** feature: it needs libpq 14+ (or a driver with its own implementation, such as pgx or the PostgreSQL JDBC driver's batching), and works against older servers too.
- Only the **extended query protocol** may be used inside a pipeline. Measured with pgbench 14.24: a script with `\startpipeline ... \endpipeline` aborted in the default simple mode with `cannot use pipeline mode with the simple query protocol`, and ran with `-M extended`. libpq 14 still accepted `PQsendQuery()` in pipeline mode; libpq 15 rejects it, so use `PQsendQueryParams()`.[^7]
- An error inside a pipeline aborts the remaining queries up to the next sync point; they return `PGRES_PIPELINE_ABORTED`. Code must process results in order and treat a pipeline segment like a transaction for error handling.
- Pipelining does not make statements atomic. Wrap related writes in `BEGIN`/`COMMIT` inside the pipeline if they must commit together.

***

#### PostgreSQL 14.x Minor Releases (Fixes That Require Action)

Minor releases contain only fixes and need no dump/restore, but several 14.x releases require follow-up work. Before you approve "the server is on 14", check the minor version: anything older than 14.24 lacks published security fixes.[^5][^4]

- **14.1 (November 11, 2021)**: CVE-2021-23214 and CVE-2021-23222 (server and libpq accepted injected data after an SSL/GSS handshake). Update standbys before the primary. Several index-corruption bugs fixed; reindex affected indexes.
- **14.2 (February 10, 2022)**: more index-corruption fixes; reindex possibly affected indexes.
- **14.3 (May 12, 2022)**: CVE-2022-1552 (more maintenance operations, such as autovacuum, `REINDEX` and `CLUSTER`, now run index expressions inside a "security restricted operation" sandbox). Reindex GiST indexes on `ltree` columns.
- **14.4 (June 16, 2022, out of cycle)**: **`CREATE INDEX CONCURRENTLY` and `REINDEX CONCURRENTLY` could build corrupt indexes on 14.0–14.3**, missing rows that were updated during the build. Rebuild every index that was built concurrently on 14.0–14.3. `pg_amcheck --heapallindexed` detects affected B-tree indexes.[^6]
- **14.5 (August 11, 2022)**: CVE-2022-2625 (extension scripts could replace objects not owned by the extension).
- **14.6 (November 10, 2022)**: bug fixes only.
- **14.7 (February 9, 2023)**: CVE-2022-41862 (libpq memory disclosure during GSSAPI encryption setup).
- **14.8 (May 11, 2023)**: CVE-2023-2454 (`CREATE SCHEMA ... schema_element` defeating a protective `search_path`), CVE-2023-2455 (RLS policies not enforced correctly after inlining a set-returning SQL function).
- **14.9 (August 10, 2023)**: CVE-2023-39417 (`@extschema@` substitution in extension scripts). Consider reindexing BRIN indexes.
- **14.10 (November 9, 2023)**: CVE-2023-5868/5869/5870 (memory disclosure in aggregates with unknown-type arguments, array-dimension overflow, `pg_signal_backend` able to signal background workers). Reindex GiST indexes (page-split bug), B-tree indexes on `interval` columns, and BRIN `minmax_multi` indexes on date/time columns.
- **14.11 (February 8, 2024)**: CVE-2024-0985 (`REFRESH MATERIALIZED VIEW CONCURRENTLY` ran code as the refreshing user). GIN corruption under concurrent updates fixed; reindex suspect GIN indexes.
- **14.12 (May 9, 2024)**: CVE-2024-4317 (`pg_stats_ext`/`pg_stats_ext_exprs` exposed statistics to users without table privileges). **Existing databases need the manual catalog fix described in the release notes**; installing the binaries is not enough.
- **14.13 (August 8, 2024)**: CVE-2024-7348 (object replacement during `pg_dump` let attackers run code as the dumping user); new setting `restrict_nonsystem_relation_kind`.
- **14.14 (November 14, 2024)**: CVE-2024-10976/10977/10978/10979 (RLS under cached plans after role changes, libpq trusting pre-TLS error messages, `SET ROLE`/`SET SESSION AUTHORIZATION` interaction, PL/Perl environment variables). Repair for detached partitions with foreign keys to partitioned tables. **14.15 (November 21, 2024)**: out of cycle; restores the extension ABI (`ResultRelInfo`) broken by 14.14 and the `ALTER ROLE|DATABASE ... SET role` setting broken by the CVE-2024-10978 fix.
- **14.16 (February 13, 2025)**: CVE-2025-1094 (libpq quoting functions `PQescapeLiteral()` etc. and psql with invalidly encoded input: SQL injection). **14.17 (February 20, 2025)**: out of cycle; `PQescapeLiteral()`/`PQescapeIdentifier()` again honour their length argument.
- **14.18 (May 8, 2025)**: CVE-2025-4207 (GB18030 over-read). Self-referential foreign keys on partitioned tables may need to be recreated; reindex BRIN bloom indexes.
- **14.19 (August 14, 2025)**: CVE-2025-8713 (optimizer statistics leaked through views and partitions; permission errors can now appear at plan time), CVE-2025-8714/8715 (`pg_dump` output could inject psql meta-commands; psql gains `\restrict`). Reindex BRIN `numeric_minmax_multi_ops` indexes.
- **14.20 (November 13, 2025)**: CVE-2025-12817 (`CREATE STATISTICS` now checks `CREATE` on the schema), CVE-2025-12818 (libpq allocation overflow).
- **14.21 (February 12, 2026)**: CVE-2026-2003 to 2006 (`oidvector` checks, `intarray` selectivity, pgcrypto PGP buffer overrun, multibyte length validation). **14.22 (February 26, 2026)**: out of cycle; fixes multixact WAL replay failures ("could not access status of transaction") on a standby that runs the newer minor while the primary runs an older one, and a too-strict encoding check in `substring()` on TOASTed data.
- **14.23 (May 14, 2026)**: CVE-2026-6479 (startup-packet recursion), CVE-2026-6473 (allocation-size overflows, `ts_headline()`, `intarray`/`ltree` query types), CVE-2026-6474 (time zone names in `timeofday()`), CVE-2026-6472 (multirange schema privilege), CVE-2026-6478 (timing-safe password comparison), CVE-2026-6477 (`PQfn()` marked unsafe), CVE-2026-6475 (path traversal in `pg_basebackup`/`pg_rewind`), CVE-2026-6637 (`contrib/spi`).
- **14.24 (August 13, 2026)**: 28 CVEs (CVE-2026-6464 through CVE-2026-19385), among them memory-safety bugs in `to_char()`, the regexp functions and `ascii()`, `EXECUTE`/`FETCH` portal row-type confusion (CVE-2026-16239), and psql `\unrestrict` backquote expansion (CVE-2026-18408). Post-update actions: logical decoding output plugins other than `pgoutput`/`test_decoding` must be listed in the new setting `output_plugin_libraries` (CVE-2026-6471); pgcrypto refuses to decrypt data that was never really encrypted because OpenSSL rejected the cipher (CVE-2026-14663, recover with `'ignore-cipher-failure=1'` and re-encrypt); reindex `btree_gist` indexes on float columns that may contain NaN and B-tree indexes on very long `ltree` values. psql now skips the inline data of a `COPY ... FROM STDIN` that failed at startup (CVE-2026-6464).[^4]

**psql scripts after 14.24**: measured on 14.24, a migration script containing `COPY t FROM STDIN WITH (FORMAT csv, HEADER match);` (invalid on 14) failed, and psql then treated **every following line up to `\.`** as COPY data, so the rest of the script silently did not run: psql printed only the `COPY` error (`header requires a Boolean value`) and exited with status 0. The same happened for `COPY nonexistent FROM STDIN;` followed by data and `SELECT` lines: nothing after the `COPY` ran. With a `\.` line after the data, the following statements ran normally; with `-v ON_ERROR_STOP=1`, psql stopped at the failed `COPY` and exited with status 3. The skipping is done by the **psql client** (14.24 and the other August 2026 minors), not by the server: an older psql would instead execute the inline data lines as SQL, which is the injection hazard CVE-2026-6464 closes. Always terminate inline COPY data with `\.`, run psql scripts with `ON_ERROR_STOP=1` so a failed command stops the script, and use an up-to-date psql for deployments.

***

#### Schema Migrations and Locking

This is the most important review area for production PostgreSQL. A migration that is correct but takes the wrong lock can take an application down.

##### Lock levels of common DDL (measured on 14.24 with pg_locks)

| Statement | Lock on the table | Blocks reads? | Blocks writes? | Rewrite / full scan |
|---|---|---|---|---|
| `ALTER TABLE ADD COLUMN c int` (no default, or a non-volatile default) | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none (metadata only) |
| `ADD COLUMN c int NOT NULL DEFAULT 0` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none |
| `ADD COLUMN c uuid DEFAULT gen_random_uuid()` (volatile default) | `ACCESS EXCLUSIVE` | yes | yes | **rewrite** |
| `ADD COLUMN c timestamptz DEFAULT now()` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none (`now()` is stable, evaluated once) |
| `ALTER COLUMN c TYPE bigint` (from `int`) | `ACCESS EXCLUSIVE` | yes | yes | **rewrite** (and index rebuild) |
| `ALTER COLUMN v TYPE varchar(20)` from `varchar(10)`, or `varchar(n)` → `text` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none |
| `ALTER COLUMN v TYPE varchar(10)` from `text` | `ACCESS EXCLUSIVE` | yes | yes | **rewrite** |
| `ALTER COLUMN c SET NOT NULL` | `ACCESS EXCLUSIVE` | yes | yes | full scan, unless a valid `CHECK (c IS NOT NULL)` proves it |
| `ADD CONSTRAINT ... CHECK (...) NOT VALID` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none |
| `VALIDATE CONSTRAINT` (check) | `SHARE UPDATE EXCLUSIVE` | no | no | full scan |
| `ADD CONSTRAINT ... FOREIGN KEY ... NOT VALID` | `SHARE ROW EXCLUSIVE` on **both** tables | no | yes (briefly) | none |
| `VALIDATE CONSTRAINT` (foreign key) | `SHARE UPDATE EXCLUSIVE` (and `ROW SHARE` on the referenced table) | no | no | full scan |
| `ALTER COLUMN SET STATISTICS` | `SHARE UPDATE EXCLUSIVE` | no | no | none |
| `ALTER COLUMN SET COMPRESSION` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none |
| `DROP COLUMN` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none |
| `SET UNLOGGED` / `SET LOGGED` | `ACCESS EXCLUSIVE` | yes | yes | **rewrite** |
| `CREATE INDEX` | `SHARE` | no | **yes**, for the whole build | full scan |
| `CREATE INDEX CONCURRENTLY` | `SHARE UPDATE EXCLUSIVE` | no | no | two scans and waits for old transactions |
| `ATTACH PARTITION` | `SHARE UPDATE EXCLUSIVE` on parent, `ACCESS EXCLUSIVE` on the attached table and on the **default partition** | parent: no | parent: no | scans the new partition unless a matching `CHECK` exists; also scans the default partition |
| `DETACH PARTITION ... CONCURRENTLY` (14) | `SHARE UPDATE EXCLUSIVE` | no | no | not allowed in a transaction block or with a default partition |
| `VACUUM FULL`, `CLUSTER` | `ACCESS EXCLUSIVE` | yes | yes | rewrite |

The rewrite facts above were measured by comparing `pg_relation_filenode()` before and after each statement inside the migration's transaction, and the locks by querying `pg_locks` for the session's own PID before `COMMIT`. A rewrite also showed an extra `SHARE` lock on the table. Source for lock modes: the "Explicit Locking" and `ALTER TABLE` documentation.[^19][^20]

##### The lock queue: why "fast" DDL still causes outages

Even a metadata-only `ALTER TABLE` needs `ACCESS EXCLUSIVE`. It waits behind any transaction that has touched the table, even one holding only an `ACCESS SHARE` lock from a `SELECT`. **While it waits, every new query on the table queues behind it.** Measured on 14.24: with session R holding an open read transaction on `q` and session A waiting on `ALTER TABLE q ADD COLUMN x int`, a plain `SELECT count(*) FROM q` in a third session blocked until its own `lock_timeout` fired (`canceling statement due to lock timeout`).

Every migration session must set a short `lock_timeout` and retry:

```sql
-- BAD: can wait forever and block all traffic on the table meanwhile
ALTER TABLE orders ADD COLUMN note text;

-- GOOD: fail fast, retry from the migration tool with backoff
SET lock_timeout = '3s';
SET statement_timeout = '15min';   -- upper bound for scans, validations, etc.
ALTER TABLE orders ADD COLUMN note text;
```

`SET LOCAL` limits the setting to the current transaction (verified: `lock_timeout` was `3s` inside and `0` again after `COMMIT`). Migration frameworks that wrap each migration in a transaction should use `SET LOCAL`, so the setting does not leak into a pooled connection. There is no `transaction_timeout` on 14 (17).

##### Adding columns

```sql
-- GOOD: metadata-only since PostgreSQL 11 (constant and stable defaults are stored, not written)
ALTER TABLE orders ADD COLUMN priority int NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN imported_at timestamptz DEFAULT now();  -- one value for all old rows

-- BAD on large tables: volatile default → full table rewrite under ACCESS EXCLUSIVE
ALTER TABLE orders ADD COLUMN public_id uuid DEFAULT gen_random_uuid();

-- GOOD: add without default, set default for new rows, backfill in batches
ALTER TABLE orders ADD COLUMN public_id uuid;
ALTER TABLE orders ALTER COLUMN public_id SET DEFAULT gen_random_uuid();
-- backfill in batches (see below), then enforce NOT NULL safely (next section)
```

`DEFAULT now()` gives every *existing* row the same timestamp (the time of the migration). That is usually wrong for "created_at"-style columns. Backfill from real data.

##### Adding NOT NULL without a long ACCESS EXCLUSIVE scan

```sql
SET lock_timeout = '3s';
ALTER TABLE t ADD CONSTRAINT t_v_not_null CHECK (v IS NOT NULL) NOT VALID;  -- instant
ALTER TABLE t VALIDATE CONSTRAINT t_v_not_null;  -- scans under SHARE UPDATE EXCLUSIVE; writes continue
ALTER TABLE t ALTER COLUMN v SET NOT NULL;       -- no scan: proven by the valid CHECK
ALTER TABLE t DROP CONSTRAINT t_v_not_null;      -- optional cleanup
```

With `client_min_messages = debug1`, 14.24 confirms the skip (measured on a column named `c1`): `existing constraints on column "t.c1" are sufficient to prove that it does not contain nulls`. (`ADD CONSTRAINT ... NOT NULL ... NOT VALID` is 18. Do not suggest it for 14.)

##### Foreign keys and CHECK constraints

```sql
-- BAD: validates every row while holding SHARE ROW EXCLUSIVE on both tables (blocks writes)
ALTER TABLE orders ADD CONSTRAINT orders_customer_fk
  FOREIGN KEY (customer_id) REFERENCES customers(id);

-- GOOD: two steps
ALTER TABLE orders ADD CONSTRAINT orders_customer_fk
  FOREIGN KEY (customer_id) REFERENCES customers(id) NOT VALID;
ALTER TABLE orders VALIDATE CONSTRAINT orders_customer_fk;
```

Index the referencing column (`orders.customer_id`). PostgreSQL does not do it automatically. Without the index, every `DELETE`/key `UPDATE` on `customers` scans `orders`, and `ON DELETE CASCADE` becomes very slow.

`ON DELETE SET NULL` sets **every** referencing column to NULL. With a composite key such as `(tenant_id, author_id)`, that fails on a `NOT NULL` `tenant_id`. The column-list form `ON DELETE SET NULL (author_id)` is 15; on 14 use a single-column foreign key where possible, or a `BEFORE DELETE` trigger on the parent that nulls only the intended column.

##### Changing column types

- `int` → `bigint` rewrites the table and its indexes under `ACCESS EXCLUSIVE`. For big tables, add a new `bigint` column, backfill it in batches, keep it in sync with a trigger, then swap in one short transaction. Better: create primary keys as `bigint` from the start.
- Widening `varchar(n)` or converting `varchar(n)` → `text` is metadata-only. Narrowing, or converting `text` → `varchar(n)`, rewrites the table.
- A `USING` clause does not force a rewrite by itself: `TYPE text USING tx::text` (from `varchar`) kept the relfilenode on 14.24. A `USING` expression that changes the stored value, or a type change that is not binary-coercible, rewrites the table and its indexes.

##### Indexes in migrations

```sql
-- BAD on a live table: blocks INSERT/UPDATE/DELETE for the whole build
CREATE INDEX orders_status_idx ON orders(status);

-- GOOD
CREATE INDEX CONCURRENTLY orders_status_idx ON orders(status);
```

`CREATE INDEX CONCURRENTLY` rules that reviewers must enforce (all verified on 14.24):

- **Run 14.4 or later.** Concurrent index builds and rebuilds on 14.0–14.3 could miss rows; such indexes must be rebuilt (see [minor releases](#postgresql-14x-minor-releases-fixes-that-require-action)).[^6]
- It **cannot run inside a transaction block**: `CREATE INDEX CONCURRENTLY cannot run inside a transaction block`. Migration tools that wrap migrations in a transaction need a per-migration opt-out.
- A failure (deadlock, unique violation, cancellation) leaves an **INVALID index** behind. It still slows down writes and is never used for reads. **`CREATE INDEX CONCURRENTLY IF NOT EXISTS` then skips silently** (`NOTICE: relation "dup_a" already exists, skipping`) and leaves the invalid index in place. Idempotent migrations must check `pg_index.indisvalid` and `DROP INDEX CONCURRENTLY` before retrying.
- It cannot be used on a partitioned parent (`cannot create index on partitioned table "m" concurrently`). Pattern: `CREATE INDEX idx ON ONLY parent (...)` (creates an invalid parent index), then `CREATE INDEX CONCURRENTLY` on each partition and `ALTER INDEX idx ATTACH PARTITION part_idx`. The parent index became valid once all partitions were attached.
- It waits for all transactions that could use the old snapshot. A long-running transaction stalls it. Watch progress in `pg_stat_progress_create_index`.
- `DROP INDEX CONCURRENTLY` and `REINDEX ... CONCURRENTLY` (12+; on partitioned tables 14+) are the non-blocking counterparts.
- Add a primary key or unique constraint without a long lock by building the unique index concurrently and attaching it: `ALTER TABLE pkt ADD CONSTRAINT pkt_pkey PRIMARY KEY USING INDEX pkt_id;`. If a key column is still nullable, the command sets `NOT NULL` itself and scans the whole table under `ACCESS EXCLUSIVE` (14.24 logs `verifying table "pkt"` at `debug1`). Make the columns `NOT NULL` first with the `CHECK ... NOT VALID` pattern. The index is renamed to the constraint name.

```sql
-- Find invalid indexes left by failed concurrent builds
SELECT indexrelid::regclass, indrelid::regclass FROM pg_index WHERE NOT indisvalid;
```

##### Backfills and large deletes

```sql
-- BAD: one giant transaction; long locks, huge WAL burst, bloat, replication lag
UPDATE orders SET public_id = gen_random_uuid() WHERE public_id IS NULL;

-- GOOD: bounded batches committed separately (driven from the application or a procedure)
WITH batch AS (
  SELECT id FROM orders
  WHERE public_id IS NULL
  ORDER BY id
  LIMIT 5000
  FOR UPDATE SKIP LOCKED
)
UPDATE orders o SET public_id = gen_random_uuid()
FROM batch WHERE o.id = batch.id;
```

PostgreSQL has no `DELETE ... LIMIT` (`syntax error at or near "LIMIT"`). Use the same CTE pattern with `DELETE ... USING`. For time-series retention, drop or detach whole partitions instead of deleting rows.

A procedure can commit between batches (11+), but only when it is called outside an explicit transaction block:

```sql
CREATE PROCEDURE backfill_public_id() LANGUAGE plpgsql AS $$
DECLARE n int;
BEGIN
  LOOP
    UPDATE orders SET public_id = gen_random_uuid()
    WHERE id IN (SELECT id FROM orders WHERE public_id IS NULL ORDER BY id LIMIT 5000);
    GET DIAGNOSTICS n = ROW_COUNT;
    EXIT WHEN n = 0;
    COMMIT;
  END LOOP;
END $$;
CALL backfill_public_id();          -- inside BEGIN ... : ERROR: invalid transaction termination
```

##### Other migration rules

- Renaming or dropping a column or table that running application code still uses causes errors. Use expand/contract: add the new column, deploy code that writes both, backfill, switch reads, drop later.
- Avoid `VACUUM FULL`/`CLUSTER` on live tables. They hold `ACCESS EXCLUSIVE` for the whole rewrite. Use `pg_repack` or `pg_squeeze` if bloat really requires it.
- **Enums**: `ALTER TYPE ... ADD VALUE` is transactional (a `ROLLBACK` removed the value), but the new value **cannot be used in the same transaction**, even when the type itself was created in that transaction: on 14.24 both cases raised `unsafe use of new value "b" of enum type ...` / `HINT: New enum values must be committed before they can be used.` Migrations that add an enum value and then insert rows with it must commit in between. Removing or renaming values needs a new type. Consider a lookup table or `text` with a `CHECK` constraint for value sets that change often.
- `REFRESH MATERIALIZED VIEW` holds `ACCESS EXCLUSIVE` and blocks readers. `REFRESH ... CONCURRENTLY` needs a unique index without a `WHERE` clause (see [Materialized views](#materialized-views)).
- Changing a function used by an index (`CREATE OR REPLACE FUNCTION`) does not rebuild the index. If the results change, the index is silently wrong until `REINDEX`.
- **Functions in expression indexes and materialized views run with the caller's `search_path` on 14.** PostgreSQL 17 runs maintenance commands with `search_path = pg_catalog, pg_temp`, so such objects break on upgrade. Pin `SET search_path` on these functions now, or use `BEGIN ATOMIC` bodies (see [Upgrading](#upgrading-off-postgresql-14)).
- **`BEGIN ATOMIC` functions add dependencies**: `DROP TABLE` or `ALTER TABLE ... DROP COLUMN` on objects they reference fails until the function is dropped or replaced.

***

#### Indexing

##### Index types

| Type | Use for | Notes |
|---|---|---|
| B-tree (default) | `=`, `<`, `>`, `BETWEEN`, `IN`/`= ANY`, `ORDER BY`, `IS NULL`, prefix `LIKE 'x%'` (with `C` collation or `text_pattern_ops`) | Only efficient when the leading columns are constrained (no skip scan before 18). Deduplication (13), bottom-up deletion (14) |
| Hash | `=` only | WAL-logged and crash-safe since 10. Rarely better than B-tree |
| GIN | `jsonb` (`@>`, `?`, `?\|`, `?&`, `@?`, `@@`), arrays (`@>`, `&&`), full-text `tsvector`, `pg_trgm` (`LIKE '%x%'`, `ILIKE`, `~`, and `=` since 14) | Slower writes; `fastupdate` pending list |
| GiST | ranges and multiranges, geometry, exclusion constraints, `pg_trgm`, nearest-neighbour `ORDER BY <->` | Presorted builds (14) are faster and smaller |
| SP-GiST | non-balanced data (IP prefixes, quadtrees, text prefixes) | `INCLUDE` columns since 14 |
| BRIN | very large, append-only tables where the column correlates with physical order (timestamps) | `minmax_multi` and `bloom` operator classes since 14; several 14.x releases advise reindexing BRIN indexes |

```sql
CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_orders_user_status ON orders (user_id, status);             -- order matters
CREATE INDEX idx_active_users ON users (email) WHERE status = 'active';       -- partial
CREATE UNIQUE INDEX idx_users_email_lower ON users (lower(email));           -- expression
CREATE INDEX idx_users_email_cov ON users (email) INCLUDE (name, created_at); -- covering (11+)
CREATE INDEX idx_posts_fts ON posts USING gin (to_tsvector('english', title || ' ' || body));
CREATE INDEX idx_events_doc ON events USING gin (doc jsonb_path_ops);
CREATE INDEX idx_logs_at_brin ON logs USING brin (at);
CREATE INDEX idx_logs_at_mm ON logs USING brin (at timestamptz_minmax_multi_ops);   -- 14
```

##### Rules for reviewing indexes

- **The predicate must match the index expression exactly.** Measured on 14.24: `WHERE date_trunc('day', created_at) = '2024-02-01'` and `WHERE customer_id::text = '42'` both gave a `Seq Scan` despite indexes on the columns. The half-open range `created_at >= '2024-02-01' AND created_at < '2024-02-02'` gave `Index Scan using orders_created_idx`.
- **Type mismatches defeat indexes.** `WHERE id = 42.0` on a `bigint` column was planned as `Filter: ((id)::numeric = 42.0)`, a sequential scan. Drivers that send parameters as `numeric` or `text` cause the same problem. Bind parameters with the column's type.
- **Expression indexes need IMMUTABLE functions.** `CREATE INDEX ON orders (date(created_at))` on a `timestamptz` column fails with `functions in index expression must be marked IMMUTABLE`, and so does `date_trunc('day', created_at, 'UTC')` on 14 (that form is `STABLE` until 16). Use `((created_at AT TIME ZONE 'UTC')::date)` and query with exactly that expression.
- **Prefix `LIKE` and collations.** The official image's default collation is `en_US.utf8`. There, `email LIKE 'user12%'` did not use a plain B-tree index (`Seq Scan`). A `text_pattern_ops` index gave `Index Cond: ((email ~>=~ 'user12') AND (email ~<~ 'user13'))`. A column in the `C` collation works with a plain index. `starts_with(s, 'k123')` is **not** indexable on 14 even with the `C` collation (measured `Seq Scan ... Filter: starts_with(...)`; 15 added that), so write `s LIKE 'k123%'`. Leading-wildcard `LIKE '%x'` needs `pg_trgm`.
- **Escape `LIKE` input.** `_` and `%` in user input are wildcards. Escape them (`replace(replace(replace(x, '\', '\\'), '%', '\%'), '_', '\_')`).
- **Every foreign key column used for joins or cascades needs an index.**
- **Partial indexes** must have a predicate that the query implies literally. For `ON CONFLICT` on a partial unique index, the statement must repeat the predicate: `ON CONFLICT (user_id) WHERE active DO NOTHING`. Without it: `there is no unique or exclusion constraint matching the ON CONFLICT specification`.
- **Multicolumn order**: equality columns first, then the range or sort column. `(status, created_at)` serves `WHERE status = $1 ORDER BY created_at DESC LIMIT 20`.
- **No skip scan in 14.** With an index on `(a, b)` and only 4 distinct `a` values, `WHERE b = 500` was a `Seq Scan`. Adding `a = ANY('{0,1,2,3}')` gave `Index Only Scan ... Index Cond: ((a = ANY (...)) AND (b = 500))`. Don't accept "the composite index covers it" for a query that filters only on a later column.
- **Over-indexing**: each index slows writes, prevents HOT updates on its columns, and adds vacuum and WAL work. Look for unused indexes (`pg_stat_user_indexes.idx_scan = 0` over a representative period, and on replicas too) and duplicate prefixes.
- **Unique constraints and NULLs**: `UNIQUE (a, b)` allows any number of rows with `b IS NULL` (two `(1, NULL)` rows were accepted). `NULLS NOT DISTINCT` is 15. On 14, add a partial unique index for the NULL case: `CREATE UNIQUE INDEX ON t (a) WHERE b IS NULL` rejected the second `(1, NULL)` with `duplicate key value violates unique constraint`. A `coalesce(b, -1)` expression index also works but needs a sentinel that can never be a real value, and `ON CONFLICT` must name the same expression.
- **Case-insensitive uniqueness**: `CREATE UNIQUE INDEX ... (lower(email))` plus `ON CONFLICT (lower(email))`. Queries must use `lower(email) = lower($1)`.

***

#### Query Performance and EXPLAIN

##### Reading plans

```sql
EXPLAIN SELECT ...;                                   -- estimated plan, does not run
EXPLAIN (ANALYZE, BUFFERS) SELECT ...;                -- runs the query; real rows, timing, buffers
EXPLAIN (ANALYZE, BUFFERS, VERBOSE, SETTINGS, WAL) SELECT ...;
```

- `BUFFERS` must be requested explicitly with `ANALYZE` on 14 (18 adds it automatically). It shows temporary file blocks (`temp read=6973 written=7668` for an external sort with `work_mem = 64kB`); timing of temporary file I/O is 15. `WAL` (13+) shows WAL records and bytes of DML.
- **`EXPLAIN ANALYZE` executes the statement.** `EXPLAIN ANALYZE DELETE FROM ex` really deleted the row. Wrap DML in `BEGIN; EXPLAIN ANALYZE ...; ROLLBACK;`.
- `EXPLAIN (VERBOSE)` prints `Query Identifier: ...` when `compute_query_id` is on (14).
- There is no `EXPLAIN (GENERIC_PLAN)` (16) or `SERIALIZE`/`MEMORY` (17). To see a generic plan on 14, `PREPARE` the statement and `EXPLAIN EXECUTE` it with `SET plan_cache_mode = force_generic_plan`.
- Compare estimated `rows=` with `actual rows=`. Errors of 10x or more point to stale or insufficient statistics, correlated columns (`CREATE STATISTICS name (dependencies, ndistinct, mcv) ON a, b FROM t`, or on expressions since 14) or non-sargable predicates.
- `Sort Method: external merge  Disk: ...` means the sort spilled. Raise `work_mem` per query or role rather than globally.
- A `Seq Scan` is not a defect in itself. On small tables, or when most rows are returned, it is the right plan.

##### Pagination

```sql
-- BAD on large offsets: reads and discards every skipped row
SELECT id FROM orders ORDER BY id LIMIT 20 OFFSET 100000;
-- Index Only Scan using orders_pkey (actual rows=100020 loops=1)   <- 100k rows read to return 20

-- GOOD: keyset pagination with a row-value comparison and a matching composite index
CREATE INDEX ord_created_id ON orders (created_at, id);
SELECT id, created_at FROM orders
WHERE (created_at, id) < ($1, $2)
ORDER BY created_at DESC, id DESC
LIMIT 20;
-- Limit -> Index Only Scan Backward using ord_created_id
--            Index Cond: (ROW(created_at, id) < ROW(...))
```

The row-value form matters. The expanded `created_at < $1 OR (created_at = $1 AND id < $2)` was planned as a `Filter` on a backward index scan, not as an `Index Cond`, so it can read many rows before finding the page. All key columns must be sorted in the same direction for a backward scan.

##### Common query anti-patterns

```sql
-- BAD: NOT IN with a nullable subquery returns no rows when any NULL is present
SELECT 1 WHERE 3 NOT IN (1, 2, NULL);                       -- 0 rows
-- GOOD
SELECT ... WHERE NOT EXISTS (SELECT 1 FROM t WHERE t.x = outer.x);

-- BAD: BETWEEN with timestamps silently drops most of the last day
SELECT timestamptz '2024-01-31 23:59:59.5+00' BETWEEN '2024-01-01' AND '2024-01-31';  -- false
-- GOOD: half-open range
WHERE at >= '2024-01-01' AND at < '2024-02-01'

-- BAD: N+1 queries from application loops
-- GOOD: one query with = ANY($1::bigint[]) or a JOIN

-- GOOD for approximate counts on large tables:
SELECT reltuples::bigint FROM pg_class WHERE oid = 'orders'::regclass;
```

- **Correlated `IN (subquery)` is not flattened on 14.** `WHERE c.id IN (SELECT o.customer_id FROM orders o WHERE o.total > c.region)` was planned as `Seq Scan on cust c  Filter: (SubPlan 1)`, re-running the subquery per row (17 turns this into a semi-join). Rewrite it as `EXISTS (SELECT 1 FROM orders o WHERE o.customer_id = c.id AND o.total > c.region)`.
- **`IS NULL` on a `NOT NULL` column is not optimized away on 14**: `WHERE status IS NULL` on a `NOT NULL` column was a full `Seq Scan` with `Filter: (status IS NULL)`.
- `SELECT *` matters in hot paths and for wide tables: it prevents index-only scans, pulls TOASTed columns, and breaks when columns are added (`INSERT INTO x SELECT * FROM y`).
- **CTEs** are inlined since 12 when they are referenced once and have no side effects: `WITH c AS (SELECT * FROM orders) SELECT * FROM c WHERE id = 5` was planned as `Index Scan using orders_pkey`. `WITH c AS MATERIALIZED (...)` forces a `CTE Scan` over a full `Seq Scan`. Use it deliberately.
- Prefer `= ANY($1)` with an array parameter over generating `IN ($1, ..., $n)`: one prepared statement for every list length, and an empty array works (`= ANY('{}')` matches nothing) while an empty `IN ()` is a syntax error. Since 14, long `IN (const, ...)` lists are evaluated with a hash table (not visible in `EXPLAIN`); `NOT IN` lists get that in 15.
- `ORDER BY random() LIMIT n` reads and sorts every row. Use `TABLESAMPLE SYSTEM (p)` for large tables.
- `DISTINCT` used to hide duplicate rows from a wrong join is a bug, not an optimization.

##### Prepared statements and generic plans

```sql
-- skew: k = 1 in 99% of rows
PREPARE q(int) AS SELECT count(*) FROM skew WHERE k = $1;
EXPLAIN (COSTS OFF) EXECUTE q(1);      -- Seq Scan, Filter: (k = 1)
EXPLAIN (COSTS OFF) EXECUTE q(99999);  -- Index Only Scan using skew_k_idx
SET plan_cache_mode = force_generic_plan;
EXPLAIN (COSTS OFF) EXECUTE q(1);      -- Index Only Scan ... Index Cond: (k = $1)  <- wrong for k = 1
```

With `plan_cache_mode = auto` (the default), the first five executions are planned with the actual values. After that the server may switch to a generic plan if its estimated cost is not worse on average. For heavily skewed columns this can lock in a bad plan. Fix it per role, session or function with `plan_cache_mode = force_custom_plan`, or restructure the query. Don't disable prepared statements in the driver globally. `pg_prepared_statements.generic_plans`/`custom_plans` (14) show which kind a session used.

##### Statistics and maintenance

- Run `ANALYZE` after bulk loads and after creating expression indexes.
- Raise per-column statistics targets for skewed columns: `ALTER TABLE t ALTER COLUMN c SET STATISTICS 1000`. Reset with `SET STATISTICS -1` on 14 (`SET STATISTICS DEFAULT` is 17).
- Extended statistics for correlated columns: `CREATE STATISTICS addr_s (dependencies, mcv) ON city, zip FROM addresses; ANALYZE addresses;`. A name is required on 14.
- Autovacuum never analyzes a partitioned parent; run `ANALYZE parent` after large changes.
- `pg_upgrade` does **not** carry optimizer statistics over to 14–17 targets. Run `vacuumdb --all --analyze-in-stages` right after upgrading (18 keeps them).

***

#### Transactions, Isolation and Concurrency

##### Isolation levels (behaviour verified with two sessions on 14.24)[^22]

| Level | Snapshot | What you must handle |
|---|---|---|
| READ COMMITTED (default) | per statement | Lost updates in read-modify-write; non-repeatable reads |
| REPEATABLE READ | per transaction | `could not serialize access due to concurrent update` (SQLSTATE `40001`) on conflicting writes. Retry |
| SERIALIZABLE | per transaction + SSI | `40001` "read/write dependencies" errors. Retry the **whole** transaction |

```sql
-- BAD (READ COMMITTED): read in app, compute, write back → lost update
SELECT bal FROM acc WHERE id = 1;            -- app reads 100
-- (another session: UPDATE acc SET bal = bal - 30 WHERE id = 1; commits → 70)
UPDATE acc SET bal = 50 WHERE id = 1;        -- app writes 100 - 50; the -30 is lost
-- measured on 14.24: final balance 50, correct result 20

-- GOOD: atomic update
UPDATE acc SET bal = bal - 50 WHERE id = 1 AND bal >= 50 RETURNING bal;
-- GOOD: lock the row when the logic needs it
BEGIN;
SELECT bal FROM acc WHERE id = 1 FOR UPDATE;
UPDATE acc SET bal = $new WHERE id = 1;
COMMIT;
-- GOOD: optimistic concurrency with a version column
UPDATE acc SET bal = $new, version = version + 1 WHERE id = 1 AND version = $read_version;
-- check the affected row count; 0 means "someone else changed it"
```

**Write skew** (two on-call doctors both go off duty) is prevented only by `SERIALIZABLE`. In the test, the second commit failed with `could not serialize access due to read/write dependencies among transactions`. Code that uses `REPEATABLE READ` or `SERIALIZABLE` **must have a retry loop** for SQLSTATE `40001` (and `40P01` deadlock). Without one, the higher isolation level only turns data corruption into user-visible errors. A `REPEATABLE READ` transaction that updated a row changed by a concurrent committed transaction failed with `could not serialize access due to concurrent update`.

##### Row locks and queues

```sql
-- Job queue: each worker takes different rows without blocking
SELECT id FROM jobs WHERE NOT done ORDER BY id LIMIT 10 FOR UPDATE SKIP LOCKED;
```

- `SKIP LOCKED` is for queues. It returns an inconsistent view, so never use it for general reads.
- `NOWAIT` fails immediately (`could not obtain lock on row in relation "jobs"`). `lock_timeout` also applies to row locks.
- Use `FOR NO KEY UPDATE` when the key is not changed. It does not conflict with foreign key checks (`FOR KEY SHARE`) from inserts into child tables.
- Deadlocks: lock rows in a consistent order (`ORDER BY id ... FOR UPDATE`), and keep transactions short.
- **Advisory locks**: prefer `pg_advisory_xact_lock()`/`pg_try_advisory_xact_lock()`, released at commit or rollback. Session-level `pg_advisory_lock()` leaks across transaction-pooled connections and must be paired with `pg_advisory_unlock()` in every code path.

##### Upserts (there is no MERGE on 14)

```sql
INSERT INTO kv (k, v) VALUES ($1, $2)
ON CONFLICT (k) DO UPDATE
  SET v = kv.v + EXCLUDED.v
RETURNING *, (xmax = 0) AS inserted;    -- xmax = 0 → row was inserted (implementation detail)
```

- `INSERT ... ON CONFLICT` is atomic under concurrency: either the insert happens or the existing row is locked and updated. It is the correct tool on 14 for "insert or update by key". `MERGE` (15) does not exist; code generated for newer servers that uses it fails with `syntax error at or near "MERGE"`.
- A multi-row `INSERT ... ON CONFLICT DO UPDATE` with the same key twice fails: `ON CONFLICT DO UPDATE command cannot affect row a second time`. Deduplicate input first (`DISTINCT ON (k)`).
- `ON CONFLICT DO NOTHING RETURNING *` returns **no row** for conflicting keys. Code that expects an id back must `SELECT` it afterwards, or use `DO UPDATE SET k = EXCLUDED.k` (which takes a row lock and writes a new row version).
- The conflict target must match a unique index or constraint exactly, including expression and partial-index predicate.
- **Synchronising a table from a source** (what `MERGE` is for on 15+): in one transaction, `INSERT ... SELECT ... ON CONFLICT (k) DO UPDATE SET ... WHERE (t.*) IS DISTINCT FROM (EXCLUDED.*)` for inserts and changes, then `DELETE FROM t WHERE NOT EXISTS (SELECT 1 FROM src WHERE src.k = t.k)` scoped to the batch. The `WHERE ... IS DISTINCT FROM` avoids rewriting unchanged rows.

##### Timeouts and long transactions

- Long-running and idle-in-transaction sessions block vacuum cluster-wide (bloat, transaction ID wraparound risk) and block DDL.
- Set per role: `statement_timeout`, `idle_in_transaction_session_timeout`, `idle_session_timeout` (14). All default to 0 (disabled), verified with `pg_settings`. There is no `transaction_timeout` on 14.
- Never keep a transaction open across network calls, user think-time or message-queue waits.
- Sequences are non-transactional. A rolled-back insert consumed a value (the next identity was 3 after one committed and one rolled-back insert). Never assume identity values are gap-free or ordered by commit time.

***

#### Security

##### SQL injection in dynamic SQL

```sql
-- BAD: concatenation; input  x' OR '1'='1  matches every row
EXECUTE 'SELECT count(*) FROM acct WHERE name = ''' || n || '''' INTO c;

-- GOOD: parameters with USING
EXECUTE 'SELECT count(*) FROM acct WHERE name = $1' INTO c USING n;

-- GOOD: identifiers with %I, values with $n or %L
EXECUTE format('SELECT count(*) FROM %I.%I WHERE name = $1', schema_name, table_name)
  INTO c USING n;
```

`format()` placeholders: `%I` quotes identifiers (`"Weird Name"`), `%L` quotes literals and renders NULL as the bare word `NULL`, and **`%s` does no quoting at all** (`format('%s', 'O''Reilly')` gives `O'Reilly`). Any `%s` that receives external input in SQL text is an injection.

In application code, always use driver parameters (`$1`, `?`, named binds). Identifiers (table names, `ORDER BY` columns, sort direction) cannot be parameters; validate them against an allow-list. Never build `IN (...)` lists by string joining; use `= ANY($1)`. Client-side quoting with libpq's `PQescapeLiteral()` and friends had an injection bug with invalidly encoded input that 14.16 fixed (CVE-2025-1094); clients linked against an older libpq are still affected.[^5]

##### The public schema is writable by everyone on 14

On 14, every database is created with `CREATE` on schema `public` granted to `PUBLIC`, and `public` is owned by the bootstrap superuser:

```sql
SELECT nspname, pg_get_userbyid(nspowner), nspacl FROM pg_namespace WHERE nspname = 'public';
--  public | postgres | {postgres=UC/postgres,=UC/postgres}        (14.24)
SET ROLE app;                       -- ordinary login role
CREATE TABLE public.t_app(a int);   -- succeeds on 14; fails on a new 15+ database
```

Any role that can connect can create functions and operators in `public` that shadow objects other roles resolve through `search_path` (CVE-2018-1058). **Revoke it explicitly in every 14 database** and keep application objects in a dedicated schema owned by the migration role:[^23]

```sql
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
CREATE SCHEMA app AUTHORIZATION migrator;
GRANT USAGE ON SCHEMA app TO app_user;
```

After the `REVOKE`, the same `CREATE TABLE public.t_app` by `app` failed with `permission denied for schema public`. This also makes the database behave like a fresh 15+ database, so migrations behave the same after the upgrade (`pg_upgrade` keeps the old permissive ACL).

##### SECURITY DEFINER functions

```sql
-- BAD: runs as the owner, but resolves unqualified names using the caller's search_path
CREATE FUNCTION app.n_unsafe() RETURNS bigint LANGUAGE plpgsql SECURITY DEFINER AS $$
BEGIN
  RETURN (SELECT count(*) FROM app.secrets WHERE lower(s) = lower(s));
END $$;

-- attacker (owns schema evil):
--   CREATE FUNCTION evil.lower(text) RETURNS text ... RAISE NOTICE 'running as %', current_user ...
--   SET search_path = evil, pg_catalog;
--   SELECT app.n_unsafe();     -- NOTICE: running as postgres  ← attacker code ran as superuser

-- GOOD
CREATE FUNCTION app.n_safe() RETURNS bigint LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
  RETURN (SELECT count(*) FROM app.secrets WHERE lower(s) = lower(s));
END $$;
REVOKE EXECUTE ON FUNCTION app.n_safe() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION app.n_safe() TO app_role;
```

Measured on 14.24: the attacker's `evil.lower()` ran inside `n_unsafe()` as `postgres`; `n_safe()` ignored it. Every `SECURITY DEFINER` function needs `SET search_path` with trusted schemas only and `pg_temp` **last**, schema-qualified object references, and `REVOKE EXECUTE ... FROM PUBLIC`.[^16] Functions are executable by `PUBLIC` by default. On 14 the writable `public` schema makes the attack available to every role unless `CREATE` was revoked.

To change the default `EXECUTE` grant, run **`ALTER DEFAULT PRIVILEGES [FOR ROLE owner] REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC`** without `IN SCHEMA`. Measured on 14.24: the per-schema form `ALTER DEFAULT PRIVILEGES IN SCHEMA priv REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC` had **no effect** (a new function in `priv` was still executable by an unrelated role), because per-schema default privileges can only add to the global defaults. The global form worked (`permission denied for function f2`), but only for functions created by the role it was run as (or the `FOR ROLE`).[^30]

14.8 fixed CVE-2023-2454 (`CREATE SCHEMA ... schema_element` could defeat a `SECURITY DEFINER` function's `search_path`), and 14.11 fixed CVE-2024-0985 (`REFRESH MATERIALIZED VIEW CONCURRENTLY` running code as the refreshing user).[^5]

##### Roles and privileges

- The application connects as a role that **does not own** the schema objects and is not a superuser. Migrations run as a separate owner role.
- **`CREATEROLE` is almost superuser on 14.** Measured on 14.24: a role with only `CREATEROLE` ran `GRANT pg_execute_server_program TO cr` on itself and then `COPY pwn FROM PROGRAM 'id -un'`, which ran a shell command as the `postgres` OS user (the row read `postgres`). It cannot create superusers (`must be superuser to create superusers`), but it can grant itself membership in any non-superuser role, including `pg_execute_server_program`, `pg_read_server_files`, `pg_write_server_files`, `pg_read_all_data` and `pg_write_all_data`. 16 restricts this.[^8] Treat `CREATEROLE` as superuser-equivalent and flag any application or "tenant admin" role that has it.
- **A login role can grant itself to others on 14.** Logged in as `alice`, `GRANT alice TO bob` succeeded without any `ADMIN OPTION`; 15 rejects it (`must have admin option on role "alice"`). An application role that can run arbitrary SQL can thereby share its privileges.
- Use predefined roles instead of superuser: `pg_read_all_data`/`pg_write_all_data` (14), `pg_monitor`, `pg_signal_backend`, `pg_read_all_stats`. There is no `pg_checkpoint` (15) and no `GRANT SET ON PARAMETER` (15) on 14; `CHECKPOINT` and superuser-only settings require superuser.
- Passwords: `scram-sha-256` (default since 14). Never put passwords in SQL that gets logged (`ALTER ROLE ... PASSWORD 'x'` appears in the server log when statement logging is on, and in `pg_stat_activity` while it runs). Use psql's `\password`, which sends a hashed password.

##### Row-level security

```sql
ALTER TABLE docs ENABLE ROW LEVEL SECURITY;
ALTER TABLE docs FORCE ROW LEVEL SECURITY;   -- also apply to the table owner
CREATE POLICY tenant_isolation ON docs
  USING (tenant = current_setting('app.tenant', true))
  WITH CHECK (tenant = current_setting('app.tenant', true));
```

Behaviour verified on 14.24:[^24]

- **Superusers and roles with `BYPASSRLS` always bypass RLS**, even with `FORCE`.
- Without `FORCE ROW LEVEL SECURITY`, the **table owner bypasses** policies (a non-superuser owner saw both rows). With `FORCE` the owner's insert of a row for another tenant failed with `new row violates row-level security policy for table "docs"`: a `USING` policy also checks new rows when `WITH CHECK` is omitted.
- `current_setting('app.tenant', true)` returns NULL when the setting was never defined, and the policy then matches nothing (safe). `current_setting('app.tenant')` without `true` raises `unrecognized configuration parameter "app.tenant"`. **After a `set_config('app.tenant', 't2', true)` transaction ends, the setting reads as `''` (empty string), not NULL.** Policies must not treat `''` as a valid tenant.
- With transaction pooling, set the tenant with `SET LOCAL` / `set_config(..., true)` **inside each transaction**. A session-level `SET` leaks to the next client that gets the connection.
- `pg_read_all_data` does not bypass RLS (it saw 0 rows); `BYPASSRLS` does.
- User functions in policies or in queries on RLS tables may see rows before the policy filters them unless they are `LEAKPROOF`. Keep policies simple and indexable. 14.14 fixed CVE-2024-10976 (cached plans could ignore a role change when RLS applied to a subquery or CTE).[^5]

##### Views and row-level security on 14

A view accesses its tables with the **view owner's** privileges, and RLS policies are evaluated for the **view owner**. `security_invoker` views (which check the caller) are 15; on 14 the option is rejected with `unrecognized parameter "security_invoker"`.[^31]

```sql
-- owner_r owns r.docs (RLS enabled, no FORCE) and the view; reader may only read the view
CREATE VIEW r.v_definer AS SELECT * FROM r.docs;
GRANT SELECT ON r.v_definer TO reader;
SET ROLE reader;
SELECT count(*) FROM r.v_definer;   -- 2: every tenant's rows, the owner bypasses its own RLS
```

- Measured on 14.24: through the view, `reader` saw **all rows** of all tenants. That is a tenant-isolation hole whenever the view owner is the table owner (without `FORCE ROW LEVEL SECURITY`), a superuser, or has `BYPASSRLS`.
- After `ALTER TABLE r.docs FORCE ROW LEVEL SECURITY`, the same view returned 0 rows with no tenant set and 1 row after `set_config('app.tenant', 't1', true)`. The policy is then applied as the owner, so policies must be based on something the **caller** controls per transaction (`current_setting('app.tenant', true)`), not on `current_user` (which is the view owner inside the view).
- Options on 14: (1) `FORCE ROW LEVEL SECURITY` plus setting-based policies; (2) let a role without `BYPASSRLS` that does not own the table own the view (measured: a view owned by such a role, with `SELECT` on the table, returned only the rows of the tenant set in the transaction); (3) don't use views for tenant-scoped access, query the table directly; (4) a `SECURITY DEFINER` function that filters explicitly. Plan to switch to `WITH (security_invoker = true)` after upgrading.
- `WITH (security_barrier)` views (9.2+) prevent leaky functions in the caller's `WHERE` from seeing rows the view's own `WHERE` filters out. They do not change whose privileges are used.

##### Other security items

- `COPY ... FROM/TO 'file'` and `COPY ... PROGRAM` run on the server as the server's OS user (superuser or `pg_read_server_files`/`pg_write_server_files`/`pg_execute_server_program`). Treat them as remote code execution if inputs reach them. Application code should use `COPY ... FROM STDIN` through the driver.
- `dblink`/`postgres_fdw` connection strings built from user input are injection and SSRF vectors.
- `pg_dump` output is executable SQL. Since 14.19 `pg_dump` emits psql's `\restrict` to stop a malicious source server from injecting psql meta-commands (CVE-2025-8714/8715), and 14.24 closed a gap in that fix. Restore dumps with an up-to-date psql.[^5][^4]
- `pgcrypto`: use `aes256` with `pgp_sym_encrypt()`. 14.24 detects ciphers that OpenSSL refused (legacy ciphers in OpenSSL 3, FIPS mode); pgcrypto had silently XOR'd such data with an unencrypted block, making the "encryption" trivially breakable; 14.24 refuses to decrypt it unless `ignore-cipher-failure=1` is passed (CVE-2026-14663). Affected are typically `cipher-algo=bf`, `twofish`, `cast5` and `3des`. Measured on 14.24 in the official image (OpenSSL 3 without the legacy provider): `pgp_sym_encrypt('secret', 'k', 'cipher-algo=bf')` now fails with `encrypt error: Cipher cannot be initialized ?`, while `aes256` round-trips.[^4]
- **Keep the server on 14.24** until the upgrade. 14.24 alone fixed 28 CVEs, including memory-safety bugs in `to_char()`, the regexp functions and `ascii()`. After November 12, 2026, new vulnerabilities in 14 will not be fixed.[^4]

***

#### Data Types: Pitfalls

##### Timestamps

```sql
SET timezone = 'Europe/Berlin';
SELECT '2024-06-01 12:00+00'::timestamp;     -- 2024-06-01 12:00:00     (offset silently DROPPED)
SELECT '2024-06-01 12:00+00'::timestamptz;   -- 2024-06-01 14:00:00+02  (correct instant)
SELECT '2024-03-31 02:30'::timestamptz;      -- 2024-03-31 03:30:00+02  (nonexistent local time shifted)

-- DST: '1 day' is calendar-aware, '24 hours' is not
SELECT timestamptz '2024-10-27 00:00 Europe/Berlin' + interval '1 day';    -- 2024-10-28 00:00:00+01
SELECT timestamptz '2024-10-27 00:00 Europe/Berlin' + interval '24 hours'; -- 2024-10-27 23:00:00+01

SELECT (timestamptz '2024-01-31 00:00+00' + interval '1 month')::date;  -- 2024-02-29 (clamped)
```

- Use **`timestamptz`** for points in time.[^28] It stores UTC and converts on input and output using the session `TimeZone`. Plain `timestamp` drops offsets on input and is only right for "wall clock" values.
- `ts::timestamp AT TIME ZONE 'UTC'` on a `timestamptz` is a common bug. It converts to session-local wall time first and then interprets that as UTC: with a Berlin session, `(timestamptz '2024-06-01 12:00+00')::timestamp AT TIME ZONE 'UTC'` gave `2024-06-01 16:00:00+02`, two hours off. Use `ts AT TIME ZONE 'UTC'` (gives `timestamp` UTC wall time) or keep `timestamptz`.
- `now()`/`CURRENT_TIMESTAMP` is the **transaction start time**. Use `clock_timestamp()` for wall time inside long transactions.
- `timestamp(0)` **rounds**: `'2024-01-01 23:59:59.7'::timestamp(0)` is `2024-01-02 00:00:00`.
- **`EXTRACT()` returns `numeric` since 14** (`extract(epoch FROM ts)` is `1704067200.123456`), `date_part()` still returns `double precision`. Code ported from 13 that compares or formats the result can change: drivers decode `numeric` as decimal types, and `extract(second FROM timestamp '2024-01-01 00:00:01.5')` prints `1.500000`. `EXTRACT(hour FROM date)` is an error on 14.
- `date_bin()` (14) buckets by fixed lengths; see [date_bin](#new-in-14-date_bin-string_to_table-and-other-functions) for its DST behaviour. `date_add()` with a time zone is 16; `AT LOCAL` is 17.
- **Interval input on 14**: fractional years are truncated to months (`interval '1.99 years'` is `1 year 11 mons`; 15 rounds to `2 years`). An `ago` anywhere negates the whole value (`'1 day ago 2 hours'` is `-1 days -02:00:00`); 17 rejects `ago` that is not last, so write it last.

##### Numbers

```sql
SELECT 0.1::float8 + 0.2::float8 = 0.3::float8;   -- f
SELECT 0.1::numeric + 0.2 = 0.3;                  -- t
SELECT 1/2;                                       -- 0   (integer division)
SELECT round(2.5::numeric), round(2.5::float8);   -- 3, 2  (numeric: half away from zero; float: half to even on this platform)
SELECT '12.345'::numeric(5,2);                    -- 12.35 (silently rounded)
SELECT 2147483647::int + 1;                       -- ERROR: integer out of range
SELECT 1_000_000;                                 -- 1  (!) column "_000_000" on 14
```

- Money: `numeric(p,s)` or integer minor units (cents in `bigint`). Never `float`/`real`/`double precision`. Avoid the `money` type: its output depends on `lc_monetary` (`'1.5'::money` gave `$1.50`).
- Integer division truncates. Cast one side (`x::numeric / y`) when a fraction is expected.
- `numeric(p,s)` rounds extra scale silently and raises an error only when the integer digits overflow. Negative scales are 15.
- `numeric` accepts `Infinity` since 14; validate where infinity is not meaningful.
- Identifiers: `bigint`. A busy `int` primary key or sequence overflows at 2,147,483,647, and fixing it is a full table rewrite.
- **No `1_000_000`, `0xFF`, `0o17` or `0b101` literals on 14, and no error either**: they are read as a number followed by a column alias. Flag them in any SQL that targets 14.

##### Text

- Use `text` (or `varchar` without a length). `varchar(n)` only adds a length check; changing `n` later can need a rewrite (narrowing). Use `CHECK (length(x) <= n)` if a business rule needs a limit.
- **Never `char(n)`**: it pads with spaces and ignores trailing spaces when comparing (`'ab '::char(5) = 'ab'::char(5)` is true, `length()` is 2, `octet_length()` is 5).
- An explicit cast to `varchar(n)` **silently truncates** (`'abcdef'::varchar(3)` is `abc`). Inserting a too-long value into a `varchar(3)` column raises `value too long for type character varying(3)`.
- Case-insensitive matching: `lower(x)` with an expression index, `citext`, or a nondeterministic ICU collation (no `LIKE` with nondeterministic collations before 18).
- Regular expressions since 14: `\D`, `\W` (and `\S`) match newlines in newline-sensitive mode, `[[:word:]]` exists, and back-references ignore constraints. Re-test validation patterns ported from 13.

##### Identity vs serial

```sql
-- GOOD
CREATE TABLE s2 (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, v int);
-- AVOID in new code
CREATE TABLE s1 (id serial PRIMARY KEY, v int);
```

Measured differences:

- `serial` accepts explicit ids, which collide with the sequence later (`duplicate key value violates unique constraint "s1_pkey"`). `GENERATED ALWAYS` rejects explicit values (`cannot insert a non-DEFAULT value into column "id"`) unless the statement uses `OVERRIDING SYSTEM VALUE`.
- With `serial`, a role with `INSERT` on the table also needs `USAGE` on the sequence (`permission denied for sequence s1_id_seq`). Identity columns need no extra grant.
- After bulk-loading explicit ids, reset the sequence: `SELECT setval(pg_get_serial_sequence('s2','id'), (SELECT max(id) FROM s2));`.
- UUID keys: `gen_random_uuid()` (v4, core since 13) causes random B-tree inserts. There is no `uuidv7()` on 14 (18). Generate time-ordered UUIDs in the application if you need them.

##### NULL handling

```sql
SELECT NULL = NULL;                         -- NULL (not true)
SELECT NULL IS NOT DISTINCT FROM NULL;      -- t
SELECT count(*), count(x) FROM (VALUES (1),(NULL)) v(x);   -- 2, 1
SELECT (ARRAY[10,20,30])[0];                               -- NULL (arrays are 1-based)
SELECT array_length('{}'::int[], 1), cardinality('{}'::int[]);  -- NULL, 0
```

`array_length(empty, 1)` is NULL, so `array_length(a, 1) = 0` never matches an empty array. Use `cardinality(a) = 0`. `sum()` over no rows (or only NULLs) is NULL: use `coalesce(sum(x), 0)`.

##### Enums vs lookup tables vs CHECK

Enums are compact and ordered, but values cannot be removed or renamed easily, and a value added with `ADD VALUE` cannot be used until the transaction commits. Use a `CHECK (status IN (...))` or a foreign key to a lookup table when the set changes often.

##### Ranges and multiranges

Use range types (`tstzrange`, `daterange`) for periods, with `[)` bounds, and an exclusion constraint for "no overlapping bookings" (`EXCLUDE USING gist (room WITH =, during WITH &&)`, with `btree_gist` for the scalar column). Use multiranges (14) for sets of periods. Exclusion constraints cannot be declared on partitioned tables on 14 (17).

***

#### JSONB

PostgreSQL 14 has **no SQL/JSON standard functions** (`JSON_OBJECT`, `IS JSON` are 16; `JSON_TABLE`, `JSON_VALUE`, `JSON_QUERY`, `JSON_EXISTS` are 17). Use the `jsonb_*` functions, operators, subscripting (14) and jsonpath (12).

##### json vs jsonb

```sql
SELECT '{"a":1,"a":2}'::jsonb;        -- {"a": 2}           (last key wins, keys sorted)
SELECT '{"b":1,"a":2}'::json;         -- {"b":1,"a":2}      (text kept verbatim)
SELECT '{"n":1.0}'::jsonb = '{"n":1}'::jsonb;   -- t
```

Use `jsonb` for storage and querying. Use `json` only when the exact input text must be preserved.

##### Operators and extraction

```sql
SELECT doc -> 'a'   AS json_value,    -- jsonb
       doc ->> 'a'  AS text_value,    -- text
       doc['a']     AS subscript      -- jsonb (14); parenthesize casts: ('{}'::jsonb)['a']
FROM t;

SELECT ('{"a":"5"}'::jsonb ->> 'a')::int + 1;   -- 6
SELECT ('null'::jsonb)::int;                     -- ERROR: cannot cast jsonb null to type integer
SELECT '{"a":null}'::jsonb ->> 'a' IS NULL;      -- t: JSON null becomes SQL NULL via ->>

-- jsonpath with variables: never build jsonpath text by concatenation
SELECT jsonb_path_query('{"a":[1,2,3]}', '$.a[*] ? (@ > $min)', jsonb_build_object('min', $1));
```

- `jsonb_set(doc, path, NULL)` returns **NULL for the whole document** (the function is strict). `jsonb_set('{"a":1}', '{b}', to_jsonb(NULL::text))` returned NULL, so `UPDATE t SET doc = jsonb_set(doc, ..., $1)` with a NULL parameter wipes the column. Use `jsonb_set_lax(doc, path, value)` (13+, default `'use_json_null'`, gives `{"a": 1, "b": null}`), subscripting assignment (14, stores JSON `null`), or `coalesce`.
- Typed extraction: `(doc->>'k')::int`. It raises on bad data, which is usually what you want for imports. Check the type first with `jsonb_typeof()` when the field can legitimately vary.
- Validating JSON text without exceptions (`pg_input_is_valid`, `IS JSON`) is 16+. On 14, cast inside a PL/pgSQL `BEGIN ... EXCEPTION WHEN invalid_text_representation` block, or validate in the application.

##### Indexing JSONB (plans from 14.24)

```sql
CREATE INDEX evj_doc_gin ON evj USING gin (doc jsonb_path_ops);

EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc @> '{"type":"click"}';
-- Bitmap Index Scan on evj_doc_gin  Index Cond: (doc @> '{"type": "click"}'::jsonb)
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc @? '$.user ? (@.id == 5)';
-- Bitmap Index Scan on evj_doc_gin
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc ? 'type';
-- Seq Scan  (jsonb_path_ops does not support ?, ?|, ?&; the default jsonb_ops does)
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc['type'] = '"click"';
-- Seq Scan  (GIN does not help subscript or ->> equality)

CREATE INDEX evj_type ON evj ((doc->>'type'));
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc->>'type' = 'click';
-- Bitmap Index Scan on evj_type
```

- `jsonb_path_ops` is smaller and faster for `@>`, `@?` and `@@`. The default `jsonb_ops` also supports `?`, `?|` and `?&`.
- For a frequently filtered scalar, use a B-tree expression index on `(doc->>'key')`, or better, promote it to a real column. An index on `(doc->>'type')` is not used by `doc['type'] = '"click"'`, and vice versa: the query must repeat the indexed expression.
- JSONB is not a replacement for a schema. Fields that are always present, used in joins, constrained, or updated often belong in columns. Updating one key rewrites the whole (possibly TOASTed) document and its GIN entries; subscripting assignment does not change that.

***

#### Partitioning

```sql
CREATE TABLE m (
  id bigint GENERATED ALWAYS AS IDENTITY,
  at timestamptz NOT NULL,
  v  int,
  PRIMARY KEY (id, at)                              -- must include the partition key
) PARTITION BY RANGE (at);
CREATE TABLE m_2025 PARTITION OF m FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE m_2026 PARTITION OF m FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');
```

Rules verified on 14.24:[^25]

- **Unique and primary keys must include all partition key columns**: `PRIMARY KEY (id)` on a table partitioned by `at` fails with `unique constraint on partitioned table must include all partitioning columns`. Global uniqueness of `id` alone is not enforced.
- **Identity columns on a partitioned parent work only through the parent.** `INSERT INTO m(at) ...` got an id from the parent's sequence, but `INSERT INTO m_2025(at) ...` directly into a partition failed with `null value in column "id" of relation "m_2025" violates not-null constraint`, because partitions do not share the identity before 17. Code and ETL tools that write directly into partitions need explicit ids or a sequence default instead.
- **Pruning needs the raw partition key.** `WHERE at >= '2025-03-01' AND at < '2025-04-01'` scanned only `m_2025`. `WHERE date_trunc('month', at) = '2025-03-01'` scanned **every** partition. Stable expressions like `now() - interval '1 day'` are pruned at execution time (`Subplans Removed: 1` in `EXPLAIN ANALYZE`). Since 14, `UPDATE`/`DELETE` on partitioned tables also use execution-time pruning.
- **Default partitions are a trap.** `ATTACH PARTITION` for a new range took `ACCESS EXCLUSIVE` on the default partition and scanned it. With a matching row already in the default partition, the attach failed (`updated partition constraint for default partition "ap_def" would be violated by some row`). `DETACH ... CONCURRENTLY` is refused while a default partition exists. Create future partitions ahead of time (cron, `pg_partman`) and alert when the default partition receives rows.
- **Attaching without a long scan**: add a `CHECK` constraint on the staging table that matches the partition bounds (validated before attaching). `ATTACH PARTITION` then takes only `SHARE UPDATE EXCLUSIVE` on the parent.
- **Indexes**: `ON ONLY` + per-partition `CREATE INDEX CONCURRENTLY` + `ALTER INDEX ... ATTACH PARTITION` (see migrations). `REINDEX ... CONCURRENTLY` works on the partitioned table since 14.
- **No exclusion constraints on partitioned tables on 14** (`exclusion constraints are not supported on partitioned tables`; 17). Enforce non-overlap per partition or in a non-partitioned table.
- **No `CLUSTER` on a partitioned table** (15); cluster partitions individually.
- Retention: `ALTER TABLE m DETACH PARTITION m_2023 CONCURRENTLY; DROP TABLE m_2023;` instead of large `DELETE`s (requires no default partition, and runs outside a transaction block).
- Autovacuum never analyzes the **parent** of a partitioned table, so run `ANALYZE parent` after large data changes if queries rely on parent-level statistics.
- Too many partitions (thousands) slow planning and locking. Partition for retention, pruning or maintenance, not by default.
- 14.18 found that self-referential foreign keys on partitioned tables could be enforced incorrectly; such constraints may need to be recreated.[^5]

***

#### SQL Idioms and Pitfalls

All outputs in this section are from 14.24.

##### Aggregates and NULL

```sql
SELECT pg_typeof(sum(1::int)), pg_typeof(sum(1::bigint)), pg_typeof(avg(1::int));
-- bigint | numeric | numeric

SELECT 'a' || NULL, concat('a', NULL), concat_ws(',', 'a', NULL, 'b');
-- NULL | a | a,b
```

- `||` with a NULL operand yields NULL. Use `concat_ws()` or `coalesce` for names built from optional parts.
- `avg()` and `sum()` ignore NULLs. If NULL means zero, write `avg(coalesce(x, 0))`, and say so explicitly.
- `sum(int)` returns `bigint` and `sum(bigint)` returns `numeric`. Driver code that maps the result to a 32-bit integer can overflow or fail to decode.
- `count(*) FILTER (WHERE ...)` instead of `sum(CASE ...)`.
- `string_agg(x, ',' ORDER BY x)` and `array_agg(x ORDER BY x)`: without `ORDER BY` inside the aggregate, the order is unspecified.
- `date - date` is an `integer` number of days. `age()` returns a calendar `interval` (`age('2024-03-01', '2024-01-31')` is `1 mon 1 day`); don't use `extract(day FROM age(...))` as "days between".
- No `any_value()` on 14 (16).

##### Regular expressions without the 15 functions

```sql
SELECT 'abcabc' ~ 'b';                                         -- t        (instead of regexp_like)
SELECT (regexp_match('a1b22c333', '\d+'))[1];                  -- 1        (first match; instead of regexp_substr)
SELECT (SELECT count(*) FROM regexp_matches('abcabc', 'b', 'g'));  -- 2   (instead of regexp_count)
SELECT regexp_replace('a-a-a', 'a', 'X', 'g');                 -- X-X-X    (all matches need the 'g' flag)
```

Patterns from user input are a denial-of-service risk (catastrophic backtracking) and raise errors on invalid syntax. Use `LIKE` with escaping, or validate patterns, and set `statement_timeout`. 14.24 fixed a buffer overrun in the regexp match/split functions with invalidly encoded input (CVE-2026-14664).[^4]

##### Window functions

- `row_number()` over a non-unique `ORDER BY` is **nondeterministic**. "Latest row per group" queries must add a unique tie-breaker (`ORDER BY at DESC, id DESC`).
- **The default frame is `RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW`.** `last_value(amount) OVER (ORDER BY at)` returned `10, 30, 20` (the current row's value), not the last value `20`. Write `ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING`, or use `first_value(...) OVER (ORDER BY at DESC)`.
- `DESC` sorts NULLs **first** by default. Add `NULLS LAST` when ranking by nullable values.

##### Top-N per group

```sql
-- DISTINCT ON: one row per group; the ORDER BY must start with the DISTINCT ON keys
SELECT DISTINCT ON (region) region, rep, amount
FROM sales
ORDER BY region, amount DESC NULLS LAST, rep;      -- rep breaks ties deterministically

-- LATERAL: N rows per group, index-driven (index on ord(cust_id, at DESC))
SELECT c.id, o.id
FROM cust c
CROSS JOIN LATERAL (
  SELECT id FROM ord WHERE ord.cust_id = c.id ORDER BY at DESC LIMIT 2
) o;
```

`CROSS JOIN LATERAL` drops parents without children. `LEFT JOIN LATERAL (...) o ON true` keeps them with NULLs.

##### Full-text search

```sql
SELECT to_tsvector('english', 'The quick brown foxes jumped')
       @@ websearch_to_tsquery('english', 'fox -cat');     -- t
SELECT to_tsquery('english', 'fox & (');                   -- ERROR: no operand in tsquery
SELECT websearch_to_tsquery('english', 'fox & (');         -- 'fox' (never raises on user input)
```

- Pass user search input to `websearch_to_tsquery()` or `plainto_tsquery()`, never to `to_tsquery()`.
- Always pass the configuration (`'english'`) explicitly. The one-argument form uses `default_text_search_config` and cannot be used in an index expression. Queries must use the same configuration as the index.
- Store a `tsvector` as a `GENERATED ALWAYS AS (...) STORED` column with a GIN index for large tables.
- 14 changed how `to_tsquery()`/`websearch_to_tsquery()` handle discarded tokens such as `_` (`to_tsquery('pg_class <-> pg')` is `'pg' <-> 'class' <-> 'pg'` on 14, `( 'pg' & 'class' ) <-> 'pg'` on 13). Search results for such input differ after an upgrade from 13.

##### Generated columns

```sql
CREATE TABLE gen(price numeric, qty int,
                 total numeric GENERATED ALWAYS AS (price * qty) STORED);
INSERT INTO gen(price, qty) VALUES (2, 3) RETURNING total;   -- 6
INSERT INTO gen VALUES (1, 1, 5);
-- ERROR: cannot insert a non-DEFAULT value into column "total"
CREATE TABLE gen2(t timestamptz, d date GENERATED ALWAYS AS (t::date) STORED);
-- ERROR: generation expression is not immutable
```

On 14 every generated column must be `STORED` (virtual columns are 18), and the expression cannot be changed in place (`SET EXPRESSION` is 17): drop and re-add the column, which rewrites the table. Always list target columns in `INSERT`.

##### Materialized views

- `REFRESH MATERIALIZED VIEW` takes `ACCESS EXCLUSIVE` and blocks readers for the whole refresh. Since 14 the refresh query can run in parallel.
- `REFRESH MATERIALIZED VIEW CONCURRENTLY` needs a **unique index without a `WHERE` clause**. With only a partial unique index it fails: `cannot refresh materialized view "public.mv" concurrently` / `HINT: Create a unique index with no WHERE clause on one or more columns of the materialized view.` It cannot be used for the first population and is slower than a full refresh when most rows change.
- A materialized view is a snapshot. Code that reads it must tolerate stale data, and something must schedule the refresh.

##### Collation-dependent results

```sql
SELECT 'Z' < 'a' COLLATE "C"          AS c_coll,  -- t   (byte order)
       'Z' < 'a' COLLATE "en_US.utf8" AS en;      -- f   (linguistic order)
```

Sort order and range comparisons on text depend on the collation. Keyset pagination and range filters on text columns must use the same collation as the index, and tests run under a different database locale (for example `C` in CI, `en_US.utf8` in production) can hide ordering bugs. OS upgrades that change glibc collation rules (glibc 2.28, for example Debian 9 → 10 or RHEL 7 → 8) corrupt text indexes. 14 does not record the version of the database's default collation (`pg_database.datcollversion` is 15), so it does not warn when that changes; reindex text indexes after any glibc change.

***

#### Functions, Procedures and Triggers

- **Volatility labels are promises.** `IMMUTABLE` means "same result for the same arguments forever, no table access". A function that reads tables, depends on settings (`TimeZone`, `DateStyle`, `search_path`), or calls `now()` must be `STABLE` or `VOLATILE`. A mislabeled `IMMUTABLE` function in an index gives wrong query results after data changes.
- Prefer SQL-standard bodies (`BEGIN ATOMIC ... END` or `RETURN expr`, 14) for SQL functions: parsed at creation, dependencies tracked, name resolution fixed. Remember that they make `DROP`/`ALTER` of referenced objects fail.
- Set `search_path` on functions that use unqualified names: a must for `SECURITY DEFINER` functions, and strongly advised for functions used by indexes, constraints and materialized views so the code survives the upgrade to 17+.
- PL/pgSQL: `RAISE EXCEPTION` with `USING ERRCODE`, `GET STACKED DIAGNOSTICS` in exception handlers. `EXCEPTION` blocks create a subtransaction per execution, which is costly in loops and consumes subtransaction IDs. Since 14, PL/pgSQL can assign to array slices and nested record fields, and `RETURN QUERY` can run in parallel.
- Procedures (`CREATE PROCEDURE`, `CALL`) can `COMMIT` inside, which suits batch jobs, but not when called inside an outer transaction block (`invalid transaction termination`). `OUT` parameters (14) must be passed as placeholders in `CALL`.
- Triggers: `BEFORE` row triggers for normalization and validation, `AFTER` for side effects. Statement-level triggers with transition tables (`REFERENCING NEW TABLE AS ...`) are cheaper for bulk changes. `CREATE OR REPLACE TRIGGER` (14).
- **PL/Python**: write new code for `plpython3u` only. `plpythonu`/`plpython2u` are removed in 15, so 14 is the last release where Python 2 functions can exist; port them before upgrading. Untrusted languages (`*u`) run as the server OS user; only superusers can create such functions.
- `ALTER FUNCTION ... SET work_mem/plan_cache_mode` scopes settings to one function.

***

#### Connection Pooling and Application Access

- **Always pool.** Each PostgreSQL connection is a process with several MB of memory. 14's snapshot scalability work makes many idle connections cheaper, but pool sizes should still be a small multiple of CPU cores, not the number of application threads.
- **PgBouncer transaction mode** forbids session state across transactions: `SET` without `LOCAL`, session advisory locks, `LISTEN`, `WITH HOLD` cursors, temporary tables, and SQL-level `PREPARE`/`EXECUTE`.[^29] Use `SET LOCAL`/`set_config(..., true)` inside transactions, and `pg_advisory_xact_lock`.
- **Protocol-level prepared statements** work in transaction mode with PgBouncer 1.21+ when `max_prepared_statements` is non-zero. SQL `PREPARE` statements are still not tracked.[^29]
- `idle_in_transaction_session_timeout` and `idle_session_timeout` (14) **terminate** the server connection. Pools must validate connections and drivers must retry.
- `target_session_attrs=read-write|read-only|primary|standby|prefer-standby` (libpq 14) routes connections in multi-host connection strings; check `in_hot_standby` (14) from SQL.
- Set `application_name` per service so `pg_stat_activity` and the logs identify the caller.
- Retries: retry on `40001` (serialization), `40P01` (deadlock), and connection loss before commit. Never blindly retry after an unknown commit outcome for non-idempotent writes. Use idempotency keys.
- `sslmode=verify-full` for connections over untrusted networks.
- Use `COPY FROM STDIN` (driver copy APIs) or multi-row `INSERT` for bulk writes instead of row-by-row inserts. `pg_stat_progress_copy` (14) shows progress.

***

#### Maintenance, Monitoring and Observability

```sql
-- Slow statements (requires shared_preload_libraries = 'pg_stat_statements' + CREATE EXTENSION)
SELECT query, calls, total_exec_time, mean_exec_time, blk_read_time, toplevel   -- 14 column names
FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT 10;

-- Tables with heavy sequential scans (columns are relname, not tablename)
SELECT schemaname, relname, seq_scan, seq_tup_read, idx_scan, n_dead_tup, last_autovacuum
FROM pg_stat_user_tables ORDER BY seq_tup_read DESC LIMIT 10;

-- Unused indexes (check replicas too; counters reset with stats resets)
SELECT schemaname, relname, indexrelname, idx_scan, pg_size_pretty(pg_relation_size(indexrelid))
FROM pg_stat_user_indexes WHERE idx_scan = 0
ORDER BY pg_relation_size(indexrelid) DESC;

-- Long or idle-in-transaction sessions
SELECT pid, state, now() - xact_start AS xact_age, wait_event_type, wait_event, query_id, query
FROM pg_stat_activity WHERE xact_start IS NOT NULL ORDER BY xact_start;

-- Who blocks whom, and since when (waitstart is 14)
SELECT a.pid, pg_blocking_pids(a.pid) AS blocked_by, l.waitstart, a.query
FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid AND NOT l.granted;

-- Checkpoints (pg_stat_bgwriter until 16)
SELECT checkpoints_timed, checkpoints_req, checkpoint_write_time, buffers_checkpoint, buffers_backend
FROM pg_stat_bgwriter;
```

- `pg_stat_user_tables` has `relname`; `pg_stat_user_indexes` has `relname` and `indexrelname`. Queries that use `tablename`/`indexname` (columns of `pg_tables`/`pg_indexes`) fail with `column "tablename" does not exist`.
- Monitoring written for 15+ fails on 14: `pg_stat_io`, `last_seq_scan`, `pg_stat_checkpointer`, `pg_stat_subscription_stats`, `shared_blk_read_time` and `temp_blk_read_time` do not exist. Monitoring written for 14 fails on 13 (`pg_stat_wal`, `query_id`, `waitstart`).
- On 14, cumulative statistics are kept by the **statistics collector** process and written to `stats_temp_directory`; they are lost after a crash or immediate shutdown. Don't base "unused index" decisions on counters that were reset recently (`pg_stat_database.stats_reset`).
- Autovacuum: tune per table for large, hot tables (`ALTER TABLE t SET (autovacuum_vacuum_scale_factor = 0.01)`) rather than turning it off. Watch `n_dead_tup`, `last_autovacuum`, and `age(datfrozenxid)` for wraparound; the 14 failsafe (`vacuum_failsafe_age`, default 1.6 billion) is a last resort, not monitoring.
- Logging on 14 is quiet by default: `log_checkpoints = off` and `log_autovacuum_min_duration = -1`. Set `log_checkpoints = on`, `log_autovacuum_min_duration`, `log_min_duration_statement`, `log_lock_waits = on`, `log_temp_files = 0`, and `auto_explain` for slow plans. Structured logs: `csvlog` (`jsonlog` is 15).
- Plain `REINDEX` blocks writes. Use `REINDEX ... CONCURRENTLY` (14.4+), and only when bloat, corruption or a release note warrants it.

***

#### Upgrading Off PostgreSQL 14

PostgreSQL 14 receives its final release on November 12, 2026. Plan the upgrade now, and go directly to a current release (18, supported until November 14, 2030, or 17, until November 8, 2029); `pg_upgrade` and dump/restore can skip major versions.[^2][^33] Do not target 15 (end of life November 11, 2027) or 16 (November 9, 2028) for a new upgrade project unless an external constraint forces it. PostgreSQL 19 is a release candidate in October 2026; wait for GA and the first minor releases before planning on it.

##### Methods

- **`pg_upgrade --link`** (minutes of downtime, needs the same host or storage, and a tested rollback plan because the old cluster cannot be started after the new one has run). `pg_upgrade --check` first.
- **Dump/restore** (`pg_dump -Fd -j N` / `pg_restore -j N`) for small databases or when changing hosts, encodings or collations.
- **Logical replication** from 14 to the new version for near-zero downtime: publish all tables, subscribe on the new cluster, wait for catch-up, switch writers. Sequences, DDL and large objects are not replicated; copy sequence values at switchover. Tables that receive `UPDATE`/`DELETE` need a primary key or another `REPLICA IDENTITY`, or the publisher rejects those writes once the table is published.
- Managed services (RDS, Cloud SQL, Azure) have their own major-version upgrade tooling and their own end-of-support dates for 14; check those and any extended-support fees.

##### What breaks on the way from 14 (checklist)

1. **Backups (15)**: `pg_start_backup()`/`pg_stop_backup()`/`pg_is_in_backup()` and exclusive backups are gone. Move scripts to `pg_basebackup` or a maintained tool (pgBackRest, Barman, WAL-G), or to `pg_backup_start()`/`pg_backup_stop()` in one session.
2. **PL/Python 2 (15)**: port `plpythonu`/`plpython2u` functions to `plpython3u` before the upgrade.
3. **Configuration (15–18)**: remove `stats_temp_directory` (15), `force_parallel_mode`, `vacuum_defer_cleanup_age`, `promote_trigger_file` (16), `old_snapshot_threshold`, `db_user_namespace` (17); a server with an unknown core setting in `postgresql.conf` refuses to start.
4. **Lexer (15)**: `123abc`, `1_000` and similar glued literals become errors on 15 (and valid numbers on 16+, with a different value than on 14). Search generated SQL and literals.
5. **`interval::text` indexes (15)** fail to rebuild; `interval '1.99 years'` changes value.
6. **`public` schema (15)**: upgraded databases keep the 14 ACL; new databases created after the upgrade do not grant `CREATE` to `PUBLIC`. Revoke it on 14 now so both behave the same.
7. **Roles (15, 16)**: login roles lose implicit `ADMIN OPTION` on themselves (15); `CREATEROLE` can no longer administer roles it lacks `ADMIN OPTION` on, and role membership gains `INHERIT`/`SET` options (16). Provisioning scripts need review.
8. **`search_path` during maintenance (17)**: functions used by expression indexes, index predicates and materialized views must have `SET search_path` or qualified names, or `ANALYZE`, `REINDEX` and `REFRESH` fail after the upgrade.
9. **Monitoring (15, 17)**: `pg_stat_bgwriter.checkpoints_*`/`buffers_backend` move to `pg_stat_checkpointer`/`pg_stat_io` (17); `pg_stat_statements.blk_read_time` becomes `shared_blk_read_time` (17); `log_checkpoints` and autovacuum logging are on by default (15).
10. **Memory (15)**: `hash_mem_multiplier` defaults to 2.0; re-check `work_mem` sizing.
11. **Random values (15)**: `setseed()` + `random()` sequences change; fix tests that hard-code them.
12. **Interval syntax (17)**: `ago` must be the last token.
13. **18 specifics**: `initdb` enables data checksums by default, and `pg_upgrade` requires matching checksum settings (use `initdb --no-data-checksums` for the new cluster, or enable checksums on the old one with `pg_checksums` while it is stopped); MD5 passwords are deprecated with warnings.[^10]
14. **After the upgrade**: run `vacuumdb --all --analyze-in-stages` (targets before 18 start without optimizer statistics), reindex text indexes if the OS collation library changed, and apply the post-update steps of the new version's latest minor release.

##### If you must stay on 14 a little longer

- Run **14.24**, then the final 14.x release of November 12, 2026, and nothing older.
- Revoke `CREATE` on `public`, remove `CREATEROLE` from non-admin roles, and restrict network access (`pg_hba.conf`, firewalls) to reduce exposure to vulnerabilities that will not be fixed.
- Write new SQL so that it already runs on 17/18: no glued numeric literals, `SET search_path` on functions used by indexes, no exclusive backups, no PL/Python 2.

***

#### Production / Review Checklist

##### Version compatibility

- [ ] No PostgreSQL 15+ features: `MERGE`, `NULLS NOT DISTINCT`, `security_invoker`, `regexp_count()`/`regexp_like()`/`regexp_substr()`/`regexp_instr()`, `ON DELETE SET NULL (col)`, `GRANT SET ON PARAMETER`, `pg_checkpoint`, `jsonlog`, `pg_backup_start()`, publication row filters/column lists
- [ ] No 16+ features: SQL/JSON (`JSON_OBJECT`, `IS JSON`), `pg_input_is_valid()`, `any_value()`, unaliased `FROM` subqueries, `GRANT ... WITH INHERIT/SET`, `EXPLAIN (GENERIC_PLAN)`; **no `1_000` or `0x1F` literals** (silently wrong on 14)
- [ ] No 17+/18+ features: `JSON_TABLE`, `JSON_VALUE`, `COPY ON_ERROR`, `transaction_timeout`, `MAINTAIN`, `random(min,max)`, `pg_stat_checkpointer`, `uuidv7()`, virtual generated columns, `RETURNING old/new`
- [ ] Code shared with 13 does not use 14 features (jsonb subscripting, multiranges, `date_bin()`, `string_to_table()`, `SEARCH`/`CYCLE`, procedure `OUT` parameters, `BEGIN ATOMIC`, `CREATE OR REPLACE TRIGGER`, `pg_read_all_data`)
- [ ] No removed 14 features: postfix/factorial operators, `@`/`~` containment operators, boolean `password_encryption`, `EXTRACT()` assumed to be `double precision`
- [ ] Infrastructure pins flag the November 12, 2026 end of life and an upgrade plan; server on 14.24 (or the final 14.x)

##### Migrations

- [ ] `lock_timeout` set (with retry) for every DDL on existing tables; `SET LOCAL` inside transactions
- [ ] `CREATE INDEX CONCURRENTLY` / `DROP INDEX CONCURRENTLY` on live tables, outside transaction blocks; invalid indexes handled on retry; indexes built concurrently on 14.0–14.3 rebuilt
- [ ] Foreign keys and `CHECK` constraints added `NOT VALID` then `VALIDATE`d; `SET NOT NULL` preceded by a validated `CHECK (col IS NOT NULL)`
- [ ] No volatile defaults or type changes on large tables without a rewrite plan
- [ ] Backfills and deletes in batches
- [ ] Expand/contract for renames and drops; new enum values committed before use
- [ ] `REVOKE CREATE ON SCHEMA public FROM PUBLIC`; objects in an application schema
- [ ] Functions used by indexes or materialized views have `SET search_path` (future 17 upgrade)
- [ ] Foreign key columns indexed
- [ ] psql migration scripts run with `ON_ERROR_STOP=1`; inline `COPY` data terminated with `\.`

##### Queries

- [ ] Parameters bound (no string concatenation), `= ANY($1)` for lists, identifiers allow-listed
- [ ] Predicates sargable: no functions or casts on indexed columns, half-open time ranges, parameter types match column types
- [ ] Keyset pagination with a row-value comparison and a matching composite index
- [ ] `NOT EXISTS` instead of `NOT IN (subquery)` on nullable columns; correlated `IN` rewritten as `EXISTS`
- [ ] Composite indexes match leading columns (no skip scan); prefix searches use `LIKE 'x%'`, not `starts_with()`
- [ ] Recursive CTEs have cycle protection (`CYCLE` or a depth limit)
- [ ] `EXPLAIN (ANALYZE, BUFFERS)` evidence for performance claims; DML explained inside `BEGIN ... ROLLBACK`

##### Transactions and concurrency

- [ ] Read-modify-write made atomic (`UPDATE ... SET x = x + ...`, `FOR UPDATE`, or version checks)
- [ ] Retry loops for `40001`/`40P01` with `REPEATABLE READ`/`SERIALIZABLE`
- [ ] Upserts use `INSERT ... ON CONFLICT` with deduplicated input
- [ ] Short transactions; no network calls inside; timeouts set per role (`idle_session_timeout` only where no pool is used)
- [ ] Queues use `FOR UPDATE SKIP LOCKED`; advisory locks are transaction-scoped

##### Security

- [ ] `SECURITY DEFINER` functions: `SET search_path = ..., pg_temp`, qualified names, `REVOKE EXECUTE ... FROM PUBLIC`
- [ ] Default `EXECUTE` revoked with the global `ALTER DEFAULT PRIVILEGES` form, not `IN SCHEMA`
- [ ] Dynamic SQL uses `format('%I')` + `USING`, never `%s` with input
- [ ] App role is not owner, superuser or `CREATEROLE`, and not a member of `pg_write_all_data`; RLS tables use `FORCE ROW LEVEL SECURITY` where owners connect; tenant set with `SET LOCAL`; `''` not accepted as a tenant
- [ ] Views over RLS-protected tables do not leak rows through the owner's privileges (no `security_invoker` on 14)
- [ ] No server-side `COPY` file/program from application paths; no passwords in logged SQL; SCRAM passwords

##### Data types and schema

- [ ] `timestamptz` for instants; `bigint` identity for keys; `numeric` or integer minor units for money; `text` + `CHECK` instead of `char(n)`
- [ ] `GENERATED ALWAYS AS IDENTITY` instead of `serial`; no direct inserts into partitions of a table whose identity lives on the parent
- [ ] NULL-tolerant uniqueness handled with a partial unique index (no `NULLS NOT DISTINCT`)
- [ ] JSONB only for genuinely flexible data; subscript results treated as `jsonb` (`->>` for text); GIN `jsonb_path_ops` for `@>`/`@?`; expression indexes match the query expression; `jsonb_set_lax` or NULL guards
- [ ] Partitioned tables: keys include the partition key, queries filter on the raw key, future partitions pre-created, default partition monitored
- [ ] `date_bin()` not used for calendar days or months in DST time zones

##### Operations

- [ ] Pooling in place; no session state with transaction pooling
- [ ] `pg_stat_statements`, `log_lock_waits`, `log_min_duration_statement`, `log_checkpoints` configured; monitoring queries use 14 column names
- [ ] Backups via `pg_basebackup`/maintained tools or non-exclusive `pg_start_backup(label, fast, false)`/`pg_stop_backup(false)` in one session, never exclusive mode
- [ ] Post-update steps of 14.x minors applied (14.4 concurrent indexes, 14.12 `pg_stats_ext` fix, 14.24 `output_plugin_libraries`, pgcrypto, `btree_gist`/`ltree` reindex)
- [ ] Upgrade to 17/18 scheduled before or shortly after November 12, 2026
