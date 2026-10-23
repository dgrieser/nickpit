### PostgreSQL Style Guide

#### Overview

> **Scope**: SQL that targets **PostgreSQL** when the exact server version is not known: schema migrations and lock safety, data types, constraints, indexing, query writing, `EXPLAIN`, transactions and concurrency, upserts, JSONB, partitioning, security (roles, row-level security, `SECURITY DEFINER`, dynamic SQL), functions and triggers, vacuum and bloat, connection pooling, monitoring queries and upgrades.
> **Version stance**: the rules below hold on every supported release (14 to 18) unless a rule names a version. Every feature that depends on the server version carries the release that introduced it, written **(15+)**, **(18+)** and so on. **Before recommending a labelled feature, find the server version the project runs** (see "Version-aware review"). Never suggest it to a project that may run an older server.
> **Verification**: every example was executed on **PostgreSQL 18.6** (official `postgres:18` Docker image). Version labels of SQL-visible features from 10 to 18 were checked by running the feature on the release that introduced it and on the release before it (9.6.24, 10.21, 11.16, 12.22, 13.23, 14.24, 15.19, 16.15, 17.11, 18.6), and 19 features on 19rc1. Labels older than 10, labels of server-internal changes (planner, vacuum, tools, configuration), and behaviour that cannot run in a container (managed services, poolers, authentication methods) come from the sources cited in the footnotes.
> **Note**: This document is self-contained. When the version is known, the version-specific guide for that release gives more detail; this guide does not contradict them.

Global truths about PostgreSQL (all versions in scope):

- **MVCC.** Readers never block writers and writers never block readers at the row level. `UPDATE` writes a new row version and `DELETE` marks the old one dead; `VACUUM` (normally autovacuum) reclaims dead versions. A long-running transaction anywhere in the cluster stops that reclamation for every table.
- **DDL is transactional, but it takes table locks.** Most `ALTER TABLE` forms take `ACCESS EXCLUSIVE`, which conflicts with every other lock, including the one a plain `SELECT` takes. The lock is held until commit, and while it waits it blocks every later query on the table.
- **The default isolation level is READ COMMITTED.** Each statement sees a new snapshot. A read-modify-write in application code is a lost update unless the database makes it atomic.
- **NULL is three-valued.** `NULL = NULL` is NULL, `NOT IN` with a NULL in the list is never true, `count(col)` skips NULLs, and `sum()` over no rows is NULL.
- **Unquoted identifiers fold to lower case.** `CREATE TABLE "Users"` creates a table that `SELECT * FROM Users` cannot find. String literals use single quotes; double quotes are identifiers.
- **Arrays are 1-based.** `arr[0]` is NULL, not an error.
- **Defaults to reach for**: `timestamptz` for instants, `text` for strings, `bigint` identity (or `uuid`) keys, `numeric` or integer minor units for money, `jsonb` (not `json`) for documents.

***

#### Release Lines and Support (as of October 2026)

PostgreSQL ships one major version per year (normally in September or October) and supports each major for **five years**. Minor releases contain only fixes, appear at least once a quarter (the next one is scheduled for **November 12, 2026**), and need no dump/restore; some ask for follow-up actions such as a `REINDEX`.[^1][^2]

| Major | First release | Latest minor (date) | Status in October 2026 |
|---|---|---|---|
| 12 and older | October 3, 2019 (12.0) | 12.22 | **End of life** since November 21, 2024 |
| 13 | September 24, 2020 | 13.23 (November 13, 2025) | **End of life** since November 13, 2025 |
| 14 | September 30, 2021 | 14.24 (August 13, 2026) | Supported; **end of life on November 12, 2026** (its final minor release is due that day) |
| 15 | October 13, 2022 | 15.19 (August 13, 2026) | Supported until November 11, 2027 |
| 16 | September 14, 2023 | 16.15 (August 13, 2026) | Supported until November 9, 2028 |
| 17 | September 26, 2024 | 17.11 (August 13, 2026) | Supported until November 8, 2029 |
| **18** | **September 25, 2025** | **18.6 (August 13, 2026)** | **Current stable release**; supported until November 14, 2030. (18.5 was skipped: 18.4 is followed directly by 18.6.) |
| 19 | not released | 19 RC 1 (October 15, 2026) | **Release candidate, not a deployment target.** RC 2 is planned for November 12, 2026 and general availability for November 19, 2026[^3] |

Review consequences:

- Flag a project on 13 or older as running an unsupported server, and a project on 14 as due for an upgrade within weeks. Still judge the code against the version it actually runs.
- Flag pinned minors that are far behind the branch's latest (for example `postgres:16.2`): minor releases fix security vulnerabilities (the August 13, 2026 update releases alone fixed 28) and wrong-result bugs.[^4]
- Do not recommend 19 features (see "Not available in any released version").

***

#### Version-aware Review

##### Find the server version first

Look for it in, in this order: Docker image tags (`docker-compose.yml`, `Dockerfile`, CI `services:` blocks, `image: postgres:16-alpine`), Testcontainers code (`new PostgreSQLContainer("postgres:15.4")`), Helm values and Kubernetes manifests (including operator CRDs such as CloudNativePG `imageName` or `postgresVersion` fields), Terraform/CloudFormation (`engine_version = "16.3"` for RDS/Aurora, `database_version = "POSTGRES_17"` for Cloud SQL), `.tool-versions`/`asdf`/`mise` files, ORM and migration configuration (Django `DATABASES` with a minimum supported version, Rails `schema.rb`/`structure.sql` header `-- Dumped from database version 16.4`, Prisma/Sequelize settings), and `pg_dump` headers in checked-in SQL. At run time, `SELECT version()` or `SHOW server_version_num` (`180006` for 18.6) gives the answer; `current_setting('server_version_num')::int >= 150000` is the idiomatic test inside SQL.

```sql
SHOW server_version_num;                                   -- 180006 on 18.6
SELECT current_setting('server_version_num')::int >= 150000 AS has_merge;   -- t
```

- **A floating tag** (`postgres:17`, `postgres:17-alpine`) means the newest minor of that major; `postgres:latest` follows the newest major (18 today, 19 after November 19, 2026). Flag `latest` in anything but throwaway experiments: a major upgrade changes the on-disk format and the container fails to start on an existing data directory.
- **Managed services** (Amazon RDS and Aurora PostgreSQL, Google Cloud SQL and AlloyDB, Azure Database for PostgreSQL) run community majors with their own minor schedules, extension allow-lists, parameter defaults and missing superuser access. Aurora, AlloyDB, Neon, CockroachDB, YugabyteDB and Redshift speak the PostgreSQL protocol but are not PostgreSQL: their locking, isolation, DDL and feature sets differ. Check the platform's own documentation before applying a PostgreSQL rule to them.
- **Client libraries are versioned separately.** `libpq` options (for example `sslnegotiation=direct` in 17, `max_protocol_version` in 18) need a new enough libpq on the client and a server (and pooler) that understands them.

##### Rules for the reviewer

- **If the version is unknown, recommend only features available on 14** (the oldest supported major) and label every newer suggestion explicitly: "requires PostgreSQL 15+", "requires 18". For a library or extension that claims support for older servers, use its documented minimum.
- **Say "not available in PostgreSQL X" only when the table below or the documentation of that version proves it.** Many features are older than reviewers remember: `ON CONFLICT` and `SKIP LOCKED` are 9.5, identity columns 10, `INCLUDE` indexes 11, `REINDEX CONCURRENTLY` and stored generated columns 12, `gen_random_uuid()` in core 13.
- **Flag code that only runs on a newer server than the project pins** (for example `MERGE` on 14, `JSON_TABLE` on 16, `uuidv7()` on 17) and **code that relies on behaviour that changed** (table "Removed and changed behaviour across versions").
- **Same text, different meaning.** A few statements are valid on several versions but behave differently: `GENERATED ALWAYS AS (expr)` without `STORED` is a syntax error before 18 and a *virtual* (unindexable) column on 18; `SELECT 1_000` returns `1` aliased as `_000` on 14 and 1000 on 16+; an expression index on a function with unqualified table names builds on 16 and fails on 17. These deserve a comment even when the code "works".

##### Feature availability by version

Use this table to decide whether a suggestion is allowed. Each row lists what a release **added**; everything is available in all later releases unless the removal table says otherwise.[^5]

| Since | Features a reviewer commonly meets |
|---|---|
| 9.5 | `INSERT ... ON CONFLICT DO NOTHING / DO UPDATE`, `SELECT ... FOR UPDATE SKIP LOCKED`, row-level security (`CREATE POLICY`), BRIN indexes, `GROUPING SETS`/`CUBE`/`ROLLUP`, `jsonb_set()`, `jsonb` `||` and `-` operators |
| 9.6 | `idle_in_transaction_session_timeout`, parallel sequential scans, joins and aggregates, `phraseto_tsquery()` |
| 10 | Identity columns (`GENERATED ... AS IDENTITY`), declarative partitioning (`PARTITION BY RANGE / LIST`), crash-safe (WAL-logged) hash indexes, `CREATE STATISTICS`, built-in logical replication (`CREATE PUBLICATION` / `SUBSCRIPTION`), SCRAM-SHA-256 authentication |
| 11 | `ADD COLUMN ... DEFAULT <non-volatile>` without a table rewrite, covering indexes (`INCLUDE`), `CREATE PROCEDURE`/`CALL` with `COMMIT` inside, hash partitioning, default partitions, indexes and primary keys on partitioned tables, `websearch_to_tsquery()` |
| 12 | Stored generated columns (`GENERATED ALWAYS AS (...) STORED`), `REINDEX CONCURRENTLY`, CTE inlining and `WITH ... AS [NOT] MATERIALIZED`, SQL/JSON path language (`jsonb_path_query()`, `@?`, `@@`), foreign keys referencing partitioned tables, `ALTER TYPE ... ADD VALUE` inside a transaction block, `ATTACH PARTITION` under `SHARE UPDATE EXCLUSIVE` on the parent, `plan_cache_mode` |
| 13 | `gen_random_uuid()` in core (no `pgcrypto` needed), B-tree deduplication, incremental sort, `FETCH FIRST n ROWS WITH TIES`, `jsonb_set_lax()`, parallel vacuum of indexes, `pg_stat_statements.total_exec_time` (renamed from `total_time`), trusted extensions |
| 14 | Multirange types and `range_agg()`, SQL-standard function bodies (`RETURN expr`, `BEGIN ATOMIC ... END`), `jsonb` subscripting (`doc['a']`), `date_bin()`, `string_to_table()`, `SEARCH`/`CYCLE` in recursive CTEs, `GROUP BY DISTINCT`, `ALTER TABLE ... DETACH PARTITION ... CONCURRENTLY`, `REINDEX` of partitioned tables, `CREATE OR REPLACE TRIGGER`, `idle_session_timeout`, `pg_read_all_data`/`pg_write_all_data`/`pg_database_owner`, `password_encryption` default `scram-sha-256`, LZ4 TOAST compression |
| 15 | `MERGE`, `UNIQUE NULLS NOT DISTINCT`, views `WITH (security_invoker = true)`, `PUBLIC` loses `CREATE` on schema `public` in new databases, `regexp_count()`/`regexp_instr()`/`regexp_like()`/`regexp_substr()`, `ON DELETE SET NULL (col, ...)`, `GRANT SET ON PARAMETER`, `pg_checkpoint` role, `log_destination = jsonlog`, publication row filters and column lists |
| 16 | SQL/JSON constructors `JSON_OBJECT()`, `JSON_ARRAY()`, `JSON_OBJECTAGG()`, `JSON_ARRAYAGG()` and `IS JSON`, `any_value()`, numeric literals with underscores and `0x`/`0o`/`0b` integers, `pg_input_is_valid()`, `EXPLAIN (GENERIC_PLAN)`, `pg_stat_io`, `last_seq_scan`/`last_idx_scan` in `pg_stat_*_tables`, role membership options (`WITH INHERIT / SET / ADMIN`), restricted `CREATEROLE`, `date_add()`/`date_subtract()`, subqueries in `FROM` without an alias |
| 17 | `JSON_TABLE()`, `JSON_VALUE()`/`JSON_QUERY()`/`JSON_EXISTS()`, `MERGE ... RETURNING` with `merge_action()`, `WHEN NOT MATCHED BY SOURCE`, `MERGE` into views, `COPY ... (ON_ERROR ignore)`, `transaction_timeout`, `MAINTAIN` privilege and `pg_maintain` role, identity columns and exclusion constraints on partitioned tables, `ALTER COLUMN ... SET EXPRESSION`, `random(min, max)`, `uuid_extract_timestamp()`, builtin collation provider (`C.UTF-8`), `EXPLAIN (SERIALIZE, MEMORY)`, `pg_stat_checkpointer`, incremental backups (`pg_basebackup --incremental`) |
| 18 | `uuidv7()`/`uuidv4()`, **virtual generated columns (now the default kind)**, `RETURNING old.* / new.*`, temporal keys (`PRIMARY KEY (id, valid WITHOUT OVERLAPS)`, `FOREIGN KEY (..., PERIOD ...)`), `ADD CONSTRAINT ... NOT NULL ... NOT VALID`, `NOT ENFORCED` check/foreign-key constraints, B-tree skip scan, `EXPLAIN ANALYZE` includes `BUFFERS` by default, asynchronous I/O (`io_method`), `casefold()`, `array_sort()`, `COPY ... REJECT_LIMIT`, `VACUUM ONLY`/`ANALYZE ONLY`, OAuth authentication, data checksums on by default in `initdb` |

##### Not available in any released version

PostgreSQL 19 is at RC 1. The following work on 19rc1 and fail on 18.6 (verified); never suggest them for production code until the project runs 19:[^3]

| Feature | Error on 18.6 |
|---|---|
| `REPACK` / `REPACK (CONCURRENTLY)` (online replacement for `VACUUM FULL`/`CLUSTER`) | syntax error |
| `INSERT ... ON CONFLICT ... DO SELECT` | syntax error |
| `IGNORE NULLS` / `RESPECT NULLS` in `lag()`, `lead()`, `first_value()`, `last_value()`, `nth_value()` | syntax error |
| `WAIT FOR LSN ...` | syntax error |
| `COPY ... TO ... (FORMAT json)` | syntax error (`COPY format "json" not recognized` when quoted) |
| `error_on_null()`, `random(date, date)` | function does not exist |
| `EXPLAIN (ANALYZE, IO)` | unrecognized `EXPLAIN` option |

These are not in any version, including 19rc1 (verified): `ALTER TABLE ... MERGE PARTITIONS`/`SPLIT PARTITION`, `UPDATE/DELETE ... FOR PORTION OF`, `GROUP BY ALL`, `DELETE ... LIMIT`, `UPDATE ... LIMIT`, `CREATE INDEX CONCURRENTLY` on a partitioned table, unique constraints on a partitioned table that omit the partition key (global indexes), and query hints (only the third-party `pg_hint_plan` extension provides them).

##### Removed and changed behaviour across versions

Old code meets these when the server is upgraded. Each item names the release that changed it.[^5]

