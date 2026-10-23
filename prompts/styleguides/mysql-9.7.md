### MySQL 9.7 LTS: The Complete SQL Review Guide

#### Overview

> **Version**: MySQL 9.7 LTS. 9.7.0 became generally available on **April 21, 2026**.[^1][^2] The latest release is **9.7.4 (October 20, 2026)**. The series so far: 9.7.0 (2026-04-21), 9.7.1 (2026-06-16, Critical Security Patch Update), 9.7.2 (2026-07-28), 9.7.3 (2026-08-18, Critical Security Patch Update for the Docker image only), 9.7.4 (2026-10-20).[^2]
> **Support**: 9.7 is a Long-Term Support series under the Oracle Lifetime Support Policy: five years of Premier Support and three years of Extended Support,[^3] i.e. Premier Support until **April 2031** and Extended Support until **April 2034**.[^4]
> **Landscape as of October 2026**: 9.7 is the newest LTS and the **last release line with sequential version numbers**. Releases after it use calendar versions (`YY.M.P`): the Innovation releases **26.7.0** (July 28, 2026), **26.7.1** (August 18, 2026) and **26.10.0** (October 20, 2026) are newer than 9.7 but are *not* 9.7 patches.[^3][^5] The previous LTS, 8.4, is still supported (latest 8.4.13, October 20, 2026); MySQL 8.0 reached End of Life in April 2026 (last release 8.0.46, April 21, 2026).[^6] Supported upgrade path: 8.4 LTS → 9.7 LTS; an LTS series cannot be skipped.[^3]
> **Scope**: SQL that runs on MySQL 9.7 with InnoDB: schema and index design, migrations and online DDL, queries and DML in application code, transactions and locking, stored programs, security, and everything that changed relative to 8.4 LTS (all of the 9.0–9.6 Innovation releases are folded into 9.7.0). Every example in this guide was executed on **MySQL 9.7.4** (Community Server, official `mysql:9.7` image); boundary claims were checked on **8.4.13**.
> **Note**: This document is self-contained. Sections that cover behaviour that is new or different compared with 8.4 are marked **New in 9.7** (or with the 9.x release that introduced it).

Apply this guide when a project pins MySQL 9.7.x (`mysql:9.7`, `mysql:9.7.4`, `engine_version = "9.7..."`). A project that pins a 9.0–9.6 Innovation release runs an out-of-support server (an Innovation release is supported only until the next one ships);[^3] most of this guide applies, but check the "Introduced in" column of the tables below before relying on a feature, and recommend moving to 9.7 LTS. **Treat every feature introduced in 26.7 or later as unavailable** (see "Do not use or recommend").

Global truths about MySQL 9.7 with InnoDB:

- **The table is the primary key.** InnoDB stores rows in a B-tree clustered on the primary key; every secondary index entry carries the full primary key. A wide or random primary key makes every index bigger and every insert slower.
- **REPEATABLE READ is the default isolation level.** Plain `SELECT`s read a consistent snapshot; `UPDATE`, `DELETE` and locking reads (`FOR UPDATE`/`FOR SHARE`) read the latest committed version and take next-key (record + gap) locks on every index record they scan.
- **DDL is not transactional.** Every DDL statement implicitly commits the current transaction. A single DDL statement is atomic (crash-safe), but a migration of several statements is not.
- **Strict mode is on by default.** `sql_mode` is `ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION`. `INSERT IGNORE`, `UPDATE IGNORE` and a session that clears `sql_mode` silently turn errors back into truncation warnings.
- **`utf8mb4` / `utf8mb4_0900_ai_ci` is the default.** That collation is accent-insensitive, case-insensitive and `NO PAD`. `utf8` still means the deprecated 3-byte `utf8mb3` unless the session enables the new `INTERPRET_UTF8_AS_UTF8MB4` SQL mode.
- **Foreign keys are enforced however they are written.** Since 9.0, a column-level `REFERENCES` clause creates a real constraint (on 8.4 it was silently ignored), and since 9.6 foreign keys are handled by the SQL layer instead of InnoDB.
- **GTIDs are on by default** (`gtid_mode=ON`, `enforce_gtid_consistency=ON`), so statements that are unsafe for GTID replication fail with `ERROR 1785` even on a server that has no replicas.
- **`EXPLAIN` prints a tree by default** (`explain_format=TREE`) and `FORMAT=JSON` uses the access-path layout (`explain_json_format_version=2`).
- **`mysql_native_password` is gone from the server**, `MD5()`/`SHA1()` only exist when the `classic_hashing` component is installed, and the `mysql` command-line client no longer processes most client commands (`source`, `use`, `\G`, `status`, `system`, ...) unless started with `--commands`.
- **Still no `RETURNING`, no `MERGE`, no `FULL OUTER JOIN`, no `IS DISTINCT FROM`, no `ADD COLUMN IF NOT EXISTS`, no partial or `INCLUDE` indexes, no sequences, no transactional DDL.** Code or advice copied from PostgreSQL or MariaDB does not run.

***

#### What's New in MySQL 9.7 (since 8.4)

MySQL 9.7.0 is the first LTS after 8.4 and bundles everything from the 9.0–9.6 Innovation releases. LTS rules: features are added and removed only in the first LTS release (9.7.0); later 9.7.x releases contain fixes, and in-place upgrade and downgrade within 9.7.x is supported.[^3] In practice a few 9.7.x patches changed observable behaviour; they are listed separately below. The manual's "What Is New in MySQL 9.7" page only compares 9.7 with 9.6; the full list of changes since 8.4 is spread over the 9.0.0–9.6.0 release notes, summarized here.[^1][^7]

##### Breaking changes for application SQL and migrations [New in 9.7]

| Change | 8.4 behaviour | 9.7 behaviour | Introduced |
|---|---|---|---|
| Inline column `REFERENCES` | Parsed and **silently ignored**: no constraint | **Creates and enforces a foreign key** (`INSERT` of an orphan → `ERROR 1452`). `REFERENCES parent` without a column list references the parent's primary key | 9.0.0[^8] |
| `ALTER TABLE ... ADD COLUMN c INT REFERENCES p (id)` | Instant metadata change; no constraint | Adds a foreign key: `ALGORITHM=INSTANT`/`INPLACE` are rejected (`ERROR 1846 ... Adding foreign keys needs foreign_key_checks=OFF`), without an `ALGORITHM` clause the table is **copied** | 9.0.0 (tested) |
| `mysql_native_password` | Server plugin present but disabled; `--mysql-native-password=ON` re-enables it | Server plugin **removed**, together with `--mysql-native-password` and `--mysql-native-password-proxy-users`; such accounts cannot log in; the client-side plugin remains | 9.0.0[^8] |
| `MD5()`, `SHA1()` | Built-in functions | Deprecated in 9.4.0, **moved to the `classic_hashing` component** in 9.6.0: `ERROR 1305 FUNCTION db.MD5 does not exist` until `INSTALL COMPONENT 'file://component_classic_hashing'` | 9.4.0 / 9.6.0[^9][^10] |
| `EXTERNAL`, `LIBRARY` | Plain identifiers | **Reserved words**: `CREATE TABLE external (...)` / `library (...)` → `ERROR 1064`; quote them (the only differences between the reserved-word lists of 8.4.13 and 9.7.4, tested via `information_schema.keywords`; `CUBE`, `QUALIFY`, `TABLESAMPLE` are reserved in both) | 9.x[^11] |
| `IGNORE` and `ER_SUBQUERY_NO_1_ROW` | `INSERT/UPDATE/DELETE IGNORE` downgrade "Subquery returns more than 1 row" to a warning | **`ERROR 1242`** even with `IGNORE` | 9.0.0[^8] |
| Mixing non-transactional (MyISAM, MEMORY) and InnoDB writes in one transaction | Allowed (GTIDs off by default) | With the new default `gtid_mode=ON`/`enforce_gtid_consistency=ON`: `ERROR 1785 Statement violates GTID consistency`; also deprecated (warning 6414) | 9.0.0 / 9.5.0[^8][^12] |
| `explain_format` / `explain_json_format_version` | `TRADITIONAL` / `1` | **`TREE` / `2`**: plain `EXPLAIN` prints the iterator tree; JSON plans have a different schema (`$.query_plan...`, not `$.query_block...`) | 9.5.0[^12] |
| `mysql` client `--commands` | Enabled (8.4.6+) | **Disabled by default** (since 9.4.0): `source`, `use` (sent to the server instead), `\G`, `status`, `system`, `tee`, `pager`, `warnings`, `connect`, `quit`, ... become SQL syntax errors; only `delimiter` and `\C` still work | 9.4.0[^9][^13] |
| Column references through a derived table qualified with the schema (`SELECT db.dt.a FROM (...) AS dt`) | Accepted | `ERROR 1054 Unknown column` | 9.1.0[^14] |
| Outer references in a window's `PARTITION BY`/`ORDER BY` | Accepted | `ERROR 6432 Outer reference ... in window's ORDER BY or PARTITION BY clause not allowed` | 9.2.0[^15] |
| Nondeterministic `ORDER BY` with `RANGE` window frames (`ORDER BY RAND() RANGE ...`) | Accepted | Rejected (`ERROR 3587`) | 9.4.0[^9] |
| `ROLLUP` over an empty input | Empty result | One grand-total row (`NULL`, `0` for `COUNT(*)`) | 9.1.0[^14] |
| `SUM(DISTINCT b'1100')` | 9 (wrong) | 12 | 9.1.0[^14] |
| `SELECT (SELECT 1 LIMIT 1 OFFSET 10)` | 1 (wrong) | `NULL` | 9.2.0[^15] |
| `REPLACE` of a referenced parent row with an unchanged key | Cascades `ON DELETE` to children, or fails with `ERROR 1451` (`NO ACTION`/`RESTRICT`) | Neither: children are kept, statement succeeds (SQL-layer foreign keys) | 9.6.0 (tested) |
| `GROUPING()` in a query without `ROLLUP` | `ERROR 1111 Invalid use of group function` | Allowed (returns 0) | 9.6.0[^10] |
| Removed server variables | `default_authentication_plugin`, `innodb_log_file_size`, `innodb_log_files_in_group`, `innodb_undo_tablespaces`, `temptable_use_mmap`, `replica_parallel_type`, `group_replication_allow_local_lower_version_join` exist (deprecated) | `ERROR 1193 Unknown system variable`; a config file that still sets them stops the server from starting. Use `innodb_redo_log_capacity` for redo sizing | 9.0.0–9.5.0[^8][^16][^9][^12] |
| Removed plugins | `semisync_master`/`semisync_slave`, Version Tokens | Removed (use `semisync_source`/`semisync_replica`) | 9.3.0 / 9.5.0[^16][^12] |
| `replica_parallel_workers = 0` | Allowed | Minimum is 1 | 9.3.0[^16] |
| `BINLOG` as an unquoted stored-program label | Allowed | Syntax error | 9.2.0[^15] |
| `SPATIAL` indexes | — | A 9.2.0 fix for spatial-index corruption requires dropping and re-creating spatial indexes during the upgrade (before the affected tables are used) | 9.2.0[^15] |
| Foreign-key violations and `AUTO_INCREMENT` | A child insert rejected with `ERROR 1452` still consumes an `AUTO_INCREMENT` value | The SQL layer checks before the row is written: no value is consumed (tested: next id 1 on 9.7.4, 2 on 8.4.13) | 9.6.0[^46] |
| Downgrade between Innovation releases | — | Not possible, not even within one Innovation series; within 9.7.x LTS in-place downgrade is supported | 9.3.0[^16][^3] |

```sql
-- MySQL 9.7.4 (8.4.13 results in the comments where they differ)
CREATE TABLE person (id INT PRIMARY KEY, name VARCHAR(20));
CREATE TABLE pet (id INT PRIMARY KEY, owner INT REFERENCES person (id));
INSERT INTO pet VALUES (1, 999);
-- ERROR 1452 (23000): Cannot add or update a child row: a foreign key constraint fails
--   (`t`.`pet`, CONSTRAINT `pet_ibfk_1` FOREIGN KEY (`owner`) REFERENCES `person` (`id`))
--   (8.4.13: row inserted, no constraint exists)

CREATE USER legacy@'%' IDENTIFIED WITH mysql_native_password BY 'x';
-- ERROR 1524 (HY000): Plugin 'mysql_native_password' is not loaded   (same text on 8.4)
SELECT MD5('a');             -- ERROR 1305 (42000): FUNCTION t.MD5 does not exist   (8.4: works)
CREATE TABLE external (id INT);   -- ERROR 1064   (8.4: OK)
CREATE TABLE `external` (id INT); -- OK when quoted
SELECT @@innodb_log_file_size;    -- ERROR 1193 (HY000): Unknown system variable 'innodb_log_file_size'
```

##### New SQL features in 9.0–9.7 [New in 9.7]

| Feature | Introduced | Edition |
|---|---|---|
| `VECTOR(n)` data type; `STRING_TO_VECTOR()`/`TO_VECTOR()`, `VECTOR_TO_STRING()`/`FROM_VECTOR()`, `VECTOR_DIM()` | 9.0.0[^8] | All |
| Enforced inline `REFERENCES`; `REFERENCES parent` without a column list | 9.0.0[^8] | All |
| `EXPLAIN ANALYZE FORMAT=JSON INTO @var` (requires `explain_json_format_version=2`, now the default) | 9.0.0[^8] | All |
| `PREPARE` of `CREATE EVENT`, `ALTER EVENT`, `DROP EVENT` (no `?` placeholders) | 9.0.0[^8] | All |
| Correlated subqueries with `LIMIT 1` can be transformed into derived tables | 9.0.0[^8] | All |
| `CREATE VIEW IF NOT EXISTS` (not combinable with `OR REPLACE`) | 9.1.0[^14] | All |
| `GROUP BY ROLLUP (a, b)` syntax (also on 8.4.x) | 9.1.0[^14] | All |
| `table_open_cache_triggers`: triggers are parsed only for statements that modify data | 9.1.0[^14] | All |
| Max `INSTANT` row versions per table raised from 64 to **255** | 9.1.0[^14] | All |
| `CREATE_SPATIAL_REFERENCE_SYSTEM` privilege (instead of `SUPER`) | 9.2.0[^15] | All |
| `subquery_to_derived` handles all `>ANY`/`<ALL`/... quantified comparisons, also in the `SELECT` list | 9.3.0[^16] | All |
| `mysqldump --users` (dumps `CREATE USER`/`GRANT`) | 9.3.0[^16] | All |
| `CREATE JSON DUALITY VIEW`, `JSON_DUALITY_OBJECT()`, `ETAG()`; DML on duality views | 9.4.0 (DML in Community: 9.7.0)[^9][^1] | All |
| `activate_mandatory_roles` (default `ON`) | 9.5.0[^12] | All |
| `INTERPRET_UTF8_AS_UTF8MB4` SQL mode | 9.5.0[^12] | All |
| `GROUPING()` without `ROLLUP` | 9.6.0[^10] | All |
| SQL-layer foreign keys (InnoDB handling only with the read-only startup option `innodb_native_foreign_keys=ON`) | 9.6.0[^10] | All |
| **Hypergraph optimizer** (`optimizer_switch='hypergraph_optimizer=on'`) and therefore **`QUALIFY`** | 9.7.0[^1] | All (opt-in) |
| `enable_cascade_triggers` (FK `CASCADE`/`SET NULL` fire child triggers) | 9.7.0[^1] | All |
| `sql_require_primary_key` / `sql_generate_invisible_primary_key` accept a `UNIQUE NOT NULL` key as primary-key equivalent | 9.7.0[^1] | All |
| `caching_sha2_password_storage_format` (`CRYPT5` default, `PBKDF2` with SHA-512), `caching_sha2_password_enforce_storage_format` | 9.7.0[^1][^17] | All |
| `replica_allow_higher_version_source` | 9.7.0[^1] | All |
| Telemetry component, Replication Applier Metrics and Group Replication resource-manager / flow-control / primary-election components (previously Enterprise) | 9.7.0[^1] | All |
| `VERSION_WITH_UPDATE()`, `@@version_with_update` (four-part version, e.g. `9.7.4.0`) | **9.7.4**[^2] | All |
| JavaScript stored programs (`LANGUAGE JAVASCRIPT`), `CREATE LIBRARY` / `USING (lib)`, WebAssembly libraries (`LANGUAGE WASM`) | 9.0.0 / 9.2.0 / 9.4.0[^8][^15][^9] | **Enterprise only** (MLE component) |
| Dynamic data masking (`CREATE MASKING POLICY`, needs the `object_policy` component); `CURRENT_ROLE_IN()`, `CURRENT_USER_IN()` | 9.7.0[^1] | Masking policies: **Enterprise only**; the two functions also exist in Community |

```sql
-- MySQL 9.7.4
CREATE VIEW v1 AS SELECT 1 AS a;
CREATE VIEW IF NOT EXISTS v1 AS SELECT 2 AS a;   -- Note 1050 Table 'v1' already exists; v1 unchanged
CREATE OR REPLACE VIEW IF NOT EXISTS v1 AS SELECT 3 AS a;   -- ERROR 1064

SET @r = 0;
PREPARE ps FROM 'CREATE EVENT IF NOT EXISTS e1 ON SCHEDULE EVERY 1 DAY DO SET @r = 1';
EXECUTE ps;                                       -- OK (8.4: ERROR 1295 at PREPARE)

CREATE TABLE sales (y INT, c VARCHAR(10), amt INT);
SELECT y, COUNT(*) FROM sales GROUP BY y WITH ROLLUP;   -- NULL | 0   (8.4: empty set)
INSERT INTO sales VALUES (2024,'de',10),(2024,'fr',5),(2025,'de',7);
SELECT y, GROUPING(y) FROM sales GROUP BY y;            -- 2024 | 0 ; 2025 | 0   (8.4: ERROR 1111)
SELECT VERSION(), VERSION_WITH_UPDATE();                -- 9.7.4 | 9.7.4.0
```

##### Changed server defaults [New in 9.7]

The rows down to `mysql` client options were observed on 8.4.13 and 9.7.4 in the same 4-CPU container; the replication-TLS row is taken from the release notes, the container-sizing row from the release notes plus a test with `--cpuset-cpus`/`--memory` limits:

| Variable / option | 8.4 | 9.7 | Changed in |
|---|---|---|---|
| `explain_format` | `TRADITIONAL` | `TREE` | 9.5.0[^12] |
| `explain_json_format_version` | 1 | 2 | 9.5.0[^12] |
| `gtid_mode` / `enforce_gtid_consistency` | `OFF` / `OFF` | `ON` / `ON` | 9.5.0[^12] |
| `innodb_change_buffering` | `none` | `all` | 9.5.0[^12] |
| `innodb_change_buffer_max_size` | 25 | 5 | 9.4.0[^9] |
| `caching_sha2_password_digest_rounds` | 5000 | 10000 | 9.5.0[^12] |
| `activate_mandatory_roles` | — | `ON` (mandatory roles become active at login) | 9.5.0[^12] |
| `binlog_transaction_dependency_history_size` | 25000 | 10000000 (the 9.5.0 release note says 1,000,000; the manual and the server say 10,000,000) | 9.5.0[^12][^18] |
| `SOURCE_SSL` (`CHANGE REPLICATION SOURCE TO`), `group_replication_ssl_mode`, `group_replication_recovery_use_ssl` | 0 / `DISABLED` / `OFF` | 1 / `REQUIRED` / `ON` | 9.5.0[^12] |
| `innodb_log_writer_threads` | `ON` | `OFF` unless the server has more than 32 logical CPUs with binary logging on (more than 4 without) | 9.1.0 / 9.5.0[^14][^12] |
| `back_log` | 151 | 10000 | 9.4.0[^9] |
| `mysql` client `--commands` / `--system-command` | `ON` / `ON` (8.4.13 client) | `OFF` / `OFF` | 9.4.0 / 9.1.0[^9][^14] |
| InnoDB sizing (`innodb_buffer_pool_instances`, `innodb_page_cleaners`, `innodb_purge_threads`, `innodb_read_io_threads`, `temptable_max_ram`, ...) | From host CPUs/RAM | Derived from the *available* logical CPUs and memory (9.3.0; `cpuset` affinity counted since 9.7.0), but cgroup CPU/memory **limits are ignored unless the server runs with `--container-aware=ON`** (9.6.0, default `OFF`; tested: 9.7.4 logs `Server ignores the discovered container restrictions as --container_aware is OFF`) | 9.3.0 / 9.6.0 / 9.7.0[^16][^10][^1] |

Unchanged and still relevant for review: `transaction_isolation=REPEATABLE-READ`, `binlog_format=ROW` (the variable is deprecated: warning 1287 on every read), `innodb_autoinc_lock_mode=2`, `innodb_lock_wait_timeout=50`, `lock_wait_timeout=31536000` (one year), `explicit_defaults_for_timestamp=ON`, `local_infile=OFF`, `activate_all_roles_on_login=OFF`, `group_concat_max_len=1024`, `cte_max_recursion_depth=1000`, `block_encryption_mode=aes-128-ecb`, `innodb_adaptive_hash_index=OFF`, `restrict_fk_on_non_standard_key=ON`, `temptable_max_mmap=0`, `optimizer_switch` with `hypergraph_optimizer=off`.

##### Behaviour changes inside the 9.7 LTS series

| Release | Change |
|---|---|
| 9.7.0 | Stricter InnoDB row-size calculation: some `CREATE`/`ALTER TABLE` statements that exceed documented limits are rejected (`ER_TOO_BIG_ROWSIZE`)[^1] |
| 9.7.1 | Critical Security Patch Update only[^2] |
| 9.7.2 | `CUBE` becomes a reserved word (with `EXTERNAL`, `QUALIFY`, `TABLESAMPLE`; `MANUAL` and `PARALLEL` become non-reserved), so an unquoted `cube` identifier that worked on 9.7.0/9.7.1 fails; `POW(x, NULL)` returns `NULL` instead of 1; `IN` lists of `DATE`/`TIME` constants no longer match unrelated strings; fix: adding an `AUTO_INCREMENT` column to an existing table could skip records and assign inaccurate values; fixes for wrong results with reverse range scans + `ORDER BY ... LIMIT`, `UNION` with pushed-down `IS NULL` conditions, nested merged views with outer joins, and `JSON_VALUE(... RETURNING DATETIME DEFAULT ...)` in a `DATE` context; hypergraph: `ORDER BY RAND()` or side-effecting functions are evaluated once per row; `thread_pool_max_unused_threads` default 2 → 32 (Enterprise thread pool); `group_replication_communication_stack` and `group_replication_ip_allowlist` deprecated[^19] |
| 9.7.3 | Critical Security Patch Update for the Docker image only[^2] |
| 9.7.4 | Four-part version (`VERSION_WITH_UPDATE()`); the 9.7.0 row-size check is relaxed back to the legacy calculation except for `ALTER TABLE ... ALGORITHM=INSTANT`; `ALTER TABLE ... EXCHANGE PARTITION` validates rows against the destination's `CHECK` constraints in both directions even `WITHOUT VALIDATION` (`ERROR 3819`) and rejects tables whose generated columns differ; fixes: hypergraph single-table `UPDATE`/`DELETE` with `ORDER BY` and a semijoin/antijoin subquery could **modify the wrong rows**; `index_merge_union=off` plans could drop a disjunct and return incomplete results[^20] |

Run the latest 9.7.x. In particular, do not enable the hypergraph optimizer on anything older than 9.7.4.

##### Version-aware review

- Determine the exact server version from the project (Docker image tag, `docker-compose.yml`, Helm values, Terraform `engine_version`, CI service containers). `mysql:9.7` floats to the newest 9.7.x; a pinned `mysql:9.7.0` lacks `VERSION_WITH_UPDATE()`, the 9.7.2 and 9.7.4 wrong-result fixes and the relaxed row-size validation of 9.7.4.
- Docker Hub tags as of October 2026: `mysql:9.7`, `mysql:9` and `mysql:lts` all point to 9.7.4, but `mysql:latest` and `mysql:innovation` point to the calendar-versioned Innovation release 26.10.0 (and `mysql:26` follows the 26.x line). A project that wants the LTS must pin `mysql:9.7` (or `lts`), never `latest`.
- Before claiming that syntax or a function is unavailable, check it against the 9.7 manual or the release notes. Say "not available in MySQL 9.7" only when the pinned version proves it. Many PostgreSQL-isms are genuinely missing, but `INTERSECT`, `EXCEPT`, `LATERAL`, `JSON_TABLE`, `VALUES ROW(...)`, window functions, recursive CTEs, `CHECK` constraints, functional and descending indexes, `CREATE VIEW IF NOT EXISTS` and (with the hypergraph optimizer) `QUALIFY` all exist.
- Distinguish **Community** from **Enterprise**: JavaScript/WebAssembly stored programs and libraries, masking policies, the thread pool, audit log and firewall are Enterprise features. On Community, `CREATE FUNCTION ... LANGUAGE JAVASCRIPT` and `CREATE LIBRARY` *succeed* with only `Warning 6001 Language component: Not available`, and the routine fails when called (`ERROR 6001`). A migration that "applies cleanly" in CI on Community proves nothing about such code.
- Distinguish **HeatWave** (Oracle's cloud service) from MySQL Server: `DISTANCE()`/vector search functions, `TABLESAMPLE` (needs a secondary engine: `ERROR 3889`), materialized views (`CREATE MATERIALIZED VIEW` parses but creates a plain view), Lakehouse external tables and GenAI routines are not available in Community or Enterprise Server 9.7.
- Managed services (Amazon RDS/Aurora, Azure, Google Cloud SQL) publish their own 9.x support and parameter defaults; check them instead of assuming the upstream defaults in this guide.
- MariaDB is not MySQL. `RETURNING`, `CREATE OR REPLACE TABLE`, `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, sequences and system-versioned tables are MariaDB features; on MySQL 9.7 they are syntax errors.

