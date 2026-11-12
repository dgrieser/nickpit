### Protocols, RPC, and messaging security guide
Applies to: gRPC/Protobuf/Thrift, binary protocols, streaming RPC, webhooks, brokers, queues, pub/sub.

#### Hunt for
- **Parser disagreement**: gateway vs service, old vs new schema decode length, duplicate/unknown fields, encoding, numeric width, envelope-vs-body precedence differently.
- **Enum/default confusion**: unknown variants, missing discriminators, zero values, default branches treated as validated; old consumers reading new fields.
- **Envelope/payload mismatch**: authorization reads envelope; handler acts on tenant, sender, object from body.
- **Interceptor gaps**: authentication or authorization skips streaming, reflection, health, gateway-transcoded, compatibility services; compare every registration of same operation.
- **Channel vs claimed principal**: mTLS, workload identity, broker credentials authenticate channel; caller field picks user or tenant unbound.
- **Stream authorization**: stream, subscription, batch authorized once; later items name other resources or outlive revocation.
- **Correlation IDs**: predictable, reused, cross-tenant IDs let reply, cancel, ack satisfy another caller's operation.
- **Topic scope**: client picks other tenant's topic, wildcard, consumer group, reply queue, dead-letter route; tenant text in payload not isolation.
- **Dead-letter disclosure**: DLQs, error topics, traces carry secrets or cross-tenant payloads to lower-trust consumers; check redaction on failure path.
- **Forged control messages**: admin event, provider callback, replication record accepted without signature and source/audience binding.
- **Duplicate delivery**: redelivery repeats non-idempotent effect; dedup missing, after mutation, or key collides across tenants.
- **Reordering**: stale state, revoked membership, canceled work overwrites newer; no sequence, version, tombstone.
- **Ack/commit order**: ack before durable commit loses security work; commit before ack duplicates mutation; outbox/inbox recovery; multi-consumer transition half-committed.

#### Validate before reporting
- "Internal" not authentication; name peer identity per hop and how it becomes authorization principal.
- Schema validation proves shape only, not provenance, authority, ordering.
- Disagreement: cite both parsers and divergent decoded principal, resource, or operation; safe rejection by either side defeats claim.
- Replay/ordering: establish actual delivery, retry, ack, dedup guarantees first.
- Check every interceptor, gateway, broker ACL before calling authorization missing.