| Release | Change | Review consequence |
|---|---|---|
| 12 | `WITH OIDS` tables and `recovery.conf` removed; CTEs are inlined by default | Old DDL with `WITH OIDS` fails; CTEs used as an "optimization fence" need `AS MATERIALIZED` |
| 13 | `pg_stat_statements.total_time` → `total_exec_time` (plus `total_plan_time`) | Monitoring queries using `total_time` fail |
| 14 | `EXTRACT()` returns `numeric` instead of `double precision`; `password_encryption` defaults to `scram-sha-256` | Driver type mapping of `EXTRACT` results changes; MD5-hashed passwords stay MD5 until reset |
| 15 | `CREATE` on schema `public` revoked from `PUBLIC` in new databases (upgraded databases keep the old ACL); exclusive backup functions `pg_start_backup()`/`pg_stop_backup()` renamed `pg_backup_start()`/`pg_backup_stop()`; PL/Python 2 removed; numeric literals followed directly by letters (`123abc`) rejected | Deploy scripts that create objects in `public` as an unprivileged role fail on fresh databases; old backup scripts fail |
| 16 | `CREATEROLE` roles can only manage roles on which they have `ADMIN OPTION`; `lc_collate`/`lc_ctype` no longer `SHOW`-able; `vacuum_defer_cleanup_age` and `promote_trigger_file` removed | Provisioning scripts that alter roles they did not create fail |
| 17 | Maintenance commands (`ANALYZE`, `CREATE INDEX`, `REINDEX`, `REFRESH MATERIALIZED VIEW`, `VACUUM`, `CLUSTER`) run functions with `search_path = pg_catalog, pg_temp`; `old_snapshot_threshold` and `db_user_namespace` removed; checkpoint columns moved from `pg_stat_bgwriter` to `pg_stat_checkpointer`; `pg_stat_statements.blk_read_time` → `shared_blk_read_time`; `ago` must be the last token of an interval literal | Expression indexes and materialized views over functions with unqualified names fail to build or refresh; dashboards break |
| 18 | Generated columns are virtual unless `STORED` is written; MD5 password hashes deprecated (warning); `initdb` enables data checksums by default; `EXPLAIN ANALYZE` prints buffers and fractional row counts; `VACUUM`/`ANALYZE` recurse into inheritance children; unlogged partitioned tables rejected; `AFTER` triggers run as the role that queued the event; casting a `jsonb` `null` to a scalar type yields SQL `NULL` instead of an error; CSV `COPY FROM` no longer treats `\.` as end of data; `pg_stat_io.op_bytes` and `pg_stat_wal` write/sync columns removed | New DDL that omits `STORED` gets an unindexable column; plan parsers and dashboards break; `pg_upgrade` from a non-checksum cluster needs `initdb --no-data-checksums` |

***

#### Schema Migrations and Locking

Most production incidents caused by migrations are lock problems, not data problems. Review every migration against three questions: **which lock does each statement take, how long is it held, and does the statement rewrite or scan the table?**[^6][^7]

##### Lock levels of common DDL

Measured with `pg_locks` on 18.6; the lock modes are the same on every supported release unless a version is named.

| Statement | Lock on the table | Blocks reads? | Blocks writes? | Rewrite / scan |
|---|---|---|---|---|
| `ADD COLUMN` (no default, or a non-volatile default, 11+) | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none (catalog only) |
| `ADD COLUMN ... DEFAULT <volatile>` (e.g. `gen_random_uuid()`, `clock_timestamp()`) | `ACCESS EXCLUSIVE` | yes | yes | **rewrite** |
| `ALTER COLUMN ... TYPE` (not binary-compatible, e.g. `int` → `bigint`) | `ACCESS EXCLUSIVE` | yes | yes | **rewrite** + rebuild of every index |
| `ALTER COLUMN ... SET NOT NULL` | `ACCESS EXCLUSIVE` | yes | yes | full scan (skipped if a valid `CHECK (col IS NOT NULL)` proves it, 12+) |
| `DROP COLUMN`, `RENAME`, `SET DEFAULT`, `DROP DEFAULT` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none |
| `ADD CONSTRAINT ... CHECK (...) NOT VALID` | `ACCESS EXCLUSIVE` | yes (briefly) | yes | none |
| `ADD CONSTRAINT ... FOREIGN KEY ... NOT VALID` | `SHARE ROW EXCLUSIVE` on **both** tables | no | yes (briefly) | none |
| `VALIDATE CONSTRAINT` | `SHARE UPDATE EXCLUSIVE` (FK: plus `ROW SHARE` on the referenced table) | no | no | full scan |
| `CREATE INDEX` | `SHARE` | no | **yes, for the whole build** | full scan |
| `CREATE INDEX CONCURRENTLY`, `DROP INDEX CONCURRENTLY`, `REINDEX ... CONCURRENTLY` (12+) | `SHARE UPDATE EXCLUSIVE` | no | no | two scans; waits for older transactions |
| `DROP INDEX` (plain) | `ACCESS EXCLUSIVE` on the table | yes | yes | none |
| `REINDEX INDEX` (plain) | `SHARE` on the table, `ACCESS EXCLUSIVE` on the index | queries that use the index | yes | rebuild |
| `ALTER TABLE ... SET (autovacuum_...)`, `ALTER COLUMN ... SET STATISTICS` | `SHARE UPDATE EXCLUSIVE` | no | no | none |
| `VACUUM` (plain), `ANALYZE` | `SHARE UPDATE EXCLUSIVE` | no | no | — |
| `VACUUM FULL`, `CLUSTER`, `TRUNCATE`, `REFRESH MATERIALIZED VIEW` (plain) | `ACCESS EXCLUSIVE` | yes | yes | rewrite |
| `ATTACH PARTITION` (12+) | `SHARE UPDATE EXCLUSIVE` on the parent; `ACCESS EXCLUSIVE` on the attached table and on the default partition | parent: no | parent: no | scans the new partition unless a matching valid `CHECK` exists |
| `DETACH PARTITION ... CONCURRENTLY` (14+) | `SHARE UPDATE EXCLUSIVE` | no | no | — |

```sql
-- How the table above was measured: run the DDL in an open transaction and inspect pg_locks
CREATE TABLE lk (id int PRIMARY KEY, v int);
BEGIN;
ALTER TABLE lk ADD CONSTRAINT lk_v_pos CHECK (v > 0) NOT VALID;
SELECT mode FROM pg_locks WHERE relation = 'lk'::regclass AND pid = pg_backend_pid();
--  AccessExclusiveLock
COMMIT;
BEGIN;
ALTER TABLE lk VALIDATE CONSTRAINT lk_v_pos;
SELECT mode FROM pg_locks WHERE relation = 'lk'::regclass AND pid = pg_backend_pid();
--  ShareUpdateExclusiveLock
COMMIT;
```

##### The lock queue: why a "fast" ALTER takes the site down

An `ALTER TABLE` that needs `ACCESS EXCLUSIVE` waits behind every open transaction that has touched the table, even one that only ran a `SELECT`. **While it waits, every later query on that table queues behind it.** Measured on 18.6 with three sessions:

```text
session A: BEGIN; SELECT count(*) FROM lq; SELECT pg_sleep(8); COMMIT;   -- holds ACCESS SHARE
session B: ALTER TABLE lq ADD COLUMN x int;                                -- waits for A
session C: SET statement_timeout = '2s'; SELECT count(*) FROM lq;           -- waits behind B
           ERROR:  canceling statement due to statement timeout
pg_stat_activity: B and C wait_event_type = Lock; pg_blocking_pids(B) = {A}, pg_blocking_pids(C) = {B}
```

The `ALTER` itself is instantaneous; the outage is the queue. Every migration that touches a busy table must set a short `lock_timeout` and the migration runner must retry:

```sql
-- BAD: can wait indefinitely and block all traffic on the table meanwhile
ALTER TABLE orders ADD COLUMN note text;

-- GOOD: fail fast, retry with backoff from the migration tool
SET lock_timeout = '2s';
SET statement_timeout = '15min';   -- upper bound for scans and validations
ALTER TABLE orders ADD COLUMN note text;
-- ERROR:  canceling statement due to lock timeout   (when a long transaction holds the table)
```

`lock_timeout`, `statement_timeout`, `idle_in_transaction_session_timeout` (9.6+) and `transaction_timeout` (17+) all default to `0`, which means disabled. Inside a transaction use `SET LOCAL`, which reverts at commit and cannot leak into a pooled connection.

- **One risky statement per transaction.** Several `ACCESS EXCLUSIVE` statements in one transaction hold the strongest lock until `COMMIT`; a slow statement after a fast `ALTER` keeps the table blocked for the whole duration.
- **Find blockers before migrating**: long transactions (`pg_stat_activity.xact_start`) and `idle in transaction` sessions on the primary. Queries on a hot standby do not block DDL on the primary; instead the replayed `ACCESS EXCLUSIVE` lock cancels them on the standby after `max_standby_streaming_delay`.

##### Which changes rewrite the table

A rewrite copies the whole table and rebuilds every index while holding `ACCESS EXCLUSIVE`. Measured with `pg_relation_filenode()` before and after on 18.6:

| Change | Rewrite? |
|---|---|
| `ADD COLUMN c int`, `ADD COLUMN c int NOT NULL DEFAULT 0`, `DEFAULT now()` (11+: non-volatile defaults are stored in the catalog) | no |
| `ADD COLUMN c uuid DEFAULT gen_random_uuid()` (volatile default) | **yes** |
| `ADD COLUMN c int GENERATED ALWAYS AS (a * 2) STORED` | **yes** |
| `ADD COLUMN c int GENERATED ALWAYS AS (a * 2)` (virtual, 18+) | no |
| `ALTER COLUMN c TYPE bigint` from `int` | **yes** |
| `ALTER COLUMN v TYPE varchar(20)` from `varchar(10)`, or `varchar(n)` → `text` | no |
| `ALTER COLUMN v TYPE varchar(10)` from `varchar(20)` or from `text` | **yes** |
| `ALTER COLUMN ts TYPE timestamptz` from `timestamp` | **yes**, unless the session `TimeZone` is `UTC` (then no) |
| `DROP COLUMN` | no (space is reclaimed by later rewrites only) |
| `SET LOGGED` / `SET UNLOGGED` | **yes** |
| `SET TABLESPACE`, `SET ACCESS METHOD` to a different access method (15+) | **yes** (from the documentation; not measured) |

```sql
CREATE TABLE rw (id int PRIMARY KEY, a int, v varchar(10));
INSERT INTO rw SELECT g, g, 'x' FROM generate_series(1, 1000) g;
SELECT pg_relation_filenode('rw') AS before \gset
ALTER TABLE rw ADD COLUMN c1 int NOT NULL DEFAULT 0;            -- catalog only
ALTER TABLE rw ALTER COLUMN v TYPE varchar(20);                  -- catalog only
SELECT pg_relation_filenode('rw') = :before AS same_file;       -- t
ALTER TABLE rw ADD COLUMN c2 uuid DEFAULT gen_random_uuid();     -- volatile default
SELECT pg_relation_filenode('rw') = :before AS same_file;       -- f: rewritten
```

`DEFAULT now()` gives every existing row the time of the migration, which is wrong for `created_at`-style columns: backfill those from real data.

##### Adding a column with a volatile value, then NOT NULL

```sql
-- BAD: one statement, rewrite of a large table under ACCESS EXCLUSIVE
ALTER TABLE events ADD COLUMN public_id uuid NOT NULL DEFAULT gen_random_uuid();

-- GOOD: instant add, default for new rows, batched backfill, constraint without a long lock
ALTER TABLE events ADD COLUMN public_id uuid;                               -- instant
ALTER TABLE events ALTER COLUMN public_id SET DEFAULT gen_random_uuid();    -- instant, new rows only
-- backfill in batches, each in its own transaction (see "Backfills and large deletes")
ALTER TABLE events ADD CONSTRAINT events_public_id_nn CHECK (public_id IS NOT NULL) NOT VALID;
ALTER TABLE events VALIDATE CONSTRAINT events_public_id_nn;   -- SHARE UPDATE EXCLUSIVE: traffic continues
ALTER TABLE events ALTER COLUMN public_id SET NOT NULL;        -- 12+: no scan, the CHECK proves it
ALTER TABLE events DROP CONSTRAINT events_public_id_nn;
```

With `client_min_messages = debug1` the server confirms the shortcut: `existing constraints on column "events.public_id" are sufficient to prove that it does not contain nulls`. On **18+** a not-null constraint can itself be added `NOT VALID` and validated later (`ALTER TABLE events ADD CONSTRAINT events_public_id_nn NOT NULL public_id NOT VALID;` then `VALIDATE CONSTRAINT`); on 17 and older that is a syntax error, so use the `CHECK` pattern above unless the project is on 18.

##### Foreign keys and CHECK constraints: `NOT VALID` + `VALIDATE`

```sql
-- BAD: validates every row while holding SHARE ROW EXCLUSIVE on both tables (writes blocked)
ALTER TABLE orders ADD CONSTRAINT orders_customer_fk
  FOREIGN KEY (customer_id) REFERENCES customers (id);

-- GOOD: enforce for new rows now, check old rows later without blocking writes
ALTER TABLE orders ADD CONSTRAINT orders_customer_fk
  FOREIGN KEY (customer_id) REFERENCES customers (id) NOT VALID;
ALTER TABLE orders VALIDATE CONSTRAINT orders_customer_fk;
```

- A `NOT VALID` constraint is enforced for every inserted or updated row immediately; only the scan of existing rows is deferred. If `VALIDATE` finds a violating row it fails and the constraint stays `NOT VALID` (`pg_constraint.convalidated = false`).
- Index the referencing column (`orders.customer_id`). PostgreSQL indexes the referenced key (it must be unique) but never the referencing side; without that index every `DELETE` or key `UPDATE` on `customers` scans `orders`, and `ON DELETE CASCADE` becomes a sequential scan per deleted parent row.
- **18+** adds `NOT ENFORCED` for check and foreign-key constraints. Such a constraint is metadata only; flag it wherever the application relies on it for integrity.

##### Indexes in migrations

```sql
-- BAD on a live table: blocks INSERT/UPDATE/DELETE for the whole build
CREATE INDEX orders_status_idx ON orders (status);

-- GOOD
CREATE INDEX CONCURRENTLY orders_status_idx ON orders (status);
```

Rules for `CREATE INDEX CONCURRENTLY` (all verified on 18.6):[^8]

- It **cannot run inside a transaction block** (`ERROR: CREATE INDEX CONCURRENTLY cannot run inside a transaction block`). Migration tools that wrap each migration in a transaction need their opt-out: `disable_ddl_transaction!` (Rails), `atomic = False` (Django migrations), `-- +goose NO TRANSACTION`, Flyway `executeInTransaction=false` or a non-transactional script, Liquibase `runInTransaction="false"`, Alembic `with op.get_context().autocommit_block():`.
- A failure (deadlock, unique violation, cancel, `lock_timeout`) leaves an **INVALID index** behind. It is never used for reads but is still maintained on every write. **`CREATE INDEX CONCURRENTLY IF NOT EXISTS` then skips silently** (`NOTICE: relation "..." already exists, skipping`) and leaves the broken index in place. Idempotent migrations must check `pg_index.indisvalid` and `DROP INDEX CONCURRENTLY` before retrying (or `REINDEX INDEX CONCURRENTLY`, 12+).
- It waits for every transaction that could still see the table without the index, including transactions in other tables and databases that hold old snapshots. One forgotten `idle in transaction` session stalls it.
- It cannot be used on a partitioned table (`cannot create index on partitioned table "..." concurrently`). See "Partitioning" for the `ON ONLY` + `ATTACH PARTITION` pattern.
- `DROP INDEX CONCURRENTLY` and `REINDEX ... CONCURRENTLY` (12+) are the non-blocking counterparts; plain `DROP INDEX` takes `ACCESS EXCLUSIVE` on the table.

