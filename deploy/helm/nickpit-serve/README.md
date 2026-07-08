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
| ConfigMap | `server.yaml` (rendered from `serve.*`) + `nickpit.yaml` (from `config.nickpitYaml`). Secrets stay as `${VAR}`. |
| Secret | Group tokens, webhook secrets, LLM API key — unless `existingSecret` is used. |
| ServiceAccount | No RBAC; token not mounted (daemon never calls the k8s API). |

## Prerequisites

- Image `ghcr.io/dgrieser/nickpit:<tag>` reachable from the cluster. It is built
  and pushed by the repo's Docker workflow on `v*` git tags. To build/push manually:
  ```sh
  docker build -t ghcr.io/dgrieser/nickpit:dev .
  docker push ghcr.io/dgrieser/nickpit:dev
  ```
  If the ghcr package is private, create a pull secret and set `imagePullSecrets`.
- A GitLab group access token (scope: api) and a webhook secret per group.
- An LLM API key (default profile uses the mittwald internal endpoint).

## Install

Do not put real secrets in a committed values file. Create your own
`prod-values.yaml` for non-secret config (host, groups) and pass secrets on the
command line or via `existingSecret`.

```sh
# 1. (recommended) create the secret out-of-band
kubectl -n mw-internal create secret generic nickpit-serve \
  --from-literal=MITTWALD_LLM_API_KEY=... \
  --from-literal=NICKPIT_GROUP_EXAMPLE_TOKEN=glpat-... \
  --from-literal=NICKPIT_GROUP_EXAMPLE_SECRET=...

# 2. install (namespace already set by your kube-context, shown here explicitly)
helm upgrade --install nickpit-serve deploy/helm/nickpit-serve \
  -n mw-internal \
  --set existingSecret=nickpit-serve \
  --set ingress.host=nickpit.mittwald.it \
  -f prod-values.yaml
```

Or let the chart create the Secret (fine for a quick test):

```sh
helm upgrade --install nickpit-serve deploy/helm/nickpit-serve -n mw-internal \
  --set ingress.host=nickpit.mittwald.it \
  --set secrets.MITTWALD_LLM_API_KEY=... \
  --set secrets.NICKPIT_GROUP_EXAMPLE_TOKEN=glpat-... \
  --set secrets.NICKPIT_GROUP_EXAMPLE_SECRET=...
```

## Key values

| Key | Default | Notes |
| --- | --- | --- |
| `image.repository` / `image.tag` | `ghcr.io/dgrieser/nickpit` / `""`→appVersion | |
| `ingress.enabled` / `ingress.host` | `true` / `nickpit.example.com` | Set the host. GitLab must reach it. |
| `serve.groups` | one example | `path`, `tokenEnv`, `webhookSecretEnv` per group. |
| `serve.reviewConcurrency` | `2` | Max parallel review child processes. |
| `serve.shutdownGrace` | `10m` | In-flight reviews finish on SIGTERM. |
| `terminationGracePeriodSeconds` | `660` | Must exceed `serve.shutdownGrace`. |
| `existingSecret` | `""` | Reference a pre-made Secret instead of the chart's. |
| `config.nickpitYaml` | mittwald profile | LLM provider/model config for review children. |

## Notes / caveats

- **Do not scale past 1 replica.** State (review queue, dedup LRU) is in-memory.
- **Grace vs. termination.** `terminationGracePeriodSeconds` must stay `>` the
  seconds in `serve.shutdownGrace`, else Kubernetes SIGKILLs mid-review. An
  interrupted publish heals on the next run via comment fingerprints.
- **No NetworkPolicy shipped.** The daemon needs egress to GitLab and the LLM
  endpoint; add a policy if the namespace is default-deny.
- **Storage is ephemeral.** `/work` clones and per-review logs vanish on restart.
