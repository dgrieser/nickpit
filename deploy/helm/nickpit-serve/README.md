# nickpit-serve Helm chart

Deploys the [`nickpit gitlab serve`](../../..) webhook daemon: an HTTP service that
receives GitLab group webhooks (merge-request, comment, emoji events) and runs an
LLM code review for each qualifying MR as an isolated child process.

Target for this repo's setup: cluster `coabkube-prod`, namespace `mw-internal`,
GitLab `gitlab.mittwald.it`, mittwald internal LLM.

## What it deploys

| Object | Purpose |
| --- | --- |
| Deployment | Single-replica daemon (`Recreate` strategy). Read-only root fs; git clones and per-review logs go to an ephemeral `/work` emptyDir. |
| Service | ClusterIP on port 8080 (`/webhooks/gitlab`, `/healthz`). |
| Ingress | Public webhook endpoint (set `ingress.host`). |
| ConfigMap | `server.yaml` (rendered from `serve.*`); plus `nickpit.yaml` only if `config.nickpitYaml` overrides the binary's built-in LLM profiles. Secrets stay as `${VAR}`. |
| Secret | LLM API key + the group inventory (`groups.yaml` key: paths, access tokens, signing tokens) — unless `existingSecret` is used. |
| ServiceAccount | No RBAC; token not mounted (daemon never calls the k8s API). |

## Prerequisites

- Image `ghcr.io/dgrieser/nickpit:<tag>` reachable from the cluster. It is built
  and pushed by the repo's Docker workflow on `v*` git tags. To build/push manually:
  ```sh
  docker build -t ghcr.io/dgrieser/nickpit:dev .
  docker push ghcr.io/dgrieser/nickpit:dev
  ```
  If the ghcr package is private, create a pull secret and set `imagePullSecrets`.
- A GitLab group access token (scope: api) per group.
- A webhook signing token per group: when adding the group webhook, choose
  "Generate signing token" and copy the `whsec_...` value (GitLab shows it once
  and never returns it via API). The daemon verifies each delivery's
  HMAC-SHA256 signature (headers `webhook-id` / `webhook-timestamp` /
  `webhook-signature`). A legacy plaintext secret token is still supported.
- An LLM API key (default profile uses the mittwald internal endpoint).

## Install

Do not put real secrets in a committed values file. Create your own
`prod-values.yaml` for non-secret config (host) and pass secrets on the
command line or via `existingSecret`.

The group inventory lives in the Secret (key `groups.yaml`, tokens included),
not in chart values: adding or removing a group means editing only the Secret.
Write the inventory to a local `groups.yaml`:

```yaml
groups:
  - path: "mygroup"
    token: "glpat-..."
    signing_token: "whsec_..."
```

Host, ingress class and TLS are baked into `values.yaml`, so install only
needs the secret.

```sh
# 1. (recommended) create the secret out-of-band
kubectl -n mw-internal create secret generic nickpit-serve \
  --from-literal=MITTWALD_LLM_API_KEY=... \
  --from-file=groups.yaml

# 2. install (namespace also comes from your kube-context; shown explicitly)
helm upgrade --install nickpit-serve deploy/helm/nickpit-serve -n mw-internal \
  --set existingSecret=nickpit-serve
```

Or let the chart create the Secret (fine for a quick test):

```sh
helm upgrade --install nickpit-serve deploy/helm/nickpit-serve -n mw-internal \
  --set secrets.MITTWALD_LLM_API_KEY=... \
  --set-file secrets.groups\.yaml=groups.yaml
```

To keep groups in chart values instead (rendered into the ConfigMap with
`${ENV}` token references), set `serve.groupsSecretKey=""` and list
`serve.groups` entries (`path`, `tokenEnv`, `signingTokenEnv` /
`webhookSecretEnv`); the referenced env vars must then exist as Secret keys.

## Key values

| Key | Default | Notes |
| --- | --- | --- |
| `image.repository` / `image.tag` | `ghcr.io/dgrieser/nickpit` / `""`→appVersion | |
| `ingress.enabled` / `ingress.host` | `true` / `nickpit.prod.mittwald.systems` | GitLab must reach it. |
| `serve.groupsSecretKey` | `groups.yaml` | Secret key holding the group inventory, mounted as `/etc/nickpit/groups.yaml`. `""` disables. |
| `serve.groups` | `[]` | Optional inline groups: `path`, `tokenEnv`, `signingTokenEnv` (or legacy `webhookSecretEnv`). |
| `serve.reviewConcurrency` | `2` | Max parallel review child processes. |
| `serve.shutdownGrace` | `10m` | In-flight reviews finish on SIGTERM. |
| `terminationGracePeriodSeconds` | `660` | Must exceed `serve.shutdownGrace`. |
| `existingSecret` | `""` | Reference a pre-made Secret instead of the chart's. |
| `serve.review.extraArgs` | `--profile mittwald` | Args for every review child; selects the built-in LLM profile. |
| `config.nickpitYaml` | `""` | Optional `.nickpit.yaml` override; empty = built-in profiles (recommended). |

## Notes / caveats

- **Do not scale past 1 replica.** State (review queue, dedup LRU) is in-memory.
- **Grace vs. termination.** `terminationGracePeriodSeconds` must stay `>` the
  seconds in `serve.shutdownGrace`, else Kubernetes SIGKILLs mid-review. An
  interrupted publish heals on the next run via comment fingerprints.
- **Group changes need a restart with `existingSecret`.** The daemon reads
  `groups.yaml` once at startup. The chart-managed Secret is covered by a
  checksum annotation (rollout on `helm upgrade`), but edits to an external
  Secret require `kubectl rollout restart deployment/nickpit-serve`.
- **No NetworkPolicy shipped.** The daemon needs egress to GitLab and the LLM
  endpoint; add a policy if the namespace is default-deny.
- **Storage is ephemeral.** `/work` clones and per-review logs vanish on restart.