```sql
-- Find invalid indexes left by failed concurrent builds
SELECT indexrelid::regclass AS index, indrelid::regclass AS table
FROM pg_index WHERE NOT indisvalid;
```

##### Unique constraints and primary keys without a long lock

```sql
CREATE UNIQUE INDEX CONCURRENTLY users_email_key_idx ON users (email);
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE USING INDEX users_email_key_idx;   -- brief lock, no scan
```

The index is renamed to the constraint name. `PRIMARY KEY USING INDEX` works the same way, but if a key column is still nullable the command sets `NOT NULL` itself and scans the table under `ACCESS EXCLUSIVE`; make the column `NOT NULL` first with the validated `CHECK` pattern.

##### Changing column types

- `int` → `bigint` rewrites the table and every index under `ACCESS EXCLUSIVE`, and also rechecks referencing foreign keys. For large tables: add a `bigint` column, keep it in sync with a trigger, backfill in batches, build the unique index concurrently, then swap names and constraints in one short transaction. Better: create keys as `bigint` from the start; `int` keys overflow at 2,147,483,647.
- Widening `varchar(n)` or converting `varchar(n)` → `text` is metadata-only; narrowing or `text` → `varchar(n)` rewrites and checks every row.
- A `USING` clause rewrites the table whenever the stored value changes.
- Changing a function that an index, a generated column or a check constraint uses (`CREATE OR REPLACE FUNCTION`) does not rebuild anything. If its results change, the index is silently wrong until `REINDEX`.

##### Backfills and large deletes

```sql
-- BAD: one giant transaction; long row locks, WAL burst, replication lag, bloat
UPDATE orders SET public_id = gen_random_uuid() WHERE public_id IS NULL;

-- GOOD: bounded batches committed separately (loop in the application or a procedure)
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

PostgreSQL has no `DELETE ... LIMIT` (`syntax error at or near "LIMIT"`); use the same CTE with `DELETE ... USING` or `WHERE id IN (SELECT ... LIMIT n)`. For time-based retention, detach and drop partitions instead of deleting rows. A procedure (11+) can `COMMIT` between batches when called with `CALL` outside an explicit transaction block.

##### Other migration rules

- **Expand/contract for renames and drops.** Running application code that still uses the old name fails the moment the rename commits. Add the new column, deploy code that writes both, backfill, switch reads, then drop.
- **Enums**: `ALTER TYPE ... ADD VALUE` works inside a transaction block (12+; earlier releases refuse it), but the new value cannot be used until that transaction commits (`unsafe use of new value`), except when the enum type itself was created in the same transaction (17+). Values cannot be removed or reordered without recreating the type.
- **`VACUUM FULL` / `CLUSTER`** rewrite under `ACCESS EXCLUSIVE` for the whole run. Online alternatives are the `pg_repack` and `pg_squeeze` extensions; the built-in `REPACK (CONCURRENTLY)` is 19 only.
- **`TRUNCATE`** takes `ACCESS EXCLUSIVE` and is not MVCC-safe: a concurrent transaction with an older snapshot sees the table as empty.
- **`REFRESH MATERIALIZED VIEW`** blocks readers; `REFRESH MATERIALIZED VIEW CONCURRENTLY` needs a unique index on the view without a `WHERE` clause and cannot be used before the view has been populated once.
- **Passwords never belong in migrations** (`CREATE ROLE app PASSWORD '...'` ends up in version control and, with statement logging, in server logs).

***
#### Data Types and Their Pitfalls

##### Timestamps and time zones

```sql
SET timezone = 'Europe/Berlin';
SELECT '2026-06-01 12:00+00'::timestamp;      -- 2026-06-01 12:00:00      (offset silently DROPPED)
SELECT '2026-06-01 12:00+00'::timestamptz;    -- 2026-06-01 14:00:00+02   (correct instant)
SELECT '2026-03-29 02:30'::timestamptz;       -- 2026-03-29 03:30:00+02   (nonexistent local time shifted)

-- DST: '1 day' is calendar-aware, '24 hours' is elapsed time
SELECT timestamptz '2026-10-25 00:00' + interval '1 day';     -- 2026-10-26 00:00:00+01
SELECT timestamptz '2026-10-25 00:00' + interval '24 hours';  -- 2026-10-25 23:00:00+01

SELECT date '2026-01-31' + interval '1 month';                -- 2026-02-28 00:00:00 (clamped)
SELECT timestamptz '2026-01-31 23:59:59.5' BETWEEN '2026-01-01' AND '2026-01-31';   -- f
```

- **Store instants as `timestamptz`.** It stores UTC and converts on input and output using the session `TimeZone`. `timestamp` (without time zone) keeps whatever wall-clock value the client sent and drops any offset; it is right only for values that are not instants (a shop's opening time, a birthday at midnight).[^9][^10]
- **Use half-open ranges**: `created_at >= $1 AND created_at < $2`. `BETWEEN '2026-01-01' AND '2026-01-31'` misses everything after midnight on the 31st.
- `now()` / `current_timestamp` return the **transaction start time**, identical for every statement of the transaction. Use `clock_timestamp()` for wall time inside long transactions or loops.
- **Day boundaries depend on the session time zone.** `created_at::date` gives different days to clients with different `TimeZone` settings; write `(created_at AT TIME ZONE 'Europe/Berlin')::date` explicitly. `timestamptz::date` is only `STABLE` and cannot be used in an index expression (`functions in index expression must be marked IMMUTABLE`); `(created_at AT TIME ZONE 'UTC')::date` can.
- `now()::timestamp AT TIME ZONE 'UTC'` is a common bug: it first converts to session-local wall time and then interprets that as UTC. Use `now() AT TIME ZONE 'UTC'` or keep `timestamptz`.
- Monthly arithmetic clamps to the month end and drifts (`Jan 31 → Feb 28 → Mar 28`); compute each date from the anchor.
- `timestamp(0)` and `timestamptz(0)` **round** fractional seconds (`'2026-01-01 23:59:59.7'::timestamp(0)` is `2026-01-02 00:00:00`).
- Do not use `timetz`: an offset without a date is meaningless across DST changes.
- Bucket times with `date_trunc()` or `date_bin()` (14+).

##### Numbers: `numeric`, floating point, `money`

```sql
SELECT 0.1::float8 + 0.2::float8 = 0.3::float8;   -- f
SELECT 0.1::numeric + 0.2 = 0.3;                  -- t
SELECT 1 / 2, -7 / 2, -7 % 2;                     -- 0 | -3 | -1   (integer division truncates)
SELECT round(2.5::numeric), round(2.5::float8);   -- 3 | 2   (numeric: half away from zero; float8: half to even)
SELECT '12.345'::numeric(5,2);                    -- 12.35  (extra scale silently rounded)
SELECT 2147483647::int + 1;                       -- ERROR: integer out of range
SELECT pg_typeof(sum(1::int)), pg_typeof(sum(1::bigint)), pg_typeof(avg(1::int));   -- bigint | numeric | numeric
```

- Money and anything that must add up exactly: `numeric(p, s)` or integer minor units (`bigint` cents). Never `real`/`double precision`.
- Avoid the `money` type: input and output depend on `lc_monetary`, and the fractional precision is fixed.
- `numeric(p, s)` rounds extra decimal places silently and only raises an error when the integer part overflows; validate input precision in the application or with a `CHECK` if rounding is not acceptable.
- Integer division truncates: cast one operand (`x::numeric / y`) when a fraction is expected.
- `sum(int)` returns `bigint`, `sum(bigint)` and `avg()` of integers return `numeric`. Drivers that map these to 32-bit or 64-bit integers can overflow or fail to decode.
- `EXTRACT()` returns `numeric` (14+; `double precision` before), `date_part()` still returns `double precision`.

##### Text: `text`, `varchar(n)`, `char(n)`

```sql
SELECT 'ab '::char(5) = 'ab'::char(5), length('ab '::char(5)), octet_length('ab '::char(5));  -- t | 2 | 5
SELECT 'abcdef'::varchar(3);                      -- abc  (an explicit cast silently truncates!)
CREATE TABLE vt (v varchar(3));
INSERT INTO vt VALUES ('abcdef');                 -- ERROR: value too long for type character varying(3)
SELECT '' IS NULL;                                -- f   (unlike Oracle, '' is not NULL)
```

- Use `text` (or `varchar` without a length). `varchar(n)` is not faster; it only adds a length check, and changing it later can mean a rewrite. If a limit is a business rule, `CHECK (length(col) <= n)` can be changed with `NOT VALID` + `VALIDATE` and no rewrite.
- **Never use `char(n)`**: it pads with spaces, ignores trailing spaces in comparisons and `length()`, and wastes space.
- `'a' || NULL` is NULL. A name built as `first || ' ' || middle || ' ' || last` disappears when `middle` is NULL; use `concat_ws(' ', first, middle, last)` or `coalesce`.
- Case-insensitive uniqueness: a unique index on `lower(email)` and queries that use `lower(email) = lower($1)`; or a nondeterministic ICU collation; or the `citext` extension. `casefold()` (18+) handles full Unicode case folding (`ß` → `ss` with a suitable collation).
- Sort order and range comparisons on text depend on the collation (`'Z' < 'a'` is true under `"C"` and false under `en_US.utf8`). Tests running under a different database locale than production hide ordering bugs, and an OS upgrade that changes glibc collation data can corrupt text indexes (reindex them, or use the builtin `C.UTF-8` provider, 17+, for new databases).

##### Keys: identity, `serial`, sequences

```sql
-- GOOD: SQL-standard identity (10+), bigint
CREATE TABLE s2 (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, v text);
INSERT INTO s2 (id, v) VALUES (5, 'x');
-- ERROR:  cannot insert a non-DEFAULT value into column "id"
-- HINT:  Use OVERRIDING SYSTEM VALUE to override.

-- AVOID in new schemas: serial = int column + separately owned sequence + DEFAULT
CREATE TABLE s1 (id serial PRIMARY KEY, v text);
INSERT INTO s1 (id, v) VALUES (1, 'explicit');
INSERT INTO s1 (v) VALUES ('from sequence');
-- ERROR:  duplicate key value violates unique constraint "s1_pkey"
```

- `GENERATED ALWAYS AS IDENTITY` rejects accidental explicit ids; `GENERATED BY DEFAULT AS IDENTITY` allows them, which means data imports must reset the sequence: `SELECT setval(pg_get_serial_sequence('s2', 'id'), (SELECT max(id) FROM s2));`.
- With `serial`, a role that has `INSERT` on the table also needs `USAGE` on the sequence (`permission denied for sequence s1_id_seq`); identity columns need no extra grant.
- `serial` is `int`; use `bigint` (or `bigserial`) for anything that can grow. Converting later is a full rewrite.
- **Sequences are not transactional.** A rolled-back insert consumes a value, and concurrent transactions commit out of id order. Never assume gap-free or commit-ordered ids; invoice numbers need a counter row updated under a lock (`UPDATE counters SET n = n + 1 WHERE k = 'invoice' RETURNING n`).

##### UUIDs

```sql
SELECT gen_random_uuid();                                -- version 4, random (core since 13)
SELECT pg_column_size(gen_random_uuid()), pg_column_size(gen_random_uuid()::text);   -- 16 | 40
SELECT uuidv7();                                         -- 18+: time-ordered version 7
SELECT uuid_extract_timestamp(uuidv7());                 -- 17+: creation time of a v1/v7 UUID
```

- Store UUIDs in the `uuid` type (16 bytes), never in `text`/`varchar(36)`: larger, slower to compare, and case or hyphen differences make equal values unequal.
- `gen_random_uuid()` is in core since 13; on 12 and older it needs the `pgcrypto` extension (and `uuid-ossp`'s `uuid_generate_v4()` is an extension function on every version).
- Random (v4) keys scatter inserts over the whole B-tree, which costs cache and WAL on large insert-heavy tables. `uuidv7()` (18+) is time-ordered and inserts at the right edge like a `bigint` identity. On 17 and older it does not exist (`function uuidv7() does not exist`); generate v7 UUIDs in the application if needed.
- A v7 UUID exposes its creation time (`uuid_extract_timestamp()`, 17+). Use v4 where creation time is sensitive, and never use a UUID instead of a `created_at` column.

##### Enums, `CHECK` lists and lookup tables

```sql
CREATE TYPE mood AS ENUM ('sad', 'ok', 'happy');
SELECT 'happy'::mood > 'sad'::mood;            -- t: declaration order, not alphabetical
ALTER TYPE mood ADD VALUE 'meh' BEFORE 'ok';   -- allowed; removing a value is not
```

Enums are compact and ordered, but values cannot be dropped or reordered without recreating the type (renaming works since 10 with `ALTER TYPE ... RENAME VALUE`), and new values cannot be used until the adding transaction commits. For value sets that change, prefer `text` with a `CHECK (status IN (...))` constraint (replace it with `NOT VALID` + `VALIDATE`) or a foreign key to a lookup table.

##### JSON: `jsonb` vs `json`

```sql
SELECT '{"a":1,"a":2}'::jsonb, '{"a":1,"a":2}'::json;   -- {"a": 2} | {"a":1,"a":2}
SELECT '{"n":1.0}'::jsonb = '{"n":1}'::jsonb;           -- t
```

Use `jsonb` for storage and querying (binary, indexable, duplicate keys removed, key order not preserved); use `json` only when the exact input text must be kept. Fields the application filters, joins or constrains on belong in real columns. See "JSONB and SQL/JSON".

##### NULL semantics

```sql
SELECT 1 WHERE 1 NOT IN (SELECT unnest(ARRAY[2, NULL]));                         -- 0 rows!
SELECT 1 WHERE NOT EXISTS (SELECT 1 FROM unnest(ARRAY[2, NULL]) u WHERE u = 1);  -- 1 row
SELECT count(*), count(x) FROM (VALUES (1), (NULL)) v(x);                         -- 2 | 1
SELECT sum(x) FROM (SELECT 1 WHERE false) v(x);                                   -- NULL, not 0
SELECT NULL = NULL, NULL IS DISTINCT FROM NULL;                                   -- NULL | f
SELECT (ARRAY[10, 20, 30])[0], array_length('{}'::int[], 1), cardinality('{}'::int[]);  -- NULL | NULL | 0
```

- **`NOT IN (subquery)` returns no rows as soon as the subquery yields one NULL.** Use `NOT EXISTS`. This is one of the most common real bugs found in SQL review.
- `sum()`, `avg()`, `max()` over zero rows return NULL: wrap them in `coalesce(..., 0)` where the caller expects a number.
- `col = NULL` is never true; write `col IS NULL` or `col IS NOT DISTINCT FROM $1` for nullable parameters.
- `array_length(a, 1)` is NULL for an empty array, so `array_length(a, 1) = 0` never matches; use `cardinality(a) = 0`.
- A unique constraint allows any number of rows with NULL in a key column unless it is declared `UNIQUE NULLS NOT DISTINCT` (15+).

##### Generated columns

```sql
CREATE TABLE gen (price numeric, qty int,
                  total numeric GENERATED ALWAYS AS (price * qty) STORED);   -- 12+
