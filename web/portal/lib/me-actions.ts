"use server";

import { revalidatePath } from "next/cache";
import { apiFetch } from "./api";
import type { LicenseRequest, PublicKey, ReplaceKeyInput } from "./types";

async function readError(res: Response): Promise<string> {
  const data = (await res.json().catch(() => ({}))) as { error?: string };

  return data.error ?? `request failed (${res.status})`;
}

/**
 * replaceKeyAction sets the caller's active public key. The portal requires the
 * user to acknowledge the rotation caveat before calling this (the UI enforces
 * it); we forward acknowledged=true. The server performs the atomic
 * revoke-prior-key + revoke-prior-license + reissue transaction.
 */
export async function replaceKeyAction(input: ReplaceKeyInput): Promise<PublicKey> {
  const res = await apiFetch("/api/v1/me/key", {
    method: "PUT",
    body: JSON.stringify({
      public_key_pem: input.publicKeyPem,
      title: input.title,
      acknowledged: true,
    }),
  });

  if (!res.ok) {
    throw new Error(await readError(res));
  }

  revalidatePath("/me");

  return res.json() as Promise<PublicKey>;
}

/** requestLicenseAction requests a license/seat against an agreement. */
export async function requestLicenseAction(agreementId: number): Promise<LicenseRequest> {
  const res = await apiFetch("/api/v1/me/license-requests", {
    method: "POST",
    body: JSON.stringify({ agreement_id: agreementId }),
  });

  if (!res.ok) {
    throw new Error(await readError(res));
  }

  revalidatePath("/me");

  return res.json() as Promise<LicenseRequest>;
}
