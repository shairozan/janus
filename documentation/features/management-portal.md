# Management Portal
This is essentially a UI (Next.js) that will sit on top of the license-server APIs. 

The goal is twofold:

1. Provide a UI management mechanism for managing the JWKS and rotating keys (Janus administrators)
2. Provide a mechanism for having organizations sign up
   3. After signup they can accept agreements
   4. Add payment details (all credit card)
   5. From here they can _add SSO configurations_.
      6. In cognito the first user that's invited has a group assigned to it called `customer_admin`
      7. The group should be idempotently created _if not present in the pool_
      8. Users that come in from the customer's SSO source _will not have this group_
   9. Users that are not admins (coming from SSO) can login and request a license
   10. A customer admin can define rules for _auto acceptance_ such as by domain or min threshold licenses
       11. All failures in auto application rules means the customer admin must be notified
   12. Once they have a rule they can upload their public key and _download their JWT license.
   13. Customer admins can also define _other customer admins from SSO_ ocne they join
   14. All customer admins are notified on auto allocation failures


This provides a self managed ecosystem from signup -> purchasing -> license assignemnt etc

# SSO Setup and Configuration
This means that organizations will need to be able to define the bare minimum OIDC requirements
necessary to do SSO setup such that _the api_ can update the pool with a new OIDC provider. 
The portal should _also_ provide customer admins with the URL for login (that has variables encoded to go straight to SSO login)

The goal there is to bypass even _seeing cognito and selecting a provider_. 

---

# Public Key Binding Is Per-User

A core constraint that shapes the entire data model: **the signing public key is bound to an
individual user, not to an organization or an agreement.**

## Why per-user

Each end user signs their Janus run logs locally with their _own_ RSA private key. Janus verifies
those signatures using the matching public key, which is embedded in that user's license JWT. Two
users on the same agreement hold different private keys, so they must receive different licenses
carrying different public keys. The binding granularity is therefore **per-user (per-seat)**, and
both customer admins and SSO end users follow the same "upload your public key → receive your
license" flow. The only difference is _who approves the request_.

## How the binding works today (verified in code)

The cryptographic chain already exists and is reusable as-is. Only the persistence/identity layer
around it is missing.

1. **Entry** — a PEM-encoded RSA public key arrives as `signing_public_key` in the
   `POST /api/v1/tokens` request body (`TokenRequest.SigningPublicKey`,
   `internal/license/server/handlers/tokens.go`).
2. **Binding** — it is embedded into the license JWT as the `signing_public_key` claim
   (`internal/license/jwt/claims.go`, `Claims.SigningPublicKey` / `NewClaims`). The JWT is signed
   by Janus's own key, so the user's public key is tamper-proof once issued.
3. **Verification** — at runtime, `internal/runlog/signer.go` (`VerifyRecord`) verifies run-log
   signatures with the embedded public key, and `internal/license/validator/validator.go`
   (`ValidateSigningKeyPair`) asserts _at startup_ that the user's local private key matches the
   public key in their license. A mismatch is a hard error.

## The gap the portal closes

Today this is **stateless** — the key is read from the request body at generation time and never
persisted, and there is **no end-user model** (the system knows Organizations, Agreements,
Licenses, and IssuedTokens, where `IssuedToken.UserEmail` is just a string). The portal adds the
identity + persistence layer beneath the existing crypto: `user → seat → public key → approval →
issued license`, keyed on the Cognito `sub`.

---

# Data Model

New entities follow existing conventions in `internal/license/models/`: `int64` autoincrement PKs,
`time.Time` timestamps, and nullable `*time.Time` (`DeactivatedAt` / `RevokedAt`) for soft
deactivation rather than hard deletes.

> **Naming convention.** The tables below list **Go struct field names** (PascalCase). The
> corresponding **database columns are snake_case** — GORM's default naming strategy maps
> `OrganizationID` → `organization_id`, `CognitoSub` → `cognito_sub`, etc., matching the
> `json`/`db` tags on the existing models. `OrgUser` is shown as a full struct below as the
> reference for how every entity here is tagged; the remaining entities are given as field tables
> for brevity but follow the identical convention.

## OrgUser — the missing identity layer

Represents a person within an organization, keyed on their stable Cognito identity. This is the
anchor everything else hangs off of. Shown in full as the tagging reference; DB columns are the
snake_case `db`/`json` tag values.

