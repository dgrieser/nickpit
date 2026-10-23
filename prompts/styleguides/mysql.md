### MySQL Style Guide

#### Overview

> **Scope**: SQL that targets **MySQL Server with InnoDB** when the exact server version is not known: schema and data types, character sets, `sql_mode`, indexing and `EXPLAIN`, query patterns, upserts and bulk writes, transactions and locking, migrations and online DDL, stored programs, security, replication-safe SQL, drivers and connection pools.
> **Version stance**: the rules below hold on every supported release (8.4 LTS, 9.7 LTS) and on the end-of-life 8.0 line unless a rule names a version. Every feature that depends on the server version is labelled with the release that introduced it, e.g. **(8.0.16+)**, **(9.0+)**. **Before recommending a labelled feature, find the server version the project runs** (see "Version-aware review"); never suggest it to a project that may run an older server.
> **Verification**: every example was executed on **MySQL 8.4.13** (Community, official `mysql:8.4` image). Version labels between release lines were checked on **8.0.46** and **9.7.4**; labels on individual 8.0.x patch releases and on 5.7 come from the release notes and manuals cited in the footnotes.
> **Note**: This document is self-contained. When the version is known, a version-specific guide (8.0, 8.4, 9.7) gives more detail; this guide never contradicts them.

Global truths about MySQL with InnoDB (all versions in scope):

- **The table is the primary key.** InnoDB stores rows in a B-tree clustered on the primary key; every secondary index entry carries the full primary key. A wide or random primary key makes every index bigger and every insert slower.
- **REPEATABLE READ is the default isolation level.** Plain `SELECT`s read a consistent snapshot; `UPDATE`, `DELETE` and locking reads (`FOR UPDATE`, `FOR SHARE`) read the latest committed version and take next-key (record + gap) locks on every index record they scan.
- **DDL is not transactional.** Every DDL statement implicitly commits the current transaction. A single DDL statement is atomic (crash-safe) since 8.0; a migration of several statements never is.
- **Strict mode is on by default** (5.7.5+): `STRICT_TRANS_TABLES` and `ONLY_FULL_GROUP_BY` are in the default `sql_mode`. `INSERT IGNORE`, `UPDATE IGNORE` and sessions that clear `sql_mode` silently turn errors back into truncation warnings.
- **`utf8` means the 3-byte `utf8mb3`**, which cannot store emoji and many CJK characters. Use `utf8mb4` everywhere. The default character set is `utf8mb4` with collation `utf8mb4_0900_ai_ci` since 8.0 (5.7: `latin1`).
- **No `RETURNING`, no `MERGE`, no `FULL OUTER JOIN`, no `IS DISTINCT FROM`, no `ADD COLUMN IF NOT EXISTS`, no `CREATE INDEX IF NOT EXISTS`, no partial or `INCLUDE` indexes, no sequences, no materialized views, no transactional DDL** — in any MySQL version up to 9.7. Code or advice copied from PostgreSQL or MariaDB does not run.

***

#### Release Lines and Support (as of October 2026)

MySQL publishes two tracks since 8.1: **LTS** series (features are added and removed only in the first release of the series; later patches contain fixes; 5 years Premier plus 3 years Extended Support) and **Innovation** releases (new features, behaviour changes and removals in every release; each is supported only until the next one ships).[^1] **9.7 is the last line with sequential version numbers**; later releases use calendar versions `YY.M.P` (26.7 is the July 2026 release).[^1][^2]

| Line | Type | First GA | Latest release (date) | Status in October 2026 |
|---|---|---|---|---|
| 5.7 | GA series (pre-LTS model) | 5.7.9 (October 2015) | 5.7.44 (2023-10-25) | **End of life** since October 2023[^3] |
| 8.0 | GA series, bug fixes only since 8.0.34 | 8.0.11 (2018-04-19) | 8.0.46 (2026-04-21) | **End of life** since April 2026 ("with version 8.0.46, MySQL 8.0 reaches End of Life")[^4] |
| 8.1, 8.2, 8.3 | Innovation | 2023-07-18 / 2023-10-25 / 2024-01-16 | 8.1.0 / 8.2.0 / 8.3.0 | Unsupported (superseded) |
| **8.4** | **LTS** | 8.4.0 (2024-04-30) | **8.4.13 (2026-10-20)** | Supported; Premier Support until April 2029, Extended until April 2032[^5][^6] |
| 9.0 – 9.6 | Innovation | 9.0.0 (2024-07-01) … 9.6.0 (2026-01-20) | — | Unsupported (superseded) |
| **9.7** | **LTS** | 9.7.0 (2026-04-21) | **9.7.4 (2026-10-20)** | Supported; Premier Support until April 2031, Extended until April 2034[^6][^7] |
| 26.7, 26.10 | Innovation (calendar versions) | 26.7.0 (2026-07-28) | **26.10.0 (2026-10-20)** is the newest MySQL release | 26.10 is the current Innovation release; 26.7 is superseded[^2] |

Upgrade paths:[^8]

- **An LTS series cannot be skipped**: 5.7 → 8.0 → 8.4 LTS → 9.7 LTS. 8.0 → 9.7 directly is not supported.
- Within an LTS series, in-place upgrade **and downgrade** between patch releases is supported. Innovation releases upgrade to newer Innovation releases; a jump across a major version goes through the LTS in between (8.3 → 8.4 → 9.x). Downgrading an Innovation release needs a logical dump and reload.
- Docker Hub tags: `mysql:8.4`, `mysql:9.7` and `mysql:lts` follow the LTS lines; **`mysql:latest` and `mysql:innovation` point to the newest Innovation release** (26.10.0 in October 2026). A project that wants an LTS must pin `mysql:8.4` or `mysql:9.7` (better: a full patch version), never `latest`.

Review consequences: flag a project on 5.7, 8.0 or any superseded Innovation release (8.1–8.3, 9.0–9.6, 26.7) as running an unsupported server and recommend the next LTS on the upgrade path; still judge the code against the version it actually runs.

***

#### Version-aware Review

##### Find the server version first

