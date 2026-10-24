### Citations: MySQL 9.7 LTS: The Complete SQL Review Guide

Sources for `mysql-9.7.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Changes in MySQL 9.7.0 (2026-04-21) — hypergraph optimizer in Community, enable_cascade_triggers, duality-view DML in Community, PBKDF2 storage format, sql_require_primary_key/GIPK with UNIQUE NOT NULL, temporal fixes (WL #16895), cpuset awareness, replica_allow_higher_version_source, components moved to Community. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-7-0.html
    - Overview — quote block: "…9.7 LTS. 9.7.0 became generally available on **April 21, 2026**."
    - What's New in MySQL 9.7 (since 8.4) — paragraph 1: "…8.4 is spread over the 9.0.0–9.6.0 release notes, summarized here."
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 13: "DML on duality views | 9.4.0 (DML in Community: 9.7.0)"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 18: "**Hypergraph optimizer** (`optimizer_switch='hypergraph_optimizer=on'`) and therefore **`QUALIFY`** | 9.7.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 19: "`enable_cascade_triggers` (FK `CASCADE`/`SET NULL` fire child triggers) | 9.7.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 20: "…a `UNIQUE NOT NULL` key as primary-key equivalent | 9.7.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 21: "`caching_sha2_password_storage_format` (`CRYPT5` default, `PBKDF2` with SHA-512), `caching_sha2_password_enforce_storage_format` | 9.7.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 22: "`replica_allow_higher_version_source` | 9.7.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 23: "…resource-manager / flow-control / primary-election components (previously Enterprise) | 9.7.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 26: "`CURRENT_ROLE_IN()`, `CURRENT_USER_IN()` | 9.7.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 13: "…as --container_aware is OFF`) | 9.3.0 / 9.6.0 / 9.7.0"
    - What's New in MySQL 9.7 (since 8.4) › Behaviour changes inside the 9.7 LTS series — table row 1: "…`CREATE`/`ALTER TABLE` statements that exceed documented limits are rejected (`ER_TOO_BIG_ROWSIZE`)"
    - Schema Design and Data Types › Primary keys — paragraph 3: "…`UNIQUE` key over `NOT NULL` columns as a primary-key equivalent:"
    - Schema Design and Data Types › Foreign keys [New in 9.7] — paragraph 1: "…SQL layer (9.6.0), and cascades can optionally fire triggers (9.7.0)."
    - Schema Design and Data Types › Foreign keys [New in 9.7] — bullet 4: "…but it is already deprecated: every `SET` raises warning 1681."
    - Date, Time and Time Zones › Temporal function fixes in 9.7.0 [New in 9.7] — paragraph 1: "…depended on the old results change behaviour after the upgrade:"
    - Query Patterns and Performance › Top-N per group, window functions, LATERAL, QUALIFY — paragraph 1: "…**hypergraph optimizer** is enabled, which Community Server allows since 9.7.0:"
    - Stored Programs, Triggers, Views and Events › JSON duality views [New in 9.7] — paragraph 1: "…documents; since 9.7.0 Community Server also accepts `INSERT`/`UPDATE`/`DELETE` through them."
    - Replication and Operations Awareness [New in 9.7] — bullet 4: "…accepts a source with a higher version (for rolling upgrades)."
    - Replication and Operations Awareness [New in 9.7] — bullet 10: "…sizes itself for the whole host and can be OOM-killed."
    - Observability — bullet 5: "…/ `'file://component_replication_applier_metrics'`; tested: the tables do not exist until then)."
[^2]: MySQL 9.7 Release Notes (index of 9.0.0–9.7.4 with dates; 9.7.1 and 9.7.3 Critical Security Patch Updates; 9.7.4 four-part version). https://dev.mysql.com/doc/relnotes/mysql/9.7/en/
    - Overview — quote block: "…9.7 LTS. 9.7.0 became generally available on **April 21, 2026**."
    - Overview — quote block: "…Security Patch Update for the Docker image only), 9.7.4 (2026-10-20)."
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 24: "`9.7.4.0`) | **9.7.4**"
    - What's New in MySQL 9.7 (since 8.4) › Behaviour changes inside the 9.7 LTS series — table row 2: "9.7.1 | Critical Security Patch Update only"
    - What's New in MySQL 9.7 (since 8.4) › Behaviour changes inside the 9.7 LTS series — table row 4: "…Critical Security Patch Update for the Docker image only"
