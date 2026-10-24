### Citations: PostgreSQL 18: The Complete Production SQL Review Guide

Sources for `postgresql-18.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: PostgreSQL 18 Release Notes (E.6, incl. "Migration to Version 18"). https://www.postgresql.org/docs/18/release-18.html
    - PostgreSQL 18: The Complete Production SQL Review Guide — quote block: "The next scheduled minor release is November 12, 2026."
    - Version-aware review — bullet 4: "…before 1.21 cannot use protocol-level prepared statements in transaction mode."
    - Version-aware review › Deprecated in 18 (still works, flag new uses) — bullet 1: "Use SCRAM (`password_encryption = scram-sha-256`, the default since 14)."
    - Version-aware review › Deprecated in 18 (still works, flag new uses) — bullet 2: "**`ssl_ecdh_curve`** is renamed `ssl_groups` (old name still accepted)."
    - Version-aware review › Deprecated in 18 (still works, flag new uses) — bullet 3: "**`pg_recvlogical --two-phase`** is renamed `--enable-two-phase`."
    - What's new in PostgreSQL 18 (overview) — paragraph 1: "The release-note highlights, plus the incompatibilities a reviewer must know."
    - New in 18: Asynchronous I/O — paragraph 1: "…and vacuum, especially on cloud block storage with high latency."
    - New in 18: Virtual generated columns (now the default) — paragraph 1: "…(the only kind in 12–17) must now be spelled out."
    - New in 18: `OLD` and `NEW` in `RETURNING` — paragraph 1: "…`MERGE` can return the row before and after the change."
    - New in 18: Temporal constraints (`WITHOUT OVERLAPS`, `PERIOD`) — paragraph 1: "…`PERIOD`, requiring the referenced ranges to cover the referencing range."
    - New in 18: B-tree skip scan and other planner improvements › Skip scan — paragraph 1: "…has no equality condition, by "skipping" through its distinct values."
    - New in 18: B-tree skip scan and other planner improvements › Other planner changes worth knowing (verified plans on 18.6 vs 17.11) — paragraph 1: "…functions, and better `generate_series()` row estimates for `numeric` and `timestamp`."
    - New in 18: `NOT NULL` constraints in `pg_constraint`, `NOT VALID`, `NOT ENFORCED` › NOT NULL constraints are real constraints — paragraph 1: "…and can be added as `NOT VALID` and validated later."
    - New in 18: `NOT NULL` constraints in `pg_constraint`, `NOT VALID`, `NOT ENFORCED` › `NOT ENFORCED` (New in 18) — paragraph 1: "…ENFORCED`: they are recorded (`pg_constraint.conenforced = false`) but not checked."
    - New in 18: `NOT NULL` constraints in `pg_constraint`, `NOT VALID`, `NOT ENFORCED` › Foreign keys on partitioned tables — paragraph 1: "ONLY` works on partitioned tables."
    - New in 18: EXPLAIN, COPY, functions, collations › EXPLAIN — bullet 5: "`EXPLAIN (WAL)` reports full WAL buffers."
    - New in 18: EXPLAIN, COPY, functions, collations › COPY — bullet 1: "…terminator, and older `psql` clients may misbehave against 18 servers."
    - New in 18: EXPLAIN, COPY, functions, collations › New and changed functions (all verified on 18.6, absent on 17.11) — bullet 3: "PL/pgSQL accepts `=>` for named cursor arguments."
    - New in 18: EXPLAIN, COPY, functions, collations › Collations — bullet 2: "…(17 raised an error), e.g. a case-insensitive ICU collation `und-u-ks-level2`."
    - New in 18: EXPLAIN, COPY, functions, collations › Collations — bullet 4: "…`pg_upgrade` of an ICU/builtin cluster, reindex full-text and `pg_trgm` indexes."
    - New in 18: Authentication, security and operations changes — bullet 3: "Security-sensitive trigger code that used `current_user` behaves differently after upgrade."
    - New in 18: Authentication, security and operations changes — bullet 4: "…the new cluster (or enabling checksums offline with `pg_checksums` first)."
    - New in 18: Authentication, security and operations changes — bullet 6: "…gain `--statistics`, `--statistics-only`, `--no-statistics`, `--no-data`, `--no-schema`, `--no-policies`, and `pg_dump --sequence-data`."
    - New in 18: Authentication, security and operations changes — bullet 7: "`ALTER DEFAULT PRIVILEGES` for large objects."
    - New in 18: Authentication, security and operations changes — bullet 8: "…only when `max_protocol_version` allows (default still 3.0 in 18's libpq)."
    - New in 18: Authentication, security and operations changes › Other 18 changes a reviewer may meet — paragraph 1: "…settings and function names were checked with `SHOW`/`pg_proc` on 18.6:"
    - Data types and their pitfalls › Date and time — bullet 7: "…resolution: abbreviations of the session's `TimeZone` are checked before `timezone_abbreviations`."
    - Query writing and performance › Eliminate N+1 queries — paragraph 1: "…17.11 records three separate entries for the same three statements.)"
    - Query writing and performance › Materialized views — paragraph 1: "…index that supports equality) and allows reads during the refresh."
    - Partitioning — bullet 7: "`ANALYZE ONLY parent` analyzes just the parent."
    - Maintenance and observability › Vacuum and statistics — bullet 1: "…and eager freezing of all-visible pages during normal vacuums (`vacuum_max_eager_freeze_failure_rate`)."
    - Maintenance and observability › 18 monitoring changes — bullet 6: "…`DECLARE`, parameterizes `SET` values, adds `wal_buffers_full` and parallel worker columns."
    - Removed / changed in 18 — migration notes — paragraph 1: "…when moving from 17 to 18 (each verified where executable):"
