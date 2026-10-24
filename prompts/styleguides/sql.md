### SQL Optimization Patterns

> **Scope**: SQL in a change whose database dialect is not known: schema migrations, queries embedded in application code (raw strings, query builders, ORM escapes), views, stored routines and seed data. The guide covers what holds in every mainstream relational database — injection safety, NULL semantics, indexing, sargable predicates, pagination, batching, upserts, counting, transactions, lock-safe migrations, data types, time zones and money — and shows **PostgreSQL / MySQL / SQLite / SQL Server** variants side by side wherever syntax or behaviour differs.
> **Dialect stance**: identify the dialect before judging syntax (see "Identify the dialect first"). A statement that is correct on one engine is a syntax error — or, worse, silently different — on another.
> **Verification**: every PostgreSQL example was executed on **PostgreSQL 18.6**, every MySQL example on **MySQL 8.4.13** ("none up to 9.7" claims and the inline `REFERENCES` boundary also on 9.7.4 and 8.0.46), every SQLite example on **SQLite 3.45.1** (Python `sqlite3` module, Ubuntu build); build-dependent SQLite limits and the 3.46+/3.53+ features were checked on the official **SQLite 3.54.1** command-line build from sqlite.org. SQL Server statements were **not executed**; they follow the Microsoft Learn T-SQL reference cited in the footnotes.
> **Note**: This document is self-contained. When a PostgreSQL or MySQL server version is detected, the dialect-specific guide replaces this one; nothing here contradicts those guides.

***

#### Identify the Dialect First

##### Where to look

Find the engine before commenting on syntax. In order of reliability:

1. **Driver and connection code**: `pgx`, `lib/pq`, `psycopg`, `asyncpg`, `node-postgres`, `Npgsql` (PostgreSQL); `go-sql-driver/mysql`, `mysql2`, `PyMySQL`, `mysqlclient`, `MySqlConnector`, `mysql-connector-*` (MySQL/MariaDB); `mattn/go-sqlite3`, `modernc.org/sqlite`, `better-sqlite3`, Python `sqlite3`, `Microsoft.Data.Sqlite` (SQLite); `go-mssqldb`, `tedious`, `pyodbc`/`pymssql`, `Microsoft.Data.SqlClient` (SQL Server). Connection strings: `postgres://`, `mysql://`, `file:…db`, `sqlserver://`, `Server=…;Database=…`.
2. **Infrastructure**: Docker images (`postgres:18`, `mysql:8.4`, `mcr.microsoft.com/mssql/server`), Compose/Helm/Terraform files, CI service containers, Testcontainers code.
3. **ORM / migration configuration**: Django `ENGINE`, SQLAlchemy URL, Hibernate dialect, Prisma `provider`, Doctrine `driver`, Rails `adapter`, EF Core `UseNpgsql`/`UseMySql`/`UseSqlite`/`UseSqlServer`, Flyway/Liquibase/golang-migrate folder names.
4. **Syntax tell-tales** in the SQL itself (not proof — some engines accept several forms):

| Tell-tale | Points to |
| --- | --- |
| `$1, $2` placeholders, `::type` casts, `ILIKE`, `RETURNING`, `SERIAL`/`BIGSERIAL`, `JSONB`, `ON CONFLICT`, `CREATE INDEX CONCURRENTLY`, `$$` function bodies | PostgreSQL (`RETURNING`, `ON CONFLICT` also SQLite) |
| Backtick identifiers, `AUTO_INCREMENT`, `ON DUPLICATE KEY UPDATE`, `ENGINE=InnoDB`, `LIMIT offset, count`, `UNSIGNED`, `TINYINT(1)` | MySQL / MariaDB |
| `INTEGER PRIMARY KEY AUTOINCREMENT`, `PRAGMA`, `?1`/`:name`/`@name` placeholders with no server, `STRICT` tables, `WITHOUT ROWID` | SQLite |
| `[bracketed]` identifiers, `TOP (n)`, `@param`, `NVARCHAR`, `IDENTITY(1,1)`, `GO` batch separators, `OUTPUT inserted.*`, `GETDATE()`, `WITH (NOLOCK)` | SQL Server (T-SQL) |

##### Rules for the reviewer

- **Dialect known**: judge against that engine (and its version, when pinned). Do not ask for "portable" SQL in code that only ever runs on one engine — using `RETURNING` on PostgreSQL or `ON DUPLICATE KEY UPDATE` on MySQL is correct.
- **Dialect unknown**: recommend the standard or portable form, or show the variants side by side. Never suggest dialect-specific syntax as if it were universal.
- **More than one engine** (tests on SQLite, production on PostgreSQL is common): flag every construct whose behaviour differs between them — type affinity, `LIKE` case sensitivity, foreign-key enforcement, date arithmetic, integer division, upsert and `RETURNING` support. Tests that pass on SQLite prove nothing about PostgreSQL semantics.
- **Compatible engines are not the engine**: MariaDB is not MySQL (different JSON, `RETURNING` support, optimizer); Amazon Aurora, CockroachDB, YugabyteDB, TiDB, PlanetScale/Vitess and Azure SQL each publish their own differences. Check their documentation before asserting behaviour.
- **Version matters even within a dialect**: the table below lists the release that introduced each commonly reviewed feature. Say "not available" only when the pinned version proves it.

##### Feature availability (minimum versions)

| Feature | PostgreSQL | MySQL | SQLite | SQL Server |
| --- | --- | --- | --- | --- |
| Window functions | yes | 8.0 | 3.25.0 | yes |
| CTEs / `WITH RECURSIVE` | yes | 8.0 | 3.8.3 | yes (no `RECURSIVE` keyword) |
| Upsert | `ON CONFLICT` 9.5; `MERGE` 15 | `ON DUPLICATE KEY UPDATE` (row alias `AS new` 8.0.19) | `ON CONFLICT` 3.24.0 | `MERGE` |
| Return modified rows | `RETURNING` | — (none, up to 9.7) | `RETURNING` 3.35.0 | `OUTPUT` |
| `OFFSET … FETCH FIRST` | yes | — (`LIMIT` only) | — (`LIMIT` only) | 2012 |
| Identity column (`GENERATED … AS IDENTITY`) | 10 | — (`AUTO_INCREMENT`) | — (`INTEGER PRIMARY KEY`) | — (`IDENTITY(s,i)`) |
| `IS [NOT] DISTINCT FROM` | yes | — (`<=>`) | 3.39.0 (and `IS`/`IS NOT`) | 2022 |
| `NULLS FIRST/LAST` in `ORDER BY` | yes | — | 3.30.0 | — |
| Partial / filtered index | yes | — | 3.8.0 | filtered index |
| Index on expression | yes | functional key part 8.0.13 | 3.9.0 | via indexed computed column |
| `INCLUDE` columns in index | 11 | — | — | yes |
| `FOR UPDATE SKIP LOCKED` | 9.5 | 8.0 | — (no row locks) | `WITH (READPAST)` hint |
| Enforced `CHECK` constraints | yes | 8.0.16 (parsed and ignored before) | yes | yes |
| `UPDATE … FROM` | yes | multi-table `UPDATE … JOIN` | 3.33.0 | yes |
| `ALTER TABLE … DROP COLUMN` | yes | yes | 3.35.0 | yes |
| `RIGHT` / `FULL OUTER JOIN` | yes | `RIGHT` only, no `FULL` | 3.39.0 | yes |
| `concat()` function | yes | yes | 3.44.0 | yes |
| `\|\|` string concatenation | yes | logical OR by default (deprecated)[^39] | yes | 2025 / Azure SQL |

"yes" means available in every release still in support (PostgreSQL 14+, MySQL 8.4/9.7, SQL Server 2017+). PostgreSQL and MySQL boundaries come from their release notes; SQLite boundaries from its release history; SQL Server boundaries from Microsoft Learn "Applies to" notes.[^1][^2][^3][^4][^5][^6]

***

#### Dialect Differences at a Glance

