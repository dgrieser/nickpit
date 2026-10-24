### Citations: SQL Optimization Patterns

Sources for `sql.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: PostgreSQL Versioning Policy and release notes (feature introduction per major version). https://www.postgresql.org/support/versioning/ , https://www.postgresql.org/docs/release/
    - Identify the Dialect First › Feature availability (minimum versions) — paragraph 1: "SQL Server boundaries from Microsoft Learn "Applies to" notes."
[^2]: MySQL 8.4 Reference Manual — What Is New in MySQL 8.0 / 8.4 (row alias 8.0.19, CHECK 8.0.16, functional key parts 8.0.13, SKIP LOCKED 8.0). https://dev.mysql.com/doc/refman/8.0/en/mysql-nutshell.html , https://dev.mysql.com/doc/refman/8.4/en/mysql-nutshell.html
    - Identify the Dialect First › Feature availability (minimum versions) — paragraph 1: "SQL Server boundaries from Microsoft Learn "Applies to" notes."
[^3]: SQLite Release History (UPSERT 3.24.0, window functions 3.25.0, NULLS FIRST/LAST 3.30.0, UPDATE FROM 3.33.0, RETURNING and DROP COLUMN 3.35.0, STRICT 3.37.0, RIGHT/FULL JOIN and IS DISTINCT FROM 3.39.0, concat() 3.44.0, ceiling/floor date modifiers 3.46.0, ALTER TABLE NOT NULL/CHECK constraints 3.53.0; latest 3.54.1, 2026-10-16). https://www.sqlite.org/changes.html
    - Identify the Dialect First › Feature availability (minimum versions) — paragraph 1: "SQL Server boundaries from Microsoft Learn "Applies to" notes."
[^4]: Microsoft Learn — IS [NOT] DISTINCT FROM (Transact-SQL) (SQL Server 2022+). https://learn.microsoft.com/en-us/sql/t-sql/queries/is-distinct-from-transact-sql
    - Identify the Dialect First › Feature availability (minimum versions) — paragraph 1: "SQL Server boundaries from Microsoft Learn "Applies to" notes."
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
[^5]: Microsoft Learn — || (String Concatenation) (Transact-SQL) (SQL Server 2025, Azure SQL; always NULL on NULL input). https://learn.microsoft.com/en-us/sql/t-sql/language-elements/string-concatenation-pipes-transact-sql
    - Identify the Dialect First › Feature availability (minimum versions) — paragraph 1: "SQL Server boundaries from Microsoft Learn "Applies to" notes."
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
    - NULL Semantics › String concatenation with NULL — paragraph 1: "…return NULL (`||` always, `+` with the default `CONCAT_NULL_YIELDS_NULL ON`)."
[^6]: Microsoft Learn — ORDER BY Clause (Transact-SQL) (OFFSET/FETCH from SQL Server 2012, no TOP with OFFSET, NULLs lowest). https://learn.microsoft.com/en-us/sql/t-sql/queries/select-order-by-clause-transact-sql
    - Identify the Dialect First › Feature availability (minimum versions) — paragraph 1: "SQL Server boundaries from Microsoft Learn "Applies to" notes."
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
[^7]: Microsoft Learn — OUTPUT clause (Transact-SQL) (inserted/deleted, triggers restriction, READPAST queues). https://learn.microsoft.com/en-us/sql/t-sql/queries/output-clause-transact-sql
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
    - Writes: Batching, Upserts, Generated Keys › Returning generated keys and modified rows — bullet 2: "…the target table has an enabled trigger for that action."
    - Transactions, Isolation and Locking › Work queues: `SKIP LOCKED` — paragraph 1: "…TOP (1) … WITH (READPAST) OUTPUT deleted.*` for destructive reads."
[^8]: Microsoft Learn — MERGE (Transact-SQL) (semicolon terminator, concurrency considerations, HOLDLOCK). https://learn.microsoft.com/en-us/sql/t-sql/statements/merge-transact-sql
    - Writes: Batching, Upserts, Generated Keys › Upserts: never check-then-insert — paragraph 3: "…the `MERGE`", `HOLDLOCK` (a synonym for `SERIALIZABLE`) prevents unique-key violations."
