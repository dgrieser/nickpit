### Data isolation and lifecycle security guide
Applies to: tenant-scoped storage, derived copies, object links, export/import/restore, migrations, deletion, revocation.

#### Hunt for
- **Tenant binding**: query trusts body field over authenticated tenant/owner; compare direct, nested, background, admin, import, legacy paths.
- **Key collision**: cache keys, object paths, unique constraints, search doc IDs, temp files, dedupe keys omit tenant or environment.
- **Policy/query drift**: row-level policy, ORM default scope vs raw, `unscoped`, service clients; joins, aggregates, views, missing request context.
- **Signed references**: object keys, version IDs, share links, signed URLs not bound to operation, exact object and version, audience, expiry, tenant; valid after ACL change.
- **Derived copies**: ACL change not invalidating search, cache, embedding, thumbnail, preview, index; retrieval relies on ingestion filter.
- **Alternate readers**: private data or credentials in logs, traces, analytics, diagnostics with broader access, longer retention, tenant mixing; field names, public IDs, operator-only content under intended policy not enough.
- **Oracles**: counts, filters, ordering, errors, unique-constraint failures, timing, existence checks reveal confidential predicate; need observable distinction, not general response variance.
- **Export scope**: exports, snapshots include other tenants, hidden fields, soft-deleted rows, secrets, history; per-item and download authorization.
- **Import/restore**: archive trusted; owner, ACL, uniqueness, validation not re-applied; overwrites existing resources or writes into foreign tenants.
- **Migrations**: backfill, dual write, rollback, resume give old records wrong tenant, ACL, lifecycle; old and new readers differ in defaults.
- **Soft delete**: lookups, search, relations, links, jobs ignore deleted predicate; deleted identifiers re-registrable while referenced.
- **Stale authorization**: membership removal, downgrade, consent withdrawal, revocation not invalidating sessions, caches, subscriptions, jobs, materialized data.
- **Resurrection**: retries, queued jobs, exports recreate deleted data; undelete or restore revives credentials, memberships, permissions current policy forbids.

#### Validate before reporting
- Cite source-of-truth check and changed path omitting it; confirm no caller, middleware, policy enforces it.
- Tenant field on record not isolation; find query, key, or policy enforcing it on every path.
- Privacy preference alone not vulnerability; need access boundary or deletion/revocation guarantee plus unauthorized reader.
