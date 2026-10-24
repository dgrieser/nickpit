### Citations: PostgreSQL 19: The Complete Production SQL Review Guide

Sources for `postgresql-19.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: PostgreSQL 19 Release Notes (E.1, draft "as of 2026-10-09", incl. "Migration to Version 19"). https://www.postgresql.org/docs/19/release-19.html
    - PostgreSQL 19: The Complete Production SQL Review Guide — quote block: "…still land before GA, so RC behaviour may still change."
    - Version-aware review — bullet 5: "…restore correctly into 19; use 19's `pg_dump`/`pg_dumpall` for the upgrade."
    - Version-aware review › Deprecated in 19 (still works, flag new uses) — bullet 1: "Both still run on 19rc1."
    - Version-aware review › Deprecated in 19 (still works, flag new uses) — bullet 2: "…exists since 18, where it only covers setting a password)."
    - Version-aware review › Deprecated in 19 (still works, flag new uses) — bullet 3: "**`pg_verifybackup --wal-directory`** is deprecated in favour of `--wal-path`."
    - Version-aware review › Deprecated in 19 (still works, flag new uses) — bullet 4: "**`pg_buffercache_numa_pages()`** remains only for backward compatibility; use `pg_buffercache_os_pages()`."
    - What's new in PostgreSQL 19 (overview) — paragraph 1: "Release-note highlights plus the incompatibilities a reviewer must know."
    - What's new in PostgreSQL 19 (overview) — paragraph 2: "…release-note items that are not covered in their own section:"
    - New in 19: `REPACK` (replaces `VACUUM FULL` / `CLUSTER`) — paragraph 1: "`VACUUM FULL` and `CLUSTER` still work but are superseded."
    - New in 19: `INSERT ... ON CONFLICT DO SELECT` — paragraph 1: "…nothing or updating it — an atomic "get or create"."
    - New in 19: `IGNORE NULLS` in window functions — paragraph 1: "…accept the SQL-standard `IGNORE NULLS` / `RESPECT NULLS` (default) clause."
    - New in 19: `COPY` changes › Multi-line headers, partitioned tables, `WHERE` restrictions — bullet 4: "…uses SIMD and is faster; nothing to change in SQL."
    - New in 19: Replication — `WAIT FOR LSN`, logical decoding without restart, sequences, `EXCEPT` › Logical decoding with `wal_level = replica` — paragraph 1: "…slot still retains WAL (`idle_replication_slot_timeout`, 18, default `0` = off)."
    - New in 19: Replication — `WAIT FOR LSN`, logical decoding without restart, sequences, `EXCEPT` › Other replication changes — bullet 1: "…failover" gap — only at those refresh points, not continuously."
    - New in 19: Replication — `WAIT FOR LSN`, logical decoding without restart, sequences, `EXCEPT` › Other replication changes — bullet 3: "`wal_receiver_timeout` can be set per subscription/role."
    - New in 19: Replication — `WAIT FOR LSN`, logical decoding without restart, sequences, `EXCEPT` › Other replication changes — bullet 6: "`pg_sync_replication_slots()` now waits for synchronization to complete."
    - New in 19: Data types and functions › Text search — paragraph 1: "…configuration on such a dictionary to keep the old stems."
    - New in 19: Constraints, DDL and utility commands › Other DDL and utility changes (verified) — bullet 5: "CONNECTION` (extension authors)."
    - New in 19: Planner, `EXPLAIN`, plan advice › Optimizer improvements (verified plans, 19rc1 vs 18.6) — bullet 3: "…startup costs for partial paths, faster foreign-key checks, radix sort."
    - New in 19: Planner, `EXPLAIN`, plan advice › `EXPLAIN (ANALYZE, IO)` — bullet 1: "`auto_explain.log_io` adds it to `auto_explain`."
    - New in 19: I/O, autovacuum and changed defaults › Asynchronous I/O (18) in 19 — paragraph 1: "…bulk deletion, GIN vacuuming, `bloom` and `pgstattuple` use streaming reads."
    - New in 19: I/O, autovacuum and changed defaults › Autovacuum — bullet 4: "`ANALYZE ONLY public.pt;` appears; 18.6's `--echo` output had no parent)."
    - New in 19: Security, authentication and string literals › `standard_conforming_strings` is always `on` — paragraph 1: "Client libraries still support old servers with the setting off."
    - New in 19: Security, authentication and string literals › Authentication — bullet 4: "…details; libpq gains `oauth_ca_file` (`PGOAUTHCAFILE`) and a v2 bearer-token hook."
    - New in 19: Security, authentication and string literals › Authentication — bullet 6: "…tablespace names, and `pg_upgrade` refuses clusters that contain such names."
    - New in 19: Security, authentication and string literals › `postgres_fdw` sessions inherit `READ ONLY` / `DEFERRABLE` — paragraph 1: "…on the remote server break; that is the intended fix."
    - New in 19: Security, authentication and string literals › `postgres_fdw` sessions inherit `READ ONLY` / `DEFERRABLE` — paragraph 2: "…of sampling rows; it runs as the foreign table owner."
    - Maintenance and observability › 19 monitoring changes (verified in the 19rc1 catalog) — bullet 6: "`generic_plan_calls`, `custom_plan_calls`."
    - Removed / changed in 19 — migration notes from 18 — paragraph 1: "…18 to 19 (verified on 19rc1 vs 18.6 where executable):"
    - Removed / changed in 19 — migration notes from 18 — paragraph 2: "`pg_stat_statements` 1.13, `btree_gist` 1.9, `postgres_fdw` 1.3 ship with 19rc1)."
