### MySQL 8.0 (and 5.7): The Complete SQL Review Guide

#### Overview

> **Version**: MySQL 8.0. The first GA release, 8.0.11, shipped on **April 19, 2018**.[^1] The final release is **8.0.46 (April 21, 2026)**.[^2]
> **Support status: END OF LIFE.** "As of April 2026, with version 8.0.46, MySQL 8.0 reaches End of Life (EoL)."[^2] Premier Support ended in April 2025 and Extended Support in April 2026;[^3] no further bug or security fixes are published for 8.0. **MySQL 5.7** reached End of Life in **October 2023**; its last release is 5.7.44 (October 25, 2023).[^3][^4]
> **Landscape as of October 2026**: the supported LTS series are **8.4** (latest 8.4.13, October 20, 2026; Premier Support until April 2029) and **9.7** (latest 9.7.4, October 20, 2026; Premier Support until April 2031).[^5][^6] Innovation releases after 9.7 use calendar versions; the newest is **26.10.0** (October 20, 2026). An LTS (or bugfix) series cannot be skipped when upgrading: the supported path is **8.0 → 8.4 LTS → 9.7 LTS**, and 5.7 must go through 8.0 first (**5.7 → 8.0 → 8.4**).[^7][^51] Recommend planning the move to 8.4 in every review of an 8.0 or 5.7 project, while still judging the code against the version it actually runs on.
> **Scope**: SQL that runs on MySQL 8.0 (and, where marked, 5.7) with InnoDB: schema and index design, migrations and online DDL, queries and DML in application code, transactions and locking, stored programs, security, and the differences to 5.7 and 8.4. Every example was executed on **MySQL 8.0.46** (Community, official `mysql:8.0` image). Boundary claims were checked on **5.7.44** (official `mysql:5.7` image) and **8.4.13**.
> **Note**: This document is self-contained. Features that 5.7 lacks are marked **8.0+** (with the 8.0.x release that introduced them, e.g. **8.0.16+**), so the guide also serves projects pinned to MySQL 5.7 or to an early 8.0 patch release.

This guide applies to every MySQL version **below 8.1**: 8.0.x and 5.7.x (and, loosely, 5.6 and older, which lack even more). For a 5.7 project, every feature marked **8.0+** is unavailable and must not be suggested; the "MySQL 5.7 differences" notes in each section say what behaves differently there.

Global truths about MySQL 8.0 with InnoDB:

- **The table is the primary key.** InnoDB stores rows in a B-tree clustered on the primary key; every secondary index entry carries the full primary key. A wide or random primary key makes every index bigger and every insert slower.
- **REPEATABLE READ is the default isolation level.** Plain `SELECT`s read a consistent snapshot; `UPDATE`, `DELETE` and locking reads (`FOR UPDATE`, `FOR SHARE`/`LOCK IN SHARE MODE`) read the latest committed version and take next-key (record + gap) locks on every index record they scan.
- **DDL is not transactional.** Every DDL statement implicitly commits the current transaction. Since 8.0 a single DDL statement is atomic (crash-safe, all-or-nothing); on 5.7 it is not. A migration of several statements is never atomic.
- **Strict mode is on by default** (8.0 and 5.7): `sql_mode` is `ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION` (5.7 adds `NO_AUTO_CREATE_USER`). `INSERT IGNORE`, `UPDATE IGNORE` and sessions that clear `sql_mode` silently turn errors back into truncation warnings.
- **`utf8mb4` / `utf8mb4_0900_ai_ci` is the 8.0 default** (5.7: `latin1` / `latin1_swedish_ci`). `utf8` means the deprecated 3-byte `utf8mb3` in every version.
- **`caching_sha2_password` is the 8.0 default authentication plugin** (5.7: `mysql_native_password`). `mysql_native_password` still works on 8.0 but is deprecated (8.0.34) and disabled by default on 8.4.
- **No `RETURNING`, no `MERGE`, no `FULL OUTER JOIN`, no `IS DISTINCT FROM`, no `ADD COLUMN IF NOT EXISTS`, no partial or `INCLUDE` indexes, no transactional DDL, no `QUALIFY`.** Code or advice copied from PostgreSQL or MariaDB does not run.

***

#### What's New in MySQL 8.0 (since 5.7)

MySQL 8.0 was the last series under the old "continuous delivery" model: new SQL features arrived in patch releases from 8.0.11 to 8.0.33; from 8.0.34 on, 8.0 "transition[ed] to only bug fixes until 8.0 End-Of-Life" (the 8.0.34 and 8.0.35 releases still added deprecations).[^50] A project pinned to an early 8.0 patch therefore lacks features that later 8.0 releases have. The table lists what a reviewer most often needs, with the release that introduced each item.[^8]

##### SQL features added in the 8.0 series [8.0+]

| Feature | Since | On 5.7.44 |
|---|---|---|
| Common table expressions, `WITH RECURSIVE` | 8.0.11 (GA) | `ERROR 1064` |
| Window functions (`ROW_NUMBER()`, `RANK()`, `LAG()`, `SUM() OVER`, `WINDOW w AS`) | 8.0.11 (GA) | `ERROR 1064` |
| `SELECT ... FOR UPDATE SKIP LOCKED` / `NOWAIT`, `FOR SHARE`, `FOR UPDATE OF tbl` | 8.0.11 (GA) | `ERROR 1064` (`LOCK IN SHARE MODE` works) |
| Roles (`CREATE ROLE`, `SET DEFAULT ROLE`) | 8.0.11 (GA) | `ERROR 1064` |
| Descending indexes (`KEY (a DESC)`) | 8.0.11 (GA) | parsed and **ignored** (an ascending index is built) |
| Invisible indexes | 8.0.11 (GA) | `ERROR 1064` |
| `caching_sha2_password` as default plugin | 8.0.11 (GA) | plugin does not exist |
| `utf8mb4` / `utf8mb4_0900_ai_ci` defaults | 8.0.11 (GA) | `ERROR 1273 Unknown collation: 'utf8mb4_0900_ai_ci'` |
| Atomic DDL, transactional data dictionary | 8.0.11 (GA) | `DROP TABLE a, missing` drops `a` and then fails |
| Persistent `AUTO_INCREMENT` counter (survives restart) | 8.0.11 (GA) | counter recomputed as `MAX(id)+1` at restart (ids are reused) |
| `JSON_TABLE()` | 8.0.11 (GA) | `ERROR 1064` |
| Histograms (`ANALYZE TABLE ... UPDATE HISTOGRAM`) | 8.0.11 (GA) | not available |
| `SET_VAR` optimizer hint | 8.0.11 (GA) | warning 1064 "Optimizer hint syntax error"; hint ignored |
| `REGEXP_LIKE()`, `REGEXP_REPLACE()` etc. (ICU, multibyte-safe) | 8.0.11 (GA) | `ERROR 1305` |
| `UUID_TO_BIN()`, `BIN_TO_UUID()` | 8.0.11 (GA) | `ERROR 1305` |
| `ALGORITHM=INSTANT` `ADD COLUMN` (last position only) | **8.0.12** | `ERROR 1800 Unknown ALGORITHM 'INSTANT'` |
| Functional (expression) index key parts `KEY ((expr))` | **8.0.13** | `ERROR 1064` |
| Expression defaults `DEFAULT (expr)`, incl. for `BLOB`/`TEXT`/`JSON` | **8.0.13** | `ERROR 1064` |
| Index skip scan | **8.0.13** | — |
| `sql_require_primary_key` | **8.0.13** | `ERROR 1193` |
| `LATERAL` derived tables | **8.0.14** | `ERROR 1064` |
| `CHECK` constraints **enforced** | **8.0.16** | parsed and **silently ignored** |
| `EXPLAIN FORMAT=TREE` | **8.0.16** (experimental until 8.0.18) | `ERROR 1791 Unknown EXPLAIN format name` |
| Multi-valued indexes, `MEMBER OF()`, `JSON_OVERLAPS()`, `JSON_SCHEMA_VALID()` | **8.0.17** | `ERROR 1064` / `ERROR 1305` |
| `EXPLAIN ANALYZE`, hash join (inner equi-joins)[^49] | **8.0.18** | `ERROR 1064`; block nested loop |
| Row alias in `INSERT ... VALUES (...) AS new ON DUPLICATE KEY UPDATE` | **8.0.19** | `ERROR 1064` |
| `TABLE t`, `VALUES ROW(...)` statements; `LIMIT` in recursive CTEs | **8.0.19** | `ERROR 1064` |
| Datetime literals with offsets (`'2024-06-01 12:00:00+05:00'`) | **8.0.19** | `ERROR 1292` on insert; `CAST` truncates the offset with warning 1292 |
| `FAILED_LOGIN_ATTEMPTS` / `PASSWORD_LOCK_TIME` | **8.0.19** | not available |
| `INDEX`/`NO_INDEX`/`JOIN_INDEX`/`ORDER_INDEX`/`GROUP_INDEX` hints; hash join for all former BNL joins | **8.0.20** | — |
| `JSON_VALUE()`; atomic `CREATE TABLE ... SELECT` (GTID-safe) | **8.0.21** | `ERROR 1305` |
| `START REPLICA`, `SHOW REPLICA STATUS`, `SHOW REPLICAS`; `CAST(ts AT TIME ZONE '+00:00' AS DATETIME)`; derived condition pushdown; `performance_schema.processlist` | **8.0.22** | `ERROR 1064` |
| Invisible columns; `CHANGE REPLICATION SOURCE TO` | **8.0.23** | `ERROR 1064` |
| 64-bit `UNIX_TIMESTAMP()`/`FROM_UNIXTIME()`/`CONVERT_TZ()` (beyond 2038); `RENAME COLUMN ... ALGORITHM=INSTANT` | **8.0.28** | 32-bit range |
| `ALGORITHM=INSTANT` `ADD COLUMN` at any position and `DROP COLUMN`; 64 row versions per table | **8.0.29** (withdrawn; use ≥ 8.0.30) | — |
| Generated invisible primary keys (`sql_generate_invisible_primary_key`), `innodb_redo_log_capacity` | **8.0.30** | — |
| `INTERSECT`, `EXCEPT` | **8.0.31** | `ERROR 1064` |
| `explain_format` system variable | **8.0.32** | — |
| `DEFAULT (CURRENT_USER())` and similar user functions as column defaults | **8.0.34** | — |

Features that arrived in the 8.0.0–8.0.4 development milestones and release candidates are listed as 8.0.11, the first production release. Use the minimum version the project pins. For a project that floats on `mysql:8.0` the image is 8.0.46, where every row above is available.

##### Breaking changes for application SQL when moving 5.7 → 8.0

These are the 5.7 habits that fail or change meaning on 8.0 (all verified on 5.7.44 vs 8.0.46):[^8][^9]

| 5.7 code | 8.0 behaviour |
|---|---|
| `GROUP BY a DESC` / `GROUP BY a ASC` | `ERROR 1064` (removed in 8.0.13; 5.7 warns 1287) |
| Relying on `GROUP BY` sorting its output | `GROUP BY` no longer sorts implicitly; add `ORDER BY` |
| `GRANT ... TO new_user IDENTIFIED BY 'x'` creating users | `ERROR 1064` (syntax error: `IDENTIFIED BY` is gone from `GRANT`); a plain `GRANT` to a missing account is `ERROR 1410 You are not allowed to create a user with GRANT`; use `CREATE USER` then `GRANT` |
| `PASSWORD('x')`, `ENCODE()`, `DECODE()`, `ENCRYPT()`, `DES_ENCRYPT()` | removed: `PASSWORD('x')` is `ERROR 1064`, the others `ERROR 1305 FUNCTION ... does not exist` |
| `SET sql_mode = '...,NO_AUTO_CREATE_USER,...'` (common in old dumps and ORMs) | `ERROR 1231 Variable 'sql_mode' can't be set to the value of 'NO_AUTO_CREATE_USER'` |
| Query cache (`SQL_CACHE`, `query_cache_*` variables) | removed; `SQL_NO_CACHE` accepted with a deprecation warning |
| Unquoted new reserved words as identifiers: `CUME_DIST`, `DENSE_RANK`, `EMPTY`, `EXCEPT`, `FIRST_VALUE`, `GROUPING`, `GROUPS`, `INTERSECT`, `JSON_TABLE`, `LAG`, `LAST_VALUE`, `LATERAL`, `LEAD`, `NTH_VALUE`, `NTILE`, `OF`, `OVER`, `PERCENT_RANK`, `RANK`, `RECURSIVE`, `ROW_NUMBER`, `SYSTEM`, `WINDOW`, and `CUBE` (reserved since 8.0.1; `CREATE TABLE cube (...)` fails on 8.0.46)[^10] | `ERROR 1064`; quote them (`` `rank` ``) |
| `information_schema.innodb_locks` / `innodb_lock_waits` | removed; use `performance_schema.data_locks` / `data_lock_waits` |
| `EXPLAIN EXTENDED`, `EXPLAIN PARTITIONS` | removed (the information is always shown) |
| `\N` as `NULL` in SQL, `PROCEDURE ANALYSE()`, `JSON_APPEND()` | removed |
| `innodb_file_format`, `innodb_large_prefix`, `tx_isolation`, `tx_read_only` in `my.cnf` or `SET` | removed: the server refuses to start / `ERROR 1193`; use `transaction_isolation` |
| Clients without `caching_sha2_password` support | login fails for new accounts; see Security |
| `TIMESTAMP` columns without explicit defaults | `explicit_defaults_for_timestamp=ON`: no implicit `DEFAULT CURRENT_TIMESTAMP ON UPDATE ...`, columns are nullable |
| Character set and collation not specified in DDL | new tables get `utf8mb4_0900_ai_ci` instead of `latin1`; joins against old `utf8mb4_general_ci`/`utf8mb4_unicode_ci` columns fail with `ERROR 1267` (see Character Sets) |
| `CHECK (...)` clauses written years ago "for documentation" | **enforced since 8.0.16**: existing violating rows make the next `ALTER`/reload fail, new violating inserts error |

```sql
-- 5.7.44:
SELECT a FROM gb GROUP BY a DESC;              -- works, Warning 1287 'GROUP BY with ASC/DESC' is deprecated
CREATE TABLE ck57 (a INT, CHECK (a > 0));
INSERT INTO ck57 VALUES (-1);                  -- accepted: CHECK is parsed and ignored on 5.7
-- 8.0.46:
SELECT a FROM gb GROUP BY a DESC;              -- ERROR 1064
INSERT INTO ck57 VALUES (-1);                  -- ERROR 3819 (HY000): Check constraint 'ck57_chk_1' is violated.
SET SESSION sql_mode = 'NO_AUTO_CREATE_USER';  -- ERROR 1231
GRANT SELECT ON t.* TO nosuchuser@'%';         -- ERROR 1410 (42000): You are not allowed to create a user with GRANT
```

