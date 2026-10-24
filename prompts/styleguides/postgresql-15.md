### PostgreSQL 15: The Complete Review Guide

#### Overview

> **Version**: PostgreSQL 15.0 was released on **October 13, 2022**. The latest minor release is **15.19 (August 13, 2026)**; the next scheduled minor release date is November 12, 2026. PostgreSQL 15 is **supported until November 11, 2027**, so it has roughly one year of community support left.
> **Newest releases (October 2026)**: PostgreSQL **18** is the current stable major version (18.0 on September 25, 2025; current minor 18.6). 17 (17.11) and 16 (16.15) are also supported. PostgreSQL **19 is at Release Candidate 1** (October 15, 2026). It is pre-release software and not a deployment target. PostgreSQL 14 reaches end of life on November 12, 2026.
> **Scope**: SQL that runs on a PostgreSQL 15 server: schema migrations, queries embedded in application code, functions, procedures and triggers, schema design, indexing, transactions, security, JSON, partitioning, logical replication, connection pooling and operations-facing SQL.
> **Note**: This document is self-contained. It carries forward everything from PostgreSQL 14 and earlier that is still correct, and it marks the version in which each feature arrived. Sections that cover 15-specific behaviour are marked **New in 15**. Every example was executed on PostgreSQL 15.19. "New in 15" claims were shown to fail (or behave differently) on 14.24, and every item on the "Do not use" list was shown to fail on 15.19.

PostgreSQL 15 is the release that brought the **SQL `MERGE` command**, **`UNIQUE NULLS NOT DISTINCT`**, **`security_invoker` views**, **row filters, column lists and `FOR TABLES IN SCHEMA` for logical replication publications**, the **`regexp_count()` / `regexp_instr()` / `regexp_like()` / `regexp_substr()`** functions, **LZ4 and Zstandard compression** for WAL full-page images and base backups, **`log_destination = jsonlog`**, **ICU collations as cluster and database defaults** with recorded collation versions, **per-parameter `GRANT SET` / `GRANT ALTER SYSTEM`**, the **`pg_checkpoint` role**, cumulative statistics kept **in shared memory** (no more statistics collector process), and faster in-memory and on-disk sorts.

PostgreSQL 15 also changes behaviour that breaks old code and deployments. **`PUBLIC` no longer has `CREATE` on the `public` schema** in newly created databases, and `public` is owned by `pg_database_owner`. **Exclusive backup mode is gone** (`pg_start_backup()`/`pg_stop_backup()` are renamed `pg_backup_start()`/`pg_backup_stop()`, `pg_is_in_backup()` is removed). **PL/Python 2 (`plpythonu`, `plpython2u`) is removed.** `stats_temp_directory` is removed. Numeric literals followed by letters (`123abc`) are now a syntax error, fractional years in interval input round to the nearest month, `interval` text output is no longer immutable, `random()` returns different values for the same seed, and `log_checkpoints` and `log_autovacuum_min_duration` now log by default.

The SQL/JSON standard functions (`JSON_TABLE`, `JSON_VALUE`, `JSON_QUERY`, `JSON_EXISTS`, `JSON_OBJECT`, `IS JSON`, ...) appeared in the 15 beta releases but **were removed before 15.0** (15 Beta 4, September 2022). They are not in any 15.x release. Do not trust blog posts or old beta docs that show them for 15.

Global truths about PostgreSQL 15:

