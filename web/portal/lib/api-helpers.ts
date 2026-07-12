// Framework-free helpers, unit-testable without initializing Auth.js.

/** licenseServerBaseUrl is the license-server origin the BFF proxies to. */
export function licenseServerBaseUrl(
  env: Record<string, string | undefined> = process.env
): string {
  return env.LICENSE_SERVER_URL ?? "";
}

/**
 * buildAuthHeaders returns request headers with the Cognito access token
 * attached as a Bearer token (when present), defaulting JSON content-type.
 */
export function buildAuthHeaders(token: string | undefined, init?: HeadersInit): Headers {
  const headers = new Headers(init);
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  if (!headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  return headers;
}

/**
 * ssoAuthParams returns the extra Cognito-authorization params that send a user
 * straight to their organization's IdP on the hosted UI (bypassing the provider
 * picker). Pass the SSOConfiguration.ProviderName. Empty/blank → no SSO param
 * (falls back to the native hosted-UI login form).
 */
export function ssoAuthParams(providerName: string): Record<string, string> {
  const name = providerName.trim();
  return name ? { identity_provider: name } : {};
}