##### Changes inside the 8.0 series that affect review

| Release | Change |
|---|---|
| 8.0.13 | `GROUP BY ... ASC/DESC` removed; assigning user variables in expressions (`SELECT @x := ...`) deprecated (warning 1287)[^11] |
| 8.0.16 | `CHECK` constraints enforced (previously ignored); the server upgrades the data dictionary itself, `mysql_upgrade` deprecated[^12] |
| 8.0.17 | Deprecated: integer display width (`INT(11)`), `ZEROFILL`, `FLOAT(M,D)`, `AUTO_INCREMENT` on `FLOAT`/`DOUBLE`, `&&`/`||`/`!`, `SQL_CALC_FOUND_ROWS`/`FOUND_ROWS()`[^8] |
| 8.0.19 | `SHOW CREATE TABLE` and `INFORMATION_SCHEMA.COLUMNS` stop printing integer display widths (`int` instead of `int(11)`; `tinyint(1)` and `ZEROFILL` columns keep it); row alias for upserts, `TABLE`/`VALUES` statements, datetime literals with offsets[^46] |
| 8.0.20 | `VALUES(col)` in `ON DUPLICATE KEY UPDATE` deprecated; hash join replaces block nested loop everywhere[^13] |
| 8.0.22 | `MASTER`/`SLAVE` replication statements deprecated in favour of `REPLICA` aliases; prepared statements are prepared once at `PREPARE` (parameter type resolution changed: `ORDER BY ?` no longer means "order by column number")[^14] |
| 8.0.23 | `CHANGE MASTER TO` deprecated (`CHANGE REPLICATION SOURCE TO`, `SOURCE_*` options); `FLUSH HOSTS` deprecated[^15] |
| 8.0.26 | Replication system variables, status variables and options get `replica_*`/`source_*` names (`replica_parallel_workers`, `SOURCE_POS_WAIT()`); old names deprecated; Performance Schema instrument and thread-stage names change incompatibly (`terminology_use_previous=BEFORE_8_0_26` restores them)[^47] |
| 8.0.27 | `default_authentication_plugin` deprecated (replaced by `authentication_policy`); `replica_parallel_workers=4` and `replica_preserve_commit_order=ON` become defaults (multithreaded replica applier on by default)[^48] |
| 8.0.28 | 64-bit `UNIX_TIMESTAMP()`/`FROM_UNIXTIME()`/`CONVERT_TZ()`; `ASCII`/`UNICODE` charset shorthands and `ucs2`, `macroman`, `macce`, `dec`, `hp8` deprecated; TLSv1/TLSv1.1 removed[^16] |
| 8.0.29 | INSTANT `ADD COLUMN` anywhere / `DROP COLUMN`; **release withdrawn** ("a critical issue that could cause data in InnoDB tables having added columns to be interpreted incorrectly. Please upgrade to MySQL 8.0.30 instead")[^17] |
| 8.0.30 | GIPK mode; `innodb_redo_log_capacity` replaces `innodb_log_file_size`/`innodb_log_files_in_group` (deprecated); `--skip-host-cache` deprecated[^8] |
| 8.0.31 | `INTERSECT`/`EXCEPT`[^8] |
| 8.0.32 | `$` as the first character of unquoted identifiers deprecated (warning 1681); unquoted `FULL` as an identifier deprecated (warning 4119); `explain_format` added[^18] |
| 8.0.34 | `mysql_native_password` deprecated (logged in the error log, not as an SQL warning); `binlog_format`, `log_bin_trust_function_creators` deprecated (warning 1287 on every read or `SET`); `mysqlpump`, `mysql_ssl_rsa_setup`, `keyring_file` deprecated; a version comment whose number is not followed by whitespace (`/*!80034KEY_BLOCK_SIZE=1024*/`) is deprecated (warning 4164; write `/*!80034 KEY_BLOCK_SIZE=1024*/`)[^19] |
| 8.0.35 | `INFORMATION_SCHEMA.PROCESSLIST` deprecated (warning 1287); wildcards `%`/`_` in database-level grants and `%` as a synonym for `localhost` deprecated (no warning is raised); `binlog_transaction_dependency_tracking` deprecated; `STR_TO_DATE()` range-checks dates[^20] |
| 8.0.38 | **Release withdrawn**: "removed due to a critical issue that could stop the server from restarting following the creation of a very large number of tables (8001 or more). Please upgrade to MySQL 8.0.39 instead"; `mysql_native_password` deprecation warning is written to the error log only once instead of per authentication[^52] |
| 8.0.41 | `ADD COLUMN`/`DROP COLUMN` **without an `ALGORITHM` clause on an empty table** uses `INPLACE` and no longer consumes a row version (8.0.40 still consumed one: `total_row_versions` 1 after `ADD`, 2 after `DROP`); incompatible fix for spatial-index corruption (drop and re-create spatial indexes around the upgrade)[^21] |
| 8.0.42 | `--check-table-functions=WARN|ABORT` (default `ABORT`): an upgrade aborts when a constraint, default, partitioning expression or generated column uses a function that now raises an error; `ALTER TABLE ... INPLACE` rejects indexes over 767 bytes on `REDUNDANT`/`COMPACT` tables[^22] |
| 8.0.46 | Final release: fix for semi-join materialization queries that "may return incorrect results due to a missing condition"; OpenSSL 3.5.5[^2] |

Run 8.0.46 if the project must stay on 8.0 for now: it is the last build with any fixes.

##### Version-aware review

- Determine the exact server version from the project (Docker image tag, `docker-compose.yml`, Helm values, Terraform `engine_version`, CI service containers). `mysql:8.0` floats to 8.0.46; a tag such as `mysql:8.0.28` lacks INSTANT `DROP COLUMN`, GIPK and `INTERSECT`; `mysql:5.7` lacks everything marked **8.0+**.
- Before claiming that syntax or a function is unavailable, check this guide's "Since" column or the release notes of the pinned patch. Say "not available in MySQL 8.0.x" only when the pinned version proves it. Many PostgreSQL-isms are genuinely missing, but on current 8.0 `INTERSECT`, `EXCEPT`, `LATERAL`, `JSON_TABLE`, `VALUES ROW(...)`, window functions, recursive CTEs, `CHECK` constraints, functional and descending indexes all exist.
- On 5.7: do not suggest CTEs, window functions, `SKIP LOCKED`, roles, functional indexes, `JSON_TABLE`, `CHECK` (ignored!), `ALGORITHM=INSTANT`, `EXPLAIN ANALYZE` or any other **8.0+** item; propose the 5.7-compatible form (derived tables, user-variable-free self joins, generated columns with an index instead of a functional index, application-side checks or triggers instead of `CHECK`) and recommend the upgrade.
- Managed services lag or diverge: Amazon RDS / Aurora MySQL 3 (8.0-compatible), Azure, Cloud SQL each document their own versions, defaults and extended-support programs; Aurora's version numbers are not MySQL's.
- MariaDB is not MySQL: `RETURNING`, `CREATE OR REPLACE TABLE`, `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, sequences and system-versioned tables are MariaDB features; on MySQL 8.0 they are syntax errors.
- Code that must also run on 8.4 (the upgrade target) should avoid everything in "Deprecated in 8.0" and "Removed / changed in 8.4" below.

##### Do not use or recommend (features newer than 8.0)

The 8.0 server does not have these. Never suggest them for code that runs on 8.0 or 5.7, and flag them as bugs when they appear:

| Feature | Introduced | On 8.0.46 |
|---|---|---|
| `SHOW BINARY LOG STATUS`, `RESET BINARY LOGS AND GTIDS` | 8.2.0 | `ERROR 1064` (use `SHOW MASTER STATUS`, `RESET MASTER`) |
| `CREATE/ALTER EVENT ... DISABLE ON REPLICA` | 8.2.0 | `ERROR 1064` (use `DISABLE ON SLAVE`, which is not deprecated on 8.0) |
| `EXPLAIN FORMAT=JSON INTO @var`, `EXPLAIN ... FOR SCHEMA db`, `explain_json_format_version` | 8.1–8.4[^23] | `ERROR 1064` / `ERROR 1193` |
| `EXPLAIN ANALYZE FORMAT=JSON` | 8.1–8.4 (8.4.13 accepts it only with `explain_json_format_version=2`) | `ERROR 1235 ... doesn't yet support 'EXPLAIN ANALYZE with JSON format'` |
| `GROUP BY ROLLUP (a, b)` | 8.4 (documented in 8.4.3) | `ERROR 1064`; use `GROUP BY a, b WITH ROLLUP` |
| `ANALYZE TABLE ... UPDATE HISTOGRAM ... AUTO UPDATE` | 8.4.0[^23] | `ERROR 1064` |
| `restrict_fk_on_non_standard_key`, privileges `SET_ANY_DEFINER`, `ALLOW_NONEXISTENT_DEFINER`, `FLUSH_PRIVILEGES`, `OPTIMIZE_LOCAL_TABLE`, `TRANSACTION_GTID_TAG`; tagged GTIDs | 8.2.0–8.4.0[^23] | `ERROR 1193` for the variable; `GRANT SET_ANY_DEFINER ...` is `ERROR 1149` (syntax error: unknown privilege; 8.0 uses `SET_USER_ID`) |
| Hash-based `INTERSECT`/`EXCEPT` (`hash_set_operations`) | 8.1–8.4[^23] | not in `optimizer_switch` |
| `VERSION_WITH_UPDATE()`, `@@version_with_update` | 8.4.13 / 9.7.4 | `ERROR 1305` |
| `VECTOR(n)`, `STRING_TO_VECTOR()`, `VECTOR_DIM()` | 9.0.0[^24] | `ERROR 1064` / `ERROR 1305` |
| `CREATE VIEW IF NOT EXISTS` | 9.1.0 | `ERROR 1064`; use `CREATE OR REPLACE VIEW` |
| JavaScript stored programs, `CREATE LIBRARY`, JSON duality views | 9.0–9.7 | not available |
| `activate_mandatory_roles`, `GROUPING()` without `ROLLUP`, `enable_cascade_triggers`, hypergraph optimizer / `QUALIFY` in Community | 9.5–9.7[^24] | `ERROR 1193` / `ERROR 1111` / `ERROR 3999` / `ERROR 1064` |

Not available on 8.0.46 (nor on 8.4 or 9.7): `INSERT ... RETURNING`, `IS [NOT] DISTINCT FROM` (use `<=>`), `FULL OUTER JOIN`, `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, `DROP INDEX IF EXISTS`, `CREATE OR REPLACE TABLE`, `CREATE SEQUENCE`, `CREATE MATERIALIZED VIEW`, `WITH x AS MATERIALIZED (...)`, partial indexes, `INCLUDE` columns.

##### Deprecated in 8.0 (works, but flag it)

These print a warning on 8.0.46 (unless noted) and are removed or disabled in 8.4 or 9.x. Keep them out of new code so the 8.4 upgrade is mechanical:

- `VALUES(col)` in `ON DUPLICATE KEY UPDATE` (warning 1287, 8.0.20) → row alias `INSERT ... VALUES (...) AS new ON DUPLICATE KEY UPDATE c = new.c` (8.0.19+).
- `SQL_CALC_FOUND_ROWS` / `FOUND_ROWS()` (warning 1287) → separate `COUNT(*)`.
- `&&`, `||` as `OR`, `!` as `NOT` (warning 1287) → `AND`, `OR`, `NOT`.
- Assigning user variables inside expressions (`SELECT @y := 2`, warning 1287) → `SET` or `SELECT ... INTO`, window functions for running counters.
- Integer display width (`INT(11)`), `ZEROFILL`, `FLOAT(M,D)`/`DOUBLE(M,D)` (warning 1681); `AUTO_INCREMENT` on `FLOAT`/`DOUBLE` (warning 3856, an error on 8.4).
- `utf8` (warning 3719) / `utf8mb3` (warning 1287), `ucs2` and the `ASCII`/`UNICODE` column-charset shorthands (warning 1287, 8.0.28) → `utf8mb4`.
- `$` as the first character of an unquoted identifier (warning 1681, 8.0.32; `$a$b` is a syntax error on 8.4); unquoted `FULL` as an identifier (warning 4119).
- `MASTER`/`SLAVE` replication statements and options (warning 1287) → `REPLICA`/`SOURCE` forms (8.0.22/8.0.23). `SHOW MASTER STATUS`, `SHOW MASTER LOGS`, `RESET MASTER`, `PURGE MASTER LOGS` and `DISABLE ON SLAVE` print **no** warning on 8.0 but are syntax errors on 8.4.
- `FLUSH HOSTS` (warning 1287) → `TRUNCATE TABLE performance_schema.host_cache`; `LOCK TABLES ... LOW_PRIORITY WRITE` (warning 1287).
- `INFORMATION_SCHEMA.PROCESSLIST` (warning 1287, 8.0.35) → `performance_schema.processlist`. With the default `performance_schema_show_processlist=OFF`, a plain `SHOW PROCESSLIST` uses that implementation and prints the same warning 1287 on 8.0.46. `INFORMATION_SCHEMA.TABLESPACES` (warning 1681).
- `binlog_format`, `log_bin_trust_function_creators`, `binlog_transaction_dependency_tracking`, `expire_logs_days` (warning 1287 on read or `SET`) → row-based logging only, `binlog_expire_logs_seconds`.
- `JSON_MERGE()` (warning 1287) → `JSON_MERGE_PRESERVE()` or `JSON_MERGE_PATCH()`.
- `mysql_native_password` (8.0.34; no SQL warning, only an error-log entry, written once since 8.0.38), `default_authentication_plugin` (8.0.27; warning 1287 on read), `sha256_password` (8.0.16; `CREATE USER ... IDENTIFIED WITH sha256_password` prints no warning).
- `innodb_log_file_size`, `innodb_log_files_in_group` (8.0.30; reading them prints no warning) → `innodb_redo_log_capacity`.
- Wildcards `%`/`_` in database-level `GRANT` and `%` matching `localhost` (8.0.35; no warning).
- `mysql_upgrade`, `mysqlpump`, `mysql_ssl_rsa_setup` (all removed in 8.4).

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
- **GOOD**: if UUIDs are required, store them as `BINARY(16)`. For the version-1 UUIDs that `UUID()` returns, `UUID_TO_BIN(u, 1)` (8.0+) swaps the time fields so values become roughly monotonic. MySQL has no built-in UUIDv7 generator; generate v7 in the application if you want time-ordered random IDs. On 5.7, convert with `UNHEX(REPLACE(u, '-', ''))`.

```sql
SELECT HEX(UUID_TO_BIN('6ccd780c-baba-1026-9564-5b8c656024db'))    AS plain,    -- 6CCD780CBABA102695645B8C656024DB
       HEX(UUID_TO_BIN('6ccd780c-baba-1026-9564-5b8c656024db', 1)) AS swapped;  -- 1026BABA6CCD780C95645B8C656024DB