[^9]: Microsoft Learn — SCOPE_IDENTITY (Transact-SQL) (vs @@IDENTITY and IDENT_CURRENT with triggers). https://learn.microsoft.com/en-us/sql/t-sql/functions/scope-identity-transact-sql
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
    - Writes: Batching, Upserts, Generated Keys › Returning generated keys and modified rows — bullet 2: "…the target table has an enabled trigger for that action."
[^10]: Microsoft Learn — SET TRANSACTION ISOLATION LEVEL (Transact-SQL) (READ COMMITTED default; READ_COMMITTED_SNAPSHOT default ON in Azure SQL Database). https://learn.microsoft.com/en-us/sql/t-sql/statements/set-transaction-isolation-level-transact-sql
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
    - Transactions, Isolation and Locking › Defaults differ — paragraph 1: "Sources: engine manuals."
    - Dialect-specific Features › Optimizer hints — bullet 3: "…read; it is not a performance switch for correctness-sensitive reads."
[^11]: Microsoft Learn — CREATE INDEX (Transact-SQL) (UNIQUE allows one NULL, filtered indexes, INCLUDE, ONLINE edition availability, RESUMABLE not in explicit transactions). https://learn.microsoft.com/en-us/sql/t-sql/statements/create-index-transact-sql
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
    - NULL Semantics › NULLs in UNIQUE constraints — paragraph 1: "…most one NULL** in a unique constraint or unique index."
    - Schema Migrations Without Outages › DDL and transactions — bullet 1: "…nor `RESUMABLE = ON` index operations inside an explicit transaction."
    - Schema Migrations Without Outages › Principles that hold everywhere — table row 3: "…— online index operations are not available in every edition"
[^12]: Microsoft Learn — CONCAT (Transact-SQL) (NULL converted to empty string). https://learn.microsoft.com/en-us/sql/t-sql/functions/concat-transact-sql
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
    - NULL Semantics › String concatenation with NULL — paragraph 1: "…return NULL (`||` always, `+` with the default `CONCAT_NULL_YIELDS_NULL ON`)."
[^13]: Microsoft Learn — / (Division) (Transact-SQL) (integer division truncates). https://learn.microsoft.com/en-us/sql/t-sql/language-elements/divide-transact-sql
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
    - Data Types › Integer division — paragraph 1: "…and SQL Server (integer operands truncate), but `3.5000` in MySQL."
[^14]: Microsoft Learn — bit (Transact-SQL). https://learn.microsoft.com/en-us/sql/t-sql/data-types/bit-transact-sql
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
[^15]: Microsoft Learn — LEN (Transact-SQL) (excludes trailing spaces; DATALENGTH for bytes). https://learn.microsoft.com/en-us/sql/t-sql/functions/len-transact-sql
    - Strings, Collations and Case Sensitivity › Length functions — table row 4: "SQL Server | `LEN()` (**excludes trailing spaces**)"
[^16]: Microsoft Learn — Collation and Unicode support (CI/CS/AS options, default setup collation SQL_Latin1_General_CP1_CI_AS for English (United States)). https://learn.microsoft.com/en-us/sql/relational-databases/collations/collation-and-unicode-support
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
    - Strings, Collations and Case Sensitivity › Equality and `LIKE` — paragraph 1: "…for an "English (United States)" installation is `SQL_Latin1_General_CP1_CI_AS` (case-insensitive, accent-sensitive)."
[^17]: Microsoft Learn — BEGIN TRANSACTION (Transact-SQL) (XACT_ABORT, nested transactions). https://learn.microsoft.com/en-us/sql/t-sql/language-elements/begin-transaction-transact-sql
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
    - Transactions, Isolation and Locking › Deadlocks and retries — bullet 4: "…transaction — an inner `ROLLBACK` rolls back the outermost transaction."
