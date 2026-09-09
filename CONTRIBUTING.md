# Contributing to Janus

Thanks for considering it. Janus is a desktop tool for running and tracking
pharmacometric models — NONMEM, PsN, bbi — and most of the useful contributions
come from people who actually run models and hit something that annoyed them.

## Before you start

For anything more than a small fix, **open an issue first**. Janus has a few
design commitments that are not obvious from the code, and it is much less
frustrating to hear about them before you have written the patch than after. The
big ones are in [CLAUDE.md](CLAUDE.md) and
[documentation/features/signed_runlog.md](documentation/features/signed_runlog.md).

Bug reports are more useful with: what you ran, what you expected, what happened,
your OS, your execution mode (`execution-mode` in your config), and the Janus
version from `janus version`.

## Getting a build

You need **Go 1.25+** and a C toolchain, because the GUI is [Fyne][fyne] and Fyne
needs cgo. Windows is the fiddly one — see the compiler notes in the
[README](README.md#development-setup-windows).

```bash
git clone https://github.com/shairozan/janus
cd janus
go build ./...
go test ./...
```

No credentials, no tokens, no private registries. If a build step asks you for
any of those, that is a bug — please report it.

**Docker is easier**, if you have it, because it pins the toolchain for you:

```bash
mage docker:buildDev    # first time only, 5-10 minutes
mage docker:check       # format, lint, unit tests
```

## Tests

Janus has several test tiers, separated by build tags, and it is worth knowing
which one you are running:

```bash
go test ./...                              # the default tier — fast, no external tools
go test -tags unit ./...                   # adds the unit-tagged suites; CI runs this
go test -tags validation ./internal/validation/...   # CFR-21 requirements suite
```

The `validation` suite maps to numbered requirements and is what a regulated user
executes to qualify their own installation. If you change execution behaviour,
check whether a requirement covers it.

Both `go test ./...` and `go test -tags unit ./...` must pass before a PR merges.
A few suites skip themselves when the host cannot support them — no NONMEM, no
credential store, no Docker — and that is expected rather than a failure.

## Style

Run the linter before pushing; it is stricter than `go vet`:

```bash
mage lint     # or: mage docker:lint
```

Beyond what the linter catches, the thing reviewers actually care about is
**comments that explain why**. The codebase leans heavily on this, particularly
anywhere the obvious implementation would be wrong. If you find yourself writing
a subtle guard, say what breaks without it.

Match the surrounding code rather than importing conventions from elsewhere.

## Commits and pull requests

Conventional-commit prefixes (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`,
`chore:`), and a body that explains the reasoning rather than restating the diff.

Keep a PR to one concern. A rename touching 200 files and a behaviour change in
the same commit is genuinely hard to review — split them.

If your change alters what a user sees or does, update the docs in the same PR.
Documentation that lags the code is worse than none, because people trust it.

## Things that need care

Some parts of Janus carry more consequence than the code suggests:

- **The run log signing and verification path** (`internal/runlog`,
  `internal/signing`). This is tamper-evidence people rely on in regulated work.
  A change that makes a forged record verify is the most serious bug this project
  can have. Never let a record's own embedded key vouch for itself.
- **Container image labels** (`internal/container/types.go`). These are a
  compatibility contract with images users have already built; renaming one
  silently stops those images being discovered.
- **The NONMEM license lookup** (`internal/execution/category`). Unrelated to
  Janus's own licensing — it locates the user's `nonmem.lic`.

## Licensing

Janus is [MIT licensed](LICENSE). Contributions are accepted under the same
terms; there is no CLA to sign.

Do not contribute code you do not have the right to relicense, and **never**
commit anything NONMEM-derived. NONMEM is licensed commercial software, so its
source, binaries, and container images cannot be redistributed here — which is
why Janus ships no NONMEM image and asks you to build your own.

## Security

Do not report vulnerabilities in a public issue. See [SECURITY.md](SECURITY.md).

[fyne]: https://fyne.io
