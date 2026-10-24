### Citations: PostgreSQL Style Guide

Sources for `postgresql.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: PostgreSQL Versioning Policy (supported versions, first and final release dates). https://www.postgresql.org/support/versioning/
    - Release Lines and Support (as of October 2026) — paragraph 1: "…dump/restore; some ask for follow-up actions such as a `REINDEX`."
[^2]: PostgreSQL Roadmap (next minor release November 12, 2026). https://www.postgresql.org/developer/roadmap/
    - Release Lines and Support (as of October 2026) — paragraph 1: "…dump/restore; some ask for follow-up actions such as a `REINDEX`."
[^3]: PostgreSQL 19 RC 1 Released! (October 15, 2026; RC 2 on November 12, GA planned November 19, 2026) and PostgreSQL 19 Release Notes. https://www.postgresql.org/about/news/postgresql-19-rc-1-released-3409/ , https://www.postgresql.org/docs/19/release-19.html
    - Release Lines and Support (as of October 2026) — table row 8: "…November 12, 2026 and general availability for November 19, 2026"
    - Version-aware Review › Not available in any released version — paragraph 1: "…suggest them for production code until the project runs 19:"
[^4]: PostgreSQL 18.6, 17.11, 16.15, 15.19, 14.24 and 19 Beta 3 Released (August 13, 2026). https://www.postgresql.org/about/news/postgresql-186-1711-1615-1519-1424-and-19-beta-3-released-3365/
    - Release Lines and Support (as of October 2026) — bullet 2: "…13, 2026 update releases alone fixed 28) and wrong-result bugs."
    - Upgrades and Minor Releases — bullet 6: "…and autoanalyze on that table until an `ANALYZE` resets it."
[^5]: PostgreSQL Release Notes, 9.5 through 18 (feature and incompatibility lists). https://www.postgresql.org/docs/release/
    - Version-aware Review › Feature availability by version — paragraph 1: "…in all later releases unless the removal table says otherwise."
    - Version-aware Review › Removed and changed behaviour across versions — paragraph 1: "Each item names the release that changed it."
[^6]: Explicit Locking (table-level lock modes and conflicts). https://www.postgresql.org/docs/current/explicit-locking.html
    - Schema Migrations and Locking — paragraph 1: "…held, and does the statement rewrite or scan the table?**"
[^7]: ALTER TABLE (lock levels, NOT VALID, VALIDATE CONSTRAINT, rewrites). https://www.postgresql.org/docs/current/sql-altertable.html
    - Schema Migrations and Locking — paragraph 1: "…held, and does the statement rewrite or scan the table?**"
[^8]: CREATE INDEX (Building Indexes Concurrently, ON ONLY). https://www.postgresql.org/docs/current/sql-createindex.html
    - Schema Migrations and Locking › Indexes in migrations — paragraph 1: "Rules for `CREATE INDEX CONCURRENTLY` (all verified on 18.6):"
[^9]: Date/Time Types. https://www.postgresql.org/docs/current/datatype-datetime.html
    - Data Types and Their Pitfalls › Timestamps and time zones — bullet 1: "…not instants (a shop's opening time, a birthday at midnight)."
[^10]: PostgreSQL Wiki: Don't Do This (timestamp, timetz, char(n), money, NOT IN, BETWEEN, serial). https://wiki.postgresql.org/wiki/Don%27t_Do_This
    - Data Types and Their Pitfalls › Timestamps and time zones — bullet 1: "…not instants (a shop's opening time, a birthday at midnight)."
[^11]: Using EXPLAIN. https://www.postgresql.org/docs/current/using-explain.html
    - Reading EXPLAIN — paragraph 1: "What to look for:"
[^12]: Transaction Isolation (Read Committed re-evaluation example, serialization failures). https://www.postgresql.org/docs/current/transaction-iso.html
    - Transactions, Isolation and Concurrency › Isolation levels — paragraph 1: "PostgreSQL never shows dirty reads."
    - Transactions, Isolation and Concurrency › READ COMMITTED anomalies (verified with two sessions on 18.6) — paragraph 2: "…multi-row conditions need `REPEATABLE READ`/`SERIALIZABLE` (with retries) or explicit locks."
[^13]: INSERT (ON CONFLICT). https://www.postgresql.org/docs/current/sql-insert.html
    - Transactions, Isolation and Concurrency › Upserts: `INSERT ... ON CONFLICT` vs `MERGE` — bullet 1: "…`MERGE` for set-based synchronization where conflicts are impossible or handled."
[^14]: MERGE. https://www.postgresql.org/docs/current/sql-merge.html
    - Transactions, Isolation and Concurrency › Upserts: `INSERT ... ON CONFLICT` vs `MERGE` — bullet 1: "…`MERGE` for set-based synchronization where conflicts are impossible or handled."
[^15]: Table Partitioning. https://www.postgresql.org/docs/current/ddl-partitioning.html
    - Partitioning — paragraph 1: "…do not partition small tables, and avoid thousands of partitions."
[^16]: CREATE ROLE (CREATEROLE) and Role Attributes; PostgreSQL 16 Release Notes. https://www.postgresql.org/docs/current/sql-createrole.html , https://www.postgresql.org/docs/release/16.0/
    - Security › `CREATEROLE` is powerful — paragraph 1: "…grant `CREATEDB`, `REPLICATION` or `BYPASSRLS` if it has them itself."
[^17]: Row Security Policies. https://www.postgresql.org/docs/current/ddl-rowsecurity.html
    - Security › Row-level security — paragraph 1: "Verified on 18.6:"
[^18]: CREATE FUNCTION: Writing SECURITY DEFINER Functions Safely. https://www.postgresql.org/docs/current/sql-createfunction.html
    - Security › `SECURITY DEFINER` functions and `search_path` — paragraph 1: "…resolved through the **caller's** `search_path` unless the function pins it."
[^19]: Routine Vacuuming (dead tuples, visibility map, transaction ID wraparound). https://www.postgresql.org/docs/current/routine-vacuuming.html
    - Vacuum, Bloat and Maintenance › How dead rows accumulate — paragraph 1: "Three things stop it from removing anything, cluster-wide:"
[^20]: PgBouncer features (SQL feature map for pooling modes). https://www.pgbouncer.org/features.html
    - Connection Pooling and Application Access › Pooling — paragraph 2: "…server connection, so session state does not survive between transactions:"
[^21]: PgBouncer changelog (1.21.0: prepared statements; 1.24.0: max_prepared_statements default 200). https://www.pgbouncer.org/changelog.html
    - Connection Pooling and Application Access › Pooling — paragraph 2: "…server connection, so session state does not survive between transactions:"
