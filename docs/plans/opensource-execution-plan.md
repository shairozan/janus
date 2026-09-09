# Opensourcing Janus — Execution Plan

**Status**: proposed
**Created**: 2026-09-09
**Source brief**: [opensource.md](opensource.md)

## Goal

Turn Janus from a proprietary, license-gated product into an MIT-licensed open
source project that anyone can clone, build, and run with no license file, no
license server, and no private-repo credentials — while keeping the run-log
signature chain fully functional.

## The shape of the problem

Signing is the only part of this that is a rewrite rather than a deletion.
Everything else is removal, renaming, and CI reconstruction.

Today the trust chain runs: license JWT carries a `SigningPublicKey` claim →
[`appsetup.BuildSigner`](../../internal/appsetup/appsetup.go) proves the configured
private key matches that claim → [`runlog.NewLicenseTrust`](../../internal/runlog/trust.go)
anchors run-log verification in the license. Remove the license and both the key
source and the trust anchor lose their footing.

The replacement is two separate mechanisms that are easy to conflate:

| Half | What it holds | Where it lives |
|---|---|---|
| **Credential store** | the user's *private* signing key, plus the identity bound to it | OS credential store (macOS Keychain, Windows Credential Manager, Linux Secret Service) |
| **Trust anchor** | the *public* keys whose signatures you accept | user/org-maintained keyring file |

The trust-anchor half already exists: `runlog.NewKeyringTrust(path)` is written
and unit-tested at [trust.go:130](../../internal/runlog/trust.go:130), but nothing
calls it. It was left as a seam for exactly this change. The credential-store
half is new work with no dependency in `go.mod` yet.

## Signer identity: why it is bound to the key

`SignerEmail` is written into every run record and is the label the trust keyring
matches a public key to. Verification anchors on the key *fingerprint*, never on
the email — so the email is a human-readable property of a **key**, not of the
process that happens to be running.

That rules out a runtime lookup. If the identity were resolved per-run, the email
on a record could drift from the key that signed it: the same keypair, a user
edits their git config or moves to another host, and now two identities exist for
one key with nothing detecting the mismatch. In a run log whose entire purpose is
tamper-evident provenance, that is a defect rather than a cosmetic wrinkle.

So identity is captured **once**, at `janus keys generate`, and stored in the
credential store alongside the private key. Record identity then matches the
signing key by construction.

### Why git seeds it, and why `os/user` does not

`os/user` cannot supply an email on any platform — the best it offers is a
display name, which would put a non-email into a field serialized as
`signer_email`. Its name field is also unreliable on precisely the hosts Janus
targets:

| Platform | `user.Current().Name` | Notes |
|---|---|---|
| Windows | populated (verified: `"Darrell Breeden"`) | `Username` is machine-qualified (`HOST\user`) |
| macOS | populated | `pw_gecos` filled from Open Directory RealName |
| Linux | **frequently empty** | `pw_gecos` cut at the first comma; `useradd` without `-c` and nearly all container images leave it blank |

Worse, under Kubernetes `runAsUser: 1001` with no matching passwd entry,
`user.Current()` does not return an empty name — it fails outright with
`user: unknown userid 1001`. SLURM nodes and containers are Janus's primary
execution targets, so the weakest platform is the most common one.

`git config --get user.email` gives a real email, honors the
local > global > system precedence chain, and is an identity users already
curate — so it is usually correct on the first try.

**Git is a setup-time dependency, not a runtime one.** It is consulted only when
generating or importing a key. Requiring it per-run would mean shelling out on
exactly the hosts least likely to have it (HPC compute nodes, slim containers),
and Janus already builds with `-buildvcs=false`, deliberately avoiding git at
build time. When git is absent or unconfigured, `keys generate` prompts and
stores whatever the user supplies; nothing downstream can tell the difference,
because the value is just a string in the credential store from that point on.

The address is not hard-validated — shared and role identities are legitimate,
and the keyring, not the format of the string, is the authority on who is
trusted.

