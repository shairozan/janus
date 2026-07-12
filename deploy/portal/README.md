# Janus Management Portal — GKE Deployment

Kustomize manifests + a SOPS/AGE secret for running the Next.js management portal on
GKE, fronted by a cert-manager TLS certificate at **`app.januspk.com`**.

The portal is **colocated with the license-server in the `janus-license` namespace** —
that namespace holds both the backend and the UI. It reuses what the license-server
deploy already provides there, so this package adds only the portal's own resources.

## Architecture

```
            external-dns ──Cloudflare API──> A record app.januspk.com → LB IP
            cert-manager ──Cloudflare API (DNS-01)──> TLS secret portal-tls
                      │
Internet ──HTTPS──> GCE Ingress (TLS from portal-tls)
                      │  (HTTP→HTTPS redirect via FrontendConfig)
                      ▼
              Service (ClusterIP + NEG, container-native LB)
                      │  health check → /api/health (BackendConfig)
                      ▼
              portal Deployment (Next standalone, HTTP :3000, replicas: 2)
                      │  LICENSE_SERVER_URL (BFF → license-server)
                      ▼
              license-server  (same namespace — Service "license-server:80")
```

### Reused from the license-server deploy (not recreated here)

The license-server deploy must be applied first; the portal depends on these existing
in `janus-license`:

- the **namespace** itself,
- the cert-manager **Issuers** `letsencrypt-prod` / `letsencrypt-staging`,
- the **`cloudflare-api-token`** Secret (cert-manager's DNS-01 solver),
- the **`ghcr-pull`** image-pull Secret,
- the cluster-wide **external-dns** instance (`domain-filter=januspk.com`), which
  publishes `app.januspk.com` automatically.

### Portal-specific (in this package)

`portal` Deployment + Service, `portal-config` ConfigMap, `portal-backendconfig`,
`portal` Ingress + `portal-frontendconfig`, the `portal-tls` Certificate (issued by
the shared `letsencrypt-prod` Issuer), and the `portal-secrets` Secret.

- **Stateless** SSR app → `replicas: 2` with a zero-downtime rolling update.
- Reaches the license-server in-cluster as `license-server.janus-license.svc.cluster.local`.
- TLS terminates at the LB; `AUTH_TRUST_HOST=true` lets Auth.js trust the LB's
  `X-Forwarded-*` headers, and `AUTH_URL` must equal the public origin so OAuth
  redirects are correct.

## Layout

```
deploy/portal/
  base/                            # configmap, deployment, service, backendconfig,
                                   #   certificate, ingress  (namespace: janus-license)
  overlays/gke/                    # image (registry/tag) + env (config patch) + KSOPS
  secrets/
    portal-secrets.example.yaml          # template (committed) — AUTH_SECRET + Cognito client secret
    portal-secrets.sops.yaml             # SOPS-encrypted Secret (you create this)
```

## Configuration

Non-secret values live in the ConfigMap; environment-specific ones are set in
`overlays/gke/portal-config-patch.yaml` — **fill these in before deploying**:

| Key | Meaning |
| --- | --- |
| `AUTH_URL` | Public origin, `https://app.januspk.com` (must match the Ingress host). |
| `AUTH_COGNITO_ID` | The portal's Cognito **app client id** (non-secret). |
| `AUTH_COGNITO_ISSUER` | The Cognito user-pool issuer URL. |
| `LICENSE_SERVER_URL` | In-cluster license-server, `http://license-server.janus-license.svc.cluster.local`. |

The only secret to create is `portal-secrets.sops.yaml`: `AUTH_SECRET` (Auth.js
session key, `openssl rand -base64 32`) and `AUTH_COGNITO_SECRET` (the confidential
client secret).

## Deploy

Apply the **license-server** overlay first (it creates the namespace and the shared
Issuers/token/pull-secret), then the portal. The shared
[`../Makefile`](../Makefile) drives both via the `OVERLAY` variable:

```bash
cd deploy

# 1. Create the portal Secret from the template, fill values, and SOPS-encrypt:
cp portal/secrets/portal-secrets.example.yaml portal/secrets/portal-secrets.sops.yaml
# ...edit with real values...
sops -e -i portal/secrets/portal-secrets.sops.yaml

# 2. Render / diff / apply (KSOPS decrypts the secret inside the build):
make build OVERLAY=portal/overlays/gke
make diff  OVERLAY=portal/overlays/gke
make apply OVERLAY=portal/overlays/gke

# Inspect/edit the encrypted secret:
make secrets-view SECRET_DIR=portal/secrets
make secrets-edit F=portal/secrets/portal-secrets.sops.yaml
```

The image (`ghcr.io/pharmalytica/janus/portal`) is built and published by the
[`portal-image.yml`](../../.github/workflows/portal-image.yml) workflow.