| Topic | PostgreSQL | MySQL (InnoDB) | SQLite | SQL Server |
| --- | --- | --- | --- | --- |
| Placeholders (typical drivers) | `$1` (native); `%s` (psycopg) | `?` | `?`, `?1`, `:name`, `@name`, `$name` | `@p1` / `@name` |
| Identifier quoting | `"Name"`; unquoted folded to lower case | `` `Name` `` (`"Name"` only with `ANSI_QUOTES`) | `"Name"`, also `` ` `` and `[ ]` | `[Name]`, `"Name"` with `QUOTED_IDENTIFIER ON` |
| String concatenation | `\|\|` (NULL → NULL), `concat()` (skips NULL) | `CONCAT()` (any NULL → NULL), `CONCAT_WS()` | `\|\|` (NULL → NULL), `concat()` 3.44+ (skips NULL) | `+`, `CONCAT()` (NULL → `''`), `\|\|` 2025+ |
| `7 / 2` | `3` | `3.5000` (`DIV` for integer division) | `3` | `3` |
| `1 / 0` | error | `NULL` + warning in `SELECT` | `NULL` | error (with the default `ANSI_WARNINGS ON`)[^61] |
| Boolean type | `boolean` | `BOOL` = `TINYINT(1)` | none (integers 0/1; `TRUE`/`FALSE` keywords 3.23+) | `bit` |
| Default `=` on strings | case-sensitive | case- and accent-insensitive (`utf8mb4_0900_ai_ci`) | case-sensitive (`BINARY`) | depends on collation; common defaults are case-insensitive (`_CI_`) |
| `LIKE` | case-sensitive (`ILIKE` for insensitive) | follows column collation | case-insensitive for ASCII only | follows collation |
| NULLs in `ORDER BY … ASC` | last | first | first | first |
| Multiple NULLs in a `UNIQUE` column | allowed (`NULLS NOT DISTINCT` 15+ forbids) | allowed | allowed | **one NULL only** (use a filtered unique index) |
| Default isolation | Read Committed | Repeatable Read | Serializable | Read Committed (locking; Azure SQL Database uses row versioning, RCSI) |
| DDL inside a transaction | transactional (except `CONCURRENTLY` and a few others) | **implicit commit** before and after DDL | transactional | transactional (except `CREATE/ALTER DATABASE`, backup/restore, resumable index builds) |
| Auto-generated key | `GENERATED … AS IDENTITY`, `serial` | `AUTO_INCREMENT` | `INTEGER PRIMARY KEY` (rowid) | `IDENTITY(1,1)`, `SEQUENCE` |
| Fetch generated key | `RETURNING id` | `LAST_INSERT_ID()` / driver `LastInsertId` | `RETURNING id`, `last_insert_rowid()` | `OUTPUT inserted.id`, `SCOPE_IDENTITY()` |
| Row limit | `LIMIT n OFFSET m`, `OFFSET m ROWS FETCH FIRST n ROWS ONLY` | `LIMIT n OFFSET m`, `LIMIT m, n` | `LIMIT n OFFSET m` | `TOP (n)`, `ORDER BY … OFFSET m ROWS FETCH NEXT n ROWS ONLY` |
| `now()` / current time | constant for the whole **transaction** | constant for the **statement** | `'now'` is constant within one `sqlite3_step()` call, UTC text[^49] | `SYSDATETIME()` (local), `SYSUTCDATETIME()` |
| Time-zone-aware type | `timestamptz` (stores UTC) | `TIMESTAMP` (converts via session time zone, 1970–2038) | none (store ISO-8601 UTC text or Unix integers) | `datetimeoffset` |
| Type enforcement | strict | strict with default `sql_mode`; `INSERT IGNORE` downgrades errors | **flexible typing** unless `STRICT` table (3.37+) | strict |
| Foreign keys | enforced | enforced (InnoDB) | **off unless `PRAGMA foreign_keys = ON`** per connection | enforced |

Sources for the SQL Server column: Microsoft Learn pages on `ORDER BY`, `CONCAT`, `||`, division, `bit`, collations, `CREATE INDEX`, isolation levels, transactions, the database/backup statements, `OUTPUT` and `SCOPE_IDENTITY`.[^4][^5][^6][^7][^9][^10][^11][^12][^13][^14][^16][^17][^63]

***

#### Example Schema

The examples use this schema. Types differ per dialect; the shape does not.

```sql
-- PostgreSQL
CREATE TABLE users (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email      text        NOT NULL,
    name       text        NOT NULL,
    status     text        NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE orders (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    bigint        NOT NULL REFERENCES users (id),
    status     text          NOT NULL,
    total      numeric(12,2) NOT NULL,
    created_at timestamptz   NOT NULL
);
CREATE UNIQUE INDEX users_email_key ON users (email);
CREATE INDEX orders_user_id_idx ON orders (user_id);
CREATE INDEX orders_status_created_idx ON orders (status, created_at, id);
```

```sql
-- MySQL
CREATE TABLE users (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    email      VARCHAR(255) NOT NULL,
    name       VARCHAR(255) NOT NULL,
    status     VARCHAR(20)  NOT NULL DEFAULT 'active',
    created_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE KEY users_email_key (email)
);
CREATE TABLE orders (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT        NOT NULL,
    status     VARCHAR(20)   NOT NULL,
    total      DECIMAL(12,2) NOT NULL,
    created_at DATETIME(6)   NOT NULL,
    CONSTRAINT orders_user_fk FOREIGN KEY (user_id) REFERENCES users (id),
    KEY orders_status_created_idx (status, created_at, id)
);
```

```sql
-- SQLite
CREATE TABLE users (
    id         INTEGER PRIMARY KEY,
    email      TEXT NOT NULL,
    name       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;
CREATE TABLE orders (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id),
    status     TEXT    NOT NULL,
    total      INTEGER NOT NULL,   -- minor units (cents)
    created_at TEXT    NOT NULL
) STRICT;
CREATE UNIQUE INDEX users_email_key ON users (email);
CREATE INDEX orders_user_id_idx ON orders (user_id);
CREATE INDEX orders_status_created_idx ON orders (status, created_at, id);
```

***

#### Security: Parameters, Identifiers, Dynamic SQL

##### Values: always bind parameters

Every value that comes from outside the code — request fields, headers, file contents, other services, even rows read back from the database — is bound as a parameter. String concatenation or interpolation into SQL is a **blocking finding**, whatever escaping function is used.[^52]

```python
# BAD: injection — email = "x' OR '1'='1" returns every row
cur.execute(f"SELECT id FROM users WHERE email = '{email}'")
cur.execute("SELECT id FROM users WHERE email = '%s'" % email)

# GOOD: the driver sends the value separately from the statement text
cur.execute("SELECT id FROM users WHERE email = %s", (email,))   # psycopg, PyMySQL
cur.execute("SELECT id FROM users WHERE email = ?", (email,))    # sqlite3, pyodbc
```

```go
// BAD
db.QueryContext(ctx, "SELECT id FROM users WHERE email = '"+email+"'")
// GOOD — placeholder syntax depends on the driver
db.QueryContext(ctx, "SELECT id FROM users WHERE email = $1", email) // pgx, lib/pq
db.QueryContext(ctx, "SELECT id FROM users WHERE email = ?", email)  // mysql, sqlite3
db.QueryContext(ctx, "SELECT id FROM users WHERE email = @p1", email) // go-mssqldb
```

Review points:

- **The placeholder must match the driver.** `?` sent to a PostgreSQL native driver is a syntax error; `$1` sent to MySQL is a syntax error. `'%s'` with quotes around the placeholder breaks psycopg/PyMySQL statements, because the driver adds its own quotes.
- **Client-side interpolation is not parameterization.** Some drivers can emulate prepared statements by escaping values into the text (`interpolateParams=true` in go-sql-driver/mysql, PDO emulation). That is acceptable only with a correct connection character set; never enable a "multi statements" option together with string-built SQL.
- **ORMs are safe until a raw escape hatch is used**: `raw()`, `extra()`, `text()`, `Sequelize.literal`, `whereRaw`, `FromSqlRaw`, `.Where(fmt.Sprintf(...))`. Review those as raw SQL.
- **`LIKE` patterns**: a bound value is still interpreted as a pattern. Escape `%`, `_` and the escape character when the user supplies a substring, and declare `ESCAPE` where the dialect needs it.

```sql
-- PostgreSQL / MySQL: backslash is the default LIKE escape character
SELECT 'a_c' LIKE 'a\_c', 'abc' LIKE 'a\_c';            -- true, false
-- SQLite (no default escape character) and portable form
SELECT 'a_c' LIKE 'a\_c' ESCAPE '\', 'abc' LIKE 'a\_c' ESCAPE '\';   -- 1, 0
```

In MySQL the backslash is also a string-literal escape unless `NO_BACKSLASH_ESCAPES` is set; with bound parameters only the `LIKE` escaping matters.

##### IN lists

A single placeholder cannot carry a list in most drivers: `WHERE id IN (?)` with a list either fails or matches one stringified value.

```python
# BAD: one placeholder for a list
cur.execute("SELECT * FROM orders WHERE user_id IN (?)", (user_ids,))

# GOOD (portable): one placeholder per element, built from a trusted count
marks = ",".join("?" * len(user_ids))
cur.execute(f"SELECT * FROM orders WHERE user_id IN ({marks})", user_ids)
```

```sql
-- PostgreSQL: one array parameter, one statement shape for every list length
PREPARE orders_for_users(bigint[]) AS
    SELECT count(*) FROM users WHERE id = ANY($1);
EXECUTE orders_for_users('{1,2,3}');
```

Bound the list size. Hard limits on parameters per statement: **PostgreSQL 65,535** (16-bit count in the protocol's Bind message), **MySQL 65,535** ("Prepared statement contains too many placeholders", error 1390)[^40], **SQLite 32,766** by default since 3.32.0 (999 before; the official sqlite.org build uses the default, while Debian/Ubuntu system libraries are compiled with 250,000 — a test suite on Ubuntu proves nothing about other builds), **SQL Server 2,100** parameters per stored procedure (client-side parameterized queries are sent as `sp_executesql` calls and hit the same limit).[^29][^47][^51] With psql/libpq on PostgreSQL 18.6, a statement with 65,535 bound parameters runs and 65,536 is rejected ("number of parameters must be between 0 and 65535"); MySQL 8.4 `PREPARE` accepts 65,535 `?` and rejects 65,536 with error 1390. For larger sets, load keys into a temporary table and join, or use an array (`= ANY($1)`) or table-valued parameter.

##### Identifiers: allow-list, then quote

Table names, column names, sort directions and `ORDER BY` columns cannot be parameters. Map user input to a fixed set of identifiers in code; never pass it through.

```python
# BAD: sort column straight from the query string
cur.execute(f"SELECT id, email FROM users ORDER BY {request.args['sort']}")

# GOOD: allow-list
SORTS = {"email": "email", "created": "created_at", "-created": "created_at DESC"}
order_by = SORTS.get(request.args.get("sort"), "id")
cur.execute(f"SELECT id, email FROM users ORDER BY {order_by}, id")
```

##### Dynamic SQL inside the database

Stored routines that build SQL need the same discipline: bind values, quote identifiers with the dialect's function.

```sql
-- PostgreSQL: %I quotes identifiers, %L quotes literals; prefer EXECUTE ... USING for values
SELECT format('SELECT * FROM %I WHERE name = %L', 'my table', 'O''Brien');
-- select * from "my table" where name = 'O''Brien'
```

- PostgreSQL PL/pgSQL: `EXECUTE format('… %I …', tbl) USING value`.
- MySQL: `PREPARE stmt FROM @sql; EXECUTE stmt USING @v;` — values go through `USING`; identifiers must come from an allow-list.
- SQL Server: `sp_executesql N'… WHERE id = @id', N'@id int', @id = @value` for values and `QUOTENAME(@name)` for identifiers; `EXEC('…' + @input)` is injectable.[^20]

##### Least privilege and secrets

- The application account owns no schema and has no `SUPERUSER`/`SUPER`/`sysadmin`/`db_owner` rights; migrations run under a separate role.
- Credentials come from the environment or a secret store, never from source, migrations or seed files.
- Error messages returned to clients must not include SQL text or driver errors.

***

#### NULL Semantics

##### Three-valued logic

`NULL` means "unknown". Any comparison with `NULL` — including `NULL = NULL` — is `NULL`, and `WHERE` keeps only rows whose condition is true.

```sql
-- Portable (PostgreSQL, MySQL, SQLite)
SELECT 1 WHERE NULL = NULL;          -- no rows
SELECT 1 WHERE NULL IS NULL;         -- one row
```

`WHERE col = NULL` and `WHERE col != NULL` are always bugs. Use `IS NULL` / `IS NOT NULL`.

##### `NOT IN` with a nullable subquery returns nothing

If the subquery yields a single `NULL`, `x NOT IN (…)` is never true: no row qualifies. This is a frequent silent bug in "find rows without a match" queries.

```sql
-- Portable — BAD: returns 0 on PostgreSQL, MySQL and SQLite
SELECT count(*) FROM (SELECT 1 AS x UNION ALL SELECT 2 UNION ALL SELECT 3) t
WHERE x NOT IN (SELECT 1 UNION ALL SELECT NULL);

-- GOOD: NOT EXISTS ignores NULLs in the subquery — returns 2
SELECT count(*) FROM (SELECT 1 AS x UNION ALL SELECT 2 UNION ALL SELECT 3) t
WHERE NOT EXISTS (
    SELECT 1 FROM (SELECT 1 AS y UNION ALL SELECT NULL) s WHERE s.y = t.x
);
```

Use `NOT EXISTS` (or `LEFT JOIN … WHERE right.key IS NULL`) for anti-joins. `NOT IN` is safe only when the subquery column is `NOT NULL` — and stays safe only as long as the schema does.[^28]

##### Null-safe equality

To compare two nullable values as equal when both are `NULL` (change detection, optional filters):

| Dialect | Null-safe equal | Null-safe not equal |
| --- | --- | --- |
| PostgreSQL | `a IS NOT DISTINCT FROM b` | `a IS DISTINCT FROM b` |
| MySQL | `a <=> b` | `NOT (a <=> b)` (no `IS DISTINCT FROM` up to 9.7) |
| SQLite | `a IS b`, `a IS NOT DISTINCT FROM b` (3.39+) | `a IS NOT b`, `a IS DISTINCT FROM b` |
| SQL Server | `a IS NOT DISTINCT FROM b` (2022+) | `a IS DISTINCT FROM b` (2022+) |

```sql
-- PostgreSQL / SQLite 3.39+
SELECT NULL IS DISTINCT FROM NULL, 1 IS DISTINCT FROM NULL;   -- false, true
-- MySQL
SELECT NULL <=> NULL, 1 <=> NULL;                             -- 1, 0
```

Portable fallback: `(a = b OR (a IS NULL AND b IS NULL))`.

##### Aggregates and NULL

- `COUNT(*)` counts rows; `COUNT(col)` counts non-NULL values; `AVG(col)` ignores NULLs (it is not `SUM / COUNT(*)`).
- `SUM`, `AVG`, `MIN`, `MAX` over **zero rows** return `NULL`, not 0. Code that reads the result into a non-nullable integer crashes or reports 0 by accident; wrap with `COALESCE(SUM(x), 0)` when 0 is the intended answer.

```sql
-- Portable
SELECT sum(total), count(total), count(*) FROM orders WHERE 1 = 0;   -- NULL, 0, 0
```

##### NULLs in UNIQUE constraints

Most engines treat NULLs as distinct in a unique constraint, so a nullable unique column accepts any number of NULLs. **SQL Server allows at most one NULL** in a unique constraint or unique index.[^11]

```sql
-- PostgreSQL, MySQL and SQLite: both rows are accepted
CREATE TABLE u (a int UNIQUE);
INSERT INTO u VALUES (NULL), (NULL);
SELECT count(*) FROM u;                          -- 2

-- PostgreSQL 15+: treat NULLs as equal
CREATE TABLE u2 (a int UNIQUE NULLS NOT DISTINCT);
INSERT INTO u2 VALUES (NULL);
INSERT INTO u2 VALUES (NULL);                    -- ERROR: duplicate key value
```

```sql
-- SQL Server (not executed): "unique among non-NULL values" needs a filtered index
CREATE UNIQUE INDEX users_external_id_key ON users (external_id) WHERE external_id IS NOT NULL;
```

A "unique per tenant, optional value" rule therefore needs different DDL per engine; check the migration matches the engine.

##### NULL ordering

```sql
-- PostgreSQL: NULLs sort last in ASC, first in DESC
SELECT x FROM (VALUES (1), (NULL), (2)) t(x) ORDER BY x;        -- 1, 2, NULL
-- MySQL and SQLite: NULLs sort first in ASC (SQL Server too: "NULL values are treated as the lowest possible values")
SELECT x FROM (SELECT 1 AS x UNION ALL SELECT NULL UNION ALL SELECT 2) t ORDER BY x;   -- NULL, 1, 2
-- Explicit placement: PostgreSQL and SQLite 3.30+
SELECT x FROM (SELECT 1 AS x UNION ALL SELECT NULL UNION ALL SELECT 2) t ORDER BY x NULLS LAST;
-- MySQL / SQL Server: sort on a NULL flag first
SELECT x FROM (SELECT 1 AS x UNION ALL SELECT NULL UNION ALL SELECT 2) t ORDER BY x IS NULL, x;  -- MySQL
```

In SQL Server write `ORDER BY CASE WHEN x IS NULL THEN 1 ELSE 0 END, x`. Keyset pagination over a nullable sort column breaks in every engine; make sort keys `NOT NULL`.

##### String concatenation with NULL

```sql
-- PostgreSQL
SELECT concat('a', NULL, 'b'), 'a' || NULL || 'b';   -- 'ab', NULL
-- MySQL
SELECT CONCAT('a', NULL, 'b'), CONCAT_WS('-', 'a', NULL, 'b');   -- NULL, 'a-b'
-- SQLite 3.44+
SELECT concat('a', NULL, 'b'), 'a' || NULL || 'b';   -- 'ab', NULL
```

SQL Server: `CONCAT` converts NULL to an empty string; `+` and `||` return NULL (`||` always, `+` with the default `CONCAT_NULL_YIELDS_NULL ON`).[^5][^12] A display name built from optional parts is empty on one engine and fine on another — check which function is used.

***

#### Data Types

##### Choose the narrowest correct type

- Integers for counts and identifiers; `bigint` for primary keys of any table that can grow (a 32-bit key overflows at 2,147,483,647, and changing the type later rewrites the table and every referencing column).
- Exact decimals (`numeric`/`DECIMAL`) for money and quantities that must add up; never `float`/`double`/`real` (see "Money and numeric precision").
- Text with a length limit only when the limit is a business rule; in PostgreSQL `text` and `varchar(n)` perform the same, in MySQL `VARCHAR(n)` length affects index size limits and `VARCHAR` without a length is a syntax error.
- Dates as date/time types, not strings — except in SQLite, which has none (see "Dates, times and time zones").
- Booleans as the dialect's boolean type (`boolean`, `BOOL`/`TINYINT(1)`, `bit`, SQLite integer 0/1 with a `CHECK`).

##### Overflow is an error — except in SQLite

```sql
-- PostgreSQL
SELECT 2147483647::int + 1;            -- ERROR: integer out of range
-- MySQL (strict mode, the default)
CREATE TABLE ti (a INT);
INSERT INTO ti VALUES (2147483648);    -- ERROR 1264: Out of range value for column 'a'
-- SQLite: integer overflow silently becomes a floating-point value
SELECT 9223372036854775807 + 1;        -- 9.223372036854776e+18
```

##### Integer division

`7 / 2` is `3` in PostgreSQL, SQLite and SQL Server (integer operands truncate), but `3.5000` in MySQL.[^13] Ratios and percentages computed as `count_a / count_b` are 0 on three engines.

```sql
-- PostgreSQL
SELECT 7 / 2, 7 / 2.0, 7::numeric / 2;     -- 3, 3.5000000000000000, 3.5000000000000000
-- MySQL
SELECT 7 / 2, 7 DIV 2;                      -- 3.5000, 3
-- SQLite
SELECT 7 / 2, 7 / 2.0;                      -- 3, 3.5
```

Portable: multiply by `1.0` or `CAST(x AS DECIMAL(…))` before dividing, and guard the divisor with `NULLIF(divisor, 0)` — division by zero is an error in PostgreSQL and SQL Server but returns `NULL` in MySQL (`SELECT`) and SQLite.

##### SQLite type affinity

SQLite stores whatever value it is given unless the table is declared `STRICT` (3.37.0+): an `INTEGER` column accepts `'abc'`, a `BOOLEAN` column accepts `'yes'`.[^41][^42]

```sql
-- SQLite
CREATE TABLE loose (n INTEGER, d TEXT);
INSERT INTO loose VALUES ('abc', 5);
SELECT n, typeof(n), d, typeof(d) FROM loose;      -- 'abc', 'text', '5', 'text'

CREATE TABLE strict_t (n INTEGER) STRICT;
INSERT INTO strict_t VALUES ('abc');               -- cannot store TEXT value in INTEGER column
```

Arithmetic treats non-numeric text as 0 without an error (`SELECT 'x' + 1` is `1`). Recommend `STRICT` tables (and `CHECK` constraints for booleans and enumerations) for new SQLite schemas. `STRICT` allows only `INT`, `INTEGER`, `REAL`, `TEXT`, `BLOB` and `ANY` as column types.

##### Implicit conversions between strings and numbers

Comparing a string column with a number converts **every row's value** to a number. Besides preventing index use (see "Sargable predicates"), it changes results: in MySQL `'0000000042' = 42`, `'42abc' = 42` and `'abc' = 0` are all true (with only a 1292 "Truncated incorrect DOUBLE value" warning).[^32]

```sql
-- MySQL: phone is VARCHAR with an index
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE phone = 42;
-- -> Filter: (users.phone = 42)
--     -> Covering index scan on users using users_phone_idx      (every index entry)
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE phone = '0000000042';
-- -> Covering index lookup on users using users_phone_idx (phone='0000000042')
```

PostgreSQL refuses the comparison (`operator does not exist: text = integer`); an untyped bound parameter takes the column's type, which is fine. Bind values with the column's type.

##### Identity columns and sequences

```sql
-- PostgreSQL 10+: prefer identity columns over serial
CREATE TABLE t_id (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, v text);
INSERT INTO t_id (v) VALUES ('a'), ('b') RETURNING id;     -- 1, 2
INSERT INTO t_id (id, v) VALUES (10, 'c');
-- ERROR: cannot insert a non-DEFAULT value into column "id" (use OVERRIDING SYSTEM VALUE)
```

- MySQL: `BIGINT AUTO_INCREMENT PRIMARY KEY`; SQLite: `INTEGER PRIMARY KEY` (an alias of the rowid; `AUTOINCREMENT` only prevents reuse of deleted maximum ids and costs an extra table write); SQL Server: `IDENTITY(1,1)` or a `SEQUENCE`.
- **Generated keys have gaps** in every engine: rolled-back inserts, upserts that end in an update, and cached sequence values all consume numbers. Never use them for invoice numbers or "count of rows" logic.
- **UUIDs**: random UUIDv4 primary keys scatter inserts across a B-tree (and, in MySQL/InnoDB and SQL Server clustered indexes, across the table itself). Prefer time-ordered keys (UUIDv7, ULID) or a `bigint` key with a separate public UUID. Store UUIDs as `uuid` (PostgreSQL), `BINARY(16)` (MySQL), `uniqueidentifier` (SQL Server) or `BLOB`/`TEXT` (SQLite).

##### JSON columns

JSON is for data whose shape varies; columns the application filters, joins or constrains on belong in typed columns. Every engine indexes JSON differently (PostgreSQL GIN on `jsonb`, MySQL generated columns or multi-valued indexes, SQLite expression indexes on `->>`/`json_extract`, SQL Server computed columns). A `WHERE` on a JSON path without such an index is a full scan.

***

#### Constraints and Referential Integrity

The database is the last line of defence; application checks race with each other.

- **`NOT NULL`** on every column that is never legitimately unknown.
- **`UNIQUE`** for every business key the code "checks first" (`SELECT … then INSERT` without a unique constraint creates duplicates under concurrency).
- **`CHECK`** for ranges and enumerations. MySQL enforces `CHECK` only since **8.0.16**; earlier versions parse and ignore it.[^54]
- **Foreign keys** for every reference, with an explicit `ON DELETE` choice.

```sql
-- MySQL 8.0.16+
CREATE TABLE ck (x INT CHECK (x >= 0));
INSERT INTO ck VALUES (-1);    -- ERROR 3819: Check constraint 'ck_chk_1' is violated.
```

##### Foreign keys: engine differences

- **SQLite does not enforce foreign keys** unless every connection runs `PRAGMA foreign_keys = ON` (the default is off; it cannot be changed inside a transaction).[^43]

```sql
-- SQLite
CREATE TABLE parent (id INTEGER PRIMARY KEY);
CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent (id));
INSERT INTO child VALUES (1, 999);   -- accepted: enforcement is off
PRAGMA foreign_keys = ON;
INSERT INTO child VALUES (2, 999);   -- FOREIGN KEY constraint failed
```

- **Index the referencing column.** PostgreSQL does not create an index on the referencing (child) columns; without one, every delete or key update on the parent scans the child table, and joins from parent to child scan too. MySQL/InnoDB creates one automatically if none exists. SQL Server does not create one either ("creating a foreign key constraint doesn't automatically create a corresponding index").[^58]

```sql
-- PostgreSQL: only the primary key index exists on child
CREATE TABLE parent (id int PRIMARY KEY);
CREATE TABLE child (id int PRIMARY KEY, parent_id int REFERENCES parent (id));
SELECT indexname FROM pg_indexes WHERE tablename = 'child';   -- child_pkey
CREATE INDEX child_parent_id_idx ON child (parent_id);
```

- MySQL before 9.0 **silently ignores inline column `REFERENCES`** (`parent_id INT REFERENCES parent(id)`); only a table-level `FOREIGN KEY (…) REFERENCES …` clause creates a constraint. Use the table-level form in MySQL schemas.
- `ON DELETE CASCADE` on large child tables deletes in one transaction and can lock for a long time; deleting in batches from the application is often safer.

***

#### Indexing

##### What a B-tree index does

A B-tree index (the default in every engine) serves equality, ranges, prefix `LIKE 'abc%'` (subject to collation rules), `ORDER BY` on the indexed columns and `MIN`/`MAX`. It does not serve `LIKE '%abc'`, arbitrary expressions on the column, or inequality to a single value (`<>`) efficiently.

##### Composite index column order

An index on `(a, b, c)` is sorted by `a`, then `b`, then `c`. It serves predicates on a **leftmost prefix**: `a`, `a AND b`, `a AND b AND c`, and range scans on the first column after the equalities.

Rule of thumb: **equality columns first, then the one range or sort column, then columns needed only for covering**.

```sql
-- Query: WHERE status = ? AND created_at >= ? AND created_at < ? ORDER BY created_at
-- GOOD: equality column first, then the range/sort column
CREATE INDEX orders_status_created_idx ON orders (status, created_at, id);
-- BAD for this query: range column first — status can only be checked as a filter
CREATE INDEX orders_created_status_idx ON orders (created_at, status);
```

```sql
-- PostgreSQL
EXPLAIN (COSTS OFF)
SELECT id FROM orders
WHERE status = 'paid' AND created_at >= '2024-01-02' AND created_at < '2024-01-03';
-- Index Only Scan using orders_status_created_idx on orders
--   Index Cond: ((status = 'paid'::text) AND (created_at >= ...) AND (created_at < ...))
```

A query that omits the leading column usually cannot seek in the index. Some optimizers can **skip-scan** a leading column with few distinct values (PostgreSQL 18, MySQL 8.0.13+, SQLite after `ANALYZE`); do not rely on it for hot queries — design the index for the query.

Redundant indexes: `(a)` is redundant when `(a, b)` exists (except for uniqueness). Each extra index costs write amplification, storage, cache and vacuum/purge work.

##### Covering indexes

A query answered entirely from the index skips the table lookup ("Index Only Scan" in PostgreSQL, "Covering index" in MySQL and SQLite).

```sql
-- PostgreSQL 11+ and SQL Server: non-key payload columns with INCLUDE
CREATE INDEX orders_user_cover_idx ON orders (user_id) INCLUDE (total);
EXPLAIN (COSTS OFF)
SELECT user_id, sum(total) FROM orders WHERE user_id BETWEEN 1 AND 50 GROUP BY user_id;
-- GroupAggregate
--   ->  Index Only Scan using orders_user_cover_idx on orders

