### Citations: PostgreSQL 19: The Complete Production SQL Review Guide

Footnote sources for `postgresql-19.md`. Not sent to the model; kept for maintainers.

[^1]: PostgreSQL 19 Release Notes (E.1, draft "as of 2026-10-09", incl. "Migration to Version 19"). https://www.postgresql.org/docs/19/release-19.html
[^2]: PostgreSQL Versioning Policy (supported versions, current minors, final release dates). https://www.postgresql.org/support/versioning/
[^3]: PostgreSQL 19 RC 1 Released! (October 15, 2026; RC 2 on November 12, GA planned November 19, 2026; changes since Beta 4). https://www.postgresql.org/about/news/postgresql-19-rc-1-released-3409/
[^4]: PostgreSQL Roadmap (minor release schedule: November 12, 2026; February 11, May 13, August 12, 2027). https://www.postgresql.org/developer/roadmap/
[^5]: PostgreSQL 18.6, 17.11, 16.15, 15.19, 14.24 and 19 Beta 3 Released (August 13, 2026). https://www.postgresql.org/about/news/postgresql-186-1711-1615-1519-1424-and-19-beta-3-released-3365/
[^6]: REPACK. https://www.postgresql.org/docs/19/sql-repack.html
[^7]: INSERT (ON CONFLICT DO SELECT / DO UPDATE). https://www.postgresql.org/docs/19/sql-insert.html
[^8]: WAIT FOR. https://www.postgresql.org/docs/19/sql-wait.html
[^9]: COPY (FORMAT json, FORCE_ARRAY, ON_ERROR set_null, HEADER). https://www.postgresql.org/docs/19/sql-copy.html
[^10]: Window Functions (IGNORE NULLS / RESPECT NULLS). https://www.postgresql.org/docs/19/functions-window.html
[^11]: pg_plan_advice — help the planner get the right plan. https://www.postgresql.org/docs/19/pgplanadvice.html
[^12]: pg_stash_advice — store and automatically apply plan advice. https://www.postgresql.org/docs/19/pgstashadvice.html
[^13]: EXPLAIN. https://www.postgresql.org/docs/19/sql-explain.html
[^14]: ALTER TABLE (lock levels, NOT VALID, ALTER CONSTRAINT ... ENFORCED). https://www.postgresql.org/docs/19/sql-altertable.html
[^15]: Connections and Authentication (password_expiration_warning_threshold, md5_password_warnings). https://www.postgresql.org/docs/19/runtime-config-connection.html
[^16]: Resource Consumption (io_method, io_min_workers, io_max_workers, autovacuum workers). https://www.postgresql.org/docs/19/runtime-config-resource.html
[^17]: Write Ahead Log (wal_level, effective_wal_level). https://www.postgresql.org/docs/19/runtime-config-wal.html
[^18]: CREATE PUBLICATION (ALL SEQUENCES, ALL TABLES EXCEPT). https://www.postgresql.org/docs/19/sql-createpublication.html
[^19]: JSON Functions and Operators (jsonpath string methods, SQL/JSON). https://www.postgresql.org/docs/19/functions-json.html
[^20]: Binary String Functions and Operators (encode/decode base64url, base32hex). https://www.postgresql.org/docs/19/functions-binarystring.html
[^21]: Range/Multirange Functions and Operators (range_minus_multi, multirange_minus_multi). https://www.postgresql.org/docs/19/functions-range.html
[^22]: pg_stat_statements. https://www.postgresql.org/docs/19/pgstatstatements.html
[^23]: The Cumulative Statistics System. https://www.postgresql.org/docs/19/monitoring-stats.html
[^24]: PostgreSQL 20devel documentation (built 2026-10-21 from master; release notes placeholder, index, JSON_TABLE PLAN, toast_value_type). https://www.postgresql.org/docs/devel/release-20.html , https://www.postgresql.org/docs/devel/bookindex.html , https://www.postgresql.org/docs/devel/functions-json.html , https://www.postgresql.org/docs/devel/sql-createtable.html
[^25]: PostgreSQL 18 Release Notes. https://www.postgresql.org/docs/18/release-18.html
[^26]: Explicit Locking (table- and row-level lock modes). https://www.postgresql.org/docs/19/explicit-locking.html
[^27]: CREATE FUNCTION — Writing SECURITY DEFINER Functions Safely. https://www.postgresql.org/docs/19/sql-createfunction.html
[^28]: Row Security Policies. https://www.postgresql.org/docs/19/ddl-rowsecurity.html
[^29]: PL/pgSQL — Executing Dynamic Commands. https://www.postgresql.org/docs/19/plpgsql-statements.html
[^30]: Transaction Isolation. https://www.postgresql.org/docs/19/transaction-iso.html
[^31]: MERGE. https://www.postgresql.org/docs/19/sql-merge.html
[^32]: JSON Types (jsonb indexing, operator classes). https://www.postgresql.org/docs/19/datatype-json.html
[^33]: Table Partitioning. https://www.postgresql.org/docs/19/ddl-partitioning.html
[^34]: CREATE INDEX (CONCURRENTLY, partitioned tables, ON ONLY). https://www.postgresql.org/docs/19/sql-createindex.html
[^35]: Using EXPLAIN. https://www.postgresql.org/docs/19/using-explain.html
[^36]: PgBouncer — SQL feature map for pooling modes. https://www.pgbouncer.org/features.html
[^37]: PgBouncer changelog (1.21.0 prepared statements in transaction mode; 1.24.0 max_prepared_statements default 200). https://www.pgbouncer.org/changelog.html
[^38]: PostgreSQL 14, 15, 16 and 17 Release Notes. https://www.postgresql.org/docs/14/release-14.html , https://www.postgresql.org/docs/15/release-15.html , https://www.postgresql.org/docs/16/release-16.html , https://www.postgresql.org/docs/17/release-17.html
[^39]: PostgreSQL Wiki — Don't Do This (timestamp, timetz, char(n), money, NOT IN, BETWEEN). https://wiki.postgresql.org/wiki/Don%27t_Do_This
[^40]: Schemas — The Public Schema and Usage Patterns. https://www.postgresql.org/docs/19/ddl-schemas.html
[^41]: Routine Vacuuming (autovacuum, partitioned tables, wraparound). https://www.postgresql.org/docs/19/routine-vacuuming.html
[^42]: Date/Time Types. https://www.postgresql.org/docs/19/datatype-datetime.html
[^43]: CREATE VIEW (security_invoker, security_barrier). https://www.postgresql.org/docs/19/sql-createview.html
[^44]: libpq — Connection Strings (sslmode, min/max_protocol_version, sslkeylogfile, servicefile). https://www.postgresql.org/docs/19/libpq-connect.html
[^45]: UUID Functions (uuidv7, uuidv4, uuid_extract_timestamp). https://www.postgresql.org/docs/19/functions-uuid.html
[^46]: Generated Columns. https://www.postgresql.org/docs/19/ddl-generated-columns.html
[^47]: Returning Data from Modified Rows (OLD/NEW). https://www.postgresql.org/docs/19/dml-returning.html
[^48]: CREATE TABLE (WITHOUT OVERLAPS, PERIOD, NOT ENFORCED, generated columns). https://www.postgresql.org/docs/19/sql-createtable.html
[^49]: Multicolumn Indexes (skip scan). https://www.postgresql.org/docs/19/indexes-multicolumn.html
[^50]: pg_upgrade. https://www.postgresql.org/docs/19/pgupgrade.html
