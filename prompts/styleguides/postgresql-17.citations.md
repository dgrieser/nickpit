### Citations: PostgreSQL 17: The Complete Review Guide

Sources for `postgresql-17.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: PostgreSQL 17 Release Notes (E.12, Release 17.0). https://www.postgresql.org/docs/release/17.0/
    - Overview — quote block: "PostgreSQL 17.0 was released on **September 26, 2024**."
    - Overview — paragraph 1: "…**direct TLS negotiation** (`sslnegotiation=direct`) and **failover for logical replication slots**."
    - Overview — paragraph 2: "…moved or were renamed (`pg_stat_bgwriter` → `pg_stat_checkpointer`, `pg_stat_statements.blk_read_time` → `shared_blk_read_time`)."
    - What's New in PostgreSQL 17 — paragraph 1: "All items below are verified against the official release notes"
    - What's New in PostgreSQL 17 › Server, performance and operations — bullet 1: "`vacuum_buffer_usage_limit` defaults to 2MB (256kB in 16)."
    - What's New in PostgreSQL 17 › Server, performance and operations — bullet 2: "`io_combine_limit` (default 128kB) sets the maximum read size."
    - What's New in PostgreSQL 17 › Server, performance and operations — bullet 4: "…BY` keys can be reordered to match `ORDER BY` (`enable_group_by_reordering`)."
    - What's New in PostgreSQL 17 › Server, performance and operations — bullet 15: "PL/pgSQL `%TYPE` can reference a column of a composite variable."
    - New in 17: MAINTAIN Privilege, pg_maintain and the Maintenance search_path › Safe search_path during maintenance (breaking change) — paragraph 1: "…and `VACUUM` now run functions with `search_path = pg_catalog, pg_temp`."
[^2]: PostgreSQL Versioning Policy (supported versions, 17.11 current, EOL November 8, 2029; 18.6, 19 RC1 news) and PostgreSQL 17.11 Release Notes (August 13, 2026). https://www.postgresql.org/support/versioning/ , https://www.postgresql.org/docs/release/17.11/
    - Overview — quote block: "…the next scheduled minor release date is November 12, 2026."
    - Overview — quote block: "PostgreSQL 17 is **supported until November 8, 2029**."
    - Overview — quote block: "It is pre-release software and not a deployment target."
    - Security › Other security items — bullet 3: "…17.3) and CVE-2025-8713/8714/8715 (statistics leak and `pg_dump`/psql restore injection, 17.6)."
    - Maintenance, Monitoring and Observability — bullet 5: "…`btree_gist` float indexes with `NaN` and `ltree` indexes after 17.11)."
    - Upgrading to 17 and Staying Current › 17.x minor releases — bullet 10: "…WAL-logged) and a default-partition pruning bug that could drop rows."
[^3]: PostgreSQL Roadmap (next minor releases: November 12, 2026, ...). https://www.postgresql.org/developer/roadmap/
    - Overview — quote block: "…the next scheduled minor release date is November 12, 2026."
[^4]: PostgreSQL 17 Released! (press release, September 26, 2024: vacuum memory up to 20x less, WAL write throughput up to 2x, streaming I/O). https://www.postgresql.org/about/news/postgresql-17-released-2936/
    - Overview — paragraph 1: "…**direct TLS negotiation** (`sslnegotiation=direct`) and **failover for logical replication slots**."
    - What's New in PostgreSQL 17 › Server, performance and operations — bullet 1: "`vacuum_buffer_usage_limit` defaults to 2MB (256kB in 16)."
    - What's New in PostgreSQL 17 › Server, performance and operations — bullet 6: "**WAL** throughput under high concurrency improves by up to 2x."
[^5]: PostgreSQL 18 Release Notes (Release 18.0). https://www.postgresql.org/docs/release/18.0/
    - Do Not Use or Recommend (PostgreSQL 18+ Features) — paragraph 1: "…list was introduced in **PostgreSQL 18** (released September 25, 2025)"