SELECT SUBSTRING(UUID(), 15, 1) AS uuid_version;                              -- 1 (UUID() is version 1)

-- Expression defaults: 8.0.13+
CREATE TABLE exd (
  id   BINARY(16) PRIMARY KEY DEFAULT (UUID_TO_BIN(UUID(), 1)),
  meta JSON DEFAULT (JSON_OBJECT()),
  body TEXT DEFAULT (''),
  created DATE DEFAULT (CURRENT_DATE)
);
INSERT INTO exd () VALUES ();
SELECT BIN_TO_UUID(id, 1), meta, body, created FROM exd;

CREATE TABLE lit (body TEXT DEFAULT 'x');
-- ERROR 1101 (42000): BLOB, TEXT, GEOMETRY or JSON column 'body' can't have a default value
```

Expression defaults (`DEFAULT (expr)`, with parentheses) work for `BLOB`/`TEXT`/`JSON` columns; a literal default on those types does not. On 5.7 neither works (`ERROR 1064` for the parenthesized form).

A table without a primary key gets a hidden 6-byte row ID, cannot be replicated efficiently with row-based replication and is rejected by many managed services (and by Group Replication). Two server settings help:

```sql
SET SESSION sql_require_primary_key = ON;            -- 8.0.13+
CREATE TABLE nopk2 (a INT);
-- ERROR 3750 (HY000): Unable to create or change a table without a primary key, when the system
--   variable 'sql_require_primary_key' is set. ...
CREATE TABLE uq_only (a INT NOT NULL, UNIQUE KEY (a));   -- ERROR 3750 as well: a UNIQUE NOT NULL key does not count
SET SESSION sql_require_primary_key = OFF;

SET SESSION sql_generate_invisible_primary_key = ON;  -- 8.0.30+
CREATE TABLE nopk (a INT);
SHOW CREATE TABLE nopk;
-- `my_row_id` bigint unsigned NOT NULL AUTO_INCREMENT /*!80023 INVISIBLE */, ... PRIMARY KEY (`my_row_id`)
SELECT * FROM nopk;         -- my_row_id is not returned by SELECT *
SET SESSION sql_generate_invisible_primary_key = OFF;
```

Flag migrations that create tables without a primary key.

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

Deprecated attributes still accepted with warning 1681 on 8.0.17+ (5.7 accepts them silently) — remove them in new DDL:

```sql
CREATE TABLE w (id INT(11), b TINYINT(1), z INT ZEROFILL, f FLOAT(7,2));
-- Warning 1681: Integer display width is deprecated and will be removed in a future release. (twice)
-- Warning 1681: The ZEROFILL attribute is deprecated ...
-- Warning 1681: Specifying number of digits for floating point data types is deprecated ...
```

`TINYINT(1)` is the one display width that still has meaning for connectors (it is how `BOOLEAN`/`BOOL` is stored and how drivers detect booleans); MySQL has no real boolean type. Schema-diff tools comparing 5.7 and 8.0 output see `int(11)` vs `int` because 8.0.19+ no longer prints display widths in `SHOW CREATE TABLE`; that is cosmetic, but schema-parsing code that expects `int(11)` breaks.

**Auto-increment persistence (8.0+).** On 8.0 the counter survives a restart. On 5.7 it is recomputed as `MAX(id) + 1` at startup, so ids of deleted rows are **reused** after a restart — dangerous when ids are referenced from other systems or archives:

```sql
CREATE TABLE aip (id INT AUTO_INCREMENT PRIMARY KEY);
INSERT INTO aip VALUES (), (), ();
DELETE FROM aip WHERE id = 3;
-- restart the server, then:
INSERT INTO aip VALUES ();
SELECT id FROM aip;          -- 8.0.46: 1, 2, 4      5.7.44: 1, 2, 3 (id 3 reused)
```

##### Strings, ENUM, JSON

- `VARCHAR(n)` counts characters; the byte limit matters for indexes (see Indexing).
- `ENUM` sorts by declaration index, not alphabetically, and compares to integers by index. Adding a value at the end is metadata-only; reordering or inserting in the middle rebuilds the table. A lookup table or a `VARCHAR` with a `CHECK` constraint (8.0.16+) is often easier to evolve.

```sql
CREATE TABLE en (s ENUM('small','large','medium'));
INSERT INTO en VALUES ('small'), ('large'), ('medium');
SELECT s FROM en ORDER BY s;        -- small, large, medium  (declaration order)
SELECT s FROM en WHERE s = 2;       -- large                 (index 2)
INSERT INTO en VALUES ('huge');     -- ERROR 1265 (01000): Data truncated for column 's' at row 1
```

- `JSON` (5.7.8+) is stored in a binary format, validated on insert and partially updatable in place with `JSON_SET`/`JSON_REPLACE`/`JSON_REMOVE` (8.0+). Do not use `JSON` to avoid modelling columns that are filtered, joined or constrained; index JSON paths with generated columns, functional indexes or multi-valued indexes when you must (see Indexing).

##### Constraints: NOT NULL, CHECK, defaults

`CHECK` constraints are enforced since **8.0.16**; on 5.7 and 8.0.0–8.0.15 they are parsed and silently ignored, so a 5.7 schema can contain `CHECK` clauses that never held. A `CHECK` that evaluates to `NULL` passes (add `NOT NULL` separately), `INSERT IGNORE` turns a violation into a warning and silently skips the row, and nondeterministic functions are rejected:

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

`CHECK (JSON_SCHEMA_VALID(...))` (8.0.17+) validates JSON documents on write:

```sql
CREATE TABLE cj (id INT PRIMARY KEY, doc JSON,
  CHECK (JSON_SCHEMA_VALID('{"type":"object","required":["name"]}', doc)));
INSERT INTO cj VALUES (1, '{"x":1}');      -- ERROR 3819 (HY000): Check constraint 'cj_chk_1' is violated.
INSERT INTO cj VALUES (2, '{"name":"a"}'); -- OK
```

On 5.7 enforce such invariants with a `BEFORE INSERT`/`BEFORE UPDATE` trigger that `SIGNAL`s, or in the application. When a 5.7 schema moves to 8.0, check existing data against every `CHECK` clause first: adding or rebuilding with an enforced constraint fails on violating rows.

##### Foreign keys

InnoDB enforces foreign keys, with these traps on 8.0 and 5.7:

1. **Inline `REFERENCES` on a column definition is silently ignored.** Only a table-level `FOREIGN KEY (...) REFERENCES ...` clause (or `CONSTRAINT name FOREIGN KEY ...`) creates a constraint. MySQL 9.0 started enforcing the inline form,[^24] so the same DDL behaves differently after an eventual upgrade to 9.7.

```sql
-- BAD on 8.0 and 5.7: no constraint is created, no warning is raised
CREATE TABLE person (id INT PRIMARY KEY);
CREATE TABLE pet (id INT PRIMARY KEY, owner INT REFERENCES person (id));
INSERT INTO pet VALUES (1, 999);         -- succeeds on 8.0.46 and 5.7.44
SELECT COUNT(*) FROM information_schema.referential_constraints
WHERE constraint_schema = DATABASE() AND table_name = 'pet';    -- 0

-- GOOD: table-level constraint, enforced on every version
CREATE TABLE pet2 (
  id INT PRIMARY KEY,
  owner INT NOT NULL,
  CONSTRAINT fk_pet2_owner FOREIGN KEY (owner) REFERENCES person (id)
);
```

2. **8.0 accepts foreign keys that reference a non-unique index**, which the SQL standard forbids and 8.4 rejects by default (`ERROR 6125`, `restrict_fk_on_non_standard_key=ON`). Semantics are surprising: a parent row cannot be deleted while any child references its value, **even if another parent row has the same value**. Reference a `PRIMARY KEY` or `UNIQUE` key; flag new FKs to non-unique columns because they block the 8.4 upgrade path (dumps fail to reload there).

```sql
CREATE TABLE pn (id INT PRIMARY KEY, code INT, KEY (code));
CREATE TABLE cn (id INT PRIMARY KEY, code INT, FOREIGN KEY (code) REFERENCES pn (code));  -- accepted on 8.0.46
INSERT INTO pn VALUES (1, 7), (2, 7);
INSERT INTO cn VALUES (1, 7);
DELETE FROM pn WHERE id = 1;
-- ERROR 1451 (23000): Cannot delete or update a parent row: a foreign key constraint fails ...
--   (although parent id 2 still provides code 7)
```

3. **Cascaded changes do not fire triggers.** `ON DELETE CASCADE` / `ON UPDATE CASCADE` / `SET NULL` change child rows without running the child table's triggers; audit triggers on child tables miss them.[^25]

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
7. Column types must be compatible: integer types need the same size and signedness, string columns the same character set and collation (lengths may differ). Otherwise: `ERROR 3780 Referencing column 'p' and referenced column 'id' in foreign key constraint 'cb2_ibfk_1' are incompatible.` (parent `INT`, child `BIGINT`).

##### Generated and invisible columns

```sql
CREATE TABLE ic (id INT PRIMARY KEY, secret VARCHAR(10) INVISIBLE);   -- 8.0.23+
INSERT INTO ic VALUES (1);                       -- invisible columns are skipped by positional INSERT
INSERT INTO ic (id, secret) VALUES (2, 's');
SELECT * FROM ic;                                -- returns only id
SELECT id, secret FROM ic;                       -- explicit reference works
```

Invisible columns (8.0.23+) let you add a column without breaking `SELECT *` or `INSERT ... VALUES` without a column list in old code. Generated columns (5.7.6+) are `VIRTUAL` (computed on read, indexable since 5.7.8, added `INSTANT`ly on 8.0) or `STORED` (materialized; adding one rebuilds the table with `ALGORITHM=COPY`).

***

#### Character Sets and Collations

##### Defaults and the `utf8` trap

```sql
SELECT @@character_set_server, @@collation_server;     -- 8.0: utf8mb4 | utf8mb4_0900_ai_ci
                                                       -- 5.7: latin1  | latin1_swedish_ci
CREATE TABLE m3 (s VARCHAR(10) CHARACTER SET utf8);
-- 8.0: Warning 3719: 'utf8' is currently an alias for the character set UTF8MB3, but will be an alias
--   for UTF8MB4 in a future release. Please consider using UTF8MB4 in order to be unambiguous.
-- 5.7: no warning
SHOW CREATE TABLE m3;       -- `s` varchar(10) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci

INSERT INTO m3 VALUES ('😀');
-- ERROR 1366 (HY000): Incorrect string value: '\xF0\x9F\x98\x80' for column 's' at row 1
SET SESSION sql_mode = '';
INSERT INTO m3 VALUES ('a😀b');      -- Warning 1366; stored as 'a?b'  (silent data loss)
SET SESSION sql_mode = DEFAULT;
```

- Flag `utf8`, `utf8mb3`, `utf8_general_ci`, `utf8_unicode_ci` in new DDL; use `utf8mb4`.
- **5.7 projects**: the server default is `latin1`. A `CREATE DATABASE`/`CREATE TABLE` without an explicit `CHARACTER SET utf8mb4` creates `latin1` tables that cannot store most non-Western text. Require explicit `DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci` (or `_general_ci`) in every 5.7 migration; note that `utf8mb4_0900_*` collations do not exist on 5.7 (`ERROR 1273 Unknown collation`), so a dump from 8.0 does not load into 5.7 without rewriting collations.
- Flag connections that do not set the character set. The `mysql` command-line client picks its character set from the OS locale; in a container without a locale it connects as `latin1` (observed: `@@character_set_connection = latin1` in the official image), and non-ASCII literals in scripts are then misinterpreted. Use `--default-character-set=utf8mb4` or `SET NAMES utf8mb4` in migration runners; drivers should set `charset=utf8mb4` and an explicit collation.

##### `utf8mb4_0900_ai_ci` semantics [8.0+]

The 8.0 default collation is case-insensitive, accent-insensitive and **NO PAD** (trailing spaces are significant). Older collations (`utf8mb4_general_ci`, `utf8mb4_unicode_ci`, `utf8mb4_bin`, and every 5.7 collation) are `PAD SPACE`.[^26]

```sql
SET NAMES utf8mb4;
SELECT 'a' = 'A', 'e' = 'é', 'a' = 'a ', 'ß' = 'ss';      -- 8.0: 1 | 1 | 0 | 1
                                                          -- 5.7 (utf8mb4_general_ci): 1 | 1 | 1 | 0
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

- **Unique keys are collation-aware.** With the default collation, `'Muller'`, `'müller'` and `'MÜLLER'` collide; `'Muller'` and `'Muller '` do not (they do collide under the `PAD SPACE` collations).

```sql
CREATE TABLE u (name VARCHAR(50) PRIMARY KEY);
INSERT INTO u VALUES ('Muller');
INSERT INTO u VALUES ('müller');    -- ERROR 1062 (23000): Duplicate entry 'müller' for key 'u.PRIMARY'
INSERT INTO u VALUES ('Muller ');   -- OK: NO PAD, trailing space is significant
```

  Usernames, slugs, tokens, hashes and other identifiers that must be distinct byte-for-byte need `COLLATE utf8mb4_bin` (or `utf8mb4_0900_bin`) or a `VARBINARY` column. Email addresses are usually fine case-insensitive, but accent-insensitivity may still surprise.
- **Upgrading 5.7 → 8.0 changes comparison results** for new columns: a table created on 8.0 without an explicit collation is `NO PAD` and treats `ß = ss`, while 5.7-era tables keep their old collation. The same query can return different rows depending on which table it hits.
- **`LOWER(col) = ...` is redundant** with a `_ci` collation and prevents index use (see Indexing).
- **Mixing collations** in a comparison or join raises an error, and a join between columns with different collations cannot use the index on the converted side:

```sql
CREATE TABLE ca (s VARCHAR(10) COLLATE utf8mb4_unicode_ci);
CREATE TABLE cb (s VARCHAR(10) COLLATE utf8mb4_0900_ai_ci);
SELECT * FROM ca JOIN cb ON ca.s = cb.s;
-- ERROR 1267 (HY000): Illegal mix of collations (utf8mb4_unicode_ci,IMPLICIT) and
--   (utf8mb4_0900_ai_ci,IMPLICIT) for operation '='
```

  Schemas migrated from 5.7 typically contain `utf8mb4_general_ci` or `utf8mb4_unicode_ci` columns next to new `utf8mb4_0900_ai_ci` columns. Specify collations explicitly at database or table level in migrations so new tables match old ones (or set `collation_server`/`default_collation_for_utf8mb4` deliberately).
- `ALTER TABLE ... CONVERT TO CHARACTER SET` rewrites the whole table (`ALGORITHM=COPY`) and may change index lengths and uniqueness; review it as a heavy migration.

***

#### SQL Mode and Strictness

The server's default `sql_mode` is strict on 8.0 and on 5.7.5+:[^27]

