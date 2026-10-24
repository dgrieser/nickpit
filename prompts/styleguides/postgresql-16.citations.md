### Citations: PostgreSQL 16: The Complete Review Guide

Sources for `postgresql-16.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: PostgreSQL 16 Release Notes (E.16, Release 16; release date 2023-09-14). https://www.postgresql.org/docs/release/16.0/
    - Overview — quote block: "PostgreSQL 16.0 was released on **September 14, 2023**."
    - Overview — paragraph 1: "…in `pg_hba.conf`**, and the libpq options **`require_auth`**, **`load_balance_hosts`** and **`sslrootcert=system`**."
    - Overview — paragraph 2: "…`force_parallel_mode` (now `debug_parallel_query`), `vacuum_defer_cleanup_age`, `promote_trigger_file`, `lc_collate` and `lc_ctype` are gone."
    - What's New in PostgreSQL 16 — paragraph 1: "All items below are verified against the official release notes"
    - What's New in PostgreSQL 16 › Server, performance and operations — bullet 1: "…and `array_agg()` can run in parallel; cached `RANGE`/`LIST` partition lookups."
    - New in 16: Roles, CREATEROLE and Membership Options — paragraph 1: "…in 16 for provisioning scripts, multi-tenant setups and admin tooling."
[^2]: PostgreSQL Versioning Policy (supported versions: 16.15 current, first release September 14, 2023, final release November 9, 2028; 17.11, 18.6; 19 RC 1 news) and PostgreSQL 16.15 Release Notes (August 13, 2026). https://www.postgresql.org/support/versioning/ , https://www.postgresql.org/docs/release/16.15/
    - Overview — quote block: "…the next scheduled minor release date is November 12, 2026."
    - Overview — quote block: "PostgreSQL 16 is **supported until November 9, 2028**."
    - Overview — quote block: "It is pre-release software and not a deployment target."
    - New in 16: Logical Replication — paragraph 1: "CVE-2026-6471)."
    - Security › Other security items — bullet 5: "…functions and `ascii()`, and missing `USAGE` checks on data types."
    - Upgrading to 16 and Staying Current › 16.x minor releases — bullet 14: "…contain `NaN` and `ltree` B-tree indexes with very long values."
[^3]: PostgreSQL Roadmap (next minor releases: November 12, 2026, February 11, 2027). https://www.postgresql.org/developer/roadmap/
    - Overview — quote block: "…the next scheduled minor release date is November 12, 2026."
[^4]: PostgreSQL 16 Released! (press release, September 14, 2023). https://www.postgresql.org/about/news/postgresql-16-released-2715/
    - Overview — quote block: "PostgreSQL 16.0 was released on **September 14, 2023**."
    - Overview — paragraph 1: "…in `pg_hba.conf`**, and the libpq options **`require_auth`**, **`load_balance_hosts`** and **`sslrootcert=system`**."
[^5]: PostgreSQL 17 Release Notes (Release 17.0). https://www.postgresql.org/docs/release/17.0/
    - Do Not Use or Recommend (PostgreSQL 17+ Features) — paragraph 1: "…table was introduced in **PostgreSQL 17** (released September 26, 2024)"
[^6]: PostgreSQL 18 Release Notes (Release 18.0). https://www.postgresql.org/docs/release/18.0/
    - Do Not Use or Recommend (PostgreSQL 17+ Features) — paragraph 2: "From **PostgreSQL 18** (released September 25, 2025)"
[^7]: PostgreSQL 19 Release Notes (draft, Release Candidate 1). https://www.postgresql.org/docs/19/release-19.html
    - Do Not Use or Recommend (PostgreSQL 17+ Features) — paragraph 3: "…(`syntax error at or near "REPACK"` on 16.15) and `error_on_null()`."
[^8]: PostgreSQL 15 and 14 Release Notes (feature version markers). https://www.postgresql.org/docs/release/15.0/ , https://www.postgresql.org/docs/release/14.0/
    - Features from PostgreSQL 14–15 You Can Rely On — paragraph 1: "…exist on 16 and should be used where they fit."
    - New in 16: Numeric Literals and Input — paragraph 1: "…`1 | 0` (15 started rejecting trailing junk after numbers)."
[^9]: PostgreSQL 16 Documentation: JSON Functions and Operators (SQL/JSON constructors, IS JSON). https://www.postgresql.org/docs/16/functions-json.html
    - New in 16: SQL/JSON Constructors and IS JSON — paragraph 1: "…adds the SQL-standard constructor syntax and the `IS JSON` predicate."
[^10]: PostgreSQL 16.1 – 16.14 Release Notes. https://www.postgresql.org/docs/release/16.1/ (and /16.2/ ... /16.14/ under https://www.postgresql.org/docs/release/)
    - New in 16: SQL/JSON Constructors and IS JSON › JSON_OBJECTAGG and JSON_ARRAYAGG — bullet 4: "…could perform wrong query transformations when an input was NULL)."
    - New in 16: Logical Replication — paragraph 1: "CVE-2026-6471)."
    - Security › SQL injection in dynamic SQL — paragraph 2: "…16.8: invalidly encoded input could break out of the quotes)."
    - Security › Row-level security — bullet 8: "Multi-tenant RLS setups must run a current minor."
    - Security › Other security items — bullet 4: "…trusted sources, and restore with the matching (current) client tools."
    - Upgrading to 16 and Staying Current › 16.x minor releases — paragraph 1: "…only fixes and need no dump/restore, but some require actions:"
[^11]: PostgreSQL 16 Documentation: CREATE ROLE, GRANT, Role Membership and Predefined Roles (pg_create_subscription, pg_use_reserved_connections). https://www.postgresql.org/docs/16/sql-createrole.html , https://www.postgresql.org/docs/16/sql-grant.html , https://www.postgresql.org/docs/16/role-membership.html , https://www.postgresql.org/docs/16/predefined-roles.html
    - New in 16: Roles, CREATEROLE and Membership Options — paragraph 1: "…in 16 for provisioning scripts, multi-tenant setups and admin tooling."
    - New in 16: Roles, CREATEROLE and Membership Options › New predefined roles and reserved connections — bullet 1: "…(`ERROR: password is required`), and `password_required = false` is superuser-only."
[^12]: PostgreSQL 16 Documentation: Connections and Authentication (reserved_connections, scram_iterations). https://www.postgresql.org/docs/16/runtime-config-connection.html
    - New in 16: Roles, CREATEROLE and Membership Options › New predefined roles and reserved connections — bullet 2: "…they can still log in when the application exhausts `max_connections`."
[^13]: PostgreSQL 16 Documentation: Explicit Locking. https://www.postgresql.org/docs/16/explicit-locking.html
    - Schema Migrations and Locking › Lock levels of common DDL (measured on 16.15 with pg_locks) — paragraph 1: "…for lock modes: the "Explicit Locking" and `ALTER TABLE` documentation."
[^14]: PostgreSQL 16 Documentation: ALTER TABLE; CREATE INDEX (Building Indexes Concurrently). https://www.postgresql.org/docs/16/sql-altertable.html , https://www.postgresql.org/docs/16/sql-createindex.html
    - Schema Migrations and Locking › Lock levels of common DDL (measured on 16.15 with pg_locks) — paragraph 1: "…for lock modes: the "Explicit Locking" and `ALTER TABLE` documentation."
[^15]: PostgreSQL 16 Documentation: Transaction Isolation. https://www.postgresql.org/docs/16/transaction-iso.html
    - Transactions, Isolation and Concurrency › Isolation levels (all behaviour verified with two sessions on 16.15) — section heading
[^16]: PostgreSQL 16 Documentation: MERGE. https://www.postgresql.org/docs/16/sql-merge.html
    - Transactions, Isolation and Concurrency › Upserts and MERGE — bullet 4: "ON CONFLICT DO UPDATE`."