##### Do not use or recommend (features newer than 9.7)

The server does not have these. Never suggest them for code that runs on 9.7, and flag them as bugs when they appear:[^5][^21]

| Feature | Introduced | On 9.7.4 |
|---|---|---|
| Change Stream Applier: `CHANGE REPLICATION SOURCE TO APPLIER_VERSION = 2`, `APPLIER_WORKER_COUNT`, `APPLIER_EVENT_MEMORY_LIMIT` | 26.7.0 | not available |
| Post-quantum TLS: `force_pqc`, `admin_force_pqc`, `replication_force_pqc`, `tls_kex`, `admin_tls_kex`, `replication_tls_kex`, `use_pqc_sign`, ... | 26.7.0 | `ERROR 1193 Unknown system variable` |
| Thread Pool plugin in Community Edition | 26.7.0 | Enterprise only (`@@thread_pool_max_unused_threads` → `ERROR 1193` on Community 9.7.4) |
| `innodb_autoinc_preallocate`; `mysqldump --extended-insert-multiline` | 26.7.0 | not available |
| MySQL REST Service as a server component (`component_mysql_rest_service`, `mysql_rest_service.*` variables, `mysql_rest_service_start()`) | 26.10.0 | not available |
| Health Monitor component in Community | 26.10.0 | not available |
| `authentication_webauthn` server plugin in Community | 26.10.0 | Enterprise only |
| JSON duality view `WITH (CHECK / NO CHECK / UPDATE / NO UPDATE)` field annotations for ETAG participation and mutability | 26.10.0 | only table-level `INSERT`/`UPDATE`/`DELETE` tags (9.6.0) |
| Strict `optimizer_switch` value parsing (`hash_join=of` rejected) | 26.10.0 | `SET optimizer_switch='hash_join=of'` is accepted and means `off` |
| Partition names in iterator-based `EXPLAIN` / `EXPLAIN ANALYZE` | 26.10.0 | not shown |

Not available on 9.7.4 at all (verified): `INSERT ... RETURNING`, `IS [NOT] DISTINCT FROM` (use `<=>`), `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, `DROP INDEX IF EXISTS`, `FULL OUTER JOIN`, `CREATE SEQUENCE`, `WITH x AS MATERIALIZED (...)`, `REFRESH MATERIALIZED VIEW`, a UUIDv7 function (`UUID_V7()` → `ERROR 1305`), range access for row-constructor inequalities such as `(a, b) > (?, ?)`, `DISTANCE()`/`VECTOR_DISTANCE()`, indexes on `VECTOR` columns.

##### Deprecated in 9.7 (works, but flag it)

These still work on 9.7.4 (most print a warning) but will break on a later release:

- `FLUSH PRIVILEGES` (warning 1681 since 9.2.0), the `FLUSH_PRIVILEGES` privilege, `mysqladmin flush-privileges`/`reload`. `CREATE USER`, `GRANT`, `ALTER USER` never needed it.[^15]
- `enable_cascade_triggers`: introduced *and* deprecated in 9.7.0; every `SET` prints warning 1681 (and 4166 when it is reset).[^22]
- `MD5()` / `SHA1()` (deprecated in 9.4.0; only available through the `classic_hashing` component).[^9][^10]
- Transactions that update both transactional (InnoDB) and non-transactional (MyISAM) tables (warning 6414, since 9.0.0).[^8]
- `CREATE SPATIAL REFERENCE SYSTEM` under `SUPER` (use `CREATE_SPATIAL_REFERENCE_SYSTEM`).[^15]
- Plugins replaced by components: keyring plugins (`keyring_aws`, `keyring_hashicorp`, `keyring_okv`), connection-control plugins, firewall plugin, audit log plugin; `--early-plugin-load`.[^14][^15][^16][^9][^12]
- Carried over from 8.4 and still deprecated with warnings: `VALUES(col)` in `ON DUPLICATE KEY UPDATE` (1287), `SQL_CALC_FOUND_ROWS`/`FOUND_ROWS()` (1287), `&&`/`||`/`!` as logical operators (1287), user-variable assignment in expressions (1287), integer display width/`ZEROFILL`/`FLOAT(M,D)` (1681), `utf8`/`utf8mb3` (3719), `$` as the first character of an unquoted identifier (1681), `@@binlog_format` (1287), `INFORMATION_SCHEMA.PROCESSLIST` (1287), `restrict_fk_on_non_standard_key=OFF` (4166), wildcards in database names of `GRANT`, `DISABLE ON SLAVE`.
- `performance_schema.variables_info.MIN_VALUE`/`MAX_VALUE` (use `variables_metadata`, 9.0.0), SCRAM-SHA-1 for LDAP SASL (9.5.0; `authentication_ldap_sasl_auth_method_name` now defaults to `SCRAM-SHA-256`), `group_replication_communication_stack`, `group_replication_ip_allowlist` (9.7.2).[^8][^12][^7][^19]
- `innodb_native_foreign_keys` (the startup option that restores InnoDB-native foreign keys) is itself deprecated: starting with it logs `The syntax '--innodb_native_foreign_keys' is deprecated and will be removed in a future release` (tested on 9.7.4). Treat it as a temporary migration aid only.[^46]

***

#### Schema Design and Data Types

##### Primary keys

Every InnoDB table needs a primary key that is narrow, immutable and, ideally, monotonically increasing.

```sql
-- GOOD: compact, monotonic clustered key
CREATE TABLE orders (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id     BIGINT UNSIGNED NOT NULL,
  status      VARCHAR(16)     NOT NULL,
  total       DECIMAL(12,2)   NOT NULL,
  created_at  DATETIME(3)     NOT NULL,
  KEY ix_orders_user_created (user_id, created_at),
  KEY ix_orders_status_created (status, created_at)
);
```

- **BAD**: `CHAR(36)` UUID primary keys. 36 characters (up to 144 bytes of index space in `utf8mb4`) repeated in every secondary index, random insert positions, page splits.
- **GOOD**: if UUIDs are required, store them as `BINARY(16)`. For time-based version-1 UUIDs from `UUID()`, `UUID_TO_BIN(u, 1)` swaps the time fields so values become roughly monotonic. MySQL 9.7 has no built-in UUIDv7 generator; generate v7 in the application if you want time-ordered random IDs.

```sql
SELECT HEX(UUID_TO_BIN('6ccd780c-baba-1026-9564-5b8c656024db'))    AS plain,    -- 6CCD780CBABA102695645B8C656024DB
       HEX(UUID_TO_BIN('6ccd780c-baba-1026-9564-5b8c656024db', 1)) AS swapped;  -- 1026BABA6CCD780C95645B8C656024DB
SELECT SUBSTRING(UUID(), 15, 1) AS uuid_version;                              -- 1 (UUID() is version 1)

CREATE TABLE exd (
  id   BINARY(16) PRIMARY KEY DEFAULT (UUID_TO_BIN(UUID(), 1)),
  meta JSON DEFAULT (JSON_OBJECT()),
  body TEXT DEFAULT (''),
  created DATE DEFAULT (CURRENT_DATE)
);
INSERT INTO exd () VALUES ();
SELECT BIN_TO_UUID(id, 1), meta, body, created FROM exd;
```

Expression defaults (`DEFAULT (expr)`, with parentheses) work for `BLOB`/`TEXT`/`JSON` columns too; a literal default on those types does not.

A table without a primary key gets a hidden 6-byte row ID, cannot be replicated efficiently with row-based replication and is rejected by many managed services. Two server settings help, and **9.7.0 changed both** to treat a `UNIQUE` key over `NOT NULL` columns as a primary-key equivalent:[^1]

```sql
SET SESSION sql_require_primary_key = ON;
CREATE TABLE uq (a INT NOT NULL, UNIQUE KEY (a));   -- OK on 9.7 (8.4: ERROR 3750)
CREATE TABLE uq2 (a INT, UNIQUE KEY (a));           -- ERROR 3750 (HY000): Unable to create or change a table
                                                    --   without a primary key ... (nullable column)
SET SESSION sql_require_primary_key = OFF;

SET SESSION sql_generate_invisible_primary_key = ON;
CREATE TABLE gi (a INT NOT NULL, UNIQUE KEY (a));   -- 9.7: no my_row_id added (8.4: adds my_row_id)
CREATE TABLE gi2 (a INT);                           -- gets `my_row_id` bigint unsigned NOT NULL AUTO_INCREMENT INVISIBLE
SELECT * FROM gi2;                                  -- my_row_id is not returned by SELECT *
SET SESSION sql_generate_invisible_primary_key = OFF;
```

Flag migrations that create tables without a primary key. A `UNIQUE NOT NULL` key now satisfies the server, but an explicit `PRIMARY KEY` is still clearer and is what tools such as `gh-ost`, managed-service checks and many ORMs look for.

##### Integers, DECIMAL, FLOAT

- Money and anything compared for equality: `DECIMAL(p,s)`. Never `FLOAT`/`DOUBLE`.
- `FLOAT` is single precision: about 7 significant digits.
- `UNSIGNED` subtraction that goes negative is an error, not a wrap-around.
- An `AUTO_INCREMENT` column that reaches its type's maximum fails with a duplicate-key error on the next insert. Use `BIGINT UNSIGNED` for anything that grows.

```sql
CREATE TABLE money (f FLOAT, d DOUBLE, dec2 DECIMAL(12,2));
INSERT INTO money VALUES (0.1, 0.1, 0.1), (0.2, 0.2, 0.2);
SELECT SUM(f), SUM(d), SUM(dec2), SUM(d) = 0.3, SUM(dec2) = 0.3 FROM money;
-- 0.30000000447034836 | 0.30000000000000004 | 0.30 | 0 | 1

INSERT INTO money (f) VALUES (123456789);
SELECT f FROM money WHERE dec2 IS NULL;          -- 123457000

SELECT CAST(1 AS UNSIGNED) - 2;
-- ERROR 1690 (22003): BIGINT UNSIGNED value is out of range in '(cast(1 as unsigned) - 2)'

CREATE TABLE ai (id TINYINT UNSIGNED AUTO_INCREMENT PRIMARY KEY);
INSERT INTO ai VALUES (254);
INSERT INTO ai VALUES ();     -- 255
INSERT INTO ai VALUES ();     -- ERROR 1062 (23000): Duplicate entry '255' for key 'ai.PRIMARY'
```

Deprecated attributes still accepted with warning 1681 — remove them in new DDL:

```sql
CREATE TABLE w (id INT(11), b TINYINT(1), z INT ZEROFILL, f FLOAT(7,2));
-- Warning 1681: Integer display width is deprecated ... (twice)
-- Warning 1681: The ZEROFILL attribute is deprecated ...
-- Warning 1681: Specifying number of digits for floating point data types is deprecated ...
```

`TINYINT(1)` is the one display width that still has meaning for connectors (it is how `BOOLEAN`/`BOOL` is stored and how drivers detect booleans); MySQL has no real boolean type.

##### Strings, ENUM, JSON

- `VARCHAR(n)` counts characters; the byte limit matters for indexes (see Indexing).
- `ENUM` sorts by declaration index, not alphabetically, and compares to integers by index. Adding a value at the end is metadata-only; reordering or inserting in the middle rebuilds the table. A lookup table or a `VARCHAR` with a `CHECK` constraint is often easier to evolve.

```sql
CREATE TABLE en (s ENUM('small','large','medium'));
INSERT INTO en VALUES ('small'), ('large'), ('medium');
SELECT s FROM en ORDER BY s;        -- small, large, medium  (declaration order)
SELECT s FROM en WHERE s = 2;       -- large                 (index 2)
INSERT INTO en VALUES ('huge');     -- ERROR 1265 (01000): Data truncated for column 's' at row 1
```

- `JSON` is stored in a binary format, validated on insert and partially updatable with `JSON_SET`/`JSON_REPLACE`/`JSON_REMOVE`. Do not use `JSON` to avoid modelling columns that are filtered, joined or constrained; index JSON paths with functional or multi-valued indexes when you must (see Indexing).

##### VECTOR [New in 9.7]

`VECTOR(n)` (9.0.0) stores up to `n` 4-byte floats (default 2048, maximum 16383). It is a storage type only in MySQL Server: there is no vector index, no distance function and no comparison other than equality.[^8][^23]

```sql
CREATE TABLE emb (id INT PRIMARY KEY, v VECTOR(3));
INSERT INTO emb VALUES (1, STRING_TO_VECTOR('[1,2,3]')), (2, TO_VECTOR('[4, 5, 6] '));
SELECT id, VECTOR_TO_STRING(v), VECTOR_DIM(v), LENGTH(v) FROM emb;
-- 1 | [1.00000e+00,2.00000e+00,3.00000e+00] | 3 | 12

INSERT INTO emb VALUES (3, STRING_TO_VECTOR('[1,2]'));   -- accepted: n is a MAXIMUM, not an exact dimension
INSERT INTO emb VALUES (4, '[1,2,3]');
-- ERROR 6136 (HY000): Value of type 'string, size: 7' cannot be converted to 'vector' type.
SELECT * FROM emb WHERE v < STRING_TO_VECTOR('[1,2,3]');  -- ERROR 1210 (HY000): Incorrect arguments to <
CREATE TABLE emb2 (v VECTOR(3), KEY (v));
-- ERROR 6134 (HY000): Non-scalar (e.g., vector) column 'v' cannot be used as key.
SELECT DISTANCE(v, STRING_TO_VECTOR('[1,2,3]'), 'COSINE') FROM emb;
-- ERROR 1305 (42000): FUNCTION t.DISTANCE does not exist   (HeatWave only)
CREATE TABLE emb4 (v VECTOR(16384));
-- ERROR 6137 (HY000): Data size (65536 Bytes, 16384 dimensions) exceeds VECTOR max (65532 Bytes, 16383 dimensions) ...
```

Review rules:

- **BAD**: assuming `VECTOR(1536)` guarantees 1536 dimensions. A truncated embedding is stored silently. `VECTOR` values are not accepted in `CHECK` constraints or generated columns (`CHECK (VECTOR_DIM(v) = 1536)` → `ERROR 3064 Incorrect type for argument v in function CHECK CONSTRAINT`), so validate in the application or with a trigger:

```sql
CREATE TABLE ev (id INT PRIMARY KEY, v VECTOR(4));
DELIMITER //
CREATE TRIGGER ev_bi BEFORE INSERT ON ev FOR EACH ROW
BEGIN
  IF VECTOR_DIM(NEW.v) <> 4 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'embedding must have 4 dimensions';
  END IF;
END//
DELIMITER ;
INSERT INTO ev VALUES (1, TO_VECTOR('[1,2]'));       -- ERROR 1644 (45000): embedding must have 4 dimensions
INSERT INTO ev VALUES (2, TO_VECTOR('[1,2,3,4]'));   -- OK
```

  (Add a matching `BEFORE UPDATE` trigger as well.)
- **BAD**: similarity search written for HeatWave (`DISTANCE()`, `VECTOR INDEX`) in code that runs on MySQL Server. Similarity ranking must happen in the application or in a dedicated vector store; a full scan computing distances in a stored function does not scale.
- `ORDER BY v` "works" but orders by the binary representation, which is meaningless.
- `VECTOR` is not accepted by `CAST(... AS CHAR)`, JSON functions or numeric operators; convert explicitly with `VECTOR_TO_STRING()`.

##### Constraints: NOT NULL, CHECK, defaults

`CHECK` constraints are enforced. A `CHECK` that evaluates to `NULL` passes (add `NOT NULL` separately), `INSERT IGNORE` turns a violation into a warning and silently skips the row, and nondeterministic functions are rejected:

```sql
CREATE TABLE pr (
  id INT PRIMARY KEY,
  price DECIMAL(10,2) NOT NULL,
  discount DECIMAL(10,2) NOT NULL DEFAULT 0,
  CONSTRAINT chk_price CHECK (price >= 0),
  CONSTRAINT chk_disc  CHECK (discount <= price)
);
INSERT INTO pr VALUES (2, 5, 6);              -- ERROR 3819 (HY000): Check constraint 'chk_disc' is violated.
INSERT IGNORE INTO pr VALUES (4, -1, 0);      -- Warning 3819; row skipped, statement "succeeds"
CREATE TABLE chk3 (a INT, CHECK (a > 0));
INSERT INTO chk3 VALUES (NULL);               -- accepted: NULL > 0 is UNKNOWN, which passes
CREATE TABLE ck (a INT CHECK (a > 0), b INT, CHECK (b > RAND()));
-- ERROR 3814 (HY000): An expression of a check constraint 'ck_chk_2' contains disallowed function: rand.

CREATE TABLE ck2 (a INT, CONSTRAINT c CHECK (a > 0) NOT ENFORCED);
INSERT INTO ck2 VALUES (-1);                  -- accepted: NOT ENFORCED is documentation only
```

`CHECK (JSON_SCHEMA_VALID(...))` validates JSON documents on write:

```sql
CREATE TABLE cj (id INT PRIMARY KEY, doc JSON,
  CHECK (JSON_SCHEMA_VALID('{"type":"object","required":["name"]}', doc)));
INSERT INTO cj VALUES (1, '{"x":1}');      -- ERROR 3819 (HY000): Check constraint 'cj_chk_1' is violated.
INSERT INTO cj VALUES (2, '{"name":"a"}'); -- OK
```

Since 9.7.4, `ALTER TABLE ... EXCHANGE PARTITION` checks the swapped rows against the destination's `CHECK` constraints even with `WITHOUT VALIDATION`.[^20]

##### Foreign keys [New in 9.7]

Three things changed since 8.4: inline `REFERENCES` is enforced (9.0.0), foreign keys are implemented in the SQL layer (9.6.0), and cascades can optionally fire triggers (9.7.0).[^8][^10][^1]

1. **Inline `REFERENCES` creates a constraint.** All of these forms now create an enforced foreign key named `<table>_ibfk_<n>`:

```sql
CREATE TABLE pet  (id INT PRIMARY KEY, owner INT REFERENCES person (id));
CREATE TABLE pet2 (id INT PRIMARY KEY, owner INT, FOREIGN KEY (owner) REFERENCES person);       -- implicit PK
CREATE TABLE pet3 (id INT PRIMARY KEY, owner INT REFERENCES person ON DELETE CASCADE);
SELECT table_name, constraint_name, delete_rule FROM information_schema.referential_constraints
WHERE constraint_schema = DATABASE() ORDER BY 1, 2;
-- pet | pet_ibfk_1 | NO ACTION ;  pet2 | pet2_ibfk_1 | NO ACTION ;  pet3 | pet3_ibfk_1 | CASCADE

