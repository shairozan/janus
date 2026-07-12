# Janus License Server — GKE Deployment

Kustomize manifests + SOPS/AGE secrets for running the Janus license server on GKE,
fronted by a Google-managed TLS certificate at **`licenses.januspk.com`**.

> The **management portal** (`app.januspk.com`) is colocated in this same
> `janus-license` namespace — its manifests live under [`portal/`](./portal/README.md)
> and reuse this deploy's namespace, cert-manager Issuers, Cloudflare token,
> ghcr-pull secret, and external-dns. Apply this overlay first, then the portal.

## Architecture

```
            external-dns ──Cloudflare API──> A record licenses.januspk.com → LB IP
            cert-manager ──Cloudflare API (DNS-01)──> TLS secret license-server-tls
                      │
Internet ──HTTPS──> GCE Ingress (TLS from license-server-tls)
                      │  (HTTP→HTTPS redirect via FrontendConfig)
                      ▼
              Service (ClusterIP + NEG, container-native LB)
                      │  health check → /health (BackendConfig)
                      ▼
              license-server Deployment (HTTP :8080, replicas: 1)
                      │  LICENSING_DATABASE_URL
                      ▼
              license-postgres StatefulSet (PVC, in-cluster Postgres 16)
```

- **No static IP.** external-dns publishes `licenses.januspk.com` to Cloudflare pointing
  at whatever address the GCE load balancer is assigned.
- **TLS via cert-manager** using Let's Encrypt ACME **DNS-01** (Cloudflare) — issuance
  needs no inbound reachability. The cert lands in the `license-server-tls` Secret that
  the GCE Ingress consumes.
- The server speaks **plain HTTP**; TLS terminates at the load balancer.
- DB **migrations run automatically** on startup; signing **private keys are stored
  encrypted** in Postgres using `LICENSING_ENCRYPTION_KEY` — protect that key and the
  Postgres PVC accordingly.
- `replicas: 1` keeps migrations and key rotation serialized.

> Cluster-wide prerequisites (cert-manager + external-dns) are installed separately —
> see [`platform/README.md`](./platform/README.md).

## Layout

```
deploy/
  .sops.yaml                         # SOPS rules (AGE recipient) for secrets/
  Makefile                           # render / apply / secrets helpers
  platform/                          # cluster-wide prereqs (cert-manager, external-dns)
    README.md
    external-dns/                    # Helm values + Cloudflare token template
  license-server/
    base/                            # namespace, configmap, deployment, service,
                                     #   backendconfig, issuer, certificate, ingress, postgres
    overlays/gke/                    # image (registry/tag) + env tweaks
    secrets/
      license-secrets.example.yaml         # template (committed)
      cloudflare-api-token.example.yaml    # template (committed)
      ghcr-pull.example.yaml               # template (committed)
      *.sops.yaml                          # SOPS-encrypted Secrets (you create these)
```

## Prerequisites

