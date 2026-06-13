# team-sample-app

The Phase-1 sample application for the Capstone Internal Developer Platform (IDP).
A minimal Go HTTP service (D-004) that proves the golden path end to end:
PR → preview, merge → dev, tag → staging, manual gate → prod, reading a Sealed
Secret (D-006) along the way.

## Repo layout — the `.devops/` contract

```
team-sample-app/
├── app/        ←  YOU EDIT THIS.   Your application code + Dockerfile.
└── .devops/    ←  DO NOT EDIT.     Platform-managed deployment template.
```

You own `app/`. The platform owns `.devops/`. The **only** values you declare are
the four fields in `.devops/app-metadata.yaml`:

```yaml
team: sample
semester: 2026-fall
app-name: sample
port: 8080
```

Everything else — Deployment, Service, Ingress, namespaces, the ingress host,
quotas, RBAC, network policy — is derived from those values by the platform.

## The app (`app/`)

A standard-library-only Go service:

| Route | Behavior |
| --- | --- |
| `GET /healthz` | `200 ok` — liveness/readiness probe. Always succeeds while the process is up, independent of secret state. |
| `GET /` | `200` and prints a **proof** that it read `APP_SECRET` (`secret loaded: <bool>, length=N, sha256=<8 hex>`) **without** echoing the value. |

It reads two env vars at startup: `APP_SECRET` (from the Sealed Secret) and
`PORT` (defaults to `8080`, the `app-metadata.yaml` port).

### Build & test locally

```sh
cd app
go test ./...        # unit tests — must be green
go run .             # serves on :8080 ; try: curl localhost:8080/ and /healthz
```

### Container image

`app/Dockerfile` is a multi-stage build → `scratch` (≈6 MB, non-root, instant
cold start, ADR-004):

```sh
docker build -t sample:local app/
docker run --rm -e APP_SECRET=hello -p 8080:8080 sample:local
curl localhost:8080/        # secret loaded: true, length=5, sha256=...
```

## The golden path

| You do this (git event) | The platform deploys to |
| --- | --- |
| Open a PR | an ephemeral **preview** namespace (`sample-pr-<n>`) |
| Merge to `main` | **dev** (`sample-dev`) — auto-synced |
| Push a tag `vX.Y.Z` | **staging** (`sample-staging`) — auto-synced |
| A human approves the prod sync | **prod** (`sample-prod`) — manual gate |

Each environment is reachable at `sample.sample[.<env>].127.0.0.1.sslip.io`
(dev has no env segment). See `.devops/README.md` for the image-tag seam and the
`kubeseal` workflow for managing `APP_SECRET`.

> Phase 1 runs entirely on local k3d. The PR-preview trigger is a git-branch
> stand-in (D-009); the live GitHub PR generator swaps in when `GITHUB_ORG`
> wiring completes (D-007).
