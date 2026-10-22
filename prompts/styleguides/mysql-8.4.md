### MySQL 8.4 LTS: The Complete SQL Review Guide

#### Overview

> **Version**: MySQL 8.4 LTS. 8.4.0 became generally available on **April 30, 2024**.[^1][^2] The latest release is **8.4.13 (October 20, 2026)**; 8.4.12 (August 18, 2026) was a Critical Security Patch Update for the Docker image only.[^2][^3]
> **Support**: 8.4 is a Long-Term Support series under the Oracle Lifetime Support Policy: five years of Premier Support and three years of Extended Support,[^4] i.e. Premier Support until **April 2029** and Extended Support until **April 2032**.[^5]
> **Landscape as of October 2026**: MySQL 8.0 reached **End of Life in April 2026** with 8.0.46 (April 21, 2026).[^6] The next LTS, **9.7**, shipped on April 21, 2026 (latest 9.7.4, October 20, 2026).[^7] Innovation releases now use calendar version numbers: **26.7.0** (July 28, 2026) and **26.10.0** (October 20, 2026) are the newest Innovation releases.[^8] Supported upgrade path: 8.4 LTS → 9.7 LTS; an LTS series cannot be skipped.[^4]
> **Scope**: SQL that runs on MySQL 8.4 with InnoDB: schema and index design, migrations and online DDL, queries and DML in application code, transactions and locking, stored programs, security, and what changed relative to 8.0. Every example in this guide was executed on MySQL 8.4.13 (Community, official `mysql:8.4` image); boundary claims were checked on 8.0.46 and 9.7.4.
> **Note**: This document is self-contained. Sections that cover behaviour specific to the 8.1–8.4 line are marked **New in 8.4** (or with the exact 8.4.x patch).

This guide is also the one to apply when a project pins an **Innovation release between 8.1 and 9.6** (for example `mysql:8.3`, `mysql:9.2`). All of those releases are out of support (an Innovation release is supported only until the next one ships).[^4] Review such code against 8.4 LTS semantics, recommend moving to an LTS (8.4 or 9.7), and **treat every feature introduced in 9.0–9.7 or 26.x as unavailable unless the project pins 9.7 LTS or later** (see "Do not use or recommend"). Where a 9.x Innovation release already behaves differently from 8.4 in a way that breaks code, this guide says so.

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

MySQL 8.4.0 is the first LTS after 8.0 and bundles everything from the 8.1, 8.2 and 8.3 Innovation releases. LTS rules: features are added and removed only in the first LTS release (8.4.0); later 8.4.x releases contain fixes, and in-place upgrade and downgrade within 8.4.x is supported.[^4] In practice a few 8.4.x patches did change SQL-visible behaviour; they are listed separately below.

##### Breaking changes for application SQL and migrations [New in 8.4]