[^3]: MySQL 9.7 Reference Manual — MySQL Releases: Innovation and LTS (calendar versioning after 9.7, LTS support policy, upgrade paths). https://dev.mysql.com/doc/refman/9.7/en/mysql-releases.html
    - Overview — quote block: "…years of Premier Support and three years of Extended Support,"
    - Overview — quote block: "…2026) are newer than 9.7 but are *not* 9.7 patches."
    - Overview — quote block: "…LTS → 9.7 LTS; an LTS series cannot be skipped."
    - Overview — paragraph 1: "…Innovation release is supported only until the next one ships);"
    - What's New in MySQL 9.7 (since 8.4) — paragraph 1: "…fixes, and in-place upgrade and downgrade within 9.7.x is supported."
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 24: "…series; within 9.7.x LTS in-place downgrade is supported | 9.3.0"
    - Replication and Operations Awareness [New in 9.7] — bullet 5: "Innovation releases (26.x) cannot be downgraded in place."
    - Removed / Changed in 9.7 — Migration Notes (from 8.4) — bullet 13: "…LTS series); keep a logical backup from before the upgrade."
[^4]: endoflife.date — MySQL (9.7: Premier Support until 2031-04-30, Extended Support until 2034-04-30). https://endoflife.date/mysql
    - Overview — quote block: "…Support until **April 2031** and Extended Support until **April 2034**."
[^5]: MySQL 26.10 Release Notes (26.7.0 2026-07-28, 26.7.1 2026-08-18, 26.10.0 2026-10-20). https://dev.mysql.com/doc/relnotes/mysql/26.10/en/
    - Overview — quote block: "…2026) are newer than 9.7 but are *not* 9.7 patches."
    - What's New in MySQL 9.7 (since 8.4) › Do not use or recommend (features newer than 9.7) — paragraph 1: "…on 9.7, and flag them as bugs when they appear:"
[^6]: MySQL 8.4 Release Notes (8.4.13, 2026-10-20) and MySQL 8.0 Release Notes (8.0.46, 2026-04-21). https://dev.mysql.com/doc/relnotes/mysql/8.4/en/ , https://dev.mysql.com/doc/relnotes/mysql/8.0/en/
    - Overview — quote block: "…Life in April 2026 (last release 8.0.46, April 21, 2026)."
[^7]: MySQL 9.7 Reference Manual — What Is New in MySQL 9.7; Server and Status Variables and Options Added, Deprecated, or Removed in MySQL 9.7. https://dev.mysql.com/doc/refman/9.7/en/mysql-nutshell.html , https://dev.mysql.com/doc/refman/9.7/en/added-deprecated-removed.html
    - What's New in MySQL 9.7 (since 8.4) — paragraph 1: "…8.4 is spread over the 9.0.0–9.6.0 release notes, summarized here."
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 8: "`authentication_ldap_sasl_auth_method_name` now defaults to `SCRAM-SHA-256`), `group_replication_communication_stack`, `group_replication_ip_allowlist` (9.7.2)."
[^8]: Changes in MySQL 9.0.0 (2024-07-01) — VECTOR, inline REFERENCES enforced, mysql_native_password removed, EXPLAIN ANALYZE INTO, prepared event statements, IGNORE and ER_SUBQUERY_NO_1_ROW, mixed-engine transactions deprecated, JavaScript programs. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-0-0.html
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 1: "…a column list references the parent's primary key | 9.0.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 3: "…accounts cannot log in; the client-side plugin remains | 9.0.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 6: "…a warning | **`ERROR 1242`** even with `IGNORE` | 9.0.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 7: "…GTID consistency`; also deprecated (warning 6414) | 9.0.0 / 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 18: "Use `innodb_redo_log_capacity` for redo sizing | 9.0.0–9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 1: "`STRING_TO_VECTOR()`/`TO_VECTOR()`, `VECTOR_TO_STRING()`/`FROM_VECTOR()`, `VECTOR_DIM()` | 9.0.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 2: "`REFERENCES parent` without a column list | 9.0.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 3: "…FORMAT=JSON INTO @var` (requires `explain_json_format_version=2`, now the default) | 9.0.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 4: "…EVENT`, `ALTER EVENT`, `DROP EVENT` (no `?` placeholders) | 9.0.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 5: "…`LIMIT 1` can be transformed into derived tables | 9.0.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 25: "…WebAssembly libraries (`LANGUAGE WASM`) | 9.0.0 / 9.2.0 / 9.4.0"
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 4: "…transactional (InnoDB) and non-transactional (MyISAM) tables (warning 6414, since 9.0.0)."
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 8: "`authentication_ldap_sasl_auth_method_name` now defaults to `SCRAM-SHA-256`), `group_replication_communication_stack`, `group_replication_ip_allowlist` (9.7.2)."
    - Schema Design and Data Types › VECTOR [New in 9.7] — paragraph 1: "…index, no distance function and no comparison other than equality."
    - Schema Design and Data Types › Foreign keys [New in 9.7] — paragraph 1: "…SQL layer (9.6.0), and cascades can optionally fire triggers (9.7.0)."
    - SQL Mode and Strictness — bullet 3: "…suppresses "Subquery returns more than 1 row"; the statement fails:"
    - Reading Plans: EXPLAIN and EXPLAIN ANALYZE [New in 9.7] — bullet 4: "…with `FORMAT=JSON` (`ERROR 6007 EXPLAIN INTO does not support FORMAT=TREE`)."
    - Stored Programs, Triggers, Views and Events › JavaScript stored programs and libraries [New in 9.7, Enterprise] — paragraph 1: "…component, which ships only with **MySQL Enterprise Edition** (and HeatWave)."
    - Security › Authentication: caching_sha2_password only [New in 9.7] — paragraph 1: "…an external/plugin method), and every client library must support it:"