-- MySQL and SQLite: no INCLUDE — append the columns to the key instead
CREATE INDEX orders_user_total_idx ON orders (user_id, total);
```

Notes:

- PostgreSQL's index-only scan still visits the heap for pages not marked all-visible in the visibility map ("Heap Fetches" in `EXPLAIN ANALYZE`); tables with heavy churn need regular vacuuming to benefit.
- InnoDB secondary indexes already contain the primary key, and SQLite indexes contain the rowid: `SELECT id FROM users WHERE email = ?` is covered by an index on `email` alone.

##### Partial (filtered) indexes

Index only the rows the query targets — smaller, cheaper to maintain, and the only way to express "unique among active rows".

```sql
-- PostgreSQL and SQLite 3.8.0+ (SQL Server: filtered index with the same WHERE syntax)
CREATE INDEX users_active_email_idx ON users (email) WHERE status = 'active';
```

The query's `WHERE` must imply the index predicate (same literal, or a parameter the planner can prove matches — in PostgreSQL a generic prepared plan with `status = $1` cannot use an index whose predicate is `status = 'active'`). MySQL has no partial indexes up to 9.7.

##### Expression (functional) indexes

```sql
-- PostgreSQL and SQLite 3.9.0+
CREATE INDEX users_lower_email_idx ON users (lower(email));
-- MySQL 8.0.13+: note the double parentheses
CREATE INDEX users_lower_email_idx ON users ((lower(email)));
```

The query must use the identical expression (`lower(email) = lower(?)`). SQL Server indexes a computed column instead.

##### Other index types (PostgreSQL)

PostgreSQL offers index methods beyond B-tree; the other engines mostly do not:

- **Hash**: equality only.
- **GIN**: `jsonb` containment, arrays, full-text `tsvector`, trigram (`pg_trgm`) for `LIKE '%abc%'`.
- **GiST / SP-GiST**: geometric types, ranges, exclusion constraints, nearest-neighbour.
- **BRIN**: very large, naturally ordered tables (append-only time series); tiny and lossy.

```sql
-- PostgreSQL
CREATE INDEX posts_search_idx ON posts USING gin (to_tsvector('english', title || ' ' || body));
CREATE INDEX events_metadata_idx ON events USING gin (metadata);
```

MySQL offers `FULLTEXT` and `SPATIAL` indexes plus multi-valued indexes on JSON arrays; SQLite offers the FTS5 and R*Tree virtual tables; SQL Server has full-text, columnstore, XML and spatial indexes.

##### When an index does not help

- The query returns a large fraction of the table — a sequential scan is cheaper, and the planner is right to ignore the index.
- Low-selectivity columns alone (`status` with three values) — useful only as the leading part of a composite index or as a partial-index predicate.
- Tiny tables.

***

#### Sargable Predicates

A predicate is *sargable* when the engine can use an index to find matching rows. Wrapping the indexed column in a function, a cast or arithmetic makes it non-sargable.

##### Functions and casts on the column

```sql
-- PostgreSQL — BAD: function on the column → sequential scan
EXPLAIN (COSTS OFF) SELECT id FROM orders WHERE created_at::date = date '2024-01-02';
-- Parallel Seq Scan on orders  Filter: ((created_at)::date = '2024-01-02'::date)

