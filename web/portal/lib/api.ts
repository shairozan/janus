import { auth } from "@/auth";
import { buildAuthHeaders, licenseServerBaseUrl } from "./api-helpers";

/**
 * apiFetch is the server-side BFF call to the license-server: it reads the
 * current session's Cognito access token and forwards it as a Bearer token.
 * Use only in server contexts (route handlers, server components, server
 * actions) — the access token never has to round-trip through client code.
 */
export async function apiFetch(path: string, init?: RequestInit): Promise<Response> {
  const session = await auth();
  const headers = buildAuthHeaders(session?.accessToken, init?.headers);

  return fetch(`${licenseServerBaseUrl()}${path}`, { ...init, headers });
}
