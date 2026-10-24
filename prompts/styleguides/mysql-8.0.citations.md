### Citations: MySQL 8.0 (and 5.7): The Complete SQL Review Guide

Sources for `mysql-8.0.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Changes in MySQL 8.0.11 (2018-04-19, first General Availability release). https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-11.html
    - Overview — quote block: "The first GA release, 8.0.11, shipped on **April 19, 2018**."
[^2]: MySQL 8.0 Release Notes (index; End of Life statement; 8.0.46 of 2026-04-21) and Changes in MySQL 8.0.46. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/ , https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-46.html
    - Overview — quote block: "The final release is **8.0.46 (April 21, 2026)**."
    - Overview — quote block: "…with version 8.0.46, MySQL 8.0 reaches End of Life (EoL).""
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 20: "OpenSSL 3.5.5"
[^3]: endoflife.date — MySQL (Premier/Extended support end dates per series). https://endoflife.date/mysql
    - Overview — quote block: "…ended in April 2025 and Extended Support in April 2026;"
    - Overview — quote block: "…**October 2023**; its last release is 5.7.44 (October 25, 2023)."
[^4]: MySQL 5.7 Release Notes (index; 5.7.44 of 2023-10-25 is the last release). https://dev.mysql.com/doc/relnotes/mysql/5.7/en/
    - Overview — quote block: "…**October 2023**; its last release is 5.7.44 (October 25, 2023)."
[^5]: MySQL 8.4 Release Notes (index of 8.4.0–8.4.13 with dates). https://dev.mysql.com/doc/relnotes/mysql/8.4/en/
    - Overview — quote block: "Premier Support until April 2031)."
[^6]: MySQL 9.7 Release Notes (index of 9.0.0–9.7.4 with dates). https://dev.mysql.com/doc/relnotes/mysql/9.7/en/
    - Overview — quote block: "Premier Support until April 2031)."
[^7]: MySQL 8.4 Reference Manual — MySQL Releases: Innovation and LTS (support policy, upgrade paths). https://dev.mysql.com/doc/refman/8.4/en/mysql-releases.html
    - Overview — quote block: "…must go through 8.0 first (**5.7 → 8.0 → 8.4**)."
    - Removed / Changed in 8.4 — Migration Notes (from 8.0) — bullet 9: "…is not supported, because an LTS series cannot be skipped."
[^8]: MySQL 8.0 Reference Manual — What Is New in MySQL 8.0 (features added, deprecated and removed, with versions). https://dev.mysql.com/doc/refman/8.0/en/mysql-nutshell.html
    - What's New in MySQL 8.0 (since 5.7) — paragraph 1: "…most often needs, with the release that introduced each item."
    - What's New in MySQL 8.0 (since 5.7) › Breaking changes for application SQL when moving 5.7 → 8.0 — paragraph 1: "…change meaning on 8.0 (all verified on 5.7.44 vs 8.0.46):"
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 3: "…display width (`INT(11)`), `ZEROFILL`, `FLOAT(M,D)`, `AUTO_INCREMENT` on `FLOAT`/`DOUBLE`, `&&`/`||`/`!`, `SQL_CALC_FOUND_ROWS`/`FOUND_ROWS()`"
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 12: "`--skip-host-cache` deprecated"
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 13: "8.0.31 | `INTERSECT`/`EXCEPT`"
[^9]: MySQL 8.0 Reference Manual — Changes in MySQL 8.0 (upgrading from 5.7). https://dev.mysql.com/doc/refman/8.0/en/upgrading-from-previous-series.html
    - What's New in MySQL 8.0 (since 5.7) › Breaking changes for application SQL when moving 5.7 → 8.0 — paragraph 1: "…change meaning on 8.0 (all verified on 5.7.44 vs 8.0.46):"
    - Removed / Changed in 8.4 — Migration Notes (from 8.0) — paragraph 2: "The upgrade checker reports most of these."
[^10]: MySQL 8.0 Reference Manual — Keywords and Reserved Words (new reserved words in 8.0). https://dev.mysql.com/doc/refman/8.0/en/keywords.html
    - What's New in MySQL 8.0 (since 5.7) › Breaking changes for application SQL when moving 5.7 → 8.0 — table row 7: "`CREATE TABLE cube (...)` fails on 8.0.46)"
[^11]: Changes in MySQL 8.0.13 (2018-10-22) — functional key parts, expression defaults, skip scan, sql_require_primary_key, GROUP BY ASC/DESC removed, user-variable assignment deprecated. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-13.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 1: "…variables in expressions (`SELECT @x := ...`) deprecated (warning 1287)"
[^12]: Changes in MySQL 8.0.16 (2019-04-25) — CHECK constraints, EXPLAIN FORMAT=TREE, server-side upgrade. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-16.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 2: "…ignored); the server upgrades the data dictionary itself, `mysql_upgrade` deprecated"
[^13]: Changes in MySQL 8.0.20 (2020-04-27) — VALUES() deprecation, index-level hints, hash join replaces BNL. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-20.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 5: "…KEY UPDATE` deprecated; hash join replaces block nested loop everywhere"
[^14]: Changes in MySQL 8.0.22 (2020-10-19) — REPLICA statement aliases, performance_schema.processlist, CAST AT TIME ZONE, prepare-once, derived condition pushdown. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-22.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 6: "`ORDER BY ?` no longer means "order by column number")"
    - Replication and Operations Awareness — bullet 1: "Monitoring that reads `Seconds_Behind_Master` must read `Seconds_Behind_Source` (8.0.22+ output)."