-- GOOD: half-open range on the raw column
EXPLAIN (COSTS OFF) SELECT id FROM orders
WHERE status = 'paid' AND created_at >= '2024-01-02' AND created_at < '2024-01-03';
-- Index Only Scan using orders_status_created_idx on orders
```

```sql
-- MySQL — BAD
EXPLAIN FORMAT=TREE SELECT id FROM orders WHERE date(created_at) = '2024-01-02';
-- -> Filter: (cast(orders.created_at as date) = '2024-01-02')
--     -> Covering index scan on orders using orders_status_created_idx   (all 200k entries)
-- GOOD
EXPLAIN FORMAT=TREE SELECT id FROM orders
WHERE status = 'paid' AND created_at >= '2024-01-02' AND created_at < '2024-01-03';
-- -> Covering index range scan on orders using orders_status_created_idx over (status = 'paid' AND ...)
```

The same applies to `YEAR(col) = 2024`, `col + 1 = ?`, `COALESCE(col, '') = ?`, `UPPER(col) = ?`, `CAST(col AS …)`. Rewrite as a range on the column, or create an expression index that matches the expression exactly:

```sql
-- PostgreSQL
EXPLAIN (COSTS OFF) SELECT id FROM users WHERE lower(email) = 'user5@example.com';
-- Seq Scan on users  Filter: (lower(email) = 'user5@example.com'::text)
CREATE INDEX users_lower_email_idx ON users (lower(email));
ANALYZE users;
EXPLAIN (COSTS OFF) SELECT id FROM users WHERE lower(email) = 'user5@example.com';
-- Index Scan using users_lower_email_idx on users
```

Storing the value already normalized (lower-cased e-mail with a `CHECK (email = lower(email))`) avoids the expression entirely.

##### Type mismatches

A comparison between different types converts one side. When the column side is converted, the index on it is unusable (see the MySQL `phone = 42` example under "Data types"). In SQL Server the operand with the lower data-type precedence is converted: an `nvarchar` parameter compared with a `varchar` column converts the column, and an `int` parameter compared with a `varchar` column converts every value to `int` (failing on non-numeric rows).[^55] Make parameter types match column types in driver code (for example, set the parameter type explicitly instead of letting the driver send every string as `nvarchar`).

##### `LIKE`

- `LIKE 'abc%'` can use a B-tree index; `LIKE '%abc'` and `LIKE '%abc%'` cannot. Use PostgreSQL `pg_trgm` GIN/GiST indexes, MySQL `FULLTEXT`, SQLite FTS5 or SQL Server full-text search for substring search.
- PostgreSQL: a plain B-tree on a `text` column supports `LIKE 'abc%'` only with the `C` collation or a `text_pattern_ops` index.
- **SQLite: `LIKE` is case-insensitive by default, so an ordinary (`BINARY`) index is not used for `LIKE 'abc%'`**; the LIKE optimization needs a `NOCASE` index, or `PRAGMA case_sensitive_like = ON` with a `BINARY` index.[^46]

```sql
-- SQLite
CREATE TABLE e (id INTEGER PRIMARY KEY, email TEXT);
CREATE INDEX e_email ON e (email);
EXPLAIN QUERY PLAN SELECT id FROM e WHERE email LIKE 'a%';
-- SCAN e                                       (full scan, no index search)
EXPLAIN QUERY PLAN SELECT id FROM e WHERE email >= 'a' AND email < 'b';
-- SEARCH e USING COVERING INDEX e_email (email>? AND email<?)
```

##### `OR` across different columns

`WHERE a = ? OR b = ?` cannot use a single composite index. PostgreSQL can combine two indexes (BitmapOr), MySQL can use `index_merge`, SQLite has an OR-by-union optimization when each term is indexed; otherwise rewrite as `UNION ALL` of two indexed queries (deduplicating where needed). Check the plan rather than assuming.

##### Date ranges: half-open, not `BETWEEN`

`BETWEEN '2024-01-02' AND '2024-01-03'` on a timestamp includes rows at exactly midnight of the 3rd, so consecutive daily ranges count those rows twice.

```sql
-- PostgreSQL (orders every 30 seconds)
SELECT count(*) FROM orders WHERE created_at BETWEEN '2024-01-02' AND '2024-01-03';           -- 2881
SELECT count(*) FROM orders WHERE created_at >= '2024-01-02' AND created_at < '2024-01-03';   -- 2880
```

Always use `>= start AND < next_start`.

***

#### Reading Execution Plans

##### Commands per dialect

| Dialect | Estimated plan | Actual execution statistics |
| --- | --- | --- |
| PostgreSQL | `EXPLAIN SELECT …` | `EXPLAIN (ANALYZE, BUFFERS) SELECT …` (18: `BUFFERS` is included by default with `ANALYZE`) |
| MySQL | `EXPLAIN FORMAT=TREE SELECT …` (8.0.16+) | `EXPLAIN ANALYZE SELECT …` (8.0.18+) |
| SQLite | `EXPLAIN QUERY PLAN SELECT …` | — (measure in the application) |
| SQL Server | `SET SHOWPLAN_XML ON` / estimated plan in SSMS | `SET STATISTICS IO, TIME ON`; actual plan; Query Store |

**`EXPLAIN ANALYZE` executes the statement.**[^25][^38] For `INSERT`/`UPDATE`/`DELETE` in PostgreSQL, wrap it in a transaction and roll back:

```sql
-- PostgreSQL
BEGIN;
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, SUMMARY OFF, BUFFERS OFF)
UPDATE users SET name = 'x' WHERE id = 1;
-- Update on users (actual rows=0.00 loops=1)
--   ->  Index Scan using users_pkey on users (actual rows=1.00 loops=1)
SELECT name FROM users WHERE id = 1;      -- 'x' — the update really happened
ROLLBACK;
```

##### What to look for

- **Full scans on large tables in hot paths**: `Seq Scan` (PostgreSQL), `Table scan` / `Covering index scan` over all entries (MySQL), `SCAN t` (SQLite), `Table Scan` / `Clustered Index Scan` (SQL Server). Acceptable for small tables and large-fraction reads.
- **Filter above a scan**: the index found candidates but a condition is evaluated row by row — often a non-sargable predicate or a missing column in the index.
- **Estimated vs actual rows** far apart (×10 or more): stale or missing statistics, correlated columns, or a function hiding the column from statistics. Run `ANALYZE` / `ANALYZE TABLE` / `ANALYZE` (SQLite) / `UPDATE STATISTICS` and re-check.
- **Sorts** on large inputs (`Sort`, `USE TEMP B-TREE FOR ORDER BY`, `filesort`) for queries with `LIMIT` — an index in the `ORDER BY` order turns them into ordered scans.
- **Nested loop with an unindexed inner side**: the inner table is scanned once per outer row.
- **Rows read vs rows returned**: `OFFSET 50000` reads 50,020 rows to return 20 (see "Pagination").
- **Join strategies**: nested loop (few outer rows, indexed inner), hash join (large unsorted inputs; MySQL 8.0.18+), merge join (both sides sorted). None is "good" or "bad" in itself.
- **Costs** are planner units, comparable only between plans of the same query on the same server.

Test plans against realistic data volumes. A plan on a 100-row development table says nothing about production.

***

#### Query Patterns

##### Avoid `SELECT *` where it matters

```sql
-- Portable — avoid in application code on wide or hot tables
SELECT * FROM users WHERE id = 123;
-- Prefer
SELECT id, email, name FROM users WHERE id = 123;
```

This matters most for wide tables (large text/JSON/blob columns travel over the network), hot paths, enabling index-only scans, and keeping the column contract stable (`SELECT *` into a positional row mapping breaks when a column is added). `SELECT *` in an ad-hoc query, an `EXISTS` subquery or a narrow table is not a defect; other examples in this guide use `*` for brevity.

##### Eliminate N+1 queries

One query per parent row multiplies round trips with the result size.

```python
# BAD: 1 + N queries
users = db.query("SELECT id, name FROM users ORDER BY id LIMIT 10")
for user in users:
    user.orders = db.query("SELECT id, total FROM orders WHERE user_id = ?", user.id)
```

```python
# GOOD: one JOIN …
rows = db.query("""
    SELECT u.id, u.name, o.id AS order_id, o.total
    FROM users u
    LEFT JOIN orders o ON o.user_id = u.id
    WHERE u.id IN (SELECT id FROM users ORDER BY id LIMIT 10)
""")
# … or one batch query, grouped in code
users = db.query("SELECT id, name FROM users ORDER BY id LIMIT 10")
ids = [u.id for u in users]
marks = ",".join("?" * len(ids))
orders = db.query(f"SELECT user_id, id, total FROM orders WHERE user_id IN ({marks})", ids)
by_user = {}
for o in orders:
    by_user.setdefault(o.user_id, []).append(o)
```

MySQL rejects `LIMIT` inside an `IN` subquery (error 1235); there, select the ten ids first or join a derived table (`JOIN (SELECT id FROM users ORDER BY id LIMIT 10) u10 ON …`). ORMs hide N+1 behind lazy loading: look for relationship access inside loops and require eager loading (`select_related`/`prefetch_related`, `joinedload`/`selectinload`, `Include`, `preload`, `with`).

##### Prefer explicit JOINs

```sql
-- Avoid: implicit comma join (easy to forget the join predicate)
SELECT u.name, o.total
FROM users u, orders o
WHERE u.id = o.user_id AND u.created_at > '2024-01-01';

-- Prefer: explicit JOIN keeps the join condition visible
SELECT u.name, o.total
FROM users u
JOIN orders o ON o.user_id = u.id
WHERE u.created_at > '2024-01-01';
```

This is a readability and safety preference, not a performance one: planners treat both forms identically. Explicit `JOIN` makes a missing join predicate (accidental cross join) much harder to overlook.

##### Join fan-out and `DISTINCT`

Joining a one-to-many relationship multiplies parent rows; aggregating after the join double-counts (`SUM(u.credit)` across a join with orders). `SELECT DISTINCT` added "to remove duplicates" usually hides such a join error and costs a sort or hash. Use `EXISTS` for "has at least one", aggregate the child side in a subquery before joining, or join to a pre-aggregated derived table.

```sql
-- Portable: users with at least one paid order — no duplicates, stops at the first match
SELECT u.id, u.email
FROM users u
WHERE EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id AND o.status = 'paid');
```

##### Correlated subqueries

A scalar subquery in the select list runs per outer row unless the optimizer decorrelates it. With an index on the inner key that is often fine for a few rows; for many rows, aggregate once and join:

```sql
-- Per-row subquery
SELECT u.name, u.email,
       (SELECT count(*) FROM orders o WHERE o.user_id = u.id) AS order_count
FROM users u;

-- Aggregate once
SELECT u.name, u.email, coalesce(c.order_count, 0) AS order_count
FROM users u
LEFT JOIN (SELECT user_id, count(*) AS order_count FROM orders GROUP BY user_id) c
       ON c.user_id = u.id;