- `gcloud`, `kubectl`, `sops`, `age`, `helm`.
- **`kustomize`** (standalone — not `kubectl kustomize`, which can't run exec plugins).
- **`ksops`** exec plugin on `PATH` — decrypts the committed SOPS secrets during
  `kustomize build`. Install:
  ```bash
  go install github.com/viaduct-ai/kustomize-sops@latest   # installs the `ksops` binary
  ```
- A GKE cluster with the **HTTP(S) Load Balancing** add-on (default) so `Ingress`,
  `BackendConfig`, and `FrontendConfig` work.
- **cert-manager** and **external-dns** installed cluster-wide — see
  [`platform/README.md`](./platform/README.md).
- `kubectl` context pointed at the target cluster; AGE private key available to sops
  (`SOPS_AGE_KEY_FILE` or `~/.config/sops/age/keys.txt`).

## One-time setup

### 1. AGE key for SOPS

```bash
age-keygen -o ~/.config/sops/age/keys.txt      # prints "Public key: age1..."
```

Put the **public** key in `deploy/.sops.yaml` (replace the `age1PLACEHOLDER…` line).
SOPS finds the private key automatically via `SOPS_AGE_KEY_FILE` or the default path
`~/.config/sops/age/keys.txt`. Share the public key with any other operators who need
to encrypt; share the private key only with those who must decrypt/deploy.

### 2. Create the encrypted secrets

Two SOPS-encrypted Secrets live in `license-server/secrets/`:

```bash
cd deploy

# (a) App secret: DB DSN, 32-byte encryption key, Postgres password.
cp license-server/secrets/license-secrets.example.yaml \
   license-server/secrets/license-secrets.sops.yaml
$EDITOR license-server/secrets/license-secrets.sops.yaml   # fill real values:
#   LICENSING_ENCRYPTION_KEY : exactly 32 bytes  (e.g. openssl rand -base64 24)
#   POSTGRES_PASSWORD        : e.g. openssl rand -base64 24
#   LICENSING_DATABASE_URL   : same password as above
sops --encrypt --in-place license-server/secrets/license-secrets.sops.yaml

# (b) Cloudflare API token used by cert-manager's DNS-01 solver (janus-license ns).
cp license-server/secrets/cloudflare-api-token.example.yaml \
   license-server/secrets/cloudflare-api-token.sops.yaml
$EDITOR license-server/secrets/cloudflare-api-token.sops.yaml
sops --encrypt --in-place license-server/secrets/cloudflare-api-token.sops.yaml

# (c) ghcr.io image pull secret (janus-license ns). Skip if the package is public.
kubectl create secret docker-registry ghcr-pull \
  --namespace janus-license \
  --docker-server=ghcr.io \
  --docker-username=<github-username> \
  --docker-password=<github-PAT-read:packages> \
  --dry-run=client -o yaml \
  > license-server/secrets/ghcr-pull.sops.yaml
sops --encrypt --in-place license-server/secrets/ghcr-pull.sops.yaml
```

`make secrets-view` decrypts all of them to stdout; `make secrets-edit F=<file>` edits
one in place. (external-dns uses a copy of the same Cloudflare token in its own
namespace — see [`platform/README.md`](./platform/README.md).)

> Pull secrets are **namespace-scoped** — an existing `ghcr-pull` secret in another
> namespace cannot be reused; it must exist in `janus-license`. If the image is public,
> drop `imagePullSecrets` from `base/deployment.yaml` and skip step 2(c).

### 3. Build and push the image

The image is built from `docker/license/Dockerfile` (private Go modules need a token)
and pushed to ghcr.io:

```bash
echo "$GITHUB_PAT" | docker login ghcr.io -u <github-username> --password-stdin
docker build -f docker/license/Dockerfile \
  --build-arg GITHUB_TOKEN=$GITHUB_TOKEN \
  -t ghcr.io/pharmalytica/janus-license-server:<tag> .
docker push ghcr.io/pharmalytica/janus-license-server:<tag>
```

Set the owner/name/tag in `license-server/overlays/gke/kustomization.yaml` (`images:`).

### 4. DNS & TLS — handled automatically

No static IP and no manual A record. Once deployed:

- **external-dns** creates/updates the `licenses.januspk.com` record in Cloudflare,
  pointing at the GCE load balancer's assigned address.
- **cert-manager** solves the ACME DNS-01 challenge via the Cloudflare API and writes
  the cert into the `license-server-tls` Secret, which the Ingress consumes.

You only need the Cloudflare API token from step 2(b) and the cluster-wide components
from [`platform/README.md`](./platform/README.md).

## Deploy

Secrets are committed encrypted and decrypted by KSOPS during the build, so the whole
stack deploys in one pipeline:

```bash
cd deploy
make build                        # = kustomize build --enable-alpha-plugins --enable-exec <overlay>
make apply                        # = kustomize build ... | kubectl apply -f -
```

Equivalently, without make:

```bash
kustomize build --enable-alpha-plugins --enable-exec license-server/overlays/gke \
  | kubectl apply -f -
```

(`make build` prints decrypted secret material to stdout — it's the full manifest set.)

### Two components: license-server (API) + portal (UI)

Both live in the `janus-license` namespace but ship as separate overlays, each
pinning its own image. Deploy them independently by pointing `OVERLAY` at each:

```bash
cd deploy
make apply OVERLAY=license-server/overlays/gke   # the Go API   (licenses.januspk.com)
make apply OVERLAY=portal/overlays/gke           # the Next.js UI (app.januspk.com)
```

### Pinning a released version (`VERSION=`)

The overlays track `:latest` in git. To deploy a specific release instead — e.g. the
images CI publishes from git tag **`v0.2.4`** (published as image tag **`0.2.4`**;
`docker/metadata-action` strips the leading `v`) — pass `VERSION` at apply time:

```bash
make apply OVERLAY=license-server/overlays/gke VERSION=0.2.4
make apply OVERLAY=portal/overlays/gke         VERSION=0.2.4
```

`VERSION` retags only for that run — the overlay's `kustomization.yaml` is backed up,
retagged with `kustomize edit`, and restored byte-for-byte on exit, so git stays clean
and nothing is committed. The image name is derived from `OVERLAY` (each overlay pins
exactly one image), so `VERSION` targets that component alone — you can pin the API and
UI to different tags. Because a pinned run restores the overlay from a backup, commit
any manual overlay edits before using `VERSION`. Preview first with
`make diff OVERLAY=... VERSION=...`.

Watch rollout, DNS, and certificate issuance (first issuance is usually a few minutes):

```bash
kubectl -n janus-license rollout status deploy/license-server
kubectl -n janus-license get certificate license-server-tls -w     # want READY=True
kubectl -n janus-license describe certificate license-server-tls   # troubleshoot DNS-01 here
kubectl -n janus-license get ingress license-server                # shows the LB address
kubectl -n external-dns logs deploy/external-dns | grep -i januspk  # record published?
```

Tip: point the Certificate's `issuerRef` at `letsencrypt-staging` while validating to
avoid Let's Encrypt rate limits, then switch back to `letsencrypt-prod`.

Once `https://licenses.januspk.com/health` returns 200, the server is live.

## Bootstrap signing keys (the "start over from a key perspective" step)

The server has no signing key until you create one. Against the deployed server:

```bash
BASE=https://licenses.januspk.com

# Generate a master signing key (valid 365 days)
curl -fsSL -X POST "$BASE/api/v1/keys/rotate" \
  -H 'Content-Type: application/json' -d '{"expires_in_days": 365}'

# List keys / fetch a public key by id
curl -fsSL "$BASE/api/v1/keys"
curl -fsSL "$BASE/api/v1/keys/public?key_id=<KEY_ID>"
```

To embed the public key(s) into Janus builds, export them into the build's
`.license_public_key.pem` — the mage target does this (it concatenates **all**
non-revoked master keys, newest first):

```bash
# from repo root, with .env pointing LICENSING_HOST/PORT at the server
mage license:exportPublicKey
```

For CI release builds, place the same concatenated PEMs in the `LICENSE_PUBLIC_KEYS`
GitHub secret (see the multi-key embedding work in `internal/license/publickey`).

## Notes & operations

- **Backups:** the Postgres PVC holds the encrypted signing keys. Snapshot the PD (or
  add `pg_dump` to a CronJob) before any key rotation or cluster maintenance.
- **`make delete`** removes the workload but intentionally leaves the namespace and the
  Postgres PVC so data survives. Delete the PVC explicitly to wipe key state.
- **OIDC** is optional: set `LICENSING_OIDC_ISSUER` / `LICENSING_OIDC_CLIENT_ID` (and
  adjust `LICENSING_OIDC_ALLOWED_DOMAINS`) in the ConfigMap to enable it.
- **Rotation of `LICENSING_ENCRYPTION_KEY`** re-encrypts stored private keys — do not
  change it casually; losing it makes stored private keys unrecoverable.



helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set crds.enabled=true