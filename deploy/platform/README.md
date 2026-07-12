# Platform prerequisites (cluster-wide)

The license-server deployment expects two cluster-wide components. Install them once
per cluster; they are shared infrastructure, not part of the app's Kustomize base.

## cert-manager

Issues the TLS certificate via Let's Encrypt ACME **DNS-01** (Cloudflare). The
namespaced `Issuer`/`Certificate` live with the app (`license-server/base/`); only the
controller + CRDs are installed here.

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager --create-namespace \
  --set crds.enabled=true \
  --set global.leaderElection.namespace=cert-manager   # REQUIRED on GKE Autopilot
```

> **GKE Autopilot:** cert-manager defaults its leader-election lease to `kube-system`,
> which Autopilot blocks (`GKE Warden ... managed-namespaces-limitation`). Without the
> lease, `cainjector` never injects the webhook CA bundle, the webhook stays untrusted
> (`x509: certificate signed by unknown authority`), and `startupapicheck` crashloops.
> `--set global.leaderElection.namespace=cert-manager` moves the lease into cert-manager's
> own namespace and fixes it. If you installed without the flag, run the same command
> with `helm upgrade` to repair in place.

The Cloudflare token cert-manager uses lives in the **janus-license** namespace
(`deploy/license-server/secrets/cloudflare-api-token.*`).

## external-dns

Publishes `licenses.januspk.com` to Cloudflare, pointing at whatever address the GCE
load balancer is assigned — this is what lets us **avoid a reserved static IP**.

```bash
# Create the namespace and its Cloudflare token secret (SOPS-encrypted).
kubectl create namespace external-dns
cp external-dns/cloudflare-token.example.yaml external-dns/cloudflare-token.sops.yaml
# edit the token + namespace, then:
sops --encrypt --in-place external-dns/cloudflare-token.sops.yaml
sops --decrypt external-dns/cloudflare-token.sops.yaml | kubectl apply -f -

# Install external-dns with the provided values.
helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
helm repo update
helm install external-dns external-dns/external-dns \
  --namespace external-dns \
  -f external-dns/values.yaml
```

Verify it manages the zone:

```bash
kubectl -n external-dns logs deploy/external-dns | grep -i januspk
```

## Cloudflare API token

Both components use a token scoped to the `januspk.com` zone:

- **Zone → Zone → Read**
- **Zone → DNS → Edit**

Create it once in the Cloudflare dashboard; store one copy in each namespace
(`janus-license` for cert-manager, `external-dns` for external-dns) as SOPS-encrypted
Secrets.

## Order of operations

1. Install cert-manager and external-dns (this README).
2. Create both Cloudflare token secrets.
3. Deploy the app (`deploy/README.md` → `make apply`).
4. external-dns creates the DNS record; cert-manager solves DNS-01 and writes the
   `license-server-tls` Secret; the GCE Ingress picks it up. First issuance can take a
   few minutes.