[^2]: PostgreSQL Versioning Policy (supported versions, first/final release dates). https://www.postgresql.org/support/versioning/
    - PostgreSQL 18: The Complete Production SQL Review Guide — quote block: "The next scheduled minor release is November 12, 2026."
    - PostgreSQL 18: The Complete Production SQL Review Guide — quote block: "…November 12, 2026 and general availability for November 19, 2026."
[^3]: PostgreSQL 18.6, 17.11, 16.15, 15.19, 14.24 and 19 Beta 3 Released (August 13, 2026; 18.5 skipped; post-update steps). https://www.postgresql.org/about/news/postgresql-186-1711-1615-1519-1424-and-19-beta-3-released-3365/
    - PostgreSQL 18: The Complete Production SQL Review Guide — quote block: "The next scheduled minor release is November 12, 2026."
    - Version-aware review — bullet 3: "…introduces configuration changes (see "Changes inside the 18.x series" below)."
[^4]: PostgreSQL 19 Release Notes (RC 1, October 15, 2026). https://www.postgresql.org/docs/19/release-19.html
    - PostgreSQL 18: The Complete Production SQL Review Guide — quote block: "…November 12, 2026 and general availability for November 19, 2026."
    - Version-aware review › Do not use or recommend (PostgreSQL 19 and later, or never released) — paragraph 1: "…18.6 (verified) — never suggest it for an 18 codebase:"
    - Version-aware review › Do not use or recommend (PostgreSQL 19 and later, or never released) — paragraph 2: "…to `off` (all verified on 18.6; each changes in 19)."
[^5]: PostgreSQL Roadmap (minor release schedule, 19 release plan). https://www.postgresql.org/developer/roadmap/
    - PostgreSQL 18: The Complete Production SQL Review Guide — quote block: "The next scheduled minor release is November 12, 2026."
    - PostgreSQL 18: The Complete Production SQL Review Guide — quote block: "…November 12, 2026 and general availability for November 19, 2026."
[^6]: PostgreSQL 18 Released! (September 25, 2025). https://www.postgresql.org/about/news/postgresql-18-released-3142/
    - What's new in PostgreSQL 18 (overview) — paragraph 1: "The release-note highlights, plus the incompatibilities a reviewer must know."
[^7]: UUID Functions (uuidv7, uuidv4, uuid_extract_timestamp). https://www.postgresql.org/docs/18/functions-uuid.html
    - New in 18: `uuidv7()` and UUID functions — paragraph 1: "…sub-millisecond fraction in the high bits, followed by random bits."