CREATE TABLE pet4 (id INT PRIMARY KEY, owner INT CONSTRAINT fk_named REFERENCES person (id));
-- ERROR 1064: a name can only be given with the table-level form
```

   Review consequences: DDL written for 8.4 that "worked" with inline `REFERENCES` can now fail on insert (`ERROR 1452`) or on parent delete (`ERROR 1451`) because orphans that used to be accepted are now rejected; tables created on 8.4 do not gain the constraint during an in-place upgrade (tested), and dumps from 8.4 do not contain it, so a schema built from old migration files on 9.7 differs from an upgraded or restored one. On 8.4, the table-level form `FOREIGN KEY (owner) REFERENCES person` (no column list) is `ERROR 1239`; on 9.7 it references the primary key. Prefer the explicit, named table-level form `CONSTRAINT fk_x FOREIGN KEY (col) REFERENCES parent (id)` so names are stable and the intent is visible.

2. **Adding a column with inline `REFERENCES` is no longer an instant change:**

```sql
ALTER TABLE dd ADD COLUMN owner INT REFERENCES person (id), ALGORITHM=INSTANT;
-- ERROR 1846 (0A000): ALGORITHM=INSTANT is not supported. Reason: Adding foreign keys needs
--   foreign_key_checks=OFF. Try ALGORITHM=COPY/INPLACE.     (8.4.13: OK, and no FK is created)
ALTER TABLE dd ADD COLUMN owner INT REFERENCES person (id);
-- Query OK, 1 row affected: the table was COPIED (Records: n), blocking writes for the duration
```

3. **The referenced columns must form a `PRIMARY KEY` or `UNIQUE` key** (`restrict_fk_on_non_standard_key=ON`, unchanged from 8.4). Referencing a table without a primary key via the implicit form fails: `ERROR 1239 Incorrect foreign key definition ... Key reference and table reference don't match`.
4. **Cascaded changes do not fire triggers by default.** `ON DELETE CASCADE` / `ON UPDATE CASCADE` / `SET NULL` change child rows without running the child table's triggers. 9.7.0 adds the session variable `enable_cascade_triggers` (default `OFF`), but it is already deprecated: every `SET` raises warning 1681.[^1][^22] Do not build audit logic on it; audit at the parent or in the application.

```sql
CREATE TABLE audit (id INT AUTO_INCREMENT PRIMARY KEY, what VARCHAR(50));
CREATE TABLE p2 (id INT PRIMARY KEY);
CREATE TABLE c2 (id INT PRIMARY KEY, p INT,
                 FOREIGN KEY (p) REFERENCES p2 (id) ON DELETE CASCADE);
CREATE TRIGGER c2_ad AFTER DELETE ON c2 FOR EACH ROW
  INSERT INTO audit (what) VALUES (CONCAT('deleted child ', OLD.id));
INSERT INTO p2 VALUES (1), (2);
INSERT INTO c2 VALUES (10, 1), (11, 1), (20, 2);
DELETE FROM p2 WHERE id = 1;
SELECT (SELECT COUNT(*) FROM c2) AS children, (SELECT COUNT(*) FROM audit) AS audit_rows;   -- 1 | 0

SET SESSION enable_cascade_triggers = ON;
-- Warning 1681: 'enable_cascade_triggers' is deprecated and will be removed in a future release.
DELETE FROM p2 WHERE id = 2;
SELECT (SELECT COUNT(*) FROM c2) AS children, (SELECT COUNT(*) FROM audit) AS audit_rows;   -- 0 | 1
```

5. **SQL-layer foreign keys** (9.6.0): constraint checks and cascades are executed by the server instead of InnoDB, so cascaded child-row changes are written to the binary log (tested: a cascading `DELETE` logs `Delete_rows` events for the child table with the flag `USE_SQL_FOREIGN_KEY_F`; with `--innodb-native-foreign-keys=ON`, as on 8.4, only the parent row is logged). `innodb_native_foreign_keys` is a read-only, already deprecated startup option (default `OFF`) that restores the InnoDB implementation; `SET GLOBAL innodb_native_foreign_keys = ON` fails with `ERROR 1238`.[^10] Other documented differences of the SQL layer:[^46] a child insert rejected with `ERROR 1452` no longer consumes an `AUTO_INCREMENT` value (tested), a cascade may traverse at most 15 levels (`ER_FK_DEPTH_EXCEEDED`), and when one statement violates several constraints a different constraint may be named in the error than on 8.4 — do not match error texts on constraint names.
6. **`REPLACE` on a parent table** cascades (or fails) only when the replaced row's referenced key value changes; with an unchanged key, 9.7 keeps the children and skips the `RESTRICT`/`NO ACTION` check (8.4 cascaded or failed). See Writes.
7. **`SET foreign_key_checks = 0` lets orphans in.** Adding a foreign key with checks disabled does not validate existing rows (that is why `ALGORITHM=INPLACE` is only allowed then). Run an orphan query before re-enabling.
8. Foreign keys are not supported on partitioned InnoDB tables, and the referencing columns get an index automatically if none exists.
9. Column types must be compatible: integer types need the same size and signedness, string columns the same character set and collation (lengths may differ). Otherwise: `ERROR 3780 Referencing column ... and referenced column ... are incompatible`.

##### Generated and invisible columns

```sql
CREATE TABLE ic (id INT PRIMARY KEY, secret VARCHAR(10) INVISIBLE);
INSERT INTO ic VALUES (1);                       -- invisible columns are skipped by positional INSERT
INSERT INTO ic (id, secret) VALUES (2, 's');
SELECT * FROM ic;                                -- returns only id
SELECT id, secret FROM ic;                       -- explicit reference works
```

Invisible columns let you add a column without breaking `SELECT *` or `INSERT ... VALUES` without a column list in old code. Generated columns are `VIRTUAL` (computed on read, can be indexed, added `INSTANT`ly) or `STORED` (materialized; adding one rebuilds the table with `ALGORITHM=COPY`).

***

#### Character Sets and Collations

##### Defaults and the `utf8` trap

```sql
SELECT @@character_set_server, @@collation_server;     -- utf8mb4 | utf8mb4_0900_ai_ci

CREATE TABLE m3 (s VARCHAR(10) CHARACTER SET utf8);
-- Warning 3719: 'utf8' is currently an alias for the character set UTF8MB3, but will be an alias
--   for UTF8MB4 in a future release. Please consider using UTF8MB4 in order to be unambiguous.
SHOW CREATE TABLE m3;       -- `s` varchar(10) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci

INSERT INTO m3 VALUES ('😀');
-- ERROR 1366 (HY000): Incorrect string value: '\xF0\x9F\x98\x80' for column 's' at row 1
SET SESSION sql_mode = '';
INSERT INTO m3 VALUES ('a😀b');      -- Warning 1366; stored as 'a?b'  (silent data loss)
SET SESSION sql_mode = DEFAULT;
```

**New in 9.5**: the `INTERPRET_UTF8_AS_UTF8MB4` SQL mode (off by default) makes `utf8` and `utf8_*` collation names in statements mean `utf8mb4`:[^12]

```sql
SET SESSION sql_mode = CONCAT(@@sql_mode, ',INTERPRET_UTF8_AS_UTF8MB4');
CREATE TABLE u8 (s VARCHAR(10) CHARACTER SET utf8);
SELECT character_set_name, collation_name FROM information_schema.columns WHERE table_name = 'u8';
-- utf8mb4 | utf8mb4_0900_ai_ci     (without the mode: utf8mb3 | utf8mb3_general_ci, warning 3719)
SET SESSION sql_mode = DEFAULT;
```

- Flag `utf8`, `utf8mb3`, `utf8_general_ci`, `utf8_unicode_ci` in new DDL; write `utf8mb4` explicitly. Do not rely on `INTERPRET_UTF8_AS_UTF8MB4` to fix old DDL: the result depends on the session's `sql_mode`, so the same migration creates different columns on different connections, and the mode does not exist on 8.4 (`ERROR 1231`).
- Flag connections that do not set the character set. The `mysql` command-line client picks its character set from the OS locale; in a container with no locale it connects as `latin1`, and non-ASCII literals in scripts are then misinterpreted. Use `--default-character-set=utf8mb4` (or `SET NAMES utf8mb4`) in migration runners; drivers should set `charset=utf8mb4` / `collation=utf8mb4_0900_ai_ci`.

##### `utf8mb4_0900_ai_ci` semantics

The default collation is case-insensitive, accent-insensitive and **NO PAD** (trailing spaces are significant). Older collations (`utf8mb4_general_ci`, `utf8mb4_unicode_ci`, `utf8mb4_bin`) are `PAD SPACE`.[^24]

```sql
SET NAMES utf8mb4;
SELECT 'a' = 'A', 'e' = 'é', 'a' = 'a ', 'ß' = 'ss';      -- 1 | 1 | 0 | 1
SELECT 'a' = 'a ' COLLATE utf8mb4_general_ci AS general_pad,
       'a' = 'a ' COLLATE utf8mb4_unicode_ci AS unicode_pad,
       'a' = 'a ' COLLATE utf8mb4_0900_ai_ci AS nopad;     -- 1 | 1 | 0
SELECT 'ß' = 'ss' COLLATE utf8mb4_general_ci AS g,
       'ß' = 'ss' COLLATE utf8mb4_unicode_ci AS u,
       'ß' = 'ss' COLLATE utf8mb4_0900_ai_ci AS n;         -- 0 | 1 | 1
SELECT 'a' = 'A' COLLATE utf8mb4_0900_as_cs AS cs,
       'a' = 'A' COLLATE utf8mb4_bin AS b,
       'e' = 'é' COLLATE utf8mb4_0900_as_ci AS asci;       -- 0 | 0 | 0
```

Consequences a reviewer must catch:

- **Unique keys are collation-aware.** With the default collation, `'Muller'`, `'müller'` and `'MÜLLER'` collide; `'Muller'` and `'Muller '` do not.

```sql
CREATE TABLE u (name VARCHAR(50) PRIMARY KEY);
INSERT INTO u VALUES ('Muller');
INSERT INTO u VALUES ('müller');    -- ERROR 1062 (23000): Duplicate entry 'müller' for key 'u.PRIMARY'
INSERT INTO u VALUES ('Muller ');   -- OK: NO PAD, trailing space is significant
```

  Usernames, slugs, tokens, hashes and other identifiers that must be distinct byte-for-byte need `COLLATE utf8mb4_bin` (or `utf8mb4_0900_bin`) or a `VARBINARY` column.
- **`LOWER(col) = ...` is redundant** with a `_ci` collation and prevents index use (see Indexing).
- **Mixing collations** in a comparison or join raises an error, and a join between columns with different collations cannot use the index on the converted side:

```sql
CREATE TABLE ca (s VARCHAR(10) COLLATE utf8mb4_unicode_ci);
CREATE TABLE cb (s VARCHAR(10) COLLATE utf8mb4_0900_ai_ci);
SELECT * FROM ca JOIN cb ON ca.s = cb.s;
-- ERROR 1267 (HY000): Illegal mix of collations (utf8mb4_unicode_ci,IMPLICIT) and
--   (utf8mb4_0900_ai_ci,IMPLICIT) for operation '='
```

  Specify collations explicitly at database or table level in migrations so new tables match old ones.
- 9.4.0 changed collation derivation so that an expression with derivation `NONE` (for example `IFNULL()` of mixed collations) yields to the other operand instead of raising `ER_CANT_AGGREGATE_2COLLATIONS` in `LIKE`; `COERCIBILITY()` values shifted accordingly.[^9]
- `ALTER TABLE ... CONVERT TO CHARACTER SET` rewrites the whole table (`ALGORITHM=COPY`) and may change index lengths and uniqueness; review it as a heavy migration.

***

#### SQL Mode and Strictness

The server's default `sql_mode` is strict:[^25]

```sql
SELECT @@sql_mode;
-- ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION
```

With the default mode, bad data is rejected:

```sql
CREATE TABLE s1 (id INT PRIMARY KEY, code VARCHAR(5) NOT NULL, qty TINYINT UNSIGNED, d DATE);
INSERT INTO s1 VALUES (1, 'toolong', 1, '2024-01-01'); -- ERROR 1406 (22001): Data too long for column 'code'
INSERT INTO s1 VALUES (2, 'ok', 300, '2024-01-01');    -- ERROR 1264 (22003): Out of range value for column 'qty'
INSERT INTO s1 VALUES (3, 'ok', 1, '2024-02-30');      -- ERROR 1292 (22007): Incorrect date value: '2024-02-30'
INSERT INTO s1 VALUES (4, 'ok', 1, '0000-00-00');      -- ERROR 1292 (22007): Incorrect date value: '0000-00-00'
INSERT INTO s1 (id, qty) VALUES (5, 1);                -- ERROR 1364 (HY000): Field 'code' doesn't have a default value
INSERT INTO s1 VALUES (6, 'ok', 1/0, '2024-01-01');    -- ERROR 1365 (22012): Division by 0
```

`IGNORE` and a relaxed session `sql_mode` bring back silent truncation — even storing a zero date despite `NO_ZERO_DATE`:

```sql
INSERT IGNORE INTO s1 VALUES (7, 'toolong', 300, '2024-02-30');
-- Warning 1265 Data truncated for column 'code'; Warning 1264 Out of range ... 'qty'; Warning 1264 ... 'd'
SELECT * FROM s1;                              -- 7 | toolo | 255 | 0000-00-00

SET SESSION sql_mode = '';
INSERT INTO s1 (id, qty) VALUES (8, -5);       -- Warning 1264, Warning 1364
SELECT * FROM s1 WHERE id = 8;                 -- 8 |  | 0 | NULL
SET SESSION sql_mode = DEFAULT;
```

Review rules:

- **BAD**: application code or ORMs that set `sql_mode=''` or drop `STRICT_TRANS_TABLES`/`ONLY_FULL_GROUP_BY` to make legacy queries pass. Fix the query.
- **BAD**: `INSERT IGNORE` as an "upsert" or "skip duplicates" shortcut. It also hides truncation, out-of-range values, invalid dates, `CHECK` violations and **foreign-key violations** (see Writes). Use `INSERT ... ON DUPLICATE KEY UPDATE id = id` or catch error 1062 when only duplicates should be skipped.
- **New in 9.0**: `IGNORE` no longer suppresses "Subquery returns more than 1 row"; the statement fails:[^8]

```sql
CREATE TABLE ig2 (id INT PRIMARY KEY, v INT NOT NULL);
INSERT IGNORE INTO ig2 VALUES (1, (SELECT y FROM sales));
-- ERROR 1242 (21000): Subquery returns more than 1 row   (8.4: warning, row inserted with v = 0)
```

- **New in 9.5**: invalid temporal values sent *with a temporal type* through the binary (prepared-statement) protocol are rejected even in a non-strict `sql_mode`; they used to be adjusted silently. Values bound as strings still follow `sql_mode` (see Application Code).[^12]
- Strict mode applies to values stored in columns; it does not validate values that never reach a column (e.g. `SELECT` expressions):

```sql
SELECT '1abc' + 0, '1abc' = 1, 'abc' = 0;      -- 1 | 1 | 1   (warnings 1292 only)
SELECT STR_TO_DATE('31/02/2024', '%d/%m/%Y');  -- NULL, Warning 1411
```

##### ONLY_FULL_GROUP_BY

```sql
CREATE TABLE o (id INT PRIMARY KEY, cust INT, total DECIMAL(10,2), status VARCHAR(10));
INSERT INTO o VALUES (1, 1, 10, 'new'), (2, 1, 20, 'paid'), (3, 2, 5, 'new');

SELECT cust, status, SUM(total) FROM o GROUP BY cust;
-- ERROR 1055 (42000): Expression #2 of SELECT list is not in GROUP BY clause and contains
--   nonaggregated column 't.o.status' which is not functionally dependent on columns in GROUP BY clause

SELECT cust, ANY_VALUE(status), SUM(total) FROM o GROUP BY cust;   -- explicit "any row" (nondeterministic)
SELECT o.id, o.cust, o.status FROM o GROUP BY o.id;                -- OK: functionally dependent on the PK
```

`ANY_VALUE()` is acceptable only when any value is correct. To get the row with the max/min per group, use a window function or a join (see Query patterns). `GROUP BY` does not sort; add `ORDER BY` when order matters.

##### Operators that differ from standard SQL

```sql
SELECT 'a' || 'b';                     -- 0: || is logical OR (deprecated, warning 1287), not concatenation
SELECT CONCAT('a', 'b', NULL);         -- NULL: any NULL argument makes CONCAT NULL
SELECT CONCAT_WS('-', 'a', NULL, 'b'); -- a-b: CONCAT_WS skips NULLs
```

Use `CONCAT()`. Do not enable `PIPES_AS_CONCAT` or `ANSI_QUOTES` in one session only; queries then behave differently depending on which connection runs them.

***

#### Date, Time and Time Zones

##### DATETIME vs TIMESTAMP

| | `DATETIME(fsp)` | `TIMESTAMP(fsp)` |
|---|---|---|
| Range | `1000-01-01` – `9999-12-31` | `1970-01-01 00:00:01` UTC – `2038-01-19 03:14:07` UTC |
| Time zone | Stored as written; no conversion | Converted from session `time_zone` to UTC on write and back on read |
| Storage | 5 bytes + fsp | 4 bytes + fsp |

```sql
CREATE TABLE tt (id INT PRIMARY KEY, ts TIMESTAMP NULL, dt DATETIME NULL, ts6 TIMESTAMP(6) NULL);
SET time_zone = '+00:00';
INSERT INTO tt VALUES (1, '2024-06-01 12:00:00', '2024-06-01 12:00:00', NULL);
SET time_zone = '+02:00';
SELECT ts, dt FROM tt;               -- 2024-06-01 14:00:00 | 2024-06-01 12:00:00

SET time_zone = '+00:00';
INSERT INTO tt VALUES (3, '2038-01-19 03:14:08', '2038-01-19 03:14:08', NULL);
-- ERROR 1292 (22007): Incorrect datetime value: '2038-01-19 03:14:08' for column 'ts' at row 1
```

- **BAD**: new `TIMESTAMP` columns for business dates or anything that may exceed January 2038 (expiry dates, contracts) — `TIMESTAMP` also cannot hold dates before 1970.
- **GOOD**: `DATETIME(3)` or `DATETIME(6)` storing UTC, written by the application or with `UTC_TIMESTAMP(3)`, and a connection `time_zone='+00:00'`.
- `TIMESTAMP` remains appropriate for "row changed at" audit columns where automatic UTC conversion is wanted, but every connection must agree on `time_zone`, or reads differ by connection.
- With `explicit_defaults_for_timestamp=ON` (default), a `TIMESTAMP` column has no implicit `DEFAULT CURRENT_TIMESTAMP` or `ON UPDATE`; write them explicitly:

```sql
CREATE TABLE ts2 (
  id INT PRIMARY KEY,
  v INT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  plain      TIMESTAMP
);
INSERT INTO ts2 (id, v) VALUES (1, 1);
SELECT plain FROM ts2;           -- NULL  (no automatic initialization)
```

  `DATETIME` columns accept the same clauses (with matching precision, e.g. `DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3)`).

##### Fractional seconds round by default

```sql
INSERT INTO tt VALUES (6, NULL, '2024-06-01 12:00:00.999', '2024-06-01 12:00:00.1234567');
SELECT dt, ts6 FROM tt WHERE id = 6;   -- 2024-06-01 12:00:01 | 2024-06-01 12:00:00.123457

SET sql_mode = CONCAT(@@sql_mode, ',TIME_TRUNCATE_FRACTIONAL');
INSERT INTO tt VALUES (7, NULL, '2024-06-01 12:00:00.999', NULL);
SELECT dt FROM tt WHERE id = 7;        -- 2024-06-01 12:00:00
SET sql_mode = DEFAULT;
```

Writing a millisecond value into a `DATETIME` (precision 0) column can move it into the next second — or the next day at `23:59:59.5`. Declare the precision you need (`DATETIME(3)`) and make application and column precision match.

##### Literals with offsets, `AT TIME ZONE`, named zones

```sql
SET time_zone = '+00:00';
INSERT INTO tt VALUES (5, NULL, '2024-06-01 12:00:00+05:00', NULL);   -- offset literal
SELECT dt FROM tt WHERE id = 5;       -- 2024-06-01 07:00:00 (converted to the session time zone)

SET time_zone = '+02:00';
SELECT CAST(ts AT TIME ZONE '+00:00' AS DATETIME) AS utc, ts FROM tt WHERE id = 1;
-- 2024-06-01 12:00:00 | 2024-06-01 14:00:00
```

`CAST(... AT TIME ZONE ...)` only accepts a `TIMESTAMP` value and the zone `'+00:00'`/`'UTC'`. Named zones (`'Europe/Berlin'`) in `CONVERT_TZ()` and `SET time_zone` need the time-zone tables to be loaded[^26] (`mysql_tzinfo_to_sql`); otherwise `CONVERT_TZ()` returns `NULL` without a warning, and `SET time_zone = 'Europe/Berlin'` fails with `ERROR 1298 Unknown or incorrect time zone`. The official Docker image ships them loaded; many installations do not. Check `SELECT COUNT(*) FROM mysql.time_zone_name` when code relies on named zones.

```sql
SELECT CONVERT_TZ('2024-03-31 01:30:00', 'UTC', 'Europe/Berlin');   -- 2024-03-31 03:30:00 (tables loaded)
SELECT '2024-01-31' + INTERVAL 1 MONTH;                              -- 2024-02-29 (clamped to month end)
SELECT UNIX_TIMESTAMP('2038-01-19 03:14:08'), FROM_UNIXTIME(2147483648);   -- 64-bit range works
```

- `NOW()` is constant for the statement; `SYSDATE()` is evaluated per call and is unsafe for statement-based replication. Use `NOW()`.
- Do not filter with `DATE(col) = ?` — use a half-open range (see Indexing).

##### Temporal function fixes in 9.7.0 [New in 9.7]