INSERT INTO gen (price, qty) VALUES (2, 3) RETURNING total;   -- 6
INSERT INTO gen VALUES (1, 1, 5);
-- ERROR:  cannot insert a non-DEFAULT value into column "total"
```

- The expression must be immutable (`GENERATED ALWAYS AS (t::date) STORED` on a `timestamptz` fails: `generation expression is not immutable`).
- **On 18+ the keyword decides the kind**: without `STORED` (or with `VIRTUAL`) the column is computed on read. A virtual column cannot be indexed, be part of a unique or primary key, carry a foreign key, use user-defined functions or types, or be published by logical replication. On 12–17, omitting `STORED` is a syntax error. Flag 18 DDL that omits `STORED` for a value that will be filtered or indexed.
- Adding a stored generated column rewrites the table; `ALTER COLUMN ... SET EXPRESSION` (17+) rewrites it too.
- `INSERT INTO t VALUES (...)` without a column list breaks as soon as a column is added; always list target columns in application code.

***

#### Constraints and Keys

- **Every table has a primary key.** Logical replication of `UPDATE`/`DELETE` needs a replica identity, and rows without a key cannot be deduplicated or addressed safely.
- **Index foreign-key columns on the referencing side** (see migrations above).
- **Choose `ON DELETE` deliberately.** `CASCADE` suits owned children (order lines) and is dangerous for shared references (deleting a customer must not delete invoices). `ON DELETE SET NULL (col, ...)` (15+) nulls only the listed columns of a composite foreign key.
- **`CHECK` constraints must be immutable expressions of the row.** They are not rechecked when data elsewhere changes, so a function that reads another table gives a false sense of integrity (and breaks `pg_dump` restores).
- **Deferrable constraints** (`DEFERRABLE INITIALLY DEFERRED`) are checked at commit. A deferrable unique constraint cannot be an `ON CONFLICT` arbiter (`ON CONFLICT does not support deferrable unique constraints/exclusion constraints as arbiters`). Use them only where cyclic updates need them.
- **Exclusion constraints** prevent overlaps that application checks cannot prevent under concurrency:

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;   -- GiST operator classes for scalar "=" parts
CREATE TABLE booking (
  room   int,
  during tstzrange,
  EXCLUDE USING gist (room WITH =, during WITH &&)
);
INSERT INTO booking VALUES (1, '[2026-01-01 10:00, 2026-01-01 11:00)');
INSERT INTO booking VALUES (1, '[2026-01-01 10:30, 2026-01-01 12:00)');
-- ERROR:  conflicting key value violates exclusion constraint "booking_room_during_excl"
```

  18+ can express the same as a temporal key, `PRIMARY KEY (room, during WITHOUT OVERLAPS)`, and temporal foreign keys with `PERIOD`; on 17 and older both are syntax errors.

- **`UNIQUE NULLS NOT DISTINCT`** (15+) when NULL should count as a value:

```sql
CREATE TABLE nd (a int, b int, UNIQUE NULLS NOT DISTINCT (a, b));
INSERT INTO nd VALUES (1, NULL);
INSERT INTO nd VALUES (1, NULL);   -- ERROR: duplicate key value violates unique constraint "nd_a_b_key"
```

  On 14 and older, the usual substitute is a partial unique index per nullable column (`CREATE UNIQUE INDEX ... (a) WHERE b IS NULL`).

***

#### Indexing

##### Index types

| Type | Use for | Notes |
|---|---|---|
| B-tree (default) | `=`, `<`, `>`, `BETWEEN`, `IN` / `= ANY`, `ORDER BY`, `IS NULL`, prefix `LIKE 'x%'` (needs a `C`/`C.UTF-8` collation or a `text_pattern_ops` index) | Efficient only when the leading column is constrained; 18+ can "skip scan" a low-cardinality leading column |
| Hash | `=` only | Crash-safe since 10; cannot be unique or multicolumn; rarely beats B-tree |
| GIN | `jsonb` (`@>`, `?`, `?|`, `?&`, `@?`, `@@`), arrays (`@>`, `&&`), full-text `tsvector`, `pg_trgm` (`LIKE '%x%'`, `ILIKE`, `~`) | Larger, slower writes (`fastupdate` pending list) |
| GiST | ranges, geometry, exclusion constraints, `pg_trgm`, nearest-neighbour `ORDER BY col <-> value` | Lossy for some types; rechecks rows |
| SP-GiST | non-balanced data: IP prefixes (`inet`), quadtrees, text prefixes | |
| BRIN (9.5+) | very large, append-mostly tables whose column correlates with physical order (timestamps) | Tiny; useless when `pg_stats.correlation` is far from ±1 |

```sql
CREATE INDEX orders_user_status ON orders (user_id, status);              -- composite: order matters
CREATE INDEX users_active_email ON users (email) WHERE status = 'active';   -- partial
CREATE UNIQUE INDEX users_email_lower ON users (lower(email));             -- expression
CREATE INDEX users_email_cov ON users (email) INCLUDE (name, created_at);   -- covering (11+)
CREATE INDEX posts_fts ON posts USING gin (to_tsvector('english', title || ' ' || body));
CREATE INDEX events_doc ON events USING gin (doc jsonb_path_ops);
CREATE INDEX logs_at_brin ON logs USING brin (at);
```

##### Rules for reviewing indexes

Plans below are from 18.6 on a 100,000-row table with the default `en_US.utf8` collation of the Docker image.

| Query | Index | Plan |
|---|---|---|
| `WHERE email LIKE 'user42%'` | plain `(email)` | **Seq Scan**: a B-tree under a non-C collation cannot serve `LIKE` |
| same | `(email text_pattern_ops)` | Bitmap Index Scan, `Index Cond: ((email ~>=~ 'user42') AND (email ~<~ 'user43'))` |
| `WHERE lower(email) = '...'` | plain `(email)` | Seq Scan; needs an index on `lower(email)` |
| `WHERE id = 42::numeric` (`id` is `int`) | primary key | **Seq Scan**, `Filter: ((id)::numeric = '42'::numeric)`: the cast is on the column |
| `WHERE created_at::date = '2026-02-03'` | `(created_at)` | Seq Scan; a half-open range uses the index |
| `WHERE email LIKE '%r4242%'` | `gin (email gin_trgm_ops)` (`pg_trgm`) | Bitmap Index Scan |
| `WHERE (created_at, id) < ($1, $2) ORDER BY created_at DESC, id DESC LIMIT 20` | `(created_at DESC, id DESC)` | Index Only Scan (only these columns selected), `Index Cond: (ROW(created_at, id) < ROW(...))`, no Sort |

- **The predicate must match the indexed expression and type.** Functions or casts on the column (`date(created_at)`, `id::text = $1`, `lower(email)` without an expression index) disable the index. Drivers or ORMs that bind parameters as `numeric` or `text` for an `int`/`uuid` column cause the same problem.
- **Composite order**: equality columns first, then the range or sort column. `(status, created_at)` serves `WHERE status = $1 ORDER BY created_at DESC LIMIT 20`. Before 18 a query that filters only on `created_at` cannot use this index efficiently; 18's skip scan helps only when `status` has few distinct values. Do not accept "the composite index covers it" without a plan.
- **Expression indexes need IMMUTABLE functions** and queries that repeat the exact expression. After creating one, run `ANALYZE` so the planner has statistics for the expression.
- **Partial indexes**: the query must imply the predicate at plan time. A prepared statement with `status = $1` cannot use an index `WHERE status = 'active'` under a generic plan; inline the constant. `ON CONFLICT` on a partial unique index must repeat the predicate (`ON CONFLICT (user_id) WHERE active DO NOTHING`).
- **Prefix `LIKE`** needs `text_pattern_ops` (or a `C` collation). Leading-wildcard `LIKE '%x'` needs a `pg_trgm` GIN/GiST index. User input in `LIKE` must have `%`, `_` and `\` escaped, or use `starts_with(col, $1)`.
- **Covering indexes** (`INCLUDE`, 11+) enable index-only scans; the included columns cannot be searched or sorted on. Index-only scans need an up-to-date visibility map (vacuum), visible as low `Heap Fetches:` in `EXPLAIN ANALYZE`.
- **Every index costs writes**, WAL and vacuum work, and an update that changes any indexed column cannot be a HOT update. Remove duplicates (`(a)` next to `(a, b)`) and unused indexes (`pg_stat_user_indexes.idx_scan = 0` over a full business cycle, checked on every replica: statistics are per server).
- **GIN on `jsonb`**: `jsonb_path_ops` is smaller and faster but supports only `@>`, `@?` and `@@`; the default `jsonb_ops` also supports `?`, `?|`, `?&`. Neither serves `doc->>'type' = 'x'`, which needs a B-tree expression index on `(doc->>'type')`.

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE TABLE ix (id int PRIMARY KEY, email text, created_at timestamptz);
INSERT INTO ix SELECT g, 'user' || g || '@example.com', '2026-01-01'::timestamptz + g * interval '1 minute'
FROM generate_series(1, 100000) g;
CREATE INDEX ix_email ON ix (email);
CREATE INDEX ix_created ON ix (created_at);
ANALYZE ix;
EXPLAIN (COSTS OFF) SELECT * FROM ix WHERE email LIKE 'user42%';             -- Seq Scan
CREATE INDEX ix_email_pat ON ix (email text_pattern_ops);
EXPLAIN (COSTS OFF) SELECT * FROM ix WHERE email LIKE 'user42%';             -- Bitmap Index Scan on ix_email_pat
EXPLAIN (COSTS OFF) SELECT * FROM ix WHERE id = 42::numeric;                 -- Seq Scan, (id)::numeric
EXPLAIN (COSTS OFF) SELECT * FROM ix WHERE created_at::date = '2026-02-03';  -- Seq Scan
EXPLAIN (COSTS OFF) SELECT * FROM ix
WHERE created_at >= '2026-02-03' AND created_at < '2026-02-04';               -- Index Scan / Bitmap on ix_created
CREATE INDEX ix_email_trgm ON ix USING gin (email gin_trgm_ops);
EXPLAIN (COSTS OFF) SELECT * FROM ix WHERE email LIKE '%r4242%';             -- Bitmap Index Scan on ix_email_trgm
```

***

#### Query Writing

##### Sargable predicates and time ranges

```sql
-- BAD: function or cast on the indexed column
WHERE date(created_at) = '2026-02-03'
WHERE created_at::date = '2026-02-03'
WHERE id::text = $1

-- GOOD: compare the raw column with a range or a correctly typed parameter
WHERE created_at >= '2026-02-03' AND created_at < '2026-02-04'
```

On partitioned tables the same rule decides partition pruning: `created_at::date = ...` scans every partition.

##### Lists and N+1 queries

```python
# BAD: one query per user
for user in users:
    cur.execute("SELECT * FROM orders WHERE user_id = %s", (user.id,))

# GOOD: one round trip; the list is a single array parameter
cur.execute("SELECT * FROM orders WHERE user_id = ANY(%s)", (user_ids,))
```

`= ANY($1::bigint[])` is preferable to generating `IN ($1, $2, ..., $n)`: one statement text and one prepared plan for every list length, and an empty array is valid (`= ANY('{}')` matches nothing, while an empty `IN ()` is a syntax error). Never build `IN (...)` lists by string concatenation.

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

The row-value comparison `(a, b) < (x, y)` becomes one index range (`Index Cond: (ROW(created_at, id) < ROW(...))`); the expanded `a < x OR (a = x AND b < y)` is planned as a filter. The sort key must be unique (add the primary key) or rows are skipped or repeated across pages, and every column must sort in the same direction for the index to serve it. `LIMIT` without `ORDER BY` returns an arbitrary subset that can change between executions.

##### Counting

`SELECT count(*) FROM big_table` reads the whole table (or the smallest index with an index-only scan). For UI counters use an estimate (`SELECT reltuples::bigint FROM pg_class WHERE oid = 'big_table'::regclass`) or a maintained counter. `count(col)` skips NULLs and is not the same as `count(*)`. `EXISTS (SELECT 1 ...)` is the right test for "any rows?", not `count(*) > 0`.

##### CTEs, subqueries, joins

- Since 12, a non-recursive, side-effect-free CTE that is referenced once is **inlined** into the outer query; `WITH x AS MATERIALIZED (...)` forces the old optimization fence, `NOT MATERIALIZED` forces inlining. On 11 and older every CTE is materialized, so a filter on the CTE's result is not pushed down. Do not ask for "rewrite the CTE as a subquery for performance" on 12+ unless the plan shows it.
- Data-modifying CTEs (`WITH d AS (DELETE ... RETURNING ...)`) always run exactly once and to completion, even if the outer query does not read them; all sub-statements see the same snapshot and cannot see each other's changes.
- Top-N per group: `DISTINCT ON (group_col) ... ORDER BY group_col, sort_col DESC, id` for one row per group (the `ORDER BY` must start with the `DISTINCT ON` keys), or `CROSS JOIN LATERAL (... ORDER BY ... LIMIT n)` with an index on `(group_col, sort_col)`. `CROSS JOIN LATERAL` drops groups without rows; `LEFT JOIN LATERAL (...) ON true` keeps them.
- `UNION` sorts or hashes to remove duplicates; use `UNION ALL` when duplicates are impossible or wanted.
- `DISTINCT` added to hide duplicates from a wrong join is a bug, not an optimization.
- Correlated subqueries in the select list run once per outer row; prefer a join with `GROUP BY` or a `LATERAL` join when they are expensive.
- `ORDER BY random() LIMIT n` sorts the whole table; use `TABLESAMPLE SYSTEM (p)` or a keyed random pick for large tables.
- Comma joins and `JOIN ... ON` produce the same plans; explicit `JOIN` is a readability rule (a forgotten predicate is visible).

##### Window functions and aggregates

```sql
CREATE TABLE sales (region text, rep text, at int, amount int);
INSERT INTO sales VALUES ('n','a',1,10), ('n','b',2,30), ('n','c',3,30), ('s','d',1,5), ('s','e',2,NULL);
SELECT rep, amount,
       rank()       OVER w AS rnk,      -- n: 1,1,3   (gaps after ties)
       dense_rank() OVER w AS drnk,     -- n: 1,1,2
       row_number() OVER w AS rn        -- n: 1,2,3   (ties broken arbitrarily!)
FROM sales WHERE region = 'n'
WINDOW w AS (ORDER BY amount DESC);
SELECT at, last_value(amount) OVER (ORDER BY at) FROM sales WHERE region = 'n';
-- 1 | 10,  2 | 30,  3 | 30   -- default frame ends at the current row (and its peers)
SELECT region, count(*) FILTER (WHERE amount > 20) AS big, sum(amount), avg(amount)
FROM sales GROUP BY region ORDER BY region;
-- n | 2 | 70 | 23.33...   s | 0 | 5 | 5.00...   (NULL amounts are ignored)
```