```sql
SELECT @@sql_mode;
-- 8.0: ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION
-- 5.7: ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION
```

`NO_AUTO_CREATE_USER` was removed in 8.0; scripts, dumps (`/*!50003 SET sql_mode=... */` in routines) and ORM connection settings that still name it fail with `ERROR 1231`. Remove it.

With the default mode, bad data is rejected:

```sql
CREATE TABLE s1 (id INT PRIMARY KEY, code VARCHAR(5) NOT NULL, qty TINYINT UNSIGNED, d DATE);
INSERT INTO s1 VALUES (1, 'toolong', 1, '2024-01-01'); -- ERROR 1406 (22001): Data too long for column 'code' at row 1
INSERT INTO s1 VALUES (2, 'ok', 300, '2024-01-01');    -- ERROR 1264 (22003): Out of range value for column 'qty' at row 1
INSERT INTO s1 VALUES (3, 'ok', 1, '2024-02-30');      -- ERROR 1292 (22007): Incorrect date value: '2024-02-30' ...
INSERT INTO s1 VALUES (4, 'ok', 1, '0000-00-00');      -- ERROR 1292 (22007): Incorrect date value: '0000-00-00' ...
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

- **BAD**: application code or ORMs that set `sql_mode=''` or drop `STRICT_TRANS_TABLES`/`ONLY_FULL_GROUP_BY` to make legacy queries pass. This is common in projects upgraded from 5.6. Fix the query.
- **BAD**: `INSERT IGNORE` as an "upsert" or "skip duplicates" shortcut. It also hides truncation, out-of-range values, invalid dates, `CHECK` violations and **foreign-key violations** (see Writes). Use `INSERT ... ON DUPLICATE KEY UPDATE id = id` or catch error 1062 when only duplicates should be skipped.
- Strict mode applies to transactional tables; it does not validate values that never reach a column (e.g. `SELECT` expressions):

```sql
SELECT '1abc' + 0, '1abc' = 1, 'abc' = 0;      -- 1 | 1 | 1   (warnings 1292 only)
SELECT STR_TO_DATE('31/02/2024', '%d/%m/%Y');  -- NULL, Warning 1411
```

##### ONLY_FULL_GROUP_BY and GROUP BY ordering

```sql
CREATE TABLE o (id INT PRIMARY KEY, cust INT, total DECIMAL(10,2), status VARCHAR(10));
INSERT INTO o VALUES (1, 1, 10, 'new'), (2, 1, 20, 'paid'), (3, 2, 5, 'new');

SELECT cust, status, SUM(total) FROM o GROUP BY cust;
-- ERROR 1055 (42000): Expression #2 of SELECT list is not in GROUP BY clause and contains
--   nonaggregated column 't.o.status' which is not functionally dependent on columns in GROUP BY clause; ...

SELECT cust, ANY_VALUE(status), SUM(total) FROM o GROUP BY cust;   -- explicit "any row" (nondeterministic)
SELECT o.id, o.cust, o.status FROM o GROUP BY o.id;                -- OK: functionally dependent on the PK
```

`ANY_VALUE()` (5.7.5+) is acceptable only when any value is correct. To get the row with the max/min per group, use a window function (8.0+) or a join (see Query patterns).

**`GROUP BY` does not sort on 8.0.** 5.7 sorted the result of `GROUP BY` implicitly and allowed `GROUP BY col DESC`; 8.0 removed both (the `ASC`/`DESC` qualifiers are `ERROR 1064` since 8.0.13). Code ported from 5.7 that paginates or displays grouped results without `ORDER BY` may return rows in a different order after the upgrade. Always add `ORDER BY` when order matters.

##### Operators that differ from standard SQL

```sql
SELECT 'a' || 'b';                     -- 0: || is logical OR (deprecated on 8.0, warning 1287), not concatenation
SELECT CONCAT('a', 'b', NULL);         -- NULL: any NULL argument makes CONCAT NULL
SELECT CONCAT_WS('-', 'a', NULL, 'b'); -- a-b: CONCAT_WS skips NULLs
SELECT NULL <=> NULL;                  -- 1: NULL-safe equality (MySQL has no IS NOT DISTINCT FROM)
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
- With `explicit_defaults_for_timestamp=ON` (8.0 default), a `TIMESTAMP` column has no implicit `DEFAULT CURRENT_TIMESTAMP` or `ON UPDATE` and is nullable; write the clauses explicitly:

```sql
CREATE TABLE ts2 (
  id INT PRIMARY KEY,
  v INT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  plain      TIMESTAMP
);
INSERT INTO ts2 (id, v) VALUES (1, 1);
SELECT plain FROM ts2;           -- NULL  (no automatic initialization on 8.0)
```

  **5.7 differs** (`explicit_defaults_for_timestamp=OFF`): the first `TIMESTAMP` column of a table silently becomes `NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` — so it changes on every update — and later ones get a zero-date default, which strict mode rejects:

```sql
CREATE TABLE ts57 (id INT PRIMARY KEY, a TIMESTAMP, b TIMESTAMP);
-- 5.7.44: ERROR 1067 (42000): Invalid default value for 'b'
-- 8.0.46: OK, both columns `timestamp NULL DEFAULT NULL`
```

  The same DDL therefore produces different tables on 5.7 and 8.0. Require explicit `NULL`/`NOT NULL` and `DEFAULT` clauses on every `TIMESTAMP` column.
- `DATETIME` columns accept the same `DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` clauses; the precision must match the column, otherwise the DDL fails:

```sql
CREATE TABLE ts3 (id INT PRIMARY KEY, d3 DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3));  -- OK
CREATE TABLE ts4 (id INT PRIMARY KEY, d3 DATETIME(3) DEFAULT CURRENT_TIMESTAMP);
-- ERROR 1067 (42000): Invalid default value for 'd3'
```

##### Fractional seconds round by default

```sql
INSERT INTO tt VALUES (6, NULL, '2024-06-01 12:00:00.999', '2024-06-01 12:00:00.1234567');
SELECT dt, ts6 FROM tt WHERE id = 6;   -- 2024-06-01 12:00:01 | 2024-06-01 12:00:00.123457
INSERT INTO tt VALUES (8, NULL, '2024-12-31 23:59:59.5', NULL);
SELECT dt FROM tt WHERE id = 8;        -- 2025-01-01 00:00:00  (next day, next year)

SET sql_mode = CONCAT(@@sql_mode, ',TIME_TRUNCATE_FRACTIONAL');   -- 8.0+
INSERT INTO tt VALUES (7, NULL, '2024-06-01 12:00:00.999', NULL);
SELECT dt FROM tt WHERE id = 7;        -- 2024-06-01 12:00:00
SET sql_mode = DEFAULT;
```

Writing a millisecond value into a `DATETIME` (precision 0) column can move it into the next second — or the next day/year. Declare the precision you need (`DATETIME(3)`) and make application and column precision match.

##### Literals with offsets, `AT TIME ZONE`, named zones

```sql
SET time_zone = '+00:00';
INSERT INTO tt VALUES (5, NULL, '2024-06-01 12:00:00+05:00', NULL);   -- 8.0.19+: offset literal
SELECT dt FROM tt WHERE id = 5;       -- 2024-06-01 07:00:00 (converted to the session time zone)
-- 5.7.44: ERROR 1292 (22007): Incorrect datetime value: '2024-06-01 12:00:00+05:00'

SET time_zone = '+02:00';
SELECT CAST(ts AT TIME ZONE '+00:00' AS DATETIME) AS utc, ts FROM tt WHERE id = 1;    -- 8.0.22+
-- 2024-06-01 12:00:00 | 2024-06-01 14:00:00
SELECT CAST(ts AT TIME ZONE 'Europe/Berlin' AS DATETIME) FROM tt WHERE id = 1;
-- ERROR 1298 (HY000): Unknown or incorrect time zone: 'Europe/Berlin'
SELECT CAST(dt AT TIME ZONE '+00:00' AS DATETIME) FROM tt WHERE id = 1;
-- ERROR 3998 (HY000): Cannot cast value to TIMESTAMP WITH TIME ZONE.
```

`CAST(... AT TIME ZONE ...)` only accepts a `TIMESTAMP` value and the zone `'+00:00'`/`'UTC'`. Named zones (`'Europe/Berlin'`) in `CONVERT_TZ()` and `SET time_zone` need the time-zone tables to be loaded[^28] (`mysql_tzinfo_to_sql`); otherwise `CONVERT_TZ()` returns `NULL` **without a warning**, and `SET time_zone = 'Europe/Berlin'` fails with `ERROR 1298 Unknown or incorrect time zone`. The official Docker image ships them loaded; many installations do not. Prefer fixed offsets or conversion in the application, and check `SELECT COUNT(*) FROM mysql.time_zone_name` when code relies on named zones.

```sql
SELECT CONVERT_TZ('2024-03-31 01:30:00', 'UTC', 'Europe/Berlin');   -- 2024-03-31 03:30:00 (tables loaded)
SELECT CONVERT_TZ('2024-03-31 01:30:00', 'UTC', 'Nowhere/Zone');    -- NULL, no warning
SELECT '2024-01-31' + INTERVAL 1 MONTH;                              -- 2024-02-29 (clamped to month end)
SET time_zone = '+00:00';
SELECT UNIX_TIMESTAMP('2038-01-19 03:14:08'), FROM_UNIXTIME(2147483648);
-- 2147483648 | 2038-01-19 03:14:08   (64-bit range since 8.0.28; earlier releases and 5.7 return 0 / NULL)
```

- `NOW()` is constant for the statement; `SYSDATE()` is evaluated per call and is unsafe for statement-based replication. Use `NOW()`.
- Do not filter with `DATE(col) = ?` — use a half-open range (see Indexing).

***

#### Indexing

##### What InnoDB indexes can and cannot do

MySQL 8.0 InnoDB has B-tree indexes (`PRIMARY`, `UNIQUE`, secondary), `FULLTEXT` and `SPATIAL` (R-tree) indexes, plus functional (8.0.13+) and multi-valued (8.0.17+) B-tree indexes.[^29] It has **no** partial (`WHERE`) indexes, no `INCLUDE` columns, no GIN/GiST/BRIN, no hash indexes on InnoDB (`USING HASH` is accepted with Note 3502 and a B-tree is built instead), no `CREATE INDEX CONCURRENTLY` and no `CREATE INDEX IF NOT EXISTS`. Advice written for PostgreSQL does not translate; the MySQL equivalents are:

| PostgreSQL idea | MySQL 8.0 equivalent | MySQL 5.7 |
|---|---|---|
| Partial index `WHERE status = 'active'` | Composite index with the filter column first, or a generated column that is `NULL` outside the subset | same |
| `INCLUDE (a, b)` covering index | Append the columns to the index key; every secondary index already contains the primary key | same |
| `CREATE INDEX CONCURRENTLY` | `ALTER TABLE ... ADD INDEX ..., ALGORITHM=INPLACE, LOCK=NONE` | same (5.6+) |
| Expression index `ON (lower(email))` | Usually unnecessary (`_ci` collation); otherwise functional index `((expr))` | indexed `VIRTUAL` generated column |
| GIN on a JSONB array | Multi-valued index `((CAST(doc->'$.tags' AS CHAR(n) ARRAY)))` (8.0.17+) | side table with one row per element |
| `pg_trgm` for `LIKE '%x%'` | `FULLTEXT` index with `MATCH ... AGAINST` (word-based, not substring), or an external search engine | same |

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
-- -> Covering index skip scan on orders using ix_orders_status_created over '2024-03-01 ...' <= created_at < ...
```

"Covering index ..." in the tree output (or `Using index` in traditional output) means no clustered-index lookup was needed. On 8.0, a full scan of a secondary index is printed as `Index scan on ... using ...` even when it is covering (8.4 prints `Covering index scan`). A skip scan is only efficient when the leading column has few distinct values; do not rely on it instead of a proper index. `EXPLAIN FORMAT=TREE` needs 8.0.16+; on 5.7 read the traditional `EXPLAIN` columns `type`, `key`, `rows`, `Extra`.

##### Things that prevent index use

**Implicit type conversion.** Comparing a string column with a number converts every row's value to a number: full index scan, and wrong matches (`'1abc' = 1` is true). Comparing an integer column with a string literal is harmless.

```sql
-- BAD: phone is VARCHAR, literal is a number
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE phone = 4915100042;
-- -> Filter: (users.phone = 4915100042)  (rows=9973)
--     -> Index scan on users using ix_users_phone  (rows=99733)
-- Warning 1739: Cannot use ref access on index 'ix_users_phone' due to type or collation conversion on field 'phone'
-- Warning 1739: Cannot use range access on index 'ix_users_phone' due to type or collation conversion on field 'phone'

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
--     -> Index scan on users using ix_users_created  (rows=99733)

-- GOOD: half-open range
EXPLAIN FORMAT=TREE SELECT id FROM users
WHERE created_at >= '2024-01-02' AND created_at < '2024-01-03';
-- -> Covering index range scan on users using ix_users_created
--      over ('2024-01-02 00:00:00' <= created_at < '2024-01-03 00:00:00')  (rows=1440)

-- BAD: LOWER() is redundant with a _ci collation and disables the index
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE LOWER(email) = 'user42@example.com';
-- -> Filter: (lower(users.email) = 'user42@example.com')
--     -> Index scan on users using uq_users_email  (rows=99733)

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
--     -> Index scan on users using uq_users_email  (rows=99733)
```

**`OR` across different columns** may use `index_merge` (union) or fall back to a scan; check the plan, and consider `UNION ALL` of two indexed queries.

**Collation mismatch** between join columns (see Character Sets) also defeats indexes.

##### Index size limits

With `ROW_FORMAT=DYNAMIC` (the default since 5.7.9) an index key part may be at most 3072 bytes. In `utf8mb4` that is `VARCHAR(768)`:

```sql
CREATE TABLE ix1 (s VARCHAR(768), KEY (s));    -- OK
CREATE TABLE ix2 (s VARCHAR(769), KEY (s));
-- ERROR 1071 (42000): Specified key was too long; max key length is 3072 bytes
```

`COMPACT`/`REDUNDANT` tables (common in schemas created on 5.6) are limited to 767 bytes per key part, i.e. `VARCHAR(191)` in `utf8mb4`; 8.0.42+ rejects `ALTER TABLE ... ALGORITHM=INPLACE` that would exceed it.[^22] Use a prefix index (`KEY (s(191))`) only for non-unique lookups: a prefix `UNIQUE` key enforces uniqueness of the prefix, not the value, and a prefix index can never be covering. For uniqueness of long values, index a hash in a generated column (`sha BINARY(32) AS (UNHEX(SHA2(url, 256))) STORED UNIQUE`; works on 5.7 too).

##### Functional indexes and JSON [8.0.13+ / 8.0.17+]

A functional index is used only when the query contains the **same expression with the same type and collation**. JSON extraction is the classic trap: `->>` returns `utf8mb4_bin`, while `CAST(... AS CHAR)` produces the connection collation, so an index on the plain cast is not used by `doc->>'$.name' = ?`.[^29]

```sql
CREATE TABLE docs (id INT AUTO_INCREMENT PRIMARY KEY, doc JSON NOT NULL);
-- 5,000 rows like {"name": "n42", "age": 42, "tags": ["t2", "t1"]}

