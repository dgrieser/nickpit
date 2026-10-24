### PostgreSQL 17: The Complete Review Guide

#### Overview

> **Version**: PostgreSQL 17.0 was released on **September 26, 2024**. The latest minor release is **17.11 (August 13, 2026)**; the next scheduled minor release date is November 12, 2026. PostgreSQL 17 is **supported until November 8, 2029**.
> **Newest releases (October 2026)**: PostgreSQL **18** is the current stable major version (18.0 on September 25, 2025; current minor 18.6). PostgreSQL **19 is at Release Candidate 1** (October 15, 2026). It is pre-release software and not a deployment target. PostgreSQL 14 reaches end of life on November 12, 2026.
> **Scope**: SQL that runs on a PostgreSQL 17 server: schema migrations, queries embedded in application code, functions, procedures and triggers, schema design, indexing, transactions, security, JSON, partitioning, connection pooling and operations-facing SQL.
> **Note**: This document is self-contained. It carries forward everything from PostgreSQL 14–16 that is still correct, and it marks the version in which each feature arrived. Sections that cover 17-specific behaviour are marked **New in 17**. Every example was executed on PostgreSQL 17.11. Boundary claims ("new in 17", "not available in 17") were checked on 16.15 and 18.6. Older "since 14/15/16" markers were checked on 13.23, 14.24 and 15.19.

PostgreSQL 17 is mainly a performance and operations release with a few large SQL features. Its headline changes are **`JSON_TABLE()` and the SQL/JSON query functions** (`JSON_EXISTS`, `JSON_QUERY`, `JSON_VALUE`) and constructors (`JSON()`, `JSON_SCALAR()`, `JSON_SERIALIZE()`), **`MERGE ... RETURNING`** with `merge_action()`, `WHEN NOT MATCHED BY SOURCE` and `MERGE` into views, **`COPY ... ON_ERROR ignore`**, **incremental backups** (`pg_basebackup --incremental` plus `pg_combinebackup`), a new **vacuum dead-tuple store that uses up to 20x less memory** and is no longer capped at 1 GB, **streaming I/O** for sequential scans and `ANALYZE`, faster **B-tree `IN (...)` / `= ANY(array)` lookups**, **identity columns and exclusion constraints on partitioned tables**, the **`MAINTAIN` privilege and `pg_maintain` role**, **`transaction_timeout`**, a **builtin collation provider** (`C.UTF-8`), **direct TLS negotiation** (`sslnegotiation=direct`) and **failover for logical replication slots**.

PostgreSQL 17 also changes behaviour that breaks old code. **Maintenance commands now run functions with a safe `search_path`** (`pg_catalog, pg_temp`), so expression indexes and materialized views whose functions use unqualified names can fail on `CREATE INDEX`, `REINDEX`, `REFRESH` or after a `pg_upgrade`. **`ago` must be the last token of an interval literal.** `old_snapshot_threshold`, `db_user_namespace`, `adminpack` and `trace_recovery_messages` are gone. Several monitoring columns moved or were renamed (`pg_stat_bgwriter` → `pg_stat_checkpointer`, `pg_stat_statements.blk_read_time` → `shared_blk_read_time`).

Global truths about PostgreSQL 17:

- **MVCC.** Readers never block writers and writers never block readers at the row level. `UPDATE` writes a new row version and `DELETE` marks the old one dead. `VACUUM` (usually autovacuum) reclaims dead versions. Long transactions stop that reclamation everywhere in the cluster.
- **DDL is transactional, but it takes locks.** Almost every `ALTER TABLE` form takes an `ACCESS EXCLUSIVE` lock. The lock is held until commit, and it queues behind running queries while blocking new ones (see [Schema migrations and locking](#schema-migrations-and-locking)).
- **Default isolation is READ COMMITTED.** Each statement sees a fresh snapshot. A read-modify-write in application code is a lost update unless it is made atomic.
- **NULL is three-valued.** `NULL = NULL` is NULL, `NOT IN` with a NULL in the list never returns true, and `count(col)` skips NULLs.
- **Identifiers fold to lower case** unless they are double-quoted. String literals use single quotes.
- **Arrays are 1-based.** `arr[0]` is NULL, not an error.
- **Use `timestamptz`, `text`, `bigint` identity and `numeric` for money** by default. The rest of this guide explains why.

***

#### Version-Aware Review

The project pins PostgreSQL 17 (for example `image: postgres:17`, `postgresql 17.x` in `.tool-versions`, or a CI service container). Apply these rules when reviewing code for it:

- **Verify before you claim something is unavailable.** A function or syntax that you remember as "new" may already be in 17 (for example `JSON_TABLE`, `MERGE ... RETURNING`, `random(min, max)`, `COPY ... ON_ERROR`). Say "not available in PostgreSQL 17" only for items on the [Do Not Use or Recommend](#do-not-use-or-recommend-postgresql-18-features) list or when the 17 documentation proves it.
- **Never suggest PostgreSQL 18 features.** `uuidv7()`, virtual generated columns, `RETURNING old.*/new.*`, `WITHOUT OVERLAPS`, `NOT ENFORCED`, B-tree skip scan and asynchronous I/O do not exist in 17. Code that uses them fails with a syntax error or "function does not exist".
- **Flag code that only works on newer servers.** Example: `GENERATED ALWAYS AS (expr)` without `STORED` is valid on 18 (virtual column) and a **syntax error on 17**.
- **Flag code that assumes older behaviour.** Example: a migration that creates an expression index on a function with an unqualified table reference worked on 16 and fails on 17.
- **Many projects run several versions.** If the code also has to run on 16 (a library, an extension, a shared migration set), every **New in 17** feature in this guide is a portability bug there.
- **Client libraries are separate.** `sslnegotiation=direct`, `PQchangePassword()` and chunked rows mode are libpq 17 features. They need a version 17 libpq on the client, not only a version 17 server.

***

#### What's New in PostgreSQL 17

All items below are verified against the official release notes and, where they are SQL-visible, were executed on 17.11 and shown to fail on 16.15.

##### SQL and data types

| Feature | Notes |
|---|---|
| `JSON_TABLE()` | Turns a JSON document into rows and columns in `FROM`. Supports `COLUMNS`, `FOR ORDINALITY`, `EXISTS PATH`, `NESTED PATH`, `PASSING`, `ON EMPTY`/`ON ERROR`. |
| `JSON_EXISTS()`, `JSON_QUERY()`, `JSON_VALUE()` | SQL-standard query functions with `RETURNING`, `WRAPPER`, `QUOTES`, `ON EMPTY`, `ON ERROR`. |
| `JSON()`, `JSON_SCALAR()`, `JSON_SERIALIZE()` | SQL/JSON constructors. `JSON(... WITH UNIQUE KEYS)` rejects duplicate keys. |
| jsonpath methods | `.bigint()`, `.boolean()`, `.date()`, `.decimal([p[,s]])`, `.integer()`, `.number()`, `.string()`, `.time()`, `.time_tz()`, `.timestamp()`, `.timestamp_tz()` |
| `MERGE ... RETURNING` | With the new `merge_action()` function (`'INSERT'`, `'UPDATE'`, `'DELETE'`). |
| `WHEN NOT MATCHED BY SOURCE` | `MERGE` can update or delete target rows that have no source row. |
| `MERGE` into views | Auto-updatable views or views with `INSTEAD OF` triggers. |
| `COPY ... ON_ERROR ignore` | Skips rows with data type conversion errors. `LOG_VERBOSITY verbose` logs each skipped row. |
| `COPY ... FORCE_NULL *`, `FORCE_NOT_NULL *` | Apply to all columns. |
| Identity columns on partitioned tables | `GENERATED ... AS IDENTITY` on the parent. |
| Exclusion constraints on partitioned tables | They must include the partition key columns with equality. |
| `ALTER TABLE ... ALTER COLUMN ... SET EXPRESSION AS (...)` | Changes a stored generated column. Rewrites the table. |
| `ALTER TABLE ... SET STATISTICS DEFAULT` | Replaces the old `SET STATISTICS -1`. |
| `ALTER TABLE ... SET ACCESS METHOD DEFAULT`, table AMs on partitioned tables | |
| `interval` `'infinity'` / `'-infinity'` | |
| `AT LOCAL` | `ts AT LOCAL` converts using the session time zone. |
| `random(min, max)` | For `integer`, `bigint` and `numeric`. |
| `uuid_extract_timestamp()`, `uuid_extract_version()` | |
| `to_bin()`, `to_oct()` | |
| `to_timestamp()` / `to_date()` accept `TZ` and `OF` | `OF` was output-only before. |
| `unicode_version()`, `icu_unicode_version()`, `unicode_assigned()` | |
| `pg_basetype()`, `to_regtypemod()`, `pg_column_toast_chunk_id()`, `xmltext()` | |
| Enum values added in the same transaction that created the type are usable | 16 raised "unsafe use of new value". |
| PL/pgSQL `%TYPE[]` / `%ROWTYPE[]` | Arrays of a column's or row's type. |

##### Server, performance and operations

- **Vacuum** stores dead tuple IDs in a new radix-tree structure. It uses up to 20x less memory and is no longer silently limited to 1 GB of `maintenance_work_mem`/`autovacuum_work_mem`. Vacuum also writes less WAL. `vacuum_buffer_usage_limit` defaults to 2MB (256kB in 16).
- **Streaming I/O.** Sequential scans and `ANALYZE` combine adjacent block reads. `io_combine_limit` (default 128kB) sets the maximum read size.
- **B-tree `IN` lists.** One index scan handles several array values that are on the same leaf page instead of descending once per value (measured below: 4 buffer hits on 17 against 64 on 16 for 20 adjacent keys).
- **Planner.** It removes `IS NOT NULL` on `NOT NULL` columns, replaces `IS NULL` on such columns with a constant-false filter, turns correlated `IN (subquery)` into semi-joins, can use statistics and sort order from CTEs, and can use `MergeAppend` for `UNION`. `GROUP BY` keys can be reordered to match `ORDER BY` (`enable_group_by_reordering`).
- **BRIN indexes** can be built in parallel.
- **WAL** throughput under high concurrency improves by up to 2x.
- **Incremental backup.** `summarize_wal = on` plus `pg_basebackup --incremental=<manifest>`, then rebuild with `pg_combinebackup`.
- **Logical replication.** Failover slots (`failover` option, `sync_replication_slots`, `synchronized_standby_slots`), `pg_createsubscriber`, and `pg_upgrade` now keeps logical slots and subscriptions (when upgrading *from* 17 or later).
- **New settings.** `transaction_timeout`, `allow_alter_system`, `event_triggers`, `huge_pages_status`, `summarize_wal`, `wal_summary_keep_time`, `io_combine_limit` and the SLRU buffer settings (`transaction_buffers`, `subtransaction_buffers`, `multixact_*_buffers`, ...).
- **Monitoring.** `pg_stat_checkpointer` (checkpoint columns moved out of `pg_stat_bgwriter`), `pg_wait_events`, `pg_stat_progress_vacuum.indexes_total`/`indexes_processed`, `pg_replication_slots.inactive_since`/`invalidation_reason`, and `EXPLAIN (SERIALIZE, MEMORY)`.
- **Privileges.** The `MAINTAIN` table privilege and the `pg_maintain` predefined role.
- **Builtin collation provider.** `provider = builtin` with locales `C` and `C.UTF-8`. It is platform-independent and immutable across OS upgrades.
- **Event triggers** on login (`ON login`) and for `REINDEX`.
- **libpq.** `sslnegotiation=direct` (TLS without the `SSLRequest` round trip, using ALPN), `PQchangePassword()`, `PQclosePrepared()`/`PQclosePortal()`, new encrypted cancel APIs (`PQcancelCreate()` and related), `PQsetChunkedRowsMode()`, `PQsocketPoll()`, `PQsendPipelineSync()`.
- **Other server changes.** `ALTER SYSTEM` (and `GRANT ... ON PARAMETER`) accepts unrecognized custom settings such as `myapp.x`; `pg_stat_reset_shared()` with no argument resets all shared statistics; partition pruning works for `IS [NOT] UNKNOWN` on boolean partition keys; `postgres_fdw` pushes down `EXISTS`/`IN` subqueries; `ltree` supports hash indexes; `pg_buffercache_evict()`; the `unaccent.rules` file syntax changed (rules may contain whitespace and quotes); PL/pgSQL `%TYPE` can reference a column of a composite variable.
- **Tools.** `pg_dump --filter`, `pg_dump --exclude-extension`, `pg_restore --transaction-size`, `pg_upgrade --copy-file-range`, `vacuumdb/reindexdb/clusterdb --all` with patterns, `pg_walsummary`.

***

#### Do Not Use or Recommend (PostgreSQL 18+ Features)

The server is PostgreSQL 17. Everything in this list was introduced in **PostgreSQL 18** (released September 25, 2025) and was shown to fail on 17.11. Never suggest these, and flag them as bugs when they appear in code that targets 17:

| PostgreSQL 18 feature | What happens on 17 | Use on 17 instead |
|---|---|---|
| `uuidv7()`, `uuidv4()` | `function uuidv7() does not exist` | `gen_random_uuid()` (v4, core since 13), or generate UUIDv7 in the application or with an extension |
| Virtual generated columns: `GENERATED ALWAYS AS (expr)` without `STORED`, or `... VIRTUAL` | **syntax error** | `GENERATED ALWAYS AS (expr) STORED`, or a view |
| `RETURNING old.col, new.col` in `INSERT`/`UPDATE`/`DELETE`/`MERGE` | `missing FROM-clause entry for table "old"` | A CTE that selects the old row first, or a trigger |
| Temporal constraints: `PRIMARY KEY (id, during WITHOUT OVERLAPS)`, `FOREIGN KEY (..., PERIOD during)` | syntax error | `EXCLUDE USING gist (id WITH =, during WITH &&)` (needs `btree_gist`) |
| `CHECK (...) NOT ENFORCED`, foreign keys `NOT ENFORCED` | syntax error | Leave the constraint out, or add it `NOT VALID` |
| `ALTER TABLE ... ADD CONSTRAINT name NOT NULL col NOT VALID`, named/inheritable `NOT NULL` constraints in `pg_constraint` | syntax error | `CHECK (col IS NOT NULL) NOT VALID`, `VALIDATE`, then `SET NOT NULL` (see below) |
| `VACUUM ONLY t` / `ANALYZE ONLY t` | syntax error | Plain `VACUUM t`/`ANALYZE t` (on a partitioned table they process the partitions). See [Partitioning](#partitioning) |
| `COPY ... REJECT_LIMIT n`, `LOG_VERBOSITY silent` | `option "reject_limit" not recognized` / `COPY LOG_VERBOSITY "silent" not recognized` | `ON_ERROR ignore` plus checking the `NOTICE` count, or a staging table |
| `casefold()` | function does not exist | `lower()`, `citext`, or a nondeterministic ICU collation |
| `array_sort()`, `array_reverse()` | function does not exist | `ARRAY(SELECT x FROM unnest(a) x ORDER BY x)` |
| `crc32()`, `crc32c()`, `reverse(bytea)`, `gamma()`, `lgamma()` | function does not exist | |
| `integer`/`bigint` ↔ `bytea` casts (`1::int::bytea`) | `cannot cast type integer to bytea` | `int4send(1)` |
| `('null'::jsonb)::int` returning NULL | **raises** `cannot cast jsonb null to type integer` on 17 | `(doc->>'k')::int`, or `JSON_VALUE(doc, '$.k' RETURNING int)` |
| `pg_get_acl()`, `has_largeobject_privilege()`, `pg_restore_relation_stats()` and the other stats import functions | function does not exist | |
| B-tree **skip scan** (using `(a, b)` for `WHERE b = ?`) | 17 does not do it. The measured plan below is a sequential scan. | An index that leads with the filtered column |
| Asynchronous I/O (`io_method`, `pg_aios`), `io_max_combine_limit` | unrecognized parameter | |
| `EXPLAIN ANALYZE` that includes `BUFFERS` automatically | 17 shows buffers only with `EXPLAIN (ANALYZE, BUFFERS)` | Write `BUFFERS` explicitly |
| OAuth authentication (`oauth` in `pg_hba.conf`), `md5_password_warnings` | | `scram-sha-256` |
| `json[b]_strip_nulls(x, strip_in_arrays)`, named arguments for the `regexp_*` functions, `to_number(..., 'RN')` | `function ... does not exist`; `to_number` raises `"RN" not supported for input` | |
| `log_lock_failures`, `idle_replication_slot_timeout`, `autovacuum_vacuum_max_threshold`, `autovacuum_worker_slots`, `vacuum_truncate` GUC, `enable_self_join_elimination`, `enable_distinct_reordering`, `ssl_tls13_ciphers`, `ssl_groups`, `extension_control_path`, `file_copy_method` | unrecognized configuration parameter | |
| `pg_upgrade` keeping optimizer statistics, `pg_dump --statistics`, `--no-policies`, `--sequence-data` | Statistics are **not** carried over by a `pg_upgrade` *to* 17 | Run `vacuumdb --all --analyze-in-stages` after upgrading |
| `initdb` enabling data checksums by default | 17's `initdb` does **not** enable checksums unless you pass `--data-checksums` (`-k`) | |
| PL/pgSQL `=>` for named cursor arguments | only `:=` | |

Anything from **PostgreSQL 19** (Release Candidate 1, October 15, 2026, not yet released) is also unavailable, for example the `REPACK` command (`REPACK (CONCURRENTLY)`) and `error_on_null()`. Both work on 19rc1 and fail on 17.11 (`syntax error at or near "REPACK"`, `function error_on_null(integer) does not exist`). `GROUP BY ALL` is not a feature of any of these versions: it is a syntax error on 17.11, 18.6 and 19rc1.

##### Deprecated or removed in 17 (do not use)

| Removed / changed | Replacement |
|---|---|
| `old_snapshot_threshold` (the "snapshot too old" feature) | None. Use `idle_in_transaction_session_timeout`, `transaction_timeout` and monitoring of old snapshots |
| `db_user_namespace` | Separate roles |
| `adminpack` extension | None (it served pgAdmin III) |
| `trace_recovery_messages` | `log_min_messages` |
| `pg_stat_bgwriter.buffers_backend`, `buffers_backend_fsync` | `pg_stat_io` (16+) |
| `pg_stat_bgwriter.checkpoints_timed`, `checkpoints_req`, `checkpoint_write_time`, `checkpoint_sync_time`, `buffers_checkpoint` | `pg_stat_checkpointer.num_timed`, `num_requested`, `write_time`, `sync_time`, `buffers_written` |
| `pg_stat_statements.blk_read_time` / `blk_write_time` | `shared_blk_read_time` / `shared_blk_write_time` (plus new `local_blk_*_time`) |
| `pg_stat_progress_vacuum.max_dead_tuples`, `num_dead_tuples` | `max_dead_tuple_bytes`, `num_dead_item_ids`, `dead_tuple_bytes` |
| `pg_collation.colliculocale`, `pg_database.daticulocale` | `colllocale`, `datlocale` |
| `'1 day ago 2 hours'::interval` | `ago` must be last: `'2 hours 1 day ago'` |
| `wal_sync_method = fsync_writethrough` on Windows | `fsync` |
| `pgbench -d` meaning "debug" | `-d` is now the database name; debug is `--debug` |
| `pg_statistic_ext.stxstattarget = -1` for "default" | Now NULL, like `pg_attribute.attstattarget` |
| `pg_stat_slru` column names, and the names accepted by `pg_stat_reset_slru()` | Renamed in 17; check monitoring queries against the 17 view |
| `information_schema.element_types.domain_default` | Removed |
| `pgrowlocks` lock mode labels | Output labels changed; parsers of its output need updating |
| `pg_walfile_name()` / `pg_walfile_name_offset()` on an exact segment boundary | Now return the current segment, not the previous one (`pg_walfile_name('0/1000000')` is `...0001` on 17, `...0000` on 16). WAL archiving scripts that compensated for the old behaviour are off by one |
| `SET SESSION AUTHORIZATION` checking superuser status as of connection time | Now checks the session user's superuser status when the command runs |
| Interval input in which an empty (implicit) unit appears more than once | Rejected |

Already gone before 17, and still common in old code: `lc_collate`/`lc_ctype` as `SHOW`-able settings (removed in 16; use `SELECT datcollate FROM pg_database WHERE datname = current_database()`), exclusive backup mode `pg_start_backup()`/`pg_stop_backup()` (removed in 15; use `pg_backup_start()`/`pg_backup_stop()`), `pg_stat_statements.total_time` (renamed `total_exec_time` in 13), `CREATE` on `public` for all users (revoked by default for new databases since 15), PL/Python 2 (removed in 15).

***

#### Features from PostgreSQL 14–16 You Can Rely On

All of these exist on 17 and should be used where they fit. The version marker shows when each arrived. The marker was confirmed by running the feature on the version before it, where it fails.

##### Since 14

- **SQL-standard function bodies**: `CREATE FUNCTION f(int) RETURNS int LANGUAGE sql BEGIN ATOMIC SELECT $1 + 1; END;`. The body is parsed at creation time and dependencies are tracked, so `DROP TABLE` refuses to drop a table a function depends on.
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

- **`MERGE`** (17 adds `RETURNING`, `NOT MATCHED BY SOURCE` and views).
- **`UNIQUE NULLS NOT DISTINCT`**.
- **`security_invoker` views**: `CREATE VIEW v WITH (security_invoker = true) AS ...`.
- **`CREATE` on schema `public` is revoked from `PUBLIC`** in newly created databases (`public` is owned by `pg_database_owner`). Databases upgraded with `pg_upgrade` keep their old ACL.
- `regexp_count()`, `regexp_instr()`, `regexp_like()`, `regexp_substr()`.
- `ON DELETE SET NULL (col, ...)` / `SET DEFAULT (col, ...)` with column lists.
- `numeric` with negative scale or scale greater than precision.
- `log_destination = jsonlog`, `GRANT SET/ALTER SYSTEM ON PARAMETER`, the `pg_checkpoint` role.
- Logical replication row filters and column lists.

##### Since 16

- **SQL/JSON constructors and predicates**: `JSON_OBJECT()`, `JSON_ARRAY()`, `JSON_OBJECTAGG()`, `JSON_ARRAYAGG()`, `IS JSON [VALUE | OBJECT | ARRAY | SCALAR] [WITH UNIQUE KEYS]`.
- `pg_input_is_valid()`, `pg_input_error_info()` for checking input without raising errors.
- `any_value()` aggregate.
- Numeric literals with underscores (`1_000_000`) and non-decimal integer literals (`0x1F`, `0o17`, `0b101`).
- `array_sample()`, `array_shuffle()`, `random_normal()`, `date_add()`/`date_subtract()` for `timestamptz` with a time zone argument, `SYSTEM_USER`.
- `pg_stat_io`, `pg_stat_*_tables.last_seq_scan` / `last_idx_scan`.
- `GRANT role TO user WITH INHERIT FALSE, SET FALSE` (role membership options), and tighter `CREATEROLE` rules.
- `COPY ... FROM ... (DEFAULT '\D')`.
- `VACUUM (BUFFER_USAGE_LIMIT ..., SKIP_DATABASE_STATS, ONLY_DATABASE_STATS, PROCESS_MAIN ...)`.
- `EXPLAIN (GENERIC_PLAN)` for statements with `$1` placeholders.
- Logical decoding on standbys, parallel apply.

***

#### New in 17: SQL/JSON Query Functions and JSON_TABLE

##### JSON_TABLE

`JSON_TABLE` replaces long chains of `jsonb_array_elements()` and `->>` casts. Each column has a type, a path and error handling.

```sql
SELECT jt.*
FROM JSON_TABLE(
  '{"items":[{"sku":"a1","qty":2,"price":"9.50"},{"sku":"b2","qty":"x"}]}'::jsonb,
  '$.items[*]' COLUMNS (
    ord       FOR ORDINALITY,
    sku       text          PATH '$.sku',
    qty       int           PATH '$.qty' DEFAULT 0 ON ERROR,
    price     numeric(10,2) PATH '$.price',
    has_price boolean       EXISTS PATH '$.price'
  )) AS jt;
--  ord | sku | qty | price | has_price
--    1 | a1  |   2 |  9.50 | t
--    2 | b2  |   0 |       | f
```

Nested arrays use `NESTED PATH`. A parent row with an empty child array still appears once, with NULL child columns, which is outer-join semantics:

```sql
SELECT * FROM JSON_TABLE(
  '{"orders":[{"id":1,"lines":[{"sku":"a","q":2},{"sku":"b","q":1}]},{"id":2,"lines":[]}]}'::jsonb,
  '$.orders[*]' COLUMNS (
    order_id int PATH '$.id',
    NESTED PATH '$.lines[*]' COLUMNS (sku text PATH '$.sku', q int PATH '$.q')
  )) jt;
--  order_id | sku | q
--         1 | a   | 2
--         1 | b   | 1
--         2 |     |
```

`JSON_TABLE` is a table function, so it can reference earlier `FROM` items without writing `LATERAL`: `FROM orders o, JSON_TABLE(o.doc, '$.lines[*]' COLUMNS (...)) l`.

**Pitfall: errors become NULL silently.** By default each column is `NULL ON EMPTY NULL ON ERROR`. A table-level `ERROR ON ERROR` (after `COLUMNS (...)`) applies to the row path only. It does **not** change column behaviour, as tested:

```sql
-- Still returns one row with a = NULL: the bad value is swallowed
SELECT * FROM JSON_TABLE('{"a":"x"}'::jsonb, '$'
  COLUMNS (a int PATH '$.a') ERROR ON ERROR) jt;

-- Raises: invalid input syntax for type integer: "x"
SELECT * FROM JSON_TABLE('{"a":"x"}'::jsonb, '$'
  COLUMNS (a int PATH '$.a' ERROR ON ERROR)) jt;
```

When the JSON comes from an external system and bad data must be detected, add `ERROR ON ERROR` **to each column** (or an explicit `DEFAULT ... ON ERROR` that can be found later). Flag ingestion code that relies on the defaults and then treats NULL as "absent".

##### JSON_VALUE, JSON_QUERY, JSON_EXISTS

```sql
SELECT JSON_VALUE(jsonb '{"a":{"b":42}}', '$.a.b' RETURNING int)          AS v,   -- 42
       JSON_QUERY(jsonb '{"a":[1,2]}', '$.a')                              AS q,   -- [1, 2]
       JSON_QUERY(jsonb '{"a":[1,2]}', '$.a[*]' WITH WRAPPER)              AS qw,  -- [1, 2]
       JSON_EXISTS(jsonb '{"a":1}', '$.b')                                 AS e;   -- f

-- PASSING binds jsonpath variables: never build jsonpath strings by concatenation
SELECT JSON_VALUE(jsonb '{"a":1}', '$.a ? (@ > $min)' PASSING 0 AS min RETURNING int);  -- 1

SELECT JSON_VALUE(jsonb '{}', '$.missing' DEFAULT 'n/a' ON EMPTY);  -- n/a
```

Behaviour verified on 17.11 that reviewers must know:

| Expression | Result | Why |
|---|---|---|
| `JSON_VALUE(jsonb '{"a":"x"}', '$.a' RETURNING int)` | NULL | Default is `NULL ON ERROR` |
| `... RETURNING int ERROR ON ERROR` | error `invalid input syntax for type integer: "x"` | |
| `JSON_VALUE(jsonb '{"d":"2024-02-30"}', '$.d' RETURNING date)` | NULL | Invalid date is swallowed |
| `JSON_VALUE(jsonb '{"a":[1,2]}', '$.a')` | NULL | `JSON_VALUE` only returns scalars |
| `JSON_QUERY(jsonb '{"a":[1,2]}', '$.a[*]')` | NULL | Several items without `WITH WRAPPER` is an error, and the error becomes NULL |
| `JSON_QUERY(jsonb '{"a":"x"}', '$.a')` | `"x"` (with quotes) | `JSON_QUERY` returns JSON |
| `JSON_QUERY(jsonb '{"a":"x"}', '$.a' OMIT QUOTES)` | NULL | `x` is not valid `jsonb`, so the error becomes NULL |
| `JSON_QUERY(jsonb '{"a":"x"}', '$.a' RETURNING text OMIT QUOTES)` | `x` | |
| `JSON_EXISTS(jsonb '{"a":[1,2,3]}', 'strict $.a[5]')` | `f` | `JSON_EXISTS` defaults to `FALSE ON ERROR` |

Rules of thumb:

- Use `JSON_VALUE(... RETURNING type)` for scalars and `JSON_QUERY` for objects and arrays.
- Add `ERROR ON ERROR` wherever silently getting NULL would corrupt results (imports, validation, financial data).
- Bind values with `PASSING`. Don't splice user input into the jsonpath text.

**Indexing**: `JSON_EXISTS(doc, path)` is **not** indexable by a GIN index. The planner shows `Seq Scan ... Filter: JSON_EXISTS(...)`, while `doc @? path` with the same path uses the `jsonb_path_ops` GIN index (see [JSONB](#jsonb)). Prefer the operators `@>`, `@?` and `@@` in hot `WHERE` clauses, and use the SQL/JSON functions for extracting values.

##### Constructors

```sql
SELECT JSON('{"a":1,"a":2}');                    -- {"a":1,"a":2}  (type json, keeps duplicates)
SELECT JSON('{"a":1,"a":2}' WITH UNIQUE KEYS);   -- ERROR: duplicate JSON object key value
SELECT JSON_SCALAR(date '2026-10-22');           -- "2026-10-22" (a json string)
SELECT JSON_SERIALIZE('{"a":1}'::json RETURNING bytea);
```

On 16, `JSON('...')` parses as a function-style cast to `json`. Only clauses such as `WITH UNIQUE KEYS` (a syntax error on 16) and `JSON_SCALAR`/`JSON_SERIALIZE` are new in 17.

##### New jsonpath item methods

```sql
SELECT jsonb_path_query('"2024-09-26"', '$.date()'),     -- "2024-09-26"
       jsonb_path_query('"12.345"', '$.decimal(5,2)'),   -- 12.35
       jsonb_path_query('"7"', '$.integer()'),           -- 7
       jsonb_path_query('1', '$.boolean()');             -- true
```

On 16 these fail with `syntax error at or near "(" of jsonpath input`.

***

#### New in 17: MERGE Improvements

```sql
CREATE TABLE stock(sku text PRIMARY KEY, qty int NOT NULL);
INSERT INTO stock VALUES ('a',1),('b',2),('z',9);

MERGE INTO stock s
USING (VALUES ('a',5),('c',3)) AS v(sku, qty) ON s.sku = v.sku
WHEN MATCHED THEN UPDATE SET qty = s.qty + v.qty
WHEN NOT MATCHED THEN INSERT VALUES (v.sku, v.qty)
WHEN NOT MATCHED BY SOURCE AND s.sku <> 'z' THEN DELETE
RETURNING merge_action(), s.*;
--  merge_action | sku | qty
--  UPDATE       | a   |   6
--  DELETE       | b   |   2
--  INSERT       | c   |   3
```

- `merge_action()` is only valid in a `MERGE ... RETURNING` list.
- In `RETURNING`, target columns show **new** values for `INSERT`/`UPDATE` and **old** values for `DELETE`. `RETURNING *` returns the **source columns followed by the target columns**. Use explicit lists in application code.
- `WHEN NOT MATCHED BY SOURCE` without an extra condition deletes or updates **every** target row that has no source row. That is a full-table sync, which is rarely what an incremental feed wants. Look for a guard condition (`AND s.batch_id = ...`) or a pre-filtered target (a CTE or a view as the target).
- The target can be a view if it is automatically updatable with no `INSTEAD OF` triggers, or if it has `INSTEAD OF` triggers for every action used. Views with rules, materialized views and foreign tables are not supported.
- **Concurrency**: `MERGE` is **not** an atomic upsert. With concurrent inserts of the same key, the `WHEN NOT MATCHED THEN INSERT` branch can raise a unique violation. For "insert or update one row by key" under concurrency, use `INSERT ... ON CONFLICT DO UPDATE`.
- If one target row matches several source rows, a repeated `UPDATE`/`DELETE` raises a cardinality violation and a repeated `INSERT` raises a unique violation. Deduplicate the source (`DISTINCT ON`) first.
- Not available in 17: `RETURNING old.*, new.*` (18).

***

#### New in 17: COPY ON_ERROR and Related Options

```sql
CREATE TABLE ci(a int, b date);
COPY ci FROM STDIN WITH (FORMAT csv, ON_ERROR ignore, LOG_VERBOSITY verbose);
1,2024-01-01
x,2024-01-02
3,notadate
4,2024-01-04
\.
-- NOTICE:  skipping row due to data type incompatibility at line 2 for column "a": "x"
-- NOTICE:  skipping row due to data type incompatibility at line 3 for column "b": "notadate"
-- NOTICE:  2 rows were skipped due to data type incompatibility
-- COPY 2
```

What `ON_ERROR ignore` does **not** cover (tested):

- Constraint violations (`CHECK`, `NOT NULL`, unique, foreign key) still abort the whole `COPY`.
- Malformed rows, such as `extra data after last expected column`, still abort.
- `FORMAT binary` rejects it: `only ON_ERROR STOP is allowed in BINARY mode`.
- There is no limit option in 17 (`REJECT_LIMIT` is 18). The command succeeds even if 99% of rows were dropped, so code must check the skipped count (the `NOTICE`, or `pg_stat_progress_copy.tuples_skipped` while it runs) and decide whether to commit. For strict loads, keep the default `ON_ERROR stop`.

Other 17 additions: `FORCE_NULL *` and `FORCE_NOT_NULL *` apply to every column in CSV mode. On 16, `FORCE_NULL *` fails with `argument to option "force_null" must be a list of column names`.

Server-side `COPY ... FROM '/path'` reads files on the **database server** and needs superuser or `pg_read_server_files`. Application code should use `COPY ... FROM STDIN` (driver copy APIs) or psql's `\copy`. Flag server-side file paths and `COPY ... TO PROGRAM` in application code as security problems.

**Not available in 17**: `REJECT_LIMIT` and `LOG_VERBOSITY silent` (both 18).

***

#### New in 17: Partitioning Features

##### Identity columns on partitioned tables

```sql
CREATE TABLE ev(
  id bigint GENERATED ALWAYS AS IDENTITY,
  at date NOT NULL,
  PRIMARY KEY (id, at)
) PARTITION BY RANGE (at);
CREATE TABLE ev_2024 PARTITION OF ev FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
INSERT INTO ev(at) VALUES ('2024-05-01') RETURNING id;   -- 1
```

On 16 the parent accepts an identity column, but its partitions do not inherit it: inserts through the parent work, while a direct `INSERT INTO` a partition fails with a not-null violation. On 16, use a sequence default on every partition if rows are written to partitions directly. In 17, a table that is attached with `ATTACH PARTITION` **must not have its own identity column**. A staging table created with `LIKE parent INCLUDING ALL` copies the identity and then fails: `table "m_2026" being attached contains an identity column "id"`. Use `LIKE parent INCLUDING DEFAULTS INCLUDING CONSTRAINTS` (without `INCLUDING IDENTITY`/`ALL`), or `CREATE TABLE ... PARTITION OF`.

##### Exclusion constraints on partitioned tables

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;
-- OK: the partition key (at) is part of the constraint with equality
CREATE TABLE booking(room int, at date, during tstzrange,
  EXCLUDE USING gist (room WITH =, at WITH =, during WITH &&)) PARTITION BY RANGE (at);

-- ERROR: unique constraint on partitioned table must include all partitioning columns
CREATE TABLE booking2(room int, at date, during tstzrange,
  EXCLUDE USING gist (room WITH =, during WITH &&)) PARTITION BY RANGE (at);
```

On 16: `exclusion constraints are not supported on partitioned tables`. The constraint is enforced per partition only, which is why the partition key must be compared with `=`. A booking that spans two partitions is not protected.

***

#### New in 17: DDL Additions

```sql
-- Change the expression of a STORED generated column (rewrites the table under ACCESS EXCLUSIVE)
CREATE TABLE g(a int, b int GENERATED ALWAYS AS (a*2) STORED);
ALTER TABLE g ALTER COLUMN b SET EXPRESSION AS (a*3);

-- Reset a per-column statistics target (stored as NULL in 17)
ALTER TABLE g ALTER COLUMN a SET STATISTICS DEFAULT;

-- Enum values added in the same transaction as CREATE TYPE are usable
BEGIN;
CREATE TYPE mood AS ENUM ('a');
ALTER TYPE mood ADD VALUE 'b';
SELECT 'b'::mood;    -- 17: works. 16: ERROR: unsafe use of new value "b" of enum type mood
COMMIT;
```

`SET EXPRESSION` was measured to take `ACCESS EXCLUSIVE` and assign a new relfilenode, which means a full rewrite. Treat it like a column type change in migrations. Values added to an *existing* enum inside a transaction still cannot be used until that transaction commits.

`pg_attribute.attstattarget` is now NULL for "default" (it was `-1`). Monitoring queries that test `attstattarget = -1` silently stop matching.

***

#### New in 17: MAINTAIN Privilege, pg_maintain and the Maintenance search_path

##### MAINTAIN

```sql
GRANT MAINTAIN ON TABLE orders TO ops_role;   -- per table
GRANT pg_maintain TO ops_role;                -- all relations
```

`MAINTAIN` allows `VACUUM`, `ANALYZE`, `CLUSTER`, `REFRESH MATERIALIZED VIEW`, `REINDEX` and `LOCK TABLE`. It does **not** allow reading data. A `pg_maintain` member got `permission denied for table acct` on `SELECT` after a successful `VACUUM`/`ANALYZE`/`REINDEX`. Use it instead of making job roles table owners or superusers. On 16: `unrecognized privilege type "maintain"` and `role "pg_maintain" does not exist`.

##### Safe search_path during maintenance (breaking change)

`ANALYZE`, `CLUSTER`, `CREATE INDEX`, `CREATE MATERIALIZED VIEW`, `REFRESH MATERIALIZED VIEW`, `REINDEX` and `VACUUM` now run functions with `search_path = pg_catalog, pg_temp`.

```sql
CREATE TABLE lookup(k int PRIMARY KEY, v int);
CREATE FUNCTION f_lookup(int) RETURNS int LANGUAGE sql IMMUTABLE
  AS 'SELECT v FROM lookup WHERE k = $1';
CREATE TABLE mt(a int);
INSERT INTO mt VALUES (1);

CREATE INDEX ON mt (f_lookup(a));
-- 17: ERROR:  relation "lookup" does not exist
--     CONTEXT:  SQL function "f_lookup" during inlining
-- 16: CREATE INDEX succeeds

-- Fix: pin the search_path on the function (or schema-qualify every reference)
CREATE FUNCTION f_lookup2(int) RETURNS int LANGUAGE sql IMMUTABLE
  SET search_path = public
  AS 'SELECT v FROM lookup WHERE k = $1';
CREATE INDEX ON mt (f_lookup2(a));   -- OK
```

`CREATE MATERIALIZED VIEW mv AS SELECT f_lookup(a) FROM mt` fails the same way. Review every function used in an expression index, an index predicate, a check constraint evaluated by maintenance, or a materialized view: it needs `SET search_path = ...` or fully qualified names. Existing indexes and materialized views from an upgraded cluster keep working for ordinary `INSERT`/`UPDATE` and plain `VACUUM`, but fail later: measured on 17.11, `ANALYZE` (including autovacuum's auto-analyze and `VACUUM (ANALYZE)`), `REINDEX`, `VACUUM FULL`/`CLUSTER` and `REFRESH MATERIALIZED VIEW` raise `relation "lookup" does not exist`. On 17 even `CREATE INDEX` on an empty table fails, because the function is inlined at plan time. (The example function also lies about being `IMMUTABLE`, because it reads a table. See [Functions](#functions-procedures-and-triggers).)

***

#### New in 17: Server Settings Application Code Touches

##### transaction_timeout

```sql
SET transaction_timeout = '500ms';
BEGIN; SELECT pg_sleep(1); COMMIT;
-- FATAL:  terminating connection due to transaction timeout
-- server closed the connection unexpectedly
```

`transaction_timeout` limits the total duration of a transaction, including a single-statement implicit transaction. **It terminates the session**, like `idle_in_transaction_session_timeout`. It does not just cancel the statement, as `statement_timeout` does. Consequences for review:

- Setting it in a pooled connection kills the server connection. Clients must handle connection loss and retry the whole transaction.
- Set it per role or per database (`ALTER ROLE app SET transaction_timeout = '5min'`) for interactive or OLTP roles. Do not set it globally, because it would also kill legitimate long batch and migration transactions.
- If `statement_timeout` or `idle_in_transaction_session_timeout` is greater than or equal to `transaction_timeout`, the longer one never fires. Keep `statement_timeout < transaction_timeout`.

##### Other 17 settings

- `allow_alter_system = off` disables `ALTER SYSTEM` (for config-managed clusters). It is a convenience, not a security boundary: superusers can still change the configuration in other ways.
- `event_triggers = off` disables event triggers temporarily. This is useful when a broken `ON login` trigger locks everyone out.
- `summarize_wal` (default `off`) must be `on` for incremental backups.
- `io_combine_limit` (default 128kB) is the streaming I/O read size.
- `vacuum_buffer_usage_limit` defaults to 2MB (256kB in 16).
- `huge_pages_status` reports whether huge pages are actually in use.

##### Builtin collation provider

```sql
CREATE COLLATION c_utf8 (provider = builtin, locale = 'C.UTF-8');
-- or for the whole database:
-- CREATE DATABASE app LOCALE_PROVIDER builtin BUILTIN_LOCALE 'C.UTF-8' TEMPLATE template0;
```

The builtin `C.UTF-8` sorts by code point but classifies characters with Unicode rules (`upper()`, regex character classes). It does not depend on glibc or ICU versions, so OS upgrades cannot silently corrupt text indexes, a real risk with libc collations (for example the glibc 2.28 change). It also makes `LIKE 'prefix%'` index-friendly with a plain B-tree index. In a `C.UTF-8` builtin database, the plan was `Bitmap Index Scan on u_email_idx  Index Cond: ((email >= 'user12') AND (email < 'user13'))`, while the same query under `en_US.utf8` was a sequential scan. The trade-off is that the ordering is not linguistic: `'a' < 'B'` is false. Character classification still works (`upper('é')` is `É`, and `'é' ~ '^[[:alpha:]]$'` is true). Changing the collation of an existing database is a dump/restore, so this is a decision for new databases.

***

#### New in 17: Performance Changes Visible in Plans

The plans below are from 17.11 with `EXPLAIN (COSTS OFF)`, compared with 16.15 on the same data.

##### B-tree IN lists / = ANY(array)

```sql
-- 500,000-row table, bigint primary key
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF, SUMMARY OFF)
SELECT count(*) FROM orders WHERE id IN (1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20);
-- 17:  Index Only Scan using orders_pkey ... Buffers: shared hit=4
-- 16:  Index Only Scan using orders_pkey ... Buffers: shared hit=64
```

17 reads a leaf page once for all array keys that fall on it. Large `IN` lists and `= ANY($1::bigint[])` batch lookups are cheaper than on 16. The node shape does not reveal it (the comparison above used `enable_bitmapscan = off` so that both versions chose the same index-only scan; with default settings 17 picked a bitmap scan on this data), so only `BUFFERS` shows the difference. Still prefer `= ANY($1)` with an array parameter over generating `IN ($1, $2, ... $n)`. The array form gives one prepared statement for every list length and also works with an empty list (`= ANY('{}')` matches nothing). An empty `IN ()` is a syntax error.

##### NULL tests on NOT NULL columns

```sql
EXPLAIN (COSTS OFF) SELECT * FROM orders WHERE status IS NULL;   -- status is NOT NULL
-- 17:  Result
--        One-Time Filter: false
-- 16 (same shape, NOT NULL column a):  Seq Scan on nn
--                                       Filter: (a IS NULL)
```

##### Correlated IN subqueries become semi-joins

```sql
EXPLAIN (COSTS OFF)
SELECT * FROM cust c
WHERE c.id IN (SELECT o.customer_id FROM orders o WHERE o.total > c.region);
-- 17:  Hash Semi Join
--        Hash Cond: (c.id = o.customer_id)
--        Join Filter: (o.total > (c.region)::numeric)
-- 16:  Seq Scan on cust c
--        Filter: (SubPlan 1)
```

Older advice to rewrite such queries by hand as `EXISTS` is less important on 17, but `EXISTS` is still the clearer form.

##### No skip scan in 17

```sql
CREATE INDEX ON sk(a, b);    -- a has 4 distinct values
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF, SUMMARY OFF) SELECT * FROM sk WHERE b = 500;
-- 17:  Seq Scan on sk  Filter: (b = 500)  Rows Removed by Filter: 199999
-- 18:  Index Only Scan using sk_a_b_idx on sk  Index Cond: (b = 500)
```

On 17, a multicolumn B-tree index is only efficient when the query constrains its **leading** column(s). Don't accept "the composite index covers it" for a query that filters only on a later column. You can add a leading-column constraint (`a IN (...)`, `a = ANY(...)`) or another index.

##### Vacuum and I/O

These are not visible in plans but matter for capacity planning. `maintenance_work_mem`/`autovacuum_work_mem` above 1 GB now actually help vacuum on huge tables, and vacuum needs far fewer index passes. Sequential scans and `ANALYZE` use larger combined reads.

***

#### New in 17: EXPLAIN and Monitoring

```sql
EXPLAIN (ANALYZE, SERIALIZE, MEMORY, COSTS OFF, TIMING OFF, SUMMARY OFF) SELECT * FROM stock;
--  Seq Scan on stock (actual rows=3 loops=1)
--  Planning:
--    Memory: used=7kB  allocated=8kB
--  Serialization: output=1kB  format=text
```

- `SERIALIZE` measures the cost of converting the result to text or binary for the client. Wide `jsonb`/TOASTed columns that `EXPLAIN ANALYZE` alone hides become visible.
- `MEMORY` reports planner memory use.
- `BUFFERS` output now includes local-buffer I/O timing.
- `pg_stat_checkpointer` has the checkpoint columns that moved out of `pg_stat_bgwriter` (`num_timed`, `num_requested`, `write_time`, `sync_time`, `buffers_written`). `pg_wait_events` lists all wait events with descriptions.
- `pg_stat_statements` 1.11 renames `blk_read_time`/`blk_write_time` to `shared_blk_read_time`/`shared_blk_write_time`, adds `local_blk_*_time`, `stats_since`, `minmax_stats_since`, and replaces `CALL` arguments, savepoint names and 2PC GIDs with placeholders.

***

#### New in 17: Connections, libpq, Backup and Replication

- **`sslnegotiation=direct`** (libpq 17 and server 17): the client starts TLS immediately with ALPN `postgresql`, which saves a round trip. libpq refuses weak modes: `weak sslmode "disable" may not be used with sslnegotiation=direct (use "require", "verify-ca", or "verify-full")`. Only use it when every server the client talks to, including poolers and proxies, is 17+ and understands direct TLS. Otherwise keep the default `sslnegotiation=postgres`.
- **Cancel requests** can be encrypted with the new libpq cancel API (`PQcancelCreate()` and related). The legacy `PQcancel()` sends the cancel key in clear text.
- **Incremental backup**: enable `summarize_wal`, take a full `pg_basebackup`, then `pg_basebackup --incremental=/path/backup_manifest`. Restore with `pg_combinebackup full incr1 incr2 -o restored`. An incremental backup is useless without its whole chain. Backup scripts must keep and verify every link (`pg_verifybackup`).
- **Failover slots**: `pg_create_logical_replication_slot(..., failover => true)` or `CREATE SUBSCRIPTION ... WITH (failover = true)`, plus `sync_replication_slots = on` on the standby and `synchronized_standby_slots` on the primary. Logical replication can then continue after a physical failover.
- **`pg_createsubscriber`** converts a physical standby into a logical subscriber.
- **`pg_upgrade`** keeps logical slots and subscriptions only when the *old* cluster is 17 or later. An upgrade from 16 to 17 still needs slots to be recreated.

***

#### Schema Migrations and Locking

This is the most important review area for production PostgreSQL. A migration that is correct but takes the wrong lock can take an application down.

##### Lock levels of common DDL (measured on 17.11 with pg_locks)

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
| `VALIDATE CONSTRAINT` | `SHARE UPDATE EXCLUSIVE` (and `ROW SHARE` on the referenced table) | no | no | full scan |
| `ALTER COLUMN SET STATISTICS` | `SHARE UPDATE EXCLUSIVE` | no | no | none |
| `CREATE INDEX` | `SHARE` | no | **yes**, for the whole build | full scan |
| `CREATE INDEX CONCURRENTLY` | `SHARE UPDATE EXCLUSIVE` | no | no | two scans and waits for old transactions |
| `ALTER COLUMN ... SET EXPRESSION` (17) | `ACCESS EXCLUSIVE` | yes | yes | **rewrite** |
| `ATTACH PARTITION` | `SHARE UPDATE EXCLUSIVE` on parent, `ACCESS EXCLUSIVE` on the attached table and on the **default partition** | parent: no | parent: no | scans the new partition unless a matching `CHECK` exists. Also scans the default partition |
| `DETACH PARTITION ... CONCURRENTLY` (14+) | `SHARE UPDATE EXCLUSIVE` | no | no | not allowed inside a transaction block, or when a default partition exists |
| `VACUUM FULL`, `CLUSTER` | `ACCESS EXCLUSIVE` | yes | yes | rewrite |
| plain `REINDEX INDEX` | `SHARE` on the table, `ACCESS EXCLUSIVE` on the index | queries that would use the index block | yes | rebuilds the index |

The rewrite facts above were measured by comparing `pg_relation_filenode()` before and after each statement. Source for lock modes: the "Explicit Locking" and `ALTER TABLE` documentation.

##### The lock queue: why "fast" DDL still causes outages

Even a metadata-only `ALTER TABLE` needs `ACCESS EXCLUSIVE`. It waits behind any transaction that has touched the table, even one holding only an `ACCESS SHARE` lock from a `SELECT`. **While it waits, every new query on the table queues behind it.** Measured on 17.11: with session R holding a read transaction and session A waiting on `ALTER TABLE q ADD COLUMN x int`, a plain `SELECT count(*) FROM q` in a third session blocked until its `lock_timeout` fired (`canceling statement due to lock timeout`).

Every migration session must set a short `lock_timeout` and retry:

```sql
-- BAD: can wait forever and block all traffic on the table meanwhile
ALTER TABLE orders ADD COLUMN note text;

-- GOOD: fail fast, retry from the migration tool with backoff
SET lock_timeout = '3s';
SET statement_timeout = '15min';   -- upper bound for scans, validations, etc.
ALTER TABLE orders ADD COLUMN note text;
```

`SET LOCAL` limits the setting to the current transaction. It was verified that `lock_timeout` reverted to `0` after `COMMIT`. Migration frameworks that wrap each migration in a transaction should use `SET LOCAL`, so the setting does not leak into a pooled connection.

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
ALTER TABLE t ADD CONSTRAINT t_v_not_null CHECK (v IS NOT NULL) NOT VALID;  -- instant
ALTER TABLE t VALIDATE CONSTRAINT t_v_not_null;  -- scans under SHARE UPDATE EXCLUSIVE; writes continue
ALTER TABLE t ALTER COLUMN v SET NOT NULL;       -- no scan: proven by the valid CHECK
ALTER TABLE t DROP CONSTRAINT t_v_not_null;      -- optional cleanup
```

With `client_min_messages = debug1`, 17 confirms the skip: `existing constraints on column "t.v" are sufficient to prove that it does not contain nulls`. (On 18 you could add a `NOT NULL ... NOT VALID` constraint directly. Do not suggest that for 17.)

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
- A `USING` clause does not force a rewrite by itself: measured on 17.11, `TYPE text USING v::text` (from `varchar`) and `TYPE int USING n` kept the relfilenode. Any `USING` expression that actually changes the stored value, or a type change that is not binary-coercible, rewrites the table and its indexes.

##### Indexes in migrations

```sql
-- BAD on a live table: blocks INSERT/UPDATE/DELETE for the whole build
CREATE INDEX orders_status_idx ON orders(status);

-- GOOD
CREATE INDEX CONCURRENTLY orders_status_idx ON orders(status);
```

`CREATE INDEX CONCURRENTLY` rules that reviewers must enforce (all verified):

- It **cannot run inside a transaction block**: `ERROR: CREATE INDEX CONCURRENTLY cannot run inside a transaction block`. Migration tools that wrap migrations in a transaction need a per-migration opt-out (for example a "no transaction" flag).
- A failure (deadlock, unique violation, cancellation) leaves an **INVALID index** behind. It still slows down writes and is never used for reads. **`CREATE INDEX CONCURRENTLY IF NOT EXISTS` then skips silently** (`NOTICE: relation "dup_a" already exists, skipping`) and leaves the invalid index in place. Idempotent migrations must check `pg_index.indisvalid` and `DROP INDEX CONCURRENTLY` before retrying.
- It cannot be used on a partitioned parent (`cannot create index on partitioned table "m" concurrently`). Pattern: `CREATE INDEX idx ON ONLY parent (...)` (an invalid shell index), then `CREATE INDEX CONCURRENTLY` on each partition and `ALTER INDEX idx ATTACH PARTITION part_idx`. The parent index becomes valid when all partitions are attached (verified).
- It waits for all transactions that could use the old snapshot. A long-running transaction stalls it.
- `DROP INDEX CONCURRENTLY` and `REINDEX ... CONCURRENTLY` (12+) are the non-blocking counterparts.
- Add a primary key or unique constraint without a long lock by building the unique index concurrently and then attaching it: `ALTER TABLE pkt ADD CONSTRAINT pkt_pkey PRIMARY KEY USING INDEX pkt_id;`. If a key column is still nullable, the command sets `NOT NULL` itself, which scans the whole table under `ACCESS EXCLUSIVE` (17.11 logs `verifying table`). Make the columns `NOT NULL` first with the `CHECK ... NOT VALID` pattern above. The index is renamed to the constraint name.

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
- `DROP COLUMN` is metadata-only, but it still takes `ACCESS EXCLUSIVE`.
- Avoid `VACUUM FULL`/`CLUSTER` on live tables. They hold `ACCESS EXCLUSIVE` for the whole rewrite. Use `pg_repack` or `pg_squeeze` if bloat really requires it.
- `CREATE TYPE ... AS ENUM` changes: `ALTER TYPE ... ADD VALUE` is transactional since 12 (a `ROLLBACK` removed the new value on 17.11), but the new value cannot be used in the same transaction (except when the type itself was created in that transaction, 17). Removing or renaming enum values needs a new type. Consider a lookup table or `text` with a `CHECK` constraint for value sets that change often.
- `REFRESH MATERIALIZED VIEW` holds `ACCESS EXCLUSIVE` and blocks readers. `REFRESH ... CONCURRENTLY` needs a unique index on the view (a plain unique index, without a `WHERE` clause).
- Changing a function used by an index (`CREATE OR REPLACE FUNCTION`) does not rebuild the index. If the results change, the index is silently wrong until `REINDEX`.

***

#### Indexing

##### Index types

| Type | Use for | Notes |
|---|---|---|
| B-tree (default) | `=`, `<`, `>`, `BETWEEN`, `IN`/`= ANY`, `ORDER BY`, `IS NULL`, prefix `LIKE 'x%'` (with `C`/`C.UTF-8` collation or `text_pattern_ops`) | Only efficient when the leading columns are constrained (no skip scan in 17) |
| Hash | `=` only | WAL-logged and crash-safe since 10. Rarely better than B-tree |
| GIN | `jsonb` (`@>`, `?`, `?|`, `?&`, `@?`, `@@`), arrays (`@>`, `&&`), full-text `tsvector`, `pg_trgm` (`LIKE '%x%'`, `ILIKE`, `~`) | Slower writes; `fastupdate` pending list; no parallel build in 17 (18 adds it) |
| GiST | ranges, geometry, exclusion constraints, `pg_trgm`, nearest-neighbour `ORDER BY <->` | |
| SP-GiST | non-balanced data (IP prefixes, quadtrees, text prefixes) | |
| BRIN | very large, append-only tables where the column correlates with physical order (timestamps) | Tiny; parallel build is new in 17 |

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

- **The predicate must match the index expression exactly.** Measured: `WHERE date_trunc('day', created_at) = '2024-02-01'` and `WHERE customer_id::text = '42'` both lead to a sequential scan even with an index on the column. Rewrite as a range (`created_at >= '2024-02-01' AND created_at < '2024-02-02'`, giving `Index Scan using orders_created_idx`) or compare with the column's own type.
- **Type mismatches defeat indexes.** `WHERE id = 42.0` on a `bigint` column is planned as `(id)::numeric = 42.0`, a sequential scan. Drivers that send parameters as `numeric` or `text` cause the same problem. Bind parameters with the column's type.
- **Expression indexes need IMMUTABLE functions.** `CREATE INDEX ON ev2 (date(created_at))` on a `timestamptz` column fails with `functions in index expression must be marked IMMUTABLE`, because the date depends on the session time zone. Use `((created_at AT TIME ZONE 'UTC')::date)` and query with exactly that expression.
- **Prefix `LIKE` and collations.** With the default `en_US.utf8` database collation, `email LIKE 'user12%'` did not use a plain B-tree index (seq scan). A `text_pattern_ops` index, or a column/database using `C`/`C.UTF-8`, gives `Index Cond: ((email ~>=~ 'user12') AND (email ~<~ 'user13'))`. Leading-wildcard `LIKE '%x'` needs `pg_trgm` (GIN or GiST).
- **Escape `LIKE` input.** `_` and `%` in user input are wildcards: `'abc' LIKE 'a_c'` is true. Escape user input (`replace(replace(replace(x, '\', '\\'), '%', '\%'), '_', '\_')`) or use `starts_with()` / `left(col, n) = $1` when only a prefix test is needed.
- **Every foreign key column used for joins or cascades needs an index** (PostgreSQL never creates one automatically).
- **Partial indexes** must have a predicate that the query implies literally (`WHERE active` in both). For `ON CONFLICT` on a partial unique index, the statement must repeat the predicate: `ON CONFLICT (user_id) WHERE active DO NOTHING`. Without it: `there is no unique or exclusion constraint matching the ON CONFLICT specification`.
- **Multicolumn order**: put equality columns first, then the range or sort column. `(status, created_at)` serves `WHERE status = $1 ORDER BY created_at DESC LIMIT 20`.
- **Over-indexing**: each index slows writes, prevents HOT updates on its columns, and adds vacuum and WAL work. Look for unused indexes (`pg_stat_user_indexes.idx_scan = 0` over a representative period, and on replicas too) and duplicate prefixes (`(a)` next to `(a, b)`).
- **Unique constraints and NULLs**: `UNIQUE (a, b)` allows any number of rows with `b IS NULL` (two `(1, NULL)` rows were inserted). Use `UNIQUE NULLS NOT DISTINCT` (15+) when NULL should count as a value.
- **Case-insensitive uniqueness**: `CREATE UNIQUE INDEX ... (lower(email))` plus `ON CONFLICT (lower(email))`. Queries must use `lower(email) = lower($1)`.

***

#### Query Performance and EXPLAIN

##### Reading plans

```sql
EXPLAIN SELECT ...;                                   -- estimated plan, does not run
EXPLAIN (ANALYZE, BUFFERS) SELECT ...;                -- runs the query; real rows, timing, buffers
EXPLAIN (ANALYZE, BUFFERS, VERBOSE, SETTINGS) SELECT ...;
EXPLAIN (ANALYZE, BUFFERS, SERIALIZE) SELECT ...;     -- 17: include output conversion cost
EXPLAIN (GENERIC_PLAN) SELECT ... WHERE id = $1;      -- 16+: plan for a parameterized query
```

- In 17, `BUFFERS` must be requested explicitly with `ANALYZE` (18 adds it automatically).
- **`EXPLAIN ANALYZE` executes the statement.** `EXPLAIN ANALYZE DELETE FROM ex` really deleted the row (the count afterwards was 0). Wrap DML in `BEGIN; EXPLAIN ANALYZE ...; ROLLBACK;`.
- Compare estimated `rows=` with `actual rows=`. Errors of 10x or more point to stale or insufficient statistics, correlated columns (`CREATE STATISTICS ... (dependencies, ndistinct, mcv)`) or non-sargable predicates.
- Node names: `Seq Scan`, `Index Scan`, `Index Only Scan` (check `Heap Fetches:`, which should be near 0 after vacuum), `Bitmap Heap Scan`, `Nested Loop` (good for few outer rows), `Hash Join`, `Merge Join`, `Memoize` (14+), `Incremental Sort` (13+), `Gather`/`Parallel ...`.
- A `Seq Scan` is not a defect in itself. On small tables, or when most rows are returned, it is the right plan.

##### Pagination

```sql
-- BAD on large offsets: reads and discards every skipped row
SELECT id FROM orders ORDER BY id LIMIT 20 OFFSET 400000;
-- Index Only Scan ... (actual rows=400020 loops=1)   <- 400k rows read to return 20

-- GOOD: keyset pagination with a row-value comparison and a matching composite index
CREATE INDEX ord_created_id ON ord (created_at, id);
SELECT id, created_at FROM ord
WHERE (created_at, id) < ($1, $2)
ORDER BY created_at DESC, id DESC
LIMIT 20;
-- Limit -> Index Only Scan Backward using ord_created_id
--            Index Cond: (ROW(created_at, id) < ROW(...))
```

The row-value form is important. The equivalent expanded `created_at < $1 OR (created_at = $1 AND id < $2)` was planned as a filter, not an index condition. Without the `(created_at, id)` index, the row comparison was only a `Filter` on top of a `created_at` index scan with an incremental sort. The sort direction of all key columns must be the same (all `DESC` or all `ASC`) for a backward scan to work.

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
- Correlated subqueries in the select list run once per row. Prefer a `JOIN` + `GROUP BY` or `LEFT JOIN LATERAL` when they are expensive.
- **CTEs** are inlined since 12 when they are referenced once and have no side effects. `WITH c AS (SELECT * FROM ev) SELECT * FROM c WHERE id = 5` is planned as a direct scan with `Filter: (id = 5)`. `WITH c AS MATERIALIZED (...)` forces an optimization fence (`CTE Scan`). Use it deliberately, not out of habit from pre-12 advice. Data-modifying CTEs (`WITH x AS (DELETE ... RETURNING ...)`) always run exactly once.
- `ORDER BY random() LIMIT n` reads and sorts every row (a top-N sort over a full scan). Use `TABLESAMPLE SYSTEM (p)` or a keyed random pick for large tables.
- `DISTINCT` used to hide duplicate rows from a wrong join is a bug, not an optimization.

##### Prepared statements and generic plans

```sql
-- skew: k = 1 in 99% of rows
PREPARE q(int) AS SELECT count(*) FROM skew WHERE k = $1;
EXPLAIN (COSTS OFF) EXECUTE q(1);      -- Seq Scan, Filter: (k = 1)
EXPLAIN (COSTS OFF) EXECUTE q(99999);  -- Index Only Scan using skew_k_idx
```

With `plan_cache_mode = auto` (the default), the first five executions are planned with the actual values. After that the server may switch to a generic plan if its estimated cost is not worse on average. For heavily skewed columns this can lock in a bad plan. Fix it per role/session with `plan_cache_mode = force_custom_plan`, or restructure the query. Don't disable prepared statements in the driver globally.

##### Statistics and maintenance

- Run `ANALYZE` after bulk loads and after creating expression indexes (the expression gets its own statistics).
- Raise per-column statistics targets for skewed columns: `ALTER TABLE t ALTER COLUMN c SET STATISTICS 1000` (reset with `SET STATISTICS DEFAULT` in 17).
- Extended statistics for correlated columns: `CREATE STATISTICS s (dependencies, mcv) ON city, zip FROM addresses; ANALYZE addresses;`.
- `pg_upgrade` to 17 does **not** carry optimizer statistics over. Run `vacuumdb --all --analyze-in-stages` right after upgrading, or plans will be bad.

***

#### Transactions, Isolation and Concurrency

##### Isolation levels (all behaviour verified with two sessions on 17.11)

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
-- measured result: 50, correct result: 20

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

**Write skew** (two on-call doctors both go off duty) is prevented only by `SERIALIZABLE`. In the test, the second commit failed with `could not serialize access due to read/write dependencies among transactions`. Code that uses `REPEATABLE READ` or `SERIALIZABLE` **must have a retry loop** for SQLSTATE `40001` (and `40P01` deadlock). Without one, the higher isolation level only turns data corruption into user-visible errors.

##### Row locks and queues

```sql
-- Job queue: each worker takes different rows without blocking
SELECT id FROM jobs WHERE NOT done ORDER BY id LIMIT 10 FOR UPDATE SKIP LOCKED;
```

- `SKIP LOCKED` is for queues. It returns an inconsistent view, so never use it for general reads.
- `NOWAIT` fails immediately (`could not obtain lock on row in relation "jobs"`). `lock_timeout` also applies to row locks (`canceling statement due to lock timeout ... while locking tuple`).
- Use `FOR NO KEY UPDATE` when the key is not changed. It does not conflict with foreign key checks (`FOR KEY SHARE`) from inserts into child tables.
- Deadlocks: lock rows in a consistent order (`ORDER BY id ... FOR UPDATE`), and keep transactions short.
- **Advisory locks**: prefer `pg_advisory_xact_lock()`/`pg_try_advisory_xact_lock()`, which are released at commit or rollback. Session-level `pg_advisory_lock()` leaks across transaction-pooled connections and must be paired with `pg_advisory_unlock()` in every code path.

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
- Prefer `INSERT ... ON CONFLICT` over `MERGE` for concurrent single-key upserts (see [MERGE](#new-in-17-merge-improvements)).

##### Timeouts and long transactions

- Long-running and idle-in-transaction sessions block vacuum cluster-wide (bloat, transaction ID wraparound risk), and they block DDL.
- Set per role: `statement_timeout`, `idle_in_transaction_session_timeout`, `idle_session_timeout` (14+), and `transaction_timeout` (17, terminates the session). All default to 0 (disabled), as verified with `SHOW`.
- Never keep a transaction open across network calls, user think-time or message-queue waits.
- Sequences are non-transactional. A rolled-back insert consumes a value (measured: after a rollback the next id skipped one). Never assume identity values are gap-free or ordered by commit time.

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

In application code: always use driver parameters (`$1`, `?`, named binds). Identifiers (table names, `ORDER BY` columns, sort direction) cannot be parameters. Validate them against an allow-list. Don't "escape" them by hand. Never build `IN (...)` lists by string joining. Use `= ANY($1)`.

##### SECURITY DEFINER functions

```sql
-- BAD: runs as the owner, but resolves unqualified names using the caller's search_path
CREATE FUNCTION app.n_unsafe() RETURNS bigint LANGUAGE plpgsql SECURITY DEFINER AS $$
BEGIN
  RETURN (SELECT count(*) FROM app.secrets WHERE lower(s) = lower(s));
END $$;

-- attacker (has CREATE on schema evil):
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

Every `SECURITY DEFINER` function needs `SET search_path` with trusted schemas only and `pg_temp` **last**, schema-qualified object references, and `REVOKE EXECUTE ... FROM PUBLIC`. Functions are executable by `PUBLIC` by default. To stop new functions from being executable by `PUBLIC`, run `ALTER DEFAULT PRIVILEGES [FOR ROLE owner] REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC` **without** `IN SCHEMA`: per-schema default privileges can only add to the global defaults, so the `IN SCHEMA` form of this revoke has no effect. It applies only to functions later created by that role. Owners should be dedicated, non-superuser roles.

##### Roles and privileges

- The application connects as a role that **does not own** the schema objects and is not a superuser. Migrations run as a separate owner role.
- Since 15, new databases do not let `PUBLIC` create objects in `public`. A non-owner role got `permission denied for schema public`. Don't "fix" this with `GRANT CREATE ON SCHEMA public TO PUBLIC`. Create an application schema owned by the migration role.
- Use predefined roles instead of superuser: `pg_read_all_data`/`pg_write_all_data` (14), `pg_monitor`, `pg_signal_backend`, `pg_checkpoint` (15), `pg_create_subscription` (16), `pg_maintain` (17), and `GRANT MAINTAIN ON TABLE` (17).
- Role membership options (16): `GRANT app TO bob WITH INHERIT FALSE, SET FALSE` makes bob a member without inheriting privileges (`pg_has_role('bob','app','USAGE')` is false).
- Passwords: `scram-sha-256` (default since 14). `md5` still works on 17. 18 deprecates it, so don't add new `md5` entries. Never put passwords in SQL that gets logged (`ALTER ROLE ... PASSWORD 'x'` appears in the server log when statement logging is on, and in `pg_stat_activity` while it runs). Use `\password` or `PQchangePassword()` (libpq 17), which send a hashed password.

##### Row-level security

```sql
ALTER TABLE docs ENABLE ROW LEVEL SECURITY;
ALTER TABLE docs FORCE ROW LEVEL SECURITY;   -- also apply to the table owner
CREATE POLICY tenant_isolation ON docs
  USING (tenant = current_setting('app.tenant', true))
  WITH CHECK (tenant = current_setting('app.tenant', true));
```

Behaviour verified on 17.11:

- **Superusers and roles with `BYPASSRLS` always bypass RLS**, even with `FORCE`.
- Without `FORCE ROW LEVEL SECURITY`, the **table owner bypasses** policies (a non-superuser owner saw all rows). With `FORCE` the owner saw 0 rows with no tenant set.
- A `USING` policy also checks inserts when `WITH CHECK` is omitted: `new row violates row-level security policy for table "docs"`.
- `current_setting('app.tenant', true)` returns NULL when the setting was never defined, and the policy then matches nothing (safe). `current_setting('app.tenant')` without `true` raises `unrecognized configuration parameter`. **After a `set_config('app.tenant', 't2', true)` transaction ends, the setting reads as `''` (empty string), not NULL.** Policies must not treat `''` as a valid tenant.
- With transaction pooling, set the tenant with `SET LOCAL` / `set_config(..., true)` **inside each transaction**. A session-level `SET` leaks to the next client that gets the connection.
- Views run with the **view owner's** privileges and bypass RLS for the owner unless they are created `WITH (security_invoker = true)` (15+). In the test, a definer view returned all rows to an unprivileged role, while the invoker view raised `permission denied for table docs`.
- Leakproof matters. User functions in policies or in queries on RLS tables may see rows before the policy filters them, unless they are `LEAKPROOF`. Keep policies simple and indexable.

##### Other security items

- `COPY ... FROM/TO 'file'` and `COPY ... PROGRAM` run on the server with the server's OS user (superuser or `pg_read_server_files`/`pg_write_server_files`/`pg_execute_server_program`). Treat them as remote code execution if inputs reach them.
- `dblink`/`postgres_fdw` connection strings built from user input are injection and SSRF vectors.
- Keep the server on the latest 17.x. 17.11 alone fixed 28 CVEs, including memory-safety bugs in `to_char()`, regexp functions and `pg_stat_statements`. Earlier 17.x releases fixed CVE-2025-1094 (libpq quoting functions with invalidly encoded input, 17.3) and CVE-2025-8713/8714/8715 (statistics leak and `pg_dump`/psql restore injection, 17.6).

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

- Use **`timestamptz`** for points in time. It stores UTC and converts on input and output using the session `TimeZone`. Plain `timestamp` drops offsets on input and is only right for "wall clock" values (for example a store's opening hours).
- `now()::timestamp AT TIME ZONE 'UTC'` is a common bug. It first converts to session-local wall time and then interprets that as UTC. With a Berlin session it produced a value two hours off. Use `now() AT TIME ZONE 'UTC'` (gives `timestamp`) or keep `timestamptz`.
- `now()`/`CURRENT_TIMESTAMP` is the **transaction start time**. All rows in one transaction get the same value. Use `clock_timestamp()` for wall time inside long transactions.
- `timestamp(0)` **rounds**: `'2024-01-01 23:59:59.7'::timestamp(0)` gives `2024-01-02 00:00:00`.
- `date_bin('15 minutes', ts, origin)` (14+) buckets times. `AT LOCAL` (17) converts with the session time zone.
- `interval` supports `'infinity'` in 17. `ago` must be the last token (17).

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

##### Text

- Use `text` (or `varchar` without a length). `varchar(n)` only adds a length check. It is not faster, and changing `n` later can need a rewrite (narrowing) or an `ACCESS EXCLUSIVE` lock. Use a `CHECK (length(x) <= n)` constraint if a business rule needs a limit.
- **Never `char(n)`**: it pads with spaces and ignores trailing spaces when comparing (`'ab '::char(5) = 'ab'::char(5)` is true, `length()` is 2, `octet_length()` is 5). Plain `varchar` treats `'ab '` and `'ab'` as different.
- An explicit cast to `varchar(n)` **silently truncates** (`'abcdef'::varchar(3)` gives `abc`). Inserting a too-long value into a `varchar(3)` column raises `value too long`.
- Case-insensitive matching: `lower(x)` with an expression index, `citext`, or a nondeterministic ICU collation (on 17, `LIKE` with a nondeterministic collation raises `nondeterministic collations are not supported for LIKE`). No `casefold()` in 17.

##### Identity vs serial

```sql
-- GOOD
CREATE TABLE s2 (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, v int);
-- AVOID in new code
CREATE TABLE s1 (id serial PRIMARY KEY, v int);
```

Measured differences:

- `serial` allows explicit ids, which then collide with the sequence later (`duplicate key value violates unique constraint "s1_pkey"`). `GENERATED ALWAYS` rejects explicit values (`cannot insert a non-DEFAULT value into column "id"`) unless the statement uses `OVERRIDING SYSTEM VALUE`.
- With `serial`, a role that has `INSERT` on the table also needs `USAGE` on the separate sequence (`permission denied for sequence s1_id_seq`). Identity sequences need no extra grant.
- After bulk-loading explicit ids, reset the sequence: `SELECT setval(pg_get_serial_sequence('s2','id'), (SELECT max(id) FROM s2));`.
- Identity on partitioned tables works since 17 (see above).
- UUID keys: `gen_random_uuid()` (v4, core since 13) causes random B-tree inserts. That is fine for moderate sizes, but `uuidv7()` is **18 only**. Generate time-ordered UUIDs in the application if you need them on 17. `uuid_extract_version()`/`uuid_extract_timestamp()` (17) can inspect them.

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

Enums are compact and ordered, but values cannot be removed or renamed easily, and `ADD VALUE` cannot be used inside a transaction until it commits (except for a type created in the same transaction, 17). Use a `CHECK (status IN (...))` or a foreign key to a lookup table when the set changes often.

***

#### JSONB

##### json vs jsonb

```sql
SELECT '{"a":1,"a":2}'::jsonb;        -- {"a": 2}           (last key wins, keys sorted)
SELECT '{"b":1,"a":2}'::json;         -- {"b":1,"a":2}      (text kept verbatim)
SELECT '{"n":1.0}'::jsonb = '{"n":1}'::jsonb;   -- t
```

Use `jsonb` for storage and querying. Use `json` only when the exact input text (key order, duplicates, whitespace) must be preserved.

##### Operators and extraction

```sql
SELECT doc -> 'a'   AS json_value,    -- jsonb
       doc ->> 'a'  AS text_value,    -- text
       doc['a']     AS subscript      -- jsonb (14+), needs parentheses around casts: ('{}'::jsonb)['a']
FROM t;

SELECT ('{"a":"5"}'::jsonb ->> 'a')::int + 1;   -- 6
SELECT ('null'::jsonb)::int;                     -- 17: ERROR cannot cast jsonb null to type integer (18: NULL)
SELECT '{"a":null}'::jsonb ->> 'a' IS NULL;      -- t: JSON null becomes SQL NULL via ->>
```

- `jsonb_set(doc, path, NULL)` returns **NULL for the whole document** (the function is strict). A measured `jsonb_set('{"a":1}', '{b}', to_jsonb(NULL::text))` returned NULL, and `UPDATE t SET doc = jsonb_set(doc, ..., $1)` with a NULL parameter wipes the column. Use `jsonb_set_lax(doc, path, value)` (default `'use_json_null'`), or `coalesce`.
- In 17, prefer `JSON_VALUE(doc, '$.a' RETURNING int ERROR ON ERROR)` for typed extraction when bad data must raise an error. `(doc->>'a')::int` also raises on bad data, but leaves the cast error without context.

##### Indexing JSONB (plans from 17.11)

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
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE JSON_EXISTS(doc, '$.user ? (@.id == 5)');
-- Seq Scan  (SQL/JSON functions are not GIN-indexable)

CREATE INDEX evj_type ON evj ((doc->>'type'));
EXPLAIN (COSTS OFF) SELECT count(*) FROM evj WHERE doc->>'type' = 'click';
-- Bitmap Index Scan on evj_type
```

- `jsonb_path_ops` is smaller and faster for `@>`, `@?` and `@@`. The default `jsonb_ops` also supports the key-existence operators `?`, `?|` and `?&`.
- For a frequently filtered scalar, use a B-tree expression index on `(doc->>'key')`, or better, promote it to a real column.
- JSONB is not a replacement for a schema. Fields that are always present, used in joins, constrained, or updated often belong in columns. Updating one key rewrites the whole (possibly TOASTed) document and its GIN entries.

##### SQL/JSON (16 + 17)

```sql
SELECT '{"a":1}' IS JSON OBJECT;                               -- 16+
SELECT pg_input_is_valid('[1', 'jsonb');                       -- 16+: f (no exception)
SELECT JSON_OBJECT('a': 1, 'b': NULL ABSENT ON NULL);          -- {"a" : 1}  (16+)
SELECT JSON_ARRAYAGG(x ORDER BY x) FROM (VALUES (2),(1)) v(x); -- [1, 2]     (16+)
-- 17: JSON_TABLE, JSON_VALUE, JSON_QUERY, JSON_EXISTS, JSON(), JSON_SCALAR(), JSON_SERIALIZE()
```

***

#### Partitioning

```sql
CREATE TABLE m (
  id bigint GENERATED ALWAYS AS IDENTITY,          -- identity on partitioned table: 17+
  at timestamptz NOT NULL,
  v  int,
  PRIMARY KEY (id, at)                              -- must include the partition key
) PARTITION BY RANGE (at);
CREATE TABLE m_2025 PARTITION OF m FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE m_def  PARTITION OF m DEFAULT;
```

Rules verified on 17.11:

- **Unique and primary keys must include all partition key columns**: `PRIMARY KEY (id)` on a table partitioned by `at` fails. Global uniqueness of `id` alone is not enforced, so don't assume it.
- **Pruning needs the raw partition key.** `WHERE at >= '2025-03-01' AND at < '2025-04-01'` scans only `m_2025`. `WHERE date_trunc('month', at) = '2025-03-01'` scans **every** partition. Stable expressions like `now() - interval '1 day'` are pruned at execution time (`EXPLAIN ANALYZE` shows `Subplans Removed: n`).
- **Default partitions are a trap.** `ATTACH PARTITION` for a new range takes `ACCESS EXCLUSIVE` on the default partition and scans it. If matching rows were already inserted there, the attach fails (`updated partition constraint for default partition "m_def" would be violated by some row`). `DETACH ... CONCURRENTLY` is not allowed while a default partition exists. Prefer creating future partitions ahead of time (cron, `pg_partman`) and alerting when the default partition receives rows.
- **Attaching without a long scan**: add a `CHECK` constraint on the staging table that matches the partition bounds (validated before attaching). `ATTACH PARTITION` then takes only `SHARE UPDATE EXCLUSIVE` on the parent. The staging table must not have an identity column (17, see above).
- **Indexes**: create indexes on the parent with `ON ONLY` + per-partition `CREATE INDEX CONCURRENTLY` + `ATTACH PARTITION` (see migrations).
- Retention: `ALTER TABLE m DETACH PARTITION m_2023 CONCURRENTLY; DROP TABLE m_2023;` instead of large `DELETE`s.
- Exclusion constraints (17) must include the partition key with `=`.
- In 17, `VACUUM`/`ANALYZE` on a partitioned parent process the partitions. Autovacuum never analyzes the **parent** itself, so run `ANALYZE parent` after large data changes if queries rely on parent-level statistics. (`VACUUM ONLY`/`ANALYZE ONLY` are 18.)
- Too many partitions (thousands) slow planning and locking. Partition when it helps retention, pruning or maintenance, not by default.

***

#### SQL Idioms and Pitfalls

All outputs in this section are from 17.11.

##### Aggregates and NULL

```sql
SELECT region,
       count(*) FILTER (WHERE amount > 20) AS big,    -- FILTER instead of sum(CASE ...)
       sum(amount)                          AS total,
       avg(amount)                          AS avg_ignores_null
FROM sales GROUP BY region;
-- region 's' has amounts 5 and NULL: total = 5, avg = 5.0000000000000000 (NULL rows are ignored)

SELECT pg_typeof(sum(1::int)), pg_typeof(sum(1::bigint)), pg_typeof(avg(1::int)), pg_typeof(count(*));
-- bigint | numeric | numeric | bigint

SELECT 'a' || NULL, concat('a', NULL), concat_ws(',', 'a', NULL, 'b');
-- NULL | a | a,b
```

- `||` with a NULL operand yields NULL. A name built as `first || ' ' || middle || ' ' || last` disappears when `middle` is NULL. Use `concat_ws()` or `coalesce`.
- `avg()` and `sum()` ignore NULLs. If NULL means zero, write `avg(coalesce(x, 0))`, and say so explicitly.
- `sum(int)` returns `bigint` and `sum(bigint)` returns `numeric`. Driver code that maps the result to a 32-bit integer can overflow or fail to decode.
- `string_agg(x, ',' ORDER BY x)` and `array_agg(x ORDER BY x)`: without `ORDER BY` inside the aggregate, the order is unspecified.
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
- **The default frame is `RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW`.** `last_value(amount) OVER (ORDER BY at)` returns the *current* row's value (`10, 30, 30`), not the partition's last value. Write `ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING`, or use `first_value(...) OVER (ORDER BY at DESC)`. With `RANGE`, peers (equal `ORDER BY` values) are included in running sums. Use `ROWS` for strictly row-by-row running totals.
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

`CROSS JOIN LATERAL` drops parents without children. `LEFT JOIN LATERAL (...) o ON true` keeps them with NULLs (verified). Choose deliberately.

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
SELECT to_tsquery('english', 'fox & (');                   -- ERROR: no operand in tsquery
SELECT websearch_to_tsquery('english', 'fox & (');         -- 'fox' (never raises on user input)
```

- Pass user search input to `websearch_to_tsquery()` or `plainto_tsquery()`, never to `to_tsquery()`. Malformed input raises errors (a 500 from the search box).
- Always pass the configuration (`'english'`) explicitly. The one-argument form uses `default_text_search_config` and is not immutable, so it cannot be used in an index expression. Queries must use the same configuration as the index.
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
```

On 17 every generated column must be `STORED` (virtual columns are 18). `INSERT INTO t VALUES (...)` without a column list breaks when a generated column is added. Always list the target columns. Changing the expression later needs `SET EXPRESSION` (17), which rewrites the table.

##### Materialized views

- `REFRESH MATERIALIZED VIEW` takes `ACCESS EXCLUSIVE` and blocks readers for the whole refresh.
- `REFRESH MATERIALIZED VIEW CONCURRENTLY` needs a **unique index without a `WHERE` clause**. With only a partial unique index it still fails: `cannot refresh materialized view "public.mv" concurrently`, `HINT: Create a unique index with no WHERE clause on one or more columns of the materialized view.` It cannot be used for the first population (`WITH NO DATA`), and it is slower than a full refresh when most rows change.
- A materialized view is a snapshot. Code that reads it must tolerate stale data, and something must schedule the refresh.
- Functions in materialized view queries are evaluated under the restricted maintenance `search_path` in 17 (see above).

##### Collation-dependent results

```sql
SELECT 'Z' < 'a' COLLATE "C"          AS c_coll,  -- t   (byte/code-point order)
       'Z' < 'a' COLLATE "en_US.utf8" AS en;      -- f   (linguistic order)
```

Sort order and range comparisons on text depend on the collation. Keyset pagination and range filters on text columns must use the same collation as the index, and tests run under a different database locale (for example `C` in CI, `en_US.utf8` in production) can hide ordering bugs.

***

#### Functions, Procedures and Triggers

- **Volatility labels are promises.** `IMMUTABLE` means "same result for the same arguments forever, no table access". A function that reads tables, depends on settings (`TimeZone`, `search_path`), or calls `now()` must be `STABLE` or `VOLATILE`. A mislabeled `IMMUTABLE` function in an index gives wrong query results after data changes.
- Prefer SQL-standard bodies (`BEGIN ATOMIC ... END`, 14+) for SQL functions. They are parsed at creation time, so dependencies are tracked and `search_path` is captured at creation (dropping a table used by such a function fails with `cannot drop table logs because other objects depend on it`).
- Set `search_path` on functions that use unqualified names: a must for `SECURITY DEFINER` and functions used by indexes, constraints and materialized views (17 maintenance behaviour).
- PL/pgSQL: `RAISE EXCEPTION` with `USING ERRCODE`, `GET STACKED DIAGNOSTICS` in exception handlers. `EXCEPTION` blocks create a subtransaction per execution, which is costly in loops and consumes subtransaction IDs.
- Procedures (`CREATE PROCEDURE`, `CALL`) can `COMMIT` inside, which suits batch jobs. They cannot when called inside an outer transaction block.
- Triggers: `BEFORE` row triggers for normalization and validation, `AFTER` for side effects. Avoid row triggers that query other large tables. Statement-level triggers with transition tables (`REFERENCING NEW TABLE AS ...`) are cheaper for bulk changes. `CREATE OR REPLACE TRIGGER` (14+).
- Event triggers: `ON login` (17) runs for every connection. A failing login trigger blocks logins. The recovery switch is `event_triggers = off` (17).
- `ALTER FUNCTION ... SET work_mem/plan_cache_mode` scopes settings to one function.

***

#### Connection Pooling and Application Access

- **Always pool.** Each PostgreSQL connection is a process with several MB of memory. Size pools to roughly a small multiple of CPU cores, not to the number of application threads. `max_connections` in the thousands is a design smell.
- **PgBouncer transaction mode** (the most common setup) forbids session state across transactions: `SET` without `LOCAL`, session advisory locks, `LISTEN`, `WITH HOLD` cursors, temporary tables, and SQL-level `PREPARE`/`EXECUTE`. Use `SET LOCAL`/`set_config(..., true)` inside transactions, and `pg_advisory_xact_lock`.
- **Protocol-level prepared statements** work in transaction mode with PgBouncer 1.21+ when `max_prepared_statements` is non-zero (default 200 in current releases). SQL `PREPARE` statements are still not tracked.
- `transaction_timeout` (17) and `idle_in_transaction_session_timeout` **terminate** the server connection. Pools must validate connections and drivers must retry.
- Set `application_name` per service so `pg_stat_activity` and the logs identify the caller.
- Retries: retry on `40001` (serialization), `40P01` (deadlock), and connection loss before commit. Never blindly retry after an unknown commit outcome for non-idempotent writes. Use idempotency keys.
- `sslmode=verify-full` for connections over untrusted networks. `sslnegotiation=direct` only for all-17+ server paths (see above).
- Use `COPY FROM STDIN` (via driver copy APIs) or multi-row `INSERT` for bulk writes instead of row-by-row inserts in a loop.

***

#### Maintenance, Monitoring and Observability

```sql
-- Slow statements (requires shared_preload_libraries = 'pg_stat_statements' + CREATE EXTENSION)
SELECT query, calls, total_exec_time, mean_exec_time, shared_blk_read_time  -- 17 column name
FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT 10;

-- Tables with heavy sequential scans (columns are relname, not tablename)
SELECT schemaname, relname, seq_scan, seq_tup_read, idx_scan, last_seq_scan, last_idx_scan
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

-- Checkpoints (17: pg_stat_checkpointer, not pg_stat_bgwriter)
SELECT num_timed, num_requested, write_time, sync_time, buffers_written FROM pg_stat_checkpointer;
```

- `pg_stat_user_tables` has `relname`; `pg_stat_user_indexes` has `relname` and `indexrelname`. Queries that use `tablename`/`indexname` (columns of `pg_tables`/`pg_indexes`) fail with `column "tablename" does not exist`.
- Old checkpoint queries against `pg_stat_bgwriter.checkpoints_timed` fail on 17 (`column "checkpoints_timed" does not exist`).
- Autovacuum: tune per table for large, hot tables (`ALTER TABLE t SET (autovacuum_vacuum_scale_factor = 0.01)`) rather than turning it off. Watch `n_dead_tup`, `last_autovacuum`, and `age(datfrozenxid)` for wraparound.
- Logging: `log_min_duration_statement`, `log_lock_waits = on`, `log_temp_files = 0`, `auto_explain` for slow plans. Logs contain query text and parameters, so treat them as sensitive.
- Plain `REINDEX` blocks writes. Use `REINDEX ... CONCURRENTLY` and only when bloat or corruption warrants it. 17.x release notes sometimes ask for specific reindexes (for example BRIN `numeric_minmax_multi_ops` indexes after 17.6, `btree_gist` float indexes with `NaN` and `ltree` indexes after 17.11).

***

#### Upgrading to 17 and Staying Current

##### Before upgrading from 14/15/16

1. Search for functions used in expression indexes, index predicates and materialized views without `SET search_path`/qualified names. They fail under the 17 maintenance `search_path`.
2. Search for interval literals with `ago` not at the end.
3. Update monitoring: `pg_stat_bgwriter` checkpoint columns → `pg_stat_checkpointer`, `buffers_backend` → `pg_stat_io`, `pg_stat_statements` `blk_*_time` → `shared_blk_*_time`, `pg_stat_progress_vacuum` renamed columns, `attstattarget` NULL semantics, `colliculocale`/`daticulocale` → `colllocale`/`datlocale`.
4. Remove `old_snapshot_threshold`, `db_user_namespace`, `trace_recovery_messages` from the configuration (the server refuses to start with unknown core settings in `postgresql.conf`).
5. Remove `adminpack` from extension lists and `pg_dump` restores.
6. After `pg_upgrade`: run `vacuumdb --all --analyze-in-stages` (statistics are not transferred) and recreate logical slots (slots are kept only when upgrading *from* 17+).
7. If the OS (glibc) changes at the same time, check libc-collated text indexes (`amcheck`/`pg_amcheck`) or reindex them. Consider the builtin `C.UTF-8` provider for new databases.

##### 17.x minor releases

Minor releases contain only fixes and need no dump/restore, but some require actions:

- **17.1 (November 14, 2024)**: four CVEs (CVE-2024-10976 RLS-dependent cached plans, CVE-2024-10977 libpq trusting pre-TLS error messages, CVE-2024-10978 `SET SESSION AUTHORIZATION`/`SET ROLE` interaction, CVE-2024-10979 trusted PL/Perl changing environment variables). A table that was detached from a partitioned table with a foreign key to another partitioned table, and not dropped, can have broken FK catalog state that must be repaired. Databases with `LC_CTYPE = C` but a different `LC_COLLATE` must reindex text indexes.
- **17.2 (November 21, 2024)** was an out-of-cycle release that restored extension binary compatibility (`ResultRelInfo`) broken by 17.1, and restored `ALTER ROLE/DATABASE ... SET role`.
- **17.3 (February 13, 2025)** fixed CVE-2025-1094 in libpq quoting functions (`PQescapeLiteral()` and friends now validate encoding).
- **17.4 (February 20, 2025)** was out of cycle: it fixed a regression in the 17.3 quoting fix (`PQescapeLiteral()`/`PQescapeIdentifier()` ignored their length argument).
- **17.5 (May 8, 2025)**: CVE-2025-4207 (GB18030 over-read). Self-referential foreign keys on partitioned tables may need to be recreated; BRIN bloom indexes should be reindexed.
- **17.6 (August 14, 2025)** fixed CVE-2025-8713 (statistics leak through views and partitions; permission errors can now appear at planning time), CVE-2025-8714/8715 (`pg_dump` output injection; psql gained `\restrict`). Reindex BRIN `numeric_minmax_multi_ops` indexes.
- **17.7 (November 13, 2025)**: CVE-2025-12817 (`CREATE STATISTICS` now checks `CREATE` on the schema) and CVE-2025-12818 (libpq allocation overflow).
- **17.8 (February 12, 2026)**: four CVEs (CVE-2026-2003 to 2006: `oidvector` checks, `intarray` selectivity estimator code execution, pgcrypto PGP buffer overrun, multibyte length validation). Attaching a non-built-in selectivity estimator to an operator now needs superuser, and string functions may now report `invalid byte sequence for encoding` for invalid text already stored. **17.9 (February 26, 2026)** was out of cycle and fixed regressions from 17.8 (multixact WAL replay on standbys, a too-strict `substring()` encoding check).
- **17.10 (May 14, 2026)**: ten CVEs, including quoting fixes in `pg_createsubscriber` and `ALTER SUBSCRIPTION ... REFRESH PUBLICATION`, path traversal in `pg_basebackup`/`pg_rewind`, and allocation overflows. libpq's `PQfn()` is now marked unsafe and deprecated.
- **17.11 (August 13, 2026)**: 28 CVE fixes. Logical decoding output plugins other than `pgoutput`/`test_decoding` must now be allow-listed with `output_plugin_libraries` (CVE-2026-6471), and `pg_upgrade --check` fails if the new cluster does not allow the plugins of existing slots. pgcrypto PGP decryption fails for data that was never actually encrypted because OpenSSL rejected the cipher (CVE-2026-14663; the `ignore-cipher-failure=1` option recovers such data). psql now skips the inline data of a failed `COPY ... FROM STDIN` (CVE-2026-6464): test scripts need a `\.` after deliberately failing COPY commands. `amcheck` now restricts `search_path` before running index expressions (CVE-2026-14673). It also recommends reindexing `btree_gist` float indexes that may contain `NaN` and `ltree` B-tree indexes with very long values, and fixes incremental-backup correctness (visibility-map clears were not WAL-logged) and a default-partition pruning bug that could drop rows.

Pin `postgres:17` (floating minor) or an exact `postgres:17.11` in containers, and update minors promptly.

***

#### Production / Review Checklist

##### Version compatibility

- [ ] No PostgreSQL 18+ features: `uuidv7()`, virtual generated columns (`GENERATED ALWAYS AS (...)` without `STORED`), `RETURNING old/new`, `WITHOUT OVERLAPS`/`PERIOD`, `NOT ENFORCED`, `NOT NULL ... NOT VALID`, `VACUUM/ANALYZE ONLY`, `COPY REJECT_LIMIT`, `casefold()`, `array_sort()`, int↔bytea casts
- [ ] Code shared with 16 does not use 17 features (`JSON_TABLE`, `JSON_VALUE`, `MERGE ... RETURNING`, `NOT MATCHED BY SOURCE`, `COPY ON_ERROR`, `random(min,max)`, `transaction_timeout`, `MAINTAIN`)
- [ ] No removed settings or columns (`old_snapshot_threshold`, `pg_stat_bgwriter.checkpoints_*`, `buffers_backend`, `blk_read_time`, `lc_collate`)
- [ ] Interval literals end with `ago`, if they use it at all

##### Migrations

- [ ] `lock_timeout` set (with retry) for every DDL on existing tables
- [ ] `CREATE INDEX CONCURRENTLY` / `DROP INDEX CONCURRENTLY` on live tables, outside transaction blocks; invalid indexes handled on retry
- [ ] Foreign keys and `CHECK` constraints added `NOT VALID` then `VALIDATE`d; `SET NOT NULL` preceded by a validated `CHECK (col IS NOT NULL)`
- [ ] No volatile defaults, type changes or `SET EXPRESSION` on large tables without a rewrite plan
- [ ] Backfills and deletes in batches; no `UPDATE`/`DELETE` of millions of rows in one transaction
- [ ] Expand/contract for renames and drops; enum changes considered irreversible
- [ ] Functions used by indexes or materialized views have `SET search_path` (17 maintenance change)
- [ ] Foreign key columns indexed

##### Queries

- [ ] Parameters bound (no string concatenation), `= ANY($1)` for lists, identifiers allow-listed
- [ ] Predicates sargable: no functions or casts on indexed columns, half-open time ranges, parameter types match column types
- [ ] Keyset pagination with a row-value comparison and a matching composite index for deep pages
- [ ] `NOT EXISTS` instead of `NOT IN (subquery)` on nullable columns
- [ ] Composite indexes match leading columns (no skip scan in 17)
- [ ] `EXPLAIN (ANALYZE, BUFFERS)` evidence for performance claims; DML explained inside `BEGIN ... ROLLBACK`

##### Transactions and concurrency

- [ ] Read-modify-write made atomic (`UPDATE ... SET x = x + ...`, `FOR UPDATE`, or version checks)
- [ ] Retry loops for `40001`/`40P01` with `REPEATABLE READ`/`SERIALIZABLE`
- [ ] Upserts use `INSERT ... ON CONFLICT` with an exact conflict target; inputs deduplicated
- [ ] `MERGE ... WHEN NOT MATCHED BY SOURCE` guarded so it cannot delete unrelated rows
- [ ] Short transactions; no network calls inside; timeouts set per role
- [ ] Queues use `FOR UPDATE SKIP LOCKED`; advisory locks are transaction-scoped

##### Security

- [ ] `SECURITY DEFINER` functions: `SET search_path = ..., pg_temp`, qualified names, `REVOKE EXECUTE ... FROM PUBLIC`
- [ ] Dynamic SQL uses `format('%I')` + `USING`, never `%s` with input
- [ ] App role is not owner or superuser; RLS tables use `FORCE ROW LEVEL SECURITY` where owners connect; tenant set with `SET LOCAL`; `''` not accepted as a tenant
- [ ] Views over RLS-protected or restricted tables use `security_invoker = true`
- [ ] No server-side `COPY` file/program from application paths; no passwords in logged SQL
- [ ] Server on the latest 17.x minor

##### Data types and schema

- [ ] `timestamptz` for instants; `bigint` identity for keys; `numeric` or integer minor units for money; `text` + `CHECK` instead of `char(n)`/arbitrary `varchar(n)`
- [ ] `GENERATED ALWAYS AS IDENTITY` instead of `serial`; sequences reset after explicit-id loads
- [ ] JSONB only for genuinely flexible data; GIN `jsonb_path_ops` for `@>`/`@?`; expression indexes for hot keys; `jsonb_set_lax` or NULL guards
- [ ] SQL/JSON functions use explicit `ERROR ON ERROR` where silent NULLs would be wrong
- [ ] Partitioned tables: keys include the partition key, queries filter on the raw key, future partitions pre-created, default partition monitored

##### Operations

- [ ] Pooling in place; no session state with transaction pooling
- [ ] `pg_stat_statements`, `log_lock_waits`, `log_min_duration_statement` configured; monitoring queries use 17 column names
- [ ] `ANALYZE` after bulk loads and after `pg_upgrade`
- [ ] Incremental backups (if used) keep and verify the full chain; `summarize_wal = on`