- `row_number()` over a non-unique `ORDER BY` is nondeterministic. "Latest row per group" queries need a unique tie-breaker (`ORDER BY at DESC, id DESC`).
- **The default frame is `RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW`.** `last_value()` therefore returns the current row's value; write `ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING` or use `first_value(...) OVER (ORDER BY ... DESC)`. Running sums with `RANGE` include peers; use `ROWS` for strictly row-by-row totals.
- `ORDER BY x DESC` puts NULLs **first**; add `NULLS LAST` when ranking nullable values.
- `string_agg(x, ',' ORDER BY x)` and `array_agg(x ORDER BY x)`: without an `ORDER BY` inside the aggregate the order is unspecified.
- Prefer `count(*) FILTER (WHERE ...)` (9.4+) over `sum(CASE ... END)` for conditional counts.

##### Recursive CTEs

Every recursive CTE over user-controlled graphs (categories, org charts, permissions) needs cycle protection: the `CYCLE` clause (14+), a depth counter, or `UNION` instead of `UNION ALL` when the recursive rows are fully identical on a revisit. One bad edge otherwise produces an endless query.

```sql
CREATE TABLE edge (src int, dst int);
INSERT INTO edge VALUES (1, 2), (2, 3), (3, 1);
WITH RECURSIVE walk(n) AS (
  SELECT 1
  UNION ALL
  SELECT e.dst FROM walk w JOIN edge e ON e.src = w.n
) CYCLE n SET is_cycle USING path
SELECT n, is_cycle FROM walk;
-- 1 f / 2 f / 3 f / 1 t
```

##### Full-text search

```sql
SELECT to_tsvector('english', 'The quick brown foxes jumped')
       @@ websearch_to_tsquery('english', 'fox -cat');      -- t
SELECT to_tsquery('english', 'fox & (');                    -- ERROR: no operand in tsquery: "fox & ("
SELECT websearch_to_tsquery('english', 'fox & (');          -- 'fox'  (never raises on user input)
```

Pass user input to `websearch_to_tsquery()` (11+) or `plainto_tsquery()`, never to `to_tsquery()`. Always pass the text search configuration explicitly: the one-argument forms depend on `default_text_search_config`, are not immutable, and cannot be used in index expressions; the query must use the same configuration as the index.

##### Prepared statements and generic plans

With `plan_cache_mode = auto` (the default; the setting exists since 12), a prepared statement is planned with the actual parameter values for its first five executions; after that the server may switch to a generic plan when its estimated cost is not worse on average. For heavily skewed columns this can lock in a bad plan for the rare values. Fix it with `SET plan_cache_mode = force_custom_plan` for the affected role or function (`ALTER FUNCTION ... SET plan_cache_mode = ...`), or restructure the query; do not disable prepared statements in the driver globally.

##### Planner settings in application code

`SET enable_nestloop = off` (or any `enable_*`) affects every later query in the session, and behind a pooler other clients' queries too. Fix statistics instead: `ANALYZE`, `ALTER TABLE ... ALTER COLUMN ... SET STATISTICS 1000` for skewed columns, and `CREATE STATISTICS ... (dependencies, ndistinct, mcv)` (10+; `mcv` 12+) for correlated columns. PostgreSQL has no query hints in core.

##### Batch writes

```sql
-- GOOD: multi-row VALUES, or COPY ... FROM STDIN through the driver for bulk loads
INSERT INTO users (name, email) VALUES ('Alice', 'a@example.com'), ('Bob', 'b@example.com');

-- Batch UPDATE from a VALUES list (or a temporary table / unnest of arrays)
UPDATE users u SET status = v.status
FROM (VALUES (1, 'active'), (2, 'inactive')) AS v(id, status)
WHERE u.id = v.id;
```

Run `ANALYZE` after bulk loads into new or truncated tables; autovacuum's analyze may come too late for the next query.

***

#### Reading EXPLAIN

```sql
EXPLAIN SELECT ...;                                -- estimated plan only; does not execute
EXPLAIN (ANALYZE, BUFFERS) SELECT ...;             -- executes: actual rows, time, buffer counts
EXPLAIN (ANALYZE, BUFFERS, VERBOSE, SETTINGS) ...; -- SETTINGS: 12+
EXPLAIN (GENERIC_PLAN) SELECT * FROM t WHERE id = $1;   -- 16+: plan with placeholders, no values
EXPLAIN (ANALYZE, SERIALIZE, MEMORY) ...;          -- 17+: output conversion cost, planner memory
```

- **`EXPLAIN ANALYZE` executes the statement.** For `INSERT`/`UPDATE`/`DELETE`/`MERGE` wrap it in `BEGIN; EXPLAIN ANALYZE ...; ROLLBACK;`.
- Ask for **`BUFFERS`** explicitly: before 18 it is only shown when requested; on 18+ `EXPLAIN ANALYZE` includes it by default (`BUFFERS OFF` suppresses it).
- 18 prints actual row counts with two decimals (`rows=1.00`) and adds `Index Searches: N` to index scans; tools or regression tests that parse `rows=(\d+)` need updating.

What to look for:[^11]

- **Estimated vs actual rows** off by an order of magnitude or more: stale or insufficient statistics, correlated columns (extended statistics), or a non-sargable predicate.
- **`Rows Removed by Filter`** large on a scan: missing or unusable index.
- **`Buffers: shared read=`** high compared with `hit=`: the query touches too much data or the cache is cold. Buffers are the most stable measure for comparing two plans; timing varies with the cache.
- **`Sort Method: external merge  Disk: ...`**: `work_mem` too small for this sort; raise it per query or role, not globally (every sort and hash node of every connection may use it).
- **`Heap Fetches:`** high on an Index Only Scan: the visibility map is stale; vacuum the table.
- **`Nested Loop`** with many outer rows over an inner `Seq Scan`: missing join index or a misestimate.
- **`Subplans Removed: N`** under `Append`: run-time partition pruning worked.
- A `Seq Scan` is not a defect by itself: on small tables or when most rows qualify it is the right plan.

```sql
CREATE TABLE ex (id int PRIMARY KEY, v int);
INSERT INTO ex SELECT g, g % 100 FROM generate_series(1, 100000) g;
ANALYZE ex;
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF, SUMMARY OFF)
SELECT * FROM ex WHERE id = ANY ('{1,500,90000}');
--  Index Scan using ex_pkey on ex (actual rows=3.00 loops=1)      -- 18: rows with two decimals
--    Index Cond: (id = ANY ('{1,500,90000}'::integer[]))
--    Index Searches: 3                                            -- 18+
--    Buffers: shared hit=...
BEGIN;
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, SUMMARY OFF) DELETE FROM ex WHERE id = 1;
ROLLBACK;                                                          -- the DELETE really ran
SELECT count(*) FROM ex WHERE id = 1;                              -- 1
```

***
#### Transactions, Isolation and Concurrency

##### Isolation levels

| Level | Snapshot | What the code must handle |
|---|---|---|
| READ COMMITTED (default) | new snapshot per statement | lost updates in read-modify-write, check-then-insert races, non-repeatable reads, re-evaluated `WHERE` clauses on concurrently updated rows |
| REPEATABLE READ | one snapshot per transaction | `ERROR: could not serialize access due to concurrent update` (SQLSTATE `40001`) when updating a row changed after the snapshot; write skew is still possible |
| SERIALIZABLE | per transaction plus SSI dependency tracking | `40001` "read/write dependencies among transactions"; retry the **whole** transaction |

`READ UNCOMMITTED` is accepted but behaves like READ COMMITTED: PostgreSQL never shows dirty reads.[^12]

##### READ COMMITTED anomalies (verified with two sessions on 18.6)

```sql
-- 1. Lost update: read in the application, compute, write back
--    A: SELECT n FROM ctr WHERE id = 1;           -- 0
--    B: SELECT n FROM ctr WHERE id = 1;           -- 0
--    A: UPDATE ctr SET n = 1 WHERE id = 1; COMMIT;
--    B: UPDATE ctr SET n = 1 WHERE id = 1; COMMIT; -- final value 1, one increment lost

-- GOOD: atomic in the database
UPDATE ctr SET n = n + 1 WHERE id = 1 RETURNING n;
-- GOOD: lock the row when the new value is computed in application code
BEGIN;
SELECT n FROM ctr WHERE id = 1 FOR UPDATE;
UPDATE ctr SET n = 2 WHERE id = 1;
COMMIT;
-- GOOD: optimistic concurrency with a version column; 0 rows updated = conflict
UPDATE doc SET body = 'new', version = version + 1 WHERE id = 1 AND version = 3;
```

```sql
-- 2. Check-then-insert race without a unique constraint
--    A: BEGIN; INSERT INTO users_nc (email) SELECT 'a@x' WHERE NOT EXISTS (SELECT 1 FROM users_nc WHERE email = 'a@x');
--    B: (same statement)  -- does not block: A's row is invisible to B
--    A: COMMIT;  B: COMMIT;
--    SELECT count(*) FROM users_nc WHERE email = 'a@x';   -- 2
```

Only a unique constraint (with `ON CONFLICT`) or `SERIALIZABLE` prevents duplicates; "check first" code in the application has the same race.

```sql
-- 3. A concurrent UPDATE makes a WHERE clause miss rows (re-check of the updated version)
CREATE TABLE website (hits int);
INSERT INTO website VALUES (9), (10);
--    A: BEGIN; UPDATE website SET hits = hits + 1;     -- 9→10, 10→11, not yet committed
--    B: DELETE FROM website WHERE hits = 10;           -- blocks on the row A changed
--    A: COMMIT;
--    B: DELETE 0   -- the old 10 row is now 11; the new 10 row was not in B's snapshot
```

The documentation's own example: under READ COMMITTED a statement that waits for a concurrent update re-evaluates its `WHERE` clause on the new row version only, so it can see an inconsistent mix. Complex multi-row conditions need `REPEATABLE READ`/`SERIALIZABLE` (with retries) or explicit locks.[^12]

##### REPEATABLE READ and SERIALIZABLE need retry loops

Verified on 18.6: a `REPEATABLE READ` transaction updating a row that another transaction committed after its snapshot fails with `could not serialize access due to concurrent update`; under `SERIALIZABLE`, two "on call" doctors each checking that someone else is on call and going off call (write skew) cannot both commit: one gets `could not serialize access due to read/write dependencies among transactions`.

- Code that uses `REPEATABLE READ` or `SERIALIZABLE` **must retry the whole transaction** on SQLSTATE `40001`, and every code path should also retry deadlocks (`40P01`). Without a retry loop the higher level only turns silent corruption into user-visible errors.
- Retrying means re-running the business logic, including the reads; replaying only the failed statement is wrong.
- `SERIALIZABLE` is the simplest correct option for complex invariants. Keep such transactions short, declare read-only ones `READ ONLY`, and use `SERIALIZABLE READ ONLY DEFERRABLE` for long reports (no serialization failures, may wait at the start).
- Retry `40001`/`40P01` and connection loss *before* commit; never blindly retry after an unknown commit outcome for non-idempotent writes (use idempotency keys).

##### Row locks, queues and deadlocks

```sql
-- Job queue: concurrent workers take different rows and never wait for each other (9.5+)
BEGIN;
SELECT id FROM jobs WHERE NOT done ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED;
-- ... process the job, then
UPDATE jobs SET done = true WHERE id = 1;
COMMIT;
```

- `SKIP LOCKED` is for queue-like consumers only; it returns an inconsistent view of the table and is wrong for general reads.
- `NOWAIT` fails immediately (`could not obtain lock on row in relation "jobs"`); `lock_timeout` also limits waits for row locks.
- Use `FOR NO KEY UPDATE` when key columns do not change: foreign-key checks from inserts into child tables take `FOR KEY SHARE` on the parent row, which conflicts with `FOR UPDATE` but not with `FOR NO KEY UPDATE` (a plain `UPDATE` of non-key columns takes `FOR NO KEY UPDATE` itself).
- Deadlocks come from locking rows in different orders. Lock in a consistent order (`SELECT ... ORDER BY id FOR UPDATE`), keep transactions short, and retry on `40P01`.
- Never keep a transaction open across network calls, user think time or message-queue waits.

##### Advisory locks

```sql
-- GOOD: transaction-scoped; released automatically at COMMIT/ROLLBACK
BEGIN;
SELECT pg_try_advisory_xact_lock(hashtextextended('nightly-report', 0));   -- t / f
COMMIT;

-- RISKY: session-scoped; survives errors and COMMIT until pg_advisory_unlock or disconnect
SELECT pg_advisory_lock(42);
SELECT pg_advisory_unlock(42);
```

Prefer `pg_advisory_xact_lock()` / `pg_try_advisory_xact_lock()`. Session-level advisory locks leak across a transaction-pooled connection (PgBouncer transaction mode does not support them), and every code path must unlock them. Keys are one `bigint` or two `int`s; derive them deterministically (`hashtextextended(text, 0)`, 11+) and document the key space so unrelated features do not collide.

##### Upserts: `INSERT ... ON CONFLICT` vs `MERGE`

```sql
CREATE TABLE kv (k text PRIMARY KEY, v int, updated_at timestamptz);
INSERT INTO kv (k, v, updated_at) VALUES ('a', 1, now())
ON CONFLICT (k) DO UPDATE
   SET v = excluded.v, updated_at = excluded.updated_at
 WHERE kv.updated_at < excluded.updated_at          -- ignore stale writes
RETURNING k, v;
```

- `ON CONFLICT` (9.5+) is atomic against concurrent inserts of the same key. **`MERGE` (15+) is not**: verified, two concurrent `MERGE ... WHEN NOT MATCHED THEN INSERT` for the same key make one fail with `duplicate key value violates unique constraint`, while the same race with `ON CONFLICT DO UPDATE` succeeds. Use `ON CONFLICT` for single-row "insert or update" under concurrency; use `MERGE` for set-based synchronization where conflicts are impossible or handled.[^13][^14]
- The conflict target must match a unique index or constraint exactly (columns, expression, and the predicate of a partial index); otherwise `there is no unique or exclusion constraint matching the ON CONFLICT specification`.
- A multi-row upsert that hits the same key twice fails with `ON CONFLICT DO UPDATE command cannot affect row a second time`; deduplicate the input (`DISTINCT ON`) first.
- `ON CONFLICT DO NOTHING RETURNING` returns **nothing** for skipped rows. To get the existing id, select it afterwards, or use `DO UPDATE SET k = excluded.k` (which locks the row and writes a new row version: bloat and triggers). `ON CONFLICT ... DO SELECT` is 19 only.
- Telling inserts from updates: on 18+ use `RETURNING (old.k IS NULL) AS inserted`; the older `RETURNING (xmax = 0) AS inserted` relies on an implementation detail.
- `MERGE ... RETURNING` with `merge_action()` and `WHEN NOT MATCHED BY SOURCE` are **17+**. `WHEN NOT MATCHED BY SOURCE THEN DELETE` without an extra condition deletes **every** target row that is absent from the source; look for a guard (`AND t.tenant_id = ...`).
- If one target row matches several source rows, `MERGE` raises `MERGE command cannot affect row a second time`; deduplicate the source.