[^2]: PostgreSQL Versioning Policy (supported versions, current minors, final release dates). https://www.postgresql.org/support/versioning/
    - PostgreSQL 19: The Complete Production SQL Review Guide — quote block: "…and 14 receives its final release on November 12, 2026."
[^3]: PostgreSQL 19 RC 1 Released! (October 15, 2026; RC 2 on November 12, GA planned November 19, 2026; changes since Beta 4). https://www.postgresql.org/about/news/postgresql-19-rc-1-released-3409/
    - PostgreSQL 19: The Complete Production SQL Review Guide — quote block: "…still land before GA, so RC behaviour may still change."
    - Version-aware review — bullet 1: "…automatic logical-replication enablement and parallel autovacuum during the beta period)."
    - New in 19: Security, authentication and string literals › `postgres_fdw` sessions inherit `READ ONLY` / `DEFERRABLE` — paragraph 2: "…of sampling rows; it runs as the foreign table owner."
    - Removed / changed in 19 — migration notes from 18 — paragraph 3: "…not put data you cannot reload on a pre-release build."
[^4]: PostgreSQL Roadmap (minor release schedule: November 12, 2026; February 11, May 13, August 12, 2027). https://www.postgresql.org/developer/roadmap/
    - PostgreSQL 19: The Complete Production SQL Review Guide — quote block: "…and 14 receives its final release on November 12, 2026."
[^5]: PostgreSQL 18.6, 17.11, 16.15, 15.19, 14.24 and 19 Beta 3 Released (August 13, 2026). https://www.postgresql.org/about/news/postgresql-186-1711-1615-1519-1424-and-19-beta-3-released-3365/
    - PostgreSQL 19: The Complete Production SQL Review Guide — quote block: "…and 14 receives its final release on November 12, 2026."
[^6]: REPACK. https://www.postgresql.org/docs/19/sql-repack.html
    - Version-aware review › Deprecated in 19 (still works, flag new uses) — bullet 1: "Both still run on 19rc1."
    - New in 19: `REPACK` (replaces `VACUUM FULL` / `CLUSTER`) — paragraph 1: "`VACUUM FULL` and `CLUSTER` still work but are superseded."
    - New in 19: `REPACK` (replaces `VACUUM FULL` / `CLUSTER`) — bullet 4: "…system catalogs, TOAST tables, non-heap access methods and `user_catalog_table` tables."
    - New in 19: `REPACK` (replaces `VACUUM FULL` / `CLUSTER`) — bullet 5: "…while long `REPEATABLE READ`/`SERIALIZABLE` transactions may still read the table."
    - New in 19: `REPACK` (replaces `VACUUM FULL` / `CLUSTER`) — bullet 7: "…than about 104 million rows are updated or deleted concurrently."
    - New in 19: `REPACK` (replaces `VACUUM FULL` / `CLUSTER`) — bullet 10: "…purposes; it is not a substitute for `VACUUM` against wraparound."
