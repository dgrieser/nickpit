### PostgreSQL 16: The Complete Review Guide

#### Overview

> **Version**: PostgreSQL 16.0 was released on **September 14, 2023**.[^1][^4] The latest minor release is **16.15 (August 13, 2026)**; the next scheduled minor release date is November 12, 2026.[^2][^3] PostgreSQL 16 is **supported until November 9, 2028**.[^2]
> **Newest releases (October 2026)**: PostgreSQL **18** is the current stable major version (18.0 on September 25, 2025; current minor 18.6). PostgreSQL 17 is at 17.11. PostgreSQL **19 is at Release Candidate 1** (October 15, 2026). It is pre-release software and not a deployment target.[^2] PostgreSQL 14 reaches end of life on November 12, 2026, and PostgreSQL 15 on November 11, 2027.
> **Scope**: SQL that runs on a PostgreSQL 16 server: schema migrations, queries embedded in application code, functions, procedures and triggers, schema design, indexing, transactions, security, JSON, partitioning, connection pooling and operations-facing SQL.
> **Note**: This document is self-contained. It carries forward everything from PostgreSQL 14 and 15 that is still correct, and it marks the version in which each feature arrived. Sections that cover 16-specific behaviour are marked **New in 16**. Every example was executed on PostgreSQL 16.15. "New in 16" claims were checked to fail on 15.19, "not available in 16" claims were checked to fail on 16.15 (and the 17 behaviour changes were checked on 17.11). "Since 15" markers were checked to fail on 14.24.

PostgreSQL 16 is a developer-experience, security and replication release. Its SQL-visible headline features are the **SQL/JSON constructors and predicates** (`JSON_OBJECT()`, `JSON_ARRAY()`, `JSON_OBJECTAGG()`, `JSON_ARRAYAGG()`, `IS JSON [VALUE | OBJECT | ARRAY | SCALAR] [WITH UNIQUE KEYS]`), **non-decimal integer literals** (`0x1F`, `0o17`, `0b101`) and **underscores in numbers** (`1_000_000`), the **`any_value()` aggregate**, **`pg_input_is_valid()`/`pg_input_error_info()`**, time-zone-aware **`date_add()`/`date_subtract()`/`generate_series()`**, **`COPY ... DEFAULT`**, and **`EXPLAIN (GENERIC_PLAN)`**. For operations it adds **`pg_stat_io`**, `last_seq_scan`/`last_idx_scan`, **parallel `FULL` and right outer hash joins**, **incremental sort for `DISTINCT`**, pre-sorted input for `ORDER BY`/`DISTINCT` aggregates, **logical decoding on standbys**, **parallel apply** of large transactions on subscribers, the `pg_create_subscription` and `pg_use_reserved_connections` roles, **regular expressions and include files in `pg_hba.conf`**, and the libpq options **`require_auth`**, **`load_balance_hosts`** and **`sslrootcert=system`**.[^1][^4]

PostgreSQL 16 also changes behaviour that breaks old code and scripts. **`CREATEROLE` no longer means "almost superuser"**: changing, dropping or granting other roles now needs `ADMIN OPTION` on them. **Role membership has explicit `INHERIT` and `SET` options**, fixed at `GRANT` time. **Manually created `ON SELECT` rules (hand-made views) are rejected**, **`NULLS NOT DISTINCT` indexes can no longer back a primary key**, **`REINDEX DATABASE` skips system catalogs**, **PL/pgSQL bound cursor variables are no longer named after the variable**, and the settings `force_parallel_mode` (now `debug_parallel_query`), `vacuum_defer_cleanup_age`, `promote_trigger_file`, `lc_collate` and `lc_ctype` are gone.[^1]

Global truths about PostgreSQL 16:

- **MVCC.** Readers never block writers and writers never block readers at the row level. `UPDATE` writes a new row version and `DELETE` marks the old one dead. `VACUUM` (usually autovacuum) reclaims dead versions. Long transactions stop that reclamation everywhere in the cluster.
- **DDL is transactional, but it takes locks.** Almost every `ALTER TABLE` form takes an `ACCESS EXCLUSIVE` lock. The lock is held until commit, and it queues behind running queries while blocking new ones (see [Schema migrations and locking](#schema-migrations-and-locking)).
- **Default isolation is READ COMMITTED.** Each statement sees a fresh snapshot. A read-modify-write in application code is a lost update unless it is made atomic.
- **NULL is three-valued.** `NULL = NULL` is NULL, `NOT IN` with a NULL in the list never returns true, and `count(col)` skips NULLs.
- **Identifiers fold to lower case** unless they are double-quoted. String literals use single quotes.
- **Arrays are 1-based.** `arr[0]` is NULL, not an error.
- **Use `timestamptz`, `text`, `bigint` identity and `numeric` for money** by default. The rest of this guide explains why.

***

#### Version-Aware Review

The project pins PostgreSQL 16 (for example `image: postgres:16`, `postgresql 16.x` in `.tool-versions`, a managed-database engine version, or a CI service container). Apply these rules when reviewing code for it:

- **Verify before you claim something is unavailable.** A function or syntax that you remember as "new" may already be in 16 (for example `JSON_OBJECT`, `IS JSON`, `any_value()`, `0x1F` literals, `MERGE` from 15). Say "not available in PostgreSQL 16" only for items on the [Do Not Use or Recommend](#do-not-use-or-recommend-postgresql-17-features) list or when the 16 documentation proves it.
- **Never suggest PostgreSQL 17 or 18 features.** `JSON_TABLE`, `JSON_VALUE`, `JSON_QUERY`, `JSON_EXISTS`, `MERGE ... RETURNING`, `WHEN NOT MATCHED BY SOURCE`, `COPY ... ON_ERROR`, `transaction_timeout`, `random(min, max)`, `GRANT MAINTAIN`, `uuidv7()`, virtual generated columns and `RETURNING old/new` do not exist in 16. Code that uses them fails with a syntax error or "function does not exist".
- **Flag code that only works on newer servers.** Example: `SELECT JSON_VALUE(doc, '$.a' RETURNING int)` is fine on 17 and fails on 16 with `syntax error at or near "RETURNING"` (without `RETURNING`, the error is `function json_value(jsonb, unknown) does not exist`).
- **Flag code that assumes older behaviour.** Examples: a provisioning script in which a `CREATEROLE` role alters or grants roles it did not create (worked on 15, fails on 16); monitoring that runs `SHOW lc_collate` (removed in 16).
- **Flag code that will break on the next upgrade.** 16 still runs maintenance commands with the caller's `search_path` and accepts `'1 day ago 2 hours'`; 17 does not (see [Preparing for PostgreSQL 17](#preparing-for-postgresql-17)). Point these out as forward-compatibility risks, not as bugs on 16.
- **Many projects run several versions.** If the code also has to run on 15 (a library, an extension, a shared migration set), every **New in 16** feature in this guide is a portability bug there.
- **Client libraries are separate.** `require_auth`, `load_balance_hosts`, `sslcertmode` and `sslrootcert=system` are libpq 16 features. They need libpq 16 or later on the client (or a driver that implements them itself), not only a 16 server.

***

#### What's New in PostgreSQL 16

All items below are verified against the official release notes[^1] and, where they are SQL-visible, were executed on 16.15 and shown to fail on 15.19.

##### SQL and data types

| Feature | Notes |
|---|---|
| `JSON_OBJECT()`, `JSON_ARRAY()` | SQL-standard constructors: `JSON_OBJECT('a': 1, 'b' VALUE 2)`, `NULL ON NULL`/`ABSENT ON NULL`, `WITH UNIQUE KEYS`, `RETURNING jsonb`, `FORMAT JSON`. `JSON_ARRAY(SELECT ...)` builds an array from a subquery. |
| `JSON_OBJECTAGG()`, `JSON_ARRAYAGG()` | SQL-standard aggregates with the same clauses, plus `ORDER BY` inside `JSON_ARRAYAGG`. |
| `expr IS [NOT] JSON [VALUE \| OBJECT \| ARRAY \| SCALAR] [WITH \| WITHOUT UNIQUE KEYS]` | Validates text, `json`, `jsonb` and `bytea` without raising errors. |
| Non-decimal integer literals | `0x1F`, `0o17`, `0b101`. Values above `bigint` become `numeric`. |
| Underscores in numeric literals | `1_000_000`, `1_000.5`. Also accepted by integer and `numeric` **input** (`'1_000'::int`, `'0x10'::bigint`). |
| `any_value(expr)` | Aggregate that returns an arbitrary non-NULL value of the group. |
| `pg_input_is_valid(text, type)`, `pg_input_error_info(text, type)` | Check whether a string is valid input for a type without raising an error or opening a subtransaction. |
| `date_add()`, `date_subtract()`, `generate_series(start, stop, step, timezone)` | `timestamptz` arithmetic in a named time zone, independent of the session `TimeZone`. |
| `date_trunc(unit, timestamptz, timezone)` is now `IMMUTABLE` | Usable in expression indexes. |
| `array_sample()`, `array_shuffle()`, `random_normal()`, `erf()`, `erfc()` | |
| `SYSTEM_USER` | Authentication method and authenticated identity, e.g. `scram-sha-256:alice`. NULL for `trust`. |
| Subqueries in `FROM` without an alias | `SELECT * FROM (SELECT 1 AS x)` |
| `XMLSERIALIZE(... INDENT)` | Pretty-printed XML. |
| `'+infinity'` datetime input, `pg_size_bytes('10 B')` | |
| `pg_read_file(path, missing_ok)` / `pg_read_binary_file(path, missing_ok)`, `to_reg*()` accepting numeric OIDs (`to_regclass('1259')`), `pg_split_walfile_name()` | |
| `fuzzystrmatch`: `daitch_mokotoff()` | Daitch-Mokotoff Soundex codes as an array |
| Numeric literal syntax (hex, underscores) inside jsonpath | `'$.a ? (@ == 0xFF)'` |
| `CREATE TABLE ... (col type STORAGE EXTERNAL)` | Previously only `ALTER TABLE`. |
| `CREATE STATISTICS ON a, b FROM t` | The name is generated when omitted. |
| PL/pgSQL `GET DIAGNOSTICS v = PG_ROUTINE_OID` | OID of the current function. |
| ICU: predefined collation `unicode`, custom rules (`CREATE COLLATION ... (provider = icu, rules = '...')`) | ICU is built by default. `ucs_basic` existed before. |

##### Utility commands

| Feature | Notes |
|---|---|
| `COPY ... FROM ... (DEFAULT '\D')` | A marker string in the input means "use the column default". |
| `VACUUM (BUFFER_USAGE_LIMIT '...')`, `vacuum_buffer_usage_limit` (default 256kB) | Ring buffer size for `VACUUM`/`ANALYZE` and autovacuum. `vacuumdb --buffer-usage-limit`. |
| `VACUUM (PROCESS_MAIN false)` | Vacuum only the TOAST table. |
| `VACUUM (SKIP_DATABASE_STATS)`, `VACUUM (ONLY_DATABASE_STATS)` | Defer or only do the database-wide frozen-XID update. |
| `EXPLAIN (GENERIC_PLAN)` | Explains a statement with `$1` placeholders without values. Cannot be combined with `ANALYZE`. |
| `REINDEX DATABASE` / `REINDEX SYSTEM` without a name | And `REINDEX DATABASE` no longer touches system catalogs. |
| `GRANT ... WITH ADMIN TRUE/FALSE`, `WITH INHERIT TRUE/FALSE`, `WITH SET TRUE/FALSE`, `GRANTED BY` | Per-grant role membership options. |
| `ALTER GROUP g ADD USER u` with `ADMIN OPTION` on `g` | No `CREATEROLE` or superuser needed |
| psql `\bind`, `\drg`, `\watch` count, `SHELL_ERROR`/`SHELL_EXIT_CODE`, `\pset xheader_width`, `S` modifier for `\dp`/`\z` | `\bind` sends a query with the extended protocol: `SELECT $1::int + 1 \bind 41 \g`. |

##### Server, performance and operations

- **Planner.** Parallel hash `FULL` and right outer joins; incremental sort for `SELECT DISTINCT`; aggregates with `ORDER BY`/`DISTINCT` (`array_agg(x ORDER BY x)`, `string_agg(DISTINCT ...)`) can use pre-sorted input (`enable_presorted_aggregate`); anti-joins with the non-nullable side as the inner relation; `Memoize` above `UNION ALL`; window functions use `ROWS` mode internally when `RANGE` is unnecessary; run conditions for `ntile()`, `cume_dist()` and `percent_rank()`; `string_agg()` and `array_agg()` can run in parallel; cached `RANGE`/`LIST` partition lookups.[^1]
- **Vacuum.** Opportunistic page freezing during normal vacuums makes anti-wraparound full-table freezes less necessary. `vacuum_buffer_usage_limit` and `BUFFER_USAGE_LIMIT` control the buffer ring size.
- **HOT updates** are now possible when only BRIN-indexed columns change.
- **Monitoring.** `pg_stat_io` (I/O per backend type, object and context), `pg_stat_*_tables.last_seq_scan`/`last_idx_scan` and `pg_stat_*_indexes.last_idx_scan`, `n_tup_newpage_upd`, speculative insertion locks in `pg_locks`, result types in `pg_prepared_statements`, checkpoint and REDO LSN in `log_checkpoints` messages, `pg_stat_get_backend_subxact()`.
- **Logical replication.** Logical decoding on standbys (`pg_log_standby_snapshot()`), parallel apply (`streaming = parallel`, `max_parallel_apply_workers_per_subscription`), `origin = none` to avoid replication loops, binary initial table sync, B-tree index use for `REPLICA IDENTITY FULL`, apply as the table owner (`run_as_owner` restores the old behaviour), `password_required = false` (superuser-only), per-subscription `wal_retrieve_retry_interval`.
- **Privileges.** `pg_create_subscription`, `pg_use_reserved_connections` with `reserved_connections`, `createrole_self_grant`, simpler `LOCK TABLE` permissions, `makeaclitem()` with several privileges.
- **Authentication and configuration.** Regular expressions (`/pattern`) for databases and users in `pg_hba.conf`, `include`/`include_if_exists`/`include_dir` in `pg_hba.conf` and `pg_ident.conf`, `scram_iterations` (default 4096), Kerberos credential delegation (`gss_accept_delegation`), `send_abort_for_crash`/`send_abort_for_kill`, `debug_io_direct` (developer option), `archive_library` and `archive_command` can no longer both be set, `debug_logical_replication_streaming` (developer option that forces immediate streaming or serialization of logical changes), configuration file nesting is limited to 10 levels, and the bootstrap superuser can no longer lose superuser.
- **libpq.** `require_auth`, `load_balance_hosts=random`, `sslcertmode`, `sslrootcert=system` (which also forces `sslmode=verify-full`), `gssdelegation`. SCM credential authentication support was removed from libpq.
- **Tools.** `pg_dump --table-and-children`/`--exclude-table-and-children`/`--exclude-table-data-and-children`, LZ4 and Zstandard compression in `pg_dump` (`--compress=zstd:5`), `vacuumdb --schema`/`--exclude-schema`, `initdb -c name=value`, `pg_upgrade --copy`, `pg_waldump --save-fullpage`, `createuser --member-of` (`--role` is deprecated).
- **Foreign data.** `COPY` into foreign tables can insert in batches (`batch_size`); truncate triggers on foreign tables; `postgres_fdw` aborts remote transactions in parallel (`parallel_abort`) and ships `reg*` constants only for built-in or shippable-extension objects.
- **Extensions.** `pg_stat_statements` normalizes constants in utility commands; `auto_explain.log_parameter_max_length` logs bind parameters; `pg_buffercache_summary()` and `pg_buffercache_usage_counts()`; `pg_walinspect` changes; `@extschema:name@` in extension scripts; `ltree` labels up to 1000 characters and may contain hyphens.
- **Platforms.** Windows 10 or later is required; meson is a new build system.

***

#### Do Not Use or Recommend (PostgreSQL 17+ Features)

The server is PostgreSQL 16. Everything in the first table was introduced in **PostgreSQL 17** (released September 26, 2024)[^5] and was shown to fail on 16.15. Never suggest these, and flag them as bugs when they appear in code that targets 16:

| PostgreSQL 17 feature | What happens on 16 | Use on 16 instead |
|---|---|---|
| `JSON_TABLE(...)` | `syntax error at or near "COLUMNS"` | `jsonb_array_elements()` / `jsonb_to_recordset()` in a `LATERAL` join, with explicit casts |
| `JSON_VALUE()`, `JSON_QUERY()`, `JSON_EXISTS()` | `function json_value(jsonb, unknown) does not exist`; with a `RETURNING`/`ON ERROR` clause, `syntax error` | `(doc->>'a')::int`, `doc->'a'`, `jsonb_path_query_first()`, `doc @? '$.a'` / `jsonb_path_exists()` |
| `JSON(... WITH UNIQUE KEYS)`, `JSON_SCALAR()`, `JSON_SERIALIZE()` | `syntax error at or near "WITH"` / function does not exist (a bare `JSON('...')` parses as a cast to `json` on 16) | `x IS JSON WITH UNIQUE KEYS` (16), `to_json()`, `::text` |
| jsonpath methods `.integer()`, `.bigint()`, `.decimal()`, `.boolean()`, `.string()`, `.date()`, `.time()`, `.timestamp()`, `.timestamp_tz()` ... | `syntax error at or near "(" of jsonpath input` | Extract with `->>` and cast in SQL |
| `MERGE ... RETURNING`, `merge_action()` | `syntax error at or near "RETURNING"` | `INSERT ... ON CONFLICT ... RETURNING`, or a data-modifying CTE |
| `MERGE ... WHEN NOT MATCHED BY SOURCE` | `syntax error at or near "BY"` | A separate `DELETE ... WHERE NOT EXISTS (...)` in the same transaction |
| `MERGE` into a view | `cannot execute MERGE on relation "sv"` | `MERGE` into the base table |
| `COPY ... (ON_ERROR ignore)`, `LOG_VERBOSITY`, `FORCE_NULL *`/`FORCE_NOT_NULL *` | `option "on_error" not recognized`; `argument to option "force_null" must be a list of column names` | Load into a `text` staging table and validate with `pg_input_is_valid()` (16), list columns explicitly |
| `transaction_timeout` | `unrecognized configuration parameter "transaction_timeout"` | `statement_timeout` + `idle_in_transaction_session_timeout`, and application-side deadlines |
| `GRANT MAINTAIN ON TABLE`, `pg_maintain` | `unrecognized privilege type "maintain"`, `role "pg_maintain" does not exist` | Table ownership (or membership in the owning role) for `VACUUM FULL`/`CLUSTER`/`REINDEX`/`REFRESH`; `pg_checkpoint`, `pg_signal_backend`, `pg_monitor` where they fit |
| `random(min, max)` | `function random(integer, integer) does not exist` | `floor(random() * (max - min + 1))::int + min` |
| `uuid_extract_timestamp()`, `uuid_extract_version()`, `to_bin()`, `to_oct()`, `unicode_version()`, `pg_basetype()`, `to_regtypemod()` | function does not exist | |
| `ts AT LOCAL` | `syntax error at or near "LOCAL"` | `ts AT TIME ZONE current_setting('TimeZone')` |
| `interval 'infinity'` | `invalid input syntax for type interval: "infinity"` | NULL or a sentinel, documented |
| Identity columns that **partitions** share; exclusion constraints on partitioned tables | Partitions do not get the identity (direct inserts into a partition fail); `exclusion constraints are not supported on partitioned tables` | Insert through the parent; per-partition exclusion constraints (see [Partitioning](#partitioning)) |
| `ALTER TABLE ... ALTER COLUMN ... SET EXPRESSION AS (...)` | `syntax error at or near "EXPRESSION"` | Drop and re-add the generated column (rewrites the table) |
| `ALTER TABLE ... SET STATISTICS DEFAULT` | syntax error | `SET STATISTICS -1` |
| Using an enum value added in the same transaction that created the type | `unsafe use of new value "b" of enum type mood2` | Commit between `ALTER TYPE ... ADD VALUE` and its first use |
| `EXPLAIN (SERIALIZE)`, `EXPLAIN (MEMORY)` | `unrecognized EXPLAIN option "serialize"` | `EXPLAIN (ANALYZE, BUFFERS)` |
| `pg_stat_checkpointer`, `pg_wait_events` | `relation "pg_stat_checkpointer" does not exist` | `pg_stat_bgwriter.checkpoints_timed`, `checkpoints_req` ... (the 16 names) |
| Builtin collation provider (`provider = builtin`, `C.UTF-8`, `pg_c_utf8`) | `unrecognized collation provider: builtin` | `COLLATE "C"`, ICU, or `text_pattern_ops` indexes |
| `ON login` event triggers, `event_triggers` setting | `unrecognized event name "login"` | |
| `allow_alter_system`, `summarize_wal`, `io_combine_limit`, `sync_replication_slots`, `synchronized_standby_slots` | unrecognized configuration parameter | |
| Failover logical slots (`failover => true`), `pg_createsubscriber`, incremental backups (`pg_basebackup --incremental`, `pg_combinebackup`) | not available | Recreate slots after failover; full base backups |
| `sslnegotiation=direct`, `PQchangePassword()`, the new libpq cancel API | libpq 17 features | `sslmode=verify-full`; psql `\password` |
| `pg_stat_reset_shared()` with no argument | `function pg_stat_reset_shared() does not exist` | Pass the target (`'bgwriter'`, `'io'`, ...) |
| PL/pgSQL `%TYPE[]`/`%ROWTYPE[]` | not available | Declare the array type explicitly |

From **PostgreSQL 18** (released September 25, 2025)[^6], also shown to fail on 16.15: `uuidv7()`/`uuidv4()` (`function uuidv7() does not exist`; use `gen_random_uuid()`, core since 13), virtual generated columns (`GENERATED ALWAYS AS (expr)` without `STORED` is a **syntax error** on 16), `RETURNING old.*/new.*` (`missing FROM-clause entry for table "old"`), `WITHOUT OVERLAPS`/`PERIOD` temporal keys (use an `EXCLUDE USING gist` constraint with `btree_gist`), `NOT ENFORCED` constraints, `NOT NULL ... NOT VALID` constraints, `VACUUM ONLY`/`ANALYZE ONLY`, `COPY ... REJECT_LIMIT`, `casefold()`, `array_sort()`, `array_reverse()`, `crc32()`, `reverse(bytea)`, `gamma()`, integer↔`bytea` casts (`cannot cast type integer to bytea`), `('null'::jsonb)::int` returning NULL (16 raises `cannot cast jsonb null to type integer`), `jsonb_strip_nulls(x, true)`, asynchronous I/O (`io_method`), B-tree skip scan, `EXPLAIN ANALYZE` that includes `BUFFERS` automatically, OAuth authentication, `md5_password_warnings`, and `pg_upgrade` keeping optimizer statistics.

Anything from **PostgreSQL 19** (Release Candidate 1, October 15, 2026, not yet released) is also unavailable, for example the `REPACK` command (`syntax error at or near "REPACK"` on 16.15) and `error_on_null()`.[^7] `GROUP BY ALL` is not available on 16 either (syntax error).

##### Removed or changed in 16 (do not use)

| Removed / changed | Replacement |
|---|---|
| `force_parallel_mode` | Renamed to `debug_parallel_query` (a developer setting; never set it in production) |
| `vacuum_defer_cleanup_age` | `hot_standby_feedback` and replication slots |
| `promote_trigger_file` | `pg_ctl promote` or `SELECT pg_promote()` |
| `SHOW lc_collate`, `SHOW lc_ctype` (read-only settings) | `SELECT datcollate, datctype FROM pg_database WHERE datname = current_database()` |
| Creating a view by hand with `CREATE RULE "_RETURN" AS ON SELECT ...` on a table | `CREATE VIEW`. 16 raises `relation "rv" cannot have ON SELECT rules` (15 silently turned the table into a view) |
| `PRIMARY KEY` backed by a `NULLS NOT DISTINCT` index (`ADD PRIMARY KEY USING INDEX`) | A plain unique index. 16 raises `primary keys cannot use NULLS NOT DISTINCT indexes` |
| `REINDEX DATABASE` / `reindexdb` rebuilding system catalog indexes | `REINDEX SYSTEM` / `reindexdb --system` for catalogs |
| PL/pgSQL bound cursor variable holding its own name before `OPEN` | The portal name is assigned at `OPEN` (`<unnamed portal 1>`). Assign the name before `OPEN` if code fetches the cursor by name (see [Functions](#functions-procedures-and-triggers)) |
| `CREATEROLE` roles changing any non-superuser role | Needs `ADMIN OPTION` on the target role; `CREATEDB`, `REPLICATION`, `BYPASSRLS` can be granted only by roles that have them |
| Role inheritance decided by the member's `INHERIT` attribute for all grants | Fixed per grant at `GRANT` time (`WITH INHERIT`); later `ALTER ROLE ... [NO]INHERIT` affects only future grants |
| Generated columns: parent and partitions with different generation status | All must agree; the expressions may now differ |
| Datetime input `'y2024m01d02'` (undocumented) and `epoch`/`infinity` combined with other fields | Rejected: `invalid input syntax for type date` |
| `pg_walinspect` `..._till_end_of_wal()` functions | `pg_get_wal_records_info()`/`pg_get_wal_stats()` with an end LSN beyond the end of WAL |
| psql `\du` column "Member of" | `\drg` |
| psql `\df+` showing source code | `\sf` |
| `postmaster` symlink, `postmaster -n`, Windows before 10 | `postgres` |
| `createuser --role` | `--member-of` (deprecated, still accepted) |

Already gone before 16, and still common in old code: exclusive backup mode `pg_start_backup()`/`pg_stop_backup()` (removed in 15; use `pg_backup_start()`/`pg_backup_stop()`), `pg_stat_statements.total_time` (renamed `total_exec_time` in 13), `CREATE` on `public` for all users (revoked by default for new databases since 15), PL/Python 2 (removed in 15), `recovery.conf` (removed in 12).

Still present in 16 but **removed in 17**, so new code should not start depending on them: `old_snapshot_threshold`, `db_user_namespace`, `trace_recovery_messages`, the `adminpack` extension, and the checkpoint and backend columns of `pg_stat_bgwriter`.

***

#### Features from PostgreSQL 14–15 You Can Rely On

All of these exist on 16 and should be used where they fit.[^8]

##### Since 14

- **SQL-standard function bodies**: `CREATE FUNCTION f(int) RETURNS int LANGUAGE sql BEGIN ATOMIC SELECT $1 + 1; END;`. The body is parsed at creation time and dependencies are tracked: `DROP TABLE logs` failed on 16.15 with `cannot drop table logs because other objects depend on it` (`function nlogs() depends on table logs`).
- **jsonb subscripting**: `(doc)['a']['b']`, and `UPDATE t SET doc['a'] = '1'`.
- **Multirange types** (`int4multirange`, `tstzmultirange`, ...) and `range_agg()`.
- `date_bin()`, `string_to_table()`, `trim_array()`, `bit_count()`, `split_part()` with negative positions, `GROUP BY DISTINCT`, the `SEARCH` and `CYCLE` clauses for recursive CTEs, `JOIN ... USING (...) AS alias`.
- `CREATE OR REPLACE TRIGGER`.
- `ALTER TABLE ... DETACH PARTITION ... CONCURRENTLY`.
- `idle_session_timeout`, `client_connection_check_interval`, `compute_query_id`.
- Predefined roles `pg_read_all_data`, `pg_write_all_data`, `pg_database_owner`.
- `password_encryption` defaults to `scram-sha-256`.
- **Behaviour change**: `EXTRACT()` returns `numeric` instead of `double precision` (`date_part()` still returns `double precision`).
- `lz4` TOAST compression (`default_toast_compression`, `ALTER TABLE ... SET COMPRESSION`), if the server was built with it.

##### Since 15

- **`MERGE`** (`WHEN [NOT] MATCHED [AND cond] THEN UPDATE | INSERT | DELETE | DO NOTHING`). No `RETURNING`, no `NOT MATCHED BY SOURCE`, no views on 16.
- **`UNIQUE NULLS NOT DISTINCT`**.
- **`security_invoker` views**: `CREATE VIEW v WITH (security_invoker = true) AS ...`.
- **`CREATE` on schema `public` is revoked from `PUBLIC`** in newly created databases (`public` is owned by `pg_database_owner`). Databases upgraded with `pg_upgrade` keep their old ACL.
- `regexp_count()`, `regexp_instr()`, `regexp_like()`, `regexp_substr()`.
- `ON DELETE SET NULL (col, ...)` / `SET DEFAULT (col, ...)` with column lists.
- `numeric` with negative scale or scale greater than precision.
- `log_destination = jsonlog`, `GRANT SET/ALTER SYSTEM ON PARAMETER`, the `pg_checkpoint` role.
- Logical replication row filters and column lists, `ALTER SUBSCRIPTION ... SKIP`.
- `pg_backup_start()`/`pg_backup_stop()` (non-exclusive backups only).

***

#### New in 16: SQL/JSON Constructors and IS JSON

16 adds the SQL-standard constructor syntax and the `IS JSON` predicate.[^9] The query functions (`JSON_VALUE`, `JSON_QUERY`, `JSON_EXISTS`, `JSON_TABLE`) are **not** in 16; they arrived in 17.

##### IS JSON

```sql
SELECT '{"a":1}'       IS JSON                  AS j,   -- t
       '{"a":1}'       IS JSON OBJECT           AS o,   -- t
       '[1]'           IS JSON ARRAY            AS a,   -- t
       '1'             IS JSON SCALAR           AS s,   -- t
       '{"a":1,"a":2}' IS JSON WITH UNIQUE KEYS AS u,   -- f
       'x'             IS JSON                  AS bad, -- f (no exception)
       NULL::text      IS JSON                  AS n;   -- NULL
```

- `IS JSON` accepts `text`, `json`, `jsonb` and `bytea` (UTF-8). Other types are an error: `123 IS JSON` gives `cannot use type integer in IS JSON predicate`.
- `'null' IS JSON SCALAR` is true and `'null' IS JSON OBJECT` is false. `IS JSON` alone accepts any JSON value, including a bare `null`, `42` or `"x"`. Use `IS JSON OBJECT` when the code expects an object.
- `WITH UNIQUE KEYS` only makes sense for `text`/`json` input. A `jsonb` value has already collapsed duplicates (`'{"a":1,"a":2}'::jsonb` is `{"a": 2}`), so `jsonb IS JSON WITH UNIQUE KEYS` is always true.
- It is a cheap way to validate raw JSON at the boundary. Measured:

```sql
CREATE TABLE raw(payload text CHECK (payload IS JSON OBJECT WITH UNIQUE KEYS));
INSERT INTO raw VALUES ('{"a":1}');          -- OK
INSERT INTO raw VALUES ('{"a":1,"a":2}');    -- ERROR: violates check constraint "raw_payload_check"
INSERT INTO raw VALUES ('[1]');              -- ERROR: violates check constraint
INSERT INTO raw VALUES ('not json');         -- ERROR: violates check constraint
```

On 15: `syntax error at or near "JSON"`.

##### JSON_OBJECT and JSON_ARRAY

```sql
SELECT JSON_OBJECT('a': 1, 'b': NULL);                 -- {"a" : 1, "b" : null}
SELECT JSON_OBJECT('a': 1, 'b': NULL ABSENT ON NULL);  -- {"a" : 1}
SELECT JSON_OBJECT('a' VALUE 1, 'a' VALUE 2);          -- {"a" : 1, "a" : 2}   duplicates kept (type json)
SELECT JSON_OBJECT('a' VALUE 1, 'a' VALUE 2 WITH UNIQUE KEYS);
-- ERROR:  duplicate JSON object key value: "a"
SELECT JSON_OBJECT('a': 1 RETURNING jsonb);            -- {"a": 1}  (type jsonb)
SELECT JSON_OBJECT(NULL: 1);                           -- ERROR: null value not allowed for object key

SELECT JSON_ARRAY(1, NULL, 'x');                       -- [1, "x"]          ABSENT ON NULL is the default
SELECT JSON_ARRAY(1, NULL, 'x' NULL ON NULL);          -- [1, null, "x"]
SELECT JSON_ARRAY(SELECT x FROM (VALUES (2),(1)) v(x) ORDER BY x);   -- [1, 2]
```

Behaviour verified on 16.15 that reviewers must know:

| Point | Detail |
|---|---|
| Different NULL defaults | `JSON_OBJECT` and `JSON_OBJECTAGG` default to **`NULL ON NULL`** (keep the key with `null`). `JSON_ARRAY` and `JSON_ARRAYAGG` default to **`ABSENT ON NULL`** (drop the element). An array built with `JSON_ARRAY(a, b, c)` silently changes length when one value is NULL, so positional consumers break. Write the clause explicitly. |
| Result type | Without `RETURNING`, the result is **`json`**, not `jsonb` (`pg_typeof` is `json`). Comparing, indexing or concatenating it with `jsonb` needs `RETURNING jsonb` or a cast. |
| Duplicate keys | `json` output keeps duplicates. With `RETURNING jsonb` the last value wins silently, unless `WITH UNIQUE KEYS` is given. |
| Strings that contain JSON | `JSON_OBJECT('j': '{"x":1}')` gives `{"j" : "{\"x\":1}"}` (an escaped string). Add `FORMAT JSON` to embed it: `JSON_OBJECT('j': '{"x":1}' FORMAT JSON)` gives `{"j" : {"x":1}}`. Values that are already `json`/`jsonb` are embedded as JSON. |
| Other SQL types | `numeric` keeps its scale (`1.50`), `date` becomes a JSON string, `boolean` becomes `true`/`false`. |

These constructors are equivalent in power to `json_build_object()`/`json_build_array()`/`jsonb_build_object()` that already existed. They are not faster. Prefer them for portability to other SQL-standard databases, or when `ABSENT ON NULL`/`WITH UNIQUE KEYS` are useful. Don't flag existing `jsonb_build_object()` code as outdated.

##### JSON_OBJECTAGG and JSON_ARRAYAGG

```sql
SELECT JSON_ARRAYAGG(x ORDER BY x) FROM (VALUES (2),(1),(NULL)) v(x);              -- [1, 2]
SELECT JSON_ARRAYAGG(x ORDER BY x NULL ON NULL) FROM (VALUES (2),(1),(NULL)) v(x); -- [1, 2, null]
SELECT JSON_OBJECTAGG(k: v) FROM (VALUES ('a',1),('b',NULL)) t(k,v);               -- { "a" : 1, "b" : null }
SELECT JSON_OBJECTAGG(k: v ABSENT ON NULL RETURNING jsonb)
FROM (VALUES ('a',1),('b',NULL)) t(k,v);                                           -- {"a": 1}
SELECT JSON_OBJECTAGG(k: v RETURNING jsonb) FROM (VALUES ('a',1),('a',2)) t(k,v); -- {"a": 2}  (silent)
SELECT JSON_OBJECTAGG(k: v WITH UNIQUE KEYS) FROM (VALUES ('a',1),('a',2)) t(k,v);
-- ERROR:  duplicate JSON object key value: "a"
SELECT JSON_ARRAYAGG(x) FROM (SELECT 1 WHERE false) v(x);                          -- NULL, not []
```

- Over zero rows the aggregates return **NULL**, not `[]`/`{}`, exactly like `json_agg()`. API code that returns "an empty list" needs `coalesce(JSON_ARRAYAGG(...), '[]')`.
- `json_agg()` keeps NULLs (`[null]`), while `JSON_ARRAYAGG()` drops them by default. Replacing one with the other changes results.
- Order inside the aggregate is unspecified without `ORDER BY` in the call.
- 16.11 fixed the planner treating JSON constructor expressions as strict (it could perform wrong query transformations when an input was NULL).[^10] Make sure the server is on a current minor.

On 15: `function json_array(integer) does not exist` and `syntax error at or near ":"`.

##### JSON on 16: what to use for extraction

16 has no `JSON_VALUE`/`JSON_TABLE`. Use the operators and the jsonpath functions:

```sql
SELECT (doc->>'qty')::int,                        -- raises on bad data: invalid input syntax for type integer
       doc #>> '{a,b}',
       jsonb_path_query_first(doc, '$.items[0].sku'),
       jsonb_path_exists(doc, '$.items[*] ? (@.qty > $min)', jsonb_build_object('min', 5))
FROM t;

-- Rows from an array (the 16 replacement for JSON_TABLE)
SELECT o.id, l.sku, l.qty
FROM orders o
CROSS JOIN LATERAL jsonb_to_recordset(o.doc->'lines') AS l(sku text, qty int);
```

Pass jsonpath variables through the `vars` argument (`jsonb_build_object('min', $1)`) instead of concatenating user input into the path text. Use `pg_input_is_valid(doc->>'qty', 'integer')` to test values before casting when bad data must be skipped rather than raise an error.

***

#### New in 16: Numeric Literals and Input

```sql
SELECT 0x1F, 0o17, 0b101, 1_000_000, 1_000.5;   -- 31 | 15 | 5 | 1000000 | 1000.5
SELECT 0xFFFFFFFFFFFFFFFFFF, pg_typeof(0xFFFFFFFFFFFFFFFFFF);  -- 4722366482869645213695 | numeric
SELECT 0x;        -- ERROR: invalid hexadecimal integer at or near "0x"
SELECT 1__000;    -- ERROR: trailing junk after numeric literal at or near "1__000"
```

On 15 all of these literals fail with `trailing junk after numeric literal`. On 14 and older they do **not** fail: `SELECT 1_000, 0x1F` parses as `SELECT 1 AS _000, 0 AS x1F` and silently returns `1 | 0` (15 started rejecting trailing junk after numbers).[^8] Code that must also run on 14 must not use these literals.

**Behaviour change for input strings (found by testing; easy to miss).** The integer types and `numeric` also accept the new forms when converting **text input**:

```sql
SELECT '0x10'::bigint, '0b11'::smallint, '1_000.5'::numeric, '0x10'::numeric, ' 0o17 '::int;
-- 16 | 3 | 1000.5 | 16 | 15
SELECT '1_000'::float8;   -- ERROR: invalid input syntax for type double precision (float types do not)
SELECT '_1000'::int;      -- ERROR (underscores only between digits)
```

On 15, `'0x1F'::int` and `'1_000'::int` raise `invalid input syntax for type integer`. Consequences for review:

- Validation that relies on the database cast rejecting anything but decimal digits (`$1::int` on user input, a `COPY` of external data, `pg_input_is_valid(x, 'int4')`) now **accepts** `0x1F`, `0b101`, `0o17` and `1_000`. If only decimal input is allowed, validate with a regular expression (`x ~ '^[+-]?[0-9]+$'`) or in the application.
- Data loaded on 16 may contain rows that the same pipeline rejected on 15 (for example `0x10` in a CSV column becomes 16).
- Client libraries are not affected: drivers that send integers as typed binary or decimal text behave as before.

jsonpath accepts the same literal forms: `jsonb_path_query('{"a":255}', '$.a ? (@ == 0xFF)')` returns `255` (a syntax error on 15).

***

#### New in 16: Other SQL Functions and Syntax

##### any_value

```sql
SELECT g, any_value(v) FROM (VALUES (1,'a'),(1,'b'),(2,NULL),(2,'c')) t(g, v)
GROUP BY g ORDER BY g;
-- 1 | a
-- 2 | c      <- NULL is skipped: any_value returns some non-NULL value, if there is one
```

`any_value()` replaces `min(col)`/`max(col)` hacks for columns that are functionally dependent on the group but not declared as such (or not provable to the planner). The value is arbitrary, so use it only when every row in the group has the same value, or when any value is acceptable. It is not a "first row" function. It does not run faster than `min()`, but it states the intent.

##### pg_input_is_valid and pg_input_error_info

```sql
SELECT pg_input_is_valid('42', 'integer'),          -- t
       pg_input_is_valid('4x', 'integer'),          -- f
       pg_input_is_valid('2024-02-30', 'date');     -- f
SELECT * FROM pg_input_error_info('12345', 'numeric(3,1)');
--  message: numeric field overflow | detail: A field with precision 3, scale 1 must round to an absolute value less than 10^2. | sql_error_code: 22003
```

They replace PL/pgSQL functions that cast inside `BEGIN ... EXCEPTION WHEN others` blocks. Those create a subtransaction per call, which is slow and consumes subtransaction IDs in bulk validations. The type name is a string, so the same function works for any type, including typmods (`'numeric(3,1)'`, `'varchar(10)'`). Remember the literal change above: `pg_input_is_valid('0x1F', 'int4')` is true on 16.

##### Time-zone-aware timestamptz arithmetic

```sql
SET timezone = 'UTC';
SELECT '2024-10-27 00:00+02'::timestamptz + interval '1 day';                         -- 2024-10-27 22:00:00+00 (wrong for Berlin)
SELECT date_add('2024-10-27 00:00+02'::timestamptz, interval '1 day', 'Europe/Berlin'); -- 2024-10-27 23:00:00+00 (= Berlin midnight)
SELECT date_subtract('2024-10-28 00:00 Europe/Berlin'::timestamptz, interval '1 day', 'Europe/Berlin');
SELECT g FROM generate_series('2024-10-26 00:00 Europe/Berlin'::timestamptz,
                              '2024-10-29 00:00 Europe/Berlin', '1 day', 'Europe/Berlin') g;
-- 2024-10-25 22:00:00+00, 2024-10-26 22:00:00+00, 2024-10-27 23:00:00+00, 2024-10-28 23:00:00+00
```

`timestamptz + interval '1 day'` uses the **session** `TimeZone` for the day arithmetic. A server or pool with `TimeZone = UTC` computes UTC days, which is off by an hour across DST changes for local business days. Code that computes calendar days for a user's zone should use `date_add()`/`date_subtract()`/the four-argument `generate_series()` with an explicit zone (16+), or `AT TIME ZONE` round trips.

`date_trunc(unit, timestamptz, zone)` is **`IMMUTABLE`** in 16 (it was `STABLE` in 15), so it can be indexed:

```sql
CREATE INDEX ox_dt ON ox (date_trunc('day', created_at, 'UTC'));
EXPLAIN (COSTS OFF) SELECT * FROM ox WHERE date_trunc('day', created_at, 'UTC') = '2024-02-01';
-- Bitmap Index Scan on ox_dt
--   Index Cond: (date_trunc('day'::text, created_at, 'UTC'::text) = '2024-02-01 00:00:00+00'::timestamp with time zone)
```

The two-argument `date_trunc('day', created_at)` on `timestamptz` still depends on the session time zone and cannot be indexed. A half-open range on the raw column (`created_at >= $1 AND created_at < $2`) is usually still the better query.

##### Smaller additions

```sql
SELECT * FROM (SELECT 1 AS x);                                -- alias no longer required (15: "subquery in FROM must have an alias")
SELECT array_sample(ARRAY[1,2,3,4,5], 3), array_shuffle(ARRAY[1,2,3]);
SELECT array_sample(ARRAY[1,2], 3);                           -- ERROR: sample size must be between 0 and 2
SELECT random_normal(), random_normal(10, 2);                 -- double precision
SELECT erf(0.5), erfc(0.5);
SELECT SYSTEM_USER;                                           -- scram-sha-256:alice ; NULL with trust auth
SELECT XMLSERIALIZE(DOCUMENT '<a><b>1</b></a>' AS text INDENT);
SELECT '+infinity'::timestamptz, pg_size_bytes('10 B');
```

- `random_normal()` uses the same generator as `random()`, so `setseed()` makes it repeatable. `array_sample()` and `array_shuffle()` do **not** follow `setseed()` on 16: after `SELECT setseed(0.5)` twice, `random_normal()` returned the same value both times while `array_shuffle()` returned different orders. Tests that seed and expect a fixed shuffle are flaky. None of these functions is cryptographically secure: use `gen_random_bytes()` (pgcrypto) or `gen_random_uuid()` for tokens.
- `SYSTEM_USER` reports the authenticated identity (for example the certificate CN or Kerberos principal) even when it differs from the database role. Use it in audit triggers instead of `session_user` when identity mapping is configured.
- Omitting a subquery alias is fine on 16, but code that must also run on 15 still needs it.

***

#### New in 16: Roles, CREATEROLE and Membership Options

This is the most important behaviour change in 16 for provisioning scripts, multi-tenant setups and admin tooling.[^1][^11]

##### CREATEROLE is restricted

Measured on 16.15 with a role `cr` that has `CREATEROLE`:

```sql
CREATE ROLE cr CREATEROLE LOGIN;
CREATE ROLE victim LOGIN;              -- created by the superuser
SET ROLE cr;
ALTER ROLE victim CREATEDB;
-- ERROR:  permission denied to alter role
-- DETAIL: Only roles with the CREATEROLE attribute and the ADMIN option on role "victim" may alter this role.
CREATE ROLE mine LOGIN;                -- OK: cr automatically gets ADMIN OPTION on roles it creates
ALTER ROLE mine CREATEDB;
-- DETAIL: Only roles with the CREATEDB attribute may change the CREATEDB attribute.
ALTER ROLE mine BYPASSRLS;
-- DETAIL: Only roles with the BYPASSRLS attribute may change the BYPASSRLS attribute.
ALTER ROLE mine PASSWORD 'x';          -- OK
GRANT victim TO mine;
-- ERROR:  permission denied to grant role "victim"
-- DETAIL: Only roles with the ADMIN option on role "victim" may grant this role.
```

- A `CREATEROLE` role can only alter, drop, rename or grant roles on which it has **`ADMIN OPTION`**. It gets that automatically for roles it creates (the grant is recorded in `pg_auth_members` with `admin_option = t` and the bootstrap superuser as grantor).
- It can hand out `CREATEDB`, `REPLICATION` and `BYPASSRLS` only if it has them itself.
- On 15, the same `cr` could change `victim` (`ALTER ROLE victim CREATEDB` succeeded) and could grant membership in almost any non-superuser role. Scripts that depended on that now fail with `permission denied`. Fix them by creating roles from the role that administers them, or by granting `ADMIN OPTION` explicitly (`GRANT victim TO cr WITH ADMIN TRUE`).
- The same applies to managed services that hand out a `CREATEROLE` "admin" role: it can manage the roles it created, not roles created by another role, unless it holds `ADMIN OPTION` on them.

##### Membership options: INHERIT, SET, ADMIN

```sql
GRANT app TO bob WITH INHERIT FALSE, SET FALSE;
-- as bob (a real login, not SET ROLE from a superuser session):
SELECT * FROM secret;  -- ERROR: permission denied for table secret  (privileges not inherited)
SET ROLE app;          -- ERROR: permission denied to set role "app"

GRANT app TO bob WITH INHERIT FALSE, SET TRUE;   -- after REVOKE app FROM bob
SET ROLE app;          -- OK: bob must switch explicitly to use app's privileges
```

| Option | Meaning | Default |
|---|---|---|
| `INHERIT` | Member automatically has the role's privileges | The member role's `INHERIT` attribute **at grant time** |
| `SET` | Member may `SET ROLE` to the role (and create objects owned by it with `ALTER ... OWNER TO`) | `TRUE` |
| `ADMIN` | Member may grant/revoke the role and (with `CREATEROLE`) alter it | `FALSE` |

- `pg_has_role('bob', 'app', 'USAGE')` is false with `INHERIT FALSE`, while `pg_has_role(..., 'MEMBER')` is true. Use the right check in code that tests permissions.
- `GRANT ... WITH ADMIN TRUE/FALSE` is the new spelling; `WITH ADMIN OPTION` still works. `GRANT ... GRANTED BY role` records another grantor, which must itself have `ADMIN OPTION`.
- Changing a role's `INHERIT` attribute with `ALTER ROLE` no longer changes existing memberships. It only sets the default for future grants.
- Dependencies are tracked: revoking `ADMIN OPTION` that was used to grant other memberships fails unless `CASCADE` is given.
- `ALTER DEFAULT PRIVILEGES FOR ROLE r` is only allowed for roles whose privileges the current user **inherits**.
- psql `\du` lost its "Member of" column. Use `\drg` to see grants with their options.
- `SET ROLE` checks the **session user's** membership. Testing membership rules with `SET ROLE bob; SET ROLE app;` from a superuser session proves nothing (the superuser may switch to any role). Test with a real login.

##### createrole_self_grant

```sql
ALTER ROLE admin_r SET createrole_self_grant = 'set, inherit';
-- roles that admin_r creates are now also granted to admin_r WITH INHERIT TRUE, SET TRUE
```

Measured: with `createrole_self_grant = 'set, inherit'`, creating `child` produced two `pg_auth_members` rows for `admin_r`: one with `admin_option = t` and one with `inherit_option = t, set_option = t`. The default is empty (`''`): the creator administers the role but does not inherit its privileges, which is the safer choice.

##### New predefined roles and reserved connections

- **`pg_create_subscription`**: lets a role run `CREATE SUBSCRIPTION`, but only **with `CREATE` privilege on the database** (measured: `permission denied for database postgres` without it). Non-superusers must put a password in the connection string (`ERROR: password is required`), and `password_required = false` is superuser-only.[^11]
- **`pg_use_reserved_connections`** with **`reserved_connections`** (default 0): when free slots fall to `superuser_reserved_connections + reserved_connections` or fewer, only superusers and members of this role can connect. Give it to monitoring and admin roles so they can still log in when the application exhausts `max_connections`.[^12]
- **`LOCK TABLE`** permissions are simpler: `UPDATE`, `DELETE` or `TRUNCATE` on the table permits every lock mode, `INSERT` permits `ROW EXCLUSIVE` and weaker modes, and `SELECT` permits `ACCESS SHARE`. Measured: a role with only `UPDATE` can now `LOCK TABLE lt IN ACCESS SHARE MODE` and also `IN ACCESS EXCLUSIVE MODE` (15: `permission denied for table lt` for `ACCESS SHARE`); a role with only `SELECT` still cannot take `ROW EXCLUSIVE`.
- `makeaclitem()` accepts a list (`'SELECT, UPDATE'`).

***

#### New in 16: Utility Commands

##### COPY ... DEFAULT

```sql
CREATE TABLE cd(a int, b text DEFAULT 'dflt');
COPY cd FROM STDIN WITH (FORMAT csv, DEFAULT '\D');
1,x
2,\D
\.
-- rows: (1, x), (2, dflt)
```

The marker must not collide with real data. Without the option, an absent value can only become NULL (or an empty string). On 15: `option "default" not recognized`. 16 has **no** `ON_ERROR` option; a single bad row aborts the whole `COPY`. For tolerant imports, `COPY` into a staging table of `text` columns and move valid rows with `pg_input_is_valid()`:

```sql
INSERT INTO target(a, b)
SELECT a::int, b::date FROM staging
WHERE pg_input_is_valid(a, 'int4') AND pg_input_is_valid(b, 'date');
```

Server-side `COPY ... FROM '/path'` reads files on the **database server** and needs superuser or `pg_read_server_files`. Application code should use `COPY ... FROM STDIN` (driver copy APIs) or psql's `\copy`. Flag server-side file paths and `COPY ... TO PROGRAM` in application code as security problems.

##### VACUUM options

```sql
VACUUM (BUFFER_USAGE_LIMIT '256MB') big_table;  -- larger ring: faster manual vacuum, more cache pressure
VACUUM (PROCESS_MAIN false) t;                   -- TOAST table only
VACUUM (SKIP_DATABASE_STATS) t1; VACUUM (SKIP_DATABASE_STATS) t2; VACUUM (ONLY_DATABASE_STATS);
```

`vacuum_buffer_usage_limit` defaults to **256kB** on 16 (17 raised it to 2MB). A value of `0` disables the ring (vacuum may use all of `shared_buffers`). For a one-off manual vacuum of a huge table in a maintenance window a larger value helps. Don't raise it globally without measuring cache effects. All four options fail on 15 with `unrecognized VACUUM option`.

##### EXPLAIN (GENERIC_PLAN)

```sql
EXPLAIN (GENERIC_PLAN, COSTS OFF) SELECT * FROM cd WHERE a = $1;
-- Seq Scan on cd
--   Filter: (a = $1)
EXPLAIN (GENERIC_PLAN, ANALYZE) SELECT * FROM cd WHERE a = $1;
-- ERROR:  EXPLAIN options ANALYZE and GENERIC_PLAN cannot be used together
```

Use it to explain statements copied from `pg_stat_statements` or application logs, which contain `$n` placeholders. It shows the plan the server would cache after switching to a generic plan, which is exactly the plan that hurts with skewed data (see [Prepared statements](#prepared-statements-and-generic-plans)). On 15: `there is no parameter $1`.

##### Other DDL changes

```sql
CREATE TABLE stor(a text STORAGE EXTERNAL);   -- 16: STORAGE in CREATE TABLE
CREATE STATISTICS ON a, b FROM cd;            -- 16: name generated (cd_a_b_stat)
REINDEX DATABASE;                             -- 16: current database, user indexes only

CREATE TABLE pkn2(a int NOT NULL);
CREATE UNIQUE INDEX pkn2_i ON pkn2(a) NULLS NOT DISTINCT;
ALTER TABLE pkn2 ADD PRIMARY KEY USING INDEX pkn2_i;
-- 16: ERROR: primary keys cannot use NULLS NOT DISTINCT indexes   (15 accepted it)

CREATE TABLE rv(a int);
CREATE RULE "_RETURN" AS ON SELECT TO rv DO INSTEAD SELECT 1 AS a;
-- 16: ERROR: relation "rv" cannot have ON SELECT rules             (15 converted rv into a view)
```

- **`REINDEX DATABASE` no longer reindexes system catalogs.** Runbooks that use it to repair catalog index corruption must run `REINDEX SYSTEM` as well (`reindexdb --system`).
- Old dump files or migration tools that create views with `CREATE TABLE` + `CREATE RULE "_RETURN"` (some very old `pg_dump` output does this for circular view dependencies) fail on 16. Use `CREATE VIEW`.

***

#### New in 16: Planner and Performance Changes Visible in Plans

The plans below are from 16.15 with `EXPLAIN (COSTS OFF)`, compared with 15.19 on the same data.

##### Parallel FULL and right outer hash joins

```sql
SET parallel_setup_cost = 0; SET parallel_tuple_cost = 0; SET min_parallel_table_scan_size = 0;
EXPLAIN (COSTS OFF) SELECT count(*) FROM a1 FULL JOIN b1 ON a1.id = b1.id;
-- 16:  Finalize Aggregate
--        ->  Gather
--              ->  Partial Aggregate
--                    ->  Parallel Hash Full Join
--                          Hash Cond: (a1.id = b1.id)
-- 15:  Aggregate
--        ->  Hash Full Join            (serial join; only the scans below are parallel)
```

Reconciliation queries (`FULL JOIN` of two snapshots) and anti-join-like reports scale with workers on 16.

##### Incremental sort for DISTINCT and pre-sorted aggregates

```sql
-- index on ds(a); 200,000 rows
SET enable_hashagg = off;
EXPLAIN (COSTS OFF) SELECT DISTINCT a, b FROM ds;
-- 16:  Unique -> Incremental Sort (Sort Key: a, b; Presorted Key: a) -> Index Scan using ds_a_idx
-- 15:  Unique -> Sort (Sort Key: a, b) -> Seq Scan on ds

EXPLAIN (COSTS OFF) SELECT a, array_agg(b ORDER BY b) FROM ds GROUP BY a;
-- 16:  GroupAggregate (Group Key: a) -> Incremental Sort (Sort Key: a, b; Presorted Key: a) -> Index Scan
-- 15:  GroupAggregate (Group Key: a) -> Index Scan using ds_a_idx    (each group is sorted inside the aggregate)
```

`enable_presorted_aggregate` (default `on`) can switch the second change off if a plan regresses. Ordered aggregates such as `string_agg(x, ',' ORDER BY x)` now show an explicit `Sort` below the `Aggregate` in `EXPLAIN`, so the sort cost is visible.

##### Parallel string_agg and array_agg

`SELECT string_agg(status, ',') FROM orders` and `array_agg(id)` can now be planned as `Finalize Aggregate -> Gather -> Partial Aggregate -> Parallel Seq Scan`. Without `ORDER BY` in the aggregate, the element order was never guaranteed, and with parallel workers it can change between runs. Code that relies on the "natural" order of `string_agg`/`array_agg` output is a bug. Put `ORDER BY` inside the call.

##### Other planner changes

- `Hash Anti Join` can put the non-nullable side inside, which helps `NOT EXISTS` against large tables.
- `ntile()`, `cume_dist()` and `percent_rank()` get a `Run Condition` (measured: `WindowAgg  Run Condition: (ntile(4) OVER (?) <= 1)`), so "top quartile" filters stop early, like `row_number()` and `rank()` since 15.
- `Memoize` can sit above `UNION ALL` subqueries.

##### What 16 does not do (17 and 18 do)

Measured on 16.15:

```sql
-- B-tree IN list: one descent per value
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF, SUMMARY OFF)
SELECT count(*) FROM orders WHERE id IN (1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20);
-- Index Only Scan using orders_pkey ... Buffers: shared hit=64      (17: 4 for the same query)

-- IS NULL on a NOT NULL column is not removed
EXPLAIN (COSTS OFF) SELECT * FROM nn WHERE a IS NULL;   -- Seq Scan on nn  Filter: (a IS NULL)

-- Correlated IN (subquery) is not turned into a semi-join
EXPLAIN (COSTS OFF) SELECT * FROM cust c WHERE c.id IN (SELECT o.customer_id FROM ord o WHERE o.total > c.region);
-- Seq Scan on cust c  Filter: (SubPlan 1)

-- No skip scan: an index on (a, b) does not help WHERE b = ?
EXPLAIN (COSTS OFF) SELECT * FROM sk WHERE b = 500;     -- Seq Scan on sk  Filter: (b = 500)
```

- Correlated `IN (subquery)` with a reference to the outer query runs the subplan once per outer row on 16. Rewrite it as `EXISTS (SELECT 1 FROM ord o WHERE o.customer_id = c.id AND o.total > c.region)`, which 16 plans as a semi-join.
- A multicolumn B-tree index is only efficient when the query constrains its **leading** column(s). Don't accept "the composite index covers it" for a query that filters only on a later column. Add a leading-column constraint (`a = ANY(...)`) or another index.
- Large `IN` lists cost one index descent per value. Still prefer `= ANY($1::bigint[])` with an array parameter over generating `IN ($1, ..., $n)`: one prepared statement for every list length, and an empty array works (`= ANY('{}')` matches nothing), while an empty `IN ()` is a syntax error.

##### HOT updates and freezing

- An `UPDATE` that changes only columns covered by BRIN indexes can be a HOT update on 16 (no new index entries in B-tree indexes). Other indexed columns still prevent HOT.
- Vacuum freezes pages opportunistically, which reduces the size of later anti-wraparound vacuums on append-mostly tables.

***

#### New in 16: Monitoring

```sql
-- I/O by backend type, object and context
SELECT backend_type, object, context, reads, writes, extends, hits, evictions, reuses, fsyncs
FROM pg_stat_io
WHERE backend_type = 'client backend' AND object = 'relation';
-- contexts: normal, vacuum, bulkread, bulkwrite

-- When was a table last scanned?
SELECT relname, seq_scan, last_seq_scan, idx_scan, last_idx_scan, n_tup_newpage_upd
FROM pg_stat_user_tables ORDER BY seq_tup_read DESC LIMIT 10;

SELECT indexrelname, idx_scan, last_idx_scan FROM pg_stat_user_indexes ORDER BY last_idx_scan NULLS FIRST;
```

- **`pg_stat_io`** has one row per backend type, object (`relation`, `temp relation`) and context. Columns include `reads`, `read_time`, `writes`, `write_time`, `writebacks`, `extends`, `hits`, `evictions`, `reuses`, `fsyncs` and `op_bytes`. `client backend` rows with many `writes` in the `normal` context mean backends are writing dirty buffers themselves: `shared_buffers` or the background writer may be undersized. Timing columns stay 0 unless `track_io_timing = on`.
- **`last_seq_scan` / `last_idx_scan`** (timestamps) make "unused index" checks much more reliable than `idx_scan = 0`: an index whose `last_idx_scan` is months old, on the primary **and** on every replica, is a removal candidate. They are NULL until the first scan after the statistics were reset or the server was upgraded.
- **`n_tup_newpage_upd`** counts updates whose new row version went to another page (not HOT). High values relative to `n_tup_hot_upd` suggest a lower `fillfactor`.
- `pg_stat_statements` on 16 is version **1.10**. Its I/O timing columns are still `blk_read_time`/`blk_write_time` (renamed in 17), plus `temp_blk_read_time`/`temp_blk_write_time` and the JIT columns. Constants in utility statements are now normalized to `$n`. Measured: `SELECT shared_blk_read_time FROM pg_stat_statements` fails on 16 with `column "shared_blk_read_time" does not exist`.
- `pg_stat_bgwriter` still has `checkpoints_timed`, `checkpoints_req`, `buffers_backend` ... on 16. These columns move to `pg_stat_checkpointer` or are removed in 17. Write monitoring that must work on both versions with a version check (`current_setting('server_version_num')::int >= 170000`).
- `log_checkpoints` messages include the checkpoint and REDO LSN; `pg_locks` shows speculative insertion locks of `INSERT ... ON CONFLICT`.

***

#### New in 16: Logical Replication

| Change | Review notes |
|---|---|
| Logical decoding on **standbys**[^24] | A standby can be a publisher (create the slot on the standby; `pg_log_standby_snapshot()` on the primary speeds slot creation). Needs `wal_level = logical` on the primary, `hot_standby_feedback = on` on the standby and, as the documentation recommends, a physical replication slot between primary and standby; otherwise catalog rows can be removed and the logical slot is invalidated. Slots are **not** synchronized across failover in 16 (that is 17). |
| `CREATE SUBSCRIPTION ... WITH (streaming = parallel)` | Large in-progress transactions are applied by parallel workers (`max_parallel_apply_workers_per_subscription`, default 2). Apply order of separate transactions is unchanged. |
| `origin = none` | The subscriber ignores changes that came from another replication origin. Use it for bidirectional setups to avoid loops; it does not resolve conflicts. |
| Apply runs **as the table owner** | The subscription owner must be a superuser or be able to `SET ROLE` to every table owner. `run_as_owner = true` restores the old behaviour (apply as the subscription owner) and is less safe: table owners can then run code (triggers) as the subscription owner. |
| `binary = true` initial sync | The initial table copy also uses binary format (faster). Every column type needs binary send/receive functions, and types must match on both sides. |
| `REPLICA IDENTITY FULL` uses B-tree indexes on the subscriber | Tables without a primary key no longer force a sequential scan per change, if a suitable index exists. Still prefer a primary key. |
| `password_required = false` | Superuser-only. Non-superuser subscription owners must supply a password in the connection string. |

16.14 fixed SQL injection through unquoted schema and table names in `ALTER SUBSCRIPTION ... REFRESH PUBLICATION` (CVE-2026-6638), and 16.15 restricts logical decoding output plugins to `output_plugin_libraries` (default `pgoutput, test_decoding`; CVE-2026-6471).[^2][^10] After updating to 16.15, add any third-party plugin (for example `wal2json`) to that setting or decoding stops.

***

#### New in 16: Authentication, Connections and libpq

Sources: libpq connection parameters[^22] and `pg_hba.conf`[^23].

```text
# pg_hba.conf (16): regular expressions start with a slash; include files
host   all   /^app_.*$      10.0.0.0/8    scram-sha-256
host   "/literal_db_name" all 10.0.0.0/8  scram-sha-256     # quote names that start with /
include_dir conf.d
```

```text
# libpq 16 connection strings
postgresql://app@db1,db2,db3/app?load_balance_hosts=random&target_session_attrs=read-only
postgresql://app@db/app?require_auth=scram-sha-256&sslmode=verify-full
postgresql://app@db/app?sslrootcert=system        # OS trust store; implies sslmode=verify-full
```

- **`require_auth`** makes the client refuse a server that does not use the listed method. Measured against a `trust` server: `authentication method requirement "scram-sha-256" failed: server did not complete authentication`. It protects clients against a spoofed server that skips authentication or asks for a clear-text password (`require_auth=!password`). Recommend it for services with stored credentials.
- **`load_balance_hosts=random`** shuffles hosts and addresses (also several A records). It spreads connections, but it is not a health-aware load balancer, and a pool still needs `target_session_attrs` to avoid writing to a standby.
- **`sslrootcert=system`** trusts the operating system's CA store and forces `verify-full`. Use it with managed services that present publicly trusted certificates. A weaker `sslmode` together with it is rejected: `weak sslmode "require" may not be used with sslrootcert=system (use "verify-full")`.
- **`sslcertmode`** (`disable`, `allow`, `require`) controls whether the client certificate is sent.
- **`scram_iterations`** (default 4096) sets the SCRAM iteration count for new password hashes. Higher values slow brute force on stolen hashes and every connection attempt.
- Regular expressions in `pg_hba.conf` match database and role names. Anchor them (`^...$`). An unanchored `/app` matches any name containing `app`.
- psql `\bind` sends parameters with the extended protocol: `SELECT $1::int + 1 \bind 41 \g` returns `42`. Useful for reproducing driver behaviour (generic plans, parameter types).

***

#### Schema Migrations and Locking

This is the most important review area for production PostgreSQL. A migration that is correct but takes the wrong lock can take an application down.

##### Lock levels of common DDL (measured on 16.15 with pg_locks)

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
| `ADD CONSTRAINT ... FOREIGN KEY ... NOT VALID` | `SHARE ROW EXCLUSIVE` on **both** tables | no | yes (briefly) | none |
| `VALIDATE CONSTRAINT` (foreign key) | `SHARE UPDATE EXCLUSIVE` (and `ROW SHARE` on the referenced table) | no | no | full scan |
| `ALTER COLUMN SET STATISTICS` | `SHARE UPDATE EXCLUSIVE` | no | no | none |
| `DROP COLUMN` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none (metadata only) |
| `CREATE INDEX` | `SHARE` | no | **yes**, for the whole build | full scan |
| `CREATE INDEX CONCURRENTLY` | `SHARE UPDATE EXCLUSIVE` | no | no | two scans and waits for old transactions |
| `ATTACH PARTITION` | `SHARE UPDATE EXCLUSIVE` on parent, `ACCESS EXCLUSIVE` on the attached table and on the **default partition** | parent: no | parent: no | scans the new partition unless a matching `CHECK` exists. Also scans the default partition |
| `DETACH PARTITION ... CONCURRENTLY` (14+) | `SHARE UPDATE EXCLUSIVE` | no | no | not allowed inside a transaction block, or when a default partition exists |
| `VACUUM FULL`, `CLUSTER` | `ACCESS EXCLUSIVE` | yes | yes | rewrite |
| plain `REINDEX INDEX` | `SHARE` on the table, `ACCESS EXCLUSIVE` on the index | queries that would use the index block | yes | rebuilds the index |

The rewrite facts above were measured by comparing `pg_relation_filenode()` before and after each statement inside a rolled-back transaction, and the lock modes by reading `pg_locks` for the session before `ROLLBACK`. Source for lock modes: the "Explicit Locking" and `ALTER TABLE` documentation.[^13][^14]

##### The lock queue: why "fast" DDL still causes outages

Even a metadata-only `ALTER TABLE` needs `ACCESS EXCLUSIVE`. It waits behind any transaction that has touched the table, even one holding only an `ACCESS SHARE` lock from a `SELECT`. **While it waits, every new query on the table queues behind it.** Measured on 16.15: with session R holding a read transaction and session A waiting on `ALTER TABLE q ADD COLUMN x int`, a plain `SELECT count(*) FROM q` in a third session blocked until its `lock_timeout` fired (`canceling statement due to lock timeout`).

Every migration session must set a short `lock_timeout` and retry:

```sql
-- BAD: can wait forever and block all traffic on the table meanwhile
ALTER TABLE orders ADD COLUMN note text;

-- GOOD: fail fast, retry from the migration tool with backoff
SET lock_timeout = '3s';
SET statement_timeout = '15min';   -- upper bound for scans, validations, etc.
ALTER TABLE orders ADD COLUMN note text;
```

`SET LOCAL` limits the setting to the current transaction. It was verified that `lock_timeout` reverted to `0` after `COMMIT`. Migration frameworks that wrap each migration in a transaction should use `SET LOCAL`, so the setting does not leak into a pooled connection. There is no `transaction_timeout` on 16 (17), so bound long migrations with `statement_timeout`.

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

Note: `DEFAULT now()` gives every *existing* row the same timestamp (the time of the migration). That is usually wrong for "created_at"-style columns. Backfill from real data.

##### Adding NOT NULL without a long ACCESS EXCLUSIVE scan

```sql
SET lock_timeout = '3s';
ALTER TABLE t ADD CONSTRAINT t_n_nn CHECK (n IS NOT NULL) NOT VALID;  -- instant
ALTER TABLE t VALIDATE CONSTRAINT t_n_nn;  -- scans under SHARE UPDATE EXCLUSIVE; writes continue
ALTER TABLE t ALTER COLUMN n SET NOT NULL; -- no scan: proven by the valid CHECK
ALTER TABLE t DROP CONSTRAINT t_n_nn;      -- optional cleanup
```

With `client_min_messages = debug1`, 16.15 confirms the skip: `existing constraints on column "t.n" are sufficient to prove that it does not contain nulls`. (18 adds `NOT NULL ... NOT VALID` constraints; do not suggest that for 16.)

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
- A `USING` clause does not force a rewrite by itself: measured on 16.15, `TYPE text USING v::text` (from `varchar`) and `TYPE int USING n` kept the relfilenode. Any `USING` expression that actually changes the stored value, or a type change that is not binary-coercible, rewrites the table and its indexes.
- Generated columns cannot be changed in place on 16 (`SET EXPRESSION` is 17). Dropping and re-adding a `STORED` generated column rewrites the table.

##### Indexes in migrations

```sql
-- BAD on a live table: blocks INSERT/UPDATE/DELETE for the whole build
CREATE INDEX orders_status_idx ON orders(status);

-- GOOD
CREATE INDEX CONCURRENTLY orders_status_idx ON orders(status);
```

`CREATE INDEX CONCURRENTLY` rules that reviewers must enforce (all verified on 16.15):

- It **cannot run inside a transaction block**: `ERROR: CREATE INDEX CONCURRENTLY cannot run inside a transaction block`. Migration tools that wrap migrations in a transaction need a per-migration opt-out (for example a "no transaction" flag).
- A failure (deadlock, unique violation, cancellation) leaves an **INVALID index** behind. It still slows down writes and is never used for reads. **`CREATE INDEX CONCURRENTLY IF NOT EXISTS` then skips silently** (`NOTICE: relation "dup_a" already exists, skipping`) and leaves the invalid index in place. Idempotent migrations must check `pg_index.indisvalid` and `DROP INDEX CONCURRENTLY` before retrying.
- It cannot be used on a partitioned parent (`cannot create index on partitioned table "m" concurrently`). Pattern: `CREATE INDEX idx ON ONLY parent (...)` (an invalid shell index), then `CREATE INDEX CONCURRENTLY` on each partition and `ALTER INDEX idx ATTACH PARTITION part_idx`. The parent index became valid when all partitions were attached.
- It waits for all transactions that could use the old snapshot. A long-running transaction stalls it.
- `DROP INDEX CONCURRENTLY` and `REINDEX ... CONCURRENTLY` (12+) are the non-blocking counterparts.
- Add a primary key or unique constraint without a long lock by building the unique index concurrently and then attaching it: `ALTER TABLE pkt ADD CONSTRAINT pkt_pkey PRIMARY KEY USING INDEX pkt_id;`. If a key column is still nullable, the command sets `NOT NULL` itself, which scans the whole table under `ACCESS EXCLUSIVE` (16.15 logs `verifying table "pkt"` at `debug1`). Make the columns `NOT NULL` first with the `CHECK ... NOT VALID` pattern above. The index is renamed to the constraint name. On 16 the index must not be `NULLS NOT DISTINCT`.

```sql
-- Find invalid indexes left by failed concurrent builds
SELECT indexrelid::regclass, indrelid::regclass
FROM pg_index WHERE NOT indisvalid;
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

- Renaming or dropping a column or table that running application code still uses causes errors. Use expand/contract: add the new column, deploy code that writes both, backfill, switch reads, and drop later.
- Avoid `VACUUM FULL`/`CLUSTER` on live tables. They hold `ACCESS EXCLUSIVE` for the whole rewrite. Use `pg_repack` or `pg_squeeze` if bloat really requires it.
- **Enums**: `ALTER TYPE ... ADD VALUE` is transactional since 12 (a `ROLLBACK` removed the new value on 16.15), but on 16 the new value **cannot be used in the same transaction**, even when the type itself was created in that transaction: `unsafe use of new value "b" of enum type mood2` (`HINT: New enum values must be committed before they can be used.`). A migration that creates an enum type, extends it and inserts rows using the new value must be split into two transactions on 16 (17 relaxes the case of a type created in the same transaction). Removing or renaming enum values needs a new type. Consider a lookup table or `text` with a `CHECK` constraint for value sets that change often.
- `REFRESH MATERIALIZED VIEW` holds `ACCESS EXCLUSIVE` and blocks readers. `REFRESH ... CONCURRENTLY` needs a unique index on the view (a plain unique index, without a `WHERE` clause).
- Changing a function used by an index (`CREATE OR REPLACE FUNCTION`) does not rebuild the index. If the results change, the index is silently wrong until `REINDEX`.
- `REINDEX DATABASE` on 16 skips system catalogs (see [Utility Commands](#new-in-16-utility-commands)).

***

#### Functions in Indexes, Constraints and Materialized Views: search_path

On 16, `CREATE INDEX`, `REINDEX`, `ANALYZE`, `VACUUM`, `CLUSTER` and `REFRESH MATERIALIZED VIEW` evaluate functions with the **caller's** `search_path`. A function that uses unqualified table names therefore works in day-to-day use, and fails as soon as it runs with a different `search_path`:

```sql
CREATE TABLE lookup(k int PRIMARY KEY, v int);
CREATE FUNCTION f_lookup(int) RETURNS int LANGUAGE sql IMMUTABLE
  AS 'SELECT v FROM lookup WHERE k = $1';
CREATE TABLE mt(a int);
CREATE INDEX mt_f ON mt (f_lookup(a));            -- 16: OK with search_path = "$user", public
CREATE MATERIALIZED VIEW mv AS SELECT f_lookup(a) FROM mt;

SET search_path = '';                             -- what pg_dump/pg_restore scripts do
REINDEX TABLE public.mt;                          -- ERROR: relation "lookup" does not exist
ANALYZE public.mt;                                -- same error
REFRESH MATERIALIZED VIEW public.mv;              -- same error
```

Measured consequences on 16.15:

- **Restoring a `pg_dump` fails.** Dump scripts run `SELECT pg_catalog.set_config('search_path', '', false)`. Restoring the dump above into a new database failed twice with `relation "lookup" does not exist` (index creation and `REFRESH MATERIALIZED VIEW`): after the restore the index did not exist and the materialized view was unpopulated (`pg_matviews.ispopulated = f`). The restore only reports errors; psql without `ON_ERROR_STOP` continues.
- Jobs that connect with a different `search_path` (schema-per-tenant setups, `ALTER ROLE ... SET search_path`) fail the same way in `ANALYZE` or `REINDEX`.
- **On 17 the same `CREATE INDEX` fails immediately**, because 17 runs these maintenance commands with `search_path = pg_catalog, pg_temp` (verified on 17.11). An upgrade from 16 then breaks `ANALYZE`, autovacuum's auto-analyze, `REINDEX` and `REFRESH` for such objects.

Fix on 16 (and it stays correct on 17): pin the `search_path` on the function or schema-qualify every reference.

```sql
CREATE FUNCTION f_lookup2(int) RETURNS int LANGUAGE sql IMMUTABLE
  SET search_path = public
  AS 'SELECT v FROM lookup WHERE k = $1';
-- or a SQL-standard body, which resolves names at creation time:
CREATE FUNCTION f_lookup3(k int) RETURNS int LANGUAGE sql STABLE
  BEGIN ATOMIC SELECT v FROM public.lookup WHERE lookup.k = f_lookup3.k; END;
```

Review every function used in an expression index, an index predicate, a `CHECK` constraint, a generated column or a materialized view. (The example also lies about being `IMMUTABLE`, because it reads a table: see [Functions](#functions-procedures-and-triggers). A `SET search_path` clause also prevents the function from being inlined.)

***

#### Indexing

##### Index types

| Type | Use for | Notes |
|---|---|---|
| B-tree (default) | `=`, `<`, `>`, `BETWEEN`, `IN`/`= ANY`, `ORDER BY`, `IS NULL`, prefix `LIKE 'x%'` (with `C` collation or `text_pattern_ops`) | Only efficient when the leading columns are constrained (no skip scan in 16) |
| Hash | `=` only | WAL-logged and crash-safe since 10. Rarely better than B-tree |
| GIN | `jsonb` (`@>`, `?`, `?|`, `?&`, `@?`, `@@`), arrays (`@>`, `&&`), full-text `tsvector`, `pg_trgm` (`LIKE '%x%'`, `ILIKE`, `~`) | Slower writes; `fastupdate` pending list; serial build only |
| GiST | ranges, geometry, exclusion constraints, `pg_trgm`, nearest-neighbour `ORDER BY <->` | |
| SP-GiST | non-balanced data (IP prefixes, quadtrees, text prefixes) | |
| BRIN | very large, append-only tables where the column correlates with physical order (timestamps) | Tiny. Serial build on 16 (parallel in 17). Updates of only BRIN-indexed columns can be HOT on 16 |

```sql
CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_orders_user_status ON orders (user_id, status);           -- order matters
CREATE INDEX idx_active_users ON users (email) WHERE status = 'active';     -- partial
CREATE UNIQUE INDEX idx_users_email_lower ON users (lower(email));         -- expression
CREATE INDEX idx_users_email_cov ON users (email) INCLUDE (name, created_at); -- covering (11+)
CREATE INDEX idx_posts_fts ON posts USING gin (to_tsvector('english', title || ' ' || body));
CREATE INDEX idx_events_doc ON events USING gin (doc jsonb_path_ops);
CREATE INDEX idx_logs_at_brin ON logs USING brin (at);
```

##### Rules for reviewing indexes

- **The predicate must match the index expression exactly.** Measured on 16.15: `WHERE date_trunc('day', created_at) = '2024-02-01'` and `WHERE customer_id::text = '42'` both led to a sequential scan even with an index on the column. Rewrite as a range (`created_at >= '2024-02-01' AND created_at < '2024-02-02'`) or compare with the column's own type.
- **Type mismatches defeat indexes.** `WHERE id = 42.0` on a `bigint` column is planned as `(id)::numeric = 42.0`, a sequential scan. Drivers that send parameters as `numeric` or `text` cause the same problem. Bind parameters with the column's type.
- **Expression indexes need IMMUTABLE functions.** `CREATE INDEX ON ox (date(created_at))` on a `timestamptz` column fails with `functions in index expression must be marked IMMUTABLE`, because the date depends on the session time zone. Use `((created_at AT TIME ZONE 'UTC')::date)` or, new in 16, `date_trunc('day', created_at, 'UTC')`, and query with exactly that expression. The one-argument `to_tsvector(t)` is not immutable either (it depends on `default_text_search_config`); use `to_tsvector('english', t)`.
- **Prefix `LIKE` and collations.** With the default `en_US.utf8` database collation, `email LIKE 'user12%'` did not use a plain B-tree index (seq scan). A `text_pattern_ops` index gives `Index Cond: ((email ~>=~ 'user12'::text) AND (email ~<~ 'user13'::text))`; a column declared `COLLATE "C"` with a plain index gives `Index Cond: ((email >= 'user12'::text) AND (email < 'user13'::text))`. (The builtin `C.UTF-8` provider is 17.) Leading-wildcard `LIKE '%x'` needs `pg_trgm` (GIN or GiST).
- **Escape `LIKE` input.** `_` and `%` in user input are wildcards: `'abc' LIKE 'a_c'` is true. Escape user input (`replace(replace(replace(x, '\', '\\'), '%', '\%'), '_', '\_')`) or use `starts_with()` / `left(col, n) = $1` when only a prefix test is needed.
- **Every foreign key column used for joins or cascades needs an index** (PostgreSQL never creates one automatically).
- **Partial indexes** must have a predicate that the query implies literally (`WHERE active` in both). For `ON CONFLICT` on a partial unique index, the statement must repeat the predicate: `ON CONFLICT (user_id) WHERE active DO NOTHING`. Without it: `there is no unique or exclusion constraint matching the ON CONFLICT specification`.
- **Multicolumn order**: put equality columns first, then the range or sort column. `(status, created_at)` serves `WHERE status = $1 ORDER BY created_at DESC LIMIT 20`.
- **Over-indexing**: each index slows writes, prevents HOT updates on its columns, and adds vacuum and WAL work. On 16, use `pg_stat_user_indexes.last_idx_scan` (new) together with `idx_scan` to find unused indexes, on the primary and on every replica, and look for duplicate prefixes (`(a)` next to `(a, b)`).
- **Unique constraints and NULLs**: `UNIQUE (a, b)` allows any number of rows with `b IS NULL` (two `(1, NULL)` rows were inserted). Use `UNIQUE NULLS NOT DISTINCT` (15+) when NULL should count as a value. Such an index cannot back a primary key (16).
- **Case-insensitive uniqueness**: `CREATE UNIQUE INDEX ... (lower(email))` plus `ON CONFLICT (lower(email))`. Queries must use `lower(email) = lower($1)`. A nondeterministic ICU collation also works for equality, but `LIKE` on it fails on 16: `nondeterministic collations are not supported for LIKE`.

***

#### Query Performance and EXPLAIN

##### Reading plans

```sql
EXPLAIN SELECT ...;                                   -- estimated plan, does not run
EXPLAIN (ANALYZE, BUFFERS) SELECT ...;                -- runs the query; real rows, timing, buffers
EXPLAIN (ANALYZE, BUFFERS, VERBOSE, SETTINGS) SELECT ...;
EXPLAIN (GENERIC_PLAN) SELECT ... WHERE id = $1;      -- 16: plan for a parameterized query
```

- On 16, `BUFFERS` must be requested explicitly with `ANALYZE` (18 adds it automatically). `SERIALIZE` and `MEMORY` are 17.
- **`EXPLAIN ANALYZE` executes the statement.** `EXPLAIN ANALYZE DELETE FROM ex` really deleted the row (the count afterwards was 0). Wrap DML in `BEGIN; EXPLAIN ANALYZE ...; ROLLBACK;`.
- Compare estimated `rows=` with `actual rows=`. Errors of 10x or more point to stale or insufficient statistics, correlated columns (`CREATE STATISTICS ... (dependencies, ndistinct, mcv)`) or non-sargable predicates.
- Node names: `Seq Scan`, `Index Scan`, `Index Only Scan` (check `Heap Fetches:`, which should be near 0 after vacuum), `Bitmap Heap Scan`, `Nested Loop` (good for few outer rows), `Hash Join`, `Merge Join`, `Memoize` (14+), `Incremental Sort` (13+, used for `DISTINCT` and ordered aggregates since 16), `Gather`/`Parallel ...` (`Parallel Hash Full Join` since 16).
- A `Seq Scan` is not a defect in itself. On small tables, or when most rows are returned, it is the right plan.

##### Pagination

```sql
-- BAD on large offsets: reads and discards every skipped row
SELECT id FROM ofs ORDER BY id LIMIT 20 OFFSET 400000;
-- Index Only Scan using ofs_pkey ... (actual rows=400020 loops=1)   <- 400k rows read to return 20

-- GOOD: keyset pagination with a row-value comparison and a matching composite index
CREATE INDEX ord_created_id ON ordp (created_at, id);
SELECT id, created_at FROM ordp
WHERE (created_at, id) < ($1, $2)
ORDER BY created_at DESC, id DESC
LIMIT 20;
-- Limit -> Index Only Scan Backward using ord_created_id
--            Index Cond: (ROW(created_at, id) < ROW(...))
```

The row-value form is important: it becomes a single index condition. Measured on 16.15, the expanded `created_at < $1 OR (created_at = $1 AND id < $2)` was only a `Filter` on a backward scan of the whole index, which degrades as the client pages deeper. The sort direction of all key columns must be the same (all `DESC` or all `ASC`) for a backward scan to work.

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

-- BAD: SELECT count(*) on a large table to show "approximate" totals
-- GOOD for estimates:
SELECT reltuples::bigint FROM pg_class WHERE oid = 'orders'::regclass;
```

- `SELECT *` matters in hot paths and for wide tables: it prevents index-only scans, pulls TOASTed columns, and breaks when columns are added (`INSERT INTO x SELECT * FROM y`). It is not a defect in ad-hoc or narrow queries.
- Keep non-aggregate filters in `WHERE`, not `HAVING`. It is clearer, though the planner moves such conditions itself.
- Explicit `JOIN ... ON` instead of comma joins is a readability and safety rule (a forgotten predicate becomes visible), not a performance rule.
- Correlated subqueries in the select list run once per row, and on 16 so do correlated `IN (subquery)` filters (see above). Prefer `EXISTS`, a `JOIN` + `GROUP BY`, or `LEFT JOIN LATERAL` when they are expensive.
- **CTEs** are inlined since 12 when they are referenced once and have no side effects. `WITH c AS (SELECT * FROM evx) SELECT * FROM c WHERE id = 5` is planned as `Seq Scan on evx  Filter: (id = 5)`. `WITH c AS MATERIALIZED (...)` forces an optimization fence (`CTE Scan`). Use it deliberately, not out of habit from pre-12 advice. Data-modifying CTEs (`WITH x AS (DELETE ... RETURNING ...)`) always run exactly once.
- `ORDER BY random() LIMIT n` reads and sorts every row (`Limit -> Sort (Sort Key: random()) -> Seq Scan`). Use `TABLESAMPLE SYSTEM (p)` or a keyed random pick for large tables.
- `DISTINCT` used to hide duplicate rows from a wrong join is a bug, not an optimization.

##### Prepared statements and generic plans

```sql
-- skew: k = 1 in 99% of rows
PREPARE q(int) AS SELECT count(*) FROM skew WHERE k = $1;
EXPLAIN (COSTS OFF) EXECUTE q(1);      -- Seq Scan on skew, Filter: (k = 1)
EXPLAIN (COSTS OFF) EXECUTE q(99999);  -- Index Only Scan using skew_k_idx
EXPLAIN (GENERIC_PLAN) SELECT count(*) FROM skew WHERE k = $1;   -- 16: the plan used after the switch
```

With `plan_cache_mode = auto` (the default), the first five executions are planned with the actual values. After that the server may switch to a generic plan if its estimated cost is not worse on average. For heavily skewed columns this can lock in a bad plan. Fix it per role, session or function with `plan_cache_mode = force_custom_plan`, or restructure the query. Don't disable prepared statements in the driver globally.

##### Statistics and maintenance

- Run `ANALYZE` after bulk loads and after creating expression indexes (the expression gets its own statistics).
- Raise per-column statistics targets for skewed columns: `ALTER TABLE t ALTER COLUMN c SET STATISTICS 1000` (reset with `SET STATISTICS -1` on 16; `DEFAULT` is 17). `pg_attribute.attstattarget` is `-1` for "default" on 16 (NULL on 17).
- Extended statistics for correlated columns: `CREATE STATISTICS (dependencies, mcv) ON city, zip FROM addresses; ANALYZE addresses;` (the name is optional since 16).
- `pg_upgrade` to 16 does **not** carry optimizer statistics over. Run `vacuumdb --all --analyze-in-stages` right after upgrading, or plans will be bad.

***

#### Transactions, Isolation and Concurrency

##### Isolation levels (all behaviour verified with two sessions on 16.15)[^15]

| Level | Snapshot | What you must handle |
|---|---|---|
| READ COMMITTED (default) | per statement | Lost updates in read-modify-write; non-repeatable reads |
| REPEATABLE READ | per transaction | `ERROR: could not serialize access due to concurrent update` (SQLSTATE `40001`) on conflicting writes. Retry |
| SERIALIZABLE | per transaction + SSI | `40001` "read/write dependencies" errors. Retry the **whole** transaction |

```sql
-- BAD (READ COMMITTED): read in app, compute, write back → lost update
SELECT bal FROM acc WHERE id = 1;            -- app reads 100
-- (another session: UPDATE acc SET bal = bal - 30 WHERE id = 1; commits → 70)
UPDATE acc SET bal = 50 WHERE id = 1;        -- app writes 100 - 50; the -30 is lost

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

**Write skew** (two on-call doctors both go off duty) is prevented only by `SERIALIZABLE`. In the test, the second commit failed with `could not serialize access due to read/write dependencies among transactions` (`Reason code: Canceled on identification as a pivot, during commit attempt.`). Code that uses `REPEATABLE READ` or `SERIALIZABLE` **must have a retry loop** for SQLSTATE `40001` (and `40P01` deadlock). Without one, the higher isolation level only turns data corruption into user-visible errors.

##### Row locks and queues

```sql
-- Job queue: each worker takes different rows without blocking
SELECT id FROM jobs WHERE NOT done ORDER BY id LIMIT 10 FOR UPDATE SKIP LOCKED;
```

- `SKIP LOCKED` is for queues. It returns an inconsistent view, so never use it for general reads. (Measured: while another session held rows 1–3, the query returned 4, 5, 6.)
- `NOWAIT` fails immediately (`could not obtain lock on row in relation "jobs"`). `lock_timeout` also applies to row locks (`canceling statement due to lock timeout`, `CONTEXT: while locking tuple (0,1) in relation "jobs"`).
- Use `FOR NO KEY UPDATE` when the key is not changed. It does not conflict with foreign key checks (`FOR KEY SHARE`) from inserts into child tables.
- Deadlocks: lock rows in a consistent order (`ORDER BY id ... FOR UPDATE`), and keep transactions short.
- **Advisory locks**: prefer `pg_advisory_xact_lock()`/`pg_try_advisory_xact_lock()`, which are released at commit or rollback. Session-level `pg_advisory_lock()` leaks across transaction-pooled connections and must be paired with `pg_advisory_unlock()` in every code path.

##### Upserts and MERGE

```sql
INSERT INTO kv (k, v, updated_at) VALUES ($1, $2, now())
ON CONFLICT (k) DO UPDATE
  SET v = kv.v + EXCLUDED.v, updated_at = EXCLUDED.updated_at
RETURNING *, (xmax = 0) AS inserted;    -- xmax = 0 → row was inserted (implementation detail)
```

- A multi-row `INSERT ... ON CONFLICT DO UPDATE` with the same key twice fails: `ON CONFLICT DO UPDATE command cannot affect row a second time`. Deduplicate input first.
- `ON CONFLICT DO NOTHING RETURNING *` returns **no row** for conflicting keys. Code that expects an id back must `SELECT` it afterwards, or use `DO UPDATE SET k = EXCLUDED.k` (which takes a row lock and writes a new row version).
- The conflict target must match a unique index or constraint exactly, including expression and partial-index predicate.
- **`MERGE` (15+)** on 16 has no `RETURNING`, no `WHEN NOT MATCHED BY SOURCE` and cannot target views (all 17). It is **not** an atomic upsert: with concurrent inserts of the same key, the `WHEN NOT MATCHED THEN INSERT` branch can raise a unique violation. For "insert or update one row by key" under concurrency, use `INSERT ... ON CONFLICT DO UPDATE`.[^16] If one target row matches several source rows, `MERGE` fails with `MERGE command cannot affect row a second time`. Deduplicate the source (`DISTINCT ON`) first. 16.1 fixed a `READ COMMITTED` recheck bug in concurrent `MERGE` updates, so run a current minor.

```sql
MERGE INTO stock s
USING (VALUES ('a',5),('c',3)) AS v(sku, qty) ON s.sku = v.sku
WHEN MATCHED THEN UPDATE SET qty = s.qty + v.qty
WHEN NOT MATCHED THEN INSERT VALUES (v.sku, v.qty);
-- MERGE 2
```

##### Timeouts and long transactions

- Long-running and idle-in-transaction sessions block vacuum cluster-wide (bloat, transaction ID wraparound risk), and they block DDL.
- Set per role: `statement_timeout`, `idle_in_transaction_session_timeout` and `idle_session_timeout` (14+). All default to 0 (disabled), as verified with `SHOW`. `idle_in_transaction_session_timeout` and `idle_session_timeout` **terminate** the session. There is no `transaction_timeout` on 16 (17).
- Never keep a transaction open across network calls, user think-time or message-queue waits.
- Sequences are non-transactional. A rolled-back insert consumes a value (measured: after a rollback the next id was 2). Never assume identity values are gap-free or ordered by commit time.

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

`format()` placeholders: `%I` quotes identifiers (`"Weird Name"`), `%L` quotes literals and renders NULL as the bare word `NULL`, and **`%s` does no quoting at all** (`format('%s', 'O''Reilly')` gives `O'Reilly`). Any `%s` that receives external input in SQL text is an injection. The same applies to `quote_ident()`/`quote_literal()`/`quote_nullable()`, which work but are easier to get wrong than `format()` + `USING`.

In application code: always use driver parameters (`$1`, `?`, named binds). Identifiers (table names, `ORDER BY` columns, sort direction) cannot be parameters. Validate them against an allow-list. Don't "escape" them by hand. Never build `IN (...)` lists by string joining. Use `= ANY($1)`. Code that quotes with libpq's `PQescapeLiteral()`/`PQescapeString()` needs libpq 16.8 or later (CVE-2025-1094, fixed in 16.7 with a regression fixed in 16.8: invalidly encoded input could break out of the quotes).[^10]

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

Measured on 16.15: `n_unsafe()` called the attacker's `lower()` as `postgres`; `n_safe()` did not. Every `SECURITY DEFINER` function needs `SET search_path` with trusted schemas only and `pg_temp` **last**, schema-qualified object references, and `REVOKE EXECUTE ... FROM PUBLIC`. Functions are executable by `PUBLIC` by default. To stop new functions from being executable by `PUBLIC`, run `ALTER DEFAULT PRIVILEGES [FOR ROLE owner] REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC` **without** `IN SCHEMA`. Measured on 16.15: the per-schema form (`ALTER DEFAULT PRIVILEGES IN SCHEMA s REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC`) stored nothing and had **no effect**, because per-schema default privileges can only add to the global defaults. The global form worked, but only for functions later created by the role it names (or the current role); a function that another role created in the same schema was still executable by `PUBLIC`. Keep the explicit `REVOKE EXECUTE ON FUNCTION ... FROM PUBLIC` per function.[^17] Owners should be dedicated, non-superuser roles.

##### Roles and privileges

- The application connects as a role that **does not own** the schema objects and is not a superuser. Migrations run as a separate owner role.
- Since 15, new databases do not let `PUBLIC` create objects in `public`. A non-owner role got `permission denied for schema public` on 16.15. Don't "fix" this with `GRANT CREATE ON SCHEMA public TO PUBLIC`. Create an application schema owned by the migration role.
- Use predefined roles instead of superuser: `pg_read_all_data`/`pg_write_all_data` (14), `pg_monitor`, `pg_signal_backend`, `pg_checkpoint` (15), `pg_create_subscription` and `pg_use_reserved_connections` (16). `pg_maintain` and `GRANT MAINTAIN` are 17.
- Review `CREATEROLE` usage against the 16 rules and use membership options (`WITH INHERIT FALSE`, `SET TRUE`) for "break-glass" roles that should be assumed explicitly (see [Roles](#new-in-16-roles-createrole-and-membership-options)).
- Passwords: `scram-sha-256` (default since 14). `md5` still works on 16; 18 deprecates it, so don't add new `md5` entries. Never put passwords in SQL that gets logged (`ALTER ROLE ... PASSWORD 'x'` appears in the server log when statement logging is on, and in `pg_stat_activity` while it runs). Use psql `\password`, which sends a hashed password.
- Clients: `require_auth=scram-sha-256` and `sslmode=verify-full` (or `sslrootcert=system`) with libpq 16.

##### Row-level security

```sql
ALTER TABLE docs ENABLE ROW LEVEL SECURITY;
ALTER TABLE docs FORCE ROW LEVEL SECURITY;   -- also apply to the table owner
CREATE POLICY tenant_isolation ON docs
  USING (tenant = current_setting('app.tenant', true))
  WITH CHECK (tenant = current_setting('app.tenant', true));
```

Behaviour verified on 16.15:[^18]

- **Superusers and roles with `BYPASSRLS` always bypass RLS**, even with `FORCE`.
- Without `FORCE ROW LEVEL SECURITY`, the **table owner bypasses** policies (a non-superuser owner saw all 2 rows). With `FORCE` the owner saw 0 rows with no tenant set.
- A `USING` policy also checks inserts when `WITH CHECK` is omitted: `new row violates row-level security policy for table "docs"`.
- `current_setting('app.tenant', true)` returns NULL when the setting was never defined, and the policy then matches nothing (safe). `current_setting('app.tenant')` without `true` raises `unrecognized configuration parameter`. **After a `set_config('app.tenant', 't2', true)` transaction ends, the setting reads as `''` (empty string), not NULL.** Policies must not treat `''` as a valid tenant.
- With transaction pooling, set the tenant with `SET LOCAL` / `set_config(..., true)` **inside each transaction**. A session-level `SET` leaks to the next client that gets the connection.
- Views run with the **view owner's** privileges and bypass RLS for the owner unless they are created `WITH (security_invoker = true)` (15+). In the test, a definer view owned by a superuser returned all rows to an unprivileged role, while the invoker view raised `permission denied for table docs`.
- Leakproof matters. User functions in policies or in queries on RLS tables may see rows before the policy filters them, unless they are `LEAKPROOF`. Keep policies simple and indexable.
- 16.5 fixed CVE-2024-10976: cached plans could ignore a role change when RLS applied to a subquery, CTE or security-invoker view, and return the wrong rows. 16.15 adds further invalidation of role-dependent cached plans. Multi-tenant RLS setups must run a current minor.[^10]

##### Other security items

- `COPY ... FROM/TO 'file'` and `COPY ... PROGRAM` run on the server with the server's OS user (superuser or `pg_read_server_files`/`pg_write_server_files`/`pg_execute_server_program`). Treat them as remote code execution if inputs reach them.
- `dblink`/`postgres_fdw` connection strings built from user input are injection and SSRF vectors.
- Statistics views: since 16.3 `pg_stats_ext`/`pg_stats_ext_exprs` show rows only to the table owner (CVE-2024-4317). Existing clusters needed the `fix-CVE-2024-4317.sql` script in every database.
- `pg_dump` scripts restored with psql can now protect the restoring OS user with `\restrict` (16.10, CVE-2025-8714/8715). Only restore dumps from trusted sources, and restore with the matching (current) client tools.[^10]
- Keep the server on the latest 16.x. 16.15 alone fixed 28 CVEs, including memory-safety bugs in `to_char()`, regexp functions and `ascii()`, and missing `USAGE` checks on data types.[^2]

***

#### Data Types: Pitfalls

All outputs in this section are from 16.15.

##### Timestamps

```sql
SET timezone = 'Europe/Berlin';
SELECT '2024-06-01 12:00+00'::timestamp;     -- 2024-06-01 12:00:00     (offset silently DROPPED)
SELECT '2024-06-01 12:00+00'::timestamptz;   -- 2024-06-01 14:00:00+02  (correct instant)
SELECT '2024-03-31 02:30'::timestamptz;      -- 2024-03-31 03:30:00+02  (nonexistent local time shifted)

-- DST: '1 day' is calendar-aware (in the session time zone), '24 hours' is not
SELECT timestamptz '2024-10-27 00:00 Europe/Berlin' + interval '1 day';    -- 2024-10-28 00:00:00+01
SELECT timestamptz '2024-10-27 00:00 Europe/Berlin' + interval '24 hours'; -- 2024-10-27 23:00:00+01

SELECT (timestamptz '2024-01-31 00:00+00' + interval '1 month')::date;  -- 2024-02-29 (clamped)
```

- Use **`timestamptz`** for points in time.[^19] It stores UTC and converts on input and output using the session `TimeZone`. Plain `timestamp` drops offsets on input and is only right for "wall clock" values (for example a store's opening hours).
- `now()::timestamp AT TIME ZONE 'UTC'` is a common bug. It first converts to session-local wall time and then interprets that as UTC. With a Berlin session it produced a value two hours off. Use `now() AT TIME ZONE 'UTC'` (gives `timestamp`) or keep `timestamptz`.
- `now()`/`CURRENT_TIMESTAMP` is the **transaction start time**. All rows in one transaction get the same value. Use `clock_timestamp()` for wall time inside long transactions.
- `timestamp(0)` **rounds**: `'2024-01-01 23:59:59.7'::timestamp(0)` gives `2024-01-02 00:00:00`.
- Day arithmetic for a specific zone: `date_add(ts, '1 day', 'Europe/Berlin')` (16) instead of relying on the session `TimeZone`. `date_bin('15 minutes', ts, origin)` (14+) buckets times.
- `interval` has no `infinity` on 16 (17). `'1 day ago 2 hours'::interval` is accepted on 16 and means `-1 days -02:00:00` (the whole interval is negated); 17 rejects it (`invalid input syntax for type interval`). Put `ago` last, or better, write negative intervals explicitly.
- `'+infinity'` is accepted for `timestamp`/`timestamptz`/`date` input since 16. `epoch` or `infinity` combined with other fields (`'epoch 2024-01-01'`) is rejected.

##### Numbers

```sql
SELECT 0.1::float8 + 0.2::float8 = 0.3::float8;   -- f
SELECT 0.1::numeric + 0.2 = 0.3;                  -- t
SELECT 1/2;                                       -- 0   (integer division)
SELECT round(2.5::numeric), round(2.5::float8);   -- 3, 2  (numeric: half away from zero; float: platform rounding, usually half to even)
SELECT '12.345'::numeric(5,2);                    -- 12.35 (silently rounded)
SELECT 2147483647::int + 1;                       -- ERROR: integer out of range
```

- Money: `numeric(p,s)` or integer minor units (cents in `bigint`). Never `float`/`real`/`double precision`. Avoid the `money` type, because its output depends on `lc_monetary` (`'1.5'::money` gave `$1.50`).
- Integer division truncates. Cast one side (`x::numeric / y`) when a fraction is expected.
- `numeric(p,s)` rounds extra scale silently, and raises an error only when the integer digits overflow.
- Identifiers: `bigint`. A busy `int` primary key or sequence overflows at 2,147,483,647, and fixing it is a full table rewrite.
- `EXTRACT()` returns `numeric` (14+). `date_part()` returns `double precision`.
- Integer and `numeric` input accept `0x`/`0o`/`0b` prefixes and underscores on 16 (see [Numeric Literals and Input](#new-in-16-numeric-literals-and-input)).

##### Text

- Use `text` (or `varchar` without a length). `varchar(n)` only adds a length check. It is not faster, and changing `n` later can need a rewrite (narrowing) or an `ACCESS EXCLUSIVE` lock. Use a `CHECK (length(x) <= n)` constraint if a business rule needs a limit.
- **Never `char(n)`**: it pads with spaces and ignores trailing spaces when comparing (`'ab '::char(5) = 'ab'::char(5)` is true, `length()` is 2, `octet_length()` is 5). Plain `varchar` treats `'ab '` and `'ab'` as different.
- An explicit cast to `varchar(n)` **silently truncates** (`'abcdef'::varchar(3)` gives `abc`). Inserting a too-long value into a `varchar(3)` column raises `value too long for type character varying(3)`.
- Case-insensitive matching: `lower(x)` with an expression index, `citext`, or a nondeterministic ICU collation (`CREATE COLLATION ci (provider = icu, locale = 'und-u-ks-level2', deterministic = false)`; `'A' = 'a' COLLATE ci` is true, but `LIKE` raises `nondeterministic collations are not supported for LIKE` on 16). There is no `casefold()` on 16 (18).
- ICU is built by default since 16, and 16 adds the `unicode` collation and custom tailoring rules (`CREATE COLLATION custom (provider = icu, locale = 'und', rules = '&a < g')` sorts `a, g, b`). `SHOW lc_collate` is gone; read `pg_database.datcollate`/`datlocprovider`.

##### Identity vs serial

```sql
-- GOOD
CREATE TABLE s2 (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, v int);
-- AVOID in new code
CREATE TABLE s1 (id serial PRIMARY KEY, v int);
```

Measured differences:

- `serial` allows explicit ids, which then collide with the sequence later (`duplicate key value violates unique constraint "s1_pkey"`). `GENERATED ALWAYS` rejects explicit values (`cannot insert a non-DEFAULT value into column "id"`) unless the statement uses `OVERRIDING SYSTEM VALUE`.
- With `serial`, a role that has `INSERT` on the table also needs `USAGE` on the separate sequence (`permission denied for sequence sq_id_seq`). Identity sequences need no extra grant.
- After bulk-loading explicit ids, reset the sequence: `SELECT setval(pg_get_serial_sequence('s2','id'), (SELECT max(id) FROM s2));`.
- Identity on partitioned tables works on 16 only through the parent (see [Partitioning](#partitioning)).
- UUID keys: `gen_random_uuid()` (v4, core since 13) causes random B-tree inserts. That is fine for moderate sizes, but `uuidv7()` is **18 only**. Generate time-ordered UUIDs in the application if you need them on 16.

##### NULL handling

```sql
SELECT NULL = NULL;                         -- NULL (not true)
SELECT NULL IS NOT DISTINCT FROM NULL;      -- t
SELECT count(*), count(x) FROM (VALUES (1),(NULL)) v(x);   -- 2, 1
SELECT sum(x) FROM (VALUES (NULL::int)) v(x);              -- NULL, not 0 → coalesce(sum(x), 0)
SELECT (ARRAY[10,20,30])[0];                               -- NULL (arrays are 1-based)
SELECT array_length('{}'::int[], 1), cardinality('{}'::int[]);  -- NULL, 0
```

`array_length(empty, 1)` is NULL, so `array_length(a, 1) = 0` never matches an empty array. Use `cardinality(a) = 0`.

##### Enums vs lookup tables vs CHECK

Enums are compact and ordered, but values cannot be removed or renamed easily, and on 16 a value added with `ADD VALUE` cannot be used until the transaction commits (even for a type created in the same transaction). Use a `CHECK (status IN (...))` or a foreign key to a lookup table when the set changes often.

***

#### JSONB

##### json vs jsonb

```sql
SELECT '{"a":1,"a":2}'::jsonb;        -- {"a": 2}           (last key wins, keys sorted)
SELECT '{"b":1,"a":2}'::json;         -- {"b":1,"a":2}      (text kept verbatim)
SELECT '{"n":1.0}'::jsonb = '{"n":1}'::jsonb;   -- t
```

Use `jsonb` for storage and querying. Use `json` only when the exact input text (key order, duplicates, whitespace) must be preserved. Remember that the 16 SQL/JSON constructors return `json` unless they say `RETURNING jsonb`.

##### Operators and extraction

```sql
SELECT doc -> 'a'   AS json_value,    -- jsonb
       doc ->> 'a'  AS text_value,    -- text
       doc['a']     AS subscript      -- jsonb (14+), needs parentheses around casts: ('{}'::jsonb)['a']
FROM t;

SELECT ('{"a":"5"}'::jsonb ->> 'a')::int + 1;   -- 6
SELECT ('null'::jsonb)::int;                     -- ERROR: cannot cast jsonb null to type integer (18: NULL)
SELECT '{"a":null}'::jsonb ->> 'a' IS NULL;      -- t: JSON null becomes SQL NULL via ->>
```

- `jsonb_set(doc, path, NULL)` returns **NULL for the whole document** (the function is strict). A measured `jsonb_set('{"a":1}', '{b}', to_jsonb(NULL::text))` returned NULL, and `UPDATE t SET doc = jsonb_set(doc, ..., $1)` with a NULL parameter wipes the column. Use `jsonb_set_lax(doc, path, value)` (default `'use_json_null'`, which gave `{"a": 1, "b": null}`), or `coalesce`.
- `(doc->>'a')::int` raises an error on bad data, which is usually what an import wants. To skip bad values instead, test them first with `pg_input_is_valid(doc->>'a', 'int4')` (16).
- `json_build_object('a', NULL)` keeps the key (`{"a" : null}`); `jsonb_strip_nulls()` removes object keys with null values (array elements are kept on 16).

##### Indexing JSONB (plans from 16.15)

```sql
CREATE INDEX evj_doc_gin ON evj USING gin (doc jsonb_path_ops);

EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc @> '{"type":"click"}';
-- Bitmap Index Scan on evj_doc_gin  Index Cond: (doc @> '{"type": "click"}'::jsonb)
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc @? '$.user ? (@.id == 5)';
-- Bitmap Index Scan on evj_doc_gin
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc ? 'type';
-- Parallel Seq Scan  (jsonb_path_ops does not support ?, ?|, ?&; the default jsonb_ops does)
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc->>'type' = 'click';
-- Parallel Seq Scan  (GIN does not help ->> equality)

CREATE INDEX evj_type ON evj ((doc->>'type'));
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc->>'type' = 'click';
-- Index Scan using evj_type on evj
```

- `jsonb_path_ops` is smaller and faster for `@>`, `@?` and `@@`. The default `jsonb_ops` also supports the key-existence operators `?`, `?|` and `?&`.
- For a frequently filtered scalar, use a B-tree expression index on `(doc->>'key')`, or better, promote it to a real column.
- JSONB is not a replacement for a schema. Fields that are always present, used in joins, constrained, or updated often belong in columns. Updating one key rewrites the whole (possibly TOASTed) document and its GIN entries.

***

#### Partitioning

```sql
CREATE TABLE m (
  id bigserial,                                      -- or identity, see below
  at timestamptz NOT NULL,
  v  int,
  PRIMARY KEY (id, at)                               -- must include the partition key
) PARTITION BY RANGE (at);
CREATE TABLE m_2025 PARTITION OF m FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE m_def  PARTITION OF m DEFAULT;
```

Rules verified on 16.15:[^20]

- **Unique and primary keys must include all partition key columns**: `PRIMARY KEY (id)` on a table partitioned by `at` fails (`unique constraint on partitioned table must include all partitioning columns`). Global uniqueness of `id` alone is not enforced, so don't assume it.
- **Pruning needs the raw partition key.** `WHERE at >= '2025-03-01' AND at < '2025-04-01'` scans only `m_2025`. `WHERE date_trunc('month', at) = '2025-03-01'` scans **every** partition. Stable expressions like `now() - interval '1 day'` are pruned at execution time (`EXPLAIN ANALYZE` shows `Subplans Removed: 2`).
- **Default partitions are a trap.** `ATTACH PARTITION` for a new range takes `ACCESS EXCLUSIVE` on the default partition and scans it (measured with `pg_locks`). If matching rows were already inserted there, the attach fails (`updated partition constraint for default partition "m_def" would be violated by some row`). `DETACH ... CONCURRENTLY` is not allowed while a default partition exists (`cannot detach partitions concurrently when a default partition exists`). Prefer creating future partitions ahead of time (cron, `pg_partman`) and alerting when the default partition receives rows.
- **Attaching without a long scan**: add a `CHECK` constraint on the staging table that matches the partition bounds (validated before attaching). `ATTACH PARTITION` then takes only `SHARE UPDATE EXCLUSIVE` on the parent.
- **Identity columns on 16 work only through the parent.** `CREATE TABLE ev(id bigint GENERATED ALWAYS AS IDENTITY, at date NOT NULL, ...) PARTITION BY RANGE (at)` is accepted and `INSERT INTO ev(at) ...` gets ids, but the partitions do **not** have the identity: `INSERT INTO ev_2024(at) VALUES (...)` failed with `null value in column "id" of relation "ev_2024" violates not-null constraint`. Code (ETL, `COPY` into a partition, a sharded writer) that inserts directly into partitions needs explicit ids or a `bigserial`-style default shared by all partitions. 17 makes partitions share the identity.
- **Exclusion constraints are not supported on partitioned tables** on 16 (`exclusion constraints are not supported on partitioned tables`). Put the constraint on each partition (it then only protects within a partition) or don't partition that table.
- **Generated columns** must be generated in the parent and in every partition (`column "b" in child table must be a generated column`), but since 16 the **expressions may differ, and the partition's expression wins**: a partition attached with `b GENERATED ALWAYS AS (a*3) STORED` under a parent with `(a*2)` returned `b = 33` for `a = 11` inserted through the parent. Check generated expressions when attaching existing tables.
- **Indexes**: create indexes on the parent with `ON ONLY` + per-partition `CREATE INDEX CONCURRENTLY` + `ATTACH PARTITION` (see migrations).
- Retention: `ALTER TABLE m DETACH PARTITION m_2023 CONCURRENTLY; DROP TABLE m_2023;` instead of large `DELETE`s.
- `VACUUM`/`ANALYZE` on a partitioned parent process the partitions. Autovacuum never analyzes the **parent** itself, so run `ANALYZE parent` after large data changes if queries rely on parent-level statistics. (`VACUUM ONLY`/`ANALYZE ONLY` are 18.)
- Too many partitions (thousands) slow planning and locking. Partition when it helps retention, pruning or maintenance, not by default.

***

#### SQL Idioms and Pitfalls

All outputs in this section are from 16.15.

##### Aggregates and NULL

```sql
SELECT region,
       count(*) FILTER (WHERE amount > 20) AS big,    -- FILTER instead of sum(CASE ...)
       sum(amount)                          AS total,
       avg(amount)                          AS avg_ignores_null
FROM sales GROUP BY region;
-- region 's' has amounts 5 and NULL: total = 5, avg = 5.0000000000000000 (NULL rows are ignored)

SELECT pg_typeof(sum(1::int)), pg_typeof(sum(1::bigint)), pg_typeof(avg(1::int));
-- bigint | numeric | numeric

SELECT 'a' || NULL, concat('a', NULL), concat_ws(',', 'a', NULL, 'b');
-- NULL | a | a,b
```

- `||` with a NULL operand yields NULL. A name built as `first || ' ' || middle || ' ' || last` disappears when `middle` is NULL. Use `concat_ws()` or `coalesce`.
- `avg()` and `sum()` ignore NULLs. If NULL means zero, write `avg(coalesce(x, 0))`, and say so explicitly.
- `sum(int)` returns `bigint` and `sum(bigint)` returns `numeric`. Driver code that maps the result to a 32-bit integer can overflow or fail to decode.
- `string_agg(x, ',' ORDER BY x)` and `array_agg(x ORDER BY x)`: without `ORDER BY` inside the aggregate, the order is unspecified, and parallel aggregation on 16 makes that visible.
- `any_value(x)` (16) instead of `min(x)` for "any value of the group"; it skips NULLs.
- `date - date` is an `integer` number of days. `age()` returns a calendar `interval` (`age('2024-03-01', '2024-01-31')` is `1 mon 1 day`). Don't use `extract(day FROM age(...))` as "days between".

##### Window functions

```sql
SELECT region, rep, amount,
       rank()       OVER w,   -- 1,1,3  (gaps after ties)
       row_number() OVER w,   -- 1,2,3  (ties broken arbitrarily)
       dense_rank() OVER w    -- 1,1,2
FROM sales
WINDOW w AS (PARTITION BY region ORDER BY amount DESC NULLS LAST);
```

- `row_number()` over a non-unique `ORDER BY` is **nondeterministic**. "Latest row per group" queries must add a unique tie-breaker (`ORDER BY at DESC, id DESC`).
- **The default frame is `RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW`.** `last_value(amount) OVER (ORDER BY at)` returns the *current* row's value (`30, 30, 10`), not the partition's last value. Write `ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING`, or use `first_value(...) OVER (ORDER BY at DESC)`. With `RANGE`, peers (equal `ORDER BY` values) are included in running sums. Use `ROWS` for strictly row-by-row running totals.
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

`CROSS JOIN LATERAL` drops parents without children. `LEFT JOIN LATERAL (...) o ON true` keeps them with NULLs. Choose deliberately.

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

Every recursive CTE over user-controlled graphs (org charts, categories, permissions) needs cycle protection (`CYCLE`, a depth limit, or `UNION` instead of `UNION ALL` when rows are fully identical). Without it, one bad edge produces an endless query.

##### Full-text search

```sql
SELECT to_tsvector('english', 'The quick brown foxes jumped')
       @@ websearch_to_tsquery('english', 'fox -cat');     -- t
SELECT to_tsquery('english', 'fox & (');                   -- ERROR: no operand in tsquery: "fox & ("
SELECT websearch_to_tsquery('english', 'fox & (');         -- 'fox' (never raises on user input)
```

- Pass user search input to `websearch_to_tsquery()` or `plainto_tsquery()`, never to `to_tsquery()`. Malformed input raises errors (a 500 from the search box).
- Always pass the configuration (`'english'`) explicitly. The one-argument form uses `default_text_search_config` and is not immutable, so it cannot be used in an index expression (`functions in index expression must be marked IMMUTABLE`). Queries must use the same configuration as the index.
- Store a `tsvector` as a `GENERATED ALWAYS AS (to_tsvector('english', coalesce(title,'') || ' ' || coalesce(body,''))) STORED` column with a GIN index for large tables.

##### Generated columns

```sql
CREATE TABLE gen(price numeric, qty int,
                 total numeric GENERATED ALWAYS AS (price * qty) STORED);
INSERT INTO gen(price, qty) VALUES (2, 3) RETURNING total;   -- 6
INSERT INTO gen VALUES (1, 1, 5);
-- ERROR: cannot insert a non-DEFAULT value into column "total"
CREATE TABLE gen2(t timestamptz, d date GENERATED ALWAYS AS (t::date) STORED);
-- ERROR: generation expression is not immutable
CREATE TABLE gen3(a int, b int GENERATED ALWAYS AS (a*2));
-- ERROR: syntax error at or near ")"     (virtual generated columns are 18)
```

On 16 every generated column must be `STORED`. `INSERT INTO t VALUES (...)` without a column list breaks when a generated column is added. Always list the target columns. Changing the expression later means drop and re-add (rewrite); `SET EXPRESSION` is 17.

##### Materialized views

- `REFRESH MATERIALIZED VIEW` takes `ACCESS EXCLUSIVE` and blocks readers for the whole refresh.
- `REFRESH MATERIALIZED VIEW CONCURRENTLY` needs a **unique index without a `WHERE` clause**. With only a partial unique index it fails: `cannot refresh materialized view "public.mvc" concurrently`, `HINT: Create a unique index with no WHERE clause on one or more columns of the materialized view.` It cannot be used for the first population (`CONCURRENTLY cannot be used when the materialized view is not populated`), and it is slower than a full refresh when most rows change.
- A materialized view is a snapshot. Code that reads it must tolerate stale data, and something must schedule the refresh.
- Functions in the view query need a pinned `search_path` (see [search_path](#functions-in-indexes-constraints-and-materialized-views-search_path)).

##### Collation-dependent results

```sql
SELECT 'Z' < 'a' COLLATE "C"          AS c_coll,  -- t   (byte/code-point order)
       'Z' < 'a' COLLATE "en_US.utf8" AS en;      -- f   (linguistic order)
```

Sort order and range comparisons on text depend on the collation. Keyset pagination and range filters on text columns must use the same collation as the index, and tests run under a different database locale (for example `C` in CI, `en_US.utf8` in production) can hide ordering bugs. libc collations can change with glibc upgrades; reindex text indexes (or check them with `amcheck`) when the OS changes under a cluster.

***

#### Functions, Procedures and Triggers

- **Volatility labels are promises.** `IMMUTABLE` means "same result for the same arguments forever, no table access". A function that reads tables, depends on settings (`TimeZone`, `search_path`), or calls `now()` must be `STABLE` or `VOLATILE`. A mislabeled `IMMUTABLE` function in an index gives wrong query results after data changes.
- Prefer SQL-standard bodies (`BEGIN ATOMIC ... END`, 14+) for SQL functions. They are parsed at creation time, so dependencies are tracked and names are resolved at creation (a `BEGIN ATOMIC` function worked unchanged with `search_path = ''`).
- Set `search_path` on functions that use unqualified names: a must for `SECURITY DEFINER` and for functions used by indexes, constraints and materialized views.
- PL/pgSQL: `RAISE EXCEPTION` with `USING ERRCODE`, `GET STACKED DIAGNOSTICS` in exception handlers. `EXCEPTION` blocks create a subtransaction per execution, which is costly in loops and consumes subtransaction IDs. For "is this valid input?" checks, use `pg_input_is_valid()` (16) instead of a cast inside `EXCEPTION`.
- **Bound cursor variables changed in 16.** Before 16, a bound cursor variable held its own name (`c`) before `OPEN`, so code could `OPEN c` and then `FETCH ... FROM c` (or return `c` as a `refcursor` and let the client fetch from `"c"`). On 16 the name is assigned at `OPEN` and is a generated portal name. Measured: a function returning `coalesce(n_before, 'NULL') || ' / ' || n_after` gave `NULL / <unnamed portal 1>` on 16 and `c / c` on 15. Code that relies on a fixed portal name must assign it before opening:

```sql
CREATE FUNCTION open_report() RETURNS refcursor LANGUAGE plpgsql AS $$
DECLARE c CURSOR FOR SELECT 1;
BEGIN
  c := 'report_cur';    -- 16: required if callers FETCH FROM "report_cur"
  OPEN c;
  RETURN c;
END $$;
BEGIN; SELECT open_report(); FETCH ALL FROM report_cur; COMMIT;
```

- `GET DIAGNOSTICS v = PG_ROUTINE_OID` (16) gives the current function's OID (0 in a `DO` block), useful for logging.
- Procedures (`CREATE PROCEDURE`, `CALL`) can `COMMIT` inside, which suits batch jobs. They cannot when called inside an outer transaction block.
- Triggers: `BEFORE` row triggers for normalization and validation, `AFTER` for side effects. Avoid row triggers that query other large tables. Statement-level triggers with transition tables (`REFERENCING NEW TABLE AS ...`) are cheaper for bulk changes. `CREATE OR REPLACE TRIGGER` (14+). Truncate triggers on foreign tables are new in 16.
- `ALTER FUNCTION ... SET work_mem/plan_cache_mode` scopes settings to one function.

***

#### Connection Pooling and Application Access

- **Always pool.** Each PostgreSQL connection is a process with several MB of memory. Size pools to roughly a small multiple of CPU cores, not to the number of application threads. `max_connections` in the thousands is a design smell. Keep a few slots for operators with `reserved_connections` + `pg_use_reserved_connections` (16).
- **PgBouncer transaction mode** (the most common setup) forbids session state across transactions: `SET` without `LOCAL`, session advisory locks, `LISTEN`, `WITH HOLD` cursors, temporary tables, and SQL-level `PREPARE`/`EXECUTE`.[^21] Use `SET LOCAL`/`set_config(..., true)` inside transactions, and `pg_advisory_xact_lock`.
- **Protocol-level prepared statements** work in transaction mode with PgBouncer 1.21+ when `max_prepared_statements` is non-zero (default 200 in current releases). SQL `PREPARE` statements are still not tracked.[^21]
- `idle_in_transaction_session_timeout` and `idle_session_timeout` **terminate** the server connection. Pools must validate connections and drivers must retry.
- Set `application_name` per service so `pg_stat_activity` and the logs identify the caller.
- Retries: retry on `40001` (serialization), `40P01` (deadlock), and connection loss before commit. Never blindly retry after an unknown commit outcome for non-idempotent writes. Use idempotency keys.
- `sslmode=verify-full` (or `sslrootcert=system`) for connections over untrusted networks; `require_auth` to refuse downgraded authentication (libpq 16).
- `load_balance_hosts=random` (libpq 16) spreads connections over several hosts, but is not a replacement for a pooler or for health checks.
- Use `COPY FROM STDIN` (via driver copy APIs) or multi-row `INSERT` for bulk writes instead of row-by-row inserts in a loop.

***

#### Maintenance, Monitoring and Observability

```sql
-- Slow statements (requires shared_preload_libraries = 'pg_stat_statements' + CREATE EXTENSION)
SELECT query, calls, total_exec_time, mean_exec_time, blk_read_time   -- 16 column name (17: shared_blk_read_time)
FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT 10;

-- Tables with heavy sequential scans (columns are relname, not tablename)
SELECT schemaname, relname, seq_scan, seq_tup_read, last_seq_scan, idx_scan, last_idx_scan
FROM pg_stat_user_tables ORDER BY seq_tup_read DESC LIMIT 10;

-- Unused indexes (check replicas too; counters reset with stats resets)
SELECT schemaname, relname, indexrelname, idx_scan, last_idx_scan,
       pg_size_pretty(pg_relation_size(indexrelid))
FROM pg_stat_user_indexes
WHERE idx_scan = 0 OR last_idx_scan < now() - interval '90 days'
ORDER BY pg_relation_size(indexrelid) DESC;

-- Long or idle-in-transaction sessions
SELECT pid, state, now() - xact_start AS xact_age, wait_event_type, wait_event, query
FROM pg_stat_activity WHERE xact_start IS NOT NULL ORDER BY xact_start;

-- Who blocks whom
SELECT pid, pg_blocking_pids(pid) AS blocked_by, query FROM pg_stat_activity
WHERE cardinality(pg_blocking_pids(pid)) > 0;

-- Checkpoints (16: still in pg_stat_bgwriter)
SELECT checkpoints_timed, checkpoints_req, checkpoint_write_time, buffers_checkpoint, buffers_backend
FROM pg_stat_bgwriter;

-- I/O (16)
SELECT backend_type, object, context, reads, writes, extends, hits, evictions
FROM pg_stat_io WHERE reads > 0 OR writes > 0;
```

- `pg_stat_user_tables` has `relname`; `pg_stat_user_indexes` has `relname` and `indexrelname`. Queries that use `tablename`/`indexname` (columns of `pg_tables`/`pg_indexes`) fail with `column "tablename" does not exist`.
- Queries written for 17 (`pg_stat_checkpointer`, `pg_wait_events`, `shared_blk_read_time`) fail on 16, and the 16 `pg_stat_bgwriter` checkpoint/backend columns fail on 17. Dashboards shared across versions need a version switch.
- Autovacuum: tune per table for large, hot tables (`ALTER TABLE t SET (autovacuum_vacuum_scale_factor = 0.01)`) rather than turning it off. Watch `n_dead_tup`, `last_autovacuum`, and `age(datfrozenxid)` for wraparound.
- Logging: `log_min_duration_statement`, `log_lock_waits = on`, `log_temp_files = 0`, `auto_explain` for slow plans (16 can log bind parameters: `auto_explain.log_parameter_max_length`). Logs contain query text and parameters, so treat them as sensitive.
- Plain `REINDEX` blocks writes. Use `REINDEX ... CONCURRENTLY` and only when bloat or corruption warrants it, or when a 16.x release note asks for it (see below).

***

#### Upgrading to 16 and Staying Current

##### Before upgrading from 14/15

1. **`CREATEROLE` scripts**: find provisioning code where a `CREATEROLE` role alters, drops or grants roles it did not create, or grants `CREATEDB`/`REPLICATION`/`BYPASSRLS` it does not have. Give it `ADMIN OPTION` on those roles, or run the code as the role that created them.
2. **Role inheritance**: `NOINHERIT` roles now produce memberships with `INHERIT FALSE` at grant time, and changing `INHERIT` later does not change existing grants. Review `\drg` output after the upgrade.
3. Remove `force_parallel_mode` (use `debug_parallel_query`), `vacuum_defer_cleanup_age` and `promote_trigger_file` from configuration files; an unknown core setting in `postgresql.conf` stops the server from starting.
4. Replace `SHOW lc_collate`/`SHOW lc_ctype` and `current_setting('lc_collate')` in code and monitoring with `pg_database.datcollate`/`datctype`.
5. Search for PL/pgSQL code that uses a bound cursor variable's name before or without `OPEN` assigning it.
6. Search old dumps and tools for `CREATE RULE "_RETURN"` and for primary keys built on `NULLS NOT DISTINCT` indexes.
7. Runbooks: `REINDEX DATABASE` no longer covers catalogs; `reindexdb --system` does.
8. Input validation that depends on `'0x10'::int` or `'1_000'::int` failing must be replaced by explicit checks.
9. Logical replication: subscriptions apply as the table owner. The subscription owner needs to be able to `SET ROLE` to every table owner, or set `run_as_owner = true` deliberately.
10. After `pg_upgrade`: run `vacuumdb --all --analyze-in-stages` (statistics are not transferred). Logical replication slots are not kept by `pg_upgrade` to 16; recreate them.
11. If the OS (glibc) changes at the same time, check libc-collated text indexes (`amcheck`/`pg_amcheck`) or reindex them.

##### 16.x minor releases

Minor releases contain only fixes and need no dump/restore, but some require actions:[^10]

- **16.1 (November 9, 2023)**: three CVEs (CVE-2023-5868 memory disclosure via `DISTINCT "any"` aggregates, CVE-2023-5869 array overflow with possible code execution, CVE-2023-5870 `pg_signal_backend` signalling background workers). **Reindex** all GiST indexes built with a recursive page-split bug, all B-tree indexes on `interval` columns (de-duplication could give wrong index-only scan results), and BRIN `minmax_multi` indexes on `date`/`timestamp`/`timestamptz`. A `MERGE` recheck bug under `READ COMMITTED` was fixed.
- **16.2 (February 8, 2024)**: CVE-2024-0985 (`REFRESH MATERIALIZED VIEW CONCURRENTLY` running code with the refreshing user's privileges). Reindex GIN indexes suspected of corruption from concurrent updates.
- **16.3 (May 9, 2024)**: CVE-2024-4317. `pg_stats_ext`/`pg_stats_ext_exprs` leaked statistics; existing clusters must run `fix-CVE-2024-4317.sql` in every database.
- **16.4 (August 8, 2024)**: CVE-2024-7348, code execution during a concurrent `pg_dump`. The new setting `restrict_nonsystem_relation_kind` (used by `pg_dump`) closes it when both server and `pg_dump` are updated.
- **16.5 (November 14, 2024)**: four CVEs (CVE-2024-10976 RLS-dependent cached plans, CVE-2024-10977 libpq trusting pre-TLS error messages, CVE-2024-10978 `SET SESSION AUTHORIZATION`/`SET ROLE` interaction, CVE-2024-10979 trusted PL/Perl changing environment variables). A partition detached from a partitioned table with a foreign key to another partitioned table, and not dropped, can have broken catalog state that must be repaired.
- **16.6 (November 21, 2024)** was out of cycle: it restored extension binary compatibility (`ResultRelInfo`) broken by 16.5 and restored `ALTER ROLE/DATABASE ... SET role`.
- **16.7 (February 13, 2025)**: CVE-2025-1094 in libpq quoting functions (`PQescapeLiteral()` and friends now validate encoding). Parallel workers are no longer subject to connection privilege checks and limits.
- **16.8 (February 20, 2025)** was out of cycle: it fixed a regression in the 16.7 quoting fix (`PQescapeLiteral()`/`PQescapeIdentifier()` ignored their length argument).
- **16.9 (May 8, 2025)**: CVE-2025-4207 (GB18030 over-read). Self-referential foreign keys on partitioned tables may need to be dropped and recreated (they were not fully enforced); BRIN bloom indexes should be reindexed.
- **16.10 (August 14, 2025)**: CVE-2025-8713 (statistics leak through views and partitions; view permission checks now happen at the start of planning, so permission errors can appear earlier), CVE-2025-8714/8715 (`pg_dump` output injection; psql gained `\restrict`/`\unrestrict`). Reindex BRIN `numeric_minmax_multi_ops` indexes.
- **16.11 (November 13, 2025)**: CVE-2025-12817 (`CREATE STATISTICS` now checks `CREATE` on the schema) and CVE-2025-12818 (libpq allocation overflow). JSON constructor expressions are no longer treated as strict by the planner.
- **16.12 (February 12, 2026)**: four CVEs (CVE-2026-2003 to 2006: `oidvector` checks, `intarray` selectivity estimator code execution, pgcrypto PGP buffer overrun, multibyte length validation). Attaching a non-built-in selectivity estimator to an operator now needs superuser, and string functions may now report invalid encoding for invalid text already stored. **16.13 (February 26, 2026)** was out of cycle and fixed regressions from 16.12 (multixact WAL replay on standbys, a too-strict `substring()` encoding check).
- **16.14 (May 14, 2026)**: nine CVEs, including unbounded recursion on startup packets, allocation-size overflows, quoting in `ALTER SUBSCRIPTION ... REFRESH PUBLICATION` (CVE-2026-6638), `CREATE` checks for multirange type schemas, timing-safe authentication comparisons and path traversal in `pg_basebackup`/`pg_rewind`. libpq's `PQfn()` is marked unsafe and deprecated.
- **16.15 (August 13, 2026)**: 28 CVE fixes. Logical decoding output plugins other than `pgoutput`/`test_decoding` must be allow-listed with `output_plugin_libraries` (CVE-2026-6471). psql now skips the inline data of a failed `COPY ... FROM STDIN` (CVE-2026-6464): test scripts need a `\.` after deliberately failing `COPY` commands. `amcheck` restricts `search_path` before running index expressions (CVE-2026-14673), role-dependent cached plans are invalidated after role changes, and missing `USAGE` checks on data types were added. Reindex `btree_gist` float indexes that may contain `NaN` and `ltree` B-tree indexes with very long values.[^2]

Pin `postgres:16` (floating minor) or an exact `postgres:16.15` in containers, and update minors promptly.

***

#### Preparing for PostgreSQL 17

16 is supported until November 2028, but code reviewed today should not make the next upgrade harder. Behaviour that works on 16.15 and fails on 17.11 (all verified):

| Works on 16 | On 17 | Do now |
|---|---|---|
| `CREATE INDEX ON mt (f_lookup(a))` with an unqualified table reference inside the function | `relation "lookup" does not exist` (maintenance runs with `search_path = pg_catalog, pg_temp`) | `SET search_path` on the function or qualify names |
| `'1 day ago 2 hours'::interval` | `invalid input syntax for type interval` | `ago` last, or a negative interval |
| `pg_stat_bgwriter.checkpoints_timed`, `buffers_backend` | `column "checkpoints_timed" does not exist` | Version-switch monitoring queries |
| `pg_stat_statements.blk_read_time` | renamed `shared_blk_read_time` | |
| `attstattarget = -1` means default | NULL means default | Test `attstattarget IS NULL OR attstattarget < 0` |
| `old_snapshot_threshold`, `db_user_namespace`, `trace_recovery_messages`, `adminpack` | removed | Do not adopt them |
| `pg_walfile_name('0/1000000')` = `...000000000000` | `...000000000001` | Review WAL-archiving scripts that compensate |

***

#### Production / Review Checklist

##### Version compatibility

- [ ] No PostgreSQL 17+ features: `JSON_TABLE`, `JSON_VALUE`/`JSON_QUERY`/`JSON_EXISTS`, `MERGE ... RETURNING`, `NOT MATCHED BY SOURCE`, `COPY ... ON_ERROR`, `transaction_timeout`, `random(min, max)`, `GRANT MAINTAIN`/`pg_maintain`, `SET EXPRESSION`, `EXPLAIN (SERIALIZE)`, `pg_stat_checkpointer`, builtin `C.UTF-8` collation
- [ ] No PostgreSQL 18+ features: `uuidv7()`, virtual generated columns (`GENERATED ALWAYS AS (...)` without `STORED`), `RETURNING old/new`, `WITHOUT OVERLAPS`/`PERIOD`, `NOT ENFORCED`, `NOT NULL ... NOT VALID`, `VACUUM/ANALYZE ONLY`, `COPY REJECT_LIMIT`, `casefold()`, `array_sort()`, int↔bytea casts
- [ ] Code shared with 15 does not use 16 features (`IS JSON`, `JSON_OBJECT`/`JSON_ARRAY`/`..AGG`, `0x`/`_` literals, `any_value`, `pg_input_is_valid`, `date_add`, subqueries without alias, `COPY ... DEFAULT`, `GRANT ... WITH INHERIT/SET`)
- [ ] No removed settings: `force_parallel_mode`, `vacuum_defer_cleanup_age`, `promote_trigger_file`, `SHOW lc_collate`/`lc_ctype`
- [ ] No hand-made `ON SELECT` rules; no primary key on a `NULLS NOT DISTINCT` index
- [ ] Forward-compatible with 17: functions in indexes/materialized views have `SET search_path`; interval literals end with `ago`; monitoring queries switch on `server_version_num`

##### Migrations

- [ ] `lock_timeout` set (with retry) for every DDL on existing tables; `SET LOCAL` inside transactional migrations
- [ ] `CREATE INDEX CONCURRENTLY` / `DROP INDEX CONCURRENTLY` on live tables, outside transaction blocks; invalid indexes handled on retry
- [ ] Foreign keys and `CHECK` constraints added `NOT VALID` then `VALIDATE`d; `SET NOT NULL` preceded by a validated `CHECK (col IS NOT NULL)`
- [ ] No volatile defaults or type changes on large tables without a rewrite plan
- [ ] Backfills and deletes in batches; no `UPDATE`/`DELETE` of millions of rows in one transaction
- [ ] Expand/contract for renames and drops; enum values added in one transaction are used only after commit
- [ ] Foreign key columns indexed

##### Queries

- [ ] Parameters bound (no string concatenation), `= ANY($1)` for lists, identifiers allow-listed
- [ ] Predicates sargable: no functions or casts on indexed columns, half-open time ranges, parameter types match column types
- [ ] Keyset pagination with a row-value comparison and a matching composite index for deep pages
- [ ] `NOT EXISTS` instead of `NOT IN (subquery)` on nullable columns; `EXISTS` instead of correlated `IN (subquery)`
- [ ] Composite indexes match leading columns (no skip scan in 16)
- [ ] `ORDER BY` inside `string_agg`/`array_agg`/`JSON_ARRAYAGG` when order matters
- [ ] `EXPLAIN (ANALYZE, BUFFERS)` evidence for performance claims; DML explained inside `BEGIN ... ROLLBACK`; `EXPLAIN (GENERIC_PLAN)` for parameterized statements

##### JSON

- [ ] SQL/JSON constructors state their NULL handling (`NULL ON NULL`/`ABSENT ON NULL`) and use `RETURNING jsonb` when the result is compared or stored as `jsonb`
- [ ] `JSON_ARRAYAGG`/`json_agg` results wrapped in `coalesce(..., '[]')` where an empty list is expected
- [ ] `IS JSON OBJECT` (not plain `IS JSON`) when an object is required
- [ ] GIN `jsonb_path_ops` for `@>`/`@?`; expression indexes for hot keys; `jsonb_set_lax` or NULL guards

##### Transactions and concurrency

- [ ] Read-modify-write made atomic (`UPDATE ... SET x = x + ...`, `FOR UPDATE`, or version checks)
- [ ] Retry loops for `40001`/`40P01` with `REPEATABLE READ`/`SERIALIZABLE`
- [ ] Upserts use `INSERT ... ON CONFLICT` with an exact conflict target; inputs deduplicated; `MERGE` not used as a concurrent upsert
- [ ] Short transactions; no network calls inside; timeouts set per role
- [ ] Queues use `FOR UPDATE SKIP LOCKED`; advisory locks are transaction-scoped

##### Security

- [ ] `SECURITY DEFINER` functions: `SET search_path = ..., pg_temp`, qualified names, `REVOKE EXECUTE ... FROM PUBLIC`
- [ ] Dynamic SQL uses `format('%I')` + `USING`, never `%s` with input
- [ ] App role is not owner or superuser; RLS tables use `FORCE ROW LEVEL SECURITY` where owners connect; tenant set with `SET LOCAL`; `''` not accepted as a tenant
- [ ] Views over RLS-protected or restricted tables use `security_invoker = true`
- [ ] `CREATEROLE` provisioning works with 16 rules (`ADMIN OPTION`); membership options (`INHERIT`, `SET`) chosen deliberately; permission tests use real logins
- [ ] Input validation does not rely on casts rejecting `0x..`/`1_000`
- [ ] No server-side `COPY` file/program from application paths; no passwords in logged SQL
- [ ] Clients use `sslmode=verify-full`/`sslrootcert=system` and `require_auth` where libpq 16 is available
- [ ] Server on the latest 16.x minor; reindex actions from the minor release notes done

##### Data types and schema

- [ ] `timestamptz` for instants; zone-specific day arithmetic with `date_add(..., zone)`; `bigint` identity for keys; `numeric` or integer minor units for money; `text` + `CHECK` instead of `char(n)`/arbitrary `varchar(n)`
- [ ] `GENERATED ALWAYS AS IDENTITY` instead of `serial`; sequences reset after explicit-id loads
- [ ] Partitioned tables: keys include the partition key, queries filter on the raw key, future partitions pre-created, default partition monitored, no direct inserts into partitions that rely on the parent's identity, generated expressions identical across partitions

##### Operations

- [ ] Pooling in place; no session state with transaction pooling; `reserved_connections` for operators
- [ ] `pg_stat_statements`, `log_lock_waits`, `log_min_duration_statement` configured; monitoring queries use 16 column names; `pg_stat_io` and `last_idx_scan` used for I/O and index reviews
- [ ] `ANALYZE` after bulk loads and after `pg_upgrade`
- [ ] Logical replication: third-party output plugins listed in `output_plugin_libraries` (16.15); subscription owners can `SET ROLE` to table owners

***

[^1]: PostgreSQL 16 Release Notes (E.16, Release 16; release date 2023-09-14). https://www.postgresql.org/docs/release/16.0/
[^2]: PostgreSQL Versioning Policy (supported versions: 16.15 current, first release September 14, 2023, final release November 9, 2028; 17.11, 18.6; 19 RC 1 news) and PostgreSQL 16.15 Release Notes (August 13, 2026). https://www.postgresql.org/support/versioning/ , https://www.postgresql.org/docs/release/16.15/
[^3]: PostgreSQL Roadmap (next minor releases: November 12, 2026, February 11, 2027). https://www.postgresql.org/developer/roadmap/
[^4]: PostgreSQL 16 Released! (press release, September 14, 2023). https://www.postgresql.org/about/news/postgresql-16-released-2715/
[^5]: PostgreSQL 17 Release Notes (Release 17.0). https://www.postgresql.org/docs/release/17.0/
[^6]: PostgreSQL 18 Release Notes (Release 18.0). https://www.postgresql.org/docs/release/18.0/
[^7]: PostgreSQL 19 Release Notes (draft, Release Candidate 1). https://www.postgresql.org/docs/19/release-19.html
[^8]: PostgreSQL 15 and 14 Release Notes (feature version markers). https://www.postgresql.org/docs/release/15.0/ , https://www.postgresql.org/docs/release/14.0/
[^9]: PostgreSQL 16 Documentation: JSON Functions and Operators (SQL/JSON constructors, IS JSON). https://www.postgresql.org/docs/16/functions-json.html
[^10]: PostgreSQL 16.1 – 16.14 Release Notes. https://www.postgresql.org/docs/release/16.1/ (and /16.2/ ... /16.14/ under https://www.postgresql.org/docs/release/)
[^11]: PostgreSQL 16 Documentation: CREATE ROLE, GRANT, Role Membership and Predefined Roles (pg_create_subscription, pg_use_reserved_connections). https://www.postgresql.org/docs/16/sql-createrole.html , https://www.postgresql.org/docs/16/sql-grant.html , https://www.postgresql.org/docs/16/role-membership.html , https://www.postgresql.org/docs/16/predefined-roles.html
[^12]: PostgreSQL 16 Documentation: Connections and Authentication (reserved_connections, scram_iterations). https://www.postgresql.org/docs/16/runtime-config-connection.html
[^13]: PostgreSQL 16 Documentation: Explicit Locking. https://www.postgresql.org/docs/16/explicit-locking.html
[^14]: PostgreSQL 16 Documentation: ALTER TABLE; CREATE INDEX (Building Indexes Concurrently). https://www.postgresql.org/docs/16/sql-altertable.html , https://www.postgresql.org/docs/16/sql-createindex.html
[^15]: PostgreSQL 16 Documentation: Transaction Isolation. https://www.postgresql.org/docs/16/transaction-iso.html
[^16]: PostgreSQL 16 Documentation: MERGE. https://www.postgresql.org/docs/16/sql-merge.html
[^17]: PostgreSQL 16 Documentation: CREATE FUNCTION — Writing SECURITY DEFINER Functions Safely. https://www.postgresql.org/docs/16/sql-createfunction.html
[^18]: PostgreSQL 16 Documentation: Row Security Policies. https://www.postgresql.org/docs/16/ddl-rowsecurity.html
[^19]: PostgreSQL Wiki: Don't Do This. https://wiki.postgresql.org/wiki/Don't_Do_This
[^20]: PostgreSQL 16 Documentation: Table Partitioning. https://www.postgresql.org/docs/16/ddl-partitioning.html
[^21]: PgBouncer configuration (max_prepared_statements, transaction pooling restrictions). https://www.pgbouncer.org/config.html
[^22]: PostgreSQL 16 Documentation: libpq Connection Parameters (require_auth, load_balance_hosts, sslcertmode, sslrootcert=system). https://www.postgresql.org/docs/16/libpq-connect.html
[^23]: PostgreSQL 16 Documentation: The pg_hba.conf File (regular expressions, include directives). https://www.postgresql.org/docs/16/auth-pg-hba-conf.html
[^24]: PostgreSQL 16 Documentation: Logical Decoding Concepts (slots on standbys, hot_standby_feedback). https://www.postgresql.org/docs/16/logicaldecoding-explanation.html
