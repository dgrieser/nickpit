### Cloud and deployment security guide
Applies to: IAM, IaC, containers, Kubernetes/Helm, mesh, serverless, edge, storage, deploy config.

#### Hunt for
- **Workload identity**: pod/function/node identity beyond role, input selecting target; trace request, object, label, event to credential performing final operation and condition that should scope it; policy conditions, wildcards, service-account attachment, fallback credentials.
- **Cross-account roles**: assume-role, external IDs, federation unbound to caller account, audience, repository, namespace; establish both trust policy and caller-controlled claim.
- **Spoofed metadata**: identity headers, tags, account IDs not proven from control plane or proxy.
- **Reachability**: ingress, Service, security group, load-balancer annotation, port mapping, 0.0.0.0 bind exposing admin, debug, metrics, control-plane, internal API to lower-trust network.
- **Mesh bypass**: identity or mTLS subject accepted from peers outside ingress/sidecar; client copies of identity headers not stripped; alternate port, health or legacy path; fail-open without proxy.
- **Metadata service**: untrusted URL to instance metadata, control-plane sockets.
- **Host access**: privileged, added capabilities, hostNetwork/hostPID, hostPath or runtime-socket mounts, automounted service-account tokens.
- **Admission paths**: policy on one deploy route, not another controller, job, upgrade, restore.
- **Labels**: policy selecting on labels, annotations, namespaces lower-trust principals set.
- **Precedence drift**: dev values, chart defaults, env, flags, overlays disabling auth, TLS, tenant scoping, audit; judge rendered result.
- **Secret exposure**: crash reports, process arguments, shared env, broad volumes, read APIs other workloads see.
- **Credential renewal**: failed mount, refresh, revocation keeping stale credentials or weaker identity.
- **Signed URLs**: bucket policy, CDN origin, signed URL unbound to principal, operation, prefix, audience, expiry; list, write.
- **Event sources**: body fields trusted as source over signed envelope, topic, account, replay; push, retry, dead-letter.
- **Edge/origin**: edge assuming secrets, isolation origin lacks; fallback changing authority, caching.

#### Validate before reporting
- Manifest alone not live exposure; establish consuming environment, overlays, active path.
- Broad policy, missing seccomp, read-only filesystem: hardening unless lower-trust input reaches action.
- Secret reference, public endpoint, key ID not disclosure; need lower-trust reader, log, fallback.
- Count ingress, mesh, admission controls only if config and attachment visible; never assume public IP, open firewall, missing admission.
