### AI and LLM security guide
Applies to: prompts, RAG, agent memory, tool calls, MCP, sub-agents, model output.

#### Hunt for
- **Indirect injection**: who writes retrieved documents, issues, tool responses, metadata; retrieval scope; consumer's capabilities.
- **Context bleed**: tenant/ACL filter absent from query or history, embedding, prompt-cache keys; batch paths.
- **Memory poisoning**: who writes memory; provenance, tenant scope lost; low-trust text or summaries becoming durable instructions for others.
- **Role confusion**: concatenation, untyped history, caller-set roles, serialization letting untrusted text pose as system, tool result, memory.
- **Tool arguments**: follow each model-produced field from decoded call to SQL, shell, file, URL-fetch, privileged API sinks; schema fixes shape, not authorization or path/URL safety.
- **Confused deputy**: service identity or broad credential used without requester permission check on named resource; shared credential with enforced per-user scope not defect.
- **Approval binding**: confirmation binds normalized tool, full arguments, requester, target, amount, expiry; retry/resume never runs mutated or duplicate action.
- **Schema vs dispatcher**: aliases, extra fields, duplicate keys, coercion, ranges read differently by validator and handler.
- **Unbounded loops**: repeated spend, sends, writes without budget, per-action authorization, cancellation, idempotency.
- **Delegation**: sub-agents/MCP get full session, credentials, memory; results trusted.
- **MCP identity**: routing by tool name, request ID, model alias instead of authenticated connection and pending request; duplicate tool claims; reconnect rebinding.
- **MCP metadata**: peer descriptions, schemas, prompts used as authorization.
- **Output rendering**: output to HTML, Markdown (auto-loaded resources), template, URL, command unencoded.
- **Context extraction**: credentials, others' data, access-granting values leakable via user-steered output.

#### Validate before reporting
- Prompt injection alone not a finding; need code-level boundary failure: another principal's context, authority requester lacks, data they cannot read, sink they cannot reach.
- Guardrail prompt not a control; only deterministic checks, resource-scoped authorization, isolation, binding, narrow credentials.
- User using own authority via model, generic instruction leak: not defects.
- Confused deputy: attacker cannot act directly. Approval: attacker content caused action victim never requested or approved; victim's valid authorization not intent.
- Memory/retrieval: cite attacker write and later cross-principal read or privileged use.
- Gate runs after arguments final, before every side effect, on direct, queued, retry, resume, delegated paths.