Look for it in, in this order: Docker image tags (`docker-compose.yml`, `Dockerfile`, CI `services:`), Helm values and Kubernetes manifests (`image: mysql:8.4.6`), Terraform/CloudFormation (`engine_version`), Testcontainers code (`new MySQLContainer("mysql:8.0.36")`), migration tool config (Flyway, Liquibase, `golang-migrate`), ORM and framework settings (Hibernate dialects such as `MySQL8Dialect`, Doctrine/Symfony `server_version`, Laravel Sail's `docker-compose.yml`), README and ops scripts. In SQL, `SELECT VERSION()` (and, on 8.4.13+/9.7.4+, `VERSION_WITH_UPDATE()`) returns it at run time.

- A floating tag (`mysql:8.4`, `mysql:9.7`) means the newest patch; a pinned patch (`mysql:8.0.28`) lacks everything introduced later.
- **Managed services diverge.** Amazon Aurora MySQL has its own version numbers (Aurora MySQL 3 is 8.0-compatible); RDS, Azure Database for MySQL and Google Cloud SQL publish their own supported versions, parameter defaults and extended-support programs. **HeatWave** (Oracle's cloud service) has features that MySQL Server does not (vector `DISTANCE()`, `TABLESAMPLE`, materialized views, Lakehouse); do not accept them in code that runs on a self-managed server.
- **MariaDB, Percona Server, TiDB, Vitess/PlanetScale, Aurora** are not upstream MySQL; see "MariaDB is not MySQL" and check the platform's own documentation.

##### Rules for the reviewer

- **If the version is unknown, recommend only features available on 8.0** (and preferably flag 8.0 as end of life), and mark any newer suggestion explicitly: "requires MySQL 8.4+", "requires 9.7".
- Say "not available in MySQL X" only when the version table below or the manual of that version proves it. Many PostgreSQL features are genuinely missing, but on 8.0.31+ `INTERSECT`, `EXCEPT`, `LATERAL`, `JSON_TABLE`, `VALUES ROW(...)`, window functions, recursive CTEs, `CHECK` constraints, functional and descending indexes all exist.
- Do not suggest features **newer** than the pinned version, and flag code that uses them; flag features **removed** in the pinned version (table "Removed and changed behaviour across versions").
- A version-tied rule that changes *correctness* (inline `REFERENCES`, `CHECK` enforcement, `GROUP BY` sorting, authentication plugins) matters most: the same DDL can silently mean different things on two servers.

##### Feature availability by version

The release that introduced each feature a reviewer commonly needs. "8.0.x" items are missing from earlier 8.0 patches and from 5.7.[^9][^10][^11][^12]

| Feature | Introduced |
|---|---|
| `JSON` type; `->` / `->>` operators; generated columns (`VIRTUAL`/`STORED`, indexable) | 5.7.8 / 5.7.9 / 5.7.13; 5.7.6 (indexed virtual: 5.7.8) |
| Optimizer hints `/*+ ... */`; `MAX_EXECUTION_TIME`; `ANY_VALUE()`; `sys` schema; `JSON_ARRAYAGG()`/`JSON_OBJECTAGG()` | 5.7.7; 5.7.8; 5.7.5; 5.7.7; 5.7.22 |
| CTEs and `WITH RECURSIVE`; window functions; `FOR UPDATE SKIP LOCKED` / `NOWAIT`; `FOR SHARE`; roles; descending and invisible indexes; `JSON_TABLE()`; histograms; `SET_VAR` hint; ICU `REGEXP_*()` functions; `UUID_TO_BIN()`; atomic DDL; persistent `AUTO_INCREMENT` counter; `caching_sha2_password` default; `utf8mb4_0900_ai_ci` default | 8.0.11 (8.0 GA) |
| `ALGORITHM=INSTANT` `ADD COLUMN` (last position only) | 8.0.12 |
| Functional index key parts `((expr))`; expression defaults `DEFAULT (expr)` (also for `TEXT`/`BLOB`/`JSON`); index skip scan; `sql_require_primary_key` | 8.0.13 |
| `LATERAL` derived tables | 8.0.14 |
| **`CHECK` constraints enforced** (parsed and ignored before); `EXPLAIN FORMAT=TREE` | **8.0.16** |
| Multi-valued indexes, `MEMBER OF`, `JSON_OVERLAPS()`, `JSON_SCHEMA_VALID()` | 8.0.17 |
| `EXPLAIN ANALYZE`; hash join | 8.0.18 |
| Row alias in `INSERT ... VALUES (...) AS new ON DUPLICATE KEY UPDATE`; `TABLE` and `VALUES ROW(...)` statements; datetime literals with time-zone offsets; `FAILED_LOGIN_ATTEMPTS` | 8.0.19 |
| `INDEX`/`NO_INDEX`/`JOIN_INDEX`/`GROUP_INDEX`/`ORDER_INDEX` hints | 8.0.20 |
| `JSON_VALUE()` | 8.0.21 |
| `START REPLICA`/`SHOW REPLICA STATUS`; `CAST(ts AT TIME ZONE '+00:00' AS DATETIME)` | 8.0.22 |
| Invisible columns; `CHANGE REPLICATION SOURCE TO` | 8.0.23 |
| 64-bit `UNIX_TIMESTAMP()`/`FROM_UNIXTIME()` (beyond 2038) | 8.0.28 |
| `ALGORITHM=INSTANT` `ADD COLUMN` at any position and `DROP COLUMN` (64 row versions per table) | 8.0.29 (release withdrawn; use 8.0.30+) |
| Generated invisible primary keys (`sql_generate_invisible_primary_key`) | 8.0.30 |
| `INTERSECT`, `EXCEPT` | 8.0.31 |
| `SHOW BINARY LOG STATUS`, `RESET BINARY LOGS AND GTIDS`, `DISABLE ON REPLICA`, `SET_ANY_DEFINER` privilege | 8.2.0 (first LTS: 8.4) |
| `EXPLAIN FORMAT=JSON INTO @var`; `ANALYZE TABLE ... UPDATE HISTOGRAM ... AUTO UPDATE`; `GROUP BY ROLLUP (a, b)` spelling; foreign keys must reference a `PRIMARY`/`UNIQUE` key (`restrict_fk_on_non_standard_key=ON`) | 8.4 (8.1–8.4 Innovation/LTS) |
| `VERSION_WITH_UPDATE()` | 8.4.13 / 9.7.4 |
| `VECTOR(n)` type and `STRING_TO_VECTOR()`/`VECTOR_TO_STRING()`/`VECTOR_DIM()`; **inline column `REFERENCES` enforced**; `PREPARE` of event DDL | 9.0.0 (first LTS: 9.7) |
| `CREATE VIEW IF NOT EXISTS`; 255 instant row versions | 9.1.0 |
| `CREATE JSON DUALITY VIEW` | 9.4.0 (DML on duality views in Community: 9.7.0) |
| `activate_mandatory_roles`; `INTERPRET_UTF8_AS_UTF8MB4` SQL mode | 9.5.0 |
| SQL-layer foreign keys; `GROUPING()` without `ROLLUP` | 9.6.0 |
| Hypergraph optimizer in Community (opt-in) and therefore `QUALIFY` | 9.7.0 |
| JavaScript stored programs, `CREATE LIBRARY`, masking policies | 9.0–9.7, **Enterprise Edition only** |

Verified boundaries (same statement, adjacent release lines):

```sql
-- 8.0.46: ERROR 1064   8.4.13 / 9.7.4: works
SELECT y, SUM(amt) FROM (SELECT 2024 AS y, 1 AS amt) AS s GROUP BY ROLLUP (y);
SHOW BINARY LOG STATUS;

-- 8.4.13: ERROR 1064 (VECTOR) and ERROR 1064 (IF NOT EXISTS)   9.7.4: works
CREATE TABLE vb (id INT PRIMARY KEY, v VECTOR(3));
CREATE VIEW IF NOT EXISTS vb_view AS SELECT 1 AS a;

-- 8.0.46: works (SHOW MASTER STATUS without warning)   8.4.13 / 9.7.4: ERROR 1064
SHOW MASTER STATUS;
```

##### Not available in any MySQL version up to 9.7

Never suggest these for MySQL; when they appear in code, flag them as bugs (syntax errors, verified on 8.0.46, 8.4.13 and 9.7.4 unless noted):

| PostgreSQL / MariaDB / standard SQL | MySQL alternative |
|---|---|
| `INSERT/UPDATE/DELETE ... RETURNING` | `LAST_INSERT_ID()` for the generated key; a follow-up `SELECT` in the same transaction |
| `MERGE`, `INSERT ... ON CONFLICT` | `INSERT ... ON DUPLICATE KEY UPDATE` |
| `FULL OUTER JOIN` | `LEFT JOIN ... UNION ALL ... RIGHT JOIN ... WHERE left.id IS NULL` |
| `a IS [NOT] DISTINCT FROM b` | `a <=> b` / `NOT (a <=> b)` |
| `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, `DROP INDEX IF EXISTS` | check `information_schema` in the migration tool |
| `CREATE OR REPLACE TABLE`, `CREATE SEQUENCE`, `nextval()` | `DROP TABLE IF EXISTS` + `CREATE TABLE`; `AUTO_INCREMENT` or a counter table |
| Partial indexes (`WHERE`), `INCLUDE` columns, GIN/GiST/BRIN, `CREATE INDEX CONCURRENTLY` | composite or functional indexes; online DDL (`ALGORITHM=INPLACE, LOCK=NONE`) |
| Materialized views (`CREATE MATERIALIZED VIEW` is a syntax error up to 8.4; 9.7.4 accepts it but silently creates a plain view), `WITH x AS MATERIALIZED (...)` | summary table refreshed by a job; `MERGE`/`NO_MERGE` hints |
| `ILIKE`, `::type` casts, `SERIAL`, `BOOLEAN` as a real type, arrays, `TIMESTAMPTZ` | `_ci` collation, `CAST()`, `BIGINT UNSIGNED AUTO_INCREMENT`, `TINYINT(1)` (`BOOL` alias), JSON arrays, `DATETIME` in UTC |
| Transactional DDL | one DDL per migration step (see Migrations) |
| `QUALIFY` | outer query filtering a window result (`QUALIFY` only with the 9.7+ hypergraph optimizer) |
| A UUIDv7 function | generate UUIDv7 in the application |

***

#### MariaDB Is Not MySQL

MariaDB forked from MySQL 5.5 and has diverged for over a decade; its version numbers (10.x, 11.x, 12.x) are unrelated to MySQL's. Never assume feature parity in either direction, and never treat a MariaDB-only feature as valid MySQL (or vice versa):[^13]

- **MariaDB-only syntax** (syntax errors on MySQL): `INSERT ... RETURNING`, `DELETE ... RETURNING`, `CREATE OR REPLACE TABLE`, `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, `CREATE SEQUENCE`, system-versioned tables (`WITH SYSTEM VERSIONING`).
- **`JSON` differs**: in MariaDB `JSON` is an alias for `LONGTEXT` with a `JSON_VALID()` check, not MySQL's binary format; MySQL's multi-valued indexes and binary JSON storage do not carry over, and row-based replication of MySQL `JSON` columns to MariaDB is not supported.
- **Authentication, collations and replication differ**: only recent MariaDB releases support MySQL's default `caching_sha2_password`; older MariaDB releases reject MySQL's `utf8mb4_0900_ai_ci` collation in dumps (`ERROR 1273 Unknown collation`), newer ones map it as an alias to their own UCA 14.0.0 collations, so comparison results can still differ; MariaDB's GTID format differs from MySQL's.
- MySQL features without a direct MariaDB equivalent: multi-valued JSON indexes, invisible indexes (MariaDB has "ignored indexes"), MySQL's `INSTANT` DDL rules, MySQL's `VECTOR` functions.

If a project runs MariaDB (image `mariadb:*`, `MariaDBDialect`, `mariadb` driver), this guide does not apply; review against MariaDB's documentation.

***

#### Schema Design and Data Types

##### Primary keys

Every InnoDB table needs a primary key that is narrow, immutable and ideally monotonically increasing:

```sql
-- GOOD: compact, monotonic clustered key
CREATE TABLE orders (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id     BIGINT UNSIGNED NOT NULL,
  status      VARCHAR(16)     NOT NULL,
  total       DECIMAL(12,2)   NOT NULL,
  created_at  DATETIME(3)     NOT NULL,
  KEY ix_orders_user_created (user_id, created_at),
  KEY ix_orders_status_created (status, created_at)
);
```

- **BAD**: `CHAR(36)` UUID primary keys: 36 characters (up to 144 bytes of index space in `utf8mb4`) repeated in every secondary index, random insert positions, page splits.
- **GOOD**: if UUIDs are required, store them as `BINARY(16)`. For the version-1 UUIDs that `UUID()` returns, `UUID_TO_BIN(u, 1)` **(8.0+)** swaps the time fields so values become roughly monotonic. MySQL has no UUIDv7 generator; generate v7 in the application for time-ordered random IDs. On 5.7, convert with `UNHEX(REPLACE(u, '-', ''))`.

```sql
SELECT HEX(UUID_TO_BIN('6ccd780c-baba-1026-9564-5b8c656024db'))    AS plain,    -- 6CCD780CBABA102695645B8C656024DB
       HEX(UUID_TO_BIN('6ccd780c-baba-1026-9564-5b8c656024db', 1)) AS swapped;  -- 1026BABA6CCD780C95645B8C656024DB

CREATE TABLE exd (                                                -- expression defaults: 8.0.13+
  id   BINARY(16) PRIMARY KEY DEFAULT (UUID_TO_BIN(UUID(), 1)),
  meta JSON DEFAULT (JSON_OBJECT()),
  body TEXT DEFAULT ('')
);
INSERT INTO exd () VALUES ();
SELECT BIN_TO_UUID(id, 1) IS NOT NULL AS has_id, meta, body FROM exd;   -- 1 | {} | (empty)
```

A table without a primary key gets a hidden 6-byte row ID that cannot be used for lookups, makes row-based replication apply changes with full scans on replicas, and is rejected by several managed services and by Group Replication. Flag migrations that create tables without a primary key. Servers can enforce it:

```sql
SET SESSION sql_require_primary_key = ON;               -- 8.0.13+
CREATE TABLE nopk (a INT);
-- ERROR 3750 (HY000): Unable to create or change a table without a primary key, when the system variable
--   'sql_require_primary_key' is set. ...
SET SESSION sql_require_primary_key = DEFAULT;
```

`sql_generate_invisible_primary_key=ON` **(8.0.30+)** adds an invisible `my_row_id BIGINT UNSIGNED AUTO_INCREMENT` primary key instead. Since 9.7.0, both variables accept a `UNIQUE NOT NULL` key as a primary-key substitute; earlier versions do not.[^14]

##### Integers, DECIMAL, FLOAT

- Money and anything compared for equality: `DECIMAL(p,s)`. Never `FLOAT`/`DOUBLE`. `FLOAT` is single precision (about 7 significant digits).
- `UNSIGNED` arithmetic that goes negative is an error, not a wrap-around.
- An `AUTO_INCREMENT` column that reaches its type's maximum fails with a duplicate-key error on the next insert. Use `BIGINT UNSIGNED` for anything that grows; widening `INT` to `BIGINT` later is a full table copy.

```sql
CREATE TABLE money (f FLOAT, d DOUBLE, dec2 DECIMAL(12,2));
INSERT INTO money VALUES (0.1, 0.1, 0.1), (0.2, 0.2, 0.2);
SELECT SUM(f), SUM(d), SUM(dec2), SUM(d) = 0.3, SUM(dec2) = 0.3 FROM money;
-- 0.30000000447034836 | 0.30000000000000004 | 0.30 | 0 | 1

SELECT CAST(1 AS UNSIGNED) - 2;
-- ERROR 1690 (22003): BIGINT UNSIGNED value is out of range in '(cast(1 as unsigned) - 2)'

CREATE TABLE ai (id TINYINT UNSIGNED AUTO_INCREMENT PRIMARY KEY);
INSERT INTO ai VALUES (254);
INSERT INTO ai VALUES ();     -- 255
INSERT INTO ai VALUES ();     -- ERROR 1062 (23000): Duplicate entry '255' for key 'ai.PRIMARY'
```

- Integer display width (`INT(11)`), `ZEROFILL` and `FLOAT(M,D)` are deprecated since 8.0.17 (warning 1681); remove them from new DDL. `TINYINT(1)` draws the same warning but still matters: `BOOL`/`BOOLEAN` is stored as `tinyint(1)`, and drivers use that width to map the column to a boolean. MySQL has no real boolean type.
- `AUTO_INCREMENT` counters survive restarts since 8.0; on 5.7 the counter is recomputed as `MAX(id) + 1` at startup, so ids of deleted rows can be reused.

##### Strings, ENUM, BLOB/TEXT

- `VARCHAR(n)` counts characters; the byte limit matters for indexes (see Indexing) and for the 65,535-byte row-size limit shared by all columns.
- `CHAR(n)` is right-padded; with `PAD SPACE` collations trailing spaces are ignored in comparisons.
- `TEXT`/`BLOB` columns are stored off-page when large, cannot be fully indexed (prefix or functional index only) and cannot have a literal default (an expression default `DEFAULT ('')` works since 8.0.13). Use `VARCHAR` for bounded strings.
- `ENUM` sorts by declaration index, not alphabetically, and compares to integers by index. Adding a value at the end is a metadata change; reordering or inserting in the middle rebuilds the table. A lookup table or a `VARCHAR` with a `CHECK` constraint (8.0.16+) is easier to evolve.

```sql
CREATE TABLE en (s ENUM('small','large','medium'));
INSERT INTO en VALUES ('small'), ('large'), ('medium');
SELECT s FROM en ORDER BY s;        -- small, large, medium  (declaration order)
SELECT s FROM en WHERE s = 2;       -- large                 (index 2)
INSERT INTO en VALUES ('huge');     -- ERROR 1265 (01000): Data truncated for column 's' at row 1
```

##### JSON

- `JSON` (5.7.8+) is stored in a binary format, validated on insert, and can be updated in place with `JSON_SET`/`JSON_REPLACE`/`JSON_REMOVE` (partial updates, 8.0+).
- **BAD**: storing attributes in `JSON` that are filtered, joined, sorted or constrained in every query. Model them as columns; index JSON paths only when necessary (generated columns, functional indexes 8.0.13+, multi-valued indexes 8.0.17+; see Indexing).
- JSON `null` is not SQL `NULL`, and a missing path is SQL `NULL`:

```sql
SELECT JSON_EXTRACT('{"a": 1}', '$.b') IS NULL;                 -- 1: missing path is SQL NULL
SELECT JSON_EXTRACT('{"a": null}', '$.a') IS NULL;              -- 0: JSON null is not SQL NULL
SELECT JSON_TYPE(JSON_EXTRACT('{"a": null}', '$.a'));           -- NULL (the string 'NULL', a JSON type name)
```

- `col->'$.p'` returns JSON (strings keep their quotes); `col->>'$.p'` returns unquoted text with collation `utf8mb4_bin`. Both accept only a column on the left; use `JSON_EXTRACT()`/`JSON_UNQUOTE()` on expressions.
- `JSON_MERGE()` is deprecated: use `JSON_MERGE_PATCH()` (RFC 7396, later keys win) or `JSON_MERGE_PRESERVE()` (combines arrays) and pick deliberately.
- Validate structure with `CHECK (JSON_SCHEMA_VALID(...))` (8.0.17+ function, enforced as a constraint on 8.0.16+).

##### Generated and invisible columns

```sql
CREATE TABLE gc (
  id INT PRIMARY KEY,
  price DECIMAL(10,2) NOT NULL,
  qty INT NOT NULL,
  line_total DECIMAL(12,2) AS (price * qty) VIRTUAL,
  email VARCHAR(255) NOT NULL,
  email_domain VARCHAR(255) AS (SUBSTRING_INDEX(email, '@', -1)) STORED,
  KEY ix_gc_domain (email_domain)
);
INSERT INTO gc (id, price, qty, email) VALUES (1, 2.50, 4, 'a@example.com');
SELECT line_total, email_domain FROM gc;    -- 10.00 | example.com
INSERT INTO gc VALUES (2, 1, 1, 5, 'b@x.org', 'x.org');
-- ERROR 3105 (HY000): The value specified for generated column 'line_total' in table 'gc' is not allowed.
```

- Generated columns (5.7.6+) are `VIRTUAL` (computed on read; can be indexed; added with `ALGORITHM=INSTANT` on 8.0+) or `STORED` (materialized; adding one rebuilds the table with `ALGORITHM=COPY`).
- Expressions must be deterministic; a generated column whose expression calls a function that a later version removes (e.g. `MD5()`/`SHA1()` moved out of the server in 9.6) breaks the table after an upgrade. Prefer `SHA2()` in generated columns, defaults, `CHECK` constraints and functional indexes.[^15]
- Invisible columns **(8.0.23+)** are skipped by `SELECT *` and by `INSERT` without a column list; they help add columns without breaking legacy code but are still returned when named explicitly.

##### VECTOR (9.0+)

`VECTOR(n)` exists only on 9.0+ (first LTS: 9.7). It is a storage type in MySQL Server: no vector index, no distance function (`DISTANCE()` is HeatWave-only), no ordering other than by bytes; `n` is a maximum, not an exact dimension. On 8.4 and older `VECTOR` is a syntax error. Validate dimensions in the application, and do similarity search in the application or a dedicated vector store.[^16]

***

#### Constraints: NOT NULL, CHECK, Defaults

Declare `NOT NULL` on every column that must have a value; `NULL` makes `NOT IN`, `UNIQUE` and comparisons behave unexpectedly (a `UNIQUE` key accepts any number of `NULL`s).

`CHECK` constraints are **enforced since 8.0.16**. On 5.7 and 8.0.0–8.0.15 they are parsed and **silently ignored**, so a schema from that era can contain `CHECK` clauses that never held; the first `ALTER TABLE` or reload on 8.0.16+ then fails on violating rows.[^9]

```sql
CREATE TABLE pr (
  id INT PRIMARY KEY,
  price DECIMAL(10,2) NOT NULL,
  discount DECIMAL(10,2) NOT NULL DEFAULT 0,
  CONSTRAINT chk_price CHECK (price >= 0),
  CONSTRAINT chk_disc  CHECK (discount <= price)
);
INSERT INTO pr VALUES (1, 5, 6);              -- ERROR 3819 (HY000): Check constraint 'chk_disc' is violated.
INSERT IGNORE INTO pr VALUES (2, -1, 0);      -- Warning 3819; row skipped, statement "succeeds"
CREATE TABLE chk3 (a INT, CHECK (a > 0));
INSERT INTO chk3 VALUES (NULL);               -- accepted: NULL > 0 is UNKNOWN, which passes
CREATE TABLE ck (a INT, b INT, CHECK (b > RAND()));
-- ERROR 3814 (HY000): An expression of a check constraint 'ck_chk_1' contains disallowed function: rand.
CREATE TABLE ck2 (a INT, CONSTRAINT c CHECK (a > 0) NOT ENFORCED);
INSERT INTO ck2 VALUES (-1);                  -- accepted: NOT ENFORCED is documentation only

CREATE TABLE cj (id INT PRIMARY KEY, doc JSON,
  CHECK (JSON_SCHEMA_VALID('{"type":"object","required":["name"]}', doc)));
INSERT INTO cj VALUES (1, '{"x":1}');         -- ERROR 3819 (HY000): Check constraint 'cj_chk_1' is violated.
INSERT INTO cj VALUES (2, '{"name":"a"}');    -- OK
```

Review rules:

- Pair `CHECK` with `NOT NULL` where `NULL` must not pass.
- `NOT ENFORCED` constraints are documentation; do not count them as validation.
- On a server that may be older than 8.0.16, a `CHECK` clause guarantees nothing: enforce the invariant in the application or with a `BEFORE INSERT`/`BEFORE UPDATE` trigger that `SIGNAL`s.
- Adding a `CHECK` to an existing table validates every row with `ALGORITHM=COPY`; plan it like a rebuild.

***

#### Foreign Keys

InnoDB enforces foreign keys. Traps that depend on how and where the DDL runs:

1. **Inline column `REFERENCES` is silently ignored before 9.0 and enforced from 9.0.** Only the table-level form `[CONSTRAINT name] FOREIGN KEY (col) REFERENCES parent (id)` creates a constraint on 5.7, 8.0 and 8.4. On 9.0+ the inline form creates a real constraint, so the same DDL produces different schemas, and code that inserted orphans "successfully" starts failing with `ERROR 1452` after an upgrade (tables created earlier do not gain the constraint during an in-place upgrade). Always write the explicit, named table-level form:[^17]

```sql
CREATE TABLE person (id INT PRIMARY KEY);
-- BAD: inline REFERENCES (8.4.13 and 8.0.46: no constraint, no warning; 9.7.4: enforced)
CREATE TABLE pet (id INT PRIMARY KEY, owner INT REFERENCES person (id));
INSERT INTO pet VALUES (1, 999);   -- 8.4.13: succeeds   9.7.4: ERROR 1452 (23000): Cannot add or update a child row ...
SELECT COUNT(*) FROM information_schema.referential_constraints
WHERE constraint_schema = DATABASE() AND table_name = 'pet';   -- 8.4.13: 0   9.7.4: 1

-- GOOD: table-level, named, enforced on every version
CREATE TABLE pet2 (
  id INT PRIMARY KEY,
  owner INT NOT NULL,
  CONSTRAINT fk_pet2_owner FOREIGN KEY (owner) REFERENCES person (id)
);
```

2. **The referenced columns must form a `PRIMARY KEY` or `UNIQUE` key on 8.4+** (`restrict_fk_on_non_standard_key=ON`); 8.0 and 5.7 accept a foreign key to any indexed column prefix. Such legacy constraints still load after an upgrade, but recreating them (e.g. restoring a dump) fails:

```sql
CREATE TABLE fkp (id INT PRIMARY KEY, code INT, KEY (code));
CREATE TABLE fkc (id INT PRIMARY KEY, pcode INT, FOREIGN KEY (pcode) REFERENCES fkp (code));
-- 8.0.46: OK   8.4.13 / 9.7.4: ERROR 6125 (HY000): Failed to add the foreign key constraint.
--   Missing unique key for constraint 'fkc_ibfk_1' in the referenced table 'fkp'
```

3. **Cascaded changes do not fire triggers.** `ON DELETE CASCADE`, `ON UPDATE CASCADE` and `SET NULL` modify child rows without running the child table's triggers; audit triggers on child tables miss them. (9.7.0 adds an opt-in, already deprecated `enable_cascade_triggers`.)[^18]

```sql
CREATE TABLE audit (id INT AUTO_INCREMENT PRIMARY KEY, what VARCHAR(50));
CREATE TABLE p2 (id INT PRIMARY KEY);
CREATE TABLE c2 (id INT PRIMARY KEY, p INT,
                 CONSTRAINT fk_c2_p FOREIGN KEY (p) REFERENCES p2 (id) ON DELETE CASCADE);
CREATE TRIGGER c2_ad AFTER DELETE ON c2 FOR EACH ROW
  INSERT INTO audit (what) VALUES (CONCAT('deleted child ', OLD.id));
INSERT INTO p2 VALUES (1);
INSERT INTO c2 VALUES (10, 1), (11, 1);
DELETE FROM p2 WHERE id = 1;
SELECT (SELECT COUNT(*) FROM c2) AS children, (SELECT COUNT(*) FROM audit) AS audit_rows;   -- 0 | 0
```

4. **`REPLACE` on a parent row deletes and re-inserts it**, which cascades `ON DELETE` to children on 8.4 and older (9.6+ with SQL-layer foreign keys keeps the children when the key value is unchanged). See Writes.
5. **`SET foreign_key_checks = 0` lets orphans in.** Adding a foreign key with checks disabled does not validate existing rows. Run an orphan query before re-enabling, and never leave the setting off in application sessions.
6. Column types must match: integer types with the same size and signedness, strings with the same character set and collation (`ERROR 3780 ... are incompatible` otherwise). The child columns get an index automatically if none exists.
7. Partitioned InnoDB tables cannot have foreign keys.
8. Foreign keys make parent deletes and child inserts take shared locks on the other table's rows; with heavy concurrency they participate in deadlocks. That is not a reason to drop them, but a reason to keep transactions short.

***

#### Character Sets and Collations

##### Use utf8mb4, set it explicitly

```sql
SELECT @@character_set_server, @@collation_server;       -- utf8mb4 | utf8mb4_0900_ai_ci  (8.0+; 5.7: latin1 | latin1_swedish_ci)

CREATE TABLE m3 (s VARCHAR(10) CHARACTER SET utf8);
-- Warning 3719: 'utf8' is currently an alias for the character set UTF8MB3, but will be an alias for UTF8MB4
--   in a future release. Please consider using UTF8MB4 in order to be unambiguous.
SET NAMES utf8mb4;
INSERT INTO m3 VALUES ('😀');
-- ERROR 1366 (HY000): Incorrect string value: '\xF0\x9F\x98\x80' for column 's' at row 1
```

- Flag `utf8`, `utf8mb3`, `utf8_general_ci`, `utf8_unicode_ci` in new DDL; use `utf8mb4`.
- On a server that may be 5.7, `CREATE DATABASE`/`CREATE TABLE` without an explicit character set creates `latin1` tables. Always specify `DEFAULT CHARSET=utf8mb4` and a collation in migrations; `utf8mb4_0900_*` collations do not exist on 5.7 (`ERROR 1273 Unknown collation`), so a schema that must also run on 5.7 or MariaDB uses `utf8mb4_unicode_ci`.
- **Connections must declare their character set.** Drivers: `charset=utf8mb4` (or the driver's equivalent). The `mysql` command-line client takes its character set from the OS locale; in a container without a locale it can connect as `latin1`, and non-ASCII literals in migration scripts are then stored as mojibake. Use `--default-character-set=utf8mb4` or `SET NAMES utf8mb4` in migration runners.

##### Collation semantics

`utf8mb4_0900_ai_ci` (8.0 default) is case-insensitive, accent-insensitive and **NO PAD** (trailing spaces are significant). Older collations (`utf8mb4_general_ci`, `utf8mb4_unicode_ci`, `utf8mb4_bin`, all 5.7 collations) are `PAD SPACE`.[^19]

```sql
SET NAMES utf8mb4;
SELECT 'a' = 'A', 'e' = 'é', 'a' = 'a ', 'ß' = 'ss';      -- 1 | 1 | 0 | 1
SELECT 'a' = 'a ' COLLATE utf8mb4_general_ci AS general_pad,
       'a' = 'a ' COLLATE utf8mb4_unicode_ci AS unicode_pad,
       'a' = 'a ' COLLATE utf8mb4_0900_ai_ci AS nopad;     -- 1 | 1 | 0
SELECT 'a' = 'A' COLLATE utf8mb4_0900_as_cs AS cs,
       'a' = 'A' COLLATE utf8mb4_bin AS b;                 -- 0 | 0

CREATE TABLE u (name VARCHAR(50) PRIMARY KEY);
INSERT INTO u VALUES ('Muller');
INSERT INTO u VALUES ('müller');    -- ERROR 1062 (23000): Duplicate entry 'müller' for key 'u.PRIMARY'
INSERT INTO u VALUES ('Muller ');   -- OK: NO PAD, trailing space is significant
```

Consequences a reviewer must catch:

- **Unique keys are collation-aware.** Usernames, slugs, tokens, hashes and external IDs that must be distinct byte-for-byte need `COLLATE utf8mb4_bin` (or `utf8mb4_0900_bin`) or a `VARBINARY` column.
- **`LOWER(col) = ?` is redundant** with a `_ci` collation and prevents index use.
- **Mixing collations** in a comparison raises `ERROR 1267 Illegal mix of collations`, and a join between columns with different collations cannot use an index on the converted side. Schemas migrated from 5.7 often contain `utf8mb4_general_ci`/`utf8mb4_unicode_ci` columns next to new `utf8mb4_0900_ai_ci` columns; specify the collation explicitly at database or table level so new tables match old ones.

```sql
CREATE TABLE ca (s VARCHAR(10) COLLATE utf8mb4_unicode_ci);
CREATE TABLE cb (s VARCHAR(10) COLLATE utf8mb4_0900_ai_ci);
SELECT * FROM ca JOIN cb ON ca.s = cb.s;
-- ERROR 1267 (HY000): Illegal mix of collations (utf8mb4_unicode_ci,IMPLICIT) and
--   (utf8mb4_0900_ai_ci,IMPLICIT) for operation '='
```

- `ALTER TABLE ... CONVERT TO CHARACTER SET` rewrites the whole table (`ALGORITHM=COPY`) and can change uniqueness (values that were distinct may now collide) and index lengths. Review it as a heavy migration.

***

#### SQL Mode and Strictness

The default `sql_mode` is strict (5.7.5+):[^20]

```sql
SELECT @@sql_mode;
-- ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION
--   (5.7 additionally has NO_AUTO_CREATE_USER, which 8.0+ rejects: ERROR 1231)
```

With the default mode, bad data is rejected; `IGNORE` and a relaxed session mode bring back silent truncation:

```sql
CREATE TABLE s1 (id INT PRIMARY KEY, code VARCHAR(5) NOT NULL, qty TINYINT UNSIGNED, d DATE);
INSERT INTO s1 VALUES (1, 'toolong', 1, '2024-01-01'); -- ERROR 1406 (22001): Data too long for column 'code' at row 1
INSERT INTO s1 VALUES (2, 'ok', 300, '2024-01-01');    -- ERROR 1264 (22003): Out of range value for column 'qty' at row 1
INSERT INTO s1 VALUES (3, 'ok', 1, '2024-02-30');      -- ERROR 1292 (22007): Incorrect date value: '2024-02-30' ...
INSERT INTO s1 (id, qty) VALUES (5, 1);                -- ERROR 1364 (HY000): Field 'code' doesn't have a default value
INSERT INTO s1 VALUES (6, 'ok', 1/0, '2024-01-01');    -- ERROR 1365 (22012): Division by 0

INSERT IGNORE INTO s1 VALUES (7, 'toolong', 300, '2024-02-30');
SELECT * FROM s1;                              -- 7 | toolo | 255 | 0000-00-00   (three warnings, data mangled)

SET SESSION sql_mode = '';
INSERT INTO s1 (id, qty) VALUES (8, -5);       -- Warning 1264, Warning 1364
SELECT * FROM s1 WHERE id = 8;                 -- 8 |  | 0 | NULL
SET SESSION sql_mode = DEFAULT;
```

Review rules:

- **BAD**: application code, ORM settings or connection strings that set `sql_mode=''` or remove `STRICT_TRANS_TABLES`/`ONLY_FULL_GROUP_BY` to make legacy queries pass. Fix the query instead.
- **BAD**: `INSERT IGNORE` as "skip duplicates": it also hides truncation, out-of-range values, invalid dates, `CHECK` violations and foreign-key violations. Use `INSERT ... ON DUPLICATE KEY UPDATE id = id` or catch error 1062.
- **BAD**: setting `sql_mode` with `NO_AUTO_CREATE_USER` (common in old dumps): 8.0+ rejects it with `ERROR 1231`.
- Strict mode validates values written to columns, not expressions: `'1abc' + 0` is 1 with a warning, `'abc' = 0` is true. Compare strings with strings.
- Do not enable `ANSI_QUOTES` or `PIPES_AS_CONCAT` in some sessions only: the same SQL then means different things on different connections.

##### ONLY_FULL_GROUP_BY

```sql
CREATE TABLE o (id INT PRIMARY KEY, cust INT, total DECIMAL(10,2), status VARCHAR(10));
INSERT INTO o VALUES (1, 1, 10, 'new'), (2, 1, 20, 'paid'), (3, 2, 5, 'new');

SELECT cust, status, SUM(total) FROM o GROUP BY cust;
-- ERROR 1055 (42000): Expression #2 of SELECT list is not in GROUP BY clause and contains nonaggregated
--   column 't.o.status' which is not functionally dependent on columns in GROUP BY clause; ...

SELECT cust, ANY_VALUE(status), SUM(total) FROM o GROUP BY cust;   -- explicit "any value" (nondeterministic)
SELECT o.id, o.cust, o.status FROM o GROUP BY o.id;                -- OK: functionally dependent on the PK
```

`ANY_VALUE()` is acceptable only when any value is correct; for "the row with the max per group" use a window function (8.0+) or a join. **`GROUP BY` does not sort since 8.0** (5.7 sorted implicitly and allowed `GROUP BY col DESC`, which is `ERROR 1064` since 8.0.13): add `ORDER BY` whenever order matters.

##### Operators that differ from standard SQL

```sql
SELECT 'a' || 'b';                     -- 0: || is logical OR (deprecated, warning 1287), not concatenation
SELECT CONCAT('a', 'b', NULL);         -- NULL: any NULL argument makes CONCAT NULL
SELECT CONCAT_WS('-', 'a', NULL, 'b'); -- a-b: CONCAT_WS skips NULLs
SELECT NULL <=> NULL, 1 <=> NULL;      -- 1 | 0: null-safe equality (MySQL's IS NOT DISTINCT FROM)
SELECT 5 / 2, 5 DIV 2, 5 % 2;          -- 2.5000 | 2 | 1
```

`&&`, `||` (as `OR`) and `!` are deprecated since 8.0.17; write `AND`, `OR`, `NOT`, `CONCAT()`.

***

#### Date, Time and Time Zones

##### DATETIME vs TIMESTAMP

| | `DATETIME(fsp)` | `TIMESTAMP(fsp)` |
|---|---|---|
| Range | `1000-01-01` – `9999-12-31` | `1970-01-01 00:00:01` UTC – `2038-01-19 03:14:07` UTC |
| Time zone | Stored as written; no conversion | Converted from session `time_zone` to UTC on write and back on read |
| Storage | 5 bytes + fsp | 4 bytes + fsp |

```sql
CREATE TABLE tt (id INT PRIMARY KEY, ts TIMESTAMP NULL, dt DATETIME NULL, dt3 DATETIME(3) NULL);
SET time_zone = '+00:00';
INSERT INTO tt VALUES (1, '2024-06-01 12:00:00', '2024-06-01 12:00:00', NULL);
SET time_zone = '+02:00';
SELECT ts, dt FROM tt WHERE id = 1;  -- 2024-06-01 14:00:00 | 2024-06-01 12:00:00
SET time_zone = '+00:00';
INSERT INTO tt VALUES (2, '2038-01-19 03:14:08', '2038-01-19 03:14:08', NULL);
-- ERROR 1292 (22007): Incorrect datetime value: '2038-01-19 03:14:08' for column 'ts' at row 1
```

- **BAD**: `TIMESTAMP` for business dates or anything that may exceed January 2038 (contract end dates, expiry dates, scheduled events) or precede 1970 (birthdays).
- **GOOD**: `DATETIME(3)`/`DATETIME(6)` in UTC, written by the application or with `UTC_TIMESTAMP(3)`, and every connection with `time_zone='+00:00'`.
- `TIMESTAMP` is acceptable for "row changed at" audit columns, but every connection must agree on `time_zone` or reads differ by connection.
- With `explicit_defaults_for_timestamp=ON` (the default on 8.0+; `OFF` on 5.7) a `TIMESTAMP` column has no implicit `DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP`. On 5.7 the first `TIMESTAMP` column of a table silently gets both, so it changes on every update. Write `NULL`/`NOT NULL`, `DEFAULT` and `ON UPDATE` explicitly on every temporal column; `DATETIME` accepts the same clauses (`DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)`).

##### Fractional seconds round by default

```sql
INSERT INTO tt VALUES (3, NULL, '2024-06-01 23:59:59.500', '2024-06-01 23:59:59.9995');
SELECT dt, dt3 FROM tt WHERE id = 3;   -- 2024-06-02 00:00:00 | 2024-06-02 00:00:00.000
```

Writing a value with more fractional digits than the column holds **rounds** (here into the next day) unless `sql_mode` contains `TIME_TRUNCATE_FRACTIONAL` (8.0+). Declare the precision the application uses (`DATETIME(3)` for milliseconds, `(6)` for microseconds) and make both sides match.

##### Time zones and date arithmetic

- Named zones (`'Europe/Berlin'`) in `CONVERT_TZ()` and `SET time_zone` need the time-zone tables to be loaded (`mysql_tzinfo_to_sql`); otherwise `CONVERT_TZ()` returns `NULL` and `SET time_zone = 'Europe/Berlin'` fails with `ERROR 1298 Unknown or incorrect time zone`. Many installations and managed services do not load them by default. Prefer UTC offsets or conversion in the application; check `SELECT COUNT(*) FROM mysql.time_zone_name` when code relies on named zones.[^21]
- `NOW()` is constant within a statement; `SYSDATE()` is evaluated per call and unsafe for statement-based replication. Use `NOW()`/`UTC_TIMESTAMP()`.
- Month arithmetic clamps: `'2024-01-31' + INTERVAL 1 MONTH` is `2024-02-29`.
- Filtering with `DATE(col) = ?` or `YEAR(col) = ?` disables index use; use half-open ranges `col >= ? AND col < ?` (see Indexing). Avoid `BETWEEN '...' AND '... 23:59:59'`: it misses fractional seconds.
- Datetime literals with offsets (`'2024-06-01 12:00:00+05:00'`) are accepted since 8.0.19 and converted to the session time zone; 5.7 rejects them.
- `UNIX_TIMESTAMP()`/`FROM_UNIXTIME()` handle values beyond 2038 on 64-bit platforms since 8.0.28; earlier versions return 0/`NULL`.

```sql
SELECT '2024-01-31' + INTERVAL 1 MONTH;                         -- 2024-02-29
SELECT CONVERT_TZ('2024-01-01 12:00:00', '+00:00', '+05:30');   -- 2024-01-01 17:30:00 (offsets need no tables)
```

***

#### Indexing

##### What InnoDB indexes can and cannot do

InnoDB has B-tree indexes (`PRIMARY`, `UNIQUE`, secondary), `FULLTEXT` and `SPATIAL` (R-tree) indexes, plus functional **(8.0.13+)** and multi-valued **(8.0.17+)** B-tree indexes.[^22] It has no partial (`WHERE`) indexes, no `INCLUDE` columns, no hash indexes (`USING HASH` is accepted on InnoDB with a note and builds a B-tree), no `CREATE INDEX CONCURRENTLY`, no `CREATE INDEX IF NOT EXISTS`.

| PostgreSQL idea | MySQL equivalent |
|---|---|
| Partial index `WHERE status = 'active'` | Composite index with the filter column first |
| `INCLUDE (a, b)` covering index | Append the columns to the key; every secondary index already contains the primary key |
| `CREATE INDEX CONCURRENTLY` | `ALTER TABLE ... ADD INDEX ..., ALGORITHM=INPLACE, LOCK=NONE` |
| Expression index `ON (lower(email))` | Usually unnecessary (`_ci` collation); otherwise a functional index `((expr))` (8.0.13+) or an indexed generated column (5.7+) |
| GIN on a JSON array | Multi-valued index `((CAST(doc->'$.tags' AS CHAR(n) ARRAY)))` (8.0.17+) |
| `pg_trgm` for `LIKE '%x%'` | `FULLTEXT` index with `MATCH ... AGAINST` (word-based, not substring) or an external search engine |

##### Leftmost prefix, ranges, covering indexes

A composite index `(a, b, c)` serves predicates on `a`, `a,b`, `a,b,c`; equalities on leading columns followed by one range column is ideal. Columns after the first range column are not used for seeking (only for index condition pushdown). Every secondary index implicitly ends with the primary-key columns, so `KEY (user_id, created_at)` covers `SELECT id, created_at ... WHERE user_id = ?`.

The examples below use a generated data set (recursive CTE: 8.0+):

```sql
CREATE TABLE users (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  email VARCHAR(255) NOT NULL,
  phone VARCHAR(20) NOT NULL,
  created_at DATETIME NOT NULL,
  UNIQUE KEY uq_users_email (email),
  KEY ix_users_phone (phone),
  KEY ix_users_created (created_at)
);
SET SESSION cte_max_recursion_depth = 20000;
INSERT INTO users (email, phone, created_at)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 20000)
SELECT CONCAT('user', n, '@example.com'), CONCAT('49151', LPAD(n, 5, '0')),
       '2024-01-01' + INTERVAL n MINUTE FROM seq;
INSERT INTO orders (user_id, status, total, created_at)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 20000)
SELECT n % 2000 + 1, ELT(n % 3 + 1, 'new', 'paid', 'cancelled'), n % 100, '2024-01-01' + INTERVAL n MINUTE FROM seq;
ANALYZE TABLE users, orders;

EXPLAIN FORMAT=TREE
SELECT id, created_at FROM orders WHERE user_id = 42 ORDER BY created_at DESC LIMIT 10;
-- -> Limit: 10 row(s)
--     -> Covering index lookup on orders using ix_orders_user_created (user_id=42) (reverse)  ...
```

"Covering index" in the tree output (`Using index` in the traditional format) means no clustered-index lookup. 8.0 prints a covering full index scan as `Index scan on ...`; 8.4+ as `Covering index scan on ...`.

##### Things that prevent index use

```sql
-- BAD: implicit conversion — phone is VARCHAR, the literal is a number: every row is converted
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE phone = 4915100042;
-- -> Filter: (users.phone = 4915100042) ... -> Covering index scan on users using ix_users_phone
-- Warning 1739: Cannot use ref access on index 'ix_users_phone' due to type or collation conversion on field 'phone'
-- GOOD
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE phone = '4915100042';
-- -> Covering index lookup on users using ix_users_phone (phone='4915100042')

-- BAD: function on the indexed column
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE DATE(created_at) = '2024-01-02';
-- -> Filter: (cast(users.created_at as date) = '2024-01-02') -> Covering index scan on users ...
-- GOOD: half-open range
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE created_at >= '2024-01-02' AND created_at < '2024-01-03';
-- -> Covering index range scan on users using ix_users_created over ('2024-01-02 00:00:00' <= created_at < '2024-01-03 00:00:00')

-- BAD: LOWER() is redundant with a _ci collation and disables the index
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE LOWER(email) = 'user42@example.com';
-- -> Filter: (lower(users.email) = 'user42@example.com') -> Covering index scan on users using uq_users_email
-- GOOD
SELECT id FROM users WHERE email = 'USER42@example.com';     -- 42 (case-insensitive unique lookup)

-- Leading wildcard: full index scan
EXPLAIN FORMAT=TREE SELECT id FROM users WHERE email LIKE '%42@example.com';
-- -> Filter: (users.email like '%42@example.com') -> Covering index scan on users using uq_users_email
```

- Bind strings for string columns: a driver that binds an integer for a `VARCHAR` external ID or phone number causes the full scan above and wrong matches (`'1abc' = 1` is true).
- `OR` across different columns may use `index_merge` or fall back to a scan; check the plan, and consider `UNION ALL` of two indexed queries.
- A collation or character-set mismatch between join columns has the same effect as a type conversion.

##### Index size limits

With `ROW_FORMAT=DYNAMIC` (default since 5.7.9) an index key part may be at most 3072 bytes, i.e. `VARCHAR(768)` in `utf8mb4`; `COMPACT`/`REDUNDANT` tables (often created on 5.6) allow 767 bytes, i.e. `VARCHAR(191)`:

```sql
CREATE TABLE ix1 (s VARCHAR(768), KEY (s));    -- OK
CREATE TABLE ix2 (s VARCHAR(769), KEY (s));
-- ERROR 1071 (42000): Specified key was too long; max key length is 3072 bytes
```

A prefix index (`KEY (s(191))`) is acceptable for non-unique lookups only: a prefix `UNIQUE` key enforces uniqueness of the prefix, and a prefix index is never covering. For uniqueness of long values (URLs), index a hash in a generated column: `url_hash BINARY(32) AS (UNHEX(SHA2(url, 256))) STORED UNIQUE`.

##### Functional indexes and JSON (8.0.13+ / 8.0.17+)

A functional index is used only when the query contains the **same expression with the same type and collation**. `->>` returns `utf8mb4_bin`, so an index on a plain `CAST(... AS CHAR)` (connection collation) is not used by `doc->>'$.name' = ?`:[^22]

```sql
CREATE TABLE docs (id INT AUTO_INCREMENT PRIMARY KEY, doc JSON NOT NULL);
INSERT INTO docs (doc)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 2000)
SELECT JSON_OBJECT('name', CONCAT('n', n), 'age', n % 90, 'tags', JSON_ARRAY(CONCAT('t', n % 10), 't9')) FROM seq;

-- BAD: collation of the index expression differs from doc->>'$.name'
ALTER TABLE docs ADD INDEX ix_name ((CAST(doc->>'$.name' AS CHAR(32))));
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE doc->>'$.name' = 'n42';
-- -> Filter: (json_unquote(json_extract(docs.doc,'$.name')) = 'n42') -> Table scan on docs

-- GOOD: match the collation of ->>
ALTER TABLE docs DROP INDEX ix_name;
ALTER TABLE docs ADD INDEX ix_name ((CAST(doc->>'$.name' AS CHAR(32)) COLLATE utf8mb4_bin));
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE doc->>'$.name' = 'n42';
-- -> Index lookup on docs using ix_name ((cast(json_unquote(json_extract(doc,_utf8mb4'$.name')) as char(32) charset utf8mb4) collate utf8mb4_bin)='n42')

-- Also GOOD and works on 5.7: an indexed virtual generated column (8.0+ also matches the expression)
ALTER TABLE docs ADD COLUMN age INT AS (doc->>'$.age') VIRTUAL, ADD INDEX ix_age (age);
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE age = 42;   -- -> Covering index lookup on docs using ix_age (age=42)

-- Multi-valued index (8.0.17+): serves MEMBER OF, JSON_CONTAINS, JSON_OVERLAPS — not JSON_SEARCH or LIKE
ALTER TABLE docs ADD INDEX ix_tags ((CAST(doc->'$.tags' AS CHAR(8) ARRAY)));
EXPLAIN FORMAT=TREE SELECT id FROM docs WHERE 't3' MEMBER OF (doc->'$.tags');
-- -> Index lookup on docs using ix_tags (...=json'"t3"')
```

##### Descending, invisible and redundant indexes

- Descending indexes (`KEY (a, b DESC)`) **(8.0+)** matter for mixed-direction sorts; on 5.7 `DESC` is parsed and ignored. A single-direction sort can use a backward scan of an ascending index.
- Invisible indexes **(8.0+)**: `ALTER TABLE t ALTER INDEX ix INVISIBLE` hides an index from the optimizer while still maintaining it (and still enforcing `UNIQUE`); use it to test a drop safely. A primary key cannot be invisible.
- Flag redundant indexes (`KEY (a)` next to `KEY (a, b)`; `sys.schema_redundant_indexes` lists them) and over-indexing on write-heavy tables: each secondary index is one more B-tree write per row.
- Index hints (`FORCE INDEX (ix)`) that name a dropped index make the query fail with `ERROR 1176`; search the code for hints before dropping or renaming an index.

##### Histograms (8.0+)

For non-indexed columns used in filters and joins, `ANALYZE TABLE t UPDATE HISTOGRAM ON col WITH 32 BUCKETS` gives the optimizer selectivity estimates without index maintenance. Histograms are not refreshed automatically before 8.4 (`... AUTO UPDATE`, 8.4+); a stale histogram can be worse than none.

***

#### Reading Plans: EXPLAIN and EXPLAIN ANALYZE

- `EXPLAIN` (traditional table, default up to 9.4), `EXPLAIN FORMAT=TREE` **(8.0.16+)**, `EXPLAIN FORMAT=JSON`. **9.5+ prints the tree by default** (`explain_format=TREE`) and uses a different JSON layout (`explain_json_format_version=2`): tooling that parses `EXPLAIN` output must pin the format explicitly.[^23]
- `EXPLAIN ANALYZE` **(8.0.18+)** **executes** the statement and reports actual rows, loops and time per iterator. Never run it on production data with expensive queries on the assumption that it is harmless.
- Signals: `Table scan` or `Index scan` over many rows in a hot path; `Filter` above a scan with a large `rows` estimate; `Sort` on big inputs; `Materialize`/temporary tables; `Nested loop` whose inner side has no index; `Hash join` (8.0.18+) where an index is expected. Compare estimated and actual rows: large gaps point to missing statistics (`ANALYZE TABLE`, histograms).
- `EXPLAIN FOR CONNECTION <id>` shows the plan of a running statement.

```sql
EXPLAIN ANALYZE SELECT id, total FROM orders WHERE status = 'paid' ORDER BY created_at, id LIMIT 20 OFFSET 5000;
-- -> Limit/Offset: 20/5000 row(s)  (actual ... rows=20 loops=1)
--     -> Index lookup on orders using ix_orders_status_created (status='paid')  (actual ... rows=5020 loops=1)
```

***

#### Query Patterns and Performance

##### Pagination: OFFSET vs keyset — and the row-constructor trap

`LIMIT n OFFSET m` reads and discards `m` rows (5,020 rows read above to return 20). Use keyset (seek) pagination with a deterministic order that ends in a unique column.

**MySQL does not use range access for row-constructor inequalities** like `(created_at, id) > (?, ?)` (verified on 8.0, 8.4 and 9.7). The predicate becomes a `Filter` over an index scan that starts at the beginning — as slow as `OFFSET` on later pages. Write the expanded `OR` form:[^24]

```sql
-- BAD on MySQL: filter over the index from its start
EXPLAIN FORMAT=TREE SELECT id, total FROM orders
WHERE status = 'paid' AND (created_at, id) > ('2024-01-10 00:00:00', 13000)
ORDER BY created_at, id LIMIT 20;
-- -> Limit: 20 row(s)
--     -> Filter: ((orders.created_at,orders.id) > ('2024-01-10 00:00:00',13000))
--         -> Index lookup on orders using ix_orders_status_created (status='paid')

-- GOOD: expanded predicate → range scan starting at the cursor
EXPLAIN FORMAT=TREE SELECT id, total FROM orders
WHERE status = 'paid'
  AND (created_at > '2024-01-10 00:00:00'
       OR (created_at = '2024-01-10 00:00:00' AND id > 13000))
ORDER BY created_at, id LIMIT 20;
-- -> Limit: 20 row(s)
--     -> Index range scan on orders using ix_orders_status_created over (status = 'paid' AND created_at = '2024-01-10 00:00:00.000' AND 13000 < id) OR (status = 'paid' AND '2024-01-10 00:00:00.000' < created_at) ...
```

Row-constructor equality `(a, b) = (?, ?)` and `IN ((?, ?), ...)` are optimized normally.

##### COUNT(*)

InnoDB has no stored row count: `SELECT COUNT(*) FROM t` scans the smallest index. For dashboards use estimates (`information_schema.tables.table_rows`) or maintained counters; filtered counts need a covering index on the filter columns. `SQL_CALC_FOUND_ROWS`/`FOUND_ROWS()` are deprecated since 8.0.17 (warning 1287) and usually slower than a separate `COUNT(*)`.

##### N+1 queries and IN lists

Replace per-row queries with one join or one `IN (...)` batch. Keep `IN` lists bounded (hundreds to a few thousand values): huge lists cost parse time and can exceed `range_optimizer_max_mem_size` (8 MiB default), after which range access is abandoned (with a warning). For large sets, load keys into a temporary table and join.

##### Subqueries: MySQL-specific restrictions

```sql
-- Cannot modify a table and select from it in a subquery:
UPDATE users SET phone = '' WHERE id IN (SELECT id FROM users WHERE email LIKE 'zzz%');
-- ERROR 1093 (HY000): You can't specify target table 'users' for update in FROM clause
-- Use a multi-table UPDATE/DELETE with JOIN instead:
UPDATE users u JOIN (SELECT id FROM users WHERE email LIKE 'zzz%') AS x ON x.id = u.id SET u.phone = '';

-- LIMIT is not allowed inside IN/ALL/ANY/SOME subqueries:
SELECT id FROM users WHERE id IN (SELECT id FROM users ORDER BY id LIMIT 3);
-- ERROR 1235 (42000): This version of MySQL doesn't yet support 'LIMIT & IN/ALL/ANY/SOME subquery'
SELECT u.id FROM users u JOIN (SELECT id FROM users ORDER BY id LIMIT 3) AS top3 USING (id);
```

`NOT IN` with a subquery that can return `NULL` returns no rows:

```sql
CREATE TABLE nk (id INT PRIMARY KEY);
INSERT INTO nk VALUES (1), (2);
CREATE TABLE ni (v INT);
INSERT INTO ni VALUES (1), (NULL);
SELECT COUNT(*) FROM nk WHERE id NOT IN (SELECT v FROM ni);                        -- 0
SELECT COUNT(*) FROM nk WHERE NOT EXISTS (SELECT 1 FROM ni WHERE ni.v = nk.id);    -- 1
```

Use `NOT EXISTS` (or `LEFT JOIN ... IS NULL`), or make the column `NOT NULL`.

##### Top-N per group: window functions and LATERAL (8.0+)

```sql
-- Latest order per user (window functions: 8.0+)
WITH ranked AS (
  SELECT o.id, o.user_id, o.created_at,
         ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC, id DESC) AS rn
  FROM orders o WHERE user_id IN (1, 2, 3)
)
SELECT user_id, id, created_at FROM ranked WHERE rn = 1 ORDER BY user_id;

-- Same with LATERAL (8.0.14+): one index probe per user
SELECT u.id, lo.id AS last_order
FROM users u,
     LATERAL (SELECT id FROM orders o WHERE o.user_id = u.id ORDER BY created_at DESC, id DESC LIMIT 1) AS lo
WHERE u.id IN (1, 2, 3) ORDER BY u.id;

-- 5.7-compatible: join against a grouped derived table (ties on created_at need a tie-breaker)
SELECT o.user_id, o.id, o.created_at
FROM orders o
JOIN (SELECT user_id, MAX(created_at) AS mx FROM orders WHERE user_id IN (1, 2, 3) GROUP BY user_id) AS m
  ON m.user_id = o.user_id AND m.mx = o.created_at
ORDER BY o.user_id;
```

- Give `ROW_NUMBER()` a total order (add the primary key), otherwise results change between runs.
- The default window frame with `ORDER BY` is `RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW` (peers included); write `ROWS UNBOUNDED PRECEDING` for running totals.
- **BAD**: the 5.7-era user-variable trick (`@rn := IF(@prev = user_id, @rn + 1, 1)`): evaluation order is undefined, and assigning variables in expressions is deprecated since 8.0.13.
- `QUALIFY` is a reserved word since 8.4 but usable only with the hypergraph optimizer on 9.7+ (8.4: `ERROR 6037`); filter the window result in an outer query.

##### CTEs and recursion (8.0+)

```sql
WITH RECURSIVE r (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r WHERE n < 5)
SELECT GROUP_CONCAT(n) FROM r;                                  -- 1,2,3,4,5
SET SESSION cte_max_recursion_depth = DEFAULT;
WITH RECURSIVE r (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r)
SELECT COUNT(*) FROM r;
-- ERROR 3636 (HY000): Recursive query aborted after 1001 iterations. Try increasing @@cte_max_recursion_depth to a larger value.
```

Recursive CTEs over data (trees, graphs) need a termination condition that does not assume acyclic data: carry a depth or path and stop on cycles. Raise `cte_max_recursion_depth` per session or with `/*+ SET_VAR(cte_max_recursion_depth = N) */`, not globally. `WITH x AS MATERIALIZED` (PostgreSQL) is a syntax error; use `MERGE`/`NO_MERGE` hints.

##### Set operations, GROUP_CONCAT, misc

- `UNION` de-duplicates (extra work); use `UNION ALL` unless duplicates must go. `INTERSECT`/`EXCEPT` exist since **8.0.31** only.
- `GROUP_CONCAT` truncates at `group_concat_max_len` (1024 bytes by default) with only a warning; ID lists and JSON built with it silently lose data. Use `JSON_ARRAYAGG()` (5.7.22+) or raise `group_concat_max_len` for the session.

```sql
SELECT LENGTH(GROUP_CONCAT(email)) FROM users WHERE id <= 200;   -- 1024 (Warning 1260: Row 55 was cut by GROUP_CONCAT())
SELECT JSON_LENGTH(JSON_ARRAYAGG(email)) FROM users WHERE id <= 200;   -- 200
```

- `ORDER BY RAND() LIMIT n` sorts the whole table; pick random keys in the application.
- `SELECT *` matters on wide tables, with `TEXT`/`BLOB`/`JSON` columns and when a covering index could answer the query; it also breaks positional code when columns are added.
- `DISTINCT` to remove duplicates produced by a join usually hides a wrong join; fix the join or use `EXISTS`.
- `LIMIT` without a deterministic `ORDER BY` returns arbitrary rows, and different ones after a plan change.

##### Optimizer hints

```sql
SELECT /*+ MAX_EXECUTION_TIME(1000) */ COUNT(*) FROM orders;      -- read-only SELECT timeout in ms (5.7.8+)
SELECT /*+ SET_VAR(sort_buffer_size = 1048576) */ COUNT(*) FROM users;   -- per-statement variable (8.0+)
```

Prefer optimizer hints (`/*+ ... */`; `INDEX`/`NO_INDEX` since 8.0.20) to `FORCE/USE/IGNORE INDEX`: a hint naming a missing index only warns (3128), while `FORCE INDEX (missing)` fails with `ERROR 1176`. Every hint is a maintenance liability; require a comment with the measured reason. Some proxies and clients strip comments and with them the hints.

***

#### Writes: INSERT, Upserts, UPDATE, DELETE

##### Batch inserts and bulk loads

- Multi-row `INSERT ... VALUES (...), (...)` in transactions of a few hundred to a few thousand rows is the standard fast path. Bound the batch by `max_allowed_packet` (64 MiB default since 8.0; 4 MiB on 5.7).
- `LOAD DATA INFILE` reads server-side files restricted by `secure_file_priv` and the `FILE` privilege. `LOAD DATA LOCAL INFILE` needs `local_infile=ON` on the server (default `OFF` since 8.0, `ON` on 5.7) **and** client opt-in, and lets the server request any file the client can read — keep it off unless the client is a trusted batch job.
- For one-off large loads: load into a staging table, then `INSERT ... SELECT` in primary-key batches; disable nothing globally (`foreign_key_checks`, `unique_checks`) in shared environments.
- `LAST_INSERT_ID()` after a multi-row insert returns the **first** generated id; with `innodb_autoinc_lock_mode=2` (default since 8.0) ids of concurrent inserts interleave, so do not assume a contiguous range.

##### INSERT ... ON DUPLICATE KEY UPDATE

```sql
CREATE TABLE inv (sku VARCHAR(20) PRIMARY KEY, qty INT NOT NULL, id INT AUTO_INCREMENT UNIQUE);
INSERT INTO inv (sku, qty) VALUES ('a', 1);

-- Deprecated since 8.0.20 (warning 1287), but the only form on 5.7 and 8.0 < 8.0.19:
INSERT INTO inv (sku, qty) VALUES ('a', 5) ON DUPLICATE KEY UPDATE qty = qty + VALUES(qty);

-- GOOD on 8.0.19+: row alias
INSERT INTO inv (sku, qty) VALUES ('a', 5), ('b', 2) AS new
ON DUPLICATE KEY UPDATE qty = inv.qty + new.qty;
SELECT ROW_COUNT();                  -- 3  (2 for the updated row, 1 for the inserted row)
SELECT sku, qty, id FROM inv ORDER BY id;   -- a | 11 | 1 ;  b | 2 | 3   (id 2 was burnt by the first upsert)
```

Pitfalls to flag:

- **Affected rows**: 1 = inserted, 2 = updated, 0 = matched but unchanged. Code that checks `RowsAffected() == 1` as "row exists" is wrong; with the client flag `CLIENT_FOUND_ROWS` (Go `clientFoundRows=true`, JDBC `useAffectedRows=false` default) an unchanged match reports 1.
- **`AUTO_INCREMENT` values are consumed by every attempted insert**, also when the row is updated; upsert-heavy tables burn ids quickly (`INT` exhaustion).
- **Several unique keys**: if the new row conflicts with different rows on different unique keys, only one row is updated and which one is unspecified. Upsert only against one relevant unique constraint.
- `LAST_INSERT_ID()` after an update branch does not return that row's id unless the update list contains `id = LAST_INSERT_ID(id)`.

##### REPLACE and INSERT IGNORE

`REPLACE` is `DELETE` + `INSERT`: it changes the auto-increment id, fires delete triggers and (on 8.4 and older) **cascades the delete to child rows**:

```sql
CREATE TABLE par (id INT PRIMARY KEY, name VARCHAR(10));
CREATE TABLE chi (id INT PRIMARY KEY, par_id INT,
                  CONSTRAINT fk_chi_par FOREIGN KEY (par_id) REFERENCES par (id) ON DELETE CASCADE);
INSERT INTO par VALUES (1, 'one');
INSERT INTO chi VALUES (100, 1), (101, 1);
REPLACE INTO par VALUES (1, 'uno');
SELECT COUNT(*) AS children_left FROM chi;   -- 8.0.46 / 8.4.13: 0   9.7.4: 2

INSERT IGNORE INTO chi VALUES (102, 999);
-- Warning 1452: Cannot add or update a child row: a foreign key constraint fails ...  (row silently dropped)
```

Flag both unless delete-and-reinsert or ignore-every-error semantics are explicitly intended; use `INSERT ... ON DUPLICATE KEY UPDATE`.

##### UPDATE/DELETE in batches

```sql
-- GOOD: bounded batches, each its own short transaction; repeat until ROW_COUNT() = 0
DELETE FROM orders WHERE status = 'cancelled' AND created_at < '2024-01-02' ORDER BY id LIMIT 500;
```

- One huge `UPDATE`/`DELETE` creates a long transaction, a large undo log, long-held locks and replica lag. Batch by primary key and pause when replicas lag.
- `UPDATE`/`DELETE ... LIMIT` without `ORDER BY` is nondeterministic (and unsafe for statement-based replication).
- Multi-table `UPDATE a JOIN b ... SET` and `DELETE a FROM a JOIN b ...` are MySQL's forms of `UPDATE ... FROM`; `ORDER BY`/`LIMIT` are not allowed with them.
- An `UPDATE`/`DELETE` whose `WHERE` is not served by an index locks every row it scans (see Locking).

***

#### Transactions, Isolation and Locking

##### Consistent reads vs current reads

Under REPEATABLE READ the snapshot is taken at the **first read** in the transaction (`START TRANSACTION WITH CONSISTENT SNAPSHOT` takes it immediately). Plain `SELECT` reads the snapshot; `UPDATE`, `DELETE`, `INSERT ... SELECT` and locking reads see the latest committed data.[^25] Read-modify-write in application code therefore loses updates:

```text
-- Session A                                        -- Session B
START TRANSACTION;
SELECT bal FROM acct WHERE id = 10;   -- 100
                                                    UPDATE acct SET bal = bal - 50 WHERE id = 10;  -- autocommit
SELECT bal FROM acct WHERE id = 10;   -- 100 (snapshot)
UPDATE acct SET bal = 100 - 30 WHERE id = 10;   -- app computed from the stale 100
COMMIT;
SELECT bal FROM acct WHERE id = 10;   -- 70: B's withdrawal of 50 is lost
```

Fixes, in order of preference:

1. Do the arithmetic in SQL and check affected rows: `UPDATE acct SET bal = bal - 30 WHERE id = 10 AND bal >= 30`.
2. Lock before reading: `SELECT ... FOR UPDATE` inside the transaction.
3. Optimistic concurrency: `UPDATE ... SET ..., version = version + 1 WHERE id = ? AND version = ?`; retry on 0 rows.

MySQL's REPEATABLE READ does **not** detect write skew or raise serialization failures like PostgreSQL; `SERIALIZABLE` turns plain `SELECT`s into shared locking reads (with autocommit off) and rarely is the right fix.

##### Locks taken by locking reads and writes

InnoDB locks **every index record it scans**, plus the gap before it (next-key locks) under REPEATABLE READ:[^26]

```sql
CREATE TABLE acct (id INT PRIMARY KEY, owner INT NOT NULL, bal INT NOT NULL, note VARCHAR(10), KEY ix_owner (owner));
INSERT INTO acct VALUES (10,1,100,'a'), (20,2,100,'b'), (30,3,100,'c'), (40,4,100,'d');

START TRANSACTION;
SELECT * FROM acct WHERE id BETWEEN 15 AND 25 FOR UPDATE;
SELECT index_name, lock_type, lock_mode, lock_data FROM performance_schema.data_locks
WHERE object_name = 'acct' ORDER BY lock_type, lock_data;
-- PRIMARY | RECORD | X     | 20     (next-key lock: record 20 + gap before it)
-- PRIMARY | RECORD | X,GAP | 30     (gap before 30: inserts of 21..29 block)
-- NULL    | TABLE  | IX    | NULL
ROLLBACK;

START TRANSACTION;
SELECT * FROM acct WHERE note = 'b' FOR UPDATE;      -- note is not indexed
SELECT COUNT(*) FROM performance_schema.data_locks WHERE object_name = 'acct' AND lock_type = 'RECORD';
-- 5: every row plus the supremum — the whole table is locked
ROLLBACK;
```

- A locking read or `UPDATE`/`DELETE` whose `WHERE` is not served by an index locks the whole table under REPEATABLE READ. Check with `EXPLAIN`.
- "Select, then insert if absent" with `SELECT ... FOR UPDATE` on a missing key takes a gap lock; two sessions doing it concurrently both get it and deadlock on their inserts. Use `INSERT ... ON DUPLICATE KEY UPDATE`, or insert first and handle error 1062.
- READ COMMITTED (requires row-based binary logging, the default) takes far fewer gap locks and suits many OLTP services; set it explicitly (`transaction_isolation`; the 5.7 name `tx_isolation` is gone in 8.0) and review code that assumed a stable snapshot.
- `performance_schema.data_locks` (8.0+) shows all locks; 5.7 only has `information_schema.innodb_locks` (locks involved in waits), which 8.0 removed.

##### Deadlocks and lock wait timeouts

- **Error 1213 (deadlock)** rolls back the **whole transaction**. Retry the whole transaction (bounded attempts, backoff), not the failing statement.
- **Error 1205 (lock wait timeout, `innodb_lock_wait_timeout` = 50 s)** rolls back **only the statement** by default (`innodb_rollback_on_timeout=OFF`). The transaction stays open with its earlier changes; code that catches 1205 and then commits commits a partial transaction. Roll back explicitly before retrying:

```sql
-- another session holds a lock on acct id = 30
SET SESSION innodb_lock_wait_timeout = 1;
START TRANSACTION;
UPDATE acct SET bal = 0 WHERE id = 40;   -- OK
UPDATE acct SET bal = 0 WHERE id = 30;   -- ERROR 1205 (HY000): Lock wait timeout exceeded; try restarting transaction
SELECT trx_state, trx_rows_modified FROM information_schema.innodb_trx
WHERE trx_mysql_thread_id = CONNECTION_ID();     -- RUNNING | 1   (still open)
ROLLBACK;                                        -- not COMMIT
```

- Prevent deadlocks: touch rows in a consistent order (sort ids before updating), keep transactions short, never wait for users or network calls inside a transaction, index columns used in locking `WHERE` clauses.
- Diagnose with `SHOW ENGINE INNODB STATUS` (`LATEST DETECTED DEADLOCK`), `innodb_print_all_deadlocks=ON`, and `sys.innodb_lock_waits`.

##### Queues: SKIP LOCKED and NOWAIT (8.0+)

```sql
CREATE TABLE jobs (id INT AUTO_INCREMENT PRIMARY KEY,
                   status ENUM('ready','running','done') NOT NULL DEFAULT 'ready',
                   run_at DATETIME NOT NULL,
                   KEY ix_jobs_status_run (status, run_at));
INSERT INTO jobs (run_at) VALUES ('2024-01-01'), ('2024-01-02'), ('2024-01-03'), ('2024-01-04');
START TRANSACTION;
SELECT id FROM jobs WHERE status = 'ready' ORDER BY run_at LIMIT 2 FOR UPDATE SKIP LOCKED;   -- 1, 2
UPDATE jobs SET status = 'running' WHERE id IN (1, 2);
COMMIT;
-- A concurrent worker running the same SELECT while the first transaction is open gets 3, 4;
-- with FOR UPDATE NOWAIT instead it fails at once:
--   ERROR 3572 (HY000): Statement aborted because lock(s) could not be acquired immediately and NOWAIT is set.
```

- `SKIP LOCKED` needs an index matching the filter and order; otherwise each worker scans and locks far more rows.
- `SKIP LOCKED` results are an inconsistent view by design; never use it for anything but work queues.
- On 5.7 none of this exists (`ERROR 1064`); claim jobs with an atomic `UPDATE jobs SET status = 'running', owner = ? WHERE status = 'ready' ORDER BY run_at LIMIT 1`, then read the claimed row by owner.
- `FOR SHARE` and `FOR UPDATE OF tbl` are 8.0+; `LOCK IN SHARE MODE` is the older spelling of `FOR SHARE`.

##### Implicit commits and autocommit

```sql
START TRANSACTION;
INSERT INTO par VALUES (2, 'two');
CREATE TABLE tmp_x (id INT);     -- implicit COMMIT
ROLLBACK;
SELECT * FROM par WHERE id = 2;  -- 2 | two   (the insert was committed)
```

DDL, `LOCK TABLES`, `START TRANSACTION` inside a transaction, `SET autocommit = 1`, account and privilege statements implicitly commit. Do not mix DDL into transactional application logic or tests that rely on rollback; `CREATE TEMPORARY TABLE` does not commit.

Long-running or idle-in-transaction connections hold undo history (purge lag, growing undo tablespaces), locks, and metadata locks that block DDL. Alert on old entries in `information_schema.innodb_trx`.

Avoid MyISAM/MEMORY tables in new schemas: they are not transactional and do not roll back. Since 9.5 `gtid_mode=ON` is the default, and mixing them with InnoDB writes in one transaction fails with `ERROR 1785` even without replicas.

***

#### Schema Migrations and Online DDL

##### No transactional DDL, no IF NOT EXISTS for columns or indexes

A multi-statement migration is applied statement by statement: if statement 3 fails, statements 1–2 stay applied, and the migration tool's "rollback" does nothing for DDL. Write one DDL change per migration (or make each step idempotent by checking `information_schema` in the tool), and combine changes to one table into **one `ALTER TABLE`** (one rebuild, one metadata-lock window):

```sql
-- BAD (MariaDB syntax): ERROR 1064 on MySQL
ALTER TABLE orders ADD COLUMN IF NOT EXISTS note VARCHAR(100);
CREATE INDEX IF NOT EXISTS ix_orders_total ON orders (total);

-- GOOD: one statement, explicit algorithm so it fails fast instead of silently copying (INSTANT: 8.0.12+)
ALTER TABLE orders
  ADD COLUMN note VARCHAR(100) NULL,
  ADD COLUMN source VARCHAR(20) NULL,
  ALGORITHM = INSTANT;
```

`CREATE TABLE IF NOT EXISTS`, `DROP TABLE IF EXISTS`, `CREATE OR REPLACE VIEW` and `DROP VIEW IF EXISTS` exist everywhere; `CREATE VIEW IF NOT EXISTS` only since 9.1. A single DDL statement is atomic since 8.0; on 5.7 `DROP TABLE a, missing` drops `a` and then fails.

##### ALGORITHM and LOCK

Specify `ALGORITHM` (and `LOCK` for `INPLACE`) on every `ALTER TABLE` against a large table. Without it MySQL silently picks the "best" algorithm, which may be a full table copy that blocks writes; with it, an unsupported combination fails immediately:[^27]

| Operation | INSTANT | INPLACE, LOCK=NONE | Notes |
|---|---|---|---|
| `ADD COLUMN` (nullable or with default) | 8.0.12+ (last position); 8.0.29+ any position | yes (rebuild) | INSTANT consumes a row version (8.0.29+) |
| `DROP COLUMN` | 8.0.29+ | yes (rebuild) | |
| `RENAME COLUMN` | 8.0.28+ | yes | |
| `SET/DROP DEFAULT`, add `VIRTUAL` generated column | 8.0.12+ | yes | |
| Add `STORED` generated column, `ADD CHECK`, `CONVERT TO CHARACTER SET`, change column type (`INT → BIGINT`) | no | no | `COPY`: blocks writes |
| `MODIFY VARCHAR(50) → VARCHAR(60)` (utf8mb4) | no | yes | the 1-byte length prefix (≤ 255 bytes) is kept |
| `MODIFY VARCHAR(60) → VARCHAR(100)` (utf8mb4) | no | **no** | 240 → 400 bytes crosses the 255-byte length-prefix boundary: `COPY` |
| `NULL → NOT NULL` | no | yes (rebuild) | fails if rows contain `NULL` |
| `ADD INDEX`, `DROP INDEX`, `RENAME INDEX` | no | yes | |
| `ADD FOREIGN KEY` | no | only with `foreign_key_checks=0` | `COPY` with checks on |

```sql
CREATE TABLE dd (id INT PRIMARY KEY, a VARCHAR(50));
ALTER TABLE dd MODIFY a VARCHAR(60), ALGORITHM=INSTANT;
-- ERROR 1845 (0A000): ALGORITHM=INSTANT is not supported for this operation. Try ALGORITHM=COPY/INPLACE.
ALTER TABLE dd MODIFY a VARCHAR(60), ALGORITHM=INPLACE, LOCK=NONE;     -- OK
ALTER TABLE dd MODIFY a VARCHAR(100), ALGORITHM=INPLACE, LOCK=NONE;
-- ERROR 1846 (0A000): ALGORITHM=INPLACE is not supported. Reason: Cannot change column type INPLACE. Try ALGORITHM=COPY.
ALTER TABLE dd MODIFY id BIGINT, ALGORITHM=INPLACE, LOCK=NONE;
-- ERROR 1846 (0A000): ALGORITHM=INPLACE is not supported. Reason: Cannot change column type INPLACE. Try ALGORITHM=COPY.
```

- On 5.7 there is no `INSTANT` (`ERROR 1800 Unknown ALGORITHM 'INSTANT'`); `ADD COLUMN` rebuilds the table.
- Even `INPLACE, LOCK=NONE` operations that rebuild the table cost I/O, temporary disk space up to the table size, and replica lag (the replica applies the `ALTER` only after it finished on the source). Concurrent writes are buffered in `innodb_online_alter_log_max_size` (128 MiB); on overflow the `ALTER` fails at the end.
- For very large tables use `gh-ost` or `pt-online-schema-change`, and review their configuration like the migration itself.

##### INSTANT row-version limit

Each `ALGORITHM=INSTANT` `ADD`/`DROP COLUMN` (8.0.29+) increments the table's row version. The limit is **64** on 8.0 and 8.4 and **255** since 9.1; at the limit, an explicit `ALGORITHM=INSTANT` fails (`ERROR 4092`) and an `ALTER` without `ALGORITHM` silently falls back to a full rebuild during the deploy. Watch `information_schema.innodb_tables.total_row_versions` for tables that ORMs or feature flags alter frequently.

```sql
ALTER TABLE dd ADD COLUMN b INT, ALGORITHM=INSTANT;
SELECT total_row_versions FROM information_schema.innodb_tables WHERE name = CONCAT(DATABASE(), '/dd');   -- 1
```

##### Metadata locks: the most common migration outage

Every DDL, even `ALGORITHM=INSTANT`, needs an exclusive metadata lock (MDL) for a moment. It waits for **every open transaction that has touched the table** — including an idle one that only ran a `SELECT` — and while it waits, **every new query on the table queues behind it**:[^28]

```text
-- Session A (a pooled connection left in a transaction)
START TRANSACTION; SELECT COUNT(*) FROM acct;      -- ... stays open
-- Session B (the migration)
ALTER TABLE acct ADD COLUMN extra INT, ALGORITHM=INSTANT;      -- waits: "Waiting for table metadata lock"
-- Session C (normal traffic): blocked behind B for as long as A stays open
SELECT COUNT(*) FROM acct;                                      -- waits too
```

`lock_wait_timeout` (which governs MDL waits) defaults to **31,536,000 seconds (one year)**. Every migration session should set a short value and retry:

```sql
SET SESSION lock_wait_timeout = 5;
ALTER TABLE acct ADD COLUMN extra INT, ALGORITHM=INSTANT;
-- under contention: ERROR 1205 (HY000): Lock wait timeout exceeded; try restarting transaction — traffic resumes
SET SESSION lock_wait_timeout = DEFAULT;
```

Find blockers with `sys.schema_table_lock_waits` and `performance_schema.metadata_locks` (on 5.7 the MDL instrument is disabled by default).

##### Changing columns safely (expand/contract)

- Renaming or dropping a column or table breaks the running application version during a rolling deploy. Expand/contract: add the new column, dual-write, backfill in batches, switch reads, drop the old column in a later release.
- Converting a nullable column to `NOT NULL` rebuilds the table and fails on existing `NULL`s: backfill first.
- Widen `INT` keys to `BIGINT` before they are exhausted; it is a full copy.
- `RENAME TABLE a TO a_old, a_new TO a` is atomic and is the swap step of copy-based tools.
- Backfills (`UPDATE ... SET new_col = ...`) run in primary-key batches, not as one statement.
- `DELIMITER` is a `mysql` client command, not SQL: migration tools and drivers that send statements one at a time must not include it. Since 9.4 the `mysql` client also disables most client commands (`source`, `\G`, `system`, ...) unless started with `--commands`; init scripts that rely on `source` break there.[^29]

##### Partitioning

Partitioning is a data-lifecycle tool (drop a month with `ALTER TABLE ... DROP PARTITION`), not a general speed-up. Every unique key, including the primary key, must contain all partitioning columns; partitioned InnoDB tables cannot have foreign keys; queries without a filter on the partition key touch all partitions.

```sql
CREATE TABLE pt (id INT, created DATE, PRIMARY KEY (id)) PARTITION BY RANGE (YEAR(created)) (
  PARTITION p2024 VALUES LESS THAN (2025), PARTITION pmax VALUES LESS THAN MAXVALUE);
-- ERROR 1503 (HY000): A PRIMARY KEY must include all columns in the table's partitioning function (prefixed columns are not considered).
```

***

#### Stored Programs, Triggers, Views and Events

##### Stored functions and binary logging

With binary logging enabled (default since 8.0; off on 5.7), creating a stored function requires a characteristic that declares it safe:[^30]

```sql
DELIMITER //
CREATE FUNCTION f_nd(x INT) RETURNS INT BEGIN RETURN x + 1; END//
-- ERROR 1418 (HY000): This function has none of DETERMINISTIC, NO SQL, or READS SQL DATA in its declaration and
--   binary logging is enabled (you *might* want to use the less safe log_bin_trust_function_creators variable)
CREATE FUNCTION f_ok(x INT) RETURNS INT DETERMINISTIC NO SQL BEGIN RETURN x + 1; END//
DELIMITER ;
```

Declaring `DETERMINISTIC` on a function that is not (uses `NOW()`, `RAND()`, `UUID()`, reads changing tables) is a lie the optimizer and replication rely on; flag it. Setting `log_bin_trust_function_creators=ON` globally to make migrations pass is a smell (and the variable is deprecated since 8.0.34).

##### Error handling: handlers, SIGNAL, RESIGNAL

```sql
DELIMITER //
CREATE PROCEDURE withdraw(IN p_id INT, IN p_amt INT)
BEGIN
  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    ROLLBACK;
    RESIGNAL;                 -- re-raise the original error to the caller
  END;
  START TRANSACTION;
  UPDATE acct SET bal = bal - p_amt WHERE id = p_id;
  IF (SELECT bal FROM acct WHERE id = p_id) < 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'insufficient funds', MYSQL_ERRNO = 5001;
  END IF;
  COMMIT;
END//
DELIMITER ;
CALL withdraw(10, 500);   -- ERROR 5001 (45000): insufficient funds   (balance unchanged: 100)
CALL withdraw(10, 5);     -- OK: 95
```

- **BAD**: `DECLARE CONTINUE HANDLER FOR SQLEXCEPTION BEGIN END;` — swallows every error and continues with partial work.
- **BAD**: an `EXIT HANDLER` that neither rolls back nor re-signals: the caller sees success while the transaction stays open.
- A procedure that runs its own `START TRANSACTION` implicitly commits the caller's open transaction. Document which side owns the transaction.
- Dynamic SQL (`PREPARE stmt FROM CONCAT(...)`) has the same injection risk as application code: pass values with `EXECUTE stmt USING @v`; placeholders cannot stand for identifiers, so validate identifiers against an allow-list.

```sql
SET @lim = 2;
PREPARE ps FROM 'SELECT id FROM acct ORDER BY id LIMIT ?';
EXECUTE ps USING @lim;                    -- 10, 20: placeholders work for values, including LIMIT
DEALLOCATE PREPARE ps;
PREPARE ps2 FROM 'SELECT COUNT(*) FROM ?';
-- ERROR 1064 (42000): ... near '?' ...   (identifiers cannot be parameters)
```

##### DEFINER, SQL SECURITY, triggers, views, events

- Views, routines, triggers and events run with their `DEFINER`'s privileges by default (`SQL SECURITY DEFINER`); a definer with broad privileges is a privilege-escalation path. Prefer `SQL SECURITY INVOKER` for views and routines that only encapsulate queries. Dumps carry `DEFINER=` clauses of accounts that may not exist elsewhere; setting another definer needs `SET_USER_ID` (8.0) or `SET_ANY_DEFINER` (8.2+).
- Triggers do not fire for foreign-key cascades or `TRUNCATE TABLE`, cannot modify the table that fired them (`ERROR 1442`), and multiply the cost of bulk statements. Keep them small (auditing, counters) and tested.
- MySQL has no materialized views (9.7.4 accepts `CREATE MATERIALIZED VIEW` but creates an ordinary view). Views with `GROUP BY`, `DISTINCT`, aggregates, `UNION` or `LIMIT` are evaluated with a temporary table, and outer predicates may not be pushed into them; check plans of queries on views.
- The event scheduler is `ON` by default since 8.0 (`OFF` on 5.7: events created there never run unless enabled). Events replicated to a replica are disabled there.

***

#### Security

##### SQL injection: parameters for values, allow-lists for identifiers

Send every value as a bound parameter. Identifiers (table, column, `ORDER BY` column, `ASC`/`DESC`) cannot be parameters and must come from a fixed allow-list in code:

```go
// BAD: concatenation — input "x' OR '1'='1" matches every row
db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE email = '"+input+"'").Scan(&n)

// GOOD: placeholder
db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE email = ?", input).Scan(&n)

// BAD: a placeholder in ORDER BY binds a string constant — the query sorts by a constant, i.e. not at all
db.QueryContext(ctx, "SELECT id FROM users ORDER BY ? LIMIT 10", "email")

// GOOD: allow-list
sortColumns := map[string]string{"email": "email", "created": "created_at"}
col, ok := sortColumns[userInput]
if !ok {
	col = "id"
}
rows, err := db.QueryContext(ctx, "SELECT id FROM users ORDER BY "+col+" LIMIT 50")

// GOOD: IN lists — one placeholder per value
q := "SELECT COUNT(*) FROM users WHERE id IN (?" + strings.Repeat(",?", len(ids)-1) + ")"
```

- Escaping functions (`mysql_real_escape_string`, hand-written `'` → `''` replacement) are not a substitute: correctness depends on the connection character set and on `NO_BACKSLASH_ESCAPES`.
- Keep multi-statement support off (Go `multiStatements=false`, JDBC `allowMultiQueries=false`, PHP `mysqli::multi_query` avoided): with it off, stacked-query payloads are syntax errors.
- ORMs and query builders are safe only in their parameterized APIs; `Raw`, `Literal`, `Expr`, string-built `WHERE` fragments need the same review.

##### LIKE patterns

User input inside `LIKE` must escape `%`, `_` and the escape character `\` (or use an explicit `ESCAPE` clause, required when `NO_BACKSLASH_ESCAPES` is set):

```sql
SELECT COUNT(*) FROM users WHERE email LIKE CONCAT('user1_', '%');      -- 11111: '_' matched any character
SELECT COUNT(*) FROM users WHERE email LIKE CONCAT('user1\\_', '%');    -- 0: escaped
SELECT COUNT(*) FROM users WHERE email LIKE 'user1!_%' ESCAPE '!';      -- 0
```

##### Authentication plugins

| Version | Default plugin | `mysql_native_password` |
|---|---|---|
| 5.7 | `mysql_native_password` | default |
| 8.0 | `caching_sha2_password` | available; deprecated since 8.0.34 |
| 8.4 | `caching_sha2_password` | **disabled** by default (`--mysql-native-password=ON` re-enables) |
| 9.0+ | `caching_sha2_password` | **removed** from the server |

```sql
CREATE USER legacy@'%' IDENTIFIED WITH mysql_native_password BY 'Legacy-pass-1';
-- 8.0.46: OK   8.4.13 / 9.7.4: ERROR 1524 (HY000): Plugin 'mysql_native_password' is not loaded
```

- New code and accounts use `caching_sha2_password`. Upgrades: accounts still on `mysql_native_password` cannot log in after moving to 8.4 (without the option) or 9.x; migrate them first: `ALTER USER u@'%' IDENTIFIED WITH caching_sha2_password BY '...'`.
- The first login after a server restart or cache flush needs **TLS** or RSA key exchange; over plain TCP without either it fails (`ERROR 2061 ... Authentication requires secure connection`) — the classic "works until the database restarts" bug. Use TLS and, in production, require it (`require_secure_transport=ON` or `REQUIRE SSL` per account) with certificate verification in clients. Flag `allowPublicKeyRetrieval=true` (JDBC), `tls=skip-verify` (Go) and `ssl-mode=DISABLED` in production configs.
- Old client libraries (very old PHP `mysqlnd`, `MySQLdb` built against libmysqlclient 5.x, the Node `mysql` package) do not support `caching_sha2_password`; `mysql2` and current connectors do.
- `GRANT ... IDENTIFIED BY` no longer creates users (8.0+: `ERROR 1410`), and `PASSWORD()` is removed; use `CREATE USER`, then `GRANT`. `FLUSH PRIVILEGES` is unnecessary after account statements (and deprecated since 9.2).

##### Least privilege and roles

```sql
CREATE ROLE app_read, app_write;                    -- roles: 8.0+
GRANT SELECT ON t.* TO app_read;
GRANT INSERT, UPDATE, DELETE ON t.* TO app_write;
CREATE USER app@'%' IDENTIFIED BY 'App-pass-123';
GRANT app_read, app_write TO app@'%';
SET DEFAULT ROLE ALL TO app@'%';                    -- without this the roles are granted but NOT active at login
SHOW GRANTS FOR app@'%';
```

- `activate_all_roles_on_login` is `OFF` by default: every role grant needs `SET DEFAULT ROLE` (or `SET ROLE` in the session), otherwise the account has no effective privileges. Since 9.5, roles in `mandatory_roles` are activated at login for every account (`activate_mandatory_roles=ON`); review that list.
- Application accounts get DML on their schema only; migrations run under a separate account with DDL privileges. No `ALL PRIVILEGES ON *.*`, `SUPER`, `FILE`, `PROCESS`, `SYSTEM_USER`, `GRANT OPTION` or `CONNECTION_ADMIN` for applications.
- `_` and `%` in database names in `GRANT ... ON db_name.*` are wildcards (`GRANT ... ON db_1.*` also matches `dbx1`); escape them (`` `db\_1` ``) or avoid them in schema names. The wildcard behaviour is deprecated since 8.0.35 (no warning is printed).
- Host part: prefer specific hosts or networks over `'%'`.
- Credentials never in migrations, code or logs; the Go driver's `FormatDSN()` output contains the password.

##### Other security settings and functions

- `local_infile=OFF` (default since 8.0): keep it off.
- `secure_file_priv` restricts `LOAD DATA INFILE`/`SELECT ... INTO OUTFILE` to one directory; an empty value allows the whole filesystem.
- `MD5()`, `SHA1()`, `SHA2()` are fast hashes, unsuitable for passwords; hash passwords in the application with bcrypt/scrypt/Argon2. `MD5()`/`SHA1()` are deprecated since 9.4 and absent from a default 9.6+ server (`classic_hashing` component); use `SHA2(x, 256)` for checksums.
- `AES_ENCRYPT()` defaults to `block_encryption_mode=aes-128-ecb`: deterministic, no IV, leaks equal plaintexts; there is no authenticated mode. Encrypt in the application; if it must happen in SQL, use a CBC mode with a random IV per value and keep keys out of SQL text (it appears in logs and the processlist):

```sql
SELECT HEX(AES_ENCRYPT('secret', 'key')) = HEX(AES_ENCRYPT('secret', 'key')) AS ecb_deterministic;  -- 1
SET SESSION block_encryption_mode = 'aes-256-cbc';
SET @iv = RANDOM_BYTES(16);
SET @key = UNHEX(SHA2('k', 256));
SELECT AES_DECRYPT(AES_ENCRYPT('secret', @key, @iv), @key, @iv);   -- secret
SET SESSION block_encryption_mode = 'aes-256-gcm';                 -- ERROR 1231: not supported
SET SESSION block_encryption_mode = DEFAULT;
```

- General log, slow log, binary log and `performance_schema` history contain literal values of non-parameterized statements: one more reason to bind parameters.

***

#### Application Code, Drivers and Connection Pooling

Concepts below are shown with Go's `database/sql` and `github.com/go-sql-driver/mysql`; every mature driver has equivalent settings.[^31]

```go
cfg := mysql.NewConfig()
cfg.User, cfg.Passwd, cfg.Net, cfg.Addr, cfg.DBName = "app", os.Getenv("DB_PASSWORD"), "tcp", "db:3306", "app"
cfg.ParseTime = true                                    // DATETIME/TIMESTAMP → time.Time
cfg.Loc = time.UTC                                      // how DATETIME values are interpreted
cfg.Params = map[string]string{"time_zone": "'+00:00'"} // session time_zone for TIMESTAMP conversion
cfg.TLSConfig = "custom"                                // registered with mysql.RegisterTLSConfig, verifying the server
db, err := sql.Open("mysql", cfg.FormatDSN())
db.SetMaxOpenConns(20)
db.SetMaxIdleConns(20)
db.SetConnMaxLifetime(5 * time.Minute)
```

Driver pitfalls:

- **Time handling**: without `parseTime=true` (Go) a `DATETIME` scans as `[]byte`; the driver's location and the session `time_zone` must agree with how data is written, or every `TIMESTAMP` shifts by the offset. JDBC: `connectionTimeZone`/`forceConnectionTimeZoneToSession` (Connector/J 8.0.23+); set them explicitly.
- **Character set**: `charset=utf8mb4` (Go), `characterEncoding=UTF-8` (JDBC), `charset='utf8mb4'` (PyMySQL/mysqlclient). The legacy `utf8` setting means `utf8mb3` and fails on emoji.
- **Client-side interpolation** (`interpolateParams=true` in Go, emulated prepares in PDO, `useServerPrepStmts=false` in JDBC) substitutes parameters in the driver. It is safe only with ASCII-compatible connection character sets; the Go driver refuses it for BIG5, CP932, GB2312, GBK and SJIS. Do not enable it without measurements.
- **Server-side prepared statements** are limited by `max_prepared_stmt_count` (16,382 default); drivers that prepare per query without closing leak statements until the limit fails all prepares. Close statements, or let the pool cache them.
- **Multi-statements off** (see Security).
- **Booleans**: `TINYINT(1)` maps to bool in most drivers; `TINYINT(2)` or `TINYINT UNSIGNED` does not.
- **Large integers**: `BIGINT UNSIGNED` values above 2^63−1 overflow signed 64-bit types; JavaScript drivers return numbers above 2^53 imprecisely unless configured to return strings or BigInt (`mysql2`: `supportBigNumbers`/`bigNumberStrings`).
- **Affected rows**: drivers differ in reporting found vs. changed rows (`clientFoundRows` in Go, `useAffectedRows` in Connector/J); see Upserts.
- **Context and timeouts**: use `QueryContext`/`ExecContext` with deadlines. On cancellation the client abandons the connection, but the server may keep running the statement; bound expensive reads with `MAX_EXECUTION_TIME` too. Always close result sets (`defer rows.Close()`) and check `rows.Err()`.
- **Transactions on pools**: never send `START TRANSACTION`/`COMMIT` as plain statements on a pool (`db.Exec`): they land on different connections. Use the driver's transaction object (`BeginTx`), which can also set the isolation level.
- **Errors by number**, never by message text:

```go
var me *mysql.MySQLError
if errors.As(err, &me) {
	switch me.Number {
	case 1062: // ER_DUP_ENTRY → conflict
	case 1213: // ER_LOCK_DEADLOCK → transaction rolled back: retry the whole transaction
	case 1205: // ER_LOCK_WAIT_TIMEOUT → only the statement rolled back: ROLLBACK, then retry
	case 3572: // ER_LOCK_NOWAIT
	case 3024: // ER_QUERY_TIMEOUT (MAX_EXECUTION_TIME)
	case 1451, 1452: // FK violation on parent delete / child insert
	}
}
```

Connection pooling:

- Set the pool's maximum connection lifetime below the server's `wait_timeout` (28,800 s default) and below any proxy, NAT or load-balancer idle timeout; otherwise the pool hands out dead connections (`invalid connection`, `Communications link failure`, `server has gone away`).
- Total connections across all application instances must stay below `max_connections` (151 default) minus headroom for admin and migrations. Every MySQL connection is a server thread with its own buffers; hundreds of mostly idle connections cost memory and context switches. Use a proxy (ProxySQL, MySQL Router, RDS Proxy) for very high fan-in.
- Session state leaks between pool users: `SET SESSION ...`, user variables, temporary tables, `LOCK TABLES`, `GET_LOCK()` locks and open transactions stay on the connection when it goes back to the pool, and most pools do not reset it. Avoid session state in pooled code, or reset it explicitly before returning the connection; prefer per-statement hints (`SET_VAR`, 8.0+) over `SET SESSION`.
- Retries: only idempotent operations or whole transactions after 1213; never blindly retry a non-idempotent statement after a network error (it may have committed).

***

#### Replication-safe SQL

Most production servers replicate (replicas, Group Replication, CDC tools reading the binary log). Review for:

- **Row-based logging** (`binlog_format=ROW`, default since 5.7.7) makes nondeterministic statements safe to replicate; the `binlog_format` variable is deprecated since 8.0.34 and only `ROW` should be used. Do not set `binlog_format=STATEMENT` per session. Under statement-based logging, `LIMIT` without `ORDER BY`, `SYSDATE()`, `UUID()`, `RAND()` and user-defined functions diverge on replicas.
- **Every table needs a primary key**: row-based replication applies updates and deletes on tables without one by scanning.
- **Large transactions** (bulk `UPDATE`/`DELETE`, data backfills, copying `ALTER TABLE`) cause replica lag proportional to their duration and huge binlog events (`max_binlog_cache_size`); batch them.
- **DDL replicates after it finishes on the source** and is applied serially on replicas: a 1-hour `ALTER` adds an hour of lag; use online schema-change tools for big tables.
- **Read-your-writes**: replica reads are eventually consistent; route such reads to the source or wait with `WAIT_FOR_EXECUTED_GTID_SET()`.
- **GTID consistency** (on by default since 9.5; common in managed services): mixing non-transactional and transactional writes in one transaction fails (`ERROR 1785`); `CREATE TABLE ... SELECT` is GTID-safe only since 8.0.21.
- **Terminology**: `CHANGE MASTER TO`, `START/STOP/RESET SLAVE`, `SHOW SLAVE STATUS`, `SHOW MASTER STATUS`, `RESET MASTER` were deprecated in 8.0.22–8.2 and are **syntax errors since 8.4**; use `CHANGE REPLICATION SOURCE TO` (8.0.23+), `START REPLICA`/`SHOW REPLICA STATUS` (8.0.22+), `SHOW BINARY LOG STATUS`/`RESET BINARY LOGS AND GTIDS` (8.2+). Monitoring must read `Seconds_Behind_Source`, not `Seconds_Behind_Master`. Scripts that must run on both 8.0 and 8.4 need version checks.

***

#### Observability

```sql
-- Top statements by total latency (performance_schema is on by default)
SELECT DIGEST_TEXT, COUNT_STAR, ROUND(SUM_TIMER_WAIT / 1e12, 3) AS total_s,
       SUM_ROWS_EXAMINED, SUM_ROWS_SENT, SUM_NO_INDEX_USED
FROM performance_schema.events_statements_summary_by_digest
WHERE SCHEMA_NAME = DATABASE() ORDER BY SUM_TIMER_WAIT DESC LIMIT 5;

SELECT * FROM sys.statements_with_full_table_scans LIMIT 5;
SELECT table_name, redundant_index_name, dominant_index_name
FROM sys.schema_redundant_indexes WHERE table_schema = DATABASE();
SELECT * FROM sys.innodb_lock_waits;                                     -- row lock waits
SELECT * FROM sys.schema_table_lock_waits;                               -- metadata lock waits
SELECT trx_id, trx_started, trx_mysql_thread_id FROM information_schema.innodb_trx ORDER BY trx_started LIMIT 5;
```

- `SUM_ROWS_EXAMINED / SUM_ROWS_SENT` far above 1 indicates a missing or unused index.
- Slow query log is off by default (`long_query_time=10`); enable it with a threshold matching the latency budget and analyze with `pt-query-digest`.
- `sys.schema_unused_indexes` counts only since the last restart and only on that server (an index unused on the source may serve replica reads); make an index invisible before dropping it.
- `SHOW ENGINE INNODB STATUS` shows the last deadlock and the history list length (purge lag).
- `information_schema.processlist` is deprecated since 8.0.35; use `performance_schema.processlist`.

***

#### Removed and Changed Behaviour Across Versions

When a project upgrades — or when code written for one version runs on another — these are the changes that break SQL or change results.[^9][^10][^11][^12]

##### 5.7 → 8.0

- `GROUP BY` no longer sorts; `GROUP BY col ASC/DESC` is a syntax error (8.0.13).
- `CHECK` constraints are enforced (8.0.16) — existing violating data blocks the next rebuild.
- New reserved words (`RANK`, `ROW_NUMBER`, `GROUPS`, `WINDOW`, `LATERAL`, `OF`, `SYSTEM`, `EMPTY`, `CUBE`, …): quote such identifiers.
- Default character set `utf8mb4_0900_ai_ci` (`NO PAD`, accent-insensitive) instead of `latin1`; joins between old and new tables can fail with `ERROR 1267`.
- `caching_sha2_password` is the default; old clients cannot log in to new accounts.
- `GRANT ... IDENTIFIED BY` user creation, `PASSWORD()`, `NO_AUTO_CREATE_USER`, query cache, `tx_isolation`, `information_schema.innodb_locks` are removed.
- `explicit_defaults_for_timestamp=ON`; binary logging on by default; `innodb_autoinc_lock_mode=2`; `local_infile=OFF`; event scheduler on.

##### 8.0 → 8.4 LTS

- `mysql_native_password` disabled; `default_authentication_plugin` removed (use `authentication_policy`).
- `MASTER`/`SLAVE` replication statements and options removed.
- Foreign keys must reference a `PRIMARY`/`UNIQUE` key; `AUTO_INCREMENT` on `FLOAT`/`DOUBLE` removed; `FLUSH HOSTS` removed.
- `QUALIFY`, `TABLESAMPLE` reserved; InnoDB defaults retuned (adaptive hash index and change buffering off, larger I/O capacity).

```sql
CREATE TABLE tablesample (id INT);   -- 8.0.46: OK   8.4.13 / 9.7.4: ERROR 1064 (reserved since 8.4)
```

##### 8.4 LTS → 9.7 LTS

- Inline `REFERENCES` enforced (9.0); SQL-layer foreign keys change `REPLACE` and `AUTO_INCREMENT` behaviour on FK failures (9.6).
- `mysql_native_password` removed from the server (9.0); `MD5()`/`SHA1()` only via the `classic_hashing` component (9.6) — tables with these functions in generated columns, defaults, `CHECK` constraints or functional indexes break during the upgrade.
- `EXTERNAL` and `LIBRARY` reserved.
- Defaults: `gtid_mode=ON` and `enforce_gtid_consistency=ON`, `explain_format=TREE`, `explain_json_format_version=2` (9.5); `activate_mandatory_roles=ON` (9.5).
- The `mysql` client disables most client commands by default (9.4).
- Result changes: `ROLLUP` over an empty input returns a grand-total row (9.1); `IGNORE` no longer suppresses "Subquery returns more than 1 row" (9.0).

```sql
SELECT @@explain_format, @@gtid_mode;      -- 8.4.13: TRADITIONAL | OFF   9.7.4: TREE | ON
SELECT MD5('a');                          -- 8.4.13: 0cc175b9c0f1b6a831c399e269772661   9.7.4: ERROR 1305
CREATE TABLE external (id INT);           -- 8.4.13: OK   9.7.4: ERROR 1064
```

##### Deprecated everywhere (keep out of new code)

`VALUES(col)` in `ON DUPLICATE KEY UPDATE` (8.0.20), `SQL_CALC_FOUND_ROWS`/`FOUND_ROWS()`, `&&`/`||`/`!`, user-variable assignment inside expressions, integer display width/`ZEROFILL`/`FLOAT(M,D)`, `utf8`/`utf8mb3`, `$` as the first character of unquoted identifiers, `JSON_MERGE()`, `binlog_format`, `information_schema.processlist`, wildcard database grants, `mysql_native_password`, `MD5()`/`SHA1()`.

```sql
SELECT 1 && 1;      -- 1, Warning 1287: '&&' is deprecated and will be removed in a future release. Please use AND instead
```

***

#### Production / Review Checklist

##### Version

- [ ] Server version identified from the project; version-tied features (labels in this guide) only used when the pinned version has them
- [ ] Unknown version: no suggestions beyond 8.0 features without an explicit "requires X" note
- [ ] Not on an end-of-life line (5.7, 8.0, superseded Innovation releases); LTS pinned (`mysql:8.4`/`mysql:9.7`, not `latest`); upgrade path planned without skipping an LTS
- [ ] Not MariaDB/Aurora/HeatWave-specific syntax unless the project runs that platform

##### Schema and migrations

- [ ] Narrow, immutable primary key on every table (`BIGINT UNSIGNED AUTO_INCREMENT` or `BINARY(16)`); no `CHAR(36)` UUID keys
- [ ] `utf8mb4` with an explicit collation consistent with existing tables; `_bin` collation for case-sensitive identifiers
- [ ] `DECIMAL` for money; `BIGINT` for growing ids; no display widths, `ZEROFILL`, `FLOAT(M,D)`
- [ ] `DATETIME(n)` in UTC for business timestamps; no `TIMESTAMP` beyond 2038; explicit `NULL`/`DEFAULT` on temporal columns
- [ ] Foreign keys as named table-level `CONSTRAINT ... FOREIGN KEY` clauses referencing a `PRIMARY`/`UNIQUE` key with identical types; no inline `REFERENCES`
- [ ] `CHECK` constraints only relied upon on 8.0.16+, with `NOT NULL` where needed
- [ ] Every large `ALTER TABLE` states `ALGORITHM` (and `LOCK`), runs with a short `lock_wait_timeout`, combines changes per table, and is compatible with both application versions during the deploy
- [ ] No `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, `RETURNING`, sequences, materialized views or other non-MySQL syntax
- [ ] No `DELIMITER`/client commands in SQL sent by drivers or migration tools

##### Queries

- [ ] `EXPLAIN` checked for new queries on large tables; no full scans in hot paths
- [ ] No functions or implicit type conversions on indexed columns; strings bound for string columns
- [ ] Keyset pagination with the expanded `OR` predicate, not `(a, b) > (?, ?)` and not deep `OFFSET`
- [ ] `NOT EXISTS` instead of `NOT IN (subquery)` on nullable columns
- [ ] Deterministic `ORDER BY` (ending in a unique column) wherever `LIMIT` or `ROW_NUMBER()` is used; `ORDER BY` present wherever order matters after `GROUP BY`
- [ ] No `sql_mode` relaxation; no careless `ANY_VALUE`
- [ ] `GROUP_CONCAT` length considered; JSON functional indexes match expression and collation

##### Writes and transactions

- [ ] Upserts via `INSERT ... ON DUPLICATE KEY UPDATE` (row alias on 8.0.19+); affected-rows semantics and `AUTO_INCREMENT` gaps handled
- [ ] No `REPLACE` on parent tables; no `INSERT IGNORE` unless ignoring every error class is intended
- [ ] Read-modify-write in one `UPDATE`, under `SELECT ... FOR UPDATE`, or with optimistic versioning
- [ ] Locking statements served by an index; consistent lock order
- [ ] Deadlock (1213) retries the whole transaction; lock wait timeout (1205) rolls back before retrying
- [ ] Short transactions, no network calls or DDL inside; large changes batched by primary key

##### Security

- [ ] Values bound as parameters; identifiers from allow-lists; multi-statements off; `LIKE` input escaped
- [ ] Least-privilege accounts; roles activated with `SET DEFAULT ROLE`; separate migration account
- [ ] `caching_sha2_password`; TLS required and verified; no `allowPublicKeyRetrieval`/`skip-verify` in production
- [ ] `local_infile` off; no SQL password hashing; no `AES_ENCRYPT` in ECB mode; `DEFINER`/`SQL SECURITY` reviewed

##### Operations

- [ ] Pool lifetime below `wait_timeout` and proxy timeouts; pool size within `max_connections`; session state not leaked
- [ ] Time zone and character set set explicitly per connection
- [ ] Replication-safe: row-based logging, primary keys, batched bulk changes, current replication terminology
- [ ] Slow log or `performance_schema` digests monitored; deadlocks logged

***

[^1]: MySQL 9.7 Reference Manual — MySQL Releases: Innovation and LTS (release tracks, support periods, calendar versioning after 9.7). https://dev.mysql.com/doc/refman/9.7/en/mysql-releases.html
[^2]: MySQL 26.10 Release Notes (26.7.0 2026-07-28, 26.7.1 2026-08-18, 26.10.0 2026-10-20); Changes in MySQL 26.7.0 (calendar-based versioning after 9.7 LTS). https://dev.mysql.com/doc/relnotes/mysql/26.10/en/ , https://dev.mysql.com/doc/relnotes/mysql/26.10/en/news-26-7-0.html
[^3]: MySQL 5.7 Release Notes (5.7.44, 2023-10-25, last release). https://dev.mysql.com/doc/relnotes/mysql/5.7/en/
[^4]: MySQL 8.0 Release Notes (8.0.46, 2026-04-21; End of Life statement). https://dev.mysql.com/doc/relnotes/mysql/8.0/en/
[^5]: MySQL 8.4 Release Notes (8.4.0 2024-04-30 … 8.4.13 2026-10-20). https://dev.mysql.com/doc/relnotes/mysql/8.4/en/
[^6]: endoflife.date — MySQL (Premier/Extended Support end dates per series). https://endoflife.date/mysql
[^7]: MySQL 9.7 Release Notes (9.0.0 2024-07-01 … 9.7.0 2026-04-21 … 9.7.4 2026-10-20). https://dev.mysql.com/doc/relnotes/mysql/9.7/en/
[^8]: MySQL 9.7 Reference Manual — Upgrade Paths (LTS series cannot be skipped). https://dev.mysql.com/doc/refman/9.7/en/upgrade-paths.html
[^9]: MySQL 8.0 Reference Manual — What Is New in MySQL 8.0 (features added per 8.0.x release, deprecations, removals). https://dev.mysql.com/doc/refman/8.0/en/mysql-nutshell.html
[^10]: MySQL 8.0 Reference Manual — Changes in MySQL 8.0 (upgrading from 5.7). https://dev.mysql.com/doc/refman/8.0/en/upgrading-from-previous-series.html
[^11]: MySQL 8.4 Reference Manual — What Is New in MySQL 8.4 since MySQL 8.0. https://dev.mysql.com/doc/refman/8.4/en/mysql-nutshell.html
[^12]: Changes in MySQL 9.0.0–9.7.0 (release notes: VECTOR, inline REFERENCES, mysql_native_password removal, CREATE VIEW IF NOT EXISTS, MD5/SHA1 component, changed defaults, SQL-layer foreign keys, hypergraph optimizer). https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-0-0.html , https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-5-0.html , https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-6-0.html , https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-7-0.html
[^13]: MariaDB Documentation — Incompatibilities and Feature Differences Between MariaDB and MySQL; MySQL to MariaDB Compatibility Matrix. https://mariadb.com/docs/release-notes/compatibility-and-differences/incompatibilities-and-feature-differences-between-mariadb-10-6-and-mysql-8 , https://mariadb.com/docs/server/server-management/install-and-upgrade-mariadb/migrating-to-mariadb/moving-from-mysql/mysql-to-mariadb-compatibility-matrix
[^14]: MySQL 9.7 Reference Manual — Generated Invisible Primary Keys; Server System Variables (sql_require_primary_key). https://dev.mysql.com/doc/refman/9.7/en/create-table-gipks.html
[^15]: MySQL 9.7 Reference Manual — Legacy Hashing Component (MD5()/SHA1() after 9.6). https://dev.mysql.com/doc/refman/9.7/en/legacy-hashing-component.html
[^16]: MySQL 9.7 Reference Manual — The VECTOR Type. https://dev.mysql.com/doc/refman/9.7/en/vector.html
[^17]: MySQL 8.4 / 9.7 Reference Manual — FOREIGN KEY Constraints (inline REFERENCES ignored before 9.0, non-standard keys, cascades and triggers). https://dev.mysql.com/doc/refman/8.4/en/create-table-foreign-keys.html , https://dev.mysql.com/doc/refman/9.7/en/create-table-foreign-keys.html
[^18]: Changes in MySQL 9.7.0 (enable_cascade_triggers). https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-7-0.html
[^19]: MySQL 8.4 Reference Manual — Unicode Character Sets (PAD SPACE vs NO PAD). https://dev.mysql.com/doc/refman/8.4/en/charset-unicode-sets.html
[^20]: MySQL 8.4 Reference Manual — Server SQL Modes. https://dev.mysql.com/doc/refman/8.4/en/sql-mode.html
[^21]: MySQL 8.4 Reference Manual — MySQL Server Time Zone Support. https://dev.mysql.com/doc/refman/8.4/en/time-zone-support.html
[^22]: MySQL 8.4 Reference Manual — CREATE INDEX Statement (functional key parts, multi-valued indexes). https://dev.mysql.com/doc/refman/8.4/en/create-index.html
[^23]: MySQL 8.4 Reference Manual — EXPLAIN Statement; Changes in MySQL 9.5.0 (explain_format default TREE). https://dev.mysql.com/doc/refman/8.4/en/explain.html , https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-5-0.html
[^24]: MySQL 8.4 Reference Manual — Row Constructor Expression Optimization. https://dev.mysql.com/doc/refman/8.4/en/row-constructor-optimization.html
[^25]: MySQL 8.4 Reference Manual — Consistent Nonlocking Reads. https://dev.mysql.com/doc/refman/8.4/en/innodb-consistent-read.html
[^26]: MySQL 8.4 Reference Manual — Locks Set by Different SQL Statements in InnoDB. https://dev.mysql.com/doc/refman/8.4/en/innodb-locks-set.html
[^27]: MySQL 8.4 Reference Manual — Online DDL Operations. https://dev.mysql.com/doc/refman/8.4/en/innodb-online-ddl-operations.html
[^28]: MySQL 8.4 Reference Manual — Metadata Locking. https://dev.mysql.com/doc/refman/8.4/en/metadata-locking.html
[^29]: MySQL 9.7 Reference Manual — mysql Client Options (--commands). https://dev.mysql.com/doc/refman/9.7/en/mysql-command-options.html
[^30]: MySQL 8.4 Reference Manual — Stored Program Binary Logging. https://dev.mysql.com/doc/refman/8.4/en/stored-programs-logging.html
[^31]: go-sql-driver/mysql — README (parseTime, loc, multiStatements, interpolateParams, clientFoundRows). https://github.com/go-sql-driver/mysql