9.7.0 (WL #16895) fixed a batch of temporal edge cases; queries that depended on the old results change behaviour after the upgrade:[^1]

```sql
-- MySQL 9.7.4 (8.4.13 in the comments where it differs)
SELECT TIMEDIFF('2024-01-02 00:00:00', DATE'2024-01-01');  -- 24:00:00     (8.4: NULL)
SELECT FROM_DAYS(-1);                                      -- NULL, Warning 6527 DATE value is out of range  (8.4: 0000-00-00)
SELECT DAYNAME('2024-05-07') + 0;                          -- 0, Warning 1292 'Tuesday'  (8.4: 1, the weekday number)
CREATE TABLE dt2i (d DATE, i BIGINT);
INSERT INTO dt2i (d) VALUES ('2024-05-06');
UPDATE dt2i SET i = d;
SELECT i FROM dt2i;                                        -- 20240506  (8.4: 20240506000000)
```

`TIMEDIFF(DATETIME, DATE)`, `ADDTIME`/`SUBTIME`/`ADDDATE`/`SUBDATE` with invalid arguments, `FROM_DAYS()` range handling, `DAYNAME()` in arithmetic, `CAST()` of `YEAR` columns and `DATE` values assigned to integer columns (no longer padded with `000000`) are the affected areas. Flag code that used these functions on doubtful input; validate input in the application instead of relying on a particular garbage-in result.

***

#### Indexing

##### What InnoDB indexes can and cannot do

MySQL 9.7 InnoDB has B-tree indexes (`PRIMARY`, `UNIQUE`, secondary), `FULLTEXT` and `SPATIAL` (R-tree) indexes, plus functional and multi-valued B-tree indexes.[^27] It has **no** partial (`WHERE`) indexes, no `INCLUDE` columns, no GIN/GiST/BRIN, no vector indexes, no hash indexes on InnoDB (`USING HASH` is accepted with Note 3502 and a B-tree is built instead), no `CREATE INDEX CONCURRENTLY` and no `CREATE INDEX IF NOT EXISTS`. Advice written for PostgreSQL does not translate; the MySQL equivalents are:

| PostgreSQL idea | MySQL 9.7 equivalent |
|---|---|
| Partial index `WHERE status = 'active'` | Composite index with the filter column first, or a generated column that is `NULL` for rows outside the subset |
| `INCLUDE (a, b)` covering index | Append the columns to the index key; every secondary index already contains the primary key |
| `CREATE INDEX CONCURRENTLY` | `ALTER TABLE ... ADD INDEX ..., ALGORITHM=INPLACE, LOCK=NONE` |
| Expression index `ON (lower(email))` | Usually unnecessary (`_ci` collation); otherwise functional index `((expr))` |
| GIN on JSONB array | Multi-valued index `((CAST(doc->'$.tags' AS CHAR(n) ARRAY)))` |
| `pg_trgm` for `LIKE '%x%'` | `FULLTEXT` index with `MATCH ... AGAINST` (word-based, not substring), or an external search engine |
| pgvector HNSW/IVFFlat index | None in MySQL Server; `VECTOR` columns cannot be indexed |

##### Leftmost prefix, ranges, covering indexes

A composite index `(a, b, c)` serves predicates on `a`, `a,b`, `a,b,c`; an equality on the leading columns followed by one range column is ideal. Columns after the first range column are not used for seeking (they can still be used for index condition pushdown). Every secondary index implicitly ends with the primary-key columns.

The examples below use `users` (100,000 rows) and `orders` (200,000 rows); plans are shown in the 9.7 default `TREE` format with the default (non-hypergraph) optimizer and cost figures removed:

```sql
CREATE TABLE users (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  email VARCHAR(255) NOT NULL,
  phone VARCHAR(20) NOT NULL,
  status ENUM('active','blocked') NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL,
  UNIQUE KEY uq_users_email (email),
  KEY ix_users_phone (phone),
  KEY ix_users_created (created_at)
);
-- orders as defined in "Primary keys", plus:
--   CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users (id)
```

```sql
EXPLAIN SELECT id, created_at FROM orders WHERE user_id = 42 ORDER BY created_at DESC LIMIT 10;
-- -> Limit: 10 row(s)
--     -> Covering index lookup on orders using ix_orders_user_created (user_id = 42) (reverse)

EXPLAIN SELECT status, created_at FROM orders
WHERE status = 'paid' AND created_at >= '2024-05-01' ORDER BY created_at LIMIT 20;
-- -> Limit: 20 row(s)
--     -> Filter: ((orders.`status` = 'paid') and (orders.created_at >= TIMESTAMP'2024-05-01 00:00:00'))
--         -> Covering index range scan on orders using ix_orders_status_created
--              over (status = 'paid' AND '2024-05-01 00:00:00.000' <= created_at)

-- No predicate on the leading column: the optimizer may use a skip scan instead of a full scan
EXPLAIN SELECT id FROM orders WHERE created_at >= '2024-03-01' AND created_at < '2024-03-02';
-- -> Filter: (...)
--     -> Covering index skip scan on orders using ix_orders_status_created over '2024-03-01 ...' <= created_at < ...
```

"Covering index" in the tree output (or `Using index` in traditional output) means no clustered-index lookup was needed. The 9.7 tree output prints a `Filter:` line above many range scans that 8.4 did not show; it re-checks conditions that the range already guarantees and is not a sign of a bad plan by itself. A skip scan is only efficient when the leading column has few distinct values; do not rely on it instead of a proper index.

##### Things that prevent index use

**Implicit type conversion.** Comparing a string column with a number converts every row's value to a number: full index scan, and wrong matches (`'1abc' = 1` is true). Comparing an integer column with a string literal is harmless.

```sql
-- BAD: phone is VARCHAR, literal is a number
EXPLAIN SELECT id FROM users WHERE phone = 4915100042;
-- -> Filter: (users.phone = 4915100042)  (rows=9973)
--     -> Covering index scan on users using ix_users_phone  (rows=99733)
-- Warning 1739: Cannot use ref access on index 'ix_users_phone' due to type or collation conversion on field 'phone'
-- Warning 1739: Cannot use range access on index 'ix_users_phone' due to type or collation conversion on field 'phone'

-- GOOD
EXPLAIN SELECT id FROM users WHERE phone = '4915100042';
-- -> Covering index lookup on users using ix_users_phone (phone = '4915100042')  (rows=1)
```

In application code this happens when a driver binds an integer to a `VARCHAR` column (e.g. a Go `int64` for a phone or external ID). Bind strings for string columns.

**Functions on indexed columns.**

```sql
-- BAD: full index scan
EXPLAIN SELECT id FROM users WHERE DATE(created_at) = '2024-01-02';
-- -> Filter: (cast(users.created_at as date) = '2024-01-02')
--     -> Covering index scan on users using ix_users_created  (rows=99733)

-- GOOD: half-open range
EXPLAIN SELECT id FROM users WHERE created_at >= '2024-01-02' AND created_at < '2024-01-03';
-- -> Filter: (...)
--     -> Covering index range scan on users using ix_users_created
--          over ('2024-01-02 00:00:00' <= created_at < '2024-01-03 00:00:00')  (rows=1440)

-- BAD: LOWER() is redundant with a _ci collation and disables the index
EXPLAIN SELECT id FROM users WHERE LOWER(email) = 'user42@example.com';
-- -> Filter: (lower(users.email) = 'user42@example.com')
--     -> Covering index scan on users using uq_users_email  (rows=99733)

-- GOOD: the collation already ignores case
SELECT id FROM users WHERE email = 'USER42@example.com';      -- 42 (const lookup)
```

Avoid `BETWEEN '2024-01-02' AND '2024-01-02 23:59:59'` for `DATETIME(3)` columns: it misses `23:59:59.500`. Use `>=` start and `<` next start.

**Leading wildcards.** `LIKE 'abc%'` is a range scan; `LIKE '%abc'` reads the whole index.

```sql
EXPLAIN SELECT id FROM users WHERE email LIKE 'user42%';
-- -> Filter: (users.email like 'user42%')
--     -> Covering index range scan on users using uq_users_email over ('user42' <= email <= 'user42????...')
EXPLAIN SELECT id FROM users WHERE email LIKE '%42@example.com';
-- -> Filter: (users.email like '%42@example.com')
--     -> Covering index scan on users using uq_users_email  (rows=99733)
```

**`OR` across different columns** may use `index_merge` (union) or fall back to a scan; check the plan, and consider `UNION ALL` of two indexed queries. Run 9.7.4 or later if you rely on `index_merge_union=off`: earlier 9.7 releases could drop a disjunct and return incomplete results with that setting.[^20]

**Collation mismatch** between join columns (see Character Sets) also defeats indexes.

##### Index size limits

With `ROW_FORMAT=DYNAMIC` (default) an index key part may be at most 3072 bytes. In `utf8mb4` that is `VARCHAR(768)`:

```sql
CREATE TABLE ix1 (s VARCHAR(768), KEY (s));    -- OK
CREATE TABLE ix2 (s VARCHAR(769), KEY (s));
-- ERROR 1071 (42000): Specified key was too long; max key length is 3072 bytes
```

Use a prefix index (`KEY (s(191))`) only for non-unique lookups: a prefix `UNIQUE` key enforces uniqueness of the prefix, not the value, and a prefix index can never be covering. For uniqueness of long values, index a hash in a generated column (`sha BINARY(32) AS (UNHEX(SHA2(url, 256))) STORED UNIQUE`). Do not use `MD5()`/`SHA1()` for this on 9.7: they are component functions (UDFs) now and are rejected in generated columns, defaults, `CHECK` constraints and functional indexes even when `classic_hashing` is installed (`ERROR 3763 Expression of generated column 'h' contains a disallowed function: md5.`).

##### Functional indexes and JSON

A functional index is used only when the query contains the **same expression with the same type and collation**. JSON extraction is the classic trap: `->>` returns `utf8mb4_bin`, while `CAST(... AS CHAR)` produces the connection collation, so an index on the plain cast is not used by `doc->>'$.name' = ?`.[^27]

```sql
CREATE TABLE docs (id INT AUTO_INCREMENT PRIMARY KEY, doc JSON NOT NULL);
-- 5,000 rows like {"name": "n42", "age": 42, "tags": ["t0", "t9"]}

-- BAD: the index expression's collation differs from doc->>'$.name'
ALTER TABLE docs ADD INDEX ix_name ((CAST(doc->>'$.name' AS CHAR(32))));
EXPLAIN SELECT id FROM docs WHERE doc->>'$.name' = 'n42';
-- -> Filter: (json_unquote(json_extract(docs.doc,'$.name')) = 'n42')
--     -> Table scan on docs

-- GOOD: match the collation of ->> (utf8mb4_bin)
ALTER TABLE docs DROP INDEX ix_name;
ALTER TABLE docs ADD INDEX ix_name ((CAST(doc->>'$.name' AS CHAR(32)) COLLATE utf8mb4_bin));
EXPLAIN SELECT id FROM docs WHERE doc->>'$.name' = 'n42';
-- -> Index lookup on docs using ix_name ((cast(...) collate utf8mb4_bin) = 'n42')  (rows=1)
```

A virtual generated column with an ordinary index is often clearer, and the optimizer also matches the underlying expression:

```sql
ALTER TABLE docs ADD COLUMN age INT AS (doc->>'$.age') VIRTUAL, ADD INDEX ix_age (age);
EXPLAIN SELECT id FROM docs WHERE doc->>'$.age' = 42;   -- Index lookup on docs using ix_age (age = 42)
EXPLAIN SELECT id FROM docs WHERE age = 42;             -- Covering index lookup on docs using ix_age (age = 42)
```

Multi-valued indexes serve `MEMBER OF`, `JSON_CONTAINS` and `JSON_OVERLAPS` — not `JSON_SEARCH` or `LIKE`:

```sql
ALTER TABLE docs ADD INDEX ix_tags ((CAST(doc->'$.tags' AS CHAR(8) ARRAY)));
EXPLAIN SELECT id FROM docs WHERE 't3' MEMBER OF (doc->'$.tags');
-- -> ... Index lookup on docs using ix_tags (cast(json_extract(doc,_utf8mb4'$.tags') as char(8) array) = json'"t3"')
EXPLAIN SELECT id FROM docs WHERE JSON_CONTAINS(doc->'$.tags', '"t3"');
-- -> ... Index range scan on docs using ix_tags ...
EXPLAIN SELECT id FROM docs WHERE JSON_SEARCH(doc->'$.tags', 'one', 't3') IS NOT NULL;
-- -> Filter: (...) -> Table scan on docs
```

Values longer than the `CHAR(n)` in the `ARRAY` cast cause an error on insert (`ERROR 3907 Data too long for functional index`); size it for the data.

The literals inside a functional or multi-valued index expression are stored with the **connection character set of the session that ran the DDL**. Tested on 9.7.4: `ix_tags` created by a `mysql` client that connected as `latin1` (the default in a container without a locale) is stored as `json_extract(doc,_latin1'$.tags')` and is **not used** by the same `MEMBER OF` query from a `utf8mb4` connection (table scan), and `COLLATE utf8mb4_bin` in the DDL fails with `ERROR 1253 COLLATION 'utf8mb4_bin' is not valid for CHARACTER SET 'latin1'`. Run migrations with `--default-character-set=utf8mb4` and check `SHOW CREATE TABLE` for `_latin1` introducers in index expressions.

##### Descending, invisible and redundant indexes

```sql
CREATE INDEX ix_orders_created_desc ON orders (created_at DESC);
EXPLAIN SELECT id, created_at FROM orders ORDER BY created_at DESC LIMIT 5;
-- -> Limit: 5 row(s) -> Covering index scan on orders using ix_orders_created_desc

-- Test the effect of dropping an index without dropping it:
ALTER TABLE orders ALTER INDEX ix_orders_created_desc INVISIBLE;
EXPLAIN SELECT id, created_at FROM orders ORDER BY created_at DESC LIMIT 5;
-- -> Limit: 5 row(s)
--     -> Sort: orders.created_at DESC, limit input to 5 row(s) per chunk  (rows=199510)
--         -> Covering index scan on orders using ix_orders_user_created
SET SESSION optimizer_switch = 'use_invisible_indexes=on';   -- per-session opt-in for testing
SET SESSION optimizer_switch = 'use_invisible_indexes=off';

ALTER TABLE users ALTER INDEX `PRIMARY` INVISIBLE;
-- ERROR 3522 (HY000): A primary key index cannot be invisible
```

- Descending indexes matter for mixed-direction sorts (`ORDER BY a ASC, b DESC`); for a single-direction sort a backward scan of an ascending index works too.
- Making an index invisible is a metadata change; an invisible index is still maintained on writes and still enforces `UNIQUE`.
- Flag redundant indexes: `KEY (a)` next to `KEY (a, b)`, or `KEY (user_id)` next to the FK index. `sys.schema_redundant_indexes` lists them.
- Flag over-indexing on write-heavy tables: every secondary index costs a B-tree update per write. 9.5.0 turned change buffering back on (`innodb_change_buffering=all`, buffer capped at 5% of the buffer pool), which softens but does not remove that cost.[^12][^9]

##### Histograms

For non-indexed columns used in filters and joins, a histogram gives the optimizer selectivity estimates without index maintenance cost, and can refresh automatically:[^28]

```sql
ANALYZE TABLE orders UPDATE HISTOGRAM ON total WITH 32 BUCKETS AUTO UPDATE;
SELECT column_name,
       JSON_EXTRACT(histogram, '$."histogram-type"') AS type,
       JSON_EXTRACT(histogram, '$."auto-update"')    AS auto
FROM information_schema.column_statistics WHERE table_name = 'orders';        -- total | "equi-height" | true
ANALYZE TABLE orders DROP HISTOGRAM ON total;
```

***

#### Reading Plans: EXPLAIN and EXPLAIN ANALYZE [New in 9.7]

9.5.0 changed the defaults: plain `EXPLAIN` now prints the iterator **tree** (`explain_format=TREE`), and `FORMAT=JSON` uses **version 2** (access-path based).[^12][^29]

```sql
EXPLAIN SELECT id, created_at FROM orders WHERE user_id = 42 ORDER BY created_at DESC LIMIT 10;
-- -> Limit: 10 row(s)  (cost=0.451 rows=2)
--     -> Covering index lookup on orders using ix_orders_user_created (user_id = 42) (reverse)  (cost=0.451 rows=2)

EXPLAIN FORMAT=TRADITIONAL SELECT id, created_at FROM orders WHERE user_id = 42 ORDER BY created_at DESC LIMIT 10;
-- id | select_type | table | ... | type | key                    | rows | Extra
-- 1  | SIMPLE      | orders| ... | ref  | ix_orders_user_created | 2    | Backward index scan; Using index
```

Consequences for code and tooling:

- **BAD**: scripts, CI checks or tests that parse `EXPLAIN` output as the traditional table (`type`, `key`, `rows`, `Extra` columns). Use `EXPLAIN FORMAT=TRADITIONAL` explicitly, or switch to `FORMAT=JSON` and the v2 schema.
- **BAD**: tooling that reads `$.query_block...` from `EXPLAIN FORMAT=JSON`. Version 2 has a different structure; the 8.4 path silently returns `NULL`:

```sql
EXPLAIN FORMAT=JSON INTO @plan SELECT id FROM orders WHERE status = 'paid' LIMIT 5;
SELECT JSON_EXTRACT(@plan, '$.query_block.table.access_type') AS v1_path,     -- NULL on 9.7
       JSON_KEYS(@plan);                                                      -- ["query", "query_plan", "query_type", "json_schema_version"]
SELECT JSON_EXTRACT(@plan, '$.query_plan.inputs[0].index_name');             -- "ix_orders_status_created"
```

- Tree conditions are printed with spaces around `=` (`(user_id = 42)`; 8.4: `(user_id=42)`). Golden-file tests that compare plan text break.
- `EXPLAIN ANALYZE` **executes** the query and reports actual rows, loops and time per iterator. `EXPLAIN ANALYZE FORMAT=JSON INTO @var` (9.0.0) stores the measured plan; it needs `explain_json_format_version=2` (`ERROR 6412` with version 1) and `INTO` only works with `FORMAT=JSON` (`ERROR 6007 EXPLAIN INTO does not support FORMAT=TREE`).[^8]

```sql
EXPLAIN ANALYZE FORMAT=JSON INTO @ap SELECT id FROM orders WHERE status = 'paid' LIMIT 5;
SELECT JSON_EXTRACT(@ap, '$.query_plan.actual_rows'),                -- 5.0
       JSON_EXTRACT(@ap, '$.query_plan.inputs[0].estimated_rows');   -- 99755.0
```

- Single-table `UPDATE`/`DELETE` under `EXPLAIN ANALYZE` print `-> <not executable by iterator executor>`; multi-table forms show timings. Never run `EXPLAIN ANALYZE` of DML or of an expensive `SELECT` against production data on the assumption that it is harmless.
- Key signals: `Table scan`, `Covering index scan` over many rows, `Filter` above a full scan with a large `rows` estimate, `Sort` on big inputs, `Materialize`/temporary tables, `Nested loop` with a large outer side and no index on the inner side, `Hash join` (equi-joins without a usable index).
- Compare estimated `rows` with actual `rows` in `EXPLAIN ANALYZE`; large gaps point to missing statistics (`ANALYZE TABLE`, histograms) or correlated predicates.
- With the hypergraph optimizer enabled, costs are printed as ranges (`cost=1.5..2.99`) and plan shapes differ; compare plans only between runs with the same `optimizer_switch`.

```sql
EXPLAIN ANALYZE SELECT id, total FROM orders WHERE status = 'paid' ORDER BY created_at, id LIMIT 20 OFFSET 40000;
-- -> Limit/Offset: 20/40000 row(s)  (actual time=94.3..94.3 rows=20 loops=1)
--     -> Index lookup on orders using ix_orders_status_created (status = 'paid')  (actual time=1.6..92.2 rows=40020 loops=1)
```

***

#### Query Patterns and Performance

##### Pagination: OFFSET vs keyset — and the row-constructor trap

`LIMIT n OFFSET m` reads and discards `m` rows (40,020 rows read above to return 20). Use keyset (seek) pagination with a deterministic order that ends in a unique column.

**MySQL 9.7 does not use range access for row-constructor inequalities** like `(created_at, id) > (?, ?)`, with either optimizer. The predicate becomes a `Filter` over an index scan that starts at the beginning of the index — as slow as `OFFSET` for later pages. Write the expanded `OR` form, which becomes a range scan:[^30]

```sql
-- BAD: reads from the start of the index and filters (56,040 rows read)
EXPLAIN ANALYZE SELECT id, total FROM orders
WHERE status = 'paid' AND (created_at, id) > ('2024-08-21 10:00:00.000', 160000)
ORDER BY created_at, id LIMIT 20;
-- -> Limit: 20 row(s)  (actual time=218..218 rows=20 loops=1)
--     -> Filter: ((orders.created_at,orders.id) > ('2024-08-21 10:00:00.000',160000))
--         -> Index lookup on orders using ix_orders_status_created (status = 'paid') (actual ... rows=56040 loops=1)

-- GOOD: expanded predicate → range scan, 20 rows read
EXPLAIN ANALYZE SELECT id, total FROM orders
WHERE status = 'paid'
  AND (created_at > '2024-08-21 10:00:00.000'
       OR (created_at = '2024-08-21 10:00:00.000' AND id > 160000))
ORDER BY created_at, id LIMIT 20;
-- -> Limit: 20 row(s)  (actual time=... rows=20 loops=1)
--     -> Index range scan on orders using ix_orders_status_created over
--          (status = 'paid' AND created_at = '2024-08-21 10:00:00.000' AND 160000 < id)
--          OR (status = 'paid' AND '2024-08-21 10:00:00.000' < created_at), with index condition: ...  (actual ... rows=20)
```

Verified identical with `hypergraph_optimizer=on`. Row-constructor *equality* `(a, b) = (?, ?)` and `IN ((?, ?), ...)` are optimized normally.

##### COUNT(*)

InnoDB has no stored row count: `SELECT COUNT(*) FROM t` scans the smallest index (in parallel, `innodb_parallel_read_threads`). For dashboards, use estimates (`information_schema.tables.table_rows`) or maintained counters. Filtered counts need a covering index on the filter columns. `SQL_CALC_FOUND_ROWS`/`FOUND_ROWS()` are deprecated and usually slower than a separate `COUNT(*)`:

```sql
SELECT SQL_CALC_FOUND_ROWS id FROM users LIMIT 1;
-- Warning 1287: SQL_CALC_FOUND_ROWS is deprecated and will be removed in a future release. Consider using two separate queries instead.
SELECT FOUND_ROWS();
-- Warning 1287: FOUND_ROWS() is deprecated and will be removed in a future release. Consider using COUNT(*) instead.
```

##### N+1 queries and IN lists

Replace per-row queries with one join or one `IN (...)` batch. Keep `IN` lists bounded (hundreds to a few thousand values): very large lists increase parse time and memory and can exceed `range_optimizer_max_mem_size` (8 MiB default), after which the optimizer stops using range access and falls back to a scan (with a warning). For huge sets, load the keys into a temporary table and join.

##### Subqueries: MySQL-specific restrictions

```sql
-- Cannot modify a table and select from it in a subquery:
UPDATE users SET status = 'blocked' WHERE id IN (SELECT id FROM users WHERE email LIKE 'zzz%');
-- ERROR 1093 (HY000): You can't specify target table 'users' for update in FROM clause

-- Works: the derived table is materialized first (or use a multi-table UPDATE/DELETE with JOIN)
DELETE FROM users WHERE id IN (SELECT id FROM (SELECT id FROM users WHERE email LIKE 'zzz%') AS x);

-- LIMIT is not allowed inside IN/ALL/ANY/SOME subqueries:
SELECT id FROM users WHERE id IN (SELECT id FROM users ORDER BY id LIMIT 3);
-- ERROR 1235 (42000): This version of MySQL doesn't yet support 'LIMIT & IN/ALL/ANY/SOME subquery'
-- Rewrite as a join against a derived table:
SELECT u.id FROM users u JOIN (SELECT id FROM users ORDER BY id LIMIT 3) AS top3 USING (id);
```

Prefer an explicit multi-table `UPDATE ... JOIN` / `DELETE t FROM t JOIN ...` over the nested-derived-table trick for clarity.

`NOT IN` with a subquery that can return `NULL` returns no rows at all:

```sql
CREATE TABLE nk (id INT PRIMARY KEY);
INSERT INTO nk VALUES (1), (2);
CREATE TABLE ni (v INT);
INSERT INTO ni VALUES (1), (NULL);
SELECT COUNT(*) FROM nk WHERE id NOT IN (SELECT v FROM ni);                           -- 0 (2 is "unknown")
SELECT COUNT(*) FROM nk WHERE NOT EXISTS (SELECT 1 FROM ni WHERE ni.v = nk.id);       -- 1
```

Use `NOT EXISTS` (or `LEFT JOIN ... IS NULL`), or make the column `NOT NULL`.

##### Top-N per group, window functions, LATERAL, QUALIFY

```sql
-- Latest order per user: window function
WITH ranked AS (
  SELECT o.*, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC, id DESC) AS rn
  FROM orders o WHERE user_id IN (1, 2, 3)
)
SELECT user_id, id, created_at FROM ranked WHERE rn = 1;

-- Same with LATERAL: one index probe per user when (user_id, created_at) is indexed
SELECT u.id, lo.id AS last_order
FROM users u,
     LATERAL (SELECT id FROM orders o WHERE o.user_id = u.id ORDER BY created_at DESC LIMIT 1) AS lo
WHERE u.id IN (1, 2, 3);

-- Running totals with an explicit frame and a named window
SELECT user_id, id, total,
       ROW_NUMBER() OVER w AS rn,
       SUM(total) OVER (PARTITION BY user_id ORDER BY created_at ROWS UNBOUNDED PRECEDING) AS running
FROM orders WHERE user_id IN (1, 2)
WINDOW w AS (PARTITION BY user_id ORDER BY created_at DESC)
ORDER BY user_id, created_at;
```

- Always give `ROW_NUMBER()` a total order (add the primary key as a tie-breaker), otherwise results change between runs.
- The default frame with `ORDER BY` is `RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW`, which includes peers with equal sort keys; write `ROWS ...` for running totals.
- **New in 9.x**: a window's `PARTITION BY`/`ORDER BY` may no longer reference columns of an outer query (`ERROR 6432`, 9.2.0), and `RANGE` frames reject nondeterministic `ORDER BY` expressions (`ERROR 3587`, 9.4.0).[^15][^9]

**`QUALIFY` [New in 9.7]**: the clause is reserved since 8.4 but only executes when the **hypergraph optimizer** is enabled, which Community Server allows since 9.7.0:[^1]

```sql
SELECT y, c, amt FROM sales QUALIFY ROW_NUMBER() OVER (PARTITION BY y ORDER BY amt DESC) = 1;
-- ERROR 6037 (HY000): 'QUALIFY clause' can be used only if the hypergraph optimizer is enabled.

SELECT /*+ SET_VAR(optimizer_switch='hypergraph_optimizer=on') */ y, c, amt
FROM sales QUALIFY ROW_NUMBER() OVER (PARTITION BY y ORDER BY amt DESC) = 1;
-- 2024 | de | 10 ;  2025 | de | 7
```

- **BAD**: `QUALIFY` in application SQL that runs with the default optimizer (it fails), or that enables the hypergraph optimizer only to get `QUALIFY` without testing every other query under it. Prefer a derived table or CTE that filters on the window result; it works with both optimizers and on 8.4.
- The hypergraph optimizer is opt-in (`hypergraph_optimizer=off` by default). Enabling it globally changes plans for every query; 9.7.2 and 9.7.4 fixed hypergraph bugs that evaluated `ORDER BY RAND()` more than once per row and that made `UPDATE`/`DELETE ... ORDER BY` with semijoin subqueries **modify the wrong rows**.[^19][^20] Require ≥ 9.7.4, a measured benefit and a regression run of the query workload before enabling it, preferably per statement (`SET_VAR` hint) or per session.

##### CTEs and recursion

```sql
WITH RECURSIVE r (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r WHERE n < 5)
SELECT GROUP_CONCAT(n) FROM r;                                  -- 1,2,3,4,5

WITH RECURSIVE r (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r)
SELECT COUNT(*) FROM r;
-- ERROR 3636 (HY000): Recursive query aborted after 1001 iterations. Try increasing @@cte_max_recursion_depth ...

WITH RECURSIVE r (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r LIMIT 10)
SELECT COUNT(*) FROM r;                                         -- 10 (LIMIT in the recursive CTE)
```

Recursive CTEs over data (trees, graphs) need a termination condition that does not depend on the data being acyclic: carry a depth counter or a path and stop on cycles. Raising `cte_max_recursion_depth` globally hides runaway queries; set it per session (`SET SESSION` or `/*+ SET_VAR(cte_max_recursion_depth = N) */`) only where needed. PostgreSQL's `WITH x AS MATERIALIZED (...)` is a syntax error (use the `MERGE`/`NO_MERGE` optimizer hints instead).

##### Set operations, VALUES, TABLE, ROLLUP

```sql
SELECT 1 AS x UNION SELECT 2 INTERSECT SELECT 2;     -- 1, 2 (INTERSECT binds tighter than UNION)
(SELECT id FROM nk) EXCEPT (SELECT 1);               -- 2
TABLE nk ORDER BY id LIMIT 1;                         -- 1
SELECT * FROM (VALUES ROW(1, 'a'), ROW(2, 'b')) AS v (id, name);   -- VALUES needs ROW()
SELECT y, c, SUM(amt) FROM sales GROUP BY ROLLUP (y, c);           -- same as GROUP BY y, c WITH ROLLUP
```

`UNION` removes duplicates (extra sort/hash); use `UNION ALL` unless de-duplication is intended. **New in 9.1**: `ROLLUP` over an empty input returns one grand-total row instead of an empty result — code that treated "no rows" as "no data" must check the aggregate instead.[^14]

##### GROUP_CONCAT truncation

```sql
SELECT LENGTH(GROUP_CONCAT(email)) FROM users WHERE id <= 200;   -- 1024
-- Warning 1260: Row 55 was cut by GROUP_CONCAT()
```

`group_concat_max_len` defaults to 1024 bytes and truncation is only a warning. Code that builds ID lists or JSON with `GROUP_CONCAT` silently loses data. Use `JSON_ARRAYAGG()`/`JSON_OBJECTAGG()` (limited by `max_allowed_packet`, 64 MiB) or raise `group_concat_max_len` for the session.

##### JSON functions

```sql
SELECT doc->'$.name'  AS arrow,      -- "n1"  (JSON, quoted)
       doc->>'$.name' AS unquote,    -- n1    (text, utf8mb4_bin)
       JSON_VALUE(doc, '$.age' RETURNING UNSIGNED) AS age
FROM docs WHERE id = 1;

SELECT JSON_EXTRACT('{"a": 1}', '$.b') IS NULL;              -- 1: missing path is SQL NULL
SELECT JSON_EXTRACT('{"a": null}', '$.a') IS NULL;           -- 0: JSON null is not SQL NULL
SELECT JSON_TYPE(JSON_EXTRACT('{"a": null}', '$.a')) = 'NULL';   -- 1

SELECT jt.* FROM JSON_TABLE(
  '[{"id":1,"v":"a"},{"id":2,"v":"b"},{"id":"x"}]', '$[*]'
  COLUMNS (rid FOR ORDINALITY,
           id INT PATH '$.id' DEFAULT '0' ON ERROR,
           v VARCHAR(5) PATH '$.v' NULL ON EMPTY)) AS jt;
-- 1 | 1 | a ;  2 | 2 | b ;  3 | 0 | NULL
```

`->` and `->>` only accept a column on the left (`CAST('{"a":1}' AS JSON) ->> '$.a'` is `ERROR 1064`); use `JSON_EXTRACT()`/`JSON_UNQUOTE()` for expressions. Distinguish JSON `null` from a missing key when validating input. Run ≥ 9.7.2 if code uses `JSON_VALUE(... RETURNING DATETIME DEFAULT ...)` in a `DATE` context (earlier releases could return an invalid constant date).[^19]

##### Optimizer hints and index hints

```sql
SELECT /*+ MAX_EXECUTION_TIME(100) */ COUNT(*) FROM orders a JOIN orders b ON a.total = b.total;
-- ERROR 3024 (HY000): Query execution was interrupted, maximum statement execution time exceeded
SELECT /*+ MAX_EXECUTION_TIME(100) */ SLEEP(1);   -- returns 1 (interrupted), no error
SELECT /*+ SET_VAR(sort_buffer_size = 1048576) */ 1;

-- A hint can force a worse plan than the optimizer would choose:
EXPLAIN SELECT /*+ INDEX(orders ix_orders_status_created) */ id FROM orders WHERE user_id = 5 AND status = 'paid';
-- -> Filter: (orders.user_id = 5)
--     -> Index lookup on orders using ix_orders_status_created (status = 'paid')  (rows=99755)
```

- Hints (`/*+ ... */`)[^31] are preferred over `USE/FORCE/IGNORE INDEX`: they are scoped to a query block and can name join orders (`JOIN_ORDER`), index choice (`INDEX`, `NO_INDEX`), and session variables (`SET_VAR`, including `optimizer_switch`). Every hint is a maintenance liability. An optimizer hint naming a missing index only raises `Warning 3128` and is ignored, while `FORCE INDEX (missing)` makes the query fail with `ERROR 1176 Key 'ix_gone' doesn't exist in table`. Search the code for index hints before dropping or renaming an index, and require a comment explaining the measured plan for every hint.
- `MAX_EXECUTION_TIME` / `max_execution_time` apply to read-only `SELECT` only; `SLEEP()` returns 1 when interrupted rather than erroring.
- Some clients and proxies strip comments, and with them the hints.

##### Miscellaneous performance rules

- `ORDER BY RAND() LIMIT n` sorts the whole table; pick random IDs in the application or sample a key range. (`TABLESAMPLE` is reserved but requires a HeatWave secondary engine: `ERROR 3889`.)
- `SELECT *` matters on wide tables, with `TEXT`/`BLOB`/`JSON`/`VECTOR` columns (off-page storage) and when a covering index could serve the query.
- `DISTINCT` to fix duplicate rows from a join usually hides a wrong join; fix the join (or use `EXISTS`).
- Prefer explicit `JOIN ... ON` over comma joins.
- Large `DELETE`/`UPDATE` in one statement create a long transaction, a huge undo log and replica lag; batch by primary-key ranges (see Writes).
- Internal temporary tables use the TempTable engine with `temptable_max_ram` sized from available memory (cgroup limits count only with `--container-aware=ON`) and no mmap spill files (`temptable_max_mmap=0`; `temptable_use_mmap` was removed in 9.4.0).[^9]

***

#### Writes: INSERT, Upserts, UPDATE, DELETE

##### Batch inserts

Multi-row `INSERT ... VALUES (...), (...)` in transactions of a few thousand rows is the standard fast path. `LOAD DATA INFILE` reads files on the server (restricted by `secure_file_priv` and the `FILE` privilege); `LOAD DATA LOCAL INFILE` needs `local_infile=ON` on the server (default `OFF`) **and** client opt-in, and lets the server request any file the client can read — keep it off unless required. There is no `COPY`.

##### INSERT ... ON DUPLICATE KEY UPDATE

Use the row-alias syntax; `VALUES(col)` in the update clause is deprecated:[^32]

```sql
CREATE TABLE inv (sku VARCHAR(20) PRIMARY KEY, qty INT NOT NULL, id INT AUTO_INCREMENT UNIQUE);
INSERT INTO inv (sku, qty) VALUES ('a', 1);

-- BAD (deprecated)
INSERT INTO inv (sku, qty) VALUES ('a', 5) ON DUPLICATE KEY UPDATE qty = qty + VALUES(qty);
-- Warning 1287: 'VALUES function' is deprecated and will be removed in a future release. Please use an
--   alias (INSERT INTO ... VALUES (...) AS alias) and replace VALUES(col) ... with alias.col instead

-- GOOD
INSERT INTO inv (sku, qty) VALUES ('a', 5), ('b', 2) AS new
ON DUPLICATE KEY UPDATE qty = inv.qty + new.qty;
SELECT ROW_COUNT();                  -- 3  (1 per inserted row, 2 per updated row)

-- Column aliases are allowed too
INSERT INTO inv (sku, qty) VALUES ('a', 11) AS new (s, q) ON DUPLICATE KEY UPDATE qty = q;

-- INSERT ... SELECT: alias the SELECT as a derived table
INSERT INTO inv (sku, qty)
SELECT * FROM (SELECT 'b' AS s, 10 AS q) AS src
ON DUPLICATE KEY UPDATE qty = inv.qty + src.q;
```

Pitfalls to flag:

- **Affected-rows semantics**: 1 = inserted, 2 = updated, 0 = matched but unchanged. Code that checks `RowsAffected() == 1` for "row exists now" is wrong; many drivers can switch to found-rows semantics (`CLIENT_FOUND_ROWS`, e.g. Go `clientFoundRows=true`).
- **`AUTO_INCREMENT` values are consumed by the attempted insert even when the row is updated** (`innodb_autoinc_lock_mode=2`), so IDs have gaps; upsert-heavy tables can exhaust an `INT` key quickly.

```sql
INSERT INTO inv (sku, qty) VALUES ('z1', 1), ('z2', 2);
SELECT LAST_INSERT_ID();             -- 7: the FIRST id of the multi-row insert
SELECT sku, id FROM inv ORDER BY id; -- a 1, b 3, z1 7, z2 8  (2, 4, 5, 6 burnt by upserts)
```

- **Multiple unique keys**: if the new row conflicts with different rows on different unique keys, only one row is updated and which one is not specified. Upsert only against tables with a single unique constraint relevant to the conflict.
- `LAST_INSERT_ID()` after an ODKU that updated a row does not return that row's id unless you write `id = LAST_INSERT_ID(id)` in the update list.

##### REPLACE and INSERT IGNORE [changed in 9.6]

`REPLACE` is `DELETE` + `INSERT`: it changes the auto-increment id of a row found through another unique key, fires `DELETE` and `INSERT` triggers, and interacts with foreign keys. **That interaction changed with the SQL-layer foreign keys of 9.6.0**: when the replaced parent row keeps its referenced key value, 9.7 neither cascades nor rejects the statement; when the key value changes, the delete still cascades:

```sql
CREATE TABLE par (id INT PRIMARY KEY, name VARCHAR(10));
CREATE TABLE chi (id INT PRIMARY KEY, par_id INT,
                  FOREIGN KEY (par_id) REFERENCES par (id) ON DELETE CASCADE);
INSERT INTO par VALUES (1, 'one');
INSERT INTO chi VALUES (100, 1), (101, 1);
REPLACE INTO par VALUES (1, 'uno');
SELECT ROW_COUNT();                        -- 2 (one delete + one insert)
SELECT COUNT(*) AS children_left FROM chi; -- 9.7.4: 2   (8.4.13: 0 — the children were cascade-deleted)

CREATE TABLE rp (id INT PRIMARY KEY, name VARCHAR(10));
CREATE TABLE rc (id INT PRIMARY KEY, p INT, FOREIGN KEY (p) REFERENCES rp (id));   -- NO ACTION
INSERT INTO rp VALUES (1, 'one');
INSERT INTO rc VALUES (100, 1);
REPLACE INTO rp VALUES (1, 'uno');         -- 9.7.4: OK, 2 rows affected
                                           -- 8.4.13: ERROR 1451 Cannot delete or update a parent row

CREATE TABLE rp2 (id INT AUTO_INCREMENT PRIMARY KEY, code VARCHAR(10) UNIQUE, name VARCHAR(10));
CREATE TABLE rc2 (id INT PRIMARY KEY, p INT, FOREIGN KEY (p) REFERENCES rp2 (id) ON DELETE CASCADE);
INSERT INTO rp2 (code, name) VALUES ('a', 'x');
INSERT INTO rc2 VALUES (1, 1);
REPLACE INTO rp2 (code, name) VALUES ('a', 'y');   -- conflict on `code`: the row gets id 2
SELECT COUNT(*) FROM rc2;                           -- 0: the children of id 1 were cascade-deleted
```

Code that relied on `REPLACE` to fail (`ERROR 1451`) or to clear child rows behaves differently after the upgrade. The 9.6.0 release notes call the SQL-layer change "fully backward compatible" and do not mention this; it was found by testing (9.7.4 vs. 8.4.13, also with `ON DELETE SET NULL`: the child keeps its reference). Starting the server with the deprecated `--innodb-native-foreign-keys=ON` restores the 8.4 behaviour (tested), at the cost of the other SQL-layer foreign-key changes. Delete triggers on the parent still fire in all cases (tested: an `AFTER DELETE` trigger on the parent fired while the child row survived). Flag `REPLACE` on any table that is referenced by foreign keys or has triggers, and use `INSERT ... ON DUPLICATE KEY UPDATE`.

`INSERT IGNORE` silently drops rows for most errors, including foreign-key violations:

```sql
INSERT IGNORE INTO chi VALUES (102, 999);
-- Warning 1452: Cannot add or update a child row: a foreign key constraint fails ...
SELECT COUNT(*) FROM chi;                 -- 2: row 102 silently not inserted
```

Flag `INSERT IGNORE` unless ignoring every error class is explicitly intended.

##### UPDATE/DELETE in batches

```sql
-- GOOD: bounded batches by primary key, each its own short transaction
DELETE FROM orders WHERE status = 'cancelled' AND created_at < '2024-02-01' ORDER BY id LIMIT 1000;
-- repeat until ROW_COUNT() = 0, sleeping between batches if replicas lag
```

`DELETE`/`UPDATE ... LIMIT` without `ORDER BY` is nondeterministic. Under REPEATABLE READ, a `DELETE ... WHERE created_at < ?` without a usable index locks every scanned row and the gaps between them for the whole transaction.

Multi-table `UPDATE ... JOIN` and `DELETE t1 FROM t1 JOIN t2 ...` are the MySQL forms of `UPDATE ... FROM` (PostgreSQL); `ORDER BY`/`LIMIT` are not allowed with multi-table statements.

***

#### Transactions, Isolation and Locking

##### Consistent reads vs current reads

Under REPEATABLE READ, the snapshot is taken at the **first read** in the transaction, not at `START TRANSACTION` (use `START TRANSACTION WITH CONSISTENT SNAPSHOT` to take it immediately). Plain `SELECT` reads the snapshot; `UPDATE`, `DELETE`, `INSERT ... SELECT` and locking reads see the latest committed data.[^33] Read-modify-write in application code therefore loses updates:

```sql
-- Session A                                        -- Session B
START TRANSACTION;
SELECT bal FROM acct WHERE id = 10;   -- 100
                                                    UPDATE acct SET bal = bal - 50 WHERE id = 10;  -- autocommit
SELECT bal FROM acct WHERE id = 10;   -- 100 (snapshot)
SELECT bal FROM acct WHERE id = 10 FOR SHARE;  -- 50 (current read)
UPDATE acct SET bal = 100 - 30 WHERE id = 10;   -- app computed from the stale 100
COMMIT;
SELECT bal FROM acct WHERE id = 10;   -- 70: B's withdrawal of 50 is lost
```

Fixes, in order of preference:

1. Do the arithmetic in SQL: `UPDATE acct SET bal = bal - 30 WHERE id = 10 AND bal >= 30` and check affected rows.
2. Lock before reading: `SELECT ... FOR UPDATE` inside the transaction.
3. Optimistic concurrency: `UPDATE ... SET ..., version = version + 1 WHERE id = ? AND version = ?` and retry when 0 rows are affected.

REPEATABLE READ in MySQL does **not** detect write skew or raise serialization errors like PostgreSQL's REPEATABLE READ/SERIALIZABLE. `SERIALIZABLE` turns plain `SELECT`s into `FOR SHARE` reads (with autocommit off) and greatly increases locking; it is rarely the right fix.

##### Locks taken by locking reads and writes

InnoDB locks **every index record it scans**, plus the gap before it (next-key locks) under REPEATABLE READ.[^34]

```sql
CREATE TABLE acct (id INT PRIMARY KEY, owner INT NOT NULL, bal INT NOT NULL, note VARCHAR(10), KEY ix_owner (owner));
INSERT INTO acct VALUES (10,1,100,'a'), (20,2,100,'b'), (30,3,100,'c'), (40,4,100,'d');

START TRANSACTION;
SELECT * FROM acct WHERE id BETWEEN 15 AND 25 FOR UPDATE;
SELECT index_name, lock_type, lock_mode, lock_data FROM performance_schema.data_locks
WHERE object_name = 'acct' ORDER BY lock_data;
-- NULL    | TABLE  | IX    | NULL
-- PRIMARY | RECORD | X     | 20     (next-key lock: record 20 + gap before it)
-- PRIMARY | RECORD | X,GAP | 30     (gap before 30: inserts of 21..29 block)
ROLLBACK;

START TRANSACTION;
SELECT * FROM acct WHERE id = 25 FOR UPDATE;     -- no such row
-- PRIMARY | RECORD | X,GAP | 30     -- still blocks inserts of 21..29
ROLLBACK;

START TRANSACTION;
SELECT * FROM acct WHERE note = 'b' FOR UPDATE;  -- note is not indexed
-- PRIMARY | X | 5 locks: every row plus the supremum → the whole table is locked
ROLLBACK;

SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED;
START TRANSACTION;
SELECT * FROM acct WHERE note = 'b' FOR UPDATE;
-- PRIMARY | X,REC_NOT_GAP | 1 lock: non-matching rows are released, no gap locks
ROLLBACK;
```

Review consequences:

- A locking read or `UPDATE`/`DELETE` whose `WHERE` clause is not served by an index locks the whole table in REPEATABLE READ. Make sure such statements use an index.
- "Select-then-insert if absent" with `SELECT ... FOR UPDATE` on a missing key takes a gap lock; two sessions doing it concurrently both get the gap lock and then deadlock on their inserts. Use `INSERT ... ON DUPLICATE KEY UPDATE` or insert first and handle error 1062.
- READ COMMITTED (requires row-based binlogging, the default) takes far fewer gap locks and is a reasonable choice for OLTP services that do not rely on repeatable snapshots. Set it explicitly per session or in the connection string.

##### Deadlocks and lock wait timeouts

```sql
-- Session A                                   -- Session B
START TRANSACTION;
UPDATE acct SET bal = bal - 10 WHERE id = 10;
                                               START TRANSACTION;
                                               UPDATE acct SET bal = bal - 5 WHERE id = 20;
UPDATE acct SET bal = bal + 10 WHERE id = 20;  -- waits for B
                                               UPDATE acct SET bal = bal + 5 WHERE id = 10;
                                               -- ERROR 1213 (40001): Deadlock found when trying to get lock;
                                               --   try restarting transaction  (B is rolled back entirely)
COMMIT;                                        -- A succeeds: 10 → 90, 20 → 110
```

- **Error 1213 (deadlock)** rolls back the whole transaction. The application must retry the **whole transaction** (bounded attempts, backoff), not just the failing statement.
- **Error 1205 (lock wait timeout, `innodb_lock_wait_timeout` = 50 s)** rolls back **only the statement** (with `innodb_rollback_on_timeout=OFF`, the default). The transaction stays open with its earlier changes; code that catches 1205 and then `COMMIT`s commits a partial transaction:

```sql
-- Session A holds a lock on id = 30
-- Session B:
SET SESSION innodb_lock_wait_timeout = 1;
START TRANSACTION;
UPDATE acct SET bal = 0 WHERE id = 40;   -- OK
UPDATE acct SET bal = 0 WHERE id = 30;   -- ERROR 1205 (HY000): Lock wait timeout exceeded; try restarting transaction
SELECT trx_state, trx_rows_modified FROM information_schema.innodb_trx
WHERE trx_mysql_thread_id = CONNECTION_ID();     -- RUNNING | 1   (still open)
COMMIT;                                  -- commits the id = 40 change only
```

  On 1205, roll back explicitly before retrying.
- Prevent deadlocks: touch rows in a consistent order, keep transactions short, avoid user interaction or network calls inside transactions, index the columns used in locking `WHERE` clauses.
- Diagnose with `SHOW ENGINE INNODB STATUS` (`LATEST DETECTED DEADLOCK`) and `innodb_print_all_deadlocks=ON` (default `OFF`); `sys.innodb_lock_waits` shows current waits.

##### Queues: SKIP LOCKED and NOWAIT

```sql
CREATE TABLE jobs (id INT AUTO_INCREMENT PRIMARY KEY,
                   status ENUM('ready','running','done') NOT NULL DEFAULT 'ready',
                   run_at DATETIME NOT NULL,
                   KEY ix_jobs_status_run (status, run_at));
-- Worker 1
START TRANSACTION;
SELECT id FROM jobs WHERE status = 'ready' ORDER BY run_at LIMIT 2 FOR UPDATE SKIP LOCKED;   -- 1, 2
-- Worker 2 (concurrently)
SELECT id FROM jobs WHERE status = 'ready' ORDER BY run_at LIMIT 2 FOR UPDATE SKIP LOCKED;   -- 3, 4
SELECT id FROM jobs WHERE status = 'ready' ORDER BY run_at LIMIT 1 FOR UPDATE NOWAIT;
-- ERROR 3572 (HY000): Statement aborted because lock(s) could not be acquired immediately and NOWAIT is set.
```

`SKIP LOCKED` needs an index that matches the filter and order; otherwise each worker scans (and locks) far more rows. `FOR UPDATE OF tbl` restricts locking to the named tables in a join; `LOCK IN SHARE MODE` is the older spelling of `FOR SHARE`.

##### Implicit commits, autocommit and GTID consistency [New in 9.7]

```sql
START TRANSACTION;
INSERT INTO par VALUES (2, 'two');
CREATE TABLE tmp_x (id INT);     -- implicit COMMIT
ROLLBACK;
SELECT * FROM par WHERE id = 2;  -- 2 | two   (the insert was committed)
```

DDL, `LOCK TABLES`, `START TRANSACTION` (inside another transaction), `SET autocommit = 1`, user/privilege statements and others implicitly commit. Do not mix DDL into transactional application logic or tests that rely on rollback. `CREATE TEMPORARY TABLE` does not commit.

With `gtid_mode=ON` and `enforce_gtid_consistency=ON` (the 9.5.0 defaults), **writing a non-transactional table (MyISAM, MEMORY) inside a transaction that also writes InnoDB fails** — on every server, replicas or not:[^12]

```sql
CREATE TABLE my (id INT PRIMARY KEY) ENGINE=MyISAM;
CREATE TABLE inn (id INT PRIMARY KEY);
START TRANSACTION;
INSERT INTO inn VALUES (1);
INSERT INTO my VALUES (1);
-- ERROR 1785 (HY000): Statement violates GTID consistency: Updates to non-transactional tables can only be done
--   in either autocommitted statements or single-statement transactions, and never in the same statement as
--   updates to transactional tables.
--   (plus Warning 6414: Combining the storage engines MyISAM and InnoDB is deprecated ...)
COMMIT;                                     -- commits the InnoDB row only
-- 8.4.13 with default settings: both inserts succeed
```

Flag MyISAM/MEMORY tables in new schemas and any transaction that touches them together with InnoDB tables (a MEMORY table fails the same way). `CREATE TABLE ... SELECT` and `CREATE TEMPORARY TABLE` inside a transaction remain allowed under GTID consistency (tested).

Long-running transactions (including idle connections left in a transaction by a pool) hold undo history (purge lag, growing undo tablespaces, slower reads) and metadata locks that block DDL. Set statement timeouts in the application and alert on old entries in `information_schema.innodb_trx`.

***

#### Schema Migrations and Online DDL

##### No transactional DDL, no IF NOT EXISTS for columns or indexes

A migration with several statements is applied statement by statement. If statement 3 fails, statements 1–2 stay applied and the migration tool's "rollback" does nothing for DDL. Write migrations as one DDL per file or make them idempotent by checking `information_schema` in the tool, and combine related changes into **one `ALTER TABLE`** (one rebuild, one metadata-lock window):

```sql
-- BAD (MariaDB syntax, ERROR 1064 on MySQL 9.7)
ALTER TABLE inn ADD COLUMN IF NOT EXISTS b INT;
CREATE INDEX IF NOT EXISTS ix ON inn (id);

-- GOOD: one statement, explicit algorithm so it fails fast instead of silently copying
ALTER TABLE orders
  ADD COLUMN note VARCHAR(100) NULL,
  ADD COLUMN source VARCHAR(20) NULL,
  ALGORITHM = INSTANT;
```

`CREATE TABLE IF NOT EXISTS`, `DROP TABLE IF EXISTS`, `CREATE OR REPLACE VIEW`, `CREATE VIEW IF NOT EXISTS` (9.1.0) and `DROP VIEW IF EXISTS` exist; `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS` and `DROP INDEX IF EXISTS` do not. `CREATE VIEW IF NOT EXISTS` leaves an existing view **unchanged** (Note 1050) — it is not a way to deploy a new view definition; use `CREATE OR REPLACE VIEW` for that.

##### Migration scripts and the `mysql` client [New in 9.7]

Since 9.4.0 the `mysql` client starts with `--commands=OFF`.[^9][^13] In a script piped into the 9.7 client:

```sql
-- migrate.sql, run as: mysql -u... app < migrate.sql
SOURCE 001_create.sql;
-- ERROR 1064 (42000): ... near 'SOURCE 001_create.sql'   (sent to the server as SQL)
-- Without the ';' (the usual way to write client commands) the line is glued to the next
-- statement and both fail: near 'SOURCE 001_create.sql\nSELECT 9'
SELECT 2\G
-- ERROR at line 2: Unknown command '\G'.
DELIMITER //
CREATE PROCEDURE pd() BEGIN SELECT 1; END//
DELIMITER ;
-- works: DELIMITER and \C are still processed
USE mysql;
-- works (with the semicolon it is passed to the server); `use mysql` without ';' is glued to the next line
```

- **BAD**: migration runners, Dockerfiles, `docker-entrypoint-initdb.d` scripts or CI steps that rely on `source`, `\. file`, `system`, `tee`, `\G`, `status`, `warnings` or `charset` with the 9.7 client. Concatenate files, call the client once per file, or pass `--commands` explicitly.
- `DELIMITER` remains a client-only command: drivers and migration tools that send statements one at a time must not include it.

##### ALGORITHM and LOCK

Specify `ALGORITHM` (and `LOCK` for `INPLACE`) on every `ALTER TABLE` against a large table. Without it MySQL silently picks the "best" algorithm, which may be a full table copy that blocks writes for hours. With it, an unsupported combination fails immediately.[^35]

Verified on 9.7.4:

| Operation | INSTANT | INPLACE, LOCK=NONE | Notes |
|---|---|---|---|
| `ADD COLUMN` (any position, nullable or with default) | yes | — | consumes a row version |
| `ADD COLUMN ... REFERENCES t (c)` | **no** | **no** | adds a foreign key since 9.0: COPY (or INPLACE with `foreign_key_checks=0`) |
| `DROP COLUMN` | yes | — | consumes a row version |
| `RENAME COLUMN` | yes | | |
| `ALTER COLUMN ... SET/DROP DEFAULT` | yes | | |
| Add `VIRTUAL` generated column | yes | | |
| Add `STORED` generated column | no | no | `ALGORITHM=COPY` only |
| `MODIFY VARCHAR(50) → VARCHAR(60)` (utf8mb4) | no | yes | length prefix stays 1 byte (≤ 255 bytes) |
| `MODIFY VARCHAR(60) → VARCHAR(100)` (utf8mb4) | no | **no** | 240 → 400 bytes crosses the 255-byte length-prefix boundary: COPY |
| Change column type (`INT → BIGINT`) | no | **no** | COPY, blocks writes |
| `NULL → NOT NULL` | no | yes (rebuild) | fails if existing rows contain NULL |
| `ADD INDEX` | no | yes | |
| `RENAME INDEX`, `ALTER INDEX ... INVISIBLE` | no | yes | metadata-only |
| `ADD FOREIGN KEY` | no | only with `foreign_key_checks=0` | with checks on: COPY (and validation) |
| `ADD CHECK` constraint | no | no | COPY (validates all rows) |
| `CONVERT TO CHARACTER SET` | no | no | COPY |
| `DROP PRIMARY KEY, ADD PRIMARY KEY` | no | yes (rebuild) | |

```sql
ALTER TABLE dd MODIFY a VARCHAR(60), ALGORITHM=INSTANT;
-- ERROR 1845 (0A000): ALGORITHM=INSTANT is not supported for this operation. Try ALGORITHM=COPY/INPLACE.
ALTER TABLE dd MODIFY a VARCHAR(100), ALGORITHM=INPLACE, LOCK=NONE;
-- ERROR 1846 (0A000): ALGORITHM=INPLACE is not supported. Reason: Cannot change column type INPLACE. Try ALGORITHM=COPY.
ALTER TABLE dd ADD CONSTRAINT fk_dd FOREIGN KEY (pid) REFERENCES person (id), ALGORITHM=INPLACE;
-- ERROR 1846 (0A000): ALGORITHM=INPLACE is not supported. Reason: Adding foreign keys needs foreign_key_checks=OFF. Try ALGORITHM=COPY.
```

Even `INPLACE, LOCK=NONE` operations rebuild the table (I/O, temporary disk space up to the table size, replica lag because the replica applies the ALTER after it finished on the source). Online DDL also buffers concurrent writes in `innodb_online_alter_log_max_size` (128 MiB default); if it overflows, the ALTER fails at the end. For very large tables use `gh-ost` or `pt-online-schema-change` and verify that the tool version supports MySQL 9.7 (GTIDs on by default, new reserved words, SQL-layer foreign keys).

##### INSTANT row-version limit [New in 9.1]

Each `INSTANT` `ADD COLUMN`/`DROP COLUMN` increments the table's row version; 9.1.0 raised the limit from 64 to **255**:[^14][^35]

```sql
-- after 255 instant add/drop operations:
ALTER TABLE rv ADD COLUMN c128 INT, ALGORITHM=INSTANT;
-- ERROR 4092 (HY000): Maximum row versions reached for table t/rv. No more columns can be added or dropped instantly. Please use COPY/INPLACE.
SELECT total_row_versions FROM information_schema.innodb_tables WHERE name = 't/rv';   -- 255
ALTER TABLE rv ADD COLUMN z INT;        -- no ALGORITHM: silently falls back to a table rebuild (versions reset to 0)
```

Without an explicit `ALGORITHM=INSTANT`, the change after the limit quietly becomes a full rebuild during the deploy. Adding or dropping a column on an **empty** table without an `ALGORITHM` clause uses `INPLACE` and does not consume a row version; an explicit `ALGORITHM=INSTANT` does.

##### Metadata locks: the most common migration outage

Every DDL, even `ALGORITHM=INSTANT`, needs an exclusive metadata lock (MDL) on the table for a moment. It waits for **every open transaction that has touched the table** — including an idle transaction that only ran a `SELECT`. While it waits, **every new query on the table queues behind it**:[^36]

```sql
-- Session A (e.g. a pooled connection left in a transaction)
START TRANSACTION;
SELECT COUNT(*) FROM acct;
-- ... stays open

-- Session B (the migration)
ALTER TABLE acct ADD COLUMN extra INT, ALGORITHM=INSTANT;      -- waits

-- Session C (normal traffic): blocked behind B for as long as A stays open
SELECT COUNT(*) FROM acct;

SELECT id, state, LEFT(info, 60) FROM performance_schema.processlist WHERE state LIKE '%metadata%';
-- 33 | Waiting for table metadata lock | SELECT COUNT(*) FROM acct
-- 31 | Waiting for table metadata lock | ALTER TABLE acct ADD COLUMN extra INT, ALGORITHM=INSTANT
SELECT object_name, lock_type, lock_status FROM performance_schema.metadata_locks WHERE object_name = 'acct';
-- acct | SHARED_READ       | GRANTED
-- acct | SHARED_UPGRADABLE | GRANTED
-- acct | EXCLUSIVE         | PENDING
-- acct | SHARED_READ       | PENDING
```

`lock_wait_timeout` (which governs MDL waits) defaults to **31,536,000 seconds (one year)**. Every migration session should set a short value and retry:

```sql
SET SESSION lock_wait_timeout = 5;
ALTER TABLE acct ADD COLUMN extra2 INT, ALGORITHM=INSTANT;
-- ERROR 1205 (HY000): Lock wait timeout exceeded; try restarting transaction
-- (traffic resumes immediately; the migration tool retries later)
```

Review checklist for migrations: explicit `ALGORITHM`/`LOCK`, short `lock_wait_timeout`, no long transactions running against the table, one `ALTER` per table, backfills in batches, and replica lag considered.

##### Changing columns safely

- Widening `INT` to `BIGINT` is a full copy; plan it early (the most common emergency migration is an exhausted `INT AUTO_INCREMENT`).
- Renaming columns or tables breaks running application versions; use expand/contract: add the new column, dual-write, backfill in batches, switch reads, drop the old column in a later release.
- Converting an existing nullable column to `NOT NULL` rebuilds and fails on existing `NULL`s (backfill first).
- Dropping a column that code still names explicitly breaks the old release during a rolling deploy (making it `INVISIBLE` does not help: invisible columns are still returned when named).
- Adding an `AUTO_INCREMENT` column to an existing table could assign wrong values before 9.7.2; require ≥ 9.7.2 for such migrations.[^19]
- `RENAME TABLE a TO a_old, a_new TO a` is atomic and is the swap step of copy-based tools.

##### Partitioning

Partitioning is a data-lifecycle tool (drop a month with `ALTER TABLE ... DROP PARTITION`), not a general speed-up. Every unique key (including the primary key) must include all partitioning columns, partitioned InnoDB tables cannot have foreign keys, and queries that do not filter on the partition key touch all partitions. Index prefixes are not allowed in partitioning keys.

```sql
CREATE TABLE pt (id INT, s VARCHAR(20), PRIMARY KEY (id, s(5))) PARTITION BY KEY (id, s) PARTITIONS 2;
-- ERROR 1503 (HY000): A PRIMARY KEY must include all columns in the table's partitioning function
--   (prefixed columns are not considered).
```

***

#### Stored Programs, Triggers, Views and Events

##### Stored functions and binary logging

With binary logging enabled (the default), creating a stored function requires a characteristic that declares it safe:[^37]

```sql
DELIMITER //
CREATE FUNCTION f_nd(x INT) RETURNS INT BEGIN RETURN x + 1; END//
-- ERROR 1418 (HY000): This function has none of DETERMINISTIC, NO SQL, or READS SQL DATA in its
--   declaration and binary logging is enabled (you *might* want to use the less safe
--   log_bin_trust_function_creators variable)
CREATE FUNCTION f_ok(x INT) RETURNS INT DETERMINISTIC NO SQL BEGIN RETURN x + 1; END//
DELIMITER ;
```

Declaring `DETERMINISTIC` on a function that is not (uses `NOW()`, `RAND()`, `UUID()`, reads tables that change) is a lie the optimizer and replication rely on; flag it. Setting `log_bin_trust_function_creators=ON` globally to make migrations pass is a security and replication smell.

##### Error handling: handlers, SIGNAL, RESIGNAL

```sql
DELIMITER //
CREATE PROCEDURE withdraw(IN p_id INT, IN p_amt INT)
BEGIN
  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    ROLLBACK;
    RESIGNAL;                 -- re-raise the original error to the caller
  END;
  START TRANSACTION;
  UPDATE acct SET bal = bal - p_amt WHERE id = p_id;
  IF (SELECT bal FROM acct WHERE id = p_id) < 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'insufficient funds', MYSQL_ERRNO = 5001;
  END IF;
  COMMIT;
END//
DELIMITER ;
CALL withdraw(10, 500);   -- ERROR 5001 (45000): insufficient funds   (balance unchanged: 100)
CALL withdraw(10, 5);     -- OK: 95
```

- **BAD**: `DECLARE CONTINUE HANDLER FOR SQLEXCEPTION BEGIN END;` — swallows every error and continues with partial work.
- **BAD**: an `EXIT HANDLER` that does not `ROLLBACK` and does not `RESIGNAL`: the caller sees success while the transaction stays open.
- A procedure that starts and commits its own transaction cannot be safely called from inside a caller's transaction (`START TRANSACTION` implicitly commits the caller's work). Document which side owns the transaction.
- Dynamic SQL in procedures (`PREPARE stmt FROM CONCAT(...)`) has the same injection risk as application code; use `EXECUTE stmt USING @param` for values and validate identifiers against an allow-list.

