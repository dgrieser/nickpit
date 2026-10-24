### Citations: PostgreSQL 14: The Complete Review Guide

Sources for `postgresql-14.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: PostgreSQL 14 Release Notes (Release 14.0, September 30, 2021), including "Migration to Version 14". https://www.postgresql.org/docs/release/14.0/
    - Overview — quote block: "PostgreSQL 14.0 was released on **September 30, 2021**."
    - Overview — paragraph 1: "…**`pg_stat_wal`**, **`pg_stat_replication_slots`**, **`pg_stat_progress_copy`** and **`pg_backend_memory_contexts`**, and **pipeline mode in libpq**."
    - Overview — paragraph 2: "…and `factorial()` of a negative number now raises an error."
    - What's New in PostgreSQL 14 — paragraph 1: "All items below are taken from the official release notes."
[^2]: PostgreSQL Versioning Policy (supported versions and final release dates: 14.24 current, final release November 12, 2026; 13.23, November 13, 2025; 18.6, 17.11, 16.15, 15.19). https://www.postgresql.org/support/versioning/
    - Overview — quote block: "PostgreSQL 14.0 was released on **September 30, 2021**."
    - Overview — quote block: "…same day as the next quarterly update of all branches."
    - Overview — quote block: "…community publishes no more bug or security fixes for 14.x."
    - Overview — quote block: "…end of life since November 13, 2025 (last release 13.23)."
    - Overview — quote block: "It is pre-release software and not a deployment target."
    - Upgrading Off PostgreSQL 14 — paragraph 1: "`pg_upgrade` and dump/restore can skip major versions."
[^3]: PostgreSQL Roadmap (next minor releases November 12, 2026, February 11, 2027, ...). https://www.postgresql.org/developer/roadmap/
    - Overview — quote block: "…same day as the next quarterly update of all branches."
[^4]: PostgreSQL 14.24 Release Notes (August 13, 2026). https://www.postgresql.org/docs/release/14.24/
    - Overview — quote block: "The latest minor release is **14.24 (August 13, 2026)**."
    - PostgreSQL 14.x Minor Releases (Fixes That Require Action) — paragraph 1: "…minor version: anything older than 14.24 lacks published security fixes."
    - PostgreSQL 14.x Minor Releases (Fixes That Require Action) — bullet 21: "FROM STDIN` that failed at startup (CVE-2026-6464)."
    - Security › Other security items — bullet 3: "Restore dumps with an up-to-date psql."
    - Security › Other security items — bullet 4: "Cipher cannot be initialized ?`, while `aes256` round-trips."
    - Security › Other security items — bullet 5: "…12, 2026, new vulnerabilities in 14 will not be fixed."
    - SQL Idioms and Pitfalls › Regular expressions without the 15 functions — paragraph 1: "…in the regexp match/split functions with invalidly encoded input (CVE-2026-14664)."
[^5]: PostgreSQL 14.1–14.23 Release Notes. https://www.postgresql.org/docs/release/14.1/ (and /14.2/ … /14.23/ under the same path)
    - New in 14: Multirange Types — bullet 4: "…a missing `CREATE` privilege check on the multirange's schema (CVE-2026-6472)."
    - New in 14: REINDEX on Partitioned Tables, DETACH PARTITION CONCURRENTLY — bullet 4: "…the repair for such partitions if they were not dropped."
    - PostgreSQL 14.x Minor Releases (Fixes That Require Action) — paragraph 1: "…minor version: anything older than 14.24 lacks published security fixes."
    - Security › SQL injection in dynamic SQL — paragraph 2: "…(CVE-2025-1094); clients linked against an older libpq are still affected."
    - Security › SECURITY DEFINER functions — paragraph 3: "…(`REFRESH MATERIALIZED VIEW CONCURRENTLY` running code as the refreshing user)."
    - Security › Row-level security — bullet 6: "…role change when RLS applied to a subquery or CTE)."
    - Security › Other security items — bullet 3: "Restore dumps with an up-to-date psql."
    - Partitioning — bullet 12: "…be enforced incorrectly; such constraints may need to be recreated."
[^6]: PostgreSQL 14.4 Release Notes (June 16, 2022: corruption of indexes built with CONCURRENTLY; Memoize fixes). https://www.postgresql.org/docs/release/14.4/ ; announcement with the `pg_amcheck --heapallindexed` detection advice: https://www.postgresql.org/about/news/postgresql-144-released-2470/
    - New in 14: Planner, Statistics and Monitoring › Memoize — paragraph 1: "`enable_memoize = off` is the escape hatch for a regression."
    - PostgreSQL 14.x Minor Releases (Fixes That Require Action) — bullet 4: "`pg_amcheck --heapallindexed` detects affected B-tree indexes."
    - Schema Migrations and Locking › Indexes in migrations — bullet 1: "…miss rows; such indexes must be rebuilt (see [minor releases](#postgresql-14x-minor-releases-fixes-that-require-action))."