[^8]: Generated Columns. https://www.postgresql.org/docs/18/ddl-generated-columns.html
    - New in 18: Virtual generated columns (now the default) — paragraph 1: "…(the only kind in 12–17) must now be spelled out."
    - New in 18: Virtual generated columns (now the default) — paragraph 4: "…this is only secure if the expression uses leakproof functions."
[^9]: Returning Data from Modified Rows (OLD/NEW). https://www.postgresql.org/docs/18/dml-returning.html
    - New in 18: `OLD` and `NEW` in `RETURNING` — paragraph 1: "…`MERGE` can return the row before and after the change."
[^10]: CREATE TABLE (WITHOUT OVERLAPS, PERIOD, NOT ENFORCED, VIRTUAL). https://www.postgresql.org/docs/18/sql-createtable.html
    - New in 18: Temporal constraints (`WITHOUT OVERLAPS`, `PERIOD`) — paragraph 1: "…`PERIOD`, requiring the referenced ranges to cover the referencing range."
[^11]: ALTER TABLE (lock levels, NOT VALID, VALIDATE CONSTRAINT). https://www.postgresql.org/docs/18/sql-altertable.html
    - New in 18: `NOT NULL` constraints in `pg_constraint`, `NOT VALID`, `NOT ENFORCED` › NOT NULL constraints are real constraints — paragraph 1: "…and can be added as `NOT VALID` and validated later."
    - Schema migrations and locking safety — paragraph 1: "…EXCLUSIVE`, which conflicts with **every** other lock including plain `SELECT`."
[^12]: Explicit Locking (table- and row-level lock modes). https://www.postgresql.org/docs/18/explicit-locking.html
    - Schema migrations and locking safety — paragraph 1: "…EXCLUSIVE`, which conflicts with **every** other lock including plain `SELECT`."
[^13]: Resource Consumption — Asynchronous I/O (io_method, io_workers, io_combine_limit, effective_io_concurrency). https://www.postgresql.org/docs/18/runtime-config-resource.html
    - New in 18: Asynchronous I/O — paragraph 1: "…and vacuum, especially on cloud block storage with high latency."
[^14]: Multicolumn Indexes (skip scan). https://www.postgresql.org/docs/18/indexes-multicolumn.html
    - New in 18: B-tree skip scan and other planner improvements › Skip scan — paragraph 1: "…has no equality condition, by "skipping" through its distinct values."
[^15]: pg_upgrade (statistics transfer, --swap, post-upgrade vacuumdb). https://www.postgresql.org/docs/18/pgupgrade.html
    - New in 18: Authentication, security and operations changes — bullet 5: "…unusable once file transfer starts) and parallel checks via `--jobs`."
    - Removed / changed in 18 — migration notes — paragraph 2: "…and `vacuumdb --all --analyze-only`, and recreate extended statistics with `ANALYZE`."
[^16]: OAuth Authorization/Authentication. https://www.postgresql.org/docs/18/auth-oauth.html
    - New in 18: Authentication, security and operations changes — bullet 1: "…the validator — review it like code that grants superuser."
[^17]: Password Authentication (scram-sha-256, md5 deprecation). https://www.postgresql.org/docs/18/auth-password.html
    - Version-aware review › Deprecated in 18 (still works, flag new uses) — bullet 1: "Use SCRAM (`password_encryption = scram-sha-256`, the default since 14)."
    - New in 18: Authentication, security and operations changes — bullet 2: "…so migrate passwords first, then change the method to `scram-sha-256`."
[^18]: CREATE FUNCTION — Writing SECURITY DEFINER Functions Safely. https://www.postgresql.org/docs/18/sql-createfunction.html
    - Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL › `SECURITY DEFINER` functions and `search_path` — paragraph 1: "…resolved through the **caller's** `search_path` unless the function pins it."
[^19]: Row Security Policies. https://www.postgresql.org/docs/18/ddl-rowsecurity.html
    - Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL › Row-level security — bullet 5: "…evaluated before the policy filter and leak values through errors."