```

Check the plan before demanding a rewrite: PostgreSQL shows the per-row form as a `SubPlan` executed per row; with an index on `orders.user_id` and a selective outer filter it is cheap.

##### `GROUP BY` rules differ

The SQL standard allows selecting a non-aggregated column only if it is functionally dependent on the `GROUP BY` columns.

- **PostgreSQL**: allows columns of a table whose primary key is grouped; rejects everything else.
- **MySQL**: with `ONLY_FULL_GROUP_BY` (default since 5.7.5) it detects functional dependencies like PostgreSQL; sessions or servers that drop the mode return an arbitrary value per group.
- **SQLite**: accepts any "bare" column and returns a value from an arbitrary row of the group (a row matching `min()`/`max()` when exactly one of those is used).[^53]
- **SQL Server**: every non-aggregated column must be listed in `GROUP BY`.

```sql
-- PostgreSQL and MySQL: valid — u.email depends on u.id (the primary key)
SELECT u.id, u.email, count(o.id)
FROM users u LEFT JOIN orders o ON o.user_id = u.id
WHERE u.id < 3
GROUP BY u.id;
-- PostgreSQL and MySQL: rejected — u.email does not depend on u.name
SELECT u.email, count(*) FROM users u WHERE u.id < 3 GROUP BY u.name;
```

For portable code list every non-aggregated column in `GROUP BY`. Code that relies on SQLite's bare columns returns nondeterministic data and fails on other engines.

Keep non-aggregate filters in `WHERE`; reserve `HAVING` for conditions on aggregates:

```sql
-- Works, but mixes concerns: non-aggregate condition in HAVING
SELECT user_id, count(*) AS order_count
FROM orders
WHERE status = 'paid'
GROUP BY user_id
HAVING count(*) > 10 AND user_id > 1000;

-- Clearer: non-aggregate filters in WHERE
SELECT user_id, count(*) AS order_count
FROM orders
WHERE status = 'paid' AND user_id > 1000
GROUP BY user_id
HAVING count(*) > 10;
```

Optimizers typically move non-aggregate predicates on grouping columns from `HAVING` into the scan, so this is a readability preference, not a reportable performance defect.

##### CTEs

CTEs make long queries readable. Their optimization differs:

- PostgreSQL 12+ inlines a non-recursive CTE referenced once unless it is written `AS MATERIALIZED`; before 12 every CTE was an optimization fence.
- MySQL 8.0 merges or materializes derived tables and CTEs by cost.
- SQLite 3.35+ accepts `MATERIALIZED` / `NOT MATERIALIZED` hints.

```sql
-- PostgreSQL: inlined — the id predicate reaches the index
EXPLAIN (COSTS OFF)
WITH r AS (SELECT * FROM users WHERE status = 'active') SELECT * FROM r WHERE id = 5;
-- Index Scan using users_pkey on users  Index Cond: (id = 5)
-- AS MATERIALIZED forces a full evaluation first
EXPLAIN (COSTS OFF)
WITH r AS MATERIALIZED (SELECT * FROM users WHERE status = 'active') SELECT * FROM r WHERE id = 5;
-- CTE Scan on r  Filter: (id = 5)  ->  Seq Scan on users
```

##### Top-N per group: window functions

```sql
-- Portable (PostgreSQL, MySQL 8.0+, SQLite 3.25+, SQL Server): latest order per user
SELECT user_id, id, total
FROM (
    SELECT o.user_id, o.id, o.total,
           row_number() OVER (PARTITION BY o.user_id ORDER BY o.created_at DESC, o.id DESC) AS rn
    FROM orders o
) ranked
WHERE rn = 1;
```

For "latest row of one user", `ORDER BY created_at DESC, id DESC LIMIT 1` with an index on `(user_id, created_at, id)` beats any window function.

##### `UNION` vs `UNION ALL`

`UNION` removes duplicates (sort or hash over the whole result). Use `UNION ALL` unless deduplication is required.

##### Deterministic ordering

Without `ORDER BY`, row order is unspecified in every engine — it can change with plans, parallelism, or data growth. Any query whose consumer depends on order (pagination, "first", UI lists, `LIMIT`, tests comparing lists) needs an `ORDER BY` that ends in a unique column.

***

#### Pagination

##### Syntax per dialect

```sql
-- PostgreSQL: both forms
SELECT x FROM generate_series(1, 10) x ORDER BY x LIMIT 3 OFFSET 2;                         -- 3, 4, 5
SELECT x FROM generate_series(1, 10) x ORDER BY x OFFSET 2 ROWS FETCH FIRST 3 ROWS ONLY;    -- 3, 4, 5
-- MySQL: LIMIT only (FETCH FIRST is a syntax error); LIMIT offset, count is MySQL-only
SELECT id FROM users ORDER BY id LIMIT 3 OFFSET 2;
SELECT id FROM users ORDER BY id LIMIT 2, 3;
-- SQLite: LIMIT only
SELECT id FROM users ORDER BY id LIMIT 3 OFFSET 2;
```

```sql
-- SQL Server 2012+ (not executed): OFFSET/FETCH requires ORDER BY and cannot be combined with TOP
SELECT id FROM users ORDER BY id OFFSET 2 ROWS FETCH NEXT 3 ROWS ONLY;
SELECT TOP (3) id FROM users ORDER BY id;
```

##### OFFSET reads and discards rows

`OFFSET n` produces and throws away `n` rows: page 2,500 of 20 reads 50,020 rows.

```sql
-- PostgreSQL
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, SUMMARY OFF, BUFFERS OFF)
SELECT id FROM orders WHERE status = 'paid' ORDER BY created_at, id LIMIT 20 OFFSET 50000;
-- Limit (actual rows=20.00 loops=1)
--   ->  Index Only Scan using orders_status_created_idx on orders (actual rows=50020.00 loops=1)
```

OFFSET pagination is also unstable: rows inserted or deleted between requests shift pages, so users see duplicates or miss rows. It is fine for small, bounded result sets and admin screens.

##### Keyset (seek) pagination

Remember the sort key of the last row and continue after it. The `ORDER BY` must be deterministic (end with a unique column) and match an index.

```sql
-- PostgreSQL and SQLite 3.15+: row-value comparison is index-friendly
SELECT id, created_at
FROM orders
WHERE status = 'paid' AND (created_at, id) > ('2024-01-10', 13000)
ORDER BY created_at, id
LIMIT 20;
-- PostgreSQL plan: Index Only Scan using orders_status_created_idx
--   Index Cond: ((status = 'paid') AND (ROW(created_at, id) > ROW(...)))
```

**MySQL does not use an index range for row-constructor inequalities** — the same query becomes a filter over all `status = 'paid'` entries, as slow as `OFFSET` on late pages. Write the expanded form:[^31]

```sql
-- MySQL — BAD: filter over the index from its start
EXPLAIN FORMAT=TREE SELECT id FROM orders
WHERE status = 'paid' AND (created_at, id) > ('2024-01-10 00:00:00', 13000)
ORDER BY created_at, id LIMIT 20;
-- -> Filter: ((orders.created_at,orders.id) > ('2024-01-10 00:00:00',13000))
--     -> Covering index lookup on orders using orders_status_created_idx (status='paid')

-- GOOD (portable): expanded predicate → range scan starting at the cursor
EXPLAIN FORMAT=TREE SELECT id FROM orders
WHERE status = 'paid'
  AND (created_at > '2024-01-10 00:00:00'
       OR (created_at = '2024-01-10 00:00:00' AND id > 13000))
ORDER BY created_at, id LIMIT 20;
-- -> Covering index range scan on orders using orders_status_created_idx over (status = 'paid' AND created_at = ... AND 13000 < id) OR (...)
```

SQL Server has no row-value comparison; use the expanded form there too.

Keyset review points:

- The cursor must contain **every** sort column, including the unique tie-breaker; `WHERE created_at > ?` alone skips rows sharing a timestamp.
- Mixed directions (`ORDER BY created_at DESC, id ASC`) cannot use a single row comparison; use the expanded form with matching operators, and an index with matching directions where the engine needs it.
- Sort columns must be `NOT NULL` (NULLs never satisfy `>`).
- Encode cursors opaquely (signed/base64) so clients cannot craft arbitrary predicates — and still bind the decoded values as parameters.

***

#### Counting and Existence Checks

##### Existence: `EXISTS`, not `COUNT`

```sql
-- BAD: counts every match to answer a yes/no question
SELECT count(*) FROM orders WHERE user_id = 7;     -- then: if count > 0
-- GOOD: stops at the first match
SELECT EXISTS (SELECT 1 FROM orders WHERE user_id = 7);
```

`SELECT EXISTS (…)` works in PostgreSQL, MySQL and SQLite; SQL Server needs `SELECT CASE WHEN EXISTS (…) THEN 1 ELSE 0 END` or `IF EXISTS (…)`.

##### `COUNT(*)` on large tables

PostgreSQL and InnoDB keep no exact row count that `COUNT(*)` could read for free (MVCC: each transaction may see a different count); they scan the table or an index. For dashboards and "about N results", use an estimate or a maintained counter:

```sql
-- PostgreSQL: planner estimate (-1 if the table was never vacuumed or analyzed, 14+)
SELECT reltuples::bigint AS estimate FROM pg_class WHERE relname = 'orders';
-- MySQL: InnoDB estimate (can differ from the real count by 40–50 %)
SELECT table_rows FROM information_schema.tables
WHERE table_schema = DATABASE() AND table_name = 'orders';
```

The MySQL manual itself says `TABLE_ROWS` may vary from the actual value by as much as 40 % to 50 % for InnoDB.[^59]

Filtered counts (`WHERE status = ? AND created_at >= ?`) are fast when an index covers the filter columns. "Total pages" for paginated UIs is a common hidden full count; prefer "has next page" (fetch `LIMIT n + 1`).

`COUNT(*)` and `COUNT(1)` are equivalent in every engine here. `COUNT(col)` skips NULLs — a different question, not an optimization.

***

#### Writes: Batching, Upserts, Generated Keys

##### Batch inserts

```sql
-- BAD: one round trip and (in autocommit) one commit per row
INSERT INTO users (email, name) VALUES ('alice@example.com', 'Alice');
INSERT INTO users (email, name) VALUES ('bob@example.com', 'Bob');

-- GOOD: multi-row VALUES (portable), or driver batch APIs inside one transaction
INSERT INTO users (email, name) VALUES
    ('alice@example.com', 'Alice'),
    ('bob@example.com', 'Bob'),
    ('carol@example.com', 'Carol');
```

- Keep each statement under the parameter limit (see "IN lists") and each transaction to a size the engine handles comfortably (thousands to tens of thousands of rows).
- **Many single-row inserts in SQLite without an explicit transaction** each perform a full commit with file syncs; wrapping them in `BEGIN … COMMIT` is the most effective SQLite write optimization.
- Bulk loading: PostgreSQL `COPY … FROM STDIN` (or `\copy` in psql; server-side `COPY … FROM 'file'` reads a file on the database server and needs `pg_read_server_files` or superuser); MySQL `LOAD DATA [LOCAL] INFILE` (`LOCAL` must be enabled on client and server); SQL Server `BULK INSERT` / `bcp` / `SqlBulkCopy`.

##### Upserts: never check-then-insert

```python
# BAD: race — two concurrent requests both see "absent" and both insert
row = db.query_one("SELECT n FROM counters WHERE k = ?", k)
if row is None:
    db.execute("INSERT INTO counters (k, n) VALUES (?, 1)", k)
else:
    db.execute("UPDATE counters SET n = ? WHERE k = ?", row.n + 1, k)
```

Use the engine's atomic upsert on a **unique constraint**:

```sql
-- PostgreSQL 9.5+
INSERT INTO counters (k, n) VALUES ('a', 1)
ON CONFLICT (k) DO UPDATE SET n = counters.n + excluded.n
RETURNING k, n;

-- SQLite 3.24+ (RETURNING 3.35+)
INSERT INTO counters (k, n) VALUES ('a', 1)
ON CONFLICT (k) DO UPDATE SET n = n + excluded.n
RETURNING k, n;