| Change | 8.0 behaviour | 8.4 behaviour |
|---|---|---|
| `mysql_native_password` | Enabled; `CREATE USER ... IDENTIFIED WITH mysql_native_password` works | Plugin is `DISABLED`; the same statement fails with `ERROR 1524: Plugin 'mysql_native_password' is not loaded`. Re-enable only with `--mysql-native-password=ON`[^1] |
| `default_authentication_plugin` | System variable exists | Removed (`ERROR 1193 Unknown system variable`); use `authentication_policy` (default `*,,`)[^1] |
| Replication SQL | `CHANGE MASTER TO`, `START/STOP SLAVE`, `SHOW SLAVE STATUS`, `SHOW MASTER STATUS`, `RESET MASTER`, `SHOW SLAVE HOSTS`, `PURGE MASTER LOGS`, `MASTER_*` options: deprecated with warning 1287 | All removed: `ERROR 1064` syntax error. Use `CHANGE REPLICATION SOURCE TO`, `START/STOP REPLICA`, `SHOW REPLICA STATUS`, `SHOW BINARY LOG STATUS`, `RESET BINARY LOGS AND GTIDS`, `SHOW REPLICAS`, `PURGE BINARY LOGS`, `SOURCE_*` options[^1] |
| Foreign key to a non-unique key | Accepted | Rejected by default: `ERROR 6125 Failed to add the foreign key constraint. Missing unique key for constraint ...` (`restrict_fk_on_non_standard_key=ON`)[^1] |
| `AUTO_INCREMENT` on `FLOAT`/`DOUBLE` | Deprecated (warning 3856) | `ERROR 1063 Incorrect column specifier`; such tables block the upgrade[^1] |
| `FLUSH HOSTS` | Deprecated | Syntax error; use `TRUNCATE TABLE performance_schema.host_cache`[^1] |
| `LOCK TABLES ... LOW_PRIORITY WRITE` | Deprecated | Syntax error[^1] |
| Unquoted identifiers starting with `$` and containing another `$` (`$a$b`) | Accepted | Syntax error; a single leading `$` (`$ab`) still works with deprecation warning 1681[^1] |
| Reserved words | — | `QUALIFY` and `TABLESAMPLE` are new **reserved** words (unquoted use as an identifier is `ERROR 1064`); `AUTO`, `BERNOULLI`, `GTIDS`, `LOG`, `MANUAL`, `PARALLEL`, `PARSE_TREE`, `S3` are new non-reserved keywords[^9] |
| `INFORMATION_SCHEMA.TABLESPACES` | Exists | Removed; use `INNODB_TABLESPACES` / `INNODB_DATAFILES`[^1] |
| Server options | `--ssl`, `--admin-ssl`, `have_ssl`, `have_openssl`, `--old`, `--new`, `--language`, `--skip-host-cache`, `--innodb`/`--skip-innodb`, `expire_logs_days`, `binlog_transaction_dependency_tracking`, `avoid_temporal_upgrade`, `show_old_temporals`, `log_bin_use_v1_events`, `master_info_repository`/`relay_log_info_repository`, `transaction_write_set_extraction`, `--character-set-client-handshake`, `--old-style-user-limits` | Removed; a config file that still sets them stops the server from starting[^1] |
| Utilities | `mysql_upgrade`, `mysqlpump`, `mysql_ssl_rsa_setup` | Removed; the server upgrades itself at startup and generates missing TLS/RSA files[^1] |
| Privileges | `RELOAD` covers `FLUSH PRIVILEGES`; `SET_USER_ID` | New `FLUSH_PRIVILEGES`, `OPTIMIZE_LOCAL_TABLE`, `TRANSACTION_GTID_TAG`, `SET_ANY_DEFINER`, `ALLOW_NONEXISTENT_DEFINER`; `SET_USER_ID` removed[^1] |

```sql
-- MySQL 8.4.13 (each statement fails; on 8.0.46 all of them work, most with warning 1287)
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

- **`EXPLAIN FORMAT=JSON INTO @var`** stores the plan in a user variable; `INTO` requires an explicit `FORMAT=JSON`.[^10]
- **`EXPLAIN ... FOR SCHEMA db`** (synonym `FOR DATABASE`) explains a statement as if it ran in another schema.[^10]
- **`explain_json_format_version`** (1 = default, 2 = access-path based). With version 2, `EXPLAIN ANALYZE FORMAT=JSON` works on 8.4.13 (on 8.0 it fails with `ERROR 1235 ... 'EXPLAIN ANALYZE with JSON format'`); the 8.4 manual still says JSON is never accepted with `EXPLAIN ANALYZE`, so verify on the pinned patch before relying on it.[^10]
- **Automatic histogram updates**: `ANALYZE TABLE t UPDATE HISTOGRAM ON col WITH n BUCKETS AUTO UPDATE` (default `MANUAL UPDATE`).[^1]
- **Hash-based `INTERSECT` / `EXCEPT`** (`hash_set_operations` optimizer switch, `set_operations_buffer_size`). `INTERSECT` and `EXCEPT` themselves exist since 8.0.31.[^1]
- **Correlated scalar subqueries** can be transformed into derived tables (`subquery_to_derived`, off by default).[^1]
- **`GROUP BY ROLLUP (a, b)`**: alternative spelling of `GROUP BY a, b WITH ROLLUP`, **added in 8.4.3** (WL#15843; also in 9.1.0). It is a syntax error on 8.0 and on 8.4.0–8.4.2.[^11]
- **Tagged GTIDs** (`SET gtid_next = 'AUTOMATIC:tag'`), `SHOW BINARY LOG STATUS`, `SHOW REPLICAS`.[^1]
- **`VERSION_WITH_UPDATE()`** and `@@version_with_update` return a four-part version (`8.4.13.0`) **since 8.4.13**; `VERSION()` keeps returning three parts.[^3]

```sql
-- MySQL 8.4.13
EXPLAIN FORMAT=JSON INTO @plan SELECT * FROM users WHERE id = 1;
SELECT JSON_EXTRACT(@plan, '$.query_block.table.access_type');   -- "const"