[^9]: Changes in MySQL 9.4.0 (2025-07-22) — JSON duality views, MD5()/SHA1() deprecated, mysql --commands, temptable_use_mmap removed, innodb_change_buffer_max_size, collation derivation change, RANGE frame with nondeterministic ORDER BY rejected, WASM libraries. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-4-0.html
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 4: "…not exist` until `INSTALL COMPONENT 'file://component_classic_hashing'` | 9.4.0 / 9.6.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 9: "…syntax errors; only `delimiter` and `\C` still work | 9.4.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 12: "…RANGE ...`) | Accepted | Rejected (`ERROR 3587`) | 9.4.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 18: "Use `innodb_redo_log_capacity` for redo sizing | 9.0.0–9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 13: "DML on duality views | 9.4.0 (DML in Community: 9.7.0)"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 25: "…WebAssembly libraries (`LANGUAGE WASM`) | 9.0.0 / 9.2.0 / 9.4.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 5: "`innodb_change_buffer_max_size` | 25 | 5 | 9.4.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 11: "`back_log` | 151 | 10000 | 9.4.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 12: "…(8.4.13 client) | `OFF` / `OFF` | 9.4.0 / 9.1.0"
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 3: "…`SHA1()` (deprecated in 9.4.0; only available through the `classic_hashing` component)."
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 6: "`--early-plugin-load`."
    - Character Sets and Collations › `utf8mb4_0900_ai_ci` semantics — bullet 4: "`COERCIBILITY()` values shifted accordingly."
    - Indexing › Descending, invisible and redundant indexes — bullet 4: "…buffer pool), which softens but does not remove that cost."
    - Query Patterns and Performance › Top-N per group, window functions, LATERAL, QUALIFY — bullet 3: "…`RANGE` frames reject nondeterministic `ORDER BY` expressions (`ERROR 3587`, 9.4.0)."
    - Query Patterns and Performance › Miscellaneous performance rules — bullet 6: "`temptable_use_mmap` was removed in 9.4.0)."
    - Schema Migrations and Online DDL › Migration scripts and the `mysql` client [New in 9.7] — paragraph 1: "Since 9.4.0 the `mysql` client starts with `--commands=OFF`."
    - Stored Programs, Triggers, Views and Events › JavaScript stored programs and libraries [New in 9.7, Enterprise] — paragraph 1: "…component, which ships only with **MySQL Enterprise Edition** (and HeatWave)."
    - Stored Programs, Triggers, Views and Events › JSON duality views [New in 9.7] — paragraph 1: "…documents; since 9.7.0 Community Server also accepts `INSERT`/`UPDATE`/`DELETE` through them."
[^10]: Changes in MySQL 9.6.0 (2026-01-20) — SQL-layer foreign keys (innodb_native_foreign_keys), MD5/SHA1 moved to the classic_hashing component, GROUPING() without ROLLUP, temporary account locks table, container_aware startup option. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-6-0.html
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 4: "…not exist` until `INSTALL COMPONENT 'file://component_classic_hashing'` | 9.4.0 / 9.6.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 17: "…use of group function` | Allowed (returns 0) | 9.6.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 16: "`GROUPING()` without `ROLLUP` | 9.6.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 17: "…handling only with the read-only startup option `innodb_native_foreign_keys=ON`) | 9.6.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 13: "…as --container_aware is OFF`) | 9.3.0 / 9.6.0 / 9.7.0"
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 3: "…`SHA1()` (deprecated in 9.4.0; only available through the `classic_hashing` component)."
    - Schema Design and Data Types › Foreign keys [New in 9.7] — paragraph 1: "…SQL layer (9.6.0), and cascades can optionally fire triggers (9.7.0)."
    - Schema Design and Data Types › Foreign keys [New in 9.7] — bullet 5: "`SET GLOBAL innodb_native_foreign_keys = ON` fails with `ERROR 1238`."
    - Security › Authentication: caching_sha2_password only [New in 9.7] — bullet 5: "…n PASSWORD_LOCK_TIME d`; 9.6.0 adds `performance_schema.temporary_account_locks` to see locked accounts)."
    - Replication and Operations Awareness [New in 9.7] — bullet 8: "…on 8.4, and may need to stop emulating cascades themselves."
    - Replication and Operations Awareness [New in 9.7] — bullet 10: "…sizes itself for the whole host and can be OOM-killed."