-- BAD: the index expression's collation differs from doc->>'$.name'
ALTER TABLE docs ADD INDEX ix_name ((CAST(doc->>'$.name' AS CHAR(32))));
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE doc->>'$.name' = 'n42';
-- -> Filter: (json_unquote(json_extract(docs.doc,'$.name')) = 'n42')
--     -> Table scan on docs  (rows=5009)

-- GOOD: match the collation of ->> (utf8mb4_bin)
ALTER TABLE docs DROP INDEX ix_name;
ALTER TABLE docs ADD INDEX ix_name ((CAST(doc->>'$.name' AS CHAR(32)) COLLATE utf8mb4_bin));
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE doc->>'$.name' = 'n42';
-- -> Index lookup on docs using ix_name ((cast(...) collate utf8mb4_bin)='n42')  (rows=1)
```

A virtual generated column with an ordinary index is often clearer, works on **5.7** as well, and the 8.0 optimizer also matches the underlying expression:

```sql
ALTER TABLE docs ADD COLUMN age INT AS (doc->>'$.age') VIRTUAL, ADD INDEX ix_age (age);
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE doc->>'$.age' = 42;   -- Index lookup on docs using ix_age (age=42)
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE age = 42;             -- Covering index lookup on docs using ix_age
```

Multi-valued indexes (8.0.17+) serve `MEMBER OF`, `JSON_CONTAINS` and `JSON_OVERLAPS` — not `JSON_SEARCH` or `LIKE`:

```sql
ALTER TABLE docs ADD INDEX ix_tags ((CAST(doc->'$.tags' AS CHAR(8) ARRAY)));
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE 't3' MEMBER OF (doc->'$.tags');
-- -> Index lookup on docs using ix_tags (cast(json_extract(doc,_utf8mb4'$.tags') as char(8) array)=json'"t3"')
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE JSON_CONTAINS(doc->'$.tags', '"t3"');
-- -> Index range scan on docs using ix_tags over ('t3' MEMBER OF (...))
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE JSON_SEARCH(doc->'$.tags', 'one', 't3') IS NOT NULL;
-- -> Table scan on docs
INSERT INTO docs (doc) VALUES ('{"name":"x","tags":["waytoolongtag"]}');
-- ERROR 3907 (22001): Data too long for functional index 'ix_tags'.
```

The optimizer chooses multi-valued indexes by cost: with 20% of rows matching, `JSON_OVERLAPS(doc->'$.tags', '["t3","t4"]')` used a table scan on 8.0.46 and an index range scan only with `/*+ INDEX(docs ix_tags) */`. Values longer than the `CHAR(n)` in the `ARRAY` cast make inserts fail (above); size it for the data.

##### Descending, invisible and redundant indexes [8.0+]

```sql
CREATE INDEX ix_orders_created_desc ON orders (created_at DESC);    -- 5.7: DESC is parsed and ignored
EXPLAIN FORMAT=TREE SELECT id, created_at FROM orders ORDER BY created_at DESC LIMIT 5;
-- -> Limit: 5 row(s)
--     -> Index scan on orders using ix_orders_created_desc

-- Test the effect of dropping an index without dropping it:
ALTER TABLE orders ALTER INDEX ix_orders_created_desc INVISIBLE;
EXPLAIN FORMAT=TREE SELECT id, created_at FROM orders ORDER BY created_at DESC LIMIT 5;
-- -> Limit: 5 row(s)
--     -> Sort: orders.created_at DESC, limit input to 5 row(s) per chunk  (rows=199510)
--         -> Index scan on orders using ix_orders_user_created
SET SESSION optimizer_switch = 'use_invisible_indexes=on';   -- per-session opt-in for testing
SET SESSION optimizer_switch = 'use_invisible_indexes=off';
ALTER TABLE orders ALTER INDEX ix_orders_created_desc VISIBLE;

ALTER TABLE users ALTER INDEX `PRIMARY` INVISIBLE;
-- ERROR 3522 (HY000): A primary key index cannot be invisible
```

- Descending indexes matter for mixed-direction sorts (`ORDER BY a ASC, b DESC`); for a single-direction sort a backward scan of an ascending index works too. A 5.7 schema with `DESC` in index definitions has ascending indexes; after the upgrade, a dump/reload creates real descending indexes, which can change plans.
- Making an index invisible is a metadata change (`ALGORITHM=INPLACE`, not `INSTANT`); an invisible index is still maintained on writes and still enforces `UNIQUE`.
- Flag redundant indexes: `KEY (a)` next to `KEY (a, b)`, or `KEY (user_id)` next to the FK index. `sys.schema_redundant_indexes` lists them.
- Flag over-indexing on write-heavy tables: every secondary index costs a B-tree update per write.

##### Histograms [8.0+]

For non-indexed columns used in filters and joins, a histogram gives the optimizer selectivity estimates without index maintenance cost. On 8.0 histograms are **never refreshed automatically**; schedule `ANALYZE TABLE ... UPDATE HISTOGRAM` after significant data changes (automatic updates arrived in 8.4):[^30]

```sql
ANALYZE TABLE orders UPDATE HISTOGRAM ON total WITH 32 BUCKETS;
SELECT column_name, JSON_EXTRACT(histogram, '$."histogram-type"') AS type
FROM information_schema.column_statistics WHERE table_name = 'orders';        -- total | "equi-height"
ANALYZE TABLE orders UPDATE HISTOGRAM ON total WITH 32 BUCKETS AUTO UPDATE;   -- ERROR 1064 on 8.0 (8.4+)
ANALYZE TABLE orders DROP HISTOGRAM ON total;
```

***

#### Reading Plans: EXPLAIN and EXPLAIN ANALYZE

- `EXPLAIN` (traditional table; the default, and `explain_format` can change it since 8.0.32), `EXPLAIN FORMAT=TREE` (8.0.16+, iterator tree, easiest to read), `EXPLAIN FORMAT=JSON` (with costs).[^31]
- `EXPLAIN ANALYZE` (8.0.18+) **executes** the query and reports actual rows, loops and time per iterator. It supports `SELECT`, `TABLE` and multi-table `UPDATE`/`DELETE`; only `FORMAT=TREE` is accepted (`FORMAT=JSON` is `ERROR 1235` on 8.0). On 8.0.46, single-table `UPDATE`/`DELETE` and `INSERT` print `<not executable by iterator executor>`, and a multi-table `UPDATE` printed timings without changing rows in tests. Never run `EXPLAIN ANALYZE` of DML against production data on the assumption that it is harmless, and remember the `SELECT` really runs (an expensive query runs in full).
- Key signals: `Table scan`, `Index scan` over many rows, `Filter` above a scan with a large `rows` estimate, `Sort` (filesort) on big inputs, `Materialize`/temporary tables, `Nested loop` with a large outer side and no index on the inner side, `Hash join` (8.0.18+; since 8.0.20 used wherever 5.7 used `Using join buffer (Block Nested Loop)`).
- Compare estimated `rows` with actual `rows` in `EXPLAIN ANALYZE`; large gaps point to missing statistics (`ANALYZE TABLE`, histograms) or correlated predicates.
- `rows` estimates on InnoDB come from sampling; `information_schema.tables.table_rows` is an estimate as well.
- 5.7 has only traditional and JSON `EXPLAIN`; `EXPLAIN EXTENDED`/`PARTITIONS` exist there but are removed in 8.0.

```sql
EXPLAIN ANALYZE SELECT id, total FROM orders
WHERE status = 'paid' ORDER BY created_at, id LIMIT 20 OFFSET 40000;
-- -> Limit/Offset: 20/40000 row(s)  (actual time=191..191 rows=20 loops=1)
--     -> Index lookup on orders using ix_orders_status_created (status='paid')
--          (actual time=2.82..182 rows=40020 loops=1)
```

***

#### Query Patterns and Performance

##### Pagination: OFFSET vs keyset — and the row-constructor trap

`LIMIT n OFFSET m` reads and discards `m` rows (40,020 rows read above to return 20). Use keyset (seek) pagination with a deterministic order that ends in a unique column.

**MySQL does not use range access for row-constructor inequalities** like `(created_at, id) > (?, ?)`. The predicate becomes a `Filter` over an index scan that starts at the beginning of the index — just as slow as `OFFSET` for later pages. Write the expanded `OR` form, which the optimizer turns into a range scan:[^32]

```sql
-- BAD on MySQL: reads from the start of the index and filters (56,040 rows read)
EXPLAIN ANALYZE SELECT id, total FROM orders
WHERE status = 'paid' AND (created_at, id) > ('2024-08-21 10:00:00.000', 160000)
ORDER BY created_at, id LIMIT 20;
-- -> Limit: 20 row(s)  (actual time=301..301 rows=20 loops=1)
--     -> Filter: ((orders.created_at,orders.id) > ('2024-08-21 10:00:00.000',160000))
--         -> Index lookup on orders using ix_orders_status_created (status='paid') (actual ... rows=56040)

-- GOOD: expanded predicate → range scan, 20 rows read
EXPLAIN ANALYZE SELECT id, total FROM orders
WHERE status = 'paid'
  AND (created_at > '2024-08-21 10:00:00.000'
       OR (created_at = '2024-08-21 10:00:00.000' AND id > 160000))
ORDER BY created_at, id LIMIT 20;
-- -> Limit: 20 row(s)  (actual time=5.62..5.63 rows=20 loops=1)
--     -> Index range scan on orders using ix_orders_status_created over
--          (status = 'paid' AND created_at = '2024-08-21 10:00:00.000' AND 160000 < id)
--          OR (status = 'paid' AND '2024-08-21 10:00:00.000' < created_at) ...
```

The same limitation exists on 8.4 and 9.7. Row-constructor *equality* `(a, b) = (?, ?)` and `IN ((?, ?), ...)` are optimized normally.

##### COUNT(*)

InnoDB has no stored row count: `SELECT COUNT(*) FROM t` scans the smallest index (in parallel since 8.0.14, `innodb_parallel_read_threads`). For dashboards, use estimates (`information_schema.tables.table_rows`) or maintained counters. Filtered counts need a covering index on the filter columns. `SQL_CALC_FOUND_ROWS`/`FOUND_ROWS()` are deprecated and usually slower than a separate `COUNT(*)`:

```sql
SELECT SQL_CALC_FOUND_ROWS id FROM par LIMIT 1;
-- Warning 1287: SQL_CALC_FOUND_ROWS is deprecated and will be removed in a future release. Consider using two separate queries instead.
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

The nested derived table works because it is materialized before the outer statement modifies the table; it reads as a trick, so prefer an explicit multi-table `UPDATE ... JOIN` / `DELETE t FROM t JOIN ...` for clarity.

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

##### Top-N per group, window functions, LATERAL [8.0+]

```sql
-- Latest order per user: window function (8.0+)
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
- There is no `QUALIFY` (`ERROR 1064` on 8.0); filter on the window result in an outer query or CTE.
- **5.7 replacement** for "latest row per group" (no window functions, no `LATERAL`): a join against a grouped derived table, with a tie-breaker:

```sql
-- Works on 5.7 and 8.0
SELECT o.user_id, o.id, o.created_at
FROM orders o
JOIN (SELECT user_id, MAX(created_at) AS mx FROM orders WHERE user_id IN (1, 2, 3) GROUP BY user_id) m
  ON m.user_id = o.user_id AND m.mx = o.created_at;
```

  Flag the 5.7-era user-variable trick (`@rn := IF(@prev = user_id, @rn + 1, 1)`) in new code: its evaluation order is undefined, it is deprecated on 8.0 (warning 1287), and on 8.0 window functions replace it.

##### CTEs and recursion [8.0+]

```sql
WITH RECURSIVE r (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r WHERE n < 5)
SELECT GROUP_CONCAT(n) FROM r;                                  -- 1,2,3,4,5

WITH RECURSIVE r (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r)
SELECT COUNT(*) FROM r;
-- ERROR 3636 (HY000): Recursive query aborted after 1001 iterations. Try increasing @@cte_max_recursion_depth to a larger value.

WITH RECURSIVE r (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r LIMIT 10)
SELECT COUNT(*) FROM r;                                         -- 10 (LIMIT in the recursive CTE, 8.0.19+)
```

Recursive CTEs over data (trees, graphs) need a termination condition that does not depend on the data being acyclic: carry a depth counter or a path and stop on cycles. Raising `cte_max_recursion_depth` globally hides runaway queries; set it per session (`SET SESSION` or `/*+ SET_VAR(cte_max_recursion_depth = N) */`) only where needed. Non-recursive CTEs are handled like derived tables (merged or materialized by the optimizer); PostgreSQL's `WITH x AS MATERIALIZED (...)` is a syntax error (use the `MERGE`/`NO_MERGE` optimizer hints instead). On 5.7 there are no CTEs: use derived tables, and for hierarchies a closure table or application-side traversal.

##### Set operations, VALUES, TABLE

```sql
SELECT 1 AS x UNION SELECT 2 INTERSECT SELECT 2;     -- 1, 2 (INTERSECT binds tighter than UNION; 8.0.31+)
(SELECT id FROM par) EXCEPT (SELECT 1);              -- INTERSECT/EXCEPT: 8.0.31+
TABLE par ORDER BY id LIMIT 1;                        -- TABLE statement: 8.0.19+
SELECT * FROM (VALUES ROW(1, 'a'), ROW(2, 'b')) AS v (id, name);   -- VALUES needs ROW(); 8.0.19+
```

Before 8.0.31, emulate `INTERSECT` with `EXISTS` and `EXCEPT` with `NOT EXISTS`. `UNION` removes duplicates (extra sort/hash); use `UNION ALL` unless de-duplication is intended.

##### GROUP_CONCAT truncation

```sql
SELECT LENGTH(GROUP_CONCAT(email)) FROM users WHERE id <= 200;   -- 1024
-- Warning 1260: Row 55 was cut by GROUP_CONCAT()
```

`group_concat_max_len` defaults to 1024 bytes and truncation is only a warning. Code that builds ID lists or JSON with `GROUP_CONCAT` silently loses data. Use `JSON_ARRAYAGG()`/`JSON_OBJECTAGG()` (5.7.22+; limited by `max_allowed_packet`: 64 MiB by default on 8.0, 4 MiB on 5.7) or raise `group_concat_max_len` for the session.

##### JSON functions

```sql
SELECT doc->'$.name'  AS arrow,      -- "n1"  (JSON, quoted; 5.7.9+)
       doc->>'$.name' AS unquote,    -- n1    (text, utf8mb4_bin; 5.7.13+)
       JSON_VALUE(doc, '$.age' RETURNING UNSIGNED) AS age    -- 8.0.21+