[^15]: Changes in MySQL 8.0.23 (2021-01-18) — invisible columns, CHANGE REPLICATION SOURCE TO, FLUSH HOSTS deprecated. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-23.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 7: "`FLUSH HOSTS` deprecated"
    - Replication and Operations Awareness — bullet 1: "Monitoring that reads `Seconds_Behind_Master` must read `Seconds_Behind_Source` (8.0.22+ output)."
[^16]: Changes in MySQL 8.0.28 (2022-01-18) — 64-bit UNIX_TIMESTAMP()/FROM_UNIXTIME()/CONVERT_TZ(), charset deprecations, TLSv1/1.1 removed, RENAME COLUMN INSTANT. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-28.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 10: "TLSv1/TLSv1.1 removed"
    - Security › Other server-side security settings — bullet 4: "…supports only TLSv1.2 and TLSv1.3 (`@@tls_version` = `TLSv1.2,TLSv1.3` on 8.0.46)."
[^17]: Changes in MySQL 8.0.29 (2022-04-26) — release withdrawn ("Please upgrade to MySQL 8.0.30 instead"). https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-29.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 11: "Please upgrade to MySQL 8.0.30 instead")"
    - Schema Migrations and Online DDL › INSTANT row-version limit and 8.0.29 [8.0.29+] — paragraph 3: "FORCE` or `OPTIMIZE TABLE`) after upgrading, per the release-note guidance."
[^18]: Changes in MySQL 8.0.32 (2023-01-17) — explain_format, `$` identifier and unquoted FULL deprecations. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-32.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 14: "`explain_format` added"
[^19]: Changes in MySQL 8.0.34 (2023-07-18) — mysql_native_password, binlog_format, mysqlpump deprecated. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-34.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 15: "…by whitespace (`/*!80034KEY_BLOCK_SIZE=1024*/`) is deprecated (warning 4164; write `/*!80034 KEY_BLOCK_SIZE=1024*/`)"
    - Replication and Operations Awareness — bullet 3: "…is deprecated since 8.0.34; future versions support only row-based logging)."
[^20]: Changes in MySQL 8.0.35 (2023-10-25) — INFORMATION_SCHEMA.PROCESSLIST and wildcard grants deprecated, STR_TO_DATE range checks. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-35.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 16: "`STR_TO_DATE()` range-checks dates"
    - Security › Least privilege and roles [8.0+] — bullet 3: "The wildcard behaviour is deprecated since 8.0.35 without any warning."
[^21]: Changes in MySQL 8.0.41 (2025-01-21) — ADD/DROP COLUMN on empty tables uses INPLACE, spatial index incompatible change, mysql client keeps comments in hints. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-41.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 18: "…spatial-index corruption (drop and re-create spatial indexes around the upgrade)"
    - Query Patterns and Performance › Optimizer hints and index hints — bullet 3: "…`mysql` client's handling of `#` and `--` inside hint comments."
    - Schema Migrations and Online DDL › INSTANT row-version limit and 8.0.29 [8.0.29+] — paragraph 2: "…version; an explicit `ALGORITHM=INSTANT` still consumes one (verified on 8.0.46)."
[^22]: Changes in MySQL 8.0.42 (2025-04-15) — --check-table-functions, 767-byte index check for INPLACE. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-42.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 19: "INPLACE` rejects indexes over 767 bytes on `REDUNDANT`/`COMPACT` tables"
    - Indexing › Index size limits — paragraph 2: "ALGORITHM=INPLACE` that would exceed it."