- **MVCC.** Readers never block writers and writers never block readers at the row level. `UPDATE` writes a new row version and `DELETE` marks the old one dead. `VACUUM` (usually autovacuum) reclaims dead versions. Long transactions stop that reclamation everywhere in the cluster.
- **DDL is transactional, but it takes locks.** Almost every `ALTER TABLE` form takes an `ACCESS EXCLUSIVE` lock. The lock is held until commit, and it queues behind running queries while blocking new ones (see [Schema migrations and locking](#schema-migrations-and-locking)).
- **Default isolation is READ COMMITTED.** Each statement sees a fresh snapshot. A read-modify-write in application code is a lost update unless it is made atomic.
- **NULL is three-valued.** `NULL = NULL` is NULL, `NOT IN` with a NULL in the list never returns true, and `count(col)` skips NULLs.
- **Identifiers fold to lower case** unless they are double-quoted. String literals use single quotes.
- **Arrays are 1-based.** `arr[0]` is NULL, not an error.
- **Use `timestamptz`, `text`, `bigint` identity and `numeric` for money** by default. The rest of this guide explains why.

***

#### Version-Aware Review

The project pins PostgreSQL 15 (for example `image: postgres:15`, `postgresql 15.x` in `.tool-versions`, a managed-database engine version of 15, or a CI service container). Apply these rules when reviewing code for it:

- **Verify before you claim something is unavailable.** A feature you remember as "new" may already be in 15 (for example `MERGE`, `UNIQUE NULLS NOT DISTINCT`, `security_invoker`, `regexp_count()`, `ON DELETE SET NULL (col)`). Say "not available in PostgreSQL 15" only for items on the [Do Not Use or Recommend](#do-not-use-or-recommend-postgresql-16-features) list or when the 15 documentation proves it.
- **Never suggest PostgreSQL 16, 17 or 18 features.** The most commonly suggested ones are the SQL/JSON functions (`JSON_TABLE`, `JSON_VALUE`, `JSON_OBJECT`, `IS JSON`), `MERGE ... RETURNING`, `WHEN NOT MATCHED BY SOURCE`, `pg_input_is_valid()`, `any_value()`, `transaction_timeout`, `uuidv7()`, subqueries in `FROM` without an alias, and numeric literals with underscores. They fail on 15 with a syntax error or "does not exist".
- **Flag code that only works on newer servers.** Example: `SELECT * FROM (SELECT 1)` is valid on 16+ and fails on 15 with `subquery in FROM must have an alias`. ORMs and query builders generated for 16+ can emit it.
- **Flag code that assumes older behaviour.** Example: a migration that runs `CREATE TABLE public.x` as a non-owner role worked on 14 and fails on a freshly created 15 database with `permission denied for schema public`. Backup scripts that call `pg_start_backup()` fail with `function pg_start_backup(unknown) does not exist`.
- **Many projects run several versions.** If the code also has to run on 14 (a library, an extension, a shared migration set), every **New in 15** feature in this guide is a portability bug there. PostgreSQL 14 is end of life on November 12, 2026, so a 14 target is itself a finding.
- **15 is in its final support year.** Code reviews of infrastructure (Terraform, Helm values, Dockerfiles) that pin 15 should note the November 11, 2027 end of life and plan the upgrade path (see [Upgrading](#upgrading-to-15-and-staying-current)).
- **Client libraries are separate.** libpq features such as `require_auth`, `load_balance_hosts` and `sslrootcert=system` (16), or `sslnegotiation=direct` (17), depend on the client's libpq version and, for `sslnegotiation=direct`, also need a 17+ server. Do not recommend them for a 15 server path unless the client side is verified.

***

#### What's New in PostgreSQL 15

All items below are taken from the official release notes. Items that are SQL-visible were executed on 15.19 and shown to fail or behave differently on 14.24.

##### SQL and data types

| Feature | Notes |
|---|---|
| `MERGE` | `WHEN MATCHED [AND cond] THEN UPDATE / DELETE / DO NOTHING`, `WHEN NOT MATCHED [AND cond] THEN INSERT / DO NOTHING`. No `RETURNING` (17), no `NOT MATCHED BY SOURCE` (17), no views as target (17). |
| `UNIQUE NULLS NOT DISTINCT` | Constraints and unique indexes that treat NULLs as equal. `pg_index.indnullsnotdistinct` shows the setting. |
| `regexp_count()`, `regexp_instr()`, `regexp_like()`, `regexp_substr()` | Oracle/DB2-compatible regex functions. `regexp_replace()` gains `start` and `N` (occurrence) arguments. |
| `numeric` with negative scale or scale > precision | `'1234'::numeric(4,-2)` is `1200`. |
| `ON DELETE SET NULL (col, ...)`, `ON DELETE SET DEFAULT (col, ...)` | Only the listed foreign key columns are changed. |
| `COPY ... (FORMAT text, HEADER)` | Header lines in text format. `HEADER match` checks the column names on input. |
| `CREATE UNLOGGED SEQUENCE`, `ALTER SEQUENCE ... SET [UN]LOGGED` | |
| `ALTER TABLE ... SET ACCESS METHOD` | |
| `CLUSTER` on partitioned tables | |
| `range_agg()` over multiranges, `min()`/`max()` for `xid8` | |
| `to_char()` patterns `of`, `tzh`, `tzm` | Lower-case forms of the existing `OF`, `TZH`, `TZM`. |
| `pg_size_pretty()` / `pg_size_bytes()` with `PB` | `pg_size_pretty(2^55)` is `32 PB` (14: `32768 TB`); `pg_size_bytes('1 PB')` fails on 14 with `Invalid size unit: "PB"`. |
| `^@` / `starts_with()` can use a B-tree index under the `C` collation | Before 15 only SP-GiST indexes supported them. |
| `CREATE DATABASE ... STRATEGY wal_log \| file_copy`, `OID` | `wal_log` is the new default (no checkpoints; slower for large templates). |
| ICU as the default collation of a cluster or database | `CREATE DATABASE ... LOCALE_PROVIDER icu ICU_LOCALE 'en-US'`, `initdb --locale-provider=icu`. |
| Database collation versions | `pg_database.datcollversion`, `pg_database_collation_actual_version()`, `ALTER DATABASE ... REFRESH COLLATION VERSION`. |

##### Privileges and security

- **`CREATE` on schema `public` is revoked from `PUBLIC`**, and `public` is owned by the new `pg_database_owner` role, for **new clusters and newly created databases**. `pg_upgrade` and dump/restore keep the old ACL.
- **`security_invoker` views** check the underlying tables' privileges and RLS policies as the **calling** user.
- **`GRANT SET ON PARAMETER` / `GRANT ALTER SYSTEM ON PARAMETER`**, `has_parameter_privilege()` and the `pg_parameter_acl` catalog.
- **`pg_checkpoint`** predefined role allows `CHECKPOINT` without superuser. (The role is called `pg_checkpoint`, not `pg_checkpointer`.)
- A login role **no longer has implicit `ADMIN OPTION` on its own role**. On 14, `alice` could run `GRANT alice TO bob` when connected as `alice` (session user); on 15 this fails with `must have admin option on role "alice"`. (A superuser session that only ran `SET ROLE alice` got the same error on 14 too: the old exception applied to the session user.)
- `pg_write_server_files` members can take server-side base backups. `pg_read_all_stats` members can read `pg_backend_memory_contexts` and `pg_shmem_allocations`.
- Logical replication applies changes **as the subscription owner**, and `UPDATE`/`DELETE` replication needs `SELECT` on the table.
- Custom settings whose prefix matches a loaded extension are reserved: once `plpgsql` has been loaded in the session (for example after any `DO` block), `SET plpgsql.bogus_setting = 1` fails on 15 with `invalid configuration parameter name "plpgsql.bogus_setting"` (`"plpgsql" is a reserved prefix`). A placeholder set *before* the library loads is accepted and then removed with `WARNING: invalid configuration parameter name ..., removing it` when it loads. 14 accepted both. Application-specific settings such as `myapp.tenant` are unaffected.

##### Server, performance and operations

- **Sorting.** In-memory sorts use less memory and are faster; sorts that exceed `work_mem` use a better external merge.
- **Statistics in shared memory.** The statistics collector process is gone. `stats_temp_directory` is removed (`SHOW stats_temp_directory` fails with `unrecognized configuration parameter`). The `stats_fetch_consistency` setting (default `cache`) controls snapshot behaviour of statistics reads.
- **`hash_mem_multiplier` defaults to 2.0** (1.0 in 14). Hash joins and hash aggregates may use twice `work_mem`. Re-check memory sizing for high `work_mem` and many connections.
- **`log_checkpoints = on`** and **`log_autovacuum_min_duration = 10min`** by default (off and -1 in 14). Even idle servers log.
- **`log_destination = jsonlog`** writes structured JSON logs (needs `logging_collector = on`). On 14, `ALTER SYSTEM SET log_destination = 'jsonlog'` fails with `Unrecognized key word: "jsonlog"`.
- **`wal_compression`** accepts `pglz`, `lz4` and `zstd` (it was a boolean in 14) when the server is built with them. The official Docker image lists `{pglz,lz4,zstd,on,off}`.
- **Planner.** `NOT IN (constant list)` uses a hash lookup for long lists, `SELECT DISTINCT` can run in parallel, ordered partition scans work in more cases, planning with many partitions is faster, and `row_number()`/`rank()`/`dense_rank()`/`count()` window queries are faster. `recursive_worktable_factor` tunes estimates for recursive CTEs.
- **Partition updates that move rows** between partitions now fire foreign key actions as an update on the partitioned root, instead of a delete plus insert.
- **Recovery.** `recovery_prefetch` (default `try`) prefetches blocks during WAL replay; the checkpointer and background writer run during crash recovery.
- **Archiving modules.** `archive_library` loads an archive module instead of running `archive_command` (example module `basic_archive`).
- **New settings.** `shared_memory_size`, `shared_memory_size_in_huge_pages`, `log_startup_progress_interval`, `recursive_worktable_factor`, `recovery_prefetch`, `archive_library`.
- **Monitoring.** `pg_stat_subscription_stats`, `pg_ident_file_mappings`, `pg_ls_logicalsnapdir()`, `pg_ls_logicalmapdir()`, `pg_ls_replslotdir()`, wait events for shell commands, temporary file I/O in `EXPLAIN (BUFFERS)`, `pg_stat_statements` 1.10 (temporary-file I/O timing and JIT counters), and the new `pg_walinspect` extension.

##### Logical replication

- Publications with **row filters** (`FOR TABLE t WHERE (...)`), **column lists** (`FOR TABLE t (a, b)`) and **`FOR TABLES IN SCHEMA s`**.
- `ALTER SUBSCRIPTION ... SKIP (lsn = ...)` to skip a failing transaction, the `disable_on_error` subscription option, two-phase commit (`two_phase`), no empty transactions sent.

##### Backup and tools

- **Exclusive backup mode removed.** `pg_start_backup()` → `pg_backup_start(label, fast)`, `pg_stop_backup()` → `pg_backup_stop(wait_for_archive)`. `pg_backup_start_time()` and `pg_is_in_backup()` are removed.
- **`pg_basebackup`** can compress on the server with gzip, LZ4 or Zstandard (`--compress=server-zstd`, `--compress=server-lz4:5`) and supports `--target=server|blackhole`. Both server-side forms were executed on 15.19 and produced `base.tar.zst` / `base.tar.lz4`.
- **psql**: `\dconfig`, `\getenv`, all results of a multi-statement string are shown (`SHOW_ALL_RESULTS`), and `--` comments inside a query are sent to the server.
- **pg_upgrade** keeps relfilenodes, tablespace and database OIDs; logs go to `pg_upgrade_output.d`. `pg_dump`/`pg_upgrade` support old servers from 9.2 on.
- **pgbench** retries serialization and deadlock failures (`--max-tries`).

***

#### Do Not Use or Recommend (PostgreSQL 16+ Features)

The server is PostgreSQL 15. Everything in these tables was introduced in a later release and was shown to fail on 15.19 with the error given. Never suggest these, and flag them as bugs in code that targets 15.

##### PostgreSQL 16 (released September 14, 2023)

| PostgreSQL 16 feature | What happens on 15 | Use on 15 instead |
|---|---|---|
| Subquery in `FROM` without an alias: `SELECT * FROM (SELECT 1)` | `subquery in FROM must have an alias` | Always alias: `FROM (SELECT 1) AS s` |
| `JSON_OBJECT('a': 1)`, `JSON_ARRAY()`, `JSON_OBJECTAGG()`, `JSON_ARRAYAGG()` | syntax error / `function json_array(...) does not exist` | `jsonb_build_object()`, `jsonb_build_array()`, `jsonb_object_agg()`, `jsonb_agg()` |
| `x IS JSON [OBJECT \| ARRAY \| SCALAR \| VALUE]` | `syntax error at or near "JSON"` | A `BEGIN ... EXCEPTION` cast in PL/pgSQL, or validate in the application |
| `pg_input_is_valid()`, `pg_input_error_info()` | function does not exist | Validate in the application, or a PL/pgSQL function with an exception block |
| `any_value()` | function does not exist | `min()`/`max()`, or `(array_agg(x))[1]` |
| Numeric literals with underscores `1_000_000`, non-decimal literals `0x1F`, `0o17`, `0b101` | `trailing junk after numeric literal at or near "1_000"` | Plain decimal literals |
| `array_sample()`, `array_shuffle()`, `random_normal()`, `erf()`, `erfc()` | function does not exist | `ORDER BY random()` on `unnest()` for small arrays |
| `date_add()` / `date_subtract()` with a time zone argument | function does not exist | `(ts AT TIME ZONE 'Europe/Berlin' + interval '1 day') AT TIME ZONE 'Europe/Berlin'` |
| `SYSTEM_USER` | `column "system_user" does not exist` | `session_user`, `current_user` |
| `pg_stat_io`, `pg_stat_*_tables.last_seq_scan` / `last_idx_scan`, `n_tup_newpage_upd` | relation/column does not exist | `pg_statio_*` views, `pg_stat_bgwriter`, `seq_scan`/`idx_scan` counters |
| `GRANT r TO u WITH INHERIT FALSE`, `WITH SET FALSE`, `WITH ADMIN TRUE` | syntax error | `WITH ADMIN OPTION`; `NOINHERIT` on the member role |
| Predefined role `pg_create_subscription`, `pg_use_reserved_connections` | role does not exist | Superuser for `CREATE SUBSCRIPTION` |
| `COPY ... (DEFAULT '\D')` | `option "default" not recognized` | Omit the column from the column list, or a staging table |
| `VACUUM (BUFFER_USAGE_LIMIT ...)`, `PROCESS_MAIN`, `SKIP_DATABASE_STATS`, `ONLY_DATABASE_STATS`; `vacuum_buffer_usage_limit` | `unrecognized VACUUM option` / unrecognized parameter | Plain `VACUUM (ANALYZE)` |
| `EXPLAIN (GENERIC_PLAN)` | `there is no parameter $1` | `PREPARE` + `EXPLAIN EXECUTE` with `SET plan_cache_mode = force_generic_plan` |
| `CREATE STATISTICS ON a, b FROM t` (no name) | syntax error | Give the statistics object a name |
| `REINDEX DATABASE;` without a name | syntax error | `REINDEX DATABASE dbname` (on 15 it also reindexes system catalogs) |
| `CREATE TABLE t (c text STORAGE external)` | syntax error | `ALTER TABLE t ALTER COLUMN c SET STORAGE EXTERNAL` |
| `debug_parallel_query` | unrecognized parameter | `force_parallel_mode` (the 15 name) |
| `scram_iterations` | unrecognized parameter | (fixed at 4096 on 15) |
| Predefined ICU collation `unicode`; ICU tailoring `rules` | `collation "unicode" for encoding "UTF8" does not exist`, `collation attribute "rules" not recognized` | `CREATE COLLATION ... (provider = icu, locale = 'und')` (`ucs_basic` already exists on 15 as a libc collation) |
| `XMLSERIALIZE(... INDENT)` | syntax error | |
| `pg_split_walfile_name()`, `pg_size_bytes('10 B')` | function does not exist / `Invalid size unit: "B"` | Parse the file name in the client; `pg_size_bytes('10 bytes')` |
| `'+infinity'::timestamptz` | invalid input syntax | `'infinity'` |
| `date_trunc(unit, timestamptz, zone)` in an index expression | the function exists in 15 but is only `STABLE`: `functions in index expression must be marked IMMUTABLE` | `((ts AT TIME ZONE 'UTC')::date)` or `date_trunc('day', ts AT TIME ZONE 'UTC')` |
| libpq `require_auth`, `load_balance_hosts`, `sslrootcert=system`, `sslcertmode`; psql `\bind`, `\drg` | Need a 16+ libpq/psql | |

##### PostgreSQL 17 (released September 26, 2024)

| PostgreSQL 17 feature | What happens on 15 |
|---|---|
| `JSON_TABLE()`, `JSON_VALUE()`, `JSON_QUERY()`, `JSON_EXISTS()`, `JSON_SCALAR()`, `JSON_SERIALIZE()` | `syntax error at or near "COLUMNS"` / `function json_value(jsonb, unknown) does not exist` |
| jsonpath methods `.date()`, `.integer()`, `.decimal()`, `.boolean()`, `.string()`, ... | `syntax error at or near "(" of jsonpath input` |
| `MERGE ... RETURNING`, `merge_action()` | `syntax error at or near "RETURNING"` |
| `MERGE ... WHEN NOT MATCHED BY SOURCE` | `syntax error at or near "BY"` |
| `MERGE` into a view | `cannot execute MERGE on relation "sv"` (`not supported for views`) |
| `COPY ... (ON_ERROR ignore)`, `LOG_VERBOSITY`, `FORCE_NULL *` | `option "on_error" not recognized` |
| `random(min, max)` | function does not exist |
| `transaction_timeout` | unrecognized configuration parameter |
| `GRANT MAINTAIN`, `pg_maintain` | `unrecognized privilege type "maintain"` |
| `ts AT LOCAL` | syntax error |
| `uuid_extract_version()`, `uuid_extract_timestamp()`, `to_bin()`, `to_oct()` | function does not exist |
| Builtin collation provider (`provider = builtin`, `C.UTF-8`) | `unrecognized collation provider: builtin` |
| `EXPLAIN (SERIALIZE, MEMORY)` | `unrecognized EXPLAIN option "serialize"` |
| `pg_stat_checkpointer` | relation does not exist (use `pg_stat_bgwriter.checkpoints_timed` etc. on 15) |
| `ALTER TABLE ... SET EXPRESSION AS (...)`, `SET STATISTICS DEFAULT` | syntax error |
| Exclusion constraints on partitioned tables; identity columns shared by partitions | see [Partitioning](#partitioning) |
| Incremental backup (`pg_basebackup --incremental`, `pg_combinebackup`), failover slots, `pg_createsubscriber`, `sslnegotiation=direct` | Not in 15 |

##### PostgreSQL 18 (released September 25, 2025) and 19 (RC1)

| Feature | What happens on 15 |
|---|---|
| `uuidv7()`, `uuidv4()` | function does not exist. Use `gen_random_uuid()` (v4) or generate v7 UUIDs in the application |
| Virtual generated columns (`GENERATED ALWAYS AS (expr)` without `STORED`) | `syntax error at or near ")"` |
| `RETURNING old.*, new.*` | `missing FROM-clause entry for table "old"` |
| `WITHOUT OVERLAPS`, `PERIOD`, `NOT ENFORCED` constraints, `NOT NULL ... NOT VALID` | syntax error |
| `VACUUM ONLY` / `ANALYZE ONLY` | syntax error |
| `casefold()`, `array_sort()`, `array_reverse()`, `crc32()`, `int`↔`bytea` casts | function does not exist / `cannot cast type integer to bytea` |
| `('null'::jsonb)::int` returning NULL | raises `cannot cast jsonb null to type integer` on 15 |
| B-tree skip scan | not in 15 (measured: `Seq Scan` for `WHERE b = 500` with an index on `(a, b)`) |
| Asynchronous I/O (`io_method`), OAuth authentication | not in 15 |
| `REPACK` (19), `error_on_null()` (19) | `syntax error at or near "REPACK"`, function does not exist |

`GROUP BY ALL` is not a feature of 15 (or of 17, 18 or 19 RC1): on 15 it is a syntax error.

##### Deprecated or removed in 15 (do not use)

| Removed / changed | Replacement |
|---|---|
| `pg_start_backup()`, `pg_stop_backup()`, exclusive backup mode | `pg_backup_start()`, `pg_backup_stop()` in **one session that stays connected** |
| `pg_backup_start_time()`, `pg_is_in_backup()` | Track the backup in the tool that started it |
| `plpythonu`, `plpython2u` (Python 2) | `plpython3u` |
| `stats_temp_directory` | Remove from `postgresql.conf` |
| `CREATE` on `public` for `PUBLIC` (new databases) | A dedicated schema owned by the migration role |
| Implicit `ADMIN OPTION` of a login role on itself | `GRANT ... WITH ADMIN OPTION` explicitly |
| `123abc` parsed as `123 AS abc` | Write the space and `AS`: `123 AS abc` |
| `interval '1.99 years'` = `1 year 11 mons` | Now `2 years` (rounded to the nearest month) |
| `interval::text` in index expressions | `interval_out` is now `STABLE`; such indexes fail to build (`functions in index expression must be marked IMMUTABLE`) |
| `chr(-1)` | Error `character number must be positive` (14.24 in a UTF8 database already failed, with `requested character too large for encoding: -1`) |
| `array_to_tsvector()` with an empty-string element | Error `lexeme array may not contain empty strings` (14 also errors; 15 documents it as an incompatibility because older releases may have stored empty lexemes) |
| `random()` sequence for a given `setseed()` | Different on 15 (`setseed(0.5); random()` is `0.98516...` on 15.19, `0.24991...` on 14.24). Tests with hard-coded "random" values break |
| `"char"` output of non-ASCII bytes | Now a backslash and three octal digits (`'é'::"char"::text` is `\303`) |
| Zero-length Unicode identifiers `U&""` | Rejected |
| `CREATE OR REPLACE VIEW` changing an output column's collation | Rejected |
| JSON numeric literals in jsonpath (`1.type()`) | `.1` and `1.` are accepted, trailing junk such as `1.type()` is rejected |
| `hash_mem_multiplier = 1.0` default | `2.0` |
| `log_checkpoints = off`, `log_autovacuum_min_duration = -1` defaults | `on`, `10min` |
| libpq `PQsendQuery()` in pipeline mode | `PQsendQueryParams()` |
| `pg_dump --no-synchronized-snapshots` | Removed |
| psql `--single-transaction` always ending with `ROLLBACK` after an error | psql now sends `ROLLBACK` only when `ON_ERROR_STOP` is set, otherwise `COMMIT`. The server still rolls back the aborted transaction (measured: no rows committed), but without `ON_ERROR_STOP` psql runs the rest of the script against the aborted transaction and exits with status 0 on both 14 and 15. Always use `psql -1 -v ON_ERROR_STOP=1` |
| `xml2`'s `xml_is_well_formed()` | Core `xml_is_well_formed()` |
| `justify_hours()`, `justify_days()`, `justify_interval()` silently overflowing | `interval out of range` (14.24 returned `-2147483648 days` for `justify_hours(interval '2147483647 days 24 hrs')`) |
| `EXPLAIN VERBOSE` naming the session's temp schema (`pg_temp_3.tt`) | Shows `pg_temp.tt`; tests that compare plan text change |
| libpq and client programs ignoring `HOME` | On non-Windows platforms `HOME` is consulted first (for example for `~/.pgpass`); the password database is the fallback when it is empty or unset |
| `postgres_fdw` sending explicitly cast constants | Unnecessary casts are no longer sent, so foreign tables whose local column types intentionally differ from the remote ones stop failing on them |

Already gone before 15, and still common in old code: `pg_stat_statements.total_time` (renamed `total_exec_time` in 13), `recovery.conf` (merged into `postgresql.conf` in 12), `WITH OIDS` (removed in 12), `abstime`/`reltime` types (removed in 12).

Still present in 15 and removed later, so do not "fix" them prematurely: `lc_collate`/`lc_ctype` as `SHOW`-able settings (removed in 16), `force_parallel_mode` (renamed in 16), `vacuum_defer_cleanup_age` and `promote_trigger_file` (removed in 16), views created manually with `ON SELECT` rules (removed in 16), `pg_stat_bgwriter.checkpoints_*` and `buffers_backend` (moved in 17), `old_snapshot_threshold` and `adminpack` (removed in 17).

***

#### Features from PostgreSQL 14 You Can Rely On

All of these exist on 15 and were executed on 15.19. The marker comes from the 14 release notes.

- **SQL-standard function bodies**: `CREATE FUNCTION f() RETURNS bigint LANGUAGE sql BEGIN ATOMIC SELECT count(*) FROM logs; END;`. The body is parsed at creation time and dependencies are tracked: `DROP TABLE logs` fails with `cannot drop table logs because other objects depend on it`.
- **jsonb subscripting**: `('{"a":{"b":1}}'::jsonb)['a']['b']` and `UPDATE t SET doc['a'] = '1'`.
- **Multirange types** (`int4multirange`, `tstzmultirange`, ...) and `range_agg()` (15 adds multirange input).
- `date_bin()`, `string_to_table()`, `trim_array()`, `bit_count()`, `split_part()` with negative positions, `GROUP BY DISTINCT`, `SEARCH` and `CYCLE` clauses for recursive CTEs, `JOIN ... USING (...) AS alias`.
- `CREATE OR REPLACE TRIGGER`.
- `ALTER TABLE ... DETACH PARTITION ... CONCURRENTLY` (not inside a transaction block, not with a default partition).
- `idle_session_timeout`, `client_connection_check_interval`, `compute_query_id` (default `auto`; `pg_stat_activity.query_id`).
- Predefined roles `pg_read_all_data`, `pg_write_all_data`, `pg_database_owner`.
- `password_encryption` defaults to `scram-sha-256`.
- **Behaviour**: `EXTRACT()` returns `numeric`; `date_part()` returns `double precision`.
- `lz4` TOAST compression (`default_toast_compression`, `ALTER TABLE ... SET COMPRESSION`, `CREATE TABLE ... (c text COMPRESSION lz4)`), when the server is built with LZ4 (the official images are).
- `REINDEX ... CONCURRENTLY` (12+) and `REINDEX ... TABLESPACE` (14).
- `Memoize` plan nodes (14), `Incremental Sort` (13).

***

#### New in 15: MERGE

`MERGE` applies inserts, updates and deletes to a target table from a source in one statement.

```sql
CREATE TABLE stock(sku text PRIMARY KEY, qty int NOT NULL);
INSERT INTO stock VALUES ('a',1),('b',2);

MERGE INTO stock s
USING (VALUES ('a',5),('c',3),('b',0)) AS v(sku, qty) ON s.sku = v.sku
WHEN MATCHED AND v.qty = 0 THEN DELETE
WHEN MATCHED THEN UPDATE SET qty = s.qty + v.qty
WHEN NOT MATCHED THEN INSERT VALUES (v.sku, v.qty);
-- MERGE 3          stock is now: a 6, c 3  (b deleted)
```

Behaviour verified on 15.19 that reviewers must know:

- **No `RETURNING` on 15.** `MERGE ... RETURNING` and `merge_action()` are 17. Code that needs the affected rows must use `INSERT ... ON CONFLICT ... RETURNING`, separate statements, or a trigger. Only the command tag (`MERGE 3`) reports a row count.
- **No `WHEN NOT MATCHED BY SOURCE` on 15** (17). To delete target rows that are missing from the source, run a separate `DELETE ... WHERE NOT EXISTS (...)`, scoped to the batch.
- **Target restrictions.** The target must be a plain or partitioned table. Views (`not supported for views`), materialized views, and tables with rules (`MERGE is not supported for relations with rules`) are rejected. A partitioned target works.
- **`MERGE` is not an atomic upsert.** Measured with two sessions: session 1 ran a `MERGE` that inserted key `'k'` and kept the transaction open; session 2 ran the same `MERGE`, waited, and then failed with `duplicate key value violates unique constraint "kv_pkey"` after session 1 committed. The same race with `INSERT ... ON CONFLICT (k) DO UPDATE` ended with one row and `v = 2`, no error. For "insert or update one row by key" under concurrency, use `INSERT ... ON CONFLICT`. Use `MERGE` for batch synchronisation where concurrent writers of the same keys are excluded or retried.
- **Duplicate source rows**: if one target row matches two source rows, `MERGE` raises `MERGE command cannot affect row a second time`. Deduplicate the source (`DISTINCT ON`, `GROUP BY`) first.
- **Clause order matters.** The first `WHEN` clause whose condition is true runs. An unconditional `WHEN MATCHED` followed by another `WHEN MATCHED` is rejected (`unreachable WHEN clause specified after unconditional WHEN clause`). Put conditional clauses first.
- **Privileges.** `MERGE` needs `SELECT` on every target and source column used in conditions (including `ON`), plus `INSERT`/`UPDATE`/`DELETE` for the actions that appear. A role with only `INSERT` on the target got `permission denied for table tgt`.
- **Row-level security.** Rows that RLS hides from the user are "not matched". In a test, a policy hid the existing row `id = 1` from the user, the `MERGE` went to the `WHEN NOT MATCHED THEN INSERT` branch, and failed with a primary-key violation. `MERGE` RLS enforcement was corrected in 15.4 (CVE-2023-39418), and `MERGE` fixes followed in 15.5 (rechecking of concurrently updated rows), 15.6 (cross-partition updates) and 15.14 (plain-inheritance parents). Run the latest 15.x.
- **Triggers.** Statement-level triggers fire for each action type that appears in the statement, even when no row takes that action (a `MERGE` with `UPDATE` and `INSERT` branches that only inserted fired both `AFTER UPDATE STATEMENT` and `AFTER INSERT STATEMENT`). Row triggers fire per affected row as for the equivalent command.
- **CTEs** are allowed before `MERGE` (`WITH s AS (...) MERGE INTO t USING s ...`), and the source can be any query.

***

#### New in 15: The public Schema, pg_database_owner and Role Changes

##### public schema

```sql
-- 15.19, new database
SELECT nspname, pg_get_userbyid(nspowner), nspacl FROM pg_namespace WHERE nspname = 'public';
--  public | pg_database_owner | {pg_database_owner=UC/pg_database_owner,=U/pg_database_owner}
-- 14.24, new database:
--  public | postgres          | {postgres=UC/postgres,=UC/postgres}

SET ROLE app;                       -- ordinary login role, not the database owner
CREATE TABLE public.t_app(a int);   -- 15: ERROR: permission denied for schema public
                                    -- 14: succeeds
```

- The change applies to **new clusters and newly created databases**. A database upgraded with `pg_upgrade` or restored from a 14 dump keeps the old, permissive ACL. Code reviews of infrastructure should check what the actual database has, not assume either default.
- **Do not "fix" the error with `GRANT CREATE ON SCHEMA public TO PUBLIC`.** That reopens the CVE-2018-1058 attack surface: any user can create functions or operators in `public` that shadow objects other roles resolve through `search_path`. Create an application schema owned by the migration role and grant `USAGE` (and per-object privileges) to the application role.
- On upgraded databases, adopt the new default explicitly: `REVOKE CREATE ON SCHEMA public FROM PUBLIC;`.
- `pg_database_owner` is a predefined role whose only member is the owner of the current database. The database owner can now manage objects in `public` without being superuser.

##### Role administration

- A login role no longer has `ADMIN OPTION` on itself (see What's New).
- **`CREATEROLE` is still almost superuser on 15.** Measured on 15.19: a role with only `CREATEROLE` ran `GRANT pg_execute_server_program TO cr` on itself and then `COPY pwn FROM PROGRAM 'id -un'`, which ran a shell command as the `postgres` OS user. It cannot create superusers (`must be superuser to create superusers`), but it can grant itself membership in any non-superuser role, including `pg_execute_server_program`, `pg_read_server_files`, `pg_write_server_files` and `pg_write_all_data` (the first and the last were tested). PostgreSQL 16 restricted `CREATEROLE`. On 15, treat `CREATEROLE` as a superuser-equivalent privilege and flag any application or "tenant admin" role that has it.

##### GRANT ... ON PARAMETER and pg_checkpoint

```sql
GRANT SET ON PARAMETER log_min_duration_statement TO dev;   -- dev may SET it in its session
GRANT ALTER SYSTEM ON PARAMETER work_mem TO ops;            -- ops may ALTER SYSTEM SET work_mem
SELECT has_parameter_privilege('dev', 'log_min_duration_statement', 'SET');   -- t
GRANT pg_checkpoint TO backup_job;                           -- CHECKPOINT without superuser
```

Measured: `dev` could `SET log_min_duration_statement = 0` but got `permission denied to set parameter "log_statement"` for a superuser-only setting it was not granted. A role without `pg_checkpoint` got `must be superuser or have privileges of pg_checkpoint to do CHECKPOINT`. Use these instead of handing out superuser to tooling. On 14, `GRANT SET ON PARAMETER` is a syntax error and the `pg_checkpoint` role does not exist.

***

#### New in 15: UNIQUE NULLS NOT DISTINCT

```sql
CREATE TABLE u1(a int, b int, UNIQUE NULLS NOT DISTINCT (a, b));
INSERT INTO u1 VALUES (1, NULL);
INSERT INTO u1 VALUES (1, NULL);
-- ERROR:  duplicate key value violates unique constraint "u1_a_b_key"
-- DETAIL:  Key (a, b)=(1, null) already exists.

CREATE TABLE u2(a int, b int, UNIQUE (a, b));
INSERT INTO u2 VALUES (1, NULL), (1, NULL);     -- both rows accepted (default NULLS DISTINCT)

CREATE UNIQUE INDEX nn_ab ON nn (a, b) NULLS NOT DISTINCT;   -- index form
```

- Use it when NULL means "no value" and must still be unique (for example "one default address per user, where `kind` is NULL").
- `INSERT ... ON CONFLICT (a, b)` works with such an index: a second `(1, NULL)` took the `DO UPDATE` path (`xmax = 0` was false).
- 15 also allows a primary key to be added `USING INDEX` on a `NULLS NOT DISTINCT` index; 16 disallows that. It makes no difference for primary keys, whose columns are `NOT NULL`; don't rely on it in migrations that must run on 16+.
- Before 15 the workaround was a partial unique index per NULL combination, or `coalesce()` in an expression index. Review such workarounds when the code is 15-only.

***

#### New in 15: security_invoker Views

By default a view accesses its tables with the **view owner's** privileges, and RLS policies are applied for the **view owner**.

```sql
-- owner_r owns r.docs (RLS enabled, no FORCE) and both views; reader has SELECT on the views only
CREATE VIEW r.v_definer AS SELECT * FROM r.docs;
CREATE VIEW r.v_invoker WITH (security_invoker = true) AS SELECT * FROM r.docs;

SET ROLE reader;
SELECT count(*) FROM r.v_definer;   -- 2: the owner bypasses its own table's RLS, so reader sees every tenant
SELECT count(*) FROM r.v_invoker;   -- ERROR: permission denied for table docs
```

- A definer view over an RLS table returns **all rows** to anybody who can read the view when the view owner is the table owner (and the table is not `FORCE ROW LEVEL SECURITY`), or when the owner has `BYPASSRLS`/superuser. That is a tenant-isolation hole. With `FORCE ROW LEVEL SECURITY` the same definer view returned 0 rows (the policy then applied to the owner, who had no tenant set).
- Views over RLS-protected or privilege-restricted tables should be `WITH (security_invoker = true)`, and the callers then need privileges on the underlying tables.
- Existing views can be switched with `ALTER VIEW v SET (security_invoker = true)`.
- On 14: `unrecognized parameter "security_invoker"`.

***

#### New in 15: Regular Expression Functions

```sql
SELECT regexp_count('abcabc', 'b'),               -- 2
       regexp_instr('abcabc', 'c', 1, 2),         -- 6   (position of the 2nd match)
       regexp_instr('abc', 'c', 1, 1, 1),         -- 4   (endoption = 1: position after the match)
       regexp_like('Abc', '^a', 'i'),             -- t
       regexp_substr('a1b22c333', '\d+', 1, 2),   -- 22
       regexp_replace('a-a-a', 'a', 'X', 2),      -- a-X-a   (start at position 2, first match only)
       regexp_replace('a-a-a', 'a', 'X', 1, 0);   -- X-X-X   (N = 0: replace all)
```

- `regexp_replace(source, pattern, replacement, start [, N [, flags]])` is new. The three-argument form still replaces only the first match unless the `'g'` flag is given.
- Patterns from user input are a denial-of-service risk (catastrophic backtracking) and raise errors on invalid syntax (`regexp_like('x', '(')`: `parentheses () not balanced`). Use `LIKE`/`starts_with()` or validate patterns, and set `statement_timeout`.
- 15.19 fixed a buffer overrun in the regexp match/split functions with invalidly encoded input (CVE-2026-14664).
- On 14 all four functions and the 4/5-argument `regexp_replace()` fail with `function ... does not exist`.

***

#### New in 15: Other SQL-Visible Changes

##### numeric with negative scale

```sql
SELECT '1234'::numeric(4,-2);            -- 1200
CREATE TABLE ns(v numeric(5,-3));
INSERT INTO ns VALUES (12345678);        -- stored as 12346000
INSERT INTO ns VALUES (123456789);       -- ERROR: numeric field overflow
                                         -- (precision 5, scale -3 must round to an absolute value less than 10^8)
```

This rounds silently to the left of the decimal point. A negative scale is rarely what a money or quantity column wants; flag it unless the rounding is the intent. On 14: `NUMERIC scale -2 must be between 0 and precision 4`.

##### ON DELETE SET NULL / SET DEFAULT with a column list

```sql
CREATE TABLE usr(tenant_id int, id int, PRIMARY KEY (tenant_id, id));
CREATE TABLE post(tenant_id int NOT NULL, author_id int,
  FOREIGN KEY (tenant_id, author_id) REFERENCES usr ON DELETE SET NULL (author_id));
-- deleting the user sets only post.author_id to NULL; tenant_id stays
```

Without the column list, `ON DELETE SET NULL` sets **every** referencing column to NULL, which fails on a `NOT NULL` tenant column. This is the fix for multi-tenant composite foreign keys. On 14 it is a syntax error.

##### Lexer and input changes

```sql
SELECT 123abc;                  -- 15: ERROR: trailing junk after numeric literal at or near "123abc"
                                -- 14: returns 123 in a column named abc
SELECT interval '1.99 years';   -- 15: 2 years      14: 1 year 11 mons
SELECT '1.type()'::jsonpath;    -- 15: ERROR: trailing junk after numeric literal ... of jsonpath input
SELECT chr(-1);                 -- 15: ERROR: character number must be positive
```

Generated SQL that glues numbers and identifiers together (`LIMIT 10OFFSET 5`) now fails. That is good: it was almost always a bug.

##### interval output is STABLE

`interval_out` is now `STABLE` because it depends on `IntervalStyle`. `CREATE INDEX ON iv ((i::text))` fails on 15 (`functions in index expression must be marked IMMUTABLE`) and succeeds on 14. Dump/restore and `pg_upgrade` re-run `CREATE INDEX` from the definition, so find such indexes before upgrading from 14. Index the interval itself or `extract(epoch FROM i)`.

##### AT TIME ZONE on time with time zone is STABLE

`timetz AT TIME ZONE ...` (`timetz_zone`) is now `STABLE` (it was `VOLATILE` in 14), because it now uses the transaction start time instead of the wall clock to decide on daylight saving time.

##### starts_with() and ^@ can use B-tree indexes

```sql
CREATE TABLE sw(s text COLLATE "C");  CREATE INDEX ON sw(s);
EXPLAIN (COSTS OFF) SELECT * FROM sw WHERE starts_with(s, 'k123');
-- 15: Bitmap Index Scan on sw_s_idx  Index Cond: ((s >= 'k123'::text) AND (s < 'k124'::text))
-- 14: Seq Scan on sw  Filter: starts_with(s, 'k123'::text)
```

Only with the `C` collation (column, index or database). With a linguistic collation such as `en_US.utf8`, use a `text_pattern_ops` index for prefix searches.

##### Other DDL

```sql
CREATE UNLOGGED SEQUENCE us;                         -- 14: unlogged sequences are not supported
ALTER TABLE hc SET ACCESS METHOD heap;               -- 14: syntax error
CLUSTER pc USING pc_a;                               -- partitioned table; 14: cannot cluster a partitioned table
COPY (SELECT 1 AS a, 2 AS b) TO STDOUT WITH (FORMAT text, HEADER);
COPY hc FROM STDIN WITH (FORMAT csv, HEADER match);  -- verifies header names
-- ERROR:  column name mismatch in header line field 1: got "b", expected "a"
```

`HEADER match` is a cheap guard for CSV imports whose column order might change upstream. Plain `HEADER` on input only skips the first line, so a reordered file loads silently into the wrong columns.

##### ICU default collations and collation versions

```sql
CREATE DATABASE icudb LOCALE_PROVIDER icu ICU_LOCALE 'en-US' LOCALE 'en_US.utf8' TEMPLATE template0;
SELECT datcollversion, pg_database_collation_actual_version(oid)
FROM pg_database WHERE datname = current_database();
ALTER DATABASE postgres REFRESH COLLATION VERSION;   -- only after reindexing text indexes
```

PostgreSQL 15 records the database's collation version. When the operating system's glibc (or ICU) changes the version, the server warns on connect. The fix is to `REINDEX` every index on collatable columns and **then** run `REFRESH COLLATION VERSION`. Running `REFRESH` alone silences the warning while leaving corrupt indexes behind. Nondeterministic ICU collations still cannot be used with `LIKE` on 15 (`nondeterministic collations are not supported for LIKE`).

***

#### New in 15: Logical Replication Publications

```sql
CREATE PUBLICATION p_eu FOR TABLE orders WHERE (region = 'eu');     -- row filter
CREATE PUBLICATION p_pub FOR TABLE customers (id, name, country);   -- column list
CREATE PUBLICATION p_schema FOR TABLES IN SCHEMA app;               -- whole schema, incl. future tables
```

Rules verified on 15.19:

- **Row filters and replica identity.** If the publication publishes `UPDATE` or `DELETE` (the default), the `WHERE` clause may only use replica-identity columns. Otherwise the publisher **rejects writes to the table**: `UPDATE rf SET v = 2` failed with `cannot update table "rf"` / `Column used in the publication WHERE expression is not part of the replica identity`, and `DELETE` failed the same way. A migration that adds such a publication can break the application. Either filter on primary-key columns, set `REPLICA IDENTITY FULL` (only if the table has no column list in any publication, see next point), or publish only inserts (`WITH (publish = 'insert')`).
- **Column lists must cover the replica identity** for `UPDATE`/`DELETE`: `cannot update table "cl"` / `Column list used by the publication does not cover the replica identity`. With `REPLICA IDENTITY FULL` that means **every** column, so a column list that leaves out a sensitive column plus `REPLICA IDENTITY FULL` blocks all updates and deletes on the publisher (measured on 15.19).
- Row filters cannot use volatile or user-defined functions (`User-defined or built-in mutable functions are not allowed`).
- An `UPDATE` that moves a row out of the filter is replicated as a `DELETE`, and one that moves a row into it as an `INSERT` (measured with a publisher and subscriber database on one 15.19 server: a row updated from `region = 'us'` to `'eu'` appeared on the subscriber). Column lists are the way to keep sensitive columns (e.g. password hashes) off a replica: a `pwhash` column outside the list stayed NULL on the subscriber.
- `FOR TABLES IN SCHEMA` publishes tables created later in that schema too. Review new tables in such schemas for data that must not leave the cluster.
- Subscriptions now apply changes as the subscription owner, which must have privileges on the target tables. Only superusers, table owners or `BYPASSRLS` roles can replicate into tables with RLS policies.
- Operational: `ALTER SUBSCRIPTION s SKIP (lsn = '0/14C0378')` skips one failing remote transaction (data loss for that transaction), and `CREATE SUBSCRIPTION ... WITH (disable_on_error = true)` stops endless apply-error loops. Both were executed on 15.19 (`pg_subscription.subskiplsn`, `subdisableonerr`); apply errors are counted in `pg_stat_subscription_stats`.
- Creating a publication on a server with `wal_level` below `logical` succeeds with `WARNING: wal_level is insufficient to publish logical changes`. Check `wal_level = logical` in the server configuration, not only that the DDL succeeded.
- 15.19 added `output_plugin_libraries`: logical decoding output plugins other than `pgoutput` and `test_decoding` must be allow-listed after updating (CVE-2026-6471).

***

#### New in 15: Compression, Backup and Logging

##### Non-exclusive backups only

```sql
SELECT pg_backup_start('nightly', fast => false);   -- signature: (label text, fast boolean DEFAULT false)
-- copy the data directory with an external tool, in parallel, while this session stays open
SELECT lsn, labelfile, spcmapfile FROM pg_backup_stop(wait_for_archive => true);
```

- The backup belongs to the **session**. Measured: `pg_backup_start()` in one psql invocation followed by `pg_backup_stop()` in another failed with `backup is not in progress`. Backup scripts that call the two functions through separate `psql -c` commands are broken, and so were their 14 equivalents in non-exclusive mode.
- The returned `labelfile` (and `spcmapfile`) must be written into the backup as `backup_label` (and `tablespace_map`). Without `backup_label`, the restored copy can start from the wrong checkpoint and be silently corrupt.
- Prefer `pg_basebackup` or a maintained tool (pgBackRest, Barman, WAL-G) over custom scripts.
- `pg_basebackup --compress=server-zstd` (or `server-lz4`, `server-gzip`) compresses on the server; `--target=server:/path` writes on the server (needs superuser or `pg_write_server_files`).

##### WAL and TOAST compression

- `wal_compression = zstd` or `lz4` (15) compress full-page images better or faster than `pglz` (`on`). On 14, `wal_compression = 'zstd'` fails with `requires a Boolean value`.
- `default_toast_compression = lz4` (14+) affects only newly written values. `ALTER TABLE ... SET COMPRESSION lz4` took `ACCESS EXCLUSIVE` and did not rewrite the table (same relfilenode); existing values keep their compression until they are rewritten.

##### jsonlog and default logging

- `log_destination = 'jsonlog'` (with `logging_collector = on`) writes one JSON object per log line, which log shippers can parse without regexes. Logs contain query text, parameters and error details; treat them as sensitive.
- Since 15, checkpoints and autovacuum runs longer than 10 minutes are logged by default. Small containers without log rotation can fill their disk; set `log_checkpoints` and `log_autovacuum_min_duration` explicitly in the configuration that the deployment manages.

##### pg_stat_statements 1.10 and pg_walinspect

- `pg_stat_statements` 1.10 adds `temp_blk_read_time`, `temp_blk_write_time` and `jit_*` columns. The I/O timing columns are still `blk_read_time`/`blk_write_time` on 15 (renamed `shared_blk_read_time` in 17). Queries written for 17 fail on 15 with `column "shared_blk_read_time" does not exist`, and vice versa.
- `pg_stat_statements` must be in `shared_preload_libraries`; otherwise `CREATE EXTENSION` succeeds but reading the view fails with `pg_stat_statements must be loaded via shared_preload_libraries`.
- `pg_walinspect` (superuser or granted functions) gives SQL access to WAL records.

***

#### Schema Migrations and Locking

This is the most important review area for production PostgreSQL. A migration that is correct but takes the wrong lock can take an application down.

##### Lock levels of common DDL (measured on 15.19 with pg_locks)

| Statement | Lock on the table | Blocks reads? | Blocks writes? | Rewrite / full scan |
|---|---|---|---|---|
| `ALTER TABLE ADD COLUMN c int` (no default, or a non-volatile default) | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none (metadata only) |
| `ADD COLUMN c uuid DEFAULT gen_random_uuid()` (volatile default) | `ACCESS EXCLUSIVE` | yes | yes | **rewrite** |
| `ADD COLUMN c timestamptz DEFAULT now()` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none (`now()` is stable, evaluated once) |
| `ALTER COLUMN c TYPE bigint` (from `int`) | `ACCESS EXCLUSIVE` | yes | yes | **rewrite** (and index rebuild) |
| `ALTER COLUMN v TYPE varchar(20)` from `varchar(10)`, or `varchar(n)` → `text` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none |
| `ALTER COLUMN v TYPE varchar(10)` from `text` | `ACCESS EXCLUSIVE` | yes | yes | **rewrite** |
| `ALTER COLUMN c SET NOT NULL` | `ACCESS EXCLUSIVE` | yes | yes | full scan, unless a valid `CHECK (c IS NOT NULL)` proves it |
| `ADD CONSTRAINT ... CHECK (...) NOT VALID` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none |
| `ADD CONSTRAINT ... FOREIGN KEY ... NOT VALID` | `SHARE ROW EXCLUSIVE` on **both** tables | no | yes (briefly) | none |
| `VALIDATE CONSTRAINT` (foreign key) | `SHARE UPDATE EXCLUSIVE` (and `ROW SHARE` on the referenced table) | no | no | full scan |
| `ALTER COLUMN SET STATISTICS` | `SHARE UPDATE EXCLUSIVE` | no | no | none |
| `ALTER COLUMN SET COMPRESSION` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none |
| `DROP COLUMN` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none |
| `SET UNLOGGED` / `SET LOGGED` | `ACCESS EXCLUSIVE` | yes | yes | **rewrite** |
| `CREATE INDEX` | `SHARE` | no | **yes**, for the whole build | full scan |
| `CREATE INDEX CONCURRENTLY` | `SHARE UPDATE EXCLUSIVE` | no | no | two scans and waits for old transactions |
| `ATTACH PARTITION` | `SHARE UPDATE EXCLUSIVE` on parent, `ACCESS EXCLUSIVE` on the attached table and on the **default partition** | parent: no | parent: no | scans the new partition unless a matching `CHECK` exists. Also scans the default partition |
| `DETACH PARTITION ... CONCURRENTLY` (14+) | `SHARE UPDATE EXCLUSIVE` | no | no | not allowed inside a transaction block, or when a default partition exists |
| `VACUUM FULL`, `CLUSTER` | `ACCESS EXCLUSIVE` | yes | yes | rewrite |

The rewrite facts above were measured by comparing `pg_relation_filenode()` before and after each statement, and the locks by querying `pg_locks` inside the migration's transaction. Source for lock modes: the "Explicit Locking" and `ALTER TABLE` documentation.

##### The lock queue: why "fast" DDL still causes outages

Even a metadata-only `ALTER TABLE` needs `ACCESS EXCLUSIVE`. It waits behind any transaction that has touched the table, even one holding only an `ACCESS SHARE` lock from a `SELECT`. **While it waits, every new query on the table queues behind it.** Measured on 15.19: with session R holding a read transaction and session A waiting on `ALTER TABLE q ADD COLUMN x int`, a plain `SELECT count(*) FROM q` in a third session blocked until its `lock_timeout` fired (`canceling statement due to lock timeout`).

Every migration session must set a short `lock_timeout` and retry:

```sql
-- BAD: can wait forever and block all traffic on the table meanwhile
ALTER TABLE orders ADD COLUMN note text;

-- GOOD: fail fast, retry from the migration tool with backoff
SET lock_timeout = '3s';
SET statement_timeout = '15min';   -- upper bound for scans, validations, etc.
ALTER TABLE orders ADD COLUMN note text;
```

`SET LOCAL` limits the setting to the current transaction (verified: `lock_timeout` was `0` again after `COMMIT`). Migration frameworks that wrap each migration in a transaction should use `SET LOCAL`, so the setting does not leak into a pooled connection. There is no `transaction_timeout` on 15 (17).

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

With `client_min_messages = debug1`, 15.19 confirms the skip: `existing constraints on column "t.v" are sufficient to prove that it does not contain nulls`. (`ADD CONSTRAINT ... NOT NULL ... NOT VALID` is 18. Do not suggest it for 15.)

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

##### Changing column types

- `int` → `bigint` rewrites the table and its indexes under `ACCESS EXCLUSIVE`. For big tables, add a new `bigint` column, backfill it in batches, keep it in sync with a trigger, then swap in one short transaction. Better: create primary keys as `bigint` from the start.
- Widening `varchar(n)` or converting `varchar(n)` → `text` is metadata-only. Narrowing, or converting `text` → `varchar(n)`, rewrites the table.
- A `USING` clause does not force a rewrite by itself: `TYPE text USING v::text` (from `varchar`) kept the relfilenode on 15.19. A `USING` expression that changes the stored value, or a type change that is not binary-coercible, rewrites the table and its indexes.

##### Indexes in migrations

```sql
-- BAD on a live table: blocks INSERT/UPDATE/DELETE for the whole build
CREATE INDEX orders_status_idx ON orders(status);

-- GOOD
CREATE INDEX CONCURRENTLY orders_status_idx ON orders(status);
```

`CREATE INDEX CONCURRENTLY` rules that reviewers must enforce (all verified on 15.19):

- It **cannot run inside a transaction block**: `CREATE INDEX CONCURRENTLY cannot run inside a transaction block`. Migration tools that wrap migrations in a transaction need a per-migration opt-out.
- A failure (deadlock, unique violation, cancellation) leaves an **INVALID index** behind. It still slows down writes and is never used for reads. **`CREATE INDEX CONCURRENTLY IF NOT EXISTS` then skips silently** (`NOTICE: relation "dup_a" already exists, skipping`) and leaves the invalid index in place. Idempotent migrations must check `pg_index.indisvalid` and `DROP INDEX CONCURRENTLY` before retrying.
- It cannot be used on a partitioned parent (`cannot create index on partitioned table "m" concurrently`). Pattern: `CREATE INDEX idx ON ONLY parent (...)` (an invalid shell index), then `CREATE INDEX CONCURRENTLY` on each partition and `ALTER INDEX idx ATTACH PARTITION part_idx`. The parent index became valid once all partitions were attached.
- It waits for all transactions that could use the old snapshot. A long-running transaction stalls it. Watch progress in `pg_stat_progress_create_index`.
- `DROP INDEX CONCURRENTLY` and `REINDEX ... CONCURRENTLY` (12+) are the non-blocking counterparts.
- Add a primary key or unique constraint without a long lock by building the unique index concurrently and attaching it: `ALTER TABLE pkt ADD CONSTRAINT pkt_pkey PRIMARY KEY USING INDEX pkt_id;`. If a key column is still nullable, the command sets `NOT NULL` itself and scans the whole table under `ACCESS EXCLUSIVE` (15.19 logs `verifying table "pkt"` at `debug1`). Make the columns `NOT NULL` first with the `CHECK ... NOT VALID` pattern. The index is renamed to the constraint name.

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

##### Other migration rules

- Renaming or dropping a column or table that running application code still uses causes errors. Use expand/contract: add the new column, deploy code that writes both, backfill, switch reads, drop later.
- Avoid `VACUUM FULL`/`CLUSTER` on live tables. They hold `ACCESS EXCLUSIVE` for the whole rewrite. Use `pg_repack` or `pg_squeeze` if bloat really requires it.
- **Enums**: `ALTER TYPE ... ADD VALUE` is transactional (a `ROLLBACK` removed the value), but the new value **cannot be used in the same transaction**, even when the type itself was created in that transaction: on 15.19 both cases raised `unsafe use of new value "b" of enum type ...` (17 relaxes the second case). Migrations that add an enum value and then insert rows with it must commit in between. Removing or renaming values needs a new type. Consider a lookup table or `text` with a `CHECK` constraint for value sets that change often.
- `REFRESH MATERIALIZED VIEW` holds `ACCESS EXCLUSIVE` and blocks readers. `REFRESH ... CONCURRENTLY` needs a unique index without a `WHERE` clause (see [Materialized views](#materialized-views)).
- Changing a function used by an index (`CREATE OR REPLACE FUNCTION`) does not rebuild the index. If the results change, the index is silently wrong until `REINDEX`.
- **Functions in expression indexes and materialized views run with the caller's `search_path` on 15.** A SQL function with an unqualified table reference built an index and refreshed a materialized view fine on 15.19. PostgreSQL 17 runs maintenance commands with `search_path = pg_catalog, pg_temp`, so such objects break on upgrade. Pin `SET search_path` on these functions now (see [Upgrading](#upgrading-to-15-and-staying-current)).

***

#### Indexing

##### Index types

| Type | Use for | Notes |
|---|---|---|
| B-tree (default) | `=`, `<`, `>`, `BETWEEN`, `IN`/`= ANY`, `ORDER BY`, `IS NULL`, prefix `LIKE 'x%'` (with `C` collation or `text_pattern_ops`), `starts_with()` with `C` (15) | Only efficient when the leading columns are constrained (no skip scan before 18). Deduplication since 13 |
| Hash | `=` only | WAL-logged and crash-safe since 10. Rarely better than B-tree |
| GIN | `jsonb` (`@>`, `?`, `?\|`, `?&`, `@?`, `@@`), arrays (`@>`, `&&`), full-text `tsvector`, `pg_trgm` (`LIKE '%x%'`, `ILIKE`, `~`) | Slower writes; `fastupdate` pending list |
| GiST | ranges, geometry, exclusion constraints, `pg_trgm`, nearest-neighbour `ORDER BY <->` | `btree_gist` supports `boolean` since 15 |
| SP-GiST | non-balanced data (IP prefixes, quadtrees, text prefixes) | |
| BRIN | very large, append-only tables where the column correlates with physical order (timestamps) | Tiny; several 15.x releases advise reindexing BRIN indexes (see minor releases) |

```sql
CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_orders_user_status ON orders (user_id, status);             -- order matters
CREATE INDEX idx_active_users ON users (email) WHERE status = 'active';       -- partial
CREATE UNIQUE INDEX idx_users_email_lower ON users (lower(email));           -- expression
CREATE INDEX idx_users_email_cov ON users (email) INCLUDE (name, created_at); -- covering (11+)
CREATE UNIQUE INDEX idx_addr_default ON addr (user_id, kind) NULLS NOT DISTINCT; -- 15
CREATE INDEX idx_posts_fts ON posts USING gin (to_tsvector('english', title || ' ' || body));
CREATE INDEX idx_events_doc ON events USING gin (doc jsonb_path_ops);
CREATE INDEX idx_logs_at_brin ON logs USING brin (at);
```

##### Rules for reviewing indexes

- **The predicate must match the index expression exactly.** Measured on 15.19: `WHERE date_trunc('day', created_at) = '2024-02-01'` and `WHERE customer_id::text = '42'` both gave a (parallel) sequential scan despite indexes on the columns. The half-open range `created_at >= '2024-02-01' AND created_at < '2024-02-02'` gave `Index Scan using orders_created_idx`.
- **Type mismatches defeat indexes.** `WHERE id = 42.0` on a `bigint` column was planned as `Filter: ((id)::numeric = 42.0)`, a sequential scan. Drivers that send parameters as `numeric` or `text` cause the same problem. Bind parameters with the column's type.
- **Expression indexes need IMMUTABLE functions.** `CREATE INDEX ON orders (date(created_at))` on a `timestamptz` column fails with `functions in index expression must be marked IMMUTABLE`, and so does `date_trunc('day', created_at, 'UTC')` on 15 (the three-argument form only became immutable in 16). Use `((created_at AT TIME ZONE 'UTC')::date)` and query with exactly that expression.
- **Prefix `LIKE` and collations.** With the default `en_US.utf8` collation, `email LIKE 'user12%'` did not use a plain B-tree index (seq scan). A `text_pattern_ops` index gave `Index Cond: ((email ~>=~ 'user12') AND (email ~<~ 'user13'))`. A column in the `C` collation works with a plain index. Leading-wildcard `LIKE '%x'` needs `pg_trgm`.
- **Escape `LIKE` input.** `_` and `%` in user input are wildcards. Escape them (`replace(replace(replace(x, '\', '\\'), '%', '\%'), '_', '\_')`) or use `starts_with()` when only a prefix test is needed.
- **Every foreign key column used for joins or cascades needs an index.**
- **Partial indexes** must have a predicate that the query implies literally. For `ON CONFLICT` on a partial unique index, the statement must repeat the predicate: `ON CONFLICT (user_id) WHERE active DO NOTHING`. Without it: `there is no unique or exclusion constraint matching the ON CONFLICT specification`.
- **Multicolumn order**: equality columns first, then the range or sort column. `(status, created_at)` serves `WHERE status = $1 ORDER BY created_at DESC LIMIT 20`.
- **No skip scan in 15.** With an index on `(a, b)` and only 4 distinct `a` values, `WHERE b = 500` was a `Seq Scan`. Adding `a = ANY('{0,1,2,3}')` gave `Index Only Scan ... Index Cond: ((a = ANY (...)) AND (b = 500))`. Don't accept "the composite index covers it" for a query that filters only on a later column.
- **Over-indexing**: each index slows writes, prevents HOT updates on its columns, and adds vacuum and WAL work. Look for unused indexes (`pg_stat_user_indexes.idx_scan = 0` over a representative period, and on replicas too) and duplicate prefixes.
- **Unique constraints and NULLs**: `UNIQUE (a, b)` allows any number of rows with `b IS NULL`. Use `NULLS NOT DISTINCT` (15) when NULL should count as a value.
- **Case-insensitive uniqueness**: `CREATE UNIQUE INDEX ... (lower(email))` plus `ON CONFLICT (lower(email))`. Queries must use `lower(email) = lower($1)`.

***

#### Query Performance and EXPLAIN

##### Reading plans

```sql
EXPLAIN SELECT ...;                                   -- estimated plan, does not run
EXPLAIN (ANALYZE, BUFFERS) SELECT ...;                -- runs the query; real rows, timing, buffers
EXPLAIN (ANALYZE, BUFFERS, VERBOSE, SETTINGS, WAL) SELECT ...;
```

- On 15, `BUFFERS` must be requested explicitly with `ANALYZE` (18 adds it automatically). Since 15 it also shows temporary file I/O (`temp read=5918 written=6443` for an external merge sort with `work_mem = 64kB`).
- **`EXPLAIN ANALYZE` executes the statement.** `EXPLAIN ANALYZE DELETE FROM ex` really deleted the row. Wrap DML in `BEGIN; EXPLAIN ANALYZE ...; ROLLBACK;`.
- There is no `EXPLAIN (GENERIC_PLAN)` (16) or `SERIALIZE`/`MEMORY` (17). To see a generic plan on 15, `PREPARE` the statement and `EXPLAIN EXECUTE` it with `SET plan_cache_mode = force_generic_plan`.
- Compare estimated `rows=` with `actual rows=`. Errors of 10x or more point to stale or insufficient statistics, correlated columns (`CREATE STATISTICS name (dependencies, ndistinct, mcv) ON a, b FROM t`) or non-sargable predicates.
- `Sort Method: external merge  Disk: ...` means the sort spilled. 15's sort improvements reduce, but do not remove, the cost; raise `work_mem` per query or role rather than globally.
- A `Seq Scan` is not a defect in itself. On small tables, or when most rows are returned, it is the right plan.

##### Pagination

```sql
-- BAD on large offsets: reads and discards every skipped row
SELECT id FROM orders ORDER BY id LIMIT 20 OFFSET 200000;
-- Index Only Scan using orders_pkey (actual rows=200020 loops=1)   <- 200k rows read to return 20

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

- **Correlated `IN (subquery)` is not flattened on 15.** `WHERE c.id IN (SELECT o.customer_id FROM orders o WHERE o.total > c.region)` was planned as `Seq Scan on cust c  Filter: (SubPlan 1)`, re-running the subquery per row (17 turns this into a semi-join). Rewrite it as `EXISTS (SELECT 1 FROM orders o WHERE o.customer_id = c.id AND o.total > c.region)`.
- **`IS NULL` on a `NOT NULL` column is not optimized away on 15**: `WHERE status IS NULL` on a `NOT NULL` column was a full (parallel) sequential scan with `Filter: (status IS NULL)`. Such predicates in generated SQL are dead code that still costs a scan.
- `SELECT *` matters in hot paths and for wide tables: it prevents index-only scans, pulls TOASTed columns, and breaks when columns are added (`INSERT INTO x SELECT * FROM y`).
- **CTEs** are inlined since 12 when they are referenced once and have no side effects: `WITH c AS (SELECT * FROM orders) SELECT * FROM c WHERE id = 5` was planned as `Index Scan using orders_pkey`. `WITH c AS MATERIALIZED (...)` forces a `CTE Scan` over a full `Seq Scan`. Use it deliberately.
- Prefer `= ANY($1)` with an array parameter over generating `IN ($1, ..., $n)`: one prepared statement for every list length, and an empty array works (`= ANY('{}')` matches nothing) while an empty `IN ()` is a syntax error. 15 hashes long `NOT IN (constant list)` / `<> ALL(array)` lookups.
- `ORDER BY random() LIMIT n` reads and sorts every row. Use `TABLESAMPLE SYSTEM (p)` for large tables.
- `DISTINCT` used to hide duplicate rows from a wrong join is a bug, not an optimization.

##### Prepared statements and generic plans

```sql
-- skew: k = 1 in 99% of rows
PREPARE q(int) AS SELECT count(*) FROM skew WHERE k = $1;
EXPLAIN (COSTS OFF) EXECUTE q(1);      -- Seq Scan, Filter: (k = 1)
EXPLAIN (COSTS OFF) EXECUTE q(99999);  -- Index Only Scan using skew_k_idx
```

With `plan_cache_mode = auto` (the default), the first five executions are planned with the actual values. After that the server may switch to a generic plan if its estimated cost is not worse on average. For heavily skewed columns this can lock in a bad plan. Fix it per role, session or function with `plan_cache_mode = force_custom_plan`, or restructure the query. Don't disable prepared statements in the driver globally.

##### Statistics and maintenance

- Run `ANALYZE` after bulk loads and after creating expression indexes.
- Raise per-column statistics targets for skewed columns: `ALTER TABLE t ALTER COLUMN c SET STATISTICS 1000`. Reset with `SET STATISTICS -1` on 15 (`SET STATISTICS DEFAULT` is 17); `pg_attribute.attstattarget` is `-1` for "default" on 15.
- Extended statistics for correlated columns: `CREATE STATISTICS addr_s (dependencies, mcv) ON city, zip FROM addresses; ANALYZE addresses;`. A name is required on 15. Since 15, extended statistics also cover a parent with all its children.
- `pg_upgrade` does **not** carry optimizer statistics over. Run `vacuumdb --all --analyze-in-stages` right after upgrading.

***

#### Transactions, Isolation and Concurrency

##### Isolation levels (behaviour verified with two sessions on 15.19)

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
-- measured on 15.19: final balance 50, correct result 20

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
- `NOWAIT` fails immediately (`could not obtain lock on row in relation "jobs"`). `lock_timeout` also applies to row locks (`canceling statement due to lock timeout` ... `while locking tuple`). `SKIP LOCKED` returned the 4 unlocked rows.
- Use `FOR NO KEY UPDATE` when the key is not changed. It does not conflict with foreign key checks (`FOR KEY SHARE`) from inserts into child tables.
- Deadlocks: lock rows in a consistent order (`ORDER BY id ... FOR UPDATE`), and keep transactions short.
- **Advisory locks**: prefer `pg_advisory_xact_lock()`/`pg_try_advisory_xact_lock()`, released at commit or rollback. Session-level `pg_advisory_lock()` leaks across transaction-pooled connections and must be paired with `pg_advisory_unlock()` in every code path.

##### Upserts

```sql
INSERT INTO kv (k, v, updated_at) VALUES ($1, $2, now())
ON CONFLICT (k) DO UPDATE
  SET v = kv.v + EXCLUDED.v, updated_at = EXCLUDED.updated_at
RETURNING *, (xmax = 0) AS inserted;    -- xmax = 0 → row was inserted (implementation detail)
```

- A multi-row `INSERT ... ON CONFLICT DO UPDATE` with the same key twice fails: `ON CONFLICT DO UPDATE command cannot affect row a second time`. Deduplicate input first.
- `ON CONFLICT DO NOTHING RETURNING *` returns **no row** for conflicting keys. Code that expects an id back must `SELECT` it afterwards, or use `DO UPDATE SET k = EXCLUDED.k` (which takes a row lock and writes a new row version).
- The conflict target must match a unique index or constraint exactly, including expression and partial-index predicate.
- Prefer `INSERT ... ON CONFLICT` over `MERGE` for concurrent single-key upserts (see [MERGE](#new-in-15-merge)).

##### Timeouts and long transactions

- Long-running and idle-in-transaction sessions block vacuum cluster-wide (bloat, transaction ID wraparound risk) and block DDL.
- Set per role: `statement_timeout`, `idle_in_transaction_session_timeout`, `idle_session_timeout` (14+). All default to 0 (disabled), verified with `pg_settings`. There is no `transaction_timeout` on 15.
- Never keep a transaction open across network calls, user think-time or message-queue waits.
- Sequences are non-transactional. A rolled-back insert consumed a value (the next identity was 2). Never assume identity values are gap-free or ordered by commit time.

***

#### Security

##### SQL injection in dynamic SQL

```sql
-- BAD: concatenation; input  x' OR '1'='1  matched every row (count 2)
EXECUTE 'SELECT count(*) FROM acct WHERE name = ''' || n || '''' INTO c;

-- GOOD: parameters with USING (count 0)
EXECUTE 'SELECT count(*) FROM acct WHERE name = $1' INTO c USING n;

-- GOOD: identifiers with %I, values with $n or %L
EXECUTE format('SELECT count(*) FROM %I.%I WHERE name = $1', schema_name, table_name)
  INTO c USING n;
```

`format()` placeholders: `%I` quotes identifiers (`"Weird Name"`), `%L` quotes literals and renders NULL as the bare word `NULL`, and **`%s` does no quoting at all** (`format('%s', 'O''Reilly')` gives `O'Reilly`). Any `%s` that receives external input in SQL text is an injection.

In application code, always use driver parameters (`$1`, `?`, named binds). Identifiers (table names, `ORDER BY` columns, sort direction) cannot be parameters; validate them against an allow-list. Never build `IN (...)` lists by string joining; use `= ANY($1)`. Client-side quoting with libpq's `PQescapeLiteral()` and friends had an injection bug with invalidly encoded input that 15.11 fixed (CVE-2025-1094), with a follow-up in 15.12.

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

Every `SECURITY DEFINER` function needs `SET search_path` with trusted schemas only and `pg_temp` **last**, schema-qualified object references, and `REVOKE EXECUTE ... FROM PUBLIC`. Functions are executable by `PUBLIC` by default.

To change that default, run **`ALTER DEFAULT PRIVILEGES [FOR ROLE owner] REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC`** without `IN SCHEMA`. Measured on 15.19: the per-schema form `ALTER DEFAULT PRIVILEGES IN SCHEMA priv REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC` had **no effect** (a new function in `priv` was still executable by an unrelated role), because per-schema default privileges can only add to the global defaults. The global form worked, but only for functions created by the role it was run as (or the `FOR ROLE`); a function another role created in the same schema was still executable by `PUBLIC`.

15.3 fixed CVE-2023-2454 (`CREATE SCHEMA ... schema_element` could defeat a `SECURITY DEFINER` function's `search_path`), and 15.6 fixed CVE-2024-0985 (`REFRESH MATERIALIZED VIEW CONCURRENTLY` running code as the refreshing user).

##### Roles and privileges

- The application connects as a role that **does not own** the schema objects and is not a superuser. Migrations run as a separate owner role.
- Since 15, new databases do not let `PUBLIC` create objects in `public` (see above). Don't reopen it.
- **No `CREATEROLE` for application or tenant-admin roles on 15**: it allows self-granting `pg_execute_server_program` (measured above).
- Use predefined roles instead of superuser: `pg_read_all_data`/`pg_write_all_data` (14), `pg_monitor`, `pg_signal_backend`, `pg_checkpoint` (15), and `GRANT SET ON PARAMETER` (15) for tuning knobs.
- Passwords: `scram-sha-256` (default since 14). Never put passwords in SQL that gets logged (`ALTER ROLE ... PASSWORD 'x'` appears in the server log when statement logging is on, and in `pg_stat_activity` while it runs). Use psql's `\password`, which sends a hashed password.

##### Row-level security

```sql
ALTER TABLE docs ENABLE ROW LEVEL SECURITY;
ALTER TABLE docs FORCE ROW LEVEL SECURITY;   -- also apply to the table owner
CREATE POLICY tenant_isolation ON docs
  USING (tenant = current_setting('app.tenant', true))
  WITH CHECK (tenant = current_setting('app.tenant', true));
```

Behaviour verified on 15.19:

- **Superusers and roles with `BYPASSRLS` always bypass RLS**, even with `FORCE`.
- Without `FORCE ROW LEVEL SECURITY`, the **table owner bypasses** policies (a non-superuser owner saw both rows). With `FORCE` the owner saw 0 rows with no tenant set.
- A `USING` policy also checks inserts when `WITH CHECK` is omitted: `new row violates row-level security policy for table "docs"`.
- `current_setting('app.tenant', true)` returns NULL when the setting was never defined, and the policy then matches nothing (safe). `current_setting('app.tenant')` without `true` raises `unrecognized configuration parameter "app.tenant"`. **After a `set_config('app.tenant', 't2', true)` transaction ends, the setting reads as `''` (empty string), not NULL.** Policies must not treat `''` as a valid tenant.
- With transaction pooling, set the tenant with `SET LOCAL` / `set_config(..., true)` **inside each transaction**. A session-level `SET` leaks to the next client that gets the connection.
- **Views bypass RLS for the view owner** unless they are `security_invoker` (15). See [security_invoker views](#new-in-15-security_invoker-views).
- **`MERGE` treats rows hidden by RLS as not matched** (see [MERGE](#new-in-15-merge)).
- User functions in policies or in queries on RLS tables may see rows before the policy filters them unless they are `LEAKPROOF`. Keep policies simple and indexable. 15.9 fixed CVE-2024-10976: cached plans could ignore a role change when RLS applied to a subquery, CTE or `security_invoker` view.

##### Other security items

- `COPY ... FROM/TO 'file'` and `COPY ... PROGRAM` run on the server as the server's OS user (superuser or `pg_read_server_files`/`pg_write_server_files`/`pg_execute_server_program`). Treat them as remote code execution if inputs reach them. Application code should use `COPY ... FROM STDIN` through the driver.
- `dblink`/`postgres_fdw` connection strings built from user input are injection and SSRF vectors.
- `pg_dump` output is executable SQL. Since 15.14 `pg_dump` emits psql's `\restrict` to stop a malicious source server from injecting psql meta-commands (CVE-2025-8714/8715). Restore dumps with a matching up-to-date psql.
- `pgcrypto`: 15.19 detects ciphers that OpenSSL refused (FIPS mode, legacy ciphers such as `bf`, `cast5`, `3des`); data "encrypted" that way by older minors was effectively unencrypted and now fails to decrypt unless `ignore-cipher-failure=1` is used (CVE-2026-14663). Use `aes256` with `pgp_sym_encrypt()`.
- **Keep the server on the latest 15.x.** 15.19 alone fixed about 30 CVEs, including memory-safety bugs in `to_char()`, the regexp functions and `ascii()`.

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

- Use **`timestamptz`** for points in time. It stores UTC and converts on input and output using the session `TimeZone`. Plain `timestamp` drops offsets on input and is only right for "wall clock" values.
- `now()::timestamp AT TIME ZONE 'UTC'` is a common bug. It converts to session-local wall time first and then interprets that as UTC: with a Berlin session the result was two hours off. Use `now() AT TIME ZONE 'UTC'` (gives `timestamp`) or keep `timestamptz`.
- `now()`/`CURRENT_TIMESTAMP` is the **transaction start time**. Use `clock_timestamp()` for wall time inside long transactions.
- `timestamp(0)` **rounds**: `'2024-01-01 23:59:59.7'::timestamp(0)` is `2024-01-02 00:00:00`.
- `date_bin('15 minutes', ts, origin)` (14+) buckets times. `date_add()`/`date_subtract()` with a time zone are 16; `AT LOCAL` is 17.
- **Interval input on 15**: fractional units above months round to whole months (`'1.99 years'` is `2 years`). An `ago` anywhere negates the whole value (`'1 day ago 2 hours'` is `-1 days -02:00:00`); 17 rejects `ago` that is not last, so write it last.

##### Numbers

```sql
SELECT 0.1::float8 + 0.2::float8 = 0.3::float8;   -- f
SELECT 0.1::numeric + 0.2 = 0.3;                  -- t
SELECT 1/2;                                       -- 0   (integer division)
SELECT round(2.5::numeric), round(2.5::float8);   -- 3, 2  (numeric: half away from zero; float: half to even on this platform)
SELECT '12.345'::numeric(5,2);                    -- 12.35 (silently rounded)
SELECT 2147483647::int + 1;                       -- ERROR: integer out of range
```

- Money: `numeric(p,s)` or integer minor units (cents in `bigint`). Never `float`/`real`/`double precision`. Avoid the `money` type: its output depends on `lc_monetary` (`'1.5'::money` gave `$1.50`).
- Integer division truncates. Cast one side (`x::numeric / y`) when a fraction is expected.
- `numeric(p,s)` rounds extra scale silently and raises an error only when the integer digits overflow. Since 15 a **negative scale** rounds to the left of the decimal point (see above).
- Identifiers: `bigint`. A busy `int` primary key or sequence overflows at 2,147,483,647, and fixing it is a full table rewrite.
- No `1_000_000` or `0xFF` literals on 15 (16).

##### Text

- Use `text` (or `varchar` without a length). `varchar(n)` only adds a length check; changing `n` later can need a rewrite (narrowing). Use `CHECK (length(x) <= n)` if a business rule needs a limit.
- **Never `char(n)`**: it pads with spaces and ignores trailing spaces when comparing (`'ab '::char(5) = 'ab'::char(5)` is true, `length()` is 2, `octet_length()` is 5).
- An explicit cast to `varchar(n)` **silently truncates** (`'abcdef'::varchar(3)` is `abc`). Inserting a too-long value into a `varchar(3)` column raises `value too long`.
- Case-insensitive matching: `lower(x)` with an expression index, `citext`, or a nondeterministic ICU collation (no `LIKE` with nondeterministic collations on 15).
- The `"char"` type (with quotes) is an internal one-byte type; since 15 non-ASCII bytes print as octal escapes. Never use it for application data.

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
- UUID keys: `gen_random_uuid()` (v4, core since 13) causes random B-tree inserts. There is no `uuidv7()` on 15 (18). Generate time-ordered UUIDs in the application if you need them.

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

Enums are compact and ordered, but values cannot be removed or renamed easily, and on 15 a value added with `ADD VALUE` cannot be used until the transaction commits. Use a `CHECK (status IN (...))` or a foreign key to a lookup table when the set changes often.

***

#### JSONB

PostgreSQL 15 has **no SQL/JSON standard functions** (`JSON_OBJECT`, `IS JSON` are 16; `JSON_TABLE`, `JSON_VALUE`, `JSON_QUERY`, `JSON_EXISTS` are 17). Use the `jsonb_*` functions, operators and jsonpath. (`JSON('...')` parses on 15, but only as a function-style cast to the `json` type.)

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
       doc['a']     AS subscript      -- jsonb (14+); parenthesize casts: ('{}'::jsonb)['a']
FROM t;

SELECT ('{"a":"5"}'::jsonb ->> 'a')::int + 1;   -- 6
SELECT ('null'::jsonb)::int;                     -- ERROR: cannot cast jsonb null to type integer
SELECT '{"a":null}'::jsonb ->> 'a' IS NULL;      -- t: JSON null becomes SQL NULL via ->>

-- jsonpath with variables: never build jsonpath text by concatenation
SELECT jsonb_path_query('{"a":[1,2,3]}', '$.a[*] ? (@ > $min)', jsonb_build_object('min', $1));
```

- `jsonb_set(doc, path, NULL)` returns **NULL for the whole document** (the function is strict). `jsonb_set('{"a":1}', '{b}', to_jsonb(NULL::text))` returned NULL, so `UPDATE t SET doc = jsonb_set(doc, ..., $1)` with a NULL parameter wipes the column. Use `jsonb_set_lax(doc, path, value)` (default `'use_json_null'`, gives `{"a": 1, "b": null}`), or `coalesce`.
- Typed extraction on 15: `(doc->>'k')::int`. It raises on bad data, which is usually what you want for imports. Check type first with `jsonb_typeof()` when the field can legitimately vary.
- Validating JSON text without exceptions (`pg_input_is_valid`, `IS JSON`) is 16+. On 15, cast inside a PL/pgSQL `BEGIN ... EXCEPTION WHEN invalid_text_representation` block, or validate in the application.

##### Indexing JSONB (plans from 15.19)

```sql
CREATE INDEX evj_doc_gin ON evj USING gin (doc jsonb_path_ops);

EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc @> '{"type":"click"}';
-- Bitmap Index Scan on evj_doc_gin  Index Cond: (doc @> '{"type": "click"}'::jsonb)
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc @? '$.user ? (@.id == 5)';
-- Bitmap Index Scan on evj_doc_gin
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc ? 'type';
-- Seq Scan  (jsonb_path_ops does not support ?, ?|, ?&; the default jsonb_ops does)
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc->>'type' = 'click';
-- Seq Scan  (GIN does not help ->> equality)

CREATE INDEX evj_type ON evj ((doc->>'type'));
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc->>'type' = 'click';
-- Bitmap Index Scan on evj_type
```

- `jsonb_path_ops` is smaller and faster for `@>`, `@?` and `@@`. The default `jsonb_ops` also supports `?`, `?|` and `?&`.
- For a frequently filtered scalar, use a B-tree expression index on `(doc->>'key')`, or better, promote it to a real column.
- JSONB is not a replacement for a schema. Fields that are always present, used in joins, constrained, or updated often belong in columns. Updating one key rewrites the whole (possibly TOASTed) document and its GIN entries.

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
CREATE TABLE m_def  PARTITION OF m DEFAULT;
```

Rules verified on 15.19:

- **Unique and primary keys must include all partition key columns**: `PRIMARY KEY (id)` on a table partitioned by `at` fails with `unique constraint on partitioned table must include all partitioning columns`. Global uniqueness of `id` alone is not enforced.
- **Identity columns on a partitioned parent work only through the parent on 15.** `INSERT INTO m(at) ...` got an id from the parent's sequence, but `INSERT INTO m_2025(at) ...` directly into a partition failed with `null value in column "id" ... violates not-null constraint`, because partitions do not share the identity before 17. Code (and ETL tools) that write directly into partitions need explicit ids or a sequence default (`DEFAULT nextval('m_id_seq')`) instead of an identity.
- **Pruning needs the raw partition key.** `WHERE at >= '2025-03-01' AND at < '2025-04-01'` scanned only `m_2025`. `WHERE date_trunc('month', at) = '2025-03-01'` scanned **every** partition. Stable expressions like `now() - interval '1 day'` are pruned at execution time (`Subplans Removed: n` in `EXPLAIN ANALYZE`).
- **Default partitions are a trap.** `ATTACH PARTITION` for a new range took `ACCESS EXCLUSIVE` on the default partition and scanned it. With a matching row already in the default partition, the attach failed (`updated partition constraint for default partition "m_def" would be violated by some row`). `DETACH ... CONCURRENTLY` is refused while a default partition exists (`cannot detach partitions concurrently when a default partition exists`). Create future partitions ahead of time (cron, `pg_partman`) and alert when the default partition receives rows.
- **Attaching without a long scan**: add a `CHECK` constraint on the staging table that matches the partition bounds (validated before attaching). `ATTACH PARTITION` then takes only `SHARE UPDATE EXCLUSIVE` on the parent.
- **Indexes**: `ON ONLY` + per-partition `CREATE INDEX CONCURRENTLY` + `ALTER INDEX ... ATTACH PARTITION` (see migrations).
- **No exclusion constraints on partitioned tables on 15** (17). Enforce non-overlap per partition or in a non-partitioned table.
- `CLUSTER` on a partitioned table works since 15, and cross-partition `UPDATE`s fire foreign key actions as an update on the root since 15.
- Retention: `ALTER TABLE m DETACH PARTITION m_2023 CONCURRENTLY; DROP TABLE m_2023;` instead of large `DELETE`s (requires no default partition).
- Autovacuum never analyzes the **parent** of a partitioned table, so run `ANALYZE parent` after large data changes if queries rely on parent-level statistics.
- Too many partitions (thousands) slow planning and locking. 15 improves planning when only a few partitions are relevant, but partition for retention, pruning or maintenance, not by default.

***

#### SQL Idioms and Pitfalls

All outputs in this section are from 15.19.

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
- No `any_value()` on 15 (16).

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

##### Recursive CTEs

```sql
WITH RECURSIVE walk(n) AS (
  SELECT 1
  UNION ALL
  SELECT e.dst FROM walk w JOIN edge e ON e.src = w.n
) CYCLE n SET is_cycle USING path           -- 14+: stops on cycles
SELECT n, is_cycle FROM walk;
-- 1 f / 2 f / 3 f / 1 t
```

Every recursive CTE over user-controlled graphs needs cycle protection (`CYCLE`, a depth limit, or `UNION` instead of `UNION ALL` when rows are fully identical). Without it, one bad edge produces an endless query. `recursive_worktable_factor` (15) only changes cost estimates.

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
- `array_to_tsvector()` rejects empty strings since 15; `ts_delete()` and `setweight()` ignore NULL array elements since 15.

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

On 15 every generated column must be `STORED` (virtual columns are 18), and the expression cannot be changed in place (`SET EXPRESSION` is 17): drop and re-add the column, which rewrites the table. Always list target columns in `INSERT`.

##### Materialized views

- `REFRESH MATERIALIZED VIEW` takes `ACCESS EXCLUSIVE` and blocks readers for the whole refresh.
- `REFRESH MATERIALIZED VIEW CONCURRENTLY` needs a **unique index without a `WHERE` clause**. With only a partial unique index it fails: `cannot refresh materialized view "public.mv" concurrently` / `HINT: Create a unique index with no WHERE clause on one or more columns of the materialized view.` It cannot be used for the first population and is slower than a full refresh when most rows change.
- A materialized view is a snapshot. Code that reads it must tolerate stale data, and something must schedule the refresh.

##### Collation-dependent results

```sql
SELECT 'Z' < 'a' COLLATE "C"          AS c_coll,  -- t   (byte order)
       'Z' < 'a' COLLATE "en_US.utf8" AS en;      -- f   (linguistic order)
```

Sort order and range comparisons on text depend on the collation. Keyset pagination and range filters on text columns must use the same collation as the index, and tests run under a different database locale (for example `C` in CI, `en_US.utf8` in production) can hide ordering bugs. OS upgrades that change glibc collation rules corrupt text indexes; 15's recorded `datcollversion` warns about it.

***

#### Functions, Procedures and Triggers

- **Volatility labels are promises.** `IMMUTABLE` means "same result for the same arguments forever, no table access". A function that reads tables, depends on settings (`TimeZone`, `IntervalStyle`, `search_path`), or calls `now()` must be `STABLE` or `VOLATILE`. A mislabeled `IMMUTABLE` function in an index gives wrong query results after data changes.
- Prefer SQL-standard bodies (`BEGIN ATOMIC ... END`, 14+) for SQL functions. They are parsed at creation time, dependencies are tracked, and name resolution is fixed at creation.
- Set `search_path` on functions that use unqualified names: a must for `SECURITY DEFINER` functions, and strongly advised for functions used by indexes, constraints and materialized views so the code survives a later upgrade to 17+.
- PL/pgSQL: `RAISE EXCEPTION` with `USING ERRCODE`, `GET STACKED DIAGNOSTICS` in exception handlers. `EXCEPTION` blocks create a subtransaction per execution, which is costly in loops and consumes subtransaction IDs. Since 15, `CONSTANT` variables are enforced also for `CALL` output parameters and `OPEN` of refcursors.
- Procedures (`CREATE PROCEDURE`, `CALL`) can `COMMIT` inside, which suits batch jobs, but not when called inside an outer transaction block.
- Triggers: `BEFORE` row triggers for normalization and validation, `AFTER` for side effects. Statement-level triggers with transition tables (`REFERENCING NEW TABLE AS ...`) are cheaper for bulk changes. `CREATE OR REPLACE TRIGGER` (14+). `MERGE` fires statement-level triggers for every action type in its `WHEN` clauses (even if no row took that action) and row-level triggers for the rows it actually changes.
- **PL/Python**: only `plpython3u` exists on 15. `CREATE EXTENSION plpythonu`/`plpython2u` from old migrations fails. Untrusted languages (`*u`) run as the server OS user; only superusers can create such functions.
- `ALTER FUNCTION ... SET work_mem/plan_cache_mode` scopes settings to one function.

***

#### Connection Pooling and Application Access

- **Always pool.** Each PostgreSQL connection is a process with several MB of memory. Size pools to a small multiple of CPU cores, not to the number of application threads.
- **PgBouncer transaction mode** forbids session state across transactions: `SET` without `LOCAL`, session advisory locks, `LISTEN`, `WITH HOLD` cursors, temporary tables, and SQL-level `PREPARE`/`EXECUTE`. Use `SET LOCAL`/`set_config(..., true)` inside transactions, and `pg_advisory_xact_lock`.
- **Protocol-level prepared statements** work in transaction mode with PgBouncer 1.21+ when `max_prepared_statements` is non-zero. SQL `PREPARE` statements are still not tracked.
- `idle_in_transaction_session_timeout` and `idle_session_timeout` **terminate** the server connection. Pools must validate connections and drivers must retry.
- Set `application_name` per service so `pg_stat_activity` and the logs identify the caller.
- Retries: retry on `40001` (serialization), `40P01` (deadlock), and connection loss before commit. Never blindly retry after an unknown commit outcome for non-idempotent writes. Use idempotency keys.
- `sslmode=verify-full` for connections over untrusted networks. libpq has been able to match IP addresses against the certificate's Subject Alternative Name since 15.
- Use `COPY FROM STDIN` (driver copy APIs) or multi-row `INSERT` for bulk writes instead of row-by-row inserts.
- A 15.x server rejects endless SSL/GSS negotiation loops since 15.18 (CVE-2026-6479). libpq's `PQfn()` is marked unsafe since 15.18 (CVE-2026-6477): a malicious server could overrun the client's result buffer; use normal queries instead of the fast-path interface.

***

#### Maintenance, Monitoring and Observability

```sql
-- Slow statements (requires shared_preload_libraries = 'pg_stat_statements' + CREATE EXTENSION)
SELECT query, calls, total_exec_time, mean_exec_time, blk_read_time, temp_blk_read_time  -- 15 column names
FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT 10;

-- Tables with heavy sequential scans (columns are relname, not tablename)
SELECT schemaname, relname, seq_scan, seq_tup_read, idx_scan, n_dead_tup, last_autovacuum
FROM pg_stat_user_tables ORDER BY seq_tup_read DESC LIMIT 10;

-- Unused indexes (check replicas too; counters reset with stats resets)
SELECT schemaname, relname, indexrelname, idx_scan, pg_size_pretty(pg_relation_size(indexrelid))
FROM pg_stat_user_indexes WHERE idx_scan = 0
ORDER BY pg_relation_size(indexrelid) DESC;

-- Long or idle-in-transaction sessions
SELECT pid, state, now() - xact_start AS xact_age, wait_event_type, wait_event, query
FROM pg_stat_activity WHERE xact_start IS NOT NULL ORDER BY xact_start;

-- Who blocks whom
SELECT pid, pg_blocking_pids(pid) AS blocked_by, query FROM pg_stat_activity
WHERE cardinality(pg_blocking_pids(pid)) > 0;

-- Checkpoints (15: still in pg_stat_bgwriter)
SELECT checkpoints_timed, checkpoints_req, checkpoint_write_time, buffers_checkpoint, buffers_backend
FROM pg_stat_bgwriter;
```

- `pg_stat_user_tables` has `relname`; `pg_stat_user_indexes` has `relname` and `indexrelname`. Queries that use `tablename`/`indexname` (columns of `pg_tables`/`pg_indexes`) fail with `column "tablename" does not exist`.
- Monitoring written for 16+ fails on 15: `pg_stat_io`, `last_seq_scan`, `pg_stat_checkpointer` and `shared_blk_read_time` do not exist.
- Statistics live in shared memory since 15 and survive a clean restart, but are reset after a crash or immediate shutdown. Don't base "unused index" decisions on counters that were reset recently (`stats_reset`).
- Autovacuum: tune per table for large, hot tables (`ALTER TABLE t SET (autovacuum_vacuum_scale_factor = 0.01)`) rather than turning it off. Watch `n_dead_tup`, `last_autovacuum`, and `age(datfrozenxid)` for wraparound.
- Logging: `log_min_duration_statement`, `log_lock_waits = on`, `log_temp_files = 0`, `auto_explain` for slow plans, `jsonlog` for structured output. `log_checkpoints` is on by default since 15.
- Plain `REINDEX` blocks writes. Use `REINDEX ... CONCURRENTLY` and only when bloat, corruption or a release note warrants it.

***

#### Upgrading to 15 and Staying Current

##### Before upgrading from 14 (or older)

1. Remove `stats_temp_directory` from the configuration (the server refuses to start with an unknown core setting in `postgresql.conf`).
2. Replace `pg_start_backup()`/`pg_stop_backup()`/`pg_is_in_backup()` in backup scripts with `pg_backup_start()`/`pg_backup_stop()` in one persistent session, or switch to `pg_basebackup`/pgBackRest.
3. Port PL/Python 2 functions to `plpython3u` before upgrading: 15 ships no Python 2 language, so objects that depend on `plpythonu`/`plpython2u` cannot be carried over.
4. Search SQL for numeric literals directly followed by identifiers (`123abc`), interval literals with fractional years, and index expressions over `interval::text`.
5. Decide on the `public` schema ACL. Upgraded databases keep `CREATE` for `PUBLIC`; new databases created by the same deployment do not. Make the migration role and schema layout explicit so both behave the same.
6. Review `work_mem` sizing against the new `hash_mem_multiplier = 2.0`.
7. Expect more log volume from `log_checkpoints` and autovacuum logging; set the values explicitly.
8. Tests that rely on `setseed()` + `random()` values, or on psql showing only the last result of a multi-statement string, need updates.
9. After `pg_upgrade`: run `vacuumdb --all --analyze-in-stages` (statistics are not transferred).
10. If the OS (glibc) changes at the same time, reindex text indexes, then `ALTER DATABASE ... REFRESH COLLATION VERSION`.

##### Preparing 15 code for 16 and 17

PostgreSQL 15 is supported until November 11, 2027. Code written now should keep working after the upgrade:

- Add `SET search_path = ...` (or schema-qualify) to functions used in expression indexes, index predicates and materialized views; 17 runs maintenance with `search_path = pg_catalog, pg_temp` and such objects fail on `REINDEX`, `ANALYZE` and `REFRESH`.
- Don't use `force_parallel_mode`, `vacuum_defer_cleanup_age`, `promote_trigger_file`, `SHOW lc_collate` or manual `ON SELECT` rules (removed in 16), or `pg_stat_bgwriter.checkpoints_*`/`buffers_backend` and `pg_stat_statements.blk_read_time` in long-lived monitoring (moved in 17).
- Don't rely on `CREATEROLE` being able to grant arbitrary roles (restricted in 16).
- Write interval literals with `ago` last (required in 17).

##### 15.x minor releases

Minor releases contain only fixes and need no dump/restore, but some require actions:

- **15.1 (November 10, 2022)**: fixed failure to remove non-first 1 GB segments of large dropped tables (temporary tables, and WAL replay on standbys); check for orphaned `NNNN.NN` files after crashes on 15.0.
- **15.2 (February 9, 2023)**: CVE-2022-41862 (libpq memory disclosure during GSSAPI encryption setup).
- **15.3 (May 11, 2023)**: CVE-2023-2454 (`CREATE SCHEMA` defeating a protective `search_path`), CVE-2023-2455 (RLS policies not enforced after inlining a set-returning SQL function when a cached plan was reused under a different role).
- **15.4 (August 10, 2023)**: CVE-2023-39417 (extension script substitution), CVE-2023-39418 (`MERGE` did not enforce `UPDATE`/`SELECT` RLS policies). Reindex BRIN indexes that are used to search for NULLs.
- **15.5 (November 9, 2023)**: CVE-2023-5868/5869/5870 (memory disclosure, array-dimension overflow, `pg_signal_backend` signalling background workers). Reindex GiST indexes (page-split bug), B-tree indexes on `interval` columns (deduplication bug), and BRIN `minmax_multi` indexes on date/timestamp columns. `MERGE` concurrency fixes.
- **15.6 (February 8, 2024)**: CVE-2024-0985 (`REFRESH MATERIALIZED VIEW CONCURRENTLY`). Fixed possible GIN index corruption during concurrent updates; reindex suspect GIN indexes. Several `MERGE` cross-partition fixes.
- **15.7 (May 9, 2024)**: CVE-2024-4317: `pg_stats_ext` and `pg_stats_ext_exprs` exposed statistics to users without table privileges; existing databases need a manual catalog fix described in the release notes.
- **15.8 (August 8, 2024)**: CVE-2024-7348 (code execution during `pg_dump` via object replacement); new setting `restrict_nonsystem_relation_kind`.
- **15.9 (November 14, 2024)**: CVE-2024-10976/10977/10978/10979 (RLS-dependent cached plans, libpq trusting pre-TLS error messages, `SET SESSION AUTHORIZATION`/`SET ROLE` interaction, PL/Perl environment variables). Partitions detached from a partitioned table that has a foreign key to another partitioned table, and not dropped, may need catalog repair.
- **15.10 (November 21, 2024)**: out of cycle; restored extension ABI compatibility (`ResultRelInfo`) and `ALTER {ROLE|DATABASE} SET role`.
- **15.11 (February 13, 2025)**: CVE-2025-1094 (libpq quoting functions with invalidly encoded input). **15.12 (February 20, 2025)**: out of cycle; fixed the length-argument regression of that fix.
- **15.13 (May 8, 2025)**: CVE-2025-4207 (GB18030 over-read). Self-referential foreign keys on partitioned tables may need to be recreated; reindex BRIN bloom indexes.
- **15.14 (August 14, 2025)**: CVE-2025-8713 (statistics leak through views and partitions; permission errors can now appear at planning time), CVE-2025-8714/8715 (`pg_dump` output injection; psql `\restrict`). Reindex BRIN `numeric_minmax_multi_ops` indexes.
- **15.15 (November 13, 2025)**: CVE-2025-12817 (`CREATE STATISTICS` checks `CREATE` on the schema), CVE-2025-12818 (libpq allocation overflow).
- **15.16 (February 12, 2026)**: CVE-2026-2003 to 2006 (`oidvector`/`int2vector` checks, hardened selectivity estimators, pgcrypto PGP buffer overrun, multibyte length validation). **15.17 (February 26, 2026)**: out of cycle; fixed multixact WAL replay on standbys and a too-strict `substring()` encoding check.
- **15.18 (May 14, 2026)**: CVEs including startup-packet recursion (CVE-2026-6479), allocation overflows (CVE-2026-6473), time zone names in `timeofday()` (CVE-2026-6474), multirange schema privilege (CVE-2026-6472), timing-safe password comparison (CVE-2026-6478), `PQfn()` marked unsafe (CVE-2026-6477), path traversal in `pg_basebackup`/`pg_rewind` (CVE-2026-6475), over-length `ts_headline()` options (CVE-2026-6473) and `contrib/refint` quoting (CVE-2026-6637).
- **15.19 (August 13, 2026)**: about 30 CVE fixes. Logical decoding output plugins other than `pgoutput`/`test_decoding` must be allow-listed in `output_plugin_libraries` (CVE-2026-6471). pgcrypto PGP decryption fails for data that was never actually encrypted because OpenSSL rejected the cipher (CVE-2026-14663; `ignore-cipher-failure=1` recovers it). psql skips the inline data of a failed `COPY ... FROM STDIN` (CVE-2026-6464): test scripts need a `\.` after deliberately failing `COPY` commands. Buffer overruns in `to_char()` (CVE-2026-14669) and the regexp functions (CVE-2026-14664). Reindex `btree_gist` and `ltree` indexes as described in the notes.

Pin `postgres:15` (floating minor) or an exact `postgres:15.19` in containers, and update minors promptly.

***

#### Production / Review Checklist

##### Version compatibility

- [ ] No PostgreSQL 16+ features: SQL/JSON (`JSON_OBJECT`, `IS JSON`, `JSON_TABLE`, `JSON_VALUE`), `pg_input_is_valid()`, `any_value()`, `1_000`/`0x1F` literals, unaliased `FROM` subqueries, `GRANT ... WITH INHERIT/SET`, `COPY DEFAULT`, `EXPLAIN (GENERIC_PLAN)`
- [ ] No 17+ features: `MERGE ... RETURNING`, `NOT MATCHED BY SOURCE`, `MERGE` into views, `COPY ON_ERROR`, `transaction_timeout`, `MAINTAIN`, `random(min,max)`, `pg_stat_checkpointer`
- [ ] No 18+ features: `uuidv7()`, virtual generated columns, `RETURNING old/new`, `WITHOUT OVERLAPS`, `NOT ENFORCED`, `VACUUM ONLY`
- [ ] Code shared with 14 does not use 15 features (`MERGE`, `NULLS NOT DISTINCT`, `security_invoker`, `regexp_count()` and friends, `ON DELETE SET NULL (col)`, `GRANT SET ON PARAMETER`)
- [ ] No removed functions or settings (`pg_start_backup`, `pg_is_in_backup`, `plpythonu`, `stats_temp_directory`)

##### Migrations

- [ ] `lock_timeout` set (with retry) for every DDL on existing tables; `SET LOCAL` inside transactions
- [ ] `CREATE INDEX CONCURRENTLY` / `DROP INDEX CONCURRENTLY` on live tables, outside transaction blocks; invalid indexes handled on retry
- [ ] Foreign keys and `CHECK` constraints added `NOT VALID` then `VALIDATE`d; `SET NOT NULL` preceded by a validated `CHECK (col IS NOT NULL)`
- [ ] No volatile defaults or type changes on large tables without a rewrite plan
- [ ] Backfills and deletes in batches
- [ ] Expand/contract for renames and drops; new enum values committed before use
- [ ] No `CREATE` grants on `public` to `PUBLIC`; objects in an application schema
- [ ] Logical replication row filters / column lists cover the replica identity, or the publication is insert-only
- [ ] Functions used by indexes or materialized views have `SET search_path` (future 17 upgrade)
- [ ] Foreign key columns indexed

##### Queries

- [ ] Parameters bound (no string concatenation), `= ANY($1)` for lists, identifiers allow-listed
- [ ] Predicates sargable: no functions or casts on indexed columns, half-open time ranges, parameter types match column types
- [ ] Keyset pagination with a row-value comparison and a matching composite index
- [ ] `NOT EXISTS` instead of `NOT IN (subquery)` on nullable columns; correlated `IN` rewritten as `EXISTS`
- [ ] Composite indexes match leading columns (no skip scan)
- [ ] `EXPLAIN (ANALYZE, BUFFERS)` evidence for performance claims; DML explained inside `BEGIN ... ROLLBACK`

##### Transactions and concurrency

- [ ] Read-modify-write made atomic (`UPDATE ... SET x = x + ...`, `FOR UPDATE`, or version checks)
- [ ] Retry loops for `40001`/`40P01` with `REPEATABLE READ`/`SERIALIZABLE`
- [ ] Concurrent single-key upserts use `INSERT ... ON CONFLICT`, not `MERGE`; `MERGE` sources deduplicated
- [ ] Short transactions; no network calls inside; timeouts set per role
- [ ] Queues use `FOR UPDATE SKIP LOCKED`; advisory locks are transaction-scoped

##### Security

- [ ] `SECURITY DEFINER` functions: `SET search_path = ..., pg_temp`, qualified names, `REVOKE EXECUTE ... FROM PUBLIC`
- [ ] Default `EXECUTE` revoked with the global `ALTER DEFAULT PRIVILEGES` form, not `IN SCHEMA`
- [ ] Dynamic SQL uses `format('%I')` + `USING`, never `%s` with input
- [ ] App role is not owner, superuser or `CREATEROLE`; RLS tables use `FORCE ROW LEVEL SECURITY` where owners connect; tenant set with `SET LOCAL`; `''` not accepted as a tenant
- [ ] Views over RLS-protected or restricted tables use `security_invoker = true`
- [ ] No server-side `COPY` file/program from application paths; no passwords in logged SQL
- [ ] Server on the latest 15.x minor (15.19 as of October 2026), upgrade to a newer major planned before November 11, 2027

##### Data types and schema

- [ ] `timestamptz` for instants; `bigint` identity for keys; `numeric` (non-negative scale) or integer minor units for money; `text` + `CHECK` instead of `char(n)`
- [ ] `GENERATED ALWAYS AS IDENTITY` instead of `serial`; no direct inserts into partitions of a table whose identity lives on the parent
- [ ] `NULLS NOT DISTINCT` where NULL must be unique
- [ ] JSONB only for genuinely flexible data; GIN `jsonb_path_ops` for `@>`/`@?`; expression indexes for hot keys; `jsonb_set_lax` or NULL guards
- [ ] Partitioned tables: keys include the partition key, queries filter on the raw key, future partitions pre-created, default partition monitored

##### Operations

- [ ] Pooling in place; no session state with transaction pooling
- [ ] `pg_stat_statements`, `log_lock_waits`, `log_min_duration_statement` configured; monitoring queries use 15 column names
- [ ] Backups via `pg_basebackup`/maintained tools or `pg_backup_start`/`pg_backup_stop` in one session, with `backup_label` written
- [ ] `ANALYZE` after bulk loads and after `pg_upgrade`; reindex steps from minor release notes applied