-- MySQL 8.0.19+: row alias; VALUES(col) in the UPDATE part is deprecated (warning 1287)
INSERT INTO counters (k, n) VALUES ('a', 1) AS new
ON DUPLICATE KEY UPDATE n = counters.n + new.n;
```

All three statements are atomic with respect to the unique key; the column after `ON CONFLICT` must name a unique constraint or index.[^21][^30][^44][^45]

```sql
-- SQL Server (not executed): MERGE needs HOLDLOCK (SERIALIZABLE) to be safe against
-- concurrent inserts of the same key, and must end with a semicolon.
MERGE counters WITH (HOLDLOCK) AS t
USING (VALUES (@k, 1)) AS s (k, n) ON t.k = s.k
WHEN MATCHED THEN UPDATE SET t.n = t.n + s.n
WHEN NOT MATCHED THEN INSERT (k, n) VALUES (s.k, s.n);
```

Microsoft documents that, "in some scenarios where unique keys are expected to be both inserted and updated by the `MERGE`", `HOLDLOCK` (a synonym for `SERIALIZABLE`) prevents unique-key violations.[^8] A widely used alternative — a community pattern, not one Microsoft Learn documents — is one transaction with `UPDATE … WITH (UPDLOCK, SERIALIZABLE)` followed by `INSERT` when `@@ROWCOUNT = 0`; it relies on the documented hint semantics (`UPDLOCK` holds update locks until the transaction completes, `SERIALIZABLE` holds range locks so the missing key cannot be inserted concurrently).[^18]

Engine-specific upsert traps:

- **PostgreSQL** `ON CONFLICT` requires a conflict target for `DO UPDATE` (`ON CONFLICT DO UPDATE` without `(k)` is an error) and the target must match a unique index or constraint exactly (including a partial index's predicate). `MERGE` (15+) follows the normal isolation rules, so a concurrent insert of the same key can still raise a unique violation; the manual points to `INSERT … ON CONFLICT` as the alternative that can update when a concurrent insert occurs.[^21][^22]
- **MySQL** `ON DUPLICATE KEY UPDATE` fires on **any** unique key, not just the one the code has in mind — a table with two unique keys can update the wrong row. Affected-rows is 1 for an insert, 2 for an update, 0 for an unchanged row; drivers with `clientFoundRows` report differently.[^30] `INSERT IGNORE` and `REPLACE` are not upserts: `IGNORE` also turns data errors (truncation, `NULL` into `NOT NULL`) into warnings, and `REPLACE` deletes and re-inserts (new auto-increment id, cascading deletes).
- **Generated keys are consumed** by upserts that end in an update or do nothing, in PostgreSQL (sequence) and MySQL (`AUTO_INCREMENT`):

```sql
-- PostgreSQL
CREATE TABLE s (id int GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, k text UNIQUE);
INSERT INTO s (k) VALUES ('x');
INSERT INTO s (k) VALUES ('x') ON CONFLICT (k) DO NOTHING;
INSERT INTO s (k) VALUES ('y') RETURNING id;                 -- 3: id 2 was consumed
```

High-frequency upserts on mostly existing keys can exhaust an `int` key years early.

##### Returning generated keys and modified rows

```sql
-- PostgreSQL; SQLite 3.35+
INSERT INTO users (email, name) VALUES ('dave@example.com', 'Dave') RETURNING id, created_at;
UPDATE orders SET status = 'paid' WHERE id = 42 RETURNING id, status;

-- MySQL: no RETURNING (syntax error up to 9.7)
INSERT INTO users (email, name) VALUES ('dave@example.com', 'Dave');
SELECT LAST_INSERT_ID();
```

```sql
-- SQL Server (not executed): OUTPUT goes before VALUES / WHERE
INSERT INTO users (email, name) OUTPUT inserted.id VALUES (@email, @name);
UPDATE orders SET status = 'paid' OUTPUT inserted.id, deleted.status AS old_status WHERE id = @id;
```

- MySQL `LAST_INSERT_ID()` after a **multi-row** insert returns the id of the **first** inserted row; it is per connection (safe under concurrency) but meaningless across pooled connections. Do not compute the other ids by adding offsets unless `innodb_autoinc_lock_mode` guarantees consecutive values for that statement type.

```sql
-- MySQL
INSERT INTO s (k) VALUES ('p'), ('q'), ('r');
SELECT LAST_INSERT_ID();      -- id of 'p'
```

- SQL Server: use `SCOPE_IDENTITY()` or `OUTPUT`, never `@@IDENTITY`, which returns an identity generated by a trigger on another table. `OUTPUT` without `INTO` is rejected when the target table has an enabled trigger for that action.[^7][^9]
- Never read the new id with `SELECT max(id)` — it races with concurrent inserts.

##### Batch updates and deletes

Update many rows with one statement, not a loop:

```sql
-- BAD: one statement per id
UPDATE users SET status = 'active' WHERE id = 1;
UPDATE users SET status = 'active' WHERE id = 2;
-- GOOD
UPDATE users SET status = 'active' WHERE id IN (1, 2, 3, 4, 5);
```

Per-row new values: load them into a (temporary) table and join. The `UPDATE … FROM` syntax differs:

```sql
-- PostgreSQL
CREATE TEMP TABLE temp_user_updates (id bigint PRIMARY KEY, new_status text NOT NULL);
INSERT INTO temp_user_updates VALUES (1, 'inactive'), (2, 'inactive');
UPDATE users u SET status = t.new_status FROM temp_user_updates t WHERE u.id = t.id;

-- MySQL: multi-table UPDATE (VARCHAR needs a length)
CREATE TEMPORARY TABLE temp_user_updates (id BIGINT PRIMARY KEY, new_status VARCHAR(20) NOT NULL);
INSERT INTO temp_user_updates VALUES (1, 'inactive'), (2, 'inactive');
UPDATE users u JOIN temp_user_updates t ON u.id = t.id SET u.status = t.new_status;

-- SQLite 3.33+
CREATE TEMP TABLE temp_user_updates (id INTEGER PRIMARY KEY, new_status TEXT NOT NULL);
INSERT INTO temp_user_updates VALUES (1, 'inactive'), (2, 'inactive');
UPDATE users SET status = t.new_status FROM temp_user_updates t WHERE users.id = t.id;
```

SQL Server: `UPDATE u SET u.status = t.new_status FROM users u JOIN #temp_user_updates t ON u.id = t.id;`.

**Large deletes and updates run in chunks** with a commit per chunk: one huge statement holds locks for its whole duration, bloats the transaction log/undo, delays replicas and, in PostgreSQL, leaves dead tuples for vacuum.

```sql
-- PostgreSQL: DELETE has no LIMIT — select a batch of keys
WITH batch AS (
    SELECT id FROM orders
    WHERE status = 'refunded' AND created_at < '2024-01-02'
    ORDER BY id LIMIT 1000
)
DELETE FROM orders o USING batch WHERE o.id = batch.id;

-- MySQL: DELETE ... ORDER BY ... LIMIT (single-table only)
DELETE FROM orders WHERE status = 'refunded' AND created_at < '2024-01-02' ORDER BY id LIMIT 1000;

-- SQLite: DELETE ... LIMIT needs a build compiled with SQLITE_ENABLE_UPDATE_DELETE_LIMIT (Debian/Ubuntu
-- system libraries have it; the official sqlite.org builds and amalgamation-based copies cannot) — use a subquery
DELETE FROM orders WHERE id IN (
    SELECT id FROM orders WHERE status = 'refunded' AND created_at < '2024-01-02' ORDER BY id LIMIT 1000
);
```

SQL Server: `DELETE TOP (1000) FROM orders WHERE …;` in a loop until `@@ROWCOUNT = 0`. Repeat each batch until it affects 0 rows; make sure the filter is indexed so each batch does not rescan the table.

***

#### Transactions, Isolation and Locking

##### Defaults differ

| Dialect | Default isolation | Notes |
| --- | --- | --- |
| PostgreSQL | Read Committed | Each statement sees a fresh snapshot; `REPEATABLE READ` and `SERIALIZABLE` raise serialization failures (SQLSTATE `40001`) that the client must retry |
| MySQL InnoDB | Repeatable Read | Plain `SELECT` reads a snapshot from the first read; `UPDATE`/`DELETE`/locking reads see the latest committed data and take next-key (gap) locks |
| SQLite | Serializable | One writer at a time per database file; readers do not block the writer in WAL mode[^50] |
| SQL Server | Read Committed | Shared locks by default (readers block writers); with `READ_COMMITTED_SNAPSHOT ON` (default on Azure SQL Database) statements read row versions |

Sources: engine manuals.[^10][^26][^37][^60]

##### Lost updates: read-modify-write

```python
# BAD: two concurrent requests read 100, both write 90 — one withdrawal is lost
balance = db.query_one("SELECT balance FROM accounts WHERE id = ?", aid)
db.execute("UPDATE accounts SET balance = ? WHERE id = ?", balance - 10, aid)
```

Fixes, in order of preference:

1. **Atomic statement**: `UPDATE accounts SET balance = balance - 10 WHERE id = ? AND balance >= 10` and check the affected row count.
2. **Pessimistic lock**: `SELECT … FOR UPDATE` inside a transaction (PostgreSQL, MySQL; SQL Server `WITH (UPDLOCK, ROWLOCK)`; SQLite `BEGIN IMMEDIATE` locks the whole database for writing).
3. **Optimistic concurrency**: a `version` column, `UPDATE … SET version = version + 1 WHERE id = ? AND version = ?`, retry on 0 rows.
4. **`SERIALIZABLE` isolation** with a retry loop.

##### Work queues: `SKIP LOCKED`

```sql
-- PostgreSQL 9.5+ and MySQL 8.0+: each worker claims rows nobody else holds
BEGIN;   -- MySQL: START TRANSACTION
SELECT id FROM jobs WHERE state = 'queued' ORDER BY id LIMIT 2 FOR UPDATE SKIP LOCKED;
-- … process, UPDATE jobs SET state = 'done' WHERE id IN (…)
COMMIT;
```

Verified with two sessions: while session A holds ids 1 and 2, session B's identical query returns 3 and 4; `FOR UPDATE NOWAIT` on a locked row fails immediately (PostgreSQL "could not obtain lock on row", MySQL error 3572). SQL Server uses `WITH (UPDLOCK, READPAST, ROWLOCK)` hints, or `DELETE TOP (1) … WITH (READPAST) OUTPUT deleted.*` for destructive reads.[^7][^18] SQLite has no row locks; a single process or `BEGIN IMMEDIATE` serializes claimers.

##### Deadlocks and retries