[^23]: MySQL 8.4 Reference Manual — What Is New in MySQL 8.4 since MySQL 8.0 (added, deprecated, removed features; changed defaults). https://dev.mysql.com/doc/refman/8.4/en/mysql-nutshell.html
    - What's New in MySQL 8.0 (since 5.7) › Do not use or recommend (features newer than 8.0) — table row 3: "FOR SCHEMA db`, `explain_json_format_version` | 8.1–8.4"
    - What's New in MySQL 8.0 (since 5.7) › Do not use or recommend (features newer than 8.0) — table row 6: "AUTO UPDATE` | 8.4.0"
    - What's New in MySQL 8.0 (since 5.7) › Do not use or recommend (features newer than 8.0) — table row 7: "…privileges `SET_ANY_DEFINER`, `ALLOW_NONEXISTENT_DEFINER`, `FLUSH_PRIVILEGES`, `OPTIMIZE_LOCAL_TABLE`, `TRANSACTION_GTID_TAG`; tagged GTIDs | 8.2.0–8.4.0"
    - What's New in MySQL 8.0 (since 5.7) › Do not use or recommend (features newer than 8.0) — table row 8: "Hash-based `INTERSECT`/`EXCEPT` (`hash_set_operations`) | 8.1–8.4"
    - Stored Programs, Triggers, Views and Events › DEFINER and SQL SECURITY — bullet 1: "…it with `SET_ANY_DEFINER`/`ALLOW_NONEXISTENT_DEFINER`, so grant scripts differ between the two."
    - Removed / Changed in 8.4 — Migration Notes (from 8.0) — paragraph 1: "Verified differences between 8.0.46 and 8.4.13:"
[^24]: Changes in MySQL 9.0.0 (2024-07-01) — VECTOR, inline REFERENCES enforced, mysql_native_password removed. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-0-0.html
    - What's New in MySQL 8.0 (since 5.7) › Do not use or recommend (features newer than 8.0) — table row 10: "`VECTOR(n)`, `STRING_TO_VECTOR()`, `VECTOR_DIM()` | 9.0.0"
    - What's New in MySQL 8.0 (since 5.7) › Do not use or recommend (features newer than 8.0) — table row 13: "…`ROLLUP`, `enable_cascade_triggers`, hypergraph optimizer / `QUALIFY` in Community | 9.5–9.7"
    - Schema Design and Data Types › Foreign keys — bullet 1: "MySQL 9.0 started enforcing the inline form,"
[^25]: MySQL 8.0 Reference Manual — FOREIGN KEY Constraints (cascades do not activate triggers, non-unique referenced keys). https://dev.mysql.com/doc/refman/8.0/en/create-table-foreign-keys.html
    - Schema Design and Data Types › Foreign keys — bullet 3: "…child table's triggers; audit triggers on child tables miss them."
[^26]: MySQL 8.0 Reference Manual — Unicode Character Sets (PAD SPACE vs NO PAD, utf8mb4_0900 collations). https://dev.mysql.com/doc/refman/8.0/en/charset-unicode-sets.html
    - Character Sets and Collations › `utf8mb4_0900_ai_ci` semantics [8.0+] — paragraph 1: "…(`utf8mb4_general_ci`, `utf8mb4_unicode_ci`, `utf8mb4_bin`, and every 5.7 collation) are `PAD SPACE`."
[^27]: MySQL 8.0 Reference Manual — Server SQL Modes. https://dev.mysql.com/doc/refman/8.0/en/sql-mode.html
    - SQL Mode and Strictness — paragraph 1: "…server's default `sql_mode` is strict on 8.0 and on 5.7.5+:"
[^28]: MySQL 8.0 Reference Manual — MySQL Server Time Zone Support. https://dev.mysql.com/doc/refman/8.0/en/time-zone-support.html
    - Date, Time and Time Zones › Literals with offsets, `AT TIME ZONE`, named zones — paragraph 1: "…and `SET time_zone` need the time-zone tables to be loaded"
[^29]: MySQL 8.0 Reference Manual — CREATE INDEX Statement (functional key parts, multi-valued indexes). https://dev.mysql.com/doc/refman/8.0/en/create-index.html
    - Indexing › What InnoDB indexes can and cannot do — paragraph 1: "…(R-tree) indexes, plus functional (8.0.13+) and multi-valued (8.0.17+) B-tree indexes."
    - Indexing › Functional indexes and JSON [8.0.13+ / 8.0.17+] — paragraph 1: "…the plain cast is not used by `doc->>'$.name' = ?`."