##### Long transactions and timeouts

Long-running and `idle in transaction` sessions hold locks, block `CREATE INDEX CONCURRENTLY`, and stop vacuum from removing dead rows in **every** table of the cluster (bloat, eventually transaction ID wraparound risk). Set server-side limits per role rather than relying on client timeouts, which leave the query running on the server:

```sql
CREATE ROLE app_rw NOLOGIN;
ALTER ROLE app_rw SET statement_timeout = '15s';
ALTER ROLE app_rw SET lock_timeout = '5s';
ALTER ROLE app_rw SET idle_in_transaction_session_timeout = '60s';   -- 9.6+, terminates the session
ALTER ROLE app_rw SET idle_session_timeout = '30min';                -- 14+, terminates idle sessions
ALTER ROLE app_rw SET transaction_timeout = '5min';                  -- 17+, terminates the session
```

`transaction_timeout` and `idle_in_transaction_session_timeout` **terminate the connection** (not just the statement); pools must validate connections and clients must handle the disconnect. Keep `statement_timeout` below `transaction_timeout`, or the longer one never fires. Do not set these globally where batch jobs and migrations run as the same role.

***

#### JSONB and SQL/JSON

##### Operators and their traps

```sql
SELECT doc -> 'a'        AS j,        -- jsonb
       doc ->> 'a'       AS t,        -- text
       doc #>> '{a,b}'   AS p,        -- text at a path
       doc['a']['b']     AS s         -- jsonb subscripting (14+)
FROM (SELECT '{"a":{"b":"x"}}'::jsonb AS doc) d;

SELECT ('{"n":"5"}'::jsonb ->> 'n')::int;          -- 5
SELECT ('{"n":"5"}'::jsonb -> 'n')::int;           -- ERROR: cannot cast jsonb string to type integer
SELECT '{"a":null}'::jsonb ->> 'a' IS NULL;        -- t: JSON null becomes SQL NULL through ->>

SELECT jsonb_set('{"a":1}', '{b}', NULL);          -- NULL: the whole document is lost!
SELECT jsonb_set_lax('{"a":1}', '{b}', NULL);      -- {"a": 1, "b": null}   (13+)
```

- **`jsonb_set(doc, path, NULL)` returns NULL** because the function is strict: `UPDATE t SET doc = jsonb_set(doc, '{x}', to_jsonb($1))` wipes the column when `$1` is NULL. Use `jsonb_set_lax()` (13+), `coalesce`, or `doc || jsonb_build_object('x', $1)`.
- `->` returns `jsonb` (compare with `jsonb` values), `->>` returns `text` (cast the text). Casting a JSON `null` with `(doc -> 'k')::int` raises an error before 18 and returns SQL NULL on 18+.
- Updating one key rewrites the whole (possibly TOASTed) document and its GIN entries; large documents updated often are a schema smell.
- `jsonb` numbers are `numeric`; JavaScript clients lose precision above 2^53.

##### Indexing jsonb

```sql
CREATE TABLE ev (id int PRIMARY KEY, doc jsonb);
INSERT INTO ev SELECT g, jsonb_build_object('type', 't' || (g % 50), 'user', jsonb_build_object('id', g))
FROM generate_series(1, 50000) g;
CREATE INDEX ev_gin ON ev USING gin (doc jsonb_path_ops);
ANALYZE ev;
EXPLAIN (COSTS OFF) SELECT * FROM ev WHERE doc @> '{"type":"t7"}';           -- Bitmap Index Scan on ev_gin
EXPLAIN (COSTS OFF) SELECT * FROM ev WHERE doc @? '$.user ? (@.id == 5)';    -- Bitmap Index Scan on ev_gin (12+)
EXPLAIN (COSTS OFF) SELECT * FROM ev WHERE doc ? 'type';                     -- Seq Scan (jsonb_path_ops has no ?)
EXPLAIN (COSTS OFF) SELECT * FROM ev WHERE doc ->> 'type' = 't7';            -- Seq Scan (GIN does not serve ->>)
CREATE INDEX ev_type ON ev ((doc ->> 'type'));
EXPLAIN (COSTS OFF) SELECT * FROM ev WHERE doc ->> 'type' = 't7';            -- Bitmap Index Scan on ev_type
```

Write containment (`@>`) or jsonpath (`@?`, `@@`) predicates for GIN, or add B-tree expression indexes for the keys you filter by; better, promote hot keys to real columns. The SQL/JSON query functions (`JSON_EXISTS()` etc., 17+) are not GIN-indexable; use the operators in hot `WHERE` clauses.

##### SQL/JSON (version-labelled)

```sql
SELECT jsonb_path_query('{"a":[1,2,3]}', '$.a[*] ? (@ > 1)');   -- 2, 3          (12+)
SELECT '{"x":1}' IS JSON OBJECT;                                 -- t             (16+)
SELECT JSON_OBJECT('a': 1, 'b': NULL ABSENT ON NULL);            -- {"a" : 1}     (16+)
SELECT JSON_VALUE(jsonb '{"a":"5"}', '$.a' RETURNING int);       -- 5             (17+)
SELECT * FROM JSON_TABLE(jsonb '[{"id":1,"n":"a"},{"id":2,"n":"b"}]', '$[*]'
       COLUMNS (id int PATH '$.id', n text PATH '$.n')) jt;     -- 2 rows        (17+)
```

- **`JSON_VALUE`, `JSON_QUERY` and `JSON_TABLE` columns default to `NULL ON ERROR`**: `JSON_VALUE(jsonb '{"a":"x"}', '$.a' RETURNING int)` silently returns NULL. Add `ERROR ON ERROR` (per column in `JSON_TABLE`) wherever bad input must not disappear (imports, validation, money).
- Bind jsonpath variables with `PASSING` (SQL/JSON functions) or the `vars` argument of `jsonb_path_query()`; never concatenate user input into jsonpath text.
- On 16 and older use `jsonb_array_elements()`/`jsonb_to_recordset()` and `->>` with casts instead of `JSON_TABLE`/`JSON_VALUE`.

***

#### Partitioning

Declarative partitioning (10+; hash partitioning, default partitions and partitioned indexes 11+) pays off for very large tables where pruning or dropping whole partitions (retention) matters. It adds planning overhead and constraints; do not partition small tables, and avoid thousands of partitions.[^15]

```sql
CREATE TABLE m (
    id         bigint GENERATED ALWAYS AS IDENTITY,   -- identity on a partitioned table: 17+
    created_at timestamptz NOT NULL,
    v          int,
    PRIMARY KEY (id, created_at)                      -- must include the partition key
) PARTITION BY RANGE (created_at);
CREATE TABLE m_2026_01 PARTITION OF m FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE m_2026_02 PARTITION OF m FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

-- Index without blocking writes: parent shell index, then each partition concurrently
CREATE INDEX m_v_idx ON ONLY m (v);                          -- invalid, parent only, instant
CREATE INDEX CONCURRENTLY m_2026_01_v ON m_2026_01 (v);
ALTER INDEX m_v_idx ATTACH PARTITION m_2026_01_v;
CREATE INDEX CONCURRENTLY m_2026_02_v ON m_2026_02 (v);
ALTER INDEX m_v_idx ATTACH PARTITION m_2026_02_v;            -- parent index becomes valid
SELECT indisvalid FROM pg_index WHERE indexrelid = 'm_v_idx'::regclass;   -- t
```

Rules verified on 18.6:

- **Unique constraints and primary keys must include every partition key column** (`unique constraint on partitioned table must include all partitioning columns`). Uniqueness of `id` alone is not enforced across partitions.
- **Pruning needs the raw key**: `WHERE created_at >= $1 AND created_at < $2` prunes (at plan time, or at run time with a generic plan: `Subplans Removed: N`); `WHERE created_at::date = ...` or `date_trunc('month', created_at) = ...` scan every partition.
- **Default partitions are a trap.** Creating or attaching a partition for a new range scans the default partition under `ACCESS EXCLUSIVE`, fails if matching rows already landed there (`updated partition constraint for default partition "..." would be violated by some row`), and `DETACH PARTITION ... CONCURRENTLY` (14+) is refused while a default partition exists. Pre-create partitions (scheduled job or `pg_partman`) and alert on rows in the default partition.
- **Attach without a long scan**: add and validate a `CHECK` constraint matching the partition bounds on the staging table first; `ATTACH PARTITION` (12+: `SHARE UPDATE EXCLUSIVE` on the parent) then skips the scan.
- **Retention**: `ALTER TABLE m DETACH PARTITION m_2026_01 CONCURRENTLY; DROP TABLE m_2026_01;` instead of a large `DELETE`: no bloat, no long transaction. (`DETACH ... CONCURRENTLY` cannot run in a transaction block.)
- **Statistics**: autovacuum analyzes partitions but never the partitioned parent. Queries that need parent-level statistics need a scheduled `ANALYZE parent`.
- Foreign keys referencing a partitioned table need 12+; exclusion constraints on partitioned tables need 17+ and must compare the partition key with `=`; unlogged partitioned tables are rejected on 18+.
- `MERGE PARTITIONS`/`SPLIT PARTITION` do not exist in any release; repartitioning means creating new partitions and moving rows.

***

#### Security

##### Roles and privileges

- **The application never connects as a superuser or as the owner of its tables.** Superusers and `BYPASSRLS` roles skip row-level security, owners skip it unless `FORCE ROW LEVEL SECURITY` is set, and owners can `DROP` and `ALTER` everything. Use an owner role for migrations and a separate DML-only role at run time.
- **Schema `public`.** Since 15, new databases give `PUBLIC` no `CREATE` privilege on `public`, and the schema is owned by `pg_database_owner` (verified: a fresh role gets `permission denied for schema public`). Databases created on 14 or older and upgraded keep the old permissive ACL: check `\dn+ public` and run `REVOKE CREATE ON SCHEMA public FROM PUBLIC`. Do not "fix" a failing deploy with `GRANT CREATE ON SCHEMA public TO PUBLIC`; create an application schema owned by the migration role.
- **Grants on existing vs future objects.** `GRANT SELECT ON ALL TABLES IN SCHEMA app TO reader` covers tables that exist now. Future tables need `ALTER DEFAULT PRIVILEGES FOR ROLE <creating role> IN SCHEMA app GRANT SELECT ON TABLES TO reader`. Default privileges apply only to objects created **by the role named in `FOR ROLE`** (default: the current role); a migration run as a different role produces tables without the grant.
- **Functions are executable by `PUBLIC` by default.** Revoke `EXECUTE` explicitly on sensitive functions. To change the default for new functions, use the **global** form, without `IN SCHEMA`:

```sql
CREATE ROLE owner_r;  CREATE ROLE other_r LOGIN;
CREATE SCHEMA priv AUTHORIZATION owner_r;
GRANT USAGE ON SCHEMA priv TO other_r;
SET ROLE owner_r;
-- BAD: per-schema default privileges can only ADD to the global defaults; this revoke does nothing
ALTER DEFAULT PRIVILEGES IN SCHEMA priv REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;
CREATE FUNCTION priv.f1() RETURNS int LANGUAGE sql RETURN 1;   -- RETURN body: 14+
-- GOOD: global form (applies to functions that owner_r creates later, in every schema)
ALTER DEFAULT PRIVILEGES REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;
CREATE FUNCTION priv.f2() RETURNS int LANGUAGE sql RETURN 2;
RESET ROLE;
SET ROLE other_r;
SELECT priv.f1();   -- 1: still executable by PUBLIC
SELECT priv.f2();   -- ERROR: permission denied for function f2
RESET ROLE;
```

- **Predefined roles instead of superuser**: `pg_monitor`, `pg_signal_backend`, `pg_read_server_files` (11+), `pg_read_all_data`/`pg_write_all_data` (14+), `pg_checkpoint` (15+), `pg_create_subscription`, `pg_use_reserved_connections` (16+), `pg_maintain` and the per-table `MAINTAIN` privilege (17+), `pg_signal_autovacuum_worker` (18+).
- **Passwords**: SCRAM (`password_encryption = scram-sha-256`, the default since 14). MD5 password hashes are deprecated in 18 (a warning on `CREATE/ALTER ROLE`). Never put a password in SQL that is logged or committed; `\password` in psql and `PQchangePassword()` (libpq 17+) send a hash.

##### `CREATEROLE` is powerful

Before 16, a role with `CREATEROLE` could alter or drop **any** non-superuser role and grant itself membership in any role, including `pg_execute_server_program`, `pg_read_server_files` or `pg_write_server_files`, which gives operating-system access: effectively superuser. Since 16 it can only manage roles on which it holds `ADMIN OPTION` (automatic for roles it creates), and only grant `CREATEDB`, `REPLICATION` or `BYPASSRLS` if it has them itself.[^16]

```sql
CREATE ROLE cr CREATEROLE LOGIN;
CREATE ROLE victim LOGIN;
SET ROLE cr;
ALTER ROLE victim PASSWORD NULL;
-- 16+: ERROR:  permission denied to alter role
--      DETAIL:  To change another role's password, the current user must have the CREATEROLE attribute and the ADMIN option on the role.
-- 15 and older: ALTER ROLE succeeds
GRANT pg_execute_server_program TO cr;
-- 16+: ERROR:  permission denied to grant role "pg_execute_server_program"
-- 15 and older: GRANT ROLE succeeds (cr can now run COPY ... PROGRAM)
RESET ROLE;
```

Review `CREATEROLE` grants like superuser grants on 15 and older; on 16+ also check `createrole_self_grant` and the membership options (`GRANT r TO u WITH INHERIT FALSE, SET TRUE`).

##### Row-level security

```sql
CREATE TABLE docs (id int, tenant text, body text);
ALTER TABLE docs ENABLE ROW LEVEL SECURITY;
ALTER TABLE docs FORCE ROW LEVEL SECURITY;        -- also applies to the table owner
CREATE POLICY tenant_isolation ON docs
    USING (tenant = current_setting('app.tenant', true))
    WITH CHECK (tenant = current_setting('app.tenant', true));
```

Verified on 18.6:[^17]

- **Superusers and `BYPASSRLS` roles always bypass** policies, even with `FORCE`. Without `FORCE ROW LEVEL SECURITY` the **table owner** bypasses them too.
- Enabling RLS without any policy denies everything to non-owners (default deny); policies combine with `OR` (permissive, the default) unless declared `AS RESTRICTIVE`.
- A policy with only `USING` also checks inserted and updated rows (`new row violates row-level security policy`); write `WITH CHECK` explicitly anyway.
- `current_setting('app.tenant', true)` returns NULL when the setting was never defined, so the policy matches nothing (fail closed). **After a transaction that used `set_config('app.tenant', 't2', true)` ends, the setting reads as `''`, not NULL**; policies must not treat `''` as a valid tenant.
- Set the tenant **inside each transaction** with `SET LOCAL app.tenant = ...` or `set_config('app.tenant', $1, true)`. A session-level `SET` leaks to the next client of a pooled connection.
- **Views run with the view owner's privileges** and so bypass RLS when the owner does. Use `WITH (security_invoker = true)` (15+) for views over RLS tables; on 14 and older make the view owner a role that is subject to the policies.
- Functions used in policies and in queries on RLS tables should be `LEAKPROOF` or simple built-in operators; a non-leakproof function can be evaluated before the policy filter and leak values through errors or notices.
- Index the columns that policies filter on; the policy predicate is added to every query.