SELECT y, c, SUM(amt) FROM sales GROUP BY ROLLUP (y, c);          -- 8.4.3+
ANALYZE TABLE orders UPDATE HISTOGRAM ON total WITH 32 BUCKETS AUTO UPDATE;
SELECT VERSION(), VERSION_WITH_UPDATE();                          -- 8.4.13 | 8.4.13.0
```

##### Changed server defaults [New in 8.4]

These do not change SQL semantics but explain performance differences after an 8.0 → 8.4 upgrade. Values observed on 8.0.46 vs 8.4.13 in the same 4-CPU container:[^1]

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
| `group_replication_consistency` | `EVENTUAL` | `BEFORE_ON_PRIMARY_FAILOVER` |
| `SOURCE_RETRY_COUNT` | 86400 | 10 |

Unchanged and still relevant for review: `transaction_isolation=REPEATABLE-READ`, `binlog_format=ROW`, `innodb_autoinc_lock_mode=2`, `innodb_lock_wait_timeout=50`, `lock_wait_timeout=31536000` (one year), `explicit_defaults_for_timestamp=ON`, `local_infile=OFF`, `activate_all_roles_on_login=OFF`, `group_concat_max_len=1024`, `cte_max_recursion_depth=1000`, `block_encryption_mode=aes-128-ecb`, `explain_format=TRADITIONAL`, `gtid_mode=OFF`.

##### Behaviour changes inside the 8.4 LTS series

An LTS is meant to be stable, but these patch releases changed behaviour a reviewer can observe:

| Release | Change |
|---|---|
| 8.4.3 | `GROUP BY ROLLUP (...)` syntax added; `mysql` client gains `--system-command` (on by default)[^11] |
| 8.4.4 | `ALTER TABLE ... ADD/DROP COLUMN` on an **empty** table defaults to `INPLACE` instead of `INSTANT` (no row version is consumed); incompatible fix for spatial-index corruption — drop and re-create spatial indexes around the upgrade[^12] |
| 8.4.5 | New `--check-table-functions=WARN|ABORT` (default `ABORT`): an upgrade aborts if a constraint, default, partition expression or generated column uses a function that now raises an error[^13] |
| 8.4.6 | `mysql` client `--commands` option (on by default)[^14] |
| 8.4.11 | `MANUAL` and `PARALLEL` become **non-reserved** (they were reserved in 8.4.0–8.4.10); `QUALIFY`, `TABLESAMPLE`, `CUBE` are reserved; some `CREATE`/`ALTER TABLE` statements that exceed documented InnoDB row-size limits are now rejected (`ALGORITHM=INSTANT` uses the stricter calculation)[^15] |
| 8.4.13 | Four-part version (`VERSION_WITH_UPDATE()`); `ALTER TABLE ... EXCHANGE PARTITION` now validates rows against the destination table's `CHECK` constraints in both directions (`ERROR 3819`), even `WITHOUT VALIDATION`; fix for `index_merge_union=off` plans that could drop a disjunct and return incomplete results[^3] |

Run the latest 8.4.x: 8.4.13 alone fixes an optimizer bug that returned incomplete results and hardens protocol parsing against several security issues.[^3]

##### Version-aware review

- Determine the exact server version from the project (Docker image tag, `docker-compose.yml`, Helm values, Terraform `engine_version`, CI service containers, `mysql --version` in scripts). `mysql:8.4` floats to the newest 8.4.x; `mysql:8.4.2` does not have `GROUP BY ROLLUP (...)`.
- Before claiming that syntax or a function is unavailable, check it against the 8.4 manual or the release notes of the pinned patch. Say "not available in MySQL 8.4" only when the pinned version proves it; many PostgreSQL-isms are genuinely missing, but `INTERSECT`, `EXCEPT`, `LATERAL`, `JSON_TABLE`, `VALUES ROW(...)`, window functions, recursive CTEs, `CHECK` constraints, functional and descending indexes all exist.
- Managed services lag or diverge: Amazon RDS / Aurora, Azure, Google Cloud SQL and HeatWave each document their own 8.4.x versions and parameter defaults. HeatWave-only features (secondary engines, `QUALIFY` with the hypergraph optimizer, `TABLESAMPLE`) are not available on Community or Enterprise Server.
- MariaDB is not MySQL. `RETURNING`, `CREATE OR REPLACE TABLE`, `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, sequences and system-versioned tables are MariaDB features; on MySQL 8.4 they are syntax errors.
- When a project pins a 9.x Innovation release, also check the "already different in 9.x" notes in the next section before approving code that relies on 8.4 behaviour.

##### Do not use or recommend (features newer than 8.4)

The server does not have these. Never suggest them for code that runs on 8.4 (or on an Innovation release older than the one that introduced them), and flag them as bugs when they appear:

| Feature | Introduced | On 8.4.13 |
|---|---|---|
| `VECTOR(n)` type, `STRING_TO_VECTOR()`/`TO_VECTOR()`, `VECTOR_TO_STRING()`/`FROM_VECTOR()`, `VECTOR_DIM()` | 9.0.0[^16] | `ERROR 1064` / `ERROR 1305 FUNCTION ... does not exist` |
| `EXPLAIN ANALYZE FORMAT=JSON INTO @var` | 9.0.0[^16] | `ERROR 1064` |
| `PREPARE` of `CREATE/ALTER/DROP EVENT` | 9.0.0[^16] | `ERROR 1295 This command is not supported in the prepared statement protocol yet` |
| JavaScript stored programs (`LANGUAGE JAVASCRIPT`), `CREATE LIBRARY`, `USING (lib)` | 9.0.0 / 9.2.0 (Enterprise)[^16][^17] | not available |
| `CREATE VIEW IF NOT EXISTS` | 9.1.0[^18] | `ERROR 1064`; use `CREATE OR REPLACE VIEW` |
| `CREATE_SPATIAL_REFERENCE_SYSTEM` privilege | 9.2.0[^17] | use `SUPER` |
| `CREATE JSON DUALITY VIEW`, `JSON_DUALITY_OBJECT()`, `ETAG()`, DML on duality views | 9.4.0 (DML in Community: 9.7.0)[^19][^20] | `ERROR 1064` |
| `activate_mandatory_roles` variable | 9.5.0[^21] | `ERROR 1193` |
| `INTERPRET_UTF8_AS_UTF8MB4` SQL mode | 9.5.0[^21] | `ERROR 1231` |
| `GROUPING()` in a query without `ROLLUP` | 9.6.0[^22] | `ERROR 1111 Invalid use of group function` |
| `innodb_native_foreign_keys`, SQL-layer foreign keys | 9.6.0[^22] | not available |
| Hypergraph optimizer in Community (`optimizer_switch='hypergraph_optimizer=on'`) and therefore `QUALIFY` | 9.7.0[^20] | `ERROR 3999 ... does not yet support 'use in non-debug builds'`; `QUALIFY` → `ERROR 6037` |
| `enable_cascade_triggers` (FK cascades fire triggers) | 9.7.0[^20] | `ERROR 1193` |
| `CURRENT_ROLE_IN()`, `CURRENT_USER_IN()`, `CREATE MASKING POLICY` | 9.7.0 (Enterprise)[^20] | not available |
| `caching_sha2_password` PBKDF2 storage format | 9.7.0[^20] | not available |
| `replica_allow_higher_version_source` | 9.7.0[^20] | not available |