[^30]: MySQL 8.0 Reference Manual — Optimizer Statistics (histograms). https://dev.mysql.com/doc/refman/8.0/en/optimizer-statistics.html
    - Indexing › Histograms [8.0+] — paragraph 1: "…HISTOGRAM` after significant data changes (automatic updates arrived in 8.4):"
[^31]: MySQL 8.0 Reference Manual — EXPLAIN Statement (EXPLAIN ANALYZE, FORMAT=TREE, explain_format). https://dev.mysql.com/doc/refman/8.0/en/explain.html
    - Reading Plans: EXPLAIN and EXPLAIN ANALYZE — bullet 1: "…(8.0.16+, iterator tree, easiest to read), `EXPLAIN FORMAT=JSON` (with costs)."
[^32]: MySQL 8.0 Reference Manual — Row Constructor Expression Optimization. https://dev.mysql.com/doc/refman/8.0/en/row-constructor-optimization.html
    - Query Patterns and Performance › Pagination: OFFSET vs keyset — and the row-constructor trap — paragraph 2: "…`OR` form, which the optimizer turns into a range scan:"
[^33]: MySQL 8.0 Reference Manual — Optimizer Hints. https://dev.mysql.com/doc/refman/8.0/en/optimizer-hints.html
    - Query Patterns and Performance › Optimizer hints and index hints — bullet 1: "`SET_VAR` 8.0+, `INDEX`/`NO_INDEX`/`JOIN_INDEX`/`ORDER_INDEX`/`GROUP_INDEX` 8.0.20+)"
[^34]: MySQL 8.0 Reference Manual — INSERT ... ON DUPLICATE KEY UPDATE Statement. https://dev.mysql.com/doc/refman/8.0/en/insert-on-duplicate.html
    - Writes: INSERT, Upserts, UPDATE, DELETE › INSERT ... ON DUPLICATE KEY UPDATE — paragraph 1: "`VALUES(col)` in the update clause is deprecated since 8.0.20."
[^35]: MySQL 8.0 Reference Manual — AUTO_INCREMENT Handling in InnoDB (lock modes, counter persistence). https://dev.mysql.com/doc/refman/8.0/en/innodb-auto-increment-handling.html
    - Writes: INSERT, Upserts, UPDATE, DELETE › INSERT ... ON DUPLICATE KEY UPDATE — bullet 3: "…8.0, and `2` requires row-based binary logging (the default `binlog_format=ROW`)."
[^36]: MySQL 8.0 Reference Manual — Consistent Nonlocking Reads. https://dev.mysql.com/doc/refman/8.0/en/innodb-consistent-read.html
    - Transactions, Isolation and Locking › Consistent reads vs current reads — paragraph 1: "SELECT` and locking reads see the latest committed data."
[^37]: MySQL 8.0 Reference Manual — Locks Set by Different SQL Statements in InnoDB. https://dev.mysql.com/doc/refman/8.0/en/innodb-locks-set.html
    - Transactions, Isolation and Locking › Locks taken by locking reads and writes — paragraph 1: "…plus the gap before it (next-key locks) under REPEATABLE READ."
[^38]: MySQL 8.0 Reference Manual — Online DDL Operations (INSTANT/INPLACE/COPY support, row versions). https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html
    - Schema Migrations and Online DDL › ALGORITHM and LOCK — paragraph 1: "With it, an unsupported combination fails immediately."
    - Schema Migrations and Online DDL › INSTANT row-version limit and 8.0.29 [8.0.29+] — paragraph 1: "MySQL 8.0 allows **64**:"
[^39]: MySQL 8.0 Reference Manual — Metadata Locking. https://dev.mysql.com/doc/refman/8.0/en/metadata-locking.html
    - Schema Migrations and Online DDL › Metadata locks: the most common migration outage — paragraph 1: "…waits, **every new query on the table queues behind it**:"
[^40]: MySQL 8.0 Reference Manual — Stored Program Binary Logging. https://dev.mysql.com/doc/refman/8.0/en/stored-programs-logging.html
    - Stored Programs, Triggers, Views and Events › Stored functions and binary logging — paragraph 1: "…a stored function requires a characteristic that declares it safe:"