[^20]: PL/pgSQL — Executing Dynamic Commands. https://www.postgresql.org/docs/18/plpgsql-statements.html
    - Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL › SQL injection in dynamic SQL — bullet 5: "…cannot be parameters; map user input to a fixed allow-list."
[^21]: Transaction Isolation. https://www.postgresql.org/docs/18/transaction-iso.html
    - Transactions, isolation and concurrency — paragraph 1: "…**READ COMMITTED**: each statement sees data committed before it started."
[^22]: INSERT (ON CONFLICT). https://www.postgresql.org/docs/18/sql-insert.html
    - Transactions, isolation and concurrency › Upsert: `INSERT ... ON CONFLICT` vs `MERGE` — bullet 1: "…(15+) for set-based synchronization where conflicts are handled or impossible."
[^23]: MERGE. https://www.postgresql.org/docs/18/sql-merge.html
    - Transactions, isolation and concurrency › Upsert: `INSERT ... ON CONFLICT` vs `MERGE` — bullet 1: "…(15+) for set-based synchronization where conflicts are handled or impossible."
[^24]: JSON Functions and Operators (SQL/JSON query functions, JSON_TABLE). https://www.postgresql.org/docs/18/functions-json.html
    - (not cited in the guide)
[^25]: JSON Types (jsonb indexing, operator classes). https://www.postgresql.org/docs/18/datatype-json.html
    - JSONB and SQL/JSON — paragraph 1: "…input text (key order, duplicate keys, whitespace) must be preserved."
[^26]: Table Partitioning. https://www.postgresql.org/docs/18/ddl-partitioning.html
    - Partitioning — paragraph 1: "…adds planning overhead and constraints; do not partition small tables."
[^27]: CREATE INDEX (CONCURRENTLY, partitioned tables, ON ONLY). https://www.postgresql.org/docs/18/sql-createindex.html
    - Indexing — paragraph 1: "…(huge append-only tables whose physical order correlates with the column)."
[^28]: Using EXPLAIN. https://www.postgresql.org/docs/18/using-explain.html
    - Reading EXPLAIN — paragraph 2: "What to look for:"
[^29]: PgBouncer — SQL feature map for pooling modes. https://www.pgbouncer.org/features.html
    - Application code: parameters, pooling, timeouts › Connection pooling — paragraph 2: "…transactions of one client can run on different server connections:"
[^30]: PgBouncer changelog (1.21.0 prepared statements in transaction mode; 1.24.0 max_prepared_statements default 200). https://www.pgbouncer.org/changelog.html
    - Version-aware review — bullet 4: "…before 1.21 cannot use protocol-level prepared statements in transaction mode."
    - Application code: parameters, pooling, timeouts › Connection pooling — paragraph 2: "…transactions of one client can run on different server connections:"
[^31]: PostgreSQL 14, 15, 16 and 17 Release Notes. https://www.postgresql.org/docs/14/release-14.html , https://www.postgresql.org/docs/15/release-15.html , https://www.postgresql.org/docs/16/release-16.html , https://www.postgresql.org/docs/17/release-17.html
    - Features from 14–17 that a PostgreSQL 18 codebase can use — paragraph 1: "The remaining items are taken from the release notes."
[^32]: PostgreSQL Wiki — Don't Do This (timestamp, timetz, char(n), money, NOT IN, BETWEEN). https://wiki.postgresql.org/wiki/Don%27t_Do_This
    - Data types and their pitfalls › `numeric` vs floating point vs `money` — bullet 2: "…input depend on `lc_monetary`, and it has fixed fractional precision."
    - Data types and their pitfalls › Text types — bullet 2: "…spaces, strips them in comparisons and `length()`, and wastes space."
    - Data types and their pitfalls › Date and time — bullet 1: "…stores whatever wall-clock value the client sent, losing the offset."
    - Data types and their pitfalls › Date and time — bullet 6: "…zone): an offset without a date is meaningless across DST."
[^33]: Schemas — The Public Schema and Usage Patterns. https://www.postgresql.org/docs/18/ddl-schemas.html
    - Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL › Roles and privileges — bullet 2: "…`\dn+ public` and `REVOKE CREATE ON SCHEMA public FROM PUBLIC`."
