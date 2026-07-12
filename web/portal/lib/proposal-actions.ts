"use server";

import { revalidatePath } from "next/cache";
import { apiFetch } from "./api";
import type { AgreementProposal, RequestProposalInput } from "./types";

async function readError(res: Response): Promise<string> {
  const data = (await res.json().catch(() => ({}))) as { error?: string };

  return data.error ?? `request failed (${res.status})`;
}

async function post(orgID: number, path: string, body?: unknown): Promise<AgreementProposal> {
  const res = await apiFetch(path, {
    method: "POST",
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (!res.ok) {
    throw new Error(await readError(res));
  }

  revalidatePath(`/orgs/${orgID}/billing`);

  return res.json() as Promise<AgreementProposal>;
}

/** requestProposalAction asks Janus for an agreement with the given terms. */
export async function requestProposalAction(
  orgID: number,
  input: RequestProposalInput
): Promise<AgreementProposal> {
  return post(orgID, `/api/v1/orgs/${orgID}/proposals`, input);
}

/** counterProposalAction responds to an offer with revised terms (a new round). */
export async function counterProposalAction(
  orgID: number,
  pid: number,
  input: RequestProposalInput
): Promise<AgreementProposal> {
  return post(orgID, `/api/v1/orgs/${orgID}/proposals/${pid}/counter`, input);
}

/** acceptProposalAction accepts a proposed offer (terms are locked, awaiting payment). */
export async function acceptProposalAction(orgID: number, pid: number): Promise<AgreementProposal> {
  return post(orgID, `/api/v1/orgs/${orgID}/proposals/${pid}/accept`);
}

/** payProposalAction pays an accepted offer — creates the subscription + activates the agreement. */
export async function payProposalAction(orgID: number, pid: number): Promise<AgreementProposal> {
  return post(orgID, `/api/v1/orgs/${orgID}/proposals/${pid}/pay`);
}