[^18]: Microsoft Learn — Table hints (Transact-SQL) (UPDLOCK, SERIALIZABLE/HOLDLOCK, READPAST, NOLOCK). https://learn.microsoft.com/en-us/sql/t-sql/queries/hints-transact-sql-table
    - Writes: Batching, Upserts, Generated Keys › Upserts: never check-then-insert — paragraph 3: "…range locks so the missing key cannot be inserted concurrently)."
    - Transactions, Isolation and Locking › Work queues: `SKIP LOCKED` — paragraph 1: "…TOP (1) … WITH (READPAST) OUTPUT deleted.*` for destructive reads."
    - Dialect-specific Features › Optimizer hints — bullet 3: "…read; it is not a performance switch for correctness-sensitive reads."
[^19]: PostgreSQL 18 — The Cumulative Statistics System (pg_stat_user_tables.relname, pg_stat_user_indexes.indexrelname). https://www.postgresql.org/docs/18/monitoring-stats.html
    - Maintenance and Monitoring › Monitoring queries (PostgreSQL) — paragraph 1: "…in `pg_stat_user_tables`/`pg_stat_user_indexes` (those names belong to the `pg_tables`/`pg_indexes` catalog views)."
[^20]: Microsoft Learn — sp_executesql (Transact-SQL); QUOTENAME (Transact-SQL). https://learn.microsoft.com/en-us/sql/relational-databases/system-stored-procedures/sp-executesql-transact-sql , https://learn.microsoft.com/en-us/sql/t-sql/functions/quotename-transact-sql
    - Security: Parameters, Identifiers, Dynamic SQL › Dynamic SQL inside the database — bullet 3: "`EXEC('…' + @input)` is injectable."
[^21]: PostgreSQL 18 — INSERT (ON CONFLICT, RETURNING). https://www.postgresql.org/docs/18/sql-insert.html
    - Writes: Batching, Upserts, Generated Keys › Upserts: never check-then-insert — paragraph 2: "…after `ON CONFLICT` must name a unique constraint or index."
    - Writes: Batching, Upserts, Generated Keys › Upserts: never check-then-insert — bullet 1: "…the alternative that can update when a concurrent insert occurs."
[^22]: PostgreSQL 18 — MERGE. https://www.postgresql.org/docs/18/sql-merge.html
    - Writes: Batching, Upserts, Generated Keys › Upserts: never check-then-insert — bullet 1: "…the alternative that can update when a concurrent insert occurs."
[^23]: PostgreSQL 18 — CREATE INDEX (CONCURRENTLY, INCLUDE, partial indexes, NULLS NOT DISTINCT). https://www.postgresql.org/docs/18/sql-createindex.html
    - Schema Migrations Without Outages › Principles that hold everywhere — table row 1: "…transaction; a failed build leaves an `INVALID` index to drop)"
[^24]: PostgreSQL 18 — ALTER TABLE (NOT VALID, VALIDATE CONSTRAINT, lock levels). https://www.postgresql.org/docs/18/sql-altertable.html
    - Schema Migrations Without Outages › Principles that hold everywhere — bullet 4: "…(validation takes a weaker lock that permits reads and writes)."
[^25]: PostgreSQL 18 — Using EXPLAIN. https://www.postgresql.org/docs/18/using-explain.html
    - Reading Execution Plans › Commands per dialect — paragraph 1: "**`EXPLAIN ANALYZE` executes the statement.**"
[^26]: PostgreSQL 18 — Transaction Isolation. https://www.postgresql.org/docs/18/transaction-iso.html
    - Transactions, Isolation and Locking › Defaults differ — paragraph 1: "Sources: engine manuals."
[^27]: PostgreSQL 18 — Date/Time Functions and Operators (Current Date/Time: transaction start time). https://www.postgresql.org/docs/18/functions-datetime.html
    - Transactions, Isolation and Locking › Autocommit and the current time — bullet 2: "`clock_timestamp()` is wall time."
[^28]: PostgreSQL Wiki — Don't Do This (NOT IN, BETWEEN with timestamps, timestamp without time zone, money). https://wiki.postgresql.org/wiki/Don%27t_Do_This
    - NULL Semantics › `NOT IN` with a nullable subquery returns nothing — paragraph 2: "…and stays safe only as long as the schema does."
    - Dates, Times and Time Zones › Store instants in UTC with a time-zone-aware type — table row 1: "…for instants is a common bug: the zone is lost"
    - Money and Numeric Precision — bullet 3: "…output and rounds to the locale's fraction digits; avoid it."
