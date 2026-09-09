# Signing keys and the trust keyring

Janus can sign run log records so that anyone reading them later can tell whether
they have been altered, and who produced them. This page covers where your key
lives and how other people come to trust it.

Signing is **optional**. Janus runs perfectly well with no key at all; records are
then simply unsigned, and reported as such.

## The two halves

These are separate mechanisms and are easy to confuse, so it is worth being blunt
about the difference:

| | What it holds | Where it lives | What it does |
|---|---|---|---|
| **Your signing key** | your *private* key | the OS credential store, or a PEM file | signs the records you produce |
| **The trust keyring** | other people's *public* keys | `signing.keyring_path`, a YAML file | decides whose signatures you accept |

Holding a key does not make you trust anybody. Trusting somebody does not require
holding a key. You can configure either half without the other:

- a machine that only *runs* models needs a key, and no keyring
- a machine that only *reviews* other people's runs needs a keyring, and no key

A missing keyring is not an error. It means "trust nobody yet", and signed records
are reported as `Unverifiable` rather than `Valid` — never silently accepted.

## Creating your key

```bash
janus keys generate
```

This creates an RSA key, stores the private half in your operating system's
credential store, and records the identity in your Janus config.

You are asked for an **identity** — conventionally your email. If `git config
--get user.email` is set, that value is offered as the default. Git is consulted
*only here*, never while Janus is running: HPC compute nodes and slim containers
frequently have no git, and Janus must not depend on it to sign a run.

To skip the prompt:

```bash
janus keys generate --identity you@example.com
```

### Why the identity is fixed to the key

The identity is captured once, when the key is created, and stored alongside it.
It is deliberately **not** looked up on each run.

If it were resolved per run — from git, or from the OS user — then editing your
git config, or running the same key on another host, would change the identity
recorded on new records while the key stayed the same. You would end up with one
key claiming two identities and nothing able to detect it. In a log whose entire
purpose is tamper-evidence, that is a defect rather than a cosmetic wrinkle.

So `janus keys list` will *tell* you when your git email and your key's identity
have diverged, but it will never quietly correct one to match the other. To sign
under a different identity, create a key for it.

## Where the private key is stored

By default, in the OS credential store:

| Platform | Store |
|---|---|
| macOS | Keychain |
| Windows | Credential Manager |
| Linux | Secret Service (GNOME Keyring, KWallet, …) |

**Headless hosts have no such store.** Servers, containers and CI runners
generally have no unlocked session keyring, which is a common way to run Janus.
Use a key file there instead:

```yaml
signing:
  backend: file
  private_key_path: ~/.config/janus/signing.pem
  identity: you@example.com
```

Protect it with filesystem permissions the way you would an SSH private key
(`chmod 600`). Both backends produce identical signatures; nothing downstream
knows or cares which was used.

If you omit `backend`, Janus infers it: a `private_key_path` means the file
backend, an `identity` alone means the credential store. Configs written before
the credential store existed therefore keep working untouched.

### Key size

`janus keys generate` produces RSA-2048 keys. This is not arbitrary: Windows
Credential Manager caps a single credential at 2560 bytes, and a 4096-bit private
key PEM is roughly 3.2KB — it cannot be stored there at all. A 2048-bit key PEM is
about 1.7KB.

If you import a larger key on Windows you will get an explicit error saying so,
with the file backend suggested as the alternative.

## Letting other people verify your records

Send them your public key:

```bash
janus keys export-public
```

The output is a ready-to-paste keyring entry:

```yaml
keys:
  - email: you@example.com
    public_key: |
      -----BEGIN PUBLIC KEY-----
      ...
      -----END PUBLIC KEY-----
```

They add it to the file at their `signing.keyring_path`. Sharing a public key lets
others check your signatures and grants them nothing else — your private key never
leaves your machine.

Compare fingerprints over a second channel if it matters, since the fingerprint is
what verification actually matches on:

```bash
janus keys fingerprint
```

## Configuring the trust keyring

```yaml
signing:
  keyring_path: ~/.config/janus/keyring.yml
```

```yaml
# ~/.config/janus/keyring.yml
keys:
  - email: alice@example.com
    public_key: |
      -----BEGIN PUBLIC KEY-----
      ...
      -----END PUBLIC KEY-----
  - email: bob@example.com
    public_key: |
      -----BEGIN PUBLIC KEY-----
      ...
      -----END PUBLIC KEY-----
```

Verification anchors on the key **fingerprint**, not on the email. The email is a
human-readable label; a record cannot talk its way into being trusted by claiming
somebody else's address, and a record's own embedded public key is never treated
as authority for itself.

A key that is not in your keyring produces `Signed by an UNAUTHORIZED key` — the
signature may be mathematically perfect, but a signature from a key nobody
authorised proves nothing. That is exactly what a forgery looks like.

## Moving a key between machines

```bash
janus keys import --key /path/to/signing.pem
```

The file is read, not modified or removed. Delete it yourself once you have
confirmed the key works.

Importing over an existing key for the same identity requires `--force`, because
replacing a key silently would leave every record already signed with the old one
unverifiable against the new one.

## Command summary

| Command | Purpose |
|---|---|
| `janus keys generate` | Create a key and bind an identity to it |
| `janus keys import --key PATH` | Adopt an existing PEM private key |
| `janus keys list` | Show the configured key, its backend and fingerprint |
| `janus keys export-public` | Print the public key as a keyring entry |
| `janus keys fingerprint` | Print the key's SHA-256 fingerprint |

## See also

- [`signed_runlog.md`](signed_runlog.md) — how signatures are computed and what
  each verification status means
