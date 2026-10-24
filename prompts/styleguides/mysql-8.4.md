### MySQL 8.4 LTS: The Complete SQL Review Guide

#### Overview

> **Version**: MySQL 8.4 LTS. 8.4.0 became generally available on **April 30, 2024**. The latest release is **8.4.13 (October 20, 2026)**; 8.4.12 (August 18, 2026) was a Critical Security Patch Update for the Docker image only.
> **Support**: 8.4 is a Long-Term Support series under the Oracle Lifetime Support Policy: five years of Premier Support and three years of Extended Support, i.e. Premier Support until **April 2029** and Extended Support until **April 2032**.
> **Landscape as of October 2026**: MySQL 8.0 reached **End of Life in April 2026** with 8.0.46 (April 21, 2026). The next LTS, **9.7**, shipped on April 21, 2026 (latest 9.7.4, October 20, 2026). Innovation releases now use calendar version numbers: **26.7.0** (July 28, 2026) and **26.10.0** (October 20, 2026) are the newest Innovation releases. Supported upgrade path: 8.4 LTS → 9.7 LTS; an LTS series cannot be skipped.
> **Scope**: SQL that runs on MySQL 8.4 with InnoDB: schema and index design, migrations and online DDL, queries and DML in application code, transactions and locking, stored programs, security, and what changed relative to 8.0. Every example in this guide was executed on MySQL 8.4.13 (Community, official `mysql:8.4` image); boundary claims were checked on 8.0.46 and 9.7.4.
> **Note**: This document is self-contained. Sections that cover behaviour specific to the 8.1–8.4 line are marked **New in 8.4** (or with the exact 8.4.x patch).

This guide is also the one to apply when a project pins an **Innovation release between 8.1 and 9.6** (for example `mysql:8.3`, `mysql:9.2`). All of those releases are out of support (an Innovation release is supported only until the next one ships). Review such code against 8.4 LTS semantics, recommend moving to an LTS (8.4 or 9.7), and **treat every feature introduced in 9.0–9.7 or 26.x as unavailable unless the project pins 9.7 LTS or later** (see "Do not use or recommend"). Where a 9.x Innovation release already behaves differently from 8.4 in a way that breaks code, this guide says so.

Global truths about MySQL 8.4 with InnoDB:

- **The table is the primary key.** InnoDB stores rows in a B-tree clustered on the primary key; every secondary index entry carries the full primary key. A wide or random primary key makes every index bigger and every insert slower.
- **REPEATABLE READ is the default isolation level.** Plain `SELECT`s read a consistent snapshot; `UPDATE`, `DELETE` and locking reads (`FOR UPDATE`/`FOR SHARE`) read the latest committed version and take next-key (record + gap) locks on every index record they scan.
- **DDL is not transactional.** Every DDL statement implicitly commits the current transaction. A single DDL statement is atomic (crash-safe), but a migration of several statements is not.
- **Strict mode is on by default.** `sql_mode` is `ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION`. `INSERT IGNORE`, `UPDATE IGNORE` and a session that clears `sql_mode` silently turn errors back into truncation warnings.
- **`utf8mb4` / `utf8mb4_0900_ai_ci` is the default.** That collation is accent-insensitive, case-insensitive and `NO PAD`. `utf8` still means the deprecated 3-byte `utf8mb3`.
- **`mysql_native_password` is disabled.** New accounts use `caching_sha2_password`; clients must connect over TLS or use RSA key exchange for the first authentication.
- **No `RETURNING`, no `MERGE`, no `FULL OUTER JOIN`, no `IS DISTINCT FROM`, no `ADD COLUMN IF NOT EXISTS`, no partial or `INCLUDE` indexes, no transactional DDL.** Code or advice copied from PostgreSQL or MariaDB does not run.

***

#### What's New in MySQL 8.4 (since 8.0)

MySQL 8.4.0 is the first LTS after 8.0 and bundles everything from the 8.1, 8.2 and 8.3 Innovation releases. LTS rules: features are added and removed only in the first LTS release (8.4.0); later 8.4.x releases contain fixes, and in-place upgrade and downgrade within 8.4.x is supported. In practice a few 8.4.x patches did change SQL-visible behaviour; they are listed separately below.

##### Breaking changes for application SQL and migrations [New in 8.4]

| Change | 8.0 behaviour | 8.4 behaviour |
|---|---|---|
| `mysql_native_password` | Enabled; `CREATE USER ... IDENTIFIED WITH mysql_native_password` works | Plugin is `DISABLED`; the same statement fails with `ERROR 1524: Plugin 'mysql_native_password' is not loaded`. Re-enable only with `--mysql-native-password=ON` |
| `default_authentication_plugin` | System variable exists | Removed (`ERROR 1193 Unknown system variable`); use `authentication_policy` (default `*,,`) |
| Replication SQL | `CHANGE MASTER TO`, `START/STOP/RESET SLAVE`, `SHOW SLAVE STATUS`, `SHOW SLAVE HOSTS`, `MASTER_*` options: deprecated with warning 1287. `SHOW MASTER STATUS`, `SHOW MASTER LOGS`, `RESET MASTER`, `PURGE MASTER LOGS` run on 8.0 **without** a warning (they were only deprecated in 8.2.0) | All removed: `ERROR 1064` syntax error. Use `CHANGE REPLICATION SOURCE TO`, `START/STOP/RESET REPLICA`, `SHOW REPLICA STATUS`, `SHOW BINARY LOG STATUS`, `SHOW BINARY LOGS`, `RESET BINARY LOGS AND GTIDS`, `SHOW REPLICAS`, `PURGE BINARY LOGS`, `SOURCE_*` options; `START REPLICA` no longer accepts `MASTER_LOG_FILE`/`MASTER_LOG_POS` |
| Foreign key to a non-unique key | Accepted | Rejected by default: `ERROR 6125 Failed to add the foreign key constraint. Missing unique key for constraint ...` (`restrict_fk_on_non_standard_key=ON`) |
| `AUTO_INCREMENT` on `FLOAT`/`DOUBLE` | Deprecated (warning 3856) | `ERROR 1063 Incorrect column specifier`; such tables block the upgrade |
| `FLUSH HOSTS` | Deprecated | Syntax error; use `TRUNCATE TABLE performance_schema.host_cache` |
| `LOCK TABLES ... LOW_PRIORITY WRITE` | Deprecated | Syntax error |
| Unquoted identifiers starting with `$` and containing another `$` (`$a$b`) | Deprecated | Syntax error; a single leading `$` (`$ab`) still works with deprecation warning 1681 |
| Reserved words | — | `QUALIFY` and `TABLESAMPLE` are new **reserved** words (unquoted use as an identifier is `ERROR 1064`); `AUTO`, `BERNOULLI`, `GTIDS`, `LOG`, `MANUAL`, `PARALLEL`, `PARSE_TREE`, `S3` are new non-reserved keywords (`MANUAL` and `PARALLEL` were reserved in 8.4.0–8.4.10) |
| Server plugins and TLS | `keyring_file`, `keyring_encrypted_file`, `keyring_oci`, `authentication_fido`, `openssl_udf` available; weak TLS ciphers accepted | Plugins removed (use the keyring components and `authentication_webauthn`); `ssl_cipher`, `tls_ciphersuites` and their `admin_` variants reject ciphers without forward secrecy, SHA-2 or AEAD; startup options can no longer be set to `NULL` |
| `INFORMATION_SCHEMA.TABLESPACES` | Exists (deprecated, warning 1681) | Removed (`ERROR 1109 Unknown table`); use `INNODB_TABLESPACES` / `INNODB_DATAFILES`. The `ENGINE` clause of `DROP/ALTER TABLESPACE` is removed as well |
| Server options | `--ssl`, `--admin-ssl`, `have_ssl`, `have_openssl`, `--old`, `--new`, `--language`, `--skip-host-cache`, `--innodb`/`--skip-innodb`, `--no-dd-upgrade`, `expire_logs_days`, `binlog_transaction_dependency_tracking`, `avoid_temporal_upgrade`, `show_old_temporals`, `log_bin_use_v1_events`, `master_info_repository`/`relay_log_info_repository`, `--master-info-file`/`--relay-log-info-file`, `--slave-rows-search-algorithms`, `transaction_write_set_extraction`, `group_replication_ip_whitelist`, `group_replication_primary_member`, `group_replication_recovery_complete_at`, `--abort-slave-event-count`, `--disconnect-slave-event-count`, `--character-set-client-handshake`, `--old-style-user-limits` | Removed; a config file that still sets them stops the server from starting |
| Utilities | `mysql_upgrade`, `mysqlpump`, `mysql_ssl_rsa_setup` | Removed; the server upgrades itself at startup and generates missing TLS/RSA files |
| Privileges | `RELOAD` covers `FLUSH PRIVILEGES`; `SET_USER_ID` | New `FLUSH_PRIVILEGES`, `OPTIMIZE_LOCAL_TABLE`, `TRANSACTION_GTID_TAG`, `SET_ANY_DEFINER`, `ALLOW_NONEXISTENT_DEFINER`; `SET_USER_ID` removed |

```sql
-- MySQL 8.4.13 (on 8.0.46 the failing statements below succeed: SHOW SLAVE STATUS and FLUSH HOSTS
-- with warning 1287, SHOW MASTER STATUS without any warning; SHOW BINARY LOG STATUS fails there)
CREATE USER legacy@'%' IDENTIFIED WITH mysql_native_password BY 'x';
-- ERROR 1524 (HY000): Plugin 'mysql_native_password' is not loaded
SELECT @@default_authentication_plugin;
-- ERROR 1193 (HY000): Unknown system variable 'default_authentication_plugin'
SHOW SLAVE STATUS;          -- ERROR 1064
SHOW MASTER STATUS;         -- ERROR 1064
SHOW BINARY LOG STATUS;     -- works (8.2+); ERROR 1064 on 8.0
FLUSH HOSTS;                -- ERROR 1064

CREATE TABLE p (id INT PRIMARY KEY, code INT, KEY (code));
CREATE TABLE c (id INT PRIMARY KEY, pcode INT,
                FOREIGN KEY (pcode) REFERENCES p (code));
-- ERROR 6125 (HY000): Failed to add the foreign key constraint.
--   Missing unique key for constraint 'c_ibfk_1' in the referenced table 'p'

CREATE TABLE qualify (id INT);       -- ERROR 1064 (reserved since 8.4)
CREATE TABLE `qualify` (id INT);     -- OK when quoted
```

##### New SQL features in 8.1–8.4 [New in 8.4]