[^17]: PostgreSQL 16 Documentation: CREATE FUNCTION — Writing SECURITY DEFINER Functions Safely. https://www.postgresql.org/docs/16/sql-createfunction.html
    - Security › SECURITY DEFINER functions — paragraph 1: "FROM PUBLIC` per function."
[^18]: PostgreSQL 16 Documentation: Row Security Policies. https://www.postgresql.org/docs/16/ddl-rowsecurity.html
    - Security › Row-level security — paragraph 1: "Behaviour verified on 16.15:"
[^19]: PostgreSQL Wiki: Don't Do This. https://wiki.postgresql.org/wiki/Don't_Do_This
    - Data Types: Pitfalls › Timestamps — bullet 1: "Use **`timestamptz`** for points in time."
[^20]: PostgreSQL 16 Documentation: Table Partitioning. https://www.postgresql.org/docs/16/ddl-partitioning.html
    - Partitioning — paragraph 1: "Rules verified on 16.15:"
[^21]: PgBouncer configuration (max_prepared_statements, transaction pooling restrictions). https://www.pgbouncer.org/config.html
    - Connection Pooling and Application Access — bullet 2: "…locks, `LISTEN`, `WITH HOLD` cursors, temporary tables, and SQL-level `PREPARE`/`EXECUTE`."
    - Connection Pooling and Application Access — bullet 3: "SQL `PREPARE` statements are still not tracked."
[^22]: PostgreSQL 16 Documentation: libpq Connection Parameters (require_auth, load_balance_hosts, sslcertmode, sslrootcert=system). https://www.postgresql.org/docs/16/libpq-connect.html
    - New in 16: Authentication, Connections and libpq — paragraph 1: "Sources: libpq connection parameters"
[^23]: PostgreSQL 16 Documentation: The pg_hba.conf File (regular expressions, include directives). https://www.postgresql.org/docs/16/auth-pg-hba-conf.html
    - New in 16: Authentication, Connections and libpq — paragraph 1: "Sources: libpq connection parameters and `pg_hba.conf`"
[^24]: PostgreSQL 16 Documentation: Logical Decoding Concepts (slots on standbys, hot_standby_feedback). https://www.postgresql.org/docs/16/logicaldecoding-explanation.html
    - New in 16: Logical Replication — table row 1: "Logical decoding on **standbys**"