[^7]: PostgreSQL 15 Release Notes (Release 15.0, October 13, 2022; PQsendQuery() in pipeline mode). https://www.postgresql.org/docs/release/15.0/
    - Do Not Use or Recommend (PostgreSQL 15+ Features) › PostgreSQL 15 (released October 13, 2022) — section heading
    - New in 14: libpq Pipeline Mode — bullet 2: "…in pipeline mode; libpq 15 rejects it, so use `PQsendQueryParams()`."
[^8]: PostgreSQL 16 Release Notes (Release 16.0, September 14, 2023; CREATEROLE restrictions). https://www.postgresql.org/docs/release/16.0/
    - Do Not Use or Recommend (PostgreSQL 15+ Features) › PostgreSQL 16 (released September 14, 2023) — section heading
    - Security › Roles and privileges — bullet 2: "…including `pg_execute_server_program`, `pg_read_server_files`, `pg_write_server_files`, `pg_read_all_data` and `pg_write_all_data`. 16 restricts this."
[^9]: PostgreSQL 17 Release Notes (Release 17.0, September 26, 2024). https://www.postgresql.org/docs/release/17.0/
    - Do Not Use or Recommend (PostgreSQL 15+ Features) › PostgreSQL 17 (released September 26, 2024) — section heading
[^10]: PostgreSQL 18 Release Notes (Release 18.0, September 25, 2025). https://www.postgresql.org/docs/release/18.0/
    - Do Not Use or Recommend (PostgreSQL 15+ Features) › PostgreSQL 18 (released September 25, 2025) and 19 (RC1) — section heading
    - Upgrading Off PostgreSQL 14 › What breaks on the way from 14 (checklist) — bullet 13: "MD5 passwords are deprecated with warnings."
[^11]: PostgreSQL 13.23 Release Notes (November 13, 2025, final 13.x release). https://www.postgresql.org/docs/release/13.23/
    - Overview — quote block: "…end of life since November 13, 2025 (last release 13.23)."
[^12]: PostgreSQL 14 Documentation: Range Types (multiranges). https://www.postgresql.org/docs/14/rangetypes.html
    - New in 14: Multirange Types — paragraph 1: "…a multirange type: an ordered set of non-overlapping, non-adjacent ranges."
[^13]: PostgreSQL 14 Documentation: WITH Queries (Search Order, Cycle Detection). https://www.postgresql.org/docs/14/queries-with.html
    - New in 14: SEARCH and CYCLE in Recursive CTEs — bullet 4: "…= ANY(w.path)` (on the graph above it stops after `{1,2,3}`)."
[^14]: PostgreSQL 14 Documentation: JSON Types — jsonb Subscripting. https://www.postgresql.org/docs/14/datatype-json.html
    - New in 14: jsonb Subscripting — paragraph 1: "Behaviour verified on 14.24:"
[^15]: PostgreSQL 14 Documentation: CREATE PROCEDURE; CALL. https://www.postgresql.org/docs/14/sql-createprocedure.html , https://www.postgresql.org/docs/14/sql-call.html
    - New in 14: Procedures with OUT Parameters and SQL-Standard Function Bodies › OUT parameters for procedures — bullet 3: "…only way to return data from a procedure was `INOUT`."
[^16]: PostgreSQL 14 Documentation: CREATE FUNCTION (SQL-standard bodies; Writing SECURITY DEFINER Functions Safely). https://www.postgresql.org/docs/14/sql-createfunction.html
    - New in 14: Procedures with OUT Parameters and SQL-Standard Function Bodies › SQL-standard function bodies (BEGIN ATOMIC / RETURN) — bullet 5: "On 13 both forms are syntax errors."
    - Security › SECURITY DEFINER functions — paragraph 1: "FROM PUBLIC`."
[^17]: PostgreSQL 14 Documentation: Date/Time Functions (date_bin). https://www.postgresql.org/docs/14/functions-datetime.html
    - New in 14: date_bin, string_to_table and Other Functions — bullet 1: "…ts AT TIME ZONE 'Europe/Berlin')`; for months use `date_trunc('month', ...)`."
[^18]: PostgreSQL 14 Documentation: Predefined Roles. https://www.postgresql.org/docs/14/predefined-roles.html
    - What's New in PostgreSQL 14 › Privileges, security and authentication — bullet 1: "…still obey row-level security unless the role also has `BYPASSRLS`."
    - New in 14: Predefined Roles and Authentication — paragraph 1: "Measured on 14.24:"
[^19]: PostgreSQL 14 Documentation: Explicit Locking. https://www.postgresql.org/docs/14/explicit-locking.html
    - Schema Migrations and Locking › Lock levels of common DDL (measured on 14.24 with pg_locks) — paragraph 1: "…for lock modes: the "Explicit Locking" and `ALTER TABLE` documentation."
