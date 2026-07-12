"use server";

import { revalidatePath } from "next/cache";
import { apiFetch } from "./api";
import type {
  AgreementSummary,
  CreateAgreementInput,
  IssuedLicense,
  IssueLicenseInput,
  RotateKeyInput,
  RotateKeyResult,
} from "./types";

async function readError(res: Response): Promise<string> {
  const data = (await res.json().catch(() => ({}))) as { error?: string };

  return data.error ?? `request failed (${res.status})`;
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await apiFetch(path, { method: "POST", body: JSON.stringify(body) });
  if (!res.ok) {
    throw new Error(await readError(res));
  }

  return res.json() as Promise<T>;
}

/**
 * rotateKeyAction creates a new signing key (rotating the prior active one for
 * the same scope). Omit organization_id for the master key. Returns the new
 * key id + public PEM.
 */
export async function rotateKeyAction(input: RotateKeyInput): Promise<RotateKeyResult> {
  const result = await postJSON<RotateKeyResult>("/api/v1/keys/rotate", input);
  revalidatePath("/admin/keys");

  return result;
}

/**
 * issueLicenseAction mints a license JWT for a user against an agreement, signed
 * by the org's active signing key. duration_seconds and signing_public_key are
 * optional (duration derives from the agreement term when omitted).
 */
export async function issueLicenseAction(input: IssueLicenseInput): Promise<IssuedLicense> {
  return postJSON<IssuedLicense>("/api/v1/tokens", input);
}

/**
 * createAgreementAction defines an agreement directly (staff), bypassing the
 * customer proposal/payment flow — for internal, comp, or demo agreements you
 * then issue licenses against.
 */
export async function createAgreementAction(
  input: CreateAgreementInput
): Promise<AgreementSummary> {
  const agreement = await postJSON<AgreementSummary>("/api/v1/agreements", input);
  revalidatePath("/admin/agreements");
  revalidatePath("/admin/licenses");

  return agreement;
}