[^6]: PostgreSQL 17 Documentation: MERGE. https://www.postgresql.org/docs/17/sql-merge.html
    - New in 17: MERGE Improvements — bullet 2: "Use explicit lists in application code."
    - New in 17: MERGE Improvements — bullet 4: "…with rules, materialized views and foreign tables are not supported."
    - New in 17: MERGE Improvements — bullet 5: "ON CONFLICT DO UPDATE`."
[^7]: PostgreSQL 17 Documentation: Explicit Locking. https://www.postgresql.org/docs/17/explicit-locking.html
    - Schema Migrations and Locking › Lock levels of common DDL (measured on 17.11 with pg_locks) — paragraph 1: "…for lock modes: the "Explicit Locking" and `ALTER TABLE` documentation."
[^8]: PostgreSQL 17 Documentation: ALTER TABLE; CREATE INDEX (Building Indexes Concurrently). https://www.postgresql.org/docs/17/sql-altertable.html , https://www.postgresql.org/docs/17/sql-createindex.html
    - Schema Migrations and Locking › Lock levels of common DDL (measured on 17.11 with pg_locks) — paragraph 1: "…for lock modes: the "Explicit Locking" and `ALTER TABLE` documentation."
[^9]: PostgreSQL 17 Documentation: CREATE FUNCTION — Writing SECURITY DEFINER Functions Safely. https://www.postgresql.org/docs/17/sql-createfunction.html
    - Security › SECURITY DEFINER functions — paragraph 1: "It applies only to functions later created by that role."
[^10]: PostgreSQL 17.6 Release Notes (August 14, 2025; CVE-2025-8713, CVE-2025-8714, CVE-2025-8715). https://www.postgresql.org/docs/release/17.6/
    - Security › Other security items — bullet 3: "…17.3) and CVE-2025-8713/8714/8715 (statistics leak and `pg_dump`/psql restore injection, 17.6)."
    - Maintenance, Monitoring and Observability — bullet 5: "…`btree_gist` float indexes with `NaN` and `ltree` indexes after 17.11)."
    - Upgrading to 17 and Staying Current › 17.x minor releases — bullet 6: "Reindex BRIN `numeric_minmax_multi_ops` indexes."
[^11]: PgBouncer configuration (max_prepared_statements, transaction pooling restrictions). https://www.pgbouncer.org/config.html
    - Connection Pooling and Application Access — bullet 2: "…locks, `LISTEN`, `WITH HOLD` cursors, temporary tables, and SQL-level `PREPARE`/`EXECUTE`."
    - Connection Pooling and Application Access — bullet 3: "SQL `PREPARE` statements are still not tracked."
[^12]: PostgreSQL 17.2 Release Notes (November 21, 2024). https://www.postgresql.org/docs/release/17.2/
    - Upgrading to 17 and Staying Current › 17.x minor releases — bullet 2: "SET role`."
[^13]: PostgreSQL 17.3 Release Notes (February 13, 2025; CVE-2025-1094). https://www.postgresql.org/docs/release/17.3/
    - Upgrading to 17 and Staying Current › 17.x minor releases — bullet 3: "…in libpq quoting functions (`PQescapeLiteral()` and friends now validate encoding)."
[^14]: PostgreSQL 17 Documentation: SQL/JSON functions (JSON_TABLE, JSON_VALUE, JSON_QUERY, JSON_EXISTS). https://www.postgresql.org/docs/17/functions-json.html
    - New in 17: SQL/JSON Query Functions and JSON_TABLE › JSON_TABLE — paragraph 1: "Each column has a type, a path and error handling."
[^15]: PostgreSQL 17 Documentation: COPY (ON_ERROR, LOG_VERBOSITY, FORCE_NULL). https://www.postgresql.org/docs/17/sql-copy.html
    - New in 17: COPY ON_ERROR and Related Options — paragraph 1: "What `ON_ERROR ignore` does **not** cover (tested):"
[^16]: PostgreSQL 17 Documentation: Row Security Policies. https://www.postgresql.org/docs/17/ddl-rowsecurity.html
    - Security › Row-level security — paragraph 1: "Behaviour verified on 17.11:"
