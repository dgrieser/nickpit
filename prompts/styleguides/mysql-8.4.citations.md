### Citations: MySQL 8.4 LTS: The Complete SQL Review Guide

Sources for `mysql-8.4.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: MySQL 8.4 Reference Manual — What Is New in MySQL 8.4 since MySQL 8.0 (added, deprecated, removed features; changed InnoDB defaults). https://dev.mysql.com/doc/refman/8.4/en/mysql-nutshell.html
    - Overview — quote block: "…8.4 LTS. 8.4.0 became generally available on **April 30, 2024**."
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 1: "Re-enable only with `--mysql-native-password=ON`"
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 2: "…Removed (`ERROR 1193 Unknown system variable`); use `authentication_policy` (default `*,,`)"
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 3: "`START REPLICA` no longer accepts `MASTER_LOG_FILE`/`MASTER_LOG_POS`"
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 4: "Missing unique key for constraint ...` (`restrict_fk_on_non_standard_key=ON`)"
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 5: "…`ERROR 1063 Incorrect column specifier`; such tables block the upgrade"
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 6: "…HOSTS` | Deprecated | Syntax error; use `TRUNCATE TABLE performance_schema.host_cache`"
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 7: "LOW_PRIORITY WRITE` | Deprecated | Syntax error"
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 8: "…single leading `$` (`$ab`) still works with deprecation warning 1681"
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 10: "…AEAD; startup options can no longer be set to `NULL`"
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 11: "The `ENGINE` clause of `DROP/ALTER TABLESPACE` is removed as well"
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 12: "…file that still sets them stops the server from starting"
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 13: "…server upgrades itself at startup and generates missing TLS/RSA files"
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 14: "`SET_USER_ID` removed"
    - What's New in MySQL 8.4 (since 8.0) › New SQL features in 8.1–8.4 [New in 8.4] — bullet 4: "…ON col WITH n BUCKETS AUTO UPDATE` (default `MANUAL UPDATE`)."
    - What's New in MySQL 8.4 (since 8.0) › New SQL features in 8.1–8.4 [New in 8.4] — bullet 5: "`INTERSECT` and `EXCEPT` themselves exist since 8.0.31."
    - What's New in MySQL 8.4 (since 8.0) › New SQL features in 8.1–8.4 [New in 8.4] — bullet 6: "…can be transformed into derived tables (`subquery_to_derived`, off by default)."
    - What's New in MySQL 8.4 (since 8.0) › New SQL features in 8.1–8.4 [New in 8.4] — bullet 8: "…(`SET gtid_next = 'AUTOMATIC:tag'`), `SHOW BINARY LOG STATUS`, `SHOW REPLICAS`."
    - What's New in MySQL 8.4 (since 8.0) › Changed server defaults [New in 8.4] — paragraph 1: "…New" page (several computed defaults coincide on a small container):"
    - What's New in MySQL 8.4 (since 8.0) › Deprecated in 8.4 (works, but flag it) — bullet 8: "…8.0.35, both released 2023-10-25; no warning is printed on 8.4.13)."
    - What's New in MySQL 8.4 (since 8.0) › Deprecated in 8.4 (works, but flag it) — bullet 10: "…requires `ROW`, and `MIXED` is no longer supported for it."
    - What's New in MySQL 8.4 (since 8.0) › Deprecated in 8.4 (works, but flag it) — bullet 13: "DISABLE ON SLAVE` (warning 1287, use `DISABLE ON REPLICA`)."
    - Indexing › Histograms — paragraph 1: "…estimates without index maintenance cost. 8.4 can refresh it automatically:"
    - Query Patterns and Performance › Miscellaneous performance rules — bullet 6: "…spill to InnoDB on-disk temporary tables sooner than on 8.0."
    - Stored Programs, Triggers, Views and Events › DEFINER and SQL SECURITY — bullet 1: "…`SET_ANY_DEFINER` (new in 8.2, replacing `SET_USER_ID`); orphaned definers need `ALLOW_NONEXISTENT_DEFINER`."
    - Stored Programs, Triggers, Views and Events › Events — paragraph 1: "…ON SLAVE` is deprecated in favour of `DISABLE ON REPLICA`."
    - Security › Least privilege and roles — bullet 3: "…on the 8.0 line; no warning is printed on 8.4.13)."
    - Replication and Operations Awareness — bullet 1: "The `MASTER`/`SLAVE` statements and options are syntax errors on 8.4."
    - Replication and Operations Awareness — bullet 3: "…supported for that, and the `binlog_format` variable itself is deprecated)."
