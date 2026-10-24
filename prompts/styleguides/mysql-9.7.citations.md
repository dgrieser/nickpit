### Citations: MySQL 9.7 LTS: The Complete SQL Review Guide

Footnote sources for `mysql-9.7.md`. Not sent to the model; kept for maintainers.

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
