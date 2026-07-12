import type { SigningKey } from "./types";

// The filename the Janus build embeds (`//go:embed .license_public_key.pem` in
// main.go). The build verifies licenses against the public keys in this file.
export const EMBED_FILENAME = ".license_public_key.pem";

/**
 * buildLicensePublicKeyFile concatenates the PUBLIC PEMs of all non-revoked
 * global (master) signing keys — newest first — into the single file the Janus
 * build embeds. The parser (internal/license/publickey) reads every PUBLIC KEY
 * block and a license verifies if ANY key matches, so including a freshly
 * rotated key alongside recently-retired (but not revoked) ones means licenses
 * signed by either still verify. Org-scoped and revoked keys are excluded:
 *   - org-scoped (organization_id != null) sign per-org licenses, not the build's
 *   - revoked keys must never verify anything.
 * Only public material is ever emitted — the private halves never leave the
 * license-server.
 */
export function buildLicensePublicKeyFile(keys: SigningKey[]): string {
  const masters = masterEmbedKeys(keys);
  if (masters.length === 0) {
    return "";
  }

  return masters.map((k) => k.public_key_pem.trim()).join("\n") + "\n";
}

/** masterEmbedKeys returns the non-revoked global keys, newest first. */
export function masterEmbedKeys(keys: SigningKey[]): SigningKey[] {
  return keys
    .filter((k) => k.organization_id == null && k.revoked_at == null)
    .sort((a, b) => b.created_at.localeCompare(a.created_at));
}
