# Phase-1 local CI loop — runbook

The local golden-path inner loop: **edit app → build → push → bump → ArgoCD syncs.**
Everything is driven by [`../promotion.yaml`](../promotion.yaml) (the single
configured place, §4.1). Phase 2 replaces the local build/push with GitHub
Actions + Harbor but keeps this exact seam — only `registry` and the trigger change.

## Prerequisites

- The k3d cluster is up with the built-in registry (`make cluster-up` in
  `platform-infra`), and `k3d-registry.localhost` resolves to `127.0.0.1` on the
  host (cluster-up adds the `/etc/hosts` entry; otherwise:
  `echo '127.0.0.1 k3d-registry.localhost' | sudo tee -a /etc/hosts`).
- ArgoCD is installed and the `team-sample` env Applications exist (T3/T7).
- `docker`, `git`, `yq`, `go` on PATH.

## The loop

From the `team-sample-app/` repo root:

```sh
# 1. EDIT — change app code
$EDITOR app/main.go
make test                       # keep tests green (required, no exceptions)

# 2. BUILD + PUSH — image tag computed from promotion.yaml for the env
make app-build ENV=dev          # -> k3d-registry.localhost:5000/sample:<short-sha>
                                # prints  IMAGE=...  TAG=<sha>  ENV=dev

# 3. BUMP — write the new tag into the dev overlay and commit (the GitOps signal)
make bump ENV=dev TAG=<sha> COMMIT=1
#   (or do build+push+bump+commit in one shot:)
make deploy ENV=dev

# 4. ArgoCD SYNCS — the dev Application sees the changed overlay and reconciles.
#    Watch it:
argocd app get sample-dev          # or the ArgoCD UI
kubectl -n sample-dev rollout status deploy/sample
```

## Per-environment promotion (from promotion.yaml)

Field names below match `promotion.yaml` (schema `apiVersion: platform.capstone/v1`).

| Env | trigger | tagConvention | resulting tag | gate |
| --- | --- | --- | --- | --- |
| preview | `pull_request` | `pull-<sha>` | `pull-<short-sha>` | auto |
| dev | `branch:main` | `sha-<short>` | `<short-sha>` | auto |
| staging | `tag:v*` | `semver` | `<X.Y.Z>` | auto |
| prod | `tag:v*` | `semver` | `<X.Y.Z>` | **manual** |

Examples:

```sh
SEMVER=1.4.0 make app-build ENV=staging      # build+push sample:1.4.0
make bump ENV=staging TAG=1.4.0 COMMIT=1     # staging auto-syncs

SEMVER=1.4.0 make app-build ENV=prod
make bump ENV=prod TAG=1.4.0 COMMIT=1        # prod overlay updated, but...
# ...prod has NO automated sync — a human approves the sync in ArgoCD (the gate, §4).
```

## How the seam works (for reviewers)

- `build-and-push.sh <env>` reads `promotion.yaml`, resolves the env's
  `tagConvention` (`sha-<short>`/`semver`/`pull-<sha>`), builds `app/`, pushes to
  the registry, prints `IMAGE=`/`TAG=`.
- `bump-image.sh <env> <tag>` reads `promotion.yaml` for the env→overlay mapping
  and rewrites that overlay's `images[].newTag` (and keeps `newName` aligned to
  the registry). With `COMMIT=1` it commits the change — **that commit is the
  signal ArgoCD watches.** No imperative `kubectl apply`; GitOps owns the cluster.
- To change a convention (e.g. "staging tracks a release branch, not a tag"),
  edit the one entry in `promotion.yaml`. The scripts and overlays follow.
