// Server-side BFF reads for the Janus-staff "Admin" area. These hit the
// license-server's staff endpoints (email-domain gated); apiFetch forwards the
// session's Bearer token. Used by Server Components under app/(app)/admin/.
import { getJSON } from "./portal-api";
import type { AgreementSummary, LicenseTier, OrgSummary, SigningKey } from "./types";

// The Go API serializes an empty result as a nil slice → `null`; coerce to [].
async function getList<T>(path: string): Promise<T[]> {
  return (await getJSON<T[] | null>(path)) ?? [];
}

/**
 * listSigningKeys returns signing keys newest-first. Pass an orgID to scope to a
 * single organization's keys; omit it for every key (incl. the master key).
 */
export function listSigningKeys(orgID?: number): Promise<SigningKey[]> {
  const q = orgID != null ? `?organization_id=${orgID}` : "";

  return getList<SigningKey>(`/api/v1/keys${q}`);
}

/** listOrganizations returns all active organizations (staff org picker). */
export function listOrganizations(): Promise<OrgSummary[]> {
  return getList<OrgSummary>("/api/v1/organizations/list");
}

/** listAgreements returns all agreements (staff agreement picker). */
export function listAgreements(): Promise<AgreementSummary[]> {
  return getList<AgreementSummary>("/api/v1/agreements/list");
}

/** listLicenses returns the active license tiers (the agreement-create catalog). */
export function listLicenses(): Promise<LicenseTier[]> {
  return getList<LicenseTier>("/api/v1/licenses");
}