[^11]: MySQL 9.7 Reference Manual — Keywords and Reserved Words (new keywords compared to 8.4: EXTERNAL (R), LIBRARY (R), DUALITY, MASKING, POLICY, RELATIONAL, VECTOR, ...). https://dev.mysql.com/doc/refman/9.7/en/keywords.html
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 5: "`CUBE`, `QUALIFY`, `TABLESAMPLE` are reserved in both) | 9.x"
[^12]: Changes in MySQL 9.5.0 (2025-10-21) — explain_format/explain_json_format_version, gtid_mode, innodb_change_buffering, caching_sha2_password_digest_rounds, activate_mandatory_roles, INTERPRET_UTF8_AS_UTF8MB4, SOURCE_SSL, binary-protocol temporal validation, removed variables and plugins. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-5-0.html
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 7: "…GTID consistency`; also deprecated (warning 6414) | 9.0.0 / 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 8: "…plans have a different schema (`$.query_plan...`, not `$.query_block...`) | 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 18: "Use `innodb_redo_log_capacity` for redo sizing | 9.0.0–9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 19: "…Version Tokens | Removed (use `semisync_source`/`semisync_replica`) | 9.3.0 / 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 14: "`activate_mandatory_roles` (default `ON`) | 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 15: "`INTERPRET_UTF8_AS_UTF8MB4` SQL mode | 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 1: "`explain_format` | `TRADITIONAL` | `TREE` | 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 2: "`explain_json_format_version` | 1 | 2 | 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 3: "…`OFF` / `OFF` | `ON` / `ON` | 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 4: "`innodb_change_buffering` | `none` | `all` | 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 6: "`caching_sha2_password_digest_rounds` | 5000 | 10000 | 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 7: "…`ON` (mandatory roles become active at login) | 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 8: "…1,000,000; the manual and the server say 10,000,000) | 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 9: "…/ `OFF` | 1 / `REQUIRED` / `ON` | 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 10: "…logging on (more than 4 without) | 9.1.0 / 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 6: "`--early-plugin-load`."
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 8: "`authentication_ldap_sasl_auth_method_name` now defaults to `SCRAM-SHA-256`), `group_replication_communication_stack`, `group_replication_ip_allowlist` (9.7.2)."
    - Character Sets and Collations › Defaults and the `utf8` trap — paragraph 1: "…makes `utf8` and `utf8_*` collation names in statements mean `utf8mb4`:"
    - SQL Mode and Strictness — bullet 4: "Values bound as strings still follow `sql_mode` (see Application Code)."
    - Indexing › Descending, invisible and redundant indexes — bullet 4: "…buffer pool), which softens but does not remove that cost."
    - Reading Plans: EXPLAIN and EXPLAIN ANALYZE [New in 9.7] — paragraph 1: "…iterator **tree** (`explain_format=TREE`), and `FORMAT=JSON` uses **version 2** (access-path based)."
    - Transactions, Isolation and Locking › Implicit commits, autocommit and GTID consistency [New in 9.7] — paragraph 2: "…writes InnoDB fails** — on every server, replicas or not:"
    - Security › Authentication: caching_sha2_password only [New in 9.7] — bullet 4: "`caching_sha2_password_enforce_storage_format` forces users with another format to reset their passwords)."
    - Security › Least privilege and roles [New in 9.7] — paragraph 1: "…but inactive unless `activate_all_roles_on_login=ON` or `SET DEFAULT ROLE` was used:"
    - Security › Other server-side security settings — bullet 3: "…not turn it off in `CHANGE REPLICATION SOURCE TO` scripts."
    - Application Code and Drivers — bullet 9: "Use strict mode and validate input."
    - Replication and Operations Awareness [New in 9.7] — bullet 1: "…adds `SET @@SESSION.SQL_LOG_BIN=0` and `GTID_PURGED` lines when GTIDs are on."
    - Replication and Operations Awareness [New in 9.7] — bullet 2: "…is gone (9.5.0) and `replica_parallel_workers` must be ≥ 1 (9.3.0)."
    - Replication and Operations Awareness [New in 9.7] — bullet 3: "Group Replication defaults to `group_replication_ssl_mode=REQUIRED`."
    - Observability — bullet 3: "…digest; dashboards keyed on digest values change after the upgrade."