FROM docs WHERE id = 1;

SELECT JSON_EXTRACT('{"a": 1}', '$.b') IS NULL;              -- 1: missing path is SQL NULL
SELECT JSON_EXTRACT('{"a": null}', '$.a') IS NULL;           -- 0: JSON null is not SQL NULL
SELECT JSON_TYPE(JSON_EXTRACT('{"a": null}', '$.a')) = 'NULL';   -- 1

SELECT jt.* FROM JSON_TABLE(                                  -- 8.0.4+
  '[{"id":1,"v":"a"},{"id":2,"v":"b"},{"id":"x"}]', '$[*]'
  COLUMNS (rid FOR ORDINALITY,
           id INT PATH '$.id' DEFAULT '0' ON ERROR,
           v VARCHAR(5) PATH '$.v' NULL ON EMPTY)) AS jt;
-- 1 | 1 | a ;  2 | 2 | b ;  3 | 0 | NULL
```

`->` and `->>` only accept a column on the left (`CAST('{"a":1}' AS JSON) ->> '$.a'` is `ERROR 1064`); use `JSON_EXTRACT()`/`JSON_UNQUOTE()` for expressions. Distinguish JSON `null` from a missing key when validating input. `JSON_MERGE()` is deprecated (warning 1287): say `JSON_MERGE_PRESERVE()` or `JSON_MERGE_PATCH()` (8.0, and 5.7.22+).

##### Optimizer hints and index hints

```sql
SELECT /*+ MAX_EXECUTION_TIME(100) */ COUNT(*) FROM orders a JOIN orders b ON a.total = b.total;
-- ERROR 3024 (HY000): Query execution was interrupted, maximum statement execution time exceeded
SELECT /*+ MAX_EXECUTION_TIME(100) */ SLEEP(1);   -- returns 1 (interrupted), no error
SELECT /*+ SET_VAR(sort_buffer_size = 1048576) */ 1;   -- 8.0+; 5.7: Warning 1064, hint ignored

-- A hint can force a worse plan than the optimizer would choose:
EXPLAIN FORMAT=TREE SELECT /*+ INDEX(orders ix_orders_status_created) */ id
FROM orders WHERE user_id = 5 AND status = 'paid';
-- -> Filter: (orders.user_id = 5)
--     -> Index lookup on orders using ix_orders_status_created (status='paid')  (rows=99755)
```

- Optimizer hints (`/*+ ... */`, 5.7.7+; `SET_VAR` 8.0+, `INDEX`/`NO_INDEX`/`JOIN_INDEX`/`ORDER_INDEX`/`GROUP_INDEX` 8.0.20+)[^33] are preferred over `USE/FORCE/IGNORE INDEX`: they are scoped to a query block. Every hint is a maintenance liability: it does not adapt when data changes. Dropping or renaming an index interacts differently with the two kinds: an optimizer hint naming a missing index only raises `Warning 3128 Unresolved name ... for INDEX hint` and is ignored, while `FORCE INDEX (ix_gone)` / `USE INDEX (ix_gone)` makes the query fail with `ERROR 1176 Key 'ix_gone' doesn't exist in table 'orders'`. Search the code for index hints before dropping or renaming an index, and require a comment explaining the measured plan for every hint.
- `MAX_EXECUTION_TIME` / `max_execution_time` (5.7.8+) apply to read-only `SELECT` only; `SLEEP()` returns 1 when interrupted rather than erroring.
- Hints live in comments and disappear when a client or proxy strips comments; 8.0.41 fixed the `mysql` client's handling of `#` and `--` inside hint comments.[^21]

##### Miscellaneous performance rules

- `ORDER BY RAND() LIMIT n` sorts the whole table; pick random IDs in the application or sample a key range.
- `SELECT *` matters on wide tables, with `TEXT`/`BLOB`/`JSON` columns (off-page storage) and when a covering index could serve the query.
- `DISTINCT` to fix duplicate rows from a join usually hides a wrong join; fix the join (or use `EXISTS`).
- Implicit comma joins are equivalent to `JOIN` but easy to get wrong; prefer explicit `JOIN ... ON`.
- Large `DELETE`/`UPDATE` in one statement create a long transaction, a huge undo log and replica lag; batch by primary-key ranges (see Writes).
- 5.7 had a query cache (off by default, global mutex); 8.0 removed it. Do not tune or rely on it; cache in the application.

***

#### Writes: INSERT, Upserts, UPDATE, DELETE

##### Batch inserts

Multi-row `INSERT ... VALUES (...), (...)` in transactions of a few thousand rows is the standard fast path. `LOAD DATA INFILE` reads files on the server (restricted by `secure_file_priv`, `/var/lib/mysql-files/` in the official image, and the `FILE` privilege); `LOAD DATA LOCAL INFILE` needs `local_infile=ON` on the server (default `OFF` on 8.0; `ON` on 5.7) **and** client opt-in, and lets the server request any file the client can read — keep it off unless required. There is no `COPY`.

##### INSERT ... ON DUPLICATE KEY UPDATE

Use the row-alias syntax (8.0.19+); `VALUES(col)` in the update clause is deprecated since 8.0.20.[^34] On 5.7 and 8.0 < 8.0.19, `VALUES(col)` is the only form and is correct there.

```sql
CREATE TABLE inv (sku VARCHAR(20) PRIMARY KEY, qty INT NOT NULL, id INT AUTO_INCREMENT UNIQUE);
INSERT INTO inv (sku, qty) VALUES ('a', 1);

-- BAD on 8.0.20+ (deprecated); the only option on 5.7
INSERT INTO inv (sku, qty) VALUES ('a', 5) ON DUPLICATE KEY UPDATE qty = qty + VALUES(qty);
-- Warning 1287: 'VALUES function' is deprecated and will be removed in a future release. Please use an
--   alias (INSERT INTO ... VALUES (...) AS alias) and replace VALUES(col) in the ON DUPLICATE KEY UPDATE
--   clause with alias.col instead

-- GOOD (8.0.19+)
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

```sql
INSERT INTO inv (sku, qty) VALUES ('a', 11) ON DUPLICATE KEY UPDATE qty = 11;
SELECT ROW_COUNT();                  -- 0: row exists, nothing changed
```

- **`AUTO_INCREMENT` values are consumed by the attempted insert even when the row is updated**, so IDs have gaps; upsert-heavy tables can exhaust an `INT` key quickly.

```sql
INSERT INTO inv (sku, qty) VALUES ('z1', 1), ('z2', 2);
SELECT LAST_INSERT_ID();             -- 8: the FIRST id of the multi-row insert
SELECT sku, id FROM inv ORDER BY id; -- a 1, b 3, z1 8, z2 9  (2, 4, 5, 6, 7 burnt by the upserts above)
```

- **`innodb_autoinc_lock_mode` default changed**: 8.0 uses `2` (interleaved: no table-level AUTO-INC lock, ids of concurrent multi-row inserts interleave); 5.7 used `1` (consecutive). Code that assumes one `INSERT ... SELECT` gets a contiguous id range is wrong on 8.0, and `2` requires row-based binary logging (the default `binlog_format=ROW`).[^35]
- **Multiple unique keys**: if the new row conflicts with different rows on different unique keys, only one row is updated and which one is not specified. Upsert only against tables with a single unique constraint relevant to the conflict, or make the logic explicit.
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

Under REPEATABLE READ, the snapshot is taken at the **first read** in the transaction, not at `START TRANSACTION` (use `START TRANSACTION WITH CONSISTENT SNAPSHOT` to take it immediately). Plain `SELECT` reads the snapshot; `UPDATE`, `DELETE`, `INSERT ... SELECT` and locking reads see the latest committed data.[^36] Read-modify-write in application code therefore loses updates:

```sql
-- acct as defined in the next section; two concurrent sessions
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

REPEATABLE READ in MySQL does **not** detect write skew or raise serialization errors like PostgreSQL's REPEATABLE READ/SERIALIZABLE. `SERIALIZABLE` turns plain `SELECT`s into `FOR SHARE` reads (with autocommit off) and greatly increases locking; it is rarely the right fix. `FOR SHARE` is 8.0+; on 5.7 write `LOCK IN SHARE MODE` (still accepted on 8.0).

##### Locks taken by locking reads and writes

InnoDB locks **every index record it scans**, plus the gap before it (next-key locks) under REPEATABLE READ.[^37] `performance_schema.data_locks` (8.0+) shows all locks held; 5.7's `information_schema.innodb_locks` showed only locks involved in a wait and is removed in 8.0.

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
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
```

Review consequences:

- A locking read or `UPDATE`/`DELETE` whose `WHERE` clause is not served by an index locks the whole table in REPEATABLE READ. Make sure such statements use an index (check with `EXPLAIN`).
- "Select-then-insert if absent" with `SELECT ... FOR UPDATE` on a missing key takes a gap lock; two sessions doing it concurrently both get the gap lock and then deadlock on their inserts. Use `INSERT ... ON DUPLICATE KEY UPDATE` or insert first and handle error 1062.
- READ COMMITTED (requires row-based binlogging, the default) takes far fewer gap locks and is a reasonable choice for OLTP services that do not rely on repeatable snapshots. Set it explicitly per session or in the connection string (`transaction_isolation`; the 5.7 name `tx_isolation` is removed in 8.0), and review code that assumed a stable snapshot.

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

##### Queues: SKIP LOCKED and NOWAIT [8.0+]

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

`SKIP LOCKED` needs an index that matches the filter and order; otherwise each worker scans (and locks) far more rows. `FOR UPDATE OF tbl` restricts locking to the named tables in a join. On **5.7** none of this exists (`ERROR 1064`): job queues there need an atomic claim instead, e.g. `UPDATE jobs SET status = 'running', owner = ? WHERE status = 'ready' ORDER BY run_at LIMIT 1` followed by `SELECT ... WHERE owner = ?`, which serializes workers on the first row.

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
-- BAD (MariaDB syntax, ERROR 1064 on MySQL 8.0)
ALTER TABLE x1 ADD COLUMN IF NOT EXISTS b INT;
CREATE INDEX IF NOT EXISTS ix ON x1 (id);
DROP INDEX IF EXISTS ix ON x1;

-- GOOD: one statement, explicit algorithm so it fails fast instead of silently copying
ALTER TABLE orders
  ADD COLUMN note VARCHAR(100) NULL,
  ADD COLUMN source VARCHAR(20) NULL,
  ALGORITHM = INSTANT;                     -- 8.0.12+ (last position), any position 8.0.29+
```

`CREATE TABLE IF NOT EXISTS`, `DROP TABLE IF EXISTS`, `CREATE OR REPLACE VIEW` and `DROP VIEW IF EXISTS` exist; `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS` and `DROP INDEX IF EXISTS` do not.

**Atomic DDL (8.0+).** On 8.0 each DDL statement is all-or-nothing; on 5.7 a failing multi-object statement leaves partial results:

```sql
CREATE TABLE at1 (id INT);
DROP TABLE at1, does_not_exist;      -- ERROR 1051 (42S02): Unknown table 't.does_not_exist'
SHOW TABLES LIKE 'at1';              -- 8.0.46: at1 still exists     5.7.44: at1 was dropped
```

##### ALGORITHM and LOCK

Specify `ALGORITHM` (and `LOCK` for `INPLACE`) on every `ALTER TABLE` against a large table. Without it MySQL silently picks the "best" algorithm, which may be a full table copy that blocks writes for hours. With it, an unsupported combination fails immediately.[^38]

Verified on 8.0.46:

| Operation | INSTANT | INPLACE, LOCK=NONE | Notes |
|---|---|---|---|
| `ADD COLUMN` (nullable or with default) | yes | — | last position since 8.0.12; any position (`FIRST`/`AFTER`) since 8.0.29; consumes a row version (8.0.29+) |
| `DROP COLUMN` | yes (8.0.29+) | yes (rebuild) | consumes a row version |
| `RENAME COLUMN` | yes (8.0.28+) | yes | |
| `ALTER COLUMN ... SET/DROP DEFAULT` | yes | | |
| Add `VIRTUAL` generated column | yes | | |
| Add `STORED` generated column | no | no | `ALGORITHM=COPY` only |
| `MODIFY VARCHAR(50) → VARCHAR(60)` (utf8mb4) | no | yes | length prefix stays 1 byte (≤ 255 bytes) |
| `MODIFY VARCHAR(60) → VARCHAR(100)` (utf8mb4) | no | **no** | 240 → 400 bytes crosses the 255-byte length-prefix boundary: COPY |
| Change column type (`INT → BIGINT`) | no | **no** | COPY, blocks writes |
| `NULL → NOT NULL` | no | yes (rebuild) | fails with `ERROR 1138 Invalid use of NULL value` if rows contain `NULL` |
| `ADD INDEX` | no | yes | |
| `RENAME INDEX`, `ALTER INDEX ... INVISIBLE` | no | yes | metadata-only |
| `ADD FOREIGN KEY` | no | only with `foreign_key_checks=0` | with checks on: COPY (and validation) |
| `ADD CHECK` constraint | no | no | COPY (validates all rows) |
| `CONVERT TO CHARACTER SET` (actual change) | no | no | COPY |
| `DROP PRIMARY KEY, ADD PRIMARY KEY` | no | yes (rebuild) | |

```sql
ALTER TABLE dd MODIFY a VARCHAR(60), ALGORITHM=INSTANT;
-- ERROR 1845 (0A000): ALGORITHM=INSTANT is not supported for this operation. Try ALGORITHM=COPY/INPLACE.
ALTER TABLE dd MODIFY a VARCHAR(100), ALGORITHM=INPLACE, LOCK=NONE;
-- ERROR 1846 (0A000): ALGORITHM=INPLACE is not supported. Reason: Cannot change column type INPLACE. Try ALGORITHM=COPY.
ALTER TABLE d2 ADD CONSTRAINT fk_d2 FOREIGN KEY (pid) REFERENCES ddp (id), ALGORITHM=INPLACE;
-- ERROR 1846 (0A000): ALGORITHM=INPLACE is not supported. Reason: Adding foreign keys needs foreign_key_checks=OFF. Try ALGORITHM=COPY.
```

On **5.7** there is no `INSTANT` (`ERROR 1800 Unknown ALGORITHM 'INSTANT'`): `ADD COLUMN` is `INPLACE` with a full table rebuild. Even `INPLACE, LOCK=NONE` operations rebuild the table (I/O, temporary disk space up to the table size, replica lag because the replica applies the ALTER after it finished on the source, serially). Online DDL also buffers concurrent writes in `innodb_online_alter_log_max_size` (128 MiB default); if it overflows, the ALTER fails at the end. For very large tables use `gh-ost` or `pt-online-schema-change` and review their configuration like a migration.

