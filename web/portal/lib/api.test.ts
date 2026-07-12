import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock the Auth.js module so importing apiFetch doesn't boot NextAuth, and we
// can inject a session. vi.hoisted lets the mock factory (hoisted to the top of
// the file) reference authMock safely.
const { authMock } = vi.hoisted(() => ({ authMock: vi.fn() }));
vi.mock("@/auth", () => ({ auth: authMock }));

import { apiFetch } from "./api";

describe("apiFetch (BFF)", () => {
  beforeEach(() => {
    authMock.mockReset();
    process.env.LICENSE_SERVER_URL = "http://ls:8080";
  });

  it("forwards the session access token as a Bearer header to the license-server", async () => {
    authMock.mockResolvedValue({ accessToken: "access-xyz" });
    const fetchSpy = vi.spyOn(global, "fetch").mockResolvedValue(new Response("ok"));

    await apiFetch("/api/v1/me");

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const [url, init] = fetchSpy.mock.calls[0];
    expect(url).toBe("http://ls:8080/api/v1/me");
    expect((init?.headers as Headers).get("Authorization")).toBe("Bearer access-xyz");

    fetchSpy.mockRestore();
  });

  it("makes an unauthenticated call when there is no session", async () => {
    authMock.mockResolvedValue(null);
    const fetchSpy = vi.spyOn(global, "fetch").mockResolvedValue(new Response("ok"));

    await apiFetch("/health");

    const [, init] = fetchSpy.mock.calls[0];
    expect((init?.headers as Headers).has("Authorization")).toBe(false);

    fetchSpy.mockRestore();
  });
});