[^13]: MySQL 9.7 Reference Manual — mysql Client Options (--commands default FALSE; disabled commands; --system-command). https://dev.mysql.com/doc/refman/9.7/en/mysql-command-options.html
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 9: "…syntax errors; only `delimiter` and `\C` still work | 9.4.0"
    - Schema Migrations and Online DDL › Migration scripts and the `mysql` client [New in 9.7] — paragraph 1: "Since 9.4.0 the `mysql` client starts with `--commands=OFF`."
[^14]: Changes in MySQL 9.1.0 (2024-10-15) — CREATE VIEW IF NOT EXISTS, 255 row versions, derived-table column references, ROLLUP/SUM(DISTINCT) fixes, table_open_cache_triggers, GROUP BY ROLLUP (...) syntax, mysql --system-command. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-1-0.html
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 10: "…dt`) | Accepted | `ERROR 1054 Unknown column` | 9.1.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 13: "…One grand-total row (`NULL`, `0` for `COUNT(*)`) | 9.1.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 14: "`SUM(DISTINCT b'1100')` | 9 (wrong) | 12 | 9.1.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 6: "…IF NOT EXISTS` (not combinable with `OR REPLACE`) | 9.1.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 7: "…BY ROLLUP (a, b)` syntax (also on 8.4.x) | 9.1.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 8: "…are parsed only for statements that modify data | 9.1.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 9: "…versions per table raised from 64 to **255** | 9.1.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 10: "…logging on (more than 4 without) | 9.1.0 / 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 12: "…(8.4.13 client) | `OFF` / `OFF` | 9.4.0 / 9.1.0"
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 6: "`--early-plugin-load`."
    - Query Patterns and Performance › Set operations, VALUES, TABLE, ROLLUP — paragraph 1: "…"no rows" as "no data" must check the aggregate instead."
    - Schema Migrations and Online DDL › INSTANT row-version limit [New in 9.1] — paragraph 1: "…row version; 9.1.0 raised the limit from 64 to **255**:"
    - Stored Programs, Triggers, Views and Events › Triggers — bullet 4: "…shows up only on the first write, not on `SELECT`."
[^15]: Changes in MySQL 9.2.0 (2025-01-21) — CREATE LIBRARY, CREATE_SPATIAL_REFERENCE_SYSTEM, FLUSH PRIVILEGES deprecated, BINLOG label, subquery OFFSET fix, outer references in window clauses rejected. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-2-0.html
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 11: "…ORDER BY or PARTITION BY clause not allowed` | 9.2.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 15: "…1 OFFSET 10)` | 1 (wrong) | `NULL` | 9.2.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 21: "…unquoted stored-program label | Allowed | Syntax error | 9.2.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 22: "…the upgrade (before the affected tables are used) | 9.2.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 10: "`CREATE_SPATIAL_REFERENCE_SYSTEM` privilege (instead of `SUPER`) | 9.2.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 25: "…WebAssembly libraries (`LANGUAGE WASM`) | 9.0.0 / 9.2.0 / 9.4.0"
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 1: "`CREATE USER`, `GRANT`, `ALTER USER` never needed it."
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 5: "`CREATE SPATIAL REFERENCE SYSTEM` under `SUPER` (use `CREATE_SPATIAL_REFERENCE_SYSTEM`)."
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 6: "`--early-plugin-load`."
    - Query Patterns and Performance › Top-N per group, window functions, LATERAL, QUALIFY — bullet 3: "…`RANGE` frames reject nondeterministic `ORDER BY` expressions (`ERROR 3587`, 9.4.0)."
    - Stored Programs, Triggers, Views and Events › JavaScript stored programs and libraries [New in 9.7, Enterprise] — paragraph 1: "…component, which ships only with **MySQL Enterprise Edition** (and HeatWave)."
    - Removed / Changed in 9.7 — Migration Notes (from 8.4) — bullet 11: "…of the upgrade (9.2.0 incompatible change for an index-corruption fix)."
[^16]: Changes in MySQL 9.3.0 (2025-04-15) — removal of innodb_log_file_size, innodb_log_files_in_group, innodb_undo_tablespaces and Version Tokens; replica_parallel_workers minimum 1; --check-table-functions; container-aware InnoDB defaults; no Innovation downgrades. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-3-0.html
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 18: "Use `innodb_redo_log_capacity` for redo sizing | 9.0.0–9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 19: "…Version Tokens | Removed (use `semisync_source`/`semisync_replica`) | 9.3.0 / 9.5.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 20: "…= 0` | Allowed | Minimum is 1 | 9.3.0"
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 24: "…series; within 9.7.x LTS in-place downgrade is supported | 9.3.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 11: "…`>ANY`/`<ALL`/... quantified comparisons, also in the `SELECT` list | 9.3.0"
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 12: "`mysqldump --users` (dumps `CREATE USER`/`GRANT`) | 9.3.0"
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 13: "…as --container_aware is OFF`) | 9.3.0 / 9.6.0 / 9.7.0"
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 6: "`--early-plugin-load`."
    - Replication and Operations Awareness [New in 9.7] — bullet 2: "…is gone (9.5.0) and `replica_parallel_workers` must be ≥ 1 (9.3.0)."
    - Replication and Operations Awareness [New in 9.7] — bullet 10: "…sizes itself for the whole host and can be OOM-killed."