[^7]: INSERT (ON CONFLICT DO SELECT / DO UPDATE). https://www.postgresql.org/docs/19/sql-insert.html
    - New in 19: `INSERT ... ON CONFLICT DO SELECT` — paragraph 1: "…nothing or updating it — an atomic "get or create"."
    - New in 19: `INSERT ... ON CONFLICT DO SELECT` — bullet 8: "`FOR UPDATE/SHARE` additionally needs `UPDATE` on at least one column."
    - Transactions, isolation and concurrency › Upsert: `INSERT ... ON CONFLICT` vs `MERGE` — bullet 1: "…(15+) for set-based synchronization where conflicts are handled or impossible."
[^8]: WAIT FOR. https://www.postgresql.org/docs/19/sql-wait.html
    - New in 19: Replication — `WAIT FOR LSN`, logical decoding without restart, sequences, `EXCEPT` › Read-your-writes on standbys: `WAIT FOR LSN` — paragraph 1: "…'standby_replay'` (default), `'standby_write'`, `'standby_flush'` and `'primary_flush'`, `TIMEOUT '...'` and `NO_THROW`."
    - New in 19: Replication — `WAIT FOR LSN`, logical decoding without restart, sequences, `EXCEPT` › Read-your-writes on standbys: `WAIT FOR LSN` — bullet 3: "…it **first**, outside a transaction or as the first statement."
    - New in 19: Replication — `WAIT FOR LSN`, logical decoding without restart, sequences, `EXCEPT` › Read-your-writes on standbys: `WAIT FOR LSN` — bullet 5: "…failover the status `not in recovery` means "re-evaluate", not "done"."
[^9]: COPY (FORMAT json, FORCE_ARRAY, ON_ERROR set_null, HEADER). https://www.postgresql.org/docs/19/sql-copy.html
    - New in 19: `COPY` changes › JSON output — bullet 2: "(FORMAT json)`."
    - New in 19: `COPY` changes › `ON_ERROR set_null` — bullet 4: "Both `ignore` and `set_null` are for `text`/`csv` formats only."
    - New in 19: `COPY` changes › Multi-line headers, partitioned tables, `WHERE` restrictions — bullet 2: "…TO` still means `ONLY t` (inheritance children are not included)."
[^10]: Window Functions (IGNORE NULLS / RESPECT NULLS). https://www.postgresql.org/docs/19/functions-window.html
    - New in 19: `IGNORE NULLS` in window functions — paragraph 1: "…accept the SQL-standard `IGNORE NULLS` / `RESPECT NULLS` (default) clause."
[^11]: pg_plan_advice — help the planner get the right plan. https://www.postgresql.org/docs/19/pgplanadvice.html
    - New in 19: Planner, `EXPLAIN`, plan advice › `pg_plan_advice` and `pg_stash_advice` (contrib) — paragraph 1: "…loaded with `LOAD 'pg_plan_advice'`, `session_preload_libraries` or `shared_preload_libraries` (no `CREATE EXTENSION`)."
    - New in 19: Planner, `EXPLAIN`, plan advice › `pg_plan_advice` and `pg_stash_advice` (contrib) — bullet 1: "`pg_plan_advice.feedback_warnings = on` (default `off`) turns unenforced advice into warnings."
    - New in 19: Planner, `EXPLAIN`, plan advice › `pg_plan_advice` and `pg_stash_advice` (contrib) — bullet 3: "Advice cannot control aggregation strategy or set operations (documented limitation)."
[^12]: pg_stash_advice — store and automatically apply plan advice. https://www.postgresql.org/docs/19/pgstashadvice.html
    - New in 19: Planner, `EXPLAIN`, plan advice › `pg_plan_advice` and `pg_stash_advice` (contrib) — bullet 4: "…advice, so do not embed sensitive values in advice strings."
[^13]: EXPLAIN. https://www.postgresql.org/docs/19/sql-explain.html
    - New in 19: Planner, `EXPLAIN`, plan advice › `EXPLAIN (ANALYZE, IO)` — bullet 1: "`auto_explain.log_io` adds it to `auto_explain`."
    - New in 19: Planner, `EXPLAIN`, plan advice › `EXPLAIN (ANALYZE, IO)` — bullet 2: "…read-ahead did not keep up (check `effective_io_concurrency`, `io_max_workers`, storage latency)."