[^17]: PostgreSQL 17 Documentation: Transaction Isolation. https://www.postgresql.org/docs/17/transaction-iso.html
    - Transactions, Isolation and Concurrency › Isolation levels (all behaviour verified with two sessions on 17.11) — section heading
[^18]: PostgreSQL 17 Documentation: Table Partitioning. https://www.postgresql.org/docs/17/ddl-partitioning.html
    - Partitioning — paragraph 1: "Rules verified on 17.11:"
[^19]: PostgreSQL 17 Documentation: Client Connection Defaults (transaction_timeout, statement_timeout, lock_timeout). https://www.postgresql.org/docs/17/runtime-config-client.html
    - New in 17: Server Settings Application Code Touches › transaction_timeout — paragraph 1: "`transaction_timeout` limits the total duration of a transaction,"
[^20]: PostgreSQL 17 Documentation: Predefined Roles (pg_maintain). https://www.postgresql.org/docs/17/predefined-roles.html
    - New in 17: MAINTAIN Privilege, pg_maintain and the Maintenance search_path › MAINTAIN — paragraph 1: "`MAINTAIN` allows"
[^21]: PostgreSQL 17 Documentation: libpq Connection Parameters (sslnegotiation). https://www.postgresql.org/docs/17/libpq-connect.html
    - New in 17: Connections, libpq, Backup and Replication — bullet 1: "…TLS immediately with ALPN `postgresql`, which saves a round trip."
[^22]: PostgreSQL 16, 15 and 14 Release Notes (feature version markers). https://www.postgresql.org/docs/release/16.0/ , https://www.postgresql.org/docs/release/15.0/ , https://www.postgresql.org/docs/release/14.0/
    - Features from PostgreSQL 14–16 You Can Rely On — paragraph 1: "…exist on 17 and should be used where they fit."
[^23]: PostgreSQL Wiki: Don't Do This. https://wiki.postgresql.org/wiki/Don't_Do_This
    - Data Types: Pitfalls › Timestamps — bullet 1: "Use **`timestamptz`** for points in time."
[^24]: PostgreSQL 19 Release Notes (draft, Release Candidate 1). https://www.postgresql.org/docs/19/release-19.html
    - Do Not Use or Recommend (PostgreSQL 18+ Features) — paragraph 2: "…error at or near "REPACK"`, `function error_on_null(integer) does not exist`)."
[^25]: PostgreSQL 17.1, 17.4, 17.5, 17.7, 17.8, 17.9 and 17.10 Release Notes. https://www.postgresql.org/docs/release/17.1/ , https://www.postgresql.org/docs/release/17.4/ , https://www.postgresql.org/docs/release/17.5/ , https://www.postgresql.org/docs/release/17.7/ , https://www.postgresql.org/docs/release/17.8/ , https://www.postgresql.org/docs/release/17.9/ , https://www.postgresql.org/docs/release/17.10/
    - Upgrading to 17 and Staying Current › 17.x minor releases — bullet 1: "…= C` but a different `LC_COLLATE` must reindex text indexes."
    - Upgrading to 17 and Staying Current › 17.x minor releases — bullet 4: "…in the 17.3 quoting fix (`PQescapeLiteral()`/`PQescapeIdentifier()` ignored their length argument)."
    - Upgrading to 17 and Staying Current › 17.x minor releases — bullet 5: "BRIN bloom indexes should be reindexed."
    - Upgrading to 17 and Staying Current › 17.x minor releases — bullet 7: "…checks `CREATE` on the schema) and CVE-2025-12818 (libpq allocation overflow)."
    - Upgrading to 17 and Staying Current › 17.x minor releases — bullet 8: "…(multixact WAL replay on standbys, a too-strict `substring()` encoding check)."
    - Upgrading to 17 and Staying Current › 17.x minor releases — bullet 9: "…allocation overflows. libpq's `PQfn()` is now marked unsafe and deprecated."