```go
// OrgUser represents a person within a customer organization, keyed on their
// stable Cognito identity (the `sub` claim).
type OrgUser struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	OrganizationID int64      `gorm:"not null;index:idx_org_users_org" json:"organization_id" db:"organization_id"`
	CognitoSub     string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"cognito_sub" db:"cognito_sub"`
	Email          string     `gorm:"type:varchar(255);not null" json:"email" db:"email"`
	Role           string     `gorm:"type:varchar(20);not null" json:"role" db:"role"`     // customer_admin | member
	Source         string     `gorm:"type:varchar(20);not null" json:"source" db:"source"` // invited | sso
	CreatedAt      time.Time  `gorm:"not null;default:now()" json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `gorm:"not null;default:now()" json:"updated_at" db:"updated_at"`
	DeactivatedAt  *time.Time `gorm:"default:null;index:idx_org_users_active" json:"deactivated_at,omitempty" db:"deactivated_at"`

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}
```

Field notes: `CognitoSub` is the `sub` claim from the validated access token; `Email` comes from
the UserInfo the auth middleware already fetches; `Source` is `invited` (first admin, seeded into
the pool) or `sso` (federated). `Role` is the privilege boundary. It is _derived from_ the `cognito:groups` claim — `customer_admin`
group membership ⇒ admin. The DB column is a denormalized cache for queries/listing; Cognito remains
the source of truth (see Authorization Model below).

## UserPublicKey — exactly one active key per user

A user has **exactly one active public key at a time** (GitHub-_style upload UX_, but **not** a
GitHub-style multi-key list). The active key is the one embedded in newly issued licenses. Prior
keys are retained as **revoked/archived** rows so that run logs signed under an earlier license can
still be verified against the public key they were issued with — modeled separately from `OrgUser`
so a key change never mutates identity and history stays auditable.

Enforce "one active key" with a **partial unique index** on `OrgUserID` where `RevokedAt IS NULL`
(Postgres partial unique index). Uploading a new key atomically revokes the current active one.

| Field | Type | Notes |
|---|---|---|
| `ID` | `int64` | PK |
| `OrgUserID` | `int64` | FK → OrgUser. Partial-unique where `RevokedAt IS NULL` → at most one active key. |
| `PublicKeyPEM` | `string` (text) | The uploaded PEM. Validated as an RSA `PUBLIC KEY` block via `internal/signing.LoadPublicKeyFromPEM`. |
| `Fingerprint` | `string` | SHA-256 of the DER bytes, displayed `SHA256:…`. Used to dedupe and to display the active key. |
| `Title` | `string` | Optional user-supplied label (e.g. "work laptop"). |
| `CreatedAt` | `time.Time` | |
| `RevokedAt` | `*time.Time` | Soft-revocation. A revoked key is never embedded in a _new_ license but remains for verifying historical run logs. |

> **⚠️ Changing the active key is a significant operation, not a routine one.**
> Replacing the active key issues a **new license** with the new public key. Run logs signed with
> the **previous** private key will no longer verify against the user's current license — it
> functionally invalidates the continuity from previous work to current work. The operation is
> still allowed, but the portal must surface a clear caveat at upload/replace time:
>
> _"Replacing your public key issues you a new license and revokes your previous one. You'll need to
> install the new license file **and** update your local Janus configuration
> (`signing.private_key_path`) to point at the matching private key. Run logs signed with your old
> key will not verify under your new license."_

### Replacement flow

On `PUT /api/v1/me/key`, the server, in one transaction:

1. Validates the new PEM.
2. Atomically revokes the prior active `UserPublicKey` (sets `RevokedAt`), upholding the
   one-active-key invariant.
3. **Revokes the prior `IssuedToken`** (sets `RevokedAt`) — this happens at the point of request,
   treating a key change as "getting a new license."
4. Issues a new license JWT embedding the new public key.

> **Revocation here is for clarity and bookkeeping, not enforcement.** Janus deliberately does
> **not phone home** to check revocation, so the old license JWT remains cryptographically valid and
> _can still be used_. Marking the old `IssuedToken` revoked keeps the server-side record honest
> about which license is current; it does not — and is not intended to — disable the old token at
> runtime.

## LicenseRequest — the approval workflow

Tracks a user's request for a seat against an agreement and its disposition.

| Field | Type | Notes |
|---|---|---|
| `ID` | `int64` | PK |
| `OrganizationID` | `int64` | FK → Organization |
| `OrgUserID` | `int64` | FK → OrgUser (requester) |
| `AgreementID` | `int64` | FK → Agreement (which tier/seat pool) |
| `UserPublicKeyID` | `*int64` | The key to embed. Nullable until the user has uploaded one. |
| `Status` | `string` | `pending`, `approved`, `rejected`, `failed`. |
| `Decision` | `string` | `auto` or `manual`. |
| `DecidedByUserID` | `*int64` | FK → OrgUser, set for manual decisions. |
| `DecidedAt` | `*time.Time` | |
| `Reason` | `*string` | Rejection/failure detail (drives the admin notification). |
| `IssuedTokenID` | `*int64` | FK → IssuedToken, set once the license JWT is minted. |
| `CreatedAt` | `time.Time` | |