[^29]: PostgreSQL 18 — Message Formats (Bind: Int16 number of parameter values). https://www.postgresql.org/docs/18/protocol-message-formats.html
    - Security: Parameters, Identifiers, Dynamic SQL › IN lists — paragraph 2: "…are sent as `sp_executesql` calls and hit the same limit)."
[^30]: MySQL 8.4 Reference Manual — INSERT ... ON DUPLICATE KEY UPDATE Statement. https://dev.mysql.com/doc/refman/8.4/en/insert-on-duplicate.html
    - Writes: Batching, Upserts, Generated Keys › Upserts: never check-then-insert — paragraph 2: "…after `ON CONFLICT` must name a unique constraint or index."
    - Writes: Batching, Upserts, Generated Keys › Upserts: never check-then-insert — bullet 2: "…0 for an unchanged row; drivers with `clientFoundRows` report differently."
[^31]: MySQL 8.4 Reference Manual — Row Constructor Expression Optimization. https://dev.mysql.com/doc/refman/8.4/en/row-constructor-optimization.html
    - Pagination › Keyset (seek) pagination — paragraph 2: "Write the expanded form:"
[^32]: MySQL 8.4 Reference Manual — Type Conversion in Expression Evaluation. https://dev.mysql.com/doc/refman/8.4/en/type-conversion.html
    - Data Types › Implicit conversions between strings and numbers — paragraph 1: "…true (with only a 1292 "Truncated incorrect DOUBLE value" warning)."
[^33]: MySQL 8.4 Reference Manual — Statements That Cause an Implicit Commit. https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html
    - Schema Migrations Without Outages › DDL and transactions — bullet 2: "…and a `ROLLBACK` after DDL does not undo earlier DML."
[^34]: MySQL 8.4 Reference Manual — Online DDL Operations; Metadata Locking. https://dev.mysql.com/doc/refman/8.4/en/innodb-online-ddl-operations.html , https://dev.mysql.com/doc/refman/8.4/en/metadata-locking.html
    - Schema Migrations Without Outages › Principles that hold everywhere — table row 2: "…(InnoDB secondary indexes are built in place, permitting concurrent DML)"
[^35]: MySQL 8.4 Reference Manual — Date and Time Functions (NOW() is the statement start time; SYSDATE()). https://dev.mysql.com/doc/refman/8.4/en/date-and-time-functions.html
    - Transactions, Isolation and Locking › Autocommit and the current time — bullet 2: "`SYSDATE()` is wall time."
[^36]: MySQL 8.4 Reference Manual — The DATE, DATETIME, and TIMESTAMP Types. https://dev.mysql.com/doc/refman/8.4/en/datetime.html
    - Dates, Times and Time Zones › Store instants in UTC with a time-zone-aware type — table row 2: "…has no zone — the application must fix the convention"
[^37]: MySQL 8.4 Reference Manual — Transaction Isolation Levels; Locking Reads (NOWAIT, SKIP LOCKED). https://dev.mysql.com/doc/refman/8.4/en/innodb-transaction-isolation-levels.html , https://dev.mysql.com/doc/refman/8.4/en/innodb-locking-reads.html
    - Transactions, Isolation and Locking › Defaults differ — paragraph 1: "Sources: engine manuals."
[^38]: MySQL 8.4 Reference Manual — EXPLAIN Statement. https://dev.mysql.com/doc/refman/8.4/en/explain.html
    - Reading Execution Plans › Commands per dialect — paragraph 1: "**`EXPLAIN ANALYZE` executes the statement.**"
[^39]: MySQL 8.4 Reference Manual — Logical Operators (|| as OR deprecated; PIPES_AS_CONCAT). https://dev.mysql.com/doc/refman/8.4/en/logical-operators.html
    - Identify the Dialect First › Feature availability (minimum versions) — table row 18: "…string concatenation | yes | logical OR by default (deprecated)"
[^40]: MySQL 8.4 Reference Manual — Prepared Statements (placeholder limit, error 1390). https://dev.mysql.com/doc/refman/8.4/en/sql-prepared-statements.html
    - Security: Parameters, Identifiers, Dynamic SQL › IN lists — paragraph 2: "…**MySQL 65,535** ("Prepared statement contains too many placeholders", error 1390)"
