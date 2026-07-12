import { describe, it, expect } from "vitest";
import { buildAuthHeaders, ssoAuthParams, licenseServerBaseUrl } from "./api-helpers";

describe("buildAuthHeaders", () => {
  it("attaches the Bearer token when present", () => {
    const headers = buildAuthHeaders("tok-123");
    expect(headers.get("Authorization")).toBe("Bearer tok-123");
  });

  it("omits Authorization when there is no token", () => {
    const headers = buildAuthHeaders(undefined);
    expect(headers.has("Authorization")).toBe(false);
  });

  it("defaults JSON content-type but preserves caller headers", () => {
    const withDefault = buildAuthHeaders("t");
    expect(withDefault.get("Content-Type")).toBe("application/json");

    const explicit = buildAuthHeaders("t", { "Content-Type": "text/plain" });
    expect(explicit.get("Content-Type")).toBe("text/plain");
  });
});

describe("ssoAuthParams", () => {
  it("builds the identity_provider deep-link param", () => {
    expect(ssoAuthParams("knomix-okta")).toEqual({ identity_provider: "knomix-okta" });
  });

  it("returns no param for a blank provider (native login fallback)", () => {
    expect(ssoAuthParams("")).toEqual({});
    expect(ssoAuthParams("   ")).toEqual({});
  });

  it("trims surrounding whitespace", () => {
    expect(ssoAuthParams("  acme-idp  ")).toEqual({ identity_provider: "acme-idp" });
  });
});

describe("licenseServerBaseUrl", () => {
  it("reads LICENSE_SERVER_URL", () => {
    expect(licenseServerBaseUrl({ LICENSE_SERVER_URL: "http://ls:8080" })).toBe("http://ls:8080");
  });

  it("falls back to empty (same-origin) when unset", () => {
    expect(licenseServerBaseUrl({})).toBe("");
  });
});