[^2]: MySQL 8.4 Release Notes (index of 8.4.0–8.4.13 with dates). https://dev.mysql.com/doc/relnotes/mysql/8.4/en/
    - Overview — quote block: "…8.4 LTS. 8.4.0 became generally available on **April 30, 2024**."
    - Overview — quote block: "…a Critical Security Patch Update for the Docker image only."
[^3]: Changes in MySQL 8.4.13 (2026-10-20) — VERSION_WITH_UPDATE(), EXCHANGE PARTITION CHECK validation, index_merge fix, security fixes; Changes in MySQL 8.4.12 (2026-08-18, Docker image only). https://dev.mysql.com/doc/relnotes/mysql/8.4/en/news-8-4-13.html , https://dev.mysql.com/doc/relnotes/mysql/8.4/en/news-8-4-12.html
    - Overview — quote block: "…a Critical Security Patch Update for the Docker image only."
    - What's New in MySQL 8.4 (since 8.0) › New SQL features in 8.1–8.4 [New in 8.4] — bullet 9: "`VERSION()` keeps returning three parts."
    - What's New in MySQL 8.4 (since 8.0) › Behaviour changes inside the 8.4 LTS series — table row 6: "…plans that could drop a disjunct and return incomplete results"
    - What's New in MySQL 8.4 (since 8.0) › Behaviour changes inside the 8.4 LTS series — paragraph 2: "…incomplete results and hardens protocol parsing against several security issues."
    - Schema Design and Data Types › Constraints: NOT NULL, CHECK, defaults — paragraph 3: "…rows against the destination's `CHECK` constraints even with `WITHOUT VALIDATION`."
    - Indexing › Things that prevent index use — paragraph 6: "…drop a disjunct and return incomplete results with that setting."
[^4]: MySQL 8.4 Reference Manual — MySQL Releases: Innovation and LTS (support policy, upgrade paths). https://dev.mysql.com/doc/refman/8.4/en/mysql-releases.html
    - Overview — quote block: "…years of Premier Support and three years of Extended Support,"
    - Overview — quote block: "…LTS → 9.7 LTS; an LTS series cannot be skipped."
    - Overview — paragraph 1: "…Innovation release is supported only until the next one ships)."
    - What's New in MySQL 8.4 (since 8.0) — paragraph 1: "…fixes, and in-place upgrade and downgrade within 8.4.x is supported."
[^5]: endoflife.date — MySQL (Premier/Extended support end dates per series). https://endoflife.date/mysql
    - Overview — quote block: "…Support until **April 2029** and Extended Support until **April 2032**."
[^6]: MySQL 8.0 Release Notes (8.0.46, 2026-04-21; End of Life statement). https://dev.mysql.com/doc/relnotes/mysql/8.0/en/
    - Overview — quote block: "…of Life in April 2026** with 8.0.46 (April 21, 2026)."
[^7]: MySQL 9.7 Release Notes (index of 9.0.0–9.7.4 with dates). https://dev.mysql.com/doc/relnotes/mysql/9.7/en/
    - Overview — quote block: "…shipped on April 21, 2026 (latest 9.7.4, October 20, 2026)."
[^8]: MySQL 26.10 Release Notes (26.7.0 2026-07-28, 26.7.1 2026-08-18, 26.10.0 2026-10-20). https://dev.mysql.com/doc/relnotes/mysql/26.10/en/
    - Overview — quote block: "…and **26.10.0** (October 20, 2026) are the newest Innovation releases."