##### INSTANT row-version limit and 8.0.29 [8.0.29+]

Each `INSTANT` `ADD COLUMN`/`DROP COLUMN` increments the table's row version; MySQL 8.0 allows **64**:[^38]

```sql
-- after 64 instant add/drop operations:
ALTER TABLE rv ADD COLUMN c33 INT, ALGORITHM=INSTANT;
-- ERROR 4092 (HY000): Maximum row versions reached for table t/rv. No more columns can be added or dropped instantly. Please use COPY/INPLACE.
SELECT total_row_versions FROM information_schema.innodb_tables WHERE name = 't/rv';   -- 64
ALTER TABLE rv ADD COLUMN z INT;        -- no ALGORITHM: silently falls back to a table rebuild (versions reset to 0)
```

Without an explicit `ALGORITHM=INSTANT`, the 65th change quietly becomes a full rebuild during the deploy. Projects that add and drop columns frequently (feature flags, ORMs) should watch `total_row_versions`. Since **8.0.41**, `ADD`/`DROP COLUMN` on an **empty** table without an `ALGORITHM` clause uses `INPLACE` and does not consume a row version; an explicit `ALGORITHM=INSTANT` still consumes one (verified on 8.0.46).[^21]

**8.0.29 was withdrawn** because tables with instantly added or dropped columns could be read incorrectly. A project still on 8.0.29 must upgrade; tables altered with `INSTANT` on 8.0.29 should be rebuilt (`ALTER TABLE ... FORCE` or `OPTIMIZE TABLE`) after upgrading, per the release-note guidance.[^17] On 8.0.12–8.0.28, `INSTANT` only adds a column at the end and cannot drop columns.

##### Metadata locks: the most common migration outage

Every DDL, even `ALGORITHM=INSTANT`, needs an exclusive metadata lock (MDL) on the table for a moment. It waits for **every open transaction that has touched the table** — including an idle transaction that only ran a `SELECT`. While it waits, **every new query on the table queues behind it**:[^39]

```sql
-- Session A (e.g. a pooled connection left in a transaction)
START TRANSACTION;
SELECT COUNT(*) FROM acct;
-- ... stays open

-- Session B (the migration)
ALTER TABLE acct ADD COLUMN extra INT, ALGORITHM=INSTANT;      -- waits

-- Session C (normal traffic): blocked behind B for as long as A stays open
SELECT COUNT(*) FROM acct;

SELECT id, state, LEFT(info, 60) FROM performance_schema.processlist WHERE state LIKE '%metadata%';  -- 8.0.22+
-- 38 | Waiting for table metadata lock | ALTER TABLE acct ADD COLUMN extra INT, ALGORITHM=INSTANT
-- 39 | Waiting for table metadata lock | SELECT COUNT(*) FROM acct
SELECT object_name, lock_type, lock_status FROM performance_schema.metadata_locks WHERE object_name = 'acct';
-- acct | SHARED_READ       | GRANTED
-- acct | SHARED_UPGRADABLE | GRANTED
-- acct | EXCLUSIVE         | PENDING
-- acct | SHARED_READ       | PENDING
```

`information_schema.processlist` works too but raises deprecation warning 1287 on 8.0.35+. On 5.7 the `wait/lock/metadata/sql/mdl` instrument is disabled by default (`performance_schema.metadata_locks` stays empty until it is enabled in `setup_instruments`); use `SHOW PROCESSLIST` there. On 8.0.35+ a plain `SHOW PROCESSLIST` also prints warning 1287, because by default it is implemented on top of `INFORMATION_SCHEMA.PROCESSLIST`; query `performance_schema.processlist` instead.

`lock_wait_timeout` (which governs MDL waits) defaults to **31,536,000 seconds (one year)** on 8.0 and 5.7. Every migration session should set a short value and retry:

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

Partitioning is a data-lifecycle tool (drop a month with `ALTER TABLE ... DROP PARTITION`), not a general speed-up. Every unique key (including the primary key) must include all partitioning columns, partitioned InnoDB tables cannot have foreign keys, and queries that do not filter on the partition key touch all partitions. 8.0 supports partitioning only for InnoDB (and NDB); 5.7 MyISAM partitioned tables must be converted before upgrading. Index prefixes in partitioning keys are not allowed:

```sql
CREATE TABLE pt (id INT, s VARCHAR(20), PRIMARY KEY (id, s(5))) PARTITION BY KEY (id, s) PARTITIONS 2;
-- ERROR 1503 (HY000): A PRIMARY KEY must include all columns in the table's partitioning function
--   (prefixed columns are not considered).
```

***

#### Stored Programs, Triggers, Views and Events

##### Stored functions and binary logging

With binary logging enabled (the default on 8.0; **off** by default on 5.7, so this error appears only after enabling it there), creating a stored function requires a characteristic that declares it safe:[^40]

```sql
DELIMITER //
CREATE FUNCTION f_nd(x INT) RETURNS INT BEGIN RETURN x + 1; END//
-- ERROR 1418 (HY000): This function has none of DETERMINISTIC, NO SQL, or READS SQL DATA in its
--   declaration and binary logging is enabled (you *might* want to use the less safe
--   log_bin_trust_function_creators variable)
CREATE FUNCTION f_ok(x INT) RETURNS INT DETERMINISTIC NO SQL BEGIN RETURN x + 1; END//
DELIMITER ;
```

Declaring `DETERMINISTIC` on a function that is not (uses `NOW()`, `RAND()`, `UUID()`, reads tables that change) is a lie the optimizer and replication rely on; flag it. Setting `log_bin_trust_function_creators=ON` (deprecated since 8.0.34) globally to make migrations pass is a security and replication smell. `DELIMITER` is a client command of the `mysql` CLI, not SQL: migration tools and drivers that send statements one at a time must not include it.

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
PREPARE pe FROM 'CREATE EVENT e1 ON SCHEDULE EVERY 1 DAY DO SELECT 1';
-- ERROR 1295 (HY000): This command is not supported in the prepared statement protocol yet  (allowed from 9.0)
```

##### DEFINER and SQL SECURITY

Views, routines, triggers and events run with the privileges of their `DEFINER` by default (`SQL SECURITY DEFINER`). A view or procedure defined by a privileged account is a privilege-escalation path. Review:

- `DEFINER` clauses in migrations: dumps from one environment often carry `DEFINER=root@localhost` or a personal account that does not exist elsewhere. On 8.0, setting a definer other than yourself needs `SET_USER_ID` (or `SUPER`); 8.4 replaces it with `SET_ANY_DEFINER`/`ALLOW_NONEXISTENT_DEFINER`, so grant scripts differ between the two.[^23]
- Prefer `SQL SECURITY INVOKER` for views and routines that only encapsulate queries.

##### Triggers

- Triggers do not fire for foreign-key cascades (shown in Foreign keys) and are skipped by `TRUNCATE TABLE`.
- Row-by-row triggers multiply the cost of bulk statements; complex business logic in triggers is invisible to application reviewers. Keep them small (auditing, denormalized counters, `CHECK` emulation on 5.7) and covered by tests.
- A trigger cannot modify the table that fired it:

```sql
CREATE TRIGGER par_bi BEFORE INSERT ON par FOR EACH ROW INSERT INTO par VALUES (99, 'x');
INSERT INTO par VALUES (50, 'y');
-- ERROR 1442 (HY000): Can't update table 'par' in stored function/trigger because it is already used by
--   statement which invoked this stored function/trigger.
DROP TRIGGER par_bi;
```

##### Views

MySQL has no materialized views; `CREATE MATERIALIZED VIEW` is a syntax error. Emulate with a summary table refreshed by a job or maintained by the application. `CREATE VIEW IF NOT EXISTS` is 9.1+; use `CREATE OR REPLACE VIEW`. Views with `GROUP BY`, `DISTINCT`, aggregates, `UNION` or `LIMIT` are evaluated with a temporary table (`ALGORITHM=TEMPTABLE`) and predicates from the outer query may not be pushed into them (8.0.22+ derived condition pushdown helps for some derived tables); check plans of queries on views.

##### Events

The event scheduler is `ON` by default since 8.0 (**`OFF` on 5.7**: events created there never run unless `event_scheduler=ON` is configured). Events run with the definer's privileges on the source; on replicas, replicated events are created disabled. On 8.0 write `DISABLE ON SLAVE`; `DISABLE ON REPLICA` is a syntax error before 8.2.0, and `DISABLE ON SLAVE` is deprecated from 8.2 on. `CREATE/ALTER/DROP EVENT` cannot be run as prepared statements on 8.0 (`ERROR 1295`).

***

#### Security

##### SQL injection: parameters for values, allow-lists for identifiers

Every value must be sent as a bound parameter. Identifiers (table, column, `ORDER BY` column, `ASC`/`DESC`) cannot be parameters and must come from a fixed allow-list in code. Verified with Go `database/sql` and `github.com/go-sql-driver/mysql` v1.10.1 against 8.0.46:[^41]

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

##### Authentication: caching_sha2_password vs mysql_native_password

8.0 creates new accounts with `caching_sha2_password` (`default_authentication_plugin`, deprecated since 8.0.27 in favour of `authentication_policy`, default `*,,`). `mysql_native_password` still works on 8.0 — `CREATE USER ... IDENTIFIED WITH mysql_native_password` succeeds without an SQL warning; since 8.0.34 authenticating with it writes a deprecation warning to the error log (since 8.0.38 only once, not per login: three logins on 8.0.46 produced one entry) — but it is **disabled by default on 8.4** and removed in 9.0.[^42]

- The first login of a `caching_sha2_password` account after a server restart or `FLUSH PRIVILEGES` (cache empty) needs either a **TLS connection** or RSA public-key exchange; over plain TCP without either, the login fails (verified on 8.0.46):

```text
$ mysql -uapp2 -p... -h127.0.0.1 --ssl-mode=DISABLED
ERROR 2061 (HY000): Authentication plugin 'caching_sha2_password' reported error: Authentication requires secure connection.
$ mysql -uapp2 -p... -h127.0.0.1 --ssl-mode=DISABLED --get-server-public-key
(succeeds; afterwards the cached fast path works without TLS until the cache is cleared)
```

  This is the classic "works until the database restarts" bug. Use TLS (the server auto-generates certificates; clients default to `PREFERRED`), and for production require it: `require_secure_transport=ON` on the server or `REQUIRE SSL` per account, plus `VERIFY_IDENTITY` in clients. Flag driver options such as `allowPublicKeyRetrieval=true` (JDBC) or `tls=skip-verify` (Go) in production configs: they accept a man-in-the-middle key.
- **BAD (8.0)**: new accounts `IDENTIFIED WITH mysql_native_password`, or `default_authentication_plugin=mysql_native_password` in `my.cnf` "to make old clients work". Both block the 8.4 upgrade (the account cannot log in there unless the server runs with `--mysql-native-password=ON`, and the variable no longer exists). Upgrade the client library instead and migrate accounts: `ALTER USER app@'%' IDENTIFIED WITH caching_sha2_password BY '...'`. Find leftovers with `SELECT user, host, plugin FROM mysql.user WHERE plugin = 'mysql_native_password'`.
- Old client libraries (very old PHP `mysqlnd`, Python `MySQLdb` builds against libmysqlclient 5.x, the Node `mysql` package) do not support `caching_sha2_password`; `mysql2` and current connectors do.
- 5.7 has only `mysql_native_password` and `sha256_password`; its password hashes are unsalted SHA1-based. A 5.7 server should at least require TLS.
- Account management (8.0): `CREATE USER ... IDENTIFIED BY`, `ALTER USER`, password policies (`PASSWORD EXPIRE`, `PASSWORD HISTORY`, `FAILED_LOGIN_ATTEMPTS n PASSWORD_LOCK_TIME d` since 8.0.19). `GRANT` no longer creates users (`ERROR 1410`), and `PASSWORD()` is removed (`ERROR 1064`). On 5.7, `GRANT ... IDENTIFIED BY` and `SET PASSWORD = PASSWORD('x')` still work with deprecation warnings; do not write new code that relies on them.

##### Least privilege and roles [8.0+]

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

Review rules:[^43][^44]

- Application accounts get DML on their schema only; migrations run under a separate account with DDL privileges. No `ALL PRIVILEGES ON *.*`, no `SUPER` (deprecated on 8.0 in favour of dynamic privileges such as `SYSTEM_VARIABLES_ADMIN`, `CONNECTION_ADMIN`), `FILE`, `PROCESS`, `SYSTEM_USER` or `GRANT OPTION` for applications.
- Every role grant needs `SET DEFAULT ROLE` (or `SET ROLE` in the session); otherwise the account has no effective privileges. On 5.7 there are no roles: grant privileges per account.
- `_` and `%` in database names in `GRANT ... ON db_name.*` are wildcards: `GRANT SELECT ON db_1.*` also matches `dbx1`. Escape them (`` `db\_1` ``) or avoid them in schema names. The wildcard behaviour is deprecated since 8.0.35 without any warning.[^20]
- Host part: prefer specific hosts or network ranges over `'%'`; `'localhost'` matches only socket/`localhost` connections.
- Credentials never in migrations or code; use environment variables or a secret store, and do not log DSNs (the Go driver's `FormatDSN()` contains the password).

##### Other server-side security settings

- `local_infile=OFF` (8.0 default; `ON` on 5.7): keep it off; `LOAD DATA LOCAL` lets a malicious server read client files.
- `secure_file_priv` restricts `LOAD DATA INFILE` / `SELECT ... INTO OUTFILE` to one directory; an empty value (`""`) allows the whole filesystem.
- `MD5()`, `SHA1()`, `SHA2()` are fast hashes, unsuitable for passwords; hash passwords in the application with bcrypt/scrypt/Argon2. (`MD5()`/`SHA1()` are deprecated in 9.4 and moved into a component in 9.6.)
- TLS: 8.0.28+ supports only TLSv1.2 and TLSv1.3 (`@@tls_version` = `TLSv1.2,TLSv1.3` on 8.0.46).[^16]
- `AES_ENCRYPT()` defaults to `block_encryption_mode=aes-128-ecb`: deterministic, no IV, leaks equal plaintexts. There is no authenticated (GCM) mode. Prefer encrypting in the application; if it must happen in SQL, use a CBC mode with a random IV per value and keep keys out of SQL text (they appear in logs and the processlist):

```sql
SELECT HEX(AES_ENCRYPT('secret', 'key')) = HEX(AES_ENCRYPT('secret', 'key')) AS ecb_deterministic;  -- 1
SET SESSION block_encryption_mode = 'aes-256-cbc';
SET @iv = RANDOM_BYTES(16);
SET @key = UNHEX(SHA2('k', 256));               -- 32 raw bytes
SELECT AES_DECRYPT(AES_ENCRYPT('secret', @key, @iv), @key, @iv);   -- secret
SELECT AES_ENCRYPT('secret', 'key');            -- ERROR 1582: CBC requires the IV argument
SET SESSION block_encryption_mode = 'aes-256-gcm';   -- ERROR 1231: not supported
SET SESSION block_encryption_mode = DEFAULT;
```

- Logs (general log, slow log, binlog, `performance_schema` digests and history) contain literal values from non-parameterized statements: another reason to bind parameters.

***

#### Application Code and Drivers

Verified with `github.com/go-sql-driver/mysql` v1.10.1 against 8.0.46; the same concepts apply to other drivers.[^41]

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
- `interpolateParams=true` makes the driver substitute parameters client-side to save a round trip; escaping is unsafe with multibyte encodings whose trailing bytes can be `0x5c` (BIG5, CP932, GBK, GB18030, SJIS). v1.10.1 only rejects the combination when the DSN's `collation` is one of those (`invalid DSN: interpolateParams can not be used with unsafe collations`); `charset=sjis&interpolateParams=true` is accepted without error, so flag `interpolateParams` together with any such charset. Leave it off unless measured.
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
	case 3819: // ER_CHECK_CONSTRAINT_VIOLATED (8.0.16+)
	}
}
```