[^17]: MySQL 9.7 Reference Manual — Caching SHA-2 Pluggable Authentication (storage formats CRYPT5/PBKDF2). https://dev.mysql.com/doc/refman/9.7/en/caching-sha2-pluggable-authentication.html
    - What's New in MySQL 9.7 (since 8.4) › New SQL features in 9.0–9.7 [New in 9.7] — table row 21: "`caching_sha2_password_storage_format` (`CRYPT5` default, `PBKDF2` with SHA-512), `caching_sha2_password_enforce_storage_format` | 9.7.0"
    - Security › Authentication: caching_sha2_password only [New in 9.7] — paragraph 1: "…an external/plugin method), and every client library must support it:"
    - Security › Authentication: caching_sha2_password only [New in 9.7] — bullet 4: "`caching_sha2_password_enforce_storage_format` forces users with another format to reset their passwords)."
[^18]: MySQL 9.7 Reference Manual — Binary Logging Options and Variables (binlog_transaction_dependency_history_size default 10000000). https://dev.mysql.com/doc/refman/9.7/en/replication-options-binary-log.html
    - What's New in MySQL 9.7 (since 8.4) › Changed server defaults [New in 9.7] — table row 8: "…1,000,000; the manual and the server say 10,000,000) | 9.5.0"
[^19]: Changes in MySQL 9.7.2 (2026-07-28) — AUTO_INCREMENT column addition fix, optimizer wrong-result fixes, hypergraph fixes, JSON_VALUE fix, deprecated Group Replication variables. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-7-2.html
    - What's New in MySQL 9.7 (since 8.4) › Behaviour changes inside the 9.7 LTS series — table row 3: "`group_replication_communication_stack` and `group_replication_ip_allowlist` deprecated"
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 8: "`authentication_ldap_sasl_auth_method_name` now defaults to `SCRAM-SHA-256`), `group_replication_communication_stack`, `group_replication_ip_allowlist` (9.7.2)."
    - Query Patterns and Performance › Top-N per group, window functions, LATERAL, QUALIFY — bullet 5: "ORDER BY` with semijoin subqueries **modify the wrong rows**."
    - Query Patterns and Performance › JSON functions — paragraph 1: "…`DATE` context (earlier releases could return an invalid constant date)."
    - Schema Migrations and Online DDL › Changing columns safely — bullet 5: "…wrong values before 9.7.2; require ≥ 9.7.2 for such migrations."
[^20]: Changes in MySQL 9.7.4 (2026-10-20) — VERSION_WITH_UPDATE(), EXCHANGE PARTITION CHECK validation, relaxed row-size validation, hypergraph UPDATE/DELETE wrong-rows fix, index_merge_union=off fix. https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-7-4.html
    - What's New in MySQL 9.7 (since 8.4) › Behaviour changes inside the 9.7 LTS series — table row 5: "`index_merge_union=off` plans could drop a disjunct and return incomplete results"
    - Schema Design and Data Types › Constraints: NOT NULL, CHECK, defaults — paragraph 3: "…rows against the destination's `CHECK` constraints even with `WITHOUT VALIDATION`."
    - Indexing › Things that prevent index use — paragraph 6: "…drop a disjunct and return incomplete results with that setting."
    - Query Patterns and Performance › Top-N per group, window functions, LATERAL, QUALIFY — bullet 5: "ORDER BY` with semijoin subqueries **modify the wrong rows**."
[^21]: Changes in MySQL 26.7.0 (2026-07-28) and 26.10.0 (2026-10-20) — calendar versioning, Change Stream Applier, PQC TLS, MySQL REST Service component, Health Monitor, duality-view annotations, optimizer_switch parsing. https://dev.mysql.com/doc/relnotes/mysql/26.10/en/news-26-7-0.html , https://dev.mysql.com/doc/relnotes/mysql/26.10/en/news-26-10-0.html
    - What's New in MySQL 9.7 (since 8.4) › Do not use or recommend (features newer than 9.7) — paragraph 1: "…on 9.7, and flag them as bugs when they appear:"
[^22]: MySQL 9.7 Reference Manual — Server System Variables (enable_cascade_triggers: added and deprecated in 9.7.0). https://dev.mysql.com/doc/refman/9.7/en/added-deprecated-removed.html
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 2: "…`SET` prints warning 1681 (and 4166 when it is reset)."
    - Schema Design and Data Types › Foreign keys [New in 9.7] — bullet 4: "…but it is already deprecated: every `SET` raises warning 1681."
