# Releasing

Tagging `v*` runs `.github/workflows/release.yml`, which builds for six targets,
signs and notarizes the macOS packages, and publishes a GitHub release.

```bash
git tag v0.3.0 && git push origin v0.3.0
```

## Required secrets

The release is designed to degrade rather than fail. With none of these
configured it still builds and publishes — macOS artifacts are simply unsigned,
and the job log carries a warning saying so. Add them to make signing happen.

| Secret | Purpose |
|---|---|
| `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN` | pull the `janus-ci` build images |
| `APPLE_CERTIFICATE_P12` | base64 of the Developer ID **Application** `.p12` |
| `APPLE_INSTALLER_P12` | base64 of the Developer ID **Installer** `.p12` |
| `APPLE_CERTIFICATE_PASSWORD` | the `.p12` export password (both use one) |
| `APPLE_APPLICATION_CERT_NAME` | e.g. `Developer ID Application: Your Name (TEAMID)` |
| `APPLE_CERTIFICATE_NAME` | e.g. `Developer ID Installer: Your Name (TEAMID)` |
| `APPLE_TEAM_ID` | the 10-character team identifier |
| `APPLE_API_KEY_ID`, `APPLE_API_ISSUER_ID`, `APPLE_API_KEY_P8` | App Store Connect key for notarization |

Signing and notarization are gated separately. Certificates without an API key
produce a **signed but un-notarized** package, and the log says so — worth knowing,
because Gatekeeper refuses un-notarized Developer ID software on macOS 10.14.5 and
later, so that state is only useful for testing the signing half.

An App Store Connect API key is preferred over an app-specific password because it
does not break when the Apple ID password changes. Create it under
**Users and Access → Integrations → App Store Connect API**; the `.p8` downloads
once and cannot be retrieved again.

## Producing the certificates and `.p12` files

Apple issues a `.cer` — the certificate alone. The private key is generated
locally when you create the CSR and never leaves your machine, which is why Apple
cannot give you a `.p12`: only you can assemble one.

You do not need a Mac for any of this. A CSR is a standard PKCS#10, so OpenSSL is
enough.

**Generate a key and CSR per certificate.** Apple rejects a CSR whose key has
already been used, so the Application and Installer certificates need separate
keys:

```bash
openssl genrsa -out application.key 2048
openssl req -new -key application.key -out application.csr \
  -subj "/emailAddress=you@example.com/CN=Your Name/C=US"
```

Repeat with `installer.key`/`installer.csr`. On Git Bash prefix the `req` command
with `MSYS_NO_PATHCONV=1`, or the leading `/` is rewritten into a Windows path.

**Request the certificates** at developer.apple.com → Certificates → **+**,
choosing *Developer ID Application* and *Developer ID Installer*. When asked which
sub-CA to use, pick **G2**: the original Developer ID CA expires on 1 February
2027 and Apple caps a leaf certificate at its issuer's expiry, so a non-G2
certificate issued today is short-lived regardless of what the portal implies. G2
runs to September 2031.

**Assemble the `.p12`s**, chaining in the intermediate that actually issued them:

```bash
curl -sO https://www.apple.com/certificateauthority/DeveloperIDG2CA.cer
openssl x509 -inform DER -in DeveloperIDG2CA.cer -out DeveloperIDG2CA.pem
openssl x509 -inform DER -in application.cer -out application.pem
openssl pkcs12 -export -legacy -out DeveloperIDApplication.p12 \
  -inkey application.key -in application.pem -certfile DeveloperIDG2CA.pem
```

`-legacy` matters: OpenSSL 3 otherwise uses encryption that macOS `security
import` will not read. `-certfile` matters too — without it the bundle carries
only the leaf, and `codesign` fails on the runner with an incomplete-chain error
that is painful to diagnose from CI logs.

Then base64 each one into its secret:

```bash
base64 -w0 DeveloperIDApplication.p12
```

## Verify before trusting a bundle

Two mistakes are easy to make and both survive until the runner rejects them.

**The certificate must match the key you paired it with.** With separate keys per
certificate, crossing them is easy — and regenerating a key file overwrites the
one that produced an earlier CSR, orphaning that certificate silently:

```bash
openssl x509 -inform DER -in application.cer -noout -modulus | openssl md5
openssl rsa -in application.key -noout -modulus | openssl md5
```

Identical output means they pair. Different output means that certificate cannot
sign anything, and the only fix is to reissue it.

**The bundle must contain the full chain** — expect two certificates, your leaf
plus the G2 intermediate:

```bash
openssl pkcs12 -in DeveloperIDApplication.p12 -nokeys -legacy | grep -E "^subject=|^issuer="
```

## Keeping the keys

The `.key` files are the only irreplaceable part. A certificate without its key is
inert, and Developer ID certificates **cannot be revoked from the portal** — that
requires contacting Apple Developer Support — so a lost key leaves a permanent
orphan in your certificate list and costs you a reissue.

Store `application.key` and `installer.key` in a password manager as soon as they
exist. `.gitignore` covers `*.key`, `*.p12` and `*.cer`, but the reliable habit is
to keep them outside the repository entirely.

## Expiry

Certificates expire. Check with:

```bash
openssl x509 -inform DER -in application.cer -noout -dates
```

A G2 certificate lasts five years. Renewing means a new CSR and new `.p12`s — the
same steps as above — and the workflow needs no change, only updated secrets.