- **`EXPLAIN FORMAT=JSON INTO @var`** stores the plan in a user variable; `INTO` requires an explicit `FORMAT=JSON`.
- **`EXPLAIN ... FOR SCHEMA db`** (synonym `FOR DATABASE`) explains a statement as if it ran in another schema.
- **`explain_json_format_version`** (1 = default, 2 = access-path based). With version 2, `EXPLAIN ANALYZE FORMAT=JSON` works on 8.4.13 (on 8.0 it fails with `ERROR 1235 ... 'EXPLAIN ANALYZE with JSON format'`); the 8.4 manual still says JSON is never accepted with `EXPLAIN ANALYZE`, so verify on the pinned patch before relying on it.
- **Automatic histogram updates**: `ANALYZE TABLE t UPDATE HISTOGRAM ON col WITH n BUCKETS AUTO UPDATE` (default `MANUAL UPDATE`).
- **Hash-based `INTERSECT` / `EXCEPT`** (`hash_set_operations` optimizer switch, `set_operations_buffer_size`). `INTERSECT` and `EXCEPT` themselves exist since 8.0.31.
- **Correlated scalar subqueries** can be transformed into derived tables (`subquery_to_derived`, off by default).
- **`GROUP BY ROLLUP (a, b)`**: alternative spelling of `GROUP BY a, b WITH ROLLUP`. The release notes announce it in 8.4.3 (WL #15843; 9.1.0 on the Innovation track), but the parser already accepts it in 8.4.0 and 8.4.2 with identical results (tested); it is a syntax error on 8.0.46. Treat it as available on every 8.4.x.
- **Tagged GTIDs** (`SET gtid_next = 'AUTOMATIC:tag'`), `SHOW BINARY LOG STATUS`, `SHOW REPLICAS`.
- **`VERSION_WITH_UPDATE()`** and `@@version_with_update` return a four-part version (`8.4.13.0`) **since 8.4.13**; `VERSION()` keeps returning three parts.

```sql
-- MySQL 8.4.13
EXPLAIN FORMAT=JSON INTO @plan SELECT * FROM users WHERE id = 1;
SELECT JSON_EXTRACT(@plan, '$.query_block.table.access_type');   -- "const"

SELECT y, c, SUM(amt) FROM sales GROUP BY ROLLUP (y, c);          -- 8.4.x; ERROR 1064 on 8.0
ANALYZE TABLE orders UPDATE HISTOGRAM ON total WITH 32 BUCKETS AUTO UPDATE;
SELECT VERSION(), VERSION_WITH_UPDATE();                          -- 8.4.13 | 8.4.13.0
```

##### Changed server defaults [New in 8.4]

These do not change SQL semantics but explain performance differences after an 8.0 → 8.4 upgrade. The first ten rows were observed on 8.0.46 vs 8.4.13 in the same 4-CPU container with a 128 MiB buffer pool; the rows from `innodb_doublewrite_files` down are taken from the "What Is New" page (several computed defaults coincide on a small container):

| Variable | 8.0 | 8.4 |
|---|---|---|
| `innodb_adaptive_hash_index` | `ON` | `OFF` |
| `innodb_change_buffering` | `all` | `none` |
| `innodb_io_capacity` / `innodb_io_capacity_max` | 200 / 2000 | 10000 / 20000 |
| `innodb_log_buffer_size` | 16 MiB | 64 MiB |
| `innodb_flush_method` (Linux) | `fsync` | `O_DIRECT` |
| `innodb_use_fdatasync` | `OFF` | `ON` |
| `innodb_doublewrite_pages` | 4 | 128 |
| `innodb_buffer_pool_in_core_file` | `ON` | `OFF` (when `MADV_DONTDUMP` is supported) |
| `innodb_purge_threads` | 4 | 1 on ≤ 16 logical CPUs, else 4 |
| `temptable_max_ram` / `temptable_max_mmap` / `temptable_use_mmap` | 1 GiB / 1 GiB / `ON` | 3% of RAM (1–4 GiB) / 0 / `OFF` |
| `innodb_doublewrite_files` | `innodb_buffer_pool_instances` × 2 | 2 |
| `innodb_buffer_pool_instances` | 8 (1 if the pool is < 1 GiB) | computed from pool size and CPUs (1 if the pool is ≤ 1 GiB) |
| `innodb_page_cleaners` | `innodb_buffer_pool_instances` | 4 (capped at the number of buffer pool instances) |
| `innodb_parallel_read_threads` / `innodb_read_io_threads` | 4 / 4 | logical CPUs ÷ 8 / logical CPUs ÷ 2, minimum 4 each |
| `innodb_numa_interleave` | `OFF` | `ON` (NUMA-enabled builds only) |
| `group_replication_consistency` | `EVENTUAL` | `BEFORE_ON_PRIMARY_FAILOVER` |
| `group_replication_exit_state_action` | `READ_ONLY` | `OFFLINE_MODE` |
| `SOURCE_RETRY_COUNT` | 86400 | 10 |

Unchanged and still relevant for review: `transaction_isolation=REPEATABLE-READ`, `binlog_format=ROW`, `innodb_autoinc_lock_mode=2`, `innodb_lock_wait_timeout=50`, `lock_wait_timeout=31536000` (one year), `explicit_defaults_for_timestamp=ON`, `local_infile=OFF`, `activate_all_roles_on_login=OFF`, `group_concat_max_len=1024`, `cte_max_recursion_depth=1000`, `block_encryption_mode=aes-128-ecb`, `explain_format=TRADITIONAL`, `gtid_mode=OFF`.

##### Behaviour changes inside the 8.4 LTS series

An LTS is meant to be stable, but these patch releases changed behaviour a reviewer can observe:

| Release | Change |
|---|---|
| 8.4.3 | `GROUP BY ROLLUP (...)` documented (already accepted by 8.4.0–8.4.2 in tests); `mysql` client gains `--system-command` (on by default) |
| 8.4.4 | `ALTER TABLE ... ADD/DROP COLUMN` **without an `ALGORITHM` clause** on an **empty** table defaults to `INPLACE` instead of `INSTANT` (no row version is consumed; 8.4.2 still consumed one); an explicit `ALGORITHM=INSTANT` still consumes a version; incompatible fix for spatial-index corruption — drop and re-create spatial indexes around the upgrade |
| 8.4.5 | New `--check-table-functions=WARN|ABORT` (default `ABORT`): an upgrade aborts if a constraint, default, partition expression or generated column uses a function that now raises an error |
| 8.4.6 | `mysql` client `--commands` option (on by default) |
| 8.4.11 | `MANUAL` and `PARALLEL` become **non-reserved** (on 8.4.2, `CREATE TABLE manual (...)` is `ERROR 1064`); `INFORMATION_SCHEMA.KEYWORDS` is corrected (8.4.2 reports `QUALIFY`, `CUBE`, `MANUAL`, `PARALLEL` as non-reserved although the parser rejects all four unquoted). The notes also list `EXTERNAL` as now reserved, but on 8.4.13 `external` is not even a keyword and works unquoted; it **is** reserved on 9.7.4, so quote it anyway. Some `CREATE`/`ALTER TABLE` statements that exceed documented InnoDB row-size limits are now rejected (`ALGORITHM=INSTANT` uses the stricter calculation) |
| 8.4.13 | Four-part version (`VERSION_WITH_UPDATE()`); `ALTER TABLE ... EXCHANGE PARTITION` now validates rows against the destination table's `CHECK` constraints in both directions (`ERROR 3819`), even `WITHOUT VALIDATION` (8.4.2 silently moved a `-5` into a table with `CHECK (a > 0)`); fix for `index_merge_union=off` plans that could drop a disjunct and return incomplete results |

Run the latest 8.4.x: 8.4.13 alone fixes an optimizer bug that returned incomplete results and hardens protocol parsing against several security issues.

##### Version-aware review

- Determine the exact server version from the project (Docker image tag, `docker-compose.yml`, Helm values, Terraform `engine_version`, CI service containers, `mysql --version` in scripts). `mysql:8.4` floats to the newest 8.4.x; a pinned older patch such as `mysql:8.4.2` lacks `VERSION_WITH_UPDATE()`, the 8.4.4 empty-table `INPLACE` default, the 8.4.11 keyword fixes (`MANUAL`/`PARALLEL` are still reserved) and the 8.4.13 `EXCHANGE PARTITION` and optimizer fixes.
- Before claiming that syntax or a function is unavailable, check it against the 8.4 manual or the release notes of the pinned patch. Say "not available in MySQL 8.4" only when the pinned version proves it; many PostgreSQL-isms are genuinely missing, but `INTERSECT`, `EXCEPT`, `LATERAL`, `JSON_TABLE`, `VALUES ROW(...)`, window functions, recursive CTEs, `CHECK` constraints, functional and descending indexes all exist.
- Managed services lag or diverge: Amazon RDS / Aurora, Azure, Google Cloud SQL and HeatWave each document their own 8.4.x versions and parameter defaults. HeatWave-only features (secondary engines, `QUALIFY` with the hypergraph optimizer, `TABLESAMPLE`) are not available on Community or Enterprise Server.
- MariaDB is not MySQL. `RETURNING`, `CREATE OR REPLACE TABLE`, `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, sequences and system-versioned tables are MariaDB features; on MySQL 8.4 they are syntax errors.
- When a project pins a 9.x Innovation release, also check the "already different in 9.x" notes in the next section before approving code that relies on 8.4 behaviour.

##### Do not use or recommend (features newer than 8.4)

The server does not have these. Never suggest them for code that runs on 8.4 (or on an Innovation release older than the one that introduced them), and flag them as bugs when they appear:

| Feature | Introduced | On 8.4.13 |
|---|---|---|
| `VECTOR(n)` type, `STRING_TO_VECTOR()`/`TO_VECTOR()`, `VECTOR_TO_STRING()`/`FROM_VECTOR()`, `VECTOR_DIM()` | 9.0.0 | `ERROR 1064` / `ERROR 1305 FUNCTION ... does not exist` |
| `EXPLAIN ANALYZE FORMAT=JSON INTO @var` | 9.0.0 | `ERROR 1064` |
| `PREPARE` of `CREATE/ALTER/DROP EVENT` | 9.0.0 | `ERROR 1295 This command is not supported in the prepared statement protocol yet` |
| JavaScript stored programs (`LANGUAGE JAVASCRIPT`), `CREATE LIBRARY`, `USING (lib)` | 9.0.0 / 9.2.0 (Enterprise) | not available |
| `CREATE VIEW IF NOT EXISTS` | 9.1.0 | `ERROR 1064`; use `CREATE OR REPLACE VIEW` |
| `CREATE_SPATIAL_REFERENCE_SYSTEM` privilege | 9.2.0 | use `SUPER` |
| `CREATE JSON DUALITY VIEW`, `JSON_DUALITY_OBJECT()`, `ETAG()`, DML on duality views | 9.4.0 (DML in Community: 9.7.0) | `ERROR 1064` |
| `activate_mandatory_roles` variable | 9.5.0 | `ERROR 1193` |
| `INTERPRET_UTF8_AS_UTF8MB4` SQL mode | 9.5.0 | `ERROR 1231` |
| `GROUPING()` in a query without `ROLLUP` | 9.6.0 | `ERROR 1111 Invalid use of group function` |
| `innodb_native_foreign_keys`, SQL-layer foreign keys | 9.6.0 | not available |
| Hypergraph optimizer in Community (`optimizer_switch='hypergraph_optimizer=on'`) and therefore `QUALIFY` | 9.7.0 | `ERROR 3999 ... does not yet support 'use in non-debug builds'`; `QUALIFY` → `ERROR 6037` |
| `enable_cascade_triggers` (FK cascades fire triggers) | 9.7.0 | `ERROR 1193` |
| `CURRENT_ROLE_IN()`, `CURRENT_USER_IN()`, `CREATE MASKING POLICY` | 9.7.0 (Enterprise) | not available |
| `caching_sha2_password` PBKDF2 storage format | 9.7.0 | not available |
| `replica_allow_higher_version_source` | 9.7.0 | not available |

Not available on 9.7.4 either (verified on 8.4.13 and 9.7.4): `INSERT ... RETURNING`, `IS [NOT] DISTINCT FROM` (use `<=>`), `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, range access for row-constructor inequalities such as `(a, b) > (?, ?)`. Also absent on 8.4.13: `FULL OUTER JOIN`, `CREATE INDEX IF NOT EXISTS`, `DROP INDEX IF EXISTS`, `CREATE OR REPLACE TABLE`, `CREATE SEQUENCE`, `CREATE MATERIALIZED VIEW`.

**Already different in 9.x Innovation releases** (relevant when a project pins 9.0–9.6 and this guide is applied):

- 9.0.0 removes the `mysql_native_password` server plugin entirely (`--mysql-native-password=ON` is no longer an escape hatch; the client-side plugin remains). `default_authentication_plugin` was already removed in 8.4.
- New reserved words on 9.7.4 that are plain identifiers on 8.4.13: `EXTERNAL` and `LIBRARY` (`CREATE TABLE external (...)` is `ERROR 1064` on 9.7.4). Quote such identifiers in schemas that must survive the move to 9.7.
- 9.0.0 **enforces inline column `REFERENCES`** clauses. On 8.4 they are parsed and **silently ignored** (see Foreign keys). Code that "worked" on 8.4 may start failing with `ERROR 1452` on 9.x.
- 9.0.0: `IGNORE` no longer suppresses `ER_SUBQUERY_NO_1_ROW`.
- 9.1.0: column references through a derived table qualified with the schema (`SELECT db.dt.a FROM (...) AS dt`) are rejected; `ROLLUP` over an empty table returns a grand-total row `(NULL, 0)` (8.4 returns an empty set); `SUM(DISTINCT b'1100')` returns 12 (8.4.13 returns 9).
- 9.2.0: `SELECT (SELECT 1 LIMIT 1 OFFSET 10)` returns `NULL` (8.4.13 returns 1); `FLUSH PRIVILEGES` is deprecated; `BETWEEN` string literals must match the operand's collation; outer references in window `PARTITION BY`/`ORDER BY` are rejected.
- 9.3.0 removes `innodb_log_file_size`, `innodb_log_files_in_group` and `innodb_undo_tablespaces` (still accepted, deprecated, on 8.4).
- 9.4.0 deprecates `MD5()` and `SHA1()`; 9.6.0 moves them into the `classic_hashing` component (on 9.7.4 without that component: `ERROR 1305 FUNCTION MD5 does not exist`).
- 9.5.0 changes defaults: `explain_format=TREE`, `explain_json_format_version=2`, `gtid_mode=ON`, `innodb_change_buffering=all`, `caching_sha2_password_digest_rounds=10000`.

##### Deprecated in 8.4 (works, but flag it)

These are deprecated (most print a warning on 8.4.13); code that depends on them breaks on a later release:

- `VALUES(col)` in `ON DUPLICATE KEY UPDATE` (warning 1287, since 8.0.20) → row alias `INSERT ... VALUES (...) AS new ON DUPLICATE KEY UPDATE c = new.c`.
- `SQL_CALC_FOUND_ROWS` / `FOUND_ROWS()` (warning 1287) → separate `COUNT(*)`.
- `&&`, `||` as `OR`, `!` as `NOT` (warning 1287) → `AND`, `OR`, `NOT`.
- Assigning user variables inside expressions (`SELECT @y := 2`, warning 1287) → `SET` or `SELECT ... INTO`.
- Integer display width (`INT(11)`), `ZEROFILL`, `FLOAT(M,D)`/`DOUBLE(M,D)` (warning 1681).
- The `utf8` / `utf8mb3` character set (warning 3719) → `utf8mb4`.
- `$` as the first character of an unquoted identifier (warning 1681).
- Wildcards `%` and `_` in database names of `GRANT` statements (deprecated in 8.2.0 and, on the 8.0 line, 8.0.35, both released 2023-10-25; no warning is printed on 8.4.13).
- `mysql_native_password` (deprecated and disabled), `innodb_log_file_size`/`innodb_log_files_in_group` (use `innodb_redo_log_capacity`).
- The `binlog_format` variable itself (warning 1287 on every read or `SET`, deprecated since 8.0.34); writeset-based dependency tracking, which 8.4 always uses, requires `ROW`, and `MIXED` is no longer supported for it.
- `INFORMATION_SCHEMA.PROCESSLIST` (warning 1287) → `performance_schema.processlist`.
- `SET restrict_fk_on_non_standard_key = OFF` (warning 4166) — foreign keys on non-unique or partial keys.
- `%` as a synonym for `localhost` in privilege checks (deprecated since 8.2.0); `WAIT_UNTIL_SQL_THREAD_AFTER_GTIDS()` (use `WAIT_FOR_EXECUTED_GTID_SET()`); `group_replication_allow_local_lower_version_join`; `CREATE/ALTER EVENT ... DISABLE ON SLAVE` (warning 1287, use `DISABLE ON REPLICA`).

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

- **BAD**: `CHAR(36)` UUID primary keys. 36 bytes (up to 144 bytes of index space in `utf8mb4`) repeated in every secondary index, random insert positions, page splits.
- **GOOD**: if UUIDs are required, store them as `BINARY(16)`. For time-based version-1 UUIDs from `UUID()`, `UUID_TO_BIN(u, 1)` swaps the time fields so values become roughly monotonic. MySQL 8.4 has no built-in UUIDv7 generator; generate v7 in the application if you want time-ordered random IDs.

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

A table without a primary key gets a hidden 6-byte row ID, cannot be replicated efficiently with row-based replication and is rejected by many managed services. Two server settings help:

```sql
SET SESSION sql_require_primary_key = ON;
CREATE TABLE nopk2 (a INT);
-- ERROR 3750 (HY000): Unable to create or change a table without a primary key ...

SET SESSION sql_generate_invisible_primary_key = ON;
CREATE TABLE nopk (a INT);
SHOW CREATE TABLE nopk;
-- `my_row_id` bigint unsigned NOT NULL AUTO_INCREMENT /*!80023 INVISIBLE */, ... PRIMARY KEY (`my_row_id`)
SELECT * FROM nopk;         -- my_row_id is not returned by SELECT *
```

Flag migrations that create tables without a primary key. On 8.4, `sql_require_primary_key` does not accept a `UNIQUE NOT NULL` key as a substitute (9.7.0 relaxes this).

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

##### Constraints: NOT NULL, CHECK, defaults

`CHECK` constraints are enforced since 8.0.16. A `CHECK` that evaluates to `NULL` passes (add `NOT NULL` separately), `INSERT IGNORE` turns a violation into a warning and silently skips the row, and nondeterministic functions are rejected:

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

Since 8.4.13, `ALTER TABLE ... EXCHANGE PARTITION` checks the swapped rows against the destination's `CHECK` constraints even with `WITHOUT VALIDATION`.

##### Foreign keys

InnoDB enforces foreign keys, with these 8.4-specific traps:

1. **Inline `REFERENCES` on a column definition is silently ignored.** Only a table-level `FOREIGN KEY (...) REFERENCES ...` clause (or `CONSTRAINT name FOREIGN KEY ...`) creates a constraint. MySQL 9.0 started enforcing the inline form, so the same DDL behaves differently on 8.4 and 9.x.

```sql
-- BAD on 8.4: no constraint is created, no warning is raised
CREATE TABLE person (id INT PRIMARY KEY);
CREATE TABLE pet (id INT PRIMARY KEY, owner INT REFERENCES person (id));
INSERT INTO pet VALUES (1, 999);         -- succeeds on 8.4.13 (ERROR 1452 on 9.7.4)
SELECT COUNT(*) FROM information_schema.referential_constraints
WHERE constraint_schema = DATABASE() AND table_name = 'pet';    -- 0

-- GOOD: table-level constraint, enforced on every version
CREATE TABLE pet2 (
  id INT PRIMARY KEY,
  owner INT NOT NULL,
  CONSTRAINT fk_pet2_owner FOREIGN KEY (owner) REFERENCES person (id)
);
```

2. **The referenced columns must form a `PRIMARY KEY` or `UNIQUE` key** (8.4 default `restrict_fk_on_non_standard_key=ON`; see the table above). Setting it to `OFF` is deprecated and only for upgrading legacy schemas.
3. **Cascaded changes do not fire triggers.** `ON DELETE CASCADE` / `ON UPDATE CASCADE` / `SET NULL` changes child rows without running the child table's triggers; audit triggers on child tables miss them (9.7.0 adds an opt-in `enable_cascade_triggers`).

```sql
CREATE TABLE audit (id INT AUTO_INCREMENT PRIMARY KEY, what VARCHAR(50));
CREATE TABLE p2 (id INT PRIMARY KEY);
CREATE TABLE c2 (id INT PRIMARY KEY, p INT,
                 FOREIGN KEY (p) REFERENCES p2 (id) ON DELETE CASCADE);
CREATE TRIGGER c2_ad AFTER DELETE ON c2 FOR EACH ROW
  INSERT INTO audit (what) VALUES (CONCAT('deleted child ', OLD.id));
INSERT INTO p2 VALUES (1);
INSERT INTO c2 VALUES (10, 1), (11, 1);
DELETE FROM p2 WHERE id = 1;
SELECT (SELECT COUNT(*) FROM c2) AS children, (SELECT COUNT(*) FROM audit) AS audit_rows;
-- 0 | 0   -- children deleted, trigger never ran
```

4. **`REPLACE` deletes and re-inserts**, so it fires `ON DELETE CASCADE` on children (see Writes).
5. **`SET foreign_key_checks = 0` lets orphans in.** Adding a foreign key with checks disabled does not validate existing rows (that is why `ALGORITHM=INPLACE` is only allowed then). Run an orphan query before re-enabling.
6. Foreign keys are not supported on partitioned InnoDB tables, and the referencing columns get an index automatically if none exists.
7. Column types must be compatible: integer types need the same size and signedness, string columns the same character set and collation (lengths may differ). Otherwise: `ERROR 3780 Referencing column ... and referenced column ... are incompatible` (seen when the parent is `INT` and the child `BIGINT`).

##### Generated and invisible columns

```sql
CREATE TABLE ic (id INT PRIMARY KEY, secret VARCHAR(10) INVISIBLE);
INSERT INTO ic VALUES (1);                       -- invisible columns are skipped by positional INSERT
INSERT INTO ic (id, secret) VALUES (2, 's');
SELECT * FROM ic;                                -- returns only id
SELECT id, secret FROM ic;                       -- explicit reference works
```

Invisible columns (8.0.23+) let you add a column without breaking `SELECT *` or `INSERT ... VALUES` without a column list in old code. Generated columns are `VIRTUAL` (computed on read, can be indexed, added `INSTANT`ly) or `STORED` (materialized; adding one rebuilds the table with `ALGORITHM=COPY`).

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
```

- Flag `utf8`, `utf8mb3`, `utf8_general_ci`, `utf8_unicode_ci` in new DDL; use `utf8mb4`.
- Flag connections that do not set the character set. The `mysql` command-line client picks its character set from the OS locale; in a container with no locale it connects as `latin1` (observed: `@@character_set_connection = latin1` in the official image), and non-ASCII literals in scripts are then misinterpreted. Use `--default-character-set=utf8mb4` or `SET NAMES utf8mb4` in migration runners; drivers should set `charset=utf8mb4` / `collation=utf8mb4_0900_ai_ci`.

##### `utf8mb4_0900_ai_ci` semantics

The default collation is case-insensitive, accent-insensitive and **NO PAD** (trailing spaces are significant). Older collations (`utf8mb4_general_ci`, `utf8mb4_unicode_ci`, `utf8mb4_bin`) are `PAD SPACE`.

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

  Usernames, slugs, tokens, hashes and other identifiers that must be distinct byte-for-byte need `COLLATE utf8mb4_bin` (or `utf8mb4_0900_bin`) or a `VARBINARY` column. Email addresses are usually fine case-insensitive, but accent-insensitivity may still surprise.
- **`LOWER(col) = ...` is redundant** with a `_ci` collation and prevents index use (see Indexing).
- **Mixing collations** in a comparison or join raises an error, and a join between columns with different collations cannot use the index on the converted side:

```sql
CREATE TABLE ca (s VARCHAR(10) COLLATE utf8mb4_unicode_ci);
CREATE TABLE cb (s VARCHAR(10) COLLATE utf8mb4_0900_ai_ci);
SELECT * FROM ca JOIN cb ON ca.s = cb.s;
-- ERROR 1267 (HY000): Illegal mix of collations (utf8mb4_unicode_ci,IMPLICIT) and
--   (utf8mb4_0900_ai_ci,IMPLICIT) for operation '='
```

  Schemas migrated from 5.7 often contain `utf8mb4_general_ci` or `utf8mb4_unicode_ci` columns next to new `utf8mb4_0900_ai_ci` columns. Specify collations explicitly at database or table level in migrations so new tables match old ones.
- `ALTER TABLE ... CONVERT TO CHARACTER SET` rewrites the whole table (`ALGORITHM=COPY`) and may change index lengths and uniqueness; review it as a heavy migration.

***

#### SQL Mode and Strictness

The server's default `sql_mode` is strict:

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
- Strict mode applies to transactional tables; it does not validate values that never reach a column (e.g. `SELECT` expressions):

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

`ANY_VALUE()` is acceptable only when any value is correct. To get the row with the max/min per group, use a window function or a join (see Query patterns). `GROUP BY` does not sort; `GROUP BY col ASC|DESC` was removed in 8.0 (`ERROR 1064`). Add `ORDER BY` when order matters.

##### Operators that differ from standard SQL

```sql
SELECT 'a' || 'b';                     -- 0: || is logical OR (deprecated), not concatenation
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

- **BAD**: new `TIMESTAMP` columns for business dates or anything that may exceed January 2038 (expiry dates, contracts, birthdays far in the past — `TIMESTAMP` also cannot hold dates before 1970).
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

  `DATETIME` columns accept the same `DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` clauses (with matching precision, e.g. `DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3)`).

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
INSERT INTO tt VALUES (5, NULL, '2024-06-01 12:00:00+05:00', NULL);   -- 8.0.19+: offset literal
SELECT dt FROM tt WHERE id = 5;       -- 2024-06-01 07:00:00 (converted to the session time zone)

SET time_zone = '+02:00';
SELECT CAST(ts AT TIME ZONE '+00:00' AS DATETIME) AS utc, ts FROM tt WHERE id = 1;
-- 2024-06-01 12:00:00 | 2024-06-01 14:00:00
```

`CAST(... AT TIME ZONE ...)` only accepts a `TIMESTAMP` value and the zone `'+00:00'`/`'UTC'`. Named zones (`'Europe/Berlin'`) in `CONVERT_TZ()` and `SET time_zone` need the time-zone tables to be loaded (`mysql_tzinfo_to_sql`); otherwise `CONVERT_TZ()` returns `NULL` without even a warning, and `SET time_zone = 'Europe/Berlin'` fails with `ERROR 1298 Unknown or incorrect time zone`. The official Docker image ships them loaded; many installations do not. Prefer fixed offsets or conversion in the application, and check `SELECT COUNT(*) FROM mysql.time_zone_name` when code relies on named zones.

```sql
SELECT CONVERT_TZ('2024-03-31 01:30:00', 'UTC', 'Europe/Berlin');   -- 2024-03-31 03:30:00 (tables loaded)
SELECT '2024-01-31' + INTERVAL 1 MONTH;                              -- 2024-02-29 (clamped to month end)
SELECT UNIX_TIMESTAMP('2038-01-19 03:14:08'), FROM_UNIXTIME(2147483648);   -- 64-bit range works
```

- `NOW()` is constant for the statement; `SYSDATE()` is evaluated per call and is unsafe for statement-based replication. Use `NOW()`.
- Do not filter with `DATE(col) = ?` — use a half-open range (see Indexing).

***

#### Indexing

##### What InnoDB indexes can and cannot do

MySQL 8.4 InnoDB has B-tree indexes (`PRIMARY`, `UNIQUE`, secondary), `FULLTEXT` and `SPATIAL` (R-tree) indexes, plus functional (8.0.13+) and multi-valued (8.0.17+) B-tree indexes. It has **no** partial (`WHERE`) indexes, no `INCLUDE` columns, no GIN/GiST/BRIN, no hash indexes on InnoDB (`USING HASH` is accepted with Note 3502 and a B-tree is built instead), no `CREATE INDEX CONCURRENTLY` and no `CREATE INDEX IF NOT EXISTS`. Advice written for PostgreSQL does not translate; the MySQL equivalents are:

| PostgreSQL idea | MySQL 8.4 equivalent |
|---|---|
| Partial index `WHERE status = 'active'` | Composite index with the filter column first, or a generated column that is `NULL` for rows outside the subset (B-tree indexes do contain `NULL`s, but a selective value is still cheap to seek) |
| `INCLUDE (a, b)` covering index | Append the columns to the index key; every secondary index already contains the primary key |
| `CREATE INDEX CONCURRENTLY` | `ALTER TABLE ... ADD INDEX ..., ALGORITHM=INPLACE, LOCK=NONE` |
| Expression index `ON (lower(email))` | Usually unnecessary (`_ci` collation); otherwise functional index `((expr))` |
| GIN on JSONB array | Multi-valued index `((CAST(doc->'$.tags' AS CHAR(n) ARRAY)))` |
| `pg_trgm` for `LIKE '%x%'` | `FULLTEXT` index with `MATCH ... AGAINST` (word-based, not substring), or an external search engine |

##### Leftmost prefix, ranges, covering indexes

A composite index `(a, b, c)` serves predicates on `a`, `a,b`, `a,b,c`; an equality on the leading columns followed by one range column is ideal. Columns after the first range column are not used for seeking (they can still be used for index condition pushdown). Every secondary index implicitly ends with the primary-key columns.

The examples below use `users` (100,000 rows) and `orders` (200,000 rows):

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
EXPLAIN FORMAT=TREE
SELECT id, created_at FROM orders WHERE user_id = 42 ORDER BY created_at DESC LIMIT 10;
-- -> Limit: 10 row(s)
--     -> Covering index lookup on orders using ix_orders_user_created (user_id=42) (reverse)

EXPLAIN FORMAT=TREE
SELECT status, created_at FROM orders
WHERE status = 'paid' AND created_at >= '2024-05-01' ORDER BY created_at LIMIT 20;
-- -> Covering index range scan on orders using ix_orders_status_created
--      over (status = 'paid' AND '2024-05-01 00:00:00.000' <= created_at)

-- No predicate on the leading column: 8.0.13+ may use a skip scan instead of a full scan
EXPLAIN FORMAT=TREE
SELECT id FROM orders WHERE created_at >= '2024-03-01' AND created_at < '2024-03-02';
-- -> Covering index skip scan on orders using ix_orders_status_created over ... created_at ...
```

"Covering index" in the tree output (or `Using index` in traditional output) means no clustered-index lookup was needed. A skip scan is only efficient when the leading column has few distinct values; do not rely on it instead of a proper index.

##### Things that prevent index use

**Implicit type conversion.** Comparing a string column with a number converts every row's value to a number: full index scan, and wrong matches (`'1abc' = 1` is true). Comparing an integer column with a string literal is harmless.

```sql
-- BAD: phone is VARCHAR, literal is a number
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE phone = 4915100042;
-- -> Filter: (users.phone = 4915100042)  (rows=9980)
--     -> Covering index scan on users using ix_users_phone  (rows=99798)
-- Warning 1739: Cannot use ref access on index 'ix_users_phone' due to type or collation conversion on field 'phone'

-- GOOD
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE phone = '4915100042';
-- -> Covering index lookup on users using ix_users_phone (phone='4915100042')  (rows=1)
```

In application code this happens when a driver binds an integer to a `VARCHAR` column (e.g. a Go `int64` for a phone or external ID). Bind strings for string columns.

**Functions on indexed columns.**

```sql
-- BAD: full index scan
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE DATE(created_at) = '2024-01-02';
-- -> Filter: (cast(users.created_at as date) = '2024-01-02')
--     -> Covering index scan on users using ix_users_created  (rows=99798)

-- GOOD: half-open range
EXPLAIN FORMAT=TREE SELECT id FROM users
WHERE created_at >= '2024-01-02' AND created_at < '2024-01-03';
-- -> Covering index range scan on users using ix_users_created
--      over ('2024-01-02 00:00:00' <= created_at < '2024-01-03 00:00:00')  (rows=1440)

-- BAD: LOWER() is redundant with a _ci collation and disables the index
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE LOWER(email) = 'user42@example.com';
-- -> Filter: (lower(users.email) = 'user42@example.com')
--     -> Covering index scan on users using uq_users_email  (rows=99798)

-- GOOD: the collation already ignores case
SELECT id FROM users WHERE email = 'USER42@example.com';      -- 42 (const lookup)
```

Avoid `BETWEEN '2024-01-02' AND '2024-01-02 23:59:59'` for `DATETIME(3)` columns: it misses `23:59:59.500`. Use `>=` start and `<` next start.

**Leading wildcards.** `LIKE 'abc%'` is a range scan; `LIKE '%abc'` reads the whole index.

```sql
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE email LIKE 'user42%';
-- -> Covering index range scan on users using uq_users_email over ('user42' <= email <= 'user42????...')
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE email LIKE '%42@example.com';
-- -> Filter: (users.email like '%42@example.com')
--     -> Covering index scan on users using uq_users_email  (rows=99798)
```

**`OR` across different columns** may use `index_merge` (union) or fall back to a scan; check the plan, and consider `UNION ALL` of two indexed queries. Run 8.4.13 or later if you rely on `index_merge_union=off`: earlier 8.4 releases could drop a disjunct and return incomplete results with that setting.

**Collation mismatch** between join columns (see Character Sets) also defeats indexes.

##### Index size limits

With `ROW_FORMAT=DYNAMIC` (default) an index key part may be at most 3072 bytes. In `utf8mb4` that is `VARCHAR(768)`:

```sql
CREATE TABLE ix1 (s VARCHAR(768), KEY (s));    -- OK
CREATE TABLE ix2 (s VARCHAR(769), KEY (s));
-- ERROR 1071 (42000): Specified key was too long; max key length is 3072 bytes
```

Use a prefix index (`KEY (s(191))`) only for non-unique lookups: a prefix `UNIQUE` key enforces uniqueness of the prefix, not the value, and a prefix index can never be covering. For uniqueness of long values, index a hash in a generated column (`sha BINARY(32) AS (UNHEX(SHA2(url, 256))) STORED UNIQUE`).

##### Functional indexes and JSON

A functional index is used only when the query contains the **same expression with the same type and collation**. JSON extraction is the classic trap: `->>` returns `utf8mb4_bin`, while `CAST(... AS CHAR)` produces the connection collation, so an index on the plain cast is not used by `doc->>'$.name' = ?`.

```sql
CREATE TABLE docs (id INT AUTO_INCREMENT PRIMARY KEY, doc JSON NOT NULL);
-- 5,000 rows like {"name": "n42", "age": 42, "tags": ["t0", "t9"]}

-- BAD: the index expression's collation differs from doc->>'$.name'
ALTER TABLE docs ADD INDEX ix_name ((CAST(doc->>'$.name' AS CHAR(32))));
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE doc->>'$.name' = 'n42';
-- -> Filter: (json_unquote(json_extract(docs.doc,'$.name')) = 'n42')
--     -> Table scan on docs  (rows=5007)

-- GOOD: match the collation of ->> (utf8mb4_bin)
ALTER TABLE docs DROP INDEX ix_name;
ALTER TABLE docs ADD INDEX ix_name ((CAST(doc->>'$.name' AS CHAR(32)) COLLATE utf8mb4_bin));
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE doc->>'$.name' = 'n42';
-- -> Index lookup on docs using ix_name ((cast(...) collate utf8mb4_bin)='n42')  (rows=1)
```

A virtual generated column with an ordinary index is often clearer, and the optimizer also matches the underlying expression:

```sql
ALTER TABLE docs ADD COLUMN age INT AS (doc->>'$.age') VIRTUAL, ADD INDEX ix_age (age);
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE doc->>'$.age' = 42;   -- Index lookup ... using ix_age (age=42)
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE age = 42;             -- Covering index lookup ... ix_age
```

Multi-valued indexes serve `MEMBER OF`, `JSON_CONTAINS` and `JSON_OVERLAPS` — not `JSON_SEARCH` or `LIKE`:

```sql
ALTER TABLE docs ADD INDEX ix_tags ((CAST(doc->'$.tags' AS CHAR(8) ARRAY)));
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE 't3' MEMBER OF (doc->'$.tags');
-- -> Index lookup on docs using ix_tags (cast(json_extract(doc,_utf8mb4'$.tags') as char(8) array)=json'"t3"')
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE JSON_CONTAINS(doc->'$.tags', '"t3"');
-- -> Index range scan on docs using ix_tags ...
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE JSON_SEARCH(doc->'$.tags', 'one', 't3') IS NOT NULL;
-- -> Table scan on docs
```

Values longer than the `CHAR(n)` in the `ARRAY` cast cause an error on insert; size it for the data.

##### Descending, invisible and redundant indexes

```sql
CREATE INDEX ix_orders_created_desc ON orders (created_at DESC);
EXPLAIN FORMAT=TREE SELECT id, created_at FROM orders ORDER BY created_at DESC LIMIT 5;
-- -> Limit: 5 row(s) -> Covering index scan on orders using ix_orders_created_desc

-- Test the effect of dropping an index without dropping it:
ALTER TABLE orders ALTER INDEX ix_orders_created_desc INVISIBLE;
EXPLAIN FORMAT=TREE SELECT id, created_at FROM orders ORDER BY created_at DESC LIMIT 5;
-- -> Sort: orders.created_at DESC, limit input to 5 row(s) per chunk ... (rows=199485)
SET SESSION optimizer_switch = 'use_invisible_indexes=on';   -- per-session opt-in for testing
SET SESSION optimizer_switch = 'use_invisible_indexes=off';

ALTER TABLE users ALTER INDEX `PRIMARY` INVISIBLE;
-- ERROR 3522 (HY000): A primary key index cannot be invisible
```

- Descending indexes matter for mixed-direction sorts (`ORDER BY a ASC, b DESC`); for a single-direction sort a backward scan of an ascending index works too.
- Making an index invisible is a metadata change (`ALGORITHM=INPLACE`, not `INSTANT`); an invisible index is still maintained on writes and still enforces `UNIQUE`.
- Flag redundant indexes: `KEY (a)` next to `KEY (a, b)`, or `KEY (user_id)` next to the FK index. `sys.schema_redundant_indexes` lists them.
- Flag over-indexing on write-heavy tables: every secondary index costs a B-tree update per write, and with `innodb_change_buffering=none` (new 8.4 default) non-unique secondary index changes are no longer buffered.

##### Histograms

For non-indexed columns used in filters and joins, a histogram gives the optimizer selectivity estimates without index maintenance cost. 8.4 can refresh it automatically:

```sql
ANALYZE TABLE orders UPDATE HISTOGRAM ON total WITH 32 BUCKETS AUTO UPDATE;   -- AUTO UPDATE: 8.4+
SELECT column_name,
       JSON_EXTRACT(histogram, '$."histogram-type"') AS type,
       JSON_EXTRACT(histogram, '$."auto-update"')    AS auto
FROM information_schema.column_statistics WHERE table_name = 'orders';        -- total | "equi-height" | true
ANALYZE TABLE orders DROP HISTOGRAM ON total;
```

***

#### Reading Plans: EXPLAIN and EXPLAIN ANALYZE

- `EXPLAIN` (traditional table, the 8.4 default `explain_format`), `EXPLAIN FORMAT=TREE` (iterator tree, easiest to read), `EXPLAIN FORMAT=JSON` (with costs; `explain_json_format_version=2` gives the access-path layout).
- `EXPLAIN ANALYZE` **executes** the query and reports actual rows, loops and time per iterator. It supports `SELECT`, `TABLE` and multi-table `UPDATE`/`DELETE`. On 8.4.13, multi-table `UPDATE`/`DELETE` under `EXPLAIN ANALYZE` show the plan and timings but did not change any rows in tests, and single-table `UPDATE`/`DELETE` print `<not executable by iterator executor>`; still never run `EXPLAIN ANALYZE` of DML against production data on the assumption that it is harmless, and remember the `SELECT` really runs (an expensive query runs in full).
- Key signals: `Table scan`, `Covering index scan` over many rows, `Filter` above a scan with a large `rows` estimate, `Sort` (filesort) on big inputs, `Materialize`/temporary tables, `Nested loop` with a large outer side and no index on the inner side, `Hash join` (8.0.18+, used for equi-joins without a usable index).
- Compare estimated `rows` with actual `rows` in `EXPLAIN ANALYZE`; large gaps point to missing statistics (`ANALYZE TABLE`, histograms) or correlated predicates.
- `rows` estimates on InnoDB come from sampling; `information_schema.tables.table_rows` is an estimate as well.

```sql
EXPLAIN ANALYZE SELECT id, total FROM orders
WHERE status = 'paid' ORDER BY created_at, id LIMIT 20 OFFSET 40000;
-- -> Limit/Offset: 20/40000 row(s)  (actual time=308..308 rows=20 loops=1)
--     -> Index lookup on orders using ix_orders_status_created (status='paid')
--          (actual time=2.05..299 rows=40020 loops=1)
```

***

#### Query Patterns and Performance

##### Pagination: OFFSET vs keyset — and the row-constructor trap

`LIMIT n OFFSET m` reads and discards `m` rows (40,020 rows read above to return 20). Use keyset (seek) pagination with a deterministic order that ends in a unique column.

**MySQL 8.4 does not use range access for row-constructor inequalities** like `(created_at, id) > (?, ?)`. The predicate becomes a `Filter` over an index scan that starts at the beginning of the index — just as slow as `OFFSET` for later pages. Write the expanded `OR` form, which the optimizer turns into a range scan:

```sql
-- BAD on MySQL: reads from the start of the index and filters (42,035 rows read)
EXPLAIN ANALYZE SELECT id, total FROM orders
WHERE status = 'paid' AND (created_at, id) > ('2024-08-21 10:00:00.000', 160000)
ORDER BY created_at, id LIMIT 20;
-- -> Limit: 20 row(s)  (actual time=678..678 rows=20 loops=1)
--     -> Filter: ((orders.created_at,orders.id) > ('2024-08-21 10:00:00.000',160000))
--         -> Index lookup on orders using ix_orders_status_created (status='paid') (actual ... rows=42035)

-- GOOD: expanded predicate → range scan, 20 rows read
EXPLAIN ANALYZE SELECT id, total FROM orders
WHERE status = 'paid'
  AND (created_at > '2024-08-21 10:00:00.000'
       OR (created_at = '2024-08-21 10:00:00.000' AND id > 160000))
ORDER BY created_at, id LIMIT 20;
-- -> Limit: 20 row(s)  (actual time=3.92..3.93 rows=20 loops=1)
--     -> Index range scan on orders using ix_orders_status_created over
--          (status = 'paid' AND created_at = '2024-08-21 10:00:00.000' AND 160000 < id)
--          OR (status = 'paid' AND '2024-08-21 10:00:00.000' < created_at) ...
```

The same happens without an equality prefix (`WHERE (created_at, id) > (?, ?)` on `KEY (created_at)` read 86,420 index entries to return 20 rows) and with an explicit two-column index `(a, b)`. Verified identical on 9.7.4. Row-constructor *equality* `(a, b) = (?, ?)` and `IN ((?, ?), ...)` are optimized normally.

##### COUNT(*)

InnoDB has no stored row count: `SELECT COUNT(*) FROM t` scans the smallest index (in parallel, `innodb_parallel_read_threads`). For dashboards, use estimates (`information_schema.tables.table_rows`) or maintained counters. Filtered counts need a covering index on the filter columns. `SQL_CALC_FOUND_ROWS`/`FOUND_ROWS()` are deprecated and usually slower than a separate `COUNT(*)`.

```sql
SELECT SQL_CALC_FOUND_ROWS id FROM par LIMIT 1;
SELECT FOUND_ROWS();
-- Warning 1287: FOUND_ROWS() is deprecated and will be removed in a future release. Consider using COUNT(*) instead.
```

##### N+1 queries and IN lists

Replace per-row queries with one join or one `IN (...)` batch. Keep `IN` lists bounded (hundreds to a few thousand values): very large lists increase parse time and can exceed `range_optimizer_max_mem_size` (8 MiB default), after which the optimizer stops using range access and falls back to a scan (with a warning). For huge sets, load the keys into a temporary table and join.

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

The nested derived table works on 8.4.13 because it is materialized before the outer statement modifies the table; it reads as a trick, so prefer an explicit multi-table `UPDATE ... JOIN` / `DELETE t FROM t JOIN ...` for clarity.

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

##### Top-N per group, window functions, LATERAL

```sql
-- Latest order per user: window function
WITH ranked AS (
  SELECT o.*, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC, id DESC) AS rn
  FROM orders o WHERE user_id IN (1, 2, 3)
)
SELECT user_id, id, created_at FROM ranked WHERE rn = 1;

-- Same with LATERAL (8.0.14+): one index probe per user when (user_id, created_at) is indexed
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
- `QUALIFY` is reserved but not usable on 8.4 Community (`ERROR 6037 'QUALIFY clause' can be used only if the hypergraph optimizer is enabled`); filter on the window result in an outer query.

##### CTEs and recursion

```sql
WITH RECURSIVE r (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r WHERE n < 5)
SELECT GROUP_CONCAT(n) FROM r;                                  -- 1,2,3,4,5

WITH RECURSIVE r (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r)
SELECT COUNT(*) FROM r;
-- ERROR 3636 (HY000): Recursive query aborted after 1001 iterations. Try increasing @@cte_max_recursion_depth ...

WITH RECURSIVE r (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r LIMIT 10)
SELECT COUNT(*) FROM r;                                         -- 10 (LIMIT in the recursive CTE, 8.0.19+)
```

Recursive CTEs over data (trees, graphs) need a termination condition that does not depend on the data being acyclic: carry a depth counter or a path and stop on cycles. Raising `cte_max_recursion_depth` globally hides runaway queries; set it per session (`SET SESSION` or `/*+ SET_VAR(cte_max_recursion_depth = N) */`) only where needed. Non-recursive CTEs are handled like derived tables (merged or materialized by the optimizer); PostgreSQL's `WITH x AS MATERIALIZED (...)` is a syntax error (use the `MERGE`/`NO_MERGE` optimizer hints instead).

##### Set operations, VALUES, TABLE

```sql
SELECT 1 AS x UNION SELECT 2 INTERSECT SELECT 2;     -- 1, 2 (INTERSECT binds tighter than UNION)
(SELECT id FROM par) EXCEPT (SELECT 1);              -- INTERSECT/EXCEPT: 8.0.31+
TABLE par ORDER BY id LIMIT 1;                        -- TABLE statement: 8.0.19+
SELECT * FROM (VALUES ROW(1, 'a'), ROW(2, 'b')) AS v (id, name);   -- VALUES needs ROW(); 8.0.19+
```

`UNION` removes duplicates (extra sort/hash); use `UNION ALL` unless de-duplication is intended.

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
       JSON_VALUE(doc, '$.age' RETURNING UNSIGNED) AS age    -- 8.0.21+
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

`->` and `->>` only accept a column on the left (`CAST('{"a":1}' AS JSON) ->> '$.a'` is `ERROR 1064`); use `JSON_EXTRACT()`/`JSON_UNQUOTE()` for expressions. Distinguish JSON `null` from a missing key when validating input.

##### Optimizer hints and index hints

```sql
SELECT /*+ MAX_EXECUTION_TIME(100) */ COUNT(*) FROM orders a JOIN orders b ON a.total = b.total;
-- ERROR 3024 (HY000): Query execution was interrupted, maximum statement execution time exceeded
SELECT /*+ MAX_EXECUTION_TIME(100) */ SLEEP(1);   -- returns 1 (interrupted), no error
SELECT /*+ SET_VAR(sort_buffer_size = 1048576) */ 1;

-- A hint can force a worse plan than the optimizer would choose:
EXPLAIN FORMAT=TREE SELECT /*+ INDEX(orders ix_orders_status_created) */ id
FROM orders WHERE user_id = 5 AND status = 'paid';
-- -> Filter: (orders.user_id = 5)
--     -> Index lookup on orders using ix_orders_status_created (status='paid')  (rows=66495)
```

- Hints (`/*+ ... */`, 8.0+) are preferred over `USE/FORCE/IGNORE INDEX`: they are scoped to a query block and can name join orders (`JOIN_ORDER`), index choice (`INDEX`, `NO_INDEX`, 8.0.20+), and session variables (`SET_VAR`). Every hint is a maintenance liability: it does not adapt when data changes. Dropping or renaming an index interacts differently with the two kinds: an optimizer hint naming a missing index only raises `Warning 3128 Unresolved name ... for INDEX hint` and the hint is ignored, while `FORCE INDEX (missing)` / `USE INDEX (missing)` makes the query fail with `ERROR 1176 Key 'ix_gone' doesn't exist in table`. Search the code for index hints before dropping or renaming an index, and require a comment explaining the measured plan for every hint.
- `MAX_EXECUTION_TIME` / `max_execution_time` apply to read-only `SELECT` only; `SLEEP()` returns 1 when interrupted rather than erroring.
- Hints in comments are stripped by some clients and proxies when comments are stripped; the `mysql` client keeps comments by default in 8.4.

##### Miscellaneous performance rules

- `ORDER BY RAND() LIMIT n` sorts the whole table; pick random IDs in the application or sample a key range.
- `SELECT *` matters on wide tables, with `TEXT`/`BLOB`/`JSON` columns (off-page storage) and when a covering index could serve the query.
- `DISTINCT` to fix duplicate rows from a join usually hides a wrong join; fix the join (or use `EXISTS`).
- Implicit comma joins are equivalent to `JOIN` but easy to get wrong; prefer explicit `JOIN ... ON`.
- Large `DELETE`/`UPDATE` in one statement create a long transaction, a huge undo log and replica lag; batch by primary-key ranges (see Writes).
- Temporary tables for `GROUP BY`/`DISTINCT`/`UNION` use the TempTable engine; 8.4 sizes `temptable_max_ram` at 3% of RAM (1–4 GiB) and disables mmap spill files (`temptable_max_mmap=0`), so large internal temporary tables spill to InnoDB on-disk temporary tables sooner than on 8.0.

***

#### Writes: INSERT, Upserts, UPDATE, DELETE

##### Batch inserts

Multi-row `INSERT ... VALUES (...), (...)` in transactions of a few thousand rows is the standard fast path. `LOAD DATA INFILE` reads files on the server (restricted by `secure_file_priv`, here `/var/lib/mysql-files/`, and the `FILE` privilege); `LOAD DATA LOCAL INFILE` needs `local_infile=ON` on the server (default `OFF`) **and** client opt-in, and lets the server request any file the client can read — keep it off unless required. There is no `COPY`.

##### INSERT ... ON DUPLICATE KEY UPDATE

Use the row-alias syntax (8.0.19+); `VALUES(col)` in the update clause is deprecated:

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

- **Affected-rows semantics**: 1 = inserted, 2 = updated, 0 = matched but unchanged. Code that checks `RowsAffected() == 1` for "row exists now" is wrong; many drivers can switch to found-rows semantics (`CLIENT_FOUND_ROWS`, e.g. Go `clientFoundRows=true`, which reports 1 for an unchanged match).
- **`AUTO_INCREMENT` values are consumed by the attempted insert even when the row is updated** (`innodb_autoinc_lock_mode=2`), so IDs have gaps; upsert-heavy tables can exhaust an `INT` key quickly.

```sql
INSERT INTO inv (sku, qty) VALUES ('z1', 1), ('z2', 2);
SELECT LAST_INSERT_ID();             -- 7: the FIRST id of the multi-row insert
SELECT sku, id FROM inv ORDER BY id; -- a 1, b 3, z1 7, z2 8  (2, 4, 5, 6 burnt by upserts)
```

- **Multiple unique keys**: if the new row conflicts with different rows on different unique keys, only one row is updated and which one is not specified. Upsert only against tables with a single unique constraint relevant to the conflict, or make the logic explicit.
- **Statement-based replication** considers ODKU on tables with several unique keys unsafe. With the default `binlog_format=ROW` this is not a correctness issue.
- `LAST_INSERT_ID()` after an ODKU that updated a row does not return that row's id unless you write `id = LAST_INSERT_ID(id)` in the update list.

##### REPLACE and INSERT IGNORE

`REPLACE` is `DELETE` + `INSERT`: it changes the auto-increment id, fires delete triggers, and **cascades deletes to child rows**:

```sql
CREATE TABLE par (id INT PRIMARY KEY, name VARCHAR(10));
CREATE TABLE chi (id INT PRIMARY KEY, par_id INT,
                  FOREIGN KEY (par_id) REFERENCES par (id) ON DELETE CASCADE);
INSERT INTO par VALUES (1, 'one');
INSERT INTO chi VALUES (100, 1), (101, 1);
REPLACE INTO par VALUES (1, 'uno');
SELECT ROW_COUNT();                       -- 2 (one delete + one insert)
SELECT COUNT(*) AS children_left FROM chi; -- 0  !!
```

`INSERT IGNORE` silently drops rows for **any** ignorable error, including foreign-key violations:

```sql
INSERT IGNORE INTO chi VALUES (102, 999);
-- Warning 1452: Cannot add or update a child row: a foreign key constraint fails ...
SELECT COUNT(*) FROM chi;                 -- 0: row silently not inserted
```

Flag both in review unless the delete-and-insert or ignore-everything semantics are explicitly intended. Use `INSERT ... ON DUPLICATE KEY UPDATE`.

##### UPDATE/DELETE in batches

```sql
-- GOOD: bounded batches by primary key, each its own short transaction
DELETE FROM orders WHERE status = 'cancelled' AND created_at < '2024-02-01' ORDER BY id LIMIT 1000;
-- repeat until ROW_COUNT() = 0, sleeping between batches if replicas lag
```

`DELETE`/`UPDATE ... LIMIT` without `ORDER BY` is nondeterministic (and unsafe for statement-based replication). Under REPEATABLE READ, a `DELETE ... WHERE created_at < ?` without a usable index locks every scanned row and the gaps between them for the whole transaction.

Multi-table `UPDATE ... JOIN` and `DELETE t1 FROM t1 JOIN t2 ...` are the MySQL forms of `UPDATE ... FROM` (PostgreSQL); `ORDER BY`/`LIMIT` are not allowed with multi-table statements.

***

#### Transactions, Isolation and Locking

##### Consistent reads vs current reads

Under REPEATABLE READ, the snapshot is taken at the **first read** in the transaction, not at `START TRANSACTION` (use `START TRANSACTION WITH CONSISTENT SNAPSHOT` to take it immediately). Plain `SELECT` reads the snapshot; `UPDATE`, `DELETE`, `INSERT ... SELECT` and locking reads see the latest committed data. Read-modify-write in application code therefore loses updates:

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

InnoDB locks **every index record it scans**, plus the gap before it (next-key locks) under REPEATABLE READ.

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

- A locking read or `UPDATE`/`DELETE` whose `WHERE` clause is not served by an index locks the whole table in REPEATABLE READ. Make sure such statements use an index (check with `EXPLAIN`).
- "Select-then-insert if absent" with `SELECT ... FOR UPDATE` on a missing key takes a gap lock; two sessions doing it concurrently both get the gap lock and then deadlock on their inserts. Use `INSERT ... ON DUPLICATE KEY UPDATE` or insert first and handle error 1062.
- READ COMMITTED (requires row-based binlogging, the default) takes far fewer gap locks and is a reasonable choice for OLTP services that do not rely on repeatable snapshots. Set it explicitly per session or in the connection string, and review code that assumed a stable snapshot.

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

- **Error 1213 (deadlock)** rolls back the whole transaction. The application must retry the **whole transaction** (with a bounded number of attempts and backoff), not just the failing statement.
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
- Prevent deadlocks: touch rows in a consistent order (sort IDs before updating), keep transactions short, avoid user interaction or network calls inside transactions, index the columns used in locking `WHERE` clauses.
- Diagnose with `SHOW ENGINE INNODB STATUS` (`LATEST DETECTED DEADLOCK`) and `innodb_print_all_deadlocks=ON` (default `OFF`) for the error log; `sys.innodb_lock_waits` shows current waits.

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

`SKIP LOCKED` needs an index that matches the filter and order; otherwise each worker scans (and locks) far more rows. `FOR UPDATE OF tbl` restricts locking to the named tables in a join; `LOCK IN SHARE MODE` still works and is the older spelling of `FOR SHARE`.

##### Implicit commits and autocommit

```sql
START TRANSACTION;
INSERT INTO par VALUES (2, 'two');
CREATE TABLE tmp_x (id INT);     -- implicit COMMIT
ROLLBACK;
SELECT * FROM par WHERE id = 2;  -- 2 | two   (the insert was committed)
```

DDL, `LOCK TABLES`, `START TRANSACTION` (inside another transaction), `SET autocommit = 1`, user/privilege statements and others implicitly commit. Do not mix DDL into transactional application logic or tests that rely on rollback. `CREATE TEMPORARY TABLE` does not commit.

Long-running transactions (including idle connections left in a transaction by a pool) hold undo history (purge lag, growing undo tablespaces, slower reads) and metadata locks that block DDL. Set statement timeouts in the application and alert on old entries in `information_schema.innodb_trx`.

***

#### Schema Migrations and Online DDL

##### No transactional DDL, no IF NOT EXISTS for columns or indexes

A migration with several statements is applied statement by statement. If statement 3 fails, statements 1–2 stay applied and the migration tool's "rollback" does nothing for DDL. Write migrations as one DDL per file or make them idempotent by checking `information_schema` in the tool, and combine related changes into **one `ALTER TABLE`** (one table rebuild, one metadata-lock window):

```sql
-- BAD (MariaDB syntax, ERROR 1064 on MySQL 8.4 and 9.7)
ALTER TABLE x1 ADD COLUMN IF NOT EXISTS b INT;
CREATE INDEX IF NOT EXISTS ix ON x1 (id);

-- GOOD: one statement, explicit algorithm and lock so it fails fast instead of silently copying
ALTER TABLE orders
  ADD COLUMN note VARCHAR(100) NULL,
  ADD COLUMN source VARCHAR(20) NULL,
  ALGORITHM = INSTANT;
```

`CREATE TABLE IF NOT EXISTS`, `DROP TABLE IF EXISTS`, `CREATE OR REPLACE VIEW` and `DROP VIEW IF EXISTS` exist; `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS` and `DROP INDEX IF EXISTS` do not.

##### ALGORITHM and LOCK

Specify `ALGORITHM` (and `LOCK` for `INPLACE`) on every `ALTER TABLE` against a large table. Without it MySQL silently picks the "best" algorithm, which may be a full table copy that blocks writes for hours. With it, an unsupported combination fails immediately.

Verified on 8.4.13:

| Operation | INSTANT | INPLACE, LOCK=NONE | Notes |
|---|---|---|---|
| `ADD COLUMN` (any position, nullable or with default) | yes | — | 8.0.29+ any position; consumes a row version |
| `DROP COLUMN` | yes | — | consumes a row version |
| `RENAME COLUMN` | yes | | |
| `ALTER COLUMN ... SET/DROP DEFAULT` | yes | | |
| Add `VIRTUAL` generated column | yes | | |
| Add `STORED` generated column | no | no | `ALGORITHM=COPY` only |
| `MODIFY VARCHAR(50) → VARCHAR(60)` (utf8mb4) | no | yes | length prefix stays 1 byte (≤ 255 bytes) |
| `MODIFY VARCHAR(60) → VARCHAR(100)` (utf8mb4) | no | **no** | 240 → 400 bytes crosses the 255-byte length-prefix boundary: COPY |
| Change column type (`INT → BIGINT`) | no | **no** | COPY, blocks writes (`LOCK=SHARED`) |
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
ALTER TABLE d2 ADD CONSTRAINT fk_d2 FOREIGN KEY (pid) REFERENCES ddp (id), ALGORITHM=INPLACE;
-- ERROR 1846 (0A000): ALGORITHM=INPLACE is not supported. Reason: Adding foreign keys needs foreign_key_checks=OFF. Try ALGORITHM=COPY.
```

Even `INPLACE, LOCK=NONE` operations rebuild the table (I/O, temporary disk space up to the table size, replica lag because the replica applies the ALTER after it finished on the source, serially). Online DDL also buffers concurrent writes in `innodb_online_alter_log_max_size` (128 MiB default); if it overflows, the ALTER fails at the end. For very large tables use `gh-ost` or `pt-online-schema-change` and review their configuration like a migration.

##### INSTANT row-version limit

Each `INSTANT` `ADD COLUMN`/`DROP COLUMN` increments the table's row version; MySQL 8.4 allows **64**:

```sql
-- after 64 instant add/drop operations:
ALTER TABLE rv ADD COLUMN c33 INT, ALGORITHM=INSTANT;
-- ERROR 4092 (HY000): Maximum row versions reached for table t/rv. No more columns can be added or dropped instantly. Please use COPY/INPLACE.
SELECT total_row_versions FROM information_schema.innodb_tables WHERE name = 't/rv';   -- 64
ALTER TABLE rv ADD COLUMN z INT;        -- no ALGORITHM: silently falls back to a table rebuild (versions reset to 0)
```

Without an explicit `ALGORITHM=INSTANT`, the 65th change quietly becomes a full rebuild during the deploy. Projects that add and drop columns frequently (feature flags, ORMs) should watch `total_row_versions`. (9.1.0 raised the limit to 255; 8.4 stays at 64.) Since 8.4.4, adding or dropping a column on an empty table **without an `ALGORITHM` clause** uses `INPLACE` and does not consume a row version; an explicit `ALGORITHM=INSTANT` still consumes one (verified on 8.4.13).

##### Metadata locks: the most common migration outage

Every DDL, even `ALGORITHM=INSTANT`, needs an exclusive metadata lock (MDL) on the table for a moment. It waits for **every open transaction that has touched the table** — including an idle transaction that only ran a `SELECT`. While it waits, **every new query on the table queues behind it**:

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
-- (information_schema.processlist works too but raises deprecation warning 1287 on 8.4)
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

Review checklist for migrations: explicit `ALGORITHM`/`LOCK`, short `lock_wait_timeout`, no long transactions running against the table (check `information_schema.innodb_trx`), one `ALTER` per table, backfills in batches, and replica lag considered.

##### Changing columns safely

- Widening `INT` to `BIGINT` is a full copy; plan it early (the most common emergency migration is an exhausted `INT AUTO_INCREMENT`).
- Renaming columns or tables breaks running application versions; use expand/contract: add the new column, dual-write, backfill in batches, switch reads, drop the old column in a later release.
- Adding `NOT NULL` with a default to a big table is `INSTANT` only as an `ADD COLUMN`; converting an existing nullable column to `NOT NULL` rebuilds and fails on existing `NULL`s (backfill first).
- Dropping a column that code still names explicitly breaks the old release during a rolling deploy (making it `INVISIBLE` does not help: invisible columns are still returned when named). Remove the column from all code first, deploy, then drop it.
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

With binary logging enabled (the default), creating a stored function requires a characteristic that declares it safe:

```sql
DELIMITER //
CREATE FUNCTION f_nd(x INT) RETURNS INT BEGIN RETURN x + 1; END//
-- ERROR 1418 (HY000): This function has none of DETERMINISTIC, NO SQL, or READS SQL DATA in its
--   declaration and binary logging is enabled (you *might* want to use the less safe
--   log_bin_trust_function_creators variable)
CREATE FUNCTION f_ok(x INT) RETURNS INT DETERMINISTIC NO SQL BEGIN RETURN x + 1; END//
DELIMITER ;
```

Declaring `DETERMINISTIC` on a function that is not (uses `NOW()`, `RAND()`, `UUID()`, reads tables that change) is a lie the optimizer and replication rely on; flag it. Setting `log_bin_trust_function_creators=ON` globally to make migrations pass is a security and replication smell. `DELIMITER` is a client command of the `mysql` CLI, not SQL: migration tools and drivers that send statements one at a time must not include it.

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

##### DEFINER and SQL SECURITY

Views, routines, triggers and events run with the privileges of their `DEFINER` by default (`SQL SECURITY DEFINER`). A view or procedure defined by a privileged account is a privilege-escalation path. Review:

- `DEFINER` clauses in migrations: dumps from one environment often carry `DEFINER=root@localhost` or a personal account that does not exist elsewhere. Setting a definer other than yourself needs `SET_ANY_DEFINER` (new in 8.2, replacing `SET_USER_ID`); orphaned definers need `ALLOW_NONEXISTENT_DEFINER`.
- Prefer `SQL SECURITY INVOKER` for views and routines that only encapsulate queries.

##### Triggers

- Triggers do not fire for foreign-key cascades (shown in Foreign keys) and are skipped by `TRUNCATE TABLE`.
- Row-by-row triggers multiply the cost of bulk statements; complex business logic in triggers is invisible to application reviewers. Keep them small (auditing, denormalized counters) and covered by tests.
- A trigger cannot modify the table that fired it (`ERROR 1442`).

##### Views

MySQL has no materialized views; `CREATE MATERIALIZED VIEW` is a syntax error. Emulate with a summary table refreshed by a job or maintained by the application. Views with `GROUP BY`, `DISTINCT`, aggregates, `UNION` or `LIMIT` are evaluated with a temporary table (`ALGORITHM=TEMPTABLE`) and predicates from the outer query may not be pushed into them; check plans of queries on views.

##### Events

The event scheduler (`event_scheduler=ON` by default since 8.0) runs events with the definer's privileges on the source; on replicas, replicated events are created disabled (`REPLICA_SIDE_DISABLED`). `DISABLE ON SLAVE` is deprecated in favour of `DISABLE ON REPLICA`. `CREATE/ALTER/DROP EVENT` cannot be run as prepared statements on 8.4 (`ERROR 1295`; allowed from 9.0).

***

#### Security

##### SQL injection: parameters for values, allow-lists for identifiers

Every value must be sent as a bound parameter. Identifiers (table, column, `ORDER BY` column, `ASC`/`DESC`) cannot be parameters and must come from a fixed allow-list in code. Verified with Go `database/sql` and `github.com/go-sql-driver/mysql` v1.10.1 against 8.4.13:

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
- Keep `multiStatements` (and equivalents in other drivers) **off**: with it disabled, a stacked query is a syntax error (`Error 1064 ... near 'SELECT 2'`), which removes a whole class of injection payloads.
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

##### Authentication: caching_sha2_password [New in 8.4]

`mysql_native_password` is disabled in 8.4 and gone in 9.0, so every account and every client must handle `caching_sha2_password`:

- The first login of an account after a server restart or `FLUSH PRIVILEGES` (cache empty) needs either a **TLS connection** or RSA public-key exchange; over plain TCP without either, the login fails:

```text
$ mysql -uapp2 -p... -h127.0.0.1 --ssl-mode=DISABLED
ERROR 2061 (HY000): Authentication plugin 'caching_sha2_password' reported error: Authentication requires secure connection.
$ mysql -uapp2 -p... -h127.0.0.1 --ssl-mode=DISABLED --get-server-public-key
(succeeds; afterwards the cached fast path works without TLS until the cache is cleared)
```

  This is the classic "works until the database restarts" bug. Use TLS (the server auto-generates certificates; clients default to `PREFERRED`), and for production require it: `require_secure_transport=ON` on the server or `REQUIRE SSL` per account, plus `VERIFY_IDENTITY` in clients. Flag driver options such as `allowPublicKeyRetrieval=true` (JDBC) or `tls=skip-verify` (Go) in production configs: they accept a man-in-the-middle key.
- Old client libraries (very old PHP `mysqlnd`, Python `MySQLdb` builds against libmysqlclient 5.x, Node `mysql` package) do not support `caching_sha2_password`; `mysql2` and current connectors do.
- Upgrades from 8.0: accounts still using `mysql_native_password` cannot log in after the upgrade unless the server is started with `--mysql-native-password=ON`. Migrate them before upgrading: `ALTER USER app@'%' IDENTIFIED WITH caching_sha2_password BY '...'`.
- Account management statements: `CREATE USER ... IDENTIFIED BY`, `ALTER USER`, password policies (`PASSWORD EXPIRE`, `FAILED_LOGIN_ATTEMPTS n PASSWORD_LOCK_TIME d`). `GRANT` does not create users (removed in 8.0), and `PASSWORD()` does not exist.

##### Least privilege and roles

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

Review rules:

- Application accounts get DML on their schema only; migrations run under a separate account with DDL privileges. No `ALL PRIVILEGES ON *.*`, no `SUPER`, `FILE`, `PROCESS`, `SYSTEM_USER`, `GRANT OPTION` or `CONNECTION_ADMIN` for applications.
- Every role grant needs `SET DEFAULT ROLE` (or `SET ROLE` in the session); otherwise the account has no effective privileges.
- `_` and `%` in database names in `GRANT ... ON db_name.*` are wildcards: `GRANT SELECT ON db_1.*` also matches `dbx1`. Escape them (`` `db\_1` ``) or avoid them in schema names. The wildcard behaviour is deprecated since 8.2.0 (8.0.35 on the 8.0 line; no warning is printed on 8.4.13).
- Host part: prefer specific hosts or network ranges over `'%'`; `'localhost'` matches only socket/`localhost` connections.
- Credentials never in migrations or code; use environment variables or a secret store, and do not log DSNs (the Go driver's `FormatDSN()` contains the password).

##### Other server-side security settings

- `local_infile=OFF` (default): keep it off; `LOAD DATA LOCAL` lets a malicious server read client files.
- `secure_file_priv` restricts `LOAD DATA INFILE` / `SELECT ... INTO OUTFILE` to one directory; an empty value (`""`) allows the whole filesystem.
- `MD5()`, `SHA1()`, `SHA2()` are fast hashes, unsuitable for passwords; hash passwords in the application with bcrypt/scrypt/Argon2.
- `AES_ENCRYPT()` defaults to `block_encryption_mode=aes-128-ecb`: deterministic, no IV, leaks equal plaintexts. There is no authenticated (GCM) mode. Prefer encrypting in the application; if it must happen in SQL, use a CBC mode with a random IV per value and keep keys out of SQL text (they appear in logs and the processlist):

```sql
SELECT HEX(AES_ENCRYPT('secret', 'key')) = HEX(AES_ENCRYPT('secret', 'key')) AS ecb_deterministic;  -- 1
SET SESSION block_encryption_mode = 'aes-256-cbc';
SET @iv = RANDOM_BYTES(16);
SET @key = UNHEX(SHA2('k', 256));               -- 32 raw bytes; a 64-char hex string draws Warning 3237
SELECT AES_DECRYPT(AES_ENCRYPT('secret', @key, @iv), @key, @iv);   -- secret
SELECT AES_ENCRYPT('secret', 'key');            -- ERROR 1582: CBC requires the IV argument
SET SESSION block_encryption_mode = 'aes-256-gcm';   -- ERROR 1231: not supported
```
- Logs (general log, slow log, binlog, `performance_schema` digests and history) contain literal values from non-parameterized statements: another reason to bind parameters.

***

#### Application Code and Drivers

Verified with `github.com/go-sql-driver/mysql` v1.10.1; the same concepts apply to other drivers.

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
- `interpolateParams=true` makes the driver substitute parameters client-side to save a round trip; the driver rejects it with the multibyte encodings BIG5, CP932, GB2312, GBK and SJIS because escaping is unsafe there. Leave it off unless measured.
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
	case 1452, 1451: // FK violation on child insert / parent delete
	}
}
```

- Use `context` deadlines (`QueryContext`, `ExecContext`). On cancellation the Go driver abandons the connection, but the server may keep executing the statement until it notices the disconnect; bound expensive reads with `MAX_EXECUTION_TIME` as well. Always `defer rows.Close()` and check `rows.Err()`.
- Transactions: `BeginTx` with `sql.TxOptions{Isolation: sql.LevelReadCommitted}` sets the isolation per transaction. Never run transaction statements (`START TRANSACTION`, `COMMIT`) through `db.Exec` on a pool — they land on different connections.

***

#### Replication and Operations Awareness

Code review should still catch replication hazards, even with `binlog_format=ROW` as the default:

- Use 8.4 terminology in scripts and tooling: `CHANGE REPLICATION SOURCE TO SOURCE_HOST=...`, `START REPLICA`, `SHOW REPLICA STATUS`, `SHOW BINARY LOG STATUS`, `RESET BINARY LOGS AND GTIDS`. The `MASTER`/`SLAVE` statements and options are syntax errors on 8.4. Monitoring queries that read `Seconds_Behind_Master` must use `Seconds_Behind_Source`; `Com_slave_*` and `Com_show_master_status` status variables are gone.
- Large single transactions (bulk `UPDATE`/`DELETE`, `ALTER TABLE` copies) create replica lag proportional to their duration. Batch them.
- Statements with nondeterministic results (`LIMIT` without `ORDER BY`, `SYSDATE()`, `UUID()` in statement-based setups) are only safe because of row-based logging; keep `binlog_format=ROW` (8.4 always tracks dependencies with writesets, which requires row-based logging; `MIXED` is no longer supported for that, and the `binlog_format` variable itself is deprecated).
- `CREATE TABLE ... SELECT` is atomic and allowed with GTIDs since 8.0.21, but it locks the source rows for the duration under REPEATABLE READ; prefer `CREATE TABLE ... LIKE` plus batched `INSERT ... SELECT`.
- Reads from replicas are eventually consistent; read-your-writes paths must go to the source (or wait with `WAIT_FOR_EXECUTED_GTID_SET()`).

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

- The `sys` schema views above are installed by default.
- Slow query log: `slow_query_log=OFF` and `long_query_time=10` by default; enable it with a threshold that matches the service's latency budget (e.g. 0.5–1 s) and analyze with `pt-query-digest` or `mysqldumpslow`. `log_queries_not_using_indexes` floods the log on small tables; use it briefly.
- `SUM_ROWS_EXAMINED / SUM_ROWS_SENT` far above 1 indicates a missing or unused index.
- `sys.schema_unused_indexes` counts only since the last restart and only on this server; an index unused on the source may serve replica reads. Make an index invisible before dropping it.
- `SHOW ENGINE INNODB STATUS` → `LATEST DETECTED DEADLOCK`, `TRANSACTIONS` (history list length = purge lag).
- `EXPLAIN FOR CONNECTION <id>` shows the plan of a running statement.

***

#### Removed / Changed in 8.4 — Migration Notes (from 8.0)

1. **Authentication**: migrate every `mysql_native_password` account to `caching_sha2_password` (`SELECT user, host, plugin FROM mysql.user WHERE plugin = 'mysql_native_password'`), make sure all clients support it, and use TLS or RSA key retrieval. Remove `default_authentication_plugin` from `my.cnf` (the server refuses to start with removed options); use `authentication_policy`.
2. **Replication statements**: replace every `MASTER`/`SLAVE` statement, option and status-variable name in scripts, Ansible roles, Helm charts and monitoring queries (table in "What's New").
3. **Foreign keys**: find FKs that reference non-unique or partial keys before upgrading (they still load, but new ones cannot be created and dumps fail to restore with `restrict_fk_on_non_standard_key=ON`); add the missing `UNIQUE` keys.
4. **Inline `REFERENCES`**: find column-level `REFERENCES` in DDL; they never created constraints on 8.0 or 8.4. Convert them to table-level `FOREIGN KEY` clauses (and clean up orphans) before an eventual move to 9.7, which enforces them.
5. **Removed syntax**: `AUTO_INCREMENT` on `FLOAT`/`DOUBLE` columns blocks the upgrade; `FLUSH HOSTS`, `LOCK TABLES ... LOW_PRIORITY WRITE`, `$a$b` identifiers, unquoted `QUALIFY`/`TABLESAMPLE` identifiers fail.
6. **Removed server options and plugins**: `keyring_file`/`keyring_encrypted_file`/`keyring_oci` plugins (migrate to keyring components first), `authentication_fido`, `openssl_udf`, weak entries in `ssl_cipher`/`tls_ciphersuites`, `--ssl`/`have_ssl`, `expire_logs_days` (use `binlog_expire_logs_seconds`), `--skip-host-cache` (use `host_cache_size=0`), `binlog_transaction_dependency_tracking`, `master_info_repository`, `relay_log_info_repository`, `transaction_write_set_extraction`, `--old`, `--new`, `--language`, `--innodb`, `log_bin_use_v1_events`.
7. **Tools**: `mysql_upgrade` (the server upgrades on start; `--upgrade=NONE|MINIMAL|AUTO|FORCE`), `mysqlpump` (use `mysqldump` or MySQL Shell dump utilities), `mysql_ssl_rsa_setup` are gone. Prefer the MySQL Shell upgrade checker (`util.checkForServerUpgrade()`) before upgrading.
8. **Defaults**: performance tuning derived on 8.0 (`innodb_io_capacity`, adaptive hash index, change buffer, `temptable_*`) changed; re-measure instead of copying the old `my.cnf`. Remove settings for removed variables.
9. **Privileges**: scripts that grant `SET_USER_ID` fail; grant `SET_ANY_DEFINER`/`ALLOW_NONEXISTENT_DEFINER`. `FLUSH PRIVILEGES` needs `RELOAD` or the new `FLUSH_PRIVILEGES` privilege — and is unnecessary after `CREATE USER`/`GRANT`.
10. **Keep deprecated features out of new code** so the later 8.4 → 9.7 upgrade is mechanical: `VALUES()` in upserts, `SQL_CALC_FOUND_ROWS`, `utf8mb3`, `&&`/`||`, display widths, `ZEROFILL`, user-variable assignment in expressions, wildcard database grants, `MD5()`/`SHA1()` (removed into a component in 9.6).

***

#### Production / Review Checklist

##### Schema and migrations

- [ ] Every table has a narrow, immutable primary key (`BIGINT UNSIGNED AUTO_INCREMENT` or `BINARY(16)`), never `CHAR(36)` UUIDs
- [ ] `utf8mb4` with an explicit collation consistent with existing tables; no `utf8`/`utf8mb3`; `_bin` collation for case-sensitive identifiers
- [ ] `DECIMAL` for money; `BIGINT` for counters/IDs that grow; no display widths, `ZEROFILL`, `FLOAT(M,D)`
- [ ] `DATETIME(n)` in UTC for business timestamps; no `TIMESTAMP` beyond 2038; precision matches the application
- [ ] Foreign keys written as table-level `FOREIGN KEY` clauses (inline `REFERENCES` is ignored on 8.4) referencing a `PRIMARY`/`UNIQUE` key with identical column types
- [ ] No reliance on triggers firing for FK cascades; no `REPLACE` on parent tables
- [ ] `CHECK` constraints for invariants, with `NOT NULL` where `NULL` must not pass
- [ ] Every `ALTER TABLE` on a big table states `ALGORITHM` (and `LOCK`), runs with a short `lock_wait_timeout`, is a single statement per table, and is safe for the old and new application version during deploy
- [ ] No `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, `RETURNING`, `CREATE MATERIALIZED VIEW` or other MariaDB/PostgreSQL syntax
- [ ] `INSTANT` row-version budget (64) considered for frequently altered tables
- [ ] No 9.x syntax (`VECTOR`, `CREATE VIEW IF NOT EXISTS`, duality views, JavaScript routines, `GROUPING()` without `ROLLUP`) unless the project pins 9.7+; no unquoted identifiers that are reserved on 9.7 (`EXTERNAL`, `LIBRARY`)

##### Queries

- [ ] `EXPLAIN` (or `EXPLAIN ANALYZE` on a copy) for new queries on large tables; no full scans in hot paths
- [ ] No functions or implicit type conversions on indexed columns; strings bound for string columns
- [ ] Keyset pagination written with the expanded `OR` predicate, not `(a, b) > (?, ?)` and not deep `OFFSET`
- [ ] `NOT EXISTS` instead of `NOT IN (subquery)` on nullable columns
- [ ] Deterministic `ORDER BY` (ending in a unique column) wherever `LIMIT` or `ROW_NUMBER()` is used
- [ ] No `ONLY_FULL_GROUP_BY` workarounds (`sql_mode=''`, careless `ANY_VALUE`)
- [ ] `GROUP_CONCAT` length considered; `JSON_ARRAYAGG` for lists
- [ ] JSON functional indexes match the query expression and collation (`COLLATE utf8mb4_bin` for `->>`); multi-valued indexes only for `MEMBER OF`/`JSON_CONTAINS`/`JSON_OVERLAPS`
- [ ] Hints justified with a comment; `MAX_EXECUTION_TIME` or client timeouts on user-facing reads

##### Writes and transactions

- [ ] Upserts use `INSERT ... VALUES (...) AS new ON DUPLICATE KEY UPDATE col = new.col`; affected-rows semantics (1/2/0) handled; `AUTO_INCREMENT` gaps acceptable
- [ ] No `INSERT IGNORE` / `UPDATE IGNORE` except where ignoring every error class is intended
- [ ] Read-modify-write done in one `UPDATE`, under `SELECT ... FOR UPDATE`, or with optimistic version checks
- [ ] Locking statements use an index (REPEATABLE READ locks every scanned row and gap)
- [ ] Deadlock (1213) retries the whole transaction; lock wait timeout (1205) rolls back explicitly
- [ ] Transactions are short, contain no network calls or user waits, never contain DDL
- [ ] Large deletes/updates batched by primary key

##### Security

- [ ] All values bound as parameters; identifiers from allow-lists; `multiStatements` off; LIKE input escaped
- [ ] Application accounts with least privilege; roles activated via `SET DEFAULT ROLE`; no wildcard surprises in database grants
- [ ] `caching_sha2_password` everywhere; TLS required and verified; no `allowPublicKeyRetrieval`/`skip-verify` in production
- [ ] No `mysql_native_password`, no `--mysql-native-password=ON` as a permanent fix
- [ ] `local_infile` off; `secure_file_priv` set; no passwords hashed with `MD5`/`SHA1`/`SHA2`; no `AES_ENCRYPT` in ECB mode
- [ ] `DEFINER`/`SQL SECURITY` reviewed for views, routines, triggers, events

##### Operations

- [ ] Server pinned to a specific 8.4.x (≥ 8.4.13 for current security and optimizer fixes); upgrade path to 9.7 LTS planned
- [ ] Replication statements and monitoring use `SOURCE`/`REPLICA` terminology
- [ ] Connection pool lifetime below `wait_timeout`; `parseTime`/time-zone settings consistent
- [ ] Slow query log or `performance_schema` digests reviewed; deadlocks logged (`innodb_print_all_deadlocks`)