[^14]: ALTER TABLE (lock levels, NOT VALID, ALTER CONSTRAINT ... ENFORCED). https://www.postgresql.org/docs/19/sql-altertable.html
    - Features from 18 that a PostgreSQL 19 codebase uses › `NOT NULL ... NOT VALID`, `NOT ENFORCED` (18) — bullet 2: "Re-enforcing scans the table under `ACCESS EXCLUSIVE`."
    - Schema migrations and locking safety — paragraph 1: "…EXCLUSIVE`, which conflicts with **every** other lock including plain `SELECT`."
[^15]: Connections and Authentication (password_expiration_warning_threshold, md5_password_warnings). https://www.postgresql.org/docs/19/runtime-config-connection.html
    - Version-aware review › Deprecated in 19 (still works, flag new uses) — bullet 2: "…exists since 18, where it only covers setting a password)."
    - New in 19: Security, authentication and string literals › Authentication — bullet 3: "`0` disables it."
[^16]: Resource Consumption (io_method, io_min_workers, io_max_workers, autovacuum workers). https://www.postgresql.org/docs/19/runtime-config-resource.html
    - New in 19: I/O, autovacuum and changed defaults › Asynchronous I/O (18) in 19 — paragraph 1: "…bulk deletion, GIN vacuuming, `bloom` and `pgstattuple` use streaming reads."
[^17]: Write Ahead Log (wal_level, effective_wal_level). https://www.postgresql.org/docs/19/runtime-config-wal.html
    - New in 19: Replication — `WAIT FOR LSN`, logical decoding without restart, sequences, `EXCEPT` › Logical decoding with `wal_level = replica` — paragraph 1: "…slot still retains WAL (`idle_replication_slot_timeout`, 18, default `0` = off)."
[^18]: CREATE PUBLICATION (ALL SEQUENCES, ALL TABLES EXCEPT). https://www.postgresql.org/docs/19/sql-createpublication.html
    - New in 19: Replication — `WAIT FOR LSN`, logical decoding without restart, sequences, `EXCEPT` › Other replication changes — bullet 1: "…failover" gap — only at those refresh points, not continuously."
[^19]: JSON Functions and Operators (jsonpath string methods, SQL/JSON). https://www.postgresql.org/docs/19/functions-json.html
    - New in 19: Data types and functions › jsonpath string methods — paragraph 1: "…"string")` or pass `silent => true` where data is heterogeneous."
[^20]: Binary String Functions and Operators (encode/decode base64url, base32hex). https://www.postgresql.org/docs/19/functions-binarystring.html
    - New in 19: Data types and functions — bullet 2: "There is **no `'base32'`** encoding — only `base32hex`"
[^21]: Range/Multirange Functions and Operators (range_minus_multi, multirange_minus_multi). https://www.postgresql.org/docs/19/functions-range.html
    - New in 19: Data types and functions — bullet 5: "…a set of ranges instead (zero rows when nothing remains)."
[^22]: pg_stat_statements. https://www.postgresql.org/docs/19/pgstatstatements.html
    - New in 19: Planner, `EXPLAIN`, plan advice › `pg_stat_statements` 1.13 — paragraph 1: "Run `ALTER EXTENSION pg_stat_statements UPDATE` after `pg_upgrade`."
    - Query writing and performance › Eliminate N+1 queries — paragraph 1: "*/)` with 3 calls.)"
[^23]: The Cumulative Statistics System. https://www.postgresql.org/docs/19/monitoring-stats.html
    - Maintenance and observability › 19 monitoring changes (verified in the 19rc1 catalog) — bullet 6: "`generic_plan_calls`, `custom_plan_calls`."
[^24]: PostgreSQL 20devel documentation (built 2026-10-21 from master; release notes placeholder, index, JSON_TABLE PLAN, toast_value_type). https://www.postgresql.org/docs/devel/release-20.html , https://www.postgresql.org/docs/devel/bookindex.html , https://www.postgresql.org/docs/devel/functions-json.html , https://www.postgresql.org/docs/devel/sql-createtable.html
    - Version-aware review › Do not use or recommend (PostgreSQL 20 development, or never committed) — paragraph 1: "Never suggest these for a 19 codebase:"
