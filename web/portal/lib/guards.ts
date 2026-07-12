import { redirect } from "next/navigation";
import { getProfile } from "./portal-api";
import { ROLE_ADMIN, type Profile } from "./types";

/**
 * requireAdmin guards an admin page: the caller must be a customer_admin of the
 * org in the route. The license-server API enforces this too (defense in depth);
 * this gives a clean redirect instead of a 403 in the UI.
 */
export async function requireAdmin(orgID: number): Promise<Profile> {
  const profile = await getProfile();
  if (profile.role !== ROLE_ADMIN || profile.organization_id !== orgID) {
    redirect("/me");
  }

  return profile;
}

/**
 * requireStaff guards the Janus-staff "Admin" area: the caller's email domain
 * must be in the staff allowlist (surfaced as profile.is_staff). The
 * license-server enforces this on every staff endpoint too (defense in depth);
 * this just yields a clean redirect instead of a wall of 403s in the UI.
 */
export async function requireStaff(): Promise<Profile> {
  const profile = await getProfile();
  if (!profile.is_staff) {
    redirect("/me");
  }

  return profile;
}
