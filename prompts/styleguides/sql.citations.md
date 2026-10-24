### Citations: SQL Optimization Patterns

Footnote sources for `sql.md`. Not sent to the model; kept for maintainers.

[^1]: PostgreSQL Versioning Policy and release notes (feature introduction per major version). https://www.postgresql.org/support/versioning/ , https://www.postgresql.org/docs/release/
[^2]: MySQL 8.4 Reference Manual — What Is New in MySQL 8.0 / 8.4 (row alias 8.0.19, CHECK 8.0.16, functional key parts 8.0.13, SKIP LOCKED 8.0). https://dev.mysql.com/doc/refman/8.0/en/mysql-nutshell.html , https://dev.mysql.com/doc/refman/8.4/en/mysql-nutshell.html
[^3]: SQLite Release History (UPSERT 3.24.0, window functions 3.25.0, NULLS FIRST/LAST 3.30.0, UPDATE FROM 3.33.0, RETURNING and DROP COLUMN 3.35.0, STRICT 3.37.0, RIGHT/FULL JOIN and IS DISTINCT FROM 3.39.0, concat() 3.44.0, ceiling/floor date modifiers 3.46.0, ALTER TABLE NOT NULL/CHECK constraints 3.53.0; latest 3.54.1, 2026-10-16). https://www.sqlite.org/changes.html
[^4]: Microsoft Learn — IS [NOT] DISTINCT FROM (Transact-SQL) (SQL Server 2022+). https://learn.microsoft.com/en-us/sql/t-sql/queries/is-distinct-from-transact-sql
[^5]: Microsoft Learn — || (String Concatenation) (Transact-SQL) (SQL Server 2025, Azure SQL; always NULL on NULL input). https://learn.microsoft.com/en-us/sql/t-sql/language-elements/string-concatenation-pipes-transact-sql
[^6]: Microsoft Learn — ORDER BY Clause (Transact-SQL) (OFFSET/FETCH from SQL Server 2012, no TOP with OFFSET, NULLs lowest). https://learn.microsoft.com/en-us/sql/t-sql/queries/select-order-by-clause-transact-sql
[^7]: Microsoft Learn — OUTPUT clause (Transact-SQL) (inserted/deleted, triggers restriction, READPAST queues). https://learn.microsoft.com/en-us/sql/t-sql/queries/output-clause-transact-sql
[^8]: Microsoft Learn — MERGE (Transact-SQL) (semicolon terminator, concurrency considerations, HOLDLOCK). https://learn.microsoft.com/en-us/sql/t-sql/statements/merge-transact-sql
[^9]: Microsoft Learn — SCOPE_IDENTITY (Transact-SQL) (vs @@IDENTITY and IDENT_CURRENT with triggers). https://learn.microsoft.com/en-us/sql/t-sql/functions/scope-identity-transact-sql
[^10]: Microsoft Learn — SET TRANSACTION ISOLATION LEVEL (Transact-SQL) (READ COMMITTED default; READ_COMMITTED_SNAPSHOT default ON in Azure SQL Database). https://learn.microsoft.com/en-us/sql/t-sql/statements/set-transaction-isolation-level-transact-sql
[^11]: Microsoft Learn — CREATE INDEX (Transact-SQL) (UNIQUE allows one NULL, filtered indexes, INCLUDE, ONLINE edition availability, RESUMABLE not in explicit transactions). https://learn.microsoft.com/en-us/sql/t-sql/statements/create-index-transact-sql
[^12]: Microsoft Learn — CONCAT (Transact-SQL) (NULL converted to empty string). https://learn.microsoft.com/en-us/sql/t-sql/functions/concat-transact-sql
[^13]: Microsoft Learn — / (Division) (Transact-SQL) (integer division truncates). https://learn.microsoft.com/en-us/sql/t-sql/language-elements/divide-transact-sql
[^14]: Microsoft Learn — bit (Transact-SQL). https://learn.microsoft.com/en-us/sql/t-sql/data-types/bit-transact-sql
[^15]: Microsoft Learn — LEN (Transact-SQL) (excludes trailing spaces; DATALENGTH for bytes). https://learn.microsoft.com/en-us/sql/t-sql/functions/len-transact-sql
[^16]: Microsoft Learn — Collation and Unicode support (CI/CS/AS options, default setup collation SQL_Latin1_General_CP1_CI_AS for English (United States)). https://learn.microsoft.com/en-us/sql/relational-databases/collations/collation-and-unicode-support
[^17]: Microsoft Learn — BEGIN TRANSACTION (Transact-SQL) (XACT_ABORT, nested transactions). https://learn.microsoft.com/en-us/sql/t-sql/language-elements/begin-transaction-transact-sql
[^18]: Microsoft Learn — Table hints (Transact-SQL) (UPDLOCK, SERIALIZABLE/HOLDLOCK, READPAST, NOLOCK). https://learn.microsoft.com/en-us/sql/t-sql/queries/hints-transact-sql-table
[^19]: PostgreSQL 18 — The Cumulative Statistics System (pg_stat_user_tables.relname, pg_stat_user_indexes.indexrelname). https://www.postgresql.org/docs/18/monitoring-stats.html
[^20]: Microsoft Learn — sp_executesql (Transact-SQL); QUOTENAME (Transact-SQL). https://learn.microsoft.com/en-us/sql/relational-databases/system-stored-procedures/sp-executesql-transact-sql , https://learn.microsoft.com/en-us/sql/t-sql/functions/quotename-transact-sql
[^21]: PostgreSQL 18 — INSERT (ON CONFLICT, RETURNING). https://www.postgresql.org/docs/18/sql-insert.html
[^22]: PostgreSQL 18 — MERGE. https://www.postgresql.org/docs/18/sql-merge.html
[^23]: PostgreSQL 18 — CREATE INDEX (CONCURRENTLY, INCLUDE, partial indexes, NULLS NOT DISTINCT). https://www.postgresql.org/docs/18/sql-createindex.html
[^24]: PostgreSQL 18 — ALTER TABLE (NOT VALID, VALIDATE CONSTRAINT, lock levels). https://www.postgresql.org/docs/18/sql-altertable.html
[^25]: PostgreSQL 18 — Using EXPLAIN. https://www.postgresql.org/docs/18/using-explain.html
[^26]: PostgreSQL 18 — Transaction Isolation. https://www.postgresql.org/docs/18/transaction-iso.html
[^27]: PostgreSQL 18 — Date/Time Functions and Operators (Current Date/Time: transaction start time). https://www.postgresql.org/docs/18/functions-datetime.html
[^28]: PostgreSQL Wiki — Don't Do This (NOT IN, BETWEEN with timestamps, timestamp without time zone, money). https://wiki.postgresql.org/wiki/Don%27t_Do_This
[^29]: PostgreSQL 18 — Message Formats (Bind: Int16 number of parameter values). https://www.postgresql.org/docs/18/protocol-message-formats.html
[^30]: MySQL 8.4 Reference Manual — INSERT ... ON DUPLICATE KEY UPDATE Statement. https://dev.mysql.com/doc/refman/8.4/en/insert-on-duplicate.html
[^31]: MySQL 8.4 Reference Manual — Row Constructor Expression Optimization. https://dev.mysql.com/doc/refman/8.4/en/row-constructor-optimization.html
[^32]: MySQL 8.4 Reference Manual — Type Conversion in Expression Evaluation. https://dev.mysql.com/doc/refman/8.4/en/type-conversion.html
[^33]: MySQL 8.4 Reference Manual — Statements That Cause an Implicit Commit. https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html
[^34]: MySQL 8.4 Reference Manual — Online DDL Operations; Metadata Locking. https://dev.mysql.com/doc/refman/8.4/en/innodb-online-ddl-operations.html , https://dev.mysql.com/doc/refman/8.4/en/metadata-locking.html
[^35]: MySQL 8.4 Reference Manual — Date and Time Functions (NOW() is the statement start time; SYSDATE()). https://dev.mysql.com/doc/refman/8.4/en/date-and-time-functions.html
[^36]: MySQL 8.4 Reference Manual — The DATE, DATETIME, and TIMESTAMP Types. https://dev.mysql.com/doc/refman/8.4/en/datetime.html
[^37]: MySQL 8.4 Reference Manual — Transaction Isolation Levels; Locking Reads (NOWAIT, SKIP LOCKED). https://dev.mysql.com/doc/refman/8.4/en/innodb-transaction-isolation-levels.html , https://dev.mysql.com/doc/refman/8.4/en/innodb-locking-reads.html
[^38]: MySQL 8.4 Reference Manual — EXPLAIN Statement. https://dev.mysql.com/doc/refman/8.4/en/explain.html
[^39]: MySQL 8.4 Reference Manual — Logical Operators (|| as OR deprecated; PIPES_AS_CONCAT). https://dev.mysql.com/doc/refman/8.4/en/logical-operators.html
[^40]: MySQL 8.4 Reference Manual — Prepared Statements (placeholder limit, error 1390). https://dev.mysql.com/doc/refman/8.4/en/sql-prepared-statements.html
[^41]: SQLite — Datatypes In SQLite (type affinity). https://www.sqlite.org/datatype3.html
[^42]: SQLite — STRICT Tables. https://www.sqlite.org/stricttables.html
[^43]: SQLite — Foreign Key Support (PRAGMA foreign_keys). https://www.sqlite.org/foreignkeys.html
[^44]: SQLite — UPSERT. https://www.sqlite.org/lang_upsert.html
[^45]: SQLite — RETURNING. https://www.sqlite.org/lang_returning.html
[^46]: SQLite — The SQLite Query Optimizer Overview (the LIKE optimization). https://www.sqlite.org/optoverview.html
[^47]: SQLite — Limits In SQLite (SQLITE_MAX_VARIABLE_NUMBER: 999 before 3.32.0, 32766 after). https://www.sqlite.org/limits.html
[^48]: SQLite — ALTER TABLE (supported operations, generalized table rebuild procedure). https://www.sqlite.org/lang_altertable.html
[^49]: SQLite — Date And Time Functions (formats, modifiers, ceiling/floor, 'now' constant within one sqlite3_step() call). https://www.sqlite.org/lang_datefunc.html
[^50]: SQLite — Transaction (BEGIN DEFERRED/IMMEDIATE/EXCLUSIVE); Write-Ahead Logging. https://www.sqlite.org/lang_transaction.html , https://www.sqlite.org/wal.html
[^51]: Microsoft Learn — Maximum capacity specifications for SQL Server (2,100 parameters per stored procedure). https://learn.microsoft.com/en-us/sql/sql-server/maximum-capacity-specifications-for-sql-server
[^52]: OWASP — SQL Injection Prevention Cheat Sheet. https://cheatsheetseries.owasp.org/cheatsheets/SQL_Injection_Prevention_Cheat_Sheet.html
[^53]: SQLite — Quirks, Caveats, and Gotchas (flexible typing, double-quoted string literals, foreign keys off by default). https://www.sqlite.org/quirks.html
[^54]: MySQL 8.4 Reference Manual — CHECK Constraints (enforced from 8.0.16). https://dev.mysql.com/doc/refman/8.4/en/create-table-check-constraints.html
[^55]: Microsoft Learn — Data type precedence (Transact-SQL). https://learn.microsoft.com/en-us/sql/t-sql/data-types/data-type-precedence-transact-sql
[^56]: Microsoft Learn — DATEADD (Transact-SQL) (month overflow returns the last day of the month). https://learn.microsoft.com/en-us/sql/t-sql/functions/dateadd-transact-sql
[^57]: Microsoft Learn — SET LOCK_TIMEOUT (Transact-SQL). https://learn.microsoft.com/en-us/sql/t-sql/statements/set-lock-timeout-transact-sql
[^58]: Microsoft Learn — Primary and foreign key constraints (no automatic index on foreign keys). https://learn.microsoft.com/en-us/sql/relational-databases/tables/primary-and-foreign-key-constraints
[^59]: MySQL 8.4 Reference Manual — The INFORMATION_SCHEMA TABLES Table (TABLE_ROWS estimate may vary by 40–50 %). https://dev.mysql.com/doc/refman/8.4/en/information-schema-tables-table.html
[^60]: SQLite — Isolation In SQLite. https://www.sqlite.org/isolation.html
[^61]: Microsoft Learn — SET ARITHABORT (Transact-SQL) (divide-by-zero ends the query when ANSI_WARNINGS is ON, the default). https://learn.microsoft.com/en-us/sql/t-sql/statements/set-arithabort-transact-sql
[^62]: Microsoft Learn — MSSQLSERVER_1205 (deadlock victim; the victim's transaction is rolled back; rerun it). https://learn.microsoft.com/en-us/sql/relational-databases/errors-events/mssqlserver-1205-database-engine-error
[^63]: Microsoft Learn — CREATE DATABASE, ALTER DATABASE, DROP DATABASE, BACKUP and RESTORE (Transact-SQL) ("isn't allowed in an explicit or implicit transaction"). https://learn.microsoft.com/en-us/sql/t-sql/statements/create-database-transact-sql , https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-database-transact-sql , https://learn.microsoft.com/en-us/sql/t-sql/statements/drop-database-transact-sql , https://learn.microsoft.com/en-us/sql/t-sql/statements/backup-transact-sql , https://learn.microsoft.com/en-us/sql/t-sql/statements/restore-statements-transact-sql