[^25]: PostgreSQL 18 Release Notes. https://www.postgresql.org/docs/18/release-18.html
    - Features from 18 that a PostgreSQL 19 codebase uses — paragraph 1: "…available in 19 unchanged unless noted (behaviour re-verified on 19rc1)."
    - Data types and their pitfalls › Date and time — bullet 7: "…changed: abbreviations of the session's `TimeZone` are checked before `timezone_abbreviations`."
    - Query writing and performance › Eliminate N+1 queries — paragraph 1: "*/)` with 3 calls.)"
    - Query writing and performance › Materialized views — paragraph 1: "…index that supports equality) and allows reads during the refresh."
    - Partitioning — bullet 7: "…`vacuumdb --analyze-only`/`--analyze-in-stages` also issue `ANALYZE ONLY parent` for partitioned tables."
    - Maintenance and observability › Vacuum and statistics — bullet 1: "…and eager freezing of all-visible pages during normal vacuums (`vacuum_max_eager_freeze_failure_rate`)."
[^26]: Explicit Locking (table- and row-level lock modes). https://www.postgresql.org/docs/19/explicit-locking.html
    - Schema migrations and locking safety — paragraph 1: "…EXCLUSIVE`, which conflicts with **every** other lock including plain `SELECT`."
[^27]: CREATE FUNCTION — Writing SECURITY DEFINER Functions Safely. https://www.postgresql.org/docs/19/sql-createfunction.html
    - Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL › `SECURITY DEFINER` functions and `search_path` — paragraph 1: "…resolved through the **caller's** `search_path` unless the function pins it."
[^28]: Row Security Policies. https://www.postgresql.org/docs/19/ddl-rowsecurity.html
    - Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL › Row-level security — bullet 5: "…evaluated before the policy filter and leak values through errors."
[^29]: PL/pgSQL — Executing Dynamic Commands. https://www.postgresql.org/docs/19/plpgsql-statements.html
    - Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL › SQL injection in dynamic SQL — bullet 5: "…cannot be parameters; map user input to a fixed allow-list."
[^30]: Transaction Isolation. https://www.postgresql.org/docs/19/transaction-iso.html
    - Transactions, isolation and concurrency — paragraph 1: "…**READ COMMITTED**: each statement sees data committed before it started."
[^31]: MERGE. https://www.postgresql.org/docs/19/sql-merge.html
    - Transactions, isolation and concurrency › Upsert: `INSERT ... ON CONFLICT` vs `MERGE` — bullet 1: "…(15+) for set-based synchronization where conflicts are handled or impossible."
[^32]: JSON Types (jsonb indexing, operator classes). https://www.postgresql.org/docs/19/datatype-json.html
    - JSONB and SQL/JSON — paragraph 1: "…input text (key order, duplicate keys, whitespace) must be preserved."
[^33]: Table Partitioning. https://www.postgresql.org/docs/19/ddl-partitioning.html
    - Partitioning — paragraph 1: "…adds planning overhead and constraints; do not partition small tables."
[^34]: CREATE INDEX (CONCURRENTLY, partitioned tables, ON ONLY). https://www.postgresql.org/docs/19/sql-createindex.html
    - Indexing — paragraph 1: "…(huge append-only tables whose physical order correlates with the column)."
[^35]: Using EXPLAIN. https://www.postgresql.org/docs/19/using-explain.html
    - Reading EXPLAIN — paragraph 2: "What to look for:"
[^36]: PgBouncer — SQL feature map for pooling modes. https://www.pgbouncer.org/features.html
    - Application code: parameters, pooling, timeouts › Connection pooling — paragraph 2: "…transactions of one client can run on different server connections:"
[^37]: PgBouncer changelog (1.21.0 prepared statements in transaction mode; 1.24.0 max_prepared_statements default 200). https://www.pgbouncer.org/changelog.html
    - Application code: parameters, pooling, timeouts › Connection pooling — paragraph 2: "…transactions of one client can run on different server connections:"
[^38]: PostgreSQL 14, 15, 16 and 17 Release Notes. https://www.postgresql.org/docs/14/release-14.html , https://www.postgresql.org/docs/15/release-15.html , https://www.postgresql.org/docs/16/release-16.html , https://www.postgresql.org/docs/17/release-17.html
    - Features from 14–17 that a PostgreSQL 19 codebase can use — paragraph 1: "RETURNING`, `transaction_timeout`) were executed on 19rc1."