[^41]: SQLite — Datatypes In SQLite (type affinity). https://www.sqlite.org/datatype3.html
    - Data Types › SQLite type affinity — paragraph 1: "…an `INTEGER` column accepts `'abc'`, a `BOOLEAN` column accepts `'yes'`."
[^42]: SQLite — STRICT Tables. https://www.sqlite.org/stricttables.html
    - Data Types › SQLite type affinity — paragraph 1: "…an `INTEGER` column accepts `'abc'`, a `BOOLEAN` column accepts `'yes'`."
[^43]: SQLite — Foreign Key Support (PRAGMA foreign_keys). https://www.sqlite.org/foreignkeys.html
    - Constraints and Referential Integrity › Foreign keys: engine differences — bullet 1: "…default is off; it cannot be changed inside a transaction)."
[^44]: SQLite — UPSERT. https://www.sqlite.org/lang_upsert.html
    - Writes: Batching, Upserts, Generated Keys › Upserts: never check-then-insert — paragraph 2: "…after `ON CONFLICT` must name a unique constraint or index."
[^45]: SQLite — RETURNING. https://www.sqlite.org/lang_returning.html
    - Writes: Batching, Upserts, Generated Keys › Upserts: never check-then-insert — paragraph 2: "…after `ON CONFLICT` must name a unique constraint or index."
[^46]: SQLite — The SQLite Query Optimizer Overview (the LIKE optimization). https://www.sqlite.org/optoverview.html
    - Sargable Predicates › `LIKE` — bullet 3: "…index, or `PRAGMA case_sensitive_like = ON` with a `BINARY` index."
[^47]: SQLite — Limits In SQLite (SQLITE_MAX_VARIABLE_NUMBER: 999 before 3.32.0, 32766 after). https://www.sqlite.org/limits.html
    - Security: Parameters, Identifiers, Dynamic SQL › IN lists — paragraph 2: "…are sent as `sp_executesql` calls and hit the same limit)."
[^48]: SQLite — ALTER TABLE (supported operations, generalized table rebuild procedure). https://www.sqlite.org/lang_altertable.html
    - Schema Migrations Without Outages › Principles that hold everywhere — bullet 7: "…drop, rename" procedure inside a transaction with foreign keys disabled."
[^49]: SQLite — Date And Time Functions (formats, modifiers, ceiling/floor, 'now' constant within one sqlite3_step() call). https://www.sqlite.org/lang_datefunc.html
    - Dialect Differences at a Glance — table row 16: "…`'now'` is constant within one `sqlite3_step()` call, UTC text"
    - Dates, Times and Time Zones › Store instants in UTC with a time-zone-aware type — table row 3: "…date functions work on text, real (Julian day) or integer"
[^50]: SQLite — Transaction (BEGIN DEFERRED/IMMEDIATE/EXCLUSIVE); Write-Ahead Logging. https://www.sqlite.org/lang_transaction.html , https://www.sqlite.org/wal.html
    - Transactions, Isolation and Locking › Defaults differ — table row 3: "…file; readers do not block the writer in WAL mode"
[^51]: Microsoft Learn — Maximum capacity specifications for SQL Server (2,100 parameters per stored procedure). https://learn.microsoft.com/en-us/sql/sql-server/maximum-capacity-specifications-for-sql-server
    - Security: Parameters, Identifiers, Dynamic SQL › IN lists — paragraph 2: "…are sent as `sp_executesql` calls and hit the same limit)."
[^52]: OWASP — SQL Injection Prevention Cheat Sheet. https://cheatsheetseries.owasp.org/cheatsheets/SQL_Injection_Prevention_Cheat_Sheet.html
    - Security: Parameters, Identifiers, Dynamic SQL › Values: always bind parameters — paragraph 1: "…SQL is a **blocking finding**, whatever escaping function is used."
[^53]: SQLite — Quirks, Caveats, and Gotchas (flexible typing, double-quoted string literals, foreign keys off by default). https://www.sqlite.org/quirks.html
    - Query Patterns › `GROUP BY` rules differ — bullet 3: "…row matching `min()`/`max()` when exactly one of those is used)."
    - Strings, Collations and Case Sensitivity › Identifier quoting and case folding — bullet 3: "…typo in a column name silently becomes a string constant."