- Access rows and tables in a consistent order (sort ids before locking a batch).
- Keep transactions short; **never hold a transaction open across network calls**, user think time, message publishing or file I/O.
- Retry the whole transaction on deadlock and serialization errors: PostgreSQL SQLSTATE `40P01` (deadlock) and `40001` (serialization failure); MySQL errors 1213 (deadlock; the transaction was rolled back) and 1205 (lock wait timeout; by default only the statement is rolled back); SQL Server error 1205 (deadlock victim: the victim's transaction is rolled back and must be rerun)[^62]; SQLite `SQLITE_BUSY` — set a busy timeout and use `BEGIN IMMEDIATE` for transactions that will write.
- In SQL Server, `SET XACT_ABORT ON` makes run-time errors roll back the whole transaction instead of leaving it open; nested `BEGIN TRAN` is not a real nested transaction — an inner `ROLLBACK` rolls back the outermost transaction.[^17]

##### Autocommit and the current time

- Without `BEGIN`, every statement commits on its own in PostgreSQL, MySQL, SQLite and SQL Server; multi-statement business operations must be wrapped explicitly. Driver/ORM defaults vary (Python's `sqlite3` and DB-API drivers open transactions implicitly; JDBC is autocommit by default).
- PostgreSQL `now()`/`CURRENT_TIMESTAMP` return the **transaction** start time — the same value for every statement in the transaction; `clock_timestamp()` is wall time.[^27] MySQL `NOW()` is the **statement** start time; `SYSDATE()` is wall time.[^35]

```sql
-- PostgreSQL
BEGIN;
SELECT now() = transaction_timestamp();                   -- true
SELECT pg_sleep(0.1);
SELECT now() = transaction_timestamp(), clock_timestamp() > now();   -- true, true
COMMIT;
```

##### Lock timeouts

Set a lock wait limit for interactive paths and migrations, so a blocked statement fails fast instead of queueing everything behind it:

```sql
-- PostgreSQL
SET lock_timeout = '500ms';
-- MySQL: row locks / metadata (DDL) locks, in seconds
SET SESSION innodb_lock_wait_timeout = 1;
SET SESSION lock_wait_timeout = 1;
```

SQL Server: `SET LOCK_TIMEOUT 500;` (milliseconds; the default `-1` waits forever).[^57] Statement time limits: PostgreSQL `statement_timeout`, MySQL `max_execution_time` (read-only `SELECT`s), SQL Server client command timeouts.

***

#### Schema Migrations Without Outages

##### DDL and transactions

- **PostgreSQL, SQLite and SQL Server** run most DDL inside transactions: a failed migration rolls back completely. PostgreSQL exceptions include `CREATE INDEX CONCURRENTLY`, `DROP INDEX CONCURRENTLY`, `REINDEX … CONCURRENTLY`, `VACUUM`, `CREATE DATABASE` and `ALTER TYPE … ADD VALUE` before 12. SQL Server cannot run `CREATE/ALTER/DROP DATABASE`, `BACKUP` or `RESTORE` inside an explicit or implicit transaction, nor `RESUMABLE = ON` index operations inside an explicit transaction.[^11][^63]
- **MySQL commits implicitly before and after every DDL statement**: a migration with several statements that fails halfway leaves the schema half-migrated, and a `ROLLBACK` after DDL does not undo earlier DML.[^33]

```sql
-- PostgreSQL / SQLite: DDL rolls back
BEGIN; CREATE TABLE ddl_tx (x int); ROLLBACK;
-- the table does not exist afterwards

-- PostgreSQL: some DDL refuses to run in a transaction block
BEGIN; CREATE INDEX CONCURRENTLY users_name_idx ON users (name); ROLLBACK;
-- ERROR: CREATE INDEX CONCURRENTLY cannot run inside a transaction block

-- MySQL: CREATE TABLE commits the open transaction
CREATE TABLE tx (x INT);
START TRANSACTION;
INSERT INTO tx VALUES (1);
CREATE TABLE tx2 (y INT);   -- implicit COMMIT here
ROLLBACK;
SELECT count(*) FROM tx;    -- 1
```

Migration tools that wrap each file in a transaction (Flyway, golang-migrate, Alembic, Rails) behave differently per engine: on MySQL, keep one DDL statement per migration so a failure is easy to resume.

##### Principles that hold everywhere

1. **Expand, migrate, contract.** Add new structures (nullable column, new table, new index) → deploy code that writes both → backfill in batches → switch reads → drop the old structure in a later release. Never rename or drop a column that the currently deployed code still reads; during a rolling deploy old and new code run together.
2. **Know which statements rewrite or lock the table.** Adding a column with a constant default is metadata-only in PostgreSQL 11+ and `INSTANT` in MySQL 8.0.12+ (any column position from 8.0.29; that release was withdrawn, so 8.0.30+); changing a column's type usually rewrites the table everywhere.
3. **Build indexes online** on large, busy tables:

| Dialect | Online index build |
| --- | --- |
| PostgreSQL | `CREATE INDEX CONCURRENTLY` (outside a transaction; a failed build leaves an `INVALID` index to drop)[^23] |
| MySQL | `ALTER TABLE … ADD INDEX …, ALGORITHM=INPLACE, LOCK=NONE` (InnoDB secondary indexes are built in place, permitting concurrent DML)[^34] |
| SQL Server | `CREATE INDEX … WITH (ONLINE = ON[, RESUMABLE = ON])` — online index operations are not available in every edition[^11] |
| SQLite | none needed for small files; any write locks the whole database file |

4. **Add constraints without a long validation lock**: PostgreSQL `ADD CONSTRAINT … NOT VALID` then `VALIDATE CONSTRAINT` in a separate statement (validation takes a weaker lock that permits reads and writes).[^24]

```sql
-- PostgreSQL
ALTER TABLE orders ADD CONSTRAINT orders_total_nonneg CHECK (total >= 0) NOT VALID;
ALTER TABLE orders VALIDATE CONSTRAINT orders_total_nonneg;
```

5. **Set a lock timeout for DDL.** Even metadata-only DDL needs an exclusive lock for an instant; if a long transaction holds a conflicting lock, the DDL waits — and every new query queues behind the waiting DDL. Use `SET lock_timeout` (PostgreSQL) or `SET SESSION lock_wait_timeout` (MySQL) and retry.

```sql
-- MySQL: another session holds an open transaction that touched jobs
SET SESSION lock_wait_timeout = 1;
ALTER TABLE jobs ADD COLUMN note TEXT;
-- ERROR 1205 (HY000): Lock wait timeout exceeded; try restarting transaction
```

6. **Adding `NOT NULL` to an existing column** scans the table to validate it (PostgreSQL 12+ skips the scan when a validated `CHECK (col IS NOT NULL)` exists; PostgreSQL 18 also accepts `ADD CONSTRAINT name NOT NULL col NOT VALID` followed by `VALIDATE CONSTRAINT` — a syntax error on 17; MySQL rebuilds the table). Backfill first, then add the constraint.
7. **SQLite `ALTER TABLE` is limited**: it supports `RENAME TO`, `RENAME COLUMN` (3.25.0+), `ADD COLUMN` and `DROP COLUMN` (3.35.0+); **3.53.0+** adds `ALTER COLUMN col SET NOT NULL` / `DROP NOT NULL`, `ADD CONSTRAINT name CHECK (…)` and `DROP CONSTRAINT name` (verified on 3.54.1; syntax errors on older versions). Changing a type, a key, a default or a foreign key still requires the documented "create new table, copy, drop, rename" procedure inside a transaction with foreign keys disabled.[^48]
8. **Backfills run in batches** with a commit per batch (see "Batch updates and deletes"), not as one `UPDATE big_table SET new_col = …`.

***

#### Dates, Times and Time Zones

##### Store instants in UTC with a time-zone-aware type

| Dialect | Instant (point in time) | Local date-time (no zone) | Notes |
| --- | --- | --- | --- |
| PostgreSQL | `timestamptz` (stored as UTC, displayed in the session `TimeZone`) | `timestamp` | `timestamp` for instants is a common bug: the zone is lost[^28] |
| MySQL | `TIMESTAMP` (converted from/to the session `time_zone`; 1970-01-01 to 2038-01-19 UTC) or `DATETIME` holding UTC by convention | `DATETIME` | `TIMESTAMP` overflows in 2038; `DATETIME` has no zone — the application must fix the convention[^36] |
| SQLite | `TEXT` ISO-8601 in UTC (`2024-01-10T08:00:00Z`) or `INTEGER` Unix time | `TEXT` | no date type; date functions work on text, real (Julian day) or integer[^49] |
| SQL Server | `datetimeoffset` | `datetime2` | prefer `datetime2` over the legacy `datetime` |

```sql
-- MySQL: TIMESTAMP shifts with the session time zone, DATETIME does not
CREATE TABLE tz (ts TIMESTAMP, dt DATETIME);
SET time_zone = '+00:00';
INSERT INTO tz VALUES ('2024-06-01 12:00:00', '2024-06-01 12:00:00');
SET time_zone = '+02:00';
SELECT ts, dt FROM tz;      -- 2024-06-01 14:00:00, 2024-06-01 12:00:00
```

```sql
-- PostgreSQL: timestamptz is displayed in the session time zone
SET timezone = 'Europe/Berlin';
SELECT timestamptz '2024-03-31 01:30:00+00';    -- 2024-03-31 03:30:00+02
RESET timezone;
```

Review points:

- Convert to a user's local time zone at the edge (presentation), not in stored data. Store the user's IANA zone name (`Europe/Berlin`) when local-time rules matter (recurring meetings, "every day at 09:00").
- Connection/session time zones differ between environments (container `UTC`, developer laptop local). Code that depends on the session zone — `timestamp`/`DATETIME` without explicit conversion, `CURRENT_DATE` used as "today" for a user, MySQL `TIMESTAMP` — behaves differently per environment. Set the session zone explicitly (`SET TIME ZONE 'UTC'`, `SET time_zone = '+00:00'`, driver options such as `loc=UTC`/`parseTime=true` in go-sql-driver/mysql).
- SQLite text timestamps compare as strings: mixing formats breaks ordering and range filters.

```sql
-- SQLite: 'T' sorts after ' ', so the earlier instant compares as greater
SELECT '2024-01-10 09:00:00' < '2024-01-10T08:00:00';     -- 1 (true)
```

Pick one format (strftime `'%Y-%m-%dT%H:%M:%fZ'` or integer epoch) and enforce it with a `CHECK`.

##### Date arithmetic differs at month ends

```sql
-- PostgreSQL
SELECT date '2024-01-31' + interval '1 month';         -- 2024-02-29 00:00:00
-- MySQL: INTERVAL n UNIT, no quotes; '30 days' is a syntax error
SELECT CAST('2024-01-31' AS DATE) + INTERVAL 1 MONTH;  -- 2024-02-29
SELECT now() - INTERVAL 30 DAY;
-- SQLite: overflow is normalized forward by default ("ceiling")
SELECT date('2024-01-31', '+1 month');                 -- 2024-03-02
```

SQL Server `DATEADD(month, 1, '2024-08-31')` returns the last day of the target month (`2024-09-30`).[^56] `NOW() - INTERVAL '30 days'` (PostgreSQL syntax) appears in many snippets; it fails on MySQL and SQLite. SQLite 3.46+ adds `floor`/`ceiling` modifiers to choose the behaviour.

##### "Today" and half-open ranges

Compute day boundaries in the user's zone in the application, pass them as parameters and filter `created_at >= :start AND created_at < :end` — sargable and correct across DST changes.

***

#### Money and Numeric Precision

```sql
-- Binary floating point cannot represent 0.1 exactly
SELECT 0.1::float8 + 0.2::float8 = 0.3::float8, 0.1::numeric + 0.2 = 0.3;   -- PostgreSQL: false, true
SELECT 0.1e0 + 0.2e0 = 0.3e0, 0.1 + 0.2 = 0.3;                               -- MySQL: 0, 1
SELECT 0.1 + 0.2 = 0.3;                                                      -- SQLite: 0 (REAL)
```

- Use `numeric(p, s)` / `DECIMAL(p, s)` for currency, with enough precision for totals (e.g. `numeric(19, 4)` or the currency's minor unit), and store the currency code alongside.
- **SQLite has no decimal type**: `NUMERIC`/`DECIMAL` columns store `REAL` when the value has a fractional part. Store money as `INTEGER` minor units (cents) and format in the application.
- **PostgreSQL `money`** depends on the server's `lc_monetary` locale for input and output and rounds to the locale's fraction digits; avoid it.[^28]
- **Rounding on insert is silent**: `DECIMAL(10,2)` stores `12.345` as `12.35` in MySQL (a note, not an error) and PostgreSQL; round explicitly in the business layer where the rounding rule matters.
- Never compare floating-point values with `=`; never use `float` for ids or counters.
- Aggregate money with `SUM` over decimal columns; `AVG` on integer columns returns a decimal in PostgreSQL and MySQL but `REAL` in SQLite.

***

#### Strings, Collations and Case Sensitivity

##### Equality and `LIKE`

```sql
-- PostgreSQL: case-sensitive; ILIKE for case-insensitive matching
SELECT 'abc' = 'ABC', 'abc' ILIKE 'ABC', 'Abc' LIKE 'a%';                 -- false, true, false
-- MySQL (utf8mb4_0900_ai_ci, the 8.0+ default): case- and accent-insensitive, NO PAD
SET NAMES utf8mb4;
SELECT 'abc' = 'ABC', 'abc' = 'abc ', 'résumé' = 'resume';                -- 1, 0, 1
-- SQLite: = is case-sensitive; LIKE is case-insensitive for ASCII only; NOCASE is ASCII-only
SELECT 'abc' = 'ABC', 'abc' LIKE 'ABC', 'é' LIKE 'É', 'abc' = 'ABC' COLLATE NOCASE;   -- 0, 1, 0, 1
```

SQL Server: case and accent sensitivity come from the column or database collation; the default for an "English (United States)" installation is `SQL_Latin1_General_CP1_CI_AS` (case-insensitive, accent-sensitive).[^16]

Review consequences:

- **A unique index enforces the collation's notion of equality.** In MySQL, `abc` and `ABC` collide in a unique `VARCHAR` column with the default collation; in PostgreSQL and SQLite they do not. Case-insensitive uniqueness of e-mails or usernames needs a unique index on `lower(col)` (PostgreSQL, SQLite), a case-insensitive collation (MySQL default, PostgreSQL nondeterministic ICU collation, SQL Server `_CI_`), or a normalized stored value.

```sql
-- MySQL
CREATE TABLE c3 (s VARCHAR(20) UNIQUE);
INSERT INTO c3 VALUES ('abc');
INSERT INTO c3 VALUES ('ABC');     -- ERROR 1062: Duplicate entry 'ABC' for key 'c3.s'
```

- Case-insensitive lookups via `lower(col) = lower(?)` need an expression index (see "Sargable predicates").
- Code moving between engines changes semantics silently: a login lookup that is case-insensitive on MySQL becomes case-sensitive on PostgreSQL.

##### Length functions

| Dialect | Characters | Bytes |
| --- | --- | --- |
| PostgreSQL | `length()`, `char_length()` | `octet_length()` |
| MySQL | `CHAR_LENGTH()` | **`LENGTH()`** |
| SQLite | `length()` (characters for text) | `octet_length()` (3.43+) |
| SQL Server | `LEN()` (**excludes trailing spaces**)[^15] | `DATALENGTH()` |

```sql
-- MySQL (connection character set utf8mb4)
SELECT LENGTH('héllo'), CHAR_LENGTH('héllo');      -- 6, 5
```

Validation such as `LENGTH(name) <= 50` in MySQL limits bytes, not characters.

##### Character sets

MySQL's `utf8` is the 3-byte `utf8mb3` and cannot store emoji or many CJK characters — use `utf8mb4` everywhere, including the connection character set. SQL Server `varchar` stores non-Unicode text unless a `_UTF8` collation is used; use `nvarchar` (and `N'…'` literals) for Unicode.

##### Identifier quoting and case folding

- PostgreSQL folds unquoted identifiers to lower case: `CREATE TABLE "Users"` must be queried as `"Users"` forever. Use lower-case snake_case and never quote.
- MySQL table-name case sensitivity depends on the file system and `lower_case_table_names` (case-sensitive on Linux by default, insensitive on Windows and macOS).
- Double quotes are **identifiers** in standard SQL (PostgreSQL, SQLite, SQL Server with `QUOTED_IDENTIFIER ON`), but **strings** in MySQL unless `ANSI_QUOTES` is set. SQLite also accepts a double-quoted string as a literal when no such column exists — a typo in a column name silently becomes a string constant.[^53] Use single quotes for strings in every dialect.

***

#### Dialect-specific Features

##### Materialized views (PostgreSQL)

```sql
-- PostgreSQL
CREATE MATERIALIZED VIEW user_order_summary AS
SELECT u.id, u.name,
       count(o.id)       AS total_orders,
       sum(o.total)      AS total_spent,
       max(o.created_at) AS last_order_date
FROM users u
LEFT JOIN orders o ON o.user_id = u.id
GROUP BY u.id, u.name;

CREATE UNIQUE INDEX user_order_summary_id_key ON user_order_summary (id);   -- needed for CONCURRENTLY
CREATE INDEX user_order_summary_spent_idx ON user_order_summary (total_spent DESC);

REFRESH MATERIALIZED VIEW user_order_summary;               -- blocks readers while it runs
REFRESH MATERIALIZED VIEW CONCURRENTLY user_order_summary;  -- readers continue; needs the unique index
```

A materialized view is a snapshot: data is stale between refreshes, and refreshing is a full recomputation. MySQL and SQLite have none (use summary tables maintained by the application or triggers); SQL Server has indexed views, maintained synchronously on every write with many restrictions.

##### Partitioning

```sql
-- PostgreSQL declarative partitioning
CREATE TABLE events (
    id         bigint GENERATED ALWAYS AS IDENTITY,
    user_id    bigint      NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (id, created_at)             -- unique keys must include the partition key
) PARTITION BY RANGE (created_at);

CREATE TABLE events_2024_q1 PARTITION OF events FOR VALUES FROM ('2024-01-01') TO ('2024-04-01');
CREATE TABLE events_2024_q2 PARTITION OF events FOR VALUES FROM ('2024-04-01') TO ('2024-07-01');

EXPLAIN (COSTS OFF) SELECT * FROM events
WHERE created_at >= '2024-02-01' AND created_at < '2024-03-01';
-- Bitmap Heap Scan on events_2024_q1 events   (only the matching partition is scanned)
```

Partitioning helps retention (drop or detach old partitions instead of mass deletes) and queries that always filter on the partition key. It hurts queries that do not: they scan every partition. Every unique constraint must include the partition key (PostgreSQL and MySQL), so "globally unique email" is not enforceable on a table partitioned by date. Inserts for a range without a partition fail unless a `DEFAULT` partition exists — partitions must be created ahead of time.

##### Optimizer hints

- **MySQL**: index hints (`USE INDEX` restricts the candidates; `FORCE INDEX` makes a table scan very expensive so the index is used if possible; `IGNORE INDEX`) and optimizer hints in `/*+ … */` comments (`/*+ MAX_EXECUTION_TIME(1000) */`, `/*+ INDEX(t idx) */` in 8.0.20+).

```sql
-- MySQL
SELECT id FROM users FORCE INDEX (users_email_key) WHERE email = 'user@example.com';
```

- **PostgreSQL** has no query hints. Planner settings (`SET enable_nestloop = off`, `SET max_parallel_workers_per_gather = 4`) apply to the whole session until reset — use `SET LOCAL` inside a transaction to scope them, and treat them as diagnostics, not fixes. The `pg_hint_plan` extension provides per-query hints where installed.
- **SQL Server**: table hints (`WITH (INDEX(…))`, `NOLOCK`) and query hints (`OPTION (RECOMPILE)`, `OPTION (MAXDOP 1)`). `WITH (NOLOCK)` equals `READ UNCOMMITTED`: it reads uncommitted modifications (dirty reads), and rows can appear or disappear during the read; it is not a performance switch for correctness-sensitive reads.[^10][^18]

Hints freeze a plan while data distributions change. Require a comment explaining the measured reason.

***

#### Maintenance and Monitoring

##### Statistics

The planner chooses plans from statistics. After bulk loads, large deletes or new expression indexes, refresh them:

```sql
-- PostgreSQL (autovacuum also analyzes, with a delay)
ANALYZE users;
-- MySQL
ANALYZE TABLE users, orders;
```

SQLite: `ANALYZE;` (or `PRAGMA optimize;` before closing long-lived connections). SQL Server: `UPDATE STATISTICS dbo.users;` (auto-update is on by default).

##### PostgreSQL vacuum and reindex

- Autovacuum must keep up: every `UPDATE`/`DELETE` leaves dead tuples. Long-running transactions (including idle-in-transaction sessions) prevent cleanup everywhere in the cluster.
- `VACUUM FULL` and `CLUSTER` rewrite the table under an `ACCESS EXCLUSIVE` lock — reads and writes block for the duration. They are not routine maintenance.
- Reindex only when bloat warrants it. Plain `REINDEX` blocks writes to the table and takes an `ACCESS EXCLUSIVE` lock on the index; prefer `REINDEX INDEX CONCURRENTLY` (12+).

```sql
-- PostgreSQL
VACUUM (ANALYZE) users;
REINDEX INDEX CONCURRENTLY users_email_key;
```

##### Monitoring queries (PostgreSQL)

The statistics views use `relname` (table) and `indexrelname` (index) — there are no `tablename`/`indexname` columns in `pg_stat_user_tables`/`pg_stat_user_indexes` (those names belong to the `pg_tables`/`pg_indexes` catalog views).[^19]

```sql
-- PostgreSQL: tables with the most sequential reading
SELECT schemaname, relname, seq_scan, seq_tup_read, idx_scan,
       seq_tup_read / seq_scan AS avg_seq_tup_read
FROM pg_stat_user_tables
WHERE seq_scan > 0
ORDER BY seq_tup_read DESC
LIMIT 10;

-- Indexes never scanned since statistics were reset (check replicas too before dropping)
SELECT schemaname, relname, indexrelname, idx_scan, idx_tup_read, idx_tup_fetch,
       pg_relation_size(indexrelid) AS index_bytes
FROM pg_stat_user_indexes
WHERE idx_scan = 0
ORDER BY pg_relation_size(indexrelid) DESC;
```

An index with `idx_scan = 0` can still be required: it may enforce a unique constraint, serve a primary key, or be used only on a replica (statistics are per server).

```sql
-- PostgreSQL: slowest statements on average (pg_stat_statements: needs shared_preload_libraries
-- and CREATE EXTENSION pg_stat_statements; *_exec_time columns since 13)
SELECT query, calls, total_exec_time, mean_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;
```

##### Other engines

- **MySQL**: the slow query log (`long_query_time`), `performance_schema.events_statements_summary_by_digest`, and the `sys` schema views (`sys.statements_with_full_table_scans`, `sys.schema_unused_indexes`).
- **SQLite**: `EXPLAIN QUERY PLAN`, `sqlite3_trace_v2`/driver tracing, and `PRAGMA integrity_check` after crashes.
- **SQL Server**: Query Store and the `sys.dm_exec_query_stats` / `sys.dm_db_index_usage_stats` dynamic management views.

***

#### Application-side Patterns

- **Connection pooling**: reuse connections; size pools to the database's capacity, not to the number of application threads (PostgreSQL runs one process per connection). Return connections promptly — a leaked connection in an open transaction holds locks and, in PostgreSQL, blocks vacuum.
- **Pooler compatibility**: transaction-mode poolers (PgBouncer, RDS Proxy) do not preserve session state between transactions: session `SET`, temporary tables, advisory locks and, without explicit support, prepared statements break.
- **Timeouts everywhere**: connect timeout, statement/query timeout (PostgreSQL `statement_timeout`, MySQL `max_execution_time`, driver context deadlines), and lock timeout for migrations.
- **Close result sets / rows** in every code path (Go `rows.Close()` and `rows.Err()`, Java try-with-resources, Python context managers).
- **Transactions in code**: one transaction per business operation; commit or roll back on every path (including panics and exceptions); do not mix ORM sessions and raw connections inside one logical transaction unless they share the connection.
- **Read replicas**: reads after writes on a replica can be stale; route read-your-writes paths to the primary.
- **Caching** frequently read, rarely changed data (configuration, reference tables) in the application; invalidate on write.
- **Logging**: log statement fingerprints and durations, not parameter values that may contain personal data or secrets.

***

#### Production / Review Checklist

##### Dialect and portability

- [ ] Dialect (and version) identified from drivers, images or ORM config before judging syntax
- [ ] No dialect-specific syntax in code that must run on several engines (tests on SQLite, production elsewhere)
- [ ] Placeholder style matches the driver; no `RETURNING` on MySQL, no `FETCH FIRST` on MySQL/SQLite, no `||` concatenation on MySQL, no `INTERVAL '30 days'` outside PostgreSQL
- [ ] Integer division, `LIKE` case sensitivity, NULL ordering, `CONCAT` NULL handling and date arithmetic checked where results feed logic

##### Security

- [ ] Every external value bound as a parameter; no string-built SQL with user data
- [ ] Identifiers (sort columns, table names) from an allow-list
- [ ] `LIKE` wildcards escaped for user-supplied substrings
- [ ] Dynamic SQL in routines uses `format('%I')`/`USING`, `sp_executesql` + `QUOTENAME`, or allow-lists
- [ ] Least-privilege database accounts; no credentials in code or migrations

##### Correctness

- [ ] No `= NULL` / `!= NULL`; no `NOT IN` over nullable subqueries
- [ ] `SUM`/`AVG`/`MAX` over possibly empty sets handled (`COALESCE`)
- [ ] Unique constraints back every "must be unique" rule; nullable unique columns behave as intended on the target engine (SQL Server: one NULL)
- [ ] Upserts are atomic (`ON CONFLICT` / `ON DUPLICATE KEY` / `MERGE WITH (HOLDLOCK)`), not check-then-insert
- [ ] Read-modify-write uses atomic updates, `FOR UPDATE`, or version checks
- [ ] `ORDER BY` ends in a unique column wherever order matters; keyset cursors include the tie-breaker
- [ ] Money in exact decimals or integer minor units; no float equality
- [ ] Instants stored in UTC with a zone-aware type (or a documented UTC convention); half-open date ranges
- [ ] SQLite: `PRAGMA foreign_keys = ON` per connection; `STRICT` tables or `CHECK`s for types

##### Performance

- [ ] No N+1 queries (look inside loops and ORM lazy relations)
- [ ] Predicates on indexed columns are sargable (no functions/casts on the column, matching parameter types)
- [ ] Composite indexes ordered equality → range/sort → covering; no redundant indexes
- [ ] Foreign-key columns indexed (PostgreSQL and SQL Server do not do it automatically)
- [ ] Large result sets paginated with keyset, not deep `OFFSET`; MySQL keyset uses the expanded `OR` form
- [ ] Existence checks use `EXISTS`; no full `COUNT(*)` for "total pages" on large tables
- [ ] Bulk writes batched within parameter limits; large updates/deletes chunked with commits
- [ ] `SELECT *` avoided on wide/hot tables
- [ ] Plans checked with `EXPLAIN` on realistic data for new hot queries

##### Migrations

- [ ] Expand/contract: no rename or drop of columns still used by deployed code
- [ ] Large-table indexes built online (`CONCURRENTLY`, `ALGORITHM=INPLACE, LOCK=NONE`, `ONLINE = ON`)
- [ ] Lock timeout set for DDL; constraints added `NOT VALID` then validated (PostgreSQL)
- [ ] MySQL migrations account for implicit commits (one DDL per migration)
- [ ] Backfills batched; `NOT NULL` added after backfill
- [ ] SQLite table rebuilds follow the documented procedure

##### Operations

- [ ] Transactions short; no network calls inside; retries on deadlock/serialization errors
- [ ] Connection pool sized and leaks impossible (rows/statements closed on all paths)
- [ ] Statement and lock timeouts configured
- [ ] Statistics refreshed after bulk changes; monitoring queries use correct catalog columns (`relname`, `indexrelname`)