[^9]: MySQL 8.4 Reference Manual — Keywords and Reserved Words (new in 8.4; MANUAL/PARALLEL non-reserved since 8.4.11). https://dev.mysql.com/doc/refman/8.4/en/keywords.html
    - What's New in MySQL 8.4 (since 8.0) › Breaking changes for application SQL and migrations [New in 8.4] — table row 9: "…new non-reserved keywords (`MANUAL` and `PARALLEL` were reserved in 8.4.0–8.4.10)"
[^10]: MySQL 8.4 Reference Manual — EXPLAIN Statement (EXPLAIN ANALYZE, FORMAT=JSON INTO, FOR SCHEMA, explain_format). https://dev.mysql.com/doc/refman/8.4/en/explain.html
    - What's New in MySQL 8.4 (since 8.0) › New SQL features in 8.1–8.4 [New in 8.4] — bullet 1: "`INTO` requires an explicit `FORMAT=JSON`."
    - What's New in MySQL 8.4 (since 8.0) › New SQL features in 8.1–8.4 [New in 8.4] — bullet 2: "…explains a statement as if it ran in another schema."
    - What's New in MySQL 8.4 (since 8.0) › New SQL features in 8.1–8.4 [New in 8.4] — bullet 3: "…so verify on the pinned patch before relying on it."
    - Reading Plans: EXPLAIN and EXPLAIN ANALYZE — bullet 1: "`explain_json_format_version=2` gives the access-path layout)."