- Use `context` deadlines (`QueryContext`, `ExecContext`). On cancellation the Go driver abandons the connection, but the server may keep executing the statement until it notices the disconnect; bound expensive reads with `MAX_EXECUTION_TIME` as well. Always `defer rows.Close()` and check `rows.Err()`.
- Transactions: `BeginTx` with `sql.TxOptions{Isolation: sql.LevelReadCommitted}` sets the isolation per transaction. Never run transaction statements (`START TRANSACTION`, `COMMIT`) through `db.Exec` on a pool — they land on different connections.
- Connector versions: Connector/J 8.0+ (`com.mysql.cj.jdbc.Driver`), recent PHP `mysqlnd`, Python `mysqlclient`/`PyMySQL`/`mysql-connector-python` and Node `mysql2` support `caching_sha2_password`; flag the unmaintained Node `mysql` package and Connector/J 5.1.

***

#### Replication and Operations Awareness

Code review should still catch replication hazards, even with `binlog_format=ROW` as the default (since 5.7.7):

- Use the new terminology in scripts and tooling where the pinned version supports it: `START REPLICA`/`STOP REPLICA`/`SHOW REPLICA STATUS`/`SHOW REPLICAS` (8.0.22+), `CHANGE REPLICATION SOURCE TO SOURCE_HOST=...` (8.0.23+). The `MASTER`/`SLAVE` forms print warning 1287 on 8.0 and are **syntax errors on 8.4**. 8.0 has no replacement yet for `SHOW MASTER STATUS` and `RESET MASTER` (they print no warning; 8.4 renames them `SHOW BINARY LOG STATUS` and `RESET BINARY LOGS AND GTIDS`), so scripts that must run on both need a version switch. Monitoring that reads `Seconds_Behind_Master` must read `Seconds_Behind_Source` (8.0.22+ output).[^14][^15]
- Large single transactions (bulk `UPDATE`/`DELETE`, `ALTER TABLE` copies) create replica lag proportional to their duration. Batch them.
- Statements with nondeterministic results (`LIMIT` without `ORDER BY`, `SYSDATE()`, `UUID()` in statement-based setups) are only safe because of row-based logging; keep `binlog_format=ROW` (the variable is deprecated since 8.0.34; future versions support only row-based logging).[^19]
- Binary logging is **on** by default since 8.0 (`log_bin=1`), off on 5.7; binlogs expire after `binlog_expire_logs_seconds` = 30 days (`expire_logs_days` is deprecated). Plan disk accordingly.
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

- The `sys` schema (5.7.7+) views above are installed by default.[^45]
- Slow query log: `slow_query_log=OFF` and `long_query_time=10` by default; enable it with a threshold that matches the service's latency budget (e.g. 0.5–1 s) and analyze with `pt-query-digest` or `mysqldumpslow`. `log_queries_not_using_indexes` floods the log on small tables; use it briefly.
- `SUM_ROWS_EXAMINED / SUM_ROWS_SENT` far above 1 indicates a missing or unused index.
- `sys.schema_unused_indexes` counts only since the last restart and only on this server; an index unused on the source may serve replica reads. Make an index invisible (8.0+) before dropping it.
- `SHOW ENGINE INNODB STATUS` → `LATEST DETECTED DEADLOCK`, `TRANSACTIONS` (history list length = purge lag).
- `EXPLAIN FOR CONNECTION <id>` shows the plan of a running statement.

***

#### Removed / Changed in 8.4 — Migration Notes (from 8.0)

8.0 is end of life; the next step is 8.4 LTS (8.4.13 as of October 2026), then 9.7 LTS. Verified differences between 8.0.46 and 8.4.13:[^23]

1. **Authentication**: `mysql_native_password` is disabled on 8.4 (`CREATE USER ... IDENTIFIED WITH mysql_native_password` → `ERROR 1524 Plugin 'mysql_native_password' is not loaded`) and `default_authentication_plugin` is removed (`ERROR 1193`; a `my.cnf` that sets it stops the server). Migrate every account to `caching_sha2_password` and every client to a library that supports it **before** upgrading.
2. **Replication statements**: `CHANGE MASTER TO`, `START/STOP/RESET SLAVE`, `SHOW SLAVE STATUS`, `SHOW SLAVE HOSTS`, `SHOW MASTER STATUS`, `SHOW MASTER LOGS`, `RESET MASTER`, `PURGE MASTER LOGS` and all `MASTER_*` options become `ERROR 1064` on 8.4 (`EVENT ... DISABLE ON SLAVE` still works there, with warning 1287). Replace them with the `REPLICA`/`SOURCE` forms (available on 8.0.22/8.0.23) and, after the upgrade, `SHOW BINARY LOG STATUS` / `RESET BINARY LOGS AND GTIDS`.
3. **Foreign keys to non-unique keys**: accepted on 8.0, rejected on 8.4 (`ERROR 6125 Failed to add the foreign key constraint. Missing unique key ...`). Add the missing `UNIQUE` keys while still on 8.0; dumps that create such FKs fail to restore on 8.4.
4. **Removed syntax**: `AUTO_INCREMENT` on `FLOAT`/`DOUBLE` (`ERROR 1063`, blocks the upgrade), `FLUSH HOSTS`, `LOCK TABLES ... LOW_PRIORITY WRITE`, identifiers like `$a$b`; `QUALIFY` and `TABLESAMPLE` become reserved (`CREATE TABLE qualify (...)` works on 8.0.46, fails on 8.4.13).
5. **Removed objects and options**: `INFORMATION_SCHEMA.TABLESPACES`, `--ssl`/`have_ssl`, `expire_logs_days`, `--skip-host-cache`, `binlog_transaction_dependency_tracking`, `master_info_repository`/`relay_log_info_repository`, `transaction_write_set_extraction`, `keyring_file`/`keyring_encrypted_file`/`keyring_oci` plugins, `authentication_fido`; tools `mysql_upgrade`, `mysqlpump`, `mysql_ssl_rsa_setup`. Clean `my.cnf` first: the 8.4 server refuses to start with removed options.
6. **Privileges**: `SET_USER_ID` is replaced by `SET_ANY_DEFINER` and `ALLOW_NONEXISTENT_DEFINER`; `FLUSH PRIVILEGES` gets its own `FLUSH_PRIVILEGES` privilege.
7. **Changed defaults** (observed on 8.0.46 → 8.4.13 in the same container): `innodb_adaptive_hash_index` `ON` → `OFF`, `innodb_change_buffering` `all` → `none`, `innodb_io_capacity` 200 → 10000, `innodb_log_buffer_size` 16 → 64 MiB, `innodb_flush_method` `fsync` → `O_DIRECT`, `temptable_max_mmap` 1 GiB → 0. Re-measure instead of copying the 8.0 `my.cnf`.
8. **Keep deprecated features out of new 8.0 code** so the upgrade is mechanical: `VALUES()` in upserts, `SQL_CALC_FOUND_ROWS`, `utf8mb3`, `&&`/`||`, display widths, `ZEROFILL`, user-variable assignment in expressions, wildcard database grants, inline column `REFERENCES` (still ignored on 8.4, enforced on 9.x).
9. **Upgrade mechanics**: run the MySQL Shell upgrade checker (`util.checkForServerUpgrade()`) against the 8.0 server; take a logical or physical backup; upgrade in place (8.0.46 → 8.4.x is supported) or via a replica on 8.4. 8.0 → 9.7 directly is not supported, because an LTS series cannot be skipped.[^7][^51]

**Coming from 5.7 (to 8.0, then 8.4)**: 5.7 → 8.0 is an in-place upgrade with a data-dictionary conversion that cannot be downgraded. Before it: remove `NO_AUTO_CREATE_USER` from all `sql_mode` settings, quote new reserved words, replace `GRANT ... IDENTIFIED BY` user creation, drop or fix `CHECK` clauses whose data violates them, convert non-InnoDB partitioned tables, set explicit character sets/collations so new tables match old ones, add `ORDER BY` wherever `GROUP BY` order was relied upon, and decide whether to keep `explicit_defaults_for_timestamp`-dependent DDL. The upgrade checker reports most of these.[^9]

***

#### Production / Review Checklist

##### Version and support

- [ ] The project knows it runs an **end-of-life** server (8.0 since April 2026, 5.7 since October 2023) and has a dated plan to move to 8.4 LTS; until then it pins 8.0.46 (never 8.0.29 or 8.0.38, which were withdrawn)
- [ ] No feature newer than the pinned version (8.1+/8.4/9.x, see "Do not use"); on 5.7 no **8.0+** feature
- [ ] No new usage of features that 8.4 removes (`mysql_native_password`, `MASTER`/`SLAVE` statements, FKs to non-unique keys, `AUTO_INCREMENT` on floats)

##### Schema and migrations

- [ ] Every table has a narrow, immutable primary key (`BIGINT UNSIGNED AUTO_INCREMENT` or `BINARY(16)`), never `CHAR(36)` UUIDs
- [ ] `utf8mb4` with an explicit collation consistent with existing tables; no `utf8`/`utf8mb3`; `_bin` collation for case-sensitive identifiers; on 5.7 never relying on the `latin1` server default
- [ ] `DECIMAL` for money; `BIGINT` for counters/IDs that grow; no display widths, `ZEROFILL`, `FLOAT(M,D)`
- [ ] `DATETIME(n)` in UTC for business timestamps; no `TIMESTAMP` beyond 2038; explicit `NULL`/`DEFAULT` on every `TIMESTAMP` column; precision matches the application
- [ ] Foreign keys written as table-level `FOREIGN KEY` clauses (inline `REFERENCES` is ignored) referencing a `PRIMARY`/`UNIQUE` key with identical column types
- [ ] No reliance on triggers firing for FK cascades; no `REPLACE` on parent tables
- [ ] `CHECK` constraints for invariants (8.0.16+), with `NOT NULL` where `NULL` must not pass; on 5.7, `CHECK` is not relied upon (it is ignored)
- [ ] Every `ALTER TABLE` on a big table states `ALGORITHM` (and `LOCK`), runs with a short `lock_wait_timeout`, is a single statement per table, and is safe for the old and new application version during deploy
- [ ] No `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, `RETURNING`, `CREATE MATERIALIZED VIEW` or other MariaDB/PostgreSQL syntax
- [ ] `INSTANT` row-version budget (64) considered for frequently altered tables

##### Queries

- [ ] `EXPLAIN` (or `EXPLAIN ANALYZE` on a copy, 8.0.18+) for new queries on large tables; no full scans in hot paths
- [ ] No functions or implicit type conversions on indexed columns; strings bound for string columns
- [ ] Keyset pagination written with the expanded `OR` predicate, not `(a, b) > (?, ?)` and not deep `OFFSET`
- [ ] `NOT EXISTS` instead of `NOT IN (subquery)` on nullable columns
- [ ] Deterministic `ORDER BY` (ending in a unique column) wherever `LIMIT` or `ROW_NUMBER()` is used, and wherever `GROUP BY` output order matters
- [ ] No `ONLY_FULL_GROUP_BY` workarounds (`sql_mode=''`, careless `ANY_VALUE`); no `NO_AUTO_CREATE_USER` in `sql_mode`
- [ ] `GROUP_CONCAT` length considered; `JSON_ARRAYAGG` for lists
- [ ] JSON functional indexes match the query expression and collation (`COLLATE utf8mb4_bin` for `->>`); multi-valued indexes only for `MEMBER OF`/`JSON_CONTAINS`/`JSON_OVERLAPS`
- [ ] Hints justified with a comment; no `FORCE INDEX` on indexes that may be renamed; `MAX_EXECUTION_TIME` or client timeouts on user-facing reads

##### Writes and transactions

- [ ] Upserts use `INSERT ... VALUES (...) AS new ON DUPLICATE KEY UPDATE col = new.col` on 8.0.19+ (`VALUES(col)` only on older servers); affected-rows semantics (1/2/0) handled; `AUTO_INCREMENT` gaps acceptable
- [ ] No `INSERT IGNORE` / `UPDATE IGNORE` except where ignoring every error class is intended
- [ ] Read-modify-write done in one `UPDATE`, under `SELECT ... FOR UPDATE`, or with optimistic version checks
- [ ] Locking statements use an index (REPEATABLE READ locks every scanned row and gap)
- [ ] Deadlock (1213) retries the whole transaction; lock wait timeout (1205) rolls back explicitly
- [ ] Transactions are short, contain no network calls or user waits, never contain DDL
- [ ] Large deletes/updates batched by primary key

##### Security

- [ ] All values bound as parameters; identifiers from allow-lists; `multiStatements` off; LIKE input escaped
- [ ] Application accounts with least privilege; roles activated via `SET DEFAULT ROLE`; no wildcard surprises in database grants
- [ ] `caching_sha2_password` for all accounts; TLS required and verified; no `allowPublicKeyRetrieval`/`skip-verify` in production
- [ ] No new `mysql_native_password` accounts and no `default_authentication_plugin=mysql_native_password`
- [ ] `local_infile` off; `secure_file_priv` set; no passwords hashed with `MD5`/`SHA1`/`SHA2`; no `AES_ENCRYPT` in ECB mode
- [ ] `DEFINER`/`SQL SECURITY` reviewed for views, routines, triggers, events

##### Operations

- [ ] Replication statements and monitoring use `SOURCE`/`REPLICA` terminology where the version allows, with a plan for the 8.4 renames
- [ ] Connection pool lifetime below `wait_timeout`; `parseTime`/time-zone settings consistent
- [ ] Slow query log or `performance_schema` digests reviewed; deadlocks logged (`innodb_print_all_deadlocks`)
- [ ] Histograms refreshed by a job (8.0 has no automatic histogram updates); binlog disk usage planned (`log_bin` on by default)
