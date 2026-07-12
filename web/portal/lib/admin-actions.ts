"use server";

import { revalidatePath } from "next/cache";
import { apiFetch } from "./api";
import type {
  AgreementUsage,
  CreateRuleInput,
  SeatPreview,
  SSOConfig,
  SSOSetupInput,
} from "./types";

async function readError(res: Response): Promise<string> {
  const data = (await res.json().catch(() => ({}))) as { error?: string };

  return data.error ?? `request failed (${res.status})`;
}

async function send(method: string, path: string, body?: unknown): Promise<Response> {
  const res = await apiFetch(path, {
    method,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (!res.ok) {
    throw new Error(await readError(res));
  }

  return res;
}

function post(path: string, body?: unknown): Promise<Response> {
  return send("POST", path, body);
}

// ── members ────────────────────────────────────────────────────────────────

export async function promoteMemberAction(orgID: number, userID: number): Promise<void> {
  await post(`/api/v1/orgs/${orgID}/members/${userID}/promote`);
  revalidatePath(`/orgs/${orgID}/members`);
}

export async function demoteMemberAction(orgID: number, userID: number): Promise<void> {
  await post(`/api/v1/orgs/${orgID}/members/${userID}/demote`);
  revalidatePath(`/orgs/${orgID}/members`);
}

export async function offboardMemberAction(orgID: number, userID: number): Promise<void> {
  await post(`/api/v1/orgs/${orgID}/members/${userID}/offboard`);
  revalidatePath(`/orgs/${orgID}/members`);
}

// ── license requests ─────────────────────────────────────────────────────────

export async function approveRequestAction(orgID: number, reqID: number): Promise<void> {
  await post(`/api/v1/orgs/${orgID}/license-requests/${reqID}/approve`);
  revalidatePath(`/orgs/${orgID}/requests`);
}

export async function rejectRequestAction(
  orgID: number,
  reqID: number,
  reason: string
): Promise<void> {
  await post(`/api/v1/orgs/${orgID}/license-requests/${reqID}/reject`, { reason });
  revalidatePath(`/orgs/${orgID}/requests`);
}

// ── auto-acceptance rules ─────────────────────────────────────────────────────

export async function createRuleAction(orgID: number, input: CreateRuleInput): Promise<void> {
  await post(`/api/v1/orgs/${orgID}/rules`, input);
  revalidatePath(`/orgs/${orgID}/rules`);
}

export async function setRuleEnabledAction(
  orgID: number,
  ruleID: number,
  enabled: boolean
): Promise<void> {
  await send("PUT", `/api/v1/orgs/${orgID}/rules/${ruleID}`, { enabled });
  revalidatePath(`/orgs/${orgID}/rules`);
}

export async function deleteRuleAction(orgID: number, ruleID: number): Promise<void> {
  await send("DELETE", `/api/v1/orgs/${orgID}/rules/${ruleID}`);
  revalidatePath(`/orgs/${orgID}/rules`);
}

// ── SSO ───────────────────────────────────────────────────────────────────────

export async function setupSSOAction(orgID: number, input: SSOSetupInput): Promise<SSOConfig> {
  const res = await post(`/api/v1/orgs/${orgID}/sso`, input);
  revalidatePath(`/orgs/${orgID}/sso`);

  return res.json() as Promise<SSOConfig>;
}

// ── seats ─────────────────────────────────────────────────────────────────────

/**
 * previewSeatsAction fetches Stripe's proration preview for adding `add` seats —
 * read-only, shown before the admin confirms. (A GET routed through a server
 * action so the access token never reaches the client.)
 */
export async function previewSeatsAction(
  orgID: number,
  agreementID: number,
  add: number
): Promise<SeatPreview> {
  const res = await apiFetch(
    `/api/v1/orgs/${orgID}/agreements/${agreementID}/seats/preview?add=${add}`
  );
  if (!res.ok) {
    throw new Error(await readError(res));
  }

  return res.json() as Promise<SeatPreview>;
}

export async function addSeatsAction(
  orgID: number,
  agreementID: number,
  add: number
): Promise<AgreementUsage> {
  const res = await post(`/api/v1/orgs/${orgID}/agreements/${agreementID}/seats`, { add });
  revalidatePath(`/orgs/${orgID}/seats`);

  return res.json() as Promise<AgreementUsage>;
}