[^23]: MySQL 9.7 Reference Manual — The VECTOR Type; Vector Functions. https://dev.mysql.com/doc/refman/9.7/en/vector.html
    - Schema Design and Data Types › VECTOR [New in 9.7] — paragraph 1: "…index, no distance function and no comparison other than equality."
[^24]: MySQL 9.7 Reference Manual — Unicode Character Sets (PAD SPACE vs NO PAD). https://dev.mysql.com/doc/refman/9.7/en/charset-unicode-sets.html
    - Character Sets and Collations › `utf8mb4_0900_ai_ci` semantics — paragraph 1: "Older collations (`utf8mb4_general_ci`, `utf8mb4_unicode_ci`, `utf8mb4_bin`) are `PAD SPACE`."
[^25]: MySQL 9.7 Reference Manual — Server SQL Modes. https://dev.mysql.com/doc/refman/9.7/en/sql-mode.html
    - SQL Mode and Strictness — paragraph 1: "The server's default `sql_mode` is strict:"
[^26]: MySQL 9.7 Reference Manual — MySQL Server Time Zone Support. https://dev.mysql.com/doc/refman/9.7/en/time-zone-support.html
    - Date, Time and Time Zones › Literals with offsets, `AT TIME ZONE`, named zones — paragraph 1: "…and `SET time_zone` need the time-zone tables to be loaded"
[^27]: MySQL 9.7 Reference Manual — CREATE INDEX Statement (functional key parts, multi-valued indexes). https://dev.mysql.com/doc/refman/9.7/en/create-index.html
    - Indexing › What InnoDB indexes can and cannot do — paragraph 1: "…and `SPATIAL` (R-tree) indexes, plus functional and multi-valued B-tree indexes."
    - Indexing › Functional indexes and JSON — paragraph 1: "…the plain cast is not used by `doc->>'$.name' = ?`."
[^28]: MySQL 9.7 Reference Manual — Optimizer Statistics (histograms). https://dev.mysql.com/doc/refman/9.7/en/optimizer-statistics.html
    - Indexing › Histograms — paragraph 1: "…selectivity estimates without index maintenance cost, and can refresh automatically:"
[^29]: MySQL 9.7 Reference Manual — EXPLAIN Statement (formats, EXPLAIN ANALYZE, INTO). https://dev.mysql.com/doc/refman/9.7/en/explain.html
    - Reading Plans: EXPLAIN and EXPLAIN ANALYZE [New in 9.7] — paragraph 1: "…iterator **tree** (`explain_format=TREE`), and `FORMAT=JSON` uses **version 2** (access-path based)."
[^30]: MySQL 9.7 Reference Manual — Row Constructor Expression Optimization. https://dev.mysql.com/doc/refman/9.7/en/row-constructor-optimization.html
    - Query Patterns and Performance › Pagination: OFFSET vs keyset — and the row-constructor trap — paragraph 2: "Write the expanded `OR` form, which becomes a range scan:"
[^31]: MySQL 9.7 Reference Manual — Optimizer Hints. https://dev.mysql.com/doc/refman/9.7/en/optimizer-hints.html
    - Query Patterns and Performance › Optimizer hints and index hints — bullet 1: "*/`)"
[^32]: MySQL 9.7 Reference Manual — INSERT ... ON DUPLICATE KEY UPDATE Statement. https://dev.mysql.com/doc/refman/9.7/en/insert-on-duplicate.html
    - Writes: INSERT, Upserts, UPDATE, DELETE › INSERT ... ON DUPLICATE KEY UPDATE — paragraph 1: "`VALUES(col)` in the update clause is deprecated:"
[^33]: MySQL 9.7 Reference Manual — Consistent Nonlocking Reads. https://dev.mysql.com/doc/refman/9.7/en/innodb-consistent-read.html
    - Transactions, Isolation and Locking › Consistent reads vs current reads — paragraph 1: "SELECT` and locking reads see the latest committed data."
[^34]: MySQL 9.7 Reference Manual — Locks Set by Different SQL Statements in InnoDB. https://dev.mysql.com/doc/refman/9.7/en/innodb-locks-set.html
    - Transactions, Isolation and Locking › Locks taken by locking reads and writes — paragraph 1: "…plus the gap before it (next-key locks) under REPEATABLE READ."
[^35]: MySQL 9.7 Reference Manual — Online DDL Operations. https://dev.mysql.com/doc/refman/9.7/en/innodb-online-ddl-operations.html
    - Schema Migrations and Online DDL › ALGORITHM and LOCK — paragraph 1: "With it, an unsupported combination fails immediately."
    - Schema Migrations and Online DDL › INSTANT row-version limit [New in 9.1] — paragraph 1: "…row version; 9.1.0 raised the limit from 64 to **255**:"
