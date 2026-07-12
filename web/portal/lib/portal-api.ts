// Server-side BFF reads from the license-server. Each function runs in a server
// context (apiFetch forwards the session's Bearer token). Used by Server
// Components.
import { cache } from "react";
import { apiFetch } from "./api";
import type {
  AgreementProposal,
  AgreementUsage,
  AuditEvent,
  AutoAcceptanceRule,
  KeysResponse,
  LicenseRequest,
  OrgUser,
  Profile,
  SSOConfig,
} from "./types";

export async function getJSON<T>(path: string): Promise<T> {
  const res = await apiFetch(path);
  if (!res.ok) {
    throw new Error(`GET ${path} failed: ${res.status}`);
  }

  return res.json() as Promise<T>;
}

/**
 * getArray fetches a JSON list. The Go API serializes an empty result as a nil
 * slice → `null`, so coerce to [] here; every list consumer can then .map/.length
 * safely without each guarding for null.
 */
async function getArray<T>(path: string): Promise<T[]> {
  return (await getJSON<T[] | null>(path)) ?? [];
}

// cache() dedupes the profile fetch within a single request (the shell + the
// page both need it).
export const getProfile = cache((): Promise<Profile> => getJSON<Profile>("/api/v1/me"));

export function listMembers(orgID: number): Promise<OrgUser[]> {
  return getArray<OrgUser>(`/api/v1/orgs/${orgID}/members`);
}

export function listOrgRequests(orgID: number): Promise<LicenseRequest[]> {
  return getArray<LicenseRequest>(`/api/v1/orgs/${orgID}/license-requests`);
}

export function listRules(orgID: number): Promise<AutoAcceptanceRule[]> {
  return getArray<AutoAcceptanceRule>(`/api/v1/orgs/${orgID}/rules`);
}

export function listAgreements(orgID: number): Promise<AgreementUsage[]> {
  return getArray<AgreementUsage>(`/api/v1/orgs/${orgID}/agreements`);
}

export function listActivity(orgID: number): Promise<AuditEvent[]> {
  return getArray<AuditEvent>(`/api/v1/orgs/${orgID}/activity`);
}

export function listProposals(orgID: number): Promise<AgreementProposal[]> {
  return getArray<AgreementProposal>(`/api/v1/orgs/${orgID}/proposals`);
}

export function getProposal(orgID: number, pid: number): Promise<AgreementProposal> {
  return getJSON<AgreementProposal>(`/api/v1/orgs/${orgID}/proposals/${pid}`);
}

/** getSSO returns the org's SSO configuration, or null if none is set up yet. */
export async function getSSO(orgID: number): Promise<SSOConfig | null> {
  const res = await apiFetch(`/api/v1/orgs/${orgID}/sso`);
  if (res.status === 404) {
    return null;
  }

  if (!res.ok) {
    throw new Error(`GET /api/v1/orgs/${orgID}/sso failed: ${res.status}`);
  }

  return res.json() as Promise<SSOConfig>;
}

export async function getKeys(): Promise<KeysResponse> {
  const keys = await getJSON<KeysResponse>("/api/v1/me/key");

  // history may come back as a nil slice (`null`) for a user with no prior keys.
  return { active: keys.active ?? null, history: keys.history ?? [] };
}

export function getLicenseRequests(): Promise<LicenseRequest[]> {
  return getArray<LicenseRequest>("/api/v1/me/license-requests");
}

/** getLicense returns the user's downloadable license JWT, or null if none yet. */
export async function getLicense(): Promise<string | null> {
  const res = await apiFetch("/api/v1/me/license");
  if (res.status === 404) {
    return null;
  }

  if (!res.ok) {
    throw new Error(`GET /api/v1/me/license failed: ${res.status}`);
  }

  const data = (await res.json()) as { license: string };

  return data.license;
}