[^20]: PostgreSQL 14 Documentation: ALTER TABLE; CREATE INDEX (Building Indexes Concurrently). https://www.postgresql.org/docs/14/sql-altertable.html , https://www.postgresql.org/docs/14/sql-createindex.html
    - New in 14: REINDEX on Partitioned Tables, DETACH PARTITION CONCURRENTLY — paragraph 1: "Verified on 14.24 (and on 13.23 for the failures):"
    - Schema Migrations and Locking › Lock levels of common DDL (measured on 14.24 with pg_locks) — paragraph 1: "…for lock modes: the "Explicit Locking" and `ALTER TABLE` documentation."
[^21]: PostgreSQL 14 Documentation: REINDEX. https://www.postgresql.org/docs/14/sql-reindex.html
    - New in 14: REINDEX on Partitioned Tables, DETACH PARTITION CONCURRENTLY — paragraph 1: "Verified on 14.24 (and on 13.23 for the failures):"
[^22]: PostgreSQL 14 Documentation: Transaction Isolation. https://www.postgresql.org/docs/14/transaction-iso.html
    - Transactions, Isolation and Concurrency › Isolation levels (behaviour verified with two sessions on 14.24) — section heading
[^23]: PostgreSQL 14 Documentation: Schemas — Usage Patterns (secure schema usage, CVE-2018-1058). https://www.postgresql.org/docs/14/ddl-schemas.html
    - Security › The public schema is writable by everyone on 14 — paragraph 2: "…objects in a dedicated schema owned by the migration role:"
[^24]: PostgreSQL 14 Documentation: Row Security Policies. https://www.postgresql.org/docs/14/ddl-rowsecurity.html
    - Security › Row-level security — paragraph 1: "Behaviour verified on 14.24:"
[^25]: PostgreSQL 14 Documentation: Table Partitioning. https://www.postgresql.org/docs/14/ddl-partitioning.html
    - Partitioning — paragraph 1: "Rules verified on 14.24:"
[^26]: PostgreSQL 14 Documentation: libpq Pipeline Mode; pgbench (\startpipeline). https://www.postgresql.org/docs/14/libpq-pipeline-mode.html , https://www.postgresql.org/docs/14/pgbench.html
    - What's New in PostgreSQL 14 › Client side and tools — bullet 1: "…the simple query protocol` and ran it with `-M extended`."
    - New in 14: libpq Pipeline Mode — paragraph 1: "…waiting for each result, which cuts latency over slow links."
[^27]: PostgreSQL 14 Documentation: pg_stat_statements. https://www.postgresql.org/docs/14/pgstatstatements.html
    - New in 14: Planner, Statistics and Monitoring › Query IDs and pg_stat_statements 1.9 — bullet 2: "…(15), and `total_time` does not exist (renamed `total_exec_time` in 13)."
[^28]: PostgreSQL Wiki: Don't Do This. https://wiki.postgresql.org/wiki/Don't_Do_This
    - Data Types: Pitfalls › Timestamps — bullet 1: "Use **`timestamptz`** for points in time."
[^29]: PgBouncer configuration (max_prepared_statements, transaction pooling restrictions). https://www.pgbouncer.org/config.html
    - Connection Pooling and Application Access — bullet 2: "…locks, `LISTEN`, `WITH HOLD` cursors, temporary tables, and SQL-level `PREPARE`/`EXECUTE`."
    - Connection Pooling and Application Access — bullet 3: "SQL `PREPARE` statements are still not tracked."
[^30]: PostgreSQL 14 Documentation: ALTER DEFAULT PRIVILEGES. https://www.postgresql.org/docs/14/sql-alterdefaultprivileges.html
    - Security › SECURITY DEFINER functions — paragraph 2: "…the role it was run as (or the `FOR ROLE`)."
[^31]: PostgreSQL 14 Documentation: CREATE VIEW (security_barrier; no security_invoker). https://www.postgresql.org/docs/14/sql-createview.html
    - Security › Views and row-level security on 14 — paragraph 1: "…on 14 the option is rejected with `unrecognized parameter "security_invoker"`."
[^32]: PostgreSQL 18.6, 17.11, 16.15, 15.19, 14.24 and 19 Beta 3 Released ("PostgreSQL 14 EOL Notice": no fixes after November 12, 2026). https://www.postgresql.org/about/news/postgresql-186-1711-1615-1519-1424-and-19-beta-3-released-3365/
    - Overview — quote block: "…community publishes no more bug or security fixes for 14.x."
[^33]: PostgreSQL 14 Documentation: pg_upgrade; Upgrading a PostgreSQL Cluster. https://www.postgresql.org/docs/14/pgupgrade.html , https://www.postgresql.org/docs/14/upgrading.html
    - Upgrading Off PostgreSQL 14 — paragraph 1: "`pg_upgrade` and dump/restore can skip major versions."
[^34]: PostgreSQL 19 RC 1 Released! (October 15, 2026; RC 2 on November 12, GA planned November 19, 2026). https://www.postgresql.org/about/news/postgresql-19-rc-1-released-3409/
    - Overview — quote block: "It is pre-release software and not a deployment target."
