### Citations: MySQL Style Guide

Sources for `mysql.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: MySQL 9.7 Reference Manual — MySQL Releases: Innovation and LTS (release tracks, support periods, calendar versioning after 9.7). https://dev.mysql.com/doc/refman/9.7/en/mysql-releases.html
    - Release Lines and Support (as of October 2026) — paragraph 1: "…release; each is supported only until the next one ships)."
    - Release Lines and Support (as of October 2026) — paragraph 1: "…use calendar versions `YY.M.P` (26.7 is the July 2026 release)."
[^2]: MySQL 26.10 Release Notes (26.7.0 2026-07-28, 26.7.1 2026-08-18, 26.10.0 2026-10-20); Changes in MySQL 26.7.0 (calendar-based versioning after 9.7 LTS). https://dev.mysql.com/doc/relnotes/mysql/26.10/en/ , https://dev.mysql.com/doc/relnotes/mysql/26.10/en/news-26-7-0.html
    - Release Lines and Support (as of October 2026) — paragraph 1: "…use calendar versions `YY.M.P` (26.7 is the July 2026 release)."
    - Release Lines and Support (as of October 2026) — table row 7: "…26.10 is the current Innovation release; 26.7 is superseded"
[^3]: MySQL 5.7 Release Notes (5.7.44, 2023-10-25, last release). https://dev.mysql.com/doc/relnotes/mysql/5.7/en/
    - Release Lines and Support (as of October 2026) — table row 1: "…5.7.44 (2023-10-25) | **End of life** since October 2023"
[^4]: MySQL 8.0 Release Notes (8.0.46, 2026-04-21; End of Life statement). https://dev.mysql.com/doc/relnotes/mysql/8.0/en/
    - Release Lines and Support (as of October 2026) — table row 2: "…2026 ("with version 8.0.46, MySQL 8.0 reaches End of Life")"
[^5]: MySQL 8.4 Release Notes (8.4.0 2024-04-30 … 8.4.13 2026-10-20). https://dev.mysql.com/doc/relnotes/mysql/8.4/en/
    - Release Lines and Support (as of October 2026) — table row 4: "Premier Support until April 2029, Extended until April 2032"
[^6]: endoflife.date — MySQL (Premier/Extended Support end dates per series). https://endoflife.date/mysql
    - Release Lines and Support (as of October 2026) — table row 4: "Premier Support until April 2029, Extended until April 2032"
    - Release Lines and Support (as of October 2026) — table row 6: "Premier Support until April 2031, Extended until April 2034"
[^7]: MySQL 9.7 Release Notes (9.0.0 2024-07-01 … 9.7.0 2026-04-21 … 9.7.4 2026-10-20). https://dev.mysql.com/doc/relnotes/mysql/9.7/en/
    - Release Lines and Support (as of October 2026) — table row 6: "Premier Support until April 2031, Extended until April 2034"
[^8]: MySQL 9.7 Reference Manual — Upgrade Paths (LTS series cannot be skipped). https://dev.mysql.com/doc/refman/9.7/en/upgrade-paths.html
    - Release Lines and Support (as of October 2026) — paragraph 2: "Upgrade paths:"
[^9]: MySQL 8.0 Reference Manual — What Is New in MySQL 8.0 (features added per 8.0.x release, deprecations, removals). https://dev.mysql.com/doc/refman/8.0/en/mysql-nutshell.html
    - Version-aware Review › Feature availability by version — paragraph 1: "…items are missing from earlier 8.0 patches and from 5.7."
    - Constraints: NOT NULL, CHECK, Defaults — paragraph 2: "…TABLE` or reload on 8.0.16+ then fails on violating rows."
    - Removed and Changed Behaviour Across Versions — paragraph 1: "…these are the changes that break SQL or change results."
[^10]: MySQL 8.0 Reference Manual — Changes in MySQL 8.0 (upgrading from 5.7). https://dev.mysql.com/doc/refman/8.0/en/upgrading-from-previous-series.html
    - Version-aware Review › Feature availability by version — paragraph 1: "…items are missing from earlier 8.0 patches and from 5.7."
    - Removed and Changed Behaviour Across Versions — paragraph 1: "…these are the changes that break SQL or change results."
