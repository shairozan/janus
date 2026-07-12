import { request } from "@playwright/test";

const MOCK_URL = `http://localhost:${process.env.MOCK_PORT ?? "4319"}`;

// A valid-looking PEM public key — passes the portal's client-side check
// (lib/pem.ts); the mock never actually parses it.
export const PUBLIC_KEY_PEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEArandomE2EpublicKeyMaterial
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
-----END PUBLIC KEY-----`;

export const SECOND_KEY_PEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAsecondRotatedKeyMaterial99
BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
-----END PUBLIC KEY-----`;

/** resetMock restores the mock license-server to its seed state (per-test isolation). */
export async function resetMock(): Promise<void> {
  const ctx = await request.newContext();
  await ctx.post(`${MOCK_URL}/__reset`);
  await ctx.dispose();
}

/**
 * enableStaff flips the mock caller to Janus-staff so /me reports is_staff and
 * the staff "Admin" area is reachable. Call after resetMock in staff specs.
 */
export async function enableStaff(): Promise<void> {
  const ctx = await request.newContext();
  await ctx.post(`${MOCK_URL}/__staff`);
  await ctx.dispose();
}
