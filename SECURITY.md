# Security Policy

## Reporting a vulnerability

**Please do not open a public issue for a security vulnerability.**

Report it privately through GitHub's [Security Advisories][advisories] on this
repository ("Report a vulnerability"). That gives us a private channel to confirm
and fix the problem before details become public.

Please include what you would want if you were receiving the report: what the
issue is, how to reproduce it, which version or commit you saw it on, and what an
attacker could actually achieve with it.

[advisories]: https://github.com/shairozan/janus/security/advisories/new

## What to expect

Janus is maintained by volunteers, not a staffed security team. There is no
guaranteed response time, and no bug bounty. That said, a credible report will be
acknowledged and worked on, and you will be credited in the fix unless you would
rather not be.

If you have not heard back in two weeks, please open a plain issue saying only
that you sent a security report and had no reply — with no details in it.

## Supported versions

Fixes land on `main` and go out in the next release. Older releases are not
patched. If you are running Janus in a regulated environment, track releases and
qualify updates through your own change control.

## Scope

Things worth reporting:

- Forging or tampering with a **run log record** so that Janus reports it as
  `Valid` — the signature and hash chain are the security boundary that matters
  most here, since the run log is the tamper-evidence users rely on
- Getting a record signed by a key that is not in the trust keyring to verify
- Extracting a signing private key from the credential store through Janus
- Remote access to the MCP server, which binds loopback and requires a bearer
  token; anything that bypasses either
- Command injection through model files, configuration, or container image
  references

Things that are **not** vulnerabilities:

- **Someone with your signing private key can sign anything.** That is what a
  signing key is. Protect it the way you protect an SSH key.
- **An unrecognised signing key reports as `Untrusted`, not `Valid`.** That is
  the design working — see
  [documentation/features/signed_runlog.md](documentation/features/signed_runlog.md).
- **Janus does not implement access control, retention, or electronic
  signatures** under 21 CFR Part 11. It never claimed to; see
  [documentation/gxp/INTENDED_USE.md](documentation/gxp/INTENDED_USE.md).
- Vulnerabilities in NONMEM, PsN, Docker, or your scheduler. Report those to
  their maintainers.
- A private key stored in a file being readable by someone who already has your
  filesystem permissions.

## Handling of credentials

Janus stores two secrets locally, and neither is ever transmitted:

| Secret | Where | Notes |
|---|---|---|
| Run-log signing key | OS credential store, or a PEM file you point at | Never leaves the machine; only the public half is shared |
| MCP bearer token | your Janus config file | Protect the config file accordingly |

Janus has no telemetry and phones home to nothing.