A request can only be `approved` once it has both a decision _and_ a `UserPublicKeyID`. Minting the
license reads the PEM from that key, not from a request body.

## AutoAcceptanceRule — admin-defined auto-approval

| Field | Type | Notes |
|---|---|---|
| `ID` | `int64` | PK |
| `OrganizationID` | `int64` | FK → Organization |
| `AgreementID` | `*int64` | Optional scope to a specific agreement; null = org-wide. |
| `RuleType` | `string` | `email_domain` or `seat_threshold`. |
| `MatchDomain` | `*string` | For `email_domain`: auto-approve requesters whose email matches. |
| `MaxAutoSeats` | `*int` | For `seat_threshold`: auto-approve until N seats are consumed, then fall back to manual. |
| `Enabled` | `bool` | |
| `CreatedByUserID` | `int64` | FK → OrgUser |
| `CreatedAt` | `time.Time` | |
| `DeactivatedAt` | `*time.Time` | |

**Any rule evaluation that fails (e.g. seat pool exhausted, domain mismatch with no fallback, key
validation error during auto-issuance) must notify all `customer_admin` users for the org.** The
`LicenseRequest.Reason` carries the failure detail. Notification channel is TBD (see Open Questions).

## SSOConfiguration — per-org OIDC federation (durable IdP lifecycle record)

The minimum OIDC parameters the API needs to register a new identity provider in the Cognito pool on
the org's behalf, **and the durable record of that provisioned IdP** — the source of truth for "this
org has an IdP named X."

| Field | Type | Notes |
|---|---|---|
| `ID` | `int64` | PK |
| `OrganizationID` | `int64` | FK → Organization (one active config per org). |
| `ProviderName` | `string` | The Cognito identity-provider name the API creates (derived, stable). **≤ 32 chars** (Cognito limit) — use an org slug or short hash. |
| `UserPoolID` | `string` | The Cognito user pool this IdP lives in. Stored per-config so the token validator can stay multi-issuer (a pool caps at 300/1,000 IdPs). |
| `OIDCIssuer` | `string` | Customer IdP issuer URL. |
| `OIDCClientID` | `string` | Customer IdP client ID. |
| `OIDCClientSecretEncrypted` | `*string` | **Secret — write-only.** Encrypted at rest (same scheme as `SigningKey.PrivateKeyEncrypted`); **never returned** by the API — responses expose only a presence flag `has_client_secret: true`. |
| `Scopes` | `string` | Space-delimited (default `openid email profile`). |
| `AttributeMapping` | `jsonb` | IdP-claim → Cognito-attribute map (email, name, …). |
| `CognitoStatus` | `string` | `pending`, `active`, `failed` — provisioning state in the pool. A `failed` create is surfaced read-only and resolved via support, not silently retried. |
| `LoginURL` | `string` | Pre-encoded deep link that lands the user straight at their IdP, bypassing the Cognito provider picker. |
| `CreatedAt` / `UpdatedAt` | `time.Time` | |

**Lifecycle: create-once, read-only thereafter.** Changing a live IdP can brick account access (lock
out every SSO user in the org), so it is deliberately a **support question**, not a portal button. The
portal exposes only **create** (`POST`, rejected if an active config already exists) and **read**
(`GET`, returning the non-secret fields above + the presence flag). Updates and deletes go through
**internal/support-only** tooling (`UpdateIdentityProvider` / `DeleteIdentityProvider`), staff-gated
and audited — never wired to a customer-reachable route.

## Relationship to existing models

- `IssuedToken` gains an optional `OrgUserID *int64` FK so issued licenses link back to the seat
  (today it only carries `UserEmail`).
- `Organization`, `Agreement`, `License`, `SigningKey` are unchanged. `SigningKey` remains Janus's
  _JWT-signing_ keypair and is unrelated to the per-user run-log keys above.

---

# API Surface

All portal endpoints sit behind the Cognito access-token middleware already in place
(`Authentication` + `RequireUserInfo`). Routes are grouped by audience; the admin/member split is
enforced by Cognito group membership, not a static domain allowlist.