[^11]: MySQL 8.4 Reference Manual — What Is New in MySQL 8.4 since MySQL 8.0. https://dev.mysql.com/doc/refman/8.4/en/mysql-nutshell.html
    - Version-aware Review › Feature availability by version — paragraph 1: "…items are missing from earlier 8.0 patches and from 5.7."
    - Removed and Changed Behaviour Across Versions — paragraph 1: "…these are the changes that break SQL or change results."
[^12]: Changes in MySQL 9.0.0–9.7.0 (release notes: VECTOR, inline REFERENCES, mysql_native_password removal, CREATE VIEW IF NOT EXISTS, MD5/SHA1 component, changed defaults, SQL-layer foreign keys, hypergraph optimizer). https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-0-0.html , https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-5-0.html , https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-6-0.html , https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-7-0.html
    - Version-aware Review › Feature availability by version — paragraph 1: "…items are missing from earlier 8.0 patches and from 5.7."
    - Removed and Changed Behaviour Across Versions — paragraph 1: "…these are the changes that break SQL or change results."
[^13]: MariaDB Documentation — Incompatibilities and Feature Differences Between MariaDB and MySQL; MySQL to MariaDB Compatibility Matrix. https://mariadb.com/docs/release-notes/compatibility-and-differences/incompatibilities-and-feature-differences-between-mariadb-10-6-and-mysql-8 , https://mariadb.com/docs/server/server-management/install-and-upgrade-mariadb/migrating-to-mariadb/moving-from-mysql/mysql-to-mariadb-compatibility-matrix
    - MariaDB Is Not MySQL — paragraph 1: "…treat a MariaDB-only feature as valid MySQL (or vice versa):"
[^14]: MySQL 9.7 Reference Manual — Generated Invisible Primary Keys; Server System Variables (sql_require_primary_key). https://dev.mysql.com/doc/refman/9.7/en/create-table-gipks.html
    - Schema Design and Data Types › Primary keys — paragraph 3: "…NULL` key as a primary-key substitute; earlier versions do not."
[^15]: MySQL 9.7 Reference Manual — Legacy Hashing Component (MD5()/SHA1() after 9.6). https://dev.mysql.com/doc/refman/9.7/en/legacy-hashing-component.html
    - Schema Design and Data Types › Generated and invisible columns — bullet 2: "…`SHA2()` in generated columns, defaults, `CHECK` constraints and functional indexes."
[^16]: MySQL 9.7 Reference Manual — The VECTOR Type. https://dev.mysql.com/doc/refman/9.7/en/vector.html
    - Schema Design and Data Types › VECTOR (9.0+) — paragraph 1: "…similarity search in the application or a dedicated vector store."
[^17]: MySQL 8.4 / 9.7 Reference Manual — FOREIGN KEY Constraints (inline REFERENCES ignored before 9.0, non-standard keys, cascades and triggers). https://dev.mysql.com/doc/refman/8.4/en/create-table-foreign-keys.html , https://dev.mysql.com/doc/refman/9.7/en/create-table-foreign-keys.html
    - Foreign Keys — bullet 1: "Always write the explicit, named table-level form:"
[^18]: Changes in MySQL 9.7.0 (enable_cascade_triggers). https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-7-0.html
    - Foreign Keys — bullet 3: "(9.7.0 adds an opt-in, already deprecated `enable_cascade_triggers`.)"
[^19]: MySQL 8.4 Reference Manual — Unicode Character Sets (PAD SPACE vs NO PAD). https://dev.mysql.com/doc/refman/8.4/en/charset-unicode-sets.html
    - Character Sets and Collations › Collation semantics — paragraph 1: "…collations (`utf8mb4_general_ci`, `utf8mb4_unicode_ci`, `utf8mb4_bin`, all 5.7 collations) are `PAD SPACE`."
[^20]: MySQL 8.4 Reference Manual — Server SQL Modes. https://dev.mysql.com/doc/refman/8.4/en/sql-mode.html
    - SQL Mode and Strictness — paragraph 1: "The default `sql_mode` is strict (5.7.5+):"
[^21]: MySQL 8.4 Reference Manual — MySQL Server Time Zone Support. https://dev.mysql.com/doc/refman/8.4/en/time-zone-support.html
    - Date, Time and Time Zones › Time zones and date arithmetic — bullet 1: "…`SELECT COUNT(*) FROM mysql.time_zone_name` when code relies on named zones."