```sql
SET @lim = 2;
PREPARE ps FROM 'SELECT id FROM par ORDER BY id LIMIT ?';
EXECUTE ps USING @lim;                    -- placeholders work for values, including LIMIT
PREPARE ps2 FROM 'SELECT COUNT(*) FROM ?';
-- ERROR 1064: placeholders cannot stand for identifiers
```

##### JavaScript stored programs and libraries [New in 9.7, Enterprise]

`CREATE FUNCTION/PROCEDURE ... LANGUAGE JAVASCRIPT AS $$ ... $$`, `CREATE LIBRARY ... LANGUAGE JAVASCRIPT|WASM` and `USING (lib)` need the Multilingual Engine (MLE) component, which ships only with **MySQL Enterprise Edition** (and HeatWave).[^8][^15][^9][^38] On Community 9.7.4 the DDL is accepted and only the call fails:

```sql
CREATE FUNCTION gcd(a INT, b INT) RETURNS INT NO SQL LANGUAGE JAVASCRIPT AS $$ return a $$;
-- Query OK, 0 rows affected, 1 warning:  Warning 6001 Language component: Not available.
SELECT gcd(4, 6);
-- ERROR 6001 (HY000): Language component: Not available.
CREATE LIBRARY lib1 LANGUAGE JAVASCRIPT AS $$ export function f() { return 1 } $$;   -- OK, Warning 6001
```

