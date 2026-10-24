### Citations: PostgreSQL 15: The Complete Review Guide

Sources for `postgresql-15.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: PostgreSQL 15 Release Notes (Release 15.0, October 13, 2022), including "Migration to Version 15". https://www.postgresql.org/docs/release/15.0/
    - Overview — quote block: "PostgreSQL 15.0 was released on **October 13, 2022**."
    - Overview — paragraph 1: "…more statistics collector process), and faster in-memory and on-disk sorts."
    - Overview — paragraph 2: "…same seed, and `log_checkpoints` and `log_autovacuum_min_duration` now log by default."
    - What's New in PostgreSQL 15 — paragraph 1: "All items below are taken from the official release notes."
    - What's New in PostgreSQL 15 › Privileges and security — bullet 1: "`pg_upgrade` and dump/restore keep the old ACL."
    - What's New in PostgreSQL 15 › Server, performance and operations — bullet 1: "…faster; sorts that exceed `work_mem` use a better external merge."
[^2]: PostgreSQL Versioning Policy (supported versions: 15.19 current, final release November 11, 2027; 18.6, 17.11, 16.15) and PostgreSQL 15.19 Release Notes (August 13, 2026). https://www.postgresql.org/support/versioning/ , https://www.postgresql.org/docs/release/15.19/
    - Overview — quote block: "…the next scheduled minor release date is November 12, 2026."
    - Overview — quote block: "…so it has roughly one year of community support left."
    - Overview — quote block: "It is pre-release software and not a deployment target."
    - New in 15: Regular Expression Functions — bullet 3: "…in the regexp match/split functions with invalidly encoded input (CVE-2026-14664)."
    - New in 15: Logical Replication Publications — bullet 9: "…than `pgoutput` and `test_decoding` must be allow-listed after updating (CVE-2026-6471)."
    - Security › Other security items — bullet 4: "Use `aes256` with `pgp_sym_encrypt()`."
    - Security › Other security items — bullet 5: "…including memory-safety bugs in `to_char()`, the regexp functions and `ascii()`."
    - Upgrading to 15 and Staying Current › 15.x minor releases — bullet 17: "Reindex `btree_gist` and `ltree` indexes as described in the notes."
[^3]: PostgreSQL Roadmap (next minor releases November 12, 2026, February 11, 2027, ...). https://www.postgresql.org/developer/roadmap/
    - Overview — quote block: "…the next scheduled minor release date is November 12, 2026."
[^4]: PostgreSQL 15 Released! (press release, October 13, 2022). https://www.postgresql.org/about/news/postgresql-15-released-2526/
    - Overview — paragraph 1: "…more statistics collector process), and faster in-memory and on-disk sorts."
    - What's New in PostgreSQL 15 › Server, performance and operations — bullet 1: "…faster; sorts that exceed `work_mem` use a better external merge."
[^5]: PostgreSQL 15 Beta 4 Released (September 8, 2022: "The SQL/JSON features proposed for this release have been removed"). https://www.postgresql.org/about/news/postgresql-15-beta-4-released-2507/
    - Overview — paragraph 3: "…but **were removed before 15.0** (15 Beta 4, September 2022)."
[^6]: PostgreSQL 15 Documentation: Schemas — Usage Patterns (secure schema usage, CVE-2018-1058). https://www.postgresql.org/docs/15/ddl-schemas.html
    - What's New in PostgreSQL 15 › Privileges and security — bullet 1: "`pg_upgrade` and dump/restore keep the old ACL."
    - New in 15: The public Schema, pg_database_owner and Role Changes › public schema — bullet 2: "…and grant `USAGE` (and per-object privileges) to the application role."
[^7]: PostgreSQL 15 Documentation: CREATE VIEW (security_invoker). https://www.postgresql.org/docs/15/sql-createview.html
    - What's New in PostgreSQL 15 › Privileges and security — bullet 2: "…underlying tables' privileges and RLS policies as the **calling** user."
    - New in 15: security_invoker Views — paragraph 1: "…privileges, and RLS policies are applied for the **view owner**."
[^8]: PostgreSQL 16 Release Notes (Release 16.0, September 14, 2023; CREATEROLE restrictions). https://www.postgresql.org/docs/release/16.0/
    - Do Not Use or Recommend (PostgreSQL 16+ Features) › PostgreSQL 16 (released September 14, 2023) — section heading
    - New in 15: The public Schema, pg_database_owner and Role Changes › Role administration — bullet 2: "PostgreSQL 16 restricted `CREATEROLE`."
[^9]: PostgreSQL 17 Release Notes (Release 17.0, September 26, 2024). https://www.postgresql.org/docs/release/17.0/
    - Do Not Use or Recommend (PostgreSQL 16+ Features) › PostgreSQL 17 (released September 26, 2024) — section heading
[^10]: PostgreSQL 18 Release Notes (Release 18.0, September 25, 2025) and PostgreSQL 19 Release Candidate 1 (October 15, 2026). https://www.postgresql.org/docs/release/18.0/ , https://www.postgresql.org/docs/19/release-19.html
    - Do Not Use or Recommend (PostgreSQL 16+ Features) › PostgreSQL 18 (released September 25, 2025) and 19 (RC1) — section heading
[^11]: PostgreSQL 14 Release Notes (Release 14.0). https://www.postgresql.org/docs/release/14.0/
    - Features from PostgreSQL 14 You Can Rely On — paragraph 1: "The marker comes from the 14 release notes."
[^12]: PostgreSQL 15 Documentation: MERGE. https://www.postgresql.org/docs/15/sql-merge.html
    - New in 15: MERGE — paragraph 1: "…to a target table from a source in one statement."
[^13]: PostgreSQL 15.1–15.18 Release Notes. https://www.postgresql.org/docs/release/15.1/ (and /15.2/ … /15.18/ under the same path)
    - New in 15: MERGE — bullet 8: "Run the latest 15.x."
    - Security › SECURITY DEFINER functions — paragraph 3: "…(`REFRESH MATERIALIZED VIEW CONCURRENTLY` running code as the refreshing user)."
    - Security › Row-level security — bullet 8: "…when RLS applied to a subquery, CTE or `security_invoker` view."
    - Security › Other security items — bullet 3: "Restore dumps with a matching up-to-date psql."
    - Connection Pooling and Application Access — bullet 9: "…result buffer; use normal queries instead of the fast-path interface."
    - Upgrading to 15 and Staying Current › 15.x minor releases — paragraph 1: "…only fixes and need no dump/restore, but some require actions:"
[^14]: PostgreSQL 15 Documentation: Logical Replication — Row Filters and Column Lists. https://www.postgresql.org/docs/15/logical-replication-row-filter.html , https://www.postgresql.org/docs/15/logical-replication-col-lists.html
    - New in 15: Logical Replication Publications — paragraph 1: "Rules verified on 15.19:"
[^15]: PostgreSQL 15 Documentation: Explicit Locking. https://www.postgresql.org/docs/15/explicit-locking.html
    - Schema Migrations and Locking › Lock levels of common DDL (measured on 15.19 with pg_locks) — paragraph 1: "…for lock modes: the "Explicit Locking" and `ALTER TABLE` documentation."
[^16]: PostgreSQL 15 Documentation: ALTER TABLE; CREATE INDEX (Building Indexes Concurrently). https://www.postgresql.org/docs/15/sql-altertable.html , https://www.postgresql.org/docs/15/sql-createindex.html
    - Schema Migrations and Locking › Lock levels of common DDL (measured on 15.19 with pg_locks) — paragraph 1: "…for lock modes: the "Explicit Locking" and `ALTER TABLE` documentation."
[^17]: PostgreSQL 15 Documentation: Transaction Isolation. https://www.postgresql.org/docs/15/transaction-iso.html
    - Transactions, Isolation and Concurrency › Isolation levels (behaviour verified with two sessions on 15.19) — section heading
[^18]: PostgreSQL 15.11 and 15.12 Release Notes (CVE-2025-1094). https://www.postgresql.org/docs/release/15.11/ , https://www.postgresql.org/docs/release/15.12/
    - Security › SQL injection in dynamic SQL — paragraph 2: "…input that 15.11 fixed (CVE-2025-1094), with a follow-up in 15.12."
[^19]: PostgreSQL 15 Documentation: CREATE FUNCTION — Writing SECURITY DEFINER Functions Safely; ALTER DEFAULT PRIVILEGES. https://www.postgresql.org/docs/15/sql-createfunction.html , https://www.postgresql.org/docs/15/sql-alterdefaultprivileges.html
    - Security › SECURITY DEFINER functions — paragraph 1: "FROM PUBLIC`."
[^20]: PostgreSQL 15 Documentation: Row Security Policies. https://www.postgresql.org/docs/15/ddl-rowsecurity.html
    - Security › Row-level security — paragraph 1: "Behaviour verified on 15.19:"
[^21]: PostgreSQL Wiki: Don't Do This. https://wiki.postgresql.org/wiki/Don't_Do_This
    - Data Types: Pitfalls › Timestamps — bullet 1: "Use **`timestamptz`** for points in time."
[^22]: PostgreSQL 15 Documentation: Table Partitioning. https://www.postgresql.org/docs/15/ddl-partitioning.html
    - Partitioning — paragraph 1: "Rules verified on 15.19:"
[^23]: PgBouncer configuration (max_prepared_statements, transaction pooling restrictions). https://www.pgbouncer.org/config.html
    - Connection Pooling and Application Access — bullet 2: "…locks, `LISTEN`, `WITH HOLD` cursors, temporary tables, and SQL-level `PREPARE`/`EXECUTE`."
    - Connection Pooling and Application Access — bullet 3: "SQL `PREPARE` statements are still not tracked."