##### `SECURITY DEFINER` functions and `search_path`

A `SECURITY DEFINER` function runs with its owner's privileges, but any unqualified name it uses is resolved through the **caller's** `search_path` unless the function pins it.[^18]

```sql
CREATE SCHEMA sec;
CREATE TABLE sec.secrets (s text);
INSERT INTO sec.secrets VALUES ('top secret');
-- BAD: lower() is resolved through the caller's search_path
CREATE FUNCTION sec.bad_norm(name text) RETURNS text
SECURITY DEFINER LANGUAGE plpgsql AS $$
BEGIN
  RETURN lower(name);
END $$;
-- GOOD: pinned search_path with pg_temp last, EXECUTE revoked from PUBLIC and granted narrowly
CREATE FUNCTION sec.good_norm(name text) RETURNS text
SECURITY DEFINER SET search_path = pg_catalog, pg_temp LANGUAGE plpgsql AS $$
BEGIN
  RETURN lower(name);
END $$;
REVOKE ALL ON FUNCTION sec.good_norm(text) FROM PUBLIC;

-- The attack: an unprivileged role with its own schema
CREATE ROLE mallory LOGIN;
GRANT USAGE ON SCHEMA sec TO mallory;           -- callers need USAGE to call sec.bad_norm
CREATE SCHEMA evil AUTHORIZATION mallory;
SET ROLE mallory;
CREATE FUNCTION evil.lower(text) RETURNS text LANGUAGE plpgsql AS $$
BEGIN
  RAISE NOTICE 'running as %: %', current_user, (SELECT string_agg(s, ',') FROM sec.secrets);
  RETURN $1;
END $$;
SET search_path = evil, pg_catalog;
SELECT sec.bad_norm('X');
-- NOTICE:  running as postgres: top secret     <- attacker code ran with the owner's privileges
RESET search_path;
RESET ROLE;
```

- Every `SECURITY DEFINER` function needs `SET search_path` listing only trusted schemas with **`pg_temp` last** (or `search_path = ''` and fully qualified names). `pg_catalog` is searched first only when it is not listed; listing it after a writable schema puts it behind. A function with `pg_temp` first was hijacked by a temporary table in the same test.
- `REVOKE EXECUTE ... FROM PUBLIC`, then `GRANT EXECUTE` to the roles that need it.
- The owner should be a dedicated role with only the privileges the function needs, not a superuser.
- **17+ applies the same idea to maintenance**: `ANALYZE`, `CREATE INDEX`, `REINDEX`, `REFRESH MATERIALIZED VIEW`, `VACUUM` and `CLUSTER` run functions with `search_path = pg_catalog, pg_temp`. Functions used in expression indexes, index predicates and materialized views must pin `search_path` or qualify every name, or they fail on 17+ (`relation "..." does not exist`).

##### SQL injection in dynamic SQL

```sql
CREATE TABLE acct (name text);
INSERT INTO acct VALUES ('alice'), ('bob');
DO $$
DECLARE n bigint; val text := $v$x' OR '1'='1$v$; tbl text := 'acct';
BEGIN
  -- BAD: value concatenated into SQL text
  EXECUTE 'SELECT count(*) FROM acct WHERE name = ''' || val || '''' INTO n;
  RAISE NOTICE 'concatenated: %', n;                    -- 2: injected
  -- BAD: format('%s') does no quoting either
  EXECUTE format('SELECT count(*) FROM acct WHERE name = ''%s''', val) INTO n;
  RAISE NOTICE 'format %%s: %', n;                      -- 2: injected
  -- GOOD: identifier through %I, value as a parameter
  EXECUTE format('SELECT count(*) FROM %I WHERE name = $1', tbl) INTO n USING val;
  RAISE NOTICE 'format %%I + USING: %', n;              -- 0
END $$;
SELECT format('%I.%I', 'public', 'My Table'), format('%L', 'O''Reilly'), format('%L', NULL);
-- public."My Table" | 'O''Reilly' | NULL
SELECT quote_ident('Users'), quote_literal('O''Reilly'), quote_literal(NULL), quote_nullable(NULL);
-- "Users" | 'O''Reilly' | NULL (SQL NULL!) | NULL (the word)
```

- `format()`: `%I` quotes identifiers, `%L` quotes literals and renders NULL as the bare word `NULL`, **`%s` inserts raw text**. Any `%s` that receives external input in SQL text is an injection.
- `quote_literal(NULL)` returns SQL NULL, so `'... = ' || quote_literal(v)` turns the whole query into NULL (`query string argument of EXECUTE is null`); use `quote_nullable()` or `%L`.
- Prefer `EXECUTE ... USING` parameters over `%L`: no quoting at all, types preserved, plans reusable.
- Quoting identifiers prevents injection, not access to other tables: validate user-chosen identifiers against an allow-list (or cast to `regclass` and check the schema).
- **In application code bind every value** (`$1`, `?`, named parameters). Identifiers and `ORDER BY` columns/directions cannot be parameters: map user input to a fixed allow-list. Escape `%`, `_` and `\` in values used with `LIKE`.

##### Other security items

- `COPY ... FROM/TO '/path'` and `COPY ... PROGRAM` run on the **database server** as its OS user and need superuser or `pg_read_server_files` / `pg_write_server_files` / `pg_execute_server_program`. Application code uses `COPY ... FROM STDIN` through the driver (or psql `\copy`); flag server-side paths or `PROGRAM` built from input as remote code execution.
- `dblink` and `postgres_fdw` connection strings built from user input are injection and SSRF vectors.
- Client connections: `sslmode=verify-full` with a trusted root certificate over any untrusted network. libpq's default `sslmode=prefer` silently falls back to plaintext and never verifies the server.
- `pg_hba.conf`: no `trust` outside local development, `scram-sha-256` instead of `md5` or `password`, narrow address ranges.
- Logs contain query text and, with `log_statement`, parameters; treat them as sensitive.

***
#### Functions, Procedures and Triggers

##### Volatility, strictness, parallel safety

- **`IMMUTABLE` is a promise PostgreSQL does not check.** The planner pre-evaluates immutable calls and indexes trust them. Verified: a SQL function `RETURNS timestamptz IMMUTABLE AS 'SELECT now()'` is accepted in an index expression, and the index is then silently wrong. Functions that read tables, depend on settings (`TimeZone`, `search_path`, `DateStyle`) or call `now()` are `STABLE` or `VOLATILE`. Built-ins that depend on settings are only `STABLE` (`to_char(timestamptz, ...)`, `timestamptz::date`) and are rejected in index expressions.
- `STABLE` for functions that read the database or settings; `VOLATILE` (the default) for anything with side effects or `random()`/`clock_timestamp()`/`nextval()`.
- `STRICT` functions return NULL for any NULL argument without running.
- User functions are `PARALLEL UNSAFE` unless declared otherwise; declare `PARALLEL SAFE` only for functions that do not write, use temporary objects or depend on session state.
- `SECURITY DEFINER` and `SET` clauses prevent inlining of `LANGUAGE sql` functions; do not add them to hot-path SQL functions that rely on inlining unless they need them.

```sql
CREATE TABLE fx (t timestamptz);
CREATE INDEX ON fx (date(t));
-- ERROR:  functions in index expression must be marked IMMUTABLE
CREATE INDEX ON fx (((t AT TIME ZONE 'UTC')::date));   -- OK: fixed zone, immutable
```

##### SQL-standard function bodies (14+)

```sql
CREATE TABLE logs (id int, msg text);
CREATE FUNCTION log_count() RETURNS bigint LANGUAGE sql STABLE
BEGIN ATOMIC
  SELECT count(*) FROM logs;
END;
DROP TABLE logs;
-- ERROR:  cannot drop table logs because other objects depend on it
```

`BEGIN ATOMIC ... END` and `RETURN expr` bodies are parsed when the function is created: references are bound to the objects that existed at that time (no `search_path` hijacking at call time) and dependencies are tracked. String bodies (`AS $$ ... $$`) are parsed only when the function runs and resolve names through the `search_path` in effect at that time. Prefer the standard form for SQL functions on 14+.

##### PL/pgSQL traps

```sql
CREATE TABLE pu (id int, name text);
INSERT INTO pu VALUES (1, 'a'), (1, 'b');
DO $$ DECLARE n text; BEGIN
  SELECT name INTO n FROM pu WHERE id = 1;          -- two rows: silently takes one, no error
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
- Raise errors with an SQLSTATE the application can map: `RAISE EXCEPTION 'insufficient funds for account %', id USING ERRCODE = 'P0001', HINT = '...'`. Catch specific conditions (`WHEN unique_violation THEN`), not `WHEN OTHERS` that swallows everything.
- Every entry into a block with an `EXCEPTION` clause starts a subtransaction. In a per-row loop this is slow, and more than 64 subtransactions in one transaction overflow the per-backend cache and slow down snapshots for everyone (notably on hot standbys). The same applies to drivers that issue a `SAVEPOINT` per statement ("autosave" modes).
- Row-by-row loops that issue one `UPDATE` per row are an anti-pattern; write one set-based statement.
- Procedures (11+, `CALL`) can `COMMIT`/`ROLLBACK` inside; functions cannot. A procedure called inside an explicit transaction block cannot commit either (`invalid transaction termination`).

##### Triggers

- A `BEFORE ... FOR EACH ROW` trigger that returns NULL silently skips the row; it must `RETURN NEW` (or `OLD` for `DELETE`) to proceed.
- `AFTER` row triggers queue an event per row in memory for the whole statement; statements over millions of rows can exhaust memory. Statement-level triggers with transition tables (`REFERENCING NEW TABLE AS new_rows`, 10+) scale better for bulk changes.
- Triggers that write other tables hide work from the reader of the application code and can cascade; keep them for invariants and auditing, not business workflows.
- `updated_at` maintenance in a `BEFORE UPDATE` trigger with `NEW.updated_at = now()` gives the transaction start time; use `clock_timestamp()` if the actual write time matters.
- On 18+, deferred and other `AFTER` triggers run as the role that queued the event, not the role active at commit; audit triggers that record `current_user` behave differently after an upgrade.
- `CREATE OR REPLACE TRIGGER` is 14+; on older versions use `DROP TRIGGER IF EXISTS` + `CREATE TRIGGER` in one transaction.
- Event triggers (DDL; `login` on 17+) run for every matching event; a failing login event trigger blocks all logins (the escape hatch is `event_triggers = off`, 17+).

***

#### Vacuum, Bloat and Maintenance

##### How dead rows accumulate

Every `UPDATE` and `DELETE` leaves a dead row version. Autovacuum removes dead versions that no running transaction can see any more, updates the visibility map (needed for index-only scans) and freezes old transaction IDs. Three things stop it from removing anything, cluster-wide:[^19]

```sql
-- 1. Long-running or idle-in-transaction sessions (oldest snapshot)
SELECT pid, state, now() - xact_start AS xact_age, backend_xmin, left(query, 60) AS query
FROM pg_stat_activity
WHERE xact_start IS NOT NULL
ORDER BY xact_start
LIMIT 10;

-- 2. Replication slots that are inactive or lagging
SELECT slot_name, active, xmin, catalog_xmin,
       pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn)) AS retained_wal
FROM pg_replication_slots;

-- 3. Forgotten prepared transactions (two-phase commit)
SELECT gid, prepared, owner FROM pg_prepared_xacts;
```

An abandoned logical replication slot also retains WAL until the disk fills. Monitor slots and set `max_slot_wal_keep_size` (13+); 18 adds `idle_replication_slot_timeout`.

##### Tables that need attention

```sql
SELECT relname, n_live_tup, n_dead_tup, n_tup_upd, n_tup_hot_upd,
       last_autovacuum, last_autoanalyze, autovacuum_count
FROM pg_stat_user_tables
ORDER BY n_dead_tup DESC
LIMIT 10;

-- Transaction ID age per database (wraparound protection starts at autovacuum_freeze_max_age, default 200 million)
SELECT datname, age(datfrozenxid) AS xid_age FROM pg_database ORDER BY 2 DESC;
```

- **Autovacuum must stay on.** Turning it off for a table (`autovacuum_enabled = false`) only postpones the work into an emergency anti-wraparound vacuum.
- **Tune hot tables per table** instead of globally: `ALTER TABLE orders SET (autovacuum_vacuum_scale_factor = 0.02, autovacuum_analyze_scale_factor = 0.01);`. The default thresholds (20 % dead rows for vacuum, 10 % changed rows for analyze) are far too lax for tables with hundreds of millions of rows; 18+ caps the vacuum threshold with `autovacuum_vacuum_max_threshold` (default 100,000,000 rows).
- **HOT updates** avoid index maintenance when no indexed column changes and the new version fits on the same page. A low `n_tup_hot_upd / n_tup_upd` ratio on an update-heavy table suggests a lower `fillfactor` (for example `ALTER TABLE t SET (fillfactor = 90)`, effective for newly written pages) or fewer indexes on frequently updated columns.
- **Queue tables** (insert, update, delete within minutes) bloat quickly; keep them small, vacuum them aggressively, and consider partitioning by time with dropped partitions.
- **`VACUUM FULL`** returns space to the OS but rewrites the table under `ACCESS EXCLUSIVE`; plain `VACUUM` only makes space reusable (and truncates empty pages at the end). Use `pg_repack`/`pg_squeeze` for online compaction (built-in `REPACK` is 19 only).
- **Index bloat**: `REINDEX INDEX CONCURRENTLY` (12+). B-tree deduplication (13+) reduces bloat from duplicate keys.
- **`ANALYZE` after bulk loads**, after creating an expression index, and after `pg_upgrade` (statistics are carried over by `pg_upgrade` only from 18 on; before that the new cluster starts without statistics: run `vacuumdb --all --analyze-in-stages`).

***

#### Connection Pooling and Application Access

##### Pooling