[^54]: MySQL 8.4 Reference Manual — CHECK Constraints (enforced from 8.0.16). https://dev.mysql.com/doc/refman/8.4/en/create-table-check-constraints.html
    - Constraints and Referential Integrity — bullet 3: "…`CHECK` only since **8.0.16**; earlier versions parse and ignore it."
[^55]: Microsoft Learn — Data type precedence (Transact-SQL). https://learn.microsoft.com/en-us/sql/t-sql/data-types/data-type-precedence-transact-sql
    - Sargable Predicates › Type mismatches — paragraph 1: "…column converts every value to `int` (failing on non-numeric rows)."
[^56]: Microsoft Learn — DATEADD (Transact-SQL) (month overflow returns the last day of the month). https://learn.microsoft.com/en-us/sql/t-sql/functions/dateadd-transact-sql
    - Dates, Times and Time Zones › Date arithmetic differs at month ends — paragraph 1: "…'2024-08-31')` returns the last day of the target month (`2024-09-30`)."
[^57]: Microsoft Learn — SET LOCK_TIMEOUT (Transact-SQL). https://learn.microsoft.com/en-us/sql/t-sql/statements/set-lock-timeout-transact-sql
    - Transactions, Isolation and Locking › Lock timeouts — paragraph 2: "`SET LOCK_TIMEOUT 500;` (milliseconds; the default `-1` waits forever)."
[^58]: Microsoft Learn — Primary and foreign key constraints (no automatic index on foreign keys). https://learn.microsoft.com/en-us/sql/relational-databases/tables/primary-and-foreign-key-constraints
    - Constraints and Referential Integrity › Foreign keys: engine differences — bullet 2: "…a foreign key constraint doesn't automatically create a corresponding index")."
[^59]: MySQL 8.4 Reference Manual — The INFORMATION_SCHEMA TABLES Table (TABLE_ROWS estimate may vary by 40–50 %). https://dev.mysql.com/doc/refman/8.4/en/information-schema-tables-table.html
    - Counting and Existence Checks › `COUNT(*)` on large tables — paragraph 2: "…as much as 40 % to 50 % for InnoDB."
[^60]: SQLite — Isolation In SQLite. https://www.sqlite.org/isolation.html
    - Transactions, Isolation and Locking › Defaults differ — paragraph 1: "Sources: engine manuals."
[^61]: Microsoft Learn — SET ARITHABORT (Transact-SQL) (divide-by-zero ends the query when ANSI_WARNINGS is ON, the default). https://learn.microsoft.com/en-us/sql/t-sql/statements/set-arithabort-transact-sql
    - Dialect Differences at a Glance — table row 5: "…`SELECT` | `NULL` | error (with the default `ANSI_WARNINGS ON`)"
[^62]: Microsoft Learn — MSSQLSERVER_1205 (deadlock victim; the victim's transaction is rolled back; rerun it). https://learn.microsoft.com/en-us/sql/relational-databases/errors-events/mssqlserver-1205-database-engine-error
    - Transactions, Isolation and Locking › Deadlocks and retries — bullet 3: "…the victim's transaction is rolled back and must be rerun)"
[^63]: Microsoft Learn — CREATE DATABASE, ALTER DATABASE, DROP DATABASE, BACKUP and RESTORE (Transact-SQL) ("isn't allowed in an explicit or implicit transaction"). https://learn.microsoft.com/en-us/sql/t-sql/statements/create-database-transact-sql , https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-database-transact-sql , https://learn.microsoft.com/en-us/sql/t-sql/statements/drop-database-transact-sql , https://learn.microsoft.com/en-us/sql/t-sql/statements/backup-transact-sql , https://learn.microsoft.com/en-us/sql/t-sql/statements/restore-statements-transact-sql
    - Dialect Differences at a Glance — paragraph 1: "…INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`."
    - Schema Migrations Without Outages › DDL and transactions — bullet 1: "…nor `RESUMABLE = ON` index operations inside an explicit transaction."
