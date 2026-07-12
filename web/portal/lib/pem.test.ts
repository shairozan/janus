import { describe, it, expect } from "vitest";
import { looksLikePublicKeyPem } from "./pem";

const VALID = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0Z...
-----END PUBLIC KEY-----`;

describe("looksLikePublicKeyPem", () => {
  it("accepts a well-formed PUBLIC KEY block (with surrounding whitespace)", () => {
    expect(looksLikePublicKeyPem(`\n  ${VALID}\n`)).toBe(true);
  });

  it("rejects arbitrary text", () => {
    expect(looksLikePublicKeyPem("not a key")).toBe(false);
  });

  it("rejects a private key block", () => {
    expect(
      looksLikePublicKeyPem("-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----")
    ).toBe(false);
  });

  it("rejects an empty string", () => {
    expect(looksLikePublicKeyPem("")).toBe(false);
  });
});