Each connection is a server process with its own memory and snapshot cost. Use a pooler (PgBouncer, pgcat, Odyssey, RDS Proxy, or the driver's pool) and size the server-side pool to roughly the number of queries the hardware can run concurrently, not to the number of application threads. `max_connections` in the thousands is a design smell.

PgBouncer in **transaction pooling** mode gives each transaction any free server connection, so session state does not survive between transactions:[^20][^21]

| Feature | Session pooling | Transaction pooling |
|---|---|---|
| `SET` / `RESET` (session level) | works | **broken**: leaks to other clients or is lost; use `SET LOCAL` / `set_config(..., true)` |
| `LISTEN` | works | **broken** (`NOTIFY` works) |
| Session advisory locks (`pg_advisory_lock`) | works | **broken**; use `pg_advisory_xact_lock` |
| SQL `PREPARE` / `DEALLOCATE` | works | **broken** |
| Protocol-level prepared statements (driver `prepare`) | works | work with PgBouncer **1.21+** and `max_prepared_statements` > 0 (default 200 since 1.24) |
| `WITH HOLD` cursors | works | **broken** |
| Temporary tables `ON COMMIT PRESERVE/DELETE ROWS` | works | **broken** (`ON COMMIT DROP` works) |
| `LOAD` | works | **broken** |

Review any `SET search_path`, `SET statement_timeout`, `SET ROLE` or `SET app.tenant` executed outside a transaction in code that runs behind a transaction pooler; move them into the transaction with `SET LOCAL`, into the role (`ALTER ROLE ... SET`), or into connection startup parameters that the pooler tracks. Drivers that auto-prepare (JDBC `prepareThreshold`, `pgx`, `asyncpg`, `node-postgres` named statements) need PgBouncer 1.21+ with prepared statement support enabled, or prepared statements disabled in the driver.

##### Parameters and identifiers

```python
# BAD
cur.execute(f"SELECT * FROM users WHERE email = '{email}'")
# GOOD
cur.execute("SELECT * FROM users WHERE email = %s", (email,))
```

Bind every value. Map user-selected sort columns and directions to an allow-list. Use `= ANY($1)` for lists.

##### Timeouts, retries, identification

- Server-side `statement_timeout`, `lock_timeout` and `idle_in_transaction_session_timeout` per role (see above); a client-side timeout alone leaves the query running on the server.
- Retry `40001` (serialization failure) and `40P01` (deadlock) by re-running the whole transaction; do not retry `23505` (unique violation) blindly; handle connection loss before commit.
- Set `application_name` per service so `pg_stat_activity` and the logs identify the caller.
- Use `COPY ... FROM STDIN` (driver copy API) or multi-row `INSERT` for bulk writes, not row-by-row inserts in a loop.
- `sslnegotiation=direct` (libpq 17+) works only when every server, pooler and proxy on the path is 17+ and understands direct TLS; wire protocol 3.2 (`max_protocol_version=3.2`, 18+) likewise needs pooler support.

***

#### Monitoring and Observability

```sql
-- Slowest statements (needs shared_preload_libraries = 'pg_stat_statements' and CREATE EXTENSION)
-- total_exec_time / mean_exec_time: 13+ (total_time / mean_time before)
SELECT query, calls, total_exec_time, mean_exec_time, rows, shared_blks_read
FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT 10;

-- Tables with heavy sequential scans: the column is relname, not tablename
SELECT schemaname, relname, seq_scan, seq_tup_read, idx_scan, n_live_tup
FROM pg_stat_user_tables ORDER BY seq_tup_read DESC LIMIT 10;

-- Unused indexes: relname is the table, indexrelname the index
SELECT schemaname, relname, indexrelname, idx_scan,
       pg_size_pretty(pg_relation_size(indexrelid)) AS size
FROM pg_stat_user_indexes
WHERE idx_scan = 0
ORDER BY pg_relation_size(indexrelid) DESC;

-- Who blocks whom
SELECT pid, wait_event_type, wait_event, state, pg_blocking_pids(pid) AS blocked_by, left(query, 60) AS query
FROM pg_stat_activity
WHERE cardinality(pg_blocking_pids(pid)) > 0;

-- Cache hit ratio per table
SELECT relname, heap_blks_read, heap_blks_hit,
       round(heap_blks_hit * 100.0 / nullif(heap_blks_hit + heap_blks_read, 0), 1) AS hit_pct
FROM pg_statio_user_tables ORDER BY heap_blks_read DESC LIMIT 10;
```

- **`pg_stat_user_tables` and `pg_stat_user_indexes` have `relname` and `indexrelname`.** `tablename` and `indexname` are columns of the catalog views `pg_tables` and `pg_indexes`; a query that selects `tablename` from `pg_stat_user_tables` fails with `column "tablename" does not exist`.
- Version-dependent columns: `last_seq_scan`/`last_idx_scan` (16+), `n_tup_newpage_upd` (16+), `total_vacuum_time`/`total_autovacuum_time` (18+); checkpoint counters live in `pg_stat_checkpointer` on 17+ (`num_timed`, `num_requested`) and in `pg_stat_bgwriter` (`checkpoints_timed`, `checkpoints_req`) before; `pg_stat_statements` I/O timing columns are `shared_blk_read_time` on 17+ and `blk_read_time` before; `pg_stat_io` exists on 16+ and has byte columns instead of `op_bytes` on 18+. Dashboards must match the server version.
- `pg_stat_statements` normalizes constants; on 18+ it also collapses `IN` lists of different lengths into one entry.
- Statistics are per server: an index unused on the primary may serve all reads on a replica.
- Log settings worth reviewing: `log_min_duration_statement`, `log_lock_waits = on` (lock waits longer than `deadlock_timeout`), `log_temp_files = 0`, `log_autovacuum_min_duration`, and `auto_explain` for slow plans.

***

#### Upgrades and Minor Releases

- **Major upgrades** (`pg_upgrade`, dump/restore, or logical replication) change the on-disk format; read the "Migration to Version N" section of every release note between the old and new major. The table "Removed and changed behaviour across versions" lists what most often breaks application code.
- After `pg_upgrade` to 17 or older, statistics are empty until `ANALYZE` runs: `vacuumdb --all --analyze-in-stages`. From 18 on, `pg_upgrade` keeps planner statistics (not extended statistics): run `vacuumdb --all --analyze-in-stages --missing-stats-only`, then `vacuumdb --all --analyze-only`.
- Logical replication slots survive `pg_upgrade` only when the *old* cluster is 17+.
- `initdb` on 18 enables data checksums by default; `pg_upgrade` from a cluster without checksums needs the new cluster initialized with `--no-data-checksums`.
- **Collations**: an OS upgrade that changes glibc (for example to 2.28) can silently change text sort order and corrupt indexes on text columns. Check with `amcheck`/`pg_amcheck` or reindex after such an upgrade; `ALTER DATABASE ... REFRESH COLLATION VERSION` (15+) only updates the recorded version.
- **Minor releases** are drop-in binary updates, but some ask for actions (a `REINDEX` of certain index types, re-creating constraints). Read the release notes of every skipped minor. The August 13, 2026 releases (18.6, 17.11, 16.15, 15.19, 14.24) require logical decoding output plugins other than `pgoutput`/`test_decoding` to be listed in the new `output_plugin_libraries` setting (third-party plugins such as `wal2json` stop working otherwise) and recommend reindexing `btree_gist` indexes on `float4`/`float8` columns that may contain `NaN` (and, on 18, B-tree indexes over `ltree` values with very many labels).[^4]

***

#### Production / Review Checklist

##### Version

- [ ] Server major version identified (image tag, IaC, CI); not 13 or older; 14 has an upgrade plan before November 12, 2026
- [ ] Minor version current (18.6, 17.11, 16.15, 15.19, 14.24 as of October 2026); no `postgres:latest`
- [ ] Every version-labelled feature used or suggested is available on the pinned version; no 19 features (`REPACK`, `ON CONFLICT DO SELECT`, `IGNORE NULLS`, `WAIT FOR LSN`)
- [ ] Code shared across versions (libraries, extensions, migration sets) avoids features newer than the oldest supported server

##### Migrations

- [ ] `lock_timeout` (and `statement_timeout`) set for DDL on existing tables; runner retries on lock timeout
- [ ] One risky statement per transaction; no long work after an `ACCESS EXCLUSIVE` statement in the same transaction
- [ ] No rewrites of large tables: volatile defaults, `int` → `bigint`, narrowing types, stored generated columns, `SET EXPRESSION`
- [ ] Indexes created/dropped `CONCURRENTLY`, outside transaction blocks; invalid leftovers handled (`indisvalid`)
- [ ] Foreign keys and `CHECK`s added `NOT VALID` and validated separately; `SET NOT NULL` preceded by a validated `CHECK (col IS NOT NULL)` (or `NOT NULL ... NOT VALID` on 18+)
- [ ] Unique constraints and primary keys via `CREATE UNIQUE INDEX CONCURRENTLY` + `... USING INDEX`
- [ ] Backfills and deletes in bounded batches; expand/contract for renames and drops; enum changes treated as irreversible
- [ ] No passwords or secrets in migration files

##### Schema and types

- [ ] Primary key on every table; `bigint` identity (not `serial`) or `uuid` keys
- [ ] `timestamptz` for instants; half-open time ranges; explicit time zone for day boundaries
- [ ] `numeric` or integer minor units for money; no `float`/`money` for exact values; no `char(n)`, `timetz`
- [ ] `text` (with `CHECK` if a limit is a business rule) instead of arbitrary `varchar(n)`
- [ ] `uuid` type (not text) for UUIDs
- [ ] Foreign-key columns indexed on the referencing side; `ON DELETE` actions deliberate
- [ ] On 18+: `STORED` written explicitly for generated columns that are indexed, unique, referenced or replicated

##### Queries

- [ ] Values bound as parameters; identifiers and sort orders allow-listed; `= ANY($1)` for lists
- [ ] `NOT EXISTS` instead of `NOT IN (subquery)`; `coalesce` around aggregates that may see zero rows
- [ ] Predicates sargable; parameter types match column types
- [ ] Keyset pagination with a unique tie-breaker; `ORDER BY` with every `LIMIT`
- [ ] Window functions with deterministic ordering and the intended frame
- [ ] Recursive CTEs protected against cycles
- [ ] Performance claims backed by `EXPLAIN (ANALYZE, BUFFERS)`; DML explained inside `BEGIN ... ROLLBACK`
- [ ] No session-level `enable_*` toggles in application code

##### Transactions and concurrency

- [ ] Read-modify-write made atomic (`SET x = x + ...`, `FOR UPDATE`, or version checks)
- [ ] Uniqueness enforced by constraints, not by check-then-insert
- [ ] Upserts use `INSERT ... ON CONFLICT` with an exact conflict target; input deduplicated; `MERGE` not used for concurrent single-key upserts
- [ ] `MERGE ... WHEN NOT MATCHED BY SOURCE` (17+) guarded by a condition
- [ ] Retry loops for `40001`/`40P01` with `REPEATABLE READ`/`SERIALIZABLE`
- [ ] Queues use `FOR UPDATE SKIP LOCKED`; advisory locks transaction-scoped
- [ ] Short transactions; no network calls inside; per-role `statement_timeout`, `idle_in_transaction_session_timeout`

##### JSON

- [ ] `jsonb`, not `json`; filtered keys indexed (GIN with `@>`/`@?`, or expression B-tree)
- [ ] No `jsonb_set(..., NULL)` data loss; `->` vs `->>` correct
- [ ] SQL/JSON functions (17+) use `ERROR ON ERROR` where a silent NULL is wrong

##### Security

- [ ] Application role is not superuser, owner or `BYPASSRLS`; DML-only grants; default privileges set for the creating role
- [ ] `public` schema not writable by `PUBLIC` (check databases that predate 15)
- [ ] Default `EXECUTE` revoked with the global `ALTER DEFAULT PRIVILEGES` form, not `IN SCHEMA`
- [ ] `CREATEROLE` treated as near-superuser on 15 and older; `ADMIN OPTION` rules respected on 16+
- [ ] RLS tables: `FORCE ROW LEVEL SECURITY`, explicit `WITH CHECK`, tenant set with `SET LOCAL`/`set_config(..., true)`, `''` not accepted as a tenant
- [ ] Views over protected tables use `security_invoker = true` (15+)
- [ ] `SECURITY DEFINER` functions: pinned `search_path` with `pg_temp` last, `EXECUTE` revoked from `PUBLIC`
- [ ] Dynamic SQL uses `format('%I')` + `EXECUTE ... USING`; no `%s` or `||` with values
- [ ] No server-side `COPY` file/`PROGRAM` from application paths; `sslmode=verify-full`; SCRAM passwords

##### Operations

- [ ] Pooling mode compatible with the code (no session state in transaction pooling; PgBouncer 1.21+ for driver prepared statements)
- [ ] Autovacuum on; hot tables tuned per table; long transactions, stale replication slots and prepared transactions monitored
- [ ] Partitioned tables: keys include the partition key, queries filter on the raw key, partitions pre-created, default partition watched, parents analyzed
- [ ] Monitoring queries use `relname`/`indexrelname` and the column names of the server's version
- [ ] `ANALYZE` after bulk loads and after `pg_upgrade`

***

[^1]: PostgreSQL Versioning Policy (supported versions, first and final release dates). https://www.postgresql.org/support/versioning/
[^2]: PostgreSQL Roadmap (next minor release November 12, 2026). https://www.postgresql.org/developer/roadmap/
[^3]: PostgreSQL 19 RC 1 Released! (October 15, 2026; RC 2 on November 12, GA planned November 19, 2026) and PostgreSQL 19 Release Notes. https://www.postgresql.org/about/news/postgresql-19-rc-1-released-3409/ , https://www.postgresql.org/docs/19/release-19.html
[^4]: PostgreSQL 18.6, 17.11, 16.15, 15.19, 14.24 and 19 Beta 3 Released (August 13, 2026). https://www.postgresql.org/about/news/postgresql-186-1711-1615-1519-1424-and-19-beta-3-released-3365/
[^5]: PostgreSQL Release Notes, 9.5 through 18 (feature and incompatibility lists). https://www.postgresql.org/docs/release/
[^6]: Explicit Locking (table-level lock modes and conflicts). https://www.postgresql.org/docs/current/explicit-locking.html
[^7]: ALTER TABLE (lock levels, NOT VALID, VALIDATE CONSTRAINT, rewrites). https://www.postgresql.org/docs/current/sql-altertable.html
[^8]: CREATE INDEX (Building Indexes Concurrently, ON ONLY). https://www.postgresql.org/docs/current/sql-createindex.html
[^9]: Date/Time Types. https://www.postgresql.org/docs/current/datatype-datetime.html
[^10]: PostgreSQL Wiki: Don't Do This (timestamp, timetz, char(n), money, NOT IN, BETWEEN, serial). https://wiki.postgresql.org/wiki/Don%27t_Do_This
[^11]: Using EXPLAIN. https://www.postgresql.org/docs/current/using-explain.html
[^12]: Transaction Isolation (Read Committed re-evaluation example, serialization failures). https://www.postgresql.org/docs/current/transaction-iso.html
[^13]: INSERT (ON CONFLICT). https://www.postgresql.org/docs/current/sql-insert.html
[^14]: MERGE. https://www.postgresql.org/docs/current/sql-merge.html
[^15]: Table Partitioning. https://www.postgresql.org/docs/current/ddl-partitioning.html
[^16]: CREATE ROLE (CREATEROLE) and Role Attributes; PostgreSQL 16 Release Notes. https://www.postgresql.org/docs/current/sql-createrole.html , https://www.postgresql.org/docs/release/16.0/
[^17]: Row Security Policies. https://www.postgresql.org/docs/current/ddl-rowsecurity.html
[^18]: CREATE FUNCTION: Writing SECURITY DEFINER Functions Safely. https://www.postgresql.org/docs/current/sql-createfunction.html
[^19]: Routine Vacuuming (dead tuples, visibility map, transaction ID wraparound). https://www.postgresql.org/docs/current/routine-vacuuming.html
[^20]: PgBouncer features (SQL feature map for pooling modes). https://www.pgbouncer.org/features.html
[^21]: PgBouncer changelog (1.21.0: prepared statements; 1.24.0: max_prepared_statements default 200). https://www.pgbouncer.org/changelog.html