[^36]: MySQL 9.7 Reference Manual — Metadata Locking. https://dev.mysql.com/doc/refman/9.7/en/metadata-locking.html
    - Schema Migrations and Online DDL › Metadata locks: the most common migration outage — paragraph 1: "…waits, **every new query on the table queues behind it**:"
[^37]: MySQL 9.7 Reference Manual — Stored Program Binary Logging. https://dev.mysql.com/doc/refman/9.7/en/stored-programs-logging.html
    - Stored Programs, Triggers, Views and Events › Stored functions and binary logging — paragraph 1: "…a stored function requires a characteristic that declares it safe:"
[^38]: MySQL 9.7 Reference Manual — JavaScript Stored Programs / Multilingual Engine Component (MLE), MySQL Enterprise Edition. https://dev.mysql.com/doc/refman/9.7/en/stored-routines-js.html
    - Stored Programs, Triggers, Views and Events › JavaScript stored programs and libraries [New in 9.7, Enterprise] — paragraph 1: "…component, which ships only with **MySQL Enterprise Edition** (and HeatWave)."
[^39]: MySQL 9.7 Reference Manual — JSON Duality Views (CREATE JSON DUALITY VIEW, DML, ETAG concurrency). https://dev.mysql.com/doc/refman/9.7/en/json-duality-views.html
    - Stored Programs, Triggers, Views and Events › JSON duality views [New in 9.7] — paragraph 1: "…documents; since 9.7.0 Community Server also accepts `INSERT`/`UPDATE`/`DELETE` through them."
[^40]: go-sql-driver/mysql — README (DSN parameters parseTime, loc, multiStatements, interpolateParams, clientFoundRows). https://github.com/go-sql-driver/mysql
    - Security › SQL injection: parameters for values, allow-lists for identifiers — paragraph 1: "Verified with Go `database/sql` and `github.com/go-sql-driver/mysql` v1.10.1 against 9.7.4:"
    - Application Code and Drivers — paragraph 1: "…v1.10.1 against 9.7.4; the same concepts apply to other drivers."
[^41]: MySQL 9.7 Reference Manual — Using Roles (mandatory roles, activate_all_roles_on_login, activate_mandatory_roles). https://dev.mysql.com/doc/refman/9.7/en/roles.html
    - Security › Least privilege and roles [New in 9.7] — paragraph 2: "Review rules:"
[^42]: MySQL 9.7 Reference Manual — GRANT Statement (wildcards in database names). https://dev.mysql.com/doc/refman/9.7/en/grant.html
    - Security › Least privilege and roles [New in 9.7] — paragraph 2: "Review rules:"
[^43]: MySQL 9.7 Reference Manual — Upgrade Paths. https://dev.mysql.com/doc/refman/9.7/en/upgrade-paths.html
    - Replication and Operations Awareness [New in 9.7] — bullet 5: "Innovation releases (26.x) cannot be downgraded in place."
    - Removed / Changed in 9.7 — Migration Notes (from 8.4) — bullet 13: "…LTS series); keep a logical backup from before the upgrade."
[^44]: MySQL 9.7 Reference Manual — The sys Schema. https://dev.mysql.com/doc/refman/9.7/en/sys-schema.html
    - Observability — bullet 1: "The `sys` schema views above are installed by default."
[^45]: MySQL 9.7 Reference Manual — Legacy Hashing Component (component_classic_hashing; generated columns using MD5()/SHA1() stop working after upgrading to 9.6+). https://dev.mysql.com/doc/refman/9.7/en/legacy-hashing-component.html
    - Security › Hashing and encryption functions [New in 9.7] — paragraph 1: "…when the component is installed (`ERROR 3763`, `3770`, `3814`, `3758`)."
[^46]: MySQL 9.7 Reference Manual — FOREIGN KEY Constraints (SQL-layer handling, `innodb_native_foreign_keys`, AUTO_INCREMENT on rejected inserts, 15-level cascade limit, REPLACE and cascade triggers) and InnoDB Startup Options (`innodb_native_foreign_keys`: deprecated). https://dev.mysql.com/doc/refman/9.7/en/create-table-foreign-keys.html , https://dev.mysql.com/doc/refman/9.7/en/innodb-parameters.html
    - What's New in MySQL 9.7 (since 8.4) › Breaking changes for application SQL and migrations [New in 9.7] — table row 23: "…next id 1 on 9.7.4, 2 on 8.4.13) | 9.6.0"
    - What's New in MySQL 9.7 (since 8.4) › Deprecated in 9.7 (works, but flag it) — bullet 9: "Treat it as a temporary migration aid only."
    - Schema Design and Data Types › Foreign keys [New in 9.7] — bullet 5: "Other documented differences of the SQL layer:"
