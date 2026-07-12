import { describe, it, expect } from "vitest";
import { buildLicensePublicKeyFile, masterEmbedKeys, EMBED_FILENAME } from "./embed-keys";
import type { SigningKey } from "./types";

const key = (over: Partial<SigningKey>): SigningKey => ({
  key_id: "key-x",
  organization_id: null,
  public_key_pem: "-----BEGIN PUBLIC KEY-----\nX\n-----END PUBLIC KEY-----",
  active: false,
  created_at: "2026-01-01T00:00:00Z",
  expires_at: "2027-01-01T00:00:00Z",
  ...over,
});

describe("embed-keys", () => {
  it("uses the filename the build embeds", () => {
    expect(EMBED_FILENAME).toBe(".license_public_key.pem");
  });

  it("includes only non-revoked global keys, newest first", () => {
    const keys = [
      key({ key_id: "old", created_at: "2026-01-01T00:00:00Z", public_key_pem: "PEM_OLD" }),
      key({ key_id: "new", created_at: "2026-06-01T00:00:00Z", public_key_pem: "PEM_NEW", active: true }),
      key({ key_id: "org", organization_id: 7, public_key_pem: "PEM_ORG" }), // org-scoped → excluded
      key({ key_id: "revoked", revoked_at: "2026-03-01T00:00:00Z", public_key_pem: "PEM_REVOKED" }),
    ];

    expect(masterEmbedKeys(keys).map((k) => k.key_id)).toEqual(["new", "old"]);

    const file = buildLicensePublicKeyFile(keys);
    expect(file).toBe("PEM_NEW\nPEM_OLD\n");
    expect(file).not.toContain("PEM_ORG");
    expect(file).not.toContain("PEM_REVOKED");
  });

  it("trims each PEM block and newline-separates them", () => {
    const file = buildLicensePublicKeyFile([
      key({ key_id: "a", created_at: "2026-02-01T00:00:00Z", public_key_pem: "\nAAA\n\n" }),
      key({ key_id: "b", created_at: "2026-01-01T00:00:00Z", public_key_pem: "  BBB  " }),
    ]);
    expect(file).toBe("AAA\nBBB\n");
  });

  it("returns an empty string when there are no global keys", () => {
    expect(buildLicensePublicKeyFile([])).toBe("");
    expect(buildLicensePublicKeyFile([key({ organization_id: 3 })])).toBe("");
  });
});