[^39]: PostgreSQL Wiki — Don't Do This (timestamp, timetz, char(n), money, NOT IN, BETWEEN). https://wiki.postgresql.org/wiki/Don%27t_Do_This
    - Data types and their pitfalls › `numeric` vs floating point vs `money` — bullet 2: "…input depend on `lc_monetary`, and it has fixed fractional precision."
    - Data types and their pitfalls › Text types — bullet 2: "…spaces, strips them in comparisons and `length()`, and wastes space."
    - Data types and their pitfalls › Date and time — bullet 1: "…stores whatever wall-clock value the client sent, losing the offset."
    - Data types and their pitfalls › Date and time — bullet 6: "…zone): an offset without a date is meaningless across DST."
[^40]: Schemas — The Public Schema and Usage Patterns. https://www.postgresql.org/docs/19/ddl-schemas.html
    - Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL › Roles and privileges — bullet 2: "…`\dn+ public` and `REVOKE CREATE ON SCHEMA public FROM PUBLIC`."
[^41]: Routine Vacuuming (autovacuum, partitioned tables, wraparound). https://www.postgresql.org/docs/19/routine-vacuuming.html
    - Partitioning — bullet 7: "…`vacuumdb --analyze-only`/`--analyze-in-stages` also issue `ANALYZE ONLY parent` for partitioned tables."
    - Maintenance and observability › Vacuum and statistics — bullet 1: "…and eager freezing of all-visible pages during normal vacuums (`vacuum_max_eager_freeze_failure_rate`)."
[^42]: Date/Time Types. https://www.postgresql.org/docs/19/datatype-datetime.html
    - Data types and their pitfalls › Date and time — bullet 1: "…stores whatever wall-clock value the client sent, losing the offset."
[^43]: CREATE VIEW (security_invoker, security_barrier). https://www.postgresql.org/docs/19/sql-createview.html
    - Security: roles, privileges, RLS, `SECURITY DEFINER`, dynamic SQL › Row-level security — bullet 3: "…`security_invoker` or owned by a role subject to the policies."
[^44]: libpq — Connection Strings (sslmode, min/max_protocol_version, sslkeylogfile, servicefile). https://www.postgresql.org/docs/19/libpq-connect.html
    - Application code: parameters, pooling, timeouts › TLS and connection strings — paragraph 1: "…you enable it. 19's libpq adds the `servicefile` connection parameter."
[^45]: UUID Functions (uuidv7, uuidv4, uuid_extract_timestamp). https://www.postgresql.org/docs/19/functions-uuid.html
    - Features from 18 that a PostgreSQL 19 codebase uses › `uuidv7()` (18) — section heading
[^46]: Generated Columns. https://www.postgresql.org/docs/19/ddl-generated-columns.html
    - Features from 18 that a PostgreSQL 19 codebase uses › Virtual generated columns are the default (18) — paragraph 1: "…"b" in publication column list`) — all verified on 19rc1."
[^47]: Returning Data from Modified Rows (OLD/NEW). https://www.postgresql.org/docs/19/dml-returning.html
    - Features from 18 that a PostgreSQL 19 codebase uses › `RETURNING old.*` / `new.*` (18) — paragraph 1: "…`UPDATE`, `DELETE`, `MERGE` and, in 19, `ON CONFLICT DO SELECT`."
[^48]: CREATE TABLE (WITHOUT OVERLAPS, PERIOD, NOT ENFORCED, generated columns). https://www.postgresql.org/docs/19/sql-createtable.html
    - Features from 18 that a PostgreSQL 19 codebase uses › Temporal constraints (18) — paragraph 1: "…stays application work. 19's `range_minus_multi()` helps compute the remaining pieces."
[^49]: Multicolumn Indexes (skip scan). https://www.postgresql.org/docs/19/indexes-multicolumn.html
    - Features from 18 that a PostgreSQL 19 codebase uses › Planner and `EXPLAIN` (18) — bullet 3: "…remove redundant self-joins for performance — the planner does it."
[^50]: pg_upgrade. https://www.postgresql.org/docs/19/pgupgrade.html
    - Removed / changed in 19 — migration notes from 18 — paragraph 2: "`pg_stat_statements` 1.13, `btree_gist` 1.9, `postgres_fdw` 1.3 ship with 19rc1)."