[^34]: COPY (ON_ERROR, REJECT_LIMIT, LOG_VERBOSITY, CSV end-of-data). https://www.postgresql.org/docs/18/sql-copy.html
    - New in 18: EXPLAIN, COPY, functions, collations › COPY — bullet 1: "…terminator, and older `psql` clients may misbehave against 18 servers."
[^35]: The Cumulative Statistics System (pg_stat_io, pg_stat_user_tables, pg_stat_user_indexes). https://www.postgresql.org/docs/18/monitoring-stats.html
    - Maintenance and observability › 18 monitoring changes — bullet 6: "…`DECLARE`, parameterizes `SET` values, adds `wal_buffers_full` and parallel worker columns."
[^36]: Routine Vacuuming (autovacuum, partitioned tables, wraparound). https://www.postgresql.org/docs/18/routine-vacuuming.html
    - Partitioning — bullet 7: "`ANALYZE ONLY parent` analyzes just the parent."
    - Maintenance and observability › Vacuum and statistics — bullet 1: "…and eager freezing of all-visible pages during normal vacuums (`vacuum_max_eager_freeze_failure_rate`)."
[^37]: initdb (--data-checksums default, --no-data-checksums). https://www.postgresql.org/docs/18/app-initdb.html
    - New in 18: Authentication, security and operations changes — bullet 4: "…the new cluster (or enabling checksums offline with `pg_checksums` first)."
[^38]: Collation Support (builtin providers, nondeterministic collations). https://www.postgresql.org/docs/18/collation.html
    - New in 18: EXPLAIN, COPY, functions, collations › Collations — bullet 2: "…(17 raised an error), e.g. a case-insensitive ICU collation `und-u-ks-level2`."
[^39]: Date/Time Types. https://www.postgresql.org/docs/18/datatype-datetime.html
    - Data types and their pitfalls › Date and time — bullet 1: "…stores whatever wall-clock value the client sent, losing the offset."
[^40]: pg_stat_statements. https://www.postgresql.org/docs/18/pgstatstatements.html
    - Query writing and performance › Eliminate N+1 queries — paragraph 1: "…17.11 records three separate entries for the same three statements.)"
    - Maintenance and observability › 18 monitoring changes — bullet 6: "…`DECLARE`, parameterizes `SET` values, adds `wal_buffers_full` and parallel worker columns."
[^41]: CREATE VIEW (security_invoker, security_barrier). https://www.postgresql.org/docs/18/sql-createview.html
    - Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL › Row-level security — bullet 3: "…`security_invoker` or owned by a role subject to the policies."
[^42]: libpq — Connection Strings (sslmode, min/max_protocol_version, sslkeylogfile). https://www.postgresql.org/docs/18/libpq-connect.html
    - Application code: parameters, pooling, timeouts › TLS and connection strings — paragraph 1: "…poolers and proxies must support it before you enable it."
[^43]: PostgreSQL 19 RC 1 Released! (October 15, 2026; RC 2 on November 12, GA planned November 19, 2026). https://www.postgresql.org/about/news/postgresql-19-rc-1-released-3409/
    - PostgreSQL 18: The Complete Production SQL Review Guide — quote block: "…November 12, 2026 and general availability for November 19, 2026."
[^44]: PostgreSQL 18.1, 18.2, 18.3, 18.4 and 18.6 Release Notes. https://www.postgresql.org/docs/18/release-18-1.html , https://www.postgresql.org/docs/18/release-18-2.html , https://www.postgresql.org/docs/18/release-18-3.html , https://www.postgresql.org/docs/18/release-18-4.html , https://www.postgresql.org/docs/18/release-18-6.html
    - Version-aware review — bullet 3: "…introduces configuration changes (see "Changes inside the 18.x series" below)."
    - Version-aware review › Changes inside the 18.x series (minor releases) — paragraph 1: "…18.x fixes change behaviour that code or operations can observe:"
    - New in 18: Virtual generated columns (now the default) — paragraph 5: "…so code relying on virtual columns needs at least 18.6."
    - New in 18: `OLD` and `NEW` in `RETURNING` — bullet 5: "Code using `RETURNING old` needs 18.6."
