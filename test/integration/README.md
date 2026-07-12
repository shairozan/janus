# Integration E2E (non-hermetic) — #150

The **real** portal + **real** license-server + **real** Postgres, with only the
true externals faked (a mock OIDC issuer). This catches the integration *joints*
the hermetic Playwright suite (`web/portal/e2e/`) mocks away: token validation,
`ResolveOrgUser`, JSON shapes (nil-slice→`null`), and route registration.

## Run it

```bash
# from the repo root — no credentials needed; the license-server builds scoped
# from source (test/integration/Dockerfile.license-server, no private hermes dep).
# The runner builds + starts the stack, waits on the playwright container, prints
# its logs, exits with its code, and tears the stack down.
bash test/integration/run.sh
```

> The runner uses `up -d` + `docker wait playwright` rather than
> `--abort-on-container-exit`, because the one-shot `oidc-ready`/`seed` jobs exit
> early and would otherwise abort the stack before the test runs.

The `playwright` service's exit code is the suite result.

## What's in the stack

| service | role |
| --- | --- |
| `postgres` | real DB; the license-server auto-migrates on boot |
| `oidc` | [navikt/mock-oauth2-server](https://github.com/navikt/mock-oauth2-server) — discovery + JWKS + authorize/token/refresh, stamping `token_use: access` (`test/integration/oidc/config.json`) |
| `license-server` | real Go binary (`docker/license/Dockerfile`); Stripe/Cognito-admin/Mailgun/Redis left unset (those routes self-disable) |
| `seed` | inserts the org + customer-admin user once migrations exist (`seed.sql`) |
| `portal` | real Next standalone (`docker/portal/Dockerfile`); BFF → license-server, Auth.js → `oidc` |
| `playwright` | drives the real portal UI (`specs/`, `playwright.config.ts`) |

## Scope boundary

This is the **integration** tier — real in the middle, faked only at the true
externals. It does **not** reproduce Cognito-specific behavior (the non-compliant
`nonce`, hosted-UI domain, PKCE-metadata gap); those stay a manual check against
real Cognito. It complements — does not replace — the fast hermetic suite.