## Decisions locked

- **Credential store**: OS-native credential store for the private key, with a
  file-based fallback for headless/CI use.
- **Signer identity**: bound to the signing key at generation time and stored
  with it in the credential store — *not* looked up at runtime. Seeded from
  `git config --get user.email`, with a prompt fallback. See
  [Signer identity](#signer-identity-why-it-is-bound-to-the-key) below.
- **Module path**: `github.com/pharmalytica/janus` → `github.com/shairozan/janus`.
- **Portal and licensing API**: deleted outright, not archived in-tree.
- **LICENSE**: relicensed to MIT.
- **Validation test suites**: the `//go:build validation` suites under
  `internal/validation/` stay untouched.

## Dependency inventory: what actually consumes the Janus license

The Janus license enters the process as a `*validator.Claims` and reaches exactly
nine call sites outside `internal/license`. That value — not the word "license" —
is the thing to trace.

### The three entry points

There are **three independent license gates**, not one:

| Gate | Path discovery | Enforcement |
|---|---|---|
| GUI / main binary | `--license` flag, default `~/.config/janus/license.jwt` ([cmd/root.go:101](../../cmd/root.go:101)) | [cmd/gui/gui.go:97](../../cmd/gui/gui.go:97) → `SetLicenseClaims` |
| `executor` binary | `--executor-license` flag, then `$JANUS_LICENSE`, then `~/.config/janus/license.jwt`, then `./license.jwt` ([cmd/executor/config.go:103](../../cmd/executor/config.go:103)) | hard exit at [cmd/executor/main.go:68](../../cmd/executor/main.go:68) |
| `janus mcp server` | own `--license` flag + `defaultLicensePath()` ([cmd/janus/commands/mcp/server.go:62](../../cmd/janus/commands/mcp/server.go:62)) | hard fail at [server.go:98](../../cmd/janus/commands/mcp/server.go:98) |

Each carries its own flag and its own path resolution; the executor adds an env
var and its own `embed.FS`
([cmd/executor/license.go:13](../../cmd/executor/license.go:13)). Removing only the
GUI gate leaves the other two refusing to start.

> The MCP gate was invisible during the first pass through this codebase because
> `cmd/janus/` was excluded from git by the `.gitignore` bug described in the
> Sprint 0 results. It surfaced only once the tree was recovered — a reminder
> that on this repo, "not in git" did not mean "not in the build".

**The executor gate is already dead code.** Its `//go:embed *` cannot ever
supply `.license_public_key.pem`: `go:embed *` excludes dotfiles by rule, and the
file is not in `cmd/executor/` anyway — the CI action writes it only to the repo
root. So `publickey.GetPublicKeys` always fails with "no public key embedded",
`verifyLicense` takes the dev-mode branch ([license.go:34](../../cmd/executor/license.go:34)),
prints a warning, and returns nil. Deleting it changes no runtime behavior.

### The three values actually consumed

`Claims` carries seven fields. Only three are read outside `internal/license`:

| Field | Consumed by | Replacement |
|---|---|---|
| `Features` | 4 real gates (below) | delete the gating |
| `UserEmail` | signer identity: [app.go:3815](../../internal/gui/app.go:3815), [mcp_bridge.go:44](../../internal/gui/mcp_bridge.go:44), stored on every record as `RunRecord.SignerEmail` | identity bound to the key |
| `SigningPublicKey` | trust anchor via `NewLicenseTrust` ([appsetup.go:106](../../internal/appsetup/appsetup.go:106)) and key-pair proof via `ValidateSigningKeyPair` ([appsetup.go:72](../../internal/appsetup/appsetup.go:72)) | keyring file |

`AgreementID`, `OrganizationID`, `Tier`, and `MaxSeats` are read only by a debug
`printf` and one startup log line. Nothing branches on them. The replacement
surface is three values, not a struct.

### The fail-closed trap

Both feature-gate helpers return **false** when claims are nil:
[`App.HasFeature`](../../internal/gui/app.go:270) and
[`Service.hasFeature`](../../internal/mcpservice/service.go:81). So the naive move
— stop loading the license, leave claims nil — silently *disables* the features
instead of opening them. The four gates that would go dark:

- `grid`: [app.go:617](../../internal/gui/app.go:617) (SLURM scheduler selection),
  [slurm_monitor.go:584](../../internal/gui/slurm_monitor.go:584)
- `runlog`: [app.go:4387](../../internal/gui/app.go:4387),
  [app.go:4784](../../internal/gui/app.go:4784),
  [mcpservice/service.go:220](../../internal/mcpservice/service.go:220)

This is why Sprint 1 deletes the gating rather than defaulting it.

### Not in scope, despite matching a text search

- **`internal/config` has zero Janus-license configuration.** Every `License`
  field there is NONMEM: `NONMEMLicenseConfig`, the deprecated
  `HermesLicenseConfig`, `ValidateNONMEMLicenseForExecution`,
  `RequiresNONMEMLicense`. The Janus license was never a config key — it is a
  file path from a flag or env var. The only config surface entangled with it is
  `SigningConfig.PrivateKeyPath`, and only via its doc comment and
  `ValidateSigningKeyPair`.
- **`internal/execution` and `internal/execution/category`** read `nonmem.lic` and
  ship it into Hermes containers (`RequiresLicense`, `GetLicense`,
  `readLicenseFile`, `LICENSE_FLAG`). Entirely NONMEM.
- **`internal/qa`** — `nonmemLicenseCheck` in the OQ preflight. NONMEM.
- **`internal/runlog`** — every hit but `NewLicenseTrust` is a comment.

---

# Sprint 0 — Pre-flight (blocking, start immediately)

Short sprint, but the Apple item has the longest lead time in the whole plan and
gates Sprint 6. Start it before Sprint 1.

1. **Verify the Apple Developer ID certificates in Bitwarden are still valid.**
   The real risk is not the files — it is the *account*. If the Developer ID
   certs were issued under a Pharmalytica organization enrollment and that
   enrollment lapsed when the company shut down, the certificates are revoked or
   unrenewable regardless of whether the `.p12` still opens. Confirm:
   - Which Apple Developer account (personal vs. org) holds the enrollment
   - Whether that account is active and paid
   - Certificate expiry dates for both *Developer ID Installer* and
     *Developer ID Application*
   - Whether the private keys came across in the Bitwarden export (a `.p12`
     without its key is useless)
   - **If the enrollment is gone**: re-enrolling under a personal Apple Developer
     account takes days to weeks. Everything else in this plan can proceed; only
     signed/notarized macOS packages are blocked.
2. Confirm `github.com/shairozan/hermes` is public, note the tag to pin, and check
   whether its own module path was rewritten in the move (if the module line still
   says `pharmalytica`, that changes the Sprint 4 import swap).
3. Confirm the `dukeofubuntu` Docker Hub account and its CI images survive the
   shutdown — `release.yml` and `test.yml` build inside `dukeofubuntu/janus-ci:*`.

**Exit criteria**: a written yes/no on Apple signing viability, a pinned hermes
version, and confirmed Docker Hub access.

## Sprint 0 results (2026-09-09)

| Item | Result |
|---|---|
| hermes public | ✅ clones unauthenticated; MIT licensed |
| hermes module path | ✅ **resolved during this sprint** — `v0.0.4` (97ddf16) declares `github.com/shairozan/hermes` and is indexed by the Go proxy. Sprint 4 pins that. |
| Docker Hub | ✅ `dukeofubuntu/janus-ci` public: `ubuntu20`, `ubuntu22`, `ubuntu24`, plus an `ubuntu26` the release matrix does not yet use |
| janus git remote | ✅ already `git@github.com:shairozan/janus.git` |
| Apple certificates | ⬜ outstanding — needs the Bitwarden export and the Developer account status |

Hermes needed a **new version number, not a re-tag**: `v0.0.1`-`v0.0.3` point at
commits predating the rename and still declare `github.com/pharmalytica/hermes`
inside, and `proxy.golang.org` caches versions immutably — so force-moving an
existing tag would have kept serving the old content and produced a checksum
mismatch. Cutting `v0.0.4` off `main` sidestepped that.

### Two build breakages found on `main`

Neither is caused by this work; both must be fixed for the repo to be usable
once public.

**1. `.gitignore` was swallowing a source tree.** Line 6 held the bare pattern
`janus`, intended for the built binary at the repo root. Git patterns without a
slash match at any depth *and* match directories, so it also matched
`cmd/janus/` — and `git add` skipped the whole tree silently, with no warning,
from the initial commit onward. The four packages
`cmd/janus/commands/{execute,hermes,mcp,validate}`, imported by
[cmd/root.go:14-17](../../cmd/root.go:14), were never in git history at all.

*Fixed*: the pattern is now anchored as `/janus`, and the tree was recovered from
a local backup. The other patterns (`janus.exe`, `janus-ubuntu*`, `janus_*.deb`)
are specific enough to be safe.

**2. `//go:embed .license_public_key.pem`** at [main.go:18](../../main.go:18) makes
a clean clone unbuildable — the file is only ever created by the CI
`setup-license-key` action, so `go build ./...` fails locally with
`pattern .license_public_key.pem: no matching files found`. Sprint 1 removes this
embed, which fixes it as a side effect.

---

# Sprint 1 — Unlicensed startup

Make Janus boot and run with no license file present. This sprint deliberately
does *not* delete `internal/license`; it stops the application from calling into
it, so the tree keeps compiling and every step stays testable.

### Work

**Gate 1 — the main binary.** Remove the `--license` flag and
`getDefaultLicensePath()` from [cmd/root.go:101](../../cmd/root.go:101), and the
`validateLicense` → `SetLicenseClaims` chain in
[cmd/gui/gui.go:97-111](../../cmd/gui/gui.go:97). Drop the `licenseClaims` field
from `App` ([app.go:49](../../internal/gui/app.go:49)) and the
`mcpservice.Options.License` it feeds ([mcp_bridge.go:30](../../internal/gui/mcp_bridge.go:30),
[service.go:45](../../internal/mcpservice/service.go:45)).

**Gate 2 — the executor.** Delete `cmd/executor/license.go`, the
`verifyLicense` call at [main.go:68](../../cmd/executor/main.go:68),
`ExecFlags.License` and its parsing ([args.go:48-56](../../cmd/executor/args.go:48)),
`getDefaultLicensePath` including the `$JANUS_LICENSE` lookup
([config.go:103](../../cmd/executor/config.go:103)), and the `--executor-license`
lines in [help.go:23](../../cmd/executor/help.go:23) and
[help.go:68](../../cmd/executor/help.go:68). Per the inventory above this gate is
already inert, so it should be behaviour-neutral — but it is a hard `os.Exit`
path, so verify by running the executor with `$JANUS_LICENSE` set to a garbage
path both before and after.

**Gate 3 — the MCP daemon.** Delete the `--license` flag and
`defaultLicensePath()` from
[cmd/janus/commands/mcp/server.go:62](../../cmd/janus/commands/mcp/server.go:62),
the `ValidateLicensePath` call and the `License validated: Org=…` log line
([server.go:96-104](../../cmd/janus/commands/mcp/server.go:96)), and the
`License: claims` option at
[server.go:135](../../cmd/janus/commands/mcp/server.go:135). Also update the example
systemd unit in the command's help text
([server.go:47](../../cmd/janus/commands/mcp/server.go:47)), which tells users to
pass `--license` — leaving it would ship a flag that no longer exists.
`Command(assets)` and `serverCommand(assets)` lose their parameter here.

**Un-thread the embeds.** Two of them: `//go:embed .license_public_key.pem` at
[main.go:18](../../main.go:18), threaded as `assets embed.FS` through
`cmd.Command` → `gui.Command` → `RunGUI` → `appsetup.ValidateLicense*`; and
`//go:embed *` at [cmd/executor/license.go:13](../../cmd/executor/license.go:13).
Remove both and drop the parameter from all five signatures.
(`internal/qa/embed.go` has an unrelated `embed.FS` — leave it.)

**Open the feature gates — delete, do not default.** Both helpers fail closed on
nil claims, so leaving them in place with no license silently disables `grid` and
`runlog`. Remove `App.HasFeature` ([app.go:269](../../internal/gui/app.go:269)) and
`Service.hasFeature` ([service.go:80](../../internal/mcpservice/service.go:80))
along with all four call sites, rather than hardcoding them to `true` — that
leaves no dead tier machinery to mislead the next reader.

**Rewire the trust anchor.** `appsetup.BuildTrustStore(claims)` loses its claims
parameter and becomes keyring-backed via the already-written
`runlog.NewKeyringTrust`, defaulting to a path under `~/.config/janus/`. A
missing keyring is not an error — it means "verify nothing", the nil-trust path
[verify.go:362](../../internal/runlog/verify.go:362) already handles.

**Loosen the signer.** `BuildSigner` keeps loading `cfg.Signing.PrivateKeyPath`
but drops the `validator.ValidateSigningKeyPair` proof
([appsetup.go:72](../../internal/appsetup/appsetup.go:72)), which has no counterpart
without a license. Signing stays optional and off by default; credential-store
backing lands in Sprint 3.

**Signer identity.** Both consumers — `SetSigner`
([app.go:3815](../../internal/gui/app.go:3815)) and `App.signerEmail`
([mcp_bridge.go:41](../../internal/gui/mcp_bridge.go:41)) — stop reading
`claims.UserEmail` and instead take the identity carried by the signer itself.
In this sprint the signer is still file-backed, so the identity comes from a new
`signing.identity` config key; Sprint 3 moves it into the credential store next
to the key, and the call sites do not change again. Do **not** substitute an
`os/user` lookup here, even temporarily — see
[Signer identity](#signer-identity-why-it-is-bound-to-the-key).

This changes the value written to `RunRecord.SignerEmail` on new records.
Existing signed records keep their license-derived email and still verify, since
verification anchors on the fingerprint rather than the email, so old and new
records coexist safely.

### Exit criteria

- `janus` starts with no `license.jwt` on the system and no `$JANUS_LICENSE` set
- `executor` runs a model under the same conditions
- Grid appears with a SLURM scheduler configured; run-log recording is on
- A run signs, and verifies against a keyring file
- Unit tests green

---

# Sprint 2 — Delete the licensing service

Pure removal, roughly 300 files. Nothing here should require design decisions if
Sprint 1 landed cleanly.

### Delete

| Path | Files |
|---|---|
| `internal/license/**` | 144 |
| `web/portal/**` | 110 |
| `deploy/license-server/**` | 22 |
| `deploy/portal/**` | 14 |
| `cmd/license-server/**` | 8 |
| `docker/license/**` | 4 |
| `.github/workflows/license-server-image.yml` | 1 |
| `.github/actions/setup-license-key/**` | 1 |
| `licensing.md`, `documentation/licensing.md`, `documentation/LICENSE_KEY_WORKFLOW.md`, `documentation/features/management-portal.md` | 4 |
| `scripts/generate-license.sh`, `scripts/license-workflow.sh`, `scripts/license-server-api.postman_collection.json`, `scripts/LICENSE_QUICK_START.md` | 4 |

Check `scripts/generate.sh` too — it references the licensing service but may
have unrelated responsibilities worth keeping.

Also: the `build-license-server` job in `release.yml`, and the
`LICENSE_PUBLIC_KEY` secret references in `release.yml` (lines 34, 217) and
`test.yml` (line 28). (`cmd/executor/license.go` already went in Sprint 1.)

### Notes

- CI breaks the moment `setup-license-key` is deleted, so the workflow edits
  belong in *this* sprint, not deferred to Sprint 6. Sprint 6 is the *rebuild* of
  release signing, not the de-licensing.
- Run `go mod tidy` afterward — Cognito/AWS/Stripe/SSO dependencies pulled in
  only by `internal/license` should fall out of `go.mod`. Expect a large and
  welcome dependency reduction.
- `release.yml:143` builds `./cmd/license-server` with ldflags into
  `internal/license/version` — that whole job goes.
- Sanity check is the *import graph*, not a text search: `grep -r
  "internal/license"` returns nothing and `go build ./...` is green. A `licens`
  text sweep is useless here — the surviving hits are all NONMEM, per the
  inventory above, and treating them as scope is the main way this sprint goes
  wrong.

### Exit criteria

Full build green, `mage docker:checkAll` passes, no reference to the licensing
service remains in Go, YAML, or Dockerfiles.

---

# Sprint 3 — Local credential store for signing

The one sprint with genuine design work.

### Work

- **Pick and add the credential-store library.** `99designs/keyring` (broadest
  backend coverage, including an encrypted-file backend for headless hosts) or
  `zalando/go-keyring` (smaller, no file fallback). The headless case argues for
  the former, since `mage docker:*` and GitHub runners have no unlocked desktop
  keychain.
- **Extend `SigningConfig`.** Today it is a bare `PrivateKeyPath`
  ([config.go:275](../../internal/config/config.go:275)). Add a backend selector so
  a key can live in the OS store *or* on disk, keeping the file path working for
  existing users and containers.
- **Load through the store.** `signing.NewSigner` currently only reads a PEM file
  ([signing.go:26](../../internal/signing/signing.go:26)). Add a constructor that
  pulls PEM bytes from the credential store and reuses the existing
  `ParsePrivateKeyFromPEM`. The RSA-SHA256 signing and verification logic in
  `internal/signing` and `internal/runlog/signer.go` is unchanged — this is
  strictly about where key bytes come from.
- **Add key management commands.** `janus keys generate | import | list |
  export-public | fingerprint`. Generating a key must be a first-run-friendly
  path; a user should get from clone to signed run without hand-rolling OpenSSL
  commands.
- **Capture identity at generate/import time.** Both commands take
  `--identity`, and when it is omitted they seed from
  `git config --get user.email` and confirm, falling back to a prompt when git is
  missing, unconfigured, or the command errors. The identity is stored in the
  credential store entry alongside the private key, and every signer built from
  that entry carries it — so `RunRecord.SignerEmail` always matches the key that
  produced the signature. `export-public` emits the identity and public key as a
  ready-to-paste keyring entry.
- **Detect and report drift.** Since the identity is now a property of the key,
  `janus keys list` should surface when a key's stored identity differs from the
  current `git config user.email`. Report it; do not auto-correct. Silently
  rewriting the identity would reintroduce exactly the drift this design
  prevents.
- **Document the two halves.** The credential store holds your private key; the
  keyring file says whose signatures you accept. Users will conflate these — the
  docs need to be explicit, and `export-public` should emit something that pastes
  straight into a keyring file.

### Risks

- Backends differ sharply: macOS Keychain prompts on access, Windows Credential
  Manager caps blob size (RSA PEMs are comfortably under it, but verify), and
  Linux Secret Service is absent on headless servers — which is exactly Janus's
  deployment profile in pharma. The file fallback is not a nicety; it is the
  primary path for a meaningful share of users.
- CGO: macOS Keychain backends need it. `release.yml` already builds with
  `CGO_ENABLED: 1`, so this is compatible, but confirm the Linux container builds
  are unaffected.
- Git invocation must be defensive. `git config --get user.email` exits non-zero
  when the key is unset, which is an ordinary "no seed available" case and not an
  error to surface. Look git up on `PATH` rather than assuming it; a repo-local
  `.git/config` legitimately overrides the global value, so run it from the
  user's working directory and let git resolve precedence itself rather than
  parsing `~/.gitconfig`.

### Exit criteria

On macOS, Windows, and Linux: generate a key into the store, sign a run, verify
against a keyring, and confirm the file fallback works headless. Generation seeds
the identity from git when configured, prompts when git is absent or has no
`user.email`, and the identity on a signed record matches the key's stored
identity in every case.

---

# Sprint 4 — Repo identity: pharmalytica → shairozan

Large but mechanical. Keep it as its own commit series so review stays tractable.

- **Module path**: `github.com/pharmalytica/janus` → `github.com/shairozan/janus`
  in `go.mod` and 239 files of self-imports. One mechanical commit.
- **Hermes**: `github.com/pharmalytica/hermes v0.0.1` →
  `github.com/shairozan/hermes v0.0.4` — 36 import lines
  across 10 files (`internal/execution/hermes*.go`, `internal/runlog/logger.go`,
  `internal/runlog/types.go`, `internal/kube/kube_test.go`).
- **Delete the private-repo plumbing**, now pointless since hermes is public:
  - `GOPRIVATE` + `GITHUB_TOKEN` passing in
    [magefiles/docker.go:49](../../magefiles/docker.go:49) and
    [magefiles/docker.go:186](../../magefiles/docker.go:186)
  - `ARG GITHUB_TOKEN` / `git config url.insteadOf` in
    [docker/Dockerfile.dev:8](../../docker/Dockerfile.dev:8)
  - `documentation/DOCKER_PRIVATE_REPOS.md`
- **Textual sweep** over the remaining ~300 files: `release.yml` ldflags and the
  `Homepage:` / `go install` lines (270-413), `deploy/`, `docker/`,
  `documentation/`, `.claude/workflows/council.js`.

### Exit criteria

`grep -ri pharmalytica` returns nothing; a clean clone builds with no GitHub
token and no `GOPRIVATE` set.

---

# Sprint 5 — Documentation and licence

- **`LICENSE` → MIT.** Replace the "PROPRIETARY SOFTWARE LICENSE / All rights
  reserved" text; carry the existing copyright holder forward.
- **Delete** `validation.md` (the CFR 21 Part 11 source-validation requirements
  doc) and `documentation/features/iqoq.md`. Review `documentation/gxp/` —
  `INTENDED_USE.md` and `KNOWN_LIMITATIONS.md` may be worth *rewriting* rather
  than deleting, since an open-source project still owes users an honest
  statement of what it does and does not claim.
- **Keep** `internal/validation/**` — those are build-tagged tests, not
  documentation, and stay per the brief.
- **Rewrite** `README.md` and `CLAUDE.md` for an OSS audience: no license
  purchase, no portal, build-from-source instructions, MIT badge.
- **Add** `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, issue and PR
  templates.
- **Document git as a setup-time prerequisite.** It is used to seed signer
  identity during `janus keys generate`, and is optional — Janus runs and signs
  without it. State that plainly wherever prerequisites are listed, so nobody
  reads it as a runtime requirement on compute nodes:
  - `README.md` prerequisites
  - `installer/macos/README.md` and the other installer READMEs, if they carry
    prerequisite lists
  - `documentation/features/signed_runlog.md` — needs the largest rewrite of any
    page. It currently instructs users to generate an RSA keypair by hand with
    OpenSSL (line 133) and submit the public key via `./scripts/generate-license.sh`
    (line 168), and ties non-repudiation to the license (line 8). All of that is
    replaced by `janus keys generate` and the keyring.

    It also documents a *limitation this work removes*: line 210 states that the
    commercial build populates the trust store with exactly one key and there is
    "no supported way to add a second key," leaving multi-user verification
    unreachable. Wiring `NewKeyringTrust` in Sprint 1 fixes that — colleagues can
    verify each other's records for the first time. Worth calling out in release
    notes as a capability gained, not just a license removed.
- **New docs**: credential-store setup per OS, the keyring trust model, how
  signer identity is captured and why it is bound to the key rather than looked
  up per run, and a plain statement of the project's compliance posture now that
  no vendor stands behind a validation package.

### Exit criteria

A newcomer can go from `git clone` to a running signed build using only in-repo
docs.

---

# Sprint 6 — CI and release reconstruction

Pharmalytica is shut down, so every secret and signing identity has to be
re-established under personal accounts.

### Secrets inventory

| Secret | Status |
|---|---|
| `LICENSE_PUBLIC_KEY` | **delete** (removed in Sprint 2) |
| `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` | re-issue under the `dukeofubuntu` account |
| `GITHUB_TOKEN` | automatic, no action |
| Apple signing + notarization | **all new** (see below) |

### macOS packaging — this is a build, not a re-secreting

There are currently **no** `APPLE_*` secrets in any workflow.
[`installer/macos/build-pkg.sh`](../../installer/macos/build-pkg.sh) expects the
Developer ID certificates to be sitting in an already-populated local keychain
(`security find-identity` at line 321), which is true on a developer's laptop and
false on a GitHub runner. So CI-side signing has to be written from scratch:

1. Import a base64-encoded `.p12` into a temporary keychain created on the
   runner, unlock it, and set it in the default search list.
2. Sign binaries with *Developer ID Application*, then the package with
   *Developer ID Installer* — `build-pkg.sh` already distinguishes the two via
   `APPLE_APPLICATION_CERT_NAME` / `APPLE_CERTIFICATE_NAME`.
3. Notarize with `notarytool` and staple the ticket. Prefer an App Store Connect
   API key (`APPLE_API_KEY_ID`, `APPLE_API_ISSUER_ID`, `APPLE_API_KEY_P8`) over an
   app-specific password — it does not expire with the Apple ID password.
4. Delete the temporary keychain in an `always()` step.

New secrets: `APPLE_CERTIFICATE_P12`, `APPLE_CERTIFICATE_PASSWORD`,
`APPLE_CERTIFICATE_NAME`, `APPLE_APPLICATION_CERT_NAME`, `APPLE_TEAM_ID`, plus
the notarization credentials.

**Contingency**: if Sprint 0 finds the Apple enrollment is gone, ship macOS
unsigned with documented Gatekeeper instructions and a tracking issue, rather
than blocking the whole release.

### Also in scope

- Drop the `build-license-server` job and `license-server-image.yml`.
- Rebuild the CI container images under the personal Docker Hub namespace and
  confirm `dukeofubuntu/janus-ci:ubuntu{20,22,24}` still pull.
- Fix `release.yml:413`'s `go install github.com/pharmalytica/janus@...` and the
  `Homepage:` field in the Debian control block.
- Windows and Linux installers: confirm nothing under `installer/` depended on
  license artifacts.

### Exit criteria

A tagged release builds and publishes artifacts for all six matrix targets from
a clean fork with only the new secrets configured.

---

## Sequencing

Sprints 1 → 2 → 3 are strictly ordered. Sprint 4 (renaming) can run in parallel
with Sprint 3 but should not overlap Sprint 2 — deleting 300 files and renaming
300 files at once produces an unreviewable diff. Sprint 5 can start any time
after Sprint 2. Sprint 6 depends on Sprint 0's Apple finding and Sprint 4's
module path.

## Open questions

- Does the GitHub repository itself move to `shairozan/janus`, and does the old
  `pharmalytica/janus` remote still exist to redirect from?
- Should the first open-source release be cut as `v1.0.0` or continue the
  existing version line? This affects the `go install` path and any users
  upgrading from a licensed build.
- Is there an existing licensed user base that needs a migration note for turning
  their license-embedded signing key into a credential-store key?