Not available in any MySQL version up to 9.7.4 either (verified on 8.4.13 and 9.7.4): `INSERT ... RETURNING`, `IS [NOT] DISTINCT FROM` (use `<=>`), `FULL OUTER JOIN`, `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, range access for row-constructor inequalities such as `(a, b) > (?, ?)`.

**Already different in 9.x Innovation releases** (relevant when a project pins 9.0–9.6 and this guide is applied):

- 9.0.0 removes `mysql_native_password` from the server entirely and removes `default_authentication_plugin`.[^16]
- 9.0.0 **enforces inline column `REFERENCES`** clauses. On 8.4 they are parsed and **silently ignored** (see Foreign keys). Code that "worked" on 8.4 may start failing with `ERROR 1452` on 9.x.[^16]
- 9.0.0: `IGNORE` no longer suppresses `ER_SUBQUERY_NO_1_ROW`.[^16]
- 9.1.0: column references through a derived table qualified with the schema (`SELECT db.dt.a FROM (...) AS dt`) are rejected; `ROLLUP` over an empty table returns a grand-total row `(NULL, 0)` (8.4 returns an empty set); `SUM(DISTINCT b'1100')` returns 12 (8.4.13 returns 9).[^18]
- 9.2.0: `SELECT (SELECT 1 LIMIT 1 OFFSET 10)` returns `NULL` (8.4.13 returns 1); `FLUSH PRIVILEGES` is deprecated; `BETWEEN` string literals must match the operand's collation; outer references in window `PARTITION BY`/`ORDER BY` are rejected.[^17]
- 9.3.0 removes `innodb_log_file_size`, `innodb_log_files_in_group` and `innodb_undo_tablespaces` (still accepted, deprecated, on 8.4).[^23]
- 9.4.0 deprecates `MD5()` and `SHA1()`; 9.6.0 moves them into the `classic_hashing` component (on 9.7.4 without that component: `ERROR 1305 FUNCTION MD5 does not exist`).[^19][^22]
- 9.5.0 changes defaults: `explain_format=TREE`, `explain_json_format_version=2`, `gtid_mode=ON`, `innodb_change_buffering=all`, `caching_sha2_password_digest_rounds=10000`.[^21]

##### Deprecated in 8.4 (works, but flag it)

Each of these produces a warning on 8.4.13; code that depends on them breaks on a later LTS:

- `VALUES(col)` in `ON DUPLICATE KEY UPDATE` (warning 1287, since 8.0.20) → row alias `INSERT ... VALUES (...) AS new ON DUPLICATE KEY UPDATE c = new.c`.
- `SQL_CALC_FOUND_ROWS` / `FOUND_ROWS()` (warning 1287) → separate `COUNT(*)`.
- `&&`, `||` as `OR`, `!` as `NOT` (warning 1287) → `AND`, `OR`, `NOT`.
- Assigning user variables inside expressions (`SELECT @y := 2`, warning 1287) → `SET` or `SELECT ... INTO`.
- Integer display width (`INT(11)`), `ZEROFILL`, `FLOAT(M,D)`/`DOUBLE(M,D)` (warning 1681).
- The `utf8` / `utf8mb3` character set (warning 3719) → `utf8mb4`.
- `$` as the first character of an unquoted identifier (warning 1681).
- Wildcards `%` and `_` in database names of `GRANT` statements (deprecated since 8.2.0; no warning is printed on 8.4.13).[^1]
- `mysql_native_password` (deprecated and disabled), `innodb_log_file_size`/`innodb_log_files_in_group` (use `innodb_redo_log_capacity`).

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

Flag migrations that create tables without a primary key. On 8.4, `sql_require_primary_key` does not accept a `UNIQUE NOT NULL` key as a substitute (9.7.0 relaxes this).[^20]

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

Since 8.4.13, `ALTER TABLE ... EXCHANGE PARTITION` checks the swapped rows against the destination's `CHECK` constraints even with `WITHOUT VALIDATION`.[^3]

##### Foreign keys

InnoDB enforces foreign keys, with these 8.4-specific traps:

1. **Inline `REFERENCES` on a column definition is silently ignored.** Only a table-level `FOREIGN KEY (...) REFERENCES ...` clause (or `CONSTRAINT name FOREIGN KEY ...`) creates a constraint. MySQL 9.0 started enforcing the inline form,[^16] so the same DDL behaves differently on 8.4 and 9.x.

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
3. **Cascaded changes do not fire triggers.** `ON DELETE CASCADE` / `ON UPDATE CASCADE` / `SET NULL` changes child rows without running the child table's triggers; audit triggers on child tables miss them (9.7.0 adds an opt-in `enable_cascade_triggers`).[^24][^20]

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
7. Column types must match exactly (signedness, length for strings, character set and collation): `ERROR 3780 Referencing column ... and referenced column ... are incompatible`.

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

The default collation is case-insensitive, accent-insensitive and **NO PAD** (trailing spaces are significant). Older collations (`utf8mb4_general_ci`, `utf8mb4_unicode_ci`, `utf8mb4_bin`) are `PAD SPACE`.[^25]

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

`CAST(... AT TIME ZONE ...)` only accepts a `TIMESTAMP` value and the zone `'+00:00'`/`'UTC'`. Named zones (`'Europe/Berlin'`) in `CONVERT_TZ()` and `SET time_zone` need the time-zone tables to be loaded (`mysql_tzinfo_to_sql`); otherwise `CONVERT_TZ()` returns `NULL` silently. The official Docker image ships them loaded; many installations do not. Prefer fixed offsets or conversion in the application, and check `SELECT COUNT(*) FROM mysql.time_zone_name` when code relies on named zones.

```sql
SELECT CONVERT_TZ('2024-03-31 01:30:00', 'UTC', 'Europe/Berlin');   -- 2024-03-31 03:30:00 (tables loaded)
SELECT '2024-01-31' + INTERVAL 1 MONTH;                              -- 2024-02-29 (clamped to month end)
SELECT UNIX_TIMESTAMP('2038-01-19 03:14:08'), FROM_UNIXTIME(2147483648);   -- 64-bit range works
```

- `NOW()` is constant for the statement; `SYSDATE()` is evaluated per call and is unsafe for statement-based replication. Use `NOW()`.
- Do not filter with `DATE(col) = ?` — use a half-open range (see Indexing).