Review rules: JavaScript routines are a hard dependency on Enterprise Edition; flag them in projects that deploy Community (including the official Docker image) or that must stay portable to 8.4. Treat warning 6001 in migration output as a failure. When they are legitimately used, review them like application code (input validation, SQL built with the MLE SQL API's parameter binding, no secrets in code), and remember they run with the routine's `DEFINER` privileges.

##### DEFINER and SQL SECURITY

Views, routines, triggers and events run with the privileges of their `DEFINER` by default (`SQL SECURITY DEFINER`). A view or procedure defined by a privileged account is a privilege-escalation path. Review:

- `DEFINER` clauses in migrations: dumps from one environment often carry `DEFINER=root@localhost` or a personal account that does not exist elsewhere. Setting a definer other than yourself needs `SET_ANY_DEFINER`; orphaned definers need `ALLOW_NONEXISTENT_DEFINER`.
- Prefer `SQL SECURITY INVOKER` for views and routines that only encapsulate queries.

##### Triggers

- Triggers do not fire for foreign-key cascades unless the session sets the deprecated `enable_cascade_triggers=ON` (see Foreign keys), and are skipped by `TRUNCATE TABLE`.
- Row-by-row triggers multiply the cost of bulk statements; complex business logic in triggers is invisible to application reviewers. Keep them small (auditing, denormalized counters) and covered by tests.
- A trigger cannot modify the table that fired it (`ERROR 1442`).
- Since 9.1.0 trigger bodies are parsed only when a statement modifies the table (`table_open_cache_triggers`); a trigger with a body the server can no longer parse shows up only on the first write, not on `SELECT`.[^14]

##### Views

MySQL Server has no materialized views. The syntax exists for HeatWave (9.5.0+), and the 9.7 parser accepts `CREATE MATERIALIZED VIEW` silently: on Community 9.7.4 it creates an **ordinary view** (`SHOW CREATE VIEW` prints `MATERIALIZED /*(BY ENGINE=UNKNOWN)*/`), every query against it recomputes the result and only raises `Note 1105 Materialized view ... could not be used. Query is not running on the view's materialization engine 'UNKNOWN'`; `REFRESH MATERIALIZED VIEW` is a syntax error (8.4.13: `CREATE MATERIALIZED VIEW` is `ERROR 1064`). Flag `CREATE MATERIALIZED VIEW` in code for MySQL Server: it gives no caching at all. Emulate with a summary table refreshed by a job or maintained by the application. Views with `GROUP BY`, `DISTINCT`, aggregates, `UNION` or `LIMIT` are evaluated with a temporary table (`ALGORITHM=TEMPTABLE`) and predicates from the outer query may not be pushed into them; check plans of queries on views.

##### JSON duality views [New in 9.7]

A JSON duality view (9.4.0) exposes relational rows as JSON documents; since 9.7.0 Community Server also accepts `INSERT`/`UPDATE`/`DELETE` through them.[^9][^1][^39] Writes are translated into DML on the base tables, and every document carries an ETAG for optimistic concurrency:

```sql
CREATE TABLE customer (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(50) NOT NULL);
CREATE TABLE corder (id INT AUTO_INCREMENT PRIMARY KEY, customer_id INT NOT NULL, amount DECIMAL(10,2),
                     FOREIGN KEY (customer_id) REFERENCES customer (id));
INSERT INTO customer VALUES (1, 'Ann');
INSERT INTO corder VALUES (1, 1, 9.50);

CREATE JSON DUALITY VIEW customer_dv AS
SELECT JSON_DUALITY_OBJECT( WITH (INSERT, UPDATE, DELETE)
  '_id': c.id, 'name': c.name,
  'orders': (SELECT JSON_ARRAYAGG(JSON_DUALITY_OBJECT( WITH (INSERT, UPDATE, DELETE) 'oid': o.id, 'amount': o.amount))
             FROM corder o WHERE o.customer_id = c.id)
) FROM customer c;

SELECT * FROM customer_dv;
-- {"_id": 1, "name": "Ann", "orders": [{"oid": 1, "amount": 9.50}], "_metadata": {"etag": "1fde97..."}}
INSERT INTO customer_dv VALUES ('{"_id": 2, "name": "Bob", "orders": [{"oid": 2, "amount": 5}]}');   -- 2 rows affected
UPDATE customer_dv SET data = JSON_SET(data, '$.name', 'Ann B') WHERE data->'$._id' = 1;              -- OK
UPDATE customer_dv SET data = '{"_id": 1, "name": "X", "orders": [{"oid": 1, "amount": 9.50}],
                                "_metadata": {"etag": "00000000000000000000000000000000"}}'
WHERE data->'$._id' = 1;
-- ERROR 6494 (HY000): Cannot update JSON duality view. The ETAG of the document in the database did not
--   match the ETAG '"00000000000000000000000000000000"' passed in.
DELETE FROM customer_dv WHERE data->'$._id' = 2;      -- 2 rows affected: the customer and its order
```

Review rules:

- Clients that write full documents must send back the `_metadata.etag` they read and handle `ERROR 6494` as a concurrency conflict (re-read and retry), not as a generic failure.
- A single document write changes several base tables; `DELETE` through the view deletes child rows too (`WITH (DELETE)` on the nested object). Review the `WITH (...)` tags on every nested object: they define what an API client can do to each table.
- Field-level `WITH (CHECK | NO CHECK | UPDATE | NO UPDATE)` annotations are 26.10 syntax, not 9.7.

##### Events

The event scheduler (`event_scheduler=ON` by default) runs events with the definer's privileges on the source; on replicas, replicated events are created disabled (`REPLICA_SIDE_DISABLED`). `CREATE/ALTER/DROP EVENT` can be prepared since 9.0.0 (without `?` placeholders); on 8.4 that is `ERROR 1295`. `DISABLE ON SLAVE` is deprecated in favour of `DISABLE ON REPLICA`.

***

#### Security

##### SQL injection: parameters for values, allow-lists for identifiers

Every value must be sent as a bound parameter. Identifiers (table, column, `ORDER BY` column, `ASC`/`DESC`) cannot be parameters and must come from a fixed allow-list in code. Verified with Go `database/sql` and `github.com/go-sql-driver/mysql` v1.10.1 against 9.7.4:[^40]

```go
// BAD: string concatenation — the input "x' OR '1'='1" matches every row
evil := "x' OR '1'='1"
db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE email = '"+evil+"'").Scan(&n) // n == 100000

// GOOD: placeholder
db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE email = ?", evil).Scan(&n)     // n == 0

// BAD: a placeholder in ORDER BY binds a string constant — the query runs without error
// but sorts by a constant, i.e. not at all.
db.QueryContext(ctx, "SELECT id FROM users ORDER BY ? LIMIT 1", "email")                // err == nil

// GOOD: allow-list
var sortColumns = map[string]string{"email": "email", "created": "created_at"}
col, ok := sortColumns[userInput]
if !ok {
	col = "id"
}
q := "SELECT id FROM users ORDER BY " + col + " LIMIT 50"

// GOOD: IN lists — one placeholder per value
ids := []any{1, 2, 3}
q = "SELECT COUNT(*) FROM users WHERE id IN (?" + strings.Repeat(",?", len(ids)-1) + ")"
db.QueryRowContext(ctx, q, ids...).Scan(&n)                                              // n == 3
```

- Escaping functions (`mysql_real_escape_string`, hand-written `strings.ReplaceAll(s, "'", "''")`) are not a substitute: they depend on the connection character set and on `NO_BACKSLASH_ESCAPES`.
- Keep `multiStatements` (and equivalents in other drivers) **off**: with it disabled, a stacked query is a syntax error (`Error 1064`), which removes a whole class of injection payloads.
- ORMs and query builders are safe only in their parameterized APIs; `Raw`, `Literal`, `Expr`, string-built `WHERE` fragments need the same review.

##### LIKE patterns

User input inside `LIKE` must escape `%`, `_` and the escape character `\`:

```go
user := "user1_"
// BAD: '_' is a wildcard — matches user10..., user11..., ... (11112 rows in the test data)
db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE email LIKE CONCAT(?, '%')", user).Scan(&n)
// GOOD
esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(user)
db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE email LIKE CONCAT(?, '%')", esc).Scan(&n) // 0
```

The default escape character is `\` (unless `NO_BACKSLASH_ESCAPES` is set; then add an explicit `ESCAPE '!'` clause and escape with that character).

##### Authentication: caching_sha2_password only [New in 9.7]

The server-side `mysql_native_password` plugin was removed in 9.0.0, together with `--mysql-native-password`. Every account must use `caching_sha2_password` (or an external/plugin method), and every client library must support it:[^8][^17]

- **Upgrade blocker**: accounts that still use `mysql_native_password` on 8.4 cannot authenticate after the upgrade to 9.7 (tested: the error log says `The plugin 'mysql_native_password' used to authenticate user 'legacy'@'%' is not loaded. Nobody can currently login using this account.`; the login fails with `ERROR 1524`), and the 8.4 escape hatch (`--mysql-native-password=ON`) no longer exists — a `my.cnf` that sets it prevents the server from starting. Migrate them on 8.4 first: `SELECT user, host FROM mysql.user WHERE plugin = 'mysql_native_password'`, then `ALTER USER ... IDENTIFIED WITH caching_sha2_password BY '...'` (the same statement also repairs such an account after the upgrade).
- The first login of an account after a server restart or a cache flush needs either a **TLS connection** or RSA public-key exchange; over plain TCP without either, the login fails:

```text
$ mysql -upb2 -p... -h127.0.0.1 --ssl-mode=DISABLED
ERROR 2061 (HY000): Authentication plugin 'caching_sha2_password' reported error: Authentication requires secure connection.
```

  This is the classic "works until the database restarts" bug. Use TLS (the server auto-generates certificates; clients default to `PREFERRED`), and for production require it: `require_secure_transport=ON` on the server or `REQUIRE SSL` per account, plus `VERIFY_IDENTITY` in clients. Flag driver options such as `allowPublicKeyRetrieval=true` (JDBC) or `tls=skip-verify` (Go) in production configs: they accept a man-in-the-middle key.
- Old client libraries (very old PHP `mysqlnd`, `MySQLdb` against libmysqlclient 5.x, the Node `mysql` package) do not support `caching_sha2_password`; `mysql2` and current connectors do.
- **New in 9.5/9.7**: `caching_sha2_password_digest_rounds` defaults to 10,000 (5,000 on 8.4), and 9.7.0 adds a PBKDF2-with-SHA-512 storage format (`caching_sha2_password_storage_format`, default `CRYPT5`; `caching_sha2_password_enforce_storage_format` forces users with another format to reset their passwords).[^12][^17] Changing the format affects only passwords set afterwards.
- Account management: `CREATE USER ... IDENTIFIED BY`, `ALTER USER`, password policies (`PASSWORD EXPIRE`, `FAILED_LOGIN_ATTEMPTS n PASSWORD_LOCK_TIME d`; 9.6.0 adds `performance_schema.temporary_account_locks` to see locked accounts).[^10] `GRANT` does not create users, `PASSWORD()` does not exist, and **`FLUSH PRIVILEGES` is deprecated** (warning 1681) and never needed after account statements.

##### Least privilege and roles [New in 9.7]

```sql
CREATE ROLE app_read, app_write;
GRANT SELECT ON t.* TO app_read;
GRANT INSERT, UPDATE, DELETE ON t.* TO app_write;

CREATE USER app@'%' IDENTIFIED BY 'App-pass-123';
GRANT app_read, app_write TO app@'%';
-- BAD: roles are granted but not active — activate_all_roles_on_login is OFF by default
--   $ mysql -uapp ... t  →  ERROR 1044 (42000): Access denied for user 'app'@'%' to database 't'

CREATE USER app2@'%' IDENTIFIED BY 'App-pass-123';
GRANT app_read TO app2@'%';
SET DEFAULT ROLE ALL TO app2@'%';      -- GOOD: active at login
--   $ mysql -uapp2 ... t -e 'SELECT CURRENT_ROLE()'  →  `app_read`@`%`
```

**New in 9.5**: `activate_mandatory_roles` (default `ON`) activates the roles listed in `mandatory_roles` at login for every account. On 8.4, mandatory roles were granted but inactive unless `activate_all_roles_on_login=ON` or `SET DEFAULT ROLE` was used:[^12]

```sql
GRANT SELECT ON t.* TO app_read;
SET PERSIST mandatory_roles = 'app_read';
CREATE USER m1@'%' IDENTIFIED BY 'M1-pass-123';
--   $ mysql -um1 ... t -e 'SELECT CURRENT_ROLE(); SELECT COUNT(*) FROM users'
--   9.7.4: `app_read`@`%` | 100000
--   8.4.13: ERROR 1044 (42000): Access denied for user 'm1'@'%' to database 't'
```

Review rules:[^41][^42]

- After an upgrade from 8.4, every account immediately gets the privileges of `mandatory_roles`. Review that list: a broad mandatory role is now a broad grant to everyone, including application and monitoring accounts.
- Application accounts get DML on their schema only; migrations run under a separate account with DDL privileges. No `ALL PRIVILEGES ON *.*`, no `SUPER`, `FILE`, `PROCESS`, `SYSTEM_USER`, `GRANT OPTION` or `CONNECTION_ADMIN` for applications.
- Every non-mandatory role grant needs `SET DEFAULT ROLE` (or `SET ROLE` in the session).
- `_` and `%` in database names in `GRANT ... ON db_name.*` are wildcards: `GRANT SELECT ON db_1.*` also matches `dbx1`. Escape them (`` `db\_1` ``) or avoid them in schema names.
- Host part: prefer specific hosts or network ranges over `'%'`; `'localhost'` matches only socket/`localhost` connections.
- `CURRENT_ROLE_IN()` / `CURRENT_USER_IN()` exist in Community 9.7 but are meant for Enterprise masking policies; do not build authorization logic in SQL views on them.
- Credentials never in migrations or code; use environment variables or a secret store, and do not log DSNs (the Go driver's `FormatDSN()` contains the password).

##### Hashing and encryption functions [New in 9.7]

- `MD5()` and `SHA1()` do not exist on a default 9.7 server (moved to the `classic_hashing` component in 9.6.0). Existing code, generated columns, `CHECK` constraints, defaults and views that call them fail:

```sql
SELECT MD5('a');          -- ERROR 1305 (42000): FUNCTION t.MD5 does not exist
INSTALL COMPONENT 'file://component_classic_hashing';
SELECT MD5('a'), SHA1('a');   -- 0cc175b9c0f1b6a831c399e269772661 | 86f7e437faa5a7fce15d1ddcb9eaeaea377667b8
UNINSTALL COMPONENT 'file://component_classic_hashing';
```

  The component functions are UDFs: they work in queries and views, but are rejected in generated columns, `DEFAULT` expressions, `CHECK` constraints and functional indexes even when the component is installed (`ERROR 3763`, `3770`, `3814`, `3758`).[^45] Prefer `SHA2(x, 256)` for checksums and deduplication keys; install the component only as a bridge for queries and track its removal.
- **Upgrade trap** (tested 8.4.13 → 9.7.4 in place): a table with `MD5()`/`SHA1()` in a `DEFAULT` expression, a functional index or a `CHECK` constraint made the 9.7.4 server **crash during the upgrade's table-definition check** (`Error MY-001305 occurred while checking table-definitions: FUNCTION mysql.md5 does not exist`, `Incorrect information in file: './t/d.frm'`, then `mysqld got signal 6`), also with `--check-table-functions=WARN`; the 8.4.13 server still started on the same data directory afterwards. A table with `MD5()` in a `STORED` generated column upgraded silently but was unusable afterwards: `SELECT`, `INSERT`, `SHOW CREATE TABLE` fail with `ERROR 1305`, and with the component installed with `ERROR 3763`; even `ALTER TABLE ... DROP COLUMN` fails, only `DROP TABLE` works (the data is lost unless exported before the upgrade). A view that calls `MD5()` fails with `ERROR 1356` until the component is installed. Remove every such expression on 8.4 before upgrading (replace with `SHA2()` or a trigger) and test the upgrade on a copy of the data.
- No SQL hash function is suitable for passwords; hash passwords in the application with bcrypt/scrypt/Argon2.
- `AES_ENCRYPT()` defaults to `block_encryption_mode=aes-128-ecb`: deterministic, no IV, leaks equal plaintexts. There is no authenticated (GCM) mode. Prefer encrypting in the application; if it must happen in SQL, use a CBC mode with a random IV per value and keep keys out of SQL text (they appear in logs and the processlist):

```sql
SELECT HEX(AES_ENCRYPT('secret', 'key')) = HEX(AES_ENCRYPT('secret', 'key')) AS ecb_deterministic;  -- 1
SET SESSION block_encryption_mode = 'aes-256-cbc';
SET @iv = RANDOM_BYTES(16);
SET @key = UNHEX(SHA2('k', 256));
SELECT AES_DECRYPT(AES_ENCRYPT('secret', @key, @iv), @key, @iv);   -- secret
SELECT AES_ENCRYPT('secret', 'key');            -- ERROR 1582: CBC requires the IV argument
SET SESSION block_encryption_mode = 'aes-256-gcm';   -- ERROR 1231: not supported
SET SESSION block_encryption_mode = DEFAULT;
```

##### Other server-side security settings

- `local_infile=OFF` (default): keep it off; `LOAD DATA LOCAL` lets a malicious server read client files.
- `secure_file_priv` restricts `LOAD DATA INFILE` / `SELECT ... INTO OUTFILE` to one directory; an empty value (`""`) allows the whole filesystem.
- Replication connections are encrypted by default since 9.5.0 (`SOURCE_SSL=1`); do not turn it off in `CHANGE REPLICATION SOURCE TO` scripts.[^12]
- Logs (general log, slow log, binlog, `performance_schema` digests and history) contain literal values from non-parameterized statements: another reason to bind parameters.

***

#### Application Code and Drivers

Verified with `github.com/go-sql-driver/mysql` v1.10.1 against 9.7.4; the same concepts apply to other drivers.[^40]

```go
cfg := mysql.NewConfig()
cfg.User, cfg.Passwd, cfg.Net, cfg.Addr, cfg.DBName = "app", os.Getenv("DB_PASSWORD"), "tcp", "db:3306", "t"
cfg.ParseTime = true                                 // DATETIME/TIMESTAMP → time.Time
cfg.Loc = time.UTC                                   // how DATETIME values are interpreted
cfg.Params = map[string]string{"time_zone": "'+00:00'"} // session time_zone for TIMESTAMP conversion
// cfg.TLSConfig = "custom" (registered with mysql.RegisterTLSConfig) for verified TLS
db, err := sql.Open("mysql", cfg.FormatDSN())
// cfg.FormatDSN() == "app:...@tcp(db:3306)/t?parseTime=true&time_zone=%27%2B00%3A00%27"
```

- Without `parseTime=true`, scanning a `DATETIME` into `time.Time` fails: `unsupported Scan, storing driver.Value type []uint8 into type *time.Time`.
- `loc` and `time_zone` must agree with how the data is written; a mismatch shifts every `TIMESTAMP` by the zone offset.
- `multiStatements=false` (default) — keep it.
- `interpolateParams=true` makes the driver substitute parameters client-side. Combined with the multibyte encodings BIG5, CP932, GB2312, GBK or SJIS it allows SQL injection; the driver rejects only a DSN whose `collation=` names such an encoding (`invalid DSN: interpolateParams can not be used with unsafe collations`), while `charset=sjis&interpolateParams=true` is accepted silently (tested with v1.10.1). Leave it off unless measured, and never combine it with those character sets.
- Pool settings: `SetConnMaxLifetime` below the server's `wait_timeout` (28,800 s default) and any proxy/load-balancer idle timeout; `SetMaxOpenConns` below `max_connections` (151 default) divided by the number of application instances.
- Classify errors by number, not message text:

```go
var me *mysql.MySQLError
if errors.As(err, &me) {
	switch me.Number {
	case 1062: // ER_DUP_ENTRY → conflict
	case 1213: // ER_LOCK_DEADLOCK → transaction already rolled back: retry the whole transaction
	case 1205: // ER_LOCK_WAIT_TIMEOUT → statement rolled back only: ROLLBACK, then retry
	case 3572: // ER_LOCK_NOWAIT
	case 3024: // ER_QUERY_TIMEOUT (max_execution_time)
	case 1452, 1451: // FK violation on child insert / parent delete (now also from inline REFERENCES)
	case 1785: // GTID consistency violation (non-transactional table in a transaction)
	case 6494: // JSON duality view ETAG mismatch → re-read and retry
	case 1305: // FUNCTION ... does not exist (e.g. MD5/SHA1 without classic_hashing)
	}
}
```

- Use `context` deadlines (`QueryContext`, `ExecContext`). On cancellation the Go driver abandons the connection, but the server may keep executing the statement; bound expensive reads with `MAX_EXECUTION_TIME` as well. Always `defer rows.Close()` and check `rows.Err()`.
- Transactions: `BeginTx` with `sql.TxOptions{Isolation: sql.LevelReadCommitted}` sets the isolation per transaction. Never run transaction statements (`START TRANSACTION`, `COMMIT`) through `db.Exec` on a pool — they land on different connections.
- Invalid temporal values sent *with a temporal type* through the binary protocol (prepared statements) are rejected even in non-strict mode since 9.5.0. Parameters bound as strings are unaffected: the Go driver sends a `string` as a string, and `'2024-13-45 00:00:00'` bound to a `DATETIME` column under `sql_mode=''` was still stored as a zero date with a warning on 9.7.4. Use strict mode and validate input.[^12]
- Drivers that query `@@version` or parse `VERSION()` keep working (three-part `9.7.4`); use `VERSION_WITH_UPDATE()` only on ≥ 9.7.4.

***

#### Replication and Operations Awareness [New in 9.7]

- **GTIDs are on by default** (9.5.0). Besides the consistency errors above, scripts that set `gtid_next`, use `sql_slave_skip_counter`-style recovery, or restore dumps with `SET @@GLOBAL.GTID_PURGED` need review; `mysqldump` adds `SET @@SESSION.SQL_LOG_BIN=0` and `GTID_PURGED` lines when GTIDs are on.[^12]
- Use `SOURCE`/`REPLICA` terminology (`CHANGE REPLICATION SOURCE TO`, `START REPLICA`, `SHOW REPLICA STATUS`, `SHOW BINARY LOG STATUS`); the `MASTER`/`SLAVE` statements were removed in 8.4. `replica_parallel_type` is gone (9.5.0) and `replica_parallel_workers` must be ≥ 1 (9.3.0).[^12][^16]
- Replication connections use TLS by default (`SOURCE_SSL=1`); Group Replication defaults to `group_replication_ssl_mode=REQUIRED`.[^12]
- `replica_allow_higher_version_source` (9.7.0) controls whether a replica accepts a source with a higher version (for rolling upgrades).[^1]
- Upgrades: 8.4 → 9.7 in place, by dump/load or via replication; 9.7 → the next LTS later; Innovation releases (26.x) cannot be downgraded in place.[^3][^43] Run MySQL Shell's upgrade checker (`util.checkForServerUpgrade()`) before upgrading.
- Large single transactions (bulk `UPDATE`/`DELETE`, `ALTER TABLE` copies) create replica lag proportional to their duration. Batch them.
- Statements with nondeterministic results (`LIMIT` without `ORDER BY`, `SYSDATE()`, `UUID()`) are safe only because of row-based logging; keep `binlog_format=ROW` (the variable itself is deprecated).
- Foreign-key cascades are SQL-layer operations since 9.6.0 and appear as `Delete_rows`/`Update_rows` events for the child tables in the binary log (tested); CDC consumers (Debezium and similar) now receive cascaded child changes that were invisible to them on 8.4, and may need to stop emulating cascades themselves.[^10]
- Reads from replicas are eventually consistent; read-your-writes paths must go to the source (or wait with `WAIT_FOR_EXECUTED_GTID_SET()`).
- Container deployments: InnoDB derives buffer-pool instances, I/O and purge threads, `temptable_max_ram` and (with `--innodb-dedicated-server`) the buffer pool and redo capacity from the available CPUs and memory (9.3.0). The container's cgroup memory and CPU-quota limits are honoured only with `--container-aware=ON` (9.6.0; the default `OFF` ignores them and logs warning MY-015655), while a `cpuset` restriction is counted either way (9.7.0). In a memory-limited container, start the server with `--container-aware=ON` or set the memory-dependent variables explicitly (`innodb_buffer_pool_size`, `temptable_max_ram`); otherwise the server sizes itself for the whole host and can be OOM-killed.[^16][^10][^1]

***

#### Observability

```sql
-- Top statements by total latency (performance_schema is on by default)
SELECT DIGEST_TEXT, COUNT_STAR,
       ROUND(SUM_TIMER_WAIT / 1e12, 3) AS total_s,
       ROUND(AVG_TIMER_WAIT / 1e9, 3)  AS avg_ms,
       SUM_ROWS_EXAMINED, SUM_ROWS_SENT, SUM_NO_INDEX_USED
FROM performance_schema.events_statements_summary_by_digest
WHERE SCHEMA_NAME = 't'
ORDER BY SUM_TIMER_WAIT DESC LIMIT 10;

SELECT * FROM sys.statements_with_full_table_scans LIMIT 10;
SELECT * FROM sys.schema_unused_indexes WHERE object_schema = 't';     -- since last restart only
SELECT table_name, redundant_index_name, dominant_index_name
FROM sys.schema_redundant_indexes WHERE table_schema = 't';
SELECT waiting_pid, blocking_pid, sql_kill_blocking_connection FROM sys.innodb_lock_waits;
SELECT object_schema, object_name, waiting_pid, blocking_pid FROM sys.schema_table_lock_waits;  -- MDL waits
SELECT * FROM information_schema.innodb_trx ORDER BY trx_started LIMIT 5;                      -- old transactions
```

- The `sys` schema views above are installed by default.[^44]
- Slow query log: `slow_query_log=OFF` and `long_query_time=10` by default; enable it with a threshold that matches the service's latency budget and analyze with `pt-query-digest` or `mysqldumpslow`.
- `SUM_ROWS_EXAMINED / SUM_ROWS_SENT` far above 1 indicates a missing or unused index. 9.5.0 improved digests so queries that differ only in `ORDER BY` no longer share a digest; dashboards keyed on digest values change after the upgrade.[^12]
- `sys.schema_unused_indexes` counts only since the last restart and only on this server. Make an index invisible before dropping it.
- **New in 9.7**: the Telemetry component (OpenTelemetry traces, metrics and logs, including slow and general log export) and the replication applier metrics tables (`performance_schema.replication_applier_metrics`, `replication_applier_progress_by_worker`) are available in Community. Both ship as components that must be installed first (`INSTALL COMPONENT 'file://component_telemetry'` / `'file://component_replication_applier_metrics'`; tested: the tables do not exist until then).[^1]
- `EXPLAIN FOR CONNECTION <id>` shows the plan of a running statement.

***

#### Removed / Changed in 9.7 — Migration Notes (from 8.4)

1. **Authentication**: migrate every `mysql_native_password` account to `caching_sha2_password` on 8.4 *before* upgrading; remove `mysql-native-password` and `mysql-native-password-proxy-users` from `my.cnf`. Make sure every client supports `caching_sha2_password` and uses TLS or RSA key retrieval.
2. **Inline `REFERENCES`**: search all DDL and migration files for column-level `REFERENCES`. On 9.7 they create constraints: clean up orphans, decide whether the constraint is wanted (convert to a named table-level `FOREIGN KEY` or delete the clause), and expect `ADD COLUMN ... REFERENCES` to copy the table.
3. **`MD5()`/`SHA1()`**: on 8.4, before upgrading, remove them from generated columns, `DEFAULT` expressions, `CHECK` constraints and functional indexes (in tests the upgrade crashed for defaults, `CHECK` constraints and functional indexes, and left tables with stored generated columns unreadable; see Security). In queries and views, replace them with `SHA2()` or install `classic_hashing` after the upgrade.
4. **Reserved words**: quote `EXTERNAL` and `LIBRARY` used as identifiers (`SELECT word FROM information_schema.keywords WHERE reserved = 1` on the target).
5. **GTIDs on by default**: find transactions that mix MyISAM/MEMORY and InnoDB writes (they fail with `ERROR 1785`), convert those tables to InnoDB, and review backup/restore scripts for GTID handling. Set `gtid_mode=OFF` explicitly only as a deliberate, documented choice.
6. **EXPLAIN tooling**: scripts that parse `EXPLAIN` output must request `FORMAT=TRADITIONAL` or understand the tree / JSON v2 formats.
7. **`mysql` client scripts**: replace `source`, `\.`, `system`, `\G`, `tee` and similar client commands in scripts, or run the client with `--commands`.
8. **Removed variables and plugins**: `innodb_log_file_size`, `innodb_log_files_in_group` (use `innodb_redo_log_capacity`), `innodb_undo_tablespaces`, `temptable_use_mmap`, `replica_parallel_type`, `group_replication_allow_local_lower_version_join`, `semisync_master`/`semisync_slave`, Version Tokens. Remove them from `my.cnf`, Helm values and Terraform parameter groups; the server refuses to start with unknown options.
9. **Changed results**: `IGNORE` with multi-row scalar subqueries now fails; `ROLLUP` on empty input returns a row; schema-qualified derived-table column references fail; outer references in window clauses fail; temporal function edge cases (`TIMEDIFF`, `FROM_DAYS`, `DAYNAME` arithmetic, `DATE` assigned to integer columns) return different values; `REPLACE` on a referenced parent row no longer cascades or fails when the key value is unchanged. Run the test suite against 9.7.
10. **Roles**: review `mandatory_roles` — they become active for every account at login (`activate_mandatory_roles=ON`).
11. **Spatial indexes**: drop and re-create `SPATIAL` indexes as part of the upgrade (9.2.0 incompatible change for an index-corruption fix).[^15]
12. **Containers**: decide on `--container-aware=ON` or explicit memory settings; the default ignores cgroup memory limits.
13. **Downgrade**: an in-place downgrade from 9.7 to 8.4 is not supported (only within an LTS series); keep a logical backup from before the upgrade.[^3][^43]
14. **Keep deprecated features out of new code**: `FLUSH PRIVILEGES`, `enable_cascade_triggers`, `VALUES()` in upserts, `SQL_CALC_FOUND_ROWS`, `utf8mb3`, `&&`/`||`, display widths, `ZEROFILL`, user-variable assignment in expressions, mixed-engine transactions.

***

#### Production / Review Checklist

##### Schema and migrations

- [ ] Every table has a narrow, immutable primary key (`BIGINT UNSIGNED AUTO_INCREMENT` or `BINARY(16)`), never `CHAR(36)` UUIDs
- [ ] `utf8mb4` with an explicit collation consistent with existing tables; no `utf8`/`utf8mb3`; no reliance on `INTERPRET_UTF8_AS_UTF8MB4`; `_bin` collation for case-sensitive identifiers
- [ ] `DECIMAL` for money; `BIGINT` for counters/IDs that grow; no display widths, `ZEROFILL`, `FLOAT(M,D)`
- [ ] `DATETIME(n)` in UTC for business timestamps; no `TIMESTAMP` beyond 2038; precision matches the application
- [ ] Foreign keys as named table-level `FOREIGN KEY` clauses referencing a `PRIMARY`/`UNIQUE` key with identical column types; inline `REFERENCES` only where an enforced constraint (and a table copy on `ADD COLUMN`) is intended
- [ ] No reliance on triggers firing for FK cascades or on `enable_cascade_triggers`; no `REPLACE` on parent tables
- [ ] `VECTOR` columns have a dimension check; no HeatWave-only functions (`DISTANCE`, vector indexes)
- [ ] `CHECK` constraints for invariants, with `NOT NULL` where `NULL` must not pass; no `MD5()`/`SHA1()` in generated columns, defaults or constraints
- [ ] Every `ALTER TABLE` on a big table states `ALGORITHM` (and `LOCK`), runs with a short `lock_wait_timeout`, is a single statement per table, and is safe for the old and new application version during deploy
- [ ] No `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, `RETURNING` or other MariaDB/PostgreSQL syntax; no `CREATE MATERIALIZED VIEW` (accepted, but a plain view outside HeatWave); `CREATE VIEW IF NOT EXISTS` not used to deploy changed definitions
- [ ] Unquoted identifiers are not reserved on 9.7 (`EXTERNAL`, `LIBRARY`, `CUBE`, `QUALIFY`, `TABLESAMPLE`)
- [ ] No MyISAM/MEMORY tables written inside InnoDB transactions
- [ ] Migration scripts work with the 9.7 `mysql` client (no `source`/`\G`/`system` without `--commands`)
- [ ] No JavaScript/WASM routines or masking policies unless the deployment is Enterprise; warning 6001 treated as failure
- [ ] No 26.x features (Change Stream Applier options, PQC TLS variables, MRS component, duality-view field annotations)

##### Queries

- [ ] `EXPLAIN` (or `EXPLAIN ANALYZE` on a copy) for new queries on large tables; no full scans in hot paths
- [ ] Plan-parsing code requests `FORMAT=TRADITIONAL` or handles TREE / JSON v2
- [ ] No functions or implicit type conversions on indexed columns; strings bound for string columns
- [ ] Keyset pagination written with the expanded `OR` predicate, not `(a, b) > (?, ?)` and not deep `OFFSET`
- [ ] `NOT EXISTS` instead of `NOT IN (subquery)` on nullable columns
- [ ] Deterministic `ORDER BY` (ending in a unique column) wherever `LIMIT` or `ROW_NUMBER()` is used
- [ ] `QUALIFY` only together with an explicit hypergraph opt-in that was tested; hypergraph only on ≥ 9.7.4
- [ ] No `ONLY_FULL_GROUP_BY` workarounds (`sql_mode=''`, careless `ANY_VALUE`)
- [ ] `GROUP_CONCAT` length considered; `JSON_ARRAYAGG` for lists; `ROLLUP` results on empty input handled
- [ ] JSON functional indexes match the query expression and collation (`COLLATE utf8mb4_bin` for `->>`); multi-valued indexes only for `MEMBER OF`/`JSON_CONTAINS`/`JSON_OVERLAPS`
- [ ] Hints justified with a comment; `MAX_EXECUTION_TIME` or client timeouts on user-facing reads

##### Writes and transactions

- [ ] Upserts use `INSERT ... VALUES (...) AS new ON DUPLICATE KEY UPDATE col = new.col`; affected-rows semantics (1/2/0) handled; `AUTO_INCREMENT` gaps acceptable
- [ ] No `INSERT IGNORE` / `UPDATE IGNORE` except where ignoring every error class is intended (and not to hide multi-row subqueries, which now fail anyway)
- [ ] Read-modify-write done in one `UPDATE`, under `SELECT ... FOR UPDATE`, or with optimistic version checks
- [ ] Locking statements use an index (REPEATABLE READ locks every scanned row and gap)
- [ ] Deadlock (1213) retries the whole transaction; lock wait timeout (1205) rolls back explicitly; duality-view writes handle ETAG conflicts (6494)
- [ ] Transactions are short, contain no network calls or user waits, never contain DDL
- [ ] Large deletes/updates batched by primary key

##### Security

- [ ] All values bound as parameters; identifiers from allow-lists; `multiStatements` off; LIKE input escaped
- [ ] Application accounts with least privilege; roles activated via `SET DEFAULT ROLE`; `mandatory_roles` reviewed (active for everyone on 9.7); no wildcard surprises in database grants
- [ ] `caching_sha2_password` everywhere; no `mysql_native_password` accounts or options; TLS required and verified; no `allowPublicKeyRetrieval`/`skip-verify` in production
- [ ] No `FLUSH PRIVILEGES` in new scripts
- [ ] `local_infile` off; `secure_file_priv` set; no passwords hashed in SQL; no `AES_ENCRYPT` in ECB mode; `classic_hashing` not installed permanently
- [ ] `DEFINER`/`SQL SECURITY` reviewed for views, routines, triggers, events

##### Operations

- [ ] Server pinned to a specific 9.7.x (≥ 9.7.4 for current security and correctness fixes); image tag is `mysql:9.7` (or `lts`), not `latest`/`innovation` (those are 26.x)
- [ ] `my.cnf` contains no removed options; GTID, replication TLS and container resource limits set deliberately
- [ ] Replication statements and monitoring use `SOURCE`/`REPLICA` terminology; CDC consumers ready for SQL-layer cascade events
- [ ] Connection pool lifetime below `wait_timeout`; `parseTime`/time-zone settings consistent
- [ ] Slow query log or `performance_schema` digests reviewed; deadlocks logged (`innodb_print_all_deadlocks`)

***

[^1]: Changes in MySQL 9.7.0 (2026-04-21) — hypergraph optimizer in Community, enable_cascade_triggers, duality-view DML in Community, PBKDF2 storage format, sql_require_primary_key/GIPK with UNIQUE NOT NULL, temporal fixes (WL #16895), cpuset awareness, replica_allow_higher_version_source, components moved to Community. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-7-0.html
[^2]: MySQL 9.7 Release Notes (index of 9.0.0–9.7.4 with dates; 9.7.1 and 9.7.3 Critical Security Patch Updates; 9.7.4 four-part version). https://dev.mysql.com/doc/relnotes/mysql/9.7/en/
[^3]: MySQL 9.7 Reference Manual — MySQL Releases: Innovation and LTS (calendar versioning after 9.7, LTS support policy, upgrade paths). https://dev.mysql.com/doc/refman/9.7/en/mysql-releases.html
[^4]: endoflife.date — MySQL (9.7: Premier Support until 2031-04-30, Extended Support until 2034-04-30). https://endoflife.date/mysql
[^5]: MySQL 26.10 Release Notes (26.7.0 2026-07-28, 26.7.1 2026-08-18, 26.10.0 2026-10-20). https://dev.mysql.com/doc/relnotes/mysql/26.10/en/
[^6]: MySQL 8.4 Release Notes (8.4.13, 2026-10-20) and MySQL 8.0 Release Notes (8.0.46, 2026-04-21). https://dev.mysql.com/doc/relnotes/mysql/8.4/en/ , https://dev.mysql.com/doc/relnotes/mysql/8.0/en/
[^7]: MySQL 9.7 Reference Manual — What Is New in MySQL 9.7; Server and Status Variables and Options Added, Deprecated, or Removed in MySQL 9.7. https://dev.mysql.com/doc/refman/9.7/en/mysql-nutshell.html , https://dev.mysql.com/doc/refman/9.7/en/added-deprecated-removed.html
[^8]: Changes in MySQL 9.0.0 (2024-07-01) — VECTOR, inline REFERENCES enforced, mysql_native_password removed, EXPLAIN ANALYZE INTO, prepared event statements, IGNORE and ER_SUBQUERY_NO_1_ROW, mixed-engine transactions deprecated, JavaScript programs. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-0-0.html
[^9]: Changes in MySQL 9.4.0 (2025-07-22) — JSON duality views, MD5()/SHA1() deprecated, mysql --commands, temptable_use_mmap removed, innodb_change_buffer_max_size, collation derivation change, RANGE frame with nondeterministic ORDER BY rejected, WASM libraries. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-4-0.html
[^10]: Changes in MySQL 9.6.0 (2026-01-20) — SQL-layer foreign keys (innodb_native_foreign_keys), MD5/SHA1 moved to the classic_hashing component, GROUPING() without ROLLUP, temporary account locks table, container_aware startup option. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-6-0.html
[^11]: MySQL 9.7 Reference Manual — Keywords and Reserved Words (new keywords compared to 8.4: EXTERNAL (R), LIBRARY (R), DUALITY, MASKING, POLICY, RELATIONAL, VECTOR, ...). https://dev.mysql.com/doc/refman/9.7/en/keywords.html
[^12]: Changes in MySQL 9.5.0 (2025-10-21) — explain_format/explain_json_format_version, gtid_mode, innodb_change_buffering, caching_sha2_password_digest_rounds, activate_mandatory_roles, INTERPRET_UTF8_AS_UTF8MB4, SOURCE_SSL, binary-protocol temporal validation, removed variables and plugins. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-5-0.html
[^13]: MySQL 9.7 Reference Manual — mysql Client Options (--commands default FALSE; disabled commands; --system-command). https://dev.mysql.com/doc/refman/9.7/en/mysql-command-options.html
[^14]: Changes in MySQL 9.1.0 (2024-10-15) — CREATE VIEW IF NOT EXISTS, 255 row versions, derived-table column references, ROLLUP/SUM(DISTINCT) fixes, table_open_cache_triggers, GROUP BY ROLLUP (...) syntax, mysql --system-command. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-1-0.html
[^15]: Changes in MySQL 9.2.0 (2025-01-21) — CREATE LIBRARY, CREATE_SPATIAL_REFERENCE_SYSTEM, FLUSH PRIVILEGES deprecated, BINLOG label, subquery OFFSET fix, outer references in window clauses rejected. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-2-0.html
[^16]: Changes in MySQL 9.3.0 (2025-04-15) — removal of innodb_log_file_size, innodb_log_files_in_group, innodb_undo_tablespaces and Version Tokens; replica_parallel_workers minimum 1; --check-table-functions; container-aware InnoDB defaults; no Innovation downgrades. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-3-0.html
[^17]: MySQL 9.7 Reference Manual — Caching SHA-2 Pluggable Authentication (storage formats CRYPT5/PBKDF2). https://dev.mysql.com/doc/refman/9.7/en/caching-sha2-pluggable-authentication.html
[^18]: MySQL 9.7 Reference Manual — Binary Logging Options and Variables (binlog_transaction_dependency_history_size default 10000000). https://dev.mysql.com/doc/refman/9.7/en/replication-options-binary-log.html
[^19]: Changes in MySQL 9.7.2 (2026-07-28) — AUTO_INCREMENT column addition fix, optimizer wrong-result fixes, hypergraph fixes, JSON_VALUE fix, deprecated Group Replication variables. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-7-2.html
[^20]: Changes in MySQL 9.7.4 (2026-10-20) — VERSION_WITH_UPDATE(), EXCHANGE PARTITION CHECK validation, relaxed row-size validation, hypergraph UPDATE/DELETE wrong-rows fix, index_merge_union=off fix. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-7-4.html
[^21]: Changes in MySQL 26.7.0 (2026-07-28) and 26.10.0 (2026-10-20) — calendar versioning, Change Stream Applier, PQC TLS, MySQL REST Service component, Health Monitor, duality-view annotations, optimizer_switch parsing. https://dev.mysql.com/doc/relnotes/mysql/26.10/en/news-26-7-0.html , https://dev.mysql.com/doc/relnotes/mysql/26.10/en/news-26-10-0.html
[^22]: MySQL 9.7 Reference Manual — Server System Variables (enable_cascade_triggers: added and deprecated in 9.7.0). https://dev.mysql.com/doc/refman/9.7/en/added-deprecated-removed.html
[^23]: MySQL 9.7 Reference Manual — The VECTOR Type; Vector Functions. https://dev.mysql.com/doc/refman/9.7/en/vector.html
[^24]: MySQL 9.7 Reference Manual — Unicode Character Sets (PAD SPACE vs NO PAD). https://dev.mysql.com/doc/refman/9.7/en/charset-unicode-sets.html
[^25]: MySQL 9.7 Reference Manual — Server SQL Modes. https://dev.mysql.com/doc/refman/9.7/en/sql-mode.html
[^26]: MySQL 9.7 Reference Manual — MySQL Server Time Zone Support. https://dev.mysql.com/doc/refman/9.7/en/time-zone-support.html
[^27]: MySQL 9.7 Reference Manual — CREATE INDEX Statement (functional key parts, multi-valued indexes). https://dev.mysql.com/doc/refman/9.7/en/create-index.html
[^28]: MySQL 9.7 Reference Manual — Optimizer Statistics (histograms). https://dev.mysql.com/doc/refman/9.7/en/optimizer-statistics.html
[^29]: MySQL 9.7 Reference Manual — EXPLAIN Statement (formats, EXPLAIN ANALYZE, INTO). https://dev.mysql.com/doc/refman/9.7/en/explain.html
[^30]: MySQL 9.7 Reference Manual — Row Constructor Expression Optimization. https://dev.mysql.com/doc/refman/9.7/en/row-constructor-optimization.html
[^31]: MySQL 9.7 Reference Manual — Optimizer Hints. https://dev.mysql.com/doc/refman/9.7/en/optimizer-hints.html
[^32]: MySQL 9.7 Reference Manual — INSERT ... ON DUPLICATE KEY UPDATE Statement. https://dev.mysql.com/doc/refman/9.7/en/insert-on-duplicate.html
[^33]: MySQL 9.7 Reference Manual — Consistent Nonlocking Reads. https://dev.mysql.com/doc/refman/9.7/en/innodb-consistent-read.html
[^34]: MySQL 9.7 Reference Manual — Locks Set by Different SQL Statements in InnoDB. https://dev.mysql.com/doc/refman/9.7/en/innodb-locks-set.html
[^35]: MySQL 9.7 Reference Manual — Online DDL Operations. https://dev.mysql.com/doc/refman/9.7/en/innodb-online-ddl-operations.html
[^36]: MySQL 9.7 Reference Manual — Metadata Locking. https://dev.mysql.com/doc/refman/9.7/en/metadata-locking.html
[^37]: MySQL 9.7 Reference Manual — Stored Program Binary Logging. https://dev.mysql.com/doc/refman/9.7/en/stored-programs-logging.html
[^38]: MySQL 9.7 Reference Manual — JavaScript Stored Programs / Multilingual Engine Component (MLE), MySQL Enterprise Edition. https://dev.mysql.com/doc/refman/9.7/en/stored-routines-js.html
[^39]: MySQL 9.7 Reference Manual — JSON Duality Views (CREATE JSON DUALITY VIEW, DML, ETAG concurrency). https://dev.mysql.com/doc/refman/9.7/en/json-duality-views.html
[^40]: go-sql-driver/mysql — README (DSN parameters parseTime, loc, multiStatements, interpolateParams, clientFoundRows). https://github.com/go-sql-driver/mysql
[^41]: MySQL 9.7 Reference Manual — Using Roles (mandatory roles, activate_all_roles_on_login, activate_mandatory_roles). https://dev.mysql.com/doc/refman/9.7/en/roles.html
[^42]: MySQL 9.7 Reference Manual — GRANT Statement (wildcards in database names). https://dev.mysql.com/doc/refman/9.7/en/grant.html
[^43]: MySQL 9.7 Reference Manual — Upgrade Paths. https://dev.mysql.com/doc/refman/9.7/en/upgrade-paths.html
[^44]: MySQL 9.7 Reference Manual — The sys Schema. https://dev.mysql.com/doc/refman/9.7/en/sys-schema.html
[^45]: MySQL 9.7 Reference Manual — Legacy Hashing Component (component_classic_hashing; generated columns using MD5()/SHA1() stop working after upgrading to 9.6+). https://dev.mysql.com/doc/refman/9.7/en/legacy-hashing-component.html
[^46]: MySQL 9.7 Reference Manual — FOREIGN KEY Constraints (SQL-layer handling, `innodb_native_foreign_keys`, AUTO_INCREMENT on rejected inserts, 15-level cascade limit, REPLACE and cascade triggers) and InnoDB Startup Options (`innodb_native_foreign_keys`: deprecated). https://dev.mysql.com/doc/refman/9.7/en/create-table-foreign-keys.html , https://dev.mysql.com/doc/refman/9.7/en/innodb-parameters.html
