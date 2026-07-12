// Lightweight client-side check that a string looks like a PEM-encoded public
// key, so we can validate before submit and give immediate feedback. The
// authoritative RSA parse happens server-side (signing.LoadPublicKeyFromPEM).
const PUBLIC_KEY_PEM = /-----BEGIN PUBLIC KEY-----[\s\S]+?-----END PUBLIC KEY-----/;

export function looksLikePublicKeyPem(value: string): boolean {
  return PUBLIC_KEY_PEM.test(value.trim());
}