## End user (any authenticated SSO member)

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/me` | Current user profile (org, role, seat status). |
| `GET` | `/api/v1/me/key` | Get the caller's active public key (+ fingerprint), and archived-key history. |
| `PUT` | `/api/v1/me/key` | Set/replace the active public key (validated + fingerprinted). In one transaction: revokes the prior active key, revokes the prior `IssuedToken`, and issues a new license. **Must require explicit acknowledgement of the rotation caveat** before replacing an existing key. |
| `POST` | `/api/v1/me/license-requests` | Request a license/seat against an agreement. |
| `GET` | `/api/v1/me/license-requests` | Status of the caller's requests. |
| `GET` | `/api/v1/me/license` | Download the issued license JWT (only once `approved` + active key). |
| `GET` | `/api/v1/me/download` | Janus build download link, gated on an issued license. |

## Customer admin (`customer_admin` group)

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/orgs/{id}/members` | List org members + roles. |
| `POST` | `/api/v1/orgs/{id}/members/{userId}/promote` | Grant `customer_admin` (assigns the Cognito group). |
| `POST` | `/api/v1/orgs/{id}/members/{userId}/demote` | Revoke `customer_admin`. |
| `GET` | `/api/v1/orgs/{id}/license-requests` | Review the queue (pending + history). |
| `POST` | `/api/v1/orgs/{id}/license-requests/{reqId}/approve` | Manual approval → triggers minting. |
| `POST` | `/api/v1/orgs/{id}/license-requests/{reqId}/reject` | Manual rejection with reason. |
| `GET`/`POST`/`PUT`/`DELETE` | `/api/v1/orgs/{id}/rules` | CRUD auto-acceptance rules. |
| `POST` | `/api/v1/orgs/{id}/sso` | **Create-once.** Drives Cognito IdP provisioning and returns the encoded `LoginURL`; rejected if an active config already exists. |
| `GET` | `/api/v1/orgs/{id}/sso` | **Read-only.** Returns the non-secret config (provider name, issuer, client ID, scopes, mappings, status, login URL) + `has_client_secret`. No `PUT`/`DELETE` — changes are support-only. |
| `GET` | `/api/v1/orgs/{id}/sso/login-url` | Fetch the pre-encoded deep-link for distribution to users. |

## Org onboarding / billing

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/v1/organizations` | Signup (exists). |
| `POST`/`GET` | `/api/v1/agreements` | Accept/list agreements (exists). |
| `POST` | `/api/v1/organizations/{id}/payment` | Add credit-card payment details (processor TBD). |

## Janus administrator (internal)

Existing protected endpoints — JWKS/key management (`/api/v1/keys`, `/api/v1/keys/rotate`),
organization and agreement CRUD. These remain Janus-staff-only.

---

# Authorization Model — important note

The auth middleware shipped for the license server enforces a **static email-domain allowlist**
(`LICENSING_OIDC_ALLOWED_DOMAINS`), which is correct for **Janus-staff** access to the admin API.

The portal is **multi-tenant**: SSO end users arrive with arbitrary customer email domains, so the
static allowlist cannot be the authorization gate for portal endpoints. Portal authorization must
instead be based on:

1. **Org membership** — the caller's `OrgUser` record (resolved from the token `sub`) must belong to
   the org in the route.
2. **Cognito group** — `customer_admin` membership (from the `cognito:groups` claim) gates the admin
   routes; everyone authenticated can hit the `/me` routes.

This implies the token-validation layer should additionally surface the `cognito:groups` claim
(currently `ValidateToken` extracts only `sub`, `client_id`, `token_use`). Two authorization
profiles will coexist: **domain-allowlist** for the staff admin API, **membership + group** for the
multi-tenant portal API.

---

# Open Questions

- **Notification channel** for auto-allocation failures (req. 11 & 14) — email (SES?) vs. in-portal
  inbox vs. both.
- **Payment processor** — "all credit card" names no provider (Stripe?). Affects the payment
  endpoint + webhook surface.
- **Key rotation semantics** — _decided:_ one active key per user; replacing it is allowed but
  consequential. On replace, a new license is issued immediately and the prior `IssuedToken` is
  revoked at the point of request (treated as "getting a new license"). The user must install the
  new license file **and** update their local `signing.private_key_path`. Prior keys are archived
  for verifying historical run logs. Note this revocation is bookkeeping only — Janus does not phone
  home, so the old token remains cryptographically usable; revoking it just keeps the server record
  honest about which license is current.
- **Seat accounting** — `Agreement.Seats` is currently a bare count; the portal needs to track
  consumption (issued licenses per agreement) to enforce `seat_threshold` rules.