[^41]: go-sql-driver/mysql — README (DSN parameters parseTime, loc, multiStatements, interpolateParams, clientFoundRows). https://github.com/go-sql-driver/mysql
    - Security › SQL injection: parameters for values, allow-lists for identifiers — paragraph 1: "Verified with Go `database/sql` and `github.com/go-sql-driver/mysql` v1.10.1 against 8.0.46:"
    - Application Code and Drivers — paragraph 1: "…v1.10.1 against 8.0.46; the same concepts apply to other drivers."
[^42]: MySQL 8.0 Reference Manual — Caching SHA-2 Pluggable Authentication. https://dev.mysql.com/doc/refman/8.0/en/caching-sha2-pluggable-authentication.html
    - Security › Authentication: caching_sha2_password vs mysql_native_password — paragraph 1: "…is **disabled by default on 8.4** and removed in 9.0."
[^43]: MySQL 8.0 Reference Manual — Using Roles (activate_all_roles_on_login, SET DEFAULT ROLE). https://dev.mysql.com/doc/refman/8.0/en/roles.html
    - Security › Least privilege and roles [8.0+] — paragraph 1: "Review rules:"
[^44]: MySQL 8.0 Reference Manual — GRANT Statement (wildcards in database names). https://dev.mysql.com/doc/refman/8.0/en/grant.html
    - Security › Least privilege and roles [8.0+] — paragraph 1: "Review rules:"
[^45]: MySQL 8.0 Reference Manual — The sys Schema. https://dev.mysql.com/doc/refman/8.0/en/sys-schema.html
    - Observability — bullet 1: "The `sys` schema (5.7.7+) views above are installed by default."
[^46]: Changes in MySQL 8.0.19 (2020-01-13) — row alias in INSERT ... ON DUPLICATE KEY UPDATE, TABLE and VALUES statements, LIMIT in recursive CTEs, datetime literals with time zone offsets, FAILED_LOGIN_ATTEMPTS, display width no longer shown. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-19.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 4: "…row alias for upserts, `TABLE`/`VALUES` statements, datetime literals with offsets"
[^47]: Changes in MySQL 8.0.26 (2021-07-20) — source/replica terminology for variables, options and instruments. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-26.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 8: "…Schema instrument and thread-stage names change incompatibly (`terminology_use_previous=BEFORE_8_0_26` restores them)"
[^48]: Changes in MySQL 8.0.27 (2021-10-19) — replica_parallel_workers=4 and replica_preserve_commit_order=ON defaults, authentication_policy, default_authentication_plugin deprecated. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-27.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 9: "…and `replica_preserve_commit_order=ON` become defaults (multithreaded replica applier on by default)"
[^49]: Changes in MySQL 8.0.18 (2019-10-14) — EXPLAIN ANALYZE, hash joins for inner equi-joins. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-18.html
    - What's New in MySQL 8.0 (since 5.7) › SQL features added in the 8.0 series [8.0+] — table row 25: "`EXPLAIN ANALYZE`, hash join (inner equi-joins)"
[^50]: InfoQ — MySQL Introduces Innovation and Long-Term Support Releases (quoting Oracle: "8.0.34+ will transition to only bug fixes until 8.0 End-Of-Life (EOL) scheduled for April-2026"); Oracle announcement: https://blogs.oracle.com/mysql/post/introducing-mysql-innovation-and-longterm-support-lts-versions . https://www.infoq.com/news/2023/08/mysql-innovation-release/
    - What's New in MySQL 8.0 (since 5.7) — paragraph 1: "…8.0 End-Of-Life" (the 8.0.34 and 8.0.35 releases still added deprecations)."
[^51]: MySQL 8.4 Reference Manual — Upgrade Paths ("a bugfix or LTS series cannot be skipped"; 5.7 → 8.0 → 8.4). https://dev.mysql.com/doc/refman/8.4/en/upgrade-paths.html
    - Overview — quote block: "…must go through 8.0 first (**5.7 → 8.0 → 8.4**)."
    - Removed / Changed in 8.4 — Migration Notes (from 8.0) — bullet 9: "…is not supported, because an LTS series cannot be skipped."
[^52]: Changes in MySQL 8.0.38 (2024-07-01; release withdrawn, upgrade to 8.0.39) and Changes in MySQL 8.0.39 (2024-07-23). https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-38.html , https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-39.html
    - What's New in MySQL 8.0 (since 5.7) › Changes inside the 8.0 series that affect review — table row 17: "…to the error log only once instead of per authentication"