[^11]: Changes in MySQL 8.4.3 (2024-10-15) — GROUP BY ROLLUP (...) alternative syntax (WL #15843), mysql --system-command. https://dev.mysql.com/doc/relnotes/mysql/8.4/en/news-8-4-3.html
    - What's New in MySQL 8.4 (since 8.0) › New SQL features in 8.1–8.4 [New in 8.4] — bullet 7: "Treat it as available on every 8.4.x."
    - What's New in MySQL 8.4 (since 8.0) › Behaviour changes inside the 8.4 LTS series — table row 1: "`mysql` client gains `--system-command` (on by default)"
[^12]: Changes in MySQL 8.4.4 (2025-01-21) — ADD/DROP COLUMN on empty tables uses INPLACE; spatial index incompatible change. https://dev.mysql.com/doc/relnotes/mysql/8.4/en/news-8-4-4.html
    - What's New in MySQL 8.4 (since 8.0) › Behaviour changes inside the 8.4 LTS series — table row 2: "…corruption — drop and re-create spatial indexes around the upgrade"
    - Schema Migrations and Online DDL › INSTANT row-version limit — paragraph 2: "…version; an explicit `ALGORITHM=INSTANT` still consumes one (verified on 8.4.13)."
[^13]: Changes in MySQL 8.4.5 (2025-04-15) — --check-table-functions. https://dev.mysql.com/doc/relnotes/mysql/8.4/en/news-8-4-5.html
    - What's New in MySQL 8.4 (since 8.0) › Behaviour changes inside the 8.4 LTS series — table row 3: "…generated column uses a function that now raises an error"
[^14]: Changes in MySQL 8.4.6 (2025-07-22) — mysql client --commands. https://dev.mysql.com/doc/relnotes/mysql/8.4/en/news-8-4-6.html
    - What's New in MySQL 8.4 (since 8.0) › Behaviour changes inside the 8.4 LTS series — table row 4: "8.4.6 | `mysql` client `--commands` option (on by default)"
[^15]: Changes in MySQL 8.4.11 (2026-07-28) — keyword reservation fixes (Bug #114874), InnoDB row-size validation. https://dev.mysql.com/doc/relnotes/mysql/8.4/en/news-8-4-11.html
    - What's New in MySQL 8.4 (since 8.0) › Behaviour changes inside the 8.4 LTS series — table row 5: "…row-size limits are now rejected (`ALGORITHM=INSTANT` uses the stricter calculation)"
[^16]: Changes in MySQL 9.0.0 (2024-07-01) — VECTOR, inline REFERENCES enforced, mysql_native_password removed, EXPLAIN ANALYZE INTO, prepared event statements. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-0-0.html
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 1: "`VECTOR(n)` type, `STRING_TO_VECTOR()`/`TO_VECTOR()`, `VECTOR_TO_STRING()`/`FROM_VECTOR()`, `VECTOR_DIM()` | 9.0.0"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 2: "`EXPLAIN ANALYZE FORMAT=JSON INTO @var` | 9.0.0"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 3: "`PREPARE` of `CREATE/ALTER/DROP EVENT` | 9.0.0"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 4: "…JAVASCRIPT`), `CREATE LIBRARY`, `USING (lib)` | 9.0.0 / 9.2.0 (Enterprise)"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — bullet 1: "`default_authentication_plugin` was already removed in 8.4."
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — bullet 3: "…on 8.4 may start failing with `ERROR 1452` on 9.x."
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — bullet 4: "`IGNORE` no longer suppresses `ER_SUBQUERY_NO_1_ROW`."
    - Schema Design and Data Types › Foreign keys — bullet 1: "MySQL 9.0 started enforcing the inline form,"
[^17]: Changes in MySQL 9.2.0 (2025-01-21) — CREATE LIBRARY, CREATE_SPATIAL_REFERENCE_SYSTEM, FLUSH PRIVILEGES deprecated, behaviour changes. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-2-0.html
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 4: "…JAVASCRIPT`), `CREATE LIBRARY`, `USING (lib)` | 9.0.0 / 9.2.0 (Enterprise)"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 6: "`CREATE_SPATIAL_REFERENCE_SYSTEM` privilege | 9.2.0"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — bullet 6: "…collation; outer references in window `PARTITION BY`/`ORDER BY` are rejected."
[^18]: Changes in MySQL 9.1.0 (2024-10-15) — CREATE VIEW IF NOT EXISTS, row versions 255, derived-table references, ROLLUP/SUM(DISTINCT) fixes. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-1-0.html
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 5: "`CREATE VIEW IF NOT EXISTS` | 9.1.0"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — bullet 5: "`SUM(DISTINCT b'1100')` returns 12 (8.4.13 returns 9)."
    - Schema Migrations and Online DDL › INSTANT row-version limit — paragraph 2: "(9.1.0 raised the limit to 255; 8.4 stays at 64."
[^19]: Changes in MySQL 9.4.0 (2025-07-22) — JSON duality views, MD5()/SHA1() deprecated. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-4-0.html
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 7: "…DML on duality views | 9.4.0 (DML in Community: 9.7.0)"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — bullet 8: "`ERROR 1305 FUNCTION MD5 does not exist`)."
[^20]: Changes in MySQL 9.7.0 (2026-04-21) — hypergraph optimizer in Community, enable_cascade_triggers, duality-view DML, PBKDF2, sql_require_primary_key change. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-7-0.html
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 7: "…DML on duality views | 9.4.0 (DML in Community: 9.7.0)"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 12: "Hypergraph optimizer in Community (`optimizer_switch='hypergraph_optimizer=on'`) and therefore `QUALIFY` | 9.7.0"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 13: "`enable_cascade_triggers` (FK cascades fire triggers) | 9.7.0"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 14: "`CURRENT_ROLE_IN()`, `CURRENT_USER_IN()`, `CREATE MASKING POLICY` | 9.7.0 (Enterprise)"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 15: "`caching_sha2_password` PBKDF2 storage format | 9.7.0"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 16: "`replica_allow_higher_version_source` | 9.7.0"
    - Schema Design and Data Types › Primary keys — paragraph 4: "…`UNIQUE NOT NULL` key as a substitute (9.7.0 relaxes this)."
    - Schema Design and Data Types › Foreign keys — bullet 3: "…on child tables miss them (9.7.0 adds an opt-in `enable_cascade_triggers`)."
[^21]: Changes in MySQL 9.5.0 (2025-10-21) — activate_mandatory_roles, INTERPRET_UTF8_AS_UTF8MB4, changed defaults. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-5-0.html
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 8: "`activate_mandatory_roles` variable | 9.5.0"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 9: "`INTERPRET_UTF8_AS_UTF8MB4` SQL mode | 9.5.0"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — bullet 9: "`explain_format=TREE`, `explain_json_format_version=2`, `gtid_mode=ON`, `innodb_change_buffering=all`, `caching_sha2_password_digest_rounds=10000`."
[^22]: Changes in MySQL 9.6.0 (2026-01-20) — SQL-layer foreign keys, MD5/SHA1 moved to classic_hashing component, GROUPING() without ROLLUP. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-6-0.html
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 10: "`GROUPING()` in a query without `ROLLUP` | 9.6.0"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — table row 11: "`innodb_native_foreign_keys`, SQL-layer foreign keys | 9.6.0"
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — bullet 8: "`ERROR 1305 FUNCTION MD5 does not exist`)."
[^23]: Changes in MySQL 9.3.0 (2025-04-15) — removal of innodb_log_file_size, innodb_log_files_in_group, innodb_undo_tablespaces. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-3-0.html
    - What's New in MySQL 8.4 (since 8.0) › Do not use or recommend (features newer than 8.4) — bullet 7: "…removes `innodb_log_file_size`, `innodb_log_files_in_group` and `innodb_undo_tablespaces` (still accepted, deprecated, on 8.4)."
[^24]: MySQL 8.4 Reference Manual — FOREIGN KEY Constraints (cascades do not activate triggers, non-standard keys). https://dev.mysql.com/doc/refman/8.4/en/create-table-foreign-keys.html
    - Schema Design and Data Types › Foreign keys — bullet 3: "…on child tables miss them (9.7.0 adds an opt-in `enable_cascade_triggers`)."
[^25]: MySQL 8.4 Reference Manual — Unicode Character Sets / Trailing Space Handling in Comparisons (PAD SPACE vs NO PAD). https://dev.mysql.com/doc/refman/8.4/en/charset-unicode-sets.html
    - Character Sets and Collations › `utf8mb4_0900_ai_ci` semantics — paragraph 1: "Older collations (`utf8mb4_general_ci`, `utf8mb4_unicode_ci`, `utf8mb4_bin`) are `PAD SPACE`."
[^26]: MySQL 8.4 Reference Manual — Server SQL Modes. https://dev.mysql.com/doc/refman/8.4/en/sql-mode.html
    - SQL Mode and Strictness — paragraph 1: "The server's default `sql_mode` is strict:"
[^27]: MySQL 8.4 Reference Manual — MySQL Server Time Zone Support. https://dev.mysql.com/doc/refman/8.4/en/time-zone-support.html
    - Date, Time and Time Zones › Literals with offsets, `AT TIME ZONE`, named zones — paragraph 1: "…and `SET time_zone` need the time-zone tables to be loaded"
[^28]: MySQL 8.4 Reference Manual — Row Constructor Expression Optimization. https://dev.mysql.com/doc/refman/8.4/en/row-constructor-optimization.html
    - Query Patterns and Performance › Pagination: OFFSET vs keyset — and the row-constructor trap — paragraph 2: "…`OR` form, which the optimizer turns into a range scan:"
[^29]: MySQL 8.4 Reference Manual — CREATE INDEX Statement (functional key parts, multi-valued indexes). https://dev.mysql.com/doc/refman/8.4/en/create-index.html
    - Indexing › What InnoDB indexes can and cannot do — paragraph 1: "…(R-tree) indexes, plus functional (8.0.13+) and multi-valued (8.0.17+) B-tree indexes."
    - Indexing › Functional indexes and JSON — paragraph 1: "…the plain cast is not used by `doc->>'$.name' = ?`."
[^30]: MySQL 8.4 Reference Manual — Online DDL Operations (INSTANT/INPLACE/COPY support, row versions). https://dev.mysql.com/doc/refman/8.4/en/innodb-online-ddl-operations.html
    - Schema Migrations and Online DDL › ALGORITHM and LOCK — paragraph 1: "With it, an unsupported combination fails immediately."
    - Schema Migrations and Online DDL › INSTANT row-version limit — paragraph 1: "MySQL 8.4 allows **64**:"
[^31]: MySQL 8.4 Reference Manual — Metadata Locking. https://dev.mysql.com/doc/refman/8.4/en/metadata-locking.html
    - Schema Migrations and Online DDL › Metadata locks: the most common migration outage — paragraph 1: "…waits, **every new query on the table queues behind it**:"
[^32]: MySQL 8.4 Reference Manual — Locks Set by Different SQL Statements in InnoDB. https://dev.mysql.com/doc/refman/8.4/en/innodb-locks-set.html
    - Transactions, Isolation and Locking › Locks taken by locking reads and writes — paragraph 1: "…plus the gap before it (next-key locks) under REPEATABLE READ."
[^33]: MySQL 8.4 Reference Manual — Consistent Nonlocking Reads. https://dev.mysql.com/doc/refman/8.4/en/innodb-consistent-read.html
    - Transactions, Isolation and Locking › Consistent reads vs current reads — paragraph 1: "SELECT` and locking reads see the latest committed data."
[^34]: MySQL 8.4 Reference Manual — INSERT ... ON DUPLICATE KEY UPDATE Statement. https://dev.mysql.com/doc/refman/8.4/en/insert-on-duplicate.html
    - Writes: INSERT, Upserts, UPDATE, DELETE › INSERT ... ON DUPLICATE KEY UPDATE — paragraph 1: "`VALUES(col)` in the update clause is deprecated:"
[^35]: MySQL 8.4 Reference Manual — Caching SHA-2 Pluggable Authentication. https://dev.mysql.com/doc/refman/8.4/en/caching-sha2-pluggable-authentication.html
    - Security › Authentication: caching_sha2_password [New in 8.4] — paragraph 1: "…9.0, so every account and every client must handle `caching_sha2_password`:"
[^36]: MySQL 8.4 Reference Manual — Using Roles (activate_all_roles_on_login, SET DEFAULT ROLE). https://dev.mysql.com/doc/refman/8.4/en/roles.html
    - Security › Least privilege and roles — paragraph 1: "Review rules:"
[^37]: MySQL 8.4 Reference Manual — GRANT Statement (wildcards in database names). https://dev.mysql.com/doc/refman/8.4/en/grant.html
    - Security › Least privilege and roles — paragraph 1: "Review rules:"
[^38]: MySQL 8.4 Reference Manual — Stored Program Binary Logging. https://dev.mysql.com/doc/refman/8.4/en/stored-programs-logging.html
    - Stored Programs, Triggers, Views and Events › Stored functions and binary logging — paragraph 1: "…a stored function requires a characteristic that declares it safe:"
[^39]: MySQL 8.4 Reference Manual — Optimizer Hints. https://dev.mysql.com/doc/refman/8.4/en/optimizer-hints.html
    - Query Patterns and Performance › Optimizer hints and index hints — bullet 1: "*/`, 8.0+)"
[^40]: MySQL 8.4 Reference Manual — Optimizer Statistics (histograms, AUTO UPDATE). https://dev.mysql.com/doc/refman/8.4/en/optimizer-statistics.html
    - Indexing › Histograms — paragraph 1: "…estimates without index maintenance cost. 8.4 can refresh it automatically:"
[^41]: MySQL 8.4 Reference Manual — The sys Schema. https://dev.mysql.com/doc/refman/8.4/en/sys-schema.html
    - Observability — bullet 1: "The `sys` schema views above are installed by default."
[^42]: go-sql-driver/mysql — README (DSN parameters parseTime, loc, multiStatements, interpolateParams, clientFoundRows). https://github.com/go-sql-driver/mysql
    - Security › SQL injection: parameters for values, allow-lists for identifiers — paragraph 1: "Verified with Go `database/sql` and `github.com/go-sql-driver/mysql` v1.10.1 against 8.4.13:"
    - Application Code and Drivers — paragraph 1: "…with `github.com/go-sql-driver/mysql` v1.10.1; the same concepts apply to other drivers."