[^22]: MySQL 8.4 Reference Manual — CREATE INDEX Statement (functional key parts, multi-valued indexes). https://dev.mysql.com/doc/refman/8.4/en/create-index.html
    - Indexing › What InnoDB indexes can and cannot do — paragraph 1: "…(R-tree) indexes, plus functional **(8.0.13+)** and multi-valued **(8.0.17+)** B-tree indexes."
    - Indexing › Functional indexes and JSON (8.0.13+ / 8.0.17+) — paragraph 1: "…CHAR)` (connection collation) is not used by `doc->>'$.name' = ?`:"
[^23]: MySQL 8.4 Reference Manual — EXPLAIN Statement; Changes in MySQL 9.5.0 (explain_format default TREE). https://dev.mysql.com/doc/refman/8.4/en/explain.html , https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-5-0.html
    - Reading Plans: EXPLAIN and EXPLAIN ANALYZE — bullet 1: "…tooling that parses `EXPLAIN` output must pin the format explicitly."
[^24]: MySQL 8.4 Reference Manual — Row Constructor Expression Optimization. https://dev.mysql.com/doc/refman/8.4/en/row-constructor-optimization.html
    - Query Patterns and Performance › Pagination: OFFSET vs keyset — and the row-constructor trap — paragraph 2: "Write the expanded `OR` form:"
[^25]: MySQL 8.4 Reference Manual — Consistent Nonlocking Reads. https://dev.mysql.com/doc/refman/8.4/en/innodb-consistent-read.html
    - Transactions, Isolation and Locking › Consistent reads vs current reads — paragraph 1: "SELECT` and locking reads see the latest committed data."
[^26]: MySQL 8.4 Reference Manual — Locks Set by Different SQL Statements in InnoDB. https://dev.mysql.com/doc/refman/8.4/en/innodb-locks-set.html
    - Transactions, Isolation and Locking › Locks taken by locking reads and writes — paragraph 1: "…plus the gap before it (next-key locks) under REPEATABLE READ:"
[^27]: MySQL 8.4 Reference Manual — Online DDL Operations. https://dev.mysql.com/doc/refman/8.4/en/innodb-online-ddl-operations.html
    - Schema Migrations and Online DDL › ALGORITHM and LOCK — paragraph 1: "…that blocks writes; with it, an unsupported combination fails immediately:"
[^28]: MySQL 8.4 Reference Manual — Metadata Locking. https://dev.mysql.com/doc/refman/8.4/en/metadata-locking.html
    - Schema Migrations and Online DDL › Metadata locks: the most common migration outage — paragraph 1: "…waits, **every new query on the table queues behind it**:"
[^29]: MySQL 9.7 Reference Manual — mysql Client Options (--commands). https://dev.mysql.com/doc/refman/9.7/en/mysql-command-options.html
    - Schema Migrations and Online DDL › Changing columns safely (expand/contract) — bullet 6: "…with `--commands`; init scripts that rely on `source` break there."
[^30]: MySQL 8.4 Reference Manual — Stored Program Binary Logging. https://dev.mysql.com/doc/refman/8.4/en/stored-programs-logging.html
    - Stored Programs, Triggers, Views and Events › Stored functions and binary logging — paragraph 1: "…a stored function requires a characteristic that declares it safe:"
[^31]: go-sql-driver/mysql — README (parseTime, loc, multiStatements, interpolateParams, clientFoundRows). https://github.com/go-sql-driver/mysql
    - Application Code, Drivers and Connection Pooling — paragraph 1: "…Go's `database/sql` and `github.com/go-sql-driver/mysql`; every mature driver has equivalent settings."
[^32]: Changes in MySQL 8.0.35 and Changes in MySQL 8.2.0 (2023-10-25), Deprecation and Removal Notes — wildcards in database-level grants and '%' as a synonym for localhost deprecated. https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-35.html , https://dev.mysql.com/doc/relnotes/mysql/8.2/en/news-8-2-0.html
    - Security › Least privilege and roles — bullet 3: "…8.0.35 and 8.2.0 (both released 2023-10-25); no warning is printed."
